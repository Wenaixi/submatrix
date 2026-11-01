package check

import (
	"fmt"
	"strings"

	"github.com/beck-8/subs-check/config"
)

// DeduplicateProxies 根据 server:port:type 物理特征去重
func DeduplicateProxies(proxies []map[string]any) []map[string]any {
	seen := make(map[string]struct{}, len(proxies))
	res := make([]map[string]any, 0, len(proxies))

	for _, p := range proxies {
		server, _ := p["server"].(string)
		port := fmt.Sprintf("%v", p["port"])
		pType, _ := p["type"].(string)

		key := fmt.Sprintf("%s:%s:%s", strings.TrimSpace(server), strings.TrimSpace(port), strings.TrimSpace(pType))
		if key == "::" {
			// 没有基本连接信息的非法节点直接过滤
			continue
		}

		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			res = append(res, p)
		}
	}
	return res
}

// HandleFailedAliveNode 处理存活检测失败的节点
// 若 DropOnFail 为 true，直接淘汰 (返回 nil)
// 若 DropOnFail 为 false，不淘汰，标记为 [离线] 沉底保留
func HandleFailedAliveNode(grp config.ResolvedGroupConfig, proxy map[string]any) *Result {
	if grp.DropOnFail {
		return nil
	}

	origName := ""
	if n, ok := proxy["name"].(string); ok {
		origName = n
	}

	offlineName := RenderOfflineNode(origName, grp.Tag)

	// 克隆 map 避免污染源数据
	cloned := make(map[string]any, len(proxy)+2)
	for k, v := range proxy {
		cloned[k] = v
	}
	cloned["name"] = offlineName
	cloned["offline"] = true

	return &Result{
		Proxy:   cloned,
		Country: "",
		IPType:  "",
		Speed:   0,
	}
}

// ExecuteGroupProxies 按组上下文执行节点过滤与改名
func ExecuteGroupProxies(grp config.ResolvedGroupConfig, rawProxies []map[string]any) ([]Result, error) {
	deduped := DeduplicateProxies(rawProxies)

	// 1. 若不测活 (透传聚合组/直通组)，直接装配结果并秒级返回
	if !grp.CheckAlive {
		results := make([]Result, 0, len(deduped))
		for _, p := range deduped {
			nodeMap := make(map[string]any, len(p))
			for k, v := range p {
				nodeMap[k] = v
			}

			// 如果组声明了 tag 且节点未带 sub_tag，补充组标签
			if grp.Tag != "" {
				if _, ok := nodeMap["sub_tag"]; !ok {
					nodeMap["sub_tag"] = grp.Tag
				}
			}

			results = append(results, Result{
				Proxy:   nodeMap,
				Country: "",
				IPType:  "",
				Speed:   0,
			})
		}
		return results, nil
	}

	// 2. 测活组正常返回装配好的结果
	results := make([]Result, 0, len(deduped))
	for _, p := range deduped {
		nodeMap := make(map[string]any, len(p))
		for k, v := range p {
			nodeMap[k] = v
		}
		if grp.Tag != "" {
			if _, ok := nodeMap["sub_tag"]; !ok {
				nodeMap["sub_tag"] = grp.Tag
			}
		}

		results = append(results, Result{
			Proxy: nodeMap,
		})
	}
	return results, nil
}
