# 查询引擎与 audio-grep 工具面接口设计（QUERY_ENGINE_V1_DESIGN，L1-3）

> **状态：规划（未实现）**。本文档是路线图 [AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md](AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md) L1-3 段的接口层设计（卡 `coord/cards/todo/2026-09-27-L1-3-DESIGN-1.md`），CURRENT-STATE 登记由决策侧收口。零代码改动；全部接口为设计态签名，落地按 §9 排程拆卡。
>
> 规格锚：G1 裁定 [coord/decisions/2026-09-27-g1-ref-schema-ruling.md](../coord/decisions/2026-09-27-g1-ref-schema-ruling.md) 的 L0 文法（实现已落 `agent/internal/agentprotocol/refschema.go`，分支 port/refschema-l0-1）。事实基线：CCB-VIEW-RECON（18 view/7 缺口/零尺度参数化）、L1-2-RECON（触发链/17 依赖边/19 事件/10 物化种子）、L1-1-RECON（31 生成点/7 组不一致）、[OBSERVATION_PROJECTION_MANIFEST.md](OBSERVATION_PROJECTION_MANIFEST.md)（peer 拓扑权威）。
>
> 一句话定位：查询引擎是 L0 ref 寻址之上的**只读检索层**——中央段索引管"哪些 ref 存在"，各投影局部载荷索引管"按内容字段过滤/排序/topK"，物化层管"内容与新鲜度"，工具面以 observe / query / diff.evidence 三动词暴露给 LLM 并明示成本分级。

---

## 0. 设计输入与既有事实（不自创现状）

以下事实全部来自勘察报告与代码锚点，本设计只在其上做接口决策。

| # | 事实 | 出处 |
|---|---|---|
| F1 | L0 文法已定版且有实现：`vit://kind/scope/window@snapshot#hash`，window 与 hash 双段强制（`t=all` 显式；`#-`=显式未 CAS 化），保留字符 `%/@#:` 百分号转义，解析三态 parsed/legacy/opaque；`ParseRef`/`FormatRef`/`Ref.Validate`/`RegisteredRefKinds` 已可用 | G1 ruling + refschema.go（port/refschema-l0-1） |
| F2 | 注册表现存 kind 仅 `{dom, fxm, com, rlm}`（A 类四变体投影内容 ID）；`mom/tim/tom/epm` 无自生成 ID，`acousticpackage` 以 SourceHash/Fingerprint 作身份面，DAD 系（`dad.l3.*`/`dad.l2_render_probe`/`dad.compressor_dual_tap`）是 legacy 前缀非注册 kind | L1-1 §2/§4；refschema.go 注册表 |
| F3 | CCB 已是 pull 模型雏形：18 个语义 view，模型经 `ccb.observation_catalog`/`ccb.observation_request` 显式请求；选择面 5 参数（view_ids/target/freshness_class/max_disclosure_bytes/max_items），**尺度面零参数化**（无时间窗/无轨道谓词/无 per-view 粒度/无 topK）；7 条缺口带锚点 | CCB-VIEW-RECON §0-§3 |
| F4 | 九投影全部请求驱动惰性现算，无事件驱动重算；物化插入点天然存在（`FinalizeObservationContext` 单点编排，`agent/internal/mixboard/catalog.go:59-86`）；脏传播雏形=free-state 台账按受影响域打 stale（M2）+ shadow scope 映射表（M3）；物化种子 10 项（F5 铃行级 freshness、acousticpackage Store 交叉失效、evidence CAS、L2 probe 指纹缓存等） | L1-2-RECON §1/§4/§5 |
| F5 | CAS 已有：`projectstore.PutEvidence` 返回 `evidence://<sha256>` 内容寻址句柄，pin 不可逐；观察票 `obs_*` 落盘可按 ID 重拉 | L1-2 M7；projectstore/evidence.go |
| F6 | CCB view 键（点分）与 evidence ref（冒号 scheme）是两套正交命名法，靠装配代码手工对应——G1 L0 落地后统一到 ref 主键 | CCB-VIEW-RECON §3.7；L1-1 §2.F |
| F7 | `GetViewsForDimension(dimension, targetID string) []string`（按诊断维度反查 view）全库仅测试调用、无生产接线——可复用为 observe 目录的谓词入口 | capabilitycontext/free_state_observation.go:1143-1155；CCB-VIEW-RECON §3.6 |
| F8 | 拓扑红线：九投影是 DAD 之上同级 peer，不是链；CCB 不做自动 view 选择；缺失/partial/stale 原样保留不升级 readiness；Context Runtime 优先消费投影自带 LLMContext，不展开 raw package | OBSERVATION_PROJECTION_MANIFEST §1；AGENTS.md §5 |

**D2/D5 裁定回顾（本设计的两条宪法）**：

- **D2 统一寻址不统一存储引擎**：ref schema 统一的是寻址语言；各投影层自建局部索引（倒排/立方体各随其便），查询引擎面向 ref 谓词跨层路由。
- **D5 冷热分离**：索引查询=毫秒级 / compile（结构断言）=亚秒级 / probe（真实 render）=秒级；工具面明示成本分级，agent 知道哪个动作便宜哪个贵。

---

## 1. 总体形态：两层索引 + 一个只读物化读口

```
                     ┌────────────────────────────────────────────┐
   LLM 工具面         │  observe（CCB view 披露，既有双工具参数化升级）  │
 （§2）              │  query（即席谓词 audio-grep，新）               │
                     │  diff.evidence（ref 差分，新）                 │
                     └───────────────┬────────────────────────────┘
                                     │ 只读
                     ┌───────────────▼────────────────────────────┐
   查询引擎           │  路由器 Route()：谓词类 → 中央索引/局部索引     │
 （agent/internal/    │  中央段索引：ref 五段 + 声明标量载荷            │
  queryengine，新包）  │  载荷索引契约：ProjectionIndex（各投影自建）    │
                     └───────┬───────────────────────┬────────────┘
                             │ 段谓词/候选集            │ 载荷谓词/排序
                     ┌───────▼───────┐       ┌───────▼────────────┐
                     │ 中央索引        │       │ 投影局部索引 ×N      │
                     │（引擎自有）     │       │（契约实现，随其便）   │
                     └───────┬───────┘       └───────┬────────────┘
                             │        数据喂给
                     ┌───────▼───────────────────────▼────────────┐
   物化层（L1-2 产出） │  MaterializedStore 读侧三函数（§5）           │
                     │  失效/新鲜度归物化层管，查询引擎不自行判断        │
                     └────────────────────────────────────────────┘
```

三条职责边界，全部对齐既有纪律：

