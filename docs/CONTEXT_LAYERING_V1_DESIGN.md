# 上下文管理设计：四层稳定前缀 + 退场机制（CONTEXT_LAYERING_V1_DESIGN，L1-4）

> **状态：规划（未实现）**。本文档是路线图 [AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md](AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md) L1-4 段的接口层设计（卡 `coord/cards/doing/2026-10-01-L1-4-DESIGN-1.md`，PC 会话③），CURRENT-STATE 登记由决策侧收口。零代码改动；全部接口为设计态签名，落地按 §9 排程拆卡。
>
> 规格锚：路线图 D6（四层前缀 + 退场机制/浮动窗口）、D14（三种记忆三种键——**画像数学不在本卡**）、D15（新 harness 衔接）；G1 裁定 [coord/decisions/2026-09-27-g1-ref-schema-ruling.md](../coord/decisions/2026-09-27-g1-ref-schema-ruling.md)（L0 文法 + 三态解析，不私改）；[QUERY_ENGINE_V1_DESIGN.md](QUERY_ENGINE_V1_DESIGN.md)（查询面与物化读口契约，**接口冻结基线**）。事实基线：[CONTEXT_ASSEMBLY_INVENTORY.md](../coord/runs/L1-4-RECON-1/CONTEXT_ASSEMBLY_INVENTORY.md)（下称勘察报告）。
>
> 一句话定位：把"稳定前缀"从意图变成机制——四层前缀各得载体与聚合键、稳定段与前缀指纹进可测断言、轮次退场统一为「结论 + 句柄」策略、退场内容经已合入的 ref.query/ref.diff 查询面做三态重拉；**不推翻 G1/G2/L1-3 既有裁定，全部为装配层增量**。

---

## 0. 设计输入与既有事实（不自创现状）

以下事实全部来自勘察报告与代码锚点（行号以领取时 HEAD `7582976b` 复核，勘察报告未带包前缀的两处路径已修正标注），本设计只在其上做接口决策。

| # | 事实 | 出处 |
|---|---|---|
| F1 | promptruntime Section 已带 `Kind`（static/session/delta/runtime/current_user 五类，`prompt.go:15-19`）与 `Stable`/`CacheKey`（`prompt.go:22-28`），但**零前缀复用消费方**：`Build`（`prompt.go:59-73`）把全部 system sections 渲染进**单条 system 消息**；指纹（`prompt.go:130-165`）是整装配 sha256，只进遥测。Stable=true 的 chat system 每轮重发全文 | `agent/internal/promptruntime/prompt.go` |
| F2 | 三条装配入口共享 contextruntime.Build：chat `buildAssembly`（`chat/server.go:5680`，历史截尾 limit=12 `:5686/:5778`，remember 只存文本对 `:5769`）；agentloop `assembly`（`agentloop/message_loop.go:3818-3837`）与中性族 `assemblyNeutralFamilySelection`（`:3840-3866`，历史裁剪为只保留最后一条 `<final_gate>` 反馈 `:3868-3881`）；planner（`planner/planner.go:143-201`，无 history） | 各包 |
| F3 | **断裂源**：快照逐轮全量重建，`CreatedAt` RFC3339Nano 必变（`contextruntime/context.go:133`）；chat 把 `chat_context_snapshot`（Runtime/Stable=false）**挂在 system 节内**——与 Stable=true 的 chat_system 同渲染进一条 system 消息，快照每轮变 → 整条 system 消息字节变 → 前缀全断；中性族 system 段 Stable=true 但内容由状态拼装（`ccb_model_prompt.go:18-58` prefix + full-access 规则，`message_loop.go:3844`）——"Stable 是意图不是机制"的两个实例。**对照：agentloop 通用路径已是正确形态**（稳定 system + 动态 user 节装 snapshot/ledger JSON，`message_loop.go:3823-3830`） | F2 各锚 |
| F4 | 分层预算与冷引用：hot 10 KiB / warm 5 KiB / full 24 KiB（`contextruntime/model_projection.go:18-20`）；超限逐段降级为 `audit_snapshot://` 冷引用 + CompactionMarker（`:197/:209/:225/:120/:145/:167`），**降级为引用永不删事实**；hot 超限 fail-closed；观察账本 available_views 最新优先 24 对（`:317-318`）、rejected_view_sets 跨轮持久、receipts 只留当前 delta 窗 | `contextruntime/model_projection.go` |
| F5 | 退场机制分散五处、无统一策略：① chat 截尾 limit=12；② 快照 recent_turns=8（`context.go:328-329`）+ 首/末条折叠（`:349-389`）；③ 观察账本窗口 24 对 + rejected 持久；④ 预算降级冷引用（F4）；⑤ execution_memory `ExpiresAfterContextChange` 显式过期位（`agentloop/execution_memory.go:75/:113/:150/:175`）+ 跨快照 22 键 allow-list（`contextruntime/context.go:1002-1015`） | 勘察报告 §4 |
| F6 | refs 存续形态：历史中=字符串残骸（remember 只存文本对）；观察票全量落盘 `<sessionDir>/observations/<ID>.json` 可按 ID 重拉（`mixboard/mixboard.go:3737`，跨 root `:3716`）；账本行带 observation_id；**无结构化重拉通道**（L1-1 报告 §5 痛点②③防御解析税的根源）；中性族路径旧轮 refs 不随历史存续，跨轮证据连续性完全由观察账本+ledger 承担 | 勘察报告 §4 |
| F7 | D6 四层缺载体：agent 规则=编译期常量无版本化（chat system 约 15.7 KB 源文本、中性族骨架约 13.3 KB，勘察实测；命令目录随构建注入）；用户偏好=mixstyle 样式**工程内**模板无跨工程档；环境实例=仅 pluginsemantics 语义索引无实例卡；工程账本=ProjectHistorySummary/journal/coord 三者分立无语义账本 | 勘察报告 §5 |
| F8 | 相位半退休：相位门已随 TIMING-1 退役（`agentloop/ccb_model_prompt.go:428-431` 注释"proposals are admissible in every closure phase"）；现役角色=ledger `decision_phase` 字段 + GATE PATH/TERMINAL TURN/预算指令披露位 + 模型剖面选择（`agentloop/helpers.go:116-129`，剖面只影响快照分区裁剪） | 各锚 |
| F9 | 查询面已合入且冻结：`queryengine` 包 `Query`（`engine.go:127`）/`Expand`（`:286`）/`Catalog`（`:400`）/`DiffEvidence`（`diff.go:59`）；`MaterializedStore` 三函数契约（`store.go:19`：SnapshotView/Resolve/Subscribe，失效与新鲜度归物化层）；工具面 `ref_query`/`ref_diff` CommandSpec（`tools/catalog.go:726/:800`），ref.query 带 expand=handle/summary 与 expand_max_bytes；freshness 显式谓词、stale 不删行不升级（T6 语义） | `agent/internal/queryengine/` |
| F10 | ref kind 注册表已补全九投影+DAD：dom/fxm/com/rlm（内容 ID 族）+ mom/tim/tom/epm（无前缀族，snapshot 段承载=observation_id）+ acp + dad.l3/dad.l2_render_probe/dad.compressor_dual_tap/dad.frequency_evidence（`agentprotocol/refschema.go:119-164`，REFSCHEMA-L0-2）；解析三态 parsed/legacy/opaque，opaque WARN 一次透传（G1 ruling #8） | `agentprotocol/refschema.go` |
| F11 | LLMContext 七投影同构 + RLM 无类型；`do_not_include_raw_package` 七处写死 true；模型投影优先直取 llm_context | 勘察报告 §2 |
| F12 | L0 文法（G1 定版，本设计全文引用不修改）：`vit://kind/scope[/window]@snapshot#hash`，window 显式 `t=all` 强制、hash 段 `#-`=显式未 CAS 化；CAS 句柄=`evidence://<sha256>`（projectstore.PutEvidence，pin 不可逐） | G1 ruling；QUERY_ENGINE §0 F1/F5 |

