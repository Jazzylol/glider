package sxx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/nadoo/glider/pkg/log"
)

// SXProxyClient SX Proxy API客户端
// 封装对SX Proxy API的调用
type SXProxyClient struct {
	BaseURL string
	Client  *http.Client
}

// 如果 baseURL 为空，返回 nil
func NewSXProxyClientWithHost(baseURL string) *SXProxyClient {
	if baseURL == "" {
		return nil
	}
	return &SXProxyClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ============= 内部辅助方法 =============

// doRequest 执行HTTP请求并返回响应体
func (c *SXProxyClient) doRequest(method, url string, body io.Reader, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	return respBody, nil
}

// doGet 执行GET请求
func (c *SXProxyClient) doGet(endpoint string, params url.Values) ([]byte, error) {
	requestURL := fmt.Sprintf("%s%s?%s", c.BaseURL, endpoint, params.Encode())
	return c.doRequest("GET", requestURL, nil, nil)
}

// doPost 执行POST请求（JSON）
func (c *SXProxyClient) doPost(endpoint string, params url.Values, body interface{}) ([]byte, error) {
	requestURL := fmt.Sprintf("%s%s?%s", c.BaseURL, endpoint, params.Encode())

	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("JSON序列化错误: %v", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	return c.doRequest("POST", requestURL, bytes.NewBuffer(jsonData), headers)
}

// doDelete 执行DELETE请求
func (c *SXProxyClient) doDelete(endpoint string, params url.Values) ([]byte, error) {
	requestURL := fmt.Sprintf("%s%s?%s", c.BaseURL, endpoint, params.Encode())
	return c.doRequest("DELETE", requestURL, nil, nil)
}

// parseResponse 解析响应并检查成功状态
func parseResponse(body []byte, result interface{}) error {
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("JSON解析错误: %v", err)
	}
	return nil
}

// checkSuccess 检查通用响应是否成功
func checkSuccess(resp Response) error {
	if !resp.Success {
		errMsg := "API调用失败"
		if resp.Message != nil {
			if msg, ok := resp.Message.(string); ok && msg != "" {
				errMsg = msg
			}
		}
		return fmt.Errorf(errMsg)
	}
	return nil
}

// ============= API方法实现 =============

// GetPortList 获取端口列表
// GET /v2/proxy/ports?apiKey={apiKey}&page={page}&per_page={per_page}
func (c *SXProxyClient) GetPortList(req PortListRequest) (*ProxyListResponse, error) {
	if req.APIKey == "" {
		return nil, fmt.Errorf("API Key不能为空")
	}

	// 设置默认分页参数
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PerPage == 0 {
		req.PerPage = 100
	}

	// 构建查询参数
	params := url.Values{}
	params.Set("apiKey", req.APIKey)
	params.Set("page", strconv.Itoa(req.Page))
	params.Set("per_page", strconv.Itoa(req.PerPage))

	if req.ID != nil {
		params.Set("id", strconv.Itoa(*req.ID))
	}
	if req.Proxy != "" {
		params.Set("proxy", req.Proxy)
	}
	if req.CountryName != "" {
		params.Set("countryName", req.CountryName)
	}
	if req.StateName != "" {
		params.Set("stateName", req.StateName)
	}
	if req.CityName != "" {
		params.Set("cityName", req.CityName)
	}
	if req.ASN != nil {
		params.Set("asn", strconv.Itoa(*req.ASN))
	}
	if req.Status != nil {
		params.Set("status", strconv.Itoa(*req.Status))
	}
	if req.TemplateID != nil {
		params.Set("template_id", strconv.Itoa(*req.TemplateID))
	}

	// 发送请求
	body, err := c.doGet("/v2/proxy/ports", params)
	if err != nil {
		log.F("[SXProxyClient] GetPortList error: %v", err)
		return nil, err
	}

	log.F("[SXProxyClient] GetPortList response: %s", string(body))

	// 解析响应
	var result ProxyListResponse
	if err := parseResponse(body, &result); err != nil {
		log.F("[SXProxyClient] GetPortList parse error: %v", err)
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("API调用失败")
	}

	return &result, nil
}

// GetAllPorts 获取所有端口列表（无筛选条件）
func (c *SXProxyClient) GetAllPorts(apiKey string) (*ProxyListResponse, error) {
	return c.GetPortList(PortListRequest{
		APIKey:  apiKey,
		Page:    1,
		PerPage: 100,
	})
}

// GetPortByID 根据ID获取特定端口
func (c *SXProxyClient) GetPortByID(apiKey string, id int) (*ProxyListResponse, error) {
	return c.GetPortList(PortListRequest{
		APIKey:  apiKey,
		Page:    1,
		PerPage: 100,
		ID:      &id,
	})
}

// GetCountries 获取国家列表
// GET /v2/dir/countries?apiKey={apiKey}
func (c *SXProxyClient) GetCountries(apiKey string) (*CountryListResponse, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API Key不能为空")
	}

	params := url.Values{}
	params.Set("apiKey", apiKey)

	body, err := c.doGet("/v2/dir/countries", params)
	if err != nil {
		log.F("[SXProxyClient] GetCountries error: %v", err)
		return nil, err
	}

	log.F("[SXProxyClient] GetCountries response: %s", string(body))

	var result CountryListResponse
	if err := parseResponse(body, &result); err != nil {
		log.F("[SXProxyClient] GetCountries parse error: %v", err)
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("API调用失败")
	}

	return &result, nil
}

