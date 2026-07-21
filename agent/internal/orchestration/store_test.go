package orchestration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestFileStoreRestoresSessionAndProposalAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestration.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSession("s1", "p1", "plan B2", EngineV1, CapabilityInvocation{CapabilityID: "static_mix.static_balance.v0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(session); err != nil {
		t.Fatal(err)
	}
	updated, err := session.SetProposal(Proposal{ID: "proposal-1", Revision: 1, ProjectCutHash: "cut-1", ActionSetHash: "action-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(updated, session.Revision); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := reopened.Load("s1")
	if !ok || restored.ActiveProposal == nil || restored.EngineOwner != EngineV1 {
		t.Fatalf("session did not survive reopen: %#v", restored)
	}
}

func TestFileStoreRestoresProposalPresentationAndApprovalDecision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestration.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSession("durable-proposal", "project", "plan B2", EngineV1, CapabilityInvocation{CapabilityID: "static_mix.static_balance.v0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(session); err != nil {
		t.Fatal(err)
	}
	cut := ProjectCut{ProjectUUID: "project", ProjectEpoch: "epoch", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	actionSet := ActionSet{ID: "actions", CapabilityID: "static_mix.static_balance.v0", ProjectCutHash: cut.Hash, Actions: []Action{{
		ID: "gain-1", Command: "track_gain_adjust", TargetRef: "track-1", Args: map[string]any{"delta_db": 0.3}, Compensatable: true,
	}}}
	actionSet.Hash = actionSet.ComputeHash()
	presentation := &ProposalPresentation{
		SchemaVersion: ProposalPresentationSchema, ProposalID: "proposal-1", ProposalRevision: 1,
		CapabilityID: actionSet.CapabilityID, Title: "B2 静态平衡方案", Conclusion: "建议微调一条轨道。", ActionCount: 1,
		Actions: []ProposalActionPreview{{ActionID: "gain-1", TrackID: "track-1", TrackName: "Lead", Delta: 0.3, Unit: "dB"}},
	}
	proposal := Proposal{
		ID: presentation.ProposalID, Revision: presentation.ProposalRevision, CapabilityID: actionSet.CapabilityID,
		ProjectCutHash: cut.Hash, ActionSetHash: actionSet.Hash, TargetScope: []string{"track-1"}, Presentation: presentation,
	}
	frozen, err := session.SetFrozenPlan(FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(frozen, session.Revision); err != nil {
		t.Fatal(err)
	}
	authorized, err := frozen.Authorize(testAuthorization(proposal.ID, proposal.Revision, proposal.ActionSetHash, proposal.ProjectCutHash, "turn-approval"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(authorized, frozen.Revision); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := reopened.Load(session.ID)
	if !ok || restored.ActiveProposal == nil || restored.ActiveProposal.Presentation == nil || restored.Authorization == nil || restored.Authorization.Decision == nil {
		t.Fatalf("durable proposal interaction state was lost: %#v", restored)
	}
	if restored.ActiveProposal.Presentation.Conclusion != presentation.Conclusion || !restored.Authorization.Decision.ExactApprovalFor(*restored.ActiveProposal) {
		t.Fatalf("presentation or exact approval binding changed after reopen: %#v", restored)
	}
}

func TestFileStoreSerializesIndependentInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestration.json")
	a, _ := NewFileStore(path)
	b, _ := NewFileStore(path)
	for i, store := range []*FileStore{a, b} {
		session, err := NewSession(fmt.Sprintf("instance-%d", i), "project", "test", EngineV1, CapabilityInvocation{CapabilityID: "test.capability"})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Create(session); err != nil {
			t.Fatal(err)
		}
	}
	if sessions, err := a.ListWithError(); err != nil || len(sessions) != 2 {
		t.Fatalf("independent instances lost an update: count=%d err=%v", len(sessions), err)
	}
}

func TestFileStoreSerializesMultipleProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestration.json")
	children := make([]*exec.Cmd, 0, 2)
	for worker := 0; worker < 2; worker++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestFileStoreProcessHelper$")
		cmd.Env = append(os.Environ(),
			"VIT_FILESTORE_PROCESS_HELPER=1",
			"VIT_FILESTORE_PROCESS_PATH="+path,
			"VIT_FILESTORE_PROCESS_WORKER="+strconv.Itoa(worker),
		)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, cmd)
	}
	for _, child := range children {
		if err := child.Wait(); err != nil {
			t.Fatalf("child failed: %v", err)
		}
	}
	store, _ := NewFileStore(path)
	sessions, err := store.ListWithError()
	if err != nil || len(sessions) != 80 {
		t.Fatalf("multi-process store lost updates: count=%d err=%v", len(sessions), err)
	}
}

func TestFileStoreProcessHelper(t *testing.T) {
	if os.Getenv("VIT_FILESTORE_PROCESS_HELPER") != "1" {
		return
	}
	path := os.Getenv("VIT_FILESTORE_PROCESS_PATH")
	worker := os.Getenv("VIT_FILESTORE_PROCESS_WORKER")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		session, err := NewSession(fmt.Sprintf("worker-%s-%03d", worker, i), "project", "test", EngineV1, CapabilityInvocation{CapabilityID: "test.capability"})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Create(session); err != nil {
			t.Fatal(err)
		}
	}
}
