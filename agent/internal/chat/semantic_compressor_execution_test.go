package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/com"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/semanticeffect"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func semanticCompressorExecutionFixture(t *testing.T) (semanticeffect.CompressorPlan, plugingrabber.ParameterDigest, map[string]any, map[string]any) {
	t.Helper()
	summary, _ := testCompressorSummaryForApply(t)
	summary["classification"] = "threshold_driven"
	digest := plugingrabber.ParameterDigest{TrackID: "1007", PluginID: "1013", PluginName: "Test Compressor",
		ParameterCount: 1, Parameters: []plugingrabber.ParameterInfo{{ID: "1", Name: "Threshold", NormalizedValue: .5, ValueText: "-30.0 dB"}}}
	target := -18.0
	plan := semanticeffect.CompressorPlan{SchemaVersion: semanticeffect.CompressorPlanSchema, PlanningOnly: true,
		MutationAuthorized: false, Target: semanticeffect.Target{TrackID: "1007", PluginID: "1013"}, UserGoal: "compress more",
		IdentityCardID: "card-1", TopologyGeneration: "c2t1_apply", SelectedAxes: []string{"activation_intensity"},
		Evidence: semanticeffect.CompressorPlanEvidence{COMProjectionID: "com-before", COMMode: com.ModePairedIO,
			COMStatus: com.StatusReady, Summary: "paired compressor behavior"},
		Controls: []semanticeffect.CompressorControlProposal{{ProposalID: "threshold-1", Axis: "activation_intensity",
			PathKey: "main", Role: "threshold", Target: semanticeffect.CompressorControlTarget{ValueDB: &target},
			Purpose: "increase bounded gain action", Confidence: "medium"}},
		EvaluationContract: semanticeffect.CompressorEvaluationContract{SchemaVersion: "semantic_effect.compressor_evaluation_contract.v1",
			FrozenUserGoal: "compress more", Axes: []semanticeffect.CompressorAxisEvaluation{{Axis: "activation_intensity",
				DesiredDirection: "more bounded action", COMDimensions: []string{"gain_action"}, AcceptanceCondition: "change_delta is bounded"}},
			LevelMatchRequired: true, SuccessPolicy: "bounded_com_change_delta_plus_user_acceptance", NoAutoIteration: true},
		Revision: semanticeffect.CompressorRevision{SchemaVersion: semanticeffect.CompressorRevisionSchema, MaxAttempts: 1}}
	comContext := map[string]any{"mode": com.ModePairedIO, "status": com.StatusReady, "projection_id": "com-before"}
	return plan, digest, summary, comContext
}

func semanticCompressorInteraction(ticket compressorExecutionTicket) PendingInteraction {
	return PendingInteraction{ID: "interaction-1", CreatedAt: time.Now(), Kind: "confirmation",
		Workflow: semanticCompressorExecutionWorkflow, Stage: "waiting_for_user", PlanID: ticket.TicketID,
		ConversationID: ticket.ConversationID, GoalID: ticket.GoalID, RunID: ticket.RunID,
		Data: map[string]any{"ticket": compressorExecutionTicketMap(ticket), "workflow_data": map[string]any{
			"status": "waiting_confirmation", "mutation_authorized": false, "mutation_performed": false}}}
}

func TestMaterializeSemanticCompressorPlanBindsLocallyWithoutPreviewLeak(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	ticket, preview, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", map[string]any{"clip_id": "2001", "start_sample": 0, "end_sample": 48000})
	if err != nil {
		t.Fatal(err)
	}
	if ticket.SchemaVersion != semanticCompressorExecutionSchema || ticket.TopologyClass != "threshold_driven" || len(ticket.Controls) != 1 {
		t.Fatalf("ticket = %+v", ticket)
	}
	if ticket.Controls[0].ControlRef == "" {
		t.Fatal("local control reference was not materialized")
	}
	for _, forbidden := range []string{"control_ref", "c2cr1_", "param_id", "normalized"} {
		if strings.Contains(strings.ToLower(preview), forbidden) {
			t.Fatalf("preview leaked %q: %s", forbidden, preview)
		}
	}
}

