package materialize

// recompute_test.go — MAT-C 红先行（G2-B 对拍+三态 flag+影子对账；本文件先于
// 实现落库，未定义符号=编译红，红证见回执）。
//
// 规格权威：docs/MATERIALIZATION_V1_DESIGN.md §5.3（G2-B：增量轨迹与全量轨迹
// 收敛态一致）、§7.1（三态 flag+kind 白名单+影子对账 ShadowDivergences）、
// §2.2（lazy 重算三触发点之 observe 回填语义的前置：RecomputeLazy）、§5.1
// （断言证据三抓手）。合成工程口径对齐 QUERY_ENGINE §6.1（≥2 轨+插件链、
// 种子固定、确定性）。
//
// 对拍可信度边界（§5.3 原文）：FullRebuild 与增量轨共享同一 Build 函数与输入
// 读取路径——对拍检测的是传播漏标（dirty 集合计算错误），不是 Build 自身错误
// （后者归各投影单测）。此边界写死在本注释，禁改 oracle 凑绿（AGENTS §10）。
//
// 行坐标约定（MAT-C 裁定，随实现申报）：precomputable kind（tom/acp/dom）行
// snapshot 段=稳定实例 token "current"（同坐标 in-place 换代；latest 视图的
// 换代由 generation 机制承载，OQ-1 v1 只保当前代）；registered kind（fxm/com）
// 行 snapshot=observation_id（历史产物按观察累积，exact 可查）。

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/fxm"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/shadow"
)

// ---------------------------------------------------------------------------
// 合成工程（QUERY_ENGINE §6.1 生成器同源口径：≥2 轨+插件链、确定性、可重放）
// ---------------------------------------------------------------------------

// matCSynthProject 是事件驱动可演化的合成工程状态：shadow 工程快照（tom 输入
// 域）+特征快照行（dom 输入域）+acp 包行（acp 输入域）。事件先落状态（模拟
// shadow 应用 delta / 遥测写 snapshot），再通知物化层（MAT-B 三入口）。
type matCSynthProject struct {
	revision string
	tracks   map[string]map[string]any
	order    []string
	features map[string]map[string]any
	packages []acousticpackage.Status
	seq      int
}

func newMatCSynthProject() *matCSynthProject {
	p := &matCSynthProject{
		revision: "r1",
		tracks: map[string]map[string]any{
			"T3": {"track_id": "T3", "track_name": "lead vox", "gain_db": -6.0,
				"plugins": []any{map[string]any{"uid": "comp1", "name": "Compressor", "bypassed": false}}},
			"T7": {"track_id": "T7", "track_name": "drums", "gain_db": -3.0,
				"plugins": []any{map[string]any{"uid": "eq1", "name": "EQ", "bypassed": false}}},
		},
		order:    []string{"T3", "T7"},
		features: map[string]map[string]any{},
		packages: []acousticpackage.Status{},
	}
	p.features["T3"] = matCWaveformRow("T3", "sr1", -20.5, -3.2)
	p.packages = append(p.packages, matCPackage("T3", "sr1"))
	return p
}

func matCWaveformRow(trackID, sourceRevision string, rms, peak float64) map[string]any {
	return map[string]any{
		"status": "ready", "track_id": trackID, "source_revision": sourceRevision,
		"clip_revision": "cr1", "rms_dbfs": rms, "peak_dbfs": peak, "headroom_db": -peak,
		"duration_seconds": 12.0, "sample_rate": 48000.0, "channel_count": 2,
		"analyzed_sample_count": 576000, "window_ms": 200.0, "hop_ms": 100.0,
		"time_segments": []any{
			map[string]any{"start_seconds": 0.0, "end_seconds": 6.0, "rms_dbfs": rms, "peak_dbfs": peak},
			map[string]any{"start_seconds": 6.0, "end_seconds": 12.0, "rms_dbfs": rms + 1.5, "peak_dbfs": peak + 0.5},
		},
	}
}

func matCPackage(trackID, sourceRevision string) acousticpackage.Status {
	return acousticpackage.Status{
		SchemaVersion: acousticpackage.SchemaVersion, Status: acousticpackage.StatusReady,
		TrackID: trackID, SourceRevision: sourceRevision, ClipRevision: "cr1",
		RenderRevision: "rr1", DurationSec: 12.0,
		PackageLayers: map[string]acousticpackage.LayerStatus{
			"band_energy": {Status: acousticpackage.StatusReady,
				Features: map[string]acousticpackage.FeatureStatus{
					"band_energy_summary": {Status: acousticpackage.StatusReady}}},
		},
	}
}

func (p *matCSynthProject) nextChangeID() string {
	p.seq++
	return fmt.Sprintf("matc_change_%06d", p.seq)
}

