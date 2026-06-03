package chat

import (
	"context"
	"os"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/policy"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("VIT_CONTEXT_SNAPSHOT_PATH", "off")
	os.Exit(m.Run())
}

func TestLLMMessagesFromTypedProjectHistory(t *testing.T) {
	messages := llmMessagesFromProjectHistory(map[string]any{
		"conversation_messages": []history.ConversationMessage{
			{Role: "user", Content: "search plugin library"},
			{Role: "assistant", Content: "TDR Nova is an EQ."},
		},
	})
	if len(messages) != 2 || messages[0].Role != "user" || messages[0].Content != "search plugin library" || messages[1].Role != "assistant" {
		t.Fatalf("messages = %+v", messages)
	}
}

func TestProjectPathFromChatContext(t *testing.T) {
	if got := projectPathFromChatContext(map[string]any{"project_path": `D:\song\worktree.vit`}); got != `D:\song\worktree.vit` {
		t.Fatalf("direct project path = %q", got)
	}
	if got := projectPathFromChatContext(map[string]any{"project_history": map[string]any{"current_project_path": "/tmp/song.vit"}}); got != "/tmp/song.vit" {
		t.Fatalf("nested project path = %q", got)
	}
	if got := projectPathFromChatContext(map[string]any{"project_path": "   ", "current_project_path": nil}); got != "" {
		t.Fatalf("empty project path = %q", got)
	}
}

func TestParseChatToolCommand(t *testing.T) {
	tool, args, confirmed, ok, err := parseChatToolCommand(`/tool workspace.grep {"root_id":"godot","query":"rack","max_results":2}`)
	if err != nil {
		t.Fatalf("parseChatToolCommand: %v", err)
	}
	if !ok || confirmed || tool != "workspace.grep" {
		t.Fatalf("parsed = tool=%q confirmed=%v ok=%v", tool, confirmed, ok)
	}
	if args["root_id"] != "godot" || args["query"] != "rack" || args["max_results"].(float64) != 2 {
		t.Fatalf("args = %+v", args)
	}

	tool, args, confirmed, ok, err = parseChatToolCommand(`/tool! version.checkpoint {"message":"smoke"}`)
	if err != nil {
		t.Fatalf("parseChatToolCommand confirmed: %v", err)
	}
	if !ok || !confirmed || tool != "version.checkpoint" || args["message"] != "smoke" {
		t.Fatalf("confirmed parsed = tool=%q args=%+v confirmed=%v ok=%v", tool, args, confirmed, ok)
	}
}

func TestRenderChatToolResultOmitsSnapshotXML(t *testing.T) {
	reply := renderChatToolResult(harness.InvokeResponse{
		Status:      "ok",
		Tool:        "project.snapshot_export",
		CommandName: "project_snapshot_export",
		Result: map[string]any{
			"snapshot_bytes": 42,
			"snapshot_xml":   strings.Repeat("x", 1200),
		},
	})
	if strings.Contains(reply, strings.Repeat("x", 100)) {
		t.Fatalf("reply leaked snapshot XML: %s", reply)
	}
	if !strings.Contains(reply, "<omitted") || !strings.Contains(reply, "snapshot_bytes") {
		t.Fatalf("reply did not summarize snapshot result: %s", reply)
	}
}

func TestDevGoalSmokeCreatesPendingConfirmation(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp, handled := server.handleDevChatCommand(context.Background(), "chat_test", ChatRequest{
		Message: "/smoke goal",
		Context: map[string]any{
			"goal_id": "goal_test",
			"run_id":  "run_test",
		},
	})
	if !handled || !resp.NeedsConfirmation || resp.PlanID == "" {
		t.Fatalf("resp = %+v handled=%v", resp, handled)
	}
	server.mu.Lock()
	plan, ok := server.pending[resp.PlanID]
	server.mu.Unlock()
	if !ok || plan.Workflow != "goal_ui_smoke" || plan.Context["goal_id"] != "goal_test" {
		t.Fatalf("pending plan = %+v ok=%v", plan, ok)
	}
}

