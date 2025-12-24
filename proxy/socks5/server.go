package socks5

import (
	"encoding/base64"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/nadoo/glider/pkg/log"
	"github.com/nadoo/glider/pkg/pool"
	"github.com/nadoo/glider/pkg/socks"
	"github.com/nadoo/glider/proxy"
)

var nm sync.Map

// GetSxxAuthKey 由 main 包注入，用于获取 sxxKey（动态代理模式使用）
var GetSxxAuthKey func() string

func init() {
	proxy.RegisterServer("socks5", NewSocks5Server)
}

// NewSocks5Server returns a socks5 proxy server.
func NewSocks5Server(s string, p proxy.Proxy) (proxy.Server, error) {
	return NewSocks5(s, nil, p)
}

// ListenAndServe serves socks5 requests.
func (s *Socks5) ListenAndServe() {
	go s.ListenAndServeUDP()
	s.ListenAndServeTCP()
}

// ListenAndServeTCP listen and serve on tcp port.
func (s *Socks5) ListenAndServeTCP() {
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatalf("[socks5] failed to listen on %s: %v", s.addr, err)
		return
	}

	log.F("[socks5] listening TCP on %s", s.addr)

	for {
		c, err := l.Accept()
		if err != nil {
			log.F("[socks5] failed to accept: %v", err)
			continue
		}

		// IP whitelist check
		if len(s.ipAllowMap) > 0 {
			host, _, err := net.SplitHostPort(c.RemoteAddr().String())
			if err != nil || host == "" || !s.ipAllowMap[host] {
				log.F("[socks5] IP not allowed: %s", c.RemoteAddr())
				c.Close()
				continue
			}
		}

		go s.Serve(c)
	}
}

// Serve serves a connection.
func (s *Socks5) Serve(c net.Conn) {
	defer c.Close()

	if c, ok := c.(*net.TCPConn); ok {
		c.SetKeepAlive(true)
	}

	tgt, dynamicUser, err := s.handshake(c)
	if err != nil {
		// UDP: keep the connection until disconnect then free the UDP socket
		if err == socks.Errors[9] {
			buf := pool.GetBuffer(1)
			defer pool.PutBuffer(buf)
			// block here
			for {
				_, err := c.Read(buf)
				if err, ok := err.(net.Error); ok && err.Timeout() {
					continue
				}
				// log.F("[socks5] servetcp udp associate end")
				return
			}
		}

		log.F("[socks5] failed in handshake with %s: %v", c.RemoteAddr(), err)
		return
	}

	// 检查是否为动态代理模式
	if dynamicUser != "" {
		s.serveDynamic(c, tgt.String(), dynamicUser)
		return
	}

	rc, dialer, err := s.proxy.Dial("tcp", tgt.String())
	if err != nil {
		log.F("[socks5] %s <-> %s via %s, error in dial: %v", c.RemoteAddr(), tgt, dialer.Addr(), err)
		return
	}
	defer rc.Close()

	startTime := time.Now()
	log.F("[socks5] %s <-> %s via %s", c.RemoteAddr(), tgt, dialer.Addr())

	upBytes, downBytes, err := proxy.RelayWithStats(c, rc)
	duration := time.Since(startTime)

	if err != nil {
		log.F("[socks5] %s <-> %s via %s, relay error: %v, duration: %.2fs, up: %.2f KB, down: %.2f KB",
			c.RemoteAddr(), tgt, dialer.Addr(), err, duration.Seconds(), float64(upBytes)/1024, float64(downBytes)/1024)
		// record remote conn failure only
		if !strings.Contains(err.Error(), s.addr) {
			s.proxy.Record(dialer, false)
		}
	} else {
		log.F("[socks5] %s <-> %s via %s, duration: %.2fs, up: %.2f KB, down: %.2f KB",
			c.RemoteAddr(), tgt, dialer.Addr(), duration.Seconds(), float64(upBytes)/1024, float64(downBytes)/1024)
	}

	// 异步记录流量统计（不阻塞主流程）
	go safeRecordTraffic(tgt.String(), upBytes, downBytes)
}

// safeRecordTraffic 安全地记录流量，捕获任何 panic
func safeRecordTraffic(target string, upBytes, downBytes int64) {
	defer func() {
		if r := recover(); r != nil {
			log.F("[traffic] RecordTraffic panic recovered: %v", r)
		}
	}()
	proxy.RecordTraffic(target, upBytes, downBytes)
}

