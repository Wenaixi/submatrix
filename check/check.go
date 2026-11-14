// Package check 订阅检测主逻辑
package check

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/juju/ratelimit"
	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/constant"
	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/samber/lo"
	"github.com/Wenaixi/submatrix/assets"
	"github.com/Wenaixi/submatrix/check/platform"
	"github.com/Wenaixi/submatrix/config"
	proxyutils "github.com/Wenaixi/submatrix/proxy"
	"github.com/Wenaixi/submatrix/utils"
)

// 对外暴露变量，兼容GUI调用
var (
	Progress   atomic.Uint32 // 已检测数量（语义见算法）
	Available  atomic.Uint32 // 已可用数量（测速阶段完成,可用即+1）
	ProxyCount atomic.Uint32 // 总数（动态=总节点；分阶段=当前阶段规模）

	TotalBytes     atomic.Uint64
	UP             atomic.Uint64
	DOWN           atomic.Uint64
	ForceClose     atomic.Bool
	Successlimited atomic.Bool
	ProcessResults atomic.Bool

	Bucket *ratelimit.Bucket

	CheckStartTime    time.Time
	CheckEndTime      time.Time
	CheckDuration     time.Duration
	CheckTrafficTotal string

	Fetching atomic.Bool // 拉取订阅阶段
	Checking atomic.Bool // 检测阶段
)

// 存储测速和流媒体检测开关状态
var (
	speedON        bool
	mediaON        bool
	progressWeight ProgressWeight
)

const MediaCheckMaxRetries = 3

// Result 存储节点检测结果
type Result struct {
	Proxy          map[string]any
	Openai         bool
	OpenaiWeb      bool
	Copilot        bool
	CopilotAPI     bool
	X              bool
	Youtube        string
	Netflix        bool
	Google         bool
	Cloudflare     bool
	Disney         bool
	Gemini         platform.GeminiStatus
	TikTok         string
	IP             string
	IPRisk         string
	Country        string
	CountryCodeTag string
	ISPTag         string
	Speed          int
}

// ProxyChecker 处理代理检测的主要结构体
type ProxyChecker struct {
	results     []Result
	resultChan  chan Result
	proxyCount  int
	threadCount int
	available   atomic.Int32

	// grp 是本组当前的参数化配置（组级字段在此读取，全局字段仍走 config.GlobalConfig）
	grp config.ResolvedGroupConfig

	aliveConcurrent int
	speedConcurrent int
	mediaConcurrent int

	aliveChan chan *ProxyJob
	speedChan chan *ProxyJob
	mediaChan chan *ProxyJob

	pt *ProgressTracker

	// 用于回收失败但还有分数的历史节点、以及本次成功的节点
	historyChan chan map[string]any
	HistoryPool []map[string]any
	historyWg   sync.WaitGroup
}

// ProxyJob 在测活-测速-流媒体检测任务间传输信息
type ProxyJob struct {
	Client *ProxyClient
	Result Result

	CfLoc string
	CfIP  string

	doneOnce sync.Once

	aliveMarked atomic.Bool
	speedMarked atomic.Bool
	mediaMarked atomic.Bool

	Speed int

	NeedCF         bool
	IsCfAccessible bool

	GoogleCountry string // policies.google.com 预取的国家码（alpha-2），供 youtube/gemini 共享
}

// Close 确保 ProxyJob 的底层资源(mihomo客户端)被正确释放一次。
func (job *ProxyJob) Close() {
	job.doneOnce.Do(func() {
		if job.Client != nil {
			// 在关闭前读取该节点消耗的总流量
			traffic := job.Client.BytesRead.Load() + job.Client.BytesWritten.Load()
			if traffic > 0 && job.Result.Proxy != nil {
				if subURL, ok := job.Result.Proxy["sub_url"].(string); ok && subURL != "" {
					proxyutils.SubStatsMutex.Lock()
					st := proxyutils.SubStats[subURL]
					st.Traffic += traffic // 累加流量到订阅统计中
					proxyutils.SubStats[subURL] = st
					proxyutils.SubStatsMutex.Unlock()
				}
			}

			job.Client.Close()
			job.Client = nil // 切断对底层资源的引用
		}
		// 切断map引用，释放内存
		job.Result.Proxy = nil
		job.Result = Result{}
	})
}

// calcSpeedConcurrency 根据总速度限制计算速度测试的最佳并发数。
func calcSpeedConcurrency(proxyCount, minSpeed int) int {
	if config.GlobalConfig.TotalSpeedLimit <= 0 {
		threadCount := min(proxyCount, config.GlobalConfig.Concurrent)
		fnSpeed := NewPowerDecay(32, 1.1, 32, 1)
		return min(config.GlobalConfig.Concurrent, RoundInt(fnSpeed(float64(threadCount))))
	}
	L := float64(config.GlobalConfig.TotalSpeedLimit) // 单位: MB/s
	r := float64(minSpeed) / 1024                     // 目标每线程吞吐: MB/s
	c := max(int(L/r), 1)
	c = min(c, config.GlobalConfig.Concurrent)
	return c
}

// NewProxyChecker 创建新的检测器实例
func NewProxyChecker(proxyCount int, grp config.ResolvedGroupConfig) *ProxyChecker {
	threadCount := min(proxyCount, config.GlobalConfig.Concurrent)

	cAlive := config.GlobalConfig.AliveConcurrent
	cSpeed := config.GlobalConfig.SpeedConcurrent
	cMedia := config.GlobalConfig.MediaConcurrent

	// 分别设置测活\测速\媒体检测阶段并发数
	// 使用衰减算法,简单防呆设计
	aliveConc := 0
	speedConc := 0
	mediaConc := 0

	// 如果明确设置了正数
	if cAlive > 0 && cSpeed > 0 && cMedia > 0 {
		aliveConc = min(cAlive, proxyCount)
		speedConc = min(cSpeed, proxyCount)
		mediaConc = min(cMedia, proxyCount)
	} else {
		// 自动模式
		// 使用相对平滑的衰减方案
		fnAlive := NewLogDecay(400, 0.005, 400)
		fnMedia := NewExpDecay(400, 0.001, 100)
		if !grp.CheckSpeed {
			fnMedia = NewExpDecay(400, 0.001, 150)
		}

		aliveConc = min(proxyCount, RoundInt(fnAlive(float64(threadCount))))
		speedConc = min(calcSpeedConcurrency(proxyCount, grp.MinSpeed), proxyCount)
		mediaConc = min(proxyCount, RoundInt(fnMedia(float64(threadCount))))

		// 超大线程数
		if threadCount > 1000 {
			slog.Info("除非你的 CPU 和路由器同时允许, 超过 1000 并发可能影响其它上网程序,如确有需求,请在配置文件分别指定测活-测速-媒体检测每个阶段并发数")
			slog.Info("已限制测活并发数", "并发", aliveConc)
		}
	}

	var speedChanLength int
	// 测速阶段的缓冲通道不用太大,以形成阻塞,避免测活浪费资源
	fnScLength := NewTanhDecay(100, 0.0004, float64(aliveConc))
	speedChanLength = RoundInt(fnScLength(float64(speedConc)))
	if !grp.CheckSpeed {
		speedChanLength = 1 // 不启用测速时，设置为最小缓冲
	}

	return &ProxyChecker{
		proxyCount:  proxyCount,
		threadCount: threadCount,
		grp:         grp,

		// 设置不同检测阶段的并发数
		aliveConcurrent: aliveConc,
		speedConcurrent: speedConc,
		mediaConcurrent: mediaConc,

		// 设置缓冲通道
		aliveChan: make(chan *ProxyJob, int(float64(aliveConc)*1.2)),
		speedChan: make(chan *ProxyJob, speedChanLength),
		mediaChan: make(chan *ProxyJob, mediaConc*2),

		// 设置进度跟踪
		pt: NewProgressTracker(proxyCount),

		historyChan: make(chan map[string]any, proxyCount), // 缓冲设为最大值防止阻塞
		HistoryPool: make([]map[string]any, 0),
	}
}

