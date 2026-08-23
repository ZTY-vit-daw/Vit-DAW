package shadow

import (
	"testing"
)

func TestAuthoritativeSnapshotProducesProjectChangeReceipt(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"status": "ok", "project_uuid": "p1", "project_epoch": "e1", "project_revision": 1, "snapshot_hash": "h1",
		"tracks": []any{map[string]any{
			"track_id": "1007", "track_name": "Bass", "track_type": "hybrid", "is_audio_track": true,
			"gain_db": 0.0, "pan": 0.0,
		}},
	})
	p.Initialize(map[string]any{
		"status": "ok", "project_uuid": "p1", "project_epoch": "e1", "project_revision": 2, "snapshot_hash": "h2",
		"tracks": []any{map[string]any{
			"track_id": "1007", "track_name": "Bass", "track_type": "hybrid", "is_audio_track": true,
			"gain_db": -1.0, "pan": 0.25,
		}},
	})

	receipt := p.LatestChangeReceipt()
	if receipt == nil {
		t.Fatal("expected authoritative change receipt")
	}
	if receipt["schema_version"] != ProjectChangeReceiptSchema || receipt["source"] != ChangeSourceAuthoritativeSnapshot {
		t.Fatalf("receipt identity = %#v", receipt)
	}
	if receipt["authoritative"] != true || receipt["freshness"] != "current_snapshot" {
		t.Fatalf("receipt freshness = %#v", receipt)
	}
	entities := mapRowsFromChangeTest(receipt["changed_entities"])
	track := findChangeEntityTest(entities, "track", "1007")
	if track == nil {
		t.Fatalf("track change missing: %#v", receipt)
	}
	fields := mapChangeFieldsTest(track["fields"])
	if fields["gain_db"]["after"] != -1.0 || fields["pan"]["after"] != 0.25 {
		t.Fatalf("track fields = %#v", fields)
	}
	scopes := stringsFromChangeTest(receipt["affected_scopes"])
	if !containsChangeTest(scopes, "mix.multitrack_relationship") || !containsChangeTest(scopes, "mix.masking_relationship") {
		t.Fatalf("affected scopes = %#v", scopes)
	}
	if p.Summary()["state_epoch"] != int64(2) {
		t.Fatalf("state epoch = %#v", p.Summary()["state_epoch"])
	}
}

func TestExecutorDeltaReceiptRemainsPendingUntilAuthoritativeRefresh(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"status": "ok", "project_uuid": "p1", "project_epoch": "e1", "project_revision": 1,
		"tracks": []any{map[string]any{"track_id": "1007", "track_name": "Bass", "is_audio_track": true, "gain_db": 0.0}},
	})
	p.ApplyDeltaWithSource(map[string]any{
		"type": "delta_update", "target_uid": "1007", "action": "property_changed:volume_db", "value": -1.0,
	}, ChangeSourceExecutorDelta)

	receipt := p.LatestChangeReceipt()
	if receipt == nil || receipt["source"] != ChangeSourceExecutorDelta {
		t.Fatalf("executor receipt = %#v", receipt)
	}
	if receipt["authoritative"] != false || receipt["freshness"] != "pending_authoritative_refresh" {
		t.Fatalf("executor receipt freshness = %#v", receipt)
	}
	entities := mapRowsFromChangeTest(receipt["changed_entities"])
	track := findChangeEntityTest(entities, "track", "1007")
	fields := mapChangeFieldsTest(track["fields"])
	if fields["gain_db"]["before"] != 0.0 || fields["gain_db"]["after"] != -1.0 {
		t.Fatalf("executor gain field = %#v", fields["gain_db"])
	}
}

