package contextruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/planner"
)

const (
	CCBModelProjectionSchema = "ccb_model_projection.v1"
	ccbCatalogSchema         = "ccb_observation_catalog.v1"
	ccbBundleSchema          = "ccb_observation_bundle.v1"
)

// ModelProjectionInput separates the complete runtime snapshot from the
// model-visible projection. GoalTrace remains authoritative for audit and
// recovery; only its canonical CCB projection is copied into the model view.
type ModelProjectionInput struct {
	Snapshot          Snapshot
	GoalTrace         []planner.TraceEvent
	RecentObservation map[string]any
}

// ProjectModelSnapshot removes repeated CCB summaries from the known snapshot
// sections and installs exactly one canonical active_observation.
func ProjectModelSnapshot(in ModelProjectionInput, opts Options) map[string]any {
	opts = normalizeOptions(opts)
	out := in.Snapshot.Map()
	projections, refsByCallID, refsByTool := collectCCBProjections(in.GoalTrace, opts)
	stripRepeatedCCBResults(out, refsByCallID, refsByTool)

	active := latestCCBProjection(projections)
	if len(active) == 0 {
		active = projectRecentCCBObservation(in.RecentObservation, opts)
	}
	if len(active) > 0 {
		out["active_observation"] = active
	}
	if catalogRef := latestCatalogReference(projections); len(catalogRef) > 0 && modelText(active["kind"]) != "catalog" {
		out["observation_catalog_ref"] = catalogRef
	}
	return out
}

