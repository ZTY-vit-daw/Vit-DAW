# 397 集成续段 — 零栈复核 + 跨卡适配 + 真栈 S1 增强（证据/说明）

- 日期：2026-11-18 晚（PC）；执行：`lead/score397stack`（DeepSeek-V4.1-Flash 子代理；主管 `lead`）。
- 集成 worktree：`C:/Users/timoz/.codex/worktrees/pc397-merge/Vit_DAW`；分支 `codex/score-binding-397-integ`；基线 main `f529e3f2`；**合并提交 `df0e134b`**（干净合并 397 的 `c6991e18`+`c17bf4bf`，无冲突）。本续段在其上做适配并形成本地 commit（hash 见回执/HEAD）。
- **边界=零真栈**：未启动 VitApp 内核、Go agent、Godot、VitShell；未登记/占用 PC-RUNTIME-STACK；未 push；未改卡态/主检出/397 原 worktree。真实浏览器只用于**零栈 Vite bundle + HTTP fixture** 的自检 probe（无 agent/kernel），属 397 render-smoke 同款零栈装置。

## 1 集成候选零栈三门（本 worktree 亲跑）

| 命令 | exit | 结果/证据 |
|---|---:|---|
| `npm ci` | 0 | added 170 packages；`logs/npm-ci.log`（sha256 D1AFAFAD…DB9EE） |
| `npm run test` | **0** | **61 files / 733 tests passed**（无 unhandled error/FAIL）；原始日志 17,746,267 B sha256 `2AA06F8CF236BB9BFAAF4C1685222FB2787AEAA54334E771BD140E66A5AB4C0F` → `logs/full-vitest.log.gz` |
| `npm run build` | 0 | tsc --noEmit 0 错 + vite built（3.01s）；`logs/build.log`；dist/index.html sha256 `7F3C027042522BB2B1D0B85BF54E83F49D0602363ADA676663654D221C7F9178` |
| `node --check scripts/score_workbench_smoke.mjs` | 0 | S1 增强后语法 |
| `node --check scripts/webui_rendered_dom_smoke.mjs` | 0 | 397 适配后语法 |
| 零栈 probe（见 §3.2） | 0 | 2/2 变体 PASS |

计数说明：733 = 397 分支的 717 + 集成分支基线 main 带来的 2 个测试文件/16 项（733−717=16）；不是 397 断言弱化/虚增。397 自身断言仍全绿。

## 2 跨卡回归修复：`scripts/webui_rendered_dom_smoke.mjs` score 段

**风险**（397 独立复核发现）：397 后谱面编辑器改为“显式目标”语义——未选轨/选 clip 时 `[data-score-root]` 不挂载。E2E-WEBUI-1 的 score 段原来在点 score 页签后直接 `waitForSelector("[data-score-root]")`（原 5128 行），会在合并分支上 15s 超时。
**场景**：该 smoke 是“真实 agent、无内核”装置（`/agent/ui/state` 无可用 MIDI clip；374 记录的状态为 `loadStatus=内核装载失败：... there are no user-visible clips`），397 后诚实形态=绑定容器 + 轨道列表 + 明确无目标提示，不再自动挂面板。

**适配（最小，+75/−109 行；保留 fail-honest 语义）**：
- 点 score 页签后等 `[data-score-binding-root]`（397 新容器）并等待 catalog/poll 稳定；
- 读取真实 `/agent/ui/state` 判定诚实空态（`project_path` 可得但无 MIDI 目标 / 无工程身份），断言：
  - **S1e**（原 S1 等价）：绑定容器在场；**无隐式编辑目标**——`[data-score-root]`=0、`[data-score-svg]`+`[data-score-overlay]`=0、`[data-score-apply]`=0（原意=“不得假装已装载/可编辑的内核态”，现由更严格的 fail-closed 断言承接）；
  - **S2e**（原 S2 等价）：诚实无目标态在场——有工程身份时 `[data-score-binding-empty]` 含“选择左侧轨道”或轨道列表含“还没有轨道”；无工程身份时 `role=alert` 含“工程身份不可得/轨道读取失败”；轨道列表真实渲染。原意=“渲染面有真实内容且不伪造状态”，谱面纸面本体改由 `agent/webui/src/scoreworkbench/render-smoke.mjs`（真实 target fixture）覆盖；
  - **S3e**（原 S3–S7 等价/转移）：原真实指针放置/时长/apply 失败/放弃/游标断言**需要已绑定 target**，本零内核/无轨道装置不可能执行（不是弱化，是不适用）；这些语义已由 `render-smoke.mjs`（真实 bundle+fixture target，含 pointer placement/apply target/failed load）与真栈 DG1-A A1–A6 覆盖，且在 report notes 中显式记录，不静默丢弃；本段额外断言 **refresh 后仍无伪造目标**（`[data-score-root]`=0 且容器在）。
- 脚本头/报告 `score_workbench_source` 同步更新；原 S0 tab 断言保留。

