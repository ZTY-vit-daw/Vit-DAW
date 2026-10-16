package plugingrabber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// M2-TIMEFX-1 reverb control topology. The recognizer recovers only a
// parameter-proved reverb tail stage from the live parameter surface; product
// identity, vendor names, and current values stay out of classification.
// The provable core is the decay-time evidence (decay / size / length
// wording); pre-delay, character, and wet-balance bindings extend the same
// stage but never substitute for the core. The plate-family damper wording
// (PLUGGRABBER-TIMEDYN-COMPAT-1) proves the same tail stage at a reduced
// evidence tier: a plate's damper is its decay-time control by construction,
// but it does not name the decay time itself, so it stays below the
// direct-wording tier and a discrete plate/model selector stays character.

const reverbTopologySchema = "reverb-control-topology/v1"

// Reverb tail evidence tiers. Direct wording names the decay-time family
// outright; plate-family wording proves the same tail through plate physics
// and therefore carries a reduced confidence, never substituting for direct
// wording where both are present.
const (
	reverbTailEvidenceDirect      = "direct_wording"
	reverbTailEvidencePlateFamily = "plate_damper_family"
)

type ReverbModel struct {
	Classification  string
	Confidence      float64
	Stage           ReverbStage
	AuxiliaryStages []ReverbAuxiliaryStage
	Generation      string
}

type ReverbStage struct {
	Key          string
	Tail         []CompressorBinding
	Timing       []CompressorBinding
	Character    []CompressorBinding
	Output       []CompressorBinding
	Capabilities ReverbCapabilities
	TailEvidence string
}

type ReverbCapabilities struct {
	Tail      []string
	Timing    []string
	Character []string
	Output    []string
}

type ReverbAuxiliaryStage struct {
	Kind     string
	ParamIDs []string
}

type reverbCandidate struct {
	Param         ParameterInfo
	Role          string
	Section       string
	AuxiliaryKind string
	TailEvidence  string
}

// DetectReverbModelWithBoundary recovers independently provable reverb stages
// from the live parameter surface. A surface without decay-time evidence is
// not a reverb (or an explicit auxiliary-family boundary); mix/character
// evidence alone stays unresolved rather than guessed.
func DetectReverbModelWithBoundary(digest ParameterDigest) (*ReverbModel, string) {
	candidates := make([]reverbCandidate, 0, len(digest.Parameters))
	auxByKind := map[string][]string{}
	for _, param := range digest.Parameters {
		candidate := classifyReverbParameter(param)
		if candidate.AuxiliaryKind != "" {
			auxByKind[candidate.AuxiliaryKind] = append(auxByKind[candidate.AuxiliaryKind], param.ID)
			continue
		}
		if candidate.Role != "" && param.HostControllable {
			candidates = append(candidates, candidate)
		}
	}
	hasTail := false
	for _, candidate := range candidates {
		hasTail = hasTail || candidate.Section == "tail"
	}
	if !hasTail {
		return nil, reverbUnsupportedBoundary(digest, candidates, auxByKind)
	}
	stage := buildReverbStage(candidates)
	model := &ReverbModel{Classification: classifyReverbStage(stage), Confidence: reverbStageConfidence(stage), Stage: stage}
	for kind, ids := range auxByKind {
		sort.Strings(ids)
		model.AuxiliaryStages = append(model.AuxiliaryStages, ReverbAuxiliaryStage{Kind: kind, ParamIDs: ids})
	}
	sort.Slice(model.AuxiliaryStages, func(i, j int) bool { return model.AuxiliaryStages[i].Kind < model.AuxiliaryStages[j].Kind })
	model.Generation = reverbTopologyGeneration(model)
	return model, ""
}

func DetectReverbModel(digest ParameterDigest) *ReverbModel {
	model, _ := DetectReverbModelWithBoundary(digest)
	return model
}

func BuildReverbSummaryWithBoundary(digest ParameterDigest) (map[string]any, string) {
	model, boundary := DetectReverbModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	aux := make([]map[string]any, 0, len(model.AuxiliaryStages))
	for _, stage := range model.AuxiliaryStages {
		aux = append(aux, map[string]any{"kind": stage.Kind, "param_ids": stage.ParamIDs})
	}
	return map[string]any{
		"schema_version": reverbTopologySchema,
		"mapping_source": "generic_structural",
		"classification": model.Classification,
		"confidence":     model.Confidence,
		"tail_evidence":  model.Stage.TailEvidence,
		"control_topology": map[string]any{
			"schema_version": reverbTopologySchema,
			"generation":     model.Generation,
		},
		"reverb_stage":     reverbStageSummary(model.Stage),
		"auxiliary_stages": aux,
	}, ""
}

