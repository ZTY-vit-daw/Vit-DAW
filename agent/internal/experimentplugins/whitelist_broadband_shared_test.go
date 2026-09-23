package experimentplugins

// FIX-BROADBAND-SHARED-1 (PORT-PCA-FULL-CANDIDATES-1 decision A): a
// broadband_compression entry may pin the single shared threshold form
// (threshold_param_id, field name aligned with the de_esser v5 precedent,
// FAM1-S1 single-entry batch write semantics) beside the historical dual
// ch1/ch2 form. The two forms are mutually exclusive — both present or both
// absent fails closed — and historical dual entries keep loading
// byte-identically (the PC live Vertigo VSC-2 entry must not regress).

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/processorattestation"
)

// fixtureSharedBroadbandCompressionPlugin mirrors the mac-side Waves comp
// shape the decision A ruling unblocks: one shared Threshold parameter, no
// ch pair.
func fixtureSharedBroadbandCompressionPlugin() BroadbandCompressionPlugin {
	return BroadbandCompressionPlugin{
		PluginName:       "Fixture Shared Comp",
		Manufacturer:     "Example",
		Format:           "VST3",
		PluginIdentifier: "fixture-shared-comp",
		PluginPath:       "/plugins/example-shared-comp.vst3",
		ThresholdParamID: "thresh_shared",
	}
}

// red ① loader: a v6 single-form entry loads and round-trips, and the
// threshold accessor answers the shared id with an empty ch2.
func TestLoadParsesSharedThresholdBroadbandEntry(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion:        SchemaVersion,
		BroadbandCompression: BroadbandCompressionPlugins{fixtureSharedBroadbandCompressionPlugin()},
	}
	path := writeFixture(t, "whitelist_broadband_shared.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, whitelist) {
		t.Fatalf("loaded=%+v want=%+v", loaded, whitelist)
	}
	threshold, thresholdCH2, err := loaded.BroadbandThresholdParams("")
	if err != nil || threshold != "thresh_shared" || thresholdCH2 != "" {
		t.Fatalf("shared threshold params=(%q,%q,%v)", threshold, thresholdCH2, err)
	}
}

// red ① loader (mixed family): dual-form and shared-form candidates coexist
// in one v6 list; each pinned selection resolves with its own write shape.
func TestLoadParsesMixedFormBroadbandCandidates(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		BroadbandCompression: BroadbandCompressionPlugins{
			fixtureBroadbandCompressionPlugin(),
			fixtureSharedBroadbandCompressionPlugin(),
		},
	}
	path := writeFixture(t, "whitelist_broadband_mixed_forms.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, whitelist) {
		t.Fatalf("loaded=%+v want=%+v", loaded, whitelist)
	}
	dual, err := loaded.SelectBroadbandCompression("fixture-comp")
	if err != nil {
		t.Fatal(err)
	}
	if dualParam, dualCH2 := dual.ThresholdParamPair(); dualParam != "thresh_a" || dualCH2 != "thresh_b" {
		t.Fatalf("dual entry pair=(%q,%q)", dualParam, dualCH2)
	}
	shared, err := loaded.SelectBroadbandCompression("fixture-shared-comp")
	if err != nil {
		t.Fatal(err)
	}
	if sharedParam, sharedCH2 := shared.ThresholdParamPair(); sharedParam != "thresh_shared" || sharedCH2 != "" {
		t.Fatalf("shared entry pair=(%q,%q)", sharedParam, sharedCH2)
	}
}

// red ① loader (v5 valve): the legacy single-object wire form also accepts
// the shared-threshold entry and normalizes it to a one-element list.
func TestLoadParsesV5SharedThresholdBroadbandObject(t *testing.T) {
	raw := `{"schema_version":"` + SchemaVersionV5 + `","broadband_compression":{` +
		`"plugin_name":"Fixture Shared Comp","manufacturer":"Example","format":"VST3",` +
		`"plugin_identifier":"fixture-shared-comp","plugin_path":"/plugins/example-shared-comp.vst3",` +
		`"threshold_param_id":"thresh_shared"}}`
	path := writeFixture(t, "whitelist_v5_broadband_shared.json", raw)
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SchemaVersion != SchemaVersion {
		t.Fatalf("canonical schema version=%q want %q", loaded.SchemaVersion, SchemaVersion)
	}
	if want := fixtureSharedBroadbandCompressionPlugin(); len(loaded.BroadbandCompression) != 1 ||
		!reflect.DeepEqual(loaded.BroadbandCompression[0], want) {
		t.Fatalf("v5 shared entry content drifted: got=%+v want=%+v", loaded.BroadbandCompression, want)
	}
}

