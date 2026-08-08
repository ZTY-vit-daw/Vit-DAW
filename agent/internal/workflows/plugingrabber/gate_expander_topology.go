package plugingrabber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

const gateExpanderTopologySchema = "gate-expander-control-topology/v1"

type GateExpanderModel struct {
	Classification          string
	Confidence              float64
	Stage                   GateExpanderStage
	UnsupportedCapabilities []GateExpanderUnsupportedCapability
	AuxiliaryStages         []GateExpanderAuxiliaryStage
	Generation              string
}

type GateExpanderStage struct {
	Key              string
	Direction        string
	Detector         []GateExpanderBinding
	OperatingPoint   []GateExpanderBinding
	GainAction       []GateExpanderBinding
	DirectionControl []GateExpanderBinding
	Timing           []GateExpanderBinding
	Mode             []GateExpanderBinding
	Output           []GateExpanderBinding
	Capabilities     GateExpanderCapabilities
}

type GateExpanderBinding = CompressorBinding

type GateExpanderCapabilities struct {
	Detector         []string
	OperatingPoint   []string
	GainAction       []string
	DirectionControl []string
	Timing           []string
	Mode             []string
	Output           []string
}

type GateExpanderUnsupportedCapability struct {
	Kind     string
	ParamIDs []string
}

type GateExpanderAuxiliaryStage struct {
	Kind     string
	ParamIDs []string
}

type gateExpanderCandidate struct {
	Param              ParameterInfo
	Role               string
	Section            string
	GateQualified      bool
	UpwardThreshold    bool
	UpwardRatio        bool
	ExplicitGate       bool
	ExplicitExpander   bool
	DirectionHasGate   bool
	DirectionHasExpand bool
	AuxiliaryKind      string
	UnsupportedKind    string
}

var gateExpanderChannelToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:channel|ch)\s*[1-9][0-9]*(?:[^a-z0-9]|$)`)
var gateExpanderLinkPairToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])link\s*[1-9][0-9]*\s*\+\s*[1-9][0-9]*(?:[^a-z0-9]|$)`)
var gateExpanderBandToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:band|b)\s*[1-9][0-9]*(?:[^a-z0-9]|$)`)

// DetectGateExpanderModelWithBoundary recovers only a single shared hard-gate
// or provably downward-expanding stage. Product identity, existing normalized
// roles, display groups, and current values are excluded from recognition.
func DetectGateExpanderModelWithBoundary(digest ParameterDigest) (*GateExpanderModel, string) {
	candidates := make([]gateExpanderCandidate, 0, len(digest.Parameters))
	unsupported := map[string][]string{}
	auxiliary := map[string][]string{}
	upwardThreshold, upwardRatio := false, false
	for _, param := range digest.Parameters {
		candidate := classifyGateExpanderParameter(param)
		upwardThreshold = upwardThreshold || candidate.UpwardThreshold
		upwardRatio = upwardRatio || candidate.UpwardRatio
		if candidate.UnsupportedKind != "" {
			unsupported[candidate.UnsupportedKind] = append(unsupported[candidate.UnsupportedKind], param.ID)
			continue
		}
		if candidate.AuxiliaryKind != "" {
			auxiliary[candidate.AuxiliaryKind] = append(auxiliary[candidate.AuxiliaryKind], param.ID)
			continue
		}
		if candidate.Role != "" && param.HostControllable {
			candidates = append(candidates, candidate)
		}
	}

	stage, classification, confidence := buildGateExpanderStage(candidates, upwardThreshold && upwardRatio)
	if classification == "" {
		return nil, gateExpanderUnsupportedBoundary(digest, candidates, unsupported, auxiliary)
	}
	model := &GateExpanderModel{Classification: classification, Confidence: confidence, Stage: stage}
	for kind, ids := range unsupported {
		sort.Strings(ids)
		model.UnsupportedCapabilities = append(model.UnsupportedCapabilities, GateExpanderUnsupportedCapability{Kind: kind, ParamIDs: ids})
	}
	for kind, ids := range auxiliary {
		sort.Strings(ids)
		model.AuxiliaryStages = append(model.AuxiliaryStages, GateExpanderAuxiliaryStage{Kind: kind, ParamIDs: ids})
	}
	sort.Slice(model.UnsupportedCapabilities, func(i, j int) bool {
		return model.UnsupportedCapabilities[i].Kind < model.UnsupportedCapabilities[j].Kind
	})
	sort.Slice(model.AuxiliaryStages, func(i, j int) bool { return model.AuxiliaryStages[i].Kind < model.AuxiliaryStages[j].Kind })
	model.Generation = gateExpanderTopologyGeneration(model)
	return model, ""
}

func DetectGateExpanderModel(digest ParameterDigest) *GateExpanderModel {
	model, _ := DetectGateExpanderModelWithBoundary(digest)
	return model
}

func BuildGateExpanderSummaryWithBoundary(digest ParameterDigest) (map[string]any, string) {
	model, boundary := DetectGateExpanderModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	unsupported := make([]map[string]any, 0, len(model.UnsupportedCapabilities))
	for _, capability := range model.UnsupportedCapabilities {
		unsupported = append(unsupported, map[string]any{"kind": capability.Kind, "param_ids": capability.ParamIDs})
	}
	auxiliary := make([]map[string]any, 0, len(model.AuxiliaryStages))
	for _, stage := range model.AuxiliaryStages {
		auxiliary = append(auxiliary, map[string]any{"kind": stage.Kind, "param_ids": stage.ParamIDs})
	}
	return map[string]any{
		"schema_version": gateExpanderTopologySchema,
		"mapping_source": "generic_structural",
		"classification": model.Classification,
		"confidence":     model.Confidence,
		"control_topology": map[string]any{
			"schema_version": gateExpanderTopologySchema,
			"generation":     model.Generation,
		},
		"gate_expander_stage":      gateExpanderStageSummary(model.Stage),
		"unsupported_capabilities": unsupported,
		"auxiliary_stages":         auxiliary,
	}, ""
}

func BuildGateExpanderSummary(digest ParameterDigest) map[string]any {
	summary, _ := BuildGateExpanderSummaryWithBoundary(digest)
	return summary
}

func classifyGateExpanderParameter(param ParameterInfo) gateExpanderCandidate {
	text := gateExpanderParameterText(param)
	candidate := gateExpanderCandidate{Param: param}
	candidate.ExplicitGate = gateExpanderHasToken(text, "gate")
	candidate.ExplicitExpander = gateExpanderContains(text, "expander", "expansion", "expand ratio")
	candidate.GateQualified = candidate.ExplicitGate

	if text == "" || gateExpanderContains(text, "preset", "bank", "meter", "display", "audition", "visualizer") {
		return candidate
	}
	if gateExpanderContains(text, "midi") {
		candidate.UnsupportedKind = "midi_trigger"
		return candidate
	}
	if gateExpanderContains(text, "left side chain", "right side chain", "left sidechain", "right sidechain") {
		candidate.UnsupportedKind = "multichannel_detector"
		return candidate
	}
	if gateExpanderChannelToken.MatchString(text) || gateExpanderLinkPairToken.MatchString(text) {
		candidate.UnsupportedKind = "multichannel_detector"
		return candidate
	}
	if gateExpanderBandToken.MatchString(text) || gateExpanderContains(text, "crossover", "cross over") {
		candidate.AuxiliaryKind = "multiband_dynamics"
		return candidate
	}
	if gateExpanderContains(text, "limiter", "limit ceiling", "true peak") || gateExpanderHasToken(text, "ceiling") {
		candidate.AuxiliaryKind = "limiter"
		return candidate
	}
	if gateExpanderContains(text, "clipper", "clipping", "clip ceiling", "clip threshold") {
		candidate.AuxiliaryKind = "clipper"
		return candidate
	}
	if gateExpanderContains(text, "de-esser", "de esser", "deesser", "s-reduction", "s reduction") {
		candidate.AuxiliaryKind = "de_esser"
		return candidate
	}
	if gateExpanderContains(text, "sustain", "transient amount", "attack amount") {
		candidate.AuxiliaryKind = "transient_shaper"
		return candidate
	}
	if gateExpanderHasToken(text, "upward") {
		candidate.UpwardThreshold = gateExpanderHasToken(text, "threshold") || gateExpanderHasToken(text, "thresh")
		candidate.UpwardRatio = gateExpanderHasToken(text, "ratio")
		if candidate.UpwardThreshold || candidate.UpwardRatio {
			candidate.AuxiliaryKind = "upward_expander"
			return candidate
		}
	}
	// C1-style composite surfaces expose the gate controls with an explicit
	// Gate prefix while their compressor controls use Comp/Strap prefixes.
	// Keep the latter out of the gate stage so the shared parameter names do
	// not collapse two physical processors into one topology.
	if gateExpanderContains(text, "comp ", "compressor", "strap ") && !candidate.GateQualified && !candidate.ExplicitExpander {
		candidate.AuxiliaryKind = "broadband_compressor"
		return candidate
	}

	if gateExpanderDirectionControl(param, text, &candidate) {
		return candidate
	}
	switch {
	case gateExpanderContains(text, "gate open"):
		candidate.Role, candidate.Section, candidate.GateQualified = "threshold", "operating_point", true
	case gateExpanderContains(text, "gate close"):
		candidate.Role, candidate.Section, candidate.GateQualified = "hysteresis", "operating_point", true
	case gateExpanderHasToken(text, "floor"):
		candidate.Role, candidate.Section, candidate.GateQualified = "range", "gain_action", true
	case gateExpanderContains(text, "sc hpf", "sc hp", "sidechain hpf", "sidechain high pass", "detector hpf", "detector high pass", "hpf freq", "high pass frequency"):
		candidate.Role, candidate.Section = "sidechain_highpass", "detector"
	case gateExpanderContains(text, "sc lpf", "sc lp", "sidechain lpf", "sidechain low pass", "detector lpf", "detector low pass", "lpf freq", "low pass frequency"):
		candidate.Role, candidate.Section = "sidechain_lowpass", "detector"
	case gateExpanderContains(text, "sidechain filter", "sc filter"):
		candidate.Role, candidate.Section = "sidechain_filter", "detector"
	case gateExpanderContains(text, "analysis gain", "detector gain", "sidechain gain"):
		candidate.Role, candidate.Section = "detector_gain", "detector"
	case gateExpanderContains(text, "rms-peak", "rms peak", "peak-rms", "peak rms"):
		candidate.Role, candidate.Section = "detector_character", "detector"
	case gateExpanderHasToken(text, "sidechain") || gateExpanderHasToken(text, "detector"):
		candidate.Role, candidate.Section = "detector_mode", "detector"
	case gateExpanderHasToken(text, "threshold") || gateExpanderHasToken(text, "thresh"):
		candidate.Role, candidate.Section = "threshold", "operating_point"
	case gateExpanderHasToken(text, "hysteresis"):
		candidate.Role, candidate.Section = "hysteresis", "operating_point"
	case gateExpanderHasToken(text, "ratio"):
		candidate.Role, candidate.Section = "expansion_ratio", "gain_action"
	case gateExpanderHasToken(text, "range") || gateExpanderHasToken(text, "reduction") || gateExpanderContains(text, "attenuation depth", "gate depth"):
		candidate.Role, candidate.Section = "range", "gain_action"
	case gateExpanderHasToken(text, "attack"):
		candidate.Role, candidate.Section = "attack", "timing"
	case gateExpanderHasToken(text, "hold"):
		candidate.Role, candidate.Section = "hold", "timing"
	case gateExpanderHasToken(text, "release") || gateExpanderHasToken(text, "recovery"):
		candidate.Role, candidate.Section = "release", "timing"
	case gateExpanderContains(text, "lookahead", "look ahead"):
		candidate.Role, candidate.Section = "lookahead", "timing"
	case gateExpanderContains(text, "cycle delay", "retrigger delay"):
		candidate.Role, candidate.Section = "cycle_delay", "timing"
	case candidate.ExplicitGate && (gateExpanderHasToken(text, "enable") || gateExpanderHasToken(text, "active") || gateExpanderHasToken(text, "bypass")):
		candidate.Role, candidate.Section = "stage_enable", "mode"
	case gateExpanderContains(text, "dry/wet", "dry wet") || gateExpanderHasToken(text, "mix"):
		candidate.Role, candidate.Section = "mix", "output"
	case gateExpanderContains(text, "output gain", "output level", "out gain", "out level"):
		candidate.Role, candidate.Section = "output_gain", "output"
	}
	if candidate.GateQualified && candidate.Role == "" {
		candidate.GateQualified = false
	}
	return candidate
}

func gateExpanderDirectionControl(param ParameterInfo, text string, candidate *gateExpanderCandidate) bool {
	if !(gateExpanderHasToken(text, "direction") || gateExpanderContains(text, "dynamic mode", "dynamics mode", "gate mode", "expander mode", "process mode")) {
		return false
	}
	hasGate, hasExpand := gateExpanderDirectionLabels(param)
	if !hasGate && !hasExpand {
		return false
	}
	candidate.Role, candidate.Section = "direction_mode", "direction"
	candidate.DirectionHasGate, candidate.DirectionHasExpand = hasGate, hasExpand
	return true
}

func gateExpanderDirectionLabels(param ParameterInfo) (bool, bool) {
	if param.DisplayProbe == nil {
		return false, false
	}
	labels := append([]string{}, param.DisplayProbe.AllLabels...)
	for _, row := range param.DisplayProbe.DiscreteLabels {
		labels = append(labels, row.Label)
	}
	hasGate, hasExpand := false, false
	for _, label := range labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		// Upward expansion and compression are the opposite gain direction;
		// neither can prove this downward Gate/Expander surface.
		if gateExpanderContains(lower, "upward", "compressor", "compression") {
			continue
		}
		hasGate = hasGate || gateExpanderHasToken(lower, "gate")
		hasExpand = hasExpand || gateExpanderContains(lower, "expander", "expansion", "downward expand", "downward")
	}
	return hasGate, hasExpand
}

func buildGateExpanderStage(candidates []gateExpanderCandidate, upwardPairProven bool) (GateExpanderStage, string, float64) {
	stage := GateExpanderStage{Key: "gate_expander", Direction: "unknown"}
	gateQualifiedSurface := false
	for _, candidate := range candidates {
		gateQualifiedSurface = gateQualifiedSurface || candidate.GateQualified
	}
	explicitGate, explicitExpand := false, false
	directionHasGate, directionHasExpand := false, false
	expansionDomainProven := false
	for _, candidate := range candidates {
		// A generic Threshold on a composite C1 surface belongs to the
		// compressor stage when a separately qualified Gate surface exists.
		if gateQualifiedSurface && !candidate.GateQualified &&
			(candidate.Role == "threshold" || candidate.Role == "expansion_ratio") {
			continue
		}
		binding := gateExpanderBinding(candidate)
		switch candidate.Section {
		case "detector":
			stage.Detector = append(stage.Detector, binding)
		case "operating_point":
			stage.OperatingPoint = append(stage.OperatingPoint, binding)
		case "gain_action":
			stage.GainAction = append(stage.GainAction, binding)
		case "direction":
			stage.DirectionControl = append(stage.DirectionControl, binding)
		case "timing":
			stage.Timing = append(stage.Timing, binding)
		case "mode":
			stage.Mode = append(stage.Mode, binding)
		case "output":
			stage.Output = append(stage.Output, binding)
		}
		// Hold is state timing, not identity evidence: broadband compressors can
		// expose it too. Require an explicit Gate surface, hysteresis, or a
		// reachable Gate direction before promoting the shared stage.
		explicitGate = explicitGate || candidate.ExplicitGate || candidate.Role == "hysteresis"
		explicitExpand = explicitExpand || candidate.ExplicitExpander
		directionHasGate = directionHasGate || candidate.DirectionHasGate
		directionHasExpand = directionHasExpand || candidate.DirectionHasExpand
		if candidate.Role == "expansion_ratio" {
			expansionDomainProven = expansionDomainProven || gateExpansionRatioDomainProven(binding)
		}
	}
	for _, bindings := range []*[]GateExpanderBinding{&stage.Detector, &stage.OperatingPoint, &stage.GainAction, &stage.DirectionControl, &stage.Timing, &stage.Mode, &stage.Output} {
		sortCompressorBindings(bindings)
	}

	hasThreshold := gateExpanderBindingsHaveRole(stage.OperatingPoint, "threshold")
	hasRange := gateExpanderBindingsHaveRole(stage.GainAction, "range")
	hasAttack := gateExpanderBindingsHaveRole(stage.Timing, "attack")
	hasHold := gateExpanderBindingsHaveRole(stage.Timing, "hold")
	hasRelease := gateExpanderBindingsHaveRole(stage.Timing, "release")
	hasStateTiming := hasRelease && (hasAttack || hasHold)
	hardGate := hasThreshold && hasRange && hasStateTiming && (explicitGate || directionHasGate)
	// Some real downward expanders expose a fixed/implicit attack and only
	// publish Release. Accept that shape only with detector filtering and a
	// measured negative-dB attenuation range; the display domain supplies the
	// direction evidence that a bare Threshold/Range/Release trio lacks.
	releaseOnlyExpander := hasThreshold && hasRange && hasRelease && !hasAttack && !hasHold &&
		(gateExpanderBindingsHaveRole(stage.Detector, "sidechain_highpass") ||
			gateExpanderBindingsHaveRole(stage.Detector, "sidechain_lowpass"))
	releaseOnlyExpander = releaseOnlyExpander && gateExpanderRangeDomainProven(stage.GainAction)
	pairedDirectionalExpander := hasThreshold && hasRange && hasStateTiming && upwardPairProven &&
		expansionDomainProven && gateExpanderAttenuationDepthDomainProven(stage.GainAction)
	downwardExpander := hasThreshold && hasRange && hasStateTiming && explicitExpand && expansionDomainProven ||
		releaseOnlyExpander || pairedDirectionalExpander
	selectableExpander := hasThreshold && hasRange && hasStateTiming && directionHasExpand

	classification, confidence := "", 0.0
	switch {
	case directionHasGate && directionHasExpand && hardGate:
		classification, stage.Direction, confidence = "selectable_gate_expander", "selectable_gate_or_downward_expansion", 0.96
	case downwardExpander || selectableExpander:
		classification, stage.Direction, confidence = "downward_expander", "downward_expansion", 0.90
	case hardGate:
		classification, stage.Direction, confidence = "hard_gate", "closed_attenuation", 0.96
	}
	stage.Capabilities = gateExpanderCapabilities(stage)
	return stage, classification, confidence
}

func gateExpanderRangeDomainProven(bindings []GateExpanderBinding) bool {
	for _, binding := range bindings {
		if binding.Role != "range" || binding.Domain == nil || binding.Domain.Min == nil || binding.Domain.Max == nil {
			continue
		}
		if strings.EqualFold(binding.PhysicalUnit, "db") && *binding.Domain.Min < 0 && *binding.Domain.Max <= 0 {
			return true
		}
	}
	return false
}

func gateExpanderAttenuationDepthDomainProven(bindings []GateExpanderBinding) bool {
	for _, binding := range bindings {
		if binding.Role != "range" || binding.Domain == nil || binding.Domain.Min == nil || binding.Domain.Max == nil {
			continue
		}
		if strings.EqualFold(binding.PhysicalUnit, "db") && *binding.Domain.Min >= 0 && *binding.Domain.Max > 0 {
			return true
		}
	}
	return false
}

func gateExpansionRatioDomainProven(binding GateExpanderBinding) bool {
	minimum, maximum, observed := 0.0, 0.0, false
	observe := func(value float64) {
		if !observed {
			minimum, maximum, observed = value, value, true
			return
		}
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	for _, point := range binding.Curve {
		observe(point[1])
	}
	for _, value := range binding.Reachable {
		if value.Physical != nil {
			observe(*value.Physical)
		}
	}
	if binding.Domain != nil && binding.Domain.Min != nil && binding.Domain.Max != nil {
		observe(*binding.Domain.Min)
		observe(*binding.Domain.Max)
	}
	return observed && minimum >= 1 && maximum > 1
}

func gateExpanderBinding(candidate gateExpanderCandidate) GateExpanderBinding {
	parseRole := candidate.Role
	if parseRole == "expansion_ratio" {
		parseRole = "ratio"
	} else if parseRole == "cycle_delay" {
		parseRole = "hold"
	}
	param := candidate.Param
	binding := GateExpanderBinding{ParamID: param.ID, Name: parameterDisplayName(param), Role: candidate.Role,
		Channel: "shared", CompressionStage: "gate_expander", CurrentText: param.ValueText, CurrentNormalized: anyFloat(param.NormalizedValue),
		Domain: param.DisplayDomainCandidate, Curve: compressorCurveFromProbe(param, parseRole), Reachable: compressorReachableValues(param, parseRole)}
	if len(binding.Curve) == 0 && compressorProbeHasNonFiniteSentinel(param.DisplayProbe) {
		binding.Domain = nil
	}
	if value, ok := parseCompressorPhysical(parseRole, param.ValueText); ok {
		binding.CurrentPhysical = &value
	}
	binding.PhysicalUnit = compressorBindingPhysicalUnit(param, parseRole, binding.Domain)
	if candidate.Role == "expansion_ratio" {
		binding.PhysicalUnit = "ratio"
	}
	binding.TransactionalProbe = compressorBindingNeedsTransactionalProbe(param, binding)
	return binding
}

func gateExpanderStageSummary(stage GateExpanderStage) map[string]any {
	return map[string]any{
		"stage_key": stage.Key, "direction": stage.Direction,
		"detector": compressorBindingRows(stage.Detector), "operating_point": compressorBindingRows(stage.OperatingPoint),
		"gain_action": compressorBindingRows(stage.GainAction), "direction_control": compressorBindingRows(stage.DirectionControl),
		"timing": compressorBindingRows(stage.Timing), "mode": compressorBindingRows(stage.Mode), "output": compressorBindingRows(stage.Output),
		"capabilities": gateExpanderCapabilitiesMap(stage.Capabilities),
	}
}

func gateExpanderCapabilities(stage GateExpanderStage) GateExpanderCapabilities {
	return GateExpanderCapabilities{Detector: gateExpanderBindingRoles(stage.Detector), OperatingPoint: gateExpanderBindingRoles(stage.OperatingPoint),
		GainAction: gateExpanderBindingRoles(stage.GainAction), DirectionControl: gateExpanderBindingRoles(stage.DirectionControl),
		Timing: gateExpanderBindingRoles(stage.Timing), Mode: gateExpanderBindingRoles(stage.Mode), Output: gateExpanderBindingRoles(stage.Output)}
}

func gateExpanderCapabilitiesMap(capabilities GateExpanderCapabilities) map[string]any {
	return map[string]any{"detector": capabilities.Detector, "operating_point": capabilities.OperatingPoint,
		"gain_action": capabilities.GainAction, "direction_control": capabilities.DirectionControl,
		"timing": capabilities.Timing, "mode": capabilities.Mode, "output": capabilities.Output}
}

func gateExpanderBindingRoles(bindings []GateExpanderBinding) []string {
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

func gateExpanderBindingsHaveRole(bindings []GateExpanderBinding, role string) bool {
	for _, binding := range bindings {
		if binding.Role == role {
			return true
		}
	}
	return false
}

func gateExpanderUnsupportedBoundary(digest ParameterDigest, candidates []gateExpanderCandidate, unsupported, auxiliary map[string][]string) string {
	if len(auxiliary["multiband_dynamics"]) > 0 {
		return "unsupported_multiband_dynamics"
	}
	if compressorHasSpectralDynamicsSurface(digest) {
		return "unsupported_spectral_dynamics"
	}
	for _, kind := range []string{"de_esser", "transient_shaper", "clipper", "limiter"} {
		if len(auxiliary[kind]) > 0 {
			return "unsupported_" + kind
		}
	}
	if compressor, _ := DetectCompressorModelWithBoundary(digest); compressor != nil {
		return "unsupported_broadband_compressor"
	}
	if len(candidates) > 0 {
		return "unresolved_gate_expander_surface"
	}
	if len(unsupported["multichannel_detector"]) > 0 {
		return "unsupported_multichannel_gate_expander"
	}
	if len(unsupported["midi_trigger"]) > 0 {
		return "unsupported_midi_gate_expander"
	}
	return "not_gate_expander"
}

func gateExpanderTopologyGeneration(model *GateExpanderModel) string {
	type generationBinding struct {
		ParamID            string                     `json:"param_id"`
		Role               string                     `json:"role"`
		Unit               string                     `json:"unit,omitempty"`
		Domain             *PluginDisplayDomain       `json:"domain,omitempty"`
		Curve              [][2]float64               `json:"curve,omitempty"`
		Reachable          []CompressorReachableValue `json:"reachable,omitempty"`
		TransactionalProbe bool                       `json:"transactional_probe,omitempty"`
	}
	convert := func(bindings []GateExpanderBinding) []generationBinding {
		out := make([]generationBinding, 0, len(bindings))
		for _, binding := range bindings {
			out = append(out, generationBinding{ParamID: binding.ParamID, Role: binding.Role, Unit: binding.PhysicalUnit,
				Domain: binding.Domain, Curve: binding.Curve, Reachable: binding.Reachable, TransactionalProbe: binding.TransactionalProbe})
		}
		return out
	}
	payload := struct {
		Schema                                                                       string `json:"schema"`
		Classification                                                               string `json:"classification"`
		Direction                                                                    string `json:"direction"`
		Detector, OperatingPoint, GainAction, DirectionControl, Timing, Mode, Output []generationBinding
		Unsupported                                                                  []GateExpanderUnsupportedCapability `json:"unsupported,omitempty"`
		Auxiliary                                                                    []GateExpanderAuxiliaryStage        `json:"auxiliary,omitempty"`
	}{Schema: gateExpanderTopologySchema, Classification: model.Classification, Direction: model.Stage.Direction,
		Detector: convert(model.Stage.Detector), OperatingPoint: convert(model.Stage.OperatingPoint), GainAction: convert(model.Stage.GainAction),
		DirectionControl: convert(model.Stage.DirectionControl), Timing: convert(model.Stage.Timing), Mode: convert(model.Stage.Mode), Output: convert(model.Stage.Output),
		Unsupported: model.UnsupportedCapabilities, Auxiliary: model.AuxiliaryStages}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return "g1t1_" + hex.EncodeToString(sum[:12])
}

func gateExpanderParameterText(param ParameterInfo) string {
	parts := []string{}
	seen := map[string]bool{}
	for _, value := range []string{param.Name, param.RawName, param.Alias} {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value != "" && !seen[key] {
			seen[key] = true
			parts = append(parts, value)
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func gateExpanderContains(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func gateExpanderHasToken(text, token string) bool {
	pattern := regexp.MustCompile(`(?i)(?:^|[^a-z0-9])` + regexp.QuoteMeta(token) + `(?:[^a-z0-9]|$)`)
	return pattern.MatchString(text)
}
