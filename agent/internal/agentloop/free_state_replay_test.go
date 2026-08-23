package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

// Matrix M11–M17 (docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md §2/§3): L2
// transcript replay. Fixtures live in testdata/free_state_replay/ and record
// their artifact source, sha256, and transform type. Replay is fully
// deterministic: the fake completer supplies the model turns and the fake
// executor supplies CCB bundles, so zero network and zero real model requests
// happen here. Assertions never hardcode volatile identifiers (track names,
// conversation ids); only contract-level view ids and statuses are asserted.

type replayFixture struct {
	Family      string           `json:"family"`
	MatrixIDs   []string         `json:"matrix_ids"`
	Source      map[string]any   `json:"source"`
	Intent      string           `json:"intent"`
	Context     map[string]any   `json:"context"`
	Responses   []string         `json:"responses"`
	ErrorAtTurn []int            `json:"completer_error_at_turn"`
	ToolResults []map[string]any `json:"tool_results"`
	Expected    map[string]any   `json:"expected"`
}

type replayExecutor struct {
	results  []map[string]any
	calls    []planner.ToolCall
	rejected []string
}

// knownReplayViews mirrors the CCB catalog identifiers the observation layer
// can actually serve; anything else is a malformed view id.
var knownReplayViews = map[string]bool{
	"project.structure": true, "track.basic_energy": true, "track.peak_structure": true,
	"track.timbre_frequency": true, "track.time_dynamics": true,
	"mix.frequency_relationship": true, "mix.multitrack_relationship": true,
	"comparison.before_after": true,
}

func (e *replayExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	e.calls = append(e.calls, in.ToolCall)
	views := messageLoopStringList(in.ToolCall.Args["view_ids"])
	for _, viewID := range views {
		if !knownReplayViews[viewID] {
			e.rejected = append(e.rejected, viewID)
			return executorpkg.Result{
				ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, CommandName: "ccb_observation_request", Status: "rejected",
				Result: map[string]any{"status": "rejected", "bundle": map[string]any{
					"schema_version": "ccb_observation_bundle.v1", "status": "rejected", "read_only": true,
					"observation_id": "obs-replay-rejected", "requested_views": views,
					"views": map[string]any{}, "freshness": map[string]any{"status": "rejected"},
					"omission_reasons": []any{"unknown view id: " + viewID},
					"audit_receipt": map[string]any{
						"schema_version": "ccb_observation_receipt.v1", "receipt_id": "ccbr-replay-rejected", "status": "rejected",
						"rejection_reasons": []any{"unknown view id: " + viewID},
					},
				}},
			}, nil
		}
	}
	if len(e.results) == 0 {
		return executorpkg.Result{
			ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, CommandName: "ccb_observation_request", Status: "ok",
			Result: map[string]any{"status": "ready", "bundle": map[string]any{
				"schema_version": "ccb_observation_bundle.v1", "status": "ready", "read_only": true,
				"observation_id": "obs-replay-default", "requested_views": views, "views": map[string]any{},
			}},
		}, nil
	}
	scripted := e.results[0]
	e.results = e.results[1:]
	bundle := map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": scripted["status"], "read_only": true,
		"observation_id": scripted["observation_id"], "requested_views": views,
		"project_binding": map[string]any{"project_revision": scripted["project_revision"]},
		"freshness":       map[string]any{"status": scripted["freshness"], "project_revision": scripted["project_revision"]},
		"views":           map[string]any{},
	}
	if limitations, ok := scripted["limitations"].([]any); ok {
		bundle["limitations"] = limitations
	}
	for _, viewID := range views {
		view := map[string]any{"status": scripted["status"]}
		if limitations, ok := scripted["limitations"].([]any); ok {
			view["limitations"] = limitations
		}
		bundle["views"].(map[string]any)[viewID] = view
	}
	return executorpkg.Result{
		ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, CommandName: "ccb_observation_request", Status: "ok",
		Result: map[string]any{"status": scripted["status"], "bundle": bundle},
	}, nil
}

func loadReplayFixture(t *testing.T, name string) replayFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "free_state_replay", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	fx := replayFixture{}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	if strings.TrimSpace(fx.Intent) == "" || len(fx.Responses) == 0 {
		t.Fatalf("fixture %s missing intent/responses", name)
	}
	if hash, ok := fx.Source["sha256"].(string); !ok || len(hash) < 16 { // provenance discipline
		t.Fatalf("fixture %s missing source sha256", name)
	}
	return fx
}

