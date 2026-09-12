package chat

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

const auditionSchemaVersion = "vit.kernel_audition.v1"

// auditionBlindEnvVar is the minimal configuration entry for the blind A/B
// tier (B12-3 batch protocol passes --blind through it). Unset/0 keeps the
// historical non-blind default: canonical assignment, canonical settlement,
// canonical panel copy, byte-for-byte.
const auditionBlindEnvVar = "VIT_DAW_AUDITION_BLIND"

const (
	auditionPhysicalBefore = "before"
	auditionPhysicalAfter  = "after"

	auditionBlindDisclosureSchema = "vit.audition_blind_disclosure.v1"
	auditionBlindDisclosureEvent  = "audition.blind_disclosure"

	auditionCandidateA = "candidate-a"
	auditionCandidateB = "candidate-b"
)

// auditionBlindSettings answers whether the blind tier runs and which
// configuration surface said so. The draw itself happens once per session
// (prepare), and only the boolean mode — never the drawn assignment — reaches
// the session snapshot.
type auditionBlindSettings struct {
	enabled bool
	source  string
}

// auditionBlindConfig is the agent-side configuration surface for the blind A/B
// tier (B12-3 `--blind` passthrough). Until this card the only switch was
// VIT_DAW_AUDITION_BLIND, which the agent reads exactly once at process start —
// a Godot-launched agent inherited an environment without it and the whole run
// silently stayed canonical. The file surface is read at each prepare: no
// process restart, no environment inheritance, no stale value.
//
// Path: $VIT_DAW_AGENT_ROOT (default D:/Vit_DAW) + VitApp/Workspace/agent_runtime_config.json
// Shape: {"audition_blind": true}  — the environment variable wins when it is set.
const (
	auditionBlindConfigEnvRoot = "VIT_DAW_AGENT_ROOT"
	auditionBlindConfigRoot    = "D:/Vit_DAW"
	auditionBlindConfigRelPath = "VitApp/Workspace/agent_runtime_config.json"
	auditionBlindConfigKey     = "audition_blind"
)

func auditionRootDir() string {
	if root := strings.TrimSpace(os.Getenv(auditionBlindConfigEnvRoot)); root != "" {
		return filepath.FromSlash(strings.ReplaceAll(root, "\\", "/"))
	}
	return filepath.FromSlash(auditionBlindConfigRoot)
}

func auditionBlindConfigPath() string {
	return filepath.Join(auditionRootDir(), filepath.FromSlash(auditionBlindConfigRelPath))
}

func auditionTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// auditionBlindSettingsFor resolves the blind mode for the next session.
// Precedence: VIT_DAW_AUDITION_BLIND, then the config file key. A missing file
// or a config without the key keeps the historical non-blind default; a file
// that cannot be parsed is reported as invalid instead of silently keeping the
// default, because a blind run that quietly became canonical is worse than a
// refused one.
func auditionBlindSettingsFor() auditionBlindSettings {
	source := "environment:" + auditionBlindEnvVar
	if raw, ok := os.LookupEnv(auditionBlindEnvVar); ok && strings.TrimSpace(raw) != "" {
		return auditionBlindSettings{enabled: auditionTruthy(raw), source: source}
	}

	path := auditionBlindConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return auditionBlindSettings{source: "default:absent"}
		}
		return auditionBlindSettings{source: "config_unreadable:" + path}
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return auditionBlindSettings{source: "config_invalid:" + path}
	}
	value, present := config[auditionBlindConfigKey]
	if !present {
		return auditionBlindSettings{enabled: false, source: "config:" + path}
	}
	switch typed := value.(type) {
	case bool:
		return auditionBlindSettings{enabled: typed, source: "config:" + path}
	case string:
		return auditionBlindSettings{enabled: auditionTruthy(typed), source: "config:" + path}
	}
	return auditionBlindSettings{source: "config_invalid:" + path}
}

// auditionBlindEnabled reports whether the blind tier is configured through any
// surface.
func auditionBlindEnabled() bool {
	return auditionBlindSettingsFor().enabled
}

// drawAuditionBlind is the session-level coin flip behind the blind
// assignment. Tests override Server.auditionBlindDraw for determinism.
func (s *Server) drawAuditionBlind() bool {
	if s != nil && s.auditionBlindDraw != nil {
		return s.auditionBlindDraw()
	}
	var buffer [1]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		// An unreadable entropy source must not silently turn a blind session
		// into a canonical one: keep the physical order (documented degenerate
		// draw) but the session still runs blind and still discloses.
		return false
	}
	return buffer[0]&1 == 1
}

type auditionCommandClient interface {
	AuditionPrepare(context.Context, kernel.AuditionSessionRequest) (*kernel.VSPCommandResult, error)
	AuditionStatus(context.Context, string) (*kernel.VSPCommandResult, error)
	AuditionSelect(context.Context, string, string) (*kernel.VSPCommandResult, error)
	AuditionStop(context.Context, string) (*kernel.VSPCommandResult, error)
}

func (s *Server) emitAuditionEvent(conversationID, eventType string, session map[string]any, details map[string]any) AgentEvent {
	payload := map[string]any{"schema_version": auditionSchemaVersion, "session": cloneContext(session)}
	for key, value := range details {
		payload[key] = value
	}
	sessionID := firstStringFromMap(session, "session_id")
	status := firstStringFromMap(session, "status")
	if status == "" {
		status = strings.TrimPrefix(eventType, "audition.")
	}
	return s.emitAgentEvent(conversationID, AgentEvent{
		Type: eventType, ItemID: sessionID, ItemType: "audition", Status: status,
		Title: "Kernel audition", Body: firstNonEmpty(firstStringFromMap(details, "message"), eventType),
		Payload: payload, Lifecycle: "transient", Persistence: "none", MessageKind: "activity",
		LogicalMessageID: "audition:" + sessionID + ":" + eventType,
	})
}

func (s *Server) ingestAuditionTelemetry(event map[string]any) {
	eventType := strings.TrimSpace(fmt.Sprint(event["type"]))
	if !strings.HasPrefix(eventType, "audition.") {
		return
	}
	session := firstMapFromAny(event["session"])
	conversationID := firstNonEmpty(firstStringFromMap(event, "conversation_id"), firstStringFromMap(session, "conversation_id"))
	if conversationID == "" {
		return
	}
	s.updateAuditionSessionSnapshot(conversationID, session)
	details := map[string]any{}
	for _, key := range []string{"code", "message"} {
		if value := event[key]; value != nil {
			details[key] = value
		}
	}
	s.emitAuditionEvent(conversationID, eventType, session, details)
	if eventType == "audition.ready" {
		s.requestAuditionJudgment(conversationID, firstStringFromMap(session, "session_id"))
	}
}

func (s *Server) emitRestoredAuditionProjections() {
	if s == nil {
		return
	}
	s.mu.Lock()
	loops := make([]freeStateReasoningLoop, 0, len(s.freeStateLoops))
	for _, loop := range s.freeStateLoops {
		loops = append(loops, cloneFreeStateLoop(loop))
	}
	s.mu.Unlock()
	for _, loop := range loops {
		if loop.Experiment == nil || loop.AuditionSessionID == "" || len(loop.AuditionSessionSnapshot) == 0 {
			continue
		}
		status := strings.ToLower(firstStringFromMap(loop.AuditionSessionSnapshot, "status"))
		eventType := "audition." + status
		if status == "playing" {
			eventType = "audition.selected"
		}
		s.emitAuditionEvent(loop.ConversationID, eventType, loop.AuditionSessionSnapshot, map[string]any{"restored": true})
		round, err := loop.Experiment.CurrentRound()
		if err != nil || round.AuditionSessionID != loop.AuditionSessionID {
			continue
		}
		if len(round.UserJudgmentEvidence) > 0 {
			evidence := round.UserJudgmentEvidence[len(round.UserJudgmentEvidence)-1]
			outcome := trajectory.EvaluationAmbiguous
			if evidence.HeardDifference == experiment.HeardDifferenceYes && (evidence.Preference == experiment.PreferenceA || evidence.Preference == experiment.PreferenceB) {
				outcome = trajectory.EvaluationHumanConfirmed
			}
			s.emitFreeStateExperimentEvents([]trajectory.Event{{
				Type: trajectory.EventUserJudgmentRecorded, ConversationID: loop.ConversationID,
				GoalID: loop.GoalID, RunID: loop.RunID, ItemID: "restored:judgment:" + evidence.ID,
				Title: "User judgment restored", Body: "recorded user A/B judgment restored",
				Payload: trajectory.Payload{SchemaVersion: trajectory.SchemaVersion, TraceNodeID: "restored:judgment:" + evidence.ID,
					TurnID: loop.Experiment.ID, RoundID: round.ID, NodeKind: trajectory.NodeJudgment, Phase: "user_judgment",
					Status: trajectory.StatusCompleted, Outcome: outcome,
					Details: map[string]any{"audition_session_id": loop.AuditionSessionID, "evidence": evidence, "restored": true}},
			}})
		} else if round.UserJudgmentRequested {
			s.emitFreeStateExperimentEvents([]trajectory.Event{{
				Type: trajectory.EventUserJudgmentRequested, ConversationID: loop.ConversationID,
				GoalID: loop.GoalID, RunID: loop.RunID, ItemID: "restored:judgment-request:" + loop.AuditionSessionID,
				Title: "User judgment restored", Body: "pending user A/B judgment restored",
				Payload: trajectory.Payload{SchemaVersion: trajectory.SchemaVersion, TraceNodeID: "restored:judgment-request:" + loop.AuditionSessionID,
					TurnID: loop.Experiment.ID, RoundID: round.ID, NodeKind: trajectory.NodeJudgment, Phase: "user_judgment",
					Status: trajectory.StatusWaiting, Outcome: trajectory.EvaluationHumanAuditionReady,
					Details: map[string]any{"audition_session_id": loop.AuditionSessionID, "restored": true}},
			}})
		}
	}
}

