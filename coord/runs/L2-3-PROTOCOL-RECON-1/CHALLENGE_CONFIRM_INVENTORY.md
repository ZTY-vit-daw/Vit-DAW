# L2-3-PROTOCOL-RECON-1：质询-确权协议现状盘点报告（D13 统一协议前置）

- 执行侧：Mac 夜间托管会话（纯只读勘察，零代码改动）
- 领取时 origin/main：`6b4eba9`（领取提交 `91bf88d`）
- 领取时 `git status --short`：`M VitApp/Workspace/Settings.xml`、`M VitApp/Workspace/default_project.xml`、`?? .zcodeignore`、`?? VitApp/Workspace/Artifacts/`、`?? coord/runs/FIX-PCA-AUTOSWEEP-1/20260925_mac/`——领取前已有，未触碰
- 日期：2026-09-27；行号以本卡工作 HEAD 为准

---

## 0. 执行摘要

1. 盘出 **6 个现存"类质询"面**（卡面要求 ≥4）：pendingmanager 候选卡、typed/plugin\_prep 审批事件、capabilityinteraction 会话审批解析、PCA 准入+认证 token、盲测 A/B 用户判断、C2 目标确认（机器侧参考面）。
2. **三要素（假设+证据引用+低成本反馈）唯一齐全的是盲测 A/B 判断回路**（M5）；PCA 准入（M4）证据结构化最强但无用户反馈面；typed 审批（M2）有低成本二选一但**审批结构体无 evidence\_refs 字段**。
3. 固化路径分级明确：PCA store 是唯一"一次认证终身可用"的持久确权样板（二进制指纹失效即 stale）；A/B 判断持久但**不外推**（下轮/下工程重问，无偏好沉淀）；pendingmanager 纯内存。
4. 惰性现状总体健康：确认都发生在"用到时"（命令执行前/实验落地后/装载时），无冷启动批发式问询；冲突再确权在 M3/M4/M5 各有机制（revision/fingerprint-stale/supersedes）。
5. D13 四适用域（段落命名/角色身份/知识缺口/主观偏好）中，**前两域无任何现存确权机制**（role\_guess 是无确权猜测、段落是派生结果）——统一协议的净新增面。

## 1. 现存确认/审批机制全景（6 面）

### M1：pendingmanager 候选卡（agent 侧待确权状态机）

- **触发**：LLM 提案落成 `PendingCandidate`（`agent/internal/agentprotocol/types.go:223-242`）——active 判定按 status（`agent/internal/pendingmanager/manager.go:133-147`，verified/verification\_failed/blocked 为终态）。
- **呈现形式**：候选卡随 chat 响应携带（`agent/internal/chat/server.go:164` PendingCandidates 列表）；卡面字段=Summary/Rationale/NeedsResolution/Confidence/Risk。
- **证据引用**：**有**——`EvidenceRefs []string`（types.go:231）直挂候选。
- **留痕位置**：`MemoryManager` 内存 map（manager.go:22-26），**不落盘，重启即失**。
- **结果去向**：`Transition(id, status, reason)`（manager.go:81）状态机推进；按 Conversation/Goal/TargetRef 三键检索（:52-69）。

### M2：typed 审批事件与 plugin\_prep 用户确认面

- **触发**：命令目录标记 RequiresConfirmation（tools/catalog.go 各 spec 的 confirm 位）→ 构造 `ApprovalRequest`（`agent/internal/chat/typed_protocol.go:18-40` 工具调用前/:49-75 结果侧）。
- **呈现形式**：ApprovalRequest=ResourceDomain/Operation/TargetRef/Reason + **AvailableDecisions ["approve\_once","deny"] 二选一**（typed\_protocol.go:33/:68）；plugin\_prep 变体面向中文用户："写入插件参数前必须先由用户确认该候选。" + approve/revise/cancel 三选 + 配套用户输入事件（`agent/internal/chat/plugin_prep_worker.go:993-1010` 审批事件、:1012+ `pluginPrepWorkerUserInputEvent`）。
- **证据引用**：**无**——ApprovalRequest 结构体（types.go:244-253）**没有 EvidenceRefs 字段**，只有 Reason（preview 文本）。
- **留痕位置**：chat 事件流（agentprotocol.NewEvent → 前端可见）。
- **结果去向**：decisions 状态驱动 executor 执行/放弃（typedApprovalDecisionEvents :42-48）。

### M3：capabilityinteraction 会话审批解析（冻结提案的自然语言确认）

- **触发**：会话存在 ActiveProposal+FrozenPlan 期间，每条用户轮进 `Resolve`（`agent/internal/capabilityinteraction/resolver.go:20-30`；无提案→ambiguous）。
- **呈现形式**：自然语言意图六分类——question/reject/revise/narrow/approve/ambiguous（:43-79）；调整量正则解析（max/replace/set 三模式，:14-18）；中文意图词表（exclude/include，:87-93）。
- **证据引用**：**无**——决策绑定的是提案身份指纹（ProposalID/Revision/ActionSetHash/ProjectCutHash，:32-36），这是**提案自证**不是证据引用。
- **留痕位置**：ApprovalDecision（含 Adjustments/Unresolved/Confidence）。
- **结果去向**：approve→执行；revise/narrow→`ReviseFrozenPlan` 生成**新不可变提案修订**（`revise.go:28-50`，永不写工程）；解析不了→`ErrRevisionNeedsClarification`（revise.go:17）回问。

