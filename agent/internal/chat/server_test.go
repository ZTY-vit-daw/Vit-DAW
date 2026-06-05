package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	"vit-daw-agent/internal/workflows/plugingrabber"
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

func TestPluginGrabberUpsertCommandIncludesLegacyAndPluginSkill(t *testing.T) {
	target := pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Demo Comp"}
	digest := pluginParameterDigest{
		PluginID:   "plugin_1",
		PluginName: "Demo Comp",
		PluginIdentity: map[string]any{
			"profile_key": "plugin_demo",
			"plugin_name": "Demo Comp",
		},
		Parameters: []pluginParameterInfo{
			{ID: "threshold", Name: "Threshold", HostControllable: true},
			{ID: "ratio", Name: "Ratio", HostControllable: true},
		},
	}
	patch := pluginProfilePatch{
		QuickControlIDs: []string{"threshold", "ratio"},
		Aliases:         map[string]string{"threshold": "Threshold", "ratio": "Ratio"},
		DisplayGroups:   map[string]string{"threshold": "Dynamics", "ratio": "Dynamics"},
		NormalizedRoles: map[string]string{"threshold": "threshold", "ratio": "ratio"},
		Class:           "compressor",
		Groups: []map[string]any{{
			"id":    "main_dynamics",
			"role":  "compressor",
			"label": "Main dynamics",
			"params": map[string]any{
				"threshold": map[string]any{"param_id": "threshold", "label": "Threshold", "confidence": 0.9},
				"ratio":     "ratio",
			},
		}},
		VirtualControls: []map[string]any{{
			"name":         "tighten dynamics",
			"inputs":       []string{"amount"},
			"component_id": "main_dynamics",
			"resolver":     "local_profile_mapping",
			"params":       map[string]any{"threshold": "threshold", "ratio": "ratio"},
		}},
		Safety: map[string]any{"max_gain_change_db": 6},
	}

	cmd, validation, err := buildPluginProfileUpsertCommand(target, patch, digest)
	if err != nil {
		t.Fatalf("buildPluginProfileUpsertCommand: %v", err)
	}
	if cmd["cmd"] != "plugin_grabber_upsert_project_profile" || cmd["global"] != true {
		t.Fatalf("unexpected command envelope: %+v", cmd)
	}
	if quick, ok := cmd["quick_control_ids"].([]string); !ok || len(quick) != 2 {
		t.Fatalf("legacy quick_control_ids missing: %+v", cmd["quick_control_ids"])
	}
	if cmd["class"] != "compressor" || cmd["groups"] == nil || cmd["virtual_controls"] == nil || cmd["safety"] == nil {
		t.Fatalf("extended fields missing: %+v", cmd)
	}
	if cmd["schema_version"] != plugingrabber.PluginSkillSchemaVersion {
		t.Fatalf("schema_version = %v", cmd["schema_version"])
	}
	skill, ok := cmd["plugin_skill"].(plugingrabber.PluginSkillDocument)
	if !ok {
		t.Fatalf("plugin_skill missing or wrong type: %T", cmd["plugin_skill"])
	}
	if skill.SchemaVersion != plugingrabber.PluginSkillSchemaVersion || len(skill.Components) == 0 || len(skill.Operations) == 0 {
		t.Fatalf("plugin_skill incomplete: %+v", skill)
	}
	if skill.Legacy.ProjectDefault["quick_control_ids"] == nil {
		t.Fatalf("plugin_skill legacy project_default missing quick controls: %+v", skill.Legacy.ProjectDefault)
	}
	if validation.Coverage["threshold"] != "used_in_component" {
		t.Fatalf("unexpected validation coverage: %+v", validation.Coverage)
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

func TestPluginLearningPromptDigestCompactsLargeRuntimeProfile(t *testing.T) {
	params := make([]pluginParameterInfo, 0, 90)
	for i := 0; i < 90; i++ {
		params = append(params, pluginParameterInfo{
			ID:               fmt.Sprintf("param_%03d", i),
			Name:             fmt.Sprintf("Parameter %03d", i),
			RawName:          fmt.Sprintf("Raw Parameter %03d", i),
			DisplayGroup:     "Group",
			NormalizedRole:   "role",
			HostControllable: true,
			Value:            strings.Repeat("value_blob_", 40),
			Min:              -1000.0,
			Max:              1000.0,
		})
	}
	digest := pluginParameterDigest{
		TrackID:              "1007",
		PluginID:             "1013",
		PluginName:           "Large Plugin",
		GlobalProfileApplied: true,
		GlobalProfile: map[string]any{
			"class":                 "eq",
			"large_parameter_cache": strings.Repeat("big_parameter_snapshot", 12000),
			"plugin_skill": map[string]any{
				"schema_version": 2,
				"components": []any{
					map[string]any{"id": "main", "params": map[string]any{"gain": map[string]any{"param_id": "param_001"}}},
				},
			},
		},
		ParameterCount: len(params),
		Parameters:     params,
	}
	raw, err := json.MarshalIndent(digest, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent raw digest: %v", err)
	}
	if len(raw) <= pluginLearningDigestMaxBytes {
		t.Fatalf("raw digest should exceed learning prompt limit for regression coverage: %d", len(raw))
	}
	promptJSON, err := pluginLearningPromptDigestJSON(digest)
	if err != nil {
		t.Fatalf("pluginLearningPromptDigestJSON: %v", err)
	}
	if len(promptJSON) >= pluginLearningDigestMaxBytes {
		t.Fatalf("prompt digest should be compacted below limit: %d", len(promptJSON))
	}
	prompt := string(promptJSON)
	if strings.Contains(prompt, "big_parameter_snapshot") || strings.Contains(prompt, "value_blob_") {
		t.Fatalf("prompt digest leaked bulky runtime/value fields")
	}
	if !strings.Contains(prompt, "param_001") {
		t.Fatalf("prompt digest should retain parameter IDs: %s", prompt)
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

func TestPluginGrabberContextPackIncludesRuntimeProfile(t *testing.T) {
	digest := buildPluginParameterDigest(map[string]any{
		"track_id":               "1007",
		"plugin_id":              "plugin_a",
		"global_profile_applied": true,
		"global_profile_source":  "global_profile",
		"plugin_identity": map[string]any{
			"plugin_name": "Demo Comp",
		},
		"global_profile": map[string]any{
			"class": "compressor",
			"groups": []any{
				map[string]any{"id": "main_dynamics", "role": "compressor", "label": "Main dynamics"},
			},
			"virtual_controls": []any{
				map[string]any{"name": "tighten dynamics", "component_id": "main_dynamics", "resolver": "local_profile_mapping"},
			},
			"safety": map[string]any{"max_gain_change_db": 6},
			"plugin_skill": map[string]any{
				"schema_version": 2,
				"components": []any{
					map[string]any{
						"id":    "main_dynamics",
						"role":  "compressor",
						"label": "Main dynamics",
						"params": map[string]any{
							"threshold": map[string]any{"param_id": "threshold", "label": "Threshold", "confidence": 0.9},
						},
					},
				},
				"operations": []any{
					map[string]any{
						"name":         "tighten dynamics",
						"component_id": "main_dynamics",
						"resolver":     "local_profile_mapping",
						"params":       map[string]any{"threshold": "threshold"},
					},
				},
			},
		},
		"quick_controls": []any{
			map[string]any{"param_id": "threshold", "label": "Threshold", "display_group": "Dynamics", "normalized_role": "comp_threshold"},
		},
		"parameters": []any{
			map[string]any{"id": "threshold", "display_group": "Dynamics", "normalized_role": "comp_threshold", "host_controllable": true},
		},
	})
	if digest.PluginClass != "compressor" || len(digest.PluginGroups) != 1 || len(digest.PluginSkill) == 0 {
		t.Fatalf("digest runtime profile fields missing: %+v", digest)
	}
	pack := buildPluginGrabberContextPack(digest)
	runtimeProfile, ok := pack["runtime_profile"].(map[string]any)
	if !ok || runtimeProfile["available"] != true {
		t.Fatalf("runtime profile missing: %+v", pack["runtime_profile"])
	}
	if runtimeProfile["class"] != "compressor" || runtimeProfile["component_count"] != 1 || runtimeProfile["operation_count"] != 1 {
		t.Fatalf("runtime profile summary = %+v", runtimeProfile)
	}
	components := mapRowsValue(runtimeProfile["components"])
	if len(components) != 1 || len(mapRowsValue(components[0]["params"])) != 1 {
		t.Fatalf("runtime components missing param mappings: %+v", runtimeProfile["components"])
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

func TestPluginGrabberExplainInvokeAcceptsUnderscoreToolName(t *testing.T) {
	cmd, ok := pluginGrabberExplainInvokeCommand(harness.InvokeRequest{
		Tool: "plugin_grabber_explain_controls",
		Args: map[string]any{
			"track_id":  "1007",
			"plugin_id": "plugin_a",
		},
	})
	if !ok {
		t.Fatal("expected underscore tool name to route to explain workflow")
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

func TestTakePendingPlanClearsPluginGrabberLearningAliases(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := PendingPlan{
		ID:           "plan_learn",
		Workflow:     "plugin_grabber_teach",
		WorkflowData: map[string]any{"mode": "teach", "conversation_id": "chat_test"},
		Context:      map[string]any{"goal_id": "goal_1"},
	}
	server.pending[plan.ID] = plan
	server.pending["plugin_grabber_learning:goal_1:run_1:tool_1"] = plan

	got, ok, stale := server.takePendingPlan(plan.ID)
	if !ok || stale || got.ID != plan.ID {
		t.Fatalf("takePendingPlan = plan=%+v ok=%v stale=%v", got, ok, stale)
	}
	if len(server.pending) != 0 {
		t.Fatalf("pending aliases were not cleared: %+v", server.pending)
	}
	if pending, ok := server.pendingPlanForChat("chat_test", map[string]any{"goal_id": "goal_1"}); ok {
		t.Fatalf("stale pending still blocks chat: %+v", pending)
	}
}

func TestPendingPlanForChatDropsStalePluginGrabberLearningAlias(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := PendingPlan{
		ID:           "plan_learn",
		Workflow:     "plugin_grabber_auto_learn",
		WorkflowData: map[string]any{"mode": "auto_learn", "conversation_id": "chat_test"},
		Context:      map[string]any{"goal_id": "goal_1"},
	}
	server.pending["plugin_grabber_learning:goal_1:run_1:tool_1"] = plan

	if pending, ok := server.pendingPlanForChat("chat_test", map[string]any{"goal_id": "goal_1"}); ok {
		t.Fatalf("stale alias should not block chat: %+v", pending)
	}
	if len(server.pending) != 0 {
		t.Fatalf("stale alias was not removed: %+v", server.pending)
	}
}

func TestConfirmationInteractionRequestUsesGenericSchema(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp := ChatResponse{
		ConversationID:    "chat_1",
		Reply:             "将执行一项需要确认的操作。",
		NeedsConfirmation: true,
		PlanID:            "plan_1",
		Preview:           "preview",
		Workflow:          "test_workflow",
		Commands: []policy.Decision{{
			Name: "test_command",
			Risk: policy.RiskConfirm,
		}},
	}
	server.attachInteractionRequests(&resp)
	if len(resp.InteractionRequests) != 1 {
		t.Fatalf("interaction_requests = %+v", resp.InteractionRequests)
	}
	req := resp.InteractionRequests[0]
	if req.Kind != "confirmation" || req.Source != "vit_agent" || req.Payload["plan_id"] != "plan_1" {
		t.Fatalf("request = %+v", req)
	}
	if req.Type != "confirmation" || req.Data["plan_id"] != "plan_1" {
		t.Fatalf("legacy compatibility fields missing: %+v", req)
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{`"kind":"confirmation"`, `"source":"vit_agent"`, `"payload":`} {
		if !strings.Contains(text, want) {
			t.Fatalf("serialized request missing %s: %s", want, text)
		}
	}
}

func TestInteractionRespondStaleReturnsChineseChatResponse(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	body := bytes.NewBufferString(`{"interaction_id":"missing"}`)
	req := httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", body)
	rec := httptest.NewRecorder()
	server.handleInteractionRespond(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Reply, "已处理或已过期") || resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("response = %+v", resp)
	}
}

func TestPluginLearningCandidateInteractionUsesReviewKind(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp := ChatResponse{
		ConversationID: "chat_learn",
		Reply:          "已生成插件抓手候选，请先测试并确认显示域。",
		Workflow:       "plugin_grabber_auto_learn",
		PluginLearning: map[string]any{
			"mode":              "auto_learn",
			"stage":             "candidate_review",
			"needs_user_review": true,
			"plugin_name":       "TDR Nova",
			"parameter_count":   12,
			"component_count":   4,
			"operation_count":   5,
			"profile_patch":     map[string]any{"class": "eq"},
			"experiments":       []map[string]any{{"param_id": "B1 Gain"}},
		},
	}
	server.attachInteractionRequests(&resp)
	if len(resp.InteractionRequests) != 1 {
		t.Fatalf("interaction_requests = %+v", resp.InteractionRequests)
	}
	req := resp.InteractionRequests[0]
	if req.Kind != "review" || req.Source != "plugin_grabber" || req.Workflow != "plugin_grabber_auto_learn" {
		t.Fatalf("request = %+v", req)
	}
	if len(req.ReviewItems) == 0 {
		t.Fatalf("expected review items: %+v", req)
	}
	if req.Payload["profile_patch"] == nil || req.Data["profile_patch"] == nil {
		t.Fatalf("payload/data missing profile patch: %+v", req)
	}
	if len(req.Actions) == 0 || req.Actions[0].ID != "submit" || !strings.Contains(req.Actions[0].Label, "候选草图") {
		t.Fatalf("expected candidate confirmation action: %+v", req.Actions)
	}
}

func TestBuildPluginLearningExperimentsRecordsRestoreValues(t *testing.T) {
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{ID: "gain", Name: "Gain", HostControllable: true, NormalizedValue: 0.25, ValueText: "-6 dB"},
			{ID: "bypass", Name: "Bypass", HostControllable: true, NormalizedValue: 1.0, IsBoolean: true, NormalizedRole: "common_bypass"},
			{ID: "meter", Name: "Meter", HostControllable: false, NormalizedValue: 0.5},
		},
	}
	patch := pluginProfilePatch{
		VirtualControls: []map[string]any{{
			"name":         "adjust gain",
			"component_id": "main",
			"params": map[string]any{
				"gain":   map[string]any{"param_id": "gain", "label": "Gain"},
				"bypass": map[string]any{"param_id": "bypass", "label": "Bypass"},
				"meter":  map[string]any{"param_id": "meter", "label": "Meter"},
			},
		}},
	}
	experiments := buildPluginLearningExperiments(digest, patch, 3)
	if len(experiments) != 2 {
		t.Fatalf("experiments = %+v", experiments)
	}
	byParam := map[string]map[string]any{}
	for _, experiment := range experiments {
		byParam[strings.TrimSpace(fmt.Sprint(experiment["param_id"]))] = experiment
	}
	gainExperiment := byParam["gain"]
	if gainExperiment == nil || gainExperiment["before_normalized"] != 0.25 {
		t.Fatalf("gain experiment = %+v", gainExperiment)
	}
	if float64(gainExperiment["after_normalized"].(float64)) <= 0.25 {
		t.Fatalf("gain experiment did not move up: %+v", gainExperiment)
	}
	bypassExperiment := byParam["bypass"]
	if bypassExperiment == nil || bypassExperiment["after_normalized"] != float64(0) {
		t.Fatalf("bypass experiment = %+v", bypassExperiment)
	}
}

func TestBuildPluginLearningExperimentsSamplesGroupParamsWithoutVirtualControlParams(t *testing.T) {
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{ID: "b1_freq", Name: "B1 Frequency", HostControllable: true, NormalizedValue: 0.40, NormalizedRole: "eq_frequency", ValueText: "500 Hz"},
			{ID: "b1_gain", Name: "B1 Gain", HostControllable: true, NormalizedValue: 0.50, NormalizedRole: "eq_gain", ValueText: "0.0 dB"},
			{ID: "b1_q", Name: "B1 Q", HostControllable: true, NormalizedValue: 0.35, NormalizedRole: "eq_q", ValueText: "1.00"},
		},
	}
	patch := pluginProfilePatch{
		Groups: []map[string]any{{
			"id":    "b1",
			"label": "B1",
			"role":  "eq_band",
			"params": map[string]any{
				"frequency": map[string]any{
					"param_id": "b1_freq",
					"label":    "B1 Frequency",
					"display_domain": map[string]any{
						"status":     "inferred",
						"confidence": 0.78,
						"unit":       "Hz",
					},
				},
				"gain": map[string]any{
					"param_id": "b1_gain",
					"label":    "B1 Gain",
					"display_domain": map[string]any{
						"status":     "inferred",
						"confidence": 0.66,
						"unit":       "dB",
					},
				},
				"q": map[string]any{
					"param_id": "b1_q",
					"label":    "B1 Q",
					"display_domain": map[string]any{
						"status":     "inferred",
						"confidence": 0.70,
					},
				},
			},
		}},
		VirtualControls: []map[string]any{{
			"name":     "eq.cut_region",
			"resolver": "choose_nearest_or_free_band",
		}},
	}
	experiments := buildPluginLearningExperiments(digest, patch, 3)
	if len(experiments) != 3 {
		t.Fatalf("experiments = %+v", experiments)
	}
	seen := map[string]bool{}
	for _, experiment := range experiments {
		if experiment["component_id"] != "b1" {
			t.Fatalf("component_id = %+v", experiment)
		}
		if experiment["sample_reason"] != "low_confidence_high_impact" {
			t.Fatalf("sample_reason = %+v", experiment)
		}
		seen[strings.TrimSpace(fmt.Sprint(experiment["param_id"]))] = true
	}
	for _, paramID := range []string{"b1_freq", "b1_gain", "b1_q"} {
		if !seen[paramID] {
			t.Fatalf("missing %s experiment: %+v", paramID, experiments)
		}
	}
}

func TestPluginLearningExperimentInfersDisplayDomainDefaults(t *testing.T) {
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{ID: "delay", Name: "Delay_Ms", HostControllable: true, NormalizedValue: 0.5, NormalizedRole: "time_delay", ValueText: "300.0 ms"},
		},
	}
	patch := pluginProfilePatch{
		VirtualControls: []map[string]any{{
			"name":         "Echo Length",
			"component_id": "delay",
			"params": map[string]any{
				"time": map[string]any{"param_id": "delay", "label": "Delay ms"},
			},
		}},
	}
	experiments := buildPluginLearningExperiments(digest, patch, 1)
	if len(experiments) != 1 {
		t.Fatalf("experiments = %+v", experiments)
	}
	if experiments[0]["inferred_display_domain_text"] != "0~2000 ms" {
		t.Fatalf("inferred display domain = %+v", experiments[0])
	}
	fields := pluginGrabberDisplayDomainFields(map[string]any{
		"experiment_answers": []map[string]any{{
			"experiment": experiments[0],
		}},
	})
	if len(fields) != 1 || fields[0].Value != "0~2000 ms" {
		t.Fatalf("fields = %+v", fields)
	}
}

func TestPluginLearningExperimentFieldsExposeDisplayDomainOptions(t *testing.T) {
	fields := pluginGrabberExperimentFields(map[string]any{
		"inferred_display_domain_text": "0~100 %",
		"slot":                         "feedback",
		"label":                        "Feedback",
	})
	if len(fields) != 3 {
		t.Fatalf("fields = %+v", fields)
	}
	if fields[0].ID != "display_domain_text" || fields[0].Kind != "choice" || fields[0].Value != "0~100 %" || len(fields[0].Options) < 4 {
		t.Fatalf("display domain choice field = %+v", fields[0])
	}
	if fields[1].ID != "observation" || fields[1].Kind != "text" {
		t.Fatalf("notes field = %+v", fields[1])
	}
	if fields[2].ID != "custom_display_domain_text" || fields[2].Kind != "text" {
		t.Fatalf("custom display domain field = %+v", fields[2])
	}
	summary := pluginGrabberExperimentObservationSummary("sound_changed", "尾音更长")
	if summary != "声音变化；尾音更长" {
		t.Fatalf("summary = %q", summary)
	}
}

func TestPluginProfilePatchDisplayDomainEnrichmentUsesDigestCandidates(t *testing.T) {
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{
				ID:               "dry level",
				Name:             "Dry Level",
				HostControllable: true,
				NormalizedRole:   "common_mix",
				DisplayDomainCandidate: &plugingrabber.PluginDisplayDomain{
					Text:       "-12~0 dB",
					Unit:       "dB",
					Scale:      "linear",
					Status:     "inferred",
					Source:     "display_probe_inferred",
					Confidence: 0.90,
				},
			},
			{ID: "6", Name: "Feedback", HostControllable: true, NormalizedRole: "mod_feedback"},
		},
	}
	patch := pluginProfilePatch{
		Groups: []map[string]any{{
			"id": "mix",
			"params": map[string]any{
				"dry_level": map[string]any{"param_id": "dry level", "label": "Dry Level"},
			},
		}},
		VirtualControls: []map[string]any{{
			"name": "Set Feedback",
			"params": map[string]any{
				"feedback": "6",
			},
		}},
	}
	patch = enrichPluginProfilePatchDisplayDomains(patch, digest)
	groupParams := mapValue(patch.Groups[0]["params"])
	dry := mapValue(groupParams["dry_level"])
	if dry["display_domain_text"] != "-12~0 dB" {
		t.Fatalf("dry mapping = %+v", dry)
	}
	controlParams := mapValue(patch.VirtualControls[0]["params"])
	feedback := mapValue(controlParams["feedback"])
	if feedback["param_id"] != "6" || feedback["display_domain_text"] != "0~100 %" {
		t.Fatalf("feedback mapping = %+v", feedback)
	}
	if len(mapRowsValue(feedback["provenance"])) == 0 {
		t.Fatalf("feedback provenance missing: %+v", feedback)
	}
}

