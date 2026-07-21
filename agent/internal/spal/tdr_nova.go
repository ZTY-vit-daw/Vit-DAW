package spal

import (
	"fmt"
	"math"
	"strings"
)

const (
	ExperimentalTDRNovaProviderID = "lab.tdr_nova.vst3.static_bell.v0"
	ExperimentalTDRNovaSignature  = "plugin_f9788adda4203df8:tdr-nova-vst3-2.2.2:static-bell.v0"
	tdrNovaQMin                   = 0.1
	tdrNovaQMax                   = 6.0
)

// TDRNovaAdapter is deliberately an experimental Provider. It is useful for
// a conformance fixture and vertical slice, but Registry.Resolve never picks
// it unless the current project explicitly exposes a verified instance.
type TDRNovaAdapter struct {
	descriptor ProviderDescriptor
	bands      map[string]tdrNovaBand
}

type tdrNovaBand struct {
	EnableParamID    string
	DynEnableParamID string
	FrequencyParamID string
	GainParamID      string
	QParamID         string
}

// TDRNovaConformanceProfile is produced by Plugin Learning after its raw
// profile has been signature-checked. It remains inside SPAL/Plugin Lab; the
// capability layer never sees this vendor-specific information.
type TDRNovaConformanceProfile struct {
	ProfileKey         string
	PluginName         string
	PluginFormat       string
	PluginVersion      string
	ParameterSignature string
	Bands              map[string]TDRNovaBandConformance
}

type TDRNovaBandConformance struct {
	StaticBellReady bool
	Parameters      map[string]TDRNovaParameterConformance
}

type TDRNovaParameterConformance struct {
	ParameterID string
	Unit        string
	Min         float64
	Max         float64
	Confirmed   bool
}

type TDRNovaConformanceReport struct {
	ProviderID   string
	BandSlot     string
	Status       string
	EvidenceRefs []string
}

func NewExperimentalTDRNovaAdapter() (*TDRNovaAdapter, error) {
	adapter := &TDRNovaAdapter{
		descriptor: ProviderDescriptor{
			ID:               ExperimentalTDRNovaProviderID,
			AdapterVersion:   "v0",
			PluginName:       "TDR Nova",
			PluginFormat:     "VST3",
			ProfileSignature: ExperimentalTDRNovaSignature,
			Status:           ProviderVerified,
			Experimental:     true,
			SupportedSchemas: []string{StaticBellControlID},
			ConformanceEvidence: []string{
				"plugin-grabber-profile:plugin_f9788adda4203df8",
				"tdr-nova-static-bell-conformance:v0",
			},
		},
		bands: map[string]tdrNovaBand{
			"band1": {EnableParamID: "1", DynEnableParamID: "6", FrequencyParamID: "4", GainParamID: "2", QParamID: "3"},
			"band2": {EnableParamID: "13", DynEnableParamID: "18", FrequencyParamID: "16", GainParamID: "14", QParamID: "15"},
			"band3": {EnableParamID: "25", DynEnableParamID: "30", FrequencyParamID: "28", GainParamID: "26", QParamID: "27"},
			"band4": {EnableParamID: "37", DynEnableParamID: "42", FrequencyParamID: "40", GainParamID: "38", QParamID: "39"},
		},
	}
	if err := adapter.descriptor.Valid(); err != nil {
		return nil, err
	}
	return adapter, nil
}

func (a *TDRNovaAdapter) Descriptor() ProviderDescriptor {
	if a == nil {
		return ProviderDescriptor{}
	}
	d := a.descriptor
	d.SupportedSchemas = append([]string(nil), a.descriptor.SupportedSchemas...)
	d.ConformanceEvidence = append([]string(nil), a.descriptor.ConformanceEvidence...)
	return d
}

