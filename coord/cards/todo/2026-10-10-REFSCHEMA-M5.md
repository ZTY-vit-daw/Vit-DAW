# REFSCHEMA-M5：E 类回执族归一（semantic_eq_batch / c2.dynamic_plugin_load_batch）+ R5 去留裁定落账

- 发卡：GLM 主管决策侧（/morning 会话）/ 2026-10-10
- 派发确认：已确认（用户 2026-10-10 /morning 裁定标准日+机会面清理（gate 建议序③））
- 验收负责人：GLM 主管决策流
- 池序 41（[G1 终审 M5 行](../../runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md) §4 + [M8 报告 R5](../../runs/REFSCHEMA-M8/report.md)）；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5 天 / **REFSCHEMA-M4 合入后领取**（refschema.go 文件域相交，串行）
- 模型分级：L1 / flash 可接（M4 同型先例；本卡为 M4 之后的第二批注册+归一）

## 范围裁定（主管预裁定，随卡生效）

- **G1 终审 M5 原文**：E 类回执（semantic_eq_batch/c2.dynamic_plugin_load_batch）legacy → 同 M4（进 EvidenceRefs 边界处归一）。
- **R5 去留预裁定（M8 报告新输入，主管裁定）**：`mix-report:` / `mixboard-project:`（harness.go:4548-4549 + chat/mixboard_decision_projection.go:100）——纯响应面 ref 形态字段、**无下游读取者**（M8 全库 grep 实证）、不进 EvidenceRefs 面 → **登记不动作**（不注册翻译条目：无解析消费方，注册徒增词条噪音）。本卡执行腿在 G1 终审记录 M5 行回写时一并落此注记。

## 已核实锚点（领取时回查行号；M8 报告 2026-10-09 实读）

1. `semantic_eq_batch`：`capabilityadapters/semantic_eq.go`、`chat/c1_frequency_cleanup_runtime.go`、`chat/b4_eq_runtime.go`（发卡侧 grep 非测试命中）。
2. `dynamic_plugin_load_batch`：`chat/c2_dynamic_batch.go`（发卡侧 grep 唯一非测试命中）。
3. 注册表：`agentprotocol/refschema.go:130-159`（M4 合入后词条数前移，以当期为准）。

## 目标

1. **消费链核实**：两族是否进入 EvidenceRefs/解析面（M8 附录 grep 口径复跑+全库消费方清点，清单入回执）。
2. 进面 → 按 M2/M4 型边界归一；纯响应面 → 注册翻译条目即完成（identity/回执族承载：含批序号与时间成分 → slot=snapshot 建议款，TargetKind 留空，证据矛盾触发停止条件）。
3. **G1 终审记录回写**：M5 行落地注记+R5 去留裁定注记（不动作依据=零消费方）。

## 文件域

`internal/agentprotocol/refschema.go` + `internal/capabilityadapters/semantic_eq.go` + `internal/chat/{c1_frequency_cleanup_runtime,b4_eq_runtime,c2_dynamic_batch}.go`（按消费链核实结果定改面）+ 新增测试。**不碰** harness.go:4548-4549 与 mixboard_decision_projection.go:100（R5=不动作）。

## 语义红线

同 M4：vit:// 文法零改动、既有词条零改动、legacy 往返回归、fail-closed 不变。

## 验收标准

`go build ./...` + `go test ./internal/agentprotocol ./internal/capabilityadapters ./internal/chat -count=1` 全绿 + 全量 `go test ./... -count=1` 0 FAIL + 触碰文件 blob 级 gofmt 净；新词条解析测试+legacy 往返回归；消费链清单入回执。

## 停止条件

- 两族实际进入 EvidenceRefs 的位点跨出上述文件域 → 清单上交。
- R5 字段在领取时点出现下游读取者（与 M8 实证相反）→ 停止上交（去留裁定重开）。

## 并行与资源

- 与 HYGIENE 两卡文件域零重叠；**依赖 M4 先合入**（串行领取）。不需要真栈；无资源占用。

- 领取：（时间 / origin/main hash / owner 模型+机器+会话 / 分支 / worktree / 领取提交）
- 回执：（commit hash / 消费链清单 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
