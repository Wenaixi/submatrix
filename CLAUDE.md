# SubMatrix 核心工程架构与运维规范

## 一、项目定位与基座架构
- **仓库地址**：`https://github.com/Wenaixi/submatrix`
- **官方镜像**：`wenxiloveyou/submatrix:latest`（支持 `linux/amd64`、`linux/arm64` 双架构）
- **核心基座**：采用 `sinspired/subs-check-pro/v3` 原生高并发底座，深度整合现代化全功能 WebUI。
- **单端口设计**：仅暴露公网端口 `8199`。内置 Sub-Store 严格监听 `127.0.0.1:8299`，由 Go 核心服务通过 `UnifiedAuthMiddleware` 反向代理至 `/sub-store/*`，彻底杜绝端口冲突与未授权暴露。

## 二、核心特性与实现规范

### 1. 订阅分组拓扑矩阵 (Groups)
- 支持多组级联继承，每组均可独立控制：`check-alive`（测活）、`check-speed`（测速）、`rename-node`（重命名）、`drop-on-fail`（失败丢弃/沉底）、`urls` 等。
- **纯透传聚合组**：通过设置 `check-alive: false` 与 `check-speed: false`，零探测耗时直接汇聚子组订阅，实现极速两阶段调度。
- **物理节点去重**：聚合时按 `strings.ToLower(fmt.Sprintf("%s:%s:%s", server, port, type))` 进行物理去重。

### 2. 客观 IP 类型画像打标 (tag-ip-type)
- 通过 GeoIP / ASN / ISP 数据进行 4 级客观打标，绝无主观猜测：
  - `|家宽`：ISP 为住宅宽带接入商；
  - `|机房`：托管服务商、云厂商与数据中心（IDC）；
  - `|移动`：移动蜂窝网络。

### 3. 原名安全保护 (append-original-name)
- 节点尾部统一追加：` (原名: <RootOrig>)`，使用小括号包裹，严格避免与 Clash 的分节符产生正则冲突；
- 通过 `extractRootOriginalName` 函数实现幂等拆解，杜绝多次改名后的递归嵌套膨胀。

### 4. 统一鉴权网关 (UnifiedAuthMiddleware)
- 覆盖全站所有敏感端点：`/admin/*`、`/api/*`、`/group/*`、`/sub/*`、`/sub-store/*`。
- 支持四种鉴权凭据输入方式：
  - URL Query 参数：`?token=xxx`
  - URL Query 参数：`?api_key=xxx`
  - 请求头：`X-API-Key: xxx`
  - 请求头：`Authorization: Bearer xxx`
- 采用 `subtle.ConstantTimeCompare` 恒定时间比对，全面防范时序侧信道攻击。

## 三、云端 CI/CD 构建规范
- 所有发布镜像一律由 GitHub Actions 云端流水线（`.github/workflows/docker.yml`）自动构建，禁止本地交叉编译；
- 容器采用 `golang:1.24-bookworm` + `busybox:glibc` 多阶段构建，原生支持 CGO 动态链接与自适应 Go 工具链（`GOTOOLCHAIN=auto`）。

## 四、测试与验收基线
- 严格基于用户指定的 23 个 GitHub 开源免费订阅源；
- 实测 20,003 个真实节点拉取成功，全量并发测活耗时 2 分 13 秒，可用节点数 110 个；
- 经 Google Chrome（CDP 远程调试）全景截取仪表盘、配置面板、YAML 编辑器与分析报告，所有功能均已通过真实视觉审计。
