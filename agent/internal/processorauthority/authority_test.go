package processorauthority

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
)

func TestImportStrongEQReceiptPromotesOnlyObservedActions(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "Example EQ.vst3")
	writeFixture(t, pluginPath, []byte("eq-binary"))
	entry := fixtureEntry("Example EQ", "Vendor", "example-eq", pluginPath)
	receiptPath := filepath.Join(root, "20260806_120000", "summary.json")
	writeJSONFixture(t, receiptPath, map[string]any{
		"schema_version": SchemaPluginAllianceEQLive, "status": "passed",
		"results": []any{map[string]any{
			"case_id": "example_eq", "plugin_name": entry.Name, "mode": "positive", "status": "exact", "restored": true,
			"resolution": map[string]any{"name": entry.Name, "manufacturer": entry.Manufacturer, "format": entry.Format,
				"identifier": entry.Identifier, "plugin_path": entry.PluginPath},
			"requested_edits": []any{map[string]any{"action": "upsert", "shape": "bell"}},
			"undo":            map[string]any{"rollback": map[string]any{"verified": true}},
		}},
	})
	store, _ := processorattestation.NewStore(filepath.Join(root, "attestations.json"))
	report, err := ImportReceipts(store, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}}, []string{receiptPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Promoted) != 1 || report.Promoted[0].Status != processorattestation.StatusPromoted {
		t.Fatalf("import report=%+v", report)
	}
	want := map[string]bool{"upsert/bell": true, "undo/bell": true}
	for _, coverage := range report.Promoted[0].Coverage {
		delete(want, coverage.Action+"/"+coverage.Shape)
	}
	if len(want) != 0 || len(report.Promoted[0].Coverage) != 2 {
		t.Fatalf("unexpected EQ coverage=%+v missing=%+v", report.Promoted[0].Coverage, want)
	}
	encoded, _ := json.Marshal(report.Promoted[0])
	for _, forbidden := range []string{"param_id", "topology_generation", "profile", "vps", "spal"} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("badge leaked %q: %s", forbidden, encoded)
		}
	}
	if _, err := ImportReceipts(store, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}}, []string{receiptPath}); err != nil {
		t.Fatal(err)
	}
	library, _, err := store.Read()
	if err != nil || len(library.Attestations) != 1 || library.Revision != 1 {
		t.Fatalf("repeat import was not idempotent: records=%d revision=%d err=%v", len(library.Attestations), library.Revision, err)
	}
}

func TestImportStrongCompressorReceiptProjectsVerifiedRoles(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "Example Compressor.vst3")
	writeFixture(t, pluginPath, []byte("compressor-binary"))
	entry := fixtureEntry("Example Compressor", "Plugin Alliance", "example-compressor", pluginPath)
	receiptRoot := filepath.Join(root, "20260806_130000")
	receiptPath := filepath.Join(receiptRoot, "summary.json")
	result := map[string]any{
		"id": "example_compressor", "plugin_name": entry.Name, "expectation": "compressor", "status": "passed",
		"temporary_track_deleted": true, "topology_generation_stable": true, "apply_status": "exact",
		"restore_status": "exact", "write_count": 2, "full_snapshot_restored": true,
		"selected_roles": []string{"threshold", "mix"},
	}
	writeJSONFixture(t, receiptPath, map[string]any{
		"schema_version": SchemaCompressorRegression11, "verdict": "passed", "results": []any{result},
	})
	writeJSONFixture(t, filepath.Join(receiptRoot, "cases", "01_example_compressor", "evidence.json"), map[string]any{
		"completed_at": time.Date(2026, 8, 6, 5, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"case": map[string]any{"id": "example_compressor", "plugin_name": entry.Name, "plugin_path": pluginPath,
			"plugin_identifier": entry.Identifier},
		"result": result,
	})
	store, _ := processorattestation.NewStore(filepath.Join(root, "attestations.json"))
	report, err := ImportReceipts(store, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}}, []string{receiptPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Promoted) != 1 {
		t.Fatalf("promoted=%+v skipped=%+v", report.Promoted, report.Skipped)
	}
	want := map[string]bool{"activation_intensity": true, "parallel_balance": true}
	for _, coverage := range report.Promoted[0].Coverage {
		delete(want, coverage.Axis)
	}
	if len(want) != 0 || len(report.Promoted[0].Coverage) != 2 || len(report.Promoted[0].Evidence) != 2 {
		t.Fatalf("coverage=%+v evidence=%+v missing=%+v", report.Promoted[0].Coverage, report.Promoted[0].Evidence, want)
	}
}

func TestRecognizerOnlyCompressorReceiptReportsEvidenceGap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "20260806_140000", "summary.json")
	writeJSONFixture(t, path, map[string]any{
		"schema_version": SchemaCompressorCompat, "status": "ok",
		"results": []any{map[string]any{"id": "waves_comp", "plugin_name": "Waves Comp", "status": "captured"}},
	})
	report, err := ReadReceipt(path, pluginsemantics.Index{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Candidates) != 0 || len(report.Skipped) != 1 ||
		report.Skipped[0].Reason != "evidence_insufficient_control_write_restore_missing" {
		t.Fatalf("recognizer-only receipt was not held: %+v", report)
	}
}

func fixtureEntry(name, manufacturer, identifier, path string) pluginsemantics.Entry {
	return pluginsemantics.Entry{Name: name, Manufacturer: manufacturer, Format: "VST3", Identifier: identifier, PluginPath: path}
}

func writeJSONFixture(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, path, raw)
}

func writeFixture(t *testing.T, path string, value []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, value, 0o600); err != nil {
		t.Fatal(err)
	}
}
