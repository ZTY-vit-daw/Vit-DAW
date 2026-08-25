package chat

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/executionruntime"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/orchestration"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

func d1LoopForTest(t *testing.T, revision string) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1", ConversationID: "conversation-d1", GoalID: "goal-d1", RunID: "run-d1", Status: "awaiting_experiment", OriginalIntent: "inspect and improve the mix", LatestObservation: d1FreshObservationForTest(revision), CreatedAt: now, UpdatedAt: now}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return loop
}

func TestD1S1AdmissionBindsFreshObservedTarget(t *testing.T) {
	loop := d1LoopForTest(t, "7")
	if loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		t.Fatalf("experiment=%+v", loop.Experiment)
	}
	stale := loop
	stale.Experiment = nil
	stale.LatestObservation = d1FreshObservationForTest("7")
	stale.LatestObservation.Summary["audit_receipt"].(map[string]any)["freshness"] = map[string]any{"status": "stale"}
	if _, err := freeStateExperimentAdmission(stale, experimentTestProposal()); err == nil || !strings.Contains(err.Error(), "explicitly fresh") {
		t.Fatalf("stale admission error=%v", err)
	}
	mismatch := loop
	mismatch.Experiment = nil
	mismatch.LatestObservation = d1FreshObservationForTest("7")
	mismatch.LatestObservation.Summary["target_ref"] = map[string]any{"kind": "track", "id": "other"}
	if _, err := freeStateExperimentAdmission(mismatch, experimentTestProposal()); err == nil || !strings.Contains(err.Error(), "must match") {
		t.Fatalf("target mismatch error=%v", err)
	}
}

func TestD1S1AdmissionRequiresTargetObservationID(t *testing.T) {
	loop := d1LoopForTest(t, "7")
	proposal := experimentTestProposal()
	proposal.EvidenceRefs = []string{"receipt-before"}
	if _, err := freeStateExperimentAdmission(loop, proposal); err == nil || !strings.Contains(err.Error(), "observation ID") {
		t.Fatalf("receipt-only proposal was admitted: err=%v", err)
	}
}

func TestFreeStateAdmissionBoundaryReceiptRetainsCandidateEvidenceAndGates(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-admission-boundary", ConversationID: "conversation-admission-boundary",
		ProjectUUID: "project-1", ProjectRevision: "7", OriginalIntent: "improve the mix",
		Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "project", ID: "project-1"}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = (audioclosure.Driver{}).AdmitRound(state, state.Revision, now)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = (audioclosure.Driver{}).UpdateFrontier(state, state.Revision, audioclosure.HypothesisFrontier{CandidateID: "candidate:frontier-1", Candidates: []audioclosure.Candidate{{ID: "candidate:frontier-1", TrackIDs: []string{"1007"}, SourceObservationID: "obs-target", EvidenceRefs: []string{"obs-target"}}}}, audioclosure.ActionabilityUnknown, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-admission-boundary", ConversationID: state.ConversationID,
		Status: "observing", OriginalIntent: "improve the mix", ActiveIntent: "improve the mix",
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now,
	})
	loop, ok := s.recordFreeStateDecision(state.ConversationID, agentloop.Result{
		FreeStateDecision: &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateCapabilityBlocked,
			EvidenceStatus: "insufficient", Summary: "no legal improvement proposal was submitted", StopReason: "proposal_missing"},
	})
	if !ok {
		t.Fatal("free-state loop was not retained")
	}
	receipt := loop.AdmissionReceipt
	if firstStringFromMap(receipt, "candidate_id") != "candidate:frontier-1" || firstStringFromMap(receipt, "target_evidence_ref") != "obs-target" {
		t.Fatalf("admission receipt lost candidate/evidence binding: %+v", receipt)
	}
	if len(freeStateStringSlice(receipt["failed_gate_ids"])) == 0 || firstStringFromMap(receipt, "boundary") == "" {
		t.Fatalf("admission receipt is not auditable: %+v", receipt)
	}
	gateResults := firstMapFromAny(receipt["gate_results"])
	for _, gateID := range []string{"G1_project_binding", "G2_capacity_assessed", "G3_project_scan", "G4_dimension_closed", "G5_frontier_established", "G6_target_evidence", "G7_fresh_revision_bound_refs"} {
		if _, ok := gateResults[gateID]; !ok {
			t.Fatalf("admission receipt omitted %s result: %+v", gateID, receipt)
		}
	}
	if loop.Status == "no_candidate_found" || strings.Contains(strings.ToLower(loop.LastError), "no_candidate_found") {
		t.Fatalf("runtime fabricated no_candidate_found: %+v", loop)
	}
}

