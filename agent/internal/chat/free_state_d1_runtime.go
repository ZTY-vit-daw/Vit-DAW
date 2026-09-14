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
	"vit-daw-agent/internal/harness"
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

// d1FailedActionKernelRevision observes the kernel's authoritative project
// revision after a fail-closed D1 execution. It exists as a package-level
// indirection so the failed-action attribution path stays unit-testable;
// production reads a live VSP state snapshot.
var d1FailedActionKernelRevision = func(ctx context.Context, client *kernel.Client) (string, bool) {
	if client == nil {
		return "", false
	}
	state, err := client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return "", false
	}
	return strings.TrimSpace(fmt.Sprint(state.Revision)), true
}

// D2-1.5 broadband_compression mirrors the experiment table row; the values
// live here only as routing labels (the table remains the bounds authority).
const d1BroadbandCompressionDomain = "broadband_compression"
const d1BroadbandCompressionKind = "broadband_threshold_adjust"

// FAM1-S1 de_esser mirrors the experiment table row as a routing label; the
// table remains the bounds authority.
const d1DeEsserDomain = "de_esser"
const d1DeEsserKind = "de_esser_threshold_adjust"

// FAM2-S1 transient_shaper mirrors the experiment table row as a routing
// label; the table remains the bounds authority.
const d1TransientShaperDomain = "transient_shaper"
const d1TransientShaperKind = "transient_attack_adjust"

// FAM4-S1 limiter mirrors the experiment table row as a routing label; the
// table remains the bounds authority.
const d1LimiterDomain = "limiter"
const d1LimiterKind = "limiter_ceiling_adjust"

// FAM5-S1 gate_expander mirrors the experiment table row as a routing label;
// the table remains the bounds authority.
const d1GateExpanderDomain = "gate_expander"
const d1GateExpanderKind = "gate_range_adjust"

// FAM6-S1 multiband mirrors the experiment table row as a routing label; the
// table remains the bounds authority.
const d1MultibandDomain = "multiband_dynamics"
const d1MultibandKind = "multiband_band_threshold_adjust"

// FAM3-S1 pan mirrors the experiment table row as a routing label; the table
// remains the bounds authority.
const d1PanDomain = "pan"
const d1PanKind = "track_pan_adjust"

// d1StaticEQCapabilityID identifies the bounded static EQ band adjustment in
// the frozen ActionSet/Proposal. The D1 execution path routes by the explicit
// mutation port (StaticEQVSPPort), so no capability registry registration is
// required; the id is audit metadata only (design record D2-1 S2a).
const d1StaticEQCapabilityID = "static_mix.static_eq.v0"

// defaultD1StaticEQPluginIdentifier is the plan-layer default EQ instance
// identifier used when the admission does not pin one. The StaticEQVSPPort
// loads it through rack_add_node only when the target track has no plugin_id
// yet.
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
	// journalSpec, when resolved, drives the same audit shape from any domain
	// table row; it supersedes the bool. Existing rows produce byte-identical
	// records either way.
	journalSpec *experiment.D1S1DomainSpec
}

func (p *d1JournalMutationPort) Preflight(ctx context.Context, set orchestration.ActionSet, cut orchestration.ProjectCut) error {
	return p.inner.Preflight(ctx, set, cut)
}

