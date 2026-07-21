package chat

import (
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationruntime"
)

func TestSPALProviderRegistrationIntentOwnsNaturalLanguageRequest(t *testing.T) {
	message := "注册当前 TDR Nova 为 SPAL Reference EQ Provider：频段: band1，已确认静态 Bell"
	if got := inferProjectAwareCapability(message); got != spalReferenceEQProviderRegistrationCapabilityID {
		t.Fatalf("registration intent routed to %q", got)
	}
	request := parseSPALReferenceEQProviderRegistrationRequest(ChatRequest{
		Message: message,
		Context: map[string]any{"selected_plugin_id": "nova-1", "selected_plugin_track_id": "bass"},
	})
	if len(request.Missing) != 0 || request.PluginID != "nova-1" || request.TargetRef != "bass" || request.BandSlot != "band1" || !request.StaticBellConfirmed {
		t.Fatalf("registration parser lost explicit controls: %#v", request)
	}
}

func TestSPALProviderRegistrationUsesOnlyObservedTargetInstance(t *testing.T) {
	state := &kernel.VSPStateResult{LegacyState: map[string]any{"tracks": []any{
		map[string]any{
			"track_id": "bass", "name": "Bass", "plugins": []any{
				map[string]any{"plugin_id": "nova-bass", "plugin_name": "TDR Nova", "format": "VST3", "plugin_path": "C:/VST3/TDR Nova.vst3"},
			},
			"rack_nodes": []any{
				// The VSP projection can expose the same instance through both
				// views. It must not become an ambiguous double candidate.
				map[string]any{"plugin_item_id": "nova-bass", "plugin_name": "TDR Nova", "format": "VST3"},
			},
		},
		map[string]any{
			"track_id": "vocal", "name": "Vocal", "rack": map[string]any{"nodes": []any{
				map[string]any{"node_id": "nova-vocal", "name": "TDR Nova", "format": "VST3"},
			}},
		},
	}}}
	candidate, all, reason := selectObservedSPALReferenceEQProviderCandidate(state, spalReferenceEQProviderRegistrationRequest{TargetRef: "track:bass", BandSlot: "band1", StaticBellConfirmed: true})
	if reason != "" || candidate.PluginID != "nova-bass" || candidate.TrackID != "bass" || len(all) != 1 {
		t.Fatalf("observed target resolution=%#v candidates=%#v reason=%q", candidate, all, reason)
	}
	_, choices, reason := selectObservedSPALReferenceEQProviderCandidate(state, spalReferenceEQProviderRegistrationRequest{BandSlot: "band1", StaticBellConfirmed: true})
	if reason != "provider_instance_selection_required" || len(choices) != 2 {
		t.Fatalf("unscoped request should not choose a project instance: candidates=%#v reason=%q", choices, reason)
	}
	_, choices, reason = selectObservedSPALReferenceEQProviderCandidate(state, spalReferenceEQProviderRegistrationRequest{TargetRef: "track:bass", PluginID: "nova-bass", BandSlot: "band1", StaticBellConfirmed: true})
	if reason != "" || len(choices) != 1 {
		t.Fatalf("explicit instance selection failed: candidates=%#v reason=%q", choices, reason)
	}
}

func TestVPSProviderEnumerationDoesNotApplyTDRReferenceEQFilter(t *testing.T) {
	state := &kernel.VSPStateResult{LegacyState: map[string]any{"tracks": []any{
		map[string]any{
			"track_id": "mix", "rack": map[string]any{"nodes": []any{
				map[string]any{"plugin_id": "pro-q-3", "plugin_name": "Pro-Q 3", "format": "VST3", "plugin_path": "C:/VST3/FabFilter Pro-Q 3.vst3"},
				map[string]any{"plugin_id": "nova", "plugin_name": "TDR Nova", "format": "VST3", "plugin_path": "C:/VST3/TDR Nova.vst3"},
			}},
		},
	}}}
	all := observedVPSProviderCandidates(state)
	if len(all) != 2 {
		t.Fatalf("generic VST3 enumeration = %#v", all)
	}
	reference := observedSPALReferenceEQProviderCandidates(state)
	if len(reference) != 1 || reference[0].PluginID != "nova" {
		t.Fatalf("TDR-only Reference EQ enumeration = %#v", reference)
	}
}

func TestSPALReferenceEQProviderTargetMatchesAnyDoesNotLetAnUnresolvedRefShadowARealOne(t *testing.T) {
	// Observed live: the model passed target_ref="current_selection" (a
	// literal context-field name it failed to resolve) alongside a perfectly
	// good track_id. Matching against target_ref alone drops every real
	// candidate; matching against any supplied ref must still succeed.
	candidate := spalReferenceEQProviderCandidate{TrackID: "1007", TargetRef: "track:1007"}
	refs := []string{"current_selection", "1007"}
	if !spalReferenceEQProviderTargetMatchesAny(candidate, refs) {
		t.Fatalf("expected a match via track_id even though target_ref was an unresolved literal")
	}
	if spalReferenceEQProviderTargetMatchesAny(candidate, []string{"current_selection"}) {
		t.Fatal("an unresolved literal alone must not match any candidate")
	}
}

func TestSPALProviderRegistrationSessionRetainsNoFallbackConstraint(t *testing.T) {
	runtime := orchestrationruntime.New()
	session, err := runtime.StartSPALReferenceEQProviderRegistrationChatSession("provider-registration", "conversation", "project", "Register provider", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	if session.Invocation.CapabilityID != spalReferenceEQProviderRegistrationCapabilityID || !containsSPALReferenceEQConstraint(session.Constraints, "requires_explicit_observed_instance") || !containsSPALReferenceEQConstraint(session.Constraints, "no_plugin_loading_or_provider_fallback") {
		t.Fatalf("unexpected registration session: %#v", session)
	}
}
