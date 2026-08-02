package mixboard

import (
	"math"
	"testing"
)

func TestProjectStaticLevelAggregatesAllClipsInLinearEnergy(t *testing.T) {
	state := map[string]any{"snapshot_hash": "cut-1", "tracks": []any{map[string]any{
		"track_id": "track-1", "track_name": "Vocal", "volume_db": 0.0,
		"clips": []any{
			map[string]any{"clip_id": "clip-a", "source_path": "vocal.wav", "length_seconds": 10.0, "clip_gain_db": 0.0},
			map[string]any{"clip_id": "clip-b", "source_path": "vocal-b.wav", "length_seconds": 10.0, "clip_gain_db": 6.0},
		},
	}}}
	snap := featureSnapshot{TrackWaveformEnvelopes: []map[string]any{
		{"status": "ready", "track_id": "track-1", "clip_id": "clip-a", "source_path": "vocal.wav", "rms_dbfs": -20.0, "peak_dbfs": -5.0, "duration_seconds": 10.0},
		{"status": "ready", "track_id": "track-1", "clip_id": "clip-b", "source_path": "vocal-b.wav", "rms_dbfs": -20.0, "peak_dbfs": -5.0, "duration_seconds": 10.0},
	}}
	packet := buildProjectPackage(state, TargetRef{}, ListenScope{}, snap)
	inputs := mapValue(packet["static_level_relationship_inputs"])
	rows := mapRowsAny(inputs["tracks"])
	if len(rows) != 1 || featureStatus(rows[0]) != "ready" {
		t.Fatalf("static level rows = %#v", rows)
	}
	got, ok := numberField(rows[0], "effective_static_rms_dbfs")
	want := 10 * math.Log10((math.Pow(10, -20.0/10)+math.Pow(10, -14.0/10))/2)
	if !ok || math.Abs(got-want) > 0.001 {
		t.Fatalf("effective RMS = %.6f ok=%v want %.6f row=%#v", got, ok, want, rows[0])
	}
	peak, _ := numberField(rows[0], "effective_static_peak_dbfs")
	if math.Abs(peak-1.0) > 0.001 {
		t.Fatalf("effective peak = %.3f row=%#v", peak, rows[0])
	}
}

func TestProjectStaticLevelExcludesMutedClipsFromAggregation(t *testing.T) {
	state := map[string]any{"tracks": []any{map[string]any{
		"track_id": "track-1", "volume_db": -2.0,
		"clips": []any{
			map[string]any{"clip_id": "clip-active", "source_path": "active.wav", "length_seconds": 10.0, "clip_gain_db": 3.0},
			map[string]any{"clip_id": "clip-muted", "source_path": "muted.wav", "length_seconds": 10.0, "clip_gain_db": 12.0, "clip_mute": true},
		},
	}}}
	snap := featureSnapshot{TrackWaveformEnvelopes: []map[string]any{
		{"status": "ready", "track_id": "track-1", "clip_id": "clip-active", "source_path": "active.wav", "rms_dbfs": -24.0, "peak_dbfs": -8.0},
		{"status": "ready", "track_id": "track-1", "clip_id": "clip-muted", "source_path": "muted.wav", "rms_dbfs": -6.0, "peak_dbfs": -1.0},
	}}
	packet := buildProjectPackage(state, TargetRef{}, ListenScope{}, snap)
	rows := mapRowsAny(mapValue(packet["static_level_relationship_inputs"])["tracks"])
	if len(rows) != 1 || featureStatus(rows[0]) != "ready" {
		t.Fatalf("static level rows = %#v", rows)
	}
	got, _ := numberField(rows[0], "effective_static_rms_dbfs")
	if math.Abs(got-(-23.0)) > 0.001 {
		t.Fatalf("muted clip affected effective RMS: got %.3f row=%#v", got, rows[0])
	}
	ids := removeStringFromAnySlice(rows[0]["included_clip_ids"], "")
	if len(ids) != 1 || ids[0] != "clip-active" {
		t.Fatalf("included clips = %#v, want only active clip", ids)
	}
}

func TestProjectStaticLevelReusesSourceMetricForHomogeneousSplitClips(t *testing.T) {
	state := map[string]any{"tracks": []any{map[string]any{
		"track_id": "track-1", "volume_db": 0.0,
		"clips": []any{
			map[string]any{"clip_id": "clip-a", "source_path": "same.wav", "length_seconds": 8.0, "clip_gain_db": 3.0},
			map[string]any{"clip_id": "clip-b", "source_path": "same.wav", "length_seconds": 2.0, "clip_gain_db": 3.0},
		},
	}}}
	snap := featureSnapshot{TrackWaveformEnvelopes: []map[string]any{
		{"status": "ready", "track_id": "track-1", "clip_id": "clip-a", "source_path": "same.wav", "rms_dbfs": -24.0, "peak_dbfs": -8.0},
	}}
	packet := buildProjectPackage(state, TargetRef{}, ListenScope{}, snap)
	rows := mapRowsAny(mapValue(packet["static_level_relationship_inputs"])["tracks"])
	if len(rows) != 1 || featureStatus(rows[0]) != "approximate" {
		t.Fatalf("split projection = %#v", rows)
	}
	got, _ := numberField(rows[0], "effective_static_rms_dbfs")
	if math.Abs(got-(-21.0)) > 0.001 {
		t.Fatalf("split effective RMS = %.3f row=%#v", got, rows[0])
	}
	ids := removeStringFromAnySlice(rows[0]["included_clip_ids"], "")
	if len(ids) != 2 || ids[0] != "clip-a" || ids[1] != "clip-b" {
		t.Fatalf("included clips = %#v", ids)
	}
}

