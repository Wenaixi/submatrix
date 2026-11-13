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
