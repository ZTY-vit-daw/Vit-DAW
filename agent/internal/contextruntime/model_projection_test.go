package contextruntime

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/planner"
)

func TestProjectCCBToolResultRecognizesHarnessWrappers(t *testing.T) {
	opts := fixedOptions()
	bundleResult := testCCBBundleResult("ccb-call-1", "model-visible-fact", "raw-bundle-secret")
	projection, ok := ProjectCCBToolResult(bundleResult, opts)
	if !ok || projection["kind"] != "observation" || projection["observation_id"] != "obs-1" {
		t.Fatalf("bundle projection = %#v, ok=%v", projection, ok)
	}
	if projection["projection_status"] != "partial" {
		t.Fatalf("bundle projection status = %#v, want partial", projection["projection_status"])
	}
	if projection["requested_views"] == nil || projection["freshness"] == nil || projection["limitations"] == nil || projection["evidence_refs"] == nil || projection["audit_ref"] == nil {
		t.Fatalf("bundle projection lost required metadata: %#v", projection)
	}

	catalogResult := testCCBCatalogResult("ccb-catalog-1", 3)
	catalogProjection, ok := ProjectCCBToolResult(catalogResult, opts)
	if !ok || catalogProjection["kind"] != "catalog" || catalogProjection["catalog_version"] == nil || catalogProjection["catalog_index"] == nil {
		t.Fatalf("catalog projection = %#v, ok=%v", catalogProjection, ok)
	}

	if projection, ok := ProjectCCBToolResult(planner.ToolResult{
		ToolCallID: "normal-call", Tool: "workspace.read_file", Status: "ok",
		Result: map[string]any{"status": "ok", "content": "ordinary"},
	}, opts); ok || projection != nil {
		t.Fatalf("ordinary result was classified as CCB: %#v", projection)
	}

	// The bundle schema at the result root is not the real harness transport
	// shape and must not be mistaken for a wrapped CCB response.
	if projection, ok := ProjectCCBToolResult(planner.ToolResult{
		Tool: "ccb.observation_request", Result: map[string]any{"schema_version": ccbBundleSchema},
	}, opts); ok || projection != nil {
		t.Fatalf("unwrapped bundle was accepted: %#v", projection)
	}
}

func TestCCBProjectionUsesLLMContextAndNeverFallsBackToRawView(t *testing.T) {
	result := testCCBBundleResult("ccb-call-1", "model-visible-fact", "raw-bundle-secret")
	projection, ok := ProjectCCBToolResult(result, fixedOptions())
	if !ok {
		t.Fatal("fixture was not recognized as CCB")
	}
	data, _ := json.Marshal(projection)
	body := string(data)
	if !strings.Contains(body, "model-visible-fact") {
		t.Fatalf("llm_context was not selected: %s", body)
	}
	for _, forbidden := range []string{"raw-bundle-secret", "Secret Vendor", "plugin-secret", "parameter-secret", "compressor-family"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("projection leaked %q: %s", forbidden, body)
		}
	}
	views := projection["views"].(map[string]any)
	unknown := views["unknown.future_view"].(map[string]any)
	if unknown["projection_status"] != "unsupported_projection" || unknown["view_projection"] != nil || unknown["llm_context"] != nil {
		t.Fatalf("unknown view used an unsafe fallback: %#v", unknown)
	}
}

