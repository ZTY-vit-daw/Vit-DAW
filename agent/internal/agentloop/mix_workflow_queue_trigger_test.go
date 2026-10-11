package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/fastpath"
)

// MIXLAYER-M1-2 trigger-face coverage: read/write predicates, annotation
// command parsing, and the fastpath registration contract (the two queue
// entries are part of DefaultEntryNames and registered in the same order).

func TestMixWorkflowQueueStatusRequestPredicate(t *testing.T) {
	for _, text := range []string{
		"混音工作队列", "看一下工作队列", "节点队列状态", "队列进展如何", "mix_workflow_queue", "show me the mix workflow queue",
	} {
		if !MixWorkflowQueueStatusRequest(text) {
			t.Fatalf("status predicate must match %q", text)
		}
	}
	for _, text := range []string{
		"帮我混音", "工程黑板状态", "音量平衡", "队列标记 C4 延期", "队列标注B2跳过", "继续",
	} {
		if MixWorkflowQueueStatusRequest(text) {
			t.Fatalf("status predicate must NOT match %q", text)
		}
	}
}

func TestMixWorkflowQueueNodeStateCommandParse(t *testing.T) {
	cases := []struct {
		text       string
		wantNode   string
		wantState  string
		wantReason string
	}{
		{"队列标记 C4 延期", "C4", "deferred", ""},
		{"队列标注B2跳过", "B2", "skipped", ""},
		{"队列标记 c3 阻塞 内核无 reverb inspector", "C3", "blocked", "内核无 reverb inspector"},
		{"队列标记 E1 进行中", "E1", "in_progress", ""},
		{"队列标记 D2 等待确认", "D2", "needs_confirmation", ""},
		{"队列标记 C4 恢复", "C4", "in_progress", ""},
		{"队列回写 A1 deferred 按用户要求", "A1", "deferred", "按用户要求"},
	}
	for _, tc := range cases {
		nodeID, state, reason, ok := ParseMixWorkflowQueueNodeStateCommand(tc.text)
		if !ok || nodeID != tc.wantNode || state != tc.wantState || reason != tc.wantReason {
			t.Fatalf("parse(%q) = (%q,%q,%q,%t), want (%q,%q,%q,true)", tc.text, nodeID, state, reason, ok, tc.wantNode, tc.wantState, tc.wantReason)
		}
	}
	for _, text := range []string{"队列标记 Z9 延期", "队列标记 C4 完成", "标记 C4 延期", "队列查看 C4"} {
		if _, _, _, ok := ParseMixWorkflowQueueNodeStateCommand(text); ok {
			t.Fatalf("annotation parse must reject %q (unknown node / derived-only state / wrong wording)", text)
		}
	}
}

func TestMixWorkflowQueueFastPathRegistrationContract(t *testing.T) {
	names := fastpath.DefaultEntryNames
	found := 0
	for _, name := range names {
		switch name {
		case "mix_workflow_queue_node_state", "mix_workflow_queue_status":
			found++
		}
	}
	if found != 2 {
		t.Fatalf("DefaultEntryNames must carry both queue entries, got %d: %v", found, names)
	}
	// The annotation entry precedes the status entry (specific command wins).
	joined := strings.Join(names, ",")
	if strings.Index(joined, "mix_workflow_queue_node_state") > strings.Index(joined, "mix_workflow_queue_status") {
		t.Fatalf("queue annotation entry must precede the status entry: %v", names)
	}
	router, err := (&MessageLoop{}).newFastPathRouter()
	if err != nil {
		t.Fatalf("canonical registration must pass the drift contract: %v", err)
	}
	queueNames := 0
	for _, name := range router.Names() {
		if strings.HasPrefix(name, "mix_workflow_queue_") {
			queueNames++
		}
	}
	if queueNames != 2 {
		t.Fatalf("router carries %d queue entries, want 2", queueNames)
	}
}
