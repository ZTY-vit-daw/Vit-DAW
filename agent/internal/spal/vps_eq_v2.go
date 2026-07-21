package spal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// EnumParameterBinding records only enum states that passed capability-level
// conformance.  Observed labels that are absent here remain non-dispatchable.
type EnumParameterBinding struct {
	ParameterID string             `json:"parameter_id"`
	Values      map[string]float64 `json:"values"`
}

type EQV2BandBinding struct {
	ComponentID string `json:"component_id"`
	// Allocated is an optional plug-in-specific resource-acquisition control.
	// Some EQs expose a Band only after a separate Used/Allocated switch is
	// asserted. It belongs to the action implementation, not to the semantic
	// instruction, and is written before the ordinary band controls.
	Allocated     *ParameterBinding    `json:"allocated,omitempty"`
	Enabled       ParameterBinding     `json:"enabled"`
	ResponseShape EnumParameterBinding `json:"response_shape"`
	FrequencyHz   ParameterBinding     `json:"frequency_hz"`
	GainDB        ParameterBinding     `json:"gain_db"`
	Q             ParameterBinding     `json:"q"`
	Dynamic       *EQV2DynamicBinding  `json:"dynamic,omitempty"`
}

type EQV2DynamicBinding struct {
	Mode        EnumParameterBinding `json:"dynamics_mode"`
	Routing     EnumParameterBinding `json:"routing_scope"`
	ThresholdDB ParameterBinding     `json:"threshold_db"`
	Ratio       *ParameterBinding    `json:"ratio,omitempty"`
	AttackMS    *ParameterBinding    `json:"attack_ms,omitempty"`
	ReleaseMS   *ParameterBinding    `json:"release_ms,omitempty"`
}

type EQV2PassFilterBinding struct {
	ComponentID string `json:"component_id"`
	// Allocated and ResponseShape make it possible to realize a semantic pass
	// filter through an EQ band instead of a dedicated HP/LP module. When
	// present, the adapter selects the requested highpass/lowpass enum state
	// and allocates that band before setting cutoff and slope.
	Allocated         *ParameterBinding     `json:"allocated,omitempty"`
	ResponseShape     *EnumParameterBinding `json:"response_shape,omitempty"`
	Enabled           ParameterBinding      `json:"enabled"`
	CutoffFrequencyHz ParameterBinding      `json:"cutoff_frequency_hz"`
	SlopeDBPerOctave  EnumParameterBinding  `json:"slope_db_per_octave"`
}

type EQV2OutputBinding struct {
	ComponentID  string            `json:"component_id"`
	Bypass       *ParameterBinding `json:"bypass,omitempty"`
	DryMix       *ParameterBinding `json:"dry_mix_percent,omitempty"`
	OutputGainDB *ParameterBinding `json:"output_gain_db,omitempty"`
}

// EQV2Binding is the persisted work-card capability matrix.  A non-nil raw
// mapping is insufficient: the corresponding schema must also be present in
// ConformedSchemas before the adapter advertises or dispatches it.
type EQV2Binding struct {
	ConformedSchemas []string                   `json:"conformed_schemas"`
	Bands            map[string]EQV2BandBinding `json:"bands,omitempty"`
	HighPass         *EQV2PassFilterBinding     `json:"highpass,omitempty"`
	LowPass          *EQV2PassFilterBinding     `json:"lowpass,omitempty"`
	Output           *EQV2OutputBinding         `json:"output,omitempty"`
}

type EQV2ProviderDefinition struct {
	Descriptor   ProviderDescriptor `json:"descriptor"`
	VPSID        string             `json:"vps_id"`
	CredentialID string             `json:"credential_id"`
	Binding      EQV2Binding        `json:"binding"`
}

type VPSEQV2Adapter struct{ definition EQV2ProviderDefinition }

func NewVPSEQV2Adapter(definition EQV2ProviderDefinition) (*VPSEQV2Adapter, error) {
	if err := definition.valid(); err != nil {
		return nil, err
	}
	return &VPSEQV2Adapter{definition: cloneEQV2ProviderDefinition(definition)}, nil
}

func (a *VPSEQV2Adapter) Descriptor() ProviderDescriptor {
	if a == nil {
		return ProviderDescriptor{}
	}
	d := a.definition.Descriptor
	d.SupportedSchemas = append([]string(nil), d.SupportedSchemas...)
	d.ConformanceEvidence = append([]string(nil), d.ConformanceEvidence...)
	return d
}