// ModelJSON uses compact JSON for the model-only view. The audit snapshot
// retains its existing serialization contract.
func ModelJSON(snapshot map[string]any) string {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func ModelSectionBytes(snapshot map[string]any) map[string]int {
	out := make(map[string]int, len(snapshot))
	for key, value := range snapshot {
		data, _ := json.Marshal(value)
		out[key] = len(data)
	}
	return out
}

// ProjectCCBToolResult recognizes the real harness wrapper shapes:
// result["catalog"] and result["bundle"]. It never falls back to a raw CCB
// view when a safe model projection is unavailable.
func ProjectCCBToolResult(result planner.ToolResult, opts Options) (map[string]any, bool) {
	opts = normalizeOptions(opts)
	if catalog := modelMap(result.Result["catalog"]); modelText(catalog["schema_version"]) == ccbCatalogSchema {
		return projectCCBCatalog(result, catalog, opts), true
	}
	if bundle := modelMap(result.Result["bundle"]); modelText(bundle["schema_version"]) == ccbBundleSchema {
		return projectCCBBundle(result, bundle, opts), true
	}
	return nil, false
}

func projectCCBCatalog(result planner.ToolResult, catalog map[string]any, opts Options) map[string]any {
	views := make([]map[string]any, 0)
	for _, view := range modelRows(catalog["views"]) {
		row := selectModelFields(view, "view_id", "availability", "limitations")
		if questions := modelStrings(view["questions"]); len(questions) > 0 {
			row["question"] = questions[0]
		}
		if len(row) > 0 {
			views = append(views, row)
		}
	}
	index := selectModelFields(catalog, "schema_version", "boundary", "target_ref", "exclusions")
	index["views"] = views
	index["projection_status"] = "partial"
	index["projection_kind"] = "short_index"
	index["omitted_fields"] = []any{"supported_target_kinds", "temporal_resolution", "cost_latency_class", "quality_ceiling", "required_dependencies"}
	version := catalogProjectionVersion(index)
	return removeEmptyModelFields(map[string]any{
		"schema_version":  CCBModelProjectionSchema,
		"kind":            "catalog",
		"tool_call_id":    strings.TrimSpace(result.ToolCallID),
		"tool":            strings.TrimSpace(result.Tool),
		"status":          strings.TrimSpace(result.Status),
		"catalog_version": version,
		"catalog_index":   sanitizeModelProjection(index, opts),
	})
}

func projectCCBBundle(result planner.ToolResult, bundle map[string]any, opts Options) map[string]any {
	views := modelMap(bundle["views"])
	requested := modelStrings(bundle["requested_views"])
	if len(requested) == 0 {
		for viewID := range views {
			requested = append(requested, viewID)
		}
		sort.Strings(requested)
	}
	projectedViews := make(map[string]any, len(requested))
	readyViews := 0
	unsupportedViews := 0
	for _, viewID := range requested {
		projected := projectCCBView(viewID, modelMap(views[viewID]), opts)
		projectedViews[viewID] = projected
		switch modelText(projected["projection_status"]) {
		case "ready", "partial":
			readyViews++
		default:
			unsupportedViews++
		}
	}
	bundleProjectionStatus := "ready"
	if readyViews == 0 && unsupportedViews > 0 {
		bundleProjectionStatus = "unsupported_projection"
	} else if unsupportedViews > 0 {
		bundleProjectionStatus = "partial"
	}
	audit := modelMap(bundle["audit_receipt"])
	auditRef := removeEmptyModelFields(map[string]any{
		"receipt_id":       modelText(audit["receipt_id"]),
		"receipt_schema":   modelText(audit["schema_version"]),
		"bundle_id":        modelText(bundle["bundle_id"]),
		"view_set_matches": audit["view_set_matches"],
	})
	return removeEmptyModelFields(map[string]any{
		"schema_version":     CCBModelProjectionSchema,
		"kind":               "observation",
		"projection_status":  bundleProjectionStatus,
		"tool_call_id":       strings.TrimSpace(result.ToolCallID),
		"tool":               strings.TrimSpace(result.Tool),
		"status":             firstModelText(result.Status, modelText(bundle["status"])),
		"bundle_status":      modelText(bundle["status"]),
		"observation_id":     modelText(bundle["observation_id"]),
		"bundle_id":          modelText(bundle["bundle_id"]),
		"request_id":         modelText(bundle["request_id"]),
		"read_only":          bundle["read_only"],
		"mutation_authority": bundle["mutation_authority"],
		"target_ref":         sanitizeModelProjection(bundle["target_ref"], opts),
		"freshness":          sanitizeModelProjection(bundle["freshness"], opts),
		"requested_views":    requested,
		"views":              projectedViews,
		"limitations":        sanitizeModelProjection(bundle["limitations"], opts),
		"omission_reasons":   sanitizeModelProjection(bundle["omission_reasons"], opts),
		"omissions":          sanitizeModelProjection(bundle["omissions"], opts),
		"evidence_refs":      sanitizeModelProjection(bundle["evidence_refs"], opts),
		"audit_ref":          auditRef,
	})
}

func projectCCBView(viewID string, view map[string]any, opts Options) map[string]any {
	status := modelText(view["status"])
	out := removeEmptyModelFields(map[string]any{
		"view_id":     firstModelText(modelText(view["view_id"]), viewID),
		"status":      status,
		"limitations": sanitizeModelProjection(view["limitations"], opts),
	})
	if llmContext := findViewLLMContext(view); len(llmContext) > 0 {
		out["projection_status"] = projectionStatus(status)
		out["llm_context"] = sanitizeModelProjection(llmContext, opts)
		return out
	}
	if equivalent := equivalentCCBViewProjection(viewID, modelMap(view["facts"]), opts); len(equivalent) > 0 {
		out["projection_status"] = projectionStatus(status)
		out["view_projection"] = equivalent
		return out
	}
	if status == "missing" || status == "unavailable" || status == "deferred" {
		out["projection_status"] = "omitted"
	} else {
		out["projection_status"] = "unsupported_projection"
	}
	return out
}

func findViewLLMContext(view map[string]any) map[string]any {
	if direct := modelMap(view["llm_context"]); len(direct) > 0 {
		return direct
	}
	facts := modelMap(view["facts"])
	for _, key := range sortedModelKeys(facts) {
		if context := modelMap(modelMap(facts[key])["llm_context"]); len(context) > 0 {
			return context
		}
	}
	return nil
}

func equivalentCCBViewProjection(viewID string, facts map[string]any, opts Options) map[string]any {
	var out map[string]any
	switch viewID {
	case "project.structure":
		out = map[string]any{
			"project_summary": modelFactFields(facts, "project.static.summary", opts,
				"status", "track_count", "active_track_count", "duration_seconds", "sample_rate", "channel_count"),
			"track_summary": modelFactFields(facts, "project.tracks.summary", opts,
				"status", "track_count", "active_track_count", "tracks"),
			"mom_structure": modelNestedFactFields(facts, "observation.mom_projection", "project_structure", opts,
				"status", "freshness", "duration_seconds", "sample_rate", "channel_count", "target_ref", "listen_scope", "evidence_refs", "limitations"),
			"technical_summary": modelFactFields(facts, "observation.tim_projection", opts,
				"status", "technical_summary", "risk_summary", "coverage", "evidence_refs", "limitations"),
		}
	case "track.basic_energy":
		out = map[string]any{
			"track":  modelFactFields(facts, factKeyWithSuffix(facts, ".static.identity"), opts, "track_identity", "status"),
			"levels": modelFactFields(facts, factKeyWithSuffix(facts, ".fast.levels"), opts, "status", "rms_dbfs", "peak_dbfs", "headroom_db", "crest_db", "lufs", "limitations", "evidence_refs"),
		}
	case "track.time_dynamics":
		out = map[string]any{
			"time_energy":     modelFactFields(facts, factKeyWithSuffix(facts, ".slow.time_energy.summary"), opts, "status", "summary", "rows", "macro_dynamics", "limitations", "evidence_refs"),
			"source_dynamics": modelNestedFactFields(facts, "observation.com_projection", "source_dynamics", opts, "status", "summary", "macro_dynamics", "dynamic_range", "crest", "limitations", "evidence_refs"),
		}
	case "track.timbre_frequency":
		out = map[string]any{"band_energy": modelFactFields(facts, factKeyWithSuffix(facts, ".slow.band_energy.summary"), opts, "status", "bands", "band_energy", "summary", "limitations", "evidence_refs")}
	case "track.peak_structure", "track.activity_structure", "track.frequency_time_events", "track.transient_structure", "track.band_dynamics":
		dimension := map[string]string{
			"track.peak_structure":        "peak_structure",
			"track.activity_structure":    "activity_structure",
			"track.frequency_time_events": "frequency_time_events",
			"track.transient_structure":   "transient_structure",
			"track.band_dynamics":         "band_dynamics",
		}[viewID]
		out = map[string]any{
			dimension: modelNestedFactFields(facts, "observation.dom_projection", dimension, opts,
				"status", "summary", "facts", "events", "segments", "bands", "limitations", "evidence_refs"),
			"readiness": modelFactFields(facts, "observation.dom_projection", opts, "status", "dimension_readiness", "trust_quality", "evidence_refs", "limitations"),
		}
	case "track.stereo_space":
		out = map[string]any{"stereo": modelFactFields(facts, factKeyWithSuffix(facts, ".slow.stereo.summary"), opts, "status", "correlation_estimate", "width", "balance", "summary", "limitations", "evidence_refs")}
	case "mix.multitrack_relationship":
		out = map[string]any{
			"relationship": modelNestedFactFields(facts, "observation.mom_projection", "multitrack_relation", opts,
				"status", "freshness", "scope", "loudness_order", "peak_order", "headroom_risk_tracks", "band_conflict_candidates", "phase_risk_tracks", "limitations", "evidence_refs"),
			"relationship_inputs": modelFactFields(facts, "project.relationship_inputs", opts, "status", "track_count", "tracks", "limitations", "evidence_refs"),
			"level_ranking":       modelFactFields(facts, "project.rankings.level", opts, "status", "tracks", "ranking", "limitations", "evidence_refs"),
			"peak_ranking":        modelFactFields(facts, "project.rankings.peak", opts, "status", "tracks", "ranking", "limitations", "evidence_refs"),
			"headroom_risks":      modelFactFields(facts, "project.risks.headroom", opts, "status", "tracks", "risks", "limitations", "evidence_refs"),
		}
	case "mix.frequency_relationship":
		out = map[string]any{
			"frequency_relationship": modelNestedFactFields(facts, "observation.mom_projection", "frequency_relationship", opts,
				"status", "freshness", "scope", "tap_point", "coverage", "conflict_candidates", "tonal_tendencies", "verification_dimensions", "limitations", "evidence_refs"),
			"relationship_inputs": modelFactFields(facts, "project.frequency_relationship_inputs", opts,
				"status", "track_count", "usable_track_count", "tap_points", "decision_tracks_truncated", "tracks"),
		}
	case "processor.behavior":
		out = map[string]any{"behavior": modelFactFields(facts, "observation.com_projection", opts, "status", "mode", "gain_action", "transient_response", "recovery_motion", "level_effect", "stereo_behavior", "trigger_relation", "trust_quality", "limitations", "evidence_refs")}
	case "processor.change_delta":
		out = map[string]any{"change": modelFactFields(facts, "observation.com_projection", opts, "status", "mode", "behavior_change", "trust_quality", "limitations", "evidence_refs")}
	case "comparison.before_after":
		out = map[string]any{"comparison": modelFactFields(facts, "observation.before_after.latest", opts, "status", "summary", "changes", "limitations", "evidence_refs")}
	default:
		return nil
	}
	out = modelMap(sanitizeModelProjection(out, opts))
	removeEmptyModelFields(out)
	if !hasMeaningfulModelProjection(out) {
		return nil
	}
	return out
}

func modelFactFields(facts map[string]any, factKey string, opts Options, keys ...string) map[string]any {
	if strings.TrimSpace(factKey) == "" {
		return nil
	}
	return modelMap(sanitizeModelProjection(selectModelFields(modelMap(facts[factKey]), keys...), opts))
}

func modelNestedFactFields(facts map[string]any, factKey, nested string, opts Options, keys ...string) map[string]any {
	return modelMap(sanitizeModelProjection(selectModelFields(modelMap(modelMap(facts[factKey])[nested]), keys...), opts))
}

func factKeyWithSuffix(facts map[string]any, suffix string) string {
	for _, key := range sortedModelKeys(facts) {
		if strings.HasSuffix(key, suffix) {
			return key
		}
	}
	return ""
}

func projectionStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "partial", "stale", "suspect", "approximate":
		return "partial"
	case "missing", "deferred", "unavailable", "blocked":
		return "omitted"
	default:
		return "ready"
	}
}

