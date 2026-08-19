package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/lowendrelation"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/semanticeffect"
)

func b4PlannerTestModel() lowendrelation.Model {
	return lowendrelation.Model{
		SchemaVersion: lowendrelation.ModelSchemaVersion, ModelID: "diag-1", ObservationID: "obs-1",
		Readiness: capabilityruntime.Readiness{CanProceed: true}, EvidenceRefs: []string{"mix.observe:obs-1"},
		Tracks:    []lowendrelation.LowEndTrack{{TrackID: "kick", TrackName: "Kick"}, {TrackID: "bass", TrackName: "Bass"}},
		Conflicts: []lowendrelation.LowEndConflict{{ID: "conflict-1", Band: "bass"}},
	}
}

func b4EQPlannerTestAction(trackID, pluginID string) string {
	return strings.Replace(semanticEQPlannerTestAction(trackID, pluginID),
		`"observation_id":"obs-eq-test"`, `"observation_id":"obs-1"`, 1)
}

func TestDecodeB4TreatmentPlanBindsFullProjectIdentities(t *testing.T) {
	text := `{"schema_version":"low_end_relation.treatment_plan.v1","summary":"separate kick and bass","targets":[{"order":2,"track_id":"bass","relationship_refs":["conflict-1"],"listening_goal":"retain weight below kick"},{"order":1,"track_id":"kick","relationship_refs":["conflict-1"],"listening_goal":"define the transient foundation"}]}`
	plan, err := decodeB4TreatmentPlan(text, b4PlannerTestModel())
	if err != nil {
		t.Fatalf("decode treatment plan: %v", err)
	}
	if plan.ObservationScope != "full_project" || len(plan.Targets) != 2 || plan.Targets[0].TrackID != "kick" || plan.Targets[1].TrackID != "bass" {
		t.Fatalf("unexpected finalized plan: %+v", plan)
	}
}

func TestDecodeB4InstanceSelectionsRejectsInventedPlugin(t *testing.T) {
	ambiguous := map[string][]semanticTreatmentInstance{"kick": {{TrackID: "kick", PluginID: "eq-a"}, {TrackID: "kick", PluginID: "eq-b"}}}
	selected, err := decodeB4InstanceSelections(`{"schema_version":"b4.eq_instance_selection.v1","selections":[{"track_id":"kick","plugin_id":"eq-b"}]}`, ambiguous)
	if err != nil || selected["kick"] != "eq-b" {
		t.Fatalf("exact loaded selection rejected: selected=%v err=%v", selected, err)
	}
	if _, err := decodeB4InstanceSelections(`{"schema_version":"b4.eq_instance_selection.v1","selections":[{"track_id":"kick","plugin_id":"invented"}]}`, ambiguous); err == nil {
		t.Fatal("invented plugin identity was accepted")
	}
}

