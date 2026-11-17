# 391 / 394 / 395 / 402 组合验收 RUNBOOK（装置准备，未放行启栈）

当前271 `/root/closure271`独占PC栈。本文件只冻结装置计划；本会话不得启动Godot（含解析/import）、VitShell、agent、kernel或访问运行中HTTP。只做静态/PowerShell AST/node --check。正式启栈须主管另登记owner、确认组合main与前端HEAD，再放行。

## 范围和实现文件

- 主仓只改已有3脚本：`scripts/vitshell_full_smoke.ps1`、`scripts/vitshell_full_smoke_cdp.mjs`、`scripts/run_entry_management_acceptance.ps1`，新开关默认off。产品UI不改，389/395由主管ff纳入干净branch，不自行merge其他执行流。
- 402-B前端唯一worktree=`C:/Users/timoz/.codex/worktrees/godot-entry-396/vit-daw-frontend`，领取HEAD=`0017bba55c7f2c69fcd0dd5cb1c9b11430b1282a`，领取status/diff为空，AGENTS.md不存在。仅3文件：`app/startup/start_page.gd`、`vit_dock/scenes/vit_dock_root.gd`、`app/shell/vitshell_launch_readiness.gd`；C++不改。402-B与391装置独立commit。
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

CDP在任何click/directive前注册真实`chrome.webview.message`监听器，记录本run unique subscription_id、本页sentinel、wall receive time、递增capture index；再发既有`window_presentation_ready`握手。握手的replay只作为初态，request_id/seq不重写。监听同时抓`vit_launch_progress`、`vit_window_state`、`vit_launcher_result`、surface；DOM快照包含395 feedback request/seq/terminal、native-focus、步骤标签、reduced-motion computed。

在同一CDP长阶段中启动40ms只读GET7879/status采样循环（每条保存observed_at_tick_ms），**确认首条采样完成后才点open**；不依赖opened后补取样。完整interval保留丢失/错误/null，不填零、不造中间阶段。request_id由真实spawning进度确定，launch_id由Godot日志与native accepted/rejected同ID复核；上一请求终态不得终结新请求。每次实际spawn前另由ps1/包装器fetch远端owner+≤60s即时进程/端口净空，长构建/import后重做。

`seq`在宿主实例全局单调递增，replay可重复但不生成新步。对当前request采集唯一seq严格递增；peer旧request/较小seq只记录，不把它计入当前步骤。所有progress.step必须属于生产枚举，不能编造文件读取/音轨恢复/设备阶段或百分比。

## 真实ready和窗口断言

- 保留每个Godot进程实际日志（默认Godot app_userdata/Vit_DAW_Frontend/logs/godot.log按launch后立即复制、不能等下次spawn覆盖）。按launch_id+godot_pid+workspace_instance_id匹配`readiness.begin`→`workspace.mounted`→`workspace.snapshot_applied`→`readiness.decided(oktrue,stageworkspace_ready)`→`launch_result.post`原始行序；native原bridge必须有同nonce `launch_result.accepted`，之后真实progress opened且stageworkspace_ready、真实result opened。场景受理不等同ready。
- 加载期必须观察至少一条guard active且DAW HWND在场、hooks_ready=true、shell_above_daw=true、daw_noactivate=true；foreground必须实际观察shell在前，不能none/other/null视为通过。不调用桌面点击、SetForegroundWindow、输入注入或CUA来制造条件。未观测到shell前台/事实不可得记FAIL或待主管裁定boundary，不能全绿。
- ready、失败、timeout、exit后必须观察guard=false，release_reason按真实路径记录，DAW仍在时其被保护NOACTIVATE位应释放为false（本新建窗原本无该位）；已退出时daw_hwnd=null只能证明不存在，不替代存活窗style断言。ready后的状态不等于“正常点击谁前台”已实测，该普通焦点体验留用户手测。
- wrong/old nonce只在delayed_success已真实spawning且guardactive、首snapshot延迟期间投递HTTP。wrong_id为本run随机nonce，old_id来自normal已完成request；POST oktrue仅是注入载荷，HTTP202不算成功。要求native各一条wrong_launch_id拒绝/expected=current，错误POST后当前链仍pending/guardactive/零opened；随后只能真实Godot当前nonce ready终结。保留独立注入区间与拒绝日志。
- 395真实DOM按同request/seq推进，反馈没有百分比，pending不提前ready；末态只有真实workspace_ready才ready。旧/缺stage、错误request、重复较小seq的负例属明确synthetic consumer probe，与真实native链分卷记录，不冒充生产progress。若WebView真实EventTarget不支持安全分发，不侵入产品provider，改用已通过的395纯状态测试并声明此负例浏览器边界。
- 395 data-native-focus与真实window观察一致；未实际DAW前台时不能把“没有focus hint”说成DAW轻态正面通过。侧栏DOM toggle与aria-expanded/inert配对；emulate reduced-motion后computed animation/transition none，恢复侧栏供388管理回归。浏览器DOM自动化允许，主机窗口输入禁止。

## 卫生、运行命令与产物

仍走既有3脚本，新增可选`-WindowBehaviorChecks -PresentationScenario normal|delayed_success|scene_failed|snapshot_failed|workspace_timeout|exit_before_ready`；包装器支持`-CardId SHELL-WIN-BEHAVIOR-1`并按主管资源记录匹配entry388 owner。`-PreviousLaunchId`只给delayed_success，必须读normal原phase证据后传，不写死。CLI实现后锁定逐命令清单；当前不执行。

拟正式入口（主仓先由主管ff到389/395组合、前端含402-B后替换实际HEAD）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg A -CardId SHELL-WIN-BEHAVIOR-1 -WindowBehaviorChecks -PresentationScenario normal -FrontendWorktree 'C:/Users/timoz/.codex/worktrees/godot-entry-396/vit-daw-frontend' -FixtureSource 'D:/Vit_DAW/temp/f3_empty_project.vit' -RunRoot '<absolute unique normal run>'
# remaining scenarios use same existing wrapper with scenario + unique run;
# delayed_success additionally requires -PreviousLaunchId from normal artifact.
```

外层在seed/build前快照recents bytes/absence、两个root Settings/default XML；每场景finally还原并SHA256亲证、fixture源前后哈希，失败非0。只写run fixture副本；所有配置/env只在本run子树，不触碰271当前运行态。最终完整TCP7878/7879/8787/5555/5556/9333、UDP4445与Vit进程全零后报告主管释放，自己不写协调main。

工件每run独立：HEAD/status/diffstat、完整命令/inner+wrapper exit、构建hash、stdout、native原bridge、Godot原日志、capture消息/采样全量JSONL、当前request阶段JSON、395 DOM证据/截图、wrong/old注入与拒绝、telemetry局部快照、hygiene before/after与fixture hash。沿用原生meta.json/phase JSON，不造d1report。readiness/noactivate/consumer任一不足必须标剩余边界；不能以字面phase出现代替最终exit。
