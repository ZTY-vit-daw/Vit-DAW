package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/journal"
)

// B12-1 nails. The blind tier exchanges only the physical render behind the
// fixed A/B labels; every assertion below therefore states BOTH the label the
// user picked and the physical side it carried at the moment of the judgment.

func d1RenderRevisionForTest(experimentID, phase, projectRevision string) string {
	return "d1_render:" + experimentID + ":" + phase + ":" + projectRevision + ":" + strings.Repeat("a", 16)
}

// blindAuditionCandidateRowsForTest builds the two candidate rows the way the
// pre-judgment session snapshot carries them: same field family as the
// production rows (source_ref / render_revision / checkpoint identity), with the
// physical renders exchanged when swapped is set.
func blindAuditionCandidateRowsForTest(t *testing.T, experimentID, baselineRef string, swapped bool) []any {
	t.Helper()
	root := t.TempDir()
	before := map[string]any{
		"status":     "ready",
		"source_ref": filepath.Join(root, "before_revision_7.wav"), "preview_ref": "audio_file:beforehash",
		"checkpoint_ref": baselineRef, "commit_id": baselineRef, "project_revision": "7",
		"render_revision": d1RenderRevisionForTest(experimentID, "before", "7"),
	}
	after := map[string]any{
		"status":     "ready",
		"source_ref": filepath.Join(root, "after_revision_8.wav"), "preview_ref": "audio_file:afterhash",
		"checkpoint_ref": "action:d1-action", "commit_id": "action:d1-action", "project_revision": "8",
		"render_revision": d1RenderRevisionForTest(experimentID, "after", "8"),
	}
	// The blind draw exchanges the physical render behind the fixed labels; the
	// label ids stay put, exactly as d1AuditionCandidatesForAssignment does.
	physicalA, physicalB := before, after
	if swapped {
		physicalA, physicalB = after, before
	}
	row := func(id, label string, physical map[string]any) map[string]any {
		out := map[string]any{"id": id, "label": label}
		for key, value := range physical {
			out[key] = value
		}
		return out
	}
	return []any{row("candidate-a", "A", physicalA), row("candidate-b", "B", physicalB)}
}

// blindAuditionLoopForTest parks the D1 fixture at the judgment boundary of a
// blind session whose draw either exchanged the renders (swapped) or kept the
// canonical order.
func blindAuditionLoopForTest(t *testing.T, s *Server, loop freeStateReasoningLoop, swapped bool) freeStateReasoningLoop {
	t.Helper()
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	loop.AuditionSessionSnapshot = map[string]any{
		"session_id": loop.AuditionSessionID, "conversation_id": loop.ConversationID, "status": "ready",
		"scope": "target", "project_revision": "8", "blind": true,
		"candidates": blindAuditionCandidateRowsForTest(t, loop.Experiment.ID, round.CheckpointRef, swapped),
	}
	s.storeFreeStateLoop(loop)
	stored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok {
		t.Fatal("blind audition fixture was not stored")
	}
	return stored
}