// GetStates 获取州/省列表
// GET /v2/dir/states?apiKey={apiKey}&countryId={countryId}
func (c *SXProxyClient) GetStates(apiKey string, countryID int) (*StateListResponse, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API Key不能为空")
	}
	if countryID == 0 {
		return nil, fmt.Errorf("国家ID不能为空")
	}

	params := url.Values{}
	params.Set("apiKey", apiKey)
	params.Set("countryId", strconv.Itoa(countryID))

	body, err := c.doGet("/v2/dir/states", params)
	if err != nil {
		log.F("[SXProxyClient] GetStates error: %v", err)
		return nil, err
	}

	log.F("[SXProxyClient] GetStates response: %s", string(body))

	var result StateListResponse
	if err := parseResponse(body, &result); err != nil {
		log.F("[SXProxyClient] GetStates parse error: %v", err)
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("API调用失败")
	}

	return &result, nil
}

// GetCities 获取城市列表
// GET /v2/dir/cities?apiKey={apiKey}&countryId={countryId}&stateId={stateId}
func (c *SXProxyClient) GetCities(apiKey string, countryID, stateID int) (*CityListResponse, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API Key不能为空")
	}
	if countryID == 0 {
		return nil, fmt.Errorf("国家ID不能为空")
	}
	if stateID == 0 {
		return nil, fmt.Errorf("州/省ID不能为空")
	}

	params := url.Values{}
	params.Set("apiKey", apiKey)
	params.Set("countryId", strconv.Itoa(countryID))
	params.Set("stateId", strconv.Itoa(stateID))

	body, err := c.doGet("/v2/dir/cities", params)
	if err != nil {
		log.F("[SXProxyClient] GetCities error: %v", err)
		return nil, err
	}

	log.F("[SXProxyClient] GetCities response: %s", string(body))

	var result CityListResponse
	if err := parseResponse(body, &result); err != nil {
		log.F("[SXProxyClient] GetCities parse error: %v", err)
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("API调用失败")
	}

	return &result, nil
}

// GetASNs 获取ASN列表
// GET /v2/dir/asns?apiKey={apiKey}&countryId={countryId}&stateId={stateId}&cityId={cityId}
func (c *SXProxyClient) GetASNs(apiKey string, countryID, stateID, cityID int) (*ASNListResponse, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API Key不能为空")
	}
	if countryID == 0 {
		return nil, fmt.Errorf("国家ID不能为空")
	}
	if stateID == 0 {
		return nil, fmt.Errorf("州/省ID不能为空")
	}
	if cityID == 0 {
		return nil, fmt.Errorf("城市ID不能为空")
	}

	params := url.Values{}
	params.Set("apiKey", apiKey)
	params.Set("countryId", strconv.Itoa(countryID))
	params.Set("stateId", strconv.Itoa(stateID))
	params.Set("cityId", strconv.Itoa(cityID))

	body, err := c.doGet("/v2/dir/asns", params)
	if err != nil {
		log.F("[SXProxyClient] GetASNs error: %v", err)
		return nil, err
	}

	log.F("[SXProxyClient] GetASNs response: %s", string(body))

	var result ASNListResponse
	if err := parseResponse(body, &result); err != nil {
		log.F("[SXProxyClient] GetASNs parse error: %v", err)
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("API调用失败")
	}

	return &result, nil
}

// CreateProxy 创建代理
// POST /v2/proxy/create-port?apiKey={apiKey}
func (c *SXProxyClient) CreateProxy(apiKey string, req CreateProxyRequest) error {
	if apiKey == "" {
		return fmt.Errorf("API Key不能为空")
	}
	if req.CountryCode == "" {
		return fmt.Errorf("国家代码不能为空")
	}
	if len(req.ProxyTypeIDs) == 0 {
		return fmt.Errorf("代理类型不能为空")
	}

	params := url.Values{}
	params.Set("apiKey", apiKey)

	body, err := c.doPost("/v2/proxy/create-port", params, req)
	if err != nil {
		log.F("[SXProxyClient] CreateProxy error: %v", err)
		return err
	}

	log.F("[SXProxyClient] CreateProxy response: %s", string(body))

	var result Response
	if err := parseResponse(body, &result); err != nil {
		log.F("[SXProxyClient] CreateProxy parse error: %v", err)
		return err
	}

	return checkSuccess(result)
}

