package chat

// UX-AB-AUDITION-IMPL-1（2026-10）：语义动态执行（executeSemanticDynamicTicket）
// 的原生 A/B 对照试听接线。模式模板是 mix_tick_audition.go（B12-2），纪律同源：
//
//  1. **时序**：before 渲染在参数写（settlement+apply 整块）之前解析，after 在其
//     之后；时序由 runSemanticDynamicAuditionBracket 单点表达（复刻
//     runMixTickAuditionBracket 的结构性质，而非两处代码的巧合）。
//  2. **fail-open**：任一相渲染失败只降级（before/after/prepare failed），绝不动
//     用户已确认的 mutation 腿；settlement 自身的 fail-closed 语义原样保留——
//     mutate 报错时不 mount、既有失败响应逐字不变。
//  3. **会话复用**：播放走既有内核 audition 会话（audition.prepare + audio_file
//     候选），身份族 audition:dynamic:<ticketID> / dynamic_turn: / dynamic_round:
//     与实验域（audition:<expID>:<roundID>）和 mix-tick 域（audition:mix_tick:*）
//     字面互不冲突；判定经 /agent/audition/judgment 的第三身份支路落账。
//  4. **实验契约零接触**：不碰 loop.Experiment、不写实验回合证据、不进实验
//     状态机；判定落 user_judgment 证据到 conversation/trajectory（自有身份域），
//     只落账+记录 disposition——物理回滚参数不在首版范围（卡面声明）。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/trajectory"
)

const (
	semanticDynamicAuditionSchemaVersion = "vit.semantic_dynamic_audition.v1"

	semanticDynamicAuditionPhaseBefore = "before"
	semanticDynamicAuditionPhaseAfter  = "after"

	semanticDynamicAuditionStatusReady = "ready"

	semanticDynamicAuditionRenderDir = "semantic_dynamic_audition"
	semanticDynamicAuditionSource    = "semantic_dynamic_audition"
)

// semanticDynamicAuditionRecord is the conversation-scoped state of a mounted
// semantic-dynamic A/B card, mirroring mixTickAuditionRecord: in-memory only,
// the rendered WAVs and their sha256 are the durable artifacts.
type semanticDynamicAuditionRecord struct {
	SchemaVersion   string
	ConversationID  string
	GoalID          string
	RunID           string
	SessionID       string
	TurnID          string
	RoundID         string
	Status          string
	TicketID        string
	Family          string
	ProcessorType   string
	TrackID         string
	PluginID        string
	ProjectRef      string
	ProjectUUID     string
	ProjectRevision string
	Blind           bool
	BlindSwap       bool
	JudgmentPending bool
	Session         map[string]any
	Before          map[string]any
	After           map[string]any
	UpdatedAt       time.Time
}

// semanticDynamicAuditionPlan carries everything the two renders and the
// session prepare need. BeforeRevision is resolved before the mutation runs
// (pre-write kernel revision); AfterRevision is a closure because the
// post-write revision only exists after the settlement/apply block has landed.
type semanticDynamicAuditionPlan struct {
	ConversationID string
	GoalID         string
	RunID          string
	TicketID       string
	Family         string
	ProcessorType  string
	TrackID        string
	PluginID       string
	BeforeRevision string
	AfterRevision  func() string
}

// semanticDynamicAuditionBracket is what the ordering bracket hands back to
// the report composer: a mounted card, or the degraded reason that kept it
// off. A mutation error is reported separately (bracket zero value + err) so
// the caller keeps the historical failure contract.
type semanticDynamicAuditionBracket struct {
	Record   *semanticDynamicAuditionRecord
	Degraded string
}

func semanticDynamicAuditionSessionID(conversationID, ticketID string) string {
	return "audition:dynamic:" + sanitizeCanaryID(firstNonEmpty(ticketID, conversationID))
}

func semanticDynamicAuditionTurnID(conversationID, ticketID string) string {
	return "dynamic_turn:" + sanitizeCanaryID(firstNonEmpty(ticketID, conversationID))
}

func semanticDynamicAuditionRoundID(ticketID, trackID string) string {
	return "dynamic_round:" + sanitizeCanaryID(firstNonEmpty(ticketID, trackID))
}

// semanticTopologyAuditionFamily marks the direct bus-topology leg channel on
// the mounted record and its judgment outcome (UX-AB-AUDITION-COVERAGE-1);
// it never collides with the ticket families (processorintent families).
const semanticTopologyAuditionFamily = "topology_bus"

// semanticTopologyAuditionTicketID namespaces the bus-topology legs' audition
// inside the audition:dynamic: family: the parked plan id is unique per
// confirmation, so the identity is stable for a re-executed plan and
// collision-free against ticket sessions in the same conversation.
func semanticTopologyAuditionTicketID(planID string) string {
	return "topo_" + firstNonEmpty(strings.TrimSpace(planID), "plan_"+randomID())
}

