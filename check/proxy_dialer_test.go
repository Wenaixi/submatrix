package check

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewGroupProxyDialer(t *testing.T) {
	// 测试解析 HTTP
	d, err := NewGroupProxyDialer("http://127.0.0.1:10808")
	if err != nil || d == nil {
		t.Fatalf("failed to create http dialer: %v", err)
	}

	// 测试解析 SOCKS5
	d2, err2 := NewGroupProxyDialer("socks5://user:pass@127.0.0.1:1080")
	if err2 != nil || d2 == nil {
		t.Fatalf("failed to create socks5 dialer: %v", err2)
	}

	// 测试空代理
	_, err3 := NewGroupProxyDialer("")
	if err3 == nil {
		t.Fatalf("expected error on empty proxy URL")
	}
}

func TestGroupProxyDialer_HTTP_CONNECT(t *testing.T) {
	// 启动一个模拟的本地 HTTP CONNECT 代理服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijack not supported", http.StatusInternalServerError)
				return
			}
			clientConn, _, err := hijacker.Hijack()
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
			_ = clientConn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	dialer, err := NewGroupProxyDialer(ts.URL)
	if err != nil {
		t.Fatalf("failed to create dialer: %v", err)
	}

	conn, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")
	if err != nil {
		t.Fatalf("failed to dial through proxy: %v", err)
	}
	defer conn.Close()
}
