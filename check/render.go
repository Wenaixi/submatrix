package check

import (
	"fmt"
	"strings"

	"github.com/beck-8/subs-check/config"
	proxyutils "github.com/beck-8/subs-check/proxy"
)

// RenderName 根据 Result 的结构化字段构造展示名。
func RenderName(r Result, includeSpeed bool) string {
	return RenderNameParts(r, includeSpeed).String()
}

// NameParts is the structured form of a display name
type NameParts struct {
	Base         string
	IPType       string
	SpeedTag     string
	Media        []MediaTag
	SubTag       string
	OriginalName string // 原始节点名
}

// MediaTag is one platform's result; an empty Tag means not unlocked.
type MediaTag struct {
	Platform string `json:"platform"`
	Tag      string `json:"tag"`
}

func (p NameParts) String() string {
	var tags []string
	if p.IPType == "家宽" || p.IPType == "机房" || p.IPType == "移动" {
		tags = append(tags, p.IPType)
	}
	if p.SpeedTag != "" {
		tags = append(tags, p.SpeedTag)
	}
	for _, m := range p.Media {
		if m.Tag != "" {
			tags = append(tags, m.Tag)
		}
	}
	if p.SubTag != "" {
		tags = append(tags, p.SubTag)
	}

	res := p.Base
	if len(tags) > 0 {
		if res != "" {
			res = res + "|" + strings.Join(tags, "|")
		} else {
			res = strings.Join(tags, "|")
		}
	}

	if p.OriginalName != "" {
		res = fmt.Sprintf("%s (原名: %s)", res, p.OriginalName)
	}
	return res
}

// RenderOfflineNode 渲染免丢弃组中测活失败的离线节点
func RenderOfflineNode(origName, subTag string) string {
	origName = strings.TrimSpace(origName)
	subTag = strings.TrimSpace(subTag)
	if subTag != "" {
		return fmt.Sprintf("[离线] %s|%s", origName, subTag)
	}
	return fmt.Sprintf("[离线] %s", origName)
}

// RenderNameParts is the structured form of RenderName.
func RenderNameParts(r Result, includeSpeed bool) NameParts {
	var p NameParts
	p.IPType = r.IPType

	// 1. base 名字
	if config.GlobalConfig.RenameNode {
		p.Base = config.GlobalConfig.NodePrefix + proxyutils.Rename(r.Country)
	} else if r.Proxy != nil {
		if n, ok := r.Proxy["name"].(string); ok {
			p.Base = strings.TrimSpace(n)
		}
	}

	// 2. 原始名字提取 (仅当开启了改名且开启了原名保留时追加)
	if r.Proxy != nil {
		rawName := ""
		if orig, ok := r.Proxy["raw_name"].(string); ok && orig != "" {
			rawName = orig
		} else if orig, ok := r.Proxy["name"].(string); ok && orig != "" {
			rawName = orig
		}

		if rawName != "" && config.GlobalConfig.RenameNode && config.GlobalConfig.AppendOriginalName {
			p.OriginalName = rawName
		}
	}

	// 3. 速度标签
	if includeSpeed && config.GlobalConfig.SpeedTestUrl != "" && r.Speed > 0 {
		p.SpeedTag = formatSpeedTag(r.Speed)
	}

	// 4. 流媒体平台标签
	for _, plat := range config.GlobalConfig.Platforms {
		p.Media = append(p.Media, MediaTag{Platform: plat, Tag: mediaTagFor(plat, &r)})
	}

	// 5. sub_tag 追加到最后
	if r.Proxy != nil {
		if t, ok := r.Proxy["sub_tag"].(string); ok && t != "" {
			p.SubTag = t
		}
	}

	return p
}

// mediaTagFor 返回单个平台的展示标签,未命中返回空字符串。
func mediaTagFor(plat string, r *Result) string {
	switch plat {
	case "openai":
		if r.Openai != nil {
			if r.Openai.Full {
				if r.Openai.Region != "" {
					return fmt.Sprintf("GPT⁺-%s", r.Openai.Region)
				}
				return "GPT⁺"
			}
			if r.Openai.Web {
				if r.Openai.Region != "" {
					return fmt.Sprintf("GPT-%s", r.Openai.Region)
				}
				return "GPT"
			}
		}
	case "netflix":
		if r.Netflix != nil {
			if r.Netflix.Full {
				if r.Netflix.Region != "" {
					return fmt.Sprintf("NF-%s", r.Netflix.Region)
				}
				return "NF"
			}
			if r.Netflix.OriginalsOnly {
				return "NF"
			}
		}
	case "disney":
		if r.Disney != nil && r.Disney.Unlocked {
			if r.Disney.Region != "" {
				return fmt.Sprintf("D+-%s", r.Disney.Region)
			}
			return "D+"
		}
	case "gemini":
		if r.Gemini != "" {
			return fmt.Sprintf("GM-%s", r.Gemini)
		}
	case "claude":
		if r.Claude != "" {
			return fmt.Sprintf("CL-%s", r.Claude)
		}
	case "spotify":
		if r.Spotify != "" {
			return fmt.Sprintf("SP-%s", r.Spotify)
		}
	case "iprisk":
		if r.IPRisk != "" {
			return r.IPRisk
		}
	case "youtube":
		if r.Youtube != "" {
			return fmt.Sprintf("YT-%s", r.Youtube)
		}
	case "tiktok":
		if r.TikTok != "" {
			return fmt.Sprintf("TK-%s", r.TikTok)
		}
	}
	return ""
}

// formatSpeedTag 把测速结果(KB/s)格式化为展示字符串。
func formatSpeedTag(speed int) string {
	if speed < 1024 {
		return fmt.Sprintf("%dKB/s", speed)
	}
	return fmt.Sprintf("%.1fMB/s", float64(speed)/1024)
}
