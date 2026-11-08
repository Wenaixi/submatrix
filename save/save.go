// Package save 保存检测结果
package save

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/Wenaixi/submatrix/check"
	"github.com/Wenaixi/submatrix/config"
	"github.com/Wenaixi/submatrix/save/method"
	"github.com/Wenaixi/submatrix/utils"
)

func ReadFileIfExists(path string) ([]byte, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return os.ReadFile(path)
}


func mergeMihomoTemplate(templateData []byte, proxies []map[string]any) ([]byte, error) {
	if len(strings.TrimSpace(string(templateData))) == 0 {
		return nil, fmt.Errorf("mihomo 覆写模板为空")
	}

	merged := make(map[string]any)
	if err := yaml.Unmarshal(templateData, &merged); err != nil {
		return nil, fmt.Errorf("解析 mihomo 覆写模板失败: %w", err)
	}
	templateData = nil //nolint:ineffassign
	merged["proxies"] = proxies
	result, err := yaml.Marshal(merged)
	merged = nil  //nolint:ineffassign
	proxies = nil //nolint:ineffassign
	return result, err
}

func fetchMihomoTemplate() ([]byte, error) {
	source := strings.TrimSpace(config.GlobalConfig.MihomoOverwriteURL)
	if source == "" {
		source = "http://127.0.0.1:8199/Mihomo-Rules-CDN.yaml"
	}

	if !strings.Contains(source, "://") {
		return os.ReadFile(source)
	}

	if utils.IsLocalURL(source) {
		if data, err := readLocalMihomoTemplate(source); err == nil && len(data) > 0 {
			return data, nil
		}
		return nil, fmt.Errorf("本地模板不可达: %s", source)
	}

	return fetchAny(
		utils.WarpURL(source, utils.IsGhProxyAvailable),
		utils.WarpURL(source, false),
	)
}

func readLocalMihomoTemplate(source string) ([]byte, error) {
	u, err := url.Parse(source)
	if err != nil {
		return nil, err
	}
	filename := path.Base(u.Path)
	if filename == "." || filename == "/" || filename == "" {
		return nil, fmt.Errorf("无效的 mihomo 覆写模板路径: %s", source)
	}

	saver, err := method.NewLocalSaver()
	if err != nil {
		return nil, err
	}

	candidate := filepath.Join(saver.OutputPath, filename)
	return ReadFileIfExists(candidate)
}

func newProxyClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{}
	if proxyStr := config.GlobalConfig.SystemProxy; proxyStr != "" {
		if proxyURL, err := url.Parse(proxyStr); err != nil {
			slog.Warn("解析系统代理 URL 失败，不使用代理", "proxy_url", proxyStr, "err", err)
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}

func fetchURL(client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "clash.meta")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态码异常: %d", resp.StatusCode)
	}
	return body, nil
}

func fetchAny(proxyPrefixURL, directURL string) ([]byte, error) {
	type attempt struct {
		label  string
		client *http.Client
		url    string
	}

	directClient := &http.Client{Timeout: 20 * time.Second}
	sysClient := newProxyClient(20 * time.Second)

	attempts := []attempt{
		{"GithubProxy", directClient, proxyPrefixURL},
		{"系统代理", sysClient, directURL},
		{"裸直连", directClient, directURL},
	}

	seen := make(map[string]struct{})
	for _, a := range attempts {
		if _, dup := seen[a.url+a.label]; dup {
			continue
		}
		seen[a.url+a.label] = struct{}{}

		data, err := fetchURL(a.client, a.url)
		if err != nil {
			slog.Warn("拉取失败，尝试下一策略", "策略", a.label, "url", a.url, "err", err)
			continue
		}
		slog.Debug("拉取成功", "策略", a.label, "url", a.url)
		return data, nil
	}
	return nil, fmt.Errorf("所有策略均不可达: %s", directURL)
}

// deduplicateByProxyKey 通过物理连结参数精准去重
// 由于传入的 proxies 已经按 history_score 降序排列，
// 这个去重过程会天然保留相同物理节点中分数最高的那一个
func deduplicateByProxyKey(proxies []map[string]any) []map[string]any {
	seen := make(map[string]bool)
	var result []map[string]any

	for _, p := range proxies {
		// 兜底清理可能残留的旧标签
		delete(p, "sub_was_succeed")
		delete(p, "sub_from_history")

		key := utils.GenerateProxyKey(p)
		if !seen[key] {
			seen[key] = true
			result = append(result, p)
		}
	}

	// 在历史节点物理去重后，再进行一次名称唯一性处理
	utils.RenameDuplicateNodes(result)

	return result
}


