# RUN 2026-11-18 SCORE-BINDING-SAFE-1(397) 真栈第二次正式轮报告（PASS）

- run_id：`run_20261118_202521`
- owner：`lead/score397stack`（DeepSeek-V4.1-Flash 执行子代理；主管 `lead` 2026-11-18 20:23 书面放行第二次正式轮，PC-RUNTIME-STACK 顶部预留行与候选一致）
- 授权范围：仅一次正式真栈轮；通过=收口；失败即停上报、不原样重跑、不自起第三次正式轮。
- 时间：2026-11-18 20:25:21 组装 → 20:25:31 栈启动 → 20:26:12 腿结束（35.5s）→ 20:26:24 teardown 完成（+0800）
- 候选：worktree `C:/Users/timoz/.codex/worktrees/pc397-merge/Vit_DAW`；分支 `codex/score-binding-397-integ`；**HEAD `e882267e`**（第二轮修订提交：仅 J1/J2 两条过期期望 + runbook；断言面仍 33）。主仓 main HEAD `84ac6d9c`（含 20:23 预留提交；合并基础 `8972c5a5`）。
- 装置：`dev_agent_smoke.ps1 -StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke -KernelExe <run>/kernel_runtime/VitApp.exe -RunArtifactsDir <run>`；berth（kernel+agent，无 UI/Godot）；Chromium headless 自起；LLM 死端点 `VIT_AGENT_LLM_BASE_URL=http://127.0.0.1:1`。
- **结论：PASS。PS1 exit 0 / 腿 exit 0 / summary exit_code=0；33/33 断言全 PASS；零 LLM。**

## 1 命令与退出

```powershell
cd C:\Users\timoz\.codex\worktrees\pc397-merge\Vit_DAW
$env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke `
  -KernelExe "<run>/kernel_runtime/VitApp.exe" `
  -RunArtifactsDir "<run>" *> "<run>/ps1_console_full.txt"
# PS1_EXIT=0；console: "ok: score workbench DG1-A berth smoke passed"
```

- ps1 console：`ps1_console_full.txt`（sha256 `B1A66F92…499A`）；腿 console：`score_workbench_console.txt`（`3A3441A8…F FDA`）
- summary：`score_workbench_summary.json`（sha256 `75E9B014B4A0037D268E67A5C3A65D0C1BDBA5E656503AED8BBF9D87B9B5DB5D`；duration 35,500 ms）
- 关键 console 行：`[score-wb] verdict: PASS (33 assertions)` + `ok: score workbench DG1-A berth smoke passed`

## 2 33/33 断言（全 PASS）

- **S1 五断言全 PASS**：`S1-ui-binding-P1`、`S1-ui-binding-P4-reopen`、`S1-ui-binding-P5-D4-regression`、`S1-ui-binding-J1-journey`、`S1-apply-target`（UI 绑定身份==真实 apply 载荷目标==kernel 回读目标；选轨前无隐式目标）。
- **A1–A8 十九断言全 PASS**（load/write-readback、id binding/stable、symbol coverage、双拍号、saved-after-verify、save→open→reload、A7 模板音源 edit-readback+probe non-silent、A8 WAV 差分）。
- **J1/J2/J3 九断言全 PASS**：其中第二轮修订的两条期望按修正后模板形态通过（status=`已从内核装载（2/4 @ 80bpm，clip 8 拍）`）：
  - `J2-non120-clip-grid-snapped` PASS（修正后资产形态；旧 buggy 资产 10.75/上取整 已退役）
  - `J1-score-panel-from-project-journey` PASS（旅程链+身份绑定+修正后形态）
  - `J2-non120-panel-load` / `J2-non120-error-boundary-absent` 保留并 PASS（面板存活、零 pageerror、无错误边界触发）
- P5：手架模板旋律轨 Surge XT 按名装载；A4-template 2/4@80bpm；probe 非静音；WAV 对 before peak 6604/rms 869.11、after peak 7023/rms 899.83、max_abs_diff 7312（阈值 500）；probe rel_delta 0.0008（支持性证据）。

## 3 哈希/provenance

（全量见 `runtime_provenance.txt`）
- kernel（run 内 place-copy）`A59473614152882B21433DCEE684F8B13AE0BD5B632A97CD5E2C8765BB18348E`
- 插件库 Settings `5ABF7D95C344D5F1CF8FF34E27EA72905F7209A16A7ABA40AA03FC21EBDAE076`
- agent（ps1 现场 go build）`6802D1199CBA60700C6BC5DE5DC4B28D5495F4521D41A7008CFAB820D5CD065F`（VitAgent.exe 与 dev-smoke 同哈希）
- webui dist（候选 worktree 构建）index.html `7F3C0270…F9178`；main-*.js `F4F8E7B3…D5C9`
- boot/default_project.xml（ps1 git-clean 拷贝）`4E899FB6…B571`（11,609 B LF）
- lesson_project.xml（384 修正版模板）`2B487058…C6BB`
- WAV 对（本地保留不入 git）：before 2,304,784 B `F771B67C…E0798`；after 2,304,784 B `998987EA…0043A`
- 内核日志：`kernel_logs/VitHeadlessServer2026-11-18_20-25-31.log.gz`（185,717 B `3FAF258C…E2443`）+ 3 小 diag；agent 日志 `agent_last.log`（39,908 B）

## 4 recents / teardown / 栈位

- 跑前即时核净（20:25:28）：Vit 系进程=0、TCP 7878/7879/8787/5555/5556/9333=0、UDP 4445=0；PC-RUNTIME-STACK 顶部=本次预留（owner/候选/worktree 一致，来自 origin/main 只读核对）。
- recents：owner 跑前快照 = ps1 腿内快照 = ps1 腿内还原后 = owner teardown 后 = **`807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353`（哈希亲证一致）**。
- **TEARDOWN_ALL_ZERO（20:26:24）**：owner 亲停 VitAgent 24472 / VitApp 11688；复核 Vit 系进程=0、TCP 六口=0、UDP 4445=0；记录 `teardown/teardown_record.txt`。
- 释放行：受本段“不 push、不改主检出”约束，未写 main 的 PC-RUNTIME-STACK.md；已拟 `STACK_RELEASE_NOTE.md` 请主管登记（栈物理上已净空）。

## 5 §8 声明

- 本次为授权范围内**唯一一次**正式轮，PASS 即收口；无重跑、无第三次轮。
- 失败分类：不适用（exit 0）；零环境中断、零 LLM。
- 第一轮（run_20261118_201839）的 2 条 smoke_expectation_stale 已在本轮修订后关闭；本轮 33/33 全绿。
- 边界：按 runbook §8，S2（非目标 clip 不变）/S3（dirty 切换真栈腿）/S4（多 clip 显式选择真栈腿）仍未实施；E2E-WEBUI-1 score 段适配需其 owner 在 agent 实跑面复跑复绿；用户手测仍按既有裁定。
