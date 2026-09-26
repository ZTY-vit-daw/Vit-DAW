# L1-4-RECON-1：上下文装配现状盘点报告（四层前缀与退场机制前置勘察）

- 执行侧：Mac 夜间托管会话（纯只读勘察，零代码改动）
- 领取时 origin/main：`0245860`（领取提交 `4426903`）
- 领取时 `git status --short`：`M VitApp/Workspace/Settings.xml`、`M VitApp/Workspace/default_project.xml`、`?? .zcodeignore`、`?? VitApp/Workspace/Artifacts/`、`?? coord/runs/FIX-PCA-AUTOSWEEP-1/20260925_mac/`——领取前已有，未触碰
- 交叉引用：`coord/runs/L1-1-RECON-1/EVIDENCE_REFS_INVENTORY.md`（下称 L1-1 报告）、`coord/runs/CCB-VIEW-RECON-1/CCB_VIEW_INVENTORY.md`（下称 CCB-VIEW 报告）
- 日期：2026-09-27；行号以本卡工作 HEAD 为准

---

## 0. 执行摘要

1. **装配骨架已经统一在 promptruntime**（system sections + history + user sections，`agent/internal/promptruntime/prompt.go:59-73`），且 Section 已带 `Kind`（static/session/delta/runtime/current\_user 五类，:14-20）与 `Stable` 标志——**四层前缀的"稳定段"元数据已存在但无消费方**：Stable=true 的 chat system 每轮重发全文，没有任何前缀缓存/指纹复用逻辑读它（fingerprint :130-165 只用于遥测）。
2. **三条装配入口共享一个快照构建器**（contextruntime.Build，12 分区）但快照内容逐轮全量重建（CreatedAt 每轮变化），模型面再过 ProjectModelSnapshot 分层预算投影（hot 10KiB/warm 5KiB/fail-closed）。事实上的稳定前缀只有 system prompt 常量本身。
3. **LLMContext 七投影同构**（五字段基础+变体），`do_not_include_raw_package` 七处全部写死 true；RLM 是唯一无 LLMContext 的投影。
4. **相位（FS0-FS9）已从控制流降级为披露**：相位门在 TIMING-1 退役（提案在各相位均可受理），现存角色是 ledger JSON 字段+prompt 机械状态指令；与路线图"相位降级为冷启动第一轮"的裁定方向一致，现状是"半退休"。
5. 退场机制分散在五处（对话截尾/recent 折叠/账本窗口/预算降级/过期标志），无统一"什么该退场"策略；evidence refs 在历史消息中以字符串存续、观察票可按 ID 重拉，但无结构化重拉通道（互证 L1-1 报告 §5 痛点②③的防御解析税）。

## 1. 现行上下文装配链（从触发到 prompt 成形）

**装配器**：`promptruntime.Build`（`agent/internal/promptruntime/prompt.go:59-73`）——system sections 渲染为单条 system 消息 → history 逐条 → user sections 渲染为单条 user 消息；`normalizedHistory`（:103-114）滤空。Section 元数据（Kind/Stable/CacheKey）不进消息、只进指纹与统计。

**入口 A：chat 会话**（`agent/internal/chat/server.go:5673-5763` `buildAssembly`）：

1. 取 UserStateSummary/ModelCatalogSummary/projectHistory，`conversationHistory(ctx, id, 12, …)`（:5679，截尾 limit=12，:5771-5779；空历史回退 ProjectHistory 的 conversation\_messages，:5786-5799）
2. `contextruntime.Build`（:5682-5689）全量快照并 `AppendDefault` 落盘 JSONL（:5690，审计面）
3. system = 巨型常量字符串（命令目录+操作纪律，源跨 :5685-5757，**约 15.7 KB 源文本**）+ modeInstruction + 命令目录
4. 装配（:5751-5763）：`chat_system`（Static/Stable=true）+ `chat_context_snapshot`（**Runtime/Stable=false，挂在 system 节内**）+ `chat_current_user`（CurrentUser）

**入口 B：agentloop MessageLoop**（`agent/internal/agentloop/message_loop.go:3818-3866`）：

