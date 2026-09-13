package chat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/shadow"
)

// CONFIRM-CHAT-1（早窗-2，2026-09-13）单测钉。缺陷工件：goal_20c9933c 工程
// 912.vit，待确认问句「…确认后我就执行，需要我直接做吗？」→ 用户答「需要」→
// 英文诊断串直达用户面（语义入口 unresolved）。
//
// 本文件只钉行为契约，不复制实现表：断言直接驱动真实入口
// （handlePendingMixTickChat / semanticEntryUnresolvedResponse /
// storePendingMixTickCandidateForMode），因此「词表是否够宽」由行为证明。

// 卡片、语义入口与既有钉共用的肯定/否定作答语料（自然中文，非书面明确指令）。
var confirmChat1Affirmatives = []string{
	"需要", "要", "好的", "好", "行", "是的", "对", "嗯", "没问题", "同意", "确定", "可以", "需要的",
}

var confirmChat1Negatives = []string{
	"不用", "不要", "算了", "先不", "取消", "不需要", "暂时不用", "别了",
}

var confirmChat1ASCII = regexp.MustCompile("[A-Za-z]")

func confirmChat1HasCJK(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func confirmChat1PendingServer() *Server {
	server := New(nil, shadow.New(nil), nil)
	server.pendingMixTicks["chat_confirm"] = agentloop.PendingMixTickCandidate{
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
	return server
}

func confirmChat1ExecutedTools(resp ChatResponse) []string {
	tools := make([]string, 0, len(resp.ExecutedKernelReply))
	for _, row := range resp.ExecutedKernelReply {
		tools = append(tools, cleanContextText(row["tool"]))
	}
	return tools
}

func confirmChat1CardActions(card AgentInteractionRequest) []string {
	ids := make([]string, 0, len(card.Actions))
	for _, action := range card.Actions {
		ids = append(ids, action.ID)
	}
	return ids
}

// 钉①：待确认候选在场时，「需要」等自然肯定词必须被消费为确认执行——
// 路由进 typed propose/apply 确认链，而不是停在歧义面、更不是落成无法分类的
// 新请求。自然肯定词全部与「可以执行」同路由（同一执行入口）。
func TestConfirmChatNaturalAffirmativesAreConsumedAsConfirmation(t *testing.T) {
	for _, message := range confirmChat1Affirmatives {
		t.Run(message, func(t *testing.T) {
			server := confirmChat1PendingServer()
			resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_confirm", ChatRequest{
				Message: message,
				Context: map[string]any{"conversation_id": "chat_confirm"},
			}, agentModeDefault)
			if !handled {
				t.Fatalf("natural affirmative %q was not consumed by the pending confirmation surface: %+v", message, resp)
			}
			switch resp.StopReason {
			case "ambiguous_mix_tick_confirmation":
				t.Fatalf("natural affirmative %q was held as ambiguous instead of consumed", message)
			case "semantic_entry_unresolved", "semantic_entry_invalid":
				t.Fatalf("natural affirmative %q fell through to semantic entry", message)
			case "no_pending_mix_tick_candidate":
				t.Fatalf("natural affirmative %q did not see the pending candidate", message)
			}
			tools := confirmChat1ExecutedTools(resp)
			if !testStringSliceContains(tools, "mix.propose_tick") || !testStringSliceContains(tools, "mix.apply_tick") {
				t.Fatalf("affirmative %q did not enter the typed confirmation loop: stop=%s tools=%v reply=%q",
					message, resp.StopReason, tools, resp.Reply)
			}
			if !confirmChat1HasCJK(resp.Reply) {
				t.Fatalf("affirmative %q produced a user-facing reply with no Chinese at all: %q", message, resp.Reply)
			}
			if strings.Contains(resp.Reply, "underspecified") || strings.Contains(resp.Reply, "The user request") {
				t.Fatalf("affirmative %q leaked the semantic-entry diagnostic: %q", message, resp.Reply)
			}
		})
	}
}

