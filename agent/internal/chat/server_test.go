package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/artifacts"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/mom"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/policy"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/workflows/plugingrabber"
)

type recordingChatKernel struct {
	replies        []map[string]any
	replyByCommand map[string][]map[string]any
	commands       []map[string]any
}

func (k *recordingChatKernel) SendCommand(_ context.Context, cmd map[string]any) (map[string]any, string, error) {
	k.commands = append(k.commands, cloneTestCommand(cmd))
	reply := map[string]any{"status": "ok"}
	if commandName := cleanContextText(cmd["cmd"]); commandName != "" && len(k.replyByCommand[commandName]) > 0 {
		reply = k.replyByCommand[commandName][0]
		k.replyByCommand[commandName] = k.replyByCommand[commandName][1:]
	} else if len(k.replies) > 0 {
		reply = k.replies[0]
		k.replies = k.replies[1:]
	}
	if errText := strings.TrimSpace(fmt.Sprint(reply["error"])); errText != "" && errText != "<nil>" {
		return reply, "", fmt.Errorf("%s", errText)
	}
	data, _ := json.Marshal(reply)
	return reply, string(data), nil
}

func cloneTestCommand(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	data, err := json.Marshal(in)
	if err != nil {
		out := make(map[string]any, len(in))
		for key, value := range in {
			out[key] = value
		}
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil || out == nil {
		out = make(map[string]any, len(in))
		for key, value := range in {
			out[key] = value
		}
	}
	return out
}

func TestMain(m *testing.M) {
	_ = os.Setenv("VIT_CONTEXT_SNAPSHOT_PATH", "off")
	_ = os.Setenv("VIT_AGENT_JOURNAL_PATH", "off")
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

func TestCompactAgentStateProjectHistoryOmitsLargeConversationPayloads(t *testing.T) {
	history := map[string]any{
		"available":             true,
		"project_path":          `D:\\songs\\mix.vit`,
		"project_uuid":          "vitproj_test",
		"active_branch":         "main",
		"active_node_id":        "node_42",
		"head":                  "commit_42",
		"commit_count":          42,
		"branches":              map[string]any{"main": "commit_42"},
		"recent_checkpoints":    []map[string]any{{"id": "commit_42"}},
		"conversation_graph":    map[string]any{"nodes": []map[string]any{{"message_data": strings.Repeat("x", 17<<20)}}},
		"conversation_messages": []map[string]any{{"content": strings.Repeat("y", 17<<20)}},
	}

	got := compactAgentStateProjectHistory(history)
	for _, key := range []string{"available", "project_path", "project_uuid", "active_branch", "active_node_id", "head", "commit_count", "branches", "recent_checkpoints"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("compact history omitted status field %q: %+v", key, got)
		}
	}
	for _, key := range []string{"conversation_graph", "conversation_messages"} {
		if _, ok := got[key]; ok {
			t.Fatalf("compact history retained large field %q", key)
		}
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal compact history: %v", err)
	}
	if len(encoded) >= 1<<20 {
		t.Fatalf("compact history is unexpectedly large: %d bytes", len(encoded))
	}
}

func TestCompactHostLifecycleInvokeResponseOmitsLargeHistoryGraphs(t *testing.T) {
	largeGraph := map[string]any{"nodes": []map[string]any{{"message_data": strings.Repeat("x", 17<<20)}}}
	resp := compactHostLifecycleInvokeResponseForTransport(harness.InvokeResponse{
		Status: "ok",
		Tool:   "version.project_opened",
		Result: map[string]any{
			"status":                    "ok",
			"project_path":              `D:\\songs\\mix.vit`,
			"project_uuid":              "vitproj_test",
			"project_package_committed": true,
			"project_package_path":      `D:\\songs\\mix.vit_project`,
			"conversation_graph":        largeGraph,
			"conversation_messages":     []map[string]any{{"content": strings.Repeat("y", 17<<20)}},
			"refresh":                   map[string]any{"shadow_refreshed": true},
		},
		ProjectHistory: map[string]any{
			"available":          true,
			"project_path":       `D:\\songs\\mix.vit`,
			"active_branch":      "main",
			"conversation_graph": largeGraph,
		},
	})
	if _, ok := resp.Result["conversation_graph"]; ok {
		t.Fatal("lifecycle result retained conversation graph")
	}
	if _, ok := resp.Result["conversation_messages"]; ok {
		t.Fatal("lifecycle result retained conversation messages")
	}
	if _, ok := resp.ProjectHistory["conversation_graph"]; ok {
		t.Fatal("lifecycle project history retained conversation graph")
	}
	if !boolValue(resp.Result["project_package_committed"]) || firstStringFromMap(resp.Result, "project_package_path") == "" {
		t.Fatalf("lifecycle response omitted committed package identity: %+v", resp.Result)
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= 1<<20 {
		t.Fatalf("lifecycle response is unexpectedly large: %d bytes", len(encoded))
	}
}

func TestCompactProjectSavePrepareInvokeResponsePreservesIdentityAndOmitsLargeHistory(t *testing.T) {
	largeGraph := map[string]any{"nodes": []map[string]any{{"message_data": strings.Repeat("x", 17<<20)}}}
	resp := compactProjectSavePrepareInvokeResponseForTransport(harness.InvokeResponse{
		Status: "ok",
		Tool:   "version.project_save_prepare",
		Result: map[string]any{
			"status":                       "ok",
			"prepared":                     true,
			"prepare_id":                   "prepare_test",
			"generation_id":                "save_test",
			"agent_history_generation":     "save_test",
			"project_path":                 `D:\\songs\\mix.vit`,
			"project_uuid":                 "vitproj_test",
			"save_kind":                    "save_as",
			"project_package_prepared":     true,
			"project_package_prepare_path": strings.Repeat("p", 17<<20),
			"conversation_graph":           largeGraph,
			"conversation_messages":        []map[string]any{{"content": strings.Repeat("y", 17<<20)}},
		},
		ProjectHistory: map[string]any{
			"available":          true,
			"conversation_graph": largeGraph,
		},
	})
	for _, key := range []string{
		"status", "prepared", "prepare_id", "generation_id", "agent_history_generation",
		"project_path", "project_uuid", "save_kind", "project_package_prepared",
	} {
		if _, ok := resp.Result[key]; !ok {
			t.Fatalf("save prepare response omitted required field %q: %+v", key, resp.Result)
		}
	}
	for _, key := range []string{"project_package_prepare_path", "conversation_graph", "conversation_messages"} {
		if _, ok := resp.Result[key]; ok {
			t.Fatalf("save prepare response retained transport-only large field %q", key)
		}
	}
	if len(resp.ProjectHistory) != 0 {
		t.Fatalf("save prepare response retained project history: %+v", resp.ProjectHistory)
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= 1<<20 {
		t.Fatalf("save prepare response is unexpectedly large: %d bytes", len(encoded))
	}
}

func TestCompactSaveAsFolderInvokeResponsePreservesMediaPreflight(t *testing.T) {
	largeGraph := map[string]any{"nodes": []map[string]any{{"message_data": strings.Repeat("x", 17<<20)}}}
	resp := compactSaveAsFolderInvokeResponseForTransport(harness.InvokeResponse{
		Status: "ok",
		Tool:   "project.save_as_folder",
		Result: map[string]any{
			"status":                 "require_media_policy",
			"directory_path":         `D:\\songs\\Portable`,
			"project_path":           `D:\\songs\\Portable\\mix.vit`,
			"referenced_audio_count": 12,
			"external_audio_count":   10,
			"missing_audio_count":    2,
			"estimated_copy_bytes":   987654321,
			"conversation_graph":     largeGraph,
		},
		ProjectHistory: map[string]any{"available": true, "conversation_graph": largeGraph},
	})
	for _, key := range []string{
		"status", "directory_path", "project_path", "referenced_audio_count",
		"external_audio_count", "missing_audio_count", "estimated_copy_bytes",
	} {
		if _, ok := resp.Result[key]; !ok {
			t.Fatalf("folder preflight response omitted required field %q: %+v", key, resp.Result)
		}
	}
	if _, ok := resp.Result["conversation_graph"]; ok {
		t.Fatal("folder preflight retained conversation graph")
	}
	if resp.ProjectHistory != nil {
		t.Fatal("folder preflight retained project history")
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= 1<<20 {
		t.Fatalf("folder preflight response is unexpectedly large: %d bytes", len(encoded))
	}
}

func TestCompactSaveAsFolderInvokeResponsePreservesPublishedSnapshotIdentity(t *testing.T) {
	resp := compactSaveAsFolderInvokeResponseForTransport(harness.InvokeResponse{
		Status: "ok",
		Tool:   "project.save_as_folder",
		Result: map[string]any{
			"status":                   "ok",
			"project_lifecycle":        "save_as_folder",
			"project_path":             `D:\\songs\\Portable\\mix.vit`,
			"project_uuid":             "vitproj_target",
			"source_project_path":      `D:\\songs\\mix.vit`,
			"source_project_uuid":      "vitproj_source",
			"directory_path":           `D:\\songs\\Portable`,
			"media_policy":             "copy_referenced_audio",
			"audio_self_contained":     true,
			"package_status":           "complete",
			"package_manifest_path":    `D:\\songs\\Portable\\package_manifest.v2.json`,
			"active_project_unchanged": true,
			"project_workspace":        map[string]any{"conversation_graph": strings.Repeat("x", 17<<20)},
		},
	})
	for _, key := range []string{
		"status", "project_lifecycle", "project_path", "project_uuid", "source_project_path",
		"source_project_uuid", "directory_path", "media_policy", "audio_self_contained",
		"package_status", "package_manifest_path", "active_project_unchanged",
	} {
		if _, ok := resp.Result[key]; !ok {
			t.Fatalf("folder snapshot response omitted required field %q: %+v", key, resp.Result)
		}
	}
	if _, ok := resp.Result["project_workspace"]; ok {
		t.Fatal("folder snapshot retained large internal workspace")
	}
}

func TestWriteJSONSetsContentLengthForBodyLargerThanGodotChunkLimit(t *testing.T) {
	payload := map[string]any{"status": "ok", "data": strings.Repeat("x", (16<<20)+1)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, payload)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ContentLength != int64(len(body)) || resp.Header.Get("Content-Length") != strconv.Itoa(len(body)) {
		t.Fatalf("content length header=%q parsed=%d body=%d", resp.Header.Get("Content-Length"), resp.ContentLength, len(body))
	}
	if len(resp.TransferEncoding) != 0 {
		t.Fatalf("unexpected chunked transfer encoding: %+v", resp.TransferEncoding)
	}
}

func TestIsVersionProjectSavePrepareInvokeAcceptsToolAndCommandAliases(t *testing.T) {
	for _, req := range []harness.InvokeRequest{
		{Tool: "version.project_save_prepare"},
		{Command: map[string]any{"cmd": "version_project_save_prepare"}},
		{Args: map[string]any{"tool": "version.project_save_prepare"}},
	} {
		if !isVersionProjectSavePrepareInvoke(req) {
			t.Fatalf("save prepare request was not recognized: %+v", req)
		}
	}
	if isVersionProjectSavePrepareInvoke(harness.InvokeRequest{Tool: "version.project_saved"}) {
		t.Fatal("project saved notification was misclassified as save prepare")
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

func TestChatToolConfirmationCreatesInteractionRequest(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp, handled := server.handleDevChatCommand(context.Background(), "chat_test", ChatRequest{
		Message: `/tool midi.create_clip {"track_id":"1007"}`,
		Context: map[string]any{
			"goal_id": "goal_tool",
			"run_id":  "run_tool",
		},
	})
	if !handled || !resp.NeedsConfirmation || resp.PlanID == "" {
		t.Fatalf("resp = %+v handled=%v", resp, handled)
	}
	if strings.Contains(resp.Reply, "Run again with `/tool! ") {
		t.Fatalf("reply used legacy text confirmation: %s", resp.Reply)
	}
	server.mu.Lock()
	plan, ok := server.pending[resp.PlanID]
	server.mu.Unlock()
	if !ok || len(plan.Decisions) != 1 || plan.Decisions[0].Name != "create_midi_clip" {
		t.Fatalf("pending plan = %+v ok=%v", plan, ok)
	}
	server.attachInteractionRequests(&resp)
	if len(resp.InteractionRequests) != 1 {
		t.Fatalf("interaction_requests = %+v", resp.InteractionRequests)
	}
	req := resp.InteractionRequests[0]
	if req.Kind != "confirmation" || req.PlanID != resp.PlanID || len(req.Actions) < 2 {
		t.Fatalf("confirmation interaction = %+v", req)
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

func TestAgentLoopCompletedWithoutExecutionForQuestionStaysCompleted(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp := server.chatResponseFromAgentLoopResult("chat_test", agentModeDefault, agentloop.Result{
		GoalID:      "goal_1",
		RunID:       "run_1",
		Status:      agentruntime.StatusCompleted,
		GoalSummary: "MIDI clip 是什么？",
		Reply:       "MIDI clip 是一段 MIDI 数据。",
	})
	if resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("goal status = %q reply=%q", resp.GoalStatus, resp.Reply)
	}
}

func TestAgentLoopPendingMixTreatmentResponseNeedsConfirmation(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp := server.chatResponseFromAgentLoopResult("chat_mix", agentModeDefault, agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		Status: agentruntime.StatusCompleted,
		Reply:  "我建议先把 1007 的电平调整 -1.50 dB。\n\n如果你认可这个混音建议，需要我继续执行吗？",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &agentloop.MixTreatmentPending{
				SchemaVersion: "mix_treatment_pending.v0",
				Status:        "pending_confirmation",
				TargetRef:     "track:1007",
				ActionKind:    "gain_balance",
				ProcessorType: "utility",
				DeltaDB:       -1.5,
				Confidence:    "high",
			},
		},
	})

	if !resp.NeedsConfirmation || resp.GoalStatus != string(agentruntime.StatusWaitingConfirmation) || resp.Workflow != "mix_treatment" {
		t.Fatalf("response confirmation fields = needs=%v status=%q workflow=%q", resp.NeedsConfirmation, resp.GoalStatus, resp.Workflow)
	}
	if len(resp.InteractionRequests) != 1 {
		t.Fatalf("interaction requests = %+v", resp.InteractionRequests)
	}
	req := resp.InteractionRequests[0]
	if req.Kind != "mix_treatment_confirmation" || len(req.Actions) < 2 {
		t.Fatalf("interaction request = %+v", req)
	}
	if fmt.Sprint(req.Payload["delta_db"]) != "-1.5" || fmt.Sprint(resp.WorkflowData["action_kind"]) != "gain_balance" {
		t.Fatalf("payload=%+v workflow_data=%+v", req.Payload, resp.WorkflowData)
	}
}

func TestInteractionRespondMixTreatmentConfirmationExecutesPendingTreatment(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "mix.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"volume_db":      -3.0,
		}},
	})
	kernel := &recordingChatKernel{replies: []map[string]any{
		{"status": "ok", "project_path": projectPath, "snapshot_xml": "<project/>"},
		{"status": "ok", "track_id": "1007", "volume_db": -4.5},
		{"status": "ok", "project_path": projectPath, "tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"volume_db":      -4.5,
		}}},
	}}
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	server.harness.EnsureGoal("goal_1", "run_1", "bring vocal down slightly")
	initial := server.chatResponseFromAgentLoopResult("chat_mix", agentModeDefault, agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		Status: agentruntime.StatusCompleted,
		Reply:  "我建议先把 1007 的电平调整 -1.50 dB。\n\n如果你认可这个混音建议，需要我继续执行吗？",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &agentloop.MixTreatmentPending{
				SchemaVersion:             "mix_treatment_pending.v0",
				Status:                    "pending_confirmation",
				ConversationID:            "chat_mix",
				ObservationID:             "obs_1",
				Intent:                    "bring vocal down slightly",
				TargetRef:                 "track:1007",
				ActionKind:                "gain_balance",
				ProcessorType:             "utility",
				DeltaDB:                   -1.5,
				ReasoningSummary:          "explicit small gain move",
				Confidence:                "high",
				ExpiresAfterContextChange: true,
				Fingerprint: map[string]any{
					"target_track_id": "1007",
					"track_count":     1,
					"track_gain_db":   -3.0,
					"before_track":    map[string]any{"track_id": "1007", "volume_db": -3.0, "peak_dbfs": -6.0, "rms_dbfs": -15.0, "headroom_db": 6.0},
				},
			},
		},
	})

	if len(initial.InteractionRequests) != 1 {
		t.Fatalf("interaction requests = %+v", initial.InteractionRequests)
	}
	interactionID := initial.InteractionRequests[0].ID
	if _, ok := server.takePendingInteraction(interactionID); !ok {
		t.Fatal("mix treatment confirmation interaction was not stored")
	}
	server.storePendingInteraction(initial.InteractionRequests[0], initial.InteractionRequests[0].Payload)
	body, _ := json.Marshal(InteractionRespondRequest{
		InteractionID: interactionID,
		ActionID:      "approve",
		Decision:      "approve",
	})
	rec := httptest.NewRecorder()
	server.handleInteractionRespond(rec, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if strings.Contains(resp.Reply, "已处理或已过期") || resp.StopReason != "mix_tick_applied_reobserved" {
		t.Fatalf("resp = %+v", resp)
	}
	var tools []string
	for _, row := range resp.ExecutedKernelReply {
		tools = append(tools, cleanContextText(row["tool"]))
	}
	if !testStringSliceContains(tools, "mix.propose_tick") || !testStringSliceContains(tools, "mix.apply_tick") || !testStringSliceContains(tools, "mix.observe") {
		t.Fatalf("executed tools = %+v", tools)
	}
	if _, ok := server.pendingMixTreatmentForConversation("chat_mix"); ok {
		t.Fatal("pending treatment should be consumed after approval")
	}
	var setVolumeCommands []map[string]any
	for _, cmd := range kernel.commands {
		if cleanContextText(cmd["cmd"]) == "set_volume" {
			setVolumeCommands = append(setVolumeCommands, cmd)
		}
	}
	if len(setVolumeCommands) != 1 || cleanContextText(setVolumeCommands[0]["track_id"]) != "1007" || fmt.Sprint(setVolumeCommands[0]["db"]) != "-4.5" {
		t.Fatalf("set_volume commands = %+v all=%+v", setVolumeCommands, kernel.commands)
	}
	if goal := server.harness.RuntimeStatus("goal_1"); goal.Status != agentruntime.StatusCompleted || goal.RunID != "run_1" || goal.Summary != "bring vocal down slightly" {
		t.Fatalf("goal not finalized on original runtime entry: %+v", goal)
	}
	if current := server.harness.RuntimeStatus(""); current.GoalID != "goal_1" {
		t.Fatalf("interaction response should not create a fresh current goal: %+v", current)
	}
	if !testAgentEventsContainTurnCompleted(server, "chat_mix", "goal_1", agentruntime.StatusCompleted) {
		t.Fatalf("missing final turn.completed event: %+v", mustAgentEvents(server, "chat_mix"))
	}
}

func TestInteractionRespondMixTickConfirmationFinalizesRuntimeAndEvents(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "mix.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{map[string]any{
			"track_id":       "1010",
			"track_name":     "Band",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"volume_db":      0.0,
		}},
	})
	kernel := &recordingChatKernel{replies: []map[string]any{
		{"status": "ok", "project_path": projectPath, "snapshot_xml": "<project/>"},
		{"status": "ok", "track_id": "1010", "volume_db": -1.0},
		{"status": "ok", "project_path": projectPath, "tracks": []any{map[string]any{
			"track_id":       "1010",
			"track_name":     "Band",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"volume_db":      -1.0,
		}}},
	}}
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	server.harness.EnsureGoal("goal_1", "run_1", "bring vocal forward")
	candidate := agentloop.PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       "1010",
		DeltaDB:       -1,
		ObservationID: "obs_before",
		Status:        "pending_confirmation",
	}
	server.storePendingMixTickCandidate("chat_mix", "goal_1", "run_1", candidate)
	interaction := mixTickInteractionRequest("chat_mix", "goal_1", "run_1", candidate)
	server.storePendingInteraction(interaction, interaction.Payload)

	body, _ := json.Marshal(InteractionRespondRequest{
		InteractionID: interaction.ID,
		ActionID:      "approve",
		Decision:      "approve",
	})
	rec := httptest.NewRecorder()
	server.handleInteractionRespond(rec, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if resp.StopReason != "mix_tick_applied_reobserved" || resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("resp = %+v", resp)
	}
	if goal := server.harness.RuntimeStatus("goal_1"); goal.Status != agentruntime.StatusCompleted || goal.RunID != "run_1" || goal.Summary != "bring vocal forward" {
		t.Fatalf("goal not finalized on original runtime entry: %+v", goal)
	}
	if current := server.harness.RuntimeStatus(""); current.GoalID != "goal_1" {
		t.Fatalf("interaction response should not create a fresh current goal: %+v", current)
	}
	if !testAgentEventsContainTurnCompleted(server, "chat_mix", "goal_1", agentruntime.StatusCompleted) {
		t.Fatalf("missing final turn.completed event: %+v", mustAgentEvents(server, "chat_mix"))
	}
}

func TestInteractionRespondMixTickCancelFinalizesAndExpiresPending(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.harness.EnsureGoal("goal_1", "run_1", "bring vocal forward")
	candidate := agentloop.PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       "1010",
		DeltaDB:       -1,
		ObservationID: "obs_before",
		Status:        "pending_confirmation",
	}
	server.storePendingMixTickCandidate("chat_mix", "goal_1", "run_1", candidate)
	interaction := mixTickInteractionRequest("chat_mix", "goal_1", "run_1", candidate)
	server.storePendingInteraction(interaction, interaction.Payload)

	body, _ := json.Marshal(InteractionRespondRequest{
		InteractionID: interaction.ID,
		ActionID:      "cancel",
		Decision:      "cancel",
	})
	rec := httptest.NewRecorder()
	server.handleInteractionRespond(rec, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if resp.GoalStatus != string(agentruntime.StatusCancelled) || len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("cancel response = %+v", resp)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; ok {
		t.Fatal("cancelled mix tick should expire")
	}
	if active := server.pendingManager.ActiveForConversation("chat_mix"); len(active) != 0 {
		t.Fatalf("cancelled mix tick should not remain active: %+v", active)
	}
	if goal := server.harness.RuntimeStatus("goal_1"); goal.Status != agentruntime.StatusCancelled {
		t.Fatalf("goal not cancelled: %+v", goal)
	}
	if !testAgentEventsContainTurnCompleted(server, "chat_mix", "goal_1", agentruntime.StatusCancelled) {
		t.Fatalf("missing final cancelled turn.completed event: %+v", mustAgentEvents(server, "chat_mix"))
	}
}

func TestInteractionRespondMixTreatmentCancelUsesTreatmentRejectPath(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.harness.EnsureGoal("goal_1", "run_1", "reduce low-mid mud")
	initial := server.chatResponseFromAgentLoopResult("chat_mix", agentModeDefault, agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		Status: agentruntime.StatusCompleted,
		Reply:  "confirm treatment",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &agentloop.MixTreatmentPending{
				SchemaVersion:    "mix_treatment_pending.v0",
				Status:           "pending_confirmation",
				ConversationID:   "chat_mix",
				ObservationID:    "obs_1",
				Intent:           "reduce low-mid mud",
				TargetRef:        "track:1007",
				ActionKind:       "plugin_treatment",
				ProcessorType:    "eq",
				PluginID:         "plugin_eq",
				PluginName:       "Test EQ",
				ReasoningSummary: "low-mid buildup around the vocal",
			},
		},
	})
	if len(initial.InteractionRequests) != 1 {
		t.Fatalf("interaction requests = %+v", initial.InteractionRequests)
	}

	body, _ := json.Marshal(InteractionRespondRequest{
		InteractionID: initial.InteractionRequests[0].ID,
		ActionID:      "cancel",
		Decision:      "cancel",
	})
	rec := httptest.NewRecorder()
	server.handleInteractionRespond(rec, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if resp.StopReason != "mix_treatment_rejected" || resp.GoalStatus != string(agentruntime.StatusCompleted) || len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("cancel should route through treatment reject path: %+v", resp)
	}
	if len(resp.InteractionRequests) != 0 {
		t.Fatalf("mix treatment cancel should not return generic mode-boundary card: %+v", resp.InteractionRequests)
	}
	if _, ok := server.pendingMixTreatmentForConversation("chat_mix"); ok {
		t.Fatal("cancelled mix treatment should expire")
	}
	if active := server.pendingManager.ActiveForConversation("chat_mix"); len(active) != 0 {
		t.Fatalf("cancelled mix treatment should not remain active: %+v", active)
	}
	if goal := server.harness.RuntimeStatus("goal_1"); goal.Status != agentruntime.StatusCompleted {
		t.Fatalf("goal not completed after reject: %+v", goal)
	}
	if !testAgentEventsContainTurnCompleted(server, "chat_mix", "goal_1", agentruntime.StatusCompleted) {
		t.Fatalf("missing final turn.completed event: %+v", mustAgentEvents(server, "chat_mix"))
	}
}

func TestInteractionRespondMixTreatmentPluginPreparationReturnsNewConfirmation(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "mix.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
		}},
	})
	kernel := &recordingChatKernel{}
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	initial := server.chatResponseFromAgentLoopResult("chat_mix", agentModeDefault, agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		Status: agentruntime.StatusCompleted,
		Reply:  "我建议先做一个轻度压缩/限幅式的响度处理，确认后再进入安全执行准备。",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &agentloop.MixTreatmentPending{
				SchemaVersion:             "mix_treatment_pending.v0",
				Status:                    "pending_confirmation",
				ConversationID:            "chat_mix",
				ObservationID:             "obs_loudness",
				Intent:                    "当前轨道响度有些小，我想更大声点",
				TargetRef:                 "track:1007",
				ActionKind:                "plugin_treatment",
				ProcessorType:             "compressor",
				PluginID:                  `C:\VST3\Test Compressor.vst3`,
				PluginName:                "Test Compressor",
				ReasoningSummary:          "peak is near 0 dBFS, prepare compression or limiting before increasing loudness",
				Confidence:                "medium",
				NeedsResolution:           []string{"plugin_instance", "plugin_profile", "exact_control"},
				ExpiresAfterContextChange: true,
			},
		},
	})

	if len(initial.InteractionRequests) != 1 {
		t.Fatalf("initial interaction requests = %+v", initial.InteractionRequests)
	}
	interactionID := initial.InteractionRequests[0].ID
	body, _ := json.Marshal(InteractionRespondRequest{
		InteractionID: interactionID,
		ActionID:      "approve",
		Decision:      "approve",
	})
	rec := httptest.NewRecorder()
	server.handleInteractionRespond(rec, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if resp.StopReason != "mix_treatment_preparation_started" || resp.Workflow != pluginGrabberLoadCommand {
		t.Fatalf("resp = %+v", resp)
	}
	if !resp.NeedsConfirmation || resp.PlanID == "" || resp.GoalStatus != string(agentruntime.StatusWaitingConfirmation) {
		t.Fatalf("plugin preparation should return a new waiting confirmation: %+v", resp)
	}
	if len(resp.InteractionRequests) != 1 || resp.InteractionRequests[0].Kind != "confirmation" {
		t.Fatalf("new interaction requests = %+v", resp.InteractionRequests)
	}
	if resp.InteractionRequests[0].ID == interactionID {
		t.Fatal("plugin preparation confirmation must be a new interaction, not the consumed mix treatment interaction")
	}
	if strings.Contains(resp.Reply, "I have") || strings.Contains(resp.Reply, "Resolver") || strings.Contains(resp.Reply, "Prep steps") || strings.Contains(resp.Reply, "已处理或已过期") {
		t.Fatalf("reply should be a Chinese continuation card, got %q", resp.Reply)
	}
	if len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("plugin preparation confirmation should not execute tools yet: %+v", resp.ExecutedKernelReply)
	}
	var mutatingCommands []map[string]any
	for _, cmd := range kernel.commands {
		switch cleanContextText(cmd["cmd"]) {
		case "rack_add_node", "set_plugin_param", "n_apply_control":
			mutatingCommands = append(mutatingCommands, cmd)
		}
	}
	if len(mutatingCommands) != 0 {
		t.Fatalf("plugin preparation should not load or write before second confirmation: %+v", kernel.commands)
	}
	if _, ok := server.takePendingInteraction(resp.InteractionRequests[0].ID); !ok {
		t.Fatal("new plugin preparation interaction was not stored")
	}
}

func TestAgentLoopFailedAfterExecutionReportsPartialSuccess(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp := server.chatResponseFromAgentLoopResult("chat_test", agentModeDefault, agentloop.Result{
		GoalID:      "goal_1",
		RunID:       "run_1",
		Status:      agentruntime.StatusFailed,
		StopReason:  agentloop.StopReasonFailed,
		Reply:       "执行失败：LLM HTTP error 502",
		Error:       "LLM HTTP error 502",
		GoalSummary: "观察当前音频",
		Executed: []map[string]any{{
			"status":       "ok",
			"tool":         "mix.request_observation",
			"command_name": "mix_request_observation",
			"result":       map[string]any{"artifact": map[string]any{"id": "obs_test", "kind": "document", "title": "obs.json"}},
		}},
	})
	if !strings.Contains(resp.Reply, "前面的工具操作已完成") || !strings.Contains(resp.Reply, "LLM HTTP error 502") {
		t.Fatalf("reply = %q", resp.Reply)
	}
	if len(resp.Artifacts) != 1 {
		t.Fatalf("artifacts = %+v", resp.Artifacts)
	}
}

func TestContextWithArtifactSummariesCompactsMultiArtifactPrompt(t *testing.T) {
	summaries := make([]artifacts.Summary, 0, 14)
	for i := 0; i < 14; i++ {
		summaries = append(summaries, artifacts.Summary{
			ID:        fmt.Sprintf("art_%02d", i),
			Kind:      "audio",
			Title:     fmt.Sprintf("stem_%02d.wav", i),
			Path:      fmt.Sprintf(`E:\BaiduNetdiskDownload\sattelites\stem_%02d.wav`, i),
			MIME:      "audio/wav",
			SizeBytes: 1000 + int64(i),
			Status:    "ready",
			Metadata:  map[string]any{"sample_rate": 48000},
			Summary:   "Audio file, duration 120.00s",
		})
	}

	ctx := contextWithArtifactSummaries(nil, summaries)
	if ctx["artifact_count"] != 14 || ctx["artifacts_omitted_count"] != 2 {
		t.Fatalf("artifact catalog fields = %+v", ctx)
	}
	rows := mapRowsValue(ctx["artifacts"])
	if len(rows) != 12 {
		t.Fatalf("prompt artifact rows = %d, ctx=%+v", len(rows), ctx)
	}
	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "BaiduNetdiskDownload") || strings.Contains(text, "metadata") || strings.Contains(text, "stem_13.wav") {
		t.Fatalf("multi-artifact prompt context leaked full payload: %s", text)
	}

	single := contextWithArtifactSummaries(nil, summaries[:1])
	row := mapRowsValue(single["artifacts"])[0]
	if !strings.Contains(fmt.Sprint(row["path"]), "BaiduNetdiskDownload") {
		t.Fatalf("single explicit artifact should retain path for direct import: %+v", single)
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

func TestHandleChatMergesCachedUIContextForSelectedClipFadeGain(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_BASE_URL", "http://127.0.0.1:9/v1")
	t.Setenv("VIT_AGENT_LLM_API_KEY", "test-key")
	t.Setenv("VIT_AGENT_LLM_MODEL", "test-model")

	kernel := &recordingChatKernel{replyByCommand: map[string][]map[string]any{
		"clip.fade.read": {{
			"status":           "ok",
			"clip_id":          "1011",
			"fade_in_seconds":  0.125,
			"fade_out_seconds": 0.25,
		}},
		"clip.gain.read": {{
			"status":  "ok",
			"clip_id": "1011",
			"gain_db": 1.5,
		}},
	}}
	shadowProject := shadow.New(nil)
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	server.uiContext = sanitizeUIContext(map[string]any{
		"selected_track_id":        "1007",
		"selected_track_name":      "Track 1",
		"selected_clip_id":         "1011",
		"selected_clip_ids":        []any{"1011"},
		"selected_clip_track_id":   "1007",
		"current_playhead_seconds": 0,
	})

	body, _ := json.Marshal(ChatRequest{
		ConversationID: "chat_clip_ui_context",
		Message:        "read selected clip fade and gain status",
		Context: map[string]any{
			"agent_mode":        agentModeDefault,
			"selected_track_id": "1007",
		},
	})
	rec := httptest.NewRecorder()
	server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.GoalStatus == string(agentruntime.StatusWaitingClarification) || resp.StopReason == "needs_clarification" || strings.Contains(resp.Reply, "Please select") || strings.Contains(resp.Reply, "请先在 GUI") {
		t.Fatalf("selected clip from cached UI context was not recognized: %+v", resp)
	}
	seen := map[string]map[string]any{}
	for _, cmd := range kernel.commands {
		name := cleanContextText(cmd["cmd"])
		if name != "" {
			seen[name] = cmd
		}
		if strings.HasPrefix(name, "mix.") {
			t.Fatalf("clip fade/gain chat routed to mix command %s; commands=%+v", name, kernel.commands)
		}
	}
	for _, want := range []string{"clip.fade.read", "clip.gain.read"} {
		cmd := seen[want]
		if cmd == nil {
			t.Fatalf("missing %s command; commands=%+v response=%+v", want, kernel.commands, resp)
		}
		if cleanContextText(cmd["clip_id"]) != "1011" {
			t.Fatalf("%s clip_id = %q, want 1011; cmd=%+v", want, cleanContextText(cmd["clip_id"]), cmd)
		}
	}
}

func TestSelectedClipRangesSurvivePromptAndCurrentSelectionContext(t *testing.T) {
	uiContext := map[string]any{
		"selected_clip_ranges": []any{
			map[string]any{
				"range_id":                 "range_1",
				"clip_id":                  "1011",
				"track_id":                 "1007",
				"start_seconds":            2.0,
				"end_seconds":              3.5,
				"duration_seconds":         1.5,
				"clip_local_start_seconds": 0.5,
				"clip_local_end_seconds":   2.0,
			},
		},
		"selected_clip_range_count": 1,
	}
	merged := contextWithCachedUIContext(map[string]any{}, uiContext)
	withMessage := contextWithUserMessage(merged, "检查这个范围")
	current := mapValue(withMessage["current_selection"])
	if current["selected_clip_range_count"] != 1 {
		t.Fatalf("current selection lost range count: %+v", current)
	}
	ranges, ok := current["selected_clip_ranges"].([]any)
	if !ok || len(ranges) != 1 {
		t.Fatalf("current selection lost clip ranges: %+v", current["selected_clip_ranges"])
	}
	prompt := agentContextForPrompt(withMessage)
	for _, want := range []string{"selected_clip_ranges", "range_1", "1011", "duration_seconds"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt context missing %q: %s", want, prompt)
		}
	}
}

