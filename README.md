# SubMatrix

高性能订阅拓扑矩阵与节点检测管理工具。按组独立检测、串行按优先级执行、每个组的订阅产物独立分发，内置 WebUI 与无头 CLI。

基于 Go 与 React 构建，单二进制部署，适配云端、路由器与桌面环境。

---

## 特性

- **按组独立检测**：每个组配置自己的订阅源、检测参数、规则预设，组间串行按 `priority` 执行
- **组级优先级**：`priority` 小→大串行；优先级相同的组按配置声明顺序
- **节点过滤双轨制**：`filter-protocols` 命中的节点不参与检测，但**仍然落盘**（`all-raw.yaml`）——取消过滤即瞬间恢复全部节点
- **凭证指纹去重**：`type + server + port + 凭证 + TLS` 全字段指纹，同 IP 不同账号的节点都会保留
- **组间引用**：组 URL 支持相对路径（`/group/<name>/all.yaml`），自动展开为公网域名，杜绝回环地址
- **规则集预设**：内置 5 档预设，默认使用 126 策略组最详分流模板（含流媒体/AI 全分流）
- **按组落盘**：每组独立生成 `all.yaml`（已检测）/ `all-raw.yaml`（全量原始）/ `mihomo.yaml`（融合规则）/ `history.yaml`（历史评分）
- **现代 WebUI**：设置表单、分组拖拽排序、检测报告、文件管理、日志查看
- **无头 CLI**：`subctl` 命令行精确读写配置，适合容器/服务器场景
- **内置 Sub-Store**：订阅管理、合成、分享码，与主服务单端口整合
- **增强位置标签**：`HK²`（非CF直连）/ `HK¹⁺`（CF同区）/ `HK¹-US⁰`（CF跨区）

## 快速开始

### 二进制运行

下载 Releases 中适合平台的版本，解压后直接运行：

```bash
./submatrix -f ./config/config.yaml
```

首次运行会生成默认配置。记录终端输出的 `api-key`，访问 WebUI。

### Docker 运行

```bash
docker run -d \
  --name submatrix \
  -p 8199:8199 \
  -v ./config:/app/config \
  -v ./output:/app/output \
  --restart always \
  wenxiloveyou/submatrix:latest
```

### WebUI 控制面板

浏览器访问 `http://localhost:8199/admin`，输入 `api-key` 登录。

默认端口 `8199`，如已修改请以实际为准。

## 核心概念

### 组（Group）

组是 SubMatrix 的基本执行单元：一组订阅源 + 一组检测参数 + 一组规则预设。组配置写在 `config.yaml` 的 `groups` 段：

```yaml
groups:
  - name: free-a
    display-name: 公网免费 A 组
    priority: 10          # 越小越先执行
    check-speed: true     # 是否测速
    min-speed: 0          # 最低测速门槛（KB/s，0 不限制）
    rule-preset: pro_full # 规则集预设
    filter-protocols: [http]  # 过滤协议：不检测但保留
    urls:
      - https://example.com/sub.yaml
  - name: all-in-one
    display-name: 全量聚合
    priority: 900         # 聚合组放最后
    filter-protocols: []
    urls:
      - /group/free-a/all.yaml  # 相对路径：引用其他组产物
      - /group/free-b/all.yaml
```

### 组间引用与公网地址

组 URL 支持两种写法：

- **绝对地址**：`https://example.com/sub.yaml` —— 直接拉取。指向本机回环地址（`127.0.0.1` / `localhost`）会被拒绝。
- **相对路径**：`/group/<name>/all.yaml` —— 系统内引用，自动展开为 `https://<public-url>/group/<name>/all.yaml?token=<api-key>`，并附加鉴权参数。

相对路径展开依赖 `public-url` 配置。未配置时该组报错跳过（防止回环引用）。

### 过滤双轨制

`filter-protocols` 决定哪些协议**不参与检测**（默认过滤 `http` 明文协议）：

- 命中的节点跳过测活/测速/打标，但**仍然落盘**到 `all-raw.yaml`
- `all.yaml` 只包含已检测节点，`mihomo.yaml` 只融合已检测节点
- 把 `filter-protocols` 置空即可恢复全部节点的检测与落盘

### 检测流程

```text
ResolveAllGroups → 按 priority 排序 → 逐组串行：
  FetchGroupProxies（拉组 URLs → 凭证指纹去重 → 过滤打标）
    → CheckGroup（测活 → 测速 → 媒体检测 → 重命名）
      → SaveGroup（落盘 all/mihomo/history/all-raw）
```

