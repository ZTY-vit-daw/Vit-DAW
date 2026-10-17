package tom

import (
	"testing"
)

// AGENT-SENDROUTE-SEMANTICS-1 验收：TOM v0_3 轨行 fx_role+send_edge_count，
// 旧字段不动；无 send 拓扑输入时保持 v0_2 形态（新字段省略）。

func TestBuildFromImportRowsStampsSendTopologyFacts(t *testing.T) {
	input := ImportInput{
		ObservationID: "obs_tom_send",
		SendTopology: map[string]any{
			"sends": []any{
				map[string]any{"source_track_id": "T1", "fx_track_id": "FX1", "amount_db": -6.0, "send_bus": 0, "return_bus": 1},
				map[string]any{"source_track_id": "T2", "fx_track_id": "FX1", "amount_db": -6.0, "send_bus": 0, "return_bus": 1},
			},
			"dangling": []any{},
			"fx_tracks": []any{
				map[string]any{"track_id": "FX9", "name": "Reverb Bus", "send_bus": 0, "return_bus": 1, "source_count": 2},
			},
		},
		Rows: []map[string]any{
			{"track_id": "T1", "track_name": "vocals", "clip_name": "vocals.wav"},
			{"track_id": "T2", "track_name": "guitar", "clip_name": "gtr.wav"},
			{"track_id": "FX9", "track_name": "Reverb Bus", "clip_name": ""},
		},
	}
	proj := BuildFromImportRows(input)
	if proj.SchemaVersion != "tom.projection.v0_3" || proj.TOMVersion != "v0.3" {
		t.Fatalf("schema/version = %s / %s", proj.SchemaVersion, proj.TOMVersion)
	}
	fxRole := map[string]string{}
	sendCount := map[string]int{}
	for _, group := range proj.GroupProposals {
		for _, assignment := range group.Assignments {
			fxRole[assignment.TrackID] = assignment.FxRole
			sendCount[assignment.TrackID] = assignment.SendEdgeCount
		}
	}
	if fxRole["FX9"] != "send_return_fx" {
		t.Fatalf("FX track must carry fx_role=send_return_fx, got %q", fxRole["FX9"])
	}
	if fxRole["T1"] != "" || fxRole["T2"] != "" {
		t.Fatalf("source tracks must have empty fx_role, got %q/%q", fxRole["T1"], fxRole["T2"])
	}
	if sendCount["T1"] != 1 || sendCount["T2"] != 1 {
		t.Fatalf("send_edge_count = %d/%d, want 1/1", sendCount["T1"], sendCount["T2"])
	}
	if sendCount["FX9"] != 0 {
		t.Fatalf("FX track carries no outgoing send edges, got %d", sendCount["FX9"])
	}
	// 旧字段仍按 v0_2 语义填充（轨名/组归属）。
	found := false
	for _, group := range proj.GroupProposals {
		for _, assignment := range group.Assignments {
			if assignment.TrackID == "T1" && assignment.TrackName == "vocals" && assignment.GroupID != "" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("legacy assignment fields must stay populated")
	}
}

func TestBuildFromImportRowsWithoutSendTopologyKeepsV02Shape(t *testing.T) {
	proj := BuildFromImportRows(ImportInput{
		ObservationID: "obs_tom_plain",
		Rows:          []map[string]any{{"track_id": "T1", "track_name": "vocals", "clip_name": "v.wav"}},
	})
	if proj.SchemaVersion != "tom.projection.v0_3" {
		t.Fatalf("schema = %s", proj.SchemaVersion)
	}
	for _, group := range proj.GroupProposals {
		for _, assignment := range group.Assignments {
			if assignment.FxRole != "" || assignment.SendEdgeCount != 0 {
				t.Fatalf("absent topology must keep fields zero-valued, got %+v", assignment)
			}
		}
	}
}