func TestHandleChatClipFadeGainReadExpiresStaleClipGainConfirmation(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_BASE_URL", "http://127.0.0.1:9/v1")
	t.Setenv("VIT_AGENT_LLM_API_KEY", "test-key")
	t.Setenv("VIT_AGENT_LLM_MODEL", "test-model")

	kernel := &recordingChatKernel{replyByCommand: map[string][]map[string]any{
		"clip.fade.read": {{
			"status":           "ok",
			"clip_id":          "1011",
			"fade_in_seconds":  0.125,
			"fade_out_seconds": 0.25,
		}},
		"clip.gain.read": {{
			"status":       "ok",
			"clip_id":      "1011",
			"clip_gain_db": -3.0,
		}},
	}}
	shadowProject := shadow.New(nil)
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	server.uiContext = sanitizeUIContext(map[string]any{
		"selected_track_id":      "1007",
		"selected_clip_id":       "1011",
		"selected_clip_ids":      []any{"1011"},
		"selected_clip_track_id": "1007",
	})

	server.pending["plan_stale_clip_gain"] = PendingPlan{
		ID:        "plan_stale_clip_gain",
		CreatedAt: time.Now(),
		Context: map[string]any{
			"conversation_id": "chat_clip_pending",
			"goal_id":         "goal_stale",
			"run_id":          "run_stale",
		},
		WorkflowData: map[string]any{"conversation_id": "chat_clip_pending"},
		Decisions: []policy.Decision{{
			Name: "clip.gain.set",
			Risk: policy.RiskConfirm,
			Command: map[string]any{
				"cmd":      "clip.gain.set",
				"clip_id":  "1011",
				"track_id": "1007",
				"gain_db":  -3,
			},
		}},
	}
	server.storePendingInteraction(AgentInteractionRequest{
		ID:             "interaction_stale_clip_gain",
		Kind:           "confirmation",
		Type:           "confirmation",
		PlanID:         "plan_stale_clip_gain",
		ConversationID: "chat_clip_pending",
		GoalID:         "goal_stale",
		RunID:          "run_stale",
	}, map[string]any{"plan_id": "plan_stale_clip_gain"})
	server.goalContinuations["goal_stale"] = agentloop.Continuation{GoalID: "goal_stale", RunID: "run_stale"}

	body, _ := json.Marshal(ChatRequest{
		ConversationID: "chat_clip_pending",
		Message:        "读取当前选中 clip 的 fade 和 gain 状态",
		Context:        map[string]any{"agent_mode": agentModeDefault},
	})
	rec := httptest.NewRecorder()
	server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.NeedsConfirmation || resp.GoalStatus == string(agentruntime.StatusWaitingConfirmation) || resp.PlanID != "" {
		t.Fatalf("clip read should not be intercepted by stale confirmation: %+v", resp)
	}
	if strings.Contains(resp.Reply, "等待确认") || strings.Contains(resp.Reply, "waiting") {
		t.Fatalf("reply still looks like pending confirmation: %q", resp.Reply)
	}
	seen := map[string]bool{}
	for _, cmd := range kernel.commands {
		seen[cleanContextText(cmd["cmd"])] = true
	}
	for _, want := range []string{"clip.fade.read", "clip.gain.read"} {
		if !seen[want] {
			t.Fatalf("missing %s command; commands=%+v response=%+v", want, kernel.commands, resp)
		}
	}
	if _, ok := server.pending["plan_stale_clip_gain"]; ok {
		t.Fatal("stale clip gain pending plan should be expired")
	}
	if _, ok := server.goalContinuations["goal_stale"]; ok {
		t.Fatal("stale goal continuation should be cleared")
	}
	if _, ok := server.takePendingInteraction("interaction_stale_clip_gain"); ok {
		t.Fatal("stale clip gain interaction should be expired")
	}
}

func TestAgentLoopCapabilityNamesDetectsConversationalMix(t *testing.T) {
	caps := agentLoopCapabilityNames("把当前主唱轨道往前一点", map[string]any{"selected_track_id": "track_1"})
	seen := map[string]bool{}
	for _, cap := range caps {
		seen[cap] = true
	}
	if !seen["mix"] {
		t.Fatalf("expected mix capability, got %+v", caps)
	}
	server := New(nil, shadow.New(nil), nil)
	tools := toolNamesForAgentLoopCapabilities(server.harness, agentModeDefault, []string{"mix"})
	toolSet := map[string]bool{}
	for _, tool := range tools {
		toolSet[tool] = true
	}
	if !toolSet["mix.request_observation"] {
		t.Fatalf("mix tools should include observation, got %+v", tools)
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
	planInstruction := agentModeSystemInstruction(agentModePlan)
	for _, want := range []string{"Plan mode", "read-only", "do not say the DAW lacks that capability"} {
		if !strings.Contains(planInstruction, want) {
			t.Fatalf("plan instruction missing %q: %s", want, planInstruction)
		}
	}
}

func TestGoalIDsFromContextTreatsMissingValuesAsEmpty(t *testing.T) {
	goalID, runID := goalIDsFromContext(map[string]any{})
	if goalID != "" || runID != "" {
		t.Fatalf("missing ids = goal %q run %q", goalID, runID)
	}
	goalID, runID = goalIDsFromContext(map[string]any{"goal_id": nil, "run_id": nil})
	if goalID != "" || runID != "" {
		t.Fatalf("nil ids = goal %q run %q", goalID, runID)
	}
	goalID, runID = goalIDsFromContext(map[string]any{"goal_id": " goal_1 ", "run_id": " run_1 "})
	if goalID != "goal_1" || runID != "run_1" {
		t.Fatalf("trimmed ids = goal %q run %q", goalID, runID)
	}
}

func TestContextWithConversationID(t *testing.T) {
	out := contextWithConversationID(map[string]any{"selected_track_id": "track_1"}, " chat_1 ")
	if out["conversation_id"] != "chat_1" || out["selected_track_id"] != "track_1" {
		t.Fatalf("context = %+v", out)
	}
	out = contextWithConversationID(nil, "")
	if len(out) != 0 {
		t.Fatalf("empty conversation should not add field: %+v", out)
	}
}

func TestUIStateReturnsWebUIReadModel(t *testing.T) {
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": "D:/song/demo.vit",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Vocal Lead", "track_type": "hybrid", "is_audio_track": true, "plugins": []any{map[string]any{"plugin_id": "p1", "plugin_name": "EQ"}}},
			map[string]any{"track_id": "1008", "track_name": "Master", "track_type": "master", "is_audio_track": false},
		},
	})
	server := New(nil, shadowProject, nil)
	server.artifactRoot = t.TempDir()

	req := httptest.NewRequest(http.MethodGet, "/agent/ui/state", nil)
	rec := httptest.NewRecorder()
	server.handleUIState(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["project"] == nil || body["transport"] == nil || body["plugin_rack"] == nil {
		t.Fatalf("ui state missing expected fields: %+v", body)
	}
	tracks, ok := body["tracks"].([]any)
	if !ok || len(tracks) != 1 {
		t.Fatalf("tracks = %+v", body["tracks"])
	}
}

func TestUIContextSelectionMergesIntoUIState(t *testing.T) {
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Vocal Lead", "track_type": "hybrid", "is_audio_track": true, "plugins": []any{
				map[string]any{"plugin_id": "p1", "plugin_name": "TDR Nova"},
			}},
		},
	})
	server := New(nil, shadowProject, nil)
	server.artifactRoot = t.TempDir()

	payload := strings.NewReader(`{"selected_plugin_id":"p1","selected_plugin_name":"TDR Nova","selected_plugin_track_id":"1007","selected_plugin_source":"rack"}`)
	contextReq := httptest.NewRequest(http.MethodPost, "/agent/ui/context", payload)
	contextRec := httptest.NewRecorder()
	server.handleUIContext(contextRec, contextReq)
	if contextRec.Code != http.StatusOK {
		t.Fatalf("context status = %d body=%s", contextRec.Code, contextRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/agent/ui/state", nil)
	rec := httptest.NewRecorder()
	server.handleUIState(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ui state status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	selectedPlugin, ok := body["selected_plugin"].(map[string]any)
	if !ok || selectedPlugin["plugin_id"] != "p1" || selectedPlugin["plugin_name"] != "TDR Nova" {
		t.Fatalf("selected_plugin = %+v", body["selected_plugin"])
	}
	rack := body["plugin_rack"].(map[string]any)
	plugins := rack["plugins"].([]any)
	if len(plugins) != 1 || plugins[0].(map[string]any)["selected"] != true {
		t.Fatalf("plugin rack selection = %+v", rack["plugins"])
	}
}

func TestUIContextPreservesSelectedClipRanges(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	payload := strings.NewReader(`{
		"selected_clip_ranges": [
			{
				"range_id": "range_1",
				"clip_id": "clip_a",
				"track_id": "track_a",
				"start_seconds": 2.0,
				"end_seconds": 3.5,
				"duration_seconds": 1.5,
				"clip_local_start_seconds": 0.5,
				"clip_local_end_seconds": 2.0
			}
		],
		"selected_clip_range_count": 1,
		"selected_clip_range": {
			"range_id": "range_1",
			"clip_id": "clip_a",
			"track_id": "track_a",
			"start_seconds": 2.0,
			"end_seconds": 3.5,
			"duration_seconds": 1.5,
			"clip_local_start_seconds": 0.5,
			"clip_local_end_seconds": 2.0
		}
	}`)
	contextReq := httptest.NewRequest(http.MethodPost, "/agent/ui/context", payload)
	contextRec := httptest.NewRecorder()
	server.handleUIContext(contextRec, contextReq)
	if contextRec.Code != http.StatusOK {
		t.Fatalf("context status = %d body=%s", contextRec.Code, contextRec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(contextRec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	context := mapValue(body["context"])
	if context["selected_clip_range_count"] != float64(1) {
		t.Fatalf("range count was not preserved: %+v", context)
	}
	ranges, ok := context["selected_clip_ranges"].([]any)
	if !ok || len(ranges) != 1 {
		t.Fatalf("selected_clip_ranges not preserved: %+v", context["selected_clip_ranges"])
	}
	first := mapValue(ranges[0])
	if cleanContextText(first["range_id"]) != "range_1" || cleanContextText(first["clip_id"]) != "clip_a" {
		t.Fatalf("unexpected range payload: %+v", first)
	}
}

func TestChatSmokeRangeContextUsesCachedUIContext(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	payload := strings.NewReader(`{
		"selected_clip_ranges": [
			{
				"range_id": "range_1",
				"clip_id": "clip_a",
				"track_id": "track_a",
				"start_seconds": 2.0,
				"end_seconds": 3.5,
				"duration_seconds": 1.5,
				"clip_local_start_seconds": 0.5,
				"clip_local_end_seconds": 2.0
			}
		],
		"selected_clip_range_count": 1
	}`)
	contextReq := httptest.NewRequest(http.MethodPost, "/agent/ui/context", payload)
	contextRec := httptest.NewRecorder()
	server.handleUIContext(contextRec, contextReq)
	if contextRec.Code != http.StatusOK {
		t.Fatalf("context status = %d body=%s", contextRec.Code, contextRec.Body.String())
	}

	body, _ := json.Marshal(ChatRequest{
		ConversationID: "chat_range_context_smoke",
		Message:        "/smoke range_context",
		Context:        map[string]any{"agent_mode": agentModeDefault},
	})
	rec := httptest.NewRecorder()
	server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.StopReason != "range_context_smoke_ok" {
		t.Fatalf("range context smoke failed: %+v", resp)
	}
	for _, want := range []string{"top_level", "current_selection", "ui_context", "prompt_context"} {
		if !strings.Contains(resp.Reply, want) {
			t.Fatalf("range context smoke reply missing %q: %q", want, resp.Reply)
		}
	}
}

func TestUIMacroControlsPreserveBoundMacroWhenEmptyRackMacroAlsoExists(t *testing.T) {
	state := map[string]any{
		"macro_controls": []map[string]any{{
			"macro_id": "mix_macro_session_track_volume",
			"name":     "Track volume",
			"track_id": "1007",
			"source":   "mixboard_control_plan",
			"control":  "track.volume",
			"value":    -6.0,
			"min":      -60.0,
			"max":      12.0,
			"unit":     "dB",
			"bindings": []map[string]any{{
				"control":  "track.volume",
				"track_id": "1007",
				"param_id": "track.volume",
			}},
		}},
		"rack_control_macros": []map[string]any{{
			"macro_id": "mix_macro_session_track_volume",
			"name":     "Track volume",
			"track_id": "1007",
			"source":   "godot_rack",
			"value":    0.0,
			"min":      0.0,
			"max":      1.0,
			"bindings": []map[string]any{},
		}},
	}

	macros := uiMacroControls(state)
	if len(macros) != 1 {
		t.Fatalf("macros = %#v", macros)
	}
	macro := macros[0]
	if mixFloatNumber(macro["min"]) != -60 || mixFloatNumber(macro["max"]) != 12 || mixFloatNumber(macro["value"]) != -6 {
		t.Fatalf("dB macro was downgraded: %#v", macro)
	}
	bindings := mapRowsFromAny(macro["bindings"])
	if len(bindings) != 1 || cleanContextText(bindings[0]["param_id"]) != "track.volume" {
		t.Fatalf("bindings = %#v", bindings)
	}
}

func TestUIMacroControlsEnrichEmptyTrackVolumeMacroFromTrackState(t *testing.T) {
	state := map[string]any{
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": -8.25,
		}},
		"rack_control_macros": []map[string]any{{
			"macro_id": "mix_macro_session_track_volume",
			"name":     "Track volume",
			"track_id": "1007",
			"source":   "godot_rack",
			"value":    0.0,
			"min":      0.0,
			"max":      1.0,
		}},
	}

	macros := uiMacroControls(state)
	if len(macros) != 1 {
		t.Fatalf("macros = %#v", macros)
	}
	macro := macros[0]
	if cleanContextText(macro["control"]) != "track.volume" || mixFloatNumber(macro["min"]) != -60 || mixFloatNumber(macro["max"]) != 12 {
		t.Fatalf("macro was not enriched as track volume: %#v", macro)
	}
	if mixFloatNumber(macro["value"]) != -8.25 {
		t.Fatalf("macro value did not follow track volume: %#v", macro)
	}
	bindings := mapRowsFromAny(macro["bindings"])
	if len(bindings) != 1 || cleanContextText(bindings[0]["param_id"]) != "track.volume" {
		t.Fatalf("bindings = %#v", bindings)
	}
}

func TestArtifactHTTPUploadReadFileAndList(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.artifactRoot = t.TempDir()

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("file", "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("hello artifact upload")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/agent/artifacts/upload", &form)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	server.handleArtifactUpload(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d body=%s", rec.Code, rec.Body.String())
	}
	var upload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &upload); err != nil {
		t.Fatal(err)
	}
	rows := upload["artifacts"].([]any)
	if len(rows) != 1 {
		t.Fatalf("upload artifacts = %+v", upload["artifacts"])
	}
	id := rows[0].(map[string]any)["id"].(string)

	readReq := httptest.NewRequest(http.MethodGet, "/agent/artifact?id="+id, nil)
	readRec := httptest.NewRecorder()
	server.handleArtifact(readRec, readReq)
	if readRec.Code != http.StatusOK || !strings.Contains(readRec.Body.String(), "hello artifact upload") {
		t.Fatalf("read status=%d body=%s", readRec.Code, readRec.Body.String())
	}

	fileReq := httptest.NewRequest(http.MethodGet, "/agent/artifact/file?id="+id, nil)
	fileRec := httptest.NewRecorder()
	server.handleArtifactFile(fileRec, fileReq)
	if fileRec.Code != http.StatusOK || fileRec.Body.String() != "hello artifact upload" {
		t.Fatalf("file status=%d body=%q", fileRec.Code, fileRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/agent/artifacts?limit=10", nil)
	listRec := httptest.NewRecorder()
	server.handleArtifacts(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), id) {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}
}

func TestArtifactUploadStoresPluginLearningMetadata(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.artifactRoot = t.TempDir()

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	fields := map[string]string{
		"plugin_learning_session_id": "pl_session_test",
		"plugin_learning_purpose":    pluginLearningUIReferencePurpose,
		"track_id":                   "track_1",
		"plugin_id":                  "plugin_1",
		"plugin_name":                "Example EQ",
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", "ui.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("not a real png but still an uploaded image artifact")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/agent/artifacts/upload", &form)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	server.handleArtifactUpload(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d body=%s", rec.Code, rec.Body.String())
	}
	var upload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &upload); err != nil {
		t.Fatal(err)
	}
	rows := upload["artifacts"].([]any)
	id := rows[0].(map[string]any)["id"].(string)
	stored, err := server.artifactStore().Get(id)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range fields {
		if got := strings.TrimSpace(fmt.Sprint(stored.Metadata[key])); got != want {
			t.Fatalf("metadata[%s] = %q, want %q", key, got, want)
		}
	}
}

func TestPluginGrabberAutoLearnStartsUIReferenceRequestBeforeKernel(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp := server.runPluginGrabberLearningWorkflow(context.Background(), "conv_ui_ref", "", nil, config.EngineConfig{}, map[string]any{
		"mode":        "auto_learn",
		"track_id":    "track_1",
		"plugin_id":   "plugin_1",
		"plugin_name": "Example EQ",
	})
	if resp.Error != "" {
		t.Fatalf("first auto_learn returned error = %q", resp.Error)
	}
	if got := strings.TrimSpace(fmt.Sprint(resp.PluginLearning["stage"])); got != pluginLearningUIReferenceStage {
		t.Fatalf("stage = %q, want %q", got, pluginLearningUIReferenceStage)
	}
	if got := strings.TrimSpace(fmt.Sprint(resp.PluginLearning["plugin_learning_session_id"])); got == "" {
		t.Fatalf("missing plugin_learning_session_id: %+v", resp.PluginLearning)
	}
	if strings.Contains(resp.Reply, "kernel client is nil") {
		t.Fatalf("workflow reached kernel before UI reference request: %q", resp.Reply)
	}
}

func TestPluginGrabberUIReferenceCarriesMixTreatmentPreparationPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := map[string]any{
		"schema_version": "mix_treatment_preparation.v0",
		"safe_route":     []any{"plugin_grabber.learn_project_profile", "plugin_grabber.apply_control"},
		"blocked_routes": []any{"daw.invoke", "plugin.set_parameter"},
	}
	resp := server.runPluginGrabberLearningWorkflow(context.Background(), "conv_ui_ref", "reduce mud", map[string]any{
		"mix_treatment_preparation":      true,
		"mix_treatment_preparation_plan": plan,
	}, config.EngineConfig{}, map[string]any{
		"mode":        "auto_learn",
		"track_id":    "track_1",
		"plugin_id":   "plugin_1",
		"plugin_name": "Example EQ",
	})
	if resp.Error != "" {
		t.Fatalf("first auto_learn returned error = %q", resp.Error)
	}
	got := mapValue(resp.PluginLearning["mix_treatment_preparation_plan"])
	if cleanContextText(got["schema_version"]) != "mix_treatment_preparation.v0" {
		t.Fatalf("plugin learning plan = %+v", resp.PluginLearning)
	}
	if !testAnyStringSliceContains(got["safe_route"], "plugin_grabber.apply_control") || !testAnyStringSliceContains(got["blocked_routes"], "plugin.set_parameter") {
		t.Fatalf("plugin learning route contract = %+v", got)
	}
	req := server.pluginLearningInteractionRequest(resp)
	reqPlan := mapValue(req.Payload["mix_treatment_preparation_plan"])
	if cleanContextText(reqPlan["schema_version"]) != "mix_treatment_preparation.v0" {
		t.Fatalf("interaction payload did not carry plan: %+v", req.Payload)
	}
}

func TestPluginLearningUIReferenceInteractionRequest(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp := pluginGrabberUIReferenceRequestResponse("conv_ui_ref", "", nil, pluginLearningTarget{
		TrackID:    "track_1",
		PluginID:   "plugin_1",
		PluginName: "Example EQ",
	}, "pl_session_test", time.Now().UTC().Format(time.RFC3339Nano))
	req := server.pluginLearningInteractionRequest(resp)
	if req.Type != pluginLearningUIReferenceType || req.Stage != pluginLearningUIReferenceStage {
		t.Fatalf("interaction type/stage = %q/%q", req.Type, req.Stage)
	}
	actions := map[string]bool{}
	for _, action := range req.Actions {
		actions[action.ID] = true
	}
	for _, id := range []string{"continue_with_ui_reference", "skip_ui_reference", "cancel_plugin_learning"} {
		if !actions[id] {
			t.Fatalf("missing action %q in %+v", id, req.Actions)
		}
	}
}

func TestPluginWebReferenceDecisionNormalizesOptionalCardPayload(t *testing.T) {
	if got := pluginWebReferenceDecision(map[string]any{"web_reference_decision": "enabled"}); got != "enabled" {
		t.Fatalf("decision = %q", got)
	}
	if got := pluginWebReferenceDecision(map[string]any{"web_reference_enabled": true}); got != "enabled" {
		t.Fatalf("bool decision = %q", got)
	}
	if got := pluginWebReferenceDecision(map[string]any{"web_reference_decision": "skipped"}); got != "skipped" {
		t.Fatalf("skipped decision = %q", got)
	}
}

func TestValidPluginUIReferenceArtifactsAreSessionScopedImages(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.artifactRoot = t.TempDir()
	store := server.artifactStore()
	sessionID := "pl_session_test"
	startedAt := time.Now().UTC()
	target := pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Example EQ"}
	makeArtifact := func(id, kind string, createdAt time.Time, metadata map[string]any) artifacts.Artifact {
		a := artifacts.New(createdAt)
		a.ID = id
		a.Kind = kind
		if kind == "image" {
			a.MIME = "image/png"
		} else {
			a.MIME = "text/plain"
		}
		a.Metadata = metadata
		if _, err := store.Upsert(a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	validMeta := map[string]any{
		"plugin_learning_session_id": sessionID,
		"plugin_learning_purpose":    pluginLearningUIReferencePurpose,
		"track_id":                   target.TrackID,
		"plugin_id":                  target.PluginID,
	}
	makeArtifact("art_valid", "image", startedAt.Add(time.Second), validMeta)
	makeArtifact("art_old", "image", startedAt.Add(-time.Second), validMeta)
	makeArtifact("art_no_session", "image", startedAt.Add(time.Second), map[string]any{"track_id": target.TrackID, "plugin_id": target.PluginID})
	makeArtifact("art_wrong_session", "image", startedAt.Add(time.Second), map[string]any{
		"plugin_learning_session_id": "other",
		"plugin_learning_purpose":    pluginLearningUIReferencePurpose,
		"track_id":                   target.TrackID,
		"plugin_id":                  target.PluginID,
	})
	makeArtifact("art_text", "text", startedAt.Add(time.Second), validMeta)
	args := map[string]any{
		"plugin_learning_session_id":         sessionID,
		"plugin_learning_session_started_at": startedAt.Format(time.RFC3339Nano),
		"ui_reference_artifact_ids":          []string{"art_valid", "art_old", "art_no_session", "art_wrong_session", "art_text"},
	}
	valid, summaries, _ := server.validPluginUIReferenceArtifacts(args, target)
	if len(valid) != 1 || len(summaries) != 1 || valid[0].ID != "art_valid" {
		t.Fatalf("valid artifacts = %+v summaries=%+v", valid, summaries)
	}
}

func TestResolvePluginUIReferenceVisionUnavailableContinues(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.artifactRoot = t.TempDir()
	sessionID := "pl_session_test"
	startedAt := time.Now().UTC()
	target := pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Example EQ"}
	imagePath := filepath.Join(t.TempDir(), "ui.png")
	if err := os.WriteFile(imagePath, []byte("image bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := artifacts.New(startedAt.Add(time.Second))
	a.ID = "art_ui_ref"
	a.Kind = "image"
	a.MIME = "image/png"
	a.Path = imagePath
	a.Metadata = map[string]any{
		"plugin_learning_session_id": sessionID,
		"plugin_learning_purpose":    pluginLearningUIReferencePurpose,
		"track_id":                   target.TrackID,
		"plugin_id":                  target.PluginID,
	}
	if _, err := server.artifactStore().Upsert(a); err != nil {
		t.Fatal(err)
	}
	uiReference, summaries, err := server.resolvePluginUIReference(context.Background(), config.EngineConfig{
		BaseURL:      "https://example.test/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: false, Provider: "default"},
		},
	}, map[string]any{
		"ui_reference_decision":              "provided",
		"ui_reference_artifact_ids":          []string{"art_ui_ref"},
		"plugin_learning_session_id":         sessionID,
		"plugin_learning_session_started_at": startedAt.Format(time.RFC3339Nano),
	}, target, pluginParameterDigest{})
	if err != nil {
		t.Fatalf("resolvePluginUIReference error: %v", err)
	}
	if got := strings.TrimSpace(fmt.Sprint(uiReference["status"])); got != "vision_unavailable" {
		t.Fatalf("status = %q uiReference=%+v", got, uiReference)
	}
	if len(summaries) != 1 || summaries[0].ID != "art_ui_ref" {
		t.Fatalf("summaries = %+v", summaries)
	}
}

func TestBuildPluginUIReferenceDigestUsesTargetedVisionProbe(t *testing.T) {
	var visionCalled bool
	var textCalled bool
	backendParamSeenInVision := false
	serverHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		switch r.URL.Path {
		case "/v1/responses":
			visionCalled = true
			bodyText := string(body)
			if !strings.Contains(bodyText, "input_image") || !strings.Contains(bodyText, "data:image/png;base64,") {
				t.Fatalf("vision request did not include image input: %s", bodyText)
			}
			backendParamSeenInVision = strings.Contains(bodyText, "plugin_ui_reference_visual_target_focus.v1") && strings.Contains(bodyText, "p_gain")
			if !backendParamSeenInVision {
				t.Fatalf("vision probe should include targeted backend candidates: %s", bodyText)
			}
			if strings.Contains(bodyText, "plugin_backend_visual_search_guide.v1") {
				t.Fatalf("vision probe should not receive the full backend candidate ledger: %s", bodyText)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"{\"schema\":\"plugin_ui_reference_target_probe.v1\",\"target_results\":[{\"target_id\":\"target_01\",\"control\":\"Band 1 Gain\",\"status\":\"visible\",\"visible_label\":\"GAIN 0.0 dB\",\"visible_group\":\"Band 1\",\"visible_unit\":\"dB\",\"display_domain\":\"-24 to +24 dB\",\"backend_candidate_param_ids\":[\"p_gain\"],\"likely_backend_param_id\":\"p_gain\",\"mapping_reason\":\"Visible gain readout matches backend gain control.\",\"confidence\":0.94}],\"candidate_parameter_matches\":[{\"visible_label\":\"GAIN 0.0 dB\",\"visible_group\":\"Band 1\",\"visible_unit\":\"dB\",\"display_domain\":\"-24 to +24 dB\",\"backend_param_id\":\"p_gain\",\"backend_param_name\":\"Band 1 Gain\",\"match_reason\":\"Visible gain readout matches backend gain control.\",\"evidence\":[\"ui_label\",\"backend_name\",\"display_probe\"],\"confidence\":0.93}],\"layout\":{\"summary\":\"One band section is visible.\"},\"confidence\":0.92}"}]}]}`))
		case "/v1/chat/completions":
			textCalled = true
			t.Fatalf("targeted visual learning should not call text matching: %s", string(body))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer serverHTTP.Close()

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: serverHTTP.Client()}
	imagePath := filepath.Join(t.TempDir(), "ui.png")
	if err := os.WriteFile(imagePath, []byte("image bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Example EQ"}
	digest, warning, err := server.buildPluginUIReferenceDigest(context.Background(), config.EngineConfig{
		BaseURL:      serverHTTP.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default", Model: "vision-model"},
		},
	}, target, []artifacts.Artifact{{
		Kind: "image",
		MIME: "image/png",
		Path: imagePath,
	}}, pluginParameterDigest{
		ParameterCount: 1,
		Parameters: []pluginParameterInfo{{
			ID:               "p_gain",
			Name:             "Band 1 Gain",
			DisplayGroup:     "Band 1",
			NormalizedRole:   "eq_gain",
			ControlRelevance: "used_in_component",
			HostControllable: true,
			DisplayProbe: &plugingrabber.ParameterDisplayProbe{
				CurrentText: "0.0 dB",
				Samples: []plugingrabber.ParameterDisplayProbeSample{{
					NormalizedValue: 0.5,
					Text:            "0.0 dB",
				}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("buildPluginUIReferenceDigest error: %v", err)
	}
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if !visionCalled || textCalled || !backendParamSeenInVision {
		t.Fatalf("visionCalled=%v textCalled=%v backendParamSeenInVision=%v", visionCalled, textCalled, backendParamSeenInVision)
	}
	if got := firstNonEmptyText(digest, "schema"); got != "plugin_ui_reference_digest.v1" {
		t.Fatalf("schema = %q digest=%+v", got, digest)
	}
	if got := firstNonEmptyText(digest, "visual_extraction_schema"); got != "plugin_ui_reference_target_probe.v1" {
		t.Fatalf("visual extraction schema = %q digest=%+v", got, digest)
	}
	if got := firstNonEmptyText(digest, "matching_status"); got != "target_probe_observed" {
		t.Fatalf("matching status = %q digest=%+v", got, digest)
	}
	if audit := mapValue(digest["parameter_coverage_audit"]); intNumber(audit["backend_param_total"]) != 1 || intNumber(audit["dropped_param_count"]) != 0 {
		t.Fatalf("parameter coverage audit = %+v", audit)
	}
	matches := mapRowsValue(digest["candidate_parameter_matches"])
	if len(matches) != 1 || firstNonEmptyText(matches[0], "backend_param_id") != "p_gain" {
		t.Fatalf("candidate matches = %+v", matches)
	}
	targets := mapRowsValue(digest["target_results"])
	if len(targets) != 1 || firstNonEmptyText(targets[0], "likely_backend_param_id") != "p_gain" {
		t.Fatalf("target results = %+v", targets)
	}
}

func TestBuildPluginUIReferenceDigestBatchesLargeVisualBackendMatching(t *testing.T) {
	t.Skip("legacy batched text matching is no longer used by type-guided targeted visual probing")
	roles := []struct {
		name string
		unit string
		text string
	}{
		{name: "Frequency", unit: "Hz", text: "100 Hz"},
		{name: "Gain", unit: "dB", text: "0.0 dB"},
		{name: "Amount", unit: "%", text: "50%"},
		{name: "Time", unit: "ms", text: "250 ms"},
		{name: "Mix", unit: "%", text: "35%"},
		{name: "Mode", unit: "", text: "A"},
		{name: "Threshold", unit: "dB", text: "-18 dB"},
		{name: "Ratio", unit: "ratio", text: "2:1"},
		{name: "Attack", unit: "ms", text: "12 ms"},
		{name: "Release", unit: "ms", text: "120 ms"},
		{name: "Enable", unit: "toggle", text: "On"},
		{name: "Shape", unit: "", text: "Bell"},
	}
	params := make([]pluginParameterInfo, 0, 72)
	for groupIndex := 0; groupIndex < 6; groupIndex++ {
		group := fmt.Sprintf("Module %d", groupIndex+1)
		for roleIndex, role := range roles {
			id := fmt.Sprintf("p_%02d_%02d", groupIndex+1, roleIndex+1)
			params = append(params, pluginParameterInfo{
				ID:               id,
				Name:             group + " " + role.name,
				DisplayGroup:     group,
				NormalizedRole:   strings.ToLower(role.name),
				ControlRelevance: "used_in_component",
				HostControllable: true,
				Unit:             role.unit,
				ValueText:        role.text,
				DisplayProbe: &plugingrabber.ParameterDisplayProbe{
					CurrentText: role.text,
					Label:       role.name,
					Samples: []plugingrabber.ParameterDisplayProbeSample{{
						NormalizedValue: 0.5,
						Text:            role.text,
					}},
				},
			})
		}
	}
	visibleControls := make([]map[string]any, 0, 12)
	unitRows := make([]map[string]any, 0, 12)
	groupRows := make([]map[string]any, 0, 6)
	for groupIndex := 0; groupIndex < 6; groupIndex++ {
		group := fmt.Sprintf("Module %d", groupIndex+1)
		groupRows = append(groupRows, map[string]any{"label": group, "description": "Visible processing section", "confidence": 0.9})
		for _, control := range []struct {
			label string
			unit  string
			value string
		}{
			{label: "Frequency", unit: "Hz", value: "100 Hz"},
			{label: "Gain", unit: "dB", value: "0.0 dB"},
		} {
			visibleControls = append(visibleControls, map[string]any{
				"label":             group + " " + control.label,
				"value_text":        control.value,
				"control_type":      "knob",
				"control_kind":      "continuous_parameter",
				"semantic_class":    strings.ToLower(control.label),
				"ui_salience":       "primary",
				"group":             group,
				"unit":              control.unit,
				"visible_range":     "visible display-domain hint",
				"is_parameter_like": true,
				"confidence":        0.92,
			})
			unitRows = append(unitRows, map[string]any{
				"label":           group + " " + control.label,
				"unit":            control.unit,
				"range_or_values": control.value,
				"group":           group,
				"confidence":      0.9,
			})
		}
	}
	imageDigest := map[string]any{
		"schema":                    "plugin_ui_reference_image_extract.v1",
		"visible_controls":          visibleControls,
		"groups":                    groupRows,
		"units_and_display_domains": unitRows,
		"confidence":                0.91,
	}

	var visionAttempts int
	var textAttempts int
	maxTextBodyBytes := 0
	candidateCounts := []int{}
	sourceCounts := []int{}
	writeResponsesText := func(w http.ResponseWriter, text string) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": []map[string]any{{
				"content": []map[string]any{{
					"type": "output_text",
					"text": text,
				}},
			}},
		})
	}
	writeChatText := func(w http.ResponseWriter, text string) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{"content": text},
			}},
		})
	}
	extractBatchInput := func(prompt string) map[string]any {
		startMarker := "Batch input:\n"
		endMarker := "\n\nOptional learning evidence context:"
		start := strings.Index(prompt, startMarker)
		if start < 0 {
			t.Fatalf("missing batch input marker in prompt: %s", prompt)
		}
		rest := prompt[start+len(startMarker):]
		end := strings.Index(rest, endMarker)
		if end < 0 {
			t.Fatalf("missing optional evidence marker in prompt: %s", prompt)
		}
		var batch map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(rest[:end])), &batch); err != nil {
			t.Fatalf("unmarshal batch input: %v\n%s", err, rest[:end])
		}
		return batch
	}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		switch r.URL.Path {
		case "/v1/responses":
			visionAttempts++
			data, _ := json.Marshal(imageDigest)
			writeResponsesText(w, string(data))
		case "/v1/chat/completions":
			textAttempts++
			if len(body) > maxTextBodyBytes {
				maxTextBodyBytes = len(body)
			}
			var req struct {
				Messages []llm.Message `json:"messages"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("unmarshal chat request: %v\n%s", err, string(body))
			}
			var prompt strings.Builder
			for _, msg := range req.Messages {
				prompt.WriteString(msg.Content)
				prompt.WriteString("\n")
			}
			batch := extractBatchInput(prompt.String())
			backend := mapValue(batch["backend_parameter_candidates"])
			candidates := mapRowsValue(backend["candidate_parameters"])
			sourceCount := intNumber(backend["source_candidate_count"])
			candidateCounts = append(candidateCounts, len(candidates))
			sourceCounts = append(sourceCounts, sourceCount)
			if len(candidates) == 0 {
				t.Fatalf("batch has no backend candidates: %+v", batch)
			}
			if len(candidates) > 36 {
				t.Fatalf("batch candidate_count = %d, want <= 36; batch=%+v", len(candidates), batch)
			}
			if sourceCount != len(params) {
				t.Fatalf("source_candidate_count = %d, want %d", sourceCount, len(params))
			}
			visible := mapRowsValue(batch["visible_controls"])
			visibleLabel := "visible control"
			visibleGroup := ""
			visibleUnit := "unknown"
			if len(visible) > 0 {
				visibleLabel = firstNonEmptyText(visible[0], "label")
				visibleGroup = firstNonEmptyText(visible[0], "group")
				visibleUnit = firstNonEmptyText(visible[0], "unit")
			}
			candidate := candidates[0]
			match := map[string]any{
				"schema": "plugin_ui_reference_match.v1",
				"candidate_parameter_matches": []map[string]any{{
					"visible_label":      visibleLabel,
					"visible_group":      visibleGroup,
					"visible_unit":       visibleUnit,
					"display_domain":     "batch-visible display-domain hint",
					"backend_param_id":   firstNonEmptyText(candidate, "param_id"),
					"backend_param_name": firstNonEmptyText(candidate, "name"),
					"match_reason":       "Batch candidate shares visible group/label/unit evidence.",
					"evidence":           []string{"ui_label", "backend_name", "display_probe"},
					"confidence":         0.82,
				}},
				"confidence": 0.8,
			}
			data, _ := json.Marshal(match)
			writeChatText(w, string(data))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer api.Close()

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: api.Client()}
	imagePath := filepath.Join(t.TempDir(), "ui.png")
	if err := os.WriteFile(imagePath, []byte("image bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, warning, err := server.buildPluginUIReferenceDigest(context.Background(), config.EngineConfig{
		BaseURL:      api.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default", Model: "vision-model"},
		},
	}, pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Large Plugin"}, []artifacts.Artifact{{
		Kind: "image",
		MIME: "image/png",
		Path: imagePath,
	}}, pluginParameterDigest{
		ParameterCount: len(params),
		QuickControls: []pluginQuickControlDigest{{
			ParamID:      "p_01_01",
			Label:        "Module 1 Frequency",
			DisplayGroup: "Module 1",
		}},
		Parameters: params,
	})
	if err != nil {
		t.Fatalf("buildPluginUIReferenceDigest error: %v", err)
	}
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if visionAttempts != 1 {
		t.Fatalf("visionAttempts = %d, want 1", visionAttempts)
	}
	if textAttempts <= 1 {
		t.Fatalf("textAttempts = %d, want multiple matching batches", textAttempts)
	}
	if maxTextBodyBytes >= 90_000 {
		t.Fatalf("largest text matching request body = %d bytes, want < 90000", maxTextBodyBytes)
	}
	for i := range candidateCounts {
		if candidateCounts[i] >= sourceCounts[i] {
			t.Fatalf("batch %d candidate_count=%d source_candidate_count=%d; expected a compact candidate subset", i, candidateCounts[i], sourceCounts[i])
		}
	}
	if got := firstNonEmptyText(digest, "matching_mode"); got != "batched" {
		t.Fatalf("matching_mode = %q digest=%+v", got, digest)
	}
	if intNumber(digest["batch_count"]) != textAttempts || len(mapRowsValue(digest["match_batch_summaries"])) != textAttempts {
		t.Fatalf("batch metadata mismatch: textAttempts=%d digest=%+v", textAttempts, digest)
	}
	if audit := mapValue(digest["parameter_coverage_audit"]); intNumber(audit["backend_param_total"]) != len(params) || intNumber(audit["dropped_param_count"]) != 0 {
		t.Fatalf("parameter coverage audit = %+v", audit)
	}
	if len(mapRowsValue(digest["candidate_parameter_matches"])) == 0 {
		t.Fatalf("candidate matches missing from batched digest: %+v", digest)
	}
}

