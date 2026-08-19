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

func TestCCBHistoricalDecisionDigestUsesExtremaAndOmitsAuditPayload(t *testing.T) {
	bundle := map[string]any{
		"schema_version": ccbBundleSchema,
		"views": map[string]any{
			"track.timbre_frequency": map[string]any{"status": "ready", "facts": map[string]any{
				"track.1007.slow.band_energy.summary": map[string]any{
					"status": "ready",
					"bands": map[string]any{
						"sub":      map[string]any{"energy_db": -24.0, "unit_energy": 0.02, "sample_count": 999},
						"bass":     map[string]any{"energy_db": -10.0, "unit_energy": 0.60, "sample_count": 999},
						"low_mid":  map[string]any{"energy_db": -13.0, "unit_energy": 0.30, "sample_count": 999},
						"mid":      map[string]any{"energy_db": -18.0, "unit_energy": 0.06, "sample_count": 999},
						"presence": map[string]any{"energy_db": -21.0, "unit_energy": 0.015, "sample_count": 999},
						"air":      map[string]any{"energy_db": -31.0, "unit_energy": 0.005, "sample_count": 999},
					},
					"evidence_refs": []any{`dad.l3:D:\private\source.wav`},
					"limitations":   []any{"raw repeated limitation"},
					"scope":         map[string]any{"source_path": `D:\private\source.wav`},
				},
			}},
			"track.band_dynamics": map[string]any{"status": "partial", "facts": map[string]any{
				"observation.dom_projection": map[string]any{
					"status": "partial",
					"band_dynamics": map[string]any{
						"status": "partial", "time_varying_ready": false, "per_band_crest_ready": false,
						"cross_band_relation_ready": false,
						"bands":                     []any{map[string]any{"id": "bass", "energy_db": -10.0}},
						"evidence_refs":             []any{`dad.l3:D:\private\source.wav`},
					},
				},
			}},
		},
	}
	digest := ProjectCCBViewConclusion(bundle, "track.timbre_frequency", fixedOptions())
	body := ModelJSON(digest)
	for _, required := range []string{"decision_digest", "strongest", "second_strongest", "weakest", "spread_db", "bass", "air"} {
		if !strings.Contains(body, required) {
			t.Fatalf("historical digest lost %q: %s", required, body)
		}
	}
	for _, forbidden := range []string{"source.wav", "sample_count", "evidence_refs", "limitations", "scope", "presence"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("historical digest retained %q: %s", forbidden, body)
		}
	}
	dynamics := ProjectCCBViewConclusion(bundle, "track.band_dynamics", fixedOptions())
	dynamicsBody := ModelJSON(dynamics)
	if strings.Contains(dynamicsBody, "band_profile") || !strings.Contains(dynamicsBody, "time_varying_ready") {
		t.Fatalf("co-requested band dynamics repeated the timbre distribution or lost readiness: %s", dynamicsBody)
	}

	active, ok := ProjectCCBToolResult(planner.ToolResult{
		ToolCallID: "ccb-active", Tool: "ccb.observation_request", Status: "ok", Result: map[string]any{"bundle": bundle},
	}, fixedOptions())
	if !ok {
		t.Fatal("active wrapper was not recognized")
	}
	activeBody := ModelJSON(active)
	if !strings.Contains(activeBody, "presence") || !strings.Contains(activeBody, "mid") || !strings.Contains(activeBody, "source.wav") {
		t.Fatalf("canonical active projection lost safe facts or required evidence refs: %s", activeBody)
	}
}