// isDynamicProxyMode 检查是否为动态代理模式
// 条件：端口为 10800 且密码等于 sxxkey
func (s *Socks5) isDynamicProxyMode(password string) bool {
	if GetSxxAuthKey == nil {
		return false
	}
	_, port, err := net.SplitHostPort(s.addr)
	if err != nil || port != "10800" {
		return false
	}
	sxxKey := GetSxxAuthKey()
	return sxxKey != "" && password == sxxKey
}

// serveDynamic 处理动态代理请求
func (s *Socks5) serveDynamic(c net.Conn, target, base64User string) {
	// Base64 URL Safe 无填充解码
	decoded, err := base64.RawURLEncoding.DecodeString(base64User)
	if err != nil {
		log.F("[socks5-dynamic] decode error: %v", err)
		return
	}
	proxyURL := string(decoded)

	// 创建默认的 Direct dialer 作为 fallback
	defaultDialer, err := proxy.NewDirect("", 3*time.Second, 3*time.Second)
	if err != nil {
		log.F("[socks5-dynamic] create default dialer error: %v", err)
		return
	}

	// 创建动态 Dialer
	dialer, err := proxy.DialerFromURL(proxyURL, defaultDialer)
	if err != nil {
		log.F("[socks5-dynamic] create dialer error: %s <-> %s <-> %s, err=%v", c.RemoteAddr(), base64User, proxyURL, err)
		return
	}

	// 建立连接
	rc, err := dialer.Dial("tcp", target)
	if err != nil {
		log.F("[socks5-dynamic] dial error: %s <-> %s <-> %s <-> %s, err=%v", c.RemoteAddr(), base64User, proxyURL, target, err)
		return
	}
	defer rc.Close()

	startTime := time.Now()
	log.F("[socks5-dynamic] %s <-> %s <-> %s <-> %s", c.RemoteAddr(), base64User, proxyURL, target)

	// 双向转发并统计流量
	upBytes, downBytes, err := proxy.RelayWithStats(c, rc)
	duration := time.Since(startTime)

	if err != nil {
		log.F("[socks5-dynamic] %s <-> %s <-> %s <-> %s, relay error: %v, duration=%.2fs, up=%.2fKB, down=%.2fKB",
			c.RemoteAddr(), base64User, proxyURL, target, err, duration.Seconds(), float64(upBytes)/1024, float64(downBytes)/1024)
	} else {
		log.F("[socks5-dynamic] %s <-> %s <-> %s <-> %s, duration=%.2fs, up=%.2fKB, down=%.2fKB",
			c.RemoteAddr(), base64User, proxyURL, target, duration.Seconds(), float64(upBytes)/1024, float64(downBytes)/1024)
	}

	// 异步记录流量统计（不阻塞主流程）
	go safeRecordTraffic(target, upBytes, downBytes)
}

// ListenAndServeUDP serves udp requests.
func (s *Socks5) ListenAndServeUDP() {
	lc, err := net.ListenPacket("udp", s.addr)
	if err != nil {
		log.Fatalf("[socks5] failed to listen on UDP %s: %v", s.addr, err)
		return
	}
	defer lc.Close()

	log.F("[socks5] listening UDP on %s", s.addr)

	s.ServePacket(lc)
}

// ServePacket implements proxy.PacketServer.
func (s *Socks5) ServePacket(pc net.PacketConn) {
	for {
		c := NewPktConn(pc, nil, nil, nil)
		buf := pool.GetBuffer(proxy.UDPBufSize)

		n, srcAddr, dstAddr, err := c.readFrom(buf)
		if err != nil {
			log.F("[socks5u] remote read error: %v", err)
			continue
		}

		var session *Session
		sessionKey := srcAddr.String()

		v, ok := nm.Load(sessionKey)
		if !ok || v == nil {
			session = newSession(sessionKey, srcAddr, dstAddr, c)
			nm.Store(sessionKey, session)
			go s.serveSession(session)
		} else {
			session = v.(*Session)
		}

		session.msgCh <- message{dstAddr, buf[:n]}
	}
}

func (s *Socks5) serveSession(session *Session) {
	dstPC, dialer, err := s.proxy.DialUDP("udp", session.srcPC.target.String())
	if err != nil {
		log.F("[socks5u] remote dial error: %v", err)
		nm.Delete(session.key)
		return
	}
	defer dstPC.Close()

	go func() {
		proxy.CopyUDP(session.srcPC, nil, dstPC, 2*time.Minute, 5*time.Second)
		nm.Delete(session.key)
		close(session.finCh)
	}()

	log.F("[socks5u] %s <-> %s via %s", session.src, session.srcPC.target, dialer.Addr())

	for {
		select {
		case msg := <-session.msgCh:
			_, err = dstPC.WriteTo(msg.msg, msg.dst)
			if err != nil {
				log.F("[socks5u] writeTo %s error: %v", msg.dst, err)
			}
			pool.PutBuffer(msg.msg)
			msg.msg = nil
		case <-session.finCh:
			return
		}
	}
}

