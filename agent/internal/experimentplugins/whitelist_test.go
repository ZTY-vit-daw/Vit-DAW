package experimentplugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/processorattestation"
)

const fixturePluginPath = "/plugins/example.vst3"

func fixtureStaticEQPlugin() StaticEQPlugin {
	return StaticEQPlugin{
		PluginName:       "Example EQ",
		Manufacturer:     "Example",
		Format:           "VST3",
		PluginIdentifier: "example-eq",
		PluginPath:       fixturePluginPath,
		Bands: []Band{
			{CenterHz: 100, GainParamIDCH1: "gain_100_ch1", GainParamIDCH2: "gain_100_ch2"},
			{CenterHz: 1000, GainParamIDCH1: "gain_1000_ch1", GainParamIDCH2: "gain_1000_ch2"},
			{CenterHz: 10000, GainParamIDCH1: "gain_10000_ch1", GainParamIDCH2: "gain_10000_ch2"},
		},
	}
}

func fixtureWhitelist() Whitelist {
	return Whitelist{SchemaVersion: SchemaVersion, StaticEQ: func() *StaticEQPlugin {
		plugin := fixtureStaticEQPlugin()
		return &plugin
	}()}
}

func writeFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadParsesValidFileRoundTrip(t *testing.T) {
	path := writeFixture(t, "whitelist.json", marshalOrPanic(fixtureWhitelist()))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := fixtureWhitelist()
	if !reflect.DeepEqual(loaded, expected) {
		t.Fatalf("loaded=%+v want=%+v", loaded, expected)
	}
}

func TestLoadAllowsMissingStaticEQ(t *testing.T) {
	path := writeFixture(t, "whitelist.json", marshalOrPanic(Whitelist{SchemaVersion: SchemaVersion}))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SchemaVersion != SchemaVersion || loaded.StaticEQ != nil {
		t.Fatalf("empty whitelist=%+v", loaded)
	}
}

func TestLoadMissingFileIsErrNotConfigured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	_, err := Load(path)
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("missing file err=%v want ErrNotConfigured", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("err=%v must contain path %s", err, path)
	}
}

func TestLoadRejectsMalformedFiles(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"broken_json", `{"schema_version":`},
		{"unknown_top_level_field", fmt.Sprintf(
			`{"schema_version":%q,"unexpected_field":true}`, SchemaVersion)},
		{"unknown_static_eq_field", fmt.Sprintf(
			`{"schema_version":%q,"static_eq":{"plugin_name":"x","manufacturer":"m","format":"VST3","plugin_identifier":"i","plugin_path":"p","bands":[{"center_hz":100,"gain_param_id_ch1":"a","gain_param_id_ch2":"b"}],"surprise":1}}`,
			SchemaVersion)},
		{"wrong_schema_version", marshalOrPanic(Whitelist{
			SchemaVersion: "vit.free_state_experiment_plugins.v0", StaticEQ: func() *StaticEQPlugin {
				plugin := fixtureStaticEQPlugin()
				return &plugin
			}(),
		})},
		{"trailing_content", marshalOrPanic(fixtureWhitelist()) + "\n{}"},
		{"static_eq_without_bands_key", marshalOrPanic(Whitelist{SchemaVersion: SchemaVersion, StaticEQ: &StaticEQPlugin{
			PluginName: "Example EQ", Manufacturer: "Example", Format: "VST3",
			PluginIdentifier: "example-eq", PluginPath: fixturePluginPath,
		}})},
		{"empty_bands_array", marshalWhitelistWithBands(t, nil)},
		{"duplicate_center_hz", marshalWhitelistWithBands(t, []Band{
			{CenterHz: 100, GainParamIDCH1: "a", GainParamIDCH2: "b"},
			{CenterHz: 100, GainParamIDCH1: "c", GainParamIDCH2: "d"},
		})},
		{"out_of_order_bands", marshalWhitelistWithBands(t, []Band{
			{CenterHz: 1000, GainParamIDCH1: "a", GainParamIDCH2: "b"},
			{CenterHz: 100, GainParamIDCH1: "c", GainParamIDCH2: "d"},
		})},
		{"center_below_range", marshalWhitelistWithBands(t, []Band{
			{CenterHz: 19.5, GainParamIDCH1: "a", GainParamIDCH2: "b"},
		})},
		{"center_above_range", marshalWhitelistWithBands(t, []Band{
			{CenterHz: 20000, GainParamIDCH1: "a", GainParamIDCH2: "b"},
			{CenterHz: 20001, GainParamIDCH1: "c", GainParamIDCH2: "d"},
		})},
		{"duplicate_gain_param_id_ch1", marshalWhitelistWithBands(t, []Band{
			{CenterHz: 100, GainParamIDCH1: "shared", GainParamIDCH2: "b"},
			{CenterHz: 1000, GainParamIDCH1: "shared", GainParamIDCH2: "d"},
		})},
		{"empty_gain_param_id_ch1", marshalWhitelistWithBands(t, []Band{
			{CenterHz: 100, GainParamIDCH1: "", GainParamIDCH2: "b"},
		})},
		{"empty_gain_param_id_ch2", marshalWhitelistWithBands(t, []Band{
			{CenterHz: 100, GainParamIDCH1: "a", GainParamIDCH2: ""},
		})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeFixture(t, test.name+".json", test.payload)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("%s was accepted", test.name)
			}
			if errors.Is(err, ErrNotConfigured) {
				t.Fatalf("%s rejected as ErrNotConfigured: %v", test.name, err)
			}
			if !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), path) {
				t.Fatalf("err=%v must contain \"invalid\" and path %s", err, path)
			}
			if test.name == "wrong_schema_version" {
				if !strings.Contains(err.Error(), SchemaVersion) ||
					!strings.Contains(err.Error(), "vit.free_state_experiment_plugins.v0") {
					t.Fatalf("schema error=%v must contain expected and actual versions", err)
				}
			}
		})
	}
}

