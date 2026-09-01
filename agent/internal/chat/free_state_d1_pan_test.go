package chat

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/orchestration"
)

func panTestProposal() *agentprotocol.ImprovementProposal {
	return &agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "vocal"}, EvidenceRefs: []string{"obs-before"},
		ImprovementIntent: "rebalance the off-center stereo image", Hypothesis: "a bounded pan move may restore left-right balance",
		ExpectedEffect: "balanced stereo image without a level change", ActionDomain: agentprotocol.ImprovementActionDomainPan,
		ActionKind: d1PanKind, ParameterBounds: map[string]any{"delta_pan": 0.1},
		VerificationPlan: map[string]any{"view_ids": []any{"track.stereo_space"}, "experiment_budget": 1}, Confidence: 0.6,
	}
}

func d1PanLoopForTest(t *testing.T, revision string) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1-pan", ConversationID: "conversation-d1-pan", GoalID: "goal-d1-pan", RunID: "run-d1-pan", Status: "awaiting_experiment", OriginalIntent: "improve the stereo balance", LatestObservation: d1FreshObservationForTest(revision), CreatedAt: now, UpdatedAt: now}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: panTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return loop
}

// The generalized native admission branch must keep the track_gain row's
// construction byte-identical (its AdmissionValueKey is delta_db, so the
// generalization has to reproduce the historical map exactly, aliases
// included).
func TestTrackGainAdmissionShapeStaysByteIdentical(t *testing.T) {
	loop := d1LoopForTest(t, "7")
	admission, err := freeStateExperimentAdmission(loop, experimentTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	wantTyped := map[string]any{"action_domain": "track_gain", "action_kind": "track_gain_adjust", "target_db": nil, "delta_db": -1.0}
	if !reflect.DeepEqual(admission.TypedAction, wantTyped) {
		t.Fatalf("track_gain typed action drifted: %+v", admission.TypedAction)
	}
	wantBounds := map[string]any{"source": "proposal", "bounds": map[string]any{"delta_db": -1.0}, "delta_db": -1.0, "max_action_attempts": 1}
	if !reflect.DeepEqual(admission.DiagnosticDoseBounds, wantBounds) || !reflect.DeepEqual(admission.RetainedDoseBounds, wantBounds) {
		t.Fatalf("track_gain dose bounds drifted: diagnostic=%+v retained=%+v", admission.DiagnosticDoseBounds, admission.RetainedDoseBounds)
	}
	// The legacy alias chain stays on the delta_db row only.
	alias := experimentTestProposal()
	alias.ParameterBounds = map[string]any{"db_delta": -0.5}
	aliasAdmission, err := freeStateExperimentAdmission(loop, alias)
	if err != nil {
		t.Fatal(err)
	}
	if aliasAdmission.TypedAction["delta_db"] != -0.5 {
		t.Fatalf("track_gain legacy alias extraction drifted: %+v", aliasAdmission.TypedAction)
	}
}

func TestPanAdmissionRidesGeneralizedNativeBranch(t *testing.T) {
	loop := d1PanLoopForTest(t, "7")
	admission := loop.Experiment.Admission
	if !admission.IsD1S1() {
		t.Fatalf("pan admission is not D1-S1: %+v", admission)
	}
	wantTyped := map[string]any{"action_domain": "pan", "action_kind": "track_pan_adjust", "target_pan": nil, "delta_pan": 0.1}
	if !reflect.DeepEqual(admission.TypedAction, wantTyped) {
		t.Fatalf("pan typed action shape: %+v", admission.TypedAction)
	}
	wantBounds := map[string]any{"source": "proposal", "bounds": map[string]any{"delta_pan": 0.1}, "delta_pan": 0.1, "max_action_attempts": 1}
	if !reflect.DeepEqual(admission.DiagnosticDoseBounds, wantBounds) || !reflect.DeepEqual(admission.RetainedDoseBounds, wantBounds) {
		t.Fatalf("pan dose bounds: diagnostic=%+v retained=%+v", admission.DiagnosticDoseBounds, admission.RetainedDoseBounds)
	}
	if err := admission.ValidateD1S1(); err != nil {
		t.Fatalf("validated pan admission rejected: %v", err)
	}
}

func TestD1S1PanPlanContainsOneBoundedAction(t *testing.T) {
	loop := d1PanLoopForTest(t, "7")
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "pan": 0.05}}}
	plan, err := d1TrackPanPlan(loop, agentloop.PendingMixTickCandidate{Operation: d1PanKind, TrackID: "vocal", DeltaPan: 0.1}, 7, "project-1", "epoch-1", "snapshot-7", state)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ActionSet.Actions) != 1 {
		t.Fatalf("plan=%+v", plan)
	}
	action := plan.ActionSet.Actions[0]
	if action.Command != d1PanKind || action.TargetRef != "vocal" || !strings.HasSuffix(action.ID, "_pan") {
		t.Fatalf("action=%+v", action)
	}
	if action.Args["delta_pan"] != 0.1 {
		t.Fatalf("action args=%+v", action.Args)
	}
	if targetPan, ok := action.Args["target_pan"].(float64); !ok || math.Abs(targetPan-0.15) > 1e-9 {
		t.Fatalf("action args=%+v", action.Args)
	}
	if action.BeforeFingerprint != "track:vocal:pan:fader_pan:pending" {
		t.Fatalf("before fingerprint=%q", action.BeforeFingerprint)
	}
	if !containsStringFold(plan.ProjectCut.ContractVersions, "action:track_pan_adjust") || plan.ProjectCut.BaseProjectRevision != "7" || plan.PreviousObservationID != "obs-before" {
		t.Fatalf("plan cut=%+v prev=%q", plan.ProjectCut, plan.PreviousObservationID)
	}
	// The pan admission must never ride the track_gain plan builder: its kind
	// guard rejects the pan candidate (the dispatch-arm RED at plan level).
	if _, err = d1TrackGainPlan(loop, agentloop.PendingMixTickCandidate{Operation: d1PanKind, TrackID: "vocal", DeltaDB: -1}, 7, "project-1", "epoch-1", "snapshot-7", state); err == nil {
		t.Fatal("pan admission rode the track_gain plan builder")
	}
	if _, err = d1TrackPanPlan(loop, agentloop.PendingMixTickCandidate{Operation: d1PanKind, TrackID: "other", DeltaPan: 0.1}, 7, "project-1", "epoch-1", "snapshot-7", map[string]any{"tracks": []any{map[string]any{"track_id": "other", "pan": 0.0}}}); err == nil {
		t.Fatal("unobserved target accepted")
	}
	if _, err = d1TrackPanPlan(loop, agentloop.PendingMixTickCandidate{Operation: d1PanKind, TrackID: "vocal", DeltaPan: 0.2}, 7, "project-1", "epoch-1", "snapshot-7", state); err == nil {
		t.Fatal("out-of-bounds candidate delta accepted")
	}
}

