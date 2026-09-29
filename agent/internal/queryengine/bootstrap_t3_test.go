package queryengine

// bootstrap_t3_test.go — T3 真实 fixture 双轨（QUERY_ENGINE §6.2 T3，IMPL-B）：
//
//	真实产物轨：artifacts/ 下既有合成工程运行产物（obs JSON/声学包/桥快照），
//	  只读复制到 t.TempDir() 后扫描（§10 防污染：源树零写入；fixture 为盘上
//	  专属工件，非全检出库——缺席即 skip，不伪造存在性）；
//	合成轨：确定性 fixture，常绿。
//
// 断言形态：引擎在真实产物上不 panic、行数可解释（逐 kind 与从产物独立
// 重数的期望对账——期望值不硬编码，防 fixture 演化致脆）。
//
// 另锁 v0 轮询 Subscribe（§5.2：mtime/目录列举模拟，秒级粒度）：新增产物
// →added、内容替换→replaced、产物消失→marked_stale（不删行语义的事件面：
// 消失行的可见性经事件保留，整表重建后如实反映盘面）。

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- 合成 fixture ----

const (
	bootstrapSessionUUID = "vitproj_bootstrapsynthetic"
	bootstrapObsFull     = "obs_synthetic_full"
	bootstrapObsFxmOnly  = "obs_synthetic_fxm"
)

type syntheticBootstrapTree struct {
	agentRoot      string // <tmp>/agent/.vit_agent
	acousticPath   string // <tmp>/acoustic/acoustic_package_status.json
	comEvidenceDir string // <tmp>/com_evidence
}

func newSyntheticBootstrapTree(t *testing.T) syntheticBootstrapTree {
	t.Helper()
	base := t.TempDir()
	tree := syntheticBootstrapTree{
		agentRoot:      filepath.Join(base, "agent", ".vit_agent"),
		acousticPath:   filepath.Join(base, "acoustic", "acoustic_package_status.json"),
		comEvidenceDir: filepath.Join(base, "com_evidence"),
	}
	for _, dir := range []string{
		filepath.Join(tree.agentRoot, bootstrapSessionUUID, "observations"),
		filepath.Join(tree.agentRoot, bootstrapSessionUUID, "mixboard"),
		filepath.Dir(tree.acousticPath),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	return tree
}

// writeObsTicket 落一张合成观察票（dom/fxm/com 投影按开关携带）。
func writeObsTicket(t *testing.T, tree syntheticBootstrapTree, obsID string, withDOM, withFXM, withCOM bool) string {
	t.Helper()
	packet := map[string]any{
		"schema_version": "mixboard_observation.v1",
		"observation_id": obsID,
		"mix_session_id": "mix_bootstrap",
		"status":         "ready",
		"target_ref":     map[string]any{"kind": "track", "id": "T1", "label": "synthetic-lead"},
		"created_at":     "2026-09-29T00:00:00Z",
	}
	if withDOM {
		packet["dom_projection"] = map[string]any{
			"schema_version": "dom_projection.v1", "status": "ready", "mode": "source_only",
		}
	}
	if withFXM {
		packet["fxm_projection"] = map[string]any{
			// fxm/com 行化门槛=projection.SchemaVersion 非空（FXMRowsFromObservation
			// 同款判定），合成票必须带 schema_version。
			"schema_version": "fxm_measurement.v1", "status": "ready",
			"target_ref": map[string]any{"kind": "track", "id": "T1"},
			"effect_delta": map[string]any{
				"delta_lufs": -1.5, "correlation": 0.82,
			},
		}
	}
	if withCOM {
		packet["com_projection"] = map[string]any{
			"schema_version": "com_projection.v1", "status": "ready", "mode": "source_only",
			"target_ref": map[string]any{"kind": "track", "id": "T1"},
		}
	}
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal obs ticket: %v", err)
	}
	path := filepath.Join(tree.agentRoot, bootstrapSessionUUID, "observations", obsID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write obs ticket: %v", err)
	}
	return path
}