func TestBuildPluginUIReferenceDigestKeepsLargeTargetedVisionProbeCompact(t *testing.T) {
	params := make([]pluginParameterInfo, 0, 72)
	for groupIndex := 0; groupIndex < 6; groupIndex++ {
		group := fmt.Sprintf("Module %d", groupIndex+1)
		for _, role := range []string{"Frequency", "Gain", "Amount", "Time", "Mix", "Mode", "Threshold", "Ratio", "Attack", "Release", "Enable", "Shape"} {
			id := fmt.Sprintf("p_%02d_%s", groupIndex+1, strings.ToLower(role))
			params = append(params, pluginParameterInfo{
				ID:               id,
				Name:             group + " " + role,
				DisplayGroup:     group,
				NormalizedRole:   strings.ToLower(role),
				ControlRelevance: "used_in_component",
				HostControllable: true,
				DisplayProbe: &plugingrabber.ParameterDisplayProbe{
					CurrentText: role,
					Label:       role,
				},
			})
		}
	}

	var visionAttempts int
	var textAttempts int
	maxVisionBodyBytes := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		switch r.URL.Path {
		case "/v1/responses":
			visionAttempts++
			if len(body) > maxVisionBodyBytes {
				maxVisionBodyBytes = len(body)
			}
			bodyText := string(body)
			if !strings.Contains(bodyText, "plugin_ui_reference_visual_target_focus.v1") {
				t.Fatalf("vision prompt missing target focus: %s", bodyText)
			}
			if strings.Count(bodyText, "\"param_id\"") >= len(params) {
				t.Fatalf("vision prompt appears to include the full parameter ledger")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"{\"schema\":\"plugin_ui_reference_target_probe.v1\",\"target_results\":[{\"target_id\":\"target_01\",\"control\":\"Module 1 Frequency\",\"status\":\"visible\",\"visible_label\":\"Module 1 Frequency\",\"visible_group\":\"Module 1\",\"visible_unit\":\"Hz\",\"display_domain\":\"100 Hz\",\"backend_candidate_param_ids\":[\"p_01_frequency\"],\"likely_backend_param_id\":\"p_01_frequency\",\"confidence\":0.9}],\"candidate_parameter_matches\":[{\"visible_label\":\"Module 1 Frequency\",\"visible_group\":\"Module 1\",\"visible_unit\":\"Hz\",\"display_domain\":\"100 Hz\",\"backend_param_id\":\"p_01_frequency\",\"backend_param_name\":\"Module 1 Frequency\",\"match_reason\":\"Targeted candidate shares visible group/label/unit evidence.\",\"evidence\":[\"ui_label\",\"backend_name\",\"display_probe\"],\"confidence\":0.86}],\"confidence\":0.86}"}]}]}`))
		case "/v1/chat/completions":
			textAttempts++
			t.Fatalf("targeted visual probe should not call text matching: %s", string(body))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer api.Close()

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: api.Client()}
	imagePath := filepath.Join(t.TempDir(), "ui.png")
	if err := os.WriteFile(imagePath, []byte("image bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, warning, err := server.buildPluginUIReferenceDigest(context.Background(), config.EngineConfig{
		BaseURL:      api.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default", Model: "vision-model"},
		},
	}, pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Large Plugin"}, []artifacts.Artifact{{
		Kind: "image",
		MIME: "image/png",
		Path: imagePath,
	}}, pluginParameterDigest{
		ParameterCount: len(params),
		QuickControls: []pluginQuickControlDigest{{
			ParamID:      "p_01_frequency",
			Label:        "Module 1 Frequency",
			DisplayGroup: "Module 1",
		}},
		Parameters: params,
	})
	if err != nil {
		t.Fatalf("buildPluginUIReferenceDigest error: %v", err)
	}
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if visionAttempts != 1 || textAttempts != 0 {
		t.Fatalf("visionAttempts=%d textAttempts=%d", visionAttempts, textAttempts)
	}
	if maxVisionBodyBytes >= 80_000 {
		t.Fatalf("vision request body = %d bytes, want < 80000", maxVisionBodyBytes)
	}
	if got := firstNonEmptyText(digest, "visual_matching_mode"); got != "type_guided_single_pass" {
		t.Fatalf("visual_matching_mode = %q digest=%+v", got, digest)
	}
	if len(mapRowsValue(digest["candidate_parameter_matches"])) == 0 {
		t.Fatalf("candidate matches missing from targeted digest: %+v", digest)
	}
}

func TestBuildPluginUIReferenceDigestRetriesTransientVisionExtractionError(t *testing.T) {
	visionAttempts := 0
	textAttempts := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			visionAttempts++
			if visionAttempts == 1 {
				http.Error(w, "temporary stream error: stream ID 1; INTERNAL_ERROR", http.StatusGatewayTimeout)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"{\"schema\":\"plugin_ui_reference_image_extract.v1\",\"visible_controls\":[{\"label\":\"GAIN 0.0 dB\",\"control_type\":\"knob\",\"group\":\"Band 1\",\"unit\":\"dB\",\"confidence\":0.94}],\"candidate_parameter_matches\":[{\"visible_label\":\"GAIN 0.0 dB\",\"visible_group\":\"Band 1\",\"visible_unit\":\"dB\",\"backend_param_id\":\"p_gain\",\"backend_param_name\":\"Band 1 Gain\",\"match_reason\":\"Visible gain readout matches backend gain control.\",\"evidence\":[\"ui_label\",\"backend_name\"],\"confidence\":0.9}],\"confidence\":0.92}"}]}]}`))
		case "/v1/chat/completions":
			textAttempts++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"schema\":\"plugin_ui_reference_match.v1\",\"candidate_parameter_matches\":[{\"visible_label\":\"GAIN 0.0 dB\",\"visible_group\":\"Band 1\",\"visible_unit\":\"dB\",\"backend_param_id\":\"p_gain\",\"backend_param_name\":\"Band 1 Gain\",\"match_reason\":\"Visible gain readout matches backend gain control.\",\"evidence\":[\"ui_label\",\"backend_name\"],\"confidence\":0.93}],\"confidence\":0.91}"}}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer api.Close()

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: api.Client()}
	imagePath := filepath.Join(t.TempDir(), "ui.png")
	if err := os.WriteFile(imagePath, []byte("image bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, warning, err := server.buildPluginUIReferenceDigest(context.Background(), config.EngineConfig{
		BaseURL:      api.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default", Model: "vision-model"},
		},
	}, pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Example Plugin"}, []artifacts.Artifact{{
		Kind: "image",
		MIME: "image/png",
		Path: imagePath,
	}}, pluginParameterDigest{
		ParameterCount: 1,
		Parameters: []pluginParameterInfo{{
			ID:               "p_gain",
			Name:             "Band 1 Gain",
			DisplayGroup:     "Band 1",
			ControlRelevance: "used_in_component",
			HostControllable: true,
		}},
	})
	if err != nil {
		t.Fatalf("buildPluginUIReferenceDigest error: %v", err)
	}
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if visionAttempts != 2 {
		t.Fatalf("visionAttempts = %d, want 2", visionAttempts)
	}
	if textAttempts != 0 {
		t.Fatalf("textAttempts = %d, want 0", textAttempts)
	}
	if got := firstNonEmptyText(digest, "matching_status"); got != "target_probe_empty" {
		t.Fatalf("matching_status = %q digest=%+v", got, digest)
	}
}

func TestBuildPluginUIReferenceDigestDoesNotCallTextMatchingAfterTargetProbe(t *testing.T) {
	visionAttempts := 0
	textAttempts := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			visionAttempts++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"{\"schema\":\"plugin_ui_reference_image_extract.v1\",\"visible_controls\":[{\"label\":\"GAIN 0.0 dB\",\"control_type\":\"knob\",\"group\":\"Band 1\",\"unit\":\"dB\",\"confidence\":0.94}],\"confidence\":0.92}"}]}]}`))
		case "/v1/chat/completions":
			textAttempts++
			if textAttempts == 1 {
				http.Error(w, "temporary 524 upstream timeout", http.StatusGatewayTimeout)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"schema\":\"plugin_ui_reference_match.v1\",\"candidate_parameter_matches\":[{\"visible_label\":\"GAIN 0.0 dB\",\"visible_group\":\"Band 1\",\"visible_unit\":\"dB\",\"backend_param_id\":\"p_gain\",\"backend_param_name\":\"Band 1 Gain\",\"match_reason\":\"Visible gain readout matches backend gain control.\",\"evidence\":[\"ui_label\",\"backend_name\"],\"confidence\":0.93}],\"confidence\":0.91}"}}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer api.Close()

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: api.Client()}
	imagePath := filepath.Join(t.TempDir(), "ui.png")
	if err := os.WriteFile(imagePath, []byte("image bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, warning, err := server.buildPluginUIReferenceDigest(context.Background(), config.EngineConfig{
		BaseURL:      api.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default", Model: "vision-model"},
		},
	}, pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Example Plugin"}, []artifacts.Artifact{{
		Kind: "image",
		MIME: "image/png",
		Path: imagePath,
	}}, pluginParameterDigest{
		ParameterCount: 1,
		Parameters: []pluginParameterInfo{{
			ID:               "p_gain",
			Name:             "Band 1 Gain",
			DisplayGroup:     "Band 1",
			ControlRelevance: "used_in_component",
			HostControllable: true,
		}},
	})
	if err != nil {
		t.Fatalf("buildPluginUIReferenceDigest error: %v", err)
	}
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if visionAttempts != 1 {
		t.Fatalf("visionAttempts = %d, want 1", visionAttempts)
	}
	if textAttempts != 0 {
		t.Fatalf("textAttempts = %d, want 0", textAttempts)
	}
	if got := firstNonEmptyText(digest, "matching_status"); got != "target_probe_empty" {
		t.Fatalf("matching_status = %q digest=%+v", got, digest)
	}
}

func TestBuildPluginUIReferenceDigestSkipsLegacyEmptyTextMatchingRetry(t *testing.T) {
	visionAttempts := 0
	textAttempts := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			visionAttempts++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"{\"schema\":\"plugin_ui_reference_image_extract.v1\",\"visible_controls\":[{\"label\":\"GAIN 0.0 dB\",\"control_type\":\"knob\",\"group\":\"Band 1\",\"unit\":\"dB\",\"confidence\":0.94}],\"confidence\":0.92}"}]}]}`))
		case "/v1/chat/completions":
			textAttempts++
			w.Header().Set("Content-Type", "application/json")
			if textAttempts == 1 {
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"schema\":\"plugin_ui_reference_match.v1\",\"candidate_parameter_matches\":[{\"visible_label\":\"GAIN 0.0 dB\",\"visible_group\":\"Band 1\",\"visible_unit\":\"dB\",\"backend_param_id\":\"p_gain\",\"backend_param_name\":\"Band 1 Gain\",\"match_reason\":\"Visible gain readout matches backend gain control.\",\"evidence\":[\"ui_label\",\"backend_name\"],\"confidence\":0.93}],\"confidence\":0.91}"}}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer api.Close()

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: api.Client()}
	imagePath := filepath.Join(t.TempDir(), "ui.png")
	if err := os.WriteFile(imagePath, []byte("image bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, warning, err := server.buildPluginUIReferenceDigest(context.Background(), config.EngineConfig{
		BaseURL:      api.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default", Model: "vision-model"},
		},
	}, pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Example Plugin"}, []artifacts.Artifact{{
		Kind: "image",
		MIME: "image/png",
		Path: imagePath,
	}}, pluginParameterDigest{
		ParameterCount: 1,
		Parameters: []pluginParameterInfo{{
			ID:               "p_gain",
			Name:             "Band 1 Gain",
			DisplayGroup:     "Band 1",
			ControlRelevance: "used_in_component",
			HostControllable: true,
		}},
	})
	if err != nil {
		t.Fatalf("buildPluginUIReferenceDigest error: %v", err)
	}
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if visionAttempts != 1 {
		t.Fatalf("visionAttempts = %d, want 1", visionAttempts)
	}
	if textAttempts != 0 {
		t.Fatalf("textAttempts = %d, want 0", textAttempts)
	}
	if got := firstNonEmptyText(digest, "matching_status"); got != "target_probe_empty" {
		t.Fatalf("matching_status = %q digest=%+v", got, digest)
	}
}

func TestResolvePluginUIReferenceVisionParseFailureRecordsPreviewWarning(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"I can see a visible GAIN readout, but this is not JSON."}]}]}`))
	}))
	defer api.Close()

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: api.Client()}
	server.artifactRoot = t.TempDir()
	sessionID := "pl_session_parse_failure"
	startedAt := time.Now().UTC()
	target := pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Example EQ"}
	imagePath := filepath.Join(t.TempDir(), "ui.png")
	if err := os.WriteFile(imagePath, []byte("image bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := artifacts.New(startedAt.Add(time.Second))
	a.ID = "art_ui_ref_parse_failure"
	a.Kind = "image"
	a.MIME = "image/png"
	a.Path = imagePath
	a.Metadata = map[string]any{
		"plugin_learning_session_id": sessionID,
		"plugin_learning_purpose":    pluginLearningUIReferencePurpose,
		"track_id":                   target.TrackID,
		"plugin_id":                  target.PluginID,
	}
	if _, err := server.artifactStore().Upsert(a); err != nil {
		t.Fatal(err)
	}
	uiReference, summaries, err := server.resolvePluginUIReference(context.Background(), config.EngineConfig{
		BaseURL:      api.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default", Model: "vision-model"},
		},
	}, map[string]any{
		"ui_reference_decision":              "provided",
		"ui_reference_artifact_ids":          []string{"art_ui_ref_parse_failure"},
		"plugin_learning_session_id":         sessionID,
		"plugin_learning_session_started_at": startedAt.Format(time.RFC3339Nano),
	}, target, pluginParameterDigest{})
	if err != nil {
		t.Fatalf("resolvePluginUIReference error: %v", err)
	}
	if got := firstNonEmptyText(uiReference, "status"); got != "vision_failed" {
		t.Fatalf("status = %q uiReference=%+v", got, uiReference)
	}
	warnings := strings.Join(stringListValue(uiReference["warnings"]), "\n")
	if !strings.Contains(warnings, "visible GAIN readout") || !strings.Contains(warnings, "response_preview") {
		t.Fatalf("warnings missing response preview: %q", warnings)
	}
	if len(summaries) != 1 || summaries[0].ID != "art_ui_ref_parse_failure" {
		t.Fatalf("summaries = %+v", summaries)
	}
}

func TestParsePluginUIReferenceJSONUsesFirstCompleteObject(t *testing.T) {
	raw := "```json\n{\"schema\":\"plugin_ui_reference_image_extract.v1\",\"visible_controls\":[{\"label\":\"GAIN\"}]}\n```\nExtra note with {not json}."
	digest, err := parsePluginUIReferenceImageExtract(raw)
	if err != nil {
		t.Fatalf("parsePluginUIReferenceImageExtract error: %v", err)
	}
	controls := mapRowsValue(digest["visible_controls"])
	if len(controls) != 1 || firstNonEmptyText(controls[0], "label") != "GAIN" {
		t.Fatalf("visible controls = %+v digest=%+v", controls, digest)
	}
}

func TestPluginUIReferenceFocusSummaryKeepsFullBackendCandidateLedger(t *testing.T) {
	params := make([]pluginParameterInfo, 0, 135)
	for i := 0; i < 135; i++ {
		unit := "dB"
		role := "gain"
		if i%3 == 0 {
			unit = "Hz"
			role = "frequency"
		}
		params = append(params, pluginParameterInfo{
			ID:               fmt.Sprintf("p_%03d", i),
			Name:             fmt.Sprintf("Band %d %s", i+1, role),
			DisplayGroup:     fmt.Sprintf("Band %d", (i%8)+1),
			NormalizedRole:   role,
			ControlRelevance: "used_in_component",
			HostControllable: true,
			Unit:             unit,
		})
	}
	digest := pluginParameterDigest{
		ParameterCount: len(params),
		QuickControls: []pluginQuickControlDigest{{
			ParamID:        "p_000",
			Label:          "Band 1 Frequency",
			DisplayGroup:   "Band 1",
			NormalizedRole: "frequency",
		}},
		Parameters: params,
	}

	focus := pluginUIReferenceVisualFocusSummary(digest)
	if firstNonEmptyText(focus, "schema") != "plugin_ui_reference_visual_focus_summary.v1" || focus["parameters_retained"] != true {
		t.Fatalf("focus summary = %+v", focus)
	}
	if intNumber(focus["source_parameter_count"]) != len(params) || intNumber(focus["dropped_param_count"]) != 0 {
		t.Fatalf("focus summary lost parameter coverage: %+v", focus)
	}
	if len(mapRowsValue(focus["visual_search_terms"])) == 0 || len(mapRowsValue(focus["group_summary"])) == 0 {
		t.Fatalf("focus summary missing search terms/groups: %+v", focus)
	}

	candidates := pluginUIReferenceBackendCandidateDigest(digest)
	rows := mapRowsValue(candidates["candidate_parameters"])
	if len(rows) != len(params) || candidates["parameters_retained"] != true || candidates["candidates_truncated"] == true {
		t.Fatalf("backend candidate ledger should be complete, got count=%d digest=%+v", len(rows), candidates)
	}

	audit := pluginUIReferenceCoverageAudit(digest, focus, map[string]any{
		"matching_status": "matched",
		"visible_controls": []map[string]any{{
			"label": "Band 1 Frequency",
		}},
		"candidate_parameter_matches": []map[string]any{{
			"backend_param_id": "p_000",
		}},
	})
	if intNumber(audit["backend_param_total"]) != len(params) || intNumber(audit["dropped_param_count"]) != 0 || intNumber(audit["matched_param_count"]) != 1 {
		t.Fatalf("coverage audit = %+v", audit)
	}
}

func TestApplyPluginUIReferenceEvidencePreservesParamIDs(t *testing.T) {
	patch := pluginProfilePatch{
		Groups: []map[string]any{{
			"id": "main",
			"params": map[string]any{
				"gain": "p_gain",
			},
		}},
	}
	next := applyPluginUIReferenceEvidence(patch, map[string]any{
		"plugin_learning_session_id": "pl_session_test",
		"artifact_ids":               []string{"art_ui_ref"},
		"visual_digest": map[string]any{
			"schema": "plugin_ui_reference_digest.v1",
		},
	})
	params := mapValue(next.Groups[0]["params"])
	mapping := mapValue(params["gain"])
	if got := firstNonEmptyText(mapping, "param_id"); got != "p_gain" {
		t.Fatalf("param_id = %q mapping=%+v", got, mapping)
	}
	found := false
	for _, row := range mapRowsValue(mapping["evidence"]) {
		if firstNonEmptyText(row, "kind") == "plugin_ui_reference_image" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing plugin_ui_reference_image evidence: %+v", mapping["evidence"])
	}
}

func TestApplyPluginUIReferenceEvidenceEnhancesDeterministicMappings(t *testing.T) {
	patch := pluginProfilePatch{
		Aliases:       map[string]string{"2": "B1 Gain"},
		DisplayGroups: map[string]string{"2": "EQ"},
		Groups: []map[string]any{{
			"id":    "b1",
			"role":  "eq_band",
			"label": "B1",
			"params": map[string]any{
				"gain": map[string]any{
					"param_id": "2",
					"label":    "B1 Gain",
					"source":   "auto_learn_eq_band_pattern",
					"display_domain": map[string]any{
						"text":       "-18~18 dB",
						"confidence": 0.9,
						"source":     "display_probe_inferred",
					},
					"display_domain_text": "-18~18 dB",
				},
			},
		}},
	}
	uiReference := map[string]any{
		"plugin_learning_session_id": "pl_session_test",
		"artifact_ids":               []string{"art_ui_ref"},
		"visual_digest": map[string]any{
			"schema":          "plugin_ui_reference_digest.v1",
			"matching_status": "matched",
			"candidate_parameter_matches": []map[string]any{{
				"visible_label":      "Band 1 Gain",
				"visible_group":      "Low Band",
				"visible_unit":       "dB",
				"display_domain":     "-24~24 dB",
				"backend_param_id":   "2",
				"backend_param_name": "B1 Gain",
				"match_reason":       "Visible Band 1 Gain control matches backend B1 Gain.",
				"evidence":           []string{"ui_label", "backend_name"},
				"confidence":         0.95,
			}},
		},
	}
	next := applyPluginUIReferenceEvidence(patch, uiReference)
	params := mapValue(next.Groups[0]["params"])
	mapping := mapValue(params["gain"])
	if got := firstNonEmptyText(mapping, "param_id"); got != "2" {
		t.Fatalf("param_id changed to %q mapping=%+v", got, mapping)
	}
	if got := firstNonEmptyText(mapping, "visual_label"); got != "Band 1 Gain" {
		t.Fatalf("visual_label = %q mapping=%+v", got, mapping)
	}
	if got := firstNonEmptyText(mapping, "label"); got != "Band 1 Gain" {
		t.Fatalf("label = %q mapping=%+v", got, mapping)
	}
	if got := next.Aliases["2"]; got != "Band 1 Gain" {
		t.Fatalf("alias = %q aliases=%+v", got, next.Aliases)
	}
	if got := next.DisplayGroups["2"]; got != "Low Band" {
		t.Fatalf("display group = %q groups=%+v", got, next.DisplayGroups)
	}
	if got := firstNonEmptyText(mapping, "display_domain_text"); got != "-18~18 dB" {
		t.Fatalf("high-confidence host display domain should remain primary, got %q mapping=%+v", got, mapping)
	}
	if got := firstNonEmptyText(mapping, "visual_display_domain_text"); got != "-24~24 dB" {
		t.Fatalf("visual display domain = %q mapping=%+v", got, mapping)
	}
	if rows := mapRowsValue(uiReference["visual_matches"]); len(rows) != 1 || firstNonEmptyText(rows[0], "param_id") != "2" {
		t.Fatalf("visual matches = %+v", uiReference["visual_matches"])
	}
}

func TestApplyPluginUIReferenceEvidenceUsesCandidateParameterMatches(t *testing.T) {
	patch := pluginProfilePatch{
		Groups: []map[string]any{{
			"id":    "b1",
			"label": "B1",
			"params": map[string]any{
				"gain": map[string]any{
					"param_id": "p_gain",
					"label":    "Internal Gain",
					"source":   "auto_learn_eq_band_pattern",
				},
			},
		}},
	}
	uiReference := map[string]any{
		"plugin_learning_session_id": "pl_session_test",
		"artifact_ids":               []string{"art_ui_ref"},
		"visual_digest": map[string]any{
			"schema":          "plugin_ui_reference_digest.v1",
			"matching_status": "matched",
			"candidate_parameter_matches": []map[string]any{{
				"visible_label":      "GAIN",
				"visible_group":      "Band I",
				"visible_unit":       "dB",
				"display_domain":     "-24~24 dB",
				"backend_param_id":   "p_gain",
				"backend_param_name": "Band 1 Gain",
				"match_reason":       "Visible GAIN knob in Band I matches backend Band 1 gain.",
				"evidence":           []string{"ui_label", "backend_name"},
				"confidence":         0.93,
			}},
		},
	}
	next := applyPluginUIReferenceEvidence(patch, uiReference)
	mapping := mapValue(mapValue(next.Groups[0]["params"])["gain"])
	if got := firstNonEmptyText(mapping, "param_id"); got != "p_gain" {
		t.Fatalf("param_id = %q mapping=%+v", got, mapping)
	}
	if got := firstNonEmptyText(mapping, "visual_backend_param_id"); got != "p_gain" {
		t.Fatalf("visual backend id = %q mapping=%+v", got, mapping)
	}
	if got := firstNonEmptyText(mapping, "visual_label"); got != "GAIN" {
		t.Fatalf("visual_label = %q mapping=%+v", got, mapping)
	}
	if got := firstNonEmptyText(mapping, "visual_match_reason"); !strings.Contains(got, "Band 1 gain") {
		t.Fatalf("visual_match_reason = %q mapping=%+v", got, mapping)
	}
	rows := mapRowsValue(uiReference["visual_matches"])
	if len(rows) != 1 || firstNonEmptyText(rows[0], "param_id") != "p_gain" || firstNonEmptyText(rows[0], "backend_name") != "Band 1 Gain" {
		t.Fatalf("visual matches = %+v", rows)
	}
}

func TestApplyPluginUIReferenceEvidenceDoesNotRewriteMappingsWhenMatchingFailed(t *testing.T) {
	patch := pluginProfilePatch{
		Aliases:       map[string]string{"p_gain": "B2 Gain"},
		DisplayGroups: map[string]string{"p_gain": "B2"},
		Groups: []map[string]any{{
			"id":    "b2",
			"label": "B2",
			"params": map[string]any{
				"gain": map[string]any{
					"param_id":            "p_gain",
					"label":               "B2 Gain",
					"display_domain_text": "-18~18 dB",
				},
			},
		}},
	}
	uiReference := map[string]any{
		"plugin_learning_session_id": "pl_session_test",
		"artifact_ids":               []string{"art_ui_ref"},
		"visual_digest": map[string]any{
			"schema":          "plugin_ui_reference_digest.v1",
			"matching_status": "match_failed",
			"visible_controls": []map[string]any{{
				"label":          "BAND I",
				"group":          "BAND I",
				"display_domain": "-24~24 dB",
				"confidence":     0.99,
			}},
		},
	}
	next := applyPluginUIReferenceEvidence(patch, uiReference)
	mapping := mapValue(mapValue(next.Groups[0]["params"])["gain"])
	if got := firstNonEmptyText(mapping, "label"); got != "B2 Gain" {
		t.Fatalf("label should not be rewritten, got %q mapping=%+v", got, mapping)
	}
	if got := firstNonEmptyText(mapping, "visual_label"); got != "" {
		t.Fatalf("visual_label should not be applied on match_failed, got %q mapping=%+v", got, mapping)
	}
	if got := next.DisplayGroups["p_gain"]; got != "B2" {
		t.Fatalf("display group should not be rewritten, got %q groups=%+v", got, next.DisplayGroups)
	}
	if got := firstNonEmptyText(mapping, "display_domain_text"); got != "-18~18 dB" {
		t.Fatalf("display_domain_text should not be rewritten, got %q mapping=%+v", got, mapping)
	}
	foundEvidence := false
	for _, row := range mapRowsValue(mapping["evidence"]) {
		if firstNonEmptyText(row, "kind") == "plugin_ui_reference_image" {
			foundEvidence = true
		}
	}
	if !foundEvidence {
		t.Fatalf("expected weak visual provenance evidence, mapping=%+v", mapping)
	}
	if rows := mapRowsValue(uiReference["visual_matches"]); len(rows) != 0 {
		t.Fatalf("visual_matches should not be produced on match_failed: %+v", rows)
	}
}

func TestApplyPluginUIReferenceEvidenceKeepsUnboundVisibleControlsWeak(t *testing.T) {
	patch := pluginProfilePatch{
		Aliases:       map[string]string{"p_gain": "Gain"},
		DisplayGroups: map[string]string{"p_gain": "Main"},
		Groups: []map[string]any{{
			"id":    "main",
			"label": "Main",
			"params": map[string]any{
				"gain": map[string]any{
					"param_id":            "p_gain",
					"label":               "Gain",
					"display_domain_text": "-18~18 dB",
				},
				"frequency": map[string]any{
					"param_id": "p_freq",
					"label":    "Frequency",
				},
			},
		}},
	}
	uiReference := map[string]any{
		"plugin_learning_session_id": "pl_session_test",
		"artifact_ids":               []string{"art_ui_ref"},
		"visual_digest": map[string]any{
			"schema":          "plugin_ui_reference_digest.v1",
			"matching_status": "matched",
			"visible_controls": []map[string]any{{
				"label":        "ON",
				"group":        "Band",
				"control_type": "toggle",
				"unit":         "toggle",
				"confidence":   0.99,
			}},
			"groups": []map[string]any{{
				"label":      "ON",
				"confidence": 0.99,
			}},
		},
	}
	next := applyPluginUIReferenceEvidence(patch, uiReference)
	params := mapValue(next.Groups[0]["params"])
	gain := mapValue(params["gain"])
	freq := mapValue(params["frequency"])
	if got := firstNonEmptyText(gain, "label"); got != "Gain" {
		t.Fatalf("gain label should not be rewritten by unbound visible control, got %q mapping=%+v", got, gain)
	}
	if got := firstNonEmptyText(freq, "label"); got != "Frequency" {
		t.Fatalf("frequency label should not be rewritten by unbound visible control, got %q mapping=%+v", got, freq)
	}
	if got := next.DisplayGroups["p_gain"]; got != "Main" {
		t.Fatalf("display group should not be rewritten by unbound visible control, got %q groups=%+v", got, next.DisplayGroups)
	}
	if rows := mapRowsValue(uiReference["visual_matches"]); len(rows) != 0 {
		t.Fatalf("unbound visible controls must not produce visual matches: %+v", rows)
	}
	for _, mapping := range []map[string]any{gain, freq} {
		foundEvidence := false
		for _, row := range mapRowsValue(mapping["evidence"]) {
			if firstNonEmptyText(row, "kind") == "plugin_ui_reference_image" {
				foundEvidence = true
			}
		}
		if !foundEvidence {
			t.Fatalf("expected weak visual evidence on mapping: %+v", mapping)
		}
	}
}