// Conform verifies the small subset of a learned TDR Nova profile that SPAL v0
// is permitted to trust. It deliberately does not promote dynamic, filter or
// vendor-specific features into the canonical semantic language.
func (a *TDRNovaAdapter) Conform(profile TDRNovaConformanceProfile, bandSlot string) (TDRNovaConformanceReport, error) {
	if a == nil {
		return TDRNovaConformanceReport{}, fmt.Errorf("TDR Nova adapter is nil")
	}
	if strings.TrimSpace(profile.ProfileKey) != "plugin_f9788adda4203df8" || !strings.EqualFold(strings.TrimSpace(profile.PluginName), "TDR Nova") || !strings.EqualFold(strings.TrimSpace(profile.PluginFormat), "VST3") {
		return TDRNovaConformanceReport{}, fmt.Errorf("profile identity is not the experimental TDR Nova fixture")
	}
	if strings.TrimSpace(profile.ParameterSignature) != ExperimentalTDRNovaSignature {
		return TDRNovaConformanceReport{}, fmt.Errorf("TDR Nova parameter signature is not conformed")
	}
	bandSlot = strings.ToLower(strings.TrimSpace(bandSlot))
	expected, ok := a.bands[bandSlot]
	if !ok {
		return TDRNovaConformanceReport{}, fmt.Errorf("unknown TDR Nova band slot %q", bandSlot)
	}
	actual, ok := profile.Bands[bandSlot]
	if !ok || !actual.StaticBellReady {
		return TDRNovaConformanceReport{}, fmt.Errorf("TDR Nova band %s has not been verified as a static Bell", bandSlot)
	}
	required := map[string]struct {
		id   string
		min  float64
		max  float64
		unit string
	}{
		"enable":     {expected.EnableParamID, 0, 1, "toggle"},
		"dyn_enable": {expected.DynEnableParamID, 0, 1, "toggle"},
		"frequency":  {expected.FrequencyParamID, 10, 40000, "Hz"},
		"gain":       {expected.GainParamID, -18, 18, "dB"},
		"q":          {expected.QParamID, tdrNovaQMin, tdrNovaQMax, "Q"},
	}
	for role, want := range required {
		got, ok := actual.Parameters[role]
		if !ok || !got.Confirmed || got.ParameterID != want.id || !strings.EqualFold(strings.TrimSpace(got.Unit), want.unit) || got.Min > want.min || got.Max < want.max {
			return TDRNovaConformanceReport{}, fmt.Errorf("TDR Nova %s mapping is not conformed for band %s", role, bandSlot)
		}
	}
	return TDRNovaConformanceReport{
		ProviderID: a.descriptor.ID, BandSlot: bandSlot, Status: ProviderVerified,
		EvidenceRefs: []string{
			"plugin-grabber-profile:" + profile.ProfileKey,
			"tdr-nova-static-bell-conformance:" + bandSlot,
		},
	}, nil
}

func (a *TDRNovaAdapter) Bind(instance ProviderInstance, instruction Instruction) (RuntimeBinding, error) {
	if a == nil {
		return RuntimeBinding{}, fmt.Errorf("TDR Nova adapter is nil")
	}
	if err := instruction.Validate(); err != nil {
		return RuntimeBinding{}, err
	}
	if instruction.SchemaID != StaticBellControlID {
		return RuntimeBinding{}, fmt.Errorf("TDR Nova does not implement %s", instruction.SchemaID)
	}
	if err := instance.Valid(); err != nil {
		return RuntimeBinding{}, err
	}
	if instance.ProviderID != a.descriptor.ID || instance.Status != InstanceVerified {
		return RuntimeBinding{}, fmt.Errorf("TDR Nova instance is not verified for this Provider")
	}
	if signature := strings.TrimSpace(instance.PluginSignature); signature != a.descriptor.ProfileSignature {
		return RuntimeBinding{}, fmt.Errorf("TDR Nova profile signature does not match the experimental conformance fixture")
	}
	bandSlot := strings.ToLower(strings.TrimSpace(instance.Metadata["band_slot"]))
	band, ok := a.bands[bandSlot]
	if !ok {
		return RuntimeBinding{}, fmt.Errorf("TDR Nova instance requires a known band_slot")
	}
	// The plug-in profile has no trustworthy cross-version enum value for Bell.
	// The Plugin Lab must prove this invariant before SPAL is allowed to bind it.
	if !strings.EqualFold(strings.TrimSpace(instance.Metadata["static_bell_ready"]), "true") {
		return RuntimeBinding{}, fmt.Errorf("TDR Nova band %s is not conformed as a static Bell", bandSlot)
	}
	binding := RuntimeBinding{
		ID:       "binding:" + instance.ID + ":" + StaticBellControlID,
		SchemaID: StaticBellControlID,
		Provider: a.Descriptor(),
		Instance: instance,
		ParameterBindings: map[string]ParameterBinding{
			"center_frequency_hz": {ParameterID: band.FrequencyParamID, Unit: "Hz", Min: 10, Max: 40000},
			"gain_db":             {ParameterID: band.GainParamID, Unit: "dB", Min: -18, Max: 18},
			"q":                   {ParameterID: band.QParamID, Unit: "Q", Min: tdrNovaQMin, Max: tdrNovaQMax},
			"_enable":             {ParameterID: band.EnableParamID, Unit: "toggle", Min: 0, Max: 1},
			"_dyn_enable":         {ParameterID: band.DynEnableParamID, Unit: "toggle", Min: 0, Max: 1},
		},
		Invariants: map[string]string{
			"band_slot":         bandSlot,
			"static_bell_ready": "true",
			"dynamic_mode":      "disabled",
		},
	}
	if err := binding.Valid(); err != nil {
		return RuntimeBinding{}, err
	}
	return binding, nil
}

