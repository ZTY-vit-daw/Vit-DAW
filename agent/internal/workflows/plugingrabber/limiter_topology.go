package plugingrabber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

const limiterTopologySchema = "limiter-control-topology/v1"

type LimiterModel struct {
	Classification  string
	Confidence      float64
	Stages          []LimiterStage
	SharedControls  []LimiterBinding
	AuxiliaryStages []LimiterAuxiliaryStage
	Generation      string
}

type LimiterStage struct {
	Key            string
	Scope          string
	OperatingPoint []LimiterBinding
	Safety         []LimiterBinding
	Timing         []LimiterBinding
	Detector       []LimiterBinding
	Mode           []LimiterBinding
	Output         []LimiterBinding
	Capabilities   LimiterCapabilities
}

type LimiterBinding = CompressorBinding

type LimiterCapabilities struct {
	OperatingPoint []string
	Safety         []string
	Timing         []string
	Detector       []string
	Mode           []string
	Output         []string
}

type LimiterAuxiliaryStage struct {
	Kind     string
	ParamIDs []string
}

type limiterCandidate struct {
	Param            ParameterInfo
	Role             string
	Section          string
	Index            string
	Band             string
	ExplicitLimiter  bool
	AmbiguousDrive   bool
	AmbiguousOutput  bool
	AuxiliaryKind    string
	EvidenceStrength int
}

var limiterIndexedSuffix = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])([0-9]+)\s*$`)
var limiterBandIndex = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:band|b)\s*([0-9]+)(?:[^a-z0-9]|$)`)

// DetectLimiterModelWithBoundary recovers independently provable limiter
// stages from the live parameter surface. Product identity and current values
// are intentionally excluded from both recognition and generation.
func DetectLimiterModelWithBoundary(digest ParameterDigest) (*LimiterModel, string) {
	candidates := make([]limiterCandidate, 0, len(digest.Parameters))
	auxByKind := map[string][]string{}
	explicitIndexes := map[string]bool{}
	for _, param := range digest.Parameters {
		candidate := classifyLimiterParameter(param)
		if candidate.AuxiliaryKind != "" {
			auxByKind[candidate.AuxiliaryKind] = append(auxByKind[candidate.AuxiliaryKind], param.ID)
		}
		if candidate.ExplicitLimiter && candidate.Index != "" {
			explicitIndexes[candidate.Index] = true
		}
		if candidate.Role != "" && param.HostControllable {
			candidates = append(candidates, candidate)
		}
	}

	stages, shared := buildLimiterStages(candidates, explicitIndexes)
	if len(stages) == 0 {
		return nil, limiterUnsupportedBoundary(digest, candidates, auxByKind)
	}

	model := &LimiterModel{Stages: stages, SharedControls: shared}
	for kind, ids := range auxByKind {
		sort.Strings(ids)
		model.AuxiliaryStages = append(model.AuxiliaryStages, LimiterAuxiliaryStage{Kind: kind, ParamIDs: ids})
	}
	if compressor, _ := DetectCompressorModelWithBoundary(digest); compressor != nil {
		model.AuxiliaryStages = append(model.AuxiliaryStages, LimiterAuxiliaryStage{Kind: "broadband_compressor"})
	}
	sort.Slice(model.AuxiliaryStages, func(i, j int) bool { return model.AuxiliaryStages[i].Kind < model.AuxiliaryStages[j].Kind })
	model.Classification = classifyLimiterModel(model)
	model.Confidence = limiterModelConfidence(model)
	model.Generation = limiterTopologyGeneration(model)
	return model, ""
}

func DetectLimiterModel(digest ParameterDigest) *LimiterModel {
	model, _ := DetectLimiterModelWithBoundary(digest)
	return model
}

