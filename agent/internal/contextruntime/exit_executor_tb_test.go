package contextruntime

// exit_executor_tb_test.go — T-B4/T-B5（CONTEXT_LAYERING_V1_DESIGN §6.2
// 退场一致性组）+ 判据/三态判定的机械覆盖。
//
//	T-B4 结论先于退场（不变式执法）：无结论无句柄的退场候选 → Violations
//	    非空、退场被拒（无 decision）、WARN 落遥测面、动态区宁可超预算。
//	T-B5 账本 append-only：逐轮快照账本文件字节序列=尾部追加（字节级
//	    starts-with），含撤销路径（撤销=追加 Supersedes 条目）。

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/contextruntime/carriers"
)

func fixedClock(t *testing.T) func() time.Time {
	t.Helper()
	base := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return base }
}

func findDecision(report ExitReport, id string) *ExitDecision {
	for i := range report.Decisions {
		if report.Decisions[i].Unit.ID == id {
			return &report.Decisions[i]
		}
	}
	return nil
}

// TestTB4InvariantRefusesEvidenceBearingExit：无结论无句柄的
// observation_bundle 退场候选被拒绝——Violations 非空、WARN 落遥测挂点、
// 不产 decision（单元留在动态窗，宁可超预算）。
func TestTB4InvariantRefusesEvidenceBearingExit(t *testing.T) {
	var warns []string
	executor := NewExitExecutor(ExitExecutorConfig{Warn: func(line string) { warns = append(warns, line) }})
	event := TurnBoundaryEvent{
		TurnID: "turn-2",
		Window: WindowState{Units: []WindowUnit{
			{Unit: ExitUnit{Kind: ExitUnitObservationBundle, ID: "obs_naked"}, Bytes: 900, TurnID: "turn-2"},
		}},
		Budget: BudgetState{HotBytes: 2000, HotLimitBytes: 1000}, // 超预算：拒退场=宁可超预算
	}
	report := executor.OnTurnBoundary(context.Background(), event)

	if len(report.Violations) == 0 {
		t.Fatalf("T-B4 red: violations empty for no-conclusion no-handle candidate")
	}
	if !strings.Contains(report.Violations[0], "obs_naked") {
		t.Fatalf("violation must locate the refused unit: %q", report.Violations[0])
	}
	if findDecision(report, "obs_naked") != nil {
		t.Fatalf("T-B4 red: refused unit must not receive an exit decision (宁可超预算)")
	}
	if len(warns) == 0 || !strings.Contains(warns[0], "obs_naked") {
		t.Fatalf("T-B4 red: WARN not delivered to telemetry hook: %v", warns)
	}
	if !strings.Contains(warns[0], "宁可超预算") {
		t.Fatalf("WARN text must state the invariant policy: %q", warns[0])
	}
}