func TestCCBProjectionPreservesExactSetRejectionScope(t *testing.T) {
	result := planner.ToolResult{
		ToolCallID: "ccb-reject", Tool: "ccb.observation_request", Status: "rejected",
		Result: map[string]any{"bundle": map[string]any{
			"schema_version": ccbBundleSchema, "status": "rejected", "requested_views": []any{"mix.masking_relationship", "mix.frequency_relationship"},
			"rejection_scope": "exact_view_set", "blocking_view_ids": []any{"mix.masking_relationship"},
			"non_blocking_view_ids": []any{"mix.frequency_relationship"}, "views": map[string]any{},
			"audit_receipt": map[string]any{"schema_version": "ccb_observation_receipt.v1", "receipt_id": "receipt-reject", "view_set_matches": false,
				"rejection_scope": "exact_view_set", "blocking_view_ids": []any{"mix.masking_relationship"}, "non_blocking_view_ids": []any{"mix.frequency_relationship"}},
		}},
	}
	projection, ok := ProjectCCBToolResult(result, fixedOptions())
	if !ok || projection["rejection_scope"] != "exact_view_set" {
		t.Fatalf("rejection projection = %#v", projection)
	}
	if len(modelStrings(projection["blocking_view_ids"])) != 1 || len(modelStrings(projection["non_blocking_view_ids"])) != 1 {
		t.Fatalf("rejection view metadata lost: %#v", projection)
	}
	audit := modelMap(projection["audit_ref"])
	if audit["rejection_scope"] != "exact_view_set" {
		t.Fatalf("audit rejection scope lost: %#v", audit)
	}
}

func TestProjectModelSnapshotKeepsOneCanonicalCCBProjection(t *testing.T) {
	ccb := testCCBBundleResult("ccb-call-1", "single-model-fact", "raw-audit-payload")
	ordinary := planner.ToolResult{
		ToolCallID: "normal-call", Tool: "workspace.read_file", Status: "ok",
		Result: map[string]any{"status": "ok", "content": "ordinary-tool-result"},
	}
	trace := []planner.TraceEvent{
		{Kind: "tool_result", ToolResult: &ccb},
		{Kind: "tool_result", ToolResult: &ccb},
		{Kind: "tool_result", ToolResult: &ordinary},
	}
	recentProjection, _ := ProjectCCBToolResult(ccb, fixedOptions())
	recent := map[string]any{
		"tool_call_id": ccb.ToolCallID,
		"tool":         ccb.Tool,
		"status":       ccb.Status,
		"summary":      recentProjection,
	}
	full := Build(Input{GoalTrace: trace, RecentObservation: recent}, fixedOptions())
	if len(full.ToolResultSummary) != 3 {
		t.Fatalf("internal snapshot lost audit summaries: %#v", full.ToolResultSummary)
	}
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot:          full,
		GoalTrace:         trace,
		RecentObservation: recent,
	}, fixedOptions())
	data, _ := json.Marshal(model)
	body := string(data)
	if count := strings.Count(body, `"schema_version":"`+CCBModelProjectionSchema+`"`); count != 1 {
		t.Fatalf("canonical projection count = %d, want 1: %s", count, body)
	}
	if count := strings.Count(body, "single-model-fact"); count != 1 {
		t.Fatalf("model fact count = %d, want 1: %s", count, body)
	}
	if strings.Contains(body, "raw-audit-payload") {
		t.Fatalf("raw bundle entered model snapshot: %s", body)
	}
	if !strings.Contains(body, "ordinary-tool-result") {
		t.Fatalf("ordinary tool result was removed: %s", body)
	}

	rows := modelRows(modelMap(model["goal_trace_summary"])["recent_events"])
	if len(rows) != 3 || rows[0]["tool_result"] != nil || rows[0]["tool_result_ref"] == nil || rows[1]["tool_result_ref"] == nil {
		t.Fatalf("CCB trace event was not replaced by a reference: %#v", rows)
	}
	if rows[2]["tool_result"] == nil {
		t.Fatalf("ordinary trace result was unexpectedly replaced: %#v", rows[2])
	}
	semantic := modelMap(model["daw_semantic_summary"])
	if semantic["recent_execution_result"] == nil {
		// The ordinary result is latest and must retain the existing behavior.
		t.Fatalf("ordinary recent execution result was removed: %#v", semantic)
	}
	recentContext := modelMap(model["recent_goal_context"])
	if recentContext["recent_observation"] != nil || recentContext["recent_observation_ref"] == nil {
		t.Fatalf("recent CCB observation was duplicated: %#v", recentContext)
	}

	// Projection is read-only with respect to the audit trace.
	bundle := modelMap(ccb.Result["bundle"])
	if !strings.Contains(string(mustJSON(t, bundle)), "raw-audit-payload") || modelMap(bundle["audit_receipt"])["receipt_id"] != "receipt-1" {
		t.Fatalf("complete audit bundle was mutated: %#v", bundle)
	}
}

