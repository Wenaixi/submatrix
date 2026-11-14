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

	// 1. 测试未定义任何 groups 时，默认组自动继承 filter_protocols = ["http"] 与 rule_preset = "acl4ssr"
	groups := ResolveAllGroups(global)
	if len(groups) != 1 {
		t.Fatalf("expected 1 default group, got %d", len(groups))
	}
	def := groups[0]
	if !reflect.DeepEqual(def.FilterProtocols, []string{"http"}) {
		t.Errorf("expected default FilterProtocols = [http], got %v", def.FilterProtocols)
	}
	if def.RulePreset != RulePresetACL4SSR {
		t.Errorf("expected default RulePreset = acl4ssr, got %s", def.RulePreset)
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
	if gb.RulePreset != RulePresetACL4SSR {
		t.Errorf("expected custom-b fallback RulePreset acl4ssr, got %s", gb.RulePreset)
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

func TestResolveAllGroupsPriorityEdge(t *testing.T) {
	// 极端：负数 priority、同 priority、零值混合排序
	g := Config{
		Groups: []GroupConfig{
			{Name: "neg", URLs: []string{"https://neg"}, Priority: -5},
			{Name: "zero", URLs: []string{"https://zero"}, Priority: 0},
			{Name: "same1", URLs: []string{"https://s1"}, Priority: 10},
			{Name: "same2", URLs: []string{"https://s2"}, Priority: 10},
			{Name: "large", URLs: []string{"https://lg"}, Priority: 900},
		},
	}
	got := ResolveAllGroups(g)
	want := []string{"neg", "zero", "same1", "same2", "large"} // 负数最前，同值稳定
	for i, w := range want {
		if got[i].Name != w {
			t.Fatalf("极端排序错: got[%d]=%s want %s (全序 %v)", i, got[i].Name, w, names(got))
		}
	}
}

func TestFindGroupByTagOrURLNoMatch(t *testing.T) {
	// 极端：无匹配的 tag/URL 应回退第一个组而非 panic
	cfg := Config{
		Groups: []GroupConfig{
			{Name: "g1", URLs: []string{"https://g1"}},
		},
	}
	GlobalConfig = &cfg
	defer func() { GlobalConfig = &Config{} }()

	got := FindGroupByTagOrURL("不存在", "https://nope")
	if got.Name != "g1" {
		t.Fatalf("无匹配应回退第一个组 g1, 实际 %s", got.Name)
	}
}

// TestResolveGroupEnhancedTagInheritance 校验 enhanced-tag 的组级继承：
// 未配置时跟随全局；显式 true 可打开；显式 false 必须能覆盖全局的 true。
func TestResolveGroupEnhancedTagInheritance(t *testing.T) {
	on, off := true, false

	cases := []struct {
		name   string
		global bool
		group  *bool
		want   bool
	}{
		{"全局开、组未配", true, nil, true},
		{"全局关、组未配", false, nil, false},
		{"全局关、组显式开", false, &on, true},
		{"全局开、组显式关", true, &off, false},
	}
	for _, tc := range cases {
		got := ResolveGroup(GroupConfig{Name: "g", EnhancedTag: tc.group}, Config{EnhancedTag: tc.global})
		if got.EnhancedTag != tc.want {
			t.Errorf("%s: EnhancedTag = %v, want %v", tc.name, got.EnhancedTag, tc.want)
		}
	}

	// 未配置任何 groups 时，default 组等价于全局配置
	for _, global := range []bool{true, false} {
		groups := ResolveAllGroups(Config{EnhancedTag: global})
		if len(groups) != 1 || groups[0].EnhancedTag != global {
			t.Errorf("default 组应继承全局 EnhancedTag=%v，实际 %v", global, groups[0].EnhancedTag)
		}
	}
}
