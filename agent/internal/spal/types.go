// Package spal implements Vit's Semantic Parameter Application Layer.
//
// SPAL is deliberately a finite, vendor-neutral control language. Capability
// code selects a registered semantic control and its values; adapters resolve
// that instruction to a verified tool instance and physical parameters.
// Adapters never expose vendor parameter IDs to capability code.
package spal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	SchemaVersion       = "vit.spal.v0"
	StaticBellControlID = "spectral.static_bell.v1"
	// EQ v2 models a complete equalizer as a work-card Provider with a
	// capability matrix.  Bell/low-shelf/high-shelf are response-shape values
	// of an allocated Band, never separate Band identities.
	EQBandPatchControlID        = "eq.band.patch.v2"
	EQPassFilterPatchControlID  = "eq.pass_filter.patch.v2"
	EQDynamicBandPatchControlID = "eq.dynamic_band.patch.v2"
	EQOutputPatchControlID      = "eq.output.patch.v2"

	ResolutionBound                = "bound"
	ResolutionRequiresProvisioning = "requires_provisioning"
	ResolutionNoVerifiedProvider   = "no_verified_provider"

	ProviderCandidate = "candidate"
	ProviderVerified  = "verified"
	InstanceVerified  = "verified"
)

// ParameterSpec belongs to the stable semantic language, not to an individual
// plug-in. Provider-specific valid ranges are included in an offer/binding.
type ParameterSpec struct {
	ID       string `json:"id"`
	Unit     string `json:"unit,omitempty"`
	Kind     string `json:"kind,omitempty"` // number | string | boolean
	Required bool   `json:"required"`
}

type ControlSchema struct {
	ID          string          `json:"id"`
	Version     string          `json:"version"`
	Description string          `json:"description,omitempty"`
	Parameters  []ParameterSpec `json:"parameters"`
}

// StaticBellSchema is the only canonical processor control exposed by SPAL v0.
func StaticBellSchema() ControlSchema {
	return ControlSchema{
		ID:          StaticBellControlID,
		Version:     "v1",
		Description: "Static bell spectral gain control with frequency, gain and Q.",
		Parameters: []ParameterSpec{
			{ID: "center_frequency_hz", Unit: "Hz", Required: true},
			{ID: "gain_db", Unit: "dB", Required: true},
			{ID: "q", Unit: "Q", Required: true},
		},
	}
}

func EQBandPatchSchema() ControlSchema {
	return ControlSchema{ID: EQBandPatchControlID, Version: "v2", Description: "Patch one configured EQ Band slot and its response shape.", Parameters: []ParameterSpec{
		{ID: "band_ref", Kind: "string", Required: true},
		{ID: "enabled", Kind: "boolean"},
		{ID: "response_shape", Kind: "string", Required: true},
		{ID: "frequency_hz", Unit: "Hz", Kind: "number", Required: true},
		{ID: "gain_db", Unit: "dB", Kind: "number", Required: true},
		{ID: "q", Unit: "Q", Kind: "number", Required: true},
	}}
}

func EQPassFilterPatchSchema() ControlSchema {
	return ControlSchema{ID: EQPassFilterPatchControlID, Version: "v2", Description: "Patch an independently addressed high-pass or low-pass filter.", Parameters: []ParameterSpec{
		{ID: "filter_kind", Kind: "string", Required: true},
		{ID: "enabled", Kind: "boolean", Required: true},
		{ID: "cutoff_frequency_hz", Unit: "Hz", Kind: "number", Required: true},
		{ID: "slope_db_per_octave", Unit: "dB/oct", Kind: "number", Required: true},
	}}
}

func EQDynamicBandPatchSchema() ControlSchema {
	return ControlSchema{ID: EQDynamicBandPatchControlID, Version: "v2", Description: "Patch the generic, independently routed dynamic mode of one EQ Band.", Parameters: []ParameterSpec{
		{ID: "band_ref", Kind: "string", Required: true},
		{ID: "dynamics_mode", Kind: "string", Required: true},
		{ID: "routing_scope", Kind: "string", Required: true},
		{ID: "threshold_db", Unit: "dB", Kind: "number", Required: true},
		{ID: "ratio", Unit: ":1", Kind: "number"},
		{ID: "attack_ms", Unit: "ms", Kind: "number"},
		{ID: "release_ms", Unit: "ms", Kind: "number"},
	}}
}

