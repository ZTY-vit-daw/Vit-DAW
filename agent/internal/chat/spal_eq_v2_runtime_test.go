package chat

import (
	"testing"

	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationruntime"
	"vit-daw-agent/internal/spal"
)

func TestSPALEQV2ParsesBandShapeAsBandState(t *testing.T) {
	request := parseSPALEQV2Request(ChatRequest{Message: "用 TDR Nova 把 Band II 设为 High Shelf，632 Hz，+5.4 dB，Q 0.95", Context: map[string]any{"selected_track_id": "1007", "selected_plugin_id": "1013"}})
	if len(request.Missing) != 0 {
		t.Fatalf("unexpected missing fields: %#v", request.Missing)
	}
	if request.Instruction.SchemaID != spal.EQBandPatchControlID || request.Instruction.StringParameters["band_ref"] != "b2" || request.Instruction.StringParameters["response_shape"] != "high_shelf" || request.Instruction.Parameters["frequency_hz"] != 632 || request.Instruction.Parameters["gain_db"] != 5.4 || request.Instruction.Parameters["q"] != .95 {
		t.Fatalf("unexpected Band instruction: %#v", request.Instruction)
	}
	if inferProjectAwareCapability("用 TDR Nova 把 Band II 设为 High Shelf，632 Hz，+5.4 dB，Q 0.95") != "" {
		t.Fatal("ordinary EQ request bypassed AgentLoop")
	}
}

func TestEqualizerCapabilityPlanBuildsFiniteSPALInstruction(t *testing.T) {
	plan := buildEqualizerCapabilityPlan(map[string]any{
		"task": "spectral_region_adjust", "band_ref": "b3", "response_shape": "bell",
		"frequency_hz": 3400.0, "gain_db": 3.0, "q": 1.0,
	}, map[string]any{"selected_track_id": "1007", "selected_plugin_id": "1013"})
	if len(plan.MissingSemantic) != 0 || len(plan.MissingTechnical) != 0 {
		t.Fatalf("unexpected capability gaps: semantic=%v technical=%v", plan.MissingSemantic, plan.MissingTechnical)
	}
	instruction := plan.Request.Instruction
	if instruction.SchemaID != spal.EQBandPatchControlID || instruction.TargetRef != "1007" || plan.Request.PluginID != "1013" || instruction.StringParameters["band_ref"] != "b3" || instruction.StringParameters["response_shape"] != "bell" || instruction.Parameters["frequency_hz"] != 3400 || instruction.Parameters["gain_db"] != 3 || instruction.Parameters["q"] != 1 {
		t.Fatalf("unexpected structured instruction: %#v request=%#v", instruction, plan.Request)
	}
}

func TestEqualizerCapabilityPlanReturnsAgentAndResourceGaps(t *testing.T) {
	plan := buildEqualizerCapabilityPlan(map[string]any{
		"task": "spectral_region_adjust", "frequency_hz": 3400.0, "gain_db": 3.0,
	}, map[string]any{"selected_track_id": "1007"})
	for _, field := range []string{"response_shape", "q"} {
		if !containsIgnoreCase(plan.MissingSemantic, field) {
			t.Fatalf("missing semantic gap %s: %#v", field, plan)
		}
	}
	if !containsIgnoreCase(plan.MissingTechnical, "band_ref") {
		t.Fatalf("Band resource gap missing: %#v", plan)
	}
}

func TestEqualizerCapabilityMapsHumanCutTermsWithoutNaturalLanguageParser(t *testing.T) {
	lowCut := buildEqualizerCapabilityPlan(map[string]any{
		"task": "low_cut", "cutoff_frequency_hz": 80.0, "slope_db_per_octave": 24.0,
	}, map[string]any{"selected_track_id": "1007"})
	if len(lowCut.MissingSemantic) != 0 || len(lowCut.MissingTechnical) != 0 || lowCut.Request.Instruction.StringParameters["filter_kind"] != "highpass" {
		t.Fatalf("low cut task was not normalized to highpass: %#v", lowCut)
	}
	highCut := buildEqualizerCapabilityPlan(map[string]any{
		"task": "high_cut", "cutoff_frequency_hz": 12000.0, "slope_db_per_octave": 12.0,
	}, map[string]any{"selected_track_id": "1007"})
	if len(highCut.MissingSemantic) != 0 || len(highCut.MissingTechnical) != 0 || highCut.Request.Instruction.StringParameters["filter_kind"] != "lowpass" {
		t.Fatalf("high cut task was not normalized to lowpass: %#v", highCut)
	}
}

