package chat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/trajectory"
)

type fakeAuditionKernel struct {
	prepareRequests []kernel.AuditionSessionRequest
	selectCalls     []string
	prepareError    error
	selectError     error
}

func (f *fakeAuditionKernel) AuditionPrepare(_ context.Context, request kernel.AuditionSessionRequest) (*kernel.VSPCommandResult, error) {
	f.prepareRequests = append(f.prepareRequests, request)
	if f.prepareError != nil {
		return nil, f.prepareError
	}
	return &kernel.VSPCommandResult{LegacyReply: map[string]any{"status": "ok", "session": map[string]any{
		"session_id": request.SessionID, "conversation_id": request.ConversationID, "status": "preparing",
		"active_project_plane": map[string]any{"plane": "active_project", "project_revision": request.ActiveProjectRevision},
		"candidates":           []any{map[string]any{"id": "candidate-a", "status": "preparing"}, map[string]any{"id": "candidate-b", "status": "preparing"}},
	}}}, nil
}
func (f *fakeAuditionKernel) AuditionStatus(_ context.Context, sessionID string) (*kernel.VSPCommandResult, error) {
	return nil, nil
}
func (f *fakeAuditionKernel) AuditionSelect(_ context.Context, sessionID, candidateID string) (*kernel.VSPCommandResult, error) {
	f.selectCalls = append(f.selectCalls, sessionID+":"+candidateID)
	if f.selectError != nil {
		return nil, f.selectError
	}
	return &kernel.VSPCommandResult{LegacyReply: map[string]any{"status": "ok", "session": map[string]any{
		"session_id": sessionID, "status": "ready", "active_candidate_id": candidateID,
		"candidates": []any{map[string]any{"id": "candidate-a", "status": "ready", "preview_ref": "a"}, map[string]any{"id": "candidate-b", "status": "ready", "preview_ref": "b"}},
	}}}, nil
}
func (f *fakeAuditionKernel) AuditionStop(_ context.Context, sessionID string) (*kernel.VSPCommandResult, error) {
	return nil, nil
}

type candidateDriverForTest struct {
	plane         auditionProjectPlane
	checkoutCalls []auditionCandidateReference
	checkpoints   int
	observations  int
	checkoutErr   error
}

func (d *candidateDriverForTest) CurrentPlane(context.Context) (auditionProjectPlane, error) {
	return d.plane, nil
}
func (d *candidateDriverForTest) CreateCheckpoint(_ context.Context, request auditionCheckpointRequest) (map[string]any, error) {
	d.checkpoints++
	return map[string]any{"status": "ok", "commit_id": fmt.Sprintf("candidate-commit-%d", d.checkpoints), "project_path": request.ProjectPath}, nil
}
func (d *candidateDriverForTest) CheckoutCandidate(_ context.Context, ref auditionCandidateReference) (map[string]any, error) {
	d.checkoutCalls = append(d.checkoutCalls, ref)
	if d.checkoutErr != nil && ref.CandidateID != "safety" {
		return nil, d.checkoutErr
	}
	d.plane.HistoryHead = firstNonEmpty(ref.CommitID, ref.CheckpointRef)
	return map[string]any{"status": "ok", "refresh": map[string]any{"kernel_reloaded": true, "shadow_refreshed": true}}, nil
}
func (d *candidateDriverForTest) RefreshPlane(context.Context, string) (auditionProjectPlane, map[string]any, error) {
	return d.plane, map[string]any{"freshness": "current_snapshot", "change_id": "test-change"}, nil
}
func (d *candidateDriverForTest) RequestObservation(_ context.Context, _ freeStateReasoningLoop, _ experiment.Round) (map[string]any, *agentloop.RecentObservation, error) {
	d.observations++
	summary := map[string]any{"status": "ready", "observation_id": "obs-adoption", "requested_views": []any{"track.timbre_frequency"}, "actual_executed_view_ids": []any{"track.timbre_frequency"}, "evidence_refs": []any{"obs-adoption"}, "audit_receipt": map[string]any{"view_set_matches": true, "actual_executed_view_ids": []any{"track.timbre_frequency"}, "freshness": map[string]any{"status": "fresh"}}}
	return summary, &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ready", ToolCallID: "obs-adoption", Summary: summary}, nil
}
// auditionProjectPathForTest keeps project references absolute so history
// writes land inside the test's temp workspace instead of the package dir.
func auditionProjectPathForTest(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "active.vit")
}

