package agentloop

import (
	"strings"

	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
)

// FULLACCESS-AUTONOMY-1: full project access is a product-level grant of
// autonomous execution, not a confirmation shortcut.
//
// The user's ruling (2026-09-13, verbatim): 完全访问权限应该是它能够自主进行所有执行，
// 我只负责AB试听。 Under manual confirmation the Agent legitimately asks before a
// bounded move; under full project access the whole chain — observe with its own
// observation calls, choose the direction and the dose inside the admitted domain
// table, execute, explain the choice, hand the result to the user's A/B audition —
// is the Agent's own work. Asking the user to pick a direction, a processor
// family, a plug-in instance, or a dose is therefore a behavioral defect in this
// mode, not a courtesy.
//
// Two surfaces implement that contract, and they are deliberately paired:
//
//   - the prompt surface (fullAccessAutonomyDirective, injected into both the
//     ordinary ReAct prompt and the free-state neutral-family prompt) states who
//     owns the choice, so the model can act correctly the first time;
//   - the guardrail surface (messageLoopFullAccessDirectionClarificationIssue +
//     messageLoopFullAccessDirectionClarificationBounce) is the deterministic
//     backstop for when it does not, and it is bounded to one bounce per turn so
//     a genuine boundary can never be trapped.
//
// The directive decides only WHO chooses. It never lowers a runtime boundary:
// capability, evidence freshness, the admission gates, the PCA load gate, and
// safety refusals keep their exact wording. The one clarification class that
// survives is request-object ambiguity — the user's wording does not say which
// object the request is about and no disclosed project fact identifies it (the
// AGENT-F3 / focus-track contract: messageLoopFocusTrackClarificationQuestion)
// — plus a safety boundary outside the bounded reversible envelope.

const (
	// fullAccessAutonomyBounceKey counts the deterministic bounces this turn
	// already spent. It lives on the turn context rather than on runState so the
	// guardrail needs no new state field and stays observable in the context
	// snapshot.
	fullAccessAutonomyBounceKey = "full_access_autonomy_bounces"

	// fullAccessAutonomyMaxBounces bounds the guardrail at one bounce per turn.
	// A model that repeats the same direction question after being told that this
	// mode already authorizes the choice is kept rather than looped: the second
	// question is delivered as-is, so the mechanism can never trap a turn that
	// genuinely needs the user.
	fullAccessAutonomyMaxBounces = 1

	// fullAccessAutonomyDiagnosticStage is the artifact-log stage for the bounce,
	// alongside the existing "free_state_final_gate" / "parse_failed" stages.
	fullAccessAutonomyDiagnosticStage = "full_access_autonomy_gate"

	// authorityModeFullProjectAccess is the context value the chat server binds
	// (turn_control.go: bindChatAuthorityMode) and the harness load gate already
	// reads (harness.go: explicitFullProjectAccess). Full access requires the
	// explicit flag as well: a bare mode string is never treated as a grant.
	authorityModeFullProjectAccess = "full_project_access"
)

// fullAccessAutonomyDirective is the prompt-surface half of the contract. It is
// injected only when the user's own authority control granted full project
// access for this turn, so the manual/ordinary prompt stays byte-identical.
const fullAccessAutonomyDirective = "Full project access is granted for this turn (selected by the user's own authority control):" + `
- You execute the entire chain yourself: observe with your own observation tool calls, choose the treatment direction and the dose inside the admitted domain table, execute it, then report the chosen direction, the chosen dose, and why you chose them, and hand the result to the user's A/B listening. In this mode the user's only duty is the A/B audition.
- Do NOT ask the user to choose a listening direction, a treatment goal, a processor family, a plug-in instance, an observation view, a target that the disclosed project facts already resolve, or a parameter/dose. Those choices are yours. When evidence is missing, obtain it with your own observation calls instead of asking for it.
- A clarification request is legal in this mode only for (a) request-object ambiguity: the user's wording does not say which object (track or clip) the request is about and no disclosed project fact identifies it, or (b) a safety boundary: the requested operation falls outside the bounded, reversible experiment envelope and cannot be undone.
- Never ask the user to confirm a step this mode already authorizes. Under full project access the runtime applies the admitted bounded experiment without a per-step confirmation card, so "waiting for the user's confirmation" is not a legal outcome of this turn.
- This directive decides only who chooses. It never lowers a runtime boundary: capability, evidence-freshness, admission-gate, PCA, and safety refusals keep their normal wording and are still returned as boundaries, not as questions.
- Do not announce the authority mode as a question or offer the user a menu of directions. State the chosen direction and dose with the reason, in the user's language.

`

// fullAccessDirectionClarificationBounceText is the guardrail message fed back
// through the existing <final_gate> channel. It names the legal clarification
// classes explicitly so the model can either comply or disclose a real boundary
// instead of silently re-asking.
const fullAccessDirectionClarificationBounceText = "full project access is granted for this turn: you own the choice and you execute the chain yourself. Do not ask the user to choose a direction, a listening goal, a processor family, a plug-in instance, an observation view, a target the disclosed project facts already resolve, or a parameter/dose. Obtain missing evidence with your own observation tool calls, then execute and report the chosen direction and dose with the reason, and hand the result to the user's A/B audition. A clarification is legal in this mode only for request-object ambiguity (the wording does not identify which track or clip the request is about and no disclosed project fact resolves it) or for a safety boundary outside the bounded reversible envelope. Return the final answer, the tool calls, or that concrete boundary now."

