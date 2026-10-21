package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/shadow"
)

// FS-NAMED-TARGET-1 unit faces: named-target merge, deterministic resolution
// (fail-open / ambiguity / Chinese alias), gate-side fail-closed projection,
// G5/G6/G8 derivation through the real gate evaluator, and the §11
// persistence-compatibility regressions. Fixture track names and ids are
// synthetic (AGENTS §10 sealed discipline: no real project track names).

// fsNamedTargetFixtureProject builds the synthetic track inventory used by
// the resolution faces: one vocals-keyword track, two drum-keyword tracks
// (the ambiguity probe), one backing-keyword track, one non-audio folder
// sharing a vocals keyword (the audio-filter probe), and one neutral track.
func fsNamedTargetFixtureProject() *shadow.Project {
	proj := shadow.New(nil)
	proj.Initialize(map[string]any{
		"project_uuid": "proj-fsnamed-fixture",
		"tracks": []any{
			map[string]any{"id": "9001", "name": "Fixture Lead 人声 A", "is_audio_track": true},
			map[string]any{"id": "9002", "name": "Fixture Drums Bus", "is_audio_track": true},
			map[string]any{"id": "9003", "name": "Fixture Drums Snap", "is_audio_track": true},
			map[string]any{"id": "9004", "name": "Fixture 伴奏 Keys", "is_audio_track": true},
			map[string]any{"id": "9005", "name": "Fixture 人声 Folder", "type": "folder"},
			map[string]any{"id": "9006", "name": "Fixture Piano Neutral", "is_audio_track": true},
		},
	})
	return proj
}

func fsNamedTargetServer() *Server {
	return &Server{harness: harness.NewWithSender(nil, fsNamedTargetFixtureProject(), nil)}
}

func TestFSNamedTargetResolutionFaces(t *testing.T) {
	server := fsNamedTargetServer()
	cases := []struct {
		name     string
		text     string
		expected string
	}{
		{"chinese vocals alias unique hit", "帮我把人声处理一下，高频有点刺", "9001"},
		{"latin vocals alias maps to chinese keyword", "the vocals need more air", "9001"},
		{"exact track name containment", "Fixture 伴奏 Keys 太响了压一点", "9004"},
		{"unresolvable text fails open", "整体响度再大一点", ""},
		{"ambiguous drum family rejected", "鼓组能不能更紧一点", ""},
		{"single-char chinese alias never fires", "这个安排鼓励了我", ""},
	}
	for _, testCase := range cases {
		if got := server.resolveFreeStateUserNamedTarget(testCase.text); got != testCase.expected {
			t.Errorf("%s: resolve(%q) = %q, want %q", testCase.name, testCase.text, got, testCase.expected)
		}
	}
	// Fail-open hardware: a server without a harness never guesses.
	if got := (&Server{}).resolveFreeStateUserNamedTarget("人声"); got != "" {
		t.Fatalf("nil-harness server resolved a target: %q", got)
	}
	// The vocals-unique case also pins the audio filter: including the
	// 人声-named folder would make the family ambiguous.
}

func TestFSNamedTargetPrepareCaptureFailsOpenSticky(t *testing.T) {
	server := fsNamedTargetServer()
	requestContext := func() map[string]any { return map[string]any{"free_state_diagnostic_only": true} }
	if _, ok := server.prepareFreeStateReasoningContext("conv-fsnamed-1", "帮我把人声处理一下", requestContext()); !ok {
		t.Fatalf("diagnostic-only prepare did not start a loop")
	}
	loop, ok := server.freeStateLoop("conv-fsnamed-1")
	if !ok || loop.UserNamedTarget != "9001" {
		t.Fatalf("prepare did not capture the resolved named target: ok=%v target=%q", ok, loop.UserNamedTarget)
	}
	if freeStateLoopMap(loop)["user_named_target"] != "9001" {
		t.Fatalf("transport loop map dropped user_named_target")
	}
	// Fail-open is sticky, never erasing: an unresolvable follow-up keeps the
	// captured pointer (latest unique naming wins, a miss changes nothing).
	if _, ok := server.prepareFreeStateReasoningContext("conv-fsnamed-1", "整体响度好一点", requestContext()); !ok {
		t.Fatalf("active-loop prepare failed")
	}
	after, _ := server.freeStateLoop("conv-fsnamed-1")
	if after.UserNamedTarget != "9001" {
		t.Fatalf("unresolvable follow-up erased the captured target: %q", after.UserNamedTarget)
	}
	// §11-1: a loop with no resolution serializes without the new key, so
	// old-state behavior is byte-identical.
	fresh := fsNamedTargetServer()
	if _, ok := fresh.prepareFreeStateReasoningContext("conv-fsnamed-2", "整体响度好一点", requestContext()); !ok {
		t.Fatalf("unresolved prepare did not start a loop")
	}
	unresolved, _ := fresh.freeStateLoop("conv-fsnamed-2")
	if unresolved.UserNamedTarget != "" {
		t.Fatalf("unresolved loop captured a target: %q", unresolved.UserNamedTarget)
	}
	encoded, err := json.Marshal(unresolved)
	if err != nil || strings.Contains(string(encoded), "user_named_target") {
		t.Fatalf("unresolved loop serialized the new field: %v %s", err, encoded)
	}
}