// CheckGroup 按单个组的配置执行代理检测
// 组级字段来自 grp，全局字段仍读 config.GlobalConfig；串行调用，无并发残留。
func CheckGroup(grp config.ResolvedGroupConfig) ([]Result, []map[string]any, error) {
	proxyutils.ResetRenameCounter()
	ForceClose.Store(false)
	Successlimited.Store(false)
	ProcessResults.Store(false)

	ProxyCount.Store(0)
	Available.Store(0)
	Progress.Store(0)

	TotalBytes.Store(0)

	// 重置预计剩余时间计算
	ResetETA()

	// 初始化测速和流媒体检测开关（派生于组配置）
	speedON = grp.CheckSpeed
	mediaON = grp.MediaCheck

	// 标记订阅获取阶段开始
	Fetching.Store(true)
	CurrentStepName.Store("解析组订阅")
	// 按组拉取节点（不再使用全局 GetProxies · 组 URLs 独立拉取合并去重）
	proxies, rawCount, err := proxyutils.FetchGroupProxies(grp)
	if err != nil {
		return nil, nil, fmt.Errorf("获取组订阅失败: %w", err)
	}
	Progress.Store(uint32(len(proxies)))
	ProxyCount.Store(uint32(rawCount))
	Available.Store(0)
	slog.Info("已获取节点", "数量", rawCount)
	slog.Info("去重后节点", "数量", len(proxies))

	proxyutils.ClearCache()

	debug.FreeOSMemory()

	Available.Store(0)
	Progress.Store(0)
	ProxyCount.Store(0)

	CurrentStepName.Store("节点乱序")

	// 假设有 15 个相似的 IP/域名
	calcMinSpacing := max(config.GlobalConfig.Concurrent*5, len(proxies)/15)

	// 随机乱序并根据 IP/域名/真实Host 打乱节点顺序, 减少测速直接测死的概率
	cfg := proxyutils.ShuffleConfig{
		Threshold:  float64(config.GlobalConfig.Threshold),
		Passes:     3,
		MinSpacing: calcMinSpacing,
		ScanLimit:  config.GlobalConfig.Concurrent * 2,
	}

	// 获取动态映射的 CIDR 和 域名层级 文本
	cidr, domainLevel := proxyutils.ThresholdToLevel(cfg.Threshold)

	// 全量乱序（无上次成功节点优先排序）
	proxyutils.SmartShuffleByServer(proxies, cfg)
	slog.Info(fmt.Sprintf("节点乱序, 相同 CIDR %s 或 %s 最小间距: %d", cidr, domainLevel, cfg.MinSpacing))

	CurrentStepName.Store("获取订阅完成")
	if len(proxies) == 0 {
		slog.Info("没有需要检测的节点")
		return nil, nil, nil
	}

	// 标记订阅获取阶段结束
	Fetching.Store(false)
	Checking.Store(true)

	checker := NewProxyChecker(len(proxies), grp)

	results, historyPool, err := checker.run(proxies)
	checker = nil //nolint:ineffassign
	return results, historyPool, err
}

