package main

import (
	"fmt"
	"sort"

	"github.com/urfave/cli/v2"

	"github.com/Wenaixi/submatrix/config"
)

// presetCatalog 内置规则集目录：ID → 名称与说明。
// 模板文件实际从 assets/ 或 output/ 加载，见 save.fetchTemplateByPreset。
var presetCatalog = map[string][2]string{
	"pro_full":   {"最详分流矩阵", "43 策略组 + 流媒体/AI 全分流（当前实际 126 组模板）"},
	"pro_lite":   {"精简分流矩阵", "PRO-Lite 核心轻量策略组"},
	"acl4ssr":    {"ACL4SSR 经典", "经典 ACL4SSR 在线全量规则集"},
	"direct":     {"纯节点直通", "仅节点列表，不注入任何策略组"},
	"custom":     {"自定义外链", "使用 overwrite-url 指定的外部模板"},
}

// presetsCmd 查看内置规则集目录与各组的预设
var presetsCmd = &cli.Command{
	Name:  "presets",
	Usage: "查看规则集目录与各组的预设",
	Action: func(c *cli.Context) error {
		cfg, err := loadConfig(c.String("config"))
		if err != nil {
			return err
		}

		ids := make([]string, 0, len(presetCatalog))
		for id := range presetCatalog {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		fmt.Println("可用规则集:")
		for _, id := range ids {
			info := presetCatalog[id]
			fmt.Printf("  %-10s %s\n", id, info[0])
			fmt.Printf("            %s\n", info[1])
		}

		fmt.Println("\n各组的预设:")
		for _, g := range cfg.Groups {
			rule := g.RulePreset
			if rule == "" {
				rule = "pro_full"
			}
			fmt.Printf("  %-12s -> %s\n", g.Name, rule)
		}
		fmt.Println("\n如需给某个组换预设: subctl set groups.<序号>.rule-preset <id>")
		fmt.Println("自定义模板: preset=custom + overwrite-url")
		return nil
	},
}

// splitCmd 触发按组切分：读取现有检测产物 output/all.yaml，
// 按节点归属分发到各组，并生成组级 mihomo.yaml。
// 不重新检测，适合在 WebUI/容器检测完成后马上落盘。
var splitCmd = &cli.Command{
	Name:  "split",
	Usage: "按组切分现有全局结果并落盘（不重新检测）",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "input",
			Value: "output/all.yaml",
			Usage: "全局结果文件（proxies 列表）",
		},
	},
	Action: func(c *cli.Context) error {
		_ = config.GlobalConfig // 保持 import 双方一致
		return fmt.Errorf("split 为独立服务子命令，将在 CLI 服务模式实现")
	},
}