// TestTB4TriStateDecisions：三态判定的机械覆盖——retain（结论在）、ref
// （句柄在）、drop（无证据义务/历史审计面）、CAS 例外路径与白名单外拒绝。
func TestTB4TriStateDecisions(t *testing.T) {
	var warns []string
	executor := NewExitExecutor(ExitExecutorConfig{
		Warn: func(line string) { warns = append(warns, line) },
		Now:  fixedClock(t),
		CASWriter: func(kind string, content any) (string, int64, error) {
			return "evidence://cafebabecafebabe", 42, nil
		},
	})
	event := TurnBoundaryEvent{
		TurnID: "turn-9",
		Window: WindowState{Units: []WindowUnit{
			{Unit: ExitUnit{Kind: ExitUnitObservationBundle, ID: "obs_concluded"}, TurnID: "turn-9",
				RetainedStatement: "T1 响度收敛至 -14.2 LUFS（结论级）",
				EvidenceRefs:      []string{"vit://mom/track:T1/t=all@obs_1#sha256:0123456789abcdef", "weird://opaque-artifact"}},
			{Unit: ExitUnit{Kind: ExitUnitObservationBundle, ID: "obs_ticketed"}, TurnID: "turn-9",
				HandleRef: "vit://dom/track:T1/t=all@obs_2#-"},
			{Unit: ExitUnit{Kind: ExitUnitToolResult, ID: "tool_plain"}, TurnID: "turn-9"}, // 无 refs 无结论 → drop
			{Unit: ExitUnit{Kind: ExitUnitToolResult, ID: "tool_cas"}, TurnID: "turn-9",
				CAS: &CASPayload{Kind: "tool_result_excerpt", Content: map[string]any{"hit": "long evidence segment"}}},
			{Unit: ExitUnit{Kind: ExitUnitToolResult, ID: "tool_refs_no_handle"}, TurnID: "turn-9",
				EvidenceRefs: []string{"vit://mom/track:T2/t=all@obs_3#-"}},
			{Unit: ExitUnit{Kind: ExitUnitObservationBundle, ID: "obs_cas_not_whitelisted"}, TurnID: "turn-9",
				CAS: &CASPayload{Kind: "observation_bundle", Content: map[string]any{"raw": true}}},
		}},
	}
	report := executor.OnTurnBoundary(context.Background(), event)

	if d := findDecision(report, "obs_concluded"); d == nil || d.Action != ExitRetain {
		t.Fatalf("concluded observation must retain: %+v", report.Decisions)
	} else {
		if d.LedgerEntry == nil || d.LedgerEntry.Kind != carriers.LedgerKindObservationConclusion {
			t.Fatalf("retain pending entry kind must be observation_conclusion family: %+v", d.LedgerEntry)
		}
		if len(d.LedgerEntry.EvidenceRefs) != 2 {
			t.Fatalf("opaque artifact tolerated (宽容透传), both refs admitted: %+v", d.LedgerEntry.EvidenceRefs)
		}
		if d.Reason != ExitReasonTurnEnd {
			t.Fatalf("reason = %s, want turn_end", d.Reason)
		}
	}
	if d := findDecision(report, "obs_ticketed"); d == nil || d.Action != ExitRef || d.HandleRef == "" {
		t.Fatalf("ticketed observation must ref with handle: %+v", report.Decisions)
	}
	if d := findDecision(report, "tool_plain"); d == nil || d.Action != ExitDrop {
		t.Fatalf("no-refs tool result must drop (无证据义务): %+v", report.Decisions)
	}
	if d := findDecision(report, "tool_cas"); d == nil || d.Action != ExitRef || d.HandleRef != "evidence://cafebabecafebabe" {
		t.Fatalf("whitelisted tool_result must CAS-handle: %+v", report.Decisions)
	}
	// 白名单外（observation_bundle 带 CAS 载荷）：不走 CAS；无结论无句柄 → 拒退场。
	if findDecision(report, "obs_cas_not_whitelisted") != nil {
		t.Fatalf("observation_bundle must not take the CAS exception path (白名单 v1=tool_result only)")
	}
	if findDecision(report, "tool_refs_no_handle") != nil {
		t.Fatalf("refs-bearing tool result without handle/conclusion must be refused")
	}
	if len(report.Violations) != 2 {
		t.Fatalf("expected exactly 2 violations (naked bundle + non-whitelisted CAS), got %v", report.Violations)
	}
}

