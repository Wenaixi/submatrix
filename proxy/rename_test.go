package proxies

import (
	"testing"
)

// enhancedTag 返回测试要用的开关值，并重置序号计数器。
func enhancedTag(t *testing.T, on bool) bool {
	t.Helper()
	ResetRenameCounter()
	return on
}

func TestRenameWithoutEnhancedTag(t *testing.T) {
	on := enhancedTag(t, false)

	got := Rename("HK", "HK", on)
	if got != "🇭🇰HK_1" {
		t.Fatalf("Rename(HK, HK) = %q, want %q", got, "🇭🇰HK_1")
	}

	// 同一地区再次出现时序号递增
	if got := Rename("HK", "HK", on); got != "🇭🇰HK_2" {
		t.Fatalf("第二次 Rename = %q, want %q", got, "🇭🇰HK_2")
	}
}

func TestRenameWithEnhancedTag(t *testing.T) {
	on := enhancedTag(t, true)

	cases := []struct {
		name      string
		country   string
		tag       string
		want      string
	}{
		{"非 CF 节点", "HK", "HK²", "🇭🇰HK²_1"},
		{"CF 位置一致", "JP", "JP¹⁺", "🇯🇵JP¹⁺_1"},
		{"CF 位置不一致", "SG", "SG¹-US⁰", "🇸🇬SG¹-US⁰_1"},
	}
	for _, tc := range cases {
		ResetRenameCounter()
		if got := Rename(tc.country, tc.tag, on); got != tc.want {
			t.Errorf("%s: Rename(%q, %q) = %q, want %q", tc.name, tc.country, tc.tag, got, tc.want)
		}
	}
}

// enhanced-tag 开启但上游没给出带角标的位置标签时，补 ˣ 后缀以示区分。
func TestRenameWithEnhancedTagFallsBackToX(t *testing.T) {
	on := enhancedTag(t, true)

	if got := Rename("HK", "", on); got != "🇭🇰HKˣ_1" {
		t.Fatalf("Rename(HK, \"\") = %q, want %q", got, "🇭🇰HKˣ_1")
	}
}

// 位置查询失败时不得以空国家拼接出无法识别的名字，应回落为占位旗。
func TestRenameUnknownCountry(t *testing.T) {
	on := enhancedTag(t, false)

	got := Rename("", "", on)
	if got != "🏴‍☠_1" {
		t.Fatalf("Rename(\"\", \"\") = %q, want %q", got, "🏴‍☠_1")
	}
}

func TestCountryCodeToFlag(t *testing.T) {
	cases := map[string]string{
		"HK": "🇭🇰",
		"US": "🇺🇸",
		"SG": "🇸🇬",
		"":   "🏴‍☠",
		"H":  "🏴‍☠",
		"HKX": "🏴‍☠",
	}
	for code, want := range cases {
		if got := CountryCodeToFlag(code); got != want {
			t.Errorf("CountryCodeToFlag(%q) = %q, want %q", code, got, want)
		}
	}
}
