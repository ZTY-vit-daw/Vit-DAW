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
	CCBModelProjectionSchema    = "ccb_model_projection.v1"
	ccbCatalogSchema            = "ccb_observation_catalog.v1"
	ccbBundleSchema             = "ccb_observation_bundle.v1"
	ModelHotBudgetBytes         = 10 * 1024
	ModelWarmBudgetBytes        = 5 * 1024
	ModelFullRequestBudgetBytes = 24 * 1024
	// modelDigestFallbackMaxBytes bounds a per-view hot digest fallback
	// (views without an LLMContext). Oversized fallbacks degrade to a cold
	// reference instead of inlining a full projection into the hot layer.
	modelDigestFallbackMaxBytes = 2 * 1024
)

// ModelProjectionInput separates the complete runtime snapshot from the
// model-visible projection. GoalTrace remains authoritative for audit and
// recovery; only its canonical CCB projection is copied into the model view.
type ModelProjectionInput struct {
	Snapshot          Snapshot
	GoalTrace         []planner.TraceEvent
	RecentObservation map[string]any
	ObservationLedger map[string]any
	Profile           ModelContextProfile
}

// ModelContextProfile selects the amount of non-authoritative context that is
// visible to the model. Cold data remains in the audit snapshot and is never
// implicitly rehydrated into a model request.
type ModelContextProfile string

const (
	ModelContextProfileFull        ModelContextProfile = "full"
	ModelContextProfileSelection   ModelContextProfile = "processor_selection"
	ModelContextProfileMaterialize ModelContextProfile = "processor_materialization"
	ModelContextProfilePostAction  ModelContextProfile = "post_action_evaluation"
)

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
	if ledger := ProjectObservationLedger(in.ObservationLedger, opts); len(ledger) > 1 {
		out["observation_ledger"] = ledger
	}
	if catalogRef := latestCatalogReference(projections); len(catalogRef) > 0 && modelText(active["kind"]) != "catalog" {
		out["observation_catalog_ref"] = catalogRef
	}
	return applyModelContextProfile(out, in.Profile, opts)
}

func applyModelContextProfile(snapshot map[string]any, profile ModelContextProfile, opts Options) map[string]any {
	if len(snapshot) == 0 {
		return snapshot
	}
	if profile != "" && profile != ModelContextProfileFull {
		keep := modelProfileKeepSections(profile)
		degraded := []string{}
		for key := range snapshot {
			if !keep[key] {
				delete(snapshot, key)
				degraded = append(degraded, key)
			}
		}
		sort.Strings(degraded)
		snapshot["context_profile"] = string(profile)
		contextDegradation := map[string]any{
			"status":           "degraded",
			"omitted_sections": degraded,
			"cold_data":        "audit_snapshot_only",
		}
		if len(degraded) == 0 {
			contextDegradation["status"] = "none"
		}
		snapshot["context_degradation"] = contextDegradation
	} else {
		snapshot["context_degradation"] = map[string]any{"status": "none", "cold_data": "audit_snapshot_only"}
	}
	hotSections, warmSections := modelProfileLayerSections(profile, snapshot)
	snapshot["context_layers"] = map[string]any{
		"hot": map[string]any{
			"sections":     hotSections,
			"budget_bytes": ModelHotBudgetBytes,
			"policy":       "current-decision-and-fresh-evidence",
		},
		"warm": map[string]any{
			"sections":     warmSections,
			"budget_bytes": ModelWarmBudgetBytes,
			"mode":         "digest_and_reference_only",
		},
		"cold": map[string]any{
			"mode":   "audit_snapshot_only",
			"access": "deterministic_projection_required",
			"policy": "never-rehydrate-automatically",
		},
	}
	// Budget hard enforcement: degrade non-blocking sections to cold refs with
	// a CompactionMarker each, then fail closed with an explicit
	// context_overflow when the request still exceeds the layered budgets.
	// Facts are never silently truncated; degradation replaces content with a
	// reference, never drops it.
	markers, overflowed := enforceModelLayerBudgets(snapshot, hotSections, warmSections)
	hotBytes := modelLayerBytes(snapshot, hotSections)
	warmBytes := modelLayerBytes(snapshot, warmSections)
	contextDegradation := modelMap(snapshot["context_degradation"])
	switch {
	case overflowed:
		contextDegradation["budget_status"] = "context_overflow"
		contextDegradation["budget_action"] = "fail_closed_context_overflow"
	case len(markers) > 0:
		contextDegradation["budget_status"] = "degraded_within_budget"
		contextDegradation["budget_action"] = "degraded_non_blocking_sections_to_refs"
	default:
		contextDegradation["budget_status"] = "within_budget"
		contextDegradation["budget_action"] = "within_budget"
	}
	contextDegradation["hot_bytes"] = hotBytes
	contextDegradation["hot_target_bytes"] = ModelHotBudgetBytes
	contextDegradation["warm_bytes"] = warmBytes
	contextDegradation["warm_target_bytes"] = ModelWarmBudgetBytes
	snapshot["context_degradation"] = contextDegradation
	if len(markers) > 0 {
		snapshot["compaction_markers"] = markers
	}
	// The size report is part of the model view, so compute it to a small fixed
	// point: adding the report changes the serialized size (and its own section
	// size) by a few digits. This keeps the telemetry honest without truncating
	// any effective projection. Per-section bytes are recomputed by telemetry
	// from the emitted JSON and are not embedded in the model view.
	snapshot["context_size"] = map[string]any{}
	for i := 0; i < 4; i++ {
		snapshot["context_size"] = map[string]any{
			"total_bytes": modelJSONSize(snapshot),
			"hot_bytes":   hotBytes,
			"warm_bytes":  warmBytes,
			"hot_budget":  ModelHotBudgetBytes,
			"warm_budget": ModelWarmBudgetBytes,
			"full_budget": ModelFullRequestBudgetBytes,
		}
	}
	return snapshot
}

