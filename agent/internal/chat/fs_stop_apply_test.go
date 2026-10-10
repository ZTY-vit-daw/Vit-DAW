package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/orchestrationcontroller"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

// FS-STOP-APPLY-1 regression fixtures: the 2026-10-01 18:30–18:36 M1 live run
// (conversation webui_mupe9yh1, goal_36b2f2e8 → park → user_stop 18:34:07;
// goal_6b07c186 18:35:32 → immediate audio_closure_controller_failure
// "conversation is already owned by minimal_audio_closure controller
// audio_closure_997396f72e46902c"). The fixtures mirror the persisted shape of
// that run: a judgment-parked experiment loop, an fs7 non-terminal closure,
// and a controller owner acquired for that closure.

const fsStopApplyConversation = "conversation-fs-stop-apply-1"

// fsStopApplyFixtureServer builds the M1 post-application shape: task contract
// admitted, experiment admitted with an applied round awaiting the human
// judgment, closure walked to fs7, controller owner acquired, and the loop
// parked exactly like the live loop persisted at 18:34:0x.
func fsStopApplyFixtureServer(t *testing.T) (*Server, agentruntime.Goal, freeStateReasoningLoop, audioclosure.State) {
	t.Helper()
	server := &Server{
		harness:            harness.NewWithSender(nil, nil, nil),
		audioClosures:      audioclosure.NewMemoryStore(),
		controllerOwners:   orchestrationcontroller.NewRegistry(),
		freeStateLoops:     map[string]freeStateReasoningLoop{},
		capabilityRoutes:   map[string]CapabilityRouteRecord{},
		conversationGoals:  map[string]string{},
		conversationMemory: map[string]agentloop.ExecutionMemory{},
	}
	goal := server.harness.EnsureGoal("goal-fs-stop-apply-1", "run-fs-stop-apply-1", "检查一下当前工程有什么问题吗")
	if _, err := server.ensureAudioTaskContract(fsStopApplyConversation, audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-fs-stop-apply-1"}, "project-fs-stop-apply-1", "rev-1",
		map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}

	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-fs-stop-apply-1",
		ConversationID: fsStopApplyConversation, GoalID: goal.GoalID, RunID: "run-fs-stop-apply-1",
		Status: "awaiting_experiment", OriginalIntent: "检查一下当前工程有什么问题吗",
		LatestObservation: d1FreshObservationForTest("rev-1"),
		CreatedAt:         time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := server.applyFreeStateDecisionSemantic(&loop, agentloop.FreeStateDecision{
		Status: agentloop.FreeStateNeedsExperiment, Summary: "fixture proposal admission",
		ImprovementProposal: experimentTestProposal(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := server.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	d2ApplyAndObserveForTest(t, loop.Experiment, "fs-stop-apply-action-1", "rev-1")
	server.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{
			State: experiment.MaterialitySubthreshold, Evaluation: "insufficient_dose",
			Attempt: 1, EvidenceRefs: []string{"obs-fs-stop-apply-action-1"},
		},
		ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
	})
	// The 18:34:06 park form: judgment boundary, audition session armed.
	loop.Status = "blocked"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	loop.AuditionSessionID = "audition:turn:fs-stop-apply-1:round-1"
	loop.AuditionSessionSnapshot = map[string]any{"status": "ready", "session_id": loop.AuditionSessionID}
	server.storeFreeStateLoop(loop)

	// The closure at fs7 — the non-terminal spine state the live closure kept
	// after the stop (M1 forensic: "closure 停 fs7 非终态").
	current := server.harness.RuntimeStatus(goal.GoalID)
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-fs-stop-apply-1", ConversationID: fsStopApplyConversation,
		TaskID: current.Task.TaskID, GoalID: goal.GoalID, RunID: current.RunID,
		ContractID: current.Task.Contract.ContractID, TaskState: current.Task.SemanticState.State,
		TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID:       "project-fs-stop-apply-1", ProjectRevision: "rev-1",
		OriginalIntent: "检查一下当前工程有什么问题吗", Mode: audioclosure.ModeTreatment,
		Scope: audioclosure.Scope{Kind: "project", ID: "project-fs-stop-apply-1"},
		Now:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	driver := audioclosure.Driver{}
	now := time.Now().UTC()
	walk := func(to audioclosure.Phase, guard audioclosure.PhaseGuardInput) {
		next, walkErr := driver.TransitionPhase(state, state.Revision, to, guard, "fs-stop-apply-1 fixture", now)
		if walkErr != nil {
			t.Fatalf("fixture phase walk to %s failed: %v", to, walkErr)
		}
		state = next
	}
	walk(audioclosure.PhaseFS0SemanticEntry, audioclosure.PhaseGuardInput{})
	walk(audioclosure.PhaseFS1ProjectBound, audioclosure.PhaseGuardInput{ProjectBound: true})
	walk(audioclosure.PhaseFS2CapacityAssessed, audioclosure.PhaseGuardInput{CapacityAssessed: true})
	walk(audioclosure.PhaseFS3ProjectScan, audioclosure.PhaseGuardInput{ScanUsable: true})
	walk(audioclosure.PhaseFS4DiagnosticRound, audioclosure.PhaseGuardInput{QueueNonEmpty: true})
	walk(audioclosure.PhaseFS5CandidateFrontier, audioclosure.PhaseGuardInput{FrontierEstablished: true, DimensionClosed: true})
	walk(audioclosure.PhaseFS6TargetConfirmed, audioclosure.PhaseGuardInput{TargetEvidence: true})
	walk(audioclosure.PhaseFS7ImprovementProposal, audioclosure.PhaseGuardInput{GatePassed: true})
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	// The live wedge's other half: the controller owner acquired for this
	// closure and never released.
	if err := server.ensureAudioClosureOwner(state); err != nil {
		t.Fatal(err)
	}
	return server, goal, loop, state
}

// Scenario (a) — 观察运行中取消 → 干预不应用 + 轮诚实停止。
// The M1 Part A shape: the user pressed stop while the turn was actively
// executing; the in-flight LLM decision returned with a bounded improvement
// proposal; the pre-fix flow applied the EQ 6s later (18:34:04) and stored a
// pending mix tick. The gate between "decision returned" and "intervention
// executed" must skip the application, land the stop, and settle the closure
// (Part B) in the same boundary.
func TestStopPendingDecisionSkipsInterventionApplication(t *testing.T) {
	server, goal, _, closure := fsStopApplyFixtureServer(t)
	// The turn is actively executing (the runner is inside the LLM call when
	// the user presses stop): RequestGoalStop only latches the stop.
	server.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusProcessing, nil)
	server.harness.RequestGoalStop(goal.GoalID, "user_stop")
	if pending := server.goalStopPending(goal.GoalID); !pending {
		t.Fatal("setup: the stop latch must read as pending while the turn is executing")
	}
	// The decision returns: needs_experiment + a bounded proposal — exactly
	// what improvementProposalResponse would route straight into
	// executePendingMixTickCandidate under full access.
	res := agentloop.Result{
		GoalID: goal.GoalID, RunID: goal.RunID, Status: agentruntime.StatusWaitingContinue,
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status: agentloop.FreeStateNeedsExperiment, Summary: "fixture decision",
			ImprovementProposal: experimentTestProposal(),
		},
	}
	resp, stopped := server.stopPendingExperimentProposalResponse(context.Background(), fsStopApplyConversation, agentModeDefault, res)
	if !stopped {
		t.Fatal("the stop-pending gate must intercept the returned proposal")
	}
	if resp.GoalStatus != string(agentruntime.StatusStopped) {
		t.Fatalf("goal status=%q want stopped", resp.GoalStatus)
	}
	if resp.StopReason != stopPendingProposalSkippedReason {
		t.Fatalf("stop reason=%q want %q", resp.StopReason, stopPendingProposalSkippedReason)
	}
	if resp.NeedsConfirmation || len(resp.InteractionRequests) > 0 {
		t.Fatalf("the skipped proposal must not leave an answerable interaction: %+v", resp.InteractionRequests)
	}
	if !boolValue(resp.WorkflowData["intervention_skipped"]) || firstStringFromMap(resp.WorkflowData, "skip_reason") != "user_stop_pending" {
		t.Fatalf("workflow data must record the honest skip: %+v", resp.WorkflowData)
	}
	// No new intervention fired: no pending mix tick was stored for the turn.
	if _, ok := server.pendingMixTickForConversation(fsStopApplyConversation); ok {
		t.Fatal("a pending mix tick was stored although the stop was already pending")
	}
	// The turn honestly stopped: goal stopped, experiment stopped (never
	// presented as a governed settle success), and the closure settled with
	// the ownership released (Part B in the same boundary).
	if status := server.harness.RuntimeStatus(goal.GoalID).Status; status != agentruntime.StatusStopped {
		t.Fatalf("goal status=%q want stopped", status)
	}
	stored, _ := server.freeStateLoop(fsStopApplyConversation)
	if stored.Status != "stopped" || stored.Experiment == nil || stored.Experiment.Status != experiment.StatusStopped {
		t.Fatalf("loop status=%q experiment=%+v", stored.Status, stored.Experiment)
	}
	settled, ok := server.audioClosures.Load(closure.ClosureID)
	if !ok || !settled.Terminal() {
		t.Fatalf("closure must settle at the stopped-turn boundary: ok=%v settlement=%+v", ok, settled.Settlement)
	}
	if settled.Settlement != nil && settled.Settlement.Reason == audioclosure.StopSatisfied {
		t.Fatalf("a user-stopped closure must not settle as satisfied: %+v", settled.Settlement)
	}
	if _, active := server.controllerOwners.Active(fsStopApplyConversation); active {
		t.Fatal("the controller owner must be released at the stopped-turn boundary")
	}
}

