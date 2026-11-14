package config

import (
	"sort"
	"strings"
)

// 规则集预设标识符定义
const (
	RulePresetProFull    = "pro_full"    // 默认最详分流矩阵 (PRO 全量，126 策略组 + 流媒体 + AI 全分流)
	RulePresetProLite    = "pro_lite"    // 精简分流矩阵 (PRO-Lite 核心轻量)
	RulePresetACL4SSR    = "acl4ssr"     // 经典 ACL4SSR 在线全量规则集
	RulePresetDirectPure = "direct_pure" // 纯节点直通 (仅节点列表，不带策略组)
	RulePresetCustom     = "custom"      // 自定义外部/内部链接 (使用 OverwriteURL)
)

// GroupConfig 代表用户在 config.yaml 的 groups 节点下定义的单组配置
type GroupConfig struct {
	Name               string   `json:"name" yaml:"name"`
	Enabled            *bool    `json:"enabled" yaml:"enabled"`
	DisplayName        string   `json:"display_name" yaml:"display-name"`
	Tag                string   `json:"tag" yaml:"tag"`
	CheckAlive         *bool    `json:"check_alive" yaml:"check-alive"`
	CheckSpeed         *bool    `json:"check_speed" yaml:"check-speed"`
	MinSpeed           *int     `json:"min_speed" yaml:"min-speed"`
	MediaCheck         *bool    `json:"media_check" yaml:"media-check"`
	TagIPType          *bool    `json:"tag_ip_type" yaml:"tag-ip-type"`
	RenameNode         *bool    `json:"rename_node" yaml:"rename-node"`
	AppendOriginalName *bool    `json:"append_original_name" yaml:"append-original-name"`
	EnhancedTag        *bool    `json:"enhanced_tag" yaml:"enhanced-tag"`
	NodePrefix         *string  `json:"node_prefix" yaml:"node-prefix"`
	ISPCheck           *bool    `json:"isp_check" yaml:"isp-check"`
	DropOnFail         *bool    `json:"drop_on_fail" yaml:"drop-on-fail"`
	RulePreset         string   `json:"rule_preset" yaml:"rule-preset"`
	OverwriteURL       string   `json:"overwrite_url" yaml:"overwrite-url"`
	Timeout            int      `json:"timeout" yaml:"timeout"`
	URLs               []string `json:"urls" yaml:"urls"`
	Proxy              string   `json:"proxy" yaml:"proxy"`
	FetchProxy         string   `json:"fetch_proxy" yaml:"fetch-proxy"`
	FilterProtocols    []string `json:"filter_protocols" yaml:"filter-protocols"`
	SuccessLimit       *int32   `json:"success_limit" yaml:"success-limit"`
	DownloadMB         *int     `json:"download_mb" yaml:"download-mb"`
	Priority           int      `json:"priority" yaml:"priority,omitempty"`
}

// ResolvedGroupConfig 级联继承全局配置后的最终可执行组配置
type ResolvedGroupConfig struct {
	Name               string
	Enabled            bool
	DisplayName        string
	Tag                string
	CheckAlive         bool
	CheckSpeed         bool
	MinSpeed           int
	MediaCheck         bool
	TagIPType          bool
	RenameNode         bool
	AppendOriginalName bool
	EnhancedTag        bool
	DropOnFail         bool
	RulePreset         string
	OverwriteURL       string
	Timeout            int
	URLs               []string
	Proxy              string
	FetchProxy         string
	FilterProtocols    []string
	SuccessLimit       int32
	DownloadMB         int
	Priority           int

	// 组级继承的全局字段（检测层参数化所需）
	SpeedTestURL string
	Platforms    []string
	NodePrefix   string
	ISPCheck     bool
}

// MatchesProtocol 检查指定协议是否属于本组配置过滤排除的协议
func (r ResolvedGroupConfig) MatchesProtocol(protocol string) bool {
	proto := strings.ToLower(strings.TrimSpace(protocol))
	for _, fp := range r.FilterProtocols {
		if strings.ToLower(strings.TrimSpace(fp)) == proto {
			return true
		}
	}
	return false
}

