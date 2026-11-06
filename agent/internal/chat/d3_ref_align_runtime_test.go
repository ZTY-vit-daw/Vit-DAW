package chat

// D3-REF-ALIGN-THIN-1: D3 reference-alignment face tests — zero LLM. Covers
// the intent family (explicit check requests only, no bare-topic hijack), the
// tier-1 reference-standard resolution (render binding first, intent-named
// builtin second, none = unmeasurable), the three-state criterion (delegated
// to loudnesswatch.Compare: approximate grade, band semantics, peak necessary
// condition), the verdict store discipline (dedupe / foreign-schema
// fail-closed / history trim), the handler wiring (verdict on disk, gap parks
// the governed gain card with zero mutation, aligned/unmeasurable park none,
// D3 node lit), and the no-vocabulary entry baseline (check phrases stay off
// the capability entry word list; the explicit d3 stage marker enters the
// observation pack).

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/loudnesswatch"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/rlm"
	agentruntime "vit-daw-agent/internal/runtime"
)

func d3RefTestServer(t *testing.T, rows []map[string]any) (*Server, string) {
	t.Helper()
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	projectRoot := t.TempDir()
	if _, _, err := projectstore.Activate(filepath.Join(projectRoot, "song.vitproj"), "proj-d3"); err != nil {
		t.Fatalf("activate projectstore: %v", err)
	}
	server := &Server{
		harness:         harness.New(nil, nil, nil),
		pendingMixTicks: map[string]agentloop.PendingMixTickCandidate{},
		interactions:    map[string]PendingInteraction{},
	}
	snapshot := map[string]any{"schema_version": "mixboard_feature_snapshot.v1", "loudness_summaries": rows}
	snapshotDir := filepath.Dir(mixboard.FeatureSnapshotPath(map[string]any{}))
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatalf("mkdir snapshot dir: %v", err)
	}
	snapshotData, _ := json.Marshal(snapshot)
	if err := os.WriteFile(mixboard.FeatureSnapshotPath(map[string]any{}), snapshotData, 0o644); err != nil {
		t.Fatalf("seed feature snapshot: %v", err)
	}
	return server, snapshotDir
}

// d3RefBoundProjection builds a minimal RLM projection carrying one bound
// render→delivery binding (the D3 reference face of the observation pack).
func d3RefBoundProjection(profileID string) rlm.Projection {
	binding, err := rlm.NewRenderProfileBinding("render-1", profileID, "2026-11-07T00:00:00Z")
	if err != nil {
		panic(err)
	}
	disclosure := rlm.BuildRenderBindingDisclosure([]rlm.RenderProfileBinding{binding})
	return rlm.Projection{SchemaVersion: rlm.SchemaVersion, RenderBindings: disclosure}
}

func d3RefGapRows() []map[string]any {
	// ebu_r128 band [-23.5, -22.5]: c1 at -18 is 4.5 dB above the ceiling and
	// its sample peak sits above the -1 dBTP bound; c2 is inside the band.
	return []map[string]any{
		{"track_id": "t1", "clip_id": "c1", "integrated_lufs": -18.0, "sample_peak_dbfs": -0.4, "status": "ready"},
		{"track_id": "t2", "clip_id": "c2", "integrated_lufs": -23.0, "sample_peak_dbfs": -3.0, "status": "ready"},
	}
}

func d3RefAlignedRows() []map[string]any {
	// spotify band (≤ -14, no floor): both clips inside; peaks under -1 dBTP.
	return []map[string]any{
		{"track_id": "t1", "clip_id": "c1", "integrated_lufs": -15.0, "sample_peak_dbfs": -3.0, "status": "ready"},
		{"track_id": "t2", "clip_id": "c2", "integrated_lufs": -14.0, "sample_peak_dbfs": -1.0, "status": "ready"},
	}
}

