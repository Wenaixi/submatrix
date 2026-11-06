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

### CLI subctl（cmd/subctl/）
- 命令：`validate` / `get <路径>` / `set <路径> <值>` / `groups list|toggle` / `presets`。
- 路径语法：点号 + 切片下标，如 `subctl get groups.0.name`；
  `set` 支持 bool/int/string/逗号分隔 []string/YAML 片段。
- 依赖：新增 `github.com/urfave/cli/v2`（v2.27.5）。
- 用途：容器/服务器/无浏览器环境下精确读写配置，替代 WebUI 操作。
