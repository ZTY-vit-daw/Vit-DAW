package capabilityadapters

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/executionruntime"
	"vit-daw-agent/internal/executionverifiers"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
)

type spalExecutionVSP struct {
	snapshots []*kernel.VSPStateResult
	legacy    []*kernel.VSPCommandResult
	batches   []*kernel.VSPCommandResult
}

func (f *spalExecutionVSP) VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error) {
	if len(f.snapshots) == 0 {
		return nil, fmt.Errorf("unexpected state snapshot")
	}
	result := f.snapshots[0]
	f.snapshots = f.snapshots[1:]
	return result, nil
}

func (f *spalExecutionVSP) SendVSPLegacyCommandWithIDs(context.Context, map[string]any, string, string) (*kernel.VSPCommandResult, error) {
	if len(f.legacy) == 0 {
		return nil, fmt.Errorf("unexpected plugin readback")
	}
	result := f.legacy[0]
	f.legacy = f.legacy[1:]
	return result, nil
}

func (f *spalExecutionVSP) SendVSPCommandWithIDs(context.Context, string, map[string]any, string, string) (*kernel.VSPCommandResult, error) {
	if len(f.batches) == 0 {
		return nil, fmt.Errorf("unexpected parameter batch")
	}
	result := f.batches[0]
	f.batches = f.batches[1:]
	return result, nil
}

type spalExecutionPersistence struct{}

func (spalExecutionPersistence) PrepareExecution(context.Context, orchestration.PlanningSession, orchestration.ActionSet, orchestration.ProjectCut) ([]string, error) {
	return []string{"project-history:spal-test"}, nil
}

type spalExecutionSignal struct{}

func (spalExecutionSignal) VerifySPALSignal(context.Context, spal.ExecutionManifest, orchestration.ActionReceipt) (spal.SignalVerification, error) {
	return spal.SignalVerification{Status: "pass", EvidenceRefs: []string{"l2:before", "l2:after"}, Summary: "same-tap band energy decreased"}, nil
}

func TestSPALVerticalSliceRunsThroughFrozenProposalCoordinatorAndReceipt(t *testing.T) {
	adapter, err := spal.NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	registry := spal.NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	preimage := []spal.PhysicalParameter{{ParameterID: "1", Value: 0}, {ParameterID: "6", Value: 1}, {ParameterID: "4", Value: 100}, {ParameterID: "2", Value: 0}, {ParameterID: "3", Value: 1}}
	cut := executableSPALCut()
	planned, err := PlanSPAL(SPALPlanRequest{
		SessionID: "spal-session", Goal: "reduce bass low-end conflict", Mode: orchestration.InteractionPropose,
		CapabilityID: "static_mix.low_end_relation.v0", CapabilityVersion: "v0", ProjectCut: cut,
		Instruction: adapterInstruction(), Registry: registry, PreimageReader: adapterPreimageReader{values: preimage},
		Instances: []spal.ProviderInstance{{
			ID: "instance", ProviderID: spal.ExperimentalTDRNovaProviderID, TargetRef: "track:bass", TrackID: "bass", PluginID: "plugin", PluginSignature: spal.ExperimentalTDRNovaSignature, Status: spal.InstanceVerified,
			Metadata: map[string]string{"band_slot": "band1", "static_bell_ready": "true"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, actionSet, err := FreezeSPALProposal(planned, "static_mix.low_end_relation.v0", "v0", cut, 1)
	if err != nil {
		t.Fatal(err)
	}
	session, err := orchestration.NewSession("spal-session", "p", "reduce bass low-end conflict", orchestration.EngineV1, orchestration.CapabilityInvocation{
		CapabilityID: "static_mix.low_end_relation.v0", CapabilityVer: "v0", InteractionMode: orchestration.InteractionPropose, ProcessingPath: orchestration.PathCapability,
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SetFrozenPlan(orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: planned.Bundle.ID, PreviousObservationID: "obs-before"})
	if err != nil {
		t.Fatal(err)
	}
	store, err := orchestration.NewFileStore(filepath.Join(t.TempDir(), "spal-orchestration.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(session); err != nil {
		t.Fatal(err)
	}
	decision := orchestration.ApprovalDecision{SchemaVersion: orchestration.ApprovalDecisionSchema, Kind: orchestration.ApprovalApprove, ProposalID: proposal.ID, ProposalRevision: proposal.Revision, ActionSetHash: actionSet.Hash, ProjectCutHash: cut.Hash, SourceTurnID: "turn-1"}
	authorized, err := session.Authorize(orchestration.Authorization{ProposalID: proposal.ID, ProposalRevision: proposal.Revision, ActionSetHash: actionSet.Hash, ProjectCutHash: cut.Hash, SourceTurnID: "turn-1", Sequence: 1, Decision: &decision})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(authorized, session.Revision); err != nil {
		t.Fatal(err)
	}
	writes := planned.Preparation.Manifest.Writes
	client := &spalExecutionVSP{
		snapshots: []*kernel.VSPStateResult{
			{Response: map[string]any{"type": "state.snapshot"}, Payload: map[string]any{"status": "ok"}, ProjectEpoch: "e", Revision: 1},
			{Response: map[string]any{"type": "state.snapshot"}, Payload: map[string]any{"status": "ok"}, ProjectEpoch: "e", Revision: 2},
		},
		legacy: []*kernel.VSPCommandResult{
			executionReadback(preimage),
			executionReadback(writes),
		},
		batches: []*kernel.VSPCommandResult{{Response: map[string]any{"type": "command.response"}, Payload: map[string]any{"status": "ok"}, TransactionID: "tx-1"}},
	}
	coordinator := executionruntime.New(store)
	completed, err := coordinator.ExecuteWithPersistence(context.Background(), session.ID, actionSet, cut,
		&executionports.SPALVSPPort{Client: client}, executionverifiers.SPAL{Signal: spalExecutionSignal{}}, spalExecutionPersistence{})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != orchestration.StatusNeedsReview || completed.Execution == nil || completed.Execution.VerificationResult == nil || completed.Execution.VerificationResult.UserAcceptance != "unknown" {
		t.Fatalf("SPAL did not preserve the governed execution/review lifecycle: %#v", completed)
	}
	reloaded, ok := store.Load(session.ID)
	if !ok || len(reloaded.Execution.Receipts) != 1 || reloaded.Execution.Receipts[0].Details["spal_manifest_id"] != planned.Preparation.Manifest.ID {
		t.Fatalf("SPAL Receipt did not persist the bound execution facts: %#v", reloaded.Execution)
	}
}

func executionReadback(parameters []spal.PhysicalParameter) *kernel.VSPCommandResult {
	rows := make([]any, 0, len(parameters))
	for _, parameter := range parameters {
		rows = append(rows, map[string]any{"parameter_id": parameter.ParameterID, "value": parameter.Value})
	}
	return &kernel.VSPCommandResult{LegacyReply: map[string]any{"status": "ok", "parameters": rows}}
}
