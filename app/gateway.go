package app

import (
	"crypto/subtle"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// UnifiedAuthMiddleware 提供全站统一鉴权中间件
// 优先级支持：
// 1. URL Query 参数: ?token=xxx 或 ?api_key=xxx
// 2. HTTP Header: X-API-Key: xxx
// 3. HTTP Header: Authorization: Bearer xxx
func UnifiedAuthMiddleware(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expectedKey == "" {
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

		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expectedKey)) != 1 {
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
		relPath := strings.TrimPrefix(c.Param("filepath"), "/")
		if relPath == "" {
			relPath = "all.yaml"
		}

		targetFile := filepath.Join(outputPath, "groups", groupName, relPath)
		if _, err := os.Stat(targetFile); os.IsNotExist(err) {
			c.String(http.StatusNotFound, "组订阅文件不存在")
			return
		}

		c.File(targetFile)
	})
}
