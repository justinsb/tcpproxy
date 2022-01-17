package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type EndpointCache struct {
	mutex sync.RWMutex

	endpoints map[types.NamespacedName]*discoveryv1.EndpointSlice

	// We use an ImmutableList so we don't need a defensive copy

	serviceIndex map[types.NamespacedName]ImmutableList_EndpointSlice
}

func (c *EndpointCache) Start(ctx context.Context, kube *SimpleClientset, namespace string) error {
	c.endpoints = make(map[types.NamespacedName]*discoveryv1.EndpointSlice)
	c.serviceIndex = make(map[types.NamespacedName]ImmutableList_EndpointSlice)

	resource, err := kube.Resource(discoveryv1.SchemeGroupVersion.WithResource("endpointslices"))
	if err != nil {
		return err
	}

	lw := NewListWatch(resource, namespace, c)
	go func() {
		lw.WatchForever(ctx)
	}()

	return nil
}

func (c *EndpointCache) WaitForSync(ctx context.Context) error {
	klog.Warningf("WaitForSync is not implemented")
	time.Sleep(5 * time.Second)
	return nil
}

func (c *EndpointCache) OnUpdate(obj runtime.Object) {
	endpoints := obj.(*discoveryv1.EndpointSlice)
	key := types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	}

	newIndexed := computeServiceID(endpoints)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	old := c.endpoints[key]
	oldIndexed := computeServiceID(old)

	c.endpoints[key] = endpoints

	c.updateIndex(oldIndexed, newIndexed, endpoints)
}

func computeServiceID(endpoints *discoveryv1.EndpointSlice) Optional_NamespacedName {
	if endpoints == nil {
		return nil
	}

	serviceNamespace := endpoints.Namespace
	serviceName := endpoints.Labels["kubernetes.io/service-name"]

	if serviceName == "" {
		klog.Warningf("could not get service-name from endpoints %s/%s", endpoints.Namespace, endpoints.Name)
		return nil
	}

	return Optional_NamespacedName{
		{Namespace: serviceNamespace, Name: serviceName},
	}
}

func (c *EndpointCache) OnDelete(obj runtime.Object) {
	endpoints := obj.(*discoveryv1.EndpointSlice)
	key := types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	old := c.endpoints[key]

	if old != nil {
		oldIndexed := computeServiceID(old)

		delete(c.endpoints, key)

		c.updateIndex(oldIndexed, nil, old)
	}
}

type FormatJSON struct {
	obj interface{}
}

func (f FormatJSON) String() string {
	b, err := json.Marshal(f.obj)
	if err != nil {
		return fmt.Sprintf("<jsonError:%v>", err)
	}
	return string(b)
}

func (c *EndpointCache) updateIndex(oldIndexed, newIndexed Optional_NamespacedName, endpoints *discoveryv1.EndpointSlice) {
	for _, k := range newIndexed {
		if oldIndexed.Contains(k) {
			c.serviceIndex[k] = c.serviceIndex[k].Replace(endpoints)
		} else {
			c.serviceIndex[k] = c.serviceIndex[k].Add(endpoints)
		}
	}

	for _, k := range oldIndexed {
		if newIndexed.Contains(k) {
			continue
		}

		c.serviceIndex[k] = c.serviceIndex[k].Remove(endpoints)
	}
}

func (c *EndpointCache) LookupService(service types.NamespacedName) ImmutableList_EndpointSlice {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	values := c.serviceIndex[service]
	return values
}
