package config

// GroupConfig 代表用户在 config.yaml 的 groups 节点下定义的单组配置
type GroupConfig struct {
	Name               string   `yaml:"name"`
	Enabled            *bool    `yaml:"enabled"`
	DisplayName        string   `yaml:"display-name"`
	Tag                string   `yaml:"tag"`
	CheckAlive         *bool    `yaml:"check-alive"`
	CheckSpeed         *bool    `yaml:"check-speed"`
	MinSpeed           *int     `yaml:"min-speed"`
	MediaCheck         *bool    `yaml:"media-check"`
	TagIPType          *bool    `yaml:"tag-ip-type"`
	RenameNode         *bool    `yaml:"rename-node"`
	AppendOriginalName *bool    `yaml:"append-original-name"`
	DropOnFail         *bool    `yaml:"drop-on-fail"`
	OverwriteURL       string   `yaml:"overwrite-url"`
	Timeout            int      `yaml:"timeout"`
	URLs               []string `yaml:"urls"`
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
	DropOnFail         bool
	OverwriteURL       string
	Timeout            int
	URLs               []string
}

// ResolveGroup 将单组配置与全局配置级联合并
func ResolveGroup(g GroupConfig, global Config) ResolvedGroupConfig {
	enabled := true
	if g.Enabled != nil {
		enabled = *g.Enabled
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
		RenameNode:         global.RenameNode,
		AppendOriginalName: global.AppendOriginalName,
		DropOnFail:         true,
		OverwriteURL:       g.OverwriteURL,
		Timeout:            global.Timeout,
		URLs:               g.URLs,
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
	if g.DropOnFail != nil {
		res.DropOnFail = *g.DropOnFail
	}
	if g.Timeout > 0 {
		res.Timeout = g.Timeout
	}

	return res
}

// ResolveAllGroups 解析所有组；若未显式配置 groups 则将全局 sub-urls 映射为默认组向后兼容
func ResolveAllGroups(cfg Config) []ResolvedGroupConfig {
	if len(cfg.Groups) > 0 {
		resolved := make([]ResolvedGroupConfig, 0, len(cfg.Groups))
		for _, g := range cfg.Groups {
			if g.Name == "" {
				continue
			}
			resolved = append(resolved, ResolveGroup(g, cfg))
		}
		if len(resolved) > 0 {
			return resolved
		}
	}

	// 向后兼容模式
	return []ResolvedGroupConfig{
		ResolveGroup(GroupConfig{
			Name:        "default",
			DisplayName: "默认订阅组",
			URLs:        cfg.SubUrls,
		}, cfg),
	}
}
