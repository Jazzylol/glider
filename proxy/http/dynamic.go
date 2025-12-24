package http

import (
	"encoding/base64"
	"io"
	"net"
	"time"

	"github.com/nadoo/glider/pkg/log"
	"github.com/nadoo/glider/pkg/pool"
	"github.com/nadoo/glider/proxy"
)

// GetSxxAuthKey 由 main 包注入，用于获取 sxxKey（动态代理模式使用）
var GetSxxAuthKey func() string

// isDynamicProxyMode 检查是否为动态代理模式
// 条件：端口为 10800 且密码等于 sxxkey
func isDynamicProxyMode(serverAddr, password string) bool {
	if GetSxxAuthKey == nil {
		return false
	}
	_, port, err := net.SplitHostPort(serverAddr)
	if err != nil || port != "10800" {
		return false
	}
	sxxKey := GetSxxAuthKey()
	return sxxKey != "" && password == sxxKey
}

// servDynamic 处理动态代理请求
// base64User 是 Base64 URL Safe 无填充编码的真实代理 URL
func (s *HTTP) servDynamic(req *request, c *proxy.Conn, base64User string) {
	// Base64 URL Safe 无填充解码
	decoded, err := base64.RawURLEncoding.DecodeString(base64User)
	if err != nil {
		io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		log.F("[http-dynamic] decode error: %v", err)
		return
	}
	proxyURL := string(decoded)

	// 创建默认的 Direct dialer 作为 fallback
	defaultDialer, err := proxy.NewDirect("", 3*time.Second, 3*time.Second)
	if err != nil {
		io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		log.F("[http-dynamic] create default dialer error: %v", err)
		return
	}

	// 创建动态 Dialer
	dialer, err := proxy.DialerFromURL(proxyURL, defaultDialer)
	if err != nil {
		io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		log.F("[http-dynamic] create dialer error, base64=%s raw=%s err=%v", base64User, proxyURL, err)
		return
	}

	// 确定目标地址
	target := req.uri
	if req.method != "CONNECT" {
		target = req.target
	}

	// 建立连接
	rc, err := dialer.Dial("tcp", target)
	if err != nil {
		io.WriteString(c, req.proto+" 502 ERROR\r\n\r\n")
		log.F("[http-dynamic] %s <-> %s, dial error, base64=%s raw=%s err=%v", c.RemoteAddr(), target, base64User, proxyURL, err)
		return
	}
	defer rc.Close()

	startTime := time.Now()

	// HTTPS 隧道需要返回 200
	if req.method == "CONNECT" {
		io.WriteString(c, "HTTP/1.1 200 Connection established\r\n\r\n")
	} else {
		// HTTP 请求需要转发原始请求
		buf := pool.GetBytesBuffer()
		defer pool.PutBytesBuffer(buf)
		req.WriteBuf(buf)
		rc.Write(buf.Bytes())
	}

	log.F("[http-dynamic] %s <-> %s, base64=%s raw=%s", c.RemoteAddr(), target, base64User, proxyURL)

	// 双向转发并统计流量
	upBytes, downBytes, err := s.relayWithStats(c, rc)
	duration := time.Since(startTime)

	if err != nil {
		log.F("[http-dynamic] %s <-> %s, relay error, base64=%s raw=%s err=%v, duration=%.2fs, up=%.2fKB, down=%.2fKB",
			c.RemoteAddr(), target, base64User, proxyURL, err, duration.Seconds(), float64(upBytes)/1024, float64(downBytes)/1024)
	} else {
		log.F("[http-dynamic] %s <-> %s, base64=%s raw=%s, duration=%.2fs, up=%.2fKB, down=%.2fKB",
			c.RemoteAddr(), target, base64User, proxyURL, duration.Seconds(), float64(upBytes)/1024, float64(downBytes)/1024)
	}

	// 异步记录流量统计（不阻塞主流程）
	go safeRecordTraffic(target, upBytes, downBytes)
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

