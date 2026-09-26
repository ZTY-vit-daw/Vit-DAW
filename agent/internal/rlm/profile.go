package rlm

// Delivery profile parameterization v1 (card L2-1-RLM-1, roadmap D9).
// Scope is pinned to the data-structure + selection/validation layer: no
// metering, supervision, or render-chain wiring (later cards).
//
// Compatibility contract (AGENTS.md §11):
//   - absent profile field on any record = current behavior, unchanged;
//   - unknown enum values and unknown schema versions fail closed on parse.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	DeliveryProfileSchemaVersion = "rlm.delivery_profile.v0"
	RenderBindingSchemaVersion   = "rlm.render_profile_binding.v0"
)

// Channel configuration for loudness measurement (ITU-R BS.1770 K-weighting
// applies per channel). Unknown values fail closed on parse/validation.
const (
	ChannelMono       = "mono"
	ChannelStereo     = "stereo"
	ChannelSurround51 = "surround_5_1"
)

// HeadroomStrategy is the special-rule extension slot v1: how headroom is
// defined for the deliverable. Unknown values fail closed.
const (
	// HeadroomTruePeakBound: headroom is implied by the true-peak ceiling.
	HeadroomTruePeakBound = "true_peak_bound"
)

// LoudnessBand is a target band, not a one-sided threshold. Max is required
// (the band ceiling); Min is nil when the cited standard sets no floor
// (e.g. platform rules of the "do not exceed -16 LUFS" shape).
type LoudnessBand struct {
	Min *float64 `json:"min_lufs,omitempty"`
	Max float64  `json:"max_lufs"`
}

// DeliveryProfile is the loudness parameter vector for one deliverable.
// ShortTermMax/MomentaryMax are nil when the cited standard sets no
// programme-level ceiling for that window; the fields exist so future
// standards (and render-side supervision) can populate them.
type DeliveryProfile struct {
	SchemaVersion        string       `json:"schema_version"`
	ProfileID            string       `json:"profile_id"`
	DisplayName          string       `json:"display_name,omitempty"`
	ChannelConfiguration string       `json:"channel_configuration"`
	Integrated           LoudnessBand `json:"integrated_lufs"`
	ShortTermMax         *float64     `json:"short_term_max_lufs,omitempty"`
	MomentaryMax         *float64     `json:"momentary_max_lufs,omitempty"`
	TruePeakMaxDBTP      float64      `json:"true_peak_max_dbtp"`
	HeadroomStrategy     string       `json:"headroom_strategy,omitempty"`
	Source               string       `json:"source,omitempty"`
}

var builtinDeliveryProfiles = []DeliveryProfile{
	{
		SchemaVersion:        DeliveryProfileSchemaVersion,
		ProfileID:            "builtin:ebu_r128",
		DisplayName:          "EBU R128 (broadcast)",
		ChannelConfiguration: ChannelStereo,
		Integrated:           LoudnessBand{Min: bandPtr(-23.5), Max: -22.5},
		TruePeakMaxDBTP:      -1,
		HeadroomStrategy:     HeadroomTruePeakBound,
		Source:               "EBU R128 Loudness normalisation: target programme loudness -23 LUFS, tolerance +/-0.5 LU; maximum true peak -1 dBTP (measured per ITU-R BS.1770).",
	},
	{
		SchemaVersion:        DeliveryProfileSchemaVersion,
		ProfileID:            "builtin:aes_td1004",
		DisplayName:          "AES streaming (TD1004)",
		ChannelConfiguration: ChannelStereo,
		Integrated:           LoudnessBand{Min: bandPtr(-20), Max: -16},
		TruePeakMaxDBTP:      -1,
		HeadroomStrategy:     HeadroomTruePeakBound,
		Source:               "AES TD1004 Recommendations for Loudness of Audio Streaming and Network File Playback: programme loudness -16 to -20 LUFS; maximum true peak not above -1 dBTP.",
	},
	{
		SchemaVersion:        DeliveryProfileSchemaVersion,
		ProfileID:            "builtin:apple_music",
		DisplayName:          "Apple Music",
		ChannelConfiguration: ChannelStereo,
		Integrated:           LoudnessBand{Min: nil, Max: -16},
		TruePeakMaxDBTP:      -1,
		HeadroomStrategy:     HeadroomTruePeakBound,
		Source:               "Apple Digital Masters / Apple Music Sound Check guidance: overall loudness at or below -16 LUFS; true peak at or below -1 dBTP (one-sided cap, no floor).",
	},
	{
		SchemaVersion:        DeliveryProfileSchemaVersion,
		ProfileID:            "builtin:spotify",
		DisplayName:          "Spotify",
		ChannelConfiguration: ChannelStereo,
		Integrated:           LoudnessBand{Min: nil, Max: -14},
		TruePeakMaxDBTP:      -1,
		HeadroomStrategy:     HeadroomTruePeakBound,
		Source:               "Spotify for Artists loudness normalisation: playback target -14 LUFS; masters recommended at or below -1 dBTP true peak (one-sided cap, no floor).",
	},
	{
		SchemaVersion:        DeliveryProfileSchemaVersion,
		ProfileID:            "builtin:gy_282_2014",
		DisplayName:          "GY/T 282-2014 (China broadcast)",
		ChannelConfiguration: ChannelStereo,
		Integrated:           LoudnessBand{Min: bandPtr(-26), Max: -22},
		TruePeakMaxDBTP:      -2,
		HeadroomStrategy:     HeadroomTruePeakBound,
		Source:               "GY/T 282-2014 数字电视节目平均响度和真峰值音频电平技术要求: average programme loudness -24 LKFS, tolerance +/-2 LU; maximum true peak -2 dBTP (aligned with ITU-R BS.1864).",
	},
}