func TestPluginLearningExperimentsSpotCheckHighConfidenceDisplayProbe(t *testing.T) {
	minValue, maxValue := 0.0, 2000.0
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{
				ID:               "delay",
				Name:             "Delay",
				HostControllable: true,
				NormalizedValue:  0.5,
				NormalizedRole:   "time_delay",
				DisplayDomainCandidate: &plugingrabber.PluginDisplayDomain{
					Text:       "0~2000 ms",
					Unit:       "ms",
					Min:        &minValue,
					Max:        &maxValue,
					Scale:      "linear",
					Status:     "inferred",
					Source:     "display_probe_inferred",
					Confidence: 0.90,
				},
			},
		},
	}
	patch := pluginProfilePatch{
		VirtualControls: []map[string]any{{
			"name":         "Echo Length",
			"component_id": "delay",
			"params": map[string]any{
				"time": map[string]any{"param_id": "delay", "label": "Delay"},
			},
		}},
	}
	experiments := buildPluginLearningExperiments(digest, patch, 1)
	if len(experiments) != 1 {
		t.Fatalf("experiments = %+v", experiments)
	}
	if experiments[0]["sample_reason"] != "spot_check_high_confidence" || experiments[0]["param_id"] != "delay" {
		t.Fatalf("experiment = %+v", experiments[0])
	}
}

