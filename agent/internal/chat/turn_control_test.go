package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/trajectory"
)

func gapTestAdmission(mode experiment.AuthorityMode) experiment.Admission {
	return experiment.Admission{
		SchemaVersion: experiment.SchemaVersion, TargetRef: map[string]any{"kind": "track", "id": "vocal"}, EvidenceRefs: []string{"obs-1"},
		Hypothesis: "bounded treatment", TypedAction: map[string]any{"domain": "eq"}, DiagnosticDoseBounds: map[string]any{"max": 1}, RetainedDoseBounds: map[string]any{"max": 2},
		ExperimentBudget: 2, ExpectedEffect: "forwardness", VerificationPlan: map[string]any{"view_ids": []any{"track.timbre_frequency"}}, CheckpointRef: "checkpoint-1", RollbackPlan: map[string]any{"kind": "undo"}, AuthorityMode: mode,
	}
}

func TestAuthorityModeIsExplicitAndImmutableForActiveTurn(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	bound, err := s.bindChatAuthorityMode(authorityModeFull, map[string]any{"conversation_id": "chat-authority"})
	if err != nil || firstStringFromMap(bound, "authority_mode") != authorityModeFull || !boolValue(bound["authority_mode_explicit"]) {
		t.Fatalf("bound=%+v err=%v", bound, err)
	}
	turn, err := experiment.NewTurn(experiment.Identity{ConversationID: "chat-authority"}, "test", gapTestAdmission(experiment.AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, ConversationID: "chat-authority", LoopID: "loop-authority", OriginalIntent: "test", Status: "reasoning", AuthorityMode: experiment.AuthorityFull, Experiment: &turn})
	if _, err := s.bindChatAuthorityMode(authorityModeManual, map[string]any{"conversation_id": "chat-authority"}); err == nil {
		t.Fatal("active Turn authority mode changed")
	}
}

func TestHandleTurnStopMarksGoalStoppedAndEmitsTrajectory(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "Stop.vit")
	if err := os.WriteFile(projectPath, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": "vitproj_stop"})
	s := New(nil, project, nil)
	s.activateCurrentProjectWorkspace(context.Background())
	goal := s.harness.BeginGoal("stop me")
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusWaitingContinue, nil)
	body, _ := json.Marshal(map[string]any{"conversation_id": "chat-stop", "goal_id": goal.GoalID, "run_id": goal.RunID, "turn_id": "turn-stop", "reason": "user_stop"})
	req := httptest.NewRequest(http.MethodPost, "/agent/turn/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	s.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	stopped := s.harness.RuntimeStatus(goal.GoalID)
	if stopped.Status != agentruntime.StatusStopped || stopped.LastCheckpoint == "" {
		t.Fatalf("goal=%+v", stopped)
	}
	events, _ := s.agentEventsSince("chat-stop", 0, 100)
	found := false
	for _, event := range events {
		if event.Type == string(trajectory.EventTurnStopped) {
			found = true
			if event.Payload["schema_version"] != trajectory.SchemaVersion {
				t.Fatalf("payload=%+v", event.Payload)
			}
		}
	}
	if !found {
		t.Fatalf("missing trajectory.turn.stopped events=%+v", events)
	}
}

func TestStopPreventsNewExperimentActivityAndPreservesCheckpointOnRuntimeRestore(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	goal := s.harness.BeginGoal("experiment stop")
	turn, err := experiment.NewTurn(experiment.Identity{ConversationID: "chat-experiment-stop", GoalID: goal.GoalID, RunID: goal.RunID, TurnID: "turn-experiment-stop"}, "bounded", gapTestAdmission(experiment.AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	s.authorityMode = authorityModeFull
	if _, err := turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-round", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, ConversationID: "chat-experiment-stop", LoopID: "loop-stop", GoalID: goal.GoalID, RunID: goal.RunID, OriginalIntent: "bounded", Status: "reasoning", AuthorityMode: experiment.AuthorityFull, Experiment: &turn})
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusWaitingContinue, nil)
	body, _ := json.Marshal(map[string]any{"conversation_id": "chat-experiment-stop", "goal_id": goal.GoalID, "run_id": goal.RunID, "turn_id": turn.ID})
	req := httptest.NewRequest(http.MethodPost, "/agent/turn/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	s.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	stopped, _ := s.freeStateLoop("chat-experiment-stop")
	if stopped.Status != "stopped" || stopped.Experiment == nil || stopped.Experiment.Status != experiment.StatusStopped {
		t.Fatalf("loop=%+v", stopped)
	}
	if _, err := stopped.Experiment.StartRound([]string{"track.transient"}, "checkpoint-2", "rev-2", time.Now().UTC()); err == nil {
		t.Fatal("stopped experiment accepted another round")
	}
	state := s.projectAgentRuntimeStateLocked()
	restarted := New(nil, shadow.New(nil), nil)
	restarted.restoreProjectAgentRuntimeStateLocked(state)
	if restarted.authorityMode != authorityModeFull {
		t.Fatalf("authority mode=%q", restarted.authorityMode)
	}
	restoredGoal := restarted.harness.RuntimeStatus(goal.GoalID)
	if restoredGoal.Status != agentruntime.StatusStopped {
		t.Fatalf("restored goal=%+v", restoredGoal)
	}
	restoredLoop, ok := restarted.freeStateLoop("chat-experiment-stop")
	if !ok || restoredLoop.Experiment == nil || restoredLoop.Experiment.Status != experiment.StatusStopped {
		t.Fatalf("restored loop=%+v ok=%v", restoredLoop, ok)
	}
}

func TestFullAccessRequestFlowsIntoExperimentAdmission(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	ctx, err := s.bindChatAuthorityMode(authorityModeFull, map[string]any{"conversation_id": "chat-flow"})
	if err != nil {
		t.Fatal(err)
	}
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-flow", ConversationID: "chat-flow", OriginalIntent: "improve", Status: "reasoning", AuthorityMode: authorityModeFromContext(ctx)}
	proposal := experimentTestProposal()
	admission, err := freeStateExperimentAdmission(loop, proposal)
	if err != nil {
		t.Fatal(err)
	}
	if admission.AuthorityMode != experiment.AuthorityFull {
		t.Fatalf("admission authority=%q", admission.AuthorityMode)
	}
}

func TestAuthorityEndpointPersistsExplicitMode(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	body := bytes.NewBufferString(`{"authority_mode":"full_project_access"}`)
	req := httptest.NewRequest(http.MethodPost, "/agent/authority", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	s.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || s.authorityMode != authorityModeFull {
		t.Fatalf("status=%d body=%s mode=%q", resp.Code, resp.Body.String(), s.authorityMode)
	}
	state := s.projectAgentRuntimeStateLocked()
	restarted := New(nil, shadow.New(nil), nil)
	restarted.restoreProjectAgentRuntimeStateLocked(state)
	if restarted.authorityMode != authorityModeFull {
		t.Fatalf("restored authority mode=%q", restarted.authorityMode)
	}
}
