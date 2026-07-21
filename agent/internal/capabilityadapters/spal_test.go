package capabilityadapters

import (
	"context"
	"testing"

	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
)

type adapterPreimageReader struct{ values []spal.PhysicalParameter }

func (r adapterPreimageReader) CaptureSPALPreimage(context.Context, spal.RuntimeBinding, []spal.PhysicalParameter) ([]spal.PhysicalParameter, error) {
	return append([]spal.PhysicalParameter(nil), r.values...), nil
}

func TestPlanSPALBlocksWithoutVerifiedInstanceInsteadOfChoosingTDRNova(t *testing.T) {
	adapter, err := spal.NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	registry := spal.NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	planned, err := PlanSPAL(SPALPlanRequest{
		SessionID: "s", Goal: "reduce low-end overlap", Mode: orchestration.InteractionPropose, CapabilityID: "static_mix.low_end_relation.v0",
		ProjectCut: executableSPALCut(), Instruction: adapterInstruction(), Registry: registry,
	})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Outcome.Kind != orchestration.OutcomeBlocked || len(planned.Outcome.Blockers) != 2 || planned.Preparation.Manifest != nil {
		t.Fatalf("missing provider did not become a Plugin Skill blocker: %#v", planned)
	}
}

func TestPlanSPALFreezesVendorIndependentProposalAfterBinding(t *testing.T) {
	adapter, err := spal.NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	registry := spal.NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	preimage := []spal.PhysicalParameter{{ParameterID: "1", Value: 0}, {ParameterID: "6", Value: 1}, {ParameterID: "4", Value: 100}, {ParameterID: "2", Value: 0}, {ParameterID: "3", Value: 1}}
	request := SPALPlanRequest{
		SessionID: "s", Goal: "reduce low-end overlap", Mode: orchestration.InteractionPropose, CapabilityID: "static_mix.low_end_relation.v0", CapabilityVersion: "v0",
		ProjectCut: executableSPALCut(), Instruction: adapterInstruction(), Registry: registry, PreimageReader: adapterPreimageReader{values: preimage},
		Instances: []spal.ProviderInstance{{
			ID: "instance", ProviderID: spal.ExperimentalTDRNovaProviderID, TargetRef: "track:bass", TrackID: "bass", PluginID: "plugin", Status: spal.InstanceVerified,
			PluginSignature: spal.ExperimentalTDRNovaSignature,
			Metadata:        map[string]string{"band_slot": "band1", "static_bell_ready": "true"},
		}},
	}
	planned, err := PlanSPAL(request)
	if err != nil {
		t.Fatal(err)
	}
	if planned.Outcome.Kind != orchestration.OutcomeProposal || planned.Preparation.Manifest == nil {
		t.Fatalf("unexpected SPAL plan: %#v", planned)
	}
	proposal, set, err := FreezeSPALProposal(planned, request.CapabilityID, request.CapabilityVersion, request.ProjectCut, 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.ActionSetHash != set.Hash || set.Actions[0].Command != spal.ActionCommand || set.Actions[0].Args["plugin_id"] != nil {
		t.Fatalf("frozen semantic action leaked or lost execution data: proposal=%#v set=%#v", proposal, set)
	}
}

func executableSPALCut() orchestration.ProjectCut {
	cut := orchestration.ProjectCut{ProjectUUID: "p", ProjectEpoch: "e", BaseProjectRevision: "1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	return cut
}

func adapterInstruction() spal.Instruction {
	return spal.Instruction{SchemaID: spal.StaticBellControlID, TargetRef: "track:bass", Parameters: map[string]float64{"center_frequency_hz": 92, "gain_db": -2, "q": 1.2}}
}