// DepInputs 把合成工程当前态打包成适配器输入（两轨共用同一份——对拍前提）。
func (p *matCSynthProject) DepInputs() DepInputs {
	trackRows := make([]any, 0, len(p.order))
	for _, id := range p.order {
		trackRows = append(trackRows, p.tracks[id])
	}
	featureSnapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
	}
	trackWaves := make([]any, 0, len(p.features))
	for _, id := range p.order {
		if row, ok := p.features[id]; ok {
			trackWaves = append(trackWaves, row)
		}
	}
	featureSnapshot["track_waveform_envelopes"] = trackWaves
	return DepInputs{
		ProjectState:     map[string]any{"project_revision": p.revision, "tracks": trackRows},
		FeatureSnapshot:  featureSnapshot,
		AcousticPackages: append([]acousticpackage.Status(nil), p.packages...),
		ProjectRevision:  p.revision,
	}
}

// 事件：track.level 变更（invalidate，scope 可收窄到 track）。
func (p *matCSynthProject) applyGainChange(trackID string, gain float64) shadow.ChangeReceipt {
	if track, ok := p.tracks[trackID]; ok {
		track["gain_db"] = gain
	}
	p.revision = p.revision + "g"
	return shadow.ChangeReceipt{
		ChangeID:        p.nextChangeID(),
		AffectedScopes:  []string{"track.level"},
		ChangedEntities: []shadow.ChangeEntity{{Kind: "track", ID: trackID}},
	}
}

// 事件：project.structure 变更（加轨；invalidate 全曲域）。
func (p *matCSynthProject) applyAddTrack(trackID, name string) shadow.ChangeReceipt {
	if _, exists := p.tracks[trackID]; !exists {
		p.tracks[trackID] = map[string]any{"track_id": trackID, "track_name": name, "gain_db": 0.0,
			"plugins": []any{}}
		p.order = append(p.order, trackID)
	}
	p.revision = p.revision + "s"
	return shadow.ChangeReceipt{
		ChangeID:       p.nextChangeID(),
		AffectedScopes: []string{"project.structure"},
	}
}

// 事件：波形特征到达（arrive；#4 词表，scope 收窄到 track）。
func (p *matCSynthProject) applyWaveformArrival(trackID, requestID string) map[string]any {
	p.features[trackID] = matCWaveformRow(trackID, "sr2", -19.0, -2.8)
	return map[string]any{
		"command": "audio_feature_data_ready", "feature_type": "waveform_envelope",
		"track_id": trackID, "request_id": requestID,
	}
}

// 事件：L3 声学到达（arrive；#6 词表；同时推进 acp 包行——acp 输入域变化）。
func (p *matCSynthProject) applyL3Arrival(trackID, requestID string) map[string]any {
	p.packages = append(p.packages, matCPackage(trackID, "sr2"))
	return map[string]any{
		"command": "audio_feature_data_ready", "feature_type": "band_energy_summary",
		"track_id": trackID, "request_id": requestID,
	}
}

// 事件：render 终态（批 3 全 kind 保守标脏）。
func (p *matCSynthProject) applyRenderJob(jobID string) (string, string, string) {
	p.revision = p.revision + "r"
	return jobID, "render_done", "out.wav"
}

// 事件：未知域收据（宁多勿漏兜底）。
func (p *matCSynthProject) applyUnknownScope() shadow.ChangeReceipt {
	return shadow.ChangeReceipt{ChangeID: p.nextChangeID(), AffectedScopes: []string{"some.unknown.domain"}}
}

// ---------------------------------------------------------------------------
// 对拍脚手架
// ---------------------------------------------------------------------------

// matCEventStep 是一步事件：先 mutate 合成工程，再把通知投给两轨 Store。
type matCEventStep struct {
	name     string
	apply    func(p *matCSynthProject) any // 返回通知载荷（receipt / telemetry / render）
	deliver  func(s *Store, payload any)   // 按通知类型投递
	skipWhen func(p *matCSynthProject) bool
}

func matCDeliverReceipt(s *Store, payload any) {
	s.HandleReceipt(payload.(shadow.ChangeReceipt))
}

func matCDeliverTelemetry(s *Store, payload any) {
	s.HandleFeatureArrival(payload.(map[string]any))
}

func matCDeliverRender(s *Store, payload any) {
	job := payload.([3]string)
	s.HandleRenderJob(job[0], job[1], job[2])
}

func matCStandardScript() []matCEventStep {
	return []matCEventStep{
		{"gain_change_T3", func(p *matCSynthProject) any { return p.applyGainChange("T3", -9.0) }, matCDeliverReceipt, nil},
		{"waveform_arrival_T7", func(p *matCSynthProject) any { return p.applyWaveformArrival("T7", "wf-1") }, matCDeliverTelemetry, nil},
		{"add_track_T9", func(p *matCSynthProject) any { return p.applyAddTrack("T9", "bass") }, matCDeliverReceipt, nil},
		{"l3_arrival_T7", func(p *matCSynthProject) any { return p.applyL3Arrival("T7", "l3-1") }, matCDeliverTelemetry, nil},
		{"gain_change_T7", func(p *matCSynthProject) any { return p.applyGainChange("T7", -4.5) }, matCDeliverReceipt, nil},
		{"render_job", func(p *matCSynthProject) any { return p.applyRenderJob("job-1") }, matCDeliverRender, nil},
		{"unknown_scope", func(p *matCSynthProject) any { return p.applyUnknownScope() }, matCDeliverReceipt, nil},
		{"gain_change_T9", func(p *matCSynthProject) any { return p.applyGainChange("T9", -2.0) }, matCDeliverReceipt, nil},
	}
}

