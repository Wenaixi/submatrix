// Package app 应用程序主入口，app\app.go
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/component/resolver"
	"github.com/robfig/cron/v3"
	"github.com/Wenaixi/submatrix/app/monitor"
	"github.com/Wenaixi/submatrix/check"
	"github.com/Wenaixi/submatrix/config"
	proxyutils "github.com/Wenaixi/submatrix/proxy"
	"github.com/Wenaixi/submatrix/save"
	"github.com/Wenaixi/submatrix/substore"
	"github.com/Wenaixi/submatrix/utils"
)

// App 结构体用于管理应用程序状态
type App struct {
	ctx        context.Context
	cancel     context.CancelFunc
	configPath string
	interval   int
	watcher    *fsnotify.Watcher
	// watcherCancel 用于停止轮询配置监听 goroutine。
	// 轮询无条件启动（容器内 inotify 可能收不到 bind mount 的事件），
	// 因此该字段始终非 nil。
	watcherCancel context.CancelFunc
	// lastConfigMod + configModMu 是配置重载的唯一去重依据：
	// inotify 事件与轮询 tick 共用它，只有 mtime 真正前进才触发 onConfigChange。
	lastConfigMod time.Time
	configModMu   sync.Mutex
	// reloading 保证同一时刻只有一轮 onConfigChange 在执行。
	// 与 configModMu 分离：锁只保护 mtime 比较，副作用（SubStore 重启等秒级操作）
	// 在锁外执行，避免堵死轮询 tick。
	reloading atomic.Bool
	// pendingReload 标记「检测期间收到过配置变更，需等本轮结束后应用」。
	// 避免检测中途读到半新半旧的全局配置。
	pendingReload atomic.Bool
	// debounceTimer 是 inotify 事件的防抖定时器（避免编辑器写临时文件产生两次事件）。
	// 提升为字段是为了让 Shutdown 能 Stop 它：否则关闭窗口内回调仍可能触发
	// onConfigChange → setTimer → close(app.done)，造成二次关闭 panic。
	debounceTimerMu sync.Mutex
	debounceTimer   *time.Timer
	checkChan       chan struct{} // 触发检测的通道
	checking      atomic.Bool   // 检测状态标志
	ticker        *time.Ticker
	done          chan struct{} // 用于结束ticker goroutine的信号
	cron          *cron.Cron    // crontab调度器（代理检测定时任务）
	// updateCron 版本更新定时任务调度器，独立存储以便 SetupUpdateTasks 重调时先停止旧实例，
	// 避免每次配置变更触发重建时 goroutine 持续累积。
	updateCron *cron.Cron
	updateMu   sync.Mutex

	// 用于独立管理各项更新任务的 ID
	idSelfUpdate cron.EntryID
	idGeoDB      cron.EntryID
	idSubStore   cron.EntryID

	version       string
	originVersion string
	latestVersion string
	httpServer    *http.Server
	stopCh        <-chan struct{}

	lastCheck lastCheckResult

	router *gin.Engine

	// currentTheme 存储用户选择的主题，"auto"/"dark"/"light"
	currentTheme string
	themeMu      sync.RWMutex
}

type lastCheckResult struct {
	time      atomic.Value // 存储 time.Time
	duration  atomic.Int64
	Total     atomic.Int64
	available atomic.Int64
	traffic   atomic.Int64
}

// New 创建新的应用实例
// 不再在这里调用 flag.Parse() 或定义 flags，全部由 main 负责。
func New(originVersion string, version string, configPath string) *App {
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		ctx:           ctx,
		cancel:        cancel,
		configPath:    configPath,
		checkChan:     make(chan struct{}),
		done:          make(chan struct{}),
		version:       version,
		originVersion: originVersion,
	}
}

// InitConfigLoad 初始化配置文件加载
func (app *App) InitConfigLoad() error {
	// 初始化配置文件路径
	if err := app.initConfigPath(); err != nil {
		return fmt.Errorf("初始化配置文件路径失败: %w", err)
	}

	// 迁移旧配置文件
	if err := app.migrateConfig(); err != nil {
		slog.Warn("Singbox 配置迁移失败，跳过迁移继续加载", "error", err)
	}

	// 加载配置文件
	if err := app.loadConfig(); err != nil {
		return fmt.Errorf("加载配置文件失败: %w", err)
	}
	return nil
}

