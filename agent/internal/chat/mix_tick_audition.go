package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/trajectory"
)

// B12-2（2026-09-12）：mix-tick 原生微调分支的 A/B 试听接线。设计依据
// queue/reports/2026-09-11-B12-ab-audition-design.md §Q5。
//
// 本文件是 mix-tick A/B 的唯一实现面，纪律四条：
//
//  1. **时序**：整曲离线渲染复用 free_state_d1_render.go 的 start_render 等待
//     模式（start_render -> WaitRender -> sha256 溯源），before 在 apply **之前**
//     解析（当前态），after 在 apply 之后解析（改动后态）——与 D1 同序。时序由
//     runMixTickAuditionBracket 单点表达（bracket 是两相渲染的唯一调用者，且
//     不先跑完 mutate 就到不了 after 相），故「before 先于 mutation」是结构性质
//     而非两处代码的巧合。
//  2. **会话复用**：A/B 播放走既有内核 audition 会话（audition.prepare +
//     候选=audio_file 实时播放 + AUDITION-PLAY-1 的 select 自动起播），不新建
//     交付面。A=应用前渲染 / B=当前态渲染为**规范指派**；盲态配置面
//     （VitApp/Workspace/agent_runtime_config.json 或 VIT_DAW_AUDITION_BLIND）
//     开启时按会话抽签交换标签背后的物理 render，物理方向在判定落账时解析。
//  3. **判定不落 experiment evidence**：mix-tick 不是实验回路。判定落在
//     mix_tick 确认面（chat 消息面 + /agent/audition/judgment 的 mix-tick 支路），
//     不写 UserJudgmentEvidence、不推进 experiment 状态机、不碰 D1 结算。
//  4. **fail-open**：任一相渲染失败即降级回文本确认并留痕，绝不阻塞用户已确认
//     的这一小步（渲染是呈现增强，不是执行前置条件）。
//
// 与 PARK-1/D1-STALL-1 的分支关系：judgmentBoundaryParkFor 与
// freeStateLoopOwesExperimentOutcomeFor 都以「会话存在 durable free-state
// 循环」为前提（freeStateLoop(conversationID)）。mix-tick 原生分支按定义在
// **没有** D1-S1 实验准入时才到达（executePendingMixTickCandidate 先按准入
// 分派 executeD1TrackPan/executeD1StaticEQ/executeD1TrackGain），故两者对本
// 分支恒为假，判定落账与那两个驻留分支不可能冲突。

const (
	mixTickAuditionSchemaVersion = "vit.mix_tick_audition.v1"

	mixTickAuditionPhaseBefore = "before"
	mixTickAuditionPhaseAfter  = "after"

	mixTickAuditionStatusReady    = "ready"
	mixTickAuditionStatusDegraded = "degraded"

	mixTickAuditionDecisionRetain    = "retain"
	mixTickAuditionDecisionRollback  = "rollback"
	mixTickAuditionDecisionAmbiguous = "ambiguous"

	mixTickAuditionRenderDir = "mix_tick_audition"
	mixTickAuditionSource    = "mix_tick_audition"
)

// Server.mixTickAuditionRenderOverride is the B12-2 ordering-pin seam that this
// file's bracket resolves both phases through: production leaves it nil and
// renders with the start_render wait pattern below, while tests inject a phase
// recorder so "the before render precedes the mutation" has a deterministic
// witness without a live kernel. The seam replaces what a phase returns, never
// when it runs — the pair's order lives in runMixTickAuditionBracket alone.

