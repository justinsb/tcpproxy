package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"inet.af/tcpproxy"
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

	listen := ":8443"
	flag.StringVar(&listen, "listen", listen, "endpoint on which to listen locally")
	kubeconfig := ""
	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "path to the kubeconfig file")
	flag.Parse()

	var config *rest.Config
	if kubeconfig == "" {
		c, err := rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("error loading in-cluster kube config: %w", err)
		}
		config = c
	} else {
		c, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("error loading kubeconfig in %q: %w", kubeconfig, err)
		}
		config = c
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	ingresses, err := clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing ingresses: %w", err)
	}

	endpointSlices, err := clientset.DiscoveryV1().EndpointSlices("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing ingresses: %w", err)
	}

	var p tcpproxy.Proxy
	//     p.AddHTTPHostRoute(":80", "foo.com", tcpproxy.To("10.0.0.1:8081"))
	//     p.AddHTTPHostRoute(":80", "bar.com", tcpproxy.To("10.0.0.2:8082"))
	//     p.AddRoute(":80", tcpproxy.To("10.0.0.1:8081")) // fallback
	//     p.AddSNIRoute(":443", "foo.com", tcpproxy.To("10.0.0.1:4431"))
	//     p.AddSNIRoute(":443", "bar.com", tcpproxy.To("10.0.0.2:4432"))
	//     p.AddRoute(":443", tcpproxy.To("10.0.0.1:4431")) // fallback
	//     log.Fatal(p.Run())

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

		for _, tls := range ingress.Spec.TLS {
			for _, host := range tls.Hosts {
				klog.Infof("%q sni(%q) => ingress %s/%s (service %s) => %v", listen, host, ingress.Namespace, ingress.Name, serviceName, addresses)
				p.AddSNIRoute(listen, host, tcpproxy.To(addresses[0]))
			}
		}
	}

	return p.Run()
}