func TestFreeStateCapabilityBoundaryProjectsAdmissionReceiptReadOnly(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	conversationID := "conversation-admission-projection"
	s.storeFreeStateLoop(freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-admission-projection", ConversationID: conversationID, Status: "observing", OriginalIntent: "improve the mix", ActiveIntent: "improve the mix", CreatedAt: now, UpdatedAt: now})
	state := audioclosure.State{SchemaVersion: audioclosure.SchemaVersion, ClosureID: "closure-admission-projection", ConversationID: conversationID, ProjectUUID: "project-1", ProjectRevision: "7", Settlement: &audioclosure.Settlement{Reason: audioclosure.StopCapabilityBlocked, Summary: "proposal missing", SettledAt: now}, Phase: audioclosure.PhaseFS9Terminal}
	response := s.audioClosureResponse(conversationID, "chat", state, agentloop.Result{})
	receipt := firstMapFromAny(response.WorkflowData["free_state_admission_receipt"])
	if firstStringFromMap(receipt, "boundary") != "proposal_missing" || len(firstMapFromAny(receipt["gate_results"])) != 7 {
		t.Fatalf("terminal response omitted auditable admission receipt: %+v", response.WorkflowData)
	}
	snapshot := s.projectAgentRuntimeStateLocked()
	persisted := snapshot.FreeStateLoops[conversationID].AdmissionReceipt
	if firstStringFromMap(persisted, "boundary") != "proposal_missing" || firstStringFromMap(snapshot.FreeStateLoops[conversationID].AdmissionReceipt, "status") != "capability_blocked" {
		t.Fatalf("runtime snapshot lost admission receipt: %+v", snapshot.FreeStateLoops[conversationID])
	}
}

// Exact D1-S1 FS6 replay: a real selected frontier and fresh target evidence
// exist, but the model emits no improvement proposal. The runtime must retain
// the evidence at the admission boundary and must never collapse it into
// no_candidate_found.
func TestD1S1FS6NoProposalSettlesCapabilityBoundaryWithExactEvidence(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	conversationID := "d1-s1-fs6-no-proposal"
	const projectUUID = "vitproj_2235bdda4ec740ae8c1778fd39b27fc7"
	const projectRevision = "2"
	const candidateID = "candidate:3cbb871ab02f6036"
	const observationID = "obs_20260824T055828_6b94308592ac"
	state := audioclosure.State{
		SchemaVersion: audioclosure.SchemaVersion, ClosureID: "closure-d1-fs6", ConversationID: conversationID,
		ProjectUUID: projectUUID, ProjectRevision: projectRevision, Phase: audioclosure.PhaseFS9Terminal,
		Frontier: audioclosure.HypothesisFrontier{CandidateID: candidateID, Candidates: []audioclosure.Candidate{
			{ID: candidateID, SourceObservationID: "obs-frontier", ViewID: "mix.frequency_relationship", TrackIDs: []string{"1007", "1012"}, EvidenceRefs: []string{"obs-frontier"}},
			{ID: "candidate:other-1", TrackIDs: []string{"1012"}}, {ID: "candidate:other-2", TrackIDs: []string{"1022"}}, {ID: "candidate:other-3", TrackIDs: []string{"1032"}},
		}},
		Observations: map[string]audioclosure.ObservationRecord{
			"frontier": {ObservationID: "obs-frontier", ProjectRevision: projectRevision, ViewIDs: []string{"mix.frequency_relationship"}, TargetRef: "project", RecordedAt: now},
			"target":   {ObservationID: observationID, ProjectRevision: projectRevision, ViewIDs: []string{"track.band_dynamics"}, TargetRef: "1007", RecordedAt: now},
		},
		DiagnosticRounds: []audioclosure.DiagnosticRoundRecord{{RoundID: "r-d1-fs6", EvidenceStatus: audioclosure.RoundEvidenceReady, ProjectRevision: projectRevision, ViewsRequested: []string{"mix.frequency_relationship"}}},
		Settlement:       &audioclosure.Settlement{Reason: audioclosure.StopCapabilityBlocked, Summary: "no legal improvement proposal was submitted", SettledAt: now},
	}
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1-fs6", ConversationID: conversationID,
		Status: "observing", OriginalIntent: "检查一下当前工程有什么问题？", ActiveIntent: "检查一下当前工程有什么问题？",
		LatestProjectChange: map[string]any{"project_uuid": projectUUID, "project_revision": projectRevision}, CreatedAt: now, UpdatedAt: now,
	})
	s.capabilityRoutes["d1-fs6-route"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1", ConversationID: conversationID, UpdatedAt: now,
		Assessment: &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}}
	response := s.audioClosureResponse(conversationID, "chat", state, agentloop.Result{})
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || loop.Status != "capability_blocked" || response.StopReason != string(audioclosure.StopCapabilityBlocked) {
		t.Fatalf("FS6 no-proposal path did not settle capability boundary: loop=%+v response=%+v", loop, response)
	}
	receipt := loop.AdmissionReceipt
	if firstStringFromMap(receipt, "candidate_id") != candidateID || firstStringFromMap(receipt, "target_evidence_ref") != observationID {
		t.Fatalf("FS6 receipt lost exact candidate/evidence: %+v", receipt)
	}
	failed := freeStateStringSlice(receipt["failed_gate_ids"])
	if len(failed) == 0 || !containsStringFold(failed, "G7_fresh_revision_bound_refs") {
		t.Fatalf("FS6 no-proposal receipt did not identify G7 boundary: %+v", receipt)
	}
	if firstStringFromMap(receipt, "boundary") != "proposal_missing" || firstStringFromMap(receipt, "status") != "capability_blocked" {
		t.Fatalf("FS6 receipt boundary is not explainable: %+v", receipt)
	}
	if loop.Status == "no_candidate_found" || strings.Contains(strings.ToLower(loop.LastError), "no_candidate_found") {
		t.Fatalf("FS6 frontier was misclassified as no_candidate_found: %+v", loop)
	}
}

