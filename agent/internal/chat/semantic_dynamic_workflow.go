package chat

// This file contains the family-neutral semantic workflow for the five
// processor families that already have typed controllers.  The workflow is
// intentionally narrow: the model selects semantic axes, structural binding
// paths, and physical targets; this package binds those paths to a fresh
// topology and delegates every write to the existing typed controller.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
	agentruntime "vit-daw-agent/internal/runtime"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	semanticDynamicWorkflow       = "semantic_dynamic_execution"
	semanticDynamicSchema         = "semantic_dynamic_execution.v1"
	semanticDynamicReceipt        = "semantic_dynamic_receipt.v1"
	semanticDynamicIntent         = "semantic_dynamic_intent.v1"
	semanticDynamicDecisionSchema = "semantic_dynamic_control_decision.v1"
)

type semanticDynamicSpec struct {
	Family       string
	Processor    string
	Planner      string
	InspectTool  string
	ApplyTool    string
	CoverageAxes []string
}

type semanticDynamicBinding struct {
	PathKey       string
	Section       string
	Role          string
	ControlRef    string
	CurrentText   string
	PhysicalUnit  string
	CurrentValue  any
	Domain        map[string]any
	Reachable     []map[string]any
	StructuralKey string
}

type semanticDynamicBrief struct {
	SchemaVersion      string                    `json:"schema_version"`
	Family             string                    `json:"family"`
	TopologyGeneration string                    `json:"topology_generation"`
	SelectedAxes       []string                  `json:"selected_axes"`
	Controls           []semanticDynamicBriefRow `json:"controls"`
}

type semanticDynamicBriefRow struct {
	PathKey      string           `json:"path_key"`
	Section      string           `json:"section"`
	Role         string           `json:"role"`
	CurrentText  string           `json:"current_text,omitempty"`
	PhysicalUnit string           `json:"physical_unit,omitempty"`
	Current      any              `json:"current,omitempty"`
	Domain       map[string]any   `json:"domain,omitempty"`
	Reachable    []map[string]any `json:"reachable_values,omitempty"`
}

type semanticDynamicDecision struct {
	SchemaVersion string                       `json:"schema_version"`
	Controls      []semanticDynamicDecisionRow `json:"controls"`
	Limitations   []string                     `json:"limitations,omitempty"`
}

type semanticDynamicDecisionRow struct {
	Axis       string         `json:"axis"`
	PathKey    string         `json:"path_key"`
	Role       string         `json:"role"`
	Target     map[string]any `json:"target"`
	Purpose    string         `json:"purpose"`
	Confidence string         `json:"confidence"`
}

type semanticDynamicTicket struct {
	SchemaVersion      string                     `json:"schema_version"`
	TicketID           string                     `json:"ticket_id"`
	CreatedAt          string                     `json:"created_at"`
	Family             string                     `json:"family"`
	ProcessorType      string                     `json:"processor_type"`
	TrackID            string                     `json:"track_id"`
	PluginID           string                     `json:"plugin_id"`
	TopologyGeneration string                     `json:"topology_generation"`
	Intent             processorintent.Intent     `json:"intent"`
	Controls           []semanticDynamicTicketRow `json:"controls"`
	Observation        map[string]any             `json:"observation,omitempty"`
}

type semanticDynamicTicketRow struct {
	Axis       string         `json:"axis"`
	PathKey    string         `json:"path_key"`
	Role       string         `json:"role"`
	ControlRef string         `json:"control_ref"`
	Target     map[string]any `json:"target"`
	Purpose    string         `json:"purpose"`
	Confidence string         `json:"confidence"`
}

// semanticDynamicIntentRejection preserves an explicit model-owned
// unresolved result. It is intentionally distinct from malformed JSON so the
// caller never retries by asking the model to guess a family or coverage.
type semanticDynamicIntentRejection struct {
	Rejection processorintent.Rejection
}

func (e semanticDynamicIntentRejection) Error() string {
	return fmt.Sprintf("semantic intent unresolved: %s: %s", e.Rejection.Code, e.Rejection.Reason)
}

func semanticDynamicSpecForFamily(family string) (semanticDynamicSpec, bool) {
	family = strings.ToLower(strings.TrimSpace(family))
	switch family {
	case processorintent.FamilyLimiter:
		return semanticDynamicSpec{Family: family, Processor: "limiter", Planner: "semantic_limiter", InspectTool: "plugin_grabber.inspect_limiter", ApplyTool: "plugin_grabber.apply_limiter_controls", CoverageAxes: []string{"protection_intensity", "input_drive", "output_ceiling", "peak_mode", "recovery_motion", "detector_latency", "output_normalization", "threshold", "ceiling", "release", "lookahead", "output_gain"}}, true
	case processorintent.FamilyGateExpander:
		return semanticDynamicSpec{Family: family, Processor: "gate_expander", Planner: "semantic_gate_expander", InspectTool: "plugin_grabber.inspect_gate_expander", ApplyTool: "plugin_grabber.apply_gate_expander_controls", CoverageAxes: []string{"activation_threshold", "attenuation_floor", "detector_focus", "direction_mode", "state_timing", "output_normalization", "parallel_balance", "threshold", "range", "frequency_focus", "direction", "attack", "hold", "release", "output_gain", "mix"}}, true
	case processorintent.FamilyDeEsser:
		return semanticDynamicSpec{Family: family, Processor: "de_esser", Planner: "semantic_de_esser", InspectTool: "plugin_grabber.inspect_de_esser", ApplyTool: "plugin_grabber.apply_de_esser_controls", CoverageAxes: []string{"threshold_sensitivity", "detector_focus", "sibilance_reduction", "split_scope", "recovery_motion", "output_normalization", "parallel_balance", "threshold", "frequency_focus", "range", "attack", "release", "output_gain", "mix"}}, true
	case processorintent.FamilyTransientShaper:
		return semanticDynamicSpec{Family: family, Processor: "transient_shaper", Planner: "semantic_transient_shaper", InspectTool: "plugin_grabber.inspect_transient_shaper", ApplyTool: "plugin_grabber.apply_transient_shaper_controls", CoverageAxes: []string{"envelope_emphasis", "envelope_timing", "detector_focus", "shape_mode", "output_normalization", "parallel_balance", "attack", "sustain", "duration", "frequency_focus", "mode", "output_gain", "mix"}}, true
	case processorintent.FamilyMultibandDynamics:
		return semanticDynamicSpec{Family: family, Processor: "multiband_dynamics", Planner: "semantic_multiband", InspectTool: "plugin_grabber.inspect_multiband", ApplyTool: "plugin_grabber.apply_multiband_controls", CoverageAxes: []string{"band_dynamics", "band_timing", "crossover_layout", "detector_focus", "output_normalization", "parallel_balance", "threshold", "range", "ratio", "attack", "hold", "release", "crossover", "frequency_focus", "output_gain", "mix"}}, true
	default:
		return semanticDynamicSpec{}, false
	}
}