// Scenario (b) — park 态取消 → 新输入正常新 goal（修复前红=本案实证）。
// The park case: the goal sits waiting_continue (the judgment park), the user
// presses stop → finalizeStoppedTurn runs inline. Pre-fix the closure stayed
// non-terminal and the controller owner stayed active, so the conversation
// was permanently wedged: goal_6b07c186 failed 85s later with
// audio_closure_controller_failure "conversation is already owned by
// minimal_audio_closure controller audio_closure_997396f72e46902c". Post-fix
// the same conversation accepts a fresh goal's closure acquisition.
func TestUserStopReleasesClosureOwnershipForNextGoal(t *testing.T) {
	server, goal, loop, closure := fsStopApplyFixtureServer(t)
	// The park: goal waiting_continue at the judgment boundary.
	server.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusWaitingContinue, nil)
	if !freeStateJudgmentBoundary(loop) {
		t.Fatal("setup: the fixture loop must sit at the judgment boundary")
	}
	body, _ := json.Marshal(map[string]any{"conversation_id": fsStopApplyConversation, "goal_id": goal.GoalID, "run_id": goal.RunID, "reason": "user_stop"})
	req := httptest.NewRequest(http.MethodPost, "/agent/turn/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("stop status=%d body=%s", resp.Code, resp.Body.String())
	}
	if status := server.harness.RuntimeStatus(goal.GoalID).Status; status != agentruntime.StatusStopped {
		t.Fatalf("goal status=%q want stopped", status)
	}
	// Pre-fix red #1: the closure stayed non-terminal (M1: fs7 forever).
	settled, ok := server.audioClosures.Load(closure.ClosureID)
	if !ok || !settled.Terminal() {
		t.Fatalf("the parked-state stop must settle the closure honestly: ok=%v settlement=%+v", ok, settled.Settlement)
	}
	// Pre-fix red #2: the controller owner was never released.
	if _, active := server.controllerOwners.Active(fsStopApplyConversation); active {
		t.Fatal("the controller owner must be released by the parked-state stop")
	}
	// The stopped task must be terminal (legal close migration, never a
	// fabricated governed settle).
	stoppedGoal := server.harness.RuntimeStatus(goal.GoalID)
	if stoppedGoal.Task == nil || stoppedGoal.Task.SemanticState == nil || !stoppedGoal.Task.SemanticState.Terminal {
		t.Fatalf("the stopped goal's task must be terminal: %+v", stoppedGoal.Task)
	}

	// The second message (18:35:32): a fresh goal must acquire the
	// conversation's closure cleanly — the exact intake that failed with
	// audio_closure_controller_failure in the live run.
	newGoal := server.harness.BeginGoal("检查一下当前选中的drums轨道的低频")
	entryContext := contextWithSemanticEntryDecision(map[string]any{
		"goal_id": newGoal.GoalID, "run_id": newGoal.RunID,
		"project_uuid": "project-fs-stop-apply-1", "project_revision": "rev-1",
	}, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteObservation,
		// Production assigns the controller during capacity routing before the
		// entry decision is bound into the context; the fixture assigns it
		// directly so the closure intake takes the create-new-closure branch.
		Controller:  string(orchestrationcontroller.MinimalAudioClosure),
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlObserveOnly,
		UserAuthorization: semanticEntryAuthorizationObserve, Confidence: 0.9, Reason: "fixture observation entry",
	})
	_, newState, active, err := server.prepareAudioClosureContext(fsStopApplyConversation, "检查一下当前选中的drums轨道的低频", entryContext)
	if err != nil {
		t.Fatalf("the next goal's closure intake stayed wedged (the 18:35:32 live failure): %v", err)
	}
	if !active || newState.Terminal() || newState.GoalID != newGoal.GoalID {
		t.Fatalf("the next goal must run on its own fresh closure: active=%v closure_goal=%q", active, newState.GoalID)
	}
}