// newMatCDualStore 构造同表 B、同适配器注册的双 Store（增量轨 I 与全量轨 F）。
func newMatCDualStore(t *testing.T) (incremental, full *Store) {
	t.Helper()
	mustRegister := func(s *Store) {
		if err := RegisterDefaultAdapters(s, nil); err != nil {
			t.Fatalf("RegisterDefaultAdapters: %v", err)
		}
	}
	incremental, full = NewStore(), NewStore()
	mustRegister(incremental)
	mustRegister(full)
	return incremental, full
}

// runIncrementalStep 增量轨语义（§2.2 observe 读端回填的前置形态）：事件投递后
// 只重算"无行或含 stale 行"的 precomputable kind（lazy）。
func runIncrementalStep(t *testing.T, s *Store, deps DepInputs) {
	t.Helper()
	if err := s.RecomputeLazy(deps); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
}

func runFullRebuildStep(t *testing.T, s *Store, deps DepInputs) {
	t.Helper()
	if err := s.FullRebuild(deps); err != nil {
		t.Fatalf("FullRebuild: %v", err)
	}
}

// assertViewsEqual 逐行 deep-equal（ref+freshness+payload+handle 全字段）。
func assertViewsEqual(t *testing.T, incremental, full *Store, phase string) {
	t.Helper()
	ctx := context.Background()
	viewI, err := incremental.SnapshotView(ctx, agentprotocol.SnapshotSelector{Mode: agentprotocol.SnapshotModeLatest})
	if err != nil {
		t.Fatalf("SnapshotView(I): %v", err)
	}
	viewF, err := full.SnapshotView(ctx, agentprotocol.SnapshotSelector{Mode: agentprotocol.SnapshotModeLatest})
	if err != nil {
		t.Fatalf("SnapshotView(F): %v", err)
	}
	if len(viewI) != len(viewF) {
		t.Fatalf("%s：两轨行数不一致 I=%d F=%d\nI=%s\nF=%s", phase, len(viewI), len(viewF),
			matCFormatRows(viewI), matCFormatRows(viewF))
	}
	for i := range viewI {
		if !reflect.DeepEqual(viewI[i], viewF[i]) {
			t.Fatalf("%s：第 %d 行分歧（传播漏标或行化不稳定）\nI=%+v\nF=%+v", phase, i, viewI[i], viewF[i])
		}
	}
}

func matCFormatRows(rows []agentprotocol.MaterializedRow) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, fmt.Sprintf("%s/%s@%s#%s[%s]", row.Ref.Kind, row.Ref.ScopeValue, row.Ref.Snapshot, row.Ref.Hash, row.Freshness))
	}
	return strings.Join(parts, " ")
}

// assertNonTrivialView 非空性守卫：对拍不能对空库成立（空对空恒等价）。
func assertNonTrivialView(t *testing.T, s *Store) {
	t.Helper()
	view := snapshotRows(t, s)
	kinds := map[string]int{}
	for _, row := range view {
		kinds[row.Ref.Kind]++
	}
	for _, kind := range []string{"tom", "acp", "dom"} {
		if kinds[kind] == 0 {
			t.Fatalf("非空性守卫失败：%s 无物化行（对拍空洞）——view=%s", kind, matCFormatRows(view))
		}
	}
}

// ---------------------------------------------------------------------------
// G2-B①：增量轨迹与全量轨迹收敛态一致（对拍 oracle）
// ---------------------------------------------------------------------------

// TestIncrementalMatchesFullRebuild：同一事件序列，轨 I 正常脏传播+lazy 重算、
// 轨 F 每事件后 FullRebuild，收敛态 SnapshotView 逐行 deep-equal。
func TestIncrementalMatchesFullRebuild(t *testing.T) {
	project := newMatCSynthProject()
	incremental, full := newMatCDualStore(t)

	deps := project.DepInputs()
	runIncrementalStep(t, incremental, deps) // 冷启动：增量轨先物化全部（无行=miss）
	runFullRebuildStep(t, full, deps)
	assertNonTrivialView(t, full)

	for i, step := range matCStandardScript() {
		payload := step.apply(project)
		step.deliver(incremental, payload)
		step.deliver(full, payload)
		deps = project.DepInputs()
		runIncrementalStep(t, incremental, deps)
		runFullRebuildStep(t, full, deps)
		assertViewsEqual(t, incremental, full, fmt.Sprintf("step %d %s", i, step.name))
	}
}

