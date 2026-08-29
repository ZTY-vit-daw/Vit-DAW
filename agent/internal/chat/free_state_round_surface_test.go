package chat

import (
	"testing"
	"time"
)

// D2-2-S3h1 (2026-08-29 201003 trace): round-2's fresh base observation
// (obs_20260829T121303, rev 4) reached the experiment round and the CCB
// receipts, but the model-visible available_views catalog kept pointing at
// the pre-mutation observation (obs_20260829T121227, rev 2, round-1
// pre-action, status "ready"). The model quoted exactly the pointer the
// catalog handed it and G7 rejected the quote. These tests lock the two
// halves of the evidence-surface freshness fix: the round boundary projects
// its freshly booked base into the ledger, and a continuation/transport
// overlay can no longer drag the catalog back to a lower project revision.

// roundSurfaceTrackViewKey is the ledger key the timbre view on the vocal
// track occupies (track-scoped key assigned by freeStateObservationLedgerViewKey).
const roundSurfaceTrackViewKey = "track:vocal::track.timbre_frequency"

// seedRoundSurfacePreActionLedger reproduces the S3h smoke's persisted ledger:
// the model-visible catalog presents the round-1 pre-action observation as the
// freshest ready evidence for the target view.
func seedRoundSurfacePreActionLedger() map[string]any {
	return map[string]any{
		"schema_version": freeStateObservationLedgerSchema,
		"window_round":   1,
		"available_views": map[string]any{
			roundSurfaceTrackViewKey: map[string]any{
				"view_id": "track.timbre_frequency", "status": "ready",
				"observation_id": "obs-pre-action", "tool_call_id": "call-before",
				"project_revision": "7", "round": 1,
				"freshness":  map[string]any{"status": "current_observation", "project_revision": "7"},
				"target_ref": map[string]any{"kind": "track", "id": "vocal"},
			},
		},
		"receipts": []any{
			map[string]any{
				"receipt_id": "receipt-before", "status": "ready", "observation_id": "obs-pre-action",
				"tool_call_id": "call-before", "project_revision": "7", "round": 1,
				"requested_views": []any{"track.timbre_frequency"},
				"freshness": map[string]any{"status": "current_observation", "project_revision": "7"},
			},
			// The deterministic post-action booking's receipt rows, exactly as
			// the 201003 trace persisted them: the post_action freshness class
			// G7's vocabulary refuses, no receipt_id, echoed once per merge.
			map[string]any{
				"status": "ready", "observation_id": "obs-d2-action-1",
				"tool_call_id": "d1_post_action:turn-d2", "project_revision": "8", "round": 0,
				"requested_views": []any{"track.timbre_frequency"},
				"freshness": map[string]any{"status": "ready", "class": "post_action", "project_revision": "8"},
			},
			map[string]any{
				"status": "ready", "observation_id": "obs-d2-action-1",
				"tool_call_id": "d1_post_action:turn-d2", "project_revision": "8", "round": 0,
				"requested_views": []any{"track.timbre_frequency"},
				"freshness": map[string]any{"status": "ready", "class": "post_action", "project_revision": "8"},
			},
		},
		"receipt_count":         3,
		"view_observation_count": 1,
	}
}

func roundSurfaceAvailableRow(t *testing.T, ledger map[string]any, key string) map[string]any {
	t.Helper()
	available := firstMapFromAny(ledger["available_views"])
	if len(available) == 0 {
		t.Fatalf("observation ledger carries no available_views catalog")
	}
	row := firstMapFromAny(available[key])
	if len(row) == 0 {
		t.Fatalf("available_views has no row for %s: %v", key, available)
	}
	return row
}

