package queryengine

// payloadindex.go — §3.1 投影局部载荷索引契约（D2：统一寻址不统一存储引擎）。
//
// 某 kind 需要字段级过滤/排序/topK 而中央标量不够时，由该投影自建局部索引
// （倒排/位图/立方体随其便），实现 PayloadIndex。引擎不规定存储引擎，只规定
// 吃 ref 主键。v1 大多数 kind 没有局部索引，R5/R6 降级是常态——降级在响应里
// 可见（degraded + cost_class=compile）。

import (
	"context"
	"sort"
	"sync"
)

// PayloadIndex 是投影局部索引的契约（一 kind 一索引）。
type PayloadIndex interface {
	// Kind 报告该索引服务的注册 kind（一对一）。
	Kind() string
	// Fields 声明可过滤/可排序字段及支持的算子（进 Engine.Catalog）。
	Fields() []PayloadFieldSpec
	// Search 在该 kind 内执行段谓词（引擎已按 kind 路由到此）+ 载荷谓词，
	// 返回 canonical L0 ref 行（含排序与分页；排序字段必须 ∈ Fields 且 Sortable）。
	// 契约缺口（v1 注记）：Search 不回传总命中数，引擎侧 TotalMatches 只能取
	// len(rows) 下界；kind 侧实现卡若需精确 total 须扩展本契约。
	Search(ctx context.Context, seg SegmentConstraints, pay []PayloadCondition,
		sort []SortKey, limit int, cursor string) (rows []ResultRow, next string, err error)
	// ApplyChange 由物化层变更流驱动（引擎转发），非查询路径。
	ApplyChange(ch MaterializedChange) error
}

// PayloadFieldSpec 载荷字段声明（kind.domain.path 三段命名）。
type PayloadFieldSpec struct {
	Field    string
	Type     string      // "number" | "string" | "bool"
	Ops      []PayloadOp // 该字段支持的算子
	Sortable bool
}

// SegmentConstraints 引擎下发给局部索引的段谓词（单 kind 视角，与 RefQuery
// 同构；snapshot 谓词由引擎先解析为快照视图，此处仅透传供索引自校验）。
type SegmentConstraints struct {
	Scope    *ScopePredicate
	Window   *WindowPredicate
	Snapshot SnapshotPredicate
	Hash     *HashPredicate
}

// PayloadIndexRegistry 局部索引注册表（引擎依赖注入面）。
type PayloadIndexRegistry interface {
	ForKind(kind string) (PayloadIndex, bool)
	Kinds() []string
}

type payloadIndexMap struct {
	mu     sync.RWMutex
	byKind map[string]PayloadIndex
}

// NewPayloadIndexRegistry 由一组 PayloadIndex 构建注册表（一 kind 一索引，
// 重复 kind 后者覆盖前者）。
func NewPayloadIndexRegistry(indexes ...PayloadIndex) PayloadIndexRegistry {
	m := &payloadIndexMap{byKind: make(map[string]PayloadIndex, len(indexes))}
	for _, idx := range indexes {
		if idx == nil {
			continue
		}
		m.byKind[idx.Kind()] = idx
	}
	return m
}

func (m *payloadIndexMap) ForKind(kind string) (PayloadIndex, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	idx, ok := m.byKind[kind]
	return idx, ok
}

func (m *payloadIndexMap) Kinds() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	kinds := make([]string, 0, len(m.byKind))
	for kind := range m.byKind {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}
