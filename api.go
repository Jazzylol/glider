package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/nadoo/glider/pkg/log"
	"github.com/nadoo/glider/rule"
)

// APIManager 管理API模式的全局状态
type APIManager struct {
	mu           sync.RWMutex
	currentProxy *rule.Forwarder
	proxyList    []*rule.Forwarder
	rng          *rand.Rand
}

// 全局API管理器实例
var apiManager = &APIManager{
	rng: rand.New(rand.NewSource(time.Now().UnixNano())),
}

// GetAPIManager 获取全局API管理器
func GetAPIManager() *APIManager {
	return apiManager
}

// SetProxyList 设置可用的代理列表
func (am *APIManager) SetProxyList(proxies []*rule.Forwarder) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.proxyList = make([]*rule.Forwarder, len(proxies))
	copy(am.proxyList, proxies)
	log.F("[api] updated proxy list with %d proxies", len(proxies))
}

// GetCurrentProxy 获取当前选中的代理，如果为nil则随机选择一个
func (am *APIManager) GetCurrentProxy() *rule.Forwarder {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 如果当前代理为nil，随机选择一个
	if am.currentProxy == nil && len(am.proxyList) > 0 {
		idx := am.rng.Intn(len(am.proxyList))
		am.currentProxy = am.proxyList[idx]
		log.F("[api] auto selected proxy: %s", am.currentProxy.Addr())
	}

	return am.currentProxy
}

// ChangeProxy 随机切换到不同的代理
func (am *APIManager) ChangeProxy() (*rule.Forwarder, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if len(am.proxyList) == 0 {
		return nil, nil
	}

	// 如果只有一个代理，直接返回
	if len(am.proxyList) == 1 {
		am.currentProxy = am.proxyList[0]
		return am.currentProxy, nil
	}

	oldProxy := am.currentProxy
	
	// 最多尝试3次找到不同的代理
	for i := 0; i < 3; i++ {
		idx := am.rng.Intn(len(am.proxyList))
		newProxy := am.proxyList[idx]
		
		// 如果找到不同的代理，立即使用
		if oldProxy == nil || newProxy.Addr() != oldProxy.Addr() {
			am.currentProxy = newProxy
			log.F("[api] changed proxy from %v to %s", 
				func() string {
					if oldProxy != nil {
						return oldProxy.Addr()
					}
					return "nil"
				}(), newProxy.Addr())
			return am.currentProxy, nil
		}
	}
	
	// 如果3次都没找到不同的代理，使用最后一次的结果
	if len(am.proxyList) > 0 {
		idx := am.rng.Intn(len(am.proxyList))
		am.currentProxy = am.proxyList[idx]
		log.F("[api] changed proxy to %s (after 3 attempts)", am.currentProxy.Addr())
	}
	
	return am.currentProxy, nil
}

// ProxyInfo 代理信息结构
type ProxyInfo struct {
	Address  string `json:"address"`
	Priority uint32 `json:"priority"`
	Enabled  bool   `json:"enabled"`
	Latency  int64  `json:"latency"`
}

// APIResponse API响应结构
type APIResponse struct {
	Success      bool        `json:"success"`
	Message      string      `json:"message"`
	CurrentProxy *ProxyInfo  `json:"current_proxy,omitempty"`
	ProxyList    []ProxyInfo `json:"proxy_list,omitempty"`
}

// StartAPIServer 启动API服务器
func StartAPIServer(port string) {
	mux := http.NewServeMux()
	
	// 代理切换接口
	mux.HandleFunc("/api/proxy/change", handleProxyChange)
	
	// 获取当前代理信息接口
	mux.HandleFunc("/api/proxy/current", handleGetCurrent)
	
	// 获取所有代理列表接口
	mux.HandleFunc("/api/proxy/list", handleGetProxyList)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.F("[api] starting API server on port %s", port)
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.F("[api] API server error: %v", err)
		}
	}()
}

// handleProxyChange 处理代理切换请求
func handleProxyChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIResponse(w, http.StatusMethodNotAllowed, APIResponse{
			Success: false,
			Message: "Method not allowed, use POST",
		})
		return
	}

	newProxy, err := apiManager.ChangeProxy()
	if err != nil {
		writeAPIResponse(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "Failed to change proxy: " + err.Error(),
		})
		return
	}

	if newProxy == nil {
		writeAPIResponse(w, http.StatusNotFound, APIResponse{
			Success: false,
			Message: "No proxies available",
		})
		return
	}

	response := APIResponse{
		Success: true,
		Message: "Proxy changed successfully",
		CurrentProxy: &ProxyInfo{
			Address:  newProxy.Addr(),
			Priority: newProxy.Priority(),
			Enabled:  newProxy.Enabled(),
			Latency:  newProxy.Latency(),
		},
	}

	writeAPIResponse(w, http.StatusOK, response)
}

// handleGetCurrent 处理获取当前代理请求
func handleGetCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIResponse(w, http.StatusMethodNotAllowed, APIResponse{
			Success: false,
			Message: "Method not allowed, use GET",
		})
		return
	}

	currentProxy := apiManager.GetCurrentProxy()
	if currentProxy == nil {
		writeAPIResponse(w, http.StatusNotFound, APIResponse{
			Success: false,
			Message: "No current proxy set",
		})
		return
	}

	response := APIResponse{
		Success: true,
		Message: "Current proxy retrieved successfully",
		CurrentProxy: &ProxyInfo{
			Address:  currentProxy.Addr(),
			Priority: currentProxy.Priority(),
			Enabled:  currentProxy.Enabled(),
			Latency:  currentProxy.Latency(),
		},
	}

	writeAPIResponse(w, http.StatusOK, response)
}

// handleGetProxyList 处理获取代理列表请求
func handleGetProxyList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIResponse(w, http.StatusMethodNotAllowed, APIResponse{
			Success: false,
			Message: "Method not allowed, use GET",
		})
		return
	}

	apiManager.mu.RLock()
	proxyList := make([]ProxyInfo, len(apiManager.proxyList))
	for i, proxy := range apiManager.proxyList {
		proxyList[i] = ProxyInfo{
			Address:  proxy.Addr(),
			Priority: proxy.Priority(),
			Enabled:  proxy.Enabled(),
			Latency:  proxy.Latency(),
		}
	}
	apiManager.mu.RUnlock()

	response := APIResponse{
		Success:   true,
		Message:   "Proxy list retrieved successfully",
		ProxyList: proxyList,
	}

	writeAPIResponse(w, http.StatusOK, response)
}

// writeAPIResponse 写入API响应
func writeAPIResponse(w http.ResponseWriter, status int, response APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.F("[api] failed to encode response: %v", err)
	}
}