func TestD1S1NeedsExperimentWithoutProposalDoesNotSynthesizeOrEnterFS7(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	conversationID := "d1-s1-no-synthesis"
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-no-synthesis", ConversationID: conversationID,
		Status: "observing", CurrentPhase: string(audioclosure.PhaseFS6TargetConfirmed),
		OriginalIntent: "improve the bass", CreatedAt: now, UpdatedAt: now,
	})
	loop, ok := s.recordFreeStateDecision(conversationID, agentloop.Result{
		ContextSnapshot: map[string]any{},
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment,
			EvidenceStatus: "plausible", Summary: "bounded hypothesis without a submitted proposal",
		},
	})
	if !ok {
		t.Fatal("decision was not retained")
	}
	if loop.LatestDecision == nil || loop.LatestDecision.ImprovementProposal != nil {
		t.Fatalf("runtime synthesized a proposal: %+v", loop.LatestDecision)
	}
	if loop.Status != "capability_blocked" || loop.CurrentPhase == string(audioclosure.PhaseFS7ImprovementProposal) || loop.Status == "no_candidate_found" {
		t.Fatalf("missing proposal crossed an invalid boundary: status=%q phase=%q receipt=%+v", loop.Status, loop.CurrentPhase, loop.AdmissionReceipt)
	}
	if firstStringFromMap(loop.AdmissionReceipt, "boundary") != "proposal_missing" || len(freeStateStringSlice(loop.AdmissionReceipt["failed_gate_ids"])) == 0 {
		t.Fatalf("missing-proposal admission receipt is not auditable: %+v", loop.AdmissionReceipt)
	}
}