// enforceModelLayerBudgets deterministically degrades non-blocking sections to
// cold references while a layer exceeds its budget, records a CompactionMarker
// per degradation, and reports whether the request still overflows after every
// degradable section has been replaced. Degradation order is deterministic:
// warm sections largest-first, then supporting hot state (daw_state_summary,
// daw_semantic_summary). Current-decision sections (active_observation,
// current_selection, recent_goal_context) are blocking and never ref-ified;
// an over-budget hot layer after degradation fails closed.
func enforceModelLayerBudgets(snapshot map[string]any, hotSections, warmSections []string) ([]map[string]any, bool) {
	degradable := append([]string(nil), warmSections...)
	for _, section := range hotSections {
		if section == "daw_state_summary" || section == "daw_semantic_summary" {
			degradable = append(degradable, section)
		}
	}
	sort.SliceStable(degradable, func(i, j int) bool {
		return modelJSONSize(snapshot[degradable[i]]) > modelJSONSize(snapshot[degradable[j]])
	})
	markers := []map[string]any{}
	hotBytes := modelLayerBytes(snapshot, hotSections)
	warmBytes := modelLayerBytes(snapshot, warmSections)
	for _, section := range degradable {
		if hotBytes <= ModelHotBudgetBytes && warmBytes <= ModelWarmBudgetBytes {
			break
		}
		value, ok := snapshot[section]
		if !ok || isEmptyValue(value) {
			continue
		}
		before := modelJSONSize(value)
		snapshot[section] = map[string]any{
			"ref":       "audit_snapshot://" + section,
			"degraded":  true,
			"reason":    "over_budget_non_blocking_section",
			"cold_data": "audit_snapshot_only",
		}
		after := modelJSONSize(snapshot[section])
		markers = append(markers, map[string]any{
			"section":      section,
			"action":       "degraded_to_ref",
			"reason":       "over_budget_non_blocking_section",
			"bytes_before": before,
			"bytes_after":  after,
			"cold_ref":     "audit_snapshot://" + section,
		})
		hotBytes = modelLayerBytes(snapshot, hotSections)
		warmBytes = modelLayerBytes(snapshot, warmSections)
	}
	if hotBytes > ModelHotBudgetBytes || warmBytes > ModelWarmBudgetBytes {
		snapshot["context_overflow"] = map[string]any{
			"status":            "overflow",
			"hot_bytes":         hotBytes,
			"hot_budget_bytes":  ModelHotBudgetBytes,
			"warm_bytes":        warmBytes,
			"warm_budget_bytes": ModelWarmBudgetBytes,
			"overflowing": map[string]any{
				"hot":  modelLayerSectionBytes(snapshot, hotSections),
				"warm": modelLayerSectionBytes(snapshot, warmSections),
			},
			"cold_ref": "audit_snapshot://context_snapshot",
		}
		return markers, true
	}
	return markers, false
}

func modelLayerSectionBytes(snapshot map[string]any, sections []string) map[string]int {
	out := map[string]int{}
	for _, section := range sections {
		if value, ok := snapshot[section]; ok && !isEmptyValue(value) {
			out[section] = modelJSONSize(value)
		}
	}
	return out
}

func modelProfileKeepSections(profile ModelContextProfile) map[string]bool {
	keep := map[string]bool{
		"schema_version":          true,
		"goal_id":                 true,
		"run_id":                  true,
		"goal_summary":            true,
		"user_text":               true,
		"current_selection":       true,
		"daw_state_summary":       true,
		"active_observation":      true,
		"project_change":          true,
		"observation_ledger":      true,
		"observation_catalog_ref": true,
		// tool_result_summary is the single canonical copy of every non-CCB
		// tool result in the model view; all other sections reference it.
		"tool_result_summary": true,
		// The trace/context sections contain only references after
		// stripRepeatedCCBResults; they are the warm receipt surface needed to
		// explain a prior observation or a rejected view set.
		"recent_goal_context": true,
		"goal_trace_summary":  true,
	}
	switch profile {
	case ModelContextProfileMaterialize, ModelContextProfilePostAction:
		keep["daw_semantic_summary"] = true
	}
	return keep
}

func modelProfileLayerSections(profile ModelContextProfile, snapshot map[string]any) ([]string, []string) {
	hot := []string{"active_observation", "project_change", "current_selection", "daw_state_summary"}
	warm := []string{"recent_goal_context", "goal_trace_summary", "observation_catalog_ref", "observation_ledger"}
	switch profile {
	case ModelContextProfileMaterialize:
		hot = append(hot, "daw_semantic_summary")
	case ModelContextProfilePostAction:
		hot = append(hot, "recent_goal_context")
		warm = append(warm, "daw_semantic_summary")
	}
	hot = existingModelSections(snapshot, uniqueSortedStrings(hot))
	hotSet := map[string]bool{}
	for _, section := range hot {
		hotSet[section] = true
	}
	warm = existingModelSections(snapshot, uniqueSortedStrings(warm))
	filteredWarm := make([]string, 0, len(warm))
	for _, section := range warm {
		if !hotSet[section] {
			filteredWarm = append(filteredWarm, section)
		}
	}
	return hot, filteredWarm
}