// 钉②：对应的自然否定词必须走取消——作废候选、零工程修改、零执行。
func TestConfirmChatNaturalNegativesCancelPendingConfirmation(t *testing.T) {
	for _, message := range confirmChat1Negatives {
		t.Run(message, func(t *testing.T) {
			server := confirmChat1PendingServer()
			resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_confirm", ChatRequest{
				Message: message,
				Context: map[string]any{"conversation_id": "chat_confirm"},
			}, agentModeDefault)
			if !handled || resp.StopReason != "mix_tick_rejected" {
				t.Fatalf("natural negative %q did not route to cancellation: handled=%v resp=%+v", message, handled, resp)
			}
			if len(resp.ExecutedKernelReply) != 0 {
				t.Fatalf("negative %q executed something: %+v", message, resp.ExecutedKernelReply)
			}
			if _, ok := server.pendingMixTicks["chat_confirm"]; ok {
				t.Fatalf("negative %q left the candidate pending", message)
			}
		})
	}
}

// 钉③：分类不可判时的用户面回复必须是中文引导，英文诊断串不得上面
// （只保留在审计/日志面）。err != nil（路由失败）与 route=unresolved（分类
// 失败）两种成因都给中文，且不冒充「已理解你的请求」。
func TestConfirmChatUnresolvedFallbackReplyIsChineseGuidance(t *testing.T) {
	decision := semanticEntryDecision{
		SchemaVersion:   semanticEntryDecisionSchema,
		Route:           semanticEntryRouteUnresolved,
		TargetScope:     semanticEntryScopeNone,
		ControlMode:     semanticEntryControlNone,
		UserAuthorization: semanticEntryAuthorizationNone,
		Confidence:      0.2,
		Reason:          "ambiguous",
		RejectionReason: "The user request '需要' is too underspecified to determine whether they want discussion, observation, or control.",
	}
	unresolved := semanticEntryUnresolvedResponse("chat_confirm", agentModeDefault, &decision, nil)
	if confirmChat1ASCII.MatchString(unresolved.Reply) {
		t.Fatalf("unclassified fallback exposed an English diagnostic to the user: %q", unresolved.Reply)
	}
	if !strings.Contains(unresolved.Reply, "确认") {
		t.Fatalf("unclassified fallback must offer the confirmation branch: %q", unresolved.Reply)
	}
	if unresolved.StopReason != "semantic_entry_unresolved" {
		t.Fatalf("machine-readable stop reason changed: %q", unresolved.StopReason)
	}
	if !strings.Contains(unresolved.Reply, "我没太明白") {
		t.Fatalf("unclassified fallback must be an explicit clarification prompt: %q", unresolved.Reply)
	}
	if unresolved.GoalStatus != "waiting_clarification" {
		t.Fatalf("an unclassified request that asks the user a question must stay answerable, got %q", unresolved.GoalStatus)
	}
	diagnostic := strings.ToLower(cleanContextText(decision.RejectionReason))
	if !strings.Contains(strings.ToLower(mapString(unresolved.WorkflowData)), "underspecified") {
		t.Fatalf("the model diagnostic must stay in the audit plane: %+v", unresolved.WorkflowData)
	}
	if diagnostic == "" {
		t.Fatal("fixture lost its diagnostic")
	}

	routingFailure := semanticEntryUnresolvedResponse("chat_confirm", agentModeDefault, &decision, errors.New("capability route planner failed"))
	if confirmChat1ASCII.MatchString(routingFailure.Reply) {
		t.Fatalf("routing-failure fallback exposed an English diagnostic to the user: %q", routingFailure.Reply)
	}
	if !strings.Contains(routingFailure.Reply, "没有修改工程") {
		t.Fatalf("routing-failure fallback must state that the project is untouched: %q", routingFailure.Reply)
	}
}

func mapString(values map[string]any) string {
	var builder strings.Builder
	for key, value := range values {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(cleanContextText(value))
		if nested, ok := value.(map[string]any); ok {
			builder.WriteString(mapString(nested))
		}
		builder.WriteString(";")
	}
	return builder.String()
}

