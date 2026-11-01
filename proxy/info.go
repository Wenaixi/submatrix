package proxies

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/metacubex/mihomo/common/convert"
)

// IPInfo 包含通过节点探测到的客观 IP 画像数据
type IPInfo struct {
	CountryCode string // ISO 3166-1 alpha-2 大写国家代号 (如 "HK", "US", "SG")
	IP          string // 真实出口 IP
	ISP         string // 运营商信息
	IPType      string // "家宽", "机房", "移动", 未知为 ""
}

// GetProxyIPInfo 使用 4 级高可用阶梯探测代理节点的真实国家与属性
func GetProxyIPInfo(httpClient *http.Client) IPInfo {
	if httpClient == nil {
		return IPInfo{}
	}

	// 1. Tier 1 主力端点: ip-api.com (单次请求同时获得国家代码、ISP 与机房/住宅标志)
	if info, ok := getIPApiInfo(httpClient); ok && info.CountryCode != "" {
		return info
	}

	// 2. 备用端点并发探测国家代码 (FlClash 常用 api.ip.sb + 腾讯 EdgeOne + Cloudflare)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var fallbackCountry, fallbackIP string

	endpoints := []func(*http.Client) (string, string){
		getIPSbInfo,
		GetEdgeOneProxy,
		GetCFProxy,
	}

	for _, fn := range endpoints {
		wg.Add(1)
		go func(f func(*http.Client) (string, string)) {
			defer wg.Done()
			c, ip := f(httpClient)
			if c != "" {
				mu.Lock()
				if fallbackCountry == "" {
					fallbackCountry = strings.ToUpper(strings.TrimSpace(c))
					fallbackIP = ip
				}
				mu.Unlock()
			}
		}(fn)
	}

	wg.Wait()

	return IPInfo{
		CountryCode: fallbackCountry,
		IP:          fallbackIP,
		IPType:      "", // 备用端点无机房标志，保持清爽空属性
	}
}

// getIPApiInfo 调用 ip-api.com 一步获取出口画像
func getIPApiInfo(httpClient *http.Client) (IPInfo, bool) {
	url := "http://ip-api.com/json/?fields=status,countryCode,isp,mobile,proxy,hosting,query"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return IPInfo{}, false
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())

	client := *httpClient
	client.Timeout = 6 * time.Second

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return IPInfo{}, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return IPInfo{}, false
	}

	type ipApiResponse struct {
		Status      string `json:"status"`
		CountryCode string `json:"countryCode"`
		Query       string `json:"query"`
		Hosting     bool   `json:"hosting"`
		Mobile      bool   `json:"mobile"`
		ISP         string `json:"isp"`
	}

	var res ipApiResponse
	if err := json.Unmarshal(body, &res); err != nil || res.Status != "success" {
		return IPInfo{}, false
	}

	ipType := "机房"
	if !res.Hosting && !res.Mobile {
		ipType = "家宽"
	} else if res.Mobile {
		ipType = "移动"
	}

	return IPInfo{
		CountryCode: strings.ToUpper(strings.TrimSpace(res.CountryCode)),
		IP:          res.Query,
		ISP:         res.ISP,
		IPType:      ipType,
	}, true
}

// getIPSbInfo 调用 api.ip.sb (FlClash 常用默认端点)
func getIPSbInfo(httpClient *http.Client) (string, string) {
	url := "https://api.ip.sb/geoip"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())

	client := *httpClient
	client.Timeout = 5 * time.Second

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ""
	}

	type ipsbResp struct {
		CountryCode string `json:"country_code"`
		IP          string `json:"ip"`
	}
	var res ipsbResp
	if err := json.Unmarshal(body, &res); err == nil && res.CountryCode != "" {
		return strings.ToUpper(strings.TrimSpace(res.CountryCode)), res.IP
	}
	return "", ""
}

// GetProxyCountry 保持向下兼容接口
func GetProxyCountry(httpClient *http.Client) (loc string, ip string) {
	info := GetProxyIPInfo(httpClient)
	return info.CountryCode, info.IP
}

// GetEdgeOneProxy 通过腾讯 EdgeOne 获取地理位置
func GetEdgeOneProxy(httpClient *http.Client) (loc string, ip string) {
	type GeoResponse struct {
		Eo struct {
			Geo struct {
				CountryCodeAlpha2 string `json:"countryCodeAlpha2"`
			} `json:"geo"`
			ClientIp string `json:"clientIp"`
		} `json:"eo"`
	}

	url := "https://functions-geolocation.edgeone.app/geo"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ""
	}

	var eo GeoResponse
	if err := json.Unmarshal(body, &eo); err == nil {
		return strings.ToUpper(strings.TrimSpace(eo.Eo.Geo.CountryCodeAlpha2)), eo.Eo.ClientIp
	}
	return "", ""
}

// GetCFProxy 通过 Cloudflare cdn-cgi/trace 获取地理位置
func GetCFProxy(httpClient *http.Client) (loc string, ip string) {
	url := "https://www.cloudflare.com/cdn-cgi/trace"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ""
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "loc=") {
			loc = strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(line, "loc=")))
		}
		if strings.HasPrefix(line, "ip=") {
			ip = strings.TrimSpace(strings.TrimPrefix(line, "ip="))
		}
	}
	return loc, ip
}