func TestFSNamedTargetFoldSynthesizesCandidate(t *testing.T) {
	machine := []audioclosure.Candidate{{ID: "candidate:fixture", TrackIDs: []string{"9006"}}}
	merged := audioClosureUserNamedCandidates(machine, "9002")
	if len(merged) != 2 {
		t.Fatalf("named target not appended: %+v", merged)
	}
	synthetic := merged[len(merged)-1]
	if synthetic.ID != "user_named:9002" ||
		synthetic.Source != audioClosureCandidateSourceUserIntent ||
		len(synthetic.TrackIDs) != 1 || synthetic.TrackIDs[0] != "9002" ||
		synthetic.SourceObservationID != "" || synthetic.ViewID != "" {
		t.Fatalf("synthetic candidate shape drifted: %+v", synthetic)
	}
	// Idempotent: re-folding the same named target replaces, never stacks.
	if again := audioClosureUserNamedCandidates(merged, "9002"); len(again) != 2 {
		t.Fatalf("named-target merge is not idempotent: %+v", again)
	}
	// A named track already covered by a machine candidate is left alone.
	if covered := audioClosureUserNamedCandidates(machine, "9006"); len(covered) != 1 {
		t.Fatalf("covered named target appended a duplicate: %+v", covered)
	}
	// Empty target is a strict no-op and the input slice is never mutated.
	if empty := audioClosureUserNamedCandidates(machine, "  "); len(empty) != 1 || len(machine) != 1 {
		t.Fatalf("empty target mutated the candidate list: %+v", empty)
	}
	// Fold level: the named-target variant merges, the legacy wrapper (empty
	// target, admission-fallback path) stays candidate-free byte-identically.
	decision := agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema,
		Status:        agentloop.FreeStateNeedsObservation, EvidenceStatus: "insufficient",
	}
	frontier, _ := audioClosureFrontierWithUserNamedTarget(audioclosure.HypothesisFrontier{}, decision, nil, "9002")
	found := false
	for _, candidate := range frontier.Candidates {
		if candidate.ID == "user_named:9002" {
			found = true
		}
	}
	if !found {
		t.Fatalf("fold lost the user_intent candidate: %+v", frontier.Candidates)
	}
	legacy, _ := audioClosureFrontier(audioclosure.HypothesisFrontier{}, decision, nil)
	if len(legacy.Candidates) != 0 {
		t.Fatalf("legacy frontier wrapper synthesized a candidate: %+v", legacy.Candidates)
	}
}

func TestFSNamedTargetGateProjectionFailsClosedOnUnknownSource(t *testing.T) {
	state := audioclosure.State{ProjectUUID: "proj-fsnamed-fixture", ProjectRevision: "7",
		Frontier: audioclosure.HypothesisFrontier{Candidates: []audioclosure.Candidate{
			{ID: "candidate:machine", TrackIDs: []string{"9006"}},
			{ID: "user_named:9002", Source: audioClosureCandidateSourceUserIntent, TrackIDs: []string{"9002"}},
			{ID: "candidate:foreign", Source: "quantum_oracle", TrackIDs: []string{"9004"}},
		}}}
	projected := audioClosureStateMap(state)
	rows := freeStateMapRows(firstMapFromAny(firstMapFromAny(projected["hypothesis_frontier"]))["candidates"])
	seen := map[string]bool{}
	for _, row := range rows {
		seen[firstStringFromMap(row, "id")] = true
	}
	if !seen["candidate:machine"] || !seen["user_named:9002"] {
		t.Fatalf("projection dropped an admitted source: %+v", seen)
	}
	if seen["candidate:foreign"] {
		t.Fatalf("projection admitted an unknown candidate source (fail-closed violated)")
	}
	// §11-1: machine candidates project without a source key, byte-identical
	// to the pre-FS-NAMED-TARGET-1 projection.
	for _, row := range rows {
		if firstStringFromMap(row, "id") == "candidate:machine" {
			if _, has := row["source"]; has {
				t.Fatalf("machine candidate gained a source key: %+v", row)
			}
		}
	}
	// Copy-on-write: the persisted candidate slice is untouched by a projection.
	if len(state.Frontier.Candidates) != 3 {
		t.Fatalf("projection mutated the persisted candidate list")
	}
}

func fsNamedTargetAuditContext(withNamed bool) map[string]any {
	candidates := []audioclosure.Candidate{}
	if withNamed {
		candidates = audioClosureUserNamedCandidates(candidates, "9002")
	}
	closure := audioClosureStateMap(audioclosure.State{
		ProjectUUID: "proj-fsnamed-fixture", ProjectRevision: "7",
		Frontier: audioclosure.HypothesisFrontier{Candidates: candidates},
	})
	trackTarget := map[string]any{"kind": "track", "id": "9002"}
	return map[string]any{
		"minimal_audio_closure": closure,
		"free_state_reasoning_loop": map[string]any{
			"schema_version": freeStateReasoningLoopSchema, "status": "observing",
			"observation_ledger": map[string]any{
				"receipts": []any{map[string]any{
					"status": "ready", "observation_id": "obs-fsnamed-target", "target_ref": trackTarget,
					"evidence_refs": []any{"obs-fsnamed-target"},
				}},
				"available_views": map[string]any{
					"track:9002::track.timbre_frequency": map[string]any{
						"view_id": "track.timbre_frequency", "status": "ready",
						"observation_id": "obs-fsnamed-target", "target_ref": trackTarget,
					},
				},
			},
		},
	}
}