**宪法条款（本设计的 D2/D5/D6 摘录）**：

- **D6 上下文管理**：四层前缀 = agent 规则（全局）/ 用户偏好（跨工程随用户走）/ 环境实例（机器+版本指纹，不跨设备同步）/ 工程账本（append-only 决策账本 + TOM 概览 + 总线拓扑 + 交付目标）；**决策账本只追加不重写，保证跨轮次 KV cache 命中不断裂**；退场机制 = 轮次结束后投影原始内容出窗，只留结论 + evidence refs 入账本；ref 是重拉句柄——只要每个观察都带可重拉句柄，窗口就可以激进滑动。
- **D14 三种记忆三种键**：用户偏好按"人"（跨设备同步，质询确权协议）；客观经验按"环境"（机器+版本指纹，不跨设备同步，失配降权→复验退役留痕）；工程事实按"工程"（append-only）。**画像数学（逐维贝叶斯/例外升格降格/折扣因子）不在本卡**（memory 收尾段）；本卡只定条目结构与前缀渲染语义。
- **L1-4 蓝图验收**：轮次退场后句柄重拉一致性；前缀 append-only 的 cache 命中验证。

---

## 1. 总体形态：四层前缀 + 动态区 + 退场通道

```
 每轮 LLM 消息序列（目标形态）                       变化频率
 ┌─────────────────────────────────────────────┐
 │ system 消息（纯稳定段，逐轮字节恒等）             │
 │   L1 agent 规则（ruleset 版本化 manifest）  ███ │  随二进制版本
 │   L2 用户偏好（活跃 core 偏好集渲染）       ███  │  确权事件驱动
 │   L3 环境实例（实例卡+高频客观经验蒸馏行）   ███ │  会话内恒定
 │   L4 工程账本（append-only 条目顺序串联）   █░░░ │  每轮尾部可追加
 │   命令目录/Allowed tools（随构建，归 L1 族） ███ │  随构建
 ├─────────────────────────────────────────────┤
 │ history（退场窗口：滑窗 + 退场执行器执法）      ░░░ │  每轮滑动
 ├─────────────────────────────────────────────┤
 │ user 消息（动态区，每轮合法变化）                │
 │   目标/预算/快照 JSON/ledger JSON/状态指令  ░░░ │  每轮
 └─────────────────────────────────────────────┘
          │ 轮次边界
 ┌────────▼────────────────────────────────────┐
 │ 退场执行器 ExitExecutor（确定性，零 LLM）       │
 │  retain（结论入账本）/ ref（句柄化）/ drop       │
 │  不变式：结论已留 OR 句柄可重拉，否则拒绝退场      │
 └────────┬────────────────────────────────────┘
          │ 重拉（模型发起，读侧）
 ┌────────▼────────────────────────────────────┐
 │ 既有查询面（L1-3 冻结接口，零改动）              │
 │  ref.query（等值谓词+expand）+ ref.diff         │
 │  三态响应：resolved / stale / missing+指引      │
 └─────────────────────────────────────────────┘
```

三条核心裁决（全文设计的公理）：

1. **前缀 = append-only 事实序列，不是"当前状态"投影**。"当前拓扑/当前选择/当前电平"属 pull 时代的按需内容（query/observe 拉取）或动态区小摘要；前缀只装**事实条目**（规则、确权偏好、环境事实、决策记录），条目追加使前缀**尾部增长**（cache 友好：旧 token 全命中，只新增），不做中段改写。
2. **stable 与 unstable 物理分离**：凡是逐轮可变的内容（快照 JSON、状态指令、预算数）一律不进稳定段的消息边界。治理不是发明新形态——agentloop 通用路径已是"稳定 system + 动态 user"（F3），本设计把它**统一到全部入口**（chat 的 snapshot 迁出 system 节、中性族的状态拼装迁出 system 节）。
3. **断裂必须显式可归因**：设计目标不是"前缀永不断裂"（版本升级、确权事件、账本追加都是合法断裂），而是**每一次断裂都可观测、可归因、枚举封闭**（§3.4 断裂原因枚举）。隐式断裂（字节变了却说不出哪层为什么变）是唯一红线。

职责边界（对齐既有纪律）：

- **本设计不改投影层**：四层前缀是投影产物的**又一只读消费端**（渲染 TOM 概览/RLM 目标进账本头部段走 LLMContext/摘要行，不展开 raw package，F11）；peer 拓扑不变（AGENTS §5）。
- **本设计不改查询面**：重拉通道=装配层写侧登记 + 既有 ref.query/Expand 读侧的组合（§5），工具面 JSON schema、谓词模型、物化契约（F9）全部冻结复用。
- **退场执行器零 LLM 参与**：退场是确定性策略执法（判据→三态动作），不是模型决策——与"裁判不下场"同族的可解释性立场。

---

## 2. 四层前缀载体（验收②）

### 2.0 通用段模型与层序

四层统一渲染为 promptruntime Section（机制复用 F1，不新建装配器）：

```go
// 设计态签名。每层产出一个 Section：
//   ID:    "ctx.layer.<name>"（rules/profile/env/ledger）
//   Kind:  SectionStatic（L1）；SectionSession（L2/L3/L4 头部）
//   Stable: true（全部四层——"稳定"指本轮装配内不再重算，不指永不变化）
//   CacheKey: <layer version digest>（§3.2 双轨语义）
// 消息边界：四层全部进稳定 system 消息，按稳定性递减排列（L1→L2→L3→L4→命令目录）。
// 排序原则：KV cache 前缀命中要求变化频率升序——任一字节变化使其后全部缓存失效，
// 最稳的层必须在最前（命令目录随构建，与 L1 同族，排其尾或并入其 manifest）。
```

**层装载失败语义（fail-open 但显式）**：某层载体文件缺失/损坏 → 该层不渲染，AssemblyReport 标 `absent`/`corrupt`（§3.1），不阻塞会话（旧工程包无新文件=空层是**常态路径**，§8.3）；损坏写 WARN，不静默吞。

### 2.1 L1 agent 规则——ruleset manifest（编译期常量的演进而非推翻）

**载体新建物：版本化规则清单（go:embed 资源 + manifest）**。规则文本从 Go 字符串常量迁移为嵌入资源文件，manifest 声明段元数据：

