// Package save 保存检测结果
package save

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/Wenaixi/submatrix/assets"
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

	// 【安全修复 2026-11-10】本地文件读取限制在 output/ 目录内，并拒绝绝对路径与 ..
	// 修掉「mihomo-overwrite-url 写绝对路径 → os.ReadFile 任意文件读取」链（HIGH）。
	if !strings.Contains(source, "://") {
		// 仅允许相对路径（如 output/template.yaml），不允许绝对路径/盘符/上级目录。
		// 统一在 output 根下解析，杜绝读任意文件。
		if filepath.IsAbs(source) || strings.Contains(source, "..") || strings.Contains(source, ":") {
			return nil, fmt.Errorf("本地模板路径不合法（仅允许 output/ 内相对路径）: %s", source)
		}
		localPath := filepath.Join("output", source)
		return os.ReadFile(localPath)
	}

	// 仅允许 http/https 协议，杜绝 file:// / gopher:// 等（SSRF 加固）
	u, err := url.Parse(source)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("模板地址协议不允许: %s", source)
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
// sanitizeProxyForYAML 把节点 map 里的非字符串值清洗成 YAML 可安全序列化的形态。
//
// 关键点：[]byte 必须递归到任意深度。此前只处理顶层与一层 []any，
// 而 ws-opts / plugin-opts / reality-opts 这类嵌套 map 里的 []byte
// 会被 YAML 编成 !!binary，客户端无法解析。
func sanitizeProxyForYAML(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	clean := make(map[string]any, len(raw))
	for k, v := range raw {
		clean[k] = sanitizeYAMLValue(v)
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

// sanitizeYAMLValue 递归清洗任意层级的值：[]byte → string，map 与切片逐层下钻。
func sanitizeYAMLValue(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	case map[string]any:
		if val == nil {
			return nil
		}
		out := make(map[string]any, len(val))
		for k, inner := range val {
			out[k] = sanitizeYAMLValue(inner)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = sanitizeYAMLValue(elem)
		}
		return out
	default:
		return val
	}
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
	// 其次读工作目录下的 assets/（本机直接跑二进制时存在）
	assetCandidate := filepath.Join("assets", filename)
	if data, err := os.ReadFile(assetCandidate); err == nil && len(data) > 0 {
		return data, nil
	}
	// 再次用 go:embed 的二进制内嵌副本：容器镜像里没有 assets/ 目录，
	// 这条路径是容器环境下唯一能拿到内置模板的来源。
	if embedded, ok := embeddedTemplate(filename); ok {
		return embedded, nil
	}
	return fetchMihomoTemplate()
}

// embeddedTemplate 按文件名返回 go:embed 打进二进制的内置模板。
// 容器镜像不含 assets/ 目录，这是容器内唯一的本地兜底来源。
func embeddedTemplate(filename string) ([]byte, bool) {
	switch filename {
	case "ACL4SSR_Online_Full.yaml":
		return assets.EmbeddedOverrideYamlACL4SSR, true
	case "Mihomo-Rules-CDN.yaml":
		return assets.EmbeddedOverrideYamlSinspiredRulesCDN, true
	case "Mihomo-Rules-Lite-CDN.yaml":
		return assets.EmbeddedOverrideYamlSinspiredRulesLiteCDN, true
	}
	return nil, false
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

// validGroupName 是组名白名单的最后一道防线。
//
// app 层入口（gateway / server / share_handler）已有同样的正则校验，
// 但 SaveGroup 是 save 包的直接落盘点，subctl、手改 config.yaml、
// 以及未来的新调用方都能绕过入口校验。组名会被拼进文件路径，
// 一旦允许 ../ 或绝对路径就是任意目录写入，因此这里必须再校验一次。
var groupNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

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
	if !groupNamePattern.MatchString(grp.Name) {
		slog.Error("组名含非法字符，拒绝落盘（防路径穿越）", "组名", grp.Name)
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

	// 1. 按 _state 分流：
	//    all.yaml 只放「真正可用」的节点（alive）——客户端直用，不能混入禁用的
	//    nodes-view.json 放全部三种状态（alive/skipped/slow），供前端节点明细页展示
	//    死节点根本不在这里（check 层就没送出来），符合「存活过滤=硬删除」
	//
	// 注意：_state / _state_reason 是本项目的内部标记，只用于前端展示，
	// 不能写进 all.yaml / all-raw.yaml——那是给客户端直接加载的配置，
	// 混入未知字段会污染客户端的节点定义。落盘前统一剥离。
	proxiesList := make([]map[string]any, 0, len(results)) // 仅 alive，供客户端直用
	rawList := make([]map[string]any, 0, len(results))     // 全量视图（alive+skipped+slow，含内部标记）
	for _, r := range results {
		if r.Proxy == nil {
			continue
		}
		clean := sanitizeProxyForYAML(r.Proxy)
		// 判定状态：_state 优先；没有则回退看旧标记 skipped:
		// 两套标记并存是因为 _state 是本轮新增的（携带可读原因），
		// 而 skipped 是既有字段，历史产物与部分测试仍只带它。
		// 两者都缺省时视为 alive（成功路径不写标记）。
		state, _ := clean["_state"].(string)
		if state == "" {
			if skipped, _ := clean["skipped"].(bool); skipped {
				state = "skipped"
				clean["_state"] = state
				if _, ok := clean["_state_reason"]; !ok {
					clean["_state_reason"] = "协议在过滤列表内"
				}
			} else {
				state = "alive"
				clean["_state"] = state
			}
		}
		rawList = append(rawList, clean)
		// 只有 alive 进 all.yaml；skipped/slow 是软禁用，只出现在视图里
		if state == "alive" {
			proxiesList = append(proxiesList, stripInternalState(clean))
		}
	}

	// 2. 单组节点上限（仅截取已检测列表，raw 全量保留）
	if grp.SuccessLimit > 0 && len(proxiesList) > int(grp.SuccessLimit) {
		slog.Info("单组节点上限生效", "组", grp.Name, "原数量", len(proxiesList), "截取后", grp.SuccessLimit)
		proxiesList = proxiesList[:grp.SuccessLimit]
	}

	// 3. 落盘 all.yaml（已检测）与 mihomo.yaml（融合规则）
	if writeGroupYAML(grp, proxiesList) {
		slog.Info("组产物已落盘", "组", grp.Name, "节点", len(proxiesList), "历史", len(historyPool))
	} else {
		// 主产物未写全（磁盘满/权限不足等），不打印成功日志误导排障
		slog.Error("组产物落盘不完整", "组", grp.Name)
	}

	// 4. 落盘 all-raw.yaml（全量原始，含 skipped / slow 等软禁用节点）。
	// 写盘前剥离内部标记：这是客户端可加载的配置，不能带 _state。
	if len(rawList) > 0 {
		stripped := make([]map[string]any, 0, len(rawList))
		for _, n := range rawList {
			stripped = append(stripped, stripInternalState(n))
		}
		if data, err := yaml.Marshal(map[string]any{"proxies": stripped}); err != nil {
			slog.Error("序列化全量原始节点失败", "组", grp.Name, "err", err)
		} else {
			writeGroupFile(filepath.Join(groupDir, "all-raw.yaml"), data)
		}

		// 4b. 同时落一份 JSON 给前端「节点明细」页：浏览器侧解析 YAML 不便，
		// JSON 直接 JSON.parse 即可。只保留展示需要的字段，减小体积。
		if data, err := json.Marshal(buildNodeViewPayload(grp.Name, rawList)); err != nil {
			slog.Error("序列化节点视图失败", "组", grp.Name, "err", err)
		} else {
			writeGroupFile(filepath.Join(groupDir, "nodes-view.json"), data)
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
			if data, err := yaml.Marshal(map[string]any{"proxies": deduped}); err != nil {
				slog.Error("序列化组历史池失败", "组", grp.Name, "err", err)
			} else {
				writeGroupFile(filepath.Join(groupDir, "history.yaml"), data)
			}
		}
	}

	// 成功日志已在调用 writeGroupYAML 之后打印（据其返回值决定），此处不再重复
}

// stripInternalState 返回去掉内部标记字段的节点副本。
// _state / _state_reason 只服务于前端展示与分流决策，
// 写进客户端配置（all.yaml / all-raw.yaml）会污染节点定义。
func stripInternalState(n map[string]any) map[string]any {
	out := make(map[string]any, len(n))
	for k, v := range n {
		if k == "_state" || k == "_state_reason" {
			continue
		}
		out[k] = v
	}
	return out
}

// nodeViewItem 是前端「节点明细」页需要的字段子集。
// 刻意不导出完整节点 map：里面有密码/UUID/私钥等凭据，
// 前端展示只需要状态、名称、协议、地区、速度这些无敏感信息。
type nodeViewItem struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	State    string `json:"state"`              // alive | skipped | slow
	Reason   string `json:"reason,omitempty"`   // 软禁用原因（可恢复的提示）
	SERVER   string `json:"server,omitempty"`   // 服务器地址（不是凭据）
	Port     any    `json:"port,omitempty"`
	Speed    int    `json:"speed,omitempty"`    // KB/s，仅测过速的有值
	Network  string `json:"network,omitempty"`
	SubTag   string `json:"sub_tag,omitempty"`
}

type nodeViewPayload struct {
	Group    string         `json:"group"`
	Total    int            `json:"total"`
	Alive    int            `json:"alive"`
	Skipped  int            `json:"skipped"`
	Slow     int            `json:"slow"`
	Nodes    []nodeViewItem `json:"nodes"`
}

// buildNodeViewPayload 从全量节点 map 提炼出前端展示用的小结构。
// 绝不含 password/uuid/private-key 等凭据字段。
func buildNodeViewPayload(groupName string, nodes []map[string]any) nodeViewPayload {
	payload := nodeViewPayload{
		Group: groupName,
		Total: len(nodes),
		Nodes: make([]nodeViewItem, 0, len(nodes)),
	}
	for _, n := range nodes {
		state, _ := n["_state"].(string)
		if state == "" {
			state = "alive"
		}
		switch state {
		case "alive":
			payload.Alive++
		case "skipped":
			payload.Skipped++
		case "slow":
			payload.Slow++
		}
		item := nodeViewItem{
			State:    state,
			Reason:   toStringField(n["_state_reason"]),
			Name:     toStringField(n["name"]),
			Type:     toStringField(n["type"]),
			SERVER:   toStringField(n["server"]),
			SubTag:   toStringField(n["sub_tag"]),
			Network:  toStringField(n["network"]),
		}
		item.Port = n["port"]
		// 速度：优先 _speed（若 check 层写过），否则从 speed 字段读
		if v, ok := n["speed"].(int); ok {
			item.Speed = v
		} else if v, ok := n["speed"].(float64); ok {
			item.Speed = int(v)
		}
		payload.Nodes = append(payload.Nodes, item)
	}
	return payload
}

// toStringField 安全地把 map 里的值转成字符串（缺失或 nil 返回空串）。
func toStringField(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// writeGroupFile 落盘单个组文件并检查写入结果。
// 原先一律 `_ = os.WriteFile`，磁盘满/权限不足时静默失败，日志却照打「已落盘」，
// 对外长期提供陈旧产物且无法定位。返回是否成功，供调用方决定是否打印成功日志。
//
// 用「写临时文件 + rename」而不是直接写目标路径：
// 组产物是客户端反复拉取的订阅文件，直接覆盖时若进程在写一半被杀
// （容器重启、OOM），留下的是半截 YAML，客户端会解析失败。
// rename 在同一文件系统内是原子的，保证要么旧版完整、要么新版完整。
func writeGroupFile(path string, data []byte) bool {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		slog.Error("组产物写入失败", "path", path, "err", err)
		return false
	}
	if err := os.Rename(tmp, path); err != nil {
		slog.Error("组产物重命名失败", "path", path, "err", err)
		_ = os.Remove(tmp)
		return false
	}
	return true
}

// writeGroupYAML 把一组节点写入 groups/<name>/all.yaml 与 mihomo.yaml（按规则预设融合）。
// writeGroupYAML 把一组节点写入 groups/<name>/all.yaml 与 mihomo.yaml（按规则预设融合）。
// 返回是否所有目标文件都写成功：调用方据此决定是否打印「已落盘」，
// 否则磁盘满/权限不足时日志仍报成功，正是 writeGroupFile 想解决的问题。
func writeGroupYAML(grp config.ResolvedGroupConfig, proxiesList []map[string]any) bool {
	if !groupNamePattern.MatchString(grp.Name) {
		slog.Error("组名含非法字符，拒绝落盘（防路径穿越）", "组名", grp.Name)
		return false
	}
	saver, err := method.NewLocalSaver()
	if err != nil {
		slog.Error("获取保存目录失败", "err", err)
		return false
	}

	groupDir := filepath.Join(saver.OutputPath, "groups", grp.Name)
	if err := os.MkdirAll(groupDir, 0755); err != nil {
		slog.Error("创建组输出目录失败", "dir", groupDir, "err", err)
		return false
	}

	// 节点上限已在 SaveGroup 截取（本函数为内部落盘点，不再重复截取）

	allYamlData, err := yaml.Marshal(map[string]any{"proxies": proxiesList})
	if err != nil {
		slog.Error("序列化组节点失败，跳过写入", "组", grp.Name, "err", err)
		return false
	}
	allOK := writeGroupFile(filepath.Join(groupDir, "all.yaml"), allYamlData)

	if grp.RulePreset == config.RulePresetDirectPure {
		return writeGroupFile(filepath.Join(groupDir, "mihomo.yaml"), allYamlData) && allOK
	}

	mihomoData, err := buildGroupMihomoYAML(grp, proxiesList)
	switch {
	case err == nil && len(mihomoData) > 0:
		return writeGroupFile(filepath.Join(groupDir, "mihomo.yaml"), mihomoData) && allOK
	case err != nil:
		// 模板加载/解析失败：必须记录并降级，否则客户端拿不到 mihomo.yaml 而日志毫无线索
		slog.Warn("合并组规则模板失败，降级保存纯节点列表",
			"group", grp.Name, "preset", grp.RulePreset, "err", err)
		return writeGroupFile(filepath.Join(groupDir, "mihomo.yaml"), allYamlData) && allOK
	default:
		// err == nil 且模板为空：仅纯节点模式（direct_pure 已提前 return），视为异常但可降级
		slog.Warn("规则模板为空，降级保存纯节点列表", "group", grp.Name, "preset", grp.RulePreset)
		return writeGroupFile(filepath.Join(groupDir, "mihomo.yaml"), allYamlData) && allOK
	}
}