// The guardrail's discrimination is deliberately one-sided: under full project
// access the ONLY legal clarification is a request-object question, so the
// object check runs first and stands the guardrail down unconditionally.

// fullAccessDirectionClarificationObjectNouns are the object nouns a generic
// choice word must sit next to for the question to count as a request-object
// question ("哪个轨道", "which clip").
var fullAccessDirectionClarificationObjectNouns = []string{
	"轨", "軌", "track", "clip", "片段", "对象", "對象", "工程", "文件", "总线", "總線", "bus",
}

// fullAccessDirectionClarificationTargetPhrases are the explicit request-object
// markers. The focus-track question
// (messageLoopFocusTrackClarificationQuestion: 需要先确认哪条是主唱轨…主唱是 Track 几)
// matches here and therefore keeps its existing behavior unchanged.
var fullAccessDirectionClarificationTargetPhrases = []string{
	"哪条", "哪一條", "哪一条", "哪轨", "哪軌", "哪一轨", "哪一軌",
	"哪条轨道", "哪條軌道", "哪个轨道", "哪個軌道", "哪一支轨",
	"which of", "which track", "what track", "which clip", "what clip", "track 几", "track几",
}

// fullAccessDirectionClarificationChoiceWords are the generic choice words:
// "which/what kind" asked without an object noun beside it is a direction,
// family, plug-in, or dose choice, which this mode forbids.
var fullAccessDirectionClarificationChoiceWords = []string{
	"哪个", "哪個", "哪一个", "哪一個", "哪种", "哪種", "哪類", "哪类", "哪款",
	"which", "what kind", "which one",
}

// fullAccessDirectionClarificationDirectionTokens are the treatment-direction
// markers: asking the user what the experiment should sound like or which way to
// move. The vocabulary deliberately overlaps the frozen JOURNEY-1 A1 征询词表
// (哪个/方向/选择/请问/你希望/倾向/还是) so the guardrail fires on exactly the
// class the acceptance assertion calls red.
var fullAccessDirectionClarificationDirectionTokens = []string{
	"想解决什么", "想解決什麼", "什么听感", "什麼聽感", "什么效果", "什麼效果",
	"什么方向", "什麼方向", "往哪个方向", "往哪個方向", "想要什么", "想要什麼",
	"希望达到", "希望達到", "目标是什么", "目標是什麼", "想让它", "想讓它",
	"希望它听起来", "希望它聽起來", "想更", "还是纯做", "還是純做", "想怎么调", "想怎麼調",
	"方向", "选择", "選擇", "请问", "請問", "你希望", "倾向", "傾向", "还是", "還是",
	"which direction", "what direction", "what effect", "what goal", "what should it sound",
	"toward what", "what do you want", "which listening",
}

// fullAccessDirectionClarificationSelectionTokens are the selection markers the
// user's ruling forbids under full project access: asking which processor
// family, plug-in instance, or observation view to use.
var fullAccessDirectionClarificationSelectionTokens = []string{
	"哪个插件", "哪個插件", "哪一个插件", "哪一個插件", "哪个 eq", "哪個 eq", "哪个效果器", "哪個效果器",
	"哪个处理器", "哪個處理器", "哪个家族", "哪個家族", "哪个方案", "哪個方案",
	"用哪个", "用哪個", "选哪个", "選哪個", "选一个", "選一個", "挑一个", "挑一個",
	"推荐一个", "推薦一個", "帮我选", "幫我選", "插件", "效果器", "处理器", "處理器",
	"which plugin", "which plug-in", "which processor", "which eq", "which family", "which kind",
	"pick one", "choose one", "select one", "recommend a", "plugin", "plug-in",
}

// fullAccessDirectionClarificationDoseTokens are the dose markers: asking the
// user how much. Under full project access the dose is chosen inside the
// admitted domain table, never requested.
var fullAccessDirectionClarificationDoseTokens = []string{
	"多少 db", "多少db", "多少分贝", "多少分貝", "增益多少", "调多少", "調多少", "剂量", "劑量",
	"how many db", "how much gain", "what amount", "how much should",
}

// messageLoopFullProjectAccess reports whether this turn runs under an explicit
// full project access grant. Both the mode value and the explicit flag are
// required, mirroring the harness load gate (harness.go: explicitFullProjectAccess):
// a mode string that no authority control granted is not a grant.
func messageLoopFullProjectAccess(state *runState) bool {
	if state == nil || state.input.Context == nil {
		return false
	}
	if !messageLoopBool(state.input.Context["authority_mode_explicit"]) {
		return false
	}
	return strings.EqualFold(messageLoopText(state.input.Context["authority_mode"]), authorityModeFullProjectAccess)
}