func TestDevPluginSemanticSmoke(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp, handled := server.handleDevChatCommand(context.Background(), "chat_test", ChatRequest{
		Message: "/smoke plugin_semantics",
	})
	if !handled {
		t.Fatal("expected plugin semantic smoke to be handled")
	}
	for _, want := range []string{"Plugin semantic smoke:", "natural language route: ok", "semantic search: ok", "Result: ok"} {
		if !strings.Contains(resp.Reply, want) {
			t.Fatalf("reply missing %q in:\n%s", want, resp.Reply)
		}
	}
}

func TestBeginChatGoalStartsFreshAfterTerminalContextGoal(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	old := server.harness.BeginGoal("previous completed goal")
	server.harness.CompleteGoal(old.GoalID, nil)

	next := server.beginChatGoal("chat_test", "new chat request", map[string]any{
		"goal_id": old.GoalID,
		"run_id":  old.RunID,
	})
	if next.GoalID == "" || next.GoalID == old.GoalID {
		t.Fatalf("expected fresh goal after completed context goal: old=%+v next=%+v", old, next)
	}
	if next.Status != agentruntime.StatusRunning {
		t.Fatalf("next status = %q", next.Status)
	}
	if next.Summary != "new chat request" {
		t.Fatalf("next summary = %q", next.Summary)
	}
}

func TestFreshAgentLoopGoalIDsDropTerminalContextGoal(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	old := server.harness.BeginGoal("previous completed goal")
	server.harness.CompleteGoal(old.GoalID, nil)

	goalID, runID := server.freshAgentLoopGoalIDs(map[string]any{
		"goal_id": old.GoalID,
		"run_id":  old.RunID,
	})
	if goalID != "" || runID != "" {
		t.Fatalf("fresh agentloop should drop terminal goal context, got goal=%q run=%q", goalID, runID)
	}
}

func TestRoutingUsesAgentLoopBeforeFastPaths(t *testing.T) {
	if !shouldUseAgentLoop("add a track", map[string]any{}) {
		t.Fatal("default mode should route natural language through AgentLoop")
	}
	if !shouldUseAgentLoop("add a track", map[string]any{"agent_mode": "goal"}) {
		t.Fatal("goal mode should route to AgentLoop")
	}
	if !shouldUseAgentLoop("add a track", map[string]any{"agent_mode": "plan"}) {
		t.Fatal("plan mode should route to AgentLoop with read-only tools")
	}
	if !shouldUseAgentLoop("/goal add a track", map[string]any{}) {
		t.Fatal("/goal should route to AgentLoop")
	}
	if !shouldUseAgentLoop("continue", map[string]any{"goal_id": "goal_1"}) {
		t.Fatal("continue with active goal context should route to AgentLoop")
	}
	if shouldUseAgentLoop("/tools", map[string]any{}) {
		t.Fatal("explicit slash/dev commands should stay outside AgentLoop")
	}
	if shouldUseAgentLoop("add a track", map[string]any{"use_legacy_chat": true}) {
		t.Fatal("legacy chat context should keep the old one-shot path")
	}
	text := "load the EQ and reverb plugins you found"
	if !shouldUseAgentLoop(text, map[string]any{}) {
		t.Fatal("compound default requests should route to AgentLoop")
	}
	if cmds := synthesizePluginLibraryCommands(text); len(cmds) != 0 {
		t.Fatalf("load intent should not be downgraded to library-only search: %+v", cmds)
	}
}
func TestPluginLoadCountQuestionDoesNotRouteToLoadWorkflow(t *testing.T) {
	text := "how many plugins are loaded on the current track?"
	if looksLikePluginGrabberLoadIntent(text) {
		t.Fatal("plugin count question should not be treated as a load intent")
	}
	if cmds := synthesizePluginGrabberLoadCommands(text, map[string]any{}); len(cmds) != 0 {
		t.Fatalf("count question should not synthesize load commands: %+v", cmds)
	}
	if cmd, ok := coercePluginGrabberLoadCommand([]map[string]any{{
		"cmd":          pluginGrabberLoadCommand,
		"plugin_query": "how many?",
	}}, text, map[string]any{}); ok {
		t.Fatalf("count question should not coerce model command into load workflow: %+v", cmd)
	}
	if !shouldUseAgentLoop(text, map[string]any{}) {
		t.Fatal("default questions should still enter AgentLoop")
	}
}

