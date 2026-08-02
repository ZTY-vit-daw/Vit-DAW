package capabilitycontext

import (
	"encoding/json"
	"fmt"

	"vit-daw-agent/internal/orchestration"
)

// OrchestrationBundle converts the existing B2 pack into the v1 control-plane
// representation. Full model rows remain behind the pack/artifact reference;
// only the bounded decision disclosure is exposed to the conversational layer.
func OrchestrationBundle(pack StaticBalancePack, projectCutHash string) orchestration.ContextBundle {
	disclosureBytes, _ := json.Marshal(pack.Decision)
	omissions := make([]string, 0, 2)
	omissionState := map[string]orchestration.OmissionStatus{}
	if pack.AnalyzedTrackCount > pack.DisclosedTrackCount {
		omissions = append(omissions, "omitted_budget")
		omissionState["track_rows"] = orchestration.OmissionBudget
	}
	if !pack.Readiness.CanProceed {
		omissions = append(omissions, "blocked_readiness")
		omissionState["readiness"] = orchestration.OmissionUnavailable
	}
	return orchestration.ContextBundle{
		ID:              pack.PackID,
		CapabilityID:    pack.CapabilityID,
		ProjectCutHash:  projectCutHash,
		ArtifactRefs:    []string{fmt.Sprintf("capability-pack:%s", pack.PackID)},
		EvidenceRefs:    append([]string(nil), pack.EvidenceRefs...),
		Disclosure:      string(disclosureBytes),
		OmissionReasons: omissions,
		Omissions:       omissionState,
	}
}

func PanLayoutOrchestrationBundle(pack PanLayoutPack, projectCutHash string) orchestration.ContextBundle {
	disclosureBytes, _ := json.Marshal(pack.Decision)
	omissions := make([]string, 0, 2)
	omissionState := map[string]orchestration.OmissionStatus{}
	if pack.AnalyzedTrackCount > pack.DisclosedTrackCount {
		omissions = append(omissions, "omitted_budget")
		omissionState["track_rows"] = orchestration.OmissionBudget
	}
	if !pack.Readiness.CanProceed {
		omissions = append(omissions, "blocked_readiness")
		omissionState["readiness"] = orchestration.OmissionUnavailable
	}
	return orchestration.ContextBundle{
		ID: pack.PackID, CapabilityID: pack.CapabilityID, ProjectCutHash: projectCutHash,
		ArtifactRefs: []string{fmt.Sprintf("capability-pack:%s", pack.PackID)},
		EvidenceRefs: append([]string(nil), pack.EvidenceRefs...), Disclosure: string(disclosureBytes),
		OmissionReasons: omissions, Omissions: omissionState,
	}
}

func LowEndRelationOrchestrationBundle(pack LowEndRelationPack, projectCutHash string) orchestration.ContextBundle {
	disclosureBytes, _ := json.Marshal(pack.AnalysisDisclosure)
	omissions := make([]string, 0, 1)
	omissionState := map[string]orchestration.OmissionStatus{}
	if !pack.Readiness.CanProceed {
		omissions = append(omissions, "blocked_readiness")
		omissionState["readiness"] = orchestration.OmissionUnavailable
	}
	return orchestration.ContextBundle{
		ID:              pack.PackID,
		CapabilityID:    pack.CapabilityID,
		ProjectCutHash:  projectCutHash,
		ArtifactRefs:    []string{fmt.Sprintf("capability-pack:%s", pack.PackID)},
		EvidenceRefs:    append([]string(nil), pack.EvidenceRefs...),
		Disclosure:      string(disclosureBytes),
		OmissionReasons: omissions,
		Omissions:       omissionState,
	}
}

func FrequencyCleanupOrchestrationBundle(pack FrequencyCleanupPack, projectCutHash string) orchestration.ContextBundle {
	disclosureBytes, _ := json.Marshal(pack.AnalysisDisclosure)
	omissions := []string{}
	omissionState := map[string]orchestration.OmissionStatus{}
	if !pack.Readiness.Diagnosis.CanProceed {
		omissions = append(omissions, "blocked_diagnosis_readiness")
		omissionState["diagnosis_readiness"] = orchestration.OmissionUnavailable
	}
	if !pack.Readiness.Mutation.CanProceed {
		omissions = append(omissions, "blocked_mutation_readiness")
		omissionState["post_fx_baseline"] = orchestration.OmissionUnavailable
	}
	return orchestration.ContextBundle{ID: pack.PackID, CapabilityID: pack.CapabilityID, ProjectCutHash: projectCutHash, ArtifactRefs: []string{fmt.Sprintf("capability-pack:%s", pack.PackID)}, EvidenceRefs: append([]string(nil), pack.EvidenceRefs...), Disclosure: string(disclosureBytes), OmissionReasons: omissions, Omissions: omissionState}
}