func postAuditionJudgmentForTest(t *testing.T, s *Server, loop freeStateReasoningLoop, heard, preference string) map[string]any {
	t.Helper()
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"8","heard_difference":%q,"preference":%q}`,
		loop.ConversationID, loop.Experiment.ID, round.ID, loop.AuditionSessionID, heard, preference)
	recorder := httptest.NewRecorder()
	s.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("judgment rejected: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("judgment response is not JSON: %v body=%s", err, recorder.Body.String())
	}
	return decoded
}

func rolledBackFixtureForTest(s *Server) *d1UndoSender {
	sender := &d1UndoSender{}
	s.harness = harness.NewWithSender(sender, nil, nil)
	s.harness.JournalRecord(journal.Action{AgentActionID: "d1-action", Domain: "daw", Status: journal.StatusSucceeded, Command: map[string]any{"cmd": "set_volume"}})
	s.harness.JournalRecord(journal.Action{AgentActionID: "unrelated-later-action", Domain: "daw", Status: journal.StatusSucceeded, Command: map[string]any{"cmd": "set_pan"}})
	return sender
}

// Nail 1 (highest risk): a blind session put the treatment render behind label
// A. Selecting A therefore confirms the treatment and must retain — never roll
// back the very change the user preferred.
func TestB12BlindSwapSelectingTreatmentRetainsInsteadOfRollingBack(t *testing.T) {
	s, loop := d1ServerAtHumanAuditionReady(t)
	sender := rolledBackFixtureForTest(s)
	loop = blindAuditionLoopForTest(t, s, loop, true)

	response := postAuditionJudgmentForTest(t, s, loop, "yes", "a")

	stored, _ := s.freeStateLoop(loop.ConversationID)
	round, _ := stored.Experiment.CurrentRound()
	if stored.Experiment.Status != experiment.StatusSettled || stored.Experiment.Outcome != experiment.OutcomeImproved {
		t.Fatalf("blind A(treatment) judgment did not retain: status=%s outcome=%s round_decision=%s",
			stored.Experiment.Status, stored.Experiment.Outcome, round.Decision)
	}
	if round.Decision != experiment.DecisionRetain {
		t.Fatalf("blind A(treatment) round decision=%s want retained", round.Decision)
	}
	if len(sender.commands) != 0 {
		t.Fatalf("blind A(treatment) judgment executed a rollback: %+v", sender.commands)
	}
	for key, want := range map[string]any{
		"human_confirmed": true, "ambiguous": false, "rolled_back": false, "settled": true,
		"disposition": "retain",
	} {
		if stored.D1Receipt[key] != want {
			t.Fatalf("receipt[%s]=%v want %v (receipt=%+v)", key, stored.D1Receipt[key], want, stored.D1Receipt)
		}
	}
	// The receipt layers must describe the action that actually ran: the
	// physical retain is an improvement, not a plateau or a rollback.
	if net := firstStringFromMap(firstMapFromAny(firstMapFromAny(stored.D1Receipt["layers"])["net_outcome"]), "status"); net != "improved" {
		t.Fatalf("receipt net outcome=%q want improved (receipt=%+v)", net, stored.D1Receipt)
	}
	if count, _ := treatmentNumber(stored.D1Receipt, "forward_mutation_count"); count != 1 {
		t.Fatalf("forward_mutation_count=%v want 1", count)
	}
	if count, _ := treatmentNumber(stored.D1Receipt, "rollback_compensation_count"); count != 0 {
		t.Fatalf("rollback_compensation_count=%v want 0", count)
	}

	// The un-blinding states the physical assignment and the action it produced.
	disclosure := firstMapFromAny(response["blind_disclosure"])
	if firstStringFromMap(disclosure, "candidate_a_physical") != auditionPhysicalAfter ||
		firstStringFromMap(disclosure, "candidate_b_physical") != auditionPhysicalBefore ||
		firstStringFromMap(disclosure, "selected") != "a" ||
		firstStringFromMap(disclosure, "action") != "retain" ||
		firstStringFromMap(disclosure, "summary") != "你选的 A 是改动后状态 · 已保留" ||
		firstStringFromMap(disclosure, "mapping_source") != "render_revision" {
		t.Fatalf("blind disclosure=%+v", disclosure)
	}
	events, _ := s.agentEventsSince(loop.ConversationID, 0, 200)
	var disclosed bool
	for _, event := range events {
		if event.Type != auditionBlindDisclosureEvent {
			continue
		}
		disclosed = firstStringFromMap(firstMapFromAny(event.Payload["blind_disclosure"]), "action") == "retain"
	}
	if !disclosed {
		t.Fatalf("no %s event carried the retain disclosure", auditionBlindDisclosureEvent)
	}
}

// Nail 2: the same blind session, but the user selects the label that carries
// the baseline render. That is the rollback, and it targets the original action.
func TestB12BlindSwapSelectingBaselineRollsBack(t *testing.T) {
	s, loop := d1ServerAtHumanAuditionReady(t)
	sender := rolledBackFixtureForTest(s)
	loop = blindAuditionLoopForTest(t, s, loop, true)

	response := postAuditionJudgmentForTest(t, s, loop, "yes", "b")

	stored, _ := s.freeStateLoop(loop.ConversationID)
	round, _ := stored.Experiment.CurrentRound()
	if stored.Experiment.Outcome != experiment.OutcomeRolledBack || round.Decision != experiment.DecisionRollback {
		t.Fatalf("blind B(baseline) judgment did not roll back: outcome=%s decision=%s", stored.Experiment.Outcome, round.Decision)
	}
	if len(sender.commands) != 1 || sender.commands[0]["cmd"] != "undo" || sender.commands[0]["target_action_id"] != "d1-action" {
		t.Fatalf("rollback commands=%+v", sender.commands)
	}
	if stored.D1Receipt["rolled_back"] != true || stored.D1Receipt["human_confirmed"] != true {
		t.Fatalf("receipt=%+v", stored.D1Receipt)
	}
	disclosure := firstMapFromAny(response["blind_disclosure"])
	if firstStringFromMap(disclosure, "candidate_a_physical") != auditionPhysicalAfter ||
		firstStringFromMap(disclosure, "action") != "rollback" ||
		firstStringFromMap(disclosure, "summary") != "你选的 B 是改动前状态 · 已回滚到改动前" {
		t.Fatalf("blind disclosure=%+v", disclosure)
	}
}

// Nail 3: the non-blind path resolves to the canonical assignment, so the
// label-level semantics, the receipt, and the response shape are unchanged.
func TestB12NonBlindJudgmentKeepsLabelSemantics(t *testing.T) {
	for _, test := range []struct {
		name, heard, preference, outcome, decision string
		rollback                                   bool
	}{
		{name: "prefer B keeps treatment", heard: "yes", preference: "b", outcome: "improved", decision: string(experiment.DecisionRetain)},
		{name: "prefer A rolls back", heard: "yes", preference: "a", outcome: "rolled_back", decision: string(experiment.DecisionRollback), rollback: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, loop := d1ServerAtHumanAuditionReady(t)
			sender := rolledBackFixtureForTest(s)
			round, _ := loop.Experiment.CurrentRound()
			rows := blindAuditionCandidateRowsForTest(t, loop.Experiment.ID, round.CheckpointRef, false)
			for _, row := range rows {
				delete(firstMapFromAny(row), "checkpoint_ref")
				delete(firstMapFromAny(row), "commit_id")
			}
			loop.AuditionSessionSnapshot = map[string]any{
				"session_id": loop.AuditionSessionID, "conversation_id": loop.ConversationID, "status": "ready",
				"scope": "target", "project_revision": "8", "blind": false, "candidates": rows,
			}
			s.storeFreeStateLoop(loop)

			response := postAuditionJudgmentForTest(t, s, loop, test.heard, test.preference)

			stored, _ := s.freeStateLoop(loop.ConversationID)
			storedRound, _ := stored.Experiment.CurrentRound()
			if string(stored.Experiment.Outcome) != test.outcome || string(storedRound.Decision) != test.decision {
				t.Fatalf("non-blind outcome=%s decision=%s want %s/%s", stored.Experiment.Outcome, storedRound.Decision, test.outcome, test.decision)
			}
			if test.rollback != (len(sender.commands) == 1) {
				t.Fatalf("non-blind rollback commands=%+v", sender.commands)
			}
			if _, present := response["blind_disclosure"]; present {
				t.Fatalf("non-blind response disclosed a blind mapping: %+v", response)
			}
		})
	}
}

// Nail 4: an insufficiency report stays the insufficiency path in a blind
// session — no candidate is preferred, so no direction is claimed.
func TestB12BlindNoDifferenceJudgmentKeepsInsufficiencyPath(t *testing.T) {
	s, loop := d1ServerAtHumanAuditionReady(t)
	sender := rolledBackFixtureForTest(s)
	loop = blindAuditionLoopForTest(t, s, loop, true)

	response := postAuditionJudgmentForTest(t, s, loop, "no", "a")

	stored, _ := s.freeStateLoop(loop.ConversationID)
	round, _ := stored.Experiment.CurrentRound()
	if stored.Experiment.Outcome != experiment.OutcomeNeedsJudgment || round.Decision != experiment.DecisionStopped {
		t.Fatalf("blind no-difference outcome=%s decision=%s", stored.Experiment.Outcome, round.Decision)
	}
	if len(sender.commands) != 0 || stored.D1Receipt["rolled_back"] == true || stored.D1Receipt["ambiguous"] != true {
		t.Fatalf("blind no-difference mutated the project: commands=%+v receipt=%+v", sender.commands, stored.D1Receipt)
	}
	disclosure := firstMapFromAny(response["blind_disclosure"])
	if firstStringFromMap(disclosure, "action") != "stopped" || firstStringFromMap(disclosure, "selected") != "" {
		t.Fatalf("blind no-difference disclosure=%+v", disclosure)
	}
	// The panel keeps its own label-neutral copy for this outcome.
	if _, present := disclosure["summary"]; present {
		t.Fatalf("no-difference disclosure must not claim a physical direction: %+v", disclosure)
	}
}

// Nail 5: before the judgment lands, neither the session snapshot nor the event
// stream names the label -> physical assignment.
func TestB12BlindSessionLeaksNoPhysicalAssignmentBeforeJudgment(t *testing.T) {
	s, loop := prepareBlindAuditionForTest(t, true)

	stored, _ := s.freeStateLoop(loop.ConversationID)
	if !auditionSessionIsBlind(stored.AuditionSessionSnapshot) {
		t.Fatalf("blind session did not record its mode: %+v", stored.AuditionSessionSnapshot)
	}
	if _, present := stored.AuditionSessionSnapshot["blind_disclosure"]; present {
		t.Fatalf("pre-judgment snapshot already carries the disclosure: %+v", stored.AuditionSessionSnapshot)
	}
	if leaked := auditionPhysicalMarkerKeysForTest(stored.AuditionSessionSnapshot); len(leaked) != 0 {
		t.Fatalf("pre-judgment snapshot leaks physical assignment fields: %v", leaked)
	}
	// The candidate rows keep exactly the pre-existing provenance key family.
	wantKeys := []string{"branch_ref", "checkpoint_ref", "commit_id", "id", "label", "preview_ref", "preview_revision", "project_path", "project_revision", "project_uuid", "render_revision", "scope", "source_kind", "source_ref", "status", "worktree_ref"}
	for _, row := range auditionCandidateRows(stored.AuditionSessionSnapshot) {
		for key := range row {
			if !containsString(wantKeys, key) {
				t.Fatalf("candidate row gained a key %q: %+v", key, row)
			}
		}
	}
	events, _ := s.agentEventsSince(loop.ConversationID, 0, 200)
	found := 0
	for _, event := range events {
		if !strings.HasPrefix(event.Type, "audition.") {
			continue
		}
		found++
		if event.Type == auditionBlindDisclosureEvent {
			t.Fatalf("disclosure event emitted before the judgment: %+v", event)
		}
		if leaked := auditionPhysicalMarkerKeysForTest(event.Payload); len(leaked) != 0 {
			t.Fatalf("pre-judgment %s event leaks physical assignment fields: %v", event.Type, leaked)
		}
	}
	if found == 0 {
		t.Fatal("no audition event was emitted by the blind prepare path")
	}
}

// The key-family table: the render revision is primary, the file name and the
// round baseline are fallbacks, and a collision resolves to "unknown" so the
// settlement fails closed instead of guessing a direction.
func TestB12BlindPhysicalMappingKeyFamilies(t *testing.T) {
	baseline := "checkpoint-7"
	rows := func(swapped bool) []any {
		return blindAuditionCandidateRowsForTest(t, "exp-1", baseline, swapped)
	}
	for _, test := range []struct {
		name       string
		swapped    bool
		mutate     func(evidence *experiment.UserJudgmentEvidence)
		wantSource string
		wantA      string
		wantOK     bool
	}{
		{name: "render revision canonical", wantSource: "render_revision", wantA: auditionPhysicalBefore, wantOK: true},
		{name: "render revision swapped", swapped: true, wantSource: "render_revision", wantA: auditionPhysicalAfter, wantOK: true},
		{
			name: "file name fallback", wantSource: "candidate_source_ref", wantA: auditionPhysicalBefore, wantOK: true,
			mutate: func(e *experiment.UserJudgmentEvidence) {
				e.CandidateARenderRevision = ""
				e.CandidateBRenderRevision = ""
			},
		},
		{
			name: "baseline checkpoint fallback", wantSource: "checkpoint_identity", wantA: auditionPhysicalAfter, wantOK: true, swapped: true,
			mutate: func(e *experiment.UserJudgmentEvidence) {
				e.CandidateARenderRevision = ""
				e.CandidateBRenderRevision = ""
				e.CandidateARef = "opaque-a"
				e.CandidateBRef = "opaque-b"
			},
		},
		{
			name: "both labels on one side is a collision", wantOK: false,
			mutate: func(e *experiment.UserJudgmentEvidence) {
				e.CandidateBRenderRevision = e.CandidateARenderRevision
				e.CandidateBRef = e.CandidateARef
				e.CandidateBCheckpointRef, e.CandidateBCommitID = "", ""
			},
		},
		{
			name: "disagreeing families are not stable", wantOK: false,
			mutate: func(e *experiment.UserJudgmentEvidence) {
				e.CandidateARef = "after_revision_8.wav"
				e.CandidateBRef = "before_revision_7.wav"
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			evidence := evidenceForTest(t, rows(test.swapped))
			if test.mutate != nil {
				test.mutate(&evidence)
			}
			mapping, ok := auditionPhysicalMappingForEvidence(evidence, baseline)
			if ok != test.wantOK {
				t.Fatalf("mapped=%v want %v (mapping=%+v)", ok, test.wantOK, mapping)
			}
			if !ok {
				return
			}
			if mapping.source != test.wantSource || mapping.sideA != test.wantA {
				t.Fatalf("mapping=%+v want source=%s sideA=%s", mapping, test.wantSource, test.wantA)
			}
		})
	}
}

// A blind session whose provenance cannot be resolved refuses to record the
// judgment at all: the evidence row stays unwritten, so no unresolvable A/B
// preference can settle into a direction.
func TestB12BlindUnmappableJudgmentFailsClosedBeforeEvidenceLands(t *testing.T) {
	s, loop := d1ServerAtHumanAuditionReady(t)
	rolledBackFixtureForTest(s)
	loop = blindAuditionLoopForTest(t, s, loop, true)
	for index, row := range auditionCandidateRows(loop.AuditionSessionSnapshot) {
		row["render_revision"] = "opaque-" + fmt.Sprint(index)
		if strings.Contains(firstStringFromMap(row, "source_ref"), "revision_") {
			row["source_ref"] = "opaque-" + fmt.Sprint(index)
		}
		row["checkpoint_ref"], row["commit_id"] = "", ""
	}
	s.storeFreeStateLoop(loop)

	round, _ := loop.Experiment.CurrentRound()
	body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"8","heard_difference":"yes","preference":"a"}`,
		loop.ConversationID, loop.Experiment.ID, round.ID, loop.AuditionSessionID)
	recorder := httptest.NewRecorder()
	s.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "physical candidate mapping") {
		t.Fatalf("unmappable blind judgment: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	stored, _ := s.freeStateLoop(loop.ConversationID)
	storedRound, _ := stored.Experiment.CurrentRound()
	if len(storedRound.UserJudgmentEvidence) != 0 || stored.Experiment.Status == experiment.StatusSettled {
		t.Fatalf("unmappable blind judgment landed evidence: round=%+v", storedRound.UserJudgmentEvidence)
	}
}

// The construction invariant: the labels never move, only the physical render
// behind them.
func TestB12D1AuditionCandidatesBlindSwapKeepsLabelsAndExchangesSources(t *testing.T) {
	beforePath := writeD1WAVForTest(t, "before.wav")
	afterPath := writeD1WAVForTest(t, "after.wav")
	before := map[string]any{"status": "ready", "file_path": beforePath, "project_revision": "7", "render_revision": "d1_render:exp:before:7:" + strings.Repeat("b", 16), "preview_revision": "sha256:before", "sha256": "beforehash"}
	after := map[string]any{"status": "ready", "file_path": afterPath, "project_revision": "8", "render_revision": "d1_render:exp:after:8:" + strings.Repeat("c", 16), "preview_revision": "sha256:after", "sha256": "afterhash"}
	for _, swapped := range []bool{false, true} {
		candidates, err := d1AuditionCandidatesForAssignment(before, after, "checkpoint-7", "checkpoint-8", "project.vit", "project-1", swapped)
		if err != nil {
			t.Fatal(err)
		}
		wantFirst, wantSecond := beforePath, afterPath
		wantFirstCommit := "checkpoint-7"
		if swapped {
			wantFirst, wantSecond = afterPath, beforePath
			wantFirstCommit = "checkpoint-8"
		}
		if candidates[0].ID != "candidate-a" || candidates[0].Label != "A" || candidates[1].ID != "candidate-b" || candidates[1].Label != "B" {
			t.Fatalf("swapped=%v labels moved: %+v", swapped, candidates)
		}
		if candidates[0].SourceRef != wantFirst || candidates[1].SourceRef != wantSecond || candidates[0].CheckpointRef != wantFirstCommit {
			t.Fatalf("swapped=%v sources=%q/%q commits=%q/%q", swapped, candidates[0].SourceRef, candidates[1].SourceRef, candidates[0].CheckpointRef, candidates[1].CheckpointRef)
		}
		evidence := experiment.UserJudgmentEvidence{
			CandidateARef: candidates[0].SourceRef, CandidateBRef: candidates[1].SourceRef,
			CandidateACheckpointRef: candidates[0].CheckpointRef, CandidateBCheckpointRef: candidates[1].CheckpointRef,
			CandidateARenderRevision: candidates[0].RenderRevision, CandidateBRenderRevision: candidates[1].RenderRevision,
		}
		mapping, ok := auditionPhysicalMappingForEvidence(evidence, "checkpoint-7")
		if !ok || mapping.sideA != firstNonEmpty(map[bool]string{true: auditionPhysicalAfter, false: auditionPhysicalBefore}[swapped], auditionPhysicalBefore) {
			t.Fatalf("swapped=%v round-trip mapping=%+v ok=%v", swapped, mapping, ok)
		}
	}
}

// The prepared session carries the blind mode (never the draw) and the swap is
// decided once per session.
func TestB12BlindPrepareDrawsOncePerSession(t *testing.T) {
	for _, test := range []struct {
		name         string
		draw         bool
		wantSwapped  bool
		wantSnapshot bool
	}{
		{name: "draw exchanges", draw: true, wantSwapped: true},
		{name: "draw keeps the canonical order", draw: false, wantSwapped: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, loop := prepareBlindAuditionForTest(t, test.draw)
			request := s.auditionKernel.(*fakeAuditionKernel).prepareRequests
			if len(request) != 1 {
				t.Fatalf("prepare calls=%d", len(request))
			}
			candidates := request[0].Candidates
			if candidates[0].ID != "candidate-a" || candidates[1].ID != "candidate-b" {
				t.Fatalf("labels moved: %+v", candidates)
			}
			swapped := strings.Contains(candidates[0].SourceRef, "after_revision_")
			if swapped != test.wantSwapped {
				t.Fatalf("swapped=%v want %v (a=%s b=%s)", swapped, test.wantSwapped, candidates[0].SourceRef, candidates[1].SourceRef)
			}
			stored, _ := s.freeStateLoop(loop.ConversationID)
			if !auditionSessionIsBlind(stored.AuditionSessionSnapshot) {
				t.Fatalf("blind mode missing from the snapshot: %+v", stored.AuditionSessionSnapshot)
			}
			if keys := auditionPhysicalMarkerKeysForTest(stored.AuditionSessionSnapshot); len(keys) != 0 {
				t.Fatalf("snapshot leaks the draw: %v", keys)
			}
		})
	}
}

