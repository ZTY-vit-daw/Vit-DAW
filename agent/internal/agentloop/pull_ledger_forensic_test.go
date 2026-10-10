package agentloop

// FS-LEDGER-PERSIST-1 regression: a pull session runs three CCB tool rounds
// (catalog, mix scan, track views) before a needs_experiment terminal turn.
// The SMOKE runs 20261010_185519/201713 showed two independent persistence
// faults at the Result boundary: (1) the durable free-state loop never rode
// the Result envelope (contextruntime.Build is summarize-only, terminal
// results carry no continuation), leaving the chat-side snapshot import
// structurally dead; (2) the harness embeds typed bundle structs inside the
// map result, so the strict chat-side map assertion dropped every executed
// CCB row and only the last observation survived. This driver pins the
// agentloop half: the in-flight ledger accumulates every round AND the
// committed Result's ContextSnapshot carries the loop with both receipts.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/pullharness"
)

var _ llm.Completer = (*forensicPullCompleter)(nil)

type forensicPullCompleter struct {
	responses []string
	calls     [][]llm.Message
}

func (f *forensicPullCompleter) Complete(_ context.Context, _ config.EngineConfig, messages []llm.Message) (string, error) {
	f.calls = append(f.calls, append([]llm.Message(nil), messages...))
	if len(f.responses) == 0 {
		return `{"final":true,"reply":"done"}`, nil
	}
	out := f.responses[0]
	f.responses = f.responses[1:]
	return out, nil
}

type forensicCCBExecutor struct {
	bundles []map[string]any
	calls   int
}

func (f *forensicCCBExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	index := f.calls
	f.calls++
	if index >= len(f.bundles) {
		return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "ok", Result: map[string]any{"echo": true}}, nil
	}
	return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "ok", Result: map[string]any{"bundle": f.bundles[index], "status": f.bundles[index]["status"]}}, nil
}

func forensicCCBBundle(obsID string, targetKind, targetID string, views []string, omitViews []string) map[string]any {
	viewMap := map[string]any{}
	omissions := []string{}
	for _, view := range views {
		omitted := false
		for _, omit := range omitViews {
			if omit == view {
				omitted = true
			}
		}
		if omitted {
			omissions = append(omissions, view+": omitted by disclosure budget")
			continue
		}
		viewMap[view] = map[string]any{"status": "partial"}
	}
	return map[string]any{
		"schema_version":   "ccb_observation_bundle.v1",
		"observation_id":   obsID,
		"status":           "partial",
		"requested_views":  views,
		"target_ref":       map[string]any{"kind": targetKind, "id": targetID},
		"project_binding":  map[string]any{"project_revision": "4", "project_uuid": "vitproj_forensic", "project_epoch": "epoch_forensic"},
		"freshness":        map[string]any{"class": "current_observation", "project_revision": "4", "observed_at": "2026-10-10T10:56:49Z", "status": "partial"},
		"omission_reasons": omissions,
		"views":            viewMap,
		"audit_receipt": map[string]any{
			"schema_version":           "ccb_observation_receipt.v1",
			"receipt_id":               "ccbr_" + obsID,
			"requested_by":             "model",
			"scope":                    "full_project",
			"status":                   "partial",
			"project_revision":         "4",
			"rejection_reasons":        omissions,
			"model_requested_view_ids": views,
		},
	}
}

func forensicLedgerOf(session *pullSession) map[string]any {
	loop := messageLoopMapValue(session.state.input.Context["free_state_reasoning_loop"])
	return messageLoopMapValue(loop["observation_ledger"])
}

func forensicLedgerReceiptIDs(ledger map[string]any) []string {
	rows := messageLoopMapRows(ledger["receipts"])
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, firstMapText(row, "receipt_id"))
	}
	return ids
}

