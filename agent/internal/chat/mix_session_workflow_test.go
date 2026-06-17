package chat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/mixcontrolsurface"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestMixSessionEntryFromNaturalLanguageIsNotHandledByLegacyWorkflow(t *testing.T) {
	server := New(nil, nil, nil)
	req := ChatRequest{
		ConversationID: "conv_mix",
		Message:        "帮我自动混当前轨道",
		Context: map[string]any{
			"selected_track_id":   "track_1",
			"selected_track_name": "Vocal",
		},
	}

	if resp, handled := server.runMixSessionEntryChat(context.Background(), "conv_mix", req, agentModeDefault); handled {
		t.Fatalf("legacy mix workflow should not handle natural Ask Vit mix requests: %#v", resp)
	}
}

func TestMixSessionEntryLegacyWorkflowRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("VIT_ENABLE_LEGACY_MIX_SESSION_WORKFLOW", "1")
	server := New(nil, nil, nil)
	req := ChatRequest{
		ConversationID: "conv_mix_stem",
		Message:        "你能帮我自动缩混当前轨道吗？",
		Context: map[string]any{
			"selected_track_id":   "track_1",
			"selected_track_name": "Vocal",
		},
	}

	resp, handled := server.runMixSessionEntryChat(context.Background(), "conv_mix_stem", req, agentModeDefault)
	if !handled {
		t.Fatal("auto stem/mix request was not handled")
	}
	if got := cleanContextText(resp.MixSession["mode"]); got != mixModeAuto {
		t.Fatalf("mode = %q", got)
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingConfirmation) || len(resp.InteractionRequests) != 1 {
		t.Fatalf("expected confirmation card, status=%q interactions=%#v", resp.GoalStatus, resp.InteractionRequests)
	}
	target := mapValue(resp.MixSession["target_ref"])
	if target["kind"] != "track" || target["id"] != "track_1" {
		t.Fatalf("target_ref = %#v", target)
	}
}

func TestMixSessionEntryPlanModeDoesNotCreateSession(t *testing.T) {
	server := New(nil, nil, nil)
	req := ChatRequest{
		ConversationID: "conv_mix_plan",
		Message:        "帮我自动混当前轨道",
		Context: map[string]any{
			"agent_mode":        "plan",
			"selected_track_id": "track_1",
		},
	}

	if resp, handled := server.runMixSessionEntryChat(context.Background(), "conv_mix_plan", req, agentModePlan); handled {
		t.Fatalf("legacy mix workflow should not handle plan-mode natural mix requests by default: %#v", resp)
	}
}

func TestMixSessionConfirmIsDeprecatedWithoutLegacyOptIn(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	server := New(nil, nil, nil)
	session := pendingMixSession(mixModeCo, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Source: "current_selection", Confidence: "high"}, "一起混这条轨")
	data := mixSessionWorkflowData("conv_mix", "goal_1", "run_1", map[string]any{}, session)
	interaction := PendingInteraction{
		ID:             "interaction_1",
		Source:         "mix_session",
		Workflow:       mixSessionEntryWorkflow,
		Type:           "mix_session_entry",
		ConversationID: "conv_mix",
		GoalID:         "goal_1",
		RunID:          "run_1",
		Payload:        data,
		Data:           data,
	}

	resp := server.continueMixSessionInteraction(context.Background(), interaction, nil, "confirm_mix_session")
	if resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("goal_status = %q", resp.GoalStatus)
	}
	if !strings.Contains(resp.Reply, "停用") {
		t.Fatalf("reply = %q", resp.Reply)
	}
	if len(resp.InteractionRequests) != 0 {
		t.Fatalf("deprecated confirm should not create interactions: %#v", resp.InteractionRequests)
	}
	if len(mapValue(resp.WorkflowData["mix_observation"])) != 0 {
		t.Fatalf("deprecated confirm should not create MixBoard observation: %#v", resp.WorkflowData["mix_observation"])
	}
}

func TestPublishMixBoardPlannerActionDeprecated(t *testing.T) {
	server := New(nil, nil, nil)
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "balance vocal")
	session.State = mixStatePreparing
	session.InteractionPhase = mixInteractionPlanningChat
	session.MixBoardVisibility = mixBoardVisibilityCollapsed
	data := mixSessionWorkflowData("conv_mix", "goal_1", "run_1", map[string]any{}, session)
	data["mix_planner_prep"] = initialMixPlannerPrep(session, map[string]any{})
	interaction := PendingInteraction{
		ConversationID: "conv_mix",
		GoalID:         "goal_1",
		RunID:          "run_1",
		Payload:        data,
		Data:           data,
	}

	resp := server.continueMixSessionInteraction(context.Background(), interaction, nil, "publish_mixboard")
	if resp.CurrentStep != "deprecated_mix_planner_action" {
		t.Fatalf("current step = %q", resp.CurrentStep)
	}
	if cleanContextText(mapValue(resp.MixSession)["mixboard_visibility"]) != mixBoardVisibilityHidden {
		t.Fatalf("mix session = %#v", resp.MixSession)
	}
	if len(mapValue(resp.WorkflowData["mix_observation"])) != 0 {
		t.Fatalf("deprecated publish should not request observation or create MixBoard: %#v", resp.WorkflowData["mix_observation"])
	}
	if len(resp.InteractionRequests) != 0 {
		t.Fatalf("deprecated planner action should not create interactions: %#v", resp.InteractionRequests)
	}
}

