package plugingrabber

import "testing"

// A freshly loaded plugin has no stored template_role — the kernel only writes
// it during an explicit learn step, and name-based inference does not recognise
// "TDR Nova" as an EQ. The summary must still be produced, otherwise every
// unprofiled EQ silently loses band guidance.
func TestBuildEQBandSummaryWorksWithoutTemplateRole(t *testing.T) {
	digest := eqSummaryTDRNovaDigest()
	digest.TemplateRole = "other"
	digest.PluginClass = ""

	summary := BuildEQBandSummary(digest)
	if summary == nil {
		t.Fatal("expected a summary from parameter structure alone, got nil")
	}
	if got := summary["eq_model"]; got != "fixed_slot_adjustable" {
		t.Fatalf("eq_model = %v, want fixed_slot_adjustable", got)
	}
}

// The structural gate replaces the old "two Band N Frequency parameters" check.
// It has to accept EQs that never use the word "Band" and reject effects whose
// parameters merely happen to be numbered.
func TestEQStructuralGate(t *testing.T) {
	cases := []struct {
		name   string
		params []ParameterInfo
		want   bool
	}{
		{
			name: "numbered parametric bands",
			params: []ParameterInfo{
				eqParam("1", "Band 1 Frequency", "100", eqRange(20, 20000, "log")),
				eqParam("2", "Band 1 Gain", "0", eqRange(-18, 18, "linear")),
				eqParam("3", "Band 2 Frequency", "1000", eqRange(20, 20000, "log")),
				eqParam("4", "Band 2 Gain", "0", eqRange(-18, 18, "linear")),
			},
			want: true,
		},
		{
			name: "one band is not a series",
			params: []ParameterInfo{
				eqParam("1", "Band 1 Frequency", "100", eqRange(20, 20000, "log")),
				eqParam("2", "Band 1 Gain", "0", eqRange(-18, 18, "linear")),
			},
			want: false,
		},
		{
			name: "compressor parameters",
			params: []ParameterInfo{
				eqParam("1", "Threshold", "-18.0", eqRange(-60, 0, "linear")),
				eqParam("2", "Ratio", "4.0", eqRange(1, 20, "linear")),
				eqParam("3", "Attack", "10.0", eqRange(0.1, 100, "log")),
			},
			want: false,
		},
		{
			name: "multiband compressor bands are not EQ bands",
			params: []ParameterInfo{
				eqParam("1", "Band 1 Threshold", "-18.0", eqRange(-60, 0, "linear")),
				eqParam("2", "Band 1 Ratio", "4.0", eqRange(1, 20, "linear")),
				eqParam("3", "Band 2 Threshold", "-18.0", eqRange(-60, 0, "linear")),
				eqParam("4", "Band 2 Ratio", "4.0", eqRange(1, 20, "linear")),
			},
			want: false,
		},
	}
	for _, tc := range cases {
		bands, order, indexFrequencies := collectEQBandsStructural(ParameterDigest{Parameters: tc.params})
		got := len(bands) > 0 && eqBandsLookStructural(bands, indexFrequencies)
		if got != tc.want {
			t.Errorf("%s: structural gate = %v, want %v (bands=%v)", tc.name, got, tc.want, order)
		}
	}
}
