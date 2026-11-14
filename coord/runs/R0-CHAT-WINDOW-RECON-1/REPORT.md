# R0-CHAT-WINDOW-RECON-1 · 对话窗承载与窗口生命周期只读勘察报告

- 日期：2026-11-14
- 基线：工作树 HEAD `a833a720bfcc4c304537e509247309cd13010eed`（含设计/文档未跟踪工件；本报告仅新增 `coord/runs/R0-CHAT-WINDOW-RECON-1/`）
- 方法：只读代码勘察 + 既有设计/裁定文档对照；**未启动真实栈、未改生产代码、未占窗口/端口资源**
- 结论性质：代码事实为「已证」；需要真实窗口观测的项标「待真栈观测」，不冒充验收

---

## 0. 结论摘要

1. **当前不是同一实例**：启动页对话是 Godot 的 `LauncherChatPanel`（Control）；工程内对话是 `VitDockRoot` 懒创建的 WebView2 **原生顶层窗口**（`Ask Vit WebUI`）。`change_scene_to_packed` 后 start_page 被释放，`LauncherNavBridge` 用 0.35s 定时器再调用 dock 的 `toggle_chat_window`——这是“延迟重开”，不是“同窗存活”。
2. **跨阶段存活尚未实现**：host 是 `VitDockRoot` 的成员；其析构调用 `close()` → `DestroyWindow`。把 host 移到 autoload 在结构上可行，但 main HWND 可用性、WebView2 启动时序、任务栏/焦点/最小化行为都需真栈探针。
3. **OS 能力缺口明确**：当前 detached 窗口是 `WS_OVERLAPPEDWINDOW` 无 owner 顶层窗；有原生标题栏，无 `HWND_TOPMOST`，`set_visible` 用 `SW_SHOWNA`（不激活），`focus()` 只把焦点移进 WebView，不 `SetForegroundWindow`；无 `WM_SETICON`、无 `WM_DPICHANGED`。设计 P4 的任务栏/通知/层序不能按现状宣称已具备。
4. **启动面接入面**：agent 侧已有 `/agent/launcher/greeting`、launcher 聊天流、`LauncherDirective`、recents 读取；React 侧**没有 launcher surface**（只有 full/main/workbench），也没有 launcher 桥。打开/新建/文件选择/治理确认目前全部在 Godot `start_page.gd`。
5. **建议**：目标形态选 A（React 窗启动即存在 + `AgentChatWindow` autoload 持有 + React launcher surface），B（两渲染面只做叙事连续）作为结构内退化；C（把 agent 窗移植 Godot）否决；D（独立进程顶层窗）作为 A 的宿主备选。**第一张实现卡应是 HOST-1，不是 UI 卡**。

---

## 1. 当前装配（代码事实）

### 1.1 启动页（第一面）

- 主场景是 `res://app/startup/start_page.tscn`（`project.godot:14`），Godot 主窗口 `window/size/mode=2`（最大化，`project.godot:42`），`embed_subwindows=false`（`:43`）。
- 启动页对话是 **Godot Control** `LauncherChatPanel`（`app/startup/launcher_chat_panel.gd:1-2`），不是 WebView；它发 `send_requested` / `directive_received` 信号（`:14`、`:20`）。
- 工程打开/新建、文件选择、治理确认都在 `start_page.gd`：
  - directive 消费：`_on_launcher_directive_received`（`:320-348`）；
  - 治理确认：`_launcher_governance_decision`（`:354-369`）、`LauncherGovernanceDialog`（`:450-456`）；
  - 新建/打开：`_create_new_project`（`:1180`）、`open_project_by_path`（`:1263`）；
  - 原生文件对话框：`_ensure_file_dialog` / `_on_open_project_pressed` / `_on_open_file_selected`（`:1355-1388`）；
  - 另存对话框：`_ensure_launcher_save_dialog`（`:435-447`）。
