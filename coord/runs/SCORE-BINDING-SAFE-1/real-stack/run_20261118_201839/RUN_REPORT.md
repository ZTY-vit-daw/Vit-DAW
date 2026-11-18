# RUN 2026-11-18 SCORE-BINDING-SAFE-1(397) 真栈正式轮报告（单次授权轮）

- run_id：`run_20261118_201839`
- owner：`lead/score397stack`（DeepSeek-V4.1-Flash 执行子代理，主管 `lead` 书面放行单次正式轮）
- 授权范围：仅一次正式真栈轮；断言失败不得原样重跑；本报告为该次唯一正式轮结果。
- 时间：2026-11-18 20:18:39 起（组装）→ 20:19:17 栈启动 → 20:19:55 腿结束 → 20:20:40 teardown 完成（+0800）
- 候选：worktree `C:/Users/timoz/.codex/worktrees/pc397-merge/Vit_DAW`；分支 `codex/score-binding-397-integ`；HEAD `b5f74fbb`（主管已合入主仓 main `8972c5a5`；主仓当前 HEAD `b4029366`=预约提交）
- 装置：`dev_agent_smoke.ps1 -StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke -KernelExe <run>/kernel_runtime/VitApp.exe -RunArtifactsDir <run>`；berth（kernel+agent，无 UI/Godot）；Chromium headless 由腿自起；LLM 死端点 `VIT_AGENT_LLM_BASE_URL=http://127.0.0.1:1`
- 结论：**PS1 exit 1 / leg exit 1 = assertion_failure（33 断言：31 PASS / 2 FAIL）**；失败=**J1/J2 两条旧断言期望过期**（仍按 382 时旧模板资产的“10.75/上取整”形态断言），非产品回归、非环境中断。**未重跑、未改断言**。

## 1 命令与退出

```powershell
cd C:\Users\timoz\.codex\worktrees\pc397-merge\Vit_DAW
$env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke `
  -KernelExe "<run>/kernel_runtime/VitApp.exe" `
  -RunArtifactsDir "<run>" *> "<run>/ps1_console_full.txt"
# PS1 exit = 1（score 腿失败，ps1 抛 "score workbench smoke failed with exit code 1"）
# leg summary: exit_code=1, failure_class=assertion_failure
```

- ps1 console：`ps1_console_full.txt`（sha256 `AE1F58DC…F44F`）
- 腿 console：`score_workbench_console.txt`（sha256 `9A6E2CAC…05F2`）
- summary：`score_workbench_summary.json`（sha256 `376900271E2A…0CAF`；duration 37,948 ms）
- 零 LLM：全腿确定性 invoke/ZMQ/UI 面 + 死端点 override；无 chat 调用。

## 2 33 断言结果（31 PASS / 2 FAIL）

PASS：A1–A8 全部十九断言；J1 旅程四断言中 3 PASS；J2 三断言中 2 PASS；J3 两断言 PASS；**S1 五断言全部 PASS**（S1-ui-binding-P1 / P4-reopen / P5-D4-regression / J1-journey / S1-apply-target）。

FAIL：
1. `J2-non120-clip-grid-snapped`：`status='已从内核装载（2/4 @ 80bpm，clip 8 拍）'`，断言期待含 `10.75` + `上取整`。
2. `J1-score-panel-from-project-journey`：面板装载成功、旅程链全通，但断言条件中 `journeyStatus.includes("10.75")` 不成立（status 同上）。

（同组关键面均 PASS：`J2-non120-panel-load`、`J2-non120-error-boundary-absent`、`J1-project-surface-reachable`、`J1-workbench-entry-present`、`J1-workbench-surface-switched`、`J3-return-button-present`、`J3-return-to-project`。）

## 3 失败归因（证据链）：旧断言期望过期，非产品缺陷

- 本 run 的模板资产 `lesson_project.xml`（place-copy 自 `coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml`）sha256 = `2B487058877EC0887FD20B23C7E2034A012E978116D59B335849A410FE0C6CBB`，其 **MIDICLIP 已是修正后的秒值**：`length="6.0"/6.0/12.0`@80bpm = 8/8/16 拍（模板 manifest `built_at 2026-11-16`，384 腿D 拍→秒换算修复后版本；382 取证时的旧资产为 `0CB22BA3…`）。
- 因此内核/桥按正确几何披露 `2/4 @ 80bpm，clip 8 拍`：**没有 raw 10.6667，也不需要网格上取整**——产品行为正确，且 A7/A8 音源/渲染全 PASS、J2 面板存活/无 pageerror/无错误边界 PASS。
- 两条失败断言是 382 时代针对**旧 buggy 模板**写的回归标记（`10.75 拍/上取整` 只应该在旧资产下出现）。模板资产后来被修正（跨卡变更），断言未同步 → **smoke_expectation_stale**（装置/判据陈旧），不是 face_defect、不是 env_interrupt。
- 本卡 397 相关判据全部通过：S1（UI 绑定身份==**实际** apply 载荷目标，且选轨前无隐式目标）、A1-A6（选轨→真实指针 edit→apply→目标 kernel 回读/保存重开）、J1 旅程结构、J2 面板安全回归。

