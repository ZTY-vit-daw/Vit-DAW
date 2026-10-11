package mixboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/projectstore"
)

// MIXLAYER-M1-1 free-state conclusion store: the durable, project-keyed
// persistence layer for free-state settle conclusions (experiment settle/stop
// and loop closure settles, including the user human-audition judgment
// outcome). Until now the settle state lived only in the in-process
// freeStateLoops map keyed by conversation_id, so a conclusion reached in one
// session was invisible to every later session. Records land next to the
// decision ledger (a free_state_conclusions directory beside the ledger's
// decisions directory under the same project-scoped root), are immutable once
// written, and are idempotent by ConclusionID.
//
// These records are an independent "free-state conclusions" blackboard block:
// they are NOT capability sessions and are not forced through
// MixDecisionRecord. The ledger's needs_review linkage keeps governing the
// ledger's own records; no parallel recheck graph is introduced here.
const (
	FreeStateConclusionRecordSchemaVersion = "free_state_conclusion.v1"
	FreeStateConclusionBoardSchemaVersion  = "free_state_conclusion_board.v1"

	FreeStateConclusionKindExperimentSettle  = "experiment_settle"
	FreeStateConclusionKindExperimentStop    = "experiment_stop"
	FreeStateConclusionKindLoopClosureSettle = "loop_closure_settle"
)

// FreeStateJudgmentOutcome carries the user's human-audition judgment facts
// for settles that were decided by one. It is audit metadata copied from the
// experiment's UserJudgmentEvidence; the evidence row itself stays owned by
// the experiment/trajectory layer.
type FreeStateJudgmentOutcome struct {
	EvidenceID        string `json:"evidence_id,omitempty"`
	AuditionSessionID string `json:"audition_session_id,omitempty"`
	Preference        string `json:"preference,omitempty"`
	HeardDifference   string `json:"heard_difference,omitempty"`
	Blind             bool   `json:"blind,omitempty"`
}

// FreeStateConclusionRecord is one immutable settle conclusion unit. Outcome
// carries the experiment SettlementOutcome (improved / rolled_back /
// needs_judgment / adopted_by_continuation / budget_exhausted / stopped ...)
// or, for loop closure settles, the loop's terminal decision status
// (no_candidate_found / capability_blocked / completed / cancelled ...).
// Unknown Outcome/Kind values from future schema revisions are preserved
// verbatim by readers: this is reporting data, and fail-open display beats
// dropping a conclusion the user acted on.
type FreeStateConclusionRecord struct {
	SchemaVersion   string                    `json:"schema_version"`
	ConclusionID    string                    `json:"conclusion_id"`
	ProjectUUID     string                    `json:"project_uuid"`
	ConversationID  string                    `json:"conversation_id"`
	GoalID          string                    `json:"goal_id,omitempty"`
	RunID           string                    `json:"run_id,omitempty"`
	LoopID          string                    `json:"loop_id,omitempty"`
	ExperimentID    string                    `json:"experiment_id,omitempty"`
	Kind            string                    `json:"kind"`
	Outcome         string                    `json:"outcome"`
	DecisionStatus  string                    `json:"decision_status,omitempty"`
	Summary         string                    `json:"summary"`
	CheckpointRef   string                    `json:"checkpoint_ref,omitempty"`
	ProjectRevision string                    `json:"project_revision,omitempty"`
	TargetRef       map[string]any            `json:"target_ref,omitempty"`
	Judgment        *FreeStateJudgmentOutcome `json:"judgment,omitempty"`
	EvidenceRefs    []string                  `json:"evidence_refs,omitempty"`
	SettledAt       time.Time                 `json:"settled_at"`
	RecordedAt      time.Time                 `json:"recorded_at"`
}

// FreeStateConclusionView is the lean projection the project blackboard
// report reads. It deliberately carries no receipts or payloads: references
// and terminal facts only.
type FreeStateConclusionView struct {
	ConclusionID   string                    `json:"conclusion_id"`
	Kind           string                    `json:"kind"`
	Outcome        string                    `json:"outcome"`
	DecisionStatus string                    `json:"decision_status,omitempty"`
	Summary        string                    `json:"summary"`
	ConversationID string                    `json:"conversation_id"`
	ExperimentID   string                    `json:"experiment_id,omitempty"`
	CheckpointRef  string                    `json:"checkpoint_ref,omitempty"`
	Judgment       *FreeStateJudgmentOutcome `json:"judgment,omitempty"`
	SettledAt      time.Time                 `json:"settled_at"`
}

