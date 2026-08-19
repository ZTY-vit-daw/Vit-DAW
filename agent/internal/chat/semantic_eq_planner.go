package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/semanticeffect"
)

// planOrdinaryAgentSemanticEQ is the dedicated acoustic-planning phase after
// the ordinary Agent has made its observation decision. The LLM owns every
// musical field; deterministic inputs only describe exact target identity,
// available evidence, and the generic topology constraint surface.
func (s *Server) planOrdinaryAgentSemanticEQ(ctx context.Context, conversationID, userText string, requestContext map[string]any, observation *agentloop.RecentObservation, cfg config.EngineConfig) (*semanticeffect.Action, error) {
	return s.planOrdinaryAgentSemanticEQWithFeedback(ctx, conversationID, userText, requestContext, observation, cfg, "")
}

func (s *Server) planOrdinaryAgentSemanticEQWithFeedback(ctx context.Context, conversationID, userText string, requestContext map[string]any, observation *agentloop.RecentObservation, cfg config.EngineConfig, deterministicRejection string) (*semanticeffect.Action, error) {
	if s == nil || s.llm == nil {
		return nil, fmt.Errorf("semantic EQ LLM planner is unavailable")
	}
	trackID := firstStringFromMap(requestContext, "selected_plugin_track_id", "selected_track_id")
	pluginID := firstStringFromMap(requestContext, "selected_plugin_id")
	if trackID == "" || pluginID == "" {
		return nil, fmt.Errorf("semantic EQ requires an exact selected track/plugin target")
	}
	topology := firstMapFromAny(requestContext["generic_eq_topology"])
	if len(topology) == 0 {
		return nil, fmt.Errorf("semantic EQ acoustic planning requires generic_eq_topology")
	}
	observationID, observationIssue := semanticEQAbstractObservationIssue(observation)
	if observationIssue != "" {
		return nil, fmt.Errorf("semantic EQ acoustic planning requires successful model-requested CCB evidence: %s", observationIssue)
	}
	input := map[string]any{
		"user_request": userText,
		"exact_target": map[string]any{
			"track_id": trackID, "plugin_id": pluginID,
			"track_name":  firstStringFromMap(requestContext, "selected_track_name"),
			"plugin_name": firstStringFromMap(requestContext, "selected_plugin_name"),
		},
		"generic_eq_topology": topology,
		"observation_context": semanticEQPlannerObservation(observation),
	}
	if strategy := firstMapFromAny(requestContext["semantic_treatment_strategy"]); len(strategy) > 0 {
		input["selected_treatment_strategy"] = strategy
	}
	if rejection := strings.TrimSpace(deterministicRejection); rejection != "" {
		input["deterministic_materialization_rejection"] = map[string]any{
			"status": "rejected", "reason": rejection,
			"required_response": "choose a different acoustic plan that a concrete section proves executable",
		}
	}
	inputJSON, _ := json.Marshal(input)
	system := `You are the acoustic-planning phase of an ordinary DAW Agent.
This is a horizontal generic static-EQ interaction, not B4 or any A-F specialist capability.

The user request, exact selected target, optional observation evidence, and deterministic generic EQ topology are supplied as JSON. You make the musical judgement. Deterministic code will validate, freeze, confirm, execute, read back, roll back, and verify it.

Return ONLY one JSON object matching semantic_effect_action.v1. Do not return prose, Markdown, a question, tool calls, a pending treatment, a profile, a dynamic EQ plan, or a plug-in-specific rule.

The exact outer object and one complete atom are:
` + semanticeffect.StaticEQActionPromptExample + `
Use the exact key evidence_decision, not evidence. Its basis must be observation or both and it must cite the exact supplied observation_id.

Shared generic static-EQ atom contract:
` + semanticeffect.StaticEQAtomPromptRules + `

Required rules:
- action_type="eq_edit", payload_schema="semantic_effect.eq_plan.v1".
- Copy the exact supplied track_id/plugin_id. Never invent target identities.
- eq_plan.schema_version="semantic_effect.eq_plan.v1", atomic=true, with 1-3 upsert atoms. Every atom must explicitly include "action":"upsert".
- Choose each atom's Shape/Frequency/Gain/Q/Slope from your acoustic judgement, constrained only by generic_eq_topology. Use only bell, low_shelf, high_shelf, low_cut, high_cut. A shape is usable only when at least one concrete section reports shape_capabilities.actions.upsert=true and every explicitly requested field is writable; supported_filter_kinds alone is informational and does not prove executability.
- Do not use a phrase-to-parameter lookup table. Infer conservative concrete values from the listening goal and evidence.
- Bell/shelf atoms require frequency_hz and gain_db. Cut atoms require frequency_hz and forbid gain_db. Include Q or slope only when musically useful and topology says that field is writable.
- Every supplied numeric acoustic field needs field_origins[field] = user_fixed, llm_selected, or context_inherited.
- Every atom needs a stable atom_id, distinct acoustic purpose, and low/medium/high confidence.
- Preserve negative listening constraints in negative_constraints. If the request combines a positive goal with a negative constraint and one atom cannot express both, use 2-3 coordinated static-EQ atoms with distinct purposes.
- When selected_treatment_strategy is supplied, treat its global_constraints and selected choice as binding context for the concrete EQ plan; do not replace it with another treatment method.
- The supplied CCB observation is mandatory evidence for this abstract acoustic plan. Use evidence_decision basis observation or both, copy its exact observation_id, and cite supplied evidence_refs when useful. choice=not_needed and basis=user_report are forbidden here.
- Do not ask the user to choose shelf vs bell or approve a dynamic band. You are responsible for that acoustic decision; confirmation happens after the typed plan is frozen.`
	if strings.TrimSpace(deterministicRejection) != "" {
		system += `
- A previous candidate was rejected by deterministic materialization. Treat that rejection as a hard constraint: choose a materially different reachable Shape/field combination. Do not repeat the rejected combination and do not remove the user's listening goal or negative constraints.`
	}
	request := llm.Request{
		Messages:   []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(inputJSON)}},
		Metadata:   llm.RequestMetadata{Source: "ordinary_agent_semantic_eq_planner", ConversationID: conversationID},
		PreferJSON: true,
	}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return nil, err
	}
	action, decodeErr := decodeSemanticEQLLMActionForTarget(response.Text, trackID, pluginID, userText)
	if decodeErr == nil {
		decodeErr = validateSemanticEQAbstractEvidence(action, observationID)
	}
	if decodeErr == nil {
		return &action, nil
	}
	repair := fmt.Sprintf(`Your previous semantic EQ JSON was invalid: %s
Return ONLY a corrected semantic_effect_action.v1 JSON object for the same input and exact target.
The corrected atoms must obey this shared contract: %s`, decodeErr, semanticeffect.StaticEQAtomPromptRules)
	request.Messages = append(request.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: repair})
	response, err = s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return nil, err
	}
	action, err = decodeSemanticEQLLMActionForTarget(response.Text, trackID, pluginID, userText)
	if err != nil {
		return nil, fmt.Errorf("semantic EQ planner returned invalid action after repair: %w", err)
	}
	if err := validateSemanticEQAbstractEvidence(action, observationID); err != nil {
		return nil, fmt.Errorf("semantic EQ planner returned invalid action after repair: %w", err)
	}
	return &action, nil
}

