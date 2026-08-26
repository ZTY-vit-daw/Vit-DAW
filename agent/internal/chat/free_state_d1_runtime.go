package chat

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/executionruntime"
	"vit-daw-agent/internal/executionverifiers"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
)

// D2-1 static_eq admission constants. They mirror the experiment domain
// table rows (d1s1_domains.go); the chat plan layer uses them to build the
// bounded band adjustment action without touching the experiment package.
const d1StaticEQDomain = "static_eq"
const d1StaticEQKind = "static_eq_band_adjust"

// d1StaticEQCapabilityID identifies the bounded static EQ band adjustment in
// the frozen ActionSet/Proposal. The D1 execution path routes by the explicit
// mutation port (StaticEQVSPPort), so no capability registry registration is
// required; the id is audit metadata only (design record D2-1 S2a).
const d1StaticEQCapabilityID = "static_mix.static_eq.v0"

// defaultD1StaticEQPluginIdentifier is the plan-layer default EQ instance
// identifier used when the admission does not pin one. The StaticEQVSPPort
// instantiates it through instantiate_plugin only when the target track has
// no plugin_id yet.
const defaultD1StaticEQPluginIdentifier = "juce_eq"

type d1JournalMutationPort struct {
	inner   executionruntime.MutationPort
	harness interface {
		JournalGet(string) (journal.Action, bool)
		JournalRecord(journal.Action) journal.Action
		JournalMarkResult(string, journal.ActionStatus, map[string]any, error)
	}
	goalID string
	runID  string
	// staticEQ switches the journal shape to the D2-1 static_eq variant
	// (set_plugin_param). The zero value keeps the D1-S1 track_gain shape.
	staticEQ bool
}

func (p *d1JournalMutationPort) Preflight(ctx context.Context, set orchestration.ActionSet, cut orchestration.ProjectCut) error {
	return p.inner.Preflight(ctx, set, cut)
}

func (p *d1JournalMutationPort) Apply(ctx context.Context, action orchestration.Action, key string) (orchestration.ActionReceipt, error) {
	if _, exists := p.harness.JournalGet(action.ID); !exists {
		if p.staticEQ {
			p.harness.JournalRecord(journal.Action{
				AgentActionID: action.ID, GoalID: p.goalID, RunID: p.runID, Domain: "daw", Source: "free_state_d1_s1",
				Summary: "D2-1 bounded static EQ band adjustment", Tool: "set_plugin_param", CommandName: "set_plugin_param",
			Command: map[string]any{"cmd": "set_plugin_param", "track_id": action.TargetRef,
				"plugin_id": firstNonEmpty(firstStringFromMap(action.Args, "plugin_id"), firstStringFromMap(action.Args, "plugin_identifier")), "param_id": firstStringFromMap(action.Args, "param_id"),
				"value": action.Args["target_value"]},
				RiskLevel: "confirm", RequiresConfirmation: true, ConfirmationStatus: "confirmed", Status: journal.StatusRunning,
			})
		} else {
			p.harness.JournalRecord(journal.Action{
				AgentActionID: action.ID, GoalID: p.goalID, RunID: p.runID, Domain: "daw", Source: "free_state_d1_s1",
				Summary: "D1-S1 bounded track gain adjustment", Tool: "track_gain_adjust", CommandName: "track_gain_adjust",
				Command:   map[string]any{"cmd": "set_volume", "track_id": action.TargetRef, "db": action.Args["target_db"]},
				RiskLevel: "confirm", RequiresConfirmation: true, ConfirmationStatus: "confirmed", Status: journal.StatusRunning,
			})
		}
	}
	receipt, err := p.inner.Apply(ctx, action, key)
	status := journal.StatusSucceeded
	if err != nil || !strings.EqualFold(receipt.Status, "applied") {
		status = journal.StatusFailed
	}
	p.harness.JournalMarkResult(action.ID, status, map[string]any{"execution_receipt": receipt}, err)
	return receipt, err
}