// messageLoopFullAccessAutonomyRules renders the prompt-surface directive, or ""
// when this turn is not an explicit full-access turn. Both the ordinary ReAct
// prompt and the free-state neutral-family prompt call it, so the behavioral
// contract is identical on either path; an empty return keeps the manual and
// ordinary prompts byte-identical to their previous wording.
func messageLoopFullAccessAutonomyRules(state *runState) string {
	if !messageLoopFullProjectAccess(state) {
		return ""
	}
	return fullAccessAutonomyDirective
}

// messageLoopFullAccessDirectionClarificationIssue returns the guardrail message
// when a clarification request under full project access asks the user for a
// direction, a selection, or a dose, and "" in every other case:
//
//   - not a clarification request, or not an explicit full-access turn;
//   - an empty question;
//   - a request-object question (which track/clip) — the one legal class;
//   - a question that matches no choice/direction/selection/dose marker, which
//     leaves genuine boundaries (capability, evidence, safety) untouched.
func messageLoopFullAccessDirectionClarificationIssue(state *runState, out messageLoopOutput) string {
	if state == nil || !out.NeedsClarification || !messageLoopFullProjectAccess(state) {
		return ""
	}
	question := strings.ToLower(strings.TrimSpace(firstNonEmpty(out.ClarificationQuestion, out.Reply)))
	if question == "" {
		return ""
	}
	if messageLoopFullAccessRequestObjectQuestion(question) {
		return ""
	}
	if !messageLoopTextHasAny(question, fullAccessDirectionClarificationChoiceWords...) &&
		!messageLoopTextHasAny(question, fullAccessDirectionClarificationSelectionTokens...) &&
		!messageLoopTextHasAny(question, fullAccessDirectionClarificationDirectionTokens...) &&
		!messageLoopTextHasAny(question, fullAccessDirectionClarificationDoseTokens...) {
		return ""
	}
	return fullAccessDirectionClarificationBounceText
}

// messageLoopFullAccessRequestObjectQuestion reports whether a clarification
// asks which object (track or clip) the request is about: either an explicit
// object phrase ("哪条", "which track") or a generic choice word standing
// directly beside an object noun ("哪个轨道"). This is the clarification class
// full project access keeps.
func messageLoopFullAccessRequestObjectQuestion(question string) bool {
	if messageLoopTextHasAny(question, fullAccessDirectionClarificationTargetPhrases...) {
		return true
	}
	for _, choice := range fullAccessDirectionClarificationChoiceWords {
		for index := strings.Index(question, choice); index >= 0; {
			window := []rune(question[index+len(choice):])
			if len(window) > fullAccessObjectWindowRunes {
				window = window[:fullAccessObjectWindowRunes]
			}
			if messageLoopTextHasAny(string(window), fullAccessDirectionClarificationObjectNouns...) {
				return true
			}
			next := strings.Index(question[index+len(choice):], choice)
			if next < 0 {
				break
			}
			index = index + len(choice) + next
		}
	}
	return false
}

// fullAccessObjectWindowRunes bounds the lookahead that pairs a generic choice
// word with an object noun. Four runes cover 哪个轨道 / which clip and stay short
// enough that 哪个已安装的 EQ 插件 (a plug-in selection question) is not mistaken
// for an object question.
const fullAccessObjectWindowRunes = 4

// messageLoopFullAccessDirectionClarificationBounce feeds one direction
// clarification back through the existing <final_gate> channel and reports
// whether the caller should continue the loop. It returns false once the turn
// has spent its single bounce, which is what keeps the guardrail bounded: the
// repeated request is then delivered to the user unchanged.
func messageLoopFullAccessDirectionClarificationBounce(state *runState, issue, raw string) bool {
	if state == nil || strings.TrimSpace(issue) == "" {
		return false
	}
	if messageLoopFullAccessBounceCount(state) >= fullAccessAutonomyMaxBounces {
		return false
	}
	ctx := cloneMap(state.input.Context)
	if ctx == nil {
		ctx = map[string]any{}
	}
	ctx[fullAccessAutonomyBounceKey] = messageLoopFullAccessBounceCount(state) + 1
	state.input.Context = ctx
	appendMessageLoopDiagnostic(messageLoopDiagnostic{
		Stage:          fullAccessAutonomyDiagnosticStage,
		Error:          issue,
		GoalID:         state.goal.GoalID,
		RunID:          state.goal.RunID,
		ConversationID: messageLoopConversationID(state),
		Raw:            raw,
	})
	state.trace = append(state.trace, planner.TraceEvent{Kind: "clarification", Message: issue})
	state.input.Conversation = append(state.input.Conversation, llm.Message{
		Role: "user", Content: "<final_gate>" + issue + "</final_gate>",
	})
	return true
}

// messageLoopFullAccessBounceCount reads the per-turn bounce counter.
func messageLoopFullAccessBounceCount(state *runState) int {
	if state == nil || state.input.Context == nil {
		return 0
	}
	switch value := state.input.Context[fullAccessAutonomyBounceKey].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
