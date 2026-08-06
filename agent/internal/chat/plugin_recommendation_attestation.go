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
Return ONLY JSON: {"schema_version":"processor_control_requirement.v1","processor_family":"static_eq|broadband_compressor","coverage":[{"action":"...","shape":"...","axis":"..."}],"reason":"brief reason"}.
For static_eq, every item must use action=upsert and exactly one shape from bell, low_shelf, high_shelf, low_cut, high_cut. Include every distinct shape required by the current request, at most three.
For broadband_compressor, every item must use action=adjust and exactly one axis from activation_intensity, transfer_severity, transient_timing, recovery_motion, detector_focus, output_normalization, parallel_balance, character. Include only axes required by the current request, at most four.
For a vague EQ request use upsert+bell. For a vague compression request use adjust+activation_intensity. Do not output parameter IDs, values, mappings, topology, plugin names, or prose.`
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
		if err := processorattestation.ValidateCoverage(wantFamily, coverage); err != nil {
			return fmt.Errorf("invalid processor control requirement: %w", err)
		}
		if wantFamily == processorattestation.FamilyStaticEQ && coverage.Action != "upsert" {
			return fmt.Errorf("pre-load static EQ requirement must use upsert")
		}
	}
	return nil
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
			out = append(out, candidate)
		}
	}
	return out, nil
}

func verifyPluginRecommendationAttestation(selected map[string]any, requirement pluginControlRequirement) error {
	candidate := pluginRecommendationCandidate{Name: firstStringFromMap(selected, "name"),
		Manufacturer: firstStringFromMap(selected, "manufacturer"), Format: firstStringFromMap(selected, "format"),
		Identifier: firstStringFromMap(selected, "identifier"), PluginPath: firstStringFromMap(selected, "plugin_path")}
	filtered, err := attestedPluginRecommendationCandidates([]pluginRecommendationCandidate{candidate}, requirement)
	if err != nil {
		return err
	}
	if len(filtered) != 1 {
		return fmt.Errorf("selected plugin no longer has promoted coverage for the current action")
	}
	return nil
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
