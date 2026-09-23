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

// The D2-FAM6-S1 multiband governance fixture reproduces the FAM4-S2
// 20260902_082924 wall shape in the multiband family: Lindell MBC is
// family-promoted and covers the frozen band_dynamics axis, but
// axisCoverageWhitelistIdentifiers had no multiband_dynamics case, so the
// whitelist-form gate emptied the offered set and the honest empty-set
// response would stop the chain before any selection or load.
func axisCoverageMultibandFixture(t *testing.T) (processorattestation.Subject, []pluginRecommendationCandidate) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(dir, "attestations.v2.json"))
	mbcPath := filepath.Join(dir, "Lindell MBC.vst3")
	c4Path := filepath.Join(dir, "WaveShell1-VST3 fixture.vst3")
	for path, payload := range map[string]string{mbcPath: "mbc-fixture-binary", c4Path: "waves-shell-fixture-binary"} {
		if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	mbc := processorattestation.Subject{Name: "Lindell MBC", Manufacturer: "Plugin Alliance", Format: "VST3",
		Identifier: "VST3-Lindell MBC-e2c119ce-a6b1eb75", InstalledPath: mbcPath}
	c4 := processorattestation.Subject{Name: "C4 Stereo", Manufacturer: "Waves", Format: "VST3",
		Identifier: "VST3-C4 Stereo-695648d-c4fixture00", InstalledPath: c4Path}
	for _, promotion := range []struct {
		subject  processorattestation.Subject
		coverage []processorattestation.Coverage
	}{
		{mbc, []processorattestation.Coverage{{Action: "adjust", Axis: "band_dynamics"}, {Action: "adjust", Axis: "band_timing"}}},
		{c4, []processorattestation.Coverage{{Action: "adjust", Axis: "band_dynamics"}, {Action: "adjust", Axis: "band_timing"}}},
	} {
		fingerprint, err := processorattestation.FingerprintPath(promotion.subject.InstalledPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.PromoteCurrent(processorattestation.IssueSpecV2{
			Subject: promotion.subject, BinaryFingerprint: fingerprint,
			ProcessorFamily: processorattestation.FamilyMultiband, Coverage: promotion.coverage,
			Evidence: []processorattestation.EvidenceRef{{ReceiptID: "axis-coverage-multiband-fixture", Kind: "axis_coverage_fixture",
				SHA256: "sha256:" + strings.Repeat("e", 64), ObservedAt: time.Now().UTC()}},
		}, "axis_coverage_fixture"); err != nil {
			t.Fatal(err)
		}
	}
	previous := d1StaticEQWhitelistLoader
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		return experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			Multiband: experimentplugins.MultibandPlugins{experimentplugins.MultibandPlugin{
				PluginName: "Lindell MBC", Manufacturer: "Plugin Alliance", Format: "VST3",
				PluginIdentifier: mbc.Identifier, PluginPath: mbc.InstalledPath,
				BandThresholdParamIDs: []string{"22950807", "1782596387", "2112130249"},
			}},
		}, nil
	}
	t.Cleanup(func() { d1StaticEQWhitelistLoader = previous })
	return mbc, []pluginRecommendationCandidate{
		{Key: "plugin_candidate_1", Name: c4.Name, Manufacturer: c4.Manufacturer, Format: c4.Format,
			Identifier: c4.Identifier, PluginPath: c4.InstalledPath, Category: "Fx|Dynamics", PrimaryType: "dynamics"},
		{Key: "plugin_candidate_2", Name: mbc.Name, Manufacturer: mbc.Manufacturer, Format: mbc.Format,
			Identifier: mbc.Identifier, PluginPath: mbc.InstalledPath, Category: "Fx|Dynamics", PrimaryType: "dynamics"},
	}
}

func axisCoverageFrozenMultibandIntent(axis string) map[string]any {
	return map[string]any{
		"schema_version":    "semantic_processor_intent.v1",
		"status":            "resolved",
		"family":            "multiband_dynamics",
		"intent":            "admitted bounded experiment multiband_band_threshold_adjust on axis " + axis,
		"required_coverage": []string{axis},
		"scope":             "current_track",
		"control_mode":      "semantic_loop",
		"confidence":        1.0,
		"evidence_refs":     []string{"obs_axis_fixture"},
	}
}

// RED (FAM6-S1, FAM4-S2 wall shape): the whitelist-form gate must resolve the
// multiband section so an axis-covered, whitelist-eligible Lindell MBC
// survives governance; before the fix the family switch had no
// multiband_dynamics case and the run honestly stopped on the empty set.
func TestAxisCoverageGovernedCandidatesKeepWhitelistedMultiband(t *testing.T) {
	mbc, candidates := axisCoverageMultibandFixture(t)
	governed, disclosure, err := axisCoverageGovernedPluginRecommendationCandidates(candidates, "multiband_dynamics",
		map[string]any{"free_state_semantic_processor_intent": axisCoverageFrozenMultibandIntent("band_dynamics")})
	if err != nil {
		t.Fatalf("axis coverage governance failed: %v", err)
	}
	if len(governed) != 1 || governed[0].Identifier != mbc.Identifier {
		t.Fatalf("governed candidates must keep only the axis-covered whitelist multiband: %+v", governed)
	}
	if disclosure == nil {
		t.Fatalf("governed flow must disclose frozen axes and per-candidate attested coverage")
	}
	rows := mapRowsValue(disclosure["candidates"])
	if len(rows) != 1 || firstStringFromMap(rows[0], "candidate_key") != "plugin_candidate_1" {
		t.Fatalf("per-candidate disclosure rows=%#v", rows)
	}
	attested := stringSliceFromAny(rows[0]["attested_coverage_axes"])
	if len(attested) != 2 || attested[0] != "band_dynamics" || attested[1] != "band_timing" {
		t.Fatalf("attested coverage axes=%#v", attested)
	}
}

// RED (FAM6-S1, same switch): the multiband_dynamics family must resolve its
// whitelist section.
func TestAxisCoverageWhitelistIdentifierResolvesMultibandSection(t *testing.T) {
	previous := d1StaticEQWhitelistLoader
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		return experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			Multiband: experimentplugins.MultibandPlugins{experimentplugins.MultibandPlugin{
				PluginName: "Lindell MBC", Manufacturer: "Plugin Alliance", Format: "VST3",
				PluginIdentifier: "VST3-Lindell MBC-e2c119ce-a6b1eb75", PluginPath: "C:/plugins/Lindell MBC.vst3",
				BandThresholdParamIDs: []string{"22950807", "1782596387", "2112130249"},
			}},
		}, nil
	}
	t.Cleanup(func() { d1StaticEQWhitelistLoader = previous })
	identifierSet, err := axisCoverageWhitelistIdentifiers(processorattestation.FamilyMultiband)
	if err != nil {
		t.Fatal(err)
	}
	if !identifierSet["VST3-Lindell MBC-e2c119ce-a6b1eb75"] {
		t.Fatalf("multiband whitelist identifiers=%v", identifierSet)
	}
}
