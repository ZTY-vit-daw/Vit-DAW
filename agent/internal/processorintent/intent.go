// Package processorintent defines the model-owned abstract processor intent
// exchanged between observation reasoning and deterministic routing.
package processorintent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const SchemaVersion = "semantic_processor_intent.v1"

const (
	StatusResolved   = "resolved"
	StatusUnresolved = "unresolved"

	ScopeCurrentTrack     = "current_track"
	ScopeCurrentSelection = "current_selection"
	ScopeProject          = "project"
	ScopeTrackGroup       = "track_group"

	ControlModeObserveOnly = "observe_only"
	ControlModeTyped       = "typed_control"
	ControlModeSemantic    = "semantic_loop"
	ControlModeInspectOnly = "inspect_only"
	ControlModeUnresolved  = "unresolved"
)

const (
	FamilyStaticEQ            = "static_eq"
	FamilyBroadbandCompressor = "broadband_compressor"
	FamilyLimiter             = "limiter"
	FamilyGateExpander        = "gate_expander"
	FamilyDeEsser             = "de_esser"
	FamilyTransientShaper     = "transient_shaper"
	FamilyMultibandDynamics   = "multiband_dynamics"
	FamilySpectralDynamics    = "spectral_dynamics"
	FamilyClipper             = "clipper"
)

// Intent is the only model-owned family/coverage handoff. It intentionally
// contains no plugin identity, path, parameter ID, or vendor mapping.
type Intent struct {
	SchemaVersion    string     `json:"schema_version"`
	Status           string     `json:"status"`
	Family           string     `json:"family,omitempty"`
	Intent           string     `json:"intent,omitempty"`
	RequiredCoverage []string   `json:"required_coverage,omitempty"`
	Scope            string     `json:"scope,omitempty"`
	ControlMode      string     `json:"control_mode"`
	Confidence       float64    `json:"confidence"`
	EvidenceRefs     []string   `json:"evidence_refs,omitempty"`
	Rejection        *Rejection `json:"rejection,omitempty"`
}

// UnmarshalJSON keeps the protocol strict even when an Intent is embedded in
// another JSON envelope (for example free_state_decision.v1). The caller still
// explicitly invokes Validate so parsing and policy validation remain separate.
func (i *Intent) UnmarshalJSON(data []byte) error {
	type intentAlias Intent
	var decoded intentAlias
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	*i = Intent(decoded)
	return nil
}

type Rejection struct {
	Code    string         `json:"code"`
	Reason  string         `json:"reason"`
	Details map[string]any `json:"details,omitempty"`
}

var supportedFamilies = map[string]bool{
	FamilyStaticEQ: true, FamilyBroadbandCompressor: true, FamilyLimiter: true,
	FamilyGateExpander: true, FamilyDeEsser: true, FamilyTransientShaper: true,
	FamilyMultibandDynamics: true, FamilySpectralDynamics: true, FamilyClipper: true,
}

var supportedScopes = map[string]bool{
	ScopeCurrentTrack: true, ScopeCurrentSelection: true, ScopeProject: true, ScopeTrackGroup: true,
}

var supportedControlModes = map[string]bool{
	ControlModeObserveOnly: true, ControlModeTyped: true, ControlModeSemantic: true,
	ControlModeInspectOnly: true, ControlModeUnresolved: true,
}

// Decode parses exactly one JSON object and rejects unknown/trailing fields.
// It never supplies defaults for family, coverage, scope, or control mode.
func Decode(raw string) (Intent, error) {
	var intent Intent
	decoder := json.NewDecoder(bytes.NewReader([]byte(strings.TrimSpace(raw))))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&intent); err != nil {
		return intent, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Intent{}, fmt.Errorf("trailing JSON data")
	}
	if err := intent.Validate(); err != nil {
		return Intent{}, err
	}
	return intent, nil
}

func (i Intent) Validate() error {
	if strings.TrimSpace(i.SchemaVersion) != SchemaVersion {
		return fmt.Errorf("schema_version must be %s", SchemaVersion)
	}
	status := strings.ToLower(strings.TrimSpace(i.Status))
	if status != StatusResolved && status != StatusUnresolved {
		return fmt.Errorf("status must be resolved or unresolved")
	}
	if i.Confidence < 0 || i.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}
	if err := validateUniqueStrings("evidence_refs", i.EvidenceRefs); err != nil {
		return err
	}
	if status == StatusUnresolved {
		if strings.TrimSpace(i.Family) != "" || strings.TrimSpace(i.Intent) != "" || len(i.RequiredCoverage) != 0 || strings.TrimSpace(i.Scope) != "" {
			return fmt.Errorf("unresolved intent must not contain family, intent, required_coverage, or scope")
		}
		if strings.ToLower(strings.TrimSpace(i.ControlMode)) != ControlModeUnresolved {
			return fmt.Errorf("unresolved intent requires control_mode=unresolved")
		}
		if i.Rejection == nil {
			return fmt.Errorf("unresolved intent requires structured rejection")
		}
		return i.Rejection.Validate()
	}
	if strings.TrimSpace(i.Family) == "" || !supportedFamilies[strings.ToLower(strings.TrimSpace(i.Family))] {
		return fmt.Errorf("resolved intent requires a supported family")
	}
	if strings.TrimSpace(i.Intent) == "" {
		return fmt.Errorf("resolved intent requires an open intent")
	}
	if !supportedScopes[strings.ToLower(strings.TrimSpace(i.Scope))] {
		return fmt.Errorf("resolved intent requires a supported scope")
	}
	if !supportedControlModes[strings.ToLower(strings.TrimSpace(i.ControlMode))] || strings.ToLower(strings.TrimSpace(i.ControlMode)) == ControlModeUnresolved {
		return fmt.Errorf("resolved intent requires a usable control_mode")
	}
	controlMode := strings.ToLower(strings.TrimSpace(i.ControlMode))
	family := strings.ToLower(strings.TrimSpace(i.Family))
	if controlMode == ControlModeInspectOnly {
		if family != FamilySpectralDynamics && family != FamilyClipper {
			return fmt.Errorf("inspect_only control mode is reserved for spectral_dynamics or clipper")
		}
		if len(i.RequiredCoverage) != 1 || strings.ToLower(strings.TrimSpace(i.RequiredCoverage[0])) != "inspect_only" {
			return fmt.Errorf("inspect_only control mode requires required_coverage=[inspect_only]")
		}
	}
	if len(i.RequiredCoverage) == 0 {
		return fmt.Errorf("resolved intent requires required_coverage")
	}
	if err := validateUniqueStrings("required_coverage", i.RequiredCoverage); err != nil {
		return err
	}
	if i.Rejection != nil {
		return fmt.Errorf("resolved intent must not contain rejection")
	}
	return nil
}

func (r Rejection) Validate() error {
	if strings.TrimSpace(r.Code) == "" || strings.TrimSpace(r.Reason) == "" {
		return fmt.Errorf("rejection requires code and reason")
	}
	return nil
}

func validateUniqueStrings(field string, values []string) error {
	seen := map[string]bool{}
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return fmt.Errorf("%s must contain only non-empty strings", field)
		}
		if seen[value] {
			return fmt.Errorf("%s must not contain duplicates", field)
		}
		seen[value] = true
	}
	return nil
}

func SupportedFamilies() []string {
	out := make([]string, 0, len(supportedFamilies))
	for family := range supportedFamilies {
		out = append(out, family)
	}
	sort.Strings(out)
	return out
}

func IsSupportedFamily(family string) bool {
	return supportedFamilies[strings.ToLower(strings.TrimSpace(family))]
}
