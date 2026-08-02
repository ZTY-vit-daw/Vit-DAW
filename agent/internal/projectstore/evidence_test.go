package projectstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceContentAddressingDeduplicatesAndLoads(t *testing.T) {
	roots, _, err := Ensure(filepath.Join(t.TempDir(), "song.vit"), "vitproj_evidence")
	if err != nil {
		t.Fatal(err)
	}
	content := map[string]any{"rows": []any{map[string]any{"track_id": "1", "value": 2.0}}}
	first, firstSize, err := PutEvidence(roots, "feature_snapshot", content, "obs:1")
	if err != nil {
		t.Fatal(err)
	}
	second, secondSize, err := PutEvidence(roots, "feature_snapshot", content, "obs:2")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || firstSize != secondSize {
		t.Fatalf("first=%q/%d second=%q/%d", first, firstSize, second, secondSize)
	}
	blob, err := GetEvidence(roots, first)
	if err != nil {
		t.Fatal(err)
	}
	if blob.Kind != "feature_snapshot" || blob.ProjectUUID != roots.ProjectUUID {
		t.Fatalf("blob=%+v", blob)
	}
	index, err := loadEvidenceIndex(roots)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Entries) != 1 {
		t.Fatalf("entries=%#v", index.Entries)
	}
	for _, entry := range index.Entries {
		if len(entry.Refs) != 2 {
			t.Fatalf("refs=%#v", entry.Refs)
		}
	}
}

func TestEvidenceReferencedByPinnedObservationCannotBeEvicted(t *testing.T) {
	roots, manifest, err := Ensure(filepath.Join(t.TempDir(), "song.vit"), "vitproj_decision_pin")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Budgets.EvidenceMaxBytes = 4096
	manifest.Budgets.EvidenceBlobMaxBytes = 4096
	if err := Write(roots, manifest); err != nil {
		t.Fatal(err)
	}
	_, firstSize, err := PutEvidence(roots, "feature_snapshot", map[string]any{"value": strings.Repeat("x", 400)}, "obs:obs_keep")
	if err != nil {
		t.Fatal(err)
	}
	decisionDir := filepath.Join(roots.Agent, "mixboard", "decisions", "decisions")
	if err := os.MkdirAll(decisionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decisionDir, "keep.json"), []byte(`{"refs":{"before_observation":"mix.observe:obs_keep"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err = Load(roots)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Budgets.EvidenceMaxBytes = firstSize + 100
	if err := Write(roots, manifest); err != nil {
		t.Fatal(err)
	}
	_, _, err = PutEvidence(roots, "feature_snapshot", map[string]any{"value": strings.Repeat("y", 400)}, "obs:obs_new")
	if !errors.Is(err, ErrEvidenceDisabled) {
		t.Fatalf("err=%v want ErrEvidenceDisabled", err)
	}
}

func TestEvidenceBudgetEnablesDegradedModeWhenPinnedDataCannotBeEvicted(t *testing.T) {
	roots, manifest, err := Ensure(filepath.Join(t.TempDir(), "song.vit"), "vitproj_budget")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Budgets.EvidenceMaxBytes = 1024
	manifest.Budgets.EvidenceBlobMaxBytes = 4096
	if err := Write(roots, manifest); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PutEvidence(roots, "feature_snapshot", map[string]any{"value": strings.Repeat("x", 200)}, "decision:keep"); err != nil {
		t.Fatal(err)
	}
	_, _, err = PutEvidence(roots, "feature_snapshot", map[string]any{"value": strings.Repeat("y", 900)}, "obs:new")
	if !errors.Is(err, ErrEvidenceDisabled) {
		t.Fatalf("err=%v want ErrEvidenceDisabled", err)
	}
	loaded, err := Load(roots)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Degraded.EvidenceOff || loaded.Degraded.Reason != "evidence_budget_exhausted" {
		t.Fatalf("degraded=%+v", loaded.Degraded)
	}
}
