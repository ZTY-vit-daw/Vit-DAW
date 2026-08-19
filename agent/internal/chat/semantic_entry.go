package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/orchestrationcontroller"
)

const (
	semanticEntryDecisionSchema = "semantic_entry_decision.v1"

	semanticEntryRouteDiscussion      = "discussion"
	semanticEntryRouteObservation     = "observation"
	semanticEntryRouteExplicitControl = "explicit_control"
	semanticEntryRouteOpenSemantic    = "open_semantic"
	semanticEntryRouteOther           = "other"
	semanticEntryRouteUnresolved      = "unresolved"

	semanticEntryScopeNone             = "none"
	semanticEntryScopeCurrentSelection = "current_selection"
	semanticEntryScopeProjectContext   = "project_context"

	semanticEntryControlNone         = "none"
	semanticEntryControlObserveOnly  = "observe_only"
	semanticEntryControlTyped        = "typed_control"
	semanticEntryControlSemanticLoop = "semantic_loop"
	semanticEntryControlOrdinary     = "ordinary_agent"

	semanticEntryAuthorizationNone      = "none"
	semanticEntryAuthorizationObserve   = "observe_only"
	semanticEntryAuthorizationAction    = "action_requested"
	semanticEntryVerifiedContextKey     = "semantic_entry_verified"
	semanticEntryDecisionContextKey     = "semantic_entry_decision"
	orchestrationDecisionContextKey     = "orchestration_controller_decision"
	semanticEntryMinimumRouteConfidence = 0.50
)

// semanticEntryDecision classifies only the user's requested interaction.
// Deliberately absent are processor family, plug-in identity, observation view,
// parameter, path, and vendor/product fields.
type semanticEntryDecision struct {
	SchemaVersion     string  `json:"schema_version"`
	Route             string  `json:"route"`
	Controller        string  `json:"controller,omitempty"`
	TargetScope       string  `json:"target_scope"`
	ControlMode       string  `json:"control_mode"`
	UserAuthorization string  `json:"user_authorization"`
	Confidence        float64 `json:"confidence"`
	Reason            string  `json:"reason"`
	RejectionReason   string  `json:"rejection_reason,omitempty"`
}

type semanticEntryModelInput struct {
	UserRequest     string   `json:"user_request"`
	AvailableScopes []string `json:"available_target_scopes"`
	SelectionKinds  []string `json:"current_selection_kinds,omitempty"`
}

func semanticEntrySystemPrompt() string {
	return `You are the model-owned semantic entry arbiter for an ordinary DAW Agent.
Classify only the interaction the user is requesting. Do not choose a treatment method or processor.

Return ONLY one semantic_entry_decision.v1 JSON object with exactly these fields:
{"schema_version":"semantic_entry_decision.v1","route":"discussion|observation|explicit_control|open_semantic|other|unresolved","controller":"ordinary_conversation|direct_typed_action|minimal_audio_closure|project_mix_workflow|none","target_scope":"none|current_selection|project_context","control_mode":"none|observe_only|typed_control|semantic_loop|ordinary_agent","user_authorization":"none|observe_only|action_requested","confidence":0.0,"reason":"short auditable reason","rejection_reason":"required only for unresolved"}

Route meanings:
- discussion: the user wants explanation, comparison, reasoning, or advice without requesting an observation or project change.
- observation: the user wants the Agent to inspect or evaluate available evidence without changing the project.
- explicit_control: the user requests a concrete control operation rather than leaving the acoustic method open.
- open_semantic: the user requests an audible or acoustic outcome while leaving the treatment method and controls for the Agent to decide after observation.
- A request that asks for inspection followed by a bounded, reversible improvement proposal
  that must wait for user confirmation is open_semantic with action_requested. Proposal
  admission is not a project mutation; do not downgrade it to observation merely because
  the user explicitly says not to execute yet.
- other: an ordinary Agent request outside acoustic treatment arbitration.
- unresolved: the request is too ambiguous to classify safely.

Controller meanings:
- ordinary_conversation: discussion or an ordinary request outside acoustic closure.
- direct_typed_action: an explicit concrete control operation.
- minimal_audio_closure: diagnose or close one bounded acoustic problem, including a project-context problem that is not a request to run the complete mix workflow.
- project_mix_workflow: only when the user explicitly invokes the fixed overall ProjectMix/Auto-Mix/Co-Mix workflow as a single staged capability. Do not select it merely because target_scope is project_context, the user asks about overall mix state, or the request may contain multiple evidence-grounded issues.
- none: unresolved only.

Protocol rules:
- discussion uses control_mode=none and user_authorization=none.
- observation uses control_mode=observe_only and user_authorization=observe_only.
- explicit_control uses control_mode=typed_control and user_authorization=action_requested.
- open_semantic uses control_mode=semantic_loop and user_authorization=action_requested.
- other uses control_mode=ordinary_agent and user_authorization=none or action_requested according to the user's request.
- unresolved uses control_mode=none, user_authorization=none, and a non-empty rejection_reason.
- discussion and other use controller=ordinary_conversation.
- observation uses controller=minimal_audio_closure.
- explicit_control uses controller=direct_typed_action.
- open_semantic normally uses controller=minimal_audio_closure, including project-context inspection followed by autonomous evidence-grounded issue handling. It may use controller=project_mix_workflow only when the request explicitly invokes the fixed staged whole-project workflow with target_scope=project_context.
- unresolved uses controller=none.
- Use only a target_scope supplied in available_target_scopes. Use none when no target is required.
- confidence is a number from 0 to 1. Use unresolved when confidence would be below 0.50.
- Do not output or recommend any processor family, plug-in, vendor, product, observation view, control axis, parameter identifier, plug-in path, or implementation workflow.
- Do not infer an acoustic treatment family from words in the request. Family selection, if later needed, belongs to a separate model turn after model-selected observation.`
}

