package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
)

const (
	semanticTreatmentWorkflow = "semantic_treatment_strategy"
	semanticTreatmentSchema   = "semantic_treatment_strategy.v1"
)

type semanticTreatmentInstance struct {
	Key                 string         `json:"instance_key"`
	TrackID             string         `json:"track_id"`
	PluginID            string         `json:"plugin_id"`
	PluginName          string         `json:"plugin_name,omitempty"`
	ProcessorType       string         `json:"processor_type"`
	QualificationStatus string         `json:"qualification_status"`
	NextPlanner         string         `json:"next_planner"`
	Topology            map[string]any `json:"generic_eq_topology,omitempty"`
	Limitation          string         `json:"limitation,omitempty"`
}

type semanticTreatmentChoice struct {
	ChoiceKey      string `json:"choice_key"`
	Role           string `json:"role"`
	Title          string `json:"title"`
	ProcessorType  string `json:"processor_type"`
	TargetMode     string `json:"target_mode"`
	InstanceKey    string `json:"instance_key,omitempty"`
	Reason         string `json:"reason"`
	ExpectedEffect string `json:"expected_effect"`
	Tradeoff       string `json:"tradeoff,omitempty"`
	Confidence     string `json:"confidence"`
	NextPlanner    string `json:"next_planner"`
	MateriallyDiff string `json:"material_difference,omitempty"`
}

type semanticTreatmentPlan struct {
	SchemaVersion     string                    `json:"schema_version"`
	DecisionMode      string                    `json:"decision_mode"`
	UserGoal          string                    `json:"user_goal"`
	Summary           string                    `json:"summary"`
	Choices           []semanticTreatmentChoice `json:"choices"`
	GlobalConstraints []string                  `json:"global_constraints,omitempty"`
	EvidenceRefs      []string                  `json:"evidence_refs,omitempty"`
	Limitations       []string                  `json:"limitations,omitempty"`
}

func ordinaryAgentSemanticEQMutationRequest(userText string, requestContext map[string]any) bool {
	return contextHasAnyValue(requestContext, "selected_track_id", "selected_plugin_track_id") &&
		ordinaryAgentSemanticEQTextTopic(userText) && ordinaryAgentSemanticEQMutationIntent(userText, true)
}

// This is deliberately only a routing gate. It identifies open-ended audible
// change requests; the LLM, not these words, chooses the treatment method.
func ordinaryAgentTreatmentStrategyIntent(userText string, requestContext map[string]any) bool {
	if !contextHasAnyValue(requestContext, "selected_track_id", "selected_plugin_track_id") ||
		ordinaryAgentPluginRecommendationIntent(userText, requestContext) ||
		ordinaryAgentSemanticEQMutationRequest(userText, requestContext) {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || agentLoopTextHasAny(text,
		"为什么", "為什麼", "什么原因", "什麼原因", "分析一下", "解释一下", "解釋一下",
		"why", "what causes", "how do you know", "analyse", "analyze", "explain") {
		return false
	}
	goal := agentLoopTextHasAny(text,
		"靠前", "靠后", "靠後", "有力量", "更有力", "温暖", "溫暖", "更厚", "更薄", "更贴", "更貼",
		"更稳", "更穩", "更松", "更紧", "更緊", "更清晰", "更通透", "更自然", "更有空间", "更有空間",
		"forward", "up front", "powerful", "stronger", "warmer", "thicker", "thinner", "closer", "stable", "tighter", "clearer", "open", "natural", "depth")
	action := agentLoopTextHasAny(text,
		"让", "讓", "使", "变", "變", "更", "弄得", "处理", "處理", "调整", "調整", "改善", "增加", "减少", "減少",
		"make", "bring", "move", "sound", "increase", "reduce", "improve", "adjust")
	return goal && action
}

func (s *Server) semanticTreatmentInstances(ctx context.Context, trackID string) ([]semanticTreatmentInstance, string) {
	if s == nil || s.harness == nil || strings.TrimSpace(trackID) == "" {
		return nil, ""
	}
	state := s.harness.UserStateSummary(ctx)
	refs := chatVisiblePluginRefs(state, trackID)
	if len(refs) > 16 {
		refs = refs[:16]
	}
	out := make([]semanticTreatmentInstance, 0, len(refs))
	for index, ref := range refs {
		instance := semanticTreatmentInstance{
			Key:                 fmt.Sprintf("loaded_instance_%d", index+1),
			TrackID:             ref.TrackID,
			PluginID:            ref.ID,
			PluginName:          ref.Name,
			ProcessorType:       "unknown",
			QualificationStatus: "identity_only",
			NextPlanner:         "capability_boundary",
			Limitation:          "The loaded identity is real, but no ordinary-Agent abstract parameter planner has qualified it.",
		}
		if _, summary, err := s.readLiveEQControlSurface(ctx, ref.TrackID, ref.ID); err == nil && len(summary) > 0 {
			instance.ProcessorType = "eq"
			instance.QualificationStatus = "generic_static_eq_qualified"
			instance.NextPlanner = "semantic_eq"
			instance.Topology = semanticEQTopologyPromptSummary(ref.TrackID, ref.ID, summary)
			instance.Limitation = ""
		} else if err != nil {
			instance.Limitation = err.Error()
		}
		out = append(out, instance)
	}
	return out, semanticTreatmentStateToken(state, trackID)
}

func semanticTreatmentStateToken(state map[string]any, trackID string) string {
	refs := chatVisiblePluginRefs(state, trackID)
	rows := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		rows = append(rows, map[string]any{"track_id": ref.TrackID, "plugin_id": ref.ID, "plugin_name": ref.Name})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left := firstStringFromMap(rows[i], "track_id") + "\x00" + firstStringFromMap(rows[i], "plugin_id")
		right := firstStringFromMap(rows[j], "track_id") + "\x00" + firstStringFromMap(rows[j], "plugin_id")
		return left < right
	})
	payload := map[string]any{
		"track_id": trackID, "plugins": rows,
		"project_uuid":   firstStringFromMap(state, "project_uuid", "project_id"),
		"project_path":   firstStringFromMap(state, "project_path", "current_project_path"),
		"graph_revision": state["graph_revision"], "project_revision": state["project_revision"],
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return "treatment_state_" + hex.EncodeToString(sum[:12])
}