// mixTickAuditionRecord is the conversation-scoped state of a mounted mix-tick
// A/B card. It is intentionally in-memory only: the rendered WAVs and their
// sha256 are the durable artifacts, while the judgment entry is a transient UI
// affordance like the pending confirmation slot it sits behind. Keeping it out
// of projectAgentRuntimeState avoids a new persisted field whose old-record
// default semantics would need their own compatibility contract (AGENTS §11).
type mixTickAuditionRecord struct {
	SchemaVersion   string
	ConversationID  string
	GoalID          string
	RunID           string
	SessionID       string
	TurnID          string
	RoundID         string
	Status          string
	Degraded        string
	TrackID         string
	Operation       string
	TickID          string
	ProjectPath     string
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

// mixTickAuditionPlan carries everything the two renders and the session
// prepare need. AfterRevision is a closure because the post-mutation revision
// does not exist until the mutation has landed — the bracket resolves it at the
// after phase, which is exactly the ordering the card requires.
type mixTickAuditionPlan struct {
	ConversationID string
	GoalID         string
	RunID          string
	TrackID        string
	Operation      string
	TickID         string
	ProjectPath    string
	BeforeRevision string
	AfterRevision  func() string
}

// mixTickAuditionBracket is what the ordering bracket hands back to the report
// composer: a mounted card, or the degraded reason that kept it off.
type mixTickAuditionBracket struct {
	Record   *mixTickAuditionRecord
	Degraded string
}

// mixTickAuditionDisposition is the A/B answer reduced to the action it
// authorizes at the mix_tick confirmation slot. Physical is the side of the
// pair the chosen label actually carries; it is the only thing that may decide
// retain versus rollback, so a blind swap cannot invert the direction.
type mixTickAuditionDisposition struct {
	Decision string
	Label    string
	Physical string
	Reason   string
}

// mixTickAuditionReading is the chat-surface reading of an A/B answer. A
// reading is either label-shaped (Preference) or semantic (Decision) — the
// semantic form exists because "撤销这一步" states the physical intent directly
// and must not be re-derived through a possibly-swapped label.
type mixTickAuditionReading struct {
	Heard      string
	Preference string
	Decision   string
	Reason     string
}

// newMixTickAuditionPlan binds the A/B plan to the confirmed tick.
func newMixTickAuditionPlan(conversationID, goalID, runID, projectPath, tickID string, candidate agentloop.PendingMixTickCandidate, beforeRevision string, afterRevision func() string) *mixTickAuditionPlan {
	return &mixTickAuditionPlan{
		ConversationID: conversationID,
		GoalID:         goalID,
		RunID:          runID,
		TrackID:        candidate.TrackID,
		Operation:      candidate.Operation,
		TickID:         tickID,
		ProjectPath:    projectPath,
		BeforeRevision: beforeRevision,
		AfterRevision:  afterRevision,
	}
}

func mixTickAuditionSessionID(conversationID, tickID string) string {
	return "audition:mix_tick:" + sanitizeCanaryID(firstNonEmpty(tickID, conversationID))
}

func mixTickAuditionTurnID(conversationID, tickID string) string {
	return "mix_tick_turn:" + sanitizeCanaryID(firstNonEmpty(tickID, conversationID))
}

func mixTickAuditionRoundID(tickID, trackID string) string {
	return "mix_tick_round:" + sanitizeCanaryID(firstNonEmpty(tickID, trackID))
}

// runMixTickAuditionBracket is the single expression of the B12-2 order:
// resolve the pre-mutation render, run the mutation, resolve the post-mutation
// render, mount the session. Every failure degrades open — the user's confirmed
// small step is never held hostage by the presentation layer (卡面 ④).
//
// The bracket deliberately does not own the mutation's own error handling: it
// reports the mutate error back to the caller, which keeps owning the existing
// "执行混音 tick 失败" contract byte-for-byte.
func (s *Server) runMixTickAuditionBracket(ctx context.Context, plan *mixTickAuditionPlan, mutate func() error) (mixTickAuditionBracket, error) {
	if s == nil || plan == nil {
		return mixTickAuditionBracket{}, nil
	}
	before, beforeErr := s.resolveMixTickAuditionRender(ctx, plan, mixTickAuditionPhaseBefore, plan.BeforeRevision)
	if beforeErr != nil {
		// Fail-open on the pre-mutation phase: the mutation still runs. The
		// degraded reason is kept so the terminal wording states the
		// degradation instead of promising an entry that is not there.
		s.logMixTickAuditionDegraded(plan, mixTickAuditionPhaseBefore, beforeErr)
		before = nil
	}

	if err := mutate(); err != nil {
		// The change did not land: no post-mutation render, no card. The
		// caller composes the existing failure response.
		return mixTickAuditionBracket{}, err
	}

	if before == nil {
		return mixTickAuditionBracket{Degraded: "before_render_failed"}, nil
	}

	afterRevision := ""
	if plan.AfterRevision != nil {
		afterRevision = strings.TrimSpace(plan.AfterRevision())
	}
	after, afterErr := s.resolveMixTickAuditionRender(ctx, plan, mixTickAuditionPhaseAfter, afterRevision)
	if afterErr != nil {
		s.logMixTickAuditionDegraded(plan, mixTickAuditionPhaseAfter, afterErr)
		return mixTickAuditionBracket{Degraded: "after_render_failed"}, nil
	}

	record, prepareErr := s.mountMixTickAudition(ctx, plan, before, after)
	if prepareErr != nil {
		s.logMixTickAuditionDegraded(plan, "prepare", prepareErr)
		return mixTickAuditionBracket{Degraded: "audition_prepare_failed"}, nil
	}
	return mixTickAuditionBracket{Record: record}, nil
}

// resolveMixTickAuditionRender is the one seam both phases and the ordering
// pin go through, so an override can never reorder the pair — it can only
// replace what a phase returns.
func (s *Server) resolveMixTickAuditionRender(ctx context.Context, plan *mixTickAuditionPlan, phase, projectRevision string) (map[string]any, error) {
	if s.mixTickAuditionRenderOverride != nil {
		return s.mixTickAuditionRenderOverride(ctx, phase, projectRevision, plan)
	}
	return s.renderMixTickAuditionPhase(ctx, plan.ConversationID, phase, projectRevision)
}

func (s *Server) logMixTickAuditionDegraded(plan *mixTickAuditionPlan, phase string, err error) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Warn("[mix.tick.audition] degraded phase=%s conversation=%s tick=%s err=%v",
		phase, plan.ConversationID, plan.TickID, err)
}

