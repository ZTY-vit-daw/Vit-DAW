package materialize

// store_test.go — MAT-A 契约测试（红先行：本文件先于实现落库）。
//
// 规格权威：docs/MATERIALIZATION_V1_DESIGN.md §2（三层存储/行状态机/manifest
// 恢复）、§5.1（Metrics 断言证据）、§5.5（读提交/generation 原子性/崩溃恢复
// ——卡面"G2-C T 系列"）、§6.2（三函数实现签名与语义）；读口契约签名=
// docs/QUERY_ENGINE_V1_DESIGN.md §5（零改动，仅下沉 agentprotocol）。
//
// 锁定的契约（卡面验收线）：
//
//	T1 stale 不删行/不升级  MarkStale 只改状态不删行；读路径（SnapshotView/
//	                     Resolve）不得回写 current；回到 current 的唯一路径
//	                     是写侧显式提交（Upsert）。
//	T2 读提交             SnapshotView 绑定单一已提交代（QUERY_ENGINE §3.4
//	                     read-committed-per-snapshot），并发提交不撕裂视图。
//	T3 generation 原子性  Upsert 整批 all-or-nothing；每笔提交 generation
//	                     至多 +1；含非法 ref 的整批拒绝且状态零变化。
//	T4 崩溃后代际恢复     manifest 重建后行全量在场、统一降级 material_reuse
//	                     （§2.1：不持久化 dirty 集合，恢复后所有存量行降级，
//	                     等首个权威快照事件重建基准）；坐标/句柄/载荷保真。
//	伴生                 Upsert 行级 diff 事件流（added/replaced/unchanged）、
//	                     Subscribe 提交序+慢消费丢弃+计数、Resolve 句柄语义
//	                     （坐标查找/文件句柄读/CAS 骨架边界）、selector 解析、
//	                     Metrics 快照复制语义、agentprotocol 契约实现断言。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/agentprotocol"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func testRef(kind, scopeValue, snapshot, hash string) agentprotocol.Ref {
	return agentprotocol.Ref{
		Kind:       kind,
		ScopeKind:  "track",
		ScopeValue: scopeValue,
		Window:     &agentprotocol.TimeWindow{AllTime: true},
		Snapshot:   snapshot,
		Hash:       hash,
	}
}

func hashN(n int) string {
	return fmt.Sprintf("sha256:%016x", n)
}

func mustUpsert(t *testing.T, s *Store, rows ...Row) {
	t.Helper()
	if err := s.Upsert(rows); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
}

func latestSelector() agentprotocol.SnapshotSelector {
	return agentprotocol.SnapshotSelector{Mode: agentprotocol.SnapshotModeLatest}
}

func snapshotRows(t *testing.T, s *Store) []agentprotocol.MaterializedRow {
	t.Helper()
	view, err := s.SnapshotView(context.Background(), latestSelector())
	if err != nil {
		t.Fatalf("SnapshotView: %v", err)
	}
	return view
}

func rowByScope(view []agentprotocol.MaterializedRow, kind, scopeValue string) (agentprotocol.MaterializedRow, bool) {
	for _, row := range view {
		if row.Ref.Kind == kind && row.Ref.ScopeValue == scopeValue {
			return row, true
		}
	}
	return agentprotocol.MaterializedRow{}, false
}

func drainEvents(t *testing.T, ch <-chan agentprotocol.MaterializedChange, n int) []agentprotocol.MaterializedChange {
	t.Helper()
	out := make([]agentprotocol.MaterializedChange, 0, n)
	for len(out) < n {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("事件通道提前关闭：已收 %d/%d", len(out), n)
			}
			out = append(out, ev)
		case <-time.After(2 * time.Second):
			t.Fatalf("等待第 %d/%d 个事件超时", len(out)+1, n)
		}
	}
	return out
}

func assertNoEvent(t *testing.T, ch <-chan agentprotocol.MaterializedChange) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("不应有事件到达：op=%s kind=%s scope=%s", ev.Op, ev.Row.Ref.Kind, ev.Row.Ref.ScopeValue)
	default:
	}
}