// semanticTopologyAuditionPlan builds the bracket plan for the non-ticket
// effect mutations of the bus topology legs (send routing + plugin load on the
// FX track). A plan without a conversation identity cannot mount a
// user-attributable card, so it answers nil and the caller runs the legs
// unbracketed (the historical shape) instead of mounting an anonymous session.
func (s *Server) semanticTopologyAuditionPlan(ctx context.Context, plan *PendingPlan, topologyPlan *semanticTopologyPlan) *semanticDynamicAuditionPlan {
	if s == nil || plan == nil || topologyPlan == nil || len(topologyPlan.Sends) == 0 {
		return nil
	}
	conversationID := firstNonEmpty(
		cleanContextText(plan.WorkflowData["conversation_id"]),
		cleanContextText(plan.Context["conversation_id"]),
	)
	if conversationID == "" {
		return nil
	}
	goalID, runID := goalIDsFromContext(plan.Context)
	processorType := ""
	for _, load := range topologyPlan.Loads {
		if strings.TrimSpace(load.PluginName) != "" {
			processorType = load.PluginName
			break
		}
	}
	if processorType == "" {
		processorType = firstNonEmpty(firstNonEmptyText(plan.WorkflowData, "plugin_name"), topologyPlan.FXTrack.Name)
	}
	return &semanticDynamicAuditionPlan{
		ConversationID: conversationID,
		GoalID:         goalID,
		RunID:          runID,
		TicketID:       semanticTopologyAuditionTicketID(plan.ID),
		Family:         semanticTopologyAuditionFamily,
		ProcessorType:  processorType,
		TrackID:        topologyPlan.Sends[0].SourceTrackID,
		BeforeRevision: s.semanticDynamicAuditionLiveRevision(ctx),
		AfterRevision:  func() string { return s.semanticDynamicAuditionLiveRevision(ctx) },
	}
}

// semanticDynamicAuditionLiveRevision resolves the kernel project revision
// through the same VSP state channel the settlement bracket's before snapshot
// uses (semanticSettlementStateClient), so an audition render and the
// settlement evidence can never disagree about which revision a WAV was bound
// to. An unavailable channel answers "" and the before phase degrades open.
func (s *Server) semanticDynamicAuditionLiveRevision(ctx context.Context) string {
	if s == nil {
		return ""
	}
	state := s.semanticSettlementStateClient()
	if state == nil {
		return ""
	}
	snapshot, err := state.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || snapshot == nil || !snapshot.OK() || snapshot.Revision <= 0 {
		return ""
	}
	return strconv.FormatInt(snapshot.Revision, 10)
}

// runSemanticDynamicAuditionBracket is the single expression of the A/B order:
// resolve the pre-write render, run the mutation (settlement bracket + typed
// controller + validation + settlement close, exactly as the unbracketed path
// ran them), resolve the post-write render, mount the session. Render and
// prepare failures degrade open; the mutate error is reported back to the
// caller so the existing failure response stays byte-for-byte.
func (s *Server) runSemanticDynamicAuditionBracket(ctx context.Context, plan *semanticDynamicAuditionPlan, mutate func() error) (semanticDynamicAuditionBracket, error) {
	if s == nil || plan == nil {
		return semanticDynamicAuditionBracket{}, nil
	}
	before, beforeErr := s.resolveSemanticDynamicAuditionRender(ctx, plan, semanticDynamicAuditionPhaseBefore, plan.BeforeRevision)
	if beforeErr != nil {
		// Fail-open on the pre-write phase: the confirmed mutation still runs.
		s.logSemanticDynamicAuditionDegraded(plan, semanticDynamicAuditionPhaseBefore, beforeErr)
		before = nil
	}

	if err := mutate(); err != nil {
		// The parameter write did not land: no post-write render, no card.
		return semanticDynamicAuditionBracket{}, err
	}

	if before == nil {
		return semanticDynamicAuditionBracket{Degraded: "before_render_failed"}, nil
	}

	afterRevision := ""
	if plan.AfterRevision != nil {
		afterRevision = strings.TrimSpace(plan.AfterRevision())
	}
	after, afterErr := s.resolveSemanticDynamicAuditionRender(ctx, plan, semanticDynamicAuditionPhaseAfter, afterRevision)
	if afterErr != nil {
		s.logSemanticDynamicAuditionDegraded(plan, semanticDynamicAuditionPhaseAfter, afterErr)
		return semanticDynamicAuditionBracket{Degraded: "after_render_failed"}, nil
	}

	record, prepareErr := s.mountSemanticDynamicAudition(ctx, plan, before, after)
	if prepareErr != nil {
		s.logSemanticDynamicAuditionDegraded(plan, "prepare", prepareErr)
		return semanticDynamicAuditionBracket{Degraded: "audition_prepare_failed"}, nil
	}
	return semanticDynamicAuditionBracket{Record: record}, nil
}

// resolveSemanticDynamicAuditionRender is the one seam both phases and the
// ordering pin go through: an override replaces what a phase returns, never
// when it runs — the pair's order lives in the bracket above.
func (s *Server) resolveSemanticDynamicAuditionRender(ctx context.Context, plan *semanticDynamicAuditionPlan, phase, projectRevision string) (map[string]any, error) {
	if s.semanticDynamicAuditionRenderOverride != nil {
		return s.semanticDynamicAuditionRenderOverride(ctx, phase, projectRevision, plan)
	}
	return s.renderSemanticDynamicAuditionPhase(ctx, plan.ConversationID, phase, projectRevision)
}

