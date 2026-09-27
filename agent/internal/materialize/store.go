// Package materialize 落地 docs/MATERIALIZATION_V1_DESIGN.md 的物化层骨架
// （MAT-A）：内存代际索引（整表 copy-on-write 原子交换）+ 读口三函数
// （SnapshotView/Resolve/Subscribe，契约=agentprotocol.MaterializedStore）
// + 写侧 Upsert 行级 diff + MarkStale 标脏 + Metrics（G2 断言证据）+
// manifest 落盘与崩溃恢复。
//
// MAT-A 边界（设计 §8 排程）：事件接线（Notifier 三挂点）、表 B 脏传播、
// Recompute 适配是 MAT-B/MAT-C——本包当前零装配（无 harness/mixboard 引用，
// 纯新增）。并发纪律（§7.3）：v1 单写者，但 Store 以互斥锁保证多写者正确性；
// generation 原子交换是唯一跨 goroutine 可见点。
package materialize

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"

	"vit-daw-agent/internal/agentprotocol"
)

// 错误面（读侧零推断：miss/不支持/未接线一律显式，不静默）。
var (
	// ErrNoMaterializedRow：Resolve 命中未知坐标（miss 如实上报，不静默空）。
	ErrNoMaterializedRow = errors.New("materialize: no materialized row at ref coordinate")
	// ErrUnsupportedSelector：v1 只保当前代（OQ-2），at_or_before 显式拒绝。
	ErrUnsupportedSelector = errors.New("materialize: snapshot selector unsupported in v1 (latest/exact only, OQ-2)")
	// ErrEvidenceCASNotWired：evidence:// 句柄的内容读取需要 CAS 接线（MAT-C）。
	ErrEvidenceCASNotWired = errors.New("materialize: evidence:// content read requires projectstore CAS wiring (MAT-C)")
	// ErrNoManifestDir：内存库（NewStore）没有 manifest 落盘目录。
	ErrNoManifestDir = errors.New("materialize: store has no manifest directory")
)

// subscribeBufferSize 是每订阅者的独立缓冲（§6.2：慢消费者丢弃+计数，不阻塞
// 写侧）。溢出即丢弃并计入 Metrics.DroppedChanges——订阅者可靠消费靠重拉
// SnapshotView（全量喂给是契约另一半）。
const subscribeBufferSize = 128

// Row 是写侧提交行（设计 §2.1 materialRow 的输入形态）：调用方（重算/登记
// 路径）提供内容身份与句柄；Freshness 由写侧统一标注（Upsert→current，
// §2.3 判定全在写侧），不取调用方输入。
type Row struct {
	Ref     agentprotocol.Ref // 五段完整；Hash=本次内容的 sha256:16hex
	Payload map[string]any    // 投影声明的标量（PayloadFieldSpec 对齐）
	Handle  string            // evidence://<sha256>（PutEvidence 产物）或工件路径句柄
}

// materialRow 是存储形态（§2.1）：契约行 + 存储内部字段。坐标（主键）=
// ref 五段中的四段（kind/scope/window/snapshot）；hash 是值不是键——重算前
// hash 仍是旧内容身份。
type materialRow struct {
	Ref           agentprotocol.Ref
	Freshness     string
	Payload       map[string]any
	Handle        string
	Recomputed    int64  // 重算代际计数（G2 断言证据，§5.1）
	InvalidatedBy string // 最近失效 ChangeID 记账（§4.3 步 5，台账同款语义）
}

// generation 是一次原子提交后的不可变整表（§2.1：copy-on-write、读侧无锁）。
// 发布后 rows 与 id 不再变更；下一笔提交克隆出新代。
type generation struct {
	id   int64
	rows map[string]materialRow
}

// Store 是物化库的内存代际索引。零值不可用；用 NewStore（内存库）或
// OpenStore（带 manifest 目录，可恢复）构造。
type Store struct {
	mu      sync.RWMutex
	current *generation
	subs    map[*subscription]struct{}
	dir     string // 非空 = manifest 落盘目录
	metrics metricsState
}

type subscription struct {
	ch chan agentprotocol.MaterializedChange
}

// 编译期断言：Store 实现 agentprotocol 三函数契约（§6.1 下沉版）。
var _ agentprotocol.MaterializedStore = (*Store)(nil)

// NewStore 返回内存库（无 manifest 目录；SaveManifest 将报 ErrNoManifestDir）。
func NewStore() *Store {
	return &Store{
		current: &generation{id: 0, rows: map[string]materialRow{}},
		subs:    map[*subscription]struct{}{},
	}
}