func TestFreshAgentLoopConversationIgnoresHistoryUnlessReferenced(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.remember("chat_test", "新建一条轨道，然后加载一个 TDR Nova 均衡器", "已新建轨道并加载 TDR Nova。")

	if got := server.freshAgentLoopConversation("chat_test", "新建一条轨道", nil); len(got) != 0 {
		t.Fatalf("fresh standalone task should not carry prior turns: %+v", got)
	}
	if got := server.freshAgentLoopConversation("chat_test", "像刚才一样再来一次", nil); len(got) == 0 {
		t.Fatal("explicit prior-context task should carry recent turns")
	}
}

func TestAgentModeHelpers(t *testing.T) {
	if got := agentModeFromContext(map[string]any{}); got != agentModeDefault {
		t.Fatalf("default mode = %q", got)
	}
	if got := agentModeFromContext(map[string]any{"agent_mode": "plan"}); got != agentModePlan {
		t.Fatalf("plan mode = %q", got)
	}
	if got := agentModeFromContext(map[string]any{"use_goal_runner": true}); got != agentModeGoal {
		t.Fatalf("goal override mode = %q", got)
	}
	if shouldExposeAgentPlan(agentModeDefault, ChatResponse{Reply: "ok"}) {
		t.Fatal("default response without confirmation should not expose agent_plan")
	}
	if shouldExposeAgentPlan(agentModeDefault, ChatResponse{NeedsConfirmation: true}) {
		t.Fatal("default confirmation response should not expose agent_plan")
	}
	if !shouldExposeAgentPlan(agentModePlan, ChatResponse{Reply: "plan"}) {
		t.Fatal("plan mode response should expose agent_plan")
	}
}

func TestPlanModeBlocksProjectMutationButAllowsWeb(t *testing.T) {
	decisions := policy.Analyze([]map[string]any{
		{"cmd": "create_midi_clip", "track_id": "1007"},
		{"cmd": "web_fetch", "url": "https://example.com"},
		{"cmd": "shell_run", "command": "echo hi"},
	})
	blocked := planModeBlockedDecisions(decisions)
	if len(blocked) != 2 {
		t.Fatalf("blocked decisions = %+v", blocked)
	}
	if blocked[0].Name != "create_midi_clip" || blocked[1].Name != "shell_run" {
		t.Fatalf("unexpected blocked decisions = %+v", blocked)
	}
}

func TestPlanModeBlockedResponseDoesNotQueueConfirmation(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp, handled := server.chatResponseForCommands(context.Background(), "chat_test", "create midi", "", []map[string]any{
		{"cmd": "create_midi_clip", "track_id": "1007"},
	}, map[string]any{"agent_mode": "plan"})
	if !handled {
		t.Fatal("expected plan mode block response")
	}
	if resp.NeedsConfirmation || resp.PlanID != "" {
		t.Fatalf("plan mode should not queue confirmation: %+v", resp)
	}
	if resp.AgentMode != agentModePlan || resp.AgentPlan == nil || resp.AgentPlan.Summary == "" {
		t.Fatalf("missing plan-mode agent plan: %+v", resp)
	}
}