func TestDecodeB4EQBatchRequiresEveryExactTargetInOrder(t *testing.T) {
	frequency, gain := 80.0, -1.5
	action := semanticeffect.Action{SchemaVersion: semanticeffect.ActionSchema, ActionType: semanticeffect.ActionEQEdit, PayloadSchema: semanticeffect.EQPlanSchema, Target: semanticeffect.Target{TrackID: "kick", PluginID: "eq-a"}, UserGoal: "make room for bass", Evidence: semanticeffect.EvidenceDecision{Choice: "derive", Basis: "observation", Reason: "B4 conflict", ObservationID: "obs-1"}, EQPlan: &semanticeffect.EQPlan{SchemaVersion: semanticeffect.EQPlanSchema, Atomic: true, Atoms: []semanticeffect.EQAtom{{AtomID: "kick-low", Action: "upsert", Shape: "bell", FrequencyHz: &frequency, GainDB: &gain, Purpose: "reduce overlap", Confidence: "medium", FieldOrigins: map[string]string{"frequency_hz": "llm_selected", "gain_db": "llm_selected"}}}}}
	batch := semanticeffect.Batch{SchemaVersion: semanticeffect.BatchSchema, ProjectGoal: "separate low end", Atomic: true, Actions: []semanticeffect.Action{action}}
	raw, _ := json.Marshal(batch)
	treatment, _ := decodeB4TreatmentPlan(`{"schema_version":"low_end_relation.treatment_plan.v1","summary":"separate","targets":[{"order":1,"track_id":"kick","relationship_refs":["conflict-1"],"listening_goal":"make room for bass"}]}`, b4PlannerTestModel())
	targets := []b4EQTargetContext{{Treatment: treatment.Targets[0], Instance: semanticTreatmentInstance{TrackID: "kick", PluginID: "eq-a"}}}
	if _, err := decodeB4EQBatch(string(raw), treatment, targets); err != nil {
		t.Fatalf("valid exact batch rejected: %v", err)
	}
	batch.Actions[0].Target.PluginID = "invented"
	raw, _ = json.Marshal(batch)
	if _, err := decodeB4EQBatch(string(raw), treatment, targets); err == nil {
		t.Fatal("batch changed an exact target without rejection")
	}
}

func TestPlanB4EQBatchRepairsMissingPurposeUsingSharedOrdinaryEQContract(t *testing.T) {
	validAction := b4EQPlannerTestAction("kick", "eq-a")
	validBatch := `{"schema_version":"semantic_effect_batch.v1","project_goal":"separate low end","atomic":true,"actions":[` + validAction + `]}`
	invalidBatch := strings.Replace(validBatch, `"purpose":`, `"omitted_purpose":`, 1)
	server, cfg, calls, bodies := semanticEQPlannerTestServer(t, []string{invalidBatch, validBatch + "}"})
	treatment := lowendrelation.TreatmentPlan{
		SchemaVersion:    lowendrelation.TreatmentPlanSchema,
		PlanID:           "plan-1",
		DiagnosisID:      "diag-1",
		ObservationID:    "obs-1",
		ObservationScope: "full_project",
		Summary:          "separate low end",
		Targets: []lowendrelation.TreatmentTarget{{
			TargetID: "target-1", Order: 1, TrackID: "kick", ListeningGoal: "make room for bass",
		}},
	}
	targets := []b4EQTargetContext{{
		Treatment: treatment.Targets[0],
		Instance: semanticTreatmentInstance{
			TrackID: "kick", PluginID: "eq-a", PluginName: "Test EQ",
			Topology: map[string]any{"sections": []any{map[string]any{"section": "1", "reachable_shapes": []any{"bell"}}}},
		},
	}}

	batch, err := server.planB4EQBatch(context.Background(), "conversation-1", treatment, targets, cfg, "")
	if err != nil {
		t.Fatalf("planB4EQBatch: %v", err)
	}
	if *calls != 2 || len(batch.Actions) != 1 || batch.Actions[0].EQPlan == nil || batch.Actions[0].EQPlan.Atoms[0].Purpose == "" {
		t.Fatalf("batch=%#v calls=%d, want repaired purpose after two calls", batch, *calls)
	}
	if len(*bodies) != 2 || !strings.Contains((*bodies)[0], "distinct acoustic purpose") || !strings.Contains((*bodies)[1], "purpose is required") {
		t.Fatalf("shared ordinary-EQ contract missing from B4 plan/repair requests")
	}
}

