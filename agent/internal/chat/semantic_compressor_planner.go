package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/semanticeffect"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const compressorEvaluationSchema = "semantic_effect.compressor_evaluation_contract.v1"
const compressorControlDecisionSchema = "semantic_effect.compressor_control_decision.v1"

type compressorControlDecision struct {
	SchemaVersion  string                                    `json:"schema_version"`
	Controls       []compressorControlDecisionControl        `json:"controls"`
	EvaluationAxes []semanticeffect.CompressorAxisEvaluation `json:"evaluation_axes"`
	Limitations    []string                                  `json:"limitations,omitempty"`
}

type compressorControlDecisionControl struct {
	Axis       string                                 `json:"axis"`
	PathKey    string                                 `json:"path_key"`
	Role       string                                 `json:"role"`
	Target     semanticeffect.CompressorControlTarget `json:"target"`
	Purpose    string                                 `json:"purpose"`
	Confidence string                                 `json:"confidence"`
}

func (s *Server) planOrdinaryAgentCompressorIntent(ctx context.Context, conversationID, userText string,
	card semanticeffect.AudioProcessorIdentityCard, cfg config.EngineConfig) (*semanticeffect.CompressorIntentPlan, error) {
	if s == nil || s.llm == nil {
		return nil, fmt.Errorf("compressor semantic intent planner is unavailable")
	}
	input, _ := json.Marshal(map[string]any{"user_request": userText, "processor_identity_card": card})
	system := `You are phase 1 of an ordinary DAW Agent's compressor semantic workflow.
The processor identity card is disclosed before acoustic observation so the device identity can inform what evidence and semantic axes matter. Product-name knowledge is only a prior. The card's live topology boundaries are authoritative.

Return ONLY semantic_effect.compressor_intent_plan.v1 JSON. Do not choose parameter values and do not call tools.
Shape:
{"schema_version":"semantic_effect.compressor_intent_plan.v1","user_goal":"copy exact request","negative_constraints":[],"selected_axes":["one or more axes"],"evidence_request":{"preferred_mode":"paired_io|source_only|not_needed","fallback_mode":"source_only|not_needed","dimensions":["source_dynamics|gain_action|transient_response|recovery_motion|level_effect|stereo_behavior|trigger_relation"],"reason":"why this evidence matters"},"reason":"how device identity and goal determine the investigation","needs_control_brief":true}

Allowed semantic axes: activation_intensity, transfer_severity, transient_timing, recovery_motion, detector_focus, output_normalization, parallel_balance, character.
Rules:
- Select only axes needed by the user goal, at most four.
- Prefer paired_io when current compressor behavior matters. source_only can describe material but cannot prove compressor behavior. not_needed is allowed for an exact user-specified setting.
- Never treat output gain as compression amount, or mix as internal compression intensity.
- Never invent threshold, ratio, attack, release, or any other capability forbidden by the card.
- Preserve negative listening constraints verbatim or faithfully paraphrased.`
	request := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(input)}},
		Metadata: llm.RequestMetadata{Source: "ordinary_agent_semantic_compressor_intent", ConversationID: conversationID}, PreferJSON: true}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return nil, err
	}
	plan, decodeErr := decodeCompressorIntentPlan(response.Text, userText)
	if decodeErr == nil {
		return &plan, nil
	}
	request.Messages = append(request.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: "Your JSON was invalid: " + decodeErr.Error() + ". Return only a corrected compressor intent plan for the same request and identity card."})
	response, err = s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return nil, err
	}
	plan, err = decodeCompressorIntentPlan(response.Text, userText)
	if err != nil {
		return nil, fmt.Errorf("compressor intent planner remained invalid after one repair: %w", err)
	}
	return &plan, nil
}