// ---------------------------------------------------------------------------
// G2-B②：fuzz 事件序列下增量=全量（每步对拍）
// ---------------------------------------------------------------------------

// FuzzEventSequenceIncrementalEquivalence：固定 corpus seed 起步；随机事件
// 序列（invalidate/arrive 混合、重复投递）每步后对拍收敛态。fuzz 发现分歧即
// 实现 bug（传播漏标），禁改 oracle 凑绿。
// （命名适配申报：设计 §5.3 签名 TestFuzz*(f *testing.F) 与 Go 工具链冲突
// ——fuzz 目标必须 Fuzz 前缀；语义与 corpus 不变。）
func FuzzEventSequenceIncrementalEquivalence(f *testing.F) {
	f.Add(int64(1))
	f.Add(int64(7))
	f.Add(int64(42))
	f.Add(int64(20260928))
	f.Fuzz(func(t *testing.T, seed int64) {
		rng := rand.New(rand.NewSource(seed))
		project := newMatCSynthProject()
		incremental, full := newMatCDualStore(t)

		deps := project.DepInputs()
		runIncrementalStep(t, incremental, deps)
		runFullRebuildStep(t, full, deps)

		trackPool := []string{"T3", "T7", "T9"}
		for step := 0; step < 40; step++ {
			var payload any
			var deliver func(s *Store, payload any)
			switch rng.Intn(6) {
			case 0:
				payload = project.applyGainChange(trackPool[rng.Intn(len(trackPool))], -1.0-float64(rng.Intn(12)))
				deliver = matCDeliverReceipt
			case 1:
				payload = project.applyAddTrack(fmt.Sprintf("X%d", rng.Intn(4)), "extra")
				deliver = matCDeliverReceipt
			case 2:
				payload = project.applyWaveformArrival(trackPool[rng.Intn(len(trackPool))], fmt.Sprintf("wf-%d", step))
				deliver = matCDeliverTelemetry
			case 3:
				payload = project.applyL3Arrival(trackPool[rng.Intn(len(trackPool))], fmt.Sprintf("l3-%d", step))
				deliver = matCDeliverTelemetry
			case 4:
				payload = project.applyRenderJob(fmt.Sprintf("job-%d", step))
				deliver = matCDeliverRender
			default:
				payload = project.applyUnknownScope()
				deliver = matCDeliverReceipt
			}
			deliver(incremental, payload)
			deliver(full, payload)
			if rng.Intn(4) == 0 { // 重复投递（同事件再来一次）
				deliver(incremental, payload)
				deliver(full, payload)
			}
			deps = project.DepInputs()
			runIncrementalStep(t, incremental, deps)
			runFullRebuildStep(t, full, deps)
			assertViewsEqual(t, incremental, full, fmt.Sprintf("fuzz seed=%d step=%d", seed, step))
		}
	})
}

// ---------------------------------------------------------------------------
// G2-B③：重复投递与乱序收敛
// ---------------------------------------------------------------------------

// TestDuplicateAndReorderedEventsConverge：①同收据重复投递不产生代际抖动
// （generation 不前进——锁定幂等档）；②delta 与权威快照乱序到达（快照先到、
// 旧 delta 后到被 revision 门忽略）收敛态与正序一致。
func TestDuplicateAndReorderedEventsConverge(t *testing.T) {
	project := newMatCSynthProject()

	// ① 重复投递幂等：同 changeID 收据紧邻重投，generation 不前进。
	s := NewStore()
	if err := RegisterDefaultAdapters(s, nil); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	if err := s.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
	receipt := project.applyGainChange("T3", -8.0)
	s.HandleReceipt(receipt)
	genAfterFirst := s.Generation()
	s.HandleReceipt(receipt) // 重复投递（重算前）：同 changeID 的 stale 行跳过
	if got := s.Generation(); got != genAfterFirst {
		t.Fatalf("重复投递产生代际抖动：首投 gen=%d 重投 gen=%d", genAfterFirst, got)
	}

	// ② 乱序收敛：正序=delta(r1 基)→权威快照(r2 终态)；乱序=快照(r2)先到→
	// 旧 delta 后到（revision 门忽略，不回退状态）。两轨收敛态一致。
	// 权威快照（终态：T3 gain=-6、T9 在场、revision=r2）与旧 delta（T3 gain=-9，
	// r1 基）各一份 mutator；乱序轨的旧 delta 被 revision 门吞掉（不产收据——
	// shadow 语义：未应用的 delta 无 ChangeReceipt）。
	applySnapshotR2 := func(p *matCSynthProject) shadow.ChangeReceipt {
		p.tracks["T3"]["gain_db"] = -6.0
		if _, ok := p.tracks["T9"]; !ok {
			p.tracks["T9"] = map[string]any{"track_id": "T9", "track_name": "bass", "gain_db": 0.0, "plugins": []any{}}
			p.order = append(p.order, "T9")
		}
		p.revision = "r2"
		return shadow.ChangeReceipt{ChangeID: p.nextChangeID(), AffectedScopes: []string{"project.state"}}
	}
	applyDeltaR1 := func(p *matCSynthProject) (shadow.ChangeReceipt, bool) {
		if p.revision >= "r2" {
			return shadow.ChangeReceipt{}, false // revision 门：旧 delta 忽略
		}
		return p.applyGainChange("T3", -9.0), true
	}
	runSequence := func(p *matCSynthProject, forwardOrder bool) *Store {
		store := NewStore()
		if err := RegisterDefaultAdapters(store, nil); err != nil {
			t.Fatalf("RegisterDefaultAdapters: %v", err)
		}
		deliver := func(receipt shadow.ChangeReceipt, ok bool) {
			if ok {
				store.HandleReceipt(receipt)
			}
		}
		if forwardOrder {
			receipt, ok := applyDeltaR1(p)
			deliver(receipt, ok)
			store.HandleReceipt(applySnapshotR2(p))
		} else {
			store.HandleReceipt(applySnapshotR2(p))
			receipt, ok := applyDeltaR1(p)
			deliver(receipt, ok)
		}
		if err := store.RecomputeLazy(p.DepInputs()); err != nil {
			t.Fatalf("RecomputeLazy: %v", err)
		}
		return store
	}
	forwardStore := runSequence(newMatCSynthProject(), true)
	reorderedStore := runSequence(newMatCSynthProject(), false)
	assertViewsEqual(t, reorderedStore, forwardStore, "乱序收敛")
}

