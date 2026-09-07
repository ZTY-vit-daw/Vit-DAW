package chat

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
	"vit-daw-agent/internal/shadow"
)

// PCA-1 route-load × planning-admission coverage contract. The R1/R2 real-stack
// runs (20260906_221811 / 20260906_223107) deterministically died at
// pca_admission_rejected after a route-qualified load: the load gate is
// family-level by contract, while the planning admission requires exactly the
// PCARequiredCoverage of axes an LLM planner selected after the load. These
// tests pin both sides of the aligned contract with synthetic identities only:
// the admission gate must keep failing closed on uncovered axes, and the
// receipt-certified coverage must deterministically bound the planner-owned
// axis selection so a route-loaded instance stays executable.

// seedPromotedCompressorRouteReceipt seeds a promoted v1 compressor
// attestation whose coverage mirrors the certification-runner shape (only the
// measured roles: activation_intensity + transfer_severity) and returns the
// accompanied admission receipt exactly as the post-load handoff relays it.
func seedPromotedCompressorRouteReceipt(t *testing.T, axes []string) (semanticPCAAdmissionReceipt, processorattestation.Attestation) {
	t.Helper()
	path := t.TempDir() + "\\SyntheticCompressor.vst3"
	if err := os.WriteFile(path, []byte("route-loaded-compressor-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", t.TempDir()+"\\attestations.v1.json")
	fingerprint, err := processorattestation.FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "Synthetic Compressor", Manufacturer: "Test", Format: "VST3",
		Identifier: "synthetic-comp-v1", InstalledPath: path}
	coverage := make([]processorattestation.Coverage, 0, len(axes))
	for _, axis := range axes {
		coverage = append(coverage, processorattestation.Coverage{Action: "adjust", Axis: axis})
	}
	store, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyBroadbandCompressor,
		Coverage: coverage,
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "route-load-axis-bridge", Kind: "test",
			SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Now().UTC()}},
	}, "route_load_axis_bridge_test")
	if err != nil {
		t.Fatal(err)
	}
	subjectKey, err := processorattestation.BuildSubjectKey(subject)
	if err != nil {
		t.Fatal(err)
	}
	receipt := semanticPCAAdmissionReceipt{
		ProcessorFamily: processorattestation.FamilyBroadbandCompressor, Name: subject.Name, Manufacturer: subject.Manufacturer,
		Format: subject.Format, Identifier: subject.Identifier, PluginPath: subject.InstalledPath,
		SubjectKey: subjectKey, BinaryFingerprint: fingerprint, AttestationID: attestation.AttestationID,
	}
	return receipt, attestation
}

// The R1/R2 wall, reproduced synthetically: a route-qualified loaded instance
// (current promoted receipt) whose certified coverage does not contain every
// axis the post-load planner selected must stay rejected by the planning
// admission. This pin must survive the alignment unchanged.
func TestLoadedReceiptPlanningAdmissionStillRejectsUncoveredPlannerAxes(t *testing.T) {
	receipt, _ := seedPromotedCompressorRouteReceipt(t, []string{"activation_intensity", "transfer_severity"})
	registry, err := processorregistry.Default()
	if err != nil {
		t.Fatal(err)
	}
	required, err := registry.PCARequiredCoverage(processorintent.FamilyBroadbandCompressor,
		[]string{"activation_intensity", "transient_timing", "recovery_motion"})
	if err != nil {
		t.Fatal(err)
	}
	err = semanticValidatePCAAdmissionReceiptForInput(receipt, semanticTreatmentPCAInput{
		Family: processorintent.FamilyBroadbandCompressor, RequiredCoverage: required,
	})
	if err == nil || !strings.Contains(err.Error(), "does not cover frozen controls") || !strings.Contains(err.Error(), "required_action_not_covered") {
		t.Fatalf("planning admission no longer reproduces the R1/R2 fail-closed wall: %v", err)
	}
}

// The alignment bridge: a planner-owned axis selection riding an accompanied
// receipt is deterministically narrowed to the receipt-certified axes.
func TestLoadedReceiptAxisNarrowingBoundsPlannerAxesToCertifiedCoverage(t *testing.T) {
	receipt, attestation := seedPromotedCompressorRouteReceipt(t, []string{"activation_intensity", "transfer_severity"})
	requestContext := map[string]any{"pca_admission_receipt": semanticPCAAdmissionReceiptMap(receipt)}
	narrowing, applies, err := narrowCompressorIntentAxesToLoadedReceipt(requestContext,
		[]string{"activation_intensity", "transient_timing", "recovery_motion"})
	if err != nil || !applies {
		t.Fatalf("accompanied receipt did not bound the planner axes: applies=%v err=%v", applies, err)
	}
	if attestation.AttestationID != receipt.AttestationID {
		t.Fatalf("seed mismatch: %s != %s", attestation.AttestationID, receipt.AttestationID)
	}
	if strings.Join(narrowing.KeptAxes, ",") != "activation_intensity" ||
		strings.Join(narrowing.ExcludedAxes, ",") != "transient_timing,recovery_motion" ||
		strings.Join(narrowing.CertifiedAxes, ",") != "activation_intensity,transfer_severity" {
		t.Fatalf("unexpected narrowing %+v", narrowing)
	}
}