func d1TrackGainPlan(loop freeStateReasoningLoop, candidate agentloop.PendingMixTickCandidate, stateRevision int64, projectUUID, projectEpoch, snapshotHash string, state map[string]any) (orchestration.FrozenPlan, error) {
	if loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return orchestration.FrozenPlan{}, fmt.Errorf("active D1-S1 experiment is required")
	}
	if err := loop.Experiment.Admission.ValidateD1S1(); err != nil {
		return orchestration.FrozenPlan{}, err
	}
	if candidate.Operation != experiment.D1S1ActionKind || strings.TrimSpace(candidate.TrackID) == "" || candidate.DeltaDB == 0 || math.Abs(candidate.DeltaDB) > 2 {
		return orchestration.FrozenPlan{}, fmt.Errorf("D1-S1 requires one bounded track_gain_adjust")
	}
	admittedDelta, ok := treatmentNumber(loop.Experiment.Admission.TypedAction, "delta_db")
	if !ok || math.Abs(candidate.DeltaDB-admittedDelta) > 0.0001 {
		return orchestration.FrozenPlan{}, fmt.Errorf("D1-S1 candidate delta does not match the admitted observation-backed action")
	}
	targetID := firstStringFromMap(loop.Experiment.Admission.TargetRef, "id", "track_id")
	if candidate.TrackID != targetID {
		return orchestration.FrozenPlan{}, fmt.Errorf("D1-S1 candidate target does not match admitted observation target")
	}
	currentDB, ok := d1TrackGain(state, targetID)
	if !ok {
		return orchestration.FrozenPlan{}, fmt.Errorf("D1-S1 target gain readback is unavailable")
	}
	if stateRevision <= 0 || projectUUID == "" || projectEpoch == "" || snapshotHash == "" {
		return orchestration.FrozenPlan{}, fmt.Errorf("D1-S1 requires a revision-bound VSP snapshot")
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{
		State: d1StateResult(projectUUID, projectEpoch, snapshotHash, stateRevision, state), Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: append([]string(nil), loop.Experiment.Admission.EvidenceRefs...),
		TargetFingerprints:     []string{fmt.Sprintf("track:%s:fader:%g", targetID, currentDB)},
		ContractVersions:       []string{"free_state:d1_s1", "action:track_gain_adjust"},
	})
	if err != nil {
		return orchestration.FrozenPlan{}, err
	}
	actionID := "d1_" + sanitizeCanaryID(loop.Experiment.ID) + "_gain"
	action := orchestration.Action{ID: actionID, Command: experiment.D1S1ActionKind, TargetRef: targetID,
		BeforeFingerprint: fmt.Sprintf("track:%s:fader_db:%g", targetID, currentDB),
		Args:              map[string]any{"delta_db": candidate.DeltaDB, "target_db": currentDB + candidate.DeltaDB}, Compensatable: true, IdempotencyClass: "effectively_once"}
	set := orchestration.ActionSet{ID: "d1_set_" + sanitizeCanaryID(loop.Experiment.ID), CapabilityID: staticBalanceCapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	set.Hash = set.ComputeHash()
	round, _ := loop.Experiment.CurrentRound()
	previousObservationID := ""
	for _, observation := range round.Observations {
		if !observation.PostAction {
			previousObservationID = observation.ID
		}
	}
	proposal := orchestration.Proposal{ID: "d1_proposal_" + sanitizeCanaryID(loop.Experiment.ID), Revision: 1, CapabilityID: staticBalanceCapabilityID,
		CapabilityVer: "v0", ProjectCutHash: cut.Hash, ActionSetHash: set.Hash, TargetScope: []string{targetID}, Risk: "reversible", VerificationRef: "fresh_revision_bound_ccb", Summary: loop.Experiment.Admission.Hypothesis, CreatedAt: time.Now().UTC()}
	return orchestration.FrozenPlan{Proposal: proposal, ActionSet: set, ProjectCut: cut, ContextBundleID: "d1_context_" + sanitizeCanaryID(loop.Experiment.ID), PreviousObservationID: previousObservationID, FrozenAt: time.Now().UTC()}, nil
}