func semanticEntryAvailableScopes(requestContext map[string]any) ([]string, []string) {
	scopes := []string{semanticEntryScopeNone, semanticEntryScopeProjectContext}
	kinds := make([]string, 0, 3)
	if contextHasAnyValue(requestContext, "selected_track_id", "selected_scene_track_id", "selected_plugin_track_id") {
		kinds = append(kinds, "track")
	}
	if contextHasAnyValue(requestContext, "selected_plugin_id") {
		kinds = append(kinds, "plugin")
	}
	if contextHasAnyValue(requestContext, "selected_clip_id", "piano_roll_focus_clip_id") || len(contextStringSlice(requestContext["selected_clip_ids"])) > 0 {
		kinds = append(kinds, "clip")
	}
	if len(kinds) > 0 {
		scopes = append(scopes, semanticEntryScopeCurrentSelection)
	}
	sort.Strings(kinds)
	return scopes, kinds
}

func (s *Server) planSemanticEntry(ctx context.Context, conversationID, userText string, requestContext map[string]any,
	cfg config.EngineConfig) (semanticEntryDecision, error) {
	if s == nil || s.llm == nil {
		return semanticEntryDecision{}, fmt.Errorf("semantic entry model is unavailable")
	}
	scopes, kinds := semanticEntryAvailableScopes(requestContext)
	input, _ := json.Marshal(semanticEntryModelInput{
		UserRequest: strings.TrimSpace(userText), AvailableScopes: scopes, SelectionKinds: kinds,
	})
	request := llm.Request{
		Messages: []llm.Message{
			{Role: "system", Content: semanticEntrySystemPrompt()},
			{Role: "user", Content: string(input)},
		},
		Metadata:   llm.RequestMetadata{Source: "ordinary_agent_semantic_entry", ConversationID: conversationID},
		PreferJSON: true,
	}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return semanticEntryDecision{}, err
	}
	decision, decodeErr := decodeSemanticEntryDecision(response.Text, scopes)
	if decodeErr == nil {
		return admitSemanticEntryController(userText, decision), nil
	}
	request.Messages = append(request.Messages,
		llm.Message{Role: "assistant", Content: response.Text},
		llm.Message{Role: "user", Content: "The semantic entry object was invalid: " + decodeErr.Error() + ". Return only a corrected object for the same request. Do not add fields or choose a processor family, observation view, plug-in, or parameter."},
	)
	response, err = s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return semanticEntryDecision{}, err
	}
	decision, err = decodeSemanticEntryDecision(response.Text, scopes)
	if err != nil {
		return semanticEntryDecision{}, fmt.Errorf("semantic entry remained invalid after repair: %w", err)
	}
	return admitSemanticEntryController(userText, decision), nil
}

// ProjectMixWorkflow is a fixed, project-wide capability, not the large-scope
// spelling of an ordinary semantic closure. The model may propose it, but the
// host admits that proposal only when the user explicitly names the fixed
// workflow. This keeps controller selection stable across model sampling while
// leaving acoustic-family and observation choices model-owned.
func admitSemanticEntryController(userText string, decision semanticEntryDecision) semanticEntryDecision {
	if decision.Controller != string(orchestrationcontroller.ProjectMixWorkflow) || explicitProjectMixInvocation(userText) {
		return decision
	}
	decision.Controller = string(orchestrationcontroller.MinimalAudioClosure)
	decision.Reason = "project-scoped semantic treatment without an explicit fixed ProjectMix/Auto-Mix/Co-Mix invocation"
	return decision
}