func semanticDynamicAxesForIntent(spec semanticDynamicSpec, coverage []string) []string {
	allowed := map[string]bool{}
	for _, axis := range spec.CoverageAxes {
		allowed[axis] = true
	}
	out := make([]string, 0, len(coverage))
	seen := map[string]bool{}
	for _, raw := range coverage {
		axis := strings.ToLower(strings.TrimSpace(raw))
		if axis == "" || seen[axis] {
			continue
		}
		if allowed[axis] {
			out = append(out, axis)
			seen[axis] = true
		}
	}
	return out
}

func semanticDynamicRoleAliases(family, axis string) map[string]bool {
	axis = strings.ToLower(strings.TrimSpace(axis))
	roles := map[string]bool{}
	add := func(values ...string) {
		for _, value := range values {
			roles[value] = true
		}
	}
	switch family {
	case processorintent.FamilyLimiter:
		switch axis {
		case "protection_intensity", "input_drive", "threshold":
			add("threshold", "input_drive")
		case "output_ceiling", "ceiling":
			add("ceiling")
		case "peak_mode":
			add("peak_mode", "limiter_mode", "mode")
		case "recovery_motion", "release":
			add("release", "auto_release")
		case "detector_latency", "lookahead":
			add("lookahead", "true_peak", "channel_link")
		case "output_normalization", "output_gain":
			add("output_gain")
		}
	case processorintent.FamilyGateExpander:
		switch axis {
		case "activation_threshold", "threshold":
			add("threshold", "hysteresis")
		case "attenuation_floor", "range":
			add("range", "expansion_ratio")
		case "detector_focus", "frequency_focus":
			add("sidechain_highpass", "sidechain_lowpass", "sidechain_filter", "detector_gain", "detector_character")
		case "direction_mode", "direction":
			add("direction", "direction_mode")
		case "state_timing", "attack", "hold", "release":
			add("attack", "hold", "release", "lookahead", "cycle_delay")
		case "output_normalization", "output_gain":
			add("output_gain")
		case "parallel_balance", "mix":
			add("mix")
		}
	case processorintent.FamilyDeEsser:
		switch axis {
		case "threshold_sensitivity", "threshold":
			add("threshold", "detection_amount")
		case "detector_focus", "frequency_focus":
			add("frequency", "frequency_focus", "focus_frequency", "detector_filter", "sidechain_highpass", "sidechain_lowpass")
		case "sibilance_reduction", "range":
			add("range", "reduction_amount", "reduction_range")
		case "split_scope":
			add("split_scope", "split_wide")
		case "recovery_motion", "attack", "release":
			add("attack", "release", "lookahead")
		case "output_normalization", "output_gain":
			add("output_gain")
		case "parallel_balance", "mix":
			add("mix")
		}
	case processorintent.FamilyTransientShaper:
		switch axis {
		case "envelope_emphasis", "attack", "sustain":
			// PCA v2 certifies the single-envelope TransX surface through
			// transient_range, while paired-envelope processors expose attack
			// and sustain amounts. All three are the canonical typed roles for
			// envelope_emphasis.
			add("attack_amount", "sustain_amount", "transient_range")
		case "envelope_timing", "duration":
			add("attack_duration", "sustain_duration", "duration", "release")
		case "detector_focus", "frequency_focus":
			// PCA v2 and the typed transient controller use the canonical
			// `focus_frequency` role. Keep the older `frequency_focus` alias
			// for already-materialized summaries, but do not reject the
			// certified controller surface after PCA admission.
			add("attack_sensitivity", "sustain_sensitivity", "detector_sensitivity", "focus_frequency", "frequency_focus")
		case "shape_mode", "mode":
			add("attack_shape", "sustain_shape", "shape", "mode", "processing_mode")
		case "output_normalization", "output_gain":
			add("output_gain")
		case "parallel_balance", "mix":
			add("mix")
		}
	case processorintent.FamilyMultibandDynamics:
		switch axis {
		case "band_dynamics", "threshold", "range", "ratio":
			add("threshold", "reduction_amount", "range", "ratio", "gain", "makeup_gain")
		case "band_timing", "attack", "hold", "release":
			add("attack", "hold", "release", "lookahead")
		case "crossover_layout", "crossover":
			add("crossover")
		case "detector_focus", "frequency_focus":
			add("frequency_focus", "sidechain_highpass", "sidechain_lowpass")
		case "output_normalization", "output_gain":
			add("output_gain", "makeup_gain")
		case "parallel_balance", "mix":
			add("mix")
		}
	}
	return roles
}

func semanticDynamicBindingMatchesAxis(family, axis, role string) bool {
	return semanticDynamicRoleAliases(family, axis)[strings.ToLower(strings.TrimSpace(role))]
}

func semanticDynamicGeneration(summary map[string]any) string {
	return firstStringFromMap(mapValue(summary["control_topology"]), "generation")
}

func semanticDynamicAppendRows(out *[]semanticDynamicBinding, rows []map[string]any, prefix, section string) {
	for index, row := range rows {
		role := strings.TrimSpace(firstStringFromMap(row, "role"))
		ref := strings.TrimSpace(firstStringFromMap(row, "control_ref"))
		if role == "" || ref == "" {
			continue
		}
		path := fmt.Sprintf("%s/%s/%s/%d", prefix, section, role, index)
		binding := semanticDynamicBinding{PathKey: path, Section: section, Role: role, ControlRef: ref,
			CurrentText: firstStringFromMap(row, "current_text"), PhysicalUnit: firstStringFromMap(row, "physical_unit"),
			CurrentValue: row["current_physical"], Domain: cloneContext(mapValue(row["domain"])), Reachable: mapRowsValue(row["reachable_values"]), StructuralKey: path}
		*out = append(*out, binding)
	}
}