1. **查询引擎不判断新鲜度**——物化层给状态（current/material_reuse/stale，F5 铃三分语义），查询原样透传（F8 纪律的查询版：stale 行不删除、不升级、不过滤，除非谓词显式要求）。
2. **查询引擎不装配披露**——LLMContext 装配、两层裁剪、预算执法、审计回执是 CCB（observe）的职责；query 只回 refs + 声明标量（§4 边界）。
3. **查询引擎不触发计算**——现状 observe 请求可能触发 finalize* 现算（compile 级）；query 永远只读物化结果，物化缺失即如实报 empty+deferred，不背地里重算（成本分级的诚实前提）。

---

## 2. 三动词 API（验收①：签名定到参数级）

### 2.0 动词语义划分（先定边界，再给签名）

| 动词 | 语义 | 成本分级（D5） | 与 roadmap 对齐表的对应 |
|---|---|---|---|
| **observe** | 预编排披露：从 view 目录取装配好的语义包，含 LLMContext digest、预算执法、审计回执 | index（物化命中）→ compile（物化 miss 触发现算）；**响应必带 `actual_cost_class` 与 `recomputed` 计数** | read(file:line) 定点深读（参数化 view 侧） |
| **query** | 即席谓词：audio-grep。ref 五段谓词 + 载荷谓词 + 排序/topK/分页，返回 ref 列表 + 声明标量 + 可选展开句柄 | index（毫秒级目标）；载荷索引未注册时降级为"中央候选集 + 逐条解析"（compile 级，响应明示） | grep / glob 粗定位 |
| **diff.evidence** | 差分：identity 级（两快照 ref 集合差，index）→ content 级（载荷差异，委托既有差分承载者：COM change_delta / FXM / before_after，compile~probe） | identity=index；content=compile 或 probe（需新 render 时） | 编译器/测试套件的第一跳（FXM 差分回归的地基） |

工具面命名遵循既有惯例（`ccb.observation_*` 点分族）：observe = **既有 `ccb.observation_catalog` + `ccb.observation_request` 的参数化升级，工具名不变**（保 prompt 契约，CCB-VIEW §2 的结构化协议照旧）；query = 新工具 **`ref.query`**；diff.evidence = 新工具 **`ref.diff`**。

### 2.1 谓词模型：L0 五段的结构化查询形态

全部段谓词直接对应 refschema.go 的 `Ref` 字段，不自创第二套寻址：

```go
package queryengine // agent/internal/queryengine（新包，规划态）

// ---- 段谓词（每个字段对应 L0 ref 的一段；nil/零值 = 该段不约束）----

// KindPredicate 约束 kind 段。空集 = 全部注册 kind。
// 未注册 kind 一律拒绝（ErrUnknownKind），不静默返回空集——
// "空结果"与"kind 不存在"对模型是两种完全不同的信号。
type KindPredicate struct {
    Kinds []string // ⊆ agentprotocol.RegisteredRefKinds()；校验在 Route() 入口
}

// ScopePredicate 约束 scope 段（scope_kind:scope_value）。
type ScopePredicate struct {
    Kind      string   // 精确匹配 scope_kind，如 "track"、"project"、"bus"
    Values    []string // scope_value 精确集合（轨道 ID / 总线路径…）
    Prefix    string   // scope_value 前缀匹配（总线子树：bus:master/… 的树形遍历）
    ValueSet  bool     // Values 非空时生效
    PrefixSet bool
}

// WindowPredicate 约束 window 段（采样点权威层，D3；秒/小节输入由工具面
// 换算后进入，引擎只见采样点）。
type WindowRelation string
const (
    WindowIntersects WindowRelation = "intersects" // 查询窗与 ref 窗有交集（含端点相接）
    WindowContains   WindowRelation = "contains"   // ref 窗完整落在查询窗内
    WindowWithin     WindowRelation = "within"     // ref 窗完整覆盖查询窗（粗粒度观察发现）
)
type WindowPredicate struct {
    Relation    WindowRelation
    SampleStart int64  // 含
    SampleEnd   int64  // 含；SampleStart==SampleEnd==0 且 AllTime=false 仍视为显式点查
    AllTime     bool   // true = 只匹配 t=all 的 ref
}

// SnapshotPredicate 约束 snapshot 段（数据版本轴）。
type SnapshotMode string
const (
    SnapshotLatest      SnapshotMode = "latest"        // 物化层当前已提交快照（默认）
    SnapshotExact       SnapshotMode = "exact"          // 精确 revision/observation_id
    SnapshotAtOrBefore  SnapshotMode = "at_or_before"   // ≤ 给定 revision 的最近一份（历史回溯）
)
type SnapshotPredicate struct {
    Mode     SnapshotMode
    Revision string // exact / at_or_before 必填；latest 留空
}

// HashPredicate 约束 hash 段（内容身份轴，CAS 天然支持）。
type HashPredicate struct {
    SHA256 string // 精确内容身份（16hex，不含 "sha256:" 前缀）
    Any    bool   // true = 不限（含 #-）；false 且 SHA256 空 = 排除 #-（只要已 CAS 化的）
}

// ---- 载荷谓词（投影声明并提供的标量字段；字段名带 kind 前缀防跨层撞名）----

type PayloadOp string
const (
    PayloadEq  PayloadOp = "eq"
    PayloadNe  PayloadOp = "ne"
    PayloadGt  PayloadOp = "gt"   // 数值型
    PayloadGe  PayloadOp = "ge"
    PayloadLt  PayloadOp = "lt"
    PayloadLe  PayloadOp = "le"
    PayloadIn  PayloadOp = "in"   // 字符串集合
)
type PayloadCondition struct {
    Field string    // 如 "mom.masking.candidates.median_margin_db"（§3.2 命名法）
    Op    PayloadOp
    Value float64   // 数值型条件用
    Str   string    // eq/ne/in 的字符串形态（枚举值）
    Set   []string  // in 条件
}
```

**排序、topK 与分页**：

```go
type SortDirection string
const (SortAsc SortDirection = "asc"; SortDesc SortDirection = "desc")

type SortKey struct {
    Field string        // "ref.window.sample_start"（段字段的受控别名）或载荷字段名
    Dir   SortDirection
}

// RefQuery 是 query 动词的完整输入（Go 函数面）。
type RefQuery struct {
    Kinds    KindPredicate
    Scope    *ScopePredicate
    Window   *WindowPredicate
    Snapshot SnapshotPredicate // 零值 = latest
    Hash     *HashPredicate
    Payload  []PayloadCondition // AND 语义（v1 不做 OR；OR 用两次 query 由模型组合）
    Sort     []SortKey          // 默认：kind, scope_kind, scope_value, window.sample_start
    Limit    int                // topK；0 = 默认 50；硬上限 500
    Cursor   string             // 不透明续页游标（上次响应 next_cursor）
    Expand   *ExpandOptions     // 可选展开（§2.4）
}

// ---- 输出 ----

type ResultRow struct {
    Ref       string            // canonical L0 串（FormatRef 产物）
    Freshness string            // current / material_reuse / stale（物化层透传，不升级）
    Payload   map[string]any    // 该 kind 声明且命中行携带的标量（受 max_fields 限制）
}

type QueryResult struct {
    Rows         []ResultRow
    TotalMatches int    // 谓词总命中数（不受 Limit 截断；>Rows 数即有下一页）
    NextCursor   string // 非空 = 还有结果
    Snapshot     string // 本次查询实际绑定的快照标识（可重放性）
    CostClass    string // "index" | "compile"（降级路径，见 §3.3）
    Degraded     string // 空 = 走索引；非空 = 降级原因（如 payload_index_unavailable）
}
```

