package chat

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

func auditionReadyLoop(t *testing.T) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-audition", ConversationID: "conversation-audition", GoalID: "goal", RunID: "run", OriginalIntent: "compare before and after", LatestProjectChange: map[string]any{"project_path": "active.vit", "project_revision": "rev-7"}, CreatedAt: now, UpdatedAt: now}
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

func TestReadyAuditionSelectSettlesExperimentWithoutProjectPlaneCommands(t *testing.T) {
	fake := &fakeAuditionKernel{}
	server := New(nil, nil, nil)
	server.auditionKernel = fake
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
	foundJudgment, foundDecision, foundSettlement := false, false, false
	for _, event := range events {
		foundJudgment = foundJudgment || event.Type == string(trajectory.EventUserJudgmentRecorded)
		foundDecision = foundDecision || event.Type == string(trajectory.EventRoundDecision)
		foundSettlement = foundSettlement || event.Type == string(trajectory.EventSettled)
	}
	if !foundJudgment || !foundDecision || !foundSettlement {
		t.Fatalf("events=%+v", events)
	}
}
