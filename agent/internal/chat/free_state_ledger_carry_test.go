package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
)

// FS-LEDGER-CARRY-1 (card 158): terminal-loop ledger carry (B) plus the
// cold-start seed observation (A1). The carry tests use the M2 session-2
// round-1 shape — a capability_blocked predecessor whose target-level
// evidence was delivered but whose project scan face stayed empty — and the
// §11 tests reuse the 157 pattern: a real pre-158 agent_runtime_state.json
// artifact (b13b_frontier run 20260913_105426, no new fields) loaded through
// the production decoder.

// carrySeedSemanticEntryContext is the same bootstrap the existing prepare
// tests use: a verified semantic-entry decision that routes a free-state loop.
func carrySeedSemanticEntryContext() map[string]any {
	return contextWithSemanticEntryDecision(map[string]any{}, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteOpenSemantic,
		Controller:  "minimal_audio_closure",
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlSemanticLoop,
		UserAuthorization: semanticEntryAuthorizationAction, Confidence: 0.94, Reason: "open acoustic outcome",
	})
}

// carryTerminalPredecessor builds the M2 round-1 terminal shape: the loop
// holds target-level evidence and an observation budget spent, but no usable
// project scan receipt (the G3 gap this card closes). Task-local fields
// (priority queue, observation ids/receipts, phase, cycle) are populated to
// prove they are never carried.
func carryTerminalPredecessor(now time.Time) freeStateReasoningLoop {
	return freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free_state_predecessor", ConversationID: "carry-chat",
		Status: "capability_blocked", OriginalIntent: "给人声加点空间感", ActiveIntent: "", MaxCycles: 6,
		CurrentPhase: "fs5_candidate_frontier", Cycle: 4,
		ObservationIDs:      []string{"obs-target-level"},
		ObservationReceipts: []map[string]any{{"receipt_id": "receipt-target-level"}},
		ObservationLedger: map[string]any{
			"schema_version": freeStateObservationLedgerSchema,
			"available_views": map[string]any{"track:1012::track.timbre_frequency": map[string]any{
				"view_id": "track.timbre_frequency", "status": "ready", "observation_id": "obs-target-level",
			}},
			"receipts": []map[string]any{{
				"receipt_id": "receipt-target-level", "status": "ready",
				"requested_views": []string{"track.timbre_frequency"}, "round": 4,
			}},
			"receipt_count": 1, "view_observation_count": 1, "window_round": 4,
		},
		CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Second),
	}
}

// TestPrepareCarriesTerminalPredecessorLedgerIntoNewLoop is the carry core
// (DESIGN §1-B): a persisted terminal loop seeds the new loop's observation
// ledger, lineage is recorded on the new loop only, and no task-local state
// (closure phase, candidates, ids, receipts, budget counters) is carried.
func TestPrepareCarriesTerminalPredecessorLedgerIntoNewLoop(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	pred := carryTerminalPredecessor(now)
	server.storeFreeStateLoop(pred)

	prepared, active := server.prepareFreeStateReasoningContext("carry-chat", "继续这个混音，整体再处理一下", carrySeedSemanticEntryContext())
	if !active {
		t.Fatal("free-state loop did not start after a terminal predecessor")
	}
	loop, ok := freeStateLoopFromAny(prepared["free_state_reasoning_loop"])
	if !ok {
		t.Fatal("prepared context carried no serializable loop")
	}
	if loop.LoopID == pred.LoopID || !freeStateLoopActive(loop) {
		t.Fatalf("terminal predecessor was revived instead of replaced: %#v", loop)
	}
	if loop.CarriedFromLoopID != pred.LoopID {
		t.Fatalf("lineage pointer missing: carried_from_loop_id=%q", loop.CarriedFromLoopID)
	}
	ledger := loop.ObservationLedger
	if firstStringFromMap(firstMapFromAny(firstMapFromAny(ledger["available_views"])["track:1012::track.timbre_frequency"]), "observation_id") != "obs-target-level" {
		t.Fatalf("predecessor available_views were not carried: %#v", ledger)
	}
	rows := freeStateMapRows(ledger["receipts"])
	if len(rows) != 2 || firstStringFromMap(rows[0], "receipt_id") != "receipt-target-level" {
		t.Fatalf("predecessor receipts were not carried: %#v", rows)
	}
	// The predecessor had no delivered scan, so the A1 seed fired on the same
	// boundary (the unit-server harness serves a real offline observation).
	seed := rows[1]
	if firstStringFromMap(seed, "request_id") != "fs_seed_carry-chat" ||
		!freeStateContainsString(freeStateStringSlice(seed["requested_views"]), freeStateSeedObservationViewID) {
		t.Fatalf("seed receipt missing or malformed: %#v", seed)
	}
	if !freeStateLedgerHasUsableScan(ledger) {
		t.Fatalf("carried+seed ledger still fails the G3 mirror: %#v", ledger)
	}
	if firstStringFromMap(loop.SeedObservation, "schema_version") != freeStateSeedObservationSchema ||
		firstStringFromMap(loop.SeedObservation, "observation_id") != firstStringFromMap(seed, "observation_id") {
		t.Fatalf("seed marker does not match the booked seed: %#v", loop.SeedObservation)
	}
	// Task-local fields must not cross the loop boundary (DESIGN §1-B: only
	// the ledger is carried — closure assumptions and budgets are not).
	if len(loop.ObservationIDs) != 0 || len(loop.ObservationReceipts) != 0 {
		t.Fatalf("observation ids/receipts leaked across the loop boundary: %#v", loop)
	}
	if loop.PriorityQueue != nil || loop.CurrentPhase != "" || loop.Cycle != 0 || loop.ContinuationUsed != 0 {
		t.Fatalf("closure phase or budget counters leaked across the loop boundary: %#v", loop)
	}
	if loop.Status != "reasoning" || loop.OriginalIntent != "继续这个混音，整体再处理一下" {
		t.Fatalf("new loop did not start fresh: %#v", loop)
	}
}

