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

// fixtureGateExpanderPlugin mirrors the FAM5-S1 whitelist section: one shared
// range (attenuation floor) parameter (FabFilter Pro-G, pluginprobe
// 2026-09-02), no ch pair.
func fixtureGateExpanderPlugin() GateExpanderPlugin {
	return GateExpanderPlugin{
		PluginName:       "Fixture Gate",
		Manufacturer:     "Example",
		Format:           "VST3",
		PluginIdentifier: "fixture-gate",
		PluginPath:       "/plugins/example-gate.vst3",
		RangeParamID:     "range_shared",
	}
}

func TestLoadParsesV5FileWithGateExpanderSectionRoundTrip(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		Limiter:       LimiterPlugins{fixtureLimiterPlugin()},
		GateExpander:  GateExpanderPlugins{fixtureGateExpanderPlugin()},
	}
	path := writeFixture(t, "whitelist_v5_gate.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, whitelist) {
		t.Fatalf("loaded=%+v want=%+v", loaded, whitelist)
	}
}

func TestLoadAllowsV5FileWithOnlyGateExpanderSection(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		GateExpander:  GateExpanderPlugins{fixtureGateExpanderPlugin()},
	}
	path := writeFixture(t, "whitelist_gate_only.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.GateExpander) != 1 || loaded.GateExpander[0].RangeParamID != "range_shared" {
		t.Fatalf("gate-only whitelist=%+v", loaded)
	}
}

func TestLoadRejectsMalformedGateExpanderSections(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*GateExpanderPlugin)
	}{
		{"empty_range_param_id", func(p *GateExpanderPlugin) { p.RangeParamID = "" }},
		{"empty_plugin_name", func(p *GateExpanderPlugin) { p.PluginName = "" }},
		{"empty_plugin_path", func(p *GateExpanderPlugin) { p.PluginPath = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plugin := fixtureGateExpanderPlugin()
			test.mutate(&plugin)
			data, err := json.Marshal(map[string]any{
				"schema_version": SchemaVersion,
				"gate_expander":  &plugin,
			})
			if err != nil {
				t.Fatal(err)
			}
			path := writeFixture(t, test.name+".json", string(data))
			_, err = Load(path)
			if err == nil {
				t.Fatalf("%s was accepted", test.name)
			}
			if errors.Is(err, ErrGateExpanderNotConfigured) {
				t.Fatalf("%s rejected as not-configured: %v", test.name, err)
			}
			if !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), path) {
				t.Fatalf("err=%v must contain \"invalid\" and path %s", err, path)
			}
		})
	}
	unknown := `{"schema_version":"` + SchemaVersion + `","gate_expander":{"plugin_name":"t","manufacturer":"m","format":"VST3","plugin_identifier":"i","plugin_path":"p","range_param_id":"r","surprise":1}}`
	path := writeFixture(t, "gate_unknown_field.json", unknown)
	if _, err := Load(path); err == nil {
		t.Fatal("unknown field in gate_expander section was accepted")
	}
}

func TestValidateGateExpanderAdmissionClasses(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture Gate.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-gate-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		GateExpander: GateExpanderPlugins{func() GateExpanderPlugin {
			plugin := fixtureGateExpanderPlugin()
			plugin.PluginPath = pluginPath
			return plugin
		}()},
	}
	subject := processorattestation.Subject{Name: "Fixture Gate", Manufacturer: "Example", Format: "VST3", Identifier: "fixture-gate", InstalledPath: pluginPath}
	promotedLibraryV2 := func(fingerprint string) processorattestation.LibraryV2 {
		now := time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)
		attestation, err := processorattestation.NewAttestationV2(processorattestation.IssueSpecV2{
			Subject:           subject,
			BinaryFingerprint: fingerprint,
			ProcessorFamily:   processorattestation.FamilyGateExpander,
			Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "attenuation_floor"}},
			Evidence: []processorattestation.EvidenceRef{{
				ReceiptID: "receipt-gate-1", Kind: "gate_expander_regression_receipt",
				SHA256:     "sha256:" + strings.Repeat("a", 64),
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

	if err := whitelist.ValidateGateExpanderAdmission(promotedLibraryV2(fingerprint), ""); err != nil {
		t.Fatalf("promoted matching binary rejected: %v", err)
	}

	staleErr := whitelist.ValidateGateExpanderAdmission(promotedLibraryV2("sha256:"+strings.Repeat("e", 64)), "")
	if staleErr == nil || !strings.HasPrefix(staleErr.Error(), "experiment plugin whitelist: gate_expander plugin is not PCA-promoted") ||
		!strings.Contains(staleErr.Error(), "Fixture Gate") || !strings.Contains(staleErr.Error(), "binary_fingerprint_changed") {
		t.Fatalf("fingerprint mismatch class wrong: %v", staleErr)
	}

	unknownErr := whitelist.ValidateGateExpanderAdmission(processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2}, "")
	if unknownErr == nil || !strings.Contains(unknownErr.Error(), "no_attestation") {
		t.Fatalf("unknown subject class wrong: %v", unknownErr)
	}

	unconfigured := Whitelist{}
	if err := unconfigured.ValidateGateExpanderAdmission(processorattestation.LibraryV2{}, ""); !errors.Is(err, ErrGateExpanderNotConfigured) {
		t.Fatalf("unconfigured gate_expander err=%v want ErrGateExpanderNotConfigured", err)
	}

	absent := Whitelist{
		SchemaVersion: SchemaVersion,
		GateExpander: GateExpanderPlugins{func() GateExpanderPlugin {
			plugin := fixtureGateExpanderPlugin()
			plugin.PluginPath = filepath.Join(t.TempDir(), "absent.vst3")
			return plugin
		}()},
	}
	fingerprintErr := absent.ValidateGateExpanderAdmission(promotedLibraryV2("sha256:"+strings.Repeat("9", 64)), "")
	if fingerprintErr == nil || !errors.Is(fingerprintErr, os.ErrNotExist) {
		t.Fatalf("fingerprint failure must wrap the filesystem error: %v", fingerprintErr)
	}
	if strings.HasPrefix(fingerprintErr.Error(), "experiment plugin whitelist: gate_expander plugin is not PCA-promoted") {
		t.Fatalf("fingerprint failure must stay distinguishable from PCA rejection: %v", fingerprintErr)
	}
}