func (a *VPSEQV2Adapter) Bind(instance ProviderInstance, instruction Instruction) (RuntimeBinding, error) {
	if a == nil {
		return RuntimeBinding{}, fmt.Errorf("VPS EQ v2 adapter is nil")
	}
	if err := instruction.Validate(); err != nil {
		return RuntimeBinding{}, err
	}
	if !supports(a.definition.Descriptor, instruction.SchemaID) || !containsString(a.definition.Binding.ConformedSchemas, instruction.SchemaID) {
		return RuntimeBinding{}, fmt.Errorf("VPS EQ v2 Credential has not conformed schema %s", instruction.SchemaID)
	}
	if err := instance.Valid(); err != nil {
		return RuntimeBinding{}, err
	}
	if instance.ProviderID != a.definition.Descriptor.ID || instance.Status != InstanceVerified ||
		!strings.EqualFold(instance.Metadata["vps_credential_id"], a.definition.CredentialID) ||
		!strings.EqualFold(instance.Metadata["vps_id"], a.definition.VPSID) ||
		!strings.EqualFold(instance.Metadata["eq_v2_ready"], "true") {
		return RuntimeBinding{}, fmt.Errorf("project Provider Instance no longer matches the verified VPS EQ v2 Credential")
	}
	bindings, invariants, err := a.bindingsFor(instruction)
	if err != nil {
		return RuntimeBinding{}, err
	}
	invariants["vps_id"] = a.definition.VPSID
	invariants["vps_credential_id"] = a.definition.CredentialID
	invariants["eq_v2_ready"] = "true"
	binding := RuntimeBinding{
		ID:       "binding:" + instance.ID + ":" + instruction.SchemaID,
		SchemaID: instruction.SchemaID, Provider: a.Descriptor(), Instance: instance,
		ParameterBindings: bindings, Invariants: invariants,
	}
	if err := binding.Valid(); err != nil {
		return RuntimeBinding{}, err
	}
	return binding, nil
}

func (a *VPSEQV2Adapter) Compile(instruction Instruction, binding RuntimeBinding) ([]PhysicalParameter, error) {
	if a == nil {
		return nil, fmt.Errorf("VPS EQ v2 adapter is nil")
	}
	if err := instruction.Validate(); err != nil {
		return nil, err
	}
	if err := binding.Valid(); err != nil {
		return nil, err
	}
	if binding.Provider.ID != a.definition.Descriptor.ID || binding.SchemaID != instruction.SchemaID ||
		!strings.EqualFold(binding.Invariants["vps_credential_id"], a.definition.CredentialID) ||
		!strings.EqualFold(binding.Invariants["eq_v2_ready"], "true") {
		return nil, fmt.Errorf("binding is not for the selected verified VPS EQ v2 Credential")
	}
	expected, _, err := a.bindingsFor(instruction)
	if err != nil {
		return nil, err
	}
	if !sameParameterBindingMap(binding.ParameterBindings, expected) {
		return nil, fmt.Errorf("runtime binding no longer matches the verified EQ v2 mapping")
	}
	switch instruction.SchemaID {
	case EQBandPatchControlID:
		return a.compileBand(instruction, binding)
	case EQPassFilterPatchControlID:
		return a.compilePassFilter(instruction, binding)
	case EQDynamicBandPatchControlID:
		return a.compileDynamic(instruction, binding)
	case EQOutputPatchControlID:
		return a.compileOutput(instruction, binding)
	default:
		return nil, fmt.Errorf("VPS EQ v2 adapter does not implement %s", instruction.SchemaID)
	}
}

