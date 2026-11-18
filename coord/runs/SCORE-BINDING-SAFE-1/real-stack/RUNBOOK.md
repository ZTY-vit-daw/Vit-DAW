# RUNBOOK（草案）— SCORE-BINDING-SAFE-1（397）真栈验收腿

**状态：草案 / 未授权运行。** 编制：`lead/score397stack`（DeepSeek-V4.1-Flash 执行子代理；主管 `lead`），2026-11-18 晚，PC。
初版基于 `codex/score-binding-397`@c6991e18；**2026-11-18 续段在集成分支 `codex/score-binding-397-integ`（基线 main `f529e3f2`，合并 c6991e18+c17bf4bf，merge commit `df0e134b`）上完成 smoke 适配（真栈 S1 断言 + E2E-WEBUI-1 score 段 397 适配）并同步更新本文件。**
**本文件编制期间零真栈**（未启动 VitApp/agent/Godot/VitShell/真实 Chromium、未登记占用、未改生产代码）。
**只有同时满足 §0 全部门槛并由主管书面放行后，才可按 §4 启动真栈。** 模板参考：`coord/runs/SCORE-DG1A-COMPLETE-1/RUNBOOK.md`（377 DG1-A）与 `coord/runs/SCORE-ENTRY-REACH-1/run_accept_20261116_2210/RUNBOOK.md`（382）。

卡面依据：`coord/cards/doing/2026-11-17-SCORE-BINDING-SAFE-1.md` 验收段——「生产装配真实栈选轨→edit→apply→目标回读与 J1–J3 exit 0，真栈候主管放行；单测可先行，不能记最终通过」。

## 0 前置门（全部满足才可跑；缺一即停，不得降级执行）

- **0.1 主管独立复核 + 合入裁定**：397 零栈交付 `c6991e18` + 零栈复核/runbook 草案 `c17bf4bf` 已由主管合入集成分支 `codex/score-binding-397-integ`（基于 main `f529e3f2`，合并提交 `df0e134b`，无冲突）。**运行候选 = 该集成分支的最终 commit**（含本段适配提交；运行期以 `git rev-parse HEAD` 落工件）。主管必须给出：①对集成分支的复核/验收结论；②书面授权运行候选与真栈轮；③（如需）是否把该分支合入 main 后从 main 的 worktree 运行。分支未获授权不得启栈。
- **0.2 候选确定性**：主检出（`D:/Vit_DAW`）仍有未提交运行态，不得扰动；**本轮候选 worktree = `C:/Users/timoz/.codex/worktrees/pc397-merge/Vit_DAW`**（集成分支）。候选 HEAD/status 写入 `head.txt`/`git_status.txt`；**webui dist、agent 二进制均从该候选源码构建**，禁止复用主检出/397 分支 dist。
- **0.3 候选零栈前置（本续段已在集成分支亲跑，真栈前重跑留日志）**：`npm ci` exit 0；`npm run test` **61 files / 733 tests passed** exit 0；`npm run build` exit 0（dist/index.html sha256 见 `coord/runs/SCORE-BINDING-SAFE-1/integ-20261118/ADAPTATION.md`，运行期重构建后以新值记录）；`node --check scripts/score_workbench_smoke.mjs` 与 `node --check scripts/webui_rendered_dom_smoke.mjs` 均 exit 0；S1 断言已落地（见 §7/§1：真栈断言面 28+5=33）。
- **0.4 真栈占用授权**：按 `coord/PROTOCOL.md` §2.2 在 `coord/resources/PC-RUNTIME-STACK.md` 新增「占用中」行（卡 ID、owner、候选 HEAD、run 目录、端口面、§8 口径）→ push main → fetch 重读确认 owner → **≤60 秒内即时复核净空**。占用记录未成功同步，不得启栈。**本执行子代理零栈段不执行此步；由主管或其书面授权的 owner 执行。**
- **0.5 S2–S4 裁定**：S1（UI 绑定身份==实际写入目标）**已在集成分支落地**（33 断言面）无需再裁；S2（非目标 clip 不变）、S3（dirty 切换守卫真栈腿）、S4（多 clip 显式选择真栈腿）仍为建议、本段未实施——主管决定是否先落（脚本域小改，需授权）或接受 §8 覆盖边界。**不得**把未实施项的缺失写成“已覆盖”。
- **0.6 基线哈希**：启动前记录 recents before 快照 hash、kernel/settings/boot/模板/候选 HEAD/agent/webui dist 哈希（§2/§5）。

