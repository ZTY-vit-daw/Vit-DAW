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
// 累计）+ Subscribe 慢消费者丢弃（§6.2）+ 崩溃恢复记账（§5.5，MAT-D：抢救/
// 跳过必须可见，非静默）。
type Metrics struct {
	PerKind           map[string]KindMetrics
	ShadowDivergences int64
	DroppedChanges    int64
	// ReconcileRows 影子对账累计比对行数（§7.1）——ShadowDivergences==0 的
	// 非空洞证据（比过行才谈得上零分歧）。
	ReconcileRows int64
	// ShadowMeasurementCarriedExcluded 影子对账被闸门排除的 measurement-carried
	// 行累计（MAT-D2 登记型：观察轮 dom 输入经 MixPackage 测量回退轴激活的行
	// 与物化侧不同源，登记 non_precomputable 后对账跳过——计数单列，不静默）。
	ShadowMeasurementCarriedExcluded int64
	// RecoverySkippedRows 恢复装载时被跳过的损坏行数（非法 ref/坐标重复）。
	RecoverySkippedRows int64
	// RecoverySalvaged manifest 半写经流式抢救恢复（截断记账：损失可见）。
	RecoverySalvaged bool
}

// metricsState 是 Store 内部仪表（并发安全，方法名按计数语义命名）。
type metricsState struct {
	mu      sync.Mutex
	perKind map[string]*KindMetrics
	dropped int64
	// shadowDivergences 是影子对账累计分歧行数（§7.1，MAT-C ReconcileShadow）。
	shadowDivergences int64
	// reconcileRows 是影子对账累计比对行数（MAT-D：零分歧的非空洞证据）。
	reconcileRows int64
	// measurementCarriedExcluded 是被闸门排除的 measurement-carried 行累计
	// （MAT-D2 登记型：单列计数，与分歧/比对行数分开可见）。
	measurementCarriedExcluded int64
	// recovery 记账（§5.5 MAT-D）：装载期一次写入，之后只读。
	recoverySkipped  int64
	recoverySalvaged bool
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

// addArrival 计 arrive 型事件级到达（MAT-B HandleFeatureArrival，按受影响
// kind；§5.1 Arrivals——事件发生了就计数，与行是否被标脏无关）。
func (m *metricsState) addArrival(kind string) {
	m.lock(kind).Arrivals++
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

// addShadowDivergence 累计影子对账分歧（§7.1：shadow 态每轮 observe 后现算
// 产物 vs 物化行逐行 hash 比的分歧计数；切换闸门 ShadowDivergences==0 的依据）。
func (m *metricsState) addShadowDivergence(n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shadowDivergences += n
}

// addReconcileRows 累计影子对账比对行数（MAT-D：与分歧计数同源，证明
// ShadowDivergences==0 是"比过且零"而非"没比过"）。
func (m *metricsState) addReconcileRows(n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconcileRows += n
}

// addMeasurementCarriedExcluded 累计被闸门排除的 measurement-carried 行数
// （MAT-D2 登记型：排除必须可见——计数单列，不静默）。
func (m *metricsState) addMeasurementCarriedExcluded(n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.measurementCarriedExcluded += n
}

// setRecovery 记恢复装载记账（§5.5 MAT-D：装载期一次写入；跳过行数+是否经
// 半写抢救——损失可见，非静默）。
func (m *metricsState) setRecovery(skipped int64, salvaged bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recoverySkipped = skipped
	m.recoverySalvaged = salvaged
}

// snapshot 返回深拷贝（外部改动不回写内部计数）。
func (m *metricsState) snapshot() Metrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := Metrics{
		PerKind:                         make(map[string]KindMetrics, len(m.perKind)),
		DroppedChanges:                  m.dropped,
		ShadowDivergences:               m.shadowDivergences,
		ReconcileRows:                   m.reconcileRows,
		ShadowMeasurementCarriedExcluded: m.measurementCarriedExcluded,
		RecoverySkippedRows:             m.recoverySkipped,
		RecoverySalvaged:                m.recoverySalvaged,
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
