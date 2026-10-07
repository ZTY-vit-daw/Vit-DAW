# L1-4-IMPL-C 回执（PC 执行侧，2026-10-07 晚窗）

任务卡：`coord/cards/doing/2026-10-07-L1-4-IMPL-C.md`（池序 15；规格=CONTEXT_LAYERING_V1_DESIGN §4-§5 全文+§6.2 T-B 表）

## 实现与提交

- **实现 commit：`0280b970`** @ 分支 `port/l1-4-impl-c`（独立 worktree `D:/Vit_DAW_wt_l1_4_impl_c`；本回执为随附提交）
- base = `a42c87e1`（worktree 建自领取时 origin/main；含 IMPL-A `e26a282c` / IMPL-B `61611533`）。**期间 origin/main 前移至 `d61bfde4`**（决策侧 FS-NL-PROPOSAL-RECON-1 收卡，增量仅 `coord/cards/done/…RECON-1.md` 一文件，与本卡文件域零重叠；验收时点以决策侧裁定为准是否 rebase）
- diff 规模：16 文件 +1772/−15（新增 8：contextruntime 生产 3+测试 3+carriers 测试 1+条款段资源 1；修改 8：promptruntime 1/ruleset 3/chat+agentloop 挂点与对照 4）

## 门与退出码

| 门 | 结果 |
|---|---|
| `cd agent && go build ./...` | **exit 0** |
| `cd agent && go test ./... -count=1` | **exit 0——90 包全 ok，0 FAIL**（与 IMPL-B 基线包数一致） |
| gofmt | 本卡全部 .go 文件（盘上 LF 与 committed blob 双重核验）**gofmt -l 零输出**。注：worktree `core.autocrlf=true` 使 CRLF checkout 的**未触碰**文件也被 `gofmt -l` 全库标记——checkout 伪象非仓内容（提交字节为 LF），非本卡影响面 |

## T-B 红绿形态（§6.2）

**红（现状锚点，已实证）**：base `a42c87e1` 上仅落 T-B 测试文件 → 包级编译红——`undefined: ExitReport / ExitDecision / NewExitExecutor / ExitExecutorConfig / TurnBoundaryEvent / WindowState / WindowUnit / ExitUnit / ruleset.SectionEvidenceRefDiscipline` 等（退场执法面/伴随索引/三态包装/条款段在 main 上不存在；沿 IMPL-B「新增载体=编译红即现状锚点」先例）。

**绿（测试名在案）**：

| # | 测试 | 断言面 |
|---|---|---|
| T-B1 | `TestTB1RepullResolvedAfterExit` | turn k 票退场（executor ref 态）→turn k+n 重拉 state=resolved；handle 读回内容 sha256 与原票一致；freshness=material_reuse 透传；summary 在位 |
| T-B2 | `TestTB2RepullStaleNoUpgrade` | 物化层标脏→重拉 state=stale 且 freshness=stale 原样、无升级路径（响应 note 明示语义） |
| T-B3 | `TestTB3RepullMissingWithLedgerPointer`（+`…HonestWithoutLedger` 无账本诚实降级腿） | 隔离工作区移除票工件→重拉 state=missing；pointer{ledger_entry_id=retain 条目, turn_id=LastSeenTurn 直通}；Note 指引账本行非静默；无账本时字段诚实归零+「结论未留」明示 |
| T-B4 | `TestTB4InvariantRefusesEvidenceBearingExit`（+`TestTB4TriStateDecisions` 三态判定/CAS 白名单/opaque 宽容；`TestTB4CriteriaAndBudgetOrder` 判据优先序+预算淘汰序） | 无结论无句柄候选→Violations 非空、不产 decision（退场被拒）、WARN 落遥测挂点、超预算不吞执法 |
| T-B5 | `TestTB5LedgerAppendOnlyBytes` | 三轮 retain 接线落盘逐轮字节级 starts-with；撤销路径=追加 Supersedes 条目（前缀仍不动）；链校验 4 条通过 |
| T-B6 | `TestTB6CompanionIndexTriStateDirectThrough`（+`TestTB6PureParseNoDeps` 零依赖形态） | ParseState 与 agentprotocol 逐 ref 直通（parsed/legacy/opaque/malformed→opaque 留痕）；opaque 不进可重拉面但文本残骸保留（历史零改写）；账本反指/去重/assistant-only |
| T-B7 | `carriers.TestTB7DisciplineClauseInStableLayer` | 条款段（ID=shared.discipline.evidence_refs）在 manifest 且跨族；chat/neutral 渲染含条款；四层装配 L1 稳定段含条款且 CacheKey=RulesetVersion（版本化）；稳定 system 消息（模型可见 prompt 面）含条款锚句 |

## 挂点接线（≤30 行级，零行为切换）

