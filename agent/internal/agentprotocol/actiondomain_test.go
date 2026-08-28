package agentprotocol

import (
	"strings"
	"testing"
)

// TestImprovementActionDomainVocabularyByteCompat seals the wire vocabulary:
// these literals are serialized into pending candidates, journals, and model
// handoffs, so any change to an existing value is a protocol break. The list
// is a required subset — new domains may join the vocabulary, existing ones
// may never drift or disappear.
func TestImprovementActionDomainVocabularyByteCompat(t *testing.T) {
	frozen := []string{
		"track_gain",
		"clip_gain",
		"pan",
		"eq",
		"static_eq",
		"broadband_compression",
		"compressor",
		"limiter",
		"gate_expander",
		"de_esser",
		"transient_shaper",
		"multiband_dynamics",
		"plugin",
	}
	vocabulary := ImprovementActionDomains()
	seen := make(map[string]int, len(vocabulary))
	for _, domain := range vocabulary {
		seen[domain]++
	}
	for _, domain := range frozen {
		if seen[domain] != 1 {
			t.Fatalf("frozen action_domain %q missing from or duplicated in the vocabulary (count=%d)", domain, seen[domain])
		}
	}
	if len(vocabulary) != len(seen) {
		t.Fatalf("vocabulary contains duplicate entries: %+v", vocabulary)
	}
}

// TestImprovementActionDomainVocabularyAdmittedByValidate ties the
// enumeration to the actual protocol gate: every vocabulary value must pass
// Validate, with the same normalization Validate has always applied, and
// near-miss values must stay rejected.
func TestImprovementActionDomainVocabularyAdmittedByValidate(t *testing.T) {
	for _, domain := range ImprovementActionDomains() {
		proposal := ImprovementProposal{
			SchemaVersion:     ImprovementProposalSchema,
			Target:            map[string]any{"kind": "track", "id": "vox"},
			EvidenceRefs:      []string{"obs_1"},
			ImprovementIntent: "intent",
			Hypothesis:        "hypothesis",
			ExpectedEffect:    "effect",
			ActionDomain:      domain,
			ActionKind:        "bounded_treatment",
			Confidence:        0.5,
		}
		if err := proposal.Validate(); err != nil {
			t.Fatalf("vocabulary domain %q rejected by Validate: %v", domain, err)
		}
		if !IsImprovementActionDomain(domain) {
			t.Fatalf("vocabulary domain %q not recognized by IsImprovementActionDomain", domain)
		}
		if !IsImprovementActionDomain(" " + strings.ToUpper(domain) + " ") {
			t.Fatalf("normalized variant of %q rejected, normalization parity broken", domain)
		}
		if IsImprovementActionDomain(domain + "s") {
			t.Fatalf("near-miss domain %q must stay rejected", domain+"s")
		}
	}
	if IsImprovementActionDomain("") || IsImprovementActionDomain("  ") {
		t.Fatal("empty action_domain must stay rejected")
	}
}