// OpenStore 返回以 dir 为 manifest 目录的库；目录中存在 manifest.json 时按
// §2.1 恢复语义重建：行全量在场、统一降级 material_reuse、generation 还原。
func OpenStore(dir string) (*Store, error) {
	s := NewStore()
	s.dir = dir
	if err := s.loadManifest(); err != nil {
		return nil, err
	}
	return s, nil
}

// Generation 返回当前已提交代编号（空库 0；每笔有状态变化的提交恰好 +1）。
func (s *Store) Generation() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current.id
}

// rowKey 是行的坐标键（主键四段；hash 是值不进键）。
func rowKey(ref agentprotocol.Ref) string {
	window := "all"
	if ref.Window != nil && !ref.Window.AllTime {
		window = fmt.Sprintf("%d..%d", ref.Window.SampleStart, ref.Window.SampleEnd)
	}
	return ref.Kind + "\x1f" + ref.ScopeKind + "\x1f" + ref.ScopeValue + "\x1f" + window + "\x1f" + ref.Snapshot
}

// ---------------------------------------------------------------------------
// 写侧：Upsert（重算/登记的提交点）
// ---------------------------------------------------------------------------

// Upsert 提交一批行（§6.2 写侧入口，queryengine 不知道的第四个函数）：
// 行级 diff →added/replaced 事件 →generation 原子推进。
//
// 语义（§2.3/§4.4/§5.1，测试=store_test.go T 系列与伴生）：
//   - 整批 all-or-nothing：任一行 ref 非法（refschema Validate/注册表门）
//     则整批拒绝，状态零变化；
//   - 提交行 freshness 统一标注 current（写侧标注，读侧零推断）；
//   - 行级 diff：新坐标→added；hash/payload/handle/freshness 任一变化→
//     replaced（stale 行经重算提交回 current 也走 replaced）；逐字段同值→
//     不发事件、计入 RowsUnchanged；
//   - generation：有状态变化才 +1（同内容重提交是 no-op，幂等不抖动）；
//   - 事件序=批内行序（订阅者见到的序列=提交序）。
func (s *Store) Upsert(rows []Row) error {
	if len(rows) == 0 {
		return nil
	}
	staged := make([]materialRow, 0, len(rows))
	for _, row := range rows {
		if err := row.Ref.Validate(); err != nil {
			return fmt.Errorf("materialize: upsert batch rejected (all-or-nothing): %w", err)
		}
		staged = append(staged, materialRow{
			Ref:     cloneRef(row.Ref),
			Payload: clonePayload(row.Payload),
			Handle:  row.Handle,
		})
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	next := &generation{id: s.current.id, rows: make(map[string]materialRow, len(s.current.rows)+len(staged))}
	for k, v := range s.current.rows {
		next.rows[k] = v
	}
	batchKinds := map[string]bool{}
	changedByKind := map[string]int64{}
	unchangedByKind := map[string]int64{}
	events := make([]agentprotocol.MaterializedChange, 0, len(staged))
	for _, row := range staged {
		row.Freshness = agentprotocol.FreshnessCurrent
		key := rowKey(row.Ref)
		batchKinds[row.Ref.Kind] = true
		old, exists := next.rows[key]
		if exists && sameMaterial(old, row) {
			unchangedByKind[row.Ref.Kind]++
			continue
		}
		if exists {
			row.Recomputed = old.Recomputed + 1
		} else {
			row.Recomputed = 1
		}
		next.rows[key] = row
		op := agentprotocol.MaterializedOpReplaced
		if !exists {
			op = agentprotocol.MaterializedOpAdded
		}
		events = append(events, changeEvent(op, row))
		changedByKind[row.Ref.Kind]++
	}
	for kind := range batchKinds {
		s.metrics.addRecompute(kind)
	}
	for kind, n := range unchangedByKind {
		s.metrics.addUnchanged(kind, n)
	}
	if len(events) == 0 {
		return nil // no-op 提交：generation 不前进
	}
	for kind, n := range changedByKind {
		s.metrics.addUpserted(kind, n)
	}
	next.id = s.current.id + 1
	s.current = next
	s.publish(events)
	return nil
}

// sameMaterial 判定两行（同坐标）是否逐值相同：hash（Ref 内）、freshness、
// handle、payload 任一变化即视为已变更（replaced）。payload 的 nil 与空映射
// 视为相等（标量载荷的无差别形态）。
func sameMaterial(a, b materialRow) bool {
	return a.Ref.Hash == b.Ref.Hash &&
		a.Freshness == b.Freshness &&
		a.Handle == b.Handle &&
		payloadEqual(a.Payload, b.Payload)
}

func payloadEqual(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// ---------------------------------------------------------------------------
// 写侧：MarkStale（标脏——MAT-B 脏传播的落点，MAT-A 提供行级原语）
// ---------------------------------------------------------------------------

// RowMatch 按 ref 坐标选行（MAT-B HandleReceipt 解析出受影响 kind/scope 后
// 调用）。nil/空字段=不约束（Kinds 空=全部 kind；ScopeValues 空=全部 scope）。
type RowMatch struct {
	Kinds       []string
	ScopeKind   string
	ScopeValues []string
}

// MarkStale 把匹配行标脏（§2.3：current→stale，只改状态不删行；material_reuse
// 命中同样降为 stale——脏状态是单向的）。返回本次实际标脏行数。
//
//   - 事件：每行一条 marked_stale（事件序=rowKey 字典序，多行提交确定性）；
//   - 记账：changeID 记入行（InvalidatedBy，§4.3 步 5 失效证据可追溯）；
//   - 幂等：行已 stale 且 InvalidatedBy==changeID（changeID 非空）→跳过
//     （同事件重复投递不产生代际抖动；MAT-B 事件级重投策略在本原语上收敛）；
//   - generation：有行被标脏才 +1。
func (s *Store) MarkStale(match RowMatch, changeID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := &generation{id: s.current.id, rows: make(map[string]materialRow, len(s.current.rows))}
	for k, v := range s.current.rows {
		next.rows[k] = v
	}
	keys := make([]string, 0, len(next.rows))
	for key, row := range next.rows {
		if !matchRow(match, row.Ref) {
			continue
		}
		if changeID != "" && row.Freshness == agentprotocol.FreshnessStale && row.InvalidatedBy == changeID {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return 0, nil
	}
	sort.Strings(keys)
	invalidatedKinds := map[string]bool{}
	events := make([]agentprotocol.MaterializedChange, 0, len(keys))
	for _, key := range keys {
		row := next.rows[key]
		row.Freshness = agentprotocol.FreshnessStale
		row.InvalidatedBy = changeID
		next.rows[key] = row
		events = append(events, changeEvent(agentprotocol.MaterializedOpMarkedStale, row))
		invalidatedKinds[row.Ref.Kind] = true
	}
	for kind := range invalidatedKinds {
		s.metrics.addInvalidation(kind)
	}
	next.id = s.current.id + 1
	s.current = next
	s.publish(events)
	return len(keys), nil
}

func matchRow(match RowMatch, ref agentprotocol.Ref) bool {
	if len(match.Kinds) > 0 && !containsString(match.Kinds, ref.Kind) {
		return false
	}
	if match.ScopeKind != "" && match.ScopeKind != ref.ScopeKind {
		return false
	}
	if len(match.ScopeValues) > 0 && !containsString(match.ScopeValues, ref.ScopeValue) {
		return false
	}
	return true
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 读侧三函数（契约=agentprotocol.MaterializedStore）
// ---------------------------------------------------------------------------

// SnapshotView 返回当前已提交代的不可变整表视图（read-committed-per-
// snapshot，QUERY_ENGINE §3.4：一次调用绑定单一已提交代，并发提交不撕裂
// 结果）。行序=rowKey 字典序（确定性迭代）。
//
// selector 解析（§6.2）：latest（含空 Mode 默认）=当前代全量；exact=按行
// Snapshot 段值过滤（Revision/ObservationID 皆设时取 AND，皆空=不过滤）；
// at_or_before 未支持（OQ-2：v1 只保当前代）→ErrUnsupportedSelector。
func (s *Store) SnapshotView(ctx context.Context, sel agentprotocol.SnapshotSelector) ([]agentprotocol.MaterializedRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	mode := sel.Mode
	if mode == "" {
		mode = agentprotocol.SnapshotModeLatest
	}
	var exact bool
	switch mode {
	case agentprotocol.SnapshotModeLatest:
	case agentprotocol.SnapshotModeExact:
		exact = true
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedSelector, mode)
	}

	s.mu.RLock()
	gen := s.current
	out := make([]agentprotocol.MaterializedRow, 0, len(gen.rows))
	kinds := make([]string, 0, 4)
	seenKind := map[string]bool{}
	for _, row := range gen.rows {
		if exact && !exactSnapshotMatch(sel, row.Ref.Snapshot) {
			continue
		}
		out = append(out, contractRow(row))
		if !seenKind[row.Ref.Kind] {
			seenKind[row.Ref.Kind] = true
			kinds = append(kinds, row.Ref.Kind)
		}
	}
	s.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool { return rowKey(out[i].Ref) < rowKey(out[j].Ref) })
	s.metrics.addViewReads(kinds)
	return out, nil
}

func exactSnapshotMatch(sel agentprotocol.SnapshotSelector, snapshot string) bool {
	if sel.Revision != "" && sel.Revision != snapshot {
		return false
	}
	if sel.ObservationID != "" && sel.ObservationID != snapshot {
		return false
	}
	return true
}

// Resolve 把单个 ref 解析为内容句柄+状态（§6.2）。查找按坐标四段（hash 是
// 值不是键——旧 hash 引用解析到当前行）；miss 如实报 ErrNoMaterializedRow。
//
// 句柄语义：evidence:// 句柄透传，内容读取在 CAS 接线（MAT-C）前显式报
// ErrEvidenceCASNotWired（不伪造、不静默）；工件路径句柄给 os.ReadFile 形
// ReadAll（Bytes 以 stat 为准，stat 失败置 0、读取时报错）。
func (s *Store) Resolve(ctx context.Context, ref agentprotocol.Ref) (agentprotocol.ResolvedEvidence, error) {
	if err := ctx.Err(); err != nil {
		return agentprotocol.ResolvedEvidence{}, err
	}
	s.mu.RLock()
	row, ok := s.current.rows[rowKey(ref)]
	s.mu.RUnlock()

	s.metrics.addResolveRead(ref.Kind, ok)
	if !ok {
		return agentprotocol.ResolvedEvidence{}, fmt.Errorf("%w: %s/%s", ErrNoMaterializedRow, ref.Kind, ref.ScopeValue)
	}
	res := agentprotocol.ResolvedEvidence{
		Handle:    row.Handle,
		Freshness: row.Freshness,
	}
	switch {
	case row.Handle == "":
		res.ReadAll = func() ([]byte, error) { return nil, errors.New("materialize: row carries no content handle") }
	case strings.HasPrefix(row.Handle, "evidence://"):
		res.ReadAll = func() ([]byte, error) { return nil, ErrEvidenceCASNotWired }
	default:
		path := row.Handle
		if info, err := os.Stat(path); err == nil {
			res.Bytes = info.Size()
		}
		res.ReadAll = func() ([]byte, error) { return os.ReadFile(path) }
	}
	return res, nil
}

// Subscribe 注册一个变更流订阅者（每订阅者独立缓冲 channel；事件序=提交序）。
// 返回的 cancel 注销订阅并关闭通道；已入缓冲的事件在关闭后仍可排空。慢消费
// 者溢出即丢弃并计入 Metrics.DroppedChanges——写侧永不因订阅者阻塞（§6.2）。
func (s *Store) Subscribe(ctx context.Context) (<-chan agentprotocol.MaterializedChange, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sub := &subscription{ch: make(chan agentprotocol.MaterializedChange, subscribeBufferSize)}
	s.subs[sub] = struct{}{}
	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subs[sub]; !ok {
			return
		}
		delete(s.subs, sub)
		close(sub.ch)
	}
	return sub.ch, cancel, nil
}

// publish 把一批事件按提交序 fan-out 给全部订阅者（调用方须持 s.mu）。
func (s *Store) publish(events []agentprotocol.MaterializedChange) {
	for sub := range s.subs {
		for _, ev := range events {
			select {
			case sub.ch <- ev:
			default:
				s.metrics.addDrop()
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 内部工具
// ---------------------------------------------------------------------------

// changeEvent 构造流事件（行内容经克隆，防止存储态与流消费面共享可变结构）。
func changeEvent(op agentprotocol.MaterializedOp, row materialRow) agentprotocol.MaterializedChange {
	return agentprotocol.MaterializedChange{Op: op, Row: contractRow(row)}
}

// contractRow 转契约行（Ref 深克隆 Window 指针、Payload 克隆）。
func contractRow(row materialRow) agentprotocol.MaterializedRow {
	return agentprotocol.MaterializedRow{
		Ref:       cloneRef(row.Ref),
		Freshness: row.Freshness,
		Payload:   clonePayload(row.Payload),
	}
}

// cloneRef 深克隆 Ref（Window 是指针，防跨边界共享可变态）。
func cloneRef(ref agentprotocol.Ref) agentprotocol.Ref {
	if ref.Window == nil {
		return ref
	}
	window := *ref.Window
	ref.Window = &window
	return ref
}

// clonePayload 克隆标量载荷（浅拷；投影声明的标量集，不含嵌套可变结构）。
func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	out := make(map[string]any, len(payload))
	for k, v := range payload {
		out[k] = v
	}
	return out
}
