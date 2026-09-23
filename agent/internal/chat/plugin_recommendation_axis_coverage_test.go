package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/shadow"
)

// The D2-FAM2-S3 axis coverage governance fixtures reproduce the 190456
// pre-filter form: three family-promoted transient_shaper candidates where
// only SPL Transient Designer Plus is simultaneously axis-covered
// (envelope_emphasis) and whitelist-form eligible. Smack Attack Stereo is
// family-promoted but axis-divergent; TransX Wide Stereo is axis-covered but
// not the whitelisted execution subject. All binaries and stores live in
// t.TempDir, never the developer's ~/.vit.

type axisCoverageFixture struct {
	SPL       processorattestation.Subject
	Smack     processorattestation.Subject
	TransX    processorattestation.Subject
	Whitelist experimentplugins.Whitelist
}

func axisCoverageFrozenTransientIntent(axis string) map[string]any {
	return map[string]any{
		"schema_version":    "semantic_processor_intent.v1",
		"status":            "resolved",
		"family":            "transient_shaper",
		"intent":            "admitted bounded experiment transient_attack_adjust on axis " + axis,
		"required_coverage": []string{axis},
		"scope":             "current_track",
		"control_mode":      "semantic_loop",
		"confidence":        1.0,
		"evidence_refs":     []string{"obs_axis_fixture"},
	}
}