```go
// 设计态：agent/rules/ruleset（embed.FS）+ manifest
type RuleSection struct {
    SectionID  string // 稳定段 ID（promptruntime Section.ID 对齐）
    Version    int    // 段级版本；ruleset 整体版本=sum
    Purpose    string // discipline | catalog | output_format（§8.1 内容盲审查键）
    Content    string // 规则文本（chat system 拆段 / 中性族固定骨架 / 命令目录包装层）
    AppliesTo  []string // 入口族：chat | neutral_family | planner（重叠纪律段去重的挂点）
}
type RulesetManifest struct {
    RulesetVersion string // 如 "ruleset.v3"；进 L1 CacheKey
    Sections       []RuleSection
}
```

设计要点：

1. **演进不推翻**：promptruntime Section 机制、Stable/CacheKey 字段原样复用；迁移只改"常量从哪来"（embed 资源），不改装配协议。两条入口各持一份的重叠纪律段（勘察 §5 缺口）借 `AppliesTo` 归并，语义由实现卡逐段核对（本卡不定文本内容）。
2. **版本化**：ruleset 版本进 L1 CacheKey 与前缀指纹。规则变更=版本递增=显式 `ruleset_changed` 断裂（§3.4）——升级后 KV cache 重建一次是**诚实的代价**，不伪装连续。
3. **按需装载不破前缀**：任务域规则段（如频段清理工作流细则）**装载进动态区或 session 尾部追加段，永不插入 static 段中间**。原则：static 层只进不改；on-demand 内容的追加发生在 L4 账本或动态区，不在 L1。
4. **内容盲审查键**：`Purpose` 字段使"规则层不得含域引导"（TIMING-2 钉）可机械审查——discipline/catalog/output_format 三类之外的内容禁止入 L1（§8.1）。

### 2.2 L2 用户偏好——跨工程偏好档（D13 质询确权的落点）

**载体新建物：用户级偏好档**（权威源机器可读；人类可读导出属 D14 memory 收尾段的透明性面，本卡不建）：

```go
// 设计态：<workspace>/user_profile.v1.json（用户级，工程包外）
type ProfileEntry struct {
    PrefID       string   // 稳定 ID
    Statement    string   // "混音参考轨放在 -6dB 峰值预算" 类确权陈述
    Class        string   // core（进前缀）| conditional（进前缀，带 Condition）
    Condition    string   // 条件化例外挂点：如 "genre=trap"（画像数学收尾段消费）
    EvidenceRefs []string // vit:// 文法：确权时的证据引用
    Status       string   // active | superseded（存储态；前缀只渲染活跃集）
    SupersededBy string   // PrefID
    ConfirmedAt  time.Time
    Source       string   // 确权票据 ID（D13/L2-3 协议产出；本卡只留槽位）
}
```

设计要点：

1. **前缀最小面**：只渲染 `Class=core|conditional 且 Status=active` 的条目（D14 原文"前缀只放蒸馏后的高频强偏好 core"）；长尾偏好按需检索不进前缀（memory 收尾段职责）。
2. **断裂语义（与 L4 的关键差异）**：L2 渲染**活跃集**（用户要的是当前偏好，不是偏好史），活跃集变化=前缀中段变化=**显式断裂事件 `profile_updated`**。质询确权制（D13：惰性时机、冲突触发、不做定期骚扰）天然保证断裂低频——每次断裂换取画像正确性，且必被 AssemblyReport 归因。被推翻条目走 superseded 链（保留历史），存储 append 倾向、渲染活跃集。
3. **聚合键=人**：`user_id`（v1 本地单用户=`local` 常量占位）。**跨设备同步机制不在本卡**：条目带 `ConfirmedAt`/`EvidenceRefs` 足以支撑未来同步器的合并语义，同步器留 D14 memory 收尾卡（§10 OQ-2）。
4. **D14 边界**：置信阈值、升格降格、折扣因子全部不在本卡；`Class/Condition/ConfirmedAt/Source` 四字段就是画像数学的未来挂点——载体先行，数学后置，无返工。

### 2.3 L3 环境实例——环境实例卡

**载体新建物：机器+版本指纹实例卡**：

```go
// 设计态：<workspace>/env_instance.v1.json（机器级，.local 同构，不跨设备同步）
type EnvInstanceCard struct {
    InstanceID   string // 指纹哈希：sha256(OS+arch+内核版本+音频设备指纹+块长/采样率+插件清单指纹)
    SupersededBy string // 指纹失配重建时旧卡留档链接（append：旧卡不删）
    CreatedAt    time.Time
    Components   map[string]string // 各指纹分量的明文（可读性）
    // 归并挂点（不复制内容，只持指纹+指回）：
    PluginSemanticRev string // pluginsemantics 语义索引 revision 指纹（失效检测用）
    CalibrationRefs   []string // PORT-C2 机器校准链产物的 vit:// opaque 透传引用
    // 客观经验蒸馏行（D14 环境键记忆的前缀面）：
    DistilledNotes []EnvNote // {NoteID, Statement, EvidenceRefs, VerifiedAt, Status}
}
```

设计要点：

1. **索引归并边界**：pluginsemantics 语义索引保持其注入快照的既有职责（勘察 §5）；实例卡不复制语义内容，只持 revision 指纹做失配检测——两套真相禁止。
2. **生命周期**：会话开始时构建/校验指纹；失配 → 旧卡 superseded 留档 + 新卡建立 → 显式断裂 `env_changed`。**会话内恒定**是 L3 的 cache 价值：同机器跨会话命中由此层保证（对齐 coding 域"机器环境装一次命中很久"）。
3. **客观经验挂环境键**（D14 表第二行）：`DistilledNotes` 是前缀面；失配降权→复验退役留痕的协议逻辑归 memory 收尾段（本卡只定 Status 字段语义：`verified|downweighted|retired`）。
4. **聚合键=环境**，不跨设备同步（D14 明文）——换机器不灵是**特性**不是故障：客观经验本来就只对该环境负责。

### 2.4 L4 工程账本——append-only 语义账本

**载体新建物：工程包内 append-only 账本**。这是 D6"该工程学到什么"语义层的正身，也是四层中唯一逐轮增长的层：

```go
// 设计态：<projectpkg>/ledger/project_ledger.v1.jsonl（一行一条，文件永不改写）
type LedgerEntry struct {
    EntryID      int64    // 单调递增
    PrevHash     string   // 前条目 canonical sha256（链式 append-only 证明，T-C3 篡改检测）
    Kind         string   // decision | observation_conclusion | identity_confirmation
                          // | goal | constraint | topology_delta | user_override
    Phase        string   // 决策发生时的 decision_phase 注记（披露语义，不复活相位门 §7.2）
    Statement    string   // 结论级陈述（退场 retain 态的落点）
    EvidenceRefs []string // vit:// 文法；外部人类工件（coord/decisions 等）= opaque 透传（G1 #8）
    Supersedes   int64    // 0=无；撤销也是追加（见下）
    CreatedAt    time.Time
}
```

设计要点：

