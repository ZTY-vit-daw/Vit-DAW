package agentloop

// observe_presentation_test.go — OPT-IMPL-2 P2 单测：presentation 组装规则与
// 保守退化（设计 §4.3：summary=LLM 首段整段提取 ≤3 行；证据来自观察工具记录；
// 任一门槛不过=无块回落 P1）。

import (
	"strings"
	"testing"
)

func observePresentationTestState(barrier bool) *runState {
	state := &runState{}
	state.input.UserText = "帮我看看这首歌的低频，只读观察"
	if barrier {
		state.input.Context = map[string]any{
			"semantic_entry_verified": true,
			"semantic_entry_decision": map[string]any{
				"schema_version":     "semantic_entry_decision.v1",
				"route":              "observation",
				"user_authorization": "discussion",
			},
		}
	}
	return state
}

func observeObservationRecord(tool, status string, result map[string]any) map[string]any {
	record := map[string]any{"tool": tool, "status": status}
	if result != nil {
		record["result"] = result
	}
	return record
}

func TestObservePresentationSummaryFirstParagraph(t *testing.T) {
	reply := "Lead Vocal 在 110-160Hz 有约 3.1dB 的掩蔽风险，Bass 基频能量集中在 92Hz。\n\n详细分析如下：\n频段能量分布显示……\n更多内容。"
	summary := observePresentationSummary(reply)
	if summary == "" {
		t.Fatalf("expected first-paragraph summary, got empty")
	}
	if strings.Contains(summary, "详细分析") {
		t.Fatalf("summary leaked beyond the first paragraph: %q", summary)
	}
}

func TestObservePresentationSummaryDegradesOnLongLead(t *testing.T) {
	reply := strings.Repeat("line\n", 5) + "\nrest"
	if got := observePresentationSummary(reply); got != "" {
		t.Fatalf("expected degrade on >3-line lead, got %q", got)
	}
}

func TestObservePresentationFromTurnGates(t *testing.T) {
	reply := "结论一行。\n\n细节。"

	if got := observePresentationFromTurn(observePresentationTestState(false), reply); got != nil {
		t.Fatalf("expected nil without mutation barrier (non-observation route), got %+v", got)
	}

	noEvidence := observePresentationTestState(true)
	if got := observePresentationFromTurn(noEvidence, reply); got != nil {
		t.Fatalf("expected nil without observation evidence, got %+v", got)
	}

	emptyReply := observePresentationTestState(true)
	emptyReply.executed = []map[string]any{observeObservationRecord("mix.observe", "ok", map[string]any{"tracks": 2})}
	if got := observePresentationFromTurn(emptyReply, "  "); got != nil {
		t.Fatalf("expected nil on empty reply, got %+v", got)
	}
}

func TestObservePresentationFromTurnAssembles(t *testing.T) {
	state := observePresentationTestState(true)
	state.executed = []map[string]any{
		observeObservationRecord("project.state", "ok", nil), // 非观察工具，跳过
		observeObservationRecord("mix.observe", "ok", map[string]any{
			"observation_id":  "obs_20261007T210000_ab12",
			"track_1007_lufs": "-18.4",
			"track_1008_lufs": "-21.0",
		}),
		observeObservationRecord("ccb.observation_request", "ok", map[string]any{
			"observation_id": "obs_20261007T210500_cd34",
		}),
		observeObservationRecord("mix.read", "error", nil), // 失败记录跳过
	}
	presentation := observePresentationFromTurn(state, "Bass 与 Lead Vocal 在 100-160Hz 存在约 3.1dB 掩蔽风险。\n\n细节正文。")
	if presentation == nil {
		t.Fatalf("expected presentation, got nil")
	}
	if presentation.DetailMode != "layered" {
		t.Fatalf("detail_mode = %q, want layered", presentation.DetailMode)
	}
	if !strings.Contains(presentation.Summary, "3.1dB") {
		t.Fatalf("summary missing key value: %q", presentation.Summary)
	}
	if len(presentation.EvidenceEntries) != 2 {
		t.Fatalf("evidence entries = %d, want 2 (observation tools only, ok status only)", len(presentation.EvidenceEntries))
	}
	withRef := 0
	for _, entry := range presentation.EvidenceEntries {
		if entry.Ref != "" {
			withRef++
		}
		if entry.Source == "" || entry.Text == "" {
			t.Fatalf("entry missing source/text: %+v", entry)
		}
	}
	if withRef == 0 {
		t.Fatalf("expected at least one entry carrying observation_id ref")
	}
}

func TestObservePresentationEvidenceCapsEntries(t *testing.T) {
	state := observePresentationTestState(true)
	for i := 0; i < 12; i++ {
		state.executed = append(state.executed, observeObservationRecord("mix.observe", "ok", map[string]any{"k": "v"}))
	}
	entries := observePresentationEvidence(state)
	if len(entries) != observePresentationMaxEntries {
		t.Fatalf("entries = %d, want cap %d", len(entries), observePresentationMaxEntries)
	}
}

func TestObservePresentationEvidenceTextFlattensAndClips(t *testing.T) {
	long := map[string]any{"b": strings.Repeat("x", 300), "a": "1"}
	got := flattenEvidenceSummary(long)
	if !strings.HasPrefix(got, "a: 1") {
		t.Fatalf("flattened summary not key-sorted: %q", got)
	}
	if len(got) > observePresentationEntryTextMax+3 {
		t.Fatalf("flattened summary not clipped: %d chars", len(got))
	}
	if got := flattenEvidenceSummary(map[string]any{"a": "  "}); got != "" {
		t.Fatalf("expected empty flatten on blank values, got %q", got)
	}
}

func TestReadOnlyFinalReplyStashesPresentation(t *testing.T) {
	state := observePresentationTestState(true)
	state.executed = []map[string]any{observeObservationRecord("mix.observe", "ok", map[string]any{"lufs": "-18.4"})}
	reply := messageLoopReadOnlyFinalReply(state, "整体电平健康，Bass 偏响 1.2dB。\n\n细节。")
	if state.observePresentation == nil {
		t.Fatalf("expected presentation stashed on state, reply=%q", reply)
	}
	if strings.Contains(state.observePresentation.Summary, "只读") {
		t.Fatalf("read-only notice leaked into summary: %q", state.observePresentation.Summary)
	}
	// 尾注仍在正文里（既有行为零变化），presentation 只是附加结构。
	if !strings.Contains(reply, "只读") {
		t.Fatalf("read-only notice missing from reply text: %q", reply)
	}
}

func TestReadOnlyObservationIntentExported(t *testing.T) {
	if !ReadOnlyObservationIntent("帮我观察一下这首歌的低频") {
		t.Fatalf("expected observation intent for Chinese observe phrasing")
	}
	if ReadOnlyObservationIntent("帮我把低频衰减 2dB") {
		t.Fatalf("action request must not read as observation intent")
	}
}