组间失败互不影响，`ForceClose` 可随时中断剩余组。

## 规则集预设

| 预设 | 说明 |
|---|---|
| `pro_full`（默认） | 126 策略组最详分流矩阵：流媒体 + AI + 各厂全家桶 + 去广告/隐私拦截 |
| `pro_lite` | 精简分流矩阵（PRO-Lite 核心轻量） |
| `acl4ssr` | 经典 ACL4SSR 在线全量规则集 |
| `direct_pure` | 纯节点直通（仅节点列表，无策略组） |
| `custom` | 自定义外部模板，配合 `overwrite-url` 使用 |

`pro_full` 模板内置 25 个 rule-providers：自家 5 个 Provider（指向组产物）+ blackmatrix7 全站 18 个 + AdRules 去广告 + Privacy 隐私。rules 首两条 `RULE-SET,AdRules,REJECT` 与 `RULE-SET,Privacy,REJECT` 优先拦截广告与隐私追踪。

## 产物说明

每个组的产物输出在 `output/groups/<name>/`：

| 文件 | 内容 |
|---|---|
| `all.yaml` | 已检测通过的节点纯列表（客户端直用） |
| `all-raw.yaml` | 全量原始节点（含未检测的 skipped 节点） |
| `mihomo.yaml` | 按组规则预设融合后的完整 Mihomo 配置 |
| `history.yaml` | 本组历史评分池（供下次检测复用） |

订阅直链：`https://<public-url>/group/<name>/all.yaml?token=<api-key>`（mihomo 配置在 `/group/<name>/mihomo.yaml`）。

## 无头 CLI（subctl）

`subctl` 是随主程序一起构建的命令行工具，适合容器/服务器场景精确读写配置。

```bash
subctl validate                # 校验配置语法与关键字段
subctl get groups.0.name       # 读取配置值（点号路径 + 下标）
subctl set public-url "https://sub.example.com"  # 写入配置值
subctl groups list             # 列出分组（含优先级）
subctl groups toggle free-b true   # 启停分组
subctl set groups.0.priority 10    # 调整优先级
subctl presets                 # 查看规则集目录与各组预设
```

路径语法：点号分隔字段名，切片用数字下标（如 `groups.0.urls`）。`set` 支持 bool / int / 字符串 / 逗号分隔数组。

## API 摘要

带鉴权（`X-API-Key` 头、`?token=` 查询或 Bearer）：

- `GET /api/config` / `POST /api/config`：读取 / 保存配置
- `GET /api/groups` / `POST /api/groups`：组列表 / 整表保存（拖拽排序复用此接口）
- `POST /api/groups/toggle`：组启停
- `GET /api/status`：检测状态
- `POST /api/trigger-check`：手动触发检测
- `POST /api/force-close`：强制中断
- `GET /api/logs`：日志
- `GET /api/analysis-report`：检测分析报告

公开路由：`/group/:name/*` 组订阅分发、`/sub/*` 加密分享、`/more/*` 文件分享。

## 配置速查

关键配置项（完整见 `config/config.yaml.example`）：

```yaml
public-url: ""          # 公网访问地址（组间相对路径引用依赖此项）
api-key: ""             # 所有接口与订阅链接的鉴权令牌
listen-port: ":8199"    # 监听端口
min-speed: 0            # 最低测速门槛（KB/s）
rename-node: true       # 按 IP 地区重命名节点
enhanced-tag: true      # 增强位置角标（HK² 等）
filter-protocols: [http] # 组内过滤协议（不检测但保留）
cron-expression: "0 2 * * *"  # 定时检测计划
```

## 开发

```bash
# Go 后端
go build -o submatrix .          # 需要 CGO（quickjs 依赖）；本地 Windows 无 gcc 时用 Docker 构建

# 前端
cd frontend && npm install && npm run build   # 产物输出到 webui/static/dist/

# Docker（国内网络：需清代理 build-arg）
docker build --build-arg HTTP_PROXY= --build-arg HTTPS_PROXY= \
  --build-arg http_proxy= --build-arg https_proxy= \
  --build-arg ALL_PROXY= --build-arg all_proxy= \
  -t wenxiloveyou/submatrix:latest .
```

## License

本项目基于 MIT License 分发。上游参考：subs-check-pro（sinspired）、subs-check（beck-8），本项目已在其基础上深度重构。