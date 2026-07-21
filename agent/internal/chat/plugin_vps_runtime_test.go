package chat

import (
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/pluginvps"
	"vit-daw-agent/internal/spal"
)

func chatVerifiedPluginVPS() pluginvps.Document {
	return pluginvps.Document{
		SchemaVersion: pluginvps.SchemaVersion,
		Plugin: pluginvps.PluginIdentity{
			Name: "Injected EQ", Format: "VST3", InstallPath: `C:\VST3\Injected EQ.vst3`,
			InstallationHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Capability: pluginvps.EqualizerCapability,
		Bindings: spal.EQV2Binding{
			ConformedSchemas: []string{spal.EQBandPatchControlID},
			Bands: map[string]spal.EQV2BandBinding{"b1": {
				ComponentID:   "b1",
				Enabled:       spal.ParameterBinding{ParameterID: "1", Unit: "toggle", Min: 0, Max: 1, Scale: "linear"},
				ResponseShape: spal.EnumParameterBinding{ParameterID: "2", Values: map[string]float64{"bell": 0}},
				FrequencyHz:   spal.ParameterBinding{ParameterID: "3", Unit: "Hz", Min: 20, Max: 20000, Scale: "log"},
				GainDB:        spal.ParameterBinding{ParameterID: "4", Unit: "dB", Min: -18, Max: 18, Scale: "linear"},
				Q:             spal.ParameterBinding{ParameterID: "5", Unit: "Q", Min: .1, Max: 12, Scale: "log"},
			}},
		},
		Verified: true,
		Verification: &pluginvps.Verification{
			InstallationHash:     "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ParameterSurfaceHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			VerifiedAt:           time.Date(2026, time.July, 21, 0, 0, 0, 0, time.UTC),
		},
	}
}

func TestServerStartupScansVerifiedPluginVPSDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VIT_PLUGIN_VPS_DIR", dir)
	if err := pluginvps.Save(filepath.Join(dir, "injected-eq.vps.json"), chatVerifiedPluginVPS()); err != nil {
		t.Fatal(err)
	}
	server := New(nil, nil, nil)
	if server.pluginVPS == nil || len(server.pluginVPS.Documents()) != 1 {
		t.Fatalf("startup registry = %#v", server.pluginVPS)
	}
}

func TestB4ApplyArgsIncludeVerifiedVPSForPreviewAndFreeze(t *testing.T) {
	dir := t.TempDir()
	if err := pluginvps.Save(filepath.Join(dir, "injected-eq.vps.json"), chatVerifiedPluginVPS()); err != nil {
		t.Fatal(err)
	}
	registry, warnings := pluginvps.LoadDirectory(dir)
	if len(warnings) != 0 {
		t.Fatalf("registry warnings: %v", warnings)
	}
	server := &Server{pluginVPS: registry}
	args := server.pluginEffectApplyArgsWithVerifiedVPS(map[string]any{
		"plugin_effect_apply_args": map[string]any{
			"track_id": "track-1", "plugin_id": "plugin-1", "control": "eq.cut_region",
			"target": map[string]any{"frequency_hz": 200.0, "gain_db": -3.0, "q": 1.0},
		},
	})
	profiles, ok := args["verified_vps_profiles"].([]map[string]any)
	if !ok || len(profiles) != 1 {
		t.Fatalf("verified VPS profiles = %#v", args["verified_vps_profiles"])
	}
	if profiles[0]["source"] != "verified_vps" || profiles[0]["verified"] != true {
		t.Fatalf("injected profile = %#v", profiles[0])
	}
	preview := cloneContext(args)
	preview["resolve_only"] = true
	preview["preview"] = true
	if preview["verified_vps_profiles"] == nil || args["verified_vps_profiles"] == nil {
		t.Fatalf("profile missing from preview/frozen args: preview=%#v args=%#v", preview, args)
	}
	if _, found := args["normalized_value"]; found {
		t.Fatalf("B4 runtime args contain final normalized value: %#v", args)
	}
}
