package plugingrabber

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// EQModel is the internal, strongly typed representation used by both the
// public summary and the deterministic execution path.  Detection is based on
// parameter structure and physical domains; plug-in identity is deliberately
// absent from this type.
type EQModel struct {
	Classification      string
	MappingSource       string
	Confidence          float64
	Completeness        EQCompleteness
	Capabilities        EQCapabilities
	Bands               []EQBand
	Channels            []string
	SetEQPointSupported bool
	Reason              string
}

type EQCompleteness struct {
	Complete       bool
	CompleteBands  int
	CandidateBands int
	Issues         []string
}

type EQCapabilities struct {
	FrequencyAdjustable bool
	FixedFrequency      bool
	Gain                bool
	Q                   bool
	Shape               bool
	Activation          bool
	MultiChannel        bool
}

type EQBand struct {
	Key              string
	FixedFrequencyHz *float64
	Bindings         map[string][]EQBinding
	Active           bool
	ActivationKnown  bool
	Complete         bool
	Issues           []string
}

type EQBinding struct {
	ParamID           string
	Name              string
	Role              string
	Channel           string
	ActivationKind    string
	GainPolarity      string
	CurrentText       string
	CurrentNormalized float64
	CurrentPhysical   *float64
	Domain            *PluginDisplayDomain
	Curve             [][2]float64
	Reachable         []EQReachableValue
}

type EQReachableValue struct {
	Normalized float64
	Physical   *float64
	Label      string
}

const (
	eqRoleFrequency  = "frequency"
	eqRoleGain       = "gain"
	eqRoleQ          = "q"
	eqRoleActivation = "activation"
	eqRoleShape      = "shape"
)

type eqParsedParameter struct {
	Param          ParameterInfo
	BandKey        string
	Role           string
	Channel        string
	FixedHz        *float64
	ActivationKind string
	GainPolarity   string
	Confidence     float64
}

var (
	eqCamelBoundary = regexp.MustCompile(`([a-z])([A-Z])`)
	eqTokenPattern  = regexp.MustCompile(`(?i)\d+(?:\.\d+)?\s*(?:k\s*hz|hz)?|[a-z]+`)
	eqNumberPattern = regexp.MustCompile(`[-+]?\d+(?:\.\d+)?`)
)

// DetectEQModel recovers a local band x role x channel matrix.  It deliberately
// does not consult manufacturer or plug-in name.
func DetectEQModel(digest ParameterDigest) *EQModel {
	role := strings.ToLower(strings.TrimSpace(digest.TemplateRole))
	class := strings.ToLower(strings.TrimSpace(digest.PluginClass))
	if role == "comp" || role == "compressor" || class == "comp" || class == "compressor" {
		return nil
	}

	parsed := make([]eqParsedParameter, 0, len(digest.Parameters))
	for _, param := range digest.Parameters {
		if strings.TrimSpace(param.ID) == "" {
			continue
		}
		if candidate, ok := parseEQParameter(param); ok {
			parsed = append(parsed, candidate)
		}
	}
	if len(parsed) == 0 {
		return opaqueFixedEQModel(digest)
	}

	byBand := map[string][]eqParsedParameter{}
	for _, candidate := range parsed {
		if candidate.BandKey != "" {
			byBand[candidate.BandKey] = append(byBand[candidate.BandKey], candidate)
		}
	}

	allBands := make([]EQBand, 0, len(byBand))
	coupled := false
	for key, candidates := range byBand {
		band := buildEQBand(key, candidates)
		if hasCoupledGainTopology(band) {
			coupled = true
		}
		allBands = append(allBands, band)
	}
	if coupled {
		return &EQModel{
			Classification: "fixed_slot_adjustable",
			MappingSource:  "generic_structural",
			Confidence:     0.96,
			Completeness: EQCompleteness{Complete: false, CandidateBands: len(allBands),
				Issues: []string{"coupled boost/attenuate topology"}},
			SetEQPointSupported: false,
			Reason:              "coupled boost/attenuate topology",
		}
	}

	adjustable := filterEQBands(allBands, true)
	fixed := filterEQBands(allBands, false)
	selected := adjustable
	classification := "fixed_slot_adjustable"
	if len(fixed) > len(adjustable) {
		selected = fixed
		classification = "fixed_freq"
	}
	if len(selected) < 2 {
		if opaque := opaqueFixedEQModel(digest); opaque != nil {
			return opaque
		}
		return nil
	}

	if eqBandsFreeFloatingTyped(selected) {
		classification = "free_floating"
	}
	sort.SliceStable(selected, func(i, j int) bool { return eqBandLess(selected[i], selected[j]) })

	model := &EQModel{
		Classification:      classification,
		MappingSource:       "generic_structural",
		Confidence:          eqModelConfidence(selected),
		Bands:               selected,
		SetEQPointSupported: true,
	}
	model.Completeness = validateEQMatrix(selected, len(allBands))
	model.Capabilities = capabilitiesForEQBands(selected)
	model.Channels = channelsForEQBands(selected)
	if !model.Completeness.Complete {
		model.SetEQPointSupported = false
		model.Reason = strings.Join(model.Completeness.Issues, "; ")
	}
	return model
}

