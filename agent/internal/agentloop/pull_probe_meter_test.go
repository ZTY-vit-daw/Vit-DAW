package agentloop

// pull_probe_meter_test.go — PULL-PROBE-METER-1：probe 物理成本计量接入的
// D5 分级结算面（execRecord.elapsed_ms → settleBatch → ledger.probeSpent）。
// PULL-PROBE-TIER-EXT-1 修订：clip.warm_waveform_bake 经裁定扩入 probe 级
// （rulings/2026-10-10-EVENING-BATCH §3），原"两属性不私定"断言随之翻面。

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/pullharness"
)

// meteredSession 直构最小 pull 会话（settleBatch 只触 state.executed/
// batchStart/ledger 三面，零值安全），账本从缺省已知零成本起步。
func meteredSession(records []map[string]any, maxProbeCost float64) *pullSession {
	return &pullSession{
		state: &runState{input: Input{Context: map[string]any{}}, executed: records},
		ledger: pullLedger{
			probeCostKnown: true,
			settled:        map[string]bool{},
			maxProbeCost:   maxProbeCost,
		},
	}
}

// 已知成本累加：probe 级回执的 elapsed_ms 逐笔入账，probeCostKnown 保持
// true；账本视图与披露行读到实测累计值（index 级不计）。
func TestPullSessionMeteredProbeCostAccumulates(t *testing.T) {
	session := meteredSession([]map[string]any{
		{"tool_call_id": "r1", "tool": "mix.observe", "elapsed_ms": int64(100)},
		{"tool_call_id": "r2", "tool": "ccb.observation_request", "elapsed_ms": int64(50)},
		{"tool_call_id": "r3", "tool": "ref.query", "elapsed_ms": int64(7)},
	}, 8)
	fresh := session.settleBatch()
	if len(fresh) != 3 {
		t.Fatalf("fresh receipts = %v, want 3", fresh)
	}
	if session.ledger.probeSpent != 150 {
		t.Errorf("probe spent = %v, want 150 (100+50 逐笔累加；index 级 7 不计)", session.ledger.probeSpent)
	}
	if !session.ledger.probeCostKnown {
		t.Error("metered probe receipts must keep probeCostKnown=true (enforcement gate armed)")
	}
	view := session.ledger.view()
	if view.ProbeSpent != 150 || !view.ProbeCostKnown {
		t.Errorf("ledger view = %+v, want ProbeSpent=150 ProbeCostKnown=true", view)
	}
	line := pullharness.ObservationBudget{MaxCycles: 5, MaxProbeCost: 8}.Disclosure(2, session.ledger.probeSpent)
	if !strings.Contains(line, "probe_spent=150/8") {
		t.Errorf("disclosure = %q, want measured probe_spent=150/8", line)
	}
}

// 预算越线执法：probeCostKnown=true 路径——max_probe_cost 门以实测累计值
// 真实激活（exhausted 在 9/8 越线）。
func TestPullSessionMeteredProbeCostEnforcesBudget(t *testing.T) {
	session := meteredSession([]map[string]any{
		{"tool_call_id": "r1", "tool": "mix.observe", "elapsed_ms": int64(3)},
		{"tool_call_id": "r2", "tool": "mix.observe", "elapsed_ms": int64(6)},
	}, 8)
	session.settleBatch()
	if !session.ledger.probeCostKnown {
		t.Fatal("metered probe receipts must keep probeCostKnown=true")
	}
	if !session.ledger.exhausted() {
		t.Errorf("exhausted = false at probe_spent=%v/max=8, want true (max_probe_cost enforcement armed by real metering)", session.ledger.probeSpent)
	}
}

// 无键回执翻 false 回归：probe 级回执缺 elapsed_ms=未计量——不填 0 入账，
// 账本翻 unknown（诚实语义保留）。
func TestPullSessionUnmeteredProbeReceiptFlipsUnknown(t *testing.T) {
	session := meteredSession([]map[string]any{
		{"tool_call_id": "r1", "tool": "mix.observe"},
	}, 0)
	session.settleBatch()
	if session.ledger.probeCostKnown {
		t.Error("unmetered probe receipt must flip probeCostKnown=false (no zero-cost impersonation)")
	}
	if session.ledger.probeSpent != 0 {
		t.Errorf("probe spent = %v, want 0 (未计量不填 0 不入账)", session.ledger.probeSpent)
	}
}

