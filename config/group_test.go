package config

import "testing"

func TestGroupResolutionAndBackwardCompatibility(t *testing.T) {
	global := Config{
		RenameNode:         true,
		AppendOriginalName: true,
		TagIPType:          true,
		Timeout:            5000,
		SubUrls:            []string{"http://legacy.url"},
	}

	// 1. 测试旧版 sub-urls 自动降级解析为默认组
	groups := ResolveAllGroups(global)
	if len(groups) != 1 {
		t.Fatalf("expected 1 default group, got %d", len(groups))
	}
	if groups[0].Name != "default" || groups[0].URLs[0] != "http://legacy.url" {
		t.Errorf("unexpected default group: %+v", groups[0])
	}

	// 2. 测试组级联覆盖
	f := false
	global.Groups = []GroupConfig{
		{
			Name:       "aggregation",
			CheckAlive: &f,
			RenameNode: &f,
			URLs:       []string{"http://127.0.0.1:8199/group/a/all.yaml"},
		},
	}
	groups = ResolveAllGroups(global)
	if len(groups) != 1 {
		t.Fatalf("expected 1 resolved group, got %d", len(groups))
	}
	if groups[0].CheckAlive != false || groups[0].RenameNode != false {
		t.Errorf("group override failed: %+v", groups[0])
	}
	if groups[0].AppendOriginalName != true {
		t.Errorf("global inheritance failed: %+v", groups[0])
	}
}