// Run 运行检测流程
func (pc *ProxyChecker) run(proxies []map[string]any) ([]Result, []map[string]any, error) {
	CurrentStepName.Store("初始化检测")
	// 限速设置
	limit := config.GlobalConfig.TotalSpeedLimit
	if limit <= 0 {
		limit = 100 // 默认最大 100MB/s
	}
	rate := float64(limit) * 1024 * 1024
	capacity := int64(rate / 10)
	Bucket = ratelimit.NewBucketWithRate(rate, capacity)

	// // 初始化全局上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 如果 MaxMindDBPath 为空会自动使用 subs-check-pro 内置数据库
	geoDB, err := assets.OpenMaxMindDB(config.GlobalConfig.MaxMindDBPath)
	if err != nil {
		slog.Debug(fmt.Sprintf("打开 MaxMind 数据库失败: %v", err))
		geoDB = nil
	}

	// 确保数据库在函数结束时关闭
	if geoDB != nil {
		defer func() {
			if err := geoDB.Close(); err != nil {
				slog.Debug(fmt.Sprintf("关闭 MaxMind 数据库失败: %v", err))
			}
		}()
	}

	slog.Info("开始检测节点")

	// 记录开始检测时间
	CheckStartTime = time.Now()

	// 组装参数
	args := []any{
		"enable-speedtest", speedON,
		"media-check", mediaON,
		"drop-bad-cf-nodes", config.GlobalConfig.DropBadCfNodes,
	}

	// 流水线并发参数
	if config.GlobalConfig.AliveConcurrent <= 0 || config.GlobalConfig.SpeedConcurrent <= 0 || config.GlobalConfig.MediaConcurrent <= 0 {
		args = append(args,
			"auto-concurrent", true, "concurrent", config.GlobalConfig.Concurrent,
			":alive", pc.aliveConcurrent,
		)
		if speedON {
			args = append(args, ":speed", pc.speedConcurrent)
		}
		if mediaON {
			args = append(args, ":media", pc.mediaConcurrent)
		}
	} else {
		args = append(args,
			"concurrent", config.GlobalConfig.Concurrent,
			":alive", pc.aliveConcurrent)
		if speedON {
			args = append(args, ":speed", pc.speedConcurrent)
		}
		if mediaON {
			args = append(args, ":media", pc.mediaConcurrent)
		}
	}
	// 只有在 >0 时才输出
	if pc.grp.SuccessLimit > 0 {
		args = append(args, "success-limit", pc.grp.SuccessLimit)
	}
	if config.GlobalConfig.TotalSpeedLimit > 0 && speedON {
		args = append(args, "total-speed-limit", config.GlobalConfig.TotalSpeedLimit)
	}

	// 再追加剩余参数。timeout 优先取组级值，组级未配置时回退全局。
	args = append(args,
		"timeout", pc.effectiveTimeout()*1000,
	)

	if speedON {
		args = append(args,
			"min-speed", pc.grp.MinSpeed,
			"download-timeout", config.GlobalConfig.DownloadTimeout,
			"download-mb", pc.grp.DownloadMB,
		)
	}

	if config.GlobalConfig.UseLastResult {
		args = append(args, "use-last-result", true)
	}

	if config.GlobalConfig.UseHistoryResult {
		args = append(args, "use-history-result", true)
	}

	args = append(args, "analysis", "true")

	// 添加 isp 检测参数
	cfgISP := config.GlobalConfig

	hasISPAPIKey := cfgISP.ISPCheckAPIKeyIPAPI != "" || cfgISP.ISPCheckAPIKeyIPData != "" || cfgISP.ISPCheckAPIKeyIPLocate != "" || cfgISP.ISPCheckAPIKeyProxyCheck != ""

	isISPCheck := cfgISP.ISPCheck && hasISPAPIKey

	args = append(args, "isp-check", isISPCheck)

	if config.GlobalConfig.SuccessRate > 0 {
		r := fmt.Sprintf("%.1f%%", config.GlobalConfig.SuccessRate*100)
		args = append(args, "success-rate", r)
	}

	enableIPv6 := config.GlobalConfig.EnableIPv6

	args = append(args, "ipv6", enableIPv6)

	// 设置 mihomo 变量及注入全局自定义 DNS
	SetupMihomoDNS()

	hasCustomDNS := len(config.GlobalConfig.NameServer) > 0 || len(config.GlobalConfig.ProxyServerNameserver) > 0 || len(config.GlobalConfig.ProxyServerNameserverPolicy) > 0

	if hasCustomDNS {
		args = append(args, "custom-dns", hasCustomDNS)
	}

	// 最终日志调用
	slog.Info("当前参数", args...)

	// 监测 ForceClose
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if ForceClose.Load() {
					slog.Warn("用户手动结束检测,等待收集结果")
					cancel()
					return
				}
			}
		}
	}()

	// // 进度显示 —— 使用关闭信号并等待 showProgress 完成
	// doneCh := make(chan struct{})
	// finishedCh := make(chan struct{})

	// if config.GlobalConfig.PrintProgress {
	// 	go func() {
	// 		pc.showProgress(doneCh)
	// 		close(finishedCh)
	// 	}()
	// }

	// 计算预计剩余时间
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				UpdateETA()
				return
			case <-ticker.C:
				SnapshotRate()
				UpdateETA()
			}
		}
	}()

	CurrentStepName.Store("进度")

	// 启动本机网络健康探针：独立于代理，直连探测本机网络是否被检测流量拖垮。
	// 使用独立 ctx，不随「达到成功数量限制」的 cancel 提前停止，检测收尾后统一关闭。
	probeCtx, probeCancel := context.WithCancel(context.Background())
	defer probeCancel()
	netGuardOn := config.GlobalConfig.NetGuard
	// 三个阶段各一个闸门，容量即各自配置的自然并发上限（网络健康时不产生任何额外限制）：
	//   测活 = 测活并发；测速 = 测速并发；
	//   媒体 = 媒体并发 × 平台数（每个节点会同时跑多个平台检测，闸门按「单平台任务」计）。
	// 复核预算 = 总节点数的 5%（至少 20），避免死节点被整体复测拖慢检测。
	// 由配置项 net-guard 控制，关闭后不启动探针，Acquire/Release/WaitSettled 均为空操作。
	if netGuardOn {
		caps := utils.GateCaps{Alive: pc.aliveConcurrent}
		if speedON {
			caps.Speed = pc.speedConcurrent
		}
		if mediaON {
			caps.Media = pc.mediaConcurrent * len(pc.grp.Platforms)
		}
		utils.NetGuard.Start(probeCtx, caps, max(20, pc.proxyCount/20))
	}

	pc.historyWg.Add(1)
	go func() {
		defer pc.historyWg.Done()
		for node := range pc.historyChan {
			pc.HistoryPool = append(pc.HistoryPool, node)
		}
	}()

	// 启动流水线阶段
	go pc.distributeJobs(proxies, ctx)
	go pc.runAliveStage(ctx, geoDB)
	go pc.runSpeedStage(ctx, cancel)
	pc.runMediaStageAndCollect(geoDB, ctx, cancel)

	// 流水线已结束，停止探针，并输出保护机制是否介入的摘要，便于定位「变慢」是否与保护有关
	probeCancel()
	if netGuardOn {
		if text, intervened := utils.NetGuard.Summary(); intervened {
			slog.Info(text)
		}
	}

	// 关闭历史节点收集并按分数排序
	close(pc.historyChan)
	pc.historyWg.Wait()

	sort.Slice(pc.HistoryPool, func(i, j int) bool {
		scoreI, _ := pc.HistoryPool[i]["history_score"].(int)
		scoreJ, _ := pc.HistoryPool[j]["history_score"].(int)
		return scoreI > scoreJ // 降序，最高分的排在前面
	})

	CurrentStepName.Store("处理结果")

	// 确保进度显示到 100%
	pc.pt.Finalize()

	if pc.grp.SuccessLimit > 0 && pc.available.Load() >= pc.grp.SuccessLimit {
		slog.Info(fmt.Sprintf("达到成功节点数量限制 %d, 收集结果完成。", pc.grp.SuccessLimit))
	}

	// 标记检测完成，开始处理结果，保存，上传等
	ProcessResults.Store(true)

	// 重置预计剩余时间计算
	ETASeconds.Store(0)

	// 计算本次检测所有订阅文件的体积
	var totalSubSize uint64
	for _, stat := range proxyutils.SubStats {
		totalSubSize += uint64(stat.Size)
	}

	totalSubSizeStr := utils.FormatTraffic(totalSubSize)

	slog.Info(fmt.Sprintf("可用节点数量: %d", len(pc.results)))

	totalBytes := TotalBytes.Load()

	CheckTrafficTotal = utils.FormatTraffic(totalBytes)

	// 总流量 = 订阅文件大小 + 检测流量
	totalTraffic := totalSubSize + totalBytes
	totalTrafficStr := utils.FormatTraffic(totalTraffic)

	slog.Info(fmt.Sprintf("检测消耗流量: %s", CheckTrafficTotal))

	slog.Debug("流量", "UP", UP.Load(), "DOWN", DOWN.Load())

	// 计算检测用时
	CheckEndTime = time.Now()
	CheckDuration = time.Since(CheckStartTime)

	CurrentStepName.Store("生成检测报告")
	time.Sleep(10 * time.Second)

	// 仅当本次检测中出现过拥塞，才等本机网络（NAT 连接表 / DNS / 队列）恢复后再保存、上传、通知，
	// 否则紧随其后的请求容易撞上残余拥塞而超时。没出现过拥塞时立即返回，最多等 15s。
	utils.NetGuard.WaitSettled(context.Background(), 15*time.Second)

	// 1. 深度分析 (利用上一步的成功率进行排序，生成 analysis yaml)
	pc.GenerateAnalysisReport()

	slog.Info("消耗流量统计", "总计", totalTrafficStr, "订阅", totalSubSizeStr, "检测", CheckTrafficTotal)

	// 2. 清理元数据 (删除 sub_url 等字段，防止污染最终配置)
	pc.CleanupMetadata()

	// 手动解除引用
	for i := range proxies {
		proxies[i] = nil
	}

	Bucket = nil //nolint:ineffassign

	CurrentStepName.Store("释放检测内存")

	// 在保存上传之前直接归还内存
	debug.FreeOSMemory()

	// 标记结束
	Checking.Store(false)

	return pc.results, pc.HistoryPool, nil
}

// distributeJobs 分发代理任务
func (pc *ProxyChecker) distributeJobs(proxies []map[string]any, ctx context.Context) {
	defer close(pc.aliveChan)

	concurrency := min(pc.proxyCount, pc.aliveConcurrent)
	var wg sync.WaitGroup

	// 使用原子索引来分发任务
	var proxyIndex atomic.Int64
	proxyIndex.Store(-1) // 初始化为 -1

	// 定义主动 GC 的阈值
	var gcThreshold = config.GlobalConfig.GCThreshold
	if gcThreshold == 0 {
		gcThreshold = 20000
	}

	// 启动工作协程池
	for range concurrency {
		wg.Go(func() {
			for {
				// 原子地获取下一个代理索引
				index := proxyIndex.Add(1)
				if index >= int64(len(proxies)) {
					return // 所有代理都已处理完毕
				}

				if checkCtxDone(ctx) {
					// 收到终止信号，将当前队列未处理的历史节点无伤抢救下来
					pc.scoreAndRecycle(proxies[index], "untested", 0, nil)
					proxies[index] = nil
					continue // 这里用 continue 继续消耗数组完成抢救
				}

				mapping := proxies[index]

				// 任务取出后，立即断开源切片的引用
				proxies[index] = nil

				// 周期性强制归还内存
				if index > 0 && index%gcThreshold == 0 {
					go func(currentIdx int64) {
						slog.Debug("已处理 " + strconv.FormatInt(currentIdx, 10) + " 个节点，正在执行主动内存回收...")
						debug.FreeOSMemory()
					}(index)
				}

				// 组级前置代理注入：测活与测速均走此代理
				if pc.grp.Proxy != "" {
					mapping["group_proxy"] = pc.grp.Proxy
				}

				// 协议过滤命中（拉取阶段已打 skipped 标记）：不测活不测速，直接放行落盘
				if skipped, _ := mapping["skipped"].(bool); skipped {
					pc.pt.CountAlive(true)
					r := Result{Proxy: mapping}
					pc.scoreAndRecycle(mapping, "success", 0, &r)
					pc.resultChan <- r
					continue
				}

				// 4A 核心免检直通深层重构：若所属组已关闭连通测活，彻底跳过网络握手，0 网络 I/O 瞬间放行
				if !pc.grp.CheckAlive {
					pc.pt.CountAlive(true)
					r := Result{Proxy: mapping}
					pc.scoreAndRecycle(mapping, "success", 0, &r)
					pc.resultChan <- r
					continue
				}

				cli := CreateClient(mapping, pc.effectiveTimeout())
				if cli == nil {
					// 创建失败：视为 alive 完成（失败），不进入 speed/media
					pc.pt.CountAlive(false)
					// 底层生成故障视为死节点
					pc.scoreAndRecycle(mapping, "alive_fail", 0, nil)
					continue
				}

				job := &ProxyJob{
					Client: cli,
					Result: Result{Proxy: mapping},
				}
				job.NeedCF = config.GlobalConfig.DropBadCfNodes ||
					(pc.grp.MediaCheck && needsCF(pc.grp.Platforms))

				// 当 aliveChan 满时会阻塞
				select {
				case pc.aliveChan <- job:
				case <-ctx.Done():
					job.Close()
					return
				}
			}
		})
	}

	// 等待所有工作协程完成
	wg.Wait()
	// 分发结束，再次强制清理（此时 proxies 切片虽然 length 很大，但全是 nil）
	debug.FreeOSMemory()
}

