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
	details := map[string]any{}
	for _, key := range []string{"code", "message"} {
		if value := event[key]; value != nil {
			details[key] = value
		}
	}
	s.emitAuditionEvent(conversationID, eventType, session, details)
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
	s.emitAuditionEvent(request.ConversationID, eventType, session, nil)
	if action == "select" {
		if finalizeErr := s.finalizeFreeStateAuditionSelection(r.Context(), request.ConversationID, request.SessionID, request.CandidateID); finalizeErr != nil {
			s.emitAuditionEvent(request.ConversationID, "audition.failed", session, map[string]any{"message": finalizeErr.Error(), "command": "audition.select"})
			writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": finalizeErr.Error(), "session": session})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "session": session})
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
		Candidates: []kernel.AuditionCandidate{{ID: "candidate-a", Label: "Before", SourceKind: "checkpoint", SourceRef: baselineRef}, {ID: "candidate-b", Label: "After", SourceKind: "experiment", SourceRef: treatmentRef}},
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
	if failure := auditionReplyError(result, callErr); failure != "" {
		session["status"] = "failed"
		s.emitAuditionEvent(loop.ConversationID, "audition.failed", session, map[string]any{"message": failure, "command": "audition.prepare"})
		return fmt.Errorf("audition.prepare: %s", failure)
	}
	loop.AuditionSessionID = sessionID
	eventType := "audition.prepare"
	if strings.EqualFold(firstStringFromMap(session, "status"), "ready") {
		eventType = "audition.ready"
	}
	s.emitAuditionEvent(loop.ConversationID, eventType, session, map[string]any{"command": "audition.prepare"})
	if judgmentEvents, judgmentErr := loop.Experiment.RequestUserJudgment("A/B audition required", time.Now().UTC()); judgmentErr == nil {
		s.emitFreeStateExperimentEvents(judgmentEvents)
	}
	return nil
}

func (s *Server) finalizeFreeStateAuditionSelection(ctx context.Context, conversationID, sessionID, candidateID string) error {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || loop.Experiment == nil || loop.AuditionSessionID != sessionID {
		return nil
	}
	preferTreatment := candidateID == "candidate-b"
	judgmentEvents, err := loop.Experiment.RecordUserJudgment(candidateID, preferTreatment, "user selected "+candidateID, time.Now().UTC())
	if err != nil {
		return err
	}
	s.emitFreeStateExperimentEvents(judgmentEvents)
	decision := experiment.DecisionRollback
	outcome := experiment.OutcomeRolledBack
	if preferTreatment {
		decision = experiment.DecisionRetain
		outcome = experiment.OutcomeImproved
	}
	decisionEvents, err := loop.Experiment.DecideRound(decision, "audition selection: "+candidateID, time.Now().UTC())
	if err != nil {
		return err
	}
	s.emitFreeStateExperimentEvents(decisionEvents)
	if decision == experiment.DecisionRollback {
		rollbackEvents, rollbackErr := s.rollbackFreeStateExperiment(ctx, &loop)
		if rollbackErr != nil {
			return rollbackErr
		}
		s.emitFreeStateExperimentEvents(rollbackEvents)
	}
	settlementEvents, err := loop.Experiment.Settle(outcome, "audition selection settled: "+candidateID, time.Now().UTC())
	if err != nil {
		return err
	}
	s.emitFreeStateExperimentEvents(settlementEvents)
	loop.Status = "completed"
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	return nil
}