func semanticTreatmentQualifiedEQ(instances []semanticTreatmentInstance) []semanticTreatmentInstance {
	out := make([]semanticTreatmentInstance, 0, len(instances))
	for _, instance := range instances {
		if instance.ProcessorType == "eq" && instance.QualificationStatus == "generic_static_eq_qualified" && instance.NextPlanner == "semantic_eq" {
			out = append(out, instance)
		}
	}
	return out
}

func semanticTreatmentFindInstance(instances []semanticTreatmentInstance, key string) (semanticTreatmentInstance, bool) {
	for _, instance := range instances {
		if instance.Key == key {
			return instance, true
		}
	}
	return semanticTreatmentInstance{}, false
}

func semanticTreatmentFindPlugin(instances []semanticTreatmentInstance, pluginID string) (semanticTreatmentInstance, bool) {
	for _, instance := range instances {
		if instance.PluginID == pluginID {
			return instance, true
		}
	}
	return semanticTreatmentInstance{}, false
}

func (s *Server) planSemanticTreatment(ctx context.Context, conversationID, userText, trackID, trackName string,
	observation *agentloop.RecentObservation, instances []semanticTreatmentInstance, forceInstanceChoice, nativeHandoffAvailable bool, cfg config.EngineConfig) (semanticTreatmentPlan, error) {
	if s == nil || s.llm == nil {
		return semanticTreatmentPlan{}, fmt.Errorf("semantic treatment LLM is unavailable")
	}
	if forceInstanceChoice && len(instances) == 0 {
		return semanticTreatmentPlan{}, fmt.Errorf("no qualified loaded EQ instance is available")
	}
	input := map[string]any{
		"user_request":                          userText,
		"target_scope":                          map[string]any{"kind": "track", "track_refs": []string{trackID}, "label": trackName},
		"relationship_refs":                     []string{},
		"observation_context":                   semanticEQPlannerObservation(observation),
		"loaded_plugin_instances":               instances,
		"allowed_load_required_processor_types": []string{"eq", "compressor", "reverb", "delay", "distortion", "limiter"},
		"native_agent_result_handoff_available": nativeHandoffAvailable,
		"planning_mode":                         map[bool]string{true: "loaded_eq_instance_arbitration", false: "treatment_method_arbitration"}[forceInstanceChoice],
	}
	inputJSON, _ := json.Marshal(input)
	system := `You are the treatment-strategy phase of an ordinary DAW Agent. This is a horizontal capability, not B4 or any A-F specialist workflow.
The user request, exact target scope, optional observation evidence, and real loaded instances are supplied as JSON. You own the acoustic and musical method judgement. Deterministic code owns identity binding, qualification, capability boundaries, confirmation, execution, readback, rollback, and verification.

Return ONLY one semantic_treatment_strategy.v1 JSON object:
{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct|choice_required","user_goal":"copy user goal","summary":"short summary in the user's language","choices":[{"choice_key":"stable unique key","role":"recommended|alternative","title":"short method label","processor_type":"eq|compressor|reverb|delay|distortion|limiter|native","target_mode":"existing_plugin|load_required|native","instance_key":"exact supplied key when existing_plugin","reason":"task-specific reason","expected_effect":"audible intent","tradeoff":"meaningful difference or limitation","confidence":"low|medium|high","next_planner":"semantic_eq|plugin_recommendation|existing_agent_result|capability_boundary","material_difference":"why this is not a duplicate"}],"global_constraints":[],"evidence_refs":[],"limitations":[]}

Rules:
- Do not invent a track, plugin, instance_key, observation, or executable capability.
- An existing_plugin choice must use an exact supplied instance_key. Only generic_static_eq_qualified instances may use next_planner=semantic_eq. An identity_only instance must use capability_boundary.
- A load_required choice omits instance_key and uses next_planner=plugin_recommendation. It means recommend/select/load first, not that parameters are already controllable.
- A native choice is allowed only when native_agent_result_handoff_available=true. It must be the sole direct recommendation and use next_planner=existing_agent_result; never place native in a three-way selection because that result cannot be frozen across this choice interaction.
- Use decision_mode=direct with exactly one recommended choice when one method is clearly appropriate and no meaningful user decision remains.
- Use decision_mode=choice_required only when methods have substantively different audible results, workflows, or tradeoffs. Then return exactly one recommended choice and two materially different alternatives.
- Do not pad alternatives with cosmetic variations. EQ versus compression versus saturation/space can be material; two copies of the same strategy are not.
- Respect an explicitly requested processor family as a hard method constraint.
- Preserve shared negative constraints in global_constraints.
- Do not output parameter values. A downstream planner owns concrete parameters after strategy selection.
- Do not use profile, learn, SPAL, B4, plugin-specific control rules, or network search.`
	if forceInstanceChoice {
		system += `
- This request is loaded-EQ instance arbitration. Use only existing_plugin choices from supplied generic_static_eq_qualified instances, processor_type=eq, next_planner=semantic_eq, and decision_mode=choice_required. Offer up to three real instances (one recommended, remaining alternatives); when only one is supplied, return that one recommended choice without inventing alternatives.`
	}
	request := llm.Request{
		Messages:   []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(inputJSON)}},
		Metadata:   llm.RequestMetadata{Source: "ordinary_agent_semantic_treatment_strategy", ConversationID: conversationID},
		PreferJSON: true,
	}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return semanticTreatmentPlan{}, err
	}
	plan, decodeErr := decodeAndValidateSemanticTreatmentPlan(response.Text, userText, instances, forceInstanceChoice, nativeHandoffAvailable)
	if decodeErr == nil {
		return plan, nil
	}
	repair := fmt.Sprintf("Your previous semantic treatment JSON was invalid: %s\nReturn ONLY a corrected semantic_treatment_strategy.v1 object using the same supplied identities and capabilities.", decodeErr)
	request.Messages = append(request.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: repair})
	response, err = s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return semanticTreatmentPlan{}, err
	}
	plan, err = decodeAndValidateSemanticTreatmentPlan(response.Text, userText, instances, forceInstanceChoice, nativeHandoffAvailable)
	if err != nil {
		return semanticTreatmentPlan{}, fmt.Errorf("semantic treatment strategy remained invalid after repair: %w", err)
	}
	return plan, nil
}

