package proxy

import (
	"container/list"
	"sync"
	"time"

	"github.com/nadoo/glider/pkg/log"
)

// ========== 共享的 LRU Dialer 缓存 ==========

type dialerCacheEntry struct {
	key     string
	dialer  Dialer
	element *list.Element
}

type dialerLRUCache struct {
	mu       sync.Mutex
	capacity int
	cache    map[string]*dialerCacheEntry
	order    *list.List // front = 最近使用, back = 最久未使用
}

// 全局共享缓存（HTTP 和 SOCKS5 动态代理共用）
var (
	sharedDialerCache = newDialerLRUCache(1000) // 最多缓存 1000 个 Dialer
	sharedDirectDialer Dialer
	sharedDirectOnce   sync.Once
	sharedDirectError  error
)

func newDialerLRUCache(capacity int) *dialerLRUCache {
	return &dialerLRUCache{
		capacity: capacity,
		cache:    make(map[string]*dialerCacheEntry),
		order:    list.New(),
	}
}

func (c *dialerLRUCache) get(key string) (Dialer, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.cache[key]; ok {
		// 移到最前面（最近使用）
		c.order.MoveToFront(entry.element)
		return entry.dialer, true
	}
	return nil, false
}

func (c *dialerLRUCache) set(key string, dialer Dialer) {
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

// Size 返回缓存中的条目数量
func (c *dialerLRUCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// GetSharedDirectDialer 获取共享的 Direct dialer（只创建一次）
func GetSharedDirectDialer() (Dialer, error) {
	sharedDirectOnce.Do(func() {
		sharedDirectDialer, sharedDirectError = NewDirect("", 3*time.Second, 3*time.Second)
		if sharedDirectError == nil {
			log.F("[dialer-cache] shared direct dialer initialized")
		}
	})
	return sharedDirectDialer, sharedDirectError
}

// GetOrCreateDialer 从缓存获取或创建 Dialer（HTTP/SOCKS5 动态代理共用）
func GetOrCreateDialer(proxyURL string) (Dialer, error) {
	// 先从缓存获取
	if dialer, ok := sharedDialerCache.get(proxyURL); ok {
		return dialer, nil
	}

	// 获取共享的 Direct dialer
	directDialer, err := GetSharedDirectDialer()
	if err != nil {
		return nil, err
	}

	// 创建新的 Dialer
	dialer, err := DialerFromURL(proxyURL, directDialer)
	if err != nil {
		return nil, err
	}

	// 存入缓存
	sharedDialerCache.set(proxyURL, dialer)
	log.F("[dialer-cache] cached: %s (total: %d)", proxyURL, sharedDialerCache.Size())

	return dialer, nil
}

