package projectcut

import (
	"testing"

	"vit-daw-agent/internal/kernel"
)

func TestBuildFromCurrentVSPAdapterIsBoundedNotExecutable(t *testing.T) {
	cut, err := Build(BuildRequest{
		State: &kernel.VSPStateResult{
			Response:     map[string]any{"type": "state.snapshot"},
			LegacyState:  map[string]any{"project_uuid": "project-1"},
			ProjectEpoch: "epoch-path-derived", Revision: 7, SnapshotHash: "snapshot-hash",
		},
		Guarantee:              GuaranteeAdapterSnapshot,
		DependencyFingerprints: []string{"track:v:volume_db:0"},
		TargetFingerprints:     []string{"track:v:volume_db:0"},
		ContractVersions:       []string{"capability:b2:v0", "manifest:b2:v1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cut.Consistency != "bounded" || cut.IsExecutable() {
		t.Fatalf("adapter snapshot must not become executable: %#v", cut)
	}
}

func TestDependencyChangeMatrix(t *testing.T) {
	cut, err := Build(BuildRequest{
		State: &kernel.VSPStateResult{
			Response:     map[string]any{"type": "state.snapshot"},
			LegacyState:  map[string]any{"project_uuid": "project-1"},
			ProjectEpoch: "epoch-1", Revision: 2, SnapshotHash: "hash-1",
		},
		Guarantee:              GuaranteeKernelBarrier,
		DependencyFingerprints: []string{"track:v:volume_db:0", "mom:obs-1"},
		TargetFingerprints:     []string{"track:v:volume_db:0"},
		ContractVersions:       []string{"capability:b2:v0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	base := ValidationInput{
		ProjectUUID: "project-1", ProjectEpoch: "epoch-1",
		DependencyFingerprints: []string{"mom:obs-1", "track:v:volume_db:0"},
		TargetFingerprints:     []string{"track:v:volume_db:0"},
		ContractVersions:       []string{"capability:b2:v0"}, RequireExecutable: true,
	}
	if result := Validate(cut, base); result.Status != StatusValid {
		t.Fatalf("unchanged dependencies should be valid: %#v", result)
	}

	tests := []struct {
		name   string
		change func(*ValidationInput)
		reason string
	}{
		{"target fader changed", func(in *ValidationInput) {
			in.DependencyFingerprints[1] = "track:v:volume_db:-1"
			in.TargetFingerprints[0] = "track:v:volume_db:-1"
		}, "dependency_changed"},
		{"observation changed", func(in *ValidationInput) { in.DependencyFingerprints[0] = "mom:obs-2" }, "dependency_changed"},
		{"epoch changed", func(in *ValidationInput) { in.ProjectEpoch = "epoch-2" }, "project_epoch_changed"},
		{"contract changed", func(in *ValidationInput) { in.ContractVersions[0] = "capability:b2:v1" }, "contract_changed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := base
			current.DependencyFingerprints = append([]string(nil), base.DependencyFingerprints...)
			current.TargetFingerprints = append([]string(nil), base.TargetFingerprints...)
			current.ContractVersions = append([]string(nil), base.ContractVersions...)
			test.change(&current)
			result := Validate(cut, current)
			if result.Status != StatusStale || !contains(result.Reasons, test.reason) {
				t.Fatalf("unexpected validation result: %#v", result)
			}
		})
	}

	// An unrelated UI/color change is intentionally absent from the declared
	// dependency vector and therefore does not invalidate the Cut.
	if result := Validate(cut, base); result.Status != StatusValid {
		t.Fatalf("unrelated change must not affect dependency cut: %#v", result)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
