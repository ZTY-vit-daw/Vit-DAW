package planner

import (
	"strings"
	"testing"
)

func TestMessagesPreferContextSnapshotWhenPresent(t *testing.T) {
	p := LLMPlanner{}
	messages := p.messages(Input{
		UserText: "inspect the project",
		Context:  map[string]any{"selected_track_id": "raw_track"},
		State:    map[string]any{"project_path": "raw_project"},
		ContextSnapshot: map[string]any{
			"schema_version":    "vit_context_snapshot.v1",
			"current_selection": map[string]any{"selected_track_id": "track_7"},
		},
	})
	if len(messages) != 2 || !strings.Contains(messages[1].Content, "Context snapshot JSON") {
		t.Fatalf("messages = %+v", messages)
	}
	if strings.Contains(messages[1].Content, "Selected/context JSON") || strings.Contains(messages[1].Content, "raw_project") {
		t.Fatalf("planner should not use raw context/state when snapshot is present:\n%s", messages[1].Content)
	}
}

func TestMessagesTellPlannerToUseRackForPluginLoads(t *testing.T) {
	p := LLMPlanner{}
	messages := p.messages(Input{UserText: "load eq and reverb"})
	if len(messages) != 2 {
		t.Fatalf("messages = %+v", messages)
	}
	system := messages[0].Content
	for _, want := range []string{"plugin.load_to_rack", "rack.add_node", "Do not use plugin.instantiate", "multiple plugin types"} {
		if !strings.Contains(system, want) {
			t.Fatalf("planner system prompt missing %q:\n%s", want, system)
		}
	}
}

func TestMessagesIncludeCurrentPlanItems(t *testing.T) {
	p := LLMPlanner{}
	messages := p.messages(Input{
		UserText:  "continue",
		PlanItems: []PlanItem{{ID: "load_eq", Description: "Load an EQ", Status: "running"}},
	})
	if len(messages) != 2 || !strings.Contains(messages[1].Content, "Current plan items JSON") || !strings.Contains(messages[1].Content, "load_eq") {
		t.Fatalf("planner message missing plan items:\n%+v", messages)
	}
}

func TestParseOutputNormalizesPlanItemsCompletionAndPlanItemToolCalls(t *testing.T) {
	out, err := ParseOutput(`{
		"done": false,
		"reply": " working ",
		"failure_reason": " ",
		"plan_items": [{"id":" load_eq ","description":" Load EQ ","status":""}],
		"tool_calls": [{"tool":" plugin.load_to_rack ","args":{"track_id":"1"},"plan_item_id":" load_eq "}],
		"completion": {"satisfied_plan_ids":[" load_eq ",""],"evidence":[" observed ",""],"uncertain": false},
		"verification": {"tool_call_id":" call_1 ","plan_item_id":" load_eq ","tool":" plugin.load_to_rack ","status":" verified ","message":" observed "}
	}`)
	if err != nil {
		t.Fatalf("ParseOutput error: %v", err)
	}
	if out.Reply != "working" || len(out.PlanItems) != 1 || out.PlanItems[0].ID != "load_eq" || out.PlanItems[0].Status != "pending" {
		t.Fatalf("plan items not normalized: %+v", out)
	}
	if len(out.ToolCalls) != 1 || out.ToolCalls[0].Tool != "plugin.load_to_rack" || out.ToolCalls[0].PlanItemID != "load_eq" {
		t.Fatalf("tool call not normalized: %+v", out.ToolCalls)
	}
	if out.Completion == nil || len(out.Completion.SatisfiedPlanIDs) != 1 || out.Completion.SatisfiedPlanIDs[0] != "load_eq" || len(out.Completion.Evidence) != 1 {
		t.Fatalf("completion not normalized: %+v", out.Completion)
	}
	if out.Verification == nil || out.Verification.ToolCallID != "call_1" || out.Verification.Status != "verified" {
		t.Fatalf("verification not normalized: %+v", out.Verification)
	}
}

func TestParseOutputNeedsClarificationDropsToolCalls(t *testing.T) {
	out, err := ParseOutput(`{
		"done": false,
		"needs_clarification": true,
		"clarification_question": " Which clip? ",
		"reply": " Which clip? ",
		"tool_calls": [{"tool":"midi.read_clip_notes","args":{"clip_id":"guessed"}}]
	}`)
	if err != nil {
		t.Fatalf("ParseOutput error: %v", err)
	}
	if !out.NeedsClarification || out.ClarificationQuestion != "Which clip?" {
		t.Fatalf("clarification fields not normalized: %+v", out)
	}
	if len(out.ToolCalls) != 0 {
		t.Fatalf("clarification output must not keep tool calls: %+v", out.ToolCalls)
	}
}
