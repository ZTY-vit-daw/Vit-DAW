package chat

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectstore"
)

// MIXLAYER-M1-2 unit coverage: 9-state machine legality, node catalog
// integrity, blackboard fold (ledger + routed conclusions), annotation
// write-back, and the AGENTS §11 old-state default semantics.

func queueTestDerivedTransition(t *testing.T, current, kind, want string, wantOK bool) {
	t.Helper()
	got, ok := mixWorkflowDerivedTransition(current, kind)
	if got != want || ok != wantOK {
		t.Fatalf("transition(%s, %s) = (%s, %t), want (%s, %t)", current, kind, got, ok, want, wantOK)
	}
}

func TestMixWorkflowDerivedTransitionMatrix(t *testing.T) {
	// activity: opens work everywhere except on completed nodes.
	for _, current := range []string{
		MixWorkflowNodeNotStarted, MixWorkflowNodeInProgress, MixWorkflowNodeNeedsConfirmation,
		MixWorkflowNodeBlocked, MixWorkflowNodeSkipped, MixWorkflowNodeDeferred,
		MixWorkflowNodeNeedsRecheck, MixWorkflowNodeStale,
	} {
		queueTestDerivedTransition(t, current, mixWorkflowSignalActivity, MixWorkflowNodeInProgress, true)
	}
	queueTestDerivedTransition(t, MixWorkflowNodeCompleted, mixWorkflowSignalActivity, MixWorkflowNodeCompleted, false)

	// completed: evidence may complete from any state (idempotent on completed).
	for _, current := range []string{
		MixWorkflowNodeNotStarted, MixWorkflowNodeInProgress, MixWorkflowNodeNeedsConfirmation,
		MixWorkflowNodeCompleted, MixWorkflowNodeBlocked, MixWorkflowNodeSkipped,
		MixWorkflowNodeDeferred, MixWorkflowNodeNeedsRecheck, MixWorkflowNodeStale,
	} {
		queueTestDerivedTransition(t, current, mixWorkflowSignalCompleted, MixWorkflowNodeCompleted, true)
	}

	// recheck/stale: only meaningful once work exists; not_started/skipped reject.
	for _, current := range []string{
		MixWorkflowNodeInProgress, MixWorkflowNodeNeedsConfirmation, MixWorkflowNodeCompleted,
		MixWorkflowNodeBlocked, MixWorkflowNodeDeferred, MixWorkflowNodeNeedsRecheck, MixWorkflowNodeStale,
	} {
		queueTestDerivedTransition(t, current, mixWorkflowSignalRecheck, MixWorkflowNodeNeedsRecheck, true)
		queueTestDerivedTransition(t, current, mixWorkflowSignalStale, MixWorkflowNodeStale, true)
	}
	queueTestDerivedTransition(t, MixWorkflowNodeNotStarted, mixWorkflowSignalRecheck, MixWorkflowNodeNotStarted, false)
	queueTestDerivedTransition(t, MixWorkflowNodeSkipped, mixWorkflowSignalRecheck, MixWorkflowNodeSkipped, false)
	queueTestDerivedTransition(t, MixWorkflowNodeNotStarted, mixWorkflowSignalStale, MixWorkflowNodeNotStarted, false)
	queueTestDerivedTransition(t, MixWorkflowNodeSkipped, mixWorkflowSignalStale, MixWorkflowNodeSkipped, false)

	// blocked: capability-block evidence everywhere except completed.
	for _, current := range []string{
		MixWorkflowNodeNotStarted, MixWorkflowNodeInProgress, MixWorkflowNodeNeedsConfirmation,
		MixWorkflowNodeBlocked, MixWorkflowNodeSkipped, MixWorkflowNodeDeferred,
		MixWorkflowNodeNeedsRecheck, MixWorkflowNodeStale,
	} {
		queueTestDerivedTransition(t, current, mixWorkflowSignalBlocked, MixWorkflowNodeBlocked, true)
	}
	queueTestDerivedTransition(t, MixWorkflowNodeCompleted, mixWorkflowSignalBlocked, MixWorkflowNodeCompleted, false)

	// Unknown signal kinds never move a node.
	queueTestDerivedTransition(t, MixWorkflowNodeInProgress, "bogus", MixWorkflowNodeInProgress, false)
}

