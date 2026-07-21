package capabilityadapters

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/orchestration"
)

const PluginEffectControlCapabilityID = "plugin.effect_control.v0"

type PluginResolvedParameter struct {
	Slot               string  `json:"slot,omitempty"`
	ParameterID        string  `json:"parameter_id"`
	ParameterName      string  `json:"parameter_name,omitempty"`
	OldValue           float64 `json:"old_value,omitempty"`
	OldNormalizedValue float64 `json:"old_normalized_value"`
	OldValueText       string  `json:"old_value_text,omitempty"`
}

type PluginEffectControlPlan struct {
	TrackID              string                    `json:"track_id"`
	PluginID             string                    `json:"plugin_id"`
	Control              string                    `json:"control"`
	ApplyArgs            map[string]any            `json:"apply_args"`
	ResolvedParameters   []PluginResolvedParameter `json:"resolved_parameters"`
	ExpectedParameterIDs []string                  `json:"expected_parameter_ids"`
	ProfileSource        string                    `json:"profile_source,omitempty"`
	ProfileSignature     string                    `json:"profile_signature,omitempty"`
	Resolution           map[string]any            `json:"resolution,omitempty"`
	PreviousObservation  string                    `json:"previous_observation_id,omitempty"`
	BeforeEvidence       map[string]any            `json:"before_evidence,omitempty"`
}

func FreezePluginEffectControl(plan PluginEffectControlPlan, cut orchestration.ProjectCut, revision int64) (orchestration.Proposal, orchestration.ActionSet, error) {
	plan.TrackID = strings.TrimSpace(plan.TrackID)
	plan.PluginID = strings.TrimSpace(plan.PluginID)
	plan.Control = strings.TrimSpace(plan.Control)
	if plan.TrackID == "" || plan.PluginID == "" || plan.Control == "" {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("track_id, plugin_id, and control are required")
	}
	if len(plan.ResolvedParameters) == 0 {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("resolve-only preview returned no affected parameters")
	}
	if revision < 1 {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("positive proposal revision is required")
	}
	if cut.Hash == "" {
		cut.Hash = cut.ComputeHash()
	}
	if cut.Hash == "" || !cut.IsExecutable() {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("strong executable project cut is required")
	}

	ids := canonicalPluginParameterIDs(plan.ExpectedParameterIDs)
	if len(ids) == 0 {
		for _, item := range plan.ResolvedParameters {
			ids = append(ids, strings.TrimSpace(item.ParameterID))
		}
		ids = canonicalPluginParameterIDs(ids)
	}
	if len(ids) == 0 {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("affected parameter identities are required")
	}
	plan.ExpectedParameterIDs = ids
	args := map[string]any{
		"track_id":                plan.TrackID,
		"plugin_id":               plan.PluginID,
		"control":                 plan.Control,
		"apply_args":              clonePluginMap(plan.ApplyArgs),
		"resolved_parameters":     plan.ResolvedParameters,
		"expected_parameter_ids":  append([]string(nil), ids...),
		"profile_source":          strings.TrimSpace(plan.ProfileSource),
		"profile_signature":       strings.TrimSpace(plan.ProfileSignature),
		"resolution":              clonePluginMap(plan.Resolution),
		"previous_observation_id": strings.TrimSpace(plan.PreviousObservation),
		"before_evidence":         clonePluginMap(plan.BeforeEvidence),
	}
	action := orchestration.Action{
		ID:                fmt.Sprintf("plugin_effect:%s:%s", plan.PluginID, strings.Join(ids, "+")),
		Command:           "plugin_grabber.apply_control.governed",
		TargetRef:         "plugin:" + plan.TrackID + ":" + plan.PluginID,
		BeforeFingerprint: pluginPreimageFingerprint(plan.ResolvedParameters),
		Args:              args,
		Compensatable:     true,
		IdempotencyClass:  "semantic_control_with_frozen_parameter_preimage",
	}
	actionSet := orchestration.ActionSet{
		ID:             "actionset_plugin_effect_" + plan.PluginID,
		CapabilityID:   PluginEffectControlCapabilityID,
		ProjectCutHash: cut.Hash,
		Actions:        []orchestration.Action{action},
	}
	actionSet.Hash = actionSet.ComputeHash()
	proposal := orchestration.Proposal{
		ID:              "proposal_" + actionSet.Hash[:16],
		Revision:        revision,
		CapabilityID:    PluginEffectControlCapabilityID,
		CapabilityVer:   "v0",
		ProjectCutHash:  cut.Hash,
		CandidateID:     "plugin_effect_" + actionSet.Hash[:16],
		ActionSetHash:   actionSet.Hash,
		TargetScope:     []string{action.TargetRef},
		Risk:            "bounded_reversible",
		VerificationRef: "plugin.effect_control.verification.v0",
		Summary:         fmt.Sprintf("Apply %s to plugin %s on track %s (%d frozen parameters)", plan.Control, plan.PluginID, plan.TrackID, len(ids)),
		CreatedAt:       time.Now().UTC(),
	}
	return proposal, actionSet, nil
}

func canonicalPluginParameterIDs(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func pluginPreimageFingerprint(values []PluginResolvedParameter) string {
	rows := make([]string, 0, len(values))
	for _, value := range values {
		rows = append(rows, fmt.Sprintf("%s=%0.9f", strings.TrimSpace(value.ParameterID), value.OldNormalizedValue))
	}
	sort.Strings(rows)
	return "plugin-preimage:" + strings.Join(rows, ",")
}

func clonePluginMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