func BuildLimiterSummaryWithBoundary(digest ParameterDigest) (map[string]any, string) {
	model, boundary := DetectLimiterModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	stages := make([]map[string]any, 0, len(model.Stages))
	for _, stage := range model.Stages {
		stages = append(stages, limiterStageSummary(stage))
	}
	aux := make([]map[string]any, 0, len(model.AuxiliaryStages))
	for _, stage := range model.AuxiliaryStages {
		aux = append(aux, map[string]any{"kind": stage.Kind, "param_ids": stage.ParamIDs})
	}
	return map[string]any{
		"schema_version": limiterTopologySchema,
		"mapping_source": "generic_structural",
		"classification": model.Classification,
		"confidence":     model.Confidence,
		"control_topology": map[string]any{
			"schema_version": limiterTopologySchema,
			"generation":     model.Generation,
		},
		"limiter_stages":   stages,
		"shared_controls":  compressorBindingRows(model.SharedControls),
		"auxiliary_stages": aux,
	}, ""
}

func BuildLimiterSummary(digest ParameterDigest) map[string]any {
	summary, _ := BuildLimiterSummaryWithBoundary(digest)
	return summary
}

func classifyLimiterParameter(param ParameterInfo) limiterCandidate {
	text := limiterParameterText(param)
	candidate := limiterCandidate{Param: param, Index: limiterParameterIndex(text), Band: limiterFrequencyBandKey(text)}
	candidate.ExplicitLimiter = limiterContains(text, "limiter", "limiting", "limit stage", "limit mode", "limit bypass", "limit enable")

	if limiterContains(text, "dither", "quantize", "noise shaping", "bit depth", "idr") {
		return candidate
	}
	if limiterContains(text, "clipper", "clipping", "clip threshold", "clip ceiling") {
		candidate.AuxiliaryKind = "clipper"
		return candidate
	}
	if limiterContains(text, "gate", "expander", "expansion", "hysteresis") {
		candidate.AuxiliaryKind = "gate_expander"
		return candidate
	}
	if candidate.Band != "" || limiterContains(text, "crossover", "cross over") {
		candidate.AuxiliaryKind = "multiband_dynamics"
		return candidate
	}
	if limiterContains(text, "de-esser", "de esser", "deesser", "s-reduction", "s reduction") {
		candidate.AuxiliaryKind = "de_esser"
		return candidate
	}
	if limiterContains(text, "sidechain", "side chain", "sc filter", "sc hpf", "sc hp", "sc-hp") {
		return candidate
	}

	switch {
	case limiterContains(text, "true peak", "truepeak"):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "true_peak", "detector", 2
	case limiterContains(text, "channel link", "stereo link") || limiterHasToken(text, "link"):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "channel_link", "detector", 1
	case limiterContains(text, "out ceiling", "output ceiling", "limit ceiling", "true peak ceiling", "max output") || limiterHasToken(text, "ceiling") || limiterOutputCeilingEvidence(param, text):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "ceiling", "safety", 3
	case limiterHasToken(text, "threshold") || limiterHasToken(text, "thresh"):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "threshold", "operating_point", 3
	case limiterInputDriveName(text):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "input_drive", "operating_point", 2
	case strings.TrimSpace(text) == "gain":
		candidate.Role, candidate.Section, candidate.AmbiguousDrive, candidate.EvidenceStrength = "input_drive", "operating_point", true, 1
	case limiterContains(text, "lookahead", "look ahead"):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "lookahead", "timing", 3
	case limiterContains(text, "auto release", "adaptive release") || limiterHasToken(text, "arc"):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "auto_release", "mode", 1
	case limiterHasToken(text, "release") || limiterHasToken(text, "recovery"):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "release", "timing", 3
	case limiterHasToken(text, "attack"):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "attack", "timing", 1
	case limiterHasToken(text, "ratio") || limiterHasToken(text, "knee") || limiterHasToken(text, "compression"):
		// Transfer evidence is retained for limiter/compressor boundary
		// decisions but is never exposed as a limiter control binding.
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "compressor_transfer", "auxiliary", 2
	case limiterContains(text, "oversampling", "over sampling"):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "oversampling", "mode", 1
	case candidate.ExplicitLimiter && (limiterHasToken(text, "mode") || limiterHasToken(text, "style") || limiterHasToken(text, "algorithm")):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "limiter_mode", "mode", 2
	case candidate.ExplicitLimiter && (limiterHasToken(text, "bypass") || limiterHasToken(text, "enable") || limiterHasToken(text, "active") || strings.TrimSpace(text) == "limiter"):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "stage_enable", "mode", 2
	case limiterContains(text, "limiter mix"):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "mix", "output", 1
	case limiterOutputName(text):
		candidate.Role, candidate.Section, candidate.AmbiguousOutput, candidate.EvidenceStrength = "output_gain", "output", true, 1
	case limiterMixName(text):
		candidate.Role, candidate.Section, candidate.EvidenceStrength = "mix", "output", 1
	}
	return candidate
}