func TestSPALEQV2ParsesHighpassAndDynamic(t *testing.T) {
	hp := parseSPALEQV2Request(ChatRequest{Message: "打开 Nova 的高通并设为 80 Hz / 24 dB/oct", Context: map[string]any{"selected_track_id": "1007"}})
	if len(hp.Missing) != 0 || hp.Instruction.SchemaID != spal.EQPassFilterPatchControlID || hp.Instruction.StringParameters["filter_kind"] != "highpass" || hp.Instruction.Parameters["cutoff_frequency_hz"] != 80 || hp.Instruction.Parameters["slope_db_per_octave"] != 24 {
		t.Fatalf("unexpected HP request: %#v", hp)
	}
	dynamic := parseSPALEQV2Request(ChatRequest{Message: "开启 TDR Nova Band 2 动态 EQ，Threshold -17.5 dB，Ratio 3:1，Attack 8 ms，Release 200 ms", Context: map[string]any{"selected_track_id": "1007"}})
	if len(dynamic.Missing) != 0 || dynamic.Instruction.SchemaID != spal.EQDynamicBandPatchControlID || dynamic.Instruction.StringParameters["routing_scope"] != "independent" || dynamic.Instruction.Parameters["threshold_db"] != -17.5 || dynamic.Instruction.Parameters["ratio"] != 3 || dynamic.Instruction.Parameters["attack_ms"] != 8 || dynamic.Instruction.Parameters["release_ms"] != 200 {
		t.Fatalf("unexpected dynamic request: %#v", dynamic)
	}
}

func TestSPALEQV2DoesNotCaptureVagueEQIntent(t *testing.T) {
	if isSPALEQV2ControlIntent("帮我把低频浑浊减一点") {
		t.Fatal("vague mix intent was captured")
	}
	if got := inferProjectAwareCapability("帮我把低频浑浊减一点"); got != "" {
		t.Fatalf("vague intent routed to %q", got)
	}
}

func TestSPALEQV2RollbackIsExplicitAndSeparatelyPresented(t *testing.T) {
	if spalEQV2RollbackRequested(ChatRequest{Message: "please undo the EQ"}) {
		t.Fatal("generic undo wording must not create a plug-in rollback")
	}
	if !spalEQV2RollbackRequested(ChatRequest{Message: "please rollback", Context: map[string]any{"spal_operation": "rollback"}}) {
		t.Fatal("typed rollback operation was not recognized")
	}
	if !spalEQV2RollbackRequested(ChatRequest{Message: "rollback the transaction", Context: map[string]any{"capability_id": spalEQV2CapabilityID}}) {
		t.Fatal("explicit EQ v2 rollback request was not recognized")
	}
	presentation := spalEQV2ProposalPresentation(orchestration.Proposal{ID: "proposal", Revision: 1, CapabilityID: spalEQV2CapabilityID}, spal.Instruction{}, spal.OperationRollback)
	if presentation.Title != "SPAL EQ v2 回滚方案" || !presentation.Reversible || presentation.ApprovalPrompt == "" {
		t.Fatalf("rollback presentation = %#v", presentation)
	}
}