func BuildReverbSummary(digest ParameterDigest) map[string]any {
	summary, _ := BuildReverbSummaryWithBoundary(digest)
	return summary
}

func classifyReverbParameter(param ParameterInfo) reverbCandidate {
	text := reverbParameterText(param)
	candidate := reverbCandidate{Param: param}
	if text == "" || reverbContains(text, "preset", "meter", "display", "bypass") {
		return candidate
	}
	// Dynamic-family evidence never forms reverb bindings; it is retained as
	// auxiliary stage proof for family boundary decisions.
	if reverbContains(text, "threshold", "ratio", "knee", "compression", "comp ") || reverbHasToken(text, "comp") {
		candidate.AuxiliaryKind = "broadband_compressor"
		return candidate
	}
	if reverbContains(text, "limiter", "limiting", "true peak") || reverbHasToken(text, "limit") {
		candidate.AuxiliaryKind = "limiter"
		if reverbContains(text, "clip") {
			candidate.AuxiliaryKind = "limiter_or_clipper"
		}
		return candidate
	}
	if reverbContains(text, "clipper", "clip ceiling", "clip threshold") {
		candidate.AuxiliaryKind = "clipper"
		return candidate
	}
	if reverbContains(text, "gate", "expander", "hysteresis") {
		candidate.AuxiliaryKind = "gate_expander"
		return candidate
	}
	if reverbContains(text, "de-esser", "de esser", "deesser", "sibilance") {
		candidate.AuxiliaryKind = "de_esser"
		return candidate
	}
	if reverbBandToken(text) || reverbContains(text, "crossover", "cross over") {
		candidate.AuxiliaryKind = "multiband_dynamics"
		return candidate
	}
	// Delay-specific controls (echo time / feedback / repeats) are auxiliary
	// evidence: hybrid delay-reverb surfaces stay honest instead of being
	// re-labeled reverb bindings.
	if reverbContains(text, "feedback") || reverbHasToken(text, "echo") || reverbContains(text, "delay time", "delay length", "tap time") {
		candidate.AuxiliaryKind = "delay"
		return candidate
	}
	switch {
	case reverbContains(text, "pre delay", "predelay", "pre-delay"):
		candidate.Role, candidate.Section = "pre_delay", "timing"
	case reverbContains(text, "decay time", "decaytime") || reverbHasToken(text, "decay"):
		candidate.Role, candidate.Section, candidate.TailEvidence = "decay", "tail", reverbTailEvidenceDirect
	case reverbHasToken(text, "size") || reverbContains(text, "room size", "hall size"):
		candidate.Role, candidate.Section, candidate.TailEvidence = "size", "tail", reverbTailEvidenceDirect
	case reverbContains(text, "reverb length", "tail length") || reverbHasToken(text, "length"):
		candidate.Role, candidate.Section, candidate.TailEvidence = "decay", "tail", reverbTailEvidenceDirect
	case reverbHasToken(text, "damper") && !reverbContains(text, "pedal", "sustain"):
		// Plate-family decay core: a plate reverb's Damper is its decay-time
		// control; sustain/damper-pedal wording belongs to keyboard
		// instruments and must not fabricate a reverb tail.
		candidate.Role, candidate.Section, candidate.TailEvidence = "decay", "tail", reverbTailEvidencePlateFamily
	case reverbContains(text, "damping", "damp", "diffusion", "diffuse"):
		candidate.Role, candidate.Section = "damping", "character"
	case reverbHasToken(text, "width") || reverbContains(text, "stereo width", "image width"):
		candidate.Role, candidate.Section = "width", "character"
	case reverbContains(text, "reverb mode", "space mode", "reverb algorithm", "reverb character", "plate selector", "plate select"):
		candidate.Role, candidate.Section = "reverb_mode", "character"
	case reverbHasToken(text, "modulation") || reverbContains(text, "mod rate", "mod depth", "chorus"):
		candidate.Role, candidate.Section = "damping", "character"
	case reverbContains(text, "dry/wet", "dry wet") || reverbHasToken(text, "wet"):
		candidate.Role, candidate.Section = "wet", "output"
	case reverbHasToken(text, "dry"):
		candidate.Role, candidate.Section = "dry", "output"
	case reverbHasToken(text, "mix"):
		candidate.Role, candidate.Section = "mix", "output"
	case reverbContains(text, "output gain", "output level") || reverbHasToken(text, "output"):
		candidate.Role, candidate.Section = "output_gain", "output"
	}
	return candidate
}

