package spallab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/orchestration"
)

type journalDocument struct {
	SchemaVersion string                  `json:"schema_version"`
	Entries       []ExecutionJournalEntry `json:"entries"`
}

// Journal creates a durable pre-execution record before the Coordinator may
// submit a mutation. The Coordinator's orchestration store remains the source
// of truth for final action receipts and verification results.
type Journal struct {
	mu   sync.Mutex
	path string
}

func NewJournal(path string) (*Journal, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("SPAL lab journal path is required")
	}
	return &Journal{path: path}, nil
}

func (j *Journal) Path() string {
	if j == nil {
		return ""
	}
	return j.path
}

func (j *Journal) PrepareExecution(_ context.Context, session orchestration.PlanningSession, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) ([]string, error) {
	if j == nil {
		return nil, errors.New("SPAL lab journal is nil")
	}
	if strings.TrimSpace(session.ID) == "" || strings.TrimSpace(actionSet.Hash) == "" || strings.TrimSpace(cut.Hash) == "" {
		return nil, errors.New("SPAL lab execution journal requires a session, action set and Project Cut")
	}
	executionID := "execution_" + session.ID + "_" + actionSet.Hash[:minJournalHashLength(len(actionSet.Hash))]
	if session.Execution != nil && strings.TrimSpace(session.Execution.ID) != "" {
		executionID = session.Execution.ID
	}
	entry := ExecutionJournalEntry{
		SchemaVersion:  SchemaVersion,
		SessionID:      session.ID,
		ExecutionID:    executionID,
		ActionSetHash:  actionSet.Hash,
		ProjectCutHash: cut.Hash,
		PreparedAt:     time.Now().UTC(),
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	document, err := j.loadLocked()
	if err != nil {
		return nil, err
	}
	for index, existing := range document.Entries {
		if existing.ExecutionID == entry.ExecutionID {
			document.Entries[index] = entry
			if err := j.persistLocked(document); err != nil {
				return nil, err
			}
			return []string{journalReference(j.path, entry.ExecutionID)}, nil
		}
	}
	document.Entries = append(document.Entries, entry)
	if err := j.persistLocked(document); err != nil {
		return nil, err
	}
	return []string{journalReference(j.path, entry.ExecutionID)}, nil
}

func (j *Journal) Entries() ([]ExecutionJournalEntry, error) {
	if j == nil {
		return nil, errors.New("SPAL lab journal is nil")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	document, err := j.loadLocked()
	if err != nil {
		return nil, err
	}
	return append([]ExecutionJournalEntry(nil), document.Entries...), nil
}

func (j *Journal) loadLocked() (journalDocument, error) {
	data, err := os.ReadFile(j.path)
	if errors.Is(err, os.ErrNotExist) {
		return journalDocument{SchemaVersion: SchemaVersion, Entries: []ExecutionJournalEntry{}}, nil
	}
	if err != nil {
		return journalDocument{}, fmt.Errorf("read SPAL lab journal: %w", err)
	}
	var document journalDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return journalDocument{}, fmt.Errorf("decode SPAL lab journal: %w", err)
	}
	if document.SchemaVersion == "" {
		document.SchemaVersion = SchemaVersion
	}
	if document.SchemaVersion != SchemaVersion {
		return journalDocument{}, fmt.Errorf("unsupported SPAL lab journal schema %q", document.SchemaVersion)
	}
	if document.Entries == nil {
		document.Entries = []ExecutionJournalEntry{}
	}
	return document, nil
}

func (j *Journal) persistLocked(document journalDocument) error {
	document.SchemaVersion = SchemaVersion
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode SPAL lab journal: %w", err)
	}
	directory := filepath.Dir(j.path)
	if directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("create SPAL lab journal directory: %w", err)
		}
	}
	temporary := j.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write SPAL lab journal: %w", err)
	}
	if err := os.Rename(temporary, j.path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit SPAL lab journal: %w", err)
	}
	return nil
}

func journalReference(path, executionID string) string {
	return "spal_lab_journal:" + filepath.Clean(path) + "#" + executionID
}

func minJournalHashLength(length int) int {
	if length < 16 {
		return length
	}
	return 16
}
