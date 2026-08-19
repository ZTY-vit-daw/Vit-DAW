package processorattestation

import (
	"fmt"
	"strings"
)

// EligibilityRequirement is the deterministic admission contract shared by
// recommendation and execution. Coverage is action-only and family-bound.
type EligibilityRequirement struct {
	ProcessorFamily  string     `json:"processor_family"`
	RequiredCoverage []Coverage `json:"required_coverage"`
}

// InstalledSubject binds an eligibility decision to one exact installed
// binary. Identifier is mandatory for Agent admission; names are descriptive
// metadata only.
type InstalledSubject struct {
	Subject
}

type EligibilityResult struct {
	Eligible          bool       `json:"eligible"`
	EffectiveStatus   string     `json:"effective_status"`
	Reason            string     `json:"reason"`
	SubjectKey        string     `json:"subject_key,omitempty"`
	BinaryFingerprint string     `json:"binary_fingerprint,omitempty"`
	FingerprintMatch  bool       `json:"fingerprint_match"`
	AttestationID     string     `json:"attestation_id,omitempty"`
	MissingCoverage   []Coverage `json:"missing_coverage,omitempty"`
}

// QueryInstalled computes the current binary fingerprint and reads the
// authoritative PCA store on every call. It intentionally has no product-name
// or category fallback.
func QueryInstalled(subject InstalledSubject, requirement EligibilityRequirement) (EligibilityResult, error) {
	requirement.ProcessorFamily = strings.ToLower(strings.TrimSpace(requirement.ProcessorFamily))
	subject.Subject = normalizeSubject(subject.Subject)
	if subject.Identifier == "" {
		return EligibilityResult{EffectiveStatus: "missing", Reason: "exact_identifier_required"}, nil
	}
	if subject.InstalledPath == "" {
		return EligibilityResult{EffectiveStatus: "missing", Reason: "installed_binary_path_required"}, nil
	}
	key, err := BuildSubjectKey(subject.Subject)
	if err != nil {
		return EligibilityResult{EffectiveStatus: "missing", Reason: "stable_identity_unavailable"}, nil
	}
	fingerprint, err := FingerprintPath(subject.InstalledPath)
	if err != nil {
		return EligibilityResult{SubjectKey: key, EffectiveStatus: "stale", Reason: "binary_fingerprint_unavailable"}, nil
	}
	result, err := QueryCurrent(key, fingerprint, requirement)
	if err != nil {
		return EligibilityResult{}, err
	}
	result.SubjectKey = key
	result.BinaryFingerprint = fingerprint
	if result.AttestationID != "" {
		return result, nil
	}
	return result, nil
}

// QueryInstalledAdmission checks whether one exact installed binary is
// promoted for a processor family. It is intentionally independent of the
// action coverage retained in the attestation evidence.
func QueryInstalledAdmission(subject InstalledSubject, processorFamily string) (EligibilityResult, error) {
	subject.Subject = normalizeSubject(subject.Subject)
	if subject.Identifier == "" {
		return EligibilityResult{EffectiveStatus: "missing", Reason: "exact_identifier_required"}, nil
	}
	if subject.InstalledPath == "" {
		return EligibilityResult{EffectiveStatus: "missing", Reason: "installed_binary_path_required"}, nil
	}
	key, err := BuildSubjectKey(subject.Subject)
	if err != nil {
		return EligibilityResult{EffectiveStatus: "missing", Reason: "stable_identity_unavailable"}, nil
	}
	fingerprint, err := FingerprintPath(subject.InstalledPath)
	if err != nil {
		return EligibilityResult{SubjectKey: key, EffectiveStatus: "stale", Reason: "binary_fingerprint_unavailable"}, nil
	}
	return QueryCurrentAdmission(key, fingerprint, processorFamily)
}

