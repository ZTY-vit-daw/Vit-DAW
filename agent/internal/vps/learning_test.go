package vps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func TestBuildInstallationFingerprintTracksPackageContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Learned EQ.vst3")
	if err := os.WriteFile(path, []byte("first package"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := BuildInstallationFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed package"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := BuildInstallationFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "sha256:") {
		t.Fatalf("installation fingerprint did not change: first=%q second=%q", first, second)
	}
}

func TestBuildFileFingerprintUsesRawContentGrammar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Observed EQ.vst3")
	contents := []byte("observed-worker-file-content")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := BuildFileFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(contents)
	want := "sha256:" + hex.EncodeToString(wantHash[:])
	if got != want {
		t.Fatalf("raw file fingerprint = %q, want %q", got, want)
	}
	packageFingerprint, err := BuildInstallationFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if packageFingerprint == got {
		t.Fatal("package fingerprint must remain distinguishable from raw worker file fingerprint")
	}
}

func TestLearningBuildIssuesOnlyAfterFullConformanceAndDerivesCatalog(t *testing.T) {
	input := learnedStaticEQInput(foundationTime)
	built, err := BuildLearningVPS(input, nil)
	if err != nil {
		t.Fatalf("BuildLearningVPS: %v", err)
	}
	if built.Document.Status != VPSStatusMapped || built.StaticEQBinding == nil || built.CandidateCredentialID == "" {
		t.Fatalf("learning draft = %#v", built)
	}
	if len(built.Document.ProviderCredentials) != 1 || built.Document.ProviderCredentials[0].Status != CredentialCandidate {
		t.Fatalf("candidate credential = %#v", built.Document.ProviderCredentials)
	}
	if err := built.Document.Validate(); err != nil {
		t.Fatalf("mapped learned VPS should validate: %v", err)
	}

	profile := StaticEQConformanceProfileV0()
	credential, err := built.Document.IssueCredential(ConformanceResult{
		CapabilityID:        profile.ID,
		Operations:          profile.RequiredOperations,
		Parameters:          profile.RequiredParameters,
		FilterTypes:         profile.RequiredFilterTypes,
		StaticEQBinding:     built.StaticEQBinding,
		WriteReadbackPassed: true,
		BoundaryTestsPassed: true,
		RollbackTestPassed:  true,
		EvidenceRefs:        []string{"test:write-readback", "test:boundary", "test:rollback"},
		CompletedAt:         foundationTime.Add(time.Minute),
	}, foundationTime.Add(time.Minute))
	if err != nil {
		t.Fatalf("IssueCredential: %v", err)
	}
	if credential.Status != CredentialVerified || built.Document.Status != VPSStatusVerified {
		t.Fatalf("issued credential/document = %#v %#v", credential, built.Document)
	}
	if credential.Conformance.StaticEQBinding == nil || credential.Conformance.StaticEQBinding.FrequencyParameterID != built.StaticEQBinding.FrequencyParameterID {
		t.Fatalf("Credential did not retain the conformed static EQ binding: %#v", credential.Conformance)
	}
	library, err := NewLibrary(filepath.Join(t.TempDir(), "vps_library.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := library.Upsert(built.Document); err != nil {
		t.Fatalf("persist issued VPS: %v", err)
	}
	catalog, err := library.Catalog()
	if err != nil || len(catalog.Entries) != 1 || catalog.Entries[0].CredentialID != credential.ID {
		t.Fatalf("catalog = %#v err=%v", catalog, err)
	}

	encoded, err := json.Marshal(built.Document)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "project-only-plugin-instance") {
		t.Fatalf("project plugin instance leaked into VPS: %s", encoded)
	}
}

