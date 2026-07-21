package pluginvps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/spal"
)

const testInstallationHash = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const testSurfaceHash = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func testDocument() Document {
	return Document{
		SchemaVersion: SchemaVersion,
		Plugin: PluginIdentity{
			Manufacturer:     "Example Audio",
			Name:             "Example EQ",
			Format:           "VST3",
			Version:          "1.0.0",
			InstallPath:      `C:\Program Files\Common Files\VST3\Example EQ.vst3`,
			InstallationHash: testInstallationHash,
		},
		Capability: EqualizerCapability,
		Bindings: spal.EQV2Binding{
			ConformedSchemas: []string{spal.EQBandPatchControlID},
			Bands: map[string]spal.EQV2BandBinding{
				"b1": {
					ComponentID:   "b1",
					Enabled:       spal.ParameterBinding{ParameterID: "enabled", Unit: "toggle", Min: 0, Max: 1, Scale: "linear"},
					ResponseShape: spal.EnumParameterBinding{ParameterID: "shape", Values: map[string]float64{"bell": 0, "notch": 1}},
					FrequencyHz:   spal.ParameterBinding{ParameterID: "frequency", Unit: "Hz", Min: 20, Max: 20000, Scale: "log"},
					GainDB:        spal.ParameterBinding{ParameterID: "gain", Unit: "dB", Min: -18, Max: 18, Scale: "linear"},
					Q:             spal.ParameterBinding{ParameterID: "q", Unit: "Q", Min: 0.1, Max: 12, Scale: "log"},
				},
			},
		},
	}
}

func verifiedTestDocument() Document {
	d := testDocument()
	d.Verified = true
	d.Verification = &Verification{
		InstallationHash:     d.Plugin.InstallationHash,
		ParameterSurfaceHash: testSurfaceHash,
		VerifiedAt:           time.Date(2026, time.July, 21, 2, 0, 0, 0, time.UTC),
		WorkerProtocol:       "test",
		Checks:               13,
	}
	return d
}

func TestDocumentValidateRequiresCompleteVerifiedStamp(t *testing.T) {
	d := testDocument()
	if err := d.Validate(false); err != nil {
		t.Fatalf("draft validation failed: %v", err)
	}
	if err := d.Validate(true); err == nil || !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("verified validation error = %v", err)
	}
	d.Verified = true
	d.Verification = &Verification{InstallationHash: testInstallationHash, ParameterSurfaceHash: "sha256:bad", VerifiedAt: time.Now()}
	if err := d.Validate(false); err == nil || !strings.Contains(err.Error(), "invalid verification stamp") {
		t.Fatalf("bad stamp validation error = %v", err)
	}
	d = verifiedTestDocument()
	d.Verification.InstallationHash = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if err := d.Validate(true); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale stamp validation error = %v", err)
	}
}

func TestRegistryLoadsOnlyVerifiedDocuments(t *testing.T) {
	dir := t.TempDir()
	if err := Save(filepath.Join(dir, "draft.vps.json"), testDocument()); err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(dir, "verified.vps.json"), verifiedTestDocument()); err != nil {
		t.Fatal(err)
	}
	bad := verifiedTestDocument()
	bad.Verification.ParameterSurfaceHash = "sha256:bad"
	raw, _ := json.Marshal(bad)
	if err := os.WriteFile(filepath.Join(dir, "bad.vps.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	registry, warnings := LoadDirectory(dir)
	if got := len(registry.Documents()); got != 1 {
		t.Fatalf("verified documents = %d, want 1; warnings=%v", got, warnings)
	}
	if got := len(warnings); got != 2 {
		t.Fatalf("warnings = %d, want draft and bad stamp warnings: %v", got, warnings)
	}
}

func TestRuntimeProfileProjectsDisplayDomainWithoutFinalNormalizedValue(t *testing.T) {
	doc := verifiedTestDocument()
	band := doc.Bindings.Bands["b1"]
	band.ResponseShape.DisplayLabels = map[string]string{"bell": "Bell", "notch": "Band Stop"}
	doc.Bindings.Bands["b1"] = band
	profile := doc.RuntimeProfile()
	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"normalized_value", "normalised_value"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("runtime profile contains forbidden %q: %s", forbidden, text)
		}
	}
	groups, ok := profile["groups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("groups = %#v", profile["groups"])
	}
	params := groups[0].(map[string]any)["params"].(map[string]any)
	frequency := params["frequency"].(map[string]any)
	if frequency["parameter_id"] != "frequency" {
		t.Fatalf("frequency parameter = %#v", frequency)
	}
	domain := frequency["display_domain"].(map[string]any)
	if domain["unit"] != "Hz" || domain["scale"] != "log" || domain["min"] != float64(20) || domain["max"] != float64(20000) {
		t.Fatalf("frequency display domain = %#v", domain)
	}
	shape := params["response_shape"].(map[string]any)
	values := shape["verified_values"].(map[string]float64)
	if values["Bell"] != 0 || values["Band Stop"] != 1 {
		t.Fatalf("verified display enum values = %#v", values)
	}
	if _, leaked := values["notch"]; leaked {
		t.Fatalf("semantic alias leaked into C++ verified label table: %#v", values)
	}
}