func (s *Server) logSemanticDynamicAuditionDegraded(plan *semanticDynamicAuditionPlan, phase string, err error) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Warn("[semantic.dynamic.audition] degraded phase=%s conversation=%s ticket=%s err=%v",
		phase, plan.ConversationID, plan.TicketID, err)
}

// renderSemanticDynamicAuditionPhase performs one side of the pair with the
// same start_render wait pattern as renderMixTickAuditionPhase (start_render
// with 24-bit master-plugin rendering into a revision-named WAV, wait on the
// job, then sha256 the payload for provenance); the low-level helpers
// (validD1RenderFile / sameD1RenderPath / hashD1RenderFile) are shared
// verbatim.
func (s *Server) renderSemanticDynamicAuditionPhase(ctx context.Context, conversationID, phase, projectRevision string) (map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("semantic-dynamic audition render dependencies are unavailable")
	}
	phase = strings.ToLower(strings.TrimSpace(phase))
	if phase != semanticDynamicAuditionPhaseBefore && phase != semanticDynamicAuditionPhaseAfter {
		return nil, fmt.Errorf("semantic-dynamic audition render phase must be before or after")
	}
	projectRevision = strings.TrimSpace(projectRevision)
	if projectRevision == "" {
		return nil, fmt.Errorf("semantic-dynamic audition render must be revision-bound")
	}
	if s.kernel == nil || s.harness == nil {
		return nil, fmt.Errorf("semantic-dynamic audition render dependencies are unavailable")
	}
	root := filepath.Join(s.artifactStore().Root, semanticDynamicAuditionRenderDir, sanitizeCanaryID(conversationID))
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create semantic-dynamic audition render directory: %w", err)
	}
	path := filepath.Join(root, phase+"_revision_"+sanitizeCanaryID(projectRevision)+".wav")
	if validD1RenderFile(path) {
		return finishSemanticDynamicAuditionRender(phase, projectRevision, path)
	}
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove incomplete semantic-dynamic audition render: %w", err)
		}
	}
	reply, _, err := s.kernel.SendCommand(ctx, map[string]any{"cmd": "start_render", "file_path": path, "bit_depth": 24, "use_master_plugins": true})
	if err != nil {
		return nil, fmt.Errorf("start semantic-dynamic audition %s render: %w", phase, err)
	}
	if strings.EqualFold(firstStringFromMap(reply, "status"), "error") {
		return nil, fmt.Errorf("start semantic-dynamic audition %s render: %s", phase, firstNonEmpty(firstStringFromMap(reply, "message"), "kernel rejected render"))
	}
	jobID := firstStringFromMap(reply, "job_id")
	if jobID == "" {
		return nil, fmt.Errorf("start semantic-dynamic audition %s render returned no job_id", phase)
	}
	result, waitErr := s.harness.WaitRender(ctx, jobID)
	if waitErr != nil && !validD1RenderFile(path) {
		return nil, fmt.Errorf("wait for semantic-dynamic audition %s render %s: %w", phase, jobID, waitErr)
	}
	if waitErr == nil {
		if result.Status != "ready" {
			return nil, fmt.Errorf("semantic-dynamic audition %s render %s failed: %s", phase, jobID, firstNonEmpty(result.Error, result.Status))
		}
		if result.FilePath != "" && !sameD1RenderPath(result.FilePath, path) {
			return nil, fmt.Errorf("semantic-dynamic audition %s render %s completed for an unexpected file", phase, jobID)
		}
	}
	if !validD1RenderFile(path) {
		return nil, fmt.Errorf("semantic-dynamic audition %s render %s produced no valid WAV", phase, jobID)
	}
	return finishSemanticDynamicAuditionRender(phase, projectRevision, path)
}