// ProjectObservationLedger emits the durable observation memory the model
// needs to converge on a decision across turns: every observed view's latest
// row (available_views) and every recorded rejection (rejected_view_sets)
// survive across rounds, so the model never re-requests an already-observed
// view or a view set it was told not to retry. Only receipts stay delta-only
// (they mirror the most recent observation result already surfaced in the
// message). All sections are hard-capped so the projection stays bounded;
// older rows beyond the cap are folded into counts with earliest/latest round
// and a deterministic cold ref. Rows without a round marker (legacy fixtures)
// are treated as current so the bounded projection stays deterministic.
func ProjectObservationLedger(ledger map[string]any, opts Options) map[string]any {
	opts = normalizeOptions(opts)
	if len(ledger) == 0 {
		return nil
	}
	out := map[string]any{"schema_version": "free_state_observation_ledger.v1"}
	windows := map[string]any{}
	windowRound := ledgerWindowRound(ledger)
	if available := modelMap(ledger["available_views"]); len(available) > 0 {
		projected := map[string]any{}
		pairs := ledgerAvailableViewPairs(available)
		if len(pairs) > 24 {
			pairs = pairs[:24]
		}
		for _, pair := range pairs {
			row := selectModelFields(pair.row, "status", "observation_id", "target_ref", "conclusion")
			if receiptID := modelText(modelMap(pair.row["audit_ref"])["receipt_id"]); receiptID != "" {
				row["receipt_ref"] = receiptID
			}
			if clean := modelMap(sanitizeModelProjection(row, opts)); len(clean) > 0 {
				projected[pair.key] = clean
			}
		}
		if len(projected) > 0 {
			out["available_views"] = projected
		}
		total := modelInteger(ledger["view_observation_count"])
		windows["available_views"] = observationLedgerWindow(total, len(available), len(projected), ledgerRoundRange(ledgerAvailableViewRows(available)), ledgerColdRef("available_views"))
	}
	rejectedSource := modelRows(ledger["rejected_view_sets"])
	if rows := projectObservationLedgerRows(rejectedSource, opts,
		"fingerprint", "requested_views", "status", "receipt_id", "tool_call_id", "request_id", "observation_id", "retry_policy", "rejection_scope", "blocking_view_ids", "non_blocking_view_ids"); len(rows) > 0 {
		out["rejected_view_sets"] = rows
		windows["rejected_view_sets"] = observationLedgerWindow(modelInteger(ledger["rejected_view_set_count"]), len(rejectedSource), len(rows), ledgerRoundRange(rejectedSource), ledgerColdRef("rejected_view_sets"))
	}
	receiptSource := modelRows(ledger["receipts"])
	if rows := projectObservationLedgerRowsInWindow(receiptSource, opts, windowRound,
		"receipt_id", "tool_call_id", "observation_id", "status"); len(rows) > 0 {
		out["receipts"] = rows
		windows["receipts"] = observationLedgerWindow(modelInteger(ledger["receipt_count"]), len(receiptSource), len(rows), ledgerRoundRange(receiptSource), ledgerColdRef("receipts"))
	}
	if len(windows) > 0 {
		out["history_window"] = windows
	}
	return out
}

// ledgerAvailableViewPair pairs a ledger map key with its row so the
// projection can keep the exact key the writer assigned (track-scoped keys
// like "track:2::track.time_dynamics" differ from view_id).
type ledgerAvailableViewPair struct {
	key string
	row map[string]any
}

// ledgerAvailableViewPairs returns all available_views rows newest-first
// (rows without a round marker count as newest) so a bounded cap keeps the
// most recent per-view memory across rounds.
func ledgerAvailableViewPairs(available map[string]any) []ledgerAvailableViewPair {
	pairs := make([]ledgerAvailableViewPair, 0, len(available))
	for _, key := range sortedModelKeys(available) {
		pairs = append(pairs, ledgerAvailableViewPair{key: key, row: modelMap(available[key])})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return ledgerRowRecencyRank(pairs[i].row) > ledgerRowRecencyRank(pairs[j].row)
	})
	return pairs
}

// ledgerRowRecencyRank orders rows by recency; rows without a round marker are
// treated as the newest so legacy/current rows always survive the cap.
func ledgerRowRecencyRank(row map[string]any) int {
	if row == nil {
		return 1 << 30
	}
	round, ok := row["round"]
	if !ok || round == nil {
		return 1 << 30
	}
	return modelInteger(round)
}

// ledgerWindowRound derives the current delta window from the explicit
// window_round field or, when absent, from the latest round marker found on
// any row. The runtime writer always sets window_round; the row scan keeps
// projection deterministic for hand-built fixtures.
func ledgerWindowRound(ledger map[string]any) int {
	window := modelInteger(ledger["window_round"])
	for _, row := range modelRows(ledger["receipts"]) {
		if r := modelInteger(row["round"]); r > window {
			window = r
		}
	}
	for _, row := range modelRows(ledger["rejected_view_sets"]) {
		if r := modelInteger(row["round"]); r > window {
			window = r
		}
	}
	for _, raw := range modelMap(ledger["available_views"]) {
		if r := modelInteger(modelMap(raw)["round"]); r > window {
			window = r
		}
	}
	return window
}

// ledgerRowInWindow reports whether a row belongs to the current delta window.
// Legacy rows without a round marker stay visible; a row belongs to the
// current window only when its round equals the window round.
func ledgerRowInWindow(source map[string]any, windowRound int) bool {
	if source == nil {
		return true
	}
	round, ok := source["round"]
	if !ok || round == nil {
		return true
	}
	return modelInteger(round) == windowRound
}

func ledgerRoundRange(rows []map[string]any) [2]int {
	earliest, latest := 0, 0
	seen := false
	for _, row := range rows {
		if _, ok := row["round"]; !ok {
			continue
		}
		round := modelInteger(row["round"])
		if !seen {
			earliest, latest, seen = round, round, true
			continue
		}
		if round < earliest {
			earliest = round
		}
		if round > latest {
			latest = round
		}
	}
	return [2]int{earliest, latest}
}

