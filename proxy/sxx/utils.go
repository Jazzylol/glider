package sxx

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// PrettyPrint 格式化打印结构体（用于调试）
func PrettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return string(b)
}

// ParseProxyString 解析代理字符串
// 格式: host:port:username:password 或 host:port
func ParseProxyString(proxyStr string) (host string, port int, username, password string, err error) {
	parts := strings.Split(proxyStr, ":")

	if len(parts) < 2 {
		err = fmt.Errorf("invalid proxy format: %s", proxyStr)
		return
	}

	host = parts[0]
	port, err = strconv.Atoi(parts[1])
	if err != nil {
		err = fmt.Errorf("invalid port: %s", parts[1])
		return
	}

	if len(parts) >= 3 {
		username = parts[2]
	}
	if len(parts) >= 4 {
		password = parts[3]
	}

	return
}

// FormatProxyURL 格式化代理URL
// 格式: host:port:username:password -> http://username:password@host:port
func FormatProxyURL(host string, port int, username, password string) string {
	if username != "" && password != "" {
		return fmt.Sprintf("http://%s:%s@%s:%d", username, password, host, port)
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

// ValidateAPIKey 验证API Key格式
func ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API Key不能为空")
	}
	if len(apiKey) < 16 {
		return fmt.Errorf("API Key格式错误")
	}
	return nil
}