func finishSemanticDynamicAuditionRender(phase, projectRevision, path string) (map[string]any, error) {
	digest, size, err := hashD1RenderFile(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"schema_version":     d1RenderSchema,
		"phase":              phase,
		"status":             "ready",
		"file_path":          filepath.Clean(path),
		"project_revision":   projectRevision,
		"sha256":             digest,
		"size_bytes":         size,
		"range":              "full_project",
		"bit_depth":          24,
		"use_master_plugins": true,
		"render_revision":    "semantic_dynamic_render:" + phase + ":" + projectRevision + ":" + digest[:16],
		"preview_revision":   "sha256:" + digest,
		"completed_at":       time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

// mountSemanticDynamicAudition prepares the kernel session for the pair,
// records the conversation-scoped state, and raises the two events the WebUI
// A/B card is built from: audition.ready renders the card, and the
// trajectory.user_judgment.requested node arms its judgment seats (the WebUI
// canJudge flag is only set by that event). turn_id is this channel's native
// turn domain (dynamic_turn:*) — the judgment POST echoes it back, so the
// server-side identity checks and the card badges agree on one vocabulary.
func (s *Server) mountSemanticDynamicAudition(ctx context.Context, plan *semanticDynamicAuditionPlan, before, after map[string]any) (*semanticDynamicAuditionRecord, error) {
	if s == nil || plan == nil {
		return nil, fmt.Errorf("semantic-dynamic audition dependencies are unavailable")
	}
	if s.auditionKernel == nil {
		return nil, fmt.Errorf("audition kernel unavailable")
	}
	sessionID := semanticDynamicAuditionSessionID(plan.ConversationID, plan.TicketID)
	turnID := semanticDynamicAuditionTurnID(plan.ConversationID, plan.TicketID)
	roundID := semanticDynamicAuditionRoundID(plan.TicketID, plan.TrackID)
	projectRevision := firstStringFromMap(after, "project_revision")

	projectRef := ""
	projectUUID := ""
	if s.auditionCandidateDriver != nil {
		if plane, planeErr := s.auditionCandidateDriver.CurrentPlane(ctx); planeErr == nil {
			projectRef = plane.ProjectPath
			projectUUID = firstNonEmpty(projectUUID, plane.ProjectUUID)
		}
	}
	projectRef = firstNonEmpty(projectRef, "project:active")

	// The blind tier is the same configuration face AUDITION-PLAY-1 hardened
	// (env wins, then VitApp/Workspace/agent_runtime_config.json, read per
	// session); the draw happens once per mounted session.
	settings := auditionBlindSettingsFor()
	blindSwap := false
	if settings.enabled {
		blindSwap = s.drawAuditionBlind()
	}
	if s.logger != nil {
		s.logger.Info("[semantic.dynamic.audition] blind tier %s session=%s swapped=%v", settings.source, sessionID, blindSwap)
	}

	baselineCommit := "semantic_dynamic:" + plan.TicketID + ":" + semanticDynamicAuditionPhaseBefore
	treatmentCommit := "semantic_dynamic:" + plan.TicketID + ":" + semanticDynamicAuditionPhaseAfter
	candidates, err := d1AuditionCandidatesForAssignment(before, after, baselineCommit, treatmentCommit, projectRef, projectUUID, blindSwap)
	if err != nil {
		return nil, err
	}
	request := kernel.AuditionSessionRequest{
		ConversationID: plan.ConversationID, SessionID: sessionID, Scope: "target",
		ActiveProjectRef: projectRef, ActiveProjectRevision: projectRevision, TimelineRevision: projectRevision,
		Candidates: candidates,
	}
	result, callErr := s.auditionKernel.AuditionPrepare(ctx, request)
	session := auditionReplySession(result)
	if len(session) == 0 {
		session = map[string]any{"session_id": sessionID, "conversation_id": plan.ConversationID, "status": "preparing", "candidates": candidates}
	}
	// The candidate-row projection helper is family-neutral (it only projects
	// the request's candidate rows onto the kernel reply); it is shared with
	// the mix-tick mount verbatim rather than duplicated.
	session = mergeMixTickAuditionSession(session, request, candidates)
	session["schema_version"] = auditionSchemaVersion
	session["conversation_id"] = plan.ConversationID
	session["turn_id"] = turnID
	session["round_id"] = roundID
	session["scope"] = "target"
	session["project_revision"] = projectRevision
	session["active_project_ref"] = projectRef
	session["project_uuid"] = projectUUID
	session["blind"] = settings.enabled

	if failure := auditionReplyError(result, callErr); failure != "" {
		session["status"] = "failed"
		s.emitAuditionEvent(plan.ConversationID, "audition.failed", session, map[string]any{"message": failure, "command": "audition.prepare"})
		return nil, fmt.Errorf("audition.prepare: %s", failure)
	}
	if status := strings.ToLower(firstStringFromMap(session, "status")); status != "ready" && status != "playing" && status != "stopped" {
		// A session that is not servicable is not a mounted card: keeping it
		// off the record is what makes the terminal wording honest.
		s.emitAuditionEvent(plan.ConversationID, "audition.prepare", session, map[string]any{"command": "audition.prepare"})
		return nil, fmt.Errorf("semantic-dynamic audition session is not ready: %s", status)
	}

	record := &semanticDynamicAuditionRecord{
		SchemaVersion:   semanticDynamicAuditionSchemaVersion,
		ConversationID:  plan.ConversationID,
		GoalID:          plan.GoalID,
		RunID:           plan.RunID,
		SessionID:       sessionID,
		TurnID:          turnID,
		RoundID:         roundID,
		Status:          semanticDynamicAuditionStatusReady,
		TicketID:        plan.TicketID,
		Family:          plan.Family,
		ProcessorType:   plan.ProcessorType,
		TrackID:         plan.TrackID,
		PluginID:        plan.PluginID,
		ProjectRef:      projectRef,
		ProjectUUID:     projectUUID,
		ProjectRevision: projectRevision,
		Blind:           settings.enabled,
		BlindSwap:       blindSwap,
		JudgmentPending: true,
		Session:         cloneContext(session),
		Before:          cloneContext(before),
		After:           cloneContext(after),
		UpdatedAt:       time.Now().UTC(),
	}
	s.storeSemanticDynamicAudition(record)
	s.emitAuditionEvent(plan.ConversationID, "audition.ready", session, map[string]any{"command": "audition.prepare", "semantic_dynamic": plan.TicketID})
	s.emitSemanticDynamicAuditionJudgmentRequested(plan, record)
	s.persistCurrentProjectWorkspace()
	return record, nil
}

// emitSemanticDynamicAuditionJudgmentRequested raises the single event that
// turns the rendered A/B card into an answerable one. The node carries the
// same session identity the judgment POST will echo back.
func (s *Server) emitSemanticDynamicAuditionJudgmentRequested(plan *semanticDynamicAuditionPlan, record *semanticDynamicAuditionRecord) {
	if s == nil || plan == nil || record == nil {
		return
	}
	summary := fmt.Sprintf("%s · A/B 试听判定", semanticDynamicAuditionTargetLabel(record))
	event := trajectory.Event{
		Type:           trajectory.EventUserJudgmentRequested,
		ConversationID: plan.ConversationID,
		GoalID:         plan.GoalID,
		RunID:          plan.RunID,
		ItemID:         "semantic_dynamic_judgment:" + record.SessionID,
		Title:          "user A/B judgment requested",
		Body:           "user A/B judgment requested",
		Payload: trajectory.Payload{
			SchemaVersion: trajectory.SchemaVersion,
			TraceNodeID:   "semantic_dynamic_judgment:" + record.SessionID,
			TurnID:        record.TurnID,
			RoundID:       record.RoundID,
			NodeKind:      trajectory.NodeJudgment,
			Phase:         "user_judgment",
			Status:        trajectory.StatusWaiting,
			Outcome:       trajectory.EvaluationHumanAuditionReady,
			Summary:       summary,
			Details: map[string]any{
				"audition_session_id": record.SessionID,
				"summary":             summary,
				"semantic_dynamic":    record.TicketID,
				"family":              record.Family,
				"track_id":            record.TrackID,
			},
		},
	}
	if _, err := s.emitTrajectoryEvent(plan.ConversationID, event); err != nil && s.logger != nil {
		// The trajectory node is the card's enabling affordance, not the
		// judgment contract: a rejection must not tear down a mounted session.
		s.logger.Warn("[semantic.dynamic.audition] judgment-request node rejected conversation=%s session=%s err=%v", plan.ConversationID, record.SessionID, err)
	}
}

func (s *Server) storeSemanticDynamicAudition(record *semanticDynamicAuditionRecord) {
	if s == nil || record == nil || strings.TrimSpace(record.ConversationID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.semanticDynamicAuditions == nil {
		s.semanticDynamicAuditions = map[string]*semanticDynamicAuditionRecord{}
	}
	s.semanticDynamicAuditions[record.ConversationID] = record
}

// semanticDynamicAuditionFor returns the conversation's answerable A/B card:
// mounted, ready, and still owing a judgment. A settled card is not answerable
// a second time. The store is keyed by conversation, so re-executing the same
// ticket re-seats the same session id (idempotent identity) and a new ticket
// replaces the older card, matching the mix-tick one-card-per-conversation
// semantics.
func (s *Server) semanticDynamicAuditionFor(conversationID string) (*semanticDynamicAuditionRecord, bool) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.semanticDynamicAuditions[conversationID]
	if !ok || record == nil || !record.JudgmentPending || record.Status != semanticDynamicAuditionStatusReady {
		return nil, false
	}
	return record, true
}

func (s *Server) expireSemanticDynamicAudition(conversationID string) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.semanticDynamicAuditions, conversationID)
}

