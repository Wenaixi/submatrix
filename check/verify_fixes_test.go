package check

import (
	"strconv"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// 本轮系列修复的回归验证集合。

// 1) 地区之和必须等于 Total（漏计/重复计数都会破坏这条不变式）
func TestInvariantCountrySumEqualsTotal(t *testing.T) {
	cases := []string{
		"🏴‍☠️_1|免费A",  // 海盗旗：无地区 -> Unknown
		"🇭🇰HK⁻¹",     // CF block 角标
		"🇺🇸US⁰",      // CF incon 角标（历史重复计数点）
		"🇸🇬SG¹⁺",     // CF con 双角标
		"🇹🇼TW²_1|MTF", // 非 CF
		"🇯🇵JP_1|X",   // 纯国旗
	}
	for _, n := range cases {
		s := newAnalysisStats()
		applyNodeToStats(s, n, "vless", 0, false)
		sum := 0
		for _, v := range s.Countries {
			sum += v
		}
		if sum != s.Total {
			t.Errorf("%q: 地区之和=%d, Total=%d (不变式被破坏)", n, sum, s.Total)
		}
	}
}

// 2) 报告中带双引号与井号的字段不能破坏 YAML 解析
func TestReportYAMLWithQuotes(t *testing.T) {
	errMsg := `1次重试后失败: Get "https://x.io/a?token=t": EOF`
	url := "https://x.io/group/all.yaml#tag"

	var sb strings.Builder
	sb.WriteString("subs_ranking_bad:\n")
	sb.WriteString("  - url: ")
	sb.WriteString(strconv.Quote(url))
	sb.WriteString("\n    error: ")
	sb.WriteString(strconv.Quote(errMsg))
	sb.WriteString("\n")

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(sb.String()), &parsed); err != nil {
		t.Fatalf("含引号/井号的报告应可解析: %v", err)
	}
	list := parsed["subs_ranking_bad"].([]any)
	item := list[0].(map[string]any)
	if item["error"] != errMsg {
		t.Errorf("error 未完整保留: %v", item["error"])
	}
	if item["url"] != url {
		t.Errorf("url 的 # 被截断: %v", item["url"])
	}
}

// TestCompoundTagNotDoubleCounted 验证复合角标（US¹-US⁰）只计一次地区。
//
// 这是实际生产数据里抓到的 bug：🇺🇸US¹-US⁰_1|免费A 同时命中
// reCFConsistent（US¹）与 reCFInconsistent（US⁰），各加一次 Countries，
// 导致地区之和比 Total 多 2。修复后按优先级只取第一个命中的类别。
func TestCompoundTagNotDoubleCounted(t *testing.T) {
	cases := []string{
		"🇺🇸US¹-US⁰_1|免费A", // 生产数据里真实出现的复合标注
		"🇺🇸US¹-US⁰_2|免费A",
		"🇭🇰HK¹-HK⁻¹_1",   // 理论上也可能出现的组合
		"🇸🇬SG²-SG¹_1",     // 非CF + CF
	}
	for _, n := range cases {
		s := newAnalysisStats()
		applyNodeToStats(s, n, "http", 0, false)
		sum := 0
		for _, v := range s.Countries {
			sum += v
		}
		if sum != 1 || s.Total != 1 {
			t.Errorf("%q: 地区之和=%d Total=%d Countries=%v（复合角标被重复计数）",
				n, sum, s.Total, s.Countries)
		}
	}
}
