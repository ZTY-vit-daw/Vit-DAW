package chat

// D2-STEREO-THIN-1: D2 stereo check face tests — zero LLM. Covers the intent
// family (explicit check requests only, no bare-topic hijack), the three-state
// criterion (thresholds reused from M1-4's d1d3MonoRisk), the verdict store
// discipline (dedupe / foreign-schema fail-closed / history trim), the handler
// wiring (verdict on disk, D2 node lit, no card, zero mutation), and the
// no-vocabulary entry baseline (check phrases stay off the capability entry
// word list; the explicit d2 stage marker enters the observation pack).

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
	"vit-daw-agent/internal/projectstore"
	agentruntime "vit-daw-agent/internal/runtime"
)

func d2StereoPtr(value float64) *float64 { return &value }

func d2StereoTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	projectRoot := t.TempDir()
	if _, _, err := projectstore.Activate(filepath.Join(projectRoot, "song.vitproj"), "proj-d2"); err != nil {
		t.Fatalf("activate projectstore: %v", err)
	}
	server := &Server{
		harness:         harness.New(nil, nil, nil),
		pendingMixTicks: map[string]agentloop.PendingMixTickCandidate{},
		interactions:    map[string]PendingInteraction{},
	}
	return server, d2StereoVerdictStorePath()
}

func TestD2StereoCheckIntentWords(t *testing.T) {
	for _, phrase := range []string{"d2 翻译检查", "做个翻译检查", "立体声检查一下", "单声道兼容看看", "mono 检查", "帮我查一下 mono 风险", "run a stereo check"} {
		if !d2StereoCheckIntent(phrase) {
			t.Fatalf("expected stereo-check intent for %q", phrase)
		}
	}
	// 裸话题词与相邻能力话术不是 D2 检查请求（M1-4 反门语义；裸话术入口交给
	// semantic_entry 实测，见卡面）。
	for _, phrase := range []string{"立体声有点窄", "单声道", "mono", "相位不对", "响度检查", "全局响度检查", "参考曲对比", "低频关系分析", "d1", "d2"} {
		if d2StereoCheckIntent(phrase) {
			t.Fatalf("unexpected stereo-check intent for %q", phrase)
		}
	}
}

func TestD2StereoAssessRiskFound(t *testing.T) {
	report := d1d3PreMasterReport{
		SchemaVersion: d1d3ObservationSchema, StereoStatus: "present",
		CorrelationEstimate: d2StereoPtr(0.31), BalanceDB: d2StereoPtr(0.4),
		TrackStereo: []d1d3TrackStereoRow{
			{TrackID: "t1", Status: "read", CorrelationEstimate: d2StereoPtr(0.92)},
			{TrackID: "t2", Status: "read", CorrelationEstimate: d2StereoPtr(0.10), MonoRisk: true},
			{TrackID: "t3", Status: "read", PhaseNegativeRatio: d2StereoPtr(0.21), MonoRisk: true},
			{TrackID: "t4", Status: "missing"},
		},
	}
	verdict := d2AssessStereoRisk(report)
	if verdict.Verdict != d2StereoVerdictRiskFound {
		t.Fatalf("expected risk_found, got %s", verdict.Verdict)
	}
	if len(verdict.RiskTracks) != 2 || verdict.RiskTracks[0].TrackID != "t2" || verdict.RiskTracks[1].TrackID != "t3" {
		t.Fatalf("unexpected risk tracks: %+v", verdict.RiskTracks)
	}
	if !strings.Contains(verdict.RiskTracks[0].Basis, "correlation 0.10 < 0.25") {
		t.Fatalf("risk basis must name the pinned threshold: %q", verdict.RiskTracks[0].Basis)
	}
	if !strings.Contains(verdict.RiskTracks[1].Basis, "phase_negative_ratio 0.21 > 0.10") {
		t.Fatalf("risk basis must name the pinned threshold: %q", verdict.RiskTracks[1].Basis)
	}
	if verdict.TracksMeasured != 3 {
		t.Fatalf("expected 3 measured tracks (missing row excluded), got %d", verdict.TracksMeasured)
	}
	if verdict.GlobalCorrelation == nil || *verdict.GlobalCorrelation != 0.31 {
		t.Fatalf("global correlation must be disclosed verbatim: %+v", verdict.GlobalCorrelation)
	}
	if !strings.Contains(verdict.Summary, "2 条 mono 风险轨") || !strings.Contains(verdict.Summary, "不构成翻译合规判定") {
		t.Fatalf("summary must carry the honest qualifier: %q", verdict.Summary)
	}
}