func newCandidateDriverForTest(projectPath string) *candidateDriverForTest {
	return &candidateDriverForTest{plane: auditionProjectPlane{ProjectPath: projectPath, ProjectUUID: "project-1", ProjectRevision: "rev-7", HistoryHead: "checkpoint-7", ActiveBranch: "main"}}
}

func auditionReadyLoop(t *testing.T) freeStateReasoningLoop {
	t.Helper()
	return auditionReadyLoopAt(t, auditionProjectPathForTest(t))
}

func auditionReadyLoopAt(t *testing.T, projectPath string) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-audition", ConversationID: "conversation-audition", GoalID: "goal", RunID: "run", OriginalIntent: "compare before and after", LatestProjectChange: map[string]any{"project_path": projectPath, "project_revision": "rev-7"}, CreatedAt: now, UpdatedAt: now}
	admission, err := freeStateExperimentAdmission(loop, experimentTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	turn, err := experiment.NewTurn(experiment.Identity{ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID, TurnID: "turn-audition"}, loop.OriginalIntent, admission, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-7", "rev-7", now); err != nil {
		t.Fatal(err)
	}
	before := experiment.Observation{ID: "before", RequestedViewIDs: []string{"track.timbre_frequency"}, ExecutedViewIDs: []string{"track.timbre_frequency"}, ViewSetMatches: true, Fresh: true, EvidenceRefs: []string{"before"}}
	if _, err = turn.RecordObservation(before, false, now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.ApplyIntervention(experiment.Intervention{ID: "action-7", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true, Receipt: map[string]any{"status": "ok"}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.EvaluateMateriality(experiment.MaterialityEvaluation{State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"material"}}, now); err != nil {
		t.Fatal(err)
	}
	after := before
	after.ID = "after"
	after.PostAction = true
	after.EvidenceRefs = []string{"after"}
	if _, err = turn.RecordObservation(after, true, now); err != nil {
		t.Fatal(err)
	}
	loop.Experiment = &turn
	return loop
}

func TestFreeStateHumanAuditionCallsKernelPrepareAndEmitsEvents(t *testing.T) {
	fake := &fakeAuditionKernel{}
	server := New(nil, nil, nil)
	server.auditionKernel = fake
	server.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))
	loop := auditionReadyLoop(t)
	server.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{ExperimentTargetResponse: &experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}})
	if len(fake.prepareRequests) != 1 {
		t.Fatalf("prepare calls=%d", len(fake.prepareRequests))
	}
	request := fake.prepareRequests[0]
	if request.ConversationID != loop.ConversationID || len(request.Candidates) != 2 || request.Candidates[0].SourceKind != "checkpoint" || request.Candidates[1].SourceKind != "experiment" {
		t.Fatalf("request=%+v", request)
	}
	events, _ := server.agentEventsSince(loop.ConversationID, 0, 100)
	foundTarget, foundPrepare := false, false
	for _, event := range events {
		foundTarget = foundTarget || event.Type == string(trajectory.EventTargetResponse)
		foundPrepare = foundPrepare || event.Type == "audition.prepare"
	}
	if !foundTarget || !foundPrepare {
		t.Fatalf("events=%+v", events)
	}
}

func TestKernelAuditionReadyTelemetryReturnsToAgentEventTransport(t *testing.T) {
	server := New(nil, nil, nil)
	server.HandleKernelTelemetry(map[string]any{"type": "audition.ready", "conversation_id": "conversation-ready", "session": map[string]any{"session_id": "session-ready", "conversation_id": "conversation-ready", "status": "ready"}})
	events, _ := server.agentEventsSince("conversation-ready", 0, 10)
	if len(events) != 1 || events[0].Type != "audition.ready" || events[0].ItemType != "audition" {
		t.Fatalf("events=%+v", events)
	}
}