func decodeAndValidateSemanticTreatmentPlan(text, userText string, instances []semanticTreatmentInstance, forceInstanceChoice bool, nativeHandoffAvailable ...bool) (semanticTreatmentPlan, error) {
	var plan semanticTreatmentPlan
	if err := decodePluginRecommendationJSON(text, &plan); err != nil {
		return plan, err
	}
	if plan.SchemaVersion != semanticTreatmentSchema {
		return plan, fmt.Errorf("schema_version must be %s", semanticTreatmentSchema)
	}
	if strings.TrimSpace(plan.UserGoal) == "" {
		plan.UserGoal = strings.TrimSpace(userText)
	}
	plan.DecisionMode = strings.ToLower(strings.TrimSpace(plan.DecisionMode))
	if forceInstanceChoice {
		if plan.DecisionMode != "choice_required" {
			return plan, fmt.Errorf("loaded instance arbitration requires decision_mode=choice_required")
		}
		if len(plan.Choices) < 1 || len(plan.Choices) > 3 || len(plan.Choices) > len(instances) {
			return plan, fmt.Errorf("instance arbitration choices must contain 1-3 real instances")
		}
	} else {
		switch plan.DecisionMode {
		case "direct":
			if len(plan.Choices) != 1 {
				return plan, fmt.Errorf("direct strategy requires exactly one choice")
			}
		case "choice_required":
			if len(plan.Choices) != 3 {
				return plan, fmt.Errorf("choice_required strategy requires one recommendation and two alternatives")
			}
		default:
			return plan, fmt.Errorf("decision_mode must be direct or choice_required")
		}
	}
	seenChoice := map[string]bool{}
	seenInstance := map[string]bool{}
	seenSignature := map[string]bool{}
	nativeAllowed := len(nativeHandoffAvailable) > 0 && nativeHandoffAvailable[0]
	for index := range plan.Choices {
		choice := &plan.Choices[index]
		choice.ChoiceKey = strings.TrimSpace(choice.ChoiceKey)
		choice.Role = strings.ToLower(strings.TrimSpace(choice.Role))
		choice.ProcessorType = semanticTreatmentProcessorType(choice.ProcessorType)
		choice.TargetMode = strings.ToLower(strings.TrimSpace(choice.TargetMode))
		choice.InstanceKey = strings.TrimSpace(choice.InstanceKey)
		choice.NextPlanner = strings.ToLower(strings.TrimSpace(choice.NextPlanner))
		choice.Confidence = strings.ToLower(strings.TrimSpace(choice.Confidence))
		if choice.ChoiceKey == "" || seenChoice[choice.ChoiceKey] {
			return plan, fmt.Errorf("choice %d requires a unique choice_key", index+1)
		}
		seenChoice[choice.ChoiceKey] = true
		wantRole := "alternative"
		if index == 0 {
			wantRole = "recommended"
		}
		if choice.Role != wantRole {
			return plan, fmt.Errorf("choice %d role must be %s", index+1, wantRole)
		}
		if strings.TrimSpace(choice.Reason) == "" || strings.TrimSpace(choice.ExpectedEffect) == "" {
			return plan, fmt.Errorf("choice %d requires reason and expected_effect", index+1)
		}
		switch choice.Confidence {
		case "low", "medium", "high":
		default:
			return plan, fmt.Errorf("choice %d confidence is invalid", index+1)
		}
		signature := choice.TargetMode + ":" + choice.ProcessorType + ":" + choice.InstanceKey
		if seenSignature[signature] {
			return plan, fmt.Errorf("choice %d duplicates another treatment strategy", index+1)
		}
		seenSignature[signature] = true
		switch choice.TargetMode {
		case "existing_plugin":
			instance, ok := semanticTreatmentFindInstance(instances, choice.InstanceKey)
			if !ok || seenInstance[choice.InstanceKey] {
				return plan, fmt.Errorf("choice %d uses an unknown or duplicate instance_key", index+1)
			}
			seenInstance[choice.InstanceKey] = true
			expectedPlanner := "capability_boundary"
			if instance.QualificationStatus == "generic_static_eq_qualified" && choice.ProcessorType == "eq" {
				expectedPlanner = "semantic_eq"
			}
			if choice.NextPlanner != expectedPlanner {
				return plan, fmt.Errorf("choice %d next_planner exceeds qualified instance capability", index+1)
			}
		case "load_required":
			if choice.InstanceKey != "" || choice.NextPlanner != "plugin_recommendation" || choice.ProcessorType == "" || choice.ProcessorType == "native" {
				return plan, fmt.Errorf("choice %d has an invalid load_required handoff", index+1)
			}
		case "native":
			if choice.InstanceKey != "" || choice.NextPlanner != "existing_agent_result" || !nativeAllowed || plan.DecisionMode != "direct" {
				return plan, fmt.Errorf("choice %d has an invalid native capability boundary", index+1)
			}
		default:
			return plan, fmt.Errorf("choice %d target_mode is invalid", index+1)
		}
		if forceInstanceChoice && (choice.TargetMode != "existing_plugin" || choice.ProcessorType != "eq" || choice.NextPlanner != "semantic_eq") {
			return plan, fmt.Errorf("instance arbitration may only select qualified existing EQ instances")
		}
	}
	return plan, nil
}

func semanticTreatmentProcessorType(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "native") {
		return "native"
	}
	return canonicalPluginRecommendationProcessorType(value)
}

