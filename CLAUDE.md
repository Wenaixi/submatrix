# SubMatrix 架构规范与记忆库 (SubMatrix Architectural Specification)

## 一、项目定位与核心使命
SubMatrix 是从 `sinspired/subs-check-pro/v3` 深度独立分支演进的高性能、现代化订阅拓扑矩阵与多级质检分发系统。
- **独立仓库**: `https://github.com/Wenaixi/submatrix`
- **官方镜像**: `wenxiloveyou/submatrix:latest`
- **单端口核心原则**: 宿主机仅对外暴露唯一的 `8199` 端口，内部伴生的 Sub-Store 引擎 (`127.0.0.1:8299`) 严格通过 Go 反向代理 (`/sub-store/*`) 运作，对外完全作为 SubMatrix 的内部节点转换内核，杜绝多端口冲突与外部暴露。

---

## 二、前端现代化 React 单页应用架构 (React 18 SPA)

### 1. 技术栈与工程目录
- **核心框架**: React 18 + TypeScript + Vite
- **源码目录**: `artifacts/submatrix/frontend/`
- **编译产物**: `artifacts/submatrix/webui/static/dist/` (`app.js` + `app.css`)
- **嵌入机制**: Go `webui/embed.go` 原生嵌入静态产物，`admin.html` 作为轻量 SPA 挂载壳。

### 2. 单页视图规划 (100% 零功能遗漏)
- `<GroupsView>`（默认首屏主舞台）：拓扑矩阵调度中心。提供双栏规整卡片流、4 列独立微型开关（连通测活/带宽测速/智能改名/失败丢弃）、折叠式订阅源编辑、大号黑白高对比直链复制键（Clash/Mihomo/Base64）与一键保存持久化。彻底去除画蛇添足的“快速预设”药丸，回归规整纯粹。
- `<SubscriptionsView>`：全量订阅大厅。卡片化集中管理 Clash 全量、Mihomo 专享、Base64 通用与历史归档订阅直链。
- `<MonitorView>`：运行监控与黑匣子终端。顶部常驻节点总数、可用量、当前进度与 ETA 预估微条；下方提供全屏等宽字型日志终端，支持实时滚屏、关键词检索、一键清空与复制。
- `<FilesView>`：内置文件管理。彻底取代旧式跳出弹窗的 `files.html`，在当前单页无缝加载 output 目录文件列表，支持在线代码高亮预览抽屉、一键下载与直链复制。
- `<AnalysisView>`：节点质检分析报告。彻底取代旧式跳出弹窗的 `analysis.html`，以极简数据微卡与极细线框呈现多维质量指标（存活率、平均带宽、国家地区画像分布、代理协议占比）。
- `<ConfigView>`：系统高级配置中心。提供纯文本 RAW YAML 编辑器，直通 `config.yaml` 底层参数，支持即时保存并触发 Go 后端热重载。
- `<AuthGuard>` / `login.html`：纯白极简系统访问认证卡片。

---

## 三、视觉与设计系统规范 (Claude Design System 深度实施)

### 1. 纯白为主与极致全黑深色双模系统 (Monochrome Dual-Mode)
- **纯白默认模式 (White-Dominant)**:
  - 页面背景 `--surface-1: #FAFAFA`
  - 工作台底座 `--surface-0: #FFFFFF`
  - 卡片与输入框 `--surface-2: #FFFFFF`
  - 曜石黑正文字体 `--text-primary: #09090B`
  - 次级信息中性灰 `--text-secondary: #71717A`
- **全黑深色模式 (Pitch Black)**:
  - 页面背景 `--surface-1: #09090B`
  - 核心基底 `--surface-0: #000000`
  - 卡片容器 `--surface-2: #101012`
  - 纯白正文字体 `--text-primary: #FFFFFF`
  - 次级信息冷银灰 `--text-secondary: #A1A1AA`
- **顶栏一键切换**: 顶栏右侧提供无边框 `Sun` / `Moon` 线性矢量图标按钮，无缝双向切换，自动记忆写入 LocalStorage。

### 2. 严苛设计铁律
1. **绝对禁止 Emoji**: 全站任何界面、按钮、提示与 Toast 中一律严禁出现 Emoji 表情符号，全部采用 1.5px 极细线性 Outline 矢量图标（`lucide-react`）；
2. **纯粹扁平 (Pure Flat)**: 严禁出现任何渐变、噪点、发光、彩色霓虹光晕与投影（drop-shadow）；
3. **字重锁定**: 严格锁定在 `400`（常规）和 `500`（中粗）两档，严禁使用粗笨的 600/700；
4. **精密线框 (Hairline Borders)**: 统一采用 `0.5px` ~ `1px solid var(--border)` 微细线框；
5. **迪特·拉姆斯机械开关 (Matrix Switch)**: 停用时浅灰底座白滑块，启用时纯黑底座白滑块（深色模式下反色高亮），黑白分明，机械触感十足。
6. **彻底消灭跳出与参数**: 严禁使用 `window.open` 弹新标签页，消灭所有 `?theme=dark` 拼接参数。
