package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
)

const pluginControlRequirementSchema = "processor_control_requirement.v1"

type pluginControlRequirement struct {
	SchemaVersion   string                          `json:"schema_version"`
	ProcessorFamily string                          `json:"processor_family"`
	Coverage        []processorattestation.Coverage `json:"coverage"`
	Reason          string                          `json:"reason"`
}

func processorAttestationFamily(processorType string) string {
	switch canonicalPluginRecommendationProcessorType(processorType) {
	case "eq":
		return processorattestation.FamilyStaticEQ
	case "compressor":
		return processorattestation.FamilyBroadbandCompressor
	case "limiter":
		return processorattestation.FamilyLimiter
	case "gate_expander", "gate", "expander":
		return processorattestation.FamilyGateExpander
	case "de_esser", "deesser":
		return processorattestation.FamilyDeEsser
	case "transient_shaper", "transient":
		return processorattestation.FamilyTransient
	case "multiband", "multiband_dynamics":
		return processorattestation.FamilyMultiband
	default:
		return ""
	}
}

func (s *Server) inferPluginControlRequirement(ctx context.Context, conversationID, userText, processorType string,
	requestContext map[string]any, observation *agentloop.RecentObservation, cfg config.EngineConfig) (pluginControlRequirement, error) {
	family := processorAttestationFamily(processorType)
	if family == "" {
		return pluginControlRequirement{}, nil
	}
	input := map[string]any{
		"user_request": userText, "processor_family": family,
		"target_context": map[string]any{"track_id": firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id"),
			"track_name": firstStringFromMap(requestContext, "selected_track_name")},
		"observation_context": semanticEQPlannerObservation(observation),
	}
	inputJSON, _ := json.Marshal(input)
	system := `Determine the abstract processor control action needed before local plugin recommendation. You cannot see plugin candidates and must not name a product.
Return ONLY JSON: {"schema_version":"processor_control_requirement.v1","processor_family":"static_eq|broadband_compressor|limiter|gate_expander|de_esser|transient_shaper|multiband_dynamics","coverage":[{"action":"...","shape":"...","axis":"..."}],"reason":"brief reason"}.
For static_eq, every item must use action=upsert and exactly one shape from bell, low_shelf, high_shelf, low_cut, high_cut. Include every distinct shape required by the current request, at most three. Static EQ items MUST omit the axis key entirely; never put a frequency band or semantic axis in a static_eq coverage item.
For broadband_compressor, every item must use action=adjust and exactly one axis from activation_intensity, transfer_severity, transient_timing, recovery_motion, detector_focus, output_normalization, parallel_balance, character. Include only axes required by the current request, at most four.
For limiter, use action=adjust and axes from detector_latency, input_drive, output_ceiling, output_normalization, peak_mode, protection_intensity, recovery_motion.
For gate_expander, use action=adjust and axes from activation_threshold, attenuation_floor, detector_focus, direction_mode, output_normalization, parallel_balance, state_timing.
For de_esser, use action=adjust and axes from detector_focus, output_normalization, parallel_balance, recovery_motion, sibilance_reduction, split_scope, threshold_sensitivity.
For transient_shaper, use action=adjust and axes from detector_focus, envelope_emphasis, envelope_timing, output_normalization, parallel_balance, shape_mode.
For multiband_dynamics, use action=adjust and axes from band_dynamics, band_timing, crossover_layout, detector_focus, output_normalization, parallel_balance.
For a vague EQ request use upsert+bell. For a vague compression request use adjust+activation_intensity. For a vague v2 request use the family primary axis: limiter+protection_intensity, gate_expander+activation_threshold, de_esser+sibilance_reduction, transient_shaper+envelope_emphasis, or multiband_dynamics+band_dynamics. Do not output parameter IDs, values, mappings, topology, plugin names, or prose.`
	response, err := s.llm.CompleteRequest(ctx, cfg, llm.Request{
		Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(inputJSON)}},
		Metadata: llm.RequestMetadata{Source: "processor_control_requirement", ConversationID: conversationID}, PreferJSON: true,
	})
	if err != nil {
		return pluginControlRequirement{}, err
	}
	var requirement pluginControlRequirement
	if err := decodePluginRecommendationJSON(response.Text, &requirement); err != nil {
		return requirement, fmt.Errorf("decode processor control requirement: %w", err)
	}
	requirement = normalizePluginControlRequirement(requirement)
	if err := validatePluginControlRequirement(processorType, requirement); err != nil {
		return requirement, err
	}
	return requirement, nil
}