// writeAcousticStore 落合成声学包 store（每包行带 track 坐标与状态）。
func writeAcousticStore(t *testing.T, tree syntheticBootstrapTree, pkgs ...synthAcousticPkg) {
	t.Helper()
	packages := make([]map[string]any, 0, len(pkgs))
	for _, pkg := range pkgs {
		packages = append(packages, map[string]any{
			"schema_version": "acoustic_package_status.v0", "status": pkg.Status,
			"track_id": pkg.TrackID, "source_revision": "sr_" + pkg.TrackID,
			"package_layers": map[string]any{
				"band_energy":     map[string]any{"status": "ready"},
				"stereo_relation": map[string]any{"status": "building"},
			},
		})
	}
	writeJSONFile(t, tree.acousticPath, map[string]any{
		"schema_version": "acoustic_package_status.v0",
		"updated_at":     "2026-09-29T00:00:00Z",
		"packages":       packages,
	})
}

type synthAcousticPkg struct {
	TrackID string
	Status  string
}

// writeComPair 落一只合成 COM paired 工件。
func writeComPair(t *testing.T, tree syntheticBootstrapTree, pairID, trackID string) {
	t.Helper()
	dir := filepath.Join(tree.comEvidenceDir, pairID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir pair dir: %v", err)
	}
	writeJSONFile(t, filepath.Join(dir, pairID+".json"), map[string]any{
		"schema_version": "dad.compressor_dual_tap_evidence.v1",
		"pair_id":        pairID,
		"processor_scope": map[string]any{
			"track_id": trackID, "plugin_instance_id": "1040",
			"topology_class": "threshold_driven", "support_class": "single_band_broadband",
		},
	})
}

// writeFeatureSnapshot 落合成桥快照：waveform/band 轨集 + 每轨一条 probe。
func writeFeatureSnapshot(t *testing.T, tree syntheticBootstrapTree, waveformTracks, bandTracks, probeTracks []string) {
	t.Helper()
	envelopes := make([]map[string]any, 0, len(waveformTracks))
	for _, id := range waveformTracks {
		envelopes = append(envelopes, map[string]any{
			"track_id": id, "status": "ok", "duration_seconds": 12.5,
		})
	}
	bands := make([]map[string]any, 0, len(bandTracks))
	for _, id := range bandTracks {
		bands = append(bands, map[string]any{
			"track_id": id, "status": "ok", "summary_db": -18.0,
		})
	}
	probes := make([]map[string]any, 0, len(probeTracks))
	for _, id := range probeTracks {
		probes = append(probes, map[string]any{
			"track_id": id, "command": "l2_render_probe", "status": "ok",
			"balance_state": "balanced", "correlation_estimate": 0.4,
			"render_revision": "rr_" + id,
			"bands":           []map[string]any{{"band": "low", "peak_db": -12.0}, {"band": "high", "peak_db": -20.0}},
		})
	}
	writeJSONFile(t, filepath.Join(tree.agentRoot, bootstrapSessionUUID, "mixboard", "mixboard_feature_snapshot.json"), map[string]any{
		"schema_version":           "mixboard_feature_snapshot.v1",
		"track_waveform_envelopes": envelopes,
		"band_energy_summaries":    bands,
		"l2_render_probes":         probes,
	})
}