func ledgerAvailableViewRows(available map[string]any) []map[string]any {
	rows := make([]map[string]any, 0, len(available))
	for _, raw := range available {
		if row := modelMap(raw); len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

func ledgerColdRef(section string) string {
	return "ledger://free_state_observation_ledger.v1/" + section + "/history"
}

func stripActiveObservationConclusion(ledger map[string]any, activeObservationID string) {
	if strings.TrimSpace(activeObservationID) == "" {
		return
	}
	for _, row := range modelMap(ledger["available_views"]) {
		entry := modelMap(row)
		if modelText(entry["observation_id"]) == activeObservationID {
			delete(entry, "conclusion")
		}
	}
}

func observationLedgerWindow(total, sourceCount, retained int, rounds [2]int, coldRef string) map[string]any {
	if total < sourceCount {
		total = sourceCount
	}
	if total < retained {
		total = retained
	}
	return map[string]any{
		"total": total, "retained": retained, "omitted": total - retained, "limit": 24,
		"earliest_round": rounds[0], "latest_round": rounds[1],
		"cold_ref": coldRef,
	}
}

func modelInteger(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func projectObservationLedgerRowsInWindow(value any, opts Options, windowRound int, keys ...string) []map[string]any {
	rows := modelRows(value)
	if len(rows) > 24 {
		rows = rows[len(rows)-24:]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if !ledgerRowInWindow(row, windowRound) {
			continue
		}
		if clean := modelMap(sanitizeModelProjection(selectModelFields(row, keys...), opts)); len(clean) > 0 {
			out = append(out, clean)
		}
	}
	return out
}

// projectObservationLedgerRows projects rows without a round-window filter:
// durable facts such as do_not_retry rejections stay visible across rounds so
// the model does not re-request what it was told not to retry. The row cap
// keeps the projection bounded.
func projectObservationLedgerRows(value any, opts Options, keys ...string) []map[string]any {
	rows := modelRows(value)
	if len(rows) > 24 {
		rows = rows[len(rows)-24:]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if clean := modelMap(sanitizeModelProjection(selectModelFields(row, keys...), opts)); len(clean) > 0 {
			out = append(out, clean)
		}
	}
	return out
}

func existingModelSections(snapshot map[string]any, sections []string) []string {
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		if value, ok := snapshot[section]; ok && !isEmptyValue(value) {
			out = append(out, section)
		}
	}
	return out
}

func modelLayerBytes(snapshot map[string]any, sections []string) int {
	total := 0
	for _, section := range sections {
		total += modelJSONSize(snapshot[section])
	}
	return total
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func modelJSONSize(value any) int {
	data, _ := json.Marshal(value)
	return len(data)
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

// ModelContextOverflow reports the explicit fail-closed overflow marker when
// the projected model snapshot still exceeds its layered budgets after
// deterministic degradation. The empty string means the snapshot is within
// budget; otherwise a deterministic one-line summary is returned. The runtime
// refuses to send an overflowing model request (fail closed).
func ModelContextOverflow(snapshotJSON string) string {
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return ""
	}
	overflow := modelMap(snapshot["context_overflow"])
	if len(overflow) == 0 {
		return ""
	}
	return fmt.Sprintf("status=%s hot=%d/%d warm=%d/%d cold_ref=%s",
		modelText(overflow["status"]),
		modelInteger(overflow["hot_bytes"]), modelInteger(overflow["hot_budget_bytes"]),
		modelInteger(overflow["warm_bytes"]), modelInteger(overflow["warm_budget_bytes"]),
		modelText(overflow["cold_ref"]))
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

// ProjectCCBViewConclusion emits a deterministic decision digest for a warm
// historical ledger. The canonical active observation keeps the complete safe
// model projection; this function deliberately does not copy it a second time.
// Decision facts are extracted from the flattened view facts first: the
// structured fields the model decides on (conflict candidates, coverage,
// rankings) live in the facts, not in the llm_context summary. llm_context is
// only a fallback for views without structured facts. The digest opts bound
// candidate/ranking lists so the ledger stays inside the warm budget.
func ProjectCCBViewConclusion(bundle map[string]any, viewID string, opts Options) map[string]any {
	view := modelMap(modelMap(bundle["views"])[viewID])
	if len(view) == 0 {
		return nil
	}
	opts = normalizeOptions(opts)
	projected := projectCCBView(viewID, view, opts)
	out := selectModelFields(projected, "projection_status")
	out["digest_kind"] = "decision_digest"
	digestOpts := opts
	digestOpts.MaxListItems = minModelInt(opts.MaxListItems, 4)
	source := equivalentCCBViewProjection(viewID, modelMap(view["facts"]), opts)
	if len(source) == 0 {
		if llmContext := modelMap(projected["llm_context"]); len(llmContext) > 0 {
			source = llmContext
		} else {
			source = modelMap(projected["view_projection"])
		}
	}
	if facts := projectCCBDecisionDigest(viewID, source, digestOpts); len(facts) > 0 {
		if viewID == "track.band_dynamics" && bundleHasUsableCCBView(bundle, "track.timbre_frequency") {
			delete(facts, "band_profile")
		}
		out["facts"] = facts
	} else {
		out["digest_status"] = "reference_only"
	}
	return modelMap(sanitizeModelProjection(out, opts))
}

func bundleHasUsableCCBView(bundle map[string]any, viewID string) bool {
	status := strings.ToLower(strings.TrimSpace(modelText(modelMap(modelMap(bundle["views"])[viewID])["status"])))
	return status == "ready" || status == "partial" || status == "stale" || status == "suspect" || status == "approximate"
}

func projectCCBDecisionDigest(viewID string, source map[string]any, opts Options) map[string]any {
	if len(source) == 0 {
		return nil
	}
	var out map[string]any
	switch viewID {
	case "project.structure":
		project := modelMap(source["project_summary"])
		tracks := modelMap(source["track_summary"])
		out = map[string]any{
			"status":             firstNonEmptyModelValue(project["status"], tracks["status"]),
			"track_count":        firstNonEmptyModelValue(tracks["track_count"], project["track_count"]),
			"active_track_count": firstNonEmptyModelValue(tracks["active_track_count"], project["active_track_count"]),
			"tracks":             projectDecisionTrackIndex(tracks["tracks"], opts),
		}
	case "track.basic_energy":
		out = map[string]any{"levels": projectDecisionMap(source["levels"], opts,
			"status", "rms_dbfs", "peak_dbfs", "headroom_db", "crest_db", "lufs")}
	case "track.time_dynamics":
		out = map[string]any{
			"time_energy": projectDecisionMap(source["time_energy"], opts, "status", "summary", "macro_dynamics", "rows"),
			"source_dynamics": projectDecisionMap(source["source_dynamics"], opts,
				"status", "summary", "macro_dynamics", "dynamic_range", "crest"),
		}
	case "track.timbre_frequency":
		bandEnergy := modelMap(source["band_energy"])
		out = map[string]any{"status": bandEnergy["status"], "band_profile": projectDecisionBandProfile(firstNonEmptyModelValue(bandEnergy["bands"], bandEnergy["band_energy"]), opts)}
	case "track.band_dynamics":
		dynamics := modelMap(source["band_dynamics"])
		out = map[string]any{
			"status": dynamics["status"], "band_profile": projectDecisionBandProfile(dynamics["bands"], opts),
			"time_varying_ready": dynamics["time_varying_ready"], "per_band_crest_ready": dynamics["per_band_crest_ready"],
			"cross_band_relation_ready": dynamics["cross_band_relation_ready"],
			"trust_quality": projectDecisionMap(modelMap(source["readiness"])["trust_quality"], opts,
				"overall_status", "can_support_source_description", "can_support_family_selection", "can_support_behavior_observation", "can_support_post_action_evaluation"),
		}
	case "track.frequency_time_events":
		events := modelMap(source["frequency_time_events"])
		out = map[string]any{
			"status": events["status"], "time_localized": events["time_localized"],
			"event_count_available":     events["event_count_available"],
			"whole_window_band_profile": projectDecisionBandProfile(events["whole_window_bands"], opts),
		}
	case "track.peak_structure":
		out = map[string]any{"peak_structure": projectDecisionMap(source["peak_structure"], opts,
			"status", "peak_dbfs", "headroom_db", "crest_db", "segment_peak_dbfs_distribution",
			"segment_crest_db_distribution", "at_or_above_full_scale_segment_count", "true_peak_status")}
	case "track.activity_structure":
		out = map[string]any{"activity_structure": projectDecisionMap(source["activity_structure"], opts,
			"status", "segment_count", "valid_segment_count", "active_segment_count", "low_energy_segment_count",
			"silent_segment_count", "unknown_segment_count", "coverage_ratio", "active_ratio", "low_or_silent_ratio",
			"low_or_silent_run_seconds_distribution", "noise_floor_status")}
	case "track.transient_structure":
		out = map[string]any{"transient_structure": projectDecisionMap(source["transient_structure"], opts,
			"status", "segment_crest_db_distribution", "onset_events_ready", "attack_body_contrast_ready", "sustain_decay_ready")}
	case "track.stereo_space":
		out = map[string]any{"stereo": projectDecisionMap(source["stereo"], opts,
			"status", "correlation_estimate", "width", "balance", "summary")}
	case "mix.multitrack_relationship":
		relation := modelMap(source["relationship"])
		rankingOpts := opts
		rankingOpts.MaxListItems = minModelInt(opts.MaxListItems, 3)
		out = map[string]any{
			"status":                   relation["status"],
			"track_count":              modelMap(source["relationship_inputs"])["track_count"],
			"loudness_order":           projectDecisionRows(relation["loudness_order"], rankingOpts, "track_id", "name", "rank", "value"),
			"peak_order":               projectDecisionRows(relation["peak_order"], rankingOpts, "track_id", "name", "rank", "value"),
			"headroom_risk_tracks":     projectDecisionRows(relation["headroom_risk_tracks"], rankingOpts, "track_id", "name", "headroom_db", "risk"),
			"band_conflict_candidates": projectDecisionCandidates(relation["band_conflict_candidates"], opts),
			"phase_risk_tracks":        projectDecisionRows(relation["phase_risk_tracks"], rankingOpts, "track_id", "name"),
		}
	case "mix.frequency_relationship":
		relation := modelMap(source["frequency_relationship"])
		inputs := modelMap(source["relationship_inputs"])
		out = map[string]any{
			"status": relation["status"], "tap_point": relation["tap_point"],
			"coverage": projectDecisionMap(relation["coverage"], opts,
				"conflict_candidate_count", "eligible_track_ratio", "frequency_region_count"),
			"conflict_candidates":   projectDecisionCandidates(relation["conflict_candidates"], opts),
			"interpretation_limits": projectDecisionInterpretationLimits(relation, opts),
			"relationship_inputs": projectDecisionMap(inputs, opts,
				"status", "track_count", "usable_track_count", "tap_points"),
		}
	case "mix.masking_relationship":
		relation := modelMap(source["masking_relationship"])
		out = map[string]any{
			"status":         relation["status"],
			"freshness":      relation["freshness"],
			"measurement_id": relation["measurement_id"],
			"model_version":  relation["model_version"],
			"candidate_only": relation["candidate_only"],
			"conditions":     projectDecisionMap(relation["conditions"], opts, "tap_point", "range_start_seconds", "range_end_seconds", "tail_seconds", "sample_rate", "analyzer_revision", "synchronized", "frame_count_per_track"),
			"coverage":       projectDecisionMap(relation["coverage"], opts, "track_count", "eligible_track_count", "candidate_count", "projected_candidate_count", "candidates_truncated", "complete_coverage"),
			"candidates":     projectMaskingCandidates(relation["candidates"], opts),
		}
	case "processor.behavior":
		out = map[string]any{"behavior": projectDecisionMap(source["behavior"], opts,
			"status", "mode", "gain_action", "transient_response", "recovery_motion", "level_effect", "stereo_behavior", "trigger_relation", "trust_quality")}
	case "processor.change_delta":
		out = map[string]any{"change": projectDecisionMap(source["change"], opts, "status", "mode", "behavior_change", "trust_quality")}
	case "comparison.before_after":
		out = map[string]any{"comparison": projectDecisionMap(source["comparison"], opts, "status", "summary", "changes")}
	default:
		out = projectDecisionMap(source, opts, "status", "summary", "readiness", "candidate_summary", "compact_facts")
	}
	out = modelMap(sanitizeModelProjection(projectDecisionValue(out, opts, 0), opts))
	removeEmptyModelFields(out)
	if !hasMeaningfulModelProjection(out) {
		return nil
	}
	return out
}

func projectDecisionMap(value any, opts Options, keys ...string) map[string]any {
	return modelMap(projectDecisionValue(selectModelFields(modelMap(value), keys...), opts, 0))
}

func projectDecisionRows(value any, opts Options, keys ...string) []any {
	rows := modelRows(value)
	limit := len(rows)
	if limit > opts.MaxListItems {
		limit = opts.MaxListItems
	}
	out := make([]any, 0, limit+1)
	for _, row := range rows[:limit] {
		if projected := projectDecisionMap(row, opts, keys...); len(projected) > 0 {
			out = append(out, projected)
		}
	}
	if len(rows) > limit {
		out = append(out, map[string]any{"omitted_items": len(rows) - limit})
	}
	return out
}

func projectDecisionTrackIndex(value any, opts Options) []any {
	return projectDecisionRows(value, opts, "track_id", "id", "name", "label", "role_guess", "kind", "status", "active", "muted", "solo")
}

func projectDecisionBands(value any, opts Options) []any {
	if bands := modelMap(value); len(bands) > 0 {
		out := make([]any, 0, len(bands))
		for _, band := range sortedModelKeys(bands) {
			row := projectDecisionMap(bands[band], opts, "id", "status", "energy_db", "unit_energy")
			if row["id"] == nil {
				row["id"] = band
			}
			out = append(out, row)
		}
		return out
	}
	return projectDecisionRows(value, opts, "id", "status", "energy_db", "unit_energy")
}

func projectDecisionBandProfile(value any, opts Options) map[string]any {
	bands := projectDecisionBands(value, opts)
	type rankedBand struct {
		index  int
		id     string
		energy float64
	}
	ranked := make([]rankedBand, 0, len(bands))
	for i, value := range bands {
		row := modelMap(value)
		energy, ok := modelFloat(row["energy_db"])
		if !ok {
			continue
		}
		ranked = append(ranked, rankedBand{index: i, id: modelText(row["id"]), energy: energy})
	}
	if len(ranked) == 0 {
		return nil
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].energy == ranked[j].energy {
			return ranked[i].id < ranked[j].id
		}
		return ranked[i].energy > ranked[j].energy
	})
	out := map[string]any{
		"strongest": bands[ranked[0].index],
		"weakest":   bands[ranked[len(ranked)-1].index],
		"spread_db": ranked[0].energy - ranked[len(ranked)-1].energy,
	}
	if len(ranked) > 2 {
		out["second_strongest"] = bands[ranked[1].index]
	}
	return out
}

func projectDecisionCandidates(value any, opts Options) []any {
	rows := modelRows(value)
	if len(rows) > opts.MaxListItems {
		rows = rows[:opts.MaxListItems]
	}
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		candidate := projectDecisionMap(row, opts, "type", "status", "region", "band", "confidence")
		if tracks := projectDecisionRows(row["tracks"], opts, "track_id"); len(tracks) > 0 {
			candidate["tracks"] = tracks
		}
		if limit := modelText(row["interpretation_limit"]); limit != "" {
			candidate["interpretation_limit"] = compactText(limit, minModelInt(opts.MaxTextRunes, 240))
		}
		out = append(out, candidate)
	}
	return out
}

// projectMaskingCandidates keeps the directional pair/band evidence needed
// for an improvement hypothesis while excluding raw frames and any execution
// authority. The hot and warm projections share this bounded shape.
func projectMaskingCandidates(value any, opts Options) []any {
	rows := modelRows(value)
	// Four ranked rows are enough to give the model a concrete directional
	// pair/band hypothesis while keeping the active CCB projection below the
	// strict 10 KiB hot-layer budget when mixed with structure/relationship
	// views. The MOM coverage still reports the full candidate count.
	limit := minModelInt(opts.MaxListItems, 4)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]any, 0, len(rows)+1)
	for _, row := range rows {
		candidate := projectDecisionMap(row, opts,
			"masker_track_id", "masker_track_name", "target_track_id", "target_track_name", "band_id",
			"min_hz", "max_hz", "active_frame_count", "risk_frame_count", "risk_coverage_ratio",
			"median_margin_db", "p90_margin_db", "max_margin_db")
		if len(candidate) > 0 {
			out = append(out, candidate)
		}
	}
	if len(modelRows(value)) > limit {
		out = append(out, map[string]any{"omitted_items": len(modelRows(value)) - limit})
	}
	return out
}

