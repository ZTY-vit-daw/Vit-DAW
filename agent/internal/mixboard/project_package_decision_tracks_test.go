package mixboard

import (
	"math"
	"testing"
)

func TestProjectLowEndDecisionTracksAreNotLimitedByPresentationExcerpt(t *testing.T) {
	tracks := make([]map[string]any, 0, 6)
	for index := 0; index < 6; index++ {
		tracks = append(tracks, map[string]any{
			"track_id": "track-" + string(rune('a'+index)),
			"name":     "Track",
			"band_energy": map[string]any{
				"status": "ready",
				"bands": map[string]any{
					"bass": map[string]any{"unit_energy": float64(6-index) / 10},
				},
			},
		})
	}

	occupancy := projectBandOccupancy(tracks)
	bass := mapValue(mapValue(occupancy["bands"])["bass"])
	if got := len(mapRowsAny(bass["dominant_tracks"])); got != 3 {
		t.Fatalf("dominant_tracks = %d, want presentation cap 3", got)
	}
	if got := len(mapRowsAny(bass["decision_tracks"])); got != 6 {
		t.Fatalf("decision_tracks = %d, want all 6", got)
	}

	candidates := projectConflictCandidates(tracks)
	rows := mapRowsAny(candidates["candidates"])
	if len(rows) == 0 {
		t.Fatal("expected bass conflict candidate")
	}
	if got := len(mapRowsAny(rows[0]["tracks"])); got != 3 {
		t.Fatalf("candidate excerpt tracks = %d, want 3", got)
	}
	if _, exists := rows[0]["decision_tracks"]; exists {
		t.Fatalf("conflict candidate promoted the complete decision roster to conflict membership: %#v", rows[0])
	}
}

func TestProjectPackageExposesB2EffectiveStaticLevel(t *testing.T) {
	packet := buildProjectPackage(map[string]any{"tracks": []any{map[string]any{
		"track_id": "vocal", "track_name": "Lead Vocal", "volume_db": -2.0,
		"rms_dbfs": -20.0, "peak_dbfs": -6.0,
		"clips": []any{map[string]any{"clip_id": "clip-v", "length_seconds": 10.0, "clip_gain_db": 1.0}},
	}}}, TargetRef{}, ListenScope{}, featureSnapshot{})
	tracks := mapRowsAny(packet["tracks"])
	if len(tracks) != 1 {
		t.Fatalf("project tracks=%#v", tracks)
	}
	effective, ok := numberField(tracks[0], "effective_static_rms_dbfs")
	if !ok || math.Abs(effective-(-21.0)) > 0.001 {
		t.Fatalf("effective static RMS=%v ok=%v track=%#v", effective, ok, tracks[0])
	}
	primary := mapValue(tracks[0]["primary_clip"])
	if gain, ok := numberField(primary, "clip_gain_db"); !ok || gain != 1.0 {
		t.Fatalf("primary clip gain missing from bounded projection: %#v", primary)
	}
}