func semanticDynamicBindings(family string, summary map[string]any) []semanticDynamicBinding {
	out := []semanticDynamicBinding{}
	appendSections := func(prefix string, source map[string]any, sections ...string) {
		for _, section := range sections {
			semanticDynamicAppendRows(&out, mapRowsValue(source[section]), prefix, section)
		}
	}
	switch family {
	case processorintent.FamilyLimiter:
		for _, stage := range mapRowsValue(summary["limiter_stages"]) {
			key := firstNonEmptyText(stage, "stage_key")
			appendSections("stage/"+key, stage, "operating_point", "safety", "timing", "detector", "mode", "output")
		}
		appendSections("shared", summary, "shared_controls")
	case processorintent.FamilyGateExpander:
		stage := mapValue(summary["gate_expander_stage"])
		appendSections("stage/"+firstNonEmptyText(stage, "stage_key"), stage, "detector", "operating_point", "gain_action", "direction_control", "timing", "mode", "output")
	case processorintent.FamilyDeEsser:
		for _, stage := range mapRowsValue(summary["de_esser_stages"]) {
			key := firstNonEmptyText(stage, "stage_key")
			appendSections("stage/"+key, stage, "operating_point", "frequency_selectivity", "timing", "mode", "output")
		}
	case processorintent.FamilyTransientShaper:
		stage := mapValue(summary["transient_shaper_stage"])
		appendSections("stage/"+firstNonEmptyText(stage, "stage_key"), stage, "envelope_action", "detector", "timing", "shape", "mode", "output")
	case processorintent.FamilyMultibandDynamics:
		filterbank := mapValue(summary["filterbank"])
		semanticDynamicAppendRows(&out, mapRowsValue(filterbank["crossovers"]), "filterbank", "frequency")
		for _, band := range mapRowsValue(summary["band_cells"]) {
			key := firstNonEmptyText(band, "band_key")
			appendSections("band/"+key, band, "operating_point", "transfer", "timing", "gain_action", "mode")
		}
		appendSections("shared", summary, "shared_controls")
	}
	return out
}

func semanticDynamicBriefFor(spec semanticDynamicSpec, summary map[string]any, axes []string) (semanticDynamicBrief, map[string]semanticDynamicBinding, error) {
	generation := semanticDynamicGeneration(summary)
	if generation == "" {
		return semanticDynamicBrief{}, nil, fmt.Errorf("topology_generation_unavailable")
	}
	bindings := semanticDynamicBindings(spec.Family, summary)
	brief := semanticDynamicBrief{SchemaVersion: "dynamic.control_brief.v1", Family: spec.Family, TopologyGeneration: generation, SelectedAxes: append([]string(nil), axes...)}
	byPath := map[string]semanticDynamicBinding{}
	for _, binding := range bindings {
		include := false
		for _, axis := range axes {
			if semanticDynamicBindingMatchesAxis(spec.Family, axis, binding.Role) {
				include = true
				break
			}
		}
		if !include {
			continue
		}
		byPath[binding.PathKey] = binding
		brief.Controls = append(brief.Controls, semanticDynamicBriefRow{PathKey: binding.PathKey, Section: binding.Section, Role: binding.Role, CurrentText: binding.CurrentText, PhysicalUnit: binding.PhysicalUnit, Current: binding.CurrentValue, Domain: binding.Domain, Reachable: binding.Reachable})
	}
	if len(brief.Controls) == 0 {
		return semanticDynamicBrief{}, nil, fmt.Errorf("required_coverage_unreachable")
	}
	for _, axis := range axes {
		found := false
		for _, binding := range bindings {
			if semanticDynamicBindingMatchesAxis(spec.Family, axis, binding.Role) {
				found = true
				break
			}
		}
		if !found {
			return semanticDynamicBrief{}, nil, fmt.Errorf("required_coverage_unreachable:%s", axis)
		}
	}
	return brief, byPath, nil
}

func (s *Server) readSemanticDynamicSurface(ctx context.Context, family, trackID, pluginID string) (plugingrabber.ParameterDigest, map[string]any, error) {
	if s == nil || s.eqKernelClient() == nil {
		return plugingrabber.ParameterDigest{}, nil, fmt.Errorf("kernel_unavailable")
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID, "plugin_id": pluginID, "include_parameters": true})
	if err != nil {
		return plugingrabber.ParameterDigest{}, nil, err
	}
	if !kernelReplyOK(reply) {
		return plugingrabber.ParameterDigest{}, nil, fmt.Errorf("parameter_read_failed:%s", firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
	}
	s.observePluginParametersReply(reply)
	digest := plugingrabber.BuildParameterDigest(reply)
	var summary map[string]any
	var boundary string
	switch family {
	case processorintent.FamilyLimiter:
		summary, boundary = plugingrabber.BuildLimiterSummaryWithBoundary(digest)
	case processorintent.FamilyGateExpander:
		summary, boundary = plugingrabber.BuildGateExpanderSummaryWithBoundary(digest)
	case processorintent.FamilyDeEsser:
		summary, boundary = plugingrabber.BuildDeEsserSummaryWithBoundary(digest)
	case processorintent.FamilyTransientShaper:
		summary, boundary = plugingrabber.BuildTransientShaperSummaryWithBoundary(digest)
	case processorintent.FamilyMultibandDynamics:
		summary, boundary = plugingrabber.BuildMultibandSummaryWithBoundary(digest)
	default:
		return digest, nil, fmt.Errorf("unsupported_dynamic_family:%s", family)
	}
	if summary == nil {
		return digest, nil, fmt.Errorf("%s", firstNonEmpty(boundary, "dynamic_topology_unavailable"))
	}
	var attachErr error
	switch family {
	case processorintent.FamilyLimiter:
		attachErr = attachLimiterControlRefs(summary, trackID, pluginID)
	case processorintent.FamilyGateExpander:
		attachErr = attachGateExpanderControlRefs(summary, trackID, pluginID)
	case processorintent.FamilyDeEsser:
		attachErr = attachDeEsserControlRefs(summary, trackID, pluginID)
	case processorintent.FamilyTransientShaper:
		attachErr = attachTransientShaperControlRefs(summary, trackID, pluginID)
	case processorintent.FamilyMultibandDynamics:
		attachErr = attachMultibandControlRefs(summary, trackID, pluginID)
	}
	if attachErr != nil {
		return digest, nil, fmt.Errorf("control_ref_binding_failed:%v", attachErr)
	}
	return digest, summary, nil
}

