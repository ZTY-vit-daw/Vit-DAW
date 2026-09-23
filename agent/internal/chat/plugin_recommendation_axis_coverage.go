package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
	agentruntime "vit-daw-agent/internal/runtime"
)

// D2-FAM2-S3 axis coverage governance. The family-level recommendation face
// used to offer every family-promoted candidate, while the axis-level
// execution gate (semanticValidatePCAAdmissionReceiptForInput) later rejected
// selections whose attestation does not cover the frozen semantic axis
// (190456: Smack Attack promoted for transient_shaper but certified for
// envelope_timing, not envelope_emphasis). This file narrows the offered set
// before the model chooses: when the frozen semantic intent rides the
// free-state channel, a candidate stays offered only under the three-way
// eligibility conjunction (a) family-promoted (unchanged upstream filter),
// (b) frozen-axis coverage, (c) domain whitelist form. The coverage predicate
// is the same authority the execution gate uses — no second axis semantics is
// invented here — and the axis words come from the structured intent channel
// (domain-row-derived on the admitted-experiment path) mapped through the
// processor registry, never from model prose. The execution gate itself is
// deliberately untouched and remains the fail-closed backstop.

const (
	// pluginRecommendationAxisCoverageDisclosureKey carries the server-built
	// disclosure from the governed filter to the selection payload.
	pluginRecommendationAxisCoverageDisclosureKey      = "plugin_recommendation_axis_coverage_disclosure"
	pluginRecommendationAxisCoverageDisclosureSchema   = "plugin_recommendation_axis_coverage_disclosure.v1"
	pluginRecommendationAxisCoverageNoCandidatesStatus = "no_axis_covered_candidates"
)

// axisCoverageFrozenSemanticIntent reads the frozen semantic processor intent
// from the free-state channel. Absent intent means the ungoverned
// recommendation face: callers must leave the candidate set untouched.
func axisCoverageFrozenSemanticIntent(requestContext map[string]any) (map[string]any, bool) {
	intent := firstMapFromAny(requestContext["free_state_semantic_processor_intent"])
	if len(intent) == 0 {
		return nil, false
	}
	return intent, true
}

// axisCoverageWhitelistIdentifiers resolves the machine-local whitelist
// plugin identifiers for the family (v6 candidate lists may carry more than
// one; a v5-shaped section yields its single identifier). An absent section
// (or absent whitelist file) yields no identifiers — no candidate can then
// be whitelist-form eligible, which surfaces as the honest empty set rather
// than an error. A corrupt whitelist fails closed.
func axisCoverageWhitelistIdentifiers(family string) (map[string]bool, error) {
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) {
			return nil, nil
		}
		return nil, fmt.Errorf("axis coverage governance could not read the experiment plugin whitelist: %w", err)
	}
	identifiers := map[string]bool{}
	switch family {
	case processorattestation.FamilyStaticEQ:
		for _, entry := range whitelist.StaticEQ {
			identifiers[entry.PluginIdentifier] = true
		}
	case processorattestation.FamilyBroadbandCompressor:
		for _, entry := range whitelist.BroadbandCompression {
			identifiers[entry.PluginIdentifier] = true
		}
	case processorattestation.FamilyDeEsser:
		for _, entry := range whitelist.DeEsser {
			identifiers[entry.PluginIdentifier] = true
		}
	case processorattestation.FamilyTransient:
		for _, entry := range whitelist.TransientShaper {
			identifiers[entry.PluginIdentifier] = true
		}
	case processorattestation.FamilyLimiter:
		for _, entry := range whitelist.Limiter {
			identifiers[entry.PluginIdentifier] = true
		}
	case processorattestation.FamilyGateExpander:
		for _, entry := range whitelist.GateExpander {
			identifiers[entry.PluginIdentifier] = true
		}
	case processorattestation.FamilyMultiband:
		for _, entry := range whitelist.Multiband {
			identifiers[entry.PluginIdentifier] = true
		}
	}
	return identifiers, nil
}

