package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/semanticeffect"
	"vit-daw-agent/internal/shadow"
)

func semanticEQTestAction(shape string, frequency, gain float64) semanticeffect.Action {
	return semanticeffect.Action{
		SchemaVersion: semanticeffect.ActionSchema, ActionType: semanticeffect.ActionEQEdit, PayloadSchema: semanticeffect.EQPlanSchema,
		Target:   semanticeffect.Target{TrackID: "track-1", PluginID: "eq-1", TrackName: "Vocal", PluginName: "Test EQ"},
		UserGoal: "减少一些浑浊", Constraints: []string{"不要让主体变薄"},
		Evidence: semanticeffect.EvidenceDecision{Choice: "not_needed", Basis: "user_report", Reason: "用户已经明确报告听感问题"},
		EQPlan: &semanticeffect.EQPlan{SchemaVersion: semanticeffect.EQPlanSchema, Atomic: true, Atoms: []semanticeffect.EQAtom{{
			AtomID: "mud-cut", Action: "upsert", Shape: shape, FrequencyHz: &frequency, GainDB: &gain,
			Purpose: "降低低中频堆积", Confidence: "medium",
			FieldOrigins: map[string]string{"frequency_hz": "llm_selected", "gain_db": "llm_selected"},
		}}},
	}
}

func TestSemanticEQPCAInputDoesNotInventMissingAction(t *testing.T) {
	action := semanticEQTestAction("bell", 3400, -3)
	action.EQPlan.Atoms[0].Action = ""
	if _, err := semanticEQPCAInputFromAction(action); err == nil || !strings.Contains(err.Error(), "missing its action") {
		t.Fatalf("missing action was converted into PCA coverage: %v", err)
	}
}

func TestSemanticEQReadOnlyMaterializationUsesExistingPlannerWithoutWriting(t *testing.T) {
	fake := newFakeEQKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	action := semanticEQTestAction("bell", 3400, -3)
	planned, err := server.planSemanticEQReadOnly(context.Background(), action)
	if err != nil {
		t.Fatal(err)
	}
	if planned.DigestGeneration == "" || len(planned.PlannedEdits) != 1 || len(planned.PlannedWrites) == 0 || len(planned.Preimage) == 0 {
		t.Fatalf("incomplete read-only materialization: %#v", planned)
	}
	if len(fake.batchCalls) != 0 {
		t.Fatalf("read-only proposal materialization wrote %d parameter batches", len(fake.batchCalls))
	}
	if got := firstStringFromMap(planned.PreviewResults[0], "status"); got != "exact" && got != "quantized" {
		t.Fatalf("preview status = %q", got)
	}
}

func TestSemanticEQSelectedTargetChecksTrackAndPluginTrackIndependently(t *testing.T) {
	target := semanticeffect.Target{TrackID: "track-plugin", PluginID: "eq-1"}
	mismatch := semanticEQSelectedTargetMismatch(map[string]any{
		"selected_track_id": "track-current", "selected_plugin_track_id": "track-plugin", "selected_plugin_id": "eq-1",
	}, target)
	if !strings.Contains(mismatch, "track-current") {
		t.Fatalf("current selected track mismatch was hidden by plugin track: %q", mismatch)
	}
}

