package agentprotocol

// materialstore.go — 物化层读口契约（MAT-A 契约下沉）。
//
// 规格权威：docs/MATERIALIZATION_V1_DESIGN.md §6.1（裁决 R5：契约类型下沉
// 协议共享包）；签名原文=docs/QUERY_ENGINE_V1_DESIGN.md §5（零改动，仅包
// 位置从 queryengine 语境迁移——其 §5.1 契约要点四条全部保留有效）。
//
// 下沉依据（§6.1）：① Subscribe 的消费面不止 queryengine——台账（OQ-4）、
// 审计、未来 memory 检索都是潜在订阅者，契约留在引擎包会迫使非引擎消费者
// import 引擎；② G1 裁定 #4 已把前缀注册表挂在 agentprotocol（协议归属
// 惯例）；③ 双向解耦后"物化层不知道引擎存在"从运行时语义升级为编译期事实。
// 实现方：agent/internal/materialize（v1）；v0 bootstrap 适配器归引擎侧。
//
// 伴生词表常量（freshness 三分=设计 §2.3 行状态机；op 三值；selector 三态）
// 与契约同包下沉，供实现与消费两侧共用同一套词汇，不自创第二套。

import "context"

// MaterializedStore 是查询引擎对物化层（L1-2 产出）的全部依赖面。
// 实现方：v0 = 引擎内置 bootstrap 适配器（QUERY_ENGINE §5.2）；v1 =
// agent/internal/materialize.Store（本契约的 MAT-A 落地）。
type MaterializedStore interface {
	// SnapshotView 返回某快照代下的全部 ref 行（段字段 + freshness + 声明标量）。
	// 引擎用它构建/重建中央段索引。latest 模式由物化层解析为当前已提交代。
	SnapshotView(ctx context.Context, sel SnapshotSelector) ([]MaterializedRow, error)

	// Resolve 把单个 ref 解析为内容句柄（CAS evidence://…）+ 状态。
	// 内容不进引擎——expand 只拿句柄与字节预算内摘要。
	Resolve(ctx context.Context, ref Ref) (ResolvedEvidence, error)

	// Subscribe 提供增量变更流（新增/替换/失效标脏），引擎据此交换中央索引代。
	// 失效正确性（漏标=静默错误数据）归物化层与 G2 专项——引擎只消费事件，不推断。
	Subscribe(ctx context.Context) (<-chan MaterializedChange, func(), error)
}

// SnapshotMode is the snapshot-axis selector mode (QUERY_ENGINE §2.1).
// v1 物化层只保当前代：latest 正确；exact 按行 Snapshot 段过滤；at_or_before
// 随历史代保留策略走（MATERIALIZATION OQ-2，未支持——实现侧显式报错）。
type SnapshotMode string

const (
	SnapshotModeLatest     SnapshotMode = "latest"       // 当前已提交快照（默认）
	SnapshotModeExact      SnapshotMode = "exact"        // 精确 revision/observation_id
	SnapshotModeAtOrBefore SnapshotMode = "at_or_before" // ≤ 给定 revision 的最近一份（历史回溯）
)

// SnapshotSelector selects the snapshot generation to view. Zero Mode means
// latest (QUERY_ENGINE §2.1 "latest 默认").
type SnapshotSelector struct {
	Mode          SnapshotMode
	Revision      string
	ObservationID string
}

// Row freshness vocabulary — F5 铃三分语义的物化版（MATERIALIZATION §2.3）。
// 判定全部在写侧（标脏事件/重算提交/降级），读侧零推断（M1 铃原则）。
const (
	FreshnessCurrent       = "current"        // 本代已重算（或登记）且输入域此后未再变更
	FreshnessMaterialReuse = "material_reuse" // 未被标脏的存量行（含崩溃恢复后全部存量行）
	FreshnessStale         = "stale"          // 脏传播命中，未重算——只改状态不删行
)

// MaterializedOp is the change-stream operation vocabulary.
type MaterializedOp string

const (
	MaterializedOpAdded       MaterializedOp = "added"
	MaterializedOpReplaced    MaterializedOp = "replaced"
	MaterializedOpMarkedStale MaterializedOp = "marked_stale" // 只改状态不删行——审计与"曾经存在"的可见性保留（对齐 M2 台账语义）
)

// MaterializedRow is one materialized ref row as seen by read consumers.
type MaterializedRow struct {
	Ref       Ref
	Freshness string         // current / material_reuse / stale
	Payload   map[string]any // 投影声明并提供的标量（PayloadFieldSpec 对齐）
}

// MaterializedChange is one incremental change in the Subscribe stream.
type MaterializedChange struct {
	Op  MaterializedOp
	Row MaterializedRow
}

// ResolvedEvidence is the Resolve result: content handle plus state. 摘要提取
// 由引擎 Expand 做（禁键过滤在引擎侧），物化层只给内容访问。
type ResolvedEvidence struct {
	Handle    string // evidence://<sha256>（projectstore CAS，F5）或工件路径句柄
	Bytes     int64
	Freshness string
	ReadAll   func() ([]byte, error) // 受 CAS pin 保护；大对象实现方可给 Reader 形态
}
