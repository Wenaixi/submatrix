# SubMatrix 核心工程架构与运维规范

## 一、项目定位与基座架构
- **项目名称**：`SubMatrix`（高性能订阅拓扑矩阵与节点筛选工具）
- **开源仓库**：`https://github.com/Wenaixi/submatrix`
- **官方镜像**：`wenxiloveyou/submatrix:latest`（支持 `linux/amd64`、`linux/arm64` 双架构）
- **官方专属 Logo**：采用 `gpt-image` 生成的赛博黑曜石背景与翡翠电光青（#00ffcc）立体 3D 等距网络路由拓扑矩阵魔方 Icon。
- **单端口设计**：对外暴露唯一端口 `:8199`。内置 Sub-Store 严格绑定 `127.0.0.1:8299`，由 Go 核心服务通过 `UnifiedAuthMiddleware` 安全反代至 `/sub-store/*`，彻底杜绝端口冲突与未授权暴露。

## 二、视觉与交互系统设计（去圆角高级工业美学）
- **去圆角顶级硬核切角**：全面清除软绵绵的大圆角（`border-radius: 0 !important`），卡片、按钮、输入框、导航栏、抽屉均采用极客工业级利落切边；
- **微微微倒角质感**：极少数微标签采用 `1px` 微倒角增强精致感；
- **1px 细微光框线**：采用 `1px solid rgba(255, 255, 255, 0.12)` 与深黑背景对比，展现精密仪器级美感；
- **等宽技术排印**：统计指标、节点计数、代码均采用 `font-variant-numeric: tabular-nums` 与等宽字型；
- **矩形机械滑动开关 (Matrix Switch)**：定制矩形轨道与金属光泽滑块，配合赛博荧光绿（`#00ffcc`）激活光晕。

## 三、分组矩阵架构与深度可视化 UI
- **全生命周期独立控制**：在 WebUI 中提供专属“分组矩阵 (Groups Matrix)”管理面板：
  - **总开关 (Master Switch)**：一键对单组执行【启用 / 禁用】，禁用组自动呈现灰度呼吸效果，不参与测试与聚合；
  - **4 维独立工业开关**：
    1. 连通测活开关 (`check-alive`)
    2. 带宽测速开关 (`check-speed`)
    3. 智能改名开关 (`rename-node`)
    4. 失败丢弃开关 (`drop-on-fail`，失败丢弃 vs 离线沉底)
  - **参数输入**：支持组英文标识 (`name`)、显示名称 (`display-name`)、节点标签 (`tag`)、探测超时 (`timeout`)；
  - **订阅源文本域**：支持批量粘贴与行数即时统计；
  - **聚合直链一键复制**：提供针对该组的专属直链复制（Clash YAML / Mihomo YAML / Base64），自动拼装当前 Host、Port 与 Token；
  - **新增与删除**：提供【+ 新增分组】模板创建与【删除此分组】操作。
- **配套后端 API**：
  - `GET /api/groups`：获取当前分组与端口元数据；
  - `POST /api/groups`：全量保存并热重载写回 `config.yaml`；
  - `POST /api/groups/toggle`：即时切换单组启用状态。

## 四、客观打标与原名保护规范
- **客观 IP 画像 (`tag-ip-type`)**：根据 GeoIP/ASN/ISP 客观判定并打标 `|家宽`、`|机房`、`|移动`，绝无主观猜测；
- **原名安全保留 (`append-original-name`)**：末尾追加 ` (原名: <RootOrig>)`，小括号包裹避免 Clash 正则切分歧义，并通过 `extractRootOriginalName` 幂等拆解防止多重嵌套。

## 五、CI/CD 与测试基线
- 镜像构建全部由 GitHub Actions 云端执行多架构 Buildx 并发布至 Docker Hub；
- 严格基于用户指定的 23 个 GitHub 开源免费订阅源，实测拉取 20,003 个真实节点无缝跑通！