func (fx replayFixture) startReplay(executor *replayExecutor, responses []string, errorAtTurn []int, contextOverride map[string]any) (Result, *fakeMessageCompleter) {
	errs := make([]error, 0, len(responses))
	errorSet := map[int]bool{}
	for _, index := range errorAtTurn {
		errorSet[index] = true
	}
	for i := range responses {
		if errorSet[i] {
			errs = append(errs, errors.New("context deadline exceeded"))
		} else {
			errs = append(errs, nil)
		}
	}
	client := &fakeMessageCompleter{responses: responses, errors: errs}
	loop := MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Executor: executor, Budget: Budget{MaxTurns: 8, MaxToolCalls: 8, MaxConsecutiveErrors: 2},
	}
	result := loop.Start(context.Background(), Input{
		UserText: fx.Intent, Context: contextOverride, AllowedTools: []string{"ccb.observation_request"},
	})
	return result, client
}

func replayTraceContains(result Result, kind, needle string) bool {
	for _, event := range result.Trace {
		if event.Kind == kind && strings.Contains(event.Message, needle) {
			return true
		}
	}
	return false
}

func replayAssertCommon(t *testing.T, name string, result Result, client *fakeMessageCompleter, executor *replayExecutor, fx replayFixture) {
	t.Helper()
	expected := fx.Expected
	if want := expected["terminal_status"].(string); result.FreeStateDecision == nil || result.FreeStateDecision.Status != want {
		t.Fatalf("%s: terminal decision = %+v, want status %s (error=%q)", name, result.FreeStateDecision, want, result.Error)
	}
	if want, ok := expected["model_calls"].(float64); ok && len(client.calls) != int(want) {
		t.Fatalf("%s: model calls = %d, want %d", name, len(client.calls), int(want))
	}
	if want, ok := expected["observation_calls"].(float64); ok && len(executor.calls) != int(want) {
		t.Fatalf("%s: observation calls = %d, want %d (%+v)", name, len(executor.calls), int(want), executor.calls)
	}
	if min, ok := expected["protocol_repairs_min"].(float64); ok && float64(result.ModelProtocolRepairs) < min {
		t.Fatalf("%s: protocol repairs = %d, want >= %d", name, result.ModelProtocolRepairs, int(min))
	}
}

// M11: a valid transcript replays to the same terminal decision with exactly
// the scripted model turns — no new model requests are issued.
func TestM11ReplayValidReachesSameTerminal(t *testing.T) {
	fx := loadReplayFixture(t, "valid.json")
	executor := &replayExecutor{results: append([]map[string]any(nil), fx.ToolResults...)}
	result, client := fx.startReplay(executor, fx.Responses, nil, fx.Context)
	replayAssertCommon(t, "valid", result, client, executor, fx)
	if result.StopReason == StopReasonModelProtocolFailure {
		t.Fatalf("valid replay hit a protocol failure: %q", result.Error)
	}
}

// M12: malformed transcripts (truncated JSON, unknown status, bad view id) are
// rejected with auditable errors instead of silently passing.
func TestM12ReplayMalformedRejectedAndAuditable(t *testing.T) {
	for _, name := range []string{"malformed_truncated.json", "malformed_unknown_status.json", "malformed_bad_view_id.json"} {
		fx := loadReplayFixture(t, name)
		executor := &replayExecutor{results: append([]map[string]any(nil), fx.ToolResults...)}
		result, client := fx.startReplay(executor, fx.Responses, nil, fx.Context)
		replayAssertCommon(t, name, result, client, executor, fx)
		if _, rejected := fx.Expected["rejected_observation"]; rejected && len(executor.rejected) == 0 {
			t.Fatalf("%s: unknown view id was not rejected by the observation layer", name)
		}
		retainedRejection := result.RecentObservation != nil &&
			strings.EqualFold(strings.TrimSpace(messageLoopText(result.RecentObservation.Summary["status"])), "rejected")
		if _, min := fx.Expected["protocol_repairs_min"]; !min && !replayTraceContains(result, "final_gate", "free_state") && !retainedRejection {
			t.Fatalf("%s: rejection is not auditable (no protocol repair, no final_gate trace, no retained rejection receipt)", name)
		}
	}
}

// M13: a repeated same-view-set request is rejected from the second occurrence
// on (M04 rule); the observation executes exactly once per view set.
func TestM13ReplayRepetitiveRejectedFromSecondRequest(t *testing.T) {
	fx := loadReplayFixture(t, "repetitive.json")
	executor := &replayExecutor{results: append([]map[string]any(nil), fx.ToolResults...)}
	result, client := fx.startReplay(executor, fx.Responses, nil, fx.Context)
	replayAssertCommon(t, "repetitive", result, client, executor, fx)
	if needle := fx.Expected["repeat_rejection"].(string); !replayTraceContains(result, "final_gate", needle) {
		t.Fatalf("repetitive: repeat rejection not auditable in trace (want %q)", needle)
	}
	duplicates := map[string]int{}
	for _, call := range executor.calls {
		key := strings.Join(messageLoopStringList(call.Args["view_ids"]), ",")
		duplicates[key]++
	}
	for key, count := range duplicates {
		if count > 1 {
			t.Fatalf("repetitive: view set %q executed %d times", key, count)
		}
	}
}

