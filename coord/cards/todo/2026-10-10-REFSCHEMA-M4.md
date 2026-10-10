# REFSCHEMA-M4：B 类身份族 capabilitycontext 批次承载定裁+翻译条目注册（ccbobs_/cap_pack_/ccbr_/ccbr_rejected_）

- 发卡：GLM 主管决策侧（/morning 会话）/ 2026-10-10
- 派发确认：已确认（用户 2026-10-10 /morning 裁定标准日+机会面清理（gate 建议序③，按 M8 报告新输入重排））
- 验收负责人：GLM 主管决策流
- 池序 40（[G1 终审 M4 行](../../runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md) §4 + [M8 报告 R6](../../runs/REFSCHEMA-M8/report.md)）；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5 天 / 无（main a28eb1b0）
- 模型分级：L1 / flash 可接（M1/M2/M3 同型先例：注册表数据新增+边界归一+对照测试）

## 范围裁定（主管预裁定，随卡生效）

- G1 终审 M4 行原文= B 类身份族（obs_/ccbobs_/rel_/cap_pack_/c2_plan_）"随各域触碰机会"归一。本卡**只做 capabilitycontext 批次**（M8 R6：与 ccbobs_/cap_pack_ 同批定承载，避免单条先定）；obs_/rel_/c2_plan_ 维持"随域触碰"余量，不入本卡。
- **承载预裁定**（执行侧核实证据后落地，矛盾即触发停止条件）：
  - `ccbobs_`（CCB 观察 id，identity 含时序）→ 对齐 `observation:` 先例语义：slot=snapshot、TargetKind 留空（identity 族非内容哈希）。
  - `cap_pack_`（能力包 id，identity）→ 同上款 slot=snapshot、TargetKind 留空。
  - `ccbr_` / `ccbr_rejected_`（compactID=观察 id+请求 id 哈希短串，内容寻址）→ 对齐 M8 R4 pcr1_ 建议邻域：ObservationFingerprint 族+slot=hash、TargetKind 留空。
  - **最长匹配核验**：`ccbr_rejected_` 必须先于 `ccbr_` 命中（注册表匹配序为最长匹配，实读 `matchLegacyPrefix` 确认两词条共存的命中行为并加测试）。

## 已核实锚点（领取时回查行号）

1. `capabilitycontext/free_state_observation.go:567`（ccbr_）/`:439`（ccbr_rejected_）——M8 报告 D4 实读。
2. `capabilitycontext/pack.go`——cap_pack_ 生成点（发卡侧 grep 唯一非测试命中）。
3. ccbobs_ 生成点在 `capabilitycontext/free_state_observation.go`（发卡侧 grep 唯一非测试命中）。
4. 注册表：`agentprotocol/refschema.go:130-159`（15 条 legacy 词条形态=LegacyPrefix/Family/TargetKind/Slot/Anchor）。

## 目标

1. **消费链核实**：三族四前缀的 refs 是否进入 EvidenceRefs/解析面（M8 附录 grep 口径复跑+全库消费方清点，清单入回执）。
2. **注册表新增词条**：按承载预裁定注册（含 Anchor 注记本卡+M8 报告来源；注释块风格对齐 M1/M2 追加段）。
3. **进 EvidenceRefs 面的位点**：按 M2 型边界归一（翻译为 vit:// 或 legacy 可翻译语义落地）；纯响应面（无解析消费）则注册即完成——两形态如实分记入回执。
4. **G1 终审记录回写**：M4 行标注 capabilitycontext 批次落地+余量清单（obs_/rel_/c2_plan_）。

## 文件域

`internal/agentprotocol/refschema.go` + `internal/capabilitycontext/`（按消费链核实结果定改面）+ 新增测试文件。预期 ≤5 文件；消费链核实若发现归一面超此域 → 锚点清单上交，不扩域私改。

## 语义红线

vit:// 文法零改动；既有 15 词条零改动；旧记录 legacy refs 必须可解析（往返回归）；fail-closed 语义不变。

## 验收标准

`go build ./...` + `go test ./internal/agentprotocol ./internal/capabilitycontext -count=1` 全绿 + 全量 `go test ./... -count=1` 0 FAIL + 触碰文件 blob 级 gofmt 净；新词条解析测试（含 ccbr_/ccbr_rejected_ 最长匹配）+ legacy 往返回归；消费链清单入回执。

## 停止条件

- 承载证据与预裁定矛盾（如 ccbobs_ 实为内容哈希、ccbr_ 实含时间戳/随机量）→ 该族单独上交（升 G1 终审记录缺口）。
- 族成员进入 EvidenceRefs 的位点跨出 capabilitycontext/agentprotocol 域 → 清单上交裁定。

## 并行与资源

- 并行域 B（capabilitycontext+agentprotocol）；与 HYGIENE-FASTPATH-1 / HYGIENE-GOFMT-1 零重叠。**与 REFSCHEMA-M5 相交于 refschema.go——M5 依赖本卡合入后领取（串行）**。不需要真栈；无资源占用。

- 领取：（时间 / origin/main hash / owner 模型+机器+会话 / 分支 / worktree / 领取提交）
- 回执：（commit hash / 消费链清单 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