func semanticDynamicDecodeIntent(text, userGoal string, spec semanticDynamicSpec, evidenceRefs []string) (processorintent.Intent, error) {
	intent, err := processorintent.Decode(extractJSONObject(text))
	if err != nil {
		return processorintent.Intent{}, err
	}
	if intent.Status == processorintent.StatusUnresolved {
		if intent.Rejection == nil {
			return processorintent.Intent{}, fmt.Errorf("semantic intent unresolved without rejection")
		}
		return processorintent.Intent{}, semanticDynamicIntentRejection{Rejection: *intent.Rejection}
	}
	if intent.Status != processorintent.StatusResolved || intent.Family != spec.Family {
		return processorintent.Intent{}, fmt.Errorf("dynamic intent must resolve to %s", spec.Family)
	}
	if intent.ControlMode != processorintent.ControlModeSemantic && intent.ControlMode != processorintent.ControlModeTyped {
		return processorintent.Intent{}, fmt.Errorf("dynamic intent control_mode is not executable")
	}
	if strings.TrimSpace(intent.Intent) == "" {
		intent.Intent = strings.TrimSpace(userGoal)
	}
	if len(intent.EvidenceRefs) == 0 {
		intent.EvidenceRefs = append([]string(nil), evidenceRefs...)
	}
	if len(semanticDynamicAxesForIntent(spec, intent.RequiredCoverage)) != len(intent.RequiredCoverage) {
		return processorintent.Intent{}, fmt.Errorf("dynamic intent contains unsupported coverage")
	}
	return intent, nil
}

func (s *Server) planSemanticDynamicIntent(ctx context.Context, conversationID, userGoal string, spec semanticDynamicSpec, observation map[string]any, cfg config.EngineConfig) (processorintent.Intent, error) {
	if s == nil || s.llm == nil {
		return processorintent.Intent{}, fmt.Errorf("dynamic semantic intent planner unavailable")
	}
	input, _ := json.Marshal(map[string]any{"user_request": userGoal, "family": spec.Family, "allowed_required_coverage": spec.CoverageAxes, "observation": observation})
	system := fmt.Sprintf(`You are the semantic intent phase for an already selected %s processor family. The family was selected upstream by the model and is fixed for this turn. Return only %s JSON. Choose the smallest set of required coverage axes that the user's acoustic goal actually needs. Do not choose a plugin, path, parameter ID, vendor mapping, or physical value. If the goal cannot be resolved for this family, return status=unresolved with a structured rejection. Resolved shape: {"schema_version":"semantic_processor_intent.v1","status":"resolved","family":%q,"intent":"open acoustic intent","required_coverage":["allowed axis"],"scope":"current_track|current_selection|project|track_group","control_mode":"semantic_loop","confidence":0.0,"evidence_refs":[]}. Unresolved shape uses control_mode=unresolved and rejection={"code":"...","reason":"..."}.`, spec.Family, processorintent.SchemaVersion, spec.Family)
	request := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(input)}}, Metadata: llm.RequestMetadata{Source: "ordinary_agent_semantic_" + spec.Processor + "_intent", ConversationID: conversationID}, PreferJSON: true}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return processorintent.Intent{}, err
	}
	intent, err := semanticDynamicDecodeIntent(response.Text, userGoal, spec, semanticDynamicEvidenceRefs(observation))
	if err == nil {
		return intent, nil
	}
	var unresolved semanticDynamicIntentRejection
	if errors.As(err, &unresolved) {
		return processorintent.Intent{}, err
	}
	request.Messages = append(request.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: "Return one corrected semantic_processor_intent.v1 object. Do not add plugin identity, parameter IDs, or physical targets."})
	response, retryErr := s.llm.CompleteRequest(ctx, cfg, request)
	if retryErr != nil {
		return processorintent.Intent{}, retryErr
	}
	intent, err = semanticDynamicDecodeIntent(response.Text, userGoal, spec, semanticDynamicEvidenceRefs(observation))
	if err != nil {
		return processorintent.Intent{}, fmt.Errorf("dynamic intent remained invalid after repair: %w", err)
	}
	return intent, nil
}

func semanticDynamicEvidenceRefs(observation map[string]any) []string {
	refs := stringListValue(observation["evidence_refs"])
	if len(refs) == 0 {
		refs = stringListValue(mapValue(observation["summary"])["evidence_refs"])
	}
	return refs
}

func extractJSONObject(value string) string {
	text := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(textOrEmpty(value), "```json"), "```"), "```"))
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return text
	}
	return text[start : end+1]
}

func textOrEmpty(value string) string { return value }

func (s *Server) planSemanticDynamicControls(ctx context.Context, conversationID, userGoal string, intent processorintent.Intent, spec semanticDynamicSpec, observation map[string]any, brief semanticDynamicBrief, cfg config.EngineConfig) (semanticDynamicDecision, error) {
	input, _ := json.Marshal(map[string]any{"user_request": userGoal, "intent": intent, "observation": observation, "candidate_control_brief": brief})
	system := fmt.Sprintf(`You are the physical-target phase for a governed %s semantic processor. The family, intent, required coverage, and evidence are frozen. Return only %s JSON with this exact shape: {"schema_version":"semantic_dynamic_control_decision.v1","controls":[{"axis":"one exact intent.required_coverage item","path_key":"exact candidate_control_brief path_key","role":"exact candidate_control_brief role","target":{"one physical field":0},"purpose":"short reason","confidence":"low|medium|high"}],"limitations":[]}. axis is required on every control and may only copy an exact item from intent.required_coverage; it must never be omitted or invented. Select only disclosed path_key/role pairs from candidate_control_brief. Do not output control_ref, parameter ID, plugin path, product name, vendor mapping, normalized values, or any field not in target. Each target must use exactly one of value_db, value_ms, value_hz, ratio, percent, display_value, enum_label. Use an exact enum_label for discrete controls. Every selected required coverage axis must be served by at least one control. This is planning only and cannot write parameters.`, spec.Family, semanticDynamicDecisionSchema)
	request := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(input)}}, Metadata: llm.RequestMetadata{Source: "ordinary_agent_semantic_" + spec.Processor + "_planner", ConversationID: conversationID}, PreferJSON: true}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return semanticDynamicDecision{}, err
	}
	decision, err := decodeSemanticDynamicDecision(response.Text, intent, brief)
	if err == nil {
		return decision, nil
	}
	request.Messages = append(request.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: "Return one corrected semantic_dynamic_control_decision.v1 object. Every control must include axis copied exactly from intent.required_coverage, and may use only disclosed path_key/role pairs and one physical target field."})
	response, retryErr := s.llm.CompleteRequest(ctx, cfg, request)
	if retryErr != nil {
		return semanticDynamicDecision{}, retryErr
	}
	decision, err = decodeSemanticDynamicDecision(response.Text, intent, brief)
	if err != nil {
		return semanticDynamicDecision{}, fmt.Errorf("dynamic control decision remained invalid after repair: %w", err)
	}
	return decision, nil
}