func TestCatalogBecomesShortReferenceAfterObservation(t *testing.T) {
	catalog := testCCBCatalogResult("catalog-call", 4)
	observation := testCCBBundleResult("observation-call", "active-observation-fact", "hidden-audit")
	trace := []planner.TraceEvent{
		{Kind: "tool_result", ToolResult: &catalog},
		{Kind: "tool_result", ToolResult: &observation},
	}
	full := Build(Input{GoalTrace: trace}, fixedOptions())
	model := ProjectModelSnapshot(ModelProjectionInput{Snapshot: full, GoalTrace: trace}, fixedOptions())
	active := modelMap(model["active_observation"])
	if active["kind"] != "observation" || active["observation_id"] != "obs-1" {
		t.Fatalf("active observation = %#v", active)
	}
	ref := modelMap(model["observation_catalog_ref"])
	if ref["catalog_version"] == nil || len(modelStrings(ref["view_ids"])) != 4 || ref["catalog_index"] != nil {
		t.Fatalf("catalog history was not reduced to a short reference: %#v", ref)
	}
	body := ModelJSON(model)
	if strings.Contains(body, "What bounded evidence is available?") {
		t.Fatalf("historical catalog content was replayed: %s", body)
	}
}

func TestModelProjectionResumeDoesNotRehydrateCCBPayloadCopies(t *testing.T) {
	ccb := testCCBBundleResult("ccb-call-resume", "resume-model-fact", "resume-raw-audit")
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}
	recentProjection, _ := ProjectCCBToolResult(ccb, fixedOptions())
	recent := map[string]any{
		"tool_call_id": ccb.ToolCallID, "tool": ccb.Tool, "status": ccb.Status, "summary": recentProjection,
	}
	first := Build(Input{GoalTrace: trace, RecentObservation: recent}, fixedOptions())
	second := Build(Input{
		GoalTrace:         trace,
		RecentObservation: recent,
		PreviousSnapshot:  first.Map(),
	}, fixedOptions())
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: second, GoalTrace: trace, RecentObservation: recent,
	}, fixedOptions())
	body := ModelJSON(model)
	if count := strings.Count(body, `"schema_version":"`+CCBModelProjectionSchema+`"`); count != 1 {
		t.Fatalf("resume projection count = %d, want 1: %s", count, body)
	}
	if strings.Count(body, "resume-model-fact") != 1 || strings.Contains(body, "resume-raw-audit") {
		t.Fatalf("resume rehydrated repeated/raw payload: %s", body)
	}
}

func TestModelProjectionSectionsStayBounded(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result planner.ToolResult
		max    int
	}{
		{name: "catalog", result: testCCBCatalogResult("catalog-size", 17), max: 12 * 1024},
		{name: "observation", result: testCCBBundleResult("observation-size", strings.Repeat("bounded fact ", 80), strings.Repeat("raw secret ", 4000)), max: 10 * 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &tc.result}}
			full := Build(Input{GoalTrace: trace}, fixedOptions())
			model := ProjectModelSnapshot(ModelProjectionInput{Snapshot: full, GoalTrace: trace}, fixedOptions())
			body := ModelJSON(model)
			if len(body) > tc.max {
				t.Fatalf("model snapshot = %d bytes, max %d; sections=%v", len(body), tc.max, ModelSectionBytes(model))
			}
			activeBytes := ModelSectionBytes(model)["active_observation"]
			if activeBytes == 0 || activeBytes >= len(body) {
				t.Fatalf("invalid active observation size %d/%d; sections=%v", activeBytes, len(body), ModelSectionBytes(model))
			}
		})
	}
}

