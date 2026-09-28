package queryengine

// route.go — §3.2/§3.3 路由裁决表：表驱动数据，不是散落的 if 链。
//
// 两张表：
//  1. kindRoutingTable —— 九投影 + DAD 逐 kind 裁决（13 条 = 9 投影 kind +
//     4 DAD kind），T8 锁定其与 agentprotocol.RegisteredRefKinds() 逐 kind 相等；
//  2. RouteQuery/RouteExpand/RouteDiff —— 谓词类（R1-R10）→ 路由目标/成本分级
//     的纯函数裁决，与 Engine 执行路径同源。

import (
	"fmt"
	"sort"
	"sync"

	"vit-daw-agent/internal/agentprotocol"
)

// ---- kind 路由表 ----

// KindRouteEntry 单 kind 的中央段索引种子 + 局部载荷索引 v1 建议字段 + 特记。
// 建议字段是目录性数据（实现卡可增删但须进 Catalog）；行为面（过滤求值）直接
// 吃行载荷值，不受建议清单限制——排序受声明约束（R6：未声明即 ErrUnsupportedSort）。
type KindRouteEntry struct {
	Kind          string
	LegacyPrefix  string             // legacy 翻译族前缀（mom/tim/tom/epm 无）
	CentralSeed   string             // §3.3 中央段索引种子（数据从哪来）
	PayloadFields []PayloadFieldSpec // §3.3 局部载荷索引 v1 建议
	Notes         []string           // §3.3 特记（含 hash 语义/diff 判定注记）
}

func numField(field string) PayloadFieldSpec {
	return PayloadFieldSpec{
		Field: field, Type: "number",
		Ops:      []PayloadOp{PayloadEq, PayloadNe, PayloadGt, PayloadGe, PayloadLt, PayloadLe},
		Sortable: true,
	}
}

func strField(field string) PayloadFieldSpec {
	return PayloadFieldSpec{
		Field: field, Type: "string",
		Ops:      []PayloadOp{PayloadEq, PayloadNe, PayloadIn},
		Sortable: true,
	}
}