func validatePluginControlRequirement(processorType string, requirement pluginControlRequirement) error {
	wantFamily := processorAttestationFamily(processorType)
	if requirement.SchemaVersion != pluginControlRequirementSchema || requirement.ProcessorFamily != wantFamily {
		return fmt.Errorf("processor control requirement family or schema mismatch")
	}
	if len(requirement.Coverage) == 0 || len(requirement.Coverage) > 4 {
		return fmt.Errorf("processor control requirement must contain 1-4 actions")
	}
	for _, coverage := range requirement.Coverage {
		var err error
		if processorattestation.IsV2Family(wantFamily) {
			err = processorattestation.ValidateV2Coverage(wantFamily, coverage)
		} else {
			err = processorattestation.ValidateCoverage(wantFamily, coverage)
		}
		if err != nil {
			return fmt.Errorf("invalid processor control requirement: %w", err)
		}
		if wantFamily == processorattestation.FamilyStaticEQ && coverage.Action != "upsert" {
			return fmt.Errorf("pre-load static EQ requirement must use upsert")
		}
		if processorattestation.IsV2Family(wantFamily) && coverage.Action != "adjust" {
			return fmt.Errorf("pre-load PCA v2 requirement must use adjust")
		}
	}
	return nil
}

// pluginControlRequirementFromSemanticIntent is the deterministic bridge from
// model-owned semantic axes to PCA action coverage. It never invents a family
// or a default axis and is used whenever the free-state intent is available.
func pluginControlRequirementFromSemanticIntent(value map[string]any, processorType string) (pluginControlRequirement, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return pluginControlRequirement{}, err
	}
	intent, err := processorintent.Decode(string(raw))
	if err != nil {
		return pluginControlRequirement{}, err
	}
	if intent.Status != processorintent.StatusResolved {
		return pluginControlRequirement{}, fmt.Errorf("semantic processor intent is not resolved")
	}
	wantFamily := processorAttestationFamily(processorType)
	if wantFamily == "" || intent.Family != wantFamily {
		return pluginControlRequirement{}, fmt.Errorf("semantic processor intent family does not match %s", processorType)
	}
	registry, err := processorregistry.Default()
	if err != nil {
		return pluginControlRequirement{}, err
	}
	coverage, err := registry.PCARequiredCoverage(intent.Family, intent.RequiredCoverage)
	if err != nil {
		return pluginControlRequirement{}, err
	}
	requirement := pluginControlRequirement{
		SchemaVersion: pluginControlRequirementSchema, ProcessorFamily: intent.Family,
		Coverage: coverage, Reason: "model-owned semantic_processor_intent.v1",
	}
	if err := validatePluginControlRequirement(processorType, requirement); err != nil {
		return pluginControlRequirement{}, err
	}
	return requirement, nil
}

func normalizePluginControlRequirement(requirement pluginControlRequirement) pluginControlRequirement {
	requirement.ProcessorFamily = strings.ToLower(strings.TrimSpace(requirement.ProcessorFamily))
	requirement.Reason = strings.TrimSpace(requirement.Reason)
	seen := map[string]bool{}
	coverage := make([]processorattestation.Coverage, 0, len(requirement.Coverage))
	for _, value := range requirement.Coverage {
		value.Action = strings.ToLower(strings.TrimSpace(value.Action))
		value.Shape = strings.ToLower(strings.TrimSpace(value.Shape))
		value.Axis = strings.ToLower(strings.TrimSpace(value.Axis))
		key := value.Action + "\x00" + value.Shape + "\x00" + value.Axis
		if !seen[key] {
			seen[key] = true
			coverage = append(coverage, value)
		}
	}
	requirement.Coverage = coverage
	return requirement
}

func attestedPluginRecommendationCandidates(candidates []pluginRecommendationCandidate, requirement pluginControlRequirement) ([]pluginRecommendationCandidate, error) {
	if processorattestation.IsV2Family(requirement.ProcessorFamily) {
		store, err := processorattestation.NewStoreV2("")
		if err != nil {
			return nil, err
		}
		library, _, err := store.Read()
		if err != nil {
			return nil, err
		}
		return filterAttestedPluginRecommendationCandidatesV2(library, candidates, requirement)
	}
	store, err := processorattestation.NewStore("")
	if err != nil {
		return nil, err
	}
	library, _, err := store.Read()
	if err != nil {
		return nil, err
	}
	return filterAttestedPluginRecommendationCandidates(library, candidates, requirement)
}