func TestFreeStateAuditionPrepareFailureIsObservable(t *testing.T) {
	fake := &fakeAuditionKernel{prepareError: errors.New("preview unavailable")}
	server := New(nil, nil, nil)
	server.auditionKernel = fake
	server.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))
	loop := auditionReadyLoop(t)
	if err := server.prepareFreeStateAudition(context.Background(), &loop); err == nil {
		t.Fatal("expected prepare failure")
	}
	events, _ := server.agentEventsSince(loop.ConversationID, 0, 10)
	if len(events) != 1 || events[0].Type != "audition.failed" {
		t.Fatalf("events=%+v", events)
	}
}

func TestAuditionSelectRejectsCandidateBeforeReady(t *testing.T) {
	fake := &fakeAuditionKernel{selectError: errors.New("candidate_not_ready")}
	server := New(nil, nil, nil)
	server.auditionKernel = fake
	request := httptest.NewRequest(http.MethodPost, "/agent/audition/select", bytes.NewBufferString(`{"conversation_id":"conversation","session_id":"session","candidate_id":"candidate-a"}`))
	recorder := httptest.NewRecorder()
	server.handleAuditionSelect(recorder, request)
	if recorder.Code != http.StatusConflict || len(fake.selectCalls) != 1 {
		t.Fatalf("status=%d calls=%v body=%s", recorder.Code, fake.selectCalls, recorder.Body.String())
	}
}

func TestReadyAuditionSelectOnlyChangesPreviewWithoutRecordingJudgment(t *testing.T) {
	fake := &fakeAuditionKernel{}
	server := New(nil, nil, nil)
	server.auditionKernel = fake
	server.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))
	loop := auditionReadyLoop(t)
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	loop.AuditionSessionID = "session-ready"
	server.storeFreeStateLoop(loop)
	request := httptest.NewRequest(http.MethodPost, "/agent/audition/select", bytes.NewBufferString(`{"conversation_id":"conversation-audition","session_id":"session-ready","candidate_id":"candidate-b"}`))
	recorder := httptest.NewRecorder()
	server.handleAuditionSelect(recorder, request)
	if recorder.Code != http.StatusOK || len(fake.selectCalls) != 1 || fake.selectCalls[0] != "session-ready:candidate-b" {
		t.Fatalf("status=%d calls=%v body=%s", recorder.Code, fake.selectCalls, recorder.Body.String())
	}
	events, _ := server.agentEventsSince(loop.ConversationID, 0, 100)
	for _, event := range events {
		if event.Type == string(trajectory.EventUserJudgmentRecorded) || event.Type == string(trajectory.EventRoundDecision) || event.Type == string(trajectory.EventSettled) {
			t.Fatalf("preview selection unexpectedly completed judgment/round: events=%+v", events)
		}
	}
	stored, ok := server.freeStateLoop(loop.ConversationID)
	if !ok || stored.Experiment == nil || stored.Experiment.Status == experiment.StatusSettled {
		t.Fatalf("preview selection changed experiment lifecycle: %+v", stored.Experiment)
	}
}

