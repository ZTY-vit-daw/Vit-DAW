package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
)

// D2-2-S3h4: a settle report replayed on a freshly opened multi-round round
// that has not acted yet passes the settle gates (they only cover
// requires_post_action_observation and pending settlement) and dies later at
// the chat-layer booking — a refusal the scheduler-driven continuation never
// reads (20260829_215139 trace: three scheduler replays, each bounced only by
// the broad-acoustic legacy gate). The refusal must fire inside the message
// loop's own final gate, whose feedback is the only surface every next
// continuation round consumes.

// owedRoundLoopContext mirrors the forensic loop projection shape: a running
// multi-round experiment whose round 1 acted and was decided (next_round), and
// whose freshly opened round 2 carries a fresh non-post-action base
// observation, zero interventions, and no decision.
func owedRoundLoopContext(mutate func(loop map[string]any)) map[string]any {
	loop := map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
		"original_intent": "improve the vocal boxiness",
		"rounds_note":     "see experiment.rounds",
		"experiment": map[string]any{
			"schema_version": "experiment.turn.v1", "status": "running",
			"admission": map[string]any{"experiment_budget": 2},
			"rounds": []map[string]any{
				{
					"round_id": "r_1", "status": "settled", "decision": "next_round",
					"interventions": []map[string]any{{"id": "iv_1"}},
					"observations": []map[string]any{
						{"observation_id": "obs-post-1", "post_action": true, "fresh": true, "project_revision": "3"},
					},
				},
				{
					"round_id": "r_2", "status": "running",
					"observations": []map[string]any{
						{"observation_id": "obs-base-2", "post_action": false, "fresh": true, "project_revision": "4"},
					},
				},
			},
		},
	}
	if mutate != nil {
		mutate(loop)
	}
	return loop
}

func owedRoundSettleReport() messageLoopOutput {
	return messageLoopOutput{Final: true, Reply: "audition pending",
		FreeStateDecision: &FreeStateDecision{
			SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment, EvidenceStatus: "sufficient",
			Summary: "settle report replayed on the never-acted round", ImprovementProposal: validPendingSettlementProposal(),
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialityMaterial, Evaluation: "agent_evaluable", Attempt: 1, EvidenceRefs: []string{"obs-base-2"},
			},
			ExperimentTargetResponse: &experiment.TargetEvaluation{
				Response: "directional", Outcome: "human_audition_ready", EvidenceRefs: []string{"obs-base-2"},
			},
			ExperimentRoundDecision: "user_judgment_pending",
		}}
}

const owedRoundRefusalNeedle = "no round to settle"

// The core walkthrough: the settle report on the never-acted multi-round round
// is refused by the in-loop gate, and the refusal names the round's quotable
// fresh base reference (obs id @ revision) exactly like the S3h2 HTTP face.
func TestFreeStateOwedRoundRefusesSettleReportAndNamesBaseReference(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": owedRoundLoopContext(nil),
	}}}
	issue := messageLoopFreeStateOutputIssue(state, owedRoundSettleReport())
	if !strings.Contains(issue, owedRoundRefusalNeedle) {
		t.Fatalf("settle report on the never-acted round escaped the gate: %q", issue)
	}
	if !strings.Contains(issue, "obs-base-2@4") {
		t.Fatalf("owed-round refusal does not name the fresh base reference: %q", issue)
	}
	if !strings.Contains(issue, "improvement_proposal") {
		t.Fatalf("owed-round refusal does not point at the owed proposal: %q", issue)
	}
}

// Tier scope control: the sealed single-round budget keeps its boundary
// behavior untouched — the owed-round refusal must not fire there.
func TestFreeStateOwedRoundGateSparesSealedSingleRoundTier(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": owedRoundLoopContext(func(loop map[string]any) {
			experimentRow := loop["experiment"].(map[string]any)
			experimentRow["admission"] = map[string]any{"experiment_budget": 1}
		}),
	}}}
	issue := messageLoopFreeStateOutputIssue(state, owedRoundSettleReport())
	if strings.Contains(issue, owedRoundRefusalNeedle) {
		t.Fatalf("owed-round refusal fired on the sealed single-round tier: %q", issue)
	}
}