func (s *Server) planOrdinaryAgentCompressorControls(ctx context.Context, conversationID, userText string,
	trackID, pluginID, trackName, pluginName string, card semanticeffect.AudioProcessorIdentityCard,
	intent semanticeffect.CompressorIntentPlan, comContext map[string]any, brief plugingrabber.CompressorControlBrief,
	rejection *semanticeffect.CompressorMaterializationRejection, cfg config.EngineConfig) (*semanticeffect.CompressorPlan, error) {
	decisionEvidence := semanticCompressorDecisionProjection(comContext, intent.EvidenceRequest.Dimensions)
	input := map[string]any{
		"target_labels":            map[string]any{"track_name": trackName, "plugin_name": pluginName},
		"processor_identity_card":  card,
		"intent_and_evidence_plan": intent,
		"decision_evidence":        decisionEvidence,
		"candidate_control_brief":  brief,
	}
	if rejection != nil {
		input["deterministic_materialization_rejection"] = rejection
	}
	encoded, _ := json.Marshal(input)
	system := `You are phase 2 of an ordinary DAW Agent's compressor semantic workflow. Make only the musical control decision from the frozen intent, processor identity card, dimension-filtered COM evidence, and progressively disclosed candidate controls.

Return ONLY semantic_effect.compressor_control_decision.v1 JSON. This is a planning-only decision and cannot write parameters. The deterministic host owns target identity, evidence binding, proposal IDs, revision metadata, and the full evaluation contract envelope; do not repeat them.
Shape:
{"schema_version":"semantic_effect.compressor_control_decision.v1","controls":[{"axis":"selected axis","path_key":"exact disclosed path","role":"exact disclosed role","target":{"value_db":-12},"purpose":"brief reason","confidence":"low|medium|high"}],"evaluation_axes":[{"axis":"served axis","desired_direction":"bounded behavioral change","com_dimensions":["gain_action"],"acceptance_condition":"bounded future change_delta condition"}],"limitations":[]}

Target uses exactly one of value_db, ratio, value_ms, percent, display_value, enum_label. Choose an absolute target within reachable_summary; do not output normalized values or control_ref.
Match the target field to physical_unit. For enum/toggle controls or discrete choices represented by reachable labels, use an exact enum_label; do not express an enum timing choice as value_ms. Numeric ratio targets remain valid for ratio roles when the reachable summary provides numeric bounds.
Use only exact path_key/role pairs in candidate_control_brief. A visible role is not a reason to adjust it. Every control must serve one selected semantic axis, the user goal, cited evidence, reachable topology, and a future evaluation condition.
Return 1 to 6 controls. Include exactly one concise evaluation_axes row for every axis used by a control, and no unserved axes.
Output/makeup gain may only serve output_normalization. Mix/wet/dry may only serve parallel_balance. They never mean more compression.
The local card and control brief override product-name memory. Do not invent missing controls. A source_only COM result cannot prove existing gain action, transient response, recovery, or audible improvement; disclose that limitation and use it only to make a conservative plan.
Future evaluation is change_delta plus user acceptance; do not request or promise automatic second writes. Keep purpose, desired_direction, acceptance_condition, and limitations concise.`
	if rejection != nil {
		system += `
This is the one permitted constrained revision. Do not repeat unreachable path/role/target choices described by the deterministic rejection. Keep the frozen intent and use only reachable alternatives.`
	}
	request := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(encoded)}},
		Metadata: llm.RequestMetadata{Source: "ordinary_agent_semantic_compressor_planner", ConversationID: conversationID,
			PromptStats: map[string]any{"input_bytes": len(encoded), "decision_evidence_bytes": encodedJSONSize(decisionEvidence),
				"control_brief_bytes": encodedJSONSize(brief), "identity_card_bytes": encodedJSONSize(card)}}, PreferJSON: true}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return nil, err
	}
	plan, decodeErr := decodeCompressorControlDecisionForContext(response.Text, trackID, pluginID, userText, card.CardID,
		brief.TopologyGeneration, intent, comContext, rejection)
	if decodeErr == nil {
		return &plan, nil
	}
	request.Messages = append(request.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: "Your compressor control decision JSON was invalid: " + decodeErr.Error() + ". Return only one corrected compact decision for the same frozen context."})
	response, err = s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return nil, err
	}
	plan, err = decodeCompressorControlDecisionForContext(response.Text, trackID, pluginID, userText, card.CardID,
		brief.TopologyGeneration, intent, comContext, rejection)
	if err != nil {
		return nil, fmt.Errorf("compressor semantic control decision remained invalid after one JSON repair: %w", err)
	}
	return &plan, nil
}

func encodedJSONSize(value any) int {
	encoded, _ := json.Marshal(value)
	return len(encoded)
}

func decodeCompressorIntentPlan(text, userGoal string) (semanticeffect.CompressorIntentPlan, error) {
	var plan semanticeffect.CompressorIntentPlan
	if err := decodeJSONObject(text, &plan); err != nil {
		return plan, err
	}
	if strings.TrimSpace(plan.UserGoal) == "" {
		plan.UserGoal = strings.TrimSpace(userGoal)
	} else if strings.TrimSpace(plan.UserGoal) != strings.TrimSpace(userGoal) {
		return plan, fmt.Errorf("compressor intent planner changed the exact user goal")
	}
	if err := plan.Validate(); err != nil {
		return plan, err
	}
	return plan, nil
}