func writeJSONFile(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func bootstrapStoreOf(tree syntheticBootstrapTree) *BootstrapStore {
	return NewBootstrapStore(BootstrapConfig{
		AgentRoot:      tree.agentRoot,
		AcousticStore:  tree.acousticPath,
		COMEvidenceDir: tree.comEvidenceDir,
		PollInterval:   15 * time.Millisecond,
	})
}

func countRowsByKind(rows []MaterializedRow) map[string]int {
	out := map[string]int{}
	for _, r := range rows {
		out[r.Ref.Kind]++
	}
	return out
}

// TestT3SyntheticTrack 双轨之合成轨：四产物族行数逐 kind 对账 + 引擎全链不
// panic + Resolve 句柄可读。
func TestT3SyntheticTrack(t *testing.T) {
	tree := newSyntheticBootstrapTree(t)
	writeObsTicket(t, tree, bootstrapObsFull, true, true, true)
	writeObsTicket(t, tree, bootstrapObsFxmOnly, false, true, false)
	writeAcousticStore(t, tree, synthAcousticPkg{"1007", "partial"}, synthAcousticPkg{"1012", "partial"})
	writeComPair(t, tree, "com2_synthetic01", "1012")
	writeFeatureSnapshot(t, tree, []string{"T1", "T2"}, []string{"T2"}, []string{"T1", "T2"})

	store := bootstrapStoreOf(tree)
	ctx := context.Background()
	rows, err := store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
	if err != nil {
		t.Fatalf("SnapshotView: %v", err)
	}
	got := countRowsByKind(rows)
	// 期望独立重数（不读 scanner 逻辑）：dom=1（仅 full 票）、fxm=2、com=1、
	// acp=2（两包行）、dad.compressor_dual_tap=1、dad.l3=2（T1/T2，waveform∪band）、
	// dad.l2_render_probe=2（T1/T2 probe）。
	want := map[string]int{
		"dom":                     1,
		"fxm":                     2,
		"com":                     1,
		"acp":                     2,
		"dad.compressor_dual_tap": 1,
		"dad.l3":                  2,
		"dad.l2_render_probe":     2,
	}
	if !mapsEqual(got, want) {
		t.Fatalf("行数不可解释：got %v want %v", got, want)
	}
	for _, r := range rows {
		if r.Freshness != FreshnessMaterialReuse {
			t.Fatalf("bootstrap 行 freshness 须为 material_reuse（存量行，读侧零推断），got %q on %s", r.Freshness, canon(r.Ref))
		}
		if err := r.Ref.Validate(); err != nil {
			t.Fatalf("行 ref 非法：%v（%+v）", err, r.Ref)
		}
	}

	// 引擎全链：Sync 建表 + 各 kind 查询不 panic、结果与行数一致。
	engine := NewEngine(store, nil)
	if err := engine.Sync(ctx); err != nil {
		t.Fatalf("engine sync: %v", err)
	}
	for kind, want := range want {
		res, err := engine.Query(ctx, RefQuery{Kinds: KindPredicate{Kinds: []string{kind}}})
		if err != nil {
			t.Fatalf("query kind=%s: %v", kind, err)
		}
		if res.TotalMatches != want {
			t.Fatalf("query kind=%s: total=%d want %d", kind, res.TotalMatches, want)
		}
	}

	// Resolve：坐标命中（hash 是值不是键——任意合法 hash 仍命中坐标行），
	// 路径句柄 ReadAll 可读且 Bytes=文件尺寸。
	fxmRows := filterKind(rows, "fxm")
	if len(fxmRows) != 2 {
		t.Fatalf("fxm rows = %d", len(fxmRows))
	}
	target := fxmRows[0].Ref
	target.Hash = "sha256:0000000000000000" // 旧 hash 引用按坐标解析
	resolved, err := store.Resolve(ctx, target)
	if err != nil {
		t.Fatalf("Resolve by coordinate: %v", err)
	}
	if resolved.Freshness != FreshnessMaterialReuse {
		t.Fatalf("Resolve freshness = %q", resolved.Freshness)
	}
	if !strings.HasPrefix(resolved.Handle, tree.agentRoot) && !strings.HasPrefix(filepath.ToSlash(resolved.Handle), filepath.ToSlash(tree.agentRoot)) {
		t.Fatalf("Resolve handle = %q，须为观察票工件路径", resolved.Handle)
	}
	data, err := resolved.ReadAll()
	if err != nil || len(data) == 0 {
		t.Fatalf("Resolve ReadAll: data=%d err=%v", len(data), err)
	}
	if resolved.Bytes != int64(len(data)) {
		t.Fatalf("Resolve Bytes=%d != len(data)=%d", resolved.Bytes, len(data))
	}

	// Resolve miss：未知坐标显式报错（不静默空）。
	missRef := target
	missRef.ScopeValue = "NO_SUCH_TRACK"
	missRef.Hash = "-"
	if _, err := store.Resolve(ctx, missRef); err == nil {
		t.Fatal("Resolve 未知坐标须显式报错")
	}
}

// TestT3SyntheticPollSubscribe v0 轮询 Subscribe：新增→added、替换→replaced、
// 消失→marked_stale；cancel 关闭通道。
func TestT3SyntheticPollSubscribe(t *testing.T) {
	tree := newSyntheticBootstrapTree(t)
	writeObsTicket(t, tree, bootstrapObsFull, true, true, true)
	writeAcousticStore(t, tree, synthAcousticPkg{"1007", "partial"})

	store := bootstrapStoreOf(tree)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, unsubscribe, err := store.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// 变更 1：新增 fxm-only 票 → fxm added。
	writeObsTicket(t, tree, bootstrapObsFxmOnly, false, true, false)
	// 变更 2：1007 包内容替换（status partial→ready）→ acp replaced；1012 新增 → acp added。
	writeAcousticStore(t, tree, synthAcousticPkg{"1007", "ready"}, synthAcousticPkg{"1012", "partial"})

	collector := make(chan MaterializedChange, 64)
	go func() {
		for ev := range ch {
			collector <- ev
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	ops := map[MaterializedOp]int{}
	for time.Now().Before(deadline) {
		select {
		case ev := <-collector:
			ops[ev.Op]++
		case <-time.After(50 * time.Millisecond):
		}
		if ops[MaterializedAdded] >= 2 && ops[MaterializedReplaced] >= 1 {
			break // 到齐即止（轮询粒度不锁具体时序）
		}
	}
	if ops[MaterializedAdded] < 2 || ops[MaterializedReplaced] < 1 {
		t.Fatalf("轮询事件不足：added=%d replaced=%d（期望 ≥2/≥1）", ops[MaterializedAdded], ops[MaterializedReplaced])
	}

	// 变更 3：删票 → 该行 marked_stale（消失行的可见性经事件保留）。
	if err := os.Remove(filepath.Join(tree.agentRoot, bootstrapSessionUUID, "observations", bootstrapObsFxmOnly+".json")); err != nil {
		t.Fatalf("remove obs: %v", err)
	}
	deadline = time.Now().Add(5 * time.Second)
	sawStale := false
	for time.Now().Before(deadline) {
		select {
		case ev := <-collector:
			if ev.Op == MaterializedMarkedStale && ev.Row.Ref.Kind == "fxm" && ev.Row.Freshness == FreshnessStale {
				sawStale = true
			}
		case <-time.After(50 * time.Millisecond):
		}
		if sawStale {
			break
		}
	}
	if !sawStale {
		t.Fatal("产物消失须发 marked_stale（freshness=stale，不删行语义的事件面）")
	}

	unsubscribe()
	deadline = time.Now().Add(2 * time.Second)
	closed := false
	for time.Now().Before(deadline) {
		select {
		case _, ok := <-ch:
			if !ok {
				closed = true
			}
		case <-time.After(50 * time.Millisecond):
		}
		if closed {
			break
		}
	}
	if !closed {
		t.Fatal("cancel 后通道须关闭")
	}
}

// TestT3BootstrapSelectorModes selector 三态：latest 全量、exact 按行 snapshot
// 段过滤、at_or_before 显式拒绝（OQ-2）。
func TestT3BootstrapSelectorModes(t *testing.T) {
	tree := newSyntheticBootstrapTree(t)
	writeObsTicket(t, tree, bootstrapObsFull, true, true, true)
	writeObsTicket(t, tree, bootstrapObsFxmOnly, false, true, false)
	store := bootstrapStoreOf(tree)
	ctx := context.Background()

	latest, err := store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	exact, err := store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotExact, Revision: bootstrapObsFull})
	if err != nil {
		t.Fatalf("exact: %v", err)
	}
	if got := len(filterKind(latest, "fxm")); got != 2 {
		t.Fatalf("latest fxm = %d", got)
	}
	if got := len(filterKind(exact, "fxm")); got != 1 {
		t.Fatalf("exact fxm = %d（仅 full 票的 fxm 行 snapshot=obs id）", got)
	}
	for _, r := range exact {
		if r.Ref.Snapshot != bootstrapObsFull {
			t.Fatalf("exact 过滤泄漏：snapshot=%q", r.Ref.Snapshot)
		}
	}
	if _, err := store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotAtOrBefore, Revision: "r1"}); err == nil {
		t.Fatal("at_or_before 须显式拒绝（OQ-2）")
	}
}