func (s *Server) updateAuditionSessionSnapshot(conversationID string, session map[string]any) {
	if s == nil || strings.TrimSpace(conversationID) == "" || len(session) == 0 {
		return
	}
	loop, ok := s.freeStateLoop(conversationID)
	if !ok {
		return
	}
	if loop.AuditionSessionID != "" && firstStringFromMap(session, "session_id") != "" && loop.AuditionSessionID != firstStringFromMap(session, "session_id") {
		return
	}
	if len(loop.AuditionSessionSnapshot) > 0 {
		merged := cloneContext(loop.AuditionSessionSnapshot)
		for key, value := range session {
			if key == "candidates" {
				merged[key] = mergeAuditionCandidateRows(firstMapRows(merged[key]), firstMapRows(value))
				continue
			}
			merged[key] = value
		}
		session = merged
	}
	loop.AuditionSessionSnapshot = cloneContext(session)
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
}

func auditionReplySession(result *kernel.VSPCommandResult) map[string]any {
	if result == nil {
		return nil
	}
	reply := result.LegacyLikeReply()
	if session := firstMapFromAny(reply["session"]); len(session) > 0 {
		return session
	}
	if session := firstMapFromAny(result.Payload["session"]); len(session) > 0 {
		return session
	}
	return firstMapFromAny(result.Response["session"])
}

func auditionReplyError(result *kernel.VSPCommandResult, err error) string {
	if err != nil {
		return err.Error()
	}
	if result == nil {
		return "audition command returned no result"
	}
	reply := result.LegacyLikeReply()
	if strings.EqualFold(firstStringFromMap(reply, "status"), "error") {
		return firstNonEmpty(firstStringFromMap(reply, "error", "message", "code"), "audition command failed")
	}
	return ""
}

type auditionActionRequest struct {
	ConversationID string `json:"conversation_id"`
	SessionID      string `json:"session_id"`
	CandidateID    string `json:"candidate_id,omitempty"`
}

type auditionJudgmentRequest struct {
	ConversationID  string   `json:"conversation_id"`
	TurnID          string   `json:"turn_id"`
	RoundID         string   `json:"round_id"`
	SessionID       string   `json:"audition_session_id"`
	EvidenceID      string   `json:"evidence_id,omitempty"`
	SupersedesID    string   `json:"supersedes_id,omitempty"`
	ProjectRevision string   `json:"project_revision"`
	HeardDifference string   `json:"heard_difference"`
	Preference      string   `json:"preference"`
	ReasonTags      []string `json:"reason_tags,omitempty"`
	FreeText        string   `json:"free_text,omitempty"`
}

func (s *Server) handleAuditionStatus(w http.ResponseWriter, r *http.Request) {
	s.handleAuditionCommand(w, r, "status")
}
func (s *Server) handleAuditionSelect(w http.ResponseWriter, r *http.Request) {
	s.handleAuditionCommand(w, r, "select")
}
func (s *Server) handleAuditionStop(w http.ResponseWriter, r *http.Request) {
	s.handleAuditionCommand(w, r, "stop")
}

func (s *Server) handleAuditionCommand(w http.ResponseWriter, r *http.Request, action string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var request auditionActionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	if strings.TrimSpace(request.ConversationID) == "" || strings.TrimSpace(request.SessionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "conversation_id and session_id are required"})
		return
	}
	if s.auditionKernel == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "error": "kernel unavailable"})
		return
	}
	var result *kernel.VSPCommandResult
	var err error
	switch action {
	case "status":
		result, err = s.auditionKernel.AuditionStatus(r.Context(), request.SessionID)
	case "select":
		if strings.TrimSpace(request.CandidateID) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "candidate_id is required"})
			return
		}
		// Selecting A/B is playback control only. It never records preference
		// and never settles the Experiment Round.
		result, err = s.auditionKernel.AuditionSelect(r.Context(), request.SessionID, request.CandidateID)
	case "stop":
		result, err = s.auditionKernel.AuditionStop(r.Context(), request.SessionID)
	}
	session := auditionReplySession(result)
	if failure := auditionReplyError(result, err); failure != "" {
		s.emitAuditionEvent(request.ConversationID, "audition.failed", session, map[string]any{"message": failure, "command": "audition." + action})
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": failure, "session": session})
		return
	}
	eventType := "audition." + action
	if action == "select" {
		eventType = "audition.selected"
	}
	if action == "stop" {
		eventType = "audition.stopped"
	}
	s.updateAuditionSessionSnapshot(request.ConversationID, session)
	s.persistCurrentProjectWorkspace()
	s.emitAuditionEvent(request.ConversationID, eventType, session, nil)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "session": session})
}

