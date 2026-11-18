# RUNBOOK（草案）— SCORE-BINDING-SAFE-1（397）真栈验收腿

**状态：草案 / 未授权运行。** 编制：`lead/score397stack`（DeepSeek-V4.1-Flash 执行子代理；主管 `lead`），2026-11-18 晚，PC。
**本文件编制期间零真栈**（未启动 VitApp/agent/Godot/VitShell/真实 Chromium、未登记占用、未改生产代码）。
**只有同时满足 §0 全部门槛并由主管书面放行后，才可按 §4 启动真栈。** 模板参考：`coord/runs/SCORE-DG1A-COMPLETE-1/RUNBOOK.md`（377 DG1-A）与 `coord/runs/SCORE-ENTRY-REACH-1/run_accept_20261116_2210/RUNBOOK.md`（382，同脚本 28 断言腿）。

卡面依据：`coord/cards/doing/2026-11-17-SCORE-BINDING-SAFE-1.md` 验收段——「生产装配真实栈选轨→edit→apply→目标回读与 J1–J3 exit 0，真栈候主管放行；单测可先行，不能记最终通过」。

## 0 前置门（全部满足才可跑；缺一即停，不得降级执行）

- **0.1 主管独立复核 + 合入裁定**：397 零栈交付 `c6991e1872de690b9c8976ec465366ff7c3cb1c0`（分支 `codex/score-binding-397`）已推但**尚未合入 main**（复核时 main HEAD=`f529e3f2`，`git merge-base --is-ancestor c6991e18 HEAD` exit 1）。执行子代理的独立复核见 `coord/runs/SCORE-BINDING-SAFE-1/local-397-review/REVIEW.md`（不代替主管裁定）。主管必须给出：①复核/验收结论；②运行候选=（合并后 main HEAD）或（书面授权的分支/commit）。
- **0.2 候选确定性**：主检出（`D:/Vit_DAW`）有未提交运行态（`CURRENT-STATE.md` / `VitApp/Workspace/Settings.xml` / `default_project.xml` / `tracktion_engine` 子模块 / 未跟踪 `.vit_derived/`）——不得扰动；若用 worktree 跑，候选 worktree 的 HEAD 必须=主管指定候选，且 **webui dist、agent 二进制均从同一候选源码构建**。候选 HEAD/status 写入 `head.txt`/`git_status.txt`。
- **0.3 候选零栈前置（复核时已过，真栈前在候选上重跑并留日志）**：`npm ci` / `npm run test`（717/717，59 files）/ `npm run build` 均 exit 0；`node --check scripts/score_workbench_smoke.mjs` exit 0；确认脚本迁移 diff 未弱化 A1–A8/J1–J3 断言（复核基线 c6991e18 已逐行核对，见 REVIEW §3.1）。
- **0.4 真栈占用授权**：按 `coord/PROTOCOL.md` §2.2 在 `coord/resources/PC-RUNTIME-STACK.md` 新增「占用中」行（卡 ID、owner、候选 HEAD、run 目录、端口面、§8 口径）→ push main → fetch 重读确认 owner → **≤60 秒内即时复核净空**（更早检查不得跨登记窗口沿用）。占用记录未成功同步，不得启栈。**本执行子代理零栈段不执行此步；由主管或其书面授权的 owner 执行。**
- **0.5 补充断言裁定**：主管决定是否先落 §7 的 S1–S4（脚本域小改，需授权）。不落则必须在回执按 §8 声明覆盖边界，且**不得**把本轮当作卡面「选轨→edit→apply→目标回读」完整通过。
- **0.6 基线哈希**：启动前记录 recents before 快照 hash、kernel/settings/boot/模板/候选 HEAD/agent/webui dist 哈希（§2/§5）。

## 1 目标与判据

在**生产装配**的真实栈（kernel+agent berth，不启 UI/Godot）上，由真实 Chromium 驱动生产 `/app/`：Workbench→谱面页签→**左侧轨道列表真实点击选轨**→clip picker/单 clip 自动 target→谱面面板**真实指针编辑**→「写入内核…」确认→agent/kernel 回读；并跑 J1–J3 旅程。

- **成功条件**：`dev_agent_smoke.ps1 ... -ScoreWorkbenchSmoke` **exit 0**，且 `score_workbench_summary.json` 的 `exit_code=0`、断言全 PASS：
  - A1–A8 十九断言（A1 load/write-readback、A2 id binding/stable、A3 symbol coverage、A4 双拍号、A5 saved-after-verify、A6 save→open→reload、A7 模板音源 edit-readback+probe non-silent、A8 WAV 差分）；
  - J1/J2/J3 九断言（工程面→工作台→谱面旅程、非 120bpm 面板装载回归、返回工程）；
  - 若主管授权 S1–S4，则其断言亦须全 PASS。
