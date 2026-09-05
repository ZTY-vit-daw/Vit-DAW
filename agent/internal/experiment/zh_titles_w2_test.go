package experiment

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/trajectory"
)

// AGENT-W2（2026-09-05 手测）：轨迹事件标题与应用后回复全部为英文——设计稿
// 要求中文摘要。且 round.started 节点状态为 running（独立 node 永不更新），
// UI 上"experiment round started"永久打转。
func TestRoundStartedEventIsTerminalStatusWithChineseTitle(t *testing.T) {
	if got := trajectory.DefaultStatus(trajectory.EventRoundStarted); got != trajectory.StatusCompleted {
		t.Fatalf("round.started marks a transition that already happened; its node status must be completed, got %s", got)
	}
	turn := newAdmittedTurnForTitleTest(t)
	events, err := turn.StartRound([]string{"mix.frequency_relationship"}, "cp_ref", "rev1", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Type == trajectory.EventRoundStarted {
			found = true
			if !strings.Contains(event.Title, "实验轮") {
				t.Fatalf("round.started title must be Chinese, got %q", event.Title)
			}
		}
	}
	if !found {
		t.Fatal("round.started event missing")
	}
}

func TestSettleClosesCurrentRound(t *testing.T) {
	turn := newAdmittedTurnForTitleTest(t)
	if _, err := turn.StartRound([]string{"mix.frequency_relationship"}, "cp_ref", "rev1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	events, err := turn.Settle(OutcomeNeedsJudgment, "等待人工判定", time.Now().UTC())
	_ = events
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	round, roundErr := turn.CurrentRound()
	if roundErr != nil {
		t.Fatal(roundErr)
	}
	if round.Status != RoundCompleted {
		t.Fatalf("settle must close the current round, got %s", round.Status)
	}
}

func TestAppliedReplyTextIsChinese(t *testing.T) {
	checked := 0
	for _, spec := range D1S1DomainSpecs() {
		if strings.TrimSpace(spec.AppliedReplyText) == "" {
			continue
		}
		checked++
		for _, ascii := range []string{"was applied", "pending"} {
			if strings.Contains(spec.AppliedReplyText, ascii) {
				t.Fatalf("domain %s applied reply must be Chinese, got %q", spec.ActionDomain, spec.AppliedReplyText)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no domain applied reply found to check")
	}
}

func newAdmittedTurnForTitleTest(t *testing.T) *Turn {
	t.Helper()
	turn, err := NewTurn(Identity{ConversationID: "conversation-w2", GoalID: "goal-w2", RunID: "run-w2"}, "bounded test experiment", testD1Admission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return &turn
}