- 通用路径 `assembly`（:3818-3837）：system=`messageLoopSystemPrompt`（:4228；自由态激活时直接改走中性族 :4233-4237）+ `History: state.input.Conversation` + user 节 = "Current Goal/GoalID/RunID/剩余工具调用数 + **Neutral context snapshot JSON** + Free-state reasoning ledger JSON"（:3823-3827）
- 自由态生产路径 `assemblyNeutralFamilySelection`（:3839-3866）：中性族系统提示 + **历史裁剪为只保留最后一条 `<final_gate>` 反馈**（:3868-3881，防物化身份泄漏，注释 :3857-3860）+ 同构 user 节
- 输入来源：续传体 `cont.Conversation`（:154/:218），非每轮重建全史

**入口 C：planner**（`agent/internal/planner/planner.go:143-201`）：planner\_system（Static）+ planner\_runtime（Runtime）两节，无 history。

**快照构建与投影**（agentloop 路径的核心两级）：

- `Runner.buildContextSnapshot`（`agent/internal/agentloop/helpers.go:32-54`）：从 runState 收全量输入（trace/planItems/pendingToolCall/executionMemory/recentObservation/projectChange/PreviousSnapshot）→ `contextruntime.Build`
- `Runner.buildModelContextSnapshot`（:60-93）：**中性族先重建无插件身份的快照**（SkipPluginSemanticLoad=true，:73-84）→ `ProjectModelSnapshot`（CCB-VIEW 报告 §1 已详盘：active\_observation 唯一化+观察账本+分层预算执法）→ ModelJSON
- Snapshot 12 分区（`contextruntime/context.go:54-74`）：conversation\_summary / recent\_turns / goal\_trace\_summary / current\_selection / daw\_state\_summary / daw\_semantic\_summary / recent\_goal\_context / project\_change / project\_history\_summary / plugin\_context\_summary / tool\_result\_summary / warnings；Build（:76-152）逐区摘要化
- **默认裁剪参数**（normalizeOptions :327-347）：RecentTurns=8、RecentTraceEvents=12、MaxTextRunes=900、MaxListItems=20、MaxPreviewBytes=12 KiB
- 快照落盘：`AppendDefault` → `VitApp/Workspace/Logs/agent_context_snapshots.jsonl`（:192-241，环境变量 VIT\_CONTEXT\_SNAPSHOT\_PATH 可关）

**每轮固定 vs 动态注入**：

| 块 | 固定/动态 | 证据 |
|---|---|---|
| chat system 常量 | 固定（编译期） | server.go:5685-5757；Stable=true :5753 |
| 中性族 system | 固定骨架+**动态前缀**（diagnostic-only/预算/terminal 指令按状态拼装） | ccb\_model\_prompt.go:18-58（prefix 拼装）+ :59-101（固定骨架） |
| 命令目录/Allowed tools | 固定（随构建/配置） | server.go:5759（catalog 注入）、ccb\_model\_prompt.go:100-101 |
| context snapshot JSON | 每轮动态（CreatedAt RFC3339Nano :133 必变） | context.go:131-152 |
| Free-state ledger JSON | 每轮动态（cycle/continuation\_used 推进） | message\_loop.go:3824-3827 |
| 观察账本/active\_observation | 动态（随观察） | model\_projection.go:52-72 |

**结论**：稳定前缀现状 = system 常量节（chat ~15.7 KB、中性族 ~13.3 KB 源文本，awk 源段字符数实测）；其余全部逐轮重算重发。Stable 标志是"意图"不是"机制"——没有基于 CacheKey/指纹的前缀复用消费方。

**体积度量现状**：`messageLoopModelContextPromptStats`（message\_loop.go:905-908）已把 model\_snapshot\_bytes 记入遥测；分层预算（hot 10 KiB/warm 5 KiB/full 24 KiB，model\_projection.go:18-20）是模型面唯一硬上限，超限 fail-closed（:214-228）。估算方法可复现：快照字节数走遥测已有；system 常量用源段 wc；本报告不编造 token 数。

## 2. LLMContext 结构盘点（8 投影）

七个投影有同构 `LLMContext`（基础五字段：SummaryMD/CompactFacts/DoNotIncludeRawPackage/EvidenceRefs/LimitationNotes，变体加 QualitySummary/TaskPolicy/SafetyGates/SuggestedNextStep）：

