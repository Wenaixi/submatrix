package check

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	C "github.com/metacubex/mihomo/constant"
	"golang.org/x/net/proxy"
)

// GroupProxyDialer 支持 HTTP(CONNECT) 与 SOCKS5 协议的组级前置代理拨号器
type GroupProxyDialer struct {
	proxyURL *url.URL
}

// NewGroupProxyDialer 解析代理地址字符串并构造 GroupProxyDialer
func NewGroupProxyDialer(rawURL string) (*GroupProxyDialer, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("empty proxy URL")
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}
	return &GroupProxyDialer{proxyURL: u}, nil
}

// DialContext 通过指定前置代理向目标远程地址发起连接
func (d *GroupProxyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	scheme := strings.ToLower(d.proxyURL.Scheme)
	proxyAddr := d.proxyURL.Host
	if !strings.Contains(proxyAddr, ":") {
		if scheme == "socks5" || scheme == "socks5h" {
			proxyAddr += ":1080"
		} else {
			proxyAddr += ":8080"
		}
	}

	var directDialer net.Dialer
	if scheme == "socks5" || scheme == "socks5h" {
		var auth *proxy.Auth
		if d.proxyURL.User != nil {
			auth = &proxy.Auth{
				User:     d.proxyURL.User.Username(),
				Password: "",
			}
			if pass, ok := d.proxyURL.User.Password(); ok {
				auth.Password = pass
			}
		}
		socksDialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, &directDialer)
		if err != nil {
			return nil, err
		}
		if ctxDialer, ok := socksDialer.(proxy.ContextDialer); ok {
			return ctxDialer.DialContext(ctx, network, addr)
		}
		return socksDialer.Dial(network, addr)
	}

	// 默认为 HTTP / HTTPS CONNECT 隧道代理
	conn, err := directDialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("connect to http proxy %s failed: %w", proxyAddr, err)
	}

	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
	if d.proxyURL.User != nil {
		user := d.proxyURL.User.Username()
		pass, _ := d.proxyURL.User.Password()
		auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", auth)
	}
	connectReq += "User-Agent: SubMatrix/1.0\r\n\r\n"

	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write([]byte(connectReq)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("write CONNECT request to proxy %s failed: %w", proxyAddr, err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	_ = conn.SetDeadline(time.Time{})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read CONNECT response from proxy %s failed: %w", proxyAddr, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("http proxy %s returned status %d %s", proxyAddr, resp.StatusCode, resp.Status)
	}

	return conn, nil
}

// ListenPacket 实现 C.Dialer 接口
func (d *GroupProxyDialer) ListenPacket(ctx context.Context, network, addr string, rAddr netip.AddrPort) (net.PacketConn, error) {
	var lc net.ListenConfig
	return lc.ListenPacket(ctx, network, addr)
}

// 接口静态检查
var _ C.Dialer = (*GroupProxyDialer)(nil)