func (p *d1JournalMutationPort) Apply(ctx context.Context, action orchestration.Action, key string) (orchestration.ActionReceipt, error) {
	if _, exists := p.harness.JournalGet(action.ID); !exists {
		var record journal.Action
		if p.journalSpec != nil {
			record = d1JournalRecordForSpec(*p.journalSpec, action.ID, p.goalID, p.runID, action.TargetRef, action.Args)
		} else {
			record = d1JournalRecordForAction(p.staticEQ, action.ID, p.goalID, p.runID, action.TargetRef, action.Args)
		}
		p.harness.JournalRecord(record)
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
	if err := validateD1TierAdmission(loop.Experiment.Admission); err != nil {
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
	spec, _ := experiment.D1S1SpecForAction(experiment.D1S1ActionDomain, experiment.D1S1ActionKind)
	if loop.Experiment.Admission.IsD2MultiRound() {
		if err := d2MultiRoundCheckProjectedCumulativeDelta(loop.Experiment, spec.AdmissionValueKey, candidate.DeltaDB); err != nil {
			return orchestration.FrozenPlan{}, err
		}
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{
		State: d1StateResult(projectUUID, projectEpoch, snapshotHash, stateRevision, state), Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: append([]string(nil), loop.Experiment.Admission.EvidenceRefs...),
		TargetFingerprints:     []string{d1SubstituteFingerprint(spec.TargetFingerprintTemplate, targetID, fmt.Sprintf("%g", currentDB), "")},
		ContractVersions:       append([]string(nil), spec.ContractVersions...),
	})
	if err != nil {
		return orchestration.FrozenPlan{}, err
	}
	action := orchestration.Action{ID: "d1_" + sanitizeCanaryID(loop.Experiment.ID) + d1MultiRoundRoundScopeSuffix(loop) + spec.ActionIDSuffix, Command: experiment.D1S1ActionKind, TargetRef: targetID,
		BeforeFingerprint: d1SubstituteFingerprint(spec.BeforeFingerprintTemplate, targetID, fmt.Sprintf("%g", currentDB), ""),
		Args:              map[string]any{"delta_db": candidate.DeltaDB, "target_db": currentDB + candidate.DeltaDB}, Compensatable: true, IdempotencyClass: "effectively_once"}
	return d1AssembleFrozenPlan(loop, spec, action, targetID, cut)
}

// d1StaticEQPlan mirrors d1TrackGainPlan for the bounded static EQ band
// adjustment: one admitted action, one forward mutation, revision-bound cut.
// Plugin parameter before values are read by the port's Preflight; the plan
// layer never invokes kernel commands. Without a whitelist binding it keeps
// the historical stub schema verbatim; production execution resolves the
// binding first (resolveD1StaticEQWhitelistBinding) and calls
// d1StaticEQPlanWithBinding.
func d1StaticEQPlan(loop freeStateReasoningLoop, candidate agentloop.PendingMixTickCandidate, stateRevision int64, projectUUID, projectEpoch, snapshotHash string, state map[string]any) (orchestration.FrozenPlan, error) {
	return d1StaticEQPlanWithBinding(loop, candidate, stateRevision, projectUUID, projectEpoch, snapshotHash, state, nil)
}

func d1StaticEQPlanWithBinding(loop freeStateReasoningLoop, candidate agentloop.PendingMixTickCandidate, stateRevision int64, projectUUID, projectEpoch, snapshotHash string, state map[string]any, binding *d1StaticEQWhitelistBinding) (orchestration.FrozenPlan, error) {
	if loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return orchestration.FrozenPlan{}, fmt.Errorf("active D1-S1 experiment is required")
	}
	if err := validateD1TierAdmission(loop.Experiment.Admission); err != nil {
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
	args, paramID := d1StaticEQActionArgs(loop.Experiment.Admission.TypedAction, gainDB, binding)
	if stateRevision <= 0 || projectUUID == "" || projectEpoch == "" || snapshotHash == "" {
		return orchestration.FrozenPlan{}, fmt.Errorf("D2-1 requires a revision-bound VSP snapshot")
	}
	spec, _ := experiment.D1S1SpecForAction(d1StaticEQDomain, d1StaticEQKind)
	if loop.Experiment.Admission.IsD2MultiRound() {
		if err := d2MultiRoundCheckProjectedCumulativeDelta(loop.Experiment, spec.AdmissionValueKey, gainDB); err != nil {
			return orchestration.FrozenPlan{}, err
		}
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{
		State: d1StateResult(projectUUID, projectEpoch, snapshotHash, stateRevision, state), Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: append([]string(nil), loop.Experiment.Admission.EvidenceRefs...),
		TargetFingerprints:     []string{d1SubstituteFingerprint(spec.TargetFingerprintTemplate, targetID, "", paramID)},
		ContractVersions:       append([]string(nil), spec.ContractVersions...),
	})
	if err != nil {
		return orchestration.FrozenPlan{}, err
	}
	action := orchestration.Action{ID: "d1_" + sanitizeCanaryID(loop.Experiment.ID) + d1MultiRoundRoundScopeSuffix(loop) + spec.ActionIDSuffix, Command: d1StaticEQKind, TargetRef: targetID,
		BeforeFingerprint: d1SubstituteFingerprint(spec.BeforeFingerprintTemplate, targetID, "", paramID),
		Args:              args, Compensatable: true, IdempotencyClass: "effectively_once"}
	return d1AssembleFrozenPlan(loop, spec, action, targetID, cut)
}

// d1TrackPanPlan mirrors d1TrackGainPlan for the bounded track pan adjustment
// (FAM3-S1): one admitted action, one forward mutation, revision-bound cut.
// The pan move is delta-to-absolute like track_gain (the plan computes
// target_pan from the current snapshot readback plus the admitted delta), so
// an out-of-range result surfaces in the action's absolute target where the
// port's Preflight rejects it fail-closed before any kernel write.
func d1TrackPanPlan(loop freeStateReasoningLoop, candidate agentloop.PendingMixTickCandidate, stateRevision int64, projectUUID, projectEpoch, snapshotHash string, state map[string]any) (orchestration.FrozenPlan, error) {
	if loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return orchestration.FrozenPlan{}, fmt.Errorf("active D1-S1 experiment is required")
	}
	if err := validateD1TierAdmission(loop.Experiment.Admission); err != nil {
		return orchestration.FrozenPlan{}, err
	}
	spec, ok := experiment.D1S1DomainSpecFor(loop.Experiment.Admission)
	if !ok || spec.ActionDomain != d1PanDomain || spec.ActionKind != d1PanKind {
		return orchestration.FrozenPlan{}, fmt.Errorf("FAM3-S1 pan plan requires a pan admission")
	}
	if candidate.Operation != d1PanKind || strings.TrimSpace(candidate.TrackID) == "" || candidate.DeltaPan == 0 || math.Abs(candidate.DeltaPan) > 0.15 {
		return orchestration.FrozenPlan{}, fmt.Errorf("FAM3-S1 requires one bounded track_pan_adjust")
	}
	admittedDelta, ok := treatmentNumber(loop.Experiment.Admission.TypedAction, "delta_pan")
	if !ok || math.Abs(candidate.DeltaPan-admittedDelta) > 0.0001 {
		return orchestration.FrozenPlan{}, fmt.Errorf("FAM3-S1 candidate delta does not match the admitted observation-backed action")
	}
	targetID := firstStringFromMap(loop.Experiment.Admission.TargetRef, "id", "track_id")
	if candidate.TrackID != targetID {
		return orchestration.FrozenPlan{}, fmt.Errorf("FAM3-S1 candidate target does not match admitted observation target")
	}
	currentPan, ok := d1TrackPan(state, targetID)
	if !ok {
		return orchestration.FrozenPlan{}, fmt.Errorf("FAM3-S1 target pan readback is unavailable")
	}
	if stateRevision <= 0 || projectUUID == "" || projectEpoch == "" || snapshotHash == "" {
		return orchestration.FrozenPlan{}, fmt.Errorf("FAM3-S1 requires a revision-bound VSP snapshot")
	}
	// The D2-2 cumulative dose machine is dimensioned in dB
	// (d2MultiRoundChatCumulativeBoundDB); pan units are not dB, so a pan
	// admission stays single-round until the D2-2 card dimensions the
	// cumulative bound for normalized pan units. Fail closed rather than
	// reusing the dB bound with the wrong unit.
	if loop.Experiment.Admission.IsD2MultiRound() {
		return orchestration.FrozenPlan{}, fmt.Errorf("FAM3-S1 pan multi-round execution is not admitted: the D2-2 cumulative bound is dimensioned in dB, not pan units")
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{
		State: d1StateResult(projectUUID, projectEpoch, snapshotHash, stateRevision, state), Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: append([]string(nil), loop.Experiment.Admission.EvidenceRefs...),
		TargetFingerprints:     []string{d1SubstituteFingerprint(spec.TargetFingerprintTemplate, targetID, "", "fader_pan")},
		ContractVersions:       append([]string(nil), spec.ContractVersions...),
	})
	if err != nil {
		return orchestration.FrozenPlan{}, err
	}
	action := orchestration.Action{ID: "d1_" + sanitizeCanaryID(loop.Experiment.ID) + d1MultiRoundRoundScopeSuffix(loop) + spec.ActionIDSuffix, Command: d1PanKind, TargetRef: targetID,
		BeforeFingerprint: d1SubstituteFingerprint(spec.BeforeFingerprintTemplate, targetID, "", "fader_pan"),
		Args:              map[string]any{"delta_pan": candidate.DeltaPan, "target_pan": currentPan + candidate.DeltaPan}, Compensatable: true, IdempotencyClass: "effectively_once"}
	return d1AssembleFrozenPlan(loop, spec, action, targetID, cut)
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

// d1TrackPan reads one track's current pan position from the VSP snapshot
// state, mirroring d1TrackGain's row walk over the pan readback keys the
// kernel's track rows expose (pan/pan_value/balance — the same key family
// executionports.trackPan consumes).
func d1TrackPan(state map[string]any, trackID string) (float64, bool) {
	for _, row := range mapRowsFromAny(state["tracks"]) {
		if firstStringFromMap(row, "track_id", "id") != trackID {
			continue
		}
		return treatmentNumber(row, "pan", "pan_value", "balance")
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
	// The durable session is per round on the D2-2 tier: round 2 must not
	// reuse round 1's completed session (the exists branch would re-project
	// round 1's receipt as this round's intervention and break the per-round
	// transaction identity and revision chain). Round 1 keeps the historical
	// experiment-level id byte-for-byte.
	sessionID := "d1_session_" + sanitizeCanaryID(loop.Experiment.ID) + d1MultiRoundRoundScopeSuffix(loop)
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

// executeD1StaticEQ mirrors executeD1TrackGain for every PluginBound domain
// row (static_eq today, broadband_compression added by D2-1.5): the same
// durable session/plan/authorize/execute shape, with the parameter-driven
// StaticEQVSPPort (one idempotency key, one receipt, command name injected
// from the domain table) and the table-shaped journal variant. Per-domain
// wording comes from the whitelist section label and the experiment table;
// track_gain keeps its own function untouched.
func (s *Server) executeD1StaticEQ(ctx context.Context, conversationID string, req ChatRequest, candidate agentloop.PendingMixTickCandidate) (ChatResponse, bool) {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return ChatResponse{}, false
	}
	spec, specOK := experiment.D1S1DomainSpecFor(loop.Experiment.Admission)
	if !specOK || !spec.WriteBinding.PluginBound {
		return d1BlockedResponse(loop, "D1-S1 plugin-bound execution requires an admitted PluginBound domain"), true
	}
	// The experiment plugin whitelist is the execution gate for the real
	// plugin path (PCA promoted + fingerprint check included). It runs before
	// any other dependency work so a configuration or eligibility refusal is
	// cheap and carries its distinct boundary text; frozen-plan recovery
	// re-runs the same gate so a mutated whitelist cannot be applied onto a
	// session that was admitted under different approval evidence.
	binding, bindingErr := resolveD1PluginParamWhitelistBinding(loop.Experiment.Admission.TypedAction)
	if bindingErr != nil {
		return d1BlockedResponse(loop, bindingErr.Error()), true
	}
	if s.kernel == nil || s.harness == nil || s.orchestrationRuntime == nil || !s.orchestrationRuntime.HasDurableStore() {
		return d1BlockedResponse(loop, "D1-S1 durable execution dependencies are unavailable"), true
	}
	// Per-round durable session on the D2-2 tier, mirroring executeD1TrackGain:
	// round 2 owns its own session so round 1's completed session is never
	// re-projected as this round's execution; round 1 keeps the historical id.
	sessionID := "d1_session_" + sanitizeCanaryID(loop.Experiment.ID) + d1MultiRoundRoundScopeSuffix(loop)
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
		// Instance reuse discovery: the revision-bound VSP snapshot never
		// carries plugin rows (the kernel's compact snapshot strips them), so
		// the live plugin graph is read through the same governed legacy
		// surface the bridge uses for shadow refreshes. A read failure keeps
		// the historical instantiate path instead of blocking the experiment.
		existingPluginInstanceID := ""
		graphCtx, graphCancel := context.WithTimeout(ctx, 10*time.Second)
		graphReply, _, graphErr := s.kernel.SendCommand(graphCtx, map[string]any{"cmd": "get_project_state"})
		graphCancel()
		if graphErr == nil && strings.EqualFold(strings.TrimSpace(fmt.Sprint(graphReply["status"])), "ok") {
			existingPluginInstanceID = d1ExistingPluginInstanceID(graphReply, candidate.TrackID, binding.PluginName)
		}
		plan, err = d1PluginParamPlanWithBinding(loop, candidate, state.Revision, projectUUID, state.ProjectEpoch, state.SnapshotHash, state.LegacyState, binding, existingPluginInstanceID)
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
		session, err = s.orchestrationRuntime.ReconcileActionSet(ctx, sessionID, plan.ActionSet, &executionports.StaticEQVSPPort{Client: s.kernel, CommandName: spec.ActionKind}, verifier)
	case orchestration.StatusAuthorized:
		port := &d1JournalMutationPort{inner: &executionports.StaticEQVSPPort{Client: s.kernel, CommandName: spec.ActionKind}, harness: s.harness, goalID: loop.GoalID, runID: loop.RunID, staticEQ: true, journalSpec: &spec}
		session, err = s.orchestrationRuntime.ExecuteActionSetWithPersistence(ctx, sessionID, plan.ActionSet, plan.ProjectCut, port, verifier,
			executionports.ProjectHistory{Harness: s.harness, GoalID: loop.GoalID, RunID: loop.RunID})
	case orchestration.StatusCompleted, orchestration.StatusNeedsReview:
		// Durable terminal execution is projected below without another mutation.
	default:
		return d1BlockedResponse(loop, "D1-S1 durable session is not executable at status "+string(session.Status)), true
	}
	return s.projectD1Execution(loop, session, err), true
}

// executeD1TrackPan mirrors executeD1TrackGain for the pan domain row (the
// first non-PluginBound domain after track_gain): the same durable
// session/plan/authorize/execute shape with the pan-native TrackPanVSPPort
// (set_pan CAS, revision advance gate, value readback) and the table-shaped
// journal variant. track_gain keeps its own function untouched.
func (s *Server) executeD1TrackPan(ctx context.Context, conversationID string, req ChatRequest, candidate agentloop.PendingMixTickCandidate) (ChatResponse, bool) {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return ChatResponse{}, false
	}
	spec, specOK := experiment.D1S1DomainSpecFor(loop.Experiment.Admission)
	if !specOK || spec.WriteBinding.PluginBound || spec.ActionKind != d1PanKind {
		return d1BlockedResponse(loop, "FAM3-S1 pan execution requires an admitted pan domain"), true
	}
	if s.kernel == nil || s.harness == nil || s.orchestrationRuntime == nil || !s.orchestrationRuntime.HasDurableStore() {
		return d1BlockedResponse(loop, "D1-S1 durable execution dependencies are unavailable"), true
	}
	// Per-round durable session on the D2-2 tier, mirroring executeD1TrackGain:
	// round 2 owns its own session so round 1's completed session is never
	// re-projected as this round's execution; round 1 keeps the historical id.
	sessionID := "d1_session_" + sanitizeCanaryID(loop.Experiment.ID) + d1MultiRoundRoundScopeSuffix(loop)
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
		plan, err = d1TrackPanPlan(loop, candidate, state.Revision, projectUUID, state.ProjectEpoch, state.SnapshotHash, state.LegacyState)
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
	verifier := executionverifiers.StaticPanBalance{State: s.kernel, Acoustic: executionverifiers.HarnessAcoustic{Invoker: s.harness,
		PreviousObservationID: plan.PreviousObservationID, RequireExplicitFresh: true, MixSessionID: sessionID, GoalText: loop.OriginalIntent}}
	switch session.Status {
	case orchestration.StatusExecuting, orchestration.StatusVerifying:
		session, err = s.orchestrationRuntime.ReconcileActionSet(ctx, sessionID, plan.ActionSet, &executionports.TrackPanVSPPort{Client: s.kernel}, verifier)
	case orchestration.StatusAuthorized:
		port := &d1JournalMutationPort{inner: &executionports.TrackPanVSPPort{Client: s.kernel}, harness: s.harness, goalID: loop.GoalID, runID: loop.RunID, journalSpec: &spec}
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
	ctx := context.Background()
	receipt := session.Execution.Receipts[0]
	receiptMap := structMap(receipt)
	for key, value := range receipt.Details {
		receiptMap[key] = value
	}
	receiptMap["idempotency_key"] = firstNonEmpty(firstStringFromMap(receipt.Details, "idempotency_key"), session.Execution.IdempotencyKey+":"+receipt.ActionID)
	if strings.TrimSpace(receipt.AppliedRevision) != "" {
		// Book the mutation's revision advance even for unreconciled receipts:
		// the kernel state moved, so the closure must not treat the next
		// post-action observation as external drift.
		s.syncAudioClosureGovernedRevision(loop.ConversationID, receipt.AppliedRevision)
	} else if revision, ok := d1FailedActionKernelRevision(ctx, s.kernel); ok {
		// A fail-closed action can still have advanced the kernel revision
		// (plugin instance load plus probe/restore writes). Attributing that
		// advance to the loop's own failed action keeps the closure settling
		// the failure on its real error instead of mislabeling the next
		// replayed observation as external revision drift (2026-09-02 p03
		// runs, stats §47).
		s.syncAudioClosureFailedActionRevision(loop.ConversationID, revision)
	}
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
				RequestedViewIDs: d1ObservationViewIDsFor(loop.Experiment.Admission), ExecutedViewIDs: d1ObservationViewIDsFor(loop.Experiment.Admission), ViewSetMatches: true,
				Fresh: true, PostAction: true, ProjectRevision: verification.ObservationRevision, EvidenceRefs: append([]string(nil), verification.EvidenceRefs...), RecordedAt: time.Now().UTC()}
			if len(round.Observations) == 0 || round.Observations[len(round.Observations)-1].ID != obs.ID {
				if events, err := loop.Experiment.RecordObservation(obs, true, time.Now().UTC()); err == nil {
					s.emitFreeStateExperimentEvents(events)
				}
			}
		}
	}
	verifiedPostAction := (session.Execution.VerificationResult != nil && session.Execution.VerificationResult.Fresh && session.Execution.VerificationResult.PostAction && session.Execution.VerificationResult.ObservationRevision == receipt.AppliedRevision) ||
		freeStateExperimentRoundHasFreshPostActionObservation(loop.Experiment)
	if !verifiedPostAction && s.bookD1PostActionObservation(ctx, &loop, session, receipt) {
		// The deterministic in-respond booking below is the 201842 path
		// generalized: the admitted experiment's verification plan (the
		// model-selected view set on the admitted target) is executed once
		// server-side right after the mutation, so the post-apply chain does
		// not depend on a model slice surviving budget/turn pressure to
		// produce the mandatory observation.
		verifiedPostAction = true
	}
	loop.RequiresPostActionObservation = !verifiedPostAction
	if loop.RequiresPostActionObservation {
		// The post-apply chain (fresh CCB observation -> materiality/target
		// evaluation -> judgment boundary) runs on scheduler continuations after
		// this respond parks the goal. Grant its phase-scoped budget floor here
		// so pre-apply consumption cannot starve it.
		reserveD1PostApplySlices(&loop)
	} else if receipt.Status == "applied" && freeStateLoopOwesAutoSettlement(loop) {
		// TRAJ-AUTO-SETTLE-1：观察已在响应链内确定性入账的形态下，回合仍欠同一
		// 段 chain 的尾段（materiality/目标评估 -> 判定边界）。尾段是纯机器工作，
		// 但此前拿不到地板：预算走满即预算停止，该 goal 的 durable 记录被全部
		// terminal 化，链终局于是向用户索要一句「继续」——把机器欠账转嫁成用户
		// 交互（卡面 ①类）。这里授同一段地板，让调度器自动续跑结算片；判定驻留
		// （有未决 A/B 卡）由 freeStateLoopOwesAutoSettlement 排除——那时唯一能
		// 收口的是用户的判定 POST，不是自动切片。
		reserveD1PostApplySlices(&loop)
	}
	loop.Status = "re_evaluating"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	loop.LatestProjectChange = map[string]any{"project_revision": receipt.AppliedRevision, "revision": receipt.AppliedRevision, "freshness": "current_snapshot", "d1_execution_id": session.Execution.ID}
	loop.UpdatedAt = time.Now().UTC()
	syncD1Receipt(&loop)
	s.storeFreeStateLoop(loop)
	if receipt.Status == "applied" && loop.Experiment.Admission.IsD1S1() {
		// D1-AUDITION-GAP-1: mount the A/B audition card the moment the
		// intervention lands, not only at the judgment boundary. The user's
		// product ruling is "I only do the A/B listening" — every intervention
		// applied to the project must end in an A/B card whatever happens to
		// the chain afterwards. The 2026-09-13 22:58 real stack (goal_c7ecb4fb)
		// stranded exactly there: the apply turn completed the goal before the
		// evaluation slice ran, the round never reached its judgment boundary,
		// and no audition.* event ever fired. prepareFreeStateAudition is the
		// existing D1 mounting face (before render from loop.D1State, after
		// render at the applied revision, audition.ready + judgment request);
		// it is idempotent per session (loop.AuditionSessionID early-exit) and
		// the boundary-time call double-fire-deduplicates, so mounting here
		// cannot duplicate the card. Fail-open like B12-2: a degraded mount
		// never blocks the already-applied step.
		if auditionErr := s.prepareFreeStateAudition(ctx, &loop); auditionErr != nil && s.logger != nil {
			s.logger.Warn("[audition] post-apply mount degraded conversation=%s goal=%s err=%v", loop.ConversationID, loop.GoalID, auditionErr)
		}
	}
	data := map[string]any{"schema_version": experiment.ImprovementReceiptSchema, "status": receipt.Status, "mutation_performed": receipt.Status == "applied",
		"action_domain": admissionDomain, "action_kind": admissionKind, "execution_id": session.Execution.ID,
		"execution_receipt": receiptMap, "verification": session.Execution.VerificationResult, "parameter_applied": receipt.Status == "applied",
		"readback_verified": receipt.Details["readback_verified"] == true, "evaluation_ready": verifiedPostAction,
		"human_audition_ready": false, "human_confirmed": false, "ambiguous": false, "rolled_back": false, "settled": false,
		"improvement_receipt": cloneContext(loop.D1Receipt)}
	// B6 缺陷①：应用回复由 chat 侧五要素 composer 动态生成（目标轨/参数/
	// 变更量与方向/针对的发现/去哪看），不再消费实验域表的静态 AppliedReplyText
	// ——静态模板泄漏内部代号（FAM3-S1）且预支「人工判定仍待完成」承诺。
	reply := d1AppliedReportReply(loop, receiptMap)
	response := ChatResponse{ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID, Workflow: "free_state_d1_s1", WorkflowData: data,
		Reply:      reply,
		GoalStatus: string(agentruntime.StatusWaitingContinue), StopReason: "d1_post_action_evaluation_required"}
	if executeErr != nil && session.Status != orchestration.StatusNeedsReview {
		response.Error = executeErr.Error()
	}
	return response
}