func TestCCBDOMEquivalentProjectionKeepsNeutralNumericEvidence(t *testing.T) {
	result := planner.ToolResult{
		ToolCallID: "ccb-dom", Tool: "ccb.observation_request", Status: "ok",
		Result: map[string]any{"bundle": map[string]any{
			"schema_version": ccbBundleSchema, "status": "ready", "observation_id": "obs-dom",
			"requested_views": []any{"track.peak_structure", "track.activity_structure", "track.frequency_time_events", "track.transient_structure", "track.band_dynamics"},
			"views": map[string]any{
				"track.peak_structure": map[string]any{"status": "ready", "facts": map[string]any{"observation.dom_projection": map[string]any{
					"status": "ready", "peak_structure": map[string]any{"status": "ready", "peak_dbfs": -2.1, "headroom_db": 2.1, "crest_db": 14.2, "at_or_above_full_scale_segment_count": 0, "true_peak_status": "missing"},
				}}},
				"track.activity_structure": map[string]any{"status": "ready", "facts": map[string]any{"observation.dom_projection": map[string]any{
					"status": "ready", "activity_structure": map[string]any{"status": "ready", "active_ratio": 0.72, "low_or_silent_ratio": 0.28, "noise_floor_status": "ready", "valid_segment_count": 20},
				}}},
				"track.frequency_time_events": map[string]any{"status": "ready", "facts": map[string]any{"observation.dom_projection": map[string]any{
					"status": "ready", "frequency_time_events": map[string]any{"status": "ready", "time_localized": true, "event_count_available": true, "whole_window_bands": []any{map[string]any{"id": "high", "energy_db": -18.0}}},
				}}},
				"track.transient_structure": map[string]any{"status": "ready", "facts": map[string]any{"observation.dom_projection": map[string]any{
					"status": "ready", "transient_structure": map[string]any{"status": "ready", "onset_events_ready": true, "attack_body_contrast_ready": true, "sustain_decay_ready": true, "segment_crest_db_distribution": map[string]any{"count": 20, "p50": 12.0}},
				}}},
				"track.band_dynamics": map[string]any{"status": "ready", "facts": map[string]any{"observation.dom_projection": map[string]any{
					"status": "ready", "band_dynamics": map[string]any{"status": "ready", "time_varying_ready": true, "per_band_crest_ready": true, "cross_band_relation_ready": true, "bands": []any{map[string]any{"id": "bass", "energy_db": -20.0}}},
				}}},
			},
		}},
	}
	projection, ok := ProjectCCBToolResult(result, fixedOptions())
	if !ok {
		t.Fatal("DOM bundle was not projected")
	}
	body := ModelJSON(projection)
	for _, required := range []string{"peak_dbfs", "headroom_db", "active_ratio", "noise_floor_status", "time_localized", "attack_body_contrast_ready", "time_varying_ready", "energy_db"} {
		if !strings.Contains(body, required) {
			t.Fatalf("neutral DOM field %q missing: %s", required, body)
		}
	}
	if strings.Count(body, `"schema_version":"`+CCBModelProjectionSchema+`"`) != 1 {
		t.Fatalf("DOM projection was duplicated: %s", body)
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
	if len(rows) != 3 || rows[0]["tool_result"] != nil || rows[0]["tool_result_ref"] == nil || rows[1]["tool_result_ref"] == nil || rows[2]["tool_result_ref"] == nil || rows[2]["tool_result"] != nil {
		t.Fatalf("every trace event must carry only a tool result reference: %#v", rows)
	}
	// The canonical non-CCB copy of a tool result appears exactly once in
	// tool_result_summary; every other section references it.
	summary := modelRows(model["tool_result_summary"])
	if len(summary) != 1 || modelText(summary[0]["tool_call_id"]) != "normal-call" {
		t.Fatalf("non-CCB canonical copy was lost: %#v", summary)
	}
	if count := strings.Count(body, "ordinary-tool-result"); count != 1 {
		t.Fatalf("ordinary tool result must appear exactly once, got %d: %s", count, body)
	}
	semantic := modelMap(model["daw_semantic_summary"])
	if semantic["recent_execution_result"] != nil || semantic["recent_execution_result_ref"] == nil {
		t.Fatalf("recent execution result was not reduced to a reference: %#v", semantic)
	}
	recentContext := modelMap(model["recent_goal_context"])
	if recentContext["recent_observation"] != nil || recentContext["recent_observation_ref"] == nil {
		t.Fatalf("recent CCB observation was duplicated: %#v", recentContext)
	}
	if recentContext["recent_tool_result"] != nil || recentContext["recent_tool_result_ref"] == nil {
		t.Fatalf("recent tool result was duplicated: %#v", recentContext)
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

func TestModelProjectionHotDigestDropsCompactFactsAndRevisions(t *testing.T) {
	modelFact := strings.Repeat("effective bounded fact ", 80)
	ccb := testCCBBundleResult("profile-overage", modelFact, "overage-raw")
	bundle := modelMap(ccb.Result["bundle"])
	view := modelMap(modelMap(bundle["views"])["track.time_dynamics"])
	llmContext := modelMap(modelMap(modelMap(view["facts"])["observation.com_projection"])["llm_context"])
	llmContext["compact_facts"] = []any{
		map[string]any{"fact": "level motion", "measurement_key": "measurement-key-1"},
		map[string]any{"fact": "peak motion", "source_revision": "source-rev-1", "clip_revision": "clip-rev-1"},
	}
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}
	full := Build(Input{GoalTrace: trace}, fixedOptions())
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileSelection,
	}, fixedOptions())
	body := ModelJSON(model)
	// The Hot active observation keeps only the one-line digest and evidence
	// refs; compact facts, measurement keys, and revisions never enter it.
	for _, forbidden := range []string{"measurement-key-1", "source-rev-1", "clip-rev-1", "level motion"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("hot digest inlined %q: %s", forbidden, body)
		}
	}
	active := modelMap(model["active_observation"])
	digestView := modelMap(modelMap(active["views"])["track.time_dynamics"])
	if modelText(digestView["digest"]) == "" || digestView["llm_context"] != nil || digestView["view_projection"] != nil {
		t.Fatalf("hot view digest contract broken: %#v", digestView)
	}
	if len(modelStrings(digestView["evidence_refs"])) == 0 {
		t.Fatalf("hot view digest lost evidence refs: %#v", digestView)
	}
	// The full conclusion remains reachable through the warm ledger path.
	conclusion := ProjectCCBViewConclusion(bundle, "track.time_dynamics", fixedOptions())
	if modelText(conclusion["digest_kind"]) != "decision_digest" {
		t.Fatalf("warm conclusion digest lost: %#v", conclusion)
	}
}

func TestModelProjectionDegradesNonBlockingSectionsWithCompactionMarker(t *testing.T) {
	ccb := testCCBBundleResult("degrade-call", "degrade-fact", "degrade-raw")
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}
	full := Build(Input{GoalTrace: trace}, fixedOptions())
	// A supporting hot section (daw_state_summary) large enough to push the
	// hot layer over budget must be deterministically degraded to a cold ref.
	full.DAWStateSummary = map[string]any{"state": strings.Repeat("wide daw state payload ", 4000)}
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileSelection,
	}, fixedOptions())
	degradation := modelMap(model["context_degradation"])
	if modelText(degradation["budget_status"]) != "degraded_within_budget" {
		t.Fatalf("expected deterministic degradation, got %#v", degradation)
	}
	markers := modelRows(model["compaction_markers"])
	if len(markers) == 0 {
		t.Fatalf("degradation did not write CompactionMarker: %#v", model)
	}
	for _, marker := range markers {
		if modelText(marker["action"]) != "degraded_to_ref" || modelText(marker["cold_ref"]) == "" {
			t.Fatalf("compaction marker contract broken: %#v", marker)
		}
	}
	// Degraded section is a resolvable ref, never silently dropped.
	state := modelMap(model["daw_state_summary"])
	if modelText(state["ref"]) == "" || modelText(state["cold_data"]) != "audit_snapshot_only" {
		t.Fatalf("degraded section lost its cold ref: %#v", state)
	}
	if overflow := ModelContextOverflow(ModelJSON(model)); overflow != "" {
		t.Fatalf("degraded projection must not report overflow: %s", overflow)
	}
	size := modelMap(model["context_size"])
	if modelInteger(size["hot_bytes"]) > ModelHotBudgetBytes {
		t.Fatalf("degraded hot layer still over budget: %#v", size)
	}
}

