package chat

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/orchestration"
)

func TestAttachMixboardDecisionProjectionRecordsB2B3B4C1TerminalSessions(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	capabilities := []string{staticBalanceCapabilityID, panLayoutCapabilityID, lowEndRelationCapabilityID, frequencyCleanupCapabilityID}
	for index, capabilityID := range capabilities {
		session := terminalProjectionSession(fmt.Sprintf("session-%d", index), capabilityID, time.Unix(1_700_000_000+int64(index), 0).UTC())
		response := ChatResponse{WorkflowData: map[string]any{}}
		attachMixboardDecisionProjection(&response, session)
		if response.WorkflowData["mixboard_decision_status"] != "recorded" {
			t.Fatalf("%s projection = %#v", capabilityID, response.WorkflowData)
		}
	}
	board, err := mixboard.NewStore(root).ReadProjectDecisionBoard("project-chat")
	if err != nil {
		t.Fatal(err)
	}
	if board.DecisionCount != len(capabilities) {
		t.Fatalf("decision count = %d, board=%#v", board.DecisionCount, board)
	}
}

func TestC1PluginLoadControlPlaneDefersMixboardDecisionUntilParameterBatch(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	session := terminalProjectionSession("c1-load", frequencyCleanupCapabilityID, time.Unix(1_700_000_000, 0).UTC())
	session.FrozenPlan.ActionSet.Actions[0].Command = c1LoadBatchCommand
	response := ChatResponse{WorkflowData: map[string]any{}}
	attachMixboardDecisionProjection(&response, session)
	if response.WorkflowData["mixboard_decision_status"] != "deferred_until_c1_parameter_batch" {
		t.Fatalf("load prerequisite became C1 decision: %+v", response.WorkflowData)
	}
	board, err := mixboard.NewStore(root).ReadProjectDecisionBoard("project-chat")
	if err != nil {
		t.Fatal(err)
	}
	if board.DecisionCount != 0 {
		t.Fatalf("C1 load prerequisite was persisted as mix decision: %+v", board)
	}
}

func TestC1ReadOnlySessionDoesNotPolluteMixboardDecisionLedger(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	session := terminalProjectionSession("c1-read-only", frequencyCleanupCapabilityID, time.Unix(1_700_000_000, 0).UTC())
	session.Status = orchestration.StatusCancelled
	session.FrozenPlan = nil
	session.Execution = nil
	response := ChatResponse{WorkflowData: map[string]any{}}
	attachMixboardDecisionProjection(&response, session)
	if response.WorkflowData["mixboard_decision_status"] != "read_only_or_no_action_not_recorded" {
		t.Fatalf("read-only C1 projection: %+v", response.WorkflowData)
	}
	board, err := mixboard.NewStore(root).ReadProjectDecisionBoard("project-chat")
	if err != nil {
		t.Fatal(err)
	}
	if board.DecisionCount != 0 {
		t.Fatalf("read-only C1 polluted decisions: %+v", board)
	}
}

func TestMixboardDecisionContextCarriesBoundedRefsAndFreshnessOnly(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	session := terminalProjectionSession("b2-context", staticBalanceCapabilityID, time.Unix(1_700_000_000, 0).UTC())
	if _, err := mixboard.NewStore(root).RecordCapabilitySession(session); err != nil {
		t.Fatal(err)
	}
	refs, err := mixboardDecisionContextForState(map[string]any{"project_uuid": "project-chat"}, panLayoutCapabilityID)
	if err != nil || len(refs) != 1 || refs[0].CapabilityID != staticBalanceCapabilityID {
		t.Fatalf("related refs=%#v err=%v", refs, err)
	}
	bundle := orchestration.ContextBundle{
		ID: "b3-pack", CapabilityID: panLayoutCapabilityID,
		ArtifactRefs: []string{"capability-pack:b3-pack"}, Disclosure: `{"candidate_plans":[]}`,
	}
	attachMixboardDecisionContext(&bundle, refs, nil)
	if len(bundle.ArtifactRefs) != 2 || bundle.ArtifactRefs[1] != "mixboard-decision:mixdec_b2-context" {
		t.Fatalf("artifact refs=%#v", bundle.ArtifactRefs)
	}
	disclosure := map[string]any{}
	if err := json.Unmarshal([]byte(bundle.Disclosure), &disclosure); err != nil {
		t.Fatal(err)
	}
	rows := mapRowsValue(disclosure["mixboard_decision_refs"])
	if len(rows) != 1 || firstStringFromMap(rows[0], "current_status") != mixboard.DecisionVerified {
		t.Fatalf("decision disclosure=%#v", disclosure)
	}
	if _, copied := rows[0]["verification"]; copied {
		t.Fatalf("context copied authoritative verification payload: %#v", rows[0])
	}
}

func TestAttachMixboardDecisionProjectionRecordsCancelledTerminal(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	session := terminalProjectionSession("b2-cancelled", staticBalanceCapabilityID, time.Unix(1_700_000_000, 0).UTC())
	session.Status = orchestration.StatusCancelled
	session.Execution = nil
	response := ChatResponse{WorkflowData: map[string]any{}}
	attachMixboardDecisionProjection(&response, session)
	if response.WorkflowData["mixboard_decision_current_status"] != mixboard.DecisionCancelled {
		t.Fatalf("cancelled projection=%#v", response.WorkflowData)
	}
}

func terminalProjectionSession(id, capabilityID string, completedAt time.Time) orchestration.PlanningSession {
	cut := orchestration.ProjectCut{ProjectUUID: "project-chat", ProjectEpoch: "epoch", Consistency: "strong", Hash: "cut-" + id}
	return orchestration.PlanningSession{
		ID: id, ProjectUUID: cut.ProjectUUID, EngineOwner: orchestration.EngineV1,
		Status: orchestration.StatusCompleted, Goal: "test " + capabilityID,
		Invocation: orchestration.CapabilityInvocation{CapabilityID: capabilityID, CapabilityVer: "v0"},
		FrozenPlan: &orchestration.FrozenPlan{
			Proposal:   orchestration.Proposal{ID: "proposal-" + id, Revision: 1, CapabilityID: capabilityID},
			ActionSet:  orchestration.ActionSet{ID: "actions-" + id, CapabilityID: capabilityID, Actions: []orchestration.Action{{ID: "action", TargetRef: "track"}}},
			ProjectCut: cut, PreviousObservationID: "before-" + id,
		},
		Execution: &orchestration.ExecutionRecord{
			ID: "execution-" + id, Status: "completed",
			Receipts:           []orchestration.ActionReceipt{{ActionID: "action", Status: "ok"}},
			VerificationResult: &orchestration.VerificationResult{Status: "pass", EvidenceRefs: []string{"mix.observe:after-" + id}},
		},
		CreatedAt: completedAt.Add(-time.Second), UpdatedAt: completedAt,
	}
}
