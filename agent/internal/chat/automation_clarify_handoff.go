package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
)

// ---- M3-CLARIFY-LLM-HANDOFF-1 (card 225) ----
//
// 判断权归属裁定（2026-10-30，decisions/2026-10-30-judgment-ownership.md）：
// 意图识别、执行链、安全兜底归确定性层；「问不问、问什么、提议什么」归 LLM。
// 本文件把词表命中但信息缺失（marker miss / span 候选 / 裸单端 / 类别缺失 /
// 参数缺失）的澄清出口从 Go 固定问句改为**带上下文的 LLM 轮次**：移交上下文
// 携带意图解析结构化结果、候选值、能力目录披露（含段落标记能力）与工程状态
// 摘要，由 LLM 决定问什么或提议什么。
//
// 治理边界（卡面三面不变式）：
//   - LLM 提议≠写入：模型输出的任何命令走既有治理链（chatResponseForCommands
//     的确认/执行分级按目录 spec 既有等级），不猜原则不破；
//   - 219 三面固定问句保留为 LLM 不可用（配置缺失/调用失败/超时/空回复）时的
//     fail-safe 降级路径，不再是正常路径；降级行为在 WorkflowData 如实披露；
//   - 判断权冻结令：本卡后 chat 域不得新增固定澄清文案——本文件所有面向用户
//     的问句文本均来自 LLM 或 219 既有兜底文案。

// automationClarifyHandoffTimeout bounds one hand-off LLM turn. The budget
// spans the whole bounded retry loop (TimeoutIsTotalBudget): a dead endpoint
// fails fast into the fixed-ask fallback instead of stalling the chat turn.
const automationClarifyHandoffTimeout = 45 * time.Second

// automationClarifyHandoffFingerprint tags the hand-off turn in telemetry so
// the smoke leg can tell hand-off rounds from plain chat rounds by source.
const automationClarifyHandoffFingerprint = "automation-clarify-handoff:v1"

// automationClarifyHandoffSystemInstruction is the fixed protocol text for the
// hand-off turn. It describes the protocol (what is handed over, what the
// model may decide, the no-write rule) — it is not a clarification wording:
// every user-facing question/proposal string comes from the model itself
// (judgment-ownership freeze: no new fixed clarify copy in the chat domain).
const automationClarifyHandoffSystemInstruction = `你是 Vit-DAW 的混音助手。用户提出了一段时域自动化（包络）请求，意图层已识别方向与类别，但缺少完成写入所需的部分信息，系统把识别结果、候选值、可用能力目录和工程状态移交给你。问什么、怎么问、是否给出提议，由你判断。

规则：
- 不猜原则：不能替用户拍板时间区间或参数；可以给出候选并明确等用户确认。
- 你没有直接写入权。若你认为现在就该执行某个能力（例如用户提到「副歌」等段落而工程里没有对应标记时，可以提议先打段落标记，或直接请求该能力），用 commands 输出——命令会进入确认/执行治理链，绝不能在回复里假装已写入。
- 能力目录列出了当前可用的命令（含段落标记能力）；只使用其中列出的命令名。
- 回复用简短中文，只输出 JSON：{"reply":"给用户看的话","commands":[]}`

// automationClarifyHandoffCatalogTools is the capability disclosure for the
// hand-off payload: the section-marker family (the capability the judgment
// ruling names — "要不要我先听一遍把副歌标记打上") plus the automation
// observation/write pair the choreography chain itself uses. Read-only
// consumption of the existing catalog (card stop-condition 1 not triggered:
// no registration-surface change).
var automationClarifyHandoffCatalogTools = []string{
	"project.markers.list",
	"project.markers.upsert",
	"project.markers.apply_section_markers",
	"automation.set_points",
	"automation.get_curve",
}

// automationClarifyHandoffEngine resolves the LLM engine for the hand-off
// turn. Unavailable config (missing/incomplete) means the fixed-ask fallback.
func (s *Server) automationClarifyHandoffEngine() (config.EngineConfig, bool) {
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		return config.EngineConfig{}, false
	}
	return cfg, true
}

// automationClarifyHandoffCatalog returns the disclosed capability lines for
// the payload (empty string when the harness/catalog is absent — the LLM then
// sees an honest empty disclosure rather than a fabricated one).
func (s *Server) automationClarifyHandoffCatalog() string {
	if s == nil || s.harness == nil {
		return ""
	}
	return s.harness.ModelCatalogSummaryForTools(automationClarifyHandoffCatalogTools)
}

