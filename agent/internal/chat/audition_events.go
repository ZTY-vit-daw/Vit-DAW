package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/trajectory"
)

const auditionSchemaVersion = "vit.kernel_audition.v1"

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
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "evidence": evidence})
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
	projectRef := firstNonEmpty(firstStringFromMap(loop.LatestProjectChange, "project_path", "project_id", "project_uuid"), "project:active")
	baselineRef := "checkpoint:" + round.CheckpointRef
	treatmentRef := baselineRef
	if len(round.Interventions) > 0 {
		treatmentRef = "action:" + round.Interventions[len(round.Interventions)-1].ID
	}
	request := kernel.AuditionSessionRequest{
		ConversationID: loop.ConversationID, SessionID: sessionID, Scope: "target",
		ActiveProjectRef: projectRef, ActiveProjectRevision: projectRevision, TimelineRevision: projectRevision,
		Candidates: []kernel.AuditionCandidate{{ID: "candidate-a", Label: "A", SourceKind: "checkpoint", SourceRef: baselineRef}, {ID: "candidate-b", Label: "B", SourceKind: "experiment", SourceRef: treatmentRef}},
	}
	if s.auditionKernel == nil {
		session := map[string]any{"session_id": sessionID, "conversation_id": loop.ConversationID, "status": "failed", "candidates": request.Candidates}
		s.emitAuditionEvent(loop.ConversationID, "audition.failed", session, map[string]any{"message": "kernel unavailable", "command": "audition.prepare"})
		return fmt.Errorf("kernel unavailable")
	}
	result, callErr := s.auditionKernel.AuditionPrepare(ctx, request)
	session := auditionReplySession(result)
	if len(session) == 0 {
		session = map[string]any{"session_id": sessionID, "conversation_id": loop.ConversationID, "status": "preparing", "candidates": request.Candidates}
	}
	s.enrichAuditionSession(session, loop, round, request)
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
	if round.ID != request.RoundID || round.AuditionSessionID != request.SessionID {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("round identity mismatch")
	}
	sessionProjectUUID := firstStringFromMap(loop.AuditionSessionSnapshot, "project_uuid")
	if sessionProjectUUID != "" && s.activeWorkspaceUUID != "" && sessionProjectUUID != s.activeWorkspaceUUID {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("audition session belongs to a different project")
	}
	sessionRevision := firstNonEmpty(firstStringFromMap(loop.AuditionSessionSnapshot, "project_revision"), round.ProjectRevision)
	currentRevision := firstNonEmpty(firstStringFromMap(loop.LatestProjectChange, "project_revision", "revision"), round.ProjectRevision)
	if request.ProjectRevision != sessionRevision || (currentRevision != "" && currentRevision != sessionRevision) {
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
	events, err := loop.Experiment.RecordUserJudgmentEvidence(evidence, time.Now().UTC())
	if err != nil {
		return experiment.UserJudgmentEvidence{}, err
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
	// Persist the raw judgment before adoption/rollback. If the recoverable
	// project action fails, the user's evidence must still remain auditable.
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	s.persistCurrentProjectWorkspace()
	if err := s.applyFreeStateJudgmentOutcome(ctx, &loop, evidence); err != nil {
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		s.persistCurrentProjectWorkspace()
		return experiment.UserJudgmentEvidence{}, err
	}
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	s.persistCurrentProjectWorkspace()
	return evidence, nil
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

type auditionJudgmentDisposition struct {
	Decision experiment.RoundDecision
	Outcome  experiment.SettlementOutcome
	Continue bool
}

func dispositionForUserJudgment(evidence experiment.UserJudgmentEvidence) auditionJudgmentDisposition {
	if evidence.HeardDifference != experiment.HeardDifferenceYes {
		return auditionJudgmentDisposition{Decision: experiment.DecisionNextRound, Continue: true}
	}
	switch evidence.Preference {
	case experiment.PreferenceB:
		return auditionJudgmentDisposition{Decision: experiment.DecisionRetain, Outcome: experiment.OutcomeImproved}
	case experiment.PreferenceA, experiment.PreferenceNeither:
		return auditionJudgmentDisposition{Decision: experiment.DecisionRollback, Outcome: experiment.OutcomeRolledBack}
	default:
		return auditionJudgmentDisposition{Decision: experiment.DecisionNextRound, Continue: true}
	}
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

func (s *Server) applyFreeStateJudgmentOutcome(ctx context.Context, loop *freeStateReasoningLoop, evidence experiment.UserJudgmentEvidence) error {
	if loop == nil || loop.Experiment == nil {
		return fmt.Errorf("experiment session unavailable")
	}
	disposition := dispositionForUserJudgment(evidence)
	switch disposition.Decision {
	case experiment.DecisionRetain:
		// Candidate B is the treatment already present in the active project.
		// Retain is the explicit, recoverable adoption action for this G5 model.
		events, err := loop.Experiment.DecideRound(experiment.DecisionRetain, "user preferred B; explicitly retained treatment candidate", time.Now().UTC())
		if err != nil {
			return err
		}
		annotateJudgmentDecision(events, "audition.retain_candidate", "candidate-b")
		s.emitFreeStateExperimentEvents(events)
		events, err = loop.Experiment.Settle(experiment.OutcomeImproved, "user preferred B; treatment candidate retained", time.Now().UTC())
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
		annotateJudgmentDecision(events, "audition.rollback_candidate", "candidate-a")
		s.emitFreeStateExperimentEvents(events)
		rollbackEvents, err := s.rollbackFreeStateExperiment(ctx, loop)
		if err != nil {
			return err
		}
		s.emitFreeStateExperimentEvents(rollbackEvents)
		events, err = loop.Experiment.Settle(experiment.OutcomeRolledBack, "user selected baseline or rejected both candidates", time.Now().UTC())
		if err != nil {
			return err
		}
		s.emitFreeStateExperimentEvents(events)
		loop.Status = "completed"
	default:
		// Ambiguous human evidence means the current point or dose is not yet
		// useful. Keep the turn alive and start a fresh bounded round.
		current, err := loop.Experiment.CurrentRound()
		if err != nil {
			return err
		}
		events, err := loop.Experiment.DecideRound(experiment.DecisionNextRound, "ambiguous audition judgment; continue with a new point or dose", time.Now().UTC())
		if err != nil {
			return err
		}
		annotateJudgmentDecision(events, "experiment.next_round", "")
		s.emitFreeStateExperimentEvents(events)
		views := append([]string(nil), current.RequestedViewIDs...)
		events, err = loop.Experiment.StartRound(views, current.CheckpointRef, current.ProjectRevision, time.Now().UTC())
		if err != nil {
			return err
		}
		s.emitFreeStateExperimentEvents(events)
		loop.AuditionSessionID = ""
		loop.AuditionSessionSnapshot = nil
		loop.Status = "active"
	}
	return nil
}
