package proxies

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/Wenaixi/submatrix/config"
	"github.com/Wenaixi/submatrix/proxy/parse"
	"github.com/Wenaixi/submatrix/utils"
)

// FetchGroupProxies 按单个组的配置拉取其全部 URL，合并去重后返回节点列表。
//
// 与全局 GetProxies 的差异：
//   - 只拉取本组的 URLs（不涉及 sub-urls-remote / last-result / history）
//   - 按组协议过滤（grp.MatchesProtocol）在此处完成
//   - 每个 URL 独立拉取，失败只记 Warn 不中断
//   - 组内跨 URL 用凭证指纹去重（同 IP 不同账号保留）
//
// 返回节点列表与原始节点计数（去重前）。
func FetchGroupProxies(grp config.ResolvedGroupConfig) ([]map[string]any, int, error) {
	if len(grp.URLs) == 0 {
		return nil, 0, fmt.Errorf("组 %s 没有配置任何订阅 URL", grp.Name)
	}

	var all []map[string]any
	rawCount := 0
	var lastErr error

	for _, rawURL := range grp.URLs {
		u := strings.TrimSpace(rawURL)
		if u == "" || strings.HasPrefix(u, "#") {
			continue
		}

		fetchURL, err := expandGroupRelativeURL(u)
		if err != nil {
			slog.Warn("组 URL 展开失败，跳过", "组", grp.Name, "url", u, "err", err)
			lastErr = err
			continue
		}

		data, err := FetchSubsData(fetchURL)
		if err != nil {
			slog.Warn("组订阅拉取失败，跳过", "组", grp.Name, "url", fetchURL, "err", err)
			lastErr = err
			continue
		}

		nodes, err := parse.ParseSubscriptionData(data, fetchURL)
		if err != nil {
			slog.Warn("组订阅解析失败，跳过", "组", grp.Name, "url", fetchURL, "err", err)
			lastErr = err
			continue
		}

		// 单 URL 内清洗：协议过滤、字段归一化、有效性校验
		valid := 0
		for _, node := range nodes {
			rawCount++
			if node == nil {
				continue
			}
			typ := strings.ToLower(fmt.Sprintf("%v", node["type"]))
			if typ == "" || typ == "invalid" {
				continue
			}
			// 组协议过滤（默认排除 http 明文协议）
			if grp.MatchesProtocol(typ) {
				continue
			}
			// 字段语义修正；返回 false 表示该节点无效
			if !parse.NormalizeNode(node) {
				continue
			}
			// 打上归属标记，供下游统计与报告使用
			node["sub_url"] = fetchURL
			if grp.Tag != "" {
				node["sub_tag"] = grp.Tag
			}
			all = append(all, node)
			valid++
		}
		slog.Info("组订阅已拉取", "组", grp.Name, "url", fetchURL, "节点数", valid)
	}

	if len(all) == 0 {
		if lastErr != nil {
			return nil, rawCount, fmt.Errorf("组 %s 全部 URL 拉取失败: %w", grp.Name, lastErr)
		}
		return nil, rawCount, fmt.Errorf("组 %s 未获取到任何节点", grp.Name)
	}

	// 组内跨 URL 去重：凭证指纹（同 IP 不同账号保留），保留先出现的 URL 版本
	deduped := utils.DeduplicateNodes(all)
	// 清理去重缓存字段，避免污染输出
	for _, n := range deduped {
		utils.DeleteNodeKey(n)
	}
	// 同名节点追加序号，避免客户端冲突
	utils.RenameDuplicateNodes(deduped)

	slog.Info("组节点合并完成", "组", grp.Name,
		"原始", rawCount, "合并去重后", len(deduped))

	return deduped, rawCount, nil
}

// expandGroupRelativeURL 把组内相对路径展开为公网绝对地址。
//
// 支持两种写法：
//   - 绝对地址（http:// 或 https:// 开头）：原样返回，但拒绝指向本机环回的地址
//   - 相对路径（/ 开头）：拼接 public-url + 路径 + token 查询参数
//
// public-url 未配置时直接报错——相对路径必须依赖公网入口。
func expandGroupRelativeURL(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", fmt.Errorf("URL 为空")
	}

	// 绝对地址：拒绝 127.0.0.1 / localhost 等本机地址（避免组间引用写成回环）
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		if utils.IsLocalURL(u) {
			return "", fmt.Errorf("组 URL 不允许指向本机地址: %s（相对路径请写成 /group/<name>/all.yaml）", u)
		}
		return u, nil
	}

	// 相对路径：必须以 / 开头
	if !strings.HasPrefix(u, "/") {
		return "", fmt.Errorf("组 URL 既不是绝对地址也不是相对路径: %s", u)
	}

	base := strings.TrimRight(strings.TrimSpace(config.GlobalConfig.PublicURL), "/")
	if base == "" {
		return "", fmt.Errorf("配置 public-url 为空，无法展开相对路径: %s", u)
	}
	if utils.IsLocalURL(base) {
		return "", fmt.Errorf("public-url 指向本机地址，无法用于组间引用: %s", base)
	}

	// 拼接 token（订阅鉴权），与网关 UnifiedAuthMiddleware 的 ?token= 通道对齐
	full := base + u
	apiKey := strings.TrimSpace(config.GlobalConfig.APIKey)
	if apiKey != "" && !strings.Contains(full, "token=") {
		sep := "?"
		if strings.Contains(full, "?") {
			sep = "&"
		}
		full += sep + "token=" + apiKey
	}
	return full, nil
}