// automationClarifyHandoffProjectState summarizes the project for the payload:
// visible tracks, the resolved target's plugins and the current section
// markers (kernel reads fail-open to an honest "unavailable" marker).
func (s *Server) automationClarifyHandoffProjectState(ctx context.Context, target *automationChoreoTarget) map[string]any {
	state := map[string]any{}
	if s == nil {
		return state
	}
	if s.harness != nil {
		rows := chatVisibleTrackRows(s.harness.UserStateSummary(ctx))
		tracks := make([]string, 0, len(rows))
		for _, row := range rows {
			name := firstNonEmpty(firstStringFromMap(row, "name"), firstStringFromMap(row, "track_name"), firstStringFromMap(row, "track_id"), firstStringFromMap(row, "id"))
			if strings.TrimSpace(name) != "" {
				tracks = append(tracks, name)
			}
		}
		if len(tracks) > 0 {
			state["tracks"] = tracks
		}
	}
	if target != nil && strings.TrimSpace(target.TrackID) != "" {
		state["target_track"] = map[string]any{
			"id":    target.TrackID,
			"label": target.TrackLabel,
		}
		if names := s.automationTrackPluginNames(ctx, target.TrackID); len(names) > 0 {
			state["target_track"].(map[string]any)["plugins"] = names
		}
	}
	if s.kernel != nil {
		markerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		reply, _, err := s.kernel.SendCommand(markerCtx, map[string]any{"cmd": "project.markers.list"})
		cancel()
		if err == nil && strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "ok") {
			markers := []map[string]any{}
			for _, row := range mapRowsFromAny(reply["markers"]) {
				marker := map[string]any{}
				if name := firstNonEmpty(firstStringFromMap(row, "name", "label", "title"), ""); name != "" {
					marker["name"] = name
				}
				if start, ok := automationMarkerSeconds(row, "start", "start_seconds", "time", "position_seconds"); ok {
					marker["start_seconds"] = start
				}
				if end, ok := automationMarkerSeconds(row, "end", "end_seconds", "length_seconds"); ok {
					marker["end_seconds"] = end
				}
				markers = append(markers, marker)
			}
			state["section_markers"] = markers
		} else {
			state["section_markers"] = "unavailable"
		}
	}
	return state
}

// automationClarifyHandoffPayload assembles the structured hand-off context:
// intent parse result, the deterministic candidates the resolve pass found,
// the capability disclosure and the project summary.
func automationClarifyHandoffPayload(kind, message string, intent automationIntent, candidates map[string]any, catalog string, projectState map[string]any) map[string]any {
	intentView := map[string]any{
		"category":      intent.Category,
		"category_word": intent.CategoryWord,
		"time_varying":  intent.TimeVarying,
	}
	if intent.DirectionUp {
		intentView["direction"] = "up"
	} else if intent.Category != "" || intent.TimeVarying {
		intentView["direction"] = "down_or_unspecified"
	}
	payload := map[string]any{
		"handoff_kind": kind,
		"user_text":    message,
		"intent":       intentView,
		"note":         "信息缺失形态见 handoff_kind；候选值见 candidates（若在）。请判断：问什么、给什么候选、或提议什么能力。",
	}
	if len(candidates) > 0 {
		payload["candidates"] = candidates
	}
	if strings.TrimSpace(catalog) != "" {
		payload["capability_catalog"] = catalog
	}
	if len(projectState) > 0 {
		payload["project_state"] = projectState
	}
	return payload
}

// automationClarifyHandoffTurn is the parsed model output of the hand-off
// turn (same envelope shape as the plain chat pipeline).
type automationClarifyHandoffTurn struct {
	Reply    string
	Commands []map[string]any
}

// automationClarifyHandoffCall runs the single LLM turn with the hand-off
// instruction and payload. Empty replies are treated as failures so the
// caller falls back to the fixed ask instead of rendering an empty card.
func (s *Server) automationClarifyHandoffCall(ctx context.Context, conversationID, goalID, message string, payload map[string]any) (automationClarifyHandoffTurn, error) {
	cfg, ok := s.automationClarifyHandoffEngine()
	if !ok {
		return automationClarifyHandoffTurn{}, fmt.Errorf("llm_config_unavailable")
	}
	payloadJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return automationClarifyHandoffTurn{}, err
	}
	if s.llm == nil {
		return automationClarifyHandoffTurn{}, fmt.Errorf("llm_client_unavailable")
	}
	userContent := "用户话术：" + message + "\n\n移交上下文（JSON）：\n" + string(payloadJSON)
	resp, err := s.llm.CompleteRequest(ctx, cfg, llm.Request{
		Messages: []llm.Message{
			{Role: "system", Content: automationClarifyHandoffSystemInstruction},
			{Role: "user", Content: userContent},
		},
		Metadata: llm.RequestMetadata{
			Source:            "chat",
			ConversationID:    conversationID,
			GoalID:            goalID,
			PromptFingerprint: automationClarifyHandoffFingerprint,
		},
		Timeout:              automationClarifyHandoffTimeout,
		TimeoutIsTotalBudget: true,
		PreferJSON:           true,
	})
	if err != nil {
		return automationClarifyHandoffTurn{}, err
	}
	env := parseModelEnvelope(resp.Text)
	reply := strings.TrimSpace(env.Reply)
	if reply == "" && len(env.Commands) == 0 {
		return automationClarifyHandoffTurn{}, fmt.Errorf("empty_handoff_reply")
	}
	return automationClarifyHandoffTurn{Reply: reply, Commands: env.Commands}, nil
}