// TestCarriedLoopRoundtripsThroughPersistence proves the lineage pointer and
// seed marker survive the durable serialization surfaces (§11 additive
// fields): loop map encode/decode and the project runtime state snapshot.
func TestCarriedLoopRoundtripsThroughPersistence(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	pred := carryTerminalPredecessor(now)
	pred.ObservationLedger["receipts"] = []map[string]any{{
		"receipt_id": "receipt-scan", "status": "ready",
		"requested_views": []string{"mix.multitrack_relationship"}, "round": 4,
	}}
	server.storeFreeStateLoop(pred)
	if _, active := server.prepareFreeStateReasoningContext("carry-chat", "继续这个混音，整体再处理一下", carrySeedSemanticEntryContext()); !active {
		t.Fatal("free-state loop did not start")
	}
	loop, _ := server.freeStateLoop("carry-chat")
	if loop.CarriedFromLoopID != pred.LoopID {
		t.Fatalf("lineage pointer missing before persistence: %#v", loop)
	}

	server.mu.Lock()
	state := server.projectAgentRuntimeStateLocked()
	server.mu.Unlock()
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("runtime state failed to encode: %v", err)
	}
	if !strings.Contains(string(raw), `"carried_from_loop_id":"free_state_predecessor"`) {
		t.Fatalf("serialized state lost the lineage pointer")
	}
	decoded := projectAgentRuntimeState{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("runtime state failed to decode: %v", err)
	}
	restored, ok := decoded.FreeStateLoops["carry-chat"]
	if !ok || restored.CarriedFromLoopID != pred.LoopID {
		t.Fatalf("roundtrip lost the lineage pointer: %#v", decoded.FreeStateLoops)
	}
	if _, roundtripOK := freeStateLoopFromAny(freeStateLoopMap(restored)); !roundtripOK || restored.CarriedFromLoopID != pred.LoopID {
		t.Fatalf("loop map roundtrip lost the lineage pointer: %#v", freeStateLoopMap(restored))
	}
}

