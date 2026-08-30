package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	agentruntime "vit-daw-agent/internal/runtime"
)

// D2-2-CLEAN1-1: the read-side lease of the bare-continue interception gate.
// The write side already applies continuationOccupantDrivable (S3h7 arm and
// restore-migration displacement, S3h8 completion bridge), but the gate's scan
// (interactionContinuationForConversation) returned the newest waiting_interaction
// park unfiltered: an unanswerable legacy_waiting_continue shell — still
// materialized by the pre-C restore migration's compatibility branch — or any
// terminal residue that ever slipped past the write-side guards would answer
// every "继续" with pending_interaction_requires_response and swallow the nudge
// permanently (the read-side replica of the S3h7 death chain's fifth step,
// CLEAN1 audit F7/L1: lease on the read side = no).

const leaseReadConversation = "conversation-lease-read"

func leaseReadServer() *Server {
	return New(nil, nil, nil)
}

func storeLeaseReadPark(s *Server, durable DurableContinuation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.durableContinuations[durable.ContinuationID] = durable
}

func leaseShellPark(updatedAt time.Time) DurableContinuation {
	return DurableContinuation{
		ContinuationID: "cont-lease-shell", TaskID: "task-lease-read", GoalID: "goal-lease-read", RunID: "run-lease-read",
		ConversationID: leaseReadConversation, OriginalIntent: "inspect the project",
		Continuation: agentloop.Continuation{GoalID: "goal-lease-read", RunID: "run-lease-read", TaskID: "task-lease-read",
			SliceID: "slice-lease-shell", TurnID: "turn-lease-shell", OriginalIntent: "inspect the project"},
		Status:            ContinuationWaitingInteraction,
		PendingInteraction: map[string]any{"status": "legacy_waiting_continue", "reason": "legacy checkpoint has no authoritative stop reason"},
		CreatedAt:         updatedAt, UpdatedAt: updatedAt,
	}
}

func leaseConfirmationPark(updatedAt time.Time) DurableContinuation {
	return DurableContinuation{
		ContinuationID: "cont-lease-confirmation", TaskID: "task-lease-read", GoalID: "goal-lease-read", RunID: "run-lease-read",
		ConversationID: leaseReadConversation, OriginalIntent: "inspect the project",
		Continuation: agentloop.Continuation{GoalID: "goal-lease-read", RunID: "run-lease-read", TaskID: "task-lease-read",
			SliceID: "slice-lease-confirmation", TurnID: "turn-lease-confirmation", OriginalIntent: "inspect the project"},
		Status: ContinuationWaitingInteraction,
		PendingInteraction: map[string]any{
			"status": "waiting_confirmation", "interaction_id": "interaction-lease-1", "continuation_id": "cont-lease-confirmation",
			"requests": []any{map[string]any{"request_id": "interaction-lease-1", "kind": "confirmation", "prompt": "Confirm this bounded action"}},
		},
		CreatedAt: updatedAt, UpdatedAt: updatedAt,
	}
}

func storeLeaseReadInteraction(s *Server) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interactions["interaction-lease-1"] = PendingInteraction{
		ID: "interaction-lease-1", Kind: "mix_tick_confirmation", Stage: "pending_confirmation",
		ConversationID: leaseReadConversation, GoalID: "goal-lease-read", RunID: "run-lease-read",
		Payload: map[string]any{"display": map[string]any{"summary": "确认应用这条增益调整"}},
	}
}

func continueNudge(t *testing.T, s *Server) ChatResponse {
	t.Helper()
	resp, handled := s.runAgentLoopChat(context.Background(), leaseReadConversation, ChatRequest{
		ConversationID: leaseReadConversation, Message: "继续",
	}, config.EngineConfig{})
	if !handled {
		t.Fatal("bare continue nudge was not handled")
	}
	return resp
}