// admittedPluginRecommendationCandidates is the PCA admission boundary for a
// load candidate. PCA proves the exact promoted binary belongs to the chosen
// processor family; live inspection and the Typed Executor own every concrete
// control/action decision after the plug-in is loaded.
func admittedPluginRecommendationCandidates(candidates []pluginRecommendationCandidate, processorType string) ([]pluginRecommendationCandidate, error) {
	family := processorAttestationFamily(processorType)
	if family == "" {
		return nil, fmt.Errorf("processor family is unavailable")
	}
	if processorattestation.IsV2Family(family) {
		store, err := processorattestation.NewStoreV2("")
		if err != nil {
			return nil, err
		}
		library, _, err := store.Read()
		if err != nil {
			return nil, err
		}
		return filterPCAAdmittedPluginRecommendationCandidatesV2(library, candidates, family)
	}
	store, err := processorattestation.NewStore("")
	if err != nil {
		return nil, err
	}
	library, _, err := store.Read()
	if err != nil {
		return nil, err
	}
	return filterPCAAdmittedPluginRecommendationCandidates(library, candidates, family)
}

func filterPCAAdmittedPluginRecommendationCandidates(library processorattestation.Library, candidates []pluginRecommendationCandidate, family string) ([]pluginRecommendationCandidate, error) {
	if family != processorattestation.FamilyStaticEQ && family != processorattestation.FamilyBroadbandCompressor {
		return nil, fmt.Errorf("unsupported PCA v1 family %q", family)
	}
	out := make([]pluginRecommendationCandidate, 0, len(candidates))
	fingerprints := map[string]string{}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Identifier) == "" || strings.TrimSpace(candidate.PluginPath) == "" {
			continue
		}
		subject := processorattestation.Subject{Name: candidate.Name, Manufacturer: candidate.Manufacturer, Format: candidate.Format, Identifier: candidate.Identifier, InstalledPath: candidate.PluginPath}
		subjectKey, err := processorattestation.BuildSubjectKey(subject)
		if err != nil {
			continue
		}
		pathKey := strings.ToLower(strings.TrimSpace(candidate.PluginPath))
		fingerprint := fingerprints[pathKey]
		if fingerprint == "" {
			fingerprint, err = processorattestation.FingerprintPath(candidate.PluginPath)
			if err != nil {
				continue
			}
			fingerprints[pathKey] = fingerprint
		}
		result, err := processorattestation.QueryLibraryAdmission(library, subjectKey, fingerprint, family)
		if err != nil {
			return nil, err
		}
		if result.Eligible {
			candidate.Key = fmt.Sprintf("plugin_candidate_%d", len(out)+1)
			candidate.SubjectKey, candidate.BinaryFingerprint = subjectKey, fingerprint
			candidate.ProcessorFamily, candidate.AttestationID = family, result.Attestation.AttestationID
			out = append(out, candidate)
		}
	}
	return out, nil
}

func filterPCAAdmittedPluginRecommendationCandidatesV2(library processorattestation.LibraryV2, candidates []pluginRecommendationCandidate, family string) ([]pluginRecommendationCandidate, error) {
	if !processorattestation.IsV2Family(family) {
		return nil, fmt.Errorf("unsupported PCA v2 family %q", family)
	}
	out := make([]pluginRecommendationCandidate, 0, len(candidates))
	fingerprints := map[string]string{}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Identifier) == "" || strings.TrimSpace(candidate.PluginPath) == "" {
			continue
		}
		subject := processorattestation.Subject{Name: candidate.Name, Manufacturer: candidate.Manufacturer, Format: candidate.Format, Identifier: candidate.Identifier, InstalledPath: candidate.PluginPath}
		subjectKey, err := processorattestation.BuildSubjectKey(subject)
		if err != nil {
			continue
		}
		pathKey := strings.ToLower(strings.TrimSpace(candidate.PluginPath))
		fingerprint := fingerprints[pathKey]
		if fingerprint == "" {
			fingerprint, err = processorattestation.FingerprintPath(candidate.PluginPath)
			if err != nil {
				continue
			}
			fingerprints[pathKey] = fingerprint
		}
		result, err := processorattestation.QueryLibraryAdmissionV2(library, subjectKey, fingerprint, family)
		if err != nil {
			return nil, err
		}
		if result.Eligible {
			candidate.Key = fmt.Sprintf("plugin_candidate_%d", len(out)+1)
			candidate.SubjectKey, candidate.BinaryFingerprint = subjectKey, fingerprint
			candidate.ProcessorFamily, candidate.AttestationID = family, result.Attestation.AttestationID
			out = append(out, candidate)
		}
	}
	return out, nil
}

