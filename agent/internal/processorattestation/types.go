package processorattestation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	LibrarySchema     = "processor_control_attestations.v1"
	AttestationSchema = "processor_control_attestation.v1"
	Issuer            = "vit_deterministic_local_v1"

	FamilyStaticEQ            = "static_eq"
	FamilyBroadbandCompressor = "broadband_compressor"

	StatusIssued   = "issued"
	StatusPromoted = "promoted"
	StatusStale    = "stale"
	StatusRevoked  = "revoked"
)

type Library struct {
	SchemaVersion string        `json:"schema_version"`
	Revision      uint64        `json:"revision"`
	UpdatedAt     time.Time     `json:"updated_at"`
	Attestations  []Attestation `json:"attestations"`
}

type Attestation struct {
	SchemaVersion     string        `json:"schema_version"`
	AttestationID     string        `json:"attestation_id"`
	PayloadDigest     string        `json:"payload_digest"`
	Issuer            string        `json:"issuer"`
	Subject           Subject       `json:"subject"`
	BinaryFingerprint string        `json:"binary_fingerprint"`
	ProcessorFamily   string        `json:"processor_family"`
	Coverage          []Coverage    `json:"coverage"`
	Evidence          []EvidenceRef `json:"evidence"`
	Status            string        `json:"status"`
	StatusReason      string        `json:"status_reason,omitempty"`
	IssuedAt          time.Time     `json:"issued_at"`
	PromotedAt        *time.Time    `json:"promoted_at,omitempty"`
	StaleAt           *time.Time    `json:"stale_at,omitempty"`
	RevokedAt         *time.Time    `json:"revoked_at,omitempty"`
}

type Subject struct {
	SubjectKey    string `json:"subject_key"`
	Name          string `json:"name"`
	Manufacturer  string `json:"manufacturer,omitempty"`
	Format        string `json:"format"`
	Identifier    string `json:"identifier,omitempty"`
	InstalledPath string `json:"installed_path,omitempty"`
}

type Coverage struct {
	Action string `json:"action"`
	Shape  string `json:"shape,omitempty"`
	Axis   string `json:"axis,omitempty"`
}

type EvidenceRef struct {
	ReceiptID    string    `json:"receipt_id"`
	Kind         string    `json:"kind"`
	SHA256       string    `json:"sha256"`
	ObservedAt   time.Time `json:"observed_at"`
	CorpusRecord string    `json:"corpus_record,omitempty"`
}

type IssueSpec struct {
	Subject           Subject
	BinaryFingerprint string
	ProcessorFamily   string
	Coverage          []Coverage
	Evidence          []EvidenceRef
}

type Query struct {
	SubjectKey        string
	BinaryFingerprint string
	ProcessorFamily   string
	RequiredCoverage  []Coverage
}

type QueryResult struct {
	Eligible         bool        `json:"eligible"`
	EffectiveStatus  string      `json:"effective_status"`
	Reason           string      `json:"reason"`
	Attestation      Attestation `json:"attestation,omitempty"`
	MissingCoverage  []Coverage  `json:"missing_coverage,omitempty"`
	FingerprintMatch bool        `json:"fingerprint_match"`
}

func NewAttestation(spec IssueSpec, now time.Time) (Attestation, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	spec.Subject = normalizeSubject(spec.Subject)
	key, err := BuildSubjectKey(spec.Subject)
	if err != nil {
		return Attestation{}, err
	}
	if spec.Subject.SubjectKey != "" && spec.Subject.SubjectKey != key {
		return Attestation{}, fmt.Errorf("processor attestation: subject_key does not match stable identity")
	}
	spec.Subject.SubjectKey = key
	spec.BinaryFingerprint = strings.ToLower(strings.TrimSpace(spec.BinaryFingerprint))
	spec.ProcessorFamily = strings.ToLower(strings.TrimSpace(spec.ProcessorFamily))
	spec.Coverage = normalizeCoverage(spec.Coverage)
	spec.Evidence = normalizeEvidence(spec.Evidence)
	if err := validateIssueSpec(spec); err != nil {
		return Attestation{}, err
	}
	payload := struct {
		SchemaVersion     string        `json:"schema_version"`
		Issuer            string        `json:"issuer"`
		Subject           Subject       `json:"subject"`
		BinaryFingerprint string        `json:"binary_fingerprint"`
		ProcessorFamily   string        `json:"processor_family"`
		Coverage          []Coverage    `json:"coverage"`
		Evidence          []EvidenceRef `json:"evidence"`
	}{AttestationSchema, Issuer, spec.Subject, spec.BinaryFingerprint, spec.ProcessorFamily, spec.Coverage, spec.Evidence}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Attestation{}, fmt.Errorf("processor attestation: encode immutable payload: %w", err)
	}
	digest := sha256.Sum256(encoded)
	digestText := hex.EncodeToString(digest[:])
	return Attestation{
		SchemaVersion: AttestationSchema, AttestationID: "pca1_" + digestText[:24], PayloadDigest: "sha256:" + digestText,
		Issuer: Issuer, Subject: spec.Subject, BinaryFingerprint: spec.BinaryFingerprint, ProcessorFamily: spec.ProcessorFamily,
		Coverage: spec.Coverage, Evidence: spec.Evidence, Status: StatusIssued, StatusReason: "deterministic_evidence_issued", IssuedAt: now,
	}, nil
}