func TestModelProjectionProfilesRetainHotAndReportColdDegradation(t *testing.T) {
	ccb := testCCBBundleResult("profile-call", "profile-fact", "profile-raw")
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}
	full := Build(Input{GoalTrace: trace, RecentObservation: map[string]any{
		"tool_call_id": ccb.ToolCallID, "tool": ccb.Tool, "status": ccb.Status,
		"summary": map[string]any{"schema_version": CCBModelProjectionSchema},
	}}, fixedOptions())
	model := ProjectModelSnapshot(ModelProjectionInput{Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileSelection}, fixedOptions())
	if model["active_observation"] == nil || model["context_profile"] != string(ModelContextProfileSelection) {
		t.Fatalf("selection profile lost hot context: %#v", model)
	}
	degradation := modelMap(model["context_degradation"])
	if degradation["cold_data"] != "audit_snapshot_only" {
		t.Fatalf("missing cold-data boundary: %#v", degradation)
	}
	if strings.Contains(ModelJSON(model), "profile-raw") {
		t.Fatal("cold raw bundle entered the model profile")
	}
}

func TestModelProjectionReportsHotBudgetOverageWithoutTruncation(t *testing.T) {
	modelFact := strings.Repeat("effective bounded fact ", 80)
	ccb := testCCBBundleResult("profile-overage", modelFact, "overage-raw")
	bundle := modelMap(ccb.Result["bundle"])
	view := modelMap(modelMap(bundle["views"])["track.time_dynamics"])
	llmContext := modelMap(modelMap(modelMap(view["facts"])["observation.com_projection"])["llm_context"])
	facts := make([]any, 0, 80)
	for i := 0; i < 80; i++ {
		facts = append(facts, map[string]any{
			"fact": fmt.Sprintf("effective fact %02d %s", i, strings.Repeat("bounded evidence ", 12)),
		})
	}
	llmContext["compact_facts"] = facts
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}
	full := Build(Input{GoalTrace: trace}, fixedOptions())
	opts := fixedOptions()
	opts.MaxTextRunes = 512
	opts.MaxListItems = 100
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileSelection,
	}, opts)
	active := modelMap(model["active_observation"])
	degradation := modelMap(active["degradation"])
	if degradation["status"] != "over_hot_budget" || modelText(degradation["target_bytes"]) != "10240" {
		t.Fatalf("hot overage was not reported: active=%#v", active)
	}
	profileDegradation := modelMap(model["context_degradation"])
	if !strings.Contains(modelText(profileDegradation["budget_status"]), "over_hot") || modelText(profileDegradation["budget_action"]) == "" {
		t.Fatalf("profile hot overage was not reported: %#v", profileDegradation)
	}
	body := ModelJSON(model)
	if !strings.Contains(body, "effective fact 79") || strings.Contains(body, "overage-raw") {
		t.Fatalf("hot overage was truncated or leaked raw data: %s", body)
	}
	size := modelMap(model["context_size"])
	if size["total_bytes"] == nil || modelMap(size["section_bytes"])["active_observation"] == nil {
		t.Fatalf("section size report missing: %#v", size)
	}
	if modelText(size["hot_bytes"]) != modelText(profileDegradation["hot_bytes"]) || size["warm_bytes"] == nil {
		t.Fatalf("layer byte report mismatch: size=%#v degradation=%#v", size, profileDegradation)
	}
}