func EQOutputPatchSchema() ControlSchema {
	return ControlSchema{ID: EQOutputPatchControlID, Version: "v2", Description: "Patch conformed plug-in bypass, dry/wet mix, or output gain controls.", Parameters: []ParameterSpec{
		{ID: "bypass", Kind: "boolean"},
		{ID: "dry_mix_percent", Unit: "%", Kind: "number"},
		{ID: "output_gain_db", Unit: "dB", Kind: "number"},
	}}
}

func Schema(id string) (ControlSchema, bool) {
	switch strings.TrimSpace(id) {
	case StaticBellControlID:
		return StaticBellSchema(), true
	case EQBandPatchControlID:
		return EQBandPatchSchema(), true
	case EQPassFilterPatchControlID:
		return EQPassFilterPatchSchema(), true
	case EQDynamicBandPatchControlID:
		return EQDynamicBandPatchSchema(), true
	case EQOutputPatchControlID:
		return EQOutputPatchSchema(), true
	default:
		return ControlSchema{}, false
	}
}

type SafetyBounds struct {
	// MaxAbsoluteGainDB constrains a capability instruction without encoding a
	// plug-in-specific gain range. Zero/nil means the Provider range is used.
	MaxAbsoluteGainDB *float64 `json:"max_absolute_gain_db,omitempty"`
}

type SignalExpectation struct {
	BandLowHz  float64 `json:"band_low_hz,omitempty"`
	BandHighHz float64 `json:"band_high_hz,omitempty"`
	Direction  string  `json:"direction,omitempty"` // increase | decrease
}

func (s SignalExpectation) IsRequested() bool {
	return s.BandLowHz != 0 || s.BandHighHz != 0 || strings.TrimSpace(s.Direction) != ""
}

func (s SignalExpectation) Validate() error {
	if !s.IsRequested() {
		return nil
	}
	if !finite(s.BandLowHz) || !finite(s.BandHighHz) || s.BandLowHz <= 0 || s.BandHighHz <= s.BandLowHz {
		return fmt.Errorf("signal expectation requires finite band_low_hz < band_high_hz")
	}
	switch strings.ToLower(strings.TrimSpace(s.Direction)) {
	case "increase", "decrease":
		return nil
	default:
		return fmt.Errorf("signal expectation direction must be increase or decrease")
	}
}

// SignalProbeScope freezes the non-plugin measurement context for an optional
// same-tap signal-direction check. It belongs to verification, not to the
// provider adapter: capability code may identify the musical region to check,
// while SPAL chooses and drives the actual observation tool.
//
// v0 deliberately accepts only a bounded track-post-fader offline render. A
// configured scope must identify either a clip or an explicit time range; it
// must never silently fall back to whichever clip happens to be first on a
// track.
type SignalProbeScope struct {
	TapPoint     string   `json:"tap_point,omitempty"`   // track_post_fader
	RenderMode   string   `json:"render_mode,omitempty"` // offline_probe
	ClipID       string   `json:"clip_id,omitempty"`
	StartSeconds *float64 `json:"start_seconds,omitempty"`
	EndSeconds   *float64 `json:"end_seconds,omitempty"`
	TailSeconds  *float64 `json:"tail_seconds,omitempty"`
}

func (s SignalProbeScope) IsConfigured() bool {
	return strings.TrimSpace(s.TapPoint) != "" ||
		strings.TrimSpace(s.RenderMode) != "" ||
		strings.TrimSpace(s.ClipID) != "" ||
		s.StartSeconds != nil || s.EndSeconds != nil || s.TailSeconds != nil
}

