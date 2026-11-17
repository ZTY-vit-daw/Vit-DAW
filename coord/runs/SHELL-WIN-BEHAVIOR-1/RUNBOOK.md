# 391 / 394 / 395 / 402 组合验收 RUNBOOK（装置准备，未放行启栈）

271已由主管释放；栈空闲不授本会话真实启动权。本文件冻结装置计划；正式矩阵仍等主管登记owner并放行。主管另允许skip-autostart的隔离语义/生产装配单元；已执行15+7项，未启动VitShell/agent/kernel/HTTP。

## 范围和实现文件

- 主仓已有3脚本加主管批准的第4脚本`ui_entry_smoke.ps1`可选即时守卫，默认off。产品UI不改。主管已把389/395候验+装置0a6dfcf5集成到`C:/Users/timoz/.codex/worktrees/entry-final-validation/Vit_DAW`（branch codex/entry-final-validation）；接续期间主管又合入395compact修复699ef067，最终装置f551d32e以699ef067为parent，status净。正式被测组合以最新HEAD记录，不用旧2f45基线。旧converge-entry worktree不再写。
- 402-B前端唯一worktree=`C:/Users/timoz/.codex/worktrees/godot-entry-396/vit-daw-frontend`，领取HEAD=`0017bba55c7f2c69fcd0dd5cb1c9b11430b1282a`，领取status/diff为空，AGENTS.md不存在。仅3生产gd；已独立commit1301345。主管之后水合spectrum_data.exr（tracked pointer与源SHA70736DAF…F1EF一致）与extension三DLL，无junction；已知EXR hydration M不入提交。C++不改。
- 权威协议：前端`VitShell/tests/README.md`及`tools/README-vitshell-readiness.md`。`workspace_ready`仅代表生产dock同步_ready完毕+真实首snapshot入ProjectRepository并广播project_loaded，不声称插件/音频/分析完成。

## 402-B门控与真实断点

注入只对`VIT_SHELL_DIRECT=1`、非空native `VIT_SHELL_LAUNCH_ID`且显式`VIT_SMOKE_SHELL_READINESS=1`生效；scenario白名单，默认空严格零行为。无明确开关/未知值忽略并结构日志，不允许任意路径或未界定代码执行。每条hook日志含launch_id、workspace_instance_id（未mount为空）、scenario及实际trigger。

| scenario | 实际断点与效果 | 期望真实结果 |
|---|---|---|
| scene_failed | start_page._enter_main_scene在真实load前把目标限定替换为固定不存在res:// smoke场景，执行原load和原失败分支；禁止直接fake失败receipt | 原scene_failed、okfalse，未mount/未opened |
| snapshot_failed | vit_dock._apply_kernel_project_reply_and_emit在真实reply后将首snapshot副本的tracks置为非Array，让实际ProjectRepository.apply_kernel_project_state返回false；VSP fallback/legacy失败继续走原snapshot_failed链 | 真实mounted、有repo拒绝锚点，零snapshot_applied/ready/opened |
| delay_snapshot | 同一apply边界延迟真实首snapshot应用；受控`VIT_SMOKE_SHELL_READINESS_DELAY_MS`只允许100–30000ms；所有并发首刷新共享截止点，不能旁路就绪。deadline已判失败时不迟到升级ready | delay小于deadline最终真实ready；delay大于deadline真实workspace_timeout |
| exit_before_ready | 真实workspace_mounted后实际SceneTree.quit(17)，阻止该进程在退出前发送成功ready；不fake失败POST | Godot进程退出、native失败/guard释放，零opened |

正面default不开hook，用真实日志链。正常delay成功使用6000ms、deadline30s；timeout使用delay15000ms、deadline8s，并先亲核mounted与delay_begin（缺这两锚点不能把引导慢误记为快照超时）。参数仅在单次VitShell进程子树env注入，Start finally清除；不修改用户环境。正式时间戳按实际run记录，不挪动失败attempt。

## 矩阵与预算（跑前冻结）

有效场景各1次，整体不是稳定性评估；正常主旅程1轮。先normal full journey（389新壳、388管理回归、395真实消费），再delayed_success（wrong/old nonce配对），再scene_failed、snapshot_failed、workspace_timeout、exit_before_ready各1独立run，共6次VitShell生命周期。旧基础bad-agent/occupied-port/WebView2-off/legacy不逐失败场景重跑；组合normal主旅程按受影响域重验，旧稳定A证据不得归功新焦点。