- 进入工程：`_enter_main_scene`（`:1451-1478`）`change_scene_to_packed(MAIN_WORKSPACE_SCENE)`；注释明确“切场景后本节点即释放”（`:1466-1473`）。

### 1.2 工程内对话窗（第二面）

- `VitDockRoot`（`vit_dock/scenes/vit_dock_root.gd`）在进入工程后才懒创建 WebView2：
  - `_AGENT_WEBUI_USE_STABLE_DETACHED_WEBVIEW := true`（`:61`）；
  - `_open_agent_webui`（`:3143-3174`）→ `_activate_agent_webui_host`（`:3246`）→ `_ensure_agent_webui_host`（`:3329-3375`）；
  - `create_window(_main_native_window_handle(), "Ask Vit WebUI", ...)`（`:3360-3361`、`:3364-3365`）；
  - 页面 URL `http://127.0.0.1:7878/app/`（`:53`）。
- 窗口尺寸：自然 1920×1080，按屏幕 96% / 94% 夹取（`:62-63`、`:3181-3189`）；不是设计样图的 1440×900。
- 关闭行为：`_on_agent_webui_window_close_requested`（`:4125-4131`）只 `hide()`；原生 `WM_CLOSE` 也是 `SW_HIDE`（`extension/src/vit_webview_host.cpp:280-282`）。

### 1.3 场景切换后的“带出”桥（关键证据）

`app/startup/launcher_nav_bridge.gd` 头注释：

> `start_page 在 change_scene_to_packed 后即被释放`（`:8-11`）

实现：`bring_out_chat_window_once` 用 `SceneTreeTimer` 延迟 0.35s（`:15`、`:37-42`），`bring_out_chat_now` 对当前 dock 调 `toggle_chat_window`（`:48-59`）。这是**新场景里重新开一个对话面**，不是把启动页的对话窗搬过去。

### 1.4 生命周期图

```
[Godot main window · maximized]
  start_page.tscn
    └ LauncherChatPanel (Godot Control)      ← 启动页对话，随场景释放
          │ directive_received
          ▼
    start_page._on_launcher_directive_received
          │ governance → open/new
          ▼
    change_scene_to_packed
          │ (start_page freed)
          │ 0.35s SceneTreeTimer
          ▼
  VitDockRoot (same Godot main window)
    └ VitWebViewHost (member of dock)        ← 进入工程才创建
          └ native top-level WebView2 window "Ask Vit WebUI"
                └ React /app/                ← 工程内对话
```

**实例结论**：启动页对话（Godot Control）与工程内对话（WebView2 原生窗）不是同一实例；两者之间只有 0.35s 的“重新打开”桥。

---

## 2. 六个问题

### Q1 启动页对话面板与工程内浮窗是否同一实例？

**否（已证）**：
- 启动页：`LauncherChatPanel` 是 Godot `Control`（`launcher_chat_panel.gd:1-2`）。
- 工程内：`VitWebViewHost` 原生顶层窗 + React 页（`vit_dock_root.gd:3329-3375`）。
- `LauncherNavBridge` 注释与 0.35s 延迟调用证明是重启面，不是存活（`launcher_nav_bridge.gd:8-11`、`:37-64`）。

### Q2 `change_scene_to_packed` 前后能否跨场景存活？

**现状不能（已证）；改成 autoload 结构可行但未验证（待真栈）**：
- `_agent_webui_host` 是 `VitDockRoot` 成员（`:186` 声明，`:3329` 创建）；`VitWebViewHost::~VitWebViewHost` 调 `close()`（`vit_webview_host.cpp:538-540`），`close()` 对 `child_window` 调 `DestroyWindow`（`:542-560`）。dock 被释放 → host 被释放 → 原生窗销毁。
- 启动页不持有 host；`change_scene_to_packed` 释放 start_page（`start_page.gd:1451-1478`），没有跨场景对象。
- autoload 结构可行性的代码依据：`project.godot:20-38` 有多个 autoload，`VitWebViewHost` 是 GDExtension 类；若把 host 放进 autoload（例如 `AgentChatWindow`），autoload 不随场景释放，`create_window` 的 detached 分支使用 `owner_or_parent = nullptr` 的顶层窗（`vit_webview_host.cpp:347-388`），理论上可跨场景存在。
- **未知**：autoload `_ready` 时 `DisplayServer.window_get_native_handle(MAIN_WINDOW_ID)` 是否已可用；`create(parent)` 是否依赖父窗；WebView2 在 agent 未就绪时能否先创建空白页；`SW_SHOWNA` 初显是否抢不到前台。这些必须真栈观测。

