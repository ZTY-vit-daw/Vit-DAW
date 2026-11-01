package agentloop

// M3-AUTOMATION-RESULT-DISCLOSE-1 layer 1 tests: the automation read tools
// (plugin.get_parameters / automation.get_curve) disclose a compact parameter
// surface through recentObservationForTool, while the existing Mix/CCB summary
// paths keep their exact shapes (zero-regression branch).

import (
	"fmt"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

func automationDisclosureResult(tool, commandName string, result map[string]any) executorpkg.Result {
	return executorpkg.Result{
		Status:      "ok",
		Tool:        tool,
		CommandName: commandName,
		ToolCallID:  "call_auto_1",
		Result:      result,
	}
}

func TestRecentObservationForToolDisclosesAutomationParamSurface(t *testing.T) {
	call := planner.ToolCall{ID: "call_auto_1", Tool: "plugin.get_parameters"}
	result := automationDisclosureResult("plugin.get_parameters", "get_plugin_parameters", map[string]any{
		"status":         "ok",
		"track_id":       "1010",
		"plugin_id":      "1011",
		"parameter_count": 2,
		"automation_param_surface": []any{
			map[string]any{"param_id": "volume", "label": "Volume", "min": -80.0, "max": 6.0, "supports_automation": true},
			map[string]any{"param_id": "pan", "label": "Pan", "min": -1.0, "max": 1.0, "supports_automation": true},
		},
	})
	recent := recentObservationForTool(call, result, planner.VerificationResult{}, false, nil)
	if recent.Summary == nil {
		t.Fatalf("plugin.get_parameters summary collapsed to nil: %+v", recent)
	}
	if recent.Summary["plugin_id"] != "1011" || recent.Summary["parameter_count"] != 2 {
		t.Fatalf("summary metadata = %+v", recent.Summary)
	}
	rows := messageLoopMapRows(recent.Summary["automation_param_surface"])
	if len(rows) != 2 {
		t.Fatalf("automation_param_surface rows = %+v", recent.Summary["automation_param_surface"])
	}
	if rows[0]["param_id"] != "volume" || rows[0]["min"] != -80.0 || rows[0]["max"] != 6.0 {
		t.Fatalf("volume row = %+v", rows[0])
	}
	if rows[1]["param_id"] != "pan" || rows[1]["min"] != -1.0 {
		t.Fatalf("pan row = %+v", rows[1])
	}
}

func TestRecentObservationForToolDisclosesAutomationCurveReadback(t *testing.T) {
	points := make([]any, 0, 24)
	for i := 0; i < 24; i++ {
		points = append(points, map[string]any{"t": float64(i), "v": float64(i) / 24.0, "c": 0})
	}
	call := planner.ToolCall{ID: "call_auto_1", Tool: "automation.get_curve"}
	result := automationDisclosureResult("automation.get_curve", "automation.get_curve", map[string]any{
		"status":      "ok",
		"plugin_id":   "1011",
		"param_id":    "volume",
		"time_base":   "time",
		"bypass":      false,
		"point_count": 24,
		"points":      points,
	})
	recent := recentObservationForTool(call, result, planner.VerificationResult{}, false, nil)
	if recent.Summary == nil {
		t.Fatalf("automation.get_curve summary collapsed to nil: %+v", recent)
	}
	if recent.Summary["plugin_id"] != "1011" || recent.Summary["param_id"] != "volume" || recent.Summary["point_count"] != 24 {
		t.Fatalf("curve summary metadata = %+v", recent.Summary)
	}
	rows := messageLoopMapRows(recent.Summary["points"])
	if len(rows) != automationCurvePointSummaryRowLimit {
		t.Fatalf("curve points rows = %d, want bounded %d", len(rows), automationCurvePointSummaryRowLimit)
	}
	if rows[0]["t"] != 0.0 || rows[0]["v"] != 0.0 {
		t.Fatalf("first sampled point = %+v", rows[0])
	}
}

func TestRecentObservationForToolAutomationMutationStaysUndisclosed(t *testing.T) {
	call := planner.ToolCall{ID: "call_auto_1", Tool: "automation.set_points"}
	result := automationDisclosureResult("automation.set_points", "automation.set_points", map[string]any{
		"status": "ok", "plugin_id": "1011", "param_id": "volume", "written_points": 4,
	})
	recent := recentObservationForTool(call, result, planner.VerificationResult{}, false, nil)
	if recent.Summary != nil {
		t.Fatalf("mutation tool leaked into automation disclosure surface: %+v", recent.Summary)
	}
}

func TestRecentObservationForToolAutomationSurfaceAcceptsNameAliases(t *testing.T) {
	aliases := []struct{ tool, command string }{
		{"get_plugin_parameters", "get_plugin_parameters"},
		{"plugin_get_parameters", ""},
		{"automation_get_curve", ""},
	}
	for _, alias := range aliases {
		result := automationDisclosureResult(alias.tool, alias.command, map[string]any{
			"status": "ok", "plugin_id": "1011", "parameter_count": 2,
			"automation_param_surface": []any{
				map[string]any{"param_id": "volume", "label": "Volume", "min": -80.0, "max": 6.0},
			},
		})
		recent := recentObservationForTool(planner.ToolCall{ID: "call_auto_1", Tool: alias.tool}, result, planner.VerificationResult{}, false, nil)
		if recent.Summary == nil {
			t.Fatalf("alias %q collapsed to nil", alias.tool)
		}
		if rows := messageLoopMapRows(recent.Summary["automation_param_surface"]); len(rows) != 1 {
			t.Fatalf("alias %q surface rows = %+v", alias.tool, recent.Summary["automation_param_surface"])
		}
	}
}

// TestRecentObservationForToolKeepsNonAutomationToolsCollapsed pins the
// pre-existing behavior: tools outside the Mix/CCB/automation faces keep a
// nil summary (the new branch is add-only).
func TestRecentObservationForToolKeepsNonAutomationToolsCollapsed(t *testing.T) {
	call := planner.ToolCall{ID: "call_other", Tool: "track.list"}
	result := executorpkg.Result{
		Status: "ok", Tool: "track.list", ToolCallID: "call_other",
		Result: map[string]any{"status": "ok", "tracks": []any{map[string]any{"track_id": "1010"}}},
	}
	recent := recentObservationForTool(call, result, planner.VerificationResult{}, false, nil)
	if recent.Summary != nil {
		t.Fatalf("non-automation tool summary changed shape: %+v", recent.Summary)
	}
}

// TestRecentObservationForToolMixAndCCBPathsUntouched pins zero regression on
// the existing disclosure paths: the Mix observation name still routes through
// mixObservationPromptSummary and the CCB name still carries its bundle.
func TestRecentObservationForToolMixAndCCBPathsUntouched(t *testing.T) {
	mixResult := executorpkg.Result{
		Status: "ok", Tool: "mix.observe", ToolCallID: "call_mix",
		Result: map[string]any{
			"status":     "ok",
			"acoustic_digest": map[string]any{"kind": "mix_digest"},
			"observation": map[string]any{"schema_version": "mix_observation.v1"},
		},
	}
	mixRecent := recentObservationForTool(planner.ToolCall{ID: "call_mix", Tool: "mix.observe"}, mixResult, planner.VerificationResult{}, false, nil)
	if mixRecent.Summary == nil || mixRecent.Summary["acoustic_digest"] == nil {
		t.Fatalf("mix observation summary lost its digest: %+v", mixRecent.Summary)
	}

	bundle := map[string]any{"audit_receipt": map[string]any{"receipt_id": "rcpt_1"}, "views": []any{}}
	ccbResult := executorpkg.Result{
		Status: "ok", Tool: "ccb.observation_request", ToolCallID: "call_ccb",
		Result: map[string]any{"status": "ok", "bundle": bundle},
	}
	ccbRecent := recentObservationForTool(planner.ToolCall{ID: "call_ccb", Tool: "ccb.observation_request"}, ccbResult, planner.VerificationResult{}, false, nil)
	if ccbRecent.Summary == nil || ccbRecent.Summary["audit_receipt"] == nil {
		t.Fatalf("CCB bundle summary lost its receipt: %+v", ccbRecent.Summary)
	}
}

// TestRecentObservationAutomationParamSurfaceIsBounded pins the summary-side
// row cap independently from the harness supply-side cap.
func TestRecentObservationAutomationParamSurfaceIsBounded(t *testing.T) {
	rows := make([]any, 0, 40)
	for i := 0; i < 40; i++ {
		rows = append(rows, map[string]any{"param_id": fmt.Sprintf("p%02d", i), "min": 0.0, "max": 1.0})
	}
	result := automationDisclosureResult("plugin.get_parameters", "get_plugin_parameters", map[string]any{
		"status": "ok", "plugin_id": "1011", "parameter_count": 40,
		"automation_param_surface": rows,
	})
	recent := recentObservationForTool(planner.ToolCall{ID: "call_auto_1", Tool: "plugin.get_parameters"}, result, planner.VerificationResult{}, false, nil)
	if got := len(messageLoopMapRows(recent.Summary["automation_param_surface"])); got != automationParamSurfaceSummaryRowLimit {
		t.Fatalf("summary surface rows = %d, want %d", got, automationParamSurfaceSummaryRowLimit)
	}
}