## 1 目标与判据

在**生产装配**的真实栈（kernel+agent berth，不启 UI/Godot）上，由真实 Chromium 驱动生产 `/app/`：Workbench→谱面页签→**左侧轨道列表真实点击选轨**→clip picker/单 clip 自动 target→谱面面板**真实指针编辑**→「写入内核…」确认→agent/kernel 回读；并跑 J1–J3 旅程。

- **成功条件**：`dev_agent_smoke.ps1 ... -ScoreWorkbenchSmoke` **exit 0**，且 `score_workbench_summary.json` 的 `exit_code=0`、断言全 PASS（**33 断言** = A1–A8 十九 + J1/J2/J3 九 + S1 五）：
  - A1–A8 十九断言（A1 load/write-readback、A2 id binding/stable、A3 symbol coverage、A4 双拍号、A5 saved-after-verify、A6 save→open→reload、A7 模板音源 edit-readback+probe non-silent、A8 WAV 差分）；
  - J1/J2/J3 九断言（工程面→工作台→谱面旅程、非 120bpm 面板装载回归、返回工程）；
  - **S1 五断言（本段新增）**：`S1-ui-binding-P1`、`S1-ui-binding-P4-reopen`、`S1-ui-binding-P5-D4-regression`、`S1-ui-binding-J1-journey`、`S1-apply-target`——每次生产选轨/选 clip 装载后断言 `[data-score-binding-root].data-score-project-key`/`[data-score-root].data-score-loaded-target`/`data-score-clip-id`/`data-score-write-ready` 与目标（project 归一化路径 + track id + clip id）一致且**选轨前 `[data-score-root]` count=0（无隐式目标）**；P2 真实 apply 的 `/agent/invoke` 载荷 `track_id/clip_id/time_unit/confirmed` 必须等于同一目标（并与 kernel 回读目标一致）。
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
| boot `default_project.xml`（ps1 隔离块拷到 run 根） | 运行期实测：ps1 用 `git show <candidate HEAD>:VitApp/Workspace/default_project.xml` 字节拷贝，当前集成分支复算 = `4E899FB6ECA33A0FAD5003030FB884A169634765FC89C3C15FA7E6FC6E10B571`（11,609 B，LF）。**382 runbook 的 `1F1AEAF9…` 差异已查明=指代错位，见下行与 §7 注** | 拷到 `<run>/default_project.xml` 并设 `VIT_PROJECT_XML`（ps1:1460-1512）。ps1 `cmd /c git show` 输出 blob 字节（LF，11,609 B）；若用文件系统 `Copy-Item` 拷工作树则该文件为 CRLF 11,788 B（sha256 `947E4D00…`）。**运行时以 ps1 产物实测哈希落工件** |
| boot 归因（382 记录澄清） | `1F1AEAF9…8D488` 实为 **`<run>/kernel_runtime/Workspace/default_project.xml`（2,060 B，内核工作目录自带 default project）**，不是 ps1 git-clean boot 拷贝 | 证据：382 run 目录在盘复核——`run_accept_20261116_2210/default_project.xml` 11,788 B `947E4D00…`；同 run `kernel_runtime/Workspace/default_project.xml` 2,060 B `1F1AEAF9…`。382 RUNBOOK 的 provenance 行把两者混为一行；本 runbook 分别列出，运行期分别记录 |
| P5 模板 `template.v1.xml` | manifest 校验（373 规范资产，382 记录 `0CB22BA3…21B3`） | `coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml`；脚本校验 manifest 后 place-copy 进 run 目录，源只读 |
| agent | ps1 现场 `go build -o agent/bin/VitAgent.dev-smoke.exe ./cmd/vitagent` 后安装；记录二进制 sha256/时间 | 被测源码=候选 worktree（集成分支） |
| webui dist | 候选 worktree `agent/webui`：`npm ci` → `npm run test` → `npm run build`；记录 `dist/index.html` 与 assets 哈希（本续段构建记录：`7F3C0270…F9178`，运行期重构建后以新值覆盖） | agent 以 `-WorkingDirectory agent/` 启动，`findWebUIRootFrom` 上溯到 `agent/webui/dist`（`agent/internal/chat/server.go:7235-7265`）。**禁止复用主检出/397 分支 dist** |
| Chromium | `agent/webui/node_modules/playwright` 1.65.0（Node 24） | 腿内 headless 自起 |

