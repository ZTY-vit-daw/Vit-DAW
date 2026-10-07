package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
)

// L1-4-IMPL-A agentloop 中性族 T-A1/T-A3/T-A5（CONTEXT_LAYERING_V1_DESIGN
// §3.3 治理表中性族行）：拆分判据 = 逐轮字节恒等测试——固定骨架（规则帧 +
// 会话内恒定的 catalog/allowed）留 system，凡逐轮可变的拼接件（状态
// directive、实验 tier 规则、full-access autonomy）物理迁 user 节。
// 红锚点（改造前）：system 段 Stable=true 但内容含状态拼装
// （ccb_model_prompt.go prefix + tier 槽）——"Stable 是意图不是机制"。

func neutralFamilyTestState(contextOverrides map[string]any) *runState {
	contextMap := map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
		},
	}
	for key, value := range contextOverrides {
		contextMap[key] = value
	}
	return &runState{
		input:  Input{UserText: "improve the mix", Context: contextMap, AllowedTools: []string{"ccb.observation_catalog"}},
		goal:   goalForNeutralFamilyTest,
		budget: Budget{MaxToolCalls: 12},
	}
}

// TestNeutralFamilySystemStaysByteIdenticalAcrossStateChanges（T-A3）：
// 逐轮可变的状态维度全部变化时，system 消息字节恒等；变化件全部落在
// user 节（P3 + 动态隔离）。
func TestNeutralFamilySystemStaysByteIdenticalAcrossStateChanges(t *testing.T) {
	loop := &MessageLoop{}
	baseline := loop.assembly(neutralFamilyTestState(nil), `{"turn":1,"created_at":"2026-10-07T09:00:01.000000001Z"}`)
	baselineSystem := assemblySystemText(t, baseline.Messages)
	if baselineSystem == "" {
		t.Fatalf("neutral assembly rendered no system message")
	}
	if !strings.Contains(baselineSystem, "neutral observation-and-family decision phase") {
		t.Fatalf("neutral-family skeleton missing from system message")
	}

	// 每个变体只改逐轮可变维度：连续预算临界、full-access 授权、
	// 剩余调用数、诊断轮相位。
	variants := []struct {
		name    string
		state   *runState
		markers []string // 必须出现在 user 节的逐轮件
	}{
		{
			name: "continuation_budget_critical",
			state: neutralFamilyTestState(map[string]any{
				"task_contract": map[string]any{"kind": "improvement"},
				"free_state_reasoning_loop": map[string]any{
					"schema_version": "free_state_reasoning_loop.v1", "status": "observing",
					"continuation_budget": 6, "continuation_used": 5,
				},
			}),
			markers: []string{"CONTINUATION BUDGET CRITICAL"},
		},
		{
			name: "full_access_granted",
			state: neutralFamilyTestState(map[string]any{
				"authority_mode":          "full_project_access",
				"authority_mode_explicit": true,
			}),
			markers: []string{"Full project access is granted for this turn"},
		},
		{
			name:    "remaining_tool_calls_shifted",
			state:   withRemainingCalls(neutralFamilyTestState(nil), 0, 9),
			markers: []string{"Remaining tool calls this run: 9"},
		},
	}
	for _, variant := range variants {
		variantAssembly := loop.assembly(variant.state, `{"turn":2,"created_at":"2026-10-07T09:00:02.000000002Z"}`)
		if got := assemblySystemText(t, variantAssembly.Messages); got != baselineSystem {
			t.Fatalf("%s: neutral system message bytes changed with turn state (P3 violated)", variant.name)
		}
		userTail := assemblyUserText(t, variantAssembly.Messages)
		for _, marker := range variant.markers {
			if !strings.Contains(userTail, marker) {
				t.Fatalf("%s: per-turn marker %q missing from the user turn", variant.name, marker)
			}
			if strings.Contains(baselineSystem, marker) {
				t.Fatalf("%s: per-turn marker %q leaked into the stable system skeleton", variant.name, marker)
			}
		}
	}
}