func d3RefReport(observationID string) d1d3PreMasterReport {
	peak := -0.9
	return d1d3PreMasterReport{SchemaVersion: d1d3ObservationSchema, ObservationID: observationID, LoudnessPeakStatus: "present", GlobalPeakDBFS: &peak}
}

func TestD3RefAlignIntentWords(t *testing.T) {
	for _, phrase := range []string{"d3 参考对齐", "做个参考对齐", "参考曲对比一下", "参考标准检查", "对齐检查", "交付标准检查", "run a ref align", "reference check please"} {
		if !d3RefAlignIntent(phrase) {
			t.Fatalf("expected ref-align intent for %q", phrase)
		}
	}
	// 裸话题词与相邻能力话术不是 D3 检查请求（M1-4 反门语义；裸话术入口交给
	// semantic_entry 实测，见卡面）。
	for _, phrase := range []string{"参考曲", "spotify", "响度有点大", "响度检查", "全局响度检查", "翻译检查", "mono 检查", "低频关系分析", "d3"} {
		if d3RefAlignIntent(phrase) {
			t.Fatalf("unexpected ref-align intent for %q", phrase)
		}
	}
}

func TestD3RefAlignIntentNamedProfileResolution(t *testing.T) {
	cases := []struct {
		message string
		want    string
	}{
		{"d3 参考对齐 spotify", "builtin:spotify"},
		{"对齐检查 按Apple Music的标准", "builtin:apple_music"},
		{"参考对齐 ebu", "builtin:ebu_r128"},
		{"参考标准检查 aes 流媒体", "builtin:aes_td1004"},
		{"参考对齐 国标", "builtin:gy_282_2014"},
		{"reference check gy/t 282", "builtin:gy_282_2014"},
	}
	for _, testCase := range cases {
		if got, ok := d3IntentNamedProfile(testCase.message); !ok || got != testCase.want {
			t.Fatalf("d3IntentNamedProfile(%q)=%q,%v want %q", testCase.message, got, ok, testCase.want)
		}
	}
	// 词边界：aesthetics 不得选中 aes；无点名词不解析。
	for _, phrase := range []string{"对齐检查 aesthetics", "参考对齐一下", "ref align tidal"} {
		if got, ok := d3IntentNamedProfile(phrase); ok {
			t.Fatalf("d3IntentNamedProfile(%q) unexpectedly resolved %q", phrase, got)
		}
	}
}

func TestD3RefAlignStandardResolutionBindingFirst(t *testing.T) {
	projection := d3RefBoundProjection("builtin:ebu_r128")
	standard, ok := d3ResolveRefStandard(projection, "d3 参考对齐 spotify")
	if !ok {
		t.Fatal("a bound render binding must select a standard")
	}
	if standard.Basis != "render_binding" || standard.RenderID != "render-1" || standard.Profile.ProfileID != "builtin:ebu_r128" {
		t.Fatalf("binding must win over the intent-named profile: %+v", standard)
	}
	// 无绑定（空投影）+ 意图点名 → intent_named。
	standard, ok = d3ResolveRefStandard(rlm.Projection{}, "d3 参考对齐 spotify")
	if !ok || standard.Basis != "intent_named" || standard.Profile.ProfileID != "builtin:spotify" {
		t.Fatalf("intent-named fallback broken: %+v ok=%v", standard, ok)
	}
	// 无绑定 + 无点名 → 无参考标准。
	if _, ok := d3ResolveRefStandard(rlm.Projection{}, "d3 参考对齐"); ok {
		t.Fatal("no binding and no named builtin must resolve no standard")
	}
	// unresolvable 绑定行不选标准（投影 fail-closed 披露维持，另由
	// limitations 面可见）。
	unresolvable := rlm.Projection{RenderBindings: []rlm.RenderBindingDisclosure{
		{RenderID: "render-x", ProfileID: "builtin:retired", Status: rlm.RenderBindingStatusUnresolvable, Error: "unknown"},
	}}
	if _, ok := d3ResolveRefStandard(unresolvable, "d3 参考对齐"); ok {
		t.Fatal("an unresolvable binding must not select a standard")
	}
}