// 测活
func (pc *ProxyChecker) runAliveStage(ctx context.Context, db *maxminddb.Reader) {
	// 根据是否启用测速，延迟关闭下一个阶段的通道。
	if speedON {
		defer close(pc.speedChan)
	} else {
		close(pc.speedChan)
		defer close(pc.mediaChan)
	}

	var wg sync.WaitGroup
	concurrency := pc.aliveConcurrent
	pc.pt.currentStage.Store(0)

	for range concurrency {
		wg.Go(func() {
			for job := range pc.aliveChan {
				if checkCtxDone(ctx) {
					if job.aliveMarked.CompareAndSwap(false, true) {
						pc.pt.CountAlive(false)
					}
					pc.scoreAndRecycle(job.Result.Proxy, "untested", 0, nil) // 抢救
					job.Close()
					continue
				}
				// 自适应闸门（net-guard 关闭或网络健康时几乎零开销）：
				// 仅当本机网络真的连不上外网时才收紧同时在途的测活数（最低保留 1/2），恢复后自动加回。
				if !utils.NetGuard.Acquire(ctx, utils.GateAlive) {
					if job.aliveMarked.CompareAndSwap(false, true) {
						pc.pt.CountAlive(false)
					}
					pc.scoreAndRecycle(job.Result.Proxy, "untested", 0, nil) // 抢救
					job.Close()
					continue
				}

				// 节点测活
				isAlive := checkAlive(job, ctx)

				// 拥塞期间失败：用有限的预算复核一次，避免把「本来存活」的节点误判为死亡。
				// 不等待网络恢复，只做一次短暂抖动后重测，且预算用尽后不再复核。
				if !isAlive && !checkCtxDone(ctx) && utils.NetGuard.TakeRecheck() {
					slog.Debug("拥塞期间测活失败，复核一次", "Name", job.Client.mProxy.Name())
					select {
					case <-ctx.Done():
					case <-time.After(time.Duration(300+rand.IntN(500)) * time.Millisecond):
					}
					if !checkCtxDone(ctx) {
						isAlive = checkAlive(job, ctx)
					}
				}
				utils.NetGuard.Release(utils.GateAlive)

				if !isAlive {
					// 记录非存活
					if job.aliveMarked.CompareAndSwap(false, true) {
						pc.pt.CountAlive(false)
					}
					pc.scoreAndRecycle(job.Result.Proxy, "alive_fail", 0, nil) // 失败打分扣除
					job.Close()
					continue // 不进入 speed/media
				}

				// CF 过滤
				if job.NeedCF {
					job.IsCfAccessible, job.CfLoc, job.CfIP = platform.CheckCloudflare(job.Client.Client)
					if config.GlobalConfig.DropBadCfNodes && !job.IsCfAccessible {
						// 记录丢弃
						if job.aliveMarked.CompareAndSwap(false, true) {
							pc.pt.CountAlive(false)
						}
						pc.scoreAndRecycle(job.Result.Proxy, "alive_fail", 0, nil) // 视为故障
						job.Close()
						continue
					}
				}

				// 地区过滤
				if !job.checkJobLocation(db, ctx) {
					pc.scoreAndRecycle(job.Result.Proxy, "untested", 0, nil) // 未通过地区过滤，算作无伤丢弃，保留原分数
					job.Close()
					continue
				}

				// 通过全部过滤，记录为存活
				if job.aliveMarked.CompareAndSwap(false, true) {
					pc.pt.CountAlive(true)
				}

				// 流转
				if speedON {
					select {
					case pc.speedChan <- job:

					case <-ctx.Done():
						job.Close()
					}
				} else {
					// 无测速时：通过 alive 即可视为“可用”，确保 Available 与最终可用数量一致
					if job.speedMarked.CompareAndSwap(false, true) {
						pc.incrementAvailable()
					}
					select {
					case pc.mediaChan <- job:
					case <-ctx.Done():
						job.Close()
					}
				}
			}
		})
	}
	wg.Wait()

	// alive 阶段闭合：分阶段算法据此确定下一阶段规模
	pc.pt.FinishAliveStage()
}

// 测速
func (pc *ProxyChecker) runSpeedStage(ctx context.Context, cancel context.CancelFunc) {
	if !speedON {
		return
	}
	defer close(pc.mediaChan)

	// 确保达到成功节点数量限制的日志只输出一次
	var stopOnce sync.Once

	var wg sync.WaitGroup
	concurrency := pc.speedConcurrent

	for range concurrency {
		wg.Go(func() {
			for job := range pc.speedChan {
				if checkCtxDone(ctx) {
					if job.speedMarked.CompareAndSwap(false, true) {
						pc.pt.CountSpeed(false)
					}
					pc.scoreAndRecycle(job.Result.Proxy, "untested", 0, nil) // 抢救
					job.Close()
					continue
				}
				// 测速闸门：本机网络拥塞时收紧同时下载的节点数（网络健康时不阻塞）
				if !utils.NetGuard.Acquire(ctx, utils.GateSpeed) {
					if job.speedMarked.CompareAndSwap(false, true) {
						pc.pt.CountSpeed(false)
					}
					pc.scoreAndRecycle(job.Result.Proxy, "untested", 0, nil) // 抢救
					job.Close()
					continue
				}
				getBytes := func() uint64 { return job.Client.BytesRead.Load() }
				speed, _, err := platform.CheckSpeed(job.Client.Client, Bucket, getBytes, pc.grp.SpeedTestURL, config.GlobalConfig.DownloadTimeout, pc.grp.DownloadMB)
				utils.NetGuard.Release(utils.GateSpeed)
				success := err == nil && speed >= pc.grp.MinSpeed
				if job.speedMarked.CompareAndSwap(false, true) {
					pc.pt.CountSpeed(success)
					// 仅在测速成功时计入可用数量
					if success {
						pc.incrementAvailable()
					}
				}
				if !success {
					pc.scoreAndRecycle(job.Result.Proxy, "speed_fail", 0, nil) // 失败打分扣除
					job.Close()
					continue
				}
				job.Speed = speed

				if pc.grp.SuccessLimit > 0 && pc.available.Load() >= pc.grp.SuccessLimit {
					stopOnce.Do(func() {
						Successlimited.Store(true)
						var msg string
						if mediaON {
							if speedON {
								msg = "达到成功节点数量限制 " +
									strconv.Itoa(int(pc.grp.SuccessLimit)) +
									", 等待测速和媒体检测任务完成..."
							} else {
								msg = "达到成功节点数量限制 " +
									strconv.Itoa(int(pc.grp.SuccessLimit)) +
									", 等待媒体检测任务完成..."
							}
						} else {
							if speedON {
								msg = "达到成功节点数量限制 " +
									strconv.Itoa(int(pc.grp.SuccessLimit)) +
									", 等待测速和节点重命名任务完成..."
							} else {
								msg = "达到成功节点数量限制 " +
									strconv.Itoa(int(pc.grp.SuccessLimit)) +
									", 等待节点重命名任务完成..."
							}
						}
						slog.Warn(msg)
						cancel()
					})
				}

				// 流转
				pc.mediaChan <- job
			}
		})
	}
	wg.Wait()

	// speed 阶段闭合：分阶段算法据此确定媒体阶段规模
	pc.pt.FinishSpeedStage()
}

