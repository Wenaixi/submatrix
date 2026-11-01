package check

import (
	"testing"

	"github.com/beck-8/subs-check/config"
)

// TestCheckGroup_BypassAliveAndSpeed 验证纯透传聚合组：免测活、免测速、不改名
func TestCheckGroup_BypassAliveAndSpeed(t *testing.T) {
	grp := config.ResolvedGroupConfig{
		Name:        "all-in-one",
		DisplayName: "全量聚合组",
		Tag:         "聚合",
		CheckAlive:  false,
		CheckSpeed:  false,
		RenameNode:  false,
	}

	rawNodes := []map[string]any{
		{"name": "香港专线 01", "type": "ss", "server": "1.1.1.1", "port": 8388},
		{"name": "日本高速 02", "type": "vmess", "server": "2.2.2.2", "port": 443},
	}

	results, err := ExecuteGroupProxies(grp, rawNodes)
	if err != nil {
		t.Fatalf("ExecuteGroupProxies 报错: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("预期返回 2 个节点，实际返回: %d", len(results))
	}

	// 验证未改名且原汁原味保留
	if results[0].Proxy["name"] != "香港专线 01" {
		t.Errorf("聚合组节点 1 原名被篡改: %v", results[0].Proxy["name"])
	}
	if results[1].Proxy["name"] != "日本高速 02" {
		t.Errorf("聚合组节点 2 原名被篡改: %v", results[1].Proxy["name"])
	}
	if results[0].Speed != 0 || results[1].Speed != 0 {
		t.Errorf("未测速组节点速度应为 0")
	}
}

// TestCheckGroup_DropOnFailFalse 验证免死组失败不淘汰，保留为[离线]
func TestCheckGroup_DropOnFailFalse(t *testing.T) {
	grp := config.ResolvedGroupConfig{
		Name:               "vip",
		Tag:                "专属",
		CheckAlive:         true,
		DropOnFail:         false, // 免死金牌
		RenameNode:         true,
		AppendOriginalName: true,
	}

	// 模拟测活失败的离线节点
	failedProxy := map[string]any{
		"name":   "美国故障节点",
		"server": "9.9.9.9",
		"port":   8080,
		"type":   "trojan",
	}

	res := HandleFailedAliveNode(grp, failedProxy)
	if res == nil {
		t.Fatalf("DropOnFail 为 false 时不应返回 nil (不应丢弃)")
	}

	name, ok := res.Proxy["name"].(string)
	if !ok {
		t.Fatalf("Proxy 缺少合法 name")
	}

	if name != "[离线] 美国故障节点|专属" {
		t.Errorf("离线节点名称格式不匹配: %s", name)
	}
}

// TestDeduplicateProxies 验证节点物理特征去重
func TestDeduplicateProxies(t *testing.T) {
	nodes := []map[string]any{
		{"name": "节点 A", "server": "1.2.3.4", "port": 443, "type": "vmess"},
		{"name": "节点 A 重命名版", "server": "1.2.3.4", "port": 443, "type": "vmess"}, // 重复节点
		{"name": "节点 B", "server": "5.6.7.8", "port": 443, "type": "vmess"},
	}

	deduped := DeduplicateProxies(nodes)
	if len(deduped) != 2 {
		t.Fatalf("预期去重后保留 2 个节点，实际为: %d", len(deduped))
	}
}