// prepareBlindAuditionForTest drives the real prepare path with the blind tier
// enabled and the draw pinned.
func prepareBlindAuditionForTest(t *testing.T, draw bool) (*Server, freeStateReasoningLoop) {
	t.Helper()
	t.Setenv(auditionBlindEnvVar, "1")
	s := New(nil, nil, nil)
	s.auditionKernel = &fakeAuditionKernel{}
	s.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))
	s.auditionBlindDraw = func() bool { return draw }
	loop := d1EvaluatedLoopForTest(t)
	beforePath := writeD1WAVForTest(t, "before_revision_7.wav")
	afterPath := writeD1WAVForTest(t, "after_revision_8.wav")
	loop.D1State = map[string]any{
		"before_render": map[string]any{"phase": "before", "status": "ready", "file_path": beforePath, "project_revision": "7", "sha256": strings.Repeat("d", 64), "render_revision": d1RenderRevisionForTest(loop.Experiment.ID, "before", "7"), "preview_revision": "sha256:before"},
		"after_render":  map[string]any{"phase": "after", "status": "ready", "file_path": afterPath, "project_revision": "8", "sha256": strings.Repeat("e", 64), "render_revision": d1RenderRevisionForTest(loop.Experiment.ID, "after", "8"), "preview_revision": "sha256:after"},
	}
	s.storeFreeStateLoop(loop)
	if err := s.prepareFreeStateAudition(context.Background(), &loop); err != nil {
		t.Fatalf("blind prepare failed: %v", err)
	}
	stored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok {
		t.Fatal("prepared loop was not stored")
	}
	return s, stored
}