1. **append-only 的字面执行**：文件只追加；撤销/修正=追加带 `Supersedes` 的新条目，**前缀渲染顺序串联全部条目（含被撤销的）**，撤销语义由最新条目承载（模型读到"supersedes #N"以新为准）。裁决理由：任何"活跃集归并渲染"都会造成前缀中段变化——账本层**零中段断裂**优先于渲染整洁（D6 原文"只追加不重写，保证 KV cache 命中不断裂"的字面执行）。代价：被撤销条目占前缀字节——决策条目小、撤销低频，可接受。
2. **头部段（genesis section）**：工程打开时从投影层**只读渲染一次**：TOM 概览（轨道清单摘要行）、总线拓扑（路由树文本化）、交付目标（RLM 目标/交付 profile 引用）——以 `EntryID 1..n` 的 genesis 条目入账（Kind=topology_delta 族）。**拓扑后续变化也是追加 delta 条目**（增轨/改路由各一条），头部条目永不重写——"当前拓扑"视图由读侧归并（渲染层或 pull 查询），前缀只见事实序列。
3. **归并边界（三者分立的收拢，不吞并）**：
   - **ProjectHistory**（projectstore/history，结构版本轴：分支/检查点/worktree 事件）：保持现状入快照分区；账本不复制结构事件，只在相关决策条目的 `EvidenceRefs` 里引用 checkpoint 坐标（vit:// scope）。
   - **journal**（agent_context_snapshots.jsonl 等审计流水）：保持现状；账本不吸收执行流水。
   - **coord/decisions**（人类决策侧，工程包外流程工件）：经确权后由会话侧写入账本（statement+opaque 引用）；不强行注册新 kind（G1 文法不动）。
4. **写入权限**：v1 = 会话侧结构化写入器（观察收敛/确权/交付节点触发，退场执行器 retain 态是其调用方之一，§4.3）；人工写入经 D7 override 通道（§7.3）。**不经 execution_memory 继承链写账本**（22 键 allow-list 不扩，§8.3）。
5. **聚合键=工程**（工程包），append-only 协议（D14 表第三行）。

### 2.5 聚合键与同步语义总表（D14 对齐，验收②收口）

| 层 | 载体 | 聚合键 | 同步语义 | 断裂语义 | 与既有资产关系 |
|---|---|---|---|---|---|
| L1 agent 规则 | ruleset manifest（embed） | 全局 + ruleset 版本 | 随二进制分发 | ruleset_changed（版本失配） | 编译期常量的载体化；命令目录并入 |
| L2 用户偏好 | user_profile.v1.json | 人（user_id，v1=local） | 跨设备同步**预留**（同步器归 memory 收尾卡） | profile_updated（确权事件，低频显式） | mixstyle 保持工程内样式职责，偏好档不装样式模板 |
| L3 环境实例 | env_instance.v1.json | 环境（机器+版本指纹） | **不跨设备同步**（D14 明文） | env_changed（指纹失配重建） | pluginsemantics 索引/PORT-C2 校准链以指纹+引用归并 |
| L4 工程账本 | project_ledger.v1.jsonl | 工程（工程包） | 随工程包走 | **零中段断裂**（仅尾部追加） | ProjectHistory/journal/coord 各守其位，账本持引用 |

---

## 3. 稳定段机制化与前缀指纹（验收③）

### 3.1 AssemblyReport：从意图到机制的消费方

Stable/CacheKey 字段已存在（F1），缺的是**消费方**。新建 PrefixService 装配入口（promptruntime.Build 签名不动，服务在其上包装）：

```go
// 设计态：agent/internal/contextruntime（或 promptruntime 旁，实现卡定归属）
type PrefixService interface {
    // Assemble 产出与 promptruntime.Build 同构的 Assembly，外加装配报告。
    Assemble(ctx context.Context, req PrefixRequest) (promptruntime.Assembly, AssemblyReport, error)
}

type LayerReport struct {
    LayerID     string // rules / profile / env / ledger / catalog
    Version     string // 该层版本身份（ruleset 版本 / 活跃集摘要 / 实例指纹 / 条目链尾 hash）
    CacheKey    string
    ContentHash string // 渲染产物 sha256（字节级诚实）
    Bytes       int
    EntryCount  int
    State       string // rendered | absent | corrupt | skipped
}

type BreakEvent struct {
    LayerID    string // 断裂层；history/snapshot 类动态断裂记 "dynamic"
    Reason     string // §3.4 枚举
    Detail     string // 归因细节（如 "entry 41 appended" / "profile entry p-7 superseded s-2"）
}

type AssemblyReport struct {
    Layers            []LayerReport
    PrefixFingerprint string // 稳定段级联 sha256（四层+目录，不含 history/动态区）
    PrefixBytes       int
    DynamicBytes      int
    Breaks            []BreakEvent // 相对上一轮装配的断裂清单；无事件则空
    HistoryRefs       []HistoryRefEntry // §5.2 伴随索引（退场通道写侧）
}
```

遥测接线：AssemblyReport 三字段（`prefix_bytes`/`dynamic_bytes`/`breaks`）并入既有 `messageLoopModelContextPromptStats` 族遥测（`message_loop.go:905-908` 口径延伸）——**体积计量沿用 model_snapshot_bytes 遥测口径，本设计不编造 token 数**（卡面约束；真实 token 消耗仍以 LLM 日志为准）。

### 3.2 CacheKey / content_hash 双轨语义

- **content_hash（判据轨）**：段渲染产物的 sha256。前缀命中判据**只认字节级 content_hash 级联**——cache 命中的物理前提是字节相同，任何"语义等价但字节不同"（map 遍历序、时间戳、浮点格式化）都按断裂处理。这把"稳定"从语义承诺变成字节承诺。
- **CacheKey（归因轨）**：该层**渲染输入的身份**（ruleset 版本 / 活跃集摘要 / 实例指纹 / 账本链尾 hash+条目数），用于断裂归因（哪层的输入变了）。CacheKey 不变而 content_hash 变 = 装配器 bug（渲染非确定性），红测 T-A4 锁定。
- 双轨关系：content_hash 变化 ⟹ CacheKey 变化或显式 BreakEvent；反之 CacheKey 变化不一定 content_hash 变（版本 bump 内容未变——仍记断裂：归因诚实优先于节省一次 cache 重建）。

### 3.3 CreatedAt 断裂源的动态段分离治理

快照 12 分区（contextruntime.Build 产物）整体定性为**动态区成员**：`CreatedAt` RFC3339Nano 必变（F3）不是要修的 bug，而是动态区的合法属性——治理动作是把动态区**物理挪出稳定段的消息边界**：

| 入口 | 现状 | 治理（装配顺序变更，不动工具面/投影） |
|---|---|---|
| chat（server.go:5680） | `chat_context_snapshot`（Runtime/Stable=false）挂 system 节内，与 Stable system 同渲染一条消息 → 每轮整条 system 字节变 | snapshot 迁至 **user 节尾部**（对齐 agentloop 既有形态）；稳定 system 消息只装四层 Section |
| agentloop 通用路径（:3818） | 已是"稳定 system + 动态 user"✓ | 仅追加四层 Section 进稳定 system；user 节结构不动 |
| agentloop 中性族（:3840） | system 段 Stable=true 但含状态拼装（diagnostic-only/预算/terminal 指令，ccb_model_prompt.go:18-58）+ full-access 规则 | **拆分判据=逐轮字节恒等测试（T-A3 判据的反向应用）**：固定骨架留 system（进 L1 ruleset）；凡逐轮可变的拼接件迁 user 节；full-access 规则若跨轮恒定则留 system（实现卡以恒等测试定线） |
| planner（:143-201） | static+runtime 两节，无 history ✓ | 不动 |