// A proposal that cites the selected target observation must cross the FS6
// admission boundary into the proposal phase. This test deliberately stops
// before any action executor is wired: admission is not mutation.
func TestD1S1FS6LegalProposalEntersFS8WithoutMutation(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	conversationID := "d1-s1-fs6-legal-proposal"
	const projectUUID = "vitproj_2235bdda4ec740ae8c1778fd39b27fc7"
	const projectRevision = "2"
	const candidateID = "candidate:3cbb871ab02f6036"
	const observationID = "obs_20260824T055828_6b94308592ac"

	state, err := audioclosure.Start(audioclosure.StartRequest{ClosureID: "closure-d1-fs6-legal", ConversationID: conversationID,
		ProjectUUID: projectUUID, ProjectRevision: projectRevision, OriginalIntent: "improve the bass", Mode: audioclosure.ModeTreatment,
		Scope: audioclosure.Scope{Kind: "project", ID: projectUUID}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	driver := audioclosure.Driver{}
	state, _, err = driver.AdmitRound(state, state.Revision, now)
	if err != nil {
		t.Fatal(err)
	}
	// Record both the project scan and selected-track target evidence in the
	// same admitted round, then establish the durable frontier.
	for _, key := range []audioclosure.ObservationKey{
		{ProjectUUID: projectUUID, ProjectRevision: projectRevision, Scope: audioclosure.Scope{Kind: "project", ID: projectUUID}, TargetRef: "project", ViewIDs: []string{"mix.frequency_relationship"}},
		{ProjectUUID: projectUUID, ProjectRevision: projectRevision, Scope: audioclosure.Scope{Kind: "track", ID: "1007"}, TargetRef: "1007", ViewIDs: []string{"track.band_dynamics"}},
	} {
		id := "obs-scan"
		if key.TargetRef == "1007" {
			id = observationID
		}
		out, recErr := driver.RecordObservation(state, state.Revision, key, id, now)
		if recErr != nil {
			t.Fatal(recErr)
		}
		state = out.State
	}
	state, _, err = driver.UpdateFrontier(state, state.Revision, audioclosure.HypothesisFrontier{
		CandidateID: candidateID, Candidates: []audioclosure.Candidate{{ID: candidateID, TrackIDs: []string{"1007"}, SourceObservationID: "obs-scan", EvidenceRefs: []string{"obs-scan"}}},
	}, audioclosure.ActionabilityUnknown, now)
	if err != nil {
		t.Fatal(err)
	}
	state, err = driver.RecordDiagnosticRound(state, state.Revision, audioclosure.DiagnosticRoundRecord{
		SchemaVersion: "free_state_diagnostic_round.v1", RoundID: "r_d1_legal000001", PrimaryDimension: audioclosure.DimensionFrequencyOccupancy, PriorityReason: "default_order", EvidenceStatus: audioclosure.RoundEvidenceReady,
		ProjectRevision: projectRevision, ViewsRequested: []string{"mix.frequency_relationship"}, UnresolvedQuestions: nil,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	state, err = driver.CompleteRound(state, state.Revision, now)
	if err != nil {
		t.Fatal(err)
	}
	state, err = driver.AdvancePhase(state, state.Revision, audioclosure.PhaseGuardEvidence{CapacityAssessed: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if state.Phase != audioclosure.PhaseFS6TargetConfirmed {
		t.Fatalf("fixture did not reach FS6: %s", state.Phase)
	}
	if err := s.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	s.capabilityRoutes["d1-legal-route"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1", ConversationID: conversationID, UpdatedAt: now,
		Assessment: &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}}
	proposal := experimentTestProposal()
	proposal.Target = map[string]any{"kind": "track", "id": "1007"}
	proposal.EvidenceRefs = []string{observationID}
	proposal.ParameterBounds = map[string]any{"delta_db": -1.0}
	proposal.VerificationPlan = map[string]any{"view_ids": []any{"track.band_dynamics"}, "experiment_budget": 1}
	latest := d1FreshObservationForTest(projectRevision)
	latest.Summary["observation_id"] = observationID
	latest.Summary["target_ref"] = map[string]any{"kind": "track", "id": "1007"}
	latest.Summary["requested_views"] = []any{"track.band_dynamics"}
	latest.Summary["actual_executed_view_ids"] = []any{"track.band_dynamics"}
	latest.Summary["evidence_refs"] = []any{observationID}
	latest.Summary["project_binding"] = map[string]any{"project_uuid": projectUUID, "project_revision": projectRevision}
	latest.Summary["audit_receipt"] = map[string]any{"receipt_id": "receipt-target", "view_set_matches": true, "actual_executed_view_ids": []any{"track.band_dynamics"}, "project_revision": projectRevision, "freshness": map[string]any{"status": "fresh", "project_revision": projectRevision}}
	s.storeFreeStateLoop(freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1-legal", ConversationID: conversationID,
		Status: "observing", OriginalIntent: "improve the bass", LatestObservation: latest, LatestProjectChange: map[string]any{"project_uuid": projectUUID, "project_revision": projectRevision}, CreatedAt: now, UpdatedAt: now})
	ctx := map[string]any{
		"task_contract":                  map[string]any{"project_uuid": projectUUID, "project_revision": projectRevision},
		"minimal_audio_closure":          map[string]any{"project_uuid": projectUUID, "project_revision": projectRevision, "hypothesis_frontier": map[string]any{"candidate_id": candidateID, "candidates": []any{map[string]any{"id": candidateID, "track_ids": []any{"1007"}}}}, "diagnostic_rounds": []any{map[string]any{"evidence_status": "ready"}}},
		"free_state_capacity_assessment": map[string]any{"capacity_level": "within_free_state", "selected_capability": "free_state"},
		"free_state_reasoning_loop": map[string]any{"observation_ledger": map[string]any{
			"receipts":          []any{map[string]any{"status": "ready", "observation_id": "obs-scan", "requested_views": []any{"mix.frequency_relationship"}, "project_revision": projectRevision, "freshness": map[string]any{"status": "fresh", "project_revision": projectRevision}}, map[string]any{"status": "ready", "observation_id": observationID, "requested_views": []any{"track.band_dynamics"}, "target_ref": map[string]any{"kind": "track", "id": "1007"}, "evidence_refs": []any{observationID}, "project_revision": projectRevision, "freshness": map[string]any{"status": "fresh", "project_revision": projectRevision}}},
			"available_views":   map[string]any{"track:1007::track.band_dynamics": map[string]any{"view_id": "track.band_dynamics", "status": "ready", "observation_id": observationID, "target_ref": map[string]any{"kind": "track", "id": "1007"}, "evidence_refs": []any{observationID}, "project_revision": projectRevision, "freshness": map[string]any{"status": "fresh", "project_revision": projectRevision}}},
			"diagnostic_rounds": []any{map[string]any{"evidence_status": "ready", "project_revision": projectRevision}},
		}},
	}
	loop, ok := s.recordFreeStateDecision(conversationID, agentloop.Result{ContextSnapshot: ctx, FreeStateDecision: &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment, EvidenceStatus: "plausible", Summary: "bounded bass gain hypothesis", ImprovementProposal: proposal}})
	if !ok {
		t.Fatal("decision was not retained")
	}
	if loop.Status != "awaiting_experiment" || loop.Experiment == nil {
		t.Fatalf("proposal was not admitted: %+v", loop)
	}
	if loop.CurrentPhase != string(audioclosure.PhaseFS8ExperimentVerification) {
		closure, _ := s.audioClosures.ActiveForConversation(conversationID)
		t.Fatalf("expected FS8 verification boundary after validated admission, got %q closure=%s receipt=%+v decision=%+v", loop.CurrentPhase, closure.Phase, loop.AdmissionReceipt, loop.LatestDecision)
	}
	if firstStringFromMap(loop.AdmissionReceipt, "candidate_id") != candidateID || firstStringFromMap(loop.AdmissionReceipt, "target_evidence_ref") != observationID || firstStringFromMap(loop.AdmissionReceipt, "boundary") != "admitted" {
		t.Fatalf("receipt=%+v", loop.AdmissionReceipt)
	}
	if loop.AdmissionReceipt["proposal_present"] != true || loop.AdmissionReceipt["proposal_valid"] != true || len(freeStateStringSlice(loop.AdmissionReceipt["failed_gate_ids"])) != 0 {
		t.Fatalf("gate audit=%+v", loop.AdmissionReceipt)
	}
	if len(s.pendingMixTicks) != 0 {
		t.Fatalf("admission unexpectedly queued mutation: %+v", s.pendingMixTicks)
	}
}

func TestD1S1PlanContainsOneBoundedAction(t *testing.T) {
	loop := d1LoopForTest(t, "7")
	plan, err := d1TrackGainPlan(loop, agentloop.PendingMixTickCandidate{Operation: experiment.D1S1ActionKind, TrackID: "vocal", DeltaDB: -1}, 7, "project-1", "epoch-1", "snapshot-7", map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ActionSet.Actions) != 1 || plan.ActionSet.Actions[0].Command != experiment.D1S1ActionKind || plan.ActionSet.Actions[0].Args["target_db"] != -3.0 || plan.ProjectCut.BaseProjectRevision != "7" || plan.PreviousObservationID != "obs-before" {
		t.Fatalf("plan=%+v", plan)
	}
	if _, err = d1TrackGainPlan(loop, agentloop.PendingMixTickCandidate{Operation: experiment.D1S1ActionKind, TrackID: "other", DeltaDB: -1}, 7, "project-1", "epoch-1", "snapshot-7", map[string]any{"tracks": []any{map[string]any{"track_id": "other", "volume_db": 0.0}}}); err == nil {
		t.Fatal("unobserved target accepted")
	}
}

type d1MutationPortForTest struct{ calls int }

func (p *d1MutationPortForTest) Preflight(context.Context, orchestration.ActionSet, orchestration.ProjectCut) error {
	return nil
}
func (p *d1MutationPortForTest) Apply(_ context.Context, action orchestration.Action, key string) (orchestration.ActionReceipt, error) {
	p.calls++
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true, Details: map[string]any{"idempotency_key": key}}, nil
}
func (p *d1MutationPortForTest) Reconcile(context.Context, orchestration.Action, string, orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	return orchestration.ActionReceipt{}, nil
}

type d1JournalForTest struct {
	recorded []journal.Action
	results  int
}

func (j *d1JournalForTest) JournalGet(id string) (journal.Action, bool) {
	for index := len(j.recorded) - 1; index >= 0; index-- {
		if j.recorded[index].AgentActionID == id {
			return j.recorded[index], true
		}
	}
	return journal.Action{}, false
}
func (j *d1JournalForTest) JournalRecord(action journal.Action) journal.Action {
	j.recorded = append(j.recorded, action)
	return action
}
func (j *d1JournalForTest) JournalMarkResult(string, journal.ActionStatus, map[string]any, error) {
	j.results++
}

func TestD1S1JournalRecordsOneForwardMutation(t *testing.T) {
	inner := &d1MutationPortForTest{}
	log := &d1JournalForTest{}
	port := &d1JournalMutationPort{inner: inner, harness: log, goalID: "goal", runID: "run"}
	action := orchestration.Action{ID: "d1-action", Command: experiment.D1S1ActionKind, TargetRef: "vocal", Args: map[string]any{"target_db": -1.0}}
	if _, err := port.Apply(context.Background(), action, "execution:d1-action"); err != nil {
		t.Fatal(err)
	}
	if _, err := port.Apply(context.Background(), action, "execution:d1-action"); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 || len(log.recorded) != 1 || log.results != 2 || log.recorded[0].AgentActionID != action.ID {
		t.Fatalf("inner=%d journal=%+v results=%d", inner.calls, log.recorded, log.results)
	}
	var _ executionruntime.MutationPort = port
}

func TestD1S1WorkflowFeedsReadOnlyPostActionContinuationIdempotently(t *testing.T) {
	loop := d1LoopForTest(t, "7")
	receipt := map[string]any{"action_id": "d1-action", "status": "applied", "after_revision": "8", "readback_verified": true}
	response := ChatResponse{Workflow: "free_state_d1_s1", GoalStatus: string(agentruntime.StatusWaitingContinue), WorkflowData: map[string]any{
		"status": "applied", "parameter_applied": true, "readback_verified": true, "execution_receipt": receipt,
	}}
	processor, status, projected, attempted := freeStateAcousticActionOutcome(PendingInteraction{}, response)
	if !attempted || processor != experiment.D1S1ActionDomain || status != "applied" || firstStringFromMap(projected, "action_id") != "d1-action" {
		t.Fatalf("outcome processor=%q status=%q attempted=%v receipt=%+v", processor, status, attempted, projected)
	}
	s := New(nil, nil, nil)
	s.recordFreeStateExperimentAction(&loop, processor, status, projected)
	s.recordFreeStateExperimentAction(&loop, processor, status, projected)
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if len(round.Interventions) != 1 || round.Interventions[0].ID != "d1-action" {
		t.Fatalf("D1 projection duplicated the forward mutation: %+v", round.Interventions)
	}
}

func writeD1WAVForTest(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	payload := make([]byte, 128)
	copy(payload, []byte("RIFF"))
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestD1S1AuditionCandidatesCarryDistinctAudioProvenance(t *testing.T) {
	beforePath := writeD1WAVForTest(t, "before.wav")
	afterPath := writeD1WAVForTest(t, "after.wav")
	before := map[string]any{"status": "ready", "file_path": beforePath, "project_revision": "7", "render_revision": "render-before", "preview_revision": "sha256:before", "sha256": "beforehash"}
	after := map[string]any{"status": "ready", "file_path": afterPath, "project_revision": "8", "render_revision": "render-after", "preview_revision": "sha256:after", "sha256": "afterhash"}
	candidates, err := d1AuditionCandidates(before, after, "checkpoint-7", "checkpoint-8", "project.vit", "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || candidates[0].SourceKind != "audio_file" || candidates[1].SourceKind != "audio_file" || candidates[0].ProjectRevision != "7" || candidates[1].ProjectRevision != "8" || candidates[0].RenderRevision == candidates[1].RenderRevision {
		t.Fatalf("candidates=%+v", candidates)
	}
}

func d1EvaluatedLoopForTest(t *testing.T) freeStateReasoningLoop {
	t.Helper()
	loop := d1LoopForTest(t, "7")
	now := time.Now().UTC()
	receipt := map[string]any{"action_id": "d1-action", "status": "applied", "before_revision": "7", "after_revision": "8", "applied_revision": "8", "transaction_id": "tx-d1", "idempotency_key": "key-d1", "actual_readback_db": -1.0, "readback_verified": true}
	if _, err := loop.Experiment.ApplyIntervention(experiment.Intervention{ID: "d1-action", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true, Receipt: receipt}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordObservation(experiment.Observation{ID: "after", ReceiptID: "receipt-after", RequestedViewIDs: []string{"mix.multitrack_relationship"}, ExecutedViewIDs: []string{"mix.multitrack_relationship"}, ViewSetMatches: true, Fresh: true, PostAction: true, ProjectRevision: "8", EvidenceRefs: []string{"after"}}, true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.EvaluateMateriality(experiment.MaterialityEvaluation{State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"material"}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"target"}}, now); err != nil {
		t.Fatal(err)
	}
	loop.D1State = map[string]any{"before_render": map[string]any{"status": "ready", "render_revision": "render-before"}, "after_render": map[string]any{"status": "ready", "render_revision": "render-after"}}
	loop.AuditionSessionID = "audition-d1"
	loop.AuditionSessionSnapshot = map[string]any{"session_id": loop.AuditionSessionID, "status": "ready", "candidate_a_ref": "before.wav", "candidate_b_ref": "after.wav"}
	if _, err := loop.Experiment.RequestUserJudgmentForSession("A/B", loop.AuditionSessionID, now); err != nil {
		t.Fatal(err)
	}
	return loop
}

func TestD1S1HumanJudgmentRetainAndAmbiguousSettlement(t *testing.T) {
	for _, test := range []struct {
		name       string
		heard      experiment.HeardDifference
		preference experiment.JudgmentPreference
		confirmed  bool
		ambiguous  bool
	}{
		{name: "retain", heard: experiment.HeardDifferenceYes, preference: experiment.PreferenceB, confirmed: true},
		{name: "ambiguous", heard: experiment.HeardDifferenceNo, preference: experiment.PreferenceUnsure, ambiguous: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			loop := d1EvaluatedLoopForTest(t)
			round, _ := loop.Experiment.CurrentRound()
			evidence := experiment.UserJudgmentEvidence{SchemaVersion: experiment.UserJudgmentEvidenceSchemaVersion, ConversationID: loop.ConversationID, TurnID: loop.Experiment.ID, RoundID: round.ID, AuditionSessionID: loop.AuditionSessionID, CandidateARef: "before.wav", CandidateBRef: "after.wav", HeardDifference: test.heard, Preference: test.preference, CreatedAt: time.Now().UTC()}
			if _, err := loop.Experiment.RecordUserJudgmentEvidence(evidence, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			round, _ = loop.Experiment.CurrentRound()
			evidence = round.UserJudgmentEvidence[len(round.UserJudgmentEvidence)-1]
			s := New(nil, nil, nil)
			if err := s.applyFreeStateJudgmentOutcome(context.Background(), &loop, evidence); err != nil {
				t.Fatal(err)
			}
			syncD1Receipt(&loop)
			if loop.D1Receipt["human_confirmed"] != test.confirmed || loop.D1Receipt["ambiguous"] != test.ambiguous || loop.D1Receipt["settled"] != true || loop.D1Receipt["forward_mutation_count"] != 1 {
				t.Fatalf("receipt=%+v", loop.D1Receipt)
			}
		})
	}
}

func d1ServerAtHumanAuditionReady(t *testing.T) (*Server, freeStateReasoningLoop) {
	t.Helper()
	s := New(nil, nil, nil)
	loop := d1LoopForTest(t, "7")
	receipt := orchestration.ActionReceipt{ActionID: "d1-action", Status: "applied", AppliedRevision: "8", EffectivelyOnce: true, Details: map[string]any{
		"before_revision": "7", "after_revision": "8", "transaction_id": "tx-d1", "idempotency_key": "key-d1", "actual_readback_db": -1.0, "readback_verified": true,
	}}
	session := orchestration.PlanningSession{ID: "d1-session", Status: orchestration.StatusCompleted, Execution: &orchestration.ExecutionRecord{
		ID: "execution-1", IdempotencyKey: "key-d1", Receipts: []orchestration.ActionReceipt{receipt},
		VerificationResult: &orchestration.VerificationResult{Status: "verified", Fresh: true, PostAction: true, ObservationID: "obs-after", ObservationRevision: "8", EvidenceRefs: []string{"ccb-after"}},
	}}
	_ = s.projectD1Execution(loop, session, nil)
	loop, _ = s.freeStateLoop(loop.ConversationID)
	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality:    &experiment.MaterialityEvaluation{State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"materiality-after"}},
		ExperimentTargetResponse: &experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"target-after"}},
	})
	loop.AuditionSessionID = "audition-d1"
	loop.D1State = map[string]any{
		"before_render": map[string]any{"status": "ready", "project_revision": "7", "render_revision": "render-before"},
		"after_render":  map[string]any{"status": "ready", "project_revision": "8", "render_revision": "render-after"},
	}
	loop.AuditionSessionSnapshot = map[string]any{
		"session_id": loop.AuditionSessionID, "status": "ready", "project_revision": "8", "scope": "target",
		"candidates": []any{
			map[string]any{"id": "candidate-a", "status": "ready", "source_ref": "before.wav", "preview_ref": "audio_file:before", "render_revision": "render-before"},
			map[string]any{"id": "candidate-b", "status": "ready", "source_ref": "after.wav", "preview_ref": "audio_file:after", "render_revision": "render-after"},
		},
	}
	s.storeFreeStateLoop(loop)
	s.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)
	loop, _ = s.freeStateLoop(loop.ConversationID)
	if loop.D1Receipt["parameter_applied"] != true || loop.D1Receipt["readback_verified"] != true || loop.D1Receipt["evaluation_ready"] != true || loop.D1Receipt["human_audition_ready"] != true {
		t.Fatalf("pre-judgment receipt=%+v", loop.D1Receipt)
	}
	return s, loop
}

