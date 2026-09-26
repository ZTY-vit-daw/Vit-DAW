package tim

import (
	"encoding/json"
	"testing"
)

// L2-1-TIM-1 red tests (design docs/TIM_ASSERTER_V1_DESIGN.md §4.1 T1-T9).
// Before the asserter implementation these fail: Build produces no
// assertions block and no assertion issue/limitation codes.

func assertionResultBy(t *testing.T, proj Projection, asserter, check, trackID string) AssertionResult {
	t.Helper()
	for _, row := range proj.Assertions {
		if row.Asserter == asserter && row.Check == check && row.TrackID == trackID {
			return row
		}
	}
	t.Fatalf("assertion %s/%s track=%s not found in %#v", asserter, check, trackID, proj.Assertions)
	return AssertionResult{}
}

func hasIssueCodeForTrack(issues []Issue, code, trackID string) int {
	count := 0
	for _, issue := range issues {
		if issue.Code == code && issue.TrackID == trackID {
			count++
		}
	}
	return count
}

func hasAssertionStatus(proj Projection, asserter, check, trackID, status string) bool {
	for _, row := range proj.Assertions {
		if row.Asserter == asserter && row.Check == check && row.TrackID == trackID && row.Status == status {
			return true
		}
	}
	return false
}

// T1 AS-SIG P1: nonfinite samples fail assert_signal_nonfinite and are
// registered in the assertions block plus the issue list.
func TestBuildAssertionSignalNonfiniteFail(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t1", "track_name": "Broken", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
			"acoustic":     map[string]any{"status": "ready", "nan_count": 2, "inf_count": 1, "peak_dbfs": -6.0},
		}},
	}})
	row := assertionResultBy(t, proj, "signal_hygiene", "signal_nonfinite", "t1")
	if row.Status != AssertionStatusFail {
		t.Fatalf("signal_nonfinite status = %s, want fail: %#v", row.Status, row)
	}
	if row.Code != "assert_signal_nonfinite" {
		t.Fatalf("signal_nonfinite code = %s", row.Code)
	}
	if row.Value == nil || *row.Value != 3 {
		t.Fatalf("signal_nonfinite value = %#v, want 3", row.Value)
	}
	if got := hasIssueCodeForTrack(proj.Issues, "assert_signal_nonfinite", "t1"); got != 1 {
		t.Fatalf("assert_signal_nonfinite issues = %d, want 1: %#v", got, proj.Issues)
	}
	if !hasString(proj.RiskSummary.PrimaryCodes, "assert_signal_nonfinite") {
		t.Fatalf("assertion fail missing from primary codes: %#v", proj.RiskSummary)
	}
	if proj.RiskSummary.BySeverity[SeverityWarning] < 1 {
		t.Fatalf("assertion fail did not count as warning risk: %#v", proj.RiskSummary)
	}
}

// T2 AS-SIG P2: near-full-scale peak keeps the existing
// possible_clipping_or_no_headroom issue code (no duplicate) and the
// assertions block registers signal_hygiene/clipping_headroom=fail.
func TestBuildAssertionClippingHeadroomRegistersExistingCode(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t1", "track_name": "Hot", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/hot.wav"},
			"acoustic":     map[string]any{"status": "ready", "peak_dbfs": -0.05},
		}},
	}})
	row := assertionResultBy(t, proj, "signal_hygiene", "clipping_headroom", "t1")
	if row.Status != AssertionStatusFail {
		t.Fatalf("clipping_headroom status = %s, want fail: %#v", row.Status, row)
	}
	if row.Code != "possible_clipping_or_no_headroom" {
		t.Fatalf("clipping_headroom code = %s, want existing possible_clipping_or_no_headroom", row.Code)
	}
	if got := hasIssueCodeForTrack(proj.Issues, "possible_clipping_or_no_headroom", "t1"); got != 1 {
		t.Fatalf("possible_clipping_or_no_headroom issues = %d, want exactly 1 (no double code): %#v", got, proj.Issues)
	}
}