func TestMixWorkflowExplicitTransitionRules(t *testing.T) {
	// Terminal evidence states are derived-only: the annotation face cannot
	// command completed/needs_recheck/stale.
	for _, target := range []string{MixWorkflowNodeCompleted, MixWorkflowNodeNeedsRecheck, MixWorkflowNodeStale} {
		if err := mixWorkflowExplicitTransition(MixWorkflowNodeNotStarted, target); err == nil {
			t.Fatalf("annotation target %s must be rejected as evidence-derived", target)
		}
	}
	if err := mixWorkflowExplicitTransition(MixWorkflowNodeNotStarted, "nonsense"); err == nil {
		t.Fatal("unknown state must be rejected")
	}
	// Idempotent re-annotation is fine.
	if err := mixWorkflowExplicitTransition(MixWorkflowNodeDeferred, MixWorkflowNodeDeferred); err != nil {
		t.Fatalf("idempotent annotation rejected: %v", err)
	}
	// A completed node cannot be annotated over: only ledger recheck/stale
	// evidence may reopen it.
	if err := mixWorkflowExplicitTransition(MixWorkflowNodeCompleted, MixWorkflowNodeDeferred); err == nil {
		t.Fatal("annotating over a completed node must be rejected")
	}
	for _, current := range []string{
		MixWorkflowNodeNotStarted, MixWorkflowNodeInProgress, MixWorkflowNodeNeedsConfirmation,
		MixWorkflowNodeBlocked, MixWorkflowNodeSkipped, MixWorkflowNodeDeferred,
		MixWorkflowNodeNeedsRecheck, MixWorkflowNodeStale,
	} {
		if err := mixWorkflowExplicitTransition(current, MixWorkflowNodeSkipped); err != nil {
			t.Fatalf("explicit %s→skipped must be legal: %v", current, err)
		}
	}
}

func TestMixWorkflowQueueNodeCatalogIntegrity(t *testing.T) {
	wantIDs := []string{
		"A1", "A2", "A3", "A4", "A5",
		"B1", "B2", "B3", "B4",
		"C1", "C2", "C3", "C4", "C5",
		"D1", "D2", "D3", "D4",
		"E1", "E2", "E3", "E4",
		"F1",
	}
	if len(mixWorkflowQueueNodes) != len(wantIDs) {
		t.Fatalf("node catalog has %d nodes, want %d (master plan §6.1)", len(mixWorkflowQueueNodes), len(wantIDs))
	}
	seen := map[string]bool{}
	for index, node := range mixWorkflowQueueNodes {
		if node.ID != wantIDs[index] {
			t.Fatalf("node %d = %s, want %s (catalog order is the render order)", index, node.ID, wantIDs[index])
		}
		if seen[node.ID] {
			t.Fatalf("duplicate node id %s", node.ID)
		}
		seen[node.ID] = true
		if node.Title == "" || node.FamilyLabel == "" {
			t.Fatalf("node %s missing title/family label", node.ID)
		}
	}
	if len(mixWorkflowNodeStateLabels) != 9 {
		t.Fatalf("state label table has %d entries, want 9 (master plan §6.2)", len(mixWorkflowNodeStateLabels))
	}
}

func TestMixWorkflowNodeCapabilityResolution(t *testing.T) {
	cases := map[string]string{
		"static_mix.static_balance.v0":        "B2",
		"static_mix.pan_layout.v0":            "B3",
		"static_mix.low_end_relation.v0":      "B4",
		"fine_mix.frequency_cleanup.v0":       "C1",
		"fine_mix.frequency_cleanup.v1":       "C1",
		"fine_mix.dynamic_control.v1":         "C2",
		"fine_mix.space_depth.v0":             "C3",
		"project_prep.technical_integrity.v0": "A2",
		"mixing_capability_stage.b2":          "B2",
		"unknown.capability.v9":               "",
		"":                                    "",
	}
	for capability, want := range cases {
		if got := mixWorkflowNodeForCapability(capability); got != want {
			t.Fatalf("nodeForCapability(%q) = %q, want %q", capability, got, want)
		}
	}
}