func TestModelProjectionFailsClosedWithContextOverflow(t *testing.T) {
	ccb := testCCBBundleResult("overflow-call", "overflow-fact", "overflow-raw")
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}
	full := Build(Input{GoalTrace: trace}, fixedOptions())
	// A blocking hot section (current_selection) too large to degrade must
	// fail closed instead of being silently truncated.
	full.CurrentSelection = map[string]any{"payload": strings.Repeat("current selection cannot be degraded ", 4000)}
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileSelection,
	}, fixedOptions())
	degradation := modelMap(model["context_degradation"])
	if modelText(degradation["budget_status"]) != "context_overflow" {
		t.Fatalf("expected fail-closed context_overflow, got %#v", degradation)
	}
	overflow := modelMap(model["context_overflow"])
	if modelText(overflow["status"]) != "overflow" || modelText(overflow["cold_ref"]) == "" {
		t.Fatalf("context_overflow marker contract broken: %#v", overflow)
	}
	if text := ModelContextOverflow(ModelJSON(model)); text == "" {
		t.Fatal("ModelContextOverflow did not surface the overflow marker")
	}
	// Facts were never truncated: the full content stays in the audit snapshot.
	bundle := modelMap(ccb.Result["bundle"])
	if !strings.Contains(string(mustJSON(t, bundle)), "overflow-raw") {
		t.Fatal("audit facts were mutated by the overflow projection")
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

func TestModelProjectionCarriesBoundedProjectChangeInHotAcrossPostActionRefresh(t *testing.T) {
	change := map[string]any{
		"schema_version":            "project_change_receipt.v1",
		"change_id":                 "shadow_change_000022",
		"source":                    "authoritative_snapshot",
		"observed_at":               "2026-08-16T00:00:00Z",
		"authoritative":             true,
		"freshness":                 "current_snapshot",
		"from_state_epoch":          5,
		"to_state_epoch":            6,
		"affected_scopes":           []any{"track.level", "track.stereo_space", "project.state"},
		"unresolved_refresh_scopes": []any{"processor.track-1007"},
		"changed_entities": []any{
			map[string]any{"kind": "track", "id": "1007", "fields": []any{
				map[string]any{"path": "gain_db", "before": -2.0, "after": -3.0},
			}},
		},
	}
	previous := map[string]any{
		"recent_goal_context": map[string]any{
			"execution_memory":   map[string]any{"active_work_target_track_id": "1007", "raw": strings.Repeat("stale-history ", 2000)},
			"recent_observation": map[string]any{"observation_id": "obs-before", "status": "ready"},
		},
	}
	full := Build(Input{ProjectChange: change, PreviousSnapshot: previous}, fixedOptions())
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, Profile: ModelContextProfilePostAction,
	}, fixedOptions())
	projectChange := modelMap(model["project_change"])
	if modelText(projectChange["change_id"]) != "shadow_change_000022" || modelText(projectChange["freshness"]) != "current_snapshot" {
		t.Fatalf("project change missing from model hot: %#v", projectChange)
	}
	if len(modelRows(projectChange["changed_entities"])) != 1 || len(modelStrings(projectChange["affected_scopes"])) != 3 {
		t.Fatalf("project change lost bounded delta fields: %#v", projectChange)
	}
	if strings.Contains(ModelJSON(model), "stale-history") || strings.Contains(ModelJSON(model), `"raw"`) {
		t.Fatalf("previous unknown execution-memory fields were rehydrated into model context")
	}
	size := modelMap(model["context_size"])
	if modelInteger(size["hot_bytes"]) > ModelHotBudgetBytes || modelInteger(size["warm_bytes"]) > ModelWarmBudgetBytes {
		t.Fatalf("post-action context exceeded layered budget: %#v", size)
	}
	if overflow := ModelContextOverflow(ModelJSON(model)); overflow != "" {
		t.Fatalf("post-action context overflowed: %s", overflow)
	}
}