func TestDefaultModeDoesNotExposeAgentPlanCard(t *testing.T) {
	if shouldExposeAgentPlan(agentModeDefault, ChatResponse{Reply: "ok"}) {
		t.Fatal("default plain replies should not expose agent_plan")
	}
	plan := simpleAgentPlan("goal_1", "run_1", agentruntime.StatusWaitingConfirmation, "confirm", "", "", nil)
	if agentPlanForMode(agentModeDefault, plan) != nil {
		t.Fatal("default mode should filter explicit confirmation agent_plan")
	}
	if agentPlanForMode(agentModePlan, plan) == nil || agentPlanForMode(agentModeGoal, plan) == nil {
		t.Fatal("plan and goal modes should keep agent_plan")
	}
	if !shouldExposeAgentPlan(agentModePlan, ChatResponse{Reply: "ok"}) {
		t.Fatal("plan mode should expose agent_plan")
	}
	if !shouldExposeAgentPlan(agentModeGoal, ChatResponse{Reply: "ok"}) {
		t.Fatal("goal mode should expose agent_plan")
	}
	if shouldExposeAgentPlan(agentModeDefault, ChatResponse{NeedsConfirmation: true}) {
		t.Fatal("default confirmation replies should not expose agent_plan")
	}
}

func TestActiveGoalPlanHidesEmptyIdleGoal(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	if plan := server.activeGoalPlan(agentruntime.Goal{Status: agentruntime.StatusIdle}, nil); plan != nil {
		t.Fatalf("idle plan = %+v", plan)
	}
}

func TestConfirmationPreviewUsesMidiPatchPreview(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":   "1007",
				"track_name": "Track 1",
				"clips": []any{
					map[string]any{"id": "clip_a", "name": "MIDI Clip", "start_seconds": 0.0, "length_seconds": 4.0},
				},
			},
		},
	})
	server := New(nil, project, nil)
	preview, err := server.confirmationPreview(context.Background(), []policy.Decision{
		{
			Name: "apply_midi_note_patch",
			Risk: policy.RiskConfirm,
			Command: map[string]any{
				"tool": "midi.apply_note_patch",
				"args": map[string]any{
					"operations": []map[string]any{
						{"op": "insert_note", "pitch": 60, "start": 0.0, "length": 1.0, "velocity": 96},
					},
				},
			},
		},
	}, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	})
	if err != nil {
		t.Fatalf("confirmationPreview: %v", err)
	}
	for _, want := range []string{"apply_midi_note_patch [confirm]", "Clip clip_a", "1. insert_note pitch=60 start=0 length=1 velocity=96"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview missing %q in:\n%s", want, preview)
		}
	}
	if strings.Contains(preview, `"args"`) {
		t.Fatalf("preview should be readable, got raw JSON:\n%s", preview)
	}
}

func TestConfirmationPreviewReturnsMidiTargetError(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":   "1007",
				"track_name": "Track 1",
				"clips": []any{
					map[string]any{"id": "clip_a", "name": "MIDI Clip", "start_seconds": 0.0, "length_seconds": 4.0},
				},
			},
		},
	})
	server := New(nil, project, nil)
	preview, err := server.confirmationPreview(context.Background(), []policy.Decision{
		{
			Name: "add_midi_notes_bulk",
			Risk: policy.RiskConfirm,
			Command: map[string]any{
				"tool": "midi.add_notes_bulk",
				"args": map[string]any{
					"clip_id": "1010",
					"notes": []map[string]any{
						{"pitch": 60, "start": 0.0, "duration": 1.0, "velocity": 96},
					},
				},
			},
		},
	}, map[string]any{
		"selected_track_id": "1007",
	})
	if err == nil {
		t.Fatalf("expected preview target error, preview=%q", preview)
	}
	if !strings.Contains(err.Error(), `clip_id "1010" is not present`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPluginGrabberPatchValidationRejectsUnknownIDs(t *testing.T) {
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{ID: "dry level"},
			{ID: "wet level"},
		},
	}
	_, err := validatePluginProfilePatch(pluginProfilePatch{
		QuickControlIDs: []string{"dry level", "made up"},
	}, digest)
	if err == nil {
		t.Fatal("expected unknown id error")
	}
	if !strings.Contains(err.Error(), "made up") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPluginGrabberPatchValidationKeepsFullQuickIDs(t *testing.T) {
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{ID: "dry level"},
			{ID: "wet level"},
			{ID: "threshold"},
		},
	}
	patch, err := validatePluginProfilePatch(pluginProfilePatch{
		QuickControlIDs: []string{"dry level", "wet level", "dry level"},
		DisplayGroups:   map[string]string{"threshold": "Dynamics"},
	}, digest)
	if err != nil {
		t.Fatalf("validatePluginProfilePatch: %v", err)
	}
	if got := strings.Join(patch.QuickControlIDs, ","); got != "dry level,wet level" {
		t.Fatalf("quick ids = %q", got)
	}
	if patch.DisplayGroups["threshold"] != "Dynamics" {
		t.Fatalf("display groups = %+v", patch.DisplayGroups)
	}
}

