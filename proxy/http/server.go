package http

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nadoo/glider/pkg/log"
	"github.com/nadoo/glider/pkg/pool"
	"github.com/nadoo/glider/proxy"
)

// NewHTTPServer returns a http proxy server.
func NewHTTPServer(s string, p proxy.Proxy) (proxy.Server, error) {
	return NewHTTP(s, nil, p)
}

// ListenAndServe listens on server's addr and serves connections.
func (s *HTTP) ListenAndServe() {
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatalf("[http] failed to listen on %s: %v", s.addr, err)
		return
	}
	defer l.Close()

	log.F("[http] listening TCP on %s", s.addr)

	for {
		c, err := l.Accept()
		if err != nil {
			log.F("[http] failed to accept: %v", err)
			continue
		}

		// IP whitelist check
		if len(s.ipAllowMap) > 0 {
			host, _, err := net.SplitHostPort(c.RemoteAddr().String())
			if err != nil || host == "" || !s.ipAllowMap[host] {
				log.F("[http] IP not allowed: %s", c.RemoteAddr())
				c.Close()
				continue
			}
		}

		go s.Serve(c)
	}
}

// Serve serves a connection.
func (s *HTTP) Serve(cc net.Conn) {
	if c, ok := cc.(*net.TCPConn); ok {
		c.SetKeepAlive(true)
	}

	c := proxy.NewConn(cc)
	defer c.Close()

	req, err := parseRequest(c.Reader())
	if err != nil {
		log.F("[http] can not parse request from %s, error: %v", c.RemoteAddr(), err)
		return
	}

	if s.pretend {
		fmt.Fprintf(c, "%s 404 Not Found\r\nServer: nginx\r\n\r\n404 Not Found\r\n", req.proto)
		log.F("[http] %s <-> %s, pretend as web server", c.RemoteAddr().String(), s.Addr())
		return
	}

	s.servRequest(req, c)
}

func (s *HTTP) servRequest(req *request, c *proxy.Conn) {
	// Auth
	if s.user != "" && s.password != "" {
		if user, pass, ok := extractUserPass(req.auth); !ok || user != s.user || pass != s.password {
			io.WriteString(c, "HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic\r\n\r\n")
			log.F("[http] auth failed from %s, auth info: %s:%s", c.RemoteAddr(), user, pass)
			return
		}
	}

	if req.method == "CONNECT" {
		s.servHTTPS(req, c)
		return
	}

	s.servHTTP(req, c)
}

func (s *HTTP) servHTTPS(r *request, c net.Conn) {
	rc, dialer, err := s.proxy.Dial("tcp", r.uri)
	if err != nil {
		io.WriteString(c, r.proto+" 502 ERROR\r\n\r\n")
		log.F("[http] %s <-> %s [c] via %s, error in dial: %v", c.RemoteAddr(), r.uri, dialer.Addr(), err)
		return
	}
	defer rc.Close()

	io.WriteString(c, "HTTP/1.1 200 Connection established\r\n\r\n")

	startTime := time.Now()
	log.F("[http] %s <-> %s [c] via %s", c.RemoteAddr(), r.uri, dialer.Addr())

	upBytes, downBytes, err := s.relayWithStats(c, rc)
	duration := time.Since(startTime)

	if err != nil {
		log.F("[http] %s <-> %s via %s, relay error: %v, duration: %v, up: %.2f KB, down: %.2f KB",
			c.RemoteAddr(), r.uri, dialer.Addr(), err, duration, float64(upBytes)/1024, float64(downBytes)/1024)
		// record remote conn failure only
		if !strings.Contains(err.Error(), s.addr) {
			s.proxy.Record(dialer, false)
		}
	} else {
		log.F("[http] %s <-> %s [c] via %s, duration: %v, up: %.2f KB, down: %.2f KB",
			c.RemoteAddr(), r.uri, dialer.Addr(), duration, float64(upBytes)/1024, float64(downBytes)/1024)
	}
}

func (s *HTTP) servHTTP(req *request, c *proxy.Conn) {
	rc, dialer, err := s.proxy.Dial("tcp", req.target)
	if err != nil {
		fmt.Fprintf(c, "%s 502 ERROR\r\n\r\n", req.proto)
		log.F("[http] %s <-> %s via %s, error in dial: %v", c.RemoteAddr(), req.target, dialer.Addr(), err)
		return
	}
	defer rc.Close()

	startTime := time.Now()
	var upBytes int64

	buf := pool.GetBytesBuffer()
	defer pool.PutBytesBuffer(buf)

	// send request to remote server
	req.WriteBuf(buf)
	_, err = rc.Write(buf.Bytes())
	if err != nil {
		return
	}

	// copy the left request bytes to remote server. eg. length specificed or chunked body.
	go func() {
		if _, err := c.Reader().Peek(1); err == nil {
			upBytes, _ = proxy.Copy(rc, c)
			rc.SetDeadline(time.Now())
			c.SetDeadline(time.Now())
		}
	}()

	r := pool.GetBufReader(rc)
	defer pool.PutBufReader(r)

	tpr := textproto.NewReader(r)
	line, err := tpr.ReadLine()
	if err != nil {
		return
	}

	proto, code, status, ok := parseStartLine(line)
	if !ok {
		return
	}

	header, err := tpr.ReadMIMEHeader()
	if err != nil {
		log.F("[http] read header error:%s", err)
		return
	}

	header.Set("Proxy-Connection", "close")
	header.Set("Connection", "close")

	buf.Reset()
	writeStartLine(buf, proto, code, status)
	writeHeaders(buf, header)

	c.Write(buf.Bytes())

	downBytes, _ := proxy.Copy(c, r)
	duration := time.Since(startTime)

	log.F("[http] %s <-> %s via %s, duration: %v, up: %.2f KB, down: %.2f KB",
		c.RemoteAddr(), req.target, dialer.Addr(), duration, float64(upBytes)/1024, float64(downBytes)/1024)
}

// relayWithStats relays between left and right and returns bytes transferred.
func (s *HTTP) relayWithStats(left, right net.Conn) (upBytes, downBytes int64, err error) {
	var err1 error
	var wg sync.WaitGroup
	var wait = 5 * time.Second

	wg.Add(1)
	go func() {
		defer wg.Done()
		downBytes, err1 = proxy.Copy(right, left)
		right.SetReadDeadline(time.Now().Add(wait)) // unblock read on right
	}()

	upBytes, err = proxy.Copy(left, right)
	left.SetReadDeadline(time.Now().Add(wait)) // unblock read on left
	wg.Wait()

	if err1 != nil && !errors.Is(err1, os.ErrDeadlineExceeded) {
		return upBytes, downBytes, err1
	}

	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		return upBytes, downBytes, err
	}

	return upBytes, downBytes, nil
}