func (s SignalProbeScope) Validate() error {
	if !s.IsConfigured() {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(s.TapPoint), "track_post_fader") {
		return fmt.Errorf("SPAL v0 signal probe scope requires tap_point=track_post_fader")
	}
	if !strings.EqualFold(strings.TrimSpace(s.RenderMode), "offline_probe") {
		return fmt.Errorf("SPAL v0 signal probe scope requires render_mode=offline_probe")
	}
	hasClip := strings.TrimSpace(s.ClipID) != ""
	hasStart := s.StartSeconds != nil
	hasEnd := s.EndSeconds != nil
	if hasClip && (hasStart || hasEnd) {
		return fmt.Errorf("signal probe scope must use either clip_id or an explicit time range, not both")
	}
	if !hasClip {
		if !hasStart || !hasEnd || !finite(*s.StartSeconds) || !finite(*s.EndSeconds) || *s.StartSeconds < 0 || *s.EndSeconds <= *s.StartSeconds {
			return fmt.Errorf("signal probe scope requires a clip_id or finite start_seconds < end_seconds")
		}
	}
	if s.TailSeconds != nil && (!finite(*s.TailSeconds) || *s.TailSeconds < 0 || *s.TailSeconds > 10) {
		return fmt.Errorf("signal probe scope tail_seconds must be finite and within 0..10")
	}
	return nil
}

// Instruction is the only write-level object a capability should construct.
// Parameters use canonical names and units implied by the selected schema.
type Instruction struct {
	SchemaID             string             `json:"schema_id"`
	TargetRef            string             `json:"target_ref"`
	Parameters           map[string]float64 `json:"parameters"`
	StringParameters     map[string]string  `json:"string_parameters,omitempty"`
	ChannelScope         string             `json:"channel_scope,omitempty"`
	SafetyBounds         SafetyBounds       `json:"safety_bounds,omitempty"`
	EvidenceRefs         []string           `json:"evidence_refs,omitempty"`
	ExpectedSignalChange SignalExpectation  `json:"expected_signal_change,omitempty"`
	SignalProbeScope     SignalProbeScope   `json:"signal_probe_scope,omitempty"`
}

type StaticBellValues struct {
	CenterFrequencyHz float64
	GainDB            float64
	Q                 float64
}

func (i Instruction) Validate() error {
	if strings.TrimSpace(i.SchemaID) == "" || strings.TrimSpace(i.TargetRef) == "" {
		return fmt.Errorf("schema_id and target_ref are required")
	}
	if _, ok := Schema(i.SchemaID); !ok {
		return fmt.Errorf("unsupported SPAL schema %q", i.SchemaID)
	}
	if i.SafetyBounds.MaxAbsoluteGainDB != nil {
		if !finite(*i.SafetyBounds.MaxAbsoluteGainDB) || *i.SafetyBounds.MaxAbsoluteGainDB <= 0 {
			return fmt.Errorf("max_absolute_gain_db must be positive and finite")
		}
	}
	if err := i.ExpectedSignalChange.Validate(); err != nil {
		return err
	}
	if !i.ExpectedSignalChange.IsRequested() && i.SignalProbeScope.IsConfigured() {
		return fmt.Errorf("signal_probe_scope requires an expected_signal_change")
	}
	if err := i.SignalProbeScope.Validate(); err != nil {
		return err
	}
	switch strings.TrimSpace(i.SchemaID) {
	case StaticBellControlID:
		_, err := i.StaticBellValues()
		return err
	case EQBandPatchControlID:
		return i.validateEQBandPatch()
	case EQPassFilterPatchControlID:
		return i.validateEQPassFilterPatch()
	case EQDynamicBandPatchControlID:
		return i.validateEQDynamicBandPatch()
	case EQOutputPatchControlID:
		return i.validateEQOutputPatch()
	default:
		return fmt.Errorf("unsupported SPAL schema %q", i.SchemaID)
	}
}

func (i Instruction) requiredNumber(key string) (float64, error) {
	value, ok := i.Parameters[key]
	if !ok || !finite(value) {
		return 0, fmt.Errorf("%s requires finite %s", i.SchemaID, key)
	}
	return value, nil
}

func (i Instruction) requiredString(key string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(i.StringParameters[key]))
	if value == "" {
		return "", fmt.Errorf("%s requires %s", i.SchemaID, key)
	}
	return value, nil
}