// QueryCurrent dispatches to the PCA v1 or v2 store using one normalized
// requirement. Unsupported families fail closed.
func QueryCurrent(subjectKey, fingerprint string, requirement EligibilityRequirement) (EligibilityResult, error) {
	family := strings.ToLower(strings.TrimSpace(requirement.ProcessorFamily))
	coverage := normalizeCoverage(requirement.RequiredCoverage)
	if subjectKey == "" || !validSHA256(strings.ToLower(strings.TrimSpace(fingerprint))) {
		return EligibilityResult{EffectiveStatus: "missing", Reason: "invalid_current_binary_identity"}, nil
	}
	if family == FamilyStaticEQ || family == FamilyBroadbandCompressor {
		store, err := NewStore("")
		if err != nil {
			return EligibilityResult{}, err
		}
		library, _, err := store.Read()
		if err != nil {
			return EligibilityResult{}, err
		}
		result, err := QueryLibrary(library, Query{SubjectKey: subjectKey, BinaryFingerprint: fingerprint, ProcessorFamily: family, RequiredCoverage: coverage})
		if err != nil {
			return EligibilityResult{}, err
		}
		return EligibilityResult{Eligible: result.Eligible, EffectiveStatus: result.EffectiveStatus, Reason: result.Reason, SubjectKey: subjectKey, BinaryFingerprint: fingerprint, FingerprintMatch: result.FingerprintMatch, AttestationID: result.Attestation.AttestationID, MissingCoverage: result.MissingCoverage}, nil
	}
	if IsV2Family(family) {
		store, err := NewStoreV2("")
		if err != nil {
			return EligibilityResult{}, err
		}
		library, _, err := store.Read()
		if err != nil {
			return EligibilityResult{}, err
		}
		result, err := QueryLibraryV2(library, QueryV2{SubjectKey: subjectKey, BinaryFingerprint: fingerprint, ProcessorFamily: family, RequiredCoverage: coverage})
		if err != nil {
			return EligibilityResult{}, err
		}
		return EligibilityResult{Eligible: result.Eligible, EffectiveStatus: result.EffectiveStatus, Reason: result.Reason, SubjectKey: subjectKey, BinaryFingerprint: fingerprint, FingerprintMatch: result.FingerprintMatch, AttestationID: result.Attestation.AttestationID, MissingCoverage: result.MissingCoverage}, nil
	}
	return EligibilityResult{EffectiveStatus: "unsupported", Reason: fmt.Sprintf("unsupported_processor_family:%s", family)}, nil
}

// QueryCurrentAdmission is the authoritative family-level admission query
// used before loading and after live controller discovery.
func QueryCurrentAdmission(subjectKey, fingerprint, processorFamily string) (EligibilityResult, error) {
	family := strings.ToLower(strings.TrimSpace(processorFamily))
	if subjectKey == "" || !validSHA256(strings.ToLower(strings.TrimSpace(fingerprint))) {
		return EligibilityResult{EffectiveStatus: "missing", Reason: "invalid_current_binary_identity"}, nil
	}
	if family == FamilyStaticEQ || family == FamilyBroadbandCompressor {
		store, err := NewStore("")
		if err != nil {
			return EligibilityResult{}, err
		}
		library, _, err := store.Read()
		if err != nil {
			return EligibilityResult{}, err
		}
		result, err := QueryLibraryAdmission(library, subjectKey, fingerprint, family)
		if err != nil {
			return EligibilityResult{}, err
		}
		return EligibilityResult{Eligible: result.Eligible, EffectiveStatus: result.EffectiveStatus, Reason: result.Reason, SubjectKey: subjectKey, BinaryFingerprint: fingerprint, FingerprintMatch: result.FingerprintMatch, AttestationID: result.Attestation.AttestationID}, nil
	}
	if IsV2Family(family) {
		store, err := NewStoreV2("")
		if err != nil {
			return EligibilityResult{}, err
		}
		library, _, err := store.Read()
		if err != nil {
			return EligibilityResult{}, err
		}
		result, err := QueryLibraryAdmissionV2(library, subjectKey, fingerprint, family)
		if err != nil {
			return EligibilityResult{}, err
		}
		return EligibilityResult{Eligible: result.Eligible, EffectiveStatus: result.EffectiveStatus, Reason: result.Reason, SubjectKey: subjectKey, BinaryFingerprint: fingerprint, FingerprintMatch: result.FingerprintMatch, AttestationID: result.Attestation.AttestationID}, nil
	}
	return EligibilityResult{EffectiveStatus: "unsupported", Reason: fmt.Sprintf("unsupported_processor_family:%s", family)}, nil
}