func (s *Server) ensureOrdinaryAgentSemanticEQExecutable(ctx context.Context, conversationID, userText string, requestContext map[string]any, observation *agentloop.RecentObservation, cfg config.EngineConfig, candidate *semanticeffect.Action) (*semanticeffect.Action, error) {
	observationID, observationIssue := semanticEQAbstractObservationIssue(observation)
	if observationIssue != "" {
		return nil, fmt.Errorf("semantic EQ acoustic planning requires successful model-requested CCB evidence: %s", observationIssue)
	}
	if candidate == nil {
		planned, err := s.planOrdinaryAgentSemanticEQ(ctx, conversationID, userText, requestContext, observation, cfg)
		if err != nil {
			return nil, err
		}
		candidate = planned
	}
	if err := validateSemanticEQAbstractEvidence(*candidate, observationID); err != nil {
		return nil, err
	}
	if _, err := s.planSemanticEQReadOnly(ctx, *candidate); err == nil {
		return candidate, nil
	} else {
		repaired, repairErr := s.planOrdinaryAgentSemanticEQWithFeedback(ctx, conversationID, userText, requestContext, observation, cfg, err.Error())
		if repairErr != nil {
			return nil, fmt.Errorf("semantic EQ topology repair failed after %v: %w", err, repairErr)
		}
		if _, repairedErr := s.planSemanticEQReadOnly(ctx, *repaired); repairedErr != nil {
			return nil, fmt.Errorf("semantic EQ topology repair remained unexecutable after %v: %w", err, repairedErr)
		}
		return repaired, nil
	}
}

