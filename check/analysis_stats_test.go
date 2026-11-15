package check

import (
	"testing"
)

// TestCollectGroupAliveNodesDedup 验证同一物理节点被多个组引用时只计一次。
//
// 这是「所有组加起来然后去重」的核心保证：free-a 与 free-b 常引用相同的订阅源，
// 若不去重，地区分布会按组重复膨胀。
func TestCollectGroupAliveNodesDedup(t *testing.T) {
	ResetGlobalAliveNodes()

	// 同一个节点（server/端口/密码/SNI 完全相同）
	nodeAPI := map[string]any{
		"name":   "🇹🇼TW²_1|A",
		"type":   "hysteria2",
		"server": "tw4.mtf.llc",
		"port":   23562,
		"password": "2e68b21f-66aa-47ee-a778-f1d45fa1b051",
		"sni":    "tw4.mtf.llc",
	}
	nodeAPI2 := map[string]any{
		"name":     "🇹🇼TW²_9|B", // 名字不同、组不同，但物理上是同一节点
		"type":     "hysteria2",
		"server":   "tw4.mtf.llc",
		"port":     23562,
		"password": "2e68b21f-66aa-47ee-a778-f1d45fa1b051",
		"sni":      "tw4.mtf.llc",
	}
	// 不同账号（同 IP 不同密码）应视为不同节点
	nodeOtherAcct := map[string]any{
		"name":     "🇹🇼TW²_2|B",
		"type":     "hysteria2",
		"server":   "tw4.mtf.llc",
		"port":     23562,
		"password": "different-password",
		"sni":      "tw4.mtf.llc",
	}

	CollectGroupAliveNodes([]Result{{Proxy: nodeAPI}})
	CollectGroupAliveNodes([]Result{{Proxy: nodeAPI2}})
	CollectGroupAliveNodes([]Result{{Proxy: nodeOtherAcct}})

	// 期望：第一个节点被去重成 1 条，加上不同账号 1 条 = 2 条
	if got := GlobalAliveCount(); got != 2 {
		t.Fatalf("全局去重节点数 = %d，期望 2（同节点跨组只算一次，不同账号各算一次）", got)
	}

	ResetGlobalAliveNodes()
	if got := GlobalAliveCount(); got != 0 {
		t.Fatalf("Reset 后应为 0，实得 %d", got)
	}
}

// TestApplyNodeToStatsCountries 验证地区解析取的是节点名里的宣称地区。
func TestApplyNodeToStatsCountries(t *testing.T) {
	cases := []struct {
		name     string
		wantCode string
	}{
		{"🇹🇼TW²_1|MTF", "TW"},  // 独立 VPS 角标 ²
		{"🇸🇬SG¹_1|X", "SG"},    // CF 一致 ¹
		{"🇺🇸US⁰_1|X", "US"},    // CF 不一致 ⁰
		{"🇯🇵JP_1|X", "JP"},     // 只有国旗、无角标
		{"🏴‍☠️_1|免费A", "Unknown"}, // 海盗旗，无任何地区线索
	}
	for _, c := range cases {
		s := newAnalysisStats()
		applyNodeToStats(s, c.name, "vless", 0, false)
		if s.Countries[c.wantCode] == 0 {
			t.Errorf("节点名 %q 应解析出地区 %s，实际 Countries=%v", c.name, c.wantCode, s.Countries)
		}
		if s.Total != 1 {
			t.Errorf("节点名 %q Total 应为 1，实得 %d", c.name, s.Total)
		}
		// 核心不变式：地区分布之和必须等于 Total，
		// 否则报告里各值相加会少于 alive_count，看起来总是差几个人。
		sum := 0
		for _, v := range s.Countries {
			sum += v
		}
		if sum != s.Total {
			t.Errorf("节点名 %q 地区之和 %d != Total %d", c.name, sum, s.Total)
		}
	}
}
