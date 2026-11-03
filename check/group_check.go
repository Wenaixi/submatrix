package check

import (
	"fmt"
	"strings"

	"github.com/Wenaixi/submatrix/config"
)

// DeduplicateProxies 按 server:port:type 物理特征对节点列表去重
func DeduplicateProxies(proxies []map[string]any) []map[string]any {
	seen := make(map[string]struct{}, len(proxies))
	res := make([]map[string]any, 0, len(proxies))
	for _, p := range proxies {
		server := fmt.Sprintf("%v", p["server"])
		port := fmt.Sprintf("%v", p["port"])
		typ := fmt.Sprintf("%v", p["type"])
		key := strings.ToLower(fmt.Sprintf("%s:%s:%s", server, port, typ))
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			res = append(res, p)
		}
	}
	return res
}

// RenderOfflineNode 将测试失败但免淘汰的节点格式化为离线沉底节点
func RenderOfflineNode(raw map[string]any, tag string) map[string]any {
	clone := make(map[string]any, len(raw)+2)
	for k, v := range raw {
		clone[k] = v
	}
	rawName, _ := clone["raw_name"].(string)
	if rawName == "" {
		rawName, _ = clone["name"].(string)
	}
	name := "[离线] " + rawName
	if tag != "" {
		name += "|" + tag
	}
	clone["name"] = name
	clone["offline"] = true
	return clone
}

// extractRootOriginalName 幂等提取最内层真正的原始节点名，防止聚合重入套娃
func extractRootOriginalName(raw string) string {
	const prefix = " (原名: "
	raw = strings.TrimSpace(raw)
	for {
		start := strings.Index(raw, prefix)
		if start == -1 {
			break
		}
		end := strings.LastIndex(raw, ")")
		if end > start+len(prefix) {
			raw = strings.TrimSpace(raw[start+len(prefix) : end])
		} else {
			break
		}
	}
	return raw
}

// FindGroupByTagOrURL 在所有有效分组中匹配所属组（转发至 config.FindGroupByTagOrURL）
func FindGroupByTagOrURL(tag, subURL string) config.ResolvedGroupConfig {
	return config.FindGroupByTagOrURL(tag, subURL)
}

// FilterProxiesByGroup 根据组的协议过滤规则过滤节点列表
func FilterProxiesByGroup(grp config.ResolvedGroupConfig, proxies []map[string]any) []map[string]any {
	res := make([]map[string]any, 0, len(proxies))
	for _, p := range proxies {
		if p == nil {
			continue
		}
		typ := strings.ToLower(fmt.Sprintf("%v", p["type"]))
		if grp.MatchesProtocol(typ) {
			continue
		}
		res = append(res, p)
	}
	return res
}