func decodeSemanticDynamicDecision(text string, intent processorintent.Intent, brief semanticDynamicBrief) (semanticDynamicDecision, error) {
	var decision semanticDynamicDecision
	if err := json.Unmarshal([]byte(extractJSONObject(text)), &decision); err != nil {
		return decision, err
	}
	if decision.SchemaVersion != semanticDynamicDecisionSchema {
		return decision, fmt.Errorf("schema_version must be %s", semanticDynamicDecisionSchema)
	}
	if len(decision.Controls) == 0 || len(decision.Controls) > 6 {
		return decision, fmt.Errorf("controls must contain 1-6 rows")
	}
	semanticDynamicResolveUniqueBindings(&decision, brief)
	byPath := map[string]semanticDynamicBriefRow{}
	for _, row := range brief.Controls {
		byPath[row.PathKey] = row
	}
	allowedAxes := map[string]bool{}
	for _, axis := range intent.RequiredCoverage {
		allowedAxes[axis] = true
	}
	served := map[string]bool{}
	seen := map[string]bool{}
	for index := range decision.Controls {
		row := &decision.Controls[index]
		row.Axis, row.PathKey, row.Role, row.Confidence = strings.TrimSpace(row.Axis), strings.TrimSpace(row.PathKey), strings.TrimSpace(row.Role), strings.ToLower(strings.TrimSpace(row.Confidence))
		if !allowedAxes[row.Axis] {
			return decision, fmt.Errorf("control %d introduced unrequested axis %q", index, row.Axis)
		}
		if row.PathKey == "" || seen[row.PathKey] {
			return decision, fmt.Errorf("control %d path_key is missing or duplicated", index)
		}
		seen[row.PathKey] = true
		candidate, ok := byPath[row.PathKey]
		if !ok || !strings.EqualFold(candidate.Role, row.Role) {
			return decision, fmt.Errorf("control %d path_key/role is outside the disclosed brief", index)
		}
		if row.Purpose == "" {
			return decision, fmt.Errorf("control %d purpose is required", index)
		}
		if row.Confidence != "low" && row.Confidence != "medium" && row.Confidence != "high" {
			return decision, fmt.Errorf("control %d confidence is invalid", index)
		}
		if err := validateSemanticDynamicTarget(row.Target); err != nil {
			return decision, fmt.Errorf("control %d target: %w", index, err)
		}
		served[row.Axis] = true
	}
	for _, axis := range intent.RequiredCoverage {
		if !served[axis] {
			return decision, fmt.Errorf("required coverage axis %q was not served", axis)
		}
	}
	return decision, nil
}

// semanticDynamicResolveUniqueBindings keeps exact controller ownership on the
// server. A model may express a semantic axis and role without reproducing an
// opaque structural path. When that pair maps to exactly one freshly observed
// live binding, complete it here; ambiguity remains a validation failure.
func semanticDynamicResolveUniqueBindings(decision *semanticDynamicDecision, brief semanticDynamicBrief) {
	if decision == nil {
		return
	}
	for index := range decision.Controls {
		row := &decision.Controls[index]
		row.Axis = strings.TrimSpace(row.Axis)
		row.PathKey = strings.TrimSpace(row.PathKey)
		row.Role = strings.TrimSpace(row.Role)
		if row.PathKey != "" {
			continue
		}
		matches := make([]semanticDynamicBriefRow, 0, 1)
		for _, candidate := range brief.Controls {
			if !semanticDynamicBindingMatchesAxis(brief.Family, row.Axis, candidate.Role) {
				continue
			}
			if row.Role != "" && !strings.EqualFold(row.Role, candidate.Role) {
				continue
			}
			matches = append(matches, candidate)
		}
		if len(matches) == 1 {
			row.PathKey = matches[0].PathKey
			row.Role = matches[0].Role
		}
	}
}

func validateSemanticDynamicTarget(target map[string]any) error {
	allowed := map[string]bool{"value_db": true, "value_ms": true, "value_hz": true, "ratio": true, "percent": true, "display_value": true, "enum_label": true}
	count := 0
	for key, value := range target {
		if !allowed[key] {
			return fmt.Errorf("target field %q is not allowed", key)
		}
		if value == nil {
			continue
		}
		count++
		if key == "display_value" || key == "enum_label" {
			if strings.TrimSpace(fmt.Sprint(value)) == "" {
				return fmt.Errorf("%s must not be empty", key)
			}
			continue
		}
		number, ok := value.(float64)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
			return fmt.Errorf("%s must be finite numeric", key)
		}
	}
	if count != 1 {
		return fmt.Errorf("exactly one physical or enum target is required")
	}
	return nil
}

// materializeSemanticDynamicTarget converts the family-neutral physical
// target vocabulary into the exact field vocabulary accepted by the existing
// typed controller. It never introduces a parameter identity or a new value.
func materializeSemanticDynamicTarget(spec semanticDynamicSpec, binding semanticDynamicBinding, target map[string]any) (map[string]any, error) {
	if err := validateSemanticDynamicTarget(target); err != nil {
		return nil, err
	}
	out := cloneContext(target)
	if value, ok := out["value_hz"]; ok {
		switch spec.Family {
		case processorintent.FamilyGateExpander, processorintent.FamilyDeEsser, processorintent.FamilyTransientShaper:
			delete(out, "value_hz")
			out["frequency_hz"] = value
		case processorintent.FamilyLimiter:
			return nil, fmt.Errorf("frequency target is not supported by limiter role %s", binding.Role)
		}
	}
	return out, nil
}

