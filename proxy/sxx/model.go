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

// ProxyInfo 代理信息（从 SX API 接收，保持原始 API 的命名格式）
type ProxyInfo struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Proxy        string `json:"proxy"`          // 格式: host:port
	Template     string `json:"template"`       // 完整代理URL模板
	Login        string `json:"login"`          // 用户名
	Password     string `json:"password"`       // 密码
	CountryCode  string `json:"countryCode"`    // 国家代码
	CountryName  string `json:"countryName"`    // 国家名称
	StateName    string `json:"stateName"`      // 州/省名称
	CityName     string `json:"cityName"`       // 城市名称
	ASN          int    `json:"asn"`            // ASN
	Status       int    `json:"status"`         // 状态
	ProxyTypeID  int    `json:"proxy_type_id"`  // 代理类型ID（保持原始API格式）
	CreatedAt    string `json:"created_at"`     // 创建时间（保持原始API格式）
	TrafficUsed  int64  `json:"spent_traffic_current_month"`  // 本月已用流量（保持原始API格式）
	TrafficLimit int64  `json:"traffic_limit"`  // 流量限制（保持原始API格式）
	TemplateID   *int   `json:"template_id"`    // 模板ID（保持原始API格式）
}

// ProxyListMessage 代理列表响应消息（从 SX API 接收，保持原始 API 的命名格式）
type ProxyListMessage struct {
	Proxies      []ProxyInfo `json:"proxies"`
	CountProxies int         `json:"countProxies"`  // 这个字段API实际返回是驼峰
	Pagination   struct {
		Page       int `json:"page"`
		PageCount  int `json:"pageCount"`
		PageSize   int `json:"pageSize"`
		TotalCount int `json:"totalCount"`
	} `json:"pagination,omitempty"`
}

// ProxyListResponse 代理列表响应
type ProxyListResponse struct {
	Success bool             `json:"success"`
	Message ProxyListMessage `json:"message"`
}

// Country 国家信息（从 SX API 接收，保持原始 API 的命名格式）
type Country struct {
	ID          int    `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`  // 保持原始API格式
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

// State 州/省信息（从 SX API 接收，保持原始 API 的命名格式）
type State struct {
	ID          int    `json:"id"`
	CountryID   int    `json:"country_id"`    // 保持原始API格式
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`  // 保持原始API格式
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

// City 城市信息（从 SX API 接收，保持原始 API 的命名格式）
type City struct {
	ID          int    `json:"id"`
	StateID     int    `json:"state_id"`      // 保持原始API格式
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`  // 保持原始API格式
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

// ASNInfo ASN信息（从 SX API 接收，保持原始 API 的命名格式）
type ASNInfo struct {
	ASN         int    `json:"asn"`
	DisplayName string `json:"display_name"`  // 保持原始API格式
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

// PlanInfoData 格式化后的计划信息（用于返回给客户端，使用驼峰命名）
type PlanInfoData struct {
	Tariff           string            `json:"tariff"`
	TariffName       string            `json:"tariffName"`        // 对外API使用驼峰
	TrafficLimit     int64             `json:"trafficLimit"`      // 对外API使用驼峰
	TrafficUsed      int64             `json:"trafficUsed"`       // 对外API使用驼峰
	TrafficRemaining int64             `json:"trafficRemaining"`  // 对外API使用驼峰
	ExpiresAt        string            `json:"expiresAt"`         // 对外API使用驼峰
	ExpiredSeconds   float64           `json:"expiredSeconds"`    // 对外API使用驼峰
	ElapsedDays      float64           `json:"elapsedDays"`       // 对外API使用驼峰
	URLs             map[string]string `json:"urls,omitempty"`    // 代理列表 URLs
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