// sanitizeProxyForYAML 清洗节点字典，防止外部源中由于 !!binary 导致的类型不匹配与序列化崩溃
func sanitizeProxyForYAML(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	clean := make(map[string]any, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case []byte:
			clean[k] = string(val)
		case []any:
			subList := make([]any, len(val))
			for i, elem := range val {
				if b, ok := elem.([]byte); ok {
					subList[i] = string(b)
				} else {
					subList[i] = elem
				}
			}
			clean[k] = subList
		default:
			clean[k] = val
		}
	}
	for _, stringKey := range []string{"cipher", "password", "server", "name", "raw_name", "type", "network"} {
		if val, exists := clean[stringKey]; exists && val != nil {
			if _, isStr := val.(string); !isStr {
				clean[stringKey] = fmt.Sprintf("%v", val)
			}
		}
	}
	return clean
}

// fetchTemplateByPreset 根据规则预设标识符加载对应的分流规则模板
func fetchTemplateByPreset(preset, overwriteURL string) ([]byte, error) {
	preset = strings.ToLower(strings.TrimSpace(preset))
	switch preset {
	case config.RulePresetDirectPure:
		return nil, nil // 纯节点直通模式
	case config.RulePresetProLite:
		return fetchLocalOrAssetTemplate("Mihomo-Rules-Lite-CDN.yaml")
	case config.RulePresetACL4SSR:
		return fetchLocalOrAssetTemplate("ACL4SSR_Online_Full.yaml")
	case config.RulePresetCustom:
		if strings.TrimSpace(overwriteURL) != "" {
			return fetchAny(
				utils.WarpURL(overwriteURL, utils.IsGhProxyAvailable),
				utils.WarpURL(overwriteURL, false),
			)
		}
		return fetchLocalOrAssetTemplate("Mihomo-Rules-CDN.yaml")
	case config.RulePresetProFull, "":
		fallthrough
	default:
		// 默认使用最详分流矩阵（126 策略组 + 流媒体/AI 全分流，静态化资产）
		return fetchLocalOrAssetTemplate("Mihomo-Rules-126.yaml")
	}
}

// fetchLocalOrAssetTemplate 优先从 output 读取本地副本，其次从 assets/ 嵌入资产目录读取
func fetchLocalOrAssetTemplate(filename string) ([]byte, error) {
	saver, err := method.NewLocalSaver()
	if err == nil {
		candidate := filepath.Join(saver.OutputPath, filename)
		if data, err := ReadFileIfExists(candidate); err == nil && len(data) > 0 {
			return data, nil
		}
	}
	assetCandidate := filepath.Join("assets", filename)
	if data, err := os.ReadFile(assetCandidate); err == nil && len(data) > 0 {
		return data, nil
	}
	return fetchMihomoTemplate()
}

// buildGroupMihomoYAML 将单组节点合并进组专属或内置的 Mihomo 规则模板中
func buildGroupMihomoYAML(grp config.ResolvedGroupConfig, proxies []map[string]any) ([]byte, error) {
	templateData, err := fetchTemplateByPreset(grp.RulePreset, grp.OverwriteURL)
	if err != nil {
		return nil, err
	}
	if len(templateData) == 0 {
		return yaml.Marshal(map[string]any{"proxies": proxies})
	}
	content := string(templateData)
	// 126 组模板的 proxy-providers 用 {{ProviderURL}} 占位符，
	// 落盘时替换为"本组的公网产物地址"（public-url 或本机兜底），
	// 使 ProviderALL 等始终指向当前组自身的产物。
	content = strings.ReplaceAll(content, "{{ProviderURL}}", groupProviderURL(grp))
	return mergeMihomoTemplate([]byte(content), proxies)
}

// groupProviderURL 计算组 mihomo.yaml 中 proxy-providers 指向的本组产物地址。
func groupProviderURL(grp config.ResolvedGroupConfig) string {
	base := strings.TrimRight(strings.TrimSpace(config.GlobalConfig.PublicURL), "/")
	if base == "" {
		// 未配置 public-url 时退回本机（仅用于本地导入场景）
		port := strings.TrimPrefix(config.GlobalConfig.ListenPort, ":")
		base = "http://127.0.0.1:" + port
	}
	url := base + "/group/" + grp.Name + "/all.yaml"
	apiKey := strings.TrimSpace(config.GlobalConfig.APIKey)
	if apiKey != "" {
		url += "?token=" + apiKey
	}
	return url
}