func queueTestActivateProject(t *testing.T, projectUUID string) {
	t.Helper()
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	if _, _, err := projectstore.Activate(filepath.Join(t.TempDir(), "song.vitproj"), projectUUID); err != nil {
		t.Fatalf("activate projectstore: %v", err)
	}
}

// TestMixWorkflowQueueViewOldProjectDefaultsToNotStarted is the AGENTS §11
// default: a project with no blackboard records renders a complete queue with
// every node not_started and no read errors — missing files are the
// documented empty-board semantics, not failures.
func TestMixWorkflowQueueViewOldProjectDefaultsToNotStarted(t *testing.T) {
	queueTestActivateProject(t, "proj-m1-2-old")
	server := &Server{activeWorkspaceUUID: "proj-m1-2-old"}
	view := server.BuildMixWorkflowQueueView(context.Background())
	if view.SchemaVersion != MixWorkflowQueueSchemaVersion {
		t.Fatalf("schema = %s", view.SchemaVersion)
	}
	if len(view.Nodes) != 23 {
		t.Fatalf("node count = %d, want 23", len(view.Nodes))
	}
	for _, node := range view.Nodes {
		if node.State != MixWorkflowNodeNotStarted {
			t.Fatalf("old project node %s = %s, want not_started", node.NodeID, node.State)
		}
	}
	if view.StateSummary[MixWorkflowNodeNotStarted] != 23 {
		t.Fatalf("summary = %#v", view.StateSummary)
	}
	if len(view.Sources.ReadErrors) != 0 {
		t.Fatalf("old project must read clean, got errors %#v", view.Sources.ReadErrors)
	}
	render := RenderMixWorkflowQueueView(view)
	for _, marker := range []string{MixWorkflowQueueSchemaVersion, "工程准备", "混音审查", "未开始", "数据来源"} {
		if !strings.Contains(render, marker) {
			t.Fatalf("render missing %q:\n%s", marker, render)
		}
	}
}

func queueTestTerminalSession(id, capabilityID, projectUUID, cutHash string, completedAt time.Time) orchestration.PlanningSession {
	session := orchestration.PlanningSession{
		SchemaVersion: orchestration.SchemaVersion, ID: id, ProjectUUID: projectUUID,
		EngineOwner: orchestration.EngineV1, Status: orchestration.StatusCompleted,
		Goal:       "queue test " + capabilityID,
		Invocation: orchestration.CapabilityInvocation{CapabilityID: capabilityID, CapabilityVer: "v1"},
		FrozenPlan: &orchestration.FrozenPlan{
			Proposal:              orchestration.Proposal{ID: "proposal-" + id, Revision: 1, CapabilityID: capabilityID, TargetScope: []string{"track-1"}},
			ActionSet:             orchestration.ActionSet{ID: "actions-" + id, CapabilityID: capabilityID, ProjectCutHash: cutHash, Hash: "action-hash-" + id, Actions: []orchestration.Action{{ID: "action-" + id, Command: "test.apply", TargetRef: "track-1"}}},
			ProjectCut:            orchestration.ProjectCut{ProjectUUID: projectUUID, ProjectEpoch: "epoch-1", BaseProjectRevision: "1", Consistency: "strong", Hash: cutHash},
			PreviousObservationID: "obs-before-" + id,
		},
		Execution: &orchestration.ExecutionRecord{
			ID: "execution-" + id, SessionID: id, Status: "completed",
			ReceiptRef: "receipt-" + id, PersistenceRefs: []string{"project-history:commit-" + id},
			Receipts:           []orchestration.ActionReceipt{{ActionID: "action-" + id, Status: "ok", EffectivelyOnce: true}},
			Verification:       "pass",
			VerificationResult: &orchestration.VerificationResult{Status: "pass", Structural: "pass", Acoustic: "pass"},
			UpdatedAt:          completedAt,
		},
		CreatedAt: completedAt.Add(-time.Minute), UpdatedAt: completedAt,
	}
	return session
}

func queueTestNodeState(view MixWorkflowQueueView, nodeID string) MixWorkflowQueueNodeView {
	for _, node := range view.Nodes {
		if node.NodeID == nodeID {
			return node
		}
	}
	t := MixWorkflowQueueNodeView{State: "<missing>"}
	return t
}