func semanticTreatmentObservationPayload(observation *agentloop.RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	bounded := semanticEQPlannerObservation(observation)
	out := map[string]any{
		"tool_call_id": observation.ToolCallID, "tool": observation.Tool, "command_name": observation.CommandName,
		"status": observation.Status, "error": observation.Error,
	}
	if summary := firstMapFromAny(bounded["summary"]); len(summary) > 0 {
		out["summary"] = summary
	} else {
		out["summary"] = bounded
	}
	return out
}

func semanticTreatmentObservationFromPayload(payload map[string]any) *agentloop.RecentObservation {
	row := firstMapFromAny(payload["observation_context"])
	if len(row) == 0 {
		return nil
	}
	return &agentloop.RecentObservation{
		ToolCallID: firstStringFromMap(row, "tool_call_id"), Tool: firstStringFromMap(row, "tool"),
		CommandName: firstStringFromMap(row, "command_name"), Status: firstStringFromMap(row, "status"),
		Error: firstStringFromMap(row, "error"), Summary: cloneContext(firstMapFromAny(row["summary"])),
	}
}

func (s *Server) semanticTreatmentSelectionResponse(conversationID, mode, trackID, trackName, stateToken string,
	requestContext map[string]any, res agentloop.Result, plan semanticTreatmentPlan, instances []semanticTreatmentInstance) ChatResponse {
	instanceByKey := map[string]semanticTreatmentInstance{}
	for _, instance := range instances {
		instanceByKey[instance.Key] = instance
	}
	rows := make([]map[string]any, 0, len(plan.Choices))
	reviews := make([]AgentInteractionReview, 0, len(plan.Choices))
	actions := make([]AgentInteractionAction, 0, len(plan.Choices)+1)
	lines := []string{firstNonEmpty(strings.TrimSpace(plan.Summary), "我已经根据目标、观察证据和当前可用实例形成处理策略。")}
	for index, choice := range plan.Choices {
		row := semanticTreatmentChoiceRow(choice, instanceByKey[choice.InstanceKey])
		rows = append(rows, row)
		roleLabel := "备选"
		if index == 0 {
			roleLabel = "主推荐"
		}
		lines = append(lines, fmt.Sprintf("%s：%s——%s", roleLabel, firstNonEmpty(choice.Title, pluginRecommendationProcessorLabel(choice.ProcessorType)), choice.Reason))
		body := strings.TrimSpace(choice.Reason) + "\n预期：" + strings.TrimSpace(choice.ExpectedEffect)
		if strings.TrimSpace(choice.Tradeoff) != "" {
			body += "\n取舍：" + strings.TrimSpace(choice.Tradeoff)
		}
		reviews = append(reviews, AgentInteractionReview{ID: choice.ChoiceKey, Title: roleLabel + " · " + firstNonEmpty(choice.Title, pluginRecommendationProcessorLabel(choice.ProcessorType)), Body: body, Status: choice.Role, Payload: row})
		actions = append(actions, AgentInteractionAction{
			ID: "select_" + choice.ChoiceKey, Label: "选择 " + firstNonEmpty(choice.Title, pluginRecommendationProcessorLabel(choice.ProcessorType)),
			Style: map[bool]string{true: "primary", false: "secondary"}[index == 0], Description: choice.Reason, Recommended: index == 0,
			Value: map[string]any{"choice_key": choice.ChoiceKey},
		})
	}
	actions = append(actions, AgentInteractionAction{ID: "cancel", Label: "暂不处理", Style: "secondary"})
	payload := map[string]any{
		"schema_version": semanticTreatmentSchema, "status": "awaiting_selection", "decision_mode": plan.DecisionMode,
		"listening_goal": plan.UserGoal, "summary": plan.Summary,
		"target_scope":      map[string]any{"kind": "track", "track_refs": []string{trackID}, "label": trackName},
		"relationship_refs": []string{}, "loaded_instances": instances, "choices": rows,
		"global_constraints": plan.GlobalConstraints, "evidence_refs": plan.EvidenceRefs, "limitations": plan.Limitations,
		"state_token": stateToken, "selection_performed": false, "mutation_performed": false,
		"request_context": cloneContext(requestContext), "observation_context": semanticTreatmentObservationPayload(res.RecentObservation),
	}
	res.ExecutionMemory.PendingMixTreatment = nil
	res.Status = agentruntime.StatusWaitingClarification
	res.StopReason = "semantic_treatment_selection_required"
	res.Error = ""
	res.Continuation = nil
	res.Reply = strings.Join(lines, "\n") + "\n选择只会进入对应的参数计划或插件推荐阶段；此时不会修改工程。"
	resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	resp.NeedsConfirmation = false
	resp.MessageKind = "proposal"
	resp.Workflow = semanticTreatmentWorkflow
	resp.WorkflowData = payload
	req := AgentInteractionRequest{
		ID: "interaction_" + randomID(), Kind: "semantic_treatment_question", Type: "semantic_treatment_selection",
		Source: "semantic_treatment", Workflow: semanticTreatmentWorkflow, Stage: "awaiting_selection",
		Title: "处理策略与插件实例选择", Body: res.Reply, Status: "waiting_for_user",
		ConversationID: conversationID, GoalID: res.GoalID, RunID: res.RunID,
		ReviewItems: reviews, Payload: payload, Data: payload, Actions: actions,
	}
	s.storePendingInteraction(req, payload)
	resp.InteractionRequests = []AgentInteractionRequest{req}
	s.attachInteractionRequests(&resp)
	return resp
}

func semanticTreatmentChoiceRow(choice semanticTreatmentChoice, instance semanticTreatmentInstance) map[string]any {
	row := map[string]any{
		"choice_key": choice.ChoiceKey, "role": choice.Role, "title": choice.Title,
		"processor_type": choice.ProcessorType, "target_mode": choice.TargetMode, "instance_key": choice.InstanceKey,
		"reason": choice.Reason, "expected_effect": choice.ExpectedEffect, "tradeoff": choice.Tradeoff,
		"confidence": choice.Confidence, "next_planner": choice.NextPlanner, "material_difference": choice.MateriallyDiff,
	}
	if instance.PluginID != "" {
		row["bound_instance"] = instance
	}
	return row
}

