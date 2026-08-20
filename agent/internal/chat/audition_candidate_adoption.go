package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/llm"
)

const candidateAdoptionReceiptSchema = "vit.audition_candidate_adoption_receipt.v1"

type auditionCandidateReference struct {
	CandidateID           string `json:"candidate_id"`
	SourceKind            string `json:"source_kind,omitempty"`
	SourceRef             string `json:"source_ref"`
	EngineeringSourceKind string `json:"engineering_source_kind,omitempty"`
	EngineeringSourceRef  string `json:"engineering_source_ref,omitempty"`
	CheckpointRef         string `json:"checkpoint_ref,omitempty"`
	CommitID              string `json:"commit_id,omitempty"`
	BranchRef             string `json:"branch_ref,omitempty"`
	WorktreeRef           string `json:"worktree_ref,omitempty"`
	ProjectPath           string `json:"project_path,omitempty"`
	ProjectUUID           string `json:"project_uuid,omitempty"`
	ProjectRevision       string `json:"project_revision,omitempty"`
	OwnerAgentID          string `json:"owner_agent_id,omitempty"`
	ReservationID         string `json:"reservation_id,omitempty"`
	ArtifactRef           string `json:"artifact_ref,omitempty"`
	PreviewRef            string `json:"preview_ref,omitempty"`
	RenderRevision        string `json:"render_revision,omitempty"`
}

type auditionProjectPlane struct {
	ProjectPath     string         `json:"project_path"`
	ProjectUUID     string         `json:"project_uuid,omitempty"`
	ProjectRevision string         `json:"project_revision,omitempty"`
	HistoryHead     string         `json:"history_head,omitempty"`
	ActiveBranch    string         `json:"active_branch,omitempty"`
	ActiveWorktree  string         `json:"active_worktree,omitempty"`
	History         map[string]any `json:"history,omitempty"`
}

type auditionCheckpointRequest struct {
	ProjectPath    string
	Message        string
	Source         string
	CheckpointKind string
	GoalID         string
	RunID          string
}

type auditionCandidateProjectDriver interface {
	CurrentPlane(context.Context) (auditionProjectPlane, error)
	CreateCheckpoint(context.Context, auditionCheckpointRequest) (map[string]any, error)
	CheckoutCandidate(context.Context, auditionCandidateReference) (map[string]any, error)
	RefreshPlane(context.Context, string) (auditionProjectPlane, map[string]any, error)
	RequestObservation(context.Context, freeStateReasoningLoop, experiment.Round) (map[string]any, *agentloop.RecentObservation, error)
}

type serverAuditionCandidateProjectDriver struct{ server *Server }

func (d *serverAuditionCandidateProjectDriver) CurrentPlane(ctx context.Context) (auditionProjectPlane, error) {
	if d == nil || d.server == nil || d.server.harness == nil {
		return auditionProjectPlane{}, fmt.Errorf("candidate project driver unavailable")
	}
	state := d.server.harness.StateSummary(ctx)
	path, uuid := d.server.harness.CurrentProjectIdentity(ctx)
	historyState := d.server.harness.ProjectHistorySummaryForProject(ctx, "", path)
	plane := auditionProjectPlane{ProjectPath: path, ProjectUUID: uuid, ProjectRevision: firstStringFromMap(state, "project_revision", "revision"), HistoryHead: firstStringFromMap(historyState, "head"), ActiveBranch: firstStringFromMap(historyState, "active_branch"), ActiveWorktree: firstStringFromMap(historyState, "active_worktree"), History: cloneContext(historyState)}
	if plane.ProjectPath == "" {
		return plane, fmt.Errorf("active project path unavailable")
	}
	return plane, nil
}

func (d *serverAuditionCandidateProjectDriver) CreateCheckpoint(ctx context.Context, request auditionCheckpointRequest) (map[string]any, error) {
	if d == nil || d.server == nil || d.server.harness == nil {
		return nil, fmt.Errorf("candidate checkpoint driver unavailable")
	}
	response, err := d.server.harness.Invoke(ctx, harness.InvokeRequest{Command: map[string]any{"cmd": "version_checkpoint", "project_path": request.ProjectPath, "message": request.Message, "source": request.Source, "checkpoint_kind": request.CheckpointKind}, Context: map[string]any{"audition_candidate_operation": true}, Source: request.Source, Confirmed: true, GoalID: request.GoalID, RunID: request.RunID})
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(response.Status, "error") || response.Error != "" {
		return nil, fmt.Errorf("candidate checkpoint failed: %s", firstNonEmpty(response.Error, response.Status))
	}
	out := cloneContext(response.Result)
	if out == nil {
		out = map[string]any{}
	}
	if firstStringFromMap(out, "commit_id") == "" {
		return nil, fmt.Errorf("candidate checkpoint returned no commit_id")
	}
	return out, nil
}

