package collaboration

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const SchemaVersion = "vit.worktree_collaboration.v1"

type ReservationStatus string

const (
	ReservationReserved  ReservationStatus = "reserved"
	ReservationActive    ReservationStatus = "active"
	ReservationReleased  ReservationStatus = "released"
	ReservationAbandoned ReservationStatus = "abandoned"
	ReservationFailed    ReservationStatus = "failed"
	ReservationStale     ReservationStatus = "stale"
)

func (s ReservationStatus) Terminal() bool {
	return s == ReservationReleased || s == ReservationAbandoned || s == ReservationFailed
}

func (s ReservationStatus) WriterActive() bool {
	return s == ReservationReserved || s == ReservationActive
}

type WorktreeDisposition string

const (
	DispositionRetain   WorktreeDisposition = "retain"
	DispositionPromote  WorktreeDisposition = "promote"
	DispositionContinue WorktreeDisposition = "continue"
	DispositionRelease  WorktreeDisposition = "release"
	DispositionAbandon  WorktreeDisposition = "abandon"
	DispositionFailed   WorktreeDisposition = "failed"
)

type WorktreeReservation struct {
	SchemaVersion string `json:"schema_version"`
	ID            string `json:"id"`
	ProjectUUID   string `json:"project_uuid"`
	WorktreeRef   string `json:"worktree_ref"`
	WorktreePath  string `json:"worktree_project_path"`
	BranchRef     string `json:"branch_ref,omitempty"`

	ParentNodeID          string `json:"parent_node_id,omitempty"`
	ParentCommitID        string `json:"parent_commit_id,omitempty"`
	SourceProjectRevision string `json:"source_project_revision,omitempty"`

	OwnerAgentID        string `json:"owner_agent_id"`
	OwnerGoalID         string `json:"owner_goal_id"`
	OwnerRunID          string `json:"owner_run_id"`
	OwnerConversationID string `json:"owner_conversation_id"`

	Purpose     string `json:"purpose,omitempty"`
	Hypothesis  string `json:"hypothesis,omitempty"`
	DisplayName string `json:"display_name,omitempty"`

	Status      ReservationStatus   `json:"status"`
	Disposition WorktreeDisposition `json:"disposition,omitempty"`

	CreatedAt  time.Time `json:"created_at"`
	RenewedAt  time.Time `json:"renewed_at,omitempty"`
	ReleasedAt time.Time `json:"released_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`

	CandidateRefs []string `json:"candidate_refs,omitempty"`
	ArtifactRefs  []string `json:"artifact_refs,omitempty"`
	SettlementRef string   `json:"settlement_ref,omitempty"`
}

type ChildTaskStatus string

const (
	ChildTaskPending   ChildTaskStatus = "pending"
	ChildTaskRunning   ChildTaskStatus = "running"
	ChildTaskCompleted ChildTaskStatus = "completed"
	ChildTaskStopped   ChildTaskStatus = "stopped"
	ChildTaskFailed    ChildTaskStatus = "failed"
	ChildTaskAbandoned ChildTaskStatus = "abandoned"
)

