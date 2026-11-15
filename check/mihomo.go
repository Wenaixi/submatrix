package check

import (
	"log/slog"
	"strings"

	_ "github.com/metacubex/mihomo/config"

	"github.com/metacubex/mihomo/component/resolver"
	"github.com/metacubex/mihomo/dns"
	"github.com/Wenaixi/submatrix/config"
)

// SetupMihomoDNS 初始化并注入底层的 mihomo DNS 解析器
func SetupMihomoDNS() {
	// 全局 IPv6 禁用控制
	resolver.DisableIPv6 = !config.GlobalConfig.EnableIPv6

	// 如果没有任何自定义 DNS，保持默认解析器不动。
	//
	// 这里曾错误地尝试「重建一个兜底解析器」，结果实测把检测打成全线 0 可用：
	// 用 dns.NewResolver(cfg) 覆盖 SystemResolver 会替换掉 mihomo 在 dns 包
	// init() 里构造的 systemClient（见 mihomo/dns/system.go），
	// 在受限网络下（容器只放通特定 DNS）会导致所有解析秒级失败，
	// 表现为「节点全部 0 可用、整组检测耗时不到 1 秒」。
	//
	// 正确认知：cleanupMihomo 只把 DefaultResolver 置 nil，SystemResolver 始终
	// 由 mihomo 的 init() 保证非 nil；而 resolver 各 Lookup/Resolve 入口在
	// DefaultResolver 为 nil 时会自动回退到 SystemResolver。因此无自定义 DNS 时
	// 不需要任何重建动作。
	hasCustom := len(config.GlobalConfig.NameServer) > 0 || len(config.GlobalConfig.ProxyServerNameserver) > 0 || len(config.GlobalConfig.ProxyServerNameserverPolicy) > 0

	if !hasCustom {
		return
	}

	// 构造 mihomo 核心的 DNS 配置对象
	cfg := dns.Config{
		IPv6: config.GlobalConfig.EnableIPv6,
	}

	// 提前注入兜底 Resolver (注意：NewResolver 返回的是 Resolvers，取其主 .Resolver)
	fallbackResolvers := dns.NewResolver(dns.Config{IPv6: cfg.IPv6})
	resolver.DefaultResolver = fallbackResolvers.Resolver
	resolver.SystemResolver = fallbackResolvers.Resolver
	resolver.ProxyServerHostResolver = fallbackResolvers.Resolver

	// 1. 固化 DefaultNameserver (兜底解析 DoH 等基础域名)
	defaults := []string{"114.114.114.114", "223.5.5.5", "8.8.8.8", "1.0.0.1", "1.1.8.8"}

	if parsedDefaults, err := dns.ParseNameServer(defaults); err == nil {
		cfg.Default = parsedDefaults
	}

	bootstrapResolvers := dns.NewResolver(cfg)
	resolver.DefaultResolver = bootstrapResolvers.Resolver
	resolver.SystemResolver = bootstrapResolvers.Resolver

	// 2. 解析 nameserver (主 DNS)
	if len(config.GlobalConfig.NameServer) > 0 {
		if parsedMain, err := dns.ParseNameServer(config.GlobalConfig.NameServer); err == nil {
			cfg.Main = parsedMain
		} else {
			slog.Warn("解析 nameserver 失败", "error", err)
		}
	}

	// 3. 解析 proxy-server-nameserver (节点专属域名解析 DNS)
	if len(config.GlobalConfig.ProxyServerNameserver) > 0 {
		if parsedProxy, err := dns.ParseNameServer(config.GlobalConfig.ProxyServerNameserver); err == nil {
			cfg.ProxyServer = parsedProxy
		} else {
			slog.Warn("解析 proxy-server-nameserver 失败", "error", err)
		}
	}

	// 4. 解析 proxy-server-nameserver-policy
	if len(config.GlobalConfig.ProxyServerNameserverPolicy) > 0 {
		for domainStr, v := range config.GlobalConfig.ProxyServerNameserverPolicy {
			// 兼容单条 string 或 多条 []string
			var servers []string
			switch val := v.(type) {
			case string:
				servers = []string{val}
			case []interface{}:
				for _, s := range val {
					if str, ok := s.(string); ok {
						servers = append(servers, str)
					}
				}
			case []string:
				servers = val
			}

			if len(servers) == 0 {
				continue
			}

			parsed, err := dns.ParseNameServer(servers)
			if err != nil {
				slog.Warn("解析 proxy-server-nameserver-policy 规则失败", "domain", domainStr, "error", err)
				continue
			}

			// 兼容 Mihomo 支持的逗号分隔多域名特性: "www.baidu.com,+.google.com": 1.1.1.1
			domains := strings.Split(domainStr, ",")
			for _, d := range domains {
				d = strings.TrimSpace(d)
				if d == "" {
					continue
				}
				// 直接封装为 dns.Policy 切片元素，不传 Matcher，让引擎自己构建 Trie 树！
				cfg.ProxyServerPolicy = append(cfg.ProxyServerPolicy, dns.Policy{
					Domain:      d,
					NameServers: parsed,
				})
			}
		}
	}

	// 实例化并将构造好的 Resolver 注入
	finalResolvers := dns.NewResolver(cfg)

	// 主 DNS / 系统 DNS
	resolver.DefaultResolver = finalResolvers.Resolver
	resolver.SystemResolver = finalResolvers.Resolver

	// 节点专属 DNS：精准识别并赋值给真正处理 Policy 的 ProxyResolver
	// Mihomo 的 .Invalid() 命名比较怪异，实际上返回 true 代表“有配置且可用”
	if finalResolvers.ProxyResolver != nil && finalResolvers.ProxyResolver.Invalid() {
		resolver.ProxyServerHostResolver = finalResolvers.ProxyResolver
	} else {
		// 兜底降级
		resolver.ProxyServerHostResolver = finalResolvers.Resolver
	}

	slog.Debug("使用自定义 DNS 配置",
		"dns", len(cfg.Main),
		"proxy-dns", len(cfg.ProxyServer),
		"proxy-dns-policy", len(cfg.ProxyServerPolicy))
}
