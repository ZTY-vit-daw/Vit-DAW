package plugingrabber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/semanticeffect"
)

const CompressorControlBriefSchema = "compressor.control_brief.v1"

type CompressorControlBrief struct {
	SchemaVersion      string                          `json:"schema_version"`
	TopologyGeneration string                          `json:"topology_generation"`
	SelectedAxes       []string                        `json:"selected_axes"`
	Controls           []CompressorControlBriefControl `json:"controls"`
	Boundaries         []string                        `json:"boundaries"`
}

type CompressorControlBriefControl struct {
	PathKey         string                     `json:"path_key"`
	Section         string                     `json:"section"`
	Role            string                     `json:"role"`
	CurrentPhysical *float64                   `json:"current_physical,omitempty"`
	CurrentText     string                     `json:"current_text,omitempty"`
	PhysicalUnit    string                     `json:"physical_unit,omitempty"`
	Reachable       CompressorReachableSummary `json:"reachable_summary"`
}

type CompressorReachableSummary struct {
	Kind    string   `json:"kind"`
	Minimum *float64 `json:"minimum,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`
	Labels  []string `json:"labels,omitempty"`
	Status  string   `json:"status"`
}

// BuildAudioProcessorIdentityCard derives a compact semantic identity card
// from live topology facts plus supplied identity labels. Product identity
// never participates in control discovery or reachability.
func BuildAudioProcessorIdentityCard(digest ParameterDigest) (*semanticeffect.AudioProcessorIdentityCard, string) {
	model, boundary := DetectCompressorModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	name := firstNonEmptyString(digest.PluginName, firstNonEmptyText(digest.PluginIdentity, "plugin_name", "name"), "Unknown broadband compressor")
	manufacturer := firstNonEmptyText(digest.PluginIdentity, "manufacturer", "vendor", "maker")
	identityStatus := "unknown"
	if name != "Unknown broadband compressor" {
		identityStatus = "name_only"
	}
	if manufacturer != "" {
		identityStatus = "identified"
	}
	controls := compressorSignatureControls(model)
	boundaries := compressorSemanticHardBoundaries(model, controls)
	card := semanticeffect.AudioProcessorIdentityCard{
		SchemaVersion:     semanticeffect.AudioProcessorIdentityCardSchema,
		Identity:          semanticeffect.ProcessorIdentity{Name: name, Manufacturer: manufacturer},
		Archetype:         semanticeffect.ProcessorArchetype{EffectFamily: "broadband_compressor", InteractionStyle: compressorInteractionStyle(model.Classification)},
		SignatureControls: controls,
		HardBoundaries:    boundaries,
		IdentityStatus:    identityStatus,
		TopologyEvidence:  semanticeffect.ProcessorTopologyEvidence{Classification: model.Classification, Confidence: model.Confidence, Generation: model.Generation, Source: "live_generic_structural"},
	}
	card.CardID = compressorIdentityCardID(card)
	if err := card.Validate(); err != nil {
		return nil, "identity_card_invalid:" + err.Error()
	}
	return &card, ""
}