**诚实边界（登记实现卡验证项）**：稳定段独占一条 system 消息依赖 provider 支持多 system 消息或"system+动态 user"序列；若目标 provider 强制单 system，动态区降级为 history 后首条 user 消息——分离语义不变，落点由实现卡按当时代理面定（§10 OQ-5）。

### 3.4 cache 命中验证的可测口径（验收③核心）

蓝图验收原文"前缀 append-only 的 cache 命中验证"落为三条**harness 侧可测判据**（不依赖 provider 报告）：

- **P1 append-only 保持**：无语义事件的相邻两轮，`prefixFingerprint(N-1)` 对应字节串是 `prefixFingerprint(N)` 的**前缀**（字节级 starts-with；账本追加=尾部增长是唯一合法增长形态）。
- **P2 断裂归因完备**：前缀任一字节变化 ⟹ `Breaks` 非空且 `Reason ∈` 下方封闭枚举；`Breaks` 空而 fingerprint 变 = 红（隐式断裂是唯一红线）。
- **P3 动态隔离**：无语义事件时，稳定 system 消息字节逐轮恒等（对消息序列做逐轮 diff 断言）。

**断裂原因封闭枚举**（BreakEvent.Reason，超出枚举=装配器 bug）：

| Reason | 触发 | 频率预期 |
|---|---|---|
| `ruleset_changed` | L1 版本失配（二进制升级） | 极低 |
| `profile_updated` | L2 活跃集变化（确权/升格/推翻） | 低（确权制保证） |
| `env_changed` | L3 指纹失配重建 | 会话内零；跨会话偶发 |
| `layer_appended` | L4 账本尾部追加（**非断裂**：P1 下的合法增长，记账不断缓存） | 每轮可发生 |
| `history_window_slid` | 动态区/history 滑动（不入前缀指纹，报告可见性用） | 每轮 |
| `snapshot_rotated` | 动态区快照换代（同上，仅报告） | 每轮 |

**provider cached_tokens 定位**：作为**辅助遥测**记录（后端报告可用时采集），**不作门**——harness 只能证明"发出去的前缀是 append-only 的"，provider 是否命中是它的自由；但字节级前缀保持是命中的物理前提，可测的就是这个前提（诚实边界写进实现卡回执模板）。

---

## 4. 退场机制统一策略（验收①上）

### 4.1 统一策略：判据、三态、不变式

**退出判据**（信息单元成为退场候选，任一满足）：

1. `turn_end`——轮次边界：该轮原始投影内容（观察 bundle 全文、工具结果全文、长 trace）出动态窗；
2. `budget`——动态区超预算（hot/warm 既有线，F4）→ 最旧/最大优先；
3. `semantic_expiry`——`ExpiresAfterContextChange` 类条件成立（F5-⑤）；
4. `window_slide`——历史窗口滑动（limit=12 / recent_turns=8 / final_gate-only，F5-①②）。

**退场动作三态**（统一五处机制各自的隐式语义）：

| 态 | 动作 | 落点 |
|---|---|---|
| **retain** | 结论级陈述入账本（L4 LedgerEntry，带 evidence refs） | 工程账本 / 会话内 execution_memory（短程结论） |
| **ref** | 原始内容句柄化存续：观察票=既有按 ID 落盘（天然可重拉，F6）；非票类=CAS 化 `evidence://<sha256>`（F12） | vit:// ref 进账本条目 EvidenceRefs / 伴随索引 |
| **drop** | 直接出窗（raw 内容、可由 ref 重拉的冗余、无证据价值重复） | 无痕（审计流水除外） |

**安全不变式（全策略的执法红线）**：任何退场动作必须满足「**结论已留 OR 句柄可重拉**」二者之一；不满足者**拒绝退场**并记 WARN（宁可超预算也不丢证据链——勘察 F4"降级为引用永不删事实"纪律升格为全策略不变式）。执法在退场执行器内机械完成，零 LLM 参与。

### 4.2 五处既有机制归并表（统一策略语义，不推翻既有实现）

| 既有机制（锚点） | 在统一策略中的角色 | 改动面 |
|---|---|---|
| chat 截尾 limit=12（server.go:5686） | `window_slide` 判据的执行器之一 | 不动；其丢弃面改经 ExitExecutor 检查（不变式执法） |
| 快照 recent_turns=8 + 折叠（context.go:328/:349-389） | 动态窗维护（drop 态的既有实现） | 不动 |
| 观察账本窗口 24 对 + rejected 持久（model_projection.go:317） | **retain 态的既有实现**（账本=观察域的跨轮结论面） | 不动；L4 账本是其结论级条目的**跨会话**延伸（观察账本活在 loop 生命周期，L4 账本随工程包持久） |
| 预算降级冷引用 audit_snapshot://（model_projection.go:197/:209/:225） | `ref` 态的**装配层内部**形态（冷引用指回审计快照） | 不动；跨轮持久句柄用 vit://（重拉通道地址），两形态并存、职责不同 |
| execution_memory ExpiresAfterContextChange（execution_memory.go:75 族） | `semantic_expiry` 判据的既有实现 | 不动 |

归并原则一句话：**五处机制照旧运转，统一的是"什么该退场"的判据语言与"退成什么"的三态词汇；执行器是增量前置的执法点，不是替换层。**

### 4.3 退场执行器接口

```go
// 设计态：agent/internal/contextruntime（实现卡定归属）
type ExitExecutor interface {
    // OnTurnBoundary 在轮次边界被装配层调用（确定性，零 LLM）。
    OnTurnBoundary(ctx context.Context, ev TurnBoundaryEvent) ExitReport
}

type TurnBoundaryEvent struct {
    TurnID string
    Window WindowState     // 动态窗内容清单（消息/观察票/工具结果/trace，含字节数）
    Budget BudgetState     // hot/warm 占用
}

type ExitUnit struct {
    Kind string // observation_bundle | tool_result | trace | history_message
    ID   string // observation_id / CAS hash / 消息序号
}

type ExitAction string
const (
    ExitRetain ExitAction = "retain" // 结论入账本（LedgerEntry 由执行器产出待写入器落盘）
    ExitRef    ExitAction = "ref"    // 句柄化（vit:// or 既有冷引用）
    ExitDrop   ExitAction = "drop"   // 出窗（不变式已满足：结论已留或句柄在）
)

type ExitDecision struct {
    Unit        ExitUnit
    Action      ExitAction
    Reason      string       // turn_end | budget | semantic_expiry | window_slide
    LedgerEntry *LedgerEntry // retain 态产物
    HandleRef   string       // ref 态产物
}

type ExitReport struct {
    Decisions  []ExitDecision
    Violations []string // 被拒绝的退场（不变式不满足）+ WARN 文本——进遥测，实现卡回执必查项
}
```

L1-5 衔接：接口按"轮次边界事件"抽象（§7.1）——v1 由 agentloop/chat 装配处触发，pull loop 由其循环语义触发，执行器本体不感知 harness 形态。

### 4.4 兜底语义：防"激进滑窗后模型引用已退场 ref"