// BuiltinDeliveryProfiles returns the built-in profile set (values per the
// public standards cited in each Source field).
func BuiltinDeliveryProfiles() []DeliveryProfile {
	out := make([]DeliveryProfile, len(builtinDeliveryProfiles))
	copy(out, builtinDeliveryProfiles)
	return out
}

// LookupDeliveryProfile resolves a built-in profile by id.
func LookupDeliveryProfile(profileID string) (DeliveryProfile, bool) {
	profileID = strings.TrimSpace(profileID)
	for _, profile := range builtinDeliveryProfiles {
		if profile.ProfileID == profileID {
			return profile, true
		}
	}
	return DeliveryProfile{}, false
}

// ValidateDeliveryProfile checks the parameter vector's internal consistency:
// band ordering, cross-window ordering, true-peak sanity, and enum membership.
func ValidateDeliveryProfile(profile DeliveryProfile) error {
	if profile.SchemaVersion != DeliveryProfileSchemaVersion {
		return fmt.Errorf("delivery profile %q: schema version %q is not %q", profile.ProfileID, profile.SchemaVersion, DeliveryProfileSchemaVersion)
	}
	if strings.TrimSpace(profile.ProfileID) == "" {
		return errors.New("delivery profile: profile_id is empty")
	}
	switch profile.ChannelConfiguration {
	case ChannelMono, ChannelStereo, ChannelSurround51:
	default:
		return fmt.Errorf("delivery profile %q: unknown channel configuration %q", profile.ProfileID, profile.ChannelConfiguration)
	}
	switch profile.HeadroomStrategy {
	case "", HeadroomTruePeakBound:
	default:
		return fmt.Errorf("delivery profile %q: unknown headroom strategy %q", profile.ProfileID, profile.HeadroomStrategy)
	}
	if !finite(profile.Integrated.Max) {
		return fmt.Errorf("delivery profile %q: integrated band max is not finite", profile.ProfileID)
	}
	if profile.Integrated.Min != nil {
		if !finite(*profile.Integrated.Min) {
			return fmt.Errorf("delivery profile %q: integrated band min is not finite", profile.ProfileID)
		}
		if *profile.Integrated.Min > profile.Integrated.Max {
			return fmt.Errorf("delivery profile %q: integrated band min %v above max %v", profile.ProfileID, *profile.Integrated.Min, profile.Integrated.Max)
		}
	}
	// A 3s-window average can never exceed the max 400ms-window average, so a
	// profile claiming short-term max above momentary max is inconsistent.
	if profile.ShortTermMax != nil && profile.MomentaryMax != nil {
		if !finite(*profile.ShortTermMax) {
			return fmt.Errorf("delivery profile %q: short-term max is not finite", profile.ProfileID)
		}
		if !finite(*profile.MomentaryMax) {
			return fmt.Errorf("delivery profile %q: momentary max is not finite", profile.ProfileID)
		}
		if *profile.ShortTermMax > *profile.MomentaryMax {
			return fmt.Errorf("delivery profile %q: short-term max %v above momentary max %v", profile.ProfileID, *profile.ShortTermMax, *profile.MomentaryMax)
		}
	}
	if profile.ShortTermMax != nil && !finite(*profile.ShortTermMax) {
		return fmt.Errorf("delivery profile %q: short-term max is not finite", profile.ProfileID)
	}
	if profile.MomentaryMax != nil && !finite(*profile.MomentaryMax) {
		return fmt.Errorf("delivery profile %q: momentary max is not finite", profile.ProfileID)
	}
	if !finite(profile.TruePeakMaxDBTP) {
		return fmt.Errorf("delivery profile %q: true peak max is not finite", profile.ProfileID)
	}
	if profile.TruePeakMaxDBTP > 0 {
		return fmt.Errorf("delivery profile %q: true peak max %v is above full scale", profile.ProfileID, profile.TruePeakMaxDBTP)
	}
	return nil
}

// MarshalDeliveryProfile serializes a profile. The profile must carry the
// current schema version and must validate.
func MarshalDeliveryProfile(profile DeliveryProfile) ([]byte, error) {
	if err := ValidateDeliveryProfile(profile); err != nil {
		return nil, err
	}
	return json.Marshal(profile)
}