func reverbBandToken(text string) bool {
	return reverbIndexedBandToken(text)
}

func buildReverbStage(candidates []reverbCandidate) ReverbStage {
	stage := ReverbStage{Key: "reverb"}
	for _, candidate := range candidates {
		binding := reverbBinding(candidate)
		switch candidate.Section {
		case "tail":
			stage.Tail = append(stage.Tail, binding)
			stage.TailEvidence = reverbTailEvidenceTier(stage.TailEvidence, candidate.TailEvidence)
		case "timing":
			stage.Timing = append(stage.Timing, binding)
		case "character":
			stage.Character = append(stage.Character, binding)
		case "output":
			stage.Output = append(stage.Output, binding)
		}
	}
	for _, bindings := range []*[]CompressorBinding{&stage.Tail, &stage.Timing, &stage.Character, &stage.Output} {
		sortCompressorBindings(bindings)
	}
	stage.Capabilities = reverbCapabilities(stage)
	return stage
}

// reverbTailEvidenceTier folds one tail candidate's evidence into the stage
// tier; direct wording outranks family wording so a surface exposing both
// keeps the stronger core.
func reverbTailEvidenceTier(current, incoming string) string {
	if current == reverbTailEvidenceDirect || incoming == reverbTailEvidenceDirect {
		return reverbTailEvidenceDirect
	}
	if current != "" {
		return current
	}
	return incoming
}

func reverbBinding(candidate reverbCandidate) CompressorBinding {
	param := candidate.Param
	binding := CompressorBinding{ParamID: param.ID, Name: parameterDisplayName(param), Role: candidate.Role,
		Channel: "shared", CompressionStage: "reverb", CurrentText: param.ValueText,
		CurrentNormalized: anyFloat(param.NormalizedValue), Domain: param.DisplayDomainCandidate,
		Curve:     compressorCurveFromProbe(param, reverbParseRole(candidate.Role)),
		Reachable: compressorReachableValues(param, reverbParseRole(candidate.Role))}
	if value, ok := parseTimeDomainPhysical(candidate.Role, param.ValueText); ok {
		binding.CurrentPhysical = &value
	}
	binding.PhysicalUnit = reverbPhysicalUnit(param, candidate.Role, binding.Domain)
	binding.TransactionalProbe = compressorBindingNeedsTransactionalProbe(param, binding)
	return binding
}

// reverbParseRole maps reverb roles onto the compressor timing-role family so
// the shared probe/curve helpers parse the same unit conventions (seconds
// wording included).
func reverbParseRole(role string) string {
	switch role {
	case "decay", "size", "pre_delay":
		return "release"
	}
	return role
}

func reverbPhysicalUnit(param ParameterInfo, role string, domain *PluginDisplayDomain) string {
	if domain != nil && strings.TrimSpace(domain.Unit) != "" {
		return domain.Unit
	}
	switch role {
	case "decay", "size":
		return "s"
	case "pre_delay":
		return "ms"
	}
	if domain != nil && domain.Min != nil && domain.Max != nil && *domain.Min >= -100 && *domain.Max <= 100 {
		if reverbRoleIn(role, "mix", "wet", "dry") {
			return "%"
		}
	}
	return compressorBindingPhysicalUnit(param, reverbParseRole(role), domain)
}

func reverbRoleIn(role string, values ...string) bool {
	for _, value := range values {
		if role == value {
			return true
		}
	}
	return false
}

// parseTimeDomainPhysical parses one reverb/delay role's physical value from
// display text. Decay parses to seconds (ms wording converts down); pre-delay
// parses to milliseconds; wet-balance roles parse to percentage points. Any
// other role keeps the generic number parse.
func parseTimeDomainPhysical(role, text string) (float64, bool) {
	value, unit, ok := parseProbeDisplayNumber(text, "")
	if !ok {
		return 0, false
	}
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch role {
	case "decay", "size":
		if unit == "ms" || strings.Contains(strings.ToLower(strings.TrimSpace(text)), "ms") {
			return value / 1000, true
		}
		return value, true
	case "pre_delay":
		if unit == "s" || unit == "sec" {
			return value * 1000, true
		}
		return value, true
	case "mix", "wet", "dry", "feedback":
		return value, true
	}
	return parseCompressorPhysical(reverbParseRole(role), text)
}