func semanticEQPlannerObservation(observation *agentloop.RecentObservation) map[string]any {
	if observation == nil {
		return map[string]any{"status": "not_observed"}
	}
	out := map[string]any{
		"tool": observation.Tool, "status": observation.Status,
		"tool_call_id": observation.ToolCallID, "summary": cloneContext(observation.Summary),
	}
	// Keep the dedicated planning prompt bounded even when an observer returns
	// an unexpectedly broad digest. The retained identity and references remain
	// mandatory; oversized evidence never falls back to the user report alone.
	if data, _ := json.Marshal(out); len(data) > 32000 {
		texts := semanticEQRecursiveText(observation.Summary)
		out = map[string]any{
			"tool": observation.Tool, "status": observation.Status, "tool_call_id": observation.ToolCallID,
			"observation_id": semanticEQFirstRecursive(texts, "observation_id"),
			"scope":          semanticEQFirstRecursive(texts, "scope"),
			"limitations":    texts["limitations"], "evidence_refs": texts["evidence_refs"],
			"admission_note": "observation digest exceeded semantic planner budget; retain the exact observation binding and plan only within admitted evidence",
		}
	}
	return out
}

func semanticEQAbstractObservationIssue(observation *agentloop.RecentObservation) (string, string) {
	if observation == nil {
		return "", "observation is missing"
	}
	tool := strings.ToLower(strings.TrimSpace(firstNonEmpty(observation.Tool, observation.CommandName)))
	if tool != "ccb.observation_request" && tool != "ccb_observation_request" {
		return "", "observation was not produced by ccb.observation_request"
	}
	if strings.TrimSpace(observation.Error) != "" {
		return "", "observation execution reported an error"
	}
	switch strings.ToLower(strings.TrimSpace(observation.Status)) {
	case "ok", "ready", "partial":
	default:
		return "", "observation execution did not succeed"
	}
	bundle := observation.Summary
	if len(bundle) == 0 || bundle["read_only"] != true || bundle["mutation_authority"] != false {
		return "", "observation bundle does not prove its read-only boundary"
	}
	switch strings.ToLower(firstStringFromMap(bundle, "status")) {
	case "ready", "partial":
	default:
		return "", "observation bundle is not ready or partial"
	}
	if len(firstMapFromAny(bundle["views"])) == 0 {
		return "", "observation bundle contains no usable views"
	}
	texts := semanticEQRecursiveText(bundle)
	observationID := strings.TrimSpace(semanticEQFirstRecursive(texts, "observation_id"))
	if observationID == "" {
		return "", "observation bundle has no observation_id"
	}
	return observationID, ""
}

func validateSemanticEQAbstractEvidence(action semanticeffect.Action, observationID string) error {
	basis := strings.ToLower(strings.TrimSpace(action.Evidence.Basis))
	if basis != "observation" && basis != "both" {
		return fmt.Errorf("abstract semantic EQ evidence_decision.basis must be observation or both")
	}
	if strings.EqualFold(strings.TrimSpace(action.Evidence.Choice), "not_needed") {
		return fmt.Errorf("abstract semantic EQ evidence_decision.choice cannot be not_needed")
	}
	if strings.TrimSpace(action.Evidence.ObservationID) != strings.TrimSpace(observationID) {
		return fmt.Errorf("abstract semantic EQ evidence_decision must cite the exact CCB observation_id")
	}
	return nil
}

func decodeSemanticEQLLMAction(text string) (semanticeffect.Action, error) {
	action, err := parseSemanticEQLLMAction(text)
	if err != nil {
		return action, err
	}
	if err := action.Validate(); err != nil {
		return action, err
	}
	return action, nil
}

func parseSemanticEQLLMAction(text string) (semanticeffect.Action, error) {
	var action semanticeffect.Action
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return action, fmt.Errorf("response does not contain a JSON object")
	}
	raw := []byte(text[start : end+1])
	actionRaw := raw
	if err := json.Unmarshal(actionRaw, &action); err != nil {
		return action, fmt.Errorf("decode action JSON: %w", err)
	}
	if strings.TrimSpace(action.SchemaVersion) == "" {
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper["semantic_action"]) > 0 {
			actionRaw = wrapper["semantic_action"]
			if err := json.Unmarshal(actionRaw, &action); err != nil {
				return action, fmt.Errorf("decode semantic_action wrapper: %w", err)
			}
		}
	}
	if strings.TrimSpace(action.Evidence.Choice) == "" {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(actionRaw, &fields); err == nil {
			for _, key := range []string{"evidence", "evidenceDecision"} {
				if len(fields[key]) == 0 {
					continue
				}
				if err := json.Unmarshal(fields[key], &action.Evidence); err != nil {
					return action, fmt.Errorf("decode %s alias: %w", key, err)
				}
				break
			}
		}
	}
	return action, nil
}