func (s *Server) handleAuditionJudgment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	defer s.beginContinuationSensitiveInvocation()()
	var request auditionJudgmentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	evidence, err := s.recordFreeStateAuditionJudgment(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	response := map[string]any{"status": "ok", "evidence": evidence}
	// Blind sessions answer with the un-blinding (physical assignment + the
	// action it produced); non-blind responses stay byte-for-byte as before.
	if disclosure := s.auditionBlindDisclosureForSession(request.ConversationID); len(disclosure) > 0 {
		response["blind_disclosure"] = disclosure
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) prepareFreeStateAudition(ctx context.Context, loop *freeStateReasoningLoop) error {
	if s == nil || loop == nil || loop.Experiment == nil {
		return fmt.Errorf("audition dependencies are unavailable")
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		return err
	}
	sessionID := "audition:" + loop.Experiment.ID + ":" + round.ID
	if loop.AuditionSessionID == sessionID {
		return nil
	}
	projectRevision := firstNonEmpty(round.ProjectRevision, firstStringFromMap(loop.LatestProjectChange, "project_revision", "revision"), round.CheckpointRef)
	if loop.Experiment.Admission.IsD1S1() && len(round.Interventions) == 1 {
		projectRevision = firstNonEmpty(firstStringFromMap(round.Interventions[0].Receipt, "after_revision", "applied_revision"), projectRevision)
	}
	projectRef := firstStringFromMap(loop.LatestProjectChange, "project_path")
	projectUUIDFromChange := firstStringFromMap(loop.LatestProjectChange, "project_uuid", "project_id")
	if s.auditionCandidateDriver != nil {
		if plane, planeErr := s.auditionCandidateDriver.CurrentPlane(ctx); planeErr == nil {
			projectRef = firstNonEmpty(projectRef, plane.ProjectPath)
			projectUUIDFromChange = firstNonEmpty(projectUUIDFromChange, plane.ProjectUUID)
			projectRevision = firstNonEmpty(projectRevision, plane.ProjectRevision)
		}
	}
	projectRef = firstNonEmpty(projectRef, "project:active")
	baselineCommit := strings.TrimSpace(round.CheckpointRef)
	baselineRef := "checkpoint:" + baselineCommit
	treatmentRef := baselineRef
	if len(round.Interventions) > 0 {
		treatmentRef = "action:" + round.Interventions[len(round.Interventions)-1].ID
	}
	projectUUID := projectUUIDFromChange
	request := kernel.AuditionSessionRequest{
		ConversationID: loop.ConversationID, SessionID: sessionID, Scope: "target",
		ActiveProjectRef: projectRef, ActiveProjectRevision: projectRevision, TimelineRevision: projectRevision,
		Candidates: []kernel.AuditionCandidate{
			{ID: "candidate-a", Label: "A", SourceKind: "checkpoint", SourceRef: baselineRef, CheckpointRef: baselineCommit, CommitID: baselineCommit, ProjectPath: projectRef, ProjectUUID: projectUUID, ProjectRevision: projectRevision},
			{ID: "candidate-b", Label: "B", SourceKind: "experiment", SourceRef: treatmentRef, ProjectPath: projectRef, ProjectUUID: projectUUID, ProjectRevision: projectRevision},
		},
	}
	if s.auditionCandidateDriver == nil {
		return fmt.Errorf("candidate project driver unavailable")
	}
	// The D2-2 multi-round tier must not interleave an agent-owned checkpoint
	// revision between its rounds' mutations: the frozen probe contract chains
	// round N+1's receipt directly onto round N's after_revision (2026-08-29
	// S3c smoke: the treatment checkpoint consumed revision 5 between round 1's
	// after 4 and round 2's apply). The treatment state stays durably
	// identified by the mutation receipt, the round-scoped journal action, and
	// the after render's own provenance; candidate B's kernel-required commit
	// reference carries the round-scoped action identity instead of a fresh
	// container commit. The single-round default path keeps its historical
	// checkpoint byte-for-byte.
	treatmentCommit := ""
	if !loop.Experiment.Admission.IsD2MultiRound() {
		treatmentCheckpoint, checkpointErr := s.auditionCandidateDriver.CreateCheckpoint(ctx, auditionCheckpointRequest{ProjectPath: projectRef, Message: "Audition candidate B treatment", Source: "audition.prepare", CheckpointKind: "audition_candidate_treatment", GoalID: loop.GoalID, RunID: loop.RunID})
		if checkpointErr != nil {
			return fmt.Errorf("create treatment candidate checkpoint: %w", checkpointErr)
		}
		treatmentCommit = firstStringFromMap(treatmentCheckpoint, "commit_id")
	} else if len(round.Interventions) > 0 {
		treatmentCommit = "action:" + round.Interventions[len(round.Interventions)-1].ID
	}
	if treatmentCommit != "" {
		request.Candidates[1].CheckpointRef = treatmentCommit
		request.Candidates[1].CommitID = treatmentCommit
	}
	// Blind mode is drawn once per session, and only the D1 render pair carries
	// a physical assignment to exchange (the legacy checkpoint/experiment pair
	// already identifies its own sides through source_kind). The draw itself
	// stays out of the snapshot: the session records that it runs blind, never
	// which render sits behind which label.
	blind := false
	if loop.Experiment.Admission.IsD1S1() {
		before := firstMapFromAny(loop.D1State["before_render"])
		if firstStringFromMap(before, "status") != "ready" || !validD1RenderFile(firstStringFromMap(before, "file_path")) {
			return fmt.Errorf("D1-S1 before render is unavailable")
		}
		after, renderErr := s.ensureD1Render(ctx, loop, "after", projectRevision, treatmentCommit)
		if renderErr != nil {
			return renderErr
		}
		blindSwap := false
		if settings := auditionBlindSettingsFor(); settings.enabled {
			blind = true
			blindSwap = s.drawAuditionBlind()
			if s.logger != nil {
				s.logger.Info("[audition] blind A/B tier enabled source=%s session=%s swapped=%v", settings.source, sessionID, blindSwap)
			}
		} else if s.logger != nil {
			// The absent-blind witness: the previous run could only be diagnosed
			// after the fact, from a candidate file name in the session snapshot.
			s.logger.Info("[audition] blind A/B tier disabled source=%s session=%s", settings.source, sessionID)
		}
		request.Candidates, renderErr = d1AuditionCandidatesForAssignment(before, after, baselineCommit, treatmentCommit, projectRef, projectUUID, blindSwap)
		if renderErr != nil {
			return renderErr
		}
	}
	if s.auditionKernel == nil {
		session := map[string]any{"session_id": sessionID, "conversation_id": loop.ConversationID, "status": "failed", "candidates": request.Candidates, "blind": blind}
		s.emitAuditionEvent(loop.ConversationID, "audition.failed", session, map[string]any{"message": "kernel unavailable", "command": "audition.prepare"})
		return fmt.Errorf("kernel unavailable")
	}
	result, callErr := s.auditionKernel.AuditionPrepare(ctx, request)
	session := auditionReplySession(result)
	if len(session) == 0 {
		session = map[string]any{"session_id": sessionID, "conversation_id": loop.ConversationID, "status": "preparing", "candidates": request.Candidates}
	}
	s.enrichAuditionSession(session, loop, round, request)
	session["blind"] = blind
	if failure := auditionReplyError(result, callErr); failure != "" {
		session["status"] = "failed"
		loop.AuditionSessionID = sessionID
		loop.AuditionSessionSnapshot = cloneContext(session)
		s.storeFreeStateLoop(*loop)
		s.emitAuditionEvent(loop.ConversationID, "audition.failed", session, map[string]any{"message": failure, "command": "audition.prepare"})
		return fmt.Errorf("audition.prepare: %s", failure)
	}
	loop.AuditionSessionID = sessionID
	loop.AuditionSessionSnapshot = cloneContext(session)
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(*loop)
	eventType := "audition.prepare"
	if strings.EqualFold(firstStringFromMap(session, "status"), "ready") {
		eventType = "audition.ready"
	}
	s.emitAuditionEvent(loop.ConversationID, eventType, session, map[string]any{"command": "audition.prepare"})
	if eventType == "audition.ready" {
		s.requestAuditionJudgment(loop.ConversationID, sessionID)
	}
	return nil
}

func (s *Server) enrichAuditionSession(session map[string]any, loop *freeStateReasoningLoop, round experiment.Round, request kernel.AuditionSessionRequest) {
	if session == nil || loop == nil {
		return
	}
	session["schema_version"] = auditionSchemaVersion
	session["conversation_id"] = loop.ConversationID
	session["turn_id"] = loop.Experiment.ID
	session["round_id"] = round.ID
	session["scope"] = request.Scope
	s.workspaceMu.Lock()
	activeWorkspaceUUID := s.activeWorkspaceUUID
	s.workspaceMu.Unlock()
	session["project_uuid"] = firstNonEmpty(firstStringFromMap(loop.LatestProjectChange, "project_uuid", "project_id"), activeWorkspaceUUID)
	session["project_revision"] = request.ActiveProjectRevision
	session["active_project_ref"] = request.ActiveProjectRef
	session["timeline_revision"] = request.TimelineRevision
	session["candidate_a_ref"] = request.Candidates[0].SourceRef
	session["candidate_b_ref"] = request.Candidates[1].SourceRef
	rows := auditionCandidateRows(session)
	if len(rows) == 0 {
		rows = []map[string]any{{"id": "candidate-a"}, {"id": "candidate-b"}}
	}
	for index := range request.Candidates {
		want := request.Candidates[index]
		for rowIndex := range rows {
			if firstStringFromMap(rows[rowIndex], "id") != want.ID {
				continue
			}
			data, _ := json.Marshal(want)
			projection := map[string]any{}
			_ = json.Unmarshal(data, &projection)
			for key, value := range projection {
				if !isEmptyProjectResultValue(value) {
					rows[rowIndex][key] = value
				}
			}
		}
	}
	values := make([]any, 0, len(rows))
	for _, row := range rows {
		values = append(values, row)
	}
	session["candidates"] = values
	if firstStringFromMap(session, "status") == "" {
		session["status"] = "preparing"
	}
}

func (s *Server) requestAuditionJudgment(conversationID, sessionID string) {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || loop.Experiment == nil || loop.AuditionSessionID != sessionID {
		return
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil || round.UserJudgmentRequested || len(round.UserJudgmentEvidence) > 0 {
		return
	}
	if !strings.EqualFold(firstStringFromMap(loop.AuditionSessionSnapshot, "status"), "ready") {
		return
	}
	if err := s.requireTaskHumanJudgment(&loop, sessionID, "A/B audition judgment is required before experiment settlement"); err != nil {
		if s.logger != nil {
			s.logger.Warn("[audition] canonical human judgment transition rejected: %v", err)
		}
		return
	}
	// The prepare and telemetry paths can both get past the round check above
	// inside the judgment request's write window; whoever lands second
	// observes the request the first path already parked — that is the dedup
	// success shape, recorded here instead of surfacing the historical
	// "user judgment is already pending" rejection.
	if fresh, ok := s.freeStateLoop(conversationID); ok && fresh.Experiment != nil {
		if round, err := fresh.Experiment.CurrentRound(); err == nil && (round.UserJudgmentRequested || len(round.UserJudgmentEvidence) > 0) {
			if s.logger != nil {
				s.logger.Info("[audition] audition.ready double-fire deduplicated for %s; judgment request already pending", sessionID)
			}
			return
		}
	}
	events, err := loop.Experiment.RequestUserJudgmentForSession("A/B audition required", sessionID, time.Now().UTC())
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[audition] user judgment request rejected: %v", err)
		}
		return
	}
	s.emitFreeStateExperimentEvents(events)
	s.storeFreeStateLoop(loop)
	s.persistCurrentProjectWorkspace()
}

