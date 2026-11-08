package save

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/Wenaixi/submatrix/check"
	"github.com/Wenaixi/submatrix/config"
)

func TestMergeMihomoTemplateKeepsRules(t *testing.T) {
	template := []byte("proxy-groups:\n  - name: auto\n    type: select\n    proxies:\n      - DIRECT\nrules:\n  - MATCH,DIRECT\n")
	proxies := []map[string]any{
		{
			"name":   "node-a",
			"type":   "ss",
			"server": "1.1.1.1",
			"port":   443,
		},
	}

	data, err := mergeMihomoTemplate(template, proxies)
	if err != nil {
		t.Fatalf("mergeMihomoTemplate returned error: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse merged yaml: %v", err)
	}

	if _, ok := parsed["proxy-groups"]; !ok {
		t.Fatalf("expected merged yaml to contain proxy-groups")
	}
	if _, ok := parsed["rules"]; !ok {
		t.Fatalf("expected merged yaml to contain rules")
	}
}

func TestMergeMihomoTemplateEmptyProxies(t *testing.T) {
	// 极端：空节点列表合并，不应 panic 且保留模板
	template := []byte("proxy-groups:\n  - name: auto\nrules:\n  - MATCH,DIRECT\n")
	data, err := mergeMihomoTemplate(template, nil)
	if err != nil {
		t.Fatalf("mergeMihomoTemplate(nil) error: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("mergeMihomoTemplate(nil) 返回空")
	}
}

func TestSaveGroupAllSkipped(t *testing.T) {
	// 极端：全部节点 skipped=true（被过滤协议），
	// all-raw.yaml 应保留全部，all.yaml 应为空
	if config.GlobalConfig == nil {
		config.GlobalConfig = &config.Config{}
	}
	original := *config.GlobalConfig
	t.Cleanup(func() { *config.GlobalConfig = original })
	outDir := t.TempDir()
	config.GlobalConfig.OutputDir = outDir
	config.GlobalConfig.ConfigDir = t.TempDir()
	config.GlobalConfig.RenameNode = false

	// 用 check.Result 构造真实结果
	results := []check.Result{
		{Proxy: map[string]any{"name": "http-a", "type": "http", "server": "1.1.1.1", "port": 80, "skipped": true}},
		{Proxy: map[string]any{"name": "http-b", "type": "http", "server": "2.2.2.2", "port": 80, "skipped": true}},
	}

	grp := config.ResolvedGroupConfig{
		Name:            "skip-all",
		FilterProtocols: []string{"http"},
		RulePreset:      config.RulePresetDirectPure, // 免模板融合
	}
	SaveGroup(grp, results, nil)

	groupDir := filepath.Join(outDir, "groups", "skip-all")

	// all-raw.yaml 应含 2 个节点
	rawData, err := os.ReadFile(filepath.Join(groupDir, "all-raw.yaml"))
	if err != nil {
		t.Fatalf("all-raw.yaml 缺失: %v", err)
	}
	var rawDoc map[string]any
	if err := yaml.Unmarshal(rawData, &rawDoc); err != nil {
		t.Fatalf("all-raw.yaml 解析失败: %v", err)
	}
	if n := len(rawDoc["proxies"].([]any)); n != 2 {
		t.Fatalf("all-raw.yaml 应含 2 节点, 实际 %d", n)
	}

	// all.yaml 应为空（无已检测节点）
	allData, err := os.ReadFile(filepath.Join(groupDir, "all.yaml"))
	if err != nil {
		t.Fatalf("all.yaml 缺失: %v", err)
	}
	var allDoc map[string]any
	if err := yaml.Unmarshal(allData, &allDoc); err != nil {
		t.Fatalf("all.yaml 解析失败: %v", err)
	}
	if n := len(allDoc["proxies"].([]any)); n != 0 {
		t.Fatalf("all.yaml 应含 0 节点（全部 skipped）, 实际 %d", n)
	}
}

func TestSaveGroupSuccessLimitTakesRaw(t *testing.T) {
	// 极端：SuccessLimit=1 但所有节点 skipped ——
	// all-raw 不受限全量，all 为空
	if config.GlobalConfig == nil {
		config.GlobalConfig = &config.Config{}
	}
	original := *config.GlobalConfig
	t.Cleanup(func() { *config.GlobalConfig = original })
	outDir := t.TempDir()
	config.GlobalConfig.OutputDir = outDir
	config.GlobalConfig.ConfigDir = t.TempDir()

	results := []check.Result{
		{Proxy: map[string]any{"name": "a", "type": "http", "server": "1.1.1.1", "port": 80, "skipped": true}},
		{Proxy: map[string]any{"name": "b", "type": "http", "server": "2.2.2.2", "port": 80, "skipped": true}},
	}
	grp := config.ResolvedGroupConfig{
		Name:            "limit-skip",
		FilterProtocols: []string{"http"},
		RulePreset:      config.RulePresetDirectPure,
		SuccessLimit:    1,
	}
	SaveGroup(grp, results, nil)

	groupDir := filepath.Join(outDir, "groups", "limit-skip")
	rawData, _ := os.ReadFile(filepath.Join(groupDir, "all-raw.yaml"))
	var rawDoc map[string]any
	_ = yaml.Unmarshal(rawData, &rawDoc)
	if n := len(rawDoc["proxies"].([]any)); n != 2 {
		t.Fatalf("all-raw.yaml 应含 2 节点（不受 limit 限制）, 实际 %d", n)
	}
}