func TestPlanB4EQBatchStopsAfterBoundedRepairAttempts(t *testing.T) {
	server, cfg, calls, _ := semanticEQPlannerTestServer(t, []string{"not JSON", "still not JSON", "also not JSON"})
	treatment := lowendrelation.TreatmentPlan{
		SchemaVersion: lowendrelation.TreatmentPlanSchema,
		PlanID:        "plan-1", DiagnosisID: "diag-1", ObservationID: "obs-1", ObservationScope: "full_project",
		Summary: "separate low end",
		Targets: []lowendrelation.TreatmentTarget{{TargetID: "target-1", Order: 1, TrackID: "kick", ListeningGoal: "make room for bass"}},
	}
	targets := []b4EQTargetContext{{
		Treatment: treatment.Targets[0],
		Instance:  semanticTreatmentInstance{TrackID: "kick", PluginID: "eq-a", PluginName: "Test EQ", Topology: map[string]any{"sections": []any{map[string]any{"section": "1", "reachable_shapes": []any{"bell"}}}}},
	}}

	_, err := server.planB4EQBatch(context.Background(), "conversation-1", treatment, targets, cfg, "")
	if err == nil || !strings.Contains(err.Error(), "after 2 repair attempts") {
		t.Fatalf("bounded invalid planner result: calls=%d err=%v", *calls, err)
	}
	if *calls != 3 {
		t.Fatalf("LLM calls=%d, want initial call plus exactly two repairs", *calls)
	}
}

func TestPlanB4EQBatchCompletesOnlyMissingOrderedSuffixAfterTruncatedPrefix(t *testing.T) {
	action1 := b4EQPlannerTestAction("kick", "eq-a")
	action2 := b4EQPlannerTestAction("bass", "eq-b")
	action3 := b4EQPlannerTestAction("toms", "eq-c")
	prefixWithoutActionsClose := `{"schema_version":"semantic_effect_batch.v1","project_goal":"separate low end","atomic":true,"actions":[` + action1 + `,` + action2 + `}`
	remainingBatch := `{"schema_version":"semantic_effect_batch.v1","project_goal":"separate low end","atomic":true,"actions":[` + action3 + `]}`
	server, cfg, calls, bodies := semanticEQPlannerTestServer(t, []string{prefixWithoutActionsClose, remainingBatch})
	treatment := lowendrelation.TreatmentPlan{
		SchemaVersion: lowendrelation.TreatmentPlanSchema,
		PlanID:        "plan-1", DiagnosisID: "diag-1", ObservationID: "obs-1", ObservationScope: "full_project",
		Summary: "separate low end",
		Targets: []lowendrelation.TreatmentTarget{
			{TargetID: "target-1", Order: 1, TrackID: "kick", ListeningGoal: "anchor the transient"},
			{TargetID: "target-2", Order: 2, TrackID: "bass", ListeningGoal: "retain the bass body"},
			{TargetID: "target-3", Order: 3, TrackID: "toms", ListeningGoal: "remove low-end masking"},
		},
	}
	targets := []b4EQTargetContext{
		{Treatment: treatment.Targets[0], Instance: semanticTreatmentInstance{TrackID: "kick", PluginID: "eq-a", PluginName: "Test EQ", Topology: map[string]any{"sections": []any{map[string]any{"section": "1", "reachable_shapes": []any{"bell"}}}}}},
		{Treatment: treatment.Targets[1], Instance: semanticTreatmentInstance{TrackID: "bass", PluginID: "eq-b", PluginName: "Test EQ", Topology: map[string]any{"sections": []any{map[string]any{"section": "1", "reachable_shapes": []any{"bell"}}}}}},
		{Treatment: treatment.Targets[2], Instance: semanticTreatmentInstance{TrackID: "toms", PluginID: "eq-c", PluginName: "Test EQ", Topology: map[string]any{"sections": []any{map[string]any{"section": "1", "reachable_shapes": []any{"bell"}}}}}},
	}

	batch, err := server.planB4EQBatch(context.Background(), "conversation-1", treatment, targets, cfg, "")
	if err != nil {
		t.Fatalf("complete missing B4 suffix: %v", err)
	}
	if *calls != 2 || len(batch.Actions) != 3 {
		t.Fatalf("calls=%d actions=%d, want one suffix repair and three final actions", *calls, len(batch.Actions))
	}
	for index, wantTrack := range []string{"kick", "bass", "toms"} {
		if batch.Actions[index].Target.TrackID != wantTrack {
			t.Fatalf("action %d track=%q, want exact ordered target %q", index+1, batch.Actions[index].Target.TrackID, wantTrack)
		}
	}
	if len(*bodies) != 2 || !strings.Contains((*bodies)[1], "Do not repeat completed actions") || !strings.Contains((*bodies)[1], `\"track_id\":\"toms\"`) {
		t.Fatalf("suffix repair request did not isolate the missing exact target: %s", (*bodies)[1])
	}
}