// ---- 真实产物轨 ----

// realFixtureRoot 定位盘上合成工程运行产物（artifacts/ 为盘上专属工件目录，
// 未入 git；缺席即 skip）。B5/B6 = 论文两层验证用合成工程的既有运行产物。
func realFixtureRoot(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"../../..", // agent/internal/queryengine → 仓库根
	}
	for _, rel := range candidates {
		root, err := filepath.Abs(rel)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "artifacts", "B5", "20260911_102712", "project")); err == nil {
			return root
		}
	}
	t.Skipf("真实产物 fixture 缺席（artifacts/ 为盘上专属目录）：skip 真实轨")
	return ""
}

// copyTree 只读复制 src 下匹配谓词的文件到 dst（§10 防污染：源树零写入）。
func copyTree(t *testing.T, src, dst string, keep func(relPath string) bool) int {
	t.Helper()
	copied := 0
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !keep(rel) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		copied++
		return nil
	})
	if err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	return copied
}

// recountRealExpectations 从复制的产物树独立重数期望行数（不复用 scanner 逻辑）。
func recountRealExpectations(t *testing.T, tmp string) (map[string]int, syntheticBootstrapTree) {
	t.Helper()
	tree := syntheticBootstrapTree{
		agentRoot:      filepath.Join(tmp, "project", ".vit_agent"),
		acousticPath:   "",
		comEvidenceDir: filepath.Join(tmp, "com_evidence"),
	}
	// 声学包路径发现：.vit_derived/<uuid>/acoustic_package_status.json。
	_ = filepath.WalkDir(tmp, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "acoustic_package_status.json" {
			tree.acousticPath = path
		}
		return nil
	})

	want := map[string]int{}
	_ = filepath.WalkDir(tree.agentRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".json") && filepath.Base(filepath.Dir(path)) == "observations":
			var packet map[string]any
			data, err := os.ReadFile(path)
			if err != nil || json.Unmarshal(data, &packet) != nil {
				return nil
			}
			for kind, key := range map[string]string{
				"dom": "dom_projection", "fxm": "fxm_projection", "com": "com_projection",
			} {
				// 行化门槛对齐 materialize 行化函数：fxm/com 须 projection.schema_version
				// 非空；dom 为 schema_version/status 任一非空。
				raw, ok := packet[key]
				if !ok || raw == nil {
					continue
				}
				proj, _ := raw.(map[string]any)
				sv := firstRowString(proj, "schema_version")
				status := firstRowString(proj, "status")
				if kind == "dom" && sv == "" && status == "" {
					continue
				}
				if kind != "dom" && sv == "" {
					continue
				}
				want[kind]++
			}
		case d.Name() == "mixboard_feature_snapshot.json":
			var snap struct {
				TrackWaveformEnvelopes  []map[string]any `json:"track_waveform_envelopes"`
				BandEnergySummaries     []map[string]any `json:"band_energy_summaries"`
				StereoRelationSummaries []map[string]any `json:"stereo_relation_summaries"`
				LoudnessSummaries       []map[string]any `json:"loudness_summaries"`
				L2RenderProbes          []map[string]any `json:"l2_render_probes"`
			}
			data, err := os.ReadFile(path)
			if err != nil || json.Unmarshal(data, &snap) != nil {
				return nil
			}
			l3Tracks, probeTracks := map[string]bool{}, map[string]bool{}
			for _, section := range [][]map[string]any{snap.TrackWaveformEnvelopes, snap.BandEnergySummaries, snap.StereoRelationSummaries, snap.LoudnessSummaries} {
				for _, row := range section {
					if id := firstRowString(row, "track_id", "source_track_id"); id != "" {
						l3Tracks[id] = true
					}
				}
			}
			for _, row := range snap.L2RenderProbes {
				if id := firstRowString(row, "track_id", "source_track_id"); id != "" {
					probeTracks[id] = true
				}
			}
			want["dad.l3"] += len(l3Tracks)
			want["dad.l2_render_probe"] += len(probeTracks)
		}
		return nil
	})
	if tree.acousticPath != "" {
		data, err := os.ReadFile(tree.acousticPath)
		if err == nil {
			var snap struct {
				Packages []struct {
					TrackID string `json:"track_id"`
				} `json:"packages"`
			}
			if json.Unmarshal(data, &snap) == nil {
				for _, pkg := range snap.Packages {
					if strings.TrimSpace(pkg.TrackID) != "" {
						want["acp"]++
					}
				}
			}
		}
	}
	return want, tree
}

