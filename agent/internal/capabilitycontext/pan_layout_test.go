package capabilitycontext

import (
	"testing"
	"time"
	"vit-daw-agent/internal/mixstyle"
)

func TestPanLayoutPackAnalyzesAllTracksButBoundsDisclosure(t *testing.T) {
	tracks := []any{}
	groups := []any{}
	for i := 0; i < 61; i++ {
		id := string(rune('a'+i%26)) + string(rune('A'+i/26))
		name := "Guitar " + id
		tracks = append(tracks, map[string]any{"track_id": id, "track_name": name, "track_type": "audio", "is_audio_track": true, "pan": 0.0, "channel_count": 1})
		groups = append(groups, map[string]any{"role": "guitar", "confidence": "high", "track_ids": []any{id}})
	}
	pack := BuildPanLayoutPack(PanLayoutInput{UserIntent: "B3", ProjectState: map[string]any{"tracks": tracks}, MOMProjection: map[string]any{"multitrack_relation": map[string]any{"status": "ready"}}, TOMProjection: map[string]any{"full_assignment_manifest": map[string]any{"groups": groups}}, Style: mixstyle.Default(), GeneratedAt: time.Unix(1, 0), Budget: Budget{MaxDisclosedTracks: 10, MaxRankingRows: 4, MaxStringRunes: 96}})
	if pack.AnalyzedTrackCount != 61 || pack.DisclosedTrackCount != 10 {
		t.Fatalf("counts analyzed=%d disclosed=%d", pack.AnalyzedTrackCount, pack.DisclosedTrackCount)
	}
	if len(pack.Result().Candidates) == 0 {
		t.Fatalf("pack readiness=%+v", pack.Readiness)
	}
}