func TestProjectStaticLevelMarksHeterogeneousMissingClipPartial(t *testing.T) {
	state := map[string]any{"tracks": []any{map[string]any{
		"track_id": "track-1", "volume_db": 0.0,
		"clips": []any{
			map[string]any{"clip_id": "clip-a", "source_path": "a.wav", "length_seconds": 8.0},
			map[string]any{"clip_id": "clip-b", "source_path": "b.wav", "length_seconds": 2.0},
		},
	}}}
	snap := featureSnapshot{TrackWaveformEnvelopes: []map[string]any{
		{"status": "ready", "track_id": "track-1", "clip_id": "clip-a", "source_path": "a.wav", "rms_dbfs": -24.0, "peak_dbfs": -8.0},
	}}
	packet := buildProjectPackage(state, TargetRef{}, ListenScope{}, snap)
	rows := mapRowsAny(mapValue(packet["static_level_relationship_inputs"])["tracks"])
	if len(rows) != 1 || featureStatus(rows[0]) != "partial" {
		t.Fatalf("heterogeneous projection = %#v", rows)
	}
	missing := removeStringFromAnySlice(rows[0]["missing_clip_ids"], "")
	if len(missing) != 1 || missing[0] != "clip-b" {
		t.Fatalf("missing clips = %#v", missing)
	}
}

func TestProjectStaticLevelRelationshipRollsUpReadyTracksAsReady(t *testing.T) {
	state := map[string]any{"snapshot_hash": "cut-ready", "tracks": []any{map[string]any{
		"track_id": "track-1", "volume_db": 0.0,
		"clips": []any{map[string]any{"clip_id": "clip-a", "source_path": "a.wav", "length_seconds": 4.0}},
	}}}
	snap := featureSnapshot{TrackWaveformEnvelopes: []map[string]any{
		{"status": "ready", "track_id": "track-1", "clip_id": "clip-a", "source_path": "a.wav", "rms_dbfs": -24.0, "peak_dbfs": -8.0},
	}}
	packet := buildProjectPackage(state, TargetRef{}, ListenScope{}, snap)
	relation := mapValue(packet["static_level_relationship_inputs"])
	if got := featureStatus(relation); got != "ready" {
		t.Fatalf("static relationship status = %q, want ready; relation=%#v", got, relation)
	}
}

func TestBuildObservationHydratesStaticLevelsFromProjectAnalysisManifest(t *testing.T) {
	state := map[string]any{
		"snapshot_hash": "cut-manifest",
		"tracks": []any{
			map[string]any{"track_id": "track-1", "track_name": "Lead", "volume_db": -2.0, "clips": []any{map[string]any{"clip_id": "clip-a", "source_path": "lead.wav", "length_seconds": 10.0}}},
			map[string]any{"track_id": "track-2", "track_name": "Drums", "volume_db": 0.0, "clips": []any{map[string]any{"clip_id": "clip-b", "source_path": "drums.wav", "length_seconds": 10.0}}},
		},
		"analysis_manifest": map[string]any{
			"schema_version": "vit_analysis_manifest.v1", "status": "ready",
			"l1_waveform_rows": []any{
				map[string]any{"status": "ready", "track_id": "track-1", "clip_id": "clip-a", "source_path": "lead.wav", "rms_dbfs": -24.0, "peak_dbfs": -8.0},
				map[string]any{"status": "ready", "track_id": "track-2", "clip_id": "clip-b", "source_path": "drums.wav", "rms_dbfs": -20.0, "peak_dbfs": -6.0},
			},
		},
	}
	obs := buildObservation(Request{
		MixSessionID: "manifest-b2", GoalText: "B2", ProjectState: state,
		TargetRef:   TargetRef{Kind: "project", ID: "current"},
		ListenScope: ListenScope{Source: ListenSourceScope{Mode: "full_project"}},
	}, "2026-08-02T00:00:00Z", featureSnapshot{})
	relation := obs.MOMProjection.StaticLevelRelationship
	if relation == nil {
		t.Fatal("analysis manifest did not produce a static-level relationship")
	}
	usable, _ := numberField(relation.Coverage, "usable_track_count")
	if relation.Status != "ready" || relation.ProjectCutRef != "cut-manifest" || usable != 2 {
		t.Fatalf("analysis manifest did not produce ready cut-bound static levels: %+v", relation)
	}
}

func TestProjectStaticLevelPreservesUntrustedSourceStatus(t *testing.T) {
	for _, status := range []string{"stale", "suspect"} {
		t.Run(status, func(t *testing.T) {
			state := map[string]any{"snapshot_hash": "cut-untrusted", "tracks": []any{map[string]any{
				"track_id": "track-1", "volume_db": 0.0,
				"clips": []any{map[string]any{"clip_id": "clip-a", "source_path": "a.wav", "length_seconds": 4.0}},
			}}}
			snap := featureSnapshot{TrackWaveformEnvelopes: []map[string]any{
				{"status": status, "track_id": "track-1", "clip_id": "clip-a", "source_path": "a.wav", "rms_dbfs": -24.0, "peak_dbfs": -8.0},
			}}
			packet := buildProjectPackage(state, TargetRef{}, ListenScope{}, snap)
			relation := mapValue(packet["static_level_relationship_inputs"])
			rows := mapRowsAny(relation["tracks"])
			if len(rows) != 1 || featureStatus(rows[0]) != status {
				t.Fatalf("%s track status was not preserved: %#v", status, rows)
			}
			if got := featureStatus(relation); got != status {
				t.Fatalf("static relationship status = %q, want %s; relation=%#v", got, status, relation)
			}
			if got, _ := numberField(relation, "usable_track_count"); got != 0 {
				t.Fatalf("%s evidence counted as usable: %#v", status, relation)
			}
		})
	}
}