// red ③ ambiguity fail-closed: pinning both forms, neither form, or half a
// dual pair is corrupt and must not load. The historical dual-only rejections
// (half pair, identical pair) keep refusing.
func TestLoadRejectsAmbiguousBroadbandThresholdForms(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*BroadbandCompressionPlugin)
	}{
		{"both_forms_pinned", func(p *BroadbandCompressionPlugin) { p.ThresholdParamID = "thresh_shared" }},
		{"no_threshold_form_pinned", func(p *BroadbandCompressionPlugin) {
			p.ThresholdParamIDCH1 = ""
			p.ThresholdParamIDCH2 = ""
		}},
		{"dual_ch1_only", func(p *BroadbandCompressionPlugin) { p.ThresholdParamIDCH2 = "" }},
		{"dual_ch2_only", func(p *BroadbandCompressionPlugin) { p.ThresholdParamIDCH1 = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plugin := fixtureBroadbandCompressionPlugin()
			test.mutate(&plugin)
			whitelist := Whitelist{
				SchemaVersion:        SchemaVersion,
				BroadbandCompression: BroadbandCompressionPlugins{plugin},
			}
			path := writeFixture(t, "whitelist_broadband_ambiguous_"+test.name+".json", marshalOrPanic(whitelist))
			_, err := Load(path)
			if err == nil {
				t.Fatalf("%s was accepted", test.name)
			}
			if errors.Is(err, ErrNotConfigured) || errors.Is(err, ErrCompressionNotConfigured) {
				t.Fatalf("%s rejected as not-configured: %v", test.name, err)
			}
			if !strings.Contains(err.Error(), "threshold_param_id") {
				t.Fatalf("%s refusal must name the threshold form fields: %v", test.name, err)
			}
		})
	}
}

func promotedBroadbandSharedLibrary(t *testing.T, subject processorattestation.Subject, fingerprint string) processorattestation.Library {
	t.Helper()
	now := time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)
	attestation, err := processorattestation.NewAttestation(processorattestation.IssueSpec{
		Subject:           subject,
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   processorattestation.FamilyBroadbandCompressor,
		Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "transfer_severity"}},
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "receipt-comp-shared-1", Kind: "compressor_regression_receipt",
			SHA256:     "sha256:" + strings.Repeat("c", 64),
			ObservedAt: now.Add(-time.Hour),
		}},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	promotedAt := now.Add(time.Hour)
	attestation.Status = processorattestation.StatusPromoted
	attestation.StatusReason = "test_evidence_passed"
	attestation.PromotedAt = &promotedAt
	return processorattestation.Library{
		SchemaVersion: processorattestation.LibrarySchema,
		UpdatedAt:     promotedAt,
		Attestations:  []processorattestation.Attestation{attestation},
	}
}

// red ② admission: the PCA predicate runs against a single-form entry exactly
// like a dual one — the threshold form never enters the subject construction.
func TestValidateCompressionAdmissionAcceptsSharedFormEntry(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture Shared Comp.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-shared-comp-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	plugin := fixtureSharedBroadbandCompressionPlugin()
	plugin.PluginPath = pluginPath
	whitelist := Whitelist{
		SchemaVersion:        SchemaVersion,
		BroadbandCompression: BroadbandCompressionPlugins{plugin},
	}
	subject := processorattestation.Subject{Name: plugin.PluginName, Manufacturer: plugin.Manufacturer, Format: plugin.Format, Identifier: plugin.PluginIdentifier, InstalledPath: pluginPath}
	if err := whitelist.ValidateCompressionAdmission(promotedBroadbandSharedLibrary(t, subject, fingerprint), ""); err != nil {
		t.Fatalf("promoted shared-form entry rejected: %v", err)
	}
	emptyLibrary := processorattestation.Library{SchemaVersion: processorattestation.LibrarySchema}
	if err := whitelist.ValidateCompressionAdmission(emptyLibrary, ""); err == nil || !strings.Contains(err.Error(), "not PCA-promoted") {
		t.Fatalf("unpromoted shared-form entry must keep the not-PCA-promoted class: %v", err)
	}
}
