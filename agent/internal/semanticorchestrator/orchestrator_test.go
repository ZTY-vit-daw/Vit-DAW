package semanticorchestrator

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
)

func resolvedLimiterIntent() processorintent.Intent {
	return processorintent.Intent{
		SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved,
		Family: processorintent.FamilyLimiter, Intent: "protect peaks",
		RequiredCoverage: []string{"output_ceiling"}, Scope: processorintent.ScopeCurrentTrack,
		ControlMode: processorintent.ControlModeSemantic, Confidence: .9,
	}
}

func TestOrchestratorKeepsFamilyModelOwnedAndMapsCoverageDeterministically(t *testing.T) {
	registry, err := processorregistry.Default()
	if err != nil {
		t.Fatal(err)
	}
	orchestrator, err := New(registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.AcceptIntent(resolvedLimiterIntent()); err != nil {
		t.Fatal(err)
	}
	definition, err := orchestrator.SelectAdapter()
	if err != nil || definition.Family != processorintent.FamilyLimiter || definition.PCAFamily != "limiter" {
		t.Fatalf("adapter=%+v err=%v", definition, err)
	}
	state := orchestrator.State()
	if len(state.RequiredCoverage) != 1 || state.RequiredCoverage[0].Axis != "output_ceiling" {
		t.Fatalf("coverage=%+v", state.RequiredCoverage)
	}
}

func TestOrchestratorRejectsViewMutationAndOutOfOrderApply(t *testing.T) {
	registry, _ := processorregistry.Default()
	orchestrator, _ := New(registry)
	if err := orchestrator.AcceptIntent(resolvedLimiterIntent()); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.RecordObservation([]string{"track.peak_structure"}, []string{"track.basic_energy"}); err == nil || !strings.Contains(err.Error(), "observation_view_mismatch") {
		t.Fatalf("view mismatch err=%v", err)
	}
	orchestrator, _ = New(registry)
	if err := orchestrator.AcceptIntent(resolvedLimiterIntent()); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.RecordObservation([]string{"track.peak_structure"}, []string{"track.peak_structure"}); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.Confirm(); err == nil {
		t.Fatal("out-of-order confirmation was accepted")
	}
	if err := orchestrator.SetControlBrief(); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.SetPhysicalTarget(); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.BindControlRefs([]string{"control_ref_1"}); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.Confirm(); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.RecordReceipt(map[string]any{"status": "applied"}); err != nil {
		t.Fatal(err)
	}
	if got := orchestrator.State().Stage; got != StageReceipt {
		t.Fatalf("stage=%s", got)
	}
}

func TestOrchestratorRejectsInspectOnlyFamily(t *testing.T) {
	registry, _ := processorregistry.Default()
	orchestrator, _ := New(registry)
	intent := resolvedLimiterIntent()
	intent.Family = processorintent.FamilySpectralDynamics
	intent.ControlMode = processorintent.ControlModeInspectOnly
	intent.RequiredCoverage = []string{"inspect_only"}
	if err := orchestrator.AcceptIntent(intent); err == nil || !strings.Contains(err.Error(), "inspect_only_family") {
		t.Fatalf("inspect-only family was accepted: %v", err)
	}
}