func TestProjectChangeFingerprintsStayOutOfHotContext(t *testing.T) {
	// Shadow collection fingerprints are stable JSON representations of the
	// source collection. A real clip collection can therefore carry paths and
	// manifest detail much larger than ordinary context text caps.
	fingerprint := strings.Repeat(`{"source_path":"D:/private/stems/lead-vocal.wav","clip_revision":"very-long-revision"},`, 180)
	entities := make([]any, 0, 8)
	for i := 0; i < 8; i++ {
		fields := make([]any, 0, 8)
		for _, path := range []string{"clips", "plugin_chain", "track_state_revision", "gain_db", "pan", "mute", "solo", "armed"} {
			before, after := any("before"), any("after")
			switch path {
			case "clips", "plugin_chain":
				before = map[string]any{"count": i + 1, "fingerprint": fingerprint}
				after = map[string]any{"count": i + 1, "fingerprint": fingerprint + "changed"}
			case "track_state_revision":
				before = fingerprint
				after = fingerprint + "changed"
			}
			fields = append(fields, map[string]any{"path": path, "before": before, "after": after})
		}
		entities = append(entities, map[string]any{"kind": "track", "id": fmt.Sprintf("track-%d", i), "fields": fields})
	}
	change := map[string]any{
		"schema_version":   "project_change_receipt.v1",
		"change_id":        "shadow_change_large_fingerprint",
		"source":           "authoritative_snapshot",
		"authoritative":    true,
		"freshness":        "current_snapshot",
		"affected_scopes":  []any{"project.structure", "mix.multitrack_relationship", "mix.masking_relationship"},
		"changed_entities": entities,
	}

	// Match the runtime's production text cap; project-change compaction must
	// have its own hard bound rather than inheriting that general-purpose cap.
	opts := fixedOptions()
	opts.MaxTextRunes = 900
	full := Build(Input{ProjectChange: change}, opts)
	model := ProjectModelSnapshot(ModelProjectionInput{Snapshot: full, Profile: ModelContextProfilePostAction}, opts)
	body := ModelJSON(model)
	projectChange := modelMap(model["project_change"])
	rows := modelRows(projectChange["changed_entities"])
	if len(rows) != 8 || len(modelRows(rows[0]["fields"])) != 8 {
		t.Fatalf("bounded entity/field set lost: %#v", projectChange)
	}
	clipField := modelRows(rows[0]["fields"])[0]
	if summary := modelMap(clipField["after"]); summary["kind"] != "collection_digest" || summary["fingerprint"] != nil || modelInteger(summary["count"]) != 1 {
		t.Fatalf("collection fingerprint was not summarized: %#v", clipField)
	}
	if strings.Contains(body, "private/stems") || strings.Contains(body, "very-long-revision") {
		t.Fatalf("raw collection fingerprint entered hot model context")
	}
	if hot := modelInteger(modelMap(model["context_size"])["hot_bytes"]); hot > ModelHotBudgetBytes {
		t.Fatalf("large project change hot = %d > %d; sections=%v", hot, ModelHotBudgetBytes, ModelSectionBytes(model))
	}
	if overflow := ModelContextOverflow(body); overflow != "" {
		t.Fatalf("large project change overflowed: %s", overflow)
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

func TestObservationLedgerRetainsDecisionConclusionForCanonicalActiveObservation(t *testing.T) {
	ccb := testCCBBundleResult("active-call", "active-fact", "raw-secret")
	full := Build(Input{GoalTrace: []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}}, fixedOptions())
	ledger := map[string]any{"schema_version": "free_state_observation_ledger.v1", "available_views": map[string]any{
		"track.time_dynamics": map[string]any{
			"view_id": "track.time_dynamics", "status": "ready", "observation_id": "obs-1", "tool_call_id": "active-call",
			"conclusion": map[string]any{"projection_status": "ready", "llm_context": map[string]any{"summary": "active-fact"}},
		},
		"track:2::track.time_dynamics": map[string]any{
			"view_id": "track.time_dynamics", "status": "ready", "observation_id": "obs-old", "tool_call_id": "old-call",
			"target_ref": map[string]any{"kind": "track", "id": "2"},
			"conclusion": map[string]any{"projection_status": "ready", "llm_context": map[string]any{"summary": "old-fact"}},
		},
	}}
	model := ProjectModelSnapshot(ModelProjectionInput{Snapshot: full, GoalTrace: []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}, ObservationLedger: ledger, Profile: ModelContextProfileSelection}, fixedOptions())
	body := ModelJSON(model)
	if strings.Count(body, "active-fact") != 2 || strings.Count(body, "old-fact") != 1 {
		t.Fatalf("active decision conclusion or historical conclusion lost: %s", body)
	}
	available := modelMap(modelMap(model["observation_ledger"])["available_views"])
	if modelMap(available["track.time_dynamics"])["conclusion"] == nil || modelMap(available["track:2::track.time_dynamics"])["conclusion"] == nil {
		t.Fatalf("active/historical conclusion projection = %#v", available)
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

func TestObservationLedgerKeepsViewAndRejectionMemoryAcrossRounds(t *testing.T) {
	// The model's decision loop depends on remembering what it already observed
	// and what it was told not to retry. Old-round available_views and
	// do_not_retry rejections must stay visible; only receipts stay delta-only.
	ledger := map[string]any{
		"schema_version":         "free_state_observation_ledger.v1",
		"window_round":           5,
		"view_observation_count": 3,
		"available_views": map[string]any{
			"mix.frequency_relationship": map[string]any{
				"view_id": "mix.frequency_relationship", "status": "ready", "observation_id": "obs-fr", "tool_call_id": "call-fr",
				"round":      2,
				"conclusion": map[string]any{"projection_status": "ready", "llm_context": map[string]any{"summary": "fr-fact"}},
			},
			"track:1007::track.timbre_frequency": map[string]any{
				"view_id": "track.timbre_frequency", "status": "ready", "observation_id": "obs-tf", "tool_call_id": "call-tf",
				"target_ref": map[string]any{"kind": "track", "id": "1007"},
				"round":      4,
				"conclusion": map[string]any{"projection_status": "ready", "llm_context": map[string]any{"summary": "tf-fact"}},
			},
		},
		"rejected_view_sets": []any{map[string]any{
			"fingerprint": "ccb_views:mask", "requested_views": []any{"mix.masking_relationship"}, "status": "rejected",
			"retry_policy": "do_not_retry", "rejection_scope": "exact_view_set", "blocking_view_ids": []any{"mix.masking_relationship"},
			"round": 1,
		}},
		"rejected_view_set_count": 1,
		"receipts": []any{
			map[string]any{"receipt_id": "receipt-1", "tool_call_id": "call-fr", "observation_id": "obs-fr", "status": "ready", "round": 2},
			map[string]any{"receipt_id": "receipt-5", "tool_call_id": "call-tf", "observation_id": "obs-tf", "status": "ready", "round": 5},
		},
		"receipt_count": 2,
	}
	projected := ProjectObservationLedger(ledger, fixedOptions())
	available := modelMap(projected["available_views"])
	if modelMap(available["mix.frequency_relationship"])["observation_id"] != "obs-fr" ||
		modelMap(available["track:1007::track.timbre_frequency"])["observation_id"] != "obs-tf" {
		t.Fatalf("cross-round available_views lost: %#v", available)
	}
	if modelMap(modelMap(available["mix.frequency_relationship"])["conclusion"]) == nil {
		t.Fatalf("old-round conclusion lost: %#v", available)
	}
	rejected := modelRows(projected["rejected_view_sets"])
	if len(rejected) != 1 || modelText(rejected[0]["fingerprint"]) != "ccb_views:mask" {
		t.Fatalf("cross-round do_not_retry rejection lost: %#v", rejected)
	}
	// Receipts stay delta-only: only the latest round row is projected.
	receipts := modelRows(projected["receipts"])
	if len(receipts) != 1 || modelText(receipts[0]["receipt_id"]) != "receipt-5" {
		t.Fatalf("receipts did not stay delta-only: %#v", receipts)
	}
	window := modelMap(modelMap(projected["history_window"])["available_views"])
	if modelText(window["total"]) != "3" || modelText(window["retained"]) != "2" || modelText(window["omitted"]) != "1" {
		t.Fatalf("available_views history window metadata = %#v", window)
	}
}

func TestObservationLedgerConclusionCarriesStructuredDecisionFacts(t *testing.T) {
	// Regression for run 5: the ledger conclusion digest must extract decision
	// facts (conflict candidates, coverage) from the flattened view facts, not
	// from the llm_context summary that only carries compact_facts. Before the
	// fix the digest was empty and the model concluded "no conflicts" despite
	// four real candidates, then declared the project satisfied.
	bundle := map[string]any{
		"schema_version": ccbBundleSchema, "status": "ready", "observation_id": "obs-fr",
		"requested_views": []any{"mix.frequency_relationship"},
		"views": map[string]any{"mix.frequency_relationship": map[string]any{
			"view_id": "mix.frequency_relationship", "status": "ready",
			"facts": map[string]any{
				"observation.mom_projection": map[string]any{
					"intent": "project_frequency_relationship_observation",
					"llm_context": map[string]any{
						"summary_md":                 "frequency relationship summary only",
						"compact_facts":              []any{map[string]any{"layer": "frequency_relationship", "status": "ready"}},
						"do_not_include_raw_package": true,
					},
					"frequency_relationship": map[string]any{
						"status": "ready", "tap_point": "source_file_pre_fx",
						"coverage": map[string]any{"conflict_candidate_count": 4, "eligible_track_count": 6, "eligible_track_ratio": 1, "frequency_region_count": 6, "missing_track_count": 0},
						"conflict_candidates": []any{
							map[string]any{"type": "band_overlap_candidate", "status": "candidate", "region": "sub", "min_hz": 20, "max_hz": 60, "confidence": "low_to_medium", "basis": "relative_whole_scope_band_energy", "interpretation_limit": "not_a_psychoacoustic_masking_fact", "tracks": []any{map[string]any{"track_id": "1007", "name": "bass", "energy_db": 56.4}}},
							map[string]any{"type": "band_overlap_candidate", "status": "candidate", "region": "bass", "min_hz": 60, "max_hz": 250, "confidence": "low_to_medium", "basis": "relative_whole_scope_band_energy", "interpretation_limit": "not_a_psychoacoustic_masking_fact", "tracks": []any{map[string]any{"track_id": "1012", "name": "drums", "energy_db": 65.7}}},
						},
						"tonal_tendencies": []any{map[string]any{"track_id": "1007", "status": "ready", "strongest_region": "bass", "relative_spread_db": 3.2}},
						"limitations":      []any{"energy_overlap_is_candidate_only_not_psychoacoustic_masking_fact"},
					},
				},
				"project.frequency_relationship_inputs": map[string]any{"status": "ready", "track_count": 6, "usable_track_count": 6, "tap_points": []any{"source_file_pre_fx"}},
			},
		}},
	}
	conclusion := ProjectCCBViewConclusion(bundle, "mix.frequency_relationship", fixedOptions())
	if conclusion["digest_status"] == "reference_only" {
		t.Fatalf("structured digest degraded to reference_only: %#v", conclusion)
	}
	facts := modelMap(conclusion["facts"])
	candidates := modelRows(facts["conflict_candidates"])
	if len(candidates) != 2 || modelText(candidates[0]["region"]) != "sub" || modelText(candidates[1]["region"]) != "bass" {
		t.Fatalf("conflict candidates lost in digest: %#v", facts)
	}
	if modelInteger(modelMap(facts["coverage"])["conflict_candidate_count"]) != 4 {
		t.Fatalf("coverage lost in digest: %#v", facts)
	}
	if modelText(facts["tap_point"]) != "source_file_pre_fx" {
		t.Fatalf("tap_point lost in digest: %#v", facts)
	}
	body := string(mustJSON(t, conclusion))
	for _, forbidden := range []string{"compact_facts", "summary only", "do_not_include_raw_package"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("llm_context summary leaked into decision digest: %s", body)
		}
	}
}

func TestMaskingProjectionKeepsDirectionalCandidatesInHotAndWarmLayers(t *testing.T) {
	bundle := map[string]any{
		"schema_version": ccbBundleSchema, "status": "ready", "observation_id": "obs-mask",
		"requested_views": []any{"mix.masking_relationship"},
		"views": map[string]any{"mix.masking_relationship": map[string]any{
			"view_id": "mix.masking_relationship", "status": "ready",
			"facts": map[string]any{
				"observation.mom_projection": map[string]any{
					"intent": "project_masking_relationship_observation",
					"llm_context": map[string]any{
						"summary_md":                 "masking candidates summary only",
						"compact_facts":              []any{map[string]any{"layer": "masking_relationship", "status": "ready"}},
						"do_not_include_raw_package": true,
					},
					"masking_relationship": map[string]any{
						"schema_version": "mom.masking_relationship.v1", "status": "ready", "freshness": "fresh",
						"measurement_id": "mask-1", "model_version": "vit_relative_energetic_masking_risk.v1", "candidate_only": true,
						"conditions": map[string]any{"tap_point": "track_post_fader", "range_start_seconds": 0.0, "range_end_seconds": 20.0, "tail_seconds": 0.0, "sample_rate": 48000.0, "analyzer_revision": "masking-frames-v1", "synchronized": true, "frame_count_per_track": 235},
						"coverage":   map[string]any{"track_count": 6, "eligible_track_count": 6, "candidate_count": 30, "projected_candidate_count": 30, "complete_coverage": true},
						"candidates": []any{map[string]any{
							"masker_track_id": "1017", "masker_track_name": "Bass", "target_track_id": "1007", "target_track_name": "Lead Vocal", "band_id": "low_mid",
							"min_hz": 250.0, "max_hz": 500.0, "active_frame_count": 180, "risk_frame_count": 90, "risk_coverage_ratio": 0.5,
							"median_margin_db": 1.4, "p90_margin_db": 4.8, "max_margin_db": 7.1,
						}},
						"evidence_refs": []any{"dad.masking_measurement:mask-1", "evidence://probe-1017", "evidence://probe-1007"},
						"limitations":   []any{"directional improvement risk only", "not a deterministic perceptual fact"},
						"frames":        []any{map[string]any{"levels_dbfs": map[string]any{"low_mid": -12.0}}},
					},
				},
			},
		}},
	}
	result := planner.ToolResult{ToolCallID: "ccb-mask", Tool: "ccb.observation_request", Status: "ok", Result: map[string]any{"bundle": bundle}}
	projection, ok := ProjectCCBToolResult(result, fixedOptions())
	if !ok {
		t.Fatal("masking bundle was not recognized")
	}
	view := modelMap(modelMap(projection["views"])["mix.masking_relationship"])
	hot := modelMap(modelMap(view["view_projection"])["masking_relationship"])
	candidates := modelRows(hot["candidates"])
	if len(candidates) != 1 || modelText(candidates[0]["masker_track_id"]) != "1017" || modelText(candidates[0]["target_track_id"]) != "1007" || modelText(candidates[0]["band_id"]) != "low_mid" {
		t.Fatalf("hot masking candidate was compressed away: %#v", view)
	}
	if hot["candidate_only"] != true || modelText(modelMap(hot["conditions"])["tap_point"]) != "track_post_fader" || len(modelStrings(hot["evidence_refs"])) == 0 {
		t.Fatalf("hot masking contract incomplete: %#v", hot)
	}
	body := string(mustJSON(t, projection))
	for _, forbidden := range []string{"\"frames\"", "\"levels_dbfs\"", "compact_facts", "masking candidates summary only"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("masking hot projection leaked %q: %s", forbidden, body)
		}
	}

	conclusion := ProjectCCBViewConclusion(bundle, "mix.masking_relationship", fixedOptions())
	if conclusion["digest_status"] == "reference_only" {
		t.Fatalf("masking warm conclusion degraded to reference_only: %#v", conclusion)
	}
	facts := modelMap(conclusion["facts"])
	warmCandidates := modelRows(facts["candidates"])
	if len(warmCandidates) != 1 || modelText(warmCandidates[0]["band_id"]) != "low_mid" || modelText(facts["status"]) != "ready" {
		t.Fatalf("masking warm candidate facts lost: %#v", facts)
	}
}