func parseEQParameter(param ParameterInfo) (eqParsedParameter, bool) {
	name := eqParamDisplayName(param)
	if name == "" {
		return eqParsedParameter{}, false
	}
	prepared := eqCamelBoundary.ReplaceAllString(name, `$1 $2`)
	tokens := eqTokenPattern.FindAllString(prepared, -1)
	for i := range tokens {
		tokens[i] = strings.ToLower(strings.Join(strings.Fields(tokens[i]), ""))
	}
	if len(tokens) == 0 {
		return eqParsedParameter{}, false
	}

	channel := ""
	filtered := make([]string, 0, len(tokens))
	for i, token := range tokens {
		switch token {
		case "left", "right":
			channel = token
			continue
		case "l", "r":
			if i == 0 && len(tokens) > 1 {
				if token == "l" {
					channel = "left"
				} else {
					channel = "right"
				}
				continue
			}
		}
		filtered = append(filtered, token)
	}
	tokens = filtered

	role, consumed, activationKind, polarity := eqRoleTokens(tokens)
	if role == "" {
		if hz, ok := frequencyFromTokens(tokens); ok && parameterHasGainDomain(param) {
			role = eqRoleGain
			consumed = map[int]bool{}
			value := hz
			return eqParsedParameter{Param: param, BandKey: canonicalFrequencyKey(value), Role: role,
				Channel: defaultEQChannel(channel), FixedHz: &value, Confidence: 0.98}, true
		}
		return eqParsedParameter{}, false
	}

	remaining := make([]string, 0, len(tokens))
	for i, token := range tokens {
		if consumed[i] || token == "band" || token == "eq" || token == "filter" {
			continue
		}
		remaining = append(remaining, token)
	}
	if len(remaining) == 0 {
		return eqParsedParameter{}, false
	}
	bandKey := strings.Join(remaining, " ")
	if role == eqRoleFrequency && !parameterHasFrequencyDomain(param) {
		return eqParsedParameter{}, false
	}
	if role == eqRoleGain && !parameterHasGainDomain(param) &&
		!(polarity != "" && parameterHasNumericRange(param)) {
		return eqParsedParameter{}, false
	}
	if role == eqRoleQ && !parameterHasQDomain(param) {
		return eqParsedParameter{}, false
	}
	if role == eqRoleActivation && !parameterHasEnumDomain(param) {
		return eqParsedParameter{}, false
	}

	return eqParsedParameter{Param: param, BandKey: bandKey, Role: role,
		Channel: defaultEQChannel(channel), ActivationKind: activationKind,
		GainPolarity: polarity, Confidence: 0.94}, true
}

func eqRoleTokens(tokens []string) (string, map[int]bool, string, string) {
	consume := map[int]bool{}
	for i, token := range tokens {
		switch token {
		case "frequency", "freq", "frq", "cutoff":
			consume[i] = true
			return eqRoleFrequency, consume, "", ""
		case "gain", "level":
			consume[i] = true
			return eqRoleGain, consume, "", ""
		case "boost":
			consume[i] = true
			if i+1 < len(tokens) && tokens[i+1] == "cut" {
				consume[i+1] = true
			}
			return eqRoleGain, consume, "", "boost"
		case "atten", "attenuate", "attenuation":
			consume[i] = true
			return eqRoleGain, consume, "", "attenuate"
		case "bandwidth", "width", "q":
			consume[i] = true
			return eqRoleQ, consume, "", ""
		case "enabled", "enable", "active", "used":
			consume[i] = true
			return eqRoleActivation, consume, token, ""
		case "on":
			consume[i] = true
			if i+1 < len(tokens) && tokens[i+1] == "off" {
				consume[i+1] = true
			}
			return eqRoleActivation, consume, "on_off", ""
		case "in":
			if i+1 < len(tokens) && tokens[i+1] == "out" {
				consume[i], consume[i+1] = true, true
				return eqRoleActivation, consume, "in_out", ""
			}
		case "type", "shape":
			consume[i] = true
			return eqRoleShape, consume, "", ""
		}
	}
	return "", consume, "", ""
}

