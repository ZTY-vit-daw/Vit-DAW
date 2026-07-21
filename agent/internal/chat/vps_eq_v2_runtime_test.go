package chat

import "testing"

func TestMatchVPSEQV2ProviderCandidatesFallsBackToSoleInstanceOnStalePluginID(t *testing.T) {
	// Observed live: a project reload moved Pro-Q 3 from rack slot "1013" to
	// "1018". The model kept citing "1013" in capability_equalizer_plan and
	// capability_equalizer_inspect calls for several turns afterward, even
	// after a fresher plugin_id had already been reported back to it. With a
	// strict plugin_id match, every one of those calls reports
	// no_observed_eq_v2_provider_instance even though the track has exactly
	// one loaded plug-in and the intent is unambiguous.
	candidates := []spalReferenceEQProviderCandidate{
		{TrackID: "1007", TargetRef: "track:1007", PluginID: "1018", PluginName: "Pro-Q 3"},
	}
	got := matchVPSEQV2ProviderCandidates(candidates, "track:1007", "1013")
	if len(got) != 1 || got[0].PluginID != "1018" {
		t.Fatalf("expected fallback to the sole loaded instance, got %#v", got)
	}
}

func TestMatchVPSEQV2ProviderCandidatesDoesNotFallBackWithMultipleInstances(t *testing.T) {
	// Same stale-plugin_id scenario, but now two EQ-capable plug-ins are
	// loaded on the target track. There is no unambiguous single instance to
	// fall back to, so a stale/wrong plugin_id must still miss.
	candidates := []spalReferenceEQProviderCandidate{
		{TrackID: "1007", TargetRef: "track:1007", PluginID: "1014", PluginName: "TDR Nova"},
		{TrackID: "1007", TargetRef: "track:1007", PluginID: "1018", PluginName: "Pro-Q 3"},
	}
	got := matchVPSEQV2ProviderCandidates(candidates, "track:1007", "1013")
	if len(got) != 0 {
		t.Fatalf("expected no match when the target has multiple candidates, got %#v", got)
	}
}

func TestMatchVPSEQV2ProviderCandidatesPrefersExactPluginIDMatch(t *testing.T) {
	candidates := []spalReferenceEQProviderCandidate{
		{TrackID: "1007", TargetRef: "track:1007", PluginID: "1014", PluginName: "TDR Nova"},
		{TrackID: "1007", TargetRef: "track:1007", PluginID: "1018", PluginName: "Pro-Q 3"},
	}
	got := matchVPSEQV2ProviderCandidates(candidates, "track:1007", "1018")
	if len(got) != 1 || got[0].PluginName != "Pro-Q 3" {
		t.Fatalf("expected the exact plugin_id match, got %#v", got)
	}
}