func (a *VPSEQV2Adapter) bindingsFor(instruction Instruction) (map[string]ParameterBinding, map[string]string, error) {
	bindings := map[string]ParameterBinding{}
	invariants := map[string]string{}
	switch instruction.SchemaID {
	case EQBandPatchControlID:
		bandRef := strings.ToLower(strings.TrimSpace(instruction.StringParameters["band_ref"]))
		band, ok := a.definition.Binding.Bands[bandRef]
		if !ok {
			return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed Band %s", bandRef)
		}
		shape := strings.ToLower(strings.TrimSpace(instruction.StringParameters["response_shape"]))
		shapeValue, ok := enumValue(band.ResponseShape, shape)
		if !ok {
			return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed %s on Band %s", shape, bandRef)
		}
		bindings = map[string]ParameterBinding{
			"enabled": band.Enabled, "response_shape": enumAsParameter(band.ResponseShape),
			"frequency_hz": band.FrequencyHz, "gain_db": band.GainDB, "q": band.Q,
		}
		if band.Allocated != nil {
			bindings["allocated"] = *band.Allocated
		}
		invariants["band_ref"], invariants["component_id"] = bandRef, band.ComponentID
		invariants["response_shape"] = shape
		invariants["response_shape_normalized"] = strconv.FormatFloat(shapeValue, 'g', -1, 64)
	case EQPassFilterPatchControlID:
		kind := strings.ToLower(strings.TrimSpace(instruction.StringParameters["filter_kind"]))
		var filter *EQV2PassFilterBinding
		if kind == "highpass" {
			filter = a.definition.Binding.HighPass
		} else if kind == "lowpass" {
			filter = a.definition.Binding.LowPass
		}
		if filter == nil {
			return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed %s", kind)
		}
		slopeKey := canonicalNumberKey(instruction.Parameters["slope_db_per_octave"])
		slopeValue, ok := enumValue(filter.SlopeDBPerOctave, slopeKey)
		if !ok {
			return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed %s dB/oct for %s", slopeKey, kind)
		}
		bindings = map[string]ParameterBinding{"enabled": filter.Enabled, "cutoff_frequency_hz": filter.CutoffFrequencyHz, "slope_db_per_octave": enumAsParameter(filter.SlopeDBPerOctave)}
		if filter.Allocated != nil {
			bindings["allocated"] = *filter.Allocated
		}
		if filter.ResponseShape != nil {
			shapeValue, supported := enumValue(*filter.ResponseShape, kind)
			if !supported {
				return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed %s response shape", kind)
			}
			bindings["response_shape"] = enumAsParameter(*filter.ResponseShape)
			invariants["response_shape_normalized"] = strconv.FormatFloat(shapeValue, 'g', -1, 64)
		}
		invariants["filter_kind"], invariants["component_id"] = kind, filter.ComponentID
		invariants["slope_normalized"] = strconv.FormatFloat(slopeValue, 'g', -1, 64)
	case EQDynamicBandPatchControlID:
		bandRef := strings.ToLower(strings.TrimSpace(instruction.StringParameters["band_ref"]))
		band, ok := a.definition.Binding.Bands[bandRef]
		if !ok || band.Dynamic == nil {
			return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed generic dynamics on Band %s", bandRef)
		}
		dynamic := band.Dynamic
		mode := strings.ToLower(strings.TrimSpace(instruction.StringParameters["dynamics_mode"]))
		modeValue, ok := enumValue(dynamic.Mode, mode)
		if !ok {
			return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed dynamics mode %s", mode)
		}
		routing := strings.ToLower(strings.TrimSpace(instruction.StringParameters["routing_scope"]))
		routingValue, ok := enumValue(dynamic.Routing, routing)
		if !ok || routing != "independent" {
			return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed independent dynamic routing")
		}
		bindings = map[string]ParameterBinding{"dynamics_mode": enumAsParameter(dynamic.Mode), "routing_scope": enumAsParameter(dynamic.Routing), "threshold_db": dynamic.ThresholdDB}
		for key, pointer := range map[string]*ParameterBinding{"ratio": dynamic.Ratio, "attack_ms": dynamic.AttackMS, "release_ms": dynamic.ReleaseMS} {
			if _, requested := instruction.Parameters[key]; requested {
				if pointer == nil {
					return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed dynamic %s", key)
				}
				bindings[key] = *pointer
			}
		}
		invariants["band_ref"], invariants["component_id"] = bandRef, band.ComponentID
		invariants["dynamics_mode_normalized"] = strconv.FormatFloat(modeValue, 'g', -1, 64)
		invariants["routing_scope_normalized"] = strconv.FormatFloat(routingValue, 'g', -1, 64)
	case EQOutputPatchControlID:
		if a.definition.Binding.Output == nil {
			return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed output control")
		}
		out := a.definition.Binding.Output
		for key, pointer := range map[string]*ParameterBinding{"bypass": out.Bypass, "dry_mix_percent": out.DryMix, "output_gain_db": out.OutputGainDB} {
			if _, requested := instruction.Parameters[key]; requested {
				if pointer == nil {
					return nil, nil, fmt.Errorf("EQ v2 Credential has not conformed output %s", key)
				}
				bindings[key] = *pointer
			}
		}
		invariants["component_id"] = out.ComponentID
	default:
		return nil, nil, fmt.Errorf("unsupported EQ v2 schema %s", instruction.SchemaID)
	}
	return bindings, invariants, nil
}