### M4：PCA evidence-backed control admission（含认证授权 token）

- **准入触发**：插件推荐/装载前查 `QueryInstalled`/`QueryInstalledAdmission`（`agent/internal/chat/semantic_treatment_pca.go:117/144/210`；`agent/internal/processorattestation/eligibility.go:37-66`）。
- **呈现形式**：机器判定面（无用户交互）——EligibilityResult=eligible/effective\_status/reason/missing\_coverage（eligibility.go:26-33），**fail-closed 无产品名回退**（:34-36 注释明言）。
- **证据引用**：**有且最结构化**——binary\_fingerprint+attestation\_id+coverage 轴（eligibility.go:28-32）。
- **认证侧用户面**（pcactl certify）：`processorCertificationStartRequest`（identifier/family/confirmed/consent，`agent/internal/chat/processor_certification_entry.go:66-71`）→ 产出**一次性装载授权 token**：一次性消费（:104-107 map delete）、过期检查（:109-112）、scope 三重校验（source/confirmed/tool 名，:113-118）、精确身份匹配（track/path/identifier，:119-122）、装载前二进制指纹复验（:125-127）。
- **留痕位置**：PCA attestation store（`agent/internal/processorattestation/store.go`，落盘）。
- **结果去向**：token 换取 `AuthorizeProcessorCertificationLoad` 上下文标记（:129）→ 装载放行；attestation 持久可复查。

### M5：盲测 A/B 用户判断回路（audition judgment）

- **触发**：实验轮 settle 报 `user_judgment_pending` 后，`requireTaskHumanJudgment` 挂起实验（`agent/internal/chat/audition_events.go:762`）；判断边界是模型轮的**合法终态边界**（`agent/internal/chat/free_state_reasoning_loop.go:901-916`——边界上只认 settle 决策，其余模型输出被拒）。**惰性：实验真正落地后才问**。
- **呈现形式**：`auditionJudgmentRequest`（audition\_events.go:304-314）= heard\_difference + **preference（A/B 二选一结构）** + reason\_tags + free\_text——**D13"低成本反馈（二选一/确认）"的现成形态**；盲测开关 VIT\_DAW\_AUDITION\_BLIND（:26）+ 盲测披露 schema vit.audition\_blind\_disclosure.v1（:32-33），判定应答含解盲（物理分配+动作，:558-560 注释）。
- **证据引用**：**有**——evidence\_id/supersedes\_id/project\_revision（:308-310）；判断证据绑定实验轮（round.UserJudgmentEvidence）。
- **留痕位置**：round.UserJudgmentEvidence → `storeFreeStateLoop`（free\_state\_reasoning\_loop.go:625-647）+ workspace 快照字段 FreeStateLoops（server.go:161）；轨迹事件 EventUserJudgmentRecorded/Requested 含 restored 重放（audition\_events.go:214-243）。
- **结果去向**：双落账分支——mix-tick 面 `recordMixTickAuditionJudgment` 与自由态实验面 `recordFreeStateAuditionJudgment`（:545-553，B12-2 注释：mounted 卡落账在 mix\_tick 确认面不写实验状态机）；heard+preference 组合映射 EvaluationHumanConfirmed/Ambiguous（:215-220）。

### M6：C2 目标确认（机器侧参考面）

- 触发：C2 全项目候选需深确认时对候选轨发 CCB 观察（`agent/internal/chat/c2_dynamic_control_runtime.go:193-197`，Source="c2\_target\_confirmation"，Confirmed=true 机器内部确认非用户面）；呈现=CCB bundle；结果=TargetHypothesis 成立或失败。这是"确权"概念的**机器对机器变体**——非用户质询，列入全景供同构分析。

## 2. 三要素差距矩阵（对照 D13）

| 机制 | 假设明确 | 证据引用 | 低成本反馈 |
|---|---|---|---|
| M1 pendingmanager | 部分（Rationale/Summary 自由文本无契约） | **有**（EvidenceRefs 直挂） | 无（NeedsResolution 自由文本，无选项化） |
| M2 typed/plugin\_prep 审批 | 部分（Reason=preview 文本） | **无**（ApprovalRequest 无 refs 字段，types.go:244-253） | **有**（approve\_once/deny、approve/revise/cancel） |
| M3 会话审批解析 | 有（冻结提案+四重指纹） | 无（提案指纹是自证非证据） | 部分（自然语言非低成本；revise 依赖正则再解析，解析失败回问） |
| M4 PCA 准入 | **有**（family+coverage 确定性契约） | **有**（fingerprint/attestation\_id，全库最结构化） | 不适用（机器面无用户反馈） |
| M4b 认证 token | 有 | 有（装载前指纹复验） | 低（confirmed+consent 布尔） |
| M5 盲测 A/B | **有**（实验轮假设随提案存续） | **有**（evidence\_id/project\_revision） | **有**（preference 二选一+reason\_tags） |
| M6 C2 目标确认 | 有 | 有（CCB bundle） | 不适用（机器面） |