// automationChoreoClarifyLLMChatResponse renders the hand-off clarify card:
// same card shape as the fallback ask (automationChoreoClarifyChatResponse)
// with the model's wording plus the honest disclose of where the wording came
// from (WorkflowData.clarify_llm = "handoff" | "fallback:<reason>").
func (s *Server) automationChoreoClarifyLLMChatResponse(conversationID, mode, goalID, runID string, intent automationIntent, reply, kind, llmOutcome string) ChatResponse {
	resp := ChatResponse{
		ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
		Reply:      reply,
		GoalStatus: string(agentruntime.StatusWaitingContinue),
		StopReason: "automation_choreo_clarify_" + kind,
		Workflow:   automationChoreoWorkflow,
		WorkflowData: map[string]any{
			"mutation_performed": false, "clarify_kind": kind, "category_hint": intent.Category,
			"time_varying_intent": intent.TimeVarying, "clarify_llm": llmOutcome,
		},
		InteractionRequests: []AgentInteractionRequest{{
			ID: "interaction_" + randomID(), Kind: "automation_choreo_clarify", Type: "automation_choreo_clarify",
			Workflow: automationChoreoWorkflow, Stage: "clarify", ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Title: "段落自动化：需要补充信息", Body: reply, Status: "waiting",
			Questions: []AgentInteractionField{{ID: "clarify_" + kind, Label: "补充说明", Kind: "text", Required: false, Placeholder: "例如：0 到 30 秒 / 电平"}},
			Actions:   []AgentInteractionAction{{ID: "cancel", Label: "取消", Style: "secondary"}},
		}},
	}
	if s.logger != nil {
		s.logger.Info("[automation.choreo] clarify llm_handoff conversation=%s kind=%s outcome=%s", conversationID, kind, llmOutcome)
	}
	return resp
}

// automationClarifyHandoffTurnFor assembles the hand-off payload (intent,
// candidates, capability disclosure, project state) and runs the single LLM
// turn. Errors mean the caller must render the fixed-ask fallback with an
// honest disclose.
func (s *Server) automationClarifyHandoffTurnFor(ctx context.Context, conversationID, goalID, message, kind string, intent automationIntent, candidates map[string]any, target *automationChoreoTarget) (automationClarifyHandoffTurn, error) {
	payload := automationClarifyHandoffPayload(kind, message, intent, candidates, s.automationClarifyHandoffCatalog(), s.automationClarifyHandoffProjectState(ctx, target))
	return s.automationClarifyHandoffCall(ctx, conversationID, goalID, message, payload)
}

// clarifyHandoffGovernedCommands routes model-proposed commands into the
// existing governed chain — the LLM proposes, the chain decides (confirmation
// card or graded execution per catalog spec). ok=false means the chain did
// not accept the commands: the caller falls through to the text ask or the
// fixed fallback, never fabricating an execution.
func (s *Server) clarifyHandoffGovernedCommands(ctx context.Context, conversationID, message string, turn automationClarifyHandoffTurn) (ChatResponse, bool) {
	if len(turn.Commands) == 0 {
		return ChatResponse{}, false
	}
	if resp, handled := s.chatResponseForCommands(ctx, conversationID, message, turn.Reply, turn.Commands, map[string]any{}); handled {
		if resp.WorkflowData == nil {
			resp.WorkflowData = map[string]any{}
		}
		resp.WorkflowData["clarify_llm"] = "handoff"
		return resp, true
	}
	if s.logger != nil {
		s.logger.Info("[automation.choreo] clarify llm_handoff commands not accepted conversation=%s commands=%d", conversationID, len(turn.Commands))
	}
	return ChatResponse{}, false
}

