package materialize

// metrics.go — 物化层仪表（MATERIALIZATION §5.1：断言证据三抓手之一）。
//
// per-kind 计数器族是 G2 测试与影子模式对账共用的同一仪表（注入事件后直接
// 读计数断言）。Metrics() 返回深拷贝快照——外部篡改不得污染内部计数。
// 仪表自带互斥：读路径（SnapshotView/Resolve 在 RLock 下）与写路径并发
// 计数，不依赖 Store 写锁。

import "sync"

// KindMetrics 是单 kind 的计数器族（§5.1 词表，不自增删）：
//
//	Invalidations  invalidate 型标脏事件计数（按受影响 kind）
//	Arrivals       arrive 型到达事件计数（MAT-B HandleFeatureArrival 接线后累计）
//	Recomputes     重算提交计数（每笔含该 kind 的 Upsert 批 +1）
//	RowsUpserted   added+replaced 行计数
//	RowsUnchanged  行级 diff 判定同值的行计数
//	Reads          读口调用计数（SnapshotView 按结果 kind 计、Resolve 按查 kind 计）
//	Hits / Misses  Resolve 命中/未命中计数
type KindMetrics struct {
	Invalidations int64
	Arrivals      int64
	Recomputes    int64
	RowsUpserted  int64
	RowsUnchanged int64
	Reads         int64
	Hits          int64
	Misses        int64
}

// Metrics 是全局面仪表：per-kind 族 + 影子对账分歧（§7.1，MAT-C 接线后
// 累计）+ Subscribe 慢消费者丢弃（§6.2）。
type Metrics struct {
	PerKind           map[string]KindMetrics
	ShadowDivergences int64
	DroppedChanges    int64
}

// metricsState 是 Store 内部仪表（并发安全，方法名按计数语义命名）。
type metricsState struct {
	mu      sync.Mutex
	perKind map[string]*KindMetrics
	dropped int64
}

func (m *metricsState) lock(kind string) *KindMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.perKind == nil {
		m.perKind = map[string]*KindMetrics{}
	}
	km, ok := m.perKind[kind]
	if !ok {
		km = &KindMetrics{}
		m.perKind[kind] = km
	}
	return km
}

func (m *metricsState) addRecompute(kind string) {
	m.lock(kind).Recomputes++
}

func (m *metricsState) addUpserted(kind string, n int64) {
	m.lock(kind).RowsUpserted += n
}

func (m *metricsState) addUnchanged(kind string, n int64) {
	m.lock(kind).RowsUnchanged += n
}

func (m *metricsState) addInvalidation(kind string) {
	m.lock(kind).Invalidations++
}

func (m *metricsState) addResolveRead(kind string, hit bool) {
	km := m.lock(kind)
	km.Reads++
	if hit {
		km.Hits++
	} else {
		km.Misses++
	}
}

func (m *metricsState) addViewReads(kinds []string) {
	if len(kinds) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.perKind == nil {
		m.perKind = map[string]*KindMetrics{}
	}
	for _, kind := range kinds {
		km, ok := m.perKind[kind]
		if !ok {
			km = &KindMetrics{}
			m.perKind[kind] = km
		}
		km.Reads++
	}
}

func (m *metricsState) addDrop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropped++
}

// snapshot 返回深拷贝（外部改动不回写内部计数）。
func (m *metricsState) snapshot() Metrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := Metrics{
		PerKind:        make(map[string]KindMetrics, len(m.perKind)),
		DroppedChanges: m.dropped,
	}
	for kind, km := range m.perKind {
		out.PerKind[kind] = *km
	}
	return out
}

// Metrics 返回仪表深拷贝快照。
func (s *Store) Metrics() Metrics {
	return s.metrics.snapshot()
}