func TestApplyPluginUIReferenceEvidenceKeepsImageGuidedMatchesWeakWhenTextMatchFails(t *testing.T) {
	patch := pluginProfilePatch{
		Groups: []map[string]any{{
			"id":    "b1",
			"label": "B1",
			"params": map[string]any{
				"gain": map[string]any{
					"param_id": "p_gain",
					"label":    "Internal Gain",
				},
			},
		}},
	}
	uiReference := map[string]any{
		"plugin_learning_session_id": "pl_session_test",
		"artifact_ids":               []string{"art_ui_ref"},
		"visual_digest": map[string]any{
			"schema":          "plugin_ui_reference_digest.v1",
			"matching_status": "image_guided_match_failed",
			"candidate_parameter_matches": []map[string]any{{
				"visible_label":      "GAIN 0.0 dB",
				"visible_group":      "Band 1",
				"visible_unit":       "dB",
				"display_domain":     "-24 to +24 dB",
				"backend_param_id":   "p_gain",
				"backend_param_name": "Band 1 Gain",
				"match_reason":       "Image-guided visual evidence matched the gain readout to the backend gain parameter.",
				"evidence":           []string{"ui_label", "backend_name", "display_probe"},
				"confidence":         0.9,
			}},
		},
	}
	next := applyPluginUIReferenceEvidence(patch, uiReference)
	mapping := mapValue(mapValue(next.Groups[0]["params"])["gain"])
	if got := firstNonEmptyText(mapping, "visual_backend_param_id"); got != "" {
		t.Fatalf("image-only visual backend id should not be applied, got %q mapping=%+v", got, mapping)
	}
	if got := firstNonEmptyText(mapping, "visual_label"); got != "" {
		t.Fatalf("image-only visual label should not be applied, got %q mapping=%+v", got, mapping)
	}
	foundEvidence := false
	for _, row := range mapRowsValue(mapping["evidence"]) {
		if firstNonEmptyText(row, "kind") == "plugin_ui_reference_image" {
			foundEvidence = true
		}
	}
	if !foundEvidence {
		t.Fatalf("expected weak visual provenance evidence, mapping=%+v", mapping)
	}
	if rows := mapRowsValue(uiReference["visual_matches"]); len(rows) != 0 {
		t.Fatalf("visual matches should not be produced when text matching failed: %+v", rows)
	}
}

func TestApplyPluginUIReferenceEvidenceIgnoresUnknownCandidateParamID(t *testing.T) {
	patch := pluginProfilePatch{
		Groups: []map[string]any{{
			"id":    "b1",
			"label": "B1",
			"params": map[string]any{
				"gain": map[string]any{
					"param_id": "p_gain",
					"label":    "Band 1 Gain",
					"source":   "auto_learn_eq_band_pattern",
				},
			},
		}},
	}
	uiReference := map[string]any{
		"visual_digest": map[string]any{
			"schema":          "plugin_ui_reference_digest.v1",
			"matching_status": "matched",
			"candidate_parameter_matches": []map[string]any{{
				"visible_label":      "Band 1 Gain",
				"visible_group":      "Band I",
				"display_domain":     "-24~24 dB",
				"backend_param_id":   "not_a_real_param",
				"backend_param_name": "Forged Gain",
				"confidence":         0.99,
			}},
		},
	}
	next := applyPluginUIReferenceEvidence(patch, uiReference)
	mapping := mapValue(mapValue(next.Groups[0]["params"])["gain"])
	if got := firstNonEmptyText(mapping, "visual_label"); got != "" {
		t.Fatalf("unknown param id should not visually match real parameter, visual_label=%q mapping=%+v", got, mapping)
	}
	if rows := mapRowsValue(uiReference["visual_matches"]); len(rows) != 0 {
		t.Fatalf("unexpected visual matches for unknown param id: %+v", rows)
	}
}