所有管理/窗口/故障腿零chat，独立telemetry文件必须零新增；normal full旅程既有L3仅本机dead endpoint诚实失败交付，供应商LLM预算0。成功条件先于运行冻结：脚本exit0+阶段断言+外层卫生exit0+原始证据完备。失败分别记录apparatus/assertion/crash/capability_blocked/no_candidate_found/environment interruption；环境中断必须原始退出/日志支撑。相同版本相同确定性断点连续2次失败停止原样运行，先诊断/缩域/主管裁定；不得重跑直到偶然成功。

## open前采样与关联

CDP在任何launch click/directive前注册真实`chrome.webview.message`监听器，记录本run unique subscription_id、本页sentinel、wall receive time、递增capture index；再发既有`window_presentation_ready`握手。握手的replay只作为初态，request_id/seq不重写。监听同时抓`vit_launch_progress`、`vit_window_state`、`vit_launcher_result`、surface；DOM快照包含395 feedback request/seq/terminal、native-focus、步骤文案。open前还实际toggle侧栏验证aria/inert和reduced-motion computed，再恢复展开。

在同一CDP长阶段中启动40ms只读GET7879/status采样循环（每条保存observed_at_tick_ms），**确认首条采样完成后才点open**；不依赖opened后补取样。完整interval保留丢失/错误/null，不填零、不造中间阶段。request_id由真实spawning进度确定，launch_id由Godot日志与native accepted/rejected同ID复核；上一请求终态不得终结新请求。每次实际spawn前另由ps1/包装器fetch远端owner+≤60s即时进程/端口净空，长构建/import后重做。

`seq`在宿主实例全局单调递增，replay可重复但不生成新步。对当前request采集唯一seq严格递增；peer旧request/较小seq只记录，不把它计入当前步骤。所有progress.step必须属于生产枚举，不能编造文件读取/音轨恢复/设备阶段或百分比。

## 真实ready和窗口断言

- 保留每个Godot进程实际日志（默认Godot app_userdata/Vit_DAW_Frontend/logs/godot.log按launch后立即复制、不能等下次spawn覆盖）。按launch_id+godot_pid+workspace_instance_id匹配`readiness.begin`→`workspace.mounted`→`workspace.snapshot_applied`→`readiness.decided(oktrue,stageworkspace_ready)`→`launch_result.post`原始行序；native原bridge必须有同nonce `launch_result.accepted`，之后真实progress opened且stageworkspace_ready、真实result opened。场景受理不等同ready。
- 加载期必须观察至少一条guard active且DAW HWND在场、hooks_ready=true、shell_above_daw=true、daw_noactivate=true；foreground必须实际观察shell在前，不能none/other/null视为通过。不调用桌面点击、SetForegroundWindow、输入注入或CUA来制造条件。未观测到shell前台/事实不可得记FAIL或待主管裁定boundary，不能全绿。
- ready、失败、timeout、exit后必须观察guard=false，release_reason按真实路径记录，DAW仍在时其被保护NOACTIVATE位应释放为false（本新建窗原本无该位）；已退出时daw_hwnd=null只能证明不存在，不替代存活窗style断言。ready后的状态不等于“正常点击谁前台”已实测，该普通焦点体验留用户手测。
- wrong/old nonce只在delayed_success已真实spawning且guardactive、首snapshot延迟期间投递HTTP。wrong_id为本run随机nonce，old_id来自normal已完成request；POST oktrue仅是注入载荷，HTTP202不算成功。要求native各一条wrong_launch_id拒绝/expected=current，错误POST后当前链仍pending/guardactive/零opened；随后只能真实Godot当前nonce ready终结。保留独立注入区间与拒绝日志。
- 395真实DOM按同request/seq推进，所有positive seq须存在对应native记录，反馈没有百分比，pending不提前ready；末态只有真实workspace_ready才ready。delay成功窗口内在wrong/old HTTP后另测synthetic consumer：错误request、重复seq、缺request/stage进度不能升级；无stage的legacy opened result保持unconfirmed，必须等真实native ready才升级。捕获器逐条标synthetic并排除出native成功证明，不能把注入opened当真实结果。若WebView EventTarget不支持分发，不侵入产品provider，声明浏览器负例boundary并回指395状态测试。
- 395 data-native-focus与真实window观察一致；未实际DAW前台时不能把“没有focus hint”说成DAW轻态正面通过。侧栏DOM toggle与aria-expanded/inert配对；emulate reduced-motion后computed animation/transition none，恢复侧栏供388管理回归。浏览器DOM自动化允许，主机窗口输入禁止。

## 卫生、运行命令与产物