// TestMixWorkflowQueueViewFoldsDecisionLedger: one verified C1 session
// completes node C1 from the ledger; unmapped capabilities count without a
// signal. (A second overlapping C1 record would flip the earlier record to
// needs_review through the ledger's own impact linkage — that recheck path is
// covered by the timeline fold test below.)
func TestMixWorkflowQueueViewFoldsDecisionLedger(t *testing.T) {
	queueTestActivateProject(t, "proj-m1-2-ledger")
	store := mixboard.NewStore("")
	base := time.Now().UTC().Add(-time.Hour)
	if _, err := store.RecordCapabilitySession(queueTestTerminalSession("sess-c1-ok", "fine_mix.frequency_cleanup.v1", "proj-m1-2-ledger", "cut-1", base)); err != nil {
		t.Fatalf("record C1: %v", err)
	}
	// Unmapped capability: counts, no signal.
	if _, err := store.RecordCapabilitySession(queueTestTerminalSession("sess-other", "future.family.v9", "proj-m1-2-ledger", "cut-3", base.Add(time.Minute))); err != nil {
		t.Fatalf("record unmapped: %v", err)
	}

	server := &Server{activeWorkspaceUUID: "proj-m1-2-ledger"}
	view := server.BuildMixWorkflowQueueView(context.Background())
	if node := queueTestNodeState(view, "C1"); node.State != MixWorkflowNodeCompleted {
		t.Fatalf("C1 = %s (%#v), want completed", node.State, view.Sources)
	}
	if view.Sources.MappedDecisionSignals != 1 {
		t.Fatalf("mapped signals = %d, want 1", view.Sources.MappedDecisionSignals)
	}
	if view.Sources.UnmappedDecisionRecords != 1 {
		t.Fatalf("unmapped = %d, want 1", view.Sources.UnmappedDecisionRecords)
	}
	if view.Sources.DroppedIllegalTransitions != 0 {
		t.Fatalf("dropped = %d, want 0", view.Sources.DroppedIllegalTransitions)
	}
}

// TestMixWorkflowQueueFoldDropsIllegalTimelineEntries folds the timeline
// directly: a verified completion followed by a deferred annotation (written
// while the node was not yet completed, recorded after the session landed)
// is illegal at fold time — dropped and counted, the node stays completed.
// Also pins the not_started default for unseen nodes (empty ≠ unknown).
func TestMixWorkflowQueueFoldDropsIllegalTimelineEntries(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	verified := mixWorkflowQueueTimelineEntry{
		at: base, nodeID: "C1",
		apply: func(current string) (string, bool) {
			return mixWorkflowDerivedTransition(current, mixWorkflowSignalCompleted)
		},
		basis: "ledger:verified",
	}
	lateAnnotation := mixWorkflowQueueTimelineEntry{
		at: base.Add(time.Minute), nodeID: "C1", isWrite: true,
		apply: func(current string) (string, bool) {
			if err := mixWorkflowExplicitTransition(current, MixWorkflowNodeDeferred); err != nil {
				return current, false
			}
			return MixWorkflowNodeDeferred, true
		},
		basis: "annotation:deferred",
	}
	recheckOnFresh := mixWorkflowQueueTimelineEntry{
		at: base.Add(2 * time.Minute), nodeID: "C5",
		apply: func(current string) (string, bool) {
			return mixWorkflowDerivedTransition(current, mixWorkflowSignalRecheck)
		},
		basis: "ledger:recheck",
	}
	unknownNode := mixWorkflowQueueTimelineEntry{
		at: base.Add(3 * time.Minute), nodeID: "Z9",
		apply: func(current string) (string, bool) { return current, true },
		basis: "bogus",
	}
	sources := MixWorkflowQueueSources{}
	states, _ := foldMixWorkflowQueueTimeline([]mixWorkflowQueueTimelineEntry{verified, lateAnnotation, recheckOnFresh, unknownNode}, &sources)
	if states["C1"] != MixWorkflowNodeCompleted {
		t.Fatalf("C1 = %s, want completed (late annotation must not reopen)", states["C1"])
	}
	if states["C5"] != "" {
		t.Fatalf("C5 = %q, want absent (rejected entries leave the not_started default to the view layer)", states["C5"])
	}
	if sources.DroppedIllegalTransitions != 3 {
		t.Fatalf("dropped = %d, want 3 (late annotation, fresh recheck, unknown node)", sources.DroppedIllegalTransitions)
	}
}