func TestPluginGrabberSubmittedReviewsCarryIntoExperimentPayload(t *testing.T) {
	base := map[string]any{
		"profile_patch": pluginProfilePatch{
			Groups: []map[string]any{{
				"id":    "delay",
				"label": "Delay",
				"params": map[string]any{
					"time": map[string]any{
						"param_id": "delay",
						"label":    "Delay",
						"display_domain": map[string]any{
							"status":     "needs_confirmation",
							"confidence": 0.45,
						},
					},
				},
			}},
		},
		"experiments": []map[string]any{{
			"component_id": "delay",
			"slot":         "time",
			"param_id":     "delay",
		}},
	}
	submitted := map[string]any{
		"display_domain_reviews": []map[string]any{{
			"component_id":        "delay",
			"slot":                "time",
			"param_id":            "delay",
			"display_domain_text": "0~2000 ms",
			"provenance_kind":     "user_review",
			"observation":         "Confirmed from the review card.",
		}},
	}
	payload := pluginGrabberPayloadWithSubmittedReviews(base, submitted)
	if experiments := mapRowsValue(payload["experiments"]); len(experiments) != 1 {
		t.Fatalf("experiments = %+v", experiments)
	}
	patch := mapFromJSONStruct(payload["profile_patch"])
	groups := mapRowsValue(patch["groups"])
	if len(groups) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	params := mapValue(groups[0]["params"])
	mapping := mapValue(params["time"])
	if !boolValue(mapping["confirmed"]) {
		t.Fatalf("mapping not confirmed: %+v", mapping)
	}
	domain := mapValue(mapping["display_domain"])
	if domain["unit"] != "ms" || fmt.Sprint(domain["min"]) != "0" || fmt.Sprint(domain["max"]) != "2000" {
		t.Fatalf("domain = %+v", domain)
	}
	if len(mapRowsValue(mapping["provenance"])) == 0 {
		t.Fatalf("missing provenance: %+v", mapping)
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