func collectCCBProjections(trace []planner.TraceEvent, opts Options) ([]map[string]any, map[string]map[string]any, map[string]map[string]any) {
	projections := make([]map[string]any, 0)
	byCallID := map[string]map[string]any{}
	byTool := map[string]map[string]any{}
	for _, event := range trace {
		if event.ToolResult == nil {
			continue
		}
		projection, ok := ProjectCCBToolResult(*event.ToolResult, opts)
		if !ok {
			continue
		}
		projections = append(projections, projection)
		ref := ccbProjectionReference(projection)
		if callID := modelText(projection["tool_call_id"]); callID != "" {
			byCallID[callID] = ref
		}
		if tool := normalizedModelTool(modelText(projection["tool"])); tool != "" {
			byTool[tool] = ref
		}
	}
	return projections, byCallID, byTool
}

func latestCCBProjection(projections []map[string]any) map[string]any {
	if len(projections) == 0 {
		return nil
	}
	return cloneModelMap(projections[len(projections)-1])
}

func latestCatalogReference(projections []map[string]any) map[string]any {
	for i := len(projections) - 1; i >= 0; i-- {
		if modelText(projections[i]["kind"]) != "catalog" {
			continue
		}
		ref := ccbProjectionReference(projections[i])
		if index := modelMap(projections[i]["catalog_index"]); len(index) > 0 {
			viewIDs := make([]string, 0)
			for _, row := range modelRows(index["views"]) {
				if viewID := modelText(row["view_id"]); viewID != "" {
					viewIDs = append(viewIDs, viewID)
				}
			}
			ref["view_ids"] = viewIDs
		}
		return ref
	}
	return nil
}

