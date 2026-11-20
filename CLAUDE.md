# SubMatrix 核心工程架构与记忆库

## 项目定位
- 仓库命名空间：`github.com/Wenaixi/submatrix`（全仓已彻底去外部化，归属主人专属命名空间）
- 核心引擎：高性能订阅拓扑矩阵与检测筛选工具（基于 Go 1.24+ 与 React 18 SPA 架构）
- 部署镜像：`wenxiloveyou/submatrix:latest`（本地 Docker 构建并同步发布至 GitHub Packages / Docker Hub）

## 关键技术架构与设计规范

### 1. 全自动规则装配与域名自适应 (Auto-Configured Rules & Adaptive Host)
- **零硬编码安全原则**：系统代码与前端界面严禁硬编码任何私人/私有域名。
- **纯动态自适应**：
  - 前端：采用 `location.origin` 动态自适应当前访问源，支持在【订阅链接】界面填入可选的自定义外部域名并持久化于 `localStorage`；
  - 后端网关：在 `app/gateway.go` 与 `app/server.go` 中通过 `GetCanonicalBaseURL(c)` 自动捕获 `X-Forwarded-Proto`、`X-Forwarded-Host` 与 `c.Request.Host`；
  - 自动重写：当客户端拉取 `mihomo.yaml` 时，服务端自动将 2298 行完整规则模板中的 `ProviderALL` 直链动态重写为当前可访问的对应组 `all.yaml` 地址，确保客户端瞬间秒拉 Provider 绝不报 Host 错误。

### 2. 模块结构与命名空间
- 根模块声明：`module github.com/Wenaixi/submatrix`
- 内部子包统一路径：
  - `github.com/Wenaixi/submatrix/config`
  - `github.com/Wenaixi/submatrix/check`
  - `github.com/Wenaixi/submatrix/save`
  - `github.com/Wenaixi/submatrix/app`
  - `github.com/Wenaixi/submatrix/webui`
  - `github.com/Wenaixi/submatrix/utils`

### 3. Docker 构建与部署
- 编译镜像自带 `ENV GOPROXY=https://goproxy.cn,direct`，保障国内网络环境下秒级无损下载 Go 依赖；
- 容器命名：`submatrix-test`，端口：`8199:8199`，持久化挂载宿主机 `output/` 与 `config/` 目录。

## 4. 按组切分与无头 CLI（2026-11-07）

### 组功能现状与决策
- 早期提交 0343058 曾实现「按组独立流水线」（ExecuteGroupProxies + SaveGroupConfig 调用链），
  同日的 f2109d6 以 sinspired 为基座重构时整段删除，`SaveGroupConfig` 成为零调用孤儿函数。
- 已恢复的形态：**全局检测一次 + 按组切分落盘**。切分数据源必须是 `historyNodes`
  （保留完整 sub_url/sub_tag），`results` 在 `check.CleanupMetadata()` 已被剥离归属字段。
- `SaveGroupConfig` 依赖 check.Result；`SplitResultsByGroupFromPool` 直接吃 map 池，
  两者复用 `writeGroupYAML` 落盘（groups/<name>/all.yaml + 按 rule-preset 融合 mihomo.yaml）。

### 命名修复（核心三处）
- `updateProxyName`（check/check.go）else 分支不再无条件拼接原始名，
  未知地区统一 `Rename("","")` → 占位旗 + 序号，杜绝 🏴☠_1D🔴... 滚雪球。
- `config/group.go` 新增 `MatchesNode`：按 sub_tag/sub_url 判定节点归属组。
- 诊断结论：`append-original-name` / `tag-ip-type` / `node-prefix` 在代码里无消费点（上游行为）；
  `enhanced-tag` 默认开，但当前 config.yaml 缺该键（示例配置有，值为 true）。

> 注：第 4 节顶部「按组切分」的形态已被第 5 节（按组独立检测）取代，
> 保留在此仅为历史演进记录。当前实现以第 5 节为准。

### CLI subctl（cmd/subctl/）
- 命令：`validate` / `get <路径>` / `set <路径> <值>` / `groups list|toggle` / `presets`。
- 路径语法：点号 + 切片下标，如 `subctl get groups.0.name`；
  `set` 支持 bool/int/string/逗号分隔 []string/YAML 片段。
- 依赖：新增 `github.com/urfave/cli/v2`（v2.27.5）。
- 用途：容器/服务器/无浏览器环境下精确读写配置，替代 WebUI 操作。

### Docker 构建坑（2026-11-07 实测，7 次失败总结）
- `daemon.json` 的 `proxies` 段通过 **build-arg**（非 ENV）注入构建容器，
  优先级高于 Dockerfile 的 ENV 清空；容器内 127.0.0.1 是容器自己，代理必连不上。
  解法：`docker build --build-arg HTTP_PROXY= --build-arg HTTPS_PROXY= --build-arg http_proxy= --build-arg https_proxy= --build-arg ALL_PROXY= --build-arg all_proxy= `
- `go.mod` 要求 go 1.27，本地缓存基础镜像只有 golang:1.24 → 必须 `ENV GOTOOLCHAIN=auto`
  （且改 Dockerfile 全程留意别误删此行）。
- apt 换源：`sed -i s|deb.debian.org|mirrors.aliyun.com|g /etc/apt/sources.list.d/debian.sources`，
  musl-dev 在 bookworm 无候选，不要装。
- 容器 API 健康检查必须 `curl --noproxy "*"`：本机系统代理（Clash 10801）会把
  回环请求也代理掉，产生假 502。
- 新镜像验证三件套：容器日志「组订阅已落盘」行 + `output/groups/*/all.yaml` 时间戳 + 节点名抽查。

## 5. 按组独立检测 + priority 串行（2026-11-07 完成）