func TestD3RefAlignAssessGapFound(t *testing.T) {
	projection := d3RefBoundProjection("builtin:ebu_r128")
	verdict, lw := d3AssessRefAlignment(projection, d3RefGapRows(), "d3 参考对齐")
	if verdict.Verdict != d3RefAlignVerdictGapFound {
		t.Fatalf("expected gap_found, got %s", verdict.Verdict)
	}
	if len(verdict.GapRows) != 2 {
		t.Fatalf("expected integrated + peak gaps for c1, got %+v", verdict.GapRows)
	}
	integratedGap, peakGap := verdict.GapRows[0], verdict.GapRows[1]
	if integratedGap.Metric != "integrated_lufs" || integratedGap.Measured != -18.0 || integratedGap.Threshold != -22.5 || integratedGap.DeltaDB != 4.5 {
		t.Fatalf("integrated gap must carry the pinned band numbers: %+v", integratedGap)
	}
	if peakGap.Metric != "sample_peak_dbfs" || peakGap.Threshold != -1 {
		t.Fatalf("peak gap must carry the profile TP ceiling: %+v", peakGap)
	}
	if verdict.ProfileBasis != "render_binding" || verdict.LWVerdict != loudnesswatch.VerdictBreach {
		t.Fatalf("standard basis and delegated verdict kind must be pinned: %+v", verdict)
	}
	if verdict.MeasurementGrade != loudnesswatch.MeasurementGradeApproximate {
		t.Fatalf("approximate grade must be inherited from the delegated comparator: %+v", verdict.MeasurementGrade)
	}
	if !strings.Contains(verdict.Summary, "2 处对齐差距") || !strings.Contains(verdict.Summary, "近似口径") {
		t.Fatalf("summary must quantify gaps with the approximate qualifier: %q", verdict.Summary)
	}
	if lw == nil || lw.Verdict != loudnesswatch.VerdictBreach {
		t.Fatalf("de comparator verdict must be returned for the action face: %+v", lw)
	}
}

func TestD3RefAlignAssessAligned(t *testing.T) {
	verdict, lw := d3AssessRefAlignment(rlm.Projection{}, d3RefAlignedRows(), "d3 参考对齐 spotify")
	if verdict.Verdict != d3RefAlignVerdictAligned {
		t.Fatalf("expected aligned (spotify one-sided band, both clips at/below -14), got %s", verdict.Verdict)
	}
	if len(verdict.GapRows) != 0 || verdict.MeasuredClips != 2 {
		t.Fatalf("unexpected aligned verdict payload: %+v", verdict)
	}
	if verdict.ProfileID != "builtin:spotify" || verdict.ProfileBasis != "intent_named" {
		t.Fatalf("standard context must be disclosed: %+v", verdict)
	}
	if verdict.BandMinLUFS != nil || verdict.BandMaxLUFS == nil || *verdict.BandMaxLUFS != -14 {
		t.Fatalf("spotify one-sided band must be disclosed as no-floor: %+v", verdict.BandMinLUFS)
	}
	if !strings.Contains(verdict.Summary, "目标带内") {
		t.Fatalf("summary must state the aligned outcome: %q", verdict.Summary)
	}
	if lw == nil || lw.Verdict != loudnesswatch.VerdictPass || lw.PeakCheck != loudnesswatch.PeakCheckNoBreachSeen {
		t.Fatalf("peak necessary-condition note must be inherited: %+v", lw)
	}
}

func TestD3RefAlignAssessBoundaryExactness(t *testing.T) {
	// band 界值语义继承 LW：恰在 max / 恰在 min 都不是差距（严格比较）。
	rows := []map[string]any{
		{"track_id": "t1", "clip_id": "c1", "integrated_lufs": -22.5, "sample_peak_dbfs": -1.0, "status": "ready"},
		{"track_id": "t2", "clip_id": "c2", "integrated_lufs": -23.5, "status": "ready"},
	}
	verdict, _ := d3AssessRefAlignment(d3RefBoundProfileProjectionEBU(), rows, "d3 参考对齐")
	if verdict.Verdict != d3RefAlignVerdictAligned {
		t.Fatalf("exact band boundaries must stay aligned, got %s (%+v)", verdict.Verdict, verdict.GapRows)
	}
}