func defaultEQChannel(channel string) string {
	if channel == "" {
		return "shared"
	}
	return channel
}

func buildEQBand(key string, candidates []eqParsedParameter) EQBand {
	band := EQBand{Key: key, Bindings: map[string][]EQBinding{}, Active: true, Complete: true}
	for _, candidate := range candidates {
		binding := bindingFromEQCandidate(candidate)
		band.Bindings[candidate.Role] = append(band.Bindings[candidate.Role], binding)
		if candidate.FixedHz != nil {
			if band.FixedFrequencyHz != nil && !nearlyEqual(*band.FixedFrequencyHz, *candidate.FixedHz) {
				band.Issues = append(band.Issues, "conflicting fixed frequencies")
			}
			value := *candidate.FixedHz
			band.FixedFrequencyHz = &value
		}
	}
	for role, bindings := range band.Bindings {
		if role == eqRoleActivation {
			continue
		}
		seen := map[string]bool{}
		for _, binding := range bindings {
			if seen[binding.Channel] {
				band.Issues = append(band.Issues, fmt.Sprintf("role conflict: %s/%s", role, binding.Channel))
			}
			seen[binding.Channel] = true
		}
	}
	for _, binding := range band.Bindings[eqRoleActivation] {
		band.ActivationKnown = true
		if !activationBindingIsActive(binding) {
			band.Active = false
		}
	}
	// Some products expose a combined Type/On control.  Treat a toggle-shaped
	// type as activation too, without inventing a separate shape target.
	for _, binding := range band.Bindings[eqRoleShape] {
		if bindingHasToggleLabels(binding) {
			binding.Role = eqRoleActivation
			binding.ActivationKind = "shape_on_off"
			band.Bindings[eqRoleActivation] = append(band.Bindings[eqRoleActivation], binding)
			band.ActivationKnown = true
			if !activationBindingIsActive(binding) {
				band.Active = false
			}
		}
	}
	_, hasGain := band.Bindings[eqRoleGain]
	_, hasFreq := band.Bindings[eqRoleFrequency]
	if !hasGain || (!hasFreq && band.FixedFrequencyHz == nil) {
		band.Complete = false
	}
	if len(band.Issues) > 0 {
		band.Complete = false
	}
	return band
}

func bindingFromEQCandidate(candidate eqParsedParameter) EQBinding {
	param := candidate.Param
	binding := EQBinding{ParamID: param.ID, Name: eqParamDisplayName(param), Role: candidate.Role,
		Channel: candidate.Channel, ActivationKind: candidate.ActivationKind,
		GainPolarity: candidate.GainPolarity, CurrentText: param.ValueText,
		CurrentNormalized: anyFloat(param.NormalizedValue), Domain: param.DisplayDomainCandidate,
		Curve: eqCurveFromProbe(param), Reachable: reachableValuesForEQParam(param, candidate.Role)}
	if value, ok := parseEQPhysical(candidate.Role, param.ValueText); ok {
		binding.CurrentPhysical = &value
	}
	return binding
}

func reachableValuesForEQParam(param ParameterInfo, role string) []EQReachableValue {
	probe := param.DisplayProbe
	if probe == nil {
		return nil
	}
	out := make([]EQReachableValue, 0, len(probe.DiscreteLabels))
	for _, row := range probe.DiscreteLabels {
		normalized := anyFloat(row.Value)
		item := EQReachableValue{Normalized: normalized, Label: row.Label}
		if physical, ok := parseEQPhysical(role, row.Label); ok {
			item.Physical = &physical
		}
		out = append(out, item)
	}
	if len(out) == 0 && param.NumSteps > 1 && len(probe.AllLabels) == param.NumSteps {
		for i, label := range probe.AllLabels {
			normalized := float64(i) / float64(param.NumSteps-1)
			item := EQReachableValue{Normalized: normalized, Label: label}
			if physical, ok := parseEQPhysical(role, label); ok {
				item.Physical = &physical
			}
			out = append(out, item)
		}
	}
	return out
}

func parameterHasFrequencyDomain(param ParameterInfo) bool {
	if values := reachableValuesForEQParam(param, eqRoleFrequency); len(values) > 0 {
		for _, value := range values {
			if value.Physical != nil {
				return true
			}
		}
	}
	shape := eqShapeOfParam(param)
	return shape.HasRange && shape.Lo > 0 && shape.Hi >= eqFreqPlausibleMax
}

