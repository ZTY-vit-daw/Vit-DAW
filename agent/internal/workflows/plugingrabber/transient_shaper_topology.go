package plugingrabber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strings"
)

const transientShaperTopologySchema = "transient-shaper-control-topology/v1"

type TransientShaperModel struct {
	Classification  string
	Confidence      float64
	SemanticMode    string
	Stage           TransientShaperStage
	AuxiliaryStages []TransientShaperAuxiliaryStage
	Generation      string
}

type TransientShaperStage struct {
	Key            string
	EnvelopeAction []CompressorBinding
	Detector       []CompressorBinding
	Timing         []CompressorBinding
	Shape          []CompressorBinding
	Mode           []CompressorBinding
	Output         []CompressorBinding
	Capabilities   TransientShaperCapabilities
}

type TransientShaperCapabilities struct {
	EnvelopeAction []string
	Detector       []string
	Timing         []string
	Shape          []string
	Mode           []string
	Output         []string
}

type TransientShaperAuxiliaryStage struct {
	Kind     string
	ParamIDs []string
}

type transientShaperCandidate struct {
	Param         ParameterInfo
	Role          string
	Section       string
	AuxiliaryKind string
}

var transientShaperBandToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:band|b)\s*[1-9][0-9]*(?:[^a-z0-9]|$)`)
var transientShaperIndexedAmount = regexp.MustCompile(`(?i)(?:attack|sustain|range|amount)\s*[1-9][0-9]*\s*$`)

// DetectTransientShaperModelWithBoundary recovers only a parameter-proved
// transient-envelope stage. Identity, vendor, normalized roles, and current
// amount values are excluded from classification. Current processing mode is
// consulted only when the parameter labels themselves prove role remapping.
func DetectTransientShaperModelWithBoundary(digest ParameterDigest) (*TransientShaperModel, string) {
	mode, modeBoundary := transientShaperSemanticMode(digest)
	if modeBoundary != "" {
		return nil, modeBoundary
	}
	candidates := make([]transientShaperCandidate, 0, len(digest.Parameters))
	auxiliary := map[string][]string{}
	for _, param := range digest.Parameters {
		candidate := classifyTransientShaperParameter(param, mode)
		if candidate.AuxiliaryKind != "" {
			auxiliary[candidate.AuxiliaryKind] = append(auxiliary[candidate.AuxiliaryKind], param.ID)
			continue
		}
		if candidate.Role != "" && param.HostControllable {
			candidates = append(candidates, candidate)
		}
	}
	if boundary := transientShaperUnsupportedBoundary(digest, candidates); boundary != "" {
		return nil, boundary
	}
	stage := buildTransientShaperStage(candidates)
	classification, confidence := classifyTransientShaperStage(stage)
	if classification == "" {
		if len(candidates) > 0 {
			return nil, "unresolved_transient_shaper_surface"
		}
		return nil, "not_transient_shaper"
	}
	model := &TransientShaperModel{Classification: classification, Confidence: confidence, SemanticMode: mode, Stage: stage}
	for kind, ids := range auxiliary {
		sort.Strings(ids)
		model.AuxiliaryStages = append(model.AuxiliaryStages, TransientShaperAuxiliaryStage{Kind: kind, ParamIDs: ids})
	}
	sort.Slice(model.AuxiliaryStages, func(i, j int) bool { return model.AuxiliaryStages[i].Kind < model.AuxiliaryStages[j].Kind })
	model.Generation = transientShaperTopologyGeneration(model)
	return model, ""
}

func DetectTransientShaperModel(digest ParameterDigest) *TransientShaperModel {
	model, _ := DetectTransientShaperModelWithBoundary(digest)
	return model
}

func BuildTransientShaperSummaryWithBoundary(digest ParameterDigest) (map[string]any, string) {
	model, boundary := DetectTransientShaperModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	auxiliary := make([]map[string]any, 0, len(model.AuxiliaryStages))
	for _, stage := range model.AuxiliaryStages {
		auxiliary = append(auxiliary, map[string]any{"kind": stage.Kind, "param_ids": stage.ParamIDs})
	}
	return map[string]any{
		"schema_version": transientShaperTopologySchema,
		"mapping_source": "generic_structural",
		"classification": model.Classification,
		"confidence":     model.Confidence,
		"semantic_mode":  model.SemanticMode,
		"control_topology": map[string]any{
			"schema_version": transientShaperTopologySchema,
			"generation":     model.Generation,
		},
		"transient_shaper_stage": transientShaperStageSummary(model.Stage),
		"auxiliary_stages":       auxiliary,
	}, ""
}

func BuildTransientShaperSummary(digest ParameterDigest) map[string]any {
	summary, _ := BuildTransientShaperSummaryWithBoundary(digest)
	return summary
}

func transientShaperSemanticMode(digest ParameterDigest) (string, string) {
	for _, param := range digest.Parameters {
		text := transientShaperParameterText(param)
		if !(transientShaperHasToken(text, "mode") || strings.Contains(text, "processing mode")) {
			continue
		}
		labels := transientShaperLabels(param)
		hasFull, hasDual, hasShelf := false, false, false
		for _, label := range labels {
			lower := strings.ToLower(strings.TrimSpace(label))
			hasFull = hasFull || strings.Contains(lower, "full range") || strings.Contains(lower, "fullrange")
			hasDual = hasDual || strings.Contains(lower, "dual band") || strings.Contains(lower, "dualband")
			hasShelf = hasShelf || strings.Contains(lower, "shelf eq") || strings.Contains(lower, "shelving eq")
		}
		if !(hasFull && (hasDual || hasShelf)) {
			continue
		}
		current := strings.ToLower(strings.TrimSpace(param.ValueText))
		if strings.Contains(current, "full range") || strings.Contains(current, "fullrange") {
			return "full_range", ""
		}
		if strings.Contains(current, "dual band") || strings.Contains(current, "dualband") {
			return "", "unsupported_dual_band_mode"
		}
		if strings.Contains(current, "shelf") {
			return "", "unsupported_eq_mode"
		}
		return "", "unresolved_processing_mode"
	}
	return "fixed_transient", ""
}

func classifyTransientShaperParameter(param ParameterInfo, semanticMode string) transientShaperCandidate {
	text := transientShaperParameterText(param)
	candidate := transientShaperCandidate{Param: param}
	if text == "" || transientShaperContains(text, "preset", "meter", "display", "bypass") {
		return candidate
	}
	if transientShaperContains(text, "guard", "limiter", "limit stage", "true peak") || transientShaperHasToken(text, "limit") {
		candidate.AuxiliaryKind = "limiter"
		if transientShaperContains(text, "clip") || transientShaperLabelsContain(param, "clip") {
			candidate.AuxiliaryKind = "limiter_or_clipper"
		}
		return candidate
	}
	if transientShaperContains(text, "clipper", "clip ceiling", "clip threshold") {
		candidate.AuxiliaryKind = "clipper"
		return candidate
	}
	if transientShaperBandToken.MatchString(text) || transientShaperIndexedAmount.MatchString(text) ||
		transientShaperContains(text, "crossover", "cross over") {
		candidate.AuxiliaryKind = "multiband_dynamics"
		return candidate
	}
	if transientShaperContains(text, "attack/gain h", "attack gain h") {
		if semanticMode == "full_range" {
			candidate.Role, candidate.Section = "attack_amount", "envelope_action"
		}
		return candidate
	}
	if transientShaperContains(text, "sustain/gain l", "sustain gain l") {
		if semanticMode == "full_range" {
			candidate.Role, candidate.Section = "sustain_amount", "envelope_action"
		}
		return candidate
	}
	switch {
	case transientShaperContains(text, "attack sensitivity", "attacksensitivity", "attack sense", "attacksense"):
		candidate.Role, candidate.Section = "attack_sensitivity", "detector"
	case transientShaperContains(text, "sustain sensitivity", "sustainsensitivity", "sustain sense", "sustainsense"):
		candidate.Role, candidate.Section = "sustain_sensitivity", "detector"
	case transientShaperHasToken(text, "sense") || transientShaperHasToken(text, "sensitivity"):
		candidate.Role, candidate.Section = "detector_sensitivity", "detector"
	case transientShaperContains(text, "attack duration", "attackduration"):
		candidate.Role, candidate.Section = "attack_duration", "timing"
	case transientShaperContains(text, "sustain duration", "sustainduration"):
		candidate.Role, candidate.Section = "sustain_duration", "timing"
	case transientShaperHasToken(text, "duration"):
		candidate.Role, candidate.Section = "duration", "timing"
	case transientShaperHasToken(text, "release"):
		candidate.Role, candidate.Section = "release", "timing"
	case transientShaperContains(text, "attack shape", "attackshape"):
		candidate.Role, candidate.Section = "attack_shape", "shape"
	case transientShaperContains(text, "sustain shape", "sustainshape"):
		candidate.Role, candidate.Section = "sustain_shape", "shape"
	case transientShaperContains(text, "sidechain filter on", "sidechain filter enable", "sc filter on", "sc filter enable"):
		candidate.Role, candidate.Section = "focus_enable", "detector"
	case transientShaperContains(text, "sc freq", "focus frequency"):
		candidate.Role, candidate.Section = "focus_frequency", "detector"
	case transientShaperContains(text, "sidechain filter", "sc filter"):
		candidate.Role, candidate.Section = "focus_mode", "detector"
	case transientShaperHasToken(text, "mode") || strings.Contains(text, "processing mode"):
		candidate.Role, candidate.Section = "processing_mode", "mode"
	case transientShaperHasToken(text, "attack") && transientShaperSignedActionDomain(param):
		candidate.Role, candidate.Section = "attack_amount", "envelope_action"
	case transientShaperHasToken(text, "sustain") && transientShaperSignedActionDomain(param):
		candidate.Role, candidate.Section = "sustain_amount", "envelope_action"
	case transientShaperHasToken(text, "range") && transientShaperSignedActionDomain(param):
		candidate.Role, candidate.Section = "transient_range", "envelope_action"
	case transientShaperHasToken(text, "mix") || transientShaperContains(text, "dry/wet", "dry wet"):
		candidate.Role, candidate.Section = "mix", "output"
	case transientShaperContains(text, "output gain", "output level") || transientShaperHasToken(text, "output"):
		candidate.Role, candidate.Section = "output_gain", "output"
	}
	return candidate
}

func transientShaperSignedActionDomain(param ParameterInfo) bool {
	minimum, maximum, observed := math.Inf(1), math.Inf(-1), false
	observe := func(value float64) {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return
		}
		minimum, maximum, observed = math.Min(minimum, value), math.Max(maximum, value), true
	}
	if param.DisplayDomainCandidate != nil && param.DisplayDomainCandidate.Min != nil && param.DisplayDomainCandidate.Max != nil {
		unit := strings.ToLower(strings.TrimSpace(param.DisplayDomainCandidate.Unit))
		if unit == "ms" || unit == "s" || unit == "hz" || unit == "ratio" || unit == "enum" || unit == "toggle" {
			return false
		}
		observe(*param.DisplayDomainCandidate.Min)
		observe(*param.DisplayDomainCandidate.Max)
	}
	if param.DisplayProbe != nil {
		for _, sample := range param.DisplayProbe.Samples {
			if value, ok := parseCompressorPhysical("transient_amount", sample.Text); ok {
				observe(value)
			}
		}
	}
	return observed && minimum < 0 && maximum > 0
}

func buildTransientShaperStage(candidates []transientShaperCandidate) TransientShaperStage {
	stage := TransientShaperStage{Key: "transient_shaper"}
	for _, candidate := range candidates {
		binding := transientShaperBinding(candidate)
		switch candidate.Section {
		case "envelope_action":
			stage.EnvelopeAction = append(stage.EnvelopeAction, binding)
		case "detector":
			stage.Detector = append(stage.Detector, binding)
		case "timing":
			stage.Timing = append(stage.Timing, binding)
		case "shape":
			stage.Shape = append(stage.Shape, binding)
		case "mode":
			stage.Mode = append(stage.Mode, binding)
		case "output":
			stage.Output = append(stage.Output, binding)
		}
	}
	for _, bindings := range []*[]CompressorBinding{&stage.EnvelopeAction, &stage.Detector, &stage.Timing, &stage.Shape, &stage.Mode, &stage.Output} {
		sortCompressorBindings(bindings)
	}
	stage.Capabilities = transientShaperCapabilities(stage)
	return stage
}

func transientShaperBinding(candidate transientShaperCandidate) CompressorBinding {
	param := candidate.Param
	binding := CompressorBinding{ParamID: param.ID, Name: parameterDisplayName(param), Role: candidate.Role,
		Channel: "shared", CompressionStage: "transient_shaper", CurrentText: param.ValueText,
		CurrentNormalized: anyFloat(param.NormalizedValue), Domain: param.DisplayDomainCandidate,
		Curve:     compressorCurveFromProbe(param, transientShaperParseRole(candidate.Role)),
		Reachable: compressorReachableValues(param, transientShaperParseRole(candidate.Role))}
	if value, ok := parseCompressorPhysical(transientShaperParseRole(candidate.Role), param.ValueText); ok {
		binding.CurrentPhysical = &value
	}
	binding.PhysicalUnit = transientShaperPhysicalUnit(param, candidate.Role, binding.Domain)
	binding.TransactionalProbe = compressorBindingNeedsTransactionalProbe(param, binding)
	return binding
}

func transientShaperParseRole(role string) string {
	switch role {
	case "attack_duration", "sustain_duration", "duration":
		return "attack"
	case "transient_range", "attack_amount", "sustain_amount", "attack_sensitivity", "sustain_sensitivity", "detector_sensitivity":
		return "transient_amount"
	}
	return role
}

func transientShaperPhysicalUnit(param ParameterInfo, role string, domain *PluginDisplayDomain) string {
	if domain != nil && strings.TrimSpace(domain.Unit) != "" {
		return domain.Unit
	}
	if transientShaperRoleIn(role, "attack_amount", "sustain_amount", "attack_sensitivity", "sustain_sensitivity", "detector_sensitivity", "mix") &&
		domain != nil && domain.Min != nil && domain.Max != nil && *domain.Min >= -100 && *domain.Max <= 100 {
		return "%"
	}
	return compressorBindingPhysicalUnit(param, transientShaperParseRole(role), domain)
}

func transientShaperRoleIn(role string, values ...string) bool {
	for _, value := range values {
		if role == value {
			return true
		}
	}
	return false
}

func classifyTransientShaperStage(stage TransientShaperStage) (string, float64) {
	hasAttack := transientShaperHasRole(stage.EnvelopeAction, "attack_amount") && transientShaperRoleSigned(stage.EnvelopeAction, "attack_amount")
	hasSustain := transientShaperHasRole(stage.EnvelopeAction, "sustain_amount") && transientShaperRoleSigned(stage.EnvelopeAction, "sustain_amount")
	if hasAttack && hasSustain {
		return "paired_envelope_transient_shaper", 0.97
	}
	hasRange := transientShaperHasRole(stage.EnvelopeAction, "transient_range") && transientShaperRoleSigned(stage.EnvelopeAction, "transient_range")
	hasSense := transientShaperHasRole(stage.Detector, "detector_sensitivity")
	hasDuration := transientShaperHasRole(stage.Timing, "duration")
	hasRelease := transientShaperHasRole(stage.Timing, "release")
	if hasRange && hasSense && hasDuration && hasRelease {
		return "single_envelope_range_transient_shaper", 0.92
	}
	return "", 0
}

func transientShaperRoleSigned(bindings []CompressorBinding, role string) bool {
	for _, binding := range bindings {
		if binding.Role != role {
			continue
		}
		minimum, maximum := math.Inf(1), math.Inf(-1)
		for _, point := range binding.Curve {
			minimum, maximum = math.Min(minimum, point[1]), math.Max(maximum, point[1])
		}
		if binding.Domain != nil && binding.Domain.Min != nil && binding.Domain.Max != nil {
			minimum, maximum = math.Min(minimum, *binding.Domain.Min), math.Max(maximum, *binding.Domain.Max)
		}
		if minimum < 0 && maximum > 0 {
			return true
		}
	}
	return false
}

func transientShaperUnsupportedBoundary(digest ParameterDigest, candidates []transientShaperCandidate) string {
	if transientShaperHasMultibandSurface(digest) {
		return "unsupported_multiband_dynamics"
	}
	if compressor, boundary := DetectCompressorModelWithBoundary(digest); compressor != nil || boundary == "unsupported_multiband_compressor" {
		return mapTransientShaperAdjacentBoundary("unsupported_broadband_compressor", boundary)
	}
	if gate, boundary := DetectGateExpanderModelWithBoundary(digest); gate != nil {
		return "unsupported_gate_expander"
	} else if boundary == "unsupported_multiband_dynamics" {
		return boundary
	}
	if limiter, boundary := DetectLimiterModelWithBoundary(digest); limiter != nil {
		return "unsupported_limiter"
	} else if boundary == "unsupported_clipper" {
		return boundary
	}
	if deEsser, boundary := DetectDeEsserModelWithBoundary(digest); deEsser != nil {
		return "unsupported_de_esser"
	} else if boundary == "unsupported_spectral_dynamics" {
		return boundary
	}
	_ = candidates
	return ""
}

func mapTransientShaperAdjacentBoundary(fallback, boundary string) string {
	if boundary == "unsupported_multiband_compressor" {
		return "unsupported_multiband_dynamics"
	}
	return fallback
}

func transientShaperHasMultibandSurface(digest ParameterDigest) bool {
	amounts, bandTokens := 0, 0
	for _, param := range digest.Parameters {
		text := transientShaperParameterText(param)
		if transientShaperContains(text, "crossover", "cross over", "multiband") {
			return true
		}
		if transientShaperBandToken.MatchString(text) || transientShaperIndexedAmount.MatchString(text) {
			bandTokens++
		}
		if (transientShaperHasToken(text, "range") || transientShaperHasToken(text, "attack") || transientShaperHasToken(text, "sustain")) &&
			transientShaperSignedActionDomain(param) {
			amounts++
		}
	}
	return bandTokens >= 2 && amounts >= 2
}

func transientShaperCapabilities(stage TransientShaperStage) TransientShaperCapabilities {
	roles := func(bindings []CompressorBinding) []string {
		seen, out := map[string]bool{}, []string{}
		for _, binding := range bindings {
			if binding.Role != "" && !seen[binding.Role] {
				seen[binding.Role] = true
				out = append(out, binding.Role)
			}
		}
		sort.Strings(out)
		return out
	}
	return TransientShaperCapabilities{EnvelopeAction: roles(stage.EnvelopeAction), Detector: roles(stage.Detector),
		Timing: roles(stage.Timing), Shape: roles(stage.Shape), Mode: roles(stage.Mode), Output: roles(stage.Output)}
}

func transientShaperStageSummary(stage TransientShaperStage) map[string]any {
	return map[string]any{
		"stage_key":       stage.Key,
		"envelope_action": compressorBindingRows(stage.EnvelopeAction),
		"detector":        compressorBindingRows(stage.Detector),
		"timing":          compressorBindingRows(stage.Timing),
		"shape":           compressorBindingRows(stage.Shape),
		"mode":            compressorBindingRows(stage.Mode),
		"output":          compressorBindingRows(stage.Output),
		"capabilities": map[string]any{
			"envelope_action": stage.Capabilities.EnvelopeAction,
			"detector":        stage.Capabilities.Detector,
			"timing":          stage.Capabilities.Timing,
			"shape":           stage.Capabilities.Shape,
			"mode":            stage.Capabilities.Mode,
			"output":          stage.Capabilities.Output,
		},
	}
}

func transientShaperHasRole(bindings []CompressorBinding, role string) bool {
	for _, binding := range bindings {
		if binding.Role == role {
			return true
		}
	}
	return false
}

func transientShaperParameterText(param ParameterInfo) string {
	parts, seen := []string{}, map[string]bool{}
	for _, value := range []string{param.Name, param.RawName, param.Alias} {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value != "" && !seen[key] {
			parts, seen[key] = append(parts, value), true
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func transientShaperLabels(param ParameterInfo) []string {
	labels := []string{param.ValueText}
	if param.DisplayProbe == nil {
		return labels
	}
	labels = append(labels, param.DisplayProbe.AllLabels...)
	for _, row := range param.DisplayProbe.DiscreteLabels {
		labels = append(labels, row.Label)
	}
	for _, row := range param.DisplayProbe.Samples {
		labels = append(labels, row.Text)
	}
	return labels
}

func transientShaperLabelsContain(param ParameterInfo, needle string) bool {
	for _, label := range transientShaperLabels(param) {
		if strings.Contains(strings.ToLower(label), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func transientShaperHasToken(text, token string) bool {
	text, token = strings.ToLower(strings.TrimSpace(text)), strings.ToLower(strings.TrimSpace(token))
	if text == token {
		return true
	}
	for _, part := range strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '_' || r == '-' || r == '/' || r == '(' || r == ')'
	}) {
		if part == token {
			return true
		}
	}
	return false
}

func transientShaperContains(text string, values ...string) bool {
	text = strings.ToLower(text)
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func transientShaperTopologyGeneration(model *TransientShaperModel) string {
	type generationBinding struct{ ParamID, Role, Unit, Domain, Curve, Reachable string }
	type generationStage struct {
		Key            string              `json:"key"`
		EnvelopeAction []generationBinding `json:"envelope_action"`
		Detector       []generationBinding `json:"detector"`
		Timing         []generationBinding `json:"timing"`
		Shape          []generationBinding `json:"shape"`
		Mode           []generationBinding `json:"mode"`
		Output         []generationBinding `json:"output"`
	}
	collect := func(bindings []CompressorBinding) []generationBinding {
		out := []generationBinding{}
		for _, binding := range bindings {
			domain, _ := json.Marshal(binding.Domain)
			curve, _ := json.Marshal(binding.Curve)
			reachable, _ := json.Marshal(binding.Reachable)
			out = append(out, generationBinding{binding.ParamID, binding.Role, binding.PhysicalUnit, string(domain), string(curve), string(reachable)})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].ParamID == out[j].ParamID {
				return out[i].Role < out[j].Role
			}
			return out[i].ParamID < out[j].ParamID
		})
		return out
	}
	stage := generationStage{Key: model.Stage.Key, EnvelopeAction: collect(model.Stage.EnvelopeAction), Detector: collect(model.Stage.Detector),
		Timing: collect(model.Stage.Timing), Shape: collect(model.Stage.Shape), Mode: collect(model.Stage.Mode), Output: collect(model.Stage.Output)}
	payload := map[string]any{"schema": transientShaperTopologySchema, "semantic_mode": model.SemanticMode,
		"stage": stage, "auxiliary_stages": model.AuxiliaryStages}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return "t1t1_" + hex.EncodeToString(sum[:12])
}
