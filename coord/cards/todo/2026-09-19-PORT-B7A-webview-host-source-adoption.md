# PORT-B7A：VitWebViewHost 源码收编（Windows WebView2 宿主，从未入仓的运行时依赖）

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无（B7B 的契约前置）
- 模型分级：L2 / GLM-5.3（C++ 收编纪律 + API 契约文档化）
- **背景（2026-09-19 用户实测根因，修正 PORT_AUDIT §1.3(a)）**：Ask Vit WebUI 面板（`vit_dock/scenes/vit_dock_root.gd:2543`）运行时经 **ClassDB 字符串引用** `"VitWebViewHost"` 实例化 webview 宿主——审计当年按文件名/preload 搜索判其"休眠"系盲区；该类源码 `extension/src/vit_webview_host.{cpp,h}` 与 PC 本地 SConstruct 的 webview2 接线**从未提交**（仓内 register_types.cpp 仅注册 VitWaveformReader）。mac 上类不存在 → `_ensure_agent_webui_host()` 假 → `OS.shell_open(AGENT_WEBUI_URL)` → **Safari 兜底**（用户所见）。
- 目标：
  1. **原样收编**（AGENTS §12：作为-is 权威输入，不重构不改语义）：`extension/src/vit_webview_host.{cpp,h}` + PC 本地 SConstruct 的 webview2 守卫接线（`VIT_WITH_WEBVIEW2` 宏/WebView2.h 存在性守卫/平台链接库）→ 分支 `port/b7a-webview-host` 单 commit
  2. **Windows 构建验证**：干净 worktree 用收编后树 `scons platform=windows target=template_debug`（或既有 PC 构建口径）exit 0，产物含 VitWebViewHost 导出（符号/类注册证据）
  3. **API 契约文档**：`extension/src/vit_webview_host.md`（或头注内文档块）——逐方法/逐信号清单（对照 vit_dock_root.gd 的 has_method/has_signal 调用面：create/create_window/create_overlay_window/navigate/close/set_bounds/set_visible/focus/eval_js/post_web_message/set_user_data_subdir/debug_state/get_selected_plugin_*/vit_open_strip_silence_dialog + ready/navigation_started/page_state_changed/load_error/web_message_received/new_window_requested 等），供 B7B 实现 mac 适配
  4. 收编前核对工作树该文件与 2026-09-15 审计所记形态一致性（异常漂移→上报）
- 文件域：`extension/`（新增 cpp/h/SConstruct 接线/契约文档）；不改 Godot 工程、不改 `agent/`、`VitApp/`
- 验收标准：①分支单 commit diff=工作树原样（无重排无重写）；②Windows 构建 exit 0 + 类注册证据；③契约文档覆盖调用面全集；④审计一致性核对记录
- 停止条件：源码含不该入仓内容（凭据/密钥/绝对路径密写）→ 剥离后上报；构建失败且原因在收编源码内语义（非接线）→ blocked 上交；发现 webview host 有第二份变体（overlay/别的宿主类）→ 一并列明上报决策侧定收编范围
- 领取：
- 回执：
- 验收：
