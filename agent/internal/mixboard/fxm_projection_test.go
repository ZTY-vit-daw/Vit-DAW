package mixboard

import "testing"

func TestObservationConnectsFXMProjectionAndCatalog(t *testing.T) {
	measurement := map[string]any{
		"chain":      map[string]any{"chain_hash": "chain-1", "plugins": []any{map[string]any{"order": 1, "name": "TDR Nova", "vps_id": "vps-nova"}}},
		"conditions": map[string]any{"source_revision": "source-1", "start_seconds": 0, "end_seconds": 10, "sample_rate": 48000, "channel_count": 2},
		"baseline":   map[string]any{"id": "baseline", "stage": "bypass_chain", "status": "ready", "source_revision": "source-1", "window": map[string]any{"start_seconds": 0, "end_seconds": 10, "sample_rate": 48000, "channel_count": 2}, "rms_dbfs": -18.0, "quality": map[string]any{"deterministic": true, "latency_compensated": true, "nonzero": true, "coverage": 1}},
		"processed":  map[string]any{"id": "processed", "stage": "processed_chain", "status": "ready", "source_revision": "source-1", "window": map[string]any{"start_seconds": 0, "end_seconds": 10, "sample_rate": 48000, "channel_count": 2}, "rms_dbfs": -16.5, "quality": map[string]any{"deterministic": true, "latency_compensated": true, "nonzero": true, "coverage": 1}},
	}
	obs := BuildObservation(Request{MixSessionID: "mix-fxm", TargetRef: TargetRef{Kind: "track", ID: "1007"}, Args: map[string]any{"fxm_measurement": measurement}}, "2026-07-17T00:00:00Z")
	if obs.FXMProjection == nil || obs.FXMProjection.Status != "ready" {
		t.Fatalf("FXM projection = %+v", obs.FXMProjection)
	}
	value, ok := readObservationKey(obs, "observation.fxm_projection", ReadRequest{})
	if !ok || value == nil {
		t.Fatalf("FXM read key missing: ok=%t value=%#v", ok, value)
	}
	found := false
	for _, entry := range obs.Catalog.Entries {
		if entry.Key == "observation.fxm_projection" && entry.Freshness == "fresh" {
			found = true
		}
	}
	if !found {
		t.Fatalf("FXM catalog entry missing: %+v", obs.Catalog.Entries)
	}
}

func TestObservationPublishesMissingFXMWithoutInventingEffectData(t *testing.T) {
	obs := BuildObservation(Request{MixSessionID: "mix-no-fxm", TargetRef: TargetRef{Kind: "track", ID: "1007"}, Args: map[string]any{}}, "2026-07-17T00:00:00Z")
	if obs.FXMProjection == nil || obs.FXMProjection.Status != "missing" || obs.FXMProjection.TrustQuality.CanSupportSuggestion {
		t.Fatalf("missing FXM projection should fail closed: %+v", obs.FXMProjection)
	}
}
