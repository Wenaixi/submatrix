package app

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// UnifiedAuthMiddleware 统一优雅鉴权网关中间件
// 支持 ?token=xxx / ?api_key=xxx / X-API-Key Header / Authorization: Bearer Header
func UnifiedAuthMiddleware(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		expectedKey = strings.TrimSpace(expectedKey)
		if expectedKey == "" {
			// 未设置密钥时默认放行
			c.Next()
			return
		}

		// 1. 从 URL Query 中提取 (最高优先级，客户端友好)
		providedToken := strings.TrimSpace(c.Query("token"))
		if providedToken == "" {
			providedToken = strings.TrimSpace(c.Query("api_key"))
		}

		// 2. 从 Header X-API-Key 提取
		if providedToken == "" {
			providedToken = strings.TrimSpace(c.GetHeader("X-API-Key"))
		}

		// 3. 从 Header Authorization 提取 (Bearer 模式)
		if providedToken == "" {
			authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				providedToken = strings.TrimSpace(authHeader[7:])
			}
		}

		// 4. 恒定时间安全比对
		if subtle.ConstantTimeCompare([]byte(providedToken), []byte(expectedKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "无效的API访问密钥/Token",
			})
			return
		}

		c.Next()
	}
}

// RegisterSubStoreProxy 注册 8199 -> 8299 的单端口内部透明反代
func RegisterSubStoreProxy(router *gin.Engine, targetURL string, authKey string) error {
	target, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("解析 Sub-Store 目标地址失败: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// 自定义 Director 剥离 /sub-store 前缀
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host

		// 将 /sub-store/api/... 转换为 /api/...
		path := req.URL.Path
		if strings.HasPrefix(path, "/sub-store") {
			path = strings.TrimPrefix(path, "/sub-store")
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			req.URL.Path = path
		}
	}

	// 挂载鉴权中间件与反代路由
	group := router.Group("/sub-store")
	group.Use(UnifiedAuthMiddleware(authKey))
	group.Any("/*proxyPath", func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	return nil
}

// RegisterGroupRoutes 注册按组分发文件的独立端点
func RegisterGroupRoutes(router *gin.Engine, outputRoot string, authKey string) {
	group := router.Group("/group")
	group.Use(UnifiedAuthMiddleware(authKey))

	group.GET("/:name/*subPath", func(c *gin.Context) {
		groupName := c.Param("name")
		subPath := strings.TrimPrefix(c.Param("subPath"), "/")

		if groupName == "" || subPath == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
			return
		}

		// 安全防御路径遍历
		targetPath := filepath.Clean(filepath.Join(outputRoot, "groups", groupName, subPath))
		expectedRoot := filepath.Clean(filepath.Join(outputRoot, "groups"))
		if !strings.HasPrefix(targetPath, expectedRoot) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "非法访问路径"})
			return
		}

		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "组订阅文件尚未生成"})
			return
		}

		c.Header("Content-Type", "application/yaml; charset=utf-8")
		c.Header("Cache-Control", "no-cache")
		c.File(targetPath)
	})
}