### 2.3 run 目录组装（新目录，不覆盖）
```
<run>/kernel_runtime/VitApp.exe
<run>/kernel_runtime/Workspace/Settings/Settings.xml
<run>/runtime_provenance.txt   # 候选 HEAD/status、各项 sha256（含 run 根 default_project.xml 与 kernel_runtime/Workspace/default_project.xml 分别）、源路径、时间
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
读 `<run>/score_workbench_summary.json`：`exit_code`、逐断言表（**应有 33 条目**）、`p5_handles/music_context/p5_renders`、`failure_class/fail_phase/fail_reason`；与 ps1 exit 对照。PASS 需 33 断言全绿且 exit 0。

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
- kernel/Settings/boot（run 根拷贝与 kernel_runtime/Workspace 副本分别）/模板/agent/webui dist 哈希与来源；
- ps1 完整命令 + exit；`ps1_console_full.txt`、`score_workbench_console.txt`、`score_workbench_summary.json`；
- **33 断言表**（A1–A8 十九 + J1–J3 九 + S1 五）与明细；S1 失败时须含 `expected/loaded/selected/write-ready/root-before` 与 apply 载荷；
- `kernel_readback_final.json`、`melody_readback_before/after_edit.json`、`probe_before_edit.json`、`a7_probe_after_edit.json`；
- WAV 对 `render_before_edit.wav`/`render_after_edit.wav`（本地保留；summary 记 bytes/peak/rms/max_abs_diff）；
- 截图 p1/p2/p3/p4/j1/j2/j3（含选轨身份可见的 p1/j1）；
- recents before 快照 + after 还原哈希亲证；teardown 记录（pid/port/process=0）；
- attempt 台账（§9）。

## 6 端口/进程清理与异常

- 端口面见 §4.6；异常残留（未拆净）→ 资源记录保留 owner 并标「待处置」，按环境中断记录，不得交给下一位 owner 静默复用。
- 发现「already listening」不得静默复用（SMOKE-STACK-GUARD-1 前）；立即停，核对栈归属后按 §2.2 处置。

## 7 断言面、覆盖缺口与跨卡适配（本段状态）

### 7.1 已落地（本续段，集成分支）
- **S1（+5 断言）**：在 `scripts/score_workbench_smoke.mjs` 的 `openScorePanel` 装载后（P1 / P4-reopen / P5-D4-regression 三相位）与 J1 旅程装载后断言 UI 绑定身份；P2 真实 apply 后断言 `/agent/invoke apply_midi_note_patch` 载荷目标。实现位置（集成分支 c6991e18 之上的适配提交，行号以该提交为准）：页面创建处注册 request 观察者 + `assertUiBinding` 帮助函数（`openScorePanel` 之上）；`openScorePanel` 内 `rootBefore` 与装载后断言；P2 `S1-apply-target`；J1 流程 `journeyRootBefore` 与 `S1-ui-binding-J1-journey`。**未改任何既有断言/阈值**。
- **E2E-WEBUI-1 score 段 397 适配**（跨卡回归修复，`scripts/webui_rendered_dom_smoke.mjs`）：原“点 score 页签→等 `[data-score-root]`→面板自动装载”契约已被 397 显式目标语义退役；改为等 `[data-score-binding-root]` 并断言**诚实无目标态**（S1e/S2e/S3e，原意/等价性见 `coord/runs/SCORE-BINDING-SAFE-1/integ-20261118/ADAPTATION.md` 与脚本内注释）。面板编辑/apply/discard/cursor 断言不再适用该零内核/无轨道装置，覆盖转移至 `agent/webui/src/scoreworkbench/render-smoke.mjs`（fixture target，11 断言）与真栈 A1–A6。**该 smoke 需由 E2E-WEBUI-1 owner 在 agent 实跑面复跑复绿（本零栈段未执行 agent/真实浏览器栈）；不得以本卡真栈 PASS 推定其已绿。**

### 7.2 真栈仍未覆盖（S2–S4 建议；本段未实施）
| 缺口 | 说明 |
|---|---|
| S2 | 无“非目标 clip/轨道前后不变”断言 → 未直接证明旧 buffer 不写新目标 |
| S3 | dirty 切换 confirm/cancel/discard 未在真栈触发（仅零栈 vitest/render） |
| S4 | 多 clip 显式选择分支未覆盖（fixture 目标轨/模板旋律轨均单 clip） |
| — | apply pending 锁 / 工程切换 generation 隔离（零栈覆盖+边界声明）；诚实失败态（零栈覆盖+边界声明） |

若主管需要 S2/S3/S4 真栈判据，须先授权脚本域小改并重跑 §0.3；否则按 §8 边界交付。

## 8 覆盖边界声明（回执必填）

本轮真栈能证明：生产装配下的**显式选轨（真实点击）× 目标绑定身份 == 实际写入目标**（S1：project/track/clip + apply 载荷）+ edit→apply→目标 kernel 回读（A1/A5/A2）+ A3/A4/A6/A7/A8 + J1/J2/J3。**不证明**：非目标 clip 不变（S2）、dirty 切换真栈路径（S3）、多 clip 显式选择真栈路径（S4）、apply pending/工程切换并发回包真栈路径、TOCTOU 消除（`/agent/ui/state` 仅 `project_path`，无 project_uuid/CAS）、用户手测通过。不得把零栈 vitest/render 结果写成端到端。跨卡说明：E2E-WEBUI-1 score 段已做 397 适配（S1e/S2e/S3e）但未在本零栈段实跑，须由其 owner 复跑复绿，不得以本卡 PASS 推定。

## 9 attempt 台账模板（§8 失败分类）

```
- attempt N（run_397_<stamp>，起止）＝<env_interrupt@phase | assertion_failure | probe_defect | PASS exit 0>
  - 依据/证据：<console line refs / summary.fail_phase / pid/ports>
  - 根因与变更：<装置/代码/环境；无变更不得原样重跑>
  - 是否正式轮：<是/否>
```

## 10 授权状态

**本 RUNBOOK 为草案，当前禁止运行。** 待主管：①复核集成分支 `codex/score-binding-397-integ`（含本段适配提交）并裁定合入/候选；②决定 S2–S4 是否先落（§7.2）；③按 §2.2 完成真栈占用登记；④书面放行本轮。放行后在候选 worktree 按 §4 执行，回执按 §5/§8 交付。

---
（参考先例：`coord/runs/SCORE-DG1A-COMPLETE-1/RUNBOOK.md` 377；`coord/runs/SCORE-ENTRY-REACH-1/run_accept_20261116_2210/RUNBOOK.md` 382；零栈复核 `coord/runs/SCORE-BINDING-SAFE-1/local-397-review/REVIEW.md`；本续段适配说明 `coord/runs/SCORE-BINDING-SAFE-1/integ-20261118/ADAPTATION.md`。）