func TestMixPlannerIntakeActionsGateMixBoardPublish(t *testing.T) {
	session := pendingMixSession(mixModeCo, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "clean vocal")
	prep := initialMixPlannerPrep(session, map[string]any{})
	if boolValue(prep["ready_to_publish_mixboard"]) {
		t.Fatalf("initial prep should not be ready: %#v", prep)
	}
	for _, action := range mixPlannerInteractionActions(prep) {
		if action.ID == "publish_mixboard" {
			t.Fatalf("publish action should be hidden before intake is complete: %#v", mixPlannerInteractionActions(prep))
		}
	}
	absorbMixPlannerAnswers(prep, &session, map[string]any{
		"fields": map[string]any{
			"goal_detail":        "clean low mids and keep the vocal forward",
			"allow_plugin_loads": true,
		},
	})
	updateMixPlannerPrepDerived(prep, session)
	if missing := mixPlannerMissingInputs(prep); len(missing) == 0 || missing[0] != "workspace_plugin_types" {
		t.Fatalf("missing inputs = %#v prep=%#v", missing, prep)
	}
	if cleanContextText(prep["stage"]) != "plan_drafting" {
		t.Fatalf("stage = %#v", prep)
	}
	actions := mixPlannerInteractionActions(prep)
	for _, action := range actions {
		if action.ID == "publish_mixboard" || action.ID == "confirm_mix_planner_plan" {
			t.Fatalf("publish/confirm actions should be hidden until workspace is complete: %#v", actions)
		}
	}
	prep["plugin_candidates"] = []map[string]any{{"name": "TDR Nova"}}
	prep["skill_profile_status"] = map[string]any{"profile_status": mixcontrolsurface.ProfileReady, "next_action": "confirm_control_surface"}
	prep["macro_panel_draft"] = map[string]any{"controls": []map[string]any{{"name": "Track volume", "control": "track.volume"}}}
	prep["proposed_controls"] = []map[string]any{{"name": "track volume"}}
	updateMixPlannerPrepDerived(prep, session)
	if missing := mixPlannerMissingInputs(prep); len(missing) == 0 || missing[0] != "workspace_plugin_types" {
		t.Fatalf("draft values should still require discussion acceptance, missing=%#v prep=%#v", missing, prep)
	}
	for _, action := range mixPlannerInteractionActions(prep) {
		if action.ID == "publish_mixboard" || action.ID == "confirm_mix_planner_plan" {
			t.Fatalf("publish/confirm actions should be hidden for unaccepted drafts: %#v", mixPlannerInteractionActions(prep))
		}
	}
	mixPlannerAcceptAllDraftSlots(prep)
	updateMixPlannerPrepDerived(prep, session)
	if missing := mixPlannerMissingInputs(prep); len(missing) != 1 || missing[0] != "publish_mixboard_confirmed" {
		t.Fatalf("missing after workspace complete = %#v prep=%#v", missing, prep)
	}
	if !boolValue(prep["publish_mixboard_prompted"]) || boolValue(prep["ready_to_publish_mixboard"]) {
		t.Fatalf("publish gate should prompt but not publish: %#v", prep)
	}
	actions = mixPlannerInteractionActions(prep)
	foundConfirm := false
	for _, action := range actions {
		if action.ID == "confirm_mix_planner_plan" && action.Recommended {
			foundConfirm = true
		}
		if action.ID == "publish_mixboard" {
			t.Fatalf("publish action should be hidden before user confirms generation: %#v", actions)
		}
	}
	if !foundConfirm {
		t.Fatalf("actions = %#v", actions)
	}
	absorbMixPlannerAnswers(prep, &session, map[string]any{"message": "还不行"})
	updateMixPlannerPrepDerived(prep, session)
	if missing := mixPlannerMissingInputs(prep); len(missing) != 1 || missing[0] != "planner_revision_note" {
		t.Fatalf("rejecting publish should enter revision, missing=%#v prep=%#v", missing, prep)
	}
	actions = mixPlannerInteractionActions(prep)
	for _, action := range actions {
		if action.ID == "publish_mixboard" || action.ID == "confirm_mix_planner_plan" {
			t.Fatalf("publish/confirm actions should be hidden during revision: %#v", actions)
		}
	}
	absorbMixPlannerAnswers(prep, &session, map[string]any{"message": "由你决定"})
	updateMixPlannerPrepDerived(prep, session)
	if missing := mixPlannerMissingInputs(prep); len(missing) != 1 || missing[0] != "publish_mixboard_confirmed" {
		t.Fatalf("revision should return to publish confirmation after new input, missing=%#v prep=%#v", missing, prep)
	}
	absorbMixPlannerAnswers(prep, &session, map[string]any{"message": "可以"})
	updateMixPlannerPrepDerived(prep, session)
	if missing := mixPlannerMissingInputs(prep); len(missing) != 0 {
		t.Fatalf("missing after publish confirm = %#v prep=%#v", missing, prep)
	}
	actions = mixPlannerInteractionActions(prep)
	foundPublish := false
	for _, action := range actions {
		if action.ID == "publish_mixboard" && action.Recommended {
			foundPublish = true
		}
	}
	if !foundPublish {
		t.Fatalf("actions = %#v", actions)
	}
}

func TestMixPlannerChatAbsorbsMainChatDirectionAndDelegation(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "auto mix")
	prep := initialMixPlannerPrep(session, map[string]any{})

	absorbMixPlannerAnswers(prep, &session, map[string]any{"message": "提升响度"})
	updateMixPlannerPrepDerived(prep, session)
	if got := cleanContextText(prep["goal_detail"]); got != "提升响度" {
		t.Fatalf("goal_detail = %q prep=%#v", got, prep)
	}

	absorbMixPlannerAnswers(prep, &session, map[string]any{"message": "由你决定"})
	updateMixPlannerPrepDerived(prep, session)
	if cleanContextText(prep["planner_autonomy"]) != "agent_decides" || !boolValue(prep["allow_plugin_loads"]) {
		t.Fatalf("delegation was not captured: %#v", prep)
	}
	if boolValue(prep["planner_plan_confirmed"]) {
		t.Fatalf("delegation should not confirm publishing the plan: %#v", prep)
	}
	if cleanContextText(prep["stage"]) != "plan_drafting" {
		t.Fatalf("stage = %#v", prep)
	}
}

