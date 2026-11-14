# UI-ENTRY-2 PROGRESS（断点续跑）

- 卡：UI-ENTRY-2（池序 355，P0）启动页门面退役；owner=freeflash（GLM-5.3 Flash）免费车道子代理 · PC · ZCode 主管会话派发
- 分支：port/ui-entry-2 双仓；主仓 worktree D:/Vit_DAW_wt_uientry2（基线 origin/main=32307204，status/diff 全净）；前端 worktree D:/Godot/project/vit-daw-frontend_wt_uientry2（基线 main=ae24b52，status/diff 全净）

## 步骤

- [x] S0 勘察：卡面/协议/354 RUN-SUMMARY/agent_chat_window.gd 现状/entrySurfaceBridge.ts 读取；stop condition 预核查完成（见 S0 结论）
- [x] S1 领取：79a2a0e6 推 main ✓ 远端卡面 owner 亲核 ✓
- [x] S2 M1 关窗退出（`_teardown_after_close` → window_lifecycle_changed + app.exit 行 + get_tree().quit()；guard _mode_active）
- [x] S3 M2 失败链去 Godot 化：`_mark_unavailable` 去 restore/隐藏两步；`_boot_page_deferred` health 失败仍导航+`_start_health_recovery_poll`（240×2.5s 有界恢复轮，恢复→navigate.recovery 回 launcher）；`_handle_webview2_unavailable`（3 个真实建窗失败面+VIT_SMOKE_WEBVIEW2_FAIL 注入 → webview2.unavailable 行+OS.alert+app.exit+quit）；start_page.gd 删两条兜底恢复 connect（handler 保留，卡面「仅删编排引用不删代码」）
- [x] S4 M4 splash 压制：project.godot `boot_splash/show_image=false`（残余纯色数帧记手测 v5 观测）
- [x] S5 M3 停止条件命中：CommandDispatcher 仅 new/open/save、project_client 无 close、dock 无回 launcher 路径 → vit_dock_root.gd 零改动，降级记录交主管裁定
- [x] S5.1 语法验证：headless --import ✓（extension_list.cfg 产出）；两编辑脚本 --check-only exit 0；CEF dll 缺失为 354 同款 worktree 前置条件（dll 未入库、腿面不挂 browser panel），记边界
- [ ] S5 M3 工程态往返：**停止条件命中——降级记录**（内核 CommandDispatcher 仅 new/open/save 无 close；前端 project_client 同；dock 无回 launcher 场景路径）→ 不实现，交主管裁定
- [ ] S6 烟测脚本 scripts/ui_entry2_smoke.ps1（四腿：失败注入去 Godot 化/WebView2 错误框退出/autoload 全链+关窗全栈退出断言/legacy 回归）

## 里程碑提交

- S1 领取：主仓 79a2a0e6（coord-only 推 main）
- S2-S5 前端实现：前端仓 3f852f0（port/ui-entry-2 已推远端；agent_chat_window.gd/start_page.gd/project.godot；vit_dock_root 零改动=M3 停止条件降级）
- S7 webui 回归：零 diff；vitest 47 文件 563/563 PASS + tsc/vite build exit 0（worktree agent/webui）
- S6 脚本：主仓本笔（scripts/ui_entry2_smoke.ps1 四腿 + RUNBOOK leg2 主窗可见性边界修正 + PROGRESS）
- [ ] S7 webui 回归（npm install→test→build；零 webui diff 预期）
- [ ] S8 真栈：PC-RUNTIME-STACK 占用登记→推 main→重读→≤60s 复核→跑→finally 拆栈→释放
- [ ] S9 回执：卡面回填→doing→done→coord 推送

## S0 勘察结论（M2/M1 设计锚点）

- 354 现状：agent 不可达=restore_main_window(unavailable)+隐藏 React 窗（Godot 启动页兜底）；关 React 窗=restore_main_window(chat_closed)（回退 Godot 页）。两者均为本卡要退役的回退。
- extension 面（零 cpp 改动即可满足本卡）：`set_parent_window_shown`（SW_HIDE/SW_SHOW 主窗）354 已合入；WM_CLOSE→DestroyWindow→os_event "closed" 已有；dll 已提交入库（extension/bin/），worktree 免重编免 junction。
- 引擎限制（354 probe 铁证）：Godot 层 hide 主窗=降级最小化；真隐藏必走 extension SW_HIDE——沿用。
- load_error 边界（新发现）：extension NavigationCompleted 不查 IsSuccess——连接拒绝类页面失败不发 load_error（只有 Navigate API 失败才发）。故 health 失败后的"重试"以**低速恢复轮**（re-poll→恢复即重导航）实现，不依赖 load_error 链；记为诚实边界。
- autostart 链：VIT_SKIP_DEV_AUTOSTART 未设+editor 模式时 start_page 以 OS.create_process 拉起 kernel/hub/agent 并经 VitIpcClient.register_autostart_child_process 登记；quit 时 VitIpcClient._exit_tree OS.kill 全部登记子进程——关窗=退出编排的 agent 断言面由此成立（生产链=用户从 Godot 拉起全栈）。
- 内核 VIT_PROJECT_XML env 可经 Godot 进程环境继承到 autostart 内核（隔离 workspace）。

## 失败分类（§8 预登记）

- exit 0=PASS；断言失败/环境中断/装置缺陷分开记录；同代码同断点连续 2 败停原样重跑。
- 装置风险预登记：PS1 中文注释须 BOM；EnumWindows delegate 内禁 `$pid`（用 $procId）；OS.alert 原生框为阻塞模态（烟测以 EnumWindows 找 #32770 对话框 + WM_CLOSE 关闭后断言进程退出，不留悬挂模态）。
- S8 运行中（00:3x）：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_uientry2/scripts/ui_entry2_smoke.ps1`（占用登记 98ed4ed7 已推 main+重读确认+≤60s 复核 PROCS=0/五端口 free；run 根=coord/runs/UI-ENTRY-2/run_<stamp>；断了重派先查 run 目录是否已有本轮产物再决定续跑或新 run）
- S8 第 1 轮 run_20261115_002604（exit 1）：腿 1/腿 2 PASS；腿 3 败于 surface.ack 缺失。根因（真链取证）：presented 的 vit_surface 早于 React 挂载/订阅（page_loaded#2 晚于 presented=冷加载时序），消息进空网丢失；354 同构形态（双 page_loaded/双 auto_open）仅时序偶发不同=潜在竞速非本卡回归。修复=前端 83390f5（pending surface+React 就绪宣告/+5s 兜底有限重投，343 不变量不动）。第 2 轮命令同前（占用仍持有，启动前重复核）。
- 第 2 轮 run_20261115_003340（**exit 0 PASS**）：四腿全绿——腿 3 关窗→Godot/agent/kernel 全退出+端口释放（close_* 全 true）；surface.ack(project) 经重投链收敛到达；腿 4 legacy 回归绿。RUN-SUMMARY.md 已写。
- [x] S8 完成（finally 拆栈+亲证四 TCP=0+PROCS=0）；下一步 S9 回执：卡面回填→doing→done→coord 推送。
- [x] S9 完成：卡面回填（receipt）→ doing→done（7b480b74）+回执补工件 commit 95bf0304 推 main；释放记录 ee31d4d8 推 main；工件精简件 0e42a861 推 port/ui-entry-2。
- 终态：DONE（执行侧自验完成，待主管决策验收+用户手测 v5）；双仓 worktree 留 port/ui-entry-2 待主管验收后清理（无 reparse point 亲证，移除安全）。