func axisCoverageTransientFixture(t *testing.T) axisCoverageFixture {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(dir, "attestations.v2.json"))
	splPath := filepath.Join(dir, "SPL Transient Designer Plus.vst3")
	shellPath := filepath.Join(dir, "WaveShell1-VST3 fixture.vst3")
	if err := os.WriteFile(splPath, []byte("spl-transient-fixture-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shellPath, []byte("waves-shell-fixture-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	fixture := axisCoverageFixture{
		SPL: processorattestation.Subject{Name: "SPL Transient Designer Plus", Manufacturer: "Plugin Alliance", Format: "VST3",
			Identifier: "VST3-SPL Transient Designer Plus-95d14fc6-671d42ff", InstalledPath: splPath},
		Smack: processorattestation.Subject{Name: "Smack Attack Stereo", Manufacturer: "Waves", Format: "VST3",
			Identifier: "VST3-Smack Attack Stereo-695648d-ead81c48", InstalledPath: shellPath},
		TransX: processorattestation.Subject{Name: "TransX Wide Stereo", Manufacturer: "Waves", Format: "VST3",
			Identifier: "VST3-TransX Wide Stereo-695648d-bcdf78f6", InstalledPath: shellPath},
	}
	for _, promotion := range []struct {
		subject  processorattestation.Subject
		coverage []processorattestation.Coverage
	}{
		// v2 library truth verified against ~/.vit on 2026-09-01: SPL TD+
		// covers detector_focus+envelope_emphasis, Smack Attack covers
		// envelope_timing+output_normalization (no envelope_emphasis), TransX
		// covers envelope_emphasis+output_normalization on the shared shell.
		{fixture.SPL, []processorattestation.Coverage{{Action: "adjust", Axis: "detector_focus"}, {Action: "adjust", Axis: "envelope_emphasis"}}},
		{fixture.Smack, []processorattestation.Coverage{{Action: "adjust", Axis: "envelope_timing"}, {Action: "adjust", Axis: "output_normalization"}}},
		{fixture.TransX, []processorattestation.Coverage{{Action: "adjust", Axis: "envelope_emphasis"}, {Action: "adjust", Axis: "output_normalization"}}},
	} {
		fingerprint, err := processorattestation.FingerprintPath(promotion.subject.InstalledPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.PromoteCurrent(processorattestation.IssueSpecV2{
			Subject: promotion.subject, BinaryFingerprint: fingerprint,
			ProcessorFamily: processorattestation.FamilyTransient, Coverage: promotion.coverage,
			Evidence: []processorattestation.EvidenceRef{{ReceiptID: "axis-coverage-fixture", Kind: "axis_coverage_fixture",
				SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Now().UTC()}},
		}, "axis_coverage_fixture"); err != nil {
			t.Fatal(err)
		}
	}
	fixture.Whitelist = experimentplugins.Whitelist{
		SchemaVersion: experimentplugins.SchemaVersion,
		TransientShaper: experimentplugins.TransientShaperPlugins{experimentplugins.TransientShaperPlugin{
			PluginName: "SPL Transient Designer Plus", Manufacturer: "Plugin Alliance", Format: "VST3",
			PluginIdentifier: fixture.SPL.Identifier, PluginPath: fixture.SPL.InstalledPath, AttackParamID: "1098151019",
		}},
	}
	previous := d1StaticEQWhitelistLoader
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		return fixture.Whitelist, nil
	}
	t.Cleanup(func() { d1StaticEQWhitelistLoader = previous })
	return fixture
}

// axisCoverageFamilyPromotedCandidates models the post-(a) input: three
// candidates that all passed the family-promoted PCA admission boundary, in
// the 190456 order (Smack Attack recommended first).
func axisCoverageFamilyPromotedCandidates(fixture axisCoverageFixture) []pluginRecommendationCandidate {
	return []pluginRecommendationCandidate{
		{Key: "plugin_candidate_1", Name: fixture.Smack.Name, Manufacturer: fixture.Smack.Manufacturer, Format: fixture.Smack.Format,
			Identifier: fixture.Smack.Identifier, PluginPath: fixture.Smack.InstalledPath, Category: "Fx|Dynamics", PrimaryType: "dynamics"},
		{Key: "plugin_candidate_2", Name: fixture.SPL.Name, Manufacturer: fixture.SPL.Manufacturer, Format: fixture.SPL.Format,
			Identifier: fixture.SPL.Identifier, PluginPath: fixture.SPL.InstalledPath, Category: "Fx|Dynamics", PrimaryType: "dynamics"},
		{Key: "plugin_candidate_3", Name: fixture.TransX.Name, Manufacturer: fixture.TransX.Manufacturer, Format: fixture.TransX.Format,
			Identifier: fixture.TransX.Identifier, PluginPath: fixture.TransX.InstalledPath, Category: "Fx|Dynamics", PrimaryType: "dynamics"},
	}
}

// 行为 RED：过滤前形态 = 190456 复现（三候选含轴异者）→ 过滤后单候选 SPL TD+。
func TestAxisCoverageGovernedCandidatesKeepOnlyAxisCoveredWhitelistForm(t *testing.T) {
	fixture := axisCoverageTransientFixture(t)
	governed, disclosure, err := axisCoverageGovernedPluginRecommendationCandidates(
		axisCoverageFamilyPromotedCandidates(fixture), "transient_shaper",
		map[string]any{"free_state_semantic_processor_intent": axisCoverageFrozenTransientIntent("envelope_emphasis")})
	if err != nil {
		t.Fatalf("axis coverage governance failed: %v", err)
	}
	if len(governed) != 1 || governed[0].Identifier != fixture.SPL.Identifier {
		t.Fatalf("governed candidates must keep only the axis-covered whitelist subject: %+v", governed)
	}
	if governed[0].Key != "plugin_candidate_1" {
		t.Fatalf("governed candidate keys must be renumbered: %+v", governed[0])
	}
	if disclosure == nil {
		t.Fatalf("governed flow must disclose frozen axes and per-candidate attested coverage")
	}
	if axes := stringSliceFromAny(disclosure["frozen_semantic_axes"]); len(axes) != 1 || axes[0] != "envelope_emphasis" {
		t.Fatalf("frozen axes disclosure=%#v", disclosure["frozen_semantic_axes"])
	}
	rows := mapRowsValue(disclosure["candidates"])
	if len(rows) != 1 || firstStringFromMap(rows[0], "candidate_key") != "plugin_candidate_1" {
		t.Fatalf("per-candidate disclosure rows=%#v", rows)
	}
	attested := stringSliceFromAny(rows[0]["attested_coverage_axes"])
	if len(attested) != 2 || attested[0] != "detector_focus" || attested[1] != "envelope_emphasis" {
		t.Fatalf("attested coverage axes=%#v", attested)
	}
}

// 空集 RED：冻结轴无任何候选覆盖 → 诚实空集，披露仍在，无回退候选。
func TestAxisCoverageGovernedCandidatesEmptySetStaysHonest(t *testing.T) {
	fixture := axisCoverageTransientFixture(t)
	governed, disclosure, err := axisCoverageGovernedPluginRecommendationCandidates(
		axisCoverageFamilyPromotedCandidates(fixture), "transient_shaper",
		map[string]any{"free_state_semantic_processor_intent": axisCoverageFrozenTransientIntent("shape_mode")})
	if err != nil {
		t.Fatalf("uncovered frozen axis must produce an honest empty set, not an error: %v", err)
	}
	if len(governed) != 0 {
		t.Fatalf("uncovered frozen axis must not keep candidates: %+v", governed)
	}
	if disclosure == nil {
		t.Fatalf("empty set must still disclose the frozen axes that produced it")
	}
	if axes := stringSliceFromAny(disclosure["frozen_semantic_axes"]); len(axes) != 1 || axes[0] != "shape_mode" {
		t.Fatalf("frozen axes disclosure=%#v", disclosure["frozen_semantic_axes"])
	}
	server := New(nil, shadow.New(nil), nil)
	res := agentloop.Result{GoalID: "goal-1", RunID: "run-1", Status: "completed"}
	resp := server.pluginRecommendationAxisCoverageNoCandidatesResponse("conversation-1", agentModeDefault, "改善瞬态起音",
		map[string]any{"selected_track_id": "1012"}, res, "transient_shaper", []string{"shape_mode"})
	if resp.WorkflowData["status"] != "no_axis_covered_candidates" {
		t.Fatalf("empty-set status must name the gap form: %#v", resp.WorkflowData)
	}
	if resp.StopReason != "no_axis_covered_candidates" {
		t.Fatalf("empty-set stop_reason must name the gap form: %q", resp.StopReason)
	}
	if len(mapRowsValue(resp.WorkflowData["recommendations"])) != 0 {
		t.Fatalf("empty-set response must not offer fallback candidates: %#v", resp.WorkflowData)
	}
	if boolValue(resp.WorkflowData["mutation_performed"]) || boolValue(resp.WorkflowData["selection_performed"]) {
		t.Fatalf("empty-set response must not mutate or select: %#v", resp.WorkflowData)
	}
	if axes := stringSliceFromAny(firstMapFromAny(resp.WorkflowData["axis_coverage_disclosure"])["frozen_semantic_axes"]); len(axes) != 1 || axes[0] != "shape_mode" {
		t.Fatalf("empty-set disclosure=%#v", resp.WorkflowData["axis_coverage_disclosure"])
	}
}

// 披露 RED：过滤后的推荐 payload 披露每候选认证覆盖轴 + 本实验冻结轴。
func TestPluginRecommendationSelectionResponseDisclosesAxisCoverage(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := pluginRecommendationPlan{SchemaVersion: "plugin_recommendation.v1", ProcessorType: "transient_shaper",
		UserGoal: "改善瞬态起音", Summary: "推荐 SPL", Choices: []pluginRecommendationChoice{{
			CandidateKey: "plugin_candidate_1", Role: "recommended", Reason: "轴覆盖且白名单合格", Confidence: "high"}}}
	candidates := []pluginRecommendationCandidate{{Key: "plugin_candidate_1", Name: "SPL Transient Designer Plus",
		Identifier: "VST3-SPL Transient Designer Plus-95d14fc6-671d42ff", PluginPath: "C:/fixture/SPL.vst3", ProcessorFamily: "transient_shaper"}}
	disclosure := map[string]any{
		"schema_version":       "plugin_recommendation_axis_coverage_disclosure.v1",
		"frozen_semantic_axes": []string{"envelope_emphasis"},
		"candidates":           []any{map[string]any{"candidate_key": "plugin_candidate_1", "attested_coverage_axes": []string{"detector_focus", "envelope_emphasis"}}},
	}
	res := agentloop.Result{GoalID: "goal-1", RunID: "run-1", Status: "completed"}
	resp := server.pluginRecommendationSelectionResponse("conversation-1", agentModeDefault, map[string]any{
		"selected_track_id":                           "1012",
		"free_state_semantic_processor_intent":        axisCoverageFrozenTransientIntent("envelope_emphasis"),
		pluginRecommendationAxisCoverageDisclosureKey: disclosure,
	}, res, plan, candidates)
	payload := firstMapFromAny(resp.WorkflowData["axis_coverage_disclosure"])
	if len(payload) == 0 {
		t.Fatalf("selection payload must disclose axis coverage: %#v", resp.WorkflowData)
	}
	if axes := stringSliceFromAny(payload["frozen_semantic_axes"]); len(axes) != 1 || axes[0] != "envelope_emphasis" {
		t.Fatalf("payload frozen axes=%#v", payload["frozen_semantic_axes"])
	}
	rows := mapRowsValue(payload["candidates"])
	if len(rows) != 1 || firstStringFromMap(rows[0], "candidate_key") != "plugin_candidate_1" {
		t.Fatalf("payload per-candidate rows=%#v", rows)
	}
	if attested := stringSliceFromAny(rows[0]["attested_coverage_axes"]); len(attested) != 2 {
		t.Fatalf("payload attested axes=%#v", attested)
	}
	// Without governance the payload stays byte-identical in shape: no
	// disclosure key is invented for ungoverned recommendation faces.
	plain := server.pluginRecommendationSelectionResponse("conversation-1", agentModeDefault, map[string]any{
		"selected_track_id": "1012",
	}, res, plan, candidates)
	if _, present := plain.WorkflowData["axis_coverage_disclosure"]; present {
		t.Fatalf("ungoverned selection payload must not carry a disclosure: %#v", plain.WorkflowData)
	}
}

// 对照锁定：无 intent / 原生路径候选面字节不变；坏 intent 与坏白名单 fail-closed。
func TestAxisCoverageGovernanceControlLocksFailClosedAndStaysInactive(t *testing.T) {
	fixture := axisCoverageTransientFixture(t)
	candidates := axisCoverageFamilyPromotedCandidates(fixture)

	// No frozen intent: the candidate face is returned unchanged.
	unchanged, disclosure, err := axisCoverageGovernedPluginRecommendationCandidates(candidates, "transient_shaper", map[string]any{})
	if err != nil || disclosure != nil || len(unchanged) != len(candidates) {
		t.Fatalf("no-intent path must be unchanged: n=%d disclosure=%v err=%v", len(unchanged), disclosure, err)
	}
	for index := range unchanged {
		if unchanged[index] != candidates[index] {
			t.Fatalf("no-intent path must be byte-identical: %+v != %+v", unchanged[index], candidates[index])
		}
	}

	// Malformed frozen intent: fail closed, never silently unfiltered.
	if _, disclosure, err := axisCoverageGovernedPluginRecommendationCandidates(candidates, "transient_shaper",
		map[string]any{"free_state_semantic_processor_intent": map[string]any{"schema_version": "semantic_processor_intent.v1", "status": "resolved"}}); err == nil || disclosure != nil {
		t.Fatalf("malformed intent must fail closed: err=%v disclosure=%v", err, disclosure)
	}

	// Family mismatch between intent and requested processor type: fail closed.
	if _, _, err := axisCoverageGovernedPluginRecommendationCandidates(candidates, "de_esser",
		map[string]any{"free_state_semantic_processor_intent": axisCoverageFrozenTransientIntent("envelope_emphasis")}); err == nil {
		t.Fatalf("family mismatch must fail closed")
	}

	// Corrupt whitelist: fail closed instead of guessing whitelist form.
	previous := d1StaticEQWhitelistLoader
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		return experimentplugins.Whitelist{}, context.DeadlineExceeded
	}
	t.Cleanup(func() { d1StaticEQWhitelistLoader = previous })
	if _, _, err := axisCoverageGovernedPluginRecommendationCandidates(candidates, "transient_shaper",
		map[string]any{"free_state_semantic_processor_intent": axisCoverageFrozenTransientIntent("envelope_emphasis")}); err == nil {
		t.Fatalf("corrupt whitelist must fail closed")
	}
}

// stringSliceFromAny normalizes []string and []any-of-strings for disclosure
// assertions.
func stringSliceFromAny(value any) []string {
	if typed, ok := value.([]string); ok {
		return append([]string(nil), typed...)
	}
	rows, _ := value.([]any)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if text, ok := row.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
