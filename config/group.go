package config

import (
	"fmt"
	"strings"
)

// GroupConfig 组级原始配置（允许使用指针以区分未设置和显式设置）
type GroupConfig struct {
	Name               string   `yaml:"name"`
	DisplayName        string   `yaml:"display-name"`
	Tag                string   `yaml:"tag"`
	CheckAlive         *bool    `yaml:"check-alive"`
	CheckSpeed         *bool    `yaml:"check-speed"`
	MinSpeed           *int     `yaml:"min-speed"`
	Timeout            *int     `yaml:"timeout"`
	DropOnFail         *bool    `yaml:"drop-on-fail"`
	MediaCheck         *bool    `yaml:"media-check"`
	TagIPType          *bool    `yaml:"tag-ip-type"`
	RenameNode         *bool    `yaml:"rename-node"`
	AppendOriginalName *bool    `yaml:"append-original-name"`
	OverwriteURL       string   `yaml:"overwrite-url"`
	Urls               []string `yaml:"urls"`
}

// ResolvedGroupConfig 解析继承后的生效组配置
type ResolvedGroupConfig struct {
	Name               string
	DisplayName        string
	Tag                string
	CheckAlive         bool
	CheckSpeed         bool
	MinSpeed           int
	Timeout            int
	DropOnFail         bool
	MediaCheck         bool
	TagIPType          bool
	RenameNode         bool
	AppendOriginalName bool
	OverwriteURL       string
	Urls               []string
}

// ResolveAllGroups 解析全局配置并导出所有生效的组配置
func ResolveAllGroups(cfg *Config) []ResolvedGroupConfig {
	if cfg == nil {
		return nil
	}

	// 1. 向后兼容：若没有配置 groups，将现有的 sub-urls 封装为 default 组
	if len(cfg.Groups) == 0 {
		cleanUrls := filterCleanUrls(cfg.SubUrls)
		return []ResolvedGroupConfig{
			{
				Name:               "default",
				DisplayName:        "默认组",
				Tag:                "",
				CheckAlive:         true,
				CheckSpeed:         true,
				MinSpeed:           cfg.MinSpeed,
				Timeout:            resolveTimeout(cfg.Timeout),
				DropOnFail:         true,
				MediaCheck:         cfg.MediaCheck,
				TagIPType:          cfg.TagIPType,
				RenameNode:         cfg.RenameNode,
				AppendOriginalName: cfg.AppendOriginalName,
				OverwriteURL:       cfg.MihomoOverwriteUrl,
				Urls:               cleanUrls,
			},
		}
	}

	// 2. 多组解析与级联继承
	res := make([]ResolvedGroupConfig, 0, len(cfg.Groups))
	for idx, g := range cfg.Groups {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			name = fmt.Sprintf("group_%d", idx+1)
		}

		dispName := strings.TrimSpace(g.DisplayName)
		if dispName == "" {
			dispName = name
		}

		// 测活继承与判断
		checkAlive := true
		if g.CheckAlive != nil {
			checkAlive = *g.CheckAlive
		}

		// 测速判定 (若不测活，自动不测速)
		checkSpeed := true
		if !checkAlive {
			checkSpeed = false
		} else if g.CheckSpeed != nil {
			checkSpeed = *g.CheckSpeed
		}

		// 最低速度继承
		minSpeed := cfg.MinSpeed
		if g.MinSpeed != nil {
			minSpeed = *g.MinSpeed
		}

		// 超时时间继承
		timeout := resolveTimeout(cfg.Timeout)
		if g.Timeout != nil && *g.Timeout > 0 {
			timeout = *g.Timeout
		}

		// 失败是否丢弃 (免淘汰参数，默认 true)
		dropOnFail := true
		if g.DropOnFail != nil {
			dropOnFail = *g.DropOnFail
		}

		// 流媒体检测继承
		mediaCheck := cfg.MediaCheck
		if g.MediaCheck != nil {
			mediaCheck = *g.MediaCheck
		}

		// 家宽/机房打标继承
		tagIPType := cfg.TagIPType
		if g.TagIPType != nil {
			tagIPType = *g.TagIPType
		}

		// 改名判定 (若不测活，自动不改名，原汁原味透传)
		renameNode := cfg.RenameNode
		if !checkAlive {
			renameNode = false
		} else if g.RenameNode != nil {
			renameNode = *g.RenameNode
		}

		// 保留原名继承
		appendOrig := cfg.AppendOriginalName
		if g.AppendOriginalName != nil {
			appendOrig = *g.AppendOriginalName
		}

		// 规则集覆写文件继承
		overwrite := strings.TrimSpace(g.OverwriteURL)
		if overwrite == "" {
			overwrite = cfg.MihomoOverwriteUrl
		}

		cleanUrls := filterCleanUrls(g.Urls)

		res = append(res, ResolvedGroupConfig{
			Name:               name,
			DisplayName:        dispName,
			Tag:                strings.TrimSpace(g.Tag),
			CheckAlive:         checkAlive,
			CheckSpeed:         checkSpeed,
			MinSpeed:           minSpeed,
			Timeout:            timeout,
			DropOnFail:         dropOnFail,
			MediaCheck:         mediaCheck,
			TagIPType:          tagIPType,
			RenameNode:         renameNode,
			AppendOriginalName: appendOrig,
			OverwriteURL:       overwrite,
			Urls:               cleanUrls,
		})
	}

	return res
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