// TestFSNamedTargetGateG5G6G8Derivation drives the real agentloop admission
// gate evaluator (AuditFreeStateNeedsExperimentGate) over the chat-side
// projection: with the user_intent synthetic candidate the named track
// establishes G5, carries G6 target evidence, and passes G8 target
// consistency for a proposal on exactly that track; without it the same
// context fails all three (the 156 F-gate signature).
func TestFSNamedTargetGateG5G6G8Derivation(t *testing.T) {
	decision := &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema,
		Status:        agentloop.FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "fixture summary",
		ImprovementProposal: &agentprotocol.ImprovementProposal{
			SchemaVersion:     agentprotocol.ImprovementProposalSchema,
			Target:            map[string]any{"kind": "track", "id": "9002"},
			EvidenceRefs:      []string{"obs-fsnamed-target"},
			ImprovementIntent: "make the named fixture relationship clearer", Hypothesis: "a bounded change may improve it",
			ExpectedEffect: "the relationship should be easier to compare", ActionDomain: agentprotocol.ImprovementActionDomainTrackGain,
			ActionKind: "bounded_gain_adjustment", ParameterBounds: map[string]any{"delta_db": -0.5}, Confidence: 0.55,
		},
	}
	without := agentloop.AuditFreeStateNeedsExperimentGate(fsNamedTargetAuditContext(false), decision)
	with := agentloop.AuditFreeStateNeedsExperimentGate(fsNamedTargetAuditContext(true), decision)
	for _, gate := range []string{"G5_frontier_established", "G6_target_evidence", "G8_target_consistency"} {
		if !containsString(without.FailedGateIDs, gate) {
			t.Fatalf("baseline context without the named candidate unexpectedly passed %s: %v", gate, without.FailedGateIDs)
		}
		if containsString(with.FailedGateIDs, gate) {
			t.Fatalf("user_intent candidate did not unblock %s: %v", gate, with.FailedGateIDs)
		}
	}
}

// TestFSNamedTargetLegacyRuntimeStateRoundtrip is the §11-2 regression: a
// real pre-FS-NAMED-TARGET-1 agent_runtime_state.json (2026-10-11 stack run,
// free_state_reasoning_loops without the new field) loads through the exact
// decode path, keeps every loop with an empty user-intent pointer, and
// re-serializes without the new key.
func TestFSNamedTargetLegacyRuntimeStateRoundtrip(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "fs_named_target_legacy_runtime_state.json"))
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
		if loop.UserNamedTarget != "" {
			t.Fatalf("legacy loop %s decoded a user-named target: %q", id, loop.UserNamedTarget)
		}
	}
	out, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("legacy runtime state failed to encode: %v", err)
	}
	if strings.Contains(string(out), "user_named_target") {
		t.Fatalf("legacy roundtrip invented the user_named_target key")
	}
	again := projectAgentRuntimeState{}
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("re-encoded runtime state failed to decode: %v", err)
	}
	if len(again.FreeStateLoops) != len(state.FreeStateLoops) {
		t.Fatalf("roundtrip lost free-state loops: %d -> %d", len(state.FreeStateLoops), len(again.FreeStateLoops))
	}
}

// TestFSNamedTargetCandidateSourceDeserialization is the §11-3 regression:
// candidate rows carrying the new user_intent enum — or any foreign source
// value — deserialize into the closure state without crashing, and the
// gate-side projection fails closed on the foreign value end to end.
func TestFSNamedTargetCandidateSourceDeserialization(t *testing.T) {
	raw := `{"schema_version":"` + audioclosure.SchemaVersion + `","closure_id":"closure-fsnamed-fixture",
		"project_uuid":"proj-fsnamed-fixture","project_revision":"7",
		"hypothesis_frontier":{"candidates":[
			{"id":"user_named:9002","source":"user_intent","track_ids":["9002"]},
			{"id":"candidate:foreign","source":"quantum_oracle","track_ids":["9004"]},
			{"id":"candidate:machine","track_ids":["9006"]}]}}`
	var state audioclosure.State
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		t.Fatalf("candidate source enum failed to deserialize: %v", err)
	}
	if len(state.Frontier.Candidates) != 3 {
		t.Fatalf("candidate rows were dropped at deserialization: %+v", state.Frontier.Candidates)
	}
	projected := audioClosureStateMap(state)
	rows := freeStateMapRows(firstMapFromAny(firstMapFromAny(projected["hypothesis_frontier"]))["candidates"])
	for _, row := range rows {
		if firstStringFromMap(row, "id") == "candidate:foreign" {
			t.Fatalf("foreign source survived the gate projection")
		}
	}
}