// ParseDeliveryProfile deserializes a profile record. Unknown schema versions
// and unknown enum values fail closed; the profile must also validate.
func ParseDeliveryProfile(data []byte) (DeliveryProfile, error) {
	profile := DeliveryProfile{}
	if err := json.Unmarshal(data, &profile); err != nil {
		return DeliveryProfile{}, fmt.Errorf("parse delivery profile: %w", err)
	}
	if profile.SchemaVersion != DeliveryProfileSchemaVersion {
		return DeliveryProfile{}, fmt.Errorf("parse delivery profile: schema version %q is not %q (fail closed)", profile.SchemaVersion, DeliveryProfileSchemaVersion)
	}
	if err := ValidateDeliveryProfile(profile); err != nil {
		return DeliveryProfile{}, fmt.Errorf("parse delivery profile: %w", err)
	}
	return profile, nil
}

// RenderProfileBinding binds exactly one render to exactly one delivery
// profile. Multiple deliverables are multiple renders (branch management is
// out of scope for v1). v1 resolves builtin profile ids only.
type RenderProfileBinding struct {
	SchemaVersion string `json:"schema_version"`
	RenderID      string `json:"render_id"`
	ProfileID     string `json:"profile_id"`
	BoundAt       string `json:"bound_at,omitempty"`
}

// NewRenderProfileBinding creates a binding after checking that the render id
// is present and the profile id resolves.
func NewRenderProfileBinding(renderID, profileID, boundAt string) (RenderProfileBinding, error) {
	if strings.TrimSpace(renderID) == "" {
		return RenderProfileBinding{}, errors.New("render profile binding: render_id is empty")
	}
	profileID = strings.TrimSpace(profileID)
	if _, ok := LookupDeliveryProfile(profileID); !ok {
		return RenderProfileBinding{}, fmt.Errorf("render profile binding: unknown delivery profile %q (fail closed)", profileID)
	}
	return RenderProfileBinding{
		SchemaVersion: RenderBindingSchemaVersion,
		RenderID:      strings.TrimSpace(renderID),
		ProfileID:     profileID,
		BoundAt:       boundAt,
	}, nil
}

// ResolveProfile returns the bound delivery profile.
func (b RenderProfileBinding) ResolveProfile() (DeliveryProfile, error) {
	profile, ok := LookupDeliveryProfile(b.ProfileID)
	if !ok {
		return DeliveryProfile{}, fmt.Errorf("render %q bound to unknown delivery profile %q (fail closed)", b.RenderID, b.ProfileID)
	}
	return profile, nil
}

// MarshalRenderProfileBinding serializes a binding.
func MarshalRenderProfileBinding(binding RenderProfileBinding) ([]byte, error) {
	if binding.SchemaVersion != RenderBindingSchemaVersion {
		return nil, fmt.Errorf("marshal render profile binding: schema version %q is not %q", binding.SchemaVersion, RenderBindingSchemaVersion)
	}
	return json.Marshal(binding)
}

// ParseRenderProfileBinding deserializes a binding record. Unknown schema
// versions, missing render ids, and unresolvable profiles fail closed.
func ParseRenderProfileBinding(data []byte) (RenderProfileBinding, error) {
	binding := RenderProfileBinding{}
	if err := json.Unmarshal(data, &binding); err != nil {
		return RenderProfileBinding{}, fmt.Errorf("parse render profile binding: %w", err)
	}
	if binding.SchemaVersion != RenderBindingSchemaVersion {
		return RenderProfileBinding{}, fmt.Errorf("parse render profile binding: schema version %q is not %q (fail closed)", binding.SchemaVersion, RenderBindingSchemaVersion)
	}
	if strings.TrimSpace(binding.RenderID) == "" {
		return RenderProfileBinding{}, errors.New("parse render profile binding: render_id is empty")
	}
	if _, ok := LookupDeliveryProfile(binding.ProfileID); !ok {
		return RenderProfileBinding{}, fmt.Errorf("parse render profile binding: unknown delivery profile %q (fail closed)", binding.ProfileID)
	}
	return binding, nil
}

// RenderBindingIndex enforces one render -> one profile over a set of
// bindings. Rebinding the same render to the same profile is idempotent;
// rebinding to a different profile is rejected. Not safe for concurrent use.
type RenderBindingIndex struct {
	byRender map[string]string
}

func NewRenderBindingIndex() *RenderBindingIndex {
	return &RenderBindingIndex{byRender: map[string]string{}}
}

func (idx *RenderBindingIndex) Bind(binding RenderProfileBinding) error {
	if idx.byRender == nil {
		idx.byRender = map[string]string{}
	}
	if existing, ok := idx.byRender[binding.RenderID]; ok && existing != binding.ProfileID {
		return fmt.Errorf("render %q is already bound to profile %q; one render one profile (rebind requires a new render/branch)", binding.RenderID, existing)
	}
	idx.byRender[binding.RenderID] = binding.ProfileID
	return nil
}

func (idx *RenderBindingIndex) ProfileForRender(renderID string) (string, bool) {
	profileID, ok := idx.byRender[renderID]
	return profileID, ok
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func bandPtr(value float64) *float64 {
	return &value
}

// sha256Hex pins test fixture content (see profile_test.go provenance chain).
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
