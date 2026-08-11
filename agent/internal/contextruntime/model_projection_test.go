package contextruntime

import (
	"encoding/json"
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