func TestModelProjectionProfilesUseDeterministicStageSections(t *testing.T) {
	ccb := testCCBBundleResult("profile-stage-call", "stage-fact", "stage-raw")
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}
	full := Build(Input{
		GoalSummary: "stage profile",
		GoalTrace:   trace,
		Context:     map[string]any{"stage": "selection"},
	}, fixedOptions())

	selection := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileSelection,
	}, fixedOptions())
	if selection["active_observation"] == nil || selection["daw_semantic_summary"] != nil {
		t.Fatalf("selection profile carried non-selection semantic section: %#v", selection)
	}
	if modelMap(selection["context_layers"])["cold"] == nil {
		t.Fatalf("selection profile lost cold boundary: %#v", selection["context_layers"])
	}
	selectionLayers := modelMap(selection["context_layers"])
	selectionLayersJSON := string(mustJSON(t, selectionLayers))
	if strings.Contains(selectionLayersJSON, "daw_semantic_summary") {
		t.Fatalf("selection layers advertised omitted semantic section: %#v", selectionLayers)
	}

	materialize := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileMaterialize,
	}, fixedOptions())
	if materialize["active_observation"] == nil || materialize["daw_semantic_summary"] == nil {
		t.Fatalf("materialization profile lost execution semantic section: %#v", materialize)
	}
	materializeLayers := modelMap(materialize["context_layers"])
	if !strings.Contains(string(mustJSON(t, modelMap(materializeLayers["hot"])["sections"])), "daw_semantic_summary") {
		t.Fatalf("materialization hot layer did not advertise execution semantics: %#v", materializeLayers)
	}

	postAction := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, Profile: ModelContextProfilePostAction,
	}, fixedOptions())
	if postAction["active_observation"] == nil || postAction["recent_goal_context"] == nil {
		t.Fatalf("post-action profile lost fresh evidence/verification context: %#v", postAction)
	}
	postActionLayers := modelMap(postAction["context_layers"])
	if !strings.Contains(string(mustJSON(t, modelMap(postActionLayers["hot"])["sections"])), "recent_goal_context") {
		t.Fatalf("post-action hot layer did not advertise verification context: %#v", postActionLayers)
	}
	if strings.Contains(string(mustJSON(t, modelMap(postActionLayers["warm"])["sections"])), "recent_goal_context") {
		t.Fatalf("post-action section was assigned to both hot and warm: %#v", postActionLayers)
	}
	for _, profile := range []map[string]any{selection, materialize, postAction} {
		body := ModelJSON(profile)
		if strings.Contains(body, "stage-raw") || strings.Count(body, "stage-fact") != 1 {
			t.Fatalf("profile duplicated or leaked CCB payload: %s", body)
		}
	}
}

func TestObservationLedgerProjectionIsCompactSafeAndSeparateFromActiveObservation(t *testing.T) {
	ccb := testCCBBundleResult("ledger-active-call", "ledger-active-fact", "ledger-raw-audit")
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}
	full := Build(Input{GoalTrace: trace}, fixedOptions())
	ledger := map[string]any{
		"schema_version": "free_state_observation_ledger.v1",
		"available_views": map[string]any{
			"track.time_dynamics": map[string]any{
				"view_id": "track.time_dynamics", "status": "ready", "observation_id": "obs-1", "tool_call_id": "ledger-active-call",
				"freshness": map[string]any{"status": "fresh"}, "limitations": []any{"macro only"}, "evidence_refs": []any{"evidence://one"},
				"audit_ref": map[string]any{"receipt_id": "receipt-1"},
				"views":     map[string]any{"raw": "must-not-enter"}, "plugin_id": "plugin-secret", "target_family": "compressor-family",
			},
		},
		"rejected_view_sets": []any{map[string]any{
			"fingerprint": "ccb_views:1234", "requested_views": []any{"mix.masking_relationship"}, "status": "rejected",
			"reasons": []any{"deferred"}, "retry_policy": "do_not_retry", "sealed_truth": "must-not-enter",
		}},
		"receipts": []any{map[string]any{
			"receipt_id": "receipt-1", "tool_call_id": "ledger-active-call", "observation_id": "obs-1", "status": "ready",
			"requested_views": []any{"track.time_dynamics"}, "raw_bundle": "must-not-enter", "expected_coverage": "must-not-enter",
		}},
	}
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, ObservationLedger: ledger, Profile: ModelContextProfileSelection,
	}, fixedOptions())
	body := ModelJSON(model)
	if strings.Count(body, `"schema_version":"`+CCBModelProjectionSchema+`"`) != 1 || strings.Count(body, "ledger-active-fact") != 1 {
		t.Fatalf("canonical active projection was duplicated: %s", body)
	}
	for _, forbidden := range []string{"must-not-enter", "plugin-secret", "compressor-family", "ledger-raw-audit"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("ledger projection leaked %q: %s", forbidden, body)
		}
	}
	projected := modelMap(model["observation_ledger"])
	if projectedJSON := string(mustJSON(t, projected)); strings.Contains(projectedJSON, `"views"`) {
		t.Fatalf("ledger retained raw views: %s", projectedJSON)
	}
	if len(modelRows(projected["rejected_view_sets"])) != 1 || len(modelRows(projected["receipts"])) != 1 {
		t.Fatalf("ledger references missing: %#v", projected)
	}
	if modelRows(projected["rejected_view_sets"])[0]["reasons"] != nil {
		t.Fatalf("free-text rejection reasons entered model ledger: %#v", projected)
	}
	sections := ModelSectionBytes(model)
	if sections["observation_ledger"] == 0 || sections["active_observation"] == 0 {
		t.Fatalf("ledger/active section sizes missing: %#v", sections)
	}
	layers := modelMap(model["context_layers"])
	if !strings.Contains(string(mustJSON(t, modelMap(layers["warm"])["sections"])), "observation_ledger") {
		t.Fatalf("ledger was not assigned to warm context: %#v", layers)
	}
}

