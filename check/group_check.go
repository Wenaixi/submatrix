package check

import (
	"github.com/Wenaixi/submatrix/config"
)

// FindGroupByTagOrURL 在所有有效分组中匹配所属组（转发至 config.FindGroupByTagOrURL）
// 保留为兼容外部调用；组独立检测流程中节点必然属于本组，不再依赖此函数。
func FindGroupByTagOrURL(tag, subURL string) config.ResolvedGroupConfig {
	return config.FindGroupByTagOrURL(tag, subURL)
}
