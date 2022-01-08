package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"k8s.io/klog/v2"
)

type httpProxy struct {
	config *Config

	transport *http.Transport
}

func NewHTTPProxy(config *Config) (*httpProxy, error) {
	p := &httpProxy{
		config: config,
	}
	// based on http.DefaultTransport
	transport := &http.Transport{
		// Proxy: ProxyFromEnvironment,
		DialContext:           p.dialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	p.transport = transport

	return p, nil
}
func (p *httpProxy) ListenAndServe(listen string) error {
	server := &http.Server{
		Addr:    listen,
		Handler: p,
	}

	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("error serving on %q: %w", listen, err)
	}

	return nil
}

func replyError(w http.ResponseWriter, req *http.Request, err error) {
	klog.Infof("replying with error %v", err)
	code := http.StatusInternalServerError
	http.Error(w, http.StatusText(code), code)
}

func (p *httpProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Host == "" {
		replyError(w, req, fmt.Errorf("no host provided in request"))
		return
	}

	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		replyError(w, req, fmt.Errorf("cannot parse host %q: %w", req.Host, err))
		return
	}

	klog.Infof("http request: %s %s %s", req.Method, req.Host, req.URL)

	// TODO: reuse req?
	upstream := &http.Request{
		Method: req.Method,
		Host:   host,
		URL:    req.URL,
		Header: req.Header,

		// Proto:      "HTTP/1.1",
		// ProtoMajor: 1,
		// ProtoMinor: 1,

		Body: req.Body,
	}
	upstream.URL.Scheme = "http"
	upstream.URL.Host = host

	// TODO: Is any reuse safe?   (cookies etc)
	httpClient := &http.Client{
		Transport: p.transport,
	}

	resp, err := httpClient.Do(upstream)
	if err != nil {
		replyError(w, req, fmt.Errorf("failed to make request: %w", err))
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		w.Header()[k] = vv
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *httpProxy) dialContext(ctx context.Context, network string, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse host %q: %w", addr, err)
	}

	return p.config.Dial(ctx, host)
}
