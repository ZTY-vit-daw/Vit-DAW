package experiment

import (
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// TestD1S1AdmittedDomainsAreProtocolVocabulary seals the two-source
// convergence between the protocol layer and this domain table: every
// ActionDomain string comes from the agentprotocol vocabulary constants, so a
// table row naming a domain the protocol does not admit fails here at test
// time instead of dying at the protocol gate at runtime with
// "unsupported action_domain" (the D2-1/D2-1.5 not_exercised failure).
func TestD1S1AdmittedDomainsAreProtocolVocabulary(t *testing.T) {
	specs := D1S1DomainSpecs()
	admitted := D1S1AdmittedDomains()
	if len(specs) != len(admitted) {
		t.Fatalf("spec table and admitted-domain list diverged: %d specs vs %d domains", len(specs), len(admitted))
	}
	seen := make(map[string]bool, len(specs))
	for i, spec := range specs {
		if seen[spec.ActionDomain] {
			t.Fatalf("domain %q registered more than once in the domain table", spec.ActionDomain)
		}
		seen[spec.ActionDomain] = true
		if admitted[i] != spec.ActionDomain {
			t.Fatalf("admitted-domain list drifted from the spec table at %d: %q vs %q", i, admitted[i], spec.ActionDomain)
		}
		if !agentprotocol.IsImprovementActionDomain(spec.ActionDomain) {
			t.Fatalf("admitted domain %q is absent from the agentprotocol action_domain vocabulary; add it there (as a constant) before admitting it here", spec.ActionDomain)
		}
	}
}