// kindRoutingTable — 权威 = docs/QUERY_ENGINE_V1_DESIGN.md §3.3（九投影+DAD
// 逐层裁决表）+ 代码现实核对：dom 维度名取自 dom/projection.go 的 7 个
// DimensionReadiness（5 source + 2 processor）；dom.<dim>.trust.confidence
// 为设计模板字段，现行 dom 代码 trust_quality 是投影级布尔族（无 per-dim
// 数值），故目录先落具体 readiness，confidence 留 kind 侧 PayloadIndex 声明。
var kindRoutingTable = []KindRouteEntry{
	{
		Kind:         "dom",
		LegacyPrefix: "dom_",
		CentralSeed:  "观察落盘 obs JSON 的 DOMProjection（dom_projection.go finalize 链产物）",
		PayloadFields: []PayloadFieldSpec{
			strField("dom.peak_structure.readiness"),
			strField("dom.activity_structure.readiness"),
			strField("dom.frequency_time_events.readiness"),
			strField("dom.transient_structure.readiness"),
			strField("dom.band_dynamics.readiness"),
			strField("dom.processor_behavior.readiness"),
			strField("dom.behavior_change.readiness"),
		},
		Notes: []string{
			"内容 ID=全量 JSON 哈希（时间戳置空）——hash 段语义最正的一个",
			"设计另建议 dom.<dim>.trust.confidence（CCB-VIEW §1 view 6-10 同源）；现行 trust_quality 为投影级布尔族，per-dim confidence 待 kind 侧声明",
		},
	},
	{
		Kind:        "mom",
		CentralSeed: "obs JSON 的 MOMProjection",
		PayloadFields: []PayloadFieldSpec{
			numField("mom.masking.candidates.median_margin_db"),
			strField("mom.frequency.conflict.band"),
			numField("mom.static.level_rank"),
		},
		Notes: []string{
			"masking candidates ≤12 与 multitrack ≤12×4 的硬编码截断正是 topK 参数化要解锁的",
			"无自生成 ID：snapshot 段承载=observation_id（L1-3 §3.3）",
		},
	},
	{
		Kind:        "tim",
		CentralSeed: "obs JSON + 导入回执双路径产物（message_loop.go:8535）",
		PayloadFields: []PayloadFieldSpec{
			numField("tim.issues.count"),
			numField("tim.coverage.ready_ratio"),
		},
		Notes: []string{"无自生成 ID：snapshot 段承载=observation_id"},
	},
	{
		Kind:        "tom",
		CentralSeed: "导入回执 + capability 上下文产物",
		PayloadFields: []PayloadFieldSpec{
			numField("tom.track.duration_seconds"),
			numField("tom.track.channel_count"),
		},
		Notes: []string{"轨道组织是 query 面高频起点（\"哪些轨存在\"），建议 v1 优先接"},
	},
	{
		Kind:         "fxm",
		LegacyPrefix: "fxm_",
		CentralSeed:  "obs JSON 的 FXMProjection（测量随请求携带，mixboard.go:3968）",
		PayloadFields: []PayloadFieldSpec{
			numField("fxm.ab.delta_lufs"),
			numField("fxm.ab.correlation"),
		},
		Notes: []string{"hash 含时间戳（实例身份）——与 DOM 内容身份语义不同，diff 的 Changed 判定按 kind 注记"},
	},
	{
		Kind:         "com",
		LegacyPrefix: "com_",
		CentralSeed:  "obs JSON + 执行票据 change_delta（semantic_compressor_execution.go:659）",
		PayloadFields: []PayloadFieldSpec{
			numField("com.source.gr_crest"),
			numField("com.change_delta.peak_delta_db"),
		},
		Notes: []string{"paired 工件在 Artifacts/com_evidence/（E5），expand 句柄天然指向"},
	},
	{
		Kind:        "epm",
		CentralSeed: "导入回执产物（message_loop.go:8578）",
		PayloadFields: []PayloadFieldSpec{
			numField("epm.section.count"),
			numField("epm.marker.position"),
		},
		Notes: []string{"段落坐标与 L2-2 段落投影（D10）未来合流，v1 只登记不深建"},
	},
	{
		Kind:         "rlm",
		LegacyPrefix: "rlm_",
		CentralSeed:  "capability preflight 产物（gain_staging.go:80）",
		PayloadFields: []PayloadFieldSpec{
			numField("rlm.summary.integrated_lufs"),
			numField("rlm.summary.true_peak_db"),
			numField("rlm.row.target_delta_db"),
		},
		Notes: []string{
			"无 LLMContext（manifest §5 缺口）不影响索引面",
			"E8 三源合并无 revision 比较——stale 风险按现状透传，修复归 G2",
		},
	},
	{
		Kind:         "acp",
		LegacyPrefix: "acoustic_package_status:",
		CentralSeed:  "落盘 acoustic_package_status.json（M4 store：Upsert 交叉失效已实现，物化层最成熟种子）",
		PayloadFields: []PayloadFieldSpec{
			numField("acp.band.energy_db"),
			numField("acp.loudness.lufs"),
			numField("acp.stereo.correlation"),
		},
		Notes: []string{"revision 指纹（M4 rev_hex8）直接映射 ref 的 snapshot 段"},
	},
	{
		Kind:         "dad.l3",
		LegacyPrefix: "dad.l3.",
		CentralSeed:  "L3 特征族落盘行 + probe 帧 + paired 工件（事件目录 #4-#9）",
		PayloadFields: []PayloadFieldSpec{
			numField("dad.l3.band_energy.summary_db"),
		},
		Notes: []string{"内核 D 类迁移前以 legacy 翻译条目进中央索引（三态解析 legacy 态），迁移后升 parsed"},
	},
	{
		Kind:         "dad.l2_render_probe",
		LegacyPrefix: "dad.l2_render_probe:",
		CentralSeed:  "L3 特征族落盘行 + probe 帧 + paired 工件",
		PayloadFields: []PayloadFieldSpec{
			numField("dad.l2_probe.bands.peak_db"),
		},
		Notes: []string{"迁移前 legacy 翻译条目（同 dad.l3 特记）"},
	},
	{
		Kind:         "dad.compressor_dual_tap",
		LegacyPrefix: "dad.compressor_dual_tap:",
		CentralSeed:  "L3 特征族落盘行 + probe 帧 + paired 工件",
		Notes:        []string{"迁移前 legacy 翻译条目；§3.3 未建议 v1 载荷字段，目录仅登记"},
	},
	{
		Kind:         "dad.frequency_evidence",
		LegacyPrefix: "dad.frequency_evidence:",
		CentralSeed:  "WorkflowData/orchestration 产物（read-first 库，model.go:16/46）",
		PayloadFields: []PayloadFieldSpec{
			strField("fci.issue.severity"),
		},
		Notes: []string{"C1 能力域专用，v1 可缓接；字段名按 §3.3 保留 fci 历史前缀"},
	},
}