func TestLongMaskingEvidenceRefsBecomeBoundedHotHandles(t *testing.T) {
	longRef := "dad.l2_render_probe:source=D:/private/stems/lead-vocal.wav|manifest=" + strings.Repeat("clip-state-and-source-identity|", 80)
	refs := []any{longRef, longRef + "a", longRef + "b", longRef + "c", longRef + "d", longRef + "e", longRef + "f", longRef + "g"}
	bundle := map[string]any{
		"schema_version": ccbBundleSchema, "status": "ready", "observation_id": "obs-mask-long",
		"requested_views": []any{"mix.masking_relationship"}, "evidence_refs": refs,
		"views": map[string]any{"mix.masking_relationship": map[string]any{
			"view_id": "mix.masking_relationship", "status": "ready",
			"facts": map[string]any{"observation.mom_projection": map[string]any{
				"masking_relationship": map[string]any{
					"schema_version": "mom.masking_relationship.v1", "status": "ready", "freshness": "fresh", "measurement_id": "mask-long", "candidate_only": true,
					"conditions":    map[string]any{"tap_point": "track_post_fader", "synchronized": true},
					"coverage":      map[string]any{"track_count": 6, "candidate_count": 24, "candidates_truncated": true},
					"candidates":    []any{map[string]any{"masker_track_id": "1017", "target_track_id": "1007", "band_id": "low_mid", "median_margin_db": 4.2}},
					"evidence_refs": refs,
				},
			}},
		}},
	}
	result := planner.ToolResult{ToolCallID: "ccb-mask-long", Tool: "ccb.observation_request", Status: "ok", Result: map[string]any{"bundle": bundle}}
	opts := fixedOptions()
	opts.MaxTextRunes = 900
	full := Build(Input{GoalTrace: []planner.TraceEvent{{Kind: "tool_result", ToolResult: &result}}}, opts)
	model := ProjectModelSnapshot(ModelProjectionInput{Snapshot: full, GoalTrace: []planner.TraceEvent{{Kind: "tool_result", ToolResult: &result}}, Profile: ModelContextProfileSelection}, opts)
	body := ModelJSON(model)
	if strings.Contains(body, "private/stems") || strings.Contains(body, "clip-state-and-source-identity") {
		t.Fatalf("long source evidence leaked into hot context: %s", body)
	}
	active := modelMap(model["active_observation"])
	view := modelMap(modelMap(active["views"])["mix.masking_relationship"])
	relation := modelMap(modelMap(view["view_projection"])["masking_relationship"])
	modelRefs := modelStrings(relation["evidence_refs"])
	if len(modelRefs) == 0 || !strings.HasPrefix(modelRefs[0], "evidence_ref_hash:") {
		t.Fatalf("long masking reference was not converted to a handle: %#v", relation)
	}
	if hot := modelInteger(modelMap(model["context_size"])["hot_bytes"]); hot > ModelHotBudgetBytes {
		t.Fatalf("long masking evidence hot = %d > %d; sections=%v", hot, ModelHotBudgetBytes, ModelSectionBytes(model))
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

func TestModelProjectionMultiRoundBudgetStability(t *testing.T) {
	// Simulates the replay profile from the v1 draft: consecutive rounds each
	// add one CCB observation and one ledger receipt. The model-visible
	// projection must stay within hot<=10KB, warm<=5KB, full request<=24KB
	// (snapshot + fixed static prompt) and remain size-stable across rounds.
	const staticPromptBytes = 6 * 1024
	opts := fixedOptions()
	prevTotal := 0
	for round := 1; round <= 12; round++ {
		trace := make([]planner.TraceEvent, 0, round)
		for i := 1; i <= round; i++ {
			ccb := testCCBBundleResult(fmt.Sprintf("ccb-%02d", i), fmt.Sprintf("fact-%02d", i), "raw-audit")
			trace = append(trace, planner.TraceEvent{Kind: "tool_result", ToolResult: &ccb})
		}
		receipts := make([]any, 0, round)
		for i := 1; i <= round; i++ {
			receipts = append(receipts, map[string]any{
				"receipt_id": fmt.Sprintf("receipt-%02d", i), "tool_call_id": fmt.Sprintf("ccb-%02d", i),
				"observation_id": fmt.Sprintf("obs-%02d", i), "status": "ready", "round": i,
			})
		}
		ledger := map[string]any{
			"schema_version": "free_state_observation_ledger.v1",
			"window_round":   round, "receipt_count": round, "receipts": receipts,
		}
		full := Build(Input{GoalTrace: trace}, opts)
		model := ProjectModelSnapshot(ModelProjectionInput{
			Snapshot: full, GoalTrace: trace, ObservationLedger: ledger, Profile: ModelContextProfileSelection,
		}, opts)
		body := ModelJSON(model)
		size := modelMap(model["context_size"])
		hot := modelInteger(size["hot_bytes"])
		warm := modelInteger(size["warm_bytes"])
		if hot > ModelHotBudgetBytes || warm > ModelWarmBudgetBytes {
			t.Fatalf("round %d over layered budget: hot=%d/%d warm=%d/%d", round, hot, ModelHotBudgetBytes, warm, ModelWarmBudgetBytes)
		}
		if fullRequest := len(body) + staticPromptBytes; fullRequest > ModelFullRequestBudgetBytes {
			t.Fatalf("round %d full request = %d bytes > %d", round, fullRequest, ModelFullRequestBudgetBytes)
		}
		if overflow := ModelContextOverflow(body); overflow != "" {
			t.Fatalf("round %d failed closed unexpectedly: %s", round, overflow)
		}
		// Delta-only ledger: only the current round receipts are projected.
		projected := modelMap(model["observation_ledger"])
		receiptRows := modelRows(projected["receipts"])
		if len(receiptRows) != 1 || modelText(receiptRows[0]["receipt_id"]) != fmt.Sprintf("receipt-%02d", round) {
			t.Fatalf("round %d ledger delta window = %#v", round, receiptRows)
		}
		window := modelMap(modelMap(projected["history_window"])["receipts"])
		if modelText(window["total"]) != fmt.Sprintf("%d", round) ||
			modelText(window["latest_round"]) != fmt.Sprintf("%d", round) ||
			modelText(window["earliest_round"]) != "1" || modelText(window["cold_ref"]) == "" {
			t.Fatalf("round %d ledger history folding = %#v", round, window)
		}
		// Size stability: consecutive rounds must not grow monotonically.
		if prevTotal > 0 {
			if delta := len(body) - prevTotal; delta > 2*1024 {
				t.Fatalf("round %d grew by %d bytes (unstable)", round, delta)
			}
		}
		prevTotal = len(body)
	}
}

func TestModelProjectionStructureDigestExposesTrackIdentities(t *testing.T) {
	// The structure digest must carry the minimal track identity table the
	// model needs to request track-targeted observations; without it the model
	// cannot proceed past project-level views (observed no-op loop).
	tracks := make([]any, 0, 6)
	for i := 0; i < 6; i++ {
		tracks = append(tracks, map[string]any{
			"track_id": fmt.Sprintf("10%02d", i+7), "track_name": []string{"bass", "drums", "guitar", "other", "piano", "vocals"}[i],
			"role_guess": "instrument", "active_state": "active",
		})
	}
	view := map[string]any{
		"view_id": "project.structure", "status": "ready",
		"facts": map[string]any{
			"project.tracks.summary": map[string]any{"status": "ready", "track_count": 6, "tracks": tracks},
			"observation.mom_projection": map[string]any{
				"mom_version": "v1", "intent": "project_structure",
				"llm_context": map[string]any{"summary_md": "six imported tracks visible", "evidence_refs": []any{"evidence://structure"}},
			},
		},
	}
	bundle := map[string]any{
		"schema_version": ccbBundleSchema, "status": "ready", "observation_id": "obs-struct",
		"requested_views": []any{"project.structure"}, "views": map[string]any{"project.structure": view},
	}
	result := planner.ToolResult{ToolCallID: "struct-call", Tool: "ccb.observation_request", Status: "ok", Result: map[string]any{"bundle": bundle}}
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &result}}
	full := Build(Input{GoalTrace: trace}, fixedOptions())
	model := ProjectModelSnapshot(ModelProjectionInput{Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileSelection}, fixedOptions())
	digestView := modelMap(modelMap(modelMap(model["active_observation"])["views"])["project.structure"])
	rows := modelRows(digestView["tracks"])
	if len(rows) != 6 || modelText(rows[0]["track_id"]) != "1007" || modelText(rows[5]["track_id"]) != "1012" {
		t.Fatalf("structure digest track identities missing: %#v", rows)
	}
	if modelText(digestView["digest"]) == "" {
		t.Fatalf("structure digest lost summary: %#v", digestView)
	}
	size := modelMap(model["context_size"])
	if modelInteger(size["hot_bytes"]) > ModelHotBudgetBytes {
		t.Fatalf("structure digest exceeded hot budget: %#v", size)
	}
}

