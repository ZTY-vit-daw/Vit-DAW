package chat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/shadow"
)

func boxedSplitRangeContext() map[string]any {
	return map[string]any{
		"selected_clip_id":       "smoke_clip_1",
		"selected_clip_track_id": "smoke_track_1",
		"selected_clip_ranges": []any{
			map[string]any{
				"range_id":                 "smoke_range_1",
				"clip_id":                  "smoke_clip_1",
				"track_id":                 "smoke_track_1",
				"start_seconds":            2.0,
				"end_seconds":              3.5,
				"duration_seconds":         1.5,
				"clip_start_seconds":       0.0,
				"clip_end_seconds":         6.0,
				"clip_local_start_seconds": 2.0,
				"clip_local_end_seconds":   3.5,
			},
		},
	}
}

func TestSynthesizeClipRangeSplitCommandsPreModelMatchesFallback(t *testing.T) {
	cmds := synthesizeClipRangeSplitCommandsPreModel("把这段拆出来", boxedSplitRangeContext())
	if len(cmds) != 2 {
		t.Fatalf("expected the deterministic two-cut plan, got %+v", cmds)
	}
	wantTimes := []float64{3.5, 2.0}
	for i, cmd := range cmds {
		if cmd["tool"] != "clip.split" {
			t.Fatalf("command %d tool = %v", i, cmd["tool"])
		}
		args := cmd["args"].(map[string]any)
		if args["split_time"] != wantTimes[i] {
			t.Fatalf("command %d split_time = %v, want %v (end cut first)", i, args["split_time"], wantTimes[i])
		}
	}
	fallback := synthesizeLocalDAWCommands("把这段拆出来", boxedSplitRangeContext())
	if len(fallback) != 2 {
		t.Fatalf("fallback dispatch must agree, got %+v", fallback)
	}
	for i := range cmds {
		if cmds[i]["tool"] != fallback[i]["tool"] {
			t.Fatalf("command %d tool mismatch: %v vs %v", i, cmds[i]["tool"], fallback[i]["tool"])
		}
	}
	if got := synthesizeClipRangeSplitCommandsPreModel("把这段拆出来", map[string]any{"selected_clip_id": "c1"}); len(got) != 0 {
		t.Fatalf("no ranges must stay nil, got %+v", got)
	}
	if got := synthesizeClipRangeSplitCommandsPreModel("在2.5秒切开这段", boxedSplitRangeContext()); len(got) != 0 {
		t.Fatalf("spoken seconds must stay nil, got %+v", got)
	}
}

// TestHandleChatBoxedRangeSplitTakesOverBeforeLLM pins the leg-1 fix end to
// end: the LLM base URL is a dead port, so any turn that reaches the model
// call fails with an Ask-Vit error. The boxed-range split turn must instead
// return the confirmation card with the two kernel-valid cuts (end 3.5 before
// start 2.0) deterministically.
func TestHandleChatBoxedRangeSplitTakesOverBeforeLLM(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_BASE_URL", "http://127.0.0.1:9/v1")
	t.Setenv("VIT_AGENT_LLM_API_KEY", "test-key")
	t.Setenv("VIT_AGENT_LLM_MODEL", "test-model")

	shadowProject := shadow.New(nil)
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(&recordingChatKernel{replyByCommand: map[string][]map[string]any{}}, shadowProject, nil)

	requestContext := boxedSplitRangeContext()
	requestContext["agent_mode"] = agentModeDefault
	requestContext["disable_agent_loop"] = true
	body, _ := json.Marshal(ChatRequest{
		ConversationID: "chat_range_split_premodel",
		Message:        "把这段拆出来",
		Context:        requestContext,
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
	if strings.Contains(resp.Reply, "Ask Vit AI call failed") || resp.Error != "" {
		t.Fatalf("turn must not reach the LLM (dead base URL would fail): reply=%q error=%q", resp.Reply, resp.Error)
	}
	if !resp.NeedsConfirmation {
		t.Fatalf("expected the confirmation card, got %+v", resp)
	}
	if len(resp.Commands) != 2 {
		t.Fatalf("expected two split decisions, got %+v", resp.Commands)
	}
	wantTimes := []float64{3.5, 2.0}
	for i, decision := range resp.Commands {
		command := decision.Command
		if command["tool"] != "clip.split" {
			t.Fatalf("decision %d command = %+v", i, command)
		}
		args := command["args"].(map[string]any)
		if args["split_time"] != wantTimes[i] || args["clip_id"] != "smoke_clip_1" || args["track_id"] != "smoke_track_1" {
			t.Fatalf("decision %d args = %+v (want end cut 3.5 then start cut 2.0)", i, args)
		}
	}
}

// TestHandleChatBoxedRangeSplitWithoutRangesKeepsModelTurn pins the zero-change
// regression surface: without selected_clip_ranges the same phrase must NOT be
// taken over pre-model, so the turn still goes to the (here dead) LLM.
func TestHandleChatBoxedRangeSplitWithoutRangesKeepsModelTurn(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_BASE_URL", "http://127.0.0.1:9/v1")
	t.Setenv("VIT_AGENT_LLM_API_KEY", "test-key")
	t.Setenv("VIT_AGENT_LLM_MODEL", "test-model")

	shadowProject := shadow.New(nil)
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(&recordingChatKernel{replyByCommand: map[string][]map[string]any{}}, shadowProject, nil)

	body, _ := json.Marshal(ChatRequest{
		ConversationID: "chat_range_split_no_ranges",
		Message:        "把这段拆出来",
		Context: map[string]any{
			"agent_mode":         agentModeDefault,
			"disable_agent_loop": true,
			"selected_clip_id":   "smoke_clip_1",
		},
	})
	rec := httptest.NewRecorder()
	server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))

	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Reply, "Ask Vit AI call failed") {
		t.Fatalf("without ranges the turn must keep the existing model path, got reply=%q error=%q commands=%+v", resp.Reply, resp.Error, resp.Commands)
	}
	if resp.NeedsConfirmation || len(resp.Commands) > 0 {
		t.Fatalf("no split proposals may surface without ranges: %+v", resp)
	}
}
