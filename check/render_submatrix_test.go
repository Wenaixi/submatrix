package check

import (
	"strings"
	"testing"
)

func TestRenderName_SubMatrix(t *testing.T) {
	tests := []struct {
		name     string
		parts    NameParts
		expected string
	}{
		{
			name: "家宽节点打标且保留原名",
			parts: NameParts{
				Base:         "🇭🇰HK_1",
				IPType:       "家宽",
				SpeedTag:     "2.8MB/s",
				SubTag:       "专属",
				OriginalName: "香港 01 | 优质专线 [4K]",
			},
			expected: "🇭🇰HK_1|家宽|2.8MB/s|专属 (原名: 香港 01 | 优质专线 [4K])",
		},
		{
			name: "机房节点打标且保留原名",
			parts: NameParts{
				Base:         "🇯🇵JP_1",
				IPType:       "机房",
				SpeedTag:     "1.5MB/s",
				SubTag:       "免费",
				OriginalName: "Japan Tokyo 02",
			},
			expected: "🇯🇵JP_1|机房|1.5MB/s|免费 (原名: Japan Tokyo 02)",
		},
		{
			name: "移动蜂窝节点打标且不保留原名",
			parts: NameParts{
				Base:     "🇺🇸US_1",
				IPType:   "移动",
				SpeedTag: "800KB/s",
				SubTag:   "公网",
			},
			expected: "🇺🇸US_1|移动|800KB/s|公网",
		},
		{
			name: "未知类型不打标",
			parts: NameParts{
				Base:     "🇸🇬SG_1",
				IPType:   "",
				SpeedTag: "3.0MB/s",
				SubTag:   "vip",
			},
			expected: "🇸🇬SG_1|3.0MB/s|vip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.parts.String()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRenderOfflineNode(t *testing.T) {
	origName := "美国 05 | 游戏专线"
	subTag := "专属"

	got := RenderOfflineNode(origName, subTag)
	want := "[离线] 美国 05 | 游戏专线|专属"

	if got != want {
		t.Errorf("RenderOfflineNode() = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, "[离线]") {
		t.Errorf("离线节点应具有 [离线] 前缀，实际为: %s", got)
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
