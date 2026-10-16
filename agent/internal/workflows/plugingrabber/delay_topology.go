package plugingrabber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// M2-TIMEFX-1 delay control topology. The recognizer recovers only a
// parameter-proved echo-time stage from the live parameter surface; the
// provable core is the delay-time evidence (time / delay time / sync
// subdivision wording, plus the bare or unit-qualified delay wordings
// "Voice1 Delay" / "Delay Left ms" / "Delay Sec R" and the tempo/BPM sync
// and "fdbk" abbreviation families observed on the Waves/FabFilter
// compatibility pass). Feedback, character, and wet-balance bindings extend
// the same stage but never substitute for the core. Reverb-specific controls
// stay auxiliary evidence so hybrid delay-reverb surfaces resolve honestly.

const delayTopologySchema = "delay-control-topology/v1"

type DelayModel struct {
	Classification  string
	Confidence      float64
	Stage           DelayStage
	AuxiliaryStages []DelayAuxiliaryStage
	Generation      string
}

type DelayStage struct {
	Key          string
	Timing       []CompressorBinding
	Feedback     []CompressorBinding
	Character    []CompressorBinding
	Output       []CompressorBinding
	Capabilities DelayCapabilities
}

type DelayCapabilities struct {
	Timing    []string
	Feedback  []string
	Character []string
	Output    []string
}

type DelayAuxiliaryStage struct {
	Kind     string
	ParamIDs []string
}

type delayCandidate struct {
	Param         ParameterInfo
	Role          string
	Section       string
	AuxiliaryKind string
}

// DetectDelayModelWithBoundary recovers independently provable delay stages
// from the live parameter surface. A surface without echo-time evidence is
// not a delay (or an explicit auxiliary-family boundary); feedback/wet
// evidence alone stays unresolved rather than guessed.
func DetectDelayModelWithBoundary(digest ParameterDigest) (*DelayModel, string) {
	candidates := make([]delayCandidate, 0, len(digest.Parameters))
	auxByKind := map[string][]string{}
	for _, param := range digest.Parameters {
		candidate := classifyDelayParameter(param)
		if candidate.AuxiliaryKind != "" {
			auxByKind[candidate.AuxiliaryKind] = append(auxByKind[candidate.AuxiliaryKind], param.ID)
			continue
		}
		if candidate.Role != "" && param.HostControllable {
			candidates = append(candidates, candidate)
		}
	}
	hasTiming := false
	for _, candidate := range candidates {
		hasTiming = hasTiming || candidate.Section == "timing"
	}
	if !hasTiming {
		return nil, delayUnsupportedBoundary(digest, candidates, auxByKind)
	}
	stage := buildDelayStage(candidates)
	model := &DelayModel{Classification: classifyDelayStage(stage), Confidence: delayStageConfidence(stage), Stage: stage}
	for kind, ids := range auxByKind {
		sort.Strings(ids)
		model.AuxiliaryStages = append(model.AuxiliaryStages, DelayAuxiliaryStage{Kind: kind, ParamIDs: ids})
	}
	sort.Slice(model.AuxiliaryStages, func(i, j int) bool { return model.AuxiliaryStages[i].Kind < model.AuxiliaryStages[j].Kind })
	model.Generation = delayTopologyGeneration(model)
	return model, ""
}

func DetectDelayModel(digest ParameterDigest) *DelayModel {
	model, _ := DetectDelayModelWithBoundary(digest)
	return model
}

func BuildDelaySummaryWithBoundary(digest ParameterDigest) (map[string]any, string) {
	model, boundary := DetectDelayModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	aux := make([]map[string]any, 0, len(model.AuxiliaryStages))
	for _, stage := range model.AuxiliaryStages {
		aux = append(aux, map[string]any{"kind": stage.Kind, "param_ids": stage.ParamIDs})
	}
	return map[string]any{
		"schema_version": delayTopologySchema,
		"mapping_source": "generic_structural",
		"classification": model.Classification,
		"confidence":     model.Confidence,
		"control_topology": map[string]any{
			"schema_version": delayTopologySchema,
			"generation":     model.Generation,
		},
		"delay_stage":      delayStageSummary(model.Stage),
		"auxiliary_stages": aux,
	}, ""
}

func BuildDelaySummary(digest ParameterDigest) map[string]any {
	summary, _ := BuildDelaySummaryWithBoundary(digest)
	return summary
}