func recoverSemanticTreatmentInteractionFromPayload(interactionID string, payload map[string]any) (PendingInteraction, bool) {
	if firstStringFromMap(payload, "schema_version") != semanticTreatmentSchema ||
		firstStringFromMap(payload, "status") != "awaiting_selection" || len(mapRowsValue(payload["choices"])) == 0 {
		return PendingInteraction{}, false
	}
	requestContext := cloneContext(firstMapFromAny(payload["request_context"]))
	if requestContext == nil {
		requestContext = map[string]any{}
	}
	requestContext["semantic_treatment_recovered"] = true
	return PendingInteraction{
		ID: interactionID, Kind: "semantic_treatment_question", Type: "semantic_treatment_selection",
		Source: "semantic_treatment", Workflow: semanticTreatmentWorkflow, Stage: "awaiting_selection",
		ConversationID: firstStringFromMap(requestContext, "conversation_id"), GoalID: firstStringFromMap(requestContext, "goal_id"),
		RunID: firstStringFromMap(requestContext, "run_id"), RequestContext: requestContext, Payload: payload, Data: payload,
	}, true
}

func semanticTreatmentTarget(payload map[string]any) (string, string) {
	target := firstMapFromAny(payload["target_scope"])
	trackRefs := stringListValue(target["track_refs"])
	trackID := ""
	if len(trackRefs) > 0 {
		trackID = trackRefs[0]
	}
	return trackID, firstStringFromMap(target, "label")
}

func (s *Server) continueSemanticTreatmentInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	if strings.EqualFold(decision, "cancel") || strings.EqualFold(decision, "cancel_semantic_treatment") {
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
			Reply: "已取消这次处理策略选择；没有加载插件，也没有修改任何参数。", Workflow: semanticTreatmentWorkflow,
			WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "cancelled", "selection_performed": false, "mutation_performed": false}),
			GoalStatus:   string(agentruntime.StatusCancelled)}
	}
	choiceKey := strings.TrimPrefix(strings.TrimSpace(decision), "select_")
	var selected map[string]any
	for _, row := range mapRowsValue(interaction.Payload["choices"]) {
		if firstStringFromMap(row, "choice_key") == choiceKey {
			selected = row
			break
		}
	}
	if len(selected) == 0 {
		return semanticTreatmentInvalidSelectionResponse(interaction, "没有识别到有效的处理策略；工程没有被修改。", "invalid_treatment_choice")
	}
	trackID, trackName := semanticTreatmentTarget(interaction.Payload)
	if trackID == "" || s == nil || s.harness == nil {
		return semanticTreatmentInvalidSelectionResponse(interaction, "处理策略缺少当前精确目标轨道；工程没有被修改。", "treatment_target_unavailable")
	}
	instances, currentToken := s.semanticTreatmentInstances(ctx, trackID)
	if expected := firstStringFromMap(interaction.Payload, "state_token"); expected == "" || currentToken == "" || expected != currentToken {
		return semanticTreatmentInvalidSelectionResponse(interaction, "选择期间工程或插件实例状态已经改变，旧策略已失效；请重新发起处理请求。", "stale_treatment_strategy")
	}
	requestContext := mergeContext(interaction.RequestContext, map[string]any{
		"selected_track_id": trackID, "selected_track_name": trackName, "conversation_id": interaction.ConversationID,
		"goal_id": interaction.GoalID, "run_id": interaction.RunID,
		"semantic_treatment_strategy": map[string]any{
			"schema_version": semanticTreatmentSchema, "summary": firstStringFromMap(interaction.Payload, "summary"),
			"global_constraints": stringListValue(interaction.Payload["global_constraints"]), "selected_choice": cloneContext(selected),
		},
	})
	goal := firstStringFromMap(interaction.Payload, "listening_goal")
	processorType := canonicalPluginRecommendationProcessorType(firstStringFromMap(selected, "processor_type"))
	switch firstStringFromMap(selected, "target_mode") {
	case "existing_plugin":
		bound := firstMapFromAny(selected["bound_instance"])
		pluginID := firstStringFromMap(bound, "plugin_id")
		instance, ok := semanticTreatmentFindPlugin(instances, pluginID)
		if !ok || instance.Key == "" || instance.QualificationStatus != "generic_static_eq_qualified" || processorType != "eq" || firstStringFromMap(selected, "next_planner") != "semantic_eq" {
			return semanticTreatmentCapabilityBoundaryResponse(interaction, selected, "所选已加载实例没有通过普通 Agent 的通用静态 EQ 资格确认，因此没有生成参数动作。")
		}
		requestContext["selected_plugin_track_id"] = instance.TrackID
		requestContext["selected_plugin_id"] = instance.PluginID
		requestContext["selected_plugin_name"] = instance.PluginName
		requestContext["generic_eq_topology"] = instance.Topology
		return s.semanticTreatmentPlanEQResponse(ctx, interaction.ConversationID, goal, requestContext,
			semanticTreatmentObservationFromPayload(interaction.Payload), interaction.GoalID, interaction.RunID)
	case "load_required":
		requestContext["semantic_treatment_selection"] = true
		if processorType == "eq" {
			requestContext["semantic_eq_post_load_handoff"] = true
			requestContext["semantic_eq_post_load_goal"] = goal
		}
		cfg, _, err := config.Load()
		if err != nil || !cfg.Complete() {
			if err == nil {
				err = fmt.Errorf("AI configuration is incomplete")
			}
			return semanticTreatmentInvalidSelectionResponse(interaction, "插件推荐阶段当前无法读取完整 AI 配置；没有加载插件或修改工程。", err.Error())
		}
		return s.ordinaryAgentPluginRecommendationResponseForProcessor(ctx, interaction.ConversationID,
			agentModeFromContext(requestContext), goal, requestContext, agentloop.Result{GoalID: interaction.GoalID, RunID: interaction.RunID,
				Status: agentruntime.StatusCompleted, RecentObservation: semanticTreatmentObservationFromPayload(interaction.Payload)}, processorType, cfg)
	default:
		return semanticTreatmentCapabilityBoundaryResponse(interaction, selected, "这个处理方法目前没有普通 Agent 的可治理参数 planner；策略已保留，但没有加载插件或修改工程。")
	}
}

