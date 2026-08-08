package plugingrabber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

const deEsserTopologySchema = "de-esser-control-topology/v1"

type DeEsserModel struct {
	Classification  string
	Confidence      float64
	Stages          []DeEsserStage
	AuxiliaryStages []DeEsserAuxiliaryStage
	Generation      string
}

type DeEsserStage struct {
	Key                  string
	Scope                string
	OperatingPoint       []CompressorBinding
	FrequencySelectivity []CompressorBinding
	Timing               []CompressorBinding
	Mode                 []CompressorBinding
	Output               []CompressorBinding
	Capabilities         DeEsserCapabilities
}

type DeEsserCapabilities struct {
	OperatingPoint       []string
	FrequencySelectivity []string
	Timing               []string
	Mode                 []string
	Output               []string
}

type DeEsserAuxiliaryStage struct {
	Kind     string
	ParamIDs []string
}

type deEsserCandidate struct {
	Param         ParameterInfo
	Role          string
	Section       string
	Index         string
	Explicit      bool
	AuxiliaryKind string
}

var deEsserIndexedSuffix = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])([0-9]+)\s*$`)

// DetectDeEsserModelWithBoundary recovers a frequency-selective sibilance
// reduction stage from observable parameter structure only. Product identity,
// vendor, current values, profiles, and generic normalized roles are ignored.
func DetectDeEsserModelWithBoundary(digest ParameterDigest) (*DeEsserModel, string) {
	candidates := make([]deEsserCandidate, 0, len(digest.Parameters))
	aux := map[string][]string{}
	for _, param := range digest.Parameters {
		candidate := classifyDeEsserParameter(param)
		if candidate.AuxiliaryKind != "" {
			aux[candidate.AuxiliaryKind] = append(aux[candidate.AuxiliaryKind], param.ID)
		}
		if candidate.Role != "" && param.HostControllable {
			candidates = append(candidates, candidate)
		}
	}
	boundary := deEsserUnsupportedBoundary(digest, candidates, aux)
	if boundary != "" {
		return nil, boundary
	}
	stage := buildDeEsserStage(candidates)
	if !deEsserStageIsProvable(stage, candidates) {
		return nil, deEsserUnresolvedBoundary(digest, candidates)
	}
	model := &DeEsserModel{Stages: []DeEsserStage{stage}}
	for kind, ids := range aux {
		sort.Strings(ids)
		model.AuxiliaryStages = append(model.AuxiliaryStages, DeEsserAuxiliaryStage{Kind: kind, ParamIDs: ids})
	}
	sort.Slice(model.AuxiliaryStages, func(i, j int) bool { return model.AuxiliaryStages[i].Kind < model.AuxiliaryStages[j].Kind })
	model.Classification = classifyDeEsserModel(stage)
	model.Confidence = deEsserModelConfidence(stage)
	model.Generation = deEsserTopologyGeneration(model)
	return model, ""
}

func DetectDeEsserModel(digest ParameterDigest) *DeEsserModel {
	model, _ := DetectDeEsserModelWithBoundary(digest)
	return model
}

func BuildDeEsserSummaryWithBoundary(digest ParameterDigest) (map[string]any, string) {
	model, boundary := DetectDeEsserModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	stages := make([]map[string]any, 0, len(model.Stages))
	for _, stage := range model.Stages {
		stages = append(stages, deEsserStageSummary(stage))
	}
	aux := make([]map[string]any, 0, len(model.AuxiliaryStages))
	for _, stage := range model.AuxiliaryStages {
		aux = append(aux, map[string]any{"kind": stage.Kind, "param_ids": stage.ParamIDs})
	}
	return map[string]any{
		"schema_version": deEsserTopologySchema,
		"mapping_source": "generic_structural",
		"classification": model.Classification,
		"confidence":     model.Confidence,
		"control_topology": map[string]any{
			"schema_version": deEsserTopologySchema,
			"generation":     model.Generation,
		},
		"de_esser_stages":  stages,
		"auxiliary_stages": aux,
	}, ""
}

func BuildDeEsserSummary(digest ParameterDigest) map[string]any {
	summary, _ := BuildDeEsserSummaryWithBoundary(digest)
	return summary
}

func classifyDeEsserParameter(param ParameterInfo) deEsserCandidate {
	text := deEsserParameterText(param)
	candidate := deEsserCandidate{Param: param, Index: deEsserParameterIndex(text)}
	if deEsserContains(text, "de-esser", "de esser", "deesser", "s-reduction", "s reduction", "sibilance") {
		candidate.Explicit = true
	}
	if deEsserContains(text, "multiband", "cross over", "crossover") {
		candidate.AuxiliaryKind = "multiband_dynamics"
		return candidate
	}
	if deEsserContains(text, "clipper", "clipping", "clip ceiling", "clip threshold") {
		candidate.AuxiliaryKind = "clipper"
		return candidate
	}
	if deEsserContains(text, "gate", "expander", "expansion", "hysteresis") {
		candidate.AuxiliaryKind = "gate_expander"
		return candidate
	}
	if deEsserContains(text, "ceiling", "true peak", "truepeak") {
		candidate.AuxiliaryKind = "limiter"
		return candidate
	}
	if deEsserContains(text, "capture", "learn", "freeze") && candidate.Index != "" {
		candidate.AuxiliaryKind = "spectral_dynamics"
		return candidate
	}

	switch {
	case deEsserHasToken(text, "threshold") || deEsserHasToken(text, "thresh"):
		candidate.Role, candidate.Section = "threshold", "operating_point"
	case deEsserHasToken(text, "range") || deEsserContains(text, "reduction", "amount", "sensitivity"):
		candidate.Role, candidate.Section = "reduction_range", "operating_point"
	case deEsserContains(text, "detection", "detector"):
		candidate.Role, candidate.Section = "detection_amount", "operating_point"
	case deEsserContains(text, "frequency", "freq", "high-pass", "high pass", "low-pass", "low pass"):
		candidate.Role, candidate.Section = "focus_frequency", "frequency_selectivity"
	case deEsserContains(text, "sidechain", "side chain", "filtertype", "filter type"):
		candidate.Role, candidate.Section = "detector_filter", "frequency_selectivity"
	case deEsserContains(text, "split", "wideband", "wide band"):
		candidate.Role, candidate.Section = "split_wide", "mode"
	case deEsserContains(text, "monitor", "listen", "audition"):
		candidate.Role, candidate.Section = "monitor", "mode"
	case deEsserHasToken(text, "lookahead") || deEsserContains(text, "look ahead"):
		candidate.Role, candidate.Section = "lookahead", "timing"
	case deEsserHasToken(text, "attack"):
		candidate.Role, candidate.Section = "attack", "timing"
	case deEsserHasToken(text, "release") || deEsserHasToken(text, "recovery"):
		candidate.Role, candidate.Section = "release", "timing"
	case deEsserHasToken(text, "mode") || deEsserHasToken(text, "style"):
		candidate.Role, candidate.Section = "mode", "mode"
	case deEsserContains(text, "stereo link", "channel link") || deEsserHasToken(text, "link"):
		candidate.Role, candidate.Section = "channel_link", "output"
	case deEsserHasToken(text, "mix") || deEsserContains(text, "dry/wet", "dry wet", "wet level"):
		candidate.Role, candidate.Section = "mix", "output"
	case deEsserContains(text, "output level", "output gain", "out level", "out gain"):
		candidate.Role, candidate.Section = "output_gain", "output"
	}
	return candidate
}

func deEsserParameterText(param ParameterInfo) string {
	parts := []string{}
	seen := map[string]bool{}
	for _, value := range []string{param.Name, param.RawName, param.Alias} {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value != "" && !seen[key] {
			parts = append(parts, value)
			seen[key] = true
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func deEsserParameterIndex(text string) string {
	match := deEsserIndexedSuffix.FindStringSubmatch(strings.TrimSpace(text))
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func buildDeEsserStage(candidates []deEsserCandidate) DeEsserStage {
	stage := DeEsserStage{Key: "de_esser_main", Scope: "main"}
	for _, candidate := range candidates {
		binding := deEsserBinding(candidate, stage.Key)
		switch candidate.Section {
		case "operating_point":
			stage.OperatingPoint = append(stage.OperatingPoint, binding)
		case "frequency_selectivity":
			stage.FrequencySelectivity = append(stage.FrequencySelectivity, binding)
		case "timing":
			stage.Timing = append(stage.Timing, binding)
		case "mode":
			stage.Mode = append(stage.Mode, binding)
		case "output":
			stage.Output = append(stage.Output, binding)
		}
	}
	for _, bindings := range []*[]CompressorBinding{&stage.OperatingPoint, &stage.FrequencySelectivity, &stage.Timing, &stage.Mode, &stage.Output} {
		sortCompressorBindings(bindings)
	}
	stage.Capabilities = deEsserCapabilities(stage)
	return stage
}

func deEsserBinding(candidate deEsserCandidate, stageKey string) CompressorBinding {
	param := candidate.Param
	binding := CompressorBinding{ParamID: param.ID, Name: parameterDisplayName(param), Role: candidate.Role,
		Channel: "shared", CompressionStage: stageKey, CurrentText: param.ValueText,
		CurrentNormalized: anyFloat(param.NormalizedValue), Domain: param.DisplayDomainCandidate,
		Curve: compressorCurveFromProbe(param, candidate.Role), Reachable: compressorReachableValues(param, candidate.Role)}
	if value, ok := parseCompressorPhysical(candidate.Role, param.ValueText); ok {
		binding.CurrentPhysical = &value
	}
	binding.PhysicalUnit = compressorBindingPhysicalUnit(param, candidate.Role, binding.Domain)
	binding.TransactionalProbe = compressorBindingNeedsTransactionalProbe(param, binding)
	return binding
}

func deEsserCapabilities(stage DeEsserStage) DeEsserCapabilities {
	roles := func(bindings []CompressorBinding) []string {
		seen := map[string]bool{}
		out := []string{}
		for _, binding := range bindings {
			if binding.Role != "" && !seen[binding.Role] {
				seen[binding.Role] = true
				out = append(out, binding.Role)
			}
		}
		sort.Strings(out)
		return out
	}
	return DeEsserCapabilities{OperatingPoint: roles(stage.OperatingPoint), FrequencySelectivity: roles(stage.FrequencySelectivity),
		Timing: roles(stage.Timing), Mode: roles(stage.Mode), Output: roles(stage.Output)}
}

func deEsserStageSummary(stage DeEsserStage) map[string]any {
	return map[string]any{
		"stage_key": stage.Key, "scope": stage.Scope,
		"operating_point":       compressorBindingRows(stage.OperatingPoint),
		"frequency_selectivity": compressorBindingRows(stage.FrequencySelectivity),
		"timing":                compressorBindingRows(stage.Timing), "mode": compressorBindingRows(stage.Mode),
		"output": compressorBindingRows(stage.Output),
		"capabilities": map[string]any{
			"operating_point":       stage.Capabilities.OperatingPoint,
			"frequency_selectivity": stage.Capabilities.FrequencySelectivity,
			"timing":                stage.Capabilities.Timing, "mode": stage.Capabilities.Mode,
			"output": stage.Capabilities.Output,
		},
	}
}

func deEsserStageIsProvable(stage DeEsserStage, candidates []deEsserCandidate) bool {
	hasThreshold := deEsserHasRole(stage.OperatingPoint, "threshold")
	hasReduction := deEsserHasRole(stage.OperatingPoint, "reduction_range") || deEsserHasRole(stage.OperatingPoint, "detection_amount")
	hasFrequency := deEsserHasRole(stage.FrequencySelectivity, "focus_frequency")
	hasFilterMode := deEsserHasRole(stage.FrequencySelectivity, "detector_filter") || deEsserHasRole(stage.Mode, "split_wide")
	hasSibilanceMode := hasReduction && deEsserHasRole(stage.Mode, "mode") && deEsserHasRole(stage.Mode, "monitor")
	hasExplicit := false
	for _, candidate := range candidates {
		hasExplicit = hasExplicit || candidate.Explicit
	}
	if hasExplicit && hasThreshold && (hasReduction || hasFrequency || hasFilterMode) {
		return true
	}
	if hasThreshold && hasFrequency && hasFilterMode {
		return true
	}
	if hasThreshold && hasReduction && (hasFrequency || hasFilterMode || hasSibilanceMode) {
		return true
	}
	return false
}

func deEsserUnsupportedBoundary(digest ParameterDigest, candidates []deEsserCandidate, aux map[string][]string) string {
	if len(aux["clipper"]) > 0 {
		return "unsupported_clipper"
	}
	if deEsserHasSpectralSurface(digest) {
		return "unsupported_spectral_dynamics"
	}
	if deEsserHasQWithoutSibilanceMode(digest, candidates) {
		return "unsupported_spectral_dynamics"
	}
	if deEsserHasMultipleIndexedDynamics(candidates) {
		return "unsupported_multiband_dynamics"
	}
	if len(aux["multiband_dynamics"]) > 0 {
		return "unsupported_multiband_dynamics"
	}
	if len(aux["gate_expander"]) > 0 {
		return "unsupported_gate_expander"
	}
	if len(aux["limiter"]) > 0 {
		return "unsupported_limiter"
	}
	if compressor, _ := DetectCompressorModelWithBoundary(digest); compressor != nil && !deEsserHasStrongSpecialization(candidates) {
		return "unsupported_broadband_compressor"
	}
	return ""
}

func deEsserHasStrongSpecialization(candidates []deEsserCandidate) bool {
	hasThreshold, hasRange := false, false
	hasDetection, hasFocus, hasFilter := false, false, false
	hasMode, hasMonitor, hasBroadbandTiming := false, false, false
	for _, candidate := range candidates {
		switch candidate.Role {
		case "threshold":
			hasThreshold = true
		case "reduction_range":
			hasRange = true
		case "detection_amount":
			hasDetection = true
		case "focus_frequency":
			hasFocus = true
		case "detector_filter":
			hasFilter = true
		case "mode":
			hasMode = true
		case "monitor":
			hasMonitor = true
		case "attack", "release":
			hasBroadbandTiming = true
		}
	}
	frequencySelective := hasFocus && (hasFilter || hasMonitor) && !hasBroadbandTiming
	sibilanceDetector := hasDetection && hasMode && hasMonitor
	return hasThreshold && hasRange && (frequencySelective || sibilanceDetector)
}

func deEsserUnresolvedBoundary(digest ParameterDigest, candidates []deEsserCandidate) string {
	if deEsserHasQWithoutSibilanceMode(digest, candidates) {
		return "unsupported_spectral_dynamics"
	}
	if len(candidates) > 0 {
		return "unresolved_de_esser_surface"
	}
	return "not_de_esser"
}

func deEsserHasMultipleIndexedDynamics(candidates []deEsserCandidate) bool {
	type facts struct{ op, freq, timing bool }
	bands := map[string]facts{}
	for _, candidate := range candidates {
		if candidate.Index == "" {
			continue
		}
		item := bands[candidate.Index]
		switch candidate.Section {
		case "operating_point":
			item.op = true
		case "frequency_selectivity":
			item.freq = true
		case "timing":
			item.timing = true
		}
		bands[candidate.Index] = item
	}
	complete := 0
	for _, item := range bands {
		if item.op && (item.freq || item.timing) {
			complete++
		}
	}
	return complete >= 2
}

func deEsserHasSpectralSurface(digest ParameterDigest) bool {
	thresholds, freqs, qs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, param := range digest.Parameters {
		text := deEsserParameterText(param)
		match := deEsserIndexedSuffix.FindStringSubmatch(strings.TrimSpace(text))
		if len(match) != 2 {
			continue
		}
		index := match[1]
		if deEsserHasToken(text, "threshold") {
			thresholds[index] = true
		}
		if deEsserContains(text, "frequency", "freq") {
			freqs[index] = true
		}
		if deEsserHasToken(text, "q") {
			qs[index] = true
		}
	}
	intersections := 0
	for index := range thresholds {
		if freqs[index] && qs[index] {
			intersections++
		}
	}
	return intersections >= 2
}

func deEsserHasQWithoutSibilanceMode(digest ParameterDigest, candidates []deEsserCandidate) bool {
	hasQ := false
	hasMode := false
	for _, param := range digest.Parameters {
		text := deEsserParameterText(param)
		hasQ = hasQ || deEsserHasToken(text, "q")
	}
	for _, candidate := range candidates {
		hasMode = hasMode || candidate.Role == "split_wide" || candidate.Role == "detector_filter" || candidate.Role == "monitor" || candidate.Explicit
	}
	return hasQ && !hasMode
}

func classifyDeEsserModel(stage DeEsserStage) string {
	if deEsserHasRole(stage.FrequencySelectivity, "focus_frequency") || deEsserHasRole(stage.FrequencySelectivity, "detector_filter") {
		if deEsserHasRole(stage.Mode, "split_wide") && deEsserHasRole(stage.FrequencySelectivity, "focus_frequency") {
			return "split_wide_de_esser"
		}
		return "frequency_selective_de_esser"
	}
	if deEsserHasRole(stage.OperatingPoint, "reduction_range") && deEsserHasRole(stage.Mode, "mode") && deEsserHasRole(stage.Mode, "monitor") {
		return "sibilance_detector_de_esser"
	}
	return "frequency_selective_de_esser"
}

func deEsserModelConfidence(stage DeEsserStage) float64 {
	score := len(stage.OperatingPoint)*3 + len(stage.FrequencySelectivity)*2 + len(stage.Mode) + len(stage.Timing)
	if score >= 10 {
		return 0.96
	}
	if score >= 7 {
		return 0.92
	}
	return 0.86
}

func deEsserHasRole(bindings []CompressorBinding, role string) bool {
	for _, binding := range bindings {
		if binding.Role == role {
			return true
		}
	}
	return false
}

func deEsserHasToken(text, token string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	token = strings.ToLower(strings.TrimSpace(token))
	if text == token {
		return true
	}
	for _, part := range strings.FieldsFunc(text, func(r rune) bool { return r == ' ' || r == '_' || r == '-' || r == '/' || r == '(' || r == ')' }) {
		if part == token {
			return true
		}
	}
	return false
}

func deEsserContains(text string, values ...string) bool {
	text = strings.ToLower(text)
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func deEsserTopologyGeneration(model *DeEsserModel) string {
	type generationBinding struct {
		ParamID, Role, Unit, Domain, Curve, Reachable string
	}
	type generationStage struct {
		Key                  string              `json:"key"`
		OperatingPoint       []generationBinding `json:"operating_point"`
		FrequencySelectivity []generationBinding `json:"frequency_selectivity"`
		Timing               []generationBinding `json:"timing"`
		Mode                 []generationBinding `json:"mode"`
		Output               []generationBinding `json:"output"`
	}
	stages := []generationStage{}
	for _, stage := range model.Stages {
		row := generationStage{Key: stage.Key}
		collect := func(bindings []CompressorBinding) []generationBinding {
			out := []generationBinding{}
			for _, binding := range bindings {
				curve, _ := json.Marshal(binding.Curve)
				reachable, _ := json.Marshal(binding.Reachable)
				domain, _ := json.Marshal(binding.Domain)
				out = append(out, generationBinding{ParamID: binding.ParamID, Role: binding.Role, Unit: binding.PhysicalUnit,
					Domain: string(domain), Curve: string(curve), Reachable: string(reachable)})
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].ParamID == out[j].ParamID {
					return out[i].Role < out[j].Role
				}
				return out[i].ParamID < out[j].ParamID
			})
			return out
		}
		row.OperatingPoint = collect(stage.OperatingPoint)
		row.FrequencySelectivity = collect(stage.FrequencySelectivity)
		row.Timing = collect(stage.Timing)
		row.Mode = collect(stage.Mode)
		row.Output = collect(stage.Output)
		stages = append(stages, row)
	}
	payload := map[string]any{"schema": deEsserTopologySchema, "stages": stages, "auxiliary_stages": model.AuxiliaryStages}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return "d1t1_" + hex.EncodeToString(sum[:12])
}
