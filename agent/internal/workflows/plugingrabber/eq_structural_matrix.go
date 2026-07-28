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
	Sections            []EQSection
	Channels            []string
	SetEQPointSupported bool
	Reason              string
	ControlTopology     EQControlTopology
}

// EQControlTopology is the request-agnostic, identity-free control surface
// recovered from parameter structure. Classification is retained as a public
// compatibility projection; planning is driven by the orthogonal section
// axes below.
type EQControlTopology struct {
	SchemaVersion        string
	Generation           string
	PublicClassification string
	AddressingKinds      []EQAddressingKind
	ShapeCapabilities    []EQShapeCapability
}

type EQAddressingKind string

const (
	EQAddressingAnchored    EQAddressingKind = "anchored"
	EQAddressingResident    EQAddressingKind = "resident"
	EQAddressingAllocatable EQAddressingKind = "allocatable"
)

type EQShapeCapability struct {
	Shape          EQFilterKind
	Upsert         bool
	Modify         bool
	Disable        bool
	Remove         bool
	Undo           bool
	RejectionCodes map[string][]string
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
	Slope               bool
	FilterDesign        bool
	Activation          bool
	MultiChannel        bool
}

// EQFilterKind is the audible topology of one local EQ section.  It is kept
// orthogonal to the three public placement classifications above: a
// fixed_slot_adjustable EQ may expose bell, shelf and cut sections at once.
type EQFilterKind string

const (
	EQFilterUnknown   EQFilterKind = "unknown"
	EQFilterBell      EQFilterKind = "bell"
	EQFilterLowShelf  EQFilterKind = "low_shelf"
	EQFilterHighShelf EQFilterKind = "high_shelf"
	EQFilterLowCut    EQFilterKind = "low_cut"
	EQFilterHighCut   EQFilterKind = "high_cut"
	EQFilterNotch     EQFilterKind = "notch"
)

type EQActivationStrategy string

const (
	EQActivationExplicit         EQActivationStrategy = "explicit_binding"
	EQActivationImplicitInDomain EQActivationStrategy = "implicit_in_domain"
	EQActivationPropertySentinel EQActivationStrategy = "property_sentinel"
	EQActivationAlwaysActive     EQActivationStrategy = "always_active"
)

// EQSection is the typed view of the same band x role x channel matrix used by
// Bands.  Primary sections back the legacy Bell path; auxiliary sections (for
// example dedicated HP/LP controls around a graphic EQ) remain independently
// addressable without changing the public classification.
type EQSection struct {
	Key               string
	Primary           bool
	DedicatedKind     EQFilterKind
	ReachableKinds    []EQFilterKind
	FixedFrequencyHz  *float64
	Bindings          map[string][]EQBinding
	Activation        EQActivation
	Complete          bool
	Issues            []string
	Addressing        EQAddressingKind
	Deallocatable     bool
	ExclusionCodes    []string
	ShapeCapabilities []EQShapeCapability
}

type EQActivation struct {
	Strategy    EQActivationStrategy
	BindingRole string
	Active      bool
	Known       bool
	Bindings    []EQBinding
}

type EQBand struct {
	Key                string
	FixedFrequencyHz   *float64
	Bindings           map[string][]EQBinding
	Active             bool
	ActivationKnown    bool
	ActivationStrategy EQActivationStrategy
	Complete           bool
	Issues             []string
}

type EQBinding struct {
	ParamID           string
	Name              string
	Role              string
	Channel           string
	ActivationKind    string
	GainPolarity      string
	TransformFactor   float64
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
	Kind       EQFilterKind
}

const (
	eqRoleFrequency          = "frequency"
	eqRoleGain               = "gain"
	eqRoleQ                  = "q"
	eqRoleActivation         = "activation"
	eqRoleShape              = "shape" // legacy public summary alias for filter_kind
	eqRoleFilterKind         = "filter_kind"
	eqRoleSlope              = "slope"
	eqRoleDesign             = "filter_design"
	eqRoleFrequencyTransform = "frequency_transform"
)

type eqParsedParameter struct {
	Param           ParameterInfo
	BandKey         string
	Role            string
	Channel         string
	FixedHz         *float64
	ActivationKind  string
	GainPolarity    string
	Confidence      float64
	DedicatedKind   EQFilterKind
	NamedKind       EQFilterKind
	TransformFactor float64
}

var (
	eqCamelBoundary = regexp.MustCompile(`([a-z])([A-Z])`)
	eqTokenPattern  = regexp.MustCompile(`(?i)\d+(?:\.\d+)?\s*(?:k\s*hz|hz)?|[a-z]+`)
	eqNumberPattern = regexp.MustCompile(`[-+]?\d+(?:[.,]\d+)?`)
	eqEngineeringK  = regexp.MustCompile(`(?i)^([-+]?\d+)k(\d+)(?:hz)?$`)
)

// DetectEQModel recovers a local band x role x channel matrix.  It deliberately
// does not consult manufacturer or plug-in name.
func DetectEQModel(digest ParameterDigest) *EQModel {
	role := strings.ToLower(strings.TrimSpace(digest.TemplateRole))
	class := strings.ToLower(strings.TrimSpace(digest.PluginClass))
	if role == "comp" || role == "compressor" || class == "comp" || class == "compressor" {
		return nil
	}

	parsed := parseEQParameters(digest.Parameters)
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
	sections := make([]EQSection, 0, len(byBand))
	for key, candidates := range byBand {
		band := buildEQBand(key, candidates)
		section := buildEQSection(key, candidates)
		if sectionHasEQAnchor(section) {
			sections = append(sections, section)
		}
		allBands = append(allBands, band)
	}
	applyEQSectionExclusions(sections, digest.Parameters)

	adjustable := filterEQBands(allBands, true)
	fixed := filterEQBands(allBands, false)
	selected := adjustable
	classification := "fixed_slot_adjustable"
	if len(fixed) > len(adjustable) {
		selected = fixed
		classification = "fixed_freq"
	}
	if len(selected) < 2 && countUsableEQSections(sections) < 1 {
		if opaque := opaqueFixedEQModel(digest); opaque != nil {
			return opaque
		}
		return nil
	}

	if eqBandsFreeFloatingTyped(selected) {
		classification = "free_floating"
	}
	sort.SliceStable(selected, func(i, j int) bool { return eqBandLess(selected[i], selected[j]) })
	primaryKeys := map[string]bool{}
	for _, band := range selected {
		primaryKeys[band.Key] = true
	}
	for i := range sections {
		sections[i].Primary = primaryKeys[sections[i].Key]
	}
	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].Primary != sections[j].Primary {
			return sections[i].Primary
		}
		return sections[i].Key < sections[j].Key
	})

	model := &EQModel{
		Classification:      classification,
		MappingSource:       "generic_structural",
		Confidence:          eqModelConfidence(selected),
		Bands:               selected,
		Sections:            sections,
		SetEQPointSupported: true,
	}
	model.Completeness = validateEQMatrix(selected, len(allBands))
	if len(selected) == 0 && countUsableEQSections(sections) > 0 {
		model.Completeness = EQCompleteness{Complete: true, CompleteBands: countUsableEQSections(sections), CandidateBands: len(allBands)}
	}
	model.Capabilities = capabilitiesForEQBands(selected)
	mergeEQSectionCapabilities(&model.Capabilities, sections)
	model.Channels = uniqueEQStrings(append(channelsForEQBands(selected), channelsForEQSections(sections)...))
	sort.Strings(model.Channels)
	model.Capabilities.MultiChannel = len(model.Channels) > 1
	deriveEQControlTopology(model)
	// Compatibility only. New planners consume per-shape capabilities and must
	// never use this plugin-wide projection as an execution gate.
	model.SetEQPointSupported = anyEQShapeUpsert(model.ControlTopology.ShapeCapabilities)
	if !model.SetEQPointSupported {
		model.Reason = "no universally safe static EQ shape is provably controllable"
	}
	return model
}