// 流媒体检测 + 收集结果
func (pc *ProxyChecker) runMediaStageAndCollect(db *maxminddb.Reader, ctx context.Context, cancel context.CancelFunc) {
	var wg sync.WaitGroup
	resultLength := pc.mediaConcurrent
	if pc.grp.SuccessLimit != 0 {
		resultLength = int(pc.grp.SuccessLimit)
	}

	pc.resultChan = make(chan Result, resultLength)

	// 设置成功数量限制
	var stopOnce sync.Once

	// 收集结果
	var collectorWg sync.WaitGroup
	collectorWg.Go(func() {
		pc.collectResults()
	})

	// 启动 workers（确保 collector 已启动以避免阻塞在无缓冲时）
	concurrency := pc.mediaConcurrent
	for range concurrency {
		wg.Go(func() {
			for job := range pc.mediaChan {
				if !speedON {
					// 只在没开启测速时接受媒体检测停止信号
					// 丢弃结果
					if checkCtxDone(ctx) {
						if job.mediaMarked.CompareAndSwap(false, true) {
							pc.pt.CountMedia()
						}
						pc.scoreAndRecycle(job.Result.Proxy, "untested", 0, nil) // 抢救
						job.Close()
						continue
					}

					// 设置成功数量限制
					if pc.grp.SuccessLimit > 0 && pc.available.Load() >= pc.grp.SuccessLimit {
						stopOnce.Do(func() {
							Successlimited.Store(true)
							pc.pt.FinishAliveStage()
							if mediaON {
								Successlimited.Store(true)
								slog.Warn(fmt.Sprintf("达到成功节点数量限制 %d, 等待媒体检测任务完成...", pc.grp.SuccessLimit))
								slog.Warn("测活模式将丢弃多余结果")
							} else {
								Successlimited.Store(true)
								slog.Warn(fmt.Sprintf("达到成功节点数量限制 %d, 等待节点重命名任务完成...", pc.grp.SuccessLimit))
								slog.Warn("测活模式将丢弃多余结果")
							}

							cancel()
						})
					}
				}

				if mediaON && !checkCtxDone(ctx) {
					pc.mediaCheck(job, db, ctx)
				}

				pc.updateProxyName(&job.Result, job.Client, job.Speed, db, job.CfLoc, job.CfIP, ctx)

				// 记录节点下载速度
				job.Result.Speed = job.Speed

				// 成功节点打分并入池
				pc.scoreAndRecycle(job.Result.Proxy, "success", job.Speed, &job.Result)

				// 将结果发送到 collector
				pc.resultChan <- job.Result

				if job.mediaMarked.CompareAndSwap(false, true) {
					pc.pt.CountMedia()
				}

				job.Close()
			}
		})
	}

	// 等待所有 worker 完成，再关闭 pc.resultChan，让 collector 退出
	wg.Wait()
	close(pc.resultChan)
	collectorWg.Wait()

	// 最后一次刷新并返回收集结果
	pc.pt.refresh()
	ProcessResults.Store(true)
}

// collectResults 收集检测结果
func (pc *ProxyChecker) collectResults() {
	for result := range pc.resultChan {
		pc.results = append(pc.results, result)
	}
}

// checkAlive 使用谷歌服务执行基本的存活检测。
func checkAlive(job *ProxyJob, ctx context.Context) bool {
	// 【修复 2026-11-12】测活综合判断：gstatic 不通时回退 Cloudflare 端点，
	// 避免容器内无法访问 Google 导致所有节点误判死亡（实测 0 可用根因）。
	alive, err := platform.CheckAliveWithFallback(job.Client.Client, ctx)
	if err == nil && alive {
		return true
	}
	slog.Debug("测活出错", "Name", job.Client.mProxy.Name(), "error", err)
	return false
}

// needsCF 判断所选的媒体检测平台是否需要Cloudflare访问权限。
func needsCF(platforms []string) bool {
	for _, p := range platforms {
		if p == "openai" || p == "x" {
			return true
		}
	}
	return false
}

// mediaCheck 并发检测所有媒体解锁平台
func (pc *ProxyChecker) mediaCheck(job *ProxyJob, db *maxminddb.Reader, ctx context.Context) {
	mediaTimeout := config.GlobalConfig.MediaCheckTimeout
	if mediaTimeout <= 0 {
		mediaTimeout = 10
	}

	mediaClient := &http.Client{
		Transport: job.Client.Transport,
		Timeout:   time.Duration(mediaTimeout) * time.Second,
	}

	plats := pc.grp.Platforms

	// 若同时检测 youtube 或 gemini，提前获取 Google 国家码供两者共享，避免重复请求
	needGoogleCountry := false
	for _, p := range plats {
		if p == "youtube" || p == "gemini" {
			needGoogleCountry = true
			break
		}
	}
	if needGoogleCountry {
		if country, err := platform.GetGoogleCountry(mediaClient); err == nil && country != "" {
			job.GoogleCountry = country
		}
	}

	var wg sync.WaitGroup
	for _, plat := range plats {
		wg.Go(func() {
			// 媒体闸门：按「单平台任务」限制全局并发。这里刻意用 Background 而不是 ctx：
			// 达到成功数量上限后 ctx 会被取消，但已进入媒体阶段的节点仍要完整检测完，不能被闸门跳过。
			utils.NetGuard.Acquire(context.Background(), utils.GateMedia)
			defer utils.NetGuard.Release(utils.GateMedia)
			checkOnePlatform(job, plat, mediaClient, db, ctx)
		})
	}
	wg.Wait()
}