func TestForensicPullSessionLedgerSurvivesAcrossRounds(t *testing.T) {
	mix := forensicCCBBundle("obs_105649_mix", "project", "", []string{"mix.multitrack_relationship", "mix.frequency_relationship"}, []string{"mix.frequency_relationship"})
	track := forensicCCBBundle("obs_105707_track", "track", "1037", []string{"track.basic_energy", "track.timbre_frequency"}, nil)
	exec := &forensicCCBExecutor{bundles: []map[string]any{nil, mix, track}}
	fsObs := func(views ...string) string {
		viewJSON, _ := json.Marshal(views)
		return `"free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"partial","summary":"observe","requested_view_ids":` + string(viewJSON) + `}`
	}
	callViews := func(views ...string) string {
		viewJSON, _ := json.Marshal(views)
		return `"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":` + string(viewJSON) + `}}]`
	}
	completer := &forensicPullCompleter{responses: []string{
		`{"final":false,"reply":"先看目录"` + "," + fsObs("track.basic_energy") + `,"tool_calls":[{"tool":"ccb.observation_catalog","args":{}}]}`,
		`{"final":false,"reply":"请求全曲 mix 观察",` + fsObs("mix.multitrack_relationship", "mix.frequency_relationship") + "," + callViews("mix.multitrack_relationship", "mix.frequency_relationship") + `}`,
		`{"final":false,"reply":"锁定目标轨",` + fsObs("track.basic_energy", "track.timbre_frequency") + "," + callViews("track.basic_energy", "track.timbre_frequency") + `}`,
		`{"final":true,"reply":"诊断完成，形成提案。","free_state":{"schema_version":"free_state_decision.v1","status":"needs_experiment","evidence_status":"sufficient","summary":"proposal formed","improvement_proposal":{"schema_version":"improvement_proposal.v1","target":{"kind":"track","id":"1037"},"evidence_refs":["obs_105707_track"],"improvement_intent":"bounded attenuation","hypothesis":"attenuation improves harshness","expected_effect":"A/B comparable","action_domain":"track_gain","action_kind":"gain_adjust","parameter_bounds":{"delta_db":-1.5},"confidence":0.5}}}`,
	}}
	session, _ := newForensicPullSession(t, exec, completer)
	ctx := context.Background()

	// Round 1: catalog.
	step := forensicInterpret(t, session, ctx, completer)
	forensicExecuteCycle(t, session, ctx, step, "round1")
	// Round 2: mix scan.
	step = forensicInterpret(t, session, ctx, completer)
	forensicExecuteCycle(t, session, ctx, step, "round2")
	ledger := forensicLedgerOf(session)
	ids := forensicLedgerReceiptIDs(ledger)
	if len(ids) != 1 || ids[0] != "ccbr_obs_105649_mix" {
		t.Fatalf("after mix round, ledger receipts = %v, want [ccbr_obs_105649_mix]", ids)
	}
	// Round 3: track views.
	step = forensicInterpret(t, session, ctx, completer)
	forensicExecuteCycle(t, session, ctx, step, "round3")
	ledger = forensicLedgerOf(session)
	ids = forensicLedgerReceiptIDs(ledger)
	if len(ids) != 2 {
		t.Fatalf("after track round, ledger receipts = %v, want both receipts (mix+track)", ids)
	}
	// Final round: plain done reply → stopped.
	step = forensicInterpret(t, session, ctx, completer)
	if step.Disposition != pullharness.DispositionTerminal {
		t.Fatalf("final round step = %+v, want terminal", step)
	}
	if _, err := session.Return(ctx, step); err != nil {
		t.Fatalf("return: %v", err)
	}
	host := session.HostResult()
	observed := 0
	for _, record := range host.Executed {
		if fmt.Sprint(record["tool"]) == "ccb.observation_request" {
			observed++
		}
	}

	if observed != 2 {
		t.Fatalf("host result executed carries %d CCB observation records, want 2", observed)
	}
	// The snapshot must carry the ledger with both receipts.
	snapLoop := messageLoopMapValue(messageLoopMapValue(host.ContextSnapshot)["free_state_reasoning_loop"])
	snapLedger := messageLoopMapValue(snapLoop["observation_ledger"])
	if got := len(forensicLedgerReceiptIDs(snapLedger)); got != 2 {
		t.Fatalf("context snapshot ledger receipts = %d, want 2 (cross-round persistence defect)", got)
	}
}

