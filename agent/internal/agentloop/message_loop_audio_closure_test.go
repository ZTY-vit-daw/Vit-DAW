package agentloop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
)

func TestMessageLoopAudioClosureEmptyOutputBecomesTypedProtocolFailure(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		"",
		`{"final":false,"failure_reason":"model_protocol_failure","reply":"","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{}, Budget: Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText: "make the vocal less harsh",
		Context: map[string]any{"minimal_audio_closure": map[string]any{
			"schema_version": "minimal_audio_closure.v1", "phase": "reasoning", "original_intent": "make the vocal less harsh",
		}},
	})
	if res.StopReason != StopReasonModelProtocolFailure || !res.ModelProtocolFailure || res.ModelProtocolRepairs != 1 || res.NeedsClarification {
		t.Fatalf("empty output was not typed as protocol failure: %+v", res)
	}
	if len(client.calls) != 2 {
		t.Fatalf("model calls=%d want=2", len(client.calls))
	}
	repairPrompt := client.calls[1][0].Content
	if strings.Contains(repairPrompt, `"needs_clarification":true`) || !strings.Contains(repairPrompt, "never ask the user to restate") {
		t.Fatalf("closure repair prompt permits clarification: %s", repairPrompt)
	}
}

func TestMessageLoopAudioClosureDistinguishesSuccessfulRepair(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		"not json",
		`{"final":true,"reply":"bounded conclusion","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{}, Budget: Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText: "diagnose the issue",
		Context: map[string]any{"minimal_audio_closure": map[string]any{
			"schema_version": "minimal_audio_closure.v1", "phase": "reasoning",
		}},
	})
	if res.StopReason != StopReasonDone || res.ModelProtocolFailure || res.ModelProtocolRepairs != 1 || res.Reply != "bounded conclusion" {
		t.Fatalf("successful repair was misclassified: %+v", res)
	}
}

// FIX-REPAIR-CLARIFY-DEATH-1 forensic shape: the original output is a
// well-formed proposal missing one closing brace (variance A) and the repair
// parses but violates the closure-variant ban by returning a
// needs_clarification object (variance B).
const repairClarifyMalformedOutput = `{"final":true,"reply":"3kHz presence lift proposal","tool_calls":[{"tool":"track.volume"`

const repairClarifyViolatingRepair = `{"final":false,"needs_clarification":true,"clarification_question":"需要先确认哪条是主唱轨。","reply":"需要先确认哪条是主唱轨。","tool_calls":[]}`

func repairClarifyTestLoop(client *fakeMessageCompleter) *MessageLoop {
	return &MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{}, Budget: Budget{MaxTurns: 5, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
}

func repairClarifyTestInput() Input {
	return Input{
		UserText: "make the lead vocal more forward",
		Context: map[string]any{"minimal_audio_closure": map[string]any{
			"schema_version": "minimal_audio_closure.v1", "phase": "fs2_capacity_assessed",
		}},
	}
}

func readMessageLoopDiagnosticRows(t *testing.T, path string) map[string]messageLoopDiagnostic {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read message loop diagnostics: %v", err)
	}
	rows := map[string]messageLoopDiagnostic{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row messageLoopDiagnostic
		if json.Unmarshal([]byte(line), &row) == nil {
			rows[row.Stage] = row
		}
	}
	return rows
}