func parameterHasGainDomain(param ParameterInfo) bool {
	if values := reachableValuesForEQParam(param, eqRoleGain); len(values) > 0 {
		negative, positive := false, false
		for _, value := range values {
			if value.Physical == nil {
				continue
			}
			negative = negative || *value.Physical < 0
			positive = positive || *value.Physical > 0
		}
		if negative && positive {
			return true
		}
	}
	return eqShapeOfParam(param).looksLikeGain()
}

func parameterHasQDomain(param ParameterInfo) bool {
	if values := reachableValuesForEQParam(param, eqRoleQ); len(values) > 0 {
		return true
	}
	return eqShapeOfParam(param).looksLikeQ()
}

func parameterHasEnumDomain(param ParameterInfo) bool {
	if param.IsBoolean || param.IsDiscrete || len(reachableValuesForEQParam(param, eqRoleActivation)) > 0 {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(param.ValueText))
	for _, token := range []string{"on", "off", "in", "out", "used", "unused", "enabled", "disabled"} {
		if lower == token {
			return true
		}
	}
	return false
}

func parameterHasNumericRange(param ParameterInfo) bool {
	domain := param.DisplayDomainCandidate
	return domain != nil && domain.Min != nil && domain.Max != nil && *domain.Max > *domain.Min
}

func frequencyFromTokens(tokens []string) (float64, bool) {
	for _, token := range tokens {
		lower := strings.ToLower(strings.ReplaceAll(token, " ", ""))
		if strings.HasSuffix(lower, "hz") {
			return parseFrequencyText(lower)
		}
	}
	return 0, false
}

func parseEQPhysical(role, text string) (float64, bool) {
	switch role {
	case eqRoleFrequency:
		return parseFrequencyText(text)
	case eqRoleGain, eqRoleQ:
		match := eqNumberPattern.FindString(strings.TrimSpace(text))
		if match == "" {
			return 0, false
		}
		value, err := strconv.ParseFloat(match, 64)
		return value, err == nil
	}
	return 0, false
}

func parseFrequencyText(text string) (float64, bool) {
	lower := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(text), " ", ""))
	match := eqNumberPattern.FindString(lower)
	if match == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, false
	}
	if strings.Contains(lower, "khz") {
		value *= 1000
	}
	if value <= 0 {
		return 0, false
	}
	return value, true
}

func canonicalFrequencyKey(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64) + "hz"
}

func hasCoupledGainTopology(band EQBand) bool {
	boost, attenuate := false, false
	for _, binding := range band.Bindings[eqRoleGain] {
		boost = boost || binding.GainPolarity == "boost"
		attenuate = attenuate || binding.GainPolarity == "attenuate"
	}
	return boost && attenuate
}

func filterEQBands(bands []EQBand, adjustable bool) []EQBand {
	out := []EQBand{}
	for _, band := range bands {
		_, hasFreq := band.Bindings[eqRoleFrequency]
		isAdjustable := hasFreq
		if band.Complete && isAdjustable == adjustable {
			out = append(out, band)
		}
	}
	return out
}

func validateEQMatrix(bands []EQBand, candidates int) EQCompleteness {
	result := EQCompleteness{Complete: true, CompleteBands: len(bands), CandidateBands: candidates}
	explicitChannels := channelsForEQBands(bands)
	for _, band := range bands {
		if !band.Complete {
			result.Issues = append(result.Issues, "incomplete band "+band.Key)
			continue
		}
		for _, role := range []string{eqRoleGain} {
			actual := explicitChannelsForBindings(band.Bindings[role])
			if len(explicitChannels) > 0 && !equalStringSets(actual, explicitChannels) {
				result.Issues = append(result.Issues, "channel bindings missing for band "+band.Key)
			}
		}
		if !band.Active && len(band.Bindings[eqRoleActivation]) == 0 {
			result.Issues = append(result.Issues, "inactive band has no activation binding: "+band.Key)
		}
		result.Issues = append(result.Issues, band.Issues...)
	}
	result.Issues = uniqueEQStrings(result.Issues)
	result.Complete = len(result.Issues) == 0
	return result
}

