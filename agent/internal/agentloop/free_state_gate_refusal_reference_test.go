package agentloop

import (
	"strings"
	"testing"
)

// D2-2-S3h2: the G1/G7 admission refusals told the model only that the gate
// failed. A model holding a stale pointer then re-quoted the same stale ref and
// burned its turns. The refusal feedback must carry the mechanical runtime
// fact of which fresh observation reference is quotable right now (obs id +
// revision) — state communication, no domain knowledge.

// TestGateRefusalFeedbackNamesFreshQuotableReference is the G7 walkthrough: a
// proposal quoting an unmatched ref fails G7 alone, and the refusal names the
// ledger's current fresh reference so the retry can quote it. DIAG3-2: the
// MILESTONE-E2E rejections failed on citation FORMAT, not on availability —
// the refusal must also carry the literal field shape the gate matches
// (improvement_proposal.evidence_refs with the obs-id@revision string).
func TestGateRefusalFeedbackNamesFreshQuotableReference(t *testing.T) {
	state := gateTestState(nil)
	out := gateTestProposal([]string{"obs-ghost"})
	issue := messageLoopFreeStateOutputIssue(state, out)
	if !strings.Contains(issue, "G7_fresh_revision_bound_refs") {
		t.Fatalf("G7 did not fail on the unmatched ref: %q", issue)
	}
	if !strings.Contains(issue, "obs-target@rev-7") {
		t.Fatalf("G7 refusal does not name the fresh quotable reference: %q", issue)
	}
	if !strings.Contains(issue, `improvement_proposal.evidence_refs`) ||
		!strings.Contains(issue, `["obs-target@rev-7"]`) {
		t.Fatalf("G7 refusal does not carry the literal citation format example: %q", issue)
	}
}

// TestGateRefusalFeedbackNamesReferenceOnProjectBindingFailure locks the G1
// half: a contract/closure binding disagreement fails G1, and the refusal
// still names the fresh reference and its revision.
func TestGateRefusalFeedbackNamesReferenceOnProjectBindingFailure(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		messageLoopMapValue(ctx["task_contract"])["project_revision"] = "rev-6"
	})
	out := gateTestProposal(nil)
	issue := messageLoopFreeStateOutputIssue(state, out)
	if !strings.Contains(issue, "G1_project_binding") {
		t.Fatalf("G1 did not fail on the binding disagreement: %q", issue)
	}
	if !strings.Contains(issue, "obs-target@rev-7") {
		t.Fatalf("G1 refusal does not name the fresh quotable reference: %q", issue)
	}
}

// TestGateRefusalFeedbackStaysLeanOutsideRevisionBoundGates is the scope
// control: a non-revision gate failure (G5 frontier) keeps the plain refusal —
// the reference enrichment must not leak into unrelated gate copy.
func TestGateRefusalFeedbackStaysLeanOutsideRevisionBoundGates(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		closure := messageLoopMapValue(ctx["minimal_audio_closure"])
		frontier := messageLoopMapValue(closure["hypothesis_frontier"])
		frontier["candidates"] = []any{}
	})
	out := gateTestProposal(nil)
	issue := messageLoopFreeStateOutputIssue(state, out)
	if !strings.Contains(issue, "G5_frontier_established") {
		t.Fatalf("G5 did not fail on the empty frontier: %q", issue)
	}
	if strings.Contains(issue, "obs-target@") {
		t.Fatalf("non-revision gate failure carried the fresh-reference enrichment: %q", issue)
	}
}
