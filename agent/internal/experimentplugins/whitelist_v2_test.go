package experimentplugins

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/processorattestation"
)

// fixtureBroadbandCompressionPlugin mirrors the D2-1.5 whitelist section: one
// dual-channel compressor threshold binding.
func fixtureBroadbandCompressionPlugin() BroadbandCompressionPlugin {
	return BroadbandCompressionPlugin{
		PluginName:          "Fixture Comp",
		Manufacturer:        "Example",
		Format:              "VST3",
		PluginIdentifier:    "fixture-comp",
		PluginPath:          "/plugins/example-comp.vst3",
		ThresholdParamIDCH1: "thresh_a",
		ThresholdParamIDCH2: "thresh_b",
	}
}

func TestLoadParsesV2FileWithBothSectionsRoundTrip(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion:        SchemaVersion,
		StaticEQ:             func() *StaticEQPlugin { plugin := fixtureStaticEQPlugin(); return &plugin }(),
		BroadbandCompression: func() *BroadbandCompressionPlugin { plugin := fixtureBroadbandCompressionPlugin(); return &plugin }(),
	}
	path := writeFixture(t, "whitelist_v2.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, whitelist) {
		t.Fatalf("loaded=%+v want=%+v", loaded, whitelist)
	}
}

func TestLoadAllowsV2FileWithOnlyCompressionSection(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion:        SchemaVersion,
		BroadbandCompression: func() *BroadbandCompressionPlugin { plugin := fixtureBroadbandCompressionPlugin(); return &plugin }(),
	}
	path := writeFixture(t, "whitelist_compression_only.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.StaticEQ != nil || loaded.BroadbandCompression == nil {
		t.Fatalf("compression-only whitelist=%+v", loaded)
	}
	ch1, ch2, err := loaded.BroadbandThresholdParams()
	if err != nil || ch1 != "thresh_a" || ch2 != "thresh_b" {
		t.Fatalf("threshold params=%q/%q err=%v", ch1, ch2, err)
	}
}

