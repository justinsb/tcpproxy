package main

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
)

type SimpleListWatch struct {
	namespace string // or "" for all namespaces

	client *ResourceClient

	listener Listener

	resumeRV string
}

func NewListWatch(client *ResourceClient, namespace string, listener Listener) *SimpleListWatch {
	return &SimpleListWatch{
		client:    client,
		namespace: namespace,
		listener:  listener,
	}
}

type Listener interface {
	OnUpdate(obj runtime.Object)
	OnDelete(obj runtime.Object)
}

// Run starts the secretsWatcher.
func (c *SimpleListWatch) WatchForever(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := c.watchOnce(ctx); err != nil {
			klog.Warningf("Unexpected error in secret watch, will retry: %v", err)
		}

		if ctx.Err() == nil {
			time.Sleep(10 * time.Second)
		}
	}
}

// func (c *SimpleListWatch) list(ctx context.Context, rv string) error {
// 	var listOpts metav1.ListOptions

// 	listOpts.ResourceVersion = rv

// 	results, err := c.client.List(ctx, listOpts)
// 	if err != nil {
// 		return fmt.Errorf("error watching secrets: %w", err)
// 	}

// 	for i := range results.Items {
// 		c.listener.OnUpdate(&results.Items[i])
// 		// TODO: If this is a multi-item scan, we need to delete any items not present
// 	}

// }

func (c *SimpleListWatch) watchOnce(ctx context.Context) error {
	var listOpts metav1.ListOptions
	listOpts.Watch = true
	listOpts.AllowWatchBookmarks = true
	listOpts.ResourceVersion = c.resumeRV

	watchFromEmpty := c.resumeRV == ""

	watcher, err := c.client.Watch(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("error watching secrets: %w", err)
	}

	ch := watcher.ResultChan()
	defer watcher.Stop()

	for event := range ch {
		updateRV := true

		switch event.Type {
		case watch.Bookmark:
			updateRV = true

		case watch.Added:
			// If we're watching from empty rv, we can get objects out of order
			updateRV = !watchFromEmpty
			c.listener.OnUpdate(event.Object)

		case watch.Modified:
			updateRV = true
			c.listener.OnUpdate(event.Object)

		case watch.Deleted:
			// TODO: reflector will update the rv, but the object could be old?
			c.listener.OnDelete(event.Object)

		case watch.Error:
			return fmt.Errorf("unexpected error from watch: %v", event)

		default:
			return fmt.Errorf("unknown event from watch: %v", event)
		}

		if updateRV {
			accessor, err := meta.Accessor(event.Object)
			if err != nil {
				return fmt.Errorf("failed to get accessor for event: %w", err)
			}
			rv := accessor.GetResourceVersion()
			c.resumeRV = rv
		}
	}

	return fmt.Errorf("watch channel was closed")
}