func firstRowString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok && v != nil {
			if s := strings.TrimSpace(fmt.Sprint(v)); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

// TestT3RealFixtureTrack 双轨之真实产物轨：B5 合成工程运行产物只读复制到
// t.TempDir()，引擎扫描不 panic、逐 kind 行数与独立重数对账。
func TestT3RealFixtureTrack(t *testing.T) {
	root := realFixtureRoot(t)
	src := filepath.Join(root, "artifacts", "B5", "20260911_102712", "project")
	tmp := t.TempDir()
	copied := copyTree(t, src, filepath.Join(tmp, "project"), func(rel string) bool {
		return strings.HasPrefix(rel, ".vit_agent/") || strings.HasPrefix(rel, ".vit_derived/")
	})
	if copied == 0 {
		t.Skipf("复制 0 文件（产物树形态变化）：skip 真实轨")
	}
	// com_evidence 家族在真实轨不接（活工作树不入测试）；合成轨已覆盖。
	if err := os.MkdirAll(filepath.Join(tmp, "com_evidence"), 0o755); err != nil {
		t.Fatalf("mkdir com_evidence: %v", err)
	}

	want, tree := recountRealExpectations(t, tmp)
	store := NewBootstrapStore(BootstrapConfig{
		AgentRoot:      tree.agentRoot,
		AcousticStore:  tree.acousticPath,
		COMEvidenceDir: tree.comEvidenceDir,
	})
	ctx := context.Background()
	rows, err := store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
	if err != nil {
		t.Fatalf("真实产物 SnapshotView: %v", err)
	}
	got := countRowsByKind(rows)
	if !mapsEqual(got, want) {
		t.Fatalf("真实产物行数不可解释：got %v want %v（copied=%d）", got, want, copied)
	}
	if len(rows) == 0 {
		t.Fatal("真实产物轨须扫出非零行集")
	}

	engine := NewEngine(store, nil)
	if err := engine.Sync(ctx); err != nil {
		t.Fatalf("engine sync: %v", err)
	}
	for kind := range want {
		res, err := engine.Query(ctx, RefQuery{Kinds: KindPredicate{Kinds: []string{kind}}})
		if err != nil {
			t.Fatalf("真实产物 query kind=%s: %v", kind, err)
		}
		if res.TotalMatches != want[kind] {
			t.Fatalf("真实产物 query kind=%s: total=%d want=%d", kind, res.TotalMatches, want[kind])
		}
	}
}

func filterKind(rows []MaterializedRow, kind string) []MaterializedRow {
	out := []MaterializedRow{}
	for _, r := range rows {
		if r.Ref.Kind == kind {
			out = append(out, r)
		}
	}
	return out
}

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
