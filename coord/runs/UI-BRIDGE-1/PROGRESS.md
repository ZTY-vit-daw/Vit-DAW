# UI-BRIDGE-1 执行进度（断点续跑协议）

- 卡：coord/cards/doing/2026-11-14-UI-BRIDGE-1.md（池序 348，P0/P1，v5.1 链尾关键卡）
- owner：freeflash（GLM-5.3 Flash）免费车道子代理 · PC · ZCode 主管会话派发
- 分支/worktree：port/ui-bridge-1 @ D:/Vit_DAW_wt_uibridge1（主仓）；port/ui-bridge-1 @ D:/Godot/vit-daw-frontend_wt_uibridge1（前端仓，自 128477b）
- 领取提交：0655272d（coord-only → main，远端 owner 已核对）

## 设计定案（勘察结论，中断恢复读这段）

**桥契约（channel `ask_vit_launcher`）**
- React→Godot（chrome.webview.postMessage）：`{channel:"ask_vit_launcher", type:..., ...}`
  - `launcher_surface_ready` {surface:"launcher"}（桥在卷即发，宿主日志留痕）
  - `launcher_directive` {request_id, directive:{action,file_path,project_name,reason,requires_governance}}
  - `launcher_choose_file` {request_id}
- Godot→React（host.post_web_message，JSON）：`{type:"vit_launcher_result", request_id, status: opened|failed|canceled|unsupported|invalid|dialog_shown, action, project_name, file_path, scene, governance, message}`
- smoke 附加（URL 参数 launcher_smoke=1 才发）：`launcher_result_ack` {request_id,status}（React 真实收到 result 后回执宿主日志，取证通道非状态伪造）

**Godot 路由（343 落地的既有转发面，零 autoload 改动）**
- autoload `_on_host_web_message`（agent_chat_window.gd）：vit_os_notify 拦截 → 其余转发 `current_scene._on_agent_webui_web_message(message)`（有该方法才转）
- start_page.gd 新增 `_on_agent_webui_web_message`：启动页阶段消息不再被丢弃，start_page 只处理 ask_vit_launcher，其余忽略（=今天 drop 行为不变）
- dock `_on_agent_webui_web_message`（vit_dock_root.gd:3478）加 ask_vit_launcher 分支 → dock 侧 handler：open/new/choose_file 一律回 unsupported 明确降级（dock 无 open 链，卡面允许明确降级）
- extension cpp 零改动（web_message 信号原样转发字符串，无需新路由）

**start_page 桥 handler**
- `_handle_launcher_bridge_message(payload)`：
  - surface_ready → `[LauncherBridge] surface_ready` 日志
  - directive → request_id → LauncherNavBridge.parse_launcher_directive 校验 → invalid 回执；忙（_launcher_directive_in_flight/_hub_busy）→ failed 回执带原因；治理 `_launcher_governance_decision()`（复用 336 链，记录 choice 到 `_launcher_bridge_governance_choice`）→ cancel 回执 canceled；open/new 走既有公共链 → 结果回执
  - choose_file → `_on_open_project_pressed()`（原生 FileDialog）→ 立即回 dialog_shown（人工选择域，选择后 open 链结果不再桥接回执=边界声明）
- 进入结果判定：`_launcher_bridge_enter_ok` flag（_enter_main_scene 成功 change_scene 置 true；_finish_ipc_failure 置 false+detail）——handler 执行前清零，await 后读
- 结果回投经 `LauncherNavBridge.post_launcher_result(tree, result)`（新静态函数：AgentChatWindow borrow_host → post_web_message；SceneTree 存活跨场景切换）

**React 侧**
- 新 src/launcher/launcherBridge.ts：channel 常量/isLauncherBridgeAvailable/postLauncherSurfaceReady/postLauncherDirective/postLauncherChooseFile/postLauncherResultAck/subscribeLauncherResults/requestID
- launcherData.ts：directivePlaceholder note 文案更新（保持「不执行」子串过 render smoke：fixture/gallery 仍是纯占位）；新增桥回执文案 helper（waiting/opened/failed/canceled/unsupported 全 honest，不伪造已打开）
- LauncherSurface.tsx：
  - thread directive item 携带原始 directive；执行按钮（live+桥可用）→ postLauncherDirective → state waiting → result listener 更新 opened/failed/canceled/unsupported + 重试
  - 桥不可用（浏览器 dev）：按钮禁用 + honest 说明（桥只在 Vit 窗口内可用）
  - 烟测钩子：URL launcher_smoke=1 + launcher_smoke_message（可 `||` 分隔多条，result 后 launcher_smoke_followup_delay_ms 发下一条）→ greeting ready 自动 send → directive 自动执行 → result ack 回投