三道防线（对应卡面"防激进滑窗后引用已退场 ref"）：

1. **写入侧解析预检**：账本条目/伴随索引登记的 evidence refs 在写入时经 `agentprotocol.ParseRef` 三态解析 + 物化层可 Resolve 预检（F9 契约）；不可解析 → 降级 opaque 留存 + WARN 一次（复用 G1 #8 语义）——**残骸不入账本**，账本里的 ref 从第一天就是结构可重拉的。
2. **读侧三态诚实响应**（§5.3）：重拉必回 `{resolved|stale|missing}`，missing 不静默不猜，附账本反向指引。
3. **规则层纪律条款**（L1 ruleset 新段，语义归本卡、文本归实现卡）：引用历史观察前，若该观察不在当前窗口，先经重拉确认其 state，不得凭记忆断言其内容——条款 ID 进红测素材（T-B7 断言模型行为受条款约束的 prompt 面存在性；模型实际遵循度属运行行为，不进机械断言）。

---

## 5. 结构化重拉通道（验收①下）

### 5.1 形态裁决：不新增工具动词

`ref.query` 的既有谓词面已能精确重拉（HashPredicate 身份等值 / SnapshotPredicate exact=observation_id / ScopePredicate），`Expand` 已能给 handle/summary（F9）。**重拉通道 = 装配层写侧登记 + 既有读侧组合**，工具面 JSON schema、谓词模型、物化契约零改动（卡面"L1-3 工具面接口冻结"约束的字面执行）：

- **写侧（本设计新建）**：账本 EvidenceRefs 结构化 + 历史 refs 伴随索引（§5.2）；
- **读侧（已合入，复用）**：模型经 `ref.query`（等值谓词 + expand=summary）或 `ref.diff`（变更回看）重拉——**重拉就是 query，不是新动词**。

### 5.2 历史 refs 句柄化：伴随索引（残骸 → 句柄的渐进形态）

历史消息是不可变审计面，**不改写文本**（remember 只存文本对的现状保持）；装配器在装配历史时对 assistant 消息中的 evidence refs 做**解析注记**，产出伴随索引进 AssemblyReport（§3.1 `HistoryRefs`）：

```go
// 设计态
type HistoryRefEntry struct {
    Ref         string // 原文字面（含 opaque 残骸）
    ParseState  string // parsed | legacy | opaque（agentprotocol 三态直通）
    Handle      string // parsed/legacy 且可 Resolve 时：evidence:// 句柄或观察票路径
    Freshness   string // 物化层透传（current/material_reuse/stale）；不可解析为空
    LastSeenTurn string
    LedgerEntry int64  // 相关账本条目（可回指时）
}
```

模型下一轮从动态区（装配报告的模型可见投影，实现卡定披露形态与字节预算）看到"哪些历史 refs 结构可重拉"——防御解析税（L1-1 痛点②③）的消除方式是**给模型结构化目录**，不是替模型改历史。opaque 残骸照旧留文本（宽容透传），但进不了伴随索引的可重拉面—— Tax 递减的度量=伴随索引覆盖率（遥测：parsed 占比，首审报告口径）。

### 5.3 三态重拉响应契约（退场一致性验收的语义核心）

重拉结果必须三态可判定（设计态响应契约，读侧装配在 ref.query 结果之上、由实现卡包装为模型可读注记）：

| state | 语义 | 响应必带 | 红线 |
|---|---|---|---|
| `resolved` | 票/CAS 工件在且读回成功 | ref、handle、bytes、freshness、summary（expand 预算内） | 内容 hash 与原票一致（T-B1） |
| `stale` | 内容在但物化层已标脏（G2 语义透传） | 同上 + `freshness=stale` 原样 | **不升级**为 current（T6 纪律的重拉版） |
| `missing` | 票/工件不存在（sessionDir 清理等） | ref、`pointer{ledger_entry_id, turn_id}` | **不静默、不猜**：指引账本行（"该观察产生于 turn N，结论已入 entry M"） |

`missing + 指引` 是"句柄重拉一致性"验收（蓝图 L1-4 行）的语义底线：**退场承诺的可信度不在于句柄永远活着，而在于句柄死了也死得明明白白、且有结论兜底**——这正是 retain 态存在的理由。

### 5.4 CAS 化的边界：少数路径，不是默认路径

D6 本意是"结论 + ref 在，原文可不要"——**默认退场 = retain（结论入账本）+ drop（原文出窗）**。观察票已天然落盘可重拉（F6），覆盖观察域绝大头。CAS 化（`projectstore.PutEvidence` → hash 段）只用于：**无票且被判 retain-worthy 的长文本**（如长工具结果中的关键证据段）。v1 写放大受控：CAS 化是例外路径，实现卡按单元类型白名单启用，不做全量快照——否则退场机制自己变成上下文膨胀器。

### 5.5 与 observe/CCB 的边界（重拉不构成观察声明）

沿 QUERY_ENGINE §4.1 边界表的重拉侧注记：重拉走 query+expand（廉价看一眼，无审计回执）；语义深读仍走 observe（view=唯一带审计回执的披露通道）。因此**重拉成功只恢复引用材料，不恢复观察事实的 readiness**——freshness 三态原样透传，缺/stale 不因重拉而升级。退场语义与披露预算互不越界：重拉结果进动态区受既有动态区预算管（不占 MaxDisclosureBytes——那是 observe 的）。

---

## 6. 测试计划（验收③④：红绿形态 T 系列）

全部落在 contextruntime/promptruntime/装配入口的 Go 测试与合成受控会话；隔离工作区纪律（t.TempDir / 脚本隔离工作区，AGENTS §10）；**红=现状锚点可复现，绿=实现卡交付判据**。沿 QUERY_ENGINE §6 范式编号。

### 6.1 T-A 前缀机制组（验收③落点）

| # | 测试 | 红（现状） | 绿（断言） |
|---|---|---|---|
| T-A1 | 前缀扩展性：受控会话 N 轮（无语义事件脚本），逐轮比对稳定 system 消息字节 | **现状红**：chat 入口 snapshot 挂 system 节 + CreatedAt 必变（context.go:133）→ system 消息逐轮变 | P1 成立：fingerprint(N-1) 字节串是 fingerprint(N) 的前缀；`breaks` 为空或仅 `layer_appended` |
| T-A2 | 断裂归因：构造五类语义事件各一次（ruleset bump / 确权写入 / 指纹失配 / 账本追加 / 窗口滑动） | —（无报告面） | 每次事件 `Breaks` 恰好报告对应 `Reason` 与 LayerID；无事件轮次 `Breaks` 空 |
| T-A3 | stable/unstable 物理分离：对两入口装配产物逐轮 diff | **现状红**：chat system 消息含 snapshot、中性族 system 含状态拼装 | 稳定 system 消息在无事件轮次字节恒等；动态内容全部位于 user 节（P3） |
| T-A4 | 双轨一致性：构造 CacheKey 不变而渲染输入微变的装配器缺陷 | — | content_hash 变而 CacheKey 未变 → 测试红（渲染非确定性检测） |
| T-A5 | 遥测接线：AssemblyReport 三字段进 promptStats | — | prefix_bytes/dynamic_bytes/breaks 出现在既有遥测口径，model_snapshot_bytes 口径不变 |