func TestAuditionJudgmentRecordsEvidenceAndRetainsPreferredTreatment(t *testing.T) {
	fake := &fakeAuditionKernel{}
	server := New(nil, nil, nil)
	server.auditionKernel = fake
	server.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))
	loop := auditionReadyLoop(t)
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	loop.AuditionSessionID = "session-ready"
	loop.AuditionSessionSnapshot = map[string]any{
		"session_id": "session-ready", "conversation_id": loop.ConversationID, "turn_id": loop.Experiment.ID,
		"round_id": loop.Experiment.Rounds[0].ID, "status": "ready", "scope": "target", "project_revision": "rev-7",
		"candidates": []any{
			map[string]any{"id": "candidate-a", "status": "ready", "source_ref": "checkpoint:7", "preview_ref": "preview:a"},
			map[string]any{"id": "candidate-b", "status": "ready", "source_ref": "action:7", "preview_ref": "preview:b"},
		},
	}
	server.storeFreeStateLoop(loop)
	server.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)
	round, _ := loop.Experiment.CurrentRound()
	requestBody := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"rev-7","heard_difference":"yes","preference":"b","reason_tags":["更自然"]}`, loop.ConversationID, loop.Experiment.ID, round.ID, loop.AuditionSessionID)
	recorder := httptest.NewRecorder()
	server.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(requestBody)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	stored, ok := server.freeStateLoop(loop.ConversationID)
	if !ok || stored.Experiment == nil || stored.Experiment.Status == experiment.StatusSettled || stored.Status != "awaiting_candidate_apply" {
		t.Fatalf("stored=%+v", stored)
	}
	finalRound, _ := stored.Experiment.CurrentRound()
	if len(finalRound.UserJudgmentEvidence) != 1 || finalRound.UserJudgmentEvidence[0].Preference != experiment.PreferenceB {
		t.Fatalf("evidence=%+v", finalRound.UserJudgmentEvidence)
	}
}

func TestAuditionJudgmentWithoutDifferenceStartsNextRound(t *testing.T) {
	server := New(nil, nil, nil)
	loop := auditionReadyLoop(t)
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	loop.AuditionSessionID = "session-ambiguous"
	loop.AuditionSessionSnapshot = map[string]any{"session_id": "session-ambiguous", "status": "ready", "scope": "target", "project_revision": "rev-7", "candidates": []any{map[string]any{"id": "candidate-a", "status": "ready", "source_ref": "a", "preview_ref": "a"}, map[string]any{"id": "candidate-b", "status": "ready", "source_ref": "b", "preview_ref": "b"}}}
	server.storeFreeStateLoop(loop)
	server.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)
	round, _ := loop.Experiment.CurrentRound()
	requestBody := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"rev-7","heard_difference":"no","preference":"a"}`, loop.ConversationID, loop.Experiment.ID, round.ID, loop.AuditionSessionID)
	recorder := httptest.NewRecorder()
	server.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(requestBody)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	stored, _ := server.freeStateLoop(loop.ConversationID)
	if stored.Experiment.Status == experiment.StatusSettled || len(stored.Experiment.Rounds) != 2 || stored.Experiment.Rounds[0].Decision != experiment.DecisionNextRound {
		t.Fatalf("ambiguous outcome=%+v", stored.Experiment)
	}
}

func TestUserJudgmentDispositionCoversPreferencePolicy(t *testing.T) {
	tests := []struct {
		name          string
		heard         experiment.HeardDifference
		preference    experiment.JudgmentPreference
		decision      experiment.RoundDecision
		outcome       experiment.SettlementOutcome
		continueRound bool
	}{
		{"prefer A", experiment.HeardDifferenceYes, experiment.PreferenceA, experiment.DecisionRollback, experiment.OutcomeRolledBack, false},
		{"prefer B", experiment.HeardDifferenceYes, experiment.PreferenceB, experiment.DecisionRetain, experiment.OutcomeImproved, false},
		{"equal", experiment.HeardDifferenceYes, experiment.PreferenceEqual, experiment.DecisionNextRound, "", true},
		{"neither", experiment.HeardDifferenceYes, experiment.PreferenceNeither, experiment.DecisionNextRound, "", true},
		{"unsure", experiment.HeardDifferenceYes, experiment.PreferenceUnsure, experiment.DecisionNextRound, "", true},
		{"no difference", experiment.HeardDifferenceNo, experiment.PreferenceB, experiment.DecisionNextRound, "", true},
		{"difference unsure", experiment.HeardDifferenceUnsure, experiment.PreferenceA, experiment.DecisionNextRound, "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := dispositionForUserJudgment(experiment.UserJudgmentEvidence{HeardDifference: test.heard, Preference: test.preference})
			if got.Decision != test.decision || got.Outcome != test.outcome || got.Continue != test.continueRound {
				t.Fatalf("disposition=%+v", got)
			}
		})
	}
}