func TestArtifactHTTPRenameAndDelete(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.artifactRoot = t.TempDir()
	projectPath := filepath.Join(t.TempDir(), "song.vit")
	a := artifacts.New(time.Now())
	a.ID = "art_rename_delete"
	a.Kind = "image"
	a.Title = "Old Name"
	a = artifacts.ApplyScope(a, artifacts.ScopeFromMap(map[string]any{"project_path": projectPath}))
	if _, err := server.artifactStore().Upsert(a); err != nil {
		t.Fatal(err)
	}

	renameReq := httptest.NewRequest(http.MethodPatch, "/agent/artifact?id=art_rename_delete", bytes.NewBufferString(`{"title":"Mood Board"}`))
	renameRec := httptest.NewRecorder()
	server.handleArtifact(renameRec, renameReq)
	if renameRec.Code != http.StatusOK || !strings.Contains(renameRec.Body.String(), "Mood Board") {
		t.Fatalf("rename status=%d body=%s", renameRec.Code, renameRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/agent/artifacts?limit=10&project_path="+url.QueryEscape(projectPath), nil)
	listRec := httptest.NewRecorder()
	server.handleArtifacts(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), "Mood Board") {
		t.Fatalf("list after rename status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/agent/artifact?id=art_rename_delete", nil)
	deleteRec := httptest.NewRecorder()
	server.handleArtifact(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	listAfterDeleteRec := httptest.NewRecorder()
	server.handleArtifacts(listAfterDeleteRec, listReq)
	if listAfterDeleteRec.Code != http.StatusOK || strings.Contains(listAfterDeleteRec.Body.String(), "art_rename_delete") {
		t.Fatalf("list after delete status=%d body=%s", listAfterDeleteRec.Code, listAfterDeleteRec.Body.String())
	}

	readReq := httptest.NewRequest(http.MethodGet, "/agent/artifact?id=art_rename_delete", nil)
	readRec := httptest.NewRecorder()
	server.handleArtifact(readRec, readReq)
	if readRec.Code != http.StatusNotFound {
		t.Fatalf("read after delete status=%d body=%s", readRec.Code, readRec.Body.String())
	}
}

func TestUIStateFiltersArtifactsByProjectScope(t *testing.T) {
	root := t.TempDir()
	projectA := filepath.Join(root, "song_a.vit")
	projectB := filepath.Join(root, "song_b.vit")
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectA,
	})
	server := New(nil, shadowProject, nil)
	server.artifactRoot = t.TempDir()
	store := server.artifactStore()

	a := artifacts.New(time.Now())
	a.ID = "art_project_a"
	a.Kind = "text"
	a.Title = "Project A"
	a = artifacts.ApplyScope(a, artifacts.ScopeFromMap(map[string]any{
		"project_path":      projectA,
		"root_project_path": projectA,
		"active_worktree":   "main",
		"active_branch":     "main",
	}))
	if _, err := store.Upsert(a); err != nil {
		t.Fatal(err)
	}
	b := artifacts.New(time.Now())
	b.ID = "art_project_b"
	b.Kind = "text"
	b.Title = "Project B"
	b = artifacts.ApplyScope(b, artifacts.ScopeFromMap(map[string]any{
		"project_path":      projectB,
		"root_project_path": projectB,
		"active_worktree":   "main",
		"active_branch":     "main",
	}))
	if _, err := store.Upsert(b); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/agent/ui/state", nil)
	rec := httptest.NewRecorder()
	server.handleUIState(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ui state status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	rows, ok := body["artifacts"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("artifacts = %+v", body["artifacts"])
	}
	row := rows[0].(map[string]any)
	if row["id"] != "art_project_a" || row["id"] == "art_project_b" {
		t.Fatalf("filtered artifact = %+v", row)
	}
}

func TestFindWebUIRootFromAgentBin(t *testing.T) {
	root := t.TempDir()
	dist := filepath.Join(root, "agent", "webui", "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "agent", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findWebUIRootFrom(bin); got != filepath.Clean(dist) {
		t.Fatalf("findWebUIRootFrom = %q, want %q", got, filepath.Clean(dist))
	}
}

func TestChatArtifactRefsReturnArtifactSummaries(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.artifactRoot = t.TempDir()
	filePath := filepath.Join(t.TempDir(), "ref.txt")
	if err := os.WriteFile(filePath, []byte("referenced artifact text"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := artifacts.Extract(artifacts.ArtifactFromFile(filePath, "art_ref", "chat_ref", "", ""), artifacts.DefaultTextLimit)
	if _, err := server.artifactStore().Upsert(a); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(ChatRequest{
		ConversationID: "chat_ref",
		Message:        "/tools",
		ArtifactRefs:   []string{"art_ref"},
	})
	rec := httptest.NewRecorder()
	server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Artifacts) != 1 || resp.Artifacts[0].ID != "art_ref" {
		t.Fatalf("artifacts = %+v", resp.Artifacts)
	}
}

func TestDialogueMediaReferencesPromoteReplyFileNamesToArtifacts(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.artifactRoot = t.TempDir()
	assetDir := filepath.Join(t.TempDir(), "LLM md", "参赛内容")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	intro := filepath.Join(assetDir, "宣讲视频.mp4")
	demo := filepath.Join(assetDir, "演示视频.mp4")
	if err := os.WriteFile(intro, []byte("intro"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(demo, []byte("demo"), 0o644); err != nil {
		t.Fatal(err)
	}
	userText := fmt.Sprintf("%q 能给我推荐一个这个文件夹下的视频吗？", assetDir)
	reply := "我找到了两个视频：\n1. **宣讲视频.mp4**（约18MB）\n2. **演示视频.mp4**（约294MB）"

	rows := server.artifactsFromDialogueMediaReferences(userText, reply, "chat_media_refs", "goal_media", "run_media", artifacts.Scope{})
	if len(rows) != 2 {
		t.Fatalf("artifacts = %+v", rows)
	}
	byTitle := map[string]artifacts.Summary{}
	for _, row := range rows {
		byTitle[row.Title] = row
		if row.Kind != "video" || row.Status != "ready" {
			t.Fatalf("artifact summary = %+v", row)
		}
	}
	if byTitle["宣讲视频.mp4"].Path != intro || byTitle["演示视频.mp4"].Path != demo {
		t.Fatalf("artifacts by title = %+v", byTitle)
	}
}

func TestChatResponsePromotesExecutedArtifacts(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp := server.chatResponseFromAgentLoopResult("chat_media", agentModeDefault, agentloop.Result{
		Status: agentruntime.StatusCompleted,
		Reply:  "ok",
		Executed: []map[string]any{
			{
				"tool": "media.index_authorized_folder",
				"result": map[string]any{
					"status": "ok",
					"artifacts": []artifacts.Summary{
						{ID: "art_media_loop", Kind: "audio", Title: "loop.wav", Status: "ready"},
					},
				},
			},
		},
	})
	if len(resp.Artifacts) != 1 || resp.Artifacts[0].ID != "art_media_loop" {
		t.Fatalf("artifacts = %+v", resp.Artifacts)
	}
}

func TestLegacyChatPathWritesChatTelemetrySource(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("LLM path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"{\"reply\":\"legacy ok\",\"commands\":[]}"}}],
			"usage":{"prompt_tokens":11,"completion_tokens":3}
		}`))
	}))
	defer api.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfgPath := filepath.Join(home, ".vit", "config.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(fmt.Sprintf(`{"baseUrl":%q,"apiKey":"test","defaultModel":"test-model"}`, api.URL))
	if err := os.WriteFile(cfgPath, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	telemetryPath := filepath.Join(t.TempDir(), "agent_llm_telemetry.jsonl")
	t.Setenv("VIT_AGENT_LLM_TELEMETRY_PATH", telemetryPath)

	server := New(nil, shadow.New(nil), nil)
	body, _ := json.Marshal(ChatRequest{
		ConversationID: "chat_legacy",
		Message:        "hello legacy",
		Context:        map[string]any{"use_legacy_chat": true, "goal_id": nil, "run_id": nil},
	})
	rec := httptest.NewRecorder()
	server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Reply != "legacy ok" || resp.ConversationID != "chat_legacy" {
		t.Fatalf("response = %+v", resp)
	}
	data, err := os.ReadFile(telemetryPath)
	if err != nil {
		t.Fatalf("read telemetry: %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &record); err != nil {
		t.Fatalf("decode telemetry: %v data=%s", err, string(data))
	}
	if record["source"] != "chat" || record["conversation_id"] != "chat_legacy" {
		t.Fatalf("telemetry identity = %+v", record)
	}
	if record["goal_id"] == "" || record["goal_id"] == "<nil>" {
		t.Fatalf("telemetry goal_id not cleaned: %+v", record)
	}
	if record["prompt_fingerprint"] == "" || record["section_stats"] == nil {
		t.Fatalf("telemetry missing prompt runtime metadata: %+v", record)
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

func TestLegacyChatBroadMixRequestDoesNotQueuePluginLoad(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp, handled := server.chatResponseForCommands(context.Background(), "chat_test", "我想你帮我对这段音频进行缩混可以吗", "", []map[string]any{
		{"cmd": "rack_add_node", "track_id": "1007", "plugin_path": `C:\Program Files\Common Files\VST3\TDR Nova.vst3`},
	}, map[string]any{"selected_track_id": "1007"})
	if !handled {
		t.Fatal("expected broad mix guard response")
	}
	if resp.NeedsConfirmation || resp.PlanID != "" {
		t.Fatalf("broad mix request should not queue plugin load confirmation: %+v", resp)
	}
	if !strings.Contains(resp.Reply, "mix.request_observation") {
		t.Fatalf("reply should direct observation first: %q", resp.Reply)
	}
	if len(server.pending) != 0 {
		t.Fatalf("pending plan should not be stored: %+v", server.pending)
	}
}

func TestLegacyPendingBroadMixConfirmationDoesNotExecutePluginLoad(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := PendingPlan{
		ID:        "plan_broad_mix",
		CreatedAt: time.Now(),
		Context: map[string]any{
			"user_message":      "我想你帮我对这段音频进行缩混可以吗",
			"selected_track_id": "1007",
		},
		Decisions: policy.Analyze([]map[string]any{
			{"cmd": "rack_add_node", "track_id": "1007", "plugin_path": `C:\Program Files\Common Files\VST3\TDR Nova.vst3`},
		}),
	}
	server.pending[plan.ID] = plan

	status, response := server.resolvePendingPlanDecision(context.Background(), plan.ID, "approve")
	if status != http.StatusOK {
		t.Fatalf("status = %d response=%+v", status, response)
	}
	if response["blocked"] != true {
		t.Fatalf("confirmation should be blocked, response=%+v", response)
	}
	if !strings.Contains(fmt.Sprint(response["message"]), "mix.request_observation") {
		t.Fatalf("response should direct observation first: %+v", response)
	}
	if _, ok := server.pending[plan.ID]; ok {
		t.Fatal("pending plan should be consumed after blocked confirmation")
	}
}

func TestMixTreatmentPreparationPluginLoadConfirmationBypassesBroadMixGuard(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := PendingPlan{
		ID:        "plan_mix_treatment_preparation",
		CreatedAt: time.Now(),
		Workflow:  pluginGrabberLoadCommand,
		Context: map[string]any{
			"user_message":                   "低频有点糊，帮我处理一下",
			"mix_treatment_preparation":      true,
			"mix_treatment_preparation_plan": map[string]any{"schema_version": "mix_treatment_preparation.v0"},
		},
		WorkflowData: map[string]any{
			"conversation_id":                "chat_mix",
			"plugin_query":                   "eq",
			"user_message":                   "低频有点糊，帮我处理一下",
			"mix_treatment_preparation":      true,
			"mix_treatment_preparation_plan": map[string]any{"schema_version": "mix_treatment_preparation.v0"},
		},
		Decisions: policy.Analyze([]map[string]any{
			{"cmd": "rack_add_node", "track_id": "1007", "plugin_path": `C:\Program Files\Common Files\VST3\TDR Nova.vst3`},
		}),
	}

	if pendingPlanIsMixTreatmentPreparation(plan) != true {
		t.Fatalf("plan should be recognized as mix treatment preparation: %+v", plan)
	}
	if response, blocked := server.legacyPendingPlanBroadMixBlockedConfirmResponse(context.Background(), plan.ID, plan, "goal_mix", "run_mix", agentModeDefault, ""); blocked {
		t.Fatalf("mix treatment preparation should not be blocked as legacy broad mix, response=%+v", response)
	}
}

func TestQualifiedSemanticPluginSelectionBypassesBroadMixGuardOnlyForExactCandidate(t *testing.T) {
	path := `C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3`
	identifier := "VST3-Pro-Q-3"
	plan := PendingPlan{
		ID: "plan_semantic_selection", Workflow: pluginGrabberLoadCommand,
		Context: map[string]any{
			"user_message": "减少一些浑浊", "selected_track_id": "1007",
			"semantic_plugin_recommendation_selection": true,
			"semantic_plugin_recommendation_candidate": map[string]any{
				"plugin_path": path, "identifier": identifier, "name": "Pro-Q 3",
			},
		},
		Decisions: policy.Analyze([]map[string]any{{
			"cmd": "rack_add_node", "track_id": "1007", "plugin_path": path, "plugin_identifier": identifier,
		}}),
	}
	if !pendingPlanHasQualifiedSemanticPluginSelection(plan) {
		t.Fatalf("exact semantic selection was not qualified: %#v", plan)
	}
	server := New(nil, shadow.New(nil), nil)
	if response, blocked := server.legacyPendingPlanBroadMixBlockedConfirmResponse(context.Background(), plan.ID, plan, "goal", "run", agentModeDefault, ""); blocked {
		t.Fatalf("qualified target-3 selection was blocked: %#v", response)
	}
	tampered := plan
	tampered.Decisions = policy.Analyze([]map[string]any{{
		"cmd": "rack_add_node", "track_id": "1007", "plugin_path": `C:\VST3\Other.vst3`, "plugin_identifier": identifier,
	}})
	if pendingPlanHasQualifiedSemanticPluginSelection(tampered) {
		t.Fatal("mismatched client/load candidate bypassed broad-mix guard")
	}
}

func TestRecordGoalResultStoresPendingMixTickWithoutDeadlock(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	done := make(chan struct{})
	go func() {
		server.recordGoalResult("chat_mix", agentloop.Result{
			GoalID: "goal_1",
			RunID:  "run_1",
			ExecutionMemory: agentloop.ExecutionMemory{
				PendingMixTickCandidate: &agentloop.PendingMixTickCandidate{
					Operation:     "track_gain_adjust",
					TrackID:       "1010",
					DeltaDB:       -1,
					ObservationID: "obs_1",
					Status:        "pending_confirmation",
				},
			},
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recordGoalResult deadlocked while storing pending mix tick")
	}
	if candidate, ok := server.pendingMixTickForConversation("chat_mix"); !ok || candidate.TrackID != "1010" {
		t.Fatalf("pending candidate = %+v ok=%v", candidate, ok)
	}
	events, _ := server.agentEventsSince("chat_mix", 0, 10)
	if len(events) != 1 || events[0].Type != "mix_tick.pending" {
		t.Fatalf("events = %+v", events)
	}
}

func TestRecordGoalResultUpsertsPendingManagerFromLegacyMixTreatment(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.recordGoalResult("chat_mix", agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &agentloop.MixTreatmentPending{
				SchemaVersion:    "mix_treatment_pending.v0",
				Status:           "pending_confirmation",
				ConversationID:   "chat_mix",
				ObservationID:    "obs_1",
				Intent:           "reduce low-mid mud",
				TargetRef:        "track:1007",
				ActionKind:       "plugin_treatment",
				ProcessorType:    "eq",
				PluginID:         "plugin_eq",
				PluginName:       "Test EQ",
				ReasoningSummary: "low-mid buildup around the vocal",
			},
		},
	})

	if legacy, ok := server.pendingMixTreatmentForConversation("chat_mix"); !ok || legacy.TargetRef != "track:1007" {
		t.Fatalf("legacy pending treatment = %+v ok=%v", legacy, ok)
	}
	active := server.pendingManager.ActiveForConversation("chat_mix")
	if len(active) != 1 {
		t.Fatalf("active pending manager rows = %+v", active)
	}
	pending := active[0]
	if pending.CandidateType != "mix_treatment" || pending.Status != agentprotocol.PendingStatusWaitingUser || pending.TargetRef != "track:1007" {
		t.Fatalf("pending manager candidate = %+v", pending)
	}
	server.transitionActivePendingCandidate("chat_mix", "mix_treatment", agentprotocol.PendingStatusAccepted, "user approved")
	accepted, ok := server.pendingManager.Get(pending.ID)
	if !ok || accepted.Status != agentprotocol.PendingStatusAccepted || accepted.Source.Metadata["transition_reason"] != "user approved" {
		t.Fatalf("accepted pending = %+v ok=%v", accepted, ok)
	}
	server.transitionActivePendingCandidate("chat_mix", "mix_treatment", agentprotocol.PendingStatusCommitted, "applied")
	if rows := server.pendingManager.ActiveForConversation("chat_mix"); len(rows) != 0 {
		t.Fatalf("committed pending should not remain active: %+v", rows)
	}
}

func TestPendingMixTickExplicitConfirmationExecutesTypedLoop(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation:                 "track_gain_adjust",
		TrackID:                   "track_1",
		DeltaDB:                   -1,
		ObservationID:             "obs_1",
		Evidence:                  map[string]any{"matched_text": "降低当前轨道 1 dB"},
		ExpiresAfterContextChange: true,
		Status:                    "pending_confirmation",
		Fingerprint:               map[string]any{"target_scope": "selected_track", "mix_session_id": "mix_1"},
	}
	server.shadow.Initialize(map[string]any{"tracks": []any{map[string]any{
		"track_id":       "track_1",
		"track_name":     "Vocal",
		"track_type":     "hybrid",
		"is_audio_track": true,
		"volume_db":      -3,
	}}})

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{
		Message: "可以执行",
		Context: map[string]any{"conversation_id": "chat_mix"},
	}, agentModeDefault)

	if !handled || resp.StopReason != "mix_tick_confirmation_failed" {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	var tools []string
	for _, row := range resp.ExecutedKernelReply {
		tools = append(tools, cleanContextText(row["tool"]))
	}
	if !testStringSliceContains(tools, "mix.propose_tick") || !testStringSliceContains(tools, "mix.apply_tick") {
		t.Fatalf("executed tools = %+v, want typed propose/apply attempt", tools)
	}
	if testStringSliceContains(tools, "daw.invoke") || testStringSliceContains(tools, "track.volume") {
		t.Fatalf("confirmation bypassed typed tools: %+v", tools)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; !ok {
		t.Fatal("failed apply should keep pending mix tick available for retry")
	}
}

func TestPendingMixTickApprovalPhraseClassification(t *testing.T) {
	for _, msg := range []string{
		"\u53ef\u4ee5\uff0c\u7ee7\u7eed",
		"\u53ef\u4ee5, \u7ee7\u7eed",
		"\u53ef\u4ee5\u7ee7\u7eed",
	} {
		if !messageExplicitMixTickApply(msg) {
			t.Fatalf("messageExplicitMixTickApply(%q) = false, want true", msg)
		}
	}
	if messageExplicitMixTickApply("\u53ef\u4ee5") {
		t.Fatal("messageExplicitMixTickApply(\"can\") = true, want ambiguous")
	}
	if !messageAmbiguousMixTickApproval("\u53ef\u4ee5") {
		t.Fatal("messageAmbiguousMixTickApproval(\"can\") = false, want true")
	}
}

func TestPlainApprovalWithoutPendingDoesNotRunAgentLoop(t *testing.T) {
	for _, message := range []string{"\u662f\u7684", "\u53ef\u4ee5"} {
		t.Run(message, func(t *testing.T) {
			server := New(nil, shadow.New(nil), nil)
			body, _ := json.Marshal(ChatRequest{
				ConversationID: "chat_plain_approval",
				Message:        message,
				Context:        map[string]any{"agent_mode": agentModeDefault},
			})

			rec := httptest.NewRecorder()
			server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			var resp ChatResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
			}
			if resp.StopReason != "plain_approval_without_pending_confirmation" {
				t.Fatalf("resp = %+v", resp)
			}
			if len(resp.ExecutedKernelReply) != 0 || resp.NeedsConfirmation {
				t.Fatalf("plain approval without pending should not execute or ask confirmation: %+v", resp)
			}
		})
	}
}

func TestPendingMixTreatmentRevisionFallsThroughAndExpiresOldPending(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingTreatments["chat_mix"] = agentloop.MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            "chat_mix",
		ObservationID:             "obs_1",
		Intent:                    "move guitar left a little",
		TargetRef:                 "track:1007",
		ActionKind:                "pan_balance",
		ProcessorType:             "utility",
		DeltaPan:                  -0.1,
		ExpiresAfterContextChange: true,
	}

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", ChatRequest{Message: "\u6539\u6210\u5de6 70%"}, agentModeDefault)

	if handled {
		t.Fatalf("revision should fall through to normal chat, resp=%+v", resp)
	}
	if _, ok := server.pendingTreatments["chat_mix"]; ok {
		t.Fatal("old pending treatment should expire before revised proposal is generated")
	}
}

func TestClipFadeGainRequestExpiresPendingMixStateBeforeChatHandlers(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingTreatments["chat_mix"] = agentloop.MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            "chat_mix",
		ObservationID:             "obs_1",
		Intent:                    "bring the selected track down slightly",
		TargetRef:                 "track:1007",
		ActionKind:                "gain_balance",
		ProcessorType:             "utility",
		DeltaDB:                   -1,
		ExpiresAfterContextChange: true,
	}
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation:                 "track_gain_adjust",
		TrackID:                   "1007",
		DeltaDB:                   -1,
		Status:                    "pending_confirmation",
		ExpiresAfterContextChange: true,
	}
	server.upsertPendingCandidate(server.pendingTreatments["chat_mix"].ToPendingCandidate("chat_mix", "goal_mix", "run_mix", "now"))
	server.upsertPendingCandidate(server.pendingMixTicks["chat_mix"].ToPendingCandidate("chat_mix", "goal_mix", "run_mix", "now"))
	server.storePendingInteraction(AgentInteractionRequest{
		ID:             "interaction_mix_treatment",
		Kind:           "mix_treatment_confirmation",
		Type:           "mix_treatment_confirmation",
		Workflow:       "mix_treatment",
		ConversationID: "chat_mix",
		GoalID:         "goal_mix",
		RunID:          "run_mix",
	}, map[string]any{})
	server.storePendingInteraction(AgentInteractionRequest{
		ID:             "interaction_mix_tick",
		Kind:           "mix_tick_confirmation",
		Type:           "mix_tick_confirmation",
		Workflow:       "mix_tick",
		ConversationID: "chat_mix",
		GoalID:         "goal_mix",
		RunID:          "run_mix",
	}, map[string]any{})

	cleared := server.clearPendingMixForClipFadeGainRequest("chat_mix", "\u8bfb\u53d6\u5f53\u524d\u9009\u4e2d clip \u7684 fade \u548c gain \u72b6\u6001")

	if !cleared {
		t.Fatal("clip fade/gain request should clear stale pending mix state")
	}
	if _, ok := server.pendingTreatments["chat_mix"]; ok {
		t.Fatal("pending mix treatment should be removed")
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; ok {
		t.Fatal("pending mix tick should be removed")
	}
	if active := server.pendingManager.ActiveForConversation("chat_mix"); len(active) != 0 {
		t.Fatalf("pending manager candidates should become inactive: %+v", active)
	}
	if _, ok := server.takePendingInteraction("interaction_mix_treatment"); ok {
		t.Fatal("pending mix treatment interaction should be expired")
	}
	if _, ok := server.takePendingInteraction("interaction_mix_tick"); ok {
		t.Fatal("pending mix tick interaction should be expired")
	}
	clipRequest := "\u8bfb\u53d6\u5f53\u524d\u9009\u4e2d clip \u7684 fade \u548c gain \u72b6\u6001"
	if resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", ChatRequest{Message: clipRequest}, agentModeDefault); handled {
		t.Fatalf("cleared pending treatment should not handle followup, resp=%+v", resp)
	}
	if resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: clipRequest}, agentModeDefault); handled {
		t.Fatalf("cleared pending tick should not handle followup, resp=%+v", resp)
	}
}

func TestPendingMixTreatmentPanRevisionPhraseFallsThroughAndExpiresOldPending(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingTreatments["chat_mix"] = agentloop.MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            "chat_mix",
		ObservationID:             "obs_1",
		Intent:                    "move current track left",
		TargetRef:                 "track:1007",
		ActionKind:                "pan_balance",
		ProcessorType:             "utility",
		DeltaPan:                  -0.1,
		ExpiresAfterContextChange: true,
	}

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", ChatRequest{Message: "彻底摆到右边去"}, agentModeDefault)

	if handled {
		t.Fatalf("pan revision should fall through to generate a new pending treatment, resp=%+v", resp)
	}
	if _, ok := server.pendingTreatments["chat_mix"]; ok {
		t.Fatal("old pending treatment should expire before revised pan proposal is generated")
	}
}

func TestPendingMixPlanPanRevisionPhraseExpiresGenericConfirmation(t *testing.T) {
	plan := PendingPlan{
		ID:       "plan_mix",
		Workflow: agentLoopConfirmationWorkflow,
		GoalContinuation: &agentloop.Continuation{
			PendingToolCall: &planner.ToolCall{
				Tool: "mix.apply_tick",
				Args: map[string]any{"tick_id": "mix_tick_1"},
			},
		},
	}

	if !pendingPlanCanBeRevisedByMixRequest(plan, "彻底往右") {
		t.Fatal("pan revision should expire a generic mix confirmation plan")
	}
	if pendingPlanPlainApproval("彻底往右") {
		t.Fatal("pan revision phrase must not be treated as plain approval")
	}
}

func TestPendingTrackPanPlanPanRevisionPhraseExpiresGenericConfirmation(t *testing.T) {
	plan := PendingPlan{
		ID:      "plan_track_pan",
		Context: map[string]any{"conversation_id": "chat_mix"},
		GoalContinuation: &agentloop.Continuation{
			PendingToolCall: &planner.ToolCall{
				Tool: "track.pan",
				Args: map[string]any{"track_id": "1007", "pan": -1.0},
			},
		},
		Decisions: []policy.Decision{{
			Name:    "set_pan",
			Command: map[string]any{"cmd": "set_pan", "track_id": "1007", "pan": -1.0},
			Risk:    policy.RiskUndoable,
		}},
	}

	if !pendingPlanCanBeRevisedByMixRequest(plan, "\u5f7b\u5e95\u5f80\u53f3") {
		t.Fatal("pan revision should expire a legacy track.pan confirmation plan")
	}
	if pendingPlanCanBeRevisedByMixRequest(plan, "\u53ef\u4ee5") {
		t.Fatal("plain approval should not revise the pending track.pan confirmation")
	}
}

func TestPendingMixTickRevisionFallsThroughAndExpiresOldPending(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation: "track_pan_adjust",
		TrackID:   "track_1",
		DeltaPan:  -0.1,
		Status:    "pending_confirmation",
	}

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "\u6539\u6210\u5de6 70%"}, agentModeDefault)

	if handled {
		t.Fatalf("revision should fall through to normal chat, resp=%+v", resp)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; ok {
		t.Fatal("old pending mix tick should expire before revised proposal is generated")
	}
}

func TestPendingMixTickRejectExpiresWithoutExecuting(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.storePendingMixTickCandidate("chat_mix", "goal_1", "run_1", agentloop.PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       "track_1",
		DeltaDB:       -1,
		ObservationID: "obs_1",
		Status:        "pending_confirmation",
	})

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "\u4e0d\u8981"}, agentModeDefault)

	if !handled || resp.StopReason != "mix_tick_rejected" {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	if len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("reject should not execute: %+v", resp.ExecutedKernelReply)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; ok {
		t.Fatal("rejected pending mix tick should expire")
	}
	if active := server.pendingManager.ActiveForConversation("chat_mix"); len(active) != 0 {
		t.Fatalf("rejected pending mix tick should not remain active: %+v", active)
	}
}

func TestPendingMixTreatmentRejectExpiresWithoutExecuting(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.recordGoalResult("chat_mix", agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &agentloop.MixTreatmentPending{
				SchemaVersion:    "mix_treatment_pending.v0",
				Status:           "pending_confirmation",
				ConversationID:   "chat_mix",
				ObservationID:    "obs_1",
				Intent:           "reduce low-mid mud",
				TargetRef:        "track:1007",
				ActionKind:       "plugin_treatment",
				ProcessorType:    "eq",
				PluginID:         "plugin_eq",
				PluginName:       "Test EQ",
				ReasoningSummary: "low-mid buildup around the vocal",
			},
		},
	})

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", ChatRequest{Message: "\u4e0d\u8981"}, agentModeDefault)

	if !handled || resp.StopReason != "mix_treatment_rejected" {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	if len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("reject should not execute: %+v", resp.ExecutedKernelReply)
	}
	if _, ok := server.pendingTreatments["chat_mix"]; ok {
		t.Fatal("rejected pending treatment should expire")
	}
	if active := server.pendingManager.ActiveForConversation("chat_mix"); len(active) != 0 {
		t.Fatalf("rejected pending treatment should not remain active: %+v", active)
	}
}

func TestPendingMixTreatmentConfirmationReturnsResolverPrep(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingTreatments["chat_mix"] = agentloop.MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            "chat_mix",
		ObservationID:             "obs_1",
		Intent:                    "reduce mud",
		TargetRef:                 "track:1007",
		ActionKind:                "plugin_treatment",
		ProcessorType:             "eq",
		ReasoningSummary:          "needs EQ-style treatment",
		Confidence:                "medium",
		NeedsResolution:           []string{"plugin_instance", "plugin_profile", "exact_control"},
		ExpiresAfterContextChange: true,
	}

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", ChatRequest{Message: "可以，继续"}, agentModeDefault)

	if !handled {
		t.Fatal("pending mix treatment was not handled")
	}
	if resp.StopReason != "mix_treatment_preparation_started" {
		t.Fatalf("resp=%+v", resp)
	}
	if !resp.NeedsConfirmation || resp.Workflow != pluginGrabberLoadCommand {
		t.Fatalf("resolver should start a safe load confirmation, resp=%+v", resp)
	}
	if len(resp.ExecutedKernelReply) != 0 || len(resp.Commands) == 0 {
		t.Fatalf("resolver should prepare confirmation without executing tools, commands=%+v executed=%+v", resp.Commands, resp.ExecutedKernelReply)
	}
	if _, ok := server.pendingTreatments["chat_mix"]; ok {
		t.Fatal("pending treatment should be expired after resolver decision")
	}
	events, _ := server.agentEventsSince("chat_mix", 0, 10)
	if len(events) != 1 || events[0].Type != "mix_treatment.resolver_decision" {
		t.Fatalf("events = %+v", events)
	}
	if events[0].Title != "混音建议需要准备" || strings.Contains(events[0].Body, "plugin treatment") || strings.Contains(events[0].Body, "resolver") {
		t.Fatalf("resolver event was not localized: %+v", events[0])
	}
	plan := mapValue(events[0].Payload["preparation_plan"])
	if cleanContextText(plan["schema_version"]) != "mix_treatment_preparation.v0" {
		t.Fatalf("preparation plan = %+v", events[0].Payload)
	}
	if !testAnyStringSliceContains(plan["safe_route"], "plugin_grabber.apply_control") {
		t.Fatalf("safe route missing apply_control: %+v", plan)
	}
	if !testAnyStringSliceContains(plan["blocked_routes"], "plugin.set_parameter") || !testAnyStringSliceContains(plan["blocked_routes"], "daw.invoke") {
		t.Fatalf("blocked routes missing raw plugin mutation guards: %+v", plan)
	}
	next := mapValue(plan["next_command"])
	if cleanContextText(next["cmd"]) != pluginGrabberLoadCommand {
		t.Fatalf("unknown plugin instance should produce safe load prep command: %+v", plan)
	}
	if cleanContextText(next["track_id"]) != "1007" || cleanContextText(next["plugin_query"]) != "eq" {
		t.Fatalf("load prep target = %+v", next)
	}
}

func TestPendingMixTreatmentNeedsPreparationStartsPluginLoadConfirmation(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "mix.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
		}},
	})
	kernel := &recordingChatKernel{}
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	server.pendingTreatments["chat_mix"] = agentloop.MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            "chat_mix",
		ObservationID:             "obs_1",
		Intent:                    "当前轨道响度有些小，我想更大声点",
		TargetRef:                 "track:1007",
		ActionKind:                "plugin_treatment",
		ProcessorType:             "compressor",
		PluginID:                  `C:\VST3\Test Compressor.vst3`,
		PluginName:                "Test Compressor",
		ReasoningSummary:          "direct gain risks clipping, prepare light compression or limiting",
		Confidence:                "medium",
		NeedsResolution:           []string{"plugin_instance", "plugin_profile", "exact_control"},
		ExpiresAfterContextChange: true,
	}

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", ChatRequest{
		ConversationID: "chat_mix",
		Message:        "确认执行",
		Context:        map[string]any{"conversation_id": "chat_mix", "project_path": projectPath},
	}, agentModeDefault)

	if !handled {
		t.Fatal("pending mix treatment was not handled")
	}
	if resp.StopReason != "mix_treatment_preparation_started" || resp.Workflow != pluginGrabberLoadCommand {
		t.Fatalf("resp=%+v", resp)
	}
	if !resp.NeedsConfirmation || resp.PlanID == "" || len(resp.InteractionRequests) != 1 {
		t.Fatalf("load preparation should be a confirmation interaction: %+v", resp)
	}
	if resp.InteractionRequests[0].Kind != "confirmation" {
		t.Fatalf("interaction = %+v", resp.InteractionRequests[0])
	}
	if len(resp.ExecutedKernelReply) != 0 || len(resp.Commands) == 0 {
		t.Fatalf("should only prepare plugin load confirmation; commands=%+v executed=%+v", resp.Commands, resp.ExecutedKernelReply)
	}
	if strings.Contains(resp.Reply, "I have") || strings.Contains(resp.Reply, "Resolver") || strings.Contains(resp.Reply, "Prep steps") {
		t.Fatalf("reply should be Chinese user-facing text: %q", resp.Reply)
	}
	if !strings.Contains(resp.Reply, "我还没有执行响度处理") || !strings.Contains(resp.Reply, "插件加载") {
		t.Fatalf("reply should explain safe preparation path: %q", resp.Reply)
	}
	if cleanContextText(resp.WorkflowData["plugin_query"]) != "compressor" || cleanContextText(resp.WorkflowData["track_id"]) != "1007" || cleanContextText(resp.WorkflowData["plugin_name"]) != "Test Compressor" {
		t.Fatalf("workflow data = %+v", resp.WorkflowData)
	}
	var loadCommands []map[string]any
	for _, cmd := range kernel.commands {
		switch cleanContextText(cmd["cmd"]) {
		case "plugin_search":
			// read-only inventory lookup is expected.
		case "rack_add_node", "set_plugin_param", "n_apply_control":
			loadCommands = append(loadCommands, cmd)
		}
	}
	if len(loadCommands) != 0 {
		t.Fatalf("should not load or write before the load confirmation is approved: %+v", kernel.commands)
	}
}

func TestResolveMixTreatmentReadyPluginControlRequiresProfileAndInstance(t *testing.T) {
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"plugins": []any{map[string]any{
				"plugin_id":   "nova_1",
				"plugin_name": "TDR Nova",
			}},
		}},
	})
	server := New(nil, shadowProject, nil)
	treatment := agentloop.MixTreatmentPending{
		SchemaVersion:   "mix_treatment_pending.v0",
		Status:          "pending_confirmation",
		TargetRef:       "track:1007",
		ActionKind:      "plugin_treatment",
		ProcessorType:   "eq",
		PluginID:        "nova_1",
		PluginName:      "TDR Nova",
		Control:         "control low mids with band 2",
		Target:          map[string]any{"freq_hz": 300.0, "gain_db": -1.5, "q": 1.1},
		Intent:          "reduce low-mid mud",
		NeedsResolution: []string{"plugin_instance", "plugin_profile", "exact_control"},
	}

	withoutProfile := server.resolveMixTreatment(context.Background(), treatment, map[string]any{})
	if withoutProfile.Status != "needs_preparation" {
		t.Fatalf("without profile decision = %+v", withoutProfile)
	}

	withProfile := server.resolveMixTreatment(context.Background(), treatment, map[string]any{
		"plugin_grabber_profiles": []any{map[string]any{
			"plugin_id":   "nova_1",
			"plugin_name": "TDR Nova",
			"class":       "eq",
			"virtual_controls": []any{map[string]any{
				"name":         "control low mids with band 2",
				"component_id": "band2",
				"resolver":     "local_profile_mapping",
			}},
		}},
	})
	if withProfile.Status != "ready_plugin_control" {
		t.Fatalf("with profile decision = %+v", withProfile)
	}
	if len(withProfile.Command) == 0 || cleanContextText(withProfile.Command["cmd"]) != "plugin_grabber_apply_control" {
		t.Fatalf("command = %+v", withProfile.Command)
	}
	if cleanContextText(withProfile.Command["track_id"]) != "1007" || cleanContextText(withProfile.Command["plugin_id"]) != "nova_1" {
		t.Fatalf("command target = %+v", withProfile.Command)
	}
	if cleanContextText(withProfile.Command["control"]) != "control low mids with band 2" {
		t.Fatalf("command control = %+v", withProfile.Command)
	}
	if _, ok := withProfile.Command["target"].(map[string]any); !ok {
		t.Fatalf("command target payload = %+v", withProfile.Command["target"])
	}
}

func TestResolveMixTreatmentPluginTreatmentPreparesLearningForExistingPlugin(t *testing.T) {
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"plugins": []any{map[string]any{
				"plugin_id":   "nova_1",
				"plugin_name": "TDR Nova",
			}},
		}},
	})
	server := New(nil, shadowProject, nil)
	decision := server.resolveMixTreatment(context.Background(), agentloop.MixTreatmentPending{
		SchemaVersion:   "mix_treatment_pending.v0",
		Status:          "pending_confirmation",
		TargetRef:       "track:1007",
		ActionKind:      "plugin_treatment",
		ProcessorType:   "eq",
		PluginID:        "nova_1",
		PluginName:      "TDR Nova",
		Intent:          "reduce low-mid mud",
		NeedsResolution: []string{"plugin_profile", "exact_control"},
	}, map[string]any{})

	if decision.Status != "needs_preparation" {
		t.Fatalf("decision = %+v", decision)
	}
	if len(decision.Command) != 0 {
		t.Fatalf("resolver should not produce write command: %+v", decision.Command)
	}
	if cleanContextText(decision.PrepCommand["cmd"]) != pluginGrabberLearnCommand {
		t.Fatalf("prep command = %+v", decision.PrepCommand)
	}
	if cleanContextText(decision.PrepCommand["track_id"]) != "1007" || cleanContextText(decision.PrepCommand["plugin_id"]) != "nova_1" {
		t.Fatalf("prep command target = %+v", decision.PrepCommand)
	}
	plan := decision.PreparationPlan
	if cleanContextText(plan["schema_version"]) != "mix_treatment_preparation.v0" {
		t.Fatalf("preparation plan = %+v", plan)
	}
	if !testAnyStringSliceContains(plan["safe_route"], "plugin_grabber.learn_project_profile") || !testAnyStringSliceContains(plan["safe_route"], "plugin_grabber.apply_control") {
		t.Fatalf("safe route = %+v", plan)
	}
	if !testAnyStringSliceContains(plan["blocked_routes"], "plugin.set_parameter") || !testAnyStringSliceContains(plan["blocked_routes"], "daw.invoke") {
		t.Fatalf("blocked routes = %+v", plan)
	}
	next := mapValue(plan["next_command"])
	if cleanContextText(next["cmd"]) != pluginGrabberLearnCommand || cleanContextText(next["plugin_id"]) != "nova_1" {
		t.Fatalf("next command = %+v plan=%+v", next, plan)
	}
	candidate := mapValue(plan["plugin_candidate"])
	if cleanContextText(candidate["track_id"]) != "1007" || cleanContextText(candidate["plugin_name"]) != "TDR Nova" {
		t.Fatalf("plugin candidate = %+v plan=%+v", candidate, plan)
	}
	if testStringSliceContains(decision.ToolRoute, "plugin.set_parameter") || testStringSliceContains(decision.ToolRoute, "daw.invoke") {
		t.Fatalf("unsafe route = %+v", decision.ToolRoute)
	}
}

func TestPendingMixTreatmentNeedsPreparationStartsPluginLearningCardOnly(t *testing.T) {
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"plugins": []any{map[string]any{
				"plugin_id":   "nova_1",
				"plugin_name": "TDR Nova",
			}},
		}},
	})
	server := New(nil, shadowProject, nil)
	server.pendingTreatments["chat_mix"] = agentloop.MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            "chat_mix",
		ObservationID:             "obs_1",
		Intent:                    "reduce low-mid mud",
		TargetRef:                 "track:1007",
		ActionKind:                "plugin_treatment",
		ProcessorType:             "eq",
		PluginID:                  "nova_1",
		PluginName:                "TDR Nova",
		ReasoningSummary:          "needs an EQ-style treatment",
		Confidence:                "medium",
		NeedsResolution:           []string{"plugin_profile", "exact_control"},
		ExpiresAfterContextChange: true,
	}

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", ChatRequest{
		ConversationID: "chat_mix",
		Message:        "可以，继续",
		Context:        map[string]any{"conversation_id": "chat_mix"},
	}, agentModeDefault)

	if !handled {
		t.Fatal("pending mix treatment was not handled")
	}
	if resp.StopReason != "mix_treatment_preparation_started" {
		t.Fatalf("resp=%+v", resp)
	}
	if len(resp.ExecutedKernelReply) != 0 || len(resp.Commands) != 0 {
		t.Fatalf("preparation card should not execute or confirm writes; commands=%+v executed=%+v", resp.Commands, resp.ExecutedKernelReply)
	}
	if resp.Workflow != "plugin_grabber_auto_learn" {
		t.Fatalf("workflow = %q resp=%+v", resp.Workflow, resp)
	}
	if cleanContextText(resp.PluginLearning["stage"]) != pluginLearningUIReferenceStage {
		t.Fatalf("plugin learning = %+v", resp.PluginLearning)
	}
	learningPlan := mapValue(resp.PluginLearning["mix_treatment_preparation_plan"])
	if cleanContextText(learningPlan["schema_version"]) != "mix_treatment_preparation.v0" {
		t.Fatalf("plugin learning missing preparation plan = %+v", resp.PluginLearning)
	}
	if !testAnyStringSliceContains(learningPlan["safe_route"], "plugin_grabber.apply_control") || !testAnyStringSliceContains(learningPlan["blocked_routes"], "plugin.set_parameter") {
		t.Fatalf("plugin learning preparation route contract = %+v", learningPlan)
	}
	if len(resp.InteractionRequests) != 1 || resp.InteractionRequests[0].Type != pluginLearningUIReferenceType {
		t.Fatalf("interactions = %+v", resp.InteractionRequests)
	}
	interactionPlan := mapValue(resp.InteractionRequests[0].Payload["mix_treatment_preparation_plan"])
	if cleanContextText(interactionPlan["schema_version"]) != "mix_treatment_preparation.v0" {
		t.Fatalf("interaction missing preparation plan = %+v", resp.InteractionRequests[0].Payload)
	}
	if _, ok := server.pendingTreatments["chat_mix"]; ok {
		t.Fatal("pending treatment should expire after resolver decision")
	}
	events, _ := server.agentEventsSince("chat_mix", 0, 10)
	if len(events) != 1 || events[0].Status != "needs_preparation" {
		t.Fatalf("events = %+v", events)
	}
	prep := mapValue(events[0].Payload["prep_command"])
	if cleanContextText(prep["cmd"]) != pluginGrabberLearnCommand {
		t.Fatalf("event prep command = %+v", events[0].Payload)
	}
	plan := mapValue(events[0].Payload["preparation_plan"])
	next := mapValue(plan["next_command"])
	if cleanContextText(plan["schema_version"]) != "mix_treatment_preparation.v0" || cleanContextText(next["cmd"]) != pluginGrabberLearnCommand {
		t.Fatalf("event preparation plan = %+v", events[0].Payload)
	}
	if !testAnyStringSliceContains(plan["blocked_routes"], "plugin.set_parameter") || !testAnyStringSliceContains(plan["safe_route"], "plugin_grabber.apply_control") {
		t.Fatalf("event preparation route contract = %+v", plan)
	}
}

func TestRecordGoalResultMixTreatmentPendingEventIncludesDeltaDB(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.recordGoalResult("chat_mix", agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &agentloop.MixTreatmentPending{
				SchemaVersion:   "mix_treatment_pending.v0",
				Status:          "pending_confirmation",
				ConversationID:  "chat_mix",
				ObservationID:   "obs_1",
				Intent:          "bring vocal down slightly",
				TargetRef:       "track:1007",
				ActionKind:      "gain_balance",
				ProcessorType:   "utility",
				DeltaDB:         -1.25,
				NeedsResolution: []string{},
			},
		},
	})

	events, _ := server.agentEventsSince("chat_mix", 0, 10)
	if len(events) != 1 || events[0].Type != "mix_treatment.pending" {
		t.Fatalf("events = %+v", events)
	}
	if fmt.Sprint(events[0].Payload["delta_db"]) != "-1.25" {
		t.Fatalf("payload = %+v", events[0].Payload)
	}
}

func TestPendingMixTreatmentReadyPluginControlExecutesApplyControlOnly(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "mix.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"plugins": []any{map[string]any{
				"plugin_id":   "nova_1",
				"plugin_name": "TDR Nova",
			}},
		}},
	})
	kernel := &recordingChatKernel{replies: []map[string]any{
		{"status": "ok", "project_path": projectPath, "snapshot_xml": "<project/>"},
		{
			"status":    "ok",
			"track_id":  "1007",
			"plugin_id": "nova_1",
			"applied_parameters": []any{map[string]any{
				"param_id":       "band2_gain",
				"new_value_text": "-1.5 dB",
			}},
		},
		{"status": "ok", "project_path": projectPath, "tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"plugins": []any{map[string]any{
				"plugin_id":   "nova_1",
				"plugin_name": "TDR Nova",
			}},
		}}},
	}}
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	server.pendingTreatments["chat_mix"] = agentloop.MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            "chat_mix",
		ObservationID:             "obs_1",
		Intent:                    "reduce low-mid mud",
		TargetRef:                 "track:1007",
		ActionKind:                "plugin_treatment",
		ProcessorType:             "eq",
		PluginID:                  "nova_1",
		PluginName:                "TDR Nova",
		Control:                   "control low mids with band 2",
		Target:                    map[string]any{"freq_hz": 300.0, "gain_db": -1.5, "q": 1.1},
		ReasoningSummary:          "explicit control from profile",
		Confidence:                "high",
		NeedsResolution:           []string{},
		ExpiresAfterContextChange: true,
	}
	req := ChatRequest{
		ConversationID: "chat_mix",
		Message:        "可以，继续",
		Context: map[string]any{
			"conversation_id": "chat_mix",
			"project_path":    projectPath,
			"plugin_grabber_profiles": []any{map[string]any{
				"plugin_id":   "nova_1",
				"plugin_name": "TDR Nova",
				"virtual_controls": []any{map[string]any{
					"name":         "control low mids with band 2",
					"component_id": "band2",
				}},
			}},
		},
	}

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", req, agentModeDefault)

	if !handled || resp.StopReason != "mix_treatment_applied_plugin_control_reobserved" {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	if resp.GoalStatus != "completed" || resp.CompletedSteps != 2 {
		t.Fatalf("goal status = %q steps=%d resp=%+v", resp.GoalStatus, resp.CompletedSteps, resp)
	}
	if len(resp.Commands) != 1 || resp.Commands[0].Name != "plugin_grabber_apply_control" {
		t.Fatalf("commands = %+v", resp.Commands)
	}
	if len(resp.ExecutedKernelReply) != 2 {
		t.Fatalf("executed = %+v", resp.ExecutedKernelReply)
	}
	executed := resp.ExecutedKernelReply[0]
	if cleanContextText(executed["tool"]) != "plugin_grabber.apply_control" || cleanContextText(executed["command_name"]) != "plugin_grabber_apply_control" {
		t.Fatalf("executed route = %+v", executed)
	}
	reobserve := resp.ExecutedKernelReply[1]
	if cleanContextText(reobserve["tool"]) != "mix.observe" || cleanContextText(reobserve["command_name"]) != "mix_observe" {
		t.Fatalf("reobserve route = %+v", reobserve)
	}
	if cleanContextText(mapValue(reobserve["result"])["observation_id"]) == "" {
		t.Fatalf("reobserve missing observation_id: %+v", reobserve)
	}
	if !strings.Contains(resp.Reply, "AB Result：不可信") || !strings.Contains(resp.Reply, "未把这次改动标记为已验证") {
		t.Fatalf("plugin treatment reply should surface compact AB state, got %q", resp.Reply)
	}
	if len(resp.ProjectResultCards) != 1 {
		t.Fatalf("plugin treatment project result cards should surface AB state: %+v", resp.ProjectResultCards)
	}
	abCard := mapValue(resp.ProjectResultCards[0]["ab_result"])
	if cleanContextText(abCard["reason"]) == "" {
		t.Fatalf("plugin treatment AB card = %+v", resp.ProjectResultCards)
	}
	if testStringSliceContains([]string{cleanContextText(executed["tool"]), cleanContextText(executed["command_name"])}, "daw.invoke") {
		t.Fatalf("execution used raw route: %+v", executed)
	}
	if _, ok := server.pendingTreatments["chat_mix"]; ok {
		t.Fatal("pending treatment should expire after ready execution")
	}
	events, _ := server.agentEventsSince("chat_mix", 0, 10)
	if len(events) != 1 || events[0].Type != "mix_treatment.resolver_decision" || events[0].Status != "ready_plugin_control" {
		t.Fatalf("events = %+v", events)
	}
	if events[0].Title != "混音建议已可执行" || strings.Contains(events[0].Body, "resolver") {
		t.Fatalf("resolver event was not localized: %+v", events[0])
	}
	var applyCommands []map[string]any
	for _, cmd := range kernel.commands {
		if cleanContextText(cmd["cmd"]) == "n_apply_control" {
			applyCommands = append(applyCommands, cmd)
		}
		if cleanContextText(cmd["cmd"]) == "set_plugin_param" || cleanContextText(cmd["cmd"]) == "daw.invoke" {
			t.Fatalf("unexpected raw mutation command: %+v", cmd)
		}
	}
	if len(applyCommands) != 1 {
		t.Fatalf("kernel commands = %+v, want exactly one n_apply_control", kernel.commands)
	}
	apply := applyCommands[0]
	if cleanContextText(apply["track_id"]) != "1007" || cleanContextText(apply["plugin_id"]) != "nova_1" {
		t.Fatalf("apply target = %+v", apply)
	}
	if cleanContextText(apply["control"]) != "control low mids with band 2" {
		t.Fatalf("apply control = %+v", apply)
	}
	if cleanContextText(apply["user_message"]) != "" {
		t.Fatalf("user message leaked into kernel command: %+v", apply)
	}
	target := mapValue(apply["target"])
	if fmt.Sprint(target["gain_db"]) != "-1.5" {
		t.Fatalf("apply target payload = %+v", target)
	}
}

func TestResolveMixTreatmentGainBalanceRequiresExplicitDelta(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	decision := server.resolveMixTreatment(context.Background(), agentloop.MixTreatmentPending{
		SchemaVersion: "mix_treatment_pending.v0",
		Status:        "pending_confirmation",
		TargetRef:     "track:1007",
		ActionKind:    "gain_balance",
		ProcessorType: "utility",
		Intent:        "bring vocal down a little",
	}, map[string]any{})

	if decision.Status != "needs_preparation" {
		t.Fatalf("decision = %+v", decision)
	}
	if !testStringSliceContains(decision.Needs, "exact_control") {
		t.Fatalf("needs = %+v", decision.Needs)
	}
	if len(decision.ToolRoute) != 3 || !testStringSliceContains(decision.ToolRoute, "mix.propose_tick") {
		t.Fatalf("tool route = %+v", decision.ToolRoute)
	}
}

func TestPendingMixTreatmentReadyGainBalanceExecutesTypedMixTick(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "mix.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"volume_db":      -3.0,
		}},
	})
	kernel := &recordingChatKernel{replies: []map[string]any{
		{"status": "ok", "project_path": projectPath, "snapshot_xml": "<project/>"},
		{"status": "ok", "track_id": "1007", "volume_db": -4.0},
		{"status": "ok", "project_path": projectPath, "tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"volume_db":      -4.0,
		}}},
	}}
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	server.pendingTreatments["chat_mix"] = agentloop.MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            "chat_mix",
		ObservationID:             "obs_1",
		Intent:                    "bring vocal down slightly",
		TargetRef:                 "track:1007",
		ActionKind:                "gain_balance",
		ProcessorType:             "utility",
		DeltaDB:                   -1,
		ReasoningSummary:          "explicit small gain move",
		Confidence:                "high",
		ExpiresAfterContextChange: true,
		Fingerprint: map[string]any{
			"target_track_id": "1007",
			"track_count":     1,
			"track_gain_db":   -3.0,
			"before_track":    map[string]any{"track_id": "1007", "volume_db": -3.0, "peak_dbfs": -6.0, "rms_dbfs": -15.0, "headroom_db": 6.0},
		},
	}

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", ChatRequest{
		ConversationID: "chat_mix",
		Message:        "可以，继续",
		Context:        map[string]any{"conversation_id": "chat_mix", "project_path": projectPath},
	}, agentModeDefault)

	if !handled || resp.StopReason != "mix_tick_applied_reobserved" {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	if len(resp.ExecutedKernelReply) != 3 {
		t.Fatalf("executed = %+v", resp.ExecutedKernelReply)
	}
	var tools []string
	for _, row := range resp.ExecutedKernelReply {
		tools = append(tools, cleanContextText(row["tool"]))
	}
	if !testStringSliceContains(tools, "mix.propose_tick") || !testStringSliceContains(tools, "mix.apply_tick") || !testStringSliceContains(tools, "mix.observe") {
		t.Fatalf("executed tools = %+v", tools)
	}
	if testStringSliceContains(tools, "daw.invoke") || testStringSliceContains(tools, "track.volume") || testStringSliceContains(tools, "plugin.set_parameter") {
		t.Fatalf("unexpected raw route = %+v", tools)
	}
	events, _ := server.agentEventsSince("chat_mix", 0, 10)
	if len(events) != 1 || events[0].Status != "ready_gain_tick" {
		t.Fatalf("events = %+v", events)
	}
	var setVolumeCommands []map[string]any
	for _, cmd := range kernel.commands {
		if cleanContextText(cmd["cmd"]) == "set_volume" {
			setVolumeCommands = append(setVolumeCommands, cmd)
		}
		if cleanContextText(cmd["cmd"]) == "daw.invoke" || cleanContextText(cmd["cmd"]) == "set_plugin_param" {
			t.Fatalf("unexpected kernel command: %+v", cmd)
		}
	}
	if len(setVolumeCommands) != 1 {
		t.Fatalf("kernel commands = %+v, want one set_volume", kernel.commands)
	}
	if cleanContextText(setVolumeCommands[0]["track_id"]) != "1007" || fmt.Sprint(setVolumeCommands[0]["db"]) != "-4" {
		t.Fatalf("set_volume command = %+v", setVolumeCommands[0])
	}
}

func TestResolveMixTreatmentPanBalanceRequiresExplicitMove(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	decision := server.resolveMixTreatment(context.Background(), agentloop.MixTreatmentPending{
		SchemaVersion: "mix_treatment_pending.v0",
		Status:        "pending_confirmation",
		TargetRef:     "track:1007",
		ActionKind:    "pan_balance",
		ProcessorType: "utility",
		Intent:        "move guitar a little",
	}, map[string]any{})

	if decision.Status != "needs_preparation" {
		t.Fatalf("decision = %+v", decision)
	}
	if !testStringSliceContains(decision.Needs, "exact_control") {
		t.Fatalf("needs = %+v", decision.Needs)
	}
	if len(decision.ToolRoute) != 3 || !testStringSliceContains(decision.ToolRoute, "mix.propose_tick") {
		t.Fatalf("tool route = %+v", decision.ToolRoute)
	}
}

func TestPendingMixTreatmentReadyPanBalanceExecutesTypedMixTick(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "mix.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Guitar",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"pan":            0.0,
		}},
	})
	kernel := &recordingChatKernel{replies: []map[string]any{
		{"status": "ok", "project_path": projectPath, "snapshot_xml": "<project/>"},
		{"status": "ok", "track_id": "1007", "pan": -0.1},
		{"status": "ok", "project_path": projectPath, "tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Guitar",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"pan":            -0.1,
		}}},
	}}
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	server.pendingTreatments["chat_mix"] = agentloop.MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            "chat_mix",
		ObservationID:             "obs_1",
		Intent:                    "move guitar left slightly",
		TargetRef:                 "track:1007",
		ActionKind:                "pan_balance",
		ProcessorType:             "utility",
		DeltaPan:                  -0.1,
		ReasoningSummary:          "explicit small pan move",
		Confidence:                "high",
		ExpiresAfterContextChange: true,
		Fingerprint: map[string]any{
			"target_track_id": "1007",
			"track_count":     1,
			"track_pan":       0.0,
			"before_track":    map[string]any{"track_id": "1007", "pan": 0.0, "peak_dbfs": -9.0, "rms_dbfs": -18.0, "headroom_db": 9.0},
		},
	}

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", ChatRequest{
		ConversationID: "chat_mix",
		Message:        "可以，继续",
		Context:        map[string]any{"conversation_id": "chat_mix", "project_path": projectPath},
	}, agentModeDefault)

	if !handled || resp.StopReason != "mix_tick_applied_reobserved" {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	if len(resp.ExecutedKernelReply) != 3 {
		t.Fatalf("executed = %+v", resp.ExecutedKernelReply)
	}
	var tools []string
	for _, row := range resp.ExecutedKernelReply {
		tools = append(tools, cleanContextText(row["tool"]))
	}
	if !testStringSliceContains(tools, "mix.propose_tick") || !testStringSliceContains(tools, "mix.apply_tick") || !testStringSliceContains(tools, "mix.observe") {
		t.Fatalf("executed tools = %+v", tools)
	}
	if testStringSliceContains(tools, "daw.invoke") || testStringSliceContains(tools, "track.pan") || testStringSliceContains(tools, "plugin.set_parameter") {
		t.Fatalf("unexpected raw route = %+v", tools)
	}
	events, _ := server.agentEventsSince("chat_mix", 0, 10)
	if len(events) != 1 || events[0].Status != "ready_pan_tick" {
		t.Fatalf("events = %+v", events)
	}
	var setPanCommands []map[string]any
	for _, cmd := range kernel.commands {
		if cleanContextText(cmd["cmd"]) == "set_pan" {
			setPanCommands = append(setPanCommands, cmd)
		}
		if cleanContextText(cmd["cmd"]) == "daw.invoke" || cleanContextText(cmd["cmd"]) == "set_plugin_param" {
			t.Fatalf("unexpected kernel command: %+v", cmd)
		}
	}
	if len(setPanCommands) != 1 {
		t.Fatalf("kernel commands = %+v, want one set_pan", kernel.commands)
	}
	if cleanContextText(setPanCommands[0]["track_id"]) != "1007" || fmt.Sprint(setPanCommands[0]["pan"]) != "-0.1" {
		t.Fatalf("set_pan command = %+v", setPanCommands[0])
	}
}

func TestPendingMixTreatmentReadyPanCenterExecutesTypedMixTick(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "mix.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"pan":            0.25,
		}},
	})
	kernel := &recordingChatKernel{replies: []map[string]any{
		{"status": "ok", "project_path": projectPath, "snapshot_xml": "<project/>"},
		{"status": "ok", "track_id": "1007", "pan": 0.0},
		{"status": "ok", "project_path": projectPath, "tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"pan":            0.0,
		}}},
	}}
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	targetPan := 0.0
	server.pendingTreatments["chat_mix"] = agentloop.MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            "chat_mix",
		ObservationID:             "obs_1",
		Intent:                    "center the vocal",
		TargetRef:                 "track:1007",
		ActionKind:                "pan_balance",
		ProcessorType:             "utility",
		TargetPan:                 &targetPan,
		ReasoningSummary:          "explicit center pan move",
		Confidence:                "high",
		ExpiresAfterContextChange: true,
		Fingerprint: map[string]any{
			"target_track_id": "1007",
			"track_count":     1,
			"track_pan":       0.25,
		},
	}

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_mix", ChatRequest{
		ConversationID: "chat_mix",
		Message:        "可以，继续",
		Context:        map[string]any{"conversation_id": "chat_mix", "project_path": projectPath},
	}, agentModeDefault)

	if !handled || resp.StopReason != "mix_tick_applied_reobserved" {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	var setPanCommands []map[string]any
	for _, cmd := range kernel.commands {
		if cleanContextText(cmd["cmd"]) == "set_pan" {
			setPanCommands = append(setPanCommands, cmd)
		}
	}
	if len(setPanCommands) != 1 || cleanContextText(setPanCommands[0]["track_id"]) != "1007" || fmt.Sprint(setPanCommands[0]["pan"]) != "0" {
		t.Fatalf("set_pan commands = %+v all=%+v", setPanCommands, kernel.commands)
	}
}

func TestPendingMixTickTrackLookupFindsNestedVisibleTracks(t *testing.T) {
	rows := chatVisibleTrackRows(map[string]any{
		"project_state": map[string]any{
			"shadow": map[string]any{
				"tracks": []any{map[string]any{
					"track_id":   "track_1",
					"track_name": "Vocal",
				}},
			},
		},
	})
	if len(rows) != 1 || cleanContextText(rows[0]["track_id"]) != "track_1" {
		t.Fatalf("nested rows = %#v", rows)
	}
}

func TestPendingMixTickContinueExecutesTypedLoop(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation:                 "track_gain_adjust",
		TrackID:                   "track_1",
		DeltaDB:                   -1,
		ObservationID:             "obs_1",
		ExpiresAfterContextChange: true,
		Status:                    "pending_confirmation",
		Fingerprint:               map[string]any{"target_scope": "selected_track", "mix_session_id": "mix_1"},
	}
	server.shadow.Initialize(map[string]any{"tracks": []any{map[string]any{
		"track_id":       "track_1",
		"track_name":     "Vocal",
		"track_type":     "hybrid",
		"is_audio_track": true,
		"volume_db":      -3,
	}}})

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "继续"}, agentModeDefault)

	if !handled || resp.StopReason != "mix_tick_confirmation_failed" {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	var tools []string
	for _, row := range resp.ExecutedKernelReply {
		tools = append(tools, cleanContextText(row["tool"]))
	}
	if !testStringSliceContains(tools, "mix.propose_tick") || !testStringSliceContains(tools, "mix.apply_tick") {
		t.Fatalf("executed tools = %+v, want typed propose/apply attempt", tools)
	}
	if testStringSliceContains(tools, "daw.invoke") || testStringSliceContains(tools, "track.volume") {
		t.Fatalf("confirmation bypassed typed tools: %+v", tools)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; !ok {
		t.Fatal("failed apply should keep pending mix tick available for retry")
	}
}

func TestPendingMixTickContinueDiscussingDoesNotExecute(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust",
		TrackID:   "track_1",
		DeltaDB:   -1,
		Status:    "pending_confirmation",
	}

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "继续讨论一下原因"}, agentModeDefault)

	if handled {
		t.Fatalf("discussion should fall through to normal chat, resp=%+v", resp)
	}
	if len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("discussion should not execute: %+v", resp.ExecutedKernelReply)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; !ok {
		t.Fatal("discussion should keep pending candidate")
	}
}

func TestPendingMixTickWhyQuestionDoesNotExecute(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust",
		TrackID:   "track_2",
		DeltaDB:   -1,
		Status:    "pending_confirmation",
	}

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "为什么要降 Track 2"}, agentModeDefault)

	if handled {
		t.Fatalf("why question should fall through to normal chat, resp=%+v", resp)
	}
	if len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("why question should not execute: %+v", resp.ExecutedKernelReply)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; !ok {
		t.Fatal("why question should keep pending candidate")
	}
}

func TestPendingMixTickPlainApprovalExecutesTypedLoop(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation:                 "track_gain_adjust",
		TrackID:                   "track_1",
		DeltaDB:                   -1,
		ObservationID:             "obs_1",
		ExpiresAfterContextChange: true,
		Status:                    "pending_confirmation",
		Fingerprint:               map[string]any{"target_scope": "selected_track", "mix_session_id": "mix_1"},
	}
	server.shadow.Initialize(map[string]any{"tracks": []any{map[string]any{
		"track_id":       "track_1",
		"track_name":     "Vocal",
		"track_type":     "hybrid",
		"is_audio_track": true,
		"volume_db":      -3,
	}}})

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "\u53ef\u4ee5"}, agentModeDefault)

	if !handled || resp.StopReason != "mix_tick_confirmation_failed" {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	var tools []string
	for _, row := range resp.ExecutedKernelReply {
		tools = append(tools, cleanContextText(row["tool"]))
	}
	if !testStringSliceContains(tools, "mix.propose_tick") || !testStringSliceContains(tools, "mix.apply_tick") {
		t.Fatalf("executed tools = %+v, want typed propose/apply attempt", tools)
	}
	if testStringSliceContains(tools, "daw.invoke") || testStringSliceContains(tools, "track.volume") {
		t.Fatalf("confirmation bypassed typed tools: %+v", tools)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; !ok {
		t.Fatal("failed apply should keep pending mix tick available for retry")
	}
}

func TestPendingMixTickNoCandidateDoesNotExecute(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "可以执行"}, agentModeDefault)
	if !handled || resp.StopReason != "no_pending_mix_tick_candidate" || !strings.Contains(resp.Reply, "没有可执行") {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
}

func TestPendingMixTickReportIncludesBeforeAfterAcousticsAndRollback(t *testing.T) {
	reply := pendingMixTickReport(agentloop.PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       "track_1",
		DeltaDB:       -1.5,
		ObservationID: "obs_1",
		Status:        "pending_confirmation",
		Fingerprint: map[string]any{
			"before_track": map[string]any{
				"track_id":    "track_1",
				"peak_dbfs":   -0.2,
				"rms_dbfs":    -8.4,
				"headroom_db": 0.2,
			},
		},
	}, executorResultForMixTickReport("mix.propose_tick", map[string]any{"before_db": -3.0, "after_db": -4.5}), executorResultForMixTickReport("mix.apply_tick", map[string]any{"before_db": -3.0, "after_db": -4.5}), executorResultForMixTickReport("mix.observe", map[string]any{
		"observation": map[string]any{
			"project_package": map[string]any{
				"tracks": []map[string]any{{
					"track_id":    "track_1",
					"peak_dbfs":   -1.7,
					"rms_dbfs":    -9.9,
					"headroom_db": 1.7,
				}},
			},
		},
	}), "")

	for _, want := range []string{"peak -0.2 -> -1.7", "RMS -8.4 -> -9.9", "headroom 0.2 -> 1.7", "peak risk 已改善", "rollback 可用"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, reply)
		}
	}
}

func TestPendingMixTickReportUsesSelectedAcousticWaveformAfterMetrics(t *testing.T) {
	reply := pendingMixTickReport(agentloop.PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       "track_2",
		DeltaDB:       -1,
		ObservationID: "obs_1",
		Status:        "pending_confirmation",
		Fingerprint: map[string]any{
			"before_track": map[string]any{
				"track_id":    "track_2",
				"peak_dbfs":   -0.1,
				"rms_dbfs":    -8.2,
				"headroom_db": 0.1,
			},
		},
	}, executorResultForMixTickReport("mix.propose_tick", map[string]any{"before_db": 0.0, "after_db": -1.0}), executorResultForMixTickReport("mix.apply_tick", map[string]any{"before_db": 0.0, "after_db": -1.0}), executorResultForMixTickReport("mix.observe", map[string]any{
		"resolved_target": map[string]any{
			"track_id":   "track_2",
			"track_name": "Track 2",
		},
		"acoustic_digest": map[string]any{
			"track_name": "Track 2",
			"peak_dbfs":  -1.1,
			"rms_dbfs":   -9.2,
			"waveform": map[string]any{
				"peak_dbfs":   -1.1,
				"rms_dbfs":    -9.2,
				"headroom_db": 1.1,
				"crest_db":    8.1,
			},
		},
	}), "")

	for _, want := range []string{"peak -0.1 -> -1.1", "RMS -8.2 -> -9.2", "headroom 0.1 -> 1.1"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, reply)
		}
	}
	if strings.Contains(reply, "headroom 0.1 -> 未知") || strings.Contains(reply, "headroom 0.1 -> 鏈") {
		t.Fatalf("reply should use selected acoustic waveform headroom:\n%s", reply)
	}
}

func TestPendingMixTickReportIncludesReadyABResult(t *testing.T) {
	reply := pendingMixTickReport(agentloop.PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       "track_1",
		DeltaDB:       1.0,
		ObservationID: "obs_before",
		Status:        "pending_confirmation",
	}, executorResultForMixTickReport("mix.propose_tick", map[string]any{"before_db": -3.0, "after_db": -2.0}), executorResultForMixTickReport("mix.apply_tick", map[string]any{"before_db": -3.0, "after_db": -2.0}), executorResultForMixTickReport("mix.observe", map[string]any{
		"observation": map[string]any{
			"mix_package": map[string]any{
				"current_metrics": map[string]any{
					"ab_result": map[string]any{
						"status":                 "ready",
						"tap_point":              "track_post_fader",
						"before_evidence_ref":    "dad.l2_render_probe:render_before",
						"after_evidence_ref":     "dad.l2_render_probe:render_after",
						"before_render_revision": "render_before",
						"after_render_revision":  "render_after",
						"delta": map[string]any{
							"levels": map[string]any{
								"peak_dbfs": map[string]any{"delta": 0.6},
								"rms_dbfs":  map[string]any{"delta": 1.2},
							},
							"stereo": map[string]any{
								"balance_db":           map[string]any{"delta": 0.3},
								"correlation_estimate": map[string]any{"delta": -0.08},
							},
						},
					},
				},
			},
		},
	}), "")

	for _, want := range []string{"AB Result：可信", "观测点=track_post_fader", "峰值 +0.6 dB", "RMS +1.2 dB", "声像 +0.3 dB", "相关性 -0.08"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, reply)
		}
	}
	for _, unexpected := range []string{"dad.l2_render_probe:", "render_revision："} {
		if strings.Contains(reply, unexpected) {
			t.Fatalf("reply should keep long evidence refs out of chat text, found %q:\n%s", unexpected, reply)
		}
	}
}

func TestPendingMixTickReportDoesNotTrustMissingABResult(t *testing.T) {
	reply := pendingMixTickReport(agentloop.PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       "track_1",
		DeltaDB:       1.0,
		ObservationID: "obs_before",
		Status:        "pending_confirmation",
	}, executorResultForMixTickReport("mix.propose_tick", map[string]any{"before_db": -3.0, "after_db": -2.0}), executorResultForMixTickReport("mix.apply_tick", map[string]any{"before_db": -3.0, "after_db": -2.0}), executorResultForMixTickReport("mix.observe", map[string]any{}), "")

	for _, want := range []string{"AB Result：不可信", "ab_result_missing", "未把这次改动标记为已验证"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, reply)
		}
	}
}

func TestProjectResultCardIncludesReadyABResult(t *testing.T) {
	executed := []map[string]any{{
		"status":       "ok",
		"command_name": "mix_apply_tick",
		"result":       map[string]any{"status": "ok", "track_id": "track_1"},
	}}
	observe := executorResultForMixTickReport("mix.observe", map[string]any{
		"observation": map[string]any{
			"mix_package": map[string]any{
				"current_metrics": map[string]any{
					"ab_result": map[string]any{
						"status":                 "ready",
						"tap_point":              "track_post_fader",
						"render_mode":            "offline_probe",
						"before_evidence_ref":    "dad.l2_render_probe:render_before",
						"after_evidence_ref":     "dad.l2_render_probe:render_after",
						"before_render_revision": "render_before",
						"after_render_revision":  "render_after",
						"delta": map[string]any{
							"levels": map[string]any{
								"peak_dbfs": map[string]any{"delta": -1.0},
								"rms_dbfs":  map[string]any{"delta": -1.0},
							},
						},
					},
				},
			},
		},
	})

	cards := projectResultCardsFromExecutedWithAB(executed, observe)
	if len(cards) != 1 {
		t.Fatalf("cards = %+v", cards)
	}
	ab := mapValue(cards[0]["ab_result"])
	if cleanContextText(ab["status"]) != "ready" || cleanContextText(ab["tap_point"]) != "track_post_fader" {
		t.Fatalf("ab card = %#v", ab)
	}
	if !strings.Contains(cleanContextText(ab["display_title"]), "AB Result") || !strings.Contains(cleanContextText(ab["display_body"]), "峰值 -1 dB") || !strings.Contains(cleanContextText(ab["display_body"]), "RMS -1 dB") {
		t.Fatalf("ab display should be visible and compact: %#v", ab)
	}
	if cleanContextText(ab["before_render_revision"]) != "render_before" || cleanContextText(ab["after_render_revision"]) != "render_after" {
		t.Fatalf("ab revisions = %#v", ab)
	}
}

func TestProjectResultCardReadsReadyABResultFromMOMProjectionStruct(t *testing.T) {
	executed := []map[string]any{{
		"status":       "ok",
		"command_name": "set_plugin_param",
		"result":       map[string]any{"status": "ok", "track_id": "1007", "plugin_id": "plugin_eq"},
	}}
	abResult := map[string]any{
		"schema_version":          "mom_ab_result.v1",
		"status":                  "ready",
		"tap_point":               "track_post_fader",
		"render_mode":             "offline_probe",
		"before_observation_id":   "obs_before",
		"after_observation_id":    "obs_after",
		"before_evidence_ref":     "dad.l2_render_probe:before",
		"after_evidence_ref":      "dad.l2_render_probe:after",
		"before_render_revision":  "render_before",
		"after_render_revision":   "render_after",
		"render_revision_changed": true,
		"delta": map[string]any{
			"levels": map[string]any{
				"peak_dbfs": map[string]any{"delta": 0.393},
				"rms_dbfs":  map[string]any{"delta": -0.372},
			},
			"stereo": map[string]any{
				"balance_db":           map[string]any{"delta": -0.008},
				"correlation_estimate": map[string]any{"delta": -0.032},
			},
		},
	}
	projection := &mom.Projection{
		MOMVersion: "v1.3",
		Layers: mom.Layers{
			ABResultComparison: mom.Layer{
				Status: "ready",
				Facts:  map[string]any{"ab_result": abResult},
			},
		},
	}
	observe := executorResultForMixTickReport("mix.observe", map[string]any{
		"mom_projection": projection,
	})

	reply := strings.Join(pendingMixTickABResultLines(observe), "\n")
	if !strings.Contains(reply, "AB Result：可信") || !strings.Contains(reply, "观测点=track_post_fader / offline_probe") || !strings.Contains(reply, "峰值 +0.393 dB") {
		t.Fatalf("reply should trust struct-shaped MOM projection, got:\n%s", reply)
	}
	if strings.Contains(reply, "dad.l2_render_probe:") || strings.Contains(reply, "render_revision：") {
		t.Fatalf("reply should stay compact and keep evidence refs structured, got:\n%s", reply)
	}
	cards := projectResultCardsFromExecutedWithAB(executed, observe)
	if len(cards) != 1 {
		t.Fatalf("cards = %+v", cards)
	}
	ab := mapValue(cards[0]["ab_result"])
	if cleanContextText(ab["status"]) != "ready" || cleanContextText(ab["reason"]) != "quality_gate_not_ready" {
		t.Fatalf("ab card = %#v", ab)
	}
	if !strings.Contains(cleanContextText(ab["display_title"]), "AB Result：可信") || !strings.Contains(cleanContextText(ab["display_body"]), "峰值 +0.393 dB") {
		t.Fatalf("ab card should expose trusted AB result: %#v", ab)
	}
}

func TestProjectResultCardIncludesMissingABResult(t *testing.T) {
	executed := []map[string]any{{
		"status":       "ok",
		"command_name": "set_plugin_param",
		"result":       map[string]any{"status": "ok", "track_id": "track_1", "plugin_id": "plugin_eq"},
	}}

	cards := projectResultCardsFromExecutedWithAB(executed, executorResultForMixTickReport("mix.observe", map[string]any{}))
	if len(cards) != 1 {
		t.Fatalf("cards = %+v", cards)
	}
	ab := mapValue(cards[0]["ab_result"])
	if cleanContextText(ab["status"]) != "missing" || cleanContextText(ab["reason"]) != "ab_result_missing" {
		t.Fatalf("ab card = %#v", ab)
	}
	if !strings.Contains(cleanContextText(ab["display_title"]), "AB Result") || !strings.Contains(cleanContextText(ab["display_body"]), "ab_result_missing") {
		t.Fatalf("missing ab should be explicit: %#v", ab)
	}
}

func TestNextPendingMixTickCandidateFromReobserveUsesHighHeadroomRisk(t *testing.T) {
	observe := executorResultForMixTickReport("mix.observe", map[string]any{
		"observation_id": "obs_after",
		"observation": map[string]any{
			"mix_session_id": "mix_1",
			"project_package": map[string]any{
				"active_acoustic_track_count": 2,
				"headroom_risk": []map[string]any{
					{
						"track_id":    "track_1",
						"name":        "Lead",
						"risk":        "medium",
						"headroom_db": 2.4,
					},
					{
						"track_id":    "track_2",
						"name":        "Drums",
						"risk":        "high",
						"peak_dbfs":   -0.4,
						"rms_dbfs":    -8.1,
						"headroom_db": 0.4,
						"crest_db":    7.7,
					},
				},
			},
		},
	})

	candidate, ok := nextPendingMixTickCandidateFromReobserve("chat_mix", "goal_1", "run_1", agentloop.PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       "track_1",
		DeltaDB:       -1,
		ObservationID: "obs_before",
		Status:        "pending_confirmation",
		Fingerprint:   map[string]any{"track_count": 2, "mix_session_id": "mix_1"},
	}, observe)

	if !ok {
		t.Fatal("expected high-risk reobserve row to produce a next pending candidate")
	}
	if candidate.Operation != "track_gain_adjust" || candidate.TrackID != "track_2" || candidate.DeltaDB != -1 || candidate.Status != "pending_confirmation" {
		t.Fatalf("candidate = %+v", candidate)
	}
	if candidate.ObservationID != "obs_after" {
		t.Fatalf("observation id = %q", candidate.ObservationID)
	}
	if candidate.Evidence["source"] != "reobserve_after_mix_tick" || candidate.Evidence["risk"] != "high" {
		t.Fatalf("evidence = %+v", candidate.Evidence)
	}
	if got := intNumber(candidate.Fingerprint["track_count"]); got != 2 {
		t.Fatalf("track count fingerprint = %d", got)
	}
	beforeTrack := mapValue(candidate.Fingerprint["before_track"])
	if cleanContextText(beforeTrack["track_id"]) != "track_2" || cleanContextText(candidate.Fingerprint["headroom_db"]) != "0.4" {
		t.Fatalf("fingerprint = %+v", candidate.Fingerprint)
	}
	if math.Abs(candidate.DeltaDB) > 2 {
		t.Fatalf("delta should be clamped to v1 range: %+v", candidate)
	}
}

func TestNextPendingMixTickCandidateFromReobserveIgnoresNonHighRisk(t *testing.T) {
	observe := executorResultForMixTickReport("mix.observe", map[string]any{
		"observation": map[string]any{
			"project_package": map[string]any{
				"headroom_risk": []map[string]any{
					{"track_id": "track_1", "risk": "medium", "headroom_db": 2.2},
					{"track_id": "track_2", "risk": "low", "headroom_db": 6.5},
				},
			},
		},
	})

	candidate, ok := nextPendingMixTickCandidateFromReobserve("chat_mix", "goal_1", "run_1", agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust",
		TrackID:   "track_1",
		DeltaDB:   -1,
		Status:    "pending_confirmation",
	}, observe)

	if ok {
		t.Fatalf("non-high risks should not produce next pending candidate: %+v", candidate)
	}
}

func TestNextPendingMixTickCandidateFromReobserveDoesNotRepeatJustAppliedTrack(t *testing.T) {
	observe := executorResultForMixTickReport("mix.observe", map[string]any{
		"observation": map[string]any{
			"project_package": map[string]any{
				"headroom_risk": []map[string]any{
					{"track_id": "track_1", "risk": "high", "headroom_db": 0.3},
					{"track_id": "track_2", "risk": "medium", "headroom_db": 2.2},
				},
			},
		},
	})

	candidate, ok := nextPendingMixTickCandidateFromReobserve("chat_mix", "goal_1", "run_1", agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust",
		TrackID:   "track_1",
		DeltaDB:   -1,
		Status:    "pending_confirmation",
	}, observe)

	if ok {
		t.Fatalf("same just-applied track should not be queued again in v1: %+v", candidate)
	}
}

func TestStorePendingMixTickCandidateEmitsPendingEvent(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.storePendingMixTickCandidate("chat_mix", "goal_1", "run_1", agentloop.PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       "track_2",
		DeltaDB:       -1,
		ObservationID: "obs_after",
		Status:        "pending_confirmation",
	})

	candidate, ok := server.pendingMixTickForConversation("chat_mix")
	if !ok || candidate.TrackID != "track_2" || candidate.DeltaDB != -1 {
		t.Fatalf("stored candidate = %+v ok=%v", candidate, ok)
	}
	events, _ := server.agentEventsSince("chat_mix", 0, 10)
	if len(events) != 1 || events[0].Type != "mix_tick.pending" || events[0].Payload["track_id"] != "track_2" {
		t.Fatalf("events = %+v", events)
	}
}

func executorResultForMixTickReport(tool string, result map[string]any) executor.Result {
	return executor.Result{
		Status: "ok",
		Tool:   tool,
		Result: result,
	}
}

func TestPendingMixTickTopicShiftExpiresWithoutExecuting(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust",
		TrackID:   "track_1",
		DeltaDB:   -1,
		Status:    "pending_confirmation",
	}

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "帮我看整体混音"}, agentModeDefault)

	if handled {
		t.Fatalf("topic shift should fall through to the normal agent loop, resp=%+v", resp)
	}
	if len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("topic shift should not execute: %+v", resp.ExecutedKernelReply)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; ok {
		t.Fatal("topic shift should expire stale pending candidate")
	}
}

func TestPendingMixTickMentioningOtherTrackExpiresWithoutExecuting(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust",
		TrackID:   "track_1",
		DeltaDB:   -1,
		Status:    "pending_confirmation",
	}

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "可以执行 Track 3"}, agentModeDefault)

	if handled {
		t.Fatalf("new track target should fall through to normal chat, resp=%+v", resp)
	}
	if len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("new track target should not execute stale pending: %+v", resp.ExecutedKernelReply)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; ok {
		t.Fatal("new track target should expire stale pending candidate")
	}
}

func TestPendingMixTickPluginTargetExpiresWithoutExecuting(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust",
		TrackID:   "track_1",
		DeltaDB:   -1,
		Status:    "pending_confirmation",
	}

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "给主唱加 reverb"}, agentModeDefault)

	if handled {
		t.Fatalf("plugin target should fall through to normal chat, resp=%+v", resp)
	}
	if len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("plugin target should not execute stale pending: %+v", resp.ExecutedKernelReply)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; ok {
		t.Fatal("plugin target should expire stale pending candidate")
	}
}

func TestPendingMixTickExpiresWhenTrackGainChanged(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation:                 "track_gain_adjust",
		TrackID:                   "track_1",
		DeltaDB:                   -1,
		Status:                    "pending_confirmation",
		ExpiresAfterContextChange: true,
		Fingerprint:               map[string]any{"track_gain_db": -3.0, "track_count": 1},
	}
	server.shadow.Initialize(map[string]any{"tracks": []any{map[string]any{
		"track_id":       "track_1",
		"track_name":     "Vocal",
		"track_type":     "hybrid",
		"is_audio_track": true,
		"volume_db":      -4.0,
	}}})

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "可以执行"}, agentModeDefault)

	if !handled || resp.StopReason != "expired_pending_mix_tick_candidate" || !strings.Contains(resp.Error, "音量已经变化") {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	if len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("expired candidate should not execute: %+v", resp.ExecutedKernelReply)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; ok {
		t.Fatal("expired candidate should be removed")
	}
}

func TestPendingMixTickExpiresWhenTrackCountChanged(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_mix"] = agentloop.PendingMixTickCandidate{
		Operation:                 "track_gain_adjust",
		TrackID:                   "track_1",
		DeltaDB:                   -1,
		Status:                    "pending_confirmation",
		ExpiresAfterContextChange: true,
		Fingerprint:               map[string]any{"track_gain_db": -3.0, "track_count": 1},
	}
	server.shadow.Initialize(map[string]any{"tracks": []any{
		map[string]any{
			"track_id":       "track_1",
			"track_name":     "Vocal",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"volume_db":      -3.0,
		},
		map[string]any{
			"track_id":       "track_2",
			"track_name":     "Guitar",
			"track_type":     "hybrid",
			"is_audio_track": true,
			"volume_db":      -6.0,
		},
	}})

	resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_mix", ChatRequest{Message: "可以执行"}, agentModeDefault)

	if !handled || resp.StopReason != "expired_pending_mix_tick_candidate" || !strings.Contains(resp.Error, "轨道数量已经变化") {
		t.Fatalf("resp=%+v handled=%v", resp, handled)
	}
	if len(resp.ExecutedKernelReply) != 0 {
		t.Fatalf("expired candidate should not execute: %+v", resp.ExecutedKernelReply)
	}
	if _, ok := server.pendingMixTicks["chat_mix"]; ok {
		t.Fatal("expired candidate should be removed")
	}
}

func testStringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func testAnyStringSliceContains(value any, needle string) bool {
	switch rows := value.(type) {
	case []string:
		return testStringSliceContains(rows, needle)
	case []any:
		for _, row := range rows {
			if cleanContextText(row) == needle {
				return true
			}
		}
	}
	return false
}

func mustAgentEvents(server *Server, conversationID string) []AgentEvent {
	events, _ := server.agentEventsSince(conversationID, 0, 100)
	return events
}

func testAgentEventsContainTurnCompleted(server *Server, conversationID, goalID string, status agentruntime.GoalStatus) bool {
	for _, event := range mustAgentEvents(server, conversationID) {
		if event.Type == "turn.completed" && event.GoalID == goalID && event.Status == string(status) {
			return true
		}
	}
	return false
}

func TestAgentLoopBroadMixConfirmationDoesNotResumePluginLoad(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := PendingPlan{
		ID:        "plan_agent_loop_mix",
		CreatedAt: time.Now(),
		Workflow:  agentLoopConfirmationWorkflow,
		Context: map[string]any{
			"goal_id": "goal_mix",
			"run_id":  "run_mix",
		},
		WorkflowData: map[string]any{"conversation_id": "chat_mix"},
		Decisions: policy.Analyze([]map[string]any{
			{"cmd": "rack_add_node", "track_id": "1007", "plugin_path": `C:\Program Files\Common Files\VST3\TDR Nova.vst3`},
		}),
		GoalContinuation: &agentloop.Continuation{
			GoalID:   "goal_mix",
			RunID:    "run_mix",
			UserText: "我想你帮我对这段音频进行缩混可以吗",
			Context:  map[string]any{"user_message": "我想你帮我对这段音频进行缩混可以吗"},
			PendingToolCall: &planner.ToolCall{
				ID:   "tool_step_1",
				Tool: "plugin.load_to_rack",
				Args: map[string]any{
					"track_id":     "1007",
					"plugin_path":  `C:\Program Files\Common Files\VST3\TDR Nova.vst3`,
					"plugin_query": "TDR Nova",
				},
			},
		},
	}
	server.pending[plan.ID] = plan

	status, response := server.resolvePendingPlanDecision(context.Background(), plan.ID, "approve")
	if status != http.StatusOK {
		t.Fatalf("status = %d response=%+v", status, response)
	}
	if response["blocked"] != true {
		t.Fatalf("agent-loop confirmation should be blocked, response=%+v", response)
	}
	if !strings.Contains(fmt.Sprint(response["message"]), "mix.request_observation") {
		t.Fatalf("response should direct observation first: %+v", response)
	}
}

func TestPendingMixConfirmationCanBeRevisedByNewMixRequest(t *testing.T) {
	plan := PendingPlan{
		ID:        "plan_mix_tick",
		CreatedAt: time.Now(),
		Workflow:  agentLoopConfirmationWorkflow,
		WorkflowData: map[string]any{
			"conversation_id": "chat_mix",
		},
		GoalContinuation: &agentloop.Continuation{
			PendingToolCall: &planner.ToolCall{
				Tool: "mix.apply_tick",
				Args: map[string]any{"tick_id": "mix_tick_1", "track_id": "1007"},
			},
		},
		Decisions: []policy.Decision{{
			Name:    "mix_apply_tick",
			Command: map[string]any{"tool": "mix.apply_tick", "tick_id": "mix_tick_1"},
			Risk:    policy.RiskUndoable,
		}},
	}

	if !pendingPlanCanBeRevisedByMixRequest(plan, "我想让当前轨道靠左一些") {
		t.Fatal("new subjective mix request should revise the pending mix confirmation")
	}
	if pendingPlanCanBeRevisedByMixRequest(plan, "可以执行") {
		t.Fatal("explicit execution confirmation should not revise the pending mix confirmation")
	}
}

func TestNonMixConfirmationIsNotRevisedByMixRequest(t *testing.T) {
	plan := PendingPlan{
		ID:        "plan_midi",
		CreatedAt: time.Now(),
		Workflow:  agentLoopConfirmationWorkflow,
		GoalContinuation: &agentloop.Continuation{
			PendingToolCall: &planner.ToolCall{
				Tool: "midi.apply_note_patch",
			},
		},
		Decisions: []policy.Decision{{
			Name:    "apply_midi_note_patch",
			Command: map[string]any{"tool": "midi.apply_note_patch"},
			Risk:    policy.RiskConfirm,
		}},
	}

	if pendingPlanCanBeRevisedByMixRequest(plan, "我想让当前轨道靠左一些") {
		t.Fatal("non-mix confirmation should keep the normal confirmation gate")
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

func TestPluginSkillArtifactOnlyStoredForFinalPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.artifactRoot = t.TempDir()
	target := pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Demo EQ"}
	patch := pluginProfilePatch{
		Class:           "eq",
		QuickControlIDs: []string{"gain"},
		Groups: []map[string]any{{
			"id":     "main",
			"role":   "eq",
			"params": map[string]any{"gain": map[string]any{"param_id": "gain"}},
		}},
		VirtualControls: []map[string]any{{"name": "eq.set_region"}},
	}

	candidate := map[string]any{
		"stage":   "candidate_review",
		"mode":    "auto_learn",
		"plan_id": "",
	}
	if summaries := server.storePluginProfilePatchArtifact("conv_skill", target, patch, candidate); len(summaries) != 0 {
		t.Fatalf("candidate artifact summaries = %+v", summaries)
	}
	if items, err := server.artifactStore().List(); err != nil || len(items) != 0 {
		t.Fatalf("candidate artifact store items=%+v err=%v", items, err)
	}

	final := map[string]any{
		"stage":   "final_confirmation",
		"mode":    "auto_learn",
		"plan_id": "plan_test",
		"ui_reference": map[string]any{
			"status":       "provided",
			"artifact_ids": []string{"art_ui_ref"},
			"warnings":     []string{"match was slow"},
			"visual_digest": map[string]any{
				"matching_status": "match_failed",
			},
		},
	}
	summaries := server.storePluginProfilePatchArtifact("conv_skill", target, patch, final)
	if len(summaries) != 1 {
		t.Fatalf("final artifact summaries = %+v", summaries)
	}
	if summaries[0].Kind != "plugin_skill" || summaries[0].Title != "Demo EQ.vps" {
		t.Fatalf("final artifact summary = %+v", summaries[0])
	}
	items, err := server.artifactStore().List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kind != "plugin_skill" || items[0].Metadata["plan_id"] != "plan_test" {
		t.Fatalf("final artifact items = %+v", items)
	}
	if items[0].Metadata["ui_reference_matching_status"] != "match_failed" {
		t.Fatalf("ui reference matching status metadata = %+v", items[0].Metadata)
	}
	if warnings := stringListValue(items[0].Metadata["ui_reference_warnings"]); len(warnings) != 1 || warnings[0] != "match was slow" {
		t.Fatalf("ui reference warnings metadata = %+v", items[0].Metadata)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(items[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ui_reference_matching_status"] != "match_failed" {
		t.Fatalf("ui reference matching status payload = %+v", payload)
	}

	secondPatch := patch
	secondPatch.Groups = []map[string]any{{
		"id":   "main",
		"role": "eq",
		"params": map[string]any{"gain": map[string]any{
			"param_id": "gain",
			"evidence": []map[string]any{{
				"kind":    "plugin_ui_reference_image",
				"summary": strings.Repeat("old visual evidence ", 80),
			}},
			"provenance": []map[string]any{{
				"kind":   "plugin_ui_reference_image",
				"source": "plugin_ui_reference_image",
				"data": map[string]any{
					"plugin_learning_session_id": "pl_session_old",
					"artifact_ids":               []string{"art_old"},
				},
			}},
		}},
	}}
	secondFinal := map[string]any{
		"stage":   "final_confirmation",
		"mode":    "auto_learn",
		"plan_id": "plan_test_2",
		"plugin_identity": map[string]any{
			"profile_key": "profile_demo_eq",
		},
		"param_signature_hash": "hash_demo",
	}
	firstFinalWithIdentity := map[string]any{
		"stage":   "final_confirmation",
		"mode":    "auto_learn",
		"plan_id": "plan_test_1",
		"plugin_identity": map[string]any{
			"profile_key": "profile_demo_eq",
		},
		"param_signature_hash": "hash_demo",
	}
	firstSummary := server.storePluginProfilePatchArtifact("conv_skill", target, patch, firstFinalWithIdentity)
	secondSummary := server.storePluginProfilePatchArtifact("conv_skill", target, secondPatch, secondFinal)
	if len(firstSummary) != 1 || len(secondSummary) != 1 || firstSummary[0].ID != secondSummary[0].ID {
		t.Fatalf("canonical skill artifact should use stable ID, first=%+v second=%+v", firstSummary, secondSummary)
	}
	items, err = server.artifactStore().List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("artifact store should contain initial anonymous skill plus one canonical replacement, got %+v", items)
	}
	for _, item := range items {
		if item.ID != secondSummary[0].ID {
			continue
		}
		if strings.Contains(item.Text, "pl_session_old") || strings.Contains(item.Text, "art_old") || strings.Contains(item.Text, "plugin_ui_reference_image") {
			t.Fatalf("canonical skill artifact leaked recursive visual provenance: %s", item.Text)
		}
	}
}

func TestPluginLearningDigestCompactsExistingSkillMemory(t *testing.T) {
	oldProvenance := []any{}
	oldEvidence := []any{}
	for i := 0; i < 40; i++ {
		oldEvidence = append(oldEvidence, map[string]any{
			"kind":    "plugin_ui_reference_image",
			"summary": strings.Repeat("old visual evidence ", 20),
		})
		oldProvenance = append(oldProvenance, map[string]any{
			"kind":   "plugin_ui_reference_image",
			"source": "plugin_ui_reference_image",
			"data": map[string]any{
				"plugin_learning_session_id": "pl_session_recursive",
				"artifact_ids":               []string{"art_recursive"},
			},
		})
	}
	digest := buildPluginParameterDigest(map[string]any{
		"track_id":  "track_1",
		"plugin_id": "plugin_1",
		"plugin_identity": map[string]any{
			"profile_key": "profile_recursive",
			"plugin_name": "Recursive Skill",
		},
		"global_profile": map[string]any{
			"profile_id": "profile_recursive",
			"plugin_skill": map[string]any{
				"schema_version": 2,
				"components": []any{map[string]any{
					"id":   "main",
					"role": "utility",
					"params": map[string]any{
						"amount": map[string]any{
							"param_id":            "p_amount",
							"label":               "Amount",
							"confidence":          0.9,
							"evidence":            oldEvidence,
							"provenance":          oldProvenance,
							"display_domain_text": "0~100 %",
						},
					},
				}},
			},
			"groups": []any{map[string]any{
				"id":   "main",
				"role": "utility",
				"params": map[string]any{
					"amount": map[string]any{
						"param_id":   "p_amount",
						"evidence":   oldEvidence,
						"provenance": oldProvenance,
					},
				},
			}},
		},
		"parameters": []any{map[string]any{
			"id":                "p_amount",
			"name":              "Amount",
			"host_controllable": true,
		}},
	})
	data, err := json.Marshal(digest)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "pl_session_recursive") || strings.Contains(text, "art_recursive") {
		t.Fatalf("existing skill memory leaked old session provenance: %s", text)
	}
	if strings.Contains(text, "plugin_ui_reference_image") {
		t.Fatalf("existing skill memory still contains repeated visual evidence: %d", strings.Count(text, "plugin_ui_reference_image"))
	}
	if len(digest.PluginGroups) != 1 || len(digest.PluginSkill) == 0 {
		t.Fatalf("digest lost compact skill structure: %+v", digest)
	}
}

func TestPluginLearningStageArtifactUsesStableSessionStageID(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.artifactRoot = t.TempDir()
	target := pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Demo Skill"}
	requestContext := map[string]any{"project_path": `D:\song\demo.vit`}

	first := server.recordPluginLearningStage("conv_learning", requestContext, "pl_session_test", target, "ui_reference", "provided", map[string]any{
		"visual_digest": map[string]any{"schema": "plugin_ui_reference_digest.v1"},
	})
	second := server.recordPluginLearningStage("conv_learning", requestContext, "pl_session_test", target, "ui_reference", "updated", map[string]any{
		"visual_digest": map[string]any{"schema": "plugin_ui_reference_digest.v1", "matching_status": "matched"},
	})
	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("stage artifact IDs should be stable, first=%+v second=%+v", first, second)
	}
	items, err := server.artifactStore().List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kind != "plugin_learning_stage" {
		t.Fatalf("stage artifacts = %+v", items)
	}
	if items[0].Metadata["plugin_learning_session_id"] != "pl_session_test" || items[0].Metadata["plugin_learning_stage"] != "ui_reference" || items[0].Metadata["plugin_learning_status"] != "updated" {
		t.Fatalf("stage metadata = %+v", items[0].Metadata)
	}
	if items[0].ProjectPath != `D:/song/demo.vit` {
		t.Fatalf("stage scope project path = %q", items[0].ProjectPath)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(items[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["schema"] != pluginLearningSessionSchema || payload["stage"] != "ui_reference" || payload["status"] != "updated" {
		t.Fatalf("stage payload = %+v", payload)
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

func TestPluginLearningPromptDigestForDraftCompactsOptionalEvidence(t *testing.T) {
	params := make([]pluginParameterInfo, 0, 140)
	for i := 0; i < 140; i++ {
		params = append(params, pluginParameterInfo{
			ID:               fmt.Sprintf("param_%03d", i),
			Name:             fmt.Sprintf("Very Long Backend Parameter Name %03d", i),
			DisplayGroup:     fmt.Sprintf("Group %02d", i/10),
			NormalizedRole:   "generic_role",
			HostControllable: true,
			DisplayProbe: &plugingrabber.ParameterDisplayProbe{
				CurrentText: "0.0 dB",
				Samples: []plugingrabber.ParameterDisplayProbeSample{
					{NormalizedValue: 0, Text: "-120.0 dB"},
					{NormalizedValue: 0.5, Text: "0.0 dB"},
					{NormalizedValue: 1, Text: "+24.0 dB"},
				},
				AllLabels: []string{
					strings.Repeat("oversized_label_", 300),
					strings.Repeat("another_oversized_label_", 300),
				},
			},
		})
	}
	digest := pluginParameterDigest{
		TrackID:        "track_1",
		PluginID:       "plugin_1",
		PluginName:     "Evidence Heavy Plugin",
		ParameterCount: len(params),
		Parameters:     params,
	}
	webReference := map[string]any{
		"schema": "plugin_web_reference_digest.v1",
		"status": "provided",
		"query":  "Evidence Heavy Plugin manual controls",
		"sources": []map[string]any{
			{"title": "Manual", "url": "https://example.test/manual", "text_excerpt": strings.Repeat("WEB_MANUAL_BLOB ", 12000)},
			{"title": "Support", "url": "https://example.test/support", "text_excerpt": strings.Repeat("WEB_SUPPORT_BLOB ", 12000)},
		},
	}
	typeHypothesis := map[string]any{
		"schema":       "plugin_type_hypothesis.v1",
		"status":       "provided",
		"primary_type": "hybrid",
		"core_control_expectations": []map[string]any{{
			"control":                "main amount",
			"why_it_matters":         strings.Repeat("important ", 300),
			"expected_display_clues": []string{"%", "dB"},
			"priority":               "primary",
			"confidence":             0.8,
		}},
	}
	matches := make([]map[string]any, 0, 140)
	controls := make([]map[string]any, 0, 140)
	for i := 0; i < 140; i++ {
		matches = append(matches, map[string]any{
			"visible_label":      fmt.Sprintf("Control %03d", i),
			"visible_group":      "Main",
			"display_domain":     strings.Repeat("VISUAL_RANGE_BLOB ", 100),
			"backend_param_id":   fmt.Sprintf("param_%03d", i),
			"backend_param_name": fmt.Sprintf("Very Long Backend Parameter Name %03d", i),
			"match_reason":       strings.Repeat("matched by visible label and backend display probe ", 20),
			"confidence":         0.9,
		})
		controls = append(controls, map[string]any{
			"label":         fmt.Sprintf("Control %03d", i),
			"display_value": strings.Repeat("VISIBLE_VALUE_BLOB ", 60),
			"group":         "Main",
			"unit":          "dB",
			"confidence":    0.8,
		})
	}
	uiReference := map[string]any{
		"status": "provided",
		"visual_digest": map[string]any{
			"schema":                      "plugin_ui_reference_digest.v1",
			"matching_status":             "matched",
			"candidate_parameter_matches": matches,
			"visible_controls":            controls,
		},
	}

	promptDigest := pluginLearningPromptDigestForDraft(digest, uiReference, webReference, typeHypothesis, true)
	data, err := json.MarshalIndent(promptDigest, "", "  ")
	if err != nil {
		t.Fatalf("marshal compact draft prompt: %v", err)
	}
	if len(data) >= pluginLearningDigestMaxBytes {
		t.Fatalf("compact draft prompt too large: %d", len(data))
	}
	prompt := string(data)
	if !strings.Contains(prompt, "param_000") || !strings.Contains(prompt, "param_139") {
		t.Fatalf("compact prompt dropped backend parameter IDs")
	}
	for _, marker := range []string{
		strings.Repeat("WEB_MANUAL_BLOB ", 20),
		strings.Repeat("VISUAL_RANGE_BLOB ", 20),
		strings.Repeat("oversized_label_", 20),
	} {
		if strings.Contains(prompt, marker) {
			t.Fatalf("compact prompt leaked bulky marker %q", marker)
		}
	}
}

func TestBuildPluginWebResearchDigestReturnsBoundedV2Evidence(t *testing.T) {
	var requestBody string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		requestBody = string(body)
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if !strings.Contains(requestBody, "plugin_web_reference_research_input.v1") || !strings.Contains(requestBody, "source_text_excerpt") {
			t.Fatalf("web digest prompt missing bounded source evidence: %s", requestBody)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"schema\":\"plugin_web_reference_digest.v2\",\"status\":\"provided\",\"sources\":[{\"title\":\"Manual\",\"url\":\"https://example.test/manual\",\"source_type\":\"official_manual\",\"confidence\":0.92}],\"type_hints\":[{\"type\":\"hybrid\",\"summary\":\"Manual describes multiple processing sections.\",\"source_url\":\"https://example.test/manual\",\"confidence\":0.7}],\"documented_core_controls\":[{\"control\":\"main amount\",\"description\":\"Documented as a primary adjustment.\",\"display_clues\":[\"dB\",\"%\"],\"priority\":\"primary\",\"source_url\":\"https://example.test/manual\",\"confidence\":0.8}],\"documented_units_and_ranges\":[{\"control\":\"main amount\",\"unit\":\"dB\",\"range_or_values\":\"documented range\",\"source_url\":\"https://example.test/manual\",\"confidence\":0.8}],\"ui_sections\":[{\"label\":\"Main\",\"description\":\"Main processing area.\",\"source_url\":\"https://example.test/manual\",\"confidence\":0.8}],\"warnings\":[]}"}}]}`))
	}))
	defer api.Close()

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: api.Client()}
	rawMarker := strings.Repeat("RAW_WEB_SOURCE ", 300)
	digest := server.buildPluginWebResearchDigest(context.Background(), config.EngineConfig{
		BaseURL:      api.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
	}, pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "Demo Plugin"}, pluginParameterDigest{
		PluginName:     "Demo Plugin",
		ParameterCount: 1,
		Parameters: []pluginParameterInfo{{
			ID:               "p_amount",
			Name:             "Amount",
			HostControllable: true,
		}},
	}, "Demo Plugin manual controls", []map[string]any{{
		"title": "Manual", "url": "https://example.test/manual", "snippet": "Manual snippet", "web_reference_score": 10,
	}}, []map[string]any{{
		"title": "Manual", "url": "https://example.test/manual", "text_excerpt": rawMarker, "source_rank": 1,
	}})
	if digest["schema"] != pluginLearningWebReferenceSchema || digest["status"] != "provided" {
		t.Fatalf("web digest = %+v", digest)
	}
	data, err := json.Marshal(digest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "RAW_WEB_SOURCE") || strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "source_text_excerpt") {
		t.Fatalf("public web digest leaked raw source text: %s", data)
	}
	if len(mapRowsValue(digest["documented_units_and_ranges"])) != 1 || len(mapRowsValue(digest["documented_core_controls"])) != 1 {
		t.Fatalf("web digest missing structured evidence: %+v", digest)
	}
}