// TestNeutralFamilyP1AcrossControlledTurns（T-A1 agentloop 面）：同一 loop
// 受控多轮（快照逐轮换代、状态推进），system 字节恒等、前缀指纹不变、
// 无前缀断裂类 breaks。
func TestNeutralFamilyP1AcrossControlledTurns(t *testing.T) {
	loop := &MessageLoop{}
	var previousSystem string
	var previousFingerprint string
	for turn := 1; turn <= 4; turn++ {
		state := neutralFamilyTestState(map[string]any{
			"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
				"original_intent": "improve the mix", "continuation_used": turn,
			},
		})
		snapshot := strings.Repeat("x", turn) + `{"created_at":"2026-10-07T09:00:0` + string(rune('0'+turn)) + `.123456789Z"}`
		assembly, report, err := loop.assembleWithReport(state, snapshot)
		if err != nil {
			t.Fatalf("turn %d assemble: %v", turn, err)
		}
		current := assemblySystemText(t, assembly.Messages)
		if previousSystem != "" {
			if current != previousSystem {
				t.Fatalf("P3 violated at turn %d: neutral system bytes changed on an eventless turn", turn)
			}
			if report.PrefixFingerprint != previousFingerprint {
				t.Fatalf("P1 violated at turn %d: prefix fingerprint changed without a semantic event", turn)
			}
			for _, event := range report.Breaks {
				if event.Reason.PrefixBreaking() {
					t.Fatalf("turn %d: implicit prefix break %+v", turn, event)
				}
			}
			if len(report.CacheAnomalies) != 0 {
				t.Fatalf("turn %d: cache anomalies %+v", turn, report.CacheAnomalies)
			}
		}
		previousSystem = current
		previousFingerprint = report.PrefixFingerprint
		if report.PrefixBytes != len([]byte(current)) {
			t.Fatalf("turn %d: prefix bytes mismatch", turn)
		}
	}
}

// TestAgentLoopPrefixTelemetryJoinsExistingFamily（T-A5 agentloop 面）：
// AssemblyReport 三字段与既有 promptStats 族键并存且互不覆盖
// （model_snapshot_bytes 口径不变）。
func TestAgentLoopPrefixTelemetryJoinsExistingFamily(t *testing.T) {
	loop := &MessageLoop{}
	state := neutralFamilyTestState(nil)
	_, report, err := loop.assembleWithReport(state, `{"tracks":[]}`)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	merged := map[string]any{}
	for key, value := range messageLoopModelContextPromptStats(`{"tracks":[]}`, nil) {
		merged[key] = value
	}
	for key, value := range report.PromptStatsExtras() {
		merged[key] = value
	}
	if _, ok := merged["model_snapshot_bytes"]; !ok {
		t.Fatalf("existing model_snapshot_bytes telemetry key was dropped")
	}
	for _, key := range []string{"prefix_bytes", "dynamic_bytes", "breaks"} {
		if _, ok := merged[key]; !ok {
			t.Fatalf("prefix telemetry key %q missing from the merged stats family", key)
		}
	}
	if merged["prefix_bytes"].(int) == 0 {
		t.Fatalf("prefix bytes not measured on a neutral assembly")
	}
}

func assemblySystemText(t *testing.T, messages []llm.Message) string {
	t.Helper()
	for _, message := range messages {
		if strings.EqualFold(strings.TrimSpace(message.Role), "system") {
			return message.Content
		}
	}
	return ""
}

func assemblyUserText(t *testing.T, messages []llm.Message) string {
	t.Helper()
	for index := len(messages) - 1; index >= 0; index-- {
		if strings.EqualFold(strings.TrimSpace(messages[index].Role), "user") {
			return messages[index].Content
		}
	}
	return ""
}

var goalForNeutralFamilyTest = agentruntime.Goal{GoalID: "goal-layering", RunID: "run-layering"}

// withRemainingCalls returns a shallow state copy with the spent/max tool-call
// counters moved, leaving every prompt-relevant input untouched.
func withRemainingCalls(state *runState, used, max int) *runState {
	copied := *state
	copied.toolCallsUsed = used
	copied.budget.MaxToolCalls = max
	return &copied
}