func TestMixPlannerQuestionDoesNotOverwriteGoal(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "auto mix")
	prep := initialMixPlannerPrep(session, map[string]any{})
	absorbMixPlannerAnswers(prep, &session, map[string]any{"message": "提升响度"})
	updateMixPlannerPrepDerived(prep, session)
	absorbMixPlannerAnswers(prep, &session, map[string]any{"message": "有什么插件推荐？"})
	updateMixPlannerPrepDerived(prep, session)
	if got := cleanContextText(prep["goal_detail"]); got != "提升响度" {
		t.Fatalf("plugin recommendation question should not overwrite goal, got %q prep=%#v", got, prep)
	}
	if got := cleanContextText(prep["last_user_intent"]); got != "ask_plugin_recommendation" {
		t.Fatalf("intent = %q prep=%#v", got, prep)
	}
	absorbMixPlannerAnswers(prep, &session, map[string]any{"message": "允许你加载"})
	updateMixPlannerPrepDerived(prep, session)
	if got := cleanContextText(prep["goal_detail"]); got != "提升响度" {
		t.Fatalf("plugin permission should not overwrite goal, got %q prep=%#v", got, prep)
	}
	if !boolValue(prep["allow_plugin_loads"]) {
		t.Fatalf("plugin permission was not captured: %#v", prep)
	}
	if got := cleanContextText(prep["last_user_intent"]); got != "allow_plugin_loads" {
		t.Fatalf("intent = %q prep=%#v", got, prep)
	}
}

func TestMixPlannerLLMPatchDoesNotOverwriteGoal(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "auto mix")
	prep := initialMixPlannerPrep(session, map[string]any{})
	absorbMixPlannerAnswers(prep, &session, map[string]any{"message": "提升响度"})
	updateMixPlannerPrepDerived(prep, session)

	patch := sanitizeMixPlannerLLMPatch(map[string]any{
		"intent":             "allow_plugin_loads",
		"allow_plugin_loads": true,
		"confidence":         0.91,
	})
	payload := map[string]any{"message": "允许你加载", "planner_llm_patch": patch}
	for key, value := range patch {
		payload[key] = value
	}
	absorbMixPlannerAnswers(prep, &session, payload)
	updateMixPlannerPrepDerived(prep, session)

	if got := cleanContextText(prep["goal_detail"]); got != "提升响度" {
		t.Fatalf("LLM permission patch should not overwrite goal, got %q prep=%#v", got, prep)
	}
	if !boolValue(prep["allow_plugin_loads"]) || cleanContextText(prep["last_user_intent"]) != "allow_plugin_loads" {
		t.Fatalf("LLM permission patch was not applied: %#v", prep)
	}
	if confidence := floatNumber(prep["planner_llm_confidence"]); confidence <= 0 {
		t.Fatalf("LLM confidence missing: %#v", prep)
	}
}

func TestMixPlanningWorkspaceTracksWorkflowSlots(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "auto mix")
	prep := initialMixPlannerPrep(session, map[string]any{})
	absorbMixPlannerAnswers(prep, &session, map[string]any{"message": "提升响度，但不要刺耳"})
	updateMixPlannerPrepDerived(prep, session)
	workspace := syncMixPlanningWorkspace(prep, session)

	if cleanContextText(workspace["schema_version"]) != mixPlanningWorkspaceSchemaVersion {
		t.Fatalf("workspace = %#v", workspace)
	}
	goal := mapValue(workspace["goal"])
	if got := cleanContextText(goal["detail"]); got != "提升响度，但不要刺耳" {
		t.Fatalf("goal detail = %q workspace=%#v", got, workspace)
	}
	slotIDs := map[string]bool{}
	for _, slot := range mapRowsValue(workspace["workflow_slots"]) {
		slotIDs[cleanContextText(slot["id"])] = true
	}
	for _, want := range []string{"mix_goal", "plugin_types", "local_plugin_candidates", "plugin_chain_order", "skill_profile_status", "macro_panel", "fast_tick_packet"} {
		if !slotIDs[want] {
			t.Fatalf("missing workspace slot %q slots=%#v", want, workspace["workflow_slots"])
		}
	}
	verdict := mapValue(workspace["planner_verdict"])
	if boolValue(verdict["publishable"]) {
		t.Fatalf("workspace should not be publishable before preflight: %#v", verdict)
	}
}

func TestActiveMixPlannerChatReturnsReadonlyWorkspaceSummary(t *testing.T) {
	server := New(nil, nil, nil)
	server.llm = nil
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "auto mix")
	session.ConversationID = "conv_mix"
	session.State = mixStatePreparing
	session.InteractionPhase = mixInteractionPlanningChat
	session.MixBoardVisibility = mixBoardVisibilityCollapsed
	session.PlannerPrep = initialMixPlannerPrep(session, map[string]any{})
	server.storeMixSession(session)

	resp, handled := server.continueActiveMixPlannerChat(context.Background(), "conv_mix", ChatRequest{
		ConversationID: "conv_mix",
		Message:        "提升响度",
		Context:        map[string]any{"legacy_mix_session_workflow": true},
	})
	if !handled {
		t.Fatal("active planner chat was not handled")
	}
	if len(resp.InteractionRequests) != 1 {
		t.Fatalf("interaction requests = %#v", resp.InteractionRequests)
	}
	if len(resp.InteractionRequests[0].Fields) != 0 {
		t.Fatalf("planning summary should not expose form fields: %#v", resp.InteractionRequests[0].Fields)
	}
	if len(resp.InteractionRequests[0].ReviewItems) == 0 {
		t.Fatalf("planning summary should expose readonly review items: %#v", resp.InteractionRequests[0])
	}
	workspace := mapValue(resp.WorkflowData["mix_planning_workspace"])
	goal := mapValue(workspace["goal"])
	if got := cleanContextText(goal["detail"]); got != "提升响度" {
		t.Fatalf("workspace goal detail = %q workspace=%#v", got, workspace)
	}
	if len(mapValue(resp.WorkflowData["mix_observation"])) != 0 {
		t.Fatalf("planning response should not expose mix observation before publish readiness: %#v", resp.WorkflowData["mix_observation"])
	}
	prep := mapValue(resp.WorkflowData["mix_planner_prep"])
	if missing := contextStringSlice(prep["missing_inputs"]); len(missing) == 0 || missing[0] != "allow_plugin_loads" {
		t.Fatalf("first planner turn should continue discussion, missing=%#v prep=%#v", missing, prep)
	}

	session = mixSessionFromMap(mapValue(resp.MixSession))
	session.PlannerPrep = prep
	server.storeMixSession(session)
	resp, handled = server.continueActiveMixPlannerChat(context.Background(), "conv_mix", ChatRequest{
		ConversationID: "conv_mix",
		Message:        "允许你加载",
		Context:        map[string]any{"legacy_mix_session_workflow": true},
	})
	if !handled {
		t.Fatal("active planner chat was not handled on plugin permission")
	}
	if len(mapValue(resp.WorkflowData["mix_observation"])) != 0 {
		t.Fatalf("planning response should keep observation hidden after plugin permission: %#v", resp.WorkflowData["mix_observation"])
	}
	prep = mapValue(resp.WorkflowData["mix_planner_prep"])
	if missing := contextStringSlice(prep["missing_inputs"]); len(missing) == 0 || missing[0] != "workspace_plugin_types" {
		t.Fatalf("planner should discuss drafted slots before publish, missing=%#v prep=%#v", missing, prep)
	}
	for _, action := range resp.InteractionRequests[0].Actions {
		if action.ID == "publish_mixboard" || action.ID == "confirm_mix_planner_plan" {
			t.Fatalf("publish actions should be hidden during drafted planning: %#v", resp.InteractionRequests[0].Actions)
		}
	}
}