func TestD1S1PanDispatchRoutesOwnChain(t *testing.T) {
	s := New(nil, nil, nil)
	// The live-track validation consults the harness user state; without a
	// wired stack the target track cannot be found there, so clear the
	// constructor's default harness and let the pan arm surface its own
	// dependency boundary instead.
	s.harness = nil
	loop := d1PanLoopForTest(t, "7")
	s.storeFreeStateLoop(loop)
	candidate := agentloop.PendingMixTickCandidate{Operation: d1PanKind, TrackID: "vocal", ObservationID: "obs-before", DeltaPan: 0.1, Status: "pending_confirmation"}
	s.storePendingMixTickCandidate(loop.ConversationID, loop.GoalID, loop.RunID, candidate)
	response := s.executePendingMixTickCandidate(context.Background(), loop.ConversationID, ChatRequest{ConversationID: loop.ConversationID, Message: "确认执行"}, "chat", candidate)
	if response.Workflow != "free_state_d1_s1" || response.StopReason != "d1_execution_blocked" {
		t.Fatalf("pan dispatch response=%+v", response)
	}
	// Without kernel wiring both native arms return the same blocked body, so
	// the durable settle reason is the chain-identity observable: a missing pan
	// arm falls through to executeD1TrackGain and settles the track_gain chain.
	for _, durable := range s.pendingManager.Snapshot() {
		if durable.Source.LegacyKind != "PendingMixTickCandidate" {
			continue
		}
		reason := firstStringFromMap(durable.Source.Metadata, "transition_reason")
		if !strings.Contains(reason, "pan chain") || strings.Contains(reason, "track_gain chain") {
			t.Fatalf("pan candidate settled by the wrong chain: %q", reason)
		}
		return
	}
	t.Fatal("durable pending mix tick candidate was not settled")
}

func TestD1S1PanJournalRecordsSetPan(t *testing.T) {
	spec, ok := experiment.D1S1SpecForAction(d1PanDomain, d1PanKind)
	if !ok {
		t.Fatal("pan domain spec missing from the experiment table")
	}
	inner := &d1MutationPortForTest{}
	log := &d1JournalForTest{}
	port := &d1JournalMutationPort{inner: inner, harness: log, goalID: "goal", runID: "run", journalSpec: &spec}
	action := orchestration.Action{ID: "d1-pan-action", Command: d1PanKind, TargetRef: "vocal", Args: map[string]any{"delta_pan": 0.1, "target_pan": 0.15}}
	if _, err := port.Apply(context.Background(), action, "execution:d1-pan-action"); err != nil {
		t.Fatal(err)
	}
	if _, err := port.Apply(context.Background(), action, "execution:d1-pan-action"); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 || len(log.recorded) != 1 || log.results != 2 {
		t.Fatalf("inner=%d journal=%+v results=%d", inner.calls, log.recorded, log.results)
	}
	recorded := log.recorded[0]
	if recorded.Tool != "track_pan_adjust" || recorded.CommandName != "track_pan_adjust" || recorded.Command["cmd"] != "set_pan" || recorded.Command["track_id"] != "vocal" {
		t.Fatalf("journal=%+v", recorded)
	}
	// The pan value is a raw numeric payload like db/value, not a string-coerced
	// identity key.
	if recorded.Command["pan"] != 0.15 {
		t.Fatalf("journal pan payload=%#v", recorded.Command["pan"])
	}
}