func classifyDelayParameter(param ParameterInfo) delayCandidate {
	text := delayParameterText(param)
	candidate := delayCandidate{Param: param}
	if text == "" || delayContains(text, "preset", "meter", "display", "bypass") {
		return candidate
	}
	if delayContains(text, "threshold", "ratio", "knee", "compression") || delayHasToken(text, "comp") {
		candidate.AuxiliaryKind = "broadband_compressor"
		return candidate
	}
	if delayContains(text, "limiter", "limiting", "true peak") || delayHasToken(text, "limit") {
		candidate.AuxiliaryKind = "limiter"
		if delayContains(text, "clip") {
			candidate.AuxiliaryKind = "limiter_or_clipper"
		}
		return candidate
	}
	if delayContains(text, "clipper", "clip ceiling", "clip threshold") {
		candidate.AuxiliaryKind = "clipper"
		return candidate
	}
	if delayContains(text, "gate", "expander", "hysteresis") {
		candidate.AuxiliaryKind = "gate_expander"
		return candidate
	}
	if delayContains(text, "de-esser", "de esser", "deesser", "sibilance") {
		candidate.AuxiliaryKind = "de_esser"
		return candidate
	}
	if transientShaperBandToken.MatchString(text) || delayContains(text, "crossover", "cross over") {
		candidate.AuxiliaryKind = "multiband_dynamics"
		return candidate
	}
	// Reverb-specific controls (decay / room / hall / plate / diffusion)
	// are auxiliary evidence: hybrid delay-reverb surfaces keep their delay
	// core while the reverb face stays visible as an auxiliary stage.
	if delayHasToken(text, "decay") || delayContains(text, "room size", "hall size", "plate", "diffusion", "diffuse", "predelay", "pre delay", "pre-delay") {
		candidate.AuxiliaryKind = "reverb"
		return candidate
	}
	switch {
	case delayContains(text, "delay time", "time", "delay length") || delayHasToken(text, "time"):
		candidate.Role, candidate.Section = "time", "timing"
	case delayContains(text, "sync", "subdivision", "note length", "tempo") ||
		(delayHasToken(text, "delay") && delayContains(text, "bpm")):
		candidate.Role, candidate.Section = "sync", "timing"
	case delayHasToken(text, "feedback") || delayContains(text, "repeats", "repetitions", "fdbk"):
		candidate.Role, candidate.Section = "feedback", "feedback"
	case delayContains(text, "ping pong", "pingpong"):
		candidate.Role, candidate.Section = "ping_pong", "character"
	case delayContains(text, "delay mode", "delay character", "delay algorithm", "tape mode", "vintage mode"):
		candidate.Role, candidate.Section = "delay_mode", "character"
	case delayContains(text, "dry/wet", "dry wet") || delayHasToken(text, "wet"):
		candidate.Role, candidate.Section = "wet", "output"
	case delayHasToken(text, "dry"):
		candidate.Role, candidate.Section = "dry", "output"
	case delayHasToken(text, "mix"):
		candidate.Role, candidate.Section = "mix", "output"
	case delayContains(text, "output gain", "output level") || delayHasToken(text, "output"):
		candidate.Role, candidate.Section = "output_gain", "output"
	case delayHasToken(text, "delay") && (delayHasToken(text, "ms") || delayHasToken(text, "sec")):
		// Delay-qualified second/millisecond quantities ("Delay Left ms",
		// "Delay Sec R") are the free delay time even without "time" wording.
		candidate.Role, candidate.Section = "time", "timing"
	case delayLastTokenIs(text, "delay"):
		// Bare per-voice delay wording ("Voice1 Delay") names the delay time
		// itself once every stronger wording above is excluded; trailing
		// qualifiers ("Delay Mod", "Delay Link") stay out.
		candidate.Role, candidate.Section = "time", "timing"
	}
	return candidate
}

// delayLastTokenIs reports whether the parameter text ends with the given
// word, so trailing-qualifier wordings ("Delay Mod", "Delay Link") stay out
// while delay-time names themselves ("Voice1 Delay") match.
func delayLastTokenIs(text, token string) bool {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(text)), func(r rune) bool {
		return r == ' ' || r == '_' || r == '-' || r == '/' || r == '(' || r == ')'
	})
	return len(parts) > 0 && parts[len(parts)-1] == strings.ToLower(strings.TrimSpace(token))
}

func buildDelayStage(candidates []delayCandidate) DelayStage {
	stage := DelayStage{Key: "delay"}
	for _, candidate := range candidates {
		binding := delayBinding(candidate)
		switch candidate.Section {
		case "timing":
			stage.Timing = append(stage.Timing, binding)
		case "feedback":
			stage.Feedback = append(stage.Feedback, binding)
		case "character":
			stage.Character = append(stage.Character, binding)
		case "output":
			stage.Output = append(stage.Output, binding)
		}
	}
	for _, bindings := range []*[]CompressorBinding{&stage.Timing, &stage.Feedback, &stage.Character, &stage.Output} {
		sortCompressorBindings(bindings)
	}
	stage.Capabilities = delayCapabilities(stage)
	return stage
}