func sectionHasEQAnchor(section EQSection) bool {
	return section.FixedFrequencyHz != nil || len(section.Bindings[eqRoleFrequency]) > 0
}

var eqPublicStaticShapes = []EQFilterKind{
	EQFilterBell, EQFilterLowShelf, EQFilterHighShelf, EQFilterLowCut, EQFilterHighCut,
}

func countUsableEQSections(sections []EQSection) int {
	count := 0
	for _, section := range sections {
		if section.Complete && sectionHasEQAnchor(section) && len(section.ReachableKinds) > 0 {
			count++
		}
	}
	return count
}

func applyEQSectionExclusions(sections []EQSection, params []ParameterInfo) {
	statefulSurface := false
	unsafeChannelMode := false
	for _, param := range params {
		text := strings.ToLower(strings.Join([]string{eqParamDisplayName(param), param.DisplayGroup}, " "))
		if containsAnyEQPhrase(text, "q-clone", "q clone", "capture spectrum", "captured spectrum",
			"match spectrum", "matching spectrum", "calibration curve", "reference curve", "target graph",
			"mixsense", "dynamic / static balance") || containsAnyEQToken(text, "capture") {
			statefulSurface = true
			break
		}
		if containsAnyEQPhrase(text, "eq mode", "channel mode") {
			mode := strings.ToLower(strings.TrimSpace(param.ValueText))
			if mode != "" && mode != "stereo" && mode != "linked" && mode != "link" {
				unsafeChannelMode = true
			}
		}
	}
	for i := range sections {
		section := &sections[i]
		codes := append([]string(nil), section.ExclusionCodes...)
		compactKey := strings.ReplaceAll(strings.ToLower(section.Key), " ", "")
		if containsAnyEQToken(section.Key, "sc", "sidechain", "detector", "gate") ||
			strings.Contains(compactKey, "scbell") || strings.Contains(compactKey, "scshelf") {
			codes = append(codes, "sidechain_or_detector_section_excluded")
		}
		if containsAnyEQPhrase(strings.ToLower(section.Key), "de esser", "deesser") {
			codes = append(codes, "dynamic_section_excluded")
		}
		if statefulSurface {
			codes = append(codes, "stateful_surface_dependency_unresolved")
		}
		if unsafeChannelMode {
			codes = append(codes, "nonlinked_channel_mode_excluded")
		}
		// Dynamic, threshold and sidechain parameters that share a band key are
		// auxiliary controls, not evidence that the band's independent static
		// Frequency/Gain/Q/Shape core is unsafe. Component ownership is decided
		// above from the section key itself; auxiliary siblings must not widen a
		// local exclusion to the whole main-audible EQ section.
		section.ExclusionCodes = uniqueEQStrings(codes)
	}
}

func containsAnyEQPhrase(text string, phrases ...string) bool {
	for _, phrase := range phrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func containsAnyEQToken(text string, tokens ...string) bool {
	fields := eqTokenPattern.FindAllString(strings.ToLower(text), -1)
	for _, field := range fields {
		field = strings.ToLower(strings.Join(strings.Fields(field), ""))
		for _, token := range tokens {
			if field == token {
				return true
			}
		}
	}
	return false
}

func eqParameterTextBelongsToSection(text, key string) bool {
	text = strings.ToLower(text)
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}
	if _, err := strconv.Atoi(key); err == nil {
		return containsAnyEQPhrase(text, "band "+key, "band"+key, "eq "+key, "eq"+key)
	}
	keyFields := strings.Fields(key)
	for _, field := range keyFields {
		if !containsAnyEQToken(text, field) {
			return false
		}
	}
	return true
}

func deriveEQControlTopology(model *EQModel) {
	if model == nil {
		return
	}
	addressing := []EQAddressingKind{}
	for i := range model.Sections {
		section := &model.Sections[i]
		section.Addressing = addressingForEQSection(*section)
		section.Deallocatable = sectionHasSlotLifecycle(*section)
		section.ShapeCapabilities = capabilitiesForEQSection(*section)
		addressing = append(addressing, section.Addressing)
	}
	model.ControlTopology = EQControlTopology{
		SchemaVersion:        "eq-control-topology/v1",
		PublicClassification: model.Classification,
		AddressingKinds:      uniqueEQAddressingKinds(addressing),
		ShapeCapabilities:    aggregateEQShapeCapabilities(model.Sections),
	}
	model.ControlTopology.Generation = eqTopologyGeneration(model)
}

func addressingForEQSection(section EQSection) EQAddressingKind {
	if section.FixedFrequencyHz != nil && len(section.Bindings[eqRoleFrequency]) == 0 {
		return EQAddressingAnchored
	}
	if sectionHasSlotLifecycle(section) {
		return EQAddressingAllocatable
	}
	return EQAddressingResident
}

func sectionHasSlotLifecycle(section EQSection) bool {
	for _, binding := range section.Bindings[eqRoleActivation] {
		if binding.ActivationKind != "used" {
			continue
		}
		active, inactive := false, false
		for _, value := range binding.Reachable {
			label := strings.ToLower(strings.TrimSpace(value.Label))
			active = active || label == "used" || label == "active" || label == "on"
			inactive = inactive || label == "unused" || label == "inactive" || label == "off"
		}
		if active && inactive {
			return true
		}
	}
	return false
}

func capabilitiesForEQSection(section EQSection) []EQShapeCapability {
	out := make([]EQShapeCapability, 0, len(eqPublicStaticShapes))
	for _, shape := range eqPublicStaticShapes {
		baseCodes := append([]string(nil), section.ExclusionCodes...)
		if !eqSectionReachesKind(section, shape) {
			baseCodes = append(baseCodes, "shape_not_provably_reachable")
		}
		if !section.Complete {
			baseCodes = append(baseCodes, "section_incomplete")
		}
		if !sectionHasEQAnchor(section) {
			baseCodes = append(baseCodes, "frequency_anchor_unavailable")
		}
		if !eqSectionHasPureFrequencyLaw(section) {
			baseCodes = append(baseCodes, "frequency_law_not_pure")
		}
		if shape == EQFilterBell || shape == EQFilterLowShelf || shape == EQFilterHighShelf {
			if len(section.Bindings[eqRoleGain]) == 0 {
				baseCodes = append(baseCodes, "required_gain_binding_absent")
			}
			if eqSectionHasCoupledGainTopology(section) {
				baseCodes = append(baseCodes, "gain_law_not_arbitrary_bipolar")
			}
		}
		baseCodes = uniqueEQStrings(baseCodes)
		upsert := len(baseCodes) == 0
		disableCodes := append([]string(nil), baseCodes...)
		if !sectionHasProvableDisable(section) {
			disableCodes = append(disableCodes, "activation_binding_unavailable")
		}
		removeCodes := append([]string(nil), baseCodes...)
		if !section.Deallocatable {
			removeCodes = append(removeCodes, "section_not_deallocatable")
		}
		capability := EQShapeCapability{
			Shape: shape, Upsert: upsert, Modify: upsert,
			Disable:        len(uniqueEQStrings(disableCodes)) == 0,
			Remove:         len(uniqueEQStrings(removeCodes)) == 0,
			Undo:           upsert,
			RejectionCodes: map[string][]string{},
		}
		if !capability.Upsert {
			capability.RejectionCodes["upsert"] = baseCodes
			capability.RejectionCodes["modify"] = baseCodes
			capability.RejectionCodes["undo"] = append([]string{"forward_control_unavailable"}, baseCodes...)
		}
		if !capability.Disable {
			capability.RejectionCodes["disable"] = uniqueEQStrings(disableCodes)
		}
		if !capability.Remove {
			capability.RejectionCodes["remove"] = uniqueEQStrings(removeCodes)
		}
		out = append(out, capability)
	}
	return out
}