func (d *serverAuditionCandidateProjectDriver) CheckoutCandidate(ctx context.Context, ref auditionCandidateReference) (map[string]any, error) {
	if d == nil || d.server == nil || d.server.harness == nil {
		return nil, fmt.Errorf("candidate checkout driver unavailable")
	}
	cmd := map[string]any{"project_path": ref.ProjectPath}
	switch {
	case ref.WorktreeRef != "":
		cmd["cmd"] = "version_worktree_checkout"
		if strings.HasSuffix(strings.ToLower(ref.WorktreeRef), ".vit") || filepath.IsAbs(ref.WorktreeRef) {
			cmd["project_file_path"] = ref.WorktreeRef
		} else {
			cmd["name"] = ref.WorktreeRef
		}
	case ref.BranchRef != "":
		cmd["cmd"] = "version_checkout"
		cmd["branch"] = ref.BranchRef
	default:
		commitID := firstNonEmpty(ref.CommitID, ref.CheckpointRef, strings.TrimPrefix(ref.SourceRef, "checkpoint:"))
		if commitID == "" {
			return nil, fmt.Errorf("candidate %s has no recoverable commit, branch, or worktree reference", ref.CandidateID)
		}
		cmd["cmd"] = "version_checkout"
		cmd["commit_id"] = commitID
	}
	response, err := d.server.harness.Invoke(ctx, harness.InvokeRequest{Command: cmd, Context: map[string]any{"audition_candidate_operation": true, "candidate_id": ref.CandidateID}, Source: "audition_candidate", Confirmed: true})
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(response.Status, "error") || response.Error != "" {
		return nil, fmt.Errorf("candidate checkout failed: %s", firstNonEmpty(response.Error, response.Status))
	}
	out := cloneContext(response.Result)
	if out == nil {
		out = map[string]any{}
	}
	refresh := firstMapFromAny(out["refresh"])
	if len(refresh) > 0 && (!boolValue(refresh["kernel_reloaded"]) || !boolValue(refresh["shadow_refreshed"])) {
		return out, fmt.Errorf("candidate checkout refresh incomplete")
	}
	return out, nil
}

func (d *serverAuditionCandidateProjectDriver) RefreshPlane(ctx context.Context, reason string) (auditionProjectPlane, map[string]any, error) {
	if d == nil || d.server == nil || d.server.harness == nil {
		return auditionProjectPlane{}, nil, fmt.Errorf("candidate refresh driver unavailable")
	}
	d.server.harness.RefreshShadow(ctx, reason)
	plane, err := d.CurrentPlane(ctx)
	if err != nil {
		return plane, nil, err
	}
	change := d.server.harness.LatestAuthoritativeProjectChange(8)
	return plane, cloneContext(change), nil
}

func (d *serverAuditionCandidateProjectDriver) RequestObservation(ctx context.Context, loop freeStateReasoningLoop, round experiment.Round) (map[string]any, *agentloop.RecentObservation, error) {
	if d == nil || d.server == nil || d.server.harness == nil {
		return nil, nil, fmt.Errorf("candidate observation driver unavailable")
	}
	target := firstMapFromAny(loop.TargetRef)
	if len(target) == 0 {
		target = firstMapFromAny(loop.Experiment.Admission.TargetRef)
	}
	targetKind := firstNonEmpty(firstStringFromMap(target, "kind", "target_kind"), "project")
	targetID := firstNonEmpty(firstStringFromMap(target, "id", "target_id", "track_id"), "current")
	args := map[string]any{"request_id": "audition-adoption-" + sanitizeCanaryID(loop.ConversationID), "mix_session_id": "audition_" + sanitizeCanaryID(loop.ConversationID), "view_ids": append([]string(nil), round.RequestedViewIDs...), "target_kind": targetKind, "target_id": targetID, "max_disclosure_bytes": 8192, "freshness_class": "post_action"}
	response, err := d.server.harness.Invoke(ctx, harness.InvokeRequest{Tool: "ccb.observation_request", Args: args, Context: map[string]any{"audition_candidate_adoption": true, "observation_only": true}, Source: "audition_candidate_adoption", Confirmed: true, GoalID: loop.GoalID, RunID: loop.RunID, ToolCallID: "audition:" + sanitizeCanaryID(loop.ConversationID) + ":ccb.post"})
	if err != nil {
		return nil, nil, err
	}
	if !strings.EqualFold(response.Status, "ok") {
		return response.Result, nil, fmt.Errorf("post-adoption CCB observation failed: %s", firstNonEmpty(response.Error, response.Status))
	}
	bundle := firstMapFromAny(response.Result["bundle"])
	if len(bundle) == 0 {
		bundle = cloneContext(response.Result)
	}
	obs := &agentloop.RecentObservation{ToolCallID: "audition:" + sanitizeCanaryID(loop.ConversationID) + ":ccb.post", Tool: "ccb.observation_request", CommandName: "ccb_observation_request", Status: firstNonEmpty(firstStringFromMap(bundle, "status"), response.Status), Summary: cloneContext(bundle)}
	return bundle, obs, nil
}