// Initialize 初始化应用程序
func (app *App) Initialize() error {
	if err := app.InitConfigLoad(); err != nil {
		return err
	}

	// 初始化分享存储（全新按组分发子系统，不做旧版兼容）
	GetShareStore()
	// 初始化可信代理白名单（分享限速/统计的 XFF 信任依据）
	configureTrustedProxies(config.GlobalConfig.TrustedProxyIPs)

	httpPortAvailable, subStorePortAvailable := app.CheckPortConflict()

	if config.GlobalConfig.ListenPort != "" && !httpPortAvailable {
		listenAddr := normalizeListenAddr(config.GlobalConfig.ListenPort)
		if os.Getenv("START_FROM_GUI") == "1" {
			// GUI 模式：记录冲突，跳过启动，让窗口展示提示供用户修改端口
			slog.Warn("HTTP 端口已被其他进程占用，HTTP 服务未启动，请修改端口后重启",
				"addr", listenAddr)
		} else {
			// CLI 模式：端口冲突属于致命错误，直接返回
			return fmt.Errorf("HTTP 端口 %s 已被占用，请修改配置中的 listen-port", listenAddr)
		}
	}

	// 初始化配置文件监听
	if err := app.initConfigWatcher(); err != nil {
		return fmt.Errorf("初始化配置文件监听失败: %w", err)
	}

	app.interval = func() int {
		if config.GlobalConfig.CheckInterval <= 0 {
			return 2880
		}
		if config.GlobalConfig.CheckInterval <= 60 {
			return 60
		}
		return config.GlobalConfig.CheckInterval
	}()

	if config.GlobalConfig.ListenPort != "" {
		if err := app.initHTTPServer(); err != nil {
			return fmt.Errorf("初始化HTTP服务器失败: %w", err)
		}
	}

	if config.GlobalConfig.SubStorePort != "" {
		// Sub-Store 服务启动时,Singbox 版本号才有意义
		substore.InitSingboxVersion()

		subStoreAddr := normalizeListenAddr(config.GlobalConfig.SubStorePort)
		if !subStorePortAvailable {
			substore.IsSubStoreRunning.Store(false)
			slog.Warn("Sub-Store 端口已被其他进程占用，Sub-Store 服务未启动，请修改端口后重启",
				"addr", subStoreAddr)
		} else {
			// 使用 app.ctx 启动 sub-store，让其可被取消
			go substore.RunSubStoreService(app.ctx)
			// 短暂等待，保证 Sub-Store 启动日志按预期顺序输出
			time.Sleep(500 * time.Millisecond)
		}
	} else {
		slog.Warn("Sub-Store 服务已禁用", "port", "未设置")
		substore.IsSubStoreRunning.Store(false)
	}

	// 启动内存监控
	monitor.StartMemoryMonitor()

	// 注册退出前清理逻辑（兜底）
	utils.BeforeExitHook = func() {
		// 检查内置 Sub-Store 服务是否仍在运行
		if substore.IsSubStoreRunning.Load() {
			slog.Warn("强制退出前，尝试关闭 Sub-Store 内置服务及释放端口")
			if err := substore.StopSubStore(); err != nil {
				slog.Error("强制停止 Sub-Store 服务失败", "err", err)
			} else {
				slog.Info("Sub-Store 服务已成功停止，端口已释放")
			}
		}
	}

	// 注册 ShutdownHook（第二次 Ctrl+C 立即调用）
	utils.ShutdownHook = func() {
		slog.Warn("立即退出程序")
		err := app.Shutdown()
		if err != nil {
			slog.Error("关闭应用失败", "err", err)
		} else {
			os.Exit(0)
		}
	}

	// 设置信号处理器
	app.stopCh = utils.SetupSignalHandler(&check.ForceClose, &app.checking)

	// 添加主程序版本更新和maxmind数据，sub-store前后端资源更新
	app.SetupUpdateTasks()

	return nil
}

// Run 运行应用程序主循环
func (app *App) Run() {
	defer func() {
		if app.watcher != nil {
			_ = app.watcher.Close()
		}
		if app.ticker != nil {
			app.ticker.Stop()
		}
		if app.cron != nil {
			app.cron.Stop()
		}
		if app.updateCron != nil {
			app.updateCron.Stop()
		}
		if app.watcherCancel != nil {
			app.watcherCancel()
		}
	}()

	app.setTimer()

	// 分享统计定期落盘（每 30 秒；退出时 Shutdown 也会 Flush）
	shareFlushTicker := time.NewTicker(30 * time.Second)
	go func() {
		for range shareFlushTicker.C {
			GetShareStore().Flush()
		}
	}()
	defer shareFlushTicker.Stop()

	if config.GlobalConfig.CronExpression == "" {
		app.triggerCheck()
	}

	// 并发处理 checkChan
	go func() {
		for range app.checkChan {
			go app.triggerCheck()
		}
	}()

	// 阻塞等待 stopCh 被关闭
	<-app.stopCh
	err := app.Shutdown()
	if err != nil {
		slog.Error("关闭应用失败", "err", err)
	}
}

