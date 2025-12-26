package http

import (
	"encoding/base64"
	"errors"
	"net"
	"net/textproto"

	"github.com/nadoo/glider/pkg/log"
	"github.com/nadoo/glider/pkg/pool"
	"github.com/nadoo/glider/proxy"
)

// NewHTTPDialer returns a http proxy dialer.
func NewHTTPDialer(s string, d proxy.Dialer) (proxy.Dialer, error) {
	return NewHTTP(s, d, nil)
}

// Addr returns forwarder's address.
func (s *HTTP) Addr() string {
	if s.addr == "" {
		return s.dialer.Addr()
	}
	return s.addr
}

// Dial connects to the address addr on the network net via the proxy.
func (s *HTTP) Dial(network, addr string) (net.Conn, error) {
	rc, err := s.dialer.Dial(network, s.addr)
	if err != nil {
		log.F("[http] dial to %s error: %s", s.addr, err)
		return nil, err
	}

	buf := pool.GetBytesBuffer()
	defer pool.PutBytesBuffer(buf)

	buf.WriteString("CONNECT " + addr + " HTTP/1.1\r\n")
	buf.WriteString("Host: " + addr + "\r\n")
	buf.WriteString("Proxy-Connection: Keep-Alive\r\n")

	if s.user != "" && s.password != "" {
		auth := s.user + ":" + s.password
		buf.WriteString("Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(auth)) + "\r\n")
	}

	// header ended
	buf.WriteString("\r\n")
	_, err = rc.Write(buf.Bytes())
	if err != nil {
		return nil, err
	}

	c := proxy.NewConn(rc)
	tpr := textproto.NewReader(c.Reader())
	line, err := tpr.ReadLine()
	if err != nil {
		log.F("[http] read response from proxy %s failed: %v (target: %s)", s.addr, err, addr)
		return nil, errors.New("[http] proxy " + s.addr + " read response failed: " + err.Error())
	}

	_, code, msg, ok := parseStartLine(line)
	if ok && code == "200" {
		tpr.ReadMIMEHeader()
		return c, nil
	}

	// 记录详细错误信息
	errMsg := "[http] proxy " + s.addr + " returned " + code
	if msg != "" {
		errMsg += " (" + msg + ")"
	}
	errMsg += " for target " + addr

	switch code {
	case "403":
		log.F("[http] proxy %s returned 403 Forbidden - possibly blocked or auth failed (target: %s)", s.addr, addr)
	case "405":
		log.F("[http] proxy %s returned 405 - CONNECT method not allowed (target: %s)", s.addr, addr)
	case "407":
		log.F("[http] proxy %s returned 407 - authentication required (target: %s)", s.addr, addr)
	default:
		log.F("[http] proxy %s returned %s %s (target: %s)", s.addr, code, msg, addr)
	}

	return nil, errors.New(errMsg)
}

// DialUDP connects to the given address via the proxy.
func (s *HTTP) DialUDP(network, addr string) (pc net.PacketConn, err error) {
	return nil, proxy.ErrNotSupported
}
