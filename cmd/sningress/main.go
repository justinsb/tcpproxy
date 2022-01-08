package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"inet.af/tcpproxy/pkg/proxy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
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
	listen := ":8443"
	flag.StringVar(&listen, "listen", listen, "endpoint on which to listen locally")
	kubeconfig := ""
	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "path to the kubeconfig file")
	flag.Parse()

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

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	var config Config
	if err := config.BuildFromKubernetes(ctx, clientset); err != nil {
		return fmt.Errorf("error reading kubernetes config: %w", err)
	}

	p := proxy.New(&config, helloTimeout)

	return p.ListenAndServe(listen)
}

type Config struct {
	hostnames map[string]*hostnameConfig
}

type hostnameConfig struct {
	addresses []string
}

func (c *Config) BuildFromKubernetes(ctx context.Context, clientset kubernetes.Interface) error {
	ingresses, err := clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing ingresses: %w", err)
	}

	endpointSlices, err := clientset.DiscoveryV1().EndpointSlices("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing ingresses: %w", err)
	}

	hostnames := make(map[string]*hostnameConfig)
	for _, ingress := range ingresses.Items {
		var addresses []string

		serviceName := ingress.Spec.DefaultBackend.Service.Name
		servicePort := ingress.Spec.DefaultBackend.Service.Port

		for _, endpointSlice := range endpointSlices.Items {
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

		if len(addresses) == 0 {
			klog.Warningf("could not find endpoints for ingress %s/%s (service %s)", ingress.Namespace, ingress.Name, serviceName)
			continue
		}

		hostnameConfig := &hostnameConfig{
			addresses: addresses,
		}

		for _, tls := range ingress.Spec.TLS {
			for _, host := range tls.Hosts {
				klog.Infof("sni(%q) => ingress %s/%s (service %s) => %v", host, ingress.Namespace, ingress.Name, serviceName, addresses)
				hostnames[host] = hostnameConfig
			}
		}
	}

	c.hostnames = hostnames
	return nil
}

// Match implements proxy.Config
func (c *Config) Match(hostname string) (string, bool) {
	config := c.hostnames[hostname]
	if config == nil {
		return "", false
	}

	if len(config.addresses) == 0 {
		return "", false
	}

	klog.Infof("mapping %q => %q", hostname, config.addresses[0])
	return config.addresses[0], false
}