func TestResolvePluginWebReferenceUsesOpenAIHostedSearchForGPT(t *testing.T) {
	var requestBody string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		requestBody = string(body)
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if !strings.Contains(requestBody, `"type":"web_search"`) || !strings.Contains(requestBody, "plugin_hosted_web_reference_request.v1") {
			t.Fatalf("hosted web search request missing tool or prompt: %s", requestBody)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"{\"schema\":\"plugin_web_reference_digest.v2\",\"status\":\"provided\",\"search_source\":\"openai_hosted_web_search\",\"sources\":[{\"title\":\"Official Manual\",\"url\":\"https://example.test/manual\",\"source_type\":\"official_manual\",\"confidence\":0.93}],\"type_hints\":[{\"type\":\"reverb\",\"summary\":\"Official manual describes reverb controls.\",\"source_url\":\"https://example.test/manual\",\"confidence\":0.82}],\"documented_core_controls\":[{\"control\":\"decay\",\"description\":\"Controls reverb tail length.\",\"display_clues\":[\"s\",\"seconds\"],\"priority\":\"primary\",\"source_url\":\"https://example.test/manual\",\"confidence\":0.8}],\"warnings\":[]}"}`))
	}))
	defer api.Close()

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: api.Client()}
	digest := server.resolvePluginWebReference(context.Background(), config.EngineConfig{
		BaseURL:      api.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "gpt-test",
	}, map[string]any{"web_reference_enabled": true}, pluginLearningTarget{
		TrackID:    "track_1",
		PluginID:   "plugin_1",
		PluginName: "Demo Verb",
	}, pluginParameterDigest{
		PluginName:     "Demo Verb",
		ParameterCount: 1,
		Parameters: []pluginParameterInfo{{
			ID:               "p_decay",
			Name:             "Decay",
			HostControllable: true,
		}},
	})
	if digest["status"] != "provided" || digest["search_source"] != "openai_hosted_web_search" {
		t.Fatalf("web digest = %+v", digest)
	}
	if len(mapRowsValue(digest["type_hints"])) != 1 || len(mapRowsValue(digest["documented_core_controls"])) != 1 {
		t.Fatalf("hosted digest missing evidence: %+v", digest)
	}
	if strings.Contains(fmt.Sprint(digest), "source_text_excerpt") {
		t.Fatalf("hosted digest leaked raw source fields: %+v", digest)
	}
}

func TestBuildPluginHostedWebReferenceRetriesJSONAfterEmptyResponsesStream(t *testing.T) {
	attempts := 0
	var retryBody map[string]any
	var retryAccept string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: response.completed\n"))
			_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n"))
			return
		}
		retryAccept = r.Header.Get("Accept")
		if err := json.NewDecoder(r.Body).Decode(&retryBody); err != nil {
			t.Fatalf("decode retry body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"{\"schema\":\"plugin_web_reference_digest.v2\",\"status\":\"not_found\",\"search_source\":\"openai_hosted_web_search\",\"sources\":[],\"type_hints\":[],\"documented_core_controls\":[],\"warnings\":[\"No relevant public documentation found.\"]}"}`))
	}))
	defer api.Close()

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: api.Client()}
	digest, warning := server.buildPluginHostedWebReference(context.Background(), config.EngineConfig{
		BaseURL:      api.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "gpt-test",
	}, pluginLearningTarget{TrackID: "track_1", PluginID: "plugin_1", PluginName: "No Docs Plugin"}, pluginParameterDigest{
		PluginName:     "No Docs Plugin",
		ParameterCount: 1,
		Parameters: []pluginParameterInfo{{
			ID:               "p_amount",
			Name:             "Amount",
			HostControllable: true,
		}},
	}, `"No Docs Plugin" audio plugin manual controls`)
	if warning != "" || attempts != 2 {
		t.Fatalf("warning=%q attempts=%d digest=%+v", warning, attempts, digest)
	}
	if digest["status"] != "not_found" || retryAccept != "application/json" || retryBody["stream"] != false {
		t.Fatalf("digest=%+v retryAccept=%q retryBody=%+v", digest, retryAccept, retryBody)
	}
}