func TestDecodeB4EQBatchTailRecoveryPreservesExactTargetValidation(t *testing.T) {
	validAction := b4EQPlannerTestAction("kick", "eq-a")
	validBatch := `{"schema_version":"semantic_effect_batch.v1","project_goal":"separate low end","atomic":true,"actions":[` + validAction + `]}`
	treatment := lowendrelation.TreatmentPlan{
		SchemaVersion: lowendrelation.TreatmentPlanSchema,
		ObservationID: "obs-1", ObservationScope: "full_project", Summary: "separate low end",
		Targets: []lowendrelation.TreatmentTarget{{TargetID: "target-1", Order: 1, TrackID: "kick", ListeningGoal: "make room for bass"}},
	}
	targets := []b4EQTargetContext{{Treatment: treatment.Targets[0], Instance: semanticTreatmentInstance{TrackID: "kick", PluginID: "eq-a"}}}

	if _, err := decodeB4EQBatchResponse(validBatch+"}\ntrailing noise", treatment, targets); err != nil {
		t.Fatalf("valid first object with tail noise was not recovered: %v", err)
	}
	invented := strings.Replace(validBatch, `"plugin_id":"eq-a"`, `"plugin_id":"invented"`, 1)
	if _, err := decodeB4EQBatchResponse(invented+"}", treatment, targets); err == nil {
		t.Fatal("tail recovery bypassed exact target validation")
	}
	missingActionsClose := validBatch[:len(validBatch)-2] + "}"
	if _, err := decodeB4EQBatchResponse(missingActionsClose, treatment, targets); err != nil {
		t.Fatalf("missing actions array closure was not deterministically recovered: %v", err)
	}
	inventedMissingClose := invented[:len(invented)-2] + "}"
	if _, err := decodeB4EQBatchResponse(inventedMissingClose, treatment, targets); err == nil {
		t.Fatal("array-closure recovery bypassed exact target validation")
	} else if !strings.Contains(err.Error(), "changed or reordered exact target") {
		t.Fatalf("recovered semantic failure was masked by the original syntax error: %v", err)
	}
}

