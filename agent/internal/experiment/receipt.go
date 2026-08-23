package experiment

import (
	"fmt"
	"strings"
	"time"
)

// Free-state improvement execution receipt
// (docs/FREE_STATE_IMPROVEMENT_EXECUTION_RECEIPT_V1.md). Phase B implements
// the schema types and machine validation only: no Apply/Rollback/Settlement
// or real audio writes happen here.

const (
	ImprovementReceiptSchema            = "free_state_improvement_execution_receipt.v1"
	ImprovementExperimentContractSchema = "free_state_improvement_experiment_contract.v1"
	MaxContinueOnce                     = 1
)

// ReceiptClassification is the materiality classification of one experiment.
type ReceiptClassification string

const (
	ClassificationMaterial     ReceiptClassification = "material"
	ClassificationSubthreshold ReceiptClassification = "subthreshold"
	ClassificationAmbiguous    ReceiptClassification = "ambiguous"
	ClassificationUnsupported  ReceiptClassification = "unsupported"
)

// ReceiptDisposition is what happened to the applied change.
type ReceiptDisposition string

const (
	DispositionRetain          ReceiptDisposition = "retain"
	DispositionRollback        ReceiptDisposition = "rollback"
	DispositionContinueOnce    ReceiptDisposition = "continue_once"
	DispositionRequestAudition ReceiptDisposition = "request_audition"
)

// TechnicalReadbackLayer: engineering change readback, never an acoustic
// conclusion.
type TechnicalReadbackLayer struct {
	Status string `json:"status"` // applied | failed | ambiguous
}

// AcousticMaterialityLayer: audibly-measured materiality.
type AcousticMaterialityLayer struct {
	Status string `json:"status"` // none | subthreshold | material
}

// TargetResponseLayer: evidence-backed response of the treatment target.
type TargetResponseLayer struct {
	Status string `json:"status"` // absent | directional | sufficient | ambiguous
}

// NetOutcomeLayer: net engineering outcome.
type NetOutcomeLayer struct {
	Status string `json:"status"` // improved | stable | plateau | rolled_back | ...
}

// HumanABLayer: human A/B judgment state.
type HumanABLayer struct {
	Status string `json:"status"` // not_requested | pending | decided
}

// ReceiptLayers is the five-layer separation (ADR §11): each layer is judged
// and recorded independently; no layer may fill another.
type ReceiptLayers struct {
	TechnicalReadback   TechnicalReadbackLayer   `json:"technical_readback"`
	AcousticMateriality AcousticMaterialityLayer `json:"acoustic_materiality"`
	TargetResponse      TargetResponseLayer      `json:"target_response"`
	NetOutcome          NetOutcomeLayer          `json:"net_outcome"`
	HumanAB             HumanABLayer             `json:"human_ab"`
}

// ImprovementExecutionReceipt is `free_state_improvement_execution_receipt.v1`.
type ImprovementExecutionReceipt struct {
	SchemaVersion         string                `json:"schema_version"`
	ReceiptID             string                `json:"receipt_id"`
	ExperimentContractRef string                `json:"experiment_contract_ref"`
	ProjectRevision       string                `json:"project_revision"`
	Classification        ReceiptClassification `json:"classification"`
	Disposition           ReceiptDisposition    `json:"disposition"`
	Layers                ReceiptLayers         `json:"layers"`
	EvidenceRefs          []string              `json:"evidence_refs"`
	Limitations           []string              `json:"limitations,omitempty"`
	RecordedAt            time.Time             `json:"recorded_at"`
}

var receiptLayerEnums = map[string][]string{
	"technical_readback":   {"applied", "failed", "ambiguous"},
	"acoustic_materiality": {"none", "subthreshold", "material"},
	"target_response":      {"absent", "directional", "sufficient", "ambiguous"},
	"net_outcome":          {"improved", "stable", "plateau", "rolled_back"},
	"human_ab":             {"not_requested", "pending", "decided"},
}