// automationChoreoClarifyFallbackResponse renders the fixed 219 ask with the
// honest disclose of why the LLM hand-off did not answer (never silently
// pretending the wording was model-made).
func (s *Server) automationChoreoClarifyFallbackResponse(conversationID, mode, goalID, runID, message string, intent automationIntent, kind, fallbackQuestion, disclose string) ChatResponse {
	resp := s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent, fallbackQuestion, kind)
	resp.WorkflowData["clarify_llm"] = disclose
	return resp
}

// automationChoreoClarifyHandoffResponse is the migrated plain-clarify exit
// (kinds: region / category / param). The fixed 219 ask survives verbatim as
// the fallback; the pending clarification candidate stores which wording the
// user actually saw so later turns can stay consistent.
func (s *Server) automationChoreoClarifyHandoffResponse(ctx context.Context, conversationID, mode, goalID, runID, message string, intent automationIntent, kind, fallbackQuestion string, candidates map[string]any) ChatResponse {
	turn, err := s.automationClarifyHandoffTurnFor(ctx, conversationID, goalID, message, kind, intent, candidates, nil)
	if err != nil {
		return s.automationChoreoClarifyFallbackResponse(conversationID, mode, goalID, runID, message, intent, kind, fallbackQuestion, "fallback:"+err.Error())
	}
	if resp, ok := s.clarifyHandoffGovernedCommands(ctx, conversationID, message, turn); ok {
		return resp
	}
	if turn.Reply == "" {
		return s.automationChoreoClarifyFallbackResponse(conversationID, mode, goalID, runID, message, intent, kind, fallbackQuestion, "fallback:empty_handoff_reply")
	}
	candidate := &automationChoreoCandidate{
		ID:        "automation_choreo_" + randomID(),
		CreatedAt: time.Now().UTC(),
		Status:    "pending_confirmation",
		GoalID:    goalID, RunID: runID, UserText: message,
		Category: intent.Category,
		Clarify: map[string]any{"kind": kind, "question": turn.Reply, "intent_time_varying": true,
			"llm_handoff": true, "fallback_question": fallbackQuestion},
	}
	s.storeAutomationChoreoCandidate(conversationID, candidate)
	return s.automationChoreoClarifyLLMChatResponse(conversationID, mode, goalID, runID, intent, turn.Reply, kind, "handoff")
}

// automationChoreoSpanCandidateHandoffResponse is the migrated span-candidate
// clarify exit (marker-miss-with-span and bare-span forms). The candidate
// context (span/section/resolved target) rides the pending state exactly like
// the fallback ask so the affirmative adoption chain stays intact. The fixed
// 219 ask stays inside the fallback function (its wording is the fail-safe).
func (s *Server) automationChoreoSpanCandidateHandoffResponse(ctx context.Context, conversationID, mode, goalID, runID, message string, intent automationIntent, target automationChoreoTarget, sectionName string, spanSeconds float64) ChatResponse {
	candidates := map[string]any{"span_seconds": spanSeconds, "adopt_region_seconds": []float64{0, spanSeconds}}
	if strings.TrimSpace(sectionName) != "" {
		candidates["section_name"] = sectionName
	}
	turn, err := s.automationClarifyHandoffTurnFor(ctx, conversationID, goalID, message, "region_span_candidate", intent, candidates, &target)
	if err != nil {
		resp := s.automationChoreoClarifySpanCandidateResponse(conversationID, mode, goalID, runID, message, intent, target, sectionName, spanSeconds)
		resp.WorkflowData["clarify_llm"] = "fallback:" + err.Error()
		return resp
	}
	if resp, ok := s.clarifyHandoffGovernedCommands(ctx, conversationID, message, turn); ok {
		return resp
	}
	if turn.Reply == "" {
		resp := s.automationChoreoClarifySpanCandidateResponse(conversationID, mode, goalID, runID, message, intent, target, sectionName, spanSeconds)
		resp.WorkflowData["clarify_llm"] = "fallback:empty_handoff_reply"
		return resp
	}
	candidate := &automationChoreoCandidate{
		ID:        "automation_choreo_" + randomID(),
		CreatedAt: time.Now().UTC(),
		Status:    "pending_confirmation",
		GoalID:    goalID, RunID: runID, UserText: message,
		TrackID: target.TrackID, TrackLabel: target.TrackLabel,
		Category: intent.Category, PluginID: target.PluginID, PluginName: target.PluginName, ParamID: target.ParamID,
		Clarify: map[string]any{"kind": "region_span_candidate", "question": turn.Reply, "intent_time_varying": true,
			"span_seconds": spanSeconds, "section_name": sectionName,
			"llm_handoff": true},
	}
	s.storeAutomationChoreoCandidate(conversationID, candidate)
	return s.automationChoreoClarifyLLMChatResponse(conversationID, mode, goalID, runID, intent, turn.Reply, "region_span_candidate", "handoff")
}

