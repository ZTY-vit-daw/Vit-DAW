package orchestrationcontroller

import (
	"testing"
	"time"
)

func TestSelectorMapsFourPeerControllers(t *testing.T) {
	tests := []struct {
		name  string
		input SelectionInput
		want  Kind
	}{
		{"discussion", SelectionInput{SemanticRoute: "discussion", TargetScope: "none", ControlMode: "none", Authorization: "none", Reason: "explain"}, OrdinaryConversation},
		{"direct", SelectionInput{SemanticRoute: "explicit_control", TargetScope: "current_selection", ControlMode: "typed_control", Authorization: "action_requested", Reason: "set an exact control"}, DirectTypedAction},
		{"closure", SelectionInput{SemanticRoute: "open_semantic", TargetScope: "current_selection", ControlMode: "semantic_loop", Authorization: "action_requested", Reason: "close one acoustic issue"}, MinimalAudioClosure},
		{"project mix", SelectionInput{SemanticRoute: "open_semantic", TargetScope: "project_context", ControlMode: "semantic_loop", Authorization: "action_requested", ProposedController: ProjectMixWorkflow, Reason: "explicit whole-project mix"}, ProjectMixWorkflow},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := Select(test.input)
			if err != nil || decision.Controller != test.want {
				t.Fatalf("decision=%+v err=%v", decision, err)
			}
		})
	}
}

func TestProjectMixRequiresRuntimeProposalAndFullProjectScope(t *testing.T) {
	_, err := Select(SelectionInput{SemanticRoute: "open_semantic", TargetScope: "current_selection", ControlMode: "semantic_loop", Authorization: "action_requested", ProposedController: ProjectMixWorkflow, Reason: "bad route"})
	if err == nil {
		t.Fatal("project workflow admitted a selection-scoped request")
	}
	decision, err := Select(SelectionInput{SemanticRoute: "open_semantic", TargetScope: "project_context", ControlMode: "semantic_loop", Authorization: "action_requested", Reason: "ordinary project-context issue"})
	if err != nil || decision.Controller != MinimalAudioClosure {
		t.Fatalf("project context defaulted to fixed workflow: %+v err=%v", decision, err)
	}
	observation, err := Select(SelectionInput{SemanticRoute: "observation", TargetScope: "project_context", ControlMode: "observe_only", Authorization: "observe_only", ProposedController: ProjectMixWorkflow, Reason: "runtime capacity route"})
	if err != nil || observation.Controller != ProjectMixWorkflow {
		t.Fatalf("runtime-routed project observation was rejected: %+v err=%v", observation, err)
	}
}

func TestRegistryEnforcesOneOwnerAndSettledHandoff(t *testing.T) {
	registry := NewRegistry()
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	closure := Decision{SchemaVersion: DecisionSchema, Controller: MinimalAudioClosure, TargetScope: "current_selection", Authorization: "action_requested", SourceRoute: "open_semantic", Reason: "one issue"}
	owner, acquired, err := registry.Acquire("conversation", "closure-1", closure, now)
	if err != nil || !acquired {
		t.Fatalf("owner=%+v acquired=%v err=%v", owner, acquired, err)
	}
	project := Decision{SchemaVersion: DecisionSchema, Controller: ProjectMixWorkflow, TargetScope: "project_context", Authorization: "action_requested", SourceRoute: "open_semantic", Reason: "whole project"}
	if _, _, err := registry.Acquire("conversation", "mix-1", project, now); err == nil {
		t.Fatal("second controller acquired an active conversation")
	}
	if _, err := registry.Settle("conversation", "closure-1", owner.Revision, "handoff_requested", now); err != nil {
		t.Fatal(err)
	}
	if _, acquired, err := registry.Acquire("conversation", "mix-1", project, now); err != nil || !acquired {
		t.Fatalf("settled handoff failed: acquired=%v err=%v", acquired, err)
	}
}

func TestRegistrySnapshotRestoresActiveOwner(t *testing.T) {
	registry := NewRegistry()
	decision := Decision{SchemaVersion: DecisionSchema, Controller: OrdinaryConversation, TargetScope: "none", Authorization: "none", SourceRoute: "discussion", Reason: "talk"}
	if _, _, err := registry.Acquire("conversation", "ordinary-1", decision, time.Time{}); err != nil {
		t.Fatal(err)
	}
	restored := NewRegistry()
	if err := restored.Restore(registry.Snapshot()); err != nil {
		t.Fatal(err)
	}
	owner, ok := restored.Active("conversation")
	if !ok || owner.Controller != OrdinaryConversation {
		t.Fatalf("owner=%+v ok=%v", owner, ok)
	}
}