func TestPluginGrabberDigestIncludesAllParameters(t *testing.T) {
	digest := buildPluginParameterDigest(map[string]any{
		"track_id":  "1007",
		"plugin_id": "plugin_a",
		"parameters": []any{
			map[string]any{"id": "dry level", "display_group": "Mix"},
			map[string]any{"id": "wet level", "display_group": "Mix"},
			map[string]any{"id": "hidden but controllable", "display_group": "Other"},
		},
	})
	if digest.ParameterCount != 3 || len(digest.Parameters) != 3 {
		t.Fatalf("digest dropped parameters: %+v", digest)
	}
}

func TestPluginGrabberContextPackKeepsFullParameterCount(t *testing.T) {
	digest := buildPluginParameterDigest(map[string]any{
		"track_id":       "1007",
		"plugin_id":      "plugin_a",
		"profile_source": "project_profile",
		"plugin_identity": map[string]any{
			"plugin_name": "TDR Nova",
		},
		"quick_controls": []any{
			map[string]any{"param_id": "threshold", "label": "Threshold", "display_group": "Dynamics", "normalized_role": "comp_threshold"},
		},
		"parameters": []any{
			map[string]any{"id": "threshold", "alias": "Threshold", "display_group": "Dynamics", "normalized_role": "comp_threshold", "value": -12.0, "min": -60.0, "max": 0.0, "host_controllable": true},
			map[string]any{"id": "dry level", "display_group": "Mix", "normalized_role": "common_mix", "host_controllable": true},
			map[string]any{"id": "hidden but controllable", "display_group": "Other", "normalized_role": "other", "host_controllable": true},
		},
	})
	pack := buildPluginGrabberContextPack(digest)
	if pack["parameter_count"] != 3 || pack["parameters_retained"] != true {
		t.Fatalf("context pack dropped full parameter count: %+v", pack)
	}
	quick := mapRowsValue(pack["quick_controls"])
	if len(quick) != 1 || quick[0]["param_id"] != "threshold" {
		t.Fatalf("quick controls = %+v", quick)
	}
	hint, ok := quick[0]["semantic_hint"].(map[string]any)
	if !ok {
		t.Fatalf("quick control missing semantic hint: %+v", quick[0])
	}
	if hint["source"] != "local_semantic_hint_rule" || hint["confidence"] != "medium" {
		t.Fatalf("semantic hint should be a sourced weak hint: %+v", hint)
	}
	groups := mapRowsValue(pack["groups"])
	if len(groups) < 3 {
		t.Fatalf("groups should include non-quick parameters too: %+v", groups)
	}
	macros := mapRowsValue(pack["macro_candidates"])
	if len(macros) == 0 || macros[0]["param_id"] != "threshold" || macros[0]["requires_user_review"] != true {
		t.Fatalf("macro candidates should include reviewed quick controls: %+v", macros)
	}
	policy, ok := pack["semantic_hint_policy"].(map[string]any)
	if !ok || policy["kind"] != "weak_local_heuristic" {
		t.Fatalf("semantic hint policy = %+v", pack["semantic_hint_policy"])
	}
}