// ---------------------------------------------------------------------------
// 契约实现断言（编译期）
// ---------------------------------------------------------------------------

// TestStoreImplementsMaterializedStore 锁定：Store 是 agentprotocol 三函数
// 契约（QUERY_ENGINE §5，MATERIALIZATION §6.1 下沉版）的实现方。
func TestStoreImplementsMaterializedStore(t *testing.T) {
	var _ agentprotocol.MaterializedStore = (*Store)(nil)
}

// ---------------------------------------------------------------------------
// 伴生：Upsert 行级 diff（§6.2 写侧入口）
// ---------------------------------------------------------------------------

// TestUpsertRowLevelDiffEvents 锁定：新增→added、内容换代→replaced、同内容
// →不发事件且 RowsUnchanged 计数；no-op 批不推进 generation（幂等提交）。
func TestUpsertRowLevelDiffEvents(t *testing.T) {
	s := NewStore()
	ch, cancel, err := s.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()

	mustUpsert(t, s,
		Row{Ref: testRef("dom", "T3", "obs-1", hashN(1)), Payload: map[string]any{"peak": -3.0}, Handle: "evidence://aa"},
		Row{Ref: testRef("tim", "T3", "obs-1", hashN(1)), Payload: map[string]any{"codec": "flac"}},
	)
	events := drainEvents(t, ch, 2)
	if events[0].Op != agentprotocol.MaterializedOpAdded || events[0].Row.Ref.Kind != "dom" {
		t.Fatalf("事件 0 应为 dom added，got op=%s kind=%s", events[0].Op, events[0].Row.Ref.Kind)
	}
	if events[1].Op != agentprotocol.MaterializedOpAdded || events[1].Row.Ref.Kind != "tim" {
		t.Fatalf("事件 1 应为 tim added，got op=%s kind=%s", events[1].Op, events[1].Row.Ref.Kind)
	}
	if events[0].Row.Freshness != agentprotocol.FreshnessCurrent {
		t.Fatalf("写侧标注：Upsert 提交行 freshness 应为 current，got %q", events[0].Row.Freshness)
	}

	// 第二批：dom 内容换代（hash 变）→replaced；tim 原样→无事件。
	mustUpsert(t, s,
		Row{Ref: testRef("dom", "T3", "obs-1", hashN(2)), Payload: map[string]any{"peak": -1.5}, Handle: "evidence://bb"},
		Row{Ref: testRef("tim", "T3", "obs-1", hashN(1)), Payload: map[string]any{"codec": "flac"}},
	)
	events = drainEvents(t, ch, 1)
	if events[0].Op != agentprotocol.MaterializedOpReplaced {
		t.Fatalf("内容换代应为 replaced，got %s", events[0].Op)
	}
	if events[0].Row.Ref.Hash != hashN(2) {
		t.Fatalf("replaced 行应携带新 hash，got %s", events[0].Row.Ref.Hash)
	}

	m := s.Metrics()
	if got := m.PerKind["dom"].RowsUpserted; got != 2 {
		t.Fatalf("dom RowsUpserted 应为 2（added+replaced），got %d", got)
	}
	if got := m.PerKind["tim"].RowsUnchanged; got != 1 {
		t.Fatalf("tim RowsUnchanged 应为 1，got %d", got)
	}
	if got := m.PerKind["dom"].Recomputes; got != 2 {
		t.Fatalf("dom Recomputes 应为 2（每批一次），got %d", got)
	}

	// no-op 批（逐字段同内容）：generation 不前进、不发事件。
	gen := s.Generation()
	mustUpsert(t, s, Row{Ref: testRef("tim", "T3", "obs-1", hashN(1)), Payload: map[string]any{"codec": "flac"}})
	if s.Generation() != gen {
		t.Fatalf("no-op 提交不得推进 generation：%d → %d", gen, s.Generation())
	}
	assertNoEvent(t, ch)
}

// ---------------------------------------------------------------------------
// T1：stale 不删行/不升级（§2.3 行状态机、§5.2 TestStaleNotDeletedNotUpgraded）
// ---------------------------------------------------------------------------

