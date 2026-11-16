# KERNEL-SAVE-FORENSICS-1：D1/D2 内核保存语义取证修复 + D3 agent 消毒面（374 上交锚点）

- 发卡：主管 2026-11-16 晨（承接 374 域外锚点三件，未扩修纪律正确）
- 池序 378；P1；依赖=无；真栈候栈位
- 目标：①D1：XML-boot 会话加轨 save 丢轨（取证→修复）；②D2：save 后 clip_id 披露降级直至重开（取证→修复或明示语义）；③D3：agent 消毒面剥 project_path 对谱面面板的影响（锚定消毒位→最小披露方案）；④各配回归测+真栈断言。
- 文件域：VitApp/Source 保存/披露链+agent 消毒位+测试+coord 工件；领取时锚定。
- 验收：三锚点各自的取证-修复-回归链+真栈腿。

## 领取

- 时间：2026-11-16 晚 +0800（领取提交时间以 git 为准）
- 基线：origin/main `919e9650`（fetch 亲核；领取窗口内 377 done 提交刚落，worktree 在最新 tip 上建立）
- owner：GLM-5.3 执行子代理（ZCode L2 会话）/ PC
- 分支：`port/kernel-save-forensics-1`；worktree `D:/Vit_DAW_wt_378save`；协调 worktree `C:/Users/timoz/wt_378coord`
- 领取时基线核对：HEAD=919e9650，status 干净，diff=0（新 worktree 即净）
- 文件域锚定（领取时核对）：`VitApp/Source` 保存/披露链（save_project/open_project/get_project_state 域）+agent get_project_state 消毒位（领取时锚定具体文件）+内核 C++ 测试（VitApp tests）+`scripts/` 烟测扩展+`coord/runs/KERNEL-SAVE-FORENSICS-1/**`+卡面/栈登记。doing 卡核对：WAVESHELL（只读取证）/LAUNCHER-ZEROFRICTION（只读）/SRC-PRESET-AGENT（processorregistry 域）/377 done 挂账（scoreworkbench 域）——与本卡无交集。
- 三锚点口径（卡面为权威）：D1=XML-boot 会话加轨 save_project 后新轨被丢（.vit-boot 不复现）；D2=save_project 后 get_project_state clips[].clip_id 降级 null 直至 open_project 重开；D3=agent get_project_state 消毒面剥 project_path（374 腿经 save_as 目标+--project-xml 双源绕开）。

## 回执

- **执行完成 2026-11-16 晚**；分支 `port/kernel-save-forensics-1` 已推（f6d427b4 + bae6e2e6）；领取提交 3bfaad42（基线 919e9650，期间被 377 done/bfea4955 顶掉一次按协议重应用）；栈占用/释放链在 PC-RUNTIME-STACK.md（占用 20:17 离线 kernel-only + 20:35 真栈 berth；释放 20:48 c2a48295）。

### D1（XML-boot 会话加轨 save 丢轨）：取证收口=**当前 main 不重现**（五腿证据，无代码修复）

1. kernel-only ZMQ 腿 25/25 PASS（attempt3 原序：XML-boot→add_track→insert_midi_clip→add_midi_notes→agent 同款 enriched save（file_path+app_bound_aes）→即时披露读回；内核 VitApp.exe=worktree 自建基线 7C45E745AB58BFE2）。
2. agent 面 13/13 PASS（/agent/invoke 同序，含 draft 路径 save）。
3. 竞态探针 10 轮全 PASS+28 次并发 get_project_state/ui_state 轮询零空轨、零 null clip（面板 250ms 轮询竞态假设证伪）。
4. D3 回归轮 27/27 复验（ rebuilt 内核 66001D546F1752AE）。
5. **真栈正式轮 attempt3 PASS exit 0**：dev_agent_smoke.ps1 -StartKernel -RestartAgent -ScoreWorkbenchSmoke -SaveForensicsSmoke -NoChatSmoke -KernelExe <worktree build>——strip-silence 前缀+真浏览器+worktree 新内核，F4-D1-post-save-track-disclosed PASS。
- 代码域证伪：374 观察窗（M5b agent+主仓 04:33 内核）与当前 main 的保存/披露链（VitHeadlessService/CommandDispatcher/VspKernelReference/harness/vsp.go）逐 diff 为零改动；现象成因落在该环境窗内不可复因素（374 回执自记 attempt1「陈旧部署 b6565dcf 无披露面」的部署面漂移/时序），当前代码无可修缺陷。
- 失败分类：not_reproducible_on_current_main（非环境中断——四腿全绿为正结果）。

