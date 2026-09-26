package tim

import (
	"fmt"
	"sort"
)

// Structural assertion layer v1 (docs/TIM_ASSERTER_V1_DESIGN.md).
//
// Hard rules encoded here:
//   - Referees do not play: asserters are pure predicates with no write path.
//     Output is only AssertionResult rows (and optional WARN lines through the
//     injected logger hook); corrections always go through the agent loop.
//   - Three states, not two: missing evidence is not_evaluable and is never
//     silently recorded as pass.
//   - v1 failure action is warn only (block semantics are a separate v2 decision).

const (
	AssertionStatusPass         = "pass"
	AssertionStatusFail         = "fail"
	AssertionStatusNotEvaluable = "not_evaluable"
)

// Defaults centralizes every assertion threshold (design §3.1).
const (
	clipCeilingDBFS  = -0.1
	headroomFloorDB  = 0.1
	defaultCeilingDB = -1.0
	maxWarnLines     = 20
)

// Assertion fail / not_evaluable codes (design §2). P2 clipping_headroom
// deliberately reuses the existing possible_clipping_or_no_headroom issue code
// instead of minting a second code for the same problem.
const (
	codeSignalNonfinite       = "assert_signal_nonfinite"
	codeClippingHeadroom      = "possible_clipping_or_no_headroom"
	codeLevelCeilingExceeded  = "assert_level_ceiling_exceeded"
	codeSampleRateMismatch    = "assert_sample_rate_mismatch"
	codeRoutingDeadEnd        = "assert_routing_dead_end"
	codeRoutingCycle          = "assert_routing_cycle"
	codePluginUnknownPath     = "assert_plugin_unknown_path"
	codeSignalNonfiniteNE     = "assert_signal_nonfinite_not_evaluable"
	codeClippingHeadroomNE    = "assert_clipping_headroom_not_evaluable"
	codeLevelCeilingNE        = "assert_level_ceiling_not_evaluable"
	codeSampleRateNE          = "assert_sample_rate_not_evaluable"
	codeRoutingNE             = "assert_routing_not_evaluable"
	codePluginLegalityNE      = "assert_plugin_legality_not_evaluable"
	codeUnknownEntitySkipped  = "assert_unknown_entity_skipped"
	codeCeilingSamplePeakOnly = "assert_level_ceiling_sample_peak_only"
)

// knownAsserterChecks is the v1 entity registry; anything outside it fails
// closed at load time (ReconcileLoadedAssertions).
var knownAsserterChecks = map[string]map[string]bool{
	"signal_hygiene":  {"signal_nonfinite": true, "clipping_headroom": true},
	"level_ceiling":   {"ceiling": true},
	"sample_rate":     {"consistency": true},
	"routing":         {"dead_end": true, "cycle": true},
	"plugin_legality": {"known_path": true},
}