func TestActiveMixPlannerChatDoesNotInterceptWithoutLegacyOptIn(t *testing.T) {
	server := New(nil, nil, nil)
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "auto mix")
	session.ConversationID = "conv_mix"
	session.State = mixStatePreparing
	session.InteractionPhase = mixInteractionPlanningChat
	session.MixBoardVisibility = mixBoardVisibilityCollapsed
	session.PlannerPrep = initialMixPlannerPrep(session, map[string]any{})
	server.storeMixSession(session)

	resp, handled := server.continueActiveMixPlannerChat(context.Background(), "conv_mix", ChatRequest{
		ConversationID: "conv_mix",
		Message:        "raise loudness",
		Context:        map[string]any{},
	})
	if handled {
		t.Fatalf("active planner chat should not intercept natural chat without legacy opt-in: %#v", resp)
	}
}

func TestConfirmedControlSurfacePluginLoadKeyDedupesSamePlugin(t *testing.T) {
	left := confirmedControlSurfacePluginLoadKey("track_1", map[string]any{
		"profile_id":  "plugin_tdr_nova",
		"name":        "TDR Nova",
		"plugin_path": "C:/VST/TDR Nova.vst3",
	})
	right := confirmedControlSurfacePluginLoadKey("track_1", map[string]any{
		"id":          "plugin_tdr_nova",
		"name":        "TDR Nova",
		"plugin_path": "C:/VST/TDR Nova.vst3",
	})
	if left == "" || left != right {
		t.Fatalf("load keys should match, left=%q right=%q", left, right)
	}
	if otherTrack := confirmedControlSurfacePluginLoadKey("track_2", map[string]any{"profile_id": "plugin_tdr_nova"}); otherTrack == left {
		t.Fatalf("different tracks must not dedupe together: %q", otherTrack)
	}
}

func TestMixBoardStatusInteractionDoesNotOfferDeprecatedTuningActions(t *testing.T) {
	server := New(nil, nil, nil)
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "降低当前轨道")
	session.State = mixStateObservationReady
	interaction := PendingInteraction{ConversationID: "conv_mix", GoalID: "goal_1", RunID: "run_1", RequestContext: map[string]any{}}
	req := server.mixBoardStatusInteraction(interaction, session, map[string]any{"status": "ready"})
	ids := map[string]bool{}
	for _, action := range req.Actions {
		ids[action.ID] = true
	}
	if !ids["revise_mixboard"] || !ids["refresh_observation"] {
		t.Fatalf("actions = %+v", req.Actions)
	}
	for _, deprecated := range []string{"confirm_control_surface", "execute_single_mix_tick", "start_mix_tuning", "enter_discussion", "submit_mixboard_intervention", "stop_mix_tuning", "done"} {
		if ids[deprecated] {
			t.Fatalf("deprecated action %s should be hidden: %+v", deprecated, req.Actions)
		}
	}
	session.State = mixStateTuningRunning
	session.JournalRefs = []string{"action_1"}
	req = server.mixBoardStatusInteraction(interaction, session, map[string]any{"status": "ready"})
	ids = map[string]bool{}
	for _, action := range req.Actions {
		ids[action.ID] = true
	}
	if !ids["rollback_last_mix_tick"] || ids["rollback_last_mix_turn"] || ids["stop_mix_tuning"] || ids["done"] {
		t.Fatalf("running actions = %+v", req.Actions)
	}
}

func TestConfirmControlSurfaceActionIsDeprecatedEvenWithConversationalPayload(t *testing.T) {
	server := New(nil, nil, nil)
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "mix vocal")
	session.State = mixStateObservationReady
	data := mixSessionWorkflowData("conv_mix", "goal_1", "run_1", map[string]any{}, session)
	data["mix_observation"] = map[string]any{
		"status": "ready",
		"goal_control_surface": map[string]any{
			"selected_chain": []map[string]any{{
				"role":            "tone_balance",
				"instance_status": mixcontrolsurface.InstanceNeedsLoad,
				"profile_status":  mixcontrolsurface.ProfileReady,
				"selected_plugin": map[string]any{"name": "TDR Nova", "plugin_path": "C:/VST/TDR Nova.vst3"},
			}},
		},
	}
	interaction := PendingInteraction{
		Source:         "mix_session",
		Workflow:       mixSessionEntryWorkflow,
		ConversationID: "conv_mix",
		GoalID:         "goal_1",
		RunID:          "run_1",
		Payload:        data,
		Data:           data,
	}

	resp := server.continueMixSessionInteraction(context.Background(), interaction, map[string]any{"ask_vit_mix_action": true}, "confirm_control_surface")
	if resp.CurrentStep != "deprecated_mix_planner_action" {
		t.Fatalf("current step = %q response=%#v", resp.CurrentStep, resp)
	}
	if cleanContextText(mapValue(resp.MixSession)["blocking_point"]) != "deprecated_mix_planner_action" {
		t.Fatalf("mix session = %#v", resp.MixSession)
	}
}