// d1BlockedResponseLogger is the server hook for the blocked-response
// boundary. The closure settlement projection can bury the response's Error
// field before it reaches any surface (2026-09-05 1032 forensics: four runs
// fail-closed with the reason invisible end to end); the hook keeps every
// blocked D1 execution explainable from the agent log alone.
var d1BlockedResponseLogger func(conversationID, reason string)

func d1BlockedResponse(loop freeStateReasoningLoop, reason string) ChatResponse {
	if d1BlockedResponseLogger != nil {
		d1BlockedResponseLogger(loop.ConversationID, reason)
	}
	return ChatResponse{ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID, Workflow: "free_state_d1_s1",
		WorkflowData: map[string]any{"status": "blocked", "mutation_performed": false}, GoalStatus: string(agentruntime.StatusFailed), StopReason: "d1_execution_blocked", Error: reason, Reply: reason}
}

// bookD1PostActionObservation executes the admitted D1 experiment's
// verification plan deterministically inside the applied respond chain: one
// CCB observation request with the model-selected view set (the round's
// RequestedViewIDs from admission) on the admitted target, after the mutation,
// at the applied revision. It books the resulting fresh bundle into the round
// through the same RecordObservation path the model-requested evidence uses,
// so the settle turn can proceed without burning a continuation slice on
// producing the observation (2026-08-28 10:27/11:40/11:46 smokes: post-apply
// slices ran out of turn budget before the model ever issued the request).
// Guards kept: the view set is the model's own admitted plan (no server view
// inference), the booking requires a new observation id, non-stale explicit
// freshness, and an exact revision match against the applied receipt.
func (s *Server) bookD1PostActionObservation(ctx context.Context, loop *freeStateReasoningLoop, session orchestration.PlanningSession, receipt orchestration.ActionReceipt) bool {
	if s == nil || s.harness == nil || loop == nil || loop.Experiment == nil || strings.TrimSpace(receipt.AppliedRevision) == "" {
		return false
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil || len(round.RequestedViewIDs) == 0 {
		return false
	}
	if freeStateExperimentRoundHasFreshPostActionObservation(loop.Experiment) {
		return false
	}
	args := map[string]any{
		"view_ids":        append([]string(nil), round.RequestedViewIDs...),
		"target_ref":      cloneContext(loop.Experiment.Admission.TargetRef),
		"freshness_class": "post_action",
		"mix_session_id":  session.ID,
	}
	if session.FrozenPlan != nil && strings.TrimSpace(session.FrozenPlan.PreviousObservationID) != "" {
		args["previous_observation"] = session.FrozenPlan.PreviousObservationID
	}
	response, invokeErr := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:    "ccb.observation_request",
		Args:    args,
		Context: map[string]any{"observation_only": true, "free_state_experiment": true},
		Source:  "free_state_d1_post_action", GoalID: loop.GoalID, RunID: loop.RunID,
		ToolCallID: "d1_post_action:" + sanitizeCanaryID(loop.Experiment.ID),
	})
	if invokeErr != nil || !strings.EqualFold(strings.TrimSpace(response.Status), "ok") {
		if s.logger != nil {
			s.logger.Warn("[free-state-experiment] deterministic post-action observation unavailable: %v %s", invokeErr, firstNonEmpty(response.Error, response.Status))
		}
		return false
	}
	// The harness returns the typed capabilitycontext bundle struct; project it
	// through its JSON shape so the field lookups below see the tagged keys.
	bundle := structMap(response.Result["bundle"])
	if len(bundle) == 0 || !strings.EqualFold(strings.TrimSpace(firstStringFromMap(bundle, "status")), "ready") {
		if s.logger != nil {
			s.logger.Warn("[free-state-experiment] deterministic post-action observation was not ready: status=%s", firstNonEmpty(firstStringFromMap(bundle, "status"), firstStringFromMap(response.Result, "status")))
		}
		return false
	}
	observationID := firstStringFromMap(bundle, "observation_id")
	if observationID == "" || (session.FrozenPlan != nil && observationID == session.FrozenPlan.PreviousObservationID) {
		return false
	}
	freshness := firstMapFromAny(bundle["freshness"])
	binding := firstMapFromAny(bundle["project_binding"])
	revision := firstNonEmpty(firstStringFromMap(freshness, "project_revision"), firstStringFromMap(binding, "project_revision"))
	if revision != receipt.AppliedRevision || strings.EqualFold(strings.TrimSpace(firstStringFromMap(freshness, "status")), "stale") {
		if s.logger != nil {
			s.logger.Warn("[free-state-experiment] deterministic post-action observation revision/freshness mismatch: revision=%s want=%s freshness=%s", revision, receipt.AppliedRevision, firstStringFromMap(freshness, "status"))
		}
		return false
	}
	executed := freeStateStringSlice(firstMapFromAny(bundle["audit_receipt"])["actual_executed_view_ids"])
	if len(executed) == 0 {
		executed = freeStateStringSlice(bundle["requested_views"])
	}
	if len(executed) == 0 {
		executed = append([]string(nil), round.RequestedViewIDs...)
	}
	evidence := freeStateStringSlice(bundle["evidence_refs"])
	if len(evidence) == 0 {
		evidence = []string{observationID}
	}
	observation := experiment.Observation{
		ID: observationID, ReceiptID: "d1_ccb:" + observationID,
		RequestedViewIDs: append([]string(nil), round.RequestedViewIDs...), ExecutedViewIDs: executed, ViewSetMatches: true,
		Fresh: true, PostAction: true, ProjectRevision: revision, EvidenceRefs: evidence, RecordedAt: time.Now().UTC(),
	}
	if events, recordErr := loop.Experiment.RecordObservation(observation, true, time.Now().UTC()); recordErr == nil {
		s.emitFreeStateExperimentEvents(events)
		// Project the booked evidence into the loop's observation ledger so the
		// next model turn sees the fresh post-action bundle it is supposed to
		// settle from; without this the settle turns keep requesting the
		// observation the server already produced.
		recent := &agentloop.RecentObservation{
			ToolCallID: "d1_post_action:" + sanitizeCanaryID(loop.Experiment.ID), Tool: "ccb.observation_request", CommandName: "ccb_observation_request",
			Status: "ready",
			Summary: map[string]any{
				"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": observationID,
				"requested_views": append([]string(nil), round.RequestedViewIDs...), "actual_executed_view_ids": executed,
				"views": cloneContext(firstMapFromAny(bundle["views"])), "freshness": cloneContext(freshness), "bundle": bundle,
				"evidence_refs": evidence, "target_ref": cloneContext(firstMapFromAny(bundle["target_ref"])),
			},
		}
		loop.ObservationLedger = mergeFreeStateObservationLedger(loop.ObservationLedger, recent, loop.Cycle)
		if durableLatest := freeStateLatestObservationFromLedger(loop.ObservationLedger); durableLatest != nil {
			loop.LatestObservation = durableLatest
		}
		if !freeStateContainsString(loop.ObservationIDs, observationID) {
			loop.ObservationIDs = append(loop.ObservationIDs, observationID)
		}
		return true
	} else if s.logger != nil {
		s.logger.Warn("[free-state-experiment] deterministic post-action observation rejected by the round: %v", recordErr)
	}
	return false
}
