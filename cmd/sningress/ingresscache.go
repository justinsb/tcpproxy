package main

import (
	"context"
	"sync"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type IngressCache struct {
	mutex sync.RWMutex

	ingresses  map[types.NamespacedName]*networkingv1.Ingress
	hostsIndex map[string]ImmutableList_Ingress
}

func (c *IngressCache) Start(ctx context.Context, kube *SimpleClientset, namespace string) error {
	c.ingresses = make(map[types.NamespacedName]*networkingv1.Ingress)
	c.hostsIndex = make(map[string]ImmutableList_Ingress)

	resource, err := kube.Resource(networkingv1.SchemeGroupVersion.WithResource("ingresses"))
	if err != nil {
		return err
	}

	lw := NewListWatch(resource, namespace, c)
	go func() {
		lw.WatchForever(ctx)
	}()

	return nil
}

func (c *IngressCache) WaitForSync(ctx context.Context) error {
	klog.Warningf("WaitForSync is not implemented")
	time.Sleep(5 * time.Second)
	return nil
}

func (c *IngressCache) OnUpdate(obj runtime.Object) {
	ingress := obj.(*networkingv1.Ingress)
	key := types.NamespacedName{
		Namespace: ingress.Namespace,
		Name:      ingress.Name,
	}

	newHosts := computeHosts(ingress)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	old := c.ingresses[key]
	oldHosts := computeHosts(old)

	c.ingresses[key] = ingress

	c.updateIndex(oldHosts, newHosts, ingress)

	// for _, rule := range ingress.Spec.Rules {
	// 	if rule.HTTP == nil {
	// 		klog.Warningf("skipping rule with no HTTP section: %v", ingress)
	// 		continue
	// 	}

	// 	if rule.Host == "" {
	// 		klog.Warningf("skipping rule with no HTTP host: %v", ingress)
	// 		continue
	// 	}

	// 	hosts = append(hosts, rule.Host)

	// 	// for _, httpPath := range rule.HTTP.Paths {
	// 	// 	if httpPath.Path != "/" {
	// 	// 		klog.Warningf("skipping http rule where path is not '/': %v", httpPath)
	// 	// 		continue
	// 	// 	}

	// 	// 	if httpPath.Backend.Service == nil {
	// 	// 		klog.Warningf("skipping http rule where service is not set: %v", httpPath)
	// 	// 		continue
	// 	// 	}

	// 	// 	serviceName := httpPath.Backend.Service.Name
	// 	// 	servicePort := httpPath.Backend.Service.Port

	// 	// 	key := ingressKey{
	// 	// 		Host:        rule.Host,
	// 	// 		ServiceName: serviceName,
	// 	// 		ServicePort: servicePort,
	// 	// 	}
	// 	// }
	// }
}

func computeHosts(ingress *networkingv1.Ingress) map[string]bool {
	if ingress == nil {
		return nil
	}

	hosts := make(map[string]bool)
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			klog.Warningf("skipping rule with no HTTP section: %v", ingress)
			continue
		}

		if rule.Host == "" {
			klog.Warningf("skipping rule with no HTTP host: %v", ingress)
			continue
		}

		hosts[rule.Host] = true
	}
	return hosts
}

func (c *IngressCache) OnDelete(obj runtime.Object) {
	ingress := obj.(*networkingv1.Ingress)
	key := types.NamespacedName{
		Namespace: ingress.Namespace,
		Name:      ingress.Name,
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	old := c.ingresses[key]
	oldHosts := computeHosts(old)

	delete(c.ingresses, key)

	c.updateIndex(oldHosts, nil, old)
}

func (c *IngressCache) updateIndex(oldHosts, newHosts map[string]bool, ingress *networkingv1.Ingress) {
	for k := range newHosts {
		if oldHosts[k] {
			c.hostsIndex[k] = c.hostsIndex[k].Replace(ingress)
		} else {
			c.hostsIndex[k] = c.hostsIndex[k].Add(ingress)
		}
	}

	for k := range oldHosts {
		if newHosts[k] {
			continue
		}

		c.hostsIndex[k] = c.hostsIndex[k].Remove(ingress)
	}
}

func (c *IngressCache) LookupHostname(hostname string) ImmutableList_Ingress {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.hostsIndex[hostname]
}
