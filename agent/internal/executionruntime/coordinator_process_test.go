package executionruntime

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/orchestration"
)

const crashHelperExitCode = 73

func TestCoordinatorRecoversPersistedProcessCrashPointsWithoutMutationRetry(t *testing.T) {
	tests := []struct {
		name          string
		wantCompleted bool
	}{
		{name: "prepared_before_apply", wantCompleted: false},
		{name: "apply_after_before_receipt", wantCompleted: false},
		{name: "partial_batch", wantCompleted: false},
		{name: "receipts_after_before_verify", wantCompleted: true},
		{name: "verify_after_before_finalize", wantCompleted: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			storePath := filepath.Join(root, "orchestration.json")
			projectPath := filepath.Join(root, "project_state.json")
			setupCrashFixture(t, storePath)

			command := exec.Command(os.Args[0], "-test.run=^TestCoordinatorCrashProcessHelper$")
			command.Env = append(os.Environ(),
				"VIT_EXECUTION_CRASH_HELPER=1",
				"VIT_EXECUTION_CRASH_SCENARIO="+test.name,
				"VIT_EXECUTION_CRASH_STORE="+storePath,
				"VIT_EXECUTION_CRASH_PROJECT="+projectPath,
			)
			err := command.Run()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() != crashHelperExitCode {
				t.Fatalf("helper did not terminate at crash point: err=%v", err)
			}

			store, err := orchestration.NewFileStore(storePath)
			if err != nil {
				t.Fatal(err)
			}
			session, ok := store.Load("s-crash")
			if !ok || session.Execution == nil || session.FrozenPlan == nil {
				t.Fatalf("durable execution did not survive process death: %#v", session)
			}
			port := &fileProjectReconcile{Path: projectPath}
			recovered, recoverErr := New(store).Reconcile(context.Background(), session.ID, session.FrozenPlan.ActionSet, port, fakeVerifier{"pass"})
			if len(port.MutationCalls) != 0 {
				t.Fatalf("recovery retried mutation: %#v", port.MutationCalls)
			}
			if test.wantCompleted {
				if recoverErr != nil || recovered.Status != orchestration.StatusCompleted {
					t.Fatalf("recoverErr=%v recovered=%#v", recoverErr, recovered)
				}
			} else {
				if recoverErr == nil || recovered.Status != orchestration.StatusFailed || recovered.Execution.Status != "reconcile_incomplete" {
					t.Fatalf("incomplete real state was not persisted honestly: err=%v recovered=%#v", recoverErr, recovered)
				}
				if len(recovered.Execution.Receipts) != len(session.FrozenPlan.ActionSet.Actions) {
					t.Fatalf("reconciled receipt coverage was not persisted: %#v", recovered.Execution.Receipts)
				}
			}
		})
	}
}

func TestCoordinatorCrashProcessHelper(t *testing.T) {
	if os.Getenv("VIT_EXECUTION_CRASH_HELPER") != "1" {
		return
	}
	store, err := orchestration.NewFileStore(os.Getenv("VIT_EXECUTION_CRASH_STORE"))
	if err != nil {
		t.Fatal(err)
	}
	session, ok := store.Load("s-crash")
	if !ok || session.FrozenPlan == nil {
		t.Fatal("crash fixture missing")
	}
	actions := session.FrozenPlan.ActionSet.Actions
	scenario := os.Getenv("VIT_EXECUTION_CRASH_SCENARIO")
	applied := map[string]bool{}
	session.Status = orchestration.StatusExecuting
	session.Authorization.Consumed = true
	session.Execution = &orchestration.ExecutionRecord{
		ID: "execution-crash", SessionID: session.ID, ActionSetHash: session.FrozenPlan.ActionSet.Hash,
		ProjectCut: session.FrozenPlan.ProjectCut, IdempotencyKey: "exec:crash", Status: "prepared",
	}
	switch scenario {
	case "prepared_before_apply":
	case "apply_after_before_receipt":
		applied[actions[0].ID] = true
	case "partial_batch":
		applied[actions[0].ID] = true
		session.Execution.Receipts = append(session.Execution.Receipts, orchestration.ActionReceipt{ActionID: actions[0].ID, Status: "applied", EffectivelyOnce: true})
	case "receipts_after_before_verify":
		for _, action := range actions {
			applied[action.ID] = true
			session.Execution.Receipts = append(session.Execution.Receipts, orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true})
		}
	case "verify_after_before_finalize":
		for _, action := range actions {
			applied[action.ID] = true
			session.Execution.Receipts = append(session.Execution.Receipts, orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true})
		}
		session.Status = orchestration.StatusVerifying
	default:
		t.Fatalf("unknown crash scenario %q", scenario)
	}
	data, _ := json.Marshal(applied)
	if err := os.WriteFile(os.Getenv("VIT_EXECUTION_CRASH_PROJECT"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	expected := session.Revision
	session.Revision++
	if err := store.Save(session, expected); err != nil {
		t.Fatal(err)
	}
	os.Exit(crashHelperExitCode)
}

type fileProjectReconcile struct {
	Path          string
	MutationCalls []string
}

func (p *fileProjectReconcile) Reconcile(_ context.Context, action orchestration.Action, _ string, _ orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return orchestration.ActionReceipt{}, err
	}
	applied := map[string]bool{}
	if err := json.Unmarshal(data, &applied); err != nil {
		return orchestration.ActionReceipt{}, err
	}
	status := "not_applied"
	if applied[action.ID] {
		status = "applied"
	}
	return orchestration.ActionReceipt{ActionID: action.ID, Status: status, EffectivelyOnce: applied[action.ID]}, nil
}

func setupCrashFixture(t *testing.T, storePath string) {
	t.Helper()
	store, err := orchestration.NewFileStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := orchestration.NewSession("s-crash", "p1", "crash recovery", orchestration.EngineV1, orchestration.CapabilityInvocation{CapabilityID: "static_mix.static_balance.v0"})
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "p1", ProjectEpoch: "e1", BaseProjectRevision: "7", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	actionSet := orchestration.ActionSet{ID: "as-crash", CapabilityID: session.Invocation.CapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{
		{ID: "a1", Command: "track_gain_adjust", TargetRef: "t1", BeforeFingerprint: "t1:0", Args: map[string]any{"target_db": -1.0}},
		{ID: "a2", Command: "track_gain_adjust", TargetRef: "t2", BeforeFingerprint: "t2:0", Args: map[string]any{"target_db": -2.0}},
	}}
	actionSet.Hash = actionSet.ComputeHash()
	proposal := orchestration.Proposal{ID: "prop-crash", Revision: 1, CapabilityID: actionSet.CapabilityID, ProjectCutHash: cut.Hash, ActionSetHash: actionSet.Hash}
	session, err = session.SetFrozenPlan(orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, PreviousObservationID: "obs-before"})
	if err != nil {
		t.Fatal(err)
	}
	decision := orchestration.ApprovalDecision{SchemaVersion: orchestration.ApprovalDecisionSchema, Kind: orchestration.ApprovalApprove, ProposalID: proposal.ID, ProposalRevision: proposal.Revision, ActionSetHash: actionSet.Hash, ProjectCutHash: cut.Hash, SourceTurnID: "turn"}
	session, err = session.Authorize(orchestration.Authorization{ProposalID: proposal.ID, ProposalRevision: proposal.Revision, ActionSetHash: actionSet.Hash, ProjectCutHash: cut.Hash, SourceTurnID: "turn", Sequence: 1, Decision: &decision})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(session); err != nil {
		t.Fatal(err)
	}
}
