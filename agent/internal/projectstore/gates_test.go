package projectstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnforceBudgetsEvictsOldestUnpinnedObservation(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	if err := os.WriteFile(projectPath, []byte("vit"), 0o644); err != nil {
		t.Fatal(err)
	}
	roots, manifest, err := Ensure(projectPath, "vitproj_gate")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Budgets.ObservationMaxCount = 2
	if err := Write(roots, manifest); err != nil {
		t.Fatal(err)
	}
	obsDir := filepath.Join(roots.Agent, "observations")
	if err := os.MkdirAll(obsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name string
		when time.Time
		body string
	}{
		{"obs_old", time.Unix(10, 0), `{"observation_id":"obs_old"}`},
		{"obs_pin", time.Unix(20, 0), `{"observation_id":"obs_pin"}`},
		{"obs_new", time.Unix(30, 0), `{"observation_id":"obs_new"}`},
	} {
		path := filepath.Join(obsDir, row.name+".json")
		if err := os.WriteFile(path, []byte(row.body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, row.when, row.when); err != nil {
			t.Fatal(err)
		}
	}
	decisionDir := filepath.Join(roots.Agent, "mixboard", "decisions")
	if err := os.MkdirAll(decisionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	decision, _ := json.Marshal(map[string]any{"refs": map[string]any{"before_observation": "mix.observe:obs_pin"}})
	if err := os.WriteFile(filepath.Join(decisionDir, "decision.json"), decision, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnforceBudgets(roots); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(obsDir, "obs_old.json")); !os.IsNotExist(err) {
		t.Fatalf("old unpinned observation was not evicted: %v", err)
	}
	for _, name := range []string{"obs_pin.json", "obs_new.json"} {
		if _, err := os.Stat(filepath.Join(obsDir, name)); err != nil {
			t.Fatalf("observation %s missing: %v", name, err)
		}
	}
}

func TestEnforceBudgetsEntersEvidenceOffWithoutDeletingAuthoritativeData(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	if err := os.WriteFile(projectPath, []byte("vit"), 0o644); err != nil {
		t.Fatal(err)
	}
	roots, manifest, err := Ensure(projectPath, "vitproj_total")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Budgets.StoreTotalMaxBytes = 1
	if err := Write(roots, manifest); err != nil {
		t.Fatal(err)
	}
	authoritative := filepath.Join(roots.Agent, "journal", "000001.jsonl")
	if err := os.MkdirAll(filepath.Dir(authoritative), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authoritative, []byte(`{"schema_version":"journal_action.v2"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnforceBudgets(roots); err != nil {
		t.Fatal(err)
	}
	got, err := Load(roots)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Degraded.EvidenceOff {
		t.Fatalf("expected evidence_off, manifest=%+v", got)
	}
	if _, err := os.Stat(authoritative); err != nil {
		t.Fatalf("authoritative journal was deleted: %v", err)
	}
}

func TestEnforceBudgetsEvictsRenewableEvidenceForTotalBudget(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	if err := os.WriteFile(projectPath, []byte("vit"), 0o644); err != nil {
		t.Fatal(err)
	}
	roots, _, err := Ensure(projectPath, "vitproj_total_evidence")
	if err != nil {
		t.Fatal(err)
	}
	oldRef, oldSize, err := PutEvidence(roots, "feature_snapshot", map[string]any{"value": string(make([]byte, 2048))}, "obs:old")
	if err != nil {
		t.Fatal(err)
	}
	newRef, _, err := PutEvidence(roots, "feature_snapshot", map[string]any{"value": string(make([]byte, 2048)), "new": true}, "obs:new")
	if err != nil {
		t.Fatal(err)
	}
	index, err := loadEvidenceIndex(roots)
	if err != nil {
		t.Fatal(err)
	}
	oldHash := oldRef[len("evidence://"):]
	newHash := newRef[len("evidence://"):]
	oldEntry, newEntry := index.Entries[oldHash], index.Entries[newHash]
	oldEntry.LastReferencedAt = time.Unix(1, 0)
	newEntry.LastReferencedAt = time.Unix(2, 0)
	index.Entries[oldHash], index.Entries[newHash] = oldEntry, newEntry
	if err := writeEvidenceIndex(roots, index); err != nil {
		t.Fatal(err)
	}
	manifest, err := Recalibrate(roots)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Budgets.StoreTotalMaxBytes = manifest.Counters.TotalBytes - oldSize/2
	if err := Write(roots, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := EnforceBudgets(roots); err != nil {
		t.Fatal(err)
	}
	if _, err := GetEvidence(roots, oldRef); !os.IsNotExist(err) {
		t.Fatalf("old renewable evidence was not evicted: %v", err)
	}
	if _, err := GetEvidence(roots, newRef); err != nil {
		t.Fatalf("new evidence was evicted first: %v", err)
	}
}
