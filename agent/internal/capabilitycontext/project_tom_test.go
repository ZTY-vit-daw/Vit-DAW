package capabilitycontext

import (
	"testing"
	"time"

	"vit-daw-agent/internal/mixstyle"
)

func TestProjectTOMProjectionRestoresB2RoleCoverageWithoutCreatingAuthority(t *testing.T) {
	tracks := []any{
		map[string]any{"track_id": "folder", "track_name": "Drums Folder", "track_type": "folder", "is_folder_track": true},
		projectTOMTestTrack("v", "Lead Vocal", -18),
		projectTOMTestTrack("bv", "Hook BGV 1", -20),
		projectTOMTestTrack("k", "Kick In", -16),
		projectTOMTestTrack("s", "Snare", -17),
		projectTOMTestTrack("b", "Sub Bass", -19),
		projectTOMTestTrack("g", "Guitar Double 1", -21),
		projectTOMTestTrack("sy", "Cloud Synth Pad", -22),
		projectTOMTestTrack("fx", "FX Riser", -24),
	}
	projectState := map[string]any{"tracks": tracks}
	projection := BuildProjectTOMProjection(projectState, "obs_roles", "session_roles")
	manifest := mapValue(projection["full_assignment_manifest"])
	if manifest["coverage_status"] != "complete" || len(rowsValue(manifest["groups"])) < 4 {
		t.Fatalf("rebuilt TOM projection = %#v", projection)
	}

	compared := make([]any, 0, len(tracks)-1)
	for _, value := range tracks[1:] {
		track := mapValue(value)
		compared = append(compared, map[string]any{
			"track_id": track["track_id"], "rms_dbfs": track["rms_dbfs"], "peak_dbfs": -8.0,
		})
	}
	pack := BuildStaticBalancePack(StaticBalanceInput{
		UserIntent: "B2 static balance", ProjectState: projectState, TOMProjection: projection,
		MOMProjection: map[string]any{
			"mom_version": "v1.4", "trust_quality": map[string]any{"can_support_action_preflight": true},
			"multitrack_relation": map[string]any{"status": "ready", "compared_tracks": compared},
		},
		Style: mixstyle.Default(), GeneratedAt: time.Unix(1700000000, 0),
	})
	if !pack.Readiness.CanProceed || pack.Admission.Coverage.RoleCoverage < 0.95 || pack.Admission.Coverage.EligibleTrackCount != 8 {
		t.Fatalf("rebuilt TOM did not satisfy B2 role readiness: readiness=%#v coverage=%#v", pack.Readiness, pack.Admission.Coverage)
	}
	if len(pack.Candidates) == 0 {
		t.Fatalf("rebuilt TOM produced no deterministic B2 candidates: limitations=%#v", pack.Limitations)
	}
}

func projectTOMTestTrack(id, name string, rms float64) map[string]any {
	return map[string]any{
		"track_id": id, "track_name": name, "track_type": "audio", "is_audio_track": true,
		"volume_db": 0.0, "rms_dbfs": rms, "peak_dbfs": -8.0,
		"clips": []any{map[string]any{"clip_id": "clip_" + id, "name": name, "length_seconds": 120.0, "channel_count": 2}},
	}
}