// renderMixTickAuditionPhase performs one side of the pair with the D1
// start_render wait pattern (free_state_d1_render.go:99-136): start_render with
// 24-bit master-plugin rendering into a revision-named WAV, wait on the job,
// then hash the payload for provenance. The D1 pair hangs its ready rows on
// loop.D1State because it is experiment-bound; the native mix-tick branch has
// no loop, so the row is returned and persisted with the audition record
// instead. The low-level helpers (validD1RenderFile / sameD1RenderPath /
// hashD1RenderFile) are shared verbatim rather than copied.
func (s *Server) renderMixTickAuditionPhase(ctx context.Context, conversationID, phase, projectRevision string) (map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("mix-tick audition render dependencies are unavailable")
	}
	phase = strings.ToLower(strings.TrimSpace(phase))
	if phase != mixTickAuditionPhaseBefore && phase != mixTickAuditionPhaseAfter {
		return nil, fmt.Errorf("mix-tick audition render phase must be before or after")
	}
	projectRevision = strings.TrimSpace(projectRevision)
	if projectRevision == "" {
		return nil, fmt.Errorf("mix-tick audition render must be revision-bound")
	}
	if s.kernel == nil || s.harness == nil {
		return nil, fmt.Errorf("mix-tick audition render dependencies are unavailable")
	}
	root := filepath.Join(s.artifactStore().Root, mixTickAuditionRenderDir, sanitizeCanaryID(conversationID))
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create mix-tick audition render directory: %w", err)
	}
	path := filepath.Join(root, phase+"_revision_"+sanitizeCanaryID(projectRevision)+".wav")
	if validD1RenderFile(path) {
		return finishMixTickAuditionRender(phase, projectRevision, path)
	}
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove incomplete mix-tick audition render: %w", err)
		}
	}
	reply, _, err := s.kernel.SendCommand(ctx, map[string]any{"cmd": "start_render", "file_path": path, "bit_depth": 24, "use_master_plugins": true})
	if err != nil {
		return nil, fmt.Errorf("start mix-tick audition %s render: %w", phase, err)
	}
	if strings.EqualFold(firstStringFromMap(reply, "status"), "error") {
		return nil, fmt.Errorf("start mix-tick audition %s render: %s", phase, firstNonEmpty(firstStringFromMap(reply, "message"), "kernel rejected render"))
	}
	jobID := firstStringFromMap(reply, "job_id")
	if jobID == "" {
		return nil, fmt.Errorf("start mix-tick audition %s render returned no job_id", phase)
	}
	result, waitErr := s.harness.WaitRender(ctx, jobID)
	if waitErr != nil && !validD1RenderFile(path) {
		return nil, fmt.Errorf("wait for mix-tick audition %s render %s: %w", phase, jobID, waitErr)
	}
	if waitErr == nil {
		if result.Status != "ready" {
			return nil, fmt.Errorf("mix-tick audition %s render %s failed: %s", phase, jobID, firstNonEmpty(result.Error, result.Status))
		}
		if result.FilePath != "" && !sameD1RenderPath(result.FilePath, path) {
			return nil, fmt.Errorf("mix-tick audition %s render %s completed for an unexpected file", phase, jobID)
		}
	}
	if !validD1RenderFile(path) {
		return nil, fmt.Errorf("mix-tick audition %s render %s produced no valid WAV", phase, jobID)
	}
	return finishMixTickAuditionRender(phase, projectRevision, path)
}