// M14: a partial bundle is preserved as partial; readiness is not upgraded.
func TestM14ReplayPartialPreservedReadinessNotUpgraded(t *testing.T) {
	fx := loadReplayFixture(t, "partial.json")
	executor := &replayExecutor{results: append([]map[string]any(nil), fx.ToolResults...)}
	result, client := fx.startReplay(executor, fx.Responses, nil, fx.Context)
	replayAssertCommon(t, "partial", result, client, executor, fx)
	if result.RecentObservation == nil ||
		!strings.EqualFold(strings.TrimSpace(messageLoopText(result.RecentObservation.Summary["status"])), "partial") {
		t.Fatalf("partial: recent observation = %+v, want bundle status partial", result.RecentObservation)
	}
	if result.FreeStateDecision != nil && strings.EqualFold(strings.TrimSpace(result.FreeStateDecision.EvidenceStatus), "sufficient") {
		t.Fatal("partial: readiness upgraded to sufficient from a partial bundle")
	}
}

// M15: improvement-proposal refs bound to an old project revision are rejected
// by gate G7; the loop re-requests a fresh revision-bound view.
func TestM15ReplayStaleRefsRejectedByGate(t *testing.T) {
	fx := loadReplayFixture(t, "stale.json")
	executor := &replayExecutor{results: append([]map[string]any(nil), fx.ToolResults...)}
	result, client := fx.startReplay(executor, fx.Responses, nil, fx.Context)
	replayAssertCommon(t, "stale", result, client, executor, fx)
	if needle := fx.Expected["gate_rejection"].(string); !replayTraceContains(result, "final_gate", needle) {
		t.Fatalf("stale: G7 rejection not auditable in trace (want %q)", needle)
	}
}

// M16: contradictory rounds must trigger a revisiting observation or close the
// candidate; they never pass silently as satisfied.
func TestM16ReplayContradictionRevisitedNotSilentlyPassed(t *testing.T) {
	fx := loadReplayFixture(t, "contradictory.json")
	executor := &replayExecutor{results: append([]map[string]any(nil), fx.ToolResults...)}
	result, client := fx.startReplay(executor, fx.Responses, nil, fx.Context)
	replayAssertCommon(t, "contradictory", result, client, executor, fx)
	if result.FreeStateDecision.Status == FreeStateSatisfied {
		t.Fatal("contradictory: conflicting evidence silently settled as satisfied")
	}
	if len(executor.calls) < 2 {
		t.Fatalf("contradictory: no discriminating revisit executed (%+v)", executor.calls)
	}
}

// M17: a transient transport/LLM failure after an executed observation pauses
// the loop; the resume continues and does not repeat the executed observation.
func TestM17ReplayTransientErrorRecoversWithoutRepeatingObservation(t *testing.T) {
	fx := loadReplayFixture(t, "transient_error.json")
	executor := &replayExecutor{results: append([]map[string]any(nil), fx.ToolResults...)}
	// First leg: turn 0 succeeds (observation executes), turn 1 fails with a
	// transient transport error.
	first, firstClient := fx.startReplay(executor, fx.Responses[:2], []int{1}, fx.Context)
	if _, ok := fx.Expected["transient_stop_reason"]; ok && first.StopReason != StopReasonTransientLLMError {
		t.Fatalf("transient: first leg stop reason = %q, want %s (error=%q)", first.StopReason, StopReasonTransientLLMError, first.Error)
	}
	executedAfterFirst := len(executor.calls)
	if first.Continuation == nil {
		t.Fatal("transient: paused result carries no continuation to resume from")
	}
	loopState := map[string]any{}
	if raw, ok := first.Continuation.Context["free_state_reasoning_loop"]; ok {
		if m, ok := raw.(map[string]any); ok {
			loopState = m
		}
	}
	if len(loopState) == 0 {
		t.Fatal("transient: continuation context carries no free-state loop state")
	}
	resumeContext := map[string]any{"free_state_reasoning_loop": loopState}
	second, _ := fx.startReplay(executor, fx.Responses[1:], nil, resumeContext)
	if second.FreeStateDecision == nil || second.FreeStateDecision.Status != FreeStateBlocked {
		t.Fatalf("transient: resume terminal = %+v (error=%q)", second.FreeStateDecision, second.Error)
	}
	if len(executor.calls) != executedAfterFirst {
		t.Fatalf("transient: resume repeated an executed observation: %d -> %d calls", executedAfterFirst, len(executor.calls))
	}
	_ = firstClient
}