// TestTB4CriteriaAndBudgetOrder：判据优先序（turn_end > semantic_expiry >
// window_slide > budget）与预算淘汰的确定性序（最旧优先）。
func TestTB4CriteriaAndBudgetOrder(t *testing.T) {
	executor := NewExitExecutor(ExitExecutorConfig{})
	event := TurnBoundaryEvent{
		TurnID: "turn-5",
		Window: WindowState{
			HistoryLimit: 2,
			Units: []WindowUnit{
				{Unit: ExitUnit{Kind: ExitUnitHistoryMessage, ID: "h0"}, Bytes: 10},
				{Unit: ExitUnit{Kind: ExitUnitHistoryMessage, ID: "h1"}, Bytes: 10},
				{Unit: ExitUnit{Kind: ExitUnitHistoryMessage, ID: "h2"}, Bytes: 10}, // slide（超限 1 条）
				{Unit: ExitUnit{Kind: ExitUnitToolResult, ID: "t-old"}, Bytes: 700}, // budget（最旧非候选）
				{Unit: ExitUnit{Kind: ExitUnitToolResult, ID: "t-expiry"}, Bytes: 100, SemanticExpiry: true},
				{Unit: ExitUnit{Kind: ExitUnitObservationBundle, ID: "obs-this-turn"}, Bytes: 50, TurnID: "turn-5",
					HandleRef: "vit://mom/track:T9/t=all@obs_c#-"},
			},
		},
		Budget: BudgetState{HotBytes: 1000, HotLimitBytes: 200},
	}
	report := executor.OnTurnBoundary(context.Background(), event)
	reasons := map[string]string{}
	for _, d := range report.Decisions {
		reasons[d.Unit.ID] = d.Reason
	}
	if reasons["h0"] != ExitReasonWindowSlide {
		t.Fatalf("oldest history beyond limit must window_slide: %+v", reasons)
	}
	if d := findDecision(report, "h0"); d == nil || d.Action != ExitDrop {
		t.Fatalf("history slide must drop (不可变审计面；refs 由伴随索引覆盖): %+v", report.Decisions)
	}
	if _, ok := reasons["h1"]; ok {
		t.Fatalf("history within limit must not exit: %+v", reasons)
	}
	if reasons["t-expiry"] != ExitReasonSemanticExpiry {
		t.Fatalf("semantic expiry candidate: %+v", reasons)
	}
	if reasons["obs-this-turn"] != ExitReasonTurnEnd {
		t.Fatalf("current-turn raw content must turn_end: %+v", reasons)
	}
	if reasons["t-old"] != ExitReasonBudget {
		t.Fatalf("oldest non-candidate must budget-evict: %+v", reasons)
	}
	if findDecision(report, "h2") != nil {
		t.Fatalf("history within limit (h2) must stay: %+v", reasons)
	}
}

// TestTB5LedgerAppendOnlyBytes：逐轮 retain 接线落盘后，账本文件字节序列
// 满足「每轮文件=上轮文件+尾部追加」（字节级 starts-with）；撤销路径=
// 追加 Supersedes 条目（同样只追加）；链校验通过。
func TestTB5LedgerAppendOnlyBytes(t *testing.T) {
	projectDir := t.TempDir()
	executor := NewExitExecutor(ExitExecutorConfig{Now: fixedClock(t)})
	clock := fixedClock(t)

	ledgerFile := func() []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(carriers.LedgerRelPath)))
		if err != nil {
			t.Fatalf("read ledger: %v", err)
		}
		return data
	}
	runTurn := func(turnID, statement string) {
		t.Helper()
		report := executor.OnTurnBoundary(context.Background(), TurnBoundaryEvent{
			TurnID: turnID,
			Window: WindowState{Units: []WindowUnit{{
				Unit: ExitUnit{Kind: ExitUnitObservationBundle, ID: "obs_" + turnID}, TurnID: turnID,
				RetainedStatement: statement,
				EvidenceRefs:      []string{"vit://mom/track:T1/t=all@" + turnID + "#-"},
			}}},
		})
		if len(report.Decisions) != 1 || report.Decisions[0].Action != ExitRetain {
			t.Fatalf("turn %s: expected one retain decision, got %+v", turnID, report.Decisions)
		}
		if _, err := WriteRetains(projectDir, report, clock()); err != nil {
			t.Fatalf("WriteRetains turn %s: %v", turnID, err)
		}
	}

	var previous []byte
	for round, turnID := range []string{"turn-1", "turn-2", "turn-3"} {
		runTurn(turnID, "结论 "+turnID)
		current := ledgerFile()
		if round > 0 && !bytes.HasPrefix(current, previous) {
			t.Fatalf("T-B5 red: ledger round %d is not a byte-level append (len %d vs %d)",
				round+1, len(current), len(previous))
		}
		previous = current
	}

	// 撤销路径：撤销也是追加（Supersedes 条目），前缀字节仍不动。
	if _, err := carriers.AppendLedgerEntry(projectDir, carriers.LedgerKindUserOverride, "",
		"撤销结论 turn-1（保留原文，以本条为准）", nil, 1, clock()); err != nil {
		t.Fatalf("supersede append: %v", err)
	}
	if current := ledgerFile(); !bytes.HasPrefix(current, previous) {
		t.Fatalf("T-B5 red: supersede path rewrote ledger prefix (撤销必须=追加)")
	}
	entries, err := carriers.ReadLedger(projectDir)
	if err != nil {
		t.Fatalf("chain verify after supersedes: %v", err)
	}
	if len(entries) != 4 || entries[3].Supersedes != 1 {
		t.Fatalf("expected 4 entries with #4 superseding #1: %+v", entries)
	}
}