func finishMixTickAuditionRender(phase, projectRevision, path string) (map[string]any, error) {
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
		"render_revision":    "mix_tick_render:" + phase + ":" + projectRevision + ":" + digest[:16],
		"preview_revision":   "sha256:" + digest,
		"completed_at":       time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

// mountMixTickAudition prepares the kernel session for the pair, records the
// conversation-scoped state, and raises the two events the WebUI A/B card is
// built from (audition.ready renders the card; the judgment-requested
// trajectory event enables its A/B judgment buttons — B12-2 勘察：面板的
// canJudge 只读 session.judgmentRequested，而该标志只由事件
// trajectory.user_judgment.requested 置位，不需要实验回合或轨迹节点）。
func (s *Server) mountMixTickAudition(ctx context.Context, plan *mixTickAuditionPlan, before, after map[string]any) (*mixTickAuditionRecord, error) {
	if s == nil || plan == nil {
		return nil, fmt.Errorf("mix-tick audition dependencies are unavailable")
	}
	if s.auditionKernel == nil {
		return nil, fmt.Errorf("audition kernel unavailable")
	}
	sessionID := mixTickAuditionSessionID(plan.ConversationID, plan.TickID)
	turnID := mixTickAuditionTurnID(plan.ConversationID, plan.TickID)
	roundID := mixTickAuditionRoundID(plan.TickID, plan.TrackID)
	projectRevision := firstStringFromMap(after, "project_revision")

	projectRef := strings.TrimSpace(plan.ProjectPath)
	projectUUID := ""
	if s.auditionCandidateDriver != nil {
		if plane, planeErr := s.auditionCandidateDriver.CurrentPlane(ctx); planeErr == nil {
			projectRef = firstNonEmpty(projectRef, plane.ProjectPath)
			projectUUID = firstNonEmpty(projectUUID, plane.ProjectUUID)
		}
	}
	projectRef = firstNonEmpty(projectRef, "project:active")

	// The blind tier is the same configuration face AUDITION-PLAY-1 hardened
	// (env wins, then VitApp/Workspace/agent_runtime_config.json, read per
	// session). Honouring it here rather than silently running canonical keeps
	// the documented failure mode — "a blind run that quietly became canonical
	// is worse than a refused one" — off this path too.
	settings := auditionBlindSettingsFor()
	blindSwap := false
	if settings.enabled {
		blindSwap = s.drawAuditionBlind()
	}
	if s.logger != nil {
		s.logger.Info("[mix.tick.audition] blind tier %s session=%s swapped=%v", settings.source, sessionID, blindSwap)
	}

	baselineCommit := "mix_tick:" + plan.TickID + ":" + mixTickAuditionPhaseBefore
	treatmentCommit := "mix_tick:" + plan.TickID + ":" + mixTickAuditionPhaseAfter
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
		return nil, fmt.Errorf("mix-tick audition session is not ready: %s", status)
	}

	record := &mixTickAuditionRecord{
		SchemaVersion:   mixTickAuditionSchemaVersion,
		ConversationID:  plan.ConversationID,
		GoalID:          plan.GoalID,
		RunID:           plan.RunID,
		SessionID:       sessionID,
		TurnID:          turnID,
		RoundID:         roundID,
		Status:          mixTickAuditionStatusReady,
		TrackID:         plan.TrackID,
		Operation:       plan.Operation,
		TickID:          plan.TickID,
		ProjectPath:     projectRef,
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
	s.storeMixTickAudition(record)
	s.emitAuditionEvent(plan.ConversationID, "audition.ready", session, map[string]any{"command": "audition.prepare", "mix_tick": plan.TickID})
	s.emitMixTickAuditionJudgmentRequested(plan, record)
	s.persistCurrentProjectWorkspace()
	return record, nil
}

// mergeMixTickAuditionSession projects the kernel reply onto the request's
// candidate rows the way enrichAuditionSession does for the D1 pair, without
// taking the experiment-bound loop/round parameters that face requires.
func mergeMixTickAuditionSession(session map[string]any, request kernel.AuditionSessionRequest, candidates []kernel.AuditionCandidate) map[string]any {
	if session == nil {
		return nil
	}
	rows := auditionCandidateRows(session)
	if len(rows) == 0 {
		rows = []map[string]any{{"id": auditionCandidateA}, {"id": auditionCandidateB}}
	}
	for index := range candidates {
		want := candidates[index]
		for rowIndex := range rows {
			if firstStringFromMap(rows[rowIndex], "id") != want.ID {
				continue
			}
			rows[rowIndex]["label"] = firstNonEmpty(firstStringFromMap(rows[rowIndex], "label"), want.Label)
			rows[rowIndex]["source_kind"] = firstNonEmpty(firstStringFromMap(rows[rowIndex], "source_kind"), want.SourceKind)
			rows[rowIndex]["source_ref"] = firstNonEmpty(firstStringFromMap(rows[rowIndex], "source_ref"), want.SourceRef)
			rows[rowIndex]["preview_ref"] = firstNonEmpty(firstStringFromMap(rows[rowIndex], "preview_ref"), want.PreviewRef)
			rows[rowIndex]["project_revision"] = firstNonEmpty(firstStringFromMap(rows[rowIndex], "project_revision"), want.ProjectRevision)
		}
	}
	values := make([]any, 0, len(rows))
	for _, row := range rows {
		values = append(values, row)
	}
	session["candidates"] = values
	session["candidate_a_ref"] = request.Candidates[0].SourceRef
	session["candidate_b_ref"] = request.Candidates[1].SourceRef
	return session
}

// emitMixTickAuditionJudgmentRequested raises the single event that turns the
// rendered A/B card into an answerable one. The node carries the same session
// identity the judgment POST will echo back, so the card's turn/round badges
// and the server-side identity checks agree on one vocabulary.
func (s *Server) emitMixTickAuditionJudgmentRequested(plan *mixTickAuditionPlan, record *mixTickAuditionRecord) {
	if s == nil || plan == nil || record == nil {
		return
	}
	summary := fmt.Sprintf("%s · %s", pendingMixTickHumanSummary(agentloop.PendingMixTickCandidate{
		Operation: record.Operation, TrackID: record.TrackID,
	}), "A/B 试听判定")
	event := trajectory.Event{
		Type:           trajectory.EventUserJudgmentRequested,
		ConversationID: plan.ConversationID,
		GoalID:         plan.GoalID,
		RunID:          plan.RunID,
		ItemID:         "mix_tick_judgment:" + record.SessionID,
		Title:          "user A/B judgment requested",
		Body:           "user A/B judgment requested",
		Payload: trajectory.Payload{
			SchemaVersion: trajectory.SchemaVersion,
			TraceNodeID:   "mix_tick_judgment:" + record.SessionID,
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
				"mix_tick":            record.TickID,
				"track_id":            record.TrackID,
			},
		},
	}
	if _, err := s.emitTrajectoryEvent(plan.ConversationID, event); err != nil && s.logger != nil {
		// The trajectory node is the card's enabling affordance, not the
		// judgment contract: a rejection must not tear down a mounted session.
		s.logger.Warn("[mix.tick.audition] judgment-request node rejected conversation=%s session=%s err=%v", plan.ConversationID, record.SessionID, err)
	}
}

