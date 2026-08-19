package orchestrationruntime

import (
	"testing"

	"vit-daw-agent/internal/orchestration"
)

func TestAttachParentControllerIsCASPersistedImmutableAndAuthorizationNeutral(t *testing.T) {
	runtime := New()
	session, err := runtime.StartAgentSemanticEQChatSession("session-1", "conversation-1", "project-1", "reduce harshness", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	link := orchestration.ParentControllerLink{
		SchemaVersion: orchestration.ParentControllerLinkSchema,
		ControllerID:  "closure-1", ControllerType: "minimal_audio_closure",
		ClosureID: "closure-1", ClosureRevision: 4, ActionID: "action-1", ExpectedProjectRevision: "project-revision-1",
	}
	linked, err := runtime.AttachParentController(session.ID, link)
	if err != nil {
		t.Fatal(err)
	}
	if linked.ParentController == nil || *linked.ParentController != link {
		t.Fatalf("parent link missing: %+v", linked)
	}
	if linked.Status != orchestration.StatusAnalyzing || linked.Authorization != nil || linked.Revision != session.Revision+1 {
		t.Fatalf("parent link changed capability authority: %+v", linked)
	}
	duplicate, err := runtime.AttachParentController(session.ID, link)
	if err != nil || duplicate.Revision != linked.Revision {
		t.Fatalf("duplicate parent link was not idempotent: %+v err=%v", duplicate, err)
	}
	conflict := link
	conflict.ActionID = "action-2"
	if _, err := runtime.AttachParentController(session.ID, conflict); err == nil {
		t.Fatal("conflicting parent link replaced immutable correlation")
	}
	stored, ok := runtime.Store.Load(session.ID)
	if !ok || stored.ParentController == nil || stored.ParentController.ActionID != "action-1" {
		t.Fatalf("parent link was not persisted: %+v ok=%v", stored, ok)
	}
}