func TestPluginWebReferenceRankedResultsRejectsTokyoTravelForTDRNova(t *testing.T) {
	target := pluginLearningTarget{PluginName: "TDR Nova"}
	digest := pluginParameterDigest{PluginName: "TDR Nova"}
	unrelated := []map[string]any{
		{"title": "17 Best Things to do in Tokyo, Japan", "url": "https://example.test/tokyo", "snippet": "Tokyo travel itinerary and city guide."},
		{"title": "Tokyo City Guide", "url": "https://example.test/guide", "snippet": "What to do in Tokyo."},
	}
	if got := pluginWebReferenceRankedResults(unrelated, target, digest); len(got) != 0 {
		t.Fatalf("unrelated results should be rejected: %+v", got)
	}
	relevant := []map[string]any{
		{"title": "TDR Nova Manual", "url": "https://docs.tokyodawn.net/tdr-nova", "snippet": "Tokyo Dawn Labs TDR Nova audio plugin manual."},
	}
	if got := pluginWebReferenceRankedResults(relevant, target, digest); len(got) != 1 {
		t.Fatalf("relevant result should be retained: %+v", got)
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

func TestChinesePluginInventoryQuestionUsesList(t *testing.T) {
	cmds := synthesizePluginLibraryCommands("当前插件库下有哪些效果器？")
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

func TestTakePendingPlanConsumesAgentLoopConfirmationOnlyOnce(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := PendingPlan{
		ID:           "plan_b1_clip_gain",
		Workflow:     agentLoopConfirmationWorkflow,
		WorkflowData: map[string]any{"conversation_id": "chat_b1"},
		Context:      map[string]any{"goal_id": "goal_b1", "run_id": "run_b1"},
	}
	server.pending[plan.ID] = plan

	got, ok, stale := server.takePendingPlan(plan.ID)
	if !ok || stale || got.ID != plan.ID {
		t.Fatalf("first takePendingPlan = plan=%+v ok=%v stale=%v", got, ok, stale)
	}
	if _, ok, _ := server.takePendingPlan(plan.ID); ok {
		t.Fatal("consumed B1 confirmation was returned a second time")
	}
	if pending, ok := server.pendingPlanForChat("chat_b1", map[string]any{"goal_id": "goal_b1"}); ok {
		t.Fatalf("consumed B1 confirmation still blocks chat: %+v", pending)
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

func TestPendingPlanForChatDoesNotUseOnlyPendingPlanFromOtherConversation(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.pending["plan_old_strip"] = PendingPlan{
		ID:        "plan_old_strip",
		Workflow:  agentLoopConfirmationWorkflow,
		CreatedAt: time.Now(),
		Context: map[string]any{
			"conversation_id": "chat_old",
			"goal_id":         "goal_old",
			"run_id":          "run_old",
		},
		WorkflowData: map[string]any{"conversation_id": "chat_old"},
		Decisions: []policy.Decision{{
			Name: "clip.strip_silence.apply",
			Risk: policy.RiskConfirm,
			Command: map[string]any{
				"cmd":      "clip.strip_silence.apply",
				"clip_id":  "old_clip",
				"track_id": "old_track",
				"strip_regions": []any{map[string]any{
					"clip_id":       "old_clip",
					"track_id":      "old_track",
					"start_seconds": 0.0,
					"end_seconds":   0.5,
				}},
			},
		}},
	}

	if pending, ok := server.pendingPlanForChat("chat_new", map[string]any{"goal_id": "goal_new"}); ok {
		t.Fatalf("unrelated pending plan should not block new chat: %+v", pending)
	}
	if pending, ok := server.pendingPlanForChat("chat_old", map[string]any{"goal_id": "goal_old"}); !ok || pending.ID != "plan_old_strip" {
		t.Fatalf("matching pending plan should still be available, got %+v ok=%v", pending, ok)
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
	typed := mapValue(req.Payload["typed_state"])
	if typed["kind"] != agentprotocol.KindApprovalRequest || typed["resource_domain"] != "project.commit" {
		t.Fatalf("typed approval missing or wrong: %+v", typed)
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

func TestAttachInteractionRequestsHydratesPendingConfirmationPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.mu.Lock()
	server.pending["plan_1"] = PendingPlan{
		ID:        "plan_1",
		CreatedAt: time.Now(),
		Context: map[string]any{
			"conversation_id": "chat_1",
			"goal_id":         "goal_1",
			"run_id":          "run_1",
		},
		Preview:  "preview",
		Workflow: "test_workflow",
		Decisions: []policy.Decision{{
			Name: "test_command",
			Risk: policy.RiskConfirm,
		}},
	}
	server.mu.Unlock()

	resp := ChatResponse{
		ConversationID: "chat_1",
		GoalID:         "goal_1",
		RunID:          "run_1",
		Reply:          "waiting for confirmation",
		GoalStatus:     string(agentruntime.StatusWaitingConfirmation),
	}
	server.attachInteractionRequests(&resp)
	if !resp.NeedsConfirmation || resp.PlanID != "plan_1" {
		t.Fatalf("confirmation state not hydrated: %+v", resp)
	}
	if len(resp.InteractionRequests) != 1 {
		t.Fatalf("interaction_requests = %+v", resp.InteractionRequests)
	}
	req := resp.InteractionRequests[0]
	if req.Kind != "confirmation" || req.Type != "confirmation" || req.Payload["plan_id"] != "plan_1" {
		t.Fatalf("request = %+v", req)
	}
	if len(req.Actions) != 2 || req.Actions[0].ID != "approve" || req.Actions[1].ID != "cancel" {
		t.Fatalf("actions = %+v", req.Actions)
	}
	if typed := mapValue(req.Payload["typed_state"]); typed["kind"] != agentprotocol.KindApprovalRequest {
		t.Fatalf("typed approval = %+v", typed)
	}
}

func TestMixTreatmentInteractionPayloadIncludesTypedPendingCandidate(t *testing.T) {
	payload := mixTreatmentInteractionPayload(agentloop.MixTreatmentPending{
		SchemaVersion:    "mix_treatment_pending.v0",
		Status:           "pending_confirmation",
		ConversationID:   "chat_1",
		TargetRef:        "track:vocal",
		ActionKind:       "gain_balance",
		ProcessorType:    "utility",
		DeltaDB:          -1,
		ReasoningSummary: "small gain move",
	})
	typed := mapValue(payload["typed_state"])
	if typed["kind"] != agentprotocol.KindPendingCandidate || typed["candidate_type"] != "mix_treatment" {
		t.Fatalf("typed pending = %+v", typed)
	}
	if typed["status"] != agentprotocol.PendingStatusWaitingUser {
		t.Fatalf("typed pending status = %+v", typed)
	}
}

func TestMixTreatmentInteractionPayloadLocalizesConfirmationDisplay(t *testing.T) {
	treatment := agentloop.MixTreatmentPending{
		SchemaVersion:    "mix_treatment_pending.v0",
		Status:           "pending_confirmation",
		ConversationID:   "chat_1",
		TargetRef:        "track:1007",
		ActionKind:       "plugin_treatment",
		ProcessorType:    "eq",
		Confidence:       "low",
		ReasoningSummary: "Sub/bass/low-mid band energies around -40 dB are close to mid at -38.56 dB while presence is much lower.",
		NeedsResolution:  []string{"request_more_observation", "plugin_profile"},
	}
	payload := mixTreatmentInteractionPayload(treatment)
	display := mapValue(payload["display"])
	reason := cleanContextText(display["reasoning_summary"])
	if !strings.Contains(reason, "低频/低中频") || strings.Contains(reason, "Sub/bass") || strings.Contains(reason, "band energies") {
		t.Fatalf("localized reasoning = %q", reason)
	}
	if display["confidence"] != "低" || payload["display_confidence"] != "低" {
		t.Fatalf("localized confidence display=%+v payload=%+v", display["confidence"], payload["display_confidence"])
	}
	needs := contextStringSlice(payload["display_needs_resolution"])
	if strings.Join(needs, ",") != "需要更多观察,插件控制映射" {
		t.Fatalf("localized needs = %+v", needs)
	}
	req := mixTreatmentInteractionRequest("chat_1", "goal_1", "run_1", treatment.ReasoningSummary, treatment)
	if strings.Contains(req.Body, "Sub/bass") || strings.Contains(req.Body, "band energies") || !strings.Contains(req.Body, "低频/低中频") {
		t.Fatalf("request body = %q", req.Body)
	}
	reply := localizedMixTreatmentUserReply("Sub 20-60 Hz is ready, Low-mid is partial, LUFS/masking/reference deferred.", treatment)
	for _, bad := range []string{"Sub", "Low-mid", "partial", "deferred", "masking/reference"} {
		if strings.Contains(reply, bad) {
			t.Fatalf("localized reply leaked %q in %q", bad, reply)
		}
	}
	for _, want := range []string{"超低频", "低中频", "部分可用", "掩蔽分析", "参考匹配", "暂未展开"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("localized reply missing %q in %q", want, reply)
		}
	}
}

func TestAttachInteractionRequestsAddsTypedUserInputRequest(t *testing.T) {
	resp := ChatResponse{
		InteractionRequests: []AgentInteractionRequest{{
			ID:             "interaction_form",
			Kind:           "form",
			Type:           "plugin_learning_display_domain_form",
			Source:         "plugin_grabber",
			Workflow:       "plugin_grabber_auto_learn",
			Stage:          "display_domain_form",
			Title:          "补充显示域",
			Body:           "请补充显示范围。",
			Status:         "waiting_for_user",
			ConversationID: "chat_1",
			Fields: []AgentInteractionField{{
				ID:   "range",
				Kind: "text",
			}},
		}},
	}
	server := New(nil, shadow.New(nil), nil)
	server.attachInteractionRequests(&resp)
	if len(resp.InteractionRequests) != 1 {
		t.Fatalf("interaction requests = %+v", resp.InteractionRequests)
	}
	typed := mapValue(resp.InteractionRequests[0].Payload["typed_state"])
	if typed["kind"] != agentprotocol.KindUserInputRequest || typed["status"] != agentprotocol.UserInputStatusWaitingUser {
		t.Fatalf("typed user input = %+v", typed)
	}
	if len(resp.TypedEvents) != 1 || mapValue(resp.TypedEvents[0])["event_type"] != agentprotocol.KindUserInputRequest {
		t.Fatalf("typed events = %+v", resp.TypedEvents)
	}
	server.attachInteractionRequests(&resp)
	if len(resp.TypedEvents) != 1 {
		t.Fatalf("typed events duplicated = %+v", resp.TypedEvents)
	}
}

func TestPluginPrepContinuationFromRepliesCreatesPendingParameterCandidate(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := PendingPlan{
		ID:       "plan_plugin_load",
		Workflow: pluginGrabberLoadCommand,
		Context: map[string]any{
			"conversation_id": "chat_mix",
			"goal_id":         "goal_1",
			"run_id":          "run_1",
			"user_message":    "reduce low-mid mud",
		},
		WorkflowData: map[string]any{
			"conversation_id":                "chat_mix",
			"mix_treatment_preparation":      true,
			"mix_treatment_preparation_plan": map[string]any{"schema_version": "mix_treatment_preparation.v0"},
		},
	}
	replies := []map[string]any{
		{
			"status":       "ok",
			"command_name": "rack_add_node",
			"result": map[string]any{
				"track_id":    "1007",
				"plugin_id":   "plugin_eq",
				"plugin_name": "Test EQ",
			},
		},
		{
			"status":       "ok",
			"command_name": "get_plugin_parameters",
			"result": map[string]any{
				"track_id":            "1007",
				"plugin_id":           "plugin_eq",
				"plugin_name":         "Test EQ",
				"parameter_count":     48,
				"quick_control_count": 6,
				"quick_controls":      testPluginPrepEQQuickControls(),
			},
		},
	}

	prep := server.pluginPrepContinuationFromReplies(plan, replies, "loaded")
	if prep.GoalStatus != string(agentruntime.StatusWaitingConfirmation) || prep.StopReason != pluginPrepWorkerCandidateStopReason {
		t.Fatalf("prep status = %+v", prep)
	}
	if len(prep.InteractionRequests) != 1 {
		t.Fatalf("interaction requests = %+v", prep.InteractionRequests)
	}
	req := prep.InteractionRequests[0]
	if req.Kind != "confirmation" || req.Type != pluginPrepParameterTreatmentType || req.Source != pluginPrepWorkerWorkflow || len(req.Actions) != 3 {
		t.Fatalf("candidate request = %+v", req)
	}
	candidate := mapValue(req.Payload["pending_candidate"])
	if candidate["kind"] != agentprotocol.KindPendingCandidate || candidate["candidate_type"] != pluginPrepParameterTreatmentType {
		t.Fatalf("pending candidate = %+v", candidate)
	}
	if !testTypedEventsContain(prep.TypedEvents, agentprotocol.KindTerminalResult) ||
		!testTypedEventsContain(prep.TypedEvents, agentprotocol.KindPendingCandidate) ||
		!testTypedEventsContain(prep.TypedEvents, agentprotocol.KindApprovalRequest) ||
		testTypedEventsContain(prep.TypedEvents, agentprotocol.KindUserInputRequest) {
		t.Fatalf("typed events = %+v", prep.TypedEvents)
	}
	terminal := testTypedEventState(prep.TypedEvents, agentprotocol.KindTerminalResult)
	if terminal["status"] != "parameters_extracted" {
		t.Fatalf("terminal state = %+v", terminal)
	}
	active := server.pendingManager.ActiveForConversation("chat_mix")
	if len(active) != 1 || active[0].CandidateType != pluginPrepParameterTreatmentType {
		t.Fatalf("active pending = %+v", active)
	}
	if stored, ok := server.takePendingInteraction(req.ID); !ok || stored.Type != pluginPrepParameterTreatmentType || stored.Payload["plugin_id"] != "plugin_eq" {
		t.Fatalf("stored interaction = %+v ok=%v", stored, ok)
	}
}

func TestPluginPrepWorkerChineseCardAndMissingBandEvidence(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := testPluginPrepWorkerPlan("chat_mix", true)
	plan.Context["user_message"] = "低频有点糊，帮我处理一下"
	replies := testPluginPrepWorkerReplies()

	prep := server.pluginPrepContinuationFromReplies(plan, replies, "已装载插件")
	if len(prep.InteractionRequests) != 1 {
		t.Fatalf("interaction requests = %+v", prep.InteractionRequests)
	}
	req := prep.InteractionRequests[0]
	if req.Title != "确认插件参数处理" {
		t.Fatalf("title = %q", req.Title)
	}
	if strings.Contains(req.Body, "Plugin Prep Worker prepared") ||
		strings.Contains(req.Body, "Apply candidate") ||
		!strings.Contains(req.Body, "未取得可靠的频段观测") ||
		!strings.Contains(req.Body, "保守试探") {
		t.Fatalf("body = %q", req.Body)
	}
	labels := []string{}
	for _, action := range req.Actions {
		labels = append(labels, action.Label)
	}
	if strings.Join(labels, ",") != "应用候选,调整候选,取消" {
		t.Fatalf("labels = %+v", labels)
	}
	action := mapValue(req.Payload["plugin_prep_worker"])
	if action["plugin_name"] != "TDR Nova" || action["target_ref"] != "track:1007" || action["treatment_type"] != "EQ 低频/低中频清理" {
		t.Fatalf("candidate action = %+v", action)
	}
	status := mapValue(action["evidence_status"])
	if status["band_energy"] != "missing" || status["strategy"] != "conservative_probe" || status["confidence_ceiling"] != "medium" {
		t.Fatalf("evidence_status = %+v", status)
	}
	if summary := cleanContextText(action["display_summary"]); !strings.Contains(summary, "未取得可靠的频段观测") {
		t.Fatalf("display_summary = %q", summary)
	}
	changes := mapRowsFromAny(action["parameter_change_summary"])
	if len(changes) != 1 {
		t.Fatalf("parameter_change_summary = %+v", changes)
	}
	if changes[0]["label"] != "B1 Gain" || changes[0]["target"] != "-1.5 dB" || changes[0]["current"] != "0.0 dB" {
		t.Fatalf("parameter change summary = %+v", changes[0])
	}
	if note := cleanContextText(changes[0]["frequency_note"]); !strings.Contains(note, "180 Hz") || !strings.Contains(note, "1.20") {
		t.Fatalf("frequency note = %q", note)
	}
}

func TestPluginPrepWorkerConfirmationBodyLocalizesEvidenceEnums(t *testing.T) {
	body := pluginPrepWorkerConfirmationBody(map[string]any{
		"plugin_prep_worker": map[string]any{
			"display_summary": "已根据诊断上下文中的频段能量观测和插件参数生成一个保守的 EQ 参数候选。确认前不会写入插件参数。",
			"target_ref":      "track:1007",
			"plugin_name":     "TDR Nova",
			"treatment_type":  "EQ 低频/低中频清理",
			"evidence_status": map[string]any{
				"band_energy":        "available",
				"strategy":           "request_more_observation",
				"confidence_ceiling": "low",
			},
			"evidence_summary": "pending_confirmation requires request_more_observation",
		},
	})
	for _, raw := range []string{"request_more_observation", "pending_confirmation", " low", "strategy"} {
		if strings.Contains(body, raw) {
			t.Fatalf("body leaked raw enum %q: %q", raw, body)
		}
	}
	if !strings.Contains(body, "策略：需要更多观察") || !strings.Contains(body, "置信度上限：低") {
		t.Fatalf("body missing localized evidence fields: %q", body)
	}
}

func TestPluginPrepWorkerBandEnergyReadyIsCited(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := testPluginPrepWorkerPlan("chat_mix", true)
	plan.Context["user_message"] = "低频有点糊，帮我处理一下"
	replies := testPluginPrepWorkerReplies()
	params := mapValue(replies[1]["result"])
	params["band_energy_summary"] = map[string]any{
		"status":   "ready",
		"source":   "live_level_meter_spectrum",
		"track_id": "1007",
		"bands": map[string]any{
			"bass":    "moderate",
			"low_mid": "elevated",
		},
	}

	prep := server.pluginPrepContinuationFromReplies(plan, replies, "已装载插件")
	if len(prep.InteractionRequests) != 1 {
		t.Fatalf("interaction requests = %+v", prep.InteractionRequests)
	}
	req := prep.InteractionRequests[0]
	action := mapValue(req.Payload["plugin_prep_worker"])
	status := mapValue(action["evidence_status"])
	if status["band_energy"] != "ready" || status["strategy"] != "band_observed_conservative" {
		t.Fatalf("evidence_status = %+v", status)
	}
	if summary := cleanContextText(action["evidence_summary"]); !strings.Contains(summary, "live_level_meter_spectrum") || !strings.Contains(summary, "low_mid=elevated") {
		t.Fatalf("evidence_summary = %q", summary)
	}
	refs := []string{}
	for _, ref := range action["evidence_refs"].([]string) {
		refs = append(refs, ref)
	}
	if !strings.Contains(strings.Join(refs, ","), "band_energy_summary:live_level_meter_spectrum") {
		t.Fatalf("evidence_refs = %+v", refs)
	}
	if strings.Contains(req.Body, "当前未取得可靠的频段观测") || !strings.Contains(req.Body, "频段能量观测可用") {
		t.Fatalf("body = %q", req.Body)
	}
}

func TestFinishPluginGrabberLoadWorkflowReusesExistingParameterResult(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := PendingPlan{
		ID:       "plan_plugin_load",
		Workflow: pluginGrabberLoadCommand,
		Context: map[string]any{
			"conversation_id": "chat_mix",
			"goal_id":         "goal_1",
			"run_id":          "run_1",
			"user_message":    "reduce low-mid mud",
		},
		WorkflowData: map[string]any{
			"conversation_id":           "chat_mix",
			"mix_treatment_preparation": true,
		},
	}
	replies := []map[string]any{
		{
			"status":       "ok",
			"command_name": "rack_add_node",
			"result": map[string]any{
				"track_id":    "1007",
				"plugin_id":   "plugin_eq",
				"plugin_name": "Test EQ",
			},
		},
		{
			"status":       "ok",
			"command_name": "get_plugin_parameters",
			"result": map[string]any{
				"status":              "ok",
				"track_id":            "1007",
				"plugin_id":           "plugin_eq",
				"plugin_name":         "Test EQ",
				"parameter_count":     48,
				"quick_control_count": 1,
				"quick_controls": []map[string]any{{
					"param_id":        "band_1_gain",
					"label":           "Band 1 Gain",
					"normalized_role": "eq_gain",
				}},
			},
		},
	}

	message, out := server.finishPluginGrabberLoadWorkflow(context.Background(), plan, replies, "loaded")
	if !strings.Contains(message, "未重复抓参数") {
		t.Fatalf("message did not report reused parameter fetch: %q", message)
	}
	if len(out) != len(replies) {
		t.Fatalf("finish appended duplicate replies: before=%d after=%d out=%+v", len(replies), len(out), out)
	}
	counts := map[string]int{}
	for _, row := range out {
		counts[cleanContextText(row["command_name"])]++
		switch cleanContextText(row["command_name"]) {
		case "set_plugin_param", "plugin_set_parameter", "plugin_grabber_apply_control":
			t.Fatalf("plugin prep finish wrote parameters unexpectedly: %+v", out)
		}
	}
	if counts["get_plugin_parameters"] != 1 {
		t.Fatalf("get_plugin_parameters count = %d, want 1; out=%+v", counts["get_plugin_parameters"], out)
	}
	prep := server.pluginPrepContinuationFromReplies(plan, out, message)
	if prep.GoalStatus != string(agentruntime.StatusWaitingConfirmation) || prep.StopReason != pluginPrepWorkerCandidateStopReason {
		t.Fatalf("prep = %+v", prep)
	}
	if len(prep.InteractionRequests) != 1 ||
		!testTypedEventsContain(prep.TypedEvents, agentprotocol.KindTerminalResult) ||
		!testTypedEventsContain(prep.TypedEvents, agentprotocol.KindPendingCandidate) ||
		testTypedEventsContain(prep.TypedEvents, agentprotocol.KindUserInputRequest) {
		t.Fatalf("prep did not produce terminal + pending candidate: %+v", prep)
	}
}

func TestPluginPrepContinuationWithoutParameterDigestReturnsTypedTerminalOnly(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := PendingPlan{
		ID:       "plan_plugin_load",
		Workflow: pluginGrabberLoadCommand,
		Context: map[string]any{
			"conversation_id": "chat_mix",
			"goal_id":         "goal_1",
			"run_id":          "run_1",
		},
		WorkflowData: map[string]any{
			"conversation_id":           "chat_mix",
			"mix_treatment_preparation": true,
		},
	}
	prep := server.pluginPrepContinuationFromReplies(plan, []map[string]any{
		{"command_name": "rack_add_node", "result": map[string]any{"track_id": "1007", "plugin_id": "plugin_eq", "plugin_name": "Test EQ"}},
	}, "loaded")

	if prep.GoalStatus != "" || len(prep.InteractionRequests) != 0 || prep.StopReason != "plugin_prep_terminal_no_parameter_digest" {
		t.Fatalf("prep = %+v", prep)
	}
	if !testTypedEventsContain(prep.TypedEvents, agentprotocol.KindTerminalResult) || testTypedEventsContain(prep.TypedEvents, agentprotocol.KindUserInputRequest) {
		t.Fatalf("typed events = %+v", prep.TypedEvents)
	}
	terminal := testTypedEventState(prep.TypedEvents, agentprotocol.KindTerminalResult)
	if terminal["status"] != "no_parameter_digest" {
		t.Fatalf("terminal state = %+v", terminal)
	}
}

func TestApplyPluginPrepContinuationResponseKeepsWaitingBridgeState(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := PendingPlan{
		ID:       "plan_plugin_load",
		Workflow: pluginGrabberLoadCommand,
		Context: map[string]any{
			"conversation_id": "chat_mix",
			"goal_id":         "goal_1",
			"run_id":          "run_1",
		},
		WorkflowData: map[string]any{
			"conversation_id":                "chat_mix",
			"mix_treatment_preparation":      true,
			"mix_treatment_preparation_plan": map[string]any{"schema_version": "mix_treatment_preparation.v0"},
		},
	}
	prep := server.pluginPrepContinuationFromReplies(plan, []map[string]any{
		{"command_name": "rack_add_node", "result": map[string]any{"track_id": "1007", "plugin_id": "plugin_eq", "plugin_name": "Test EQ"}},
		{"command_name": "get_plugin_parameters", "result": map[string]any{"track_id": "1007", "plugin_id": "plugin_eq", "plugin_name": "Test EQ", "parameter_count": 48, "quick_controls": testPluginPrepEQQuickControls()}},
	}, "loaded")
	response := map[string]any{
		"status":       "ok",
		"message":      "loaded",
		"goal_status":  string(agentruntime.StatusCompleted),
		"typed_events": []map[string]any{{"event_type": agentprotocol.KindApprovalRequest}},
	}

	response = applyPluginPrepContinuationResponse(response, prep)
	if response["goal_status"] != string(agentruntime.StatusWaitingConfirmation) || response["stop_reason"] != pluginPrepWorkerCandidateStopReason {
		t.Fatalf("response status = %+v", response)
	}
	if data := mapValue(response["workflow_data"]); data["plugin_id"] != "plugin_eq" || data["stage"] != "pending_confirmation" || data["type"] != pluginPrepParameterTreatmentType {
		t.Fatalf("workflow_data = %+v", data)
	}
	requests, ok := response["interaction_requests"].([]AgentInteractionRequest)
	if !ok || len(requests) != 1 || requests[0].Type != pluginPrepParameterTreatmentType {
		t.Fatalf("interaction_requests = %#v", response["interaction_requests"])
	}
	events := mapRowsFromAny(response["typed_events"])
	if !testTypedEventsContain(events, agentprotocol.KindApprovalRequest) ||
		!testTypedEventsContain(events, agentprotocol.KindTerminalResult) ||
		!testTypedEventsContain(events, agentprotocol.KindPendingCandidate) ||
		testTypedEventsContain(events, agentprotocol.KindUserInputRequest) {
		t.Fatalf("typed_events = %+v", events)
	}
}

func TestChatResponseFromAgentLoopResultPromotesAcousticPackageStatus(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	status := map[string]any{
		"schema_version":  "acoustic_package_status.v0",
		"status":          "partial",
		"project_id":      "current",
		"track_id":        "1007",
		"clip_id":         "1014",
		"source_revision": "rev_test",
		"package_layers": map[string]any{
			"l3_deep": map[string]any{"status": "building"},
		},
	}
	event := map[string]any{
		"event_type": agentprotocol.KindAcousticPackageStatus,
		"state_id":   "acoustic_package_status_current_1007_1014_rev_test",
		"status":     "partial",
	}

	resp := server.chatResponseFromAgentLoopResult("chat_acoustic", "mix", agentloop.Result{
		Status:     agentruntime.StatusCompleted,
		Reply:      "ok",
		StopReason: "done",
		Executed: []map[string]any{{
			"tool": "mix.observe",
			"result": map[string]any{
				"acoustic_package_status":      status,
				"acoustic_package_status_path": "D:\\Vit_DAW\\VitApp\\Workspace\\Artifacts\\acoustic_package_status.json",
				"typed_events":                 []map[string]any{event},
			},
		}},
	})

	if resp.AcousticPackageStatus["schema_version"] != "acoustic_package_status.v0" ||
		resp.AcousticPackageStatusPath == "" {
		t.Fatalf("acoustic status not promoted: %+v path=%q", resp.AcousticPackageStatus, resp.AcousticPackageStatusPath)
	}
	if !testTypedEventsContain(resp.TypedEvents, agentprotocol.KindAcousticPackageStatus) {
		t.Fatalf("typed events = %+v", resp.TypedEvents)
	}
}

func TestPluginPrepWorkerWithoutQuickControlsReturnsTerminalOnly(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := testPluginPrepWorkerPlan("chat_mix", true)
	prep := server.pluginPrepContinuationFromReplies(plan, []map[string]any{
		{"command_name": "rack_add_node", "result": map[string]any{"track_id": "1007", "plugin_id": "plugin_eq", "plugin_name": "Test EQ"}},
		{"command_name": "get_plugin_parameters", "result": map[string]any{"track_id": "1007", "plugin_id": "plugin_eq", "plugin_name": "Test EQ", "parameter_count": 48}},
	}, "loaded")

	if prep.GoalStatus != string(agentruntime.StatusCompleted) || prep.StopReason != pluginPrepWorkerTerminalNoMapping {
		t.Fatalf("prep = %+v", prep)
	}
	if len(prep.InteractionRequests) != 0 ||
		!testTypedEventsContain(prep.TypedEvents, agentprotocol.KindTerminalResult) ||
		testTypedEventsContain(prep.TypedEvents, agentprotocol.KindPendingCandidate) {
		t.Fatalf("typed events/interactions = %+v %+v", prep.TypedEvents, prep.InteractionRequests)
	}
}

func TestPluginPrepWorkerAmbiguousTargetReturnsUserInput(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := testPluginPrepWorkerPlan("chat_mix", false)
	prep := server.pluginPrepContinuationFromReplies(plan, []map[string]any{
		{"command_name": "get_plugin_parameters", "result": map[string]any{"plugin_id": "plugin_eq", "plugin_name": "Test EQ", "parameter_count": 48, "quick_controls": testPluginPrepEQQuickControls()}},
	}, "loaded")

	if prep.GoalStatus != string(agentruntime.StatusWaitingClarification) || prep.StopReason != pluginPrepWorkerTerminalAmbiguousTrack {
		t.Fatalf("prep = %+v", prep)
	}
	if len(prep.InteractionRequests) != 0 ||
		!testTypedEventsContain(prep.TypedEvents, agentprotocol.KindUserInputRequest) ||
		testTypedEventsContain(prep.TypedEvents, agentprotocol.KindPendingCandidate) {
		t.Fatalf("typed events/interactions = %+v %+v", prep.TypedEvents, prep.InteractionRequests)
	}
	if active := server.pendingManager.ActiveForConversation("chat_mix"); len(active) != 0 {
		t.Fatalf("ambiguous target should not create pending candidate: %+v", active)
	}
}

func TestPluginPrepWorkerDuplicateTriggerReusesPendingCandidate(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan := testPluginPrepWorkerPlan("chat_mix", true)
	replies := testPluginPrepWorkerReplies()

	first := server.pluginPrepContinuationFromReplies(plan, replies, "loaded")
	second := server.pluginPrepContinuationFromReplies(plan, replies, "loaded again")
	if len(first.InteractionRequests) != 1 || len(second.InteractionRequests) != 1 {
		t.Fatalf("interactions first=%+v second=%+v", first.InteractionRequests, second.InteractionRequests)
	}
	firstID := cleanContextText(first.InteractionRequests[0].Payload["pending_id"])
	secondID := cleanContextText(second.InteractionRequests[0].Payload["pending_id"])
	if firstID == "" || firstID != secondID {
		t.Fatalf("pending ids first=%q second=%q", firstID, secondID)
	}
	active := server.pendingManager.ActiveForConversation("chat_mix")
	if len(active) != 1 || active[0].ID != firstID {
		t.Fatalf("active pending = %+v", active)
	}
}

func TestPluginPrepWorkerConfirmationWritesAndReobserves(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "mix.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Vocal",
			"track_type":     "audio",
			"is_audio_track": true,
		}},
	})
	kernel := &recordingChatKernel{replyByCommand: map[string][]map[string]any{
		"set_plugin_param": {{
			"status":         "ok",
			"track_id":       "1007",
			"plugin_id":      "plugin_eq",
			"param_id":       "2",
			"new_value_text": "-1.5 dB",
		}},
	}}
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(kernel, shadowProject, nil)
	if !server.harness.ObservePluginParametersReply(testPluginPrepFullParameterSnapshot()) {
		t.Fatal("parameter snapshot did not load")
	}
	plan := testPluginPrepWorkerPlan("chat_mix", true)
	plan.Context["observation_id"] = "obs_before_plugin"
	plan.Context["mix_session_id"] = "mix_plugin_before"
	prep := server.pluginPrepContinuationFromReplies(plan, testPluginPrepWorkerReplies(), "loaded")
	if len(prep.InteractionRequests) != 1 {
		t.Fatalf("prep interactions = %+v", prep.InteractionRequests)
	}
	action := mapValue(prep.InteractionRequests[0].Payload["plugin_prep_worker"])
	if cleanContextText(action["before_observation_id"]) != "obs_before_plugin" || cleanContextText(action["mix_session_id"]) != "mix_plugin_before" {
		t.Fatalf("candidate should carry before observation context: %#v", action)
	}

	body, _ := json.Marshal(InteractionRespondRequest{
		InteractionID: prep.InteractionRequests[0].ID,
		ActionID:      "approve",
		Decision:      "approve",
	})
	rec := httptest.NewRecorder()
	server.handleInteractionRespond(rec, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if resp.StopReason != pluginPrepWorkerAppliedStopReason || resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("resp = %+v", resp)
	}
	if len(resp.ExecutedKernelReply) < 2 {
		t.Fatalf("executed = %+v", resp.ExecutedKernelReply)
	}
	if cleanContextText(resp.ExecutedKernelReply[0]["command_name"]) != "set_plugin_param" {
		t.Fatalf("first executed = %+v", resp.ExecutedKernelReply[0])
	}
	if cleanContextText(resp.ExecutedKernelReply[len(resp.ExecutedKernelReply)-1]["command_name"]) != "mix_observe" {
		t.Fatalf("reobserve missing: %+v", resp.ExecutedKernelReply)
	}
	if !strings.Contains(resp.Reply, "AB Result：不可信") || !strings.Contains(resp.Reply, "未把这次改动标记为已验证") {
		t.Fatalf("plugin prep confirmation reply should surface AB verification state, got %q", resp.Reply)
	}
	if len(resp.ProjectResultCards) != 1 {
		t.Fatalf("plugin prep project result card should surface AB state: %+v", resp.ProjectResultCards)
	}
	abCard := mapValue(resp.ProjectResultCards[0]["ab_result"])
	if cleanContextText(abCard["status"]) == "ready" || cleanContextText(abCard["reason"]) == "" {
		t.Fatalf("plugin prep project result card should surface AB state: %+v", resp.ProjectResultCards)
	}
	var setCommands []map[string]any
	for _, cmd := range kernel.commands {
		if cleanContextText(cmd["cmd"]) == "set_plugin_param" {
			setCommands = append(setCommands, cmd)
		}
		if cleanContextText(cmd["cmd"]) == "daw.invoke" || cleanContextText(cmd["cmd"]) == "n_apply_control" {
			t.Fatalf("unexpected fallback command: %+v", kernel.commands)
		}
	}
	if len(setCommands) != 1 || cleanContextText(setCommands[0]["param_id"]) != "2" || cleanContextText(setCommands[0]["value_text"]) != "-1.5 dB" {
		t.Fatalf("set commands = %+v all=%+v", setCommands, kernel.commands)
	}
	if active := server.pendingManager.ActiveForConversation("chat_mix"); len(active) != 0 {
		t.Fatalf("candidate should be consumed: %+v", active)
	}
}

func TestPluginPrepWorkerSetParameterStillGuarded(t *testing.T) {
	kernel := &recordingChatKernel{replyByCommand: map[string][]map[string]any{
		"set_plugin_param": {{"status": "ok"}},
	}}
	server := New(nil, shadow.New(nil), nil)
	server.harness = harness.NewWithSender(kernel, shadow.New(nil), nil)
	prep := server.pluginPrepContinuationFromReplies(testPluginPrepWorkerPlan("chat_mix", true), testPluginPrepWorkerReplies(), "loaded")
	if len(prep.InteractionRequests) != 1 {
		t.Fatalf("prep interactions = %+v", prep.InteractionRequests)
	}

	body, _ := json.Marshal(InteractionRespondRequest{
		InteractionID: prep.InteractionRequests[0].ID,
		ActionID:      "approve",
		Decision:      "approve",
	})
	rec := httptest.NewRecorder()
	server.handleInteractionRespond(rec, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))

	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if resp.GoalStatus != string(agentruntime.StatusFailed) || resp.StopReason != "plugin_prep_parameter_treatment_failed" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("guarded set_plugin_param should not reach kernel: %+v", kernel.commands)
	}
	if !testTypedEventsContain(resp.TypedEvents, agentprotocol.KindPendingCandidate) || !testTypedEventsContain(resp.TypedEvents, agentprotocol.KindTerminalResult) {
		t.Fatalf("typed events = %+v", resp.TypedEvents)
	}
}