func TestMaterializeSemanticCompressorPlanRequiresDeterministicPairedEvidence(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	comContext["mode"], comContext["status"] = com.ModeSourceOnly, com.StatusReady
	if _, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil); err == nil || !strings.Contains(err.Error(), "paired_io / ready") {
		t.Fatalf("source-only execution was not rejected: %v", err)
	}
	comContext["mode"], comContext["projection_id"] = com.ModePairedIO, "other-projection"
	if _, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("fabricated plan evidence was not rejected: %v", err)
	}
}

func TestMaterializeSemanticCompressorPlanAllowsC2SourceOnlyReversibleEvidence(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	comContext["mode"], comContext["status"] = com.ModeSourceOnly, com.StatusPartial
	plan.Evidence.COMMode, plan.Evidence.COMStatus = com.ModeSourceOnly, com.StatusPartial
	requestContext := map[string]any{"c2_source_only_reversible": true}
	if _, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", requestContext); err != nil {
		t.Fatalf("C2 source-only reversible evidence was rejected: %v", err)
	}
}

func TestMaterializeSemanticCompressorPlanAllowsC2PairedPartialWhenSourceFallbackIsWorse(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	comContext["status"] = com.StatusPartial
	plan.Evidence.COMStatus = com.StatusPartial
	requestContext := map[string]any{"c2_source_only_reversible": true}
	if _, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", requestContext); err != nil {
		t.Fatalf("C2 paired partial reversible evidence was rejected: %v", err)
	}
	if _, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil); err == nil {
		t.Fatal("ordinary compressor execution accepted paired partial evidence")
	}
}

func TestMaterializeSemanticCompressorPlanRejectsAmbiguousBinding(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	stage := mapValue(summary["compressor_stage"])
	paths := mapRowsValue(stage["control_paths"])
	path := paths[0]
	operatingPoint := mapRowsValue(path["operating_point"])
	ambiguous := cloneStringAnyMap(operatingPoint[0])
	ambiguous["domain"] = map[string]any{"min": -60.0, "max": 0.0}
	path["operating_point"] = append(operatingPoint, ambiguous)
	if _, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("ambiguous binding was accepted: %v", err)
	}
}

func TestMaterializeSemanticCompressorPlanSelectsStableEquivalentBinding(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	stage := mapValue(summary["compressor_stage"])
	paths := mapRowsValue(stage["control_paths"])
	path := paths[0]
	operatingPoint := mapRowsValue(path["operating_point"])
	path["operating_point"] = append(operatingPoint, cloneStringAnyMap(operatingPoint[0]))
	if _, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil); err != nil {
		t.Fatalf("equivalent live bindings were rejected: %v", err)
	}
}

func TestSemanticCompressorBindingPrefersUniqueContinuousInputDrive(t *testing.T) {
	summary := map[string]any{"compressor_stage": map[string]any{"control_paths": []any{
		map[string]any{"path_key": "main", "operating_point": []any{
			map[string]any{"role": "input_drive", "current_text": "Off", "reachable_values": []any{
				map[string]any{"label": "Off"}, map[string]any{"label": "On"},
			}},
			map[string]any{"role": "input_drive", "current_text": "4.0", "domain": map[string]any{"min": 0.0, "max": 10.0}},
		}},
	}}}
	binding, err := semanticCompressorBinding(summary, "main", "input_drive")
	if err != nil || firstNonEmptyText(binding, "current_text") != "4.0" {
		t.Fatalf("continuous input drive was not selected: binding=%+v err=%v", binding, err)
	}
}