// MatchesNode 判断某个节点是否归属本组。
//
// 判定依据是节点从订阅解析时留下的元数据：
//   - sub_tag：订阅 URL 的 #fragment，优先匹配组的 Tag / Name / DisplayName
//   - sub_url：节点来源订阅地址，匹配组配置里列出的 URLs
//
// 两个依据都没有时返回 false，避免把无归属节点划进任意组。
func (r ResolvedGroupConfig) MatchesNode(tag, subURL string) bool {
	if tag != "" {
		if r.Tag == tag || r.Name == tag || r.DisplayName == tag {
			return true
		}
	}
	if subURL != "" {
		for _, u := range r.URLs {
			if u == subURL {
				return true
			}
		}
	}
	return false
}

// ResolveGroup 将单组配置与全局配置级联合并
func ResolveGroup(g GroupConfig, global Config) ResolvedGroupConfig {
	enabled := true
	if g.Enabled != nil {
		enabled = *g.Enabled
	}
	proxy := strings.TrimSpace(g.Proxy)
	// 拉取代理（fetch-proxy）与检测代理（proxy）相互独立：
	// 组内显式配置优先，未配置时回退全局 system-proxy，保持旧行为不变。
	fetchProxy := strings.TrimSpace(g.FetchProxy)
	if fetchProxy == "" {
		fetchProxy = strings.TrimSpace(global.SystemProxy)
	}
	filterProtocols := []string{"http"} // 默认过滤 http 协议
	if g.FilterProtocols != nil {
		filterProtocols = g.FilterProtocols
	}

	rulePreset := strings.TrimSpace(g.RulePreset)
	if rulePreset == "" {
		rulePreset = RulePresetACL4SSR // 默认使用 ACL4SSR 经典分流（与首尔部署一致）
	}

	res := ResolvedGroupConfig{
		Name:               g.Name,
		Enabled:            enabled,
		DisplayName:        g.DisplayName,
		Tag:                g.Tag,
		CheckAlive:         true,
		CheckSpeed:         global.SpeedTestURL != "",
		MinSpeed:           global.MinSpeed,
		MediaCheck:         global.MediaCheck,
		TagIPType:          global.TagIPType,
		Priority:           g.Priority,
		RenameNode:         global.RenameNode,
		AppendOriginalName: global.AppendOriginalName,
		EnhancedTag:        global.EnhancedTag,
		DropOnFail:         true,
		RulePreset:         rulePreset,
		OverwriteURL:       g.OverwriteURL,
		Timeout:            global.Timeout,
		URLs:               g.URLs,
		Proxy:              proxy,
		FetchProxy:         fetchProxy,
		FilterProtocols:    filterProtocols,
		// 组级继承的全局字段（检测层参数化所需）
		SpeedTestURL: global.SpeedTestURL,
		Platforms:    global.Platforms,
		NodePrefix:   global.NodePrefix,
		ISPCheck:     global.ISPCheck,
	}

	if res.DisplayName == "" {
		res.DisplayName = res.Name
	}

	if g.CheckAlive != nil {
		res.CheckAlive = *g.CheckAlive
	}
	if g.CheckSpeed != nil {
		res.CheckSpeed = *g.CheckSpeed
	}
	if g.MinSpeed != nil {
		res.MinSpeed = *g.MinSpeed
	}
	if g.MediaCheck != nil {
		res.MediaCheck = *g.MediaCheck
	}
	if g.TagIPType != nil {
		res.TagIPType = *g.TagIPType
	}
	if g.RenameNode != nil {
		res.RenameNode = *g.RenameNode
	}
	if g.AppendOriginalName != nil {
		res.AppendOriginalName = *g.AppendOriginalName
	}
	if g.EnhancedTag != nil {
		res.EnhancedTag = *g.EnhancedTag
	}
	if g.NodePrefix != nil {
		res.NodePrefix = *g.NodePrefix
	}
	if g.ISPCheck != nil {
		res.ISPCheck = *g.ISPCheck
	}
	if g.DropOnFail != nil {
		res.DropOnFail = *g.DropOnFail
	}
	if g.Timeout > 0 {
		res.Timeout = g.Timeout
	}
	res.SuccessLimit = global.SuccessLimit
	if g.SuccessLimit != nil {
		res.SuccessLimit = *g.SuccessLimit
	}
	res.DownloadMB = global.DownloadMB
	if g.DownloadMB != nil && *g.DownloadMB > 0 {
		res.DownloadMB = *g.DownloadMB
	}

	// 测速地址为空时必须关闭测速，否则测速环节会构造空 URL 请求并必然失败。
	// 放在所有组级覆盖之后，确保无论哪一层把 check-speed 打开，都会被这条约束兜住：
	// 没有可用的测速目标，测速就无从谈起，开着只会把本来可用的节点判成 speed_fail。
	if strings.TrimSpace(res.SpeedTestURL) == "" {
		res.CheckSpeed = false
	}

	return res
}

