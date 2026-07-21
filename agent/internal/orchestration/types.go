// Package orchestration contains the durable control-plane contracts for the
// project-aware capability runtime. It deliberately does not execute DAW
// commands; Execution Coordinator and the project foundation own mutation.
package orchestration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const SchemaVersion = "vit.orchestration.v1"

type EngineOwner string

const (
	EngineLegacy EngineOwner = "legacy"
	EngineV1     EngineOwner = "capability_runtime_v1"
)

type InteractionMode string

const (
	InteractionInspect       InteractionMode = "inspect"
	InteractionPropose       InteractionMode = "propose"
	InteractionExecuteIntent InteractionMode = "execute_intent"
)

type ProcessingPath string

const (
	PathObservationQuery ProcessingPath = "observation_query"
	PathDirectEdit       ProcessingPath = "direct_edit"
	PathCapability       ProcessingPath = "capability"
)

type SessionStatus string

const (
	StatusAnalyzing   SessionStatus = "analyzing"
	StatusWaiting     SessionStatus = "waiting_user"
	StatusReady       SessionStatus = "ready"
	StatusAuthorized  SessionStatus = "authorized"
	StatusExecuting   SessionStatus = "executing"
	StatusVerifying   SessionStatus = "verifying"
	StatusNeedsReview SessionStatus = "needs_review"
	StatusCompleted   SessionStatus = "completed"
	StatusFailed      SessionStatus = "failed"
	StatusStale       SessionStatus = "stale"
	StatusCancelled   SessionStatus = "cancelled"
)

var terminalStatuses = map[SessionStatus]bool{
	StatusNeedsReview: true,
	StatusCompleted:   true,
	StatusFailed:      true,
	StatusStale:       true,
	StatusCancelled:   true,
}

type CapabilityOutcomeKind string

const (
	OutcomeNeedObservation CapabilityOutcomeKind = "need_observation"
	OutcomeNeedUserInput   CapabilityOutcomeKind = "need_user_input"
	OutcomeAnalysis        CapabilityOutcomeKind = "analysis"
	OutcomeProposal        CapabilityOutcomeKind = "proposal"
	OutcomeBlocked         CapabilityOutcomeKind = "blocked"
)

// CapabilityDefinition is the typed source for registry discovery. A
// capability-specific adapter may keep its internal solver pipeline private.
type CapabilityDefinition struct {
	ID              string   `json:"id"`
	Version         string   `json:"version"`
	Family          string   `json:"family"`
	Description     string   `json:"description,omitempty"`
	Effects         []string `json:"effects,omitempty"`
	RiskCeiling     string   `json:"risk_ceiling,omitempty"`
	ManifestHash    string   `json:"manifest_hash,omitempty"`
	VerificationRef string   `json:"verification_ref,omitempty"`
}

type CapabilityInvocation struct {
	ID              string          `json:"id"`
	SessionID       string          `json:"session_id"`
	ConversationID  string          `json:"conversation_id,omitempty"`
	CapabilityID    string          `json:"capability_id"`
	CapabilityVer   string          `json:"capability_version"`
	InteractionMode InteractionMode `json:"interaction_mode"`
	ProcessingPath  ProcessingPath  `json:"processing_path"`
	Goal            string          `json:"goal"`
	Constraints     []string        `json:"constraints,omitempty"`
	TargetRefs      []string        `json:"target_refs,omitempty"`
}

type ContextRequest struct {
	CapabilityID       string   `json:"capability_id"`
	ManifestID         string   `json:"manifest_id"`
	RequiredFields     []string `json:"required_fields,omitempty"`
	FreshnessClass     string   `json:"freshness_class,omitempty"`
	Consistency        string   `json:"consistency,omitempty"`
	MaxDisclosureBytes int      `json:"max_disclosure_bytes,omitempty"`
}

type OmissionStatus string

const (
	OmissionNotRequested OmissionStatus = "not_requested"
	OmissionBudget       OmissionStatus = "omitted_budget"
	OmissionByReference  OmissionStatus = "available_by_ref"
	OmissionUnavailable  OmissionStatus = "unavailable"
	OmissionStale        OmissionStatus = "stale"
	OmissionForbidden    OmissionStatus = "forbidden"
)