func (a *TDRNovaAdapter) Compile(instruction Instruction, binding RuntimeBinding) ([]PhysicalParameter, error) {
	if a == nil {
		return nil, fmt.Errorf("TDR Nova adapter is nil")
	}
	if err := binding.Valid(); err != nil {
		return nil, err
	}
	if binding.Provider.ID != a.descriptor.ID || binding.SchemaID != StaticBellControlID {
		return nil, fmt.Errorf("binding is not a TDR Nova static bell binding")
	}
	if !strings.EqualFold(binding.Invariants["static_bell_ready"], "true") || !strings.EqualFold(binding.Invariants["dynamic_mode"], "disabled") {
		return nil, fmt.Errorf("TDR Nova binding is missing static Bell invariants")
	}
	values, err := instruction.StaticBellValues()
	if err != nil {
		return nil, err
	}
	for _, check := range []struct {
		key string
		val float64
	}{
		{"center_frequency_hz", values.CenterFrequencyHz},
		{"gain_db", values.GainDB},
		{"q", values.Q},
	} {
		parameter := binding.ParameterBindings[check.key]
		if check.val < parameter.Min || check.val > parameter.Max || math.IsNaN(check.val) || math.IsInf(check.val, 0) {
			return nil, fmt.Errorf("TDR Nova %s %.3f is outside conformed range %.3f..%.3f", check.key, check.val, parameter.Min, parameter.Max)
		}
	}
	compile := func(key string, semanticValue float64) (PhysicalParameter, error) {
		parameter, ok := binding.ParameterBindings[key]
		if !ok {
			return PhysicalParameter{}, fmt.Errorf("TDR Nova binding omits %s", key)
		}
		hostValue, err := tdrNovaHostParameterValue(parameter, semanticValue)
		if err != nil {
			return PhysicalParameter{}, fmt.Errorf("TDR Nova %s host mapping: %w", key, err)
		}
		return PhysicalParameter{ParameterID: parameter.ParameterID, Value: hostValue, Unit: parameter.Unit}, nil
	}
	rows := make([]PhysicalParameter, 0, 5)
	for _, item := range []struct {
		key   string
		value float64
	}{
		{"_enable", 1},
		{"_dyn_enable", 0},
		{"center_frequency_hz", values.CenterFrequencyHz},
		{"gain_db", values.GainDB},
		{"q", values.Q},
	} {
		row, err := compile(item.key, item.value)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// tdrNovaHostParameterValue is deliberately scoped to the signature-pinned
// TDR Nova reference fixture.  The semantic Adapter, rather than the
// capability layer or VSP port, owns this vendor-specific conversion.  Nova's
// Frequency and Q controls are logarithmic in the host's native 0..1 domain;
// Gain and enable controls are linear.
func tdrNovaHostParameterValue(parameter ParameterBinding, semanticValue float64) (float64, error) {
	if !finite(semanticValue) || !finite(parameter.Min) || !finite(parameter.Max) || parameter.Max <= parameter.Min {
		return 0, fmt.Errorf("invalid conformed parameter range %.6f..%.6f", parameter.Min, parameter.Max)
	}
	if semanticValue < parameter.Min || semanticValue > parameter.Max {
		return 0, fmt.Errorf("%.6f is outside %.6f..%.6f", semanticValue, parameter.Min, parameter.Max)
	}
	if semanticValue == parameter.Min {
		return 0, nil
	}
	if semanticValue == parameter.Max {
		return 1, nil
	}
	var normalized float64
	switch strings.ToLower(strings.TrimSpace(parameter.Unit)) {
	case "hz", "q":
		if parameter.Min <= 0 {
			return 0, fmt.Errorf("logarithmic parameter requires a positive minimum")
		}
		normalized = math.Log(semanticValue/parameter.Min) / math.Log(parameter.Max/parameter.Min)
	case "db", "toggle":
		normalized = (semanticValue - parameter.Min) / (parameter.Max - parameter.Min)
	default:
		return 0, fmt.Errorf("unsupported conformed parameter unit %q", parameter.Unit)
	}
	if normalized < 0 {
		return 0, nil
	}
	if normalized > 1 {
		return 1, nil
	}
	return normalized, nil
}
