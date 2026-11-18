# 397 SCORE-BINDING-SAFE-1 零栈独立复核（lead/score397stack）

- 日期：2026-11-18 晚（PC）；复核 owner：`lead/score397stack`（DeepSeek-V4.1-Flash 执行子代理，主管 `lead` 派发）。
- worktree：`C:/Users/timoz/.codex/worktrees/score-binding-397/Vit_DAW`；分支 `codex/score-binding-397`；复核基线 `c6991e1872de690b9c8976ec465366ff7c3cb1c0`（父 `bb10c48b`）；复核开始时 `git status --short` / `git diff --stat` 空（tracked clean）。
- **本段边界=零真栈**：未启动 VitApp 内核、Go agent、Godot、VitShell、真实 Chromium 栈腿；未登记/占用 PC-RUNTIME-STACK；未改生产代码、卡态或其他工作树；未 push。所有命令在本 worktree 内执行。真实栈与 J1-J3 本轮**未运行**，判据与装置见同目录 `real-stack/RUNBOOK.md`（草案，待主管复核合入+授权后才可运行）。

## 1 交付面核对（独立复算，不采信交付回执数字）

`git show c6991e18 --stat` 复算：**26 文件，957+/149−**，与卡面/回执一致。分域：

| 域 | 文件 | numstat |
|---|---|---|
| 生产（卡面固定 5 文件） | `scoreworkbench/ScoreTrackWorkbench.tsx`（新） | +120 |
| | `scoreworkbench/binding.ts`（新） | +64 |
| | `scoreworkbench/ScoreWorkbenchPanel.tsx` | +135/−127 |
| | `scoreworkbench/scoreworkbench.css` | +38 |
| | `workbench/Workbench.tsx` | +4/−10 |
| 测试 | `scoreworkbench/ScoreTrackWorkbench.test.tsx`（新） | +190 |
| | `scoreworkbench/ScoreWorkbenchPanel.test.tsx` | +10/−8 |
| 零栈渲染装置 | `scoreworkbench/render-smoke.mjs`（新） | +124 |
| 真栈脚本（卡面授权迁移） | `scripts/score_workbench_smoke.mjs` | +19/−4 |
| 执行证据 | `coord/runs/SCORE-BINDING-SAFE-1/local-397/**`（17 文件：10 文本 + 1 gz + 2×3 PNG） | +253（PNG/gz 为二进制不计行） |

生产文件合计 **361+/137−**，恰好 5 个，全部位于卡面固定实现域 `agent/webui/src/`。5 文件 sha256（复核时 working tree=commit 内容）：

```
2832E21D59ED01142F9FDD2A38D2DA9B7AF8D9D60A66C97E3AFF4B296DBE6174  ScoreTrackWorkbench.tsx (8001 B)
D8A0751E3271AC149ABD6A15CFE4CAF1422328AB06CEF7DF6FE8F3459F275888  binding.ts (2641 B)
4D29009811F346E6E8E8A7E099B5F1A421972A14B9953CAD0B9E64C47A63CAD6  ScoreWorkbenchPanel.tsx (28319 B)
3AB6EAE47F18178B6135CEBA7AD75A2C4E86858619355D5182396C1621C3C4D8  scoreworkbench.css (5870 B)
27DF1C39C42BDD6FF643997C2E55529CCA2B533F3133B8B2BCB609A3EBD697AC  Workbench.tsx (5921 B)
```

禁止域核对：`git show --name-only c6991e18` 中**无** `VitApp/`（内核）、无 `agent/*.go`（agent 后端）、无 `App/launcher/project`、无 Godot 前端仓文件。→ 生产文件域与卡面一致。

## 2 亲跑命令与退出码

全部在 worktree `agent/webui/`（另注者除外），2026-11-18 19:59–20:0x +0800。原始日志在 `logs/`（完整性见 `logs/logs-integrity.json`）。

| 命令 | exit | 关键输出 / 证据 |
|---|---:|---|
| `npm ci` | 0 | added 170 packages in 6s；`logs/npm-ci.log` |
| `npm run test`（clean evidence run） | **0** | **59 files / 717 tests passed**；duration 5.10s；无 unhandled error；原始日志 18,343,472 B（sha256 C5DC9780…590F）→ `logs/full-vitest.log.gz` |
| `npm run test -- src/scoreworkbench/ScoreTrackWorkbench.test.tsx` | 0 | **11/11 passed**；`logs/focused-11.log` |
| `npm run build` | 0 | `tsc --noEmit` 0 错 + `vite build`（1790 modules, built 3.02s）；dist/index.html sha256 `2C6050BF…222D`；`logs/build.log` |
| `node src/scoreworkbench/render-smoke.mjs local-397-review/render` | 0 | `status=PASS`，**11 assertions**，`errors=[]`；报告+3 截图在 `render/` |
| `node --check scripts/score_workbench_smoke.mjs`（根目录） | 0 | 迁移后脚本语法 |
| `node --check agent/webui/src/scoreworkbench/render-smoke.mjs`（根目录） | 0 | 新渲染装置语法 |
| `git diff --check bb10c48b c6991e18` | 2（仅空白告警） | 仅 2 个**证据日志**文件“new blank line at EOF”（`local-397/binding-vitest*.log`）；生产/测试文件零空白错误。见 §5 备注 |