- 工件齐（§5）+ recents 还原哈希亲证 + TEARDOWN_ALL_ZERO（§6）。
- **生产路径纪律**：不得用测试后门写内核；A7/A8 编辑缝沿用 373 DG1-B B4 tool-face（历史 D4 边界，卡面指定判据形态，**不因 397 改变**）；其余编辑一律面板真实指针。

## 2 装置与 provenance

### 2.1 形态
kernel+agent berth（`-StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke`，不启 UI/Godot/VitShell）。端口面：agent HTTP 7878、kernel ZMQ 5555/5556、hub 8787、UDP 4445、CDP 9333（均仅 127.0.0.1）。断言驱动力=ps1 启动栈 + 腿内自起 Chromium headless。

### 2.2 二进制/资产（零内核改动只读复用）

| 项 | sha256 | 源 / 说明 |
|---|---|---|
| kernel `VitApp.exe` | `A59473614152882B21433DCEE684F8B13AE0BD5B632A97CD5E2C8765BB18348E` | `D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（2026-11-16 04:33 构建；374/377/382/384 同一二进制；397 零内核改动）。place-copy 到 `<run>/kernel_runtime/VitApp.exe`，跑前重算哈希 |
| 插件数据库 `Settings.xml` | `5ABF7D95C344D5F1CF8FF34E27EA72905F7209A16A7ABA40AA03FC21EBDAE076` | `D:/Vit_DAW/Export/staging/runtime/Workspace/Settings/Settings.xml`（424,113 B，含 Surge XT 描述符；只读拷贝到 `<run>/kernel_runtime/Workspace/Settings/Settings.xml`）。**缺库=Surg 不装载→P5-handles 环境中断（377 attempt2 教训）** |
| boot `default_project.xml` | 现场实测（c6991e18 复算 `4E899FB6ECA33A0FAD5003030FB884A169634765FC89C3C15FA7E6FC6E10B571`，11,609 B LF） | ps1 共享隔离块 `git show <candidate HEAD>:VitApp/Workspace/default_project.xml` 字节拷贝到 `<run>/default_project.xml` 并设 `VIT_PROJECT_XML`（ps1:1460-1512）。**382 runbook 记录的 `1F1AEAF9…` 与本次复算不一致（原因未查明），不沿用历史哈希，以运行时实测落工件** |
| P5 模板 `template.v1.xml` | manifest 校验（373 规范资产，382 记录 `0CB22BA3…21B3`） | `coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml`；脚本校验 manifest 后 place-copy 进 run 目录，源只读 |
| agent | ps1 现场 `go build -o agent/bin/VitAgent.dev-smoke.exe ./cmd/vitagent` 后安装；记录二进制 sha256/时间 | 被测源码=候选 worktree |
| webui dist | 候选 worktree `agent/webui`：`npm ci` → `npm run test` → `npm run build`；记录 `dist/index.html` 与 assets 哈希 | agent 以 `-WorkingDirectory agent/` 启动，`findWebUIRootFrom` 上溯到 `agent/webui/dist`（`agent/internal/chat/server.go:7235-7265`）。**禁止复用主检出/其他分支 dist** |
| Chromium | `agent/webui/node_modules/playwright` 1.65.0（Node 24） | 腿内 headless 自起 |

### 2.3 run 目录组装（新目录，不覆盖）
```
<run>/kernel_runtime/VitApp.exe
<run>/kernel_runtime/Workspace/Settings/Settings.xml
<run>/runtime_provenance.txt   # 候选 HEAD/status、各项 sha256、源路径、时间
<run>/head.txt、git_status.txt
<run>/global_project_config.before.json   # 由 ps1 ScoreWorkbenchSmoke 块生成（ps1:9151-9211）
<run>/ps1_console_full.txt                # 正式轮 console 全量
```

## 3 §8 口径（跑前冻结，AGENTS §8）

- **运行次数**：正式 **1 次单 run 无重跑**。装置域（probe_defect）修复后重跑须有新代码/装置 hash 变更并分记 attempt；**同代码版本同确定性断点连续失败 2 次→停手取证上交**，禁止原样重跑。
- **成功条件**：见 §1；以脚本 exit + summary 断言表 + 工件三件一致判定。
- **失败分类**：`env_interrupt`（exit 2：env-agent/env-browser/env-playwright/env-fixture/env-context/P5-kernel-zmq/P5-handles/recents-restore；分相位记录，须有原始日志/退出证据才可排除有效轮）｜`assertion_failure`/face_defect（exit 1：产品断言红，禁原样重跑）｜`probe_defect`（装置缺口，非产品，不计正式轮）。
- **阈值**（373 逐字沿用，脚本内冻结，summary.thresholds 留档）：`PROBE_MAX_ABS_MIN=0.01`；`WAV_NON_SILENT_PEAK=500`；`WAV_NON_SILENT_RMS=20.0`；`WAV_DIFF_MAX_MIN=500`。**不得改阈值/断言自愈**。
- **LLM 计费上界=0**：`-NoChatSmoke` + `$env:VIT_AGENT_LLM_BASE_URL="http://127.0.0.1:1"` 死端点 override + 全确定性 invoke/ZMQ/UI 面；以 telemetry 文件不存在/0 行亲证零调用。
- **止损**：总时长上限 60 分钟；环境故障两次→释放栈位、记录分类上交；同断点连败两次→停卡取证。止损只停投入，不改写未发生的成功。

## 4 步骤

### 4.0 预检（占用前；read-only）
```powershell
git -C <candidate worktree> rev-parse HEAD
git -C <candidate worktree> status --short
Get-Process VitApp,VitAgent,VspHub,Godot,Godot_console -ErrorAction SilentlyContinue
Get-NetTCPConnection -State Listen | ? LocalPort -in 7878,7879,8787,5555,5556,9333
Get-NetUDPEndpoint | ? LocalPort -eq 4445
(Get-FileHash "$env:APPDATA\Vit-DAW\global_project_config.json").Hash
```

### 4.1 占栈登记（§0.4）→ push main → 重读 → ≤60s 内复核 4.0 净空。

### 4.2 组装 run 目录（§2.3）并核对 kernel/Settings 哈希；构建候选 webui（§2.2）并记录 dist 哈希。

### 4.3 环境（同一 shell，agent 继承）
```powershell
$env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"   # 零 LLM 计费上界
```
确认 ps1 内建 recents 快照块生效（console 出现 `score wb recents snapshot: ... sha256=...`），`global_project_config.before.json` 已落 run 目录；缺失=环境前置失败，停。

### 4.4 正式轮（唯一正式运行）
```powershell
cd <candidate worktree root>
$run = "<candidate worktree root>\coord\runs\SCORE-BINDING-SAFE-1\real-stack\run_397_<yyyyMMdd_HHmmss>"
$env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke `
  -KernelExe "$run\kernel_runtime\VitApp.exe" `
  -RunArtifactsDir "$run" *> "$run\ps1_console_full.txt"
