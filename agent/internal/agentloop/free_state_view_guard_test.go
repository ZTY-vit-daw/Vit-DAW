package agentloop

import (
	"strings"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

// M3-LOOP-UNAVAILABLE-VIEW-GUARD-1 tests: the mechanical guard that denies the
// second request of the exact same view set + target after it returned no
// usable view (the 237 residual shape: processor.* over built-in plugins), the
// closed-template redirect disclosure on those receipts, and the zero-
// regression boundary (usable/rejected/declared_contradiction semantics).

func viewGuardLoopContext() map[string]any {
	return map[string]any{
		"schema_version":  "free_state_reasoning_loop.v1",
		"status":          "observing",
		"original_intent": "inspect plugin parameters",
	}
}

func viewGuardUnavailableObservation() *RecentObservation {
	return &RecentObservation{
		ToolCallID: "ccb-unavailable", Tool: "ccb.observation_request", Status: "ok",
		Summary: map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "status": "insufficient", "observation_id": "obs-unavailable",
			"requested_views":  []any{"processor.identity_and_controls"},
			"omission_reasons": []any{"processor.identity_and_controls: source evidence is missing"},
			"target_ref":       map[string]any{"kind": "track", "id": "1010"},
			"audit_receipt":    map[string]any{"receipt_id": "ccbr-unavailable"},
		},
	}
}

func viewGuardTarget() map[string]any {
	return map[string]any{"kind": "track", "id": "1010"}
}

// Interception 1/3: the second request of the same exact view set + target is
// mechanically denied after the set returned unavailable evidence.
func TestFreeStateUnavailableViewSetDeniedOnSecondSameSetRequest(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": viewGuardLoopContext()}}}
	recordFreeStateCCBObservation(state, viewGuardUnavailableObservation())

	issue := messageLoopFreeStateRejectedViewSetIssue(state, []string{"processor.identity_and_controls"}, viewGuardTarget())
	if issue == "" {
		t.Fatal("same exact view set + target passed after an unavailable receipt")
	}
	if !strings.Contains(issue, "view_set_retry_denied") {
		t.Fatalf("denial is not machine-readable: %q", issue)
	}

	// The ledger carries the unavailable verdict row once, with the retry
	// policy and revision the guard needs.
	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	rows := messageLoopMapRows(loop["unavailable_observation_requests"])
	if len(rows) != 1 {
		t.Fatalf("unavailable loop rows = %d, want 1: %#v", len(rows), loop)
	}
	if firstMapText(rows[0], "retry_policy") != "do_not_retry" || firstMapText(rows[0], "status") != "unavailable" {
		t.Fatalf("unavailable row form wrong: %#v", rows[0])
	}
	ledger := messageLoopMapValue(loop["observation_ledger"])
	if len(messageLoopMapRows(ledger["unavailable_view_sets"])) != 1 {
		t.Fatalf("unavailable ledger rows wrong: %#v", ledger)
	}

	// Re-recording the same fingerprint does not duplicate rows.
	recordFreeStateCCBObservation(state, viewGuardUnavailableObservation())
	loop = messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	if len(messageLoopMapRows(loop["unavailable_observation_requests"])) != 1 {
		t.Fatalf("unavailable rows duplicated: %#v", loop)
	}
}

// Interception 2/3: a different view set, or the same views under a different
// target, passes (exact_view_set semantics: the fingerprint differs).
func TestFreeStateUnavailableViewSetGuardPassesDifferentSetOrTarget(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": viewGuardLoopContext()}}}
	recordFreeStateCCBObservation(state, viewGuardUnavailableObservation())

	if issue := messageLoopFreeStateRejectedViewSetIssue(state, []string{"track.time_dynamics"}, viewGuardTarget()); issue != "" {
		t.Fatalf("different view set was denied: %q", issue)
	}
	if issue := messageLoopFreeStateRejectedViewSetIssue(state, []string{"processor.identity_and_controls"}, map[string]any{"kind": "track", "id": "1011"}); issue != "" {
		t.Fatalf("different target was denied: %q", issue)
	}
	if issue := messageLoopFreeStateRejectedViewSetIssue(state, []string{"processor.identity_and_controls", "track.time_dynamics"}, viewGuardTarget()); issue != "" {
		t.Fatalf("superset view set was denied: %q", issue)
	}
}