func projectRecentCCBObservation(recent map[string]any, opts Options) map[string]any {
	if len(recent) == 0 || !isCCBTool(modelText(recent["tool"])) {
		return nil
	}
	summary := modelMap(recent["summary"])
	if modelText(summary["schema_version"]) == CCBModelProjectionSchema {
		out := cloneModelMap(summary)
		if modelText(out["tool_call_id"]) == "" {
			out["tool_call_id"] = modelText(recent["tool_call_id"])
		}
		return out
	}
	result := planner.ToolResult{
		ToolCallID: modelText(recent["tool_call_id"]),
		Tool:       modelText(recent["tool"]),
		Status:     modelText(recent["status"]),
		Result:     map[string]any{"bundle": summary},
	}
	projection, _ := ProjectCCBToolResult(result, opts)
	return projection
}

func stripRepeatedCCBResults(snapshot map[string]any, refsByCallID, refsByTool map[string]map[string]any) {
	if snapshot["tool_result_summary"] != nil {
		rows := modelRows(snapshot["tool_result_summary"])
		kept := make([]any, 0, len(rows))
		for _, raw := range rows {
			if isCCBSummary(raw) {
				continue
			}
			kept = append(kept, raw)
		}
		if len(kept) == 0 {
			delete(snapshot, "tool_result_summary")
		} else {
			snapshot["tool_result_summary"] = kept
		}
	}
	trace := modelMap(snapshot["goal_trace_summary"])
	if trace["recent_events"] != nil {
		events := modelRows(trace["recent_events"])
		for _, event := range events {
			result := modelMap(event["tool_result"])
			if !isCCBSummary(result) {
				continue
			}
			delete(event, "tool_result")
			event["tool_result_ref"] = referenceForSummary(result, refsByCallID, refsByTool)
		}
		trace["recent_events"] = events
	}
	semantic := modelMap(snapshot["daw_semantic_summary"])
	if result := modelMap(semantic["recent_execution_result"]); isCCBSummary(result) {
		delete(semantic, "recent_execution_result")
		semantic["recent_execution_result_ref"] = referenceForSummary(result, refsByCallID, refsByTool)
	}
	if len(semantic) > 0 {
		snapshot["daw_semantic_summary"] = semantic
	}
	stripRecentGoalCCB(modelMap(snapshot["recent_goal_context"]), refsByCallID, refsByTool)
}