// d1StaticEQPlan mirrors d1TrackGainPlan for the bounded static EQ band
// adjustment: one admitted action, one forward mutation, revision-bound cut.
// Plugin parameter before values are read by the port's Preflight; the plan
// layer never invokes kernel commands.
func d1StaticEQPlan(loop freeStateReasoningLoop, candidate agentloop.PendingMixTickCandidate, stateRevision int64, projectUUID, projectEpoch, snapshotHash string, state map[string]any) (orchestration.FrozenPlan, error) {
	if loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return orchestration.FrozenPlan{}, fmt.Errorf("active D1-S1 experiment is required")
	}
	if err := loop.Experiment.Admission.ValidateD1S1(); err != nil {
		return orchestration.FrozenPlan{}, err
	}
	if spec, ok := experiment.D1S1DomainSpecFor(loop.Experiment.Admission); !ok || spec.ActionDomain != d1StaticEQDomain || spec.ActionKind != d1StaticEQKind {
		return orchestration.FrozenPlan{}, fmt.Errorf("D2-1 static_eq plan requires a static_eq admission")
	}
	if candidate.Operation != d1StaticEQKind || strings.TrimSpace(candidate.TrackID) == "" {
		return orchestration.FrozenPlan{}, fmt.Errorf("D2-1 requires one bounded static_eq_band_adjust")
	}
	gainDB, ok := treatmentNumber(loop.Experiment.Admission.TypedAction, "gain_db")
	if !ok || gainDB == 0 || math.Abs(gainDB) > 2 {
		return orchestration.FrozenPlan{}, fmt.Errorf("D2-1 requires one bounded band gain move within +/-2 dB")
	}
	targetID := firstStringFromMap(loop.Experiment.Admission.TargetRef, "id", "track_id")
	if candidate.TrackID != targetID {
		return orchestration.FrozenPlan{}, fmt.Errorf("D2-1 candidate target does not match admitted observation target")
	}
	bandIndex := 0
	if value, present := loop.Experiment.Admission.TypedAction["band_index"]; present {
		if parsed, ok := treatmentNumber(map[string]any{"band_index": value}, "band_index"); ok {
			bandIndex = int(parsed)
		}
	}
	paramID := fmt.Sprintf("band_%d_gain", bandIndex)
	pluginIdentifier := firstStringFromMap(loop.Experiment.Admission.TypedAction, "plugin_identifier")
	if pluginIdentifier == "" {
		pluginIdentifier = defaultD1StaticEQPluginIdentifier
	}
	if stateRevision <= 0 || projectUUID == "" || projectEpoch == "" || snapshotHash == "" {
		return orchestration.FrozenPlan{}, fmt.Errorf("D2-1 requires a revision-bound VSP snapshot")
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{
		State: d1StateResult(projectUUID, projectEpoch, snapshotHash, stateRevision, state), Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: append([]string(nil), loop.Experiment.Admission.EvidenceRefs...),
		TargetFingerprints:     []string{fmt.Sprintf("track:%s:eq:%s:pending", targetID, paramID)},
		ContractVersions:       []string{"free_state:d1_s1", "action:static_eq_band_adjust"},
	})
	if err != nil {
		return orchestration.FrozenPlan{}, err
	}
	actionID := "d1_" + sanitizeCanaryID(loop.Experiment.ID) + "_eq"
	action := orchestration.Action{ID: actionID, Command: d1StaticEQKind, TargetRef: targetID,
		BeforeFingerprint: fmt.Sprintf("track:%s:eq:%s:pending", targetID, paramID),
		Args:              map[string]any{"plugin_identifier": pluginIdentifier, "param_id": paramID, "target_value": gainDB}, Compensatable: true, IdempotencyClass: "effectively_once"}
	set := orchestration.ActionSet{ID: "d1_set_" + sanitizeCanaryID(loop.Experiment.ID), CapabilityID: d1StaticEQCapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	set.Hash = set.ComputeHash()
	round, _ := loop.Experiment.CurrentRound()
	previousObservationID := ""
	for _, observation := range round.Observations {
		if !observation.PostAction {
			previousObservationID = observation.ID
		}
	}
	proposal := orchestration.Proposal{ID: "d1_proposal_" + sanitizeCanaryID(loop.Experiment.ID), Revision: 1, CapabilityID: d1StaticEQCapabilityID,
		CapabilityVer: "v0", ProjectCutHash: cut.Hash, ActionSetHash: set.Hash, TargetScope: []string{targetID}, Risk: "reversible", VerificationRef: "fresh_revision_bound_ccb", Summary: loop.Experiment.Admission.Hypothesis, CreatedAt: time.Now().UTC()}
	return orchestration.FrozenPlan{Proposal: proposal, ActionSet: set, ProjectCut: cut, ContextBundleID: "d1_context_" + sanitizeCanaryID(loop.Experiment.ID), PreviousObservationID: previousObservationID, FrozenAt: time.Now().UTC()}, nil
}