func TestEnsureOrdinaryAgentSemanticEQRepairsUnexecutableShape(t *testing.T) {
	repairedAction := semanticEQTestAction("bell", 8000, 1.0)
	repairedAction.Evidence = semanticeffect.EvidenceDecision{Choice: "observe", Basis: "observation", Reason: "CCB 观察支持该计划", ObservationID: "obs-eq-test"}
	repairedJSON, err := json.Marshal(repairedAction)
	if err != nil {
		t.Fatal(err)
	}
	server, cfg, calls, bodies := semanticEQPlannerTestServer(t, []string{string(repairedJSON)})
	server.eqKernelOverride = newFakeEQKernel()
	requestContext := server.agentLoopContextWithGenericEQTopology(context.Background(), "提高一些高频", map[string]any{
		"selected_track_id": "track-1", "selected_plugin_track_id": "track-1", "selected_plugin_id": "eq-1",
		"semantic_eq_topology_authorized": true,
	})
	candidate := semanticEQTestAction("high_shelf", 8000, 1.0)
	candidate.Evidence = repairedAction.Evidence
	planned, err := server.ensureOrdinaryAgentSemanticEQExecutable(context.Background(), "conversation-1", "提高一些高频", requestContext, semanticEQPlannerTestObservation(), cfg, &candidate)
	if err != nil {
		t.Fatalf("repair failed: %v", err)
	}
	if *calls != 1 || planned == nil || planned.EQPlan.Atoms[0].Shape != "bell" {
		t.Fatalf("planned=%#v calls=%d", planned, *calls)
	}
	if len(*bodies) != 1 || !strings.Contains((*bodies)[0], "deterministic_materialization_rejection") || !strings.Contains((*bodies)[0], "actions.upsert=true") {
		t.Fatalf("repair prompt omitted deterministic constraint: %#v", *bodies)
	}
}

func TestAgentLoopPrefetchesCompactGenericEQTopologyWithoutWriting(t *testing.T) {
	fake := newFakeEQKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	contextWithTopology := server.agentLoopContextWithGenericEQTopology(context.Background(), "提高一些高频", map[string]any{
		"selected_track_id": "track-1", "selected_plugin_track_id": "track-1", "selected_plugin_id": "eq-1",
		"semantic_eq_topology_authorized": true,
	})
	topology := firstMapFromAny(contextWithTopology["generic_eq_topology"])
	if firstStringFromMap(topology, "topology_generation") == "" || len(mapRowsValue(topology["sections"])) == 0 {
		t.Fatalf("compact topology missing: %#v", topology)
	}
	if len(fake.batchCalls) != 0 {
		t.Fatalf("topology prefetch wrote %d batches", len(fake.batchCalls))
	}
}

func TestSemanticEQUnsupportedPlanRejectsBeforeAnyWrite(t *testing.T) {
	fake := newFakeEQKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	action := semanticEQTestAction("high_shelf", 8000, 2)
	_, err := server.planSemanticEQReadOnly(context.Background(), action)
	if err == nil || eqControlFailureCode(err) != "shape_not_provably_reachable" {
		t.Fatalf("unsupported shelf err=%v code=%s", err, eqControlFailureCode(err))
	}
	if len(fake.batchCalls) != 0 {
		t.Fatal("rejected semantic plan wrote parameters")
	}
}

func TestSemanticEQMutationPortPreservesExactAtomResultsAndReadback(t *testing.T) {
	fake := newFakeEQKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	action := semanticEQTestAction("bell", 3400, -3)
	planned, err := server.planSemanticEQReadOnly(context.Background(), action)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := (&semanticEQMutationPort{server: server}).Apply(context.Background(), orchestration.Action{
		ID: "eq-action", Args: map[string]any{
			"semantic_action": action, "track_id": "track-1", "plugin_id": "eq-1",
			"edits": planned.Edits, "planned_writes": planned.PlannedWrites, "parameter_preimage": planned.Preimage,
		},
	}, "exec-test")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "applied" || firstStringFromMap(receipt.Details, "structural_readback") != "pass" {
		t.Fatalf("receipt=%#v", receipt)
	}
	atoms := mapRowsValue(receipt.Details["atom_results"])
	if len(atoms) != 1 || firstStringFromMap(atoms[0], "atom_id") != "mud-cut" || len(mapRowsValue(atoms[0]["actual_readback"])) == 0 {
		t.Fatalf("atom results=%#v", atoms)
	}
	if firstStringFromMap(firstMapFromAny(receipt.Details["result"]), "operation_ref") == "" {
		t.Fatalf("operation_ref missing: %#v", receipt.Details)
	}
}

