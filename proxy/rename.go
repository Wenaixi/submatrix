package proxies

import (
	"strconv"
	"sync"
)

var (
	counter     = make(map[string]int)
	counterLock = sync.Mutex{}
)

// Rename 按国家代码生成带国旗与序号的节点名。
// enhancedTag 由调用方传入组级配置，决定是否追加位置角标。
func Rename(name, countryCodeTag string, enhancedTag bool) string {
	flag := CountryCodeToFlag(name)

	key, label := name, name
	if enhancedTag {
		if countryCodeTag != "" {
			key, label = countryCodeTag, countryCodeTag
		} else if name != "" {
			// 添加 "ˣ" 角标, 例如: "HKˣ", 以做区分
			label = name + "ˣ"
			key = label
		}
	}

	counterLock.Lock()
	counter[key]++
	n := counter[key]
	counterLock.Unlock()

	return flag + label + "_" + strconv.Itoa(n)
}

// ResetRenameCounter 将所有计数器重置为 0
func ResetRenameCounter() {
	counterLock.Lock()
	defer counterLock.Unlock()

	counter = make(map[string]int)
}

func CountryCodeToFlag(code string) string {
	if len(code) != 2 {
		return "🏴‍☠"
	}

	code = string([]rune(code)[0]&^0x20) + string([]rune(code)[1]&^0x20) // 转成大写（ASCII 位运算）

	r1 := rune(code[0]-'A') + 0x1F1E6
	r2 := rune(code[1]-'A') + 0x1F1E6

	return string([]rune{r1, r2})
}
