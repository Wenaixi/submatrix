package main

import (
	"fmt"
	"os"

	"github.com/Wenaixi/submatrix/config"
	"github.com/goccy/go-yaml"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "subctl",
		Usage:       "SubMatrix 无头配置与操作 CLI",
		Description: "精确读取、校验、修改 SubMatrix 的 config.yaml，并触发组订阅落盘。",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "config/config.yaml",
				Usage:   "配置文件路径",
			},
		},
		Commands: []*cli.Command{
			validateCmd,
			getCmd,
			setCmd,
			groupsCmd,
			presetsCmd,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

// loadConfig 读取并解析配置文件为 *config.Config
func loadConfig(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	cfg := config.Config{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	return &cfg, nil
}

// saveConfig 把 *config.Config 写回配置文件（保留原文件其它结构）
func saveConfig(path string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	// 0o644 与 save/method/local.go 的 fileMode 保持一致
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}