func TestB4EQPlannerInputDeduplicatesStructuralTopologyWithoutBroadcastingTargets(t *testing.T) {
	treatment := lowendrelation.TreatmentPlan{
		SchemaVersion: lowendrelation.TreatmentPlanSchema,
		Summary:       "project low-end relationship",
	}
	targets := make([]b4EQTargetContext, 0, 15)
	for index := 0; index < 15; index++ {
		trackID := fmt.Sprintf("track-%02d", index+1)
		pluginID := fmt.Sprintf("plugin-%02d", index+1)
		target := lowendrelation.TreatmentTarget{TargetID: "target-" + trackID, Order: index + 1, TrackID: trackID, ListeningGoal: "distinct goal for " + trackID}
		treatment.Targets = append(treatment.Targets, target)
		targets = append(targets, b4EQTargetContext{
			Treatment: target,
			Instance: semanticTreatmentInstance{
				TrackID: trackID, PluginID: pluginID, PluginName: "Pro-Q 3",
				Topology: map[string]any{
					"schema_version": "generic_eq_topology.prompt.v1", "track_id": trackID, "plugin_id": pluginID,
					"topology_generation": "instance-specific-generation-" + pluginID,
					"sections": []any{map[string]any{
						"section": "1", "active": true, "reachable_kinds": []any{"bell"}, "frequency_writable": true, "gain_writable": true,
					}},
				},
			},
		})
	}

	input, topologyCount := b4EQPlannerInput(treatment, targets, "")
	if topologyCount != 1 || len(mapRowsValue(input["generic_eq_topology_catalog"])) != 1 {
		t.Fatalf("topology catalog count=%d input=%#v, want one shared structural topology", topologyCount, input["generic_eq_topology_catalog"])
	}
	rows := mapRowsValue(input["qualified_targets"])
	if len(rows) != 15 {
		t.Fatalf("qualified target count=%d, want all 15 exact targets", len(rows))
	}
	seen := map[string]bool{}
	for _, row := range rows {
		exact := firstMapFromAny(row["exact_target"])
		key := firstStringFromMap(exact, "track_id") + "::" + firstStringFromMap(exact, "plugin_id")
		if key == "::" || seen[key] {
			t.Fatalf("missing or duplicate exact target in %#v", row)
		}
		seen[key] = true
		if firstStringFromMap(row, "generic_eq_topology_ref") == "" {
			t.Fatalf("target omitted topology reference: %#v", row)
		}
		if _, embedded := row["generic_eq_topology"]; embedded {
			t.Fatalf("target repeated embedded topology: %#v", row)
		}
	}
	entry := mapRowsValue(input["generic_eq_topology_catalog"])[0]
	structural := firstMapFromAny(entry["generic_eq_topology"])
	for _, identityKey := range []string{"track_id", "plugin_id", "topology_generation"} {
		if _, leaked := structural[identityKey]; leaked {
			t.Fatalf("shared structural topology leaked %s: %#v", identityKey, structural)
		}
	}
	if b4EQPlannerTimeout != 3*time.Minute {
		t.Fatalf("B4 planner timeout=%v, want bounded 3m", b4EQPlannerTimeout)
	}
}

func TestB4ActionabilityAndExplicitRoutingBoundary(t *testing.T) {
	if !b4MutationRequested("请执行 B4 并调整需要处理的所有轨道", nil) {
		t.Fatal("explicit full-project B4 mutation was not actionable")
	}
	if b4MutationRequested("只分析一下 B4 的低频关系", nil) {
		t.Fatal("read-only B4 analysis became actionable")
	}
	if explicitB4InvocationMarker("把所选 EQ 的低频调整一下") {
		t.Fatal("ordinary selected EQ wording became an explicit B4 invocation")
	}
	if !explicitB4InvocationMarker("运行 B4") {
		t.Fatal("explicit B4 marker was not recognized")
	}
}

func TestActiveB4DoesNotTakeSelectedTrackHorizontalEQ(t *testing.T) {
	runtime := orchestrationruntime.New()
	if _, err := runtime.StartB4ChatSession("cap_v1_b4_chat_1", "chat", "project", "B4 project pass", orchestration.InteractionPropose); err != nil {
		t.Fatal(err)
	}
	server := &Server{orchestrationRuntime: runtime}
	resolution := server.resolveCapabilityOwner("chat", ChatRequest{Message: "adjust the selected EQ to reduce low-end relation masking", Context: map[string]any{"selected_track_id": "track-1", "selected_plugin_track_id": "track-1", "selected_plugin_id": "eq-1"}})
	if resolution.CapabilityID == lowEndRelationCapabilityID {
		t.Fatalf("horizontal selected-track EQ was taken by B4: %#v", resolution)
	}
}

