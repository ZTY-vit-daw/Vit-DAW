package processorauthority

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
)

type CandidateV2 struct {
	Spec       processorattestation.IssueSpecV2
	Receipt    string
	CaseID     string
	Strong     bool
	GateReason string
}

type ReceiptReportV2 struct {
	Path       string        `json:"path"`
	Schema     string        `json:"schema"`
	Candidates []CandidateV2 `json:"-"`
	Skipped    []Skip        `json:"skipped,omitempty"`
}

type ImportReportV2 struct {
	Receipts []ReceiptReportV2                    `json:"receipts"`
	Promoted []processorattestation.AttestationV2 `json:"promoted"`
	Skipped  []Skip                               `json:"skipped,omitempty"`
}

func ReadReceiptV2(path string, index pluginsemantics.Index) (ReceiptReportV2, error) {
	path = filepath.Clean(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		return ReceiptReportV2{}, fmt.Errorf("processor authority v2: read receipt: %w", err)
	}
	var receipt rawReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return ReceiptReportV2{}, fmt.Errorf("processor authority v2: decode receipt: %w", err)
	}
	report := ReceiptReportV2{Path: path, Schema: receipt.SchemaVersion}
	if receipt.SchemaVersion != SchemaLocalProcessorCertificationV2 {
		return report, fmt.Errorf("processor authority v2: unsupported receipt schema %q", receipt.SchemaVersion)
	}
	if receipt.Verdict != "passed" || receipt.Status != "passed" {
		return report, fmt.Errorf("processor authority v2: certification receipt verdict is not passed")
	}
	ref := evidenceRef(path, receipt.SchemaVersion, raw, receipt.CompletedAt)
	for _, result := range receipt.Results {
		candidate, reason := v2Candidate(path, result, ref, index)
		if reason != "" {
			report.Skipped = append(report.Skipped, Skip{CaseID: firstText(result.ID, result.CaseID), Reason: reason})
			continue
		}
		candidate.Receipt = path
		report.Candidates = append(report.Candidates, candidate)
	}
	sort.Slice(report.Candidates, func(i, j int) bool {
		return report.Candidates[i].Spec.Subject.SubjectKey < report.Candidates[j].Spec.Subject.SubjectKey
	})
	return report, nil
}

func v2Candidate(summaryPath string, result rawReceiptResult, summaryRef processorattestation.EvidenceRef, index pluginsemantics.Index) (CandidateV2, string) {
	family := strings.ToLower(firstText(result.ProcessorFamily, result.Expectation))
	if !processorattestation.IsV2Family(family) {
		return CandidateV2{}, "unsupported_v2_processor_family"
	}
	if result.Status != "passed" || !result.TemporaryTrackDeleted || !result.TopologyGenerationStable || !successful(result.ApplyStatus) || !successful(result.RestoreStatus) || result.WriteCount <= 0 || !result.FullSnapshotRestored {
		return CandidateV2{}, "evidence_insufficient_control_gate_failed"
	}
	evidence, caseRef, err := findCaseEvidence(summaryPath, firstText(result.ID, result.CaseID))
	if err != nil {
		return CandidateV2{}, "case_evidence_missing: " + err.Error()
	}
	caseID := firstText(result.ID, result.CaseID)
	if reason := validateV2CaseConsistency(result, evidence, family, caseID); reason != "" {
		return CandidateV2{}, reason
	}
	roles := evidence.Result.SelectedRoles
	coverage := processorattestation.V2CoverageForRoles(family, roles)
	if len(coverage) == 0 {
		return CandidateV2{}, "evidence_insufficient_verified_action_axis_missing"
	}
	identity := evidence.FrozenResolution
	identity.Name = firstText(identity.Name, evidence.Case.PluginName, result.PluginName)
	identity.Identifier = firstText(identity.Identifier, evidence.Case.PluginIdentifier)
	identity.PluginPath = firstText(identity.PluginPath, evidence.Case.PluginPath)
	subject, reason := resolveSubject(identity, index)
	if reason != "" {
		return CandidateV2{}, reason
	}
	fingerprint, err := processorattestation.FingerprintPath(subject.InstalledPath)
	if err != nil {
		return CandidateV2{}, "binary_fingerprint_failed: " + err.Error()
	}
	return CandidateV2{CaseID: firstText(result.ID, result.CaseID), Strong: true, GateReason: "successful_write_readback_restore", Spec: processorattestation.IssueSpecV2{Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: family, Coverage: coverage, Evidence: []processorattestation.EvidenceRef{summaryRef, caseRef}}}, ""
}

