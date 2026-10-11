# VIS-FILM-DESIGN-1 设计件三：播放与导出（webui 播放器 + MP4 导出）

- 归属：VIS-FILM-DESIGN-1 设计基线四件之三
- 勘察底座：webui artifact 服务面/iframe 先例/时钟源现状（锚点见文内）
- v1 范围：PC webui 播放 + MP4 导出（Windows 真栈验证）；v2=移动端 feed 封面挂载+用户手改落回

## 1. webui 播放器

### 1.1 挂载面

- **surface 路由**：`agent/webui/src/main.tsx:7-12` 已有 `?surface=trajectory` 先例——加 `?surface=film` 分支渲染 `FilmPlayerView`，零构建配置改动（备选：vite 多入口 film.html，v1 不需要）。入口按钮位：v1 从资料库 artifact 预览进入（film.json 预览卡加"播放成片"动作）或 URL 直入；正式入口归 FE-GUI-SHRINK 后续轮统一裁定。
- **镜头页获取**：iframe `src` 走现有 artifact 文件端点 `GET /agent/artifact/file?id=<artifact_id>`（URL 构造 `agent/webui/src/lib/api.ts:272-274` `artifactFileURL`；handler `agent/internal/chat/server.go:1365-1389`）。镜头页自包含单文件（设计件一 §1.3）使 ServeFile 无目录路由不构成限制。film.json/整曲 WAV 同为 artifact（agent 侧生成后 `artifacts.Upsert` 登记）。
- dev/prod 同构：vite proxy 已转 `/agent`（`agent/webui/vite.config.ts`），iframe src 走相对路径两态一致。

### 1.2 时钟与帧驱动（v1 关键裁定）

**v1 播放器=本地音频元素主时钟**，与 DAW transport 解耦：

```
<audio src={artifactFileURL(wav_id)}>          // 整曲渲染 WAV（数据面同源产物）
rAF loop: t = audio.currentTime
  → film.json 镜头表查当前镜头（t_in ≤ t < t_out）
  → iframe.contentWindow.film.frame(t - t_in)
```

理由：
- 勘察实证 webui 无程序化音频时钟（全库无 AudioContext 编程用法；artifact 音频仅原生 `<audio controls>`，`App.tsx:11381`）；服务端 playhead 源 `ui_context.playhead_seconds` 轮询 8s 粒度（`App.tsx:392`）不够喂 frame(t)（事件流 250-500ms 也偏粗且 transport 事件不保证逐帧）；
- 纯函数 frame(t) 天然适配本地时钟：播放器是"成片预览"不是"混音监听"——语义本就该独立于工程 transport；
- 音画同步由数据面同源保证：WAV 即包络分析的那次渲染（sha256 关联，见 film.json `audio.source_render_sha256`），t 轴零漂移。
- 工程内同步播放（DAW transport 驱动 frame(t)）= v2 议题，需要细粒度 transport 订阅面（届时评估 ZMQ 事件直通 webui）。

镜头切换：切换瞬间新 iframe 已预载（相邻镜头预载策略：当前镜头开播即预热 next iframe，display 切换），避免切换白屏。v1 无转场（硬切）；openfilm 的 CSS 转场词汇列 v2。

### 1.3 沙箱策略（三层，v1 全量落地）

现状缺口：全仓零 CSP；唯一 iframe 先例（`App.tsx:11376-11414` artifact 文档预览）**无 sandbox 属性**——既有面不动（其内容非 agent 任意代码，另行评估），**新增 film 面必须带三层**：

1. **iframe sandbox 属性**：`sandbox="allow-scripts"`——无 `allow-same-origin`（内容为 opaque origin，摸不到父页 DOM/cookie/localStorage）；
2. **页内 CSP meta（生成器模板强制）**：`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:">`——运行时一切网络被 CSP 阻断（含字体 CDN）；生成器（film capability 的页面模板）必须注入，校验器 C3 检查该 meta 存在；
3. **校验器运行时断言**：校验期间监听被拦截请求，任何非 data: 的资源请求即 fail（设计件二 §2.1 C3）。

素材只读：v1 素材=generation-time 内联（data URI），无文件系统面；v2 若开 assets/ 目录寻址，届时配专用只读静态路由+目录白名单。

