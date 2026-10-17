package mom

import (
	"testing"
)

// AGENT-SENDROUTE-SEMANTICS-1 验收：MOM send_topology 投影行 schema。
// 投影行={source→fx, amount_db, return_edge, status}（设计件 §3）。

func TestBuildSendTopologyProjectionRowSchema(t *testing.T) {
	input := Input{
		ObservationID: "obs_send_1",
		ProjectPackage: map[string]any{
			"project_state_hash": "cut_1",
			"send_topology_inputs": map[string]any{
				"schema_version": "mom.send_topology.v0_1",
				"status":         "ready",
				"sends": []any{
					map[string]any{
						"source_track_id": "T1", "source_name": "vocals",
						"fx_track_id": "FX1", "fx_name": "Reverb Bus",
						"amount_db": -6.0, "send_bus": 0, "return_bus": 1,
					},
					map[string]any{
						"source_track_id": "T2", "source_name": "guitar",
						"fx_track_id": "FX1", "fx_name": "Reverb Bus",
						"amount_db": -9.5, "send_bus": 0, "return_bus": 1,
					},
				},
				"fx_tracks": []any{
					map[string]any{"track_id": "FX1", "name": "Reverb Bus", "send_bus": 0, "return_bus": 1, "source_count": 2},
				},
				"dangling": []any{},
			},
		},
	}
	proj := Build(input).SendTopology
	if proj == nil {
		t.Fatal("send_topology projection missing for present inputs")
	}
	if proj.SchemaVersion != "mom.send_topology.v0_1" {
		t.Fatalf("schema version = %s", proj.SchemaVersion)
	}
	if len(proj.Edges) != 2 {
		t.Fatalf("edge count = %d, want 2", len(proj.Edges))
	}
	first := proj.Edges[0]
	if first.SourceTrackID != "T1" || first.FXTrackID != "FX1" {
		t.Fatalf("edge endpoints = %s -> %s", first.SourceTrackID, first.FXTrackID)
	}
	if first.AmountDB != -6.0 || first.SendBus != 0 || first.ReturnBus != 1 {
		t.Fatalf("edge amounts/buses = %+v", first)
	}
	if first.ReturnEdge != "fx_wet_return_to_source" {
		t.Fatalf("return_edge = %s", first.ReturnEdge)
	}
	if len(proj.FXTracks) != 1 || proj.FXTracks[0].SourceCount != 2 {
		t.Fatalf("fx track rows = %+v", proj.FXTracks)
	}
	if proj.Coverage["send_edge_count"] != 2 || proj.Coverage["fx_track_count"] != 1 {
		t.Fatalf("coverage = %+v", proj.Coverage)
	}
	for _, ref := range proj.EvidenceRefs {
		if ref == "kernel.track_send_list" {
			goto evidenceOK
		}
	}
	t.Fatal("evidence refs must point at the track.send.list read-back")
evidenceOK:
	if proj.Note == "" {
		t.Fatal("structural-note must be present (not effect semantics)")
	}
}

func TestBuildSendTopologyAbsentInputsKeepsProjectionNil(t *testing.T) {
	// 读源缺席（观察装配未注入）：投影不出现，不虚构 missing 行。
	if proj := Build(Input{ProjectPackage: map[string]any{}}).SendTopology; proj != nil {
		t.Fatalf("absent inputs must keep projection nil, got %+v", proj)
	}
}

func TestBuildSendTopologyDanglingEdgesPartial(t *testing.T) {
	input := Input{
		ObservationID: "obs_send_2",
		ProjectPackage: map[string]any{
			"send_topology_inputs": map[string]any{
				"status":   "partial",
				"sends":    []any{},
				"dangling": []any{map[string]any{"source_track_id": "T9", "fx_track_id": "", "amount_db": -6.0}},
				"fx_tracks": []any{
					map[string]any{"track_id": "FX1", "name": "Reverb Bus", "send_bus": 0, "return_bus": 1, "source_count": 0},
				},
			},
		},
	}
	proj := Build(input).SendTopology
	if proj == nil {
		t.Fatal("projection missing")
	}
	if proj.DanglingCount != 1 {
		t.Fatalf("dangling count = %d, want 1", proj.DanglingCount)
	}
	foundLimit := false
	for _, limit := range proj.Limitations {
		if limit == "dangling_send_edges:1" {
			foundLimit = true
		}
	}
	if !foundLimit {
		t.Fatalf("dangling limitation missing: %+v", proj.Limitations)
	}
}