func TestD1S1ServerJudgmentHandlerSettlesRetainRollbackAndAmbiguous(t *testing.T) {
	for _, test := range []struct {
		name, heard, preference          string
		confirmed, ambiguous, rolledBack bool
	}{
		{name: "retain", heard: "yes", preference: "b", confirmed: true},
		{name: "rollback", heard: "yes", preference: "a", confirmed: true, rolledBack: true},
		{name: "ambiguous", heard: "no", preference: "unsure", ambiguous: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, loop := d1ServerAtHumanAuditionReady(t)
			var sender *d1UndoSender
			if test.rolledBack {
				sender = &d1UndoSender{}
				s.harness = harness.NewWithSender(sender, nil, nil)
				s.harness.JournalRecord(journal.Action{AgentActionID: "d1-action", Domain: "daw", Status: journal.StatusSucceeded, Command: map[string]any{"cmd": "set_volume"}})
				s.harness.JournalRecord(journal.Action{AgentActionID: "unrelated-later-action", Domain: "daw", Status: journal.StatusSucceeded, Command: map[string]any{"cmd": "set_pan"}})
			}
			round, _ := loop.Experiment.CurrentRound()
			body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"8","heard_difference":%q,"preference":%q}`, loop.ConversationID, loop.Experiment.ID, round.ID, loop.AuditionSessionID, test.heard, test.preference)
			recorder := httptest.NewRecorder()
			s.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			stored, _ := s.freeStateLoop(loop.ConversationID)
			forwardCount, _ := treatmentNumber(stored.D1Receipt, "forward_mutation_count")
			if stored.D1Receipt["human_confirmed"] != test.confirmed || stored.D1Receipt["ambiguous"] != test.ambiguous || stored.D1Receipt["rolled_back"] != test.rolledBack || stored.D1Receipt["settled"] != true || forwardCount != 1 {
				t.Fatalf("receipt=%+v", stored.D1Receipt)
			}
			if test.rolledBack && (len(sender.commands) != 1 || sender.commands[0]["target_action_id"] != "d1-action") {
				t.Fatalf("rollback targeted wrong journal action: %+v", sender.commands)
			}
		})
	}
}

type d1UndoSender struct{ commands []map[string]any }

func (s *d1UndoSender) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	s.commands = append(s.commands, cloneContext(command))
	return map[string]any{"status": "ok", "agent_action_id": "rollback-1"}, `{"status":"ok"}`, nil
}

func TestD1S1RollbackUsesExistingRollbackAction(t *testing.T) {
	loop := d1EvaluatedLoopForTest(t)
	round, _ := loop.Experiment.CurrentRound()
	evidence := experiment.UserJudgmentEvidence{SchemaVersion: experiment.UserJudgmentEvidenceSchemaVersion, ConversationID: loop.ConversationID, TurnID: loop.Experiment.ID, RoundID: round.ID, AuditionSessionID: loop.AuditionSessionID, CandidateARef: "before.wav", CandidateBRef: "after.wav", HeardDifference: experiment.HeardDifferenceYes, Preference: experiment.PreferenceA, CreatedAt: time.Now().UTC()}
	if _, err := loop.Experiment.RecordUserJudgmentEvidence(evidence, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	round, _ = loop.Experiment.CurrentRound()
	sender := &d1UndoSender{}
	s := New(nil, nil, nil)
	s.harness = harness.NewWithSender(sender, nil, nil)
	s.harness.JournalRecord(journal.Action{AgentActionID: "d1-action", Domain: "daw", Status: journal.StatusSucceeded, Command: map[string]any{"cmd": "set_volume"}})
	s.harness.JournalRecord(journal.Action{AgentActionID: "unrelated-later-action", Domain: "daw", Status: journal.StatusSucceeded, Command: map[string]any{"cmd": "set_pan"}})
	if err := s.applyFreeStateJudgmentOutcome(context.Background(), &loop, round.UserJudgmentEvidence[0]); err != nil {
		t.Fatal(err)
	}
	syncD1Receipt(&loop)
	if len(sender.commands) != 1 || sender.commands[0]["cmd"] != "undo" || sender.commands[0]["target_action_id"] != "d1-action" || loop.D1Receipt["rolled_back"] != true || loop.D1Receipt["rollback_compensation_count"] != 1 || loop.D1Receipt["forward_mutation_count"] != 1 {
		t.Fatalf("commands=%+v receipt=%+v", sender.commands, loop.D1Receipt)
	}
}

func TestD1S1RestartProjectionDoesNotDuplicateMutation(t *testing.T) {
	loop := d1LoopForTest(t, "7")
	s := New(nil, nil, nil)
	receipt := orchestration.ActionReceipt{ActionID: "d1-action", Status: "applied", AppliedRevision: "8", EffectivelyOnce: true, Details: map[string]any{"before_revision": "7", "after_revision": "8", "transaction_id": "tx", "idempotency_key": "key", "actual_readback_db": -1.0, "readback_verified": true}}
	session := orchestration.PlanningSession{ID: "d1-session", Status: orchestration.StatusCompleted, Execution: &orchestration.ExecutionRecord{ID: "execution-1", IdempotencyKey: "key", Receipts: []orchestration.ActionReceipt{receipt}, VerificationResult: &orchestration.VerificationResult{Status: "verified", Fresh: true, PostAction: true, ObservationID: "after", ObservationRevision: "8", EvidenceRefs: []string{"after"}}}}
	_ = s.projectD1Execution(loop, session, nil)
	restored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok {
		t.Fatal("projected loop was not persisted")
	}
	_ = s.projectD1Execution(restored, session, nil)
	restored, _ = s.freeStateLoop(loop.ConversationID)
	round, _ := restored.Experiment.CurrentRound()
	if len(round.Interventions) != 1 || len(round.Observations) != 2 {
		t.Fatalf("restart duplicated state: %+v", round)
	}
}
