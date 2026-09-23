package chat

// FIX-BROADBAND-SHARED-1 (PORT-PCA-FULL-CANDIDATES-1 decision A): a
// broadband_compression whitelist entry in the single shared-threshold form
// resolves the D1 binding to one parameter id (empty ParamIDCH2), and the
// plan's action args omit param_id_ch2 entirely — the FAM1-S1 single-entry
// batch write semantics the port already accepts. Historical dual entries
// (PC live Vertigo VSC-2) keep resolving to the ch pair via the untouched
// tests in free_state_d1_plan_table_s2_test.go.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/processorattestation"
)

// d1SharedCompressionWhitelistFixtureOnDisk mirrors
// d1CompressionWhitelistFixtureOnDisk with the shared-threshold form: one
// threshold_param_id, no ch pair (the mac Waves comp shape decision A
// unblocks). The PCA v1 library record promotes exactly this binary.
func d1SharedCompressionWhitelistFixtureOnDisk(t *testing.T) d1CompressionWhitelistFixture {
	t.Helper()
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture Shared Comp.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-shared-comp-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	fixture := d1CompressionWhitelistFixture{
		Whitelist: experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			BroadbandCompression: experimentplugins.BroadbandCompressionPlugins{experimentplugins.BroadbandCompressionPlugin{
				PluginName:       "Fixture Shared Comp",
				Manufacturer:     "Fixture",
				Format:           "VST3",
				PluginIdentifier: "fixture-shared-comp",
				PluginPath:       pluginPath,
				ThresholdParamID: "thr_shared",
			}},
		},
		PluginPath:  pluginPath,
		Fingerprint: fingerprint,
	}
	encoded, err := json.MarshalIndent(map[string]any{
		"schema_version":        fixture.Whitelist.SchemaVersion,
		"broadband_compression": fixture.Whitelist.BroadbandCompression,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "free_state_experiment_plugins.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return fixture
}

// red ① binding: a shared-form entry resolves the normalized binding with one
// parameter id and an empty ch2, passing the same PCA admission gate.
func TestResolveD1PluginParamBindingForCompressionSharedThreshold(t *testing.T) {
	fixture := d1SharedCompressionWhitelistFixtureOnDisk(t)
	subject := processorattestation.Subject{Name: "Fixture Shared Comp", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-shared-comp", InstalledPath: fixture.PluginPath}
	typedAction := map[string]any{"action_domain": d1BroadbandCompressionDomain, "action_kind": d1BroadbandCompressionKind, "threshold_db": 1.5}

	emptyLibrary := processorattestation.Library{SchemaVersion: processorattestation.LibrarySchema}
	d1TableOverrideLoaders(t, fixture.Whitelist, nil, emptyLibrary, nil)
	_, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "refused by the PCA admission check") || !strings.Contains(err.Error(), "not PCA-promoted") {
		t.Fatalf("shared-form ineligible class wrong: %v", err)
	}

	d1TableOverrideLoaders(t, fixture.Whitelist, nil,
		d1CompressionPromotedLibrary(t, subject, fixture.Fingerprint), nil)
	binding, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Section != d1BroadbandCompressionDomain || binding.PluginPath != fixture.PluginPath ||
		binding.PluginIdentifier != "fixture-shared-comp" ||
		binding.ParamID != "thr_shared" || binding.ParamIDCH2 != "" || binding.FrequencyHz != 0 {
		t.Fatalf("resolved shared-form compression binding=%+v", binding)
	}
}

// red ① plan shape: a shared-form binding writes through
// normalized_batch_v1 with the single parameter id and omits param_id_ch2
// entirely (GLM ruling on D2-FAM1-S1 ③, correction b, now carried by the
// broadband shared form).
func TestD1S1CompressionPlanSharedThresholdOmitsCh2(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "7")
	binding := &d1PluginParamWhitelistBinding{
		Section:          d1BroadbandCompressionDomain,
		PluginName:       "Fixture Shared Comp",
		PluginPath:       "C:/plugins/Fixture Shared Comp.vst3",
		PluginIdentifier: "fixture-shared-comp",
		ParamID:          "thr_shared",
	}
	plan, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1BroadbandCompressionKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, binding, "")
	if err != nil {
		t.Fatal(err)
	}
	action := plan.ActionSet.Actions[0]
	if action.Args["write_mode"] != "normalized_batch_v1" || action.Args["param_id"] != "thr_shared" ||
		action.Args["target_value"] != 1.5 || action.Args["plugin_identifier"] != "fixture-shared-comp" {
		t.Fatalf("shared-form binding args=%+v", action.Args)
	}
	if _, present := action.Args["param_id_ch2"]; present {
		t.Fatalf("shared-form action must omit param_id_ch2: %+v", action.Args)
	}
	if _, present := action.Args["frequency_hz"]; present {
		t.Fatalf("compression action must not carry an EQ frequency: %+v", action.Args)
	}
}