func TestModelProjectionLargeFrequencyViewStaysWithinHotBudget(t *testing.T) {
	// A ready multi-track frequency relationship is data-rich (per-track band
	// profiles, conflict candidates, regions). With an LLMContext present the
	// hot digest must stay small; a view without one must degrade to a cold
	// reference instead of inlining the full projection and overflowing hot.
	opts := fixedOptions()
	opts.MaxTextRunes = 2000
	profiles := make([]any, 0, 6)
	for i := 0; i < 6; i++ {
		profiles = append(profiles, map[string]any{
			"track_id": fmt.Sprintf("track-%02d", i), "name": "Track " + fmt.Sprint(i), "status": "ready", "tap_point": "source_file_pre_fx",
			"measurement_conditions": map[string]any{"tap_point": "source_file_pre_fx", "sample_rate": 44100.0, "channel_count": 2, "analyzer_version": "v1", "start_seconds": 0, "end_seconds": 20},
			"bands": map[string]any{
				"sub": map[string]any{"unit_energy": .1}, "bass": map[string]any{"unit_energy": .3},
				"low_mid": map[string]any{"unit_energy": .2}, "mid": map[string]any{"unit_energy": .2},
				"presence": map[string]any{"unit_energy": .1}, "air": map[string]any{"unit_energy": .05},
			},
		})
	}
	bigFR := map[string]any{
		"status": "ready", "tap_point": "source_file_pre_fx", "coverage": map[string]any{"eligible_track_count": 6, "conflict_candidate_count": 4},
		"track_profiles": profiles,
		"conflict_candidates": []any{
			map[string]any{"region": "bass", "type": "frequency_energy_overlap_candidate", "tracks": []any{map[string]any{"track_id": "track-01", "name": "Bass", "unit_energy": .3}, map[string]any{"track_id": "track-02", "name": "Drums", "unit_energy": .29}}},
			map[string]any{"region": "low_mid", "type": "frequency_energy_overlap_candidate", "tracks": []any{map[string]any{"track_id": "track-02", "name": "Drums", "unit_energy": .2}, map[string]any{"track_id": "track-03", "name": "Guitar", "unit_energy": .19}}},
			map[string]any{"region": "mid", "type": "frequency_energy_overlap_candidate", "tracks": []any{map[string]any{"track_id": "track-04", "name": "Piano", "unit_energy": .2}, map[string]any{"track_id": "track-05", "name": "Other", "unit_energy": .21}}},
			map[string]any{"region": "presence", "type": "frequency_energy_overlap_candidate", "tracks": []any{map[string]any{"track_id": "track-05", "name": "Other", "unit_energy": .15}, map[string]any{"track_id": "track-06", "name": "Vocals", "unit_energy": .16}}},
		},
		"tonal_tendencies": []any{
			map[string]any{"track_id": "track-01", "strongest_region": "bass", "weakest_region": "air", "relative_spread_db": 3.1},
			map[string]any{"track_id": "track-02", "strongest_region": "bass", "weakest_region": "air", "relative_spread_db": 2.8},
			map[string]any{"track_id": "track-03", "strongest_region": "mid", "weakest_region": "sub", "relative_spread_db": 2.4},
			map[string]any{"track_id": "track-04", "strongest_region": "mid", "weakest_region": "sub", "relative_spread_db": 3.4},
			map[string]any{"track_id": "track-05", "strongest_region": "presence", "weakest_region": "sub", "relative_spread_db": 2.9},
			map[string]any{"track_id": "track-06", "strongest_region": "presence", "weakest_region": "bass", "relative_spread_db": 3.6},
		},
		"frequency_regions": []any{}, "limitations": []string{"candidate only"},
	}
	for _, withLLMContext := range []bool{true, false} {
		facts := map[string]any{"observation.mom_projection": map[string]any{
			"mom_version": "v1", "intent": "project_frequency_relationship_observation",
			"frequency_relationship": bigFR,
		}}
		if withLLMContext {
			facts["observation.mom_projection"].(map[string]any)["llm_context"] = map[string]any{
				"summary_md": "ready multi-track frequency relationship with 4 candidate conflicts", "evidence_refs": []any{"evidence://fr"},
			}
		}
		view := map[string]any{"view_id": "mix.frequency_relationship", "status": "ready", "facts": facts}
		bundle := map[string]any{
			"schema_version": ccbBundleSchema, "status": "ready", "observation_id": "obs-fr", "requested_views": []any{"mix.frequency_relationship"},
			"views": map[string]any{"mix.frequency_relationship": view}, "evidence_refs": []any{"evidence://fr"},
		}
		result := planner.ToolResult{ToolCallID: "fr-call", Tool: "ccb.observation_request", Status: "ok", Result: map[string]any{"bundle": bundle}}
		trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &result}}
		full := Build(Input{GoalTrace: trace}, opts)
		model := ProjectModelSnapshot(ModelProjectionInput{Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileSelection}, opts)
		size := modelMap(model["context_size"])
		hot := modelInteger(size["hot_bytes"])
		digestView := modelMap(modelMap(modelMap(model["active_observation"])["views"])["mix.frequency_relationship"])
		if withLLMContext {
			if modelText(digestView["digest"]) == "" || digestView["view_projection"] != nil {
				t.Fatalf("llm_context view did not use digest path: %#v", digestView)
			}
		} else {
			// Oversized fallbacks degrade to a cold ref; a compact fallback may
			// stay inline. Either way the hot layer must stay within budget.
			if digestView["view_projection"] != nil && digestView["view_ref"] != nil {
				t.Fatalf("fallback both projected and referenced: %#v", digestView)
			}
			if digestView["view_projection"] == nil && digestView["view_ref"] == nil {
				t.Fatalf("fallback lost: %#v", digestView)
			}
		}
		if hot > ModelHotBudgetBytes {
			t.Fatalf("withLLMContext=%v hot = %d > %d", withLLMContext, hot, ModelHotBudgetBytes)
		}
		if overflow := ModelContextOverflow(ModelJSON(model)); overflow != "" {
			t.Fatalf("withLLMContext=%v overflowed: %s", withLLMContext, overflow)
		}
	}
}

