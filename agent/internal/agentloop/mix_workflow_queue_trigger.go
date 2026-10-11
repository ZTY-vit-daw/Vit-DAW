package agentloop

import (
	"context"
	"regexp"
	"strings"
	"time"

	"vit-daw-agent/internal/planner"
)

// MIXLAYER-M1-2 mix_workflow_queue.v0 deterministic preflights. The queue is
// a projection view over the blackboard persistence layer (decision ledger +
// free-state conclusions + node annotations); these preflights are the
// agent-facing read/write face inside the message loop. They emit the two
// chat-intercepted queue tools (mix_queue_view / mix_queue_node_state) —
// bookkeeping only, never a capability dispatch entry (card red line).

var messageLoopMixWorkflowQueueNodeStatePattern = regexp.MustCompile(
	`(?:队列标记|队列标注|队列回写)[\s,，:：]*([A-Fa-f][1-5])[\s,，:：]*` +
		`(延期|跳过|阻塞|进行中|等待确认|恢复|deferred|skipped|blocked|in_progress|needs_confirmation)` +
		`(?:[\s,，:：]+(.+))?`)

var messageLoopMixWorkflowQueueStateWords = map[string]string{
	"延期": "deferred", "跳过": "skipped", "阻塞": "blocked",
	"进行中": "in_progress", "等待确认": "needs_confirmation", "恢复": "in_progress",
	"deferred": "deferred", "skipped": "skipped", "blocked": "blocked",
	"in_progress": "in_progress", "needs_confirmation": "needs_confirmation",
}

// ParseMixWorkflowQueueNodeStateCommand parses an annotation write-back
// command: “队列标记 C4 延期 <可选原因>”. Exported for the chat-side
// deterministic queue intercept (MIXLAYER-M1-2), which must recognize the
// same wording before semantic-entry classification can swallow it.
func ParseMixWorkflowQueueNodeStateCommand(text string) (nodeID, state, reason string, ok bool) {
	match := messageLoopMixWorkflowQueueNodeStatePattern.FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return "", "", "", false
	}
	state, ok = messageLoopMixWorkflowQueueStateWords[strings.ToLower(strings.TrimSpace(match[2]))]
	if !ok {
		return "", "", "", false
	}
	return strings.ToUpper(strings.TrimSpace(match[1])), state, strings.TrimSpace(match[3]), true
}

// MixWorkflowQueueStatusRequest matches queue view reads. The annotation
// command is deliberately excluded so the write face wins on its own
// wording. Exported for the chat-side deterministic queue intercept.
func MixWorkflowQueueStatusRequest(text string) bool {
	trimmed := strings.TrimSpace(text)
	if messageLoopMixWorkflowQueueNodeStatePattern.MatchString(trimmed) {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, keyword := range []string{
		"混音工作队列", "工作队列", "节点队列", "队列状态", "队列进展", "队列视图",
		"mix_workflow_queue", "mix workflow queue", "workflow queue",
	} {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func (l *MessageLoop) preflightMixWorkflowQueueNodeState(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	if state == nil {
		return false, Result{}
	}
	nodeID, targetState, reason, ok := ParseMixWorkflowQueueNodeStateCommand(state.input.UserText)
	if !ok {
		return false, Result{}
	}
	args := map[string]any{"node_id": nodeID, "state": targetState}
	if reason != "" {
		args["reason"] = reason
	}
	call := planner.ToolCall{
		ID:     "mix_workflow_queue_node_state",
		Tool:   "mix_queue_node_state",
		Args:   args,
		Reason: "record one mix workflow queue node annotation (write-back face)",
	}
	if stopped, result := r.checkpoint("before_message_loop_mix_workflow_queue_node_state", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "mix workflow queue annotation was routed through mix_workflow_queue.v0",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false mix_workflow_queue_node_state=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "mix workflow queue node annotation completed the turn"})
	return true, r.complete(state, messageLoopMixWorkflowQueueReplyFromState(state, "无法回写节点标注"))
}

func (l *MessageLoop) preflightMixWorkflowQueueStatus(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	if state == nil || !MixWorkflowQueueStatusRequest(state.input.UserText) {
		return false, Result{}
	}
	call := planner.ToolCall{
		ID:     "mix_workflow_queue_view",
		Tool:   "mix_queue_view",
		Args:   map[string]any{},
		Reason: "read the mix_workflow_queue.v0 projection for the current project",
	}
	if stopped, result := r.checkpoint("before_message_loop_mix_workflow_queue_status", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "mix workflow queue status request was routed through mix_workflow_queue.v0",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false mix_workflow_queue_status=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "mix workflow queue status view completed the turn"})
	return true, r.complete(state, messageLoopMixWorkflowQueueReplyFromState(state, "无法读取混音工作队列"))
}

// messageLoopMixWorkflowQueueReplyFromState surfaces the chat-side render
// carried on the executed tool record (result.message); a failed tool renders
// its error honestly instead of a fabricated queue.
func messageLoopMixWorkflowQueueReplyFromState(state *runState, fallback string) string {
	if state == nil {
		return fallback
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		name := strings.ToLower(strings.TrimSpace(messageLoopText(record["tool"])))
		if name != "mix_queue_view" && name != "mix_queue_node_state" {
			continue
		}
		result := messageLoopMapValue(record["result"])
		if text := messageLoopText(result["message"]); text != "" {
			return text
		}
		if text := messageLoopText(record["error"]); text != "" {
			return fallback + "：" + text
		}
	}
	return fallback
}