type message struct {
	dst net.Addr
	msg []byte
}

// Session is a udp session
type Session struct {
	key   string
	src   net.Addr
	dst   net.Addr
	srcPC *PktConn
	msgCh chan message
	finCh chan struct{}
}

func newSession(key string, src, dst net.Addr, srcPC *PktConn) *Session {
	return &Session{key, src, dst, srcPC, make(chan message, 32), make(chan struct{})}
}

// Handshake fast-tracks SOCKS initialization to get target address to connect.
// Returns: target address, dynamic proxy user (base64 encoded proxy URL, empty if not dynamic mode), error
func (s *Socks5) handshake(c net.Conn) (socks.Addr, string, error) {
	// Read RFC 1928 for request and reply structure and sizes
	buf := pool.GetBuffer(socks.MaxAddrLen)
	defer pool.PutBuffer(buf)

	var dynamicUser string

	// read VER, NMETHODS, METHODS
	if _, err := io.ReadFull(c, buf[:2]); err != nil {
		return nil, "", err
	}

	nmethods := buf[1]
	if _, err := io.ReadFull(c, buf[:nmethods]); err != nil {
		return nil, "", err
	}

	// write VER METHOD
	if s.user != "" && s.password != "" {
		_, err := c.Write([]byte{Version, socks.AuthPassword})
		if err != nil {
			return nil, "", err
		}

		_, err = io.ReadFull(c, buf[:2])
		if err != nil {
			return nil, "", err
		}

		// Get username
		userLen := int(buf[1])
		if userLen <= 0 {
			c.Write([]byte{1, 1})
			return nil, "", errors.New("auth failed: wrong username length")
		}

		if _, err := io.ReadFull(c, buf[:userLen]); err != nil {
			return nil, "", errors.New("auth failed: cannot get username")
		}
		user := string(buf[:userLen])

		// Get password
		_, err = c.Read(buf[:1])
		if err != nil {
			return nil, "", errors.New("auth failed: cannot get password len")
		}

		passLen := int(buf[0])
		if passLen <= 0 {
			c.Write([]byte{1, 1})
			return nil, "", errors.New("auth failed: wrong password length")
		}

		_, err = io.ReadFull(c, buf[:passLen])
		if err != nil {
			return nil, "", errors.New("auth failed: cannot get password")
		}
		pass := string(buf[:passLen])

		// 检查是否为动态代理模式（密码匹配 sxxkey）
		if s.isDynamicProxyMode(pass) {
			dynamicUser = user
			// 动态代理模式，认证通过
			_, err = c.Write([]byte{1, 0})
			if err != nil {
				return nil, "", err
			}
		} else {
			// 普通认证模式
			if user != s.user || pass != s.password {
				_, err = c.Write([]byte{1, 1})
				if err != nil {
					return nil, "", err
				}
				return nil, "", errors.New("auth failed, authinfo: " + user + ":" + pass)
			}

			// Response auth state
			_, err = c.Write([]byte{1, 0})
			if err != nil {
				return nil, "", err
			}
		}

	} else if _, err := c.Write([]byte{Version, socks.AuthNone}); err != nil {
		return nil, "", err
	}

	// read VER CMD RSV ATYP DST.ADDR DST.PORT
	if _, err := io.ReadFull(c, buf[:3]); err != nil {
		return nil, "", err
	}
	cmd := buf[1]
	addr, err := socks.ReadAddr(c)
	if err != nil {
		return nil, "", err
	}
	switch cmd {
	case socks.CmdConnect:
		_, err = c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}) // SOCKS v5, reply succeeded
	case socks.CmdUDPAssociate:
		listenAddr := socks.ParseAddr(c.LocalAddr().String())
		if listenAddr == nil { // maybe it's unix socket
			listenAddr = socks.ParseAddr("127.0.0.1:0")
		}
		_, err = c.Write(append([]byte{5, 0, 0}, listenAddr...)) // SOCKS v5, reply succeeded
		if err != nil {
			return nil, "", socks.Errors[7]
		}
		err = socks.Errors[9]
	default:
		return nil, "", socks.Errors[7]
	}

	return addr, dynamicUser, err // skip VER, CMD, RSV fields
}