func TestB4MissingEQEntersSelectionBeforeLoadConfirmation(t *testing.T) {
	server := New(nil, nil, nil)
	treatment, err := decodeB4TreatmentPlan(`{"schema_version":"low_end_relation.treatment_plan.v1","summary":"separate","targets":[{"order":1,"track_id":"kick","relationship_refs":["conflict-1"],"listening_goal":"make room"}]}`, b4PlannerTestModel())
	if err != nil {
		t.Fatal(err)
	}
	candidate := pluginRecommendationCandidate{Key: "plugin_candidate_1", Name: "Local EQ", PluginPath: `C:\Plugins\LocalEQ.vst3`}
	recommendation := pluginRecommendationPlan{SchemaVersion: "plugin_recommendation.v1", ProcessorType: "eq", Summary: "Use the local EQ", Choices: []pluginRecommendationChoice{{CandidateKey: candidate.Key, Role: "recommended", Reason: "versatile local EQ", Confidence: "medium"}}}
	response := server.b4PluginSelectionResponse("chat", agentruntime.Goal{GoalID: "goal", RunID: "run"}, "b4-session", treatment, b4PlannerTestModel(), treatment.Targets, recommendation, []pluginRecommendationCandidate{candidate})
	if response.Workflow != "plugin_selection_required" || response.NeedsConfirmation || len(response.InteractionRequests) != 1 || boolValue(response.WorkflowData["mutation_performed"]) {
		t.Fatalf("missing EQ skipped selection boundary: %#v", response)
	}
}

func TestB4LoadRollbackUsesKernelPluginItemIDContract(t *testing.T) {
	args := b4PluginDeleteArgs(map[string]any{"track_id": "track-1", "plugin_id": "legacy-id", "plugin_item_id": "2624"})
	if args["track_id"] != "track-1" || args["plugin_item_id"] != "2624" {
		t.Fatalf("rollback args = %#v", args)
	}
	if _, ok := args["plugin_id"]; ok {
		t.Fatalf("rollback sent obsolete delete_plugin field: %#v", args)
	}
}

func TestSharedPluginLoadRecoveryTracksUnqualifiedNewInstancesForRollback(t *testing.T) {
	instances := []semanticTreatmentInstance{{TrackID: "t1", PluginID: "old", QualificationStatus: "not_eq"}, {TrackID: "t1", PluginID: "new-qualified", QualificationStatus: "generic_static_eq_qualified"}, {TrackID: "t1", PluginID: "new-unqualified", QualificationStatus: "not_eq"}}
	got := projectEQNewInstances(instances, map[string]bool{"old": true})
	if len(got) != 2 || got[0].PluginID != "new-qualified" || got[1].PluginID != "new-unqualified" {
		t.Fatalf("recovery lost new instance identities: %+v", got)
	}
}

func TestB4BatchApplyPreservesAllLeafReadbacks(t *testing.T) {
	fake := newFakeEQKernel()
	server := New(nil, nil, nil)
	server.eqKernelOverride = fake
	first := semanticEQTestAction("bell", 3400, -2)
	second := semanticEQTestAction("bell", 5200, -1)
	second.Target = semanticeffect.Target{TrackID: "track-2", PluginID: "eq-2", TrackName: "Bass", PluginName: "Test EQ 2"}
	leaves := []map[string]any{}
	for _, action := range []semanticeffect.Action{first, second} {
		planned, err := server.planSemanticEQReadOnly(context.Background(), action)
		if err != nil {
			t.Fatal(err)
		}
		leaves = append(leaves, map[string]any{"semantic_action": action, "track_id": action.Target.TrackID, "plugin_id": action.Target.PluginID, "edits": planned.Edits, "planned_writes": planned.PlannedWrites, "parameter_preimage": planned.Preimage})
	}
	receipt, err := (&b4BatchMutationPort{server: server}).applyEQ(context.Background(), orchestration.Action{ID: "b4-batch", Args: map[string]any{"leaves": leaves}}, "exec-b4")
	if err != nil {
		t.Fatal(err)
	}
	leafReceipts, ok := receipt.Details["leaf_receipts"].([]orchestration.ActionReceipt)
	if receipt.Status != "applied" || !ok || len(leafReceipts) != 2 {
		t.Fatalf("incomplete batch receipt: %#v", receipt)
	}
	for index, leaf := range leafReceipts {
		if firstStringFromMap(leaf.Details, "structural_readback") != "pass" || len(mapRowsValue(leaf.Details["atom_results"])) == 0 {
			t.Fatalf("leaf %d lost requested/actual readback: %#v", index+1, leaf)
		}
	}
}