func limiterParameterText(param ParameterInfo) string {
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

func limiterParameterIndex(text string) string {
	match := limiterIndexedSuffix.FindStringSubmatch(strings.TrimSpace(text))
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func limiterFrequencyBandKey(text string) string {
	if limiterContains(text, "sidechain", "side chain", "sc ") {
		return ""
	}
	if match := limiterBandIndex.FindStringSubmatch(text); len(match) == 2 {
		return "band_" + match[1]
	}
	for _, token := range []string{"low", "mid", "high"} {
		if limiterHasToken(text, token) && (limiterHasToken(text, "threshold") || limiterHasToken(text, "ratio") ||
			limiterHasToken(text, "attack") || limiterHasToken(text, "release") || limiterHasToken(text, "gain")) {
			return token
		}
	}
	return ""
}

func buildLimiterStages(candidates []limiterCandidate, explicitIndexes map[string]bool) ([]LimiterStage, []LimiterBinding) {
	byScope := map[string][]limiterCandidate{}
	sharedCandidates := []limiterCandidate{}
	hasIndexedCore := false
	for _, candidate := range candidates {
		if candidate.Index != "" && candidate.AuxiliaryKind == "" {
			hasIndexedCore = true
		}
	}
	for _, candidate := range candidates {
		if candidate.AuxiliaryKind != "" {
			continue
		}
		if hasIndexedCore && candidate.Index == "" {
			sharedCandidates = append(sharedCandidates, candidate)
			continue
		}
		scope := "main"
		if candidate.Index != "" {
			scope = "indexed_" + candidate.Index
		}
		byScope[scope] = append(byScope[scope], candidate)
	}

	stages := []LimiterStage{}
	for scope, rows := range byScope {
		explicit := false
		allowAmbiguousDrive := false
		if strings.HasPrefix(scope, "indexed_") {
			explicit = explicitIndexes[strings.TrimPrefix(scope, "indexed_")]
		}
		for _, row := range rows {
			explicit = explicit || row.ExplicitLimiter
			// A uniquely named output ceiling, including a measured peak-unit
			// output such as dBTP, is sufficient to disambiguate an unqualified
			// drive control within the same timed stage. It does not erase
			// compressor-transfer evidence below.
			allowAmbiguousDrive = allowAmbiguousDrive || row.Role == "ceiling"
		}
		stage := buildLimiterStage(scope, rows, explicit, allowAmbiguousDrive)
		if limiterStageIsProvable(stage, rows, explicit) {
			stages = append(stages, stage)
		}
	}
	sort.Slice(stages, func(i, j int) bool { return stages[i].Key < stages[j].Key })

	shared := make([]LimiterBinding, 0, len(sharedCandidates))
	for _, candidate := range sharedCandidates {
		if candidate.Section == "detector" || candidate.Section == "mode" || candidate.Section == "output" {
			shared = append(shared, limiterBinding(candidate, "shared"))
		}
	}
	sortCompressorBindings(&shared)
	return stages, shared
}

func buildLimiterStage(scope string, rows []limiterCandidate, explicit, allowAmbiguousDrive bool) LimiterStage {
	stage := LimiterStage{Key: "limiter_" + scope, Scope: scope}
	hasExplicitCeiling := false
	promotedCeilingKey := ""
	for _, row := range rows {
		hasExplicitCeiling = hasExplicitCeiling || row.Role == "ceiling"
	}
	if explicit && !hasExplicitCeiling {
		for _, row := range rows {
			if row.AmbiguousOutput && limiterContains(limiterParameterText(row.Param), "output level", "out level") {
				promotedCeilingKey = limiterCandidateKey(row)
				break
			}
		}
		if promotedCeilingKey == "" {
			for _, row := range rows {
				if row.AmbiguousOutput {
					promotedCeilingKey = limiterCandidateKey(row)
					break
				}
			}
		}
	}
	for _, row := range rows {
		if row.AmbiguousDrive && !allowAmbiguousDrive {
			continue
		}
		if row.AmbiguousOutput && limiterCandidateKey(row) == promotedCeilingKey {
			row.Role, row.Section = "ceiling", "safety"
		}
		binding := limiterBinding(row, stage.Key)
		switch row.Section {
		case "operating_point":
			stage.OperatingPoint = append(stage.OperatingPoint, binding)
		case "safety":
			stage.Safety = append(stage.Safety, binding)
		case "timing":
			stage.Timing = append(stage.Timing, binding)
		case "detector":
			stage.Detector = append(stage.Detector, binding)
		case "mode":
			stage.Mode = append(stage.Mode, binding)
		case "output":
			stage.Output = append(stage.Output, binding)
		}
	}
	for _, bindings := range []*[]LimiterBinding{&stage.OperatingPoint, &stage.Safety, &stage.Timing, &stage.Detector, &stage.Mode, &stage.Output} {
		sortCompressorBindings(bindings)
	}
	stage.Capabilities = limiterCapabilities(stage)
	return stage
}

func limiterCandidateKey(candidate limiterCandidate) string {
	return candidate.Param.ID + "\x00" + candidate.Param.Name + "\x00" + candidate.Param.RawName
}

func limiterOutputCeilingEvidence(param ParameterInfo, text string) bool {
	if !limiterContains(text, "output level", "out level", "output gain", "out gain") {
		return false
	}
	value := strings.ToLower(strings.TrimSpace(param.ValueText))
	return limiterContains(value, "dbtp", "true peak", "dbfs peak", "peak db")
}

func limiterStageIsProvable(stage LimiterStage, rows []limiterCandidate, explicit bool) bool {
	hasOperatingPoint := limiterBindingsHaveRole(stage.OperatingPoint, "threshold") || limiterBindingsHaveRole(stage.OperatingPoint, "input_drive")
	hasCeiling := limiterBindingsHaveRole(stage.Safety, "ceiling")
	hasTime := limiterBindingsHaveRole(stage.Timing, "release") || limiterBindingsHaveRole(stage.Timing, "lookahead")
	if !hasOperatingPoint || !hasCeiling || !hasTime {
		return false
	}
	hasCompressorTransfer := false
	hasGateEvidence := false
	for _, row := range rows {
		text := limiterParameterText(row.Param)
		hasCompressorTransfer = hasCompressorTransfer || limiterHasToken(text, "ratio") || limiterHasToken(text, "knee")
		hasGateEvidence = hasGateEvidence || limiterContains(text, "gate", "expander", "hysteresis")
	}
	if hasGateEvidence {
		return false
	}
	if hasCompressorTransfer && !explicit {
		return false
	}
	return true
}

func limiterBinding(candidate limiterCandidate, stageKey string) LimiterBinding {
	param := candidate.Param
	binding := LimiterBinding{ParamID: param.ID, Name: parameterDisplayName(param), Role: candidate.Role,
		Channel: "shared", CompressionStage: stageKey, CurrentText: param.ValueText, CurrentNormalized: anyFloat(param.NormalizedValue),
		Domain: param.DisplayDomainCandidate, Curve: compressorCurveFromProbe(param, candidate.Role), Reachable: compressorReachableValues(param, candidate.Role)}
	if len(binding.Curve) == 0 && compressorProbeHasNonFiniteSentinel(param.DisplayProbe) {
		binding.Domain = nil
	}
	if value, ok := parseCompressorPhysical(candidate.Role, param.ValueText); ok {
		binding.CurrentPhysical = &value
	}
	binding.PhysicalUnit = compressorBindingPhysicalUnit(param, candidate.Role, binding.Domain)
	binding.TransactionalProbe = compressorBindingNeedsTransactionalProbe(param, binding)
	return binding
}

func limiterStageSummary(stage LimiterStage) map[string]any {
	return map[string]any{
		"stage_key":       stage.Key,
		"scope":           stage.Scope,
		"operating_point": compressorBindingRows(stage.OperatingPoint),
		"safety":          compressorBindingRows(stage.Safety),
		"timing":          compressorBindingRows(stage.Timing),
		"detector":        compressorBindingRows(stage.Detector),
		"mode":            compressorBindingRows(stage.Mode),
		"output":          compressorBindingRows(stage.Output),
		"capabilities":    limiterCapabilitiesMap(stage.Capabilities),
	}
}

func limiterCapabilities(stage LimiterStage) LimiterCapabilities {
	return LimiterCapabilities{
		OperatingPoint: limiterBindingRoles(stage.OperatingPoint), Safety: limiterBindingRoles(stage.Safety),
		Timing: limiterBindingRoles(stage.Timing), Detector: limiterBindingRoles(stage.Detector),
		Mode: limiterBindingRoles(stage.Mode), Output: limiterBindingRoles(stage.Output),
	}
}

func limiterCapabilitiesMap(capabilities LimiterCapabilities) map[string]any {
	return map[string]any{"operating_point": capabilities.OperatingPoint, "safety": capabilities.Safety,
		"timing": capabilities.Timing, "detector": capabilities.Detector, "mode": capabilities.Mode, "output": capabilities.Output}
}

func limiterBindingRoles(bindings []LimiterBinding) []string {
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

func limiterBindingsHaveRole(bindings []LimiterBinding, role string) bool {
	for _, binding := range bindings {
		if binding.Role == role {
			return true
		}
	}
	return false
}

func classifyLimiterModel(model *LimiterModel) string {
	if len(model.Stages) > 1 {
		return "indexed_multi_stage_limiter"
	}
	stage := model.Stages[0]
	if limiterBindingsHaveRole(stage.OperatingPoint, "threshold") {
		return "threshold_ceiling_time"
	}
	return "drive_ceiling_time"
}

func limiterModelConfidence(model *LimiterModel) float64 {
	minimum := 0.96
	for _, stage := range model.Stages {
		confidence := 0.90
		if limiterBindingsHaveRole(stage.OperatingPoint, "threshold") || limiterBindingsHaveRole(stage.Detector, "true_peak") ||
			limiterBindingsHaveRole(stage.Mode, "limiter_mode") || limiterBindingsHaveRole(stage.Mode, "stage_enable") {
			confidence = 0.96
		}
		if confidence < minimum {
			minimum = confidence
		}
	}
	return minimum
}

func limiterUnsupportedBoundary(digest ParameterDigest, candidates []limiterCandidate, auxByKind map[string][]string) string {
	hasDrive, hasCeiling, hasTime, hasClipCurve := false, false, false, false
	for _, candidate := range candidates {
		hasDrive = hasDrive || candidate.Role == "threshold" || candidate.Role == "input_drive"
		hasCeiling = hasCeiling || candidate.Role == "ceiling"
		hasTime = hasTime || candidate.Role == "release" || candidate.Role == "lookahead"
	}
	for _, param := range digest.Parameters {
		text := limiterParameterText(param)
		hasClipCurve = hasClipCurve || limiterHasToken(text, "knee") || limiterHasToken(text, "type")
	}
	if len(auxByKind["clipper"]) > 0 || (hasDrive && hasCeiling && hasClipCurve && !hasTime) {
		return "unsupported_clipper"
	}
	if len(auxByKind["multiband_dynamics"]) > 0 {
		return "unsupported_multiband_dynamics"
	}
	if compressor, _ := DetectCompressorModelWithBoundary(digest); compressor != nil {
		return "unsupported_broadband_compressor"
	}
	if len(auxByKind["gate_expander"]) > 0 {
		return "unsupported_gate_expander"
	}
	if hasCeiling || hasDrive || hasTime {
		return "unresolved_limiter_surface"
	}
	return "not_limiter"
}

func limiterTopologyGeneration(model *LimiterModel) string {
	type generationBinding struct {
		ParamID string                     `json:"param_id"`
		Role    string                     `json:"role"`
		Unit    string                     `json:"unit,omitempty"`
		Domain  *PluginDisplayDomain       `json:"domain,omitempty"`
		Curve   [][2]float64               `json:"curve,omitempty"`
		Values  []CompressorReachableValue `json:"reachable,omitempty"`
		Probe   bool                       `json:"transactional_probe,omitempty"`
	}
	type generationStage struct {
		Key      string                         `json:"key"`
		Scope    string                         `json:"scope"`
		Sections map[string][]generationBinding `json:"sections"`
	}
	rows := []generationStage{}
	toRows := func(bindings []LimiterBinding) []generationBinding {
		out := make([]generationBinding, 0, len(bindings))
		for _, binding := range bindings {
			out = append(out, generationBinding{ParamID: binding.ParamID, Role: binding.Role, Unit: binding.PhysicalUnit,
				Domain: binding.Domain, Curve: binding.Curve, Values: binding.Reachable, Probe: binding.TransactionalProbe})
		}
		return out
	}
	for _, stage := range model.Stages {
		rows = append(rows, generationStage{Key: stage.Key, Scope: stage.Scope, Sections: map[string][]generationBinding{
			"operating_point": toRows(stage.OperatingPoint), "safety": toRows(stage.Safety), "timing": toRows(stage.Timing),
			"detector": toRows(stage.Detector), "mode": toRows(stage.Mode), "output": toRows(stage.Output),
		}})
	}
	payload := map[string]any{"schema": limiterTopologySchema, "classification": model.Classification, "stages": rows,
		"shared": toRows(model.SharedControls), "auxiliary_stages": model.AuxiliaryStages}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return "l1t1_" + hex.EncodeToString(sum[:12])
}

func limiterInputDriveName(text string) bool {
	if limiterContains(text, "sidechain", "side chain", "output", "makeup") || limiterHasToken(text, "pan") || limiterHasToken(text, "display") {
		return false
	}
	return limiterHasToken(text, "input") || limiterContains(text, "input gain", "input trim", "input level", "drive")
}

func limiterOutputName(text string) bool {
	if limiterHasToken(text, "pan") || limiterHasToken(text, "display") {
		return false
	}
	return limiterHasToken(text, "output") || limiterContains(text, "out gain", "out level", "output gain", "output level")
}

func limiterMixName(text string) bool {
	return !limiterHasToken(text, "pan") && (limiterHasToken(text, "mix") || limiterContains(text, "dry wet", "dry/wet", "parallel"))
}

func limiterContains(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func limiterHasToken(text, token string) bool {
	return regexp.MustCompile(`(?i)(?:^|[^a-z0-9])` + regexp.QuoteMeta(token) + `(?:[^a-z0-9]|$)`).MatchString(text)
}
