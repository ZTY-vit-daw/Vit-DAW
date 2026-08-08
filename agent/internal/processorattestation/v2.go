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

// PCA v2 extends the action-only admission vocabulary without changing the
// v1 EQ/compressor schema or its validation rules.
const (
	LibrarySchemaV2     = "processor_control_attestations.v2"
	AttestationSchemaV2 = "processor_control_attestation.v2"
	FamilyLimiter       = "limiter"
	FamilyGateExpander  = "gate_expander"
	FamilyDeEsser       = "de_esser"
	FamilyTransient     = "transient_shaper"
	FamilyMultiband     = "multiband_dynamics"
)

type LibraryV2 struct {
	SchemaVersion string          `json:"schema_version"`
	Revision      uint64          `json:"revision"`
	UpdatedAt     time.Time       `json:"updated_at"`
	Attestations  []AttestationV2 `json:"attestations"`
}

type AttestationV2 struct {
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

type IssueSpecV2 struct {
	Subject           Subject
	BinaryFingerprint string
	ProcessorFamily   string
	Coverage          []Coverage
	Evidence          []EvidenceRef
}

type QueryV2 struct {
	SubjectKey        string
	BinaryFingerprint string
	ProcessorFamily   string
	RequiredCoverage  []Coverage
}

type QueryResultV2 struct {
	Eligible         bool          `json:"eligible"`
	EffectiveStatus  string        `json:"effective_status"`
	Reason           string        `json:"reason"`
	Attestation      AttestationV2 `json:"attestation,omitempty"`
	MissingCoverage  []Coverage    `json:"missing_coverage,omitempty"`
	FingerprintMatch bool          `json:"fingerprint_match"`
}

var v2CoverageAxes = map[string][]string{
	FamilyLimiter:      {"detector_latency", "input_drive", "output_ceiling", "output_normalization", "peak_mode", "protection_intensity", "recovery_motion"},
	FamilyGateExpander: {"activation_threshold", "attenuation_floor", "detector_focus", "direction_mode", "output_normalization", "parallel_balance", "state_timing"},
	FamilyDeEsser:      {"detector_focus", "output_normalization", "parallel_balance", "recovery_motion", "sibilance_reduction", "split_scope", "threshold_sensitivity"},
	FamilyTransient:    {"detector_focus", "envelope_emphasis", "envelope_timing", "output_normalization", "parallel_balance", "shape_mode"},
	FamilyMultiband:    {"band_dynamics", "band_timing", "crossover_layout", "detector_focus", "output_normalization", "parallel_balance"},
}

// V2CoverageAxes returns a sorted copy of the public action-only vocabulary.
func V2CoverageAxes(family string) []string {
	values := append([]string(nil), v2CoverageAxes[strings.ToLower(strings.TrimSpace(family))]...)
	sort.Strings(values)
	return values
}

func IsV2Family(family string) bool {
	_, ok := v2CoverageAxes[strings.ToLower(strings.TrimSpace(family))]
	return ok
}

func ValidateV2Coverage(family string, coverage Coverage) error {
	family = strings.ToLower(strings.TrimSpace(family))
	if !IsV2Family(family) {
		return fmt.Errorf("processor attestation v2: unsupported processor family %q", family)
	}
	if strings.ToLower(strings.TrimSpace(coverage.Action)) != "adjust" || strings.TrimSpace(coverage.Shape) != "" {
		return fmt.Errorf("processor attestation v2: %s coverage requires action=adjust and no shape", family)
	}
	axis := strings.ToLower(strings.TrimSpace(coverage.Axis))
	for _, allowed := range v2CoverageAxes[family] {
		if axis == allowed {
			return nil
		}
	}
	return fmt.Errorf("processor attestation v2: unsupported %s action axis %q", family, coverage.Axis)
}

// V2CoverageForRoles maps observed typed topology roles to stable semantic
// axes. It never returns parameter IDs, normalized values, or product names.
func V2CoverageForRoles(family string, roles []string) []Coverage {
	family = strings.ToLower(strings.TrimSpace(family))
	seen := map[string]bool{}
	result := make([]Coverage, 0, len(roles))
	for _, raw := range roles {
		role := strings.ToLower(strings.TrimSpace(raw))
		axis := v2RoleAxis(family, role)
		if axis == "" || seen[axis] {
			continue
		}
		seen[axis] = true
		result = append(result, Coverage{Action: "adjust", Axis: axis})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Axis < result[j].Axis })
	return result
}