func semanticTreatmentInvalidSelectionResponse(interaction PendingInteraction, reply, code string) ChatResponse {
	return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
		Reply: reply, Workflow: semanticTreatmentWorkflow,
		WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "stale_or_invalid", "mutation_performed": false}),
		GoalStatus:   string(agentruntime.StatusWaitingClarification), Error: code}
}

func semanticTreatmentCapabilityBoundaryResponse(interaction PendingInteraction, selected map[string]any, reason string) ChatResponse {
	return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
		Reply: reason, Workflow: semanticTreatmentWorkflow,
		WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "capability_boundary", "selected_choice": selected,
			"selection_performed": true, "mutation_performed": false}),
		GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_treatment_capability_boundary"}
}

func (s *Server) semanticTreatmentPlanEQResponse(ctx context.Context, conversationID, userText string, requestContext map[string]any,
	observation *agentloop.RecentObservation, goalID, runID string) ChatResponse {
	cfg, _, err := config.Load()
	if err != nil {
		return semanticEQRejectedResponse(conversationID, agentruntime.Goal{GoalID: goalID, RunID: runID, Summary: userText, Status: agentruntime.StatusFailed}, "config_unavailable", err.Error(), nil)
	}
	action, err := s.ensureOrdinaryAgentSemanticEQExecutable(ctx, conversationID, userText, requestContext, observation, cfg, nil)
	if err != nil {
		return semanticEQRejectedResponse(conversationID, agentruntime.Goal{GoalID: goalID, RunID: runID, Summary: userText, Status: agentruntime.StatusFailed}, "acoustic_plan_failed", err.Error(), nil)
	}
	res := agentloop.Result{GoalID: goalID, RunID: runID, Status: agentruntime.StatusCompleted, RecentObservation: observation, SemanticAction: action}
	return s.materializeAgentSemanticEQAction(ctx, conversationID, ChatRequest{ConversationID: conversationID, Message: userText, Context: requestContext}, agentModeFromContext(requestContext), res)
}

func (s *Server) routeOrdinaryAgentSemanticEQ(ctx context.Context, conversationID, mode, userText string,
	requestContext map[string]any, res agentloop.Result, cfg config.EngineConfig) (ChatResponse, bool) {
	if !ordinaryAgentSemanticEQMutationRequest(userText, requestContext) {
		return ChatResponse{}, false
	}
	trackID := firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id")
	trackName := firstStringFromMap(requestContext, "selected_track_name")
	selectedPluginID := firstStringFromMap(requestContext, "selected_plugin_id")
	if selectedPluginID != "" && len(firstMapFromAny(requestContext["generic_eq_topology"])) > 0 {
		planned, err := s.ensureOrdinaryAgentSemanticEQExecutable(ctx, conversationID, userText, requestContext, res.RecentObservation, cfg, res.SemanticAction)
		if err != nil {
			goal := agentruntime.Goal{GoalID: res.GoalID, RunID: res.RunID, Summary: userText, Status: agentruntime.StatusFailed}
			return semanticEQRejectedResponse(conversationID, goal, "acoustic_plan_failed", err.Error(), nil), true
		}
		res.SemanticAction = planned
		return s.materializeAgentSemanticEQAction(ctx, conversationID,
			ChatRequest{ConversationID: conversationID, Message: userText, Context: requestContext}, mode, res), true
	}
	instances, stateToken := s.semanticTreatmentInstances(ctx, trackID)
	qualified := semanticTreatmentQualifiedEQ(instances)
	if selectedPluginID != "" {
		if instance, ok := semanticTreatmentFindPlugin(qualified, selectedPluginID); ok {
			bound := mergeContext(requestContext, map[string]any{
				"selected_plugin_track_id": instance.TrackID, "selected_plugin_id": instance.PluginID,
				"selected_plugin_name": instance.PluginName, "generic_eq_topology": instance.Topology,
			})
			planned, err := s.ensureOrdinaryAgentSemanticEQExecutable(ctx, conversationID, userText, bound, res.RecentObservation, cfg, res.SemanticAction)
			if err != nil {
				goal := agentruntime.Goal{GoalID: res.GoalID, RunID: res.RunID, Summary: userText, Status: agentruntime.StatusFailed}
				return semanticEQRejectedResponse(conversationID, goal, "acoustic_plan_failed", err.Error(), nil), true
			}
			res.SemanticAction = planned
			return s.materializeAgentSemanticEQAction(ctx, conversationID,
				ChatRequest{ConversationID: conversationID, Message: userText, Context: bound}, mode, res), true
		}
		// A selected but ineligible processor must never be silently replaced.
		// If another real EQ exists, present that explicit instance decision.
		if len(qualified) > 0 {
			plan, err := s.planSemanticTreatment(ctx, conversationID, userText, trackID, trackName, res.RecentObservation, qualified, true, false, cfg)
			if err != nil {
				return semanticTreatmentPlannerErrorResponse(conversationID, res, err), true
			}
			plan.Limitations = append(plan.Limitations, "当前选中的插件没有通过通用静态 EQ 资格确认，因此不会在未征得选择的情况下改用其他实例。")
			return s.semanticTreatmentSelectionResponse(conversationID, mode, trackID, trackName, stateToken, requestContext, res, plan, qualified), true
		}
	}
	if selectedPluginID == "" {
		switch len(qualified) {
		case 1:
			instance := qualified[0]
			bound := mergeContext(requestContext, map[string]any{
				"selected_plugin_track_id": instance.TrackID, "selected_plugin_id": instance.PluginID,
				"selected_plugin_name": instance.PluginName, "generic_eq_topology": instance.Topology,
			})
			planned, err := s.ensureOrdinaryAgentSemanticEQExecutable(ctx, conversationID, userText, bound, res.RecentObservation, cfg, nil)
			if err != nil {
				goal := agentruntime.Goal{GoalID: res.GoalID, RunID: res.RunID, Summary: userText, Status: agentruntime.StatusFailed}
				return semanticEQRejectedResponse(conversationID, goal, "acoustic_plan_failed", err.Error(), nil), true
			}
			res.SemanticAction = planned
			return s.materializeAgentSemanticEQAction(ctx, conversationID,
				ChatRequest{ConversationID: conversationID, Message: userText, Context: bound}, mode, res), true
		default:
			if len(qualified) > 1 {
				plan, err := s.planSemanticTreatment(ctx, conversationID, userText, trackID, trackName, res.RecentObservation, qualified, true, false, cfg)
				if err != nil {
					return semanticTreatmentPlannerErrorResponse(conversationID, res, err), true
				}
				return s.semanticTreatmentSelectionResponse(conversationID, mode, trackID, trackName, stateToken, requestContext, res, plan, qualified), true
			}
		}
	}
	// No loaded instance is qualified. Enter target 3 with an explicit
	// post-load EQ handoff marker; recommendation and loading remain separate.
	handoff := mergeContext(requestContext, map[string]any{
		"semantic_eq_post_load_handoff": true, "semantic_eq_post_load_goal": strings.TrimSpace(userText),
	})
	return s.ordinaryAgentPluginRecommendationResponseForProcessor(ctx, conversationID, mode, userText, handoff, res, "eq", cfg), true
}

