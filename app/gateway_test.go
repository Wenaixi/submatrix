package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUnifiedAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expectedKey := "test-secret"

	router := gin.New()
	router.Use(UnifiedAuthMiddleware(expectedKey))
	router.GET("/protected", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	tests := []struct {
		name       string
		url        string
		headerKey  string
		headerVal  string
		wantStatus int
	}{
		{"无密钥访问被拒绝", "/protected", "", "", http.StatusUnauthorized},
		{"错误密钥访问被拒绝", "/protected?token=wrong", "", "", http.StatusUnauthorized},
		{"Query参数token正确访问成功", "/protected?token=test-secret", "", "", http.StatusOK},
		{"Query参数api_key正确访问成功", "/protected?api_key=test-secret", "", "", http.StatusOK},
		{"Header X-API-Key访问成功", "/protected", "X-API-Key", "test-secret", http.StatusOK},
		{"Header Bearer Token访问成功", "/protected", "Authorization", "Bearer test-secret", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, tt.url, nil)
			if tt.headerKey != "" {
				req.Header.Set(tt.headerKey, tt.headerVal)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("%s: status = %d, want %d", tt.name, w.Code, tt.wantStatus)
			}
		})
	}
}

func TestGroupRoutesDelivery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()

	groupDir := filepath.Join(tmpDir, "groups", "free-a")
	_ = os.MkdirAll(groupDir, 0755)
	_ = os.WriteFile(filepath.Join(groupDir, "all.yaml"), []byte("proxies: []"), 0644)

	router := gin.New()
	RegisterGroupRoutes(router, tmpDir, UnifiedAuthMiddleware("secret"))

	// 正确 token 获取组订阅文件
	req, _ := http.NewRequest(http.MethodGet, "/group/free-a/all.yaml?token=secret", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "proxies: []" {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}