func (a *VPSEQV2Adapter) compileBand(i Instruction, b RuntimeBinding) ([]PhysicalParameter, error) {
	enabled := 1.0
	if value, ok := i.Parameters["enabled"]; ok {
		enabled = value
	}
	shape, err := strconv.ParseFloat(b.Invariants["response_shape_normalized"], 64)
	if err != nil {
		return nil, err
	}
	values := []valuePair{}
	if _, allocated := b.ParameterBindings["allocated"]; allocated {
		values = append(values, valuePair{"allocated", 1})
	}
	values = append(values, valuePair{"enabled", enabled}, valuePair{"response_shape", shape}, valuePair{"frequency_hz", i.Parameters["frequency_hz"]}, valuePair{"gain_db", i.Parameters["gain_db"]}, valuePair{"q", i.Parameters["q"]})
	return compileOrdered(b, values)
}

func (a *VPSEQV2Adapter) compilePassFilter(i Instruction, b RuntimeBinding) ([]PhysicalParameter, error) {
	slope, err := strconv.ParseFloat(b.Invariants["slope_normalized"], 64)
	if err != nil {
		return nil, err
	}
	values := []valuePair{}
	if _, allocated := b.ParameterBindings["allocated"]; allocated {
		values = append(values, valuePair{"allocated", 1})
	}
	if _, shapeBound := b.ParameterBindings["response_shape"]; shapeBound {
		shape, shapeErr := strconv.ParseFloat(b.Invariants["response_shape_normalized"], 64)
		if shapeErr != nil {
			return nil, shapeErr
		}
		values = append(values, valuePair{"response_shape", shape})
	}
	values = append(values, valuePair{"enabled", i.Parameters["enabled"]}, valuePair{"cutoff_frequency_hz", i.Parameters["cutoff_frequency_hz"]}, valuePair{"slope_db_per_octave", slope})
	return compileOrdered(b, values)
}

func (a *VPSEQV2Adapter) compileDynamic(i Instruction, b RuntimeBinding) ([]PhysicalParameter, error) {
	mode, err := strconv.ParseFloat(b.Invariants["dynamics_mode_normalized"], 64)
	if err != nil {
		return nil, err
	}
	routing, err := strconv.ParseFloat(b.Invariants["routing_scope_normalized"], 64)
	if err != nil {
		return nil, err
	}
	values := []valuePair{{"dynamics_mode", mode}, {"routing_scope", routing}, {"threshold_db", i.Parameters["threshold_db"]}}
	for _, key := range []string{"ratio", "attack_ms", "release_ms"} {
		if value, ok := i.Parameters[key]; ok {
			values = append(values, valuePair{key, value})
		}
	}
	return compileOrdered(b, values)
}

func (a *VPSEQV2Adapter) compileOutput(i Instruction, b RuntimeBinding) ([]PhysicalParameter, error) {
	values := []valuePair{}
	for _, key := range []string{"bypass", "dry_mix_percent", "output_gain_db"} {
		if value, ok := i.Parameters[key]; ok {
			values = append(values, valuePair{key, value})
		}
	}
	return compileOrdered(b, values)
}

type valuePair struct {
	key   string
	value float64
}

func compileOrdered(binding RuntimeBinding, values []valuePair) ([]PhysicalParameter, error) {
	rows := make([]PhysicalParameter, 0, len(values))
	for _, item := range values {
		parameter, ok := binding.ParameterBindings[item.key]
		if !ok {
			return nil, fmt.Errorf("binding omits %s", item.key)
		}
		value := item.value
		if item.key != "response_shape" && item.key != "slope_db_per_octave" && item.key != "dynamics_mode" && item.key != "routing_scope" {
			normalized, err := staticEQNormalizedValue(parameter, value)
			if err != nil {
				return nil, fmt.Errorf("EQ v2 %s: %w", item.key, err)
			}
			value = normalized
		}
		rows = append(rows, PhysicalParameter{ParameterID: parameter.ParameterID, Value: value, Unit: parameter.Unit})
	}
	return rows, nil
}

