package orchestration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionOwnerIsFixedAndSaveUsesCAS(t *testing.T) {
	s, err := NewSession("s1", "project-1", "inspect static balance", EngineV1, CapabilityInvocation{
		CapabilityID:    "static_mix.static_balance.v0",
		CapabilityVer:   "v0",
		InteractionMode: InteractionInspect,
		ProcessingPath:  PathCapability,
		Goal:            "inspect static balance",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	if err := store.Create(s); err != nil {
		t.Fatal(err)
	}
	next, err := s.Transition(StatusWaiting)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(next, s.Revision); err != nil {
		t.Fatal(err)
	}
	next.EngineOwner = EngineLegacy
	if err := store.Save(next, next.Revision); err == nil {
		t.Fatal("expected immutable engine owner rejection")
	}
	if err := store.Save(next, s.Revision); err == nil {
		t.Fatal("expected revision conflict")
	}
}

func TestNeedsReviewIsTerminalWithoutBeingExecutionFailure(t *testing.T) {
	session, err := NewSession("review", "project-1", "verify B2", EngineV1, CapabilityInvocation{CapabilityID: "static_mix.static_balance.v0"})
	if err != nil {
		t.Fatal(err)
	}
	session.Status = StatusVerifying
	review, err := session.Transition(StatusNeedsReview)
	if err != nil {
		t.Fatal(err)
	}
	if !review.Terminal() || review.Status == StatusFailed {
		t.Fatalf("needs_review must be terminal and distinct from failure: %#v", review)
	}
	if _, err := review.Transition(StatusExecuting); err == nil {
		t.Fatal("needs-review execution must not be automatically retried")
	}
}

func TestProposalAuthorizationMustMatchCurrentRevision(t *testing.T) {
	s, err := NewSession("s1", "project-1", "plan static balance", EngineV1, CapabilityInvocation{})
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.SetProposal(Proposal{ID: "p1", Revision: 1, ProjectCutHash: "cut-1", ActionSetHash: "action-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authorize(testAuthorization("p1", 1, "old", "cut-1", "turn-1")); err == nil {
		t.Fatal("expected action set mismatch")
	}
	next, err := s.Authorize(testAuthorization("p1", 1, "action-1", "cut-1", "turn-1"))
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != StatusAuthorized || next.Authorization == nil {
		t.Fatalf("unexpected authorized session: %#v", next)
	}
	if _, err := next.ClaimAuthorization(); err != nil {
		t.Fatal(err)
	}
	if _, err := next.ClaimAuthorization(); err == nil {
		t.Fatal("expected consumed authorization rejection")
	}
}

func TestProjectCutHashIsStableAndEventualCutIsNotExecutable(t *testing.T) {
	a := ProjectCut{
		ProjectUUID:            "p",
		ProjectEpoch:           "e",
		Consistency:            "strong",
		DependencyFingerprints: []string{"b", "a"},
		ContractVersions:       []string{"vms:v1", "cap:v1"},
	}
	b := a
	b.DependencyFingerprints = []string{"a", "b"}
	if a.ComputeHash() != b.ComputeHash() {
		t.Fatal("semantic cut ordering must not change hash")
	}
	a.Hash = a.ComputeHash()
	if !a.IsExecutable() {
		t.Fatal("strong cut with identity and hash should be executable")
	}
	a.Consistency = "eventual"
	if a.IsExecutable() {
		t.Fatal("eventual cut must not be executable")
	}
}

func TestFrozenPlanPersistsExactAuthorizedInputs(t *testing.T) {
	session, err := NewSession("s-frozen", "p1", "balance", EngineV1, CapabilityInvocation{CapabilityID: "static_mix.static_balance.v0"})
	if err != nil {
		t.Fatal(err)
	}
	cut := ProjectCut{ProjectUUID: "p1", ProjectEpoch: "e1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	actionSet := ActionSet{ID: "as1", CapabilityID: "static_mix.static_balance.v0", ProjectCutHash: cut.Hash, Actions: []Action{{
		ID: "a1", Command: "track_gain_adjust", TargetRef: "t1", Args: map[string]any{"target_db": -1.0}, Compensatable: true,
	}}}
	actionSet.Hash = actionSet.ComputeHash()
	proposal := Proposal{ID: "prop1", Revision: 1, CapabilityID: actionSet.CapabilityID, ProjectCutHash: cut.Hash, ActionSetHash: actionSet.Hash}
	session, err = session.SetFrozenPlan(FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: "bundle1", PreviousObservationID: "obs-before",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	if err := store.Create(session); err != nil {
		t.Fatal(err)
	}
	loaded, _ := store.Load(session.ID)
	if loaded.FrozenPlan == nil || loaded.FrozenPlan.ActionSet.Hash != actionSet.Hash || loaded.FrozenPlan.ProjectCut.Hash != cut.Hash || loaded.FrozenPlan.PreviousObservationID != "obs-before" {
		t.Fatalf("frozen execution input was not retained: %#v", loaded.FrozenPlan)
	}
	loaded.FrozenPlan.ActionSet.Actions[0].Args["target_db"] = -9.0
	reloaded, _ := store.Load(session.ID)
	if reloaded.FrozenPlan.ActionSet.Actions[0].Args["target_db"] != -1.0 {
		t.Fatalf("store leaked mutable frozen plan alias: %#v", reloaded.FrozenPlan)
	}
}

func TestActionSetHashSurvivesTypedArgsPersistenceBoundary(t *testing.T) {
	type typedPayload struct {
		Zeta  string             `json:"zeta"`
		Alpha map[string]float64 `json:"alpha"`
	}
	actionSet := ActionSet{
		ID: "typed-args", CapabilityID: "spal.reference_eq_provider_registration.v0", ProjectCutHash: "cut-1",
		Actions: []Action{{
			ID: "register", Command: "spal.reference_eq_provider.register", TargetRef: "track:1",
			Args: map[string]any{"provider_record": typedPayload{Zeta: "record", Alpha: map[string]float64{"q": 1.2}}},
		}},
	}
	actionSet.Hash = actionSet.ComputeHash()
	if actionSet.Hash == "" {
		t.Fatal("typed action set did not produce a hash")
	}
	data, err := json.Marshal(actionSet)
	if err != nil {
		t.Fatal(err)
	}
	var restored ActionSet
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Hash != restored.ComputeHash() {
		t.Fatalf("action hash changed after durable JSON boundary: stored=%s recomputed=%s", restored.Hash, restored.ComputeHash())
	}
}

func TestDefaultFileStorePathSupportsExplicitMemoryOptOut(t *testing.T) {
	t.Setenv("VIT_ORCHESTRATION_STORE_PATH", "memory")
	if path := DefaultFileStorePath(); path != "" {
		t.Fatalf("memory opt-out returned durable path %q", path)
	}
	want := filepath.Join(t.TempDir(), "state", "orchestration.json")
	t.Setenv("VIT_ORCHESTRATION_STORE_PATH", want)
	if path := DefaultFileStorePath(); path != want {
		t.Fatalf("custom store path=%q want=%q", path, want)
	}
}

func TestDefaultFileStorePathUsesPerUserConfigDirectory(t *testing.T) {
	t.Setenv("VIT_ORCHESTRATION_STORE_PATH", "")
	root, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(root) == "" {
		t.Skipf("user config directory unavailable: %v", err)
	}
	want := filepath.Join(root, "Vit", "Agent", "orchestration_v1.json")
	if path := DefaultFileStorePath(); path != want || !filepath.IsAbs(path) {
		t.Fatalf("default store path=%q want absolute %q", path, want)
	}
}

func TestFrozenProposalCannotBeReplacedAfterAuthorization(t *testing.T) {
	session, err := NewSession("s-auth", "p1", "goal", EngineV1, CapabilityInvocation{CapabilityID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SetProposal(Proposal{ID: "p1", Revision: 1, ProjectCutHash: "cut1", ActionSetHash: "as1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.Authorize(testAuthorization("p1", 1, "as1", "cut1", "turn"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.SetProposal(Proposal{ID: "p2", Revision: 2, ProjectCutHash: "cut2", ActionSetHash: "as2"}); err == nil {
		t.Fatal("authorized frozen proposal was replaceable")
	}
}

func testAuthorization(proposalID string, revision int64, actionSetHash, projectCutHash, turnID string) Authorization {
	decision := ApprovalDecision{
		SchemaVersion: ApprovalDecisionSchema, Kind: ApprovalApprove,
		ProposalID: proposalID, ProposalRevision: revision, ActionSetHash: actionSetHash, ProjectCutHash: projectCutHash,
		SourceTurnID: turnID, Confidence: "high", Reason: "test_explicit_approval",
	}
	return Authorization{
		ProposalID: proposalID, ProposalRevision: revision, ActionSetHash: actionSetHash, ProjectCutHash: projectCutHash,
		SourceTurnID: turnID, Sequence: 1, Decision: &decision,
	}
}