// GetRouter 供 Wails GUI 复用同一路由
func (app *App) GetRouter() *gin.Engine {
	return app.router
}

// GetConfigPath 返回 Initialize() 确定后的配置文件绝对路径。
// 在 Initialize() 调用前返回空字符串。
func (app *App) GetConfigPath() string {
	return app.configPath
}

// formatNextRunTime 统一格式化下一次运行时间
func (app *App) formatNextRunTime(next time.Time, loc *time.Location) string {
	if next.IsZero() {
		return "未计划"
	}
	_, offset := next.Zone()
	return fmt.Sprintf("%s UTC%+d",
		next.In(loc).Format("2006-01-02 15:04:05"),
		offset/3600,
	)
}

// setTimer 根据配置设置定时器
func (app *App) setTimer() {
	// 停止现有定时器
	if app.ticker != nil {
		// 应该先发送停止信号，防止被=nil后panic
		close(app.done)                // 发送停止信号
		app.done = make(chan struct{}) // 创建新通道
		app.ticker.Stop()
		app.ticker = nil
	}

	// 停止现有cron
	if app.cron != nil {
		app.cron.Stop()
		app.cron = nil
	}

	// 检查是否设置了cron表达式
	if config.GlobalConfig.CronExpression != "" {
		slog.Info("设置 cron 定时计划", "cron", config.GlobalConfig.CronExpression)
		app.cron = cron.New()
		_, err := app.cron.AddFunc(config.GlobalConfig.CronExpression, func() {
			app.triggerCheck()
		})
		if err != nil {
			app.cron.Stop()
			slog.Error("cron 表达式 '" + config.GlobalConfig.CronExpression + "' 解析失败: " + err.Error() + "，将使用检测间隔时间")
			// 使用间隔时间
			app.useIntervalTimer()
		} else {
			app.cron.Start()
			for _, entry := range app.cron.Entries() {
				if entry.Valid() && !entry.Next.IsZero() {
					slog.Warn("激活 cron 检测任务", "next", app.formatNextRunTime(entry.Next, app.cron.Location()))
				}
			}
		}
	} else {
		// 使用间隔时间
		app.useIntervalTimer()
	}
}

// useIntervalTimer 使用间隔时间模式运行
func (app *App) useIntervalTimer() {
	// 初始化定时器
	app.ticker = time.NewTicker(time.Duration(app.interval) * time.Minute)
	// 捕获局部变量：setTimer 会在另一 goroutine 把 app.ticker 置 nil，
	// 循环里直接读字段会撞上 nil 解引用 panic。
	ticker := app.ticker
	done := app.done
	// 启动一个goroutine监听定时器事件
	go func() {
		for {
			select {
			case <-ticker.C:
				app.triggerCheck()
			case <-done:
				return // 收到停止信号，退出goroutine
			}
		}
	}()
}

// TriggerCheck 供外部调用的触发检测方法
func (app *App) TriggerCheck() {
	select {
	case app.checkChan <- struct{}{}:
		slog.Info("手动触发检测")
	default:
		slog.Warn("已有检测正在进行，忽略本次触发")
	}
}

// triggerCheck 内部检测方法
func (app *App) triggerCheck() {
	// 如果已经在检测中，直接返回
	if !app.checking.CompareAndSwap(false, true) {
		slog.Warn("已有检测正在进行，跳过本次检测")
		return
	}
	defer app.checking.Store(false)

	if err := app.checkProxies(); err != nil {
		check.CurrentStepName.Store("检测失败")
		slog.Error("检测代理失败", "error", err)
	}

	// 检测完成后显示下次检测时间
	if app.ticker != nil {
		// 使用间隔时间模式
		app.ticker.Reset(time.Duration(app.interval) * time.Minute)
		nextCheck := time.Now().Add(time.Duration(app.interval) * time.Minute)
		slog.Info("下次检测时间", "time", app.formatNextRunTime(nextCheck, time.Local))
	} else if app.cron != nil {
		// 使用cron模式
		entries := app.cron.Entries()
		if len(entries) > 0 {
			slog.Info("下次检测时间", "time", app.formatNextRunTime(entries[0].Next, app.cron.Location()))
		}
	}
	check.CurrentStepName.Store("检测完成")

	// 释放内存
	// 注意：不要在这里提前 checking.Store(false)。releaseMemory 内部最长要等 30 秒
	// 让残留 goroutine 退出，该窗口内若放行下一次检测，两次检测会并发操作
	// resolver.DefaultResolver（cleanupMihomo 会把它置 nil），产生数据竞争。
	// 由函数开头的 defer app.checking.Store(false) 统一负责解锁。
	releaseMemory()

	// 本轮检测已彻底结束（内存也回收完），此时才应用检测期间累计的配置变更，
	// 保证下一轮检测跑在完整的新配置上。
	app.applyPendingReload()
}

