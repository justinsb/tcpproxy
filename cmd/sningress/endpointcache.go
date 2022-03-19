package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type EndpointCache struct {
	// mutex sync.RWMutex

	// endpoints map[types.NamespacedName]*discoveryv1.EndpointSlice

	// serviceIndex MultiMap[types.NamespacedName, *discoveryv1.EndpointSlice]

	endpoints IndexedMap[types.NamespacedName, *discoveryv1.EndpointSlice]
	byService *MapIndex[types.NamespacedName, *discoveryv1.EndpointSlice, types.NamespacedName]
}

// func EndpointSliceToNamespaceName(v *discoveryv1.EndpointSlice) types.NamespacedName {
// 	return types.NamespacedName{
// 		Namespace: v.Namespace,
// 		Name:      v.Name,
// 	}
// }

func (c *EndpointCache) Start(ctx context.Context, kube *SimpleClientset, namespace string) error {
	// c.endpoints = make(map[types.NamespacedName]*discoveryv1.EndpointSlice)
	// c.serviceIndex = NewMultiMap[types.NamespacedName, *discoveryv1.EndpointSlice]()

	// c.endpoints = NewMapIndex (map[types.NamespacedName]*discoveryv1.EndpointSlice)
	c.byService = AddIndex(&c.endpoints, computeServiceID)

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
	c.endpoints.Put(key, endpoints)

	// newIndexed := computeServiceID(endpoints)

	// c.mutex.Lock()
	// defer c.mutex.Unlock()

	// old := c.endpoints[key]
	// oldIndexed := computeServiceID(old)

	// c.endpoints[key] = endpoints

	// c.serviceIndex.Update(oldIndexed, newIndexed, endpoints, MapAndCompare(EndpointSliceToNamespaceName))
}

func computeServiceID(endpoints *discoveryv1.EndpointSlice) []types.NamespacedName {
	if endpoints == nil {
		return nil
	}

	serviceNamespace := endpoints.Namespace
	serviceName := endpoints.Labels["kubernetes.io/service-name"]

	if serviceName == "" {
		klog.Warningf("could not get service-name from endpoints %s/%s", endpoints.Namespace, endpoints.Name)
		return nil
	}

	return []types.NamespacedName{
		{Namespace: serviceNamespace, Name: serviceName},
	}
}

func (c *EndpointCache) OnDelete(obj runtime.Object) {
	endpoints := obj.(*discoveryv1.EndpointSlice)
	key := types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	}

	c.endpoints.Delete(key)
	// c.mutex.Lock()
	// defer c.mutex.Unlock()

	// old := c.endpoints[key]

	// if old != nil {
	// 	oldIndexed := computeServiceID(old)

	// 	delete(c.endpoints, key)

	// 	c.serviceIndex.Update(oldIndexed, nil, old, MapAndCompare(EndpointSliceToNamespaceName))
	// }
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

func (c *EndpointCache) LookupService(service types.NamespacedName) ImmutableList[*discoveryv1.EndpointSlice] {
	// c.mutex.RLock()
	// defer c.mutex.RUnlock()

	values := c.byService.Lookup(service)
	return values
}