// mediaCheck 根据平台类型分发到相应的检测函数。
func checkOnePlatform(job *ProxyJob, plat string, mediaClient *http.Client, db *maxminddb.Reader, ctx context.Context) {
	switch plat {
	case "x":
		if job.NeedCF && !job.IsCfAccessible {
			break
		}
		job.Result.X = true
	case "openai":
		if job.NeedCF && !job.IsCfAccessible {
			break
		}
		var cookiesOK, clientOK bool
		err := withRetry(ctx, func() error {
			var e error
			cookiesOK, clientOK, e = platform.CheckOpenAI(mediaClient)
			return e
		})
		if err != nil {
			break
		}
		if clientOK && cookiesOK {
			job.Result.Openai = true
		} else if clientOK || cookiesOK {
			job.Result.OpenaiWeb = true
		}
	case "copilot":
		if job.NeedCF && !job.IsCfAccessible {
			break
		}
		var homeOK, apiOK bool
		err := withRetry(ctx, func() error {
			var e error
			homeOK, apiOK, e = platform.CheckCopilot(mediaClient)
			return e
		})
		if err != nil {
			break
		}
		job.Result.Copilot, job.Result.CopilotAPI = homeOK, apiOK
	case "netflix":
		var ok bool
		withRetry(ctx, func() error { //nolint:errcheck
			var e error
			ok, e = platform.CheckNetflix(mediaClient)
			return e
		})
		if ok {
			job.Result.Netflix = true
		}
	case "disney":
		var ok bool
		withRetry(ctx, func() error { //nolint:errcheck
			var e error
			ok, e = platform.CheckDisney(mediaClient)
			return e
		})
		if ok {
			job.Result.Disney = true
		}
	case "youtube":
		var ytRaw string
		withRetry(ctx, func() error { //nolint:errcheck
			var e error
			ytRaw, e = platform.CheckYoutube(mediaClient)
			return e
		})

		// 分离地区码和 Premium 标记
		// ytRaw 可能是: "US" / "US⁻" / "⁻" / "CN" / ""
		ytRegion := strings.TrimSuffix(ytRaw, "⁻")
		ytNoPremium := strings.HasSuffix(ytRaw, "⁻")

		// 地区兜底：无论是否单独检测，逻辑统一
		switch {
		case ytRegion == "CN":
			// CN 封锁，不设置结果
		case ytRegion != "" && job.GoogleCountry != "" && ytRegion != job.GoogleCountry:
			// 两者均有值但不一致，以 YouTube 自身检测为准
			slog.Debug("YouTube地区与Google策略页不一致，以Google策略页为准",
				"youtube", ytRegion, "google_policy", job.GoogleCountry)
			job.Result.Youtube = job.GoogleCountry
		case ytRegion != "":
			job.Result.Youtube = ytRegion
		case job.GoogleCountry != "" && ytRaw != "CN":
			// YouTube 检测失败，降级使用 GoogleCountry
			slog.Debug("YouTube检测失败，降级使用GoogleCountry", "country", job.GoogleCountry)
			job.Result.Youtube = job.GoogleCountry
		}

		// Premium 标记附加在最终地区码上
		if job.Result.Youtube != "" && ytNoPremium {
			job.Result.Youtube += "⁻"
		}

	case "gemini":
		// 主路径：含特征码，可识别 Normal/Blocked/Suspect 三种状态
		var status platform.GeminiStatus
		err := withRetry(ctx, func() error {
			var e error
			status, e = platform.CheckGemini(mediaClient)
			return e
		})

		switch {
		case err == nil && status.Region != "":
			// 完全成功
			job.Result.Gemini = status
		case errors.Is(err, platform.ErrGeminiBotDetected):
			// bot 拦截：不代表地区封锁，降级判断
			// 只能区分 Normal/Blocked，无法识别 Suspect
			if job.GoogleCountry != "" {
				job.Result.Gemini = platform.CheckGeminiByCountry(job.GoogleCountry)
				slog.Debug("Gemini被bot检测拦截，降级判断", "country", job.GoogleCountry)
			}
		case err != nil:
			// 真正的网络错误，不可达，不设置结果
			slog.Debug("Gemini不可达", "error", err)
		default:
			// err==nil 但 Region 为空：页面结构变化，无法解析
			slog.Debug("Gemini响应正常但未解析到地区")
		}
	case "tiktok":
		var region string
		withRetry(ctx, func() error { //nolint:errcheck
			var e error
			region, e = platform.CheckTikTok(mediaClient)
			return e
		})
		if region != "" {
			job.Result.TikTok = region
		}
	case "iprisk":
		// 如果已有 IP，就直接用，不再调用 GetProxyCountry
		if job.Result.IP == "" {
			// iprisk 分支只关心 IP，不需要 ISP 类型标签
			country, ip, countryCodeTag, ispTag, _ := proxyutils.GetProxyCountry(mediaClient, db, ctx, job.CfLoc, job.CfIP, false)
			if ip == "" {
				break
			}
			job.Result.IP = ip
			job.Result.Country = country
			job.Result.CountryCodeTag = countryCodeTag
			job.Result.ISPTag = ispTag
		}
		var risk string
		err := withRetry(ctx, func() error {
			var e error
			risk, e = platform.CheckIPRisk(mediaClient, job.Result.IP)
			return e
		})
		if err == nil {
			job.Result.IPRisk = risk
		} else {
			// 失败的可能性高，所以放上日志
			slog.Debug("查询IP风险失败", "error", err)
		}
	}
}

// updateProxyName 更新代理名称
func (pc *ProxyChecker) updateProxyName(res *Result, httpClient *ProxyClient, speed int, db *maxminddb.Reader, cfLoc string, cfIP string, jctx context.Context) {
	// 位置信息（国家/地区码/ISP 类型）按需获取：
	// 重命名需要它，ISP 类型打标需要它，流媒体标签也要用它判断「节点地区 vs 解锁地区」
	// 是否一致（不一致才加地区后缀）。三者任一开启就查一次，否则 res.Country 恒空，
	// YT/GM/TK 标签会因 `region != ""` 而恒带地区后缀。
	needLocInfo := pc.grp.RenameNode || pc.grp.ISPCheck || pc.grp.MediaCheck
	if needLocInfo && res.Country == "" {
		// 使用 MediaCheckTimeout 替换默认的短超时
		locTimeout := config.GlobalConfig.MediaCheckTimeout
		if locTimeout <= 0 {
			locTimeout = 10
		}
		renameClient := &http.Client{
			Transport: httpClient.Transport,
			Timeout:   time.Duration(locTimeout) * time.Second,
		}

		country, _, countryCodeTag, ispTag, _ := proxyutils.GetProxyCountry(renameClient, db, jctx, cfLoc, cfIP, pc.grp.ISPCheck)
		res.Country = country
		res.CountryCodeTag = countryCodeTag
		res.ISPTag = ispTag
	}

	// 以节点IP查询位置重命名（如果开启）
	if pc.grp.RenameNode {
		if res.Country != "" {
			res.Proxy["name"] = pc.grp.NodePrefix + proxyutils.Rename(res.Country, res.CountryCodeTag, pc.grp.EnhancedTag)
		} else {
			// 位置确实取不到时：一律按未知处理，追加序号，绝不拼接原始名。
			// 原始实现会无条件拼接 originName，导致「国旗+原始名滚雪球」的怪名叠字；
			// append-original-name 开关在此没有消费方（上游行为），这里直接放弃拼接。
			res.Proxy["name"] = pc.grp.NodePrefix + proxyutils.Rename("", "", pc.grp.EnhancedTag)
		}
	}

	name := ""
	if v, ok := res.Proxy["name"].(string); ok {
		name = strings.TrimSpace(v)
	}

	var tags []string
	// 速度标签
	if pc.grp.CheckSpeed && speed > 0 {
		name = regexp.MustCompile(`\s*\|(?:\s*[\d.]+[KM]B/s)`).ReplaceAllString(name, "")
		var speedStr string
		if speed < 100 {
			speedStr = strconv.Itoa(speed) + "KB/s"
		} else {
			speedStr = strconv.FormatFloat(float64(speed)/1024, 'f', 1, 64) + "MB/s"
		}

		tags = append(tags, speedStr)
	}

	if pc.grp.MediaCheck {
		// 移除旧标签
		name = regexp.MustCompile(
			`\s*\|(?:NF|D\+|GPT[⁺]?|CP[⁻]?|GM[⁺]?[ˀ]?(?:-[A-Z]{2})?|X|YT[⁻]?(?:-[A-Z]{2}[⁻]?)?|TK[⁻]?(?:-[^|]+)?|\d+%)`).
			ReplaceAllString(name, "")
	}

	// 平台标签（按用户配置顺序）
	for _, plat := range pc.grp.Platforms {
		switch plat {
		case "openai":
			if res.Openai {
				tags = append(tags, "GPT⁺")
			} else if res.OpenaiWeb {
				tags = append(tags, "GPT")
			}
		case "copilot":
			if res.Copilot && res.CopilotAPI {
				tags = append(tags, "CP")
			} else if res.Copilot {
				tags = append(tags, "CP⁻")
			}
		case "x":
			if res.X && !strings.Contains(name, "⁻¹") && !strings.Contains(name, "🏴‍☠️") {
				tags = append(tags, "X")
			}
		case "netflix":
			if res.Netflix {
				tags = append(tags, "NF")
			}
		case "disney":
			if res.Disney {
				tags = append(tags, "D+")
			}
		case "youtube":
			yt := res.Youtube
			switch yt {
			case "":
				// 不可达或封锁，无标签
			default:
				// 分离地区和 Premium 标记
				region := strings.TrimSuffix(yt, "⁻")
				noPremium := strings.HasSuffix(yt, "⁻")

				tag := "YT"
				if noPremium {
					tag += "⁻"
				}
				if region != "" && region != res.Country {
					tag = "YT-" + region
				}
				tags = append(tags, tag)
			}
		case "gemini":
			g := res.Gemini
			if g.Region == "" {
				break
			}
			switch g.Access {
			case platform.AccessBlocked:
				// 明确封锁，不生成标签
			case platform.AccessSuspect:
				tags = append(tags, "GMˀ")
			case platform.AccessNormal:
				tag := "GM"
				if g.IsEU {
					tag = "GM⁻"
				}
				if g.Region != res.Country {
					tag = tag + "-" + g.Region
				}
				tags = append(tags, tag)
			}
		case "iprisk":
			if res.IPRisk != "" {
				tags = append(tags, res.IPRisk)
			}
		case "tiktok":
			if res.TikTok != "" {
				// 只有TikTok地区和节点位置不一致时才添加TikTok地区
				if res.Country != res.TikTok {
					tags = append(tags, "TK-"+res.TikTok)
				} else {
					tags = append(tags, "TK")
				}
			}
		}
	}

	if tag, ok := res.Proxy["sub_tag"].(string); ok && tag != "" {
		tags = append(tags, tag)
	}

	// 运营商标签
	if pc.grp.ISPCheck {
		if res.ISPTag != "" {
			tags = append(tags, res.ISPTag)
		}
	}

	// 将所有标记添加到名称中
	if len(tags) > 0 {
		name += "|" + strings.Join(tags, "|")
	}

	res.Proxy["name"] = name
}

