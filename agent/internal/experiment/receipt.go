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
	// DispositionAdoptedByContinuation（FS-PARK-TURNFAIL-1）：继续对话默认
	// 采纳的收口 dispositions——与人耳裁决（retain/rollback）严格区分，人耳
	// A/B 层呈现 skipped_by_continuation（请求过、被继续对话收口、无判断证据）。
	DispositionAdoptedByContinuation ReceiptDisposition = "adopted_by_continuation"
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
	ParameterApplied      bool                  `json:"parameter_applied"`
	ReadbackVerified      bool                  `json:"readback_verified"`
	EvaluationReady       bool                  `json:"evaluation_ready"`
	HumanAuditionReady    bool                  `json:"human_audition_ready"`
	HumanConfirmed        bool                  `json:"human_confirmed"`
	Ambiguous             bool                  `json:"ambiguous"`
	RolledBack            bool                  `json:"rolled_back"`
	Settled               bool                  `json:"settled"`
	RecordedAt            time.Time             `json:"recorded_at"`
}

var receiptLayerEnums = map[string][]string{
	"technical_readback":   {"applied", "failed", "ambiguous"},
	"acoustic_materiality": {"none", "subthreshold", "material"},
	"target_response":      {"absent", "directional", "sufficient", "ambiguous"},
	"net_outcome":          {"improved", "stable", "plateau", "rolled_back"},
	// FS-PARK-TURNFAIL-1: skipped_by_continuation = 判断被请求过、被继续对话
	// 默认采纳收口、无判断证据——与 decided（人耳已判）严格区分。
	"human_ab": {"not_requested", "pending", "decided", "skipped_by_continuation"},
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
	case DispositionRetain, DispositionRollback, DispositionContinueOnce, DispositionRequestAudition, DispositionAdoptedByContinuation:
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
		// FS-PARK-TURNFAIL-1: adopted_by_continuation is a user-driven settle
		// (the user continued the conversation), not an automatic escalation.
		if r.Disposition != DispositionRequestAudition && r.Disposition != DispositionRollback && r.Disposition != DispositionAdoptedByContinuation {
			return fmt.Errorf("classification=ambiguous only allows disposition=request_audition, rollback, or adopted_by_continuation")
		}
	}
	if len(unique(r.EvidenceRefs)) == 0 {
		return fmt.Errorf("evidence_refs are required and must point at fresh, revision-bound observation receipts")
	}
	if r.ReadbackVerified && !r.ParameterApplied {
		return fmt.Errorf("readback_verified requires parameter_applied")
	}
	if r.EvaluationReady && !r.ReadbackVerified {
		return fmt.Errorf("evaluation_ready requires readback_verified")
	}
	if r.HumanAuditionReady && !r.EvaluationReady {
		return fmt.Errorf("human_audition_ready requires evaluation_ready")
	}
	if r.HumanConfirmed && !r.HumanAuditionReady {
		return fmt.Errorf("human_confirmed requires human_audition_ready")
	}
	if r.Ambiguous && r.HumanConfirmed {
		return fmt.Errorf("ambiguous receipt cannot be human_confirmed")
	}
	if r.RolledBack && r.Disposition != DispositionRollback {
		return fmt.Errorf("rolled_back requires rollback disposition")
	}
	if r.Settled && strings.EqualFold(r.Layers.HumanAB.Status, "pending") {
		return fmt.Errorf("settled receipt cannot have pending human A/B")
	}
	// FS-PARK-TURNFAIL-1 honesty boundary: a settled-by-continuation receipt
	// must state the skipped judgment explicitly and must never claim a human
	// decision, a retain, or an audible improvement.
	if r.Disposition == DispositionAdoptedByContinuation {
		if !strings.EqualFold(r.Layers.HumanAB.Status, "skipped_by_continuation") {
			return fmt.Errorf("adopted_by_continuation disposition requires human_ab=skipped_by_continuation")
		}
		if r.HumanConfirmed || r.Ambiguous {
			return fmt.Errorf("adopted_by_continuation receipt cannot claim a human judgment (human_confirmed=%v ambiguous=%v)", r.HumanConfirmed, r.Ambiguous)
		}
		if strings.EqualFold(r.Layers.NetOutcome.Status, "improved") {
			return fmt.Errorf("adopted_by_continuation receipt cannot claim net_outcome=improved without a decided human A/B")
		}
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