- App.tsx：零改动（bridge client 在 src/launcher/**，345 自含模式）

**烟测 ui_bridge_smoke.ps1（343/349 同模式新脚本）**
- 一次运行 4 boots，内核+agent 单次启动跨全部腿；recents 种子（甲/乙=912.vit 两个不同名副本）snapshot→prepend→restore
- boot1 零摩擦腿：无 auto_open → React 桥「打开烟测桥甲」→ 治理 none → open → VitDockRoot → React result=opened（result_ack 断言）
- boot2 switch 腿：kernel 有甲 → 「打开烟测桥乙」→ 三键对话框出现 → 治理 switch → 乙 open
- boot3 cancel 腿：kernel 有乙 → 「打开烟测桥甲」→ cancel → 无场景切换 + /agent/state current_project 仍乙 + React result=canceled
- boot4 save+降级腿：kernel 有乙 → 「打开烟测桥甲」→ 治理 save（真实 save_project）→ 甲 open → dock；followup（+8s）第二条「打开烟测桥乙」→ dock handler → unsupported 降级回执 + React 呈现
- 治理三键驱动：VIT_SMOKE_LAUNCHER_GOVERNANCE=save|switch|cancel → start_page 烟测钩子在对话框可见时经 AcceptDialog custom_action/canceled 信号路径（与真实点击同一 handler 链）
- 断言面：[LauncherBridge] 日志行族 + scene_added VitDockRoot + HWND 不变量（boot 内）+ /agent/state current_project + result_ack（React 真收）+ fixture 源哈希前后一致 + recents 还原
- 零 LLM（launcher 聊天确定性路由）

## 步骤

- [x] S0 勘察（卡面/PROTOCOL §2.2/R0 §Q4/340 RUNBOOK/343+349 RUN-SUMMARY/桥面现状/launcher_flow 意图面）
- [x] S1 fetch 双仓（主 f05a0351/前端 128477b）+ doing 域核对 + 双 worktree + npm install（exit 0）
- [x] S2 领取提交 0655272d → main（远端 owner 核对 ✓）
- [x] S2b RUNBOOK + 本 PROGRESS 落盘
- [x] M1 React：launcherBridge.ts + launcherData 文案 + LauncherSurface 执行/回执/烟测钩子 + 单测 8 例（537 全过）+ build + render 三面 PASS（主仓 commit 见 git log）
- [x] M2/M3 Godot（前端仓 commit 3406c4f）：start_page 桥 handler + 治理 choice + enter 标记 + 烟测钩子 + nav_bridge post_launcher_result + dock 降级 handler；extension cpp 零改动——dll 为 git 跟踪工件免重编（worktree checkout 自带双变体 dll）；--import 预跑零 SCRIPT ERROR（CEF addon 缺失=既有环境面）
- [x] M4 webui 门槛（并入 M1：npm test 537 过/build 过/test:render 三面 PASS）
- [ ] M5 ui_bridge_smoke.ps1 编写（scripts/，主仓 worktree）
- [ ] M6 真栈：占用登记→≤60s 复核→run（4 boots）→拆栈→释放记录
- [ ] M7 卡面回执 + doing→done + coord 推送 + 双仓 port 分支推送

## 环境备注（2026-11-14 20:3x 发现）

- D:/Vit_DAW/godot-cpp 与 D:/Godot/project/vit-daw-frontend/godot-cpp 均为空实体目录（UI-OS-1/HOST-1 worktree 已被清理，其 junction 消失）——本次 extension 无需重编（cpp 零改动+dll 在 git），junction 已删；若后续卡需重编 extension，真实源可从 D:/备份/vit-daw-frontend/godot-cpp（488MB，branch 4.2+预生成 gen/）恢复到 D:/Vit_DAW/godot-cpp，版本相符性须先核对（上交主管留意）。

## 断了几次 / 中断恢复

- 尚未中断。恢复指引：读本文件「设计定案」+ 卡面；git -C D:/Vit_DAW_wt_uibridge1 log --oneline 看进度；步骤勾选状态即断点。