func TestStaleNotDeletedNotUpgraded(t *testing.T) {
	s := NewStore()
	ch, cancel, err := s.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()

	refT3 := testRef("dom", "T3", "obs-1", hashN(1))
	mustUpsert(t, s,
		Row{Ref: refT3, Payload: map[string]any{"peak": -3.0}},
		Row{Ref: testRef("dom", "T7", "obs-1", hashN(1))},
	)
	drainEvents(t, ch, 2)

	marked, err := s.MarkStale(RowMatch{Kinds: []string{"dom"}, ScopeValues: []string{"T3"}}, "chg-1")
	if err != nil {
		t.Fatalf("MarkStale: %v", err)
	}
	if marked != 1 {
		t.Fatalf("应恰好标脏 1 行（dom/T3），got %d", marked)
	}
	events := drainEvents(t, ch, 1)
	if events[0].Op != agentprotocol.MaterializedOpMarkedStale {
		t.Fatalf("标脏事件应为 marked_stale，got %s", events[0].Op)
	}
	if events[0].Row.Ref.ScopeValue != "T3" || events[0].Row.Freshness != agentprotocol.FreshnessStale {
		t.Fatalf("marked_stale 事件行应为 T3/stale，got %s/%s", events[0].Row.Ref.ScopeValue, events[0].Row.Freshness)
	}

	// 不删行：视图里两行都在；仅命中行 stale。
	view := snapshotRows(t, s)
	if len(view) != 2 {
		t.Fatalf("标脏后不得删行：视图应仍有 2 行，got %d", len(view))
	}
	rowT3, ok := rowByScope(view, "dom", "T3")
	if !ok || rowT3.Freshness != agentprotocol.FreshnessStale {
		t.Fatalf("dom/T3 应在场且 stale，got %+v", rowT3)
	}
	rowT7, ok := rowByScope(view, "dom", "T7")
	if !ok || rowT7.Freshness != agentprotocol.FreshnessCurrent {
		t.Fatalf("dom/T7 未被命中应保持 current，got %+v", rowT7)
	}

	// 不升级：读路径不得回写 current。
	res, err := s.Resolve(context.Background(), refT3)
	if err != nil {
		t.Fatalf("Resolve stale 行应命中： %v", err)
	}
	if res.Freshness != agentprotocol.FreshnessStale {
		t.Fatalf("Resolve 不得升级 freshness，got %q", res.Freshness)
	}
	if again := snapshotRows(t, s); len(again) != 2 {
		t.Fatalf("重复读不得改变行数： %d", len(again))
	}
	if rowT3Again, _ := rowByScope(snapshotRows(t, s), "dom", "T3"); rowT3Again.Freshness != agentprotocol.FreshnessStale {
		t.Fatalf("重复 SnapshotView 不得升级 freshness")
	}

	// 失效证据记账（§4.3 步 5：ChangeID 记入行）。
	if got := s.current.rows[rowKey(refT3)].InvalidatedBy; got != "chg-1" {
		t.Fatalf("行内应记账失效 ChangeID=chg-1，got %q", got)
	}
	if got := s.Metrics().PerKind["dom"].Invalidations; got != 1 {
		t.Fatalf("dom Invalidations 应为 1，got %d", got)
	}

	// 幂等重投：同 ChangeID 再标 →no-op（不前进 generation、不发事件）。
	gen := s.Generation()
	markedAgain, err := s.MarkStale(RowMatch{Kinds: []string{"dom"}, ScopeValues: []string{"T3"}}, "chg-1")
	if err != nil {
		t.Fatalf("MarkStale 重投: %v", err)
	}
	if markedAgain != 0 || s.Generation() != gen {
		t.Fatalf("同 ChangeID 重投应为 no-op：marked=%d gen=%d（want 0/%d）", markedAgain, s.Generation(), gen)
	}
	assertNoEvent(t, ch)

	// 回到 current 的唯一路径=写侧显式提交：同内容 Upsert 重算提交 →replaced。
	mustUpsert(t, s, Row{Ref: refT3, Payload: map[string]any{"peak": -3.0}})
	ev := drainEvents(t, ch, 1)
	if ev[0].Op != agentprotocol.MaterializedOpReplaced {
		t.Fatalf("stale→current 状态变化应发 replaced，got %s", ev[0].Op)
	}
	if rowT3Final, _ := rowByScope(snapshotRows(t, s), "dom", "T3"); rowT3Final.Freshness != agentprotocol.FreshnessCurrent {
		t.Fatalf("重算提交后应回 current，got %q", rowT3Final.Freshness)
	}
}