func BuildCompressorControlBrief(digest ParameterDigest, selectedAxes []string) (*CompressorControlBrief, string) {
	model, boundary := DetectCompressorModelWithBoundary(digest)
	if model == nil {
		return nil, boundary
	}
	axes := map[string]bool{}
	for _, axis := range selectedAxes {
		axes[strings.TrimSpace(axis)] = true
	}
	brief := &CompressorControlBrief{SchemaVersion: CompressorControlBriefSchema, TopologyGeneration: model.Generation, SelectedAxes: append([]string(nil), selectedAxes...)}
	appendBindings := func(pathKey, section string, bindings []CompressorBinding) {
		for _, binding := range bindings {
			if !compressorRoleServesSelectedAxis(binding.Role, axes) {
				continue
			}
			brief.Controls = append(brief.Controls, CompressorControlBriefControl{PathKey: pathKey, Section: section, Role: binding.Role,
				CurrentPhysical: cloneFloat(binding.CurrentPhysical), CurrentText: binding.CurrentText, PhysicalUnit: binding.PhysicalUnit,
				Reachable: compressorReachableSummary(binding)})
		}
	}
	for _, path := range model.Stage.ControlPaths {
		appendBindings(path.Key, "detector", path.Detector)
		appendBindings(path.Key, "operating_point", path.OperatingPoint)
		appendBindings(path.Key, "transfer", path.Transfer)
		appendBindings(path.Key, "timing", path.Timing)
		appendBindings(path.Key, "gain_action", path.GainAction)
	}
	appendBindings("stage_output", "output", model.Stage.Output)
	bindingCounts := map[string]int{}
	for _, control := range brief.Controls {
		bindingCounts[control.PathKey+"\x00"+control.Role]++
	}
	filtered := make([]CompressorControlBriefControl, 0, len(brief.Controls))
	ambiguous := []string{}
	seenAmbiguous := map[string]bool{}
	for _, control := range brief.Controls {
		key := control.PathKey + "\x00" + control.Role
		if bindingCounts[key] == 1 {
			filtered = append(filtered, control)
			continue
		}
		label := control.PathKey + "/" + control.Role
		if !seenAmbiguous[label] {
			ambiguous = append(ambiguous, label)
			seenAmbiguous[label] = true
		}
	}
	brief.Controls = filtered
	sort.Slice(brief.Controls, func(i, j int) bool {
		return brief.Controls[i].PathKey+"/"+brief.Controls[i].Role < brief.Controls[j].PathKey+"/"+brief.Controls[j].Role
	})
	brief.Boundaries = compressorSemanticHardBoundaries(model, compressorSignatureControls(model))
	sort.Strings(ambiguous)
	for _, label := range ambiguous {
		brief.Boundaries = append(brief.Boundaries, "ambiguous semantic binding unavailable: "+label)
	}
	if len(brief.Controls) == 0 {
		return nil, "selected_axes_unreachable"
	}
	return brief, ""
}

func compressorIdentityCardID(card semanticeffect.AudioProcessorIdentityCard) string {
	copy := card
	copy.CardID = ""
	encoded, _ := json.Marshal(copy)
	sum := sha256.Sum256(encoded)
	return "apsic1_" + hex.EncodeToString(sum[:10])
}

func compressorInteractionStyle(classification string) string {
	switch classification {
	case "threshold_driven":
		return "threshold_driven_variable_transfer"
	case "input_driven_ratio":
		return "input_driven_variable_transfer"
	case "input_driven_fixed_transfer":
		return "input_driven_fixed_transfer"
	case "amount_driven":
		return "amount_driven_leveler"
	default:
		return "structurally_identified_broadband_compressor"
	}
}

func compressorSignatureControls(model *CompressorModel) []string {
	priority := []string{"reduction_amount", "threshold", "input_drive", "ratio", "attack", "release", "recovery", "time_constant", "response", "sidechain_filter", "detector_mode", "reduction_range", "mix", "output_gain", "makeup_gain"}
	available := map[string]bool{}
	for _, path := range model.Stage.ControlPaths {
		for _, bindings := range [][]CompressorBinding{path.Detector, path.OperatingPoint, path.Transfer, path.Timing, path.GainAction} {
			for _, binding := range bindings {
				available[binding.Role] = true
			}
		}
	}
	for _, binding := range model.Stage.Output {
		available[binding.Role] = true
	}
	out := []string{}
	for _, role := range priority {
		if available[role] {
			out = append(out, role)
			if len(out) == semanticeffect.AudioProcessorSignatureControlMax {
				break
			}
		}
	}
	return out
}

