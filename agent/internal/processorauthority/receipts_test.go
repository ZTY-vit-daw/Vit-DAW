package processorauthority

import (
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
)

// proQ3ReceiptRow mirrors the result_row dict that the deterministic EQ
// channel writer produces (scripts/pca_autosweep.py certify_eq): upsert bell
// + modify/disable extra actions, each undone transactionally, restored
// stages and an empty final_drift list.
func proQ3ReceiptRow(entry pluginsemantics.Entry) map[string]any {
	requested := []any{map[string]any{"action": "upsert", "shape": "bell"}}
	return map[string]any{
		"case_id": "pro_q_3", "plugin_name": entry.Name, "identifier": entry.Identifier,
		"resolution": map[string]any{"name": entry.Name, "manufacturer": entry.Manufacturer,
			"format": entry.Format, "identifier": entry.Identifier, "plugin_path": entry.PluginPath},
		"mode": "positive", "status": "passed", "restored": true,
		"requested_edits": requested,
		"undo":            map[string]any{"rollback": map[string]any{"verified": true}},
		"extra_actions": []any{
			map[string]any{"action": "modify", "result": map[string]any{"status": "passed",
				"edits":    []any{map[string]any{"action": "modify", "shape": "bell", "status": "passed"}},
				"rollback": map[string]any{"verified": true}}},
			map[string]any{"action": "undo_modify", "result": map[string]any{"status": "passed",
				"rollback": map[string]any{"verified": true}}},
			map[string]any{"action": "disable", "result": map[string]any{"status": "passed",
				"edits":    []any{map[string]any{"action": "disable", "shape": "bell", "status": "passed"}},
				"rollback": map[string]any{"verified": true}}},
			map[string]any{"action": "undo_disable", "result": map[string]any{"status": "passed",
				"rollback": map[string]any{"verified": true}}},
		},
		"stages":      []any{map[string]any{"level": "parameter", "requested_edits": requested, "status": "passed", "restored": true, "undo": map[string]any{"rollback": map[string]any{"verified": true}}}},
		"final_drift": []any{},
	}
}

func TestFabFilterEQChannelReceiptAdmitsProQ3(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "FabFilter Pro-Q 3.vst3")
	writeFixture(t, pluginPath, []byte("pro-q3-binary"))
	entry := fixtureEntry("Pro-Q 3", "FabFilter", "VST3-Pro-Q 3-a1aba170-ba5f7ab2", pluginPath)
	receiptPath := filepath.Join(root, "20261109_021000", "summary.json")
	writeJSONFixture(t, receiptPath, map[string]any{
		"schema_version": SchemaFabFilterEQLive, "status": "passed",
		"results": []any{proQ3ReceiptRow(entry)},
		"audit":   map[string]any{"natural_language_chat_count": 0},
	})
	store, _ := processorattestation.NewStore(filepath.Join(root, "attestations.json"))
	report, err := ImportReceipts(store, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}}, []string{receiptPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Promoted) != 1 || report.Promoted[0].Status != processorattestation.StatusPromoted {
		t.Fatalf("import report=%+v skipped=%+v", report, report.Skipped)
	}
	promoted := report.Promoted[0]
	if promoted.ProcessorFamily != processorattestation.FamilyStaticEQ {
		t.Fatalf("family=%q want static_eq", promoted.ProcessorFamily)
	}
	if promoted.Subject.Identifier != entry.Identifier || promoted.Subject.Manufacturer != "FabFilter" || promoted.Subject.SubjectKey == "" {
		t.Fatalf("subject=%+v", promoted.Subject)
	}
	if len(promoted.Evidence) != 1 || promoted.Evidence[0].Kind != SchemaFabFilterEQLive {
		t.Fatalf("evidence=%+v", promoted.Evidence)
	}
	coverage := map[string]bool{}
	for _, row := range promoted.Coverage {
		coverage[row.Action+"/"+row.Shape] = true
	}
	for _, want := range []string{"upsert/bell", "undo/bell", "modify/bell", "disable/bell"} {
		if !coverage[want] {
			t.Fatalf("coverage missing %q: %+v", want, promoted.Coverage)
		}
	}
}

func TestUnknownEQVendorReceiptKindsStillFailClosed(t *testing.T) {
	// The vendor-neutral spelling and every other unknown vendor/version
	// string must stay outside the schema whitelist: only explicitly
	// enumerated vendor-scoped kinds are admitted (fail-closed unchanged).
	for _, schema := range []string{
		"static_eq.eq_channel_live_smoke.v1",
		"fabfilter.static_eq.eq_channel_live_smoke.v2",
		"tdr.static_eq.eq_channel_live_smoke.v1",
	} {
		path := filepath.Join(t.TempDir(), "20261109_022000", "summary.json")
		writeJSONFixture(t, path, map[string]any{
			"schema_version": schema, "status": "passed",
			"results": []any{map[string]any{"case_id": "x", "mode": "positive", "status": "passed"}},
		})
		if _, err := ReadReceipt(path, pluginsemantics.Index{}); err == nil ||
			!strings.Contains(err.Error(), "unsupported receipt schema") {
			t.Fatalf("schema %q was not fail-closed rejected: err=%v", schema, err)
		}
	}
}

func TestWavesEQLiveReceiptStillAdmits(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "Q10.vst3")
	writeFixture(t, pluginPath, []byte("q10-binary"))
	entry := fixtureEntry("Q10", "Waves", "waves-q10", pluginPath)
	receiptPath := filepath.Join(root, "20261109_023000", "summary.json")
	writeJSONFixture(t, receiptPath, map[string]any{
		"schema_version": SchemaWavesEQLive, "status": "passed",
		"results": []any{map[string]any{
			"case_id": "q10_stereo", "plugin_name": entry.Name, "mode": "positive", "status": "exact", "restored": true,
			"resolution": map[string]any{"name": entry.Name, "manufacturer": entry.Manufacturer, "format": entry.Format,
				"identifier": entry.Identifier, "plugin_path": entry.PluginPath},
			"requested_edits": []any{map[string]any{"action": "upsert", "shape": "bell"}},
			"undo":            map[string]any{"rollback": map[string]any{"verified": true}},
		}},
		"audit": map[string]any{"natural_language_chat_count": 0},
	})
	report, err := ReadReceipt(receiptPath, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Candidates) != 1 || !report.Candidates[0].Strong ||
		report.Candidates[0].Spec.ProcessorFamily != processorattestation.FamilyStaticEQ {
		t.Fatalf("waves receipt regressed: candidates=%+v skipped=%+v", report.Candidates, report.Skipped)
	}
}