type auditionConversationContext struct {
	Loop            freeStateReasoningLoop
	Messages        []llm.Message
	Goal            string
	Continuation    agentloop.Continuation
	HasContinuation bool
	Memory          agentloop.ExecutionMemory
	HasMemory       bool
}

func (s *Server) captureAuditionConversationContext(conversationID string) auditionConversationContext {
	out := auditionConversationContext{}
	if loop, ok := s.freeStateLoop(conversationID); ok {
		out.Loop = loop
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out.Messages = append([]llm.Message(nil), s.conversations[conversationID]...)
	out.Goal = s.conversationGoals[conversationID]
	out.Continuation, out.HasContinuation = s.goalContinuations[conversationID]
	out.Memory, out.HasMemory = s.conversationMemory[conversationID]
	return out
}

func (s *Server) restoreAuditionConversationContext(conversationID string, context auditionConversationContext) {
	s.mu.Lock()
	if s.conversations == nil {
		s.conversations = map[string][]llm.Message{}
	}
	if len(context.Messages) > 0 {
		s.conversations[conversationID] = append([]llm.Message(nil), context.Messages...)
	}
	if context.Goal != "" {
		s.conversationGoals[conversationID] = context.Goal
	}
	if context.HasContinuation {
		s.goalContinuations[conversationID] = context.Continuation
	}
	if context.HasMemory {
		s.conversationMemory[conversationID] = context.Memory
	}
	s.mu.Unlock()
	if context.Loop.ConversationID != "" {
		s.storeFreeStateLoop(context.Loop)
	}
}

type auditionCandidateOperationRequest struct {
	ConversationID     string `json:"conversation_id"`
	SessionID          string `json:"audition_session_id"`
	CandidateID        string `json:"candidate_id"`
	JudgmentEvidenceID string `json:"judgment_evidence_id,omitempty"`
}

type auditionCandidateAdoptionReceipt struct {
	SchemaVersion      string                     `json:"schema_version"`
	ID                 string                     `json:"id"`
	Operation          string                     `json:"operation"`
	Status             string                     `json:"status"`
	ConversationID     string                     `json:"conversation_id"`
	AuditionSessionID  string                     `json:"audition_session_id"`
	CandidateID        string                     `json:"candidate_id"`
	JudgmentEvidenceID string                     `json:"judgment_evidence_id,omitempty"`
	Before             auditionProjectPlane       `json:"before"`
	Candidate          auditionCandidateReference `json:"candidate"`
	Checkout           map[string]any             `json:"checkout,omitempty"`
	ProjectChange      map[string]any             `json:"project_change,omitempty"`
	Checkpoint         map[string]any             `json:"adoption_checkpoint,omitempty"`
	Observation        map[string]any             `json:"post_adoption_observation,omitempty"`
	Reservation        map[string]any             `json:"reservation,omitempty"`
	ApplyStrategy      string                     `json:"apply_strategy,omitempty"`
	Rollback           map[string]any             `json:"rollback,omitempty"`
	Error              string                     `json:"error,omitempty"`
	StartedAt          time.Time                  `json:"started_at"`
	CompletedAt        time.Time                  `json:"completed_at,omitempty"`
}

func newCandidateOperationReceipt(operation string, request auditionCandidateOperationRequest, before auditionProjectPlane, ref auditionCandidateReference) auditionCandidateAdoptionReceipt {
	receipt := auditionCandidateAdoptionReceipt{SchemaVersion: candidateAdoptionReceiptSchema, ID: "candidate-op-" + fmt.Sprint(time.Now().UTC().UnixNano()), Operation: operation, Status: "started", ConversationID: request.ConversationID, AuditionSessionID: request.SessionID, CandidateID: request.CandidateID, JudgmentEvidenceID: request.JudgmentEvidenceID, Before: before, Candidate: ref, StartedAt: time.Now().UTC()}
	if operation == "apply" {
		receipt.ApplyStrategy = "explicit_checkout"
	}
	return receipt
}

func candidateReferenceFromSession(session map[string]any, candidateID string) (auditionCandidateReference, error) {
	row := auditionCandidateSnapshot(session, candidateID)
	if firstStringFromMap(row, "id") != candidateID {
		return auditionCandidateReference{}, fmt.Errorf("candidate %s not found", candidateID)
	}
	ref := auditionCandidateReference{CandidateID: candidateID, SourceKind: firstStringFromMap(row, "source_kind"), SourceRef: firstStringFromMap(row, "source_ref"), EngineeringSourceKind: firstStringFromMap(row, "engineering_source_kind"), EngineeringSourceRef: firstStringFromMap(row, "engineering_source_ref"), CheckpointRef: firstStringFromMap(row, "checkpoint_ref"), CommitID: firstStringFromMap(row, "commit_id"), BranchRef: firstStringFromMap(row, "branch_ref"), WorktreeRef: firstStringFromMap(row, "worktree_ref"), ProjectPath: firstNonEmpty(firstStringFromMap(row, "project_path"), firstStringFromMap(session, "active_project_ref")), ProjectUUID: firstNonEmpty(firstStringFromMap(row, "project_uuid"), firstStringFromMap(session, "project_uuid")), ProjectRevision: firstNonEmpty(firstStringFromMap(row, "project_revision"), firstStringFromMap(session, "project_revision")), OwnerAgentID: firstStringFromMap(row, "owner_agent_id"), ReservationID: firstStringFromMap(row, "reservation_id"), ArtifactRef: firstStringFromMap(row, "artifact_ref"), PreviewRef: firstStringFromMap(row, "preview_ref"), RenderRevision: firstStringFromMap(row, "render_revision")}
	if ref.CheckpointRef == "" && ref.SourceKind == "checkpoint" {
		ref.CheckpointRef = strings.TrimPrefix(ref.SourceRef, "checkpoint:")
	}
	if ref.CommitID == "" {
		ref.CommitID = ref.CheckpointRef
	}
	if ref.ProjectPath == "" || firstNonEmpty(ref.CommitID, ref.BranchRef, ref.WorktreeRef) == "" {
		return ref, fmt.Errorf("candidate %s has no recoverable project reference", candidateID)
	}
	return ref, nil
}

func (s *Server) handleAuditionInspectCandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var request auditionCandidateOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	receipt, err := s.inspectAuditionCandidate(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": err.Error(), "receipt": receipt})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "receipt": receipt})
}

func (s *Server) handleAuditionApplyCandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var request auditionCandidateOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	receipt, err := s.applyAuditionCandidate(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": err.Error(), "receipt": receipt})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": receipt.Status, "receipt": receipt})
}

