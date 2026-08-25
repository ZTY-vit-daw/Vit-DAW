package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/artifacts"
	"vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/policy"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
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

func TestCompactHistoryListProjectHistoryPreservesLightweightTranscript(t *testing.T) {
	largePayload := strings.Repeat("x", 2<<20)
	historyMap := map[string]any{
		"available":    true,
		"project_path": `D:\\songs\\mix.vit`,
		"project_uuid": "vitproj_test",
		"conversation_messages": []history.ConversationMessage{
			{Role: "user", Content: "进行 B1", NodeID: "node-1", CommitID: "commit-1"},
			{Role: "assistant", Content: "B1 已完成", NodeID: "node-2", CommitID: "commit-2", MessageData: map[string]any{"payload": largePayload}},
		},
		"conversation_graph": history.ConversationGraph{
			ProjectPath: `D:\\songs\\mix.vit`, ProjectUUID: "vitproj_test", ActiveNodeID: "node-2", ActiveBranch: "main",
			Nodes: []history.ConversationNode{
				{ID: "node-1", Kind: "ask", Text: "进行 B1", CommitID: "commit-1"},
				{ID: "node-2", Kind: "vit", Text: "B1 已完成", CommitID: "commit-2", MessageData: map[string]any{"payload": largePayload}},
			},
		},
	}

	got := compactHistoryListProjectHistory(historyMap)
	messages := dictionaryRowsFromAny(got["conversation_messages"])
	if len(messages) != 2 || firstStringFromMap(messages[0], "content") != "进行 B1" || firstStringFromMap(messages[1], "content") != "B1 已完成" {
		t.Fatalf("lightweight transcript = %+v", messages)
	}
	if _, ok := messages[1]["message_data"]; ok {
		t.Fatal("history transcript retained large message_data")
	}
	graph := mapValue(got["conversation_graph"])
	nodes := mapRowsFromAny(graph["nodes"])
	if len(nodes) != 2 || firstStringFromMap(nodes[1], "text_preview") != "B1 已完成" {
		t.Fatalf("compact graph = %+v", graph)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= 1<<20 || strings.Contains(string(encoded), largePayload[:1024]) {
		t.Fatalf("history list transport remained too large: %d bytes", len(encoded))
	}
	if !isVersionListInvoke(harness.InvokeRequest{Tool: "version.list"}) {
		t.Fatal("version.list request was not recognized")
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

func retiredInteractionRespondMixTreatmentPluginPreparationReturnsNewConfirmation(t *testing.T) {
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
				NeedsResolution:           []string{"plugin_instance", "live_parameter_surface", "exact_control"},
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

func TestPlainApprovalGuardLetsActiveFreeStateContinuationReachAgentLoop(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	now := time.Now().UTC()
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "chat_free_state_continue",
		Status: "re_evaluating", DecisionPhase: freeStatePhasePostActionEvaluation,
		OriginalIntent: "make the vocal steadier", ActiveIntent: "make the vocal steadier",
		MaxCycles: 6, CreatedAt: now, UpdatedAt: now,
	})
	body, _ := json.Marshal(ChatRequest{
		ConversationID: "chat_free_state_continue", Message: "continue",
		Context: map[string]any{"agent_mode": agentModeDefault},
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
	if resp.StopReason == "plain_approval_without_pending_confirmation" || resp.StopReason == "no_pending_mix_tick_candidate" {
		t.Fatalf("free-state continuation was intercepted before agent loop: %+v", resp)
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

func TestRecoverPendingMixTickInteractionFromDurablePayload(t *testing.T) {
	interaction, ok := recoverPendingMixTickInteractionFromPayload("interaction-mix", map[string]any{
		"workflow": "mix_tick", "operation": "track_gain_adjust", "track_id": "1007",
		"observation_id": "obs-1", "conversation_id": "conversation-mix", "goal_id": "goal-mix", "run_id": "run-mix",
		"delta_db": -0.5, "request_context": map[string]any{"conversation_id": "conversation-mix"},
	})
	if !ok || interaction.ID != "interaction-mix" || interaction.ConversationID != "conversation-mix" || interaction.Kind != "mix_tick_confirmation" {
		t.Fatalf("recovered interaction = %+v, ok=%v", interaction, ok)
	}
}

func TestRecoverPendingMixTickInteractionFromKindOnlyPayload(t *testing.T) {
	interaction, ok := recoverPendingMixTickInteractionFromPayload("interaction-kind", map[string]any{
		"kind": "mix_tick_confirmation", "operation": "track_gain_adjust", "track_id": "1007",
		"observation_id": "obs-kind", "conversation_id": "conversation-kind", "goal_id": "goal-kind", "run_id": "run-kind",
		"delta_db": -0.5,
	})
	if !ok || interaction.Workflow != "mix_tick" || interaction.ConversationID != "conversation-kind" {
		t.Fatalf("kind-only recovery = %+v, ok=%v", interaction, ok)
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

func executorResultForMixTickReport(tool string, result map[string]any) executor.Result {
	return executor.Result{Status: "ok", Tool: tool, Result: result}
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

func TestPluginGrabberContextPackKeepsFullParameterCount(t *testing.T) {
	digest := buildPluginParameterDigest(map[string]any{
		"track_id":  "1007",
		"plugin_id": "plugin_a",
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
		NeedsResolution:  []string{"request_more_observation", "live_parameter_surface"},
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
	if len(needs) != 2 || needs[1] != "live parameter surface" {
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
			Type:           "parameter_observation_form",
			Source:         "plugin_grabber",
			Workflow:       "plugin_grabber.explain_controls",
			Stage:          "parameter_observation",
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
		case "set_plugin_param", "plugin_set_parameter":
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
