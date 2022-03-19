package main

import (
	"context"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type IngressCache struct {
	// mutex sync.RWMutex

	// ingresses  map[types.NamespacedName]*networkingv1.Ingress
	// hostsIndex MultiMap[string, *networkingv1.Ingress]

	ingresses IndexedMap[types.NamespacedName, *networkingv1.Ingress]
	byHost    *MapIndex[types.NamespacedName, *networkingv1.Ingress, string]
}

// func NetworkingIngressToNamespaceName(k *networkingv1.Ingress) types.NamespacedName {
// 	return types.NamespacedName{
// 		Namespace: k.Namespace,
// 		Name:      k.Name,
// 	}
// }

func (c *IngressCache) Start(ctx context.Context, kube *SimpleClientset, namespace string) error {
	// c.ingresses = make(map[types.NamespacedName]*networkingv1.Ingress)
	// c.hostsIndex = NewMultiMap[string, *networkingv1.Ingress]()

	c.byHost = AddIndex(&c.ingresses, computeHosts)

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

	c.ingresses.Put(key, ingress)
	// newHosts := computeHosts(ingress)

	// c.mutex.Lock()
	// defer c.mutex.Unlock()

	// old := c.ingresses[key]
	// oldHosts := computeHosts(old)

	// c.ingresses[key] = ingress

	// c.hostsIndex.Update(oldHosts, newHosts, ingress, MapAndCompare(NetworkingIngressToNamespaceName))

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

func computeHosts(ingress *networkingv1.Ingress) []string {
	if ingress == nil {
		return nil
	}

	var hosts []string
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			klog.Warningf("skipping rule with no HTTP section: %v", ingress)
			continue
		}

		if rule.Host == "" {
			klog.Warningf("skipping rule with no HTTP host: %v", ingress)
			continue
		}

		hosts = append(hosts, rule.Host)
	}
	return hosts
}

func (c *IngressCache) OnDelete(obj runtime.Object) {
	ingress := obj.(*networkingv1.Ingress)
	key := types.NamespacedName{
		Namespace: ingress.Namespace,
		Name:      ingress.Name,
	}

	c.ingresses.Delete(key)
	// c.mutex.Lock()
	// defer c.mutex.Unlock()

	// old := c.ingresses[key]
	// oldHosts := computeHosts(old)

	// delete(c.ingresses, key)

	// c.hostsIndex.Update(oldHosts, nil, old, MapAndCompare(NetworkingIngressToNamespaceName))
}

func (c *IngressCache) LookupHostname(hostname string) ImmutableList[*networkingv1.Ingress] {
	// c.mutex.RLock()
	// defer c.mutex.RUnlock()

	return c.byHost.Lookup(hostname)
}