// After the alignment the narrowed selection must pass the exact planning
// admission that rejected R1/R2 — through the loaded-instance boundary, not a
// bypass.
func TestNarrowedLoadedReceiptAxesPassLoadedInstancePlanningAdmission(t *testing.T) {
	receipt, attestation := seedPromotedCompressorRouteReceipt(t, []string{"activation_intensity", "transfer_severity"})
	requestContext := map[string]any{"pca_admission_receipt": semanticPCAAdmissionReceiptMap(receipt)}
	narrowing, applies, err := narrowCompressorIntentAxesToLoadedReceipt(requestContext,
		[]string{"activation_intensity", "transient_timing", "recovery_motion"})
	if err != nil || !applies {
		t.Fatalf("narrowing bridge unavailable: applies=%v err=%v", applies, err)
	}
	registry, err := processorregistry.Default()
	if err != nil {
		t.Fatal(err)
	}
	required, err := registry.PCARequiredCoverage(processorintent.FamilyBroadbandCompressor, narrowing.KeptAxes)
	if err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_uuid": "project-1", "tracks": []any{map[string]any{
		"track_id": "track-1", "track_name": "Drums", "track_type": "audio", "is_audio_track": true,
		"plugins": []any{map[string]any{"plugin_id": "comp-1", "plugin_name": receipt.Name}},
	}}})
	server := New(nil, project, nil)
	server.eqKernelOverride = newFakeCompressorKernel()
	surface, err := server.semanticLoadedInstancePCAAdmissionForContext(context.Background(), requestContext, "track-1", "comp-1",
		semanticTreatmentPCAInput{Family: processorintent.FamilyBroadbandCompressor, RequiredCoverage: required})
	if err != nil || !surface.PCAEligible || surface.InspectOnly || surface.PCAStatus != "promoted" ||
		surface.PCAAttestationID != attestation.AttestationID {
		t.Fatalf("narrowed loaded-instance admission=%+v err=%v", surface, err)
	}
}

func TestLoadedReceiptAxisNarrowingTable(t *testing.T) {
	receipt, _ := seedPromotedCompressorRouteReceipt(t, []string{"activation_intensity", "transfer_severity"})
	for _, test := range []struct {
		name           string
		requestContext map[string]any
		selected       []string
		wantApplies    bool
		wantKept       string
		wantExcluded   string
		wantErr        string
	}{
		{
			name:           "no receipt leaves the planner axes untouched",
			requestContext: map[string]any{},
			selected:       []string{"transient_timing"},
			wantApplies:    false,
		},
		{
			name: "malformed receipt stays with the admission fail-closed path",
			requestContext: map[string]any{"pca_admission_receipt": map[string]any{
				"processor_family": processorintent.FamilyBroadbandCompressor, "name": "Half Receipt",
			}},
			selected:    []string{"activation_intensity"},
			wantApplies: false,
		},
		{
			name:           "selection inside the certified domain is unchanged",
			requestContext: map[string]any{"pca_admission_receipt": semanticPCAAdmissionReceiptMap(receipt)},
			selected:       []string{"transfer_severity", "activation_intensity"},
			wantApplies:    true,
			wantKept:       "transfer_severity,activation_intensity",
		},
		{
			name:           "selection beyond the certified domain is narrowed with disclosure",
			requestContext: map[string]any{"pca_admission_receipt": semanticPCAAdmissionReceiptMap(receipt)},
			selected:       []string{"activation_intensity", "transient_timing", "recovery_motion"},
			wantApplies:    true,
			wantKept:       "activation_intensity",
			wantExcluded:   "transient_timing,recovery_motion",
		},
		{
			name:           "empty intersection fails with the named certified-coverage gap",
			requestContext: map[string]any{"pca_admission_receipt": semanticPCAAdmissionReceiptMap(receipt)},
			selected:       []string{"transient_timing", "recovery_motion"},
			wantErr:        "pca_certified_axes_empty",
		},
		{
			name: "stale receipt attestation id certifies nothing",
			requestContext: map[string]any{"pca_admission_receipt": func() map[string]any {
				row := semanticPCAAdmissionReceiptMap(receipt)
				row["attestation_id"] = "pca1_does_not_exist"
				return row
			}()},
			selected: []string{"activation_intensity"},
			wantErr:  "pca_certified_axes_empty",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			narrowing, applies, err := narrowCompressorIntentAxesToLoadedReceipt(test.requestContext, test.selected)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("expected %q failure, got applies=%v narrowing=%+v err=%v", test.wantErr, applies, narrowing, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if applies != test.wantApplies {
				t.Fatalf("applies=%v want %v (narrowing=%+v)", applies, test.wantApplies, narrowing)
			}
			if !test.wantApplies {
				return
			}
			if got := strings.Join(narrowing.KeptAxes, ","); got != test.wantKept {
				t.Fatalf("kept=%q want %q", got, test.wantKept)
			}
			if got := strings.Join(narrowing.ExcludedAxes, ","); got != test.wantExcluded {
				t.Fatalf("excluded=%q want %q", got, test.wantExcluded)
			}
		})
	}
}
