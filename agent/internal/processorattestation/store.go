package processorattestation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const FileName = "processor_control_attestations.v1.json"

type ReadReport struct {
	Path                string `json:"path"`
	RecoveredFromBackup bool   `json:"recovered_from_backup"`
	PrimaryError        string `json:"primary_error,omitempty"`
}

type Store struct {
	Path string
	now  func() time.Time
	mu   sync.Mutex
}

func DefaultPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("VIT_PROCESSOR_ATTESTATIONS_PATH")); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("processor attestation: user home: %w", err)
	}
	return filepath.Join(home, ".vit", FileName), nil
}

func NewStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	return &Store{Path: filepath.Clean(path), now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *Store) Read() (Library, ReadReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readUnlocked()
}

func (s *Store) Issue(spec IssueSpec) (Attestation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	library, _, err := s.readUnlocked()
	if err != nil {
		return Attestation{}, err
	}
	attestation, err := NewAttestation(spec, s.now())
	if err != nil {
		return Attestation{}, err
	}
	for _, existing := range library.Attestations {
		if existing.AttestationID == attestation.AttestationID {
			return existing, nil
		}
	}
	library.Attestations = append(library.Attestations, attestation)
	sort.Slice(library.Attestations, func(i, j int) bool {
		return library.Attestations[i].AttestationID < library.Attestations[j].AttestationID
	})
	if err := s.writeUnlocked(&library); err != nil {
		return Attestation{}, err
	}
	return attestation, nil
}

func (s *Store) Promote(attestationID, reason string) (Attestation, error) {
	return s.transition(attestationID, StatusPromoted, reason)
}

func (s *Store) MarkStale(attestationID, reason string) (Attestation, error) {
	return s.transition(attestationID, StatusStale, reason)
}

func (s *Store) Revoke(attestationID, reason string) (Attestation, error) {
	return s.transition(attestationID, StatusRevoked, reason)
}

func (s *Store) Query(query Query) (QueryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	library, _, err := s.readUnlocked()
	if err != nil {
		return QueryResult{}, err
	}
	query.SubjectKey = strings.ToLower(strings.TrimSpace(query.SubjectKey))
	query.BinaryFingerprint = strings.ToLower(strings.TrimSpace(query.BinaryFingerprint))
	query.ProcessorFamily = strings.ToLower(strings.TrimSpace(query.ProcessorFamily))
	query.RequiredCoverage = normalizeCoverage(query.RequiredCoverage)
	if query.SubjectKey == "" || !validSHA256(query.BinaryFingerprint) {
		return QueryResult{}, fmt.Errorf("processor attestation: query requires subject_key and sha256 binary fingerprint")
	}
	if query.ProcessorFamily != FamilyStaticEQ && query.ProcessorFamily != FamilyBroadbandCompressor {
		return QueryResult{}, fmt.Errorf("processor attestation: query has unsupported processor family")
	}
	if len(query.RequiredCoverage) == 0 {
		return QueryResult{}, fmt.Errorf("processor attestation: query requires current action coverage")
	}
	for _, coverage := range query.RequiredCoverage {
		if err := validateCoverage(query.ProcessorFamily, coverage); err != nil {
			return QueryResult{}, fmt.Errorf("processor attestation: invalid query coverage: %w", err)
		}
	}
	var best *Attestation
	for index := range library.Attestations {
		candidate := &library.Attestations[index]
		if candidate.Subject.SubjectKey != query.SubjectKey || candidate.ProcessorFamily != query.ProcessorFamily {
			continue
		}
		candidateFingerprintMatch := candidate.BinaryFingerprint == query.BinaryFingerprint
		bestFingerprintMatch := best != nil && best.BinaryFingerprint == query.BinaryFingerprint
		if best == nil || (candidateFingerprintMatch && !bestFingerprintMatch) ||
			(candidateFingerprintMatch == bestFingerprintMatch && statusRank(candidate.Status) > statusRank(best.Status)) ||
			(candidateFingerprintMatch == bestFingerprintMatch && statusRank(candidate.Status) == statusRank(best.Status) && candidate.AttestationID < best.AttestationID) {
			best = candidate
		}
	}
	if best == nil {
		return QueryResult{EffectiveStatus: "missing", Reason: "no_attestation", MissingCoverage: query.RequiredCoverage}, nil
	}
	result := QueryResult{Attestation: *best, EffectiveStatus: best.Status, Reason: "attestation_not_promoted",
		FingerprintMatch: best.BinaryFingerprint == query.BinaryFingerprint}
	if !result.FingerprintMatch {
		result.EffectiveStatus = StatusStale
		result.Reason = "binary_fingerprint_changed"
		result.MissingCoverage = query.RequiredCoverage
		return result, nil
	}
	if best.Status != StatusPromoted {
		result.Reason = "attestation_" + best.Status
		result.MissingCoverage = query.RequiredCoverage
		return result, nil
	}
	result.MissingCoverage = missingCoverage(best.Coverage, query.RequiredCoverage)
	if len(result.MissingCoverage) > 0 {
		result.Reason = "required_action_not_covered"
		return result, nil
	}
	result.Eligible = true
	result.Reason = "promoted_action_covered"
	return result, nil
}

