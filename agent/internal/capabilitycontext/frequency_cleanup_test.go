package capabilitycontext

import "testing"

func TestFrequencyCleanupPackKeepsWholeProjectRosterAndSeparateReadiness(t *testing.T) {
	tracks := []map[string]any{}
	for i := 0; i < 20; i++ {
		tracks = append(tracks, map[string]any{"track_id": string(rune('a' + i)), "name": "track", "status": "ready", "freshness": "fresh", "tap_point": "source_file_pre_fx", "bands": map[string]any{"mid": map[string]any{"unit_energy": .4}}, "evidence_ref": "dad"})
	}
	projection := map[string]any{"observation_id": "obs", "frequency_relationship": map[string]any{"schema_version": "mom.frequency_relationship.v1", "status": "ready", "freshness": "fresh", "project_cut_ref": "cut", "scope": map[string]any{"project_id": "p", "track_ids": func() []string {
		out := []string{}
		for _, r := range tracks {
			out = append(out, r["track_id"].(string))
		}
		return out
	}()}, "tap_point": "source_file_pre_fx", "coverage": map[string]any{"project_track_count": 20, "eligible_track_count": 20, "missing_track_count": 0, "eligible_track_ratio": 1.0, "supports_static_diagnosis": true, "supports_post_fx_compare": false}, "track_profiles": tracks, "persistence_summary": map[string]any{"status": "missing"}}}
	pack := BuildFrequencyCleanupPack(FrequencyCleanupInput{UserIntent: "clean", MOMProjection: projection, MixObservation: map[string]any{"observation_id": "obs"}, Budget: Budget{MaxDisclosedTracks: 3}})
	if pack.AnalyzedTrackCount != 20 || len(pack.Tracks) != 20 || len(pack.AnalysisDisclosure.Tracks) != 20 {
		t.Fatalf("C1 roster was truncated: %+v", pack)
	}
	if !pack.Readiness.Diagnosis.CanProceed || pack.Readiness.Mutation.CanProceed {
		t.Fatalf("unexpected readiness: %+v", pack.Readiness)
	}
	bundle := FrequencyCleanupOrchestrationBundle(pack, "cut-hash")
	if bundle.CapabilityID != FrequencyCleanupCapabilityID || bundle.Disclosure == "" || len(bundle.OmissionReasons) == 0 {
		t.Fatalf("unexpected bundle: %+v", bundle)
	}
}