func (s *Server) planBoundSemanticDynamic(ctx context.Context, conversationID, userText string, requestContext map[string]any, family string, observation map[string]any, cfg config.EngineConfig) ChatResponse {
	spec, ok := semanticDynamicSpecForFamily(family)
	if !ok {
		return semanticDynamicFailure(conversationID, requestContext, "unsupported_dynamic_family", fmt.Errorf("family %s is not a governed dynamic family", family))
	}
	trackID := firstStringFromMap(requestContext, "selected_plugin_track_id", "selected_track_id")
	pluginID := firstStringFromMap(requestContext, "selected_plugin_id")
	if trackID == "" || pluginID == "" {
		return semanticDynamicFailure(conversationID, requestContext, "target_unavailable", fmt.Errorf("selected dynamic processor target is missing"))
	}
	if observation == nil {
		observation = firstMapFromAny(requestContext["semantic_treatment_observation_context"])
	}
	intent := processorintent.Intent{}
	// Capability-owned planning may freeze the same typed intent without
	// entering the free-state improvement loop. Prefer that namespace so C2
	// remains an independent A-F capability session.
	raw := firstMapFromAny(requestContext["c2_semantic_processor_intent"])
	if len(raw) == 0 {
		raw = firstMapFromAny(requestContext["free_state_semantic_processor_intent"])
	}
	if len(raw) > 0 {
		encoded, _ := json.Marshal(raw)
		decoded, err := processorintent.Decode(string(encoded))
		if err != nil || decoded.Family != spec.Family || decoded.Status != processorintent.StatusResolved {
			return semanticDynamicFailure(conversationID, requestContext, "semantic_intent_rejected", firstErr(err, fmt.Errorf("semantic intent family mismatch")))
		}
		intent = decoded
	} else {
		var err error
		intent, err = s.planSemanticDynamicIntent(ctx, conversationID, userText, spec, observation, cfg)
		if err != nil {
			return semanticDynamicFailure(conversationID, requestContext, "intent_planning_failed", err)
		}
	}
	registry, err := processorregistry.Default()
	if err != nil {
		return semanticDynamicFailure(conversationID, requestContext, "pca_registry_unavailable", err)
	}
	pcaCoverage, err := registry.PCARequiredCoverage(spec.Family, intent.RequiredCoverage)
	if err != nil {
		return semanticDynamicFailure(conversationID, requestContext, "pca_coverage_unproven", err)
	}
	if _, err = s.semanticLoadedInstancePCAAdmissionForContext(ctx, requestContext, trackID, pluginID, semanticTreatmentPCAInput{Family: spec.Family, RequiredCoverage: pcaCoverage}); err != nil {
		return semanticDynamicFailure(conversationID, requestContext, "pca_admission_rejected", err)
	}
	digest, summary, err := s.readSemanticDynamicSurface(ctx, spec.Family, trackID, pluginID)
	if err != nil {
		return semanticDynamicFailure(conversationID, requestContext, "topology_unavailable", err)
	}
	axes := semanticDynamicAxesForIntent(spec, intent.RequiredCoverage)
	brief, bindings, err := semanticDynamicBriefFor(spec, summary, axes)
	if err != nil {
		return semanticDynamicFailure(conversationID, requestContext, "control_brief_unavailable", err)
	}
	decision, err := s.planSemanticDynamicControls(ctx, conversationID, userText, intent, spec, observation, brief, cfg)
	if err != nil {
		return semanticDynamicFailure(conversationID, requestContext, "control_planning_failed", err)
	}
	ticket, err := materializeSemanticDynamicTicket(spec, intent, decision, bindings, trackID, pluginID, semanticDynamicGeneration(summary), observation)
	if err != nil {
		return semanticDynamicFailure(conversationID, requestContext, "control_materialization_rejected", err)
	}
	if _, hasProgressive := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"]); hasProgressive {
		refs := make([]string, 0, len(ticket.Controls))
		for _, control := range ticket.Controls {
			refs = append(refs, control.ControlRef)
		}
		if err := semanticProgressiveDisclosureAdvanceToConfirmation(requestContext, refs); err != nil {
			return semanticDynamicFailure(conversationID, requestContext, "progressive_disclosure_boundary", err)
		}
	}
	return s.semanticDynamicWaitingResponse(conversationID, requestContext, ticket, brief, decision, digest)
}

func firstErr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

func materializeSemanticDynamicTicket(spec semanticDynamicSpec, intent processorintent.Intent, decision semanticDynamicDecision, bindings map[string]semanticDynamicBinding, trackID, pluginID, generation string, observation map[string]any) (semanticDynamicTicket, error) {
	ticket := semanticDynamicTicket{SchemaVersion: semanticDynamicSchema, TicketID: "dyn_" + randomID(), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Family: spec.Family, ProcessorType: spec.Processor, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, Intent: intent, Observation: cloneContext(observation)}
	for _, row := range decision.Controls {
		binding, ok := bindings[row.PathKey]
		if !ok || binding.ControlRef == "" {
			return semanticDynamicTicket{}, fmt.Errorf("control path %s is no longer bound", row.PathKey)
		}
		ticket.Controls = append(ticket.Controls, semanticDynamicTicketRow{Axis: row.Axis, PathKey: row.PathKey, Role: row.Role, ControlRef: binding.ControlRef, Target: cloneContext(row.Target), Purpose: row.Purpose, Confidence: row.Confidence})
	}
	if len(ticket.Controls) == 0 {
		return semanticDynamicTicket{}, fmt.Errorf("no controls materialized")
	}
	return ticket, nil
}

func (s *Server) semanticDynamicWaitingResponse(conversationID string, requestContext map[string]any, ticket semanticDynamicTicket, brief semanticDynamicBrief, decision semanticDynamicDecision, digest plugingrabber.ParameterDigest) ChatResponse {
	interactionID := "interaction_" + randomID()
	previewRows := make([]map[string]any, 0, len(ticket.Controls))
	for _, control := range ticket.Controls {
		previewRows = append(previewRows, map[string]any{"axis": control.Axis, "path_key": control.PathKey, "role": control.Role, "target": control.Target, "purpose": control.Purpose, "confidence": control.Confidence})
	}
	previewMap := map[string]any{"family": ticket.Family, "processor_type": ticket.ProcessorType, "track_id": ticket.TrackID, "plugin_id": ticket.PluginID, "topology_generation": ticket.TopologyGeneration, "controls": previewRows, "limitations": decision.Limitations}
	previewJSON, _ := json.Marshal(previewMap)
	workflowData := map[string]any{"schema_version": semanticDynamicSchema, "status": "waiting_confirmation", "planning_only": true, "mutation_authorized": false, "mutation_performed": false, "family": ticket.Family, "processor_type": ticket.ProcessorType, "intent": ticket.Intent, "control_brief": brief, "decision": decision, "ticket_id": ticket.TicketID, "post_action_verification": map[string]any{"status": "pending_model_observation", "requires_fresh_observation": true, "view_selection": "model_owned", "server_injected_view": false}}
	if _, ok := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"]); ok {
		workflowData["semantic_progressive_disclosure"] = requestContext["semantic_progressive_disclosure"]
	}
	interaction := AgentInteractionRequest{ID: interactionID, Kind: "confirmation", Type: "confirmation", Source: "vit_agent", Workflow: semanticDynamicWorkflow, Stage: "waiting_for_user", Title: "Confirm semantic processor adjustment", Body: string(previewJSON), Status: "waiting_for_user", ConversationID: conversationID, GoalID: firstStringFromMap(requestContext, "goal_id"), RunID: firstStringFromMap(requestContext, "run_id"), PlanID: ticket.TicketID, Payload: map[string]any{"ticket_id": ticket.TicketID, "preview": previewMap, "workflow": semanticDynamicWorkflow, "request_context": cloneContext(requestContext)}, Actions: []AgentInteractionAction{{ID: "approve", Label: "Confirm", Style: "primary", Recommended: true}, {ID: "cancel", Label: "Cancel", Style: "secondary"}}}
	s.storePendingInteraction(interaction, map[string]any{"workflow_data": workflowData, "ticket": ticket})
	return ChatResponse{ConversationID: conversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "The semantic plan is ready. No parameter has been written. Confirm to execute the typed controller.", NeedsConfirmation: true, PlanID: ticket.TicketID, Preview: string(previewJSON), Workflow: semanticDynamicWorkflow, WorkflowData: workflowData, InteractionRequests: []AgentInteractionRequest{interaction}, GoalStatus: string(agentruntime.StatusWaitingConfirmation), StopReason: "semantic_dynamic_confirmation_required"}
}