func TestSemanticEQReceiptTextReportsQuantizedActualReadback(t *testing.T) {
	receipt := orchestration.ActionReceipt{Status: "applied", Details: map[string]any{
		"result": map[string]any{"status": "quantized", "operation_ref": "eqop-1"},
		"atom_results": []map[string]any{{
			"atom_id": "air", "status": "quantized", "shape": "high_shelf",
			"requested":       map[string]any{"frequency_hz": 9100.0, "gain_db": 1.2, "slope_db_per_oct": 12.0},
			"actual_readback": []map[string]any{{"role": "freq", "physical": 9000.0}, {"role": "gain", "physical": 1.0}},
		}},
	}}
	text := semanticEQReceiptText(receipt, &orchestration.VerificationResult{
		Structural: "pass", Acoustic: "unavailable", UserAcceptance: "unknown",
	})
	for _, required := range []string{"quantized", "requested 9100.0 Hz", "slope 12.0 dB/oct", "actual freq=9000", "operation_ref：eqop-1", "acoustic=unavailable"} {
		if !strings.Contains(text, required) {
			t.Fatalf("receipt text omitted %q: %s", required, text)
		}
	}
}

func TestSemanticEQReceiptTextReportsRollbackCapabilityMetadata(t *testing.T) {
	text := semanticEQReceiptText(orchestration.ActionReceipt{Status: "applied", Details: map[string]any{
		"result": map[string]any{
			"status": "exact",
			"rollback": map[string]any{
				"available":          true,
				"operation_ref":      "eqop-rollback-1",
				"lifetime":           "process",
				"expires_on_restart": true,
			},
		},
	}}, nil)
	for _, required := range []string{
		"rollback：available=true",
		"operation_ref=eqop-rollback-1",
		"lifetime=process",
		"expires_on_restart=true",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("rollback capability text omitted %q: %s", required, text)
		}
	}
	if strings.Contains(text, "rollback：unknown") {
		t.Fatalf("rollback capability was incorrectly reported as unknown: %s", text)
	}
}

func TestSemanticEQExecutionResponsePreservesRejectedDetails(t *testing.T) {
	session := orchestration.PlanningSession{ID: "session-1", Execution: &orchestration.ExecutionRecord{
		ID: "execution-1", Status: "failed", Receipts: []orchestration.ActionReceipt{{
			Status: "failed", Error: "shape_not_provably_reachable", Details: map[string]any{
				"status": "rejected",
				"atom_results": []map[string]any{{
					"atom_id": "air", "status": "rejected", "rejection_code": "shape_not_provably_reachable",
					"actual_readback": []map[string]any{{"role": "shape", "value_text": "Bell"}},
				}},
				"rollback": map[string]any{"attempted": false, "status": "not_needed_no_write"},
			},
		}},
	}}
	response := semanticEQExecutionResponse("conversation-1", agentruntime.Goal{}, session, errors.New("shape_not_provably_reachable"))
	details := firstMapFromAny(response.WorkflowData["result"])
	atoms := mapRowsValue(details["atom_results"])
	if firstStringFromMap(details, "status") != "rejected" || len(atoms) != 1 || firstStringFromMap(atoms[0], "rejection_code") != "shape_not_provably_reachable" {
		t.Fatalf("rejected details were not preserved: response=%#v", response)
	}
	if len(mapRowsValue(atoms[0]["actual_readback"])) != 1 || firstStringFromMap(firstMapFromAny(details["rollback"]), "status") != "not_needed_no_write" {
		t.Fatalf("rejected readback/rollback limitations missing: %#v", details)
	}
	for _, required := range []string{"EQ 执行结果：rejected", "rejection=shape_not_provably_reachable", "actual shape=Bell", "rollback：not_needed_no_write", "error：shape_not_provably_reachable"} {
		if !strings.Contains(response.Reply, required) {
			t.Fatalf("rejected reply omitted %q: %s", required, response.Reply)
		}
	}
}

func TestSemanticEQReceiptNeverDefaultsMissingStatusToExact(t *testing.T) {
	text := semanticEQReceiptText(orchestration.ActionReceipt{Status: "applied", Details: map[string]any{
		"atom_results": []map[string]any{{"atom_id": "air", "shape": "high_shelf"}},
	}}, nil)
	if strings.Contains(text, "EQ 已执行：exact") || !strings.Contains(text, "EQ 已执行：applied") || !strings.Contains(text, "不能推定为 exact") || !strings.Contains(text, "actual unavailable") {
		t.Fatalf("missing receipt status was not reported conservatively: %s", text)
	}
}