### Q3 OS 行为矩阵（代码结论）

| 能力 | 代码证据 | 现状 | 需补 |
|---|---|---|---|
| 顶层窗 / 任务栏 | detached `WS_OVERLAPPEDWINDOW`，`owner_or_parent=nullptr`（`vit_webview_host.cpp:347-388`） | 无 owner 顶层窗，通常会进任务栏/Alt-Tab；**未实测** | 实测任务栏条目、标题 |
| 原生标题栏 | `WS_OVERLAPPEDWINDOW` 含 caption/min/max/close（`:356-357`） | 原生 chrome 存在；设计自定义标题栏会叠加 | 去 chrome 或接受原生栏；需探针 |
| 最小化/召回 | 无自定义最小化；`WM_CLOSE` 只 hide（`:280-282`）；`set_visible` 用 `SW_SHOWNA`（`:576-583`） | 原生最小化可用；恢复不激活 | 自定义恢复/前台策略 |
| 焦点 | `focus()` 只 `MoveFocus` 进 WebView（`:586-591`）；`_focus_agent_webui_window` 调它（`vit_dock_root.gd:4173-4177`） | 不保证窗口到前台；DAW 可能抢焦点 | `SetForegroundWindow`/层序策略 |
| always-on-top | detached 路径不设 `HWND_TOPMOST`；`always_on_top` 只用于非 detached 的 Godot `Window`（`:3203`） | 未实现 | 探针决定 owner/topmost |
| 任务栏图标 | 扩展/宿主路径未发现 `WM_SETICON` / `SetClassLongPtr` / 图标加载 | 原生窗默认图标；P4 的 V 图标未接 | 宿主设置 AppUserModelID/图标 |
| 关闭/退出 | 原生 X → `WM_CLOSE` → hide；host 析构才 `DestroyWindow`（`:542-560`） | 关闭=隐藏，不是销毁；无“关闭应用”语义 | 明确关闭/最小化/退出的区别 |
| DPI | 仅 `GetDpiForWindow` 写 debug state（`:1273`）；无 `WM_DPICHANGED`；`WM_SIZE` 重设 controller bounds（`:277-279`） | 无显式 125%/150% 处理 | 真栈 125/150 观测 |
| 窗口尺寸 | 1920×1080 夹 96%/94%（`vit_dock_root.gd:62-63`、`:3181-3189`） | 不是 1440×900 设计尺寸 | 用宿主屏幕约束定 |
| 页面/窗口关系 | detached 路径 `_sync_agent_webui_bounds` 直接 return（`:4144-4147`）；WebView 用 `WM_SIZE` 自适应 | 顶层窗客户区自适应 | 验证多屏/缩放 |

**结论**：P4 的任务栏/最小化/通知/层序目前是**设计意图**，不是已具备能力；至少 HOST-1 要让窗口可控，P4 才可能落地。

### Q4 启动面接入面（React 发起 open/new 的缺口）

**已有 agent 面（可复用）**：
- `/agent/launcher/greeting` 路由（`agent/internal/chat/server.go:734`），`handleLauncherGreeting`（`launcher_flow.go:555`）；
- launcher 流门：`context.launcher`（`launcher_flow.go:36`、`:62-72`），确定性意图/recents/结果卡（`:455-552`）；
- `LauncherDirective` schema：`action` / `file_path` / `project_name` / `reason` / `requires_governance`（`:53-58`）；
- recents 读 kernel `get_recent_projects`（`:370-391`），结果卡（`:407-552`）。

