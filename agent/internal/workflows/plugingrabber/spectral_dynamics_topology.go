package plugingrabber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

const spectralDynamicsTopologySchema = "spectral-dynamics-control-topology/v1"

// SpectralDynamicsModel is intentionally independent from the broadband
// compressor model. A positive result requires a frequency-indexed field and
// one shared, non-indexed dynamics law. Current values never affect detection
// or topology generation.
type SpectralDynamicsModel struct {
	Classification string
	Confidence     float64
	Field          SpectralField
	GlobalLaw      SpectralGlobalLaw
	Auxiliary      []SpectralAuxiliaryStage
	Observation    SpectralObservationModel
	Generation     string
}

type SpectralField struct {
	Frequency      []SpectralBinding
	Threshold      []SpectralBinding
	Q              []SpectralBinding
	DynamicRange   []SpectralBinding
	MatchedIndices []string
	IndexedCount   int
	Dense          bool
	FieldEvidence  []string
	VetoEvidence   []string
}

type SpectralGlobalLaw struct {
	OperatingPoint []SpectralBinding
	Transfer       []SpectralBinding
	Timing         []SpectralBinding
	Detector       []SpectralBinding
	Mode           []SpectralBinding
}

type SpectralBinding struct {
	ParamID           string
	Name              string
	Role              string
	Index             string
	PhysicalUnit      string
	CurrentText       string
	CurrentNormalized float64
	Domain            *PluginDisplayDomain
}

type SpectralAuxiliaryStage struct {
	Kind     string
	ParamIDs []string
}

type SpectralObservationModel struct {
	Required []string
	Optional []string
	Status   string
}

type spectralCandidate struct {
	Param ParameterInfo
	Role  string
	Index string
}