func (s *Server) auditionCandidateOperationContext(request auditionCandidateOperationRequest) (freeStateReasoningLoop, experiment.Round, auditionCandidateReference, error) {
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.CandidateID = strings.TrimSpace(request.CandidateID)
	if request.ConversationID == "" || request.SessionID == "" || request.CandidateID == "" {
		return freeStateReasoningLoop{}, experiment.Round{}, auditionCandidateReference{}, fmt.Errorf("conversation_id, audition_session_id, and candidate_id are required")
	}
	if request.CandidateID != "candidate-a" && request.CandidateID != "candidate-b" {
		return freeStateReasoningLoop{}, experiment.Round{}, auditionCandidateReference{}, fmt.Errorf("unsupported candidate_id %q", request.CandidateID)
	}
	loop, ok := s.freeStateLoop(request.ConversationID)
	if !ok || loop.Experiment == nil || loop.AuditionSessionID != request.SessionID {
		return loop, experiment.Round{}, auditionCandidateReference{}, fmt.Errorf("audition session not found")
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		return loop, round, auditionCandidateReference{}, err
	}
	if round.AuditionSessionID != "" && round.AuditionSessionID != request.SessionID {
		return loop, round, auditionCandidateReference{}, fmt.Errorf("round audition session mismatch")
	}
	ref, err := candidateReferenceFromSession(loop.AuditionSessionSnapshot, request.CandidateID)
	return loop, round, ref, err
}