// TestRecalibrationBoundaryPresentsRoundBaseOnEvidenceSurface is the S3h1 RED:
// the recalibration round boundary books the round base observation (the
// judged round's post-action bundle at the live revision) — that same booking
// must refresh the model-visible catalog, or the round-2 proposal turn is
// handed the pre-action pointer it will be rejected for quoting.
func TestRecalibrationBoundaryPresentsRoundBaseOnEvidenceSurface(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	loop.ObservationLedger = seedRoundSurfacePreActionLedger()
	driveNoDifferenceRecalibration(t, s, &loop)

	stored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok {
		t.Fatal("recalibration boundary did not store the loop")
	}
	row := roundSurfaceAvailableRow(t, stored.ObservationLedger, roundSurfaceTrackViewKey)
	if row["observation_id"] != "obs-d2-action-1" {
		t.Fatalf("round base observation did not reach the evidence surface: observation_id=%v", row["observation_id"])
	}
	if row["project_revision"] != "8" {
		t.Fatalf("round base row lost its revision binding: project_revision=%v", row["project_revision"])
	}
	if status := firstStringFromMap(row, "status"); status != "ready" && status != "partial" {
		t.Fatalf("round base row is not usable evidence: status=%v", row["status"])
	}
	if freshness := firstMapFromAny(row["freshness"]); len(freshness) > 0 {
		if revision := firstStringFromMap(freshness, "project_revision"); revision != "" && revision != "8" {
			t.Fatalf("round base row freshness is revision-bound elsewhere: %v", freshness)
		}
	}
	// Catalog truthfulness: the boundary only presents the view set the base
	// actually executed; no view is fabricated as available.
	available := firstMapFromAny(stored.ObservationLedger["available_views"])
	for key := range available {
		if key != roundSurfaceTrackViewKey {
			t.Fatalf("boundary fabricated an available view row %q beyond the base's executed set", key)
		}
	}
	// The fresh identity is quotable: the receipts section carries the same
	// observation with the same revision binding, restated in the freshness
	// vocabulary G7 accepts (the deterministic booking's post_action class is
	// superseded at the re-base, and duplicate booking echoes collapse).
	baseReceipts := 0
	for _, receipt := range freeStateMapRows(stored.ObservationLedger["receipts"]) {
		if firstStringFromMap(receipt, "observation_id") != "obs-d2-action-1" {
			continue
		}
		baseReceipts++
		if firstStringFromMap(receipt, "project_revision") != "8" {
			t.Fatalf("round base receipt lost its revision binding: %+v", receipt)
		}
		freshness := firstMapFromAny(receipt["freshness"])
		if class := firstStringFromMap(freshness, "class"); class != "" && class != "current_observation" {
			t.Fatalf("round base receipt kept a non-current freshness class %q: %+v", class, freshness)
		}
		if firstStringFromMap(freshness, "project_revision") != "8" {
			t.Fatalf("round base receipt freshness is not bound to the live revision: %+v", freshness)
		}
		if firstStringFromMap(receipt, "receipt_id") == "" {
			t.Fatalf("round base receipt has no dedup identity: %+v", receipt)
		}
	}
	if baseReceipts == 0 {
		t.Fatal("round base observation is not receipted in the ledger")
	}
	if baseReceipts > 2 {
		t.Fatalf("round base receipts were not collapsed at the re-base: %d rows", baseReceipts)
	}
}

// TestRoundSurfaceLedgerMergeKeepsHigherRevisionRow locks the drag-back fix:
// continuation and transport overlays serialised before a mutation must not
// overwrite a catalog row that already observes a higher project revision.
func TestRoundSurfaceLedgerMergeKeepsHigherRevisionRow(t *testing.T) {
	freshRow := map[string]any{
		"view_id": "track.timbre_frequency", "status": "ready",
		"observation_id": "obs-round-base", "project_revision": "8", "round": 2,
		"freshness":  map[string]any{"class": "current_observation", "project_revision": "8"},
		"target_ref": map[string]any{"kind": "track", "id": "vocal"},
	}
	base := map[string]any{
		"schema_version":  freeStateObservationLedgerSchema,
		"available_views": map[string]any{roundSurfaceTrackViewKey: cloneContext(freshRow)},
	}
	staleRow := map[string]any{
		"view_id": "track.timbre_frequency", "status": "ready",
		"observation_id": "obs-pre-action", "project_revision": "7", "round": 1,
		"freshness":  map[string]any{"status": "current_observation", "project_revision": "7"},
		"target_ref": map[string]any{"kind": "track", "id": "vocal"},
	}
	overlay := map[string]any{
		"schema_version":  freeStateObservationLedgerSchema,
		"available_views": map[string]any{roundSurfaceTrackViewKey: cloneContext(staleRow)},
	}
	merged := mergeFreeStateLedgers(base, overlay)
	row := roundSurfaceAvailableRow(t, merged, roundSurfaceTrackViewKey)
	if row["observation_id"] != "obs-round-base" {
		t.Fatalf("pre-mutation overlay dragged the catalog back: observation_id=%v", row["observation_id"])
	}
	if row["project_revision"] != "8" {
		t.Fatalf("catalog row lost its revision after the overlay merge: %v", row["project_revision"])
	}
}