// ---------------------------------------------------------------------------
// 三态 flag（§7.1）
// ---------------------------------------------------------------------------

func TestModeConfigFromEnv(t *testing.T) {
	cases := []struct {
		name      string
		mode      string
		kinds     string
		wantMode  Mode
		wantKinds []string
		wantErr   bool
	}{
		{"默认 off", "", "", ModeOff, nil, false},
		{"显式 off", "off", "tom", ModeOff, nil, false},
		{"shadow 默认白名单", "shadow", "", ModeShadow, []string{"tom", "acp", "dom"}, false},
		{"on 自定义白名单", "on", "tom,acp", ModeOn, []string{"tom", "acp"}, false},
		{"大小写与空白归一", " Shadow ", " tom , acp ", ModeShadow, []string{"tom", "acp"}, false},
		{"白名单空串回默认", "shadow", "  ", ModeShadow, []string{"tom", "acp", "dom"}, false},
		{"白名单空项剔除", "shadow", "tom,,dom,", ModeShadow, []string{"tom", "dom"}, false},
		{"非法 mode 报错回退 off", "banana", "", ModeOff, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ConfigFromEnv(func(key string) string {
				switch key {
				case "VIT_DAW_MATERIALIZATION":
					return tc.mode
				case "VIT_DAW_MATERIALIZED_KINDS":
					return tc.kinds
				}
				return ""
			})
			if tc.wantErr && err == nil {
				t.Fatalf("期望非法 mode 报错，得到 %+v", cfg)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("意外错误: %v", err)
			}
			if cfg.Mode != tc.wantMode {
				t.Fatalf("mode: got=%q want=%q", cfg.Mode, tc.wantMode)
			}
			if tc.wantMode == ModeOff {
				if len(cfg.Kinds) != 0 {
					t.Fatalf("off 态不应带白名单: %+v", cfg.Kinds)
				}
				return
			}
			if !reflect.DeepEqual(cfg.Kinds, tc.wantKinds) {
				t.Fatalf("kinds: got=%v want=%v", cfg.Kinds, tc.wantKinds)
			}
		})
	}
}