// DeleteProxy 删除代理
// DELETE /v2/proxy/delete-port?apiKey={apiKey}&id={proxyId}
func (c *SXProxyClient) DeleteProxy(apiKey, proxyID string) error {
	if apiKey == "" {
		return fmt.Errorf("API Key不能为空")
	}
	if proxyID == "" {
		return fmt.Errorf("代理ID不能为空")
	}

	params := url.Values{}
	params.Set("apiKey", apiKey)
	params.Set("id", proxyID)

	body, err := c.doDelete("/v2/proxy/delete-port", params)
	if err != nil {
		log.F("[SXProxyClient] DeleteProxy error: %v", err)
		return err
	}

	log.F("[SXProxyClient] DeleteProxy response: %s", string(body))

	var result Response
	if err := parseResponse(body, &result); err != nil {
		log.F("[SXProxyClient] DeleteProxy parse error: %v", err)
		return err
	}

	return checkSuccess(result)
}

// RefreshProxy 刷新代理
// GET /v2/proxy/refresh/{portId}?apiKey={apiKey}
func (c *SXProxyClient) RefreshProxy(apiKey, proxyID string) error {
	if apiKey == "" {
		return fmt.Errorf("API Key不能为空")
	}
	if proxyID == "" {
		return fmt.Errorf("代理ID不能为空")
	}

	params := url.Values{}
	params.Set("apiKey", apiKey)

	endpoint := fmt.Sprintf("/v2/proxy/refresh/%s", proxyID)
	body, err := c.doGet(endpoint, params)
	if err != nil {
		log.F("[SXProxyClient] RefreshProxy error: %v", err)
		return err
	}

	log.F("[SXProxyClient] RefreshProxy response: %s", string(body))

	var result Response
	if err := parseResponse(body, &result); err != nil {
		log.F("[SXProxyClient] RefreshProxy parse error: %v", err)
		return err
	}

	return checkSuccess(result)
}

// TestProxy 测试代理
// 使用代理访问 i.pn 获取出口 IP
func (c *SXProxyClient) TestProxy(host string, port int, username, password string) (string, error) {
	if host == "" {
		return "", fmt.Errorf("代理主机不能为空")
	}
	if port == 0 {
		return "", fmt.Errorf("代理端口不能为空")
	}

	// 构建代理URL
	proxyURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", host, port),
	}

	if username != "" || password != "" {
		proxyURL.User = url.UserPassword(username, password)
	}

	// 创建带代理的HTTP客户端
	proxyClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	// 访问测试URL
	testURL := "https://i.pn"
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		log.F("[SXProxyClient] TestProxy create request error: %v", err)
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("User-Agent", "curl/8.7.1")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Host", "i.pn")

	// 发送请求
	resp, err := proxyClient.Do(req)
	if err != nil {
		log.F("[SXProxyClient] TestProxy request error: %v", err)
		return "", fmt.Errorf("代理测试失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.F("[SXProxyClient] TestProxy read response error: %v", err)
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 清除 ANSI 转义序列（颜色代码）
	ansiRegex := regexp.MustCompile(`\x1B\[[0-9;]*m`)
	cleanedBody := ansiRegex.ReplaceAllString(string(body), "")

	log.F("[SXProxyClient] TestProxy response: %s", cleanedBody)

	// 解析 JSON 响应
	var result IPCheckResponse
	if err := json.Unmarshal([]byte(cleanedBody), &result); err != nil {
		log.F("[SXProxyClient] TestProxy JSON parse error: %v", err)
		return "", fmt.Errorf("JSON解析失败: %v", err)
	}

	if result.Query == "" {
		return "", fmt.Errorf("未获取到IP地址")
	}

	// 验证IP地址格式
	ipRegex := regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
	if !ipRegex.MatchString(result.Query) {
		return "", fmt.Errorf("IP格式错误: %s", result.Query)
	}

	return result.Query, nil
}

// GetPlanInfo 获取计划信息
// GET /v2/plan/info?apiKey={apiKey}
func (c *SXProxyClient) GetPlanInfo(apiKey string) (*PlanInfoResponse, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API Key不能为空")
	}

	params := url.Values{}
	params.Set("apiKey", apiKey)

	body, err := c.doGet("/v2/plan/info", params)
	if err != nil {
		log.F("[SXProxyClient] GetPlanInfo error: %v", err)
		return nil, err
	}

	log.F("[SXProxyClient] GetPlanInfo response: %s", string(body))

	var result PlanInfoResponse
	if err := parseResponse(body, &result); err != nil {
		log.F("[SXProxyClient] GetPlanInfo parse error: %v", err)
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("获取计划信息失败")
	}

	return &result, nil
}
