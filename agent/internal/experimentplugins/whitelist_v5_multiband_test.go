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

// fixtureMultibandPlugin mirrors the FAM6-S1 whitelist section: a per-band
// threshold parameter surface (Lindell MBC, pluginprobe 2026-09-02), one
// threshold id per band, no ch pair.
func fixtureMultibandPlugin() MultibandPlugin {
	return MultibandPlugin{
		PluginName:             "Fixture MBC",
		Manufacturer:           "Example",
		Format:                 "VST3",
		PluginIdentifier:       "fixture-mbc",
		PluginPath:             "/plugins/example-mbc.vst3",
		BandThresholdParamIDs:  []string{"low_threshold", "mid_threshold", "high_threshold"},
	}
}

func TestLoadParsesV5FileWithMultibandSectionRoundTrip(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		Limiter:       func() *LimiterPlugin { plugin := fixtureLimiterPlugin(); return &plugin }(),
		GateExpander:  func() *GateExpanderPlugin { plugin := fixtureGateExpanderPlugin(); return &plugin }(),
		Multiband:     func() *MultibandPlugin { plugin := fixtureMultibandPlugin(); return &plugin }(),
	}
	path := writeFixture(t, "whitelist_v5_multiband.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, whitelist) {
		t.Fatalf("loaded=%+v want=%+v", loaded, whitelist)
	}
}

func TestLoadAllowsV5FileWithOnlyMultibandSection(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		Multiband:     func() *MultibandPlugin { plugin := fixtureMultibandPlugin(); return &plugin }(),
	}
	path := writeFixture(t, "whitelist_multiband_only.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Multiband == nil || len(loaded.Multiband.BandThresholdParamIDs) != 3 {
		t.Fatalf("multiband-only whitelist=%+v", loaded)
	}
}

func TestLoadRejectsMalformedMultibandSections(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*MultibandPlugin)
	}{
		{"empty_band_threshold_param_ids", func(p *MultibandPlugin) { p.BandThresholdParamIDs = nil }},
		{"single_band_threshold_param_id", func(p *MultibandPlugin) { p.BandThresholdParamIDs = []string{"only"} }},
		{"blank_band_threshold_param_id", func(p *MultibandPlugin) { p.BandThresholdParamIDs = []string{"low", "  "} }},
		{"empty_plugin_identifier", func(p *MultibandPlugin) { p.PluginIdentifier = "" }},
		{"empty_plugin_name", func(p *MultibandPlugin) { p.PluginName = "" }},
		{"empty_plugin_path", func(p *MultibandPlugin) { p.PluginPath = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plugin := fixtureMultibandPlugin()
			test.mutate(&plugin)
			data, err := json.Marshal(map[string]any{
				"schema_version": SchemaVersion,
				"multiband":      &plugin,
			})
			if err != nil {
				t.Fatal(err)
			}
			path := writeFixture(t, test.name+".json", string(data))
			_, err = Load(path)
			if err == nil {
				t.Fatalf("%s was accepted", test.name)
			}
			if errors.Is(err, ErrMultibandNotConfigured) {
				t.Fatalf("%s rejected as not-configured: %v", test.name, err)
			}
			if !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), path) {
				t.Fatalf("err=%v must contain \"invalid\" and path %s", err, path)
			}
		})
	}
	unknown := `{"schema_version":"` + SchemaVersion + `","multiband":{"plugin_name":"t","manufacturer":"m","format":"VST3","plugin_identifier":"i","plugin_path":"p","band_threshold_param_ids":["a","b"],"surprise":1}}`
	path := writeFixture(t, "multiband_unknown_field.json", unknown)
	if _, err := Load(path); err == nil {
		t.Fatal("unknown field in multiband section was accepted")
	}
}

func TestValidateMultibandAdmissionClasses(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture MBC.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-mbc-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		Multiband: func() *MultibandPlugin {
			plugin := fixtureMultibandPlugin()
			plugin.PluginPath = pluginPath
			return &plugin
		}(),
	}
	subject := processorattestation.Subject{Name: "Fixture MBC", Manufacturer: "Example", Format: "VST3", Identifier: "fixture-mbc", InstalledPath: pluginPath}
	promotedLibraryV2 := func(fingerprint string) processorattestation.LibraryV2 {
		now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
		attestation, err := processorattestation.NewAttestationV2(processorattestation.IssueSpecV2{
			Subject:           subject,
			BinaryFingerprint: fingerprint,
			ProcessorFamily:   processorattestation.FamilyMultiband,
			Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "band_dynamics"}},
			Evidence: []processorattestation.EvidenceRef{{
				ReceiptID: "receipt-mbc-1", Kind: "multiband_regression_receipt",
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
		return processorattestation.LibraryV2{
			SchemaVersion: processorattestation.LibrarySchemaV2,
			UpdatedAt:     promotedAt,
			Attestations:  []processorattestation.AttestationV2{attestation},
		}
	}

	if err := whitelist.ValidateMultibandAdmission(promotedLibraryV2(fingerprint)); err != nil {
		t.Fatalf("promoted matching binary rejected: %v", err)
	}

	staleErr := whitelist.ValidateMultibandAdmission(promotedLibraryV2("sha256:" + strings.Repeat("e", 64)))
	if staleErr == nil || !strings.HasPrefix(staleErr.Error(), "experiment plugin whitelist: multiband plugin is not PCA-promoted") ||
		!strings.Contains(staleErr.Error(), "Fixture MBC") || !strings.Contains(staleErr.Error(), "binary_fingerprint_changed") {
		t.Fatalf("fingerprint mismatch class wrong: %v", staleErr)
	}

	unknownErr := whitelist.ValidateMultibandAdmission(processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2})
	if unknownErr == nil || !strings.Contains(unknownErr.Error(), "no_attestation") {
		t.Fatalf("unknown subject class wrong: %v", unknownErr)
	}

	unconfigured := Whitelist{}
	if err := unconfigured.ValidateMultibandAdmission(processorattestation.LibraryV2{}); !errors.Is(err, ErrMultibandNotConfigured) {
		t.Fatalf("unconfigured multiband err=%v want ErrMultibandNotConfigured", err)
	}

	absent := Whitelist{
		SchemaVersion: SchemaVersion,
		Multiband: func() *MultibandPlugin {
			plugin := fixtureMultibandPlugin()
			plugin.PluginPath = filepath.Join(t.TempDir(), "absent.vst3")
			return &plugin
		}(),
	}
	fingerprintErr := absent.ValidateMultibandAdmission(promotedLibraryV2("sha256:" + strings.Repeat("9", 64)))
	if fingerprintErr == nil || !errors.Is(fingerprintErr, os.ErrNotExist) {
		t.Fatalf("fingerprint failure must wrap the filesystem error: %v", fingerprintErr)
	}
	if strings.HasPrefix(fingerprintErr.Error(), "experiment plugin whitelist: multiband plugin is not PCA-promoted") {
		t.Fatalf("fingerprint failure must stay distinguishable from PCA rejection: %v", fingerprintErr)
	}
}
