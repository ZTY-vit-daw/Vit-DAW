package mixboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/tim"
)

// L2-1-TIM-1 agent-side assertion legs: timInputFromObservation must carry the
// project audio settings, per-track rack digests, the plugin semantics index
// as the known-plugin table, and the per-track L3 nonfinite passthrough keys
// into the TIM assertion input (design §3.1 input assembly).
func TestTIMInputCarriesAssertionLegInputs(t *testing.T) {
	indexDir := t.TempDir()
	indexPath := filepath.Join(indexDir, "plugin_semantics.json")
	index := pluginsemantics.Index{SchemaVersion: pluginsemantics.SchemaVersion, Entries: []pluginsemantics.Entry{
		{ID: "known", Name: "Known", PluginPath: "/Library/Audio/Plug-Ins/VST3/known.vst3", Format: "VST3"},
	}}
	if err := os.WriteFile(indexPath, mustJSON(t, index), 0o600); err != nil {
		t.Fatalf("write plugin index: %v", err)
	}
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", indexPath)

	obs := ObservationPacket{
		ProjectUUID: "p1",
		ProjectPackage: map[string]any{
			"project_uuid": "p1", "project_epoch": "e1", "project_revision": "7", "track_count": 1,
			"tracks": []any{map[string]any{
				"track_id": "t1", "track_name": "Kick", "clip_count": 1,
				"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/stems/kick.wav", "sample_rate_hz": 44100},
				"acoustic":     map[string]any{"status": "ready", "peak_dbfs": -6.0, "headroom_db": 6.0},
			}},
		},
		GlobalSummary: map[string]any{
			"feature_snapshot": map[string]any{
				"track_waveform_envelopes": []any{map[string]any{
					"track_id": "t1", "status": "ready", "nan_count": 2, "inf_count": 0,
				}},
			},
		},
	}
	req := Request{ProjectState: map[string]any{
		"project_uuid": "p1", "project_epoch": "e1", "project_revision": "7",
		"audio_settings": map[string]any{"sample_rate_hz": 48000},
		"tracks": []any{map[string]any{
			"track_id": "t1",
			"rack": map[string]any{
				"nodes": []any{map[string]any{
					"node_id": "n1", "enabled": true, "audio_reachable_from_rack_input": true,
					"plugin_path": "/Library/Audio/Plug-Ins/VST3/known.vst3", "plugin_format": "VST3",
				}},
				"edges": []any{map[string]any{"source_id": "RACK_INPUT", "dest_id": "n1"}},
			},
		}},
	}}

	input := timInputFromObservation(obs, req)
	if got := numberFromMap(input.AudioSettings, "sample_rate_hz"); got != 48000 {
		t.Fatalf("audio settings sample rate = %v, want 48000: %#v", got, input.AudioSettings)
	}
	if len(input.RackSummaries) != 1 || input.RackSummaries[0].TrackID != "t1" || len(input.RackSummaries[0].Nodes) != 1 {
		t.Fatalf("rack summaries missing or malformed: %#v", input.RackSummaries)
	}
	if !input.KnownPluginPaths["/Library/Audio/Plug-Ins/VST3/known.vst3"] {
		t.Fatalf("known plugin table missing the index entry: %#v", input.KnownPluginPaths)
	}
	if got := numberFromMap(input.AcousticEvidenceByTrack["t1"], "nan_count"); got != 2 {
		t.Fatalf("nonfinite evidence not carried: %#v", input.AcousticEvidenceByTrack)
	}

	proj := tim.Build(input)
	failed := false
	for _, row := range proj.Assertions {
		if row.Asserter == "signal_hygiene" && row.Check == "signal_nonfinite" && row.TrackID == "t1" {
			if row.Status != tim.AssertionStatusFail || row.Code != "assert_signal_nonfinite" {
				t.Fatalf("nonfinite assertion wrong end to end: %#v", row)
			}
			failed = true
		}
		if row.Asserter == "sample_rate" && row.Check == "consistency" && row.TrackID == "t1" {
			if row.Status != tim.AssertionStatusFail || row.Code != "assert_sample_rate_mismatch" {
				t.Fatalf("sample rate assertion wrong end to end: %#v", row)
			}
		}
		if row.Asserter == "plugin_legality" && row.TrackID == "t1" && row.Status != tim.AssertionStatusPass {
			t.Fatalf("known plugin path should pass: %#v", row)
		}
	}
	if !failed {
		t.Fatalf("nonfinite fail did not reach the projection assertions: %#v", proj.Assertions)
	}
}

// Without a plugin semantics index the plugin legality assertion must degrade
// to not_evaluable, not fabricate unknown-path fails.
func TestTIMInputPluginLegalityNotEvaluableWithoutIndex(t *testing.T) {
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(t.TempDir(), "missing_index.json"))
	obs := ObservationPacket{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks":      []any{map[string]any{"track_id": "t1", "clip_count": 1}},
	}}
	input := timInputFromObservation(obs, Request{ProjectState: map[string]any{
		"tracks": []any{map[string]any{
			"track_id": "t1",
			"rack": map[string]any{"nodes": []any{map[string]any{
				"node_id": "n1", "enabled": true, "audio_reachable_from_rack_input": true,
				"plugin_path": "/x/unknown.vst3", "plugin_format": "VST3",
			}}},
		}},
	}})
	if input.KnownPluginPaths != nil {
		t.Fatalf("missing index must yield a nil known-plugin table: %#v", input.KnownPluginPaths)
	}
	proj := tim.Build(input)
	for _, row := range proj.Assertions {
		if row.Asserter == "plugin_legality" && row.TrackID == "t1" && row.Status == tim.AssertionStatusFail {
			t.Fatalf("missing index fabricated a plugin fail: %#v", row)
		}
	}
	found := false
	for _, row := range proj.Assertions {
		if row.Asserter == "plugin_legality" && row.TrackID == "t1" && row.Status == tim.AssertionStatusNotEvaluable {
			found = true
		}
	}
	if !found {
		t.Fatalf("plugin legality not_evaluable row missing: %#v", proj.Assertions)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}
