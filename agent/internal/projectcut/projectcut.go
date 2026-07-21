// Package projectcut builds and validates dependency-aware project cuts.
package projectcut

import (
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

type SourceGuarantee string

const (
	// GuaranteeAdapterSnapshot describes the current VSP reference adapter: a
	// snapshot hash/revision is useful lineage, but not an apply-point CAS.
	GuaranteeAdapterSnapshot SourceGuarantee = "adapter_snapshot"
	// GuaranteeKernelBarrier may only be used after Kernel/VSP enforce the same
	// revision or barrier token at the command apply point.
	GuaranteeKernelBarrier SourceGuarantee = "kernel_barrier"
)

type BuildRequest struct {
	State                  *kernel.VSPStateResult
	ProjectUUID            string
	Guarantee              SourceGuarantee
	DependencyFingerprints []string
	ArtifactRefs           []string
	DerivedFrom            []string
	TargetFingerprints     []string
	ContractVersions       []string
}

func Build(request BuildRequest) (orchestration.ProjectCut, error) {
	if request.State == nil || !request.State.OK() {
		return orchestration.ProjectCut{}, fmt.Errorf("valid VSP state snapshot is required")
	}
	projectUUID := strings.TrimSpace(request.ProjectUUID)
	if projectUUID == "" {
		projectUUID = stateProjectUUID(request.State.LegacyState)
	}
	if projectUUID == "" {
		return orchestration.ProjectCut{}, fmt.Errorf("project uuid is required")
	}
	if strings.TrimSpace(request.State.ProjectEpoch) == "" || request.State.Revision <= 0 || strings.TrimSpace(request.State.SnapshotHash) == "" {
		return orchestration.ProjectCut{}, fmt.Errorf("snapshot epoch, revision and hash are required")
	}
	consistency := "bounded"
	if request.Guarantee == GuaranteeKernelBarrier {
		consistency = "strong"
	}
	lineage := append([]string(nil), request.DerivedFrom...)
	lineage = append(lineage, fmt.Sprintf("vsp.snapshot:%s:%d:%s", request.State.ProjectEpoch, request.State.Revision, request.State.SnapshotHash))
	cut := orchestration.ProjectCut{
		ProjectUUID:            projectUUID,
		ProjectEpoch:           request.State.ProjectEpoch,
		BaseProjectRevision:    fmt.Sprintf("%d", request.State.Revision),
		ReadBarrierToken:       fmt.Sprintf("vsp:%s:%d:%s", request.State.ProjectEpoch, request.State.Revision, request.State.SnapshotHash),
		Consistency:            consistency,
		DependencyFingerprints: canonical(request.DependencyFingerprints),
		ArtifactRefs:           canonical(request.ArtifactRefs),
		DerivedFrom:            canonical(lineage),
		TargetFingerprints:     canonical(request.TargetFingerprints),
		ContractVersions:       canonical(request.ContractVersions),
	}
	cut.Hash = cut.ComputeHash()
	return cut, nil
}

type ValidationStatus string

const (
	StatusValid   ValidationStatus = "valid"
	StatusStale   ValidationStatus = "stale"
	StatusInvalid ValidationStatus = "invalid"
)

type ValidationInput struct {
	ProjectUUID            string
	ProjectEpoch           string
	DependencyFingerprints []string
	TargetFingerprints     []string
	ContractVersions       []string
	RequireExecutable      bool
}

type ValidationResult struct {
	Status  ValidationStatus `json:"status"`
	Reasons []string         `json:"reasons,omitempty"`
}

func Validate(cut orchestration.ProjectCut, current ValidationInput) ValidationResult {
	if strings.TrimSpace(cut.Hash) == "" || cut.Hash != cut.ComputeHash() {
		return ValidationResult{Status: StatusInvalid, Reasons: []string{"cut_hash_mismatch"}}
	}
	reasons := make([]string, 0, 5)
	if cut.ProjectUUID != strings.TrimSpace(current.ProjectUUID) {
		reasons = append(reasons, "project_uuid_changed")
	}
	if cut.ProjectEpoch != strings.TrimSpace(current.ProjectEpoch) {
		reasons = append(reasons, "project_epoch_changed")
	}
	if !sameSet(cut.DependencyFingerprints, current.DependencyFingerprints) {
		reasons = append(reasons, "dependency_changed")
	}
	if !sameSet(cut.TargetFingerprints, current.TargetFingerprints) {
		reasons = append(reasons, "target_changed")
	}
	if !sameSet(cut.ContractVersions, current.ContractVersions) {
		reasons = append(reasons, "contract_changed")
	}
	if current.RequireExecutable && !cut.IsExecutable() {
		reasons = append(reasons, "cut_not_executable")
	}
	if len(reasons) > 0 {
		return ValidationResult{Status: StatusStale, Reasons: reasons}
	}
	return ValidationResult{Status: StatusValid}
}

func stateProjectUUID(state map[string]any) string {
	for _, key := range []string{"project_uuid", "id"} {
		if value := cleanString(state[key]); value != "" {
			return value
		}
	}
	if project, ok := state["project"].(map[string]any); ok {
		for _, key := range []string{"project_uuid", "id"} {
			if value := cleanString(project[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func cleanString(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func canonical(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sameSet(left, right []string) bool {
	l, r := canonical(left), canonical(right)
	if len(l) != len(r) {
		return false
	}
	for index := range l {
		if l[index] != r[index] {
			return false
		}
	}
	return true
}
