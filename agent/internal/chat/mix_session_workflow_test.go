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

func TestMixSessionEntryFromNaturalLanguageCreatesConfirmationCard(t *testing.T) {
	server := New(nil, nil, nil)
	req := ChatRequest{
		ConversationID: "conv_mix",
		Message:        "帮我自动混当前轨道",
		Context: map[string]any{
			"selected_track_id":   "track_1",
			"selected_track_name": "Vocal",
		},
	}

	resp, handled := server.runMixSessionEntryChat(context.Background(), "conv_mix", req, agentModeDefault)
	if !handled {
		t.Fatal("mix request was not handled")
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingConfirmation) {
		t.Fatalf("goal_status = %q", resp.GoalStatus)
	}
	if got := cleanContextText(resp.MixSession["mode"]); got != mixModeAuto {
		t.Fatalf("mode = %q", got)
	}
	target := mapValue(resp.MixSession["target_ref"])
	if target["kind"] != "track" || target["id"] != "track_1" {
		t.Fatalf("target_ref = %#v", target)
	}
	if len(resp.InteractionRequests) != 1 {
		t.Fatalf("interaction_requests = %#v", resp.InteractionRequests)
	}
	if _, exists := server.mixSessions[cleanContextText(resp.MixSession["mix_session_id"])]; exists {
		t.Fatal("pending mix session should not be stored before user confirmation")
	}
}

func TestMixSessionEntryFromNaturalLanguageAutoStemMixPhrase(t *testing.T) {
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

	resp, handled := server.runMixSessionEntryChat(context.Background(), "conv_mix_plan", req, agentModePlan)
	if !handled {
		t.Fatal("mix request was not handled")
	}
	if len(resp.MixSession) != 0 || len(server.mixSessions) != 0 {
		t.Fatalf("plan mode should not create mix session: resp=%#v stored=%#v", resp.MixSession, server.mixSessions)
	}
	if resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("goal_status = %q", resp.GoalStatus)
	}
}

func TestMixSessionConfirmStoresReadyForObservation(t *testing.T) {
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
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("goal_status = %q", resp.GoalStatus)
	}
	stored, ok := server.mixSessions[session.MixSessionID]
	if !ok {
		t.Fatal("confirmed mix session was not stored")
	}
	if stored.State != mixStateObservationUnavailable {
		t.Fatalf("state = %q", stored.State)
	}
	if stored.BlockingPoint == "" {
		t.Fatalf("blocking point should explain observation gap: %#v", stored)
	}
	if len(mapValue(resp.WorkflowData["mix_observation"])) == 0 {
		t.Fatalf("workflow data should include mix_observation: %#v", resp.WorkflowData)
	}
}

func TestMixBoardStatusInteractionOffersTuningActions(t *testing.T) {
	server := New(nil, nil, nil)
	session := pendingMixSession(mixModeAuto, MixTargetRef{Kind: "track", ID: "track_1", Label: "Vocal", Confidence: "high"}, "降低当前轨道")
	session.State = mixStateObservationReady
	interaction := PendingInteraction{ConversationID: "conv_mix", GoalID: "goal_1", RunID: "run_1", RequestContext: map[string]any{}}
	req := server.mixBoardStatusInteraction(interaction, session, map[string]any{"status": "ready"})
	ids := map[string]bool{}
	for _, action := range req.Actions {
		ids[action.ID] = true
	}
	if !ids["start_mix_tuning"] || !ids["revise_mixboard"] || !ids["refresh_observation"] {
		t.Fatalf("actions = %+v", req.Actions)
	}
	if ids["stop_mix_tuning"] || ids["done"] {
		t.Fatalf("actions = %+v", req.Actions)
	}
	session.State = mixStateTuningRunning
	session.JournalRefs = []string{"action_1"}
	req = server.mixBoardStatusInteraction(interaction, session, map[string]any{"status": "ready"})
	ids = map[string]bool{}
	for _, action := range req.Actions {
		ids[action.ID] = true
	}
	if !ids["stop_mix_tuning"] || !ids["rollback_last_mix_turn"] || ids["done"] {
		t.Fatalf("running actions = %+v", req.Actions)
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
		if action.ID == "start_mix_tuning" || action.ID == "auto_tune_mix" {
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
	resp := server.continueMixSessionInteraction(context.Background(), interaction, nil, "start_mix_tuning")
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
	if len(roundTrip.JournalRefs) != 2 || roundTrip.JournalRefs[1] != "act_2" {
		t.Fatalf("journal refs = %#v", roundTrip.JournalRefs)
	}
	if len(roundTrip.Preparation) != 1 || cleanContextText(roundTrip.Preparation[0]["id"]) != "observation" {
		t.Fatalf("preparation = %#v", roundTrip.Preparation)
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
	if resp.Status != "needs_confirmation" {
		t.Fatalf("status = %q", resp.Status)
	}
	result := resp.Result
	mix := mapValue(result["mix_session"])
	if cleanContextText(mix["mode"]) != mixModeCo {
		t.Fatalf("mix_session = %#v", mix)
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