func TestLoadRejectsEmptyStaticEQFields(t *testing.T) {
	clears := []struct {
		name  string
		clear func(*StaticEQPlugin)
	}{
		{"plugin_name", func(p *StaticEQPlugin) { p.PluginName = "" }},
		{"manufacturer", func(p *StaticEQPlugin) { p.Manufacturer = "" }},
		{"format", func(p *StaticEQPlugin) { p.Format = "" }},
		{"plugin_identifier", func(p *StaticEQPlugin) { p.PluginIdentifier = "" }},
		{"plugin_path", func(p *StaticEQPlugin) { p.PluginPath = "" }},
	}
	for _, clear := range clears {
		t.Run(clear.name, func(t *testing.T) {
			whitelist := fixtureWhitelist()
			clear.clear(whitelist.StaticEQ)
			path := writeFixture(t, clear.name+".json", marshalOrPanic(whitelist))
			if _, err := Load(path); err == nil {
				t.Fatalf("empty %s was accepted", clear.name)
			} else if errors.Is(err, ErrNotConfigured) {
				t.Fatalf("empty %s rejected as ErrNotConfigured: %v", clear.name, err)
			}
		})
	}
}

// marshalWhitelistWithBands marshals a whitelist whose static_eq carries exactly these bands.
func marshalWhitelistWithBands(t *testing.T, bands []Band) string {
	t.Helper()
	plugin := fixtureStaticEQPlugin()
	plugin.Bands = bands
	return marshalOrPanic(Whitelist{SchemaVersion: SchemaVersion, StaticEQ: &plugin})
}

func marshalOrPanic(whitelist Whitelist) string {
	data, err := json.Marshal(whitelist)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestNearestStaticEQBandResolution(t *testing.T) {
	whitelist := fixtureWhitelist()
	tests := []struct {
		name        string
		frequencyHz float64
		wantCenter  float64
	}{
		{"exact_hit_lowest_band", 100, 100},
		{"exact_hit_middle_band", 1000, 1000},
		{"resolve_upward", 640, 1000},
		{"resolve_downward", 420, 100},
		{"tie_prefers_lower_band", 550, 100},
		{"no_clamping_below_range", 10, 100},
		{"no_clamping_above_range", 30000, 10000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			band, err := whitelist.NearestStaticEQBand(test.frequencyHz)
			if err != nil {
				t.Fatal(err)
			}
			if band.CenterHz != test.wantCenter {
				t.Fatalf("frequencyHz=%v band=%+v want center %v", test.frequencyHz, band, test.wantCenter)
			}
		})
	}
	unconfigured := Whitelist{}
	if _, err := unconfigured.NearestStaticEQBand(100); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("unconfigured err=%v want ErrNotConfigured", err)
	}
}

