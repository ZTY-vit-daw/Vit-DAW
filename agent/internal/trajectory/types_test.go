package trajectory

import (
	"testing"
	"time"

	"vit-daw-agent/internal/taskstate"
)

func TestEventNormalizeAndValidate(t *testing.T) {
	event := Event{
		Type:           EventMaterialityEvaluated,
		ConversationID: " conversation-1 ",
		GoalID:         "goal-1",
		RunID:          "run-1",
		ItemID:         "materiality-1",
		Payload: Payload{
			RoundID:      "round-1",
			Materiality:  EvaluationInsufficientDose,
			EvidenceRefs: []string{" obs-1 ", "obs-1", ""},
		},
	}.Normalize()
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if event.Payload.SchemaVersion != SchemaVersion || event.Payload.TurnID != "run-1" {
		t.Fatalf("identity = %#v", event.Payload)
	}
	if event.Payload.NodeKind != NodeMateriality || event.Payload.Phase != "materiality_evaluating" || event.Payload.Status != StatusCompleted {
		t.Fatalf("defaults = %#v", event.Payload)
	}
	if len(event.Payload.EvidenceRefs) != 1 || event.Payload.EvidenceRefs[0] != "obs-1" {
		t.Fatalf("evidence refs = %#v", event.Payload.EvidenceRefs)
	}
}

func TestEventRejectsMissingRoundForRoundEvent(t *testing.T) {
	event := Event{Type: EventInterventionApplied, ConversationID: "conversation-1", RunID: "run-1"}
	if err := event.Validate(); err == nil {
		t.Fatal("expected missing round_id rejection")
	}
}

func TestEventRejectsUnsupportedEvaluationState(t *testing.T) {
	event := Event{
		Type:           EventSettled,
		ConversationID: "conversation-1",
		RunID:          "run-1",
		Payload:        Payload{Outcome: EvaluationState("magic")},
	}
	if err := event.Validate(); err == nil {
		t.Fatal("expected unsupported outcome rejection")
	}
}

func TestPayloadMapKeepsVersionedProjection(t *testing.T) {
	payload := Event{
		Type:           EventSettled,
		ConversationID: "conversation-1",
		RunID:          "run-1",
		ItemID:         "settlement-1",
		Payload: Payload{
			Outcome:       EvaluationAgentEvaluable,
			CheckpointRef: "commit-1",
		},
	}.Normalize().Payload
	row, err := PayloadMap(payload)
	if err != nil {
		t.Fatal(err)
	}
	if row["schema_version"] != SchemaVersion || row["trace_node_id"] != "settlement-1" || row["checkpoint_ref"] != "commit-1" {
		t.Fatalf("payload map = %#v", row)
	}
}

func TestBindTaskStateProjectsCanonicalTerminalSemantics(t *testing.T) {
	now := time.Now().UTC()
	contract := trajectoryTaskContract(taskstate.ContractImprovement, now)
	state, err := taskstate.New(contract, now)
	if err != nil {
		t.Fatal(err)
	}
	state, err = taskstate.Apply(contract, state, taskstate.TransitionRequest{
		Event: taskstate.EventNoCandidateReported, Reason: "bounded search complete",
		EvidenceRefs: []string{"observation://project"},
	}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	event, err := BindTaskState(Event{Type: EventSettled, ConversationID: contract.ConversationID, GoalID: contract.GoalID, RunID: contract.RunID}, contract, state)
	if err != nil {
		t.Fatal(err)
	}
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
	if event.Payload.TaskState != taskstate.StateNoCandidateFound || !event.Payload.Terminal || event.Payload.ContractID != contract.ContractID {
		t.Fatalf("canonical terminal projection = %#v", event.Payload)
	}
}

func TestBindTaskStateProjectsHumanJudgmentAsNonTerminal(t *testing.T) {
	now := time.Now().UTC()
	contract := trajectoryTaskContract(taskstate.ContractImprovement, now)
	state, err := taskstate.New(contract, now)
	if err != nil {
		t.Fatal(err)
	}
	state, err = taskstate.Apply(contract, state, taskstate.TransitionRequest{Event: taskstate.EventImprovementProposed, Reason: "candidate found", EvidenceRefs: []string{"observation://track"}, CandidateID: "track-1", Proposal: &taskstate.BoundedProposal{ProposalID: "proposal-1", Summary: "bounded adjustment", EvidenceRefs: []string{"observation://track"}, RequiresExperiment: true}}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	state, err = taskstate.Apply(contract, state, taskstate.TransitionRequest{Event: taskstate.EventExperimentRequired, Reason: "experiment admitted", ExperimentID: "experiment-1"}, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	state, err = taskstate.Apply(contract, state, taskstate.TransitionRequest{Event: taskstate.EventHumanJudgmentRequested, Reason: "audition required", ExperimentID: "experiment-1", PendingInteraction: &taskstate.PendingInteraction{InteractionID: "audition-1", Kind: "audition_judgment", Reason: "choose A or B"}}, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	event, err := BindTaskState(Event{Type: EventUserJudgmentRequested, ConversationID: contract.ConversationID, GoalID: contract.GoalID, RunID: contract.RunID}, contract, state)
	if err != nil {
		t.Fatal(err)
	}
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
	if event.Payload.TaskState != taskstate.StateHumanJudgmentRequired || event.Payload.Terminal || event.Payload.PendingInteraction == nil || event.Payload.ExperimentID != "experiment-1" {
		t.Fatalf("canonical human judgment projection = %#v", event.Payload)
	}
}

func trajectoryTaskContract(kind taskstate.ContractKind, now time.Time) taskstate.Contract {
	return taskstate.NormalizeContract(taskstate.Contract{
		ContractID: "contract-1", TaskID: "task-1", GoalID: "goal-1", RunID: "run-1", ConversationID: "conversation-1",
		OriginalIntent: "inspect and improve the project", Kind: kind, Scope: taskstate.Scope{Kind: "project"}, Temporary: true,
		TargetDiscovery: "agent_observation", AuthorizationBoundary: "governed_experiment",
		CompletionCriteria: []string{"governed outcome"}, EvidenceRequirements: []string{"observation reference"}, ProjectUUID: "project-1", ProjectRevision: "revision-1",
	}, now)
}
