package experiment

import (
	"strings"
	"testing"
)

// Matrix M19 (docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md; receipt contract
// §3 red lines): the audio-outcome five-layer separation. Each layer is judged
// independently; an analytical change never becomes an audible-improvement
// claim on its own, and ambiguous only exits through audition or rollback.

// Red line 1: technical_readback=applied must not by itself produce
// net_outcome=improved. Acoustic materiality and a decided human A/B must be
// established independently before any improvement claim.
func TestM19AppliedReadbackAloneNeverClaimsImproved(t *testing.T) {
	// applied + improved with no independent materiality: rejected.
	receipt := validReceipt()
	receipt.Classification = ClassificationSubthreshold
	receipt.Layers.NetOutcome = NetOutcomeLayer{Status: "improved"}
	if err := receipt.Validate(); err == nil || !strings.Contains(err.Error(), "cannot by itself claim") {
		t.Fatalf("applied readback alone claimed improvement: %v", err)
	}
	// Independent materiality but no decided human A/B: still rejected.
	receipt.Layers.AcousticMateriality = AcousticMaterialityLayer{Status: "material"}
	receipt.Layers.TargetResponse = TargetResponseLayer{Status: "sufficient"}
	if err := receipt.Validate(); err == nil || !strings.Contains(err.Error(), "human_ab=decided") {
		t.Fatalf("improvement claimed before a decided human A/B: %v", err)
	}
	// Both independent layers established: the claim is legal.
	receipt.Layers.HumanAB = HumanABLayer{Status: "decided"}
	receipt.Classification = ClassificationMaterial
	if err := receipt.Validate(); err != nil {
		t.Fatalf("independently established improvement rejected: %v", err)
	}
}

// Red line 2: the five layers stay independent — a receipt that judges each
// layer on its own evidence validates, and no layer's value is ever derived
// from another by the validator itself.
func TestM19FiveLayersIndependentlyJudgedNoCrossFill(t *testing.T) {
	receipt := validReceipt()
	if err := receipt.Validate(); err != nil {
		t.Fatalf("independent per-layer judgments rejected: %v", err)
	}
	// Every layer combination is validated as recorded, never silently
	// rewritten: e.g. a failed readback with a directional target response is
	// inconsistent-but-recorded for classification purposes and must be
	// reviewed, not auto-filled.
	mixed := validReceipt()
	mixed.Layers.TechnicalReadback = TechnicalReadbackLayer{Status: "failed"}
	mixed.Classification = ClassificationSubthreshold
	if err := mixed.Validate(); err != nil {
		t.Fatalf("failed readback with directional target response rejected outright: %v", err)
	}
	// Out-of-enum values in any single layer are rejected in that layer's own
	// name (independent field, independent error).
	for name, mutate := range map[string]func(*ReceiptLayers){
		"technical_readback":   func(l *ReceiptLayers) { l.TechnicalReadback = TechnicalReadbackLayer{Status: "loud"} },
		"acoustic_materiality": func(l *ReceiptLayers) { l.AcousticMateriality = AcousticMaterialityLayer{Status: "better"} },
		"target_response":      func(l *ReceiptLayers) { l.TargetResponse = TargetResponseLayer{Status: "maybe"} },
		"net_outcome":          func(l *ReceiptLayers) { l.NetOutcome = NetOutcomeLayer{Status: "nice"} },
		"human_ab":             func(l *ReceiptLayers) { l.HumanAB = HumanABLayer{Status: "maybe_heard"} },
	} {
		outOfEnum := validReceipt()
		mutate(&outOfEnum.Layers)
		err := outOfEnum.Validate()
		if err == nil || !strings.Contains(err.Error(), "layers."+name) {
			t.Fatalf("out-of-enum %s not rejected in its own layer: %v", name, err)
		}
	}
}

// Red line 3: with human_ab not decided, no layer may carry an
// audible-improvement claim; request_audition is the legal ambiguous exit.
func TestM19UndecidedHumanABBlocksImprovementClaimsAndAmbiguousExits(t *testing.T) {
	// pending and not_requested both block net_outcome=improved.
	for _, humanAB := range []string{"pending", "not_requested"} {
		receipt := validReceipt()
		receipt.Layers.NetOutcome = NetOutcomeLayer{Status: "improved"}
		receipt.Layers.AcousticMateriality = AcousticMaterialityLayer{Status: "material"}
		receipt.Layers.TargetResponse = TargetResponseLayer{Status: "sufficient"}
		receipt.Layers.HumanAB = HumanABLayer{Status: humanAB}
		if err := receipt.Validate(); err == nil || !strings.Contains(err.Error(), "audible-improvement") {
			t.Fatalf("human_ab=%s allowed an improvement claim: %v", humanAB, err)
		}
	}
	// ambiguous only leaves through request_audition or rollback.
	for _, disposition := range []ReceiptDisposition{DispositionRetain, DispositionContinueOnce} {
		receipt := validReceipt()
		receipt.Classification = ClassificationAmbiguous
		receipt.Disposition = disposition
		if err := receipt.Validate(); err == nil {
			t.Fatalf("classification=ambiguous accepted disposition=%s", disposition)
		}
	}
	for _, disposition := range []ReceiptDisposition{DispositionRequestAudition, DispositionRollback} {
		receipt := validReceipt()
		receipt.Classification = ClassificationAmbiguous
		receipt.Disposition = disposition
		receipt.Layers.TargetResponse = TargetResponseLayer{Status: "ambiguous"}
		if err := receipt.Validate(); err != nil {
			t.Fatalf("classification=ambiguous rejected legal exit %s: %v", disposition, err)
		}
	}
}