func (s *Server) restoreCandidateSafetyPoint(ctx context.Context, before auditionProjectPlane, checkpoint map[string]any, operationContext auditionConversationContext) map[string]any {
	result := map[string]any{"status": "not_attempted"}
	if s == nil || s.auditionCandidateDriver == nil {
		result["error"] = "candidate project driver unavailable"
		return result
	}
	commitID := firstStringFromMap(checkpoint, "commit_id")
	if commitID == "" {
		result["error"] = "safety checkpoint is missing commit_id"
		return result
	}
	checkout, err := s.auditionCandidateDriver.CheckoutCandidate(ctx, auditionCandidateReference{CandidateID: "safety", SourceKind: "checkpoint", SourceRef: "checkpoint:" + commitID, CheckpointRef: commitID, CommitID: commitID, ProjectPath: before.ProjectPath, ProjectUUID: before.ProjectUUID})
	result["checkout"] = checkout
	if err != nil {
		result["status"] = "failed"
		result["error"] = err.Error()
		return result
	}
	s.activateCurrentProjectWorkspace(ctx)
	s.restoreAuditionConversationContext(operationContext.Loop.ConversationID, operationContext)
	plane, change, refreshErr := s.auditionCandidateDriver.RefreshPlane(ctx, "audition_candidate_operation_rollback")
	result["plane"] = plane
	result["project_change"] = change
	if refreshErr != nil {
		result["status"] = "failed"
		result["error"] = refreshErr.Error()
		return result
	}
	result["status"] = "rolled_back"
	return result
}

func planeMatchesCandidate(plane auditionProjectPlane, ref auditionCandidateReference) bool {
	if ref.ProjectPath != "" && !sameWorkspacePath(plane.ProjectPath, ref.ProjectPath) {
		return false
	}
	if ref.ProjectUUID != "" && plane.ProjectUUID != "" && ref.ProjectUUID != plane.ProjectUUID {
		return false
	}
	if ref.CommitID != "" && plane.HistoryHead != "" && ref.CommitID != plane.HistoryHead {
		return false
	}
	if ref.BranchRef != "" && plane.ActiveBranch != "" && ref.BranchRef != plane.ActiveBranch {
		return false
	}
	if ref.WorktreeRef != "" && plane.ActiveWorktree != "" && !filepath.IsAbs(ref.WorktreeRef) && !strings.HasSuffix(strings.ToLower(ref.WorktreeRef), ".vit") && ref.WorktreeRef != plane.ActiveWorktree {
		return false
	}
	return true
}

func (s *Server) inspectAuditionCandidate(ctx context.Context, request auditionCandidateOperationRequest) (auditionCandidateAdoptionReceipt, error) {
	loop, _, ref, err := s.auditionCandidateOperationContext(request)
	if err != nil {
		return auditionCandidateAdoptionReceipt{}, err
	}
	if s.auditionCandidateDriver == nil {
		return auditionCandidateAdoptionReceipt{}, fmt.Errorf("candidate project driver unavailable")
	}
	operationContext := s.captureAuditionConversationContext(request.ConversationID)
	before, err := s.auditionCandidateDriver.CurrentPlane(ctx)
	if err != nil {
		return auditionCandidateAdoptionReceipt{}, err
	}
	receipt := newCandidateOperationReceipt("inspect", request, before, ref)
	s.emitAuditionEvent(request.ConversationID, "audition.candidate.inspect.started", loop.AuditionSessionSnapshot, map[string]any{"candidate_id": request.CandidateID, "receipt": receipt})
	safety, err := s.auditionCandidateDriver.CreateCheckpoint(ctx, auditionCheckpointRequest{ProjectPath: before.ProjectPath, Message: "Safety checkpoint before audition candidate inspection", Source: "audition.inspect_candidate", CheckpointKind: "audition_candidate_inspect_safety", GoalID: loop.GoalID, RunID: loop.RunID})
	if err != nil {
		receipt.Status, receipt.Error, receipt.CompletedAt = "failed", err.Error(), time.Now().UTC()
		s.emitAuditionEvent(request.ConversationID, "audition.candidate.inspect.failed", loop.AuditionSessionSnapshot, map[string]any{"candidate_id": request.CandidateID, "receipt": receipt, "message": err.Error()})
		return receipt, err
	}
	receipt.Checkpoint = map[string]any{"safety": safety}
	checkout, err := s.auditionCandidateDriver.CheckoutCandidate(ctx, ref)
	receipt.Checkout = checkout
	if err == nil {
		s.activateCurrentProjectWorkspace(ctx)
		s.restoreAuditionConversationContext(request.ConversationID, operationContext)
		var plane auditionProjectPlane
		plane, receipt.ProjectChange, err = s.auditionCandidateDriver.RefreshPlane(ctx, "audition_inspect_candidate")
		if err == nil && !planeMatchesCandidate(plane, ref) {
			err = fmt.Errorf("inspected active project does not match candidate reference")
		}
	}
	if err != nil {
		receipt.Rollback = s.restoreCandidateSafetyPoint(ctx, before, safety, operationContext)
		receipt.Status, receipt.Error, receipt.CompletedAt = "failed", err.Error(), time.Now().UTC()
		s.restoreAuditionConversationContext(request.ConversationID, operationContext)
		s.emitAuditionEvent(request.ConversationID, "audition.candidate.inspect.failed", loop.AuditionSessionSnapshot, map[string]any{"candidate_id": request.CandidateID, "receipt": receipt, "message": err.Error()})
		return receipt, err
	}
	loop = operationContext.Loop
	loop.AuditionSessionSnapshot["inspected_candidate_id"] = request.CandidateID
	loop.AuditionSessionSnapshot["inspection_receipt"] = receipt
	receipt.Status, receipt.CompletedAt = "inspected", time.Now().UTC()
	loop.AuditionSessionSnapshot["inspection_receipt"] = receipt
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	s.persistCurrentProjectWorkspace()
	s.emitAuditionEvent(request.ConversationID, "audition.candidate.inspected", loop.AuditionSessionSnapshot, map[string]any{"candidate_id": request.CandidateID, "receipt": receipt})
	return receipt, nil
}