// TestMixWorkflowQueueViewRoutesConclusions: a free-state settle conclusion
// maps onto its conversation's routed capability (unambiguous single-capability
// conversations only); capability_blocked conclusions block the node.
func TestMixWorkflowQueueViewRoutesConclusions(t *testing.T) {
	queueTestActivateProject(t, "proj-m1-2-routes")

	loop := &freeStateReasoningLoop{
		ConversationID: "conv-queue-1", GoalID: "goal-q1", RunID: "run-q1", LoopID: "loop-q1",
		Experiment: &experiment.Turn{ID: "turn-q1"},
	}
	loop.LatestProjectChange = map[string]any{"project_uuid": "proj-m1-2-routes"}
	server := &Server{activeWorkspaceUUID: "proj-m1-2-routes"}
	server.recordFreeStateSettlement(loop, mixboard.FreeStateConclusionKindExperimentSettle, "improved",
		"queue route join probe", nil, "")

	// No route yet: conclusion is unrouted metadata only.
	view := server.BuildMixWorkflowQueueView(context.Background())
	if view.Sources.UnroutedConclusions != 1 || view.Sources.RoutedConclusionSignals != 0 {
		t.Fatalf("unrouted fold wrong: %#v", view.Sources)
	}

	// A single-capability route for that conversation maps the conclusion.
	server.capabilityRoutes = map[string]CapabilityRouteRecord{
		"task-q1": {TaskID: "task-q1", ConversationID: "conv-queue-1",
			Assessment: &FreeStateCapacityAssessment{SelectedCapability: "fine_mix.dynamic_control.v1"}},
	}
	view = server.BuildMixWorkflowQueueView(context.Background())
	if view.Sources.RoutedConclusionSignals != 1 {
		t.Fatalf("routed signals = %d, want 1: %#v", view.Sources.RoutedConclusionSignals, view.Sources)
	}
	if node := queueTestNodeState(view, "C2"); node.State != MixWorkflowNodeInProgress {
		t.Fatalf("C2 = %s, want in_progress (improved settle = activity signal)", node.State)
	}

	// A blocked closure on C3 blocks that node through the same join.
	loopBlock := &freeStateReasoningLoop{
		ConversationID: "conv-queue-2", GoalID: "goal-q2", RunID: "run-q2", LoopID: "loop-q2",
		Experiment: &experiment.Turn{ID: "turn-q2"},
	}
	loopBlock.LatestProjectChange = map[string]any{"project_uuid": "proj-m1-2-routes"}
	server.recordFreeStateSettlement(loopBlock, mixboard.FreeStateConclusionKindLoopClosureSettle, "capability_blocked",
		"closure made no material progress", nil, "")
	server.capabilityRoutes["task-q2"] = CapabilityRouteRecord{TaskID: "task-q2", ConversationID: "conv-queue-2",
		Assessment: &FreeStateCapacityAssessment{SelectedCapability: "fine_mix.space_depth.v0"}}
	view = server.BuildMixWorkflowQueueView(context.Background())
	if node := queueTestNodeState(view, "C3"); node.State != MixWorkflowNodeBlocked {
		t.Fatalf("C3 = %s, want blocked: %#v", node.State, view.Sources)
	}

	// Ambiguous conversation (two distinct routed capabilities): conclusions
	// stay unmapped rather than guessing a node.
	server.capabilityRoutes["task-q3"] = CapabilityRouteRecord{TaskID: "task-q3", ConversationID: "conv-queue-2",
		Assessment: &FreeStateCapacityAssessment{SelectedCapability: "static_mix.pan_layout.v0"}}
	view = server.BuildMixWorkflowQueueView(context.Background())
	if view.Sources.AmbiguousRouteConversations != 1 {
		t.Fatalf("ambiguous conversations = %d, want 1", view.Sources.AmbiguousRouteConversations)
	}
	if node := queueTestNodeState(view, "C3"); node.State != MixWorkflowNodeNotStarted {
		t.Fatalf("ambiguous route must not map: C3 = %s", node.State)
	}
}