// AssertionResult is one asserter verdict for one check scope (design §3.1
// schema). Status is pass/fail/not_evaluable; Code carries the fail code on
// fail rows and the not_evaluable limitation code on not_evaluable rows.
type AssertionResult struct {
	Asserter     string   `json:"asserter"`
	Check        string   `json:"check"`
	Status       string   `json:"status"`
	TrackID      string   `json:"track_id,omitempty"`
	Value        *float64 `json:"value,omitempty"`
	Threshold    *float64 `json:"threshold,omitempty"`
	Code         string   `json:"code,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// AssertInput is the complete, read-only input for the assertion pass.
type AssertInput struct {
	TrackFacts          []TrackFact
	ProjectSampleRateHz *float64
	RackSummaries       []RackSummary
	KnownPluginPaths    map[string]bool
	CeilingDBFS         float64
}

// RackNode is the compact per-node routing fact consumed by the routing and
// plugin legality asserters.
type RackNode struct {
	NodeID                      string
	Enabled                     bool
	AudioReachableFromRackInput bool
	VitOrphanBypassCandidate    bool
	PluginPath                  string
	PluginFormat                string
}

// RackEdge is one directed rack connection; RACK_INPUT/RACK_OUTPUT are the
// sentinel endpoints used by the kernel rack state.
type RackEdge struct {
	SourceID string
	DestID   string
}

// RackSummary is the per-track routing digest assembled agent-side from the
// project state rack blocks.
type RackSummary struct {
	TrackID string
	Nodes   []RackNode
	Edges   []RackEdge
}

// AssertWarnLogger, when set by the host process, receives one formatted
// "[tim.assert] ..." line per fail row (capped per asserter). Wiring it to the
// agent logx logger is host-side work; nil keeps Build silent and pure.
var AssertWarnLogger func(line string)

var acousticAssertionRefs = []string{"mix.read:project.tracks.summary", "mix.read:project.acoustic.tracks"}
var rackAssertionRefs = []string{"mix.read:project.tracks.summary"}

// Evaluate runs every v1 asserter (AS-SIG / AS-PEAK / AS-SR / AS-ROUTE /
// AS-PLUGIN) and returns one row per (check, scope). Pure: same input, same
// output; the only side effect is optional WARN lines via AssertWarnLogger.
func Evaluate(input AssertInput) []AssertionResult {
	results := []AssertionResult{}
	results = append(results, evaluateSignalHygiene(input.TrackFacts)...)
	results = append(results, evaluateLevelCeiling(input.TrackFacts, input.CeilingDBFS)...)
	results = append(results, evaluateSampleRate(input.TrackFacts, input.ProjectSampleRateHz)...)
	results = append(results, evaluateRouting(input.RackSummaries)...)
	results = append(results, evaluatePluginLegality(input.RackSummaries, input.KnownPluginPaths)...)
	warnAssertionFailures(results)
	return results
}

func evaluateSignalHygiene(facts []TrackFact) []AssertionResult {
	out := []AssertionResult{}
	for _, fact := range facts {
		// P1 nonfinite: nan_count + inf_count must be zero.
		if fact.NanCount == nil && fact.InfCount == nil {
			out = append(out, assertionRow("signal_hygiene", "signal_nonfinite", AssertionStatusNotEvaluable, fact.TrackID, codeSignalNonfiniteNE, nil, nil, acousticAssertionRefs))
		} else {
			total := 0
			if fact.NanCount != nil {
				total += *fact.NanCount
			}
			if fact.InfCount != nil {
				total += *fact.InfCount
			}
			if total == 0 {
				out = append(out, passRow("signal_hygiene", "signal_nonfinite", fact.TrackID, floatPtr(0), floatPtr(0), acousticAssertionRefs))
			} else {
				out = append(out, failRow("signal_hygiene", "signal_nonfinite", fact.TrackID, codeSignalNonfinite, floatPtr(float64(total)), floatPtr(0), acousticAssertionRefs))
			}
		}
		// P2 clipping_headroom: peak below the clip ceiling and headroom above
		// the floor. Same semantics as the existing issue code.
		peak, headroom := fact.PeakDBFS, fact.HeadroomDB
		if peak == nil && headroom == nil {
			out = append(out, assertionRow("signal_hygiene", "clipping_headroom", AssertionStatusNotEvaluable, fact.TrackID, codeClippingHeadroomNE, nil, nil, acousticAssertionRefs))
			continue
		}
		failing := false
		value, threshold := floatPtr(0), floatPtr(0)
		if peak != nil {
			value, threshold = floatPtr(*peak), floatPtr(clipCeilingDBFS)
			if *peak >= clipCeilingDBFS {
				failing = true
			}
		}
		if headroom != nil {
			if *headroom <= headroomFloorDB {
				failing = true
				if peak == nil {
					value, threshold = floatPtr(*headroom), floatPtr(headroomFloorDB)
				}
			}
		}
		if failing {
			out = append(out, failRow("signal_hygiene", "clipping_headroom", fact.TrackID, codeClippingHeadroom, value, threshold, acousticAssertionRefs))
		} else {
			out = append(out, passRow("signal_hygiene", "clipping_headroom", fact.TrackID, value, threshold, acousticAssertionRefs))
		}
	}
	return out
}

func evaluateLevelCeiling(facts []TrackFact, ceiling float64) []AssertionResult {
	out := []AssertionResult{}
	for _, fact := range facts {
		if fact.PeakDBFS == nil {
			out = append(out, assertionRow("level_ceiling", "ceiling", AssertionStatusNotEvaluable, fact.TrackID, codeLevelCeilingNE, nil, nil, acousticAssertionRefs))
			continue
		}
		// Single-sided semantics: sample peak above the ceiling proves true
		// peak exceeds it; a passing sample peak does not prove compliance
		// (standing limitation, never a pass-only guarantee).
		if *fact.PeakDBFS > ceiling {
			out = append(out, failRow("level_ceiling", "ceiling", fact.TrackID, codeLevelCeilingExceeded, floatPtr(*fact.PeakDBFS), floatPtr(ceiling), acousticAssertionRefs))
		} else {
			out = append(out, passRow("level_ceiling", "ceiling", fact.TrackID, floatPtr(*fact.PeakDBFS), floatPtr(ceiling), acousticAssertionRefs))
		}
	}
	return out
}

func evaluateSampleRate(facts []TrackFact, projectHz *float64) []AssertionResult {
	out := []AssertionResult{}
	if projectHz == nil {
		// Project settings missing: the whole asserter stays not_evaluable.
		out = append(out, assertionRow("sample_rate", "consistency", AssertionStatusNotEvaluable, "", codeSampleRateNE, nil, nil, acousticAssertionRefs))
		return out
	}
	for _, fact := range facts {
		if fact.SampleRateHz <= 0 {
			out = append(out, assertionRow("sample_rate", "consistency", AssertionStatusNotEvaluable, fact.TrackID, codeSampleRateNE, nil, nil, acousticAssertionRefs))
			continue
		}
		if fact.SampleRateHz != *projectHz {
			out = append(out, failRow("sample_rate", "consistency", fact.TrackID, codeSampleRateMismatch, floatPtr(fact.SampleRateHz), floatPtr(*projectHz), acousticAssertionRefs))
		} else {
			out = append(out, passRow("sample_rate", "consistency", fact.TrackID, floatPtr(fact.SampleRateHz), floatPtr(*projectHz), acousticAssertionRefs))
		}
	}
	return out
}

func evaluateRouting(racks []RackSummary) []AssertionResult {
	out := []AssertionResult{}
	if len(racks) == 0 {
		out = append(out, assertionRow("routing", "dead_end", AssertionStatusNotEvaluable, "", codeRoutingNE, nil, nil, rackAssertionRefs))
		out = append(out, assertionRow("routing", "cycle", AssertionStatusNotEvaluable, "", codeRoutingNE, nil, nil, rackAssertionRefs))
		return out
	}
	for _, rack := range racks {
		deadEnds := 0
		for _, node := range rack.Nodes {
			if node.Enabled && !node.AudioReachableFromRackInput {
				deadEnds++
			}
		}
		if deadEnds > 0 {
			out = append(out, failRow("routing", "dead_end", rack.TrackID, codeRoutingDeadEnd, floatPtr(float64(deadEnds)), floatPtr(0), rackAssertionRefs))
		} else {
			out = append(out, passRow("routing", "dead_end", rack.TrackID, floatPtr(0), floatPtr(0), rackAssertionRefs))
		}
	}
	if routingGraphHasCycle(racks) {
		out = append(out, failRow("routing", "cycle", "", codeRoutingCycle, floatPtr(1), floatPtr(0), rackAssertionRefs))
	} else {
		out = append(out, passRow("routing", "cycle", "", floatPtr(0), floatPtr(0), rackAssertionRefs))
	}
	return out
}

// routingGraphHasCycle DFS-walks the union digraph of every rack's edges.
// The kernel rejects cycles at write time, so a runtime cycle means corrupted
// state — exactly what a compile-level assertion is for.
func routingGraphHasCycle(racks []RackSummary) bool {
	adjacency := map[string][]string{}
	for _, rack := range racks {
		for _, edge := range rack.Edges {
			adjacency[edge.SourceID] = append(adjacency[edge.SourceID], edge.DestID)
		}
	}
	nodes := make([]string, 0, len(adjacency))
	for node := range adjacency {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var visit func(node string) bool
	visit = func(node string) bool {
		color[node] = gray
		for _, next := range adjacency[node] {
			switch color[next] {
			case gray:
				return true
			case white:
				if visit(next) {
					return true
				}
			}
		}
		color[node] = black
		return false
	}
	for _, node := range nodes {
		if color[node] == white && visit(node) {
			return true
		}
	}
	return false
}

func evaluatePluginLegality(racks []RackSummary, known map[string]bool) []AssertionResult {
	out := []AssertionResult{}
	for _, rack := range racks {
		pluginNodes, unknown, emptyPath := 0, 0, 0
		for _, node := range rack.Nodes {
			isPluginNode := node.PluginPath != "" || node.PluginFormat != ""
			if !isPluginNode {
				continue
			}
			pluginNodes++
			switch {
			case node.PluginPath == "":
				emptyPath++
			case known == nil:
				// No known-plugin table at all: honest not_evaluable, never a
				// fabricated fail.
				emptyPath++
			case !known[node.PluginPath]:
				unknown++
			}
		}
		switch {
		case known == nil && pluginNodes > 0:
			out = append(out, assertionRow("plugin_legality", "known_path", AssertionStatusNotEvaluable, rack.TrackID, codePluginLegalityNE, floatPtr(float64(pluginNodes)), nil, rackAssertionRefs))
		case unknown > 0:
			out = append(out, failRow("plugin_legality", "known_path", rack.TrackID, codePluginUnknownPath, floatPtr(float64(unknown)), floatPtr(0), rackAssertionRefs))
		case emptyPath > 0:
			out = append(out, assertionRow("plugin_legality", "known_path", AssertionStatusNotEvaluable, rack.TrackID, codePluginLegalityNE, floatPtr(float64(emptyPath)), nil, rackAssertionRefs))
		default:
			out = append(out, passRow("plugin_legality", "known_path", rack.TrackID, floatPtr(0), floatPtr(0), rackAssertionRefs))
		}
	}
	if len(racks) == 0 {
		out = append(out, assertionRow("plugin_legality", "known_path", AssertionStatusNotEvaluable, "", codePluginLegalityNE, nil, nil, rackAssertionRefs))
	}
	return out
}

// RackSummariesFromProjectState extracts per-track rack digests from the raw
// project state (kernel get_project_state shape). Tracks without a rack block
// are simply absent from the result.
func RackSummariesFromProjectState(state map[string]any) []RackSummary {
	if len(state) == 0 {
		return nil
	}
	out := []RackSummary{}
	for _, row := range rowsFromAny(state["tracks"]) {
		rack := mapValue(row["rack"])
		if len(rack) == 0 {
			continue
		}
		summary := RackSummary{TrackID: firstNonEmptyText(row, "track_id", "id")}
		for _, rawNode := range rowsFromAny(rack["nodes"]) {
			node := RackNode{
				NodeID:       firstNonEmptyText(rawNode, "node_id", "plugin_item_id", "item_id", "id", "plugin_id"),
				PluginPath:   firstNonEmptyText(rawNode, "plugin_path", "path"),
				PluginFormat: firstNonEmptyText(rawNode, "plugin_format", "format"),
			}
			node.Enabled, _ = boolValue(rawNode["enabled"])
			node.AudioReachableFromRackInput, _ = boolValue(rawNode["audio_reachable_from_rack_input"])
			node.VitOrphanBypassCandidate, _ = boolValue(rawNode["vit_orphan_bypass_candidate"])
			if node.NodeID == "" && node.PluginPath == "" && node.PluginFormat == "" {
				continue
			}
			summary.Nodes = append(summary.Nodes, node)
		}
		for _, rawEdge := range rowsFromAny(rack["edges"]) {
			source := firstNonEmptyText(rawEdge, "source_id", "source")
			dest := firstNonEmptyText(rawEdge, "dest_id", "dest", "destination")
			if source == "" || dest == "" {
				continue
			}
			summary.Edges = append(summary.Edges, RackEdge{SourceID: source, DestID: dest})
		}
		out = append(out, summary)
	}
	return out
}

// ReconcileLoadedAssertions is the fail-closed gate for assertion rows loaded
// from persisted JSON: unknown asserter/check names are skipped and reported
// as a limitation instead of being evaluated or silently swallowed (design
// §3.2).
func ReconcileLoadedAssertions(rows []AssertionResult) (kept []AssertionResult, limitations []string) {
	kept = []AssertionResult{}
	limitations = []string{}
	for _, row := range rows {
		checks, known := knownAsserterChecks[row.Asserter]
		if !known || !checks[row.Check] {
			limitations = appendUniqueString(limitations, codeUnknownEntitySkipped)
			continue
		}
		kept = append(kept, row)
	}
	return kept, limitations
}

// AssertionCounts summarizes a result set by status for LLM-context facts.
func AssertionCounts(rows []AssertionResult) map[string]int {
	counts := map[string]int{AssertionStatusPass: 0, AssertionStatusFail: 0, AssertionStatusNotEvaluable: 0}
	for _, row := range rows {
		if _, ok := counts[row.Status]; ok {
			counts[row.Status]++
		}
	}
	return counts
}

// AssertionFailCodes returns the sorted distinct fail codes, capped.
func AssertionFailCodes(rows []AssertionResult, maxItems int) []string {
	codes := map[string]bool{}
	for _, row := range rows {
		if row.Status == AssertionStatusFail && row.Code != "" {
			codes[row.Code] = true
		}
	}
	out := make([]string, 0, len(codes))
	for code := range codes {
		out = append(out, code)
	}
	sort.Strings(out)
	if maxItems > 0 && len(out) > maxItems {
		out = out[:maxItems]
	}
	return out
}

func warnAssertionFailures(results []AssertionResult) {
	if AssertWarnLogger == nil {
		return
	}
	emitted := map[string]int{}
	for _, row := range results {
		if row.Status != AssertionStatusFail {
			continue
		}
		if emitted[row.Asserter] >= maxWarnLines {
			continue
		}
		emitted[row.Asserter]++
		AssertWarnLogger(fmt.Sprintf("[tim.assert] asserter=%s check=%s status=%s track=%s value=%s threshold=%s code=%s",
			row.Asserter, row.Check, row.Status, row.TrackID,
			formatOptionalFloat(row.Value), formatOptionalFloat(row.Threshold), row.Code))
	}
}

func assertionRow(asserter, check, status, trackID, code string, value, threshold *float64, refs []string) AssertionResult {
	return AssertionResult{
		Asserter:     asserter,
		Check:        check,
		Status:       status,
		TrackID:      trackID,
		Value:        value,
		Threshold:    threshold,
		Code:         code,
		EvidenceRefs: refs,
	}
}

func passRow(asserter, check, trackID string, value, threshold *float64, refs []string) AssertionResult {
	return assertionRow(asserter, check, AssertionStatusPass, trackID, "", value, threshold, refs)
}

func failRow(asserter, check, trackID, code string, value, threshold *float64, refs []string) AssertionResult {
	return assertionRow(asserter, check, AssertionStatusFail, trackID, code, value, threshold, refs)
}

func floatPtr(value float64) *float64 { return &value }

func formatOptionalFloat(value *float64) string {
	if value == nil {
		return "n/a"
	}
	return fmt.Sprintf("%g", *value)
}

// assertionIssueDetail renders the human-facing fail detail for an issue row.
func assertionIssueDetail(row AssertionResult) string {
	detail := fmt.Sprintf("Structural assertion %s/%s failed", row.Asserter, row.Check)
	if row.TrackID != "" {
		detail += " on track " + row.TrackID
	}
	if row.Value != nil || row.Threshold != nil {
		detail = fmt.Sprintf("%s (value=%s threshold=%s)", detail, formatOptionalFloat(row.Value), formatOptionalFloat(row.Threshold))
	}
	return detail + "."
}
