package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
)

// M2-TIMEFX-1 registration completeness contract (the free-state
// reachability face, MIX-CAP §3 pattern): the reverb/delay rows must resolve
// through EVERY registration point of the free-state experiment chain with
// one identity — protocol vocabulary, domain table, intent family support,
// PCA v2 families and role axes, routing labels, disclosure faces, terminal
// wording, and the plugin_grabber tool routing. Any single drift fails this
// contract instead of silently stranding the family outside the loop.

func TestM2TimeFXFreeStateChainRegistrationContract(t *testing.T) {
	for _, domain := range []struct {
		domain, kind, family string
		coverage             []string
	}{
		{d1ReverbDomain, d1ReverbKind, "reverb", []string{"decay_time", "pre_delay", "wet_balance"}},
		{d1DelayDomain, d1DelayKind, "delay", []string{"echo_time", "feedback_amount", "wet_balance"}},
	} {
		if !agentprotocol.IsImprovementActionDomain(domain.domain) {
			t.Fatalf("action_domain %q missing from the protocol vocabulary", domain.domain)
		}
		admitted := false
		for _, name := range experiment.D1S1AdmittedDomains() {
			if name == domain.domain {
				admitted = true
			}
		}
		if !admitted {
			t.Fatalf("action_domain %q missing from D1S1AdmittedDomains", domain.domain)
		}
		spec, ok := experiment.D1S1SpecForAction(domain.domain, domain.kind)
		if !ok {
			t.Fatalf("domain table row %s/%s missing", domain.domain, domain.kind)
		}
		if spec.SemanticIntentFamily != domain.family {
			t.Fatalf("%s semantic family=%q want %q", domain.domain, spec.SemanticIntentFamily, domain.family)
		}
		if len(spec.SemanticIntentCoverage) != len(domain.coverage) {
			t.Fatalf("%s coverage=%+v want %+v", domain.domain, spec.SemanticIntentCoverage, domain.coverage)
		}
		if !processorintent.IsSupportedFamily(spec.SemanticIntentFamily) {
			t.Fatalf("family %q not supported by processorintent", spec.SemanticIntentFamily)
		}
		// The D2-SEMINT1 derivation must decode through the processorintent
		// schema with the row's frozen family and coverage.
		intent, err := json.Marshal(map[string]any{
			"schema_version": processorintent.SchemaVersion, "status": processorintent.StatusResolved,
			"family": spec.SemanticIntentFamily, "intent": "admitted bounded experiment",
			"required_coverage": spec.SemanticIntentCoverage, "scope": processorintent.ScopeCurrentTrack,
			"control_mode": processorintent.ControlModeSemantic, "confidence": 1.0,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := processorintent.Decode(string(intent)); err != nil {
			t.Fatalf("%s frozen semantic intent does not decode: %v", domain.domain, err)
		}
		if !processorattestation.IsV2Family(spec.SemanticIntentFamily) {
			t.Fatalf("family %q not a PCA v2 family", spec.SemanticIntentFamily)
		}
		if _, ok := freeStatePluginCandidateCapabilities[domain.domain]; !ok {
			t.Fatalf("domain %q missing from the proposal-time candidate disclosure", domain.domain)
		}
		if wording := d1UserFacingWordingFor(domain.kind); wording.Parameter == "参数" {
			t.Fatalf("action kind %q fell back to the generic terminal wording", domain.kind)
		}
		if processorType := improvementProposalProcessorType(agentprotocol.ImprovementProposal{ActionDomain: domain.domain}); processorType != domain.domain {
			t.Fatalf("processor type mapping for %q = %q", domain.domain, processorType)
		}
	}
}

func TestM2TimeFXAttestationRoleAxes(t *testing.T) {
	cases := []struct {
		family, role, axis string
	}{
		{"reverb", "decay", "decay_time"},
		{"reverb", "size", "decay_time"},
		{"reverb", "pre_delay", "pre_delay"},
		{"reverb", "wet", "wet_balance"},
		{"reverb", "output_gain", "output_normalization"},
		{"delay", "time", "echo_time"},
		{"delay", "sync", "echo_time"},
		{"delay", "feedback", "feedback_amount"},
		{"delay", "mix", "wet_balance"},
	}
	for _, c := range cases {
		coverage := processorattestation.V2CoverageForRoles(c.family, []string{c.role})
		if len(coverage) != 1 || coverage[0].Axis != c.axis {
			t.Fatalf("role %s/%s mapped to %+v want axis %s", c.family, c.role, coverage, c.axis)
		}
	}
}

func TestM2TimeFXPluginGrabberToolRouting(t *testing.T) {
	for _, name := range []string{
		pluginGrabberInspectReverbTool, pluginGrabberApplyReverbTool,
		pluginGrabberInspectDelayTool, pluginGrabberApplyDelayTool,
	} {
		found := false
		for _, tool := range agentLoopPluginTools() {
			if tool == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("tool %q missing from agentLoopPluginTools", name)
		}
	}
	if !isPluginGrabberInspectReverbToolCall(planner.ToolCall{Tool: pluginGrabberInspectReverbTool}) ||
		!isPluginGrabberApplyReverbToolCall(planner.ToolCall{Tool: pluginGrabberApplyReverbTool}) ||
		!isPluginGrabberInspectDelayToolCall(planner.ToolCall{Tool: pluginGrabberInspectDelayTool}) ||
		!isPluginGrabberApplyDelayToolCall(planner.ToolCall{Tool: pluginGrabberApplyDelayTool}) {
		t.Fatal("plugin grabber executor matchers do not recognize the time-domain tools")
	}
}

type m2TimeFXWhitelistFixture struct {
	Whitelist   experimentplugins.Whitelist
	ReverbPath  string
	DelayPath   string
	Fingerprint string
}

// m2TimeFXWhitelistFixtureOnDisk materializes one whitelisted reverb and one
// whitelisted delay on disk inside t.TempDir (never the developer's ~/.vit),
// each admitted by a PCA v2 library record that promotes exactly its binary
// fingerprint.
func m2TimeFXWhitelistFixtureOnDisk(t *testing.T) m2TimeFXWhitelistFixture {
	t.Helper()
	dir := t.TempDir()
	reverbPath := filepath.Join(dir, "Fixture Reverb.vst3")
	delayPath := filepath.Join(dir, "Fixture Delay.vst3")
	if err := os.WriteFile(reverbPath, []byte("fixture-reverb-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(delayPath, []byte("fixture-delay-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(reverbPath)
	if err != nil {
		t.Fatal(err)
	}
	fixture := m2TimeFXWhitelistFixture{
		Whitelist: experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			Reverb: experimentplugins.ReverbPlugins{experimentplugins.ReverbPlugin{
				PluginName: "Fixture Reverb", Manufacturer: "Fixture", Format: "VST3",
				PluginIdentifier: "fixture-reverb", PluginPath: reverbPath,
				DecayParamID: "rev_decay", PreDelayParamID: "rev_predelay", WetParamID: "rev_wet",
			}},
			Delay: experimentplugins.DelayPlugins{experimentplugins.DelayPlugin{
				PluginName: "Fixture Delay", Manufacturer: "Fixture", Format: "VST3",
				PluginIdentifier: "fixture-delay", PluginPath: delayPath,
				TimeParamID: "dly_time", FeedbackParamID: "dly_feedback", WetParamID: "dly_wet",
			}},
		},
		ReverbPath:  reverbPath,
		DelayPath:   delayPath,
		Fingerprint: fingerprint,
	}
	return fixture
}

func m2TimeFXPromotedLibrary(t *testing.T, family, name, identifier, path string) processorattestation.LibraryV2 {
	t.Helper()
	fingerprint, err := processorattestation.FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	attestation, err := processorattestation.NewAttestationV2(processorattestation.IssueSpecV2{
		Subject:           processorattestation.Subject{Name: name, Manufacturer: "Fixture", Format: "VST3", Identifier: identifier, InstalledPath: path},
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   family,
		Coverage:          processorattestation.V2CoverageForRoles(family, []string{"decay", "pre_delay", "wet", "time", "feedback"}),
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "receipt-" + family + "-table-1", Kind: family + "_regression_receipt",
			SHA256:     "sha256:" + strings.Repeat("f", 64),
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

// m2TimeFXOverrideLoaders swaps the whitelist loader and both time-domain
// attestation readers; the other family companions stay untouched.
func m2TimeFXOverrideLoaders(t *testing.T, whitelist experimentplugins.Whitelist, library processorattestation.LibraryV2) {
	t.Helper()
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		return whitelist, nil
	}
	d1ReverbAttestationReader = func() (processorattestation.LibraryV2, error) { return library, nil }
	d1DelayAttestationReader = func() (processorattestation.LibraryV2, error) { return library, nil }
	t.Cleanup(func() {
		d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
			path, err := experimentplugins.DefaultPath()
			if err != nil {
				return experimentplugins.Whitelist{}, err
			}
			return experimentplugins.Load(path)
		}
		d1ReverbAttestationReader = func() (processorattestation.LibraryV2, error) {
			store, err := processorattestation.NewStoreV2("")
			if err != nil {
				return processorattestation.LibraryV2{}, err
			}
			library, _, err := store.Read()
			if err != nil {
				return processorattestation.LibraryV2{}, err
			}
			return library, nil
		}
		d1DelayAttestationReader = d1ReverbAttestationReader
	})
}

func TestResolveD1PluginParamBindingForTimeFX(t *testing.T) {
	fixture := m2TimeFXWhitelistFixtureOnDisk(t)
	library := processorattestation.LibraryV2{
		SchemaVersion: processorattestation.LibrarySchemaV2,
		Attestations: append(
			m2TimeFXPromotedLibrary(t, processorattestation.FamilyReverb, "Fixture Reverb", "fixture-reverb", fixture.ReverbPath).Attestations,
			m2TimeFXPromotedLibrary(t, processorattestation.FamilyDelay, "Fixture Delay", "fixture-delay", fixture.DelayPath).Attestations...),
	}
	m2TimeFXOverrideLoaders(t, fixture.Whitelist, library)

	reverbBinding, err := resolveD1PluginParamWhitelistBinding(map[string]any{"action_domain": d1ReverbDomain, "action_kind": d1ReverbKind, "decay_s": -0.3})
	if err != nil {
		t.Fatalf("reverb binding rejected: %v", err)
	}
	if reverbBinding.Section != d1ReverbDomain || reverbBinding.ParamID != "rev_decay" || reverbBinding.PluginIdentifier != "fixture-reverb" || reverbBinding.ParamIDCH2 != "" {
		t.Fatalf("reverb binding=%+v", reverbBinding)
	}
	delayBinding, err := resolveD1PluginParamWhitelistBinding(map[string]any{"action_domain": d1DelayDomain, "action_kind": d1DelayKind, "feedback_pct": 5})
	if err != nil {
		t.Fatalf("delay binding rejected: %v", err)
	}
	if delayBinding.Section != d1DelayDomain || delayBinding.ParamID != "dly_feedback" || delayBinding.PluginIdentifier != "fixture-delay" {
		t.Fatalf("delay binding=%+v", delayBinding)
	}
}

func TestResolveD1PluginParamBindingForTimeFXFailClosed(t *testing.T) {
	fixture := m2TimeFXWhitelistFixtureOnDisk(t)
	// No attestation promoted: both families must fail closed at the PCA gate.
	m2TimeFXOverrideLoaders(t, fixture.Whitelist, processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2})
	if _, err := resolveD1PluginParamWhitelistBinding(map[string]any{"action_domain": d1ReverbDomain}); err == nil || !strings.Contains(err.Error(), "PCA") {
		t.Fatalf("unpromoted reverb admitted: %v", err)
	}
	if _, err := resolveD1PluginParamWhitelistBinding(map[string]any{"action_domain": d1DelayDomain}); err == nil || !strings.Contains(err.Error(), "PCA") {
		t.Fatalf("unpromoted delay admitted: %v", err)
	}
	// No whitelist sections at all: distinct not-configured boundary.
	m2TimeFXOverrideLoaders(t, experimentplugins.Whitelist{SchemaVersion: experimentplugins.SchemaVersion}, processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2})
	if _, err := resolveD1PluginParamWhitelistBinding(map[string]any{"action_domain": d1ReverbDomain}); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unconfigured reverb admitted: %v", err)
	}
	if _, err := resolveD1PluginParamWhitelistBinding(map[string]any{"action_domain": d1DelayDomain}); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unconfigured delay admitted: %v", err)
	}
}