func reverbCapabilities(stage ReverbStage) ReverbCapabilities {
	return ReverbCapabilities{
		Tail:      reverbBindingRoles(stage.Tail),
		Timing:    reverbBindingRoles(stage.Timing),
		Character: reverbBindingRoles(stage.Character),
		Output:    reverbBindingRoles(stage.Output),
	}
}

func reverbBindingRoles(bindings []CompressorBinding) []string {
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

func reverbStageSummary(stage ReverbStage) map[string]any {
	return map[string]any{
		"stage_key": stage.Key,
		"tail":      compressorBindingRows(stage.Tail),
		"timing":    compressorBindingRows(stage.Timing),
		"character": compressorBindingRows(stage.Character),
		"output":    compressorBindingRows(stage.Output),
		"capabilities": map[string]any{
			"tail":      stage.Capabilities.Tail,
			"timing":    stage.Capabilities.Timing,
			"character": stage.Capabilities.Character,
			"output":    stage.Capabilities.Output,
		},
	}
}

func classifyReverbStage(stage ReverbStage) string {
	if len(stage.Tail) > 0 && len(stage.Timing) > 0 {
		return "predelay_decay_reverb"
	}
	return "decay_time_reverb"
}

func reverbStageConfidence(stage ReverbStage) float64 {
	if len(stage.Tail) == 0 {
		return 0.90
	}
	if stage.TailEvidence == reverbTailEvidencePlateFamily {
		// Plate-family tail wording proves the stage but rests on plate
		// physics rather than the parameter naming the decay time itself.
		return 0.90
	}
	return 0.96
}

func reverbUnsupportedBoundary(digest ParameterDigest, candidates []reverbCandidate, auxByKind map[string][]string) string {
	hasWet := false
	hasCharacter := false
	for _, candidate := range candidates {
		hasWet = hasWet || candidate.Section == "output"
		hasCharacter = hasCharacter || candidate.Section == "character"
	}
	if len(auxByKind["broadband_compressor"]) > 0 {
		return "unsupported_broadband_compressor"
	}
	if len(auxByKind["limiter"]) > 0 || len(auxByKind["limiter_or_clipper"]) > 0 {
		return "unsupported_limiter"
	}
	if len(auxByKind["gate_expander"]) > 0 {
		return "unsupported_gate_expander"
	}
	if len(auxByKind["multiband_dynamics"]) > 0 {
		return "unsupported_multiband_dynamics"
	}
	if len(auxByKind["de_esser"]) > 0 {
		return "unsupported_de_esser"
	}
	if len(auxByKind["delay"]) > 0 {
		return "unsupported_delay"
	}
	if hasWet && hasCharacter {
		return "unresolved_reverb_surface"
	}
	if hasWet || hasCharacter {
		return "unresolved_reverb_surface"
	}
	return "not_reverb"
}

func reverbTopologyGeneration(model *ReverbModel) string {
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
		Sections map[string][]generationBinding `json:"sections"`
	}
	toRows := func(bindings []CompressorBinding) []generationBinding {
		out := make([]generationBinding, 0, len(bindings))
		for _, binding := range bindings {
			out = append(out, generationBinding{ParamID: binding.ParamID, Role: binding.Role, Unit: binding.PhysicalUnit,
				Domain: binding.Domain, Curve: binding.Curve, Values: binding.Reachable, Probe: binding.TransactionalProbe})
		}
		return out
	}
	payload := map[string]any{"schema": reverbTopologySchema, "classification": model.Classification,
		"stages": []generationStage{{Key: model.Stage.Key, Sections: map[string][]generationBinding{
			"tail": toRows(model.Stage.Tail), "timing": toRows(model.Stage.Timing),
			"character": toRows(model.Stage.Character), "output": toRows(model.Stage.Output),
		}}},
		"auxiliary_stages": model.AuxiliaryStages}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return "rv1t1_" + hex.EncodeToString(sum[:12])
}

func reverbParameterText(param ParameterInfo) string {
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

func reverbContains(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func reverbHasToken(text, token string) bool {
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

// reverbIndexedBandToken reports whether the text carries an indexed band
// marker (band 1, b2, …) that proves multiband structure.
func reverbIndexedBandToken(text string) bool {
	return transientShaperBandToken.MatchString(text)
}