var spectralIndexToken = regexp.MustCompile(`(?i)(?:band|node|point|freq(?:uency)?|threshold|gain|q|attack|release)\s*([0-9]+)`)
var spectralAnyIndexToken = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])([0-9]+)(?:[^a-z0-9]|$)`)

func DetectSpectralDynamicsModel(digest ParameterDigest) *SpectralDynamicsModel {
	model, _ := DetectSpectralDynamicsModelWithBoundary(digest)
	return model
}

func DetectSpectralDynamicsModelWithBoundary(digest ParameterDigest) (*SpectralDynamicsModel, string) {
	candidates := make([]spectralCandidate, 0, len(digest.Parameters))
	aux := map[string][]string{}
	for _, param := range digest.Parameters {
		text := spectralParameterText(param)
		if kind := spectralAuxiliaryKind(text); kind != "" {
			aux[kind] = append(aux[kind], param.ID)
			continue
		}
		role := spectralRole(text)
		if role == "" || !param.HostControllable {
			continue
		}
		candidates = append(candidates, spectralCandidate{Param: param, Role: role, Index: spectralParameterIndex(text)})
	}

	if len(aux["multiband_dynamics"]) > 0 {
		return nil, "unsupported_multiband_dynamics"
	}
	if len(aux["dynamic_eq"]) > 0 {
		return nil, "unsupported_dynamic_eq"
	}
	if len(aux["de_esser"]) > 0 {
		return nil, "unsupported_de_esser"
	}
	if len(aux["clipper"]) > 0 {
		return nil, "unsupported_clipper"
	}
	if len(aux["limiter"]) > 0 {
		return nil, "unsupported_limiter"
	}

	field := SpectralField{}
	law := SpectralGlobalLaw{}
	indexed := map[string]map[string]spectralCandidate{}
	for _, c := range candidates {
		if c.Index != "" {
			if indexed[c.Role] == nil {
				indexed[c.Role] = map[string]spectralCandidate{}
			}
			indexed[c.Role][c.Index] = c
			continue
		}
		b := spectralBinding(c)
		switch c.Role {
		case "global_threshold", "global_input":
			law.OperatingPoint = append(law.OperatingPoint, b)
		case "global_ratio", "global_expander_ratio", "global_direction", "global_knee":
			law.Transfer = append(law.Transfer, b)
		case "global_attack", "global_release", "global_hold", "global_lookahead":
			law.Timing = append(law.Timing, b)
		case "global_detector", "global_sidechain":
			law.Detector = append(law.Detector, b)
		case "global_mode", "global_mix":
			law.Mode = append(law.Mode, b)
		}
	}
	for _, c := range indexed["field_frequency"] {
		field.Frequency = append(field.Frequency, spectralBinding(c))
	}
	for _, c := range indexed["field_threshold"] {
		field.Threshold = append(field.Threshold, spectralBinding(c))
	}
	for _, c := range indexed["field_q"] {
		field.Q = append(field.Q, spectralBinding(c))
	}
	for _, c := range indexed["field_dynamic_range"] {
		field.DynamicRange = append(field.DynamicRange, spectralBinding(c))
	}
	for index := range indexed["field_frequency"] {
		if _, ok := indexed["field_threshold"][index]; ok {
			field.MatchedIndices = append(field.MatchedIndices, index)
		}
	}
	sort.Strings(field.MatchedIndices)
	field.IndexedCount = len(field.Frequency)
	field.Dense = len(field.MatchedIndices) >= 3
	if field.IndexedCount >= 3 {
		field.FieldEvidence = append(field.FieldEvidence, "indexed_frequency_field")
	}
	if len(field.Threshold) >= 3 {
		field.FieldEvidence = append(field.FieldEvidence, "indexed_threshold_field")
	}
	if len(field.Q) >= 3 {
		field.FieldEvidence = append(field.FieldEvidence, "indexed_q_field")
	}
	if len(field.DynamicRange) >= 3 {
		field.FieldEvidence = append(field.FieldEvidence, "indexed_dynamic_range_field")
	}

	if spectralLooksLikeDynamicEQ(indexed) {
		return nil, "unsupported_dynamic_eq"
	}
	if spectralLooksLikeMultiband(indexed) {
		return nil, "unsupported_multiband_dynamics"
	}
	if !field.Dense {
		if spectralHasAnyIndexedField(indexed) {
			return nil, "unresolved_spectral_dynamics_surface"
		}
		if spectralHasBroadbandLaw(law) {
			return nil, "unsupported_broadband_compressor"
		}
		return nil, "not_spectral_dynamics"
	}
	if !spectralHasGlobalLaw(law) {
		return nil, "unresolved_spectral_dynamics_surface"
	}

	for _, kind := range []string{"dynamic_eq", "de_esser", "limiter", "clipper", "multiband_dynamics"} {
		if ids := aux[kind]; len(ids) > 0 {
			sort.Strings(ids)
		}
	}
	model := &SpectralDynamicsModel{
		Classification: "spectral_field_global_dynamics",
		Confidence:     0.92,
		Field:          field,
		GlobalLaw:      law,
		Observation: SpectralObservationModel{
			Required: []string{"stft_gain_field", "frequency_bin_alignment", "time_resolution", "scope_identity", "deterministic_probe"},
			Optional: []string{"latency", "lookahead", "channel_link", "change_delta"},
			Status:   "inspect_only_observation_required",
		},
	}
	for kind, ids := range aux {
		if len(ids) == 0 {
			continue
		}
		sort.Strings(ids)
		model.Auxiliary = append(model.Auxiliary, SpectralAuxiliaryStage{Kind: kind, ParamIDs: ids})
	}
	sort.Slice(model.Auxiliary, func(i, j int) bool { return model.Auxiliary[i].Kind < model.Auxiliary[j].Kind })
	model.Generation = spectralTopologyGeneration(model)
	return model, ""
}

func BuildSpectralDynamicsSummary(digest ParameterDigest) map[string]any {
	summary, _ := BuildSpectralDynamicsSummaryWithBoundary(digest)
	return summary
}

func BuildSpectralDynamicsSummaryWithBoundary(digest ParameterDigest) (map[string]any, string) {
	model, boundary := DetectSpectralDynamicsModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	aux := make([]map[string]any, 0, len(model.Auxiliary))
	for _, stage := range model.Auxiliary {
		aux = append(aux, map[string]any{"kind": stage.Kind, "param_ids": stage.ParamIDs})
	}
	return map[string]any{
		"schema_version":   spectralDynamicsTopologySchema,
		"mapping_source":   "generic_structural",
		"classification":   model.Classification,
		"confidence":       model.Confidence,
		"control_topology": map[string]any{"schema_version": spectralDynamicsTopologySchema, "generation": model.Generation},
		"spectral_field": map[string]any{
			"indexed_count":   model.Field.IndexedCount,
			"dense":           model.Field.Dense,
			"matched_indices": model.Field.MatchedIndices,
			"frequency":       spectralBindingRows(model.Field.Frequency),
			"threshold":       spectralBindingRows(model.Field.Threshold),
			"q":               spectralBindingRows(model.Field.Q),
			"dynamic_range":   spectralBindingRows(model.Field.DynamicRange),
			"evidence":        model.Field.FieldEvidence,
		},
		"global_law": map[string]any{
			"operating_point": spectralBindingRows(model.GlobalLaw.OperatingPoint),
			"transfer":        spectralBindingRows(model.GlobalLaw.Transfer),
			"timing":          spectralBindingRows(model.GlobalLaw.Timing),
			"detector":        spectralBindingRows(model.GlobalLaw.Detector),
			"mode":            spectralBindingRows(model.GlobalLaw.Mode),
		},
		"observation_model": map[string]any{"required": model.Observation.Required, "optional": model.Observation.Optional, "status": model.Observation.Status},
		"auxiliary_stages":  aux,
	}, ""
}

func spectralParameterText(param ParameterInfo) string {
	return strings.ToLower(strings.TrimSpace(parameterDisplayName(param) + " " + param.RawName + " " + param.Alias))
}

func spectralParameterIndex(text string) string {
	if match := spectralIndexToken.FindStringSubmatch(text); len(match) == 2 {
		return match[1]
	}
	if match := spectralAnyIndexToken.FindStringSubmatch(text); len(match) == 2 {
		return match[1]
	}
	return ""
}

func spectralRole(text string) string {
	idx := spectralParameterIndex(text)
	hasIndex := idx != ""
	if hasIndex && strings.Contains(text, "frequency") || hasIndex && strings.Contains(text, "freq") {
		return "field_frequency"
	}
	if hasIndex && spectralHasToken(text, "threshold", "thresh") {
		return "field_threshold"
	}
	if hasIndex && spectralHasToken(text, "q") {
		return "field_q"
	}
	if hasIndex && spectralHasToken(text, "gain", "boost", "cut") {
		return "field_gain"
	}
	if hasIndex && spectralContains(text, "dynamic range", "dynamic-range") {
		return "field_dynamic_range"
	}
	if hasIndex && spectralHasToken(text, "attack", "release", "hold") {
		return "field_timing"
	}
	switch {
	case spectralHasToken(text, "threshold", "thresh"):
		return "global_threshold"
	case spectralContains(text, "compressor ratio", "compression ratio"):
		return "global_ratio"
	case spectralContains(text, "expander ratio", "expansion ratio"):
		return "global_expander_ratio"
	case spectralHasToken(text, "ratio"):
		return "global_ratio"
	case spectralHasToken(text, "attack"):
		return "global_attack"
	case spectralHasToken(text, "release", "recovery"):
		return "global_release"
	case spectralHasToken(text, "hold"):
		return "global_hold"
	case spectralContains(text, "lookahead", "look ahead"):
		return "global_lookahead"
	case spectralHasToken(text, "knee", "shape", "curve"):
		return "global_knee"
	case spectralContains(text, "detector", "detection", "sidechain", "side chain"):
		return "global_detector"
	case spectralContains(text, "mode", "style", "direction", "upward", "downward"):
		return "global_mode"
	case spectralHasToken(text, "mix", "dry/wet", "dry wet"):
		return "global_mix"
	}
	return ""
}

func spectralAuxiliaryKind(text string) string {
	switch {
	case spectralContains(text, "dynamic eq", "dynamic-eq"):
		return "dynamic_eq"
	case spectralContains(text, "multiband", "crossover", "cross over"):
		return "multiband_dynamics"
	case spectralContains(text, "de-esser", "de esser", "deesser", "sibilance"):
		return "de_esser"
	case spectralContains(text, "clipper", "clipping", "soft clip"):
		return "clipper"
	case spectralContains(text, "ceiling", "true peak", "truepeak", "maximizer"):
		return "limiter"
	}
	return ""
}

func spectralLooksLikeDynamicEQ(indexed map[string]map[string]spectralCandidate) bool {
	return spectralIndexIntersectionCount(indexed["field_frequency"], indexed["field_gain"], indexed["field_threshold"]) >= 2 ||
		spectralIndexIntersectionCount(indexed["field_frequency"], indexed["field_gain"], indexed["field_dynamic_range"]) >= 2
}

func spectralLooksLikeMultiband(indexed map[string]map[string]spectralCandidate) bool {
	return len(indexed["field_frequency"]) == 0 && spectralIndexIntersectionCount(indexed["field_threshold"], indexed["field_timing"]) >= 2
}

func spectralIndexIntersectionCount(groups ...map[string]spectralCandidate) int {
	if len(groups) == 0 || len(groups[0]) == 0 {
		return 0
	}
	count := 0
	for index := range groups[0] {
		matched := true
		for _, group := range groups[1:] {
			if _, ok := group[index]; !ok {
				matched = false
				break
			}
		}
		if matched {
			count++
		}
	}
	return count
}

func spectralHasAnyIndexedField(indexed map[string]map[string]spectralCandidate) bool {
	return len(indexed["field_frequency"])+len(indexed["field_threshold"])+len(indexed["field_q"])+len(indexed["field_gain"])+len(indexed["field_dynamic_range"])+len(indexed["field_timing"]) > 0
}

func spectralHasGlobalLaw(law SpectralGlobalLaw) bool {
	return len(law.OperatingPoint) > 0 && len(law.Transfer) > 0 && len(law.Timing) > 0
}

func spectralHasBroadbandLaw(law SpectralGlobalLaw) bool {
	return len(law.OperatingPoint) > 0 && (len(law.Transfer) > 0 || len(law.Timing) > 0)
}

func spectralBinding(c spectralCandidate) SpectralBinding {
	value := 0.0
	if normalized, ok := c.Param.NormalizedValue.(float64); ok {
		value = normalized
	}
	return SpectralBinding{ParamID: c.Param.ID, Name: parameterDisplayName(c.Param), Role: c.Role, Index: c.Index, PhysicalUnit: c.Param.Unit, CurrentText: c.Param.ValueText, CurrentNormalized: value, Domain: c.Param.DisplayDomainCandidate}
}

func spectralBindingRows(bindings []SpectralBinding) []map[string]any {
	rows := make([]map[string]any, 0, len(bindings))
	for _, b := range bindings {
		row := map[string]any{"param_id": b.ParamID, "name": b.Name, "role": b.Role}
		if b.Index != "" {
			row["index"] = b.Index
		}
		if b.PhysicalUnit != "" {
			row["physical_unit"] = b.PhysicalUnit
		}
		if b.CurrentText != "" {
			row["current_text"] = b.CurrentText
		}
		if b.Domain != nil {
			row["domain"] = b.Domain
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["param_id"].(string) < rows[j]["param_id"].(string) })
	return rows
}

func spectralTopologyGeneration(model *SpectralDynamicsModel) string {
	canonical := func(bindings []SpectralBinding) []map[string]string {
		rows := make([]map[string]string, 0, len(bindings))
		for _, b := range bindings {
			rows = append(rows, map[string]string{"param_id": b.ParamID, "role": b.Role, "index": b.Index})
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i]["role"] != rows[j]["role"] {
				return rows[i]["role"] < rows[j]["role"]
			}
			return rows[i]["param_id"] < rows[j]["param_id"]
		})
		return rows
	}
	shape := struct {
		Classification string
		Frequency      []map[string]string
		Threshold      []map[string]string
		Q              []map[string]string
		DynamicRange   []map[string]string
		Operating      []map[string]string
		Transfer       []map[string]string
		Timing         []map[string]string
		Detector       []map[string]string
		Mode           []map[string]string
	}{
		Classification: model.Classification,
		Frequency:      canonical(model.Field.Frequency), Threshold: canonical(model.Field.Threshold), Q: canonical(model.Field.Q), DynamicRange: canonical(model.Field.DynamicRange),
		Operating: canonical(model.GlobalLaw.OperatingPoint), Transfer: canonical(model.GlobalLaw.Transfer),
		Timing: canonical(model.GlobalLaw.Timing), Detector: canonical(model.GlobalLaw.Detector), Mode: canonical(model.GlobalLaw.Mode),
	}
	data, _ := json.Marshal(shape)
	sum := sha256.Sum256(data)
	return "sdg1_" + hex.EncodeToString(sum[:16])
}

func spectralContains(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func spectralHasToken(text string, values ...string) bool {
	for _, value := range values {
		if regexp.MustCompile(`(?i)(?:^|[^a-z0-9])` + regexp.QuoteMeta(value) + `(?:[^a-z0-9]|$)`).MatchString(text) {
			return true
		}
	}
	return false
}