func delayBinding(candidate delayCandidate) CompressorBinding {
	param := candidate.Param
	binding := CompressorBinding{ParamID: param.ID, Name: parameterDisplayName(param), Role: candidate.Role,
		Channel: "shared", CompressionStage: "delay", CurrentText: param.ValueText,
		CurrentNormalized: anyFloat(param.NormalizedValue), Domain: param.DisplayDomainCandidate,
		Curve:     compressorCurveFromProbe(param, delayParseRole(candidate.Role)),
		Reachable: compressorReachableValues(param, delayParseRole(candidate.Role))}
	if value, ok := parseTimeDomainPhysical(candidate.Role, param.ValueText); ok {
		binding.CurrentPhysical = &value
	}
	binding.PhysicalUnit = delayPhysicalUnit(param, candidate.Role, binding.Domain)
	binding.TransactionalProbe = compressorBindingNeedsTransactionalProbe(param, binding)
	return binding
}

// delayParseRole maps delay roles onto the compressor timing-role family so
// the shared probe/curve helpers parse the same time-unit conventions.
func delayParseRole(role string) string {
	switch role {
	case "time", "sync":
		return "release"
	}
	return role
}

func delayPhysicalUnit(param ParameterInfo, role string, domain *PluginDisplayDomain) string {
	if domain != nil && strings.TrimSpace(domain.Unit) != "" {
		return domain.Unit
	}
	switch role {
	case "time", "sync":
		return "ms"
	}
	if domain != nil && domain.Min != nil && domain.Max != nil && *domain.Min >= -100 && *domain.Max <= 100 {
		if delayRoleIn(role, "mix", "wet", "dry", "feedback") {
			return "%"
		}
	}
	return compressorBindingPhysicalUnit(param, delayParseRole(role), domain)
}

func delayRoleIn(role string, values ...string) bool {
	for _, value := range values {
		if role == value {
			return true
		}
	}
	return false
}

func delayCapabilities(stage DelayStage) DelayCapabilities {
	return DelayCapabilities{
		Timing:    delayBindingRoles(stage.Timing),
		Feedback:  delayBindingRoles(stage.Feedback),
		Character: delayBindingRoles(stage.Character),
		Output:    delayBindingRoles(stage.Output),
	}
}

func delayBindingRoles(bindings []CompressorBinding) []string {
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

func delayStageSummary(stage DelayStage) map[string]any {
	return map[string]any{
		"stage_key": stage.Key,
		"timing":    compressorBindingRows(stage.Timing),
		"feedback":  compressorBindingRows(stage.Feedback),
		"character": compressorBindingRows(stage.Character),
		"output":    compressorBindingRows(stage.Output),
		"capabilities": map[string]any{
			"timing":    stage.Capabilities.Timing,
			"feedback":  stage.Capabilities.Feedback,
			"character": stage.Capabilities.Character,
			"output":    stage.Capabilities.Output,
		},
	}
}

func classifyDelayStage(stage DelayStage) string {
	if len(stage.Timing) > 0 && len(stage.Feedback) > 0 {
		return "feedback_delay"
	}
	return "time_delay"
}

func delayStageConfidence(stage DelayStage) float64 {
	if len(stage.Timing) == 0 {
		return 0.90
	}
	return 0.96
}

func delayUnsupportedBoundary(digest ParameterDigest, candidates []delayCandidate, auxByKind map[string][]string) string {
	hasOutput := false
	for _, candidate := range candidates {
		hasOutput = hasOutput || candidate.Section == "output"
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
	if len(auxByKind["reverb"]) > 0 {
		return "unsupported_reverb"
	}
	if hasOutput {
		return "unresolved_delay_surface"
	}
	return "not_delay"
}

func delayTopologyGeneration(model *DelayModel) string {
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
	payload := map[string]any{"schema": delayTopologySchema, "classification": model.Classification,
		"stages": []generationStage{{Key: model.Stage.Key, Sections: map[string][]generationBinding{
			"timing": toRows(model.Stage.Timing), "feedback": toRows(model.Stage.Feedback),
			"character": toRows(model.Stage.Character), "output": toRows(model.Stage.Output),
		}}},
		"auxiliary_stages": model.AuxiliaryStages}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return "dl1t1_" + hex.EncodeToString(sum[:12])
}

func delayParameterText(param ParameterInfo) string {
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

func delayContains(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func delayHasToken(text, token string) bool {
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