func d3RefBoundProfileProjectionEBU() rlm.Projection { return d3RefBoundProjection("builtin:ebu_r128") }

func TestD3RefAlignAssessUnmeasurableShapes(t *testing.T) {
	cases := []struct {
		name       string
		projection rlm.Projection
		rows       []map[string]any
		message    string
		needle     string
	}{
		{
			name:    "no reference standard at all",
			rows:    d3RefAlignedRows(),
			message: "d3 参考对齐",
			needle:  "no_reference_standard",
		},
		{
			name:       "standard resolved but no measurement rows",
			projection: d3RefBoundProfileProjectionEBU(),
			rows:       nil,
			message:    "d3 参考对齐",
			needle:     "no_measurement_rows",
		},
		{
			name:       "rows present but none carries integrated",
			projection: d3RefBoundProfileProjectionEBU(),
			rows:       []map[string]any{{"track_id": "t1", "clip_id": "c1", "sample_peak_dbfs": -3.0}},
			message:    "d3 参考对齐",
			needle:     "integrated_lufs_missing",
		},
	}
	for _, testCase := range cases {
		verdict, lw := d3AssessRefAlignment(testCase.projection, testCase.rows, testCase.message)
		if verdict.Verdict != d3RefAlignVerdictUnmeasurable {
			t.Fatalf("%s: expected unmeasurable, got %s", testCase.name, verdict.Verdict)
		}
		if !strings.Contains(verdict.UnmeasurableReason, testCase.needle) {
			t.Fatalf("%s: reason %q must carry %q", testCase.name, verdict.UnmeasurableReason, testCase.needle)
		}
		if testCase.needle == "no_reference_standard" && lw != nil {
			t.Fatalf("%s: no-standard face must not run the comparator", testCase.name)
		}
	}
}

func TestD3RefAlignVerdictStoreDedupeAndKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ref_align_verdicts.json")
	now := time.Now()
	base := d3RefAlignVerdictRecord{
		ProjectUUID: "proj-d3", ProjectRevision: "7", VerdictKind: d3RefAlignVerdictAligned,
		Verdict:     d3RefAlignVerdict{Verdict: d3RefAlignVerdictAligned, MeasuredClips: 2},
		TriggerKind: d3RefAlignTriggerKind, GapCount: 0, MeasuredClips: 2, ProfileID: "builtin:spotify",
	}
	key := recordD3RefAlignVerdict(path, base, now, func(string, ...any) {})
	if key != "proj-d3|7|aligned" {
		t.Fatalf("unexpected dedupe key: %q", key)
	}
	recordD3RefAlignVerdict(path, base, now.Add(time.Minute), func(string, ...any) {})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("store file missing: %v", err)
	}
	store := d3RefAlignVerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if store.SchemaVersion != d3RefAlignVerdictStoreSchemaVersion || store.ChecksTotal != 2 {
		t.Fatalf("unexpected store header: %+v", store)
	}
	if len(store.Verdicts) != 1 || store.Verdicts[0].RecheckCount != 1 {
		t.Fatalf("dedupe must bump recheck audit, not duplicate: %+v", store.Verdicts)
	}
	gap := base
	gap.VerdictKind = d3RefAlignVerdictGapFound
	recordD3RefAlignVerdict(path, gap, now.Add(2*time.Minute), func(string, ...any) {})
	data, _ = os.ReadFile(path)
	store = d3RefAlignVerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if len(store.Verdicts) != 2 || store.Verdicts[1].VerdictKind != d3RefAlignVerdictGapFound {
		t.Fatalf("expected a second record for the new verdict kind: %+v", store.Verdicts)
	}
}