| 投影 | 锚点（types.go） | 变体字段 | DoNotIncludeRawPackage 写死 true 锚点 |
|---|---|---|---|
| DOM | :340-347 | +SuggestedNextStep | dom/projection.go:630 |
| MOM | :264-274 | +QualitySummary/TaskPolicy/SafetyGates/SuggestedNextStep | mom/context\_pack.go:13 |
| TIM | :137-145 | +QualitySummary/SuggestedNextStep | tim/projection.go:788 |
| TOM | :162-169 | +SuggestedNextStep | tom/projection.go:994 |
| COM | :342-350 | +QualitySummary/SuggestedNextStep | com/context.go:86 |
| FXM | :151-158 | CompactFacts 带 omitempty | fxm/projection.go:153 |
| EPM | :148-155 | +SuggestedNextStep | epm/projection.go:331 |
| **RLM** | **无 LLMContext 类型** | 结构=rawRow{Key,Data,EvidenceRefs}（rlm/projection.go:151）+Summary 行级 refs（:103） | —（无该标志；行级兜底键带行序号，L1-1 报告 §2.C5） |

**消费纪律落地**：模型投影优先直取 llm\_context（`findViewLLMContext` model\_projection.go:1241-1252，直接字段→facts 内嵌二层扫描），无 LLMContext 才走 equivalentCCBViewProjection 字段白名单回退（CCB-VIEW 报告 §1）——AGENTS §5"Context Runtime 优先消费投影自带 LLMContext、不展开 raw package"在代码面成立：八投影中七投影显式声明 do\_not\_include\_raw\_package=true（RLM 缺位，其 Data 行本身就是摘要行）。

## 3. 相位轮次的装配角色

- **相位定义**：FS0-FS9 十相位（`agent/internal/audioclosure/phase.go:19-28`），法定前向链 :32-34；`AdvancePhase`（:337-340）按 PhaseGuardEvidence 守卫推进——**相位机活在 audioclosure 驱动内**。
- **chat 侧推进点**：`free_state_reasoning_loop.go:1075/:1149` 两处 `advanceAudioClosurePhase`；推进结果写入 loop ctx：`ctx["minimal_audio_closure"]=audioClosureStateMap(closure)` + `ctx["free_state_phase"]=string(closure.Phase)`（:1295-1296）；绑定面 `bound["free_state_phase"]`（audio\_closure\_controller.go:275）。
- **进入 LLM 上下文的三条路**：
  1. ledger JSON 的 `decision_phase` 字段（free\_state\_reasoning.go:393-396 compact 键表内）；
  2. `messageLoopFreeStateHostPhase` 读 ctx.free\_state\_phase 或 closure.phase（ccb\_model\_prompt.go:340-349）——供 prompt 指令拼装；
  3. `messageLoopModelContextProfile` 按 decision\_phase 选模型剖面（selection/materialize/post\_action，helpers.go:116-129）→ 快照分区裁剪。
- **关键事实：相位门已退役**。TIMING-1 注释明言"proposals are admissible in every closure phase; the former phase-timing branches … are retired with the phase gate"（ccb\_model\_prompt.go:428-431）。现存相位表达全是**机械状态披露**（GATE PATH :262-281 / TERMINAL TURN :358-382 / 预算指令 :431-474），不做输出合法性判定。
- **对照路线图**（"相位轮次降级为冷启动第一轮"）：现状是"半退休"——FS0-FS9 仍在每轮被推进与披露，但决策合法性已不依赖它；降级为冷启动第一轮时，可回收的是 AdvancePhase 推进面与 prompt 披露位，相位枚举本身仍是闭包状态机的内部骨架。

## 4. 退场机制现状（轮次间保留什么丢弃什么）

五处独立机制，无统一策略：

1. **对话历史截尾**：chat 面 limit=12 截尾（server.go:5771-5779）；contextruntime recent\_turns=8 + 更早消息折叠为首/末条摘要（context.go:349-389：first\_user\_message/last\_omitted\_\*）；中性族路径**只保留最后一条 final\_gate 反馈**（message\_loop.go:3868-3881）。
2. **trace/工具结果窗口**：RecentTraceEvents=12 + 计数折叠（context.go:391-450）；tool\_result\_summary ≤ MaxListItems 尾部保留（:84-86）。
3. **观察账本窗口**（跨轮记忆面）：available\_views 最新优先 24 对（model\_projection.go:317-319）、rejected\_view\_sets 跨轮持久（do\_not\_retry，:522-534 注释）、receipts 只留当前 delta 窗口（:342-346）；超限折叠为计数+冷引用（:474-486）。
4. **预算降级**：warm 段超 5 KiB → 逐段最大优先替换为 `audit_snapshot://` 冷引用+CompactionMarker；hot 超 10 KiB fail-closed（model\_projection.go:174-230）——**降级为引用永不删事实**（注释 :119-123）。
5. **execution\_memory 过期语义**：结构面 last\_created\_\*/active\_work\_target\_\*（身份回显）+ pending\_ 计划卡族（PendingMixTickCandidate.PendingStaticBalancePlan/PendingPanLayoutPlan/MixTreatmentPending，execution\_memory.go:66-178），多数带 `ExpiresAfterContextChange` 标志（:75/:113/:150/:175）——上下文变化即作废的显式过期位存在；**继承边界**：跨快照只放行 22 键 allow-list（context.go:1002-1015）。