func TestModelProjectionMultiViewObservationStaysWithinHotBudget(t *testing.T) {
	// A wide observation (many views, long per-view summaries) must stay inside
	// the hot budget through the digest path; the full facts live in warm/cold.
	views := map[string]any{}
	requested := make([]any, 0, 8)
	for i, viewID := range []string{"project.structure", "track.basic_energy", "track.time_dynamics", "track.timbre_frequency", "track.peak_structure", "track.activity_structure", "track.transient_structure", "track.band_dynamics"} {
		requested = append(requested, viewID)
		views[viewID] = map[string]any{
			"view_id": viewID, "status": "ready",
			"facts": map[string]any{
				fmt.Sprintf("observation.projection_%02d", i): map[string]any{
					"llm_context": map[string]any{
						"summary_md":    strings.Repeat(fmt.Sprintf("view %d summary ", i), 30),
						"evidence_refs": []any{fmt.Sprintf("evidence://view-%d", i)},
					},
				},
			},
		}
	}
	bundle := map[string]any{
		"schema_version":  ccbBundleSchema,
		"bundle_id":       "bundle-wide",
		"status":          "ready",
		"observation_id":  "obs-wide",
		"requested_views": requested,
		"evidence_refs":   []any{"evidence://bundle"},
		"views":           views,
	}
	result := planner.ToolResult{ToolCallID: "wide-call", Tool: "ccb.observation_request", Status: "ok", Result: map[string]any{"bundle": bundle}}
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &result}}
	full := Build(Input{GoalTrace: trace}, fixedOptions())
	model := ProjectModelSnapshot(ModelProjectionInput{Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileSelection}, fixedOptions())
	size := modelMap(model["context_size"])
	hot := modelInteger(size["hot_bytes"])
	if hot > ModelHotBudgetBytes {
		t.Fatalf("multi-view hot = %d > %d", hot, ModelHotBudgetBytes)
	}
	if overflow := ModelContextOverflow(ModelJSON(model)); overflow != "" {
		t.Fatalf("multi-view observation overflowed: %s", overflow)
	}
}