func (s *Server) recordFreeStateAuditionJudgment(ctx context.Context, request auditionJudgmentRequest) (experiment.UserJudgmentEvidence, error) {
	s.activateCurrentProjectWorkspace(ctx)
	if strings.TrimSpace(request.ConversationID) == "" || strings.TrimSpace(request.TurnID) == "" || strings.TrimSpace(request.RoundID) == "" || strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.ProjectRevision) == "" {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("conversation_id, turn_id, round_id, audition_session_id, and project_revision are required")
	}
	loop, ok := s.freeStateLoop(request.ConversationID)
	if !ok || loop.Experiment == nil {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("experiment session not found")
	}
	if loop.Experiment.ID != request.TurnID || loop.AuditionSessionID != request.SessionID {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("audition session identity mismatch")
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		return experiment.UserJudgmentEvidence{}, err
	}
	if round.AuditionSessionID == "" && !round.UserJudgmentRequested {
		// A scheduler transport replay can drop the durable round binding
		// while the loop stays parked at the judgment boundary (canonical
		// task state human_judgment_required, round decision
		// user_judgment_pending, loop session identity intact). Re-establish
		// the binding through the guarded judgment-request API instead of
		// weakening the identity check below; every original guard still
		// applies.
		// The bind's canonical precondition is the human_judgment_required task
		// state. The audition-ready callback that normally drives that
		// transition can miss the parked window (the session snapshot reaches
		// ready only after the settle turn), leaving the task short of the
		// state RequestUserJudgmentForSession requires — complete the same
		// guarded transition here so the parked boundary stays servable
		// (2026-08-29 S3c smoke: judgment POST -> round identity mismatch).
		bindDiagnosis := ""
		if s.hasTaskSemanticContract(loop.GoalID) {
			if goal := s.harness.RuntimeStatus(loop.GoalID); goal.Task != nil && goal.Task.SemanticState != nil &&
				goal.Task.SemanticState.State != taskstate.StateHumanJudgmentRequired {
				if transitionErr := s.requireTaskHumanJudgment(&loop, request.SessionID, "A/B audition judgment is required before experiment settlement"); transitionErr != nil {
					bindDiagnosis = "task transition: " + transitionErr.Error()
				}
			}
			// Re-bind the canonical state whatever it is: the experiment's
			// TaskState projection can lag the harness across the transport
			// replays that dropped the round binding (2026-08-29 S3c smoke:
			// the harness had already transitioned to human_judgment_required
			// while the experiment still projected needs_experiment).
			if goal := s.harness.RuntimeStatus(loop.GoalID); goal.Task != nil && goal.Task.SemanticState != nil && goal.Task.Contract != nil {
				if bindErr := loop.Experiment.BindTaskState(goal.Task.Contract.ContractID, goal.Task.SemanticState.State, goal.Task.SemanticState.Revision); bindErr != nil {
					bindDiagnosis = firstNonEmpty(bindDiagnosis+" ; ", "") + "task rebind: " + bindErr.Error()
				}
			}
		}
		if events, bindErr := loop.Experiment.RequestUserJudgmentForSession("A/B audition required", request.SessionID, time.Now().UTC()); bindErr == nil {
			s.emitFreeStateExperimentEvents(events)
			s.storeFreeStateLoop(loop)
			round, err = loop.Experiment.CurrentRound()
			if err != nil {
				return experiment.UserJudgmentEvidence{}, err
			}
		} else {
			bindDiagnosis = firstNonEmpty(bindDiagnosis+" ; ", "") + "bind: " + bindErr.Error()
		}
		if bindDiagnosis != "" {
			return experiment.UserJudgmentEvidence{}, fmt.Errorf("round identity mismatch (%s; task_state=%s)", bindDiagnosis, loop.Experiment.TaskState)
		}
	}
	if round.ID != request.RoundID || round.AuditionSessionID != request.SessionID {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("round identity mismatch")
	}
	sessionProjectUUID := firstStringFromMap(loop.AuditionSessionSnapshot, "project_uuid")
	if sessionProjectUUID != "" && s.activeWorkspaceUUID != "" && sessionProjectUUID != s.activeWorkspaceUUID {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("audition session belongs to a different project")
	}
	sessionRevision := firstNonEmpty(firstStringFromMap(loop.AuditionSessionSnapshot, "project_revision"), round.ProjectRevision)
	liveRevision := firstStringFromMap(loop.LatestProjectChange, "project_revision", "revision")
	if liveRevision == "" {
		// A scheduler transport replay can also drop LatestProjectChange. The
		// newest intervention receipt's after_revision is the authoritative
		// live revision at the judgment boundary; the round's own
		// ProjectRevision is the pre-mutation admission revision and must not
		// be used as the current revision after a forward mutation.
		if interventions := round.Interventions; len(interventions) > 0 {
			liveRevision = firstStringFromMap(interventions[len(interventions)-1].Receipt, "after_revision", "applied_revision")
		}
		if liveRevision == "" {
			liveRevision = round.ProjectRevision
		}
	}
	if request.ProjectRevision != sessionRevision || (liveRevision != "" && liveRevision != sessionRevision) {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("project revision mismatch")
	}
	candidates := auditionCandidateRows(loop.AuditionSessionSnapshot)
	if len(candidates) != 2 || !auditionCandidatesReady(candidates) {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("candidate A/B are not both ready")
	}
	sessionStatus := strings.ToLower(firstStringFromMap(loop.AuditionSessionSnapshot, "status"))
	if sessionStatus != "ready" && sessionStatus != "playing" && sessionStatus != "stopped" {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("audition session is not ready: %s", sessionStatus)
	}
	heard := experiment.HeardDifference(strings.ToLower(strings.TrimSpace(request.HeardDifference)))
	preference := experiment.JudgmentPreference(strings.ToLower(strings.TrimSpace(request.Preference)))
	if heard != experiment.HeardDifferenceYes {
		preference = experiment.PreferenceUnsure
	}
	if preference == "" {
		preference = experiment.PreferenceUnsure
	}
	evidence := buildUserJudgmentEvidence(loop, round, request.SessionID, heard, preference, request.ReasonTags, request.FreeText)
	evidence.ID = strings.TrimSpace(request.EvidenceID)
	evidence.SupersedesID = strings.TrimSpace(request.SupersedesID)
	// The physical direction of a blind session is resolved before the
	// immutable evidence row lands: a row that cannot be mapped would settle an
	// A/B preference with no physical direction, which is the exact
	// retain/rollback inversion this path exists to prevent. Nothing about the
	// mapping is recorded here — only its resolvability is checked.
	auditionSession := cloneContext(loop.AuditionSessionSnapshot)
	blindSession := auditionSessionIsBlind(auditionSession)
	baselineRef := round.CheckpointRef
	if blindSession && evidence.SupersedesID == "" && auditionPreferenceNeedsPhysicalDirection(heard, preference) {
		if _, mapped := auditionPhysicalMappingForEvidence(evidence, baselineRef); !mapped {
			return experiment.UserJudgmentEvidence{}, fmt.Errorf("blind audition session cannot resolve the physical candidate mapping from the recorded candidates; refusing to record preference %q without a physical direction", preference)
		}
	}
	events, err := loop.Experiment.RecordUserJudgmentEvidence(evidence, time.Now().UTC())
	if err != nil {
		return experiment.UserJudgmentEvidence{}, err
	}
	if recordedRound, recordedErr := loop.Experiment.CurrentRound(); recordedErr == nil && len(recordedRound.UserJudgmentEvidence) > 0 {
		evidence = recordedRound.UserJudgmentEvidence[len(recordedRound.UserJudgmentEvidence)-1]
	}
	if blindSession {
		// The recorded judgment event carries the session's blind mode so the
		// evidence projection states it; the physical assignment follows only
		// after the action has landed (audition.blind_disclosure).
		for index := range events {
			if events[index].Payload.Details == nil {
				events[index].Payload.Details = map[string]any{}
			}
			events[index].Payload.Details["blind"] = true
		}
	}
	s.emitFreeStateExperimentEvents(events)
	// A correction is a new immutable evidence row. It does not silently
	// replay adoption/rollback against an already-settled round.
	if evidence.SupersedesID != "" {
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		s.persistCurrentProjectWorkspace()
		return evidence, nil
	}
	if loop.Experiment.Admission.IsD1S1() {
		// D1-S1 already applied its only forward mutation before audition. Human
		// preference therefore settles retain/rollback directly; candidate apply
		// would be an illegal second mutation.
		if err := s.applyFreeStateJudgmentOutcome(ctx, &loop, evidence); err != nil {
			loop.UpdatedAt = time.Now().UTC()
			s.storeFreeStateLoop(loop)
			s.persistCurrentProjectWorkspace()
			return experiment.UserJudgmentEvidence{}, err
		}
	} else if evidence.HeardDifference == experiment.HeardDifferenceYes && (evidence.Preference == experiment.PreferenceA || evidence.Preference == experiment.PreferenceB) {
		// Legacy candidate workflows retain the explicit apply boundary.
		loop.Status = "awaiting_candidate_apply"
		loop.AuditionSessionSnapshot["adoption_status"] = "pending"
		loop.AuditionSessionSnapshot["judgment_evidence_id"] = evidence.ID
	} else {
		// Ambiguous/no-difference evidence does not authorize a project change.
		// Keep the established G6 next-round policy, but without candidate adoption.
		if err := s.applyFreeStateJudgmentOutcome(ctx, &loop, evidence); err != nil {
			loop.UpdatedAt = time.Now().UTC()
			s.storeFreeStateLoop(loop)
			s.persistCurrentProjectWorkspace()
			return experiment.UserJudgmentEvidence{}, err
		}
	}
	// The physical assignment is disclosed only now, after the action landed:
	// before this point the mapping exists solely inside the evidence structure
	// the settlement reads.
	s.publishAuditionBlindDisclosure(&loop, auditionSession, evidence)
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	s.persistCurrentProjectWorkspace()
	return evidence, nil
}

