package utils

import "testing"

// 凭证指纹去重极端情况测试：用户最关心的「同 IP 不同账号必须保留」

func TestGenerateProxyKeySameIPDifferentCredential(t *testing.T) {
	// 同 server/port/type，不同 password —— 必须是不同 key（各自保留）
	a := map[string]any{"server": "1.1.1.1", "port": 443, "type": "vmess", "password": "aaa"}
	b := map[string]any{"server": "1.1.1.1", "port": 443, "type": "vmess", "password": "bbb"}
	if GenerateProxyKey(a) == GenerateProxyKey(b) {
		t.Fatalf("同 IP 不同密码必须不同 key: %s == %s", GenerateProxyKey(a), GenerateProxyKey(b))
	}
}

func TestGenerateProxyKeySameCredentialDifferentIP(t *testing.T) {
	// 不同 server，同密码 —— 不同 key
	a := map[string]any{"server": "1.1.1.1", "port": 443, "type": "vmess", "password": "x"}
	b := map[string]any{"server": "2.2.2.2", "port": 443, "type": "vmess", "password": "x"}
	if GenerateProxyKey(a) == GenerateProxyKey(b) {
		t.Fatalf("不同 IP 必须不同 key")
	}
}

func TestGenerateProxyKeySameNodeSameKey(t *testing.T) {
	// 完全相同节点 —— 相同 key（可去重）
	a := map[string]any{"server": "1.1.1.1", "port": 443, "type": "vmess", "password": "x", "tls": true}
	b := map[string]any{"server": "1.1.1.1", "port": 443, "type": "vmess", "password": "x", "tls": true}
	if GenerateProxyKey(a) != GenerateProxyKey(b) {
		t.Fatalf("完全相同的节点必须同 key")
	}
}

func TestGenerateProxyKeyTLSDifference(t *testing.T) {
	// 同 IP/账号，但 tls 开关不同 —— 不同 key（ws vs wss 是两码事）
	a := map[string]any{"server": "1.1.1.1", "port": 443, "type": "vmess", "password": "x", "tls": true}
	b := map[string]any{"server": "1.1.1.1", "port": 443, "type": "vmess", "password": "x", "tls": false}
	if GenerateProxyKey(a) == GenerateProxyKey(b) {
		t.Fatalf("tls 开关不同必须不同 key")
	}
}

func TestGenerateProxyKeyUUIDVsPassword(t *testing.T) {
	// vmess 用 uuid，trojan 用 password —— 跨协议同 IP 不误并
	vmess := map[string]any{"server": "1.1.1.1", "port": 443, "type": "vmess", "uuid": "abc"}
	trojan := map[string]any{"server": "1.1.1.1", "port": 443, "type": "trojan", "password": "abc"}
	if GenerateProxyKey(vmess) == GenerateProxyKey(trojan) {
		t.Fatalf("不同协议必须不同 key")
	}
}

func TestGenerateProxyKeyEmptyCredential(t *testing.T) {
	// 无凭证节点（如纯 http 匿名代理）—— 不应 panic，且类型字段兜底
	anon := map[string]any{"server": "1.1.1.1", "port": 8080, "type": "http"}
	k := GenerateProxyKey(anon)
	if k == "" {
		t.Fatalf("空凭证节点 key 不能为空")
	}
	// 与带凭证的同服务区分
	auth := map[string]any{"server": "1.1.1.1", "port": 8080, "type": "http", "username": "u"}
	if k == GenerateProxyKey(auth) {
		t.Fatalf("匿名与有账号的必须不同 key")
	}
}

func TestGenerateProxyKeyServerCaseInsensitive(t *testing.T) {
	// server 大小写不同应归一（cn 域名大小写不敏感）
	a := map[string]any{"server": "Example.COM", "port": 443, "type": "vmess", "password": "x"}
	b := map[string]any{"server": "example.com", "port": 443, "type": "vmess", "password": "x"}
	if GenerateProxyKey(a) != GenerateProxyKey(b) {
		t.Fatalf("server 大小写应归一")
	}
}

func TestDeduplicateNodesKeepsSameIPDiffCredential(t *testing.T) {
	// 集成验证：DeduplicateNodes 对同 IP 不同账号保留两个
	nodes := []map[string]any{
		{"name": "A", "server": "1.1.1.1", "port": 443, "type": "vmess", "password": "aaa"},
		{"name": "B", "server": "1.1.1.1", "port": 443, "type": "vmess", "password": "bbb"},
		{"name": "C", "server": "1.1.1.1", "port": 443, "type": "vmess", "password": "aaa"}, // 与 A 重复
	}
	got := DeduplicateNodes(nodes)
	if len(got) != 2 {
		t.Fatalf("应保留 2 个（A/B），实际 %d", len(got))
	}
}