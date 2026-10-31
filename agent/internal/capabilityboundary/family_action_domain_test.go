package capabilityboundary

import (
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// M3-FS-GATE-EVIDENCE-WIRE-1 A2: the improvement-proposal action_domain is a
// family provenance. These tests pin the full enumeration round trip — every
// domain the protocol admits resolves to the boundary family vocabulary (or
// honestly stays "" for the domains that name no single family), and a
// proposal-only blocked turn no longer reports family_unresolved.

// Every accepted protocol action_domain must resolve deterministically; the
// map is total over the frozen enumeration (plugin/section_automation are the
// deliberate "" rows — no single family).
func TestFamilyFromImprovementActionDomainCoversProtocolVocabulary(t *testing.T) {
	expected := map[string]string{
		"track_gain": "volume", "clip_gain": "volume", "pan": "pan",
		"eq": "static_eq", "static_eq": "static_eq",
		"broadband_compression": "broadband_compressor", "compressor": "broadband_compressor",
		"limiter": "limiter", "gate_expander": "gate_expander", "de_esser": "de_esser",
		"transient_shaper": "transient_shaper", "multiband_dynamics": "multiband_dynamics",
		"reverb": "reverb", "delay": "delay",
		"plugin": "", "section_automation": "",
	}
	for _, domain := range agentprotocol.ImprovementActionDomains() {
		want, ok := expected[domain]
		if !ok {
			t.Fatalf("action_domain %q has no pinned family expectation — extend this test with the wiring decision", domain)
		}
		if got := FamilyFromImprovementActionDomain(domain); got != want {
			t.Fatalf("action_domain %q must resolve to %q, got %q", domain, want, got)
		}
	}
	// Case/whitespace tolerance on the wiring edge, unknown stays "".
	if got := FamilyFromImprovementActionDomain("  Track_Gain "); got != "volume" {
		t.Fatalf("normalization failed: got %q", got)
	}
	if got := FamilyFromImprovementActionDomain("warp_drive"); got != "" {
		t.Fatalf("unknown domain must stay unresolved, got %q", got)
	}
}

// The volume family (the slicebudget_1 g1_r1 shape: a track_gain proposal)
// must render honest absence, not family_unresolved: no PCA same/adjacent
// candidates exist because volume is not a plugin family — the three-link
// chain of the broken wire (family_unresolved +
// certification_entry_unavailable_for_family + no_loaded_instance_for_manual_guidance
// collapsing every class to the one root cause) converges to per-class truth.
func TestResolveVolumeFamilyConvergesToHonestSemantics(t *testing.T) {
	boundary := Resolve(Input{
		Code: "admission_gate_failed", Class: ClassFreeState,
		Family: FamilyFromImprovementActionDomain("track_gain"),
		Reason: "gate refusal", TrackID: "1010",
	})
	if boundary.Family != "volume" {
		t.Fatalf("family must resolve to volume, got %q", boundary.Family)
	}
	for _, reason := range boundary.UnavailableReasons {
		if reason == ReasonFamilyUnresolved {
			t.Fatalf("a resolved proposal family must not report family_unresolved: %v", boundary.UnavailableReasons)
		}
	}
	mustHold := map[string]bool{
		ReasonNoSameFamilyCandidate: false,
		ReasonNoAdjacentCandidate:   false,
		ReasonNativeUnavailable:     false,
	}
	for _, reason := range boundary.UnavailableReasons {
		if _, ok := mustHold[reason]; ok {
			mustHold[reason] = true
		}
	}
	for reason, held := range mustHold {
		if !held {
			t.Fatalf("volume family must disclose the honestly absent class %s, got %v", reason, boundary.UnavailableReasons)
		}
	}
	if len(boundary.NextActions) != 0 {
		t.Fatalf("a no-fact volume boundary must offer no fabricated actions, got %v", boundary.NextActions)
	}
	assertNoApproximateWrite(t, boundary)
}

// A plugin-family proposal domain keeps the full remediation semantics: with
// admitted candidates, loaded instances, native path and certification entry
// supplied, the resolved boundary mirrors the family the intent would have
// named — proving the new provenance feeds the same resolver contract.
func TestResolvePluginFamilyFromProposalDomain(t *testing.T) {
	family := FamilyFromImprovementActionDomain("compressor")
	if family != "broadband_compressor" {
		t.Fatalf("compressor domain must map to broadband_compressor, got %q", family)
	}
	boundary := Resolve(Input{
		Code: "admission_gate_failed", Class: ClassFreeState, Family: family,
		Reason: "gate refusal", TrackID: "1010",
		Candidates:    []Candidate{admittedCandidate(family, 0, "C1")},
		Instances:     []InstanceRef{loadedInstance("fx_1", family)},
		Native:        &NativeOption{Family: family},
		Certification: &CertificationEntry{Family: family, Endpoint: "/e", ConsentToken: "t"},
	})
	if len(boundary.NextActions) == 0 {
		t.Fatal("a fully supplied plugin-family boundary must render next actions")
	}
	assertNoApproximateWrite(t, boundary)
}