// Exact target identity is deterministic request context, not an acoustic
// judgement. A model omission is therefore completed here, while any
// non-empty identity that disagrees with the selected target is rejected.
func decodeSemanticEQLLMActionForTarget(text, trackID, pluginID string, userGoal ...string) (semanticeffect.Action, error) {
	action, err := parseSemanticEQLLMAction(text)
	if err != nil {
		return action, err
	}
	if action.Target.TrackID != "" && action.Target.TrackID != trackID {
		return action, fmt.Errorf("semantic EQ planner changed exact target track_id")
	}
	if action.Target.PluginID != "" && action.Target.PluginID != pluginID {
		return action, fmt.Errorf("semantic EQ planner changed exact target plugin_id")
	}
	if action.Target.TrackID == "" {
		action.Target.TrackID = trackID
	}
	if action.Target.PluginID == "" {
		action.Target.PluginID = pluginID
	}
	if strings.TrimSpace(action.UserGoal) == "" && len(userGoal) > 0 {
		action.UserGoal = strings.TrimSpace(userGoal[0])
	}
	// This planner contract permits only new static-EQ upserts. Completing an
	// omitted operational verb is deterministic schema binding, analogous to
	// binding the exact target above; Shape/Frequency/Gain/Q remain entirely
	// model-selected and every other malformed atom is still rejected.
	if action.EQPlan != nil {
		for index := range action.EQPlan.Atoms {
			atom := &action.EQPlan.Atoms[index]
			if strings.TrimSpace(atom.Action) == "" &&
				strings.TrimSpace(atom.Shape) != "" && atom.FrequencyHz != nil &&
				strings.TrimSpace(atom.ControlRef) == "" && strings.TrimSpace(atom.OperationRef) == "" {
				atom.Action = "upsert"
			}
		}
	}
	if err := action.Validate(); err != nil {
		return action, err
	}
	return action, nil
}

func ordinaryAgentSemanticEQActionable(userText string, requestContext map[string]any) bool {
	if !agentLoopNeedsGenericEQTopology(userText, requestContext) {
		return false
	}
	return ordinaryAgentSemanticEQMutationIntent(userText, false)
}

func ordinaryAgentSemanticEQNeedsPluginSelection(userText string, requestContext map[string]any) bool {
	if contextHasAnyValue(requestContext, "selected_plugin_id", "primary_selected_plugin_id") ||
		!contextHasAnyValue(requestContext, "selected_track_id", "selected_plugin_track_id") ||
		!ordinaryAgentSemanticEQTextTopic(userText) {
		return false
	}
	return ordinaryAgentSemanticEQMutationIntent(userText, true)
}

func ordinaryAgentSemanticEQTextTopic(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	return agentLoopTextHasAny(text,
		"浑浊", "浑", "闷", "刺耳", "尖锐", "更亮", "更暗", "高频", "低频", "中频", "低中频", "空气感", "清晰", "均衡", "频段", "赫兹",
		"mud", "muddy", "boxy", "harsh", "bright", "dark", "treble", "bass", "mid", "air", "clarity", "presence", "eq", "equalizer", "hz", "shelf", "bell", "cut",
	)
}

func ordinaryAgentSemanticEQMutationIntent(userText string, requireExplicitAction bool) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	discussion := agentLoopTextHasAny(text, "为什么", "為什麼", "什么原因", "什麼原因", "怎么判断", "怎麼判斷", "分析一下", "解释一下", "解釋一下", "why", "what causes", "how do you know")
	action := agentLoopTextHasAny(text, "减少", "減少", "降低", "收一点", "收一些", "削", "切", "提高", "提升", "增加", "更亮", "更暗", "处理", "處理", "调整", "調整", "修改", "执行", "執行", "apply", "reduce", "cut", "boost", "raise", "lower", "make it", "adjust", "change", "execute")
	if requireExplicitAction {
		return action
	}
	return !discussion || action
}