func TestObservationLedgerKeepsReadyEvidenceAlongsideScopedRejection(t *testing.T) {
	projected := ProjectObservationLedger(map[string]any{
		"schema_version": "free_state_observation_ledger.v1",
		"available_views": map[string]any{
			"mix.multitrack_relationship": map[string]any{
				"view_id": "mix.multitrack_relationship", "status": "ready", "observation_id": "obs-ready", "tool_call_id": "call-ready",
				"freshness": map[string]any{"status": "fresh"}, "evidence_refs": []any{"evidence://ready"},
			},
			"mix.frequency_relationship": map[string]any{
				"view_id": "mix.frequency_relationship", "status": "ready", "observation_id": "obs-ready", "tool_call_id": "call-ready",
			},
		},
		"rejected_view_sets": []any{map[string]any{
			"fingerprint": "ccb_views:mix", "requested_views": []any{"mix.frequency_relationship", "mix.masking_relationship"}, "status": "rejected", "retry_policy": "do_not_retry",
			"rejection_scope": "exact_view_set", "blocking_view_ids": []any{"mix.masking_relationship"}, "non_blocking_view_ids": []any{"mix.frequency_relationship"},
		}},
	}, fixedOptions())
	available := modelMap(projected["available_views"])
	if modelMap(available["mix.multitrack_relationship"])["observation_id"] != "obs-ready" || modelMap(available["mix.frequency_relationship"])["status"] != "ready" {
		t.Fatalf("ready evidence was lost: %#v", projected)
	}
	rejected := modelRows(projected["rejected_view_sets"])
	if len(rejected) != 1 || modelText(rejected[0]["rejection_scope"]) != "exact_view_set" {
		t.Fatalf("scoped rejection was not projected: %#v", projected)
	}
	if len(modelStrings(rejected[0]["blocking_view_ids"])) != 1 || len(modelStrings(rejected[0]["non_blocking_view_ids"])) != 1 {
		t.Fatalf("rejection view classes were not projected: %#v", rejected[0])
	}
}