func stripRecentGoalCCB(context map[string]any, refsByCallID, refsByTool map[string]map[string]any) {
	if len(context) == 0 {
		return
	}
	if result := modelMap(context["recent_tool_result"]); isCCBSummary(result) {
		delete(context, "recent_tool_result")
		context["recent_tool_result_ref"] = referenceForSummary(result, refsByCallID, refsByTool)
	}
	if observation := modelMap(context["recent_observation"]); isCCBTool(modelText(observation["tool"])) {
		delete(context, "recent_observation")
		context["recent_observation_ref"] = referenceForSummary(observation, refsByCallID, refsByTool)
	}
	stripRecentGoalCCB(modelMap(context["inherited_recent_goal_context"]), refsByCallID, refsByTool)
}

func referenceForSummary(summary map[string]any, refsByCallID, refsByTool map[string]map[string]any) map[string]any {
	if callID := modelText(summary["tool_call_id"]); callID != "" {
		if ref := refsByCallID[callID]; len(ref) > 0 {
			return cloneModelMap(ref)
		}
	}
	if ref := refsByTool[normalizedModelTool(modelText(summary["tool"]))]; len(ref) > 0 {
		return cloneModelMap(ref)
	}
	ref := removeEmptyModelFields(map[string]any{
		"tool_call_id": modelText(summary["tool_call_id"]),
		"tool":         modelText(summary["tool"]),
		"status":       modelText(summary["status"]),
	})
	metadata := summary
	if nested := modelMap(summary["summary"]); len(nested) > 0 {
		metadata = nested
	}
	for _, key := range []string{"observation_id", "bundle_id", "request_id", "catalog_version", "requested_views", "freshness", "limitations", "evidence_refs"} {
		if value, ok := metadata[key]; ok && !isEmptyValue(value) {
			ref[key] = sanitizeModelProjection(value, normalizeOptions(Options{}))
		}
	}
	if modelText(ref["observation_id"]) == "" {
		if important := modelMap(summary["important_fields"]); len(important) > 0 {
			for key, value := range important {
				if strings.HasSuffix(key, ".observation_id") || key == "observation_id" {
					ref["observation_id"] = value
				}
			}
		}
	}
	return ref
}

