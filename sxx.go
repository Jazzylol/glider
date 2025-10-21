package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/nadoo/glider/pkg/log"
	"github.com/nadoo/glider/proxy/sxx"
)

// SXXAPIResponse SXX API 统一响应结构
type SXXAPIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// CommonProxyInfo 通用代理信息
type CommonProxyInfo struct {
	Provider      string `json:"provider"`
	ProxyID       string `json:"proxyId"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	CountryCode   string `json:"countryCode"`
	CountryName   string `json:"countryName"`
	StateName     string `json:"stateName"`
	CityName      string `json:"cityName"`
	CityCode      string `json:"cityCode"`
	ProxyTypeID   int    `json:"proxyTypeId"`
	SpentTraffic  int64  `json:"spentTraffic"`
	TrafficLimit  int64  `json:"trafficLimit"`
	CreatedAt     string `json:"createdAt"`
	Status        int    `json:"status"`
	PlainText     string `json:"plainText"`
}

// ProxyCreateRequest 创建代理请求
type ProxyCreateRequest struct {
	APIKey           string `json:"apiKey"`
	CountryCode      string `json:"countryCode"`
	ProxyTypeIDs     []int  `json:"proxyTypeIds"`
	TypeID           int    `json:"typeId"`
	Name             string `json:"name,omitempty"`
	TTL              *int   `json:"ttl,omitempty"`
	ServerPortTypeID int    `json:"serverPortTypeId"`
	Count            int    `json:"count"`
	TrafficLimit     int    `json:"trafficLimit"`
}

// ProxyDeleteRequest 删除代理请求
type ProxyDeleteRequest struct {
	APIKey  string `json:"apiKey"`
	ProxyID string `json:"proxyId"`
}

// ProxyRefreshRequest 刷新代理请求
type ProxyRefreshRequest struct {
	APIKey  string `json:"apiKey"`
	ProxyID string `json:"proxyId"`
}

// ProxyTestRequest 测试代理请求
type ProxyTestRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ProxyListRequest 获取代理列表请求
type ProxyListRequest struct {
	APIKey string `json:"apiKey"`
}

// PlanInfoRequest 获取计划信息请求
type PlanInfoRequest struct {
	APIKey string `json:"apiKey"`
}

var (
	sxxClient *sxx.SXProxyClient
	sxxAuthKey string // SXX API鉴权密钥
)

// InitSXXAPI 初始化 SXX API（在 main.go 中调用）
// 如果 sxxHost 为空，则不初始化 SXX API（返回 error）
func InitSXXAPI(sxxHost, sxxKey string) error {
	// 如果没有配置 sxxHost，则返回错误
	if sxxHost == "" {
		log.F("[sxx] SXX API disabled: sxxhost not configured")
		return fmt.Errorf("sxxhost is required for SXX API, please set -sxxhost parameter")
	}
	
	// 创建客户端
	sxxClient = sxx.NewSXProxyClientWithHost(sxxHost)
	if sxxClient == nil {
		return fmt.Errorf("failed to create SXX client with host: %s", sxxHost)
	}
	
	sxxAuthKey = sxxKey
	
	log.F("[sxx] SXX API initialized with host: %s", sxxHost)
	
	if sxxKey != "" {
		log.F("[sxx] SXX API authentication enabled")
	} else {
		log.F("[sxx] SXX API authentication disabled (no sxxkey configured)")
	}
	
	return nil
}

// RegisterSXXAPIHandlers 注册 SXX API 路由
func RegisterSXXAPIHandlers(mux *http.ServeMux) {
	// 获取代理列表
	mux.HandleFunc("/api/sxproxy/list", authenticateSXX(handleSXXGetProxyList))

	// 创建代理
	mux.HandleFunc("/api/sxproxy/create", authenticateSXX(handleSXXCreateProxy))

	// 删除代理
	mux.HandleFunc("/api/sxproxy/delete", authenticateSXX(handleSXXDeleteProxy))

	// 刷新代理
	mux.HandleFunc("/api/sxproxy/refresh", authenticateSXX(handleSXXRefreshProxy))

	// 测试代理
	mux.HandleFunc("/api/sxproxy/test", authenticateSXX(handleSXXTestProxy))

	// 获取计划信息
	mux.HandleFunc("/api/sxproxy/plan", authenticateSXX(handleSXXGetPlanInfo))

	log.F("[sxx] SXX API handlers registered")
}

// authenticateSXX SXX API 鉴权中间件
func authenticateSXX(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 如果没有配置 sxxkey，则跳过鉴权
		if sxxAuthKey == "" {
			next(w, r)
			return
		}

		// 从请求中获取 sxxKey
		var sxxKey string
		
		// 优先从 Header 的 Authorization Bearer token 获取
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			// 支持 "Bearer token" 格式
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				sxxKey = authHeader[7:]
			} else {
				sxxKey = authHeader
			}
		}
		
		// 如果 Header 中没有，则从查询参数获取
		if sxxKey == "" {
			sxxKey = r.URL.Query().Get("sxxKey")
		}
		
		// 如果查询参数也没有，尝试从 POST Body 中获取（仅限 POST 请求）
		if sxxKey == "" && r.Method == http.MethodPost {
			var bodyMap map[string]interface{}
			// 读取 Body，但需要重新设置以便后续处理
			if err := json.NewDecoder(r.Body).Decode(&bodyMap); err == nil {
				if key, ok := bodyMap["sxxKey"].(string); ok {
					sxxKey = key
				}
				// 重新编码 Body 供后续使用
				bodyBytes, _ := json.Marshal(bodyMap)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// 验证 sxxKey
		if sxxKey != sxxAuthKey {
			log.F("[sxx] Authentication failed: invalid sxxKey")
			writeSXXResponse(w, http.StatusForbidden, SXXAPIResponse{
				Success: false,
				Message: "Authentication failed: invalid or missing sxxKey",
			})
			return
		}

		// 鉴权通过，继续处理
		next(w, r)
	}
}

// handleSXXGetProxyList 获取代理列表
func handleSXXGetProxyList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeSXXResponse(w, http.StatusMethodNotAllowed, SXXAPIResponse{
			Success: false,
			Message: "Method not allowed, use POST or GET",
		})
		return
	}

	var req ProxyListRequest

	// 支持 POST 和 GET 两种方式
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
				Success: false,
				Message: "Invalid request body: " + err.Error(),
			})
			return
		}
	} else {
		// GET 方式从查询参数获取
		req.APIKey = r.URL.Query().Get("apiKey")
	}

	if req.APIKey == "" {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "API Key不能为空",
		})
		return
	}

	// 调用 SXX 客户端获取代理列表
	response, err := sxxClient.GetAllPorts(req.APIKey)
	if err != nil {
		log.F("[sxx] GetProxyList error: %v", err)
		writeSXXResponse(w, http.StatusInternalServerError, SXXAPIResponse{
			Success: false,
			Message: "获取代理列表失败: " + err.Error(),
		})
		return
	}

	// 转换为通用代理格式
	commonProxies := make([]CommonProxyInfo, 0, len(response.Message.Proxies))
	for _, proxy := range response.Message.Proxies {
		commonProxy := convertToCommonProxy(proxy)
		commonProxies = append(commonProxies, commonProxy)
	}

	log.F("[sxx] GetProxyList success, count: %d", len(commonProxies))
	writeSXXResponse(w, http.StatusOK, SXXAPIResponse{
		Success: true,
		Message: "获取代理列表成功",
		Data:    commonProxies,
	})
}

// handleSXXCreateProxy 创建代理
func handleSXXCreateProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeSXXResponse(w, http.StatusMethodNotAllowed, SXXAPIResponse{
			Success: false,
			Message: "Method not allowed, use POST",
		})
		return
	}

	var req ProxyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// 参数校验
	if req.APIKey == "" {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "API Key不能为空",
		})
		return
	}

	if req.CountryCode == "" {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "国家代码不能为空",
		})
		return
	}

	if len(req.ProxyTypeIDs) == 0 {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "代理类型不能为空",
		})
		return
	}

	if req.TypeID == 0 {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "TypeID不能为空",
		})
		return
	}

	// 设置默认值
	if req.ServerPortTypeID == 0 {
		req.ServerPortTypeID = 0 // SHARED
	}
	if req.Count == 0 {
		req.Count = 1
	}
	if req.TrafficLimit == 0 {
		req.TrafficLimit = 10 // 默认10GB
	}

	// 调用 SXX 客户端创建代理
	createReq := sxx.CreateProxyRequest{
		CountryCode:      req.CountryCode,
		ProxyTypeIDs:     req.ProxyTypeIDs,
		TypeID:           req.TypeID,
		Name:             req.Name,
		TTL:              req.TTL,
		ServerPortTypeID: req.ServerPortTypeID,
		Count:            req.Count,
		TrafficLimit:     req.TrafficLimit,
	}

	err := sxxClient.CreateProxy(req.APIKey, createReq)
	if err != nil {
		log.F("[sxx] CreateProxy error: %v", err)
		writeSXXResponse(w, http.StatusInternalServerError, SXXAPIResponse{
			Success: false,
			Message: "创建代理失败: " + err.Error(),
		})
		return
	}

	log.F("[sxx] CreateProxy success")
	writeSXXResponse(w, http.StatusOK, SXXAPIResponse{
		Success: true,
		Message: "创建代理成功",
	})
}

// handleSXXDeleteProxy 删除代理
func handleSXXDeleteProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeSXXResponse(w, http.StatusMethodNotAllowed, SXXAPIResponse{
			Success: false,
			Message: "Method not allowed, use POST",
		})
		return
	}

	var req ProxyDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// 参数校验
	if req.APIKey == "" {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "API Key不能为空",
		})
		return
	}

	if req.ProxyID == "" {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "代理ID不能为空",
		})
		return
	}

	// 调用 SXX 客户端删除代理
	err := sxxClient.DeleteProxy(req.APIKey, req.ProxyID)
	if err != nil {
		log.F("[sxx] DeleteProxy error: %v", err)
		writeSXXResponse(w, http.StatusInternalServerError, SXXAPIResponse{
			Success: false,
			Message: "删除代理失败: " + err.Error(),
		})
		return
	}

	log.F("[sxx] DeleteProxy success, proxyId: %s", req.ProxyID)
	writeSXXResponse(w, http.StatusOK, SXXAPIResponse{
		Success: true,
		Message: "删除代理成功",
	})
}

// handleSXXRefreshProxy 刷新代理
func handleSXXRefreshProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeSXXResponse(w, http.StatusMethodNotAllowed, SXXAPIResponse{
			Success: false,
			Message: "Method not allowed, use POST",
		})
		return
	}

	var req ProxyRefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// 参数校验
	if req.APIKey == "" {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "API Key不能为空",
		})
		return
	}

	if req.ProxyID == "" {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "代理ID不能为空",
		})
		return
	}

	// 调用 SXX 客户端刷新代理
	err := sxxClient.RefreshProxy(req.APIKey, req.ProxyID)
	if err != nil {
		log.F("[sxx] RefreshProxy error: %v", err)
		writeSXXResponse(w, http.StatusInternalServerError, SXXAPIResponse{
			Success: false,
			Message: "刷新代理失败: " + err.Error(),
		})
		return
	}

	log.F("[sxx] RefreshProxy success, proxyId: %s", req.ProxyID)
	writeSXXResponse(w, http.StatusOK, SXXAPIResponse{
		Success: true,
		Message: "刷新代理成功",
	})
}

// handleSXXTestProxy 测试代理
func handleSXXTestProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeSXXResponse(w, http.StatusMethodNotAllowed, SXXAPIResponse{
			Success: false,
			Message: "Method not allowed, use POST",
		})
		return
	}

	var req ProxyTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// 参数校验
	if req.Host == "" {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "代理主机不能为空",
		})
		return
	}

	if req.Port == 0 {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "代理端口不能为空",
		})
		return
	}

	// 调用 SXX 客户端测试代理
	ip, err := sxxClient.TestProxy(req.Host, req.Port, req.Username, req.Password)
	if err != nil {
		log.F("[sxx] TestProxy error: %v", err)
		writeSXXResponse(w, http.StatusInternalServerError, SXXAPIResponse{
			Success: false,
			Message: "测试代理失败: " + err.Error(),
		})
		return
	}

	log.F("[sxx] TestProxy success, exit IP: %s", ip)
	writeSXXResponse(w, http.StatusOK, SXXAPIResponse{
		Success: true,
		Message: "测试代理成功",
		Data:    map[string]string{"exitIP": ip},
	})
}

// handleSXXGetPlanInfo 获取计划信息
func handleSXXGetPlanInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeSXXResponse(w, http.StatusMethodNotAllowed, SXXAPIResponse{
			Success: false,
			Message: "Method not allowed, use POST or GET",
		})
		return
	}

	var req PlanInfoRequest

	// 支持 POST 和 GET 两种方式
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
				Success: false,
				Message: "Invalid request body: " + err.Error(),
			})
			return
		}
	} else {
		// GET 方式从查询参数获取
		req.APIKey = r.URL.Query().Get("apiKey")
	}

	if req.APIKey == "" {
		writeSXXResponse(w, http.StatusBadRequest, SXXAPIResponse{
			Success: false,
			Message: "API Key不能为空",
		})
		return
	}

	// 调用 SXX 客户端获取计划信息
	response, err := sxxClient.GetPlanInfo(req.APIKey)
	if err != nil {
		log.F("[sxx] GetPlanInfo error: %v", err)
		writeSXXResponse(w, http.StatusInternalServerError, SXXAPIResponse{
			Success: false,
			Message: "获取计划信息失败: " + err.Error(),
		})
		return
	}

	log.F("[sxx] GetPlanInfo success")
	writeSXXResponse(w, http.StatusOK, SXXAPIResponse{
		Success: true,
		Message: "获取计划信息成功",
		Data:    response.Message.Plan,
	})
}

// convertToCommonProxy 转换 SXX 代理信息为通用格式
func convertToCommonProxy(proxy sxx.ProxyInfo) CommonProxyInfo {
	common := CommonProxyInfo{
		Provider:     "SX",
		ProxyID:      strconv.Itoa(proxy.ID),
		Name:         proxy.Name,
		Host:         proxy.Host,
		Port:         proxy.Port,
		Username:     proxy.Username,
		Password:     proxy.Password,
		CountryCode:  proxy.CountryCode,
		CountryName:  proxy.CountryName,
		StateName:    proxy.StateName,
		CityName:     proxy.CityName,
		SpentTraffic: proxy.TrafficUsed,
		TrafficLimit: proxy.TrafficLimit,
		CreatedAt:    proxy.CreatedAt,
		Status:       proxy.Status,
	}

	// 构建纯文本格式: host:port:username:password
	if common.Username != "" && common.Password != "" {
		common.PlainText = sxx.FormatProxyURL(common.Host, common.Port, common.Username, common.Password)
	} else {
		common.PlainText = sxx.FormatProxyURL(common.Host, common.Port, "", "")
	}

	return common
}

// writeSXXResponse 写入 SXX API 响应
func writeSXXResponse(w http.ResponseWriter, status int, response SXXAPIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.F("[sxx] failed to encode response: %v", err)
	}
}

