package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/planner"
)

// MIXLAYER-M1-5 unit coverage: the chat-side three-view enrichment folds the
// M1-2 queue node-state view (consumed read-only via BuildMixWorkflowQueueView)
// into a mix_report result and renders the F1 summary lines — the latest
// settle conclusion line (from the mixboard fold) and the queue node-state
// line — for both the executor path and the /agent/invoke path.

func m1FiveReportResult(latest map[string]any) map[string]any {
	result := map[string]any{
		"schema_version": "mix_report.v1",
		"project_uuid":   "proj-m1-5",
		"report_ref":     "mix-report:proj-m1-5:cut-x",
	}
	if latest != nil {
		result["latest_settle_conclusion"] = latest
	}
	return result
}

func TestMixReportEnrichFoldsQueueViewAndSettleLine(t *testing.T) {
	queueTestActivateProject(t, "proj-m1-5-enrich")
	if _, err := (mixboard.NewStore("")).RecordCapabilitySession(queueTestTerminalSession("sess-c1", "fine_mix.frequency_cleanup.v1", "proj-m1-5-enrich", "cut-c1", time.Now().UTC().Add(-time.Minute))); err != nil {
		t.Fatalf("record C1: %v", err)
	}
	server := &Server{activeWorkspaceUUID: "proj-m1-5-enrich"}
	if _, err := server.WriteMixWorkflowQueueNodeState(context.Background(), "E1", MixWorkflowNodeDeferred, "母带后置", nil, "conv-m1-5"); err != nil {
		t.Fatalf("write E1 deferred: %v", err)
	}

	result := server.enrichMixReportResult(context.Background(), m1FiveReportResult(map[string]any{
		"kind": mixboard.FreeStateConclusionKindExperimentSettle, "outcome": "improved",
		"summary": "人耳判定保留 B2 改动", "settled_at": "2026-10-11T12:00:00Z",
	}))

	queue, ok := result["mix_workflow_queue"].(map[string]any)
	if !ok {
		t.Fatalf("mix_workflow_queue missing: %#v", result["mix_workflow_queue"])
	}
	if queue["schema_version"] != MixWorkflowQueueSchemaVersion {
		t.Fatalf("queue schema = %#v", queue["schema_version"])
	}
	message, ok := result["message"].(string)
	if !ok || message == "" {
		t.Fatalf("message missing: %#v", result["message"])
	}
	if !strings.Contains(message, "最近 settle 结论：") {
		t.Fatalf("settle line missing:\n%s", message)
	}
	if !strings.Contains(message, "experiment_settle improved") || !strings.Contains(message, "人耳判定保留 B2 改动") {
		t.Fatalf("settle line does not carry the conclusion facts:\n%s", message)
	}
	if strings.Contains(message, "无记录") {
		t.Fatalf("a present conclusion must not render the no-record default:\n%s", message)
	}
	if !strings.Contains(message, "queue 节点状态：") || !strings.Contains(message, "23 节点") {
		t.Fatalf("queue node-state line missing:\n%s", message)
	}
	if !strings.Contains(message, "E1 母带 EQ[延期]") {
		t.Fatalf("non-default node E1 deferred missing:\n%s", message)
	}
	if !strings.Contains(message, "完成 1") {
		t.Fatalf("completed C1 missing from the state summary:\n%s", message)
	}
}

func TestMixReportEnrichNoConclusionRendersHonestDefault(t *testing.T) {
	queueTestActivateProject(t, "proj-m1-5-none")
	server := &Server{activeWorkspaceUUID: "proj-m1-5-none"}
	result := server.enrichMixReportResult(context.Background(), m1FiveReportResult(nil))
	message, ok := result["message"].(string)
	if !ok || !strings.Contains(message, "最近 settle 结论：无记录") {
		t.Fatalf("no-record settle default missing:\n%s", message)
	}
	if !strings.Contains(message, "queue 节点状态：") || !strings.Contains(message, "未开始 23") {
		t.Fatalf("all-not-started queue summary missing:\n%s", message)
	}
	if strings.Contains(message, "非缺省节点") {
		t.Fatalf("empty queue must not render non-default nodes:\n%s", message)
	}
}

func TestMixReportEnrichNilResultPassesThrough(t *testing.T) {
	server := &Server{}
	if got := server.enrichMixReportResult(context.Background(), nil); got != nil {
		t.Fatalf("nil result must pass through, got %#v", got)
	}
}

func TestMixReportToolRequestMatcher(t *testing.T) {
	for _, tool := range []string{"mix_report", "mix.report", "MIX_REPORT"} {
		if !mixReportToolRequest(planner.ToolCall{Tool: tool}) {
			t.Fatalf("tool %q must match", tool)
		}
	}
	for _, tool := range []string{"mix_read", "mix_queue_view", "", "mix_reporter"} {
		if mixReportToolRequest(planner.ToolCall{Tool: tool}) {
			t.Fatalf("tool %q must not match", tool)
		}
	}
}

func TestMixReportStateSummaryLineOrder(t *testing.T) {
	line := mixWorkflowStateSummaryLine(map[string]int{
		MixWorkflowNodeNotStarted: 20, MixWorkflowNodeCompleted: 2, MixWorkflowNodeDeferred: 1,
	})
	if !strings.HasPrefix(line, "完成 2") {
		t.Fatalf("completed must lead the summary: %q", line)
	}
	if !strings.HasSuffix(line, "未开始 20") {
		t.Fatalf("not_started must close the summary: %q", line)
	}
	if !strings.Contains(line, "延期 1") {
		t.Fatalf("deferred missing: %q", line)
	}
	if strings.Contains(line, "阻塞") {
		t.Fatalf("empty states must be skipped: %q", line)
	}
}