仍走既有脚本，新增可选`-WindowBehaviorChecks -PresentationScenario normal|delayed_success|scene_failed|snapshot_failed|workspace_timeout|exit_before_ready`；包装器支持`-CardId SHELL-WIN-BEHAVIOR-1`并按主管资源记录匹配entry388 owner。`-PreviousLaunchId`只给delayed_success，读normal原phase证据后传，不写死。negative/delay独立run只跑各自场景，不重复基础failure/legacy；normal仍完整旅程+388管理。timeout等延迟门实际到期且记录late-apply被拒后再拆栈。

正式命令冻结如下（先主管记录最终主仓HEAD与前端1301345、登记owner、逐场景fetch净空后执行；`$entryMatrixRoot`必须选不存在的绝对新目录）。每条原退出码独立记录，不合并成一个成功计数：

```powershell
$entryMatrixRoot = 'C:/Users/timoz/.codex/worktrees/entry-final-validation/Vit_DAW/coord/runs/SHELL-WIN-BEHAVIOR-1/matrix_20261117_XXXX'
$entryFrontend = 'C:/Users/timoz/.codex/worktrees/godot-entry-396/vit-daw-frontend'
$entryFixture = 'D:/Vit_DAW/temp/f3_empty_project.vit'
& powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg A -CardId SHELL-WIN-BEHAVIOR-1 -WindowBehaviorChecks -PresentationScenario normal -FrontendWorktree $entryFrontend -FixtureSource $entryFixture -RunRoot "$entryMatrixRoot/normal"
# STOP on nonzero: record the original exit; do not continue dependent scenes.
$entryPreviousLaunchId = (Get-Content "$entryMatrixRoot/normal/device/r1_cdp_chat_new.json.presentation.json" -Raw | ConvertFrom-Json).request_id
& powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg A -CardId SHELL-WIN-BEHAVIOR-1 -WindowBehaviorChecks -PresentationScenario delayed_success -PreviousLaunchId $entryPreviousLaunchId -FrontendWorktree $entryFrontend -FixtureSource $entryFixture -RunRoot "$entryMatrixRoot/delayed_success"
& powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg A -CardId SHELL-WIN-BEHAVIOR-1 -WindowBehaviorChecks -PresentationScenario scene_failed -FrontendWorktree $entryFrontend -FixtureSource $entryFixture -RunRoot "$entryMatrixRoot/scene_failed"
& powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg A -CardId SHELL-WIN-BEHAVIOR-1 -WindowBehaviorChecks -PresentationScenario snapshot_failed -FrontendWorktree $entryFrontend -FixtureSource $entryFixture -RunRoot "$entryMatrixRoot/snapshot_failed"
& powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg A -CardId SHELL-WIN-BEHAVIOR-1 -WindowBehaviorChecks -PresentationScenario workspace_timeout -FrontendWorktree $entryFrontend -FixtureSource $entryFixture -RunRoot "$entryMatrixRoot/workspace_timeout"
& powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg A -CardId SHELL-WIN-BEHAVIOR-1 -WindowBehaviorChecks -PresentationScenario exit_before_ready -FrontendWorktree $entryFrontend -FixtureSource $entryFixture -RunRoot "$entryMatrixRoot/exit_before_ready"
# 389 changed ui_entry_smoke itself. It has its OWN run/exit, not ui_entry2 credit.
& powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg C -CardId SHELL-WIN-BEHAVIOR-1 -FrontendWorktree $entryFrontend -FixtureSource $entryFixture -RunRoot "$entryMatrixRoot/entry389_C"
```

外层在snapshot前先守卫owner/净空，seed/build前快照recents bytes/absence、两个root Settings/default XML（窗口run另保全Godot默认日志）；每场景finally还原并SHA256亲证、fixture源前后哈希，失败非0。只写run fixture副本。C调用389修后的原ui_entry_smoke，native/import每真实spawn前调用同wrapper守卫，仅放行本run已记录PID且exe路径等于本worktree的独立kernel/agent与已确认子树，绝不一般复用同名进程。optional catch只清本run shell子树，启动前守卫失败不获全机kill权。所有配置/env只在本run子树。最终完整TCP7878/7879/8787/5555/5556/9333、UDP4445与Vit进程全零后报告主管释放，自己不写协调main。

工件每run独立：HEAD/status/diffstat、完整命令/inner+wrapper exit、构建hash、stdout、native原bridge、Godot原日志、capture消息/采样全量JSONL、当前request阶段JSON、395 DOM证据/截图、wrong/old注入与拒绝、telemetry局部快照、hygiene before/after与fixture hash。沿用原生meta.json/phase JSON，不造d1report。readiness/noactivate/consumer任一不足必须标剩余边界；不能以字面phase出现代替最终exit。