func (i Instruction) validateEQBandPatch() error {
	band, err := i.requiredString("band_ref")
	if err != nil || !oneOf(band, "b1", "b2", "b3", "b4") {
		return fmt.Errorf("EQ Band patch requires band_ref b1..b4")
	}
	shape, err := i.requiredString("response_shape")
	if err != nil || !oneOf(shape, "bell", "low_shelf", "high_shelf") {
		return fmt.Errorf("EQ Band patch response_shape must be bell, low_shelf or high_shelf")
	}
	frequency, err := i.requiredNumber("frequency_hz")
	if err != nil || frequency <= 0 {
		return fmt.Errorf("EQ Band patch requires positive frequency_hz")
	}
	gain, err := i.requiredNumber("gain_db")
	if err != nil {
		return err
	}
	q, err := i.requiredNumber("q")
	if err != nil || q <= 0 {
		return fmt.Errorf("EQ Band patch requires positive q")
	}
	if enabled, ok := i.Parameters["enabled"]; ok && enabled != 0 && enabled != 1 {
		return fmt.Errorf("EQ Band patch enabled must be 0 or 1")
	}
	if max := i.SafetyBounds.MaxAbsoluteGainDB; max != nil && math.Abs(gain) > *max {
		return fmt.Errorf("gain_db %.3f exceeds semantic safety bound %.3f", gain, *max)
	}
	return nil
}

func (i Instruction) validateEQPassFilterPatch() error {
	kind, err := i.requiredString("filter_kind")
	if err != nil || !oneOf(kind, "highpass", "lowpass") {
		return fmt.Errorf("EQ pass-filter patch filter_kind must be highpass or lowpass")
	}
	enabled, err := i.requiredNumber("enabled")
	if err != nil || (enabled != 0 && enabled != 1) {
		return fmt.Errorf("EQ pass-filter patch enabled must be 0 or 1")
	}
	frequency, err := i.requiredNumber("cutoff_frequency_hz")
	if err != nil || frequency <= 0 {
		return fmt.Errorf("EQ pass-filter patch requires positive cutoff_frequency_hz")
	}
	slope, err := i.requiredNumber("slope_db_per_octave")
	if err != nil || slope <= 0 {
		return fmt.Errorf("EQ pass-filter patch requires positive slope_db_per_octave")
	}
	return nil
}

func (i Instruction) validateEQDynamicBandPatch() error {
	band, err := i.requiredString("band_ref")
	if err != nil || !oneOf(band, "b1", "b2", "b3", "b4") {
		return fmt.Errorf("dynamic EQ patch requires band_ref b1..b4")
	}
	mode, err := i.requiredString("dynamics_mode")
	if err != nil || !oneOf(mode, "off", "normal") {
		return fmt.Errorf("generic dynamic EQ supports only dynamics_mode off or normal")
	}
	routing, err := i.requiredString("routing_scope")
	if err != nil || routing != "independent" {
		return fmt.Errorf("generic dynamic EQ requires independently conformed routing_scope=independent")
	}
	if _, err := i.requiredNumber("threshold_db"); err != nil {
		return err
	}
	for _, key := range []string{"ratio", "attack_ms", "release_ms"} {
		if value, ok := i.Parameters[key]; ok && (!finite(value) || value <= 0) {
			return fmt.Errorf("dynamic EQ %s must be positive and finite", key)
		}
	}
	return nil
}