func decodeCompressorControlDecisionForContext(text, trackID, pluginID, userGoal, cardID, generation string,
	intent semanticeffect.CompressorIntentPlan, comContext map[string]any,
	rejection *semanticeffect.CompressorMaterializationRejection) (semanticeffect.CompressorPlan, error) {
	var decision compressorControlDecision
	if err := decodeJSONObject(text, &decision); err != nil {
		return semanticeffect.CompressorPlan{}, err
	}
	if decision.SchemaVersion != compressorControlDecisionSchema {
		return semanticeffect.CompressorPlan{}, fmt.Errorf("compressor control decision schema_version must be %s", compressorControlDecisionSchema)
	}
	allowed := map[string]bool{}
	for _, axis := range intent.SelectedAxes {
		allowed[axis] = true
	}
	served := map[string]bool{}
	evidenceRefs := semanticCompressorEvidenceRefs(comContext)
	controls := make([]semanticeffect.CompressorControlProposal, 0, len(decision.Controls))
	for index, control := range decision.Controls {
		if !allowed[control.Axis] {
			return semanticeffect.CompressorPlan{}, fmt.Errorf("compressor decision introduced unselected axis %q", control.Axis)
		}
		served[control.Axis] = true
		controls = append(controls, semanticeffect.CompressorControlProposal{
			ProposalID: compressorDecisionProposalID(index, control), Axis: control.Axis, PathKey: control.PathKey,
			Role: control.Role, Target: control.Target, Purpose: control.Purpose,
			EvidenceRefs: append([]string(nil), evidenceRefs...), Confidence: control.Confidence,
		})
	}
	selectedAxes := make([]string, 0, len(served))
	for _, axis := range intent.SelectedAxes {
		if served[axis] {
			selectedAxes = append(selectedAxes, axis)
		}
	}
	revision := semanticeffect.CompressorRevision{SchemaVersion: semanticeffect.CompressorRevisionSchema, MaxAttempts: 1}
	if rejection != nil {
		revision.Attempt, revision.RejectionID = 1, rejection.RejectionID
	}
	plan := semanticeffect.CompressorPlan{
		SchemaVersion: semanticeffect.CompressorPlanSchema, PlanningOnly: true, MutationAuthorized: false,
		Target: semanticeffect.Target{TrackID: trackID, PluginID: pluginID}, UserGoal: userGoal,
		NegativeConstraints: append([]string(nil), intent.NegativeConstraints...), IdentityCardID: cardID,
		TopologyGeneration: generation, SelectedAxes: selectedAxes, Evidence: semanticCompressorPlanEvidence(comContext),
		Controls: controls, Limitations: append([]string(nil), decision.Limitations...), Revision: revision,
		EvaluationContract: semanticeffect.CompressorEvaluationContract{
			SchemaVersion: compressorEvaluationSchema, FrozenUserGoal: userGoal,
			Axes:               append([]semanticeffect.CompressorAxisEvaluation(nil), decision.EvaluationAxes...),
			LevelMatchRequired: true, SuccessPolicy: "bounded_com_change_delta_plus_user_acceptance", NoAutoIteration: true,
		},
	}
	if err := plan.Validate(); err != nil {
		return plan, err
	}
	return plan, nil
}

func compressorDecisionProposalID(index int, control compressorControlDecisionControl) string {
	encoded, _ := json.Marshal(struct {
		Index   int                                    `json:"index"`
		Axis    string                                 `json:"axis"`
		PathKey string                                 `json:"path_key"`
		Role    string                                 `json:"role"`
		Target  semanticeffect.CompressorControlTarget `json:"target"`
	}{Index: index, Axis: control.Axis, PathKey: control.PathKey, Role: control.Role, Target: control.Target})
	sum := sha256.Sum256(encoded)
	return "cp1_" + hex.EncodeToString(sum[:10])
}

func semanticCompressorEvidenceRefs(comContext map[string]any) []string {
	refs := contextStringSlice(comContext["evidence_refs"])
	if len(refs) == 0 {
		refs = contextStringSlice(mapValue(comContext["llm_context"])["evidence_refs"])
	}
	return refs
}

