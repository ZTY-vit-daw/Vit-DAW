package chat

// AF-D1-LOUDNESS-SETTLE-1: D1 loudness settle face tests — explicit check
// records a verdict, a breach parks a bounded pending-confirmation candidate
// (zero mutation before confirmation), the D1 queue node leaves not_started,
// and pass/unmeasurable outcomes do not invent action work.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/loudnesswatch"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/projectstore"
	agentruntime "vit-daw-agent/internal/runtime"
)

func d1SettleTestServer(t *testing.T, rows []any, watchConfig *loudnesswatch.Config) (*Server, string) {
	t.Helper()
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	projectRoot := t.TempDir()
	if _, _, err := projectstore.Activate(filepath.Join(projectRoot, "song.vitproj"), "proj-d1"); err != nil {
		t.Fatalf("activate projectstore: %v", err)
	}
	server := &Server{
		harness:         harness.New(nil, nil, nil),
		pendingMixTicks: map[string]agentloop.PendingMixTickCandidate{},
		interactions:    map[string]PendingInteraction{},
	}
	if watchConfig != nil {
		config := *watchConfig
		server.loudnessWatchConfig = &config
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

func d1SettleBreachRows() []any {
	return []any{
		map[string]any{"track_id": "t1", "clip_id": "c1", "integrated_lufs": -8.0, "sample_peak_dbfs": -3.0, "status": "ready", "quality_status": "ready"},
		map[string]any{"track_id": "t2", "clip_id": "c2", "integrated_lufs": -15.0, "sample_peak_dbfs": -3.0, "status": "ready", "quality_status": "ready"},
	}
}

func d1SettlePassRows() []any {
	return []any{
		map[string]any{"track_id": "t1", "clip_id": "c1", "integrated_lufs": -15.0, "sample_peak_dbfs": -3.0, "status": "ready", "quality_status": "ready"},
	}
}

func d1SettleReport(observationID string) d1d3PreMasterReport {
	peak, rms := -0.4, -12.2
	return d1d3PreMasterReport{SchemaVersion: d1d3ObservationSchema, ObservationID: observationID, LoudnessPeakStatus: "present", GlobalPeakDBFS: &peak, GlobalRMSDBFS: &rms}
}

func TestD1LoudnessSettleIntentWords(t *testing.T) {
	for _, phrase := range []string{"全局响度检查一下", "做个响度检查", "峰值检查", "母带前响度看看", "global loudness check"} {
		if !d1LoudnessSettleIntent(phrase) {
			t.Fatalf("expected settle intent for %q", phrase)
		}
	}
	for _, phrase := range []string{"低频关系分析", "参考曲对比", "响度这个词单独出现不算检查意图"} {
		if d1LoudnessSettleIntent(phrase) {
			t.Fatalf("unexpected settle intent for %q", phrase)
		}
	}
}

func TestD1LoudnessSettleBreachParksPendingCandidateAndLightsNode(t *testing.T) {
	server, snapshotDir := d1SettleTestServer(t, d1SettleBreachRows(), nil)
	req := ChatRequest{ConversationID: "conv-d1", Message: "全局响度检查"}
	goal := agentruntime.Goal{GoalID: "goal-d1", RunID: "run-d1"}
	response := server.handleD1LoudnessSettle(context.Background(), "conv-d1", req, goal, "sess-d1-1", nil, d1SettleReport("obs-d1-1"))

	if response.StopReason != "d1_loudness_settled" {
		t.Fatalf("expected stop_reason d1_loudness_settled, got %s", response.StopReason)
	}
	if response.NeedsConfirmation != true || len(response.InteractionRequests) == 0 {
		t.Fatalf("a breach must park a confirmation card: needs=%v cards=%d", response.NeedsConfirmation, len(response.InteractionRequests))
	}
	if response.WorkflowData["mutation_performed"] != false {
		t.Fatalf("settle face must not mutate the project: %+v", response.WorkflowData["mutation_performed"])
	}
	settle, ok := response.WorkflowData["d1_settle"].(map[string]any)
	if !ok || settle["verdict"] != loudnesswatch.VerdictBreach {
		t.Fatalf("expected breach verdict in workflow data, got %+v", response.WorkflowData["d1_settle"])
	}
	if settle["config_basis"] == nil || settle["config_basis"] == "" {
		t.Fatalf("config basis must be disclosed")
	}
	if suggestion, ok := settle["suggestion"].(map[string]any); !ok || suggestion["track_id"] != "t1" {
		t.Fatalf("expected a suggestion targeting the worst-breach track t1, got %+v", settle["suggestion"])
	}

	// The pending candidate is parked through the governed chain (zero mutation).
	candidate, parked := server.pendingMixTickForConversation("conv-d1")
	if !parked {
		t.Fatal("expected a parked pending mix tick candidate")
	}
	if candidate.Operation != loudnesswatch.ActionOperationGainAdjust || candidate.TrackID != "t1" {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
	if candidate.DeltaDB >= 0 {
		t.Fatalf("a ceiling breach must suggest pulling down, got delta %v", candidate.DeltaDB)
	}

	// The verdict landed in the store under the d1_settle trigger.
	data, err := os.ReadFile(filepath.Join(snapshotDir, "loudness_watch_verdicts.json"))
	if err != nil {
		t.Fatalf("verdict store missing: %v", err)
	}
	store := loudnesswatch.VerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if store.ChecksTotal != 1 || len(store.Verdicts) != 1 || store.Verdicts[0].TriggerKind != loudnesswatch.TriggerKindD1Settle {
		t.Fatalf("unexpected verdict store: %+v", store)
	}

	// The D1 queue node left not_started.
	server.mu.Lock()
	annotation, annotated := server.mixWorkflowQueueNodes["D1"]
	server.mu.Unlock()
	if !annotated || annotation.State != MixWorkflowNodeNeedsConfirmation {
		t.Fatalf("expected D1 annotation needs_confirmation, got %+v (%v)", annotation, annotated)
	}
	view := server.BuildMixWorkflowQueueView(context.Background())
	for _, node := range view.Nodes {
		if node.NodeID == "D1" && node.State == MixWorkflowNodeNotStarted {
			t.Fatalf("D1 must not stay not_started after a settle run: %+v", node)
		}
	}
}

func TestD1LoudnessSettlePassLightsNodeWithoutActionCard(t *testing.T) {
	server, _ := d1SettleTestServer(t, d1SettlePassRows(), nil)
	req := ChatRequest{ConversationID: "conv-d1", Message: "响度检查"}
	goal := agentruntime.Goal{GoalID: "goal-d1", RunID: "run-d1"}
	response := server.handleD1LoudnessSettle(context.Background(), "conv-d1", req, goal, "sess-d1-2", nil, d1SettleReport("obs-d1-2"))

	if response.NeedsConfirmation || len(response.InteractionRequests) > 0 {
		t.Fatalf("a pass verdict must not park an action card")
	}
	settle, _ := response.WorkflowData["d1_settle"].(map[string]any)
	if settle["verdict"] != loudnesswatch.VerdictPass {
		t.Fatalf("expected pass verdict, got %+v", settle)
	}
	if _, parked := server.pendingMixTickForConversation("conv-d1"); parked {
		t.Fatal("no pending candidate may exist for a pass verdict")
	}
	server.mu.Lock()
	annotation := server.mixWorkflowQueueNodes["D1"]
	server.mu.Unlock()
	if annotation.State != MixWorkflowNodeInProgress {
		t.Fatalf("expected D1 annotation in_progress, got %s", annotation.State)
	}
}

func TestD1LoudnessSettleUnconfiguredProjectUsesBuiltinDefaultHonestly(t *testing.T) {
	// No watch config anywhere: the settle face still judges (builtin streaming
	// default) and discloses that the config face was not touched.
	server, _ := d1SettleTestServer(t, d1SettleBreachRows(), nil)
	req := ChatRequest{ConversationID: "conv-d1", Message: "全局响度检查"}
	goal := agentruntime.Goal{GoalID: "goal-d1", RunID: "run-d1"}
	response := server.handleD1LoudnessSettle(context.Background(), "conv-d1", req, goal, "sess-d1-3", nil, d1SettleReport("obs-d1-3"))

	settle, _ := response.WorkflowData["d1_settle"].(map[string]any)
	basis, _ := settle["config_basis"].(string)
	if basis == "" || settle["checked"] != true {
		t.Fatalf("builtin-default check must run and disclose its basis: %+v", settle)
	}
	server.mu.Lock()
	configStillNil := server.loudnessWatchConfig == nil
	server.mu.Unlock()
	if !configStillNil {
		t.Fatal("the settle face must never write the watch config face")
	}
}

func TestD1LoudnessSettleRealtimeConfigDisclosesGapWithoutVerdict(t *testing.T) {
	realtime := loudnesswatch.Normalize(loudnesswatch.Config{Enabled: true, TriggerMode: loudnesswatch.TriggerRealtime, ProfileSource: "builtin:spotify"})
	server, snapshotDir := d1SettleTestServer(t, d1SettleBreachRows(), &realtime)
	req := ChatRequest{ConversationID: "conv-d1", Message: "全局响度检查"}
	goal := agentruntime.Goal{GoalID: "goal-d1", RunID: "run-d1"}
	response := server.handleD1LoudnessSettle(context.Background(), "conv-d1", req, goal, "sess-d1-4", nil, d1SettleReport("obs-d1-4"))

	settle, _ := response.WorkflowData["d1_settle"].(map[string]any)
	if settle["checked"] != false {
		t.Fatalf("realtime config must not produce a verdict: %+v", settle)
	}
	if _, err := os.Stat(filepath.Join(snapshotDir, "loudness_watch_verdicts.json")); !os.IsNotExist(err) {
		t.Fatalf("realtime gap must not touch the verdict store")
	}
	if _, parked := server.pendingMixTickForConversation("conv-d1"); parked {
		t.Fatal("no action card may be parked without a verdict")
	}
}

func TestD1LoudnessSettleObservationRoutedThroughPreMasterIntent(t *testing.T) {
	// The D1-D3 observation handler hands over to the settle face when the
	// message carries the settle intent; without it the read-only report
	// stays the answer (unit-level guard for the handover seam).
	if !d1LoudnessSettleIntent("帮我做个全局响度检查") {
		t.Fatal("settle-intent message must route to the settle face")
	}
	if d1LoudnessSettleIntent("d2 翻译检查") {
		t.Fatal("a D2 request must not trigger the D1 settle face")
	}
}

func TestD1LoudnessSettlePhrasesRouteToPreMasterObservation(t *testing.T) {
	// The capability-owner word list admits the explicit check phrases (the
	// settle face hangs off the D1-D3 observation entry); ordinary loudness
	// small talk stays unrouted (M1-4 discipline).
	for _, phrase := range []string{"全局响度检查", "帮我做个响度检查", "峰值检查一下", "d1"} {
		if got := inferProjectAwareCapability(phrase); got != premasterObservationCapabilityID {
			t.Fatalf("phrase %q routed to %q, want %s", phrase, got, premasterObservationCapabilityID)
		}
	}
	for _, phrase := range []string{"这首歌响度感觉不够", "低频有点浑"} {
		if got := inferProjectAwareCapability(phrase); got != "" {
			t.Fatalf("phrase %q must stay unrouted, got %q", phrase, got)
		}
	}
}