### 2.2 Go 函数面（查询引擎对外仅三个入口 + 一个目录）

```go
// Query 执行即席谓词查询。纯只读；不触发任何投影计算（§1 边界 3）。
func (e *Engine) Query(ctx context.Context, q RefQuery) (QueryResult, error)

// Expand 把 ref 列表展开为受字节预算约束的摘要 + CAS 句柄（不做 LLMContext
// 装配——那是 observe 的职责）。forbidden-field 过滤在此同样生效。
func (e *Engine) Expand(ctx context.Context, refs []string, opt ExpandOptions) ([]ExpandedRef, error)

// DiffEvidence 对两个快照（或显式 ref 集）做差分。depth=identity 全程索引；
// depth=content 时委托既有差分承载者（§2.5）。
func (e *Engine) DiffEvidence(ctx context.Context, d DiffRequest) (DiffReport, error)

// Catalog 报告引擎能力面：注册 kind、各 kind 可过滤/可排序的载荷字段、
// 各谓词类的成本分级——工具面把它编译进 ref.query 的 schema description，
// 让模型在发 query 前就知道哪些字段可查（grep 前先有 --help）。
func (e *Engine) Catalog() EngineCatalog
```

`Engine` 构造与依赖注入（函数级，见 §5 物化读口）：

```go
type Engine struct { /* 中央索引（copy-on-write 快照交换）、路由表、物化读口 */ }

func NewEngine(store MaterializedStore, payload PayloadIndexRegistry) *Engine
```

### 2.3 工具面 JSON Schema（对齐 tools/catalog.go CommandSpec 惯例）

**`ref.query`**（新；Category=mix；RiskLevel=RiskDirect；OutputSchema="read-only ref.query.v1 result"）：

```jsonc
{
  "cmd": { "const": "ref_query" },
  "query_id":            { "type": "string" },          // 幂等/审计键（可选）
  "kinds":               { "type": "array", "items": { "type": "string" } },  // ⊆ 目录注册 kind
  "scope_kind":          { "type": "string" },          // 如 "track"
  "scope_values":        { "type": "array", "items": { "type": "string" }, "maxItems": 64 },
  "scope_value_prefix":  { "type": "string" },          // 总线子树
  "time_window": { "oneOf": [
      { "const": "all" },
      { "type": "object", "required": ["start_seconds", "end_seconds"],
        "properties": { "start_seconds": {"type":"number"}, "end_seconds": {"type":"number"},
                        "units": {"enum": ["seconds","samples"], "default": "seconds"} } }
  ]},                                                       // 秒=通用人类层，samples=权威层；D3 双入口
  "snapshot":            { "enum": ["latest"], "default": "latest" },  // v1 只放 latest；回溯段进 v2
  "payload_conditions":  { "type": "array", "maxItems": 8, "items": {
      "type": "object", "required": ["field", "op"],
      "properties": { "field": {"type":"string"}, "op": {"enum":["eq","ne","gt","ge","lt","le","in"]},
                      "value": {"type":"number"}, "str": {"type":"string"}, "set": {"type":"array","items":{"type":"string"}} } } },
  "sort":                { "type": "array", "maxItems": 2, "items": {
      "type": "object", "required": ["field"], "properties": { "field": {"type":"string"}, "dir": {"enum":["asc","desc"]} } } },
  "limit":               { "type": "integer", "minimum": 1, "maximum": 500, "default": 50 },
  "cursor":              { "type": "string" },
  "expand":              { "enum": ["none", "handle", "summary"], "default": "none" },
  "expand_max_bytes":    { "type": "integer", "minimum": 256, "maximum": 32768, "default": 4096 }
}
```

响应（v1）必带五元数据：`cost_class`、`degraded`、`snapshot`、`next_cursor`、`total_matches`——成本分级是 D5 的硬要求，不是装饰字段。

**`ref.diff`**（新；同上惯例）：

```jsonc
{
  "cmd": { "const": "ref_diff" },
  "base":  { "enum": ["latest"] },                       // v1：base=显式快照/observation_id
  "base_revision":  { "type": "string" },                // 二选一必填
  "base_observation_id": { "type": "string" },
  "head":  { "const": "latest" },
  "scope": { /* 可选：与 ref.query 同构的限定谓词（kind/scope/window），缩小差分面 */ },
  "depth": { "enum": ["identity", "content"], "default": "identity" }
}
```

### 2.4 Expand：句柄展开的边界

```go
type ExpandDepth string
const (ExpandHandle ExpandDepth = "handle"; ExpandSummary ExpandDepth = "summary")

type ExpandOptions struct {
    Depth    ExpandDepth // handle=只给 evidence://<sha256> 句柄+字节数+freshness
                          // summary=句柄 + 字节预算内摘要（forbidden-field 过滤后）
    MaxBytes int          // summary 摘要预算，默认 4096，硬上限 32 KiB
}
type ExpandedRef struct {
    Ref       string
    Handle    string // evidence://<sha256>（CAS；物化层 Resolve 产出）
    Bytes     int64
    Freshness string // 透传
    Summary   map[string]any // Depth=summary 时；CCB 同款禁键过滤（freeStateForbiddenField 族）
    Truncated bool
}
```

边界规则：expand **不装配 LLMContext、不走披露预算（MaxDisclosureBytes 是 CCB 的）、不做 digest**——它是 grep 命中后的廉价"看一眼"；语义深读仍走 observe（view 是唯一带审计回执的深读通道，§4）。

### 2.5 DiffEvidence：两级差分

```go
type DiffDepth string
const (DiffIdentity DiffDepth = "identity"; DiffContent DiffDepth = "content")

type DiffRequest struct {
    Base      SnapshotRefSet // 快照标识（revision/observation_id）或显式 ref 列表
    Head      SnapshotRefSet
    Scope     *RefQuery      // 可选：差分前先按谓词缩小两侧集合
    Depth     DiffDepth
}
type SnapshotRefSet struct {
    Revision      string   // 三选一
    ObservationID string
    Refs          []string // 显式集合（模型从 query 结果里拿来对拍）
}

type RefPair struct{ Base, Head string } // 同主键坐标（kind+scope_kind+scope_value+window）

type DiffReport struct {
    Added          []string  // head 有 base 无（canonical L0）
    Removed        []string  // base 有 head 无
    Changed        []RefPair // 坐标相同、hash 不同（内容变了）
    UnchangedCount int
    // Depth=content 时：既有差分承载者的产物引用（非本引擎重算）：
    //   COM change_delta（semantic_compressor_execution.go 的 Before/After paired 链）
    //   observation before_after（applyBeforeAfterDelta，同 tap+render_revision 门）
    //   FXM A/B（fxm_measurement 请求携带的 render 身份）
    Delegated      map[string]any // 承载者 → 其 ref/工件的映射
    CostClass      string         // identity=index；content=compile（需 render 则 probe，响应明示）
}
```