// recordSemanticDynamicAuditionJudgment lands an A/B judgment that arrived
// over the HTTP judgment surface for a mounted semantic-dynamic card. It
// reports handled=false when the conversation has no answerable card (or the
// POST names a different session), so the caller falls through to the
// free-state experiment path unchanged. Nothing here touches loop.Experiment
// or the experiment state machine.
func (s *Server) recordSemanticDynamicAuditionJudgment(ctx context.Context, request auditionJudgmentRequest) (map[string]any, bool, error) {
	record, ok := s.semanticDynamicAuditionFor(request.ConversationID)
	if !ok {
		return nil, false, nil
	}
	if strings.TrimSpace(request.SessionID) != record.SessionID {
		return nil, false, nil
	}
	if err := semanticDynamicAuditionIdentityError(record, request); err != nil {
		return nil, true, err
	}
	reading := mixTickAuditionReadingFromRequest(request)
	disposition := mixTickAuditionDispositionFor(reading, record.BlindSwap)
	if strings.TrimSpace(request.FreeText) != "" && disposition.Decision == mixTickAuditionDecisionAmbiguous {
		disposition.Reason = "free_text"
	}
	return s.settleSemanticDynamicAuditionJudgment(ctx, record, disposition, request)
}

// semanticDynamicAuditionIdentityError re-applies the request-to-session
// identity checks the experiment path performs, against the semantic-dynamic
// record instead of an experiment round; the wording follows
// mixTickAuditionIdentityError. A mismatch means the answer belongs to a
// different (or a stale) session and must be refused rather than guessed.
func semanticDynamicAuditionIdentityError(record *semanticDynamicAuditionRecord, request auditionJudgmentRequest) error {
	if record == nil {
		return fmt.Errorf("semantic-dynamic audition session not found")
	}
	if strings.TrimSpace(request.ConversationID) == "" || strings.TrimSpace(request.SessionID) == "" {
		return fmt.Errorf("conversation_id and audition_session_id are required")
	}
	if turnID := strings.TrimSpace(request.TurnID); turnID != "" && turnID != record.TurnID {
		return fmt.Errorf("audition session identity mismatch")
	}
	if roundID := strings.TrimSpace(request.RoundID); roundID != "" && roundID != record.RoundID {
		return fmt.Errorf("round identity mismatch")
	}
	if revision := strings.TrimSpace(request.ProjectRevision); revision != "" && revision != record.ProjectRevision {
		return fmt.Errorf("project revision mismatch")
	}
	if status := strings.ToLower(firstStringFromMap(record.Session, "status")); status != "ready" && status != "playing" && status != "stopped" {
		return fmt.Errorf("audition session is not ready: %s", status)
	}
	rows := auditionCandidateRows(record.Session)
	if len(rows) != 2 || !auditionCandidatesReady(rows) {
		return fmt.Errorf("candidate A/B are not both ready")
	}
	return nil
}