func latestJudgmentEvidence(round experiment.Round, evidenceID string) (experiment.UserJudgmentEvidence, error) {
	if len(round.UserJudgmentEvidence) == 0 {
		return experiment.UserJudgmentEvidence{}, fmt.Errorf("candidate adoption requires recorded user judgment evidence")
	}
	if evidenceID == "" {
		return round.UserJudgmentEvidence[len(round.UserJudgmentEvidence)-1], nil
	}
	for index := len(round.UserJudgmentEvidence) - 1; index >= 0; index-- {
		if round.UserJudgmentEvidence[index].ID == evidenceID {
			return round.UserJudgmentEvidence[index], nil
		}
	}
	return experiment.UserJudgmentEvidence{}, fmt.Errorf("judgment evidence %q not found", evidenceID)
}

func judgmentAllowsCandidate(evidence experiment.UserJudgmentEvidence, candidateID string) bool {
	if evidence.HeardDifference != experiment.HeardDifferenceYes {
		return false
	}
	return (candidateID == "candidate-a" && evidence.Preference == experiment.PreferenceA) || (candidateID == "candidate-b" && evidence.Preference == experiment.PreferenceB)
}

func (s *Server) applyAuditionCandidate(ctx context.Context, request auditionCandidateOperationRequest) (auditionCandidateAdoptionReceipt, error) {
	loop, round, ref, err := s.auditionCandidateOperationContext(request)
	if err != nil {
		return auditionCandidateAdoptionReceipt{}, err
	}
	if strings.TrimSpace(request.JudgmentEvidenceID) == "" {
		return auditionCandidateAdoptionReceipt{}, fmt.Errorf("apply_candidate requires judgment_evidence_id")
	}
	evidence, err := latestJudgmentEvidence(round, strings.TrimSpace(request.JudgmentEvidenceID))
	if err != nil {
		return auditionCandidateAdoptionReceipt{}, err
	}
	request.JudgmentEvidenceID = evidence.ID
	if !judgmentAllowsCandidate(evidence, request.CandidateID) {
		return auditionCandidateAdoptionReceipt{}, fmt.Errorf("recorded judgment does not explicitly prefer %s", request.CandidateID)
	}
	if s.auditionCandidateDriver == nil {
		return auditionCandidateAdoptionReceipt{}, fmt.Errorf("candidate project driver unavailable")
	}
	operationContext := s.captureAuditionConversationContext(request.ConversationID)
	before, err := s.auditionCandidateDriver.CurrentPlane(ctx)
	if err != nil {
		return auditionCandidateAdoptionReceipt{}, err
	}
	receipt := newCandidateOperationReceipt("apply", request, before, ref)
	s.emitAuditionEvent(request.ConversationID, "audition.candidate.apply.started", loop.AuditionSessionSnapshot, map[string]any{"candidate_id": request.CandidateID, "judgment_evidence_id": evidence.ID, "receipt": receipt})
	safety, err := s.auditionCandidateDriver.CreateCheckpoint(ctx, auditionCheckpointRequest{ProjectPath: before.ProjectPath, Message: "Safety checkpoint before audition candidate adoption", Source: "audition.apply_candidate", CheckpointKind: "audition_candidate_adoption_safety", GoalID: loop.GoalID, RunID: loop.RunID})
	if err != nil {
		receipt.Status, receipt.Error, receipt.CompletedAt = "failed", err.Error(), time.Now().UTC()
		return receipt, err
	}
	receipt.Checkpoint = map[string]any{"safety": safety}
	checkout, err := s.auditionCandidateDriver.CheckoutCandidate(ctx, ref)
	receipt.Checkout = checkout
	var adoptedPlane auditionProjectPlane
	if err == nil {
		s.activateCurrentProjectWorkspace(ctx)
		s.restoreAuditionConversationContext(request.ConversationID, operationContext)
		adoptedPlane, receipt.ProjectChange, err = s.auditionCandidateDriver.RefreshPlane(ctx, "audition_apply_candidate")
		if err == nil && !planeMatchesCandidate(adoptedPlane, ref) {
			err = fmt.Errorf("applied active project does not match candidate reference")
		}
	}
	if err != nil {
		receipt.Rollback = s.restoreCandidateSafetyPoint(ctx, before, safety, operationContext)
		receipt.Status, receipt.Error, receipt.CompletedAt = "failed", err.Error(), time.Now().UTC()
		s.restoreAuditionConversationContext(request.ConversationID, operationContext)
		s.emitAuditionEvent(request.ConversationID, "audition.candidate.apply.rolled_back", loop.AuditionSessionSnapshot, map[string]any{"candidate_id": request.CandidateID, "receipt": receipt, "message": err.Error()})
		return receipt, err
	}
	adoptionCheckpoint, err := s.auditionCandidateDriver.CreateCheckpoint(ctx, auditionCheckpointRequest{ProjectPath: adoptedPlane.ProjectPath, Message: "Adopt audition " + request.CandidateID, Source: "audition.apply_candidate", CheckpointKind: "audition_candidate_adoption", GoalID: loop.GoalID, RunID: loop.RunID})
	if err != nil {
		receipt.Rollback = s.restoreCandidateSafetyPoint(ctx, before, safety, operationContext)
		receipt.Status, receipt.Error, receipt.CompletedAt = "failed", err.Error(), time.Now().UTC()
		return receipt, err
	}
	receipt.Checkpoint["adoption"] = adoptionCheckpoint
	adoptedPlane, receipt.ProjectChange, err = s.auditionCandidateDriver.RefreshPlane(ctx, "audition_apply_candidate_checkpoint")
	if err != nil {
		receipt.Rollback = s.restoreCandidateSafetyPoint(ctx, before, safety, operationContext)
		receipt.Status, receipt.Error, receipt.CompletedAt = "failed", err.Error(), time.Now().UTC()
		return receipt, err
	}
	loop = operationContext.Loop
	loop.AuditionSessionSnapshot["adopted_candidate_id"] = request.CandidateID
	loop.AuditionSessionSnapshot["adoption_status"] = "applied"
	loop.LatestProjectChange = cloneContext(receipt.ProjectChange)
	loop.ObservationLedger = invalidateFreeStateObservationLedger(loop.ObservationLedger, receipt.ProjectChange)
	checkpointID := firstStringFromMap(adoptionCheckpoint, "commit_id")
	receipt.Status, receipt.CompletedAt = "applied", time.Now().UTC()
	loop.AuditionSessionSnapshot["adoption_receipt"] = receipt
	loop.Experiment.BindCandidateAdoption(request.CandidateID, checkpointID, adoptedPlane.ProjectRevision, map[string]any{"receipt_id": receipt.ID, "judgment_evidence_id": evidence.ID, "candidate_id": request.CandidateID, "status": receipt.Status, "checkpoint_ref": checkpointID})
	bundle, observation, observationErr := s.auditionCandidateDriver.RequestObservation(ctx, loop, round)
	receipt.Observation = bundle
	if observationErr != nil {
		receipt.Status, receipt.Error, receipt.CompletedAt = "verification_pending", observationErr.Error(), time.Now().UTC()
		loop.Status = "observing"
		loop.DecisionPhase = freeStatePhasePostActionEvaluation
		loop.RequiresPostActionObservation = true
		loop.LastError = observationErr.Error()
		loop.AuditionSessionSnapshot["adoption_status"] = "verification_pending"
		loop.AuditionSessionSnapshot["adoption_receipt"] = receipt
		s.storeFreeStateLoop(loop)
		s.persistCurrentProjectWorkspace()
		s.emitAuditionEvent(request.ConversationID, "audition.candidate.verification.pending", loop.AuditionSessionSnapshot, map[string]any{"candidate_id": request.CandidateID, "receipt": receipt, "message": observationErr.Error()})
		return receipt, nil
	}
	if experimentObservation, ok := freeStateExperimentObservation(observation, true); ok {
		if events, recordErr := loop.Experiment.RecordObservation(experimentObservation, true, time.Now().UTC()); recordErr == nil {
			s.emitFreeStateExperimentEvents(events)
		} else {
			receipt.Status, receipt.Error = "verification_pending", recordErr.Error()
		}
	}
	decision := experiment.DecisionRetain
	outcome := experiment.OutcomeImproved
	if request.CandidateID == "candidate-a" {
		decision = experiment.DecisionRollback
		outcome = experiment.OutcomeRolledBack
	}
	decisionEvents, err := loop.Experiment.DecideRound(decision, "explicitly adopted "+request.CandidateID, time.Now().UTC())
	if err != nil {
		receipt.Status, receipt.Error, receipt.CompletedAt = "verification_pending", err.Error(), time.Now().UTC()
		loop.AuditionSessionSnapshot["adoption_status"] = "verification_pending"
		loop.AuditionSessionSnapshot["adoption_receipt"] = receipt
		s.storeFreeStateLoop(loop)
		return receipt, nil
	}
	annotateJudgmentDecision(decisionEvents, "audition.apply_candidate", request.CandidateID)
	s.emitFreeStateExperimentEvents(decisionEvents)
	if request.CandidateID == "candidate-a" {
		rollbackEvents, rollbackErr := loop.Experiment.MarkRollback(time.Now().UTC(), map[string]any{"status": "restored", "adoption_receipt_id": receipt.ID, "checkpoint_ref": checkpointID}, []string{evidence.ID})
		if rollbackErr == nil {
			s.emitFreeStateExperimentEvents(rollbackEvents)
		}
	}
	if ref.ReservationID != "" && s.harness != nil {
		reservation, reservationErr := s.harness.RecordCandidateReservationDisposition(ref.ReservationID, request.CandidateID, ref.ArtifactRef, checkpointID, true)
		if reservationErr != nil {
			receipt.Status, receipt.Error, receipt.CompletedAt = "verification_pending", reservationErr.Error(), time.Now().UTC()
			loop.AuditionSessionSnapshot["adoption_status"] = "verification_pending"
			loop.AuditionSessionSnapshot["adoption_receipt"] = receipt
			s.storeFreeStateLoop(loop)
			s.persistCurrentProjectWorkspace()
			return receipt, nil
		}
		data, _ := json.Marshal(reservation)
		_ = json.Unmarshal(data, &receipt.Reservation)
	}
	loop.Experiment.BindCandidateAdoption(request.CandidateID, checkpointID, adoptedPlane.ProjectRevision, map[string]any{"receipt_id": receipt.ID, "judgment_evidence_id": evidence.ID, "candidate_id": request.CandidateID, "status": receipt.Status, "checkpoint_ref": checkpointID, "apply_strategy": receipt.ApplyStrategy, "reservation": receipt.Reservation})
	settlementEvents, err := loop.Experiment.Settle(outcome, "explicit candidate adoption completed", time.Now().UTC())
	if err != nil {
		return receipt, err
	}
	s.emitFreeStateExperimentEvents(settlementEvents)
	loop.AuditionSessionSnapshot["adoption_status"] = "applied"
	loop.AuditionSessionSnapshot["adoption_receipt"] = receipt
	loop.Status, loop.RequiresPostActionObservation, loop.LastError = "completed", false, ""
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	s.persistCurrentProjectWorkspace()
	if s.harness != nil {
		s.harness.RecordConversationNodeForProjectWithData(ctx, adoptedPlane.ProjectPath, "candidate_adoption", "Adopted "+request.CandidateID, loop.GoalID, loop.RunID, map[string]any{"candidate_adoption_receipt": receipt, "judgment_evidence_id": evidence.ID, "commit_id": checkpointID})
	}
	s.emitAuditionEvent(request.ConversationID, "audition.candidate.applied", loop.AuditionSessionSnapshot, map[string]any{"candidate_id": request.CandidateID, "judgment_evidence_id": evidence.ID, "receipt": receipt})
	return receipt, nil
}