设计立场：**载荷级差分一律委托既有承载者，查询引擎不新建差分算法**。identity 级（集合差）是引擎自己的毫秒级职责；content 级是"找到对的差分证据并给出句柄"，COM/FXM/before_after 已有锚定校验（同 tap+render_revision+精确采样窗），重复实现只会造第二套真相。

> **IMPL-D 落地修订（2026-10-06，L1-3-IMPL-D 卡）**：
>
> 1. **分层裁定——content 委托接线落 harness 工具层**（`agent/internal/harness/ref_diff.go`），引擎 `DiffEvidence` 保持 identity-only。理由：三承载者的句柄面是 bootstrap/store 侧知识（观察票内部 JSON 键 `com_projection`/`fxm_projection`/`mix_package.current_metrics.*`、`com_evidence` 工件目录），引擎 `MaterializedStore` 契约只暴露 `Resolve`（handle+ReadAll）——在引擎内解析承载者内部键会复制 bootstrap 解析知识、违反 D2 存储引擎分界。IMPL-C 注记①的引擎面预授权（DiffRequest 扩字段）**未动用**；`DiffRequest`/`DiffReport` 签名零改动，`Delegated` 字段在引擎面留空（工具面响应直接承载委托映射）。
> 2. **承载者坐标路由与响应形态**：kind `com`→`com.change_delta`（票内 com_projection，changed 对双票句柄）；kind `dad.compressor_dual_tap`→`com.paired_artifact`（pair 工件路径句柄）；kind `fxm`→`fxm.ab`（票内 fxm_projection）；`observation.before_after` 为 kind 正交承载者（差分涉及票内 `before_after_delta`+`ab_result` 锚定键提取，含 `matches_base` 对齐提示）。无载者 kind 进 `unrouted_kinds` 如实报空。成本分级沿用 R10（`RouteDiff`）：content 恒 compile、`degraded` 恒空（委托是设计路径非降级；需 render 的 probe 级不在 v0 只读面）。
> 3. **fxm/com 实例身份注记（T7 口径落地）**：fxm 与 com 行 hash 均为实例身份（时间戳入哈希，materialize `instanceIdentityHash`），identity 级 Changed ≠ 内容变化——两深度响应的 changed 条目均带 `hash_semantics: instance_identity` 注记；dom/acp/dad 系=内容身份 hash，不注记。设计原文只点名 fxm，落地按 materialize 现实把 com 一并注记。
> 4. **承载者现实锚定修正**：(a) "同 tap+render_revision 门"实际位于 **ab_result**（`mom_ab_result.v1` 的 `quality_gates`：same_tap_point/render_revision_changed，mixboard.go abResultQualityGate）；`before_after_delta` 本身仅门"前观察存在"。(b) FXM 锚定门是 same_source/same_window/same_format（A/B 双侧本就异 render，每侧 RenderRevision 记录于 MeasurementRef），与 COM 口径不同。(c) §3.3 路由表 fxm 建议字段 `fxm.ab.delta_lufs`/`fxm.ab.correlation` 与实际 JSON 键不符（`effect_delta.lufs_delta_db`；correlation 差分在 ab_result 的 stereo 块、fxm EffectDelta 无此字段）——目录字段名修正归 Catalog 侧后续卡，本卡不改共享路由表。
> 5. **T7 测试形态**：引擎级四类精确命中用 explicit refs（bootstrap 单盘态下 base exact 视图≡latest 同次扫描子集，removed 结构性恒空——removed 类只能在显式 ref 集路径命中，此语义已在测试中锁定注记）。

### 2.6 observe 参数化升级（衔接参数规格，实现落 CCB 卡）

对照 CCB-VIEW 7 缺口，observe 侧只做**最小参数化**（view 的价值在预编排，不复制 query 的全谓词面）：

| 缺口 | observe 侧 v1 增量（`ccb.observation_request` schema 追加） | 归 query 侧的（不在 observe 做） |
|---|---|---|
| 1 无时间窗 | `time_window`（可选，默认全窗；同 §2.3 双标尺） | 全参数化窗口谓词（相交/包含/覆盖） |
| 2 无多目标 | `targets: [{kind,id,label}×≤8]`（track.\* view 批量；mix.\* 保持 project 域硬定，不缩窄——现有护栏保留） | 轨道谓词/排除集/总线子树 |
| 3 粒度两粗旋钮 | 保持（max_items/max_disclosure_bytes 不动） | 粒度档由物化快照承载（§7 OQ-3） |
| 4 无 topK | `top_k: {field, dir, k≤24}`（每 view 一组；字段 ⊆ 该 view 声明的可排序字段） | 任意字段 topK+分页 |
| 5 freshness 无枚举 | `freshness_class` 补 enum：`current_observation`/`post_action`/`material_reuse` | snapshot 谓词全套 |
| 6 目录无参数面 | `ccb.observation_catalog` 加 `dimension` 过滤参数——**接线 `GetViewsForDimension`**（F7，全库现存唯一未接线的目录谓词钩子） | 引擎能力目录（Engine.Catalog，另一条线） |
| 7 两套键命名法 | 不动（view 键是 CCB 内部概念；ref 统一由 G1 迁移卡处理） | — |

响应侧追加：`actual_cost_class`（index/compile）、`recomputed`（物化 miss 而触发的 finalize 计数）——"observe 便宜"的预设被现实现状打破时，模型必须看得见（诚实边界）。

---

## 3. 索引层接口契约与路由裁决表（验收②）

### 3.1 两类索引的分工（D2 的落地形态）

- **中央段索引（引擎自有，必建）**：只存 ref 五段 + freshness + 投影声明的标量载荷值。数据全部来自物化读口（§5），自身无存储引擎争议（v1 内存 copy-on-write，整表不可变快照交换——p99 目标下避免读写锁竞争；落盘形态属 L1-2）。FTS5 同构参照：段索引 ≈ FTS5 的索引列，标量载荷 ≈ contentless 表的外挂列。
- **投影局部载荷索引（契约实现，可选）**：某 kind 需要字段级过滤/排序/topK 而中央标量不够时，由该投影自建（倒排/位图/立方体随其便），实现 `PayloadIndex` 契约。**引擎不规定存储引擎，只规定吃 ref 主键**（D2 原文）。