// axisCoverageGovernedPluginRecommendationCandidates applies the three-way
// eligibility conjunction to family-promoted candidates when the frozen
// semantic intent is present. The returned disclosure is non-nil exactly when
// governance was active; it carries the frozen semantic axes and, per kept
// candidate, the attested coverage axes of its promoted attestation. An
// honest empty set (nil candidates, non-nil disclosure) means no
// family-promoted candidate is simultaneously axis-covered and
// whitelist-form eligible — callers must fail with that named gap instead of
// falling back to unqualified candidates.
func axisCoverageGovernedPluginRecommendationCandidates(candidates []pluginRecommendationCandidate, processorType string, requestContext map[string]any) ([]pluginRecommendationCandidate, map[string]any, error) {
	intent, governed := axisCoverageFrozenSemanticIntent(requestContext)
	if !governed {
		return candidates, nil, nil
	}
	// Same derivation as the execution-gate input chain: the structured
	// intent (family + required_coverage axes) is mapped through the
	// processor registry into exact PCA action coverage and validated. A
	// malformed or family-mismatched intent fails closed here instead of
	// silently widening the offered set.
	requirement, err := pluginControlRequirementFromSemanticIntent(intent, processorType)
	if err != nil {
		return nil, nil, fmt.Errorf("frozen semantic intent cannot govern recommendation eligibility: %w", err)
	}
	raw, err := json.Marshal(intent)
	if err != nil {
		return nil, nil, fmt.Errorf("frozen semantic intent cannot govern recommendation eligibility: %w", err)
	}
	decoded, err := processorintent.Decode(string(raw))
	if err != nil {
		return nil, nil, fmt.Errorf("frozen semantic intent cannot govern recommendation eligibility: %w", err)
	}
	whitelistIdentifiers, err := axisCoverageWhitelistIdentifiers(requirement.ProcessorFamily)
	if err != nil {
		return nil, nil, err
	}
	out := make([]pluginRecommendationCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if len(whitelistIdentifiers) == 0 || !whitelistIdentifiers[strings.TrimSpace(candidate.Identifier)] {
			continue
		}
		result, queryErr := processorattestation.QueryInstalled(processorattestation.InstalledSubject{Subject: processorattestation.Subject{
			Name: candidate.Name, Manufacturer: candidate.Manufacturer, Format: candidate.Format,
			Identifier: candidate.Identifier, InstalledPath: candidate.PluginPath,
		}}, processorattestation.EligibilityRequirement{ProcessorFamily: requirement.ProcessorFamily, RequiredCoverage: requirement.Coverage})
		if queryErr != nil {
			return nil, nil, fmt.Errorf("authoritative PCA coverage query failed: %w", queryErr)
		}
		if !result.Eligible {
			continue
		}
		candidate.Key = fmt.Sprintf("plugin_candidate_%d", len(out)+1)
		candidate.ProcessorFamily = requirement.ProcessorFamily
		candidate.SubjectKey = result.SubjectKey
		candidate.BinaryFingerprint = result.BinaryFingerprint
		candidate.AttestationID = result.AttestationID
		out = append(out, candidate)
	}
	disclosure, err := axisCoverageDisclosurePayload(out, requirement, decoded.RequiredCoverage)
	if err != nil {
		return nil, nil, err
	}
	return out, disclosure, nil
}

// axisCoverageDisclosurePayload discloses the frozen semantic axes plus each
// kept candidate's attested coverage axes. Content comes only from the
// promoted attestation library and the frozen intent — zero fixture truth and
// zero model text.
func axisCoverageDisclosurePayload(candidates []pluginRecommendationCandidate, requirement pluginControlRequirement, frozenAxes []string) (map[string]any, error) {
	rows := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		axes, err := axisCoverageAttestedAxes(candidate)
		if err != nil {
			return nil, err
		}
		rows = append(rows, map[string]any{
			"candidate_key":          candidate.Key,
			"attested_coverage_axes": axes,
			"binary_fingerprint":     candidate.BinaryFingerprint,
			"attestation_id":         candidate.AttestationID,
		})
	}
	required := make([]map[string]any, 0, len(requirement.Coverage))
	for _, coverage := range requirement.Coverage {
		required = append(required, map[string]any{"action": coverage.Action, "axis": coverage.Axis, "shape": coverage.Shape})
	}
	return map[string]any{
		"schema_version":           pluginRecommendationAxisCoverageDisclosureSchema,
		"frozen_semantic_axes":     append([]string(nil), frozenAxes...),
		"required_action_coverage": required,
		"candidates":               rows,
	}, nil
}