func (s *Server) continueSemanticDynamicExecutionInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	var ticket semanticDynamicTicket
	data, _ := json.Marshal(interaction.Data["ticket"])
	if err := json.Unmarshal(data, &ticket); err != nil || ticket.SchemaVersion != semanticDynamicSchema {
		return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "invalid_execution_ticket", fmt.Errorf("invalid dynamic execution ticket"))
	}
	clean := strings.ToLower(strings.TrimSpace(decision))
	if clean == "cancel" || clean == "deny" || clean == "reject" {
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "The semantic processor adjustment was cancelled; no parameters changed.", Workflow: semanticDynamicWorkflow, WorkflowData: map[string]any{"schema_version": semanticDynamicSchema, "status": "cancelled", "mutation_performed": false}, GoalStatus: string(agentruntime.StatusCancelled), StopReason: "semantic_dynamic_cancelled"}
	}
	if !isApprovalDecision(clean) {
		s.restorePendingInteraction(interaction)
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "Explicit confirmation or cancellation is required.", Workflow: semanticDynamicWorkflow, GoalStatus: string(agentruntime.StatusFailed), Error: "invalid_confirmation_decision", StopReason: "invalid_confirmation_decision"}
	}
	if _, ok := semanticProgressiveDisclosureState(interaction.RequestContext["semantic_progressive_disclosure"]); ok {
		if err := semanticProgressiveDisclosureConfirm(interaction.RequestContext); err != nil {
			return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "progressive_disclosure_confirmation_invalid", err)
		}
	}
	response := s.executeSemanticDynamicTicket(ctx, interaction, ticket)
	if _, ok := semanticProgressiveDisclosureState(interaction.RequestContext["semantic_progressive_disclosure"]); ok {
		if receipt := firstMapFromAny(response.WorkflowData["execution_receipt"]); len(receipt) > 0 {
			if err := semanticProgressiveDisclosureReceipt(interaction.RequestContext, receipt); err != nil {
				return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "progressive_disclosure_receipt_invalid", err)
			}
			response.WorkflowData["semantic_progressive_disclosure"] = interaction.RequestContext["semantic_progressive_disclosure"]
			response.WorkflowData["request_context"] = cloneContext(interaction.RequestContext)
		}
	}
	return response
}

func (s *Server) executeSemanticDynamicTicket(ctx context.Context, interaction PendingInteraction, ticket semanticDynamicTicket) ChatResponse {
	spec, ok := semanticDynamicSpecForFamily(ticket.Family)
	if !ok {
		return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "unsupported_dynamic_family", fmt.Errorf("unsupported family"))
	}
	registry, err := processorregistry.Default()
	if err != nil {
		return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "pca_registry_unavailable", err)
	}
	pcaCoverage, err := registry.PCARequiredCoverage(ticket.Family, ticket.Intent.RequiredCoverage)
	if err != nil {
		return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "pca_coverage_unproven", err)
	}
	if _, err = s.semanticLoadedInstancePCAAdmissionForContext(ctx, interaction.RequestContext, ticket.TrackID, ticket.PluginID, semanticTreatmentPCAInput{Family: ticket.Family, RequiredCoverage: pcaCoverage}); err != nil {
		return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "pca_admission_rejected", err)
	}
	_, summary, err := s.readSemanticDynamicSurface(ctx, ticket.Family, ticket.TrackID, ticket.PluginID)
	if err != nil {
		return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "stale_or_unavailable_target", err)
	}
	if semanticDynamicGeneration(summary) != ticket.TopologyGeneration {
		return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "stale_topology", fmt.Errorf("topology generation changed"))
	}
	bindings := semanticDynamicBindings(ticket.Family, summary)
	byPath := map[string]semanticDynamicBinding{}
	for _, binding := range bindings {
		byPath[binding.PathKey] = binding
	}
	controls := make([]map[string]any, 0, len(ticket.Controls))
	for _, control := range ticket.Controls {
		binding, found := byPath[control.PathKey]
		if !found || binding.ControlRef != control.ControlRef {
			return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "stale_control_ref", fmt.Errorf("control binding changed for %s", control.PathKey))
		}
		row := map[string]any{"control_ref": control.ControlRef}
		target, targetErr := materializeSemanticDynamicTarget(spec, binding, control.Target)
		if targetErr != nil {
			return semanticDynamicFailure(interaction.ConversationID, interaction.RequestContext, "target_materialization_rejected", targetErr)
		}
		for key, value := range target {
			row[key] = value
		}
		controls = append(controls, row)
	}
	cmd := map[string]any{"track_id": ticket.TrackID, "plugin_id": ticket.PluginID, "atomic": true, "controls": controls}
	// The settlement bracket (D2-SEMREC1) opens only for a free-state D1
	// experiment awaiting its single forward mutation: kernel-real before
	// revision, persisted before render, and the pre-write drift check. A
	// bracket that cannot open fails the execution before any parameter is
	// written; every other caller (C2 batches, capability sessions) proceeds
	// with the historical unbracketed shape.
	bracket, bracketErr := s.beginSemanticDynamicSettlement(ctx, interaction, ticket)
	if bracketErr != nil {
		return semanticDynamicExecutionFailure(interaction, ticket, "settlement_bracket_unavailable", bracketErr, nil)
	}
	if bracket != nil {
		cmd["request_id"] = bracket.RequestID
	}
	applyResult, applyErr := s.applySemanticDynamicController(ctx, spec, cmd, interaction.RequestContext)
	if applyErr != nil {
		return semanticDynamicExecutionFailure(interaction, ticket, "typed_controller_failed", applyErr, applyResult)
	}
	if err := validateSemanticDynamicControllerResult(applyResult, ticket); err != nil {
		return semanticDynamicExecutionFailure(interaction, ticket, "typed_controller_result_invalid", err, applyResult)
	}
	receipt := map[string]any{"schema_version": semanticDynamicReceipt, "status": "executed", "ticket_id": ticket.TicketID, "family": ticket.Family, "processor_type": ticket.ProcessorType, "track_id": ticket.TrackID, "plugin_id": ticket.PluginID, "topology_generation": ticket.TopologyGeneration, "controller_result": semanticDynamicControllerResultSummary(applyResult), "rollback": map[string]any{"status": "not_needed", "verified": true}, "post_action_verification": map[string]any{"status": "pending_model_observation", "requires_fresh_observation": true, "view_selection": "model_owned", "server_injected_view": false}}
	if bracket != nil {
		if settlement, ok := s.completeSemanticDynamicSettlement(ctx, bracket, ticket, applyResult); ok {
			// receipt_id correlates the booked intervention with the journaled
			// action (recordFreeStateExperimentAction reads it as the action id).
			receipt["receipt_id"] = bracket.ActionID
			for key, value := range settlement {
				receipt[key] = value
			}
		} else {
			// The mutation executed but its kernel settlement evidence could
			// not be proven; the D1 chain must refuse downstream rather than
			// trust an unproven revision, so no settlement fields are added.
			receipt["settlement_verified"] = false
		}
	}
	response := ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "The typed semantic processor adjustment completed with readback and transaction evidence. A fresh post-action observation is required before the original goal can be marked satisfied.", Workflow: semanticDynamicWorkflow, WorkflowData: map[string]any{"schema_version": semanticDynamicSchema, "status": "executed", "planning_only": false, "mutation_authorized": true, "mutation_performed": true, "execution_receipt": receipt, "post_action_verification": receipt["post_action_verification"]}, GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_dynamic_execution_completed"}
	return response
}

