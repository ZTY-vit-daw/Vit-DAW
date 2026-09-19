# VitWebViewHost API 契约（PORT-B7A）

> 供 PORT-B7B（mac 适配层 VitCefWebViewHost）对齐行为。本文档对照
> `extension/src/vit_webview_host.{h,cpp}`（Windows WebView2 实现，原样收编自
> 前端仓工作树）与 `vit_dock/scenes/vit_dock_root.gd`、`app/browser/browser_panel.gd`、
> `app/shell/side_panel/artifact_panel.gd` 的全部调用面整理。
> 基类 `RefCounted`，ClassDB 注册名 `"VitWebViewHost"`，SCENE 初始化级别注册。

## 0. 审计一致性核对（卡面目标 4）

`docs/PORT_AUDIT_2026-09.md` §1.3(a)（69-72 行）判定该源码"在 .gd/.tscn 中零引用、休眠代码"。
复核结论：**该判定系检索盲区**。四个消费方均经 `ClassDB.class_exists("VitWebViewHost")` +
`ClassDB.instantiate("VitWebViewHost")` 字符串引用实例化，文件名/preload 搜索不可见：

- `vit_dock/scenes/vit_dock_root.gd:2545`（Ask Vit WebUI，`_agent_webui_host`）
- `vit_dock/scenes/vit_dock_root.gd:3117`（Ask Vit Browser，`_agent_browser_host`）
- `app/browser/browser_panel.gd:437`
- `app/shell/side_panel/artifact_panel.gd:899`

源文件与前端仓 git HEAD 一致（收编时 sha256 比对通过，前端仓工作树该文件无未提交改动），
形态与审计所记无漂移——漂移的是审计的引用判定，不是源码。

## 1. 方法清单（21 个，_bind_methods 全集）

所有方法在 gd 侧一律经 `has_method(...)` 守卫后 `call(...)` 调用（动态分发）。
未就绪（`ready_` 为假）时的统一错误路径：emit `load_error` 信号，message 为
`last_error_`（空则 `"WebView2 host is not ready."`）。

### 创建族（5 个）

| 方法 | 签名 | 语义 |
|---|---|---|
| `create` | `(parent_hwnd: int) -> bool` | 子窗口内嵌宿主。hwnd 为 0 时从 `DisplayServer.window_get_native_handle(WINDOW_HANDLE, 0)` 取主窗口。同步阻塞初始化（内部泵消息循环，超时 10s）。已 ready 时直接 emit `ready` 返回 true（幂等）。 |
| `create_window` | `(parent_hwnd: int, title: String, width: int, height: int) -> bool` | 独立 detached 窗口（WS_OVERLAPPEDWINDOW）。width/height 下限 640×420，title 空时缺省 `"Vit Browser"`。已 ready 时 set_visible(true)+emit `ready` 返回 true。 |
| `create_overlay_window` | `(parent_hwnd: int, title: String, width: int, height: int) -> bool` | 无边框 WS_POPUP + WS_EX_TOOLWINDOW 覆盖窗，bounds 按 parent 客户区 ClientToScreen 换算。width/height 下限 320×240，title 缺省 `"Vit Browser Overlay"`。 |
| `create_composition` | `(parent_hwnd: int) -> bool` | DirectComposition 模式（ICoreWebView2CompositionController + dcomp visual 树），bounds 恒 zero-origin，初始 800×600。 |
| `create_probe` | `(parent_hwnd: int) -> bool` | 不创建 WebView2 控件的 HWND 探针（纯色 #0F766E 测试窗），同步 emit `ready`。 |

### 生命周期与几何（7 个）