### 2.1 vitest 两次执行装置失败（如实分记，不隐藏）

- **attempt1**（首次运行，用 PowerShell `2>&1 | Tee-Object` 落日志）：**exit 1**，但 59 files/**717 tests 全部 passed**；唯一错误=vitest unhandled error `[vitest-worker]: Timeout calling "snapshotSaved"`，无任何 `FAIL` 测试文件。原始日志 37,340,218 B（sha256 21B52043…2936）→ `logs/harness-failures/vitest-attempt1-timeline.log.gz`。
- **attempt2**（同管道复跑）：**exit 1**，14 suite FAIL；全部错误= `[vitest-worker]: Timeout calling "fetch" with "<file>.tsx","ssr"`（模块装载 RPC 超时），522 tests passed，**零断言级 FAIL**。原始日志 37,341,062 B（sha256 4FB83C6F…74CB）→ `logs/harness-failures/vitest-attempt2-timeline.log.gz`。
- 分类：**环境/执行装置面（本复核的 PowerShell 日志管道在 37MB 输出下挤占 vitest worker RPC），非产品缺陷、非测试失败**。同 worktree、同 node_modules、零代码变更下，改为 `cmd /c "npm run test > log"` 直写后 **exit 0 / 717 passed / 零 unhandled error**（§2）。按 AGENTS §11：已保存首次失败原始输出、隔离复跑排除与当前 diff 的关联；两失败轮不记为产品结论。未出现同确定性断点连败的场景（失败类型为装置 RPC 超时，修复=换日志方式，非原样重跑）。

### 2.2 715 vs 717 计数核对（防止虚报/漏报）

- 交付出 `local-397/full-vitest.log.gz`（原始 18,468,125 B，sha256 `ef57146a…c076b7`）解压复算 = **59 files / 715 tests**，其中 `ScoreTrackWorkbench.test.tsx (9 tests)`。
- 当前提交内该测试文件已是 **11 个 it**（final 追加 2 个反例），故当前全量 = **717**；715+2=717 自洽。回执明写“未将全量计数虚报为 717”，与实际一致，**未见计数造假**。

## 3 断言/实现弱化核查（读 diff，不只跑绿）

### 3.1 `scripts/score_workbench_smoke.mjs`（+19/−4）
- diff 仅两处前置：`openScorePanel`（P1/P2/P4/P5-D4 共用）与 J1 旅程入口改为——等 `[data-score-binding-root]` → 读 `/agent/ui/state` 定位目标 clip 所属轨道 → **真实点击** `.score-track-item[data-score-track-id]` → 若 picker 存在则 `selectOption(wanted)` → 等 `[data-score-root]`。
- **A1-A8 十九断言、J1/J2/J3 九断言的断言语句/阈值零改动**：逐行 diff 核对无 `assert(` 删除或条件放宽；PROBE/WAV 四阈值（0.01/500/20.0/500）原样；无跳过分支新增。
- 结论：属卡面授权的“控件断言迁移”，未以等待/点击替代判据，**无弱化**。

### 3.2 `ScoreWorkbenchPanel.test.tsx`（+10/−8）
- 变更=给既有 17 个用例的 mount 增加 `target` prop（面板新契约）；1 个用例（`apply error keeps the buffer and surfaces the error`）装载桩从“reload 失败”改为“成功装载后 apply 失败”，使“写入失败保留缓冲”在新语义下成立；断言集未减（`apply-state=error` + status 含错误 + `dirty=true` 全部保留）。
- 用例数 **17→17**，无删除。

### 3.3 新增 focused 11 反例 ↔ 卡面 meaningful 反例
1. 显式选轨/仅列 MIDI/空轨·音频·未知类型诚实显示（test 1）；
2. dirty A→B 取消保身份；放弃后 B 装载失败不写旧 buffer（test 2）；
3. apply pending 禁切换+禁编辑（test 3）；
4. 工程切换后延迟 load 回包隔离（含相同 track/clip id 折叠，test 4）；
5. 删除/改类型撤销写入 + 恢复后不覆盖脏缓冲（test 5）；
6. apply 前即时新鲜状态检查（test 6）；
7. 状态接口失败保脏+锁写（test 7）；
8. 只写显式目标 + 成功回读后 rebase（test 8）；
9. catalog 刷新不跟随 GUI 选轨（398 边界，test 9）；
10. 重复确认只写一次/拒绝保缓冲（test 10）；
11. 缺工程身份 fail-closed、不推断 MIDI、`targetInCatalog` 跨工程 false（test 11）。
- 结论：覆盖卡面反例清单，未以“不崩溃”等弱断言替代身份/写入断言；全量 717 无回归。

### 3.4 生产实现复核要点（5 文件，语义级）
- `binding.ts`：身份键 = `/agent/ui/state.project.project_path` 归一化（无 project_uuid，边界已在回执披露）；仅 `clip_type=midi` 可编辑；`targetInCatalog` 跨工程/失效 fail-closed。
- `ScoreTrackWorkbench.tsx`：左轨列表与当前编辑身份常驻；选择归属工作台（不跟随 GUI=398 边界）；单 MIDI clip 自动 target、多 clip 显式选择；dirty 切换确认；busy 禁切；轮询“下一次等上一次结束”。
- `ScoreWorkbenchPanel.tsx`：不再自拉 clip 清单，target 由外层注入；generation/ref 令牌隔离 load/apply 过期回包；`ready = loadedKey===targetKey && targetValid` 才放行编辑/写入；apply 前后 `validateTarget`；失败保留脏缓冲；新增 `data-score-write-ready` / `data-score-loaded-target` 断言面。桥的 confirmed replace_region + 回读验证语义未改。
- `scoreworkbench.css` / `Workbench.tsx`：轨道栅格/切换遮罩/窄屏；Workbench 改挂 `ScoreTrackWorkbench`。
- 未发现自动 apply、跨目标写入、把 GUI 选轨当编辑目标的路径。

### 3.5 备注（非阻塞）
- `git show --check` exit 2 仅因提交内两份证据日志文件末行为空行（`local-397/binding-vitest*.log`），不影响代码。
- smoke 脚本 P1 注释仍写 “panel auto-load”，与 397 后显式选轨语义不符（注释过期，不影响断言）。

## 4 真实栈验收缺口分析（对照 `scripts/score_workbench_smoke.mjs` 现状）

真栈判据（卡面）=生产装配“选轨→edit→apply→目标回读 + J1–J3”。现有 berth 腿（`-StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke`，28 唯一断言：A1–A8 十九 + J1/J2/J3 九）覆盖：

- **选轨**：397 迁移后 P1/P2/P4/J1/J2 前置已走生产轨道按钮 + clip picker（真实事件，无测试后门）。
- **edit→apply→读回**：A1-load-equality / A1-write-readback / A5-saved-only-after-verify / A2-id-binding / A2-id-stable-edit-apply 在 fixture 工程上经真实指针编辑→确认 apply→agent 读回内核音符集等值；A6 覆盖 save→open→再读回+面板重载。
- **J1–J3**：J1 工程面→工作台→谱面页签→面板装载（80bpm 模板）；J2 非 120bpm 面板装载/网格披露/无 pageerror；J3 返回工程面。
- A7/A8：模板工程音源判据（编辑缝=373 B4 tool-face，历史 D4 边界，非本卡引入）。

**缺口（对 397 卡判据）**：

| 编号 | 缺口 | 影响 |
|---|---|---|
| G1 | 无“选轨身份/写入目标==UI 绑定 target”的显式断言（`data-score-clip-id`/`data-score-track-id`/身份条文本） | “目标回读”靠 fixture 唯一性间接成立，不是直接判据 |
| G2 | 无“非目标 clip/轨道前后不变”断言 | 不能直接证明“旧 buffer 不写新目标” |
| G3 | dirty 切换 confirm/cancel/discard 未在真栈触发（仅零栈 vitest/render） | 397 核心安全切换判据缺真栈腿 |
| G4 | 多 clip 显式选择分支未覆盖（fixture 目标轨/模板旋律轨均为单 clip，`selectOption` 分支大概率不执行） | “多 clip 显式选择”仅零栈证据 |
| G5 | 诚实失败态（读取失败/离线/空轨/音频轨）无真栈断言 | 仅零栈证据（可保留边界声明） |
| G6 | apply pending 锁 / 工程切换 generation 隔离真栈未覆盖（确定性触发成本高） | 建议维持零栈覆盖+边界声明 |
| G7 | 未选轨时“无隐式 target”无真栈断言（`[data-score-root]` count=0） | 可随 S1/S4 顺带补 |

**最小补充断言建议（本段只列，不实施；需主管授权后按脚本域小改或另开装置卡）**：

- **S1（+3~4 assert）**：`openScorePanel` 内选轨/选 clip 后断言：`[data-score-binding-root]` 的 `data-score-project-key` == ui_state.project_path 归一化且非空；`[data-score-editing-target]` 的 `data-score-track-id` == 目标轨；`[data-score-root]` 的 `data-score-clip-id` == 目标 clip；且点击页签后、选轨前 `[data-score-root]` count==0。
- **S2（+1 assert）**：P2 apply 后除目标 clip 读回外，再读 fixture 工程内 ≥1 个其它 clip（boot demo 既有 clip 或另建对照 clip）前后 note set 不变 → 证明目标独占。
- **S3（+4 assert）**：P2 dirty 后点击另一 MIDI 轨按钮 → 断言 `[data-score-switch-confirm]` 出现；cancel 后 clip-id/模型不变；再切 discard → 目标切换且写入计数不增；随后切回原目标继续 A1（放在 P2 之后、P4 之前，避免扰动 A1 读数）。
- **S4（+2 assert，需 fixture 增建）**：目标轨再插 1 个 MIDI clip，断言该轨选中后 `[data-score-root]` count==0、picker 只列 midi、显式选择后才装载。（改 fixture 会扰动 P1 前置读数，替代=新增独立小 phase；需主管确认。）
- S5/S6（诚实失败态、并发回包）**不建议**纳入本轮：确定性/成本不划算，保留零栈+边界声明。

**裁定口径建议**：若主管不采纳 S1–S4，本轮最多按既有 28 断言判定，**不得据此认定 397 卡“选轨→edit→apply→目标回读”完整通过**，须在真栈回执显式声明：目标身份、非目标不变、dirty 切换、多 clip 显式选择仅零栈证据。

## 5 供主管裁定的风险/前提

1. **合入前提**：main 当前 HEAD `f529e3f2` **不含** c6991e18（`git merge-base --is-ancestor c6991e18 HEAD` exit 1）。主检出还有未提交运行态（`CURRENT-STATE.md`/`VitApp/Workspace/Settings.xml`/`default_project.xml`/`tracktion_engine` 子模块/未跟踪 `.vit_derived/`）——真栈前由主管决定运行候选（合并后 HEAD 或书面授权分支），并把候选 hash 写入 runbook/工件；不得扰动主检出运行态。
2. **身份协议边界**：`/agent/ui/state` 仅披露 `project_path`，无 `project_uuid`/`epoch`；实现者已如实声明“前端写前/写后校验不能提供内核级 project CAS，不能取消已发出的请求”。真栈轮只能证明目标绑定/fail-closed，不能证明 TOCTOU 消除；若需该保证应另开后端协议卡。
3. **跨卡脚本回归风险（高置信、未实跑验证）**：`scripts/webui_rendered_dom_smoke.mjs`（E2E-WEBUI-1 渲染面门槛）第 5128 行在点击 score 页签后直接 `waitForSelector("[data-score-root]")`，且无选轨前置；397 后未选轨时面板不挂载 → 该脚本 score 段大概率 15s 超时失败。本段不启 agent 实跑验证（零栈边界），建议主管：①在真栈轮前/后另开装置域小改或核对该门槛；②不得把 397 真栈 PASS 当作 E2E-WEBUI-1 仍绿。同类引用仅此一处（其余 `[data-score-root]` 引用均在 `score_workbench_smoke.mjs` 内，已迁移）。
4. **boot 工程哈希口径**：382 runbook 记录 `default_project.xml` sha256 `1F1AEAF9…`；本次对 c6991e18 与 main HEAD 的 `git show HEAD:VitApp/Workspace/default_project.xml` 复算均为 `4E899FB6…B571`（11,609 B LF）。差异原因未查明（不据此判定历史 run 失效）；真栈时应以 ps1 现场 `git show` 产物实测哈希落工件，不沿用历史哈希。
5. **零栈结论不能替代真栈/用户手测**：本报告所有通过项均为局部（vitest）或零栈渲染（HTTP fixture）级；不得写成端到端、真实保存或用户手测通过。

## 6 工件索引

- `logs/full-vitest.log.gz`（clean 全量 717，原始 18,343,472 B）+ `logs/logs-integrity.json`
- `logs/focused-11.log` / `logs/build.log` / `logs/npm-ci.log` / `logs/render-397.log`
- `logs/harness-failures/vitest-attempt{1,2}-timeline.log.gz`（执行装置失败原始日志，分类见 §2.1）
- `render/render-smoke-report.json` + `render/screens/{desktop-editing,failed-load,narrow}.png`（11 断言）
- `real-stack/RUNBOOK.md`：真栈 RUNBOOK 草案（待主管复核合入+授权后才可运行）