func d1TrackGain(state map[string]any, trackID string) (float64, bool) {
	for _, row := range mapRowsFromAny(state["tracks"]) {
		if firstStringFromMap(row, "track_id", "id") != trackID {
			continue
		}
		return treatmentNumber(row, "volume_db", "fader_db", "gain_db", "db")
	}
	return 0, false
}

func d1StateResult(projectUUID, epoch, hash string, revision int64, state map[string]any) *kernel.VSPStateResult {
	copy := cloneContext(state)
	copy["project_uuid"] = projectUUID
	return &kernel.VSPStateResult{Response: map[string]any{"type": "state.snapshot"}, Payload: map[string]any{"snapshot_hash": hash}, LegacyState: copy, ProjectEpoch: epoch, Revision: revision, SnapshotHash: hash}
}

func (s *Server) executeD1TrackGain(ctx context.Context, conversationID string, req ChatRequest, candidate agentloop.PendingMixTickCandidate) (ChatResponse, bool) {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return ChatResponse{}, false
	}
	if s.kernel == nil || s.harness == nil || s.orchestrationRuntime == nil || !s.orchestrationRuntime.HasDurableStore() {
		return d1BlockedResponse(loop, "D1-S1 durable execution dependencies are unavailable"), true
	}
	sessionID := "d1_session_" + sanitizeCanaryID(loop.Experiment.ID)
	session, exists := s.orchestrationRuntime.Store.Load(sessionID)
	var plan orchestration.FrozenPlan
	var state *kernel.VSPStateResult
	var projectUUID string
	var err error
	if exists {
		if session.FrozenPlan == nil {
			return d1BlockedResponse(loop, "D1-S1 durable session has no frozen plan"), true
		}
		plan = *session.FrozenPlan
		projectUUID = session.ProjectUUID
		beforeRender := firstMapFromAny(loop.D1State["before_render"])
		if firstStringFromMap(beforeRender, "status") != "ready" || firstStringFromMap(beforeRender, "project_revision") != plan.ProjectCut.BaseProjectRevision || !validD1RenderFile(firstStringFromMap(beforeRender, "file_path")) {
			return d1BlockedResponse(loop, "D1-S1 restart recovery requires the persisted before render bound to the frozen base revision"), true
		}
	} else {
		state, err = s.kernel.VSPStateSnapshot(ctx, "project.timeline")
		if err != nil || state == nil || !state.OK() {
			return d1BlockedResponse(loop, "D1-S1 VSP snapshot unavailable: "+canaryErrorText(err)), true
		}
		projectUUID = firstNonEmpty(firstStringFromMap(state.LegacyState, "project_uuid", "id"), firstStringFromMap(loop.LatestProjectChange, "project_uuid", "project_id"))
		round, roundErr := loop.Experiment.CurrentRound()
		if roundErr != nil || len(round.Observations) == 0 || round.Observations[0].PostAction || round.Observations[0].ProjectRevision != fmt.Sprint(state.Revision) {
			return d1BlockedResponse(loop, "D1-S1 VSP base revision does not match the fresh admitted observation"), true
		}
		if _, err = s.ensureD1Render(ctx, &loop, "before", fmt.Sprint(state.Revision), round.CheckpointRef); err != nil {
			return d1BlockedResponse(loop, err.Error()), true
		}
		afterRender, snapshotErr := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
		if snapshotErr != nil || afterRender == nil || !afterRender.OK() || afterRender.ProjectEpoch != state.ProjectEpoch || afterRender.Revision != state.Revision || afterRender.SnapshotHash != state.SnapshotHash {
			return d1BlockedResponse(loop, "D1-S1 project changed while producing the before render"), true
		}
		state = afterRender
		plan, err = d1TrackGainPlan(loop, candidate, state.Revision, projectUUID, state.ProjectEpoch, state.SnapshotHash, state.LegacyState)
		if err != nil {
			return d1BlockedResponse(loop, err.Error()), true
		}
	}
	if !exists {
		session, err = s.orchestrationRuntime.StartB2ChatSession(sessionID, conversationID, projectUUID, loop.OriginalIntent, orchestration.InteractionPropose)
		if err == nil {
			session, err = s.orchestrationRuntime.AttachFrozenPlan(sessionID, plan)
		}
		if err == nil {
			decision := orchestration.ApprovalDecision{SchemaVersion: orchestration.ApprovalDecisionSchema, Kind: orchestration.ApprovalApprove,
				ProposalID: plan.Proposal.ID, ProposalRevision: plan.Proposal.Revision, ActionSetHash: plan.ActionSet.Hash, ProjectCutHash: plan.ProjectCut.Hash,
				ApprovedScope: []string{candidate.TrackID}, SourceTurnID: loop.Experiment.ID, UserText: req.Message, Confidence: "explicit"}
			session, err = s.orchestrationRuntime.AuthorizeProposal(sessionID, orchestration.Authorization{ProposalID: plan.Proposal.ID, ProposalRevision: 1,
				ActionSetHash: plan.ActionSet.Hash, ProjectCutHash: plan.ProjectCut.Hash, Scope: []string{candidate.TrackID}, SourceTurnID: loop.Experiment.ID, Sequence: session.Revision + 1, Decision: &decision})
		}
	}
	if err != nil {
		return d1BlockedResponse(loop, err.Error()), true
	}
	if state == nil {
		state, err = s.kernel.VSPStateSnapshot(ctx, "project.timeline")
		if err != nil || state == nil || !state.OK() {
			return d1BlockedResponse(loop, "D1-S1 recovery snapshot unavailable: "+canaryErrorText(err)), true
		}
	}
	verifier := executionverifiers.StaticBalance{State: s.kernel, Acoustic: executionverifiers.HarnessAcoustic{Invoker: s.harness,
		PreviousObservationID: plan.PreviousObservationID, RequireExplicitFresh: true, MixSessionID: sessionID, GoalText: loop.OriginalIntent}}
	switch session.Status {
	case orchestration.StatusExecuting, orchestration.StatusVerifying:
		session, err = s.orchestrationRuntime.ReconcileActionSet(ctx, sessionID, plan.ActionSet, &executionports.StaticBalanceVSPPort{Client: s.kernel}, verifier)
	case orchestration.StatusAuthorized:
		port := &d1JournalMutationPort{inner: &executionports.StaticBalanceVSPPort{Client: s.kernel}, harness: s.harness, goalID: loop.GoalID, runID: loop.RunID}
		session, err = s.orchestrationRuntime.ExecuteActionSetWithPersistence(ctx, sessionID, plan.ActionSet, plan.ProjectCut, port, verifier,
			executionports.ProjectHistory{Harness: s.harness, GoalID: loop.GoalID, RunID: loop.RunID})
	case orchestration.StatusCompleted, orchestration.StatusNeedsReview:
		// Durable terminal execution is projected below without another mutation.
	default:
		return d1BlockedResponse(loop, "D1-S1 durable session is not executable at status "+string(session.Status)), true
	}
	return s.projectD1Execution(loop, session, err), true
}

