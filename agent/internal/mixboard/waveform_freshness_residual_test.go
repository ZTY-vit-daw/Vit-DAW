package mixboard

import (
	"testing"
)

// kernelPreparedWaveformRow mirrors the persisted kernel-telemetry waveform
// envelope row of the frozen spv1_p01 fixture (full field set the COM
// source-only consumer needs, four time segments included).
func kernelPreparedWaveformRow(trackID, clipID, filePath string) map[string]any {
	segments := []any{}
	for index := 0; index < 4; index++ {
		segments = append(segments, map[string]any{
			"start_seconds": float64(index * 5), "end_seconds": float64(index*5 + 5),
			"rms": 0.05 + 0.01*float64(index), "rms_dbfs": -26.0 + float64(index),
			"peak_abs": 0.3, "peak_dbfs": -10.5, "crest_db": 15.5, "energy_state": "medium",
		})
	}
	return map[string]any{
		"status": "ready", "source": "kernel_prepared_telemetry",
		"track_id": trackID, "clip_id": clipID,
		"request_id": "kernel_prepared_waveform_envelope_" + clipID,
		"file_path": filePath, "source_path": filePath,
		"source_revision": filePath + "|size=3528044|length=20.0000",
		"clip_revision":   "clip=" + clipID + "|track=" + trackID + "|source=" + filePath,
		"sample_rate": 44100.0, "channel_count": 2, "channel_layout": "stereo",
		"duration_seconds": 20.0, "total_duration": 20.0, "analyzed_sample_count": 1764000,
		"nonzero_count": 12000, "sum_abs": 1194.8, "coverage_ratio": 1.0,
		"rms": 0.09, "peak_abs": 0.54, "rms_dbfs": -20.9, "peak_dbfs": -5.3, "crest_db": 15.6,
		"quality_status": "ready", "evidence_ref": "dad:waveform:" + trackID,
		"analyzer_revision": "audio_feature.v1.2",
		"time_segments": segments,
	}
}

// l2RenderProbeLatestRequest mirrors the cross-run residual latest_request of
// the frozen fixture: an unfinished render probe whose requested_features list
// explicitly excludes waveform_envelope (2026-08-29 s2f2a forensics §3).
func l2RenderProbeLatestRequest() map[string]any {
	const probeID = "mixboard_l2_render_probe_1032_20260830T022836.175702300"
	return map[string]any{
		"schema_version": "mixboard_feature_request.v1",
		"request_id":     probeID,
		"status":         "requested",
		"lifecycle":      "l2_render_probe_ab",
		"project_id":     "vitproj_fixture",
		"requested_features": []any{map[string]any{
			"feature_type": "l2_render_probe", "request_id": probeID,
			"track_id": "1032", "clip_id": "1036", "tap_point": "track_post_fader", "render_mode": "offline_probe",
		}},
		"resolved_target": map[string]any{"track_id": "1032", "clip_id": "1036", "track_name": "vocals"},
		"target_ref":      map[string]any{"kind": "track", "id": "1032"},
	}
}

func twoTrackFixtureProjectState() map[string]any {
	return map[string]any{
		"duration_seconds": 20,
		"tracks": []any{
			map[string]any{"track_id": "1012", "track_name": "drums", "clips": []any{
				map[string]any{"clip_id": "1016", "length_seconds": 20, "file_path": "D:/fixture/stems/drums.wav"},
			}},
			map[string]any{"track_id": "1032", "track_name": "vocals", "clips": []any{
				map[string]any{"clip_id": "1036", "length_seconds": 20, "file_path": "D:/fixture/stems/vocals.wav"},
			}},
		},
	}
}

// TestNonWaveformLatestRequestKeepsReadyWaveformRows pins the S2f-2a fix: a
// persisted latest_request that explicitly requests no waveform feature (the
// cross-run l2 render probe) must not invalidate ready waveform rows for the
// observed target. The WE slot binds to the requested target from the retained
// track rows, time_energy becomes ready, and the source-only COM projection
// reaches ready on the same observation.
func TestNonWaveformLatestRequestKeepsReadyWaveformRows(t *testing.T) {
	vocalsRow := kernelPreparedWaveformRow("1032", "1036", "D:/fixture/stems/vocals.wav")
	drumsRow := kernelPreparedWaveformRow("1012", "1016", "D:/fixture/stems/drums.wav")
	snapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"latest_request": l2RenderProbeLatestRequest(),
		// The WE slot still describes the probe's own target (vocals) from the
		// previous generation.
		"waveform_envelope":        vocalsRow,
		"track_waveform_envelopes": []any{vocalsRow, drumsRow},
		"spectrogram_tiles":        map[string]any{"status": "missing"},
	}
	observation := BuildObservation(Request{
		MixSessionID: "mix_nonwaveform_baseline",
		TargetRef:    TargetRef{Kind: "track", ID: "1012"},
		ProjectState: twoTrackFixtureProjectState(),
		Args:         map[string]any{"feature_snapshot": snapshot, "com_mode": "source_only"},
	}, "2026-08-30T12:00:00Z")

	if got := observation.SourceCapabilities["waveform_envelope"]; got != "ready" {
		t.Fatalf("waveform capability = %q, snapshot state: %#v", got, observation.GlobalSummary["feature_snapshot"])
	}
	mixCaps, _ := observation.MixPackage["source_capabilities"].(map[string]string)
	if mixCaps["time_energy"] != "ready" {
		t.Fatalf("time_energy capability = %q, want ready", mixCaps["time_energy"])
	}
	featureSnap, _ := observation.GlobalSummary["feature_snapshot"].(map[string]any)
	waveform, _ := featureSnap["waveform_envelope"].(map[string]any)
	if cleanAnyString(waveform["track_id"]) != "1012" {
		t.Fatalf("WE slot did not bind to the requested target: %#v", waveform)
	}
	if len(waveformTimeSegments(waveform)) != 4 {
		t.Fatalf("WE slot time segments = %d, want 4", len(waveformTimeSegments(waveform)))
	}
	if rows := mapRowsAny(featureSnap["track_waveform_envelopes"]); len(rows) != 2 {
		t.Fatalf("ready track rows were dropped under a non-waveform baseline: %#v", rows)
	}
	projection := observation.COMProjection
	if projection == nil || projection.SourceDynamics == nil {
		t.Fatalf("COM source projection missing: %+v", projection)
	}
	if projection.SourceDynamics.Status != "ready" {
		t.Fatalf("COM source dynamics status = %q, want ready (projection %+v)", projection.SourceDynamics.Status, projection)
	}
}