func TestLoadRejectsMalformedCompressionSections(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*BroadbandCompressionPlugin)
		payload string
	}{
		{"empty_threshold_param_id_ch1", func(p *BroadbandCompressionPlugin) { p.ThresholdParamIDCH1 = "" }, ""},
		{"empty_threshold_param_id_ch2", func(p *BroadbandCompressionPlugin) { p.ThresholdParamIDCH2 = "" }, ""},
		{"empty_plugin_name", func(p *BroadbandCompressionPlugin) { p.PluginName = "" }, ""},
		{"identical_threshold_params", func(p *BroadbandCompressionPlugin) { p.ThresholdParamIDCH2 = p.ThresholdParamIDCH1 }, ""},
		{"unknown_field", func(*BroadbandCompressionPlugin) {}, `{"schema_version":"` + SchemaVersion + `","broadband_compression":{"plugin_name":"c","manufacturer":"m","format":"VST3","plugin_identifier":"i","plugin_path":"p","threshold_param_id_ch1":"a","threshold_param_id_ch2":"b","surprise":1}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var payload string
			if test.payload != "" {
				payload = test.payload
			} else {
				plugin := fixtureBroadbandCompressionPlugin()
				test.mutate(&plugin)
				data, err := json.Marshal(map[string]any{
					"schema_version":        SchemaVersion,
					"broadband_compression": &plugin,
				})
				if err != nil {
					t.Fatal(err)
				}
				payload = string(data)
			}
			path := writeFixture(t, test.name+".json", payload)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("%s was accepted", test.name)
			}
			if errors.Is(err, ErrNotConfigured) || errors.Is(err, ErrCompressionNotConfigured) {
				t.Fatalf("%s rejected as not-configured: %v", test.name, err)
			}
			if !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), path) {
				t.Fatalf("err=%v must contain \"invalid\" and path %s", err, path)
			}
		})
	}
}

func TestValidateCompressionAdmissionClasses(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture Comp.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-compressor-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		BroadbandCompression: func() *BroadbandCompressionPlugin {
			plugin := fixtureBroadbandCompressionPlugin()
			plugin.PluginPath = pluginPath
			return &plugin
		}(),
	}
	subject := processorattestation.Subject{Name: "Fixture Comp", Manufacturer: "Example", Format: "VST3", Identifier: "fixture-comp", InstalledPath: pluginPath}
	promotedLibrary := func(fingerprint string) processorattestation.Library {
		now := time.Date(2026, 8, 27, 22, 0, 0, 0, time.UTC)
		attestation, err := processorattestation.NewAttestation(processorattestation.IssueSpec{
			Subject:           subject,
			BinaryFingerprint: fingerprint,
			ProcessorFamily:   processorattestation.FamilyBroadbandCompressor,
			Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "transfer_severity"}},
			Evidence: []processorattestation.EvidenceRef{{
				ReceiptID: "receipt-comp-1", Kind: "compressor_regression_receipt",
				SHA256:     "sha256:" + strings.Repeat("d", 64),
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

	if err := whitelist.ValidateCompressionAdmission(promotedLibrary(fingerprint)); err != nil {
		t.Fatalf("promoted matching binary rejected: %v", err)
	}

	staleErr := whitelist.ValidateCompressionAdmission(promotedLibrary("sha256:" + strings.Repeat("e", 64)))
	if staleErr == nil || !strings.HasPrefix(staleErr.Error(), "experiment plugin whitelist: broadband_compression plugin is not PCA-promoted") ||
		!strings.Contains(staleErr.Error(), "Fixture Comp") || !strings.Contains(staleErr.Error(), "binary_fingerprint_changed") {
		t.Fatalf("fingerprint mismatch class wrong: %v", staleErr)
	}

	// An empty-but-valid library (what a missing store file decodes to in
	// production) leaves every subject without a record -> no_attestation.
	unknownErr := whitelist.ValidateCompressionAdmission(processorattestation.Library{SchemaVersion: processorattestation.LibrarySchema})
	if unknownErr == nil || !strings.Contains(unknownErr.Error(), "no_attestation") {
		t.Fatalf("unknown subject class wrong: %v", unknownErr)
	}

	unconfigured := Whitelist{}
	if err := unconfigured.ValidateCompressionAdmission(processorattestation.Library{}); !errors.Is(err, ErrCompressionNotConfigured) {
		t.Fatalf("unconfigured compression err=%v want ErrCompressionNotConfigured", err)
	}
	ch1, ch2, err := unconfigured.BroadbandThresholdParams()
	if err == nil || !errors.Is(err, ErrCompressionNotConfigured) || ch1 != "" || ch2 != "" {
		t.Fatalf("unconfigured threshold params=(%q,%q,%v)", ch1, ch2, err)
	}

	absent := Whitelist{
		SchemaVersion: SchemaVersion,
		BroadbandCompression: func() *BroadbandCompressionPlugin {
			plugin := fixtureBroadbandCompressionPlugin()
			plugin.PluginPath = filepath.Join(t.TempDir(), "absent.vst3")
			return &plugin
		}(),
	}
	fingerprintErr := absent.ValidateCompressionAdmission(promotedLibrary("sha256:" + strings.Repeat("9", 64)))
	if fingerprintErr == nil || !errors.Is(fingerprintErr, os.ErrNotExist) {
		t.Fatalf("fingerprint failure must wrap the filesystem error: %v", fingerprintErr)
	}
	if strings.HasPrefix(fingerprintErr.Error(), "experiment plugin whitelist: broadband_compression plugin is not PCA-promoted") {
		t.Fatalf("fingerprint failure must stay distinguishable from PCA rejection: %v", fingerprintErr)
	}
}

// fixtureDeEsserPlugin mirrors the FAM1-S1 whitelist section: one shared
// threshold parameter (FabFilter Pro-DS, pluginprobe 2026-08-31), no ch pair.
func fixtureDeEsserPlugin() DeEsserPlugin {
	return DeEsserPlugin{
		PluginName:       "Fixture DeEss",
		Manufacturer:     "Example",
		Format:           "VST3",
		PluginIdentifier: "fixture-deess",
		PluginPath:       "/plugins/example-deess.vst3",
		ThresholdParamID: "thresh_shared",
	}
}

func TestLoadParsesV3FileWithDeEsserSectionRoundTrip(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		StaticEQ:      func() *StaticEQPlugin { plugin := fixtureStaticEQPlugin(); return &plugin }(),
		BroadbandCompression: func() *BroadbandCompressionPlugin {
			plugin := fixtureBroadbandCompressionPlugin()
			return &plugin
		}(),
		DeEsser: func() *DeEsserPlugin { plugin := fixtureDeEsserPlugin(); return &plugin }(),
	}
	path := writeFixture(t, "whitelist_v3.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, whitelist) {
		t.Fatalf("loaded=%+v want=%+v", loaded, whitelist)
	}
}

func TestLoadAllowsV3FileWithOnlyDeEsserSection(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		DeEsser:       func() *DeEsserPlugin { plugin := fixtureDeEsserPlugin(); return &plugin }(),
	}
	path := writeFixture(t, "whitelist_deesser_only.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DeEsser == nil || loaded.DeEsser.ThresholdParamID != "thresh_shared" {
		t.Fatalf("de_esser-only whitelist=%+v", loaded)
	}
}

func TestLoadRejectsMalformedDeEsserSections(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DeEsserPlugin)
	}{
		{"empty_threshold_param_id", func(p *DeEsserPlugin) { p.ThresholdParamID = "" }},
		{"empty_plugin_name", func(p *DeEsserPlugin) { p.PluginName = "" }},
		{"empty_plugin_path", func(p *DeEsserPlugin) { p.PluginPath = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plugin := fixtureDeEsserPlugin()
			test.mutate(&plugin)
			data, err := json.Marshal(map[string]any{
				"schema_version": SchemaVersion,
				"de_esser":       &plugin,
			})
			if err != nil {
				t.Fatal(err)
			}
			path := writeFixture(t, test.name+".json", string(data))
			_, err = Load(path)
			if err == nil {
				t.Fatalf("%s was accepted", test.name)
			}
			if errors.Is(err, ErrDeEsserNotConfigured) {
				t.Fatalf("%s rejected as not-configured: %v", test.name, err)
			}
			if !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), path) {
				t.Fatalf("err=%v must contain \"invalid\" and path %s", err, path)
			}
		})
	}
	// DisallowUnknownFields keeps guarding the new section too.
	unknown := `{"schema_version":"` + SchemaVersion + `","de_esser":{"plugin_name":"d","manufacturer":"m","format":"VST3","plugin_identifier":"i","plugin_path":"p","threshold_param_id":"t","surprise":1}}`
	path := writeFixture(t, "deesser_unknown_field.json", unknown)
	if _, err := Load(path); err == nil {
		t.Fatal("unknown field in de_esser section was accepted")
	}
}

func TestValidateDeEsserAdmissionClasses(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture DeEss.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-deesser-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		DeEsser: func() *DeEsserPlugin {
			plugin := fixtureDeEsserPlugin()
			plugin.PluginPath = pluginPath
			return &plugin
		}(),
	}
	subject := processorattestation.Subject{Name: "Fixture DeEss", Manufacturer: "Example", Format: "VST3", Identifier: "fixture-deess", InstalledPath: pluginPath}
	promotedLibraryV2 := func(fingerprint string) processorattestation.LibraryV2 {
		now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
		attestation, err := processorattestation.NewAttestationV2(processorattestation.IssueSpecV2{
			Subject:           subject,
			BinaryFingerprint: fingerprint,
			ProcessorFamily:   processorattestation.FamilyDeEsser,
			Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "sibilance_reduction"}},
			Evidence: []processorattestation.EvidenceRef{{
				ReceiptID: "receipt-deess-1", Kind: "de_esser_regression_receipt",
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
		return processorattestation.LibraryV2{
			SchemaVersion: processorattestation.LibrarySchemaV2,
			UpdatedAt:     promotedAt,
			Attestations:  []processorattestation.AttestationV2{attestation},
		}
	}

	if err := whitelist.ValidateDeEsserAdmission(promotedLibraryV2(fingerprint)); err != nil {
		t.Fatalf("promoted matching binary rejected: %v", err)
	}

	staleErr := whitelist.ValidateDeEsserAdmission(promotedLibraryV2("sha256:" + strings.Repeat("e", 64)))
	if staleErr == nil || !strings.HasPrefix(staleErr.Error(), "experiment plugin whitelist: de_esser plugin is not PCA-promoted") ||
		!strings.Contains(staleErr.Error(), "Fixture DeEss") || !strings.Contains(staleErr.Error(), "binary_fingerprint_changed") {
		t.Fatalf("fingerprint mismatch class wrong: %v", staleErr)
	}

	// An empty-but-valid v2 library (what a missing store file decodes to in
	// production) leaves every subject without a record -> no_attestation.
	unknownErr := whitelist.ValidateDeEsserAdmission(processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2})
	if unknownErr == nil || !strings.Contains(unknownErr.Error(), "no_attestation") {
		t.Fatalf("unknown subject class wrong: %v", unknownErr)
	}

	unconfigured := Whitelist{}
	if err := unconfigured.ValidateDeEsserAdmission(processorattestation.LibraryV2{}); !errors.Is(err, ErrDeEsserNotConfigured) {
		t.Fatalf("unconfigured de_esser err=%v want ErrDeEsserNotConfigured", err)
	}

	absent := Whitelist{
		SchemaVersion: SchemaVersion,
		DeEsser: func() *DeEsserPlugin {
			plugin := fixtureDeEsserPlugin()
			plugin.PluginPath = filepath.Join(t.TempDir(), "absent.vst3")
			return &plugin
		}(),
	}
	fingerprintErr := absent.ValidateDeEsserAdmission(promotedLibraryV2("sha256:" + strings.Repeat("9", 64)))
	if fingerprintErr == nil || !errors.Is(fingerprintErr, os.ErrNotExist) {
		t.Fatalf("fingerprint failure must wrap the filesystem error: %v", fingerprintErr)
	}
	if strings.HasPrefix(fingerprintErr.Error(), "experiment plugin whitelist: de_esser plugin is not PCA-promoted") {
		t.Fatalf("fingerprint failure must stay distinguishable from PCA rejection: %v", fingerprintErr)
	}
}
