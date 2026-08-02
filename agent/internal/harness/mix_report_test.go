package harness

import (
	"context"
	"testing"
	"time"

	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/orchestration"
)

func TestRequestMixReportReadsProjectDecisionLedgerWithoutMutation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	cut := orchestration.ProjectCut{ProjectUUID: "project-report", ProjectEpoch: "epoch", Consistency: "strong", Hash: "cut-report"}
	session := orchestration.PlanningSession{
		ID: "b2-report", ProjectUUID: cut.ProjectUUID, EngineOwner: orchestration.EngineV1,
		Status: orchestration.StatusCompleted, Goal: "establish hierarchy",
		Invocation: orchestration.CapabilityInvocation{CapabilityID: "static_mix.static_balance.v0", CapabilityVer: "v0"},
		FrozenPlan: &orchestration.FrozenPlan{
			Proposal:   orchestration.Proposal{ID: "proposal", Revision: 1, CapabilityID: "static_mix.static_balance.v0"},
			ActionSet:  orchestration.ActionSet{ID: "actions", Actions: []orchestration.Action{{ID: "a", TargetRef: "track-vocal"}}},
			ProjectCut: cut, PreviousObservationID: "obs-before",
		},
		Execution: &orchestration.ExecutionRecord{
			Receipts:           []orchestration.ActionReceipt{{ActionID: "a", Status: "ok"}},
			VerificationResult: &orchestration.VerificationResult{Status: "pass", EvidenceRefs: []string{"mix.observe:obs-after"}},
		},
		CreatedAt: time.Unix(1_700_000_000, 0).UTC(), UpdatedAt: time.Unix(1_700_000_001, 0).UTC(),
	}
	if _, err := mixboard.NewStore(root).RecordCapabilitySession(session); err != nil {
		t.Fatal(err)
	}

	var h *Harness
	out, err := h.requestMixReport(context.Background(), map[string]any{
		"project_uuid": "project-report",
		"mix_intent":   map[string]any{"style": "neutral"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["schema_version"] != mixboard.MixReportSchemaVersion || out["export_readiness"] != "not_assessed" {
		t.Fatalf("report = %#v", out)
	}
	if out["mixboard_decision_board_ref"] != "mixboard-project:project-report" {
		t.Fatalf("board ref = %#v", out["mixboard_decision_board_ref"])
	}
	if rows := mapRowsFromAny(out["decision_timeline"]); len(rows) != 1 {
		t.Fatalf("decision timeline = %#v", out["decision_timeline"])
	}
}