func evidenceForTest(t *testing.T, rows []any) experiment.UserJudgmentEvidence {
	t.Helper()
	loop := freeStateReasoningLoop{ConversationID: "c", Experiment: &experiment.Turn{ID: "turn"}, AuditionSessionSnapshot: map[string]any{"session_id": "audition-1", "project_revision": "8", "candidates": rows}}
	round := experiment.Round{ID: "round", ProjectRevision: "8", TargetResponse: &experiment.TargetEvaluation{}}
	return buildUserJudgmentEvidence(loop, round, "audition-1", experiment.HeardDifferenceYes, experiment.PreferenceA, nil, "")
}

func auditionPhysicalMarkerKeysForTest(value any) []string {
	return auditionPhysicalMarkerKeysAt(value, "")
}

func auditionPhysicalMarkerKeysAt(value any, path string) []string {
	markers := []string{"blind_disclosure", "physical", "mapping", "assignment", "swap", "deblind", "unblind", "reveal"}
	found := []string{}
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			lower := strings.ToLower(key)
			for _, marker := range markers {
				if strings.Contains(lower, marker) {
					found = append(found, path+"."+key)
					break
				}
			}
			found = append(found, auditionPhysicalMarkerKeysAt(item, path+"."+key)...)
		}
	case []any:
		for index, item := range typed {
			found = append(found, auditionPhysicalMarkerKeysAt(item, fmt.Sprintf("%s[%d]", path, index))...)
		}
	}
	return found
}