// The stopped≠parked boundary: a user-stopped experiment must never be
// adopted by the next user input (FS-PARK semantics stay untouched); the
// explicit judgment POST channel on a stopped turn keeps its existing 409
// boundary (server.go stop_turn_prevents_new_action idiom).
func TestStoppedExperimentIsNotAdoptedByContinuation(t *testing.T) {
	server, goal, _, _ := fsStopApplyFixtureServer(t)
	server.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusWaitingContinue, nil)
	body, _ := json.Marshal(map[string]any{"conversation_id": fsStopApplyConversation, "goal_id": goal.GoalID, "run_id": goal.RunID, "reason": "user_stop"})
	req := httptest.NewRequest(http.MethodPost, "/agent/turn/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	server.Routes().ServeHTTP(httptest.NewRecorder(), req)
	loop, ok := server.freeStateLoop(fsStopApplyConversation)
	if !ok || loop.Experiment == nil {
		t.Fatal("setup: the stopped loop must persist")
	}
	if loop.Experiment.Status != experiment.StatusStopped {
		t.Fatalf("experiment status=%q want stopped", loop.Experiment.Status)
	}
	if judgmentParkPendingRound(loop) {
		t.Fatal("a stopped experiment must not read as a judgment-park pending round (adoption must not revive)")
	}
	if server.settleJudgmentParkOnUserContinuation(fsStopApplyConversation, "新话题，检查另一条轨道") {
		t.Fatal("a stopped turn must not be adopted as a judgment park by new input")
	}
}

// The anchor: handleTurnStop must leave a request-arrival INFO line
// (conversation/goal/run/reason/prior status) — the 2026-10-01 forensics had
// to rely on user testimony for the press moment because no log existed.
func TestHandleTurnStopLogsRequestArrival(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, "StopAnchor.vit")
	if err := os.WriteFile(projectPath, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "stop-anchor.log")
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": "vitproj_stop_anchor"})
	s := New(nil, project, logx.New(false, logPath, 256))
	s.activateCurrentProjectWorkspace(context.Background())
	goal := s.harness.BeginGoal("anchor me")
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusWaitingContinue, nil)
	body, _ := json.Marshal(map[string]any{"conversation_id": "chat-stop-anchor", "goal_id": goal.GoalID, "run_id": goal.RunID, "reason": "user_stop"})
	req := httptest.NewRequest(http.MethodPost, "/agent/turn/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.Routes().ServeHTTP(httptest.NewRecorder(), req)
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log file unreadable: %v", err)
	}
	found := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "[turn.stop] request received") &&
			strings.Contains(line, "chat-stop-anchor") &&
			strings.Contains(line, goal.GoalID) &&
			strings.Contains(line, "user_stop") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing the stop request arrival anchor line; log=%s", raw)
	}
}