```go
// PayloadIndex 是投影局部索引的契约（D2：统一寻址不统一存储引擎）。
type PayloadIndex interface {
    // Kind 报告该索引服务的注册 kind（一对一；一 kind 一索引）。
    Kind() string
    // Fields 声明可过滤/可排序字段及支持的算子（进 Engine.Catalog）。
    Fields() []PayloadFieldSpec
    // Search 在该 kind 内执行段谓词（引擎已按 kind 路由到此）+ 载荷谓词，
    // 返回 canonical L0 ref 列表（含排序与 limit；排序字段必须 ∈ Fields 且 Sortable）。
    Search(ctx context.Context, seg SegmentConstraints, pay []PayloadCondition,
        sort []SortKey, limit int, cursor string) (rows []ResultRow, next string, err error)
    // Upsert/Delete 由物化层变更流驱动（§5），非查询路径。
    ApplyChange(ch MaterializedChange) error
}

type PayloadFieldSpec struct {
    Field    string       // "mom.masking.candidates.median_margin_db"（kind.domain.path 三段命名）
    Type     string       // "number" | "string" | "bool"
    Ops      []PayloadOp  // 该字段支持的算子
    Sortable bool
}

// SegmentConstraints 是引擎下发给局部索引的段谓词（与 RefQuery 同构的字段，
// 单 kind 视角；scope/window/snapshot/hash 四段 + 载荷外的排序键）。
type SegmentConstraints struct {
    Scope    *ScopePredicate
    Window   *WindowPredicate
    Snapshot SnapshotPredicate
    Hash     *HashPredicate
}
```

### 3.2 路由裁决表（谓词类 → 索引层）

路由是**表驱动数据**（`route.go` 内一张静态表 + 单元测试锁定），不是散落的 if 链：

| 谓词类 | 例 | 路由目标 | 成本 | 依据 |
|---|---|---|---|---|
| R1 仅 kind | `kinds=[mom]` | 中央段索引 | index | 五段全在中央 |
| R2 kind+scope | `mom + scope(track, [T3,T7])` | 中央 | index | 同上 |
| R3 kind+scope+window | `dom + track:T3 + 窗相交 [10s,15s)` | 中央 | index | 同上（窗为采样点区间比较） |
| R4 snapshot/hash 谓词 | `@latest` / `#sha256:ab12…` | 中央（hash 亦可 CAS 直查验证） | index | snapshot 先行解析为快照视图再查 |
| R5 段谓词 + 载荷过滤 | `mom + median_margin_db < 6` | **有局部索引** → 该 kind PayloadIndex；**无局部索引** → 中央出候选集 → 逐条 Resolve 读值过滤（降级） | index / **compile（降级，响应 Degraded 非空）** | D2：载荷字段归投影 |
| R6 载荷排序/topK | `sort=loudness desc, limit=10` | 同 R5（Sortable 字段必须已声明；未声明 → ErrUnsupportedSort 拒绝，不静默乱序） | index / compile（降级） | 同上 |
| R7 多 kind 混合载荷 | `kinds=[mom,rlm] + 载荷字段` | 按 kind 拆分 → 各自路由（R5）→ 引擎归并排序 | index / compile | 载荷字段带 kind 前缀，天然可拆 |
| R8 expand | refs → 句柄/摘要 | 物化层 Resolve（§5） | index（句柄）/ compile（大摘要） | 内容归物化层 |
| R9 diff identity | 两快照集合差 | 中央（两快照视图各查一次 + 坐标对齐） | index | 五段可比 |
| R10 diff content | 载荷差异 | 委托既有差分承载者（§2.5） | compile / probe | 不重算已有真相 |

降级路径的存在是诚实设计：v1 大多数 kind 没有局部索引，R5/R6 降级是常态而非异常——所以降级必须在响应里可见（`degraded` + `cost_class=compile`），模型知道这次"grep"花了小钱。

### 3.3 九投影 + DAD 逐层裁决（完整表）

对每个 kind 给出：v1 kind 名（注册现状）、索引种子（喂中央索引的数据源，对应 L1-2 触发链）、局部载荷索引 v1 范围（建议字段，实现卡可增删但须进 Catalog）、特记。

| kind（注册状态） | 中央段索引种子（数据从哪来） | 局部载荷索引 v1 建议 | 特记 |
|---|---|---|---|
| **dom**（已注册） | 观察落盘 obs JSON 的 DOMProjection（`dom_projection.go:11` finalize 链产物） | 每 dimension 的 readiness/trust_quality 9 字段（CCB-VIEW §1 view 6-10 同源）：`dom.<dim>.readiness`、`dom.<dim>.trust.confidence` | 内容 ID=全量 JSON 哈希（时间戳置空）——hash 段语义最正的一个 |
| **mom**（**待注册**，D 依赖①） | obs JSON 的 MOMProjection | `mom.masking.candidates.median_margin_db`、`mom.frequency.conflict.band`、`mom.static.level_rank`——masking candidates ≤12 与 multitrack ≤12×4 的硬编码截断（CCB-VIEW §3.4）正是 topK 参数化要解锁的 | 无自生成 ID：kind 注册卡须同时定 mom 的 snapshot 段承载（observation_id） |
| **tim**（待注册①） | obs JSON + 导入回执双路径产物（`message_loop.go:8535`） | `tim.issues.count`、`tim.coverage.ready_ratio` | 同上，snapshot=observation_id |
| **tom**（待注册①） | 导入回执 + capability 上下文产物 | `tom.track.duration_seconds`、`tom.track.channel_count` | 轨道组织是 query 面高频起点（"哪些轨存在"），建议 v1 优先接 |
| **fxm**（已注册） | obs JSON 的 FXMProjection（测量随请求携带，`mixboard.go:3968`） | `fxm.ab.delta_lufs`、`fxm.ab.correlation` | hash 含时间戳（实例身份）——**与 DOM 内容身份语义不同**，diff 的 Changed 判定按 kind 注记 |
| **com**（已注册） | obs JSON + 执行票据 change_delta（`semantic_compressor_execution.go:659`） | `com.source.gr_crest`、`com.change_delta.peak_delta_db` | paired 工件在 `Artifacts/com_evidence/`（E5），expand 句柄天然指向 |
| **epm**（待注册①） | 导入回执产物（`message_loop.go:8578`） | `epm.section.count`、`epm.marker.position` | 段落坐标与 L2-2 段落投影（D10）未来合流，v1 只登记不深建 |
| **rlm**（已注册） | capability preflight 产物（`gain_staging.go:80`） | `rlm.summary.integrated_lufs`、`rlm.summary.true_peak_db`、`rlm.row.target_delta_db` | 无 LLMContext（manifest §5 缺口）不影响索引面；E8 三源合并无 revision 比较——**stale 风险按现状透传，修复归 G2** |
| **acousticpackage**（**kind 名待裁定**①：建议 `acp`；legacy 前缀 `acoustic_package_status:`） | 落盘 `acoustic_package_status.json`（M4 store：Upsert 交叉失效已实现，物化层最成熟种子） | `acp.band.energy_db`、`acp.loudness.lufs`、`acp.stereo.correlation` | revision 指纹（M4 `rev_hex8`）直接映射 ref 的 snapshot 段 |
| **DAD：audioclosure**（待注册①：`dad.l3` / `dad.l2_render_probe` / `dad.compressor_dual_tap` 三 kind，charset `[a-z0-9._]` 容忍点号） | L3 特征族落盘行 + probe 帧 + paired 工件（事件目录 #4-#9） | `dad.l3.band_energy.summary_db`、`dad.l2_probe.bands.peak_db` | 内核 C++ 生成点（D 类 5 处）格式迁移已裁独立小卡排 L0 后——**迁移前以 legacy 翻译条目进中央索引**（三态解析的 legacy 态），迁移后升 parsed |
| **DAD：frequencycleanup**（待注册①：`dad.frequency_evidence`） | WorkflowData/orchestration 产物（read-first 库，`model.go:16/46`） | `fci.issue.severity` | C1 能力域专用，v1 可缓接（频段清理会话内已有专用装配） |

