package http

import (
	"container/list"
	"encoding/base64"
	"io"
	"net"
	"sync"
	"time"

	"github.com/nadoo/glider/pkg/log"
	"github.com/nadoo/glider/pkg/pool"
	"github.com/nadoo/glider/proxy"
)

// ========== LRU Dialer 缓存 ==========

type dialerCacheEntry struct {
	key     string
	dialer  proxy.Dialer
	element *list.Element
}

type dialerLRUCache struct {
	mu       sync.Mutex
	capacity int
	cache    map[string]*dialerCacheEntry
	order    *list.List // front = 最近使用, back = 最久未使用
}

// 全局缓存和共享的 defaultDialer
var (
	dialerCache       = newDialerLRUCache(1000) // 最多缓存 1000 个 Dialer
	sharedDialer      proxy.Dialer
	sharedDialerOnce  sync.Once
	sharedDialerError error
)

func newDialerLRUCache(capacity int) *dialerLRUCache {
	return &dialerLRUCache{
		capacity: capacity,
		cache:    make(map[string]*dialerCacheEntry),
		order:    list.New(),
	}
}

func (c *dialerLRUCache) get(key string) (proxy.Dialer, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.cache[key]; ok {
		// 移到最前面（最近使用）
		c.order.MoveToFront(entry.element)
		return entry.dialer, true
	}
	return nil, false
}

func (c *dialerLRUCache) set(key string, dialer proxy.Dialer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，更新并移到最前
	if entry, ok := c.cache[key]; ok {
		entry.dialer = dialer
		c.order.MoveToFront(entry.element)
		return
	}

	// 如果满了，删除最久未使用的
	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			oldEntry := oldest.Value.(*dialerCacheEntry)
			delete(c.cache, oldEntry.key)
			c.order.Remove(oldest)
		}
	}

	// 添加新条目
	entry := &dialerCacheEntry{key: key, dialer: dialer}
	entry.element = c.order.PushFront(entry)
	c.cache[key] = entry
}

// ========== 动态代理逻辑 ==========

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

// getSharedDialer 获取共享的 Direct dialer（只创建一次）
func getSharedDialer() (proxy.Dialer, error) {
	sharedDialerOnce.Do(func() {
		sharedDialer, sharedDialerError = proxy.NewDirect("", 3*time.Second, 3*time.Second)
		if sharedDialerError == nil {
			log.F("[http-dynamic] shared default dialer initialized")
		}
	})
	return sharedDialer, sharedDialerError
}

// getOrCreateDialer 从缓存获取或创建 Dialer
func getOrCreateDialer(proxyURL string) (proxy.Dialer, error) {
	// 先从缓存获取
	if dialer, ok := dialerCache.get(proxyURL); ok {
		return dialer, nil
	}

	// 获取共享的 defaultDialer
	defaultDialer, err := getSharedDialer()
	if err != nil {
		return nil, err
	}

	// 创建新的 Dialer
	dialer, err := proxy.DialerFromURL(proxyURL, defaultDialer)
	if err != nil {
		return nil, err
	}

	// 存入缓存
	dialerCache.set(proxyURL, dialer)
	log.F("[http-dynamic] dialer cached: %s", proxyURL)

	return dialer, nil
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

	// 从缓存获取或创建 Dialer
	dialer, err := getOrCreateDialer(proxyURL)
	if err != nil {
		io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		log.F("[http-dynamic] get dialer error: %s <-> %s <-> %s, err=%v", c.RemoteAddr(), base64User, proxyURL, err)
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
		log.F("[http-dynamic] dial error: %s <-> %s <-> %s <-> %s, err=%v", c.RemoteAddr(), base64User, proxyURL, target, err)
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

	log.F("[http-dynamic] %s <-> %s <-> %s <-> %s", c.RemoteAddr(), base64User, proxyURL, target)

	// 双向转发并统计流量
	upBytes, downBytes, err := s.relayWithStats(c, rc)
	duration := time.Since(startTime)

	if err != nil {
		log.F("[http-dynamic] %s <-> %s <-> %s <-> %s, relay error: %v, duration=%.2fs, up=%.2fKB, down=%.2fKB",
			c.RemoteAddr(), base64User, proxyURL, target, err, duration.Seconds(), float64(upBytes)/1024, float64(downBytes)/1024)
	} else {
		log.F("[http-dynamic] %s <-> %s <-> %s <-> %s, duration=%.2fs, up=%.2fKB, down=%.2fKB",
			c.RemoteAddr(), base64User, proxyURL, target, duration.Seconds(), float64(upBytes)/1024, float64(downBytes)/1024)
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
