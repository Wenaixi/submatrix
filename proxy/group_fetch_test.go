package proxies

import (
	"strings"
	"testing"

	"github.com/Wenaixi/submatrix/config"
)

// withGlobalOpts 临时设置全局配置并在测试结束后还原。
func withGlobalOpts(t *testing.T, cfg config.Config) {
	t.Helper()
	old := config.GlobalConfig
	config.GlobalConfig = &cfg
	t.Cleanup(func() { config.GlobalConfig = old })
}

func TestExpandGroupRelativeURL(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		public  string
		apiKey  string
		want    string
		wantErr bool
	}{
		{
			name: "相对路径拼接公网域名与 token",
			raw:  "/group/free-a/all.yaml",
			public: "https://sub-s.wenxi.dev",
			apiKey: "wenxi-admin",
			want:  "https://sub-s.wenxi.dev/group/free-a/all.yaml?token=wenxi-admin",
		},
		{
			name: "相对路径且 public-url 带尾部斜杠",
			raw:  "/group/b/all.yaml",
			public: "https://example.com/",
			apiKey: "",
			want:  "https://example.com/group/b/all.yaml",
		},
		{
			name: "相对路径但 public-url 为空",
			raw:  "/group/a/all.yaml",
			public: "",
			wantErr: true,
		},
		{
			name: "绝对地址保留原样",
			raw:  "https://raw.githubusercontent.com/foo/bar/main/clash.yaml",
			public: "https://sub-s.wenxi.dev",
			apiKey: "k",
			want:  "https://raw.githubusercontent.com/foo/bar/main/clash.yaml",
		},
		{
			name: "拒绝指向本机 127.0.0.1 的绝对地址",
			raw:  "http://127.0.0.1:8199/group/a/all.yaml?token=x",
			public: "https://sub-s.wenxi.dev",
			wantErr: true,
		},
		{
			name: "拒绝 public-url 是本机地址",
			raw:  "/group/a/all.yaml",
			public: "http://127.0.0.1:8199",
			wantErr: true,
		},
		{
			name: "非路径非绝对地址",
			raw:  "group/a/all.yaml",
			public: "https://x.com",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withGlobalOpts(t, config.Config{PublicURL: tc.public, APIKey: tc.apiKey})
			got, err := expandGroupRelativeURL(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错但成功返回: %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expandGroupRelativeURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestExpandGroupRelativeURLRejectsLocalhost(t *testing.T) {
	withGlobalOpts(t, config.Config{PublicURL: "https://sub-s.wenxi.dev", APIKey: "k"})

	for _, raw := range []string{
		"http://localhost:8199/all.yaml",
		"http://127.0.0.1:8199/all.yaml",
		"http://[::1]:8199/all.yaml",
	} {
		if _, err := expandGroupRelativeURL(raw); err == nil {
			t.Errorf("应拒绝本机地址: %s", raw)
		}
	}
}

func TestFetchGroupProxiesEmptyURLs(t *testing.T) {
	withGlobalOpts(t, config.Config{PublicURL: "https://sub-s.wenxi.dev", APIKey: "k"})

	_, _, err := FetchGroupProxies(config.ResolvedGroupConfig{Name: "empty", URLs: nil})
	if err == nil || !strings.Contains(err.Error(), "没有配置") {
		t.Fatalf("空 URL 组应报错, got %v", err)
	}
}