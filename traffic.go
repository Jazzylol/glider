package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nadoo/glider/pkg/log"
)

// DomainStats 域名统计信息
type DomainStats struct {
	Domain        string
	Requests      int64
	UploadBytes   int64
	DownloadBytes int64
}

// TotalBytes 返回总流量
func (d *DomainStats) TotalBytes() int64 {
	return d.UploadBytes + d.DownloadBytes
}

// TrafficStatsManager 流量统计管理器
type TrafficStatsManager struct {
	mu        sync.RWMutex
	stats     map[string]*DomainStats
	startTime time.Time
}

// 全局统计管理器实例
var trafficStats *TrafficStatsManager

func init() {
	trafficStats = &TrafficStatsManager{
		stats:     make(map[string]*DomainStats),
		startTime: time.Now(),
	}
}

// RecordTraffic 记录流量统计（供 HTTP server 调用）
// 该函数保证不会 panic，出错只打印日志
func RecordTraffic(target string, upBytes, downBytes int64) {
	defer func() {
		if r := recover(); r != nil {
			log.F("[traffic] RecordTraffic panic recovered: %v", r)
		}
	}()

	domain := extractDomain(target)
	if domain == "" {
		return
	}

	trafficStats.mu.Lock()
	defer trafficStats.mu.Unlock()

	stat, ok := trafficStats.stats[domain]
	if !ok {
		stat = &DomainStats{Domain: domain}
		trafficStats.stats[domain] = stat
	}

	stat.Requests++
	stat.UploadBytes += upBytes
	stat.DownloadBytes += downBytes
}

// extractDomain 从 target 中提取域名
// target 格式可能是: "example.com:443" 或 "example.com"
func extractDomain(target string) string {
	if target == "" {
		return ""
	}

	// 移除端口号
	host := target
	if idx := strings.LastIndex(target, ":"); idx != -1 {
		// 检查是否是 IPv6 地址 [::1]:port
		if !strings.Contains(target[idx:], "]") {
			host = target[:idx]
		}
	}

	// 移除方括号（IPv6）
	host = strings.Trim(host, "[]")

	return host
}

// GetTop20ByRequests 获取请求次数最多的 Top 20 域名
func (m *TrafficStatsManager) GetTop20ByRequests() []*DomainStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := m.copyStats()

	// 按请求次数降序排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].Requests > list[j].Requests
	})

	// 取前 20 个
	if len(list) > 20 {
		list = list[:20]
	}

	return list
}

// GetTop20ByTraffic 获取总流量最大的 Top 20 域名
func (m *TrafficStatsManager) GetTop20ByTraffic() []*DomainStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := m.copyStats()

	// 按总流量降序排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].TotalBytes() > list[j].TotalBytes()
	})

	// 取前 20 个
	if len(list) > 20 {
		list = list[:20]
	}

	return list
}

// copyStats 复制所有统计到切片（需要在持有锁的情况下调用）
func (m *TrafficStatsManager) copyStats() []*DomainStats {
	list := make([]*DomainStats, 0, len(m.stats))
	for _, stat := range m.stats {
		// 深拷贝，避免并发问题
		list = append(list, &DomainStats{
			Domain:        stat.Domain,
			Requests:      stat.Requests,
			UploadBytes:   stat.UploadBytes,
			DownloadBytes: stat.DownloadBytes,
		})
	}
	return list
}

// GetTotalStats 获取所有域名的汇总统计
func (m *TrafficStatsManager) GetTotalStats() (totalDomains int, totalRequests, totalUp, totalDown int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalDomains = len(m.stats)
	for _, stat := range m.stats {
		totalRequests += stat.Requests
		totalUp += stat.UploadBytes
		totalDown += stat.DownloadBytes
	}
	return
}

