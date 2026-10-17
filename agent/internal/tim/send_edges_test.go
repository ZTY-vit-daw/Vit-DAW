package tim

import (
	"strings"
	"testing"
)

// AGENT-SENDROUTE-SEMANTICS-1 验收：TIM send 边悬空断言（设计件 §3 TIM 行）。

func TestEvaluateSendEdgesDanglingFailsWithRepairSuggestion(t *testing.T) {
	edges := []SendEdgeFact{
		{SourceTrackID: "T1", SourceName: "vocals", FXTrackID: "FX1", FXName: "Reverb Bus", AmountDB: -6.0},
		{SourceTrackID: "T9", SourceName: "old", FXTrackID: "", FXName: "", AmountDB: -6.0, Dangling: true},
	}
	rows := Evaluate(AssertInput{SendEdges: edges, ObservationID: "obs_tim_send"})
	sendRows := []AssertionResult{}
	for _, row := range rows {
		if row.Asserter == "send_edges" {
			sendRows = append(sendRows, row)
		}
	}
	if len(sendRows) != 2 {
		t.Fatalf("send edge rows = %d, want 2 (healthy + dangling)", len(sendRows))
	}
	byTrack := map[string]AssertionResult{}
	for _, row := range sendRows {
		byTrack[row.TrackID] = row
	}
	if byTrack["T1"].Status != AssertionStatusPass {
		t.Fatalf("healthy edge should pass, got %s", byTrack["T1"].Status)
	}
	dangling := byTrack["T9"]
	if dangling.Status != AssertionStatusFail {
		t.Fatalf("dangling edge should fail, got %s", dangling.Status)
	}
	if dangling.Code != codeSendEdgeDangling {
		t.Fatalf("dangling code = %s", dangling.Code)
	}
	if !strings.Contains(dangling.Detail, "track.send.disconnect") {
		t.Fatalf("repair suggestion missing from detail: %q", dangling.Detail)
	}
}

func TestEvaluateSendEdgesMissingReadStaysNotEvaluable(t *testing.T) {
	rows := Evaluate(AssertInput{SendEdges: nil, ObservationID: "obs_tim_send"})
	for _, row := range rows {
		if row.Asserter == "send_edges" {
			if row.Status != AssertionStatusNotEvaluable {
				t.Fatalf("missing read must stay not_evaluable, got %s", row.Status)
			}
			if row.Code != codeSendEdgeReadMissing {
				t.Fatalf("missing read code = %s", row.Code)
			}
			return
		}
	}
	t.Fatal("send_edges asserter did not run")
}

func TestSendEdgesFromTopologyParsesKernelReplyShape(t *testing.T) {
	topology := map[string]any{
		"sends": []any{
			map[string]any{"source_track_id": "T1", "fx_track_id": "FX1", "amount_db": -6.0, "send_bus": 0, "return_bus": 1},
		},
		"dangling": []any{
			map[string]any{"source_track_id": "T2", "source_name": "guitar", "amount_db": -6.0, "send_bus": 2},
		},
		"fx_tracks": []any{
			map[string]any{"track_id": "FX1", "name": "Reverb Bus", "send_bus": 0, "return_bus": 1, "source_count": 1},
		},
	}
	edges := SendEdgesFromTopology(topology)
	if len(edges) != 2 {
		t.Fatalf("edges = %d, want 2", len(edges))
	}
	if edges[0].Dangling || edges[0].FXTrackID != "FX1" || edges[0].AmountDB != -6.0 {
		t.Fatalf("healthy edge parsed wrong: %+v", edges[0])
	}
	if !edges[1].Dangling || edges[1].SourceName != "guitar" {
		t.Fatalf("dangling edge parsed wrong: %+v", edges[1])
	}
	if got := SendEdgesFromTopology(nil); got != nil {
		t.Fatalf("nil topology must yield nil facts, got %+v", got)
	}
	if got := SendEdgesFromTopology(map[string]any{"status": "ok"}); got == nil || len(got) != 0 {
		t.Fatalf("explicit empty topology must yield non-nil empty facts, got %+v", got)
	}
}
