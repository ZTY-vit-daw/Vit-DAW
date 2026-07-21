package spallab

import (
	"testing"

	"vit-daw-agent/internal/orchestration"
)

func TestProviderRegistrationFreezesProjectScopedConformanceRecord(t *testing.T) {
	request := labConformanceRequest()
	request.ProjectUUID = "project-reference-eq"
	record, err := ConformTDRNova(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := record.ProductReadyFor("project-reference-eq"); err != nil {
		t.Fatalf("product-scoped conformance rejected: %v", err)
	}
	if err := record.ProductReadyFor("another-project"); err == nil {
		t.Fatal("cross-project Provider record was accepted")
	}
	cut := orchestration.ProjectCut{
		ProjectUUID: "project-reference-eq", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong",
	}
	cut.Hash = cut.ComputeHash()
	proposal, actions, err := FreezeProviderRegistrationProposal(record, cut, 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.CapabilityID != ReferenceEQProviderRegistrationCapabilityID || len(actions.Actions) != 1 || actions.Actions[0].Command != ReferenceEQProviderRegistrationCommand {
		t.Fatalf("unexpected frozen registration: proposal=%#v actions=%#v", proposal, actions)
	}
	frozen, err := ProviderRecordFromRegistrationAction(actions.Actions[0])
	if err != nil {
		t.Fatal(err)
	}
	if frozen.ID != record.ID || frozen.ProjectUUID != record.ProjectUUID || frozen.Instance.PluginID != record.Instance.PluginID {
		t.Fatalf("frozen record changed identity: %#v", frozen)
	}
	session, err := orchestration.NewSession("provider-registration", cut.ProjectUUID, "register provider", orchestration.EngineV1, orchestration.CapabilityInvocation{CapabilityID: ReferenceEQProviderRegistrationCapabilityID})
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SetFrozenPlan(orchestration.FrozenPlan{Proposal: proposal, ActionSet: actions, ProjectCut: cut})
	if err != nil {
		t.Fatal(err)
	}
	if session.FrozenPlan == nil || session.FrozenPlan.ActionSet.Hash != session.FrozenPlan.ActionSet.ComputeHash() {
		t.Fatalf("provider registration action set lost its stable hash after frozen-plan persistence: %#v", session.FrozenPlan)
	}
}

func TestLegacyUnscopedRecordCannotEnterProductRegistrationAction(t *testing.T) {
	record, err := ConformTDRNova(labConformanceRequest())
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "project-reference-eq", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	if _, err := ProviderRegistrationActionSet(record, cut); err == nil {
		t.Fatal("unscoped developer-lab record became a product registration action")
	}
}