// T3 AS-PEAK: sample peak above the default -1.0 dBFS ceiling fails
// assert_level_ceiling_exceeded; the single-sided semantics stay a standing
// limitation.
func TestBuildAssertionLevelCeilingExceeded(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t1", "track_name": "Loud", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/loud.wav"},
			"acoustic":     map[string]any{"status": "ready", "peak_dbfs": -0.5},
		}},
	}})
	row := assertionResultBy(t, proj, "level_ceiling", "ceiling", "t1")
	if row.Status != AssertionStatusFail {
		t.Fatalf("level_ceiling status = %s, want fail: %#v", row.Status, row)
	}
	if row.Code != "assert_level_ceiling_exceeded" {
		t.Fatalf("level_ceiling code = %s", row.Code)
	}
	if row.Threshold == nil || *row.Threshold != -1.0 {
		t.Fatalf("level_ceiling threshold = %#v, want default -1.0", row.Threshold)
	}
	if got := hasIssueCodeForTrack(proj.Issues, "assert_level_ceiling_exceeded", "t1"); got != 1 {
		t.Fatalf("assert_level_ceiling_exceeded issues = %d: %#v", got, proj.Issues)
	}
	if !hasLimitation(proj.Limitations, "assert_level_ceiling_sample_peak_only") {
		t.Fatalf("standing sample-peak-only limitation missing: %#v", proj.Limitations)
	}

	clean := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t2", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c2", "current_source_path": "/a/ok.wav"},
			"acoustic":     map[string]any{"status": "ready", "peak_dbfs": -3.0, "nan_count": 0, "inf_count": 0, "headroom_db": 3.0},
		}},
	}})
	if !hasAssertionStatus(clean, "level_ceiling", "ceiling", "t2", AssertionStatusPass) {
		t.Fatalf("compliant peak should pass: %#v", clean.Assertions)
	}
}

// T4 AS-SR: source sample rate different from project audio settings fails
// assert_sample_rate_mismatch; missing project settings keep the asserter
// honest as not_evaluable instead of pass.
func TestBuildAssertionSampleRateMismatchAndNotEvaluable(t *testing.T) {
	proj := Build(Input{
		AudioSettings: map[string]any{"sample_rate_hz": 48000},
		ProjectPackage: map[string]any{
			"track_count": 1,
			"tracks": []any{map[string]any{
				"track_id": "t1", "clip_count": 1,
				"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/44.wav", "sample_rate_hz": 44100},
				"acoustic":     map[string]any{"status": "ready"},
			}},
		},
	})
	row := assertionResultBy(t, proj, "sample_rate", "consistency", "t1")
	if row.Status != AssertionStatusFail {
		t.Fatalf("sample_rate status = %s, want fail: %#v", row.Status, row)
	}
	if row.Code != "assert_sample_rate_mismatch" {
		t.Fatalf("sample_rate code = %s", row.Code)
	}
	if row.Value == nil || *row.Value != 44100 || row.Threshold == nil || *row.Threshold != 48000 {
		t.Fatalf("sample_rate value/threshold = %#v/%#v", row.Value, row.Threshold)
	}
	if got := hasIssueCodeForTrack(proj.Issues, "assert_sample_rate_mismatch", "t1"); got != 1 {
		t.Fatalf("assert_sample_rate_mismatch issues = %d: %#v", got, proj.Issues)
	}

	noSettings := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t1", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/44.wav", "sample_rate_hz": 44100},
			"acoustic":     map[string]any{"status": "ready"},
		}},
	}})
	if !hasAssertionStatus(noSettings, "sample_rate", "consistency", "", AssertionStatusNotEvaluable) {
		t.Fatalf("missing project settings should be not_evaluable: %#v", noSettings.Assertions)
	}
	if !hasLimitation(noSettings.Limitations, "assert_sample_rate_not_evaluable") {
		t.Fatalf("assert_sample_rate_not_evaluable limitation missing: %#v", noSettings.Limitations)
	}
	for _, row := range noSettings.Assertions {
		if row.Asserter == "sample_rate" && row.Status == AssertionStatusPass {
			t.Fatalf("missing settings must not produce pass: %#v", row)
		}
	}
}