func promotedStaticEQLibrary(t *testing.T, subject processorattestation.Subject, fingerprint string) processorattestation.Library {
	t.Helper()
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	attestation, err := processorattestation.NewAttestation(processorattestation.IssueSpec{
		Subject:           subject,
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   processorattestation.FamilyStaticEQ,
		Coverage:          []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "receipt-1", Kind: "eq_regression_receipt",
			SHA256: "sha256:" + strings.Repeat("b", 64),
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

func admissionSubject(installedPath string) processorattestation.Subject {
	return processorattestation.Subject{
		Name: "Example EQ", Manufacturer: "Example", Format: "VST3",
		Identifier: "example-eq", InstalledPath: installedPath,
	}
}

func whitelistOnDisk(t *testing.T) (Whitelist, string) {
	t.Helper()
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Example.vst3")
	if err := os.WriteFile(pluginPath, []byte("example-eq-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	whitelist := fixtureWhitelist()
	whitelist.StaticEQ.PluginPath = pluginPath
	return whitelist, pluginPath
}

func TestValidateStaticEQAdmissionAcceptsPromotedMatchingBinary(t *testing.T) {
	whitelist, pluginPath := whitelistOnDisk(t)
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	lib := promotedStaticEQLibrary(t, admissionSubject(pluginPath), fingerprint)
	if err := whitelist.ValidateStaticEQAdmission(lib); err != nil {
		t.Fatalf("promoted matching binary rejected: %v", err)
	}
}

func admissionRejectionChecks(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("admission unexpectedly accepted")
	}
	const prefix = "experiment plugin whitelist: static_eq plugin is not PCA-promoted"
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("err=%v must start with %q", err, prefix)
	}
	if !strings.Contains(err.Error(), "Example EQ") {
		t.Fatalf("err=%v must contain the plugin name", err)
	}
}

func TestValidateStaticEQAdmissionRejectsFingerprintMismatch(t *testing.T) {
	whitelist, _ := whitelistOnDisk(t)
	staleFingerprint := "sha256:" + strings.Repeat("a", 64)
	err := whitelist.ValidateStaticEQAdmission(promotedStaticEQLibrary(t, admissionSubject(whitelist.StaticEQ.PluginPath), staleFingerprint))
	admissionRejectionChecks(t, err)
	if !strings.Contains(err.Error(), "binary_fingerprint_changed") {
		t.Fatalf("err=%v must contain the query reason", err)
	}
}

func TestValidateStaticEQAdmissionRejectsUnknownProcessor(t *testing.T) {
	whitelist, _ := whitelistOnDisk(t)
	lib := promotedStaticEQLibrary(t, processorattestation.Subject{
		Name: "Other EQ", Manufacturer: "Other", Format: "VST3",
		Identifier: "other-eq", InstalledPath: "/elsewhere.vst3",
	}, "sha256:"+strings.Repeat("c", 64))
	err := whitelist.ValidateStaticEQAdmission(lib)
	admissionRejectionChecks(t, err)
	if !strings.Contains(err.Error(), "no_attestation") {
		t.Fatalf("err=%v must contain the query reason", err)
	}
}

func TestValidateStaticEQAdmissionWrapsFingerprintFailure(t *testing.T) {
	whitelist := fixtureWhitelist()
	whitelist.StaticEQ.PluginPath = filepath.Join(t.TempDir(), "absent.vst3")
	err := whitelist.ValidateStaticEQAdmission(promotedStaticEQLibrary(t, processorattestation.Subject{
		Name: "Example EQ", Manufacturer: "Example", Format: "VST3",
		Identifier: "example-eq", InstalledPath: whitelist.StaticEQ.PluginPath,
	}, "sha256:"+strings.Repeat("d", 64)))
	if err == nil {
		t.Fatal("fingerprint failure was swallowed")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err=%v must wrap the underlying filesystem error", err)
	}
	if strings.HasPrefix(err.Error(), "experiment plugin whitelist: static_eq plugin is not PCA-promoted") {
		t.Fatalf("fingerprint failure must stay distinguishable from PCA rejection: %v", err)
	}
}

func TestValidateStaticEQAdmissionRequiresConfiguration(t *testing.T) {
	whitelist := Whitelist{}
	if err := whitelist.ValidateStaticEQAdmission(processorattestation.Library{}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("unconfigured err=%v want ErrNotConfigured", err)
	}
}