// PrintTop20Table 打印 Top 20 表格（按请求次数和按流量两个表格）
// 该函数保证不会 panic，出错只打印日志
func (m *TrafficStatsManager) PrintTop20Table() {
	defer func() {
		if r := recover(); r != nil {
			log.F("[traffic] PrintTop20Table panic recovered: %v", r)
		}
	}()

	top20ByRequests := m.GetTop20ByRequests()
	top20ByTraffic := m.GetTop20ByTraffic()
	totalDomains, totalRequests, totalUp, totalDown := m.GetTotalStats()

	now := time.Now()

	// 构建表格
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("══════════════════════════════════════════════════════════════════════════════════════════════════════\n")
	sb.WriteString("                                      TRAFFIC STATISTICS REPORT                                       \n")
	sb.WriteString("══════════════════════════════════════════════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("Start Time: %s    Report Time: %s    Uptime: %s    Total Domains: %d\n",
		m.startTime.Format("2006-01-02 15:04:05"),
		now.Format("2006-01-02 15:04:05"),
		formatDuration(now.Sub(m.startTime)),
		totalDomains))
	sb.WriteString("\n")

	// ===== 按请求次数排序的表格 =====
	sb.WriteString("┌─────────────────────────────────── Top 20 by Requests ───────────────────────────────────┐\n")
	m.writeTable(&sb, top20ByRequests)

	sb.WriteString("\n")

	// ===== 按流量排序的表格 =====
	sb.WriteString("┌─────────────────────────────────── Top 20 by Traffic ────────────────────────────────────┐\n")
	m.writeTable(&sb, top20ByTraffic)

	// 汇总行
	sb.WriteString("\n")
	sb.WriteString("══════════════════════════════════════════════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("  TOTAL (All %d Domains):  Requests: %s    Upload↑: %s    Download↓: %s    Total: %s\n",
		totalDomains,
		formatNumber(totalRequests),
		formatBytes(totalUp),
		formatBytes(totalDown),
		formatBytes(totalUp+totalDown),
	))
	sb.WriteString("══════════════════════════════════════════════════════════════════════════════════════════════════════\n")

	log.F("[traffic] %s", sb.String())
}

// writeTable 写入表格内容
func (m *TrafficStatsManager) writeTable(sb *strings.Builder, stats []*DomainStats) {
	if len(stats) == 0 {
		sb.WriteString("  No traffic data yet.\n")
		return
	}

	// 表头
	sb.WriteString(fmt.Sprintf("│ %-4s  %-40s  %10s  %12s  %12s  %12s │\n",
		"Rank", "Domain", "Requests", "Upload↑", "Download↓", "Total"))
	sb.WriteString("├" + strings.Repeat("─", 98) + "┤\n")

	// 数据行
	for i, stat := range stats {
		domain := stat.Domain
		if len(domain) > 40 {
			domain = domain[:37] + "..."
		}
		sb.WriteString(fmt.Sprintf("│ %4d  %-40s  %10s  %12s  %12s  %12s │\n",
			i+1,
			domain,
			formatNumber(stat.Requests),
			formatBytes(stat.UploadBytes),
			formatBytes(stat.DownloadBytes),
			formatBytes(stat.TotalBytes()),
		))
	}
	sb.WriteString("└" + strings.Repeat("─", 98) + "┘\n")
}

// StartPeriodicReport 启动定期报告
// 该函数启动的协程保证不会因 panic 而终止
func StartPeriodicReport(interval time.Duration) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.F("[traffic] StartPeriodicReport goroutine panic recovered: %v", r)
				// 重新启动定期报告
				log.F("[traffic] Restarting periodic report...")
				StartPeriodicReport(interval)
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.F("[traffic] Traffic statistics reporter started, interval: %v", interval)

		for range ticker.C {
			trafficStats.PrintTop20Table()
		}
	}()
}

// formatBytes 格式化字节数为可读字符串
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// formatNumber 格式化数字（添加千分位分隔符）
func formatNumber(n int64) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	var result strings.Builder
	remainder := len(str) % 3
	if remainder > 0 {
		result.WriteString(str[:remainder])
		if len(str) > remainder {
			result.WriteString(",")
		}
	}

	for i := remainder; i < len(str); i += 3 {
		result.WriteString(str[i : i+3])
		if i+3 < len(str) {
			result.WriteString(",")
		}
	}

	return result.String()
}

// formatDuration 格式化持续时间
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