func forensicInterpret(t *testing.T, session *pullSession, ctx context.Context, completer *forensicPullCompleter) pullharness.Step {
	t.Helper()
	if _, err := session.Attempt(ctx); err != nil {
		t.Fatalf("attempt: %v", err)
	}
	raw, err := completer.Complete(ctx, config.EngineConfig{}, nil)
	if err != nil {
		t.Fatalf("model call: %v", err)
	}
	step, err := session.Interpret(ctx, raw)
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	return step
}

func forensicExecuteCycle(t *testing.T, session *pullSession, ctx context.Context, step pullharness.Step, label string) {
	t.Helper()
	if step.Disposition != pullharness.DispositionContinue || len(step.Calls) == 0 {
		t.Fatalf("%s interpret step = %+v, want continue with calls", label, step)
	}
	executed, err := session.Execute(ctx, step.Calls)
	if err != nil {
		t.Fatalf("%s execute: %v", label, err)
	}
	if executed.Disposition != pullharness.DispositionContinue {
		t.Fatalf("%s execute step = %+v, want continue", label, executed)
	}
	if err := session.CloseCycle(ctx, label, nil); err != nil {
		t.Fatalf("%s close cycle: %v", label, err)
	}
}

func newForensicPullSession(t *testing.T, exec *forensicCCBExecutor, completer *forensicPullCompleter) (*pullSession, *MessageLoop) {
	t.Helper()
	rt := loopTestRuntime()
	l := &MessageLoop{
		Runtime:  rt,
		Client:   completer,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 8, MaxToolCalls: 12, MaxConsecutiveErrors: 2},
		Now:      func() time.Time { return time.Date(2026, 10, 10, 10, 56, 0, 0, time.UTC) },
	}
	r := l.runner()
	goal := r.ensureGoal("goal-fslp", "run-fslp", "forensic pull ledger")
	baseBudget := normalizeBudget(l.Budget)
	state := &runState{
		input: Input{
			GoalID: goal.GoalID, RunID: goal.RunID, UserText: "全曲诊断",
			Context: map[string]any{
				"free_state_reasoning_loop": map[string]any{
					"schema_version": "free_state_reasoning_loop.v1",
					"loop_id":        "loop-fslp",
					"status":         "reasoning",
					"cycle":          0,
				},
				"task_contract":                  map[string]any{"project_uuid": "vitproj_forensic", "project_revision": "4", "kind": "improvement"},
				"free_state_capacity_assessment": map[string]any{"capacity_level": "within_free_state", "selected_capability": "free_state"},
				"minimal_audio_closure": map[string]any{
					"project_uuid": "vitproj_forensic", "project_revision": "4",
					"phase":             "fs2_capacity_assessed",
					"observations":      map[string]any{},
					"observation_order": []any{},
				},
			},
			Budget: baseBudget,
		},
		goal:               goal,
		budget:             baseBudget,
		continuationBudget: baseBudget,
		startedAt:          r.now(),
	}
	if opened, slice, ok := rt.BeginSlice(goal.GoalID, baseBudget.MaxTurns, baseBudget.MaxToolCalls); ok {
		state.goal = opened
		state.input.SliceID = slice.SliceID
		if openedGoal, turn, turnOK := rt.BeginTurn(goal.GoalID, slice.SliceID, "user"); turnOK {
			state.goal = openedGoal
			state.input.TurnID = turn.TurnID
		}
	}
	session, err := newPullSession(l, r, state, nil)
	if err != nil {
		t.Fatalf("newPullSession: %v", err)
	}
	return session, l
}