func explicitProjectMixInvocation(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	for _, marker := range []string{
		"projectmix", "project mix workflow", "project mix 工作流",
		"auto-mix", "automix", "auto mix", "自动混音工作流",
		"co-mix", "comix", "co mix", "协同混音工作流", "固定混音工作流",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func decodeSemanticEntryDecision(text string, availableScopes []string) (semanticEntryDecision, error) {
	var decision semanticEntryDecision
	text = strings.TrimSpace(text)
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return decision, fmt.Errorf("response does not contain a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(text[start : end+1])))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decision); err != nil {
		return semanticEntryDecision{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return semanticEntryDecision{}, fmt.Errorf("response contains trailing JSON data")
	}
	decision.SchemaVersion = strings.TrimSpace(decision.SchemaVersion)
	decision.Route = strings.ToLower(strings.TrimSpace(decision.Route))
	decision.Controller = strings.ToLower(strings.TrimSpace(decision.Controller))
	decision.TargetScope = strings.ToLower(strings.TrimSpace(decision.TargetScope))
	decision.ControlMode = strings.ToLower(strings.TrimSpace(decision.ControlMode))
	decision.UserAuthorization = strings.ToLower(strings.TrimSpace(decision.UserAuthorization))
	decision.Reason = strings.TrimSpace(decision.Reason)
	decision.RejectionReason = strings.TrimSpace(decision.RejectionReason)
	if decision.Controller == "" {
		decision.Controller = defaultSemanticEntryController(decision.Route)
	}
	if err := validateSemanticEntryDecision(decision, availableScopes); err != nil {
		return semanticEntryDecision{}, err
	}
	return decision, nil
}

func validateSemanticEntryDecision(decision semanticEntryDecision, availableScopes []string) error {
	if decision.SchemaVersion != semanticEntryDecisionSchema {
		return fmt.Errorf("schema_version must be %s", semanticEntryDecisionSchema)
	}
	if decision.Reason == "" {
		return fmt.Errorf("reason is required")
	}
	if decision.Confidence < 0 || decision.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}
	knownScope := false
	for _, scope := range availableScopes {
		if decision.TargetScope == scope {
			knownScope = true
			break
		}
	}
	if !knownScope {
		return fmt.Errorf("target_scope %q is not available", decision.TargetScope)
	}
	expectedMode := ""
	expectedAuthorization := ""
	switch decision.Route {
	case semanticEntryRouteDiscussion:
		expectedMode, expectedAuthorization = semanticEntryControlNone, semanticEntryAuthorizationNone
	case semanticEntryRouteObservation:
		expectedMode, expectedAuthorization = semanticEntryControlObserveOnly, semanticEntryAuthorizationObserve
	case semanticEntryRouteExplicitControl:
		expectedMode, expectedAuthorization = semanticEntryControlTyped, semanticEntryAuthorizationAction
	case semanticEntryRouteOpenSemantic:
		expectedMode, expectedAuthorization = semanticEntryControlSemanticLoop, semanticEntryAuthorizationAction
	case semanticEntryRouteOther:
		expectedMode = semanticEntryControlOrdinary
		if decision.UserAuthorization != semanticEntryAuthorizationNone && decision.UserAuthorization != semanticEntryAuthorizationAction {
			return fmt.Errorf("other requires user_authorization=none or action_requested")
		}
	case semanticEntryRouteUnresolved:
		expectedMode, expectedAuthorization = semanticEntryControlNone, semanticEntryAuthorizationNone
		if decision.RejectionReason == "" {
			return fmt.Errorf("unresolved requires rejection_reason")
		}
	default:
		return fmt.Errorf("route is invalid")
	}
	if decision.ControlMode != expectedMode {
		return fmt.Errorf("route %s requires control_mode=%s", decision.Route, expectedMode)
	}
	if expectedAuthorization != "" && decision.UserAuthorization != expectedAuthorization {
		return fmt.Errorf("route %s requires user_authorization=%s", decision.Route, expectedAuthorization)
	}
	if decision.Route != semanticEntryRouteUnresolved && decision.Confidence < semanticEntryMinimumRouteConfidence {
		return fmt.Errorf("confidence below %.2f requires route=unresolved", semanticEntryMinimumRouteConfidence)
	}
	if (decision.Route == semanticEntryRouteExplicitControl || decision.Route == semanticEntryRouteOpenSemantic) && decision.TargetScope == semanticEntryScopeNone {
		return fmt.Errorf("route %s requires a target scope", decision.Route)
	}
	if decision.Route == semanticEntryRouteUnresolved {
		if decision.Controller != "none" {
			return fmt.Errorf("unresolved requires controller=none")
		}
		return nil
	}
	if _, err := orchestrationControllerDecision(decision); err != nil {
		return err
	}
	return nil
}

func defaultSemanticEntryController(route string) string {
	switch strings.ToLower(strings.TrimSpace(route)) {
	case semanticEntryRouteDiscussion, semanticEntryRouteOther:
		return string(orchestrationcontroller.OrdinaryConversation)
	case semanticEntryRouteExplicitControl:
		return string(orchestrationcontroller.DirectTypedAction)
	case semanticEntryRouteObservation, semanticEntryRouteOpenSemantic:
		return string(orchestrationcontroller.MinimalAudioClosure)
	default:
		return "none"
	}
}

func orchestrationControllerDecision(decision semanticEntryDecision) (orchestrationcontroller.Decision, error) {
	return orchestrationcontroller.Select(orchestrationcontroller.SelectionInput{
		SemanticRoute: decision.Route, TargetScope: decision.TargetScope, ControlMode: decision.ControlMode,
		Authorization: decision.UserAuthorization, ProposedController: orchestrationcontroller.Kind(decision.Controller), Reason: decision.Reason,
	})
}

func semanticEntryDecisionMap(decision semanticEntryDecision) map[string]any {
	out := map[string]any{
		"schema_version": decision.SchemaVersion,
		"route":          decision.Route, "controller": decision.Controller, "target_scope": decision.TargetScope,
		"control_mode": decision.ControlMode, "user_authorization": decision.UserAuthorization,
		"confidence": decision.Confidence, "reason": decision.Reason,
	}
	if decision.RejectionReason != "" {
		out["rejection_reason"] = decision.RejectionReason
	}
	return out
}

func contextWithSemanticEntryDecision(requestContext map[string]any, decision semanticEntryDecision) map[string]any {
	values := map[string]any{
		semanticEntryVerifiedContextKey: true,
		semanticEntryDecisionContextKey: semanticEntryDecisionMap(decision),
	}
	if selected, err := orchestrationControllerDecision(decision); err == nil {
		values[orchestrationDecisionContextKey] = orchestrationControllerDecisionMap(selected)
	}
	return mergeContext(requestContext, values)
}

func contextWithoutUntrustedSemanticEntry(requestContext map[string]any) map[string]any {
	out := cloneContext(requestContext)
	delete(out, semanticEntryVerifiedContextKey)
	delete(out, semanticEntryDecisionContextKey)
	delete(out, orchestrationDecisionContextKey)
	delete(out, "semantic_entry_route")
	delete(out, "semantic_entry_unavailable")
	return out
}

func semanticEntryDecisionFromContext(requestContext map[string]any) (semanticEntryDecision, bool) {
	if !contextBool(requestContext, semanticEntryVerifiedContextKey) {
		return semanticEntryDecision{}, false
	}
	row := firstMapFromAny(requestContext[semanticEntryDecisionContextKey])
	if len(row) == 0 {
		return semanticEntryDecision{}, false
	}
	decision := semanticEntryDecision{
		SchemaVersion: firstStringFromMap(row, "schema_version"),
		Route:         firstStringFromMap(row, "route"), Controller: firstStringFromMap(row, "controller"), TargetScope: firstStringFromMap(row, "target_scope"),
		ControlMode: firstStringFromMap(row, "control_mode"), UserAuthorization: firstStringFromMap(row, "user_authorization"),
		Confidence: semanticEntryFloatValue(row["confidence"]), Reason: firstStringFromMap(row, "reason"),
		RejectionReason: firstStringFromMap(row, "rejection_reason"),
	}
	if decision.Controller == "" {
		decision.Controller = defaultSemanticEntryController(decision.Route)
	}
	if decision.SchemaVersion != semanticEntryDecisionSchema {
		return semanticEntryDecision{}, false
	}
	available, _ := semanticEntryAvailableScopes(requestContext)
	if err := validateSemanticEntryDecision(decision, available); err != nil {
		return semanticEntryDecision{}, false
	}
	return decision, true
}

func semanticEntryFloatValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		return parsed
	}
}

func semanticEntryUnresolvedResponse(conversationID, mode string, decision *semanticEntryDecision, err error) ChatResponse {
	reason := "The Agent could not classify this request safely. Please clarify whether you want discussion, read-only observation, an explicit control operation, or open acoustic treatment."
	status := "rejected"
	stopReason := "semantic_entry_invalid"
	workflowData := map[string]any{
		"schema_version": semanticEntryDecisionSchema,
		"status":         status, "mutation_performed": false,
	}
	if decision != nil {
		workflowData["decision"] = semanticEntryDecisionMap(*decision)
		reason = firstNonEmpty(decision.RejectionReason, decision.Reason, reason)
		status = "unresolved"
		stopReason = "semantic_entry_unresolved"
		workflowData["status"] = status
	}
	if err != nil {
		workflowData["error"] = err.Error()
	}
	return ChatResponse{
		ConversationID: conversationID, AgentMode: mode, Reply: reason,
		Workflow: "semantic_entry", WorkflowData: workflowData,
		GoalStatus: "completed", StopReason: stopReason,
	}
}
