package app

import (
	"crypto/subtle"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wenaixi/submatrix/config"
	"github.com/gin-gonic/gin"
)

// UnifiedAuthMiddleware 提供全站统一鉴权中间件
// 优先级支持：
// 1. URL Query 参数: ?token=xxx 或 ?api_key=xxx
// 2. HTTP Header: X-API-Key: xxx
// 3. HTTP Header: Authorization: Bearer xxx
//
// 【安全修复 2026-11-10】该函数改为运行时每次请求动态读取 config.GlobalConfig.APIKey，
// 杜绝「中间件闭包捕获初始 key、配置热重载后旧 key 永久有效」的凭据轮换失效问题（CRITICAL）。
func UnifiedAuthMiddleware(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 动态取当前有效 key：优先入参 expectedKey（兼容既有调用），否则实时读全局配置
		currentKey := expectedKey
		if config.GlobalConfig != nil && config.GlobalConfig.APIKey != "" {
			currentKey = config.GlobalConfig.APIKey
		}
		if currentKey == "" {
			c.Next()
			return
		}

		token := c.Query("token")
		if token == "" {
			token = c.Query("api_key")
		}
		if token == "" {
			token = c.GetHeader("X-API-Key")
		}
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				token = strings.TrimSpace(authHeader[7:])
			}
		}

		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(currentKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "API密钥无效或未提供，请通过 ?token=xxx、X-API-Key 或 Bearer Token 进行鉴权",
			})
			return
		}

		c.Next()
	}
}

// RegisterSubStoreProxy 将 /sub-store/* 内部反向代理至 127.0.0.1:8299
func RegisterSubStoreProxy(router *gin.Engine, targetURL string, authMiddleware gin.HandlerFunc) error {
	u, err := url.Parse(targetURL)
	if err != nil {
		return err
	}

	proxy := httputil.NewSingleHostReverseProxy(u)

	handler := func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}

	subStoreGroup := router.Group("/sub-store")
	if authMiddleware != nil {
		subStoreGroup.Use(authMiddleware)
	}
	subStoreGroup.Any("/*path", handler)
	return nil
}

// RegisterGroupRoutes 注册按组分发的订阅端点 /group/:name/*
func RegisterGroupRoutes(router *gin.Engine, outputPath string, authMiddleware gin.HandlerFunc) {
	groupRoute := router.Group("/group")
	if authMiddleware != nil {
		groupRoute.Use(authMiddleware)
	}

	groupRoute.GET("/:name/*filepath", func(c *gin.Context) {
		groupName := c.Param("name")
		// 组名参与文件路径（output/groups/<name>/），必须限制字符集防路径穿越
		if !groupNamePattern.MatchString(groupName) {
			c.String(http.StatusBadRequest, "组名非法")
			return
		}
		relPath := strings.TrimPrefix(c.Param("filepath"), "/")
		if relPath == "" {
			relPath = "all.yaml"
		}

		targetFile := filepath.Join(outputPath, "groups", groupName, relPath)
		if _, err := os.Stat(targetFile); os.IsNotExist(err) {
			c.String(http.StatusNotFound, "组订阅文件不存在")
			return
		}

		// 智能自适应网关：若请求的是 mihomo.yaml 完整规则配置，动态将模板中的直链 BaseURL 与 Token 替换为官方公网域名或客户端真实访问源
		if relPath == "mihomo.yaml" {
			data, err := os.ReadFile(targetFile)
			if err == nil {
				baseURL := GetCanonicalBaseURL(c)
				token := c.Query("token")
				if token == "" {
					token = c.Query("api_key")
				}

				content := string(data)
				// 动态替换 Provider 直链的基准网络地址与鉴权 Token
				content = strings.ReplaceAll(content, "http://127.0.0.1:8199", baseURL)
				content = strings.ReplaceAll(content, "https://127.0.0.1:8199", baseURL)
				if token != "" && token != defaultAPIKeyLiteral {
					content = strings.ReplaceAll(content, "token="+defaultAPIKeyLiteral, "token="+token)
				}

				c.Header("Content-Type", "text/plain; charset=utf-8")
				c.String(http.StatusOK, content)
				return
			}
		}

		c.File(targetFile)
	})
}

// GetCanonicalBaseURL 计算当前请求的权威基准 URL（优先 public-url，次之 X-Forwarded 反代头，兜底 Host）
func GetCanonicalBaseURL(c *gin.Context) string {
	if config.GlobalConfig != nil && strings.TrimSpace(config.GlobalConfig.PublicURL) != "" {
		return strings.TrimRight(strings.TrimSpace(config.GlobalConfig.PublicURL), "/")
	}
	proto := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		proto = "https"
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		host = "127.0.0.1:8199"
	}
	return proto + "://" + host
}
