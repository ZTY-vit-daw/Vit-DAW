package chat

// FIX-TURNSTOP-WEDGE-1 regression suite: a user stop on an actively executing
// goal used to wedge forever when the runner had no in-flight polling point
// (M2-EXPERIENCE-1: goal/turn stopped, task=active/slice=running frozen 10
// minutes; the second stop's finalizeStoppedTurn inline branch returned ok
// without converging anything because MarkStopped never touched the task
// bookkeeping). The three scenes below pin the fix:
//
//  1. stalled runner — handleTurnStop's settle window expires and the inline
//     finalize fallback converges goal/task/slice to terminal states;
//  2. second stop — idempotent: no duplicate trajectory event, no checkpoint
//     churn, states stay converged;
//  3. live runner — a runner checkpoint landing inside the window is left
//     alone (no inline finalize racing the real settle chain) and the same
//     task/slice convergence applies on the runner's own MarkStopped path.

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

	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/trajectory"
)

func turnStopWedgeServer(t *testing.T, settleWindow time.Duration) (*Server, agentruntime.Goal) {
	t.Helper()
	projectPath := filepath.Join(t.TempDir(), "Stop.vit")
	if err := os.WriteFile(projectPath, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": "vitproj_turnstop"})
	s := New(nil, project, nil)
	s.turnStopSettleWindowOverride = settleWindow
	s.activateCurrentProjectWorkspace(context.Background())
	goal := s.harness.BeginGoal("stop me")
	if goal.Status != agentruntime.StatusRunning {
		t.Fatalf("goal must start actively executing, got %s", goal.Status)
	}
	if _, slice, ok := s.harness.Runtime().BeginSlice(goal.GoalID, 0, 0); !ok {
		t.Fatal("slice was not opened")
	} else if slice.Status != "running" {
		t.Fatalf("slice must start running: %+v", slice)
	}
	return s, goal
}

func postTurnStop(t *testing.T, s *Server, conversationID, goalID string) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"conversation_id": conversationID, "goal_id": goalID, "reason": "user_stop"})
	req := httptest.NewRequest(http.MethodPost, "/agent/turn/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	s.Routes().ServeHTTP(resp, req)
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	return resp.Code, payload
}

func assertTurnStopConverged(t *testing.T, s *Server, goalID, wantCheckpoint string) agentruntime.Goal {
	t.Helper()
	goal := s.harness.RuntimeStatus(goalID)
	if goal.Status != agentruntime.StatusStopped || !goal.StopRequested {
		t.Fatalf("goal must be stopped with stop latched: %+v", goal)
	}
	if goal.Task == nil {
		t.Fatalf("task missing: %+v", goal)
	}
	if goal.Task.Status != agentruntime.TaskStatusCancelled {
		t.Fatalf("task must converge cancelled for a contract-less stopped turn: %s", goal.Task.Status)
	}
	if wantCheckpoint != "" && goal.LastCheckpoint != wantCheckpoint {
		t.Fatalf("checkpoint must stay %q, got %q", wantCheckpoint, goal.LastCheckpoint)
	}
	slice := goal.Task.Run.CurrentSliceID
	for _, item := range goal.Task.Run.Slices {
		if item.SliceID != slice {
			continue
		}
		if item.Status != "cancelled" || item.EndedAt.IsZero() {
			t.Fatalf("current slice must finalize cancelled with EndedAt: %+v", item)
		}
	}
	return goal
}

// Scene 1: the runner never lands the stop (stalled — no polling point will
// come). The settle window expires and the inline finalize fallback converges
// goal/task/slice inside the request instead of wedging forever.
func TestTurnStopStalledRunnerConvergesTaskAndSlice(t *testing.T) {
	s, goal := turnStopWedgeServer(t, 100*time.Millisecond)
	code, payload := postTurnStop(t, s, "conversation-turnstop-wedge", goal.GoalID)
	if code != http.StatusOK {
		t.Fatalf("status=%d payload=%v", code, payload)
	}
	if payload["goal_status"] != string(agentruntime.StatusStopped) {
		t.Fatalf("payload goal_status=%v", payload["goal_status"])
	}
	stopped := assertTurnStopConverged(t, s, goal.GoalID, "")
	if stopped.LastCheckpoint == "" {
		t.Fatalf("inline finalize must record a stop checkpoint: %+v", stopped)
	}
}

// Scene 2: a second stop on the already-stopped goal stays idempotent —
// states hold, exactly one turn-stopped trajectory event, and the checkpoint
// recorded by the first stop is not replaced.
func TestTurnStopSecondStopIsIdempotent(t *testing.T) {
	s, goal := turnStopWedgeServer(t, 100*time.Millisecond)
	code, _ := postTurnStop(t, s, "conversation-turnstop-wedge", goal.GoalID)
	if code != http.StatusOK {
		t.Fatalf("first stop status=%d", code)
	}
	stopped := assertTurnStopConverged(t, s, goal.GoalID, "")

	code, second := postTurnStop(t, s, "conversation-turnstop-wedge", goal.GoalID)
	if code != http.StatusOK {
		t.Fatalf("second stop status=%d payload=%v", code, second)
	}
	if second["goal_status"] != string(agentruntime.StatusStopped) {
		t.Fatalf("second stop goal_status=%v", second["goal_status"])
	}
	assertTurnStopConverged(t, s, goal.GoalID, stopped.LastCheckpoint)

	events, _ := s.agentEventsSince("conversation-turnstop-wedge", 0, 100)
	count := 0
	for _, event := range events {
		if event.Type == string(trajectory.EventTurnStopped) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("turn-stopped trajectory event must stay deduplicated, got %d", count)
	}
}

// Scene 3 (real-settle protection): when the runner's own checkpoint lands
// the stop inside the window, the handler observes the converged state and
// does NOT run the inline finalize — the runner keeps its checkpoint and the
// settle chain it owns. The runner-side MarkStopped converges task/slice too.
func TestTurnStopRunnerLandsWithinWindowSkipsInlineFinalize(t *testing.T) {
	s, goal := turnStopWedgeServer(t, 5*time.Second)
	landed := make(chan struct{})
	go func() {
		defer close(landed)
		time.Sleep(50 * time.Millisecond)
		s.harness.MarkGoalStopped(goal.GoalID, "checkpoint-runner-lands")
	}()
	code, payload := postTurnStop(t, s, "conversation-turnstop-runner", goal.GoalID)
	<-landed
	if code != http.StatusOK {
		t.Fatalf("status=%d payload=%v", code, payload)
	}
	// The inline fallback never ran: the response carries no checkpoint_ref of
	// its own and the runner's checkpoint is the surviving one.
	if ref, _ := payload["checkpoint_ref"].(string); ref != "" {
		t.Fatalf("handler must not fabricate a checkpoint when the runner landed the stop: %q", ref)
	}
	assertTurnStopConverged(t, s, goal.GoalID, "checkpoint-runner-lands")
}

// The settle window defaults to the documented bound and honors the
// server-level override (tests inject the short window above).
func TestTurnStopSettleWindowResolution(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	if got := s.turnStopSettleWindow(); got != turnStopSettleWindowDefault {
		t.Fatalf("default window=%s want=%s", got, turnStopSettleWindowDefault)
	}
	s.turnStopSettleWindowOverride = 250 * time.Millisecond
	if got := s.turnStopSettleWindow(); got != 250*time.Millisecond {
		t.Fatalf("override window=%s", got)
	}
}