**依赖①（注册表扩展前置卡）**：mom/tim/tom/epm/acousticpackage/DAD 系 kind 注册 + 各自 snapshot 段承载定义，是查询引擎能覆盖"九投影+DAD"的硬前置。该卡属 agentprotocol 常量表追加（零行为改动，与 refschema-L0-1 同型），**建议排在 L1-3 实现卡之前**。

### 3.4 一致性与并发

- 单次 query 绑定单一快照视图（read-committed-per-snapshot）：查询开始时解析 snapshot 谓词 → 拿到快照代 ID → 全程对该代不可变视图求值——结果自带 `snapshot` 字段，可重放（CAS 语义的查询版）。
- 中央索引更新：物化变更流到达 → 构建新不可变整表 → 原子交换指针（微秒级），读侧永远无锁等待。写放大在 v1 规模（≤10⁵ ref）可接受；增量化属 L1-2 物化层。
- 引擎不缓存跨快照推导结果（除 diff 显式请求），避免第二层失效问题。

---

## 4. CCB view 与即席 query 的边界（验收③）

### 4.1 一句话边界

**view 是"预编排披露"（CCB 编排好、裁剪好、预算执法、审计回执、进对话上下文）；query 是"即席谓词"（模型自己拼 grep、回 refs 不装配、不进披露预算）**。两者共用 ref 寻址与物化层，但输出形态、预算体系、审计要求完全不同——不合并、不互相伪装。

| 维度 | observe（view 披露） | query（即席谓词） |
|---|---|---|
| 输入 | view_ids（目录枚举的有限集）+ 最小尺度参数 | 任意谓词组合（L0 五段 + 载荷） |
| 输出 | 装配 bundle + LLMContext digest + limitations | refs + 声明标量 + 句柄 |
| 体积治理 | MaxDisclosureBytes（披露预算，超则 ready→partial→insufficient） | 行数（limit≤500）+ expand 字节预算——**不走披露预算** |
| 裁剪 | 两层（capabilitycontext 白名单 + contextruntime 分层预算） | 禁键过滤（freeStateForbiddenField 族）但无 digest 化 |
| 审计 | FreeStateObservationAuditReceipt（模型请求=服务执行证明） | query_id + 快照标识进日志；**无审计回执**（只读检索，不构成观察声明） |
| 新鲜度 | bundle.Freshness 三件套 + view 级 ready/missing/stale/partial | 行级 freshness 原样透传 |
| 成本 | index~compile（物化 miss 触发现算，响应明示 recomputed） | index（降级路径 compile，响应明示 degraded） |
| 红线 | 不暴露 raw PCM/DAD 包/mutation 权限 | 同左（expand summary 同款禁键过滤；永远不回 raw package） |

### 4.2 互操作（三层单向流）

```
query（grep 粗定位：拿 track ids / 候选 ref 集）
  → observe（定点深读：target_id(s) + view_ids 披露装配）
      → 执行/变更（VSP/PCA，本文档范围外）
          → diff.evidence（变更前后差分回看）
```

1. **query 喂 observe**：query 结果的 scope_value（轨道 ID 等）直接作 observe 的 target_id/ targets——这是两层的胶水，也是 CCB-VIEW 缺口 2（多目标）必须在 observe 侧开 `targets` 参数的原因（100 轨工程里"先 grep 出 3 条候选轨再逐个深读"是主路径）。
2. **view 目录可被 query 发现，但 view 不是查询对象**：view 是 CCB 概念（编排单位），ref 是查询概念（地址单位）；`ccb.observation_catalog` 加 dimension 参数（接线 GetViewsForDimension，F7）让"该问哪个 view"可查，但 ref.query 的谓词里不出现 view_id——防止两层语义纠缠。
3. **同源不同形**：同一物化事实既可被 view 披露（digest 形态）也可被 query 命中（ref+标量形态），freshness 语义两侧一致（同一物化读口，§5）——不存在"view 说是 stale 而 query 说是 ready"的分叉。

### 4.3 参数归属总表（7 缺口 × 两层）

| 尺度参数 | observe 侧 | query 侧 |
|---|---|---|
| 时间窗 | 可选 `time_window`（单窗，默认全窗） | 全谓词（intersects/contains/within + 双标尺） |
| 范围 | `targets ≤8`（track view 批量）；mix view 保持 project 域 | scope 精确集/前缀/总线子树/排除 |
| 粒度 | 不参数化（view 内部事） | 不参数化（粒度属内容，§7 OQ-3） |
| topK | 每 view 一组 `top_k{field,dir,k≤24}` | 任意声明字段 + 排序 + 分页 |
| 目录过滤 | `dimension` 参数（GetViewsForDimension 接线） | Engine.Catalog（能力自描述） |

---

## 5. 物化层读侧接口（验收④：函数级签名，G2 边界）

**裁定落地**：查询引擎**只读物化层**，不直读投影内存态；**失效与新鲜度归物化层管**，引擎收到什么状态就透传什么状态。存储形态（内存/文件/CAS/数据库）未决**不影响**本契约——契约只约定三件事：全量喂给、增量告知、内容解析。