**React 侧缺口**：
- 只有 `AppSurface = "full" | "main" | "workbench"`（`agent/webui/src/App.tsx:196`、`:262-264`、`:1841-1847`）；**没有 launcher surface**，也没有项目抽屉/最近工程。
- 桥只有 browser / external browser / agent mutations 三通道（`App.tsx:210-213`、`:8087-8111`）；**没有 launcher 通道**。
- Godot 的 `_on_agent_webui_web_message` 只路由 browser/external/mutations（`vit_dock_root.gd:3409-3448`），没有 launcher directive 分支。

**Godot 侧执行面（当前唯一）**：
- `start_page.gd:320-348` 消费 directive → 治理确认 → `_create_new_project` / `open_project_by_path`；
- 原生 `FileDialog`（`:1355-1388`）、`LauncherGovernanceDialog`（`:450-456`）、另存 `FileDialog`（`:435-447`）；
- `LauncherNavBridge.parse_launcher_directive` 是静态纯函数（`launcher_nav_bridge.gd:21-33`），可被新宿主复用。

**关键缺口**：进入 dock 后 `start_page` 已释放，当前**没有**“在工程内由 React 发起跨工程 open/new”的 handler；`LauncherNavBridge.bring_out_chat_now` 只 `toggle_chat_window`（`:48-59`）。A 方案必须新增 dock 侧 launcher 桥，或把 open/new 改为内核/agent 事件面。

### Q5 方案矩阵

| 方案 | 形态 | 优点 | 代价/风险 | 结论 |
|---|---|---|---|---|
| **A** | React 窗启动即创建；`AgentChatWindow` autoload 持有 host；React 增 launcher surface；Godot 启动页降为 boot/兜底；React→Godot launcher 桥 | 一窗贯穿、一套 v5 设计、符合 11-11 React 对话窗裁定；与 332“三态一窗”目标一致 | 新 React launcher UI；autoload host 生命周期；任务栏/焦点/层序；原生对话框桥；启动页隐藏/兜底 | **推荐目标** |
| **B** | 保留 Godot 启动页 + 工程内 React 浮窗；只做 0.35s 叙事连续（现状） | 零重写；最低风险；可立即交付 | P1 不是 v5 对话窗；两套设计语言；用户“启动页=对话窗”的叙事不成立 | **结构内退化/兜底** |
| **C** | 把 agent 窗移植 Godot | 同进程/同窗口 | 11k 行 React + 设计系统/交互契约重写；审计体系重建；展演前不可行 | **否决** |
| **D** | 启动时由 Godot 拉起独立进程/独立顶层 WebView 窗（不依赖 dock host） | 场景生命周期简单；任务栏/窗口控制走 OS | 双进程通信/窗口管理/单实例；仍需 React launcher 与桥 | **A 的宿主备选** |

**建议**：A 为目标，B 为结构内退化；先做 HOST-1 风险探针，再决定 A/D 宿主实现；C 不做。

### Q6 第一张实现卡建议

**HOST-1（P0，先于 UI 卡）**：`AgentChatWindow` autoload + 启动即创建 React 顶层窗 + 跨 start page→dock 存活；feature flag 包裹；不改聊天行为；B 兜底。

- 文件域：`project.godot`（autoload 注册）+ 新 `app/shell/agent_chat_window.gd`（或 `app/kernel/autoloads/`）+ `vit_dock_root.gd` 的 host 创建/可见性/焦点抽取 + `start_page.gd` 隐藏/兜底钩子；扩展仅在需要 chrome/icon/topmost 时动 `extension/src/vit_webview_host.cpp`。
- 验收：真实栈按 AGENTS §5 单 owner；同一 HWND/同一页面跨 `change_scene_to_packed`；任务栏/最小化/召回/焦点/DPI 125% 记录；exit 0；B 回退可用。
- **不要**在同卡夹带 React launcher UI；`LAUNCHER-UI-1` 可并行（WebUI 文件域不相交），`LAUNCHER-BRIDGE-1` 依赖 HOST-1 的宿主信号。