func TestD3RefAlignVerdictStoreForeignSchemaFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ref_align_verdicts.json")
	foreign := map[string]any{"schema_version": "ref_align_verdicts.v9", "verdicts": []map[string]any{{"key": "old"}}}
	data, _ := json.Marshal(foreign)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	warned := false
	record := d3RefAlignVerdictRecord{ProjectUUID: "p", VerdictKind: d3RefAlignVerdictAligned, Verdict: d3RefAlignVerdict{Verdict: d3RefAlignVerdictAligned}, TriggerKind: d3RefAlignTriggerKind}
	recordD3RefAlignVerdict(path, record, time.Now(), func(format string, args ...any) { warned = strings.Contains(format, "fail closed") })
	if !warned {
		t.Fatal("a foreign schema must WARN before failing closed")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	store := d3RefAlignVerdictStoreFile{}
	if err := json.Unmarshal(raw, &store); err != nil {
		t.Fatal(err)
	}
	if store.SchemaVersion != d3RefAlignVerdictStoreSchemaVersion || len(store.Verdicts) != 1 {
		t.Fatalf("foreign history must restart empty, got %+v", store)
	}
}

func TestD3RefAlignVerdictStoreTrimsHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ref_align_verdicts.json")
	now := time.Now()
	for index := 0; index < d3RefAlignMaxKeptVerdicts+2; index++ {
		record := d3RefAlignVerdictRecord{
			ProjectUUID: "p", ProjectRevision: fmt.Sprintf("r%d", index), VerdictKind: d3RefAlignVerdictAligned,
			Verdict: d3RefAlignVerdict{Verdict: d3RefAlignVerdictAligned}, TriggerKind: d3RefAlignTriggerKind,
		}
		recordD3RefAlignVerdict(path, record, now.Add(time.Duration(index)*time.Second), func(string, ...any) {})
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	store := d3RefAlignVerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if len(store.Verdicts) != d3RefAlignMaxKeptVerdicts {
		t.Fatalf("history must trim to %d, got %d", d3RefAlignMaxKeptVerdicts, len(store.Verdicts))
	}
	if store.Verdicts[0].ProjectRevision != "r2" {
		t.Fatalf("trim must keep the tail, oldest surviving revision=%q", store.Verdicts[0].ProjectRevision)
	}
}

