package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/llm"
)

func TestMessageLoopStripsChineseExecutionQuestion(t *testing.T) {
	reply := "L3 深度包还在 building。\n\n如果你认可这个小步建议，需要我继续执行吗？"
	if !messageLoopMixReplyAsksForExecution(reply) {
		t.Fatalf("execution question was not detected")
	}
	stripped := messageLoopStripExecutionQuestion(reply)
	if strings.Contains(stripped, "继续执行") || strings.Contains(stripped, "执行吗") {
		t.Fatalf("execution question was not stripped: %q", stripped)
	}
	if !strings.Contains(stripped, "building") {
		t.Fatalf("observation body was lost: %q", stripped)
	}
}

func TestMessageLoopIncompleteL3ObservationDoesNotCreatePendingFromReply(t *testing.T) {
	state := &runState{input: Input{UserText: "帮我看整体混音"}}
	reply := "整体混音观察结果：L3 深度特征仍是 partial，spectrogram tiles 还在 building。\n\n如果你认可这个小步建议，需要我继续执行吗？"

	got := messageLoopMixObservationFinalReply(state, reply)
	if strings.Contains(got, "继续执行") || strings.Contains(got, "执行吗") {
		t.Fatalf("execution question was retained: %q", got)
	}
	if state.executionMemory.PendingMixTickCandidate != nil || state.executionMemory.PendingMixTreatment != nil {
		t.Fatalf("incomplete L3 observe created pending action: %+v %+v", state.executionMemory.PendingMixTickCandidate, state.executionMemory.PendingMixTreatment)
	}
	if !messageLoopReplyMentionsIncompleteDeepPackage(reply) {
		t.Fatalf("reply did not detect incomplete L3 package")
	}
}

func TestMessageLoopIncompleteL3AllowsResolvedVocalActionPending(t *testing.T) {
	state := &runState{input: Input{
		UserText: "Track 1 是主唱",
		Conversation: []llm.Message{
			{Role: "user", Content: "主唱能不能更靠前"},
			{Role: "assistant", Content: "需要先确认哪条是主唱轨。"},
		},
		Context: map[string]any{"conversation_id": "test_vocal"},
	}}
	reply := `可以考虑把 Track 2 小幅降低 -1 dB，让主唱相对更突出。

不过当前 L3 深度分析仍是 partial，spectrogram tiles 仍在 building。

mix_treatment_pending: {"schema_version":"mix_treatment_pending.v0","status":"pending_confirmation","intent":"make vocal forward","target_ref":"track:1010","action_kind":"gain_balance","processor_type":"utility","delta_db":-1,"reasoning_summary":"lower backing track slightly","confidence":"medium","evidence_refs":["obs_test"],"needs_resolution":[],"expires_after_context_change":true}`

	got := messageLoopMixObservationFinalReply(state, reply)
	if state.executionMemory.PendingMixTreatment == nil {
		t.Fatalf("resolved vocal action did not create pending treatment; reply=%q", got)
	}
	if strings.Contains(got, "mix_treatment_pending") {
		t.Fatalf("internal pending marker leaked: %q", got)
	}
}

func TestMessageLoopObservationFollowupConsumesTreatmentMarker(t *testing.T) {
	state := &runState{
		input: Input{
			UserText: "那你打算怎么办？",
			Context:  map[string]any{"conversation_id": "webui_followup"},
		},
		recentObservation: &RecentObservation{
			Tool:   "mix.observe",
			Status: "ok",
			Summary: map[string]any{
				"observation_id": "obs_followup",
				"mix_session_id": "mix_session_followup",
				"target_ref": map[string]any{
					"track_id": "1007",
				},
			},
		},
	}
	reply := `我建议先做一个很小的低频整理：用 EQ 做保守低切/低中频整理，先作为可回退候选。

mix_treatment_pending: {
  "schema_version": "mix_treatment_pending.v0",
  "status": "pending_confirmation",
  "intent": "prepare a conservative low-end EQ treatment",
  "target_ref": "track:1007",
  "action_kind": "plugin_treatment",
  "processor_type": "eq",
  "reasoning_summary": "low-end and low-mid energy are prominent in the latest observation",
  "confidence": "medium",
  "evidence_refs": ["obs_followup"],
  "needs_resolution": ["plugin_instance"],
  "expires_after_context_change": true
}

你确认后我再执行。`

	got := messageLoopMixObservationFinalReply(state, reply)
	treatment := state.executionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("follow-up marker did not create pending treatment; reply=%q", got)
	}
	if treatment.ActionKind != "plugin_treatment" || treatment.TargetRef != "track:1007" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(got, "mix_treatment_pending") || strings.Contains(got, "schema_version") || strings.Contains(got, "action_kind") {
		t.Fatalf("internal pending marker leaked: %q", got)
	}
	for _, want := range []string{"结论：", "依据：", "待确认动作：", "限制：", "低频"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered pending reply missing %q: %q", want, got)
		}
	}
}

func TestMessageLoopStripMultilineTreatmentMarker(t *testing.T) {
	reply := `建议先准备一个候选。
mix_treatment_pending: {
  "schema_version": "mix_treatment_pending.v0",
  "status": "pending_confirmation",
  "target": {"track_id": "1007"}
}
等待确认。`
	got := messageLoopStripMixTreatmentPendingMarkup(reply)
	if strings.Contains(got, "mix_treatment_pending") || strings.Contains(got, "schema_version") {
		t.Fatalf("multiline marker was not stripped: %q", got)
	}
	if !strings.Contains(got, "建议先准备一个候选") || !strings.Contains(got, "等待确认") {
		t.Fatalf("visible reply text was lost: %q", got)
	}
}
