package app

import (
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ---------- 常量 ----------

// shareAllowedFiles 分享可暴露的文件白名单（防任意文件泄漏）。
var shareAllowedFiles = map[string]bool{
	"all.yaml":      true,
	"all-raw.yaml":  true,
	"history.yaml":  true,
	"mihomo.yaml":   true,
}

// rateLimit 每分钟每 IP 每分享的拉取上限（防刷）。
const rateLimitPerMin = 60

// ---------- 速率限制（内存 token bucket / 固定窗口） ----------

type rateKey struct {
	ip  string
	code string
}

type rateEntry struct {
	windowStart time.Time
	count       int
}

var (
	rateMu    sync.Mutex
	rateMap   = make(map[rateKey]*rateEntry)
)

// allowRequest 检查请求是否在速率限制内；超限返回 false。
func allowRequest(ip, code string) bool {
	now := time.Now()
	key := rateKey{ip: ip, code: code}
	rateMu.Lock()
	defer rateMu.Unlock()
	e, ok := rateMap[key]
	if !ok || now.Sub(e.windowStart) >= time.Minute {
		rateMap[key] = &rateEntry{windowStart: now, count: 1}
		// 简单清理：窗口超过 10 分钟的旧条目
		if len(rateMap) > 10000 {
			for k, v := range rateMap {
				if now.Sub(v.windowStart) > 10*time.Minute {
					delete(rateMap, k)
				}
			}
		}
		return true
	}
	e.count++
	return e.count <= rateLimitPerMin
}

// ---------- IP 获取（防伪造） ----------

// getClientIP 优先取 X-Forwarded-For 最左值，但若为私有/回环则忽略（防伪造）；
// 回退 RemoteAddr。
func getClientIP(c *gin.Context) string {
	// 默认：信任 XFF 最右非私有值（兼容 Cloudflare 隧道：CF 把真实客户端 IP 追加在最后）。
	// 若配置了 trusted-proxy-ips 白名单，则仅当 RemoteAddr 命中白名单时才信 XFF（严格模式）。
	remoteHost, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		remoteHost = c.Request.RemoteAddr
	}

	if trustedProxiesEnabled {
		// 严格模式：只有可信代理转发的请求才信 XFF
		if !trustedProxies[remoteHost] {
			return remoteHost
		}
	}

	xff := c.GetHeader("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		// 从右往左找第一个非私有值：CF 场景 = 真实客户端；直连伪造 = 最右伪造值（已知权衡）
		for i := len(parts) - 1; i >= 0; i-- {
			cand := strings.TrimSpace(parts[i])
			if cand != "" && !isPrivateIP(cand) {
				return cand
			}
		}
	}
	return remoteHost
}

// trustedProxies 可信代理 IP 白名单（配置 trusted-proxy-ips 填充）。
// 空 = 默认宽松模式（信 XFF 最右）；非空 = 严格模式（仅白名单来源信 XFF）。
var (
	trustedProxies      = map[string]bool{}
	trustedProxiesEnabled bool
)

// configureTrustedProxies 从配置填充可信代理白名单。
func configureTrustedProxies(ips []string) {
	trustedProxies = make(map[string]bool, len(ips))
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip != "" {
			trustedProxies[ip] = true
		}
	}
	trustedProxiesEnabled = len(trustedProxies) > 0
}

// isPrivateIP 判断 IP 是否为私有/回环/链路本地（不可信 XFF 来源）。
func isPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return true // 解析失败按不可信处理
	}
	return parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsLinkLocalUnicast() ||
		parsed.IsLinkLocalMulticast() || parsed.IsUnspecified()
}

// ---------- 分享文件服务 ----------