func TestD3RefAlignCheckHandlerParksGainCardOnGap(t *testing.T) {
	server, snapshotDir := d3RefTestServer(t, d3RefGapRows())
	req := ChatRequest{ConversationID: "conv-d3", Message: "d3 参考对齐 ebu"}
	goal := agentruntime.Goal{GoalID: "goal-d3", RunID: "run-d3"}
	response := server.handleD3RefAlignCheck(context.Background(), "conv-d3", req, goal, "sess-d3-1", nil, d3RefReport("obs-d3-1"), d3RefBoundProfileProjectionEBU())

	if response.StopReason != "d3_ref_align_checked" {
		t.Fatalf("expected stop_reason d3_ref_align_checked, got %s", response.StopReason)
	}
	if response.NeedsConfirmation != true || len(response.InteractionRequests) == 0 {
		t.Fatalf("an addressable gap must park a confirmation card: needs=%v cards=%d", response.NeedsConfirmation, len(response.InteractionRequests))
	}
	if response.WorkflowData["mutation_performed"] != false {
		t.Fatalf("alignment face must not mutate the project: %+v", response.WorkflowData["mutation_performed"])
	}
	payload, ok := response.WorkflowData["d3_ref_align"].(map[string]any)
	if !ok || payload["verdict"] != d3RefAlignVerdictGapFound {
		t.Fatalf("expected gap_found payload, got %+v", response.WorkflowData["d3_ref_align"])
	}
	if payload["action_face"] != d3RefAlignActionGainCard {
		t.Fatalf("addressable level-class gap must walk the existing gain card: %+v", payload["action_face"])
	}
	if payload["profile_basis"] != "render_binding" || payload["profile_id"] != "builtin:ebu_r128" {
		t.Fatalf("standard context must be disclosed in the payload: %+v", payload)
	}

	candidate, parked := server.pendingMixTickForConversation("conv-d3")
	if !parked {
		t.Fatal("expected a parked pending mix tick candidate")
	}
	if candidate.Operation != loudnesswatch.ActionOperationGainAdjust || candidate.TrackID != "t1" || candidate.DeltaDB >= 0 {
		t.Fatalf("expected a pull-down gain candidate on the worst-gap track t1, got %+v", candidate)
	}
	if candidate.Evidence["capability_id"] != d3RefAlignCapabilityID {
		t.Fatalf("candidate evidence must pin the D3 capability: %+v", candidate.Evidence)
	}

	// The verdict landed in the store under the d3_ref_align trigger.
	data, err := os.ReadFile(filepath.Join(snapshotDir, "ref_align_verdicts.json"))
	if err != nil {
		t.Fatalf("verdict store missing: %v", err)
	}
	store := d3RefAlignVerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if store.ChecksTotal != 1 || len(store.Verdicts) != 1 || store.Verdicts[0].TriggerKind != d3RefAlignTriggerKind {
		t.Fatalf("unexpected verdict store: %+v", store)
	}
	if store.Verdicts[0].VerdictKind != d3RefAlignVerdictGapFound || store.Verdicts[0].GapCount != 2 || store.Verdicts[0].ProfileBasis != "render_binding" {
		t.Fatalf("unexpected verdict record: %+v", store.Verdicts[0])
	}

	// The D3 queue node left not_started (card parked -> needs_confirmation).
	server.mu.Lock()
	annotation, annotated := server.mixWorkflowQueueNodes["D3"]
	server.mu.Unlock()
	if !annotated || annotation.State != MixWorkflowNodeNeedsConfirmation {
		t.Fatalf("expected D3 annotation needs_confirmation, got %+v (%v)", annotation, annotated)
	}
	view := server.BuildMixWorkflowQueueView(context.Background())
	for _, node := range view.Nodes {
		if node.NodeID == "D3" && node.State == MixWorkflowNodeNotStarted {
			t.Fatalf("D3 must not stay not_started after a ref-align run: %+v", node)
		}
	}
	if !strings.Contains(response.Reply, "待确认") || !strings.Contains(response.Reply, "builtin:ebu_r128") {
		t.Fatalf("reply must carry the standard and the confirmation boundary: %q", response.Reply)
	}
}

func TestD3RefAlignCheckHandlerAlignedParksNoCard(t *testing.T) {
	server, _ := d3RefTestServer(t, d3RefAlignedRows())
	req := ChatRequest{ConversationID: "conv-d3b", Message: "d3 参考对齐 spotify"}
	goal := agentruntime.Goal{GoalID: "goal-d3b", RunID: "run-d3b"}
	response := server.handleD3RefAlignCheck(context.Background(), "conv-d3b", req, goal, "sess-d3-1", nil, d3RefReport("obs-d3-2"), rlm.Projection{})

	if response.NeedsConfirmation || len(response.InteractionRequests) > 0 {
		t.Fatalf("an aligned verdict must park no card: needs=%v cards=%d", response.NeedsConfirmation, len(response.InteractionRequests))
	}
	payload, _ := response.WorkflowData["d3_ref_align"].(map[string]any)
	if payload["verdict"] != d3RefAlignVerdictAligned || payload["action_face"] != d3RefAlignActionNoneNeeded {
		t.Fatalf("aligned outcome must disclose none_needed: %+v", payload)
	}
	if _, parked := server.pendingMixTickForConversation("conv-d3b"); parked {
		t.Fatal("no pending candidate may be parked by an aligned check")
	}
	server.mu.Lock()
	annotation := server.mixWorkflowQueueNodes["D3"]
	server.mu.Unlock()
	if annotation.State != MixWorkflowNodeInProgress {
		t.Fatalf("an executed aligned check lights the node in_progress, got %s", annotation.State)
	}
}

