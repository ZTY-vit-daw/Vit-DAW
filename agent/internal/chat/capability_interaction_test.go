package chat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vit-daw-agent/internal/orchestration"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

func TestCapabilityRuntimeInteractionCancelReturnsToV1Owner(t *testing.T) {
	t.Setenv("VIT_ORCHESTRATION_STORE_PATH", "memory")
	server := New(nil, shadow.New(nil), nil)
	session, err := server.orchestrationRuntime.StartB2ChatSession("cap_v1_b2_chat-interaction_1", "chat-interaction", "p1", "B2", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	response := ChatResponse{
		ConversationID: "chat-interaction", GoalID: "goal", RunID: "run",
		Reply: "confirm", NeedsConfirmation: true, PlanID: "proposal-1",
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: map[string]any{"session_id": session.ID, "capability_id": staticBalanceCapabilityID},
	}
	server.attachInteractionRequests(&response)
	if len(response.InteractionRequests) != 1 {
		t.Fatalf("v1 proposal did not create confirmation interaction: %#v", response)
	}
	body, _ := json.Marshal(InteractionRespondRequest{
		InteractionID: response.InteractionRequests[0].ID, ActionID: "cancel", Decision: "cancel",
	})
	recorder := httptest.NewRecorder()
	server.handleInteractionRespond(recorder, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var result ChatResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.GoalStatus != string(agentruntime.StatusCancelled) || result.Workflow != "capability_runtime_v1" {
		t.Fatalf("interaction left v1 owner: %#v", result)
	}
	stored, _ := server.orchestrationRuntime.Store.Load(session.ID)
	if stored.Status != orchestration.StatusCancelled {
		t.Fatalf("cancel failed to close v1 Session: session=%#v", stored)
	}
}

func TestCapabilityRuntimeInteractionRejectAliasCancelsWithoutExecution(t *testing.T) {
	t.Setenv("VIT_ORCHESTRATION_STORE_PATH", "memory")
	server := New(nil, shadow.New(nil), nil)
	session, err := server.orchestrationRuntime.StartB2ChatSession("cap_v1_b2_chat-reject_1", "chat-reject", "p1", "B2", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	response := ChatResponse{
		ConversationID: "chat-reject", GoalID: "goal", RunID: "run",
		Reply: "confirm", NeedsConfirmation: true, PlanID: "proposal-1",
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: map[string]any{"session_id": session.ID, "capability_id": staticBalanceCapabilityID},
	}
	server.attachInteractionRequests(&response)
	body, _ := json.Marshal(InteractionRespondRequest{
		InteractionID: response.InteractionRequests[0].ID, ActionID: "reject", Decision: "reject",
	})
	recorder := httptest.NewRecorder()
	server.handleInteractionRespond(recorder, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	stored, _ := server.orchestrationRuntime.Store.Load(session.ID)
	if stored.Status != orchestration.StatusCancelled || stored.Authorization != nil || stored.Execution != nil {
		t.Fatalf("reject alias executed or failed to cancel: session=%#v", stored)
	}
}

func TestCapabilityRuntimeInteractionUnknownDecisionFailsClosed(t *testing.T) {
	t.Setenv("VIT_ORCHESTRATION_STORE_PATH", "memory")
	server := New(nil, shadow.New(nil), nil)
	session, err := server.orchestrationRuntime.StartB2ChatSession("cap_v1_b2_chat-invalid_1", "chat-invalid", "p1", "B2", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	response := ChatResponse{
		ConversationID: "chat-invalid", GoalID: "goal", RunID: "run",
		Reply: "confirm", NeedsConfirmation: true, PlanID: "proposal-1",
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: map[string]any{"session_id": session.ID, "capability_id": staticBalanceCapabilityID},
	}
	server.attachInteractionRequests(&response)
	interactionID := response.InteractionRequests[0].ID
	body, _ := json.Marshal(InteractionRespondRequest{
		InteractionID: interactionID, ActionID: "unexpected", Decision: "unexpected",
	})
	recorder := httptest.NewRecorder()
	server.handleInteractionRespond(recorder, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown decision status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	stored, _ := server.orchestrationRuntime.Store.Load(session.ID)
	if stored.Status != orchestration.StatusAnalyzing || stored.Authorization != nil || stored.Execution != nil {
		t.Fatalf("unknown decision changed session: %#v", stored)
	}
	if _, ok := server.takePendingInteraction(interactionID); !ok {
		t.Fatal("unknown decision consumed the pending interaction")
	}
}

func TestExpectedProposalMatchesRejectsStaleButtonRevision(t *testing.T) {
	proposal := &orchestration.Proposal{ID: "proposal-r2", Revision: 2, ActionSetHash: "actions-r2", ProjectCutHash: "cut-r2"}
	stale := map[string]any{
		"expected_proposal_id":       "proposal-r1",
		"expected_proposal_revision": 1,
		"expected_action_set_hash":   "actions-r1",
		"expected_project_cut_hash":  "cut-r1",
	}
	if expectedProposalMatches(stale, proposal) {
		t.Fatal("stale button expectation matched the current proposal")
	}
	current := map[string]any{
		"expected_proposal_id":       proposal.ID,
		"expected_proposal_revision": proposal.Revision,
		"expected_action_set_hash":   proposal.ActionSetHash,
		"expected_project_cut_hash":  proposal.ProjectCutHash,
	}
	if !expectedProposalMatches(current, proposal) {
		t.Fatal("current exact proposal binding was rejected")
	}
}

func TestCapabilityProposalInteractionCarriesPresentationWithoutRuntime(t *testing.T) {
	presentation := &orchestration.ProposalPresentation{
		SchemaVersion: orchestration.ProposalPresentationSchema,
		ProposalID:    "proposal-1", ProposalRevision: 1, Title: "B2 静态平衡方案", Conclusion: "建议执行两项电平调整。", ActionCount: 2,
	}
	response := ChatResponse{
		ConversationID: "conversation", PlanID: presentation.ProposalID, Reply: presentation.Conclusion,
		NeedsConfirmation: true, Workflow: "capability_runtime_v1", ProposalPresentation: presentation,
		WorkflowData: map[string]any{
			"session_id": "session", "proposal_id": presentation.ProposalID, "proposal_revision": presentation.ProposalRevision,
			"action_set_hash": "actions", "project_cut_hash": "cut", "proposal_presentation": presentation,
		},
	}
	request := (&Server{interactions: make(map[string]PendingInteraction)}).confirmationInteractionRequest(response)
	if request.Kind != "proposal_approval" || request.Stage != "proposal_review" {
		t.Fatalf("unexpected interaction kind/stage: %#v", request)
	}
	got, ok := request.Payload["proposal_presentation"].(*orchestration.ProposalPresentation)
	if !ok || got.ProposalID != presentation.ProposalID || request.Payload["action_set_hash"] != "actions" {
		t.Fatalf("proposal presentation or binding was lost: %#v", request.Payload)
	}
}

func TestCapabilityProposalInteractionRecoversOnlyExactDurableBinding(t *testing.T) {
	t.Setenv("VIT_ORCHESTRATION_STORE_PATH", "memory")
	server := New(nil, shadow.New(nil), nil)
	const capabilityID = "static_mix.static_balance.v0"
	cut := orchestration.ProjectCut{ProjectUUID: "project", ProjectEpoch: "epoch", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	actionSet := orchestration.ActionSet{ID: "actions", CapabilityID: capabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{{
		ID: "gain", Command: "track_gain_adjust", TargetRef: "track-1", Args: map[string]any{"delta_db": 0.2}, Compensatable: true,
	}}}
	actionSet.Hash = actionSet.ComputeHash()
	proposal := orchestration.Proposal{
		ID: "proposal-1", Revision: 1, CapabilityID: capabilityID, ProjectCutHash: cut.Hash, ActionSetHash: actionSet.Hash,
		Presentation: &orchestration.ProposalPresentation{SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: "proposal-1", ProposalRevision: 1, CapabilityID: capabilityID, ActionCount: 1},
	}
	session, err := orchestration.NewSession("session-recovery", "project", "B2", orchestration.EngineV1, orchestration.CapabilityInvocation{
		SessionID: "session-recovery", ConversationID: "conversation-recovery", CapabilityID: capabilityID,
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SetFrozenPlan(orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.orchestrationRuntime.Store.Create(session); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"workflow": "capability_runtime_v1", "approval_mode": "conversational", "conversation_id": "conversation-recovery",
		"session_id": session.ID, "capability_id": capabilityID, "proposal_id": proposal.ID, "proposal_revision": proposal.Revision,
		"action_set_hash": proposal.ActionSetHash, "project_cut_hash": proposal.ProjectCutHash,
	}
	recovered, ok := server.recoverCapabilityRuntimeInteractionFromPayload("interaction-recovery", payload)
	if !ok || recovered.PlanID != proposal.ID || recovered.ConversationID != "conversation-recovery" || recovered.Kind != "proposal_approval" {
		t.Fatalf("exact durable interaction was not recovered: ok=%v interaction=%#v", ok, recovered)
	}
	stale := copyStringAnyMap(payload)
	stale["proposal_revision"] = 2
	if _, ok := server.recoverCapabilityRuntimeInteractionFromPayload("interaction-stale", stale); ok {
		t.Fatal("stale proposal revision recovered an interaction")
	}
	forged := copyStringAnyMap(payload)
	forged["action_set_hash"] = "forged"
	if _, ok := server.recoverCapabilityRuntimeInteractionFromPayload("interaction-forged", forged); ok {
		t.Fatal("forged action set hash recovered an interaction")
	}
}