func (i Instruction) validateEQOutputPatch() error {
	count := 0
	for _, key := range []string{"bypass", "dry_mix_percent", "output_gain_db"} {
		value, ok := i.Parameters[key]
		if !ok {
			continue
		}
		count++
		if !finite(value) {
			return fmt.Errorf("EQ output %s must be finite", key)
		}
		if key == "bypass" && value != 0 && value != 1 {
			return fmt.Errorf("EQ output bypass must be 0 or 1")
		}
		if key == "dry_mix_percent" && (value < 0 || value > 100) {
			return fmt.Errorf("EQ output dry_mix_percent must be within 0..100")
		}
	}
	if count == 0 {
		return fmt.Errorf("EQ output patch requires at least one output parameter")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func (i Instruction) StaticBellValues() (StaticBellValues, error) {
	if strings.TrimSpace(i.SchemaID) != StaticBellControlID {
		return StaticBellValues{}, fmt.Errorf("instruction schema %q is not %s", i.SchemaID, StaticBellControlID)
	}
	value := func(key string) (float64, error) {
		v, ok := i.Parameters[key]
		if !ok || !finite(v) {
			return 0, fmt.Errorf("static bell requires finite %s", key)
		}
		return v, nil
	}
	frequency, err := value("center_frequency_hz")
	if err != nil {
		return StaticBellValues{}, err
	}
	gain, err := value("gain_db")
	if err != nil {
		return StaticBellValues{}, err
	}
	q, err := value("q")
	if err != nil {
		return StaticBellValues{}, err
	}
	if frequency <= 0 || q <= 0 {
		return StaticBellValues{}, fmt.Errorf("static bell frequency and q must be positive")
	}
	if max := i.SafetyBounds.MaxAbsoluteGainDB; max != nil && math.Abs(gain) > *max {
		return StaticBellValues{}, fmt.Errorf("gain_db %.3f exceeds semantic safety bound %.3f", gain, *max)
	}
	return StaticBellValues{CenterFrequencyHz: frequency, GainDB: gain, Q: q}, nil
}

type ProviderDescriptor struct {
	ID                  string   `json:"id"`
	AdapterVersion      string   `json:"adapter_version"`
	PluginName          string   `json:"plugin_name,omitempty"`
	PluginFormat        string   `json:"plugin_format,omitempty"`
	ProfileSignature    string   `json:"profile_signature,omitempty"`
	Status              string   `json:"status"`
	Experimental        bool     `json:"experimental,omitempty"`
	SupportedSchemas    []string `json:"supported_schemas"`
	ConformanceEvidence []string `json:"conformance_evidence,omitempty"`
}

func (d ProviderDescriptor) Valid() error {
	if strings.TrimSpace(d.ID) == "" || strings.TrimSpace(d.AdapterVersion) == "" {
		return fmt.Errorf("provider id and adapter version are required")
	}
	if d.Status != ProviderCandidate && d.Status != ProviderVerified {
		return fmt.Errorf("provider %s has unknown status %q", d.ID, d.Status)
	}
	if len(d.SupportedSchemas) == 0 {
		return fmt.Errorf("provider %s supports no schemas", d.ID)
	}
	return nil
}

type ProviderInstance struct {
	ID              string            `json:"id"`
	ProviderID      string            `json:"provider_id"`
	TargetRef       string            `json:"target_ref"`
	TrackID         string            `json:"track_id"`
	PluginID        string            `json:"plugin_id"`
	PluginSignature string            `json:"plugin_signature,omitempty"`
	Status          string            `json:"status"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

func (i ProviderInstance) Valid() error {
	if strings.TrimSpace(i.ID) == "" || strings.TrimSpace(i.ProviderID) == "" || strings.TrimSpace(i.TargetRef) == "" || strings.TrimSpace(i.TrackID) == "" || strings.TrimSpace(i.PluginID) == "" {
		return fmt.Errorf("provider instance id, provider_id, target_ref, track_id and plugin_id are required")
	}
	if strings.TrimSpace(i.Status) == "" {
		return fmt.Errorf("provider instance %s status is required", i.ID)
	}
	return nil
}

type ParameterBinding struct {
	ParameterID string  `json:"parameter_id"`
	Unit        string  `json:"unit,omitempty"`
	Min         float64 `json:"min,omitempty"`
	Max         float64 `json:"max,omitempty"`
	// Scale describes how a user-facing semantic value is translated into the
	// host's normalized 0..1 parameter domain. It is part of a learned Provider
	// binding, never a capability-layer concern.
	Scale string `json:"scale,omitempty"`
}

type RuntimeBinding struct {
	ID                string                      `json:"id"`
	SchemaID          string                      `json:"schema_id"`
	Provider          ProviderDescriptor          `json:"provider"`
	Instance          ProviderInstance            `json:"instance"`
	ParameterBindings map[string]ParameterBinding `json:"parameter_bindings"`
	Invariants        map[string]string           `json:"invariants,omitempty"`
}

func (b RuntimeBinding) Valid() error {
	if strings.TrimSpace(b.ID) == "" || strings.TrimSpace(b.SchemaID) == "" {
		return fmt.Errorf("binding id and schema_id are required")
	}
	if err := b.Provider.Valid(); err != nil {
		return err
	}
	if err := b.Instance.Valid(); err != nil {
		return err
	}
	if b.Provider.ID != b.Instance.ProviderID {
		return fmt.Errorf("binding provider does not match instance provider")
	}
	required := []string{}
	switch b.SchemaID {
	case StaticBellControlID:
		required = []string{"center_frequency_hz", "gain_db", "q"}
	case EQBandPatchControlID:
		required = []string{"enabled", "response_shape", "frequency_hz", "gain_db", "q"}
	case EQPassFilterPatchControlID:
		required = []string{"enabled", "cutoff_frequency_hz", "slope_db_per_octave"}
	case EQDynamicBandPatchControlID:
		required = []string{"dynamics_mode", "threshold_db"}
	case EQOutputPatchControlID:
		if len(b.ParameterBindings) == 0 {
			return fmt.Errorf("binding %s has no output parameter bindings", b.ID)
		}
	default:
		return fmt.Errorf("binding %s uses unsupported schema %q", b.ID, b.SchemaID)
	}
	for _, semanticID := range required {
		mapping, ok := b.ParameterBindings[semanticID]
		if !ok || strings.TrimSpace(mapping.ParameterID) == "" {
			return fmt.Errorf("binding %s omits %s", b.ID, semanticID)
		}
	}
	return nil
}

const (
	PhysicalValueModeHostNative = "host_native"
	PhysicalValueModeDisplay    = "display"
	PhysicalValueModeEnumLabel  = "enum_label"
)

type PhysicalParameter struct {
	ParameterID string `json:"parameter_id"`
	// Value is host-native only when ValueMode is empty/host_native. For the
	// retained EQ v2 data-driven compiler, display mode preserves the semantic
	// Hz/dB/Q/toggle value so the C++ plug-in control authority performs the
	// final display-to-normalised conversion required by ADR D5.
	Value float64 `json:"value,omitempty"`
	// ValueMode distinguishes legacy host-native readback/preimage values from
	// semantic display instructions and verified enum labels.
	ValueMode  string `json:"value_mode,omitempty"`
	EnumLabel  string `json:"enum_label,omitempty"`
	BindingRef string `json:"binding_ref,omitempty"`
	Unit       string `json:"unit,omitempty"`
}

func (p PhysicalParameter) Valid() error {
	if strings.TrimSpace(p.ParameterID) == "" {
		return fmt.Errorf("physical parameter id is required")
	}
	switch strings.ToLower(strings.TrimSpace(p.ValueMode)) {
	case "", PhysicalValueModeHostNative:
		if !finite(p.Value) {
			return fmt.Errorf("host-native physical parameter requires a finite value")
		}
	case PhysicalValueModeDisplay:
		if !finite(p.Value) || strings.TrimSpace(p.BindingRef) == "" {
			return fmt.Errorf("display parameter requires a finite semantic value and binding_ref")
		}
	case PhysicalValueModeEnumLabel:
		if strings.TrimSpace(p.EnumLabel) == "" || strings.TrimSpace(p.BindingRef) == "" {
			return fmt.Errorf("enum parameter requires enum_label and binding_ref")
		}
	default:
		return fmt.Errorf("unsupported physical parameter value_mode %q", p.ValueMode)
	}
	return nil
}

func (p PhysicalParameter) RequiresKernelDisplayResolution() bool {
	mode := strings.ToLower(strings.TrimSpace(p.ValueMode))
	return mode == PhysicalValueModeDisplay || mode == PhysicalValueModeEnumLabel
}

type Adapter interface {
	Descriptor() ProviderDescriptor
	Bind(ProviderInstance, Instruction) (RuntimeBinding, error)
	Compile(Instruction, RuntimeBinding) ([]PhysicalParameter, error)
}

type Resolution struct {
	Status              string          `json:"status"`
	Reason              string          `json:"reason,omitempty"`
	Binding             *RuntimeBinding `json:"binding,omitempty"`
	ProviderRequired    bool            `json:"provider_required,omitempty"`
	RequiredPluginSkill bool            `json:"required_plugin_skill,omitempty"`
}

type ResolveOptions struct {
	// Provisioning is never a fallback. It is permitted only when the caller
	// explicitly chooses a verified Provider ID for a separate confirmed plan.
	AllowProvisioning   bool
	RequestedProviderID string
}

type ControlOffer struct {
	SchemaID   string          `json:"schema_id"`
	TargetRef  string          `json:"target_ref"`
	Status     string          `json:"status"`
	Reason     string          `json:"reason,omitempty"`
	Parameters []ParameterSpec `json:"parameters,omitempty"`
}

func fingerprint(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
