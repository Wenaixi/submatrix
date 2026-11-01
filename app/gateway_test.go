package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestUnifiedAuthMiddleware 测试统一优雅鉴权网关全通道支持与未授权拦截
func TestUnifiedAuthMiddleware(t *testing.T) {
	expectedToken := "wenxi-secret-token"

	router := gin.New()
	router.Use(UnifiedAuthMiddleware(expectedToken))
	router.GET("/test-protected", func(c *gin.Context) {
		c.String(http.StatusOK, "welcome")
	})

	// 1. 无 Token 拦截测试
	req1 := httptest.NewRequest("GET", "/test-protected", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Errorf("无 Token 访问应返回 401，实际返回: %d", w1.Code)
	}

	// 2. 错误 Token 拦截测试
	req2 := httptest.NewRequest("GET", "/test-protected?token=wrong", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("错误 Token 访问应返回 401，实际返回: %d", w2.Code)
	}

	// 3. ?token= URL 查询参数放行
	req3 := httptest.NewRequest("GET", "/test-protected?token="+expectedToken, nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK || w3.Body.String() != "welcome" {
		t.Errorf("?token= 正确密钥应返回 200，实际: %d %s", w3.Code, w3.Body.String())
	}

	// 4. ?api_key= URL 查询参数放行
	req4 := httptest.NewRequest("GET", "/test-protected?api_key="+expectedToken, nil)
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Errorf("?api_key= 正确密钥应返回 200，实际: %d", w4.Code)
	}

	// 5. X-API-Key Header 放行
	req5 := httptest.NewRequest("GET", "/test-protected", nil)
	req5.Header.Set("X-API-Key", expectedToken)
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Errorf("X-API-Key Header 应返回 200，实际: %d", w5.Code)
	}

	// 6. Authorization: Bearer Header 放行
	req6 := httptest.NewRequest("GET", "/test-protected", nil)
	req6.Header.Set("Authorization", "Bearer "+expectedToken)
	w6 := httptest.NewRecorder()
	router.ServeHTTP(w6, req6)
	if w6.Code != http.StatusOK {
		t.Errorf("Authorization Bearer 应返回 200，实际: %d", w6.Code)
	}
}

// TestSubStoreReverseProxy 测试 8199 单端口透明反向代理至 8299
func TestSubStoreReverseProxy(t *testing.T) {
	expectedToken := "wenxi-admin"

	// 启动模拟 Sub-Store 8299 内部服务
	mockSubStore := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/file/mihomo" {
			w.Header().Set("Content-Type", "application/yaml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("proxies: []"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("sub-store mock response"))
	}))
	defer mockSubStore.Close()

	router := gin.New()
	RegisterSubStoreProxy(router, mockSubStore.URL, expectedToken)

	// 启动真实的本地网关 HTTP 服务测试端到端反代
	gatewayServer := httptest.NewServer(router)
	defer gatewayServer.Close()

	// 客户端通过真实 HTTP 请求 /sub-store/api/file/mihomo 访问
	resp, err := http.Get(gatewayServer.URL + "/sub-store/api/file/mihomo?token=" + expectedToken)
	if err != nil {
		t.Fatalf("请求网关反代失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("反向代理请求失败，状态码: %d", resp.StatusCode)
	}

	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	bodyStr := string(buf[:n])
	if bodyStr != "proxies: []" {
		t.Errorf("反向代理未获取到预期内容: %s", bodyStr)
	}
}

// TestGroupSubscriptionServing 测试按组独立文件分发与鉴权
func TestGroupSubscriptionServing(t *testing.T) {
	tmpDir := t.TempDir()
	vipDir := filepath.Join(tmpDir, "groups", "vip")
	os.MkdirAll(vipDir, 0755)
	os.WriteFile(filepath.Join(vipDir, "all.yaml"), []byte("vip proxies: [1,2]"), 0644)

	expectedToken := "wenxi-admin"
	router := gin.New()
	RegisterGroupRoutes(router, tmpDir, expectedToken)

	// 1. 无 token 拦截
	req1 := httptest.NewRequest("GET", "/group/vip/all.yaml", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Errorf("无 token 应被 401 拦截，实际: %d", w1.Code)
	}

	// 2. 带 token 正确获取
	req2 := httptest.NewRequest("GET", "/group/vip/all.yaml?token="+expectedToken, nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("带 token 应返回 200，实际: %d", w2.Code)
	}
	if w2.Body.String() != "vip proxies: [1,2]" {
		t.Errorf("获取组内容不匹配: %s", w2.Body.String())
	}
}