// The applied-boundary race window (requires_post_action_observation=true) is
// an acted round whose settle retry is owed: the existing observation gate
// owns that refusal and the owed-round copy must not preempt it.
func TestFreeStateOwedRoundGateSparesAppliedRaceWindow(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": owedRoundLoopContext(func(loop map[string]any) {
			loop["requires_post_action_observation"] = true
		}),
	}}}
	issue := messageLoopFreeStateOutputIssue(state, owedRoundSettleReport())
	if strings.Contains(issue, owedRoundRefusalNeedle) {
		t.Fatalf("owed-round refusal preempted the race-window settle retry: %q", issue)
	}
	if !strings.Contains(issue, "until a fresh CCB observation_request has returned in this reasoning cycle") {
		t.Fatalf("race-window settle replay did not hit the observation gate: %q", issue)
	}
}

// Budget scope control: once the experiment's intervention budget is spent the
// chat layer stops opening rounds there; the in-loop refusal mirrors that and
// stays silent.
func TestFreeStateOwedRoundGateSparesBudgetExhaustedRounds(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": owedRoundLoopContext(func(loop map[string]any) {
			experimentRow := loop["experiment"].(map[string]any)
			rounds := experimentRow["rounds"].([]map[string]any)
			rounds[1]["interventions"] = []map[string]any{{"id": "iv_2"}}
		}),
	}}}
	issue := messageLoopFreeStateOutputIssue(state, owedRoundSettleReport())
	if strings.Contains(issue, owedRoundRefusalNeedle) {
		t.Fatalf("owed-round refusal fired on a spent budget shape: %q", issue)
	}
}

// The owed work itself — a bare bounded intervention proposal without report
// fields — must never meet the owed-round refusal.
func TestFreeStateOwedRoundGateAllowsOwedProposal(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": owedRoundLoopContext(nil),
	}}}
	proposal := messageLoopOutput{Final: true, Reply: "propose the owed bounded step",
		FreeStateDecision: &FreeStateDecision{
			SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment, EvidenceStatus: "plausible",
			Summary: "owed round proposal", ImprovementProposal: &agentprotocol.ImprovementProposal{
				SchemaVersion:     agentprotocol.ImprovementProposalSchema,
				Target:            map[string]any{"kind": "track", "id": "1032"},
				EvidenceRefs:      []string{"obs-base-2"},
				ImprovementIntent: "reduce the vocal boxiness",
				Hypothesis:        "a bounded static EQ band move may reduce the boxiness",
				ExpectedEffect:    "the band relationship should be easier to compare",
				ActionDomain:      agentprotocol.ImprovementActionDomainStaticEQ,
				ActionKind:        "static_eq_band_adjust", Confidence: 0.6,
			},
		}}
	issue := messageLoopFreeStateOutputIssue(state, proposal)
	if strings.Contains(issue, owedRoundRefusalNeedle) {
		t.Fatalf("owed-round refusal fired on the owed proposal itself: %q", issue)
	}
}

// When the never-acted round carries no quotable fresh base observation, the
// refusal still fires and falls back to the unnamed fresh-base wording instead
// of quoting a stale reference.
func TestFreeStateOwedRoundRefusalFallsBackWithoutBaseReference(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": owedRoundLoopContext(func(loop map[string]any) {
			experimentRow := loop["experiment"].(map[string]any)
			rounds := experimentRow["rounds"].([]map[string]any)
			rounds[1]["observations"] = []map[string]any{}
		}),
	}}}
	issue := messageLoopFreeStateOutputIssue(state, owedRoundSettleReport())
	if !strings.Contains(issue, owedRoundRefusalNeedle) {
		t.Fatalf("settle report escaped the gate without a base observation: %q", issue)
	}
	if strings.Contains(issue, "@") && strings.Contains(issue, "citing the round's fresh base observation obs") {
		t.Fatalf("refusal quoted a base reference although none exists: %q", issue)
	}
	if !strings.Contains(issue, "fresh base") {
		t.Fatalf("refusal lost the fresh-base guidance entirely: %q", issue)
	}
}

// GLM ruling 3 scope control: while an earlier round's human judgment has not
// landed, the settle family stays the only admitted output — the owed-proposal
// refusal must not fire at that parked boundary.
func TestFreeStateOwedRoundGateSparesPendingJudgmentBoundary(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": owedRoundLoopContext(func(loop map[string]any) {
			experimentRow := loop["experiment"].(map[string]any)
			rounds := experimentRow["rounds"].([]map[string]any)
			rounds[0]["user_judgment_requested"] = true
		}),
	}}}
	if !messageLoopFreeStateJudgmentBoundary(state) {
		t.Fatal("fixture does not hold the pending judgment boundary")
	}
	issue := messageLoopFreeStateOutputIssue(state, owedRoundSettleReport())
	if strings.Contains(issue, owedRoundRefusalNeedle) {
		t.Fatalf("owed-round refusal fired at the pending judgment boundary: %q", issue)
	}
}