// executeD1StaticEQ mirrors executeD1TrackGain for the bounded static EQ
// band adjustment: the same durable session/plan/authorize/execute shape, with
// the StaticEQVSPPort (one idempotency key, one receipt) and the static_eq
// journal variant. track_gain keeps its own function untouched.
func (s *Server) executeD1StaticEQ(ctx context.Context, conversationID string, req ChatRequest, candidate agentloop.PendingMixTickCandidate) (ChatResponse, bool) {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return ChatResponse{}, false
	}
	if s.kernel == nil || s.harness == nil || s.orchestrationRuntime == nil || !s.orchestrationRuntime.HasDurableStore() {
		return d1BlockedResponse(loop, "D1-S1 durable execution dependencies are unavailable"), true
	}
	sessionID := "d1_session_" + sanitizeCanaryID(loop.Experiment.ID)
	session, exists := s.orchestrationRuntime.Store.Load(sessionID)
	var plan orchestration.FrozenPlan
	var state *kernel.VSPStateResult
	var projectUUID string
	var err error
	if exists {
		if session.FrozenPlan == nil {
			return d1BlockedResponse(loop, "D1-S1 durable session has no frozen plan"), true
		}
		plan = *session.FrozenPlan
		projectUUID = session.ProjectUUID
		beforeRender := firstMapFromAny(loop.D1State["before_render"])
		if firstStringFromMap(beforeRender, "status") != "ready" || firstStringFromMap(beforeRender, "project_revision") != plan.ProjectCut.BaseProjectRevision || !validD1RenderFile(firstStringFromMap(beforeRender, "file_path")) {
			return d1BlockedResponse(loop, "D1-S1 restart recovery requires the persisted before render bound to the frozen base revision"), true
		}
	} else {
		state, err = s.kernel.VSPStateSnapshot(ctx, "project.timeline")
		if err != nil || state == nil || !state.OK() {
			return d1BlockedResponse(loop, "D1-S1 VSP snapshot unavailable: "+canaryErrorText(err)), true
		}
		projectUUID = firstNonEmpty(firstStringFromMap(state.LegacyState, "project_uuid", "id"), firstStringFromMap(loop.LatestProjectChange, "project_uuid", "project_id"))
		round, roundErr := loop.Experiment.CurrentRound()
		if roundErr != nil || len(round.Observations) == 0 || round.Observations[0].PostAction || round.Observations[0].ProjectRevision != fmt.Sprint(state.Revision) {
			return d1BlockedResponse(loop, "D1-S1 VSP base revision does not match the fresh admitted observation"), true
		}
		if _, err = s.ensureD1Render(ctx, &loop, "before", fmt.Sprint(state.Revision), round.CheckpointRef); err != nil {
			return d1BlockedResponse(loop, err.Error()), true
		}
		afterRender, snapshotErr := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
		if snapshotErr != nil || afterRender == nil || !afterRender.OK() || afterRender.ProjectEpoch != state.ProjectEpoch || afterRender.Revision != state.Revision || afterRender.SnapshotHash != state.SnapshotHash {
			return d1BlockedResponse(loop, "D1-S1 project changed while producing the before render"), true
		}
		state = afterRender
		plan, err = d1StaticEQPlan(loop, candidate, state.Revision, projectUUID, state.ProjectEpoch, state.SnapshotHash, state.LegacyState)
		if err != nil {
			return d1BlockedResponse(loop, err.Error()), true
		}
	}
	if !exists {
		session, err = s.orchestrationRuntime.StartB2ChatSession(sessionID, conversationID, projectUUID, loop.OriginalIntent, orchestration.InteractionPropose)
		if err == nil {
			session, err = s.orchestrationRuntime.AttachFrozenPlan(sessionID, plan)
		}
		if err == nil {
			decision := orchestration.ApprovalDecision{SchemaVersion: orchestration.ApprovalDecisionSchema, Kind: orchestration.ApprovalApprove,
				ProposalID: plan.Proposal.ID, ProposalRevision: plan.Proposal.Revision, ActionSetHash: plan.ActionSet.Hash, ProjectCutHash: plan.ProjectCut.Hash,
				ApprovedScope: []string{candidate.TrackID}, SourceTurnID: loop.Experiment.ID, UserText: req.Message, Confidence: "explicit"}
			session, err = s.orchestrationRuntime.AuthorizeProposal(sessionID, orchestration.Authorization{ProposalID: plan.Proposal.ID, ProposalRevision: 1,
				ActionSetHash: plan.ActionSet.Hash, ProjectCutHash: plan.ProjectCut.Hash, Scope: []string{candidate.TrackID}, SourceTurnID: loop.Experiment.ID, Sequence: session.Revision + 1, Decision: &decision})
		}
	}
	if err != nil {
		return d1BlockedResponse(loop, err.Error()), true
	}
	if state == nil {
		state, err = s.kernel.VSPStateSnapshot(ctx, "project.timeline")
		if err != nil || state == nil || !state.OK() {
			return d1BlockedResponse(loop, "D1-S1 recovery snapshot unavailable: "+canaryErrorText(err)), true
		}
	}
	verifier := executionverifiers.StaticEQ{State: s.kernel, Acoustic: executionverifiers.HarnessAcoustic{Invoker: s.harness,
		PreviousObservationID: plan.PreviousObservationID, RequireExplicitFresh: true, MixSessionID: sessionID, GoalText: loop.OriginalIntent}}
	switch session.Status {
	case orchestration.StatusExecuting, orchestration.StatusVerifying:
		session, err = s.orchestrationRuntime.ReconcileActionSet(ctx, sessionID, plan.ActionSet, &executionports.StaticEQVSPPort{Client: s.kernel}, verifier)
	case orchestration.StatusAuthorized:
		port := &d1JournalMutationPort{inner: &executionports.StaticEQVSPPort{Client: s.kernel}, harness: s.harness, goalID: loop.GoalID, runID: loop.RunID, staticEQ: true}
		session, err = s.orchestrationRuntime.ExecuteActionSetWithPersistence(ctx, sessionID, plan.ActionSet, plan.ProjectCut, port, verifier,
			executionports.ProjectHistory{Harness: s.harness, GoalID: loop.GoalID, RunID: loop.RunID})
	case orchestration.StatusCompleted, orchestration.StatusNeedsReview:
		// Durable terminal execution is projected below without another mutation.
	default:
		return d1BlockedResponse(loop, "D1-S1 durable session is not executable at status "+string(session.Status)), true
	}
	return s.projectD1Execution(loop, session, err), true
}