func TestAuditionJudgmentRejectsStaleSessionAndRevisionMismatch(t *testing.T) {
	for _, test := range []struct{ name, status, revision string }{
		{"stale", "stale", "rev-7"},
		{"revision mismatch", "ready", "rev-old"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := New(nil, nil, nil)
			loop := auditionReadyLoop(t)
			if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			loop.AuditionSessionID = "session-guarded"
			loop.AuditionSessionSnapshot = map[string]any{"session_id": loop.AuditionSessionID, "status": test.status, "project_revision": "rev-7", "candidates": []any{map[string]any{"id": "candidate-a", "status": "ready", "source_ref": "a", "preview_ref": "a"}, map[string]any{"id": "candidate-b", "status": "ready", "source_ref": "b", "preview_ref": "b"}}}
			server.storeFreeStateLoop(loop)
			if test.status == "ready" {
				server.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)
			} else {
				// Bind the runtime request before the Kernel marks the session stale.
				loop.AuditionSessionSnapshot["status"] = "ready"
				server.storeFreeStateLoop(loop)
				server.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)
				stored, _ := server.freeStateLoop(loop.ConversationID)
				stored.AuditionSessionSnapshot["status"] = "stale"
				server.storeFreeStateLoop(stored)
			}
			stored, _ := server.freeStateLoop(loop.ConversationID)
			round, _ := stored.Experiment.CurrentRound()
			requestBody := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":%q,"heard_difference":"yes","preference":"b"}`, stored.ConversationID, stored.Experiment.ID, round.ID, stored.AuditionSessionID, test.revision)
			recorder := httptest.NewRecorder()
			server.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(requestBody)))
			if recorder.Code != http.StatusConflict {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAuditionJudgmentRejectsWrongTurnRoundAndSessionBindings(t *testing.T) {
	server := New(nil, nil, nil)
	loop := auditionReadyLoop(t)
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	loop.AuditionSessionID = "session-bound"
	loop.AuditionSessionSnapshot = map[string]any{"session_id": loop.AuditionSessionID, "status": "ready", "project_revision": "rev-7", "candidates": []any{map[string]any{"id": "candidate-a", "status": "ready", "source_ref": "a", "preview_ref": "a"}, map[string]any{"id": "candidate-b", "status": "ready", "source_ref": "b", "preview_ref": "b"}}}
	server.storeFreeStateLoop(loop)
	server.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)
	stored, _ := server.freeStateLoop(loop.ConversationID)
	round, _ := stored.Experiment.CurrentRound()
	for _, test := range []struct{ name, turnID, roundID, sessionID string }{
		{"turn", "wrong-turn", round.ID, stored.AuditionSessionID},
		{"round", stored.Experiment.ID, "wrong-round", stored.AuditionSessionID},
		{"session", stored.Experiment.ID, round.ID, "wrong-session"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"rev-7","heard_difference":"yes","preference":"b"}`, stored.ConversationID, test.turnID, test.roundID, test.sessionID)
			recorder := httptest.NewRecorder()
			server.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
			if recorder.Code != http.StatusConflict {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func boundAdoptionLoopForTest(t *testing.T, projectPath string) freeStateReasoningLoop {
	t.Helper()
	loop := auditionReadyLoopAt(t, projectPath)
	loop.AuditionSessionID = "session-adoption"
	round := loop.Experiment.Rounds[0]
	round.AuditionSessionID = loop.AuditionSessionID
	loop.Experiment.Rounds[0] = round
	loop.AuditionSessionSnapshot = map[string]any{"session_id": loop.AuditionSessionID, "conversation_id": loop.ConversationID, "turn_id": loop.Experiment.ID, "round_id": round.ID, "status": "ready", "scope": "target", "project_uuid": "project-1", "project_revision": "rev-7", "active_project_ref": projectPath, "candidates": []any{map[string]any{"id": "candidate-a", "label": "A", "status": "ready", "source_kind": "checkpoint", "source_ref": "checkpoint:checkpoint-7", "checkpoint_ref": "checkpoint-7", "commit_id": "checkpoint-7", "project_path": projectPath, "project_uuid": "project-1", "project_revision": "rev-7", "preview_ref": "preview:a"}, map[string]any{"id": "candidate-b", "label": "B", "status": "ready", "source_kind": "experiment", "source_ref": "checkpoint:treatment-7", "checkpoint_ref": "treatment-7", "commit_id": "treatment-7", "project_path": projectPath, "project_uuid": "project-1", "project_revision": "rev-7", "preview_ref": "preview:b"}}}
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	return loop
}
func recordPreferredBForTest(t *testing.T, s *Server, loop *freeStateReasoningLoop) experiment.UserJudgmentEvidence {
	t.Helper()
	s.storeFreeStateLoop(*loop)
	s.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)
	stored, _ := s.freeStateLoop(loop.ConversationID)
	round, _ := stored.Experiment.CurrentRound()
	body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"rev-7","heard_difference":"yes","preference":"b"}`, stored.ConversationID, stored.Experiment.ID, round.ID, stored.AuditionSessionID)
	rec := httptest.NewRecorder()
	s.handleAuditionJudgment(rec, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("judgment status=%d body=%s", rec.Code, rec.Body.String())
	}
	stored, _ = s.freeStateLoop(loop.ConversationID)
	round, _ = stored.Experiment.CurrentRound()
	return round.UserJudgmentEvidence[len(round.UserJudgmentEvidence)-1]
}
func TestCandidateInspectDoesNotAdopt(t *testing.T) {
	s := New(nil, nil, nil)
	project := auditionProjectPathForTest(t)
	d := newCandidateDriverForTest(project)
	s.auditionCandidateDriver = d
	loop := boundAdoptionLoopForTest(t, project)
	s.storeFreeStateLoop(loop)
	receipt, err := s.inspectAuditionCandidate(context.Background(), auditionCandidateOperationRequest{ConversationID: loop.ConversationID, SessionID: loop.AuditionSessionID, CandidateID: "candidate-b"})
	if err != nil || receipt.Status != "inspected" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	stored, _ := s.freeStateLoop(loop.ConversationID)
	if stored.Experiment.Status == experiment.StatusSettled || firstStringFromMap(stored.AuditionSessionSnapshot, "adopted_candidate_id") != "" {
		t.Fatalf("stored=%+v", stored)
	}
}
func TestCandidateInspectFailureRollsBack(t *testing.T) {
	s := New(nil, nil, nil)
	project := auditionProjectPathForTest(t)
	d := newCandidateDriverForTest(project)
	d.checkoutErr = errors.New("materialize failed")
	s.auditionCandidateDriver = d
	loop := boundAdoptionLoopForTest(t, project)
	s.storeFreeStateLoop(loop)
	receipt, err := s.inspectAuditionCandidate(context.Background(), auditionCandidateOperationRequest{ConversationID: loop.ConversationID, SessionID: loop.AuditionSessionID, CandidateID: "candidate-b"})
	if err == nil || firstStringFromMap(receipt.Rollback, "status") != "rolled_back" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
}
func TestCandidateJudgmentNeedsExplicitApply(t *testing.T) {
	s := New(nil, nil, nil)
	project := auditionProjectPathForTest(t)
	d := newCandidateDriverForTest(project)
	s.auditionCandidateDriver = d
	loop := boundAdoptionLoopForTest(t, project)
	evidence := recordPreferredBForTest(t, s, &loop)
	stored, _ := s.freeStateLoop(loop.ConversationID)
	if stored.Experiment.Status == experiment.StatusSettled {
		t.Fatal("judgment settled before apply")
	}
	receipt, err := s.applyAuditionCandidate(context.Background(), auditionCandidateOperationRequest{ConversationID: loop.ConversationID, SessionID: loop.AuditionSessionID, CandidateID: "candidate-b", JudgmentEvidenceID: evidence.ID})
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	stored, _ = s.freeStateLoop(loop.ConversationID)
	if stored.Experiment.Status != experiment.StatusSettled || firstStringFromMap(stored.AuditionSessionSnapshot, "adopted_candidate_id") != "candidate-b" || d.observations != 1 {
		t.Fatalf("stored=%+v observations=%d", stored, d.observations)
	}
}
