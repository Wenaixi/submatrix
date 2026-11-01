package check

import (
	"testing"
)

func TestDeduplicateProxies(t *testing.T) {
	nodes := []map[string]any{
		{"name": "Node A", "server": "1.1.1.1", "port": 443, "type": "vmess"},
		{"name": "Node A dup", "server": "1.1.1.1", "port": 443, "type": "vmess"},
		{"name": "Node B", "server": "1.1.1.1", "port": 8443, "type": "vmess"},
		{"name": "Node C", "server": "2.2.2.2", "port": 443, "type": "vless"},
	}

	deduped := DeduplicateProxies(nodes)
	if len(deduped) != 3 {
		t.Fatalf("expected 3 nodes after deduplication, got %d", len(deduped))
	}
}

func TestRenderOfflineNode(t *testing.T) {
	node := map[string]any{
		"name":     "US-Node-01",
		"raw_name": "US-Node-01",
		"server":   "1.2.3.4",
	}

	offline := RenderOfflineNode(node, "免费B")
	expected := "[离线] US-Node-01|免费B"
	if offline["name"] != expected {
		t.Errorf("expected offline name %q, got %q", expected, offline["name"])
	}
	if offline["offline"] != true {
		t.Errorf("expected offline flag true")
	}
}

func TestExtractRootOriginalName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"香港 01", "香港 01"},
		{"❓Other_1|免费A (原名: 香港 01)", "香港 01"},
		{"❓Other_100|免费A (原名: ❓Other_1|免费A (原名: 真实原始名))", "真实原始名"},
	}

	for _, tc := range cases {
		got := extractRootOriginalName(tc.input)
		if got != tc.want {
			t.Errorf("extractRootOriginalName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