// checkProxies 执行代理检测
func (app *App) checkProxies() error {
	if config.GlobalConfig.PrintProgress {
		slog.Info("启动检测任务", "终端进度条", "显示")
	} else {
		slog.Info("启动检测任务", "终端进度条", "隐藏")
	}

	startTime := time.Now()

	loadHistoricalCheckRate() // 注入历史速率后再开始检测

	// 每轮完整检测开始时清空全局节点集合，避免上一轮的节点残留在本轮报告里
	check.ResetGlobalAliveNodes()
	check.ResetGlobalSubAnalysis()

	check.StartProgress()
	defer check.StopProgress()

	// 组循环：按 priority 小→大串行检测各组，组间失败互不影响
	groups := config.ResolveAllGroups(*config.GlobalConfig)
	slog.Info("开始按组检测", "组数", len(groups))

	var (
		totalResults  int
		totalTx       int64
		totalPulled   int
		activeGroups  int
	)

	for _, grp := range groups {
		if !grp.Enabled {
			slog.Info("跳过禁用组", "组", grp.Name)
			continue
		}
		if check.ForceClose.Load() {
			slog.Warn("检测被手动终止，跳过剩余组", "组", grp.Name)
			break
		}

		slog.Info("开始检测组", "组", grp.Name, "优先级", grp.Priority)
		check.CurrentStepName.Store("检测组: " + grp.Name)

		results, historyNodes, err := check.CheckGroup(grp)
		if err != nil {
			slog.Error("组检测失败", "组", grp.Name, "err", err)
			continue
		}

		activeGroups++
		totalResults += len(results)
		// CheckGroup 内部会把 ProxyCount 重置为「本组节点数」，
		// 必须在每组结束后立刻取走再累加，否则多组会把前组的量覆盖掉。
		totalPulled += int(check.ProxyCount.Load())
		totalTx += int64(check.TotalBytes.Load())

		// 把本组可用节点并入全局集合（按物理指纹去重），供最终合并报告使用。
		// 传组名是为了让报告能按组给出各自的地区分布（同一个节点可能被多组引用）。
		check.CollectGroupAliveNodes(results, grp.Name)

		// 组落盘（all.yaml + mihomo.yaml + history.yaml）
		save.SaveGroup(grp, results, historyNodes)
		slog.Info("组检测完成", "组", grp.Name, "可用节点", len(results))

		// 让组产物即时对外可用（网关 /group/<name>/* ）
		utils.UpdateSubs()
	}

	// 所有组处理完，汇总计数
	app.lastCheck.time.Store(time.Now())
	app.lastCheck.duration.Store(int64(time.Since(startTime).Seconds()))
	// Total 是「拉取到的节点总数」，available 是「测活通过的节点数」。
	// 两者赋同一值会让前端可用率恒为 100%，监控失去意义。
	app.lastCheck.Total.Store(int64(totalPulled))
	app.lastCheck.available.Store(int64(totalResults))
	app.lastCheck.traffic.Store(totalTx)

	// 用「所有组合并去重」的可用节点重写报告。
	// 单组报告每组覆盖一次，最终只剩最后一组的地区分布；这里再写一次全局版，
	// 让报告里的地区/协议/解锁分布反映全部组的全貌。
	//
	// totalPulled 是本轮所有组「拉取节点数」之和，作为全局报告的分母：
	// 若不传，报告会用包级 Progress（最后一组的值），导致前端
	// 「存活率 = 全局可用数 / 单组测活数」出现 988% 这类不可能的数字。
	check.WriteGlobalAnalysisReport(totalPulled)

	check.CurrentStepName.Store("发送通知")
	utils.SendNotifyCheckResult(totalResults, check.CheckTrafficTotal)

	if config.GlobalConfig.CallbackScript != "" {
		check.CurrentStepName.Store("执行回调脚本")
	}
	utils.ExecuteCallback(totalResults)

	check.CurrentStepName.Store("清理检测缓存")
	proxyutils.ClearCache()

	// 每次运行清理旧缓存，防止多轮执行后失效订阅残留或混淆
	proxyutils.SubStatsMutex.Lock()
	proxyutils.SubStats = make(map[string]proxyutils.SubStat)
	proxyutils.SubStatsMutex.Unlock()

	slog.Info("全部组检测完成", "组数", activeGroups, "总可用节点", totalResults)
	return nil
}

