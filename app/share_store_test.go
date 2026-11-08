package app

import (
	"strings"
	"testing"
	"time"
)

// ---------- 码生成 ----------

func TestGenShareCodeUnique(t *testing.T) {
	// 生成多次应全部唯一且长度正确
	store := NewShareStore()
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		code, err := store.genShareCode()
		if err != nil {
			t.Fatalf("genShareCode error: %v", err)
		}
		if len(code) != shareCodeLen {
			t.Fatalf("码长度应 %d, 实际 %d: %s", shareCodeLen, len(code), code)
		}
		if seen[code] {
			t.Fatalf("码重复: %s", code)
		}
		seen[code] = true
	}
}

func TestGenShareCodeAlphabet(t *testing.T) {
	// 码只能在 base62 字符集内
	store := NewShareStore()
	code, err := store.genShareCode()
	if err != nil {
		t.Fatal(err)
	}
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	for _, c := range code {
		if !strings.ContainsRune(alphabet, c) {
			t.Fatalf("码含非法字符: %q", c)
		}
	}
}

// ---------- CRUD ----------

func TestShareStoreCreateAndGet(t *testing.T) {
	store := NewShareStore()
	sh, err := store.Create("free-a", "给朋友", time.Time{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := store.Get(sh.ID)
	if !ok {
		t.Fatalf("Get 未找到刚创建的分享")
	}
	if got.GroupName != "free-a" || !got.Enabled {
		t.Fatalf("字段不一致: %+v", got)
	}
}

func TestShareStoreCreateEmptyGroup(t *testing.T) {
	store := NewShareStore()
	_, err := store.Create("", "", time.Time{}, nil)
	if err == nil {
		t.Fatalf("空组名应报错")
	}
}

func TestShareStoreToggleAndDelete(t *testing.T) {
	store := NewShareStore()
	sh, _ := store.Create("free-b", "", time.Time{}, nil)

	if !store.Toggle(sh.ID, false) {
		t.Fatalf("Toggle 失败")
	}
	got, _ := store.Get(sh.ID)
	if got.Enabled {
		t.Fatalf("Toggle(false) 后应禁用")
	}

	if !store.Delete(sh.ID) {
		t.Fatalf("Delete 失败")
	}
	if _, ok := store.Get(sh.ID); ok {
		t.Fatalf("删除后不应存在")
	}
	// 重复删除应 false
	if store.Delete(sh.ID) {
		t.Fatalf("重复删除应返回 false")
	}
}

// ---------- 统计 ----------

func TestShareStatsRecordHit(t *testing.T) {
	store := NewShareStore()
	sh, _ := store.Create("free-a", "", time.Time{}, nil)

	store.RecordHit(sh.ID, "1.2.3.4", "all.yaml", "Clash/1.0")
	store.RecordHit(sh.ID, "1.2.3.4", "mihomo.yaml", "Clash/1.0")
	store.RecordHit(sh.ID, "5.6.7.8", "all.yaml", "Stash/2.0")

	st, ok := store.GetStats(sh.ID)
	if !ok {
		t.Fatal("GetStats 失败")
	}
	if st.TotalHits != 3 {
		t.Fatalf("TotalHits 应 3, 实际 %d", st.TotalHits)
	}
	if st.UniqueIPs != 2 {
		t.Fatalf("UniqueIPs 应 2, 实际 %d", st.UniqueIPs)
	}
	if st.PerFile["all.yaml"] != 2 {
		t.Fatalf("all.yaml 应 2 次, 实际 %d", st.PerFile["all.yaml"])
	}
	if len(st.IPHistory) != 3 {
		t.Fatalf("IPHistory 应 3 条, 实际 %d", len(st.IPHistory))
	}
}

func TestShareStatsRingBuffer(t *testing.T) {
	// 超过 maxIPHistory 应环形截断，只保留最近 500
	store := NewShareStore()
	sh, _ := store.Create("free-a", "", time.Time{}, nil)
	for i := 0; i < maxIPHistory+50; i++ {
		store.RecordHit(sh.ID, "1.2.3.4", "all.yaml", "")
	}
	st, _ := store.GetStats(sh.ID)
	if len(st.IPHistory) != maxIPHistory {
		t.Fatalf("环形后应 %d 条, 实际 %d", maxIPHistory, len(st.IPHistory))
	}
	// 唯一 IP 仍 1
	if st.UniqueIPs != 1 {
		t.Fatalf("唯一 IP 应 1, 实际 %d", st.UniqueIPs)
	}
}

// ---------- IP 脱敏 ----------

func TestMaskIP(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4":     "1.2.3.*",
		"203.0.113.5": "203.0.113.*",
		"2001:db8::1": "2001:*",
		"bad":         "unknown",
	}
	for in, want := range cases {
		if got := MaskIP(in); got != want {
			t.Errorf("MaskIP(%q)=%q want %q", in, got, want)
		}
	}
}

// ---------- 私有 IP 判断 ----------

func TestIsPrivateIP(t *testing.T) {
	priv := []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "172.16.0.1", "::1", "fe80::1"}
	pub := []string{"8.8.8.8", "203.0.113.5", "2606:4700::1"}

	for _, ip := range priv {
		if !isPrivateIP(ip) {
			t.Errorf("%s 应判为私有", ip)
		}
	}
	for _, ip := range pub {
		if isPrivateIP(ip) {
			t.Errorf("%s 不应判为私有", ip)
		}
	}
	// 非法值按私有处理（不可信）
	if !isPrivateIP("garbage") {
		t.Errorf("非法 IP 应判为私有（不可信）")
	}
}

// ---------- 速率限制 ----------

func TestRateLimitAllowance(t *testing.T) {
	// 单 IP 单分享在窗口内允许 limit 次，超限拒绝
	ip, code := "1.2.3.4", "testcode"
	allowed := 0
	for i := 0; i < rateLimitPerMin+20; i++ {
		if allowRequest(ip, code) {
			allowed++
		}
	}
	if allowed != rateLimitPerMin {
		t.Fatalf("应放行 %d 次, 实际 %d", rateLimitPerMin, allowed)
	}
	// 换 IP 不受影响
	if !allowRequest("9.9.9.9", code) {
		t.Fatalf("不同 IP 应继续放行")
	}
}

// ---------- 分享文件白名单 ----------

func TestShareAllowedFiles(t *testing.T) {
	ok := []string{"all.yaml", "mihomo.yaml", "all-raw.yaml", "history.yaml"}
	bad := []string{"config.yaml", "sub-store.json", "secret.txt", "../config.yaml", "sub/yaml"}

	for _, f := range ok {
		if !shareAllowedFiles[f] {
			t.Errorf("%s 应在白名单", f)
		}
	}
	for _, f := range bad {
		if shareAllowedFiles[f] {
			t.Errorf("%s 不应在白名单", f)
		}
	}
}

// ---------- 过期逻辑（辅助） ----------

func TestShareExpiry(t *testing.T) {
	store := NewShareStore()
	// 已过期分享
	past := time.Now().Add(-time.Hour)
	sh, _ := store.Create("free-a", "已过期", past, nil)
	got, _ := store.Get(sh.ID)
	if got.ExpiresAt.IsZero() || !time.Now().After(got.ExpiresAt) {
		t.Fatalf("应构造为已过期（ExpiresAt=%v）", got.ExpiresAt)
	}
}