## 2. MP4 导出

### 2.1 链路（Node playwright-core + ffmpeg，与校验器同栈）

```
scripts/film_export.mjs
  输入：film.json 路径 + 输出 MP4 路径 + fps（默认 30）
  1. 逐镜头：headless Chromium 开镜头页（file:// 直读，无需 agent 在线）
  2. 逐帧：page.evaluate(window.film.frame, t) → page.screenshot() → PNG
  3. PNG 序列 pipe/临时目录 → ffmpeg：h264 编码 + 整曲 WAV 音轨 mux → MP4
  4. ffprobe 校验：时长（±1 帧）、帧数、音轨存在、流信息
  退出码 0=PASS
```

- 帧驱动与播放器同契约（frame(t) 采样），导出即"离线播放"——这是纯函数契约的直接红利：**导出器不需要理解页面内容**。
- v1 不做快门采样运动模糊（openfilm 有）与 BT.709 标记——列 v2 打磨（展演若走投影仪色域再议）。
- file:// 直读：导出不依赖 agent/webui 运行（离线可跑），但**真栈烟测腿**（§2.2）要求在真实栈在位的运行里走通 agent 发起路径。

### 2.2 Windows 真栈验收（AGENTS §5 门槛）

- 新脚本 `scripts/run_film_export_smoke.ps1`（复用 ps1 体系模式，参照 `run_webui_rendered_dom_smoke.ps1` 隔离泊位+即起即拆）：起真实三件套→经 agent HTTP 面发起导出（能力链路或导出端点）→等 MP4→ffprobe 断言→退出码 0；
- 播放器 DOM 断言：扩展 `scripts/webui_rendered_dom_smoke.mjs` 断言组（E2E-WEBUI-1 门）加 film surface 组（iframe 存在/sandbox 属性/frame 调用计数/播放进度推进）；
- 真栈占用按 `coord/resources/PC-RUNTIME-STACK.md` 登记串行（§红线核对表）。

### 2.3 依赖底座（CONFIG-BOM 勘误与新建登记面）

**勘误（重要）**：卡面"playwright-core 入 CONFIG-BOM"所指 CONFIG-BOM-1 实为 UTF-8 BOM 字节序容错修复卡（`coord/runs/CONFIG-BOM-1/receipt.md`），**仓库不存在依赖 BOM 清单文件**。本设计裁定：新建 `coord/resources/DEPENDENCY-BOM.md`（与 PC-RUNTIME-STACK.md 同级同风格——共享资源底座登记面），首版内容：

```markdown
# DEPENDENCY-BOM：外部工具/库依赖登记（共享底座）
| 依赖 | 版本 | 用途 | 获取策略 | 引入卡 |
|---|---|---|---|---|
| playwright-core (Node) | ≥1.61（npx 缓存 1.61.1 在案） | film 校验器/MP4 导出/E2E-WEBUI-1 门 | 多候选目录自发现（webui_rendered_dom_smoke.mjs 机制泛化）+channel 回退 bundled→msedge→chrome | VIS-FILM-B2/C3 |
| ffmpeg | 9.0.1 full build 已装（WinGet: Gyan.Ffmppeg，PATH 外） | MP4 编码/ffprobe 校验 | PATH 优先→WinGet 已知路径候选→版本探测；不 vendor 入仓（GPL 全功能版体积/许可） | VIS-FILM-C3 |
| Python playwright | 1.62.0 已装（Python313） | 既有 e2e 脚本（非 film 线新增） | 已在案，无新增动作 | （存量） |
```

"playwright-core 入 CONFIG-BOM"的验收语义=登记入本文件；建文件动作归实施卡 B2 验收项。

## 3. 移动端 feed 封面（v2 边界记录）

- v2 目标：移动端 feed 封面挂载（J-card 活动画面，设计件一 §4 契约）+用户手改落回（overrides）。
- v2 输入依赖：FE-GUI-SHRINK 后续轮的工程列表"磁带壳陈列"设计裁定（前端仓）；overrides 编辑面设计。
- 本设计不展开 v2 实施细节，仅锁 v1/v2 边界不漂移。