- chat `buildAssemblyWithReport`：**+19 行**；agentloop `assembleWithReport`：**+18 行**——单入口 `contextruntime.RunTurnBoundaryHook`（伴随索引注记+执法面），retain/ref 决策不消费、账本不落盘、CAS 不接线（生产消费切换归 IMPL-D）
- HistoryLimit=12 对齐 chat 既有截尾线（`server.go:5761` `conversationHistory(ctx, id, 12, …)`——**卡面锚 server.go:5686 为 IMPL-A 前行号，领取时实核为 5761**）；chat 轮无独立 TurnID，turn_end 判据在该面不触发（观察票/工具结果单元的完整喂给归 IMPL-D）
- Violations 进遥测：`AssemblyReport.ExitViolations` → `PromptStatsExtras` 新增 `exit_violations` 键；另加 `history_refs_total/history_refs_parsed`（§5.2 Tax 递减=parsed 占比首审口径）——三键均为**加法式**，既有 prefix_bytes/dynamic_bytes/breaks 不动（T-A5 断言为存在性非精确键集，实测不受扰）

## CAS 白名单与 OQ-3 数据

v1 白名单=**仅 `tool_result`**。裁定依据（§5.4 例外路径=无票且 retain-worthy 长文本）：observation_bundle 有票（F6 天然可重拉，CAS 冗余）；trace 由审计快照冷引用覆盖（§4.2 ref 态既有形态）；history_message 是不可变审计面+伴随索引覆盖——三类不进白名单；唯 tool_result 存在「无票长文本」类目（T-B4 合成面 7 单元类型×句柄/结论覆盖矩阵）。**数据诚实边界：以上为合成受控测试数据，真实单元分布待 IMPL-D 真栈接线后采集——建议 v1 白名单维持至真栈数据到位再裁（OQ-3 决策侧复核）**。

## 红线对账

1. **五处既有退场机制零改动**：chat 截尾 12（server.go:5761）/快照 recent_turns=8（contextruntime/context.go）/观察账本窗口 24 对+audit_snapshot 冷引用（model_projection.go）/ExpiresAfterContextChange（agentloop/execution_memory.go）——锚点文件零 diff（chat/server.go、message_loop.go 的 diff 仅为挂点函数内新增，截尾调用与既有逻辑字节不动）
2. **L1-3 接口冻结**：queryengine/agentprotocol **零 diff**——重拉=既有 ref.query（等值谓词）+Expand 组合，零工具动词/schema/谓词/物化契约改动；F9 三函数零触碰
3. **账本只追加**：`WriteRetains` 全经 `carriers.AppendLedgerEntry`（IMPL-B 写入器，append-only 字面执行）；T-B5 含撤销路径（追加 Supersedes）
4. **ruleset 新段=资源新增**：既有七段 .txt 字节零动；manifest.json 增第八段+RulesetVersion `ruleset.v1`→`ruleset.v2`（卡面「段新增=版本 bump」）；**对照锁定兼容**=双源比对面限定 IMPL-B 迁移七段（`PostMigrationSectionIDs`+`RenderFamilyJoined`，chat/agentloop 两处 parity 测试与 ruleset 七段计数断言同步为八段）；wrapper 排尾不变式经新段声明位（两族 wrapper 之前）保持
5. **接口冻结（IMPL-A/B 产物）**：promptruntime.Build/Assemble 签名零动（报告加法式新字段 HistoryRefs/ExitViolations+HistoryRefEntry 类型）；carriers 公开面零动（只消费）

## 文件数申报（超卡面预计）

16 文件 > 卡面预计 ≤8。全部落在申报域内（contextruntime 执行器+伴随索引+包装 / promptruntime 报告字段（接线 ≤30 行预算内：37 行含类型与遥测） / ruleset 新段资源+辅助面 / 测试文件 / chat+agentloop 仅挂点），**无越域**；超数主因=T-B 七门分四测试文件+条款段需三处测试同步（parity×2+计数）。

## 端测边界声明（设计 §6.4）

本卡挂点为 **advisory 零生产行为切换**：执法面纯函数、无 IO、无状态突变；唯一运行时可见变化=遥测附加三键（加法式，同 IMPL-A T-A5 先例）。故未跑真实栈烟测，真栈收口（生产消费切换+真栈采集）归 **IMPL-D**；渲染面/用户旅程零触碰（无 webui/UI 改动）。若决策侧认定遥测附加键构成行为变化，以本节为显式声明。

## 诚实边界与停止条件

- **v0 bootstrap 标脏面**=产物消失事件（marked_stale 行经事件代保留，§5.1.1 只改状态不删行）——T-B2 在该面上构造；内容变更标脏归 L1-2 物化层 v1，届时 stale 响应的 handle/summary 自然补全（v0 下如实空缺）
- **legacy ref 不在 v1 结构化重拉面**（无完整 L0 坐标）——重拉按 missing+说明注记（不静默不猜）；翻译迁移归注册表补全排程
- **生产挂点的伴随索引为纯解析注记**（无引擎/账本接线，Handle/Freshness/反指空缺）——Expand/Ledger 供给面已在测试与 RepullService 侧验证，生产接线归 IMPL-D
- **停止条件：未触发**——carriers.LedgerEntry 接口足以承载 retain 接线（零改动 IMPL-B 已验收接口）；queryengine 读侧足以表达三态包装（零改动 L1-3 接口）