func projectDecisionInterpretationLimits(relation map[string]any, opts Options) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, key := range []string{"conflict_candidates", "tonal_tendencies"} {
		for _, row := range modelRows(relation[key]) {
			limit := compactText(modelText(row["interpretation_limit"]), minModelInt(opts.MaxTextRunes, 240))
			if limit != "" && !seen[limit] {
				seen[limit] = true
				out = append(out, limit)
			}
		}
	}
	return out
}

func projectDecisionValue(value any, opts Options, depth int) any {
	if depth > 5 {
		return map[string]any{"omitted_depth": true}
	}
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for _, key := range sortedModelKeys(typed) {
			if forbiddenModelProjectionKey(key) || key == "evidence_refs" || key == "limitations" || key == "scope" || strings.Contains(key, "source_path") {
				continue
			}
			out[key] = projectDecisionValue(typed[key], opts, depth+1)
		}
		return removeEmptyModelFields(out)
	case []any:
		limit := len(typed)
		if limit > opts.MaxListItems {
			limit = opts.MaxListItems
		}
		out := make([]any, 0, limit+1)
		for _, item := range typed[:limit] {
			out = append(out, projectDecisionValue(item, opts, depth+1))
		}
		if len(typed) > limit {
			out = append(out, map[string]any{"omitted_items": len(typed) - limit})
		}
		return out
	case []map[string]any:
		rows := make([]any, len(typed))
		for i := range typed {
			rows[i] = typed[i]
		}
		return projectDecisionValue(rows, opts, depth)
	case string:
		return compactText(typed, minModelInt(opts.MaxTextRunes, 240))
	default:
		return typed
	}
}