// T5 AS-ROUTE P1: an enabled node that audio cannot reach from the rack
// input is a dead end.
func TestBuildAssertionRoutingDeadEnd(t *testing.T) {
	state := map[string]any{
		"tracks": []any{map[string]any{
			"track_id": "t1",
			"rack": map[string]any{
				"nodes": []any{map[string]any{
					"node_id": "n1", "enabled": true, "audio_reachable_from_rack_input": false,
				}},
				"edges": []any{},
			},
		}},
	}
	proj := Build(Input{
		RackSummaries: RackSummariesFromProjectState(state),
		ProjectPackage: map[string]any{
			"track_count": 1,
			"tracks": []any{map[string]any{
				"track_id": "t1", "clip_count": 1,
				"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
			}},
		},
	})
	row := assertionResultBy(t, proj, "routing", "dead_end", "t1")
	if row.Status != AssertionStatusFail {
		t.Fatalf("routing dead_end status = %s, want fail: %#v", row.Status, row)
	}
	if row.Code != "assert_routing_dead_end" {
		t.Fatalf("routing dead_end code = %s", row.Code)
	}
	if got := hasIssueCodeForTrack(proj.Issues, "assert_routing_dead_end", "t1"); got != 1 {
		t.Fatalf("assert_routing_dead_end issues = %d: %#v", got, proj.Issues)
	}
	if len(RackSummariesFromProjectState(state)) != 1 {
		t.Fatalf("rack summary parsing lost the track rack")
	}
}

// T6 AS-ROUTE P2: the union graph of all rack edges must stay acyclic.
func TestBuildAssertionRoutingCycle(t *testing.T) {
	proj := Build(Input{
		RackSummaries: []RackSummary{
			{TrackID: "t1", Nodes: []RackNode{{NodeID: "A"}, {NodeID: "B"}}, Edges: []RackEdge{{SourceID: "A", DestID: "B"}}},
			{TrackID: "t2", Nodes: []RackNode{{NodeID: "B"}, {NodeID: "A"}}, Edges: []RackEdge{{SourceID: "B", DestID: "A"}}},
		},
		ProjectPackage: map[string]any{
			"track_count": 2,
			"tracks": []any{
				map[string]any{"track_id": "t1", "clip_count": 1, "primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/1.wav"}},
				map[string]any{"track_id": "t2", "clip_count": 1, "primary_clip": map[string]any{"clip_id": "c2", "current_source_path": "/a/2.wav"}},
			},
		},
	})
	row := assertionResultBy(t, proj, "routing", "cycle", "")
	if row.Status != AssertionStatusFail {
		t.Fatalf("routing cycle status = %s, want fail: %#v", row.Status, row)
	}
	if row.Code != "assert_routing_cycle" {
		t.Fatalf("routing cycle code = %s", row.Code)
	}

	acyclic := Build(Input{
		RackSummaries: []RackSummary{
			{TrackID: "t1", Nodes: []RackNode{{NodeID: "A"}, {NodeID: "B"}, {NodeID: "C"}},
				Edges: []RackEdge{{SourceID: "RACK_INPUT", DestID: "A"}, {SourceID: "A", DestID: "B"}, {SourceID: "B", DestID: "C"}, {SourceID: "C", DestID: "RACK_OUTPUT"}}},
		},
		ProjectPackage: map[string]any{
			"track_count": 1,
			"tracks":      []any{map[string]any{"track_id": "t1", "clip_count": 1, "primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/1.wav"}}},
		},
	})
	if !hasAssertionStatus(acyclic, "routing", "cycle", "", AssertionStatusPass) {
		t.Fatalf("acyclic graph should pass: %#v", acyclic.Assertions)
	}
}