func (d EQV2ProviderDefinition) valid() error {
	if strings.TrimSpace(d.VPSID) == "" || strings.TrimSpace(d.CredentialID) == "" {
		return fmt.Errorf("VPS EQ v2 definition requires VPS and Credential IDs")
	}
	if err := d.Descriptor.Valid(); err != nil {
		return err
	}
	if d.Descriptor.Status != ProviderVerified {
		return fmt.Errorf("VPS EQ v2 Provider must be verified")
	}
	if len(d.Binding.ConformedSchemas) == 0 {
		return fmt.Errorf("VPS EQ v2 binding has no conformed schemas")
	}
	for _, schema := range d.Binding.ConformedSchemas {
		if !oneOf(schema, EQBandPatchControlID, EQPassFilterPatchControlID, EQDynamicBandPatchControlID, EQOutputPatchControlID) || !supports(d.Descriptor, schema) {
			return fmt.Errorf("invalid or unadvertised conformed EQ v2 schema %s", schema)
		}
	}
	for ref, band := range d.Binding.Bands {
		if err := validateBandBinding(ref, band); err != nil {
			return err
		}
	}
	if containsString(d.Binding.ConformedSchemas, EQBandPatchControlID) && len(d.Binding.Bands) == 0 {
		return fmt.Errorf("conformed EQ Band schema has no Band bindings")
	}
	if containsString(d.Binding.ConformedSchemas, EQPassFilterPatchControlID) && d.Binding.HighPass == nil && d.Binding.LowPass == nil {
		return fmt.Errorf("conformed pass-filter schema has no HP/LP binding")
	}
	for name, filter := range map[string]*EQV2PassFilterBinding{"highpass": d.Binding.HighPass, "lowpass": d.Binding.LowPass} {
		if filter == nil {
			continue
		}
		if strings.TrimSpace(filter.ComponentID) == "" {
			return fmt.Errorf("EQ v2 %s binding requires component_id", name)
		}
		if err := validStaticEQParameterBinding(name+" enabled", filter.Enabled); err != nil {
			return err
		}
		if err := validStaticEQParameterBinding(name+" cutoff", filter.CutoffFrequencyHz); err != nil {
			return err
		}
		if err := validEnumBinding(name+" slope", filter.SlopeDBPerOctave); err != nil {
			return err
		}
		if filter.Allocated != nil {
			if err := validStaticEQParameterBinding(name+" allocation", *filter.Allocated); err != nil {
				return err
			}
		}
		if filter.ResponseShape != nil {
			if filter.Allocated == nil {
				return fmt.Errorf("EQ v2 %s band-backed filter requires allocation binding", name)
			}
			if err := validEnumBinding(name+" response shape", *filter.ResponseShape); err != nil {
				return err
			}
			if _, found := enumValue(*filter.ResponseShape, name); !found {
				return fmt.Errorf("EQ v2 %s response shape binding omits %s", name, name)
			}
		}
	}
	if containsString(d.Binding.ConformedSchemas, EQDynamicBandPatchControlID) {
		found := false
		for _, band := range d.Binding.Bands {
			if band.Dynamic != nil {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("conformed dynamic schema has no dynamic Band binding")
		}
	}
	if containsString(d.Binding.ConformedSchemas, EQOutputPatchControlID) && d.Binding.Output == nil {
		return fmt.Errorf("conformed output schema has no output binding")
	}
	if output := d.Binding.Output; output != nil {
		if strings.TrimSpace(output.ComponentID) == "" {
			return fmt.Errorf("EQ v2 output binding requires component_id")
		}
		count := 0
		for key, pointer := range map[string]*ParameterBinding{"bypass": output.Bypass, "dry_mix_percent": output.DryMix, "output_gain_db": output.OutputGainDB} {
			if pointer == nil {
				continue
			}
			count++
			if err := validStaticEQParameterBinding(key, *pointer); err != nil {
				return err
			}
		}
		if count == 0 {
			return fmt.Errorf("EQ v2 output binding has no controls")
		}
	}
	return nil
}

func validateBandBinding(ref string, band EQV2BandBinding) error {
	if !oneOf(strings.ToLower(strings.TrimSpace(ref)), "b1", "b2", "b3", "b4") || strings.TrimSpace(band.ComponentID) == "" {
		return fmt.Errorf("invalid EQ v2 Band binding %q", ref)
	}
	for key, binding := range map[string]ParameterBinding{"enabled": band.Enabled, "frequency_hz": band.FrequencyHz, "gain_db": band.GainDB, "q": band.Q} {
		if err := validStaticEQParameterBinding(key, binding); err != nil {
			return err
		}
	}
	if band.Allocated != nil {
		if err := validStaticEQParameterBinding("allocated", *band.Allocated); err != nil {
			return err
		}
	}
	if err := validEnumBinding("response_shape", band.ResponseShape); err != nil {
		return err
	}
	if band.Dynamic != nil {
		if err := validEnumBinding("dynamics_mode", band.Dynamic.Mode); err != nil {
			return err
		}
		if err := validEnumBinding("routing_scope", band.Dynamic.Routing); err != nil {
			return err
		}
		if err := validStaticEQParameterBinding("threshold_db", band.Dynamic.ThresholdDB); err != nil {
			return err
		}
		for key, pointer := range map[string]*ParameterBinding{"ratio": band.Dynamic.Ratio, "attack_ms": band.Dynamic.AttackMS, "release_ms": band.Dynamic.ReleaseMS} {
			if pointer != nil {
				if err := validStaticEQParameterBinding(key, *pointer); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validEnumBinding(name string, binding EnumParameterBinding) error {
	if strings.TrimSpace(binding.ParameterID) == "" || len(binding.Values) == 0 {
		return fmt.Errorf("EQ v2 %s enum binding is empty", name)
	}
	for key, value := range binding.Values {
		if strings.TrimSpace(key) == "" || !finite(value) || value < 0 || value > 1 {
			return fmt.Errorf("EQ v2 %s enum value %q is invalid", name, key)
		}
	}
	return nil
}

func enumAsParameter(binding EnumParameterBinding) ParameterBinding {
	return ParameterBinding{ParameterID: binding.ParameterID, Min: 0, Max: 1, Scale: "linear"}
}
func enumValue(binding EnumParameterBinding, key string) (float64, bool) {
	value, ok := binding.Values[strings.ToLower(strings.TrimSpace(key))]
	return value, ok
}
func canonicalNumberKey(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }
func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}

func sameParameterBindingMap(left, right map[string]ParameterBinding) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if other, ok := right[key]; !ok || value != other {
			return false
		}
	}
	return true
}

func cloneEQV2ProviderDefinition(in EQV2ProviderDefinition) EQV2ProviderDefinition {
	out := in
	out.Descriptor.SupportedSchemas = append([]string(nil), in.Descriptor.SupportedSchemas...)
	out.Descriptor.ConformanceEvidence = append([]string(nil), in.Descriptor.ConformanceEvidence...)
	out.Binding.ConformedSchemas = append([]string(nil), in.Binding.ConformedSchemas...)
	sort.Strings(out.Binding.ConformedSchemas)
	out.Binding.Bands = make(map[string]EQV2BandBinding, len(in.Binding.Bands))
	for key, band := range in.Binding.Bands {
		if band.Allocated != nil {
			allocated := *band.Allocated
			band.Allocated = &allocated
		}
		band.ResponseShape.Values = cloneStringFloatMap(band.ResponseShape.Values)
		if band.Dynamic != nil {
			dynamic := *band.Dynamic
			dynamic.Mode.Values = cloneStringFloatMap(dynamic.Mode.Values)
			dynamic.Routing.Values = cloneStringFloatMap(dynamic.Routing.Values)
			band.Dynamic = &dynamic
		}
		out.Binding.Bands[key] = band
	}
	if in.Binding.HighPass != nil {
		value := *in.Binding.HighPass
		if value.Allocated != nil {
			allocated := *value.Allocated
			value.Allocated = &allocated
		}
		if value.ResponseShape != nil {
			shape := *value.ResponseShape
			shape.Values = cloneStringFloatMap(shape.Values)
			value.ResponseShape = &shape
		}
		value.SlopeDBPerOctave.Values = cloneStringFloatMap(value.SlopeDBPerOctave.Values)
		out.Binding.HighPass = &value
	}
	if in.Binding.LowPass != nil {
		value := *in.Binding.LowPass
		if value.Allocated != nil {
			allocated := *value.Allocated
			value.Allocated = &allocated
		}
		if value.ResponseShape != nil {
			shape := *value.ResponseShape
			shape.Values = cloneStringFloatMap(shape.Values)
			value.ResponseShape = &shape
		}
		value.SlopeDBPerOctave.Values = cloneStringFloatMap(value.SlopeDBPerOctave.Values)
		out.Binding.LowPass = &value
	}
	if in.Binding.Output != nil {
		value := *in.Binding.Output
		out.Binding.Output = &value
	}
	return out
}

func cloneStringFloatMap(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