func filterAttestedPluginRecommendationCandidatesV2(library processorattestation.LibraryV2, candidates []pluginRecommendationCandidate, requirement pluginControlRequirement) ([]pluginRecommendationCandidate, error) {
	if err := validatePluginControlRequirement(map[string]string{
		processorattestation.FamilyLimiter: "limiter", processorattestation.FamilyGateExpander: "gate_expander",
		processorattestation.FamilyDeEsser: "de_esser", processorattestation.FamilyTransient: "transient_shaper",
		processorattestation.FamilyMultiband: "multiband_dynamics",
	}[requirement.ProcessorFamily], requirement); err != nil {
		return nil, err
	}
	promotedSubjects := map[string]bool{}
	for _, attestation := range library.Attestations {
		if attestation.Status == processorattestation.StatusPromoted && attestation.ProcessorFamily == requirement.ProcessorFamily {
			promotedSubjects[attestation.Subject.SubjectKey] = true
		}
	}
	fingerprints := map[string]string{}
	out := make([]pluginRecommendationCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Identifier) == "" || strings.TrimSpace(candidate.PluginPath) == "" {
			continue
		}
		subject := processorattestation.Subject{Name: candidate.Name, Manufacturer: candidate.Manufacturer, Format: candidate.Format, Identifier: candidate.Identifier, InstalledPath: candidate.PluginPath}
		subjectKey, err := processorattestation.BuildSubjectKey(subject)
		if err != nil || !promotedSubjects[subjectKey] {
			continue
		}
		pathKey := strings.ToLower(strings.TrimSpace(candidate.PluginPath))
		fingerprint := fingerprints[pathKey]
		if fingerprint == "" {
			fingerprint, err = processorattestation.FingerprintPath(candidate.PluginPath)
			if err != nil {
				continue
			}
			fingerprints[pathKey] = fingerprint
		}
		result, err := processorattestation.QueryLibraryV2(library, processorattestation.QueryV2{SubjectKey: subjectKey, BinaryFingerprint: fingerprint, ProcessorFamily: requirement.ProcessorFamily, RequiredCoverage: requirement.Coverage})
		if err != nil {
			return nil, err
		}
		if result.Eligible {
			candidate.Key = fmt.Sprintf("plugin_candidate_%d", len(out)+1)
			candidate.SubjectKey = subjectKey
			candidate.BinaryFingerprint = fingerprint
			candidate.ProcessorFamily = requirement.ProcessorFamily
			candidate.AttestationID = result.Attestation.AttestationID
			out = append(out, candidate)
		}
	}
	return out, nil
}

func filterAttestedPluginRecommendationCandidates(library processorattestation.Library, candidates []pluginRecommendationCandidate,
	requirement pluginControlRequirement) ([]pluginRecommendationCandidate, error) {
	if err := validatePluginControlRequirement(map[string]string{
		processorattestation.FamilyStaticEQ: "eq", processorattestation.FamilyBroadbandCompressor: "compressor",
	}[requirement.ProcessorFamily], requirement); err != nil {
		return nil, err
	}
	promotedSubjects := map[string]bool{}
	for _, attestation := range library.Attestations {
		if attestation.Status == processorattestation.StatusPromoted && attestation.ProcessorFamily == requirement.ProcessorFamily {
			promotedSubjects[attestation.Subject.SubjectKey] = true
		}
	}
	fingerprints := map[string]string{}
	out := make([]pluginRecommendationCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Identifier) == "" || strings.TrimSpace(candidate.PluginPath) == "" {
			continue
		}
		subject := processorattestation.Subject{Name: candidate.Name, Manufacturer: candidate.Manufacturer, Format: candidate.Format,
			Identifier: candidate.Identifier, InstalledPath: candidate.PluginPath}
		subjectKey, err := processorattestation.BuildSubjectKey(subject)
		if err != nil || !promotedSubjects[subjectKey] {
			continue
		}
		pathKey := strings.ToLower(strings.TrimSpace(candidate.PluginPath))
		fingerprint := fingerprints[pathKey]
		if fingerprint == "" {
			fingerprint, err = processorattestation.FingerprintPath(candidate.PluginPath)
			if err != nil {
				continue
			}
			fingerprints[pathKey] = fingerprint
		}
		result, err := processorattestation.QueryLibrary(library, processorattestation.Query{SubjectKey: subjectKey,
			BinaryFingerprint: fingerprint, ProcessorFamily: requirement.ProcessorFamily, RequiredCoverage: requirement.Coverage})
		if err != nil {
			return nil, err
		}
		if result.Eligible {
			candidate.Key = fmt.Sprintf("plugin_candidate_%d", len(out)+1)
			candidate.SubjectKey = subjectKey
			candidate.BinaryFingerprint = fingerprint
			candidate.ProcessorFamily = requirement.ProcessorFamily
			candidate.AttestationID = result.Attestation.AttestationID
			out = append(out, candidate)
		}
	}
	return out, nil
}