func (s *Server) storeMixTickAudition(record *mixTickAuditionRecord) {
	if s == nil || record == nil || strings.TrimSpace(record.ConversationID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mixTickAuditions == nil {
		s.mixTickAuditions = map[string]*mixTickAuditionRecord{}
	}
	s.mixTickAuditions[record.ConversationID] = record
}

// mixTickAuditionFor returns the conversation's answerable A/B card, i.e. one
// that is mounted, ready, and still owes a judgment. A card that already
// settled is not answerable a second time.
func (s *Server) mixTickAuditionFor(conversationID string) (*mixTickAuditionRecord, bool) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.mixTickAuditions[conversationID]
	if !ok || record == nil || !record.JudgmentPending || record.Status != mixTickAuditionStatusReady {
		return nil, false
	}
	return record, true
}

func (s *Server) expireMixTickAudition(conversationID string) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.mixTickAuditions, conversationID)
}

// mixTickAuditionDispositionFor maps an A/B answer onto the retain / rollback /
// ambiguous outcomes. Retain versus rollback is decided by the **physical** side
// the chosen label carries, never by the letter: under a blind draw the letters
// are swapped, and reading them literally is the one way this card can turn
// "keep it" into "undo it" (设计稿 §3 风险 1 的同族面).
func mixTickAuditionDispositionFor(reading mixTickAuditionReading, blindSwap bool) mixTickAuditionDisposition {
	heard := strings.ToLower(strings.TrimSpace(reading.Heard))
	preference := strings.ToLower(strings.TrimSpace(reading.Preference))
	decision := strings.ToLower(strings.TrimSpace(reading.Decision))

	switch {
	case heard == string(experiment.HeardDifferenceNo) || heard == string(experiment.HeardDifferenceUnsure):
		return mixTickAuditionDisposition{Decision: mixTickAuditionDecisionAmbiguous, Reason: "heard_no_difference"}
	case decision == mixTickAuditionDecisionRetain:
		return mixTickAuditionDisposition{Decision: mixTickAuditionDecisionRetain, Physical: mixTickAuditionPhaseAfter, Reason: "stated_retain"}
	case decision == mixTickAuditionDecisionRollback:
		return mixTickAuditionDisposition{Decision: mixTickAuditionDecisionRollback, Physical: mixTickAuditionPhaseBefore, Reason: "stated_rollback"}
	}

	if heard != string(experiment.HeardDifferenceYes) {
		return mixTickAuditionDisposition{Decision: mixTickAuditionDecisionAmbiguous, Reason: "no_heard_difference"}
	}
	label := ""
	physical := ""
	switch preference {
	case string(experiment.PreferenceA):
		label = "A"
		physical = mixTickAuditionPhaseBefore
		if blindSwap {
			physical = mixTickAuditionPhaseAfter
		}
	case string(experiment.PreferenceB):
		label = "B"
		physical = mixTickAuditionPhaseAfter
		if blindSwap {
			physical = mixTickAuditionPhaseBefore
		}
	default:
		return mixTickAuditionDisposition{Decision: mixTickAuditionDecisionAmbiguous, Reason: firstNonEmpty(reading.Reason, "no_preference")}
	}
	if physical == mixTickAuditionPhaseAfter {
		return mixTickAuditionDisposition{Decision: mixTickAuditionDecisionRetain, Label: label, Physical: physical, Reason: "label_carries_after"}
	}
	return mixTickAuditionDisposition{Decision: mixTickAuditionDecisionRollback, Label: label, Physical: physical, Reason: "label_carries_before"}
}

