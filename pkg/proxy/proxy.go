package proxy

import (
	"fmt"
	"net"
	"time"
)

func New(config Config, helloTimeout time.Duration) *Proxy {
	if helloTimeout == 0 {
		helloTimeout = DefaultHelloTimeout
	}

	return &Proxy{
		config:       config,
		helloTimeout: helloTimeout,
	}
}

// Proxy routes connections to backends based on a Config.
type Proxy struct {
	config Config
	//l            net.Listener
	helloTimeout time.Duration
}

// Serve accepts connections from l and routes them according to TLS SNI.
func (p *Proxy) Serve(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accept new conn: %s", err)
		}

		conn := &Conn{
			TCPConn:      c.(*net.TCPConn),
			config:       p.config,
			helloTimeout: p.helloTimeout,
		}
		go conn.proxy()
	}
}

// ListenAndServe creates a listener on addr calls Serve on it.
func (p *Proxy) ListenAndServe(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("create listener: %s", err)
	}
	return p.Serve(l)
}
