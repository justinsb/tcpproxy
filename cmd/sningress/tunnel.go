package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"inet.af/tcpproxy/pkg/proxy"
	"k8s.io/klog/v2"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client"
)

const (
	userAgent = "sningress-tunnel"
)

type tunnelBackend struct {
	proxyUdsName string
}

var _ proxy.Backend = &tunnelBackend{}

func (d *tunnelBackend) Dial(ctx context.Context, address string) (proxy.NetConn, error) {
	dialOption := grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		// Ignoring addr and timeout arguments:
		// addr - comes from the closure
		// timeout - is turned off as this is test code and eases debugging.
		klog.Infof("dialing %q", d.proxyUdsName)
		c, err := net.DialTimeout("unix", d.proxyUdsName, 0)
		if err != nil {
			klog.ErrorS(err, "failed to create connection to uds", "name", d.proxyUdsName)
		}
		return c, err
	})
	tunnel, err := client.CreateSingleUseGrpcTunnel(ctx, d.proxyUdsName, dialOption, grpc.WithInsecure(), grpc.WithUserAgent(userAgent))
	if err != nil {
		return nil, fmt.Errorf("failed to create tunnel %s: %w", d.proxyUdsName, err)
	}

	conn, err := tunnel.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %q: %w", address, err)
	}

	return &tunnelConnection{conn: conn}, nil
}

type tunnelConnection struct {
	conn net.Conn
}

var _ proxy.NetConn = &tunnelConnection{}

func (c *tunnelConnection) Close() error {
	return c.conn.Close()
}

func (c *tunnelConnection) CloseRead() error {
	klog.Warningf("CloseRead not implemented; mapping to Close")
	return c.conn.Close()
}

func (c *tunnelConnection) CloseWrite() error {
	klog.Warningf("CloseWrite not implemented; mapping to Close")
	return c.conn.Close()
}

func (c *tunnelConnection) Read(b []byte) (int, error) {
	return c.conn.Read(b)
}

func (c *tunnelConnection) Write(b []byte) (int, error) {
	return c.conn.Write(b)
}

func (c *tunnelConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *tunnelConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *tunnelConnection) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *tunnelConnection) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *tunnelConnection) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