| 方法 | 签名 | 语义 |
|---|---|---|
| `close` | `() -> void` | 销毁 controller/子窗口/COM，重置全部状态。析构函数调用它。close 后实例可重新 create（gd 侧 `_close_agent_webui_host` 置空引用重建）。 |
| `set_bounds` | `(x, y, width, height: int) -> void` | parent 客户区坐标。width/height 下限 1。 |
| `set_screen_bounds` | `(x, y, width, height: int) -> void` | 屏幕坐标（内部 ScreenToClient 换算）；detached 窗口直接 SetWindowPos。 |
| `set_visible` | `(visible: bool) -> void` | 同步 controller put_IsVisible 与 ShowWindow。 |
| `focus` | `() -> void` | MoveFocus(PROGRAMMATIC)。未 ready 静默返回。 |
| `send_mouse_input` | `(event_kind: String, virtual_keys: int, mouse_data: int, x: int, y: int) -> bool` | **仅 composition 模式**（ICoreWebView2CompositionController::SendMouseInput）。event_kind 枚举字符串：`move`/`leave`/`wheel`/`horizontal_wheel`/`left_button_down`/`left_button_up`/`left_button_double_click`/`right_button_down`/`right_button_up`/`right_button_double_click`/`middle_button_down`/`middle_button_up`/`middle_button_double_click`。坐标下限 clamp 0。 |
| `set_user_data_subdir` | `(subdir: String) -> void` | **仅 create 前有效**（ready 后静默忽略）。数据目录落在 `%LOCALAPPDATA%\VitDAW\WebView2\Profiles\<sanitized>`；sanitize：仅 `[A-Za-z0-9_-]`，其余折叠为 `_`，截断 96 字符，空结果取 `"default"`。 |

### 导航与脚本（7 个）

| 方法 | 签名 | 语义 |
|---|---|---|
| `navigate` | `(url: String) -> void` | 成功后 emit `navigation_started(url)`；未 ready/失败 emit `load_error`。 |
| `navigate_html` | `(html: String) -> void` | NavigateToString；current_url_ 置 `vit://embedded-webview-test`。 |
| `go_back` / `go_forward` | `() -> void` | 可退/进才执行（get_CanGoBack/Forward）。 |
| `reload` / `stop` | `() -> void` | stop 额外 emit 一次 `page_state_changed`（loading=false）。 |
| `eval_js` | `(script: String) -> Variant` | 同步执行（泵循环超时 5s）。**返回值为 JSON 字符串经最小 unescape 的 String**（不是解析后的 Variant）——消费方自行 `JSON.parse_string`。超时/失败返回 null 并 emit `load_error`。 |
| `post_web_message` | `(message: String) -> bool` | 优先 PostWebMessageAsJson，失败退 PostWebMessageAsString。 |
| `debug_state` | `() -> Dictionary` | 只读诊断快照（键见 §4）。stub 构建额外含 `linked: false`。 |

## 2. 信号清单（5 个）

| 信号 | 参数 | 触发时机 |
|---|---|---|
| `ready` | — | controller + webview 创建成功后（create 族完成回调内）。 |
| `navigation_started` | `url: String` | Navigate/NavigateToString 调用成功时、WebView2 NavigationStarting 事件。 |
| `page_state_changed` | `state: Dictionary` | NavigationStarting（loading=true）、NavigationCompleted、DocumentTitleChanged、stop()。键：`url`、`title`（可得时）、`loading: bool`、`can_go_back: bool`、`can_go_forward: bool`。 |
| `load_error` | `message: String` | 一切失败路径（未就绪调用、HRESULT 失败、超时）。message 为人类可读错误串（含 HRESULT 十六进制）。 |
| `web_message_received` | `message: String` | WebMessageReceived；message 为 **JSON 字符串**（消费方 `JSON.parse_string` 后处理）。 |

## 3. gd 侧消费面（117 处 has_method/has_signal 的分界）

vit_dock_root.gd 全文 117 处 `has_method`/`has_signal` 调用中，落在 VitWebViewHost 实例上的守卫共 19 处
（vit_dock_root 两实例 16 处，其余在 browser_panel / artifact_panel）。四个消费方合议的**实际消费全集**：

