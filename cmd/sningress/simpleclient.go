package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
)

type SimpleSchema struct {
	scheme         *runtime.Scheme
	codecs         serializer.CodecFactory
	parameterCodec runtime.ParameterCodec
}

func NewSchema(funcs ...func(s *runtime.Scheme) error) (*SimpleSchema, error) {
	s := &SimpleSchema{}
	s.scheme = runtime.NewScheme()
	s.codecs = serializer.NewCodecFactory(s.scheme)
	s.parameterCodec = runtime.NewParameterCodec(s.scheme)

	for _, fn := range funcs {
		if err := fn(s.scheme); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type SimpleClientset struct {
	schema     *SimpleSchema
	config     *rest.Config
	httpClient *http.Client
}

// NewForConfig creates a new Clientset for the given config.
// If config's RateLimiter is not set and QPS and Burst are acceptable,
// NewForConfig will generate a rate-limiter in configShallowCopy.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(c *rest.Config, schema *SimpleSchema) (*SimpleClientset, error) {
	configShallowCopy := *c

	// share the transport between all clients
	httpClient, err := rest.HTTPClientFor(&configShallowCopy)
	if err != nil {
		return nil, err
	}

	return NewForConfigAndClient(&configShallowCopy, httpClient, schema)
}

// NewForConfigAndClient creates a new Clientset for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
// If config's RateLimiter is not set and QPS and Burst are acceptable,
// NewForConfigAndClient will generate a rate-limiter in configShallowCopy.
func NewForConfigAndClient(c *rest.Config, httpClient *http.Client, schema *SimpleSchema) (*SimpleClientset, error) {
	configShallowCopy := *c
	if configShallowCopy.RateLimiter == nil && configShallowCopy.QPS > 0 {
		if configShallowCopy.Burst <= 0 {
			return nil, fmt.Errorf("burst is required to be greater than 0 when RateLimiter is not set and QPS is set to greater than 0")
		}
		configShallowCopy.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(configShallowCopy.QPS, configShallowCopy.Burst)
	}

	return &SimpleClientset{
		schema:     schema,
		config:     &configShallowCopy,
		httpClient: httpClient,
	}, nil
}

// // New creates a new DiscoveryV1Client for the given RESTClient.
// func New(c rest.Interface) *DiscoveryV1Client {
// 	return &DiscoveryV1Client{c}
// }

func (c *SimpleClientset) Resource(gvr schema.GroupVersionResource) (*ResourceClient, error) {
	config := *c.config
	if err := c.schema.setConfigDefaults(&config, gvr); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientForConfigAndClient(&config, c.httpClient)
	if err != nil {
		return nil, err
	}
	return &ResourceClient{
		schema:     c.schema,
		restClient: client,
		gvr:        gvr,
	}, nil

}

type ResourceClient struct {
	schema     *SimpleSchema
	restClient *rest.RESTClient
	gvr        schema.GroupVersionResource
	namespace  string
}

func (c *ResourceClient) WithNamespace(ns string) *ResourceClient {
	return &ResourceClient{
		schema:     c.schema,
		restClient: c.restClient,
		gvr:        c.gvr,
		namespace:  ns,
	}
}

func (c *SimpleSchema) setConfigDefaults(config *rest.Config, gvr schema.GroupVersionResource) error {
	gv := gvr.GroupVersion()
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = c.codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// List takes label and field selectors, and returns the list of Secrets that match those selectors.
func (c *ResourceClient) List(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	// result = &v1.SecretList{}
	return c.restClient.Get().
		Namespace(c.namespace).
		Resource(c.gvr.Resource).
		VersionedParams(&opts, c.schema.parameterCodec).
		Timeout(timeout).
		Do(ctx).Get()
}

// Watch returns a watch.Interface that watches the requested endpointSlices.
func (c *ResourceClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.restClient.Get().
		Namespace(c.namespace).
		Resource(c.gvr.Resource).
		VersionedParams(&opts, c.schema.parameterCodec).
		Timeout(timeout).
		Watch(ctx)
}
