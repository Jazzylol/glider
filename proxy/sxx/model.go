package sxx

// ============= 请求参数结构体 =============

// PortListRequest 端口列表请求参数
type PortListRequest struct {
	APIKey      string `json:"apiKey"`
	Page        int    `json:"page"`
	PerPage     int    `json:"per_page"`
	ID          *int   `json:"id,omitempty"`
	Proxy       string `json:"proxy,omitempty"`
	CountryName string `json:"countryName,omitempty"`
	StateName   string `json:"stateName,omitempty"`
	CityName    string `json:"cityName,omitempty"`
	ASN         *int   `json:"asn,omitempty"`
	Status      *int   `json:"status,omitempty"`
	TemplateID  *int   `json:"template_id,omitempty"`
}

// CreateProxyRequest 创建代理请求参数
type CreateProxyRequest struct {
	CountryCode      string `json:"country_code"`
	ProxyTypeIDs     []int  `json:"proxy_type_id"`
	TypeID           int    `json:"type_id"`
	Name             string `json:"name,omitempty"`
	TTL              *int   `json:"ttl,omitempty"`
	ServerPortTypeID int    `json:"server_port_type_id"`
	Count            int    `json:"count"`
	TrafficLimit     int    `json:"traffic_limit"`
}

// ============= 响应结构体 =============

// Response 通用响应结构
type Response struct {
	Success bool        `json:"success"`
	Message interface{} `json:"message"`
}

// ProxyInfo 代理信息
type ProxyInfo struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Proxy        string `json:"proxy"`
	CountryCode  string `json:"country_code"`
	CountryName  string `json:"country_name"`
	StateName    string `json:"state_name"`
	CityName     string `json:"city_name"`
	ASN          int    `json:"asn"`
	Status       int    `json:"status"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	ExpiresAt    string `json:"expires_at"`
	TrafficUsed  int64  `json:"traffic_used"`
	TrafficLimit int64  `json:"traffic_limit"`
	TemplateID   *int   `json:"template_id"`
}

// ProxyListMessage 代理列表响应消息
type ProxyListMessage struct {
	Proxies      []ProxyInfo `json:"proxies"`
	CountProxies int         `json:"count_proxies"`
	CurrentPage  int         `json:"current_page"`
	LastPage     int         `json:"last_page"`
	PerPage      int         `json:"per_page"`
	Total        int         `json:"total"`
}

// ProxyListResponse 代理列表响应
type ProxyListResponse struct {
	Success bool             `json:"success"`
	Message ProxyListMessage `json:"message"`
}

// Country 国家信息
type Country struct {
	ID          int    `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// CountryListMessage 国家列表响应消息
type CountryListMessage struct {
	Countries []Country `json:"countries"`
}

// CountryListResponse 国家列表响应
type CountryListResponse struct {
	Success bool               `json:"success"`
	Message CountryListMessage `json:"message"`
}

// State 州/省信息
type State struct {
	ID          int    `json:"id"`
	CountryID   int    `json:"country_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// StateListMessage 州/省列表响应消息
type StateListMessage struct {
	States []State `json:"states"`
}

// StateListResponse 州/省列表响应
type StateListResponse struct {
	Success bool             `json:"success"`
	Message StateListMessage `json:"message"`
}

// City 城市信息
type City struct {
	ID          int    `json:"id"`
	StateID     int    `json:"state_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// CityListMessage 城市列表响应消息
type CityListMessage struct {
	Cities []City `json:"cities"`
}

// CityListResponse 城市列表响应
type CityListResponse struct {
	Success bool            `json:"success"`
	Message CityListMessage `json:"message"`
}

// ASNInfo ASN信息
type ASNInfo struct {
	ASN         int    `json:"asn"`
	DisplayName string `json:"display_name"`
	Name        string `json:"name"`
}

// ASNListMessage ASN列表响应消息
type ASNListMessage struct {
	ASNs []ASNInfo `json:"asns"`
}

// ASNListResponse ASN列表响应
type ASNListResponse struct {
	Success bool           `json:"success"`
	Message ASNListMessage `json:"message"`
}

// PlanInfo 计划信息（从 API 返回的原始数据）
type PlanInfo struct {
	ExpiredSeconds    float64           `json:"expiredSeconds"`
	ExpiredDate       string            `json:"expiredDate"`
	ExpiredTimestamp  int64             `json:"expiredTimestamp"`
	ElapsedDays       float64           `json:"elapsedDays"`
	Tariff            string            `json:"tariff"`
	TariffName        string            `json:"tariffName"`
	Traff             int64             `json:"traff"`
	URLs              map[string]string `json:"urls,omitempty"`
}

// PlanInfoResponse 计划信息响应
type PlanInfoResponse struct {
	Success bool     `json:"success"`
	Message PlanInfo `json:"message"`
}

// PlanInfoData 格式化后的计划信息（用于返回给客户端）
type PlanInfoData struct {
	Tariff           string            `json:"tariff"`
	TariffName       string            `json:"tariff_name"`
	TrafficLimit     int64             `json:"traffic_limit"`      // 流量限制（字节）
	TrafficUsed      int64             `json:"traffic_used"`       // 已使用流量
	TrafficRemaining int64             `json:"traffic_remaining"`  // 剩余流量
	ExpiresAt        string            `json:"expires_at"`         // 过期时间
	ExpiredSeconds   float64           `json:"expired_seconds"`    // 剩余秒数
	ElapsedDays      float64           `json:"elapsed_days"`       // 已使用天数
	URLs             map[string]string `json:"urls,omitempty"`     // 代理列表 URLs
}

// IPCheckResponse IP检查响应（用于代理测试）
type IPCheckResponse struct {
	Query       string `json:"query"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	City        string `json:"city"`
	ISP         string `json:"isp"`
	AS          string `json:"as"`
}