func TestCoercePluginGrabberExplainFromGetParametersCommand(t *testing.T) {
	cmd, ok := coercePluginGrabberExplainCommand([]map[string]any{
		{
			"cmd":       "get_plugin_parameters",
			"track_id":  "1007",
			"plugin_id": "plugin_a",
		},
	}, "explain current plugin controls", map[string]any{})
	if !ok {
		t.Fatal("expected get_plugin_parameters to be coerced to explain workflow")
	}
	if cmd["cmd"] != pluginGrabberExplainCommand || cmd["track_id"] != "1007" || cmd["plugin_id"] != "plugin_a" {
		t.Fatalf("workflow cmd = %+v", cmd)
	}
}

func TestCoercePluginGrabberLoadFromInstantiateCommand(t *testing.T) {
	cmd, ok := coercePluginGrabberLoadCommand([]map[string]any{
		{
			"cmd":      "instantiate_plugin",
			"name":     "TDR Nova",
			"track_id": "1007",
		},
	}, "load TDR Nova on the current track and grab common controls", map[string]any{})
	if !ok {
		t.Fatal("expected instantiate_plugin to be coerced to grabber workflow")
	}
	if cmd["cmd"] != pluginGrabberLoadCommand || cmd["plugin_query"] != "TDR Nova" || cmd["track_id"] != "1007" {
		t.Fatalf("workflow cmd = %+v", cmd)
	}
}
func TestSynthesizePluginSearchCommand(t *testing.T) {
	cmds := synthesizePluginLibraryCommands("search TDR Nova plugin candidates only do not load")
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["cmd"] != "plugin_search" || cmds[0]["query"] != "TDR Nova" {
		t.Fatalf("cmd = %+v", cmds[0])
	}
}
func TestPluginInventoryQuestionUsesListNotNameSearch(t *testing.T) {
	cmds := synthesizePluginLibraryCommands("what plugins are available in the plugin library?")
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["cmd"] != "plugin_list_available" {
		t.Fatalf("inventory question should list plugins, got %+v", cmds[0])
	}
}
func TestSynthesizePluginSemanticRecommendationCommand(t *testing.T) {
	cmds := synthesizePluginLibraryCommands("recommend an eq plugin")
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["cmd"] != "plugin_semantic_search" || cmds[0]["type"] != "eq" || cmds[0]["query"] != "eq" {
		t.Fatalf("cmd = %+v", cmds[0])
	}

	cmds = synthesizePluginLibraryCommands("\u4f60\u80fd\u4ece\u6570\u636e\u5e93\u91cc\u627e\u5230\u4e00\u4e2a eq \u6548\u679c\u5668\u63a8\u8350\u7ed9\u6211\u5417")
	if len(cmds) != 1 || cmds[0]["cmd"] != "plugin_semantic_search" || cmds[0]["type"] != "eq" {
		t.Fatalf("chinese semantic cmd = %+v", cmds)
	}
}