- **vit_dock_root `_agent_webui_host`**（Ask Vit WebUI）：`create`/`create_window`/`navigate`/`focus`/`post_web_message`/`set_bounds`/`set_visible`/`debug_state`/`close` + 5 信号全接。创建路径：`create_window(主窗口hwnd, "Ask Vit WebUI", w, h)` 优先，退 `create(parent_hwnd)`，再退 `create_window`。
- **vit_dock_root `_agent_browser_host`**（Ask Vit Browser）：`set_user_data_subdir` → 5 信号接 4（无 web_message_received）→ `create_overlay_window` 优先，退 `create_window`，退 `create`；另用 `navigate`/`focus`/`set_screen_bounds`/`set_bounds`/`set_visible`/`eval_js`/`close` 及动态分发 `go_back`/`go_forward`/`reload`/`stop`/`focus`。
- **browser_panel `_host`**（Vit 浏览器）：`create_probe`（实验开关）→ `create_composition`（实验开关）→ `create_window` → `create` 逐级回退；`navigate`/`navigate_html`/`eval_js`/`set_bounds`/`set_screen_bounds`/`set_visible`/`focus`/`send_mouse_input`（composition 模式鼠标）/`debug_state`/`close` + 动态分发 `go_back`/`go_forward`/`reload`/`stop`。
- **artifact_panel `_webview_host`**（Vit 预览）：`create_window`/`create`/`navigate`/`set_bounds`/`set_screen_bounds`/`set_visible` + 信号 `ready`/`load_error`。

**不在本类契约内的调用面**（117 处中其余，防 B7B 误实现）：

- `get_selected_plugin_id/name/track_id/source`：目标为 `_find_agent_plugin_context_host()` 找到的 UI 面板节点（webui 内嵌页的宿主面板），非 VitWebViewHost 方法。
- `vit_open_strip_silence_dialog`：timeline 树上递归查找的宿主节点方法。
- `new_window_requested`：`_app_header`（app_header_shell.gd）的信号，非本类信号。卡面目标 3 清单中此项经核实不属于 VitWebViewHost。

## 4. debug_state 键域

恒有：`ready`、`visible`、`last_error`、`current_url`。
WebView2 后端附加：`parent_hwnd`、`child_hwnd`、`composition_mode`、`detached_window`、`overlay_window`、`user_data_subdir`、`controller`、`composition_controller`、`dcomp_device`、`dcomp_target`、`dcomp_root_visual`、`dcomp_webview_visual`、`webview`、`bounds{left,top,right,bottom}`、`controller_bounds{...}`；有子窗口时再附 `child_rect{...,visible,parent,probe_color}`、`child_client_rect{...}`、`child_dpi`。
stub 后端附加：`linked: false`（mac 兜底实现可据此区分后端）。

## 5. 构建守卫与 stub 语义（B7B 关键参照）

- 编译开关：`#if defined(_WIN32) && defined(VIT_WITH_WEBVIEW2)`。`VIT_WITH_WEBVIEW2` 由 SConstruct 在 `third_party/webview2/include/WebView2.h` 存在时定义（干净树守卫：SDK 头不入仓则整段退 stub）。
- stub 后端行为：构造/析构平凡；`create` 族一律返回 false 并 emit `load_error("VitWebViewHost native WebView2 backend is not linked in this build.")`；`navigate` 未 ready 时 emit `load_error`（ready 后 emit navigation_started/page_state_changed 假状态）；`debug_state` 含 `linked: false`。**gd 侧所有调用面在 stub 下均有定义（不崩溃）**——B7B 的 mac 适配层只需在同一 ClassDB 名下提供本契约方法/信号全集即可全链路替换。
- WebView2Loader.dll 动态加载：模块目录 → 上级 `third_party\webview2\` → PATH。找不到时 create 失败并 emit `load_error`。
- 运行时调试开关：环境变量 `VIT_WEBVIEW_DEBUG=1/t/T` 开 stderr 日志（`[VitWebViewHost]` 前缀）。
- 初始化为**同步阻塞**语义（泵消息循环等待回调，create 10s / eval_js 5s 超时）——gd 侧按同步返回值分支，mac 实现若异步化需保持返回值语义兼容或在 B7B 卡内显式分界。
