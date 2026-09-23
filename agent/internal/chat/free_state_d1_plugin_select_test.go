package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/processorattestation"
)

// FIX-PLUGIN-SELECT-1 red suite: whitelist v6 candidate lists restore the
// model-owned plugin selection on the free-state D1 path. These tests pin the
// five card acceptance behaviors: (1) a pinned member of a multi-candidate
// family binds to exactly the selected entry, (2) a non-member pin is refused
// fail-closed, (3) a v5 single-candidate whitelist behaves exactly as before,
// (4) v5 file loading/round-trip lives in the experimentplugins suite,
// (5) one candidate without a pin keeps resolving to the only entry.

// d1PluginSelectFixture materializes a TWO-candidate static_eq whitelist on
// disk inside t.TempDir (never the developer's ~/.vit) with a PCA v1 library
// that promotes exactly the second candidate's binary.
func d1PluginSelectFixture(t *testing.T) (experimentplugins.Whitelist, string, processorattestation.Library) {
	t.Helper()
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "Fixture EQ.vst3")
	secondPath := filepath.Join(dir, "Second EQ.vst3")
	if err := os.WriteFile(firstPath, []byte("fixture-eq-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("second-eq-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	secondFingerprint, err := processorattestation.FingerprintPath(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	first := experimentplugins.StaticEQPlugin{
		PluginName:       "Fixture EQ",
		Manufacturer:     "Fixture",
		Format:           "VST3",
		PluginIdentifier: "fixture-eq",
		PluginPath:       firstPath,
		Bands: []experimentplugins.Band{
			{CenterHz: 100, GainParamIDCH1: "p100_c1", GainParamIDCH2: "p100_c2"},
			{CenterHz: 315, GainParamIDCH1: "p315_c1", GainParamIDCH2: "p315_c2"},
			{CenterHz: 4000, GainParamIDCH1: "p4000_c1", GainParamIDCH2: "p4000_c2"},
		},
	}
	second := experimentplugins.StaticEQPlugin{
		PluginName:       "Second EQ",
		Manufacturer:     "Other Vendor",
		Format:           "VST3",
		PluginIdentifier: "second-eq",
		PluginPath:       secondPath,
		Bands: []experimentplugins.Band{
			{CenterHz: 90, GainParamIDCH1: "q90_c1", GainParamIDCH2: "q90_c2"},
			{CenterHz: 900, GainParamIDCH1: "q900_c1", GainParamIDCH2: "q900_c2"},
		},
	}
	whitelist := experimentplugins.Whitelist{
		SchemaVersion: experimentplugins.SchemaVersion,
		StaticEQ:      experimentplugins.StaticEQPlugins{first, second},
	}
	subject := processorattestation.Subject{Name: "Second EQ", Manufacturer: "Other Vendor", Format: "VST3", Identifier: "second-eq", InstalledPath: secondPath}
	library := d1TablePromotedLibrary(t, subject, secondFingerprint)
	return whitelist, secondPath, library
}

// red ①: the model's pinned choice among multiple certified candidates is
// respected — the binding is exactly the selected entry (path, name,
// identifier, band params all come from the chosen plugin).
func TestResolveD1StaticEQBindingRespectsPinnedMultiCandidateChoice(t *testing.T) {
	whitelist, secondPath, library := d1PluginSelectFixture(t)
	d1TableOverrideLoaders(t, whitelist, nil, library, nil)
	binding, err := resolveD1StaticEQWhitelistBinding(map[string]any{
		"frequency_hz": 400.0, "gain_db": -1.0, "plugin_identifier": "second-eq",
	})
	if err != nil {
		t.Fatal(err)
	}
	if binding.Plugin.PluginIdentifier != "second-eq" || binding.Plugin.PluginPath != secondPath ||
		binding.Plugin.PluginName != "Second EQ" {
		t.Fatalf("binding must be the selected entry: %+v", binding.Plugin)
	}
	// 400 Hz maps to the SECOND entry's 90 Hz band under its own param ids
	// (|400-90|=310 beats |400-900|=500).
	if binding.Band.CenterHz != 90 || binding.Band.GainParamIDCH1 != "q90_c1" || binding.Band.GainParamIDCH2 != "q90_c2" {
		t.Fatalf("band must resolve inside the selected entry's bands: %+v", binding.Band)
	}
	// The normalized plugin-parameter binding carries the same selection.
	normalized, err := resolveD1PluginParamWhitelistBinding(map[string]any{
		"action_domain": "static_eq", "frequency_hz": 400.0, "plugin_identifier": "second-eq",
	})
	if err != nil {
		t.Fatal(err)
	}
	if normalized.PluginIdentifier != "second-eq" || normalized.PluginPath != secondPath ||
		normalized.ParamID != "q90_c1" || normalized.ParamIDCH2 != "q90_c2" {
		t.Fatalf("normalized binding must carry the selected entry: %+v", normalized)
	}
}

// red ②: a pin outside the candidate list is refused fail-closed.
func TestResolveD1StaticEQBindingRefusesNonMemberPin(t *testing.T) {
	whitelist, _, library := d1PluginSelectFixture(t)
	d1TableOverrideLoaders(t, whitelist, nil, library, nil)
	_, err := resolveD1StaticEQWhitelistBinding(map[string]any{
		"frequency_hz": 400.0, "plugin_identifier": "intruder-eq",
	})
	if err == nil {
		t.Fatal("non-member pin was admitted")
	}
	if !strings.Contains(err.Error(), "pinned plugin_identifier") || !strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("non-member refusal wording wrong: %v", err)
	}
	if !strings.Contains(err.Error(), "fixture-eq") || !strings.Contains(err.Error(), "second-eq") {
		t.Fatalf("non-member refusal must name the admitted identifiers: %v", err)
	}
}

// A multi-candidate family without a pin is ambiguous and must fail closed.
func TestResolveD1StaticEQBindingRequiresPinOnMultiCandidateFamily(t *testing.T) {
	whitelist, _, library := d1PluginSelectFixture(t)
	d1TableOverrideLoaders(t, whitelist, nil, library, nil)
	_, err := resolveD1StaticEQWhitelistBinding(map[string]any{"frequency_hz": 400.0})
	if err == nil {
		t.Fatal("ambiguous unpinned multi-candidate family was admitted")
	}
	if !strings.Contains(err.Error(), "requires a pinned plugin_identifier") || !strings.Contains(err.Error(), "2") {
		t.Fatalf("ambiguity refusal wording wrong: %v", err)
	}
}

// red ③ + ⑤: a single-candidate (v5-shaped) whitelist keeps today's behavior —
// no pin resolves to the only entry, a wrong pin keeps the historical refusal.
func TestResolveD1StaticEQBindingSingleCandidateUnchanged(t *testing.T) {
	fixture := d1TableWriteWhitelistFixture(t)
	subject := processorattestation.Subject{Name: "Fixture EQ", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-eq", InstalledPath: fixture.PluginPath}
	d1TableOverrideLoaders(t, fixture.Whitelist, nil,
		d1TablePromotedLibrary(t, subject, fixture.Fingerprint), nil)

	binding, err := resolveD1StaticEQWhitelistBinding(map[string]any{"frequency_hz": 400.0, "gain_db": -1.0})
	if err != nil {
		t.Fatal(err)
	}
	if binding.Plugin.PluginIdentifier != "fixture-eq" || binding.Band.GainParamIDCH1 != "p315_c1" {
		t.Fatalf("single candidate without pin must resolve as today: %+v", binding)
	}

	_, err = resolveD1StaticEQWhitelistBinding(map[string]any{"frequency_hz": 400.0, "plugin_identifier": "some-other"})
	if err == nil || !strings.Contains(err.Error(), "pinned plugin_identifier") || !strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("wrong pin on single candidate must keep the historical refusal: %v", err)
	}
}

// A multi-candidate v2-family section mirrors the same membership behavior
// (de_esser is the smallest single-parameter section).
func TestResolveD1DeEsserBindingMembership(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "First DS.vst3")
	secondPath := filepath.Join(dir, "Second DS.vst3")
	if err := os.WriteFile(firstPath, []byte("first-ds-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("second-ds-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	secondFingerprint, err := processorattestation.FingerprintPath(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	whitelist := experimentplugins.Whitelist{
		SchemaVersion: experimentplugins.SchemaVersion,
		DeEsser: experimentplugins.DeEsserPlugins{
			{PluginName: "First DS", Manufacturer: "Fixture", Format: "VST3", PluginIdentifier: "first-ds", PluginPath: firstPath, ThresholdParamID: "ds1_thr"},
			{PluginName: "Second DS", Manufacturer: "Other", Format: "VST3", PluginIdentifier: "second-ds", PluginPath: secondPath, ThresholdParamID: "ds2_thr"},
		},
	}
	subject := processorattestation.Subject{Name: "Second DS", Manufacturer: "Other", Format: "VST3", Identifier: "second-ds", InstalledPath: secondPath}
	library := d1DeEsserPromotedLibrary(t, subject, secondFingerprint)
	d1DeEsserOverrideLoaders(t, whitelist, nil, library, nil)

	binding, err := resolveD1DeEsserWhitelistBinding(map[string]any{
		"action_domain": "de_esser", "plugin_identifier": "second-ds",
	})
	if err != nil {
		t.Fatal(err)
	}
	if binding.PluginIdentifier != "second-ds" || binding.ParamID != "ds2_thr" || binding.PluginPath != secondPath {
		t.Fatalf("de_esser binding must carry the selected entry: %+v", binding)
	}

	_, err = resolveD1DeEsserWhitelistBinding(map[string]any{
		"action_domain": "de_esser", "plugin_identifier": "intruder-ds",
	})
	if err == nil || !strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("de_esser non-member refusal wrong: %v", err)
	}
}

// ---- proposal-time candidate disclosure --------------------------------------

// The disclosure is bounded: structural fields only (name / manufacturer /
// format / identifier plus the family capability face), never the local
// plugin_path; and it only exists for families with MORE THAN one candidate —
// single-candidate machines keep a zero-disclosure prompt.
func TestBuildFreeStatePluginCandidateDisclosure(t *testing.T) {
	single := d1TableWriteWhitelistFixture(t)
	d1TableOverrideLoaders(t, single.Whitelist, nil, processorattestation.Library{}, nil)
	if disclosure := buildFreeStatePluginCandidateDisclosure(); disclosure != nil {
		t.Fatalf("single-candidate whitelist must not disclose candidates: %+v", disclosure)
	}

	whitelist, _, _ := d1PluginSelectFixture(t)
	d1TableOverrideLoaders(t, whitelist, nil, processorattestation.Library{}, nil)
	disclosure := buildFreeStatePluginCandidateDisclosure()
	if disclosure == nil {
		t.Fatal("multi-candidate whitelist must disclose its candidate set")
	}
	if got := disclosure["schema_version"]; got != "free_state_plugin_candidate_disclosure.v1" {
		t.Fatalf("disclosure schema_version=%v", got)
	}
	families, _ := disclosure["families"].([]map[string]any)
	if len(families) != 1 {
		t.Fatalf("exactly the multi-candidate family is disclosed: %+v", disclosure)
	}
	row := families[0]
	if row["action_domain"] != "static_eq" {
		t.Fatalf("disclosed action_domain=%v", row["action_domain"])
	}
	if capability := strings.TrimSpace(row["capability"].(string)); capability == "" {
		t.Fatalf("family capability face missing: %+v", row)
	}
	candidates, _ := row["candidates"].([]map[string]any)
	if len(candidates) != 2 {
		t.Fatalf("both candidates must be disclosed: %+v", row)
	}
	identifiers := map[string]bool{}
	for _, candidate := range candidates {
		if name := strings.TrimSpace(candidate["name"].(string)); name == "" {
			t.Fatalf("candidate name missing: %+v", candidate)
		}
		if manufacturer := strings.TrimSpace(candidate["manufacturer"].(string)); manufacturer == "" {
			t.Fatalf("candidate manufacturer missing: %+v", candidate)
		}
		if identifier := strings.TrimSpace(candidate["identifier"].(string)); identifier == "" {
			t.Fatalf("candidate identifier missing: %+v", candidate)
		}
		if _, leaksPath := candidate["plugin_path"]; leaksPath {
			t.Fatalf("disclosure must not leak the machine-local plugin_path: %+v", candidate)
		}
		identifiers[strings.TrimSpace(candidate["identifier"].(string))] = true
	}
	if !identifiers["fixture-eq"] || !identifiers["second-eq"] {
		t.Fatalf("disclosed identifiers wrong: %+v", identifiers)
	}
}