func TestSemanticEQPluginSelectionRequiredResponseClearsLegacyTreatment(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	res := agentloop.Result{
		GoalID: "goal-1", RunID: "run-1", Status: agentruntime.StatusCompleted,
		ExecutionMemory: agentloop.ExecutionMemory{PendingMixTreatment: &agentloop.MixTreatmentPending{
			SchemaVersion: "mix_treatment_pending.v0", Status: "pending_confirmation", ActionKind: "plugin_treatment", ProcessorType: "eq",
		}},
		RecentObservation: &agentloop.RecentObservation{Summary: map[string]any{"observation_id": "obs-1"}},
	}
	resp := server.semanticEQPluginSelectionRequiredResponse("conversation-1", agentModeDefault, "帮我把低频降低一些", map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Bass",
	}, res)
	if resp.NeedsConfirmation || resp.Workflow != "plugin_selection_required" || resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("response=%#v", resp)
	}
	if firstStringFromMap(resp.WorkflowData, "schema_version") != "plugin_selection_required.v1" || boolValue(resp.WorkflowData["mutation_performed"]) {
		t.Fatalf("workflow_data=%#v", resp.WorkflowData)
	}
	target := firstMapFromAny(resp.WorkflowData["target_ref"])
	if firstStringFromMap(target, "id") != "1007" || firstStringFromMap(resp.WorkflowData, "observation_id") != "obs-1" {
		t.Fatalf("target/evidence handoff=%#v", resp.WorkflowData)
	}
	if _, ok := server.pendingTreatments["conversation-1"]; ok {
		t.Fatal("legacy pending treatment survived plugin-selection handoff")
	}
}

type semanticEQTestStateReader struct{ state *kernel.VSPStateResult }

func (r semanticEQTestStateReader) VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error) {
	return r.state, nil
}

func semanticEQTestFrozenPlan(t *testing.T, server *Server, action semanticeffect.Action, revision int64) (orchestration.ActionSet, orchestration.ProjectCut) {
	t.Helper()
	planned, err := server.planSemanticEQReadOnly(context.Background(), action)
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "project", ProjectEpoch: "epoch", BaseProjectRevision: "7", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	_, actionSet, err := capabilityadapters.FreezeSemanticEQ(capabilityadapters.SemanticEQPlan{
		Action: action, TopologyGeneration: planned.DigestGeneration, Edits: planned.Edits,
		PlannedEdits: planned.PlannedEdits, PlannedWrites: planned.PlannedWrites,
		ParameterPreimage: planned.Preimage, ParameterSnapshot: planned.Snapshot, PreviewResults: planned.PreviewResults,
		AcousticVerification: "unavailable",
	}, cut, revision)
	if err != nil {
		t.Fatal(err)
	}
	return actionSet, cut
}

func TestSemanticEQPreflightRejectsStaleProjectAndPreimageWithoutWriting(t *testing.T) {
	t.Run("stale project revision", func(t *testing.T) {
		fake := newFakeEQKernel()
		server := New(nil, shadow.New(nil), nil)
		server.eqKernelOverride = fake
		actionSet, cut := semanticEQTestFrozenPlan(t, server, semanticEQTestAction("bell", 3400, -3), 1)
		port := &semanticEQMutationPort{server: server, stateReader: semanticEQTestStateReader{state: &kernel.VSPStateResult{
			Response: map[string]any{"type": "state.snapshot"}, LegacyState: map[string]any{"project_uuid": "project"},
			ProjectEpoch: "epoch", Revision: 8, SnapshotHash: "changed",
		}}}
		err := port.Preflight(context.Background(), actionSet, cut)
		if err == nil || !strings.Contains(err.Error(), "stale_project_cut") {
			t.Fatalf("stale preflight err=%v", err)
		}
		if len(fake.batchCalls) != 0 {
			t.Fatal("stale preflight wrote parameters")
		}
	})

	t.Run("parameter preimage drift", func(t *testing.T) {
		fake := newFakeEQKernel()
		server := New(nil, shadow.New(nil), nil)
		server.eqKernelOverride = fake
		actionSet, cut := semanticEQTestFrozenPlan(t, server, semanticEQTestAction("bell", 3400, -3), 1)
		fake.params["b1g"] = 0.9
		port := &semanticEQMutationPort{server: server, stateReader: semanticEQTestStateReader{state: &kernel.VSPStateResult{
			Response: map[string]any{"type": "state.snapshot"}, LegacyState: map[string]any{"project_uuid": "project"},
			ProjectEpoch: "epoch", Revision: 7, SnapshotHash: "same-revision",
		}}}
		err := port.Preflight(context.Background(), actionSet, cut)
		if err == nil || !strings.Contains(err.Error(), "parameter_preimage") {
			t.Fatalf("preimage drift err=%v", err)
		}
		if len(fake.batchCalls) != 0 {
			t.Fatal("preimage drift wrote parameters")
		}
	})
}