func TestAuthoritativeSnapshotConfirmsPendingDeltaWithoutAdditionalComparableDiff(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"status": "ok", "project_uuid": "p1", "project_epoch": "e1", "project_revision": 1,
		"tracks": []any{map[string]any{"track_id": "1007", "track_name": "Bass", "is_audio_track": true, "gain_db": 0.0}},
	})
	p.ApplyDeltaWithSource(map[string]any{
		"type": "delta_update", "target_uid": "1040", "action": "property_changed:state", "value": "changed",
	}, ChangeSourceTelemetryDelta)
	p.Initialize(map[string]any{
		"status": "ok", "project_uuid": "p1", "project_epoch": "e1", "project_revision": 1,
		"tracks": []any{map[string]any{"track_id": "1007", "track_name": "Bass", "is_audio_track": true, "gain_db": 0.0}},
	})

	receipt := p.LatestChangeReceipt()
	if receipt == nil {
		t.Fatal("expected reconciled change receipt")
	}
	if receipt["source"] != ChangeSourceTelemetryDelta || receipt["authoritative"] != true || receipt["freshness"] != "current_snapshot" {
		t.Fatalf("reconciled receipt = %#v", receipt)
	}
	if scopes := stringsFromChangeTest(receipt["unresolved_refresh_scopes"]); len(scopes) != 0 {
		t.Fatalf("unresolved scopes = %#v", scopes)
	}
}

func TestOpeningDifferentProjectResetsChangeBaseline(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"status": "ok", "project_uuid": "draft-project", "project_revision": 1,
		"tracks": []any{map[string]any{"track_id": "draft-track", "track_name": "Draft", "gain_db": 0.0}},
	})
	p.Initialize(map[string]any{
		"status": "ok", "project_uuid": "real-project", "project_revision": 7,
		"tracks": []any{map[string]any{"track_id": "1007", "track_name": "Bass", "gain_db": -1.0}},
	})
	if receipt := p.LatestChangeReceipt(); receipt != nil {
		t.Fatalf("project switch was reported as an in-project mutation: %#v", receipt)
	}
	p.Initialize(map[string]any{
		"status": "ok", "project_uuid": "real-project", "project_revision": 8,
		"tracks": []any{map[string]any{"track_id": "1007", "track_name": "Bass", "gain_db": -2.0}},
	})
	if receipt := p.LatestChangeReceipt(); receipt == nil || receipt["authoritative"] != true {
		t.Fatalf("same-project change after baseline reset was not recorded: %#v", receipt)
	}
}

func TestChangeWindowIsBoundedAndNewestFirst(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{"status": "ok", "project_uuid": "p1", "project_revision": 1, "tracks": []any{map[string]any{"track_id": "1007", "is_audio_track": true, "gain_db": 0.0}}})
	for i := 1; i <= changeHistoryLimit+5; i++ {
		p.ApplyDeltaWithSource(map[string]any{
			"type": "delta_update", "target_uid": "1007", "action": "property_changed:volume_db", "value": float64(-i),
		}, ChangeSourceExecutorDelta)
	}
	window := p.ChangeWindow(-1)
	if len(window) != changeHistoryLimit {
		t.Fatalf("change window len = %d, want %d", len(window), changeHistoryLimit)
	}
	if window[0]["change_id"] != p.LatestChangeReceipt()["change_id"] {
		t.Fatalf("window is not newest first: first=%#v latest=%#v", window[0], p.LatestChangeReceipt())
	}
}

func mapRowsFromChangeTest(value any) []map[string]any {
	rows, _ := value.([]any)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if item, ok := row.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out
}

func findChangeEntityTest(rows []map[string]any, kind, id string) map[string]any {
	for _, row := range rows {
		if row["kind"] == kind && row["id"] == id {
			return row
		}
	}
	return nil
}

func mapChangeFieldsTest(value any) map[string]map[string]any {
	rows := mapRowsFromChangeTest(value)
	out := map[string]map[string]any{}
	for _, row := range rows {
		path, _ := row["path"].(string)
		out[path] = row
	}
	return out
}

func stringsFromChangeTest(value any) []string {
	rows, _ := value.([]any)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if text, ok := row.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func containsChangeTest(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
