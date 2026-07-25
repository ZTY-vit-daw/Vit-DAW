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

func TestLooksLikeEQFromParameters(t *testing.T) {
	cases := []struct {
		name   string
		params []ParameterInfo
		want   bool
	}{
		{
			name: "two band frequencies",
			params: []ParameterInfo{
				{Name: "Band 1 Frequency"},
				{Name: "Band 2 Frequency"},
			},
			want: true,
		},
		{
			name:   "single band frequency is not enough",
			params: []ParameterInfo{{Name: "Band 1 Frequency"}},
			want:   false,
		},
		{
			name: "compressor parameters",
			params: []ParameterInfo{
				{Name: "Threshold"},
				{Name: "Ratio"},
				{Name: "Attack"},
			},
			want: false,
		},
		{
			name: "shelf-only EQ has no numbered bands",
			params: []ParameterInfo{
				{Name: "Low Shelf Frequency"},
				{Name: "High Shelf Frequency"},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		if got := looksLikeEQFromParameters(tc.params); got != tc.want {
			t.Errorf("%s: looksLikeEQFromParameters = %v, want %v", tc.name, got, tc.want)
		}
	}
}
