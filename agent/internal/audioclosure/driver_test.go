package audioclosure

import (
	"encoding/json"
	"testing"
	"time"
)

var testNow = time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)

func newTestClosure(t *testing.T) State {
	t.Helper()
	state, err := Start(StartRequest{
		ClosureID: "closure-1", ConversationID: "conversation-1", GoalID: "goal-1", RunID: "run-1",
		ProjectUUID: "project-1", ProjectRevision: "revision-1", OriginalIntent: "make the vocal less harsh",
		Mode: ModeTreatment, Scope: Scope{Kind: "track", ID: "track-vocal"}, Now: testNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func admitTestRound(t *testing.T, driver Driver, state State, offset int) State {
	t.Helper()
	next, admitted, err := driver.AdmitRound(state, state.Revision, testNow.Add(time.Duration(offset)*time.Minute))
	if err != nil || !admitted {
		t.Fatalf("admit round: admitted=%v err=%v", admitted, err)
	}
	return next
}

func TestClosureRoundBudgetPersistsAcrossEventReplay(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	for round := 1; round <= DefaultPolicy().MaxClosureRounds; round++ {
		state = admitTestRound(t, driver, state, round)
		frontier := HypothesisFrontier{HypothesisIDs: []string{"hypothesis-" + string(rune('a'+round))}}
		var err error
		state, _, err = driver.UpdateFrontier(state, state.Revision, frontier, ActionabilityUnknown, testNow.Add(time.Duration(round)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		state, err = driver.CompleteRound(state, state.Revision, testNow.Add(time.Duration(round)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
	}
	if !state.Terminal() || state.Settlement.Reason != StopRoundLimit || state.RoundsStarted != 6 {
		t.Fatalf("expected round_limit after six persistent rounds, got %+v", state)
	}
	raw, _ := json.Marshal(state.Events)
	var restoredEvents []Event
	if err := json.Unmarshal(raw, &restoredEvents); err != nil {
		t.Fatal(err)
	}
	restored, err := Fold(restoredEvents)
	if err != nil {
		t.Fatal(err)
	}
	if restored.RoundsStarted != 6 || restored.Settlement.Reason != StopRoundLimit || restored.Revision != state.Revision {
		t.Fatalf("event replay lost closure budget: %+v", restored)
	}
}

func TestDuplicateObservationDoesNotConsumeUniqueBudgetOrCountAsProgress(t *testing.T) {
	driver := Driver{}
	state := admitTestRound(t, driver, newTestClosure(t), 1)
	key := ObservationKey{ProjectUUID: "project-1", ProjectRevision: "revision-1", Scope: Scope{Kind: "track", ID: "track-vocal"}, TargetRef: "track-vocal", ViewIDs: []string{"frequency_relationship", "dynamic_profile"}, ObservationMode: "ccb", Tap: "post_fx", TimeWindow: "0:00-0:20"}
	first, err := driver.RecordObservation(state, state.Revision, key, "observation-1", testNow)
	if err != nil || !first.Accepted || first.Duplicate {
		t.Fatalf("first observation: %+v err=%v", first, err)
	}
	duplicate, err := driver.RecordObservation(first.State, first.State.Revision, key, "observation-2", testNow)
	if err != nil || !duplicate.Duplicate || duplicate.Accepted {
		t.Fatalf("duplicate observation: %+v err=%v", duplicate, err)
	}
	if len(duplicate.State.Observations) != 1 || len(duplicate.State.ObservationOrder) != 1 {
		t.Fatalf("duplicate consumed unique evidence budget: %+v", duplicate.State.ObservationOrder)
	}
}

func TestObservationFingerprintBindsRevisionScopeViewsTapAndWindow(t *testing.T) {
	base := ObservationKey{ProjectUUID: "p", ProjectRevision: "r1", Scope: Scope{Kind: "track", ID: "t1"}, TargetRef: "t1", ViewIDs: []string{"b", "a"}, ObservationMode: "ccb", Tap: "post_fx", TimeWindow: "first_chorus"}
	want, err := ObservationFingerprint(base)
	if err != nil {
		t.Fatal(err)
	}
	reordered := base
	reordered.ViewIDs = []string{"a", "b", "a"}
	got, _ := ObservationFingerprint(reordered)
	if got != want {
		t.Fatalf("view ordering should not change fingerprint: %s != %s", got, want)
	}
	variants := []ObservationKey{base, base, base, base}
	variants[0].ProjectRevision = "r2"
	variants[1].Scope.ID = "t2"
	variants[2].Tap = "pre_fx"
	variants[3].TimeWindow = "second_chorus"
	for _, variant := range variants {
		fingerprint, _ := ObservationFingerprint(variant)
		if fingerprint == want {
			t.Fatalf("changed observation identity did not change fingerprint: %+v", variant)
		}
	}
}

func TestEvidenceCeilingAllowsFourObservationsAndRejectsFifth(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	for i := 0; i < DefaultPolicy().MaxUniqueObservations; i++ {
		state = admitTestRound(t, driver, state, i+1)
		outcome, err := driver.RecordObservation(state, state.Revision, ObservationKey{ProjectUUID: "project-1", ProjectRevision: "revision-1", Scope: Scope{Kind: "track", ID: "track-vocal"}, ViewIDs: []string{"view"}, TimeWindow: string(rune('a' + i))}, "observation", testNow)
		if err != nil || !outcome.Accepted {
			t.Fatalf("observation %d: %+v err=%v", i, outcome, err)
		}
		state, _, err = driver.UpdateFrontier(outcome.State, outcome.State.Revision,
			HypothesisFrontier{HypothesisIDs: []string{string(rune('a' + i))}}, ActionabilityUnknown, testNow)
		if err != nil {
			t.Fatalf("frontier %d: %v", i, err)
		}
		state, err = driver.CompleteRound(state, state.Revision, testNow)
		if err != nil {
			t.Fatal(err)
		}
	}
	state = admitTestRound(t, driver, state, DefaultPolicy().MaxUniqueObservations+1)
	fifth, err := driver.RecordObservation(state, state.Revision, ObservationKey{ProjectUUID: "project-1", ProjectRevision: "revision-1", Scope: Scope{Kind: "track", ID: "track-vocal"}, ViewIDs: []string{"view"}, TimeWindow: "ceiling"}, "observation-ceiling", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !fifth.State.Terminal() || fifth.State.Settlement.Reason != StopEvidenceCeilingReached || len(fifth.State.Observations) != DefaultPolicy().MaxUniqueObservations {
		t.Fatalf("expected bounded evidence settlement, got %+v", fifth.State)
	}
}

func TestNewObservationAloneDoesNotCountAsClosureProgress(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	for i := 0; i < 2; i++ {
		state = admitTestRound(t, driver, state, i+1)
		outcome, err := driver.RecordObservation(state, state.Revision, ObservationKey{
			ProjectUUID: "project-1", ProjectRevision: "revision-1", Scope: Scope{Kind: "track", ID: "track-vocal"},
			ViewIDs: []string{"view"}, TimeWindow: string(rune('a' + i)),
		}, "observation", testNow)
		if err != nil || !outcome.Accepted {
			t.Fatalf("observation %d: %+v err=%v", i, outcome, err)
		}
		state, err = driver.CompleteRound(outcome.State, outcome.State.Revision, testNow)
		if err != nil {
			t.Fatalf("complete round %d: %v", i, err)
		}
	}
	if !state.Terminal() || state.Settlement.Reason != StopNoProgress || state.NoProgressStreak != 2 {
		t.Fatalf("new observation fingerprints incorrectly counted as progress: %+v", state)
	}
}

func TestAuthoritativeProjectChangeCountsAsEngineeringProgressOnlyOnce(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	state = admitTestRound(t, driver, state, 1)
	var err error
	state, err = driver.CompleteRound(state, state.Revision, testNow)
	if err != nil || state.NoProgressStreak != 1 {
		t.Fatalf("first no-progress round: state=%+v err=%v", state, err)
	}

	state = admitTestRound(t, driver, state, 2)
	state, recorded, err := driver.RecordProjectChange(state, state.Revision, "shadow_change_42", testNow)
	if err != nil || !recorded || state.LastProjectChangeID != "shadow_change_42" || !state.RoundHadProgress {
		t.Fatalf("project change was not recorded as engineering progress: state=%+v recorded=%v err=%v", state, recorded, err)
	}
	if _, recorded, err = driver.RecordProjectChange(state, state.Revision, "shadow_change_42", testNow); err != nil || recorded {
		t.Fatalf("duplicate project change should be a no-op: recorded=%v err=%v", recorded, err)
	}
	state, err = driver.CompleteRound(state, state.Revision, testNow)
	if err != nil || state.Terminal() || state.NoProgressStreak != 0 {
		t.Fatalf("engineering progress incorrectly settled or retained no-progress: state=%+v err=%v", state, err)
	}
}

func TestNoProgressSettlesAfterTwoCompletedRounds(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	for i := 0; i < 2; i++ {
		state = admitTestRound(t, driver, state, i+1)
		var err error
		state, err = driver.CompleteRound(state, state.Revision, testNow)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !state.Terminal() || state.Settlement.Reason != StopNoProgress || state.NoProgressStreak != 2 {
		t.Fatalf("expected no_progress settlement, got %+v", state)
	}
}

func TestProjectDiscoveryDoesNotSettleBeforeCandidateFrontier(t *testing.T) {
	driver := Driver{}
	state, err := Start(StartRequest{
		ClosureID: "project-closure", ConversationID: "conversation-1", GoalID: "goal-1", RunID: "run-1",
		ProjectUUID: "project-1", ProjectRevision: "revision-1", OriginalIntent: "inspect the project mix",
		Mode: ModeTreatment, Scope: Scope{Kind: "project", ID: "project-1"}, Now: testNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < DefaultPolicy().MaxNoProgressRounds; i++ {
		state = admitTestRound(t, driver, state, i+1)
		state, err = driver.CompleteRound(state, state.Revision, testNow)
		if err != nil {
			t.Fatal(err)
		}
	}
	if state.Terminal() || state.NoProgressStreak != DefaultPolicy().MaxNoProgressRounds {
		t.Fatalf("project discovery settled before candidate frontier: %+v", state)
	}
}

func TestOnlyUserChoiceRequiredCanBecomeClarification(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	if _, err := driver.Settle(state, state.Revision, StopInsufficientEvidence, "missing evidence", true, testNow); err == nil {
		t.Fatal("insufficient_evidence incorrectly became a clarification")
	}
	settled, err := driver.Settle(state, state.Revision, StopUserChoiceRequired, "choose a target", true, testNow)
	if err != nil || !settled.Settlement.NeedsUserClarification {
		t.Fatalf("user choice settlement failed: %+v err=%v", settled, err)
	}
}

func TestProtocolRepairBudgetSettlesInsteadOfAskingUser(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	state, allowed, err := driver.RecordProtocolRepair(state, state.Revision, testNow)
	if err != nil || !allowed || state.ModelProtocolRepairs != 1 {
		t.Fatalf("first repair: allowed=%v state=%+v err=%v", allowed, state, err)
	}
	state, allowed, err = driver.RecordProtocolRepair(state, state.Revision, testNow)
	if err != nil || allowed || !state.Terminal() || state.Settlement.Reason != StopModelProtocolFailure || state.Settlement.NeedsUserClarification {
		t.Fatalf("second repair should settle protocol failure: allowed=%v state=%+v err=%v", allowed, state, err)
	}
}

func TestCapabilityHandoffIsExactlyOnceAndMustSettleBeforeClosure(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	link := CapabilityLink{SessionID: "session-1", CapabilityID: "agent.effect.eq_control.v0", ActionID: "action-1", ExpectedProjectRevision: "revision-1"}
	state, started, err := driver.BeginCapability(state, state.Revision, link, testNow)
	if err != nil || !started || state.ActionAttempts != 1 {
		t.Fatalf("begin capability: started=%v state=%+v err=%v", started, state, err)
	}
	unchanged, started, err := driver.BeginCapability(state, state.Revision, link, testNow)
	if err != nil || started || unchanged.Revision != state.Revision || unchanged.ActionAttempts != 1 {
		t.Fatalf("duplicate begin was not idempotent: started=%v state=%+v err=%v", started, unchanged, err)
	}
	if _, err := driver.Settle(state, state.Revision, StopSatisfied, "done", false, testNow); err == nil {
		t.Fatal("closure settled while child capability was active")
	}
	state, recorded, err := driver.SettleCapability(state, state.Revision, CapabilitySettlement{SessionID: "session-1", ActionID: "action-1", Status: "completed"}, testNow)
	if err != nil || !recorded || state.ActiveCapability != nil || state.Phase != PhaseVerifying {
		t.Fatalf("settle capability: recorded=%v state=%+v err=%v", recorded, state, err)
	}
	unchanged, recorded, err = driver.SettleCapability(state, state.Revision, CapabilitySettlement{SessionID: "session-1", ActionID: "action-1", Status: "completed"}, testNow)
	if err != nil || recorded || unchanged.Revision != state.Revision {
		t.Fatalf("duplicate settlement was not idempotent: recorded=%v state=%+v err=%v", recorded, unchanged, err)
	}
}

func TestMemoryStoreCASAndSingleActiveOwner(t *testing.T) {
	store := NewMemoryStore()
	state := newTestClosure(t)
	if err := store.Create(state); err != nil {
		t.Fatal(err)
	}
	other := state
	other.ClosureID = "closure-2"
	other.Events = cloneEvents(state.Events)
	other.Events[0].ClosureID = "closure-2"
	other.Events[0].EventID = "closure-2:1"
	other, _ = Fold(other.Events)
	if err := store.Create(other); err == nil {
		t.Fatal("store admitted two active closures for one conversation")
	}
	driver := Driver{}
	next := admitTestRound(t, driver, state, 1)
	if err := store.Save(next, state.Revision+1); err == nil {
		t.Fatal("CAS accepted the wrong expected revision")
	}
	if err := store.Save(next, state.Revision); err != nil {
		t.Fatal(err)
	}
	loaded, ok := store.ActiveForConversation("conversation-1")
	if !ok || loaded.RoundsStarted != 1 {
		t.Fatalf("active closure lookup failed: %+v ok=%v", loaded, ok)
	}
	reopened := NewMemoryStore()
	if err := reopened.Restore(store.Snapshot()); err != nil {
		t.Fatal(err)
	}
	restored, ok := reopened.ActiveForConversation("conversation-1")
	if !ok || restored.Revision != next.Revision {
		t.Fatalf("snapshot restore failed: %+v ok=%v", restored, ok)
	}
}

func TestTerminalProjectionRejectsLaterEvents(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	settled, err := driver.Settle(state, state.Revision, StopDiagnosticComplete, "diagnosis complete", false, testNow)
	if err != nil {
		t.Fatal(err)
	}
	events := cloneEvents(settled.Events)
	events = append(events, Event{EventID: "closure-1:3", ClosureID: "closure-1", Sequence: 3, Type: EventRoundStarted, OccurredAt: testNow, Data: json.RawMessage(`{"round":1}`)})
	if _, err := Fold(events); err == nil {
		t.Fatal("event projection accepted an event after settlement")
	}
}
