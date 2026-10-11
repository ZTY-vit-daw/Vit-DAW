package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/planner"
)

// MIXLAYER-M1-5 F1 three-view alignment: the mix.report projection now
// carries, beside the decision-ledger account, the free-state settle
// conclusions (folded into the report by mixboard.BuildMixReport — goal 1)
// and the mix workflow queue node states (folded here — goal 2). The queue
// view lives on the chat Server (it folds server-side annotation/route
// state), so the enrichment is a chat-side read-only composition over the
// harness mix_report result: the base tool result is built first, then the
// queue projection (M1-2, consumed read-only via BuildMixWorkflowQueueView)
// and the F1 summary lines are merged in. Both transport faces are covered:
// the agent-loop executor (LLM tool calls) and /agent/invoke (HTTP).
//
// Red line (card MIXLAYER-M1-5): no refactoring of the M1-1/M1-2 merged
// files, no canary/routing seam changes (M1-3 domain), no new catalog
// registration (mix_report is already registered; the enrichment adds no
// tool arguments).

// mixReportToolRequest matches the mix.report catalog tool on the executor
// path (LLM tool calls).
func mixReportToolRequest(call planner.ToolCall) bool {
	switch strings.ToLower(strings.TrimSpace(call.Tool)) {
	case "mix_report", "mix.report":
		return true
	default:
		return false
	}
}

// isMixReportInvokeRequest matches the mix.report tool on the /agent/invoke
// path. The alias form arrives lowercased by handleInvoke already; keep both
// spellings for safety.
func isMixReportInvokeRequest(req harness.InvokeRequest) bool {
	switch strings.ToLower(strings.TrimSpace(req.Tool)) {
	case "mix_report", "mix.report":
		return true
	default:
		return false
	}
}

// invokeMixReportTool serves the executor path: base harness dispatch first,
// then the chat-side three-view enrichment on the successful result. The
// harness result stays authoritative; the queue view and the summary lines
// are additive keys on it.
func (e pluginGrabberWorkflowExecutor) invokeMixReportTool(ctx context.Context, in executorpkg.Input, toolCallID string) (executorpkg.Result, error) {
	out, err := e.base.RunToolCall(ctx, in)
	if strings.TrimSpace(out.ToolCallID) == "" {
		out.ToolCallID = toolCallID
	}
	if err == nil && out.Result != nil {
		out.Result = e.server.enrichMixReportResult(ctx, out.Result)
		if strings.TrimSpace(out.Preview) == "" {
			if message, ok := out.Result["message"].(string); ok {
				out.Preview = firstLine(message)
			}
		}
	}
	return out, err
}

// enrichMixReportResult folds the queue node-state view (M1-2) into a
// successful mix_report result map and renders the F1 summary lines. It is
// fail-open: the queue view itself never errors, and a nil/failed report
// result passes through untouched.
func (s *Server) enrichMixReportResult(ctx context.Context, result map[string]any) map[string]any {
	if s == nil || result == nil {
		return result
	}
	view := s.BuildMixWorkflowQueueView(ctx)
	result["mix_workflow_queue"] = mixWorkflowQueueViewMap(view)
	result["message"] = renderMixReportThreeView(result, view)
	return result
}

// mixReportLatestSettleLine renders the latest settle conclusion carried by
// the mixboard report body (goal 1 fold) as one zh line. Empty string when
// the report carries no conclusion.
func mixReportLatestSettleLine(result map[string]any) string {
	latest, _ := result["latest_settle_conclusion"].(map[string]any)
	if len(latest) == 0 {
		return ""
	}
	kind := strings.TrimSpace(fmt.Sprintf("%v", latest["kind"]))
	outcome := strings.TrimSpace(fmt.Sprintf("%v", latest["outcome"]))
	settledAt := ""
	if raw, ok := latest["settled_at"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw)); err == nil {
			settledAt = parsed.Local().Format("2006-01-02 15:04")
		} else {
			settledAt = strings.TrimSpace(raw)
		}
	}
	line := strings.TrimSpace(strings.Trim(kind+" "+outcome, " "))
	if line == "" {
		line = "(未知结论形态)"
	}
	if summary := strings.TrimSpace(fmt.Sprintf("%v", latest["summary"])); summary != "" {
		line += "—" + summary
	}
	if settledAt != "" {
		line += "（收口于 " + settledAt + "）"
	}
	return line
}

// mixWorkflowStateSummaryLine renders the queue state summary in a fixed
// state order (matching the 9-state machine), skipping empty counts.
func mixWorkflowStateSummaryLine(summary map[string]int) string {
	order := []string{
		MixWorkflowNodeCompleted, MixWorkflowNodeInProgress, MixWorkflowNodeNeedsConfirmation,
		MixWorkflowNodeNeedsRecheck, MixWorkflowNodeBlocked, MixWorkflowNodeDeferred,
		MixWorkflowNodeSkipped, MixWorkflowNodeStale, MixWorkflowNodeNotStarted,
	}
	parts := []string{}
	for _, state := range order {
		if count := summary[state]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", mixWorkflowNodeStateLabels[state], count))
		}
	}
	return strings.Join(parts, " · ")
}

// renderMixReportThreeView renders the F1-facing summary: the latest settle
// conclusion line and the queue node-state line, plus the non-default nodes
// so the reviewer sees real queue state without opening the full view.
func renderMixReportThreeView(result map[string]any, view MixWorkflowQueueView) string {
	var builder strings.Builder
	builder.WriteString("mix.report 三视图：决策账本 + 自由态实验线 + queue 节点队列\n")
	builder.WriteString("最近 settle 结论：")
	if latest := mixReportLatestSettleLine(result); latest != "" {
		builder.WriteString(latest)
	} else {
		builder.WriteString("无记录（本工程自由态实验尚未收口，或旧工程无持久层记录）")
	}
	builder.WriteString(fmt.Sprintf("\nqueue 节点状态：%d 节点（%s）", len(view.Nodes), mixWorkflowStateSummaryLine(view.StateSummary)))
	nonDefault := []string{}
	for _, node := range view.Nodes {
		if node.State != MixWorkflowNodeNotStarted {
			nonDefault = append(nonDefault, fmt.Sprintf("%s %s[%s]", node.NodeID, node.Title, node.StateLabel))
		}
	}
	if len(nonDefault) > 0 {
		builder.WriteString("\n非缺省节点：" + strings.Join(nonDefault, " · "))
	}
	builder.WriteString(fmt.Sprintf("\n数据来源：%s（决策账本 %d 条 · 自由态结论 %d 条 · 节点标注 %d）",
		MixWorkflowQueueSchemaVersion, view.Sources.DecisionRecords, view.Sources.FreeStateConclusions, view.Sources.NodeAnnotations))
	if len(view.Sources.ReadErrors) > 0 {
		builder.WriteString("\n读取受限：" + strings.Join(view.Sources.ReadErrors, "；"))
	}
	return builder.String()
}