func TestSemanticCompressorWaitingResponseKeepsTicketServerSide(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	ticket, preview, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", map[string]any{"clip_id": "2001", "start_sample": 0, "end_sample": 48000})
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{interactions: map[string]PendingInteraction{}}
	requestContext := map[string]any{"goal_id": "goal-1", "run_id": "run-1", "free_state_reasoning_loop": map[string]any{
		"schema_version": freeStateReasoningLoopSchema, "status": "awaiting_action", "original_intent": "make the vocal steadier",
	}}
	resp := server.semanticCompressorWaitingResponse("conversation-1", requestContext,
		semanticeffect.AudioProcessorIdentityCard{}, semanticeffect.CompressorIntentPlan{}, comContext,
		plugingrabber.CompressorControlBrief{}, plan, ticket, preview)
	if !resp.NeedsConfirmation || resp.GoalStatus != "waiting_confirmation" || len(resp.InteractionRequests) != 1 || len(server.interactions) != 1 {
		t.Fatalf("waiting response = %+v interactions=%+v", resp, server.interactions)
	}
	if loop := mapValue(resp.WorkflowData["free_state_reasoning_loop"]); loop["original_intent"] != "make the vocal steadier" {
		t.Fatalf("waiting response lost free-state loop: %+v", resp.WorkflowData)
	}
	visible, _ := json.Marshal(resp)
	if strings.Contains(string(visible), ticket.Controls[0].ControlRef) || strings.Contains(string(visible), "\"control_ref\"") {
		t.Fatalf("visible confirmation leaked internal materialization: %s", visible)
	}
	stored := server.interactions[resp.InteractionRequests[0].ID]
	storedTicket, decodeErr := compressorExecutionTicketFromMap(stored.Data["ticket"])
	if decodeErr != nil || storedTicket.Controls[0].ControlRef == "" {
		t.Fatalf("server-side ticket missing: %+v err=%v", storedTicket, decodeErr)
	}
}

func TestSemanticCompressorPlanningFailureKeepsFreeStateIntent(t *testing.T) {
	requestContext := map[string]any{
		"goal_id": "goal-1", "run_id": "run-1",
		"free_state_reasoning_loop": map[string]any{
			"schema_version": freeStateReasoningLoopSchema, "status": "awaiting_action", "original_intent": "make the vocal steadier",
		},
	}
	resp := semanticCompressorPlanningFailure("conversation-1", requestContext, "control_planning_failed", context.DeadlineExceeded)
	loop := mapValue(resp.WorkflowData["free_state_reasoning_loop"])
	if loop["original_intent"] != "make the vocal steadier" || mapValue(resp.WorkflowData["request_context"])["goal_id"] != "goal-1" {
		t.Fatalf("planning failure lost free-state context: %+v", resp.WorkflowData)
	}
	if resp.WorkflowData["mutation_performed"] != false || resp.StopReason != "control_planning_failed" ||
		resp.GoalStatus != string(agentruntime.StatusFailed) || resp.Error == "" {
		t.Fatalf("planning failure boundary changed: %+v", resp)
	}
}

func TestSemanticCompressorExecutionFreshnessRejectsBaselineAndTopologyChanges(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	ticket, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSemanticCompressorExecutionFreshness(ticket, digest, summary); err != nil {
		t.Fatal(err)
	}
	changed := digest
	changed.Parameters = append([]plugingrabber.ParameterInfo(nil), digest.Parameters...)
	changed.Parameters[0].NormalizedValue = .6
	if err := validateSemanticCompressorExecutionFreshness(ticket, changed, summary); err == nil || !strings.Contains(err.Error(), "baseline") {
		t.Fatalf("changed baseline was accepted: %v", err)
	}
	otherSummary := cloneStringAnyMap(summary)
	otherSummary["classification"] = "amount_driven"
	if err := validateSemanticCompressorExecutionFreshness(ticket, digest, otherSummary); err == nil || !strings.Contains(err.Error(), "topology") {
		t.Fatalf("changed topology was accepted: %v", err)
	}
}

