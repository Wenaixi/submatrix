package main

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/urfave/cli/v2"

	"github.com/Wenaixi/submatrix/config"
)

func yamlMarshal(v any) (string, error) {
	out, err := yaml.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("序列化取值失败: %w", err)
	}
	return string(out), nil
}

// validateCmd 校验配置文件语法与关键字段
var validateCmd = &cli.Command{
	Name:  "validate",
	Usage: "校验配置文件的语法与关键字段",
	Action: func(c *cli.Context) error {
		cfg, err := loadConfig(c.String("config"))
		if err != nil {
			return err
		}

		var problems []string
		if cfg.ListenPort == "" {
			problems = append(problems, "listen-port 为空")
		}
		if cfg.APIKey == "" {
			problems = append(problems, "api-key 为空")
		}
		if len(cfg.SubUrls) == 0 && len(cfg.Groups) == 0 && len(cfg.SubUrlsRemote) == 0 {
			problems = append(problems, "没有任何订阅来源（sub-urls / groups / sub-urls-remote 全空）")
		}

		seen := make(map[string]bool)
		for _, g := range cfg.Groups {
			if strings.TrimSpace(g.Name) == "" {
				problems = append(problems, "存在组名为空的组")
			}
			if seen[g.Name] {
				problems = append(problems, "组名重复: "+g.Name)
			}
			seen[g.Name] = true
			if len(g.URLs) == 0 {
				problems = append(problems, "组 "+g.Name+" 没有任何订阅 URL")
			}
		}

		fmt.Printf("配置文件: %s\n", c.String("config"))
		fmt.Printf("监听端口: %s  鉴权: %s\n", cfg.ListenPort, cfg.APIKey)
		fmt.Printf("订阅链接: %d 条 (本地) %d 条 (远程)\n", len(cfg.SubUrls), len(cfg.SubUrlsRemote))
		fmt.Printf("分组: %d 个 (启用 %d)\n", len(cfg.Groups), countEnabledGroups(cfg))
		if len(problems) == 0 {
			fmt.Println("校验结果: 通过")
			return nil
		}
		fmt.Printf("校验结果: 有%d个问题\n", len(problems))
		for _, p := range problems {
			fmt.Println("  -", p)
		}
		return nil
	},
}

// getCmd 按点号路径读取配置值，如 groups.0.name
var getCmd = &cli.Command{
	Name:  "get",
	Usage: "读取配置值，如: subctl get groups.0.name",
	Action: func(c *cli.Context) error {
		if c.Args().Len() != 1 {
			return fmt.Errorf("用法: subctl get <路径>")
		}
		path := c.Args().First()
		cfg, err := loadConfig(c.String("config"))
		if err != nil {
			return err
		}
		v, err := lookupPath(reflect.ValueOf(cfg), path)
		if err != nil {
			return err
		}
		out, err := yamlMarshal(v.Interface())
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	},
}

// setCmd 按点号路径写入配置值
var setCmd = &cli.Command{
	Name:  "set",
	Usage: "写入配置值，如: subctl set rename-node false",
	Action: func(c *cli.Context) error {
		if c.Args().Len() != 2 {
			return fmt.Errorf("用法: subctl set <路径> <值>")
		}
		path := c.Args().Get(0)
		raw := c.Args().Get(1)

		cfg, err := loadConfig(c.String("config"))
		if err != nil {
			return err
		}

		target, err := setPath(reflect.ValueOf(cfg), path, raw)
		if err != nil {
			return err
		}
		if err := saveConfig(c.String("config"), cfg); err != nil {
			return err
		}
		out, _ := yamlMarshal(target.Interface())
		fmt.Printf("已写入 %s = %s\n", path, strings.TrimSpace(out))
		return nil
	},
}

