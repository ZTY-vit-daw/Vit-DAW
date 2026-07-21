package mixstyle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const (
	SchemaVersion = "vit.mix_style.v1"
	Kind          = "mix_style"
	FileExtension = ".vms"
	MIMEType      = "application/vnd.vit.mix-style+json"

	StaticBalanceSchemaVersion = "vit.mix_style.static_balance.v1"
	PanLayoutSchemaVersion     = "vit.mix_style.pan_layout.v1"
)

type Identity struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type Compatibility struct {
	MinVitVersion       string            `json:"min_vit_version,omitempty"`
	CapabilityContracts map[string]string `json:"capability_contracts,omitempty"`
}

type StyleIntent struct {
	Density      string `json:"density,omitempty"`
	Contrast     string `json:"contrast,omitempty"`
	CenterFocus  string `json:"center_focus,omitempty"`
	Symmetry     string `json:"symmetry,omitempty"`
	Conservatism string `json:"conservatism,omitempty"`
}

// Dimensions is the stable B2 static-balance parameter registry. Values use
// [-1,1], where zero is the neutral solver prior.
type Dimensions struct {
	ForegroundPriority      float64 `json:"foreground_priority"`
	RhythmAnchorWeight      float64 `json:"rhythm_anchor_weight"`
	LowEndAnchorWeight      float64 `json:"low_end_anchor_weight"`
	HarmonicBedDepth        float64 `json:"harmonic_bed_depth"`
	SupportLayerDistance    float64 `json:"support_layer_distance"`
	TransientControlBias    float64 `json:"transient_control_bias"`
	DensityClearance        float64 `json:"density_clearance"`
	NaturalDynamicTolerance float64 `json:"natural_dynamic_tolerance"`
}

type StaticBalanceCapability struct {
	SchemaVersion string `json:"schema_version"`
	Dimensions
}

// PanLayoutDimensions is the B3 parameter registry. Amount/safety fields use
// [0,1]; SymmetryBias uses [-1,1], with zero meaning no left/right preference.
type PanLayoutDimensions struct {
	CenterDensity             float64 `json:"center_density"`
	SpreadAmount              float64 `json:"spread_amount"`
	SymmetryBias              float64 `json:"symmetry_bias"`
	ComplementaryPairBias     float64 `json:"complementary_pair_bias"`
	MonoSafety                float64 `json:"mono_safety"`
	StereoSourceConservatism  float64 `json:"stereo_source_conservatism"`
	SupportLayerSpread        float64 `json:"support_layer_spread"`
	RhythmPeripheralAllowance float64 `json:"rhythm_peripheral_allowance"`
}

type PanLayoutCapability struct {
	SchemaVersion string `json:"schema_version"`
	PanLayoutDimensions
}

type Capabilities struct {
	StaticBalance StaticBalanceCapability `json:"static_balance"`
	PanLayout     PanLayoutCapability     `json:"pan_layout"`
}

type MixStyle struct {
	SchemaVersion string        `json:"schema_version"`
	Kind          string        `json:"kind"`
	Identity      Identity      `json:"identity"`
	Compatibility Compatibility `json:"compatibility,omitempty"`
	StyleIntent   StyleIntent   `json:"style_intent,omitempty"`
	Capabilities  Capabilities  `json:"capabilities"`

	// Compatibility accessors for the existing B2 solver. They are populated
	// by Normalize and are deliberately not serialized into canonical .vms.
	ID          string     `json:"-"`
	Name        string     `json:"-"`
	Description string     `json:"-"`
	Dimensions  Dimensions `json:"-"`
}

func Normalize(style MixStyle) MixStyle {
	style.ID = strings.TrimSpace(style.Identity.ID)
	style.Name = strings.TrimSpace(style.Identity.Name)
	style.Description = strings.TrimSpace(style.Identity.Description)
	style.Dimensions = style.Capabilities.StaticBalance.Dimensions
	return style
}

func Validate(style MixStyle) error {
	style = Normalize(style)
	if strings.TrimSpace(style.SchemaVersion) != SchemaVersion {
		return fmt.Errorf("unsupported mix style schema %q", style.SchemaVersion)
	}
	if strings.TrimSpace(style.Kind) != Kind {
		return fmt.Errorf("unsupported mix style kind %q", style.Kind)
	}
	if style.ID == "" || style.Name == "" || strings.TrimSpace(style.Identity.Version) == "" {
		return fmt.Errorf("mix style identity id, name, and version are required")
	}
	if style.Capabilities.StaticBalance.SchemaVersion != StaticBalanceSchemaVersion {
		return fmt.Errorf("unsupported static_balance style schema %q", style.Capabilities.StaticBalance.SchemaVersion)
	}
	if style.Capabilities.PanLayout.SchemaVersion != PanLayoutSchemaVersion {
		return fmt.Errorf("unsupported pan_layout style schema %q", style.Capabilities.PanLayout.SchemaVersion)
	}
	for name, value := range staticBalanceValues(style.Dimensions) {
		if !finite(value) || value < -1 || value > 1 {
			return fmt.Errorf("mix style static_balance dimension %s=%g is outside [-1,1]", name, value)
		}
	}
	for name, value := range panAmountValues(style.Capabilities.PanLayout.PanLayoutDimensions) {
		if !finite(value) || value < 0 || value > 1 {
			return fmt.Errorf("mix style pan_layout dimension %s=%g is outside [0,1]", name, value)
		}
	}
	if value := style.Capabilities.PanLayout.SymmetryBias; !finite(value) || value < -1 || value > 1 {
		return fmt.Errorf("mix style pan_layout dimension symmetry_bias=%g is outside [-1,1]", value)
	}
	return nil
}

func Hash(style MixStyle) string {
	style = Normalize(style)
	data, _ := json.Marshal(style)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:12])
}

func staticBalanceValues(d Dimensions) map[string]float64 {
	return map[string]float64{
		"foreground_priority": d.ForegroundPriority, "rhythm_anchor_weight": d.RhythmAnchorWeight,
		"low_end_anchor_weight": d.LowEndAnchorWeight, "harmonic_bed_depth": d.HarmonicBedDepth,
		"support_layer_distance": d.SupportLayerDistance, "transient_control_bias": d.TransientControlBias,
		"density_clearance": d.DensityClearance, "natural_dynamic_tolerance": d.NaturalDynamicTolerance,
	}
}

func panAmountValues(d PanLayoutDimensions) map[string]float64 {
	return map[string]float64{
		"center_density": d.CenterDensity, "spread_amount": d.SpreadAmount,
		"complementary_pair_bias": d.ComplementaryPairBias, "mono_safety": d.MonoSafety,
		"stereo_source_conservatism":  d.StereoSourceConservatism,
		"support_layer_spread":        d.SupportLayerSpread,
		"rhythm_peripheral_allowance": d.RhythmPeripheralAllowance,
	}
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
