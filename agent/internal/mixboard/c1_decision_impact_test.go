package mixboard

import (
	"testing"
	"time"

	"vit-daw-agent/internal/orchestration"
)

func TestC1StaticEQInvalidatesB2AndB4ButNotB3(t *testing.T) {
	base := time.Unix(100, 0).UTC()
	b2 := terminalMixDecisionSession("b2", "static_mix.static_balance.v0", "cut-b2", base)
	b3 := terminalMixDecisionSession("b3", "static_mix.pan_layout.v0", "cut-b3", base.Add(time.Minute))
	b4 := terminalMixDecisionSession("b4", "static_mix.low_end_relation.v0", "cut-b4", base.Add(2*time.Minute))
	c1 := terminalMixDecisionSession("c1", "fine_mix.frequency_cleanup.v1", "cut-c1", base.Add(3*time.Minute))
	store := NewStore(t.TempDir())
	for _, session := range []orchestration.PlanningSession{b2, b3, b4, c1} {
		if _, err := store.RecordCapabilitySession(session); err != nil {
			t.Fatal(err)
		}
	}
	board, err := store.ReadProjectDecisionBoard("project-1")
	if err != nil {
		t.Fatal(err)
	}
	byCapability := decisionViewsByCapability(board.Decisions)
	if byCapability["static_mix.static_balance.v0"].CurrentStatus != DecisionNeedsReview {
		t.Fatalf("B2 was not invalidated: %+v", byCapability)
	}
	if byCapability["static_mix.low_end_relation.v0"].CurrentStatus != DecisionNeedsReview {
		t.Fatalf("B4 was not invalidated: %+v", byCapability)
	}
	if byCapability["static_mix.pan_layout.v0"].CurrentStatus != DecisionVerified {
		t.Fatalf("unrelated B3 changed: %+v", byCapability)
	}
	if impact := byCapability["fine_mix.frequency_cleanup.v1"].Impact; len(impact.Writes) == 0 || !impact.ProjectWide {
		t.Fatalf("C1 impact missing: %+v", impact)
	}
}