// Interception 3/3: a strictly newer project revision re-opens the exact view
// set; an equal revision keeps the denial.
func TestFreeStateUnavailableViewSetGuardPassesOnRevisionAdvance(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": viewGuardLoopContext()}}}
	observation := viewGuardUnavailableObservation()
	observation.Summary["project_binding"] = map[string]any{"project_revision": "rev-7"}
	recordFreeStateCCBObservation(state, observation)

	if issue := messageLoopFreeStateRejectedViewSetIssue(state, []string{"processor.identity_and_controls"}, viewGuardTarget()); issue == "" {
		t.Fatal("same-revision retry passed before the revision advanced")
	}

	state.input.Context["project_revision"] = "rev-8"
	if issue := messageLoopFreeStateRejectedViewSetIssue(state, []string{"processor.identity_and_controls"}, viewGuardTarget()); issue != "" {
		t.Fatalf("revision advance was denied: %q", issue)
	}

	state.input.Context["project_revision"] = "rev-7"
	if issue := messageLoopFreeStateRejectedViewSetIssue(state, []string{"processor.identity_and_controls"}, viewGuardTarget()); issue == "" {
		t.Fatal("equal revision passed; only a strictly newer revision re-opens the set")
	}
}

// Denial receipt form: machine-readable tokens, the rule citation, and the
// no-observation-round statement the card requires; a processor.* denial also
// carries the redirect disclosure, with the in-hand marker when the loop
// already holds an automation parameter surface.
func TestFreeStateUnavailableRetryIssueIsMachineReadable(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": viewGuardLoopContext()}}}
	issue := freeStateUnavailableRetryIssue(state, []string{"mix.masking_relationship"})
	for _, needle := range []string{
		"view_set_retry_denied",
		"retry_policy=do_not_retry",
		`rule="If a CCB observation returns rejected, unavailable, or deferred evidence for a requested view set, do not retry the same view set"`,
		"observation_round_consumed=false",
	} {
		if !strings.Contains(issue, needle) {
			t.Fatalf("denial receipt missing %q: %q", needle, issue)
		}
	}
	if strings.Contains(issue, "processor.* CCB projections") {
		t.Fatalf("non-processor denial carries a redirect: %q", issue)
	}

	processorIssue := freeStateUnavailableRetryIssue(state, []string{"processor.identity_and_controls"})
	if !strings.Contains(processorIssue, "redirect: processor.* CCB projections do not cover built-in track-level plugins") ||
		!strings.Contains(processorIssue, "plugin.get_parameters") {
		t.Fatalf("processor denial missing redirect disclosure: %q", processorIssue)
	}
	if strings.Contains(processorIssue, "already present in this conversation") {
		t.Fatalf("in-hand marker without an automation surface: %q", processorIssue)
	}

	// M3-AUTOMATION-SURFACE-PIN-1: the in-hand judgment reads the pinned
	// automation_surfaces ledger (the model's visible face) instead of the
	// executed-tail scan; the citation names the pinned identity and ranges.
	state.input.Context["free_state_reasoning_loop"] = map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "observing",
		"automation_surfaces": []map[string]any{{
			"plugin_id": "1011", "track_id": "1010", "project_revision": "rev-5",
			"params": []map[string]any{{"param_id": "volume", "min": 0, "max": 1}},
		}},
	}
	state.input.Context["project_revision"] = "rev-5"
	withSurface := freeStateUnavailableRetryIssue(state, []string{"processor.identity_and_controls"})
	if !strings.Contains(withSurface, "automation_surfaces in hand: plugin_id=1011") ||
		!strings.Contains(withSurface, "volume[0..1]") {
		t.Fatalf("in-hand citation missing with a pinned surface: %q", withSurface)
	}
}

// The denial reaches the model decision surface through the shared guard (the
// same function the decision validation and the tool guard call).
func TestFreeStateUnavailableViewSetDeniedThroughDecisionOutput(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": viewGuardLoopContext()}}}
	recordFreeStateCCBObservation(state, viewGuardUnavailableObservation())
	state.recentObservation = &RecentObservation{Tool: "ccb.observation_request", Status: "ok", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-ready",
		"requested_views": []any{"track.time_dynamics"}, "views": map[string]any{"track.time_dynamics": map[string]any{"status": "ready"}},
	}}
	decision := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "retry", RequestedViewIDs: []string{"processor.identity_and_controls"},
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []any{"processor.identity_and_controls"}, "target_kind": "track", "target_id": "1010",
	}}}}
	issue := messageLoopFreeStateOutputIssue(state, decision)
	if !strings.Contains(issue, "view_set_retry_denied") {
		t.Fatalf("denial did not reach the decision surface: %q", issue)
	}
}