// auditionBlindDisclosureForSession reads back the disclosure persisted with the
// session, for the judgment POST response.
func (s *Server) auditionBlindDisclosureForSession(conversationID string) map[string]any {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok {
		return nil
	}
	return firstMapFromAny(loop.AuditionSessionSnapshot["blind_disclosure"])
}

// publishAuditionBlindDisclosure emits and persists the post-landing un-blinding
// of a blind session. The event carries the session identity the panel already
// reduces, plus the disclosure at the payload top level.
func (s *Server) publishAuditionBlindDisclosure(loop *freeStateReasoningLoop, session map[string]any, evidence experiment.UserJudgmentEvidence) {
	if loop == nil {
		return
	}
	disclosure := auditionBlindDisclosure(loop, evidence)
	if len(disclosure) == 0 {
		return
	}
	eventSession := cloneContext(session)
	if len(eventSession) == 0 {
		eventSession = map[string]any{"session_id": evidence.AuditionSessionID, "conversation_id": loop.ConversationID}
	}
	eventSession["blind_disclosure"] = disclosure
	// The recalibration branch clears the session binding; never revive it here.
	if loop.AuditionSessionID != "" && len(loop.AuditionSessionSnapshot) > 0 &&
		firstStringFromMap(loop.AuditionSessionSnapshot, "session_id") == firstStringFromMap(eventSession, "session_id") {
		persisted := cloneContext(loop.AuditionSessionSnapshot)
		persisted["blind_disclosure"] = disclosure
		loop.AuditionSessionSnapshot = persisted
	}
	s.emitAuditionEvent(loop.ConversationID, auditionBlindDisclosureEvent, eventSession, map[string]any{"blind_disclosure": disclosure})
}

// auditionBlindDisclosure states what the two labels physically were and what
// the recorded judgment therefore did. It is derived from the same conversion
// the settlement uses, so the disclosed action is the settled action.
func auditionBlindDisclosure(loop *freeStateReasoningLoop, evidence experiment.UserJudgmentEvidence) map[string]any {
	if loop == nil || !auditionSessionIsBlind(loop.AuditionSessionSnapshot) {
		return nil
	}
	mapping, mapped := auditionPhysicalMappingForEvidence(evidence, auditionBaselineRef(loop))
	if !mapped {
		return nil
	}
	disposition, _, _, err := auditionSettlementDisposition(loop, evidence)
	if err != nil {
		return nil
	}
	selected := ""
	if evidence.Preference == experiment.PreferenceA || evidence.Preference == experiment.PreferenceB {
		selected = string(evidence.Preference)
	}
	action := map[experiment.RoundDecision]string{
		experiment.DecisionRetain:    "retain",
		experiment.DecisionRollback:  "rollback",
		experiment.DecisionNextRound: "next_round",
		experiment.DecisionStopped:   "stopped",
	}[disposition.Decision]
	physical := mapping.sideOfLabel(evidence.Preference)
	disclosure := map[string]any{
		"schema_version":       auditionBlindDisclosureSchema,
		"blind":                true,
		"candidate_a_physical": mapping.sideA,
		"candidate_b_physical": mapping.sideB,
		"mapping_source":       mapping.source,
		"selected":             selected,
		"selected_physical":    physical,
		"action":               action,
		"disclosed_at":         time.Now().UTC().Format(time.RFC3339Nano),
	}
	switch {
	case selected != "" && physical == auditionPhysicalAfter && disposition.Decision == experiment.DecisionRetain:
		disclosure["summary"] = "你选的 " + strings.ToUpper(selected) + " 是改动后状态 · 已保留"
	case selected != "" && physical == auditionPhysicalBefore && disposition.Decision == experiment.DecisionRollback:
		disclosure["summary"] = "你选的 " + strings.ToUpper(selected) + " 是改动前状态 · 已回滚到改动前"
	}
	return disclosure
}

func buildUserJudgmentEvidence(loop freeStateReasoningLoop, round experiment.Round, sessionID string, heard experiment.HeardDifference, preference experiment.JudgmentPreference, reasonTags []string, freeText string) experiment.UserJudgmentEvidence {
	session := loop.AuditionSessionSnapshot
	candidateA := auditionCandidateSnapshot(session, "candidate-a")
	candidateB := auditionCandidateSnapshot(session, "candidate-b")
	projectRevision := firstNonEmpty(firstStringFromMap(session, "project_revision"), round.ProjectRevision)
	transportAnchor := firstMapFromAny(session["transport_anchor"])
	if len(transportAnchor) == 0 {
		transportAnchor = firstMapFromAny(session["transport"])
	}
	loudnessReference := firstMapFromAny(session["loudness_reference"])
	candidateACheckpointRef := firstStringFromMap(candidateA, "checkpoint_ref")
	if candidateACheckpointRef == "" && firstStringFromMap(candidateA, "source_kind") == "checkpoint" {
		candidateACheckpointRef = strings.TrimPrefix(firstStringFromMap(candidateA, "source_ref"), "checkpoint:")
	}
	return experiment.UserJudgmentEvidence{
		SchemaVersion:  experiment.UserJudgmentEvidenceSchemaVersion,
		ConversationID: loop.ConversationID, TurnID: loop.Experiment.ID, RoundID: round.ID, AuditionSessionID: sessionID,
		CandidateARef:           firstNonEmpty(firstStringFromMap(candidateA, "source_ref"), firstStringFromMap(session, "candidate_a_ref"), "candidate-a"),
		CandidateBRef:           firstNonEmpty(firstStringFromMap(candidateB, "source_ref"), firstStringFromMap(session, "candidate_b_ref"), "candidate-b"),
		CandidateACheckpointRef: candidateACheckpointRef, CandidateBCheckpointRef: firstStringFromMap(candidateB, "checkpoint_ref"),
		CandidateACommitID: firstStringFromMap(candidateA, "commit_id"), CandidateBCommitID: firstStringFromMap(candidateB, "commit_id"),
		CandidateABranchRef: firstStringFromMap(candidateA, "branch_ref"), CandidateBBranchRef: firstStringFromMap(candidateB, "branch_ref"),
		CandidateAWorktreeRef: firstStringFromMap(candidateA, "worktree_ref"), CandidateBWorktreeRef: firstStringFromMap(candidateB, "worktree_ref"),
		CandidateAPreviewRef: firstStringFromMap(candidateA, "preview_ref"), CandidateBPreviewRef: firstStringFromMap(candidateB, "preview_ref"),
		CandidateARenderRevision: firstStringFromMap(candidateA, "render_revision"), CandidateBRenderRevision: firstStringFromMap(candidateB, "render_revision"),
		ProjectUUID: firstStringFromMap(session, "project_uuid"), ProjectRevision: projectRevision,
		Scope: firstStringFromMap(session, "scope"), TransportAnchor: cloneContext(transportAnchor), LoudnessReference: cloneContext(loudnessReference),
		AnalyticalEvidenceRefs: append([]string(nil), round.TargetResponse.EvidenceRefs...), HeardDifference: heard, Preference: preference,
		ReasonTags: append([]string(nil), reasonTags...), FreeText: strings.TrimSpace(freeText), CreatedAt: time.Now().UTC(),
	}
}

