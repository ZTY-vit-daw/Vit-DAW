package agentloop

import (
	"strings"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

// M3-AUTOMATION-SURFACE-PIN-1 tests: the durable automation_surfaces ledger
// that pins successful plugin.get_parameters receipts (the 240 forensics'
// eviction defect: the single-slot recent_observation dropped the plugin
// identity and parameter ranges from the model's visible face), the
// attribution restore on the neutral model projection (the exact loss layer:
// the pre-family key filter deletes every "plugin" key), and the denial
// citation that reads the ledger the model actually sees.

func pinSurfaceLoopContext() map[string]any {
	return map[string]any{
		"schema_version":  "free_state_reasoning_loop.v1",
		"status":          "observing",
		"original_intent": "ride the volume up",
	}
}

func pinSurfaceObservation(pluginID, trackID string) *RecentObservation {
	return &RecentObservation{
		ToolCallID: "pin-1", Tool: "plugin.get_parameters", Status: "ok",
		Summary: map[string]any{
			"status":          "ok",
			"track_id":        trackID,
			"plugin_id":       pluginID,
			"parameter_count": 2,
			"automation_param_surface": []map[string]any{
				{"param_id": "volume", "label": "Volume", "min": 0, "max": 1},
				{"param_id": "pan", "label": "Pan", "min": -1, "max": 1},
			},
		},
	}
}

func pinSurfaceState() *runState {
	return &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": pinSurfaceLoopContext()}}}
}

// Pin 1: a successful read pins one row on the loop face and mirrors it on the
// observation_ledger face (the 240 dual-write pattern).
func TestFreeStateAutomationSurfacePinsSuccessfulRead(t *testing.T) {
	state := pinSurfaceState()
	recordFreeStateAutomationSurface(state, pinSurfaceObservation("1011", "1010"))

	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	rows := messageLoopMapRows(loop["automation_surfaces"])
	if len(rows) != 1 {
		t.Fatalf("pinned rows = %d, want 1: %#v", len(rows), loop)
	}
	row := rows[0]
	if firstMapText(row, "plugin_id") != "1011" || firstMapText(row, "track_id") != "1010" {
		t.Fatalf("row attribution wrong: %#v", row)
	}
	params := messageLoopMapRows(row["params"])
	if len(params) != 2 || firstMapText(params[0], "param_id") != "volume" || params[0]["min"] != 0 || params[0]["max"] != 1 {
		t.Fatalf("row params wrong: %#v", params)
	}
	ledger := messageLoopMapValue(loop["observation_ledger"])
	if len(messageLoopMapRows(ledger["automation_surfaces"])) != 1 {
		t.Fatalf("observation_ledger mirror missing: %#v", ledger)
	}
}

// Pin 2: the same track_id+plugin_id fingerprint refreshes the pinned row in
// place instead of adding a second row.
func TestFreeStateAutomationSurfaceDedupesFingerprint(t *testing.T) {
	state := pinSurfaceState()
	recordFreeStateAutomationSurface(state, pinSurfaceObservation("1011", "1010"))

	refreshed := pinSurfaceObservation("1011", "1010")
	refreshed.Summary["automation_param_surface"] = []map[string]any{
		{"param_id": "volume", "label": "Volume", "min": 0, "max": 2},
	}
	recordFreeStateAutomationSurface(state, refreshed)

	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	rows := messageLoopMapRows(loop["automation_surfaces"])
	if len(rows) != 1 {
		t.Fatalf("fingerprint dedupe failed, rows = %d: %#v", len(rows), rows)
	}
	if messageLoopMapRows(rows[0]["params"])[0]["max"] != 2 {
		t.Fatalf("row was not refreshed in place: %#v", rows[0])
	}
}

// Pin 3: a strictly newer project revision expires the pinned row (the 240
// freeStateRevisionStrictlyNewer semantics — the dual of the guard's revision
// re-open); an equal revision keeps it.
func TestFreeStateAutomationSurfaceExpiresOnRevisionAdvance(t *testing.T) {
	state := pinSurfaceState()
	state.input.Context["project_revision"] = "rev-5"
	recordFreeStateAutomationSurface(state, pinSurfaceObservation("1011", "1010"))

	if !freeStateAutomationParamSurfacePresent(state) {
		t.Fatal("pinned surface not present at the same revision")
	}

	state.input.Context["project_revision"] = "rev-5"
	if !freeStateAutomationParamSurfacePresent(state) {
		t.Fatal("equal revision must keep the pinned row")
	}

	state.input.Context["project_revision"] = "rev-7"
	if freeStateAutomationParamSurfacePresent(state) {
		t.Fatal("strictly newer revision must expire the pinned row")
	}
}

// Bound: more distinct fingerprints than the limit keep the most recent rows
// only (FIFO eviction, same contract as the observation ledger rows).
func TestFreeStateAutomationSurfaceBoundsRows(t *testing.T) {
	state := pinSurfaceState()
	for index := 0; index < freeStateAutomationSurfaceLedgerLimit+3; index++ {
		recordFreeStateAutomationSurface(state, pinSurfaceObservation(
			"plugin-"+strings.Repeat("x", index+1), "1010"))
	}
	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	rows := messageLoopMapRows(loop["automation_surfaces"])
	if len(rows) != freeStateAutomationSurfaceLedgerLimit {
		t.Fatalf("bounded rows = %d, want %d", len(rows), freeStateAutomationSurfaceLedgerLimit)
	}
	if firstMapText(rows[0], "plugin_id") != "plugin-"+strings.Repeat("x", 4) {
		t.Fatalf("oldest surviving row wrong: %#v", rows[0])
	}
}

// Attribution: the summary face carries the kernel-confirmed plugin identity
// (pure key carry), and the neutral pre-family projection no longer strips it
// from automation read receipts — the exact 240 loss layer — while a CCB
// receipt keeps its projected shape untouched.
func TestAutomationAttributionSurvivesNeutralProjection(t *testing.T) {
	result := map[string]any{
		"status": "ok", "track_id": "1010", "plugin_id": "1011", "plugin_name": "Volume & Pan Plugin", "parameter_count": 2,
		"automation_param_surface": []map[string]any{
			{"param_id": "volume", "label": "Volume", "min": 0, "max": 1},
		},
	}
	execResult := executorpkg.Result{ToolCallID: "t1", Tool: "plugin.get_parameters", Status: "ok", Result: result}
	call := planner.ToolCall{ID: "t1", Tool: "plugin.get_parameters"}
	observation := recentObservationForTool(call, execResult, planner.VerificationResult{}, false, nil)
	if observation.Summary["plugin_id"] != "1011" || observation.Summary["plugin_name"] != "Volume & Pan Plugin" {
		t.Fatalf("summary attribution missing: %#v", observation.Summary)
	}

	projected, ok := messageLoopPreFamilyValue(recentObservationMap(observation)).(map[string]any)
	if !ok {
		t.Fatalf("projection returned %T", projected)
	}
	summary := messageLoopMapValue(projected["summary"])
	if _, leaked := summary["plugin_id"]; leaked {
		t.Fatalf("preFamily projection no longer strips the plugin marker — the pinned loss-layer thesis is stale: %#v", summary)
	}
	// The 241 restore: the neutral-family snapshot calls exactly this hook, so
	// the automation read receipt keeps its kernel-confirmed attribution.
	restoreFreeStateAutomationAttribution(projected, observation)
	if messageLoopMapValue(projected["summary"])["plugin_id"] != "1011" ||
		messageLoopMapValue(projected["summary"])["plugin_name"] != "Volume & Pan Plugin" {
		t.Fatalf("attribution restore failed: %#v", projected["summary"])
	}

	// Zero-regression boundary: a CCB receipt keeps its projected shape (the
	// restore is scoped to automation read receipts only).
	ccbObservation := &RecentObservation{Tool: "ccb.observation_request", Status: "ok", Summary: map[string]any{
		"plugin_hint": "leak", "status": "ready",
	}}
	ccbProjected, ok := messageLoopPreFamilyValue(recentObservationMap(ccbObservation)).(map[string]any)
	if !ok {
		t.Fatalf("ccb projection returned %T", ccbProjected)
	}
	restoreFreeStateAutomationAttribution(ccbProjected, ccbObservation)
	if _, leaked := messageLoopMapValue(ccbProjected["summary"])["plugin_hint"]; leaked {
		t.Fatalf("restore leaked a non-attribution key on a CCB receipt: %#v", ccbProjected)
	}
}

// The denial citation reads the pinned ledger (= the model's visible face) and
// cites the concrete plugin identity and ranges; without a pinned row there is
// no in-hand claim. The base redirect note stays byte-stable.
func TestFreeStateUnavailableRetryIssueCitesPinnedLedger(t *testing.T) {
	state := pinSurfaceState()
	state.input.Context["project_revision"] = "rev-5"
	issue := freeStateUnavailableRetryIssue(state, []string{"processor.identity_and_controls"})
	if !strings.Contains(issue, "redirect: processor.* CCB projections do not cover built-in track-level plugins") {
		t.Fatalf("base redirect note damaged: %q", issue)
	}
	if strings.Contains(issue, "automation_surfaces in hand") {
		t.Fatalf("in-hand citation without a pinned surface: %q", issue)
	}

	recordFreeStateAutomationSurface(state, pinSurfaceObservation("1011", "1010"))
	issue = freeStateUnavailableRetryIssue(state, []string{"processor.identity_and_controls"})
	for _, needle := range []string{
		"automation_surfaces in hand:",
		"plugin_id=1011",
		"track_id=1010",
		"volume[0..1]",
		"pan[-1..1]",
		"use these pinned rows for automation.set_points",
	} {
		if !strings.Contains(issue, needle) {
			t.Fatalf("pinned citation missing %q: %q", needle, issue)
		}
	}
}

// M3-CURRENT-VALUE-DISCLOSE-1: the pinned ledger row carries the current value
// with the same lifetime as its range (the pinrun1 breakpoint: the model held
// the range but not the write baseline for its ramp). Present when the compact
// surface has it, absent when it does not, and refreshed in place by a
// post-mutation re-read so the pin never cites a stale baseline.
func TestFreeStateAutomationSurfaceLedgerRowCarriesCurrentValue(t *testing.T) {
	state := pinSurfaceState()
	observation := pinSurfaceObservation("1011", "1010")
	observation.Summary["automation_param_surface"] = []map[string]any{
		{"param_id": "volume", "label": "Volume", "min": 0, "max": 1, "value": 0.0, "value_text": "0.0"},
		{"param_id": "pan", "label": "Pan", "min": -1, "max": 1},
	}
	recordFreeStateAutomationSurface(state, observation)

	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	rows := messageLoopMapRows(loop["automation_surfaces"])
	if len(rows) != 1 {
		t.Fatalf("pinned rows = %d: %#v", len(rows), loop)
	}
	params := messageLoopMapRows(rows[0]["params"])
	if len(params) != 2 {
		t.Fatalf("params = %#v", params)
	}
	if params[0]["value"] != 0.0 || params[0]["value_text"] != "0.0" {
		t.Fatalf("pinned row missing the current value: %#v", params[0])
	}
	if _, ok := params[1]["value"]; ok {
		t.Fatalf("value fabricated on a value-less param row: %#v", params[1])
	}
	if _, ok := params[1]["value_text"]; ok {
		t.Fatalf("value_text fabricated on a value-less param row: %#v", params[1])
	}

	refreshed := pinSurfaceObservation("1011", "1010")
	refreshed.Summary["automation_param_surface"] = []map[string]any{
		{"param_id": "volume", "label": "Volume", "min": 0, "max": 1, "value": 0.42, "value_text": "0.42"},
	}
	recordFreeStateAutomationSurface(state, refreshed)
	rows = messageLoopMapRows(messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])["automation_surfaces"])
	if len(rows) != 1 {
		t.Fatalf("refresh appended instead of replacing in place: %#v", rows)
	}
	if refreshedParams := messageLoopMapRows(rows[0]["params"]); refreshedParams[0]["value"] != 0.42 {
		t.Fatalf("refreshed row keeps a stale value: %#v", rows[0])
	}
}