func (s *Server) projectD1Execution(loop freeStateReasoningLoop, session orchestration.PlanningSession, executeErr error) ChatResponse {
	if session.Execution == nil || len(session.Execution.Receipts) != 1 {
		return d1BlockedResponse(loop, firstNonEmpty(canaryErrorText(executeErr), "D1-S1 execution receipt missing"))
	}
	receipt := session.Execution.Receipts[0]
	receiptMap := structMap(receipt)
	for key, value := range receipt.Details {
		receiptMap[key] = value
	}
	receiptMap["idempotency_key"] = firstNonEmpty(firstStringFromMap(receipt.Details, "idempotency_key"), session.Execution.IdempotencyKey+":"+receipt.ActionID)
	admissionDomain := firstNonEmpty(firstStringFromMap(loop.Experiment.Admission.TypedAction, "action_domain", "domain"), experiment.D1S1ActionDomain)
	admissionKind := firstNonEmpty(firstStringFromMap(loop.Experiment.Admission.TypedAction, "action_kind", "kind"), experiment.D1S1ActionKind)
	round, _ := loop.Experiment.CurrentRound()
	if len(round.Interventions) == 0 {
		s.recordFreeStateExperimentAction(&loop, admissionDomain, receipt.Status, receiptMap)
	}
	if session.Execution.VerificationResult != nil {
		verification := session.Execution.VerificationResult
		if verification.Fresh && verification.PostAction && verification.ObservationID != "" && verification.ObservationRevision == receipt.AppliedRevision {
			round, _ := loop.Experiment.CurrentRound()
			obs := experiment.Observation{ID: verification.ObservationID, ReceiptID: "d1_ccb:" + verification.ObservationID,
				RequestedViewIDs: []string{"mix.multitrack_relationship"}, ExecutedViewIDs: []string{"mix.multitrack_relationship"}, ViewSetMatches: true,
				Fresh: true, PostAction: true, ProjectRevision: verification.ObservationRevision, EvidenceRefs: append([]string(nil), verification.EvidenceRefs...), RecordedAt: time.Now().UTC()}
			if len(round.Observations) == 0 || round.Observations[len(round.Observations)-1].ID != obs.ID {
				if events, err := loop.Experiment.RecordObservation(obs, true, time.Now().UTC()); err == nil {
					s.emitFreeStateExperimentEvents(events)
				}
			}
		}
	}
	verifiedPostAction := session.Execution.VerificationResult != nil && session.Execution.VerificationResult.Fresh && session.Execution.VerificationResult.PostAction && session.Execution.VerificationResult.ObservationRevision == receipt.AppliedRevision
	loop.RequiresPostActionObservation = !verifiedPostAction
	loop.Status = "re_evaluating"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	loop.LatestProjectChange = map[string]any{"project_revision": receipt.AppliedRevision, "revision": receipt.AppliedRevision, "freshness": "current_snapshot", "d1_execution_id": session.Execution.ID}
	loop.UpdatedAt = time.Now().UTC()
	syncD1Receipt(&loop)
	s.storeFreeStateLoop(loop)
	data := map[string]any{"schema_version": experiment.ImprovementReceiptSchema, "status": receipt.Status, "mutation_performed": receipt.Status == "applied",
		"action_domain": admissionDomain, "action_kind": admissionKind, "execution_id": session.Execution.ID,
		"execution_receipt": receiptMap, "verification": session.Execution.VerificationResult, "parameter_applied": receipt.Status == "applied",
		"readback_verified": receipt.Details["readback_verified"] == true, "evaluation_ready": verifiedPostAction,
		"human_audition_ready": false, "human_confirmed": false, "ambiguous": false, "rolled_back": false, "settled": false,
		"improvement_receipt": cloneContext(loop.D1Receipt)}
	reply := "D1-S1 track gain parameter was applied and read back. Fresh post-action evidence is recorded separately; acoustic materiality, target response, and human judgment remain pending."
	if admissionDomain == d1StaticEQDomain {
		reply = "D2-1 static EQ band parameter was applied and read back. Fresh post-action evidence is recorded separately; acoustic materiality, target response, and human judgment remain pending."
	}
	response := ChatResponse{ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID, Workflow: "free_state_d1_s1", WorkflowData: data,
		Reply:      reply,
		GoalStatus: string(agentruntime.StatusWaitingContinue), StopReason: "d1_post_action_evaluation_required"}
	if executeErr != nil && session.Status != orchestration.StatusNeedsReview {
		response.Error = executeErr.Error()
	}
	return response
}

func d1BlockedResponse(loop freeStateReasoningLoop, reason string) ChatResponse {
	return ChatResponse{ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID, Workflow: "free_state_d1_s1",
		WorkflowData: map[string]any{"status": "blocked", "mutation_performed": false}, GoalStatus: string(agentruntime.StatusFailed), StopReason: "d1_execution_blocked", Error: reason, Reply: reason}
}