func TestPluginPrepWorkerPlainApprovalWithoutPendingDoesNotWrite(t *testing.T) {
	kernel := &recordingChatKernel{}
	server := New(nil, shadow.New(nil), nil)
	server.harness = harness.NewWithSender(kernel, shadow.New(nil), nil)
	resp, handled := server.handlePendingPluginParameterTreatmentChat(context.Background(), "chat_mix", ChatRequest{Message: "confirm"}, agentModeDefault)
	if handled || len(kernel.commands) != 0 {
		t.Fatalf("handled=%v resp=%+v commands=%+v", handled, resp, kernel.commands)
	}
}

func testPluginPrepWorkerPlan(conversationID string, includeTrack bool) PendingPlan {
	workflowData := map[string]any{
		"conversation_id":           conversationID,
		"mix_treatment_preparation": true,
	}
	if includeTrack {
		workflowData["track_id"] = "1007"
	}
	return PendingPlan{
		ID:       "plan_plugin_load",
		Workflow: pluginGrabberLoadCommand,
		Context: map[string]any{
			"conversation_id": conversationID,
			"goal_id":         "goal_1",
			"run_id":          "run_1",
			"user_message":    "reduce low-mid mud",
		},
		WorkflowData: workflowData,
	}
}

func testPluginPrepWorkerReplies() []map[string]any {
	return []map[string]any{
		{
			"status":       "ok",
			"command_name": "rack_add_node",
			"result": map[string]any{
				"track_id":    "1007",
				"plugin_id":   "plugin_eq",
				"plugin_name": "TDR Nova",
			},
		},
		{
			"status":       "ok",
			"command_name": "get_plugin_parameters",
			"result": map[string]any{
				"status":              "ok",
				"track_id":            "1007",
				"plugin_id":           "plugin_eq",
				"plugin_name":         "TDR Nova",
				"parameter_count":     78,
				"quick_control_count": 4,
				"quick_controls":      testPluginPrepEQQuickControls(),
			},
		},
	}
}

func testPluginPrepEQQuickControls() []map[string]any {
	return []map[string]any{
		{"display_group": "Tone", "label": "B1 Freq", "normalized_role": "eq_frequency", "param_id": "4", "current_value_text": "180 Hz"},
		{"display_group": "Tone", "label": "B1 Gain", "normalized_role": "eq_gain", "param_id": "2", "current_value_text": "0.0 dB"},
		{"display_group": "Tone", "label": "B1 Q", "normalized_role": "eq_q", "param_id": "3", "current_value_text": "1.20"},
		{"display_group": "Mix", "label": "Dry Mix", "normalized_role": "dry_mix", "param_id": "64"},
	}
}

func testPluginPrepFullParameterSnapshot() map[string]any {
	return map[string]any{
		"status":      "ok",
		"track_id":    "1007",
		"plugin_id":   "plugin_eq",
		"plugin_name": "TDR Nova",
		"parameters": []map[string]any{
			{"id": "2", "param_id": "2", "name": "B1 Gain"},
			{"id": "3", "param_id": "3", "name": "B1 Q"},
			{"id": "4", "param_id": "4", "name": "B1 Freq"},
			{"id": "64", "param_id": "64", "name": "Dry Mix"},
		},
	}
}

func TestObservationToolEventsAttachTypedRequestAndResult(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	in := executor.Input{
		GoalID: "goal_1",
		RunID:  "run_1",
		Context: map[string]any{
			"conversation_id": "chat_obs",
		},
		ToolCall: planner.ToolCall{
			ID:     "tool_1",
			Tool:   "mix.observe",
			Args:   map[string]any{"track_id": "1007", "goal_text": "reduce low-mid mud"},
			Reason: "inspect the current mix",
		},
		Source: "agentloop",
	}

	server.emitToolItemStarted(in, "tool_1")
	server.emitToolItemCompleted(in, executor.Result{
		ToolCallID:  "tool_1",
		Tool:        "mix.observe",
		CommandName: "mix_observe",
		Status:      "ok",
		Result: map[string]any{
			"observation_id": "obs_1",
			"summary":        "low-mid buildup",
		},
	}, nil)

	events, _ := server.agentEventsSince("chat_obs", 0, 10)
	if len(events) != 2 {
		t.Fatalf("events = %+v", events)
	}
	started := mapValue(events[0].Payload["typed_state"])
	if events[0].Type != "item.started" || started["kind"] != agentprotocol.KindObservationRequest || started["target_ref"] != "track:1007" {
		t.Fatalf("started event = %+v typed=%+v", events[0], started)
	}
	if strings.Contains(events[0].Body, "deterministic observation") || strings.Contains(events[0].Body, "inspect the current mix") || !strings.Contains(events[0].Body, "混音") {
		t.Fatalf("started event body was not localized: %q", events[0].Body)
	}
	completed := mapValue(events[1].Payload["typed_state"])
	if events[1].Type != "item.completed" || completed["kind"] != agentprotocol.KindObservationResult || completed["context_pack_id"] != "obs_1" {
		t.Fatalf("completed event = %+v typed=%+v", events[1], completed)
	}
	if events[1].Title != "已完成 混音观察" {
		t.Fatalf("completed event title was not localized: %q", events[1].Title)
	}
	if mapValue(events[0].Payload["typed_event"])["event_type"] != agentprotocol.KindObservationRequest ||
		mapValue(events[1].Payload["typed_event"])["event_type"] != agentprotocol.KindObservationResult {
		t.Fatalf("typed events missing: started=%+v completed=%+v", events[0].Payload, events[1].Payload)
	}
}

func TestAttachInteractionsToResponseMapAddsPlainConfirmation(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.mu.Lock()
	server.pending["plan_next"] = PendingPlan{
		ID:        "plan_next",
		CreatedAt: time.Now(),
		Context: map[string]any{
			"conversation_id": "chat_1",
			"goal_id":         "goal_1",
			"run_id":          "run_1",
		},
		Preview: "next preview",
		Decisions: []policy.Decision{{
			Name: "next_command",
			Risk: policy.RiskConfirm,
		}},
	}
	server.mu.Unlock()

	response := map[string]any{
		"message":            "next confirmation",
		"needs_confirmation": true,
		"plan_id":            "plan_next",
		"preview":            "next preview",
		"goal_status":        string(agentruntime.StatusWaitingConfirmation),
	}
	server.attachInteractionsToResponseMap(&response, PendingInteraction{
		ID:             "interaction_previous",
		Kind:           "confirmation",
		ConversationID: "chat_1",
		GoalID:         "goal_1",
		RunID:          "run_1",
	})

	requests, ok := response["interaction_requests"].([]AgentInteractionRequest)
	if !ok || len(requests) != 1 {
		t.Fatalf("interaction_requests = %#v", response["interaction_requests"])
	}
	req := requests[0]
	if req.Kind != "confirmation" || req.PlanID != "plan_next" || req.Payload["plan_id"] != "plan_next" {
		t.Fatalf("request = %+v", req)
	}
	if len(req.Actions) != 2 || req.Actions[0].ID != "approve" || req.Actions[1].ID != "cancel" {
		t.Fatalf("actions = %+v", req.Actions)
	}
}

func testTypedEventsContain(events []map[string]any, eventType string) bool {
	for _, event := range events {
		if cleanContextText(event["event_type"]) == eventType {
			return true
		}
	}
	return false
}

func testTypedEventState(events []map[string]any, eventType string) map[string]any {
	for _, event := range events {
		if cleanContextText(event["event_type"]) == eventType {
			return mapValue(event["state"])
		}
	}
	return nil
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
	typed := mapValue(req.Payload["typed_state"])
	if typed["kind"] != agentprotocol.KindUserInputRequest {
		t.Fatalf("typed user input missing for plugin learning review: %+v", typed)
	}
	if len(resp.TypedEvents) != 1 || mapValue(resp.TypedEvents[0])["event_type"] != agentprotocol.KindUserInputRequest {
		t.Fatalf("typed events = %+v", resp.TypedEvents)
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

func TestPluginProfilePatchDisplayDomainEnrichmentProtectsSteppedAndPercentControls(t *testing.T) {
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{ID: "1", Name: "DelaySync", HostControllable: true, ValueText: "Msec"},
			{ID: "2", Name: "DelayNote", HostControllable: true, ValueText: "1/16"},
			{ID: "4", Name: "DelayWarp", HostControllable: true, ValueText: "0.0 %"},
		},
	}
	patch := pluginProfilePatch{
		Groups: []map[string]any{{
			"id":    "delay",
			"label": "Delay",
			"params": map[string]any{
				"sync": map[string]any{"param_id": "1", "label": "Delay Sync"},
				"note": map[string]any{"param_id": "2", "label": "Delay Note"},
				"warp": map[string]any{"param_id": "4", "label": "Warp"},
			},
		}},
	}
	patch = enrichPluginProfilePatchDisplayDomains(patch, digest)
	params := mapValue(patch.Groups[0]["params"])
	sync := mapValue(params["sync"])
	note := mapValue(params["note"])
	warp := mapValue(params["warp"])
	if got := firstNonEmptyText(sync, "display_domain_text"); got != "Msec" {
		t.Fatalf("sync display domain = %q mapping=%+v", got, sync)
	}
	if unit := firstNonEmptyText(mapValue(sync["display_domain"]), "unit"); unit != "enum" {
		t.Fatalf("sync unit = %q mapping=%+v", unit, sync)
	}
	if got := firstNonEmptyText(note, "display_domain_text"); got != "1/16" {
		t.Fatalf("note display domain = %q mapping=%+v", got, note)
	}
	if unit := firstNonEmptyText(mapValue(note["display_domain"]), "unit"); unit != "enum" {
		t.Fatalf("note unit = %q mapping=%+v", unit, note)
	}
	if got := firstNonEmptyText(warp, "display_domain_text"); got != "0~100 %" {
		t.Fatalf("warp display domain = %q mapping=%+v", got, warp)
	}
	if unit := firstNonEmptyText(mapValue(warp["display_domain"]), "unit"); unit != "%" {
		t.Fatalf("warp unit = %q mapping=%+v", unit, warp)
	}
}

func TestPluginProfilePatchDisplayDomainEnrichmentPrefersStructuredRangeOverVisualProse(t *testing.T) {
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{ID: "3", Name: "Delay_Ms", Alias: "Delay", HostControllable: true, ValueText: "300.0 ms"},
		},
	}
	patch := pluginProfilePatch{
		Groups: []map[string]any{{
			"id":    "network",
			"label": "Delay Network",
			"params": map[string]any{
				"delay": map[string]any{
					"param_id":                   "3",
					"label":                      "Delay",
					"display_domain_text":        "milliseconds numeric readout under large rotary knob ms",
					"visual_display_domain_text": "milliseconds numeric readout under large rotary knob ms",
					"display_domain": map[string]any{
						"text":       "milliseconds numeric readout under large rotary knob ms",
						"unit":       "ms",
						"source":     "auto_learn_user_review",
						"status":     "needs_confirmation",
						"confidence": 1,
					},
				},
			},
		}},
	}

	patch = enrichPluginProfilePatchDisplayDomains(patch, digest)
	mapping := mapValue(mapValue(patch.Groups[0]["params"])["delay"])
	if got := firstNonEmptyText(mapping, "display_domain_text"); got != "0~2000 ms" {
		t.Fatalf("structured delay range should win, got %q mapping=%+v", got, mapping)
	}
	if got := firstNonEmptyText(mapping, "visual_display_domain_text"); got != "milliseconds numeric readout under large rotary knob ms" {
		t.Fatalf("visual prose evidence should be retained, got %q mapping=%+v", got, mapping)
	}
	if mapping["display_domain_structured_replacement"] != true || mapping["display_domain_previous_text"] == "" {
		t.Fatalf("structured replacement metadata missing: %+v", mapping)
	}
}

func TestVisualDisplayDomainReconciliationOverridesConflictingNamePatternDomain(t *testing.T) {
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{ID: "4", Name: "DelayWarp", HostControllable: true, ValueText: "0.0 %"},
		},
	}
	patch := pluginProfilePatch{
		Groups: []map[string]any{{
			"id":    "delay",
			"label": "Delay",
			"params": map[string]any{
				"warp": map[string]any{
					"param_id":                   "4",
					"label":                      "Warp",
					"display_domain_text":        "0~2000 ms",
					"display_domain":             map[string]any{"text": "0~2000 ms", "unit": "ms", "source": "auto_learn_name_pattern", "confidence": 0.72},
					"visual_display_domain_text": "",
				},
			},
		}},
	}
	uiReference := map[string]any{
		"visual_digest": map[string]any{
			"schema":          "plugin_ui_reference_digest.v1",
			"matching_status": "target_probe_observed",
			"target_results": []map[string]any{{
				"target_id":     "target_06",
				"control":       "Warp",
				"status":        "visible",
				"visible_label": "WARP",
				"visible_group": "Warp",
				"visible_unit":  "%",
				"value_text":    "0.0 %",
				"display_style": "knob",
				"scale_hint":    "unipolar",
				"confidence":    0.98,
			}},
		},
	}
	patch = reconcilePluginProfilePatchVisualDisplayDomains(patch, digest, uiReference)
	mapping := mapValue(mapValue(patch.Groups[0]["params"])["warp"])
	if got := firstNonEmptyText(mapping, "display_domain_text"); got != "0~100 %" {
		t.Fatalf("visual display domain should win, got %q mapping=%+v", got, mapping)
	}
	if unit := firstNonEmptyText(mapValue(mapping["display_domain"]), "unit"); unit != "%" {
		t.Fatalf("visual unit should win, got %q mapping=%+v", unit, mapping)
	}
	if mapping["display_domain_previous_text"] != "0~2000 ms" || mapping["display_domain_conflict_resolved"] != true {
		t.Fatalf("conflict metadata missing: %+v", mapping)
	}
}

func TestVisualDisplayDomainReconciliationRequiresIdentityAndUnitAlignment(t *testing.T) {
	digest := pluginParameterDigest{
		Parameters: []pluginParameterInfo{
			{ID: "3", Name: "Delay_Ms", Alias: "Delay", HostControllable: true, ValueText: "300.0 ms"},
			{ID: "4", Name: "DelayWarp", Alias: "Warp", HostControllable: true, ValueText: "0.0 %"},
		},
	}
	patch := pluginProfilePatch{
		Groups: []map[string]any{{
			"id":    "network",
			"label": "Delay Network",
			"params": map[string]any{
				"delay": map[string]any{
					"param_id":            "3",
					"label":               "Delay",
					"display_domain_text": "0~2000 ms",
					"display_domain":      map[string]any{"text": "0~2000 ms", "unit": "ms", "source": "auto_learn_name_pattern", "confidence": 0.72},
				},
				"warp": map[string]any{
					"param_id":            "4",
					"label":               "Warp",
					"display_domain_text": "0~2000 ms",
					"display_domain":      map[string]any{"text": "0~2000 ms", "unit": "ms", "source": "auto_learn_name_pattern", "confidence": 0.72},
				},
			},
		}},
		VirtualControls: []map[string]any{{
			"component_id": "network",
			"name":         "make rhythmic echoes clearer",
			"params": map[string]any{
				"delay": map[string]any{
					"param_id":            "3",
					"label":               "Delay",
					"display_domain_text": "0~2000 ms",
					"display_domain":      map[string]any{"text": "0~2000 ms", "unit": "ms", "source": "auto_learn_name_pattern", "confidence": 0.72},
				},
				"warp": map[string]any{
					"param_id":            "4",
					"label":               "Warp",
					"display_domain_text": "0~2000 ms",
					"display_domain":      map[string]any{"text": "0~2000 ms", "unit": "ms", "source": "auto_learn_name_pattern", "confidence": 0.72},
				},
			},
		}},
	}
	uiReference := map[string]any{
		"visual_digest": map[string]any{
			"schema":          "plugin_ui_reference_digest.v1",
			"matching_status": "target_probe_observed",
			"target_results": []map[string]any{
				{
					"target_id":      "target_delay",
					"control":        "Delay time",
					"status":         "visible",
					"visible_label":  "DELAY",
					"visible_group":  "central Delay/Warp panel",
					"visible_unit":   "ms",
					"value_text":     "300.0 ms",
					"display_domain": "milliseconds numeric readout beneath large delay knob",
					"display_style":  "knob",
					"confidence":     0.98,
				},
				{
					"target_id":      "target_warp",
					"control":        "Warp",
					"status":         "visible",
					"visible_label":  "WARP",
					"visible_group":  "central Delay/Warp panel",
					"visible_unit":   "%",
					"value_text":     "0.0 %",
					"display_domain": "percentage value shown beneath large warp knob",
					"display_style":  "knob",
					"scale_hint":     "unipolar",
					"confidence":     0.98,
				},
			},
		},
	}

	patch = reconcilePluginProfilePatchVisualDisplayDomains(patch, digest, uiReference)
	groupParams := mapValue(patch.Groups[0]["params"])
	groupWarp := mapValue(groupParams["warp"])
	groupDelay := mapValue(groupParams["delay"])
	virtualParams := mapValue(patch.VirtualControls[0]["params"])
	virtualWarp := mapValue(virtualParams["warp"])

	if got := firstNonEmptyText(groupDelay, "display_domain_text"); got != "0~2000 ms" {
		t.Fatalf("delay domain should remain ms, got %q mapping=%+v", got, groupDelay)
	}
	if got := firstNonEmptyText(groupWarp, "display_domain_text"); got != "0~100 %" {
		t.Fatalf("group warp should use its own percent visual evidence, got %q mapping=%+v", got, groupWarp)
	}
	if unit := firstNonEmptyText(mapValue(groupWarp["display_domain"]), "unit"); unit != "%" {
		t.Fatalf("group warp unit = %q mapping=%+v", unit, groupWarp)
	}
	if got := firstNonEmptyText(virtualWarp, "display_domain_text"); got != "0~100 %" {
		t.Fatalf("virtual warp should not inherit delay ms evidence, got %q mapping=%+v", got, virtualWarp)
	}
	if firstNonEmptyText(virtualWarp, "visual_display_domain_text") != "0~100 %" {
		t.Fatalf("virtual warp visual domain missing: %+v", virtualWarp)
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

func TestAgentLoopClarificationReplyResumesConversationGoal(t *testing.T) {
	var prompts []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("LLM path = %q", r.URL.Path)
		}
		var req struct {
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode LLM request: %v", err)
		}
		var prompt strings.Builder
		for _, msg := range req.Messages {
			prompt.WriteString(msg.Content)
			prompt.WriteString("\n")
		}
		prompts = append(prompts, prompt.String())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"{\"final\":true,\"needs_clarification\":true,\"reply\":\"需要先确认哪条是主唱轨。\",\"tool_calls\":[]}"}}],
			"usage":{"prompt_tokens":11,"completion_tokens":3}
		}`))
	}))
	defer api.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfgPath := filepath.Join(home, ".vit", "config.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(fmt.Sprintf(`{"baseUrl":%q,"apiKey":"test","defaultModel":"test-model"}`, api.URL))
	if err := os.WriteFile(cfgPath, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	server := New(nil, shadow.New(nil), nil)
	goal := server.harness.BeginGoal("让主唱更靠前")
	server.recordGoalResult("chat_focus", agentloop.Result{
		GoalID:                goal.GoalID,
		RunID:                 goal.RunID,
		Status:                agentruntime.StatusWaitingClarification,
		NeedsClarification:    true,
		ClarificationQuestion: "哪条是主唱轨？",
		Continuation: &agentloop.Continuation{
			GoalID:   goal.GoalID,
			RunID:    goal.RunID,
			UserText: "让主唱更靠前",
			Summary:  "让主唱更靠前",
			Context:  map[string]any{"conversation_id": "chat_focus"},
			Budget:   agentloop.Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 2},
		},
	})
	server.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusWaitingClarification, nil)

	body, _ := json.Marshal(ChatRequest{
		ConversationID: "chat_focus",
		Message:        "Track 1 是主唱",
		Context:        map[string]any{"goal_id": nil, "run_id": nil},
	})
	rec := httptest.NewRecorder()
	server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.GoalID != goal.GoalID {
		t.Fatalf("goal_id = %q, want resumed %q; resp=%+v", resp.GoalID, goal.GoalID, resp)
	}
	if len(prompts) != 1 {
		t.Fatalf("LLM prompts = %d", len(prompts))
	}
	if !strings.Contains(prompts[0], "Current Goal: 让主唱更靠前") || !strings.Contains(prompts[0], "User clarification: Track 1 是主唱") {
		t.Fatalf("clarification prompt did not preserve original goal and answer:\n%s", prompts[0])
	}
}

func TestAgentLoopPendingDecisionsIncludesPendingToolCommand(t *testing.T) {
	decisions := agentLoopPendingDecisions(&agentloop.Continuation{
		PendingToolCall: &planner.ToolCall{
			Tool: "midi.apply_note_patch",
			Args: map[string]any{
				"clip_id": "clip_a",
				"operations": []any{
					map[string]any{"op": "insert_note", "pitch": 60, "start": 0, "length": 1},
				},
			},
			Reason: "写入 1 个音符",
		},
	})
	if len(decisions) != 1 {
		t.Fatalf("decisions = %+v", decisions)
	}
	decision := decisions[0]
	if decision.Name != "apply_midi_note_patch" || decision.Command["cmd"] != "apply_midi_note_patch" || decision.Command["clip_id"] != "clip_a" {
		t.Fatalf("decision = %+v", decision)
	}
	if ops, ok := decision.Command["operations"].([]any); !ok || len(ops) != 1 {
		t.Fatalf("operations = %#v", decision.Command["operations"])
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