func TestSemanticCompressorExecutionTicketExpiresAndIsInteractionScoped(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	ticket, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil)
	if err != nil {
		t.Fatal(err)
	}
	interaction := semanticCompressorInteraction(ticket)
	createdAt, _ := time.Parse(time.RFC3339Nano, ticket.CreatedAt)
	if err := validateSemanticCompressorExecutionTicket(ticket, interaction, createdAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := validateSemanticCompressorExecutionTicket(ticket, interaction, createdAt.Add(semanticCompressorExecutionTTL)); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired ticket was accepted: %v", err)
	}
	interaction.PlanID = "another-ticket"
	if err := validateSemanticCompressorExecutionTicket(ticket, interaction, createdAt.Add(time.Minute)); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("mismatched interaction was accepted: %v", err)
	}
}

func TestSemanticCompressorCancelPerformsNoWrite(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	ticket, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp := (&Server{}).continueSemanticCompressorExecutionInteraction(context.Background(), semanticCompressorInteraction(ticket), "cancel")
	if resp.GoalStatus != "cancelled" || resp.WorkflowData["mutation_performed"] != false || resp.WorkflowData["mutation_authorized"] != false {
		t.Fatalf("cancel response = %+v", resp)
	}
}

func TestCompressorExecutionParameterAuditAllowsTargetsAndRejectsNonTargets(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	digest.ParameterCount = 2
	digest.Parameters = append(digest.Parameters, plugingrabber.ParameterInfo{ID: "2", Name: "Output", NormalizedValue: .5, ValueText: "0 dB"})
	ticket, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil)
	if err != nil {
		t.Fatal(err)
	}
	after := digest
	after.Parameters = append([]plugingrabber.ParameterInfo(nil), digest.Parameters...)
	after.Parameters[0].NormalizedValue = .7
	if audit, err := compressorExecutionParameterAudit(digest, after, ticket); err != nil || audit["non_target_parameters_unchanged"] != true {
		t.Fatalf("target-only audit = %+v err=%v", audit, err)
	}
	after.Parameters[1].NormalizedValue = .6
	if _, err := compressorExecutionParameterAudit(digest, after, ticket); err == nil || !strings.Contains(err.Error(), "non-target") {
		t.Fatalf("non-target mutation was accepted: %v", err)
	}
}

func TestValidateCompressorExecutionResultRequiresCompleteReadback(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	ticket, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]any{"status": "exact", "atomic": true, "track_id": ticket.TrackID, "plugin_id": ticket.PluginID,
		"topology_generation": ticket.TopologyGeneration, "restore_ref": "restore-1",
		"controls": []map[string]any{{"status": "exact", "actual_readback": []map[string]any{{"param_id": "1", "normalized": .7}}}}}
	if err := validateCompressorExecutionResult(result, ticket); err != nil {
		t.Fatal(err)
	}
	mapRowsValue(result["controls"])[0]["actual_readback"] = []map[string]any{}
	if err := validateCompressorExecutionResult(result, ticket); err == nil || !strings.Contains(err.Error(), "readback") {
		t.Fatalf("missing target readback was accepted: %v", err)
	}
}

func TestCompressorExecutionRollbackReceiptReportsNoRetainedMutation(t *testing.T) {
	ticket := compressorExecutionTicket{TicketID: "ticket-1"}
	interaction := PendingInteraction{ConversationID: "conversation-1", GoalID: "goal-1", RunID: "run-1"}
	applyResult := map[string]any{"status": "exact"}
	receipt := map[string]any{"rollback": map[string]any{"status": "restored", "verified": true}}
	resp := compressorExecutionFailureResponse(interaction, ticket, "parameter_audit_failed", context.Canceled, applyResult, receipt)
	if resp.WorkflowData["mutation_performed"] != false || mapValue(resp.WorkflowData["execution_receipt"])["rollback"] == nil {
		t.Fatalf("rollback failure response = %+v", resp)
	}
}
