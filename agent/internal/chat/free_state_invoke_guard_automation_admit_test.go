package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/planner"
)

// M3-AUTOMATION-GUARD-ADMIT-1: the two automation-surface admitted mutation
// commands pass the free-state invoke guard and continue into the downstream
// governance gates. Admission is not execution — the confirmation card stays
// the single authorization point, every other mutation (daw.invoke included)
// stays rejected, and nothing executes before confirmation.

func freeStateAutomationSetPointsToolCall() planner.ToolCall {
	return planner.ToolCall{
		Tool: "automation.set_points",
		Args: map[string]any{
			"plugin_id": "plg_1",
			"param_id":  "volume",
			"points": []any{
				map[string]any{"t": 0.0, "v": 0.3},
				map[string]any{"t": 30.0, "v": 0.6},
			},
		},
	}
}

func freeStateAutomationClearRegionToolCall() planner.ToolCall {
	return planner.ToolCall{
		Tool: "automation.clear_region",
		Args: map[string]any{
			"plugin_id": "plg_1",
			"param_id":  "volume",
			"region":    map[string]any{"start": 0.0, "end": 30.0},
		},
	}
}

// Card test 1 + 4: under an active free-state loop the admitted automation
// mutations pass the guard, reach the real harness path through the chat
// executor wrapper (the exact layer that used to reject them), and stop at a
// confirmation card — never an error, never a bare write, no execution
// receipt fields before confirmation.
func TestFreeStateAutomationAdmittedMutationsFormConfirmationCard(t *testing.T) {
	server := New(nil, nil, nil)
	workflowExecutor := pluginGrabberWorkflowExecutor{
		server: server,
		base:   executor.New(server.harness),
	}
	for _, call := range []planner.ToolCall{
		freeStateAutomationSetPointsToolCall(),
		freeStateAutomationClearRegionToolCall(),
	} {
		if reason := freeStateInvokeMutationReason(harness.InvokeRequest{
			Tool: call.Tool, Args: call.Args, Context: activeFreeStateInvokeContext(),
		}); reason != "" {
			t.Fatalf("%s should pass the free-state invoke guard, got rejection: %q", call.Tool, reason)
		}
		result, err := workflowExecutor.RunToolCall(nil, executor.Input{
			ToolCall: call,
			Context:  activeFreeStateInvokeContext(),
		})
		if err != nil {
			t.Fatalf("%s executor err=%v result=%+v", call.Tool, err, result)
		}
		if result.Status != "needs_confirmation" {
			t.Fatalf("%s must form a confirmation card (status=needs_confirmation), got status=%q command=%q error=%q result=%+v",
				call.Tool, result.Status, result.CommandName, result.Error, result.Result)
		}
		if !result.RequiresConfirmation {
			t.Fatalf("%s confirmation card must require confirmation", call.Tool)
		}
		if result.Error != "" {
			t.Fatalf("%s confirmation card must not be an error: %q", call.Tool, result.Error)
		}
		if performed, present := result.Result["mutation_performed"]; present {
			if flag, ok := performed.(bool); !ok || flag {
				t.Fatalf("%s zero-bare-write violated: mutation_performed=%v result=%+v", call.Tool, performed, result.Result)
			}
		}
		for _, executedField := range []string{"written", "removed_points", "point_count"} {
			if _, present := result.Result[executedField]; present {
				t.Fatalf("%s execution receipt field %q present before confirmation: %+v", call.Tool, executedField, result.Result)
			}
		}
	}
	// The confirmation card also forms outside a free-state loop: admitting
	// these commands through the guard did not weaken the confirmation gate
	// itself (Confirmed=false still stops at the card).
	result, err := workflowExecutor.RunToolCall(nil, executor.Input{ToolCall: freeStateAutomationSetPointsToolCall()})
	if err != nil || result.Status != "needs_confirmation" || !result.RequiresConfirmation {
		t.Fatalf("non-free-state confirmed=false path must still stop at the confirmation card: status=%q err=%v", result.Status, err)
	}
}

// Card test 2: everything else that mutates or refreshes stays rejected —
// the original read-only contract holds for the rest of the catalog, and an
// admitted automation name does not smuggle a companion non-admitted
// mutation past the guard.
func TestFreeStateAutomationAdmissionKeepsOtherMutationsRejected(t *testing.T) {
	for _, req := range []harness.InvokeRequest{
		{Tool: "plugin.set_parameter", Context: activeFreeStateInvokeContext()},
		{Tool: "rack.add_node", Context: activeFreeStateInvokeContext()},
		{Tool: "plugin_grabber.apply_eq_edits", Context: activeFreeStateInvokeContext()},
		{Tool: "automation.set_points", Args: map[string]any{"cmd": "plugin.set_parameter"}, Context: activeFreeStateInvokeContext()},
		{Tool: "automation.clear_region", Args: map[string]any{"cmd": "rack.add_node"}, Context: activeFreeStateInvokeContext()},
		{Tool: "ccb.observation_request", Args: map[string]any{"payload": map[string]any{"command": "plugin.set_parameter"}}, Context: activeFreeStateInvokeContext()},
	} {
		if reason := freeStateInvokeMutationReason(req); !strings.Contains(reason, "active free-state") {
			t.Fatalf("request %+v was not blocked: %q", req, reason)
		}
	}
}

// Card test 3: daw.invoke stays rejected in every shape — direct call, snake
// form, and as a companion command injected alongside an admitted automation
// name.
func TestFreeStateAutomationAdmissionKeepsDawInvokeRejected(t *testing.T) {
	for _, req := range []harness.InvokeRequest{
		{Tool: "daw.invoke", Context: activeFreeStateInvokeContext()},
		{Tool: "daw_invoke", Context: activeFreeStateInvokeContext()},
		{Tool: "daw.invoke", Args: map[string]any{"cmd": "automation.set_points"}, Context: activeFreeStateInvokeContext()},
		{Tool: "automation.set_points", Args: map[string]any{"cmd": "daw.invoke"}, Context: activeFreeStateInvokeContext()},
	} {
		if reason := freeStateInvokeMutationReason(req); !strings.Contains(reason, "daw.invoke is forbidden") {
			t.Fatalf("request %+v was not blocked as daw.invoke: %q", req, reason)
		}
	}
}