// TestNonWaveformLatestRequestStubsForeignWaveformEnvelope pins the
// misattribution guard: with no usable row for the requested target, a WE slot
// describing another target becomes an honest missing stub instead of silently
// serving the foreign track's dynamics as source evidence.
func TestNonWaveformLatestRequestStubsForeignWaveformEnvelope(t *testing.T) {
	vocalsRow := kernelPreparedWaveformRow("1032", "1036", "D:/fixture/stems/vocals.wav")
	snapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"latest_request": l2RenderProbeLatestRequest(),
		"waveform_envelope":        vocalsRow,
		"track_waveform_envelopes": []any{vocalsRow},
		"spectrogram_tiles":        map[string]any{"status": "missing"},
	}
	observation := BuildObservation(Request{
		MixSessionID: "mix_nonwaveform_baseline_unbound",
		TargetRef:    TargetRef{Kind: "track", ID: "1012"},
		ProjectState: twoTrackFixtureProjectState(),
		Args:         map[string]any{"feature_snapshot": snapshot, "com_mode": "source_only"},
	}, "2026-08-30T12:00:00Z")

	featureSnap, _ := observation.GlobalSummary["feature_snapshot"].(map[string]any)
	waveform, _ := featureSnap["waveform_envelope"].(map[string]any)
	if featureStatus(waveform) != "missing" || cleanAnyString(waveform["reason"]) != "waveform_row_not_bound_to_requested_target" {
		t.Fatalf("foreign WE slot was not stubbed honestly: %#v", waveform)
	}
	if cleanAnyString(waveform["track_id"]) != "1012" {
		t.Fatalf("unbound WE stub does not carry the requested target: %#v", waveform)
	}
	projection := observation.COMProjection
	if projection == nil || projection.SourceDynamics == nil || projection.SourceDynamics.Status == "ready" {
		t.Fatalf("foreign-target row leaked into COM source evidence: %+v", projection)
	}
}

// TestStaleWaveformStubRecordsJudgedRowDiagnostics pins the S2f-2a §4-6
// diagnostic: the stale-row stub embeds the judged row's identity so the next
// D1 artifacts settle which row the freshness gate actually condemned.
func TestStaleWaveformStubRecordsJudgedRowDiagnostics(t *testing.T) {
	foreignRow := kernelPreparedWaveformRow("track_2", "clip_2", "D:/fixture/stems/other.wav")
	snapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"latest_request": map[string]any{
			"request_id": "req_current",
			"status":     "requested",
			"requested_features": []any{map[string]any{
				"feature_type": "waveform_envelope", "request_id": "req_current", "track_id": "track_1", "clip_id": "clip_1",
			}},
			"resolved_target": map[string]any{"track_id": "track_1", "clip_id": "clip_1"},
		},
		"waveform_envelope":        foreignRow,
		"track_waveform_envelopes": []any{foreignRow},
		"spectrogram_tiles":        map[string]any{"status": "missing"},
	}
	observation := BuildObservation(Request{
		MixSessionID: "mix_waveform_baseline_stale",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{
			"duration_seconds": 20,
			"tracks": []any{map[string]any{"track_id": "track_1", "clips": []any{
				map[string]any{"clip_id": "clip_1", "length_seconds": 20, "file_path": "D:/fixture/stems/drums.wav"},
			}}},
		},
		Args: map[string]any{"feature_snapshot": snapshot},
	}, "2026-08-30T12:00:00Z")

	featureSnap, _ := observation.GlobalSummary["feature_snapshot"].(map[string]any)
	waveform, _ := featureSnap["waveform_envelope"].(map[string]any)
	if featureStatus(waveform) != "missing" || cleanAnyString(waveform["reason"]) != "stale_feature_snapshot_for_current_request" {
		t.Fatalf("stale WE downgrade missing: %#v", waveform)
	}
	if cleanAnyString(waveform["request_id"]) != "req_current" {
		t.Fatalf("stale stub baseline identity lost: %#v", waveform)
	}
	if cleanAnyString(waveform["judged_track_id"]) != "track_2" ||
		cleanAnyString(waveform["judged_clip_id"]) != "clip_2" ||
		cleanAnyString(waveform["judged_request_id"]) != "kernel_prepared_waveform_envelope_clip_2" ||
		cleanAnyString(waveform["judged_status"]) != "ready" {
		t.Fatalf("stale stub lacks judged-row diagnostics: %#v", waveform)
	}
}
