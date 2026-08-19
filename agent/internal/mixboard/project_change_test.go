package mixboard

import "testing"

func TestMixBoardPersistsBoundedProjectChangeWindow(t *testing.T) {
	change := map[string]any{
		"schema_version": "project_change_receipt.v1",
		"change_id":      "shadow_change_000012",
		"source":         "executor_delta",
		"authoritative":  false,
		"freshness":      "pending_authoritative_refresh",
		"affected_scopes": []any{
			"track.level", "mix.multitrack_relationship",
		},
	}
	window := []map[string]any{change, {"change_id": "shadow_change_000011"}, {"change_id": "shadow_change_000010"}, {"change_id": "shadow_change_000009"}, {"change_id": "shadow_change_000008"}}
	req := Request{
		MixSessionID:  "mix-change-window",
		Round:         1,
		TargetRef:     TargetRef{Kind: "track", ID: "1007"},
		ProjectState:  map[string]any{"project_uuid": "p1", "project_revision": "2", "tracks": []any{}},
		ProjectChange: change,
		ChangeWindow:  window,
	}
	obs := BuildObservation(req, "2026-08-16T00:00:00Z")
	board := buildBoard(req, obs, "", "2026-08-16T00:00:00Z")
	pack := buildContextPack(req, board, obs, "2026-08-16T00:00:00Z")

	if obs.ProjectChange["change_id"] != "shadow_change_000012" || board.LatestChange["change_id"] != "shadow_change_000012" {
		t.Fatalf("change receipt not bound to observation/board: obs=%#v board=%#v", obs.ProjectChange, board.LatestChange)
	}
	latest := mapValue(pack.LatestObservation["project_change"])
	if latest["change_id"] != "shadow_change_000012" {
		t.Fatalf("context pack latest change = %#v", pack.LatestObservation)
	}
	if len(pack.ChangeWindow) != 4 || pack.ChangeWindow[0]["change_id"] != "shadow_change_000012" || pack.ChangeWindow[3]["change_id"] != "shadow_change_000009" {
		t.Fatalf("bounded change window = %#v", pack.ChangeWindow)
	}
}

func TestProjectChangeDeltaCatalogReadAndMissingContract(t *testing.T) {
	store := NewStore(t.TempDir())
	change := map[string]any{
		"schema_version":   "project_change_receipt.v1",
		"change_id":        "shadow_change_000021",
		"source":           "executor_delta",
		"authoritative":    false,
		"freshness":        "pending_authoritative_refresh",
		"from_state_epoch": 4,
		"to_state_epoch":   5,
		"affected_scopes":  []any{"track.level", "project.state"},
	}
	result, err := store.RequestObservation(Request{
		MixSessionID:  "mix-change-delta-read",
		TargetRef:     TargetRef{Kind: "track", ID: "1007"},
		ProjectState:  map[string]any{"project_uuid": "p1", "project_revision": 5, "tracks": []any{}},
		ProjectChange: change,
	})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, entry := range result.Observation.Catalog.Entries {
		if entry.Key == "project.change_delta" {
			found = true
			if entry.Kind != "project_change" || entry.Freshness != "partial" || entry.TargetKind != "project" {
				t.Fatalf("project change catalog entry = %+v", entry)
			}
		}
	}
	if !found {
		t.Fatalf("project.change_delta missing from catalog: %+v", result.Observation.Catalog)
	}
	read, err := store.Read(ReadRequest{
		MixSessionID:  "mix-change-delta-read",
		ObservationID: result.Observation.ObservationID,
		Keys:          []string{"project.change_delta"},
	})
	if err != nil {
		t.Fatal(err)
	}
	item := read["items"].(map[string]any)["project.change_delta"].(map[string]any)
	if item["change_id"] != "shadow_change_000021" || item["status"] != "partial" || item["authoritative"] != false {
		t.Fatalf("project change read = %#v", item)
	}

	missing, err := store.RequestObservation(Request{
		MixSessionID: "mix-change-delta-missing",
		TargetRef:    TargetRef{Kind: "track", ID: "1007"},
		ProjectState: map[string]any{"project_uuid": "p1", "project_revision": 5, "tracks": []any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	readMissing, err := store.Read(ReadRequest{
		MixSessionID:  "mix-change-delta-missing",
		ObservationID: missing.Observation.ObservationID,
		Keys:          []string{"project.change_delta"},
	})
	if err != nil {
		t.Fatal(err)
	}
	missingItem := readMissing["items"].(map[string]any)["project.change_delta"].(map[string]any)
	if missingItem["status"] != "missing" || missingItem["reason"] != "project_change_monitor_unavailable" {
		t.Fatalf("missing project change read = %#v", missingItem)
	}
}