func capabilitiesForEQBands(bands []EQBand) EQCapabilities {
	var out EQCapabilities
	for _, band := range bands {
		out.Gain = out.Gain || len(band.Bindings[eqRoleGain]) > 0
		out.FrequencyAdjustable = out.FrequencyAdjustable || len(band.Bindings[eqRoleFrequency]) > 0
		out.FixedFrequency = out.FixedFrequency || band.FixedFrequencyHz != nil
		out.Q = out.Q || len(band.Bindings[eqRoleQ]) > 0
		out.Shape = out.Shape || len(band.Bindings[eqRoleShape]) > 0
		out.Activation = out.Activation || len(band.Bindings[eqRoleActivation]) > 0
	}
	out.MultiChannel = len(channelsForEQBands(bands)) > 1
	return out
}

func channelsForEQBands(bands []EQBand) []string {
	seen := map[string]bool{}
	for _, band := range bands {
		for _, binding := range band.Bindings[eqRoleGain] {
			if binding.Channel != "shared" {
				seen[binding.Channel] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for channel := range seen {
		out = append(out, channel)
	}
	sort.Strings(out)
	return out
}

func explicitChannelsForBindings(bindings []EQBinding) []string {
	seen := map[string]bool{}
	for _, binding := range bindings {
		if binding.Channel != "shared" {
			seen[binding.Channel] = true
		}
	}
	out := []string{}
	for channel := range seen {
		out = append(out, channel)
	}
	sort.Strings(out)
	return out
}

func equalStringSets(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func activationBindingIsActive(binding EQBinding) bool {
	lower := strings.ToLower(strings.TrimSpace(binding.CurrentText))
	for _, token := range []string{"unused", "disabled", "bypass", "off", "out", "false"} {
		if lower == token || strings.HasPrefix(lower, token+" ") {
			return false
		}
	}
	return true
}

func bindingHasToggleLabels(binding EQBinding) bool {
	labels := []string{}
	for _, value := range binding.Reachable {
		labels = append(labels, value.Label)
	}
	return looksLikeToggleLabels(labels)
}

func eqBandsFreeFloatingTyped(bands []EQBand) bool {
	for _, band := range bands {
		for _, binding := range band.Bindings[eqRoleActivation] {
			if binding.ActivationKind == "used" {
				return true
			}
		}
	}
	return false
}

func eqBandLess(left, right EQBand) bool {
	if left.FixedFrequencyHz != nil && right.FixedFrequencyHz != nil {
		return *left.FixedFrequencyHz < *right.FixedFrequencyHz
	}
	li, le := strconv.ParseFloat(left.Key, 64)
	ri, re := strconv.ParseFloat(right.Key, 64)
	if le == nil && re == nil {
		return li < ri
	}
	return left.Key < right.Key
}

func eqModelConfidence(bands []EQBand) float64 {
	if len(bands) >= 4 {
		return 0.96
	}
	return math.Min(0.94, 0.84+float64(len(bands))*0.03)
}

func opaqueFixedEQModel(digest ParameterDigest) *EQModel {
	bands, order, fixed := collectEQBandsStructural(digest)
	if len(bands) < 2 {
		return nil
	}
	out := make([]EQBand, 0, len(order))
	for _, key := range order {
		entry := bands[key]
		gain, ok := entry.Slots[eqSlotGain]
		if !ok || gain.ParamID == "" {
			continue
		}
		binding := EQBinding{ParamID: gain.ParamID, Role: eqRoleGain, Channel: "shared",
			CurrentText: gain.ValueText, Domain: gain.Domain, Curve: gain.Curve}
		if value, ok := parseEQPhysical(eqRoleGain, gain.ValueText); ok {
			binding.CurrentPhysical = &value
		}
		band := EQBand{Key: key, Bindings: map[string][]EQBinding{eqRoleGain: {binding}},
			Active: true, Complete: true}
		if value, ok := fixed[key]; ok {
			hz := value
			band.FixedFrequencyHz = &hz
		}
		if band.FixedFrequencyHz == nil {
			band.Complete = false
			band.Issues = []string{"fixed frequency unknown"}
		}
		out = append(out, band)
	}
	if len(out) < 2 {
		return nil
	}
	model := &EQModel{Classification: "fixed_freq", MappingSource: "generic_structural",
		Confidence: 0.82, Bands: out, Capabilities: capabilitiesForEQBands(out),
		SetEQPointSupported: true}
	model.Completeness = validateEQMatrix(out, len(out))
	if !model.Completeness.Complete {
		model.SetEQPointSupported = false
		model.Reason = strings.Join(model.Completeness.Issues, "; ")
	}
	return model
}

func anyFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case jsonNumberLike:
		parsed, _ := strconv.ParseFloat(string(typed), 64)
		return parsed
	}
	return 0
}

type jsonNumberLike string

func uniqueEQStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
