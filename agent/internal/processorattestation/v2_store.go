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

const FileNameV2 = "processor_control_attestations.v2.json"

type StoreV2 struct {
	Path string
	now  func() time.Time
	mu   sync.Mutex
}

func DefaultPathV2() (string, error) {
	if override := strings.TrimSpace(os.Getenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH")); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("processor attestation v2: user home: %w", err)
	}
	return filepath.Join(home, ".vit", FileNameV2), nil
}

func NewStoreV2(path string) (*StoreV2, error) {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultPathV2()
		if err != nil {
			return nil, err
		}
	}
	return &StoreV2{Path: filepath.Clean(path), now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *StoreV2) Read() (LibraryV2, ReadReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readUnlocked()
}

func (s *StoreV2) Issue(spec IssueSpecV2) (AttestationV2, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	library, _, err := s.readUnlocked()
	if err != nil {
		return AttestationV2{}, err
	}
	attestation, err := NewAttestationV2(spec, s.now())
	if err != nil {
		return AttestationV2{}, err
	}
	for _, existing := range library.Attestations {
		if existing.AttestationID == attestation.AttestationID {
			return existing, nil
		}
	}
	library.Attestations = append(library.Attestations, attestation)
	sortV2Attestations(library.Attestations)
	if err := s.writeUnlocked(&library); err != nil {
		return AttestationV2{}, err
	}
	return attestation, nil
}

func (s *StoreV2) Promote(attestationID, reason string) (AttestationV2, error) {
	return s.transition(attestationID, StatusPromoted, reason)
}
func (s *StoreV2) MarkStale(attestationID, reason string) (AttestationV2, error) {
	return s.transition(attestationID, StatusStale, reason)
}
func (s *StoreV2) Revoke(attestationID, reason string) (AttestationV2, error) {
	return s.transition(attestationID, StatusRevoked, reason)
}

func (s *StoreV2) PromoteCurrent(spec IssueSpecV2, reason string) (AttestationV2, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return AttestationV2{}, fmt.Errorf("processor attestation v2: promotion reason is required")
	}
	library, _, err := s.readUnlocked()
	if err != nil {
		return AttestationV2{}, err
	}
	candidate, err := NewAttestationV2(spec, s.now())
	if err != nil {
		return AttestationV2{}, err
	}
	index := -1
	changed := false
	for i := range library.Attestations {
		if library.Attestations[i].AttestationID == candidate.AttestationID {
			index = i
			break
		}
	}
	if index < 0 {
		library.Attestations = append(library.Attestations, candidate)
		index = len(library.Attestations) - 1
		changed = true
	}
	current := &library.Attestations[index]
	if current.Status != StatusIssued && current.Status != StatusPromoted {
		return AttestationV2{}, fmt.Errorf("processor attestation v2: cannot promote %s attestation", current.Status)
	}
	now := s.now().UTC()
	for i := range library.Attestations {
		other := &library.Attestations[i]
		if i == index || other.Status != StatusPromoted || other.Subject.SubjectKey != current.Subject.SubjectKey || other.ProcessorFamily != current.ProcessorFamily {
			continue
		}
		other.Status, other.StatusReason, other.StaleAt = StatusStale, "superseded_evidence", &now
		if other.BinaryFingerprint != current.BinaryFingerprint {
			other.StatusReason = "binary_fingerprint_superseded"
		}
		changed = true
	}
	if current.Status == StatusIssued {
		current.Status, current.StatusReason, current.PromotedAt = StatusPromoted, reason, &now
		changed = true
	}
	if !changed {
		return *current, nil
	}
	sortV2Attestations(library.Attestations)
	if err := s.writeUnlocked(&library); err != nil {
		return AttestationV2{}, err
	}
	for _, item := range library.Attestations {
		if item.AttestationID == candidate.AttestationID {
			return item, nil
		}
	}
	return AttestationV2{}, fmt.Errorf("processor attestation v2: promoted record disappeared")
}

func (s *StoreV2) Query(query QueryV2) (QueryResultV2, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	library, _, err := s.readUnlocked()
	if err != nil {
		return QueryResultV2{}, err
	}
	return QueryLibraryV2(library, query)
}

