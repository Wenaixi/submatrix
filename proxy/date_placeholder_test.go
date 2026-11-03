package proxies

import (
	"testing"
	"time"
)

func TestDynamicDateEngine(t *testing.T) {
	fixedTime := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		input    string
		has      bool
		expected string
	}{
		{"https://example.com/sub/{ymd}.yaml", true, "https://example.com/sub/20261104.yaml"},
		{"https://example.com/sub/${YYYYMMDD}.txt", true, "https://example.com/sub/20261104.txt"},
		{"https://example.com/data/%Y-%m-%d.json", true, "https://example.com/data/2026-11-04.json"},
		{"https://example.com/nodes/{YYYY}/{MM}/{DD}.yaml", true, "https://example.com/nodes/2026/11/04.yaml"},
		{"https://example.com/nodes/${yyyy}/${mm}/${dd}.txt", true, "https://example.com/nodes/2026/11/04.txt"},
		{"https://example.com/nodes/[yyyy][mm][dd].txt", true, "https://example.com/nodes/20261104.txt"},
		{"https://example.com/static/clash.yaml", false, "https://example.com/static/clash.yaml"},
	}

	for _, c := range cases {
		has := hasDatePlaceholder(c.input)
		if has != c.has {
			t.Errorf("hasDatePlaceholder(%q) = %v, want %v", c.input, has, c.has)
		}
		if has {
			res := replaceDatePlaceholders(c.input, fixedTime)
			if res != c.expected {
				t.Errorf("replaceDatePlaceholders(%q) = %q, want %q", c.input, res, c.expected)
			}
		}
	}
}