// TestFreeStateLedgerHasUsableScanMirrorsG3 pins the chat-side G3 mirror: the
// exact receipt shapes that do and do not count as a delivered qualified scan
// (status, view membership, disclosure-budget trim), matching the agentloop
// gate predicate.
func TestFreeStateLedgerHasUsableScanMirrorsG3(t *testing.T) {
	cases := []struct {
		name   string
		ledger map[string]any
		want   bool
	}{
		{"empty", nil, false},
		{"no receipts", map[string]any{"receipts": []map[string]any{}}, false},
		{"ready multitrack", map[string]any{"receipts": []map[string]any{{
			"receipt_id": "r1", "status": "ready", "requested_views": []string{"mix.multitrack_relationship"},
		}}}, true},
		{"partial frequency", map[string]any{"receipts": []map[string]any{{
			"receipt_id": "r2", "status": "partial", "requested_views": []string{"project.structure", "mix.frequency_relationship"},
		}}}, true},
		{"rejected scan", map[string]any{"receipts": []map[string]any{{
			"receipt_id": "r3", "status": "rejected", "requested_views": []string{"mix.multitrack_relationship"},
		}}}, false},
		{"structure only", map[string]any{"receipts": []map[string]any{{
			"receipt_id": "r4", "status": "ready", "requested_views": []string{"project.structure"},
		}}}, false},
		{"scan trimmed by disclosure budget", map[string]any{"receipts": []map[string]any{{
			"receipt_id": "r5", "status": "ready", "requested_views": []string{"mix.multitrack_relationship"},
			"rejection_reasons": []string{"mix.multitrack_relationship: omitted by disclosure budget"},
		}}}, false},
		{"non-scan view trimmed alongside delivered scan", map[string]any{"receipts": []map[string]any{{
			"receipt_id": "r6", "status": "ready", "requested_views": []string{"mix.multitrack_relationship", "project.structure"},
			"rejection_reasons": []string{"project.structure: omitted by disclosure budget"},
		}}}, true},
	}
	for _, tc := range cases {
		if got := freeStateLedgerHasUsableScan(tc.ledger); got != tc.want {
			t.Fatalf("%s: freeStateLedgerHasUsableScan=%v want %v", tc.name, got, tc.want)
		}
	}
}

// TestSeedObservationFromResultAndBooking proves the A1 seed path end to end
// at the unit level: a delivered bundle becomes a RecentObservation, booking
// it through the ordinary ledger merge makes the ledger G3-usable, and the
// marker records the seed's structural identity. Rejected or undelivered
// bundles seed nothing.
func TestSeedObservationFromResultAndBooking(t *testing.T) {
	delivered := map[string]any{
		"status": "ready",
		"bundle": map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "bundle_id": "ccbobs_seed", "status": "ready",
			"observation_id": "obs-seed", "requested_views": []any{"mix.multitrack_relationship"},
			"views": map[string]any{"mix.multitrack_relationship": map[string]any{
				"view_id": "mix.multitrack_relationship", "status": "ready", "facts": map[string]any{"observation.mom_projection": map[string]any{"status": "ready"}},
			}},
			"project_binding": map[string]any{"project_uuid": "vitproj-seed", "project_epoch": "e1", "project_revision": "9"},
			"freshness":       map[string]any{"status": "fresh", "project_revision": "9"},
			"audit_receipt": map[string]any{
				"schema_version": "ccb_observation_receipt.v1", "receipt_id": "ccbr_seed", "requested_by": "caller", "status": "executed",
			},
		},
	}
	observation := freeStateSeedObservationFromResult(delivered)
	if observation == nil || observation.Status != "ready" ||
		firstStringFromMap(observation.Summary, "observation_id") != "obs-seed" {
		t.Fatalf("delivered bundle did not become a seed observation: %#v", observation)
	}
	if contextBool(observation.Summary, "seed_observation") != true {
		t.Fatalf("seed observation lost its marker: %#v", observation.Summary)
	}
	ledger := mergeFreeStateObservationLedger(nil, observation, 0)
	if !freeStateLedgerHasUsableScan(ledger) {
		t.Fatalf("booked seed did not make the ledger G3-usable: %#v", ledger)
	}
	scanRow := firstMapFromAny(firstMapFromAny(ledger["available_views"])["mix.multitrack_relationship"])
	if firstStringFromMap(scanRow, "project_revision") != "9" {
		t.Fatalf("booked seed row lost its revision binding (G7 face): %#v", scanRow)
	}
	if firstStringFromMap(firstMapFromAny(scanRow["audit_ref"]), "receipt_id") != "ccbr_seed" {
		t.Fatalf("booked seed row lost its audit identity: %#v", scanRow)
	}
	marker := freeStateSeedObservationMarker(observation, time.Now().UTC())
	if firstStringFromMap(marker, "schema_version") != freeStateSeedObservationSchema ||
		firstStringFromMap(marker, "observation_id") != "obs-seed" ||
		firstStringFromMap(marker, "receipt_id") != "ccbr_seed" ||
		firstStringFromMap(marker, "view_id") != freeStateSeedObservationViewID {
		t.Fatalf("seed marker lost structural identity: %#v", marker)
	}

	for name, result := range map[string]map[string]any{
		"rejected bundle": {"status": "rejected", "bundle": map[string]any{
			"status": "rejected", "requested_views": []any{"mix.multitrack_relationship"},
		}},
		"empty result":   {},
		"bundle missing": {"status": "ready"},
		"no observation": {"status": "ready", "bundle": map[string]any{
			"status": "ready", "requested_views": []any{"mix.multitrack_relationship"},
		}},
	} {
		if freeStateSeedObservationFromResult(result) != nil {
			t.Fatalf("%s: unusable bundle became a seed observation", name)
		}
	}
}