func TestD2StereoAssessNoRiskAndThresholdBoundaries(t *testing.T) {
	// 阈值复用 M1-4：corr 恰为 0.25、neg 恰为 0.1 都不构成风险（严格比较，
	// 与 d1d3MonoRisk / phaseRiskCandidates 同款）。
	report := d1d3PreMasterReport{
		SchemaVersion: d1d3ObservationSchema, StereoStatus: "present",
		TrackStereo: []d1d3TrackStereoRow{
			{TrackID: "t1", Status: "read", CorrelationEstimate: d2StereoPtr(0.25)},
			{TrackID: "t2", Status: "read", PhaseNegativeRatio: d2StereoPtr(0.1)},
			{TrackID: "t3", Status: "read", CorrelationEstimate: d2StereoPtr(0.8), PhaseNegativeRatio: d2StereoPtr(0.02)},
		},
	}
	verdict := d2AssessStereoRisk(report)
	if verdict.Verdict != d2StereoVerdictNoRisk {
		t.Fatalf("expected no_risk at exact boundaries, got %s", verdict.Verdict)
	}
	if verdict.TracksMeasured != 3 || len(verdict.RiskTracks) != 0 {
		t.Fatalf("unexpected counts: measured=%d risks=%d", verdict.TracksMeasured, len(verdict.RiskTracks))
	}
	// 交叉核验：verdict 判据面与 M1-4 的 MonoRisk 标志对同一输入一致。
	for _, row := range report.TrackStereo {
		if row.MonoRisk {
			t.Fatalf("boundary row must not carry M1-4 mono risk: %+v", row)
		}
	}
}

func TestD2StereoAssessUnmeasurableShapes(t *testing.T) {
	cases := []struct {
		name   string
		report d1d3PreMasterReport
		needle string
	}{
		{
			name:   "no evidence at all",
			report: d1d3PreMasterReport{SchemaVersion: d1d3ObservationSchema, StereoStatus: "missing"},
			needle: "no_stereo_evidence",
		},
		{
			name: "rows present but unreadable",
			report: d1d3PreMasterReport{SchemaVersion: d1d3ObservationSchema, StereoStatus: "missing", TrackStereo: []d1d3TrackStereoRow{
				{TrackID: "t1", Status: "missing"}, {TrackID: "t2", Status: "error", Error: "boom"},
			}},
			needle: "no_stereo_evidence",
		},
		{
			name: "global present, track producer absent",
			report: d1d3PreMasterReport{SchemaVersion: d1d3ObservationSchema, StereoStatus: "present", CorrelationEstimate: d2StereoPtr(0.4), TrackStereo: []d1d3TrackStereoRow{
				{TrackID: "t1", Status: "read"},
			}},
			needle: "no_track_stereo_producer",
		},
		{
			name:   "observe error carried honestly",
			report: d1d3PreMasterReport{SchemaVersion: d1d3ObservationSchema, StereoStatus: "missing", ObserveError: "mix.observe: timeout"},
			needle: "mix.observe: timeout",
		},
	}
	for _, testCase := range cases {
		verdict := d2AssessStereoRisk(testCase.report)
		if verdict.Verdict != d2StereoVerdictUnmeasurable {
			t.Fatalf("%s: expected unmeasurable, got %s", testCase.name, verdict.Verdict)
		}
		if !strings.Contains(verdict.UnmeasurableReason, testCase.needle) {
			t.Fatalf("%s: reason %q must carry %q", testCase.name, verdict.UnmeasurableReason, testCase.needle)
		}
	}
}