func TestSPALEQV2SessionRequiresSeparatelyConfirmableRollback(t *testing.T) {
	runtime := orchestrationruntime.New()
	session, err := runtime.StartSPALEQV2ChatSession("eq-v2", "conversation", "project", "EQ v2", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	if !containsSPALEQV2Constraint(session.Constraints, "separately_confirmable_rollback_to_frozen_preimage") {
		t.Fatalf("EQ v2 rollback constraint missing: %#v", session.Constraints)
	}
}

func containsSPALEQV2Constraint(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestEqGrabberProfileHasEQControlsDetectsClassAndVirtualControls(t *testing.T) {
	if !eqGrabberProfileHasEQControls(map[string]any{"class": "eq"}) {
		t.Fatal("class eq should count as an EQ profile")
	}
	if !eqGrabberProfileHasEQControls(map[string]any{"class": "utility", "virtual_controls": []any{
		map[string]any{"name": "eq.cut_region"},
	}}) {
		t.Fatal("eq.* virtual control should count as an EQ profile")
	}
	if eqGrabberProfileHasEQControls(map[string]any{"class": "reverb", "virtual_controls": []any{
		map[string]any{"name": "reverb.mix"},
	}}) {
		t.Fatal("non-EQ profile without eq.* controls should not count as an EQ profile")
	}
}

func TestMatchEqGrabberFallbackProfilePicksCurrentlyLoadedPluginNotRecycledSlot(t *testing.T) {
	// Rack slot "1015" previously held TDR Nova and was later reloaded with
	// Pro-Q 3. Both saved profiles still carry plugin_id "1015" because that
	// was the slot each was learned in. Matching on plugin_id alone (as
	// opposed to path/name, which is what this function actually does) would
	// pick whichever profile happens to come first for that recycled slot.
	profiles := []map[string]any{
		{
			"class": "eq",
			"plugin_identity": map[string]any{
				"plugin_id":   "1015",
				"plugin_name": "TDR Nova",
				"plugin_path": `C:\Program Files\Common Files\VST3\TDR Nova.vst3`,
				"profile_key": "plugin_f9788adda4203df8",
			},
		},
		{
			"class": "eq",
			"plugin_identity": map[string]any{
				"plugin_id":   "1015",
				"plugin_name": "Pro-Q 3",
				"plugin_path": `C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3`,
				"profile_key": "plugin_c29109a41b7a1d93",
			},
		},
	}
	got := matchEqGrabberFallbackProfile(profiles, "1015", "Pro-Q 3", `C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3`)
	if got == nil {
		t.Fatal("expected a matching fallback profile for the currently loaded plug-in")
	}
	if got["profile_key"] != "plugin_c29109a41b7a1d93" {
		t.Fatalf("matched wrong profile: got %#v, want Pro-Q 3's profile_key", got)
	}

	if got := matchEqGrabberFallbackProfile(profiles, "1015", "Some Other Plugin", `C:\nonexistent.vst3`); got != nil {
		t.Fatalf("expected no match when name/path do not match any profile for the recycled slot, got %#v", got)
	}
}

func TestMatchEqGrabberFallbackProfileMatchesByPathEvenWhenSavedPluginIDHasDrifted(t *testing.T) {
	// Observed live: Pro-Q 3 was learned while loaded into rack slot "1015",
	// then the project reloaded and it now occupies slot "1013". The profile
	// on disk still says plugin_id "1015". The caller passes the *current*
	// slot id ("1013") for the returned plugin_id field, but matching must
	// still succeed via path even though every plugin_id disagrees.
	profiles := []map[string]any{
		{
			"class": "eq",
			"plugin_identity": map[string]any{
				"plugin_id":   "1015",
				"plugin_name": "Pro-Q 3",
				"plugin_path": `C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3`,
				"profile_key": "plugin_c29109a41b7a1d93",
			},
		},
	}
	got := matchEqGrabberFallbackProfile(profiles, "1013", "Pro-Q 3", `C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3`)
	if got == nil {
		t.Fatal("expected path-based match despite drifted plugin_id")
	}
	if got["profile_key"] != "plugin_c29109a41b7a1d93" || got["plugin_id"] != "1013" {
		t.Fatalf("unexpected match result: %#v", got)
	}
}

func TestEqGrabberProfileIsStaleDetectsFlagsAndStaleParamIDs(t *testing.T) {
	if !eqGrabberProfileIsStale(map[string]any{"stale": true}) {
		t.Fatal("stale flag should mark profile stale")
	}
	if !eqGrabberProfileIsStale(map[string]any{"profile_stale_param_ids": []any{"2"}}) {
		t.Fatal("non-empty profile_stale_param_ids should mark profile stale")
	}
	if !eqGrabberProfileIsStale(map[string]any{"profile_status": "stale"}) {
		t.Fatal("profile_status containing stale should mark profile stale")
	}
	if eqGrabberProfileIsStale(map[string]any{"profile_status": "confirmed"}) {
		t.Fatal("confirmed profile should not be reported stale")
	}
}
