package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/lowendrelation"
	"vit-daw-agent/internal/orchestration"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestLowEndRelationBlockersNeedBandAnalysis(t *testing.T) {
	if !lowEndRelationBlockersNeedBandAnalysis([]string{"other_blocker", "mom_low_end_band_occupancy"}) {
		t.Fatal("expected band occupancy blocker to be detected")
	}
	if lowEndRelationBlockersNeedBandAnalysis([]string{"unrelated_blocker"}) {
		t.Fatal("unrelated blocker must not trigger band analysis kickoff")
	}
	if lowEndRelationBlockersNeedBandAnalysis(nil) {
		t.Fatal("nil blockers must not trigger band analysis kickoff")
	}
}

// TestLowEndRelationTriggerBandAnalysisResponseStartsAnalysisInsteadOfDeadEnd
// exercises the fix for the real-world dead end: B4 was permanently blocked on
// missing band_energy for existing projects because nothing ever calls
// project.audio_analysis_start automatically, and the LLM tool-call guard
// (messageLoopIsDADAnalysisControlTool) prevents the agent from starting it
// itself. This path calls it directly via harness.Invoke, bypassing that
// guard, and must turn a static "blocked" reply into a "kicked off analysis,
// ask again shortly" reply.
func TestLowEndRelationTriggerBandAnalysisResponseStartsAnalysisInsteadOfDeadEnd(t *testing.T) {
	kernel := &fakeLowEndRelationKernel{replies: []map[string]any{
		{"status": "ok", "analysis_job_id": "job_1"},
	}}
	s := &Server{harness: harness.NewWithSender(kernel, nil, nil)}
	planned := capabilityadapters.LowEndRelationPlanResult{
		Outcome: orchestration.CapabilityOutcome{Blockers: []string{"mom_low_end_band_occupancy"}},
		Bundle:  orchestration.ContextBundle{ID: "bundle_1"},
		Pack: capabilitycontext.LowEndRelationPack{
			Readiness: capabilityruntime.Readiness{CanProceed: false},
		},
	}

	resp := s.lowEndRelationTriggerBandAnalysisResponse(context.Background(), "conv_1", agentruntime.Goal{GoalID: "goal_1"}, "session_1", planned)

	if resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("expected completed status (not a dead-end failure), got %+v", resp)
	}
	if !strings.Contains(resp.Reply, "已在后台启动") {
		t.Fatalf("expected reply to describe the background analysis kickoff, got %q", resp.Reply)
	}
	if resp.WorkflowData["canary_stage"] != "band_analysis_triggered" {
		t.Fatalf("expected canary_stage=band_analysis_triggered, got %+v", resp.WorkflowData)
	}
	if len(kernel.commands) != 1 || firstStringFromMap(kernel.commands[0], "cmd") != "project.audio_analysis_start" {
		t.Fatalf("expected a single project.audio_analysis_start kernel command, got %+v", kernel.commands)
	}
	if kernel.commands[0]["retry_missing"] != true || kernel.commands[0]["rebuild_from_project"] != true {
		t.Fatalf("expected retry_missing+rebuild_from_project so already-ready tracks are skipped, got %+v", kernel.commands[0])
	}
}

func TestLowEndRelationTriggerBandAnalysisResponseReportsKernelFailure(t *testing.T) {
	kernel := &fakeLowEndRelationKernel{replies: []map[string]any{
		{"status": "error", "message": "kernel unavailable"},
	}}
	s := &Server{harness: harness.NewWithSender(kernel, nil, nil)}
	planned := capabilityadapters.LowEndRelationPlanResult{
		Outcome: orchestration.CapabilityOutcome{Blockers: []string{"mom_low_end_band_occupancy"}},
		Bundle:  orchestration.ContextBundle{ID: "bundle_1"},
	}

	resp := s.lowEndRelationTriggerBandAnalysisResponse(context.Background(), "conv_1", agentruntime.Goal{GoalID: "goal_1"}, "session_1", planned)

	if resp.WorkflowData["canary_stage"] != "band_analysis_trigger_failed" {
		t.Fatalf("expected canary_stage=band_analysis_trigger_failed, got %+v", resp.WorkflowData)
	}
	if !strings.Contains(resp.Reply, "启动后台音频分析也失败了") {
		t.Fatalf("expected reply to surface the kernel failure, got %q", resp.Reply)
	}
}

// TestLowEndRelationCanaryAnalysisResponseSurfacesConcreteEvidence guards the
// UX fix: the analysis-stage reply used to be a single aggregate-count
// sentence ("B4 analyzed N low-end tracks: N conflicts, tendency=X") with no
// way for the user to see which tracks were actually involved. The reply must
// now also name the dominant sub/bass tracks and the conflicting track pairs.
func TestLowEndRelationCanaryAnalysisResponseSurfacesConcreteEvidence(t *testing.T) {
	planned := capabilityadapters.LowEndRelationPlanResult{
		Outcome: orchestration.CapabilityOutcome{
			Kind:    orchestration.OutcomeAnalysis,
			Summary: "B4 分析了 2 条低频相关轨道：1 处冲突，低频倾向：明显偏多",
		},
		Bundle: orchestration.ContextBundle{ID: "bundle_1"},
		Pack: capabilitycontext.LowEndRelationPack{
			Summary: lowendrelation.LowEndSummary{
				LowEndTendency:      "prominent",
				DominantSubTrackID:  "1012",
				DominantBassTrackID: "1007",
				ConflictCount:       1,
			},
			Tracks: []lowendrelation.LowEndTrack{
				{TrackID: "1007", TrackName: "Sub Bass Tone"},
				{TrackID: "1012", TrackName: "Secondary A"},
			},
			Conflicts: []lowendrelation.LowEndConflict{
				{
					Band:       "sub",
					Confidence: "low_to_medium",
					Tracks: []map[string]any{
						{"track_id": "1007"},
						{"track_id": "1012"},
					},
				},
			},
			Observations: []lowendrelation.Observation{
				{ID: "obs_tendency_1", Kind: "low_end_prominent", Summary: "sub and bass bands are both dominant across the project mix"},
				{ID: "obs_conflict_1", Kind: "sub_masking_conflict", Summary: "sub band: 2 tracks have close relative energy — masking conflict candidate"},
			},
		},
	}

	resp := lowEndRelationCanaryAnalysisResponse("conv_1", agentruntime.Goal{GoalID: "goal_1"}, "session_1", planned, orchestration.ContextEnvelope{})

	for _, want := range []string{"Secondary A", "Sub Bass Tone", "sub 频段冲突", "低到中", "低频总能量相对其他频段明显偏多"} {
		if !strings.Contains(resp.Reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, resp.Reply)
		}
	}
	if strings.Count(resp.Reply, "Sub Bass Tone vs Secondary A") + strings.Count(resp.Reply, "Secondary A vs Sub Bass Tone") != 1 {
		t.Fatalf("expected the conflict pair to be named exactly once, got:\n%s", resp.Reply)
	}
	if strings.Contains(resp.Reply, "masking conflict candidate") {
		t.Fatalf("expected the raw English masking-conflict observation summary to be deduplicated against the named conflict line:\n%s", resp.Reply)
	}
}

type fakeLowEndRelationKernel struct {
	replies  []map[string]any
	commands []map[string]any
}

func (f *fakeLowEndRelationKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	f.commands = append(f.commands, command)
	if len(f.replies) == 0 {
		return map[string]any{"status": "ok"}, "{}", nil
	}
	reply := f.replies[0]
	f.replies = f.replies[1:]
	return reply, "{}", nil
}
