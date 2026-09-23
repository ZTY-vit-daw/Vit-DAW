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

// fixtureLimiterPlugin mirrors the FAM4-S1 whitelist section: one shared
// ceiling parameter (FabFilter Pro-L 2, pluginprobe 2026-09-02), no ch pair.
func fixtureLimiterPlugin() LimiterPlugin {
	return LimiterPlugin{
		PluginName:       "Fixture Limiter",
		Manufacturer:     "Example",
		Format:           "VST3",
		PluginIdentifier: "fixture-limiter",
		PluginPath:       "/plugins/example-limiter.vst3",
		CeilingParamID:   "ceiling_shared",
	}
}

func TestLoadParsesV5FileWithLimiterSectionRoundTrip(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion:   SchemaVersion,
		StaticEQ:        StaticEQPlugins{fixtureStaticEQPlugin()},
		DeEsser:         DeEsserPlugins{fixtureDeEsserPlugin()},
		TransientShaper: TransientShaperPlugins{fixtureTransientShaperPlugin()},
		Limiter:         LimiterPlugins{fixtureLimiterPlugin()},
	}
	path := writeFixture(t, "whitelist_v5.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, whitelist) {
		t.Fatalf("loaded=%+v want=%+v", loaded, whitelist)
	}
}

func TestLoadAllowsV5FileWithOnlyLimiterSection(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		Limiter:       LimiterPlugins{fixtureLimiterPlugin()},
	}
	path := writeFixture(t, "whitelist_limiter_only.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Limiter) != 1 || loaded.Limiter[0].CeilingParamID != "ceiling_shared" {
		t.Fatalf("limiter-only whitelist=%+v", loaded)
	}
}

func TestLoadRejectsMalformedLimiterSections(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*LimiterPlugin)
	}{
		{"empty_ceiling_param_id", func(p *LimiterPlugin) { p.CeilingParamID = "" }},
		{"empty_plugin_name", func(p *LimiterPlugin) { p.PluginName = "" }},
		{"empty_plugin_path", func(p *LimiterPlugin) { p.PluginPath = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plugin := fixtureLimiterPlugin()
			test.mutate(&plugin)
			data, err := json.Marshal(map[string]any{
				"schema_version": SchemaVersion,
				"limiter":        &plugin,
			})
			if err != nil {
				t.Fatal(err)
			}
			path := writeFixture(t, test.name+".json", string(data))
			_, err = Load(path)
			if err == nil {
				t.Fatalf("%s was accepted", test.name)
			}
			if errors.Is(err, ErrLimiterNotConfigured) {
				t.Fatalf("%s rejected as not-configured: %v", test.name, err)
			}
			if !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), path) {
				t.Fatalf("err=%v must contain \"invalid\" and path %s", err, path)
			}
		})
	}
	// DisallowUnknownFields keeps guarding the new section too.
	unknown := `{"schema_version":"` + SchemaVersion + `","limiter":{"plugin_name":"t","manufacturer":"m","format":"VST3","plugin_identifier":"i","plugin_path":"p","ceiling_param_id":"c","surprise":1}}`
	path := writeFixture(t, "limiter_unknown_field.json", unknown)
	if _, err := Load(path); err == nil {
		t.Fatal("unknown field in limiter section was accepted")
	}
}

func TestValidateLimiterAdmissionClasses(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture Limiter.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-limiter-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		Limiter:       LimiterPlugins{func() LimiterPlugin { plugin := fixtureLimiterPlugin(); plugin.PluginPath = pluginPath; return plugin }()},
	}
	subject := processorattestation.Subject{Name: "Fixture Limiter", Manufacturer: "Example", Format: "VST3", Identifier: "fixture-limiter", InstalledPath: pluginPath}
	promotedLibraryV2 := func(fingerprint string) processorattestation.LibraryV2 {
		now := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
		attestation, err := processorattestation.NewAttestationV2(processorattestation.IssueSpecV2{
			Subject:           subject,
			BinaryFingerprint: fingerprint,
			ProcessorFamily:   processorattestation.FamilyLimiter,
			Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "output_ceiling"}},
			Evidence: []processorattestation.EvidenceRef{{
				ReceiptID: "receipt-limiter-1", Kind: "limiter_regression_receipt",
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

	if err := whitelist.ValidateLimiterAdmission(promotedLibraryV2(fingerprint), ""); err != nil {
		t.Fatalf("promoted matching binary rejected: %v", err)
	}

	staleErr := whitelist.ValidateLimiterAdmission(promotedLibraryV2("sha256:"+strings.Repeat("e", 64)), "")
	if staleErr == nil || !strings.HasPrefix(staleErr.Error(), "experiment plugin whitelist: limiter plugin is not PCA-promoted") ||
		!strings.Contains(staleErr.Error(), "Fixture Limiter") || !strings.Contains(staleErr.Error(), "binary_fingerprint_changed") {
		t.Fatalf("fingerprint mismatch class wrong: %v", staleErr)
	}

	unknownErr := whitelist.ValidateLimiterAdmission(processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2}, "")
	if unknownErr == nil || !strings.Contains(unknownErr.Error(), "no_attestation") {
		t.Fatalf("unknown subject class wrong: %v", unknownErr)
	}

	unconfigured := Whitelist{}
	if err := unconfigured.ValidateLimiterAdmission(processorattestation.LibraryV2{}, ""); !errors.Is(err, ErrLimiterNotConfigured) {
		t.Fatalf("unconfigured limiter err=%v want ErrLimiterNotConfigured", err)
	}

	absent := Whitelist{
		SchemaVersion: SchemaVersion,
		Limiter: LimiterPlugins{func() LimiterPlugin {
			plugin := fixtureLimiterPlugin()
			plugin.PluginPath = filepath.Join(t.TempDir(), "absent.vst3")
			return plugin
		}()},
	}
	fingerprintErr := absent.ValidateLimiterAdmission(promotedLibraryV2("sha256:"+strings.Repeat("9", 64)), "")
	if fingerprintErr == nil || !errors.Is(fingerprintErr, os.ErrNotExist) {
		t.Fatalf("fingerprint failure must wrap the filesystem error: %v", fingerprintErr)
	}
	if strings.HasPrefix(fingerprintErr.Error(), "experiment plugin whitelist: limiter plugin is not PCA-promoted") {
		t.Fatalf("fingerprint failure must stay distinguishable from PCA rejection: %v", fingerprintErr)
	}
}