func verifyPluginRecommendationAttestation(selected map[string]any, requirement pluginControlRequirement) error {
	_, err := currentAttestedPluginRecommendationCandidate(selected, requirement)
	return err
}

func currentAttestedPluginRecommendationCandidate(selected map[string]any, requirement pluginControlRequirement) (pluginRecommendationCandidate, error) {
	if strings.TrimSpace(firstStringFromMap(selected, "identifier")) == "" {
		return pluginRecommendationCandidate{}, fmt.Errorf("selected plugin exact identifier is required")
	}
	candidate := pluginRecommendationCandidate{Name: firstStringFromMap(selected, "name"),
		Manufacturer: firstStringFromMap(selected, "manufacturer"), Format: firstStringFromMap(selected, "format"),
		Identifier: firstStringFromMap(selected, "identifier"), PluginPath: firstStringFromMap(selected, "plugin_path")}
	filtered, err := attestedPluginRecommendationCandidates([]pluginRecommendationCandidate{candidate}, requirement)
	if err != nil {
		return pluginRecommendationCandidate{}, err
	}
	if len(filtered) != 1 {
		return pluginRecommendationCandidate{}, fmt.Errorf("selected plugin no longer has promoted coverage for the current action")
	}
	return filtered[0], nil
}

func currentPCAAdmittedPluginRecommendationCandidate(selected map[string]any, family string) (pluginRecommendationCandidate, error) {
	family = strings.ToLower(strings.TrimSpace(family))
	candidate := pluginRecommendationCandidate{Name: firstStringFromMap(selected, "name"),
		Manufacturer: firstStringFromMap(selected, "manufacturer"), Format: firstStringFromMap(selected, "format"),
		Identifier: firstStringFromMap(selected, "identifier"), PluginPath: firstStringFromMap(selected, "plugin_path")}
	if candidate.Identifier == "" || candidate.PluginPath == "" {
		return pluginRecommendationCandidate{}, fmt.Errorf("selected plugin exact identity is required")
	}
	var filtered []pluginRecommendationCandidate
	var err error
	if processorattestation.IsV2Family(family) {
		store, storeErr := processorattestation.NewStoreV2("")
		if storeErr != nil {
			return pluginRecommendationCandidate{}, storeErr
		}
		library, _, readErr := store.Read()
		if readErr != nil {
			return pluginRecommendationCandidate{}, readErr
		}
		filtered, err = filterPCAAdmittedPluginRecommendationCandidatesV2(library, []pluginRecommendationCandidate{candidate}, family)
	} else {
		store, storeErr := processorattestation.NewStore("")
		if storeErr != nil {
			return pluginRecommendationCandidate{}, storeErr
		}
		library, _, readErr := store.Read()
		if readErr != nil {
			return pluginRecommendationCandidate{}, readErr
		}
		filtered, err = filterPCAAdmittedPluginRecommendationCandidates(library, []pluginRecommendationCandidate{candidate}, family)
	}
	if err != nil {
		return pluginRecommendationCandidate{}, err
	}
	if len(filtered) != 1 {
		return pluginRecommendationCandidate{}, fmt.Errorf("selected plugin no longer has a promoted current-binary PCA admission")
	}
	return filtered[0], nil
}

func pluginControlRequirementFromAny(value any) (pluginControlRequirement, bool) {
	raw, err := json.Marshal(value)
	if err != nil {
		return pluginControlRequirement{}, false
	}
	var requirement pluginControlRequirement
	if err := json.Unmarshal(raw, &requirement); err != nil || requirement.SchemaVersion != pluginControlRequirementSchema {
		return pluginControlRequirement{}, false
	}
	return normalizePluginControlRequirement(requirement), true
}