// T7 AS-PLUGIN: a rack plugin node whose path is not in the known plugin
// table fails assert_plugin_unknown_path; empty paths stay not_evaluable.
func TestBuildAssertionPluginUnknownPath(t *testing.T) {
	proj := Build(Input{
		KnownPluginPaths: map[string]bool{"/Library/Audio/Plug-Ins/VST3/known.vst3": true},
		RackSummaries: []RackSummary{
			{TrackID: "t1", Nodes: []RackNode{
				{NodeID: "n1", Enabled: true, AudioReachableFromRackInput: true, PluginPath: "/Library/Audio/Plug-Ins/VST3/gone.vst3", PluginFormat: "VST3"},
				{NodeID: "n2", Enabled: true, AudioReachableFromRackInput: true, PluginPath: "/Library/Audio/Plug-Ins/VST3/known.vst3", PluginFormat: "VST3"},
			}},
		},
		ProjectPackage: map[string]any{
			"track_count": 1,
			"tracks": []any{map[string]any{
				"track_id": "t1", "clip_count": 1,
				"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
			}},
		},
	})
	row := assertionResultBy(t, proj, "plugin_legality", "known_path", "t1")
	if row.Status != AssertionStatusFail {
		t.Fatalf("plugin_legality status = %s, want fail: %#v", row.Status, row)
	}
	if row.Code != "assert_plugin_unknown_path" {
		t.Fatalf("plugin_legality code = %s", row.Code)
	}
	if got := hasIssueCodeForTrack(proj.Issues, "assert_plugin_unknown_path", "t1"); got != 1 {
		t.Fatalf("assert_plugin_unknown_path issues = %d: %#v", got, proj.Issues)
	}

	missingTable := Build(Input{
		RackSummaries: []RackSummary{
			{TrackID: "t2", Nodes: []RackNode{
				{NodeID: "n1", Enabled: true, AudioReachableFromRackInput: true, PluginPath: "/x/y.vst3", PluginFormat: "VST3"},
			}},
		},
		ProjectPackage: map[string]any{
			"track_count": 1,
			"tracks":      []any{map[string]any{"track_id": "t2", "clip_count": 1, "primary_clip": map[string]any{"clip_id": "c2", "current_source_path": "/a/b.wav"}}},
		},
	})
	if !hasAssertionStatus(missingTable, "plugin_legality", "known_path", "t2", AssertionStatusNotEvaluable) {
		t.Fatalf("missing plugin table should be not_evaluable: %#v", missingTable.Assertions)
	}
	if !hasLimitation(missingTable.Limitations, "assert_plugin_legality_not_evaluable") {
		t.Fatalf("assert_plugin_legality_not_evaluable limitation missing: %#v", missingTable.Limitations)
	}
}

// T8 three-state honesty: tracks without acoustic evidence produce
// not_evaluable rows and limitations, never a pass.
func TestBuildAssertionsNotEvaluableWithoutEvidence(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t1", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
			"acoustic":     map[string]any{"status": "missing", "reason": "pending"},
		}},
	}})
	for _, check := range []string{"signal_nonfinite", "clipping_headroom"} {
		if !hasAssertionStatus(proj, "signal_hygiene", check, "t1", AssertionStatusNotEvaluable) {
			t.Fatalf("signal_hygiene/%s without evidence should be not_evaluable: %#v", check, proj.Assertions)
		}
	}
	if !hasAssertionStatus(proj, "level_ceiling", "ceiling", "t1", AssertionStatusNotEvaluable) {
		t.Fatalf("level_ceiling without evidence should be not_evaluable: %#v", proj.Assertions)
	}
	for _, code := range []string{
		"assert_signal_nonfinite_not_evaluable",
		"assert_clipping_headroom_not_evaluable",
		"assert_level_ceiling_not_evaluable",
		"assert_routing_not_evaluable",
		"assert_plugin_legality_not_evaluable",
	} {
		if !hasLimitation(proj.Limitations, code) {
			t.Fatalf("not_evaluable limitation %s missing: %#v", code, proj.Limitations)
		}
	}
	for _, row := range proj.Assertions {
		if row.Status == AssertionStatusPass {
			t.Fatalf("evidence-free projection must not produce pass rows: %#v", row)
		}
	}
}