func inEnum(value string, allowed []string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func (r ImprovementExecutionReceipt) Validate() error {
	if strings.TrimSpace(r.SchemaVersion) != ImprovementReceiptSchema {
		return fmt.Errorf("receipt schema_version must be %s", ImprovementReceiptSchema)
	}
	if !strings.HasPrefix(strings.TrimSpace(r.ReceiptID), "fsx_") {
		return fmt.Errorf("receipt_id must use the fsx_ prefix")
	}
	if strings.TrimSpace(r.ExperimentContractRef) == "" {
		return fmt.Errorf("experiment_contract_ref is required")
	}
	if strings.TrimSpace(r.ProjectRevision) == "" {
		return fmt.Errorf("project_revision is required (receipts are revision-bound)")
	}
	switch r.Classification {
	case ClassificationMaterial, ClassificationSubthreshold, ClassificationAmbiguous, ClassificationUnsupported:
	default:
		return fmt.Errorf("unknown classification %q", r.Classification)
	}
	switch r.Disposition {
	case DispositionRetain, DispositionRollback, DispositionContinueOnce, DispositionRequestAudition:
	default:
		return fmt.Errorf("unknown disposition %q", r.Disposition)
	}
	for name, status := range map[string]string{
		"technical_readback":   r.Layers.TechnicalReadback.Status,
		"acoustic_materiality": r.Layers.AcousticMateriality.Status,
		"target_response":      r.Layers.TargetResponse.Status,
		"net_outcome":          r.Layers.NetOutcome.Status,
		"human_ab":             r.Layers.HumanAB.Status,
	} {
		if !inEnum(status, receiptLayerEnums[name]) {
			return fmt.Errorf("layers.%s status %q is outside its enumeration", name, status)
		}
	}
	// Red line 1: an applied technical readback alone may not produce an
	// improved net outcome (project-change semantics, not acoustic proof).
	if strings.EqualFold(r.Layers.TechnicalReadback.Status, "applied") &&
		strings.EqualFold(r.Layers.NetOutcome.Status, "improved") &&
		!strings.EqualFold(r.Layers.AcousticMateriality.Status, "material") {
		return fmt.Errorf("technical_readback=applied cannot by itself claim net_outcome=improved; acoustic materiality must be independently established")
	}
	// Red line 3: no audible-improvement wording before a decided human A/B.
	if !strings.EqualFold(r.Layers.HumanAB.Status, "decided") &&
		strings.EqualFold(r.Layers.NetOutcome.Status, "improved") {
		return fmt.Errorf("net_outcome=improved requires human_ab=decided before any audible-improvement claim")
	}
	switch r.Classification {
	case ClassificationMaterial:
		if !strings.EqualFold(r.Layers.AcousticMateriality.Status, "material") ||
			strings.EqualFold(r.Layers.TargetResponse.Status, "absent") {
			return fmt.Errorf("classification=material requires acoustic_materiality=material and a non-absent target_response")
		}
	case ClassificationUnsupported:
		if !strings.EqualFold(r.Layers.TargetResponse.Status, "absent") &&
			!strings.EqualFold(r.Layers.TechnicalReadback.Status, "failed") {
			return fmt.Errorf("classification=unsupported requires target_response=absent or technical_readback=failed")
		}
	case ClassificationAmbiguous:
		// ADR §8: ambiguous stops automatic escalation.
		if r.Disposition == DispositionContinueOnce {
			return fmt.Errorf("classification=ambiguous forbids disposition=continue_once (ambiguous stops automatic escalation)")
		}
		if r.Disposition != DispositionRequestAudition && r.Disposition != DispositionRollback {
			return fmt.Errorf("classification=ambiguous only allows disposition=request_audition or rollback")
		}
	}
	if len(unique(r.EvidenceRefs)) == 0 {
		return fmt.Errorf("evidence_refs are required and must point at fresh, revision-bound observation receipts")
	}
	return nil
}

// ValidateImprovementReceiptSet enforces the global invariant over a receipt
// history: continue_once at most once overall (Policy.MaxActionAttempts=1).
func ValidateImprovementReceiptSet(receipts []ImprovementExecutionReceipt) error {
	continueOnce := 0
	for i, receipt := range receipts {
		if err := receipt.Validate(); err != nil {
			return fmt.Errorf("receipt %d: %w", i, err)
		}
		if receipt.Disposition == DispositionContinueOnce {
			continueOnce++
			if continueOnce > MaxContinueOnce {
				return fmt.Errorf("disposition=continue_once exceeded its global limit of %d", MaxContinueOnce)
			}
		}
	}
	return nil
}