// TestRoundSurfaceLedgerMergeStillPrefersOverlayForSameOrNewerRevision locks
// the preserved overlay semantics: the transport echo refresh (same revision)
// and genuinely newer evidence (higher revision) still win — the guard only
// stops revision regressions.
func TestRoundSurfaceLedgerMergeStillPrefersOverlayForSameOrNewerRevision(t *testing.T) {
	viewRow := func(observationID, revision string, round int) map[string]any {
		return map[string]any{
			"view_id": "track.timbre_frequency", "status": "ready",
			"observation_id": observationID, "project_revision": revision, "round": round,
			"target_ref": map[string]any{"kind": "track", "id": "vocal"},
		}
	}
	ledger := func(row map[string]any) map[string]any {
		return map[string]any{
			"schema_version":  freeStateObservationLedgerSchema,
			"available_views": map[string]any{roundSurfaceTrackViewKey: cloneContext(row)},
		}
	}
	// Same revision: the overlay refresh still wins (historical behavior).
	same := mergeFreeStateLedgers(ledger(viewRow("obs-a", "8", 2)), ledger(viewRow("obs-b", "8", 3)))
	if row := roundSurfaceAvailableRow(t, same, roundSurfaceTrackViewKey); row["observation_id"] != "obs-b" {
		t.Fatalf("same-revision overlay no longer refreshes the row: %v", row["observation_id"])
	}
	// Higher revision: genuinely newer evidence still wins.
	forward := mergeFreeStateLedgers(ledger(viewRow("obs-old", "8", 2)), ledger(viewRow("obs-new", "9", 3)))
	if row := roundSurfaceAvailableRow(t, forward, roundSurfaceTrackViewKey); row["observation_id"] != "obs-new" {
		t.Fatalf("newer-revision overlay was refused: %v", row["observation_id"])
	}
	// Rows without a revision binding keep the historical overlay-wins merge.
	unbound := mergeFreeStateLedgers(
		ledger(map[string]any{"view_id": "track.timbre_frequency", "status": "ready", "observation_id": "obs-unbound-base"}),
		ledger(map[string]any{"view_id": "track.timbre_frequency", "status": "ready", "observation_id": "obs-unbound-overlay"}),
	)
	if row := roundSurfaceAvailableRow(t, unbound, roundSurfaceTrackViewKey); row["observation_id"] != "obs-unbound-overlay" {
		t.Fatalf("revision-free rows changed merge behavior: %v", row["observation_id"])
	}
}

// TestRecalibrationRoundSurfaceSurvivesStaleTransportEcho closes the S3h
// kill-chain end to end at the loop level: after the boundary refresh, the
// round-2 turn's transport echo (a continuation carrying the pre-action
// catalog) must not regress the evidence surface the proposal turn sees.
func TestRecalibrationRoundSurfaceSurvivesStaleTransportEcho(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	loop.ObservationLedger = seedRoundSurfacePreActionLedger()
	driveNoDifferenceRecalibration(t, s, &loop)

	stored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok {
		t.Fatal("recalibration boundary did not store the loop")
	}
	echo := stored
	echo.ObservationLedger = seedRoundSurfacePreActionLedger()
	echo.UpdatedAt = stored.UpdatedAt.Add(time.Second)
	merged := mergeFreeStateLoops(stored, echo, true)
	row := roundSurfaceAvailableRow(t, merged.ObservationLedger, roundSurfaceTrackViewKey)
	if row["observation_id"] != "obs-d2-action-1" {
		t.Fatalf("stale transport echo dragged the round-2 evidence surface back to %v", row["observation_id"])
	}
	if row["project_revision"] != "8" {
		t.Fatalf("round-2 evidence surface lost its revision after the echo merge: %v", row["project_revision"])
	}
}

// TestSingleRoundBoundaryNeverTouchesEvidenceSurface is the zero-change red
// line: the sealed single-round tier never opens a recalibration round, so its
// boundary path must leave the observation ledger catalog untouched.
func TestSingleRoundBoundaryNeverTouchesEvidenceSurface(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 1)
	loop.ObservationLedger = seedRoundSurfacePreActionLedger()
	before := cloneContext(loop.ObservationLedger)
	s.bookRecalibrationRoundBaseFromLoop(&loop)
	after := loop.ObservationLedger
	if len(firstMapFromAny(after["available_views"])) != len(firstMapFromAny(before["available_views"])) {
		t.Fatal("single-round tier boundary changed the available_views catalog size")
	}
	beforeRow := roundSurfaceAvailableRow(t, before, roundSurfaceTrackViewKey)
	afterRow := roundSurfaceAvailableRow(t, after, roundSurfaceTrackViewKey)
	if beforeRow["observation_id"] != afterRow["observation_id"] {
		t.Fatalf("single-round tier boundary moved the catalog pointer: %v -> %v",
			beforeRow["observation_id"], afterRow["observation_id"])
	}
}