// 钉④：卡片在场性（按取证结论）。取证=发射面在 chat 侧：候选落盘必发
// mix_tick.pending 事件（timeline 面），可应答卡（approve/cancel）随响应
// 投递并落 s.interactions 供 respond 面解析。缺口在**作答回合**：未被消费的
// 作答（问句复述、读不懂的短应答）走 hold 但只回文本、卡不在场，用户失去
// 可点入口（B6 以来「卡必须在场」的契约）。修后：确认未消费 ⇒ 响应必带
// 可应答卡，且卡可从存储面解析。
func TestConfirmChatHeldAnswerKeepsAnswerableCardInForce(t *testing.T) {
	server := confirmChat1PendingServer()
	server.storePendingMixTickCandidateForMode("chat_confirm_event", "goal_c1", "run_c1", agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust", TrackID: "track_1", DeltaDB: -1, ObservationID: "obs_c1",
		Status: "pending_confirmation",
	}, false)
	events, _ := server.agentEventsSince("chat_confirm_event", 0, 64)
	if b6FindMixTickPending(events) == nil {
		t.Fatal("a stored pending candidate must emit the mix_tick.pending event (timeline face)")
	}

	for _, message := range []string{"要不要", "谢谢"} {
		t.Run(message, func(t *testing.T) {
			server := confirmChat1PendingServer()
			resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_confirm", ChatRequest{
				Message: message,
				Context: map[string]any{"conversation_id": "chat_confirm"},
			}, agentModeDefault)
			if !handled {
				t.Fatalf("an unreadable answer to a live confirmation must be held, not dropped: %+v", resp)
			}
			if len(resp.ExecutedKernelReply) != 0 {
				t.Fatalf("a held answer must not touch the project: %+v", resp.ExecutedKernelReply)
			}
			if _, ok := server.pendingMixTicks["chat_confirm"]; !ok {
				t.Fatal("a held answer must keep the pending candidate for the retry the card offers")
			}
			if len(resp.InteractionRequests) == 0 {
				t.Fatalf("held answer %q left the answerable card out of场: %+v", message, resp)
			}
			card := resp.InteractionRequests[0]
			if !strings.EqualFold(card.Kind, "mix_tick_confirmation") || card.Status != "waiting_for_user" {
				t.Fatalf("held answer %q surfaced a non-answerable card: %+v", message, card)
			}
			actions := confirmChat1CardActions(card)
			if !testStringSliceContains(actions, "approve") || !testStringSliceContains(actions, "cancel") {
				t.Fatalf("held answer %q surfaced a card without approve/cancel: %v", message, actions)
			}
			if _, ok := server.peekPendingInteraction(card.ID); !ok {
				t.Fatalf("the re-surfaced card %q must be resolvable through the respond surface", card.ID)
			}
		})
	}
}

// 钉④b（反向锁定）：无待确认候选时，自然肯定词不得被混音单步确认面吞掉——
// 它必须继续落到既有普通肯定词守卫/语义入口（「没有待确认的动作」语义零回退）。
func TestConfirmChatNaturalAffirmativeWithoutPendingKeepsExistingGuard(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	for _, message := range []string{"需要", "要", "嗯", "没问题"} {
		resp, handled := server.handlePendingMixTickChat(context.Background(), "chat_no_pending", ChatRequest{
			Message: message,
			Context: map[string]any{"conversation_id": "chat_no_pending"},
		}, agentModeDefault)
		if handled {
			t.Fatalf("without a pending candidate %q must not be consumed as a mix-tick confirmation: %+v", message, resp)
		}
		if resp.StopReason == "no_pending_mix_tick_candidate" {
			t.Fatalf("%q must keep the existing plain-approval guard path, got %+v", message, resp)
		}
	}
}