// buildSemanticDynamicAuditionEvidence constructs the user_judgment evidence
// row for a semantic-dynamic judgment. The experiment path's
// buildUserJudgmentEvidence is strongly coupled to the free-state loop and
// experiment round (loop.Experiment.ID, round.TargetResponse, task-state
// projections); this is the minimal isomorphic evidence for a channel that
// owns no experiment, validated through the same experiment.UserJudgmentEvidence
// contract.
func buildSemanticDynamicAuditionEvidence(record *semanticDynamicAuditionRecord, request auditionJudgmentRequest) (experiment.UserJudgmentEvidence, error) {
	session := record.Session
	candidateA := auditionCandidateSnapshot(session, auditionCandidateA)
	candidateB := auditionCandidateSnapshot(session, auditionCandidateB)
	heard := experiment.HeardDifference(strings.ToLower(strings.TrimSpace(request.HeardDifference)))
	if heard == "" {
		heard = experiment.HeardDifferenceYes
	}
	preference := experiment.JudgmentPreference(strings.ToLower(strings.TrimSpace(request.Preference)))
	if heard != experiment.HeardDifferenceYes {
		preference = experiment.PreferenceUnsure
	}
	if preference == "" {
		preference = experiment.PreferenceUnsure
	}
	evidence := experiment.UserJudgmentEvidence{
		SchemaVersion:            experiment.UserJudgmentEvidenceSchemaVersion,
		ID:                       "judg_" + randomID(),
		ConversationID:           record.ConversationID,
		TurnID:                   record.TurnID,
		RoundID:                  record.RoundID,
		AuditionSessionID:        record.SessionID,
		CandidateARef:            firstNonEmpty(firstStringFromMap(candidateA, "source_ref"), firstStringFromMap(session, "candidate_a_ref"), auditionCandidateA),
		CandidateBRef:            firstNonEmpty(firstStringFromMap(candidateB, "source_ref"), firstStringFromMap(session, "candidate_b_ref"), auditionCandidateB),
		CandidateACheckpointRef:  firstStringFromMap(candidateA, "checkpoint_ref"),
		CandidateBCheckpointRef:  firstStringFromMap(candidateB, "checkpoint_ref"),
		CandidateACommitID:       firstStringFromMap(candidateA, "commit_id"),
		CandidateBCommitID:       firstStringFromMap(candidateB, "commit_id"),
		CandidateAPreviewRef:     firstStringFromMap(candidateA, "preview_ref"),
		CandidateBPreviewRef:     firstStringFromMap(candidateB, "preview_ref"),
		CandidateARenderRevision: firstStringFromMap(candidateA, "render_revision"),
		CandidateBRenderRevision: firstStringFromMap(candidateB, "render_revision"),
		ProjectUUID:              record.ProjectUUID,
		ProjectRevision:          record.ProjectRevision,
		Scope:                    firstStringFromMap(session, "scope"),
		HeardDifference:          heard,
		Preference:               preference,
		ReasonTags:               append([]string(nil), request.ReasonTags...),
		FreeText:                 strings.TrimSpace(request.FreeText),
		CreatedAt:                time.Now().UTC(),
	}
	if err := evidence.Validate(); err != nil {
		return experiment.UserJudgmentEvidence{}, err
	}
	return evidence, nil
}