func v2RoleAxis(family, role string) string {
	contains := func(values ...string) bool {
		for _, value := range values {
			if role == value {
				return true
			}
		}
		return false
	}
	switch family {
	case FamilyLimiter:
		switch {
		case contains("threshold", "reduction_range"):
			return "protection_intensity"
		case contains("ceiling"):
			return "output_ceiling"
		case contains("release", "hold"):
			return "recovery_motion"
		case contains("lookahead"):
			return "detector_latency"
		case contains("input_drive"):
			return "input_drive"
		case contains("output_gain"):
			return "output_normalization"
		case contains("true_peak", "oversampling"):
			return "peak_mode"
		}
	case FamilyGateExpander:
		switch {
		case contains("threshold", "hysteresis"):
			return "activation_threshold"
		case contains("range", "attenuation_range"):
			return "attenuation_floor"
		case contains("attack", "hold", "release", "cycle_delay"):
			return "state_timing"
		case contains("sidechain_highpass", "sidechain_lowpass", "focus_frequency"):
			return "detector_focus"
		case contains("direction_mode"):
			return "direction_mode"
		case contains("output_gain"):
			return "output_normalization"
		case contains("mix"):
			return "parallel_balance"
		}
	case FamilyDeEsser:
		switch {
		case contains("threshold"):
			return "threshold_sensitivity"
		case contains("reduction_range", "detection_amount"):
			return "sibilance_reduction"
		case contains("focus_frequency", "detector_filter"):
			return "detector_focus"
		case contains("attack", "release", "lookahead"):
			return "recovery_motion"
		case contains("split_wide"):
			return "split_scope"
		case contains("output_gain"):
			return "output_normalization"
		case contains("mix"):
			return "parallel_balance"
		}
	case FamilyTransient:
		switch {
		case contains("attack_amount", "sustain_amount", "transient_range"):
			return "envelope_emphasis"
		case contains("attack_duration", "sustain_duration", "duration", "release"):
			return "envelope_timing"
		case contains("focus_frequency"):
			return "detector_focus"
		case contains("processing_mode"):
			return "shape_mode"
		case contains("output_gain"):
			return "output_normalization"
		case contains("mix"):
			return "parallel_balance"
		}
	case FamilyMultiband:
		switch {
		case contains("crossover"):
			return "crossover_layout"
		case contains("threshold", "range", "ratio", "gain", "amount"):
			return "band_dynamics"
		case contains("attack", "hold", "release", "lookahead"):
			return "band_timing"
		case contains("focus_frequency", "sidechain_highpass", "sidechain_lowpass"):
			return "detector_focus"
		case contains("output_gain"):
			return "output_normalization"
		case contains("mix"):
			return "parallel_balance"
		}
	}
	return ""
}

func NewAttestationV2(spec IssueSpecV2, now time.Time) (AttestationV2, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	spec.Subject = normalizeSubject(spec.Subject)
	key, err := BuildSubjectKey(spec.Subject)
	if err != nil {
		return AttestationV2{}, err
	}
	if spec.Subject.SubjectKey != "" && spec.Subject.SubjectKey != key {
		return AttestationV2{}, fmt.Errorf("processor attestation v2: subject_key does not match stable identity")
	}
	spec.Subject.SubjectKey = key
	spec.BinaryFingerprint = strings.ToLower(strings.TrimSpace(spec.BinaryFingerprint))
	spec.ProcessorFamily = strings.ToLower(strings.TrimSpace(spec.ProcessorFamily))
	spec.Coverage = normalizeCoverage(spec.Coverage)
	spec.Evidence = normalizeEvidence(spec.Evidence)
	if err := validateIssueSpecV2(spec); err != nil {
		return AttestationV2{}, err
	}
	payload := struct {
		SchemaVersion     string        `json:"schema_version"`
		Issuer            string        `json:"issuer"`
		Subject           Subject       `json:"subject"`
		BinaryFingerprint string        `json:"binary_fingerprint"`
		ProcessorFamily   string        `json:"processor_family"`
		Coverage          []Coverage    `json:"coverage"`
		Evidence          []EvidenceRef `json:"evidence"`
	}{AttestationSchemaV2, Issuer, spec.Subject, spec.BinaryFingerprint, spec.ProcessorFamily, spec.Coverage, spec.Evidence}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return AttestationV2{}, fmt.Errorf("processor attestation v2: encode immutable payload: %w", err)
	}
	digest := sha256.Sum256(encoded)
	digestText := hex.EncodeToString(digest[:])
	return AttestationV2{SchemaVersion: AttestationSchemaV2, AttestationID: "pca2_" + digestText[:24], PayloadDigest: "sha256:" + digestText, Issuer: Issuer, Subject: spec.Subject, BinaryFingerprint: spec.BinaryFingerprint, ProcessorFamily: spec.ProcessorFamily, Coverage: spec.Coverage, Evidence: spec.Evidence, Status: StatusIssued, StatusReason: "deterministic_evidence_issued", IssuedAt: now}, nil
}

