package capabilitycontext

import (
	"strings"
	"testing"
)

func TestStaticMixGainStagingContextManifestDefinesB1EvidenceSemantics(t *testing.T) {
	manifest := StaticMixGainStagingContextManifest()
	if manifest.SchemaVersion != ContextManifestSchemaVersion || manifest.ManifestID != "static_mix.gain_staging.context_manifest.v0" {
		t.Fatalf("manifest identity = %+v", manifest)
	}
	if manifest.CapabilityID != GainStagingCapabilityID {
		t.Fatalf("manifest capability = %q", manifest.CapabilityID)
	}
	merge := ContextMergeSpec{}
	for _, item := range manifest.Merges {
		if item.ID == "tracks" {
			merge = item
			break
		}
	}
	if strings.Join(merge.RowSetIDs, ",") != "context_snapshot_tracks,mix_project_tracks,audio_analysis_track_waveforms,project_state_tracks" {
		t.Fatalf("B1 track merge order = %+v", merge.RowSetIDs)
	}
	if merge.MergePolicy != "override" {
		t.Fatalf("B1 merge policy should let project.state win current writable fields: %+v", merge)
	}
	fields := map[string]ContextFieldSpec{}
	for _, field := range manifest.Fields {
		fields[field.ID] = field
	}
	if !fields["clip_gain_db"].Writable || !fields["clip_gain_db"].ControlTarget {
		t.Fatalf("clip_gain_db should be writable B1.2 control target: %+v", fields["clip_gain_db"])
	}
	if !strings.Contains(fields["volume_db"].Usage, "not B1.2") {
		t.Fatalf("volume_db usage should forbid B1.2 source calibration: %+v", fields["volume_db"])
	}
	if !strings.Contains(fields["peak_dbfs"].Usage, "last-resort") {
		t.Fatalf("peak_dbfs should be documented as fallback/headroom evidence: %+v", fields["peak_dbfs"])
	}
	foundAudioRows := false
	for _, rowset := range manifest.RowSets {
		if rowset.ID == "audio_analysis_track_waveforms" && rowset.SourceID == "audio_analysis_status" && rowset.EvidenceRef == "project.audio_analysis_status:track_waveform_envelopes" {
			foundAudioRows = true
			break
		}
	}
	if !foundAudioRows {
		t.Fatalf("B1 manifest should include project.audio_analysis_status waveform rows: %+v", manifest.RowSets)
	}
}

func TestCapabilityContextBuilderMergesB1RowsFromManifest(t *testing.T) {
	contextBuild := BuildCapabilityContext(ContextInput{
		Manifest: StaticMixGainStagingContextManifest(),
		Sources: map[string]any{
			"project_state": map[string]any{
				"tracks": []map[string]any{{
					"track_id":  "track_1",
					"volume_db": 0.0,
					"clips": []map[string]any{{
						"clip_id":      "clip_1",
						"type":         "audio",
						"clip_gain_db": 0.0,
					}},
				}},
			},
			"mix_observation": map[string]any{
				"observation": map[string]any{
					"project_package": map[string]any{
						"tracks": []map[string]any{{
							"track_id":    "track_1",
							"track_name":  "Track One",
							"rms_dbfs":    -20.0,
							"peak_dbfs":   -6.0,
							"headroom_db": 6.0,
						}},
					},
				},
			},
		},
	})

	rows := contextBuild.Rows("tracks")
	if len(rows) != 1 {
		t.Fatalf("merged rows = %+v", rows)
	}
	row := rows[0]
	if row.Key != "track_1" {
		t.Fatalf("row key = %q", row.Key)
	}
	if row.Data["rms_dbfs"] == nil || row.Data["clips"] == nil {
		t.Fatalf("merged row did not retain acoustic and writable state: %+v", row.Data)
	}
	refs := strings.Join(row.EvidenceRefs, ",")
	if !strings.Contains(refs, "mix.observe:project_package.tracks") || !strings.Contains(refs, "project.state:tracks") {
		t.Fatalf("merged row evidence refs = %+v", row.EvidenceRefs)
	}
}

func TestCapabilityContextBuilderNormalizesB1FieldAliases(t *testing.T) {
	contextBuild := BuildCapabilityContext(ContextInput{
		Manifest: StaticMixGainStagingContextManifest(),
		Sources: map[string]any{
			"project_state": map[string]any{
				"tracks": []map[string]any{{
					"id":       "track_1",
					"name":     "Track One",
					"fader_db": -60.0,
				}},
			},
			"mix_observation": map[string]any{
				"observation": map[string]any{
					"project_package": map[string]any{
						"tracks": []map[string]any{{
							"id":       "track_1",
							"gain_db":  0.0,
							"level_db": -24.0,
							"peak_db":  -8.0,
						}},
					},
				},
			},
		},
	})

	rows := contextBuild.Rows("tracks")
	if len(rows) != 1 {
		t.Fatalf("merged rows = %+v", rows)
	}
	row := rows[0]
	if row.Key != "track_1" {
		t.Fatalf("row key = %q", row.Key)
	}
	if row.Data["track_id"] != "track_1" || row.Data["track_name"] != "Track One" {
		t.Fatalf("identity aliases were not normalized: %+v", row.Data)
	}
	if row.Data["volume_db"] != -60.0 {
		t.Fatalf("project.state fader alias should win over stale mix gain: %+v", row.Data)
	}
	if row.Data["rms_dbfs"] != -24.0 || row.Data["peak_dbfs"] != -8.0 {
		t.Fatalf("acoustic aliases were not normalized: %+v", row.Data)
	}
}