type ProjectCut struct {
	ProjectUUID            string   `json:"project_uuid"`
	ProjectEpoch           string   `json:"project_epoch"`
	BaseProjectRevision    string   `json:"base_project_revision,omitempty"`
	ReadBarrierToken       string   `json:"read_barrier_token,omitempty"`
	Consistency            string   `json:"consistency"`
	DependencyFingerprints []string `json:"dependency_fingerprints,omitempty"`
	ArtifactRefs           []string `json:"artifact_refs,omitempty"`
	DerivedFrom            []string `json:"derived_from,omitempty"`
	TargetFingerprints     []string `json:"target_fingerprints,omitempty"`
	ContractVersions       []string `json:"contract_versions,omitempty"`
	Hash                   string   `json:"hash,omitempty"`
}

func (c ProjectCut) IsExecutable() bool {
	return strings.TrimSpace(c.ProjectUUID) != "" &&
		strings.TrimSpace(c.ProjectEpoch) != "" &&
		strings.TrimSpace(c.Consistency) == "strong" &&
		strings.TrimSpace(c.Hash) != ""
}

// ComputeHash creates a stable hash from the semantic Cut contents. Hash is
// excluded so callers can safely pass an already populated Cut.
func (c ProjectCut) ComputeHash() string {
	c.Hash = ""
	for _, values := range [][]string{
		c.DependencyFingerprints, c.ArtifactRefs, c.DerivedFrom,
		c.TargetFingerprints, c.ContractVersions,
	} {
		sort.Strings(values)
	}
	b, _ := json.Marshal(c)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type ContextBundle struct {
	ID              string                    `json:"id"`
	CapabilityID    string                    `json:"capability_id"`
	ProjectCutHash  string                    `json:"project_cut_hash"`
	ArtifactRefs    []string                  `json:"artifact_refs,omitempty"`
	EvidenceRefs    []string                  `json:"evidence_refs,omitempty"`
	Disclosure      string                    `json:"disclosure,omitempty"`
	OmissionReasons []string                  `json:"omission_reasons,omitempty"`
	Omissions       map[string]OmissionStatus `json:"omissions,omitempty"`
}

type CapabilityOutcome struct {
	Kind         CapabilityOutcomeKind `json:"kind"`
	CapabilityID string                `json:"capability_id"`
	Summary      string                `json:"summary,omitempty"`
	Context      *ContextBundle        `json:"context,omitempty"`
	ProposalID   string                `json:"proposal_id,omitempty"`
	EvidenceRefs []string              `json:"evidence_refs,omitempty"`
	Blockers     []string              `json:"blockers,omitempty"`
}

type Proposal struct {
	ID              string                `json:"id"`
	Revision        int64                 `json:"revision"`
	CapabilityID    string                `json:"capability_id"`
	CapabilityVer   string                `json:"capability_version"`
	ProjectCutHash  string                `json:"project_cut_hash"`
	CandidateID     string                `json:"candidate_id,omitempty"`
	ActionSetHash   string                `json:"action_set_hash,omitempty"`
	TargetScope     []string              `json:"target_scope,omitempty"`
	Risk            string                `json:"risk,omitempty"`
	VerificationRef string                `json:"verification_ref,omitempty"`
	Summary         string                `json:"summary,omitempty"`
	Presentation    *ProposalPresentation `json:"presentation,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
}

type Action struct {
	ID                string         `json:"id"`
	Command           string         `json:"command"`
	TargetRef         string         `json:"target_ref"`
	BeforeFingerprint string         `json:"before_fingerprint,omitempty"`
	Args              map[string]any `json:"args,omitempty"`
	Compensatable     bool           `json:"compensatable"`
	IdempotencyClass  string         `json:"idempotency_class,omitempty"`
}

type ActionSet struct {
	ID             string   `json:"id"`
	CapabilityID   string   `json:"capability_id"`
	ProjectCutHash string   `json:"project_cut_hash"`
	Actions        []Action `json:"actions"`
	Hash           string   `json:"hash"`
}

func (a ActionSet) ComputeHash() string {
	a.Hash = ""
	// Action.Args is deliberately open-ended JSON data.  A caller may put a
	// typed value in it while the durable FrozenPlan restores that same value
	// as map[string]any.  Hash the persisted JSON representation rather than
	// the transient Go shape, otherwise an approved action can become invalid
	// solely by crossing the persistence boundary.
	b, err := json.Marshal(a)
	if err != nil {
		return ""
	}
	var persisted ActionSet
	if err := json.Unmarshal(b, &persisted); err != nil {
		return ""
	}
	persisted.Hash = ""
	b, err = json.Marshal(persisted)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type Authorization struct {
	ProposalID       string            `json:"proposal_id"`
	ProposalRevision int64             `json:"proposal_revision"`
	ActionSetHash    string            `json:"action_set_hash"`
	ProjectCutHash   string            `json:"project_cut_hash"`
	Scope            []string          `json:"scope,omitempty"`
	SourceTurnID     string            `json:"source_turn_id"`
	Sequence         uint64            `json:"sequence"`
	Decision         *ApprovalDecision `json:"decision,omitempty"`
	GrantedAt        time.Time         `json:"granted_at"`
	Consumed         bool              `json:"consumed"`
}

type ExecutionRecord struct {
	ID              string          `json:"id"`
	SessionID       string          `json:"session_id"`
	ActionSetHash   string          `json:"action_set_hash"`
	ProjectCut      ProjectCut      `json:"project_cut"`
	IdempotencyKey  string          `json:"idempotency_key"`
	Status          string          `json:"status"`
	Receipts        []ActionReceipt `json:"receipts,omitempty"`
	ReceiptRef      string          `json:"receipt_ref,omitempty"`
	PersistenceRefs []string        `json:"persistence_refs,omitempty"`
	Verification    string          `json:"verification,omitempty"`
	// VerificationResult preserves structural, acoustic and user-acceptance
	// outcomes plus their evidence. Verification remains as a compact status
	// for backward-compatible persisted readers.
	VerificationResult *VerificationResult `json:"verification_result,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

type ActionReceipt struct {
	ActionID        string `json:"action_id"`
	Status          string `json:"status"`
	AppliedRevision string `json:"applied_revision,omitempty"`
	EffectivelyOnce bool   `json:"effectively_once"`
	Error           string `json:"error,omitempty"`
	// EvidenceRefs and Details preserve adapter-level facts that are required
	// for an auditable Receipt (for example, a SPAL parameter readback and its
	// compensating rollback outcome). Existing ports may leave them empty.
	EvidenceRefs []string       `json:"evidence_refs,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

type VerificationResult struct {
	Status         string   `json:"status"`
	Structural     string   `json:"structural,omitempty"`
	Acoustic       string   `json:"acoustic,omitempty"`
	UserAcceptance string   `json:"user_acceptance,omitempty"`
	EvidenceRefs   []string `json:"evidence_refs,omitempty"`
	Summary        string   `json:"summary,omitempty"`
}

// FrozenPlan is the immutable execution input authorized by the user. Hashes
// alone are insufficient after restart: the exact ActionSet, ProjectCut and
// pre-execution observation identity must remain recoverable.
type FrozenPlan struct {
	Proposal              Proposal   `json:"proposal"`
	ActionSet             ActionSet  `json:"action_set"`
	ProjectCut            ProjectCut `json:"project_cut"`
	ContextBundleID       string     `json:"context_bundle_id,omitempty"`
	PreviousObservationID string     `json:"previous_observation_id,omitempty"`
	FrozenAt              time.Time  `json:"frozen_at"`
}

type PlanningSession struct {
	SchemaVersion  string               `json:"schema_version"`
	ID             string               `json:"id"`
	ProjectUUID    string               `json:"project_uuid"`
	EngineOwner    EngineOwner          `json:"engine_owner"`
	Revision       uint64               `json:"revision"`
	Status         SessionStatus        `json:"status"`
	Goal           string               `json:"goal"`
	Constraints    []string             `json:"constraints,omitempty"`
	Invocation     CapabilityInvocation `json:"invocation"`
	ActiveProposal *Proposal            `json:"active_proposal,omitempty"`
	FrozenPlan     *FrozenPlan          `json:"frozen_plan,omitempty"`
	Authorization  *Authorization       `json:"authorization,omitempty"`
	Execution      *ExecutionRecord     `json:"execution,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

func (s PlanningSession) Terminal() bool { return terminalStatuses[s.Status] }

func NewSession(id, projectUUID, goal string, owner EngineOwner, invocation CapabilityInvocation) (PlanningSession, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(projectUUID) == "" || strings.TrimSpace(goal) == "" {
		return PlanningSession{}, errors.New("session id, project uuid and goal are required")
	}
	if owner != EngineLegacy && owner != EngineV1 {
		return PlanningSession{}, fmt.Errorf("unknown engine owner %q", owner)
	}
	now := time.Now().UTC()
	invocation.SessionID = id
	return PlanningSession{
		SchemaVersion: SchemaVersion,
		ID:            id,
		ProjectUUID:   projectUUID,
		EngineOwner:   owner,
		Revision:      1,
		Status:        StatusAnalyzing,
		Goal:          goal,
		Constraints:   append([]string(nil), invocation.Constraints...),
		Invocation:    invocation,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (s PlanningSession) Transition(next SessionStatus) (PlanningSession, error) {
	if terminalStatuses[s.Status] {
		return s, fmt.Errorf("session %s is terminal at %s", s.ID, s.Status)
	}
	if !validTransition(s.Status, next) {
		return s, fmt.Errorf("invalid session transition %s -> %s", s.Status, next)
	}
	s.Status = next
	s.Revision++
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

func validTransition(from, to SessionStatus) bool {
	allowed := map[SessionStatus][]SessionStatus{
		StatusAnalyzing:  {StatusWaiting, StatusReady, StatusStale, StatusCancelled, StatusFailed},
		StatusWaiting:    {StatusAnalyzing, StatusReady, StatusAuthorized, StatusStale, StatusCancelled},
		StatusReady:      {StatusWaiting, StatusAuthorized, StatusStale, StatusCancelled},
		StatusAuthorized: {StatusExecuting, StatusStale, StatusCancelled},
		StatusExecuting:  {StatusVerifying, StatusFailed, StatusStale, StatusCancelled},
		StatusVerifying:  {StatusNeedsReview, StatusCompleted, StatusFailed, StatusStale, StatusCancelled},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return true
		}
	}
	return false
}

func (s PlanningSession) SetProposal(p Proposal) (PlanningSession, error) {
	if terminalStatuses[s.Status] {
		return s, fmt.Errorf("cannot set proposal on terminal session %s", s.ID)
	}
	if s.Status == StatusAuthorized || s.Status == StatusExecuting || s.Status == StatusVerifying {
		return s, fmt.Errorf("cannot replace frozen proposal while session is %s", s.Status)
	}
	if strings.TrimSpace(p.ID) == "" || p.Revision < 1 || strings.TrimSpace(p.ProjectCutHash) == "" {
		return s, errors.New("proposal id, positive revision and project cut hash are required")
	}
	p.TargetScope = append([]string(nil), p.TargetScope...)
	p.Presentation = cloneProposalPresentation(p.Presentation)
	s.ActiveProposal = &p
	s.FrozenPlan = nil
	s.Authorization = nil
	if s.Status == StatusAnalyzing {
		s.Status = StatusWaiting
	} else if s.Status == StatusReady {
		s.Status = StatusWaiting
	}
	s.Revision++
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

func (s PlanningSession) SetFrozenPlan(plan FrozenPlan) (PlanningSession, error) {
	if plan.ActionSet.Hash == "" || plan.ActionSet.Hash != plan.ActionSet.ComputeHash() {
		return s, errors.New("frozen action set hash is missing or invalid")
	}
	if plan.ProjectCut.Hash == "" || plan.ProjectCut.Hash != plan.ProjectCut.ComputeHash() {
		return s, errors.New("frozen project cut hash is missing or invalid")
	}
	if plan.Proposal.ActionSetHash != plan.ActionSet.Hash || plan.Proposal.ProjectCutHash != plan.ProjectCut.Hash {
		return s, errors.New("frozen proposal does not match action set and project cut")
	}
	if plan.ActionSet.ProjectCutHash != plan.ProjectCut.Hash || plan.ActionSet.CapabilityID != plan.Proposal.CapabilityID {
		return s, errors.New("frozen action set does not match proposal capability and cut")
	}
	updated, err := s.SetProposal(plan.Proposal)
	if err != nil {
		return s, err
	}
	if plan.FrozenAt.IsZero() {
		plan.FrozenAt = time.Now().UTC()
	}
	updated.FrozenPlan = cloneFrozenPlan(&plan)
	return updated, nil
}

func (s PlanningSession) Authorize(a Authorization) (PlanningSession, error) {
	if s.ActiveProposal == nil {
		return s, errors.New("cannot authorize without an active proposal")
	}
	if s.Status != StatusWaiting && s.Status != StatusReady {
		return s, fmt.Errorf("cannot authorize session in status %s", s.Status)
	}
	if a.ProposalID != s.ActiveProposal.ID || a.ProposalRevision != s.ActiveProposal.Revision || a.ActionSetHash != s.ActiveProposal.ActionSetHash || a.ProjectCutHash != s.ActiveProposal.ProjectCutHash {
		return s, errors.New("authorization does not match active proposal")
	}
	if strings.TrimSpace(a.SourceTurnID) == "" || a.Sequence == 0 {
		return s, errors.New("authorization source turn and sequence are required")
	}
	a.Scope = append([]string(nil), a.Scope...)
	if a.Decision == nil || !a.Decision.ExactApprovalFor(*s.ActiveProposal) {
		return s, errors.New("authorization requires an exact typed approval decision for the active proposal")
	}
	a.Decision = cloneApprovalDecision(a.Decision)
	a.GrantedAt = time.Now().UTC()
	s.Authorization = &a
	s.Status = StatusAuthorized
	s.Revision++
	s.UpdatedAt = a.GrantedAt
	return s, nil
}

func (s PlanningSession) ClaimAuthorization() (PlanningSession, error) {
	if s.Authorization == nil || s.Authorization.Consumed {
		return s, errors.New("authorization is missing or already consumed")
	}
	s.Authorization.Consumed = true
	s.Revision++
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

// MemoryStore is an intentionally small process-local adapter used by the
// first migration seam and contract tests. It is not the final durable store.
type MemoryStore struct {
	mu       sync.Mutex
	sessions map[string]PlanningSession
}

type Store interface {
	Create(PlanningSession) error
	Load(string) (PlanningSession, bool)
	List() []PlanningSession
	Save(PlanningSession, uint64) error
}

func ListStoreSessions(store Store) ([]PlanningSession, error) {
	if store == nil {
		return nil, errors.New("orchestration store is nil")
	}
	if detailed, ok := store.(interface {
		ListWithError() ([]PlanningSession, error)
	}); ok {
		return detailed.ListWithError()
	}
	return store.List(), nil
}

type DurableStore interface {
	Store
	Durable() bool
}

func IsDurableStore(store Store) bool {
	durable, ok := store.(interface{ Durable() bool })
	return ok && durable.Durable()
}

func (m *MemoryStore) Durable() bool { return false }

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string]PlanningSession)}
}

func (m *MemoryStore) Create(s PlanningSession) error {
	if m == nil {
		return errors.New("nil orchestration store")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessions[s.ID]; exists {
		return fmt.Errorf("session %s already exists", s.ID)
	}
	m.sessions[s.ID] = cloneSession(s)
	return nil
}

func (m *MemoryStore) Load(id string) (PlanningSession, bool) {
	if m == nil {
		return PlanningSession{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return PlanningSession{}, false
	}
	return cloneSession(s), true
}

func (m *MemoryStore) List() []PlanningSession {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]PlanningSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		out = append(out, cloneSession(session))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].UpdatedAt.Before(out[j].UpdatedAt)
	})
	return out
}

func (m *MemoryStore) Save(s PlanningSession, expectedRevision uint64) error {
	if m == nil {
		return errors.New("nil orchestration store")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.sessions[s.ID]
	if !ok {
		return fmt.Errorf("session %s not found", s.ID)
	}
	if current.Revision != expectedRevision {
		return fmt.Errorf("session %s revision conflict: expected %d, current %d", s.ID, expectedRevision, current.Revision)
	}
	if current.EngineOwner != s.EngineOwner {
		return errors.New("session engine owner is immutable")
	}
	m.sessions[s.ID] = cloneSession(s)
	return nil
}

func cloneSession(s PlanningSession) PlanningSession {
	s.Constraints = append([]string(nil), s.Constraints...)
	s.Invocation.Constraints = append([]string(nil), s.Invocation.Constraints...)
	s.Invocation.TargetRefs = append([]string(nil), s.Invocation.TargetRefs...)
	if s.ActiveProposal != nil {
		p := *s.ActiveProposal
		p.TargetScope = append([]string(nil), p.TargetScope...)
		p.Presentation = cloneProposalPresentation(p.Presentation)
		s.ActiveProposal = &p
	}
	if s.FrozenPlan != nil {
		s.FrozenPlan = cloneFrozenPlan(s.FrozenPlan)
	}
	if s.Authorization != nil {
		a := *s.Authorization
		a.Scope = append([]string(nil), a.Scope...)
		a.Decision = cloneApprovalDecision(a.Decision)
		s.Authorization = &a
	}
	if s.Execution != nil {
		e := *s.Execution
		e.Receipts = cloneActionReceipts(s.Execution.Receipts)
		e.PersistenceRefs = append([]string(nil), s.Execution.PersistenceRefs...)
		e.ProjectCut.DependencyFingerprints = append([]string(nil), s.Execution.ProjectCut.DependencyFingerprints...)
		e.ProjectCut.ArtifactRefs = append([]string(nil), s.Execution.ProjectCut.ArtifactRefs...)
		e.ProjectCut.DerivedFrom = append([]string(nil), s.Execution.ProjectCut.DerivedFrom...)
		e.ProjectCut.TargetFingerprints = append([]string(nil), s.Execution.ProjectCut.TargetFingerprints...)
		e.ProjectCut.ContractVersions = append([]string(nil), s.Execution.ProjectCut.ContractVersions...)
		if s.Execution.VerificationResult != nil {
			result := *s.Execution.VerificationResult
			result.EvidenceRefs = append([]string(nil), s.Execution.VerificationResult.EvidenceRefs...)
			e.VerificationResult = &result
		}
		s.Execution = &e
	}
	return s
}

func cloneActionReceipts(in []ActionReceipt) []ActionReceipt {
	if len(in) == 0 {
		return nil
	}
	out := make([]ActionReceipt, len(in))
	for index, receipt := range in {
		out[index] = receipt
		out[index].EvidenceRefs = append([]string(nil), receipt.EvidenceRefs...)
		if len(receipt.Details) > 0 {
			data, err := json.Marshal(receipt.Details)
			if err == nil {
				_ = json.Unmarshal(data, &out[index].Details)
			}
			if out[index].Details == nil {
				out[index].Details = map[string]any{}
				for key, value := range receipt.Details {
					out[index].Details[key] = value
				}
			}
		}
	}
	return out
}

func cloneFrozenPlan(plan *FrozenPlan) *FrozenPlan {
	if plan == nil {
		return nil
	}
	data, err := json.Marshal(plan)
	if err != nil {
		copy := *plan
		return &copy
	}
	var out FrozenPlan
	if err := json.Unmarshal(data, &out); err != nil {
		copy := *plan
		return &copy
	}
	return &out
}

// FileStore is a restart-safe migration adapter. Operations hold an OS-backed
// lock across read/CAS/write and writes use a same-directory temporary file
// plus rename, preventing lost updates when multiple VitAgent processes share
// the default per-user store.
type FileStore struct {
	mu   sync.Mutex
	path string
}

func (f *FileStore) Durable() bool { return f != nil }

func NewFileStore(path string) (*FileStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("orchestration store path is required")
	}
	return &FileStore{path: path}, nil
}

func DefaultFileStorePath() string {
	if value := strings.TrimSpace(os.Getenv("VIT_ORCHESTRATION_STORE_PATH")); value != "" {
		if strings.EqualFold(value, "memory") || strings.EqualFold(value, "off") || strings.EqualFold(value, "disabled") {
			return ""
		}
		return value
	}
	if root, err := os.UserConfigDir(); err == nil && strings.TrimSpace(root) != "" {
		return filepath.Join(root, "Vit", "Agent", "orchestration_v1.json")
	}
	return filepath.Join("VitApp", "Workspace", "State", "orchestration_v1.json")
}

func (f *FileStore) Create(s PlanningSession) error {
	if f == nil {
		return errors.New("nil file orchestration store")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	release, err := acquireStoreFileLock(f.path)
	if err != nil {
		return err
	}
	defer release()
	all, err := f.loadLocked()
	if err != nil {
		return err
	}
	if _, exists := all[s.ID]; exists {
		return fmt.Errorf("session %s already exists", s.ID)
	}
	all[s.ID] = cloneSession(s)
	return f.persistLocked(all)
}

func (f *FileStore) Load(id string) (PlanningSession, bool) {
	if f == nil {
		return PlanningSession{}, false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	release, err := acquireStoreFileLock(f.path)
	if err != nil {
		return PlanningSession{}, false
	}
	defer release()
	all, err := f.loadLocked()
	if err != nil {
		return PlanningSession{}, false
	}
	s, ok := all[id]
	if !ok {
		return PlanningSession{}, false
	}
	return cloneSession(s), true
}

func (f *FileStore) List() []PlanningSession {
	out, _ := f.ListWithError()
	return out
}

func (f *FileStore) ListWithError() ([]PlanningSession, error) {
	if f == nil {
		return nil, errors.New("nil file orchestration store")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	release, err := acquireStoreFileLock(f.path)
	if err != nil {
		return nil, err
	}
	defer release()
	all, err := f.loadLocked()
	if err != nil {
		return nil, err
	}
	out := make([]PlanningSession, 0, len(all))
	for _, session := range all {
		out = append(out, cloneSession(session))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].UpdatedAt.Before(out[j].UpdatedAt)
	})
	return out, nil
}

func (f *FileStore) Save(s PlanningSession, expectedRevision uint64) error {
	if f == nil {
		return errors.New("nil file orchestration store")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	release, err := acquireStoreFileLock(f.path)
	if err != nil {
		return err
	}
	defer release()
	all, err := f.loadLocked()
	if err != nil {
		return err
	}
	current, ok := all[s.ID]
	if !ok {
		return fmt.Errorf("session %s not found", s.ID)
	}
	if current.Revision != expectedRevision {
		return fmt.Errorf("session %s revision conflict: expected %d, current %d", s.ID, expectedRevision, current.Revision)
	}
	if current.EngineOwner != s.EngineOwner {
		return errors.New("session engine owner is immutable")
	}
	all[s.ID] = cloneSession(s)
	return f.persistLocked(all)
}

func (f *FileStore) loadLocked() (map[string]PlanningSession, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]PlanningSession{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read orchestration store: %w", err)
	}
	var all map[string]PlanningSession
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, fmt.Errorf("decode orchestration store: %w", err)
	}
	if all == nil {
		all = map[string]PlanningSession{}
	}
	return all, nil
}

func (f *FileStore) persistLocked(all map[string]PlanningSession) error {
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("encode orchestration store: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("create orchestration store directory: %w", err)
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write orchestration store: %w", err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit orchestration store: %w", err)
	}
	return nil
}