func (s *Store) transition(attestationID, nextStatus, reason string) (Attestation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	library, _, err := s.readUnlocked()
	if err != nil {
		return Attestation{}, err
	}
	attestationID = strings.TrimSpace(attestationID)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Attestation{}, fmt.Errorf("processor attestation: transition reason is required")
	}
	for index := range library.Attestations {
		attestation := &library.Attestations[index]
		if attestation.AttestationID != attestationID {
			continue
		}
		if attestation.Status == nextStatus {
			return *attestation, nil
		}
		if err := allowedTransition(attestation.Status, nextStatus); err != nil {
			return Attestation{}, err
		}
		now := s.now().UTC()
		attestation.Status, attestation.StatusReason = nextStatus, reason
		switch nextStatus {
		case StatusPromoted:
			attestation.PromotedAt = &now
		case StatusStale:
			attestation.StaleAt = &now
		case StatusRevoked:
			attestation.RevokedAt = &now
		}
		if err := s.writeUnlocked(&library); err != nil {
			return Attestation{}, err
		}
		return *attestation, nil
	}
	return Attestation{}, fmt.Errorf("processor attestation: attestation %q not found", attestationID)
}

func allowedTransition(current, next string) error {
	allowed := false
	switch next {
	case StatusPromoted:
		allowed = current == StatusIssued
	case StatusStale:
		allowed = current == StatusIssued || current == StatusPromoted
	case StatusRevoked:
		allowed = current == StatusIssued || current == StatusPromoted || current == StatusStale
	}
	if !allowed {
		return fmt.Errorf("processor attestation: transition %s -> %s is not allowed", current, next)
	}
	return nil
}

func (s *Store) readUnlocked() (Library, ReadReport, error) {
	report := ReadReport{Path: s.Path}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return emptyLibrary(), report, nil
	}
	if err == nil {
		library, decodeErr := decodeLibrary(data)
		if decodeErr == nil {
			return library, report, nil
		}
		err = decodeErr
	}
	report.PrimaryError = err.Error()
	backupData, backupErr := os.ReadFile(s.Path + ".bak")
	if backupErr != nil {
		return Library{}, report, fmt.Errorf("processor attestation: read primary: %v; read backup: %w", err, backupErr)
	}
	library, backupErr := decodeLibrary(backupData)
	if backupErr != nil {
		return Library{}, report, fmt.Errorf("processor attestation: read primary: %v; decode backup: %w", err, backupErr)
	}
	report.RecoveredFromBackup = true
	return library, report, nil
}

func (s *Store) writeUnlocked(library *Library) error {
	library.SchemaVersion = LibrarySchema
	library.Revision++
	library.UpdatedAt = s.now().UTC()
	if err := library.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(library, "", "  ")
	if err != nil {
		return fmt.Errorf("processor attestation: marshal library: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("processor attestation: create store directory: %w", err)
	}
	if current, readErr := os.ReadFile(s.Path); readErr == nil {
		if _, decodeErr := decodeLibrary(current); decodeErr == nil {
			if err := replaceFile(s.Path+".bak", current); err != nil {
				return fmt.Errorf("processor attestation: write backup: %w", err)
			}
		}
	}
	if err := replaceFile(s.Path, data); err != nil {
		return fmt.Errorf("processor attestation: replace store: %w", err)
	}
	return nil
}

func decodeLibrary(data []byte) (Library, error) {
	var library Library
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&library); err != nil {
		return Library{}, fmt.Errorf("decode library: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Library{}, fmt.Errorf("decode library: trailing JSON content")
	}
	if err := library.Validate(); err != nil {
		return Library{}, err
	}
	return library, nil
}

func replaceFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	cleanup := func() { _ = os.Remove(tempPath) }
	defer cleanup()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	oldPath := path + ".replace-old"
	_ = os.Remove(oldPath)
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, oldPath); err != nil {
			return err
		}
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Rename(oldPath, path)
		return err
	}
	_ = os.Remove(oldPath)
	return nil
}

func emptyLibrary() Library {
	return Library{SchemaVersion: LibrarySchema, Attestations: []Attestation{}}
}

func missingCoverage(have, required []Coverage) []Coverage {
	available := map[string]bool{}
	for _, value := range have {
		available[coverageKey(value)] = true
	}
	missing := make([]Coverage, 0)
	for _, value := range required {
		if !available[coverageKey(value)] {
			missing = append(missing, value)
		}
	}
	return missing
}

func statusRank(status string) int {
	switch status {
	case StatusPromoted:
		return 4
	case StatusIssued:
		return 3
	case StatusStale:
		return 2
	case StatusRevoked:
		return 1
	default:
		return 0
	}
}