// 白名单过滤：RegisterDefaultAdapters 只注册白名单内的适配器（kind 参与面）。
func TestAdapterWhitelistFiltersKinds(t *testing.T) {
	s := NewStore()
	if err := RegisterDefaultAdapters(s, []string{"tom"}); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	if err := s.RecomputeLazy(newMatCSynthProject().DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
	view := snapshotRows(t, s)
	for _, row := range view {
		if row.Ref.Kind != "tom" {
			t.Fatalf("白名单外 kind 被物化：%s（view=%s）", row.Ref.Kind, matCFormatRows(view))
		}
	}
	if len(view) == 0 {
		t.Fatalf("白名单 kind 未物化任何行（对拍空洞）")
	}
}

// ---------------------------------------------------------------------------
// 影子对账（§7.1 shadow：现算产物 vs 物化行逐行 hash 比，分歧入 Metrics）
// ---------------------------------------------------------------------------

// TestReconcileShadowCountsDivergences：同坐标 hash 不等→分歧；物化行缺失→
// 分歧；完全一致→零分歧。计数入 Metrics.ShadowDivergences。
func TestReconcileShadowCountsDivergences(t *testing.T) {
	s := NewStore()
	project := newMatCSynthProject()
	if err := RegisterDefaultAdapters(s, nil); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	if err := s.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}

	// 基准行（与物化 dom/T3 同内容同坐标）：零分歧。
	base := mustDOMRowForSynth(t, project, "T3")
	if got := s.ReconcileShadow([]Row{base}); got != 0 {
		t.Fatalf("一致产物不应计分歧：got=%d", got)
	}
	if got := s.Metrics().ShadowDivergences; got != 0 {
		t.Fatalf("Metrics.ShadowDivergences 应为 0：got=%d", got)
	}

	// 内容分歧：改 hash 后对账。
	diverged := base
	diverged.Ref.Hash = "sha256:deadbeefdeadbeef"
	if got := s.ReconcileShadow([]Row{diverged}); got != 1 {
		t.Fatalf("hash 不等应计 1 分歧：got=%d", got)
	}
	if got := s.Metrics().ShadowDivergences; got != 1 {
		t.Fatalf("Metrics.ShadowDivergences 应为 1：got=%d", got)
	}

	// 坐标缺失分歧：不存在的 track。
	missing := base
	missing.Ref.ScopeValue = "T404"
	if got := s.ReconcileShadow([]Row{missing}); got != 1 {
		t.Fatalf("坐标缺失应计 1 分歧：got=%d", got)
	}
	if got := s.Metrics().ShadowDivergences; got != 2 {
		t.Fatalf("Metrics.ShadowDivergences 应累计为 2：got=%d", got)
	}
}

// mustDOMRowForSynth 用与适配器同源的行化路径取合成工程某轨的 dom 行（对账
// 基准——与物化行同内容同坐标）。
func mustDOMRowForSynth(t *testing.T, project *matCSynthProject, trackID string) Row {
	t.Helper()
	deps := project.DepInputs()
	adapter := DOMAdapter()
	rows, err := adapter.Build(deps)
	if err != nil {
		t.Fatalf("DOMAdapter.Build: %v", err)
	}
	for _, row := range rows {
		if row.Ref.Kind == "dom" && row.Ref.ScopeValue == trackID {
			return row
		}
	}
	t.Fatalf("DOMAdapter 未产出行 dom/%s（rows=%v）", trackID, rows)
	return Row{}
}

// TestShadowReconcileSyntheticProjectZeroDivergence：影子对账核心证据——
// 现状权威路径（mixboard.RequestObservation → FinalizeObservationContext →
// DOMProjection）的产物，与物化层适配器（同输入域独立重算）行化后逐行 hash
// 相等：合成工程 ShadowDivergences==0。这是 §7.2 切换闸门的前置证明。
func TestShadowReconcileSyntheticProjectZeroDivergence(t *testing.T) {
	project := newMatCSynthProject()

	// 物化轨：适配器从输入域重算并 Upsert。
	store := NewStore()
	if err := RegisterDefaultAdapters(store, nil); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	if err := store.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}

	// 现状轨：mixboard.RequestObservation 真跑（含 FinalizeObservationContext，
	// 与物化层零共享）。feature_snapshot 经 Args 内联注入（loadFeatureSnapshot
	// 的 args 通道），target=T3。
	deps := project.DepInputs()
	snapshotCopy := map[string]any{}
	if data, err := json.Marshal(deps.FeatureSnapshot); err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	} else if err := json.Unmarshal(data, &snapshotCopy); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	result, err := mixboard.NewStore(t.TempDir()).RequestObservation(mixboard.Request{
		MixSessionID: "mix-matc-shadow",
		TargetRef:    mixboard.TargetRef{Kind: "track", ID: "T3"},
		ProjectState: deps.ProjectState,
		Args:         map[string]any{"feature_snapshot": snapshotCopy},
	})
	if err != nil {
		t.Fatalf("RequestObservation: %v", err)
	}
	if result.Observation.DOMProjection == nil {
		t.Fatalf("现状轨未产 DOMProjection（对账前提缺失）")
	}

	fresh, ok := DOMRowFromProjection("T3", *result.Observation.DOMProjection)
	if !ok {
		t.Fatalf("DOMRowFromProjection 未产行（observation DOMProjection 状态=%q）", result.Observation.DOMProjection.Status)
	}
	if got := store.ReconcileShadow([]Row{fresh}); got != 0 {
		materialized := mustDOMRowForSynth(t, project, "T3")
		t.Fatalf("合成工程影子对账分歧=%d（期望 0）\n物化行=%+v\n现状行=%+v", got, materialized, fresh)
	}
	if got := store.Metrics().ShadowDivergences; got != 0 {
		t.Fatalf("Metrics.ShadowDivergences=%d（期望 0）", got)
	}
}

// ---------------------------------------------------------------------------
// 适配器登记路径：fxm/com 随观察落盘登记（F9：不预计算）
// ---------------------------------------------------------------------------

