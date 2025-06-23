package netx

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// GetOutboundIP 获取本机外网IP
func GetOutboundIP() string {
	// 先获取本机IP
	ip := getLocalOutboundIP()
	if ip == "" {
		return ""
	}

	// 根据IP地理位置选择DNS服务器
	dnsServer := selectDNSServerByLocation(ip)

	// 使用选定的DNS服务器重新获取IP（确保准确性）
	return getOutboundIPWithDNS(dnsServer)
}

// getLocalOutboundIP 获取本地外网IP（使用默认DNS）
func getLocalOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// getOutboundIPWithDNS 使用指定DNS服务器获取外网IP
func getOutboundIPWithDNS(dnsServer string) string {
	conn, err := net.Dial("udp", dnsServer+":80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// selectDNSServerByLocation 根据IP地理位置选择DNS服务器
func selectDNSServerByLocation(ip string) string {
	// 检查是否为内网IP
	if isPrivateIP(ip) {
		// 内网IP，使用国内DNS
		return "114.114.114.114"
	}

	// 检查是否为国内IP段
	if isChineseIP(ip) {
		return "114.114.114.114"
	}

	// 国外IP，使用Google DNS
	return "8.8.8.8"
}

// isPrivateIP 检查是否为内网IP
func isPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// 检查是否为内网IP段
	privateRanges := []string{
		"10.0.0.0/8",     // A类私有网络
		"172.16.0.0/12",  // B类私有网络
		"192.168.0.0/16", // C类私有网络
		"127.0.0.0/8",    // 回环地址
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// isChineseIP 检查是否为国内IP（简化版本）
func isChineseIP(ip string) bool {
	// 这里可以集成IP地理位置数据库
	// 简化版本：通过在线API查询
	return queryIPLocation(ip)
}

// queryIPLocation 查询IP地理位置
func queryIPLocation(ip string) bool {
	// 使用免费的IP地理位置API
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=countryCode", ip)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		// 如果查询失败，默认使用国内DNS
		return true
	}
	defer resp.Body.Close()

	var result struct {
		CountryCode string `json:"countryCode"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return true
	}

	// 检查是否为中国
	return result.CountryCode == "CN"
}

// GetOutboundIPWithFallback 带降级策略的IP获取
func GetOutboundIPWithFallback() string {
	// 尝试使用国内DNS
	ip := getOutboundIPWithDNS("114.114.114.114")
	if ip != "" {
		return ip
	}

	// 如果失败，尝试Google DNS
	ip = getOutboundIPWithDNS("8.8.8.8")
	if ip != "" {
		return ip
	}

	// 最后尝试默认方法
	return getLocalOutboundIP()
}