```go
// MaterializedStore 是查询引擎对物化层（L1-2 产出）的全部依赖面。
// 实现方：v0 = 引擎内置 bootstrap 适配器（§5.2）；v1 = L1-2 物化层本体。
type MaterializedStore interface {
    // SnapshotView 返回某快照代下的全部 ref 行（段字段 + freshness + 声明标量）。
    // 引擎用它构建/重建中央段索引。latest 模式由物化层解析为当前已提交代。
    SnapshotView(ctx context.Context, sel SnapshotSelector) ([]MaterializedRow, error)

    // Resolve 把单个 ref 解析为内容句柄（CAS evidence://…）+ 状态。
    // 内容不进引擎——expand 只拿句柄与字节预算内摘要。
    Resolve(ctx context.Context, ref agentprotocol.Ref) (ResolvedEvidence, error)

    // Subscribe 提供增量变更流（新增/替换/失效标脏），引擎据此交换中央索引代。
    // 失效正确性（漏标=静默错误数据）归物化层与 G2 专项——引擎只消费事件，不推断。
    Subscribe(ctx context.Context) (<-chan MaterializedChange, func(), error)
}

type SnapshotSelector struct {
    Mode          SnapshotMode // latest / exact / at_or_before（§2.1）
    Revision      string
    ObservationID string
}

type MaterializedRow struct {
    Ref       agentprotocol.Ref
    Freshness string            // current / material_reuse / stale（F5 铃三分，M1 语义）
    Payload   map[string]any    // 投影声明并提供的标量（PayloadFieldSpec 对齐）
}

type MaterializedChange struct {
    Op   MaterializedOp // added / replaced / marked_stale
    Row  MaterializedRow
    // marked_stale 只改状态不删行——审计与"曾经存在"的可见性保留（对齐 M2 台账语义）
}

type ResolvedEvidence struct {
    Handle    string // evidence://<sha256>（projectstore CAS，F5）或工件路径句柄
    Bytes     int64
    Freshness string
    // 摘要提取由引擎 Expand 做（禁键过滤在引擎侧），物化层只给内容访问
    ReadAll   func() ([]byte, error) // 受 CAS pin 保护；大对象实现方可给 Reader 形态
}
```

### 5.1 契约要点（写进双方实现卡的验收条）

1. **stale 不删行**：物化层发 `marked_stale`，引擎保留该 ref 并透传状态；谓词默认不过滤 freshness（显式 `freshness` 谓词才过滤）——"曾经观察到"与"当前可信"是两个查询语义。
2. **snapshot 解析归物化层**：latest/exact/at_or_before 的代际管理是物化层（L1-2 D4）的职责，引擎不做 revision 比较逻辑。
3. **CAS 优先**：Resolve 返回的句柄优先 `evidence://`（pin 不可逐，天然去重，F5/M7）；未 CAS 化的旧产物给工件路径句柄并在 freshness 注记（对应 ref hash 段 `#-` 的显式语义）。
4. **单向依赖**：queryengine 包 import projectstore/agentprotocol 可以，反向禁止；物化层不知道引擎存在（它只发变更流）。

### 5.2 v0 bootstrap 适配器（不等 L1-2 的起步腿）

L1-2 未开工期间，引擎按只读扫描既有落盘产物构建 SnapshotView（不写不改）：

| 产物族 | 扫描对象 | 对应种子（L1-2 M 编号） |
|---|---|---|
| 观察票 JSON | `.vit_agent/<uuid>/observations/<obs_id>.json`（readObservationByID 同源，`mixboard.go:3721`） | M7 |
| 声学包 | `acoustic_package_status.json` store（Build/Merge 语义只读） | M4 |
| COM paired 工件 | `Artifacts/com_evidence/<pairID>/*.json` | E5 |
| 特征快照行 | 桥快照文件（F5 铃标注行） | M1 |

增量腿：v0 用"文件 mtime + 观察票目录列举"轮询模拟 Subscribe（秒级粒度，仅够冒测）；v1 换 L1-2 事件驱动变更流（事件源候选=L1-2 报告 §3 的 14 项真候选）。**v0 不触发任何 finalize/现算**——扫描不到就是 empty+deferred，如实报告。

### 5.3 与 G2 的接口面分工（失效正确性归属）

- G2 专项测物化层："多级依赖链中途变更，断言下游全部重算"（E8/E16 两个既有隐患是红测素材）。
- 本引擎只测"收到 marked_stale 后不删行、不升级、不静默"（§6 正确性 T6）——引擎侧无失效逻辑，故无可漏标。

---

## 6. 测试计划（验收⑤：延迟 + 正确性两面）

全部落在 `agent/internal/queryengine/` 测试与 `scripts/` 基准脚本（复用既有 ps1 体系入口，AGENTS §5），分五组：

### 6.1 延迟基准（毫秒级目标怎么测）

**基准数据生成器**（合成，种子固定，可重放）：

- 规模锚三档：6 轨（自由态受控工程量级）/ 60 轨 / 100 轨（混音能力层测试工程量级——对齐论文两层验证规模的诚实边界）；
- 每轨特征族 ~12 kind×窗组合（L3 特征族 + probe + paired），快照数 5~10 代 → 100 轨档约 10⁴~10⁵ ref 行；
- 双形态跑同一数据：**内存中央索引**（v1 主形态）与**磁盘物化读**（bootstrap 扫描 + Resolve 路径）分开计时。

**场景矩阵**（每场景 bench 函数 + p50/p99 采样上报 `b.ReportMetric`）：

| # | 场景 | 对应路由 |
|---|---|---|
| B1 | kind 过滤（单 kind，10 万行内） | R1 |
| B2 | kind+scope 集合（64 轨） | R2 |
| B3 | kind+scope+window 相交 | R3 |
| B4 | 载荷过滤+排序 topK=10（有局部索引 / 降级两形态） | R5/R6 |
| B5 | 大结果集分页翻三页 | R2+cursor |
| B6 | diff identity（两快照各 10 万行） | R9 |
| B7 | expand handle ×100 | R8 |

**断言形态与纪律**：

- 设计建议值（先行给出，供实现卡基线对照）：内存段谓词 B1-B3 p99 ≤ 20ms；降级 B4 p99 ≤ 500ms（compile 级声明）；B6 p99 ≤ 100ms；磁盘形态各 ×5 容忍。
- **先基线后冻结**（AGENTS §8）：实现卡第一步在基准数据上跑基线，把建议值换算成"实测 × 安全系数"写死进断言与 CURRENT-STATE——禁止看到跑出来的数字再倒推阈值。
- 延迟断言默认 `-bench`/环境开关启用，CI 常规门只跑正确性——防 flaky（§11 不稳定测试纪律前置）。
- 跑基准的机器指纹（CPU/Go 版本）随报告记录；跨机对比无效。

### 6.2 正确性对拍（谓词结果 vs 全量过滤）

**Oracle**：`oracleFilter(rows []MaterializedRow, q RefQuery) []string` ——与引擎完全独立的朴素线性扫描实现（不共享谓词求值代码路径），这是"全量过滤对拍"的字面落地。