### 6.2 T-B 退场一致性组（验收④落点，蓝图"句柄重拉一致性"）

| # | 测试 | 断言 |
|---|---|---|
| T-B1 | 退场后重拉 resolved：turn k 产生观察票 → turn k+n 票出窗 → ref.query（snapshot exact=observation_id）+ Expand | state=resolved；内容与原票 hash 一致；freshness 透传 |
| T-B2 | 退场后重拉 stale：观察后构造变更 → 物化层标脏 → 重拉 | state=stale 且 freshness=stale 原样，**无任何升级路径** |
| T-B3 | 退场后重拉 missing：隔离工作区副本中移除票工件（§10 防污染：副本上操作）→ 重拉 | state=missing 且 pointer 指向正确账本条目；非静默（响应含指引） |
| T-B4 | 结论先于退场（不变式执法）：构造无结论无句柄的退场候选 | ExitReport.Violations 非空、退场被拒、WARN 落遥测；动态区宁可超预算 |
| T-B5 | 账本 append-only：逐轮快照账本文件字节序列 | 每轮文件=上轮文件+尾部追加（字节级 starts-with）；任何中段改写=红（含撤销路径——撤销是追加 Supersedes 条目） |
| T-B6 | 伴随索引三态分布：含混合 refs（parsed/legacy/opaque）的历史装配 | HistoryRefEntry.ParseState 与 agentprotocol 直通一致；opaque 不进可重拉面但文本残骸保留 |
| T-B7 | 纪律条款 prompt 面存在性：装配产物含 §4.4-3 条款文本（ruleset 段落 ID 定位） | 条款在稳定段且随 ruleset 版本化（模型遵循度属运行行为，不进机械断言） |

### 6.3 T-C 四层载体组

| # | 测试 | 断言 |
|---|---|---|
| T-C1 | 层序与装载：四层 Section 顺序固定 rules→profile→env→ledger（→目录） | 顺序断言；缺层 fail-open（absent 渲染跳过 + 报告标注） |
| T-C2 | 聚合键失配行为：user_id 失配 / env 指纹失配 | 显式断裂（profile_updated / env_changed）+ L3 走降权重建路径；**非静默沿用** |
| T-C3 | 账本链式 hash：构造 prev_hash 链，篡改中段条目 | 校验器检出（append-only 证明可机械验证） |
| T-C4 | 持久化兼容：旧工程包/旧会话体（无四层文件）加载 | 空层缺省语义、零报错、往返一致（§8.3）；既有持久化 schema 零变更 |
| T-C5 | 渲染确定性：同输入重复装配 ×100 | content_hash 全等（前缀字节承诺的前提） |

### 6.4 端侧烟测归属

本设计零代码改动，不适用烟测门槛（沿 QUERY_ENGINE §6.5 先例）。实现卡收口线：IMPL-D 接线后按 AGENTS §5 过 `dev_agent_smoke.ps1` 体系真实栈烟测（三件套拉起 → 真实工程 → 受控多轮会话 → 断言退出码 0 + 遥测字段在位）；涉装配顺序变更（chat 入口迁移）影响呈现面，回执须显式声明端测覆盖边界（渲染面纪律）。

---

## 7. L1-5 衔接面（验收⑤上）

### 7.1 pull loop 消费面

- **前缀服务即插即用**：新 harness 的装配入口复用同一 promptruntime Section 机制——四层前缀作为 static/session Section 族挂载，pull loop 的工具循环每轮只重算动态区。PrefixService（§3.1）作为**依赖注入点**：L1-5 harness 注入其循环的轮次边界与预算状态，不自建装配报告/指纹逻辑。
- **退场接口按轮次边界抽象**：`OnTurnBoundary(turn_id, window_state)` 不假设"轮次=一次模型调用"——pull loop 可按工具循环节或逻辑任务节触发；执行器与判据不感知 harness 内部形态（§10 OQ-1：pull loop 的 turn 语义精确定义归 L1-5 设计卡，本接口留其注入缝）。
- **新旧并存（G3 前提）**：四层载体与退场通道对旧 harness（agentloop/chat）与新 harness 同构可用——切换 A/B 时前缀命中数据可对照采集（G3 质量门的输入面）。

### 7.2 相位披露位处置（推进面回收归 L1-5，本卡只定披露语义）

| 相位现役面（F8 锚点） | 披露语义裁决 | 去向 |
|---|---|---|
| ledger `decision_phase` 字段 | 账本条目元数据：决策发生时的相位注记，append 后不重写 | L4 LedgerEntry.Phase（本卡定） |
| GATE PATH / TERMINAL TURN / 预算指令披露位（ccb_model_prompt.go） | 动态区状态指令：逐轮可变，**不进前缀** | 动态区（本卡定归属；回收时机归 L1-5） |
| 模型剖面选择（helpers.go:116-129） | 动态区行为：剖面只影响快照分区裁剪参数，切剖面不触前缀字节 | 不动；退场策略与剖面正交 |
| AdvancePhase 推进面 | 半退休状态的"可回收面"——回收=相位降级冷启动第一轮（路线图 D15） | **归 L1-5**（本卡不动） |

### 7.3 D7 工程前缀生成器（VIT.md）定位

- **权威源 = L4 账本**（本卡定结构）：VIT.md 是账本的人类可读导出层（生成主体 + 明确标记 override 区，D7 原文）。
- **生成器留后续卡**：本卡只定权威源结构与 override 回灌语义——人工改动回灌 = 账本 `user_override` 条目追加（append-only 保持，机器写人可改谱系的一致落点）。
- **永不过期性质的来源**：导出由账本渲染（手改不进权威源），账本由结构化写入器追加——过期前缀污染每一轮的风险在权威源头被切断（D7 立论）。

---

## 8. 红线对账与持久化兼容（验收⑤下）

### 8.1 内容盲对账（TIMING-2 钉 / D12 瘦身原则）

| 层 | 内容盲合规判据 | 机械审查点 |
|---|---|---|
| L1 agent 规则 | **不得含域引导/处理知识**（"低音轨在 trap 里怎么处理"一行不写——D12：处理知识归模型参数）；只装纪律、目录、输出格式 | RuleSection.Purpose ∈ {discipline, catalog, output_format}；越界内容 manifest 拒载（T-C1 扩展断言） |
| L2 用户偏好 | 域内容只以**确权事实**身份进入（用户确认过的偏好），不预设领域假设 | 条目必须带 EvidenceRefs+Source（无确权来源不入档） |
| L3 环境实例 | 环境**事实**（指纹、校准、复验过的踩坑），不是处理建议 | DistilledNotes 必须带 VerifiedAt/EvidenceRefs |
| L4 工程账本 | 确权事实 / 工程事实 / 硬约束（D12 三样），Kind 枚举刻意不含"混音建议"类 | Kind 封闭枚举校验 |

路线图风险条目"全局耦合遗漏"的 D8 解法（全局不变量住常驻前缀或强制披露）**不在本卡展开**——不变量清单属编译器层（L2-1），本卡只保证：届时它以 `constraint` Kind 条目进 L4，机制就位。

### 8.2 投影/CCB 纪律对账