func (s *Server) routeOrdinaryAgentTreatmentStrategy(ctx context.Context, conversationID, mode, userText string,
	requestContext map[string]any, res agentloop.Result, cfg config.EngineConfig) (ChatResponse, bool) {
	if !ordinaryAgentTreatmentStrategyIntent(userText, requestContext) {
		return ChatResponse{}, false
	}
	trackID := firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id")
	trackName := firstStringFromMap(requestContext, "selected_track_name")
	instances, stateToken := s.semanticTreatmentInstances(ctx, trackID)
	nativeHandoff := semanticTreatmentNativeHandoffAvailable(res)
	plan, err := s.planSemanticTreatment(ctx, conversationID, userText, trackID, trackName, res.RecentObservation, instances, false, nativeHandoff, cfg)
	if err != nil {
		return semanticTreatmentPlannerErrorResponse(conversationID, res, err), true
	}
	if plan.DecisionMode == "choice_required" {
		return s.semanticTreatmentSelectionResponse(conversationID, mode, trackID, trackName, stateToken, requestContext, res, plan, instances), true
	}
	if len(plan.Choices) != 1 {
		return semanticTreatmentPlannerErrorResponse(conversationID, res, fmt.Errorf("direct strategy omitted its single choice")), true
	}
	choice := plan.Choices[0]
	requestContext = mergeContext(requestContext, map[string]any{
		"semantic_treatment_strategy": map[string]any{"schema_version": semanticTreatmentSchema, "summary": plan.Summary,
			"global_constraints": plan.GlobalConstraints, "selected_choice": semanticTreatmentChoiceRow(choice, semanticTreatmentInstance{})},
	})
	switch choice.TargetMode {
	case "existing_plugin":
		instance, ok := semanticTreatmentFindInstance(instances, choice.InstanceKey)
		if !ok || instance.QualificationStatus != "generic_static_eq_qualified" || choice.ProcessorType != "eq" || choice.NextPlanner != "semantic_eq" {
			return semanticTreatmentDirectBoundaryResponse(conversationID, res, plan, choice,
				"主推荐实例目前没有可治理的普通 Agent 参数 planner；没有修改工程。"), true
		}
		bound := mergeContext(requestContext, map[string]any{
			"selected_plugin_track_id": instance.TrackID, "selected_plugin_id": instance.PluginID,
			"selected_plugin_name": instance.PluginName, "generic_eq_topology": instance.Topology,
		})
		planned, err := s.ensureOrdinaryAgentSemanticEQExecutable(ctx, conversationID, userText, bound, res.RecentObservation, cfg, nil)
		if err != nil {
			goal := agentruntime.Goal{GoalID: res.GoalID, RunID: res.RunID, Summary: userText, Status: agentruntime.StatusFailed}
			return semanticEQRejectedResponse(conversationID, goal, "acoustic_plan_failed", err.Error(), nil), true
		}
		res.SemanticAction = planned
		return s.materializeAgentSemanticEQAction(ctx, conversationID,
			ChatRequest{ConversationID: conversationID, Message: userText, Context: bound}, mode, res), true
	case "load_required":
		if choice.ProcessorType == "eq" {
			requestContext["semantic_eq_post_load_handoff"] = true
			requestContext["semantic_eq_post_load_goal"] = strings.TrimSpace(userText)
		}
		return s.ordinaryAgentPluginRecommendationResponseForProcessor(ctx, conversationID, mode, userText, requestContext, res, choice.ProcessorType, cfg), true
	case "native":
		if nativeHandoff && choice.NextPlanner == "existing_agent_result" {
			return s.chatResponseFromAgentLoopResult(conversationID, mode, res), true
		}
		return semanticTreatmentDirectBoundaryResponse(conversationID, res, plan, choice,
			"原生控制策略已经失去可继续的 Agent 结果；没有修改工程，请重新发起请求。"), true
	default:
		return semanticTreatmentDirectBoundaryResponse(conversationID, res, plan, choice,
			"主推荐方法目前没有可治理的普通 Agent 参数 planner；策略可以讨论，但没有加载插件或修改工程。"), true
	}
}

func semanticTreatmentNativeHandoffAvailable(res agentloop.Result) bool {
	if res.ExecutionMemory.PendingMixTreatment != nil || res.ExecutionMemory.PendingMixTickCandidate != nil {
		return true
	}
	return res.Status == agentruntime.StatusWaitingConfirmation && res.Continuation != nil
}

