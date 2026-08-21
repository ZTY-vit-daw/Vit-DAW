package chat

import (
	"context"
	"testing"
)

func TestSemanticGuidanceAuditBaselineModelOwnedEntryRouting(t *testing.T) {
	t.Setenv("VIT_AGENT_USE_LEGACY_PLANNER_LOOP", "")
	trackContext := map[string]any{"selected_track_id": "track-1"}
	pluginContext := map[string]any{
		"selected_track_id":        "track-1",
		"selected_plugin_track_id": "track-1",
		"selected_plugin_id":       "plugin-1",
	}

	if shouldStartFreeStateReasoningLoop("reduce the harsh high frequencies", trackContext) {
		t.Fatal("unverified text must not start free-state")
	}
	open := contextWithSemanticEntryDecision(trackContext, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteOpenSemantic,
		Controller:  "minimal_audio_closure",
		TargetScope: semanticEntryScopeCurrentSelection, ControlMode: semanticEntryControlSemanticLoop,
		UserAuthorization: semanticEntryAuthorizationAction, Confidence: 0.9, Reason: "open acoustic treatment",
	})
	if !shouldStartFreeStateReasoningLoop("make the vocal clearer", open) {
		t.Fatal("verified open-semantic decision did not start free-state")
	}
	for _, route := range []string{semanticEntryRouteDiscussion, semanticEntryRouteObservation, semanticEntryRouteExplicitControl, semanticEntryRouteOther} {
		ctx := contextWithSemanticEntryDecision(trackContext, semanticEntryDecision{
			SchemaVersion: semanticEntryDecisionSchema, Route: route, TargetScope: semanticEntryScopeCurrentSelection,
			ControlMode:       map[string]string{semanticEntryRouteDiscussion: semanticEntryControlNone, semanticEntryRouteObservation: semanticEntryControlObserveOnly, semanticEntryRouteExplicitControl: semanticEntryControlTyped, semanticEntryRouteOther: semanticEntryControlOrdinary}[route],
			UserAuthorization: map[string]string{semanticEntryRouteDiscussion: semanticEntryAuthorizationNone, semanticEntryRouteObservation: semanticEntryAuthorizationObserve, semanticEntryRouteExplicitControl: semanticEntryAuthorizationAction, semanticEntryRouteOther: semanticEntryAuthorizationNone}[route], Confidence: 0.9, Reason: "classified",
		})
		if shouldStartFreeStateReasoningLoop("any text containing compressor and EQ", ctx) {
			t.Fatalf("route %s incorrectly started free-state", route)
		}
	}
	_ = pluginContext
}

func TestSemanticGuidanceAuditLegacyPlannerCannotBypassOpenSemanticLoop(t *testing.T) {
	t.Setenv("VIT_AGENT_USE_LEGACY_PLANNER_LOOP", "1")
	ctx := contextWithSemanticEntryDecision(map[string]any{"selected_track_id": "track-1"}, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteOpenSemantic,
		Controller:  "minimal_audio_closure",
		TargetScope: semanticEntryScopeCurrentSelection, ControlMode: semanticEntryControlSemanticLoop,
		UserAuthorization: semanticEntryAuthorizationAction, Confidence: 0.9, Reason: "open acoustic treatment",
	})
	if !shouldStartFreeStateReasoningLoop("make the selected track steadier", ctx) {
		t.Fatal("legacy planner switch bypassed the verified open-semantic loop")
	}
}

func TestSemanticGuidanceAuditBaselineCurrentPreFamilyEQTopologyInjection(t *testing.T) {
	server := New(nil, nil, nil)
	fake := newFakeEQKernel()
	server.eqKernelOverride = fake
	requestContext := map[string]any{
		"selected_track_id":        "track-1",
		"selected_plugin_track_id": "track-1",
		"selected_plugin_id":       "eq-1",
	}

	got := server.agentLoopContextWithGenericEQTopology(context.Background(), "reduce the harsh high frequencies", requestContext)
	if len(firstMapFromAny(got["generic_eq_topology"])) != 0 {
		t.Fatal("EQ topology was injected before a model-owned family authorization")
	}
	if fake.readCalls != 0 {
		t.Fatalf("unauthorized EQ topology read count = %d", fake.readCalls)
	}

	unchanged := server.agentLoopContextWithGenericEQTopology(context.Background(), "make the track wider", requestContext)
	if len(firstMapFromAny(unchanged["generic_eq_topology"])) != 0 {
		t.Fatal("current non-EQ topic unexpectedly received generic_eq_topology")
	}
	if fake.readCalls != 0 {
		t.Fatalf("current non-EQ topic caused an additional parameter read: %d", fake.readCalls)
	}
}

func TestSemanticGuidanceAuditBaselineCurrentFamilyAndInstanceBinding(t *testing.T) {
	instances := semanticTreatmentTestInstances()
	qualifiedCompressor := instances[3]
	if !semanticTreatmentInstanceExecutable(qualifiedCompressor, "compressor", "semantic_compressor") {
		t.Fatal("qualified compressor instance is not executable through its current family planner")
	}
	if semanticTreatmentInstanceExecutable(qualifiedCompressor, "eq", "semantic_eq") {
		t.Fatal("qualified compressor instance crossed into the EQ planner")
	}

	plan, err := decodeAndValidateSemanticTreatmentPlan(
		`{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct","user_goal":"make it stable","choices":[{"choice_key":"comp","role":"recommended","title":"compression","processor_type":"compressor","target_mode":"existing_plugin","instance_key":"loaded_instance_4","reason":"control dynamics","expected_effect":"more stable","confidence":"high","next_planner":"semantic_compressor"}]}`,
		"make it stable", instances, false,
	)
	if err != nil {
		t.Fatalf("current exact qualified instance selection was rejected: %v", err)
	}
	if issue := semanticTreatmentRequiredProcessorIssue(plan, "compressor"); issue != nil {
		t.Fatalf("current upstream compressor family binding was rejected: %v", issue)
	}
	if issue := semanticTreatmentRequiredProcessorIssue(plan, "eq"); issue == nil {
		t.Fatal("current treatment strategy was allowed to replace the upstream family")
	}
}
