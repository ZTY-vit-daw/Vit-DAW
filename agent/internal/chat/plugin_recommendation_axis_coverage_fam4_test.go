package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/processorattestation"
)

// The D2-FAM4-S2 limiter governance fixture reproduces the 20260902_082924
// main-run wall: Pro-L 2 is family-promoted and covers the frozen
// output_ceiling axis, but axisCoverageWhitelistIdentifiers had no limiter
// case, so the whitelist-form gate emptied the offered set and the honest
// empty-set response stopped the chain before any selection or load.
func axisCoverageLimiterFixture(t *testing.T) (processorattestation.Subject, []pluginRecommendationCandidate) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(dir, "attestations.v2.json"))
	proLPath := filepath.Join(dir, "FabFilter Pro-L 2.vst3")
	l1Path := filepath.Join(dir, "WaveShell1-VST3 fixture.vst3")
	for path, payload := range map[string]string{proLPath: "pro-l-fixture-binary", l1Path: "waves-shell-fixture-binary"} {
		if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	proL := processorattestation.Subject{Name: "Pro-L 2", Manufacturer: "FabFilter", Format: "VST3",
		Identifier: "VST3-Pro-L 2-9890e876-45398b95", InstalledPath: proLPath}
	l1 := processorattestation.Subject{Name: "L1 limiter Stereo", Manufacturer: "Waves", Format: "VST3",
		Identifier: "VST3-L1 limiter Stereo-695648d-l1fixture00", InstalledPath: l1Path}
	for _, promotion := range []struct {
		subject  processorattestation.Subject
		coverage []processorattestation.Coverage
	}{
		{proL, []processorattestation.Coverage{{Action: "adjust", Axis: "input_drive"}, {Action: "adjust", Axis: "output_ceiling"}}},
		{l1, []processorattestation.Coverage{{Action: "adjust", Axis: "output_ceiling"}, {Action: "adjust", Axis: "protection_intensity"}}},
	} {
		fingerprint, err := processorattestation.FingerprintPath(promotion.subject.InstalledPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.PromoteCurrent(processorattestation.IssueSpecV2{
			Subject: promotion.subject, BinaryFingerprint: fingerprint,
			ProcessorFamily: processorattestation.FamilyLimiter, Coverage: promotion.coverage,
			Evidence: []processorattestation.EvidenceRef{{ReceiptID: "axis-coverage-limiter-fixture", Kind: "axis_coverage_fixture",
				SHA256: "sha256:" + strings.Repeat("b", 64), ObservedAt: time.Now().UTC()}},
		}, "axis_coverage_fixture"); err != nil {
			t.Fatal(err)
		}
	}
	previous := d1StaticEQWhitelistLoader
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		return experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			Limiter: experimentplugins.LimiterPlugins{experimentplugins.LimiterPlugin{
				PluginName: "Pro-L 2", Manufacturer: "FabFilter", Format: "VST3",
				PluginIdentifier: proL.Identifier, PluginPath: proL.InstalledPath, CeilingParamID: "18",
			}},
		}, nil
	}
	t.Cleanup(func() { d1StaticEQWhitelistLoader = previous })
	return proL, []pluginRecommendationCandidate{
		{Key: "plugin_candidate_1", Name: l1.Name, Manufacturer: l1.Manufacturer, Format: l1.Format,
			Identifier: l1.Identifier, PluginPath: l1.InstalledPath, Category: "Fx|Dynamics", PrimaryType: "dynamics"},
		{Key: "plugin_candidate_2", Name: proL.Name, Manufacturer: proL.Manufacturer, Format: proL.Format,
			Identifier: proL.Identifier, PluginPath: proL.InstalledPath, Category: "Fx|Dynamics", PrimaryType: "dynamics"},
	}
}

func axisCoverageFrozenLimiterIntent(axis string) map[string]any {
	return map[string]any{
		"schema_version":    "semantic_processor_intent.v1",
		"status":            "resolved",
		"family":            "limiter",
		"intent":            "admitted bounded experiment limiter_ceiling_adjust on axis " + axis,
		"required_coverage": []string{axis},
		"scope":             "current_track",
		"control_mode":      "semantic_loop",
		"confidence":        1.0,
		"evidence_refs":     []string{"obs_axis_fixture"},
	}
}

// RED (FAM4-S2, 20260902_082924 wall): the whitelist-form gate must resolve
// the limiter section so an axis-covered, whitelist-eligible Pro-L 2 survives
// governance; before the fix the family switch had no limiter case and the
// run honestly stopped on the empty set.
func TestAxisCoverageGovernedCandidatesKeepWhitelistedLimiter(t *testing.T) {
	proL, candidates := axisCoverageLimiterFixture(t)
	governed, disclosure, err := axisCoverageGovernedPluginRecommendationCandidates(candidates, "limiter",
		map[string]any{"free_state_semantic_processor_intent": axisCoverageFrozenLimiterIntent("output_ceiling")})
	if err != nil {
		t.Fatalf("axis coverage governance failed: %v", err)
	}
	if len(governed) != 1 || governed[0].Identifier != proL.Identifier {
		t.Fatalf("governed candidates must keep only the axis-covered whitelist limiter: %+v", governed)
	}
	if disclosure == nil {
		t.Fatalf("governed flow must disclose frozen axes and per-candidate attested coverage")
	}
	rows := mapRowsValue(disclosure["candidates"])
	if len(rows) != 1 || firstStringFromMap(rows[0], "candidate_key") != "plugin_candidate_1" {
		t.Fatalf("per-candidate disclosure rows=%#v", rows)
	}
	attested := stringSliceFromAny(rows[0]["attested_coverage_axes"])
	if len(attested) != 2 || attested[0] != "input_drive" || attested[1] != "output_ceiling" {
		t.Fatalf("attested coverage axes=%#v", attested)
	}
}

// RED (FAM5 companion, same switch): the gate_expander family must resolve
// its whitelist section too.
func TestAxisCoverageWhitelistIdentifierResolvesGateExpanderSection(t *testing.T) {
	previous := d1StaticEQWhitelistLoader
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		return experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			GateExpander: experimentplugins.GateExpanderPlugins{experimentplugins.GateExpanderPlugin{
				PluginName: "Pro-G", Manufacturer: "FabFilter", Format: "VST3",
				PluginIdentifier: "VST3-Pro-G-38fcf5ad-5a6e43a2", PluginPath: "C:/plugins/FabFilter Pro-G.vst3", RangeParamID: "4",
			}},
		}, nil
	}
	t.Cleanup(func() { d1StaticEQWhitelistLoader = previous })
	identifierSet, err := axisCoverageWhitelistIdentifiers(processorattestation.FamilyGateExpander)
	if err != nil {
		t.Fatal(err)
	}
	if !identifierSet["VST3-Pro-G-38fcf5ad-5a6e43a2"] {
		t.Fatalf("gate_expander whitelist identifiers=%v", identifierSet)
	}
}