// The reproduction: an unanswerable legacy shell park must not capture the
// nudge. The gate's scan applies the same lease predicate as the arm slot, and
// the nudge falls through to the honest continue path (the durable fallback
// drive of the pre-C checkpoint, or no_continuation when nothing is drivable)
// instead of being swallowed behind a boundary nothing can ever answer.
func TestContinueNudgeDoesNotSwallowLegacyWaitingContinueShell(t *testing.T) {
	s := leaseReadServer()
	storeLeaseReadPark(s, leaseShellPark(time.Now().UTC()))
	if item, ok := s.interactionContinuationForConversation(leaseReadConversation); ok {
		t.Fatalf("read side surfaced a non-drivable legacy shell park: %+v", item)
	}
	resp := continueNudge(t, s)
	if resp.StopReason == "pending_interaction_requires_response" {
		t.Fatalf("legacy shell swallowed the bare continue nudge: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
	if resp.Workflow == "durable_continuation" {
		t.Fatalf("legacy shell still produced the interception envelope: stop=%q reply=%q data=%+v",
			resp.StopReason, resp.Reply, resp.WorkflowData)
	}
	if strings.Contains(resp.Reply, "不会越过这个交互边界") {
		t.Fatalf("legacy shell still produced the interception reply: %q", resp.Reply)
	}
}

// Red line (control): a drivable waiting_confirmation park — a real unanswered
// interaction boundary with a stored answerable request — keeps the exact
// interception semantics, reply text, and InteractionRequests surfacing.
func TestContinueNudgeStillInterceptsDrivableWaitingConfirmation(t *testing.T) {
	s := leaseReadServer()
	storeLeaseReadPark(s, leaseConfirmationPark(time.Now().UTC()))
	storeLeaseReadInteraction(s)
	if item, ok := s.interactionContinuationForConversation(leaseReadConversation); !ok || item.ContinuationID != "cont-lease-confirmation" {
		t.Fatalf("read side lost the drivable confirmation park: %+v ok=%v", item, ok)
	}
	resp := continueNudge(t, s)
	if resp.StopReason != "pending_interaction_requires_response" {
		t.Fatalf("drivable confirmation park lost its interception: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
	if resp.Reply != "当前任务正在等待明确的确认、澄清或人工判断；单独说‘继续’不会越过这个交互边界。" {
		t.Fatalf("interception reply text changed: %q", resp.Reply)
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) || resp.Workflow != "durable_continuation" {
		t.Fatalf("interception status/workflow changed: status=%q workflow=%q", resp.GoalStatus, resp.Workflow)
	}
	if resp.WorkflowData["continuation_id"] != "cont-lease-confirmation" {
		t.Fatalf("interception surfaced the wrong park: %+v", resp.WorkflowData)
	}
	if len(resp.InteractionRequests) != 1 || resp.InteractionRequests[0].ID != "interaction-lease-1" {
		t.Fatalf("interception did not surface the answerable request: %+v", resp.InteractionRequests)
	}
}

// A legacy shell materialized after a real confirmation (the restore migration's
// compatibility branch writing over a parked boundary) must not shadow it: the
// scan keeps returning the newest drivable park, and the real unanswered
// confirmation keeps the gate.
func TestLegacyShellDoesNotShadowDrivableConfirmationPark(t *testing.T) {
	s := leaseReadServer()
	now := time.Now().UTC()
	storeLeaseReadPark(s, leaseConfirmationPark(now))
	storeLeaseReadPark(s, leaseShellPark(now.Add(2*time.Second)))
	storeLeaseReadInteraction(s)
	item, ok := s.interactionContinuationForConversation(leaseReadConversation)
	if !ok || item.ContinuationID != "cont-lease-confirmation" {
		t.Fatalf("newest-wins scan let the legacy shell shadow the drivable park: %+v ok=%v", item, ok)
	}
	resp := continueNudge(t, s)
	if resp.StopReason != "pending_interaction_requires_response" || resp.WorkflowData["continuation_id"] != "cont-lease-confirmation" {
		t.Fatalf("nudge was not gated by the drivable confirmation park: stop=%q data=%+v", resp.StopReason, resp.WorkflowData)
	}
	if len(resp.InteractionRequests) != 1 || resp.InteractionRequests[0].ID != "interaction-lease-1" {
		t.Fatalf("shadowed confirmation lost its answerable request surface: %+v", resp.InteractionRequests)
	}
}

// Terminal residue (the S3h7 displacement bookkeeping's cancelled shell) never
// reaches the gate: the status filter and the lease predicate agree on it, and
// the nudge falls through to the honest continue path.
func TestContinueNudgeIgnoresTerminalResiduePark(t *testing.T) {
	s := leaseReadServer()
	now := time.Now().UTC()
	residue := leaseShellPark(now)
	residue.ContinuationID = "cont-lease-terminal"
	residue.Status = ContinuationCancelled
	residue.PendingInteraction = map[string]any{"status": "displaced_by_recalibration_arm"}
	residue.UpdatedAt = now.Add(time.Second)
	storeLeaseReadPark(s, residue)
	if item, ok := s.interactionContinuationForConversation(leaseReadConversation); ok {
		t.Fatalf("read side surfaced a terminal residue park: %+v", item)
	}
	if drivable := continuationOccupantDrivable(residue.Continuation, &residue); drivable {
		t.Fatal("lease predicate classified a cancelled residue as drivable")
	}
	resp := continueNudge(t, s)
	if resp.StopReason == "pending_interaction_requires_response" {
		t.Fatalf("terminal residue swallowed the bare continue nudge: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
	if resp.StopReason != "no_continuation" {
		t.Fatalf("nudge did not fall through to the honest continue path: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
}