func validateV2CaseConsistency(summary rawReceiptResult, evidence rawCaseEvidence, family, caseID string) string {
	if evidence.Case.ID != "" && evidence.Case.ID != caseID {
		return "case_evidence_identity_mismatch"
	}
	if evidence.Result.Status != "passed" || evidence.Result.ProcessorFamily != family || !evidence.Result.TemporaryTrackDeleted ||
		!evidence.Result.TopologyGenerationStable || !successful(evidence.Result.ApplyStatus) || !successful(evidence.Result.RestoreStatus) ||
		evidence.Result.WriteCount <= 0 || !evidence.Result.FullSnapshotRestored {
		return "case_evidence_control_gate_failed"
	}
	if summary.ProcessorFamily != "" && strings.ToLower(summary.ProcessorFamily) != family {
		return "summary_case_family_mismatch"
	}
	if !sameStringSet(summary.SelectedRoles, evidence.Result.SelectedRoles) {
		return "summary_case_roles_mismatch"
	}
	if len(evidence.Result.SelectedRoles) == 0 {
		return "case_evidence_selected_roles_missing"
	}
	return ""
}

func sameStringSet(left, right []string) bool {
	normalize := func(values []string) []string {
		seen := map[string]bool{}
		out := make([]string, 0, len(values))
		for _, value := range values {
			value = strings.ToLower(strings.TrimSpace(value))
			if value != "" && !seen[value] {
				seen[value] = true
				out = append(out, value)
			}
		}
		sort.Strings(out)
		return out
	}
	return strings.Join(normalize(left), "\x00") == strings.Join(normalize(right), "\x00")
}

func ImportReceiptsV2(store *processorattestation.StoreV2, index pluginsemantics.Index, paths []string) (ImportReportV2, error) {
	if store == nil {
		return ImportReportV2{}, fmt.Errorf("processor authority v2: attestation store is required")
	}
	paths = append([]string(nil), paths...)
	sort.Strings(paths)
	report := ImportReportV2{Receipts: []ReceiptReportV2{}, Promoted: []processorattestation.AttestationV2{}}
	groups := map[string]processorattestation.IssueSpecV2{}
	for _, path := range paths {
		receipt, err := ReadReceiptV2(path, index)
		if err != nil {
			return report, err
		}
		report.Receipts = append(report.Receipts, receipt)
		report.Skipped = append(report.Skipped, receipt.Skipped...)
		for _, candidate := range receipt.Candidates {
			if !candidate.Strong {
				report.Skipped = append(report.Skipped, Skip{CaseID: candidate.CaseID, Reason: "evidence_not_strong"})
				continue
			}
			key := candidate.Spec.Subject.SubjectKey + "\x00" + candidate.Spec.ProcessorFamily + "\x00" + candidate.Spec.BinaryFingerprint
			spec := groups[key]
			if spec.Subject.SubjectKey == "" {
				spec = candidate.Spec
			} else {
				spec.Coverage = append(spec.Coverage, candidate.Spec.Coverage...)
				spec.Evidence = append(spec.Evidence, candidate.Spec.Evidence...)
			}
			groups[key] = spec
		}
	}
	library, _, err := store.Read()
	if err != nil {
		return report, err
	}
	for key, spec := range groups {
		for _, current := range library.Attestations {
			if current.Status == processorattestation.StatusPromoted && current.Subject.SubjectKey == spec.Subject.SubjectKey && current.ProcessorFamily == spec.ProcessorFamily && current.BinaryFingerprint == spec.BinaryFingerprint {
				spec.Coverage = append(spec.Coverage, current.Coverage...)
				spec.Evidence = append(spec.Evidence, current.Evidence...)
			}
		}
		groups[key] = spec
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		promoted, err := store.PromoteCurrent(groups[key], "deterministic_strong_receipt_passed")
		if err != nil {
			return report, err
		}
		report.Promoted = append(report.Promoted, promoted)
	}
	sort.Slice(report.Skipped, func(i, j int) bool {
		left := strings.ToLower(report.Skipped[i].CaseID + "\x00" + report.Skipped[i].Reason)
		right := strings.ToLower(report.Skipped[j].CaseID + "\x00" + report.Skipped[j].Reason)
		return left < right
	})
	return report, nil
}