func QueryLibraryV2(library LibraryV2, query QueryV2) (QueryResultV2, error) {
	if err := library.Validate(); err != nil {
		return QueryResultV2{}, err
	}
	query.SubjectKey = strings.ToLower(strings.TrimSpace(query.SubjectKey))
	query.BinaryFingerprint = strings.ToLower(strings.TrimSpace(query.BinaryFingerprint))
	query.ProcessorFamily = strings.ToLower(strings.TrimSpace(query.ProcessorFamily))
	query.RequiredCoverage = normalizeCoverage(query.RequiredCoverage)
	if query.SubjectKey == "" || !validSHA256(query.BinaryFingerprint) {
		return QueryResultV2{}, fmt.Errorf("processor attestation v2: query requires subject_key and sha256 binary fingerprint")
	}
	if !IsV2Family(query.ProcessorFamily) {
		return QueryResultV2{}, fmt.Errorf("processor attestation v2: query has unsupported processor family")
	}
	if len(query.RequiredCoverage) == 0 {
		return QueryResultV2{}, fmt.Errorf("processor attestation v2: query requires current action coverage")
	}
	for _, coverage := range query.RequiredCoverage {
		if err := ValidateV2Coverage(query.ProcessorFamily, coverage); err != nil {
			return QueryResultV2{}, fmt.Errorf("processor attestation v2: invalid query coverage: %w", err)
		}
	}
	var best *AttestationV2
	for index := range library.Attestations {
		candidate := &library.Attestations[index]
		if candidate.Subject.SubjectKey != query.SubjectKey || candidate.ProcessorFamily != query.ProcessorFamily {
			continue
		}
		candidateMatch := candidate.BinaryFingerprint == query.BinaryFingerprint
		bestMatch := best != nil && best.BinaryFingerprint == query.BinaryFingerprint
		if best == nil || (candidateMatch && !bestMatch) || (candidateMatch == bestMatch && statusRank(candidate.Status) > statusRank(best.Status)) || (candidateMatch == bestMatch && statusRank(candidate.Status) == statusRank(best.Status) && candidate.AttestationID < best.AttestationID) {
			best = candidate
		}
	}
	if best == nil {
		return QueryResultV2{EffectiveStatus: "missing", Reason: "no_attestation", MissingCoverage: query.RequiredCoverage}, nil
	}
	result := QueryResultV2{Attestation: *best, EffectiveStatus: best.Status, Reason: "attestation_not_promoted", FingerprintMatch: best.BinaryFingerprint == query.BinaryFingerprint}
	if !result.FingerprintMatch {
		result.EffectiveStatus, result.Reason, result.MissingCoverage = StatusStale, "binary_fingerprint_changed", query.RequiredCoverage
		return result, nil
	}
	if best.Status != StatusPromoted {
		result.Reason, result.MissingCoverage = "attestation_"+best.Status, query.RequiredCoverage
		return result, nil
	}
	result.MissingCoverage = missingCoverage(best.Coverage, query.RequiredCoverage)
	if len(result.MissingCoverage) > 0 {
		result.Reason = "required_action_not_covered"
		return result, nil
	}
	result.Eligible, result.Reason = true, "promoted_action_covered"
	return result, nil
}

// QueryLibraryAdmission is the v2 family-level equivalent of PCA admission.
// It never treats a historical action axis as the authority for a new action.
func QueryLibraryAdmissionV2(library LibraryV2, subjectKey, binaryFingerprint, processorFamily string) (QueryResultV2, error) {
	if err := library.Validate(); err != nil {
		return QueryResultV2{}, err
	}
	subjectKey = strings.ToLower(strings.TrimSpace(subjectKey))
	binaryFingerprint = strings.ToLower(strings.TrimSpace(binaryFingerprint))
	processorFamily = strings.ToLower(strings.TrimSpace(processorFamily))
	if subjectKey == "" || !validSHA256(binaryFingerprint) {
		return QueryResultV2{}, fmt.Errorf("processor attestation v2: admission requires subject_key and sha256 binary fingerprint")
	}
	if !IsV2Family(processorFamily) {
		return QueryResultV2{}, fmt.Errorf("processor attestation v2: admission has unsupported processor family")
	}
	var best *AttestationV2
	for index := range library.Attestations {
		candidate := &library.Attestations[index]
		if candidate.Subject.SubjectKey != subjectKey || candidate.ProcessorFamily != processorFamily {
			continue
		}
		candidateMatch := candidate.BinaryFingerprint == binaryFingerprint
		bestMatch := best != nil && best.BinaryFingerprint == binaryFingerprint
		if best == nil || (candidateMatch && !bestMatch) || (candidateMatch == bestMatch && statusRank(candidate.Status) > statusRank(best.Status)) || (candidateMatch == bestMatch && statusRank(candidate.Status) == statusRank(best.Status) && candidate.AttestationID < best.AttestationID) {
			best = candidate
		}
	}
	if best == nil {
		return QueryResultV2{EffectiveStatus: "missing", Reason: "no_attestation"}, nil
	}
	result := QueryResultV2{Attestation: *best, EffectiveStatus: best.Status, Reason: "attestation_not_promoted", FingerprintMatch: best.BinaryFingerprint == binaryFingerprint}
	if !result.FingerprintMatch {
		result.EffectiveStatus, result.Reason = StatusStale, "binary_fingerprint_changed"
		return result, nil
	}
	if best.Status != StatusPromoted {
		result.Reason = "attestation_" + best.Status
		return result, nil
	}
	result.Eligible, result.Reason = true, "promoted_binary_admitted"
	return result, nil
}