## 4 §8 / 止损纪律声明

- 本次为**唯一一次**授权正式轮；出现断言失败后**未原样重跑、未自行第二次正式轮、未改脚本断言**，按授权要求停手上报。
- 失败分类：正式类 `assertion_failure`（exit 1）；子类 `smoke_expectation_stale`（证据见 §3）；无环境中断。
- 若主管决定继续：需先由主管裁定“J1/J2 断言按修正后模板资产更新”（脚本域变更，需授权与排队），再另行授权正式轮；本 owner 不自启。

## 5 栈位与 recents

- 跑前（20:18:47）即时核净：Vit 系进程=0、TCP 7878/7879/8787/5555/5556/9333=0、UDP 4445=0；PC-RUNTIME-STACK 顶部=本次预留（owner/候选一致）。
- recents 快照/还原：owner 跑前快照 `global_project_config.owner_before.json` = `807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353`；ps1 腿内快照同值；ps1 腿内还原+哈希亲证 `807226F4AF3F`；owner teardown 后复核 `global_project_config.after.json` 哈希=同值 → **一致**。无 382 所述 Strip Silence 窗口外写面差异（本次 ps1 快照点位于 strip 段之后，快照哈希仍与 owner 跑前初态一致）。
- **teardown（20:20:40）**：owner 亲停 VitAgent 27116 / VitApp 5276；复核 Vit 系进程=0、TCP 六口=0、UDP 4445=0 = **TEARDOWN_ALL_ZERO**。记录：`teardown/teardown_record.txt`。
- 释放行：因本段约束“不 push、不改主检出”，**未写 main 的 PC-RUNTIME-STACK.md**；已拟好释放文本 `STACK_RELEASE_NOTE.md`，请主管核对后登记（栈物理上已净空）。

## 6 哈希/工件清单

provenance 全量见 `runtime_provenance.txt`（已含两次追加）。要点：
- kernel（run 内 place-copy）`A59473614152882B21433DCEE684F8B13AE0BD5B632A97CD5E2C8765BB18348E`；settings `5ABF7D95…E076`
- agent（ps1 现场 go build 并安装）`6802D1199CBA60700C6BC5DE5DC4B28D5495F4521D41A7008CFAB820D5CD065F`（VitAgent.exe 与 dev-smoke 同哈希）
- webui dist：候选 worktree `npm run build`；dist/index.html `7F3C0270…F9178`；main-*.js `F4F8E7B3…D5C9`
- boot/default_project.xml（ps1 git-clean 拷贝）`4E899FB6…B571`（11,609 B LF）
- lesson_project.xml `2B487058…C6BB`（384 修正版模板）
- screens：p1/p2/p3/p4/j1/j2/j3 共 8 张（见 run 目录）
- WAV 对（本地保留，不纳入 git）：`render_before_edit.wav` 2,304,784 B `CF9E59D7…E823`；`render_after_edit.wav` 2,304,784 B `87EADBA1…4AEA`；summary.p5_renders：before peak 7364/rms 878.39、after peak 6818/rms 838.98、max_abs_diff 7853、probe rel_delta 0.005
- 内核日志：`kernel_logs/VitHeadlessServer2026-11-18_20-19-11.log.gz`（189,199 B `D493388F…A829`）+ 3 个 diag 小日志；agent 日志 `agent_last.log`（44,079 B）

## 7 供主管裁定（建议，尚未实施）

- 最小修正（脚本域）：`J2-non120-clip-grid-snapped` 与 `J1-score-panel-from-project-journey` 改为接受**修正后模板**形态：status 含 `已从内核装载` + `2/4` + `80bpm`（`clip 8 拍`），不再要求 `10.75/上取整`；保留 J2 面板存活/无 pageerror/无错误边界三条核心回归断言（它们已 PASS）。
- 若仍需回归 382 的“非网格 raw 上取整”防御本身，应另建**合成非网格 clip** 的独立用例（或保留旧资产副本只读 fixture），不能依赖已被修正的 373 模板。
- 以上为断言口径变更，需主管裁定/授权后另行排真栈轮；本段未改生产/脚本断言。