// TestRegisteredRowsFromObservation：观察产物的 fxm/com 投影登记为行
// （snapshot=observation_id，hash=实例身份含时间戳——QUERY_ENGINE §3.3）。
func TestRegisteredRowsFromObservation(t *testing.T) {
	observationID := "obs_matc_0001"
	domRow, ok := DOMRowFromProjection("T3", domReadyProjectionFixture())
	if !ok {
		t.Fatalf("DOMRowFromProjection 应产行")
	}
	if domRow.Ref.Kind != "dom" || domRow.Ref.ScopeValue != "T3" {
		t.Fatalf("dom 行坐标不符: %+v", domRow.Ref)
	}
	if domRow.Ref.Snapshot != snapshotTokenCurrent {
		t.Fatalf("precomputable 行 snapshot 应为稳定实例 token %q: got=%q", snapshotTokenCurrent, domRow.Ref.Snapshot)
	}
	fxmRows := FXMRowsFromObservation(observationID, fxmReadyProjectionFixture())
	if len(fxmRows) != 1 || fxmRows[0].Ref.Kind != "fxm" {
		t.Fatalf("fxm 登记行数/坐标不符: %+v", fxmRows)
	}
	if fxmRows[0].Ref.Snapshot != observationID {
		t.Fatalf("registered 行 snapshot 应为 observation_id: got=%q", fxmRows[0].Ref.Snapshot)
	}
	comRows := COMRowsFromObservation(observationID, comReadyProjectionFixture())
	if len(comRows) != 1 || comRows[0].Ref.Kind != "com" {
		t.Fatalf("com 登记行数/坐标不符: %+v", comRows)
	}
	if comRows[0].Ref.Snapshot != observationID {
		t.Fatalf("registered 行 snapshot 应为 observation_id: got=%q", comRows[0].Ref.Snapshot)
	}

	// 登记路径可直接 Upsert（坐标合法性——refschema 注册表门）。
	s := NewStore()
	rows := append([]Row{domRow}, fxmRows...)
	rows = append(rows, comRows...)
	if err := s.Upsert(rows); err != nil {
		t.Fatalf("Upsert 登记行: %v", err)
	}
	view := snapshotRows(t, s)
	kinds := map[string]bool{}
	for _, row := range view {
		kinds[row.Ref.Kind] = true
	}
	for _, kind := range []string{"dom", "fxm", "com"} {
		if !kinds[kind] {
			t.Fatalf("登记后缺 %s 行: view=%s", kind, matCFormatRows(view))
		}
	}
}

// ---------------------------------------------------------------------------
// RecomputeLazy / FullRebuild 契约
// ---------------------------------------------------------------------------

// TestRecomputeLazySkipsCleanKinds：无脏 kind 不重算（Metrics.Recomputes 不动）
// ——lazy 语义：不为没有读者的变更付费（§2.2）。
func TestRecomputeLazySkipsCleanKinds(t *testing.T) {
	s := NewStore()
	if err := RegisterDefaultAdapters(s, nil); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	project := newMatCSynthProject()
	if err := s.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
	before := s.Metrics()
	if err := s.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy(第二次): %v", err)
	}
	after := s.Metrics()
	for kind, metric := range after.PerKind {
		if metric.Recomputes > before.PerKind[kind].Recomputes {
			t.Fatalf("干净 kind %s 被 lazy 重算：%d→%d", kind, before.PerKind[kind].Recomputes, metric.Recomputes)
		}
	}
	if after.ShadowDivergences != before.ShadowDivergences {
		t.Fatalf("RecomputeLazy 不应触碰影子对账计数")
	}
}

// TestFullRebuildForcesAllPrecomputable：oracle 无视 dirty 集合强制全量重算
// （干净 kind 也重算——与 lazy 的差异即对拍的意义）。
func TestFullRebuildForcesAllPrecomputable(t *testing.T) {
	s := NewStore()
	if err := RegisterDefaultAdapters(s, nil); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	project := newMatCSynthProject()
	if err := s.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
	before := s.Metrics()
	if err := s.FullRebuild(project.DepInputs()); err != nil {
		t.Fatalf("FullRebuild: %v", err)
	}
	after := s.Metrics()
	for _, kind := range []string{"tom", "acp", "dom"} {
		if after.PerKind[kind].Recomputes != before.PerKind[kind].Recomputes+1 {
			t.Fatalf("FullRebuild 未强制重算 %s：%d→%d", kind, before.PerKind[kind].Recomputes, after.PerKind[kind].Recomputes)
		}
	}
}

