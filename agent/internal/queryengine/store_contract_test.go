package queryengine

// store_contract_test.go — MaterializedStore 契约测试（QUERY_ENGINE §5/§5.1，
// IMPL-B）：引擎依赖面的语义经契约口表驱动锁定。
//
// 双 fixture：
//   - bootstrap 适配器（本卡 v0 实现经合成产物树）；
//   - ProtocolStore 桥 + materialize.Store（v1 本体——upsert 代际/句柄保留/
//     marked_stale 不删行以 MAT-E 已验行为为准绳，不重定义：断言只引用
//     materialize 导出的写侧原语产生的可观察结果）。
//
// 锁定的契约要点（§5.1 四条 + MAT-E 语义）：
//   1. SnapshotView latest=全量 / exact=按行 snapshot 段过滤 / at_or_before
//      显式拒绝（代际管理归物化层，引擎零推断）；
//   2. Resolve 按坐标四段解析（hash 是值不是键：旧 hash 引用命中当前行）、
//      miss 显式报错（不静默空）、路径句柄 ReadAll 可读且 Bytes=stat 尺寸；
//   3. marked_stale 只改状态不删行（标脏后 SnapshotView 行仍在且 freshness=stale）；
//   4. upsert 幂等（同内容重提交不产生可见变化）与句柄保留（内容身份同源的
//      无句柄行不冲掉读端回填句柄——MAT-E）。

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/materialize"
)

// ---- fixture 面 ----

type contractFixture struct {
	name  string
	store MaterializedStore
	// materialize 侧句柄（写侧语义用例）；nil = bootstrap fixture（只读扫描，
	// 无写侧原语）。
	mat *materialize.Store
	// bootstrap 侧产物树（写侧等价物=落盘文件）；空 = materialize fixture。
	tree *syntheticBootstrapTree
}

func newBootstrapFixture(t *testing.T) contractFixture {
	t.Helper()
	tree := newSyntheticBootstrapTree(t)
	writeObsTicket(t, tree, bootstrapObsFull, true, true, true)
	writeAcousticStore(t, tree, synthAcousticPkg{"1007", "partial"})
	writeComPair(t, tree, "com2_contract01", "1012")
	writeFeatureSnapshot(t, tree, []string{"T1"}, nil, []string{"T1"})
	return contractFixture{
		name:  "bootstrap",
		store: bootstrapStoreOf(tree),
		tree:  &tree,
	}
}

// matRow 构造 materialize 写侧行（契约测试种子；坐标 window=all/snapshot=current，
// 与 MAT-C 行坐标裁定同款）。
func matRow(kind, scopeValue, hash, handle string, payload map[string]any) materialize.Row {
	return materialize.Row{
		Ref: agentprotocol.Ref{
			Kind: kind, ScopeKind: "track", ScopeValue: scopeValue,
			Window: &agentprotocol.TimeWindow{AllTime: true}, Snapshot: "current",
			Hash: hash,
		},
		Payload: payload,
		Handle:  handle,
	}
}

const contractHashA = "sha256:aaaaaaaaaaaaaaaa"
const contractHashB = "sha256:bbbbbbbbbbbbbbbb"

func newMaterializeFixture(t *testing.T) contractFixture {
	t.Helper()
	mat := materialize.NewStore()
	handleFile := filepath.Join(t.TempDir(), "artifact.json")
	if err := os.WriteFile(handleFile, []byte(`{"content":"contract"}`), 0o644); err != nil {
		t.Fatalf("write handle file: %v", err)
	}
	seed := []materialize.Row{
		matRow("dom", "T1", contractHashA, handleFile, map[string]any{"status": "ready"}),
		matRow("acp", "1007", contractHashA, "", map[string]any{"status": "partial"}),
	}
	if err := mat.Upsert(seed); err != nil {
		t.Fatalf("seed upsert: %v", err)
	}
	return contractFixture{
		name:  "materialize(bridge)",
		store: NewProtocolStore(mat),
		mat:   mat,
	}
}

// ---- 表驱动契约用例 ----

type contractCase struct {
	name string
	// bootstrapOK=false = 仅 materialize fixture 适用（写侧语义）。
	bootstrapOK bool
	run         func(t *testing.T, f contractFixture)
}