func TestD3RefAlignCheckUnmeasurableStillRecordsAndLights(t *testing.T) {
	server, snapshotDir := d3RefTestServer(t, d3RefAlignedRows())
	req := ChatRequest{ConversationID: "conv-d3c", Message: "d3 参考对齐"}
	goal := agentruntime.Goal{GoalID: "goal-d3c", RunID: "run-d3c"}
	response := server.handleD3RefAlignCheck(context.Background(), "conv-d3c", req, goal, "sess-d3-1", nil, d3RefReport("obs-d3-3"), rlm.Projection{})

	payload, _ := response.WorkflowData["d3_ref_align"].(map[string]any)
	if payload["verdict"] != d3RefAlignVerdictUnmeasurable {
		t.Fatalf("expected unmeasurable payload, got %+v", payload)
	}
	if payload["action_face"] != d3RefAlignActionDisclosureOnly {
		t.Fatalf("unmeasurable must stay disclosure-only: %+v", payload["action_face"])
	}
	if response.NeedsConfirmation || len(response.InteractionRequests) > 0 {
		t.Fatalf("unmeasurable must park no card: %+v", response.InteractionRequests)
	}
	data, err := os.ReadFile(filepath.Join(snapshotDir, "ref_align_verdicts.json"))
	if err != nil {
		t.Fatalf("unmeasurable check must still record a verdict: %v", err)
	}
	store := d3RefAlignVerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if len(store.Verdicts) != 1 || store.Verdicts[0].VerdictKind != d3RefAlignVerdictUnmeasurable {
		t.Fatalf("unexpected verdict store: %+v", store)
	}
	server.mu.Lock()
	annotation := server.mixWorkflowQueueNodes["D3"]
	server.mu.Unlock()
	if annotation.State != MixWorkflowNodeInProgress {
		t.Fatalf("an executed check lights the node even when unmeasurable, got %s", annotation.State)
	}
}

func TestD3RefAlignCheckHandoverSeamInsideObservationPack(t *testing.T) {
	// The D1-D3 observation handler hands over to the ref-align face only on
	// the explicit check intent; the D1/D2 faces keep their own branches.
	if !d3RefAlignIntent("帮我做个参考对齐") {
		t.Fatal("ref-align intent message must route to the D3 face")
	}
	if d1LoudnessSettleIntent("d3 参考对齐") || d2StereoCheckIntent("d3 参考对齐") {
		t.Fatal("a D3 request must not trigger the D1/D2 faces")
	}
	if d3RefAlignIntent("全局响度检查") || d3RefAlignIntent("d2 翻译检查") {
		t.Fatal("D1/D2 requests must not trigger the D3 face")
	}
}

func TestD3RefAlignCheckPhrasesStayOffCapabilityEntryWordList(t *testing.T) {
	// 入口基线（263 教训：先测裸话术，能过不建词表）：检查请求词不进
	// inferProjectAwareCapability 入口词表——它们经 semantic_entry 仲裁；
	// 只有显式 d 标记进入观察 pack（M1-4 语义不变）。
	for _, phrase := range []string{"参考对齐", "参考曲对比", "对齐检查", "帮我看看这首歌跟 spotify 的标准差多少", "reference check"} {
		if got := inferProjectAwareCapability(phrase); got != "" {
			t.Fatalf("phrase %q must stay off the capability entry word list, got %q", phrase, got)
		}
	}
	for _, phrase := range []string{"d3 参考对齐 spotify", "pre_master.observation d3"} {
		if got := inferProjectAwareCapability(phrase); got != premasterObservationCapabilityID {
			t.Fatalf("phrase %q routed to %q, want %s", phrase, got, premasterObservationCapabilityID)
		}
	}
}
