# PORT-B7B：Ask Vit WebUI mac 内嵌（VitCefWebViewHost 适配层，替换 Safari 兜底）

- 优先级 / 预估 / 依赖：P1 / 1 天 / 依赖 B7A（API 契约文档）
- 模型分级：L3 / GLM-5.3（跨宿主适配语义 + CEF 纹理集成；关键点可找参谋讨论）
- 目标：Ask Vit WebUI 在 mac 以**内嵌窗口**呈现（CEF），彻底替换 Safari 兜底路径：
  1. **平台分支**：`vit_dock/scenes/vit_dock_root.gd` `_ensure_agent_webui_host()`——`OS.get_name()=="macOS"` → 实例化 `VitCefWebViewHost`（新 GDScript 适配类），Windows → `VitWebViewHost` 原样（B3 平台分支同款纪律，Windows 臂逐字保留）
  2. **适配类**（前端仓新增 `vit_dock/scenes/vit_cef_web_view_host.gd`，`class_name VitCefWebViewHost`）：以 godot_cef `CefTexture`（B2P 手册 `docs/CEF_MAC_PROBE_2026-09.md`）实现 B7A 契约面——**功能性子集**：`create_window(parent,title,w,h)`（Godot Window+CefTexture 内构，parent 句柄仅用于居中/忽略并注明）、`create`、`navigate`、`close`、`set_bounds`、`set_visible`、`focus`、`eval_js`、`post_web_message`、`set_user_data_subdir`、`debug_state` + 信号 `ready/navigation_started/page_state_changed/load_error/web_message_received`；**降级子集**（overlay/插件编辑器面：`create_overlay_window/get_selected_plugin_*/vit_open_strip_silence_dialog`）：v1 显式 no-op+warning，回执与端测边界**显式声明降级清单**
  3. webui 交互闭环：web_message 通道（mutation/browser/external-browser）经适配层透传不断链；`AGENT_WEBUI_URL` 页面加载、会话往返（journey S3 面已证 agent 侧就绪）
  4. 验证：`godot --headless --check-only` 解析 exit 0 + headless 适配类实例化冒烟（可 headless 的面）+ **`[等待真栈验收]` 用户手测**（内嵌窗口出现、页面渲染、对话往返、关窗干净退出无 CEF 残留——清单交决策侧转用户）
- 文件域：前端仓（mac `~/Documents/vit-daw-frontend`，git 化）`vit_dock/scenes/vit_dock_root.gd` + 新增 `vit_dock/scenes/vit_cef_web_view_host.gd`；不改主仓（证据=前端仓分支 commit，决策侧在 mac 直接可审）
- 验收标准：①平台分支 diff（Windows 臂逐字保留）；②适配类实现清单 vs B7A 契约逐项对照（功能/降级标注）；③check-only + headless 冒烟 exit 0；④真栈手测清单成品+`[等待真栈验收]`；⑤前端仓分支与 commit hash 回执
- 停止条件：契约面有 webui 主链必需但 CefTexture 无法等价实现的能力 → 证据上交裁定口径；CEF 适配在真实前端工程实例化崩溃 → 按 B2P 手册 §4 排查仍败转 blocked；需改 agent/内核配合 → 域外上报
- 领取：2026-09-19 10:38 / origin/main=d117dcb1aad69982726eb55379cf4f66e4c93530（B7A 已验收合入：ruling 2026-09-19-B7A-pass + 契约文档 extension/src/vit_webview_host.md 在 main，前置满足）/ 分支 port/b7b-cef-webview-host（前端仓 ~/Documents/vit-daw-frontend，本地仓无远端）
- 回执：
- 验收：