// mixTickAuditionReadingFromMessage reads the chat-surface A/B answer. Label
// wording (选 A / A 更好) and semantic wording (撤销这一步 / 保留) are both
// accepted: the semantic form states the physical intent directly and is
// therefore not routed through a possibly-swapped label.
func mixTickAuditionReadingFromMessage(message string) (mixTickAuditionReading, bool) {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return mixTickAuditionReading{}, false
	}
	if textHasAny(text, "听不出", "没有差别", "没差别", "没有区别", "没区别", "听不出差别", "听不出来", "no difference", "cannot tell", "can't tell") {
		return mixTickAuditionReading{Heard: string(experiment.HeardDifferenceNo)}, true
	}
	if textHasAny(text, "撤销", "回滚", "退回", "恢复原样", "恢复原状", "不要这个", "撤销这一步", "undo", "rollback", "revert") {
		return mixTickAuditionReading{Heard: string(experiment.HeardDifferenceYes), Decision: mixTickAuditionDecisionRollback}, true
	}
	if textHasAny(text, "保留", "就这样", "保持", "不用改", "留着", "keep it", "keep this") {
		return mixTickAuditionReading{Heard: string(experiment.HeardDifferenceYes), Decision: mixTickAuditionDecisionRetain}, true
	}
	if heard, preference, ok := mixTickAuditionLabelAnswer(text); ok {
		return mixTickAuditionReading{Heard: heard, Preference: preference}, true
	}
	return mixTickAuditionReading{}, false
}

// mixTickAuditionLabelAnswer recognises an explicit A/B label answer. The
// letter must be standing on its own as a candidate label ("选 a", "a 更好",
// "a"), never as the "a" of an English word.
func mixTickAuditionLabelAnswer(text string) (string, string, bool) {
	for _, letter := range []string{"a", "b"} {
		for _, prefix := range []string{"选", "选择", "要", "听", "版本", "候选"} {
			if strings.Contains(text, prefix+letter) || strings.Contains(text, prefix+" "+letter) {
				return string(experiment.HeardDifferenceYes), letter, true
			}
		}
		if strings.Contains(text, letter+"更好") || strings.Contains(text, letter+" 更好") ||
			strings.Contains(text, letter+"好") || strings.Contains(text, letter+"面") {
			return string(experiment.HeardDifferenceYes), letter, true
		}
		if text == letter {
			return string(experiment.HeardDifferenceYes), letter, true
		}
	}
	return "", "", false
}

// recordMixTickAuditionJudgment lands an A/B judgment that arrived over the
// HTTP judgment surface (the WebUI card's A/B / 听不出差别 / free-text seats).
// It reports ok=false when the conversation has no answerable mix-tick card, so
// the caller falls through to the free-state experiment path unchanged.
func (s *Server) recordMixTickAuditionJudgment(ctx context.Context, request auditionJudgmentRequest) (map[string]any, bool, error) {
	record, ok := s.mixTickAuditionFor(request.ConversationID)
	if !ok {
		return nil, false, nil
	}
	if strings.TrimSpace(request.SessionID) != record.SessionID {
		return nil, false, nil
	}
	if err := mixTickAuditionIdentityError(record, request); err != nil {
		return nil, true, err
	}
	reading := mixTickAuditionReadingFromRequest(request)
	disposition := mixTickAuditionDispositionFor(reading, record.BlindSwap)
	if strings.TrimSpace(request.FreeText) != "" && disposition.Decision == mixTickAuditionDecisionAmbiguous {
		disposition.Reason = "free_text"
	}
	return s.settleMixTickAuditionJudgment(ctx, record, disposition, request.FreeText)
}

