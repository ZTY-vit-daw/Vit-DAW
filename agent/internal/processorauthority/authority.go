package processorauthority

import (
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
)

type ImportReport struct {
	Receipts []ReceiptReport                    `json:"receipts"`
	Promoted []processorattestation.Attestation `json:"promoted"`
	Skipped  []Skip                             `json:"skipped,omitempty"`
}

// ImportReceipts deterministically aggregates strong legacy receipts and
// promotes one current badge per processor subject and family.
func ImportReceipts(store *processorattestation.Store, index pluginsemantics.Index, paths []string) (ImportReport, error) {
	if store == nil {
		return ImportReport{}, fmt.Errorf("processor authority: attestation store is required")
	}
	paths = append([]string(nil), paths...)
	sort.Strings(paths)
	report := ImportReport{Receipts: []ReceiptReport{}, Promoted: []processorattestation.Attestation{}}
	groups := map[string]processorattestation.IssueSpec{}
	for _, path := range paths {
		receipt, err := ReadReceipt(path, index)
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
			if current.Status == processorattestation.StatusPromoted &&
				current.Subject.SubjectKey == spec.Subject.SubjectKey && current.ProcessorFamily == spec.ProcessorFamily &&
				current.BinaryFingerprint == spec.BinaryFingerprint {
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