// automationChoreoClarifyAffirmHandoff migrates the face-2 affirm guidance:
// while an LLM-worded clarification is pending, a bare affirmative word is a
// judgment call for the model (it knows what it asked — e.g. a marker
// proposal), not a fixed guidance string. Returns ok=false to fall back to
// the 219 differentiated guidance (fail-safe when the LLM turn is down).
// The span-candidate adoption table is NOT routed here: adopting an offered
// span by an explicit affirmative is an execution-layer contract, not a
// judgment (the caller checks that kind before reaching here).
func (s *Server) automationChoreoClarifyAffirmHandoff(ctx context.Context, conversationID, mode string, candidate automationChoreoCandidate, req ChatRequest, message string) (ChatResponse, bool) {
	cfg, cfgOK := s.automationClarifyHandoffEngine()
	if !cfgOK || s.llm == nil {
		return ChatResponse{}, false
	}
	pendingQuestion := firstStringFromMap(candidate.Clarify, "question")
	if strings.TrimSpace(pendingQuestion) == "" {
		return ChatResponse{}, false
	}
	payload := map[string]any{
		"handoff_kind":       "affirm_pending_clarify",
		"user_text":          message,
		"pending_kind":       firstStringFromMap(candidate.Clarify, "kind"),
		"pending_question":   pendingQuestion,
		"original_request":   candidate.UserText,
		"capability_catalog": firstNonEmpty(s.automationClarifyHandoffCatalog(), ""),
		"note":               "用户对先前的澄清回了一个肯定词（如「可以」「好」）。判断用户意图并回应：若你的先前问题是在提议某能力（如打段落标记），肯定词就是对该提议的同意——可用 commands 请求该能力（走确认链）；若仍缺信息，继续用一句问话澄清。",
	}
	turnCtx, cancel := context.WithTimeout(ctx, automationClarifyHandoffTimeout)
	defer cancel()
	payloadJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ChatResponse{}, false
	}
	resp, err := s.llm.CompleteRequest(turnCtx, cfg, llm.Request{
		Messages: []llm.Message{
			{Role: "system", Content: automationClarifyHandoffSystemInstruction},
			{Role: "user", Content: "用户话术：" + message + "\n\n移交上下文（JSON）：\n" + string(payloadJSON)},
		},
		Metadata: llm.RequestMetadata{
			Source: "chat", ConversationID: conversationID, GoalID: candidate.GoalID,
			PromptFingerprint: automationClarifyHandoffFingerprint,
		},
		Timeout: automationClarifyHandoffTimeout, TimeoutIsTotalBudget: true, PreferJSON: true,
	})
	if err != nil {
		return ChatResponse{}, false
	}
	env := parseModelEnvelope(resp.Text)
	reply := strings.TrimSpace(env.Reply)
	if reply == "" && len(env.Commands) == 0 {
		return ChatResponse{}, false
	}
	if len(env.Commands) > 0 {
		if resp, handled := s.chatResponseForCommands(ctx, conversationID, message, reply, env.Commands, req.Context); handled {
			if resp.WorkflowData == nil {
				resp.WorkflowData = map[string]any{}
			}
			resp.WorkflowData["clarify_llm"] = "handoff_affirm_commands"
			return resp, true
		}
	}
	if reply == "" {
		return ChatResponse{}, false
	}
	// Plain-text judgment answer: keep the pending clarification alive (same
	// contract as the 219 guidance — the user can still answer it properly).
	goalID, runID := candidate.GoalID, candidate.RunID
	if strings.TrimSpace(goalID) == "" || strings.TrimSpace(runID) == "" {
		goalID, runID = goalIDsFromContext(req.Context)
	}
	if s.logger != nil {
		s.logger.Info("[automation.choreo] clarify affirm llm_handoff conversation=%s", conversationID)
	}
	return ChatResponse{
		ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
		Reply:      reply,
		GoalStatus: string(agentruntime.StatusWaitingContinue),
		StopReason: "automation_choreo_clarify_affirm_guidance",
		Workflow:   automationChoreoWorkflow,
		WorkflowData: map[string]any{
			"mutation_performed": false, "clarify_kind": firstStringFromMap(candidate.Clarify, "kind"),
			"affirm_guidance": true, "clarify_llm": "handoff",
		},
	}, true
}
