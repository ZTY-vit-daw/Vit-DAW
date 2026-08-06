package plugingrabber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const compressorTopologySchema = "compressor-control-topology/v1"

type CompressorModel struct {
	Classification  string
	Confidence      float64
	Stage           CompressorStage
	AuxiliaryStages []CompressorAuxiliaryStage
	ExclusionCodes  []string
	Generation      string
}

type CompressorStage struct {
	Key          string
	ControlPaths []CompressorControlPath
	Output       []CompressorBinding
	Capabilities CompressorCapabilities
}

type CompressorControlPath struct {
	Key              string
	Channel          string
	CompressionStage string
	Direction        string
	Detector         []CompressorBinding
	OperatingPoint   []CompressorBinding
	Transfer         []CompressorBinding
	Timing           []CompressorBinding
	GainAction       []CompressorBinding
	Capabilities     CompressorCapabilities
}

type CompressorBinding struct {
	ParamID            string
	Name               string
	Role               string
	Channel            string
	CompressionStage   string
	PhysicalUnit       string
	CurrentText        string
	CurrentNormalized  float64
	CurrentPhysical    *float64
	Domain             *PluginDisplayDomain
	Curve              [][2]float64
	Reachable          []CompressorReachableValue
	TransactionalProbe bool
}

type CompressorReachableValue struct {
	Normalized float64
	Physical   *float64
	Label      string
}

type CompressorCapabilities struct {
	OperatingPoint []string
	Transfer       []string
	Timing         []string
	Detector       []string
	GainAction     []string
	Output         []string
}

type CompressorAuxiliaryStage struct {
	Kind     string
	ParamIDs []string
}

type compressorCandidate struct {
	Param    ParameterInfo
	Role     string
	Section  string
	Path     string
	Channel  string
	Stage    string
	AuxKind  string
	Evidence int
}

var compressorBandToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:band|b)\s*([0-9]+)(?:[^a-z0-9]|$)`)
var compressorIndexedSuffixToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])([0-9]+)\s*$`)
var compressorLeftMidToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])l\s*/\s*m(?:[^a-z0-9]|$)`)
var compressorRightSideToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])r\s*/\s*s(?:[^a-z0-9]|$)`)
var compressorLeftRightToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])l\s*/\s*r(?:[^a-z0-9]|$)`)
var compressorLeftToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])l(?:[^a-z0-9]|$)`)
var compressorRightToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])r(?:[^a-z0-9]|$)`)
var compressorMidToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])m(?:[^a-z0-9]|$)`)
var compressorSideToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(?:[^a-z0-9]|$)`)

// DetectCompressorModel recovers a compressor control-path graph exclusively
// from the live parameter surface. Plugin/vendor identity is intentionally not
// consulted, and topology generation excludes current values.
func DetectCompressorModel(digest ParameterDigest) *CompressorModel {
	model, _ := DetectCompressorModelWithBoundary(digest)
	return model
}

// DetectCompressorModelWithBoundary returns a stable reason when the live
// surface belongs to an adjacent dynamics topology that v1 intentionally does
// not control. The boundary result is derived from parameter structure only.
func DetectCompressorModelWithBoundary(digest ParameterDigest) (*CompressorModel, string) {
	candidates := make([]compressorCandidate, 0, len(digest.Parameters))
	auxByKind := map[string][]string{}
	for _, param := range digest.Parameters {
		candidate := classifyCompressorParameter(param)
		if candidate.AuxKind != "" {
			auxByKind[candidate.AuxKind] = append(auxByKind[candidate.AuxKind], param.ID)
		}
		if candidate.Role == "" || candidate.AuxKind != "" || !param.HostControllable {
			continue
		}
		candidates = append(candidates, candidate)
	}
	boundary := compressorUnsupportedBoundary(digest, candidates, auxByKind)

	paths := buildCompressorPaths(candidates)
	output := collectCompressorOutput(candidates)
	if !compressorPathsHaveStageEvidence(paths, output) {
		if boundary != "" {
			return nil, boundary
		}
		return nil, "not_compressor"
	}
	if boundary == "unsupported_clipper" || boundary == "unsupported_multiband_compressor" || boundary == "unsupported_spectral_dynamics" {
		return nil, boundary
	}
	if boundary != "" && !compressorPathsHaveIndependentTransferEvidence(paths) {
		return nil, boundary
	}
	model := &CompressorModel{Stage: CompressorStage{Key: "compressor", ControlPaths: paths}}
	for kind, ids := range auxByKind {
		sort.Strings(ids)
		model.AuxiliaryStages = append(model.AuxiliaryStages, CompressorAuxiliaryStage{Kind: kind, ParamIDs: ids})
	}
	sort.Slice(model.AuxiliaryStages, func(i, j int) bool { return model.AuxiliaryStages[i].Kind < model.AuxiliaryStages[j].Kind })
	model.Stage.Output = output
	model.Stage.Capabilities = aggregateCompressorCapabilities(paths, model.Stage.Output)
	model.Classification = classifyCompressorModel(paths, model.AuxiliaryStages)
	model.Confidence = compressorModelConfidence(paths)
	model.Generation = compressorTopologyGeneration(model)
	return model, ""
}

func BuildCompressorSummary(digest ParameterDigest) map[string]any {
	summary, _ := BuildCompressorSummaryWithBoundary(digest)
	return summary
}

func BuildCompressorSummaryWithBoundary(digest ParameterDigest) (map[string]any, string) {
	model, boundary := DetectCompressorModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	paths := make([]map[string]any, 0, len(model.Stage.ControlPaths))
	for _, path := range model.Stage.ControlPaths {
		paths = append(paths, compressorPathSummary(path))
	}
	aux := make([]map[string]any, 0, len(model.AuxiliaryStages))
	for _, stage := range model.AuxiliaryStages {
		aux = append(aux, map[string]any{"kind": stage.Kind, "param_ids": stage.ParamIDs})
	}
	return map[string]any{
		"schema_version": compressorTopologySchema,
		"classification": model.Classification,
		"mapping_source": "generic_structural",
		"confidence":     model.Confidence,
		"control_topology": map[string]any{
			"schema_version": compressorTopologySchema,
			"generation":     model.Generation,
		},
		"compressor_stage": map[string]any{
			"stage_key":     model.Stage.Key,
			"path_count":    len(paths),
			"control_paths": paths,
			"output":        compressorBindingRows(model.Stage.Output),
			"capabilities":  compressorCapabilitiesMap(model.Stage.Capabilities),
		},
		"auxiliary_stages": aux,
	}, ""
}

func classifyCompressorParameter(param ParameterInfo) compressorCandidate {
	text := compressorParameterText(param)
	stage := compressorSequentialStageKey(text)
	candidate := compressorCandidate{Param: param, Path: compressorScopedPathKey(text, stage), Channel: compressorChannel(text), Stage: stage}
	if compressorContains(text, "idr", "dither", "quantize", "noise shaping", "bit depth") {
		return candidate
	}
	if compressorSidechainText(text) {
		candidate.Role, candidate.Section, candidate.Evidence = compressorDetectorRole(text), "detector", 1
		return candidate
	}
	if kind := compressorAuxiliaryKind(text); kind != "" {
		candidate.AuxKind = kind
		return candidate
	}

	switch {
	case compressorContains(text, "sidechain", "side chain", "sc filter", "sc hpf", "sc hp", "sc-hp", "detector", "detection", "channel link", "stereo link", "comp mode", "compressor type", "thrust") || compressorHasToken(text, "link"):
		candidate.Role, candidate.Section, candidate.Evidence = compressorDetectorRole(text), "detector", 1
	case compressorContains(text, "peak reduction", "gain reduction", "compression amount", "compress amount", "pressure"):
		candidate.Role, candidate.Section, candidate.Evidence = "reduction_amount", "operating_point", 3
	case compressorContains(text, "low level"):
		candidate.Role, candidate.Section, candidate.Path, candidate.Evidence = "low_level_amount", "operating_point", "low_level", 3
	case compressorContains(text, "high level"):
		candidate.Role, candidate.Section, candidate.Path, candidate.Evidence = "high_level_amount", "operating_point", "high_level", 3
	case compressorHasToken(text, "threshold") || compressorHasToken(text, "thresh"):
		candidate.Role, candidate.Section, candidate.Evidence = "threshold", "operating_point", 3
	case compressorInputDriveName(text):
		candidate.Role, candidate.Section, candidate.Evidence = "input_drive", "operating_point", 2
	case compressorHasToken(text, "ratio"):
		candidate.Role, candidate.Section, candidate.Evidence = "ratio", "transfer", 3
	case compressorHasToken(text, "compression"):
		candidate.Role, candidate.Section, candidate.Evidence = compressorBareCompressionRole(param)
	case compressorHasToken(text, "knee"):
		candidate.Role, candidate.Section, candidate.Evidence = "knee", "transfer", 2
	case compressorHasToken(text, "style") || compressorHasToken(text, "type") || compressorHasToken(text, "revision"):
		candidate.Role, candidate.Section, candidate.Evidence = "transfer_mode", "transfer", 1
	case compressorContains(text, "comp exp", "comp/exp", "compression expansion", "stress"):
		candidate.Role, candidate.Section, candidate.Evidence = "direction_curve", "transfer", 3
	case compressorHasToken(text, "attack"):
		candidate.Role, candidate.Section, candidate.Evidence = "attack", "timing", 2
	case compressorContains(text, "auto release"):
		candidate.Role, candidate.Section, candidate.Evidence = "auto_release", "timing", 1
	case compressorHasToken(text, "release"):
		candidate.Role, candidate.Section, candidate.Evidence = "release", "timing", 2
	case compressorHasToken(text, "recovery"):
		candidate.Role, candidate.Section, candidate.Evidence = "recovery", "timing", 2
	case compressorContains(text, "time constant"):
		candidate.Role, candidate.Section, candidate.Evidence = "time_constant", "timing", 2
	case compressorHasToken(text, "response"):
		candidate.Role, candidate.Section, candidate.Evidence = "response", "timing", 2
	case compressorHasToken(text, "pdr"):
		candidate.Role, candidate.Section, candidate.Evidence = "pdr_time", "timing", 2
	case compressorHasToken(text, "lookahead") || compressorContains(text, "look ahead"):
		candidate.Role, candidate.Section, candidate.Evidence = "lookahead", "timing", 2
	case compressorHasToken(text, "punch"):
		candidate.Role, candidate.Section, candidate.Evidence = "transient_emphasis", "timing", 1
	case compressorHasToken(text, "sync") || compressorContains(text, "releasebpm", "release bpm"):
		candidate.Role, candidate.Section, candidate.Evidence = "timing_sync", "timing", 1
	case compressorContains(text, "auto hold") || compressorHasToken(text, "hold"):
		candidate.Role, candidate.Section, candidate.Evidence = "hold", "timing", 1
	case compressorHasToken(text, "range"):
		candidate.Role, candidate.Section, candidate.Evidence = "reduction_range", "gain_action", 2
	case compressorOutputName(text):
		candidate.Role, candidate.Section, candidate.Evidence = "output_gain", "output", 1
	case compressorContains(text, "auto gain", "auto makeup", "automatic gain"):
		candidate.Role, candidate.Section, candidate.Evidence = "auto_makeup", "gain_action", 1
	case compressorContains(text, "makeup", "make up"):
		candidate.Role, candidate.Section, candidate.Evidence = "makeup_gain", "gain_action", 2
	case strings.TrimSpace(text) == "gain":
		candidate.Role, candidate.Section, candidate.Evidence = "output_gain", "output", 1
	case compressorContains(text, "wet gain"):
		candidate.Role, candidate.Section, candidate.Evidence = "wet_gain", "output", 1
	case compressorContains(text, "dry gain"):
		candidate.Role, candidate.Section, candidate.Evidence = "dry_gain", "output", 1
	case compressorMixName(text):
		candidate.Role, candidate.Section, candidate.Evidence = "mix", "output", 1
	}
	return candidate
}

func compressorParameterText(param ParameterInfo) string {
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
	text := strings.Join(parts, " ")
	if text == "" {
		text = param.NormalizedRole
	}
	return strings.ToLower(text)
}

func compressorBareCompressionRole(param ParameterInfo) (string, string, int) {
	unit := ""
	if param.DisplayDomainCandidate != nil {
		unit = strings.ToLower(strings.TrimSpace(param.DisplayDomainCandidate.Unit))
	}
	valueText := strings.ToLower(param.ValueText)
	if unit == "db" || strings.Contains(valueText, "db") || strings.Contains(valueText, "%") {
		return "reduction_amount", "operating_point", 3
	}
	return "ratio", "transfer", 2
}

func compressorAuxiliaryKind(text string) string {
	switch {
	case compressorContains(text, "de-esser", "de esser", "deesser", "s-reduction", "s reduction"):
		return "de_esser"
	case compressorContains(text, "k comp"):
		return "frequency_selective_compressor"
	case compressorContains(text, "multiband", "cross over", "crossover"):
		return "multiband"
	case compressorContains(text, "clipper", "clipping", "clip threshold", "clip ceiling"):
		return "clipper"
	case compressorContains(text, "limiter", "limiting", "limit ceiling", "true peak ceiling") || compressorHasToken(text, "ceiling"):
		return "limiter"
	case compressorContains(text, "gate", "expander", "expansion") && !compressorContains(text, "comp exp", "comp/exp", "compression expansion"):
		return "gate_expander"
	}
	return ""
}

func compressorSidechainText(text string) bool {
	return compressorContains(text, "sidechain", "side chain", "sc eq", "sc filter", "sc hpf", "sc hp", "sc-hp") ||
		strings.HasPrefix(strings.TrimSpace(text), "sc ")
}

func compressorDetectorRole(text string) string {
	switch {
	case compressorContains(text, "hpf", "sc hp", "sc-hp", "high pass", "low cut", "frequency", "freq"):
		return "sidechain_filter"
	case compressorContains(text, "link"):
		return "channel_link"
	default:
		return "detector_mode"
	}
}

func compressorInputDriveName(text string) bool {
	if compressorContains(text, "sidechain", "side chain", "output", "makeup") ||
		compressorHasToken(text, "pan") || compressorHasToken(text, "display") {
		return false
	}
	return compressorHasToken(text, "input") || compressorContains(text, "input control", "input gain", "input trim", "drive")
}

func compressorOutputName(text string) bool {
	if compressorHasToken(text, "pan") || compressorHasToken(text, "display") {
		return false
	}
	return compressorHasToken(text, "output") || compressorHasToken(text, "trim") ||
		compressorContains(text, "output gain", "output level", "output attenuation", "out gain", "out level")
}

func compressorMixName(text string) bool {
	if compressorHasToken(text, "pan") {
		return false
	}
	return compressorHasToken(text, "mix") || compressorContains(text, "dry wet", "dry/wet", "parallel")
}

func compressorPathKey(text string) string {
	switch {
	case compressorContains(text, "low level"):
		return "low_level"
	case compressorContains(text, "high level"):
		return "high_level"
	case compressorLeftMidToken.MatchString(text):
		return "left_mid"
	case compressorRightSideToken.MatchString(text):
		return "right_side"
	case compressorLeftRightToken.MatchString(text):
		return "main"
	case compressorContains(text, " left", "left ") || compressorLeftToken.MatchString(text):
		return "left"
	case compressorContains(text, " right", "right ") || compressorRightToken.MatchString(text):
		return "right"
	case compressorContains(text, " mid", "mid ") || compressorMidToken.MatchString(text):
		return "mid"
	case compressorContains(text, " side", "side ") || compressorSideToken.MatchString(text):
		return "side"
	default:
		return "main"
	}
}

func compressorChannel(text string) string {
	switch compressorPathKey(text) {
	case "left", "right", "mid", "side", "left_mid", "right_side":
		return compressorPathKey(text)
	default:
		return "shared"
	}
}

func compressorSequentialStageKey(text string) string {
	switch {
	case compressorHasToken(text, "optical") || compressorHasToken(text, "opto"):
		return "optical"
	case compressorHasToken(text, "discrete"):
		return "discrete"
	default:
		return ""
	}
}

func compressorScopedPathKey(text, stage string) string {
	path := compressorPathKey(text)
	if stage == "" {
		return path
	}
	if path == "main" {
		return "stage_" + stage
	}
	return "stage_" + stage + "_" + path
}

func buildCompressorPaths(candidates []compressorCandidate) []CompressorControlPath {
	byPath := map[string][]compressorCandidate{}
	for _, candidate := range candidates {
		if candidate.Section == "output" {
			continue
		}
		byPath[candidate.Path] = append(byPath[candidate.Path], candidate)
	}
	paths := make([]CompressorControlPath, 0, len(byPath))
	for key, rows := range byPath {
		path := CompressorControlPath{Key: key, Channel: rows[0].Channel, CompressionStage: rows[0].Stage, Direction: "downward"}
		for _, row := range rows {
			binding := compressorBinding(row)
			switch row.Section {
			case "detector":
				path.Detector = append(path.Detector, binding)
			case "operating_point":
				path.OperatingPoint = append(path.OperatingPoint, binding)
			case "transfer":
				path.Transfer = append(path.Transfer, binding)
				if row.Role == "direction_curve" || (row.Role == "ratio" && compressorBindingSpansUnity(binding)) {
					path.Direction = "bidirectional"
				}
			case "timing":
				path.Timing = append(path.Timing, binding)
			case "gain_action":
				path.GainAction = append(path.GainAction, binding)
			}
		}
		path.Capabilities = capabilitiesForCompressorPath(path)
		sortCompressorBindings(&path.Detector)
		sortCompressorBindings(&path.OperatingPoint)
		sortCompressorBindings(&path.Transfer)
		sortCompressorBindings(&path.Timing)
		sortCompressorBindings(&path.GainAction)
		paths = append(paths, path)
	}
	paths = coalesceCompressorAncillaryPaths(paths)
	for index := range paths {
		path := &paths[index]
		path.Capabilities = capabilitiesForCompressorPath(*path)
		sortCompressorBindings(&path.Detector)
		sortCompressorBindings(&path.OperatingPoint)
		sortCompressorBindings(&path.Transfer)
		sortCompressorBindings(&path.Timing)
		sortCompressorBindings(&path.GainAction)
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Key < paths[j].Key })
	return paths
}

func coalesceCompressorAncillaryPaths(paths []CompressorControlPath) []CompressorControlPath {
	index := map[string]int{}
	for position := range paths {
		index[paths[position].Key] = position
	}
	removed := map[int]bool{}
	merge := func(source, target int) {
		paths[target].Detector = append(paths[target].Detector, paths[source].Detector...)
		paths[target].OperatingPoint = append(paths[target].OperatingPoint, paths[source].OperatingPoint...)
		paths[target].Transfer = append(paths[target].Transfer, paths[source].Transfer...)
		paths[target].Timing = append(paths[target].Timing, paths[source].Timing...)
		paths[target].GainAction = append(paths[target].GainAction, paths[source].GainAction...)
		removed[source] = true
	}
	for sourceKey, targetKey := range map[string]string{"left": "left_mid", "right": "right_side"} {
		source, sourceOK := index[sourceKey]
		target, targetOK := index[targetKey]
		if sourceOK && targetOK && compressorPathIsAncillaryOnly(paths[source]) {
			merge(source, target)
		}
	}
	main, hasMain := index["main"]
	if hasMain && !removed[main] && len(paths[main].OperatingPoint) == 0 {
		targets := []int{}
		for position := range paths {
			if position != main && !removed[position] && len(paths[position].OperatingPoint) > 0 {
				targets = append(targets, position)
			}
		}
		if len(targets) > 0 {
			for _, target := range targets {
				merge(main, target)
			}
			removed[main] = true
		}
	}
	coreTargets := []int{}
	for position := range paths {
		if !removed[position] && compressorPathHasCore(paths[position]) {
			coreTargets = append(coreTargets, position)
		}
	}
	if len(coreTargets) == 1 {
		target := coreTargets[0]
		for source := range paths {
			if source != target && !removed[source] && compressorPathIsAncillaryOnly(paths[source]) {
				merge(source, target)
			}
		}
	}
	if hasMain && !removed[main] && compressorPathIsAncillaryOnly(paths[main]) {
		targets := []int{}
		for position := range paths {
			if position != main && !removed[position] && compressorPathHasCore(paths[position]) {
				targets = append(targets, position)
			}
		}
		if len(targets) > 0 {
			for _, target := range targets {
				merge(main, target)
			}
			removed[main] = true
		}
	}
	out := make([]CompressorControlPath, 0, len(paths)-len(removed))
	for position, path := range paths {
		if !removed[position] {
			out = append(out, path)
		}
	}
	return out
}

func compressorPathIsAncillaryOnly(path CompressorControlPath) bool {
	return len(path.Detector) > 0 && !compressorPathHasCore(path)
}

func compressorPathHasCore(path CompressorControlPath) bool {
	return len(path.OperatingPoint) > 0 || len(path.Transfer) > 0 || len(path.Timing) > 0 || len(path.GainAction) > 0
}

func collectCompressorOutput(candidates []compressorCandidate) []CompressorBinding {
	out := []CompressorBinding{}
	for _, candidate := range candidates {
		if candidate.Section == "output" {
			out = append(out, compressorBinding(candidate))
		}
	}
	sortCompressorBindings(&out)
	return out
}

func compressorBinding(candidate compressorCandidate) CompressorBinding {
	param := candidate.Param
	binding := CompressorBinding{ParamID: param.ID, Name: parameterDisplayName(param), Role: candidate.Role,
		Channel: candidate.Channel, CompressionStage: candidate.Stage, CurrentText: param.ValueText, CurrentNormalized: anyFloat(param.NormalizedValue),
		Domain: param.DisplayDomainCandidate, Curve: compressorCurveFromProbe(param, candidate.Role), Reachable: compressorReachableValues(param, candidate.Role)}
	if len(binding.Curve) == 0 && compressorProbeHasNonFiniteSentinel(param.DisplayProbe) {
		// A finite display-domain inferred after dropping -INF/+INF does not
		// prove that normalized 0 or 1 maps to that finite endpoint.
		binding.Domain = nil
	}
	if value, ok := parseCompressorPhysical(candidate.Role, param.ValueText); ok {
		binding.CurrentPhysical = &value
	}
	binding.PhysicalUnit = compressorBindingPhysicalUnit(param, candidate.Role, binding.Domain)
	binding.TransactionalProbe = compressorBindingNeedsTransactionalProbe(param, binding)
	return binding
}

func compressorBindingPhysicalUnit(param ParameterInfo, role string, domain *PluginDisplayDomain) string {
	if domain != nil && strings.TrimSpace(domain.Unit) != "" {
		return domain.Unit
	}
	if unit := inferDisplayDomainUnit(param.ValueText); unit != "" {
		return unit
	}
	if param.DisplayProbe != nil {
		if unit := inferDisplayDomainUnit(param.DisplayProbe.Label); unit != "" {
			return unit
		}
	}
	switch role {
	case "ratio", "direction_curve":
		return "ratio"
	case "attack", "release", "recovery", "time_constant", "pdr_time", "lookahead", "hold":
		return "ms"
	}
	return ""
}

func compressorBindingNeedsTransactionalProbe(param ParameterInfo, binding CompressorBinding) bool {
	if binding.CurrentPhysical == nil || binding.Domain != nil || len(binding.Curve) > 0 || len(binding.Reachable) > 0 ||
		param.DisplayProbe == nil || len(param.DisplayProbe.Samples) < 3 {
		return false
	}
	matching := 0
	for _, sample := range param.DisplayProbe.Samples {
		value, ok := parseCompressorPhysical(binding.Role, sample.Text)
		if ok && nearlyEqual(value, *binding.CurrentPhysical) {
			matching++
		}
	}
	if matching < 3 {
		return false
	}
	return binding.PhysicalUnit != ""
}

func compressorReachableValues(param ParameterInfo, role string) []CompressorReachableValue {
	if param.DisplayProbe == nil {
		return nil
	}
	out := []CompressorReachableValue{}
	for _, row := range param.DisplayProbe.DiscreteLabels {
		value := CompressorReachableValue{Normalized: anyFloat(row.Value), Label: row.Label}
		if physical, ok := parseCompressorPhysical(role, row.Label); ok {
			value.Physical = &physical
		}
		out = append(out, value)
	}
	if len(out) == 0 && param.NumSteps > 1 && len(param.DisplayProbe.AllLabels) == param.NumSteps {
		for index, label := range param.DisplayProbe.AllLabels {
			value := CompressorReachableValue{Normalized: float64(index) / float64(param.NumSteps-1), Label: label}
			if physical, ok := parseCompressorPhysical(role, label); ok {
				value.Physical = &physical
			}
			out = append(out, value)
		}
	}
	return out
}

func parseCompressorPhysical(role, text string) (float64, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return 0, false
	}
	if role == "ratio" || role == "direction_curve" {
		parts := strings.Split(lower, ":")
		if len(parts) == 2 {
			left, leftErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			right, rightErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if leftErr == nil && rightErr == nil && right != 0 {
				return left / right, true
			}
		}
	}
	value, _, ok := parseProbeDisplayNumber(text, "")
	if !ok {
		return 0, false
	}
	if (role == "attack" || role == "release" || role == "recovery" || role == "time_constant" || role == "pdr_time" || role == "lookahead" || role == "hold") &&
		(strings.Contains(lower, " sec") || strings.HasSuffix(lower, "s")) && !strings.Contains(lower, "ms") {
		return value * 1000, true
	}
	return value, true
}

func ParseCompressorPhysical(role, text string) (float64, bool) {
	return parseCompressorPhysical(role, text)
}

func compressorCurveFromProbe(param ParameterInfo, role string) [][2]float64 {
	if param.DisplayProbe == nil || len(param.DisplayProbe.Samples) < 3 {
		return nil
	}
	curve := make([][2]float64, 0, len(param.DisplayProbe.Samples))
	firstNumeric, lastNumeric := -1, -1
	for index, sample := range param.DisplayProbe.Samples {
		physical, ok := parseCompressorPhysical(role, sample.Text)
		if !ok {
			if compressorDisplayIsNonFiniteSentinel(sample.Text) {
				continue
			}
			// Some continuous controls reserve their normalized endpoints for
			// named states (for example Clean/Dirty) while exposing a numeric
			// interior. Interior labels still invalidate the numeric curve.
			continue
		}
		if firstNumeric < 0 {
			firstNumeric = index
		}
		lastNumeric = index
		curve = append(curve, [2]float64{sample.NormalizedValue, physical})
	}
	if len(curve) < 3 {
		return nil
	}
	for index := firstNumeric; index <= lastNumeric; index++ {
		if _, ok := parseCompressorPhysical(role, param.DisplayProbe.Samples[index].Text); !ok &&
			!compressorDisplayIsNonFiniteSentinel(param.DisplayProbe.Samples[index].Text) {
			return nil
		}
	}
	direction := 0
	for index := 1; index < len(curve); index++ {
		if curve[index][0] <= curve[index-1][0] || curve[index][1] == curve[index-1][1] {
			return nil
		}
		step := 1
		if curve[index][1] < curve[index-1][1] {
			step = -1
		}
		if direction == 0 {
			direction = step
		} else if direction != step {
			return nil
		}
	}
	return curve
}

func compressorProbeHasNonFiniteSentinel(probe *ParameterDisplayProbe) bool {
	if probe == nil {
		return false
	}
	for _, sample := range probe.Samples {
		if compressorDisplayIsNonFiniteSentinel(sample.Text) {
			return true
		}
	}
	return false
}

func compressorDisplayIsNonFiniteSentinel(text string) bool {
	compact := strings.ToLower(strings.TrimSpace(text))
	compact = strings.ReplaceAll(compact, " ", "")
	return strings.Contains(compact, "-inf") || strings.Contains(compact, "+inf")
}

func compressorBindingSpansUnity(binding CompressorBinding) bool {
	minimum, maximum := math.Inf(1), math.Inf(-1)
	observe := func(value float64) {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return
		}
		minimum = math.Min(minimum, value)
		maximum = math.Max(maximum, value)
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
	return minimum < 1 && maximum > 1
}

func compressorUnsupportedBoundary(digest ParameterDigest, candidates []compressorCandidate, auxByKind map[string][]string) string {
	if compressorHasPureClipperSurface(digest, candidates, auxByKind) {
		return "unsupported_clipper"
	}
	if compressorHasMultipleFrequencyBands(candidates) {
		return "unsupported_multiband_compressor"
	}
	if compressorHasSpectralDynamicsSurface(digest) {
		return "unsupported_spectral_dynamics"
	}
	if len(auxByKind["de_esser"]) > 0 {
		return "unsupported_de_esser"
	}
	if len(auxByKind["gate_expander"]) > 0 {
		return "unsupported_gate_expander"
	}
	if len(auxByKind["limiter"]) > 0 {
		return "unsupported_limiter"
	}
	return ""
}

// compressorHasPureClipperSurface recognizes the indexed ceiling/knee/type
// surface of a clipper before the weak input-driven fixed-transfer heuristic.
// A real broadband compressor path remains eligible when it also exposes a
// threshold, reduction amount, ratio, or timing operating point.
func compressorHasPureClipperSurface(digest ParameterDigest, candidates []compressorCandidate, auxByKind map[string][]string) bool {
	if len(auxByKind["limiter"]) == 0 {
		return false
	}
	hasInput, hasKneeOrMode := false, false
	hasCompressorCore := false
	ceilingCells := 0
	for _, candidate := range candidates {
		switch candidate.Role {
		case "input_drive":
			hasInput = true
		case "knee", "transfer_mode":
			hasKneeOrMode = true
		case "threshold", "reduction_amount", "low_level_amount", "high_level_amount", "ratio",
			"direction_curve", "attack", "release", "recovery", "time_constant", "pdr_time",
			"lookahead", "hold":
			hasCompressorCore = true
		}
	}
	for _, param := range digest.Parameters {
		text := compressorParameterText(param)
		if compressorHasToken(text, "ceiling") || compressorContains(text, "clip ceiling", "clip threshold") {
			ceilingCells++
		}
	}
	return hasInput && hasKneeOrMode && !hasCompressorCore && ceilingCells > 0
}

func compressorHasMultipleFrequencyBands(candidates []compressorCandidate) bool {
	type bandFacts struct {
		operatingPoint bool
		transfer       bool
		timing         bool
	}
	bands := map[string]bandFacts{}
	for _, candidate := range candidates {
		key := compressorFrequencyBandKey(compressorParameterText(candidate.Param))
		if key == "" || candidate.Section == "detector" || candidate.Section == "output" {
			continue
		}
		facts := bands[key]
		switch candidate.Section {
		case "operating_point":
			facts.operatingPoint = facts.operatingPoint || candidate.Role == "threshold" || candidate.Role == "reduction_amount"
		case "transfer":
			facts.transfer = facts.transfer || candidate.Role == "ratio" || candidate.Role == "knee" || candidate.Role == "direction_curve"
		case "timing":
			facts.timing = facts.timing || candidate.Role == "attack" || candidate.Role == "release" || candidate.Role == "recovery" || candidate.Role == "time_constant"
		}
		bands[key] = facts
	}
	complete := 0
	for _, facts := range bands {
		if facts.operatingPoint && (facts.transfer || facts.timing) {
			complete++
		}
	}
	return complete > 1
}

func compressorFrequencyBandKey(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" || compressorContains(text, "low level", "high level") || compressorSidechainText(text) {
		return ""
	}
	if match := compressorBandToken.FindStringSubmatch(text); len(match) == 2 {
		return "band_" + match[1]
	}
	for _, token := range []string{"low", "mid", "high"} {
		if compressorHasToken(text, token) {
			return token
		}
	}
	return ""
}

func compressorHasSpectralDynamicsSurface(digest ParameterDigest) bool {
	thresholds := map[string]bool{}
	frequencyShape := map[string]bool{}
	indexedCompressionCells := map[string]bool{}
	for _, param := range digest.Parameters {
		text := compressorParameterText(param)
		match := compressorIndexedSuffixToken.FindStringSubmatch(text)
		if len(match) != 2 {
			continue
		}
		index := match[1]
		if compressorHasToken(text, "threshold") || compressorHasToken(text, "thresh") {
			thresholds[index] = true
		}
		if compressorHasToken(text, "frequency") || compressorHasToken(text, "freq") || compressorHasToken(text, "q") {
			frequencyShape[index] = true
		}
		if compressorHasToken(text, "ratio") || compressorHasToken(text, "attack") || compressorHasToken(text, "release") ||
			compressorHasToken(text, "recovery") || compressorHasToken(text, "knee") {
			indexedCompressionCells[index] = true
		}
	}
	intersections := 0
	for index := range thresholds {
		if frequencyShape[index] {
			intersections++
		}
	}
	completeIndexedCells := 0
	for index := range thresholds {
		if indexedCompressionCells[index] {
			completeIndexedCells++
		}
	}
	return intersections >= 2 && completeIndexedCells < 2
}

func compressorPathsHaveStageEvidence(paths []CompressorControlPath, output []CompressorBinding) bool {
	for _, path := range paths {
		op, transfer, timing := len(path.OperatingPoint), len(path.Transfer), len(path.Timing)
		if op > 0 && (transfer > 0 || timing > 0) {
			// A reduction/pressure control is itself a valid operating point for
			// fixed-transfer levelers, even when a separate Drive control is also
			// present. Do this before the input-driven special case below so the
			// amount evidence cannot be discarded by its continue path.
			if hasCompressorRole(path.OperatingPoint, "reduction_amount") ||
				hasCompressorRole(path.OperatingPoint, "low_level_amount") ||
				hasCompressorRole(path.OperatingPoint, "high_level_amount") {
				return true
			}
			if hasCompressorRole(path.OperatingPoint, "input_drive") && transfer == 0 {
				if hasCompressorRole(path.OperatingPoint, "threshold") {
					return true
				}
				if hasCompressorRole(path.Timing, "response") && hasCompressorRole(output, "output_gain") && hasCompressorRole(output, "mix") {
					return true
				}
				if timing > 0 && hasCompressorRole(path.Detector, "detector_mode") {
					return true
				}
				continue
			}
			return true
		}
		for _, binding := range path.OperatingPoint {
			if binding.Role == "reduction_amount" || binding.Role == "low_level_amount" || binding.Role == "high_level_amount" {
				return true
			}
		}
	}
	return false
}

func compressorPathsHaveIndependentTransferEvidence(paths []CompressorControlPath) bool {
	for _, path := range paths {
		for _, binding := range path.Transfer {
			if binding.Role == "ratio" || binding.Role == "knee" || binding.Role == "direction_curve" {
				return true
			}
		}
		for _, binding := range path.OperatingPoint {
			if binding.Role == "reduction_amount" || binding.Role == "low_level_amount" || binding.Role == "high_level_amount" {
				return true
			}
		}
		if hasCompressorRole(path.OperatingPoint, "input_drive") &&
			(hasCompressorRole(path.Timing, "response") || hasCompressorRole(path.Detector, "detector_mode")) {
			return true
		}
	}
	return false
}

func classifyCompressorModel(paths []CompressorControlPath, aux []CompressorAuxiliaryStage) string {
	if len(paths) > 1 {
		for _, path := range paths {
			if path.Key == "low_level" || path.Key == "high_level" {
				return "multi_path_level_control"
			}
		}
		stages := map[string]bool{}
		for _, path := range paths {
			if path.CompressionStage != "" {
				stages[path.CompressionStage] = true
			}
		}
		if len(stages) > 1 {
			return "multi_stage_serial_control"
		}
		return "multi_path_channel_control"
	}
	if len(paths) == 0 {
		return "unknown"
	}
	path := paths[0]
	if path.Direction == "bidirectional" {
		return "bidirectional_curve"
	}
	if hasCompressorRole(path.OperatingPoint, "threshold") {
		return "threshold_driven"
	}
	if hasCompressorRole(path.OperatingPoint, "input_drive") && hasCompressorRole(path.Transfer, "ratio") {
		return "input_driven_ratio"
	}
	if hasCompressorRole(path.OperatingPoint, "input_drive") &&
		(hasCompressorRole(path.Timing, "response") || hasCompressorRole(path.Transfer, "transfer_mode") ||
			hasCompressorRole(path.Detector, "detector_mode")) {
		return "input_driven_fixed_transfer"
	}
	if hasCompressorRole(path.OperatingPoint, "reduction_amount") {
		return "amount_driven"
	}
	if len(aux) > 0 {
		return "hybrid"
	}
	return "compound_operating_point"
}

func compressorModelConfidence(paths []CompressorControlPath) float64 {
	score := 0
	for _, path := range paths {
		score += len(path.OperatingPoint)*3 + len(path.Transfer)*2 + len(path.Timing)
	}
	switch {
	case score >= 9:
		return 0.96
	case score >= 6:
		return 0.90
	default:
		return 0.82
	}
}

func capabilitiesForCompressorPath(path CompressorControlPath) CompressorCapabilities {
	return CompressorCapabilities{
		Detector:       compressorRoles(path.Detector),
		OperatingPoint: compressorRoles(path.OperatingPoint),
		Transfer:       compressorRoles(path.Transfer),
		Timing:         compressorRoles(path.Timing),
		GainAction:     compressorRoles(path.GainAction),
	}
}

func aggregateCompressorCapabilities(paths []CompressorControlPath, output []CompressorBinding) CompressorCapabilities {
	out := CompressorCapabilities{Output: compressorRoles(output)}
	for _, path := range paths {
		out.Detector = append(out.Detector, path.Capabilities.Detector...)
		out.OperatingPoint = append(out.OperatingPoint, path.Capabilities.OperatingPoint...)
		out.Transfer = append(out.Transfer, path.Capabilities.Transfer...)
		out.Timing = append(out.Timing, path.Capabilities.Timing...)
		out.GainAction = append(out.GainAction, path.Capabilities.GainAction...)
	}
	out.Detector = uniqueCompressorStrings(out.Detector)
	out.OperatingPoint = uniqueCompressorStrings(out.OperatingPoint)
	out.Transfer = uniqueCompressorStrings(out.Transfer)
	out.Timing = uniqueCompressorStrings(out.Timing)
	out.GainAction = uniqueCompressorStrings(out.GainAction)
	return out
}

func compressorRoles(bindings []CompressorBinding) []string {
	out := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, binding.Role)
	}
	return uniqueCompressorStrings(out)
}

func compressorTopologyGeneration(model *CompressorModel) string {
	type bindingFact struct {
		ParamID, Role, Channel, CompressionStage, PhysicalUnit string
		Domain                                                 *PluginDisplayDomain
		Reachable                                              []CompressorReachableValue
		TransactionalProbe                                     bool
	}
	type pathFact struct {
		Key, Channel, CompressionStage, Direction string
		Bindings                                  []bindingFact
	}
	payload := struct {
		Schema string
		Paths  []pathFact
		Output []bindingFact
		Aux    []CompressorAuxiliaryStage
	}{Schema: compressorTopologySchema, Aux: model.AuxiliaryStages}
	for _, path := range model.Stage.ControlPaths {
		fact := pathFact{Key: path.Key, Channel: path.Channel, CompressionStage: path.CompressionStage, Direction: path.Direction}
		all := append([]CompressorBinding{}, path.Detector...)
		all = append(all, path.OperatingPoint...)
		all = append(all, path.Transfer...)
		all = append(all, path.Timing...)
		all = append(all, path.GainAction...)
		for _, binding := range all {
			fact.Bindings = append(fact.Bindings, bindingFact{binding.ParamID, binding.Role, binding.Channel, binding.CompressionStage, binding.PhysicalUnit,
				binding.Domain, binding.Reachable, binding.TransactionalProbe})
		}
		sort.Slice(fact.Bindings, func(i, j int) bool { return fact.Bindings[i].ParamID < fact.Bindings[j].ParamID })
		payload.Paths = append(payload.Paths, fact)
	}
	for _, binding := range model.Stage.Output {
		payload.Output = append(payload.Output, bindingFact{binding.ParamID, binding.Role, binding.Channel, binding.CompressionStage, binding.PhysicalUnit,
			binding.Domain, binding.Reachable, binding.TransactionalProbe})
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return "c2t1_" + hex.EncodeToString(sum[:12])
}

func compressorPathSummary(path CompressorControlPath) map[string]any {
	return map[string]any{
		"path_key":          path.Key,
		"channel":           path.Channel,
		"compression_stage": path.CompressionStage,
		"direction":         path.Direction,
		"detector":          compressorBindingRows(path.Detector),
		"operating_point":   compressorBindingRows(path.OperatingPoint),
		"transfer":          compressorBindingRows(path.Transfer),
		"timing":            compressorBindingRows(path.Timing),
		"gain_action":       compressorBindingRows(path.GainAction),
		"capabilities":      compressorCapabilitiesMap(path.Capabilities),
	}
}

func compressorBindingRows(bindings []CompressorBinding) []map[string]any {
	out := make([]map[string]any, 0, len(bindings))
	for _, binding := range bindings {
		row := map[string]any{"param_id": binding.ParamID, "name": binding.Name, "role": binding.Role,
			"channel": binding.Channel, "current_normalized": binding.CurrentNormalized, "current_text": binding.CurrentText}
		if binding.CompressionStage != "" {
			row["compression_stage"] = binding.CompressionStage
		}
		if binding.PhysicalUnit != "" {
			row["physical_unit"] = binding.PhysicalUnit
		}
		if binding.TransactionalProbe {
			row["transactional_probe_required"] = true
		}
		if binding.CurrentPhysical != nil {
			row["current_physical"] = *binding.CurrentPhysical
		}
		if binding.Domain != nil {
			row["domain"] = displayDomainMap(binding.Domain)
		}
		if len(binding.Curve) > 0 {
			row["curve"] = binding.Curve
		}
		if len(binding.Reachable) > 0 {
			reachable := make([]map[string]any, 0, len(binding.Reachable))
			for _, value := range binding.Reachable {
				item := map[string]any{"normalized": value.Normalized, "label": value.Label}
				if value.Physical != nil {
					item["physical"] = *value.Physical
				}
				reachable = append(reachable, item)
			}
			row["reachable_values"] = reachable
		}
		out = append(out, row)
	}
	return out
}

func compressorCapabilitiesMap(capabilities CompressorCapabilities) map[string]any {
	return map[string]any{"detector": capabilities.Detector, "operating_point": capabilities.OperatingPoint,
		"transfer": capabilities.Transfer, "timing": capabilities.Timing, "gain_action": capabilities.GainAction,
		"output": capabilities.Output}
}

func sortCompressorBindings(bindings *[]CompressorBinding) {
	sort.Slice(*bindings, func(i, j int) bool {
		return (*bindings)[i].Role+"/"+(*bindings)[i].ParamID < (*bindings)[j].Role+"/"+(*bindings)[j].ParamID
	})
}

func hasCompressorRole(bindings []CompressorBinding, role string) bool {
	for _, binding := range bindings {
		if binding.Role == role {
			return true
		}
	}
	return false
}

func uniqueCompressorStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func compressorContains(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func compressorHasToken(text, token string) bool {
	for _, field := range strings.FieldsFunc(text, func(r rune) bool { return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') }) {
		if field == token {
			return true
		}
	}
	return false
}

func (model CompressorModel) String() string {
	return fmt.Sprintf("%s paths=%d generation=%s", model.Classification, len(model.Stage.ControlPaths), model.Generation)
}