// index 级不计且不翻：D5 定义零成本（budget.go:17-19），catalog/state 读
// 即使带 elapsed_ms 也不入账、不制造 unknown。
func TestPullSessionIndexTierNotCounted(t *testing.T) {
	session := meteredSession([]map[string]any{
		{"tool_call_id": "r1", "tool": "ref.query", "elapsed_ms": int64(100)},
		{"tool_call_id": "r2", "tool": "ref.diff", "elapsed_ms": int64(3)},
	}, 0)
	session.settleBatch()
	if session.ledger.probeSpent != 0 {
		t.Errorf("probe spent = %v, want 0 (index 级零成本不计)", session.ledger.probeSpent)
	}
	if !session.ledger.probeCostKnown {
		t.Error("index-tier receipts are D5-known zero cost; must not flip unknown")
	}
}

// 未列名工具按 unknown 处置：分级表不私定扩权（争议上交），维持既有
// unknown 语义——入账零、翻 false（warm bake 已裁定入 probe 级，不再作
// 本例；unlisted 语义本身保留）。
func TestPullSessionUnlistedTierKeepsUnknownSemantics(t *testing.T) {
	session := meteredSession([]map[string]any{
		{"tool_call_id": "r1", "tool": "pull.echo", "elapsed_ms": int64(5)},
		{"tool_call_id": "r2", "tool": "some.future.tool", "elapsed_ms": int64(9)},
	}, 0)
	session.settleBatch()
	if session.ledger.probeSpent != 0 {
		t.Errorf("probe spent = %v, want 0 (unlisted tier never accumulates)", session.ledger.probeSpent)
	}
	if session.ledger.probeCostKnown {
		t.Error("unlisted-tier receipts keep the unknown-cost semantics (probeCostKnown must flip false)")
	}
}

// 分级表锁定：probe/index 两级名单（含下划线别名归一）+未列名兜底。
func TestProbeToolTierTable(t *testing.T) {
	for _, tool := range []string{"ccb.observation_request", "ccb_observation_request", "mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation"} {
		if got := probeToolTier(tool); got != probeTierProbe {
			t.Errorf("probeToolTier(%q) = %q, want %q", tool, got, probeTierProbe)
		}
	}
	for _, tool := range []string{"ref.query", "ref.diff", "ccb.observation_catalog", "project.state"} {
		if got := probeToolTier(tool); got != probeTierIndex {
			t.Errorf("probeToolTier(%q) = %q, want %q", tool, got, probeTierIndex)
		}
	}
	for _, tool := range []string{"clip.warm_waveform_bake", "clip.warm.waveform.bake"} {
		if got := probeToolTier(tool); got != probeTierProbe {
			t.Errorf("probeToolTier(%q) = %q, want %q (裁定扩表: 波形烘焙=真实渲染物理工作)", tool, got, probeTierProbe)
		}
	}
}

// 裁定扩表入账：clip.warm_waveform_bake 回执的 elapsed_ms 按 probe 级逐笔
// 入账 probeSpent，probeCostKnown 保持 true（两种名称形态同归一）。
func TestPullSessionWarmWaveformBakeMeteredAsProbe(t *testing.T) {
	session := meteredSession([]map[string]any{
		{"tool_call_id": "r1", "tool": "clip.warm_waveform_bake", "elapsed_ms": int64(40)},
		{"tool_call_id": "r2", "tool": "clip.warm.waveform.bake", "elapsed_ms": int64(2)},
	}, 8)
	session.settleBatch()
	if session.ledger.probeSpent != 42 {
		t.Errorf("probe spent = %v, want 42 (40+2 逐笔入账)", session.ledger.probeSpent)
	}
	if !session.ledger.probeCostKnown {
		t.Error("metered warm bake receipts must keep probeCostKnown=true (ruling-armed probe tier)")
	}
}

// E2E：executeTool 为每个真实执行的回执写 elapsed_ms 计量键（经真实
// MessageLoop/Runner/executeTool 链；值 int64 毫秒 ≥0）。
func TestExecuteToolWritesElapsedMeteringKey(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, nil)
	step, err := session.Interpret(context.Background(),
		`{"final":false,"reply":"work","tool_calls":[{"tool":"pull.echo","args":{}}]}`)
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	if _, err := session.Execute(context.Background(), step.Calls); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(session.state.executed) != 1 {
		t.Fatalf("executed records = %d, want 1", len(session.state.executed))
	}
	elapsed, ok := session.state.executed[0]["elapsed_ms"]
	if !ok {
		t.Fatal("execRecord missing elapsed_ms metering key")
	}
	ms, isInt := elapsed.(int64)
	if !isInt {
		t.Fatalf("elapsed_ms type = %T, want int64", elapsed)
	}
	if ms < 0 {
		t.Errorf("elapsed_ms = %d, want >=0", ms)
	}
}