// TestMixWorkflowQueueAnnotationWriteBackRoundtrip: the annotation face is
// the write-back surface — validation against the folded state, idempotent
// re-writes, and the completed-node guard.
func TestMixWorkflowQueueAnnotationWriteBackRoundtrip(t *testing.T) {
	queueTestActivateProject(t, "proj-m1-2-annot")
	server := &Server{activeWorkspaceUUID: "proj-m1-2-annot"}

	if _, err := server.WriteMixWorkflowQueueNodeState(context.Background(), "E1", MixWorkflowNodeDeferred, "母带后置", nil, "conv-annot"); err != nil {
		t.Fatalf("write E1 deferred: %v", err)
	}
	view := server.BuildMixWorkflowQueueView(context.Background())
	if node := queueTestNodeState(view, "E1"); node.State != MixWorkflowNodeDeferred {
		t.Fatalf("E1 = %s, want deferred", node.State)
	}
	if view.Sources.NodeAnnotations != 1 {
		t.Fatalf("annotations = %d, want 1", view.Sources.NodeAnnotations)
	}

	// Idempotent re-annotation.
	if _, err := server.WriteMixWorkflowQueueNodeState(context.Background(), "e1", MixWorkflowNodeDeferred, "", nil, ""); err != nil {
		t.Fatalf("idempotent rewrite: %v", err)
	}

	// Unknown node / unknown state / evidence-derived targets rejected.
	if _, err := server.WriteMixWorkflowQueueNodeState(context.Background(), "Z9", MixWorkflowNodeDeferred, "", nil, ""); err == nil {
		t.Fatal("unknown node must be rejected")
	}
	if _, err := server.WriteMixWorkflowQueueNodeState(context.Background(), "E2", "weird", "", nil, ""); err == nil {
		t.Fatal("unknown state must be rejected")
	}
	if _, err := server.WriteMixWorkflowQueueNodeState(context.Background(), "E2", MixWorkflowNodeCompleted, "", nil, ""); err == nil {
		t.Fatal("annotation cannot command completed")
	}

	// Completed node (ledger evidence) refuses annotation overrides.
	if _, err := (mixboard.NewStore("")).RecordCapabilitySession(queueTestTerminalSession("sess-b3", "static_mix.pan_layout.v0", "proj-m1-2-annot", "cut-b3", time.Now().UTC().Add(-time.Minute))); err != nil {
		t.Fatalf("record B3: %v", err)
	}
	if node := queueTestNodeState(server.BuildMixWorkflowQueueView(context.Background()), "B3"); node.State != MixWorkflowNodeCompleted {
		t.Fatalf("B3 = %s, want completed before the guard probe", node.State)
	}
	if _, err := server.WriteMixWorkflowQueueNodeState(context.Background(), "B3", MixWorkflowNodeSkipped, "", nil, ""); err == nil {
		t.Fatal("annotating over completed must be rejected")
	}
}

func TestMixWorkflowQueueAnnotationRestoreHygiene(t *testing.T) {
	restored := restoreMixWorkflowQueueAnnotations(nil)
	if len(restored) != 0 {
		t.Fatalf("nil overlay must restore empty, got %#v", restored)
	}
	at := time.Now().UTC()
	dirty := map[string]MixWorkflowQueueNodeAnnotation{
		"C4": {NodeID: "C4", State: MixWorkflowNodeDeferred, UpdatedAt: at},
		"Z9": {NodeID: "Z9", State: MixWorkflowNodeDeferred, UpdatedAt: at},  // unknown node
		"C5": {NodeID: "C5", State: MixWorkflowNodeCompleted, UpdatedAt: at}, // evidence-derived only
		"D1": {NodeID: "D1", State: "galactic", UpdatedAt: at},               // unknown state
	}
	clean := restoreMixWorkflowQueueAnnotations(dirty)
	if len(clean) != 1 || clean["C4"].State != MixWorkflowNodeDeferred {
		t.Fatalf("restore hygiene kept %#v, want only C4/deferred", clean)
	}
}

