package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/promptruntime"
)

// FULLACCESS-AUTONOMY-1 guardrail/behavior tests.
//
// The three deterministic properties the fix must hold:
//  1. an explicit full project access grant renders the autonomy directive on
//     BOTH prompt paths the runtime actually assembles — the ordinary ReAct
//     prompt and the free-state neutral-family prompt;
//  2. a manual (or ungranted) turn renders neither the directive nor a bounced
//     clarification: the previous prompt wording stays byte-identical;
//  3. the guardrail fires on exactly the direction/selection/dose question class
//     and never on a request-object question, and it is bounded to one bounce.

// fullAccessJourney1R9Question is the verbatim clarification text recorded in
// queue/reports/2026-09-13-JOURNEY-1.md §4.4 (run 20260913_190906) — the RED
// sample this card repairs. The same shape is on the card from the user's own
// hand test (912.vit, 2026-09-13 12:26:53).
const fullAccessJourney1R9Question = "低音轨（bass）目前还没有加载任何 EQ 插件，而且这一轮我拿不到该轨的音频观察证据（没有可用的频段/响度分析结果），所以还不能直接落一组可执行的静态 EQ 参数。请确认两件事：1）希望我用哪个已安装的 EQ 插件来做这个实验（我可以从插件库里挑一个通用 EQ）；2）这个实验想解决什么听感，比如低频太浑想更紧、想更亮更有轮廓，还是纯做前后对比试听？"

// fullAccessDirectiveMarkers are the durable sentences the guardrail and the
// prompt rely on. They are matched individually rather than as the whole
// constant because promptruntime.Build trims section-boundary whitespace.
var fullAccessDirectiveMarkers = []string{
	"Full project access is granted for this turn",
	"You execute the entire chain yourself",
	"Do NOT ask the user to choose a listening direction",
	"request-object ambiguity",
	"safety boundary",
	"Never ask the user to confirm a step this mode already authorizes",
}

func fullAccessGrantedContext(extra map[string]any) map[string]any {
	ctx := map[string]any{
		"authority_mode":          "full_project_access",
		"authority_mode_explicit": true,
	}
	for key, value := range extra {
		ctx[key] = value
	}
	return ctx
}

func fullAccessFreeStateContext(authority map[string]any) map[string]any {
	ctx := map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
		},
	}
	for key, value := range authority {
		ctx[key] = value
	}
	return ctx
}

// fullAccessAssemblySystemText returns the system text of an assembled prompt —
// the text the model actually receives.
func fullAccessAssemblySystemText(assembly promptruntime.Assembly) string {
	parts := make([]string, 0, len(assembly.Messages))
	for _, message := range assembly.Messages {
		if strings.EqualFold(strings.TrimSpace(message.Role), "system") {
			parts = append(parts, message.Content)
		}
	}
	return strings.Join(parts, "\n---\n")
}

func assertFullAccessDirective(t *testing.T, label, prompt string, want bool) {
	t.Helper()
	for _, marker := range fullAccessDirectiveMarkers {
		if got := strings.Contains(prompt, marker); got != want {
			t.Fatalf("%s: marker %q present=%t want=%t", label, marker, got, want)
		}
	}
}