func TestD2StereoGlobalCorrelationIsDisclosedNotJudged(t *testing.T) {
	// 全局 stereo_relation 的 correlation 只披露不判风险：宽立体声混音全局相关
	// 合法偏低，mono 风险判据面=逐轨（与 project_package phaseRiskCandidates
	// 语义一致）。全局 0.10 且无逐轨测量行 → unmeasurable，不是 risk_found。
	verdict := d2AssessStereoRisk(d1d3PreMasterReport{SchemaVersion: d1d3ObservationSchema, StereoStatus: "present", CorrelationEstimate: d2StereoPtr(0.10)})
	if verdict.Verdict != d2StereoVerdictUnmeasurable {
		t.Fatalf("expected unmeasurable without per-track rows, got %s", verdict.Verdict)
	}
	if verdict.GlobalCorrelation == nil || *verdict.GlobalCorrelation != 0.10 {
		t.Fatalf("global correlation must still be disclosed: %+v", verdict.GlobalCorrelation)
	}
}

func TestD2StereoSkippedTracksStayDisclosed(t *testing.T) {
	report := d1d3PreMasterReport{
		SchemaVersion: d1d3ObservationSchema, StereoStatus: "present",
		TracksSkipped: 3,
		TrackStereo:   []d1d3TrackStereoRow{{TrackID: "t1", Status: "read", CorrelationEstimate: d2StereoPtr(0.9)}},
	}
	verdict := d2AssessStereoRisk(report)
	if verdict.Verdict != d2StereoVerdictNoRisk || verdict.TracksSkipped != 3 {
		t.Fatalf("unexpected verdict: %+v", verdict)
	}
	if !strings.Contains(verdict.Summary, "3 轨超出读取上限未读") {
		t.Fatalf("skipped tracks must stay disclosed in the summary: %q", verdict.Summary)
	}
}

func TestD2StereoVerdictStoreDedupeAndKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stereo_check_verdicts.json")
	now := time.Now()
	base := d2StereoVerdictRecord{
		ProjectUUID: "proj-d2", ProjectRevision: "7", VerdictKind: d2StereoVerdictNoRisk,
		Verdict:     d2StereoVerdict{Verdict: d2StereoVerdictNoRisk, TracksMeasured: 2},
		TriggerKind: d2StereoCheckTriggerKind, RiskTrackCount: 0, TracksMeasured: 2,
	}
	key := recordD2StereoVerdict(path, base, now, func(string, ...any) {})
	if key != "proj-d2|7|no_risk" {
		t.Fatalf("unexpected dedupe key: %q", key)
	}
	recordD2StereoVerdict(path, base, now.Add(time.Minute), func(string, ...any) {})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("store file missing: %v", err)
	}
	store := d2StereoVerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if store.SchemaVersion != d2StereoVerdictStoreSchemaVersion || store.ChecksTotal != 2 {
		t.Fatalf("unexpected store header: %+v", store)
	}
	if len(store.Verdicts) != 1 {
		t.Fatalf("dedupe must not duplicate the recorded verdict: %+v", store.Verdicts)
	}
	if store.Verdicts[0].RecheckCount != 1 || store.Verdicts[0].LastRecheckedAt == "" {
		t.Fatalf("recheck audit must bump: %+v", store.Verdicts[0])
	}
	// A different verdict kind under the same project+revision is a new record.
	risk := base
	risk.VerdictKind = d2StereoVerdictRiskFound
	recordD2StereoVerdict(path, risk, now.Add(2*time.Minute), func(string, ...any) {})
	data, _ = os.ReadFile(path)
	store = d2StereoVerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if len(store.Verdicts) != 2 || store.Verdicts[1].VerdictKind != d2StereoVerdictRiskFound {
		t.Fatalf("expected a second record for the new verdict kind: %+v", store.Verdicts)
	}
}

func TestD2StereoVerdictStoreForeignSchemaFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stereo_check_verdicts.json")
	foreign := map[string]any{"schema_version": "stereo_check_verdicts.v9", "verdicts": []map[string]any{{"key": "old"}}}
	data, _ := json.Marshal(foreign)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	warned := false
	record := d2StereoVerdictRecord{ProjectUUID: "p", VerdictKind: d2StereoVerdictNoRisk, Verdict: d2StereoVerdict{Verdict: d2StereoVerdictNoRisk}, TriggerKind: d2StereoCheckTriggerKind}
	recordD2StereoVerdict(path, record, time.Now(), func(format string, args ...any) { warned = strings.Contains(format, "fail closed") })
	if !warned {
		t.Fatal("a foreign schema must WARN before failing closed")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	store := d2StereoVerdictStoreFile{}
	if err := json.Unmarshal(raw, &store); err != nil {
		t.Fatal(err)
	}
	if store.SchemaVersion != d2StereoVerdictStoreSchemaVersion || len(store.Verdicts) != 1 {
		t.Fatalf("foreign history must restart empty, got %+v", store)
	}
}

func TestD2StereoVerdictStoreTrimsHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stereo_check_verdicts.json")
	now := time.Now()
	for index := 0; index < d2StereoMaxKeptVerdicts+2; index++ {
		record := d2StereoVerdictRecord{
			ProjectUUID: "p", ProjectRevision: fmt.Sprintf("r%d", index), VerdictKind: d2StereoVerdictNoRisk,
			Verdict: d2StereoVerdict{Verdict: d2StereoVerdictNoRisk}, TriggerKind: d2StereoCheckTriggerKind,
		}
		recordD2StereoVerdict(path, record, now.Add(time.Duration(index)*time.Second), func(string, ...any) {})
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	store := d2StereoVerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if len(store.Verdicts) != d2StereoMaxKeptVerdicts {
		t.Fatalf("history must trim to %d, got %d", d2StereoMaxKeptVerdicts, len(store.Verdicts))
	}
	if store.Verdicts[0].ProjectRevision != "r2" {
		t.Fatalf("trim must keep the tail, oldest surviving revision=%q", store.Verdicts[0].ProjectRevision)
	}
}

func TestD2StereoCheckHandlerLightsNodeWithoutActionCard(t *testing.T) {
	server, storePath := d2StereoTestServer(t)
	req := ChatRequest{ConversationID: "conv-d2", Message: "d2 翻译检查"}
	goal := agentruntime.Goal{GoalID: "goal-d2", RunID: "run-d2"}
	report := d1d3PreMasterReport{
		SchemaVersion: d1d3ObservationSchema, ObservationID: "obs-d2-1", StereoStatus: "present",
		TrackStereo: []d1d3TrackStereoRow{
			{TrackID: "t1", Status: "read", CorrelationEstimate: d2StereoPtr(0.93)},
			{TrackID: "t2", Status: "read", CorrelationEstimate: d2StereoPtr(0.07), MonoRisk: true},
		},
	}
	response := server.handleD2StereoCheck(context.Background(), "conv-d2", req, goal, "sess-d2-1", nil, report)

	if response.StopReason != "d2_stereo_checked" {
		t.Fatalf("expected stop_reason d2_stereo_checked, got %s", response.StopReason)
	}
	if response.NeedsConfirmation || len(response.InteractionRequests) > 0 {
		t.Fatalf("v1 stereo check must park no card: needs=%v cards=%d", response.NeedsConfirmation, len(response.InteractionRequests))
	}
	if response.WorkflowData["mutation_performed"] != false {
		t.Fatalf("stereo check must not mutate the project: %+v", response.WorkflowData["mutation_performed"])
	}
	payload, ok := response.WorkflowData["d2_stereo_check"].(map[string]any)
	if !ok || payload["verdict"] != d2StereoVerdictRiskFound {
		t.Fatalf("expected risk_found payload, got %+v", response.WorkflowData["d2_stereo_check"])
	}
	if payload["action_face"] != "disclosure_only_v1" || payload["action_face_reason"] == nil {
		t.Fatalf("action-face boundary must be disclosed: %+v", payload)
	}
	if _, parked := server.pendingMixTickForConversation("conv-d2"); parked {
		t.Fatal("no pending candidate may be parked by the stereo check face")
	}

	// The verdict landed in the store under the d2_stereo_check trigger.
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("verdict store missing at %s: %v", storePath, err)
	}
	store := d2StereoVerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if store.ChecksTotal != 1 || len(store.Verdicts) != 1 || store.Verdicts[0].TriggerKind != d2StereoCheckTriggerKind {
		t.Fatalf("unexpected verdict store: %+v", store)
	}
	if store.Verdicts[0].Verdict.Verdict != d2StereoVerdictRiskFound || store.Verdicts[0].RiskTrackCount != 1 {
		t.Fatalf("unexpected verdict record: %+v", store.Verdicts[0])
	}

	// The D2 queue node left not_started (in_progress; no card → not needs_confirmation).
	server.mu.Lock()
	annotation, annotated := server.mixWorkflowQueueNodes["D2"]
	server.mu.Unlock()
	if !annotated || annotation.State != MixWorkflowNodeInProgress {
		t.Fatalf("expected D2 annotation in_progress, got %+v (%v)", annotation, annotated)
	}
	view := server.BuildMixWorkflowQueueView(context.Background())
	for _, node := range view.Nodes {
		if node.NodeID == "D2" && node.State == MixWorkflowNodeNotStarted {
			t.Fatalf("D2 must not stay not_started after a stereo check run: %+v", node)
		}
	}
	if !strings.Contains(response.Reply, "风险轨 t2") {
		t.Fatalf("reply must name the risk track: %q", response.Reply)
	}
	if !strings.Contains(response.Reply, "不生成写操作确认卡") {
		t.Fatalf("reply must disclose the action-face boundary: %q", response.Reply)
	}
}