func eqSectionReachesKind(section EQSection, shape EQFilterKind) bool {
	for _, kind := range section.ReachableKinds {
		if kind == shape {
			return true
		}
	}
	return false
}

func eqSectionHasCoupledGainTopology(section EQSection) bool {
	boost, attenuate := false, false
	for _, binding := range section.Bindings[eqRoleGain] {
		boost = boost || binding.GainPolarity == "boost"
		attenuate = attenuate || binding.GainPolarity == "attenuate"
	}
	return boost && attenuate
}

func eqSectionHasPureFrequencyLaw(section EQSection) bool {
	if section.FixedFrequencyHz != nil && len(section.Bindings[eqRoleFrequency]) == 0 {
		return true
	}
	bindings := section.Bindings[eqRoleFrequency]
	if len(bindings) == 0 {
		return false
	}
	for _, binding := range bindings {
		for _, value := range binding.Reachable {
			if value.Physical == nil && !eqInactiveLabel(value.Label) {
				return false
			}
		}
	}
	return true
}

func sectionHasProvableDisable(section EQSection) bool {
	if sectionHasSlotLifecycle(section) {
		return true
	}
	for _, binding := range section.Bindings[eqRoleActivation] {
		for _, value := range binding.Reachable {
			if eqInactiveLabel(value.Label) {
				return true
			}
		}
	}
	if section.Activation.Strategy == EQActivationImplicitInDomain {
		for _, binding := range section.Bindings[eqRoleFrequency] {
			for _, value := range binding.Reachable {
				if eqInactiveLabel(value.Label) {
					return true
				}
			}
		}
	}
	if section.Activation.Strategy == EQActivationPropertySentinel {
		for _, binding := range section.Bindings[section.Activation.BindingRole] {
			for _, value := range binding.Reachable {
				if eqInactiveLabel(value.Label) {
					return true
				}
			}
		}
	}
	return false
}

func eqInactiveLabel(label string) bool {
	lower := strings.ToLower(strings.TrimSpace(label))
	for _, token := range []string{"unused", "inactive", "disabled", "off", "out", "bypass", "bypassed"} {
		if lower == token || strings.HasPrefix(lower, token+" ") {
			return true
		}
	}
	return false
}

func aggregateEQShapeCapabilities(sections []EQSection) []EQShapeCapability {
	out := make([]EQShapeCapability, 0, len(eqPublicStaticShapes))
	for _, shape := range eqPublicStaticShapes {
		aggregate := EQShapeCapability{Shape: shape, RejectionCodes: map[string][]string{}}
		for _, section := range sections {
			for _, capability := range section.ShapeCapabilities {
				if capability.Shape != shape {
					continue
				}
				aggregate.Upsert = aggregate.Upsert || capability.Upsert
				aggregate.Modify = aggregate.Modify || capability.Modify
				aggregate.Disable = aggregate.Disable || capability.Disable
				aggregate.Remove = aggregate.Remove || capability.Remove
				aggregate.Undo = aggregate.Undo || capability.Undo
				for action, codes := range capability.RejectionCodes {
					aggregate.RejectionCodes[action] = append(aggregate.RejectionCodes[action], codes...)
				}
			}
		}
		for action, codes := range aggregate.RejectionCodes {
			aggregate.RejectionCodes[action] = uniqueEQStrings(codes)
		}
		for action, supported := range map[string]bool{
			"upsert": aggregate.Upsert, "modify": aggregate.Modify, "disable": aggregate.Disable,
			"remove": aggregate.Remove, "undo": aggregate.Undo,
		} {
			if supported {
				delete(aggregate.RejectionCodes, action)
			}
		}
		out = append(out, aggregate)
	}
	return out
}

func anyEQShapeUpsert(capabilities []EQShapeCapability) bool {
	for _, capability := range capabilities {
		if capability.Upsert {
			return true
		}
	}
	return false
}