type ProxyClient struct {
	*http.Client
	baseTransport *http.Transport
	BytesRead     atomic.Uint64
	BytesWritten  atomic.Uint64
	ctx           context.Context
	cancel        context.CancelFunc
	mProxy        constant.Proxy

	// conns 记录该客户端建立的所有底层连接。
	// CloseIdleConnections 只能关闭「空闲」连接，仍在使用/被遗弃的连接会一直挂着
	// （表现为连接数居高不下、路由器连接表被占满），因此 Close 时统一强制关闭。
	conns sync.Map // map[*countingConn]struct{}
}

// effectiveTimeout 返回本组生效的连接超时（毫秒）。
// 组级 Timeout 优先，未配置（<=0）时回退全局值，保持旧配置的行为不变。
func (pc *ProxyChecker) effectiveTimeout() int {
	if pc.grp.Timeout > 0 {
		return pc.grp.Timeout
	}
	return config.GlobalConfig.Timeout
}

// CreateClient 创建独立的代理客户端。
// timeoutMS 为该组生效的连接超时（毫秒），由调用方用 effectiveTimeout 计算后传入，
// 使组级 timeout 真正作用于 dial 与 client 两级。
func CreateClient(mapping map[string]any, timeoutMS int) (client *ProxyClient) {
	// 捕获 panic，防止由于底层库解析畸形节点导致整个程序崩溃
	defer func() {
		if r := recover(); r != nil {
			name, _ := mapping["name"].(string)
			slog.Debug("底层mihomo创建代理Client时发生Panic，已自动丢弃该畸形节点", "name", name, "panic", r)
			client = nil // 发生panic时，向外层返回 nil
		}
	}()

	pc := &ProxyClient{}

	var err error

	// 解析代理：如果该节点所属分组配置了专用代理，则注入 GroupProxyDialer 前置代理拨号器
	if groupProxy, ok := mapping["group_proxy"].(string); ok && strings.TrimSpace(groupProxy) != "" {
		if dialer, dialErr := NewGroupProxyDialer(groupProxy); dialErr == nil && dialer != nil {
			pc.mProxy, err = adapter.ParseProxy(mapping, adapter.WithDialerForAPI(dialer))
		} else {
			pc.mProxy, err = adapter.ParseProxy(mapping)
		}
	} else {
		pc.mProxy, err = adapter.ParseProxy(mapping)
	}

	if err != nil {
		slog.Debug("底层mihomo创建代理Client失败", "error", err)
		return nil
	}

	// 初始化全局控制 Context
	pc.ctx, pc.cancel = context.WithCancel(context.Background())
	// 捕获 ctx 用于闭包，防止 pc 指针后续变化（防御性，避免某次测试出现的nil指针错误）
	clientCtx := pc.ctx

	networkLimitDefault := true

	baseTransport := &http.Transport{
		DialContext: func(reqCtx context.Context, network, addr string) (net.Conn, error) {
			// 【修复 2026-11-12】拨号+握手用 ctx 硬超时（默认 15s，可被后续拨号更快返回覆盖），
			// 防止 mProxy 内部 uTLS 握手 socket 读永久阻塞（大规模检测卡死根因）。
			// 注意：不在此处给连接设永久 deadline——连接会被 http.Transport 复用做后续请求，
			// 绝对 deadline 会误杀正常复用（测活+测速都走同一 transport）。
			dialTimeout := 15 * time.Second
			if cfgTmo := timeoutMS; cfgTmo > 0 {
				dialTimeout = time.Duration(cfgTmo) * time.Millisecond
			}
			mergedCtx, mergedCancel := context.WithTimeout(reqCtx, dialTimeout)

			// 使用 context.AfterFunc 监听全局 clientCtx
			// 当 clientCtx (ProxyClient) 被关闭时，立即调用 mergedCancel
			stop := context.AfterFunc(clientCtx, func() {
				mergedCancel()
			})

			// 资源清理
			// 无论拨号成功还是失败，函数返回时：
			// 1. mergedCancel(): 释放 mergedCtx 资源
			// 2. stop(): 注销监听器（cancel 幂等，顺序无实际影响）
			defer mergedCancel()
			defer stop()

			host, portStr, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			var u16Port uint16
			if port, err := strconv.ParseUint(portStr, 10, 16); err == nil {
				u16Port = uint16(port)
			}

			// 使用合并后的 ctx 进行拨号
			rawConn, err := pc.mProxy.DialContext(mergedCtx, &constant.Metadata{
				Host:    host,
				DstPort: u16Port,
			})
			if err != nil {
				return nil, err
			}

			cc := &countingConn{
				Conn:         rawConn,
				readCounter:  &pc.BytesRead,
				writeCounter: &pc.BytesWritten,
				networkLimit: networkLimitDefault,
			}
			cc.onClose = func() { pc.conns.Delete(cc) }
			pc.conns.Store(cc, struct{}{})

			// 拨号期间客户端已被关闭：立即回收，避免 Close 之后才建立成功的连接泄漏
			if clientCtx.Err() != nil {
				_ = cc.Close()
				return nil, clientCtx.Err()
			}
			return cc, nil
		},
		ForceAttemptHTTP2:   true, // 强制尝试 HTTP/2 协议
		DisableKeepAlives:   false,
		Proxy:               nil,
		IdleConnTimeout:     5 * time.Second,
		MaxIdleConnsPerHost: 5,
	}

	pc.baseTransport = baseTransport
	pc.Client = &http.Client{
		Timeout:   time.Duration(timeoutMS) * time.Millisecond,
		Transport: baseTransport,
	}

	// xhttp/splithttp 禁用外层 H2：防止双层 H2 冲突导致 DATA after END_STREAM
	if network, ok := mapping["network"].(string); ok &&
		(network == "xhttp" || network == "splithttp") {
		baseTransport.ForceAttemptHTTP2 = false
	}

	return pc
}

