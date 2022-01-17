package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"inet.af/tcpproxy/pkg/proxy"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func main() {
	err := run(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	klog.InitFlags(nil)

	helloTimeout := 3 * time.Second
	listenHTTPS := ":8443"
	flag.StringVar(&listenHTTPS, "listen-https", listenHTTPS, "endpoint on which to listen for https")
	listenHTTP := ":8080"
	flag.StringVar(&listenHTTP, "listen-http", listenHTTP, "endpoint on which to listen for http")
	kubeconfig := ""
	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "path to the kubeconfig file")

	useTunnel := true
	flag.BoolVar(&useTunnel, "use-tunnel", useTunnel, "should we use tunnel")

	flag.Parse()

	tunnel := &tunnelBackend{
		proxyUdsName: "/etc/kubernetes/konnectivity-server/uds/konnectivity-server.socket",
	}
	if !useTunnel {
		tunnel = nil
	}

	var restConfig *rest.Config
	if kubeconfig == "" {
		c, err := rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("error loading in-cluster kube config: %w", err)
		}
		restConfig = c
	} else {
		c, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("error loading kubeconfig in %q: %w", kubeconfig, err)
		}
		restConfig = c
	}

	var config Config
	if err := config.BuildFromKubernetes(ctx, restConfig, tunnel); err != nil {
		return fmt.Errorf("error reading kubernetes config: %w", err)
	}

	if listenHTTP != "" {
		hp, err := NewHTTPProxy(&config)
		if err != nil {
			return fmt.Errorf("error building http proxy: %w", err)
		}
		go func() {
			if err := hp.ListenAndServe(listenHTTP); err != nil {
				klog.Fatalf("error from http proxy: %v", err)
			}
		}()
	}

	p := proxy.New(&config, helloTimeout)
	return p.ListenAndServe(listenHTTPS)
}

type Config struct {
	ingresses IngressCache
	endpoints EndpointCache

	tunnel *tunnelBackend
}

func (c *Config) BuildFromKubernetes(ctx context.Context, restConfig *rest.Config, tunnel *tunnelBackend) error {
	schema, err := NewSchema(networkingv1.AddToScheme, discoveryv1.AddToScheme)
	if err != nil {
		return err
	}
	kube, err := NewForConfig(restConfig, schema)
	if err != nil {
		return err
	}

	namespace := ""

	if err := c.ingresses.Start(ctx, kube, namespace); err != nil {
		return fmt.Errorf("failed to watch ingresses: %w", err)
	}

	if err := c.endpoints.Start(ctx, kube, namespace); err != nil {
		return fmt.Errorf("failed to watch endpoints: %w", err)
	}

	if err := c.ingresses.WaitForSync(ctx); err != nil {
		return fmt.Errorf("failed to sync ingresses: %w", err)
	}

	if err := c.endpoints.WaitForSync(ctx); err != nil {
		return fmt.Errorf("failed to sync endpoints: %w", err)
	}
	return nil
}

func (c *Config) lookupHostname(hostname string) *backend {
	ingresses := c.ingresses.LookupHostname(hostname)

	var addresses []string

	for _, ingress := range ingresses {
		for _, rule := range ingress.Spec.Rules {
			// TODO: Maybe we should index the rules instead of the ingress object?
			if rule.Host != hostname {
				continue
			}

			if rule.HTTP == nil {
				klog.Warningf("skipping rule with no HTTP section: %v", ingress)
				continue
			}

			for _, httpPath := range rule.HTTP.Paths {
				if httpPath.Path != "/" {
					klog.Warningf("skipping http rule where path is not '/': %v", httpPath)
					continue
				}

				if httpPath.Backend.Service == nil {
					klog.Warningf("skipping http rule where service is not set: %v", httpPath)
					continue
				}

				serviceName := httpPath.Backend.Service.Name
				servicePort := httpPath.Backend.Service.Port

				// klog.Infof("hostname %q => ingress %s/%s => service %s", hostname, ingress.Namespace, ingress.Name, serviceName)

				endpoints := c.endpoints.LookupService(types.NamespacedName{Namespace: ingress.Namespace, Name: serviceName})

				for _, endpointSlice := range endpoints {
					// klog.Infof("endpoints %v", endpointSlice)

					if endpointSlice.Namespace != ingress.Namespace {
						continue
					}
					if endpointSlice.Labels["kubernetes.io/service-name"] != serviceName {
						continue
					}

					var targetPort *int32
					if servicePort.Name == "" {
						if len(endpointSlice.Ports) != 1 {
							// port name should be required if using multiple ports
							klog.Warningf("unexpected number of ports for unnamed endpoint %s/%s", endpointSlice.Namespace, endpointSlice.Name)
							continue
						}
						targetPort = endpointSlice.Ports[0].Port
					} else {
						for _, port := range endpointSlice.Ports {
							if port.Name != nil && *port.Name == servicePort.Name {
								targetPort = port.Port
							}
						}

					}

					if targetPort == nil {
						klog.Warningf("could not find targetPort for ingress %s/%s (service %s)", ingress.Namespace, ingress.Name, serviceName)
						continue
					}

					for _, endpoint := range endpointSlice.Endpoints {
						for _, address := range endpoint.Addresses {
							addresses = append(addresses, fmt.Sprintf("%s:%d", address, *targetPort))
						}
					}
				}

			}
		}
	}

	if len(addresses) == 0 {
		klog.Warningf("could not find endpoints for hostname %q", hostname)
		return nil
	}

	klog.Infof("%q => %v", hostname, addresses)

	b := &backend{
		addresses: addresses,
	}

	klog.Warningf("HACK: assuming namespace != kube-system <=> use tunnel")
	for _, ingress := range ingresses {
		if ingress.Namespace != "kube-system" {
			b.tunnel = c.tunnel
		}
	}

	return b
}

func (c *Config) Dial(ctx context.Context, hostname string) (proxy.NetConn, error) {
	backend := c.lookupHostname(hostname)
	if backend == nil {
		return nil, fmt.Errorf("ingress not found for hostname %q", hostname)
	}

	return backend.Dial(ctx, hostname)
}

// Match implements proxy.Config
func (c *Config) Match(hostname string) (proxy.Backend, bool) {
	backend := c.lookupHostname(hostname)
	if backend == nil {
		return nil, false
	}

	return backend, false
}

type backend struct {
	addresses []string

	tunnel *tunnelBackend
}

var _ proxy.Backend = &backend{}

func (b *backend) Dial(ctx context.Context, hostname string) (proxy.NetConn, error) {
	if len(b.addresses) == 0 {
		return nil, fmt.Errorf("no addresses for backend")
	}

	addr := b.addresses[0]
	klog.Infof("mapping %q => %q", hostname, addr)

	if b.tunnel != nil {
		c, err := b.tunnel.Dial(ctx, addr)
		if err != nil {
			klog.Warningf("tunnel failed to dial %q: %v", addr, err)
		}
		return c, err
	}

	dialer := net.Dialer{
		Timeout: 10 * time.Second,
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %q: %w", addr, err)
	}

	return conn.(*net.TCPConn), nil
}