func TestSynthesizePluginSemanticRecommendationCommandForMultipleTypes(t *testing.T) {
	cmds := synthesizePluginLibraryCommands("\u5e2e\u6211\u627e\u4e24\u4e2a\u63d2\u4ef6\uff0c\u4e00\u4e2a\u5747\u8861\uff0c\u4e00\u4e2a\u6df7\u54cd\uff0c\u80fd\u627e\u5230\u5417")
	if len(cmds) != 2 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["cmd"] != "plugin_semantic_search" || cmds[0]["type"] != "eq" {
		t.Fatalf("first cmd = %+v", cmds[0])
	}
	if cmds[1]["cmd"] != "plugin_semantic_search" || cmds[1]["type"] != "reverb" {
		t.Fatalf("second cmd = %+v", cmds[1])
	}
}
func TestPendingConfirmationStatusQuestionDoesNotClaimExecuted(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	goal := server.harness.BeginGoal("load TDR Nova")
	server.pending["plan_test"] = PendingPlan{
		ID:      "plan_test",
		Context: map[string]any{"goal_id": goal.GoalID},
		Preview: "Load a plugin as a graph rack node.",
		Decisions: []policy.Decision{{
			Name: "rack_add_node",
			Risk: policy.RiskConfirm,
		}},
	}
	plan, ok := server.pendingPlanForChat("chat_test", map[string]any{"goal_id": goal.GoalID})
	if !ok || plan.ID != "plan_test" {
		t.Fatalf("pending plan = %+v ok=%v", plan, ok)
	}
	if !isPendingConfirmationStatusQuestion("did you load it?") {
		t.Fatal("expected status question to be recognized")
	}
}
func TestPluginLibraryIntentOverridesModelScanCommand(t *testing.T) {
	env := modelEnvelope{
		Commands: []map[string]any{{"cmd": "scan_plugins"}},
	}
	if libraryCommands := synthesizePluginLibraryCommands("search TDR Nova plugin candidates only do not load"); len(libraryCommands) > 0 {
		env.Commands = libraryCommands
	}
	if len(env.Commands) != 1 || env.Commands[0]["cmd"] != "plugin_search" {
		t.Fatalf("commands = %+v", env.Commands)
	}
}
func TestAgentLoopConfirmResponsePlanIDUsesQueuedConfirmation(t *testing.T) {
	confirmedPlanID := "plan_confirmed"
	next := ChatResponse{NeedsConfirmation: true, PlanID: "plan_next"}
	if got := AgentLoopConfirmResponsePlanID(confirmedPlanID, next); got != "plan_next" {
		t.Fatalf("plan id = %q, want next queued confirmation", got)
	}

	done := ChatResponse{NeedsConfirmation: false, PlanID: "plan_done"}
	if got := AgentLoopConfirmResponsePlanID(confirmedPlanID, done); got != confirmedPlanID {
		t.Fatalf("completed plan id = %q, want confirmed plan id", got)
	}

	missing := ChatResponse{NeedsConfirmation: true, PlanID: " "}
	if got := AgentLoopConfirmResponsePlanID(confirmedPlanID, missing); got != confirmedPlanID {
		t.Fatalf("missing next plan id = %q, want confirmed plan id", got)
	}
}

func TestAgentPlanFromAgentLoopResultMirrorsAgentLoopState(t *testing.T) {
	res := agentloop.Result{
		GoalID:                "goal_1",
		RunID:                 "run_1",
		Status:                agentruntime.StatusWaitingClarification,
		GoalSummary:           "edit midi",
		NeedsClarification:    true,
		ClarificationQuestion: "choose clip",
		PlanItems: []planner.PlanItem{{
			ID:          "choose_clip",
			Description: "Clarify target clip",
			Status:      "pending",
		}},
		ProjectHistory: map[string]any{"active_branch": "main"},
	}
	plan := agentPlanFromAgentLoopResult(res)
	if plan == nil || plan.GoalID != "goal_1" || plan.Status != string(agentruntime.StatusWaitingClarification) || !plan.NeedsClarification {
		t.Fatalf("agent plan = %+v", plan)
	}
	if plan.CurrentStep != "Clarify target clip" || len(plan.Steps) != 1 || plan.Steps[0].ID != "choose_clip" {
		t.Fatalf("agent plan steps = %+v", plan)
	}
	if plan.ProjectHistory["active_branch"] != "main" {
		t.Fatalf("project history = %+v", plan.ProjectHistory)
	}
}
