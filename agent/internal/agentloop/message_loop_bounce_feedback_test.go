package agentloop

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	agentruntime "vit-daw-agent/internal/runtime"
)

// MSGLOOP-FEEDBACK-LOSS-1 三步断言：请求 → 弹回 → 重建请求含反馈。
//
// 构造与 262 取证 run（coord/runs/AF-FAMILY-HEALTH-1/mix_tick_20261105_224459）
// 同族的失败形态：free-state loop active、project 级 view set 已在 observation
// ledger 的 available_views 里返回过 usable 证据，模型仍以 needs_observation
// 重请求同一 view set —— view-set 去重门（messageLoopFreeStateOutputIssue →
// messageLoopFreeStateAlreadyObservedIssue）弹回。
//
// 本测试把「弹回反馈必须进入下一轮重建请求」锁为回归契约：第 10947 行的
// <final_gate> append 由中性族装配面（messageLoopNeutralFamilyFeedbackHistory）
// 消费进 History 消息。若未来任何人改动 append 点或 History 裁剪面导致反馈
// 再次丢失，此测试失败。
func TestMessageLoopBounceFeedbackEntersRebuiltRequest(t *testing.T) {
	t.Setenv("VIT_AGENT_MESSAGE_LOOP_DEBUG_PATH", filepath.Join(t.TempDir(), "message_loop_debug.jsonl"))

	const bounceIssue = "the requested project-level CCB view set already returned usable evidence; choose a different cataloged view or target instead of repeating it"
	state := &runState{
		input: Input{
			UserText: "improve the mix with one bounded adjustment",
			Context: map[string]any{
				"conversation_id": "conv-msgloop-feedback-267",
				"free_state_reasoning_loop": map[string]any{
					"schema_version":  "free_state_reasoning_loop.v1",
					"status":          "active",
					"original_intent": "improve the mix with one bounded adjustment",
					// The project-level view set already returned usable
					// evidence: this is the dedup-gate premise of the bounce.
					"observation_ledger": map[string]any{
						"available_views": map[string]any{
							"project.structure":            map[string]any{"status": "usable"},
							"mix.multitrack_relationship":  map[string]any{"status": "usable"},
						},
					},
				},
			},
		},
		goal:   agentruntime.Goal{GoalID: "goal-msgloop-feedback-267", RunID: "run-msgloop-feedback-267"},
		budget: Budget{MaxToolCalls: 8},
	}
	loop := &MessageLoop{carrierWorkspaceDir: t.TempDir()}
	snapshotJSON := `{"free_state_reasoning_loop":{"status":"active"}}`

	// Step 1（请求）：弹回发生前，重建请求不含任何 final_gate 反馈。
	before := loop.assembly(state, snapshotJSON)
	for _, message := range before.Messages {
		if strings.Contains(message.Content, bounceIssue) || strings.Contains(message.Content, "<final_gate>") {
			t.Fatalf("pre-bounce request already carries gate feedback: %#v", before.Messages)
		}
	}

	// Step 2（弹回）：模型以 needs_observation 重请求同一 project 级 view set，
	// 走真实弹回路径（messageLoopFreeStateOutputIssue → 10947 append）。
	raw := `{"final":false,"reply":"requesting the project view set again for relative levels","tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["project.structure","mix.multitrack_relationship"]},"reason":"need the relative level disclosure between the two tracks"}],"free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"re-request the project multitrack view set","requested_view_ids":["project.structure","mix.multitrack_relationship"]}}`
	flow, _, _ := loop.interpretModelReply(context.Background(), &Runner{}, state, raw, before.Fingerprint)
	if flow != modelFlowRetry {
		t.Fatalf("dedup-gate bounce flow = %v, want modelFlowRetry", flow)
	}
	appended := false
	for _, message := range state.input.Conversation {
		if message.Role == "user" && message.Content == "<final_gate>"+bounceIssue+"</final_gate>" {
			appended = true
		}
	}
	if !appended {
		t.Fatalf("bounce issue was not appended to the conversation: %#v", state.input.Conversation)
	}

	// Step 3（重建请求）：下一轮装配必须把弹回反馈带进 History 面。
	after := loop.assembly(state, snapshotJSON)
	visible := false
	for _, message := range after.Messages {
		if strings.Contains(message.Content, "<final_gate>"+bounceIssue+"</final_gate>") {
			visible = true
		}
	}
	if !visible {
		t.Fatalf("rebuilt request lost the bounce feedback (History face): %#v", after.Messages)
	}
}
