# 主管验收复跑（179+183 合并态）RESULTS

- owner=GLM-5.3 主管决策流（ZCode 验收会话）；2026-10-24 19:2x–19:5x；§2.2 占用登记 dda3ea5f。
- 被测=**前端主检出合并态 5da6360**（3e9d00d=179 合入 → 5da6360=183 合入）；kernel=主检出 K3 722A79D7 只读（占用前哈希亲核）；RepoRoot=主检出；VIT_PROJECT_XML=本目录 lvlfader_iso_project/ git 干净版隔离副本（183 首轮污染教训，主检出 workspace 零触碰）。
- 验收方法=执行侧回执亲读（diff 红线逐面+工件 SUMMARY/日志判定行）+ 合并后主管独立复跑两相位。

## Phase B：SendRouteGuiSmoke 全栈腿（自动起拆）

- 命令：`dev_agent_smoke -RepoRoot D:/Vit_DAW -StartKernel -RestartAgent -SendRouteGuiSmoke -KernelExe <K3> -GodotProjectRoot <前端主检出> -GodotExe <4.6.1 console>`（VIT_PROJECT_XML=隔离副本）。
- **脚本 exit 0；stack probe SUMMARY_JSON checks_total=72 checks_failed=0 status=pass**——K3/L8/L9/L10 零回退 + **L11 推子拖动腿 8/8**（真实事件管线 motion→press→motion…→release；拖中 repository 零落值+回显；释放读回收敛；推子位随回执同步）。隔离副本下**一次过**（对照 183 执行侧首轮 67/72=主检出运行时工程预存边污染）。
- chat/confirm 腿过（脚本自带白名单话术）。console 全量=本目录 supv_sendroute_console.txt。
- 附注（如实）：脚本完成后残留 VitApp 21796/VitAgent 32548（本腿所起，路径亲核）手动清拆后复核端口/进程=0，再进 Phase A。

## Phase A：裸栈（kernel+vsphub+agent，179 bringup 形态）探针族

- bringup：kernel 18596（kernel_workspace 隔离）+vsphub 31124+agent 29928，/health 200 就绪（phaseA_bringup_state.json）；agent=Phase B 脚本构建的主检出 agent/bin/VitAgent.exe。
- 探针（Godot 4.6.1 console headless，前端主检出合并态，逐轮新进程）：

| 探针 | 轮 | 结果 |
|---|---|---|
| probe_fe_gui_shrink_phase_c | 2（活栈竞态面） | 2/2 PROBE PASS |
| probe_fe_gui_shrink_phase_a | 1 | PROBE PASS |
| probe_fe_gui_shrink_phase_b | 1 | PROBE PASS |
| probe_rack_sendlevel_fader（183 新） | 1 | PASS checks_total=35 |
| probe_rack_plugin_search_popup | 1 | PASS checks_total=111 |
| probe_rack_sendlevel_realinput | 1 | PASS checks_total=15 |
| probe_start_feed_view | 1 | PASS checks_total=244 failed=0 |

- 拆栈：Stop-Process 18596/31124/29928 → 复核 VitApp/VitAgent/vsphub/Godot=0、七端口 tcp=0、4445/5555 udp=0。

## 结论

- 179+183 合并态零回退全绿；183 L11 与 179 shrink_c 修复面在主管独立复跑下复现通过。两卡验收通过（裁定见卡面验收字段与当日 gate）。
