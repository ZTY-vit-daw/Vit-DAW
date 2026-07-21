package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestCapabilityAuthorityReportRequiresLiveCertification(t *testing.T) {
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED", "false")
	server := &Server{orchestrationRuntime: orchestrationruntime.New()}
	report := server.capabilityAuthorityReport()
	if report.SafeToDisableLegacyCode || len(report.LegacyPending) != 0 || len(report.Blockers) != 1 || !report.LegacyCreationClosed {
		t.Fatalf("unsafe cutover was reported safe: %#v", report)
	}
}

func TestCapabilityAuthorityReportCanProveDrainGate(t *testing.T) {
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT", "all")
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED", "true")
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION", "true")
	server := &Server{orchestrationRuntime: orchestrationruntime.New()}
	report := server.capabilityAuthorityReport()
	if !report.SafeToDisableLegacyCode || len(report.Blockers) != 0 || !report.LegacyCreationClosed {
		t.Fatalf("completed drain gates were not recognized: %#v", report)
	}
}

func TestLegacyCapabilityCreationGateDoesNotPersistNewB2B3Pending(t *testing.T) {
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT", "all")
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED", "true")
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION", "true")
	server := &Server{}
	b2 := testStaticBalancePlan()
	b3 := testPanLayoutPlan()
	result := server.applyLegacyCapabilityCreationGate(agentloop.Result{
		Status:          agentruntime.StatusWaitingConfirmation,
		ExecutionMemory: agentloop.ExecutionMemory{PendingStaticBalancePlan: &b2, PendingPanLayoutPlan: &b3},
	})
	if result.Status != agentruntime.StatusFailed || result.StopReason != "legacy_capability_creation_closed" || result.ExecutionMemory.PendingStaticBalancePlan != nil || result.ExecutionMemory.PendingPanLayoutPlan != nil {
		t.Fatalf("legacy creation gate leaked pending authority: %#v", result)
	}
}

func TestCapabilityRuntimeStatusCommandReturnsDrainInventory(t *testing.T) {
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT", "b2")
	server := &Server{orchestrationRuntime: orchestrationruntime.New()}
	response, handled := server.handleDevChatCommand(context.Background(), "chat", ChatRequest{Message: "/capability-runtime/status"})
	if !handled || response.Workflow != "capability_runtime_v1" || response.WorkflowData["authority_report"] == nil {
		t.Fatalf("status command missing authority report: handled=%v response=%#v", handled, response)
	}
}

func TestAuthorityReportNeverTreatsUnreadableStoreAsDrained(t *testing.T) {
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT", "all")
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED", "true")
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION", "true")
	path := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := orchestration.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{orchestrationRuntime: orchestrationruntime.NewWithStore(store)}
	report := server.capabilityAuthorityReport()
	if report.StoreError == "" || report.SafeToDisableLegacyCode {
		t.Fatalf("unreadable Store was treated as drained: %#v", report)
	}
}