func TestObservationLedgerHistoryIsBoundedWithoutSilentProjectionTruncation(t *testing.T) {
	rows := make([]any, 0, 40)
	for i := 0; i < 40; i++ {
		rows = append(rows, map[string]any{
			"receipt_id": fmt.Sprintf("receipt-%02d", i), "tool_call_id": fmt.Sprintf("call-%02d", i),
			"observation_id": fmt.Sprintf("obs-%02d", i), "status": "ready", "requested_views": []any{"track.time_dynamics"},
		})
	}
	projected := ProjectObservationLedger(map[string]any{
		"schema_version": "free_state_observation_ledger.v1", "receipts": rows,
	}, fixedOptions())
	receipts := modelRows(projected["receipts"])
	if len(receipts) != 24 || modelText(receipts[0]["receipt_id"]) != "receipt-16" || modelText(receipts[23]["receipt_id"]) != "receipt-39" {
		t.Fatalf("bounded receipt history = %#v", receipts)
	}
	window := modelMap(modelMap(projected["history_window"])["receipts"])
	if modelText(window["total"]) != "40" || modelText(window["retained"]) != "24" || modelText(window["omitted"]) != "16" {
		t.Fatalf("bounded receipt history was silently truncated: %#v", window)
	}
}

func testCCBBundleResult(callID, modelFact, rawSecret string) planner.ToolResult {
	return planner.ToolResult{
		ToolCallID: callID,
		Tool:       "ccb.observation_request",
		Status:     "ok",
		Result: map[string]any{
			"status": "ready",
			"bundle": map[string]any{
				"schema_version":     ccbBundleSchema,
				"bundle_id":          "bundle-1",
				"request_id":         "request-1",
				"status":             "ready",
				"read_only":          true,
				"mutation_authority": false,
				"observation_id":     "obs-1",
				"target_ref":         map[string]any{"kind": "track", "id": "track-1"},
				"freshness":          map[string]any{"class": "current_observation", "status": "ready"},
				"requested_views":    []any{"track.time_dynamics", "unknown.future_view"},
				"evidence_refs":      []any{"evidence://one"},
				"limitations":        []any{"bounded evidence"},
				"views": map[string]any{
					"track.time_dynamics": map[string]any{
						"view_id": "track.time_dynamics", "status": "ready", "limitations": []any{"macro only"},
						"facts": map[string]any{
							"observation.com_projection": map[string]any{
								"llm_context": map[string]any{
									"summary_md":                 modelFact,
									"compact_facts":              []any{map[string]any{"fact": "level motion", "plugin_id": "plugin-secret"}},
									"do_not_include_raw_package": true,
								},
								"raw_package":   rawSecret,
								"vendor":        "Secret Vendor",
								"parameter_id":  "parameter-secret",
								"target_family": "compressor-family",
							},
						},
					},
					"unknown.future_view": map[string]any{
						"view_id": "unknown.future_view", "status": "ready",
						"facts": map[string]any{"opaque_transport": rawSecret},
					},
				},
				"audit_receipt": map[string]any{
					"schema_version": "ccb_observation_receipt.v1", "receipt_id": "receipt-1", "view_set_matches": true,
				},
			},
		},
	}
}

func testCCBCatalogResult(callID string, viewCount int) planner.ToolResult {
	views := make([]any, 0, viewCount)
	for i := 0; i < viewCount; i++ {
		views = append(views, map[string]any{
			"view_id":                "view." + strings.Repeat("x", i%3) + string(rune('a'+i)),
			"questions":              []any{"What bounded evidence is available?", "Which relationship is observable?"},
			"supported_target_kinds": []any{"track", "project"},
			"temporal_resolution":    "bounded summary",
			"availability":           "conditional",
			"cost_latency_class":     "medium",
			"quality_ceiling":        "semantic facts only",
			"limitations":            []any{"No raw evidence is disclosed."},
			"required_dependencies":  []any{"bounded projection"},
			"plugin_identifier":      "must-not-leak",
		})
	}
	return planner.ToolResult{
		ToolCallID: callID, Tool: "ccb.observation_catalog", Status: "ok",
		Result: map[string]any{"status": "ok", "catalog": map[string]any{
			"schema_version": ccbCatalogSchema,
			"boundary":       "semantic_views_only",
			"target_ref":     map[string]any{"kind": "project", "id": "project-1"},
			"views":          views,
			"exclusions":     []any{"raw PCM", "sealed truth"},
		}},
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