// KindRouteTable 返回 kind 路由表副本。
func KindRouteTable() []KindRouteEntry {
	return append([]KindRouteEntry(nil), kindRoutingTable...)
}

var (
	kindRouteOnce      sync.Once
	kindRouteByKind    map[string]KindRouteEntry
	registeredKindOnce sync.Once
	registeredKindSet  map[string]struct{}
)

func kindRouteFor(kind string) (KindRouteEntry, bool) {
	kindRouteOnce.Do(func() {
		kindRouteByKind = make(map[string]KindRouteEntry, len(kindRoutingTable))
		for _, entry := range kindRoutingTable {
			kindRouteByKind[entry.Kind] = entry
		}
	})
	e, ok := kindRouteByKind[kind]
	return e, ok
}

func isRegisteredKind(kind string) bool {
	registeredKindOnce.Do(func() {
		registeredKindSet = make(map[string]struct{})
		for _, k := range agentprotocol.RegisteredRefKinds() {
			registeredKindSet[k] = struct{}{}
		}
	})
	_, ok := registeredKindSet[kind]
	return ok
}

// resolveQueryKinds 解析 kind 谓词：校验注册面（未知即 ErrUnknownKind），空集
// 展开为全部注册 kind，返回去重排序后的 kind 列表（决策与执行顺序确定性）。
func resolveQueryKinds(k KindPredicate) ([]string, error) {
	if len(k.Kinds) == 0 {
		all := agentprotocol.RegisteredRefKinds()
		out := make([]string, len(all))
		copy(out, all)
		sort.Strings(out)
		return out, nil
	}
	seen := make(map[string]struct{}, len(k.Kinds))
	for _, kind := range k.Kinds {
		if !isRegisteredKind(kind) {
			return nil, fmt.Errorf("%w: %q（注册面见 agentprotocol.RegisteredRefKinds）", ErrUnknownKind, kind)
		}
		seen[kind] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for kind := range seen {
		out = append(out, kind)
	}
	sort.Strings(out)
	return out, nil
}

// ---- 谓词类 → 路由目标（§3.2 表）----

// RouteTarget 路由目标六态。
type RouteTarget string

const (
	RouteCentral       RouteTarget = "central"        // 中央段索引
	RoutePayloadIndex  RouteTarget = "payload_index"  // kind 局部载荷索引
	RouteDegraded      RouteTarget = "degraded"       // 中央候选集 + 逐行载荷求值（compile）
	RouteResolve       RouteTarget = "resolve"        // 物化层 Resolve（expand）
	RouteDiffIdentity  RouteTarget = "diff_identity"  // 中央（两快照视图 + 坐标对齐）
	RouteDiffDelegated RouteTarget = "diff_delegated" // 委托既有差分承载者（§2.5）
)

// RouteDecision 单 kind 侧的裁决：谓词类 + 目标 + 成本 + 降级标注。
type RouteDecision struct {
	Class     string // "R1".."R10"
	Target    RouteTarget
	CostClass string
	Degraded  string
}

// RouteQuery 对查询按 kind 拆分并裁决路由（R7：载荷字段带 kind 前缀，天然可拆）。
// 未注册 kind 返回 nil（ErrUnknownKind 语义在 Query 入口报出）。
func RouteQuery(q RefQuery, reg PayloadIndexRegistry) []RouteDecision {
	kinds, err := resolveQueryKinds(q.Kinds)
	if err != nil {
		return nil
	}
	payloadInvolved := len(q.Payload) > 0 || hasPayloadSortKey(q.Sort)
	decisions := make([]RouteDecision, 0, len(kinds))
	for _, kind := range kinds {
		if payloadInvolved {
			class := "R6"
			if len(q.Payload) > 0 {
				class = "R5"
			}
			if reg != nil {
				if _, ok := reg.ForKind(kind); ok {
					decisions = append(decisions, RouteDecision{Class: class, Target: RoutePayloadIndex, CostClass: CostClassIndex})
					continue
				}
			}
			decisions = append(decisions, RouteDecision{
				Class: class, Target: RouteDegraded, CostClass: CostClassCompile, Degraded: DegradedPayloadIndexUnavailable,
			})
			continue
		}
		decisions = append(decisions, RouteDecision{Class: segmentClass(q), Target: RouteCentral, CostClass: CostClassIndex})
	}
	return decisions
}

// segmentClass 段谓词类判定：R4（snapshot/hash）> R3（window）> R2（scope）> R1。
func segmentClass(q RefQuery) string {
	if q.Hash != nil || (q.Snapshot.Mode != "" && q.Snapshot.Mode != SnapshotLatest) || q.Snapshot.Revision != "" {
		return "R4"
	}
	if q.Window != nil {
		return "R3"
	}
	if q.Scope != nil {
		return "R2"
	}
	return "R1"
}

// RouteExpand 裁决 expand（R8）：句柄=index；大摘要=compile。
func RouteExpand(d ExpandDepth) RouteDecision {
	if d == ExpandSummary {
		return RouteDecision{Class: "R8", Target: RouteResolve, CostClass: CostClassCompile}
	}
	return RouteDecision{Class: "R8", Target: RouteResolve, CostClass: CostClassIndex}
}

// RouteDiff 裁决差分（R9/R10）：identity=index（引擎自有集合差）；content=委托
// 既有差分承载者（COM change_delta / before_after / FXM，compile~probe）。
func RouteDiff(d DiffDepth) RouteDecision {
	if d == DiffContent {
		return RouteDecision{Class: "R10", Target: RouteDiffDelegated, CostClass: CostClassCompile}
	}
	return RouteDecision{Class: "R9", Target: RouteDiffIdentity, CostClass: CostClassIndex}
}

func hasPayloadSortKey(keys []SortKey) bool {
	for _, k := range keys {
		if !isRefSortAlias(k.Field) {
			return true
		}
	}
	return false
}

// sortFieldDeclared 排序声明检查（R6）：载荷排序字段须由路由表建议字段或已注册
// 局部索引声明（且 Sortable）；ref 别名恒可排。
func sortFieldDeclared(kind, field string, reg PayloadIndexRegistry) bool {
	if entry, ok := kindRouteFor(kind); ok {
		for _, f := range entry.PayloadFields {
			if f.Field == field && f.Sortable {
				return true
			}
		}
	}
	if reg != nil {
		if idx, ok := reg.ForKind(kind); ok {
			for _, f := range idx.Fields() {
				if f.Field == field && f.Sortable {
					return true
				}
			}
		}
	}
	return false
}