// 红1（修前红）：closure 在案 + repair 返回 needs_clarification（违令）后，
// 必须先追加一次禁-clarify 强化 repair；二次合规则本轮以恢复内容完成（不死）。
func TestMessageLoopAudioClosureRepairClarifyViolationGetsOneReinforcedRepair(t *testing.T) {
	debugPath := filepath.Join(t.TempDir(), "agent_message_loop_debug.jsonl")
	t.Setenv("VIT_AGENT_MESSAGE_LOOP_DEBUG_PATH", debugPath)
	client := &fakeMessageCompleter{responses: []string{
		repairClarifyMalformedOutput,
		repairClarifyViolatingRepair,
		`{"final":true,"reply":"bounded conclusion","tool_calls":[]}`,
	}}
	res := repairClarifyTestLoop(client).Start(context.Background(), repairClarifyTestInput())
	if res.NeedsClarification || res.ModelProtocolFailure || res.StopReason != StopReasonDone || !strings.HasPrefix(res.Reply, "bounded conclusion") {
		t.Fatalf("clarify-violating closure repair was not recovered by the reinforced repair: %+v", res)
	}
	if strings.Contains(res.Reply, "主唱") {
		t.Fatalf("the violating clarification question leaked into the recovered reply: %s", res.Reply)
	}
	if res.ModelProtocolRepairs != 1 {
		t.Fatalf("violation+reinforce must count as one protocol repair episode against the closure budget: %+v", res)
	}
	if len(client.calls) != 3 {
		t.Fatalf("model calls=%d want=3 (original + violating repair + reinforced repair)", len(client.calls))
	}
	reinforcedPrompt := client.calls[2][0].Content
	if !strings.Contains(reinforcedPrompt, "needs_clarification") || !strings.Contains(reinforcedPrompt, "violated the contract") || !strings.Contains(reinforcedPrompt, "never ask the user to restate") {
		t.Fatalf("reinforced repair prompt does not restate the ban and name the previous violation: %s", reinforcedPrompt)
	}
	rows := readMessageLoopDiagnosticRows(t, debugPath)
	if row, ok := rows["repair_succeeded"]; !ok || !strings.Contains(row.Raw, "needs_clarification") {
		t.Fatalf("repair success diagnostic row missing or without raw evidence: %+v", row)
	}
	if _, ok := rows["repair_clarify_violation"]; !ok {
		t.Fatalf("repair clarify violation diagnostic row missing: %+v", rows)
	}
	if row, ok := rows["repair_clarify_reinforce"]; !ok || row.Error != "" || !strings.Contains(row.Raw, "bounded conclusion") {
		t.Fatalf("reinforced repair diagnostic row missing or not recording the recovered raw: %+v", row)
	}
}

// 修后语义：二次仍违令——结果面保持 NeedsClarification 暂停（chat 层 557 判死
// 由 TestAudioClosureRepairClarificationBecomesProtocolFailure 锁定），且强化
// 调用恰好一次。修前红：无强化调用即暂停（calls=2）。
func TestMessageLoopAudioClosureRepairClarifyDoubleViolationStillPauses(t *testing.T) {
	debugPath := filepath.Join(t.TempDir(), "agent_message_loop_debug.jsonl")
	t.Setenv("VIT_AGENT_MESSAGE_LOOP_DEBUG_PATH", debugPath)
	client := &fakeMessageCompleter{responses: []string{
		repairClarifyMalformedOutput,
		repairClarifyViolatingRepair,
		`{"final":false,"needs_clarification":true,"clarification_question":"还是需要确认主唱轨。","reply":"还是需要确认主唱轨。","tool_calls":[]}`,
	}}
	res := repairClarifyTestLoop(client).Start(context.Background(), repairClarifyTestInput())
	if !res.NeedsClarification || res.ModelProtocolFailure {
		t.Fatalf("double violation must surface the clarification pause for the chat-layer protocol-death classification: %+v", res)
	}
	if res.ModelProtocolRepairs != 1 {
		t.Fatalf("double violation must count as one protocol repair episode: %+v", res)
	}
	if len(client.calls) != 3 {
		t.Fatalf("model calls=%d want=3 (original + violating repair + one reinforced repair)", len(client.calls))
	}
	if len(client.calls) == 3 && client.calls[2][0].Content == client.calls[1][0].Content {
		t.Fatalf("reinforced repair prompt must differ from the plain closure repair prompt")
	}
	rows := readMessageLoopDiagnosticRows(t, debugPath)
	if row, ok := rows["repair_clarify_reinforce"]; !ok || !strings.Contains(row.Error, "still returned a prohibited needs_clarification") || !strings.Contains(row.Raw, "还是需要确认主唱轨") {
		t.Fatalf("second-violation diagnostic row missing or without raw evidence: %+v", row)
	}
}

// 红2（修前绿，F4 判死路径回归锁）：真畸形不可修复（repair 仍 parse_failed）
// 不给强化机会，仍按 model protocol failure 判死。
func TestMessageLoopAudioClosureUnrecoverableRepairStillDiesAsProtocolFailure(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		"not json",
		"still not json",
	}}
	res := repairClarifyTestLoop(client).Start(context.Background(), repairClarifyTestInput())
	if res.StopReason != StopReasonModelProtocolFailure || !res.ModelProtocolFailure || res.NeedsClarification || res.ModelProtocolRepairs != 0 {
		t.Fatalf("unrecoverable repair output must keep the protocol-failure death: %+v", res)
	}
	if len(client.calls) != 2 {
		t.Fatalf("unrecoverable parse must not trigger a reinforced repair: calls=%d", len(client.calls))
	}
}
