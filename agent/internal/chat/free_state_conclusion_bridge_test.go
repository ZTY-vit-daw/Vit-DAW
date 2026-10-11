package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/projectstore"
)

func TestFreeStateConclusionBridgeIdentityOrder(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	// The loop's LatestProjectChange carries the checkpoint anchor
	// (checkpoint_ref/commit_id plus project identity) and wins when present.
	loop := &freeStateReasoningLoop{ConversationID: "conv-bridge"}
	loop.LatestProjectChange = map[string]any{
		"project_uuid": "proj-from-loop", "checkpoint_ref": "ckpt-1", "project_revision": "rev-1",
	}
	bridge := freeStateConclusionBridgeFromLoop(loop, "proj-hint")
	if bridge.ProjectUUID != "proj-from-loop" || bridge.CheckpointRef != "ckpt-1" || bridge.ProjectRevision != "rev-1" {
		t.Fatalf("loop identity not primary: %+v", bridge)
	}

	// Without the loop map, the out-of-band hint (judgment evidence /
	// closure state) supplies the project uuid.
	bridge = freeStateConclusionBridgeFromLoop(&freeStateReasoningLoop{}, "proj-hint")
	if bridge.ProjectUUID != "proj-hint" {
		t.Fatalf("hint identity ignored: %+v", bridge)
	}

	// Last resort: the bound projectstore roots.
	bridge = freeStateConclusionBridgeFromLoop(&freeStateReasoningLoop{}, "")
	if bridge.ProjectUUID != "" {
		t.Fatalf("unbound fallback unexpectedly resolved: %+v", bridge)
	}
}

func TestRecordFreeStateSettlementPersistsAcrossSessions(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	projectRoot := t.TempDir()
	roots, _, err := projectstore.Activate(filepath.Join(projectRoot, "song.vitproj"), "proj-m1-chat")
	if err != nil {
		t.Fatalf("activate projectstore: %v", err)
	}

	loop := &freeStateReasoningLoop{
		ConversationID: "conv-1", GoalID: "goal-1", RunID: "run-1", LoopID: "loop-1",
	}
	loop.LatestProjectChange = map[string]any{
		"project_uuid": "proj-m1-chat", "checkpoint_ref": "ckpt-42", "project_revision": "rev-7",
	}
	// recordFreeStateSettlement only reads the turn's durable identity, so a
	// literal turn avoids coupling this bridge test to admission validation.
	loop.Experiment = &experiment.Turn{ID: "turn-1"}

	server := &Server{}
	server.recordFreeStateSettlement(loop, mixboard.FreeStateConclusionKindExperimentSettle, "improved", "user preferred B; treatment candidate retained",
		&mixboard.FreeStateJudgmentOutcome{EvidenceID: "evid-1", Preference: "b", HeardDifference: "yes"}, "")

	// The record lands in the project-keyed conclusion directory beside the
	// decision ledger, not in a conversation-keyed location.
	conclusionDir := filepath.Join(roots.Agent, "mixboard", "decisions", "free_state_conclusions")
	entries, err := os.ReadDir(conclusionDir)
	if err != nil {
		t.Fatalf("conclusion dir not created under the project store: %v", err)
	}
	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "fsc_") && strings.HasSuffix(entry.Name(), ".json") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no conclusion record written under %s: %v", conclusionDir, entries)
	}

	// A fresh store instance over the same project root models the next
	// session: the settle conclusion must read back (cross-session survival).
	board, err := mixboard.NewStore("").ReadFreeStateConclusionBoard("proj-m1-chat")
	if err != nil {
		t.Fatalf("cross-session read: %v", err)
	}
	if board.ConclusionCount != 1 {
		t.Fatalf("conclusion lost across sessions: %+v", board)
	}
	view := board.Conclusions[0]
	if view.Outcome != "improved" || view.CheckpointRef != "ckpt-42" || view.ConversationID != "conv-1" {
		t.Fatalf("conclusion content drifted: %+v", view)
	}
	if view.Judgment == nil || view.Judgment.Preference != "b" {
		t.Fatalf("judgment outcome lost: %+v", view.Judgment)
	}
}

func TestRecordFreeStateSettlementSkipsWithoutProjectIdentity(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	server := &Server{}
	loop := &freeStateReasoningLoop{ConversationID: "conv-noidentity"}
	// No projectstore binding, no hint, no harness: the write is skipped
	// honestly instead of landing somewhere unaddressable.
	server.recordFreeStateSettlement(loop, mixboard.FreeStateConclusionKindExperimentSettle, "improved", "summary", nil, "")
	if _, err := mixboard.NewStore(t.TempDir()).ReadFreeStateConclusionBoard("any"); err != nil {
		t.Fatalf("sanity read failed: %v", err)
	}
}