// ---------------------------------------------------------------------------
// T2：读提交（QUERY_ENGINE §3.4 / MATERIALIZATION §5.5 TestReadCommittedPerGeneration）
// ---------------------------------------------------------------------------

// TestReadCommittedPerGeneration 锁定：并发提交期间，任何 SnapshotView 结果都
// 是某个完整已提交代——撕裂读（一笔两行提交只见到一行、或见到跨代混排）必须
// 被排除。写者每笔原子替换 dom/T0+tim/T1 两行且 payload 携带同一 commit 代号，
// 合法视图内两行代号必须一致。
func TestReadCommittedPerGeneration(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	// 基准态：5 笔各两行 →10 行（coords 固定：dom 偶数轨、tim 奇数轨）。
	for i := 0; i < 5; i++ {
		mustUpsert(t, s,
			Row{Ref: testRef("dom", fmt.Sprintf("T%d", 2*i), "obs-1", hashN(i+1))},
			Row{Ref: testRef("tim", fmt.Sprintf("T%d", 2*i+1), "obs-1", hashN(i+1))},
		)
	}

	coords := map[string]bool{}
	for _, row := range snapshotRows(t, s) {
		coords[row.Ref.Kind+"/"+row.Ref.ScopeValue] = true
	}
	if len(coords) != 10 {
		t.Fatalf("基准态应有 10 个坐标，got %d", len(coords))
	}

	var (
		failMu   sync.Mutex
		failures []string
	)
	record := func(format string, args ...any) {
		failMu.Lock()
		defer failMu.Unlock()
		failures = append(failures, fmt.Sprintf(format, args...))
	}
	checkView := func(view []agentprotocol.MaterializedRow) {
		if len(view) != 10 {
			record("撕裂视图：行数 %d ≠ 10", len(view))
			return
		}
		for _, row := range view {
			if !coords[row.Ref.Kind+"/"+row.Ref.ScopeValue] {
				record("撕裂视图：陌生坐标 %s/%s", row.Ref.Kind, row.Ref.ScopeValue)
				return
			}
		}
		domT0, ok1 := rowByScope(view, "dom", "T0")
		timT1, ok2 := rowByScope(view, "tim", "T1")
		if !ok1 || !ok2 {
			record("撕裂视图：耦合对缺行")
			return
		}
		c1, k1 := domT0.Payload["commit"]
		c2, k2 := timT1.Payload["commit"]
		if !k1 || !k2 {
			return // 基准态（无 commit 代号）
		}
		if c1 != c2 {
			record("撕裂视图：同笔提交的两行 commit 代号不一致（%v vs %v）——跨代混排", c1, c2)
		}
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				view, err := s.SnapshotView(ctx, latestSelector())
				if err != nil {
					record("SnapshotView: %v", err)
					return
				}
				checkView(view)
			}
		}()
	}
	for i := 0; i < 50; i++ {
		mustUpsert(t, s,
			Row{Ref: testRef("dom", "T0", "obs-1", hashN(100+i)), Payload: map[string]any{"commit": i}},
			Row{Ref: testRef("tim", "T1", "obs-1", hashN(100+i)), Payload: map[string]any{"commit": i}},
		)
	}
	close(stop)
	wg.Wait()

	if len(failures) > 0 {
		t.Fatalf("读提交违约 %d 例，首例：%s", len(failures), failures[0])
	}
}

