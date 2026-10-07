package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/llm"
)

// L1-4-IMPL-A chat 入口 T-A1/T-A3/T-A5（CONTEXT_LAYERING_V1_DESIGN §3.3
// 治理表 chat 行）：chat_context_snapshot 迁 user 节尾部，稳定 system 只
// 装 Section——无语义事件轮次 system 字节恒等（P3），动态内容全在 user
// 节。红锚点（改造前可复现）：snapshot 挂 SystemSections + CreatedAt
// RFC3339Nano 逐轮变（contextruntime/context.go:133）→ 整条 system 逐轮变。

func chatAssemblySystemText(t *testing.T, messages []llm.Message) string {
	t.Helper()
	parts := []string{}
	for _, message := range messages {
		if strings.EqualFold(strings.TrimSpace(message.Role), "system") {
			parts = append(parts, message.Content)
		}
	}
	return strings.Join(parts, "\n---\n")
}

func chatAssemblyUserTail(t *testing.T, messages []llm.Message) string {
	t.Helper()
	for index := len(messages) - 1; index >= 0; index-- {
		if strings.EqualFold(strings.TrimSpace(messages[index].Role), "user") {
			return messages[index].Content
		}
	}
	return ""
}

// TestChatAssemblyStableSystemAcrossTurns（T-A1/T-A3 chat 面）：同会话两轮
// 装配，稳定 system 消息字节恒等（P3），动态 snapshot 全在 user 节尾部。
func TestChatAssemblyStableSystemAcrossTurns(t *testing.T) {
	server := newChatServerForTest(t, nil, nil, nil)
	ctx := context.Background()
	first := server.buildAssembly(ctx, "conv-prefix-layering", "第一轮：看看轨道。", map[string]any{})
	second := server.buildAssembly(ctx, "conv-prefix-layering", "第二轮：再看看。", map[string]any{})
	firstSystem := chatAssemblySystemText(t, first.Messages)
	secondSystem := chatAssemblySystemText(t, second.Messages)
	if firstSystem == "" {
		t.Fatalf("chat assembly rendered no system message")
	}
	if firstSystem != secondSystem {
		t.Fatalf("P3 violated: chat system message bytes differ between eventless turns")
	}
	firstTail := chatAssemblyUserTail(t, first.Messages)
	secondTail := chatAssemblyUserTail(t, second.Messages)
	if !strings.Contains(firstTail, "第一轮：看看轨道。") || !strings.Contains(secondTail, "第二轮：再看看。") {
		t.Fatalf("per-turn user text missing from the dynamic user tail")
	}
	if firstTail == secondTail {
		t.Fatalf("dynamic tail should rotate per turn (snapshot CreatedAt)")
	}
	if strings.Contains(firstSystem, "Context snapshot JSON") || strings.Contains(secondSystem, "Context snapshot JSON") {
		t.Fatalf("P3 violated: context snapshot still rendered inside the system message")
	}
	if !strings.Contains(firstTail, "Context snapshot JSON") {
		t.Fatalf("context snapshot missing from the user tail")
	}
}

// TestChatAssemblyPrefixReport（T-A5 chat 面 + §3.1 层报告）：chat 装配经
// PrefixService 产出报告——prefix_bytes/dynamic_bytes 非零、无隐式断裂、
// 快照轮换记 snapshot_rotated（仅报告类）。
func TestChatAssemblyPrefixReport(t *testing.T) {
	server := newChatServerForTest(t, nil, nil, nil)
	ctx := context.Background()
	_, firstReport, err := server.buildAssemblyWithReport(ctx, "conv-prefix-report", "第一轮。", map[string]any{})
	if err != nil {
		t.Fatalf("first assemble: %v", err)
	}
	if firstReport.PrefixBytes == 0 || firstReport.DynamicBytes == 0 {
		t.Fatalf("byte accounting missing: %+v", firstReport)
	}
	if len(firstReport.Layers) == 0 {
		t.Fatalf("stable layer missing from report: %+v", firstReport)
	}
	if firstReport.Layers[0].LayerID != "chat_system" || firstReport.Layers[0].State != "rendered" {
		t.Fatalf("unexpected chat layer report: %+v", firstReport.Layers[0])
	}
	_, secondReport, err := server.buildAssemblyWithReport(ctx, "conv-prefix-report", "第二轮。", map[string]any{})
	if err != nil {
		t.Fatalf("second assemble: %v", err)
	}
	for _, event := range secondReport.Breaks {
		if event.Reason.PrefixBreaking() {
			t.Fatalf("implicit prefix break on an eventless chat turn: %+v", event)
		}
	}
	extras := secondReport.PromptStatsExtras()
	if _, ok := extras["prefix_bytes"]; !ok {
		t.Fatalf("prompt stats extras missing prefix_bytes")
	}
}