// TestTomAdapterTracksShadowStructure：tom 行域=project.structure——加轨后
// 重算产生新轨行（scope 精度的行坐标）。
func TestTomAdapterTracksShadowStructure(t *testing.T) {
	s := NewStore()
	if err := RegisterDefaultAdapters(s, []string{"tom"}); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	project := newMatCSynthProject()
	if err := s.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
	if _, ok := rowByScope(snapshotRows(t, s), "tom", "T9"); ok {
		t.Fatalf("T9 未加入前不应有 tom/T9 行")
	}
	project.applyAddTrack("T9", "bass")
	s.HandleReceipt(shadow.ChangeReceipt{ChangeID: "c1", AffectedScopes: []string{"project.structure"}})
	if err := s.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
	row, ok := rowByScope(snapshotRows(t, s), "tom", "T9")
	if !ok {
		t.Fatalf("加轨并重算后缺 tom/T9 行")
	}
	if row.Freshness != agentprotocol.FreshnessCurrent {
		t.Fatalf("新行 freshness 应为 current: %q", row.Freshness)
	}
	// payload 携带 QUERY_ENGINE §3.3 建议字段。
	if _, ok := row.Payload["track_name"]; !ok {
		t.Fatalf("tom 行 payload 缺 track_name: %+v", row.Payload)
	}
}

// TestACPAdapterRowsFromPackages：acp 行域=acoustic.l3.*（包行变化→重算反映）。
func TestACPAdapterRowsFromPackages(t *testing.T) {
	s := NewStore()
	if err := RegisterDefaultAdapters(s, []string{"acp"}); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	project := newMatCSynthProject()
	if err := s.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
	row, ok := rowByScope(snapshotRows(t, s), "acp", "T3")
	if !ok {
		t.Fatalf("缺 acp/T3 行")
	}
	if _, ok := row.Payload["status"]; !ok {
		t.Fatalf("acp 行 payload 缺 status: %+v", row.Payload)
	}
	beforeHash := row.Ref.Hash

	// 包换代（source_revision 变）+arrive 通知 → 重算 → hash 变化。
	upserted := matCPackage("T3", "sr2")
	upserted.Status = acousticpackage.StatusPartial
	project.packages = []acousticpackage.Status{upserted}
	s.HandleFeatureArrival(map[string]any{"command": "audio_feature_data_ready",
		"feature_type": "band_energy_summary", "track_id": "T3", "request_id": "l3-2"})
	if err := s.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
	row, ok = rowByScope(snapshotRows(t, s), "acp", "T3")
	if !ok {
		t.Fatalf("换代后缺 acp/T3 行")
	}
	if row.Ref.Hash == beforeHash {
		t.Fatalf("包换代后 acp 行 hash 未变（重算未反映输入域变化）")
	}
}

// ---------------------------------------------------------------------------
// 供登记/对账测试用的投影 fixture（各投影包真实类型）
// ---------------------------------------------------------------------------

// domReadyProjectionFixture 产一个 ready 态 source-only dom 投影（真实 dom.Build）。
func domReadyProjectionFixture() dom.Projection {
	rms, peak, headroom, crest := -20.5, -3.2, -0.8, 17.3
	return dom.Build(dom.Input{
		Mode:          dom.ModeSourceOnly,
		ObservationID: "obs_matc_fixture",
		MixSessionID:  "mix_matc",
		CreatedAt:     "2026-09-28T00:00:00Z",
		TargetRef:     map[string]any{"kind": "track", "id": "T3", "label": "lead vox"},
		Source: dom.SourceEvidence{
			Status: dom.StatusReady, Duration: 12.0,
			RMSDBFS: &rms, PeakDBFS: &peak, HeadroomDB: &headroom, CrestDB: &crest,
			TimeSegments: []dom.TimeSegment{
				{StartSeconds: 0, EndSeconds: 6, RMSDBFS: &rms, PeakDBFS: &peak},
				{StartSeconds: 6, EndSeconds: 12, RMSDBFS: &rms, PeakDBFS: &headroom},
			},
			Bands: []dom.BandEvidence{
				{ID: "sub", Status: dom.StatusReady, MinHz: 20, MaxHz: 60, EnergyDB: &rms},
				{ID: "bass", Status: dom.StatusReady, MinHz: 60, MaxHz: 250, EnergyDB: &peak},
			},
		},
	})
}

// fxmReadyProjectionFixture 产一个 ready 态 fxm 投影（真实字段，行化只读标量）。
func fxmReadyProjectionFixture() fxm.Projection {
	return fxm.Projection{
		SchemaVersion: fxm.SchemaVersion, FXMVersion: fxm.Version,
		ProjectionID: "fxm_matc_fixture_0001", Status: fxm.StatusReady,
		TargetRef: map[string]any{"kind": "track", "id": "T3"},
	}
}

// comReadyProjectionFixture 产一个 ready 态 com 投影（真实字段，行化只读标量）。
func comReadyProjectionFixture() com.Projection {
	return com.Projection{
		SchemaVersion: com.SchemaVersion, COMVersion: com.Version,
		ProjectionID: "com_matc_fixture_0001", Mode: com.ModeSourceOnly, Status: com.StatusReady,
		TargetRef: map[string]any{"kind": "track", "id": "T3"},
	}
}