// SaveGroup 保存单个组的检测结果到 groups/<name>/ 目录。
//   - all.yaml     纯节点列表（供 proxy-providers 引用）
//   - mihomo.yaml  按组规则预设融合后的完整配置
//   - history.yaml 本组历史节点评分池（保留 sub_url/sub_tag）
//
// results 为本组本次检测成功节点；historyPool 为本组历史池（含失败但仍有分数的节点）。
func SaveGroup(grp config.ResolvedGroupConfig, results []check.Result, historyPool []map[string]any) {
	if grp.Name == "" {
		return
	}

	saver, err := method.NewLocalSaver()
	if err != nil {
		slog.Error("获取保存目录失败", "err", err)
		return
	}
	groupDir := filepath.Join(saver.OutputPath, "groups", grp.Name)
	if err := os.MkdirAll(groupDir, 0755); err != nil {
		slog.Error("创建组输出目录失败", "dir", groupDir, "err", err)
		return
	}

	// 1. 过滤 + 清洗本次结果（区分已检测与 skipped 保留节点）
	proxiesList := make([]map[string]any, 0, len(results)) // 已检测（供客户端直用）
	rawList := make([]map[string]any, 0, len(results))      // 全量（含 skipped 未检测）
	for _, r := range results {
		if r.Proxy == nil {
			continue
		}
		clean := sanitizeProxyForYAML(r.Proxy)
		rawList = append(rawList, clean)
		if skipped, _ := clean["skipped"].(bool); skipped {
			continue
		}
		proxiesList = append(proxiesList, clean)
	}

	// 2. 单组节点上限（仅截取已检测列表，raw 全量保留）
	if grp.SuccessLimit > 0 && len(proxiesList) > int(grp.SuccessLimit) {
		slog.Info("单组节点上限生效", "组", grp.Name, "原数量", len(proxiesList), "截取后", grp.SuccessLimit)
		proxiesList = proxiesList[:grp.SuccessLimit]
	}

	// 3. 落盘 all.yaml（已检测）与 mihomo.yaml（融合规则）
	writeGroupYAML(grp, proxiesList)

	// 4. 落盘 all-raw.yaml（全量原始，含 skipped 未检测节点）
	if len(rawList) > 0 {
		if data, err := yaml.Marshal(map[string]any{"proxies": rawList}); err == nil {
			_ = os.WriteFile(filepath.Join(groupDir, "all-raw.yaml"), data, 0644)
		}
	}

	// 5. 落盘 history.yaml（组历史池，按分数降序已由 check 层保证）
	if len(historyPool) > 0 {
		pool := make([]map[string]any, 0, len(historyPool))
		for _, p := range historyPool {
			if p == nil {
				continue
			}
			typ := strings.ToLower(fmt.Sprintf("%v", p["type"]))
			if grp.MatchesProtocol(typ) {
				continue
			}
			pool = append(pool, sanitizeProxyForYAML(p))
		}
		if len(pool) > 0 {
			deduped := deduplicateByProxyKey(pool)
			if data, err := yaml.Marshal(map[string]any{"proxies": deduped}); err == nil {
				_ = os.WriteFile(filepath.Join(groupDir, "history.yaml"), data, 0644)
			}
		}
	}

	slog.Info("组产物已落盘", "组", grp.Name, "节点", len(proxiesList), "历史", len(historyPool))
}


// writeGroupYAML 把一组节点写入 groups/<name>/all.yaml 与 mihomo.yaml（按规则预设融合）。
func writeGroupYAML(grp config.ResolvedGroupConfig, proxiesList []map[string]any) {
	saver, err := method.NewLocalSaver()
	if err != nil {
		slog.Error("获取保存目录失败", "err", err)
		return
	}

	groupDir := filepath.Join(saver.OutputPath, "groups", grp.Name)
	if err := os.MkdirAll(groupDir, 0755); err != nil {
		slog.Error("创建组输出目录失败", "dir", groupDir, "err", err)
		return
	}

	// 节点上限已在 SaveGroup 截取（本函数为内部落盘点，不再重复截取）

	allYamlData, err := yaml.Marshal(map[string]any{"proxies": proxiesList})
	if err == nil {
		_ = os.WriteFile(filepath.Join(groupDir, "all.yaml"), allYamlData, 0644)
	}

	if grp.RulePreset == config.RulePresetDirectPure {
		_ = os.WriteFile(filepath.Join(groupDir, "mihomo.yaml"), allYamlData, 0644)
		return
	}

	mihomoData, err := buildGroupMihomoYAML(grp, proxiesList)
	if err == nil && len(mihomoData) > 0 {
		_ = os.WriteFile(filepath.Join(groupDir, "mihomo.yaml"), mihomoData, 0644)
	} else if err == nil {
		slog.Warn("合并组规则模板失败，降级保存纯节点", "group", grp.Name, "err", err)
		_ = os.WriteFile(filepath.Join(groupDir, "mihomo.yaml"), allYamlData, 0644)
	}
}