func firstNonEmptyModelValue(values ...any) any {
	for _, value := range values {
		if !isEmptyValue(value) {
			return value
		}
	}
	return nil
}

func modelFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func minModelInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	bundleEvidenceRefs := compactModelEvidenceRefs(bundle["evidence_refs"], opts)
	for _, viewID := range requested {
		// The canonical active observation is the Hot layer. Each view is
		// materialized as a short digest (summary + evidence refs), never the
		// full LLMContext compact facts or raw conditions/revisions.
		projected := projectCCBViewDigest(viewID, modelMap(views[viewID]), opts)
		if projected["evidence_refs"] == nil && !isEmptyValue(bundleEvidenceRefs) {
			projected["evidence_refs"] = bundleEvidenceRefs
		}
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
		"receipt_id":            modelText(audit["receipt_id"]),
		"receipt_schema":        modelText(audit["schema_version"]),
		"bundle_id":             modelText(bundle["bundle_id"]),
		"view_set_matches":      audit["view_set_matches"],
		"rejection_scope":       audit["rejection_scope"],
		"blocking_view_ids":     audit["blocking_view_ids"],
		"non_blocking_view_ids": audit["non_blocking_view_ids"],
	})
	return removeEmptyModelFields(map[string]any{
		"schema_version":        CCBModelProjectionSchema,
		"kind":                  "observation",
		"projection_status":     bundleProjectionStatus,
		"tool_call_id":          strings.TrimSpace(result.ToolCallID),
		"tool":                  strings.TrimSpace(result.Tool),
		"status":                firstModelText(result.Status, modelText(bundle["status"])),
		"bundle_status":         modelText(bundle["status"]),
		"observation_id":        modelText(bundle["observation_id"]),
		"bundle_id":             modelText(bundle["bundle_id"]),
		"request_id":            modelText(bundle["request_id"]),
		"read_only":             bundle["read_only"],
		"mutation_authority":    bundle["mutation_authority"],
		"target_ref":            sanitizeModelProjection(bundle["target_ref"], opts),
		"freshness":             sanitizeModelProjection(bundle["freshness"], opts),
		"requested_views":       requested,
		"views":                 projectedViews,
		"limitations":           sanitizeModelProjection(bundle["limitations"], opts),
		"omission_reasons":      sanitizeModelProjection(bundle["omission_reasons"], opts),
		"omissions":             sanitizeModelProjection(bundle["omissions"], opts),
		"evidence_refs":         compactModelEvidenceRefs(bundle["evidence_refs"], opts),
		"audit_ref":             auditRef,
		"rejection_scope":       modelText(bundle["rejection_scope"]),
		"blocking_view_ids":     sanitizeModelProjection(bundle["blocking_view_ids"], opts),
		"non_blocking_view_ids": sanitizeModelProjection(bundle["non_blocking_view_ids"], opts),
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

// projectCCBViewDigest emits the Hot-layer digest for one view of the
// canonical active observation: one-line summary + evidence refs only. Full
// decision facts stay in the Warm ledger conclusion and the Cold audit
// snapshot; measurement_key, source/clip revisions, and compact fact arrays
// are never inlined here. Views without an LLMContext keep their bounded
// field-selected fallback (DAD-derived views are already compact).
func projectCCBViewDigest(viewID string, view map[string]any, opts Options) map[string]any {
	status := modelText(view["status"])
	out := removeEmptyModelFields(map[string]any{
		"view_id":     firstModelText(modelText(view["view_id"]), viewID),
		"status":      status,
		"limitations": sanitizeModelProjection(view["limitations"], opts),
	})
	// Masking is already a compact MOM candidate projection. Keep its bounded
	// directional rows in the hot layer so the model can form an improvement
	// hypothesis without rehydrating the cold audit package.
	if viewID == "mix.masking_relationship" {
		if equivalent := equivalentCCBViewProjection(viewID, modelMap(view["facts"]), opts); len(equivalent) > 0 {
			out["projection_status"] = projectionStatus(status)
			out["view_projection"] = equivalent
			return out
		}
	}
	if llmContext := findViewLLMContext(view); len(llmContext) > 0 {
		out["projection_status"] = projectionStatus(status)
		summary := firstNonEmptyModelText(modelText(llmContext["summary_md"]), modelText(llmContext["summary"]))
		if summary != "" {
			// One-line digest: keep it short so a multi-view observation stays
			// inside the hot budget (full facts live in the warm conclusion and
			// cold audit snapshot).
			out["digest"] = compactText(summary, minModelInt(opts.MaxTextRunes, 120))
		}
		if refs := firstNonNilModelValue(llmContext["evidence_refs"], view["evidence_refs"]); !isEmptyValue(refs) {
			out["evidence_refs"] = compactModelEvidenceRefs(refs, opts)
		}
		// A structure digest must expose the minimal track identity set the
		// model needs to request track-targeted observations; without it the
		// model cannot proceed past project-level views (observed no-op loop).
		if viewID == "project.structure" {
			if tracks := digestProjectTrackIdentities(view, opts); len(tracks) > 0 {
				out["tracks"] = tracks
			}
		}
		return out
	}
	if equivalent := equivalentCCBViewProjection(viewID, modelMap(view["facts"]), opts); len(equivalent) > 0 {
		out["projection_status"] = projectionStatus(status)
		// Defensive bound for views without an LLMContext: a field-selected
		// fallback can still be large (e.g. a ready multi-track relationship
		// with per-track band profiles). The hot digest must never inline it;
		// oversized fallbacks become a resolvable cold reference instead.
		if modelJSONSize(equivalent) > modelDigestFallbackMaxBytes {
			out["view_ref"] = "audit_snapshot://ccb_view/" + viewID
		} else {
			out["view_projection"] = equivalent
		}
		return out
	}
	if status == "missing" || status == "unavailable" || status == "deferred" {
		out["projection_status"] = "omitted"
	} else {
		out["projection_status"] = "unsupported_projection"
	}
	return out
}

func firstNonNilModelValue(values ...any) any {
	for _, value := range values {
		if !isEmptyValue(value) {
			return value
		}
	}
	return nil
}

// digestProjectTrackIdentities extracts a compact track identity table
// (track_id + track_name) from the structure view facts so the model can
// request track-targeted observations. It reads the same project.tracks.summary
// facts the audit view exposes, projected through the same sanitizer.
func digestProjectTrackIdentities(view map[string]any, opts Options) []map[string]any {
	facts := modelMap(view["facts"])
	tracks := modelRows(modelMap(facts["project.tracks.summary"])["tracks"])
	if len(tracks) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(tracks))
	seen := map[string]bool{}
	for _, row := range tracks {
		trackID := modelText(row["track_id"])
		if trackID == "" {
			trackID = modelText(row["id"])
		}
		if trackID == "" || seen[trackID] {
			continue
		}
		seen[trackID] = true
		item := removeEmptyModelFields(map[string]any{
			"track_id":   trackID,
			"track_name": firstNonEmptyModelText(modelText(row["track_name"]), modelText(row["name"]), trackID),
			"role_guess": modelText(row["role_guess"]),
		})
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmptyModelText(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
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
		dimensionFields := map[string][]string{
			"peak_structure": {
				"status", "peak_dbfs", "headroom_db", "crest_db", "segment_peak_dbfs_distribution",
				"segment_crest_db_distribution", "at_or_above_full_scale_segment_count", "true_peak_status",
				"limitations", "evidence_refs",
			},
			"activity_structure": {
				"status", "segment_count", "valid_segment_count", "active_segment_count", "low_energy_segment_count",
				"silent_segment_count", "unknown_segment_count", "coverage_ratio", "active_ratio", "low_or_silent_ratio",
				"low_or_silent_run_seconds_distribution", "noise_floor_status", "limitations", "evidence_refs",
			},
			"frequency_time_events": {
				"status", "whole_window_bands", "time_localized", "event_count_available", "limitations", "evidence_refs",
			},
			"transient_structure": {
				"status", "segment_crest_db_distribution", "onset_events_ready", "attack_body_contrast_ready",
				"sustain_decay_ready", "limitations", "evidence_refs",
			},
			"band_dynamics": {
				"status", "bands", "time_varying_ready", "per_band_crest_ready", "cross_band_relation_ready",
				"limitations", "evidence_refs",
			},
		}[dimension]
		out = map[string]any{
			dimension:   modelNestedFactFields(facts, "observation.dom_projection", dimension, opts, dimensionFields...),
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
	case "mix.masking_relationship":
		out = map[string]any{
			"masking_relationship": compactMaskingRelationshipForModel(
				modelNestedFactValue(facts, "observation.mom_projection", "masking_relationship"), opts),
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

func modelNestedFactValue(facts map[string]any, factKey, nested string) any {
	return modelMap(modelMap(facts[factKey])[nested])
}

// compactMaskingRelationshipForModel is intentionally narrower than the MOM
// object. It is the model-facing contract for the real observation: concrete
// directional candidates, bounded conditions/coverage, and traceable refs;
// no raw frames, levels, or mutation fields cross this boundary.
func compactMaskingRelationshipForModel(value any, opts Options) map[string]any {
	relation := modelMap(value)
	if len(relation) == 0 {
		return nil
	}
	out := selectModelFields(relation, "schema_version", "status", "freshness", "measurement_id", "model_version", "candidate_only", "limitations")
	out["evidence_refs"] = compactModelEvidenceRefs(relation["evidence_refs"], opts)
	out["conditions"] = selectModelFields(modelMap(relation["conditions"]),
		"tap_point", "range_start_seconds", "range_end_seconds", "tail_seconds", "sample_rate", "analyzer_revision", "synchronized", "frame_count_per_track")
	out["coverage"] = selectModelFields(modelMap(relation["coverage"]),
		"track_count", "eligible_track_count", "candidate_count", "projected_candidate_count", "candidates_truncated", "complete_coverage")
	out["candidates"] = projectMaskingCandidates(relation["candidates"], opts)
	return modelMap(sanitizeModelProjection(out, opts))
}

// compactModelEvidenceRefs retains short, directly useful references but turns
// long transport identities into stable opaque handles. L2 probe references
// can include source paths and full clip manifests, which belong to the cold
// audit receipt rather than the model's hot decision context.
func compactModelEvidenceRefs(value any, opts Options) []any {
	refs := modelStrings(value)
	if len(refs) == 0 {
		return nil
	}
	limit := minModelInt(opts.MaxListItems, 5)
	if limit <= 0 {
		limit = 5
	}
	if len(refs) < limit {
		limit = len(refs)
	}
	out := make([]any, 0, limit+1)
	for _, ref := range refs[:limit] {
		if len([]rune(ref)) <= 160 {
			out = append(out, ref)
			continue
		}
		digest := sha256.Sum256([]byte(ref))
		out = append(out, "evidence_ref_hash:"+hex.EncodeToString(digest[:6]))
	}
	if len(refs) > limit {
		out = append(out, map[string]any{"omitted_refs": len(refs) - limit})
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
	// The canonical copy of every non-CCB tool result lives once in
	// tool_result_summary (CCB rows are removed because their canonical
	// projection is the active observation). Every other model section keeps
	// only a reference so the same data never appears twice in one request.
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
			if result := modelMap(event["tool_result"]); len(result) > 0 {
				delete(event, "tool_result")
				event["tool_result_ref"] = referenceForSummary(result, refsByCallID, refsByTool)
			} else if ref := modelMap(event["tool_result_ref"]); len(ref) > 0 {
				// A ref-only trace event is enriched with the canonical CCB
				// reference when the projection knows the observation.
				event["tool_result_ref"] = referenceForSummary(ref, refsByCallID, refsByTool)
			}
		}
		trace["recent_events"] = events
	}
	semantic := modelMap(snapshot["daw_semantic_summary"])
	if result := modelMap(semantic["recent_execution_result"]); len(result) > 0 {
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
	if result := modelMap(context["recent_tool_result"]); len(result) > 0 {
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