func TestLearningUpdateStalesPriorCredentialBeforeFreshIssue(t *testing.T) {
	input := learnedStaticEQInput(foundationTime)
	first, err := BuildLearningVPS(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	profile := StaticEQConformanceProfileV0()
	if _, err := first.Document.IssueCredential(ConformanceResult{
		CapabilityID: profile.ID, Operations: profile.RequiredOperations, Parameters: profile.RequiredParameters,
		FilterTypes: profile.RequiredFilterTypes, StaticEQBinding: first.StaticEQBinding, WriteReadbackPassed: true, BoundaryTestsPassed: true,
		RollbackTestPassed: true, EvidenceRefs: []string{"test:full"}, CompletedAt: foundationTime,
	}, foundationTime); err != nil {
		t.Fatal(err)
	}
	second, err := BuildLearningVPS(input, &first.Document)
	if err != nil {
		t.Fatal(err)
	}
	if second.Document.Status != VPSStatusMapped || len(second.StateChanges) != 1 || second.StateChanges[0].To != CredentialStale {
		t.Fatalf("refreshed learning state = %#v", second)
	}
	verified, stale, candidate := 0, 0, 0
	for _, credential := range second.Document.ProviderCredentials {
		switch credential.Status {
		case CredentialVerified:
			verified++
		case CredentialStale:
			stale++
		case CredentialCandidate:
			candidate++
		}
	}
	if verified != 0 || stale != 1 || candidate != 1 {
		t.Fatalf("credential lifecycle after learning update = %#v", second.Document.ProviderCredentials)
	}
}

func TestStaticEQBindingAcceptsConfirmedBellEnumWithoutNumericDisplayRange(t *testing.T) {
	input := learnedStaticEQInput(foundationTime)
	typeMapping := input.Skill.Components[0].Params["type"]
	typeMapping.DisplayDomain.Min = nil
	typeMapping.DisplayDomain.Max = nil
	typeMapping.DisplayDomain.Text = "Bell"
	input.Skill.Components[0].Params["type"] = typeMapping

	binding, err := StaticEQBindingForSkill(input.Skill)
	if err != nil {
		t.Fatalf("StaticEQBindingForSkill: %v", err)
	}
	if binding.FilterTypeParameterID != "type" {
		t.Fatalf("binding = %#v", binding)
	}
}

func learnedStaticEQInput(at time.Time) LearningInput {
	frequencyMin, frequencyMax := 20.0, 20000.0
	gainMin, gainMax := -18.0, 18.0
	qMin, qMax := 0.1, 12.0
	toggleMin, toggleMax := 0.0, 1.0
	typeMin, typeMax := 0.0, 2.0
	mapping := func(id, label string, minimum, maximum *float64, unit, scale string) plugingrabber.PluginSkillParamMap {
		return plugingrabber.PluginSkillParamMap{
			ParamID: id, Label: label, Confirmed: true, Source: "user_demonstrated", Confidence: 1,
			DisplayDomain: &plugingrabber.PluginDisplayDomain{Text: label, Unit: unit, Min: minimum, Max: maximum, Scale: scale},
		}
	}
	skill := plugingrabber.PluginSkillDocument{
		SchemaVersion: plugingrabber.PluginSkillSchemaVersion,
		Identity: plugingrabber.PluginSkillIdentity{
			Manufacturer: "Example Audio", Name: "Learned Static EQ", Format: "VST3", Version: "1.0.0",
			ProfileKey: "learned_static_eq", Path: "C:/VST/Learned Static EQ.vst3", PluginID: "project-only-plugin-instance", ParamSignatureHash: "legacy-skill-signature",
		},
		Components: []plugingrabber.PluginSkillComponent{{
			ID: "b1", Role: "eq_band", Label: "Band 1",
			Params: map[string]plugingrabber.PluginSkillParamMap{
				"type":      mapping("type", "Filter type", &typeMin, &typeMax, "enum", "enum"),
				"frequency": mapping("frequency", "Frequency", &frequencyMin, &frequencyMax, "Hz", "log"),
				"gain":      mapping("gain", "Gain", &gainMin, &gainMax, "dB", "linear"),
				"q":         mapping("q", "Q", &qMin, &qMax, "Q", "log"),
				"enable":    mapping("enable", "Enable", &toggleMin, &toggleMax, "toggle", "linear"),
			},
		}},
	}
	parameter := func(id string, min, max float64, text, unit, scale string) plugingrabber.ParameterInfo {
		minimum, maximum := min, max
		return plugingrabber.ParameterInfo{
			ID: id, Min: min, Max: max, HostControllable: true,
			DisplayDomainCandidate: &plugingrabber.PluginDisplayDomain{Text: text, Unit: unit, Min: &minimum, Max: &maximum, Scale: scale},
		}
	}
	digest := plugingrabber.ParameterDigest{
		PluginName: "Learned Static EQ",
		PluginIdentity: map[string]any{
			"manufacturer": "Example Audio", "plugin_name": "Learned Static EQ", "plugin_format": "VST3", "version": "1.0.0",
			"profile_key": "learned_static_eq", "plugin_path": "C:/VST/Learned Static EQ.vst3",
		},
		Parameters: []plugingrabber.ParameterInfo{
			parameter("type", typeMin, typeMax, "Filter type", "enum", "enum"),
			parameter("frequency", frequencyMin, frequencyMax, "20..20000 Hz", "Hz", "log"),
			parameter("gain", gainMin, gainMax, "-18..18 dB", "dB", "linear"),
			parameter("q", qMin, qMax, "0.1..12 Q", "Q", "log"),
			parameter("enable", toggleMin, toggleMax, "Off / On", "toggle", "linear"),
		},
	}
	digest.Parameters[0].IsDiscrete = true
	digest.Parameters[1].NormalizedValue = 0.5
	digest.Parameters[2].NormalizedValue = 0.5
	digest.Parameters[3].NormalizedValue = 0.5
	digest.Parameters[4].IsBoolean = true
	digest.Parameters[4].NormalizedValue = 1.0
	return LearningInput{Skill: skill, Digest: digest, InstallationFingerprint: "binary:learned-static-eq", ObservedAt: at}
}