func TestD2StereoCheckUnmeasurableStillRecordsAndLights(t *testing.T) {
	server, storePath := d2StereoTestServer(t)
	req := ChatRequest{ConversationID: "conv-d2b", Message: "d2 翻译检查"}
	goal := agentruntime.Goal{GoalID: "goal-d2b", RunID: "run-d2b"}
	report := d1d3PreMasterReport{SchemaVersion: d1d3ObservationSchema, StereoStatus: "missing"}
	response := server.handleD2StereoCheck(context.Background(), "conv-d2b", req, goal, "sess-d2-1", nil, report)

	payload, _ := response.WorkflowData["d2_stereo_check"].(map[string]any)
	if payload["verdict"] != d2StereoVerdictUnmeasurable {
		t.Fatalf("expected unmeasurable payload, got %+v", payload)
	}
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("unmeasurable check must still record a verdict: %v", err)
	}
	store := d2StereoVerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if len(store.Verdicts) != 1 || store.Verdicts[0].VerdictKind != d2StereoVerdictUnmeasurable {
		t.Fatalf("unexpected verdict store: %+v", store)
	}
	server.mu.Lock()
	annotation := server.mixWorkflowQueueNodes["D2"]
	server.mu.Unlock()
	if annotation.State != MixWorkflowNodeInProgress {
		t.Fatalf("an executed check lights the node even when unmeasurable, got %s", annotation.State)
	}
}

func TestD2StereoCheckHandoverSeamInsideObservationPack(t *testing.T) {
	// The D1-D3 observation handler hands over to the stereo face only on the
	// explicit check intent; the D1 settle face keeps its own branch.
	if !d2StereoCheckIntent("帮我做个翻译检查") {
		t.Fatal("stereo-check intent message must route to the stereo face")
	}
	if d1LoudnessSettleIntent("d2 翻译检查") {
		t.Fatal("a D2 request must not trigger the D1 settle face")
	}
	if d2StereoCheckIntent("全局响度检查") {
		t.Fatal("a D1 request must not trigger the D2 stereo face")
	}
}

func TestD2StereoCheckPhrasesStayOffCapabilityEntryWordList(t *testing.T) {
	// 入口基线（263 教训：先测裸话术，能过不建词表）：检查请求词不进
	// inferProjectAwareCapability 入口词表——它们经 semantic_entry 仲裁；
	// 只有显式 d 标记进入观察 pack（M1-4 语义不变）。
	for _, phrase := range []string{"翻译检查", "立体声检查", "帮我检查一下这首歌用单声道播放会不会丢内容", "mono check"} {
		if got := inferProjectAwareCapability(phrase); got != "" {
			t.Fatalf("phrase %q must stay off the capability entry word list, got %q", phrase, got)
		}
	}
	for _, phrase := range []string{"d2 翻译检查", "pre_master.observation d2"} {
		if got := inferProjectAwareCapability(phrase); got != premasterObservationCapabilityID {
			t.Fatalf("phrase %q routed to %q, want %s", phrase, got, premasterObservationCapabilityID)
		}
	}
}