func (a AttestationV2) Validate() error {
	if a.SchemaVersion != AttestationSchemaV2 || a.Issuer != Issuer {
		return fmt.Errorf("processor attestation v2: unsupported schema or issuer")
	}
	rebuilt, err := NewAttestationV2(IssueSpecV2{Subject: a.Subject, BinaryFingerprint: a.BinaryFingerprint, ProcessorFamily: a.ProcessorFamily, Coverage: a.Coverage, Evidence: a.Evidence}, a.IssuedAt)
	if err != nil {
		return err
	}
	if a.AttestationID != rebuilt.AttestationID || a.PayloadDigest != rebuilt.PayloadDigest {
		return fmt.Errorf("processor attestation v2: immutable payload digest mismatch")
	}
	switch a.Status {
	case StatusIssued:
		if a.PromotedAt != nil || a.StaleAt != nil || a.RevokedAt != nil {
			return fmt.Errorf("processor attestation v2: issued state has terminal timestamps")
		}
	case StatusPromoted:
		if a.PromotedAt == nil || a.StaleAt != nil || a.RevokedAt != nil {
			return fmt.Errorf("processor attestation v2: promoted state timestamps are invalid")
		}
	case StatusStale:
		if a.StaleAt == nil || a.RevokedAt != nil {
			return fmt.Errorf("processor attestation v2: stale state timestamps are invalid")
		}
	case StatusRevoked:
		if a.RevokedAt == nil {
			return fmt.Errorf("processor attestation v2: revoked_at is required")
		}
	default:
		return fmt.Errorf("processor attestation v2: invalid status %q", a.Status)
	}
	return nil
}

func (l LibraryV2) Validate() error {
	if l.SchemaVersion != LibrarySchemaV2 {
		return fmt.Errorf("processor attestation v2: schema_version must be %s", LibrarySchemaV2)
	}
	seen := map[string]bool{}
	for index, attestation := range l.Attestations {
		if err := attestation.Validate(); err != nil {
			return fmt.Errorf("processor attestation v2: record %d: %w", index+1, err)
		}
		if seen[attestation.AttestationID] {
			return fmt.Errorf("processor attestation v2: duplicate attestation_id %s", attestation.AttestationID)
		}
		seen[attestation.AttestationID] = true
	}
	return nil
}

func validateIssueSpecV2(spec IssueSpecV2) error {
	key, err := BuildSubjectKey(spec.Subject)
	if err != nil {
		return err
	}
	if spec.Subject.SubjectKey != key {
		return fmt.Errorf("processor attestation v2: subject_key does not match stable identity")
	}
	if !validSHA256(spec.BinaryFingerprint) {
		return fmt.Errorf("processor attestation v2: binary_fingerprint must be sha256:<64 lowercase hex>")
	}
	if !IsV2Family(spec.ProcessorFamily) {
		return fmt.Errorf("processor attestation v2: unsupported processor family %q", spec.ProcessorFamily)
	}
	if len(spec.Coverage) == 0 {
		return fmt.Errorf("processor attestation v2: coverage is required")
	}
	for _, coverage := range spec.Coverage {
		if err := ValidateV2Coverage(spec.ProcessorFamily, coverage); err != nil {
			return err
		}
	}
	if len(spec.Evidence) == 0 {
		return fmt.Errorf("processor attestation v2: evidence is required")
	}
	for _, evidence := range spec.Evidence {
		if evidence.ReceiptID == "" || evidence.Kind == "" || !validSHA256(evidence.SHA256) || evidence.ObservedAt.IsZero() {
			return fmt.Errorf("processor attestation v2: evidence requires receipt_id, kind, sha256, and observed_at")
		}
	}
	return nil
}