func (s *StoreV2) transition(attestationID, nextStatus, reason string) (AttestationV2, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	library, _, err := s.readUnlocked()
	if err != nil {
		return AttestationV2{}, err
	}
	attestationID, reason = strings.TrimSpace(attestationID), strings.TrimSpace(reason)
	if reason == "" {
		return AttestationV2{}, fmt.Errorf("processor attestation v2: transition reason is required")
	}
	for index := range library.Attestations {
		item := &library.Attestations[index]
		if item.AttestationID != attestationID {
			continue
		}
		if item.Status == nextStatus {
			return *item, nil
		}
		if err := allowedTransition(item.Status, nextStatus); err != nil {
			return AttestationV2{}, err
		}
		now := s.now().UTC()
		item.Status, item.StatusReason = nextStatus, reason
		switch nextStatus {
		case StatusPromoted:
			item.PromotedAt = &now
		case StatusStale:
			item.StaleAt = &now
		case StatusRevoked:
			item.RevokedAt = &now
		}
		if err := s.writeUnlocked(&library); err != nil {
			return AttestationV2{}, err
		}
		return *item, nil
	}
	return AttestationV2{}, fmt.Errorf("processor attestation v2: attestation %q not found", attestationID)
}

func (s *StoreV2) readUnlocked() (LibraryV2, ReadReport, error) {
	report := ReadReport{Path: s.Path}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return emptyLibraryV2(), report, nil
	}
	if err == nil {
		library, decodeErr := decodeLibraryV2(data)
		if decodeErr == nil {
			return library, report, nil
		}
		err = decodeErr
	}
	report.PrimaryError = err.Error()
	backupData, backupErr := os.ReadFile(s.Path + ".bak")
	if backupErr != nil {
		return LibraryV2{}, report, fmt.Errorf("processor attestation v2: read primary: %v; read backup: %w", err, backupErr)
	}
	library, backupErr := decodeLibraryV2(backupData)
	if backupErr != nil {
		return LibraryV2{}, report, fmt.Errorf("processor attestation v2: read primary: %v; decode backup: %w", err, backupErr)
	}
	report.RecoveredFromBackup = true
	return library, report, nil
}

func (s *StoreV2) writeUnlocked(library *LibraryV2) error {
	library.SchemaVersion = LibrarySchemaV2
	library.Revision++
	library.UpdatedAt = s.now().UTC()
	if err := library.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(library, "", "  ")
	if err != nil {
		return fmt.Errorf("processor attestation v2: marshal library: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("processor attestation v2: create store directory: %w", err)
	}
	if current, readErr := os.ReadFile(s.Path); readErr == nil {
		if _, decodeErr := decodeLibraryV2(current); decodeErr == nil {
			if err := replaceFile(s.Path+".bak", current); err != nil {
				return fmt.Errorf("processor attestation v2: write backup: %w", err)
			}
		}
	}
	if err := replaceFile(s.Path, data); err != nil {
		return fmt.Errorf("processor attestation v2: replace store: %w", err)
	}
	return nil
}

func decodeLibraryV2(data []byte) (LibraryV2, error) {
	var library LibraryV2
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&library); err != nil {
		return LibraryV2{}, fmt.Errorf("decode v2 library: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return LibraryV2{}, fmt.Errorf("decode v2 library: trailing JSON content")
	}
	if err := library.Validate(); err != nil {
		return LibraryV2{}, err
	}
	return library, nil
}
func emptyLibraryV2() LibraryV2 {
	return LibraryV2{SchemaVersion: LibrarySchemaV2, Attestations: []AttestationV2{}}
}
func sortV2Attestations(values []AttestationV2) {
	sort.Slice(values, func(i, j int) bool { return values[i].AttestationID < values[j].AttestationID })
}