| # | 测试 | 断言 |
|---|---|---|
| T1 | 表驱动：每谓词类（R1-R6）≥3 例——空集/单命中/全命中/边界（window 半开区间端点、scope 前缀与精确并存的优先级） | 引擎结果 ≡ oracle（含顺序） |
| T2 | Fuzz：`go test -fuzz` 随机谓词组合（固定 corpus seed 起步），合成行集 | 同上；fuzz 发现分歧即 bug 不是"改 oracle" |
| T3 | 真实 fixture 双轨：experiments/ 合成工程产物（只读复制到 t.TempDir()，§10 防污染）+ 合成集 | 引擎在真实产物上不 panic、行数可解释 |
| T4 | L0 依存：未注册 kind → ErrUnknownKind（**非空结果**）；legacy ref（dom_ 前缀翻译条目）可查；opaque ref 不进索引但 Expand 可透传 | 错误语义锁定 |
| T5 | 快照一致性：单 query 全程单代视图（查询中并发 ApplyChange 不影响已开始的结果集） | 结果 snapshot 字段可重放 |
| T6 | freshness 透传：stale 行在默认谓词下保留且状态原样；显式 freshness 谓词才过滤；无任何路径把 stale 升级为 current | F8/M1 纪律 |
| T7 | diff identity 正确性：构造已知差分（added/removed/changed/unchanged） fixture | DiffReport 逐类精确命中；fxm 实例身份（时间戳入哈希）按 kind 注记不误报 Changed |

### 6.3 路由与目录测试

- T8 路由表驱动：每谓词类 → 期望路由目标（含降级触发条件）与期望 cost_class/degraded 标注；
- T9 Catalog 一致性：PayloadIndexRegistry 声明的字段与 Engine.Catalog 输出一致；ref.query schema description 与 Catalog 同源生成（不双写漂移）。

### 6.4 工具面测试

- T10 schema 校验：ref.query/ref.diff 的 InputSchema 拒绝未知字段/越界 limit/空谓词全空集（至少一段谓词必填，防"全库拉取"误操作）；
- T11 成本标注回归：降级路径响应必带 degraded+compile；observe 响应带 actual_cost_class+recomputed（CCB 参数化卡的联合断言）。

### 6.5 端侧烟测（按 AGENTS §5 门槛，实现卡收口）

单测/基准全绿 ≠ 交付。实现卡须过真实栈烟测：`dev_agent_smoke.ps1` 体系加只读场景（拉起三件套 → 开既有测试工程 → 经 `/agent/invoke` 跑 ref.query 全场景 → 断言退出码 0）；只读白名单模式参照 `scripts/g_runtime_readonly_smoke.ps1`。本设计零代码改动，不适用烟测门槛；登记给实现卡。

---

## 7. 开放问题与依赖清单（停止条件对照）

停止条件未触发（无"未定版到影响接口"的前置——物化存储形态被 §5 三函数契约隔离在实现侧）。但以下四项登记上交：

| # | 开放问题/依赖 | 建议裁决 | 影响 |
|---|---|---|---|
| OQ-1 | **注册表扩展前置卡**：mom/tim/tom/epm/acp/DAD 系 kind 注册 + snapshot 段承载定义（现仅 dom/fxm/com/rlm） | 独立小卡，排 L1-3 实现卡前（agentprotocol 常量表追加，零行为改动，refschema-L0-1 同型） | 不做则 query 只能查 4 kind，"九投影+DAD"不成立 |
| OQ-2 | **granularity 不在 L0 五段内**：同坐标+同快照的多粒度档（帧/窗/段落/全曲）无法用 ref 区分 | v1 采纳"粒度属内容不属地址"：每坐标+快照一个默认物化档，粒度信息走 payload 字段（如 time_scale_coverage）；若未来需并存多档，扩 scope_kind 或 kind 变体——**G1 schema 演化裁定点，本设计不私改文法** | 影响 diff 的 Changed 判定（多档并存会误报），v1 无此问题 |
| OQ-3 | v0 bootstrap 轮询的换代时机（mtime 秒级粒度）是否够冒测用 | 够（冒测只验正确性不验实时性）；实时性归 L1-2 事件驱动 | 无阻塞 |
| OQ-4 | memory 检索共用设施（D5"与 memory 检索共用同一套设施"、D2 对象类型轴预留 memory 条目） | kind 注册表天然开放扩展（memory 条目作新 kind 注册即可），谓词/索引/工具面零改动；v1 不实现，仅确认接口不排他 | 无阻塞 |

前置依赖（排程序）：G1 L0 文法 ✅ 已定版有实现 → **OQ-1 注册表扩展卡** → L1-3 实现卡（引擎+工具面）∥ CCB 参数化卡（observe 侧）→ L1-2 物化层替换 v0 bootstrap → G2 失效专项。

---

## 8. 与既有纪律的对照自检

| 纪律 | 本设计的遵守方式 |
|---|---|
| 投影是 peer 不是链（AGENTS §5） | 路由表按 kind 平行分发，无任何"X 索引依赖 Y 索引输出"的层间链 |
| CCB 不做自动 view 选择 | query 不选 view、不代发 observe；GetViewsForDimension 接线是**目录过滤参数**不是自动推荐 |
| 缺失/partial/stale 原样保留 | T6 锁定；stale 不删行不升级（§5.1.1） |
| 不展开 raw package | expand 只给句柄+禁键过滤后摘要；raw 字段族在引擎侧同款过滤（§2.4） |
| 测试不写源码树/不碰权威 fixture | T3 只读复制到 t.TempDir()（§10） |
| LLM 运行纪律（§8） | 工具面成本分级是观察预算的第一天实现：每次响应必带 cost_class |
| EvidenceRefs 消费面零改动 | 查询引擎是纯新增读端；refschema 迁移按 G1 ruling 后续卡走，本设计不接线生成点 |

---

## 9. 实现排程建议（决策侧裁剪用）

| 卡 | 内容 | 依赖 |
|---|---|---|
| L1-3-IMPL-A | `queryengine` 包骨架：谓词模型 + 中央段索引 + 路由表 + oracle 对拍测试（T1/T2/T4-T8） | OQ-1 |
| L1-3-IMPL-B | v0 bootstrap 适配器（四产物族只读扫描 + 轮询 Subscribe）+ MaterializedStore 契约测试 | A |
| L1-3-IMPL-C | 工具面：ref.query/ref.diff CommandSpec + harness 分发 + T10/T11 + 只读烟测场景 | A、B |
| L1-3-IMPL-D | DiffEvidence content 级委托接线（COM change_delta/before_after/FXM 句柄映射）+ T7 | C |
| L1-3-BENCH | 延迟基准套件（B1-B7 + 基线→冻结流程） | A、B |
| CCB-PARAM | observe 参数化（§2.6 表）+ GetViewsForDimension 接线 + freshness enum | 独立可并行 |

每卡验收线已在 §6 对应编号；端侧烟测门槛统一在 IMPL-C/D 收口。

---

*本文档由 L1-3-DESIGN-1 卡（PC 执行侧，GLM-5.3/L2）产出；规格锚 G1 ruling L0 文法，事实锚三份勘察报告（CCB-VIEW/L1-2/L1-1）与 OBSERVATION_PROJECTION_MANIFEST。零代码改动；行号以领取时 HEAD 2397a94e 为准。*