func TestDraftFromSurfaceGeneratesDataDrivenBandWithoutPluginSpecialCase(t *testing.T) {
	var surface SurfaceSnapshot
	surface.PluginIdentity.Manufacturer = "New Vendor"
	surface.PluginIdentity.Name = "Brand New EQ"
	surface.PluginIdentity.Format = "VST3"
	surface.PluginIdentity.Version = "0.1"
	surface.PluginIdentity.InstallPath = `C:\VST3\Brand New EQ.vst3`
	surface.PluginIdentity.Fingerprint.Installation = testInstallationHash
	minFreq, maxFreq := 20.0, 20000.0
	minGain, maxGain := -24.0, 24.0
	minQ, maxQ := 0.1, 12.0
	surface.Parameters = []SurfaceParameter{
		{ID: "1", Name: "Band Enable", Unit: "toggle", HostControllable: true},
		{ID: "2", Name: "Band Shape", HostControllable: true, Observed: map[string]any{"display_choices": []string{"Bell", "Notch"}}},
		{ID: "3", Name: "Band Frequency", Unit: "Hz", HostControllable: true},
		{ID: "4", Name: "Band Gain", Unit: "dB", HostControllable: true},
		{ID: "5", Name: "Band Q", Unit: "Q", HostControllable: true},
	}
	surface.Parameters[2].DisplayDomain.Min, surface.Parameters[2].DisplayDomain.Max, surface.Parameters[2].DisplayDomain.Scale = &minFreq, &maxFreq, "log"
	surface.Parameters[3].DisplayDomain.Min, surface.Parameters[3].DisplayDomain.Max, surface.Parameters[3].DisplayDomain.Scale = &minGain, &maxGain, "linear"
	surface.Parameters[4].DisplayDomain.Min, surface.Parameters[4].DisplayDomain.Max, surface.Parameters[4].DisplayDomain.Scale = &minQ, &maxQ, "log"
	doc, err := DraftFromSurface(surface)
	if err != nil {
		t.Fatal(err)
	}
	band := doc.Bindings.Bands["b1"]
	if band.FrequencyHz.ParameterID != "3" || band.GainDB.ParameterID != "4" || band.Q.ParameterID != "5" {
		t.Fatalf("draft binding = %#v", band)
	}
	if band.ResponseShape.Values["Bell"] != 0 || band.ResponseShape.Values["Notch"] != 1 {
		t.Fatalf("draft enum values = %#v", band.ResponseShape.Values)
	}
	if doc.Verified || doc.Verification != nil {
		t.Fatalf("draft must not self-verify: %#v", doc)
	}
}

func TestHashSurfaceSortsParameterIDs(t *testing.T) {
	if HashSurface([]string{"q", "gain", "frequency"}) != HashSurface([]string{"frequency", "q", "gain"}) {
		t.Fatal("surface hash depends on input order")
	}
}
