# REFSCHEMA-M4B：ccbr_ 族三前缀承载重裁落地（ccbr_/ccbr_rejected_/ccbr_batch_ 注册）

- 发卡：GLM 主管决策侧（验收会话）/ 2026-10-10
- 派发确认：已确认（依据=[2026-10-10 MORNING-BATCH rulings §3 ccbr_ 族重裁](../../rulings/2026-10-10-MORNING-BATCH-rulings.md)：identity 族→slot=snapshot、TargetKind 留空，对齐 ccbobs_ 同款；M8 报告 D4「哈希短串」描述失准记注——行为证据优先）
- 验收负责人：GLM 主管决策流
- 池序 43；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5h / REFSCHEMA-M4 已合入（main ≥51a389c8）；**M5 在本卡合入后领取**（refschema.go 相交串行链 M4→M4B→M5）
- 模型分级：L0 / **flash 可接**（M4 同型小额：注册表数据新增+测试翻面）

## 目标（四件）

1. 注册三词条：`ccbr_`/`ccbr_rejected_`/`ccbr_batch_` → Family/Slot/TargetKind 与 ccbobs_ 三形态逐款对齐（evidence_scheme_uri / snapshot / 留空）；Anchor 注记 M4B+重裁依据；注释块说明重裁缘由（compactID 首个非空透传实证，M8 D4 描述失准）。
2. 翻钉住测试：`TestRefSchemaM4CCBRFamilyStaysUnregistered` 改为断言三前缀**已注册且翻译形态正确**（前缀包含对 ccbr_/ccbr_rejected_/ccbr_batch_ 最长匹配，与 ccbobs_ 族互不劫持）。
3. 注册表初值表测试扩 19→22。
4. G1 终审记录 M4 行批注更新：ccbr_ 族缺口闭合（重裁=identity→snapshot，落地 commit 待回填）。

## 文件域

`agent/internal/agentprotocol/refschema.go` + `refschema_test.go` + `refschema_m4_test.go` + `agent/internal/capabilitycontext/refschema_m4_anchor_test.go`（锚点测试保持，透传语义证据不动）+ G1 记录。≤5 文件。

## 语义红线

vit:// 文法零改动；既有 19 词条零改动；legacy 往返回归；cap_pack_/ccbobs_ 行为零变化。

## 验收标准

`go build ./...` + `go test ./internal/agentprotocol ./internal/capabilitycontext -count=1` 全绿 + 全量 `go test ./... -count=1` 0 FAIL + 触碰文件 blob 级 gofmt 净（机械格式化等价证明口径=HEAD blob==gofmt(父 blob)+CR=0）。

## 停止条件

领取时发现 ccbr_ 族生成点/remainder 形态与 M4 回执证据不符（如生成点已变）→ 锚点+形态上交。

## 并行与资源

不需要真栈；无资源占用；与 PROBE-METER-RECON-1（只读）可并行。

- 领取：2026-10-10 18:15 / origin/main `7dff62a0`（本地 main 同点；M4 基线 51a389c8 已在 ancestor 核对通过）/ owner=GLM-5.3-Flash 执行会话 PC（ZCode flash 会话）/ 分支 `port/refschema-m4b` / worktree `D:/Vit_DAW_wt_refschema_m4b`（origin/main 干净检出，工作树零叠加 diff）/ 领取提交=本提交
- 停止条件预核：ccbr_ 三生成点锚点实测 free_state_observation.go :567/:439/:1636，与 M4 回执证据一致，remainder=compactID 首个非空透传形态未变——未触发。
- 回执：实现 commit `a62f7125`（port/refschema-m4b，已推远待验收合入）/ 翻面测试名 **`TestRefSchemaM4BCCBRFamilyRegistered`**（原 TestRefSchemaM4CCBRFamilyStaysUnregistered 已移除）/ 端测边界声明=纯单测域（注册表数据+解析器），卡面明示不需要真栈、无资源占用，未触碰真实运行栈，无渲染面/用户旅程改动；验收全绿（build 0、定向两包 ok、全量 92 包 0 FAIL exit 0、blob CR=0 gofmt 净）——详见 coord/runs/REFSCHEMA-M4B/receipt.md（四件目标逐条对照+红线核查+G1 缺口闭合批注回写）
- 验收：（裁定文件 / 验收 commit）
