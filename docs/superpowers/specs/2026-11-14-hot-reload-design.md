# SubMatrix 全量热重载设计

日期：2026-11-14
状态：已确认

## 问题

配置热重载在 Docker Desktop 容器内**静默失效**：

```
inotify 初始化成功 → 只启用 inotify，不启动轮询
        ↓
Docker Desktop 的 bind mount 不把宿主机文件事件传进容器内核
        ↓
事件永不到达 → 永远不重载
        ↓
轮询兜底因「初始化成功」而不启用 → 无任何日志、无任何提示
```

实测证据：把 `config.yaml` 的 `concurrent: 30` 改成 `31`，等待 2 分钟，
容器日志无任何「配置文件发生变化」记录。

**设计缺陷的本质**：把「inotify 初始化成功」等同于「inotify 会收到事件」。
两者在容器 / 网络文件系统 / bind mount 场景下并不成立。

## 目标

| 目标 | 判据 |
|---|---|
| 容器内配置热重载可用 | 改 config.yaml 后 3 秒内自动重载 |
| 本机仍保持秒级响应 | inotify 生效时 100ms 防抖后重载 |
| 规则模板改动下次检测即生效 | 已天然支持，无需改动 |
| 不重复重载 | 连续快速写文件只触发一次 |

## 非目标

- 不改 WebUI（不加手动重载按钮）
- 不做「模板改动立即重新生成所有组产物」
- 不改组解析逻辑（`ResolveAllGroups` 已在每次检测时执行）

## 设计

### 核心改动：轮询无条件启动

`app/config.go` 的 `initConfigWatcher` 改为：

```go
func (app *App) initConfigWatcher() error {
    // 1. 轮询无条件启动，不再依赖 inotify 失败
    ctx, cancel := context.WithCancel(context.Background())
    app.watcherCancel = cancel
    go app.pollConfigFile(ctx)

    // 2. 尝试 inotify，失败只记日志
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        slog.Warn("inotify 不可用，仅使用轮询监听", "error", err)
        return nil
    }
    ...（成功时正常注册监听）
}
```

### 统一去重入口

两个机制共用一个 mtime 事实来源：

```go
type App struct {
    ...
    lastConfigMod time.Time
    configModMu   sync.Mutex
}

// tryReloadByModTime 是唯一的重载决策点：
// 只有 mtime 真正前进时才重载，避免 inotify 与轮询重复触发。
func (app *App) tryReloadByModTime() {
    app.configModMu.Lock()
    defer app.configModMu.Unlock()

    info, err := os.Stat(app.configPath)
    if err != nil {
        return
    }
    if !info.ModTime().After(app.lastConfigMod) {
        return
    }
    app.lastConfigMod = info.ModTime()
    slog.Info("配置文件发生变化，正在重新加载")
    app.onConfigChange()
}
```

- inotify 事件 → 调 `tryReloadByModTime()`
- 轮询 tick → 调 `tryReloadByModTime()`
- 两者并发安全（互斥锁保护）

### 轮询间隔

从 5 秒改为 **3 秒**。理由：容器内这是唯一的响应路径，3 秒对用户体验可接受，
且 stat 一次的 CPU 开销可忽略。

### 规则模板（无需改动）

`save/save.go` 的 `fetchLocalOrAssetTemplate` 每次调用都重新 `os.ReadFile`，
无任何缓存。所以 `output/` 或 `assets/` 里的模板改动，下次检测自动生效。

## 数据流

```
宿主机改 config.yaml
    ├──→ [本机] inotify 事件 → 100ms 防抖 → tryReloadByModTime()
    └──→ [容器] 轮询每 3s → tryReloadByModTime()
                              ↓
                      mtime 前进？→ onConfigChange()
                              ↓
                      loadConfig() 重新读文件 → *config.GlobalConfig = newConfig
                              ↓
                      下次检测时 ResolveAllGroups() 用新配置
```

## 错误处理

| 场景 | 行为 |
|---|---|
| inotify 初始化失败 | Warn 日志，轮询继续工作 |
| inotify 注册目录失败 | Warn 日志，关闭 watcher，轮询继续工作 |
| 配置文件被删除 | `os.Stat` 失败，跳过本轮，下次 tick 重试 |
| 配置文件解析失败 | `onConfigChange` 里 `loadConfig` 报错并 return，旧配置保留 |
| mtime 未前进 | 静默跳过（去重） |

## 测试

| 测试 | 方式 |
|---|---|
| 容器内 3 秒热重载 | 改 concurrent，观察日志 |
| 本机 inotify 秒级 | 本机跑二进制，改配置观察 |
| 去重生效 | 连续快速改两次，确认只重载一次 |
| 模板热重载 | 改 output/ACL4SSR，跑检测看产物 |
| 编译 | `CGO_ENABLED=0 go vet ./app/` |

## 影响范围

- `app/config.go`：`initConfigWatcher`、`pollConfigFile`、新增 `tryReloadByModTime`
- `app/app.go`：`App` 结构体新增两个字段
- 不改其他文件

## 风险

| 风险 | 缓解 |
|---|---|
| 轮询增加 CPU 开销 | 3 秒一次 stat，可忽略 |
| 与 inotify 重复触发 | mtime 去重，互斥锁保护 |
| mtime 精度问题（同一秒内两次修改） | 概率极低；如发生，第二次修改的内容会在下次检测时被读到（因为 loadConfig 总是重读文件） |