// axisCoverageAttestedAxes lists the promoted attestation's coverage axes for
// one kept candidate, matched by its exact attestation identity. v1
// attestations whose coverage is identified by shape fall back to the shape
// label so the disclosure never invents an axis the certificate does not
// carry.
func axisCoverageAttestedAxes(candidate pluginRecommendationCandidate) ([]string, error) {
	if candidate.AttestationID == "" {
		return nil, fmt.Errorf("axis coverage disclosure requires the promoted attestation id for %s", candidate.Identifier)
	}
	if processorattestation.IsV2Family(candidate.ProcessorFamily) {
		store, err := processorattestation.NewStoreV2("")
		if err != nil {
			return nil, err
		}
		library, _, err := store.Read()
		if err != nil {
			return nil, err
		}
		for _, attestation := range library.Attestations {
			if attestation.AttestationID != candidate.AttestationID || attestation.Status != processorattestation.StatusPromoted {
				continue
			}
			return attestationCoverageLabels(attestation.Coverage), nil
		}
		return nil, nil
	}
	store, err := processorattestation.NewStore("")
	if err != nil {
		return nil, err
	}
	library, _, err := store.Read()
	if err != nil {
		return nil, err
	}
	for _, attestation := range library.Attestations {
		if attestation.AttestationID != candidate.AttestationID || attestation.Status != processorattestation.StatusPromoted {
			continue
		}
		return attestationCoverageLabels(attestation.Coverage), nil
	}
	return nil, nil
}

// attestationCoverageLabels projects attestation coverage entries into sorted
// distinct labels, preferring the axis and falling back to the shape.
func attestationCoverageLabels(coverage []processorattestation.Coverage) []string {
	seen := map[string]bool{}
	labels := make([]string, 0, len(coverage))
	for _, item := range coverage {
		label := strings.TrimSpace(item.Axis)
		if label == "" {
			label = strings.TrimSpace(item.Shape)
		}
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

// pluginRecommendationAxisCoverageNoCandidatesResponse is the honest empty
// set for governed recommendations: the status names the exact gap (no
// family-promoted candidate is simultaneously axis-covered and
// whitelist-form eligible), no fallback candidate is offered, and nothing is
// selected, loaded, or mutated.
func (s *Server) pluginRecommendationAxisCoverageNoCandidatesResponse(conversationID, mode, userText string, requestContext map[string]any, res agentloop.Result, processorType string, frozenAxes []string) ChatResponse {
	requestContext = s.bindFreeStateAuthoritativeTrack(conversationID, requestContext)
	res.ExecutionMemory.PendingMixTreatment = nil
	res.Status = agentruntime.StatusCompleted
	res.StopReason = pluginRecommendationAxisCoverageNoCandidatesStatus
	res.Error = ""
	res.Continuation = nil
	res.Reply = fmt.Sprintf("当前冻结语义轴（%s）下，本机没有同时满足 PCA 族准入、冻结轴覆盖与实验白名单形态的 %s 候选；没有回退到不可执行候选，也没有选择、加载或修改工程。",
		strings.Join(frozenAxes, "+"), pluginRecommendationProcessorLabel(processorType))
	resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	resp.Workflow = pluginRecommendationWorkflow
	resp.WorkflowData = map[string]any{
		"schema_version": pluginRecommendationSchema,
		"status":         pluginRecommendationAxisCoverageNoCandidatesStatus,
		"admission":      "pca_promoted_axis_covered_whitelist_form",
		"processor_type": processorType,
		"listening_goal": strings.TrimSpace(userText),
		"target_ref":     map[string]any{"kind": "track", "id": firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id"), "label": firstStringFromMap(requestContext, "selected_track_name")},
		"axis_coverage_disclosure": map[string]any{
			"schema_version":       pluginRecommendationAxisCoverageDisclosureSchema,
			"frozen_semantic_axes": append([]string(nil), frozenAxes...),
		},
		"selection_performed": false,
		"mutation_performed":  false,
	}
	return resp
}