// TestFullAccessAutonomyDirectiveRendersOnBothPromptPaths pins property 1 on the
// real assembly paths. The free-state half matters most: an active free-state
// turn never reaches messageLoopSystemPrompt, because assembly() short-circuits
// into assemblyNeutralFamilySelection (messageLoopNeedsNeutralFamilyProjection
// mirrors messageLoopFreeStateActive). A hook that only covered the
// messageLoopSystemPrompt branch would leave the live prompt unchanged.
func TestFullAccessAutonomyDirectiveRendersOnBothPromptPaths(t *testing.T) {
	ordinaryState := &runState{input: Input{Context: fullAccessGrantedContext(nil)}}
	assertFullAccessDirective(t, "ordinary prompt", messageLoopSystemPrompt(ordinaryState), true)
	assertFullAccessDirective(t, "ordinary assembly", fullAccessAssemblySystemText((&MessageLoop{}).assembly(ordinaryState, "{}")), true)

	freeState := &runState{input: Input{Context: fullAccessFreeStateContext(fullAccessGrantedContext(nil))}}
	neutral := messageLoopSystemPrompt(freeState)
	if !strings.Contains(neutral, "neutral observation-and-family decision phase") {
		t.Fatalf("free-state prompt path was not taken by the test setup")
	}
	assertFullAccessDirective(t, "free-state prompt", neutral, true)
	assertFullAccessDirective(t, "free-state assembly", fullAccessAssemblySystemText((&MessageLoop{}).assembly(freeState, "{}")), true)
}

// TestManualAuthorityPromptStaysByteIdentical pins property 2 and the card's
// "manual 模式行为不变" clause: the manual / ungranted prompts must be
// byte-identical to the no-authority baseline on both assembly paths.
func TestManualAuthorityPromptStaysByteIdentical(t *testing.T) {
	manualAuthority := map[string]any{"authority_mode": "manual_confirmation", "authority_mode_explicit": true}
	ungranted := map[string]any{"authority_mode": "full_project_access"}
	cases := []struct {
		name      string
		authority map[string]any
	}{
		{name: "manual_confirmation", authority: manualAuthority},
		{name: "ungranted_full_access", authority: ungranted},
	}
	for _, tc := range cases {
		baseline := fullAccessAssemblySystemText((&MessageLoop{}).assembly(&runState{input: Input{Context: map[string]any{}}}, "{}"))
		got := fullAccessAssemblySystemText((&MessageLoop{}).assembly(&runState{input: Input{Context: tc.authority}}, "{}"))
		if got != baseline {
			t.Fatalf("%s ordinary assembly differs from the no-authority baseline", tc.name)
		}
		assertFullAccessDirective(t, tc.name+" ordinary assembly", got, false)

		baselineFree := fullAccessAssemblySystemText((&MessageLoop{}).assembly(&runState{input: Input{Context: fullAccessFreeStateContext(nil)}}, "{}"))
		gotFree := fullAccessAssemblySystemText((&MessageLoop{}).assembly(&runState{input: Input{Context: fullAccessFreeStateContext(tc.authority)}}, "{}"))
		if gotFree != baselineFree {
			t.Fatalf("%s free-state assembly differs from the no-authority baseline", tc.name)
		}
		assertFullAccessDirective(t, tc.name+" free-state assembly", gotFree, false)
	}
}