func semanticTreatmentPlannerErrorResponse(conversationID string, res agentloop.Result, err error) ChatResponse {
	return ChatResponse{ConversationID: conversationID, GoalID: res.GoalID, RunID: res.RunID,
		Reply:    "处理策略暂时无法形成可靠且可验证的结论，因此没有选择插件或修改工程。",
		Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
			"status": "unavailable", "mutation_performed": false}, GoalStatus: string(agentruntime.StatusFailed), Error: err.Error()}
}

func semanticTreatmentDirectBoundaryResponse(conversationID string, res agentloop.Result, plan semanticTreatmentPlan,
	choice semanticTreatmentChoice, reason string) ChatResponse {
	return ChatResponse{ConversationID: conversationID, GoalID: res.GoalID, RunID: res.RunID, Reply: reason,
		Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
			"status": "capability_boundary", "summary": plan.Summary, "selected_choice": semanticTreatmentChoiceRow(choice, semanticTreatmentInstance{}),
			"selection_performed": true, "mutation_performed": false}, GoalStatus: string(agentruntime.StatusCompleted),
		StopReason: "semantic_treatment_capability_boundary"}
}

// After a user confirms loading an EQ chosen by target 3, qualify the actual
// returned instance and create a separate governed EQ proposal. The load
// authorization never grants parameter-write authority.
func (s *Server) semanticEQPostLoadHandoff(ctx context.Context, plan PendingPlan, replies []map[string]any) (ChatResponse, bool) {
	if !boolValue(plan.WorkflowData["semantic_eq_post_load_handoff"]) {
		return ChatResponse{}, false
	}
	trackID, pluginID, pluginName := pluginLoadResultIDs(replies)
	if trackID == "" {
		trackID = firstStringFromMap(plan.WorkflowData, "track_id")
	}
	goalID, runID := goalIDsFromContext(plan.Context)
	conversationID := firstStringFromMap(plan.Context, "conversation_id")
	userGoal := firstNonEmpty(firstStringFromMap(plan.WorkflowData, "semantic_eq_post_load_goal"), firstStringFromMap(plan.WorkflowData, "intent"))
	if pluginID == "" {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    "EQ 已加载，但加载结果没有返回精确 plugin_id，无法进行资格确认或生成参数方案；没有写入任何 EQ 参数。",
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": "eq", "track_id": trackID, "mutation_performed": false},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_eq_post_load_identity_missing"}, true
	}
	_, summary, err := s.readLiveEQControlSurface(ctx, trackID, pluginID)
	if err != nil || len(summary) == 0 {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply: fmt.Sprintf("已加载 %s，但该实际实例没有通过普通 Agent 通用静态 EQ 资格确认，因此没有生成或写入参数方案。限制：%s",
				firstNonEmpty(pluginName, pluginID), firstNonEmpty(errorText(err), "没有可证明的通用静态 EQ topology")),
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": "eq", "track_id": trackID, "plugin_id": pluginID,
				"plugin_name": pluginName, "mutation_performed": false, "limitation": errorText(err)},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_eq_post_load_not_qualified"}, true
	}
	requestContext := cloneContext(firstMapFromAny(plan.WorkflowData["semantic_eq_post_load_request_context"]))
	if requestContext == nil {
		requestContext = cloneContext(plan.Context)
	}
	requestContext = mergeContext(requestContext, map[string]any{
		"selected_track_id": trackID, "selected_plugin_track_id": trackID, "selected_plugin_id": pluginID,
		"selected_plugin_name": firstNonEmpty(pluginName, firstStringFromMap(plan.WorkflowData, "plugin_name")),
		"generic_eq_topology":  semanticEQTopologyPromptSummary(trackID, pluginID, summary),
		"conversation_id":      conversationID, "goal_id": goalID, "run_id": runID,
	})
	observation := semanticTreatmentObservationFromPayload(map[string]any{
		"observation_context": firstMapFromAny(plan.WorkflowData["semantic_eq_post_load_observation_context"]),
	})
	resp := s.semanticTreatmentPlanEQResponse(ctx, conversationID, userGoal, requestContext, observation, goalID, runID)
	if resp.WorkflowData == nil {
		resp.WorkflowData = map[string]any{}
	}
	resp.WorkflowData["post_load_qualification"] = map[string]any{
		"status": "qualified", "processor_type": "eq", "track_id": trackID, "plugin_id": pluginID,
		"plugin_name":         firstNonEmpty(pluginName, firstStringFromMap(plan.WorkflowData, "plugin_name")),
		"topology_generation": eqTopologyGenerationFromSummary(summary),
	}
	resp.Reply = fmt.Sprintf("已加载并确认 %s 具备可执行的通用静态 EQ topology。加载授权已经结束，尚未写入 EQ 参数。\n\n%s",
		firstNonEmpty(pluginName, pluginID), resp.Reply)
	return resp, true
}

func applySemanticEQPostLoadHandoffResponse(base map[string]any, handoff ChatResponse) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	base["message"] = handoff.Reply
	base["reply"] = handoff.Reply
	base["needs_confirmation"] = handoff.NeedsConfirmation
	base["plan_id"] = handoff.PlanID
	base["preview"] = handoff.Preview
	base["workflow"] = handoff.Workflow
	base["workflow_data"] = handoff.WorkflowData
	base["goal_status"] = handoff.GoalStatus
	base["stop_reason"] = handoff.StopReason
	base["error"] = handoff.Error
	if handoff.ProposalPresentation != nil {
		base["proposal_presentation"] = handoff.ProposalPresentation
	}
	if len(handoff.InteractionRequests) > 0 {
		base["interaction_requests"] = handoff.InteractionRequests
	}
	if len(handoff.TypedEvents) > 0 {
		base["typed_events"] = append(mapRowsFromAny(base["typed_events"]), handoff.TypedEvents...)
	}
	base["message_kind"] = firstNonEmpty(handoff.MessageKind, chatResponseMessageKind(handoff))
	return base
}