func TestSemanticEQMutationFailureReportsRestoredRollback(t *testing.T) {
	fake := newFakeEQKernel()
	fake.failReadAt[2] = true
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	action := semanticEQTestAction("bell", 3400, -3)
	receipt, err := (&semanticEQMutationPort{server: server}).Apply(context.Background(), orchestration.Action{
		ID: "eq-action", Args: map[string]any{
			"semantic_action": action, "track_id": "track-1", "plugin_id": "eq-1", "edits": semanticEQEditRows(action),
		},
	}, "exec-failure")
	if err == nil || receipt.Status != "failed" {
		t.Fatalf("receipt=%#v err=%v", receipt, err)
	}
	rollback := firstMapFromAny(receipt.Details["rollback"])
	if firstStringFromMap(rollback, "status") != "restored" || !strings.Contains(receipt.Error, "full preimage restored") {
		t.Fatalf("rollback=%#v receipt=%#v", rollback, receipt)
	}
}

func TestSemanticEQVerifierReportsAcousticUnavailableWithoutPostFXEvidence(t *testing.T) {
	actionSet := orchestration.ActionSet{Actions: []orchestration.Action{{Args: map[string]any{"acoustic_verification": "unavailable"}}}}
	receipt := orchestration.ActionReceipt{ActionID: "eq", Status: "applied", Details: map[string]any{"structural_readback": "pass"}}
	result, err := (semanticEQVerifier{}).Verify(context.Background(), actionSet, []orchestration.ActionReceipt{receipt})
	if err != nil || result.Status != "pass" || result.Structural != "pass" || result.Acoustic != "unavailable" || result.UserAcceptance != "unknown" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestSemanticEQFreshEvidenceRefreshDoesNotClaimAcousticPass(t *testing.T) {
	result := orchestration.VerificationResult{Status: "pass", Structural: "pass", Acoustic: "pass", UserAcceptance: "unknown"}
	semanticEQMarkEvidenceRefreshPass(&result, "obs-after", "post_fx")
	if result.Status != "inconclusive" || result.Acoustic != "inconclusive" || !strings.Contains(result.Summary, "evidence_refresh=pass") || !strings.Contains(result.Summary, "尚未证明听感目标") {
		t.Fatalf("evidence refresh overclaimed acoustic verification: %#v", result)
	}
}

func TestSemanticEQPendingSessionDoesNotHijackUnrelatedConversation(t *testing.T) {
	runtime := orchestrationruntime.New()
	session, err := runtime.StartAgentSemanticEQChatSession("cap_v1_semantic_eq_chat_1", "chat", "project", "减少浑浊", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{orchestrationRuntime: runtime}
	if resolution := server.resolveCapabilityOwner("chat", ChatRequest{Message: "把鼓的音量降低一点"}); resolution.CapabilityID != "" {
		t.Fatalf("unrelated turn was hijacked: %#v", resolution)
	}
	resolution := server.resolveCapabilityOwner("chat", ChatRequest{Message: "确认执行这个方案"})
	if resolution.CapabilityID != agentSemanticEQCapabilityID || resolution.SessionID != session.ID {
		t.Fatalf("proposal approval did not route to semantic EQ session: %#v", resolution)
	}
}