// mixTickAuditionIdentityError re-applies the request-to-session identity
// checks the experiment path performs, against the mix-tick record instead of
// an experiment round. The WebUI builds its body from the session snapshot, so
// these fields are the card's own echo; a mismatch means the answer belongs to
// a different (or a stale) session and must be refused rather than guessed.
func mixTickAuditionIdentityError(record *mixTickAuditionRecord, request auditionJudgmentRequest) error {
	if record == nil {
		return fmt.Errorf("mix-tick audition session not found")
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

func mixTickAuditionReadingFromRequest(request auditionJudgmentRequest) mixTickAuditionReading {
	reading := mixTickAuditionReading{
		Heard:      strings.ToLower(strings.TrimSpace(request.HeardDifference)),
		Preference: strings.ToLower(strings.TrimSpace(request.Preference)),
	}
	if reading.Heard == "" {
		reading.Heard = string(experiment.HeardDifferenceYes)
	}
	if strings.TrimSpace(request.FreeText) != "" {
		reading.Reason = "free_text"
	}
	return reading
}

// settleMixTickAuditionJudgment applies the disposition at the mix_tick
// confirmation slot. Nothing here writes experiment evidence: retain keeps the
// applied state as-is (the tick already landed), rollback routes through the
// existing mix.rollback_tick tool, and ambiguity performs no project change and
// reports through the existing ambiguous wording.
func (s *Server) settleMixTickAuditionJudgment(ctx context.Context, record *mixTickAuditionRecord, disposition mixTickAuditionDisposition, freeText string) (map[string]any, bool, error) {
	if s == nil || record == nil {
		return nil, true, fmt.Errorf("mix-tick audition session not found")
	}
	outcome := map[string]any{
		"schema_version":      mixTickAuditionSchemaVersion,
		"audition_session_id": record.SessionID,
		"mix_tick":            record.TickID,
		"track_id":            record.TrackID,
		"decision":            disposition.Decision,
		"label":               disposition.Label,
		"physical":            disposition.Physical,
		"reason":              disposition.Reason,
		"blind":               record.Blind,
		"free_text":           strings.TrimSpace(freeText),
	}
	if record.Blind {
		// The physical assignment is disclosed only after the action landed,
		// mirroring the experiment path's un-blinding discipline.
		outcome["blind_disclosure"] = map[string]any{
			"schema_version":   "vit.mix_tick_audition_blind_disclosure.v1",
			"candidate_a":      mixTickAuditionPhysicalLabel(record, mixTickAuditionPhaseBefore),
			"candidate_b":      mixTickAuditionPhysicalLabel(record, mixTickAuditionPhaseAfter),
			"decision":         disposition.Decision,
			"audition_session": record.SessionID,
		}
	}

	switch disposition.Decision {
	case mixTickAuditionDecisionRollback:
		rollback, err := s.rollbackMixTickAudition(ctx, record)
		if err != nil {
			return nil, true, err
		}
		outcome["rollback"] = rollback
		outcome["status"] = "rolled_back"
	case mixTickAuditionDecisionRetain:
		outcome["status"] = "retained"
		outcome["retained"] = true
	default:
		outcome["status"] = "ambiguous"
		outcome["ambiguous_decision"] = disposition.Reason
	}

	record.JudgmentPending = false
	record.Status = mixTickAuditionStatusReady
	record.UpdatedAt = time.Now().UTC()
	s.storeMixTickAudition(record)
	s.expireMixTickAudition(record.ConversationID)
	s.emitAgentEvent(record.ConversationID, AgentEvent{
		Type:     "mix_tick.audition.judgment",
		ItemType: "mix_tick",
		Status:   disposition.Decision,
		Title:    "mix-tick A/B 试听判定已落账",
		Body:     mixTickAuditionJudgmentBody(record, disposition, freeText),
		Payload:  cloneContext(outcome),
	})
	s.persistCurrentProjectWorkspace()
	return outcome, true, nil
}

// mixTickAuditionPhysicalLabel answers which physical side a candidate letter
// carries, for the post-landing disclosure.
func mixTickAuditionPhysicalLabel(record *mixTickAuditionRecord, physical string) string {
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

func mixTickAuditionJudgmentBody(record *mixTickAuditionRecord, disposition mixTickAuditionDisposition, freeText string) string {
	target := pendingMixTickTrackLabel("")
	if record != nil {
		target = pendingMixTickTrackLabel(record.TrackID)
	}
	switch disposition.Decision {
	case mixTickAuditionDecisionRetain:
		return "已按你的试听判定保留这一步改动（" + target + "）。"
	case mixTickAuditionDecisionRollback:
		return "已按你的试听判定撤销这一步改动（" + target + "）。"
	default:
		if strings.TrimSpace(freeText) != "" {
			return "这一步保持不动：你的反馈是「" + strings.TrimSpace(freeText) + "」，我没有再做任何工程修改。"
		}
		return "这一步保持不动：你听不出差别，我没有再做任何工程修改。"
	}
}

// rollbackMixTickAudition routes the rollback arm through the existing
// mix.rollback_tick tool — the same rollback path the applied report has always
// advertised — so the A/B card does not introduce a second undo mechanism.
func (s *Server) rollbackMixTickAudition(ctx context.Context, record *mixTickAuditionRecord) (map[string]any, error) {
	if s == nil || record == nil {
		return nil, fmt.Errorf("mix-tick audition session not found")
	}
	if s.harness == nil {
		return nil, fmt.Errorf("mix tick rollback dependencies are unavailable")
	}
	exec := executor.New(s.harness)
	out, err := exec.RunToolCall(ctx, executor.Input{
		GoalID:    record.GoalID,
		RunID:     record.RunID,
		ToolCall:  planner.ToolCall{ID: "rollback_mix_tick_audition", Tool: "mix.rollback_tick", Args: map[string]any{"tick_id": record.TickID}, Reason: "user selected the pre-change candidate in the A/B audition"},
		Context:   map[string]any{"conversation_id": record.ConversationID},
		Confirmed: true,
		Source:    mixTickAuditionSource,
	})
	if err != nil {
		return nil, fmt.Errorf("rollback mix tick: %w", err)
	}
	if resultFailed(out) {
		return nil, fmt.Errorf("rollback mix tick: %s", firstNonEmpty(out.Error, cleanContextText(out.Result["error"]), "unknown error"))
	}
	return agentLoopExecutionRecord(out), nil
}

// handleMixTickAuditionChat answers an A/B judgment typed into the chat
// surface. It is the same disposition source the HTTP seat uses, so the two
// entries cannot disagree about what a letter means. Anything that is not an
// A/B answer for a live card falls through to the existing mix-tick handling
// untouched.
func (s *Server) handleMixTickAuditionChat(ctx context.Context, conversationID string, req ChatRequest, mode string) (ChatResponse, bool) {
	if s == nil {
		return ChatResponse{}, false
	}
	record, ok := s.mixTickAuditionFor(conversationID)
	if !ok {
		return ChatResponse{}, false
	}
	if messageClearlyShiftsMixTickContext(req.Message) && !messageExplicitMixTickApply(req.Message) {
		if s.logger != nil {
			s.logger.Info("[mix.tick.audition] expired on context shift conversation=%s session=%s message=%q", conversationID, record.SessionID, req.Message)
		}
		s.expireMixTickAudition(conversationID)
		return ChatResponse{}, false
	}
	reading, ok := mixTickAuditionReadingFromMessage(req.Message)
	if !ok {
		return ChatResponse{}, false
	}
	disposition := mixTickAuditionDispositionFor(reading, record.BlindSwap)
	if disposition.Decision == mixTickAuditionDecisionAmbiguous {
		if s.logger != nil {
			s.logger.Info("[mix.tick.audition] ambiguous answer held conversation=%s session=%s message=%q", conversationID, record.SessionID, req.Message)
		}
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          fmt.Sprintf("我还没有做任何工程修改。刚才应用的是：%s。要保留这一步请说「选 B」，要撤销请说「选 A」；如果确实听不出差别请说「听不出差别」。", pendingMixTickHumanSummary(agentloop.PendingMixTickCandidate{Operation: record.Operation, TrackID: record.TrackID})),
			GoalStatus:     "completed",
			StopReason:     "ambiguous_mix_tick_confirmation",
		}, true
	}
	outcome, _, err := s.settleMixTickAuditionJudgment(ctx, record, disposition, "")
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[mix.tick.audition] judgment failed conversation=%s session=%s err=%v", conversationID, record.SessionID, err)
		}
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          "你的试听判定没能执行：" + err.Error() + "。工程没有被进一步修改。",
			GoalStatus:     "failed",
			StopReason:     "mix_tick_audition_judgment_failed",
			Error:          err.Error(),
		}, true
	}
	return ChatResponse{
		ConversationID: conversationID,
		AgentMode:      mode,
		GoalID:         record.GoalID,
		RunID:          record.RunID,
		Reply:          mixTickAuditionJudgmentBody(record, disposition, ""),
		GoalStatus:     "completed",
		StopReason:     "mix_tick_audition_" + disposition.Decision,
		Workflow:       "mix_tick",
		WorkflowData:   cloneContext(outcome),
	}, true
}