// releaseMemory 释放mihomo检测内存
func releaseMemory() {
	cleanupMihomo()

	// 等待底层 xhttp/tcp goroutine 退出释放缓冲区
	waitGoroutinesDrain(30 * time.Second)

	// 连续触发两次 GC，彻底清空 go-yaml 等库的 sync.Pool
	// 第一次 GC：将 sync.Pool 中的存活对象降级到 victim cache
	runtime.GC()
	// 第二次 GC：清空 victim cache，并强制将物理内存还给操作系统
	debug.FreeOSMemory()

	slog.Debug("当前 goroutine 数", "count", runtime.NumGoroutine())

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	slog.Debug("内存状态",
		"HeapInuse", ms.HeapInuse/1024/1024,
		"HeapReleased", ms.HeapReleased/1024/1024,
		"Sys", ms.Sys/1024/1024)
}

// waitGoroutinesDrain 等待残留 goroutine 退出
func waitGoroutinesDrain(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	baseline := runtime.NumGoroutine()

	// 预期正常闲置时的 goroutine 数量大约在 30-50 之间
	for time.Now().Before(deadline) {
		current := runtime.NumGoroutine()
		if current <= baseline/2 || current < 50 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func cleanupMihomo() {
	// 清除 Hosts 映射
	if resolver.DefaultHostMapper != nil {
		resolver.DefaultHostMapper = nil
	}

	// 动清理 DNS 解析器的内部缓存和长连接，并断开引用
	if resolver.DefaultResolver != nil {
		// 释放内部的 LRU/Map 缓存，断开大量的内存引用
		resolver.DefaultResolver.ClearCache()
		resolver.DefaultResolver.ResetConnection()
		resolver.DefaultResolver = nil
	}

	resolver.ClearCache()
}

// Shutdown 尝试优雅关闭所有子服务与资源
func (app *App) Shutdown() error {
	slog.Debug("开始关闭应用...")

	// 退出前刷分享统计落盘（防丢统计）
	GetShareStore().Flush()

	var lastErr error

	// 停止轮询配置监听 goroutine（inotify 降级模式专用）
	if app.watcherCancel != nil {
		app.watcherCancel()
	}

	// 停掉尚未触发的 inotify 防抖回调：若不拦，它可能在关闭窗口内
	// 触发 onConfigChange → setTimer → close(app.done)，造成二次关闭 panic。
	app.debounceTimerMu.Lock()
	if app.debounceTimer != nil {
		app.debounceTimer.Stop()
		app.debounceTimer = nil
	}
	app.debounceTimerMu.Unlock()

	// 取消上下文，通知各子服务退出（sub-store 等）
	if app.cancel != nil {
		app.cancel()
	}

	// 停止 ticker/cron/updateCron/watcher（如果存在）
	if app.ticker != nil {
		app.ticker.Stop()
	}
	if app.cron != nil {
		app.cron.Stop()
	}
	if app.updateCron != nil {
		app.updateCron.Stop()
	}
	if app.watcher != nil {
		lastErr = app.watcher.Close()
	}

	// 优雅关闭 HTTP 服务
	if app.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.httpServer.Shutdown(ctx); err != nil {
			lastErr = errors.New("关闭 HTTP 服务器失败: " + err.Error())
			slog.Error("关闭 HTTP 服务器失败", "err", err)
		} else {
			listenPort := strings.TrimPrefix(config.GlobalConfig.ListenPort, ":")
			slog.Info("HTTP 服务器关闭", "port", listenPort)
		}
	}

	// 关闭 done 通道以通知定时 goroutine 退出（如果仍在）
	select {
	case <-app.done:
		// already closed or receiving
	default:
		// 保护性关闭 done，避免 panic
		close(app.done)
	}

	// 等待短时间，给子 goroutine 清理时间（作为最小可行方案）
	time.Sleep(500 * time.Millisecond)

	slog.Info("应用已关闭")
	return lastErr
}