func uniqueEQAddressingKinds(values []EQAddressingKind) []EQAddressingKind {
	seen := map[EQAddressingKind]bool{}
	out := []EQAddressingKind{}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func eqTopologyGeneration(model *EQModel) string {
	type bindingFact struct {
		ParamID, Role, Channel, ActivationKind, GainPolarity string
		Domain                                               *PluginDisplayDomain
		Reachable                                            []EQReachableValue
	}
	type sectionFact struct {
		Key            string
		DedicatedKind  EQFilterKind
		ReachableKinds []EQFilterKind
		FixedHz        *float64
		Addressing     EQAddressingKind
		Bindings       []bindingFact
		Exclusions     []string
	}
	payload := struct {
		Schema   string
		Sections []sectionFact
	}{Schema: "eq-control-topology/v1"}
	for _, section := range model.Sections {
		fact := sectionFact{Key: section.Key, DedicatedKind: section.DedicatedKind,
			ReachableKinds: section.ReachableKinds, FixedHz: section.FixedFrequencyHz,
			Addressing: section.Addressing, Exclusions: section.ExclusionCodes}
		roles := make([]string, 0, len(section.Bindings))
		for role := range section.Bindings {
			roles = append(roles, role)
		}
		sort.Strings(roles)
		for _, role := range roles {
			bindings := append([]EQBinding(nil), section.Bindings[role]...)
			sort.Slice(bindings, func(i, j int) bool {
				return bindings[i].ParamID+"/"+bindings[i].Channel < bindings[j].ParamID+"/"+bindings[j].Channel
			})
			for _, binding := range bindings {
				fact.Bindings = append(fact.Bindings, bindingFact{ParamID: binding.ParamID, Role: role,
					Channel: binding.Channel, ActivationKind: binding.ActivationKind,
					GainPolarity: binding.GainPolarity, Domain: binding.Domain, Reachable: binding.Reachable})
			}
		}
		payload.Sections = append(payload.Sections, fact)
	}
	sort.Slice(payload.Sections, func(i, j int) bool { return payload.Sections[i].Key < payload.Sections[j].Key })
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return "eqt1_" + hex.EncodeToString(sum[:12])
}

func parseEQParameters(params []ParameterInfo) []eqParsedParameter {
	parsed := make([]eqParsedParameter, 0, len(params))
	claimed := map[string]bool{}
	for _, param := range params {
		if strings.TrimSpace(param.ID) == "" {
			continue
		}
		if candidate, ok := parseEQParameter(param); ok {
			parsed = append(parsed, candidate)
			claimed[param.ID] = true
		}
	}

	hypotheses := make([]eqParsedParameter, 0)
	for _, param := range params {
		if strings.TrimSpace(param.ID) == "" || claimed[param.ID] {
			continue
		}
		name := eqParamDisplayName(param)
		if mayHaveCompactEQRoleSuffix(name) {
			if candidate, ok := parseCompactEQParameter(param); ok {
				hypotheses = append(hypotheses, candidate)
				continue
			}
		}
		if strings.Contains(strings.ToLower(name), "bell") {
			if candidate, ok := parseNamedEQKindToggle(param); ok {
				hypotheses = append(hypotheses, candidate)
			}
		}
	}
	if len(hypotheses) == 0 {
		return parsed
	}

	evidence := append(append([]eqParsedParameter(nil), parsed...), hypotheses...)
	byBand := map[string][]eqParsedParameter{}
	for _, candidate := range evidence {
		byBand[candidate.BandKey] = append(byBand[candidate.BandKey], candidate)
	}
	for _, candidate := range hypotheses {
		group := byBand[candidate.BandKey]
		if (candidate.DedicatedKind == EQFilterLowCut || candidate.DedicatedKind == EQFilterHighCut) &&
			candidate.Role == eqRoleFrequency {
			parsed = append(parsed, candidate)
			continue
		}
		if compactEQGroupHasPair(group, candidate.Channel) {
			parsed = append(parsed, candidate)
		}
	}
	return parsed
}

func mayHaveCompactEQRoleSuffix(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	lower = strings.TrimRight(lower, " \t\r\n_-.:;()[]{}")
	if lower == "" {
		return false
	}
	last := lower[len(lower)-1]
	if last == 'f' || last == 'g' || last == 'q' {
		return true
	}
	// Channel words may be placed after the compact stem.  Let the full token
	// parser decide these uncommon candidates without putting every unrelated
	// parameter through the expensive physical-domain fallback.
	for _, suffix := range []string{" left", " right", " l", " r"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func compactEQGroupHasPair(group []eqParsedParameter, channel string) bool {
	frequency, gain := false, false
	for _, candidate := range group {
		if !eqChannelsCompatible(channel, candidate.Channel) {
			continue
		}
		frequency = frequency || candidate.Role == eqRoleFrequency
		gain = gain || candidate.Role == eqRoleGain
	}
	return frequency && gain
}

func eqChannelsCompatible(left, right string) bool {
	return left == "shared" || right == "shared" || left == right
}

func parseCompactEQParameter(param ParameterInfo) (eqParsedParameter, bool) {
	tokens, channel := eqNameTokensAndChannel(eqParamDisplayName(param))
	if len(tokens) == 0 {
		return eqParsedParameter{}, false
	}
	compact := strings.Join(tokens, "")
	if len(compact) < 2 {
		return eqParsedParameter{}, false
	}
	stem, suffix := compact[:len(compact)-1], compact[len(compact)-1:]
	if strings.HasPrefix(stem, "band") && len(stem) > len("band") {
		stem = strings.TrimPrefix(stem, "band")
	}
	role := ""
	switch suffix {
	case "f":
		if !parameterHasFrequencyDomain(param) {
			return eqParsedParameter{}, false
		}
		role = eqRoleFrequency
	case "g":
		if !parameterHasGainDomain(param) {
			return eqParsedParameter{}, false
		}
		role = eqRoleGain
	case "q":
		if !parameterHasQDomain(param) {
			return eqParsedParameter{}, false
		}
		role = eqRoleQ
	default:
		return eqParsedParameter{}, false
	}
	bandKey, dedicatedKind := canonicalEQSectionKey([]string{stem})
	if bandKey == "" {
		return eqParsedParameter{}, false
	}
	return eqParsedParameter{Param: param, BandKey: bandKey, Role: role,
		Channel: defaultEQChannel(channel), DedicatedKind: dedicatedKind, Confidence: 0.91}, true
}

func parseNamedEQKindToggle(param ParameterInfo) (eqParsedParameter, bool) {
	tokens, channel := eqNameTokensAndChannel(eqParamDisplayName(param))
	if len(tokens) < 2 || !parameterHasEnumDomain(param) {
		return eqParsedParameter{}, false
	}
	kind, ok := parseEQFilterKind(tokens[len(tokens)-1])
	labels := eqParameterLabels(param)
	if !ok || kind != EQFilterBell ||
		(!eqLabelsContainPair(labels, "in", "out") && !eqLabelsContainPair(labels, "on", "off")) {
		return eqParsedParameter{}, false
	}
	bandKey, dedicatedKind := canonicalEQSectionKey(tokens[:len(tokens)-1])
	if bandKey == "" {
		return eqParsedParameter{}, false
	}
	return eqParsedParameter{Param: param, BandKey: bandKey, Role: eqRoleFilterKind,
		Channel: defaultEQChannel(channel), DedicatedKind: dedicatedKind,
		NamedKind: kind, Confidence: 0.93}, true
}

func eqLabelsContainPair(labels []string, first, second string) bool {
	foundFirst, foundSecond := false, false
	for _, label := range labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		foundFirst = foundFirst || lower == first
		foundSecond = foundSecond || lower == second
	}
	return foundFirst && foundSecond
}

func parseEQParameter(param ParameterInfo) (eqParsedParameter, bool) {
	name := eqParamDisplayName(param)
	if name == "" {
		return eqParsedParameter{}, false
	}
	tokens, channel := eqNameTokensAndChannel(name)
	if len(tokens) == 0 {
		return eqParsedParameter{}, false
	}
	if candidate, matched, ok := parseEQFrequencyTransform(param, tokens, channel); matched {
		return candidate, ok
	}
	if candidate, matched, ok := parseEQQualityProjection(param, tokens, channel); matched {
		return candidate, ok
	}

	role, consumed, activationKind, polarity := eqRoleTokens(tokens)
	if role == eqRoleQ && parameterLooksLikeSlope(param) {
		role = eqRoleSlope
	}
	if role == "" {
		if key, kind := canonicalEQSectionKey(tokens); (kind == EQFilterLowCut || kind == EQFilterHighCut) && parameterHasFrequencyDomain(param) {
			return eqParsedParameter{Param: param, BandKey: key, Role: eqRoleFrequency,
				Channel: defaultEQChannel(channel), Confidence: 0.96, DedicatedKind: kind}, true
		}
		if hz, ok := frequencyFromTokens(tokens); ok && parameterHasGainDomain(param) {
			role = eqRoleGain
			consumed = map[int]bool{}
			value := hz
			return eqParsedParameter{Param: param, BandKey: canonicalFrequencyKey(value), Role: role,
				Channel: defaultEQChannel(channel), FixedHz: &value, Confidence: 0.98}, true
		}
		if parameterHasGainDomain(param) && !mayHaveCompactEQRoleSuffix(name) {
			bandKey, dedicatedKind := canonicalEQSectionKey(tokens)
			return eqParsedParameter{Param: param, BandKey: bandKey, Role: eqRoleGain,
				Channel: defaultEQChannel(channel), Confidence: 0.88, DedicatedKind: dedicatedKind}, true
		}
		return eqParsedParameter{}, false
	}
	if role == eqRoleShape {
		role = classifyEQTypeRole(param)
		if role == "" {
			// A field called Type whose enum is only On/Off (or whose labels are
			// otherwise opaque) proves neither topology nor activation.  Ignore it
			// instead of guessing and accidentally changing a non-EQ function.
			return eqParsedParameter{}, false
		}
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
	bandKey, dedicatedKind := canonicalEQSectionKey(remaining)
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
	if role == eqRoleFilterKind && !parameterHasEnumDomain(param) {
		if _, currentKind := parseEQFilterKind(param.ValueText); !currentKind {
			return eqParsedParameter{}, false
		}
	}
	if (role == eqRoleSlope || role == eqRoleDesign) && !parameterHasEnumDomain(param) {
		return eqParsedParameter{}, false
	}

	return eqParsedParameter{Param: param, BandKey: bandKey, Role: role,
		Channel: defaultEQChannel(channel), ActivationKind: activationKind,
		GainPolarity: polarity, Confidence: 0.94, DedicatedKind: dedicatedKind}, true
}

func parseEQFrequencyTransform(param ParameterInfo, tokens []string, channel string) (eqParsedParameter, bool, bool) {
	frequencyIndex := -1
	for index, token := range tokens {
		if token == "frequency" || token == "freq" || token == "frq" || token == "cutoff" {
			frequencyIndex = index
			break
		}
	}
	if frequencyIndex < 0 {
		return eqParsedParameter{}, false, false
	}
	factor, factorStart := 0.0, -1
	for index := frequencyIndex + 1; index+1 < len(tokens); index++ {
		if tokens[index] != "x" {
			continue
		}
		value, err := strconv.ParseFloat(tokens[index+1], 64)
		if err == nil && value > 1 {
			factor, factorStart = value, index
			break
		}
	}
	if factorStart < 0 {
		return eqParsedParameter{}, false, false
	}
	if !parameterHasEnumDomain(param) ||
		(!eqLabelsContainPair(eqParameterLabels(param), "off", "on") &&
			!eqLabelsContainPair(eqParameterLabels(param), "out", "in")) {
		return eqParsedParameter{}, true, false
	}
	remaining := make([]string, 0, len(tokens)-3)
	for index, token := range tokens {
		if index == frequencyIndex || index == factorStart || index == factorStart+1 ||
			token == "band" || token == "eq" || token == "filter" {
			continue
		}
		remaining = append(remaining, token)
	}
	if len(remaining) == 0 {
		return eqParsedParameter{}, true, false
	}
	bandKey, dedicatedKind := canonicalEQSectionKey(remaining)
	return eqParsedParameter{Param: param, BandKey: bandKey, Role: eqRoleFrequencyTransform,
		Channel: defaultEQChannel(channel), TransformFactor: factor,
		Confidence: 0.97, DedicatedKind: dedicatedKind}, true, true
}

func parseEQQualityProjection(param ParameterInfo, tokens []string, channel string) (eqParsedParameter, bool, bool) {
	qualityIndex := -1
	for index, token := range tokens {
		if token == "quality" {
			qualityIndex = index
			break
		}
	}
	if qualityIndex < 0 {
		return eqParsedParameter{}, false, false
	}
	remaining := make([]string, 0, len(tokens)-1)
	for index, token := range tokens {
		if index == qualityIndex || token == "band" || token == "eq" || token == "filter" {
			continue
		}
		remaining = append(remaining, token)
	}
	if len(remaining) == 0 {
		return eqParsedParameter{}, true, false
	}
	bandKey, dedicatedKind := canonicalEQSectionKey(remaining)
	role := ""
	if (dedicatedKind == EQFilterLowCut || dedicatedKind == EQFilterHighCut) && parameterLooksLikeSlope(param) {
		role = eqRoleSlope
	} else if current, ok := parseEQPhysical(eqRoleQ, param.ValueText); ok && current > 0 && parameterHasQDomain(param) {
		// A generic word such as Quality is projected to Q only when the live
		// value and its physical domain are both numeric Q evidence. Shelf or
		// other symbolic states remain unresolved rather than guessed.
		role = eqRoleQ
	}
	if role == "" {
		return eqParsedParameter{}, true, false
	}
	return eqParsedParameter{Param: param, BandKey: bandKey, Role: role,
		Channel: defaultEQChannel(channel), Confidence: 0.95, DedicatedKind: dedicatedKind}, true, true
}

func eqNameTokensAndChannel(name string) ([]string, string) {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	slashSuffixChannel := ""
	if strings.HasSuffix(lowerName, "l/m") {
		slashSuffixChannel = "left"
	} else if strings.HasSuffix(lowerName, "r/s") {
		slashSuffixChannel = "right"
	}
	explicitShortSuffix := strings.HasSuffix(lowerName, "-l") || strings.HasSuffix(lowerName, "_l") ||
		strings.HasSuffix(lowerName, "-r") || strings.HasSuffix(lowerName, "_r")
	prepared := eqCamelBoundary.ReplaceAllString(name, `$1 $2`)
	tokens := eqTokenPattern.FindAllString(prepared, -1)
	for i := range tokens {
		tokens[i] = strings.ToLower(strings.Join(strings.Fields(tokens[i]), ""))
	}
	channel := ""
	filtered := make([]string, 0, len(tokens))
	compoundChannelTokens := map[int]bool{}
	for index := 0; index+1 < len(tokens); index++ {
		if tokens[index] == "l" && tokens[index+1] == "m" {
			channel = "left"
			compoundChannelTokens[index], compoundChannelTokens[index+1] = true, true
		} else if tokens[index] == "r" && tokens[index+1] == "s" {
			channel = "right"
			compoundChannelTokens[index], compoundChannelTokens[index+1] = true, true
		}
	}
	for i, token := range tokens {
		if compoundChannelTokens[i] {
			continue
		}
		if slashSuffixChannel != "" && i >= len(tokens)-2 {
			channel = slashSuffixChannel
			continue
		}
		switch token {
		case "left", "right":
			channel = token
			continue
		case "l", "r":
			if (i == 0 || i == len(tokens)-1 && (token == "r" || explicitShortSuffix)) && len(tokens) > 1 {
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
	return filtered, channel
}

func eqRoleTokens(tokens []string) (string, map[int]bool, string, string) {
	consume := map[int]bool{}
	for i, token := range tokens {
		switch token {
		case "frequency", "freq", "frq", "cutoff":
			for index, candidate := range tokens {
				if candidate == "frequency" || candidate == "freq" || candidate == "frq" || candidate == "cutoff" {
					consume[index] = true
				}
			}
			return eqRoleFrequency, consume, "", ""
		case "gain", "level", "decibels", "decibel", "db":
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
		case "type", "shape", "filter":
			consume[i] = true
			if token == "filter" && i+1 < len(tokens) && tokens[i+1] == "type" {
				consume[i+1] = true
			}
			return eqRoleShape, consume, "", ""
		case "slope":
			consume[i] = true
			return eqRoleSlope, consume, "", ""
		}
	}
	return "", consume, "", ""
}

func classifyEQTypeRole(param ParameterInfo) string {
	labels := eqParameterLabels(param)
	if len(labels) == 0 {
		if _, ok := parseEQFilterKind(param.ValueText); ok {
			return eqRoleFilterKind
		}
		return ""
	}
	kindCount := 0
	for _, label := range labels {
		if _, ok := parseEQFilterKind(label); ok {
			kindCount++
		}
	}
	if kindCount > 0 {
		return eqRoleFilterKind
	}
	for _, label := range labels {
		lower := strings.ToLower(strings.ReplaceAll(label, " ", ""))
		if strings.Contains(lower, "db/oct") || strings.Contains(lower, "dbperoct") || strings.Contains(lower, "db/octave") {
			return eqRoleSlope
		}
	}
	// Design enums are reported as a separate capability but never interpreted
	// as filter shape.  Requiring every label to be recognisable avoids turning
	// arbitrary program/style enums into EQ sections.
	design := 0
	for _, label := range labels {
		lower := strings.ToLower(label)
		if strings.Contains(lower, "digital") || strings.Contains(lower, "vint") ||
			strings.Contains(lower, "analog") || strings.Contains(lower, "modern") || strings.Contains(lower, "modrn") ||
			strings.Contains(lower, "classic") {
			design++
		}
	}
	if design == len(labels) && design > 0 {
		return eqRoleDesign
	}
	return ""
}

func parameterLooksLikeSlope(param ParameterInfo) bool {
	for _, label := range append(eqParameterLabels(param), param.ValueText) {
		if textLooksLikeSlopeUnit(label) {
			return true
		}
	}
	return false
}

func eqParameterLabels(param ParameterInfo) []string {
	if param.DisplayProbe == nil {
		return nil
	}
	labels := make([]string, 0, len(param.DisplayProbe.DiscreteLabels))
	for _, row := range param.DisplayProbe.DiscreteLabels {
		if label := strings.TrimSpace(row.Label); label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		for _, label := range param.DisplayProbe.AllLabels {
			if label = strings.TrimSpace(label); label != "" {
				labels = append(labels, label)
			}
		}
	}
	return uniqueEQStrings(labels)
}

func canonicalEQSectionKey(tokens []string) (string, EQFilterKind) {
	joined := strings.Join(tokens, " ")
	compact := strings.ReplaceAll(joined, " ", "")
	switch compact {
	case "hp", "hpf", "highpass", "hipass", "lowcut", "locut":
		return "low cut", EQFilterLowCut
	case "lp", "lpf", "lowpass", "lopass", "highcut", "hicut":
		return "high cut", EQFilterHighCut
	case "lowshelf", "loshelf":
		return "low shelf", EQFilterLowShelf
	case "highshelf", "hishelf":
		return "high shelf", EQFilterHighShelf
	case "notch":
		return "notch", EQFilterNotch
	case "bell", "peak":
		return "bell", EQFilterBell
	}
	// Tokenisation must not change the semantic kind. Hosts commonly expose
	// the same numbered control as "HighPass 1", "High-pass 1", or
	// "High Pass 1". Collapse adjacent compound tokens while preserving any
	// structural suffix; this extends the existing Cut lexicon without using
	// plug-in identity.
	for index := 0; index+1 < len(tokens); index++ {
		pair := strings.ToLower(tokens[index] + tokens[index+1])
		kind := EQFilterUnknown
		label := ""
		switch pair {
		case "highpass", "hipass", "lowcut", "locut":
			kind, label = EQFilterLowCut, "low cut"
		case "lowpass", "lopass", "highcut", "hicut":
			kind, label = EQFilterHighCut, "high cut"
		}
		if isKnownEQFilterKind(kind) {
			parts := make([]string, 0, len(tokens)-1)
			parts = append(parts, tokens[:index]...)
			parts = append(parts, label)
			parts = append(parts, tokens[index+2:]...)
			return strings.Join(parts, " "), kind
		}
	}
	for index, token := range tokens {
		normalized := strings.ReplaceAll(strings.ToLower(token), " ", "")
		kind := EQFilterUnknown
		label := ""
		switch normalized {
		case "hp", "hpf", "highpass", "hipass", "lowcut", "locut":
			kind, label = EQFilterLowCut, "low cut"
		case "lp", "lpf", "lowpass", "lopass", "highcut", "hicut":
			kind, label = EQFilterHighCut, "high cut"
		}
		if isKnownEQFilterKind(kind) {
			parts := append([]string(nil), tokens...)
			parts[index] = label
			return strings.Join(parts, " "), kind
		}
	}
	// Common band-position abbreviations and their full-word forms describe the
	// same structural key even when followed by a channel/section suffix. This
	// is lexical normalisation only; it carries no manufacturer or plug-in
	// identity knowledge.
	parts := make([]string, 0, len(tokens)+1)
	changed := false
	for _, token := range tokens {
		switch strings.ToLower(token) {
		case "lf":
			parts, changed = append(parts, "low"), true
		case "hf":
			parts, changed = append(parts, "high"), true
		case "lm", "lmf":
			parts, changed = append(parts, "low", "mid"), true
		case "hm", "hmf":
			parts, changed = append(parts, "high", "mid"), true
		default:
			parts = append(parts, token)
		}
	}
	if changed {
		return strings.Join(parts, " "), EQFilterUnknown
	}
	return joined, EQFilterUnknown
}

func parseEQFilterKind(label string) (EQFilterKind, bool) {
	lower := strings.ToLower(strings.TrimSpace(label))
	compact := strings.NewReplacer(" ", "", "-", "", "_", "", "/", "").Replace(lower)
	switch compact {
	case "bell", "peak", "peaking", "pqbell", "parametric":
		return EQFilterBell, true
	case "lowshelf", "loshelf", "lows":
		return EQFilterLowShelf, true
	case "highshelf", "hishelf", "highs":
		return EQFilterHighShelf, true
	case "lowcut", "locut", "highpass", "hipass", "hp":
		return EQFilterLowCut, true
	case "highcut", "hicut", "lowpass", "lopass", "lp":
		return EQFilterHighCut, true
	case "notch", "bandstop", "bandreject":
		return EQFilterNotch, true
	}
	if strings.Contains(compact, "lowshelf") || strings.Contains(compact, "loshelf") {
		return EQFilterLowShelf, true
	}
	if strings.Contains(compact, "highshelf") || strings.Contains(compact, "hishelf") {
		return EQFilterHighShelf, true
	}
	if strings.Contains(compact, "highpass") || strings.Contains(compact, "hipass") {
		return EQFilterLowCut, true
	}
	if strings.Contains(compact, "lowpass") || strings.Contains(compact, "lopass") {
		return EQFilterHighCut, true
	}
	return EQFilterUnknown, false
}

func defaultEQChannel(channel string) string {
	if channel == "" {
		return "shared"
	}
	return channel
}

func buildEQBand(key string, candidates []eqParsedParameter) EQBand {
	band := EQBand{Key: key, Bindings: map[string][]EQBinding{}, Active: true,
		ActivationStrategy: EQActivationAlwaysActive, Complete: true}
	for _, candidate := range candidates {
		binding := bindingFromEQCandidate(candidate)
		band.Bindings[candidate.Role] = append(band.Bindings[candidate.Role], binding)
		if candidate.Role == eqRoleFilterKind {
			legacy := binding
			legacy.Role = eqRoleShape
			band.Bindings[eqRoleShape] = append(band.Bindings[eqRoleShape], legacy)
		}
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
		band.ActivationStrategy = EQActivationExplicit
		if !activationBindingIsActive(binding) {
			band.Active = false
		}
	}
	if len(band.Bindings[eqRoleActivation]) == 0 {
		for _, binding := range band.Bindings[eqRoleFrequency] {
			if eqFrequencyTextIsInactive(binding.CurrentText) {
				band.Active = false
				band.ActivationKnown = true
				band.ActivationStrategy = EQActivationImplicitInDomain
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

func buildEQSection(key string, candidates []eqParsedParameter) EQSection {
	band := buildEQBand(key, candidates)
	section := EQSection{Key: key, DedicatedKind: EQFilterUnknown, Bindings: map[string][]EQBinding{}, Complete: true}
	if band.FixedFrequencyHz != nil {
		value := *band.FixedFrequencyHz
		section.FixedFrequencyHz = &value
	}
	for _, candidate := range candidates {
		binding := bindingFromEQCandidate(candidate)
		section.Bindings[candidate.Role] = append(section.Bindings[candidate.Role], binding)
		if isKnownEQFilterKind(candidate.DedicatedKind) {
			if isKnownEQFilterKind(section.DedicatedKind) && section.DedicatedKind != candidate.DedicatedKind {
				section.Issues = append(section.Issues, "conflicting dedicated filter kinds")
			}
			section.DedicatedKind = candidate.DedicatedKind
		}
	}
	inferDirectionalShelfAlternative(&section)
	section.ReachableKinds = reachableEQKinds(section)
	if !isKnownEQFilterKind(section.DedicatedKind) && len(section.ReachableKinds) == 0 &&
		len(section.Bindings[eqRoleGain]) > 0 &&
		(len(section.Bindings[eqRoleFrequency]) > 0 || band.FixedFrequencyHz != nil) &&
		!sectionOnlyHasOpaqueCompactAnchors(section) {
		// A movable/fixed centre frequency paired with bipolar gain is the
		// established Bell path.  This inference does not manufacture Shelf/Cut
		// capability; those require a dedicated key or an enum label.
		section.DedicatedKind = EQFilterBell
		section.ReachableKinds = []EQFilterKind{EQFilterBell}
	}
	section.Activation = activationForEQSection(section)
	for role, bindings := range section.Bindings {
		if role == eqRoleActivation {
			continue
		}
		seen := map[string]bool{}
		for _, binding := range bindings {
			if seen[binding.Channel] {
				section.Issues = append(section.Issues, fmt.Sprintf("role conflict: %s/%s", role, binding.Channel))
			}
			seen[binding.Channel] = true
		}
	}
	validateEQSectionChannels(&section)
	_, hasFrequency := section.Bindings[eqRoleFrequency]
	hasFrequency = hasFrequency || band.FixedFrequencyHz != nil
	_, hasGain := section.Bindings[eqRoleGain]
	switch sectionPrimaryKind(section) {
	case EQFilterLowCut, EQFilterHighCut, EQFilterNotch:
		section.Complete = hasFrequency
	case EQFilterBell, EQFilterLowShelf, EQFilterHighShelf:
		section.Complete = hasFrequency && hasGain
	default:
		section.Complete = band.Complete
	}
	section.Issues = append(section.Issues, band.Issues...)
	section.Issues = uniqueEQStrings(section.Issues)
	if len(section.Issues) > 0 {
		section.Complete = false
	}
	return section
}

func sectionOnlyHasOpaqueCompactAnchors(section EQSection) bool {
	bindings := append([]EQBinding(nil), section.Bindings[eqRoleFrequency]...)
	bindings = append(bindings, section.Bindings[eqRoleGain]...)
	if len(bindings) == 0 {
		return false
	}
	for _, binding := range bindings {
		name := strings.TrimSpace(binding.Name)
		if name == "" || strings.ContainsAny(name, " -_:/\\") {
			return false
		}
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, "f") && !strings.HasSuffix(lower, "g") {
			return false
		}
	}
	return true
}

func inferDirectionalShelfAlternative(section *EQSection) {
	if section == nil || len(section.Bindings[eqRoleGain]) == 0 ||
		(len(section.Bindings[eqRoleFrequency]) == 0 && section.FixedFrequencyHz == nil) {
		return
	}
	shelf := EQFilterUnknown
	keyTokens := strings.Fields(strings.ToLower(section.Key))
	for _, token := range keyTokens {
		switch token {
		case "low", "lo", "lf":
			shelf = EQFilterLowShelf
		case "high", "hi", "hf":
			shelf = EQFilterHighShelf
		}
	}
	if !isKnownEQFilterKind(shelf) {
		return
	}
	bindings := section.Bindings[eqRoleFilterKind]
	for bindingIndex := range bindings {
		hasBell := false
		for _, value := range bindings[bindingIndex].Reachable {
			hasBell = hasBell || value.Kind == EQFilterBell
		}
		if !hasBell {
			continue
		}
		for valueIndex := range bindings[bindingIndex].Reachable {
			value := &bindings[bindingIndex].Reachable[valueIndex]
			if !isKnownEQFilterKind(value.Kind) && eqInactiveLabel(value.Label) {
				value.Kind = shelf
			}
		}
	}
	section.Bindings[eqRoleFilterKind] = bindings
}

func validateEQSectionChannels(section *EQSection) {
	if section == nil {
		return
	}
	referenceSet := map[string]bool{}
	roles := []string{eqRoleFrequency, eqRoleGain, eqRoleQ, eqRoleFilterKind, eqRoleSlope, eqRoleActivation}
	for _, role := range roles {
		for _, binding := range section.Bindings[role] {
			if binding.Channel != "shared" {
				referenceSet[binding.Channel] = true
			}
		}
	}
	if len(referenceSet) < 2 {
		return
	}
	reference := make([]string, 0, len(referenceSet))
	for channel := range referenceSet {
		reference = append(reference, channel)
	}
	sort.Strings(reference)
	for _, role := range roles {
		bindings := section.Bindings[role]
		if len(bindings) == 0 {
			continue
		}
		shared := false
		for _, binding := range bindings {
			shared = shared || binding.Channel == "shared"
		}
		if shared {
			continue
		}
		if actual := explicitChannelsForBindings(bindings); !equalStringSets(actual, reference) {
			section.Issues = append(section.Issues, "channel bindings missing for role "+role)
		}
	}
}

func reachableEQKinds(section EQSection) []EQFilterKind {
	seen := map[EQFilterKind]bool{}
	if isKnownEQFilterKind(section.DedicatedKind) {
		seen[section.DedicatedKind] = true
	}
	for _, binding := range section.Bindings[eqRoleFilterKind] {
		if kind, ok := parseEQFilterKind(binding.CurrentText); ok {
			seen[kind] = true
		}
		for _, value := range binding.Reachable {
			if isKnownEQFilterKind(value.Kind) {
				seen[value.Kind] = true
				continue
			}
			if kind, ok := parseEQFilterKind(value.Label); ok {
				seen[kind] = true
				continue
			}
			if strings.EqualFold(strings.TrimSpace(value.Label), "shelf") {
				keyTokens := strings.Fields(strings.ToLower(section.Key))
				for _, token := range keyTokens {
					switch token {
					case "low", "lo", "lf":
						seen[EQFilterLowShelf] = true
					case "high", "hi", "hf":
						seen[EQFilterHighShelf] = true
					}
				}
			}
		}
	}
	out := make([]EQFilterKind, 0, len(seen))
	for kind := range seen {
		out = append(out, kind)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sectionPrimaryKind(section EQSection) EQFilterKind {
	if isKnownEQFilterKind(section.DedicatedKind) {
		return section.DedicatedKind
	}
	if len(section.ReachableKinds) == 1 {
		return section.ReachableKinds[0]
	}
	return EQFilterUnknown
}

func isKnownEQFilterKind(kind EQFilterKind) bool {
	return kind != "" && kind != EQFilterUnknown
}

func activationForEQSection(section EQSection) EQActivation {
	bindings := append([]EQBinding(nil), section.Bindings[eqRoleActivation]...)
	if len(bindings) > 0 {
		active := true
		for _, binding := range bindings {
			active = active && activationBindingIsActive(binding)
		}
		return EQActivation{Strategy: EQActivationExplicit, Active: active, Known: true, Bindings: bindings}
	}
	for _, binding := range section.Bindings[eqRoleFrequency] {
		if eqFrequencyTextIsInactive(binding.CurrentText) {
			return EQActivation{Strategy: EQActivationImplicitInDomain, Active: false, Known: true}
		}
		for _, value := range binding.Reachable {
			if eqInactiveLabel(value.Label) {
				return EQActivation{Strategy: EQActivationImplicitInDomain, Active: true, Known: true}
			}
		}
	}
	if kind := sectionPrimaryKind(section); kind == EQFilterLowCut || kind == EQFilterHighCut {
		bindings := section.Bindings[eqRoleSlope]
		if len(bindings) > 0 {
			active := true
			for _, binding := range bindings {
				if !slopeBindingHasPropertySentinel(binding) {
					return EQActivation{Strategy: EQActivationAlwaysActive, Active: true, Known: true}
				}
				active = active && !eqInactiveLabel(binding.CurrentText)
			}
			return EQActivation{Strategy: EQActivationPropertySentinel, BindingRole: eqRoleSlope,
				Active: active, Known: true, Bindings: append([]EQBinding(nil), bindings...)}
		}
	}
	return EQActivation{Strategy: EQActivationAlwaysActive, Active: true, Known: true}
}

func slopeBindingHasPropertySentinel(binding EQBinding) bool {
	inactive, activeSlope := false, false
	for _, value := range binding.Reachable {
		if eqInactiveLabel(value.Label) {
			inactive = true
			continue
		}
		activeSlope = activeSlope || value.Physical != nil && *value.Physical > 0
	}
	return inactive && activeSlope
}

func eqFrequencyTextIsInactive(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return lower == "off" || strings.HasPrefix(lower, "off ") ||
		lower == "out" || strings.HasPrefix(lower, "out ")
}

func bindingFromEQCandidate(candidate eqParsedParameter) EQBinding {
	param := candidate.Param
	reachable := reachableValuesForEQParam(param, candidate.Role)
	if isKnownEQFilterKind(candidate.NamedKind) {
		for i := range reachable {
			if eqLabelMeansActive(reachable[i].Label) {
				reachable[i].Kind = candidate.NamedKind
			}
		}
	}
	if candidate.Role == eqRoleFrequencyTransform {
		for index := range reachable {
			factor, ok := frequencyTransformFactorForLabel(reachable[index].Label, candidate.TransformFactor)
			if ok {
				reachable[index].Physical = &factor
			}
		}
	}
	binding := EQBinding{ParamID: param.ID, Name: eqParamDisplayName(param), Role: candidate.Role,
		Channel: candidate.Channel, ActivationKind: candidate.ActivationKind,
		GainPolarity: candidate.GainPolarity, TransformFactor: candidate.TransformFactor, CurrentText: param.ValueText,
		CurrentNormalized: anyFloat(param.NormalizedValue), Domain: param.DisplayDomainCandidate,
		Curve: eqCurveFromProbe(param), Reachable: reachable}
	if value, ok := parseEQPhysical(candidate.Role, param.ValueText); ok {
		binding.CurrentPhysical = &value
	} else if candidate.Role == eqRoleFrequencyTransform {
		if factor, ok := frequencyTransformFactorForLabel(param.ValueText, candidate.TransformFactor); ok {
			binding.CurrentPhysical = &factor
		}
	}
	return binding
}

func frequencyTransformFactorForLabel(label string, activeFactor float64) (float64, bool) {
	lower := strings.ToLower(strings.TrimSpace(label))
	if eqInactiveLabel(lower) || lower == "false" {
		return 1, true
	}
	if eqLabelMeansActive(lower) || lower == "true" {
		return activeFactor, activeFactor > 1
	}
	return 0, false
}

func eqLabelMeansActive(label string) bool {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "in", "on", "enabled", "active", "used":
		return true
	}
	return false
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
		if physical, ok := parseEQPhysical(role, row.Label); ok &&
			(role != eqRoleSlope || slopePhysicalLabelAllowed(param, row.Label)) {
			item.Physical = &physical
		}
		out = append(out, item)
	}
	if len(out) == 0 && param.NumSteps > 1 && len(probe.AllLabels) == param.NumSteps {
		for i, label := range probe.AllLabels {
			normalized := float64(i) / float64(param.NumSteps-1)
			item := EQReachableValue{Normalized: normalized, Label: label}
			if physical, ok := parseEQPhysical(role, label); ok &&
				(role != eqRoleSlope || slopePhysicalLabelAllowed(param, label)) {
				item.Physical = &physical
			}
			out = append(out, item)
		}
	}
	if role == eqRoleFrequency {
		for _, sample := range probe.Samples {
			if !eqFrequencyTextIsInactive(sample.Text) {
				continue
			}
			duplicate := false
			for _, value := range out {
				duplicate = duplicate || nearlyEqual(value.Normalized, sample.NormalizedValue)
			}
			if !duplicate {
				out = append(out, EQReachableValue{Normalized: sample.NormalizedValue, Label: sample.Text})
			}
		}
	}
	return out
}

func textLooksLikeSlopeUnit(text string) bool {
	lower := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(text), " ", ""))
	return strings.Contains(lower, "db/o") || strings.Contains(lower, "db/oct") ||
		strings.Contains(lower, "dbperoct") || strings.Contains(lower, "db/octave")
}

func slopePhysicalLabelAllowed(param ParameterInfo, label string) bool {
	// A parameter explicitly named Slope already owns the physical role, so
	// compact displays such as "6 dB" remain valid.  Generic Quality controls
	// are projected to Slope only from explicit dB/oct evidence; their numeric
	// Q alternatives must not become slope values.
	name := strings.ToLower(eqParamDisplayName(param))
	return !strings.Contains(name, "quality") || textLooksLikeSlopeUnit(label)
}

func parameterHasFrequencyDomain(param ParameterInfo) bool {
	if values := reachableValuesForEQParam(param, eqRoleFrequency); len(values) > 0 {
		for _, value := range values {
			if value.Physical != nil {
				return true
			}
		}
	}
	if param.DisplayProbe != nil {
		for _, sample := range param.DisplayProbe.Samples {
			if value, ok := parseFrequencyText(sample.Text); ok && value >= eqFreqPlausibleMax {
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
		for _, value := range values {
			if value.Physical != nil {
				return true
			}
		}
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
	case eqRoleGain, eqRoleQ, eqRoleSlope:
		return ParseEQLocalizedNumber(text)
	}
	return 0, false
}

// ParseEQLocalizedNumber parses the first decimal number from an EQ display,
// accepting either a decimal point or decimal comma. Keeping this helper
// shared ensures topology discovery and execution readback use identical
// physical-value semantics.
func ParseEQLocalizedNumber(text string) (float64, bool) {
	match := eqNumberPattern.FindString(strings.TrimSpace(text))
	if match == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match, ",", "."), 64)
	return value, err == nil
}

func parseFrequencyText(text string) (float64, bool) {
	return ParseEQFrequencyText(text)
}

// ParseEQFrequencyText parses a physical frequency display. In addition to
// explicit kHz labels, some plug-ins use a bare SI "k" suffix while reporting
// the parameter unit separately as Hz (for example, "3.82k"). This function
// is intentionally frequency-specific so the multiplier cannot leak into
// gain, Q, or other unitless parameter domains.
func ParseEQFrequencyText(text string) (float64, bool) {
	lower := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(text), " ", ""))
	if parts := eqEngineeringK.FindStringSubmatch(lower); len(parts) == 3 {
		whole, wholeErr := strconv.ParseFloat(parts[1], 64)
		fraction, fractionErr := strconv.ParseFloat(parts[2], 64)
		if wholeErr == nil && fractionErr == nil {
			scale := math.Pow10(len(parts[2]))
			value := (whole + math.Copysign(fraction/scale, whole)) * 1000
			if value > 0 {
				return value, true
			}
		}
	}
	match := eqNumberPattern.FindString(lower)
	if match == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match, ",", "."), 64)
	if err != nil {
		return 0, false
	}
	matchIndex := strings.Index(lower, match)
	suffix := ""
	if matchIndex >= 0 {
		suffix = lower[matchIndex+len(match):]
	}
	if strings.HasPrefix(suffix, "khz") || suffix == "k" {
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
		if !band.Active && len(band.Bindings[eqRoleActivation]) == 0 &&
			band.ActivationStrategy != EQActivationImplicitInDomain {
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

func mergeEQSectionCapabilities(out *EQCapabilities, sections []EQSection) {
	if out == nil {
		return
	}
	for _, section := range sections {
		if !section.Complete {
			continue
		}
		out.Shape = out.Shape || len(section.ReachableKinds) > 0
		out.Slope = out.Slope || len(section.Bindings[eqRoleSlope]) > 0
		out.FilterDesign = out.FilterDesign || len(section.Bindings[eqRoleDesign]) > 0
		out.Activation = out.Activation || section.Activation.Strategy != EQActivationAlwaysActive
	}
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

func channelsForEQSections(sections []EQSection) []string {
	seen := map[string]bool{}
	for _, section := range sections {
		for _, bindings := range section.Bindings {
			for _, binding := range bindings {
				if binding.Channel != "shared" {
					seen[binding.Channel] = true
				}
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
	sections := make([]EQSection, 0, len(out))
	for _, band := range out {
		section := EQSection{Key: band.Key, Primary: true, DedicatedKind: EQFilterBell,
			ReachableKinds: []EQFilterKind{EQFilterBell}, FixedFrequencyHz: band.FixedFrequencyHz,
			Bindings: band.Bindings, Activation: EQActivation{Strategy: EQActivationAlwaysActive, Active: true, Known: true},
			Complete: band.Complete, Issues: append([]string(nil), band.Issues...)}
		sections = append(sections, section)
	}
	model := &EQModel{Classification: "fixed_freq", MappingSource: "generic_structural",
		Confidence: 0.82, Bands: out, Sections: sections, Capabilities: capabilitiesForEQBands(out),
		SetEQPointSupported: true}
	model.Completeness = validateEQMatrix(out, len(out))
	deriveEQControlTopology(model)
	model.SetEQPointSupported = anyEQShapeUpsert(model.ControlTopology.ShapeCapabilities)
	if !model.SetEQPointSupported {
		model.Reason = "no universally safe static EQ shape is provably controllable"
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