// TestFullAccessDirectionClarificationIssue pins property 3a: the guardrail fires
// on the observed defect class and stands down everywhere the mode is entitled
// to ask, or must stay silent.
func TestFullAccessDirectionClarificationIssue(t *testing.T) {
	granted := &runState{input: Input{Context: fullAccessGrantedContext(nil)}}
	manual := &runState{input: Input{Context: map[string]any{"authority_mode": "manual_confirmation", "authority_mode_explicit": true}}}
	ungranted := &runState{input: Input{Context: map[string]any{"authority_mode": "full_project_access"}}}

	cases := []struct {
		name  string
		state *runState
		out   messageLoopOutput
		want  bool
	}{
		{
			name:  "journey1_r9_verbatim_direction_and_selection_question",
			state: granted,
			out:   messageLoopOutput{NeedsClarification: true, Reply: fullAccessJourney1R9Question},
			want:  true,
		},
		{
			name:  "user_hand_test_option_menu",
			state: granted,
			out:   messageLoopOutput{NeedsClarification: true, ClarificationQuestion: "请选择这次实验的方向：A. 加厚低频（80Hz 附近抬起）；B. 清理浑浊（200–300Hz 附近收敛）；C. 提亮指法与质感（800Hz–1.5kHz 附近轻抬）"},
			want:  true,
		},
		{
			name:  "selection_only_question",
			state: granted,
			out:   messageLoopOutput{NeedsClarification: true, Reply: "低音轨还没有 EQ 插件，请告诉我用哪个已安装的 EQ 插件来做这个实验。"},
			want:  true,
		},
		{
			name:  "dose_only_question",
			state: granted,
			out:   messageLoopOutput{NeedsClarification: true, Reply: "这条轨的增益你想调多少 dB？"},
			want:  true,
		},
		{
			name:  "focus_track_question_is_request_object_ambiguity",
			state: granted,
			out:   messageLoopOutput{NeedsClarification: true, ClarificationQuestion: messageLoopFocusTrackClarificationQuestion()},
			want:  false,
		},
		{
			name:  "object_question_with_generic_choice_word",
			state: granted,
			out:   messageLoopOutput{NeedsClarification: true, Reply: "你要处理的是哪个轨道？"},
			want:  false,
		},
		{
			name:  "safety_boundary_question",
			state: granted,
			out:   messageLoopOutput{NeedsClarification: true, Reply: "这一步会删除工程里的原始音频且无法撤销，确认继续吗？"},
			want:  false,
		},
		{
			name:  "manual_mode_keeps_the_same_question",
			state: manual,
			out:   messageLoopOutput{NeedsClarification: true, Reply: fullAccessJourney1R9Question},
			want:  false,
		},
		{
			name:  "ungranted_full_access_is_not_a_grant",
			state: ungranted,
			out:   messageLoopOutput{NeedsClarification: true, Reply: fullAccessJourney1R9Question},
			want:  false,
		},
		{
			name:  "non_clarification_output_is_untouched",
			state: granted,
			out:   messageLoopOutput{Final: true, Reply: "已按 200–300Hz 收敛 1.5dB，A/B 卡已挂出。"},
			want:  false,
		},
	}
	for _, tc := range cases {
		issue := messageLoopFullAccessDirectionClarificationIssue(tc.state, tc.out)
		if got := issue != ""; got != tc.want {
			t.Fatalf("%s: guardrail fired=%t want=%t (issue=%q)", tc.name, got, tc.want, issue)
		}
		if tc.want && issue != fullAccessDirectionClarificationBounceText {
			t.Fatalf("%s: bounce text = %q", tc.name, issue)
		}
	}
}

// TestFullAccessDirectionClarificationBounceIsBounded pins property 3b: one
// bounce per turn and the <final_gate> feedback goes back into the conversation.
func TestFullAccessDirectionClarificationBounceIsBounded(t *testing.T) {
	t.Setenv("VIT_AGENT_MESSAGE_LOOP_DEBUG_PATH", "off")
	state := &runState{input: Input{Context: fullAccessGrantedContext(nil), Conversation: nil}}
	if !messageLoopFullAccessDirectionClarificationBounce(state, fullAccessDirectionClarificationBounceText, "{}") {
		t.Fatalf("first bounce must be admitted")
	}
	if got := messageLoopFullAccessBounceCount(state); got != 1 {
		t.Fatalf("bounce count = %d, want 1", got)
	}
	if len(state.input.Conversation) != 1 || !strings.Contains(state.input.Conversation[0].Content, fullAccessDirectionClarificationBounceText) {
		t.Fatalf("bounce did not feed the <final_gate> correction back: %+v", state.input.Conversation)
	}
	if messageLoopFullAccessDirectionClarificationBounce(state, fullAccessDirectionClarificationBounceText, "{}") {
		t.Fatalf("second bounce must be refused so the turn can never be trapped")
	}
	if got := messageLoopFullAccessBounceCount(state); got != fullAccessAutonomyMaxBounces {
		t.Fatalf("bounce count after the refused bounce = %d, want %d", got, fullAccessAutonomyMaxBounces)
	}
}