---

## 3. 证据锚点表

| 事实 | 锚点 |
|---|---|
| 主场景/最大化主窗 | `project.godot:14`、`:42-43` |
| 启动页对话是 Godot Control | `app/startup/launcher_chat_panel.gd:1-2`、`:14`、`:20` |
| start_page 切场景即释放 | `app/startup/start_page.gd:1451-1478`（`:1466-1473` 注释） |
| 0.35s 重新带出对话窗 | `app/startup/launcher_nav_bridge.gd:8-11`、`:15`、`:37-64` |
| directive→治理→open/new | `start_page.gd:320-348`、`:354-369`、`:1180`、`:1263` |
| 原生文件/治理/另存对话框 | `start_page.gd:1355-1388`、`:450-456`、`:435-447` |
| 工程内 host 懒创建 | `vit_dock_root.gd:61`、`:3143-3174`、`:3246`、`:3329-3375` |
| host 是 dock 成员、析构销毁窗 | `vit_dock_root.gd:186`；`extension/src/vit_webview_host.cpp:538-560` |
| detached 顶层窗样式 | `extension/src/vit_webview_host.cpp:347-388` |
| 关闭=hide、WM_CLOSE | `vit_webview_host.cpp:280-282`；`vit_dock_root.gd:4125-4131` |
| 显示不激活/焦点只进 WebView | `vit_webview_host.cpp:576-591`；`vit_dock_root.gd:4173-4177` |
| 无 topmost/图标/DPI 处理 | `vit_webview_host.cpp:1273`（仅 debug DPI）；扩展/宿主路径未发现 `WM_SETICON`/`WM_DPICHANGED` 处理 |
| React surface 无 launcher | `agent/webui/src/App.tsx:196`、`:262-264`、`:1841-1847` |
| React 桥三通道 | `App.tsx:210-213`、`:8087-8111` |
| launcher greeting/flow/directive | `agent/internal/chat/server.go:734`；`launcher_flow.go:36`、`:53-58`、`:71`、`:455-552`、`:555` |
| 窗口尺寸 | `vit_dock_root.gd:62-63`、`:3181-3189` |

---

## 4. 仍未知（需真栈观测，不在本卡内做）

1. autoload `_ready` 时 main HWND 是否非 0；WebView2 `create` 是否可在 agent 未就绪时先建空窗。
2. 同一 HWND/页面能否跨 `change_scene_to_packed` 存活（autoload 方案）；若不能，D 方案的进程/窗口边界。
3. detached 顶层窗的实际任务栏条目、Alt-Tab、最小化/召回、焦点抢占、DAW 弹出时是否抢输入。
4. 原生标题栏与设计自定义标题栏的取舍；去 chrome 后任务栏/拖拽/最小化是否仍成立。
5. 125%/150% DPI 下窗口尺寸、字号、图标、WebView 内容缩放。
6. start page 隐藏后，Godot `FileDialog`/治理对话框是否仍能正常弹出并归属到正确窗口。
7. `start_page` 释放后，跨工程 open/new 由谁执行（dock 侧 launcher 桥 vs 内核/agent 事件）。

---

## 5. 只读核对

- 本报告仅新增 `coord/runs/R0-CHAT-WINDOW-RECON-1/`；未修改生产源码、tscn、协议、样式或雪碧图。
- 未启动 VitApp / Godot / VitAgent；未占用端口/真栈；未向其它会话投递。
- `git status` 中本卡相关新增仅本目录（设计目录与 R0 卡为既有未跟踪工件，不在本报告改动范围）。