### 架构
- `app.checkProxies` 组循环：ResolveAllGroups → priority 排序 → 逐个 `check.CheckGroup` → `save.SaveGroup`
- `check.CheckGroup(grp)` 用 `proxy.FetchGroupProxies` 拉组 URLs（不再走全局 GetProxies）
- `save.SaveGroup(grp, results, historyPool)` 落盘 `groups/<name>/{all,mihomo,history}.yaml`
- 相对路径 URL（`/group/x/all.yaml`）→ `expandGroupRelativeURL` 拼接 `public-url + token`；**public-url 为空则组报错跳过**（防止回环引用）
- 组内跨 URL 凭证指纹去重（GenerateProxyKey）；同 IP 不同账号保留

### 已删除
- 全局产物体系（output/all.yaml、sub/*、base64.txt 保存 + 路由 + GetProxies 的 last/history 回灌）
- `check.Check()`（被 CheckGroup 取代）、`SplitResultsByGroup*`、`ConfigSaver/SaveConfig/getSaverFunc`
- `use-last-result/use-history-result` 失去功能（字段保留标记 deprecated）

### 新能力
- WebUI 组卡片拖拽排序（原生 HTML5 DnD，drop 重写 priority）
- 组名白名单校验 `[A-Za-z0-9_-]`（saveGroupsHandler，防路径穿越）
- subctl `groups list` 显示优先级，`set groups.N.priority N` 可改

### 配置要求
- `public-url` 必须配置公网域名，聚合组（相对路径 URL）才可用
- `min-speed` 阈值直接影响可用节点数（free-a 当前 4 个节点，因 min-speed=1 太严）
- 当前实测：`public-url = https://sub-s.wenxi.dev`（Cloudflare Tunnel 公网入口）

## 6. 过滤双轨制 + 126 模板 + 设置界面（2026-11-07 完成）

### 过滤双轨制（不检测但保留）
- `FetchGroupProxies`（proxy/group_fetch.go）不再丢弃被过滤节点：
  `MatchesProtocol` 命中的节点打 `skipped: true` 标记保留
- `distributeJobs`（check/check.go）遇 `skipped` 直接放行（不测活不测速），与免检直通同路径
- `SaveGroup`（save/save.go）落盘双轨：
  - `all.yaml`：已检测节点（客户端直用）
  - `all-raw.yaml`：全量原始（含 skipped，取消过滤即恢复）
  - `mihomo.yaml`：融合规则（只含已检测）
  - `history.yaml`：历史评分池
- `filter-protocols` 置空 = 全部节点检测 + 落盘

### 126 组最详模板
- `assets/Mihomo-Rules-126.yaml`：由旧 output/mihomo.yaml 静态化，5 处 `{{ProviderURL}}` 占位符
- `pro_full` 默认指向它；`groupProviderURL`（save/save.go）落盘时替换为组产物公网直链
- 现有 25 个 rule-providers：自家 5 个 Provider（组产物）+ blackmatrix7 18 个 + AdRules 去广告 + Privacy 隐私
- rules 首两条 `RULE-SET,AdRules,REJECT` / `RULE-SET,Privacy,REJECT`
- 模板备份：/tmp/Mihomo-Rules-126.yaml.bak

### 设置界面（WebUI）
- ConfigView 重构：常用设置表单（4 区 16 项）+ 进阶 YAML 折叠编辑器
- 表单项读写 YAML 文本（行级精确替换），单一数据源；`min-speed` 标注 KB/s 单位
- 导航 Tab「系统设置」独立

### 命名格式
- 当前 `🇺🇸US_1|30KB/s|免费A`（no enhanced-tag，config 缺 `enhanced-tag: true`）
- 若要恢复 CF 角标（HK² 等）：config.yaml 加 `enhanced-tag: true` 即可

## 7. 前端产物陷阱与登录凭证断链（2026-11-14 修复）

### 前端产物必须重建才生效（最易踩）
- `webui/embed.go` 用 `//go:embed static/*` 把 `webui/static/dist/` 打进 Go 二进制，
  而 `Dockerfile` 只跑 `go build`，**不构建前端**。
- 后果：只改 `frontend/src/` 而不 `cd frontend && npm run build`，容器里跑的永远是旧产物。
  本次两个 bug（登录二次输入、订阅页四条全局直链）都源于 11-08 的旧产物与源码脱节。
- 必做顺序：改源码 → `npm run build`（产物落到 `webui/static/dist/`）→ 重建 Go / Docker 镜像。
- 本机无 zig、无 gcc，无法 `go build`（CGO 依赖 quickjs）；验证只能走 Docker 构建。

### 登录凭证两套命名（曾导致登录后二次弹窗）
- 登录页 `login.js` 写 `scp_api_key`（sessionStorage + localStorage + cookie）；
  React `client.ts` 原只读 `submatrix_api_key` / `apiKey`，两套永不交汇。
- 修法：`getApiKey()` 读取链补上 `scp_api_key`（localStorage 与 sessionStorage），
  `setApiKey()` 归一化回写三个键。
- 附带断链：`apiRequest` 曾无条件下发空 `X-API-Key`，使服务端 `authMiddleware`
  判定「header 已提供」而跳过 cookie 兜底分支。现改为 token 非空才下发 header 与 `?token=`。

### 已删除的全局产物路由（勿再在 UI 暴露）
- `/mihomo.yaml`、`/base64.txt`、`/all.yaml`、`/history.yaml` 四条均为旧全局架构遗留，
  实测全部 404。按组架构只提供 `/group/<组名>/{all,all-raw,mihomo,history}.yaml`。
- `save` 包内无任何 base64 生成代码，`/group/<组名>/base64.txt` 同样恒 404，
  `GroupsView` 的「复制通用订阅 (Base64)」按钮已移除。

### 死代码备忘（未清理，改动时勿当真）
- `App.tsx` 的 `if (gRes.apiKey) setKey(...)`：后端 `getGroupsHandler` 从不返回该字段。
- `webui/static/js/admin.js` 整文件无模板引用（`templates/*.html` 只加载 `dist/app.js` 与 `login.js`）。

### .gitignore 的 dist/ 规则曾误伤内嵌产物（2026-11-14 修正）
- 规则 `dist/` 会匹配**任意层级**的 `dist` 目录，包括 `webui/static/dist/`；
  原先只靠第 71 行注释声明「刻意不忽略」，注释不是规则，不生效。
- 已补 `!webui/static/dist/` 否定规则。验证方式：`git check-ignore -v <路径>`，
  对象为 `webui/static/dist/app.js` 时应无输出，而 `frontend/dist/test.js` 仍被忽略。
- 已跟踪文件不受忽略规则影响，但合并冲突后文件状态会变复杂，`git add` 可能被挡，
  需要 `git add -f`。

### 容器重建必查代理变量（2026-11-14 再次踩坑）
- Docker Desktop 的 `daemon.json` 会把 `proxies` 段注入**每一个新容器**，
  导致容器内出现 `HTTP_PROXY=http://127.0.0.1:10801`。
- 容器内的 127.0.0.1 是容器自己，该代理端口不存在，**检测流量全部 connection refused**。
- 本次实测后果：15186 个节点在 3 分 38 秒内全军覆没，可用节点 0。
- 重建容器命令必须带齐 `-e HTTP_PROXY= -e HTTPS_PROXY= -e http_proxy= -e https_proxy= -e ALL_PROXY= -e all_proxy=`，
  之后用 `docker inspect <容器> --format '{{range .Config.Env}}{{println .}}{{end}}' | Select-String PROXY` 复核。

## 8. 分支模型（2026-11-14 生效）

- **`master`**：稳定可发布分支，CI（`.github/workflows/docker.yml`）只在此分支推送时自动构建镜像。
- **`dev`**：日常开发分支，功能完成后合入 master。
- 已删除的 `main` 分支：曾有 77 个提交与 master 的 41 个旧提交分叉，11-04 后 master 停止更新，
  全部工作都在 main 上。11-14 将 main 内容接管为 master 并强推，旧 master 的提交已丢弃
  （其中 `check/group_check_test.go` 引用的三个函数在 main 上已不存在，属过时死测试）。
- 本地 `master` 的 upstream 需手动指向 `origin/master`（重命名后会残留 `origin/main` 的追踪，用
  `git branch --unset-upstream` 后重新 `-u`）。

## 9. CRLF 配置读取陷阱（2026-11-14 修复）

- 配置文件 `config/config.yaml` 在 Windows 下是 **CRLF 行尾**（`config.yaml.example` 经 go:embed 生成时同样）。
- 前端 `ConfigView` 的 `readYamlValue` 原用 `(.*)$` 匹配：JS 正则的 `.` 不匹配 `\r`，
  `$` 无法在行尾成立，**整行匹配失败并返回空串**。
- 后果不是单项失效，而是**设置页所有输入框都为空、所有开关都显示「已关闭」**，
  而 config.yaml 里的真实值一直在生效 —— 表现为「WebUI 显示已关闭却仍能访问」。
- 修复：正则改 `(.*?)\s*\r?$`；`writeYamlValue` 改写行时保留原 `\r`，避免 CRLF 文件局部变 LF。
- 回归验证：用真实配置内容跑 10 项断言，全部应读到正确值。
- 相关事实：`enable-web-ui` 默认值本就是 `true`（`config.yaml.example:204`），
  配置热重载由 `app/config.go` 的 fsnotify 实现，二者无需改动。

## 10. 拉取代理与检测代理分离 + 统一 UA（2026-11-14）

### 两个代理字段语义不同，勿混用
- `proxy`（yaml `proxy`）：**检测代理**，只作用于该组节点的测活与测速
  （消费点 `check/check.go` 的 `pc.grp.Proxy` → `mapping["group_proxy"]`）。
- `fetch-proxy`（yaml `fetch-proxy`）：**拉取代理**，只作用于从订阅源下载节点列表
  （消费点 `proxy/group_fetch.go` 的 `FetchSubsDataWithProxy(fetchURL, grp.FetchProxy)`）。
- 未配 `fetch-proxy` 时回退全局 `system-proxy`；配了则**只用它**，
  不再混入 ghproxy 改写与直连策略（避免「配了代理却仍从直连拉」的误判）。

### 订阅站的 UA 白名单陷阱
- 实测某订阅站（sub-yes.small1999.sbs）：`clash-verge/v1.6.0` 返回 40 个真实节点，
  `Clash/v1.18.0` 返回 `proxies: []` 空壳，`Mihomo` 返回无 proxies 段，curl 默认 UA 直接 403。
- 空壳响应**不会报错**，程序会以为拉取成功但拿到 0 节点——比 403 更隐蔽。
- 现已将 UA 池统一为 `clash-verge/v1.6.0`（`proxy/fetch.go` 的 `uaList`），
  GitHub 域名仍走浏览器 UA 分支。
- `fetchOnce` 的 UA 兜底由 `len(ua) <= 1` 改为空值判断，避免误替换短 UA。

### 容器内访问宿主机代理
- 必须写 `http://host.docker.internal:10801`，**不能用 `127.0.0.1:10801`**（那是容器自己）。
- 已验证：容器内经该地址拉取同一订阅成功返回 40114 字节。
- **`--add-host` 里写死的宿主机 IP 会随网络切换而失效（2026-11-15 实测踩坑）**：
  此前记录的是 `192.168.2.13`，换网络后宿主 WLAN IP 变成 `192.168.1.141`，
  容器内所有组级 `fetch_proxy` 全部 `i/o timeout`，表现为「4 个组拉取全失败、
  只有走公网域名的组正常」。重建容器前先用
  `powershell -NoProfile -Command "Get-NetIPAddress -AddressFamily IPv4 | Where-Object {$_.IPAddress -like '192.168.*'} | Select -ExpandProperty IPAddress"`
  确认当前 IP，再写 `--add-host "host.docker.internal:<当前IP>"`。
- 容器内测端口用 `nc -z host.docker.internal 10801`；**容器没有 curl，也没有 bash 的 `/dev/tcp`**，
  用 `sh -c 'echo > /dev/tcp/...'` 会得到 `nonexistent directory` 的假阴性。

### 无自定义 DNS 时不要重建 mihomo 解析器（2026-11-15 实测，重要）
- `SetupMihomoDNS` 在配置里没有 `nameserver` 时，曾经加过「用 `dns.NewResolver(cfg)`
  重建兜底解析器」的分支。该分支**实测把检测打到全线 0 可用**：
  hneko 与 mtf 两组 0 可用，且日志显示「开始检测节点」到「可用节点 0」
  发生在同一秒内——检测根本没有发起连接就全判失败。
- 根因：mihomo 的 `dns` 包在 `init()` 里构造了一个带 `systemClient` 的
  `resolver.SystemResolver`（见 `mihomo/dns/system.go`），那才是系统 DNS 的正确实现。
  用 `dns.NewResolver()` 覆盖它会替换掉这个实现，在受限网络下（容器只放通特定 DNS）
  导致所有解析秒级失败。
- 正确认知：`cleanupMihomo` 只把 `DefaultResolver` 置 nil，`SystemResolver`
  由 mihomo 的 `init()` 保证非 nil；`resolver` 各 `Lookup*`/`Resolve*` 入口在
  `DefaultResolver` 为 nil 时会自动回退到 `SystemResolver`。
  因此无自定义 DNS 时**不需要任何重建动作**，直接 return 即可。
- 识别特征：某组「节点全部 0 可用」+「检测耗时不到 1 秒」= DNS 解析全线失败，
  优先查解析器状态，不要先去查节点本身。

### 容器 DNS 必须显式指定，否则 ECH 节点全部失败（2026-11-14）
- Docker Desktop 默认注入 `192.168.65.7` 作为容器 DNS，该解析器**不返回 HTTPS(type 65) 记录**。
- 后果：节点带 `ech-opts: { enable: true, query-server-name: cloudflare-ech.com }` 时，
  Mihomo 报 `resolve ECH config error: no ECH config found in DNS records`，测活全部失败。
- 实测对比：宿主与公共 DNS（223.5.5.5 / 1.1.1.1 / 8.8.8.8）均返回 1 条 ECH 记录，容器旧 DNS 返回 0 条。
- 修复：重建容器时加 `--dns 223.5.5.5 --dns 119.29.29.29`。
- 排查手法：开 `-e LOG_LEVEL=debug` 后，`check/platform/google.go` 的 `slog.Debug(err.Error())`
  会打印真实失败原因；`checkAlive` 里那条 `slog.Debug("测活出错", "error", err)` 因
  `CheckAliveWithFallback` 吞掉错误而恒为 `<nil>`，**不要被它误导**。

### 测活失败的其他已知成因（按出现频率）
- `dns resolve failed: couldn't find ip`：IPv6 节点但配置 `ipv6=false`
- `failed to dial WebSocket: EOF`：节点自身已失效
- `REALITY authentication failed`：REALITY 节点凭证或时间不同步

### 配置热重载在容器内不可靠（2026-11-14 实测）
- `app/config.go` 用 fsnotify 监听 `config.yaml`，本机直接跑二进制时可靠。
- 但 Docker Desktop + Windows 挂载目录下，宿主机的文件事件**不一定传进 Linux 容器**，
  改完宿主机文件后容器可能仍用旧配置。
- 实测：把 `enabled: true` 改成 `false` 后触发检测，容器仍执行了已禁用的组。
- 结论：改宿主机 `config.yaml` 后需 `docker restart submatrix-test`；
  通过 WebUI「保存配置」走 `/api/config` 时后端主动写文件并重载，无需重启。
- **重载语义已收紧（2026-11-15）**：检测进行中收到的配置变更不再立即应用，
  而是记 `pendingReload`，等本轮检测彻底结束（含 `releaseMemory` 的内存回收）
  后由 `applyPendingReload` 统一应用。原因是 `loadConfig` 用
  `*config.GlobalConfig = newConfig` 做逐字段整结构覆盖且无锁，
  检测中途重载会让正在跑的那轮读到「一半新一半旧」的配置。
  排查时若发现「改完配置要等一轮检测结束才生效」，这是设计行为，不是故障。

## 11. 测速地址为空导致节点全灭（2026-11-14 修复）

### 症状与定位
- 症状：`check-speed: true` 的组可用节点恒为 0，`check-speed: false` 的组却有一批可用。
- 对照实验（同一组、同一批节点、同一镜像，只改一个开关）：
  `check-speed: false` → 15 可用；`true` → 0 可用。
- 日志关键行：`随机选择的测速URL  url=""` —— 测速 URL 是空字符串。

### 根因
- `config.yaml.example` 明确写着「speed-test-url 留空关闭测速」，
  但 `CheckSpeed` 没有实现该语义：空值时仍 `http.NewRequestWithContext(ctx, "GET", "", nil)`，
  `""` 不报错，请求发出后必然失败 → 节点判 `speed_fail` → 丢弃。注释与实现不符。
- 叠加因素：`fastSpeedTestURLs`（由测试聚合生成）可能为空数组，
  导致 `init()` 里 `testURLs` 保持 nil，`random` 语义也失效。

### 三层修复（`fix/alive-check`，commit dae862b）
1. `CheckSpeed` 入口：URL 为空直接返回 `(0,0,nil)`，不再发空请求。
2. `speed.go` 的 `init()`：`fastSpeedTestURLs` 为空时回退到静态 `SpeedTestURLs`（69 条）。
3. `ResolveGroup`：`SpeedTestURL` 为空时**强制** `CheckSpeed = false`，
   放在所有组级覆盖之后 —— 即使前端把开关点开也会被兜住，`min-speed>0` 时同样安全。

### 附带修复：测试会污染生产代码
- `TestAggregateSpeedResults` 直接写 `check/platform/speed_var.go`。
  网络受限时聚合结果为空，会把 24 条精选 URL 覆盖成空数组（该文件头写着 DO NOT EDIT，
  但测试自己在 EDIT 它）。
- 已加三道保护：`GENERATE_SPEED_URLS=1` 环境变量门控、聚合结果不足现有列表一半时拒绝覆盖、
  写临时文件后原子 rename（避免中断留下半截文件导致包编译失败）。
- 验证方式：`md5sum check/platform/speed_var.go` 跑测试前后应一致。

### 端到端验证结果
- hneko 组（`check-speed: true` + 空 `speed-test-url`）：修复前 0 可用 → 修复后 **6 可用**
  （2 vless + 4 hysteria2），日志 `enable-speedtest=false` 证明配置降级生效。
- mtf 组（`check-speed: false`）：15 可用。

## 12. 热重载：双保险 + 模板嵌入兜底（2026-11-14）

### 症状与根因
- 症状：容器内改 `config.yaml` 后完全无反应，等 2 分钟零日志。
- 根因：`initConfigWatcher` 把「inotify 初始化成功」当成「inotify 会收到事件」。
  Docker Desktop 的 bind mount 不把宿主机文件事件传进容器内核，事件永不到达；
  而轮询兜底只在 inotify **初始化失败**时才启动，于是两条路径同时哑火。

### 修复（commit f9e8664）
- **轮询无条件启动**：先起 `pollConfigFile` goroutine，再尝试 inotify，后者失败只记日志。
- **统一去重**：新增 `tryReloadByModTime()`，inotify 事件与轮询 tick 共用
  `configModMu` + `lastConfigMod`，只有 mtime 真正前进才触发 `onConfigChange`。
- 轮询间隔 5s → **3s**（容器内这是唯一响应路径）。
- `App` 结构体新增 `lastConfigMod time.Time` / `configModMu sync.Mutex`。

### 模板加载的三层兜底（save/save.go）
查找顺序：`output/<名>` → `assets/<名>` → **`go:embed` 内嵌副本** → HTTP 自拉。
- 容器镜像**不含 `assets/` 目录**，此前嵌入的 `EmbeddedOverrideYamlACL4SSR` 等变量
  从未被使用，读到失败就回退 HTTP 自拉（容器内回环，大概率失败）。
- 新增 `embeddedTemplate(filename)` 按文件名返回内嵌模板。

### 模板合并失败不再静默（save/save.go）
- 原实现 `err != nil` 时什么都不做：不写 `mihomo.yaml`、不打日志、不降级，
  用户只能看到文件消失而无任何线索。
- 改为 `err != nil` 与模板为空两种情况都 `slog.Warn` + 降级保存纯节点列表。

### 验证结果（hotreload2 镜像，全新空 output 目录）
| 验证项 | 结果 |
|---|---|
| 容器内配置热重载 | 1 秒内触发 |
| 连续两次改动（间隔 0.3s） | 只触发 1 次（去重生效） |
| 空目录下模板加载 | `mihomo.yaml` 正常生成 15266 字节 / 31 providers |
| 改 output/ 模板策略组名 | 产物中 17 处同步更新 |

### 规则模板热重载天然支持
`fetchLocalOrAssetTemplate` 每次调用都 `os.ReadFile`，无任何缓存。
改 `output/` 里任一模板，下次检测自动用新版，无需重启。

## 13. 全量检测配置基线（2026-11-15 实测 448 可用）

### 容器启动参数（缺一不可）
```powershell
docker run -d --name submatrix-test --restart always -p 8199:8199 `
  --dns 223.5.5.5 --dns 119.29.29.29 `
  --add-host "host.docker.internal:192.168.2.13" `
  -e HTTP_PROXY= -e HTTPS_PROXY= -e http_proxy= -e https_proxy= `
  -e ALL_PROXY= -e all_proxy= `
  -v "D:/newC/CCC/fuwuq/artifacts/submatrix/config:/app/config" `
  -v "D:/newC/CCC/fuwuq/artifacts/submatrix/output:/app/output" `
  wenxiloveyou/submatrix:latest
```
- `--dns` 公共解析器：容器默认 DNS(192.168.65.7) 不返回 ECH 的 HTTPS 记录
- `--add-host`：换 `--dns` 后 `host.docker.internal` 失效，必须写死宿主机 WLAN IP
- `-e *PROXY=`：Docker Desktop 会注入 `127.0.0.1:10801`，容器内那是自己，必然 connection refused
- Docker Desktop 的 `~/.docker/config.json` 的 `proxies` 段优先级最高，重建容器前先确认已移除

### 五组配置（2026-11-15）
| 组 | URL 数 | 可用 | 说明 |
|---|---|---|---|
| free-a | 19 | 229 | 公网免费源，可用率 1.5% |
| free-b | 5 | 16 | 公网免费源 |
| all-in-one | 2 | 181 | 聚合前两组产物 |
| hneko | 2 | 6 | 付费机场（sub-yes + study 两个域名） |
| mtf | 1 | 16 | 付费机场，可用率 73% |

### 关键配置项
- `check-speed: false`（全部组）：省流量，且规避空 `speed-test-url` 的历史坑
- `fetch-proxy: http://host.docker.internal:10801`（除聚合组）：GitHub raw 与境外订阅源必需
- `rule-preset: acl4ssr`：29 策略组 / 31 providers / 15 KB，与首尔 Debian 部署一致
- `timeout: 20000`：免费节点握手慢，5 秒不够

### Debian 订阅迁移结论
- 首尔 `subs-check` 的 token `d6ac01e7...` **已失效**（403 Forbidden 页面）
- 但其域名 `study.small1999.sbs` 仍可用，用本机有效 token 访问返回完整订阅
- 已把该域名加入 hneko 组，两个域名并行拉取

## 14. 报告地区分布：单组覆盖 → 全组合并去重（2026-11-16 修复）

### 症状
`stats/subs-analysis.yaml` 里的 `global_analysis.geography_distribution` 只显示
最后检测完的那一组。实测 mtf 最后一组跑完时报告只有 `SG 7 / KR 5 / TW 3 / US 3 / JP 1`，
一共 19 个节点——正好是 mtf 单组的量，其余四组的地区完全看不到。

### 根因
- `GenerateAnalysisReport` 是 `ProxyChecker` 的方法，数据源是 `pc.results`，**只有本组**。
- 它写的 `stats/subs-analysis.yaml` 是固定文件名，**每组的报告覆盖前一组**。
- 于是磁盘上最终只剩最后一组的统计。这不是数据计算错误，是覆盖写导致的信息丢失。

### 修法
- 新增进程级全局集合 `globalAliveNodes`（map + mutex + 插入顺序表），
  `ResetGlobalAliveNodes` 在每轮检测开始时清空。
- 每 `CheckGroup` 结束后由 `CollectGroupAliveNodes(results)` 并入，
  去重键用 `utils.GenerateProxyKey`（server + 端口 + 凭据 + SNI/Host 物理指纹）。
- 所有组检测完后 `WriteGlobalAnalysisReport` 用全局去重重写报告。
- 抽出 `applyNodeToStats`，让单组与全局共用同一套解析口径，避免两处正则分叉。

### 去重语义（务必保持）
- 同一物理节点被多个组引用 → 只算一次（free-a/free-b/all-in-one 共享订阅源，重复率约 47%）。
- **同 IP 不同凭据 → 保留为不同节点**。`GenerateProxyKey` 含密码/UUID，
  这是刻意的：不同账号限速/限流量独立，合并会少算真实可用节点。
- 单测 `check/analysis_stats_test.go` 固化了这两条语义。

### 验证手法（下次改统计时照做）
**总和校验**：把地区分布各值相加，必须等于 `alive_count`。
这次 213 = 213 才敢说统计正确；不等就说明有节点重复计入或静默丢失。

### 实测对照
| | 覆盖地区 | 去重可用节点 |
|---|---|---|
| 修复前（只 mtf） | 5 | 19 |
| 修复后（全合并） | 27 | 213（4 组合计 402，去重率 47%）|

## 15. 报告 YAML 手工拼接被引号截断，前端统计全 0（2026-11-16 修复）

### 症状
WebUI「节点质检分析报告」页面所有卡片显示 0：探测节点总数 0、可用节点总数 0、
存活率 0.00%、覆盖 0 个地区、暂无订阅源数据。但顶部的「上次检测时间」是正确的。

### 根因（两层）
1. `stats/subs-analysis.yaml` 里 `error:` 字段用手工拼引号：
   `WriteString("    error: \"") + WriteString(ErrMsg) + WriteString("\"")`。
   而 `ErrMsg` 常是 Go 标准库错误文本，形如 `Get "https://...": EOF`，**自带双引号**。
   YAML 读到内层第二个 `"` 就认为字符串结束，后面内容成为非法结构，**整份文件解析失败**。
2. `getAnalysisReport` 解析失败时走兜底分支，只返回 `{"report": 原文}`，
   没有 `report_data`。前端只认 `report_data`，因此所有卡片空白。

### 关键判据
**「上次检测时间对、其它全 0」= 接口通了但 report_data 缺失**，不是检测失败。
直接 curl `/api/analysis-report` 看有没有 `report_data` 键即可确诊。

### 修法
YAML 里所有用户/系统来源的字符串一律 `strconv.Quote`，不要手工包引号：
- `error:` 字段（`pStat.ErrMsg`、`r.ErrMsg`）
- `url:` 字段——**URL 里的 `#` 在 YAML 中是注释起始符**，
  组间引用常用的 `/group/x/all.yaml#tag` 不 Quote 会被截断。
- 已有的 `buildRemoteSubsYAML` 早就正确用了 `strconv.Quote`，说明正确做法一直清楚，
  只是 `saveDetailedAnalysis` 里漏改。

### 附：sub_url 与 SubStats 的键是两个不相交的集合（2026-11-16 踩坑）
做全组合并报告时曾试图「用节点的 `sub_url` 重建订阅源排行」，实测失败：
```
subAnalysis 键数=24   sortedURLs 键数=24   交集=0
```
- `SubStats` 的键 = **本组拉取过哪些 URL**（聚合组拉的是前组产物地址，
  如 `sub-s.wenxi.dev/group/free-a/all.yaml`）。
- 节点的 `sub_url` = **该节点来自哪个原始订阅源**（如 `raw.githubusercontent.com/...`）。

这是两个完全不同的维度，混用会让所有源掉进 `subs_ranking_bad`。
正确做法：`subs_ranking` 用各组 `GenerateAnalysisReport` 算好的 `subAnalysis`
累积（`mergeSubAnalysis`，同 URL 取较大者）；只有 `global_analysis`
（地区/协议/解锁）才用全局去重节点重建。

**两种统计的语义边界**：
- 地区分布 → 必须去重（同一节点不该在多个地区重复计入）
- 订阅源排行 → 不能去重（每个源要独立评估质量，聚合源的贡献不能被抹掉）

### 地区分布的两个静默错误（2026-11-16 用「和 == Total」校验查出）
在报告里把地区分布各值相加，与 `alive_count` 对比，一次抓出两个问题：

**1. 海盗旗节点静默消失（差 12）**
`🏴‍☠️`（`U+1F3F4 U+200D U+2620`）是**多码点组合 emoji**，机场用它标记
「匿名/未知地区」。`reFlag` 只匹配 `[\x{1F1E6}-\x{1F1FF}]{2}`（两个区域指示符），
海盗旗不在该范围，于是 `Total` 加了但 `Countries` 没加，节点在分布里凭空消失。
修法：无角标也无国旗时显式记入 `Countries["Unknown"]`。

**2. `⁰` 节点被重复计地区（多算）**
`reCFConsistent` 的字符范围是 `[\x{00B9}\x{00B3}-\x{2079}\x{207A}]`，
而 **U+2070（⁰）正好落在 `\x{00B3}-\x{2079}` 之内**，导致带 `⁰` 的节点
（本应只由 `reCFInconsistent` 处理）被两条正则各加一次 Countries。
实测 `🇺🇸US⁰_1|X` 的地区之和为 2，而 Total 为 1。
修法：把范围拆成 `[\x{00B9}\x{00B3}-\x{206F}]|[\x{2071}-\x{2079}]|[\x{207A}]`，
排除 U+2070。

**不变式（已写进单测）**：`sum(geography_distribution) == alive_count`。
改任何统计口径后都必须跑这条校验，它是发现「静默丢节点/重复计数」最便宜的手段。



### 验证手法
改完统计输出后，**不能只看文件内容像不像**（肉眼看着是合法 YAML 也可能解析失败）。
必须真的跑一次 `goccy/go-yaml` 的 `Unmarshal`，或 curl 接口确认返回体含 `report_data`。

## 16. 复合角标导致地区重复计数（2026-11-17 根因定位并修复）

### 症状
每轮检测的日志里都有 `地区统计不一致 total=X countries_sum=X+2 diff=2`，
**稳定为 2**，不是随机竞态。

### 定位过程（「加诊断日志」如何换来根因）
先在 `WriteGlobalAnalysisReport` 里加了一个**只在总和确实不一致时才跑**的
逐节点自检（正常情况零开销，异常时打印每个不合格节点的名字）。一次端到端
检测就直接打出了元凶：
```
单个节点地区计数不对称 name=🇺🇸US¹-US⁰_1|免费A type=http total=1 countries_sum=2
单个节点地区计数不对称 name=🇺🇸US¹-US⁰_2|免费A type=http total=1 countries_sum=2
```

### 根因
机场用**复合标注**表达「主状态 + 附加状态」：`🇺🇸US¹-US⁰` 表示
「CF 一致（¹）、但 IP 不一致（⁰）」。四条角标正则**各自独立匹配**：
`US¹` 命中 `reCFConsistent`、`US⁰` 命中 `reCFInconsistent`，
于是同一个节点给 `Countries` 加了两次。

### 修法
抽出 `classifyByTag(name)`，四类**互斥**只取第一个命中的，按信号强度排序：
`block(⁻¹) > incon(⁰) > consistent(¹⁺) > nonCF(²)`。

### 为什么此前的穷举测试没抓到
测试只用了**单角标**命名（`US⁰`、`SG¹⁺`、`TW²`），从没构造过
`XX¹-XX⁰` 这种**双段复合**格式。教训：解析类测试要覆盖「格式的组合与嵌套」，
不只是单一形态的枚举。

### 当前不变式（两条，都要校验）
- `sum(geography_distribution) == alive_count`
- `sum(protocol_distribution) == alive_count`
两条都过才证明「每个节点恰好被计入一次地区、一次协议」。

## 17. 地区分布支持按组筛选（2026-11-17 新增）

### 需求
地区分布不再固定显示「所有组合并去重」，用户要能选一个组、只看它自己的节点。

### 实现要点
- `globalAliveNode` 增加 `Groups []string`，记录该物理节点被哪些组引用过。
  `CollectGroupAliveNodes(results, groupName)` 在节点已存在时只追加组名、不重复计数。
  这一步是必需的前提：原数据结构只有 `{Name, Type, Speed}`，拿不到组归属。
- `WriteGlobalAnalysisReport` 在累加全局统计的同时，按 `Groups` 为每组各算一份
  `AnalysisStats`，写进报告的新段 `per_group_geography`。
- 前端 `AnalysisView` 顶部下拉：默认「全部组（合并去重）」，可选单个组；
  切换后 4 维指标卡片、世界地图、地区列表、协议分布全部跟着切。

### 口径（关键，勿混淆）
```
各组 alive 之和  >  全局去重后的 alive_count
```
因为同一节点可能被多个组引用（`all-in-one` 聚合 `free-a` + `free-b` 的产物）。
实测：5 组之和 647 vs 全局去重 338，差值 309 正是共享节点。
**这是设计如此**：每个组内部各自去重、组间不去重，才符合「这个组里有什么」的语义。

若哪天发现各组之和等于全局数，反而说明组归属记录失效了，要查 `Groups` 是否没被写进去。

### 校验方式
`per_group_geography` 里每组须满足：
```
sum(该组 geography_distribution) == 该组 alive
sum(该组 protocol_distribution) == 该组 alive
```

## 18. 节点明细页：存活硬删除 / 协议与速度软禁用（2026-11-18 新增）

### 三种状态（这是全项目的统一口径）
| 状态 | 含义 | 是否进 all.yaml | 能否恢复 |
|---|---|---|---|
| `alive` | 测活通过 + 速度达标 + 协议允许 | 是 | — |
| `skipped` | 协议命中 `filter-protocols` | **否** | 改 filter-protocols 后重跑 |
| `slow` | 速度低于 `min-speed` | **否** | 改 min-speed 后重跑 |
| 死亡 | 测活失败 | 否 | **硬删除，永久消失** |

死节点在 `check/check.go` 的 `alive_fail` 分支 `job.Close() + continue` 后
就不存在了——它从不进入 `resultChan`，因此在任何产物、任何接口里都看不到。
这是「存活过滤=硬删除」的实现方式，不是靠后续过滤。

### 产物分层
```
groups/<name>/
  all.yaml         仅 alive（客户端直用；实测 _state 出现 0 次）
  all-raw.yaml     全量（alive+skipped+slow），同样剥离 _state
  nodes-view.json  前端展示载荷，含 state 与 reason，不含任何凭据
```
`_state` / `_state_reason` 是**内部标记**，`stripInternalState` 会在写
all.yaml / all-raw.yaml 前剥离。它们在 share_handler.go 里是**可对外分享**的文件，
不能把内部字段暴露给客户端。

### 状态判定的兼容性（易踩）
`save.go` 判定状态时 `_state` 优先，缺失则**回退看旧的 `skipped: bool`**。
两套标记并存的原因：`_state` 是新增的（带可读原因），`skipped` 是既有字段，
历史产物与部分测试仍只带后者。只认 `_state` 会让这些节点被误判成 alive
（`TestSaveGroupAllSkipped` 就是因此失败的）。

### 接口
`GET /api/nodes-view?group=X`
- 有 group：直读该组 `nodes-view.json`
- 无 group：合并各组 nodes 并汇总 total/alive/skipped/slow
- 组名走 `groupNamePattern` 校验（防路径穿越）

### 验证手法（改这块必做）
1. 临时给某组配 `filter-protocols: [vless]`，跑一次检测，确认：
   - `nodes-view.json` 里 skipped > 0 且每条带原因
   - `alive` 里该协议数量为 0
2. `grep -c "_state" output/groups/*/all.yaml` 必须全为 0
3. 验证完**记得恢复配置并重启容器**（容器内热重载不可靠）

## 19. 统一命名与原始名（2026-11-18 新增）

### 统一命名 `unify-name`
开启后同组节点共用一个基础名，序号由 `RenameDuplicateNodes` 自动追加：
```
MTF 组、MTF 组-1、MTF 组-2 ……
```
基础名优先级：`node-prefix` → `display-name` → 组名。
统一命名时**不再追加 `|sub_tag` 后缀**（否则名字又乱回去）。

### 三条必须记住的实现细节
1. **`RenameDuplicateNodes` 必须作用于主产物**。
   它原先只用在历史池（save.go 的 historyPool 去重处），
   主产物 `proxiesList` 从来没做名称唯一化。统一命名一开，
   同组节点基础名完全相同，不补这一步就会出现一堆无法区分的同名节点。
   位置在 `SuccessLimit` 截取**之前**（否则截取后可能又重名）。
2. **`raw_name` 在重命名之前捕获**。
   `updateProxyName` 开头先把原始 `name` 存进 `raw_name`，
   否则重命名覆盖后就再也拿不到机场原本的命名。
   `raw_name` 已在 `sanitizeProxyForYAML` 的字符串白名单内。
3. **节点明细在「节点分组」页，不在「节点统计」页**。
   每个组卡片下方就地展开（复用 `/api/nodes-view`），
   改完组配置能立刻对比节点变化，不用两页来回跳。

### 命名链路全貌（改名字相关逻辑时照这张图核对）
```
原始名 raw_name（先在 updateProxyName 开头保存）
   ↓