// mixTickAuditionLiveRevision resolves the active project revision the pair is
// bound to. It asks the same audition project plane the D1 pair resolves its
// revision from, so a mix-tick A/B pair can never be rendered against a
// revision the audition plane would disagree with.
func (s *Server) mixTickAuditionLiveRevision(ctx context.Context) string {
	if s == nil {
		return ""
	}
	if s.auditionCandidateDriver != nil {
		if plane, err := s.auditionCandidateDriver.CurrentPlane(ctx); err == nil {
			if revision := strings.TrimSpace(plane.ProjectRevision); revision != "" {
				return revision
			}
		}
	}
	if s.harness != nil {
		state := s.harness.StateSummary(ctx)
		return firstNonEmpty(cleanContextText(state["project_revision"]), cleanContextText(state["revision"]))
	}
	return ""
}

// mixTickAuditionEntryFor is the three-state terminal-wording input (卡面 ⑤):
// the authority mode decides which branch speaks, the bracket decides whether
// the entry is real.
func mixTickAuditionEntryFor(req ChatRequest, bracket mixTickAuditionBracket) mixTickJudgmentEntry {
	return mixTickJudgmentEntry{
		Manual:   authorityModeFromContext(req.Context) != experiment.AuthorityFull,
		Mounted:  bracket.Record != nil,
		Degraded: bracket.Degraded,
	}
}

// appendMixTickAuditionClause attaches the A/B entry clause to the applied
// report. An empty clause leaves the report byte-for-byte as it was.
func appendMixTickAuditionClause(report string, entry mixTickJudgmentEntry) string {
	if clause := strings.TrimSpace(mixTickAuditionTerminalClause(entry)); clause != "" {
		return strings.TrimSpace(report) + "\n" + clause
	}
	return report
}

// mixTickAuditionDegradedLine is the fail-open witness: the user is told the
// comparison could not be produced and which existing entry still works, so no
// promise is left standing that the run cannot keep.
func mixTickAuditionDegradedLine(reason string) string {
	return "这次的 A/B 试听对比没能生成（原因：" + firstNonEmpty(reason, "渲染不可用") + "），这一步已经应用并回读验证；如果听感不对，可以说「撤销这一步」回滚。"
}

// mixTickAuditionProtocolStamp is the provenance block carried on the applied
// reply's workflow data, so a run's A/B state is auditable without log diving.
func mixTickAuditionProtocolStamp(bracket mixTickAuditionBracket) map[string]any {
	stamp := map[string]any{"schema_version": mixTickAuditionSchemaVersion, "mounted": bracket.Record != nil}
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

// mixTickJudgmentEntry is the three-state input of the terminal wording:
// manual with a mounted entry, manual with a degraded one, and full access
// (whose b6 wording is deliberately unchanged).
type mixTickJudgmentEntry struct {
	Manual   bool
	Mounted  bool
	Degraded string
}