// settleSemanticDynamicAuditionJudgment records the A/B answer. First-version
// boundary (card spec): the judgment is booked (evidence + trajectory node +
// disposition); the physical rollback of the parameter write is NOT executed —
// a rollback-preferred answer is recorded as exactly that and the project is
// left untouched, never reported as an undo that did not happen.
func (s *Server) settleSemanticDynamicAuditionJudgment(ctx context.Context, record *semanticDynamicAuditionRecord, disposition mixTickAuditionDisposition, request auditionJudgmentRequest) (map[string]any, bool, error) {
	if s == nil || record == nil {
		return nil, true, fmt.Errorf("semantic-dynamic audition session not found")
	}
	evidence, evidenceErr := buildSemanticDynamicAuditionEvidence(record, request)
	if evidenceErr != nil {
		return nil, true, evidenceErr
	}
	outcome := map[string]any{
		"schema_version":       semanticDynamicAuditionSchemaVersion,
		"audition_session_id":  record.SessionID,
		"ticket_id":            record.TicketID,
		"family":               record.Family,
		"track_id":             record.TrackID,
		"plugin_id":            record.PluginID,
		"decision":             disposition.Decision,
		"label":                disposition.Label,
		"physical":             disposition.Physical,
		"reason":               disposition.Reason,
		"blind":                record.Blind,
		"free_text":            strings.TrimSpace(request.FreeText),
		"judgment_evidence_id": evidence.ID,
	}
	if record.Blind {
		// The physical assignment is disclosed only after the answer landed,
		// mirroring the un-blinding discipline of the other audition channels.
		outcome["blind_disclosure"] = map[string]any{
			"schema_version":   "vit.semantic_dynamic_audition_blind_disclosure.v1",
			"candidate_a":      semanticDynamicAuditionPhysicalLabel(record, mixTickAuditionPhaseBefore),
			"candidate_b":      semanticDynamicAuditionPhysicalLabel(record, mixTickAuditionPhaseAfter),
			"decision":         disposition.Decision,
			"audition_session": record.SessionID,
		}
	}

	switch disposition.Decision {
	case mixTickAuditionDecisionRetain:
		outcome["status"] = "retained"
		outcome["retained"] = true
	case mixTickAuditionDecisionRollback:
		// 记录倾向但不物理回滚（首版边界）；不谎报已发生的回滚。
		outcome["status"] = "rollback_preferred"
		outcome["rollback_executed"] = false
	default:
		outcome["status"] = "ambiguous"
		outcome["ambiguous_decision"] = disposition.Reason
	}

	record.JudgmentPending = false
	record.Status = semanticDynamicAuditionStatusReady
	record.UpdatedAt = time.Now().UTC()
	s.storeSemanticDynamicAudition(record)
	s.expireSemanticDynamicAudition(record.ConversationID)
	s.emitSemanticDynamicJudgmentRecorded(record, evidence)
	s.emitAgentEvent(record.ConversationID, AgentEvent{
		Type:     "semantic_dynamic.audition.judgment",
		ItemType: "semantic_dynamic",
		Status:   disposition.Decision,
		Title:    "语义动态 A/B 试听判定已落账",
		Body:     semanticDynamicAuditionJudgmentBody(record, disposition, request.FreeText),
		Payload:  cloneContext(outcome),
	})
	s.persistCurrentProjectWorkspace()
	return outcome, true, nil
}

// emitSemanticDynamicJudgmentRecorded lands the recorded-judgment trajectory
// node in the channel's own identity domain. It is the same event type the
// WebUI card listens for to flip its recorded state; the free-state
// experiment's own judgment events are untouched.
func (s *Server) emitSemanticDynamicJudgmentRecorded(record *semanticDynamicAuditionRecord, evidence experiment.UserJudgmentEvidence) {
	if s == nil || record == nil {
		return
	}
	outcome := trajectory.EvaluationAmbiguous
	if evidence.HeardDifference == experiment.HeardDifferenceYes && (evidence.Preference == experiment.PreferenceA || evidence.Preference == experiment.PreferenceB) {
		outcome = trajectory.EvaluationHumanConfirmed
	}
	event := trajectory.Event{
		Type:           trajectory.EventUserJudgmentRecorded,
		ConversationID: record.ConversationID,
		GoalID:         record.GoalID,
		RunID:          record.RunID,
		ItemID:         "semantic_dynamic_judgment_recorded:" + evidence.ID,
		Title:          "User judgment recorded",
		Body:           "user A/B judgment recorded",
		Payload: trajectory.Payload{
			SchemaVersion: trajectory.SchemaVersion,
			TraceNodeID:   "semantic_dynamic_judgment_recorded:" + evidence.ID,
			TurnID:        record.TurnID,
			RoundID:       record.RoundID,
			NodeKind:      trajectory.NodeJudgment,
			Phase:         "user_judgment",
			Status:        trajectory.StatusCompleted,
			Outcome:       outcome,
			Summary:       fmt.Sprintf("%s · A/B 试听判定已落账", semanticDynamicAuditionTargetLabel(record)),
			Details: map[string]any{
				"audition_session_id": record.SessionID,
				"evidence":            evidence,
				"semantic_dynamic":    record.TicketID,
			},
		},
	}
	if _, err := s.emitTrajectoryEvent(record.ConversationID, event); err != nil && s.logger != nil {
		s.logger.Warn("[semantic.dynamic.audition] judgment-recorded node rejected conversation=%s session=%s err=%v", record.ConversationID, record.SessionID, err)
	}
}