RenameNode 开启时改 name：
   ├─ UnifyName → node-prefix || display-name || 组名
   └─ 否则     → NodePrefix + Rename(国家,角标,增强标签)
   ↓
追加标签：速度 / 平台 / sub_tag（UnifyName 时跳过 sub_tag）/ ISP
   ↓
save 层 RenameDuplicateNodes 补序号
```

## 20. 前端改动必须「vite build → docker build → 重启容器」三步连做（重要）

### 症状
改完前端、`docker build` 了、容器也重启了，但浏览器里看不到新功能。

### 根因（踩过两次）
前端产物 `webui/static/dist/*` 通过 `//go:embed` 烤进二进制，
所以**镜像里的前端 = 构建那一刻磁盘上的 app.js**。
如果先 `docker build` 再 `vite build`，镜像里就是旧版；
如果只 `vite build` 不重建镜像，镜像里也是旧版。

### 正确顺序
```bash
cd frontend && npx vite build
cd .. && docker build -f Dockerfile -t submatrix:xxx .
docker rm -f submatrix-test && docker run ... submatrix:xxx
```

### 排障：确认镜像里到底是哪个版本
```bash
curl -s http://127.0.0.1:8199/static/dist/app.js | grep -c "<新功能的某个中文字串>"
# 返回 1 = 已更新；0 = 镜像里是旧版
```
浏览器端还需 `Network.clearBrowserCache` + `Page.reload(ignoreCache=True)`，
普通 reload 会用缓存的旧 JS。

### 验证时注意
用 `bs4`/JS 检查页面是否含某中文串时，**确认字码正确**。
本会话曾把「看」写成 `0x7709`（那是「眉」），误判功能没生效。
正确：查 0x67e5 / 看 0x770b / 节 0x8282 / 点 0x70b9。

## 21. 免检模式（关闭连通测活）下的 UI 收起规则（2026-11-19）

关闭组的「启用连通测活」后，GroupsView 只显示基础字段（分组名/URL/开关主卡）
和一行说明文字，**以下全部隐藏**：
- 统一命名（UnifyName）
- 增强位置角标（EnhancedTag）
- 节点名前缀（NodePrefix）
- ISP 检测 / 流媒体AI检测 / 测活超时 / 测速相关
- 丢弃相关（DropOnFail）

说明文字：免检模式仅直连放行节点，探测、智能改名与丢弃功能均不生效。
（「智能改名」大开关已于 2026-11-21 移除，见第 22 节。）

## 22. 改名链路：大开关已移除，子项独立生效（2026-11-21）

### 现状
「智能改名（RenameNode）」总开关已删除。改名行为由三个独立子项驱动，
优先级 **UnifyName > EnhancedTag > NodePrefix**，全部关闭 = 保持原始名：

| 子项 | 行为 |
|---|---|
| UnifyName | 基础名（NodePrefix \|\| DisplayName \|\| Name）+ 自动序号 |
| EnhancedTag | 国旗+国家角标+序号（如 🇭🇰HK²_1\|Tag），唯一消费位置码 |
| NodePrefix | 仅在原始名前加前缀（不改名、不查位置）|

### 关键实现细节（改这块必看）
1. **needLocInfo 触发条件**（check.go:1339）：
   `ISPCheck || MediaCheck || (EnhancedTag && !UnifyName)`。
   UnifyName/NodePrefix 纯拼串不需要位置查询，不查省一次网络往返。
2. **迁移**（app/config_migrate.go rewriteRenameNode）：
   `rename-node: true` → `enhanced-tag: true`；`false` → 删行。
   **替换前先删同缩进的已有 enhanced-tag 行**（旧配置可能两个都有，防重复键）。
3. **progress.go 的 renameActive**：用 ProgressTracker 的
   `UnifyName||EnhancedTag||NodePrefix!==""`（组级），替代旧的
   `config.GlobalConfig.RenameNode`（读全局而非组级，不一致）。
4. **AppendOriginalName 已删除**：后端从无消费点（历史只剩注释说明
   「国旗+原名滚雪球」叠字 bug），raw_name 字段承担溯源。
5. 前端新组默认 `EnhancedTag: true`（与迁移一致，保默认改名体验）。

### 验证手法
- 端到端看产物节点名：enhanced-tag 时应为 `🇭🇰HK²_1|Tag` 格式；
  UnifyName 时应为 `mtf-1` 格式；全关时保持原始名。
- 迁移验证：config.yaml 里 `rename-node` 消失、`enhanced-tag` 唯一。