"exit=$LASTEXITCODE"
```
- 注意 1：`-ScoreWorkbenchSmoke` 是**非 scenario 腿**，ps1 只在 scenario berth 块自拆（`ps1:8226-8249`）；正式轮后 **owner 必须 finally 手动拆栈**，且先收工件再拆。
- 注意 2：`-SkipBuild` 仅在 §0.3 已亲证被测二进制时允许；默认现场 go build。
- 注意 3：不得加 `-StartUI`；不得启 VitShell/Godot（本腿形态与用户手测入口无关，用户手测裁定不变）。
- 注意 4：失败即按 §3 分类停；不得在失败后原样重跑。

### 4.5 结果判定
读 `<run>/score_workbench_summary.json`：`exit_code`、逐断言表、`p5_handles/music_context/p5_renders`、`failure_class/fail_phase/fail_reason`；与 ps1 exit 对照。PASS 需 28（+S*）断言全绿且 exit 0。

### 4.6 teardown（PASS/FAIL 都必须执行；finally 语义）
1. 收工件（§5，未拆栈前采集 kernel/agent 日志 tail、端口/进程证据）。
2. owner `Stop-Process` 停本 run 启动的 VitAgent/VitApp（记录 pid、时间、命令）。
3. **TEARDOWN_ALL_ZERO 亲证**：TCP 7878/7879/8787/5555/5556/9333 LISTEN=0 + UDP 4445=0 + Vit 系进程=0（逐项输出留档）。
4. **recents 还原**：ps1 ScoreWorkbenchSmoke 块内建还原+哈希亲证（console `score wb recents restored and hash-verified: <hash12>`）；**382 教训**：ps1 Strip Silence 段在 snapshot 窗口之外写 recents，故 owner 必须再以 `global_project_config.before.json` 比对/还原，记录 before/after sha256 一致；mismatch=装置非零退出按环境中断分类，不得静默。
5. 释放行写 `PC-RUNTIME-STACK.md`（run id、exit、分类、TEARDOWN_ALL_ZERO、recents hash）→ push main。

### 4.7 工件入库
新 run 目录（不覆盖）随候选分支入库；只入长期证据（§5），二进制/大 WAV 按先例不入库（报告记录字节数/哈希/差分即可）。

## 5 工件契约（AGENTS §9；关键结论必须能回指原始工件）

- run_id、起止时间、候选 HEAD + `git status --short`；
- kernel/Settings/boot/模板/agent/webui dist 哈希与来源；
- ps1 完整命令 + exit；`ps1_console_full.txt`、`score_workbench_console.txt`、`score_workbench_summary.json`；
- A1–A8/J1–J3（+S*）断言表与明细；`kernel_readback_final.json`、`melody_readback_before/after_edit.json`、`probe_before_edit.json`、`a7_probe_after_edit.json`；
- WAV 对 `render_before_edit.wav`/`render_after_edit.wav`（本地保留；summary 记 bytes/peak/rms/max_abs_diff）；
- 截图 p1/p2/p3/p4/j1/j2/j3（+S* 若实施）；
- recents before 快照 + after 还原哈希亲证；teardown 记录（pid/port/process=0）；
- attempt 台账（§9）。

## 6 端口/进程清理与异常

- 端口面见 §4.6；异常残留（未拆净）→ 资源记录保留 owner 并标「待处置」，按环境中断记录，不得交给下一位 owner 静默复用。
- 发现「already listening」不得静默复用（SMOKE-STACK-GUARD-1 前）；立即停，核对栈归属后按 §2.2 处置。

## 7 真栈覆盖缺口与最小补充断言建议（本段只列，不实施）

复核基线 `c6991e18` 的脚本迁移已把“选轨”变成生产前置，但 397 卡的“目标绑定”判据尚未全部进入真栈断言。详见 `../local-397-review/REVIEW.md` §4，摘要：

| 缺口 | 说明 |
|---|---|
| G1 | 无显式“UI 绑定身份==写入目标”断言（`data-score-clip-id`/`data-score-track-id`/身份条文本） |
| G2 | 无“非目标 clip/轨道前后不变”断言 → 不能直接证明旧 buffer 不写新目标 |
| G3 | dirty 切换 confirm/cancel/discard 未在真栈触发（仅零栈） |
| G4 | 多 clip 显式选择分支未覆盖（目标轨均单 clip） |
| G5 | 诚实失败态（读取失败/离线/空轨/音频轨）无真栈断言 |
| G6 | apply pending 锁 / 工程切换 generation 隔离真栈未覆盖（确定性触发成本高） |
| G7 | 未选轨时“无隐式 target”无真栈断言 |

最小补充断言（需主管授权后按脚本域小改或另开装置卡）：
- **S1**：选轨/选 clip 后断言 project-key/track-id/clip-id/身份条与目标一致；选轨前 `[data-score-root]` count=0。
- **S2**：A1 apply 后读 ≥1 个非目标 clip 前后 note set 不变 → 目标独占回读。
- **S3**：dirty 后切轨 → 断言 `[data-score-switch-confirm]`；cancel 保身份/模型；discard 切换且写入计数不增；再切回继续 A1。
- **S4**：目标轨第二个 MIDI clip → 断言多 clip 时未显式选择不装载、picker 只列 midi。
- S5/S6（失败态、并发回包）不纳入本轮：保留零栈覆盖+边界声明（确定性/成本不划算）。

## 8 覆盖边界声明（回执必填；未落 S1–S4 时尤其重要）

本轮真栈只能证明：生产装配下的选轨前置 + edit→apply→目标 kernel 回读（A1/A5/A2）+ A3/A4/A6/A7/A8 + J1/J2/J3。**不证明**：目标身份的显式断言、非目标不变、dirty 切换真栈路径、多 clip 显式选择真栈路径、TOCTOU 消除（无 project CAS）、用户手测通过。不得把零栈 vitest/render 结果写成端到端。另：`scripts/webui_rendered_dom_smoke.mjs`（E2E-WEBUI-1 面）在 397 后存在 score 段未迁移选轨前置的高置信回归风险（未实跑验证），真栈后需另行核对/另卡，不得以本卡 PASS 推定该门槛仍绿。

## 9 attempt 台账模板（§8 失败分类）

```
- attempt N（run_397_<stamp>，起止）＝<env_interrupt@phase | assertion_failure | probe_defect | PASS exit 0>
  - 依据/证据：<console line refs / summary.fail_phase / pid/ports>
  - 根因与变更：<装置/代码/环境；无变更不得原样重跑>
  - 是否正式轮：<是/否>
```

## 10 授权状态

**本 RUNBOOK 为草案，当前禁止运行。** 待主管：①完成对 c6991e18 的独立复核并裁定合入/候选；②决定 S1–S4 是否先落（§7）；③按 §2.2 完成真栈占用登记；④书面放行本轮。放行后在候选 worktree 按 §4 执行，回执按 §5/§8 交付。

---
（参考先例：`coord/runs/SCORE-DG1A-COMPLETE-1/RUNBOOK.md` 377；`coord/runs/SCORE-ENTRY-REACH-1/run_accept_20261116_2210/RUNBOOK.md` 382；零栈复核 `coord/runs/SCORE-BINDING-SAFE-1/local-397-review/REVIEW.md`。）