func TestMixBoardStatusInteractionBlocksTuningWhenControlSurfaceBlocked(t *testing.T) {
	server := New(nil, nil, nil)
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "balance vocal")
	session.State = mixStateObservationReady
	interaction := PendingInteraction{ConversationID: "conv_mix", GoalID: "goal_1", RunID: "run_1", RequestContext: map[string]any{}}
	observation := map[string]any{
		"status": "ready",
		"goal_control_surface": map[string]any{
			"schema_version":       mixcontrolsurface.SchemaVersion,
			"readiness":            mixcontrolsurface.ReadinessBlocked,
			"next_required_action": "learn_plugin_profile",
			"blockers":             []string{"dynamics requires Plugin Grabber learning"},
		},
	}
	req := server.mixBoardStatusInteraction(interaction, session, observation)
	for _, action := range req.Actions {
		if action.ID == "start_mix_tuning" || action.ID == "auto_tune_mix" || action.ID == "execute_single_mix_tick" {
			t.Fatalf("blocked control surface should not offer tuning action: %+v", req.Actions)
		}
	}
	if blocker := mixGoalControlSurfaceTuningBlocker(observation); !strings.Contains(blocker, "Plugin Grabber") {
		t.Fatalf("blocker = %q", blocker)
	}
}

func TestMixLatestTuneBatchUsesNewestBatch(t *testing.T) {
	turns := []map[string]any{
		{"turn": 1, "tuning_batch_id": "batch_1", "agent_action_id": "a1"},
		{"turn": 2, "tuning_batch_id": "batch_1", "agent_action_id": "a2"},
		{"turn": 3, "tuning_batch_id": "batch_1", "agent_action_id": "a3"},
		{"turn": 1, "tuning_batch_id": "batch_2", "agent_action_id": "b1", "before_db": -3.0},
		{"turn": 2, "tuning_batch_id": "batch_2", "agent_action_id": "b2"},
		{"turn": 3, "tuning_batch_id": "batch_2", "agent_action_id": "b3"},
	}
	batch := mixLatestTuneBatch(turns)
	if len(batch) != 3 || cleanContextText(batch[0]["agent_action_id"]) != "b1" {
		t.Fatalf("batch = %#v", batch)
	}
	ids := mixTuneBatchActionIDs(batch)
	if len(ids) != 3 || ids[0] != "b1" || ids[2] != "b3" {
		t.Fatalf("ids = %#v", ids)
	}
}

func TestMixLatestTuneBatchFallsBackToTurnReset(t *testing.T) {
	turns := []map[string]any{
		{"turn": 1, "agent_action_id": "a1"},
		{"turn": 2, "agent_action_id": "a2"},
		{"turn": 3, "agent_action_id": "a3"},
		{"turn": 1, "agent_action_id": "b1"},
		{"turn": 2, "agent_action_id": "b2"},
		{"turn": 3, "agent_action_id": "b3"},
	}
	batch := mixLatestTuneBatch(turns)
	if len(batch) != 3 || cleanContextText(batch[0]["agent_action_id"]) != "b1" {
		t.Fatalf("batch = %#v", batch)
	}
}

func TestEnsureMixControlPlanCreatesBuiltInVolumeMacro(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "balance vocal")
	observation := map[string]any{"mixboard": map[string]any{"status": "ready"}}
	plan := ensureMixControlPlan(observation, session)
	if cleanContextText(plan["schema_version"]) != mixControlPlanSchemaVersion {
		t.Fatalf("plan = %#v", plan)
	}
	panel := mapValue(plan["macro_control_panel"])
	controls := mixMacroControlsFromPanel(panel)
	if len(controls) != 1 {
		t.Fatalf("controls = %#v", controls)
	}
	macro := controls[0]
	if cleanContextText(macro["control"]) != mixBuiltInVolumeMacroControl || cleanContextText(macro["track_id"]) != "track_1" {
		t.Fatalf("macro = %#v", macro)
	}
	readiness := mapValue(plan["readiness"])
	if readiness["can_start_fast_loop"] != true {
		t.Fatalf("readiness = %#v", readiness)
	}
	board := mapValue(observation["mixboard"])
	if len(mapValue(board["mix_control_plan"])) == 0 {
		t.Fatalf("board = %#v", board)
	}
}

func TestMixMacroExecutionsExposeUIMutations(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "balance vocal")
	macro := mixBuiltInVolumeMacro(session, -6)
	upserts := mixMacroUpsertExecutions(macro)
	if len(upserts) != 1 || cleanContextText(upserts[0]["command_name"]) != "control_add_macro" {
		t.Fatalf("upserts = %#v", upserts)
	}
	upsertResult := mapValue(upserts[0]["result"])
	if cleanContextText(upsertResult["ui_action"]) != "rack_macro_upserted" {
		t.Fatalf("upsert result = %#v", upsertResult)
	}
	values := mixMacroValueExecution(macro, -5.25, harness.InvokeResponse{Status: "ok"})
	if len(values) != 1 || cleanContextText(values[0]["command_name"]) != "control_set_macro_values" {
		t.Fatalf("values = %#v", values)
	}
	valueResult := mapValue(values[0]["result"])
	if cleanContextText(valueResult["ui_action"]) != "rack_macro_value_changed" || mixFloatNumber(valueResult["value"]) != -5.25 {
		t.Fatalf("value result = %#v", valueResult)
	}
}