type ChildTask struct {
	SchemaVersion  string `json:"schema_version"`
	ID             string `json:"id"`
	ParentAgentID  string `json:"parent_agent_id"`
	AgentID        string `json:"agent_id"`
	GoalID         string `json:"goal_id"`
	RunID          string `json:"run_id"`
	ConversationID string `json:"conversation_id"`
	WorktreeRef    string `json:"worktree_ref"`
	ReservationID  string `json:"reservation_id"`

	Hypothesis          string          `json:"hypothesis,omitempty"`
	AllowedScope        []string        `json:"allowed_scope,omitempty"`
	AllowedCapabilities []string        `json:"allowed_capabilities,omitempty"`
	Status              ChildTaskStatus `json:"status"`
	ResultCandidateRef  string          `json:"result_candidate_ref,omitempty"`
	SettlementRef       string          `json:"settlement_ref,omitempty"`
	Error               string          `json:"error,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type Snapshot struct {
	SchemaVersion string                `json:"schema_version"`
	Reservations  []WorktreeReservation `json:"reservations,omitempty"`
	ChildTasks    []ChildTask           `json:"child_tasks,omitempty"`
}

type Registry struct {
	mu           sync.RWMutex
	reservations map[string]WorktreeReservation
	byWorktree   map[string]string
	childTasks   map[string]ChildTask
}

func NewRegistry() *Registry {
	return &Registry{
		reservations: map[string]WorktreeReservation{},
		byWorktree:   map[string]string{},
		childTasks:   map[string]ChildTask{},
	}
}

func (r *Registry) Reserve(request WorktreeReservation) (WorktreeReservation, error) {
	if r == nil {
		return WorktreeReservation{}, fmt.Errorf("collaboration registry is nil")
	}
	normalized, err := normalizeReservation(request)
	if err != nil {
		return WorktreeReservation{}, err
	}
	key := worktreeKey(normalized.ProjectUUID, normalized.WorktreeRef)
	r.mu.Lock()
	defer r.mu.Unlock()
	if existingID := r.byWorktree[key]; existingID != "" {
		existing, exists := r.reservations[existingID]
		if exists && existing.Status.WriterActive() {
			if sameOwner(existing, normalized) {
				return cloneReservation(existing), nil
			}
			return WorktreeReservation{}, fmt.Errorf("worktree %s is already reserved by agent %s run %s", normalized.WorktreeRef, existing.OwnerAgentID, existing.OwnerRunID)
		}
		delete(r.byWorktree, key)
	}
	if normalized.ID == "" {
		normalized.ID = newID("reservation")
	}
	if _, exists := r.reservations[normalized.ID]; exists {
		return WorktreeReservation{}, fmt.Errorf("reservation already exists: %s", normalized.ID)
	}
	r.reservations[normalized.ID] = cloneReservation(normalized)
	r.byWorktree[key] = normalized.ID
	return cloneReservation(normalized), nil
}

func (r *Registry) Get(id string) (WorktreeReservation, bool) {
	if r == nil {
		return WorktreeReservation{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	reservation, ok := r.reservations[strings.TrimSpace(id)]
	return cloneReservation(reservation), ok
}

func (r *Registry) ActiveReservation(projectUUID, worktreeRef string) (WorktreeReservation, bool) {
	if r == nil {
		return WorktreeReservation{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := r.byWorktree[worktreeKey(projectUUID, worktreeRef)]
	reservation, ok := r.reservations[id]
	return cloneReservation(reservation), ok && reservation.Status.WriterActive()
}

func (r *Registry) List(projectUUID string) []WorktreeReservation {
	if r == nil {
		return nil
	}
	projectUUID = strings.TrimSpace(projectUUID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]WorktreeReservation, 0, len(r.reservations))
	for _, reservation := range r.reservations {
		if projectUUID == "" || reservation.ProjectUUID == projectUUID {
			out = append(out, cloneReservation(reservation))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (r *Registry) Takeover(id, newAgentID, newGoalID, newRunID, newConversationID string, now time.Time) (WorktreeReservation, error) {
	if r == nil {
		return WorktreeReservation{}, fmt.Errorf("collaboration registry is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	reservation, ok := r.reservations[strings.TrimSpace(id)]
	if !ok {
		return WorktreeReservation{}, fmt.Errorf("reservation not found: %s", id)
	}
	for name, value := range map[string]string{"agent_id": newAgentID, "goal_id": newGoalID, "run_id": newRunID, "conversation_id": newConversationID} {
		if strings.TrimSpace(value) == "" {
			return WorktreeReservation{}, fmt.Errorf("takeover %s is required", name)
		}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	reservation.OwnerAgentID, reservation.OwnerGoalID, reservation.OwnerRunID, reservation.OwnerConversationID = strings.TrimSpace(newAgentID), strings.TrimSpace(newGoalID), strings.TrimSpace(newRunID), strings.TrimSpace(newConversationID)
	reservation.Status = ReservationActive
	reservation.RenewedAt = now.UTC()
	r.reservations[reservation.ID] = cloneReservation(reservation)
	r.byWorktree[worktreeKey(reservation.ProjectUUID, reservation.WorktreeRef)] = reservation.ID
	return cloneReservation(reservation), nil
}

func (r *Registry) TransferOwner(id, currentAgentID, currentGoalID, currentRunID, newAgentID, newGoalID, newRunID, newConversationID string, now time.Time) (WorktreeReservation, error) {
	if r == nil {
		return WorktreeReservation{}, fmt.Errorf("collaboration registry is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	reservation, ok := r.reservations[strings.TrimSpace(id)]
	if !ok {
		return WorktreeReservation{}, fmt.Errorf("reservation not found: %s", id)
	}
	if !reservation.Status.WriterActive() || reservation.OwnerAgentID != strings.TrimSpace(currentAgentID) || reservation.OwnerGoalID != strings.TrimSpace(currentGoalID) || reservation.OwnerRunID != strings.TrimSpace(currentRunID) {
		return WorktreeReservation{}, fmt.Errorf("reservation owner mismatch: %s", id)
	}
	for name, value := range map[string]string{"agent_id": newAgentID, "goal_id": newGoalID, "run_id": newRunID, "conversation_id": newConversationID} {
		if strings.TrimSpace(value) == "" {
			return WorktreeReservation{}, fmt.Errorf("new reservation %s is required", name)
		}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	reservation.OwnerAgentID, reservation.OwnerGoalID, reservation.OwnerRunID, reservation.OwnerConversationID = strings.TrimSpace(newAgentID), strings.TrimSpace(newGoalID), strings.TrimSpace(newRunID), strings.TrimSpace(newConversationID)
	reservation.Status = ReservationActive
	reservation.RenewedAt = now.UTC()
	r.reservations[reservation.ID] = cloneReservation(reservation)
	return cloneReservation(reservation), nil
}

func (r *Registry) Renew(id, ownerAgentID, ownerGoalID, ownerRunID string, expiresAt, now time.Time) (WorktreeReservation, error) {
	return r.mutateOwner(id, ownerAgentID, ownerGoalID, ownerRunID, func(reservation *WorktreeReservation) error {
		if !reservation.Status.WriterActive() {
			return fmt.Errorf("reservation %s is not renewable in status %s", reservation.ID, reservation.Status)
		}
		if now.IsZero() {
			now = time.Now().UTC()
		}
		reservation.Status = ReservationActive
		reservation.RenewedAt = now.UTC()
		reservation.ExpiresAt = expiresAt.UTC()
		return nil
	})
}

func (r *Registry) Release(id, ownerAgentID, ownerGoalID, ownerRunID string, now time.Time) (WorktreeReservation, error) {
	return r.finish(id, ownerAgentID, ownerGoalID, ownerRunID, ReservationReleased, now, "")
}

func (r *Registry) Abandon(id, ownerAgentID, ownerGoalID, ownerRunID string, now time.Time) (WorktreeReservation, error) {
	return r.finish(id, ownerAgentID, ownerGoalID, ownerRunID, ReservationAbandoned, now, "")
}

func (r *Registry) Fail(id, ownerAgentID, ownerGoalID, ownerRunID, message string, now time.Time) (WorktreeReservation, error) {
	return r.finish(id, ownerAgentID, ownerGoalID, ownerRunID, ReservationFailed, now, message)
}

func (r *Registry) RecoverStale(now time.Time) []WorktreeReservation {
	if r == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []WorktreeReservation{}
	for id, reservation := range r.reservations {
		if !reservation.Status.WriterActive() || reservation.ExpiresAt.IsZero() || reservation.ExpiresAt.After(now) {
			continue
		}
		reservation.Status = ReservationStale
		reservation.ReleasedAt = now.UTC()
		r.reservations[id] = cloneReservation(reservation)
		delete(r.byWorktree, worktreeKey(reservation.ProjectUUID, reservation.WorktreeRef))
		out = append(out, cloneReservation(reservation))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) RecordCandidate(id, candidateRef, artifactRef, settlementRef string) (WorktreeReservation, error) {
	if r == nil {
		return WorktreeReservation{}, fmt.Errorf("collaboration registry is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	reservation, ok := r.reservations[strings.TrimSpace(id)]
	if !ok {
		return WorktreeReservation{}, fmt.Errorf("reservation not found: %s", id)
	}
	reservation.CandidateRefs = appendUnique(reservation.CandidateRefs, candidateRef)
	reservation.ArtifactRefs = appendUnique(reservation.ArtifactRefs, artifactRef)
	if strings.TrimSpace(settlementRef) != "" {
		reservation.SettlementRef = strings.TrimSpace(settlementRef)
	}
	r.reservations[reservation.ID] = cloneReservation(reservation)
	return cloneReservation(reservation), nil
}

func (r *Registry) RecordCandidateAndRelease(id, candidateRef, artifactRef, settlementRef string, now time.Time) (WorktreeReservation, error) {
	if r == nil {
		return WorktreeReservation{}, fmt.Errorf("collaboration registry is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	reservation, ok := r.reservations[strings.TrimSpace(id)]
	if !ok {
		return WorktreeReservation{}, fmt.Errorf("reservation not found: %s", id)
	}
	reservation.CandidateRefs = appendUnique(reservation.CandidateRefs, candidateRef)
	reservation.ArtifactRefs = appendUnique(reservation.ArtifactRefs, artifactRef)
	if strings.TrimSpace(settlementRef) != "" {
		reservation.SettlementRef = strings.TrimSpace(settlementRef)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	reservation.Status = ReservationReleased
	if reservation.Disposition == "" {
		reservation.Disposition = DispositionRetain
	}
	reservation.ReleasedAt = now.UTC()
	r.reservations[reservation.ID] = cloneReservation(reservation)
	delete(r.byWorktree, worktreeKey(reservation.ProjectUUID, reservation.WorktreeRef))
	return cloneReservation(reservation), nil
}

func (r *Registry) SetDisposition(id string, disposition WorktreeDisposition, now time.Time) (WorktreeReservation, error) {
	if r == nil {
		return WorktreeReservation{}, fmt.Errorf("collaboration registry is nil")
	}
	switch disposition {
	case DispositionRetain, DispositionPromote, DispositionContinue, DispositionRelease, DispositionAbandon, DispositionFailed:
	default:
		return WorktreeReservation{}, fmt.Errorf("unsupported worktree disposition %q", disposition)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	reservation, ok := r.reservations[strings.TrimSpace(id)]
	if !ok {
		return WorktreeReservation{}, fmt.Errorf("reservation not found: %s", id)
	}
	reservation.Disposition = disposition
	switch disposition {
	case DispositionContinue:
		reservation.Status = ReservationActive
		reservation.RenewedAt = now.UTC()
		r.byWorktree[worktreeKey(reservation.ProjectUUID, reservation.WorktreeRef)] = reservation.ID
	case DispositionRetain, DispositionPromote, DispositionRelease:
		reservation.Status = ReservationReleased
		reservation.ReleasedAt = now.UTC()
		delete(r.byWorktree, worktreeKey(reservation.ProjectUUID, reservation.WorktreeRef))
	case DispositionAbandon:
		reservation.Status = ReservationAbandoned
		reservation.ReleasedAt = now.UTC()
		delete(r.byWorktree, worktreeKey(reservation.ProjectUUID, reservation.WorktreeRef))
	case DispositionFailed:
		reservation.Status = ReservationFailed
		reservation.ReleasedAt = now.UTC()
		delete(r.byWorktree, worktreeKey(reservation.ProjectUUID, reservation.WorktreeRef))
	}
	r.reservations[reservation.ID] = cloneReservation(reservation)
	return cloneReservation(reservation), nil
}

func (r *Registry) RecoverInactiveOwners(activeGoalIDs map[string]bool, now time.Time) []WorktreeReservation {
	if r == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []WorktreeReservation{}
	for id, reservation := range r.reservations {
		if !reservation.Status.WriterActive() || activeGoalIDs[reservation.OwnerGoalID] {
			continue
		}
		reservation.Status = ReservationStale
		reservation.ReleasedAt = now.UTC()
		r.reservations[id] = cloneReservation(reservation)
		delete(r.byWorktree, worktreeKey(reservation.ProjectUUID, reservation.WorktreeRef))
		out = append(out, cloneReservation(reservation))
		for taskID, task := range r.childTasks {
			if task.ReservationID == reservation.ID && (task.Status == ChildTaskPending || task.Status == ChildTaskRunning) {
				task.Status = ChildTaskStopped
				task.UpdatedAt = now.UTC()
				r.childTasks[taskID] = cloneChildTask(task)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) AuthorizeWrite(projectUUID, worktreeRef, agentID, goalID, runID string) error {
	if r == nil {
		return fmt.Errorf("collaboration registry is nil")
	}
	projectUUID, worktreeRef = strings.TrimSpace(projectUUID), strings.TrimSpace(worktreeRef)
	if projectUUID == "" || worktreeRef == "" {
		return fmt.Errorf("project_uuid and worktree_ref are required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := r.byWorktree[worktreeKey(projectUUID, worktreeRef)]
	if id == "" {
		return fmt.Errorf("worktree %s has no active reservation", worktreeRef)
	}
	reservation, ok := r.reservations[id]
	if !ok || !reservation.Status.WriterActive() {
		return fmt.Errorf("worktree %s is not writable", worktreeRef)
	}
	if reservation.OwnerAgentID != strings.TrimSpace(agentID) || reservation.OwnerGoalID != strings.TrimSpace(goalID) || reservation.OwnerRunID != strings.TrimSpace(runID) {
		return fmt.Errorf("write owner mismatch for worktree %s", worktreeRef)
	}
	return nil
}

func (r *Registry) RegisterChildTask(task ChildTask) (ChildTask, error) {
	if r == nil {
		return ChildTask{}, fmt.Errorf("collaboration registry is nil")
	}
	if task.SchemaVersion == "" {
		task.SchemaVersion = SchemaVersion
	}
	if task.ID == "" {
		task.ID = newID("child")
	}
	if task.Status == "" {
		task.Status = ChildTaskPending
	}
	if err := validateChildTask(task); err != nil {
		return ChildTask{}, err
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now().UTC()
	}
	task.UpdatedAt = task.CreatedAt
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.childTasks[task.ID]; exists {
		return ChildTask{}, fmt.Errorf("child task already exists: %s", task.ID)
	}
	r.childTasks[task.ID] = cloneChildTask(task)
	return cloneChildTask(task), nil
}

func (r *Registry) GetChildTask(id string) (ChildTask, bool) {
	if r == nil {
		return ChildTask{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.childTasks[strings.TrimSpace(id)]
	return cloneChildTask(task), ok
}

func (r *Registry) UpdateChildTask(task ChildTask) (ChildTask, error) {
	if r == nil {
		return ChildTask{}, fmt.Errorf("collaboration registry is nil")
	}
	if strings.TrimSpace(task.ID) == "" {
		return ChildTask{}, fmt.Errorf("child task id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, ok := r.childTasks[task.ID]
	if !ok {
		return ChildTask{}, fmt.Errorf("child task not found: %s", task.ID)
	}
	if task.ParentAgentID != previous.ParentAgentID || task.AgentID != previous.AgentID || task.WorktreeRef != previous.WorktreeRef || task.ReservationID != previous.ReservationID {
		return ChildTask{}, fmt.Errorf("child task identity is immutable: %s", task.ID)
	}
	if task.SchemaVersion == "" {
		task.SchemaVersion = previous.SchemaVersion
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = previous.CreatedAt
	}
	task.UpdatedAt = time.Now().UTC()
	r.childTasks[task.ID] = cloneChildTask(task)
	return cloneChildTask(task), nil
}

func (r *Registry) ChildTaskForOwner(agentID, goalID, runID string) (ChildTask, bool) {
	if r == nil {
		return ChildTask{}, false
	}
	agentID, goalID, runID = strings.TrimSpace(agentID), strings.TrimSpace(goalID), strings.TrimSpace(runID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, task := range r.childTasks {
		if task.AgentID == agentID && task.GoalID == goalID && task.RunID == runID && (task.Status == ChildTaskPending || task.Status == ChildTaskRunning) {
			return cloneChildTask(task), true
		}
	}
	return ChildTask{}, false
}

func (r *Registry) ListChildTasks(parentAgentID string) []ChildTask {
	if r == nil {
		return nil
	}
	parentAgentID = strings.TrimSpace(parentAgentID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []ChildTask{}
	for _, task := range r.childTasks {
		if parentAgentID == "" || task.ParentAgentID == parentAgentID {
			out = append(out, cloneChildTask(task))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) Snapshot() Snapshot {
	return Snapshot{SchemaVersion: SchemaVersion, Reservations: r.List(""), ChildTasks: r.ListChildTasks("")}
}

func (r *Registry) Restore(snapshot Snapshot) error {
	if r == nil {
		return fmt.Errorf("collaboration registry is nil")
	}
	if snapshot.SchemaVersion != "" && snapshot.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported collaboration schema_version %q", snapshot.SchemaVersion)
	}
	next := NewRegistry()
	for _, reservation := range snapshot.Reservations {
		if err := validateReservation(reservation); err != nil {
			return err
		}
		key := worktreeKey(reservation.ProjectUUID, reservation.WorktreeRef)
		if existingID := next.byWorktree[key]; existingID != "" {
			existing := next.reservations[existingID]
			if existing.Status.WriterActive() && reservation.Status.WriterActive() {
				return fmt.Errorf("duplicate active reservation for worktree %s", reservation.WorktreeRef)
			}
		}
		next.reservations[reservation.ID] = cloneReservation(reservation)
		if reservation.Status.WriterActive() {
			next.byWorktree[key] = reservation.ID
		}
	}
	for _, task := range snapshot.ChildTasks {
		if err := validateChildTask(task); err != nil {
			return err
		}
		next.childTasks[task.ID] = cloneChildTask(task)
	}
	r.mu.Lock()
	r.reservations, r.byWorktree, r.childTasks = next.reservations, next.byWorktree, next.childTasks
	r.mu.Unlock()
	return nil
}

func (r *Registry) mutateOwner(id, ownerAgentID, ownerGoalID, ownerRunID string, mutate func(*WorktreeReservation) error) (WorktreeReservation, error) {
	if r == nil {
		return WorktreeReservation{}, fmt.Errorf("collaboration registry is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id = strings.TrimSpace(id)
	reservation, ok := r.reservations[id]
	if !ok {
		return WorktreeReservation{}, fmt.Errorf("reservation not found: %s", id)
	}
	if reservation.OwnerAgentID != strings.TrimSpace(ownerAgentID) || reservation.OwnerGoalID != strings.TrimSpace(ownerGoalID) || reservation.OwnerRunID != strings.TrimSpace(ownerRunID) {
		return WorktreeReservation{}, fmt.Errorf("reservation owner mismatch: %s", id)
	}
	if err := mutate(&reservation); err != nil {
		return WorktreeReservation{}, err
	}
	r.reservations[id] = cloneReservation(reservation)
	if !reservation.Status.WriterActive() {
		delete(r.byWorktree, worktreeKey(reservation.ProjectUUID, reservation.WorktreeRef))
	}
	return cloneReservation(reservation), nil
}

func (r *Registry) finish(id, ownerAgentID, ownerGoalID, ownerRunID string, status ReservationStatus, now time.Time, message string) (WorktreeReservation, error) {
	return r.mutateOwner(id, ownerAgentID, ownerGoalID, ownerRunID, func(reservation *WorktreeReservation) error {
		if reservation.Status.Terminal() {
			return fmt.Errorf("reservation %s is already terminal", reservation.ID)
		}
		if now.IsZero() {
			now = time.Now().UTC()
		}
		reservation.Status = status
		switch status {
		case ReservationReleased:
			reservation.Disposition = DispositionRelease
		case ReservationAbandoned:
			reservation.Disposition = DispositionAbandon
		case ReservationFailed:
			reservation.Disposition = DispositionFailed
		}
		reservation.ReleasedAt = now.UTC()
		if message != "" {
			reservation.SettlementRef = message
		}
		return nil
	})
}

func normalizeReservation(in WorktreeReservation) (WorktreeReservation, error) {
	if in.SchemaVersion == "" {
		in.SchemaVersion = SchemaVersion
	}
	if in.ID == "" {
		in.ID = newID("reservation")
	}
	if in.Status == "" {
		in.Status = ReservationReserved
	}
	if err := validateReservation(in); err != nil {
		return WorktreeReservation{}, err
	}
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now().UTC()
	}
	return cloneReservation(in), nil
}

func validateReservation(in WorktreeReservation) error {
	if in.SchemaVersion != SchemaVersion {
		return fmt.Errorf("reservation schema_version must be %s", SchemaVersion)
	}
	for name, value := range map[string]string{
		"id": in.ID, "project_uuid": in.ProjectUUID, "worktree_ref": in.WorktreeRef,
		"owner_agent_id": in.OwnerAgentID, "owner_goal_id": in.OwnerGoalID,
		"owner_run_id": in.OwnerRunID, "owner_conversation_id": in.OwnerConversationID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("reservation %s is required", name)
		}
	}
	if in.Status != ReservationReserved && in.Status != ReservationActive && in.Status != ReservationReleased && in.Status != ReservationAbandoned && in.Status != ReservationFailed && in.Status != ReservationStale {
		return fmt.Errorf("unsupported reservation status %q", in.Status)
	}
	return nil
}

func validateChildTask(task ChildTask) error {
	if task.SchemaVersion == "" {
		task.SchemaVersion = SchemaVersion
	}
	if task.SchemaVersion != SchemaVersion {
		return fmt.Errorf("child task schema_version must be %s", SchemaVersion)
	}
	for name, value := range map[string]string{"id": task.ID, "parent_agent_id": task.ParentAgentID, "agent_id": task.AgentID, "goal_id": task.GoalID, "run_id": task.RunID, "conversation_id": task.ConversationID, "worktree_ref": task.WorktreeRef, "reservation_id": task.ReservationID} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("child task %s is required", name)
		}
	}
	switch task.Status {
	case ChildTaskPending, ChildTaskRunning, ChildTaskCompleted, ChildTaskStopped, ChildTaskFailed, ChildTaskAbandoned:
	default:
		return fmt.Errorf("unsupported child task status %q", task.Status)
	}
	return nil
}

func sameOwner(a, b WorktreeReservation) bool {
	return a.OwnerAgentID == b.OwnerAgentID && a.OwnerGoalID == b.OwnerGoalID && a.OwnerRunID == b.OwnerRunID && a.OwnerConversationID == b.OwnerConversationID
}

func worktreeKey(projectUUID, worktreeRef string) string {
	return strings.ToLower(strings.TrimSpace(projectUUID)) + "\x00" + strings.ToLower(strings.TrimSpace(worktreeRef))
}

func cloneReservation(in WorktreeReservation) WorktreeReservation {
	in.CandidateRefs = append([]string(nil), in.CandidateRefs...)
	in.ArtifactRefs = append([]string(nil), in.ArtifactRefs...)
	return in
}

func cloneChildTask(in ChildTask) ChildTask {
	in.AllowedScope = append([]string(nil), in.AllowedScope...)
	in.AllowedCapabilities = append([]string(nil), in.AllowedCapabilities...)
	return in
}

func newID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return prefix + "_" + time.Now().UTC().Format("20060102T150405.000000000")
	}
	return prefix + "_" + hex.EncodeToString(raw[:])
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