// ---------------------------------------------------------------------------
// T3：generation 原子性（§2.1 代际整表 copy-on-write 原子交换）
// ---------------------------------------------------------------------------

func TestGenerationAtomicCommit(t *testing.T) {
	s := NewStore()
	if got := s.Generation(); got != 0 {
		t.Fatalf("空库 generation 应为 0，got %d", got)
	}
	ch, cancel, err := s.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()

	mustUpsert(t, s,
		Row{Ref: testRef("dom", "T0", "obs-1", hashN(1))},
		Row{Ref: testRef("dom", "T2", "obs-1", hashN(1))},
		Row{Ref: testRef("tim", "T1", "obs-1", hashN(1))},
	)
	drainEvents(t, ch, 3)
	if got := s.Generation(); got != 1 {
		t.Fatalf("一笔提交 generation 应恰好 +1（0→1），got %d", got)
	}
	if got := len(snapshotRows(t, s)); got != 3 {
		t.Fatalf("整批原子可见：应 3 行，got %d", got)
	}

	// all-or-nothing：批内含非法 ref（hash 空 →refschema Validate 拒绝）→
	// 整批拒绝、generation 零变化、无事件、无部分行落库。
	gen := s.Generation()
	badBatch := []Row{
		Row{Ref: testRef("dom", "T4", "obs-1", hashN(1))},
		Row{Ref: testRef("dom", "T9", "obs-1", "")},
	}
	if err := s.Upsert(badBatch); err == nil {
		t.Fatalf("含非法 ref 的批应整体拒绝")
	}
	if s.Generation() != gen {
		t.Fatalf("拒绝的批不得推进 generation：%d → %d", gen, s.Generation())
	}
	if got := len(snapshotRows(t, s)); got != 3 {
		t.Fatalf("拒绝的批不得部分落库：应仍 3 行，got %d", got)
	}
	assertNoEvent(t, ch)

	// 未注册 kind 同样拒绝（refschema 注册表门）。
	if err := s.Upsert([]Row{Row{Ref: testRef("nosuchkind", "T5", "obs-1", hashN(1))}}); err == nil {
		t.Fatalf("未注册 kind 应拒绝")
	}
	if s.Generation() != gen {
		t.Fatalf("未注册 kind 拒绝不得推进 generation")
	}
}

// ---------------------------------------------------------------------------
// T4：崩溃后代际恢复（§2.1 manifest；§5.5 TestRecoveryFromManifestRebuildsIndex）
// ---------------------------------------------------------------------------