func contractCases() []contractCase {
	return []contractCase{
		{
			name:        "latest 全量视图（含全部提交行）",
			bootstrapOK: true,
			run: func(t *testing.T, f contractFixture) {
				rows, err := f.store.SnapshotView(context.Background(), SnapshotSelector{Mode: SnapshotLatest})
				if err != nil {
					t.Fatalf("SnapshotView: %v", err)
				}
				if len(rows) == 0 {
					t.Fatal("latest 须为非空全量")
				}
				for _, r := range rows {
					if err := r.Ref.Validate(); err != nil {
						t.Fatalf("契约行 ref 非法：%v", err)
					}
				}
			},
		},
		{
			name:        "exact 按行 snapshot 段过滤（代际解析归物化层）",
			bootstrapOK: true,
			run: func(t *testing.T, f contractFixture) {
				ctx := context.Background()
				all, err := f.store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
				if err != nil {
					t.Fatalf("latest: %v", err)
				}
				// 取一个实际存在的 snapshot 段值做 exact 过滤。
				var target string
				for _, r := range all {
					if r.Ref.Kind == "dom" {
						target = r.Ref.Snapshot
						break
					}
				}
				if target == "" {
					t.Fatal("fixture 无 dom 行")
				}
				exact, err := f.store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotExact, Revision: target})
				if err != nil {
					t.Fatalf("exact: %v", err)
				}
				if len(exact) == 0 {
					t.Fatal("exact 命中须非空")
				}
				for _, r := range exact {
					if r.Ref.Snapshot != target {
						t.Fatalf("exact 泄漏：snapshot=%q want %q", r.Ref.Snapshot, target)
					}
				}
			},
		},
		{
			name:        "at_or_before 显式拒绝（不静默空集）",
			bootstrapOK: true,
			run: func(t *testing.T, f contractFixture) {
				if _, err := f.store.SnapshotView(context.Background(), SnapshotSelector{Mode: SnapshotAtOrBefore, Revision: "r1"}); err == nil {
					t.Fatal("at_or_before 须显式报错")
				}
			},
		},
		{
			name:        "Resolve 按坐标解析（hash 是值不是键：旧 hash 引用命中当前行）",
			bootstrapOK: true,
			run: func(t *testing.T, f contractFixture) {
				ctx := context.Background()
				all, err := f.store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
				if err != nil {
					t.Fatalf("latest: %v", err)
				}
				var dom MaterializedRow
				found := false
				for _, r := range all {
					if r.Ref.Kind == "dom" {
						dom, found = r, true
						break
					}
				}
				if !found {
					t.Fatal("fixture 无 dom 行")
				}
				staleRef := dom.Ref
				staleRef.Hash = "sha256:cccccccccccccccc" // 非 persisted 值的旧 hash
				res, err := f.store.Resolve(ctx, staleRef)
				if err != nil {
					t.Fatalf("Resolve 旧 hash 引用按坐标须命中：%v", err)
				}
				if res.Freshness != dom.Freshness {
					t.Fatalf("Resolve freshness=%q want %q", res.Freshness, dom.Freshness)
				}
			},
		},
		{
			name:        "Resolve miss 显式报错（空结果与错误是两种信号）",
			bootstrapOK: true,
			run: func(t *testing.T, f contractFixture) {
				ref := agentprotocol.Ref{
					Kind: "dom", ScopeKind: "track", ScopeValue: "NO_SUCH",
					Window: &agentprotocol.TimeWindow{AllTime: true}, Snapshot: "current", Hash: "-",
				}
				if _, err := f.store.Resolve(context.Background(), ref); err == nil {
					t.Fatal("未知坐标 Resolve 须显式报错")
				}
			},
		},
		{
			name:        "路径句柄 ReadAll 可读且 Bytes=stat 尺寸（CAS 优先、路径兜底）",
			bootstrapOK: true,
			run: func(t *testing.T, f contractFixture) {
				ctx := context.Background()
				all, err := f.store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
				if err != nil {
					t.Fatalf("latest: %v", err)
				}
				var dom MaterializedRow
				found := false
				for _, r := range all {
					if r.Ref.Kind == "dom" {
						dom, found = r, true
						break
					}
				}
				if !found {
					t.Fatal("fixture 无 dom 行")
				}
				res, err := f.store.Resolve(ctx, dom.Ref)
				if err != nil {
					t.Fatalf("Resolve: %v", err)
				}
				if res.Handle == "" {
					t.Fatal("行须携带内容句柄（evidence:// 优先，未 CAS 化给工件路径句柄）")
				}
				data, err := res.ReadAll()
				if err != nil || len(data) == 0 {
					t.Fatalf("ReadAll: len=%d err=%v", len(data), err)
				}
				if res.Bytes != int64(len(data)) {
					t.Fatalf("Bytes=%d != len(data)=%d", res.Bytes, len(data))
				}
			},
		},
		{
			name: "marked_stale 只改状态不删行（§5.1.1；MAT-E 已验语义）",
			run: func(t *testing.T, f contractFixture) {
				ctx := context.Background()
				n, err := f.mat.MarkStale(materialize.RowMatch{Kinds: []string{"dom"}}, "contract-change-1")
				if err != nil || n == 0 {
					t.Fatalf("MarkStale: n=%d err=%v", n, err)
				}
				rows, err := f.store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
				if err != nil {
					t.Fatalf("SnapshotView: %v", err)
				}
				sawStale := false
				for _, r := range rows {
					if r.Ref.Kind == "dom" {
						sawStale = true
						if r.Freshness != FreshnessStale {
							t.Fatalf("标脏行 freshness=%q want stale（只改状态不删行）", r.Freshness)
						}
					}
				}
				if !sawStale {
					t.Fatal("标脏行不得从 SnapshotView 消失")
				}
				// 幂等重投：同 changeID 重复标脏不产生代际抖动（MAT-B 事件级重投语义）。
				if n, err := f.mat.MarkStale(materialize.RowMatch{Kinds: []string{"dom"}}, "contract-change-1"); err != nil || n != 0 {
					t.Fatalf("MarkStale 幂等重投：n=%d err=%v", n, err)
				}
			},
		},
		{
			name: "upsert 幂等（同内容重提交零可见变化）与替换换代（hash 变→replaced）",
			run: func(t *testing.T, f contractFixture) {
				ctx := context.Background()
				before, err := f.store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
				if err != nil {
					t.Fatalf("before: %v", err)
				}
				// 同内容重提交（AcousticPackages 坐标同、hash/payload 同）。
				if err := f.mat.Upsert([]materialize.Row{
					matRow("acp", "1007", contractHashA, "", map[string]any{"status": "partial"}),
				}); err != nil {
					t.Fatalf("no-op upsert: %v", err)
				}
				after, err := f.store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
				if err != nil {
					t.Fatalf("after no-op: %v", err)
				}
				if !contractRowsEqual(before, after) {
					t.Fatal("同内容重提交不得改变视图（generation 不前进的可观察面）")
				}
				// 内容变化提交：hash 变 → 行替换为新 hash。
				if err := f.mat.Upsert([]materialize.Row{
					matRow("acp", "1007", contractHashB, "", map[string]any{"status": "ready"}),
				}); err != nil {
					t.Fatalf("replacing upsert: %v", err)
				}
				replaced, err := f.store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
				if err != nil {
					t.Fatalf("after replace: %v", err)
				}
				for _, r := range replaced {
					if r.Ref.Kind == "acp" && r.Ref.Hash != contractHashB {
						t.Fatalf("替换后行 hash=%q want %q", r.Ref.Hash, contractHashB)
					}
				}
			},
		},
		{
			name: "句柄保留（MAT-E：内容身份同源的无句柄行不冲掉已回填句柄）",
			run: func(t *testing.T, f contractFixture) {
				ctx := context.Background()
				// 重算轮产物不带句柄、内容身份同源（hash+payload 同值）。
				if err := f.mat.Upsert([]materialize.Row{
					matRow("dom", "T1", contractHashA, "", map[string]any{"status": "ready"}),
				}); err != nil {
					t.Fatalf("shadow upsert: %v", err)
				}
				rows, err := f.store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
				if err != nil {
					t.Fatalf("SnapshotView: %v", err)
				}
				for _, r := range rows {
					if r.Ref.Kind != "dom" {
						continue
					}
					res, err := f.store.Resolve(ctx, r.Ref)
					if err != nil {
						t.Fatalf("Resolve: %v", err)
					}
					if res.Handle == "" {
						t.Fatal("无句柄重算行不得冲掉读端已回填句柄（MAT-E 句柄保留）")
					}
				}
			},
		},
		{
			name: "Subscribe 变更流按提交序投递（added 事件可见）",
			run: func(t *testing.T, f contractFixture) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ch, unsubscribe, err := f.store.Subscribe(ctx)
				if err != nil {
					t.Fatalf("Subscribe: %v", err)
				}
				defer unsubscribe()
				if err := f.mat.Upsert([]materialize.Row{
					matRow("rlm", "project", contractHashA, "", map[string]any{"integrated_lufs": -14.0}),
				}); err != nil {
					t.Fatalf("upsert: %v", err)
				}
				select {
				case ev := <-ch:
					if ev.Op != MaterializedAdded || ev.Row.Ref.Kind != "rlm" {
						t.Fatalf("事件 op=%q kind=%q，期望 added/rlm", ev.Op, ev.Row.Ref.Kind)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("upsert 后 2s 内须收到 added 事件")
				}
			},
		},
	}
}

func TestMaterializedStoreContract(t *testing.T) {
	fixtures := []struct {
		name  string
		build func(t *testing.T) contractFixture
	}{
		{"bootstrap", newBootstrapFixture},
		{"materialize-bridge", newMaterializeFixture},
	}
	for _, fixture := range fixtures {
		for _, tc := range contractCases() {
			fixture, tc := fixture, tc
			t.Run(fixture.name+"/"+tc.name, func(t *testing.T) {
				f := fixture.build(t)
				if f.mat == nil && !tc.bootstrapOK {
					t.Skip("写侧语义用例：仅 materialize fixture 适用（bootstrap 只读扫描无写侧原语）")
				}
				tc.run(t, f)
			})
		}
	}
}

func contractRowsEqual(a, b []MaterializedRow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if canon(a[i].Ref) != canon(b[i].Ref) || a[i].Freshness != b[i].Freshness {
			return false
		}
		if len(a[i].Payload) != len(b[i].Payload) {
			return false
		}
		for k, v := range a[i].Payload {
			if b[i].Payload[k] != v {
				return false
			}
		}
	}
	return true
}