func TestModelProjectionSameDataAppearsOncePerRequest(t *testing.T) {
	ccb := testCCBBundleResult("once-call", "once-fact", "once-raw")
	ordinary := planner.ToolResult{
		ToolCallID: "once-ordinary", Tool: "workspace.read_file", Status: "ok",
		Result: map[string]any{"content": "once-ordinary-content"},
	}
	trace := []planner.TraceEvent{
		{Kind: "tool_result", ToolResult: &ccb},
		{Kind: "tool_result", ToolResult: &ordinary},
	}
	full := Build(Input{GoalTrace: trace}, fixedOptions())
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, Profile: ModelContextProfileSelection,
	}, fixedOptions())
	body := ModelJSON(model)
	// The same observation fact and the same ordinary result content each
	// appear exactly once in a single model request.
	if count := strings.Count(body, "once-fact"); count != 1 {
		t.Fatalf("observation fact count = %d, want 1: %s", count, body)
	}
	if count := strings.Count(body, "once-ordinary-content"); count != 1 {
		t.Fatalf("ordinary result count = %d, want 1: %s", count, body)
	}
	if count := strings.Count(body, `"schema_version":"`+CCBModelProjectionSchema+`"`); count != 1 {
		t.Fatalf("canonical projection count = %d, want 1: %s", count, body)
	}
	if strings.Count(body, "once-raw") != 0 {
		t.Fatal("raw audit payload entered the model request")
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