// TestProjectAgentRuntimeStateQueueOverlayRoundtrip: the annotation overlay
// survives the runtime-state JSON roundtrip, and pre-M1-2 state files (field
// absent) decode to the documented empty-overlay default (AGENTS §11).
func TestProjectAgentRuntimeStateQueueOverlayRoundtrip(t *testing.T) {
	at := time.Now().UTC()
	state := projectAgentRuntimeState{
		SchemaVersion: "vit_project_agent_runtime.v2",
		MixWorkflowQueueNodes: map[string]MixWorkflowQueueNodeAnnotation{
			"E1": {NodeID: "E1", State: MixWorkflowNodeDeferred, Reason: "母带后置", UpdatedAt: at},
		},
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded projectAgentRuntimeState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	restored := restoreMixWorkflowQueueAnnotations(decoded.MixWorkflowQueueNodes)
	if len(restored) != 1 || restored["E1"].State != MixWorkflowNodeDeferred || restored["E1"].Reason != "母带后置" {
		t.Fatalf("overlay lost in roundtrip: %#v", restored)
	}

	old := []byte(`{"schema_version":"vit_project_agent_runtime.v2","project_uuid":"p1"}`)
	var decodedOld projectAgentRuntimeState
	if err := json.Unmarshal(old, &decodedOld); err != nil {
		t.Fatalf("old state decode: %v", err)
	}
	if len(restoreMixWorkflowQueueAnnotations(decodedOld.MixWorkflowQueueNodes)) != 0 {
		t.Fatal("old state must default to an empty overlay")
	}
}

// TestMixWorkflowQueueChatInterceptFaces: the deterministic chat-side face
// answers queue reads and annotation commands directly (the carrier run
// 194717 proved the semantic-entry classifier cannot be relied on for the
// annotation wording), and passes non-queue text through untouched.
func TestMixWorkflowQueueChatInterceptFaces(t *testing.T) {
	queueTestActivateProject(t, "proj-m1-2-chatface")
	server := &Server{activeWorkspaceUUID: "proj-m1-2-chatface"}
	ctx := context.Background()

	readResp, handled := server.handleMixWorkflowQueueChat(ctx, "conv-face", "", "查看混音工作队列状态")
	if !handled {
		t.Fatal("queue status query must be handled")
	}
	if !strings.Contains(readResp.Reply, MixWorkflowQueueSchemaVersion) || !strings.Contains(readResp.Reply, "数据来源") {
		t.Fatalf("status reply missing render: %s", readResp.Reply)
	}
	if readResp.Workflow != "mix_workflow_queue" || readResp.GoalStatus != "completed" {
		t.Fatalf("status reply shape drifted: %+v", readResp)
	}

	writeResp, handled := server.handleMixWorkflowQueueChat(ctx, "conv-face", "", "队列标记 E1 延期 母带后置")
	if !handled {
		t.Fatal("annotation command must be handled")
	}
	if !strings.Contains(writeResp.Reply, "已回写节点标注：E1 母带 EQ[延期]") {
		t.Fatalf("annotation ack missing: %s", writeResp.Reply)
	}
	view := server.BuildMixWorkflowQueueView(ctx)
	if node := queueTestNodeState(view, "E1"); node.State != MixWorkflowNodeDeferred {
		t.Fatalf("E1 = %s, want deferred after chat annotation", node.State)
	}

	// “队列标记 E1 完成” is outside the annotation grammar (completed is
	// evidence-derived): the deterministic face does not own the wording and
	// passes it through — the completed-target rejection is enforced at the
	// write face (TestMixWorkflowQueueAnnotationWriteBackRoundtrip).
	if _, handled := server.handleMixWorkflowQueueChat(ctx, "conv-face", "", "队列标记 E1 完成"); handled {
		t.Fatal("out-of-grammar queue wording must pass through to the normal flow")
	}

	if _, handled := server.handleMixWorkflowQueueChat(ctx, "conv-face", "", "帮我把主唱电平提高一点"); handled {
		t.Fatal("ordinary mix request must pass through to the normal flow")
	}
}