func compressorSemanticHardBoundaries(model *CompressorModel, controls []string) []string {
	has := map[string]bool{}
	for _, role := range controls {
		has[role] = true
	}
	for _, path := range model.Stage.ControlPaths {
		for _, role := range append(append(append(append(append([]string{}, path.Capabilities.OperatingPoint...), path.Capabilities.Transfer...), path.Capabilities.Timing...), path.Capabilities.Detector...), path.Capabilities.GainAction...) {
			has[role] = true
		}
	}
	for _, role := range model.Stage.Capabilities.Output {
		has[role] = true
	}
	out := []string{"single-band broadband compressor only", "live topology is the execution fact"}
	missingGroups := []string{}
	if !has["threshold"] {
		missingGroups = append(missingGroups, "no independent threshold")
	}
	if !has["ratio"] {
		missingGroups = append(missingGroups, "no independent ratio")
	}
	if !has["attack"] {
		missingGroups = append(missingGroups, "no independent attack")
	}
	if !has["release"] && !has["recovery"] && !has["time_constant"] && !has["response"] {
		missingGroups = append(missingGroups, "no independent release or recovery")
	}
	out = append(out, missingGroups...)
	if has["output_gain"] || has["makeup_gain"] {
		out = append(out, "output or makeup gain is separate from compression intensity")
	}
	if has["mix"] || has["wet_gain"] || has["dry_gain"] {
		out = append(out, "parallel balance is separate from internal compression intensity")
	}
	if len(model.Stage.ControlPaths) != 1 {
		out = append(out, fmt.Sprintf("%d control paths require explicit path selection", len(model.Stage.ControlPaths)))
	}
	if len(out) > semanticeffect.AudioProcessorHardBoundaryMax {
		out = out[:semanticeffect.AudioProcessorHardBoundaryMax]
	}
	return out
}

func compressorRoleServesSelectedAxis(role string, axes map[string]bool) bool {
	for axis, roles := range map[string][]string{
		"activation_intensity": {"threshold", "input_drive", "reduction_amount", "low_level_amount", "high_level_amount"},
		"transfer_severity":    {"ratio", "knee", "transfer_mode", "direction_curve", "reduction_range"},
		"transient_timing":     {"attack", "lookahead", "transient_emphasis", "hold"},
		"recovery_motion":      {"release", "recovery", "time_constant", "response", "pdr_time", "auto_release", "timing_sync"},
		"detector_focus":       {"sidechain_filter", "detector_mode", "channel_link"},
		"output_normalization": {"output_gain", "makeup_gain", "auto_makeup"},
		"parallel_balance":     {"mix", "wet_gain", "dry_gain"},
		"character":            {"transfer_mode", "direction_curve", "detector_mode", "transient_emphasis"},
	} {
		if !axes[axis] {
			continue
		}
		for _, candidate := range roles {
			if role == candidate {
				return true
			}
		}
	}
	return false
}

func compressorReachableSummary(binding CompressorBinding) CompressorReachableSummary {
	summary := CompressorReachableSummary{Status: "unknown"}
	if len(binding.Reachable) > 0 {
		summary.Kind, summary.Status = "discrete", "ready"
		for _, value := range binding.Reachable {
			if strings.TrimSpace(value.Label) != "" && len(summary.Labels) < 16 {
				summary.Labels = append(summary.Labels, value.Label)
			}
			if value.Physical != nil {
				if summary.Minimum == nil || *value.Physical < *summary.Minimum {
					summary.Minimum = cloneFloat(value.Physical)
				}
				if summary.Maximum == nil || *value.Physical > *summary.Maximum {
					summary.Maximum = cloneFloat(value.Physical)
				}
			}
		}
		return summary
	}
	if binding.Domain != nil && binding.Domain.Min != nil && binding.Domain.Max != nil {
		summary.Kind, summary.Status = "continuous", "ready"
		summary.Minimum, summary.Maximum = cloneFloat(binding.Domain.Min), cloneFloat(binding.Domain.Max)
		return summary
	}
	if len(binding.Curve) > 0 {
		summary.Kind, summary.Status = "sampled_continuous", "bounded"
		for _, point := range binding.Curve {
			value := point[1]
			if summary.Minimum == nil || value < *summary.Minimum {
				summary.Minimum = &value
			}
			if summary.Maximum == nil || value > *summary.Maximum {
				summary.Maximum = &value
			}
		}
	}
	return summary
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
