package queryengine

// store.go — §5 物化层读侧接口（G2 边界，函数级契约）。
//
// 查询引擎只读物化层，不直读投影内存态；失效与新鲜度归物化层管，引擎收到
// 什么状态就透传什么状态。v0 bootstrap 适配器（四产物族只读扫描 + 轮询
// Subscribe）属 IMPL-B；本包只定义契约与消费逻辑。
//
// 单向依赖（§5.1.4）：queryengine → agentprotocol / projectstore 可以，反向禁止；
// 物化层不知道引擎存在（它只发变更流）。

import (
	"context"

	"vit-daw-agent/internal/agentprotocol"
)

// MaterializedStore 是查询引擎对物化层（L1-2 产出）的全部依赖面。
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

// SnapshotSelector 与 §2.1 SnapshotPredicate 同构的物化层选择器。
type SnapshotSelector struct {
	Mode          SnapshotMode // latest / exact / at_or_before
	Revision      string
	ObservationID string
}

// MaterializedRow 中央段索引的一行输入。
type MaterializedRow struct {
	Ref       agentprotocol.Ref
	Freshness string         // current / material_reuse / stale（透传，不升级）
	Payload   map[string]any // 投影声明并提供的标量（与 PayloadFieldSpec 对齐）
}

// MaterializedOp 变更流操作三态。
type MaterializedOp string

const (
	MaterializedAdded       MaterializedOp = "added"
	MaterializedReplaced    MaterializedOp = "replaced"
	MaterializedMarkedStale MaterializedOp = "marked_stale" // 只改状态不删行（§5.1.1）
)

// MaterializedChange 一次物化变更。
type MaterializedChange struct {
	Op  MaterializedOp
	Row MaterializedRow
}

// ResolvedEvidence Resolve 的产物：CAS 句柄优先；摘要提取由引擎 Expand 做
// （禁键过滤在引擎侧），物化层只给内容访问。
type ResolvedEvidence struct {
	Handle    string // evidence://<sha256>（projectstore CAS）或工件路径句柄
	Bytes     int64
	Freshness string
	ReadAll   func() ([]byte, error)
}