// TestSeedObservationFailsOpenWithoutHarness proves the fail-open contract at
// the issue helper: a server without observation machinery gets no seed (nil,
// no panic), and a nil server is equally inert — the loop start can never be
// blocked by the seed path.
func TestSeedObservationFailsOpenWithoutHarness(t *testing.T) {
	bare := &Server{}
	if got := bare.issueFreeStateSeedObservation("cold-chat", "给人声加点空间感"); got != nil {
		t.Fatalf("nil harness invented a seed observation: %#v", got)
	}
	var nilServer *Server
	if got := nilServer.issueFreeStateSeedObservation("cold-chat", "给人声加点空间感"); got != nil {
		t.Fatalf("nil server invented a seed observation: %#v", got)
	}
}

// TestCarryPlusNewObservationsEvictAtLedgerLimit pins the §5-3 overflow
// semantics: a carried ledger already at freeStateObservationLedgerLimit and
// one new observation keep the newest freeStateObservationLedgerLimit receipts
// — the oldest carried receipt is evicted, the fresh one survives.
func TestCarryPlusNewObservationsEvictAtLedgerLimit(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	pred := carryTerminalPredecessor(now)
	full := make([]map[string]any, 0, freeStateObservationLedgerLimit)
	for i := 0; i < freeStateObservationLedgerLimit; i++ {
		full = append(full, map[string]any{
			"receipt_id": "receipt-carry-" + string(rune('a'+i)), "status": "ready",
			"requested_views": []string{"project.structure"}, "round": i + 1,
		})
	}
	pred.ObservationLedger["receipts"] = full
	pred.ObservationLedger["receipt_count"] = freeStateObservationLedgerLimit
	server.storeFreeStateLoop(pred)

	if _, active := server.prepareFreeStateReasoningContext("carry-chat", "继续这个混音，整体再处理一下", carrySeedSemanticEntryContext()); !active {
		t.Fatal("free-state loop did not start")
	}
	carried, _ := server.freeStateLoop("carry-chat")
	if len(freeStateMapRows(carried.ObservationLedger["receipts"])) != freeStateObservationLedgerLimit {
		t.Fatalf("carry did not preserve the bounded receipt set: %#v", carried.ObservationLedger)
	}

	server.recordFreeStateDecision("carry-chat", agentloop.Result{
		Executed: []map[string]any{
			{"tool": "ccb.observation_request", "tool_call_id": "call-fresh", "status": "ok", "result": map[string]any{
				"status": "ready", "bundle": map[string]any{
					"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-fresh",
					"requested_views": []any{"mix.frequency_relationship"},
					"views":           map[string]any{"mix.frequency_relationship": map[string]any{"status": "ready"}},
					"audit_receipt":   map[string]any{"schema_version": "ccb_observation_receipt.v1", "receipt_id": "receipt-fresh"},
				},
			}},
		},
	})
	loop, _ := server.freeStateLoop("carry-chat")
	rows := freeStateMapRows(loop.ObservationLedger["receipts"])
	if len(rows) != freeStateObservationLedgerLimit {
		t.Fatalf("receipt set exceeded the ledger limit: %d rows", len(rows))
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[firstStringFromMap(row, "receipt_id")] = true
	}
	if !seen["receipt-fresh"] {
		t.Fatalf("fresh receipt was evicted: %#v", seen)
	}
	if seen["receipt-carry-a"] {
		t.Fatalf("oldest carried receipt survived the overflow eviction: %#v", seen)
	}
}

