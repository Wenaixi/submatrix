package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/juju/ratelimit"
	"github.com/metacubex/mihomo/common/convert"
)

var testURLs []string

func init() {
	// fastSpeedTestURLs 由测试聚合脚本生成，可能为空（脚本未产出数据时会写成空数组）。
	// 此时回退到静态的 SpeedTestURLs，保证 "random" 语义在任何情况下都有可用 URL。
	if len(fastSpeedTestURLs) > 0 {
		testURLs = fastSpeedTestURLs
	} else {
		testURLs = SpeedTestURLs
	}
}

// networkLimitedReader 负责在读取 Body 时检查底层网络流量是否超限
type networkLimitedReader struct {
	reader      io.Reader
	getNetBytes func() uint64 // 获取底层原子计数
	startBytes  uint64        // 初始读数
	limit       uint64        // 限制阈值 (0为不限制)
}

func (r *networkLimitedReader) Read(p []byte) (int, error) {
	// 只有在限制启用且能获取底层流量时才进行拦截逻辑
	if r.limit > 0 && r.getNetBytes != nil {
		curr := r.getNetBytes()

		// 防御性处理：计数器回绕（极罕见）
		if curr < r.startBytes {
			r.startBytes = curr
		}

		readBytes := curr - r.startBytes

		// 检查是否已经超限
		if readBytes >= r.limit {
			return 0, io.EOF
		}

		// 截断 p 的长度，防止最后一次读取导致总流量大幅超出 limit
		// io.Copy 默认 buffer 是 32KB，如果不截断，可能会多读几十 KB
		remaining := r.limit - readBytes
		if uint64(len(p)) > remaining {
			p = p[:remaining]
		}
	}
	return r.reader.Read(p)
}

// CheckSpeed 执行下载测速
// speedURL/downloadTimeoutSec/downloadMB 来自组配置（组级字段参数化）
//
// 返回 (speed, traffic, err)。speedURL 为空时按配置语义「关闭测速」处理：
// 返回 (0, 0, nil)，由调用方结合 min-speed 判定——min-speed 为 0 时视为通过，
// 不再发出注定失败的请求（旧实现在此会构造空 URL 并必然失败，导致节点被误判死亡）。
func CheckSpeed(httpClient *http.Client, bucket *ratelimit.Bucket, getNetBytes func() uint64, speedURL string, downloadTimeoutSec, downloadMB int) (int, int64, error) {
	// 确定测速 URL，根据配置使用随机下载测速链接
	url := strings.TrimSpace(speedURL)
	if strings.Contains(url, "random") && len(testURLs) > 0 {
		url = testURLs[rand.IntN(len(testURLs))]
	}
	if url == "" {
		slog.Debug("测速 URL 为空，跳过测速", "downloadMB", downloadMB)
		return 0, 0, nil
	}
	slog.Debug("使用的测速URL", "url", url)

	speedClient := *httpClient
	// 【修复 2026-11-12】不再把 Timeout 清零：保留 client 级 Timeout 作为最终兜底，
	// 防止 context 超时在个别代理/连接上传导失败时导致 io.Copy 永久阻塞（测试复现卡死）。
	timeout := time.Duration(downloadTimeoutSec) * time.Second
	if timeout > 0 {
		speedClient.Timeout = timeout
	} else {
		speedClient.Timeout = 30 * time.Second // 兜底 30s
	}

	// 下载需要根据配置文件设置较长的超时。
	// timeout 必须是已验证的正值：download-timeout 缺省为 0 时（OriginDefaultConfig 未设该字段），
	// 直接 WithTimeout(_, 0) 会让 ctx 立刻过期，所有节点都判 speed_fail，
	// 上面 client.Timeout 的 30s 兜底救不回来。
	ctxTimeout := timeout
	if ctxTimeout <= 0 {
		ctxTimeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, 0, err
	}

	// 设置请求头
	req.Header.Set("User-Agent", convert.RandUserAgent())
	req.Header.Set("Cache-Control", "no-cache")

	// 准备读取器,将起始时间和起始流量的记录移到 Do(req) 之前
	var startNetBytes uint64
	if getNetBytes != nil {
		startNetBytes = getNetBytes()
	}
	startTime := time.Now()

	// 发起请求 (此时开始发生 TCP/TLS 握手)
	resp, err := speedClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, 0, fmt.Errorf("http status %d", resp.StatusCode)
	}

	var limit uint64
	if downloadMB > 0 {
		limit = uint64(downloadMB) * 1024 * 1024
	}

	limitedReader := &networkLimitedReader{
		reader:      resp.Body,
		getNetBytes: getNetBytes,
		startBytes:  startNetBytes,
		limit:       limit,
	}

	// 执行下载读取 (io.Copy)
	// copiedBytes，以便在 getNetBytes 失败时兜底
	copiedBytes, err := io.Copy(io.Discard, limitedReader)

	// 如果错误是“超时”或“EOF”，这是测速的正常结束状态，不应视为 Failure
	// 使用 errors.Is 处理嵌套错误。因为网络超时抛出的错误通常是被包装过的
	if err != nil && err != io.EOF && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		return 0, 0, err
	}

	// 计算耗时
	duration := time.Since(startTime).Seconds()
	if duration < 0.1 {
		duration = 0.1 // 防止除零
	}

	// 计算总流量
	var totalBytes int64
	useNetBytes := false

	// 尝试使用网络层流量（包含 Header、TLS握手、TCP重传等真实流量）
	if getNetBytes != nil {
		curr := getNetBytes()
		if curr >= startNetBytes {
			totalBytes = int64(curr - startNetBytes)
			useNetBytes = true
		}
	}

	// 兜底逻辑：如果无法获取网络层流量，或计算异常，回退到应用层流量
	if !useNetBytes || totalBytes <= 0 {
		totalBytes = copiedBytes
	}

	if totalBytes <= 0 {
		// 即使超时也应该有一点数据，如果完全没数据则报错
		return 0, 0, fmt.Errorf("no bytes transfer")
	}

	// 计算速度 (KB/s)
	speed := int(float64(totalBytes) / 1024.0 / duration)

	slog.Debug("测速完成",
		"speed_KB_s", speed,
		"duration_s", strconv.FormatFloat(duration, 'f', 2, 64),
		"bytes", totalBytes,
		"use_net_bytes", useNetBytes,
	)

	return speed, totalBytes, nil
}
