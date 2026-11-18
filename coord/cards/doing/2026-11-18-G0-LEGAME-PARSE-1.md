# G0-LEGAME-PARSE-1（W01-A）：乐局设计源解析与语义校验门

- 池序 423；P0；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚 R1–R6 裁定后指令继续开发；契约 v0-frozen）。
- owner=`lead/legameparse423`（DeepSeek-V4.1-Flash，PC）；分支 `codex/legame-parse-423`；worktree=`C:/Users/timoz/.codex/worktrees/pc423-parse/Vit_DAW`（主仓）；基线=`origin/main`（契约已回填 v0-frozen）。
- 权威：`docs/LEGAME_DESIGN_CONTRACT_V0.md`（v0-frozen，§3/§4/§5/§7）；设计源实例 `coord/lesson-designs/music-creation-unit2/design.v1.yaml`；卡片草案 `coord/runs/G0-LEGAME-CONTRACT-1/impl-cards-draft.md` W01-A。

## 目标

新包 `agent/internal/legamedesign/` 实现 `vit.legame_design.v0` 的 Parse+Validate，输出 E/W/I 聚合诊断；不改任何现有包行为。

## 要点（契约已冻结，按 §4/§5 实现）

1. YAML `KnownFields(true)` fail-closed；`schema_version` 必填；只认 `vit.legame_design.v0`，未知值拒绝；预留 `vit.project_intent.v0` 仅登记（不实现非乐局解析路径）；**启用判定不在本包**（R2：项目包元数据，归 W01-C）。
2. 按 §4 全字段落结构体；L1 五必填段非空检查；§5.2 规则号逐条实现 E-SCHEMA/E-REQ/E-REF/E-MAT(path)/E-CAP(stub)/W-CHK/W-MAT/I-NOTE；R5：tempo/budget 缺失→E（不注入缺省）；R6a：key_signature 只收 30 调枚举；R6b：music_region 单/列表归一 list；R3/R4 的映射语义不在本卡实现（B 卡）。
3. 诊断聚合输出：规则号前缀+字段路径+人读提示，禁止静默纠正；正例=music-creation-unit2 设计源副本（0 E）。

## 文件域（≤6）

新建 `agent/internal/legamedesign/*.go`（parse/validate/diagnostics/types）、`*_test.go`、testdata fixture 副本；不动 `agent/internal/edu/lesson_card.go`、projectpackage、chat、webui、内核。

## 验收（零栈；亲跑原文）

1. `cd agent && go test ./internal/legamedesign/ -count=1` exit 0：正例 0 E；每条 E/W 规则至少 1 负例；未知字段/未知枚举/引用悬空/缺失 L1 段/大调小调合法与集合外/单复 music_region 归一等负例；
2. `go build ./...` exit 0；`go test ./internal/edu -count=1` 回归 exit 0（不改变现有行为）；
3. fixture 用 `t.TempDir()` 落盘副本，不改 `coord/lesson-designs/` 原件；`git diff --check` exit 0。

## 停止

同确定性断点两次失败→停；需改契约→停报；不做编译器（424）/工程绑定（425）。
