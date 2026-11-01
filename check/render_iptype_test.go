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
		mustHave    string
	}{
		{
			name: "家宽节点追加家宽标签",
			result: Result{
				Proxy:  map[string]any{"name": "HK-Node-01"},
				IPType: "家宽",
			},
			expectedTag: "HK-Node-01|家宽",
			mustHave:    "家宽",
		},
		{
			name: "机房节点追加机房标签",
			result: Result{
				Proxy:  map[string]any{"name": "US-Node-02"},
				IPType: "机房",
			},
			expectedTag: "US-Node-02|机房",
			mustHave:    "机房",
		},
		{
			name: "未知类型不追加任何标签",
			result: Result{
				Proxy:  map[string]any{"name": "JP-Node-03"},
				IPType: "",
			},
			expectedTag: "JP-Node-03",
			mustHave:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := RenderName(tt.result, false)
			if rendered != tt.expectedTag {
				t.Errorf("期望渲染名字 %q, 得到 %q", tt.expectedTag, rendered)
			}
			if tt.mustHave != "" && !strings.Contains(rendered, tt.mustHave) {
				t.Errorf("渲染名字 %q 应包含 %q", rendered, tt.mustHave)
			}
		})
	}
}
