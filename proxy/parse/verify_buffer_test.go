package parse

import (
	"strings"
	"testing"
)

// TestBracketKVLongLine 验证超长行（超过 bufio 默认 64KB 上限）能被完整解析。
//
// 历史 bug：ParseBracketKVProxies 用 bufio.NewScanner 的默认缓冲，
// 一整行几百 KB 的订阅内容会 ErrTooLong 提前中断，整份内容 0 节点。
// 端到端验证时该 bug 在真实订阅源上复现过（日志 token too long，已解析=0）。
//
// 本测试构造 82KB 的合法 KV 行：去掉 scanner.Buffer 那行会让它解析出 0 个节点。
func TestBracketKVLongLine(t *testing.T) {
	longPath := "/" + strings.Repeat("a", 80*1024)
	line := "Proxy = vmess, 1.2.3.4, 443, username=11111111-2222-3333-4444-555555555555, ws=true, ws-path=" + longPath
	if len(line) < 64*1024 {
		t.Fatalf("测试行 %d 字节未超过 64KB 默认缓冲，无法验证该 bug", len(line))
	}

	nodes := ParseBracketKVProxies([]byte("[Proxy]\n" + line + "\n"))
	if len(nodes) == 0 {
		t.Fatalf("超长 KV 行（%d 字节）未解析出节点——Scanner 缓冲上限问题复现", len(line))
	}
	t.Logf("超长 KV 行 %d 字节 -> 解析出 %d 个节点", len(line), len(nodes))
}
