package spallab

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/spal"
)

type providerStoreDocument struct {
	SchemaVersion string           `json:"schema_version"`
	Records       []ProviderRecord `json:"records"`
}

// ProviderStore is a small durable registry of explicitly conformed reference
// instances.  A record is never a global default: product resolution must
// still bind it to its exact project and target.
type ProviderStore struct {
	mu   sync.Mutex
	path string
}

func NewProviderStore(path string) (*ProviderStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("SPAL lab provider store path is required")
	}
	return &ProviderStore{path: path}, nil
}

func DefaultProviderStorePath() string {
	if path := strings.TrimSpace(os.Getenv("VIT_SPAL_LAB_PROVIDER_STORE")); path != "" {
		return path
	}
	if root, err := os.UserConfigDir(); err == nil && strings.TrimSpace(root) != "" {
		return filepath.Join(root, "Vit", "Agent", "spal_lab_providers_v0.json")
	}
	return filepath.Join("VitApp", "Workspace", "State", "spal_lab_providers_v0.json")
}

// DefaultReferenceEQProviderStorePath is deliberately separate from the
// developer-lab store.  It is the durable product bridge for the Reference EQ
// vertical slice, while future Provider Registry work can replace its storage
// implementation without changing the SPAL semantic API.
func DefaultReferenceEQProviderStorePath() string {
	if path := strings.TrimSpace(os.Getenv("VIT_SPAL_REFERENCE_EQ_PROVIDER_STORE")); path != "" {
		return path
	}
	if root, err := os.UserConfigDir(); err == nil && strings.TrimSpace(root) != "" {
		return filepath.Join(root, "Vit", "Agent", "spal_reference_eq_providers_v0.json")
	}
	return filepath.Join("VitApp", "Workspace", "State", "spal_reference_eq_providers_v0.json")
}

func DefaultSessionStorePath() string {
	if path := strings.TrimSpace(os.Getenv("VIT_SPAL_LAB_SESSION_STORE")); path != "" {
		return path
	}
	if root, err := os.UserConfigDir(); err == nil && strings.TrimSpace(root) != "" {
		return filepath.Join(root, "Vit", "Agent", "spal_lab_orchestration_v0.json")
	}
	return filepath.Join("VitApp", "Workspace", "State", "spal_lab_orchestration_v0.json")
}

func DefaultJournalPath() string {
	if path := strings.TrimSpace(os.Getenv("VIT_SPAL_LAB_JOURNAL")); path != "" {
		return path
	}
	if root, err := os.UserConfigDir(); err == nil && strings.TrimSpace(root) != "" {
		return filepath.Join(root, "Vit", "Agent", "spal_lab_execution_journal_v0.json")
	}
	return filepath.Join("VitApp", "Workspace", "State", "spal_lab_execution_journal_v0.json")
}

func (s *ProviderStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *ProviderStore) Upsert(record ProviderRecord) (ProviderRecord, error) {
	if s == nil {
		return ProviderRecord{}, errors.New("SPAL lab provider store is nil")
	}
	if err := record.Valid(); err != nil {
		return ProviderRecord{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.loadLocked()
	if err != nil {
		return ProviderRecord{}, err
	}
	now := time.Now().UTC()
	updated := false
	for index, existing := range document.Records {
		if existing.ID != record.ID {
			continue
		}
		if !existing.CreatedAt.IsZero() {
			record.CreatedAt = existing.CreatedAt
		}
		record.UpdatedAt = now
		document.Records[index] = cloneRecord(record)
		updated = true
		break
	}
	if !updated {
		if record.CreatedAt.IsZero() {
			record.CreatedAt = now
		}
		record.UpdatedAt = now
		document.Records = append(document.Records, cloneRecord(record))
	}
	sort.Slice(document.Records, func(i, j int) bool { return document.Records[i].ID < document.Records[j].ID })
	if err := s.persistLocked(document); err != nil {
		return ProviderRecord{}, err
	}
	return cloneRecord(record), nil
}

func (s *ProviderStore) List() ([]ProviderRecord, error) {
	if s == nil {
		return nil, errors.New("SPAL lab provider store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	out := make([]ProviderRecord, 0, len(document.Records))
	for _, record := range document.Records {
		if err := record.Valid(); err != nil {
			return nil, fmt.Errorf("invalid persisted SPAL lab record %s: %w", record.ID, err)
		}
		out = append(out, cloneRecord(record))
	}
	return out, nil
}

func (s *ProviderStore) Instances() ([]spal.ProviderInstance, error) {
	records, err := s.List()
	if err != nil {
		return nil, err
	}
	instances := make([]spal.ProviderInstance, 0, len(records))
	for _, record := range records {
		instances = append(instances, cloneRecord(record).Instance)
	}
	return instances, nil
}

// Registry is intentionally constructed only by the lab. It registers the
// TDR Nova adapter but cannot cause a fallback: Resolve still requires one of
// the persisted verified instances passed separately to SPAL Runtime.
func (s *ProviderStore) Registry() (*spal.Registry, error) {
	if s == nil {
		return nil, errors.New("SPAL lab provider store is nil")
	}
	adapter, err := spal.NewExperimentalTDRNovaAdapter()
	if err != nil {
		return nil, err
	}
	registry := spal.NewRegistry()
	if err := registry.Register(adapter); err != nil {
		return nil, err
	}
	return registry, nil
}

func (s *ProviderStore) loadLocked() (providerStoreDocument, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return providerStoreDocument{SchemaVersion: SchemaVersion, Records: []ProviderRecord{}}, nil
	}
	if err != nil {
		return providerStoreDocument{}, fmt.Errorf("read SPAL lab provider store: %w", err)
	}
	var document providerStoreDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return providerStoreDocument{}, fmt.Errorf("decode SPAL lab provider store: %w", err)
	}
	if document.SchemaVersion == "" {
		document.SchemaVersion = SchemaVersion
	}
	if document.SchemaVersion != SchemaVersion {
		return providerStoreDocument{}, fmt.Errorf("unsupported SPAL lab provider store schema %q", document.SchemaVersion)
	}
	if document.Records == nil {
		document.Records = []ProviderRecord{}
	}
	return document, nil
}

func (s *ProviderStore) persistLocked(document providerStoreDocument) error {
	document.SchemaVersion = SchemaVersion
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode SPAL lab provider store: %w", err)
	}
	if directory := filepath.Dir(s.path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("create SPAL lab provider store directory: %w", err)
		}
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write SPAL lab provider store: %w", err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit SPAL lab provider store: %w", err)
	}
	return nil
}

func cloneRecord(record ProviderRecord) ProviderRecord {
	data, err := json.Marshal(record)
	if err != nil {
		return record
	}
	var copy ProviderRecord
	if json.Unmarshal(data, &copy) != nil {
		return record
	}
	return copy
}