// Close 关闭客户端，释放所有资源
func (pc *ProxyClient) Close() {
	// 防御性检查：防止对 nil 指针调用 Close
	if pc == nil {
		return
	}

	// 发送取消信号
	// 这会立即触发 CreateClient 中 context.AfterFunc 注册的回调，
	// 从而中断所有正在进行的 Dial 过程。
	if pc.cancel != nil {
		pc.cancel()
	}

	// 强制关闭所有仍未关闭的底层连接（sync.Map 允许在 Range 中删除）
	pc.conns.Range(func(k, _ any) bool {
		if cc, ok := k.(*countingConn); ok {
			_ = cc.Close()
		}
		return true
	})

	// 关闭mihomo代理实例
	if pc.mProxy != nil {
		if err := pc.mProxy.Close(); err != nil {
			slog.Error("关闭代理实例失败", "err", err)
		}
	}

	// 关闭 HTTP 连接池
	if pc.Client != nil {
		pc.CloseIdleConnections()
	}

	// 流量统计
	bytesRead := pc.BytesRead.Load()
	bytesWritten := pc.BytesWritten.Load()
	if bytesRead > 0 {
		DOWN.Add(bytesRead)
		TotalBytes.Add(bytesRead)
	}
	if bytesWritten > 0 {
		UP.Add(bytesWritten)
		TotalBytes.Add(bytesWritten)
	}
}

// countingConn 包裹 net.Conn，在网络连接层统计读/写字节数。
type countingConn struct {
	net.Conn
	readCounter  *atomic.Uint64
	writeCounter *atomic.Uint64
	networkLimit bool

	onClose   func() // 关闭时从 ProxyClient.conns 中注销
	closeOnce sync.Once
}

// Close 关闭连接并从所属客户端的连接表中注销（幂等）
func (c *countingConn) Close() error {
	c.closeOnce.Do(func() {
		if c.onClose != nil {
			c.onClose()
		}
	})
	return c.Conn.Close()
}

func (c *countingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		c.readCounter.Add(uint64(n))
		// 在连接层消耗 token
		if Bucket != nil && c.networkLimit {
			Bucket.Wait(int64(n))
		}
	}
	return n, err
}

func (c *countingConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 {
		c.writeCounter.Add(uint64(n))
	}
	return n, err
}

// 工具函数
func (pc *ProxyChecker) incrementAvailable() {
	pc.available.Add(1)
	Available.Add(1)
}

// checkCtxDone 提供一个非阻塞的检查，判断上下文是否已结束或是否收到强制关闭信号。
func checkCtxDone(c context.Context) bool {
	if ForceClose.Load() {
		return true
	}
	select {
	case <-c.Done():
		return true
	default:
		return false
	}
}

// isRetryable 判断错误是否值得重试（仅网络层瞬时故障）
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	// 明确不重试：context 被外部取消（超出全局限制等）
	if errors.Is(err, context.Canceled) {
		return false
	}
	// 重试：超时（含 mediaClient 自身的 Timeout）
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// 网络错误
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	if _, ok := errors.AsType[*net.OpError](err); ok {
		return true
	}

	if sysErr, ok := errors.AsType[*os.SyscallError](err); ok {
		switch {
		case errors.Is(sysErr.Err, syscall.ECONNRESET):
			return true

		case errors.Is(sysErr.Err, syscall.EPIPE):
			return true

		case errors.Is(sysErr.Err, syscall.ETIMEDOUT):
			return true
		}
	}

	return false
}

// withRetry 只在 isRetryable 时重试，否则直接返回
func withRetry(ctx context.Context, fn func() error) error {
	var err error
	for i := range MediaCheckMaxRetries {
		if i > 0 {
			// 指数退避，但不超过全局 timeout
			// 指数退避 + 随机抖动，避免大量任务同一时刻重试
			wait := time.Duration(i*i)*200*time.Millisecond + time.Duration(rand.Int64N(int64(300*time.Millisecond)))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		err = fn()
		if !isRetryable(err) {
			return err // 成功 or 明确失败，不重试
		}
	}
	return err
}

// checkJobLocation 获取节点归属地并进行地区过滤。
// 通过过滤后将归属地信息写入 job.Result，返回 false 表示节点未通过过滤。
func (job *ProxyJob) checkJobLocation(db *maxminddb.Reader, ctx context.Context) bool {
	filterLocs := config.GlobalConfig.NodeLoc
	if len(filterLocs) == 0 {
		return true
	}

	locTimeout := config.GlobalConfig.MediaCheckTimeout
	if locTimeout <= 0 {
		locTimeout = 10
	}
	locClient := &http.Client{
		Transport: job.Client.Transport,
		Timeout:   time.Duration(locTimeout) * time.Second,
	}

	// 位置过滤只关心国家/地区，不需要 ISP 类型标签
	country, ip, countryCodeTag, ispTag, err := proxyutils.GetProxyCountry(locClient, db, ctx, job.CfLoc, job.CfIP, false)
	if err != nil || country == "" {
		return false
	}

	if !containsLocation(filterLocs, country) {
		return false
	}

	job.Result.IP = ip
	job.Result.Country = country
	job.Result.ISPTag = ispTag
	job.Result.CountryCodeTag = countryCodeTag
	return true
}

func containsLocation(filterLocs []string, country string) bool {
	return lo.ContainsBy(filterLocs, func(loc string) bool {
		return strings.EqualFold(loc, country)
	})
}

// scoreAndRecycle 计算节点健康度分数。若分数 > 0，回收至 historyChan 以便后续统一排序保存。
func (pc *ProxyChecker) scoreAndRecycle(proxy map[string]any, state string, speed int, result *Result) {
	// 识别是否是历史节点或刚成功的节点
	// 历史来源仅限本机回环 URL（旧全局产物回灌），公网组 URL（/group/<name>/all.yaml）不算历史
	tag, _ := proxy["sub_tag"].(string)
	urlStr, _ := proxy["sub_url"].(string)
	isLocalHistoryURL := strings.Contains(urlStr, "127.0.0.1") ||
		strings.Contains(urlStr, "localhost") ||
		strings.Contains(urlStr, "::1")
	isHistory := tag == "#History" || tag == "#Succeed" ||
		(isLocalHistoryURL && (strings.Contains(urlStr, "/history.yaml") || strings.Contains(urlStr, "/all.yaml")))

	// 如果是全新抓取来的节点，且本次检测失败，直接无视不记录入历史
	if !isHistory && state != "success" {
		return
	}

	// 提取当前分数，首次打分赋予初始值 10
	score := 10
	if scoreRaw, exists := proxy["history_score"]; exists {
		switch v := scoreRaw.(type) {
		case int:
			score = v
		case float64:
			score = int(v) // yaml 反序列化后数值可能是 float64
		}
	}

	// 动态调整分值
	switch state {
	case "alive_fail":
		score -= 2 // 连通性彻底断开，重罚 (-2)
	case "speed_fail":
		score -= 1 // 速度不达标，轻罚 (-1)
	case "success":
		score += 1 // 存活奖励 (+1)
		// 速度奖励附加
		if speed > 50*1024*1024 { // > 50MB/s 额外 +2
			score += 2
		} else if speed > 20*1024*1024 { // > 20MB/s 额外 +1
			score += 1
		}
		// 流媒体与AI高质量奖励附加
		if result != nil && (result.Openai || result.OpenaiWeb || result.Netflix || result.Disney || result.Gemini.Region != "") {
			score += 1
		}
	case "untested":
		// 未测试(例如达到上限提前终止或被地区强行过滤)，不增不减，保留原分数
	}

	// 锁定最高分限制
	if score > 20 {
		score = 20
	}

	// 如果扣除后大于 0 就能留在历史库中
	if score > 0 {
		proxy["history_score"] = score
		pc.historyChan <- proxy
	}
}