**结论**：三要素齐全仅 M5；统一协议的最优起点=M5 的请求结构（假设+refs+二选一）与 M4 的证据形态（结构化 fingerprint/coverage），补 M2 缺的 refs 字段。

## 3. 固化路径现状（确权结果落到哪+"终身享用"评估）

| 机制 | 落点 | 持久性 | 下次免问？ |
|---|---|---|---|
| M1 | 内存 map | 会话级，重启丢 | **无固化** |
| M2 | chat 事件流 | 审计可查 | 事件不构成免问依据——同命令下次照问 |
| M3 | FrozenPlan 修订链 | session 内不可变 | 跨会话无 |
| M4 attestation | PCA store 落盘 | **跨会话持久** | **是**——一次认证终身可查，直到二进制指纹变化转 stale（eligibility.go:58-60 fingerprint\_unavailable→stale）——**全库唯一"一次性成本终身享用"现成样板** |
| M5 judgment | loop→workspace 快照+trajectory | 跨重启持久（restored 重放实证 audition\_events.go:206-243） | **否**——判断只结算当轮实验，不沉淀为偏好；下轮/下工程同类问题重问 |
| M6 | TargetHypothesis | 会话级 | 否 |

## 4. 惰性质询现状

- **用到才问（惰性）**：M2（命令执行前）、M5（实验落地后）、M4（装载时即查；认证由 pcactl 显式发起）、M6（候选成立时）。
- **存在期常驻（半惰性）**：M3——冻结提案存续期间每轮都过 Resolve（resolver.go:26），但只在用户主动说话时解析，不主动打扰。
- **无冷启动批发式问询**：未发现任何"开局问一堆"模式——D13 惰性原则的现状基础良好。
- **冲突再确权**：M5 `supersedes_id`（audition\_events.go:309，新判断显式取代旧判断）**有**；M4 指纹失配→stale→重认证 **有**；M3 revision→新提案修订 **有**；M1/M2 无。

## 5. 同构/异构关系（哪些是同一套东西的不同面）

- **同一族**：M1/M2/M3 都是"提案→等确认→执行/修订"状态机的不同挂点——M1 是 agent 侧数据结构、M2 是 harness 命令面、M3 是会话语言面；三者的"确认"语义同源（用户授权一次有界动作），但**三套独立实现**（PendingCandidate 状态机 / ApprovalRequest+decisions / ApprovalDecision 六分类），无共享抽象。
- **同一族**：M4 准入与 M4b token 是同一 PCA 体系的两段（存量查询 vs 增量认证）。
- **同一族**：M5 的双落账分支（mix-tick 面 vs 自由态实验面）是同一判断席服务两个提案体系（B12-2 注释 audition\_events.go:541-544）。
- **异构**：M6 是机器侧确权（CCB 观察），与用户面五者不同源——但它演示了"确权=观察证实假设"的机器形态，D13 统一协议若覆盖机器对机器确权可参考。

## 6. D13 四适用域的现存覆盖

| D13 域 | 现存最接近 | 判定 |
|---|---|---|
| 段落命名 | SegmentationPrimitives（L2-2，派生结果）+ pending 计划的 pending\_section\_markers | **无确权机制**——派生≠确认，命名从未回问 |
| 角色身份 | project.structure 的 role\_guess（CCB-VIEW 报告 §1） | **无确权**——猜测字段无质询回路 |
| 知识缺口 | M1 NeedsResolution / M3 Unresolved | 部分——自由文本形态，非结构化缺口卡 |
| memory 主观偏好 | M5 judgment（持久不外推） | 载体最接近；外推学习无 |

## 7. 验收对照

| 卡面验收项 | 状态 |
|---|---|
| ① 全景 ≥4 机制带锚点 | ✅ 6 面（§1），每面五要素齐 |
| ② 三要素差距矩阵完整 | ✅ §2 逐格判定 |
| ③ 固化路径清单+终身享用评估 | ✅ §3 六行表；M4 唯一持久样板 |
| ④ 惰性现状结论 | ✅ §4（惰性健康+三处冲突再确权+无批发式） |
| ⑤ 报告入库 | ✅ 本文件 `coord/runs/L2-3-PROTOCOL-RECON-1/CHALLENGE_CONFIRM_INVENTORY.md` |
| 零代码改动 | ✅ 仅新增本报告文件 |

## 8. 边界与未覆盖项

- webui 前端呈现细节（确认卡片 UI 形态、audition 按钮交互）未展开——呈现形式以协议结构体为准；渲染面属 E2E-WEBUI-1 域。
- Godot 侧手测链路的确认交互（AGENTS §5 手测入口）未跨仓取证。
- orchestration 包内 PlanningSession/Proposal 的完整生命周期（proposal 生成→冻结→执行）只盘了确认相关切面；完整管线属其他卡范围。
