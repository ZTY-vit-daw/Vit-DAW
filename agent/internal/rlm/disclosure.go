package rlm

// Render binding disclosure (card RLM-PROFILE-2, roadmap D9). This is the RLM
// package's own projection output face: it discloses the current render ->
// delivery-profile bindings together with their assertion semantics, without
// requiring any consumer-side (capabilitycontext) change. Fail-closed: a
// binding whose profile no longer resolves (e.g. a retired built-in in a
// persisted record) is disclosed as unresolvable with the error preserved,
// never silently dropped.

import (
	"fmt"
	"strings"
)

const (
	RenderBindingStatusBound        = "bound"
	RenderBindingStatusUnresolvable = "unresolvable"

	LimitationRenderBindingUnresolvable = "rlm_render_profile_binding_unresolvable"
)

// RenderBindingDisclosure is the per-binding disclosure row carried on the RLM
// projection output.
type RenderBindingDisclosure struct {
	RenderID           string           `json:"render_id"`
	ProfileID          string           `json:"profile_id"`
	Status             string           `json:"status"`
	Profile            *DeliveryProfile `json:"profile,omitempty"`
	AssertionSemantics string           `json:"assertion_semantics,omitempty"`
	BoundAt            string           `json:"bound_at,omitempty"`
	Error              string           `json:"error,omitempty"`
}

// BuildRenderBindingDisclosure resolves each binding against the built-in
// profile set. Unresolvable bindings keep their row with status unresolvable
// and the resolution error; resolvable rows carry the full parameter vector
// and its assertion semantics.
func BuildRenderBindingDisclosure(bindings []RenderProfileBinding) []RenderBindingDisclosure {
	out := make([]RenderBindingDisclosure, 0, len(bindings))
	for _, binding := range bindings {
		row := RenderBindingDisclosure{
			RenderID:  binding.RenderID,
			ProfileID: binding.ProfileID,
			BoundAt:   binding.BoundAt,
			Status:    RenderBindingStatusUnresolvable,
		}
		profile, err := binding.ResolveProfile()
		if err != nil {
			row.Error = err.Error()
			out = append(out, row)
			continue
		}
		row.Status = RenderBindingStatusBound
		profileCopy := profile
		row.Profile = &profileCopy
		row.AssertionSemantics = AssertionSemanticsFor(profile)
		out = append(out, row)
	}
	return out
}

// AssertionSemanticsFor renders the profile's assertion semantics in prose:
// the integrated loudness requirement is a target band (a two-sided band when
// the cited standard sets a floor, a one-sided target-band ceiling when it
// does not), never a one-way pass threshold; plus the true-peak ceiling,
// channel configuration, and any special-rule fields.
func AssertionSemanticsFor(profile DeliveryProfile) string {
	var band string
	if profile.Integrated.Min != nil {
		band = fmt.Sprintf("integrated loudness target band [%+.1f, %+.1f] LUFS (target band, not a one-sided threshold)", *profile.Integrated.Min, profile.Integrated.Max)
	} else {
		band = fmt.Sprintf("integrated loudness target band at or below %+.1f LUFS (one-sided target band: cited standard sets no floor)", profile.Integrated.Max)
	}
	parts := []string{
		band,
		fmt.Sprintf("true peak at or below %+.1f dBTP", profile.TruePeakMaxDBTP),
		fmt.Sprintf("channel configuration %s", profile.ChannelConfiguration),
	}
	if profile.HeadroomStrategy != "" {
		parts = append(parts, fmt.Sprintf("headroom strategy %s", profile.HeadroomStrategy))
	}
	if profile.ShortTermMax != nil {
		parts = append(parts, fmt.Sprintf("short-term max %+.1f LUFS", *profile.ShortTermMax))
	}
	if profile.MomentaryMax != nil {
		parts = append(parts, fmt.Sprintf("momentary max %+.1f LUFS", *profile.MomentaryMax))
	}
	return strings.Join(parts, "; ")
}

// renderBindingsCarryUnresolvable reports whether any disclosure row failed to
// resolve (used by Build to surface the projection-level limitation).
func renderBindingsCarryUnresolvable(rows []RenderBindingDisclosure) bool {
	for _, row := range rows {
		if row.Status == RenderBindingStatusUnresolvable {
			return true
		}
	}
	return false
}
