package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"inet.af/tcpproxy/pkg/sni"
)

const DefaultHelloTimeout = 3 * time.Second

// A Conn handles the TLS proxying of one user connection.
type Conn struct {
	*net.TCPConn
	config Config

	helloTimeout time.Duration

	tlsMinor    int
	hostname    string
	backend     Backend
	backendConn NetConn
}

type Config interface {
	// Match returns the backend for hostname, and whether to use the PROXY protocol.
	Match(hostname string) (backend Backend, useProxyProtocol bool)
}

type Backend interface {
	Dial(hostname string) (NetConn, error)
}

type NetConn interface {
	io.Reader
	io.Writer
	io.Closer

	CloseRead() error
	CloseWrite() error
}

func (c *Conn) logf(msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	log.Printf("%s <> %s: %s", c.RemoteAddr(), c.LocalAddr(), msg)
}

func (c *Conn) abort(alert byte, msg string, args ...interface{}) {
	c.logf(msg, args...)
	alertMsg := []byte{21, 3, byte(c.tlsMinor), 0, 2, 2, alert}

	if err := c.SetWriteDeadline(time.Now().Add(c.helloTimeout)); err != nil {
		c.logf("error while setting write deadline during abort: %s", err)
		// Do NOT send the alert if we can't set a write deadline,
		// that could result in leaking a connection for an extended
		// period.
		return
	}

	if _, err := c.Write(alertMsg); err != nil {
		c.logf("error while sending alert: %s", err)
	}
}

func (c *Conn) internalError(msg string, args ...interface{}) { c.abort(80, msg, args...) }
func (c *Conn) sniFailed(msg string, args ...interface{})     { c.abort(112, msg, args...) }

func (c *Conn) proxy() {
	defer c.Close()

	if err := c.SetReadDeadline(time.Now().Add(c.helloTimeout)); err != nil {
		c.internalError("Setting read deadline for ClientHello: %s", err)
		return
	}

	var (
		err          error
		handshakeBuf bytes.Buffer
	)
	c.hostname, c.tlsMinor, err = sni.ExtractSNI(io.TeeReader(c, &handshakeBuf))
	if err != nil {
		c.internalError("Extracting SNI: %s", err)
		return
	}

	c.logf("extracted SNI %s", c.hostname)

	if err = c.SetReadDeadline(time.Time{}); err != nil {
		c.internalError("Clearing read deadline for ClientHello: %s", err)
		return
	}

	addProxyHeader := false
	c.backend, addProxyHeader = c.config.Match(c.hostname)
	if c.backend == nil {
		c.sniFailed("no backend found for %q", c.hostname)
		return
	}

	backendConn, err := c.backend.Dial(c.hostname)
	if err != nil {
		c.internalError("failed to dial backend %q for %q: %s", c.backend, c.hostname, err)
		return
	}
	defer backendConn.Close()

	c.backendConn = backendConn

	// If the backend supports the HAProxy PROXY protocol, give it the
	// real source information about the connection.
	if addProxyHeader {
		remote := c.TCPConn.RemoteAddr().(*net.TCPAddr)
		local := c.TCPConn.LocalAddr().(*net.TCPAddr)
		family := "TCP6"
		if remote.IP.To4() != nil {
			family = "TCP4"
		}
		if _, err := fmt.Fprintf(c.backendConn, "PROXY %s %s %s %d %d\r\n", family, remote.IP, local.IP, remote.Port, local.Port); err != nil {
			c.internalError("failed to send PROXY header to %q: %s", c.backend, err)
			return
		}
	}

	// Replay the piece of the handshake we had to read to do the
	// routing, then blindly proxy any other bytes.
	if _, err = io.Copy(c.backendConn, &handshakeBuf); err != nil {
		c.internalError("failed to replay handshake to %q: %s", c.backend, err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go proxy(&wg, c.TCPConn, c.backendConn)
	go proxy(&wg, c.backendConn, c.TCPConn)
	wg.Wait()
}

func proxy(wg *sync.WaitGroup, a, b NetConn) {
	defer wg.Done()
	atcp, btcp := a.(*net.TCPConn), b.(*net.TCPConn)
	if _, err := io.Copy(atcp, btcp); err != nil {
		log.Printf("%s<>%s -> %s<>%s: %s", atcp.RemoteAddr(), atcp.LocalAddr(), btcp.LocalAddr(), btcp.RemoteAddr(), err)
	}
	b.CloseWrite()
	a.CloseRead()
}