func (s *Server) applySemanticDynamicController(ctx context.Context, spec semanticDynamicSpec, cmd, requestContext map[string]any) (map[string]any, error) {
	switch spec.Family {
	case processorintent.FamilyLimiter:
		return s.applyPluginGrabberLimiterControls(ctx, cmd, requestContext)
	case processorintent.FamilyGateExpander:
		return s.applyPluginGrabberGateExpanderControls(ctx, cmd, requestContext)
	case processorintent.FamilyDeEsser:
		return s.applyPluginGrabberDeEsserControls(ctx, cmd, requestContext)
	case processorintent.FamilyTransientShaper:
		return s.applyPluginGrabberTransientShaperControls(ctx, cmd, requestContext)
	case processorintent.FamilyMultibandDynamics:
		return s.applyPluginGrabberMultibandControls(ctx, cmd, requestContext)
	default:
		return nil, fmt.Errorf("unsupported dynamic family")
	}
}

func validateSemanticDynamicControllerResult(result map[string]any, ticket semanticDynamicTicket) error {
	status := firstStringFromMap(result, "status")
	if status != "exact" && status != "quantized" {
		return fmt.Errorf("typed controller returned status %q", status)
	}
	if !boolValue(result["atomic"]) {
		return fmt.Errorf("typed controller did not attest atomic execution")
	}
	if firstStringFromMap(result, "track_id") != ticket.TrackID || firstStringFromMap(result, "plugin_id") != ticket.PluginID || firstStringFromMap(result, "topology_generation") != ticket.TopologyGeneration {
		return fmt.Errorf("typed controller target or topology mismatch")
	}
	if firstStringFromMap(result, "restore_ref") == "" {
		return fmt.Errorf("typed controller omitted restore_ref")
	}
	rows := mapRowsValue(result["controls"])
	if len(rows) != len(ticket.Controls) {
		return fmt.Errorf("typed controller returned %d controls for %d requested", len(rows), len(ticket.Controls))
	}
	for index, row := range rows {
		status := firstStringFromMap(row, "status")
		if status != "exact" && status != "quantized" {
			return fmt.Errorf("control %d returned invalid status", index)
		}
		if len(mapRowsValue(row["actual_readback"])) == 0 {
			return fmt.Errorf("control %d omitted actual readback", index)
		}
	}
	return nil
}

func semanticDynamicControllerResultSummary(result map[string]any) map[string]any {
	return map[string]any{"status": result["status"], "atomic": result["atomic"], "control_count": len(mapRowsValue(result["controls"])), "restore_ref": firstStringFromMap(result, "restore_ref"), "rollback": result["rollback"]}
}

func semanticDynamicExecutionFailure(interaction PendingInteraction, ticket semanticDynamicTicket, code string, err error, controllerResult map[string]any) ChatResponse {
	data := map[string]any{"schema_version": semanticDynamicSchema, "status": "failed", "mutation_authorized": true, "mutation_performed": false, "execution": map[string]any{"ticket_id": ticket.TicketID, "failure_code": code, "message": err.Error()}, "post_action_verification": map[string]any{"status": "blocked", "requires_fresh_observation": false}}
	if len(controllerResult) > 0 {
		data["controller_result"] = semanticDynamicControllerResultSummary(controllerResult)
	}
	return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "The typed semantic processor action was rejected or rolled back; no unverified continuation was started.", Error: err.Error(), Workflow: semanticDynamicWorkflow, WorkflowData: data, GoalStatus: string(agentruntime.StatusFailed), StopReason: code}
}

func semanticDynamicFailure(conversationID string, requestContext map[string]any, code string, err error) ChatResponse {
	goalID, runID := goalIDsFromContext(requestContext)
	return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID, Reply: "The dynamic semantic workflow reached a governed boundary; no parameter was written.", Error: err.Error(), Workflow: semanticDynamicWorkflow, WorkflowData: map[string]any{"schema_version": semanticDynamicSchema, "status": "rejected", "rejection_code": code, "mutation_performed": false, "rejection_reason": err.Error()}, GoalStatus: string(agentruntime.StatusCompleted), StopReason: code}
}

func semanticDynamicObservationFromRecent(observation *agentloop.RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	return semanticTreatmentObservationPayload(observation)
}