// ResolveAllGroups 解析全局配置并导出所有生效的组配置
func ResolveAllGroups(global Config) []ResolvedGroupConfig {
	if len(global.Groups) == 0 {
		cleanUrls := filterCleanUrls(global.SubUrls)
		return []ResolvedGroupConfig{
			{
				Name:               "default",
				DisplayName:        "默认组",
				Tag:                "",
				CheckAlive:         true,
				CheckSpeed:         true,
				MinSpeed:           global.MinSpeed,
				Timeout:            resolveTimeout(global.Timeout),
				DropOnFail:         true,
				MediaCheck:         global.MediaCheck,
				TagIPType:          global.TagIPType,
				RenameNode:         global.RenameNode,
				AppendOriginalName: global.AppendOriginalName,
				EnhancedTag:        global.EnhancedTag,
				RulePreset:         RulePresetACL4SSR,
				OverwriteURL:       "",
				URLs:               cleanUrls,
				Proxy:              "",
				FetchProxy:         global.SystemProxy,
				FilterProtocols:    []string{"http"},
				SuccessLimit:       global.SuccessLimit,
				DownloadMB:         global.DownloadMB,
				// 组级继承（default 组等价于全局配置）
				SpeedTestURL: global.SpeedTestURL,
				Platforms:    global.Platforms,
				NodePrefix:   global.NodePrefix,
				ISPCheck:     global.ISPCheck,
			},
		}
	}

	res := make([]ResolvedGroupConfig, 0, len(global.Groups))
	for _, g := range global.Groups {
		res = append(res, ResolveGroup(g, global))
	}
	sortResolvedGroups(res)
	return res
}

// sortResolvedGroups 按 Priority 升序稳定排序（priority 相同保持配置声明顺序）。
func sortResolvedGroups(groups []ResolvedGroupConfig) {
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].Priority < groups[j].Priority
	})
}


func resolveTimeout(t int) int {
	if t <= 0 {
		return 5000
	}
	return t
}

func filterCleanUrls(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, u := range raw {
		s := strings.TrimSpace(u)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// FindGroupByTagOrURL 在所有有效分组中匹配所属组，若未匹配则返回全局默认组配置
func FindGroupByTagOrURL(tag, subURL string) ResolvedGroupConfig {
	cfg := Config{}
	if GlobalConfig != nil {
		cfg = *GlobalConfig
	}
	groups := ResolveAllGroups(cfg)
	for _, g := range groups {
		if tag != "" && (g.Tag == tag || g.Name == tag || g.DisplayName == tag) {
			return g
		}
		for _, u := range g.URLs {
			if u == subURL {
				return g
			}
		}
	}
	if len(groups) > 0 {
		return groups[0]
	}
	return ResolveGroup(GroupConfig{}, cfg)
}