type FreeStateConclusionBoard struct {
	SchemaVersion   string                    `json:"schema_version"`
	ProjectUUID     string                    `json:"project_uuid"`
	ConclusionCount int                       `json:"conclusion_count"`
	Conclusions     []FreeStateConclusionView `json:"conclusions"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

// RecordFreeStateConclusion persists one settle conclusion. An existing record
// with the same ConclusionID and identical immutable content is a no-op (the
// original RecordedAt is kept); the same ID with different content is an
// error, mirroring the decision ledger's immutability contract.
func (s Store) RecordFreeStateConclusion(record FreeStateConclusionRecord) (FreeStateConclusionRecord, error) {
	record.SchemaVersion = FreeStateConclusionRecordSchemaVersion
	record.ProjectUUID = strings.TrimSpace(record.ProjectUUID)
	record.ConclusionID = strings.TrimSpace(record.ConclusionID)
	record.ConversationID = strings.TrimSpace(record.ConversationID)
	record.Kind = strings.TrimSpace(record.Kind)
	record.Outcome = strings.TrimSpace(record.Outcome)
	if record.ProjectUUID == "" {
		return FreeStateConclusionRecord{}, fmt.Errorf("free state conclusion %s omitted project uuid", record.ConclusionID)
	}
	if record.ConclusionID == "" {
		return FreeStateConclusionRecord{}, fmt.Errorf("free state conclusion for project %s omitted conclusion id", record.ProjectUUID)
	}
	switch record.Kind {
	case FreeStateConclusionKindExperimentSettle, FreeStateConclusionKindExperimentStop, FreeStateConclusionKindLoopClosureSettle:
	default:
		return FreeStateConclusionRecord{}, fmt.Errorf("free state conclusion %s has unknown kind %q", record.ConclusionID, record.Kind)
	}
	if record.Outcome == "" {
		return FreeStateConclusionRecord{}, fmt.Errorf("free state conclusion %s omitted outcome", record.ConclusionID)
	}
	if record.SettledAt.IsZero() {
		record.SettledAt = s.now().UTC()
	}
	record.SettledAt = record.SettledAt.UTC()
	recordedAt := s.now().UTC()
	conclusionDir := s.freeStateConclusionDir(record.ProjectUUID)
	if err := os.MkdirAll(conclusionDir, 0o755); err != nil {
		return FreeStateConclusionRecord{}, err
	}
	recordPath := filepath.Join(conclusionDir, safePathName(record.ConclusionID)+".json")
	if existing, readErr := os.ReadFile(recordPath); readErr == nil {
		var prior FreeStateConclusionRecord
		if err := json.Unmarshal(existing, &prior); err != nil {
			return FreeStateConclusionRecord{}, fmt.Errorf("read existing free state conclusion %s: %w", record.ConclusionID, err)
		}
		record.RecordedAt = prior.RecordedAt
		if !freeStateConclusionRecordsEqual(prior, record) {
			return FreeStateConclusionRecord{}, fmt.Errorf("free state conclusion %s already exists with different immutable content", record.ConclusionID)
		}
		record = prior
	} else if !os.IsNotExist(readErr) {
		return FreeStateConclusionRecord{}, readErr
	} else {
		record.RecordedAt = recordedAt
		if err := writeJSON(recordPath, record); err != nil {
			return FreeStateConclusionRecord{}, err
		}
	}
	board, err := s.buildFreeStateConclusionBoard(record.ProjectUUID)
	if err != nil {
		return FreeStateConclusionRecord{}, err
	}
	if err := writeJSON(filepath.Join(s.freeStateConclusionDir(record.ProjectUUID), "free_state_conclusions.json"), board); err != nil {
		return FreeStateConclusionRecord{}, err
	}
	return record, nil
}

// ReadFreeStateConclusionBoard returns the persisted conclusions for a
// project. A project that never wrote conclusions (every project older than
// this store) reads as an empty board with no error — that missing-files
// default is the documented old-project semantics the blackboard report
// relies on. An empty projectUUID falls back to the currently bound
// projectstore roots before resolving, so the read follows the same
// project-scoped root the write used.
func (s Store) ReadFreeStateConclusionBoard(projectUUID string) (FreeStateConclusionBoard, error) {
	projectUUID = strings.TrimSpace(projectUUID)
	if projectUUID == "" {
		if roots, ok := projectstore.Current(); ok && strings.TrimSpace(roots.ProjectUUID) != "" {
			projectUUID = strings.TrimSpace(roots.ProjectUUID)
		}
	}
	if projectUUID == "" {
		return FreeStateConclusionBoard{}, fmt.Errorf("project uuid is required")
	}
	return s.buildFreeStateConclusionBoard(projectUUID)
}

func (s Store) freeStateConclusionDir(projectUUID string) string {
	// Sibling of the ledger's decisions directory under the same
	// project-scoped projectDecisionDir root (card MIXLAYER-M1-1: "账本
	// decisions/ 同级").
	return filepath.Join(s.projectDecisionDir(projectUUID), "free_state_conclusions")
}

func (s Store) buildFreeStateConclusionBoard(projectUUID string) (FreeStateConclusionBoard, error) {
	records, err := s.readFreeStateConclusionRecords(projectUUID)
	if err != nil {
		return FreeStateConclusionBoard{}, err
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].SettledAt.Equal(records[j].SettledAt) {
			return records[i].ConclusionID < records[j].ConclusionID
		}
		return records[i].SettledAt.Before(records[j].SettledAt)
	})
	views := make([]FreeStateConclusionView, 0, len(records))
	for _, record := range records {
		views = append(views, FreeStateConclusionView{
			ConclusionID: record.ConclusionID, Kind: record.Kind, Outcome: record.Outcome,
			DecisionStatus: record.DecisionStatus, Summary: record.Summary,
			ConversationID: record.ConversationID, ExperimentID: record.ExperimentID,
			CheckpointRef: record.CheckpointRef, Judgment: record.Judgment,
			SettledAt: record.SettledAt,
		})
	}
	return FreeStateConclusionBoard{
		SchemaVersion: FreeStateConclusionBoardSchemaVersion, ProjectUUID: projectUUID,
		ConclusionCount: len(views), Conclusions: views, UpdatedAt: s.now().UTC(),
	}, nil
}

// readFreeStateConclusionRecords reads every conclusion record for the
// project. Records with a different schema version or project uuid are
// skipped, not errors (forward compatibility); malformed JSON is an error.
func (s Store) readFreeStateConclusionRecords(projectUUID string) ([]FreeStateConclusionRecord, error) {
	dir := s.freeStateConclusionDir(projectUUID)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	records := make([]FreeStateConclusionRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		if entry.Name() == "free_state_conclusions.json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var record FreeStateConclusionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("read free state conclusion %s: %w", entry.Name(), err)
		}
		if record.SchemaVersion != FreeStateConclusionRecordSchemaVersion || record.ProjectUUID != projectUUID {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

func freeStateConclusionRecordsEqual(a, b FreeStateConclusionRecord) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}