func firstMapRows(value any) []map[string]any {
	rows := []map[string]any{}
	items, ok := value.([]any)
	if !ok {
		return rows
	}
	for _, item := range items {
		if row := firstMapFromAny(item); len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

func mergeAuditionCandidateRows(previous, incoming []map[string]any) []any {
	byID := map[string]map[string]any{}
	order := []string{}
	for _, row := range append(previous, incoming...) {
		id := firstStringFromMap(row, "id")
		if id == "" {
			continue
		}
		if _, exists := byID[id]; !exists {
			order = append(order, id)
			byID[id] = map[string]any{}
		}
		for key, value := range row {
			if !isEmptyProjectResultValue(value) {
				byID[id][key] = value
			}
		}
	}
	out := make([]any, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

func auditionCandidateRows(session map[string]any) []map[string]any {
	rows := []map[string]any{}
	if values, ok := session["candidates"].([]any); ok {
		for _, value := range values {
			if row := firstMapFromAny(value); len(row) > 0 {
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func auditionCandidatesReady(rows []map[string]any) bool {
	if len(rows) != 2 {
		return false
	}
	seen := map[string]bool{}
	for _, row := range rows {
		id := firstStringFromMap(row, "id")
		if id == "" || seen[id] || strings.ToLower(firstStringFromMap(row, "status")) != "ready" || firstStringFromMap(row, "preview_ref") == "" {
			return false
		}
		seen[id] = true
	}
	return seen["candidate-a"] && seen["candidate-b"]
}

func auditionCandidateSnapshot(session map[string]any, candidateID string) map[string]any {
	for _, row := range auditionCandidateRows(session) {
		if firstStringFromMap(row, "id") == candidateID {
			return row
		}
	}
	return map[string]any{"id": candidateID}
}

// auditionPhysicalMapping is the label -> physical D1 render assignment of one
// audition session. It is derived, never stored in the session snapshot or the
// event stream: before the judgment lands it lives only in the candidate rows
// the evidence is built from (content-blind red line), and it is republished
// only as the post-landing blind disclosure.
type auditionPhysicalMapping struct {
	sideA  string
	sideB  string
	source string
}

func (m auditionPhysicalMapping) resolved() bool {
	return (m.sideA == auditionPhysicalBefore && m.sideB == auditionPhysicalAfter) ||
		(m.sideA == auditionPhysicalAfter && m.sideB == auditionPhysicalBefore)
}

// sideOfLabel returns the physical render the given label (a/b) carries.
func (m auditionPhysicalMapping) sideOfLabel(preference experiment.JudgmentPreference) string {
	switch preference {
	case experiment.PreferenceA:
		return m.sideA
	case experiment.PreferenceB:
		return m.sideB
	}
	return ""
}

// labelCarryingSide returns the label that carries the given physical render.
// Unresolved mappings answer the canonical assignment, which is the historical
// non-blind behavior.
func (m auditionPhysicalMapping) labelCarryingSide(side string) string {
	if m.resolved() {
		if m.sideA == side {
			return auditionCandidateA
		}
		if m.sideB == side {
			return auditionCandidateB
		}
	}
	if side == auditionPhysicalBefore {
		return auditionCandidateA
	}
	return auditionCandidateB
}

// auditionPhysicalMappingForEvidence resolves which physical render candidate A
// and candidate B played, using only fields the judgment evidence already
// carries (design §Q2: existing fields, zero extension). Key families are tried
// most-stable first and must agree; a collision (both labels on one side, or two
// families disagreeing) resolves to "unknown" so the settlement can fail closed
// instead of picking a direction.
func auditionPhysicalMappingForEvidence(evidence experiment.UserJudgmentEvidence, baselineRef string) (auditionPhysicalMapping, bool) {
	sides := make([][2]string, 0, 3)
	sides = append(sides, [2]string{
		auditionRenderRevisionSide(evidence.CandidateARenderRevision),
		auditionRenderRevisionSide(evidence.CandidateBRenderRevision),
	})
	sides = append(sides, [2]string{
		auditionSourceRefSide(evidence.CandidateARef),
		auditionSourceRefSide(evidence.CandidateBRef),
	})
	sides = append(sides, auditionCheckpointKeySides(evidence, baselineRef))
	names := []string{"render_revision", "candidate_source_ref", "checkpoint_identity"}
	resolved := auditionPhysicalMapping{}
	for index, pair := range sides {
		if pair[0] != "" && pair[0] == pair[1] {
			// Both labels claim the same physical render: the provenance keys
			// collided, so no direction is stable (card stop condition). Report
			// unmapped instead of falling through to a weaker family.
			return auditionPhysicalMapping{}, false
		}
		candidate := auditionPhysicalMapping{sideA: pair[0], sideB: pair[1], source: names[index]}
		if !candidate.resolved() {
			continue
		}
		if resolved.resolved() && (resolved.sideA != candidate.sideA || resolved.sideB != candidate.sideB) {
			// Provenance families disagree: the physical direction is not
			// stable, which is the card's stop condition. Report unmapped.
			return auditionPhysicalMapping{}, false
		}
		if !resolved.resolved() {
			// Most-stable-first: the first family that resolves names the
			// mapping; later families only have to agree.
			resolved = candidate
		}
	}
	return resolved, resolved.resolved()
}

// auditionRenderRevisionSide reads the render phase out of a D1 render
// revision (finishD1Render: "d1_render:<experiment>:<phase>:<revision>:<digest>").
// This is the primary key family: the phase token is written by the render that
// produced the audio, so it survives every label permutation.
func auditionRenderRevisionSide(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "d1_render:") {
		// d1_render:<experiment>:<phase>:<project_revision>:<digest>, and the
		// experiment identity itself may contain colons (turn:<id>), so the
		// phase is read from the tail instead of by position: the last
		// before/after token followed by a 16 hex digit render digest.
		parts := strings.Split(value, ":")
		for index := len(parts) - 3; index >= 1; index-- {
			if parts[index] != auditionPhysicalBefore && parts[index] != auditionPhysicalAfter {
				continue
			}
			if index+2 < len(parts) && isHexDigest16(parts[index+2]) {
				return parts[index]
			}
		}
		for index := len(parts) - 1; index >= 1; index-- {
			if parts[index] == auditionPhysicalBefore || parts[index] == auditionPhysicalAfter {
				return parts[index]
			}
		}
		return ""
	}
	if strings.HasPrefix(value, "render-") {
		switch strings.TrimPrefix(value, "render-") {
		case auditionPhysicalBefore:
			return auditionPhysicalBefore
		case auditionPhysicalAfter:
			return auditionPhysicalAfter
		}
	}
	return ""
}

func isHexDigest16(value string) bool {
	if len(value) != 16 {
		return false
	}
	for _, symbol := range strings.ToLower(value) {
		if (symbol < '0' || symbol > '9') && (symbol < 'a' || symbol > 'f') {
			return false
		}
	}
	return true
}

// auditionSourceRefSide reads the phase out of the render file name
// (ensureD1Render: "<phase>_revision_<project_revision>.wav").
func auditionSourceRefSide(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(value, auditionPhysicalBefore+"_revision_"):
		return auditionPhysicalBefore
	case strings.Contains(value, auditionPhysicalAfter+"_revision_"):
		return auditionPhysicalAfter
	}
	return ""
}

// auditionCheckpointKeySides binds the labels to the round's baseline
// checkpoint: the render taken at the baseline commit is the before render.
func auditionCheckpointKeySides(evidence experiment.UserJudgmentEvidence, baselineRef string) [2]string {
	baseline := auditionCheckpointIdentity(baselineRef)
	if baseline == "" {
		return [2]string{}
	}
	aMatches := auditionCheckpointIdentity(evidence.CandidateACheckpointRef) == baseline || auditionCheckpointIdentity(evidence.CandidateACommitID) == baseline
	bMatches := auditionCheckpointIdentity(evidence.CandidateBCheckpointRef) == baseline || auditionCheckpointIdentity(evidence.CandidateBCommitID) == baseline
	switch {
	case aMatches && !bMatches:
		return [2]string{auditionPhysicalBefore, auditionPhysicalAfter}
	case bMatches && !aMatches:
		return [2]string{auditionPhysicalAfter, auditionPhysicalBefore}
	}
	return [2]string{}
}

func auditionCheckpointIdentity(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "action:") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, "checkpoint:"))
}

// auditionSessionIsBlind reads the session-level blind mode flag. The flag names
// the mode, never the assignment: a blind session whose draw kept the canonical
// order is still blind, and the snapshot must not let a reader infer which
// render sits behind which label.
func auditionSessionIsBlind(session map[string]any) bool {
	if len(session) == 0 {
		return false
	}
	switch value := session["blind"].(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

// auditionPreferenceNeedsPhysicalDirection reports whether a judgment selects
// one of the two heard candidates: only those authorize a retain/rollback and
// therefore need a physical direction.
func auditionPreferenceNeedsPhysicalDirection(heard experiment.HeardDifference, preference experiment.JudgmentPreference) bool {
	return heard == experiment.HeardDifferenceYes && (preference == experiment.PreferenceA || preference == experiment.PreferenceB)
}

func auditionBaselineRef(loop *freeStateReasoningLoop) string {
	if loop == nil || loop.Experiment == nil {
		return ""
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		return ""
	}
	return round.CheckpointRef
}

func auditionCandidateLetter(candidateID string) string {
	switch candidateID {
	case auditionCandidateA:
		return "A"
	case auditionCandidateB:
		return "B"
	}
	return strings.ToUpper(strings.TrimSpace(candidateID))
}

// auditionJudgmentSelectsPhysicalSide reports whether the recorded judgment
// selects the candidate that physically carries the given D1 render side.
// Evidence without resolvable render provenance answers the historical label
// semantics, so legacy non-blind receipts stay byte-for-byte.
func auditionJudgmentSelectsPhysicalSide(evidence experiment.UserJudgmentEvidence, baselineRef, side string) bool {
	if evidence.HeardDifference != experiment.HeardDifferenceYes ||
		(evidence.Preference != experiment.PreferenceA && evidence.Preference != experiment.PreferenceB) {
		return false
	}
	if mapping, mapped := auditionPhysicalMappingForEvidence(evidence, baselineRef); mapped {
		return mapping.sideOfLabel(evidence.Preference) == side
	}
	if side == auditionPhysicalAfter {
		return evidence.Preference == experiment.PreferenceB
	}
	return evidence.Preference == experiment.PreferenceA
}

// auditionSettlementDisposition is the single source of truth for the label ->
// physical action conversion, shared by the settlement and by the post-landing
// disclosure so the two can never drift.
//
// The historical mapping is label-level: preference B = retain the treatment,
// preference A = roll back to the baseline. A blind session exchanged the
// physical renders behind those fixed labels, so the disposition is recomputed
// from the physical side the user's label actually carries. Non-blind evidence
// resolves to the canonical assignment and reproduces the label-level answer
// byte-for-byte; unmapped blind evidence fails closed rather than settling an
// A/B preference without a direction.
func auditionSettlementDisposition(loop *freeStateReasoningLoop, evidence experiment.UserJudgmentEvidence) (auditionJudgmentDisposition, auditionPhysicalMapping, bool, error) {
	if loop == nil || loop.Experiment == nil {
		return auditionJudgmentDisposition{}, auditionPhysicalMapping{}, false, fmt.Errorf("experiment session unavailable")
	}
	disposition := dispositionForUserJudgment(evidence)
	if freeStateAdmissionRunsSingleRound(loop.Experiment.Admission) {
		disposition = d1DispositionForUserJudgment(evidence)
	}
	if disposition.Decision != experiment.DecisionRetain && disposition.Decision != experiment.DecisionRollback {
		return disposition, auditionPhysicalMapping{}, false, nil
	}
	mapping, mapped := auditionPhysicalMappingForEvidence(evidence, auditionBaselineRef(loop))
	if !mapped {
		if auditionSessionIsBlind(loop.AuditionSessionSnapshot) {
			return disposition, mapping, false, fmt.Errorf("blind audition session cannot resolve the physical candidate mapping for preference %q; refusing to settle without a physical direction", evidence.Preference)
		}
		return disposition, mapping, false, nil
	}
	switch mapping.sideOfLabel(evidence.Preference) {
	case auditionPhysicalAfter:
		disposition = auditionJudgmentDisposition{Decision: experiment.DecisionRetain, Outcome: experiment.OutcomeImproved}
	case auditionPhysicalBefore:
		disposition = auditionJudgmentDisposition{Decision: experiment.DecisionRollback, Outcome: experiment.OutcomeRolledBack}
	}
	return disposition, mapping, true, nil
}

type auditionJudgmentDisposition struct {
	Decision experiment.RoundDecision
	Outcome  experiment.SettlementOutcome
	Continue bool
}

// dispositionForUserJudgment implements GLM ruling 1's split of the historical
// conflated "ambiguous" outcome:
//   - heard_difference != yes is an effect-insufficiency signal, not
//     ambiguity: it stays a next-round recalibration request (the single-round
//     tier converts it to a terminal stop in d1DispositionForUserJudgment);
//   - a heard difference with preference neither/unsure/equal is true
//     ambiguity: terminal for every tier, mirroring the D1-S1 outcome, and it
//     never authorizes an automatic next round or dose change (ADR §8).
func dispositionForUserJudgment(evidence experiment.UserJudgmentEvidence) auditionJudgmentDisposition {
	if evidence.HeardDifference != experiment.HeardDifferenceYes {
		return auditionJudgmentDisposition{Decision: experiment.DecisionNextRound, Continue: true}
	}
	switch evidence.Preference {
	case experiment.PreferenceB:
		return auditionJudgmentDisposition{Decision: experiment.DecisionRetain, Outcome: experiment.OutcomeImproved}
	case experiment.PreferenceA:
		return auditionJudgmentDisposition{Decision: experiment.DecisionRollback, Outcome: experiment.OutcomeRolledBack}
	default:
		return auditionJudgmentDisposition{Decision: experiment.DecisionStopped, Outcome: experiment.OutcomeNeedsJudgment}
	}
}

func d1DispositionForUserJudgment(evidence experiment.UserJudgmentEvidence) auditionJudgmentDisposition {
	disposition := dispositionForUserJudgment(evidence)
	if disposition.Decision == experiment.DecisionNextRound {
		return auditionJudgmentDisposition{Decision: experiment.DecisionStopped, Outcome: experiment.OutcomeNeedsJudgment}
	}
	return disposition
}

func annotateJudgmentDecision(events []trajectory.Event, actionKind, candidateID string) {
	if len(events) == 0 {
		return
	}
	if events[0].Payload.Details == nil {
		events[0].Payload.Details = map[string]any{}
	}
	events[0].Payload.Details["action_kind"] = actionKind
	events[0].Payload.Details["candidate_id"] = candidateID
	events[0].Payload.Details["recoverable"] = true
}

// alignAuditionTargetResponseWithPhysicalSelection makes the round's target
// response agree with the candidate the user physically confirmed.
//
// The experiment package derives that response from the label alone
// (experiment/user_judgment.go:203-211 marks sufficient/confirmed for
// preference B, and runtime.go:801 then refuses a retain decision unless the
// response is sufficient). With fixed labels that is correct; once a blind
// session exchanged the renders, confirming the treatment can arrive as
// preference A, and the package-level write would leave the round describing
// the opposite action. Only that swapped case is compensated here — the
// canonical path already carries the response and returns immediately.
func alignAuditionTargetResponseWithPhysicalSelection(loop *freeStateReasoningLoop, evidence experiment.UserJudgmentEvidence) error {
	if loop == nil || loop.Experiment == nil {
		return fmt.Errorf("experiment session unavailable")
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		return err
	}
	if round.TargetResponse == nil {
		return fmt.Errorf("retain settlement requires a target response")
	}
	if round.TargetResponse.Response == experiment.TargetSufficient {
		return nil
	}
	index := len(loop.Experiment.Rounds) - 1
	if index < 0 || loop.Experiment.Rounds[index].ID != round.ID {
		return fmt.Errorf("retain settlement cannot locate the judged round")
	}
	round.TargetResponse.Response = experiment.TargetSufficient
	round.TargetResponse.Outcome = trajectory.EvaluationHumanConfirmed
	round.UpdatedAt = time.Now().UTC()
	loop.Experiment.Rounds[index] = round
	return nil
}

func (s *Server) applyFreeStateJudgmentOutcome(ctx context.Context, loop *freeStateReasoningLoop, evidence experiment.UserJudgmentEvidence) error {
	if loop == nil || loop.Experiment == nil {
		return fmt.Errorf("experiment session unavailable")
	}
	// The tier predicate, not bare IsD1S1: a D2-2 multi-round admission is a
	// domain member (IsD1S1()==true) that must keep the recalibration path.
	singleRound := freeStateAdmissionRunsSingleRound(loop.Experiment.Admission)
	disposition, mapping, mapped, err := auditionSettlementDisposition(loop, evidence)
	if err != nil {
		return err
	}
	// The label the settlement names is the label that physically carries the
	// acted-on render: canonical non-blind evidence resolves to candidate-B for
	// retain and candidate-A for rollback exactly as before, while a swapped
	// blind session names the label the user actually picked.
	retainedLabel := mapping.labelCarryingSide(auditionPhysicalAfter)
	rollbackLabel := mapping.labelCarryingSide(auditionPhysicalBefore)
	if !mapped {
		retainedLabel, rollbackLabel = auditionCandidateB, auditionCandidateA
	}
	switch disposition.Decision {
	case experiment.DecisionRetain:
		// The retained candidate physically carries the treatment already
		// present in the active project. Retain is the explicit, recoverable
		// adoption action for this G5 model.
		if err := alignAuditionTargetResponseWithPhysicalSelection(loop, evidence); err != nil {
			return err
		}
		retainedLetter := auditionCandidateLetter(retainedLabel)
		events, err := loop.Experiment.DecideRound(experiment.DecisionRetain, "user preferred "+retainedLetter+"; explicitly retained treatment candidate", time.Now().UTC())
		if err != nil {
			return err
		}
		annotateJudgmentDecision(events, "audition.retain_candidate", retainedLabel)
		s.emitFreeStateExperimentEvents(events)
		if s.hasTaskSemanticContract(loop.GoalID) {
			if err = s.settleTaskFromExperiment(loop, "user preferred "+retainedLetter+"; treatment candidate retained", []string{evidence.ID}); err != nil {
				return err
			}
		}
		events, err = loop.Experiment.Settle(experiment.OutcomeImproved, "user preferred "+retainedLetter+"; treatment candidate retained", time.Now().UTC())
		if err != nil {
			return err
		}
		s.emitFreeStateExperimentEvents(events)
		loop.Status = "completed"
	case experiment.DecisionRollback:
		events, err := loop.Experiment.DecideRound(experiment.DecisionRollback, "user selected rollback to baseline candidate", time.Now().UTC())
		if err != nil {
			return err
		}
		annotateJudgmentDecision(events, "audition.rollback_candidate", rollbackLabel)
		s.emitFreeStateExperimentEvents(events)
		rollbackEvents, err := s.rollbackFreeStateExperiment(ctx, loop)
		if err != nil {
			return err
		}
		s.emitFreeStateExperimentEvents(rollbackEvents)
		if s.hasTaskSemanticContract(loop.GoalID) {
			if err = s.settleTaskFromExperiment(loop, "user selected baseline or rejected both candidates", []string{evidence.ID}); err != nil {
				return err
			}
		}
		events, err = loop.Experiment.Settle(experiment.OutcomeRolledBack, "user selected baseline or rejected both candidates", time.Now().UTC())
		if err != nil {
			return err
		}
		s.emitFreeStateExperimentEvents(events)
		loop.Status = "completed"
	default:
		if disposition.Decision == experiment.DecisionNextRound && !singleRound {
			if freeStateExperimentBudgetExhausted(loop.Experiment) {
				// Same frozen contract as the materiality calibration path: the
				// experiment budget is spent, so the no-difference insufficiency
				// settles budget-exhausted instead of opening a beyond-budget
				// round (rounds may never exceed the admission budget).
				if events, err := loop.Experiment.DecideRound(experiment.DecisionStopped, "no audible difference; multi-round budget exhausted", time.Now().UTC()); err == nil {
					s.emitFreeStateExperimentEvents(events)
				}
				s.settleFreeStateExperiment(loop, experiment.OutcomeBudgetExhausted, "no audible difference; multi-round budget exhausted")
				if loop.Experiment.Status == experiment.StatusSettled {
					loop.Status = "completed"
					loop.RequiresPostActionObservation = false
				}
				return nil
			}
			// GLM ruling 1: no audible difference is an insufficiency signal,
			// not ambiguity. Non-single-round tiers recalibrate in a next
			// round under the existing admission; opening the round never
			// raises a dose by itself, and any dose change still passes a new
			// admission over the domain table.
			current, err := loop.Experiment.CurrentRound()
			if err != nil {
				return err
			}
			events, err := loop.Experiment.DecideRound(experiment.DecisionNextRound, "no audible difference; recalibrate", time.Now().UTC())
			if err != nil {
				return err
			}
			annotateJudgmentDecision(events, "experiment.next_round", "")
			s.emitFreeStateExperimentEvents(events)
			views := append([]string(nil), current.RequestedViewIDs...)
			revision := firstNonEmpty(firstStringFromMap(loop.LatestProjectChange, "project_revision", "revision"), current.ProjectRevision)
			events, err = loop.Experiment.StartRound(views, current.CheckpointRef, revision, time.Now().UTC())
			if err != nil {
				return err
			}
			s.emitFreeStateExperimentEvents(events)
			// The landed judgment releases the boundary and round 2 is open:
			// the settled round-1 pair on LatestDecision (user_judgment_pending
			// + human_audition_ready) is factually stale from this instant.
			// Left in place, runAgentLoopChat's loop copy-back projects it onto
			// round 2's limit-stop envelope and
			// continuationRequiresUserInteraction parks the owed round's
			// checkpoint at an unanswerable empty-shell waiting_interaction —
			// the scheduler never claims it and the "继续" nudges are swallowed,
			// so round 2 never acts (2026-08-29 223957 trace,
			// round_two_interventions=0 after three nudges). Neutralize before
			// the armed continuation serializes the loop projection.
			neutralizeFreeStateBoundaryResidue(loop)
			// The recalibration round needs its pre-action base booked at the
			// boundary (mirroring the admission's before-observation booking):
			// the proposing turn's result envelope never reaches the
			// decision-gated booking path, and without a revision-matched base
			// the confirmed action dies at the VSP base revision gate.
			s.bookRecalibrationRoundBaseFromLoop(loop)
			grantD2CalibrationRoundContinuation(loop)
			loop.AuditionSessionID = ""
			loop.AuditionSessionSnapshot = nil
			loop.Status = "active"
			// The judgment POST is out-of-band: the driving continuation chain
			// was drained at the audition boundary, so the opened round needs a
			// freshly armed continuation or the driver's continue nudge has
			// nothing to resume (2026-08-29 S3b/S3c smoke: no_continuation).
			s.armFreeStateRecalibrationContinuation(loop)
			return s.resumeTaskExperimentAfterJudgment(loop, "human judgment recorded; experiment remains open")
		}
		// True ambiguity (a difference was heard but neither candidate is
		// preferred) is terminal for every tier, mirroring the D1-S1 outcome
		// (GLM ruling 1 + ADR §8): it records the uncertainty and settles
		// without any further mutation.
		events, err := loop.Experiment.DecideRound(experiment.DecisionStopped, "ambiguous audition judgment; no further mutation permitted", time.Now().UTC())
		if err != nil {
			return err
		}
		annotateJudgmentDecision(events, "experiment.ambiguous", "")
		s.emitFreeStateExperimentEvents(events)
		if s.hasTaskSemanticContract(loop.GoalID) {
			if err = s.settleTaskFromExperiment(loop, "human judgment was ambiguous; no further mutation performed", []string{evidence.ID}); err != nil {
				return err
			}
		}
		events, err = loop.Experiment.Settle(experiment.OutcomeNeedsJudgment, "human judgment was ambiguous; no further mutation performed", time.Now().UTC())
		if err != nil {
			return err
		}
		s.emitFreeStateExperimentEvents(events)
		loop.Status = "completed"
	}
	return nil
}