func TestRecoveryFromManifestRebuildsIndex(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if got := len(snapshotRows(t, s)); got != 0 {
		t.Fatalf("空目录起步应为空库，got %d 行", got)
	}

	artifact := filepath.Join(dir, "probe.json")
	if err := os.WriteFile(artifact, []byte(`{"tracks":12}`), 0o644); err != nil {
		t.Fatal(err)
	}
	refT3 := testRef("dom", "T3", "obs-1", hashN(1))
	refT7 := testRef("dom", "T7", "obs-1", hashN(1))
	refProj := testRef("tom", "proj", "rev-a", hashN(3))
	mustUpsert(t, s,
		Row{Ref: refT3, Payload: map[string]any{"peak": -3.0}, Handle: "evidence://aa"},
		Row{Ref: refT7},
		Row{Ref: refProj, Payload: map[string]any{"tracks": 12}, Handle: artifact},
	)
	if _, err := s.MarkStale(RowMatch{Kinds: []string{"dom"}, ScopeValues: []string{"T7"}}, "chg-recover"); err != nil {
		t.Fatalf("MarkStale: %v", err)
	}
	gen := s.Generation()
	if err := s.SaveManifest(); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}

	s2, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("恢复 OpenStore: %v", err)
	}
	if s2.Generation() != gen {
		t.Fatalf("恢复应还原 generation：%d（manifest %d）", s2.Generation(), gen)
	}

	view := snapshotRows(t, s2)
	if len(view) != 3 {
		t.Fatalf("恢复后行应全量在场：3 行，got %d", len(view))
	}
	// §2.1：不持久化 dirty 集合——恢复后所有存量行统一降级 material_reuse
	//（含恢复前 stale 的 T7；宁多标不漏标由首个权威快照事件重建基准）。
	for _, row := range view {
		if row.Freshness != agentprotocol.FreshnessMaterialReuse {
			t.Fatalf("恢复后 %s/%s 应降级 material_reuse，got %q", row.Ref.Kind, row.Ref.ScopeValue, row.Freshness)
		}
	}
	rowT3, _ := rowByScope(view, "dom", "T3")
	if rowT3.Ref.Hash != hashN(1) || rowT3.Payload["peak"] != -3.0 {
		t.Fatalf("恢复后坐标/载荷应保真：got hash=%s payload=%v", rowT3.Ref.Hash, rowT3.Payload)
	}
	res, err := s2.Resolve(context.Background(), refT3)
	if err != nil {
		t.Fatalf("恢复后 Resolve 应命中: %v", err)
	}
	if res.Handle != "evidence://aa" || res.Freshness != agentprotocol.FreshnessMaterialReuse {
		t.Fatalf("恢复后句柄/状态应保真：handle=%s freshness=%s", res.Handle, res.Freshness)
	}
	if got := s2.current.rows[rowKey(refT7)].InvalidatedBy; got != "chg-recover" {
		t.Fatalf("恢复应保留失效记账（审计连续性），got %q", got)
	}

	// 恢复后可继续提交并再次保存（tmp+rename 覆盖写有效）。
	mustUpsert(t, s2, Row{Ref: testRef("tim", "T1", "obs-2", hashN(9))})
	if s2.Generation() != gen+1 {
		t.Fatalf("恢复后提交应从还原代继续推进：%d（want %d）", s2.Generation(), gen+1)
	}
	if err := s2.SaveManifest(); err != nil {
		t.Fatalf("二次 SaveManifest: %v", err)
	}
	s3, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("二次恢复 OpenStore: %v", err)
	}
	if got := len(snapshotRows(t, s3)); got != 4 {
		t.Fatalf("二次恢复应含新行：4 行，got %d", got)
	}

	// 内存库（NewStore）无目录 →SaveManifest 报错（显式边界，不静默丢清单）。
	if err := NewStore().SaveManifest(); !errors.Is(err, ErrNoManifestDir) {
		t.Fatalf("无目录 SaveManifest 应报 ErrNoManifestDir，got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 伴生：Subscribe 顺序/慢消费（§6.2：提交序 fan-out+慢消费者丢弃+计数）
// ---------------------------------------------------------------------------

func TestSubscribeOrderAndSlowConsumerDrop(t *testing.T) {
	s := NewStore()
	ch, cancel, err := s.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	mustUpsert(t, s, Row{Ref: testRef("dom", "T0", "obs-1", hashN(1))})
	mustUpsert(t, s, Row{Ref: testRef("dom", "T1", "obs-1", hashN(1))})
	if _, err := s.MarkStale(RowMatch{Kinds: []string{"dom"}, ScopeValues: []string{"T1"}}, "chg-order"); err != nil {
		t.Fatalf("MarkStale: %v", err)
	}
	events := drainEvents(t, ch, 3)
	want := []struct {
		op    agentprotocol.MaterializedOp
		scope string
	}{
		{agentprotocol.MaterializedOpAdded, "T0"},
		{agentprotocol.MaterializedOpAdded, "T1"},
		{agentprotocol.MaterializedOpMarkedStale, "T1"},
	}
	for i, w := range want {
		if events[i].Op != w.op || events[i].Row.Ref.ScopeValue != w.scope {
			t.Fatalf("事件 %d 应为 %s/%s，got %s/%s（订阅者见到的序列=提交序）", i, w.op, w.scope, events[i].Op, events[i].Row.Ref.ScopeValue)
		}
	}

	// 取消：通道关闭、后续写不阻塞不再投递。
	cancel()
	if _, ok := <-ch; ok {
		t.Fatalf("取消后通道应关闭")
	}
	mustUpsert(t, s, Row{Ref: testRef("dom", "T2", "obs-1", hashN(1))})

	// 慢消费者：缓冲 subscribeBufferSize、零排水 →写侧不得阻塞（阻塞=测试
	// 超时），溢出丢弃并计数（§6.2）。
	ch2, cancel2, err := s.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe 2: %v", err)
	}
	defer cancel2()
	const overflow = 72
	total := subscribeBufferSize + overflow
	for i := 0; i < total; i++ {
		mustUpsert(t, s, Row{Ref: testRef("dom", fmt.Sprintf("T1%03d", i), "obs-1", hashN(i+1))})
	}
	m := s.Metrics()
	if m.DroppedChanges != overflow {
		t.Fatalf("慢消费者丢弃计数应为 %d，got %d", overflow, m.DroppedChanges)
	}
	buffered := 0
	for {
		select {
		case _, ok := <-ch2:
			if !ok {
				goto drained
			}
			buffered++
		default:
			goto drained
		}
	}
drained:
	if buffered != subscribeBufferSize {
		t.Fatalf("缓冲内事件应恰为 %d，got %d", subscribeBufferSize, buffered)
	}
}

// ---------------------------------------------------------------------------
// 伴生：Resolve 句柄语义（§6.2；QUERY_ENGINE §5.1.3 CAS 优先）
// ---------------------------------------------------------------------------

func TestResolveReturnsHandleAndFreshness(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "artifact.json")
	content := []byte(`{"evidence":"paired"}`)
	if err := os.WriteFile(artifact, content, 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewStore()
	refT3 := testRef("dom", "T3", "obs-1", hashN(1))
	refFxm := testRef("fxm", "T5", "obs-1", hashN(2))
	mustUpsert(t, s,
		Row{Ref: refT3, Payload: map[string]any{"peak": -3.0}, Handle: artifact},
		Row{Ref: refFxm, Handle: "evidence://deadbeef0011"},
	)

	res, err := s.Resolve(context.Background(), refT3)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Handle != artifact || res.Freshness != agentprotocol.FreshnessCurrent {
		t.Fatalf("句柄/状态不符：handle=%s freshness=%s", res.Handle, res.Freshness)
	}
	if res.Bytes != int64(len(content)) {
		t.Fatalf("Bytes 应为 %d，got %d", len(content), res.Bytes)
	}
	got, err := res.ReadAll()
	if err != nil || !reflect.DeepEqual(got, content) {
		t.Fatalf("工件路径句柄应可读：%v %q", err, got)
	}

	// 坐标查找（hash 是值不是键，§2.1）：旧 hash 引用仍解析到当前行。
	res, err = s.Resolve(context.Background(), testRef("dom", "T3", "obs-1", hashN(999)))
	if err != nil || res.Handle != artifact {
		t.Fatalf("按坐标解析应命中当前行：%v handle=%s", err, res.Handle)
	}

	// CAS 句柄：MAT-C 接线前的骨架边界——句柄与状态如约返回，内容读取
	// 显式报错（不伪造、不静默）。
	res, err = s.Resolve(context.Background(), refFxm)
	if err != nil {
		t.Fatalf("Resolve CAS 行: %v", err)
	}
	if res.Handle != "evidence://deadbeef0011" {
		t.Fatalf("CAS 句柄应透传，got %s", res.Handle)
	}
	if _, err := res.ReadAll(); !errors.Is(err, ErrEvidenceCASNotWired) {
		t.Fatalf("未接线的 CAS 读应报 ErrEvidenceCASNotWired，got %v", err)
	}

	// miss：未知坐标如实报错（不静默空）。
	if _, err := s.Resolve(context.Background(), testRef("mom", "T3", "obs-1", hashN(1))); !errors.Is(err, ErrNoMaterializedRow) {
		t.Fatalf("未知坐标应报 ErrNoMaterializedRow，got %v", err)
	}

	m := s.Metrics()
	if m.PerKind["dom"].Reads != 2 || m.PerKind["dom"].Hits != 2 {
		t.Fatalf("dom Reads/Hits 应为 2/2，got %d/%d", m.PerKind["dom"].Reads, m.PerKind["dom"].Hits)
	}
	if m.PerKind["mom"].Misses != 1 {
		t.Fatalf("mom Misses 应为 1，got %d", m.PerKind["mom"].Misses)
	}
}

// ---------------------------------------------------------------------------
// 伴生：selector 解析（§6.2：latest=当前代；exact=按行 Snapshot 段过滤；
// at_or_before 未支持 →显式错误（OQ-2，v1 只保当前代））
// ---------------------------------------------------------------------------

func TestSnapshotSelectorLatestAndExact(t *testing.T) {
	s := NewStore()
	mustUpsert(t, s,
		Row{Ref: testRef("dom", "T3", "obs-1", hashN(1))},
		Row{Ref: testRef("dom", "T7", "obs-1", hashN(2))},
		Row{Ref: testRef("tim", "T3", "obs-2", hashN(3))},
		Row{Ref: testRef("tom", "proj", "rev-a", hashN(4))},
	)

	view := snapshotRows(t, s)
	if len(view) != 4 {
		t.Fatalf("latest 应返回全部 4 行，got %d", len(view))
	}

	exact, err := s.SnapshotView(context.Background(), agentprotocol.SnapshotSelector{Mode: agentprotocol.SnapshotModeExact, Revision: "obs-1"})
	if err != nil {
		t.Fatalf("exact: %v", err)
	}
	if len(exact) != 2 {
		t.Fatalf("exact{obs-1} 应 2 行，got %d", len(exact))
	}
	for _, row := range exact {
		if row.Ref.Snapshot != "obs-1" {
			t.Fatalf("exact 过滤泄漏：%s", row.Ref.Snapshot)
		}
	}

	exactObs, err := s.SnapshotView(context.Background(), agentprotocol.SnapshotSelector{Mode: agentprotocol.SnapshotModeExact, ObservationID: "obs-2"})
	if err != nil {
		t.Fatalf("exact(observation_id): %v", err)
	}
	if len(exactObs) != 1 || exactObs[0].Ref.Kind != "tim" {
		t.Fatalf("exact{obs-2} 应仅 tim 1 行，got %+v", exactObs)
	}

	if _, err := s.SnapshotView(context.Background(), agentprotocol.SnapshotSelector{Mode: agentprotocol.SnapshotModeAtOrBefore, Revision: "obs-1"}); !errors.Is(err, ErrUnsupportedSelector) {
		t.Fatalf("at_or_before 应报 ErrUnsupportedSelector（OQ-2），got %v", err)
	}

	// 空 Mode 默认 latest（QUERY_ENGINE §2.1"latest 默认"）。
	def, err := s.SnapshotView(context.Background(), agentprotocol.SnapshotSelector{})
	if err != nil || len(def) != 4 {
		t.Fatalf("空 Mode 应默认 latest 全量：%v %d", err, len(def))
	}

	if got := s.Metrics().PerKind["dom"].Reads; got < 1 {
		t.Fatalf("视图读取应计入 Reads，got %d", got)
	}
}

// ---------------------------------------------------------------------------
// 伴生：Metrics 快照复制语义（§5.1：测试与影子对账共用同一仪表）
// ---------------------------------------------------------------------------

func TestMetricsSnapshotIsCopy(t *testing.T) {
	s := NewStore()
	mustUpsert(t, s, Row{Ref: testRef("dom", "T3", "obs-1", hashN(1))})

	m1 := s.Metrics()
	if m1.PerKind["dom"].RowsUpserted != 1 {
		t.Fatalf("RowsUpserted 应为 1，got %d", m1.PerKind["dom"].RowsUpserted)
	}
	tampered := m1.PerKind["dom"]
	tampered.RowsUpserted = 99
	m1.PerKind["dom"] = tampered
	if got := s.Metrics().PerKind["dom"].RowsUpserted; got != 1 {
		t.Fatalf("Metrics() 必须返回副本（外部篡改不得污染仪表），got %d", got)
	}
}
