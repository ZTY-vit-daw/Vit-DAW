# SHELL-WIN-COMBO-1 执行计划（主管预写，2026-11-18 晚）

- 前置：410/411/412 合入并验收；413 完成最终产物刷新（VitShell.exe / fresh extension DLL / VitAgent / webui dist）。
- 真栈单 owner=本主管会话；本计划不是运行授权，运行前必须先推 `coord/resources/PC-RUNTIME-STACK.md` 预留并重读，再 ≤60 秒即时净空复核。
- 条件：默认 **shell 自然前台**（VitShell 自身前台，用户无需操作）；external 自然前台腿本轮不跑（409 已单独覆盖），若用户在场并明确配合可另裁。

## 复用装置

- 基线：`scripts/vitshell_zproduct_smoke.ps1`（409 单次产品验收 companion）+ `vitshell_zproduct_observer.cs` + `vitshell_zproduct_verdict.mjs`。
- 组合卡最小扩展（主管实施、评审在卷）：
  1. 参数化卡 ID：新增 `-CardId`（默认 `SHELL-WIN-ZPRODUCT-1`），用于报告、stage marker（`.shell409-used` 或按卡改）与 lease 断言中的卡 ID；409 默认行为不变。
  2. 395 呈现断言：新增 CDP phase（可加 `scripts/vitshell_combo_cdp.mjs` 或扩展 `vitshell_open_chain_cdp.mjs` 可选 phase），在 click-open 后轮询真实 DOM：
     - `[data-window-launch-feedback]` 在 click 后出现，`data-window-launch-request` 非空；
     - `data-window-launch-seq` 单调 ≥1；`data-window-launch-progress="unknown"`（无假进度）；
     - 终态 `data-window-launch-terminal` ∈ {`ready`,`accepted`}；`ready`=402 接缝生效，`accepted`=如实降级（记录覆盖缺口，不伪 PASS）；
     - LoadingCard 步骤标签来自真实 phase（至少 1 条，可与 probe 时点捕获）；
     - `[data-native-focus-hint]`：shell 条件下应为缺席（负例）；正例（DAW 前台）留给用户手测/另条件，边界如实声明。
  3. 409 原生断言与分类器不变；`classification_input.json` 增加 395 DOM 段，供判决脚本一并输出。
- 396 retry 腿（`scripts/ui_entry2_smoke.ps1`）为 legacy Godot 入口回归，与本产品路径 harness 不同源；本轮默认**不并入**单次栈，若用户要求 396 收口则另开同 owner 串行腿并单独登记。此边界在 414 回执声明。

## Stage 准备（每次 run 全新，不复用）

- `StageRoot=%TEMP%/shell414-combo-<hex>/`，必须无 reparse point。
- `StageRoot/host/`：413 产出的 `VitShell.exe` + `WebView2Loader.dll`。
- `StageRoot/runtime/`：`agent/bin/{VitAgent,VspHub}.exe`、`agent/webui/dist/`、`VitApp/build/VitApp_artefacts/Release/`、`VitApp/Source`、`VitApp/CMakeLists.txt`（根标记）；staged 时间戳须新于 canonical fallback。
- `StageRoot/frontend/`：合并后前端 main 的 `git archive HEAD` 解包（无 .git）+ fresh extension DLL 覆盖 + 运行 `--headless --import` 生成隔离缓存（记录 import exit 与耗时）。
- 证据：`staging_manifest.json` 记录各文件 SHA；run 目录 `coord/runs/SHELL-WIN-COMBO-1/<stamp>/`。

## 运行预算与止损

- 单次 ≤10 分钟（含 teardown）；零 LLM（telemetry 非空即 FAIL）；产品断言/装置/环境/覆盖分类分开。
- 失败即停；同一确定性断点两次→停转诊断；不自动连续跑。
- 卫生：fixture 副本、recents 快照/finally 哈希、六份权威输入哈希、栈释放亲证。

## 通过后

- 主管复核原始工件 → 出 414 ruling（必要时补证/返工）→ 刷新实际入口产物（413 已就绪）→ 放行 391 用户手测 v7。
- 手测边界：窗口不抢焦点/在对话窗后 + 真实装载反馈可见 + DAW 前台时对话窗轻量提示；导出包/稳定性不在本轮结论。