// The pinned rows ride the always-injected free-state ledger JSON so the pin
// survives the recent_observation eviction between decision rounds.
func TestFreeStateLedgerJSONCarriesAutomationSurfaces(t *testing.T) {
	state := pinSurfaceState()
	recordFreeStateAutomationSurface(state, pinSurfaceObservation("1011", "1010"))

	context := messageLoopFreeStatePromptContext(state)
	rows := messageLoopMapRows(context["automation_surfaces"])
	if len(rows) != 1 || firstMapText(rows[0], "plugin_id") != "1011" {
		t.Fatalf("ledger JSON missing pinned rows: %#v", context)
	}
}

// Non-pinning shapes stay out of the ledger: error reads, reads without a
// kernel-confirmed plugin identity, mutation tools, and curve reads (the
// card's honesty boundary — time-range facts are re-observable).
func TestFreeStateAutomationSurfaceRejectsNonPinningShapes(t *testing.T) {
	state := pinSurfaceState()

	failed := pinSurfaceObservation("1011", "1010")
	failed.Status = "error"
	recordFreeStateAutomationSurface(state, failed)

	unattributed := pinSurfaceObservation("", "1010")
	recordFreeStateAutomationSurface(state, unattributed)

	mutation := &RecentObservation{ToolCallID: "t3", Tool: "automation.set_points", Status: "ok", Summary: map[string]any{
		"plugin_id": "1011", "track_id": "1010", "point_count": 3,
	}}
	recordFreeStateAutomationSurface(state, mutation)

	curve := &RecentObservation{ToolCallID: "t4", Tool: "automation.get_curve", Status: "ok", Summary: map[string]any{
		"plugin_id": "1011", "track_id": "1010", "points": []map[string]any{{"time": 0.0}},
	}}
	recordFreeStateAutomationSurface(state, curve)

	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	if rows := messageLoopMapRows(loop["automation_surfaces"]); len(rows) != 0 {
		t.Fatalf("non-pinning shapes recorded: %#v", rows)
	}
}
