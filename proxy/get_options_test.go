package proxies

import "testing"

func TestParseSubTagAndOptions(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedTag   string
		expectedSkip  bool
	}{
		{
			name:         "普通标签无参数",
			input:        "github免费1",
			expectedTag:  "github免费1",
			expectedSkip: false,
		},
		{
			name:         "带 skip-check 参数",
			input:        "hneko1&skip-check",
			expectedTag:  "hneko1",
			expectedSkip: true,
		},
		{
			name:         "带 nofilter 参数",
			input:        "hneko1&nofilter",
			expectedTag:  "hneko1",
			expectedSkip: true,
		},
		{
			name:         "带 skip-check=true 参数",
			input:        "myairport&skip-check=true",
			expectedTag:  "myairport",
			expectedSkip: true,
		},
		{
			name:         "带多个参数混杂空格与大小写",
			input:        "airport&foo=bar&SKIP-CHECK&baz=1",
			expectedTag:  "airport",
			expectedSkip: true,
		},
		{
			name:         "空字符串",
			input:        "",
			expectedTag:  "",
			expectedSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, skip := ParseSubTagAndOptions(tt.input)
			if tag != tt.expectedTag {
				t.Errorf("tag 不匹配: 期望 %q, 得到 %q", tt.expectedTag, tag)
			}
			if skip != tt.expectedSkip {
				t.Errorf("skip 不匹配: 期望 %v, 得到 %v", tt.expectedSkip, skip)
			}
		})
	}
}