**evidence refs 在历史中的存续**（互证 L1-1 报告）：

- chat `remember` 只存 user/assistant 文本对（server.go:5765-5772）——模型上轮回复里的 `"evidence_refs":[...]` 以**字符串残骸**留在历史文本中，无结构化存续。
- 可重拉性：观察票全量落盘 `<sessionDir>/observations/<ID>.json`，`readObservationByID` 按 ID 读回（L1-1 报告 §2.B1）；ledger available\_views 每行带 observation\_id（model\_projection.go:321）——**跨轮引用句柄存在且有效**；但"旧轮 ref 字符串→重拉"无通道（模型若引用已退场轮的 ref，靠防御解析兜底，L1-1 报告 §5 痛点②③）。
- 中性族路径下 assistant 中轮内容整体被裁（只留 final\_gate），**旧轮 refs 在该路径不随历史存续**——跨轮证据连续性完全由观察账本+ledger 承担。

## 5. D6 四层映射表（现存最接近载体）

| D6 层 | 现存最接近载体 | 锚点 | 缺口 |
|---|---|---|---|
| agent 规则 | 编译期 Go 字符串常量：chat system（~15.7 KB）、中性族 prompt 骨架（~13.3 KB）、full-access autonomy 规则 | server.go:5685-5757、ccb\_model\_prompt.go:59-101、full\_access\_autonomy.go | 无版本化/分层/按需装载；改规则=改码重编译；两条入口各持一份（有重叠纪律） |
| 用户偏好 | mixstyle 样式文件（vit.mix\_style.v1，.vms，Identity+版本）挂进 pending 计划的 StyleID；盲测 A/B 用户判断记录 | mixstyle/style.go:13-30、execution\_memory.go:97-100 | 样式是**工程内**风格模板；无跨工程"该用户偏好"档（D13 memory 主观偏好无载体） |
| 环境实例 | pluginsemantics 语义索引（LoadPluginSemanticSummary 注入快照）；机器校准链（PORT-C2 域） | context.go:111-120/:275-325 | 索引只覆盖插件语义面；环境指纹无常驻"实例卡" |
| 工程账本 | ProjectHistorySummary（分支/检查点/worktree 摘要入快照）+ journal/projectstore（机器侧）+ coord/decisions（人类决策侧，仓外流程） | context.go:735-767（summarizeProjectHistory） | 三者分立；**"该工程学到什么"的语义账本无载体**（观察账本只在 loop 生命周期内） |

## 6. 验收对照

| 卡面验收项 | 状态 |
|---|---|
| ① 装配链完整（触发→prompt 成形） | ✅ 三入口（chat/agentloop/planner）+ 两级快照（Build→ProjectModelSnapshot）+ promptruntime 终装配，§1 全锚点 |
| ② LLMContext ≥6 投影 | ✅ 8 投影逐一（7 同构+RLM 无类型），do\_not\_include\_raw\_package 七处写死 true |
| ③ 相位表达锚点 | ✅ §3：定义/推进/三条入上下文路/相位门退役注释 |
| ④ 退场现状+refs 存续结论 | ✅ §4：五机制+refs 字符串残骸/观察票可重拉/中性族历史裁剪 |
| ⑤ 四层映射表 | ✅ §5，缺载体处明示 |
| ⑥ 报告入库 | ✅ 本文件 |
| 零代码改动 | ✅ 仅新增本报告 |

## 7. 边界与未覆盖项

- **按 token 精确计量未做**（卡面允许）：快照字节数已有遥测（model\_snapshot\_bytes），system 常量给了源段 wc 实测；真实每轮 token 消耗需从 LLM 日志取数，超出只读勘察面。
- goalrunner/orchestration 等次要装配入口未逐一展开（它们复用 contextruntime.Build 或自有轻量 prompt，非自由态主链）；若 G1 评审需要可补 ≤0.5h 小节。
- webui 侧呈现与 Godot 前端不在本卡范围（AGENTS §5 渲染面纪律）。