func TestC1ConfiguredSharedBatchPortPreservesLeafReadback(t *testing.T) {
	fake := newFakeEQKernel()
	server := New(nil, nil, nil)
	server.eqKernelOverride = fake
	action := semanticEQTestAction("bell", 420, -1.5)
	planned, err := server.planSemanticEQReadOnly(context.Background(), action)
	if err != nil {
		t.Fatal(err)
	}
	leaves := []map[string]any{{"semantic_action": action, "track_id": action.Target.TrackID, "plugin_id": action.Target.PluginID, "edits": planned.Edits, "planned_writes": planned.PlannedWrites, "parameter_preimage": planned.Preimage}}
	receipt, err := (&projectEQBatchMutationPort{server: server, spec: c1BatchSpec}).applyEQ(context.Background(), orchestration.Action{ID: "c1-batch", Args: map[string]any{"leaves": leaves}}, "exec-c1")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "applied" || len(receipt.EvidenceRefs) != 1 || !strings.HasPrefix(receipt.EvidenceRefs[0], "c1.semantic_eq_batch:") {
		t.Fatalf("C1 did not use the shared batch port identity/readback: %+v", receipt)
	}
}

func TestB4BatchLeafFailureRestoresEveryEarlierLeaf(t *testing.T) {
	assertProjectEQBatchLeafFailureRestoresEveryEarlierLeaf(t, projectEQBatchMutationSpec{})
}

func TestC1BatchLeafFailureRestoresEveryEarlierLeaf(t *testing.T) {
	assertProjectEQBatchLeafFailureRestoresEveryEarlierLeaf(t, c1BatchSpec)
}

func assertProjectEQBatchLeafFailureRestoresEveryEarlierLeaf(t *testing.T, spec projectEQBatchMutationSpec) {
	t.Helper()
	fake := newFakeEQKernel()
	before := fake.snapshot()
	server := New(nil, nil, nil)
	server.eqKernelOverride = fake
	first := semanticEQTestAction("bell", 3400, -2)
	second := semanticEQTestAction("bell", 5200, -1)
	second.Target = semanticeffect.Target{TrackID: "track-2", PluginID: "eq-2"}
	leaves := []map[string]any{}
	for _, action := range []semanticeffect.Action{first, second} {
		planned, err := server.planSemanticEQReadOnly(context.Background(), action)
		if err != nil {
			t.Fatal(err)
		}
		leaves = append(leaves, map[string]any{"semantic_action": action, "track_id": action.Target.TrackID, "plugin_id": action.Target.PluginID, "edits": planned.Edits, "planned_writes": planned.PlannedWrites, "parameter_preimage": planned.Preimage})
	}
	// Planning consumed reads 1-2. Each apply reads before and after writing;
	// fail the second leaf's readback. Its single-instance executor restores
	// itself, then the shared project port must restore the earlier leaf.
	fake.failReadAt[6] = true
	receipt, err := (&projectEQBatchMutationPort{server: server, spec: spec}).applyEQ(context.Background(), orchestration.Action{ID: "project-eq-batch", Args: map[string]any{"leaves": leaves}}, "exec-project-eq-fail")
	if err == nil || receipt.Status != "failed" {
		t.Fatalf("expected batch failure, receipt=%#v err=%v", receipt, err)
	}
	rollback := firstMapFromAny(receipt.Details["rollback"])
	if firstStringFromMap(rollback, "status") != "restored" || !fakeEQSnapshotsEqual(before, fake.snapshot()) {
		t.Fatalf("project rollback incomplete: rollback=%#v before=%v after=%v", rollback, before, fake.snapshot())
	}
}