func BuildSubjectKey(subject Subject) (string, error) {
	subject = normalizeSubject(subject)
	if subject.Name == "" || subject.Format == "" {
		return "", fmt.Errorf("processor attestation: subject name and format are required")
	}
	identity := strings.ToLower(subject.Format) + "\x00"
	if subject.Identifier != "" {
		identity += "identifier\x00" + strings.ToLower(subject.Identifier)
	} else {
		if subject.InstalledPath == "" {
			return "", fmt.Errorf("processor attestation: subject identifier or installed path is required")
		}
		identity += "path\x00" + strings.ToLower(strings.ReplaceAll(subject.InstalledPath, "\\", "/")) + "\x00" +
			strings.ToLower(subject.Manufacturer) + "\x00" + strings.ToLower(subject.Name)
	}
	digest := sha256.Sum256([]byte(identity))
	return "pcs1_" + hex.EncodeToString(digest[:12]), nil
}

func (a Attestation) Validate() error {
	if a.SchemaVersion != AttestationSchema || a.Issuer != Issuer {
		return fmt.Errorf("processor attestation: unsupported attestation schema or issuer")
	}
	rebuilt, err := NewAttestation(IssueSpec{Subject: a.Subject, BinaryFingerprint: a.BinaryFingerprint,
		ProcessorFamily: a.ProcessorFamily, Coverage: a.Coverage, Evidence: a.Evidence}, a.IssuedAt)
	if err != nil {
		return err
	}
	if a.AttestationID != rebuilt.AttestationID || a.PayloadDigest != rebuilt.PayloadDigest {
		return fmt.Errorf("processor attestation: immutable payload digest mismatch")
	}
	switch a.Status {
	case StatusIssued:
		if a.PromotedAt != nil || a.StaleAt != nil || a.RevokedAt != nil {
			return fmt.Errorf("processor attestation: issued state has terminal timestamps")
		}
	case StatusPromoted:
		if a.PromotedAt == nil || a.StaleAt != nil || a.RevokedAt != nil {
			return fmt.Errorf("processor attestation: promoted state timestamps are invalid")
		}
	case StatusStale:
		if a.StaleAt == nil || a.RevokedAt != nil {
			return fmt.Errorf("processor attestation: stale state timestamps are invalid")
		}
	case StatusRevoked:
		if a.RevokedAt == nil {
			return fmt.Errorf("processor attestation: revoked_at is required")
		}
	default:
		return fmt.Errorf("processor attestation: invalid status %q", a.Status)
	}
	return nil
}

func (l Library) Validate() error {
	if l.SchemaVersion != LibrarySchema {
		return fmt.Errorf("processor attestation: schema_version must be %s", LibrarySchema)
	}
	seen := map[string]bool{}
	for index, attestation := range l.Attestations {
		if err := attestation.Validate(); err != nil {
			return fmt.Errorf("processor attestation: record %d: %w", index+1, err)
		}
		if seen[attestation.AttestationID] {
			return fmt.Errorf("processor attestation: duplicate attestation_id %s", attestation.AttestationID)
		}
		seen[attestation.AttestationID] = true
	}
	return nil
}

