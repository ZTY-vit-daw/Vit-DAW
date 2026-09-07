package audioclosure

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"vit-daw-agent/internal/taskstate"
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

func TestCanonicalTaskStateOwnsClosureTerminalOutcome(t *testing.T) {
	driver := Driver{}
	state, err := Start(StartRequest{
		ClosureID: "closure-canonical", ConversationID: "conversation-canonical", TaskID: "task-canonical", GoalID: "goal-canonical", RunID: "run-canonical",
		ContractID: "contract-canonical", TaskState: taskstate.StateObservationInProgress, TaskStateRevision: 1,
		ProjectUUID: "project-canonical", ProjectRevision: "revision-1", OriginalIntent: "inspect and improve the project",
		Mode: ModeTreatment, Scope: Scope{Kind: "project", ID: "project-canonical"}, Policy: Policy{MaxClosureRounds: 1}, Now: testNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	state = admitTestRound(t, driver, state, 1)
	state, err = driver.CompleteRound(state, state.Revision, testNow.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if state.Terminal() {
		t.Fatalf("closure budget incorrectly settled the canonical task: %+v", state.Settlement)
	}
	state, changed, err := driver.ProjectTaskState(state, state.Revision, "contract-canonical", taskstate.StateNoCandidateFound, 2, testNow.Add(3*time.Minute))
	if err != nil || !changed {
		t.Fatalf("canonical task projection failed: changed=%v err=%v", changed, err)
	}
	state, err = driver.Settle(state, state.Revision, StopNoCandidateFound, "bounded search found no candidate", false, testNow.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !state.Terminal() || state.Settlement.Reason != StopNoCandidateFound || state.TaskState != taskstate.StateNoCandidateFound {
		t.Fatalf("closure terminal projection diverged from canonical task: %+v", state)
	}
}

func TestProjectRevisionRevalidationInvalidatesClosureEvidence(t *testing.T) {
	driver := Driver{}
	state := admitTestRound(t, driver, newTestClosure(t), 1)
	key := ObservationKey{ProjectUUID: "project-1", ProjectRevision: "revision-1", Scope: Scope{Kind: "track", ID: "track-vocal"}, TargetRef: "track-vocal", ViewIDs: []string{"track.time_dynamics"}, ObservationMode: "ccb"}
	result, err := driver.RecordObservation(state, state.Revision, key, "observation-revision-1", testNow.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	state, changed, err := driver.RevalidateProjectRevision(result.State, result.State.Revision, "revision-2", testNow.Add(3*time.Minute))
	if err != nil || !changed {
		t.Fatalf("project revision revalidation failed: changed=%v err=%v", changed, err)
	}
	if state.ProjectRevision != "revision-2" || len(state.Observations) != 0 || len(state.ObservationOrder) != 0 || state.RoundInProgress {
		t.Fatalf("stale closure evidence survived revision change: %+v", state)
	}
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

func TestClosureRoundBudgetExtensionPersistsAcrossEventReplay(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	state = admitTestRound(t, driver, state, 1)
	state, err := driver.CompleteRound(state, state.Revision, testNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	oldRevision := state.Revision
	state, err = driver.ExtendClosureRounds(state, state.Revision, state.Policy.MaxClosureRounds+1, testNow.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != oldRevision+1 || state.Policy.MaxClosureRounds != 7 {
		t.Fatalf("extension did not advance durable revision/policy: %+v", state)
	}
	restored, err := Fold(state.Events)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Revision != state.Revision || restored.Policy.MaxClosureRounds != 7 {
		t.Fatalf("event replay lost closure policy extension: %+v", restored)
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
		outcome, err := driver.RecordObservationWithProvenance(state, state.Revision, ObservationKey{ProjectUUID: "project-1", ProjectRevision: "revision-1", Scope: Scope{Kind: "track", ID: "track-vocal"}, ViewIDs: []string{"view"}, TimeWindow: string(rune('a' + i))}, "observation", ProvenanceModel, testNow)
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
	fifth, err := driver.RecordObservationWithProvenance(state, state.Revision, ObservationKey{ProjectUUID: "project-1", ProjectRevision: "revision-1", Scope: Scope{Kind: "track", ID: "track-vocal"}, ViewIDs: []string{"view"}, TimeWindow: "ceiling"}, "observation-ceiling", ProvenanceModel, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !fifth.State.Terminal() || fifth.State.Settlement.Reason != StopEvidenceCeilingReached || len(fifth.State.Observations) != DefaultPolicy().MaxUniqueObservations {
		t.Fatalf("expected bounded evidence settlement, got %+v", fifth.State)
	}
}

// newDualBudgetTestClosure starts a project-scoped closure with generous
// round/no-progress budgets so a test can fill both sides of the dual
// evidence budget inside a single admitted round without tripping unrelated
// lifecycle stops.
func newDualBudgetTestClosure(t *testing.T) State {
	t.Helper()
	state, err := Start(StartRequest{
		ClosureID: "closure-dual-budget", ConversationID: "conversation-dual-budget", GoalID: "goal-dual", RunID: "run-dual",
		ProjectUUID: "project-1", ProjectRevision: "revision-1", OriginalIntent: "make the mix clearer",
		Mode: ModeTreatment, Scope: Scope{Kind: "project", ID: "project-1"},
		Policy: Policy{MaxClosureRounds: 24, MaxNoProgressRounds: 24}, Now: testNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func dualBudgetObservationKey(timeWindow string) ObservationKey {
	return ObservationKey{ProjectUUID: "project-1", ProjectRevision: "revision-1", Scope: Scope{Kind: "project", ID: "project-1"}, TargetRef: "project-1", ViewIDs: []string{"view"}, TimeWindow: timeWindow}
}

// TestDutyObservationsDoNotConsumeModelEvidenceCeiling is the dual-budget
// core: duty bookings (the coverage pass's deterministic per-track evidence)
// never consume the model-side ceiling, and the fifth model observation
// still settles evidence_ceiling_reached with the original semantics.
func TestDutyObservationsDoNotConsumeModelEvidenceCeiling(t *testing.T) {
	driver := Driver{}
	state := newDualBudgetTestClosure(t)
	state = admitTestRound(t, driver, state, 1)
	for i := 0; i < 6; i++ {
		outcome, err := driver.RecordObservationWithProvenance(state, state.Revision, dualBudgetObservationKey(fmt.Sprintf("duty-%d", i)), fmt.Sprintf("observation-duty-%d", i), ProvenanceDuty, testNow)
		if err != nil || !outcome.Accepted {
			t.Fatalf("duty observation %d must not consume the model ceiling: accepted=%v err=%v", i, outcome.Accepted, err)
		}
		state = outcome.State
	}
	if state.ModelObservationCount() != 0 || len(state.Observations) != 6 {
		t.Fatalf("duty bookings must land on the duty side only: model=%d total=%d", state.ModelObservationCount(), len(state.Observations))
	}
	for i := 0; i < DefaultPolicy().MaxUniqueObservations; i++ {
		outcome, err := driver.RecordObservation(state, state.Revision, dualBudgetObservationKey(fmt.Sprintf("model-%d", i)), fmt.Sprintf("observation-model-%d", i), testNow)
		if err != nil || !outcome.Accepted || outcome.State.Terminal() {
			t.Fatalf("model observation %d must be unaffected by duty volume: accepted=%v terminal=%v err=%v", i, outcome.Accepted, outcome.State.Terminal(), err)
		}
		state = outcome.State
	}
	if state.ModelObservationCount() != DefaultPolicy().MaxUniqueObservations || len(state.Observations) != 10 {
		t.Fatalf("dual-budget ledger diverged: model=%d total=%d", state.ModelObservationCount(), len(state.Observations))
	}
	fifth, err := driver.RecordObservation(state, state.Revision, dualBudgetObservationKey("ceiling"), "observation-ceiling", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !fifth.State.Terminal() || fifth.State.Settlement.Reason != StopEvidenceCeilingReached || fifth.State.ModelObservationCount() != DefaultPolicy().MaxUniqueObservations {
		t.Fatalf("expected bounded evidence settlement on the model side, got %+v", fifth.State)
	}
	if fifth.State.Settlement.BudgetSide != ProvenanceModel {
		t.Fatalf("model ceiling settlement must record the model budget side, got %q", fifth.State.Settlement.BudgetSide)
	}
}

// TestDutyObservationBudgetIsNamedAndBounded pins the duty-side contract: the
// named default equals the coverage-pass request bound, bookings within it
// are accepted, and a duty booking beyond it is rejected without settling —
// a completed structural obligation is not an evidence ceiling — while the
// model side stays untouched by duty saturation.
func TestDutyObservationBudgetIsNamedAndBounded(t *testing.T) {
	driver := Driver{}
	state := newDualBudgetTestClosure(t)
	if state.Policy.DutyObservationLimit() != DefaultPolicy().MaxDutyObservations || DefaultPolicy().MaxDutyObservations != 8 {
		t.Fatalf("duty budget default must equal the coverage-pass bound: limit=%d default=%d", state.Policy.DutyObservationLimit(), DefaultPolicy().MaxDutyObservations)
	}
	state = admitTestRound(t, driver, state, 1)
	for i := 0; i < state.Policy.DutyObservationLimit(); i++ {
		outcome, err := driver.RecordObservationWithProvenance(state, state.Revision, dualBudgetObservationKey(fmt.Sprintf("duty-%d", i)), fmt.Sprintf("observation-duty-%d", i), ProvenanceDuty, testNow)
		if err != nil || !outcome.Accepted {
			t.Fatalf("duty observation %d within its named budget: accepted=%v err=%v", i, outcome.Accepted, err)
		}
		state = outcome.State
	}
	over, err := driver.RecordObservationWithProvenance(state, state.Revision, dualBudgetObservationKey("duty-over"), "observation-duty-over", ProvenanceDuty, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if over.Accepted || over.Duplicate || over.State.Terminal() || over.State.Revision != state.Revision || len(over.State.Observations) != state.Policy.DutyObservationLimit() {
		t.Fatalf("duty beyond its named budget must be rejected without settling: %+v", over)
	}
	model, err := driver.RecordObservation(state, state.Revision, dualBudgetObservationKey("model-after-duty"), "observation-model", testNow)
	if err != nil || !model.Accepted || model.State.Terminal() {
		t.Fatalf("model booking must be untouched by duty saturation: accepted=%v terminal=%v err=%v", model.Accepted, model.State.Terminal(), err)
	}
}

// TestGlobalObservationTotalRemainsHard pins the defense-in-depth bound: once
// both sides sit exactly at their named limits, any further booking settles
// evidence_ceiling_reached at the hard total.
func TestGlobalObservationTotalRemainsHard(t *testing.T) {
	driver := Driver{}
	state := newDualBudgetTestClosure(t)
	state = admitTestRound(t, driver, state, 1)
	for i := 0; i < state.Policy.MaxUniqueObservations; i++ {
		outcome, err := driver.RecordObservation(state, state.Revision, dualBudgetObservationKey(fmt.Sprintf("model-%d", i)), fmt.Sprintf("observation-model-%d", i), testNow)
		if err != nil || !outcome.Accepted {
			t.Fatalf("model observation %d: accepted=%v err=%v", i, outcome.Accepted, err)
		}
		state = outcome.State
	}
	for i := 0; i < state.Policy.DutyObservationLimit(); i++ {
		outcome, err := driver.RecordObservationWithProvenance(state, state.Revision, dualBudgetObservationKey(fmt.Sprintf("duty-%d", i)), fmt.Sprintf("observation-duty-%d", i), ProvenanceDuty, testNow)
		if err != nil || !outcome.Accepted {
			t.Fatalf("duty observation %d: accepted=%v err=%v", i, outcome.Accepted, err)
		}
		state = outcome.State
	}
	if len(state.Observations) != state.Policy.MaxTotalObservations() {
		t.Fatalf("dual budget did not fill to its hard total: total=%d want=%d", len(state.Observations), state.Policy.MaxTotalObservations())
	}
	over, err := driver.RecordObservationWithProvenance(state, state.Revision, dualBudgetObservationKey("over-total"), "observation-over", ProvenanceDuty, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !over.State.Terminal() || over.State.Settlement.Reason != StopEvidenceCeilingReached || len(over.State.Observations) != state.Policy.MaxTotalObservations() {
		t.Fatalf("hard total must settle at the evidence ceiling: %+v", over.State)
	}
}

// TestObservationProvenanceJSONDefaultsLegacyRecordsToModel pins the
// persistence contract: records written before the dual budget carry no
// provenance and decode as model, while an explicit duty value round-trips.
func TestObservationProvenanceJSONDefaultsLegacyRecordsToModel(t *testing.T) {
	raw, err := json.Marshal(ObservationRecord{Fingerprint: "fp", Round: 1})
	if err != nil {
		t.Fatal(err)
	}
	var legacy ObservationRecord
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Provenance != ProvenanceModel {
		t.Fatalf("legacy record must decode as model provenance, got %q", legacy.Provenance)
	}
	raw, err = json.Marshal(ObservationRecord{Fingerprint: "fp", Round: 1, Provenance: ProvenanceDuty})
	if err != nil {
		t.Fatal(err)
	}
	var duty ObservationRecord
	if err := json.Unmarshal(raw, &duty); err != nil {
		t.Fatal(err)
	}
	if duty.Provenance != ProvenanceDuty {
		t.Fatalf("explicit duty provenance must round-trip, got %q", duty.Provenance)
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

func TestFS6AllowsBoundedTerminalWhenContinuationBudgetIsExhausted(t *testing.T) {
	if err := EvaluatePhaseGuard(PhaseFS6TargetConfirmed, PhaseFS9Terminal,
		PhaseGuardInput{TerminalStopReason: string(StopNoCandidateFound)}); err != nil {
		t.Fatal(err)
	}
}

func TestFS8VerificationAllowsObservationAndEvaluationButNoSecondAction(t *testing.T) {
	if !AllowsDecisionStatus(PhaseFS8ExperimentVerification, "needs_observation") {
		t.Fatal("FS8 must admit a fresh read-only observation")
	}
	// The experiment evaluation report (experiment_materiality,
	// experiment_target_response, experiment_round_decision incl.
	// user_judgment_pending) travels on the needs_experiment decision shape;
	// FS8 must admit it or the verification phase can observe but never
	// report (2026-08-25 D1 smoke regression).
	for _, status := range []string{"needs_experiment", "improvement_proposal"} {
		if !AllowsDecisionStatus(PhaseFS8ExperimentVerification, status) {
			t.Fatalf("FS8 rejected the evaluation decision status %q", status)
		}
	}
	if AllowsDecisionStatus(PhaseFS8ExperimentVerification, "needs_action") {
		t.Fatal("FS8 admitted a second action")
	}
	if !AllowsDecisionStatus(PhaseFS8ExperimentVerification, "blocked") || !AllowsDecisionStatus(PhaseFS8ExperimentVerification, "satisfied") {
		t.Fatal("FS8 must retain terminal settlement decisions")
	}
}

func TestFS7ToFS8RequiresValidatedAdmission(t *testing.T) {
	if err := EvaluatePhaseGuard(PhaseFS7ImprovementProposal, PhaseFS8ExperimentVerification, PhaseGuardInput{}); err == nil {
		t.Fatal("FS7 advanced to FS8 without a validated admission")
	}
	if err := EvaluatePhaseGuard(PhaseFS7ImprovementProposal, PhaseFS8ExperimentVerification, PhaseGuardInput{AdmissionValid: true}); err != nil {
		t.Fatalf("validated admission did not enter FS8: %v", err)
	}
}

func TestRecordGovernedMutationBooksAppliedRevisionUnderActiveCapability(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	link := CapabilityLink{SessionID: "session-1", CapabilityID: "agent.effect.eq_control.v0", ActionID: "action-1", ExpectedProjectRevision: "revision-1"}
	state, started, err := driver.BeginCapability(state, state.Revision, link, testNow)
	if err != nil || !started {
		t.Fatalf("begin capability: started=%v err=%v", started, err)
	}
	next, err := driver.RecordGovernedMutation(state, state.Revision, "revision-3", testNow)
	if err != nil {
		t.Fatalf("governed mutation booking failed: %v", err)
	}
	if next.ProjectRevision != "revision-3" {
		t.Fatalf("tracked revision was not booked: %q", next.ProjectRevision)
	}
	if next.ActiveCapability == nil {
		t.Fatal("governed mutation booking must keep the capability active")
	}
	if _, _, err := driver.RevalidateProjectRevision(next, next.Revision, "revision-4", testNow); err == nil {
		t.Fatal("external revalidation must still be refused while the capability is active")
	}
	unchanged, err := driver.RecordGovernedMutation(next, next.Revision, "revision-3", testNow)
	if err != nil || unchanged.Revision != next.Revision {
		t.Fatalf("idempotent re-booking failed: state=%+v err=%v", unchanged, err)
	}
	if !next.SupersededProjectRevisions["revision-1"] {
		t.Fatalf("pre-mutation revision was not marked superseded: %+v", next.SupersededProjectRevisions)
	}
	round := newTestClosure(t)
	round, _, err = driver.BeginCapability(round, round.Revision, link, testNow)
	if err != nil {
		t.Fatalf("capability begin for replay flow: %v", err)
	}
	round, err = driver.RecordGovernedMutation(round, round.Revision, "revision-3", testNow)
	if err != nil {
		t.Fatalf("booking for replay flow: %v", err)
	}
	round, _, err = driver.SettleCapability(round, round.Revision, CapabilitySettlement{SessionID: link.SessionID, ActionID: link.ActionID, Status: "completed"}, testNow)
	if err != nil {
		t.Fatalf("capability settle for replay flow: %v", err)
	}
	round, admitted, err := driver.AdmitRound(round, round.Revision, testNow)
	if err != nil || !admitted || !round.RoundInProgress {
		t.Fatalf("round admission after capability settlement: admitted=%v err=%v", admitted, err)
	}
	skipped, err := driver.RecordObservation(round, round.Revision, ObservationKey{ProjectUUID: "project-1", ProjectRevision: "revision-1", Scope: round.Scope, TargetRef: "track-vocal", ViewIDs: []string{"mix.multitrack_relationship"}}, "obs-superseded", testNow)
	if err != nil || skipped.State.Terminal() || skipped.State.Revision != round.Revision {
		t.Fatalf("superseded replay must be skipped: state=%+v err=%v", skipped.State, err)
	}
	foreign, err := driver.RecordObservation(round, round.Revision, ObservationKey{ProjectUUID: "project-1", ProjectRevision: "revision-9", Scope: round.Scope, TargetRef: "track-vocal", ViewIDs: []string{"mix.multitrack_relationship"}}, "obs-foreign", testNow)
	if err != nil || !foreign.State.Terminal() || foreign.State.Settlement.Reason != StopProjectRevisionStale {
		t.Fatalf("foreign revision drift must still settle stale: state=%+v err=%v", foreign.State, err)
	}
	if _, err := driver.RecordGovernedMutation(next, next.Revision+1, "revision-5", testNow); err == nil {
		t.Fatal("stale expected revision must be refused")
	}
}

func TestRecordGovernedMutationWithoutCapabilityAndRevisionRules(t *testing.T) {
	driver := Driver{}
	state := newTestClosure(t)
	// The D1 execution path does not always manifest a capability on the
	// closure; the receipted applied revision is authoritative either way.
	booked, err := driver.RecordGovernedMutation(state, state.Revision, "revision-5", testNow)
	if err != nil || booked.ProjectRevision != "revision-5" || !booked.SupersededProjectRevisions["revision-1"] {
		t.Fatalf("capability-less booking failed: state=%+v err=%v", booked, err)
	}
	if _, err := driver.RecordGovernedMutation(state, state.Revision, "  ", testNow); err == nil {
		t.Fatal("blank applied revision must be refused")
	}
	// A revalidation onto a superseded revision is the shadow lagging behind
	// the booking: skipped, never a downgrade that wipes evidence.
	lagged, changed, err := driver.RevalidateProjectRevision(booked, booked.Revision, "revision-1", testNow)
	if err != nil || changed || lagged.ProjectRevision != "revision-5" || len(lagged.ObservationOrder) != len(booked.ObservationOrder) {
		t.Fatalf("superseded revalidation must be skipped: changed=%v state=%+v err=%v", changed, lagged, err)
	}
	// Genuinely external revisions still revalidate through the normal path.
	external, changed, err := driver.RevalidateProjectRevision(booked, booked.Revision, "revision-7", testNow)
	if err != nil || !changed || external.ProjectRevision != "revision-7" {
		t.Fatalf("external revalidation must proceed: changed=%v state=%+v err=%v", changed, external, err)
	}
}