// semanticDynamicAuditionPhysicalLabel answers which candidate letter a
// physical side carries, for the post-landing disclosure.
func semanticDynamicAuditionPhysicalLabel(record *semanticDynamicAuditionRecord, physical string) string {
	if record == nil {
		return ""
	}
	if physical == mixTickAuditionPhaseAfter {
		if record.BlindSwap {
			return "A"
		}
		return "B"
	}
	if record.BlindSwap {
		return "B"
	}
	return "A"
}

func semanticDynamicAuditionTargetLabel(record *semanticDynamicAuditionRecord) string {
	if record == nil {
		return ""
	}
	return strings.TrimSpace(pendingMixTickTrackLabel(record.TrackID)) + " 的 " + firstNonEmpty(record.ProcessorType, record.Family)
}

func semanticDynamicAuditionJudgmentBody(record *semanticDynamicAuditionRecord, disposition mixTickAuditionDisposition, freeText string) string {
	target := semanticDynamicAuditionTargetLabel(record)
	switch disposition.Decision {
	case mixTickAuditionDecisionRetain:
		return "已按你的试听判定记录：保留这次" + target + "调整（参数保持已应用状态）。"
	case mixTickAuditionDecisionRollback:
		return "已按你的试听判定记录：你更倾向调整前的" + target + "版本。本次不会自动改写参数；如果要撤销，请直接告诉我。"
	default:
		if strings.TrimSpace(freeText) != "" {
			return "这次" + target + "调整保持已应用状态：你的反馈是「" + strings.TrimSpace(freeText) + "」，我没有再做任何工程修改。"
		}
		return "这次" + target + "调整保持已应用状态：你听不出差别，我没有再做任何工程修改。"
	}
}

// appendSemanticDynamicAuditionClause attaches the A/B entry clause to the
// executed report. An empty clause leaves the report as it was.
func appendSemanticDynamicAuditionClause(report string, bracket semanticDynamicAuditionBracket) string {
	if clause := strings.TrimSpace(semanticDynamicAuditionTerminalClause(bracket)); clause != "" {
		return strings.TrimSpace(report) + "\n" + clause
	}
	return report
}

// appendSemanticTopologyAuditionClause is the bus-topology leg face of the
// same witness: the mounted wording is shared verbatim with the ticket
// channel, while the degraded wording names what actually executed (connect
// and load legs with their dispatch receipts) instead of the ticket channel's
// parameter readback, which the bus legs never performed.
func appendSemanticTopologyAuditionClause(report string, bracket semanticDynamicAuditionBracket) string {
	if bracket.Record != nil || bracket.Degraded == "" {
		return appendSemanticDynamicAuditionClause(report, bracket)
	}
	return strings.TrimSpace(report) + "\n" + "这次的 A/B 对照试听没能生成（原因：" + firstNonEmpty(bracket.Degraded, "渲染不可用") + "），发送连接与装载已执行并带回执。"
}

// semanticDynamicAuditionTerminalClause is the executed-face wording: a
// mounted card states the servicable entry (play + judge; the judgment is
// booked as evidence — the first version does not promise an automatic undo);
// a degraded run states the degradation instead of promising an entry that is
// not there.
func semanticDynamicAuditionTerminalClause(bracket semanticDynamicAuditionBracket) string {
	if bracket.Record != nil {
		return "A/B 对照试听已就绪：在试听卡里把这次调整前后的两个版本各听一遍对比，并对试听结果作出判定；你的判定会作为这次调整的人工试听证据记录在案。"
	}
	if bracket.Degraded != "" {
		return semanticDynamicAuditionDegradedLine(bracket.Degraded)
	}
	return ""
}

// semanticDynamicAuditionDegradedLine is the fail-open witness: the user is
// told the comparison could not be produced, and that the executed change
// itself is unaffected.
func semanticDynamicAuditionDegradedLine(reason string) string {
	return "这次的 A/B 对照试听没能生成（原因：" + firstNonEmpty(reason, "渲染不可用") + "），这次调整已执行并带回读验证。"
}

// semanticDynamicAuditionProtocolStamp is the provenance block carried on the
// executed reply's workflow data, so a run's A/B state is auditable without
// log diving.
func semanticDynamicAuditionProtocolStamp(bracket semanticDynamicAuditionBracket) map[string]any {
	stamp := map[string]any{"schema_version": semanticDynamicAuditionSchemaVersion, "mounted": bracket.Record != nil}
	if bracket.Degraded != "" {
		stamp["degraded"] = bracket.Degraded
	}
	if bracket.Record != nil {
		stamp["audition_session_id"] = bracket.Record.SessionID
		stamp["turn_id"] = bracket.Record.TurnID
		stamp["round_id"] = bracket.Record.RoundID
		stamp["blind"] = bracket.Record.Blind
		stamp["candidate_a_ref"] = firstStringFromMap(bracket.Record.Before, "file_path")
		stamp["candidate_b_ref"] = firstStringFromMap(bracket.Record.After, "file_path")
	}
	return stamp
}