// T9 §11 compatibility: old projection JSON without the assertions field
// loads untouched (Assertions==nil), current JSON round-trips, and unknown
// asserter names fail closed through ReconcileLoadedAssertions.
func TestAssertionSchemaCompatOldJSONAndUnknownAsserter(t *testing.T) {
	oldJSON := `{
		"schema_version": "tim.projection.v0",
		"tim_version": "v0",
		"status": "partial",
		"technical_summary": {"track_count": 1, "clip_count": 1, "source_present_count": 1},
		"coverage": {"source_path": {"status": "partial", "known_count": 0, "total_count": 1, "unknown_count": 1}},
		"risk_summary": {"overall_risk": "medium", "issue_count": 1, "by_severity": {"warning": 1}, "primary_codes": ["acoustic_package_missing_or_partial"]},
		"limitations": ["dad_acoustic_package_not_ready_for_all_tracks"]
	}`
	var loaded Projection
	if err := json.Unmarshal([]byte(oldJSON), &loaded); err != nil {
		t.Fatalf("old projection JSON must load without error: %v", err)
	}
	if loaded.Assertions != nil {
		t.Fatalf("old projection must keep Assertions nil, got %#v", loaded.Assertions)
	}
	if loaded.Limitations[0] != "dad_acoustic_package_not_ready_for_all_tracks" {
		t.Fatalf("old projection fields changed meaning: %#v", loaded.Limitations)
	}
	remarshaled, err := json.Marshal(loaded)
	if err != nil {
		t.Fatalf("old projection must re-marshal: %v", err)
	}
	var again Projection
	if err := json.Unmarshal(remarshaled, &again); err != nil || again.Assertions != nil {
		t.Fatalf("old projection round-trip broke: err=%v assertions=%#v", err, again.Assertions)
	}

	fresh := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t1", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
			"acoustic":     map[string]any{"status": "ready", "peak_dbfs": -0.2, "nan_count": 0, "inf_count": 0, "headroom_db": 0.2},
		}},
	}})
	if len(fresh.Assertions) == 0 {
		t.Fatalf("fresh projection must carry assertions")
	}
	data, err := json.Marshal(fresh)
	if err != nil {
		t.Fatalf("marshal fresh projection: %v", err)
	}
	var reloaded Projection
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("fresh projection round-trip: %v", err)
	}
	if len(reloaded.Assertions) != len(fresh.Assertions) {
		t.Fatalf("assertions round-trip count = %d, want %d", len(reloaded.Assertions), len(fresh.Assertions))
	}

	kept, limitations := ReconcileLoadedAssertions([]AssertionResult{
		{Asserter: "signal_hygiene", Check: "signal_nonfinite", Status: AssertionStatusPass, TrackID: "t1"},
		{Asserter: "future_asserter", Check: "unknown_check", Status: AssertionStatusPass, TrackID: "t1"},
	})
	if len(kept) != 1 || kept[0].Asserter != "signal_hygiene" {
		t.Fatalf("unknown asserter must be skipped, known kept: %#v", kept)
	}
	if !hasString(limitations, "assert_unknown_entity_skipped") {
		t.Fatalf("fail-closed limitation missing: %#v", limitations)
	}
}

// LLMContext must expose assertion counts on the technical_risk fact without
// expanding the raw package disclosure.
func TestBuildLLMContextCarriesAssertionCounts(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t1", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
			"acoustic":     map[string]any{"status": "ready", "peak_dbfs": -0.05},
		}},
	}})
	found := false
	for _, fact := range proj.LLMContext.CompactFacts {
		if fact["layer"] != "technical_risk" {
			continue
		}
		found = true
		counts, ok := fact["assertion_counts"].(map[string]int)
		if !ok {
			t.Fatalf("technical_risk fact missing assertion counts: %#v", fact)
		}
		if counts[AssertionStatusFail] < 1 || counts[AssertionStatusNotEvaluable] < 1 {
			t.Fatalf("assertion counts wrong for hot track: %#v", counts)
		}
	}
	if !found {
		t.Fatalf("technical_risk fact missing: %#v", proj.LLMContext.CompactFacts)
	}
	if !proj.LLMContext.DoNotIncludeRawPackage {
		t.Fatalf("raw package disclosure flag regressed")
	}
}
