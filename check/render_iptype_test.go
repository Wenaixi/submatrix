package check

import (
	"strings"
	"testing"

	"github.com/beck-8/subs-check/config"
)

func TestRenderNameIPType(t *testing.T) {
	config.GlobalConfig.RenameNode = false

	tests := []struct {
		name        string
		result      Result
		expectedTag string
		mustNotHave string
	}{
		{
			name: "家宽节点追加家宽标签",
			result: Result{
				Proxy:  map[string]any{"name": "HK-Node-01"},
				IPType: "家宽",
			},
			expectedTag: "HK-Node-01|家宽",
			mustNotHave: "机房",
		},
		{
			name: "机房节点保持清爽不追加机房标签",
			result: Result{
				Proxy:  map[string]any{"name": "US-Node-02"},
				IPType: "机房",
			},
			expectedTag: "US-Node-02",
			mustNotHave: "机房",
		},
		{
			name: "未知类型不追加任何标签",
			result: Result{
				Proxy:  map[string]any{"name": "JP-Node-03"},
				IPType: "",
			},
			expectedTag: "JP-Node-03",
			mustNotHave: "家宽",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := RenderName(tt.result, false)
			if rendered != tt.expectedTag {
				t.Errorf("期望渲染名字 %q, 得到 %q", tt.expectedTag, rendered)
			}
			if tt.mustNotHave != "" && strings.Contains(rendered, tt.mustNotHave) {
				t.Errorf("渲染名字 %q 不应包含 %q", rendered, tt.mustNotHave)
			}
		})
	}
}