func TestMixTuningRejectsNonTrackTarget(t *testing.T) {
	server := New(nil, nil, nil)
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "clip", ID: "clip_1", Label: "Clip", Confidence: "high"}, "降低当前片段")
	data := mixSessionWorkflowData("conv_mix", "goal_1", "run_1", map[string]any{}, session)
	interaction := PendingInteraction{ConversationID: "conv_mix", GoalID: "goal_1", RunID: "run_1", Payload: data, Data: data}
	resp := server.continueMixSessionInteraction(context.Background(), interaction, map[string]any{"ask_vit_mix_action": true, "legacy_mix_session_workflow": true}, "execute_single_mix_tick")
	if resp.CurrentStep != mixStateFailed {
		t.Fatalf("step = %q resp=%+v", resp.CurrentStep, resp)
	}
	if !strings.Contains(resp.Reply, "只支持当前轨道音量") {
		t.Fatalf("reply = %q", resp.Reply)
	}
}

func TestMixTuningReadsTrackVolumeAndStep(t *testing.T) {
	state := map[string]any{
		"tracks": []map[string]any{{
			"track_id":  "track_1",
			"volume_db": -3.0,
		}},
	}
	if got := mixTrackVolumeDB(state, "track_1"); got != -3.0 {
		t.Fatalf("volume = %v", got)
	}
	if got := mixTuningStepDB("降低当前轨道", nil); got != -mixAutoTuneStepDB {
		t.Fatalf("lower step = %v", got)
	}
	if got := mixTuningStepDB("提高当前轨道", map[string]any{"step_db": 4.0}); got != mixAutoTuneStepDB {
		t.Fatalf("clamped step = %v", got)
	}
	if got := mixTuningStepDB("降低当前轨道", map[string]any{"user_note": "音量增大"}); got != mixAutoTuneStepDB {
		t.Fatalf("user note should override toward louder, step = %v", got)
	}
	if got := mixTuningStepDB("降低当前轨道", map[string]any{"user_note": "增大音量"}); got != mixAutoTuneStepDB {
		t.Fatalf("user note should override toward louder regardless of word order, step = %v", got)
	}
	if got := mixTuningStepDB("", map[string]any{"fields": map[string]any{"user_note": "把音量调大一点"}}); got != mixAutoTuneStepDB {
		t.Fatalf("field user note should raise volume, step = %v", got)
	}
}

func TestMixSessionMapPreservesRuntimeFields(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "balance vocal")
	session.State = mixStateWaitingReview
	session.ExecutorType = mixFallbackExecutorType
	session.ExecutorVersion = mixFallbackExecutorVersion
	session.ReviewStatus = mixReviewEffective
	session.StopReason = mixStopReasonNoSignificantChange
	session.InteractionPhase = mixInteractionWaitingPlannerReview
	session.MixBoardVisibility = mixBoardVisibilityExecution
	session.PlannerPolicy = mixPlannerPolicyAutoAssist
	session.FastModelProfile = "fast-local"
	session.JournalRefs = []string{"act_1", "act_2"}
	session.ApprovedPlugins = []string{"eq"}
	session.Preparation = []map[string]any{{"id": "observation", "status": "ready"}}

	roundTrip := mixSessionFromMap(mixSessionMap(session))
	if roundTrip.State != mixStateWaitingReview {
		t.Fatalf("state = %q", roundTrip.State)
	}
	if roundTrip.ExecutorType != mixFallbackExecutorType || roundTrip.ExecutorVersion != mixFallbackExecutorVersion {
		t.Fatalf("executor = %q %q", roundTrip.ExecutorType, roundTrip.ExecutorVersion)
	}
	if roundTrip.ReviewStatus != mixReviewEffective || roundTrip.StopReason != mixStopReasonNoSignificantChange {
		t.Fatalf("review/stop = %q %q", roundTrip.ReviewStatus, roundTrip.StopReason)
	}
	if roundTrip.InteractionPhase != mixInteractionWaitingPlannerReview || roundTrip.MixBoardVisibility != mixBoardVisibilityExecution {
		t.Fatalf("interaction protocol = %q %q", roundTrip.InteractionPhase, roundTrip.MixBoardVisibility)
	}
	if roundTrip.PlannerPolicy != mixPlannerPolicyAutoAssist || roundTrip.FastModelProfile != "fast-local" {
		t.Fatalf("planner/fast profile = %q %q", roundTrip.PlannerPolicy, roundTrip.FastModelProfile)
	}
	if len(roundTrip.JournalRefs) != 2 || roundTrip.JournalRefs[1] != "act_2" {
		t.Fatalf("journal refs = %#v", roundTrip.JournalRefs)
	}
	if len(roundTrip.Preparation) != 1 || cleanContextText(roundTrip.Preparation[0]["id"]) != "observation" {
		t.Fatalf("preparation = %#v", roundTrip.Preparation)
	}
}

func TestBuildMixTickPacketAllowsPlanReadyVirtualControls(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "clean low mids")
	session.FastModelProfile = "fast-local"
	observation := map[string]any{
		"status":   "ready",
		"mixboard": map[string]any{"status": "ready"},
		"goal_control_surface": map[string]any{
			"schema_version": mixcontrolsurface.SchemaVersion,
			"readiness":      mixcontrolsurface.ReadinessPlanReady,
			"selected_chain": []map[string]any{{
				"role":            "tone_balance",
				"type":            "eq",
				"profile_status":  mixcontrolsurface.ProfileReady,
				"instance_status": mixcontrolsurface.InstanceExisting,
				"selected_plugin": map[string]any{"name": "TDR Nova"},
				"instance":        map[string]any{"track_id": "track_1", "plugin_id": "nova_1"},
				"proposed_controls": []map[string]any{{
					"name":         "control low mids with band 2",
					"component_id": "band_2",
				}},
			}},
		},
	}
	packet := buildMixTickPacket(session, observation, "")
	if cleanContextText(packet["status"]) != "ready" {
		t.Fatalf("packet status = %#v", packet)
	}
	if cleanContextText(packet["fast_model_profile"]) != "fast-local" {
		t.Fatalf("fast model profile = %#v", packet)
	}
	strategy := mapValue(packet["model_strategy"])
	if cleanContextText(strategy["route"]) != "mix_tick" || cleanContextText(strategy["reasoning_effort"]) != "low" || cleanContextText(strategy["chain_of_thought"]) != "disabled" {
		t.Fatalf("model strategy = %#v", strategy)
	}
	foundVirtual := false
	for _, control := range mapRowsValue(packet["allowed_controls"]) {
		if cleanContextText(control["control_kind"]) == "plugin_virtual_control" && cleanContextText(control["tool"]) == "plugin_grabber.apply_control" {
			foundVirtual = true
		}
	}
	if !foundVirtual {
		t.Fatalf("allowed_controls = %#v", packet["allowed_controls"])
	}
}