// Redirect disclosure: an unavailable processor.* receipt gains the closed-
// template mechanism note at the raw-bundle layer (recentObservationForTool),
// so every model-visible face of the receipt carries it; ledger rows keep the
// raw delivery reasons only.
func TestFreeStateUnavailableProcessorRedirectDisclosure(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": viewGuardLoopContext()}}}
	bundle := map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "insufficient", "observation_id": "obs-unavailable",
		"requested_views":  []any{"processor.identity_and_controls"},
		"omission_reasons": []any{"processor.identity_and_controls: source evidence is missing"},
		"target_ref":       viewGuardTarget(),
	}
	result := executorpkg.Result{ToolCallID: "ccb-unavailable", Tool: "ccb.observation_request", Status: "ok", Result: map[string]any{"bundle": bundle}}
	observation := recentObservationForTool(planner.ToolCall{Tool: "ccb.observation_request"}, result, planner.VerificationResult{}, false, nil)

	reasons := messageLoopStringList(observation.Summary["omission_reasons"])
	if len(reasons) != 2 {
		t.Fatalf("redirect note not appended: %#v", reasons)
	}
	if !strings.Contains(reasons[1], "processor.* CCB projections do not cover built-in track-level plugins") ||
		!strings.Contains(reasons[1], "plugin.get_parameters") {
		t.Fatalf("redirect note form wrong: %q", reasons[1])
	}
	if !strings.HasPrefix(reasons[1], "processor.* CCB projections") {
		t.Fatalf("note lead token unstable: %q", reasons[1])
	}
	// The raw bundle (trace / executed record / model projection faces hold
	// this same map) carries the note too.
	if bundleReasons := messageLoopStringList(bundle["omission_reasons"]); len(bundleReasons) != 2 {
		t.Fatalf("raw bundle note missing: %#v", bundleReasons)
	}
	// Idempotent: a second pass does not duplicate the note.
	discloseCCBProcessorRedirectInBundle(bundle)
	if bundleReasons := messageLoopStringList(bundle["omission_reasons"]); len(bundleReasons) != 2 {
		t.Fatalf("redirect note duplicated: %#v", bundleReasons)
	}

	// After the runner's record step, the ledger row keeps only the raw
	// delivery reason — never the disclosure note.
	recordFreeStateCCBObservation(state, observation)
	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	rows := messageLoopMapRows(loop["unavailable_observation_requests"])
	if len(rows) != 1 {
		t.Fatalf("unavailable rows = %d: %#v", len(rows), loop)
	}
	rowReasons := messageLoopStringList(rows[0]["reasons"])
	if len(rowReasons) != 1 || rowReasons[0] != "processor.identity_and_controls: source evidence is missing" {
		t.Fatalf("ledger row reasons polluted by disclosure: %#v", rowReasons)
	}
}