// serveShareFile 校验并返回分享文件：
// 鉴权链：码校验 → 分享存在 → 启用 → 未过期 → 文件白名单 → 记统计 → 返回
func serveShareFile(c *gin.Context, code, fileName, clientIP string) {
	// 输入码与分享码常量时间比较
	store := GetShareStore()
	sh, ok := store.Get(code)
	if !ok || !sh.Enabled {
		c.String(http.StatusForbidden, "分享不存在或已停用")
		return
	}
	// 过期检查（ExpiresAt 零值 = 永不过期）
	if !sh.ExpiresAt.IsZero() && time.Now().After(sh.ExpiresAt) {
		c.String(http.StatusForbidden, "分享已过期")
		return
	}

	// 文件白名单
	if !shareAllowedFiles[fileName] {
		c.String(http.StatusForbidden, "文件类型不允许")
		return
	}
	// 分享限定的文件列表（Files 空 = 全部）
	if len(sh.Files) > 0 {
		allowed := false
		for _, f := range sh.Files {
			if f == fileName {
				allowed = true
				break
			}
		}
		if !allowed {
			c.String(http.StatusForbidden, "文件不在分享范围内")
			return
		}
	}

	// 组名白名单（防路径穿越）
	if !groupNamePattern.MatchString(sh.GroupName) && sh.GroupName != "" {
		c.String(http.StatusForbidden, "组名非法")
		return
	}

	// 定位组产物目录
	groupDir := filepath.Join(outputDir(), "groups", sh.GroupName)
	if sh.GroupName == "" {
		// 空组名 = 全部组？此处兼容：仅允许根产物目录（实际为 groups 下全部，简化：拒绝）
		c.String(http.StatusForbidden, "分享未绑定组")
		return
	}
	absPath := filepath.Join(groupDir, fileName)
	// 防路径穿越双保险
	if !strings.HasPrefix(absPath, groupDir) {
		c.String(http.StatusForbidden, "非法路径")
		return
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		// 文件不存在统一 403（与分享不存在/停用同响应，防文件存在性探测）
		c.String(http.StatusForbidden, "分享不存在或已停用")
		return
	}

	// 记录统计（内存累加，定时落盘）
	store.RecordHit(code, clientIP, fileName, c.Request.UserAgent())

	// 返回文件（无缓存，保证拉取总是最新）
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.File(absPath)
}

// registerShareRoutes 注册按组分享路由：
//
//	GET /share/:code          分享首页（HTML 向导）
//	GET /share/:code/:file    文件直链（需 ?code= 或 X-Share-Code 头鉴权）
//
// 安全：统一 403 防探测；速率限制；白名单；统计记录。
func (app *App) registerGroupShareRoutes(router *gin.Engine) {
	router.GET("/share/:code", func(c *gin.Context) {
		code := c.Param("code")
		renderSharePage(c, http.StatusOK, SharePageData{
			Title:       "订阅分享",
			HeaderColor: "var(--accent)",
			HeaderIcon:  templateHTML(svgShieldCheck),
			HeaderTitle: "分享已启用",
			// 注意：code 来自 URL，只经 ShareCode 字段转义插值进入模板，
			// 绝不拼进 templateHTML 字符串（防反射型 XSS）。
			Description: templateHTML("此分享受分享码保护。文件直链需带 <code>?code=&lt;分享码&gt;</code> 参数。"),
			BadgeStyle:  "success",
			BadgeText:   "Share",
			ShareCode:   code,
			FooterText:  "Shared via SubMatrix",
			ExtraHint:   templateHTML("管理分享见 WebUI「分享管理」页。分享码泄漏即可拉取，请妥善保管。"),
		})
	})

	router.GET("/share/:code/*filepath", func(c *gin.Context) {
		code := c.Param("code")
		// 文件直链鉴权：?code= 或 X-Share-Code 头
		inputCode := c.Query("code")
		if inputCode == "" {
			inputCode = c.GetHeader("X-Share-Code")
		}
		if inputCode == "" || !equalConstantTime(inputCode, code) {
			c.String(http.StatusForbidden, "需要分享码")
			return
		}

		// 速率限制（防刷）
		ip := getClientIP(c)
		if !allowRequest(ip, code) {
			c.String(http.StatusTooManyRequests, "请求过于频繁")
			return
		}

		relPath := strings.TrimPrefix(c.Param("filepath"), "/")
		if relPath == "" || relPath == "/" {
			c.Redirect(http.StatusFound, "/share/"+code)
			return
		}
		// 仅允许白名单内的单文件名（拒绝子目录/nested 路径）
		cleanName := filepath.Base(filepath.Clean(relPath))
		if cleanName != relPath {
			c.String(http.StatusForbidden, "非法路径")
			return
		}
		serveShareFile(c, code, cleanName, ip)
	})
}

// templateHTML 辅助：string → template.HTML（避免每个调用点转换）。
func templateHTML(s string) template.HTML {
	return template.HTML(s)
}