func TestBuildMixTickPacketNeedsConfirmationDoesNotExecute(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "balance vocal")
	observation := map[string]any{
		"status":   "ready",
		"mixboard": map[string]any{"status": "ready"},
		"goal_control_surface": map[string]any{
			"schema_version": mixcontrolsurface.SchemaVersion,
			"readiness":      mixcontrolsurface.ReadinessNeedsConfirmation,
			"selected_chain": []map[string]any{{
				"role":            "dynamic_stability",
				"type":            "dynamics",
				"profile_status":  mixcontrolsurface.ProfileReady,
				"instance_status": mixcontrolsurface.InstanceNeedsLoad,
				"selected_plugin": map[string]any{"name": "TDR Nova"},
				"proposed_controls": []map[string]any{{
					"name": "control compression amount",
				}},
			}},
		},
	}
	packet := buildMixTickPacket(session, observation, "")
	if cleanContextText(packet["status"]) != "needs_confirmation" {
		t.Fatalf("packet = %#v", packet)
	}
	if len(mapRowsValue(packet["blocked_controls"])) == 0 {
		t.Fatalf("blocked_controls = %#v", packet["blocked_controls"])
	}
}

func TestBuildMixTickPacketLearningRequiredForMissingProfile(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "compress vocal")
	observation := map[string]any{
		"status":   "ready",
		"mixboard": map[string]any{"status": "ready"},
		"goal_control_surface": map[string]any{
			"schema_version":       mixcontrolsurface.SchemaVersion,
			"readiness":            mixcontrolsurface.ReadinessBlocked,
			"next_required_action": "learn_plugin_profile",
			"selected_chain": []map[string]any{{
				"role":            "dynamic_stability",
				"type":            "dynamics",
				"profile_status":  mixcontrolsurface.ProfileMissing,
				"instance_status": mixcontrolsurface.InstanceExisting,
				"selected_plugin": map[string]any{"id": "zl", "name": "ZL Compressor"},
				"instance":        map[string]any{"track_id": "track_1", "plugin_id": "zl_1"},
			}},
		},
	}
	packet := buildMixTickPacket(session, observation, "")
	if cleanContextText(packet["status"]) != mixInteractionLearningRequired {
		t.Fatalf("packet = %#v", packet)
	}
	request := mapValue(packet["plugin_learning_request"])
	if cleanContextText(request["plugin_name"]) != "ZL Compressor" || cleanContextText(request["next_required_action"]) != "learn_plugin_profile" {
		t.Fatalf("learning request = %#v", request)
	}
}

func TestMixPlannerPrepMarksMissingProfileAsLearningRequired(t *testing.T) {
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "compress vocal")
	prep := initialMixPlannerPrep(session, map[string]any{
		"goal_detail": "stabilize vocal dynamics",
	})
	prep["allow_plugin_loads"] = true
	observation := map[string]any{
		"status":   "ready",
		"mixboard": map[string]any{"status": "ready"},
		"goal_control_surface": map[string]any{
			"schema_version":       mixcontrolsurface.SchemaVersion,
			"readiness":            mixcontrolsurface.ReadinessBlocked,
			"next_required_action": "learn_plugin_profile",
			"blockers":             []string{"ZL Compressor requires Plugin Grabber learning"},
			"selected_chain": []map[string]any{{
				"role":            "dynamic_stability",
				"type":            "dynamics",
				"profile_status":  mixcontrolsurface.ProfileMissing,
				"instance_status": mixcontrolsurface.InstanceExisting,
				"selected_plugin": map[string]any{"id": "zl", "name": "ZL Compressor"},
				"proposed_controls": []map[string]any{{
					"name":         "control compression amount",
					"component_id": "main_compressor",
				}},
			}},
		},
	}

	fillMixPlannerPrepFromObservation(prep, observation, session)
	if cleanContextText(prep["stage"]) != mixInteractionLearningRequired || boolValue(prep["ready_to_publish_mixboard"]) {
		t.Fatalf("planner prep = %#v", prep)
	}
	request := mapValue(prep["plugin_learning_request"])
	if cleanContextText(request["plugin_name"]) != "ZL Compressor" || cleanContextText(request["next_required_action"]) != "learn_plugin_profile" {
		t.Fatalf("learning request = %#v", request)
	}
}

func TestSelectMixTickControlRejectsRawAndOutsideControls(t *testing.T) {
	packet := map[string]any{
		"allowed_controls": []map[string]any{{
			"control_id":   "macro:vol",
			"control_name": mixBuiltInVolumeMacroControl,
			"control_kind": "macro",
		}},
	}
	if _, err := selectMixTickControl(packet, map[string]any{"tool": "set_plugin_param"}); err == nil {
		t.Fatal("raw param decision should be rejected")
	}
	if _, err := selectMixTickControl(packet, map[string]any{"control_id": "virtual:other"}); err == nil {
		t.Fatal("outside control should be rejected")
	}
	control, err := selectMixTickControl(packet, map[string]any{"control_id": "macro:vol"})
	if err != nil || cleanContextText(control["control_name"]) != mixBuiltInVolumeMacroControl {
		t.Fatalf("selected control=%#v err=%v", control, err)
	}
}