// Redirect disclosure negatives: the note only attaches to unavailable
// processor.* citations, never to usable receipts, non-processor omissions,
// or disclosure-budget trims.
func TestFreeStateUnavailableProcessorRedirectNegatives(t *testing.T) {
	usable := map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "ready",
		"requested_views": []any{"processor.identity_and_controls"},
		"views":          map[string]any{"processor.identity_and_controls": map[string]any{"status": "partial"}},
		"target_ref":     viewGuardTarget(),
	}
	discloseCCBProcessorRedirectInBundle(usable)
	if reasons := messageLoopStringList(usable["omission_reasons"]); len(reasons) != 0 {
		t.Fatalf("usable receipt gained a redirect note: %#v", reasons)
	}

	nonProcessor := map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "insufficient",
		"requested_views":  []any{"track.time_dynamics"},
		"omission_reasons": []any{"track.time_dynamics: source evidence is missing"},
		"target_ref":       viewGuardTarget(),
	}
	discloseCCBProcessorRedirectInBundle(nonProcessor)
	for _, reason := range messageLoopStringList(nonProcessor["omission_reasons"]) {
		if strings.Contains(reason, "processor.* CCB projections") {
			t.Fatalf("non-processor omission gained a redirect note: %#v", nonProcessor["omission_reasons"])
		}
	}

	budget := map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "partial",
		"requested_views":  []any{"processor.identity_and_controls", "track.time_dynamics"},
		"omission_reasons": []any{"processor.identity_and_controls: omitted by disclosure budget"},
		"omissions":        map[string]any{"processor.identity_and_controls": "budget"},
		"views":            map[string]any{"track.time_dynamics": map[string]any{"status": "ready"}},
		"target_ref":       viewGuardTarget(),
	}
	discloseCCBProcessorRedirectInBundle(budget)
	for _, reason := range messageLoopStringList(budget["omission_reasons"]) {
		if strings.Contains(reason, "processor.* CCB projections") {
			t.Fatalf("budget omission gained a redirect note: %#v", budget["omission_reasons"])
		}
	}

	// Guardrun2 forensics: the model's mixed sets (usable views plus the
	// processor view) are bundle-partial — the note must still fire on the
	// per-view unavailable citation.
	mixed := map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "partial",
		"requested_views":  []any{"project.structure", "processor.identity_and_controls"},
		"omission_reasons": []any{"project.structure: evidence status is partial", "processor.identity_and_controls: source evidence is missing"},
		"omissions":        map[string]any{"processor.identity_and_controls": "unavailable"},
		"views":            map[string]any{"project.structure": map[string]any{"status": "partial"}},
		"target_ref":       viewGuardTarget(),
	}
	discloseCCBProcessorRedirectInBundle(mixed)
	mixedReasons := messageLoopStringList(mixed["omission_reasons"])
	if len(mixedReasons) != 3 || !strings.HasPrefix(mixedReasons[2], "processor.* CCB projections") {
		t.Fatalf("mixed-set processor omission did not gain the redirect note: %#v", mixedReasons)
	}
}

// Zero regression: the existing rejected-view-set semantics stay byte
// identical, and the usable / declared_contradiction re-observation admission
// is untouched by the unavailable guard.
func TestFreeStateViewGuardZeroRegressionOnRejectedAndUsableSemantics(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": viewGuardLoopContext()}}}
	rejected := &RecentObservation{
		ToolCallID: "ccb-rejected", Tool: "ccb.observation_request", Status: "rejected",
		Summary: map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "status": "rejected", "observation_id": "obs-rejected",
			"requested_views":  []any{"mix.masking_relationship"},
			"omission_reasons": []any{"deferred"},
			"target_ref":       viewGuardTarget(),
			"audit_receipt":    map[string]any{"receipt_id": "ccbr-rejected"},
		},
	}
	recordFreeStateCCBObservation(state, rejected)
	const rejectedIssue = "the requested CCB view set was already rejected; choose a different catalog view set or return blocked instead of retrying the same rejected/deferred views"
	if issue := messageLoopFreeStateRejectedViewSetIssue(state, []string{"mix.masking_relationship"}, viewGuardTarget()); issue != rejectedIssue {
		t.Fatalf("rejected semantics changed: %q", issue)
	}

	// A usable receipt keeps its exact re-observation contract: repeat denied
	// by the usable dedup, admitted under declared_contradiction when the prior
	// evidence was partial (the admission contract requires stale/partial) —
	// and the unavailable guard never fires for it.
	state2 := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": viewGuardLoopContext()}}}
	usable := &RecentObservation{ToolCallID: "ccb-ready", Tool: "ccb.observation_request", Status: "ok", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "partial", "observation_id": "obs-ready",
		"requested_views": []string{"track.time_dynamics"}, "views": map[string]any{"track.time_dynamics": map[string]any{"status": "partial"}},
		"target_ref":     viewGuardTarget(),
	}}
	recordFreeStateCCBObservation(state2, usable)
	if issue := messageLoopFreeStateRejectedViewSetIssue(state2, []string{"track.time_dynamics"}, viewGuardTarget()); issue != "" {
		t.Fatalf("usable receipt hit the rejected/unavailable guard: %q", issue)
	}
	if issue := messageLoopFreeStateAlreadyObservedIssue(state2, []string{"track.time_dynamics"}, viewGuardTarget()); issue == "" {
		t.Fatal("usable dedup no longer denies an exact repeat")
	}
	contradiction := map[string]any{"declared_contradiction": true}
	if issue := messageLoopFreeStateAlreadyObservedIssue(state2, []string{"track.time_dynamics"}, viewGuardTarget(), contradiction); issue != "" {
		t.Fatalf("declared_contradiction admission broken: %q", issue)
	}
}