// 钉⑤（应答一致性规则，机械口径钉）：待确认问句模板使用的动词本身进入必然是
// 表；问句形态的复述进入可接受集但停在歧义面；否定优先于肯定；语气助词与叠词
// 归一。全部是闭合词表的机械匹配，不含模型自由文本推导。
func TestConfirmChatAnswerVocabularyIsConsistentWithQuestionVerbs(t *testing.T) {
	for _, message := range []string{"需要", "要", "是", "可以", "行", "好", "对", "需要的", "好的好的", "嗯嗯", "没问题", "同意", "确定"} {
		if !messageNaturalMixTickApproval(message) {
			t.Fatalf("question-verb affirmative %q is not in the accepted answer set", message)
		}
	}
	for _, message := range []string{"要不要", "是否", "可不可以", "能不能", "行不行", "好不好"} {
		if messageNaturalMixTickApproval(message) {
			t.Fatalf("question echo %q must not be read as consent", message)
		}
		if !messageEchoesMixTickConfirmQuestion(message) {
			t.Fatalf("question echo %q must still be recognized as an answer to the confirmation", message)
		}
	}
	if messageNaturalMixTickApproval("不要") || !messageNaturalMixTickCancel("不要") {
		t.Fatal("negation must win over affirmation: a bare negative is a cancellation, never a 要")
	}
	if !messageNaturalMixTickCancel("不需要") || messageNaturalMixTickApproval("不需要") {
		t.Fatal("a negated question verb must cancel, not approve")
	}
	// 反向锁定：自然肯定词不进「明确执行指令」表，否则无待确认候选时会抢走既有
	// 普通肯定词守卫的中文回执（TestPlainApprovalWithoutPendingDoesNotRunAgentLoop）。
	if messageExplicitMixTickApply("需要") || messageExplicitMixTickApply("好的") {
		t.Fatal("natural affirmatives must not enter the explicit-apply table")
	}
	// 反向锁定：短直令（含动作/目标词或数字）不是「读不懂的短应答」，必须继续走
	// 语境切换/语义入口澄清通道。
	if messageBareShortPendingReply("把低音轨提1dB") {
		t.Fatal("a short direct mix command must not be held as an unreadable confirmation answer")
	}
}

// 钉⑥：歧义中间分支的中文追问句逐字保留（既有语义零回退），并点名现在也认的
// 自然肯定词，使「问句的动词」与「可接受作答」一致。
func TestConfirmChatAmbiguousHoldKeepsPinnedWording(t *testing.T) {
	candidate := agentloop.PendingMixTickCandidate{Operation: "track_gain_adjust", TrackID: "track_1", DeltaDB: -1}
	for _, message := range []string{"可以", "要不要", "谢谢"} {
		reply := mixTickPendingHoldReply(candidate, message)
		if !strings.Contains(reply, "我还没有执行。刚才待确认的是：") ||
			!strings.Contains(reply, "要执行请明确说“可以执行”；如果只是继续讨论，我会保持不动。") {
			t.Fatalf("ambiguous hold wording for %q lost its pinned sentence: %q", message, reply)
		}
		if strings.Contains(reply, "underspecified") || strings.Contains(reply, "The user request") {
			t.Fatalf("ambiguous hold wording for %q leaked the semantic-entry diagnostic: %q", message, reply)
		}
		if !strings.Contains(reply, "需要") {
			t.Fatalf("ambiguous hold wording for %q must name the natural affirmative it now accepts: %q", message, reply)
		}
	}
}

// 钉⑦（日志面）：英文诊断串只进日志——它必须出现在 agent 日志里（可审计、可复现），
// 且同一轮的用户面 Reply 一个英文字母都没有。运行态日志写 t.TempDir()。
func TestConfirmChatUnresolvedDiagnosticGoesToLogPlane(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "agent_confirm_chat.log")
	server := &Server{logger: logx.New(false, logPath, 256)}
	decision := semanticEntryDecision{
		SchemaVersion:     semanticEntryDecisionSchema,
		Route:             semanticEntryRouteUnresolved,
		TargetScope:       semanticEntryScopeNone,
		ControlMode:       semanticEntryControlNone,
		UserAuthorization: semanticEntryAuthorizationNone,
		Confidence:        0.2,
		Reason:            "ambiguous",
		RejectionReason:   "The user request '需要' is too underspecified to determine whether they want discussion, observation, or control.",
	}
	resp := server.semanticEntryUnresolvedChatResponse("chat_confirm_log", agentModeDefault, &decision, nil)
	if confirmChat1ASCII.MatchString(resp.Reply) {
		t.Fatalf("user-facing reply must stay Chinese: %q", resp.Reply)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read agent log: %v", err)
	}
	logged := string(raw)
	for _, required := range []string{"[semantic.entry] unresolved fallback", "chat_confirm_log", "underspecified"} {
		if !strings.Contains(logged, required) {
			t.Fatalf("log plane lost %q; log=%s", required, logged)
		}
	}
}
