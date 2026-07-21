package internal_test

import (
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/orchestration"
)

func TestFileStoreRestoresSessionAndProposalAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestration.json")
	store, err := orchestration.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	session, err := orchestration.NewSession("s1", "p1", "plan B2", orchestration.EngineV1, orchestration.CapabilityInvocation{CapabilityID: "static_mix.static_balance.v0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(session); err != nil {
		t.Fatal(err)
	}
	updated, err := session.SetProposal(orchestration.Proposal{ID: "proposal-1", Revision: 1, ProjectCutHash: "cut-1", ActionSetHash: "action-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(updated, session.Revision); err != nil {
		t.Fatal(err)
	}
	reopened, err := orchestration.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := reopened.Load("s1")
	if !ok || restored.ActiveProposal == nil || restored.EngineOwner != orchestration.EngineV1 {
		t.Fatalf("session did not survive reopen: %#v", restored)
	}
}
