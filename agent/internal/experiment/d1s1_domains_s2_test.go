package experiment

import (
	"strings"
	"testing"
)

// D2-1.5: the third admitted row must carry every execution descriptor so no
// execution-layer family branching is required; admitting it never relaxes
// the shared invariants (budget/attempts stay outside this table).
func TestD1S1TableCarriesBroadbandCompressionRow(t *testing.T) {
	spec, ok := D1S1SpecForAction("broadband_compression", "broadband_threshold_adjust")
	if !ok {
		t.Fatal("broadband_compression row missing from the domain table")
	}
	if spec.ActionIDSuffix != "_comp" || spec.CapabilityID != "static_mix.broadband_compression.v0" ||
		spec.AdmissionValueKey != "threshold_db" || spec.WriteBinding.PluginBound != true ||
		spec.WriteBinding.Channels != 2 || spec.WriteBinding.StubParamIDFormat != "" {
		t.Fatalf("compression descriptor drifted: %+v", spec)
	}
	if len(spec.ContractVersions) != 2 || spec.ObservationViewIDs == nil ||
		len(spec.ObservationViewIDs) != 1 || spec.ObservationViewIDs[0] != "track.time_dynamics" {
		t.Fatalf("compression views/contracts drifted: %+v", spec)
	}
	if !strings.Contains(spec.PromptParameterHint, "threshold_db") {
		t.Fatalf("prompt hint lost the threshold key: %q", spec.PromptParameterHint)
	}
	// Same equally-tight dose bounds as the sibling rows.
	if err := spec.ValidateDoseBounds("diagnostic", map[string]any{"threshold_db": 2.0}); err != nil {
		t.Fatalf("bound edge refused: %v", err)
	}
	for _, invalid := range []map[string]any{{"threshold_db": 0}, {"threshold_db": 2.01}, {}} {
		if err := spec.ValidateDoseBounds("retained", invalid); err == nil {
			t.Fatalf("invalid dose accepted: %+v", invalid)
		}
	}
	if !contains(D1S1AdmittedDomains(), "broadband_compression") {
		t.Fatalf("admitted domains=%+v missing broadband_compression", D1S1AdmittedDomains())
	}
	// The model-facing rule derives from the table automatically.
	options := make([]string, 0)
	for _, row := range D1S1DomainSpecs() {
		options = append(options, row.ActionDomain)
	}
	joined := strings.Join(options, ",")
	if !strings.Contains(joined, "broadband_compression") || !strings.Contains(joined, "static_eq") || !strings.Contains(joined, "track_gain") {
		t.Fatalf("domain enumeration drifted: %s", joined)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