### D2（save 后 clip_id 披露降级至重开）：同判**不重现**（同五腿；F5/F8 PASS）

- 附带锚定（明示语义非缺陷）：legacy `get_project_state` 面 `tracks[].clips[]` 仅披露 `id` 键；`clip_id` 是 VSP 紧凑面（normaliseClipArray）与 shadow Summary 的双键投影（clip_id+id）。agent 侧寻址链 visibleClipRefs 读 `clip_id/id/item_id` 三键兜底——save 前后寻址均绿。
- 374 工件复勘：attempt3/4 失败点=save 后 get_midi_clip_notes「clip 1042 not present」；attempt5 的 A6 基线取自 save 前读数属规避动作（非再观察）。

### D3（agent 消毒面剥 project_path）：**取证修正归因+最小披露修复**（commit f6d427b4）

- 归因修正：374「userVisibleState 消毒面剥 project_path」**不成立**——userVisibleState 仅剥 engine/internal_track_count 两键；真实缺口两层：①内核语义——XML-boot 未存工程 currentProjectPath 为空 → project_path 披露空串（诚实，非剥除）；②shadow.Summary() 重建面白名单不含该键（invoke 面=withOKStatus(userVisibleState(shadow.Summary()))，attempt2 F1/F7 断言失败实证）。
- 最小披露：内核 get_project_state 新增 `effective_project_path`（=getEffectiveProjectFile：已存=绑定路径；未存=引导 XML 回退；project_path 语义不变）+VSP 紧凑面 project{} 投影+agent legacyStateFromVSPPayload 顶层投影+shadow Summary 投影（旧内核回退=绑定路径，空保持空，不伪造）。
- 回归测：go test internal/kernel（TestLegacyStateFromVSPPayloadProjectsEffectiveProjectPathFallback 新增）+internal/shadow（TestSummarySurfacesEffectiveProjectPathFallback 新增）+internal/harness 全绿。

### 测试命令与退出码（被测=worktree HEAD f6d427b4^..bae6e2e6）

- go build ./... exit 0；go test ./internal/kernel ./internal/shadow -count=1 ok；go test ./internal/harness -count=1 ok（46.9s）
- 内核离线回归（确定性 driver，ZMQ 直连）：offline_d1d2_repro 25/25 + offline_d3_regression 27/27（含 legacy+VSP 双面 D3 断言）
- 真栈正式轮：attempt3 exit 0，29/29 断言（F1-F9+A1-A8 含 A7 探针非静音/A8 差分）；vitest/webui 不涉（webui 零触碰）
- run ID：run 目录 coord/runs/KERNEL-SAVE-FORENSICS-1/{offline_d1d2_repro,offline_d3_regression,realstack_leg/attempt{1,1b,2,3}}（@port 分支入库；.gitignore 排除 exe/WAV/草稿树，374/377 先例）

### attempt 台账（§8 失败分类）

- attempt1=driver_defect（探针脚本 parseArgs 引用未初始化 summary→TDZ 崩溃，断言前死亡；修复后跑；残留栈手动清+recents 还原）
- attempt1b=误触发 import 测试（卫生事故：node import 冒烟跑进活栈 main()，0 断言，工件留档 attempt1b_accidental_import_test，recents 还原）
- attempt2=assertion_failure（F1/F7=D3 agent 面真缺口——非假失败，驱动 shadow Summary 修复）
- attempt3=**正式轮 PASS exit 0**（单次成功，无重跑）
- LLM 计费上界=0（全确定性面，零 chat/LLM 参与）

### 端测覆盖边界声明（AGENTS §5）

- 真栈腿=kernel+agent+真浏览器 berth（不启 Godot UI）；渲染面断言=A1-A8 既有 DG1-A 全套（374/377 同款，含面板重载/A7 出声探针/A8 渲染差分）；用户旅程面（VitShell→工程→谱面入口）不在本卡覆盖（SCORE-ENTRY-REACH-1 在飞域）。
- D1/D2 结论边界：五腿均在当前 main 代码+自建内核上执行；374 现象的原始二进制（主仓 04:33 构建，provenance 不可复原）未复测——如决策侧需要可另派考古卡在 374 worktree 快照上复刻。

## 验收

（决策侧回填）

## 验收

（决策侧回填）