// groupsCmd 查看/启用/禁用分组
var groupsCmd = &cli.Command{
	Name:  "groups",
	Usage: "查看或操作分组",
	Subcommands: []*cli.Command{
		{
			Name:  "list",
			Usage: "列出所有分组",
			Action: func(c *cli.Context) error {
				cfg, err := loadConfig(c.String("config"))
				if err != nil {
					return err
				}
				if len(cfg.Groups) == 0 {
					fmt.Println("(没有配置任何分组)")
					return nil
				}
				for i, g := range cfg.Groups {
					state := "启用"
					if g.Enabled != nil && !*g.Enabled {
						state = "禁用"
					}
					rule := g.RulePreset
					if rule == "" {
						rule = "pro_full"
					}
					fmt.Printf("[%d] %s (%s) %s | 优先级=%d 测活=%v 测速=%v 规则=%s URL数=%d\n",
						i, g.Name, g.DisplayName, state, g.Priority,
						groupBool(g.CheckAlive, true), groupBool(g.CheckSpeed, true),
						rule, len(g.URLs),
					)
				}
				return nil
			},
		},
		{
			Name:  "toggle",
			Usage: "启用或禁用分组: subctl groups toggle <组名> true",
			Action: func(c *cli.Context) error {
				if c.Args().Len() != 2 {
					return fmt.Errorf("用法: subctl groups toggle <组名> true|false")
				}
				name := c.Args().Get(0)
				on, err := strconv.ParseBool(c.Args().Get(1))
				if err != nil {
					return fmt.Errorf("状态必须是 true/false: %w", err)
				}
				cfg, err := loadConfig(c.String("config"))
				if err != nil {
					return err
				}
				for i := range cfg.Groups {
					if cfg.Groups[i].Name == name {
						cfg.Groups[i].Enabled = &on
						if err := saveConfig(c.String("config"), cfg); err != nil {
							return err
						}
						fmt.Printf("组 %s 已%s\n", name, map[bool]string{true: "启用", false: "禁用"}[on])
						return nil
					}
				}
				return fmt.Errorf("未找到组: %s", name)
			},
		},
	},
}

func groupBool(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func countEnabledGroups(cfg *config.Config) int {
	n := 0
	for _, g := range cfg.Groups {
		if g.Enabled == nil || *g.Enabled {
			n++
		}
	}
	return n
}

// lookupPath 按点号路径读取配置字段（支持切片下标）
func lookupPath(root reflect.Value, path string) (reflect.Value, error) {
	cur := root
	for _, seg := range strings.Split(path, ".") {
		if seg == "" {
			continue
		}
		for cur.Kind() == reflect.Pointer {
			if cur.IsNil() {
				return reflect.Value{}, fmt.Errorf("路径 %s 指向空值", path)
			}
			cur = cur.Elem()
		}
		switch cur.Kind() {
		case reflect.Struct:
			f := cur.FieldByNameFunc(func(name string) bool {
				sf, ok := cur.Type().FieldByName(name)
				if !ok {
					return false
				}
				return fieldYAMLName(sf, seg)
			})
			if !f.IsValid() {
				return reflect.Value{}, fmt.Errorf("字段不存在: %s (在 %s)", seg, cur.Type().Name())
			}
			cur = f
		case reflect.Slice, reflect.Array:
			idx, err := strconv.Atoi(seg)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("期望下标: %s", seg)
			}
			if idx < 0 || idx >= cur.Len() {
				return reflect.Value{}, fmt.Errorf("下标越界: %d (长度 %d)", idx, cur.Len())
			}
			cur = cur.Index(idx)
		default:
			return reflect.Value{}, fmt.Errorf("路径 %s 无法继续深入 (当前类型 %s)", seg, cur.Kind())
		}
	}
	return cur, nil
}

// fieldYAMLName 判断字段名是否能匹配该路径段（依据 yaml tag）
func fieldYAMLName(f reflect.StructField, seg string) bool {
	if f.Name == seg || strings.EqualFold(f.Name, seg) {
		return true
	}
	tag := f.Tag.Get("yaml")
	if tag == "" {
		return false
	}
	name := strings.Split(tag, ",")[0]
	return name == seg
}

// setPath 把 raw 解析为适当类型并写入路径。
func setPath(root reflect.Value, path string, raw string) (reflect.Value, error) {
	target, err := lookupPath(root, path)
	if err != nil {
		return reflect.Value{}, err
	}
	if target.Kind() == reflect.Slice && target.Type().Elem().Kind() == reflect.String {
		parts := strings.Split(raw, ",")
		items := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				items = append(items, p)
			}
		}
		target.Set(reflect.ValueOf(items))
		return target, nil
	}
	var v any
	switch target.Kind() {
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("值 %q 不是 bool", raw)
		}
		v = b
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("值 %q 不是整数", raw)
		}
		v = n
	case reflect.String:
		v = raw
	default:
		if err := yaml.Unmarshal([]byte(raw), target.Addr().Interface()); err != nil {
			return reflect.Value{}, fmt.Errorf("值 %q 无法解析为 %s: %w", raw, target.Kind(), err)
		}
		return target, nil
	}
	target.Set(reflect.ValueOf(v).Convert(target.Type()))
	return target, nil
}