func ccbProjectionReference(projection map[string]any) map[string]any {
	return removeEmptyModelFields(map[string]any{
		"tool_call_id":    modelText(projection["tool_call_id"]),
		"tool":            modelText(projection["tool"]),
		"status":          modelText(projection["status"]),
		"kind":            modelText(projection["kind"]),
		"observation_id":  modelText(projection["observation_id"]),
		"bundle_id":       modelText(projection["bundle_id"]),
		"catalog_version": modelText(projection["catalog_version"]),
		"requested_views": projection["requested_views"],
	})
}

func isCCBSummary(row map[string]any) bool {
	if len(row) == 0 {
		return false
	}
	if isCCBTool(modelText(row["tool"])) || modelText(row["schema_version"]) == CCBModelProjectionSchema {
		return true
	}
	return modelText(row["kind"]) == "catalog" || modelText(row["kind"]) == "observation"
}

func isCCBTool(tool string) bool {
	tool = normalizedModelTool(tool)
	return tool == "ccb.observation_catalog" || tool == "ccb.observation_request"
}

func normalizedModelTool(tool string) string {
	tool = strings.ToLower(strings.TrimSpace(tool))
	switch tool {
	case "ccb_observation_catalog":
		return "ccb.observation_catalog"
	case "ccb_observation_request":
		return "ccb.observation_request"
	default:
		return tool
	}
}

func catalogProjectionVersion(index map[string]any) string {
	data, _ := json.Marshal(index)
	digest := sha256.Sum256(data)
	return ccbCatalogSchema + ":" + hex.EncodeToString(digest[:6])
}

func sanitizeModelProjection(value any, opts Options) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for _, key := range sortedModelKeys(typed) {
			if forbiddenModelProjectionKey(key) {
				continue
			}
			out[key] = sanitizeModelProjection(typed[key], opts)
		}
		return removeEmptyModelFields(out)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeModelProjection(item, opts))
		}
		return out
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeModelProjection(item, opts))
		}
		return out
	case string:
		return compactText(typed, opts.MaxTextRunes)
	default:
		if row := modelMapNonRecursive(value); len(row) > 0 {
			return sanitizeModelProjection(row, opts)
		}
		return value
	}
}

func forbiddenModelProjectionKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, marker := range []string{
		"plugin", "vendor", "manufacturer", "product", "generic_eq_topology",
		"identity_card", "processor_identity", "topology", "parameter_id", "param_id",
		"required_coverage", "default_coverage", "coverage_axes", "expected_coverage", "expected_view",
		"recommended_family", "recommended_processor", "target_family", "processor_type", "sealed_truth",
	} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return key == "identifier" || strings.HasSuffix(key, "_identifier")
}

func selectModelFields(source map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := source[key]; ok && !isEmptyValue(value) {
			out[key] = value
		}
	}
	return out
}

func hasMeaningfulModelProjection(row map[string]any) bool {
	for _, value := range row {
		switch typed := value.(type) {
		case map[string]any:
			if len(typed) > 0 {
				return true
			}
		case []any:
			if len(typed) > 0 {
				return true
			}
		default:
			if !isEmptyValue(value) {
				return true
			}
		}
	}
	return false
}

func removeEmptyModelFields(row map[string]any) map[string]any {
	for key, value := range row {
		if isEmptyValue(value) {
			delete(row, key)
		}
	}
	return row
}

func modelMap(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		return nil
	}
	return row
}

func modelMapNonRecursive(value any) map[string]any {
	switch value.(type) {
	case nil, string, bool, float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return nil
	}
	return modelMap(value)
}

func modelRows(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row := modelMap(item); len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	default:
		if value == nil {
			return nil
		}
		data, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		var rows []map[string]any
		_ = json.Unmarshal(data, &rows)
		return rows
	}
}

func modelStrings(value any) []string {
	out := []string{}
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
	case []any:
		for _, item := range typed {
			if text := modelText(item); text != "" {
				out = append(out, text)
			}
		}
	}
	return out
}

func modelText(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstModelText(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func sortedModelKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneModelMap(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	data, _ := json.Marshal(row)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}
