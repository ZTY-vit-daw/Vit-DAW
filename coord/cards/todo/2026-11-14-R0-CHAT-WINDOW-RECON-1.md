# R0-CHAT-WINDOW-RECON-1：对话窗承载与窗口生命周期只读勘察

- 发卡：主设计会话（lead）/ 2026-11-14（依据=用户 v5.1 设计定稿与「先写 R0 探针卡」指令）
- 派发确认：**待用户 / 主管确认**（设计会话起草，未自动派发；未确认前不领取）
- 验收负责人：用户 + 主管决策流（辅助复核可另列）
- 池序：共享池候补（292+ 下一可用号，由主管登记；本卡不自行占号）
- 优先级 / 预估 / 依赖：P0 / 0.5–1 天 / 依赖 v5.1 设计交接与图标定稿；**零生产代码改动**
- 机型与执行流：PC 只读勘察；任何模型可执行，但不得占用真实栈 owner

## 背景（为什么现在做）

v5.1 设计已定稿（`agent/webui/design/main-window-v5/`，图标 23 symbol、S3/P3 已重建、`v51-icons-8` PASS）。生产端现状与设计叙事之间还有一处未决：

- 启动页是 Godot 场景（`app/startup/start_page.gd`，内含 `LauncherChatPanel`）；
- 工程内对话窗是 `VitDockRoot` 后来创建的 **WebView2 detached top-level 窗口**（`vit_dock/scenes/vit_dock_root.gd`，`_AGENT_WEBUI_USE_STABLE_DETACHED_WEBVIEW=true`）；
- 两者**目前不是同一实例**。332 设计主张「三态一窗」、11-11 裁定主张「React/WebView 对话窗 + Godot DAW 双 OS 窗口」，并明确要求实施前做「窗口实例跨阶段存活」探针。

本卡只回答「真实装配是什么、能不能一窗贯穿、代价在哪」，**不实现、不改生产代码、不启动真栈**。

## 设计侧预勘察（非领取，不替代执行回执）

- 2026-11-14 设计会话已做只读预勘察：`coord/runs/R0-CHAT-WINDOW-RECON-1/REPORT.md`。结论供领取者复核；真栈未知项见报告 §4，不替代正式领取、回执与验收。
- 预勘察未改生产代码、未启动真实栈；执行侧领取后仍须按卡面独立取证。

## 目标（可验收的事实）

1. **装配图**：以 file:line 锚点画出启动页 → 工程 dock 的窗口/实例生命周期；标明 React 对话窗在何时、由谁、以什么窗口样式创建（`WS_OVERLAPPEDWINDOW` / owner / taskbar / 场景归属）。
2. **实例存活结论**：`change_scene_to_packed` 前后，WebView2 host/HWND 是否被销毁；若把 host 移到 autoload（例如 `AgentChatWindow` 单例）能否跨场景存活；列出需要改动的具体锚点与最小验证方式。明确回答：启动页对话面板与工程内浮窗是否同一实例（预期：否）。
3. **OS 行为矩阵**：任务栏条目、最小化/召回、Alt-Tab、always-on-top、DAW 弹出时的焦点抢占、原生标题栏 vs 自定义 chrome、125%/150% DPI；区分「现已具备 / 需宿主补 / 不可做」。
4. **启动面接入面**：`/agent/launcher/greeting`、launcher chat/directive、`web_message_received` / `post_web_message` / `ask_vit_*` 通道；React 发起 open/new/选择文件/治理确认时，哪些可复用、哪些要新增宿主桥；原生文件对话框与治理确认当前归属。
5. **方案矩阵**：至少比较 A（React 窗启动即存在 + autoload 跨场景）、B（保留两个渲染面只做叙事连续）、C（把 agent 窗移植到 Godot，成本/风险）、D（独立进程/独立顶层窗口）；给出推荐、代价、回退与双轨保险。
6. **实施切分建议**：对应设计交接 A–D 切片，给出第一张最小实现卡的文件域、依赖与验收门槛；指出哪些必须等窗口探针结论。

## 文件域（只读）

- 只读：`D:/Godot/project/vit-daw-frontend/`（startup / vit_dock / extension）、`agent/webui/src/`、`agent/internal/chat/launcher_flow.go`、`agent/internal/chat/server.go` 相关端点、`agent/webui/design/main-window-v5/` 交接文档、`coord/` 两份留存裁定。
- 唯一可写：`coord/runs/R0-CHAT-WINDOW-RECON-1/`（REPORT.md + 必要只读摘录/截图/命令输出）。
- 禁止修改任何生产源码、tscn、样式、协议或雪碧图；禁止 `git reset/clean/checkout` 丢弃改动。

## 并行与资源

- 不需要真实栈；若必须观察窗口行为才能定论，**停止并上报**，由主管登记 `coord/resources/PC-RUNTIME-STACK.md` 后另开探针腿，不在本卡内抢栈。
- 不产生后台调度、不自动向其它会话投递。

## 验收标准

- `coord/runs/R0-CHAT-WINDOW-RECON-1/REPORT.md` 存在，六个目标逐项有结论；每条结论带 file:line 或端点/命令证据，无法确认的写「未知 + 需要的证据」。
- 附一张窗口生命周期图 / 表（实例、HWND、owner、场景归属、销毁时机）。
- 附方案矩阵（A–D）与推荐；明确「窗口实例是否同一」「autoload 存活是否可行」「OS 行为缺口」三问。
- `git status --short` 仅显示本报告目录新增；无生产 diff。
- 命令与退出码记录在报告；不得把只读推断记成真栈验收。

## 停止条件

- 需要修改生产代码、tscn、协议或样式才能继续 → 停，交回证据与最小改动点。
- 需要启动/占用真实栈（Godot + 内核 + agent）→ 停，登记资源后另开探针腿；不得并行占用。
- 发现设计交接与真实装配矛盾（例如双窗口 vs 单窗、启动页归属）→ 记录矛盾，不自行裁定产品方向。

## 领取

- 时间 / origin/main hash / owner 模型+机器+会话 / 分支 / worktree / 领取提交：**待回填**

## 回执

- commit hash / 报告链接 / 端测覆盖边界声明：**待回填**

## 验收

- 裁定文件 / 验收 commit：**待回填**
