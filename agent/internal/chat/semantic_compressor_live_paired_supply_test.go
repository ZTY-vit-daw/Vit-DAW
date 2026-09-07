package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/com"
)

// R2 (run 20260907_102714) reproduced synthetically: the live planning branch
// requested paired_io, the paired mix.observe failed on the agent-side COM
// evidence workspace-root supply ("COM evidence workspace root is
// unavailable"), and the observation fell back to a source_only/partial
// projection. The execution gate must keep rejecting that supply — the fix
// belongs to the evidence supply (launch-env), never to this gate.
func TestLiveBranchMaterializationRejectsSourceOnlyFallbackFromFailedPairedSupply(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	comContext["mode"], comContext["status"] = com.ModeSourceOnly, com.StatusPartial
	plan.Evidence.COMMode, plan.Evidence.COMStatus = com.ModeSourceOnly, com.StatusPartial
	comContext["requested_mode"] = com.ModePairedIO
	comContext["fallback_reason"] = "COM evidence workspace root is unavailable"
	_, _, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor", nil)
	if err == nil || !strings.Contains(err.Error(), "paired_io / ready") || !strings.Contains(err.Error(), "actual mode=source_only status=partial") {
		t.Fatalf("source-only fallback supply from a failed paired observation was not rejected: %v", err)
	}
}

// With the supply fixed, the same live branch receives a paired ready
// projection and the identical gate path materializes the execution ticket.
func TestLiveBranchMaterializationAcceptsReadyPairedSupplyForTheSamePlan(t *testing.T) {
	plan, digest, summary, comContext := semanticCompressorExecutionFixture(t)
	comContext["requested_mode"] = com.ModePairedIO
	ticket, preview, err := materializeSemanticCompressorPlan(plan, digest, summary, comContext,
		"conversation-1", "goal-1", "run-1", "Test Compressor",
		map[string]any{"clip_id": "2001", "start_sample": 0, "end_sample": 480000})
	if err != nil {
		t.Fatalf("ready paired supply was not materialized: %v", err)
	}
	if ticket.Evidence.COMMode != com.ModePairedIO || ticket.Evidence.COMStatus != com.StatusReady || len(ticket.Controls) != 1 {
		t.Fatalf("ticket = %+v", ticket)
	}
	if ticket.ObservationContext["clip_id"] != "2001" || ticket.ObservationContext["end_sample"] != 480000 {
		t.Fatalf("observation window was not carried into the ticket: %+v", ticket.ObservationContext)
	}
	if strings.Contains(preview, ticket.Controls[0].ControlRef) {
		t.Fatal("preview leaked the control reference")
	}
}
