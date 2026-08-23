package experiment

import (
	"strings"
	"testing"
	"time"
)

// Matrix M08/M09 (docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md): legal
// classification/disposition combinations and the global continue_once limit.

func validReceipt() ImprovementExecutionReceipt {
	return ImprovementExecutionReceipt{
		SchemaVersion:         ImprovementReceiptSchema,
		ReceiptID:             "fsx_abc123def456",
		ExperimentContractRef: "free_state_improvement_experiment_contract.v1:admission-1",
		ProjectRevision:       "rev-7",
		Classification:        ClassificationSubthreshold,
		Disposition:           DispositionRetain,
		Layers: ReceiptLayers{
			TechnicalReadback:   TechnicalReadbackLayer{Status: "applied"},
			AcousticMateriality: AcousticMaterialityLayer{Status: "subthreshold"},
			TargetResponse:      TargetResponseLayer{Status: "directional"},
			NetOutcome:          NetOutcomeLayer{Status: "stable"},
			HumanAB:             HumanABLayer{Status: "not_requested"},
		},
		EvidenceRefs: []string{"obs-target"},
		RecordedAt:   time.Now().UTC(),
	}
}

func TestM08ClassificationDispositionCombinations(t *testing.T) {
	legal := []struct {
		classification ReceiptClassification
		disposition    ReceiptDisposition
		layers         func(*ReceiptLayers)
	}{
		{ClassificationSubthreshold, DispositionRetain, nil},
		{ClassificationUnsupported, DispositionRollback, func(l *ReceiptLayers) {
			l.TargetResponse = TargetResponseLayer{Status: "absent"}
		}},
		{ClassificationAmbiguous, DispositionRequestAudition, func(l *ReceiptLayers) {
			l.TargetResponse = TargetResponseLayer{Status: "ambiguous"}
		}},
		{ClassificationAmbiguous, DispositionRollback, func(l *ReceiptLayers) {
			l.TargetResponse = TargetResponseLayer{Status: "ambiguous"}
		}},
		{ClassificationMaterial, DispositionRetain, func(l *ReceiptLayers) {
			l.AcousticMateriality = AcousticMaterialityLayer{Status: "material"}
			l.TargetResponse = TargetResponseLayer{Status: "sufficient"}
			l.NetOutcome = NetOutcomeLayer{Status: "improved"}
			l.HumanAB = HumanABLayer{Status: "decided"}
		}},
	}
	for _, combination := range legal {
		receipt := validReceipt()
		receipt.Classification, receipt.Disposition = combination.classification, combination.disposition
		if combination.layers != nil {
			combination.layers(&receipt.Layers)
		}
		if err := receipt.Validate(); err != nil {
			t.Fatalf("legal combination %s/%s rejected: %v", combination.classification, combination.disposition, err)
		}
	}
	illegal := []struct {
		name   string
		mutate func(*ImprovementExecutionReceipt)
		want   string
	}{
		{"ambiguous_continue_once", func(r *ImprovementExecutionReceipt) {
			r.Classification = ClassificationAmbiguous
			r.Disposition = DispositionContinueOnce
			r.Layers.TargetResponse = TargetResponseLayer{Status: "ambiguous"}
		}, "ambiguous"},
		{"ambiguous_retain", func(r *ImprovementExecutionReceipt) {
			r.Classification = ClassificationAmbiguous
			r.Disposition = DispositionRetain
			r.Layers.TargetResponse = TargetResponseLayer{Status: "ambiguous"}
		}, "ambiguous"},
		{"material_without_acoustic_materiality", func(r *ImprovementExecutionReceipt) {
			r.Classification = ClassificationMaterial
		}, "material"},
		{"unsupported_with_present_target", func(r *ImprovementExecutionReceipt) {
			r.Classification = ClassificationUnsupported
		}, "unsupported"},
		{"applied_readback_claims_improved", func(r *ImprovementExecutionReceipt) {
			r.Layers.NetOutcome = NetOutcomeLayer{Status: "improved"}
		}, "acoustic materiality"},
		{"improved_without_decided_human_ab", func(r *ImprovementExecutionReceipt) {
			r.Layers.AcousticMateriality = AcousticMaterialityLayer{Status: "material"}
			r.Layers.NetOutcome = NetOutcomeLayer{Status: "improved"}
		}, "human_ab"},
		{"layer_status_outside_enum", func(r *ImprovementExecutionReceipt) {
			r.Layers.TechnicalReadback = TechnicalReadbackLayer{Status: "sort_of"}
		}, "technical_readback"},
		{"empty_evidence_refs", func(r *ImprovementExecutionReceipt) {
			r.EvidenceRefs = nil
		}, "evidence_refs"},
		{"unbound_project_revision", func(r *ImprovementExecutionReceipt) {
			r.ProjectRevision = ""
		}, "project_revision"},
	}
	for _, combination := range illegal {
		receipt := validReceipt()
		combination.mutate(&receipt)
		err := receipt.Validate()
		if err == nil || !strings.Contains(err.Error(), combination.want) {
			t.Fatalf("%s not rejected as expected (%q): %v", combination.name, combination.want, err)
		}
	}
}

func TestM09ContinueOnceAtMostOnceGlobally(t *testing.T) {
	first := validReceipt()
	first.Disposition = DispositionContinueOnce
	if err := ValidateImprovementReceiptSet([]ImprovementExecutionReceipt{first}); err != nil {
		t.Fatalf("first continue_once rejected: %v", err)
	}
	second := validReceipt()
	second.ReceiptID = "fsx_second000001"
	second.Disposition = DispositionContinueOnce
	err := ValidateImprovementReceiptSet([]ImprovementExecutionReceipt{first, second})
	if err == nil || !strings.Contains(err.Error(), "continue_once") {
		t.Fatalf("second continue_once not rejected: %v", err)
	}
	// A continue_once followed by any non-continue_once receipt stays legal.
	retain := validReceipt()
	retain.ReceiptID = "fsx_retain0000001"
	if err := ValidateImprovementReceiptSet([]ImprovementExecutionReceipt{first, retain}); err != nil {
		t.Fatalf("continue_once then retain rejected: %v", err)
	}
}