func TestEnterDiscussionActionDeprecated(t *testing.T) {
	server := New(nil, nil, nil)
	session := pendingMixSession(mixModeCo, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "balance vocal")
	session.State = mixStateWaitingReview
	data := mixSessionWorkflowData("conv_mix", "goal_1", "run_1", map[string]any{}, session)
	data["mix_observation"] = map[string]any{"status": "ready", "mixboard": map[string]any{"status": "ready"}}
	interaction := PendingInteraction{ConversationID: "conv_mix", GoalID: "goal_1", RunID: "run_1", Payload: data, Data: data}
	resp := server.continueMixSessionInteraction(context.Background(), interaction, nil, "enter_discussion")
	got := mapValue(resp.MixSession)
	if resp.CurrentStep != "deprecated_mix_planner_action" || cleanContextText(got["mixboard_visibility"]) != mixBoardVisibilityHidden {
		t.Fatalf("mix session = %#v", got)
	}
	if len(resp.InteractionRequests) != 0 {
		t.Fatalf("deprecated discussion action should not create interactions: %#v", resp.InteractionRequests)
	}
}

func TestUpdateMixBoardRuntimeStatePersistsBoardAndContextPack(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "current.json")
	contextPath := filepath.Join(dir, "context_pack.json")
	if err := os.WriteFile(boardPath, []byte(`{"schema_version":"mixboard.v1","status":"ready"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contextPath, []byte(`{"schema_version":"mixboard_context_pack.v1","session_header":{},"latest_observation":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "balance vocal")
	session.State = mixStateWaitingReview
	session.RoundCount = 2
	session.ReviewStatus = mixReviewEffective
	session.JournalRefs = []string{"act_1"}
	turns := []map[string]any{{
		"turn":             1,
		"executor_type":    mixFallbackExecutorType,
		"executor_version": mixFallbackExecutorVersion,
		"control":          "track.volume",
		"review_status":    mixReviewEffective,
	}}
	observation := map[string]any{
		"board_path":        boardPath,
		"context_pack_path": contextPath,
		"mixboard":          map[string]any{"status": "ready"},
		"goal_control_surface": map[string]any{
			"schema_version":       mixcontrolsurface.SchemaVersion,
			"readiness":            mixcontrolsurface.ReadinessBlocked,
			"next_required_action": "learn_plugin_profile",
			"blockers":             []string{"eq requires Plugin Grabber learning"},
		},
	}

	updateMixBoardRuntimeState(observation, session, turns)
	cardBoard := mapValue(observation["mixboard"])
	if cleanContextText(cardBoard["session_state"]) != mixStateWaitingReview {
		t.Fatalf("card board = %#v", cardBoard)
	}
	if cleanContextText(cardBoard["executor_type"]) != mixFallbackExecutorType {
		t.Fatalf("card executor = %#v", cardBoard)
	}

	var board map[string]any
	raw, err := os.ReadFile(boardPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &board); err != nil {
		t.Fatal(err)
	}
	rollbackAvailable, _ := board["rollback_available"].(bool)
	if cleanContextText(board["session_state"]) != mixStateWaitingReview || !rollbackAvailable {
		t.Fatalf("persisted board = %#v", board)
	}
	if cleanContextText(mapValue(board["goal_control_surface"])["readiness"]) != mixcontrolsurface.ReadinessBlocked {
		t.Fatalf("persisted board goal_control_surface = %#v", board)
	}

	var contextPack map[string]any
	raw, err = os.ReadFile(contextPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &contextPack); err != nil {
		t.Fatal(err)
	}
	header := mapValue(contextPack["session_header"])
	autoTune := mapValue(contextPack["auto_tune"])
	if cleanContextText(header["session_state"]) != mixStateWaitingReview {
		t.Fatalf("context header = %#v", header)
	}
	if cleanContextText(autoTune["executor_version"]) != mixFallbackExecutorVersion {
		t.Fatalf("context auto_tune = %#v", autoTune)
	}
	if cleanContextText(mapValue(contextPack["goal_control_surface"])["next_required_action"]) != "learn_plugin_profile" {
		t.Fatalf("context goal_control_surface = %#v", contextPack)
	}
}

func TestInvokeMixSessionEntryWorkflow(t *testing.T) {
	server := New(nil, nil, nil)
	resp, handled := server.invokeMixSessionEntryWorkflow(context.Background(), harness.InvokeRequest{
		Tool: "mix.session_entry",
		Args: map[string]any{
			"mix_mode":  "co_mix",
			"goal_text": "协同混音当前轨道",
		},
		Context: map[string]any{
			"mix_requested":     true,
			"conversation_id":   "conv_mix",
			"selected_track_id": "track_1",
		},
	})
	if !handled {
		t.Fatal("invoke was not handled")
	}
	if resp.Status != "ok" || resp.RequiresConfirmation {
		t.Fatalf("status = %q", resp.Status)
	}
	if !boolValue(resp.Result["disabled"]) {
		t.Fatalf("legacy mix session invoke should be disabled: %#v", resp.Result)
	}
	if len(mapValue(resp.Result["mix_session"])) != 0 {
		t.Fatalf("disabled invoke must not create mix_session: %#v", resp.Result)
	}
}

func TestInvokeMixSessionEntryPlanModeDoesNotCreateSession(t *testing.T) {
	server := New(nil, nil, nil)
	resp, handled := server.invokeMixSessionEntryWorkflow(context.Background(), harness.InvokeRequest{
		Tool: "mix.session_entry",
		Args: map[string]any{
			"mix_mode":  "auto_mix",
			"goal_text": "自动混当前轨道",
		},
		Context: map[string]any{
			"mix_requested":     true,
			"agent_mode":        "plan",
			"conversation_id":   "conv_mix",
			"selected_track_id": "track_1",
		},
	})
	if !handled {
		t.Fatal("invoke was not handled")
	}
	if resp.Status != "ok" || resp.RequiresConfirmation {
		t.Fatalf("plan mode invoke should be read-only: status=%q requires_confirmation=%v", resp.Status, resp.RequiresConfirmation)
	}
	result := resp.Result
	if len(mapValue(result["mix_session"])) != 0 {
		t.Fatalf("plan mode should not return mix_session: %#v", result)
	}
	if len(server.mixSessions) != 0 {
		t.Fatalf("plan mode should not store mix sessions: %#v", server.mixSessions)
	}
}