func semanticCompressorPlanEvidence(comContext map[string]any) semanticeffect.CompressorPlanEvidence {
	summary := firstNonEmptyText(mapValue(comContext["llm_context"]), "summary_md")
	if summary == "" {
		summary = firstNonEmptyText(comContext, "reason", "status")
	}
	return semanticeffect.CompressorPlanEvidence{
		COMProjectionID: firstNonEmptyText(comContext, "projection_id"), COMMode: firstNonEmptyText(comContext, "mode"),
		COMStatus: firstNonEmptyText(comContext, "status"), EvidenceRefs: semanticCompressorEvidenceRefs(comContext), Summary: summary,
	}
}

func decodeJSONObject(text string, target any) error {
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(text), "```json"), "```"), "```"))
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return fmt.Errorf("response does not contain a JSON object")
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func validateCompressorPlanAgainstBrief(plan semanticeffect.CompressorPlan, brief plugingrabber.CompressorControlBrief) *semanticeffect.CompressorMaterializationRejection {
	available := map[string]plugingrabber.CompressorControlBriefControl{}
	for _, control := range brief.Controls {
		available[control.PathKey+"\x00"+control.Role] = control
	}
	rejected := []string{}
	reasons := []string{}
	alternatives := []string{}
	for _, candidate := range brief.Controls {
		alternatives = append(alternatives, candidate.PathKey+"/"+candidate.Role)
	}
	for _, proposal := range plan.Controls {
		control, ok := available[proposal.PathKey+"\x00"+proposal.Role]
		if !ok {
			rejected, reasons = append(rejected, proposal.ProposalID), append(reasons, proposal.PathKey+"/"+proposal.Role+" is not in the disclosed brief")
			continue
		}
		if unit, isEnum := compressorProposalTargetUnit(proposal.Target); isEnum {
			if len(control.Reachable.Labels) == 0 {
				rejected, reasons = append(rejected, proposal.ProposalID), append(reasons, proposal.Role+" has no disclosed reachable enum labels")
				continue
			}
		} else if !compressorUnitCompatible(proposal.Role, unit, control.PhysicalUnit) {
			rejected, reasons = append(rejected, proposal.ProposalID), append(reasons,
				fmt.Sprintf("%s target type %s is incompatible with disclosed physical unit %q", proposal.Role, unit, control.PhysicalUnit))
			continue
		}
		if value, ok := compressorProposalNumericTarget(proposal.Target); ok && control.Reachable.Minimum != nil && control.Reachable.Maximum != nil && (value < *control.Reachable.Minimum || value > *control.Reachable.Maximum) {
			rejected, reasons = append(rejected, proposal.ProposalID), append(reasons, fmt.Sprintf("%s target %.6g is outside [%.6g, %.6g]", proposal.Role, value, *control.Reachable.Minimum, *control.Reachable.Maximum))
		}
		if proposal.Target.EnumLabel != "" && len(control.Reachable.Labels) > 0 {
			found := false
			for _, label := range control.Reachable.Labels {
				found = found || strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(proposal.Target.EnumLabel))
			}
			if !found {
				rejected, reasons = append(rejected, proposal.ProposalID), append(reasons, proposal.Role+" enum label is unreachable")
			}
		}
	}
	if len(rejected) == 0 {
		return nil
	}
	hash := sha256.Sum256([]byte(strings.Join(reasons, "\n") + "\n" + brief.TopologyGeneration))
	rejection := &semanticeffect.CompressorMaterializationRejection{SchemaVersion: semanticeffect.CompressorRejectionSchema,
		RejectionID: "cmr1_" + hex.EncodeToString(hash[:10]), Status: "rejected", Code: "unreachable_control_plan", Reason: strings.Join(reasons, "; "),
		RejectedProposalIDs: rejected, ReachableAlternatives: alternatives, RevisionAllowed: true, MutationPerformed: false}
	return rejection
}

func compressorProposalTargetUnit(target semanticeffect.CompressorControlTarget) (string, bool) {
	switch {
	case target.ValueDB != nil:
		return "dB", false
	case target.Ratio != nil:
		return "ratio", false
	case target.ValueMS != nil:
		return "ms", false
	case target.Percent != nil:
		return "%", false
	case target.DisplayValue != nil:
		return "display", false
	default:
		return "enum", true
	}
}

func compressorProposalNumericTarget(target semanticeffect.CompressorControlTarget) (float64, bool) {
	for _, value := range []*float64{target.ValueDB, target.Ratio, target.ValueMS, target.Percent, target.DisplayValue} {
		if value != nil {
			return *value, true
		}
	}
	return 0, false
}