- **peer 不是链**：四层前缀是投影产物的只读消费端（渲染 TOM/RLM 摘要行进账本头部），不引入任何"DAD→前缀"链式语义；缺失投影 → 头部段该部分 absent（不阻塞、不臆造）。
- **不展开 raw package**：头部段渲染消费 LLMContext/摘要行（F11 消费纪律同款）；账本/伴随索引永不含 raw PCM/DAD 包/mutation 权限。
- **CCB 不做自动 view 选择**：前缀装配不代发 observe、不隐式选 view；重拉是模型显式发起（§5.5）。
- **stale 原样保留**：伴随索引与重拉响应透传物化层 freshness，无升级路径（T-B2）。

### 8.3 §11 持久化兼容义务逐项

| 新增面 | 旧状态缺省语义 | 兼容动作 |
|---|---|---|
| user_profile.v1.json / env_instance.v1.json / project_ledger.v1.jsonl | **新增文件**，缺文件=空层（absent 渲染跳过，非错误）——旧工程包零迁移 | T-C4 往返测试 |
| 账本条目 schema 演进 | 未来加字段必须定义旧条目缺省语义（Kind 封闭枚举外的未知 Kind=fail-closed 拒载该条目+WARN，不静默丢弃） | 写进写入器实现卡验收条 |
| 既有持久化（conversation/history/snapshot JSONL/execution_memory） | **零 schema 变更** | T-C4 断言 |
| AssemblyReport/伴随索引 | 新增遥测与运行时面，不落既有记录格式 | — |
| 22 键 allow-list（context.go:1002-1015） | **不扩**：账本由独立写入器写，不经 execution_memory 继承链 | 本设计裁定 |

### 8.4 与既有纪律对照自检表

| 纪律 | 本设计的遵守方式 |
|---|---|
| 投影是 peer 不是链（AGENTS §5） | §8.2：前缀=只读消费端，无链式新语义 |
| 不展开 raw package | §8.2：头部段/伴随索引消费 LLMContext 与摘要行 |
| G1 文法不私改 | refs 全部 vit:// 文法 + opaque 透传，未注册不新造（§2.4-3） |
| L1-3 接口冻结 | §5.1：零工具动词/参数/schema 改动，重拉=query+Expand 组合 |
| G2 物化契约 | 重拉读侧只走 MaterializedStore.Resolve/Query 面，失效归物化层（F9） |
| D14 画像数学不在本卡 | §2.2-4/§2.3-3：只定条目结构与挂点，数学归 memory 收尾段 |
| 体积计量不编数 | §3.1：沿用 model_snapshot_bytes 遥测口径，token 数不造 |
| 观察预算止损（D15 第一天进设计） | 重拉成本分级沿 D5（index 级，不触发 render/probe）；退场执行零 LLM；§4.1 不变式防证据链丢失 |
| 测试不写源码树/不碰权威 fixture | §6 全部隔离工作区；T-B3 在副本上构造 missing |

---

## 9. 实现排程建议（决策侧裁剪用）

| 卡 | 内容 | 验收线 | 依赖 |
|---|---|---|---|
| L1-4-IMPL-A | PrefixService + AssemblyReport + stable/unstable 物理分离（chat snapshot 迁 user 节、中性族状态拼装拆分）+ 遥测接线 | T-A1/A3/A4/A5 绿 | 无（纯装配器域，与一切卡并行） |
| L1-4-IMPL-B | 四层载体：ruleset manifest（go:embed 迁移）+ user_profile + env_instance + L4 账本写入器/读取器/校验器 | T-C1~C5 绿；L1 内容盲审查通过 | A |
| L1-4-IMPL-C | 退场执行器 + 账本 retain 接线 + 历史 refs 伴随索引 + 三态重拉响应包装 | T-B1~B7 绿 | A、B（读侧复用已合入 queryengine bootstrap） |
| L1-4-IMPL-D | chat/agentloop 双入口全接线 + 真实栈烟测收口（`dev_agent_smoke.ps1` 体系扩场景）+ 渲染面边界声明 | 烟测 exit 0；AGENTS §5 门槛 | C |
| L1-5 钩子 | PrefixService 注入 + OnTurnBoundary 触发点 + 相位推进面回收 + 披露位迁移 | 归 L1-5 设计/实现卡 | A（接口就位即可启动设计） |

排序建议：A 最先（断裂源治理独立增值：chat 入口前缀从"每轮全断"到"稳定段恒等"，不依赖四层载体）；B/C 依序；D 收口。B 的 L2 层与 D13/L2-3 质询协议卡对齐接口（协议产出 → ProfileEntry 写入），两者文件域不相交可并行。

---

## 10. 开放问题与依赖清单（停止条件对照）

**停止条件核验**：卡面"发现 pull loop 形态或物化层契约未定版到影响退场接口设计"——物化层契约已定版且冻结（F9 三函数）；pull loop 形态未定版（L1-5 未设计），但退场接口按轮次边界事件抽象后**不依赖其内部形态**（§7.1）。**停止条件不触发**。以下登记上交：

| # | 开放问题 | 建议裁决 | 影响 |
|---|---|---|---|
| OQ-1 | pull loop 的"轮次"语义精确定义（工具循环节 vs 逻辑任务节） | 归 L1-5 设计卡；本设计的 OnTurnBoundary 抽象已留注入缝 | 不阻塞本卡 |
| OQ-2 | 用户偏好跨设备同步机制（user_id 从 `local` 占位转正） | 归 D14 memory 收尾段；ProfileEntry 结构已含合并所需字段 | 不阻塞 |
| OQ-3 | CAS 化白名单的单元类型面（§5.4 例外路径的范围） | 实现卡（L1-4-IMPL-C）按退场单元实测数据裁定，决策侧复核 | 小 |
| OQ-4 | provider 侧 cached_tokens 遥测可用性 | 辅助遥测不作门（§3.4 已裁定）；后端报告可用时采集 | 无阻塞 |
| OQ-5 | 多 system 消息的 provider 兼容性（§3.3 诚实边界） | 实现卡按当时代理面定降级形态（动态区入 user 节） | 小 |
| OQ-6 | 伴随索引的模型可见披露形态与字节预算（§5.2） | 实现卡定（进动态区，受既有动态区预算管）；预算超限时优先披露 parsed 可重拉子集 | 小 |

前置依赖（排程序）：G1 L0 文法 ✅ → L1-3 查询面 ✅（已合入，接口冻结）→ **L1-4-IMPL-A → B → C → D** → L1-5 新 harness（钩子就位）。与 L1-2 物化层 v1 替换无硬依赖（v0 bootstrap 读侧已够重拉正确性；事件实时性属物化层演进）。

---

*本文档由 L1-4-DESIGN-1 卡（PC 执行侧会话③，GLM-5.3/L2，零代码设计档）产出；规格锚路线图 D6/D14/D15 与 G1 ruling，接口冻结锚 QUERY_ENGINE_V1_DESIGN，事实锚 L1-4-RECON-1 勘察报告（行号以领取时 HEAD 7582976b 复核，含两处包路径修正：分层预算/冷引用/账本窗口锚点实属 `contextruntime/model_projection.go`，相位披露/中性族拼装锚点实属 `agentloop/ccb_model_prompt.go`）。*
