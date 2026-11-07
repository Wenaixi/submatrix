package config

import (
	"reflect"
	"testing"
)

func TestResolveAllGroups_DefaultAndProtocols(t *testing.T) {
	global := Config{
		Timeout:            3000,
		MinSpeed:           1024,
		SubUrls:            []string{"http://legacy.url"},
		AppendOriginalName: true,
	}

	// 1. 测试未定义任何 groups 时，默认组自动继承 filter_protocols = ["http"] 与 rule_preset = "pro_full"
	groups := ResolveAllGroups(global)
	if len(groups) != 1 {
		t.Fatalf("expected 1 default group, got %d", len(groups))
	}
	def := groups[0]
	if !reflect.DeepEqual(def.FilterProtocols, []string{"http"}) {
		t.Errorf("expected default FilterProtocols = [http], got %v", def.FilterProtocols)
	}
	if def.RulePreset != RulePresetProFull {
		t.Errorf("expected default RulePreset = pro_full, got %s", def.RulePreset)
	}
	if !def.MatchesProtocol("http") || !def.MatchesProtocol("HTTP") {
		t.Errorf("expected MatchesProtocol(http) to be true")
	}
	if def.MatchesProtocol("vless") {
		t.Errorf("expected MatchesProtocol(vless) to be false")
	}

	// 2. 测试显式配置组代理、规则预设与多协议过滤
	customFilter := []string{"http", "socks5", "ssr"}
	global.Groups = []GroupConfig{
		{
			Name:            "custom-a",
			Proxy:           "http://127.0.0.1:10808",
			RulePreset:      RulePresetProLite,
			FilterProtocols: customFilter,
			URLs:            []string{"https://example.com/sub"},
		},
		{
			Name:            "custom-b",
			FilterProtocols: []string{}, // 显式允许所有协议
			URLs:            []string{"https://example.com/sub2"},
		},
	}

	groups = ResolveAllGroups(global)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	ga := groups[0]
	if ga.Proxy != "http://127.0.0.1:10808" {
		t.Errorf("expected Proxy http://127.0.0.1:10808, got %s", ga.Proxy)
	}
	if ga.RulePreset != RulePresetProLite {
		t.Errorf("expected custom-a RulePreset pro_lite, got %s", ga.RulePreset)
	}
	if !ga.MatchesProtocol("SOCKS5") || !ga.MatchesProtocol("ssr") {
		t.Errorf("expected custom-a to match socks5 and ssr")
	}

	gb := groups[1]
	if gb.RulePreset != RulePresetProFull {
		t.Errorf("expected custom-b fallback RulePreset pro_full, got %s", gb.RulePreset)
	}
}

func TestResolveAllGroupsSortsByPriority(t *testing.T) {
	g := Config{
		Groups: []GroupConfig{
			{Name: "b", URLs: []string{"https://b"}, Priority: 20},
			{Name: "a", URLs: []string{"https://a"}, Priority: 10},
			{Name: "c", URLs: []string{"https://c"}, Priority: 10}, // 同 10，应保持声明顺序
		},
	}
	got := ResolveAllGroups(g)
	want := []string{"a", "c", "b"}
	for i, w := range want {
		if got[i].Name != w {
			t.Fatalf("ResolveAllGroups 排序错: got[%d]=%s want %s (全序 %v)", i, got[i].Name, w, names(got))
		}
	}
}

func names(gs []ResolvedGroupConfig) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.Name
	}
	return out
}
