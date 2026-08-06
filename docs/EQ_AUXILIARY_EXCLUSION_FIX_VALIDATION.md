# EQ 辅助参数污染回退修复与兼容性验证

日期：2026-07-28
分支：`codex/eq-structural-matrix-unification`

## 结论

本次修复把 Dynamic/Threshold/Attack/Release/Range/External Sidechain 等同频段辅助参数从主静态 EQ section 的 exclusion 判据中移除。它们仍作为只读辅助事实存在，但不再阻断同 section 中相互独立的 Frequency/Gain/Q/Shape/Activation 控制核心。

真正属于 Sidechain/Detector/Gate/DeEsser component 的 section 仍保持排除；stateful surface 与非 linked channel mode 的既有拒绝也未改变。

结果：历史 16 capture 从 14 recognized / 12 executable 恢复到 14 / 14；Waves 50 保持 44 / 38；所有对比均为零回退。TDR Nova、FreeEQ8、Pro-Q 3、F6 Stereo 的真实 Godot → Kernel → Agent 控制烟测全部通过。

## 范围

本次只处理 exclusion 作用域：

- 修改 `agent/internal/workflows/plugingrabber/eq_structural_matrix.go` 中的 `applyEQSectionExclusions`。
- 更新相应结构测试，覆盖辅助兄弟参数不污染主 EQ core，以及真实 owned dynamic/sidechain component 仍拒绝。
- 新增离线 replay before/after 比较工具 `scripts/compare_eq_replay_reports.py`。
- 更新 `scripts/eq_compat_matrix_smoke.py`，兼容 atomic EQ 响应把 `actual_readback` 放在 `edits[].actual_readback` 的现行 schema，并确保所有写后校验均处于参数恢复的 `try/finally` 内。

未做 LinEQ shape 变体 tie-break、Marvel ISO 推断、schema 重构、已退役映射链路、B4、插件名称生产分支或 Plugin Alliance 参数读取。

## 根因与修复

原实现会扫描一个 section 的全部兄弟参数。只要发现名称含 Dynamic/Threshold/Range/Sidechain 等词，就把 `dynamic_section_excluded` 或 `sidechain_or_detector_section_excluded` 写到整个 section；随后这些 code 被复制到该 section 的每种 shape/action capability。

这使 Pro-Q 3、FreeEQ8、TDR Nova、F6 等已经完整形成静态 Frequency/Gain/Q/Shape core 的 section，仅因旁边存在动态辅助字段而整体失去执行能力。

修复后的作用域规则是：

- section key 自身属于 `SC`、`Sidechain`、`Detector`、`Gate` 时，排除该 component；
- section key 自身属于 `DeEsser` 时，排除该 component；
- 同 section 的 Dynamic/Threshold/Range/Attack/Release/External Sidechain 兄弟参数不再把 exclusion 扩散到静态 EQ core；
- stateful surface 和非 linked channel mode 的全局安全拒绝保持原样。

## 离线 replay 结果

### 历史 16 capture

- 修复前：14 recognized / 12 executable
- 修复后：14 recognized / 14 executable
- 回退：0
- 扩展：4

恢复内容：

- TDR Nova：恢复 Bell、Low Shelf、High Shelf；Low Cut、High Cut 保持。
- F6 Stereo：恢复 Bell、Low Shelf、High Shelf；Low Cut、High Cut 保持。
- FreeEQ8：恢复 Bell、Low Shelf、High Shelf、Low Cut、High Cut。
- Pro-Q 3：恢复五种公开 shape，并保持 `free_floating` 与 remove 能力。
- PuigTec EQP1A 与 compressor 负例继续拒绝。

证据：

- `artifacts/eq_auxiliary_exclusion_fix/20260728/historical_diff.md`
- `artifacts/eq_auxiliary_exclusion_fix/20260728/historical_diff.csv`
- `artifacts/eq_auxiliary_exclusion_fix/20260728/historical_after.json`

### Waves 50 capture

- 修复前：44 recognized / 38 executable
- 修复后：44 recognized / 38 executable
- 回退：0
- 扩展：F6 Stereo 与 F6 RTA Stereo 的 Bell/Shelf capability
- phase-2 expectation：1250 项，0 regression，37 expansion

证据：

- `artifacts/eq_auxiliary_exclusion_fix/20260728/waves_diff.md`
- `artifacts/eq_auxiliary_exclusion_fix/20260728/waves_diff.csv`
- `artifacts/eq_auxiliary_exclusion_fix/20260728/waves_after.json`

## 真实烟测

同一四个对象各执行一次 Bell upsert：3400 Hz、-3 dB、Q 0.5。测试通过真实 Godot → VitApp Kernel → VitAgent 链，不使用音频主动探测。

| 对象 | 识别结构 | 参数总数 | 结果 |
|---|---|---:|---|
| TDR Nova | `fixed_slot_adjustable` | 78 | 控制、真实读回、完整恢复通过 |
| FreeEQ8 | `fixed_slot_adjustable` | 129 | 控制、真实读回、完整恢复通过 |
| Pro-Q 3 | `free_floating` | 348 | slot activation、控制、真实读回、完整恢复通过 |
| F6 Stereo | `fixed_slot_adjustable` | 94 | 控制、真实读回、完整恢复通过；动态辅助参数未触碰 |

最终报告：`artifacts/eq_compat_matrix/20260728_011728/capture/summary.json`，状态 `ok`、failures 为空。

第一次运行目录 `artifacts/eq_compat_matrix/20260728_011036` 的四个“失败”是 smoke harness 仍读取旧的顶层 `actual_readback` 所致。新鲜参数读回证明四次写入实际成功。随后使用运行前全参数快照逐参数恢复四个插件，四者 `full_snapshot_mismatches=[]`，再关闭仅由该次运行启动的 PID；用户原有 Godot PID 19160 未受影响。修正 harness 后对同一四对象复验通过，没有增加烟测对象。

最终运行还确认：

- 自动恢复后所有被测插件回到运行前参数状态；
- `default_project.xml`、`Settings.xml`、`bridge_last_dev.log` 在最终运行前后 byte-identical；
- 本次链的 5555、5556、7878、8787 监听均已退出；
- 用户原有 Godot PID 19160 保持运行。

## 自动测试

以下检查均通过：

- `go test ./internal/workflows/plugingrabber`
- `go test ./cmd/eqtopologyreplay`
- `go test ./internal/workflows/plugingrabber ./internal/chat ./internal/agentloop ./internal/tools ./cmd/eqtopologyreplay`
- 对应包 `go vet`
- `go test ./...`
- `python -m py_compile scripts/eq_compat_matrix_smoke.py`
- `git diff --check`

## 后续讨论项

LinEQ 同一公共 shape 对应多个内部枚举的 current-variant-preserving tie-break 仍是独立的静默正确性问题，应另开范围处理。Marvel ISO 频率锚点推断按约定继续搁置。本次没有借修回退之名扩展这两项。