func validateIssueSpec(spec IssueSpec) error {
	key, err := BuildSubjectKey(spec.Subject)
	if err != nil {
		return err
	}
	if spec.Subject.SubjectKey != key {
		return fmt.Errorf("processor attestation: subject_key does not match stable identity")
	}
	if !validSHA256(spec.BinaryFingerprint) {
		return fmt.Errorf("processor attestation: binary_fingerprint must be sha256:<64 lowercase hex>")
	}
	switch spec.ProcessorFamily {
	case FamilyStaticEQ, FamilyBroadbandCompressor:
	default:
		return fmt.Errorf("processor attestation: unsupported processor family %q", spec.ProcessorFamily)
	}
	if len(spec.Coverage) == 0 {
		return fmt.Errorf("processor attestation: coverage is required")
	}
	for _, coverage := range spec.Coverage {
		if err := validateCoverage(spec.ProcessorFamily, coverage); err != nil {
			return err
		}
	}
	if len(spec.Evidence) == 0 {
		return fmt.Errorf("processor attestation: evidence is required")
	}
	for _, evidence := range spec.Evidence {
		if evidence.ReceiptID == "" || evidence.Kind == "" || !validSHA256(evidence.SHA256) || evidence.ObservedAt.IsZero() {
			return fmt.Errorf("processor attestation: evidence requires receipt_id, kind, sha256, and observed_at")
		}
	}
	return nil
}

func validateCoverage(family string, coverage Coverage) error {
	if family == FamilyStaticEQ {
		switch coverage.Action {
		case "upsert", "modify", "disable", "remove", "undo":
		default:
			return fmt.Errorf("processor attestation: unsupported static EQ action %q", coverage.Action)
		}
		switch coverage.Shape {
		case "bell", "low_shelf", "high_shelf", "low_cut", "high_cut":
		default:
			return fmt.Errorf("processor attestation: static EQ coverage requires a supported shape")
		}
		if coverage.Axis != "" {
			return fmt.Errorf("processor attestation: static EQ coverage cannot contain an axis")
		}
		return nil
	}
	if coverage.Action != "adjust" || coverage.Shape != "" {
		return fmt.Errorf("processor attestation: compressor coverage requires action=adjust and no shape")
	}
	for _, allowed := range []string{"activation_intensity", "transfer_severity", "transient_timing", "recovery_motion", "detector_focus", "output_normalization", "parallel_balance", "character"} {
		if coverage.Axis == allowed {
			return nil
		}
	}
	return fmt.Errorf("processor attestation: unsupported compressor axis %q", coverage.Axis)
}

// ValidateCoverage validates the public action-only coverage vocabulary.
func ValidateCoverage(family string, coverage Coverage) error {
	return validateCoverage(strings.ToLower(strings.TrimSpace(family)), Coverage{
		Action: strings.ToLower(strings.TrimSpace(coverage.Action)),
		Shape:  strings.ToLower(strings.TrimSpace(coverage.Shape)),
		Axis:   strings.ToLower(strings.TrimSpace(coverage.Axis)),
	})
}

func normalizeSubject(subject Subject) Subject {
	subject.SubjectKey = strings.ToLower(strings.TrimSpace(subject.SubjectKey))
	subject.Name = strings.TrimSpace(subject.Name)
	subject.Manufacturer = strings.TrimSpace(subject.Manufacturer)
	subject.Format = strings.ToUpper(strings.TrimSpace(subject.Format))
	subject.Identifier = strings.TrimSpace(subject.Identifier)
	subject.InstalledPath = strings.TrimSpace(subject.InstalledPath)
	return subject
}

func normalizeCoverage(values []Coverage) []Coverage {
	seen := map[string]bool{}
	out := make([]Coverage, 0, len(values))
	for _, value := range values {
		value.Action = strings.ToLower(strings.TrimSpace(value.Action))
		value.Shape = strings.ToLower(strings.TrimSpace(value.Shape))
		value.Axis = strings.ToLower(strings.TrimSpace(value.Axis))
		key := coverageKey(value)
		if !seen[key] {
			seen[key] = true
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return coverageKey(out[i]) < coverageKey(out[j]) })
	return out
}

func normalizeEvidence(values []EvidenceRef) []EvidenceRef {
	seen := map[string]bool{}
	out := make([]EvidenceRef, 0, len(values))
	for _, value := range values {
		value.ReceiptID = strings.TrimSpace(value.ReceiptID)
		value.Kind = strings.ToLower(strings.TrimSpace(value.Kind))
		value.SHA256 = strings.ToLower(strings.TrimSpace(value.SHA256))
		value.CorpusRecord = strings.TrimSpace(value.CorpusRecord)
		value.ObservedAt = value.ObservedAt.UTC()
		key := value.ReceiptID + "\x00" + value.SHA256
		if !seen[key] {
			seen[key] = true
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ReceiptID+"\x00"+out[i].SHA256 < out[j].ReceiptID+"\x00"+out[j].SHA256
	})
	return out
}

func coverageKey(value Coverage) string {
	return value.Action + "\x00" + value.Shape + "\x00" + value.Axis
}

func validSHA256(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && value == strings.ToLower(value)
}