**等价性逐条**（回执要求）：
| 原断言 | 原意 | 397 后适配 | 等价/转移 |
|---|---|---|---|
| S1 | 面板挂载 + 桥 connected + 装载失败诚实显形 | S1e：容器在场 + 无隐式目标/无编辑/无 apply | 原意=不可用时不伪造可编辑态；新断言更严格（不再先挂后失败，而是 fail-closed） |
| S2 | 本地 2/4 默认模型/谱纸渲染 | S2e：无目标诚实文案/身份错误 + 轨道列表在场 | 纸面渲染转 `scoreworkbench/render-smoke.mjs`（真实 fixture target）；零目标时不渲染纸面本身即正确语义 |
| S3–S7 | 真实指针放置/时长/apply 失败保缓冲/放弃回基线/无游标 | 不适用（无 target）；S3e 只断言 refresh 不伪造目标 | 全部转移至 `scoreworkbench/render-smoke.mjs`（pointer/apply/discard）与真栈 A1–A6（apply/readback/discard 基线） |

## 3 真栈 smoke 最小增强（只加 S1）

### 3.1 `scripts/score_workbench_smoke.mjs`（+63 行，未改任何既有断言/阈值）
断言面 28 → **33**（+5 S1）：
- `S1-ui-binding-P1`（`openScorePanel`，选轨/选 clip 装载后；line ~636/658）
- `S1-ui-binding-P4-reopen`（同 helper，A6 重开后面板重载）
- `S1-ui-binding-P5-D4-regression`（同 helper，J2 非 120bpm 模板面板）
- `S1-ui-binding-J1-journey`（J1 旅程选旋律轨/clip 后；line ~1049/1064）
- `S1-apply-target`（P2 真实 apply 后；line ~738）

判定内容：
1. 选轨前 `[data-score-root]` count=0（无隐式目标）；
2. `[data-score-root].data-score-loaded-target` = `[project 归一化路径, track_id, clip_id]`，`data-score-clip-id` 与 `data-score-write-ready="true"` 同时成立；
3. P2 通过 `page.on("request")` 观察真实 `/agent/invoke` 载荷：`tool=apply_midi_note_patch`、`args.track_id/clip_id` == 同一目标、`time_unit=beats`、`confirmed=true`——即 **UI 绑定身份 == 实际写入目标**（与 kernel 回读目标一致）。

### 3.2 零栈自检 probe（未入库可执行件；结果入库）
临时脚本（已删除，仅本目录留结果）用 **Vite dev server + Chromium + HTTP fixtures**（无 agent/kernel）验证适配后的 score 段选择器/文案/逻辑，两个变体均 PASS：
- 变体 A：`project_path='P:/fixture/empty-project.vit'` + `tracks: []` → 容器/轨道列表在场、无隐式 root、空态文案“选择左侧轨道…/工程中还没有轨道”、refresh 后仍诚实；
- 变体 B：`project` 无 `project_path` → `role=alert` 含“轨道读取失败：工程身份不可得…”，其余同上。
证据：`probe/score-section-probe-report.json`（pass=true）+ `probe/score-no-tracks.png` + `probe/score-no-project.png` + `probe/probe.log`。

## 4 boot `default_project.xml` 哈希差异归因（可查项已查明）

- ps1 共享隔离块用 `cmd /c git show <HEAD>:VitApp/Workspace/default_project.xml` 把 **blob 字节**写入 `<run>/default_project.xml`。集成分支该 blob sha256 = `4E899FB6ECA33A0FAD5003030FB884A169634765FC89C3C15FA7E6FC6E10B571`（11,609 B，LF）。
- 382 runbook 记录的 `1F1AEAF9…8D488` **不是**这个文件：在 382 run 目录的磁盘复核显示 `run_accept_20261116_2210/kernel_runtime/Workspace/default_project.xml` = 2,060 B / `1F1AEAF9…`（内核工作目录自带的 default project），而 `run_accept_20261116_2210/default_project.xml`（ps1 隔离拷贝）= 11,788 B / `947E4D00…`（CRLF 文件系统拷贝形态；同 run 2217 相同）。
- 结论：差异原因=**同名不同文件 + 拷贝/行尾形态混淆**（382 RUNBOOK provenance 行指代错位），不是内容漂移。本续段 runbook 已分别列两个文件，并要求运行时实测、不沿用历史哈希。

## 5 工件索引

- `logs/npm-ci.log` / `logs/build.log` / `logs/full-vitest.log.gz`（17,746,267 B 原始日志压缩）+ 本文件 §1 的 sha256
- `adapt-scripts.diff`（本次两个 smoke 的完整 diff；sha256 `34ECA473BD597D58B728F9F4CB40E9458AB833812C1C52FCF6E7A18F8DED7632`）
- `probe/score-section-probe-report.json` + 2 截图 + `probe/probe.log`
- 真栈 runbook（更新）：`../real-stack/RUNBOOK.md`
- 397 零栈复核：`../local-397-review/REVIEW.md`

## 6 边界 / 待主管

- 本段仍未运行 E2E-WEBUI-1 的 agent 实跑面（需要真实 VitAgent + Chromium；零栈批次不启）。适配后的 score 段须由该门槛 owner 在 agent 实跑面复跑，确认 E2E-WEBUI-1 复绿；**不得以本卡真栈 PASS 推定其已绿**。
- 真栈轮仍待主管：复核集成分支 + 裁定候选 + 真栈占用登记 + 书面授权；S2–S4 真栈判据未实施（见 runbook §7.2/§8）。
- 零栈 733/61、build 0 只证明合并后单元/构建面无回归，不证明真实栈与用户手测。
