package config

import (
	"testing"
)

// TestBackwardCompatibility 测试仅有旧版 sub-urls 时自动包装为 default 组
func TestBackwardCompatibility(t *testing.T) {
	cfg := &Config{
		Timeout:    3000,
		MinSpeed:   10,
		RenameNode: true,
		SubUrls: []string{
			"https://example.com/sub1#tag1",
			"https://example.com/sub2",
		},
	}

	groups := ResolveAllGroups(cfg)
	if len(groups) != 1 {
		t.Fatalf("预期生成 1 个 default 组，实际得到: %d", len(groups))
	}

	def := groups[0]
	if def.Name != "default" {
		t.Errorf("默认组名应为 default，实际为: %s", def.Name)
	}
	if len(def.Urls) != 2 {
		t.Errorf("默认组应包含 2 个订阅链接，实际为: %d", len(def.Urls))
	}
	if def.Timeout != 3000 {
		t.Errorf("预期继承全局超时 3000，实际为: %d", def.Timeout)
	}
	if def.MinSpeed != 10 {
		t.Errorf("预期继承全局最低速度 10，实际为: %d", def.MinSpeed)
	}
	if !def.DropOnFail {
		t.Errorf("默认组应默认开启失败丢弃 (DropOnFail == true)")
	}
	if !def.CheckAlive {
		t.Errorf("默认组应开启测活 (CheckAlive == true)")
	}
}

// TestGroupInheritanceAndOverrides 测试组级参数覆写与全局继承
func TestGroupInheritanceAndOverrides(t *testing.T) {
	dropFalse := false
	appendTrue := true
	groupTimeout := 8000
	groupTagIP := true

	cfg := &Config{
		Timeout:    4000,
		MinSpeed:   20,
		RenameNode: true,
		Groups: []GroupConfig{
			{
				Name:               "vip",
				DisplayName:        "优质专线组",
				Tag:                "专属",
				Timeout:            &groupTimeout,
				DropOnFail:         &dropFalse,
				AppendOriginalName: &appendTrue,
				TagIPType:          &groupTagIP,
				Urls: []string{
					"https://vip.example.com/sub",
				},
			},
		},
	}

	groups := ResolveAllGroups(cfg)
	if len(groups) != 1 {
		t.Fatalf("预期 1 个组，实际: %d", len(groups))
	}

	vip := groups[0]
	if vip.Name != "vip" {
		t.Errorf("组名不匹配: %s", vip.Name)
	}
	if vip.Tag != "专属" {
		t.Errorf("组标签不匹配: %s", vip.Tag)
	}
	if vip.Timeout != 8000 {
		t.Errorf("组级超时应覆写为 8000，实际: %d", vip.Timeout)
	}
	if vip.MinSpeed != 20 {
		t.Errorf("组未配置最低速度应继承全局 20，实际: %d", vip.MinSpeed)
	}
	if vip.DropOnFail != false {
		t.Errorf("vip 组 DropOnFail 应为 false (免死保留)")
	}
	if !vip.AppendOriginalName {
		t.Errorf("vip 组 AppendOriginalName 应为 true")
	}
	if !vip.TagIPType {
		t.Errorf("vip 组 TagIPType 应为 true")
	}
}

// TestAggregationGroupLogic 测试纯透传聚合组的联动规则
func TestAggregationGroupLogic(t *testing.T) {
	checkAliveFalse := false
	renameFalse := false

	cfg := &Config{
		RenameNode: true,
		Groups: []GroupConfig{
			{
				Name:         "all-in-one",
				CheckAlive:   &checkAliveFalse,
				RenameNode:   &renameFalse,
				OverwriteURL: "http://127.0.0.1:8199/rules.yaml",
				Urls: []string{
					"http://127.0.0.1:8199/group/vip/all.yaml",
					"http://127.0.0.1:8199/group/free/all.yaml",
				},
			},
		},
	}

	groups := ResolveAllGroups(cfg)
	all := groups[0]

	if all.CheckAlive {
		t.Errorf("聚合组 CheckAlive 应为 false")
	}
	if all.CheckSpeed {
		t.Errorf("未测活组 CheckSpeed 应自动置为 false")
	}
	if all.RenameNode {
		t.Errorf("聚合组 RenameNode 应为 false")
	}
	if all.OverwriteURL != "http://127.0.0.1:8199/rules.yaml" {
		t.Errorf("规则覆写 URL 不匹配")
	}
}

// TestEdgeCases 空组与空URL容错
func TestEdgeCases(t *testing.T) {
	cfg := &Config{
		Groups: []GroupConfig{
			{
				Name: "",
				Urls: []string{"   ", ""},
			},
		},
	}

	groups := ResolveAllGroups(cfg)
	if len(groups) != 1 {
		t.Fatalf("预期 1 个组")
	}
	g := groups[0]
	if g.Name == "" {
		t.Errorf("空组名应自动赋予合法回退名 (如 group_1)")
	}
	if len(g.Urls) != 0 {
		t.Errorf("全空白 URLs 应被完全过滤，当前数量: %d", len(g.Urls))
	}
}