// TestLegacyRuntimeStateWithoutCarrySeedFieldsLoadsAndRoundtrips is the §11-1/2
// regression: the real pre-158 artifact (b13b_frontier run 20260913_105426,
// free_state_reasoning_loops with a full observation ledger, no
// carried_from_loop_id / seed_observation keys) decodes through the production
// struct with both fields at their §11 defaults, re-encodes without inventing
// either key, and its delivered scan receipts remain G3-usable under the
// chat-side mirror.
func TestLegacyRuntimeStateWithoutCarrySeedFieldsLoadsAndRoundtrips(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "fs_ledger_carry_legacy_runtime_state.json"))
	if err != nil {
		t.Fatalf("legacy fixture missing: %v", err)
	}
	state := projectAgentRuntimeState{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("legacy runtime state failed to decode: %v", err)
	}
	if len(state.FreeStateLoops) == 0 {
		t.Fatalf("legacy fixture carries no free-state loops")
	}
	for id, loop := range state.FreeStateLoops {
		if loop.CarriedFromLoopID != "" {
			t.Fatalf("legacy loop %s decoded a lineage pointer: %q", id, loop.CarriedFromLoopID)
		}
		if loop.SeedObservation != nil {
			t.Fatalf("legacy loop %s decoded a seed marker: %#v", id, loop.SeedObservation)
		}
		if !freeStateLedgerHasUsableScan(loop.ObservationLedger) {
			t.Fatalf("legacy loop %s delivered scan receipts not recognized by the G3 mirror", id)
		}
	}
	out, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("legacy runtime state failed to encode: %v", err)
	}
	for _, key := range []string{"carried_from_loop_id", "seed_observation"} {
		if strings.Contains(string(out), `"`+key+`"`) {
			t.Fatalf("legacy roundtrip invented the %s key", key)
		}
	}
	again := projectAgentRuntimeState{}
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("re-encoded runtime state failed to decode: %v", err)
	}
	if len(again.FreeStateLoops) != len(state.FreeStateLoops) {
		t.Fatalf("roundtrip lost free-state loops: %d -> %d", len(state.FreeStateLoops), len(again.FreeStateLoops))
	}
}

// TestRestoredLegacyTerminalLoopSeedsNewLoopLedger drives the carry with the
// real artifact end to end: the restored pre-158 loop is stale-folded to a
// terminal status, and the next routed turn in the same conversation creates
// a new loop whose ledger is seeded from the artifact's delivered evidence —
// no seed observation is issued because the carried ledger already satisfies
// G3 (§11 default semantics produce ordinary carry behavior).
func TestRestoredLegacyTerminalLoopSeedsNewLoopLedger(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "fs_ledger_carry_legacy_runtime_state.json"))
	if err != nil {
		t.Fatalf("legacy fixture missing: %v", err)
	}
	state := projectAgentRuntimeState{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("legacy runtime state failed to decode: %v", err)
	}
	server := New(nil, nil, nil)
	server.mu.Lock()
	server.restoreProjectAgentRuntimeStateLocked(state)
	server.mu.Unlock()

	conversationID := ""
	artifactLoopID := ""
	for id, loop := range state.FreeStateLoops {
		conversationID, artifactLoopID = id, loop.LoopID
	}
	if conversationID == "" {
		t.Fatal("legacy fixture carries no free-state loop")
	}
	restored, ok := server.freeStateLoop(conversationID)
	if !ok || freeStateLoopActive(restored) {
		t.Fatalf("restored legacy loop was not stale-folded to a terminal status: %#v", restored)
	}
	if _, active := server.prepareFreeStateReasoningContext(conversationID, "继续处理这个工程", carrySeedSemanticEntryContext()); !active {
		t.Fatal("free-state loop did not start after restore")
	}
	loop, _ := server.freeStateLoop(conversationID)
	if loop.CarriedFromLoopID != artifactLoopID {
		t.Fatalf("restore-path carry lost the lineage pointer: %q want %q", loop.CarriedFromLoopID, artifactLoopID)
	}
	if !freeStateLedgerHasUsableScan(loop.ObservationLedger) {
		t.Fatalf("restore-path carry lost the artifact's delivered scan: %#v", loop.ObservationLedger)
	}
	if loop.SeedObservation != nil {
		t.Fatalf("seed issued although the carried ledger already satisfied G3: %#v", loop.SeedObservation)
	}
}
