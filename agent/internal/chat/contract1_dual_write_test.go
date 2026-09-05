package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/logx"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

func newTestLogger(path string) *logx.Logger {
	return logx.New(false, path, 64)
}

func readTestLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return string(data)
}

// CONTRACT-1 RED①：C0 双写——run 级与实验级事件上都带新映射字段，且旧
// turn_id 语义与值不动（放行条件 3：增量双读/双写，旧字段择期废弃）。
// 轨迹双域实证（2026-09-05 取证）：run 级 turn 用裸 run_id，实验 turn 用
// turn:free_state_* 前缀域，UI 锚定只能精确匹配——source_turn_id 提供跨域
// join key，trajectory_turn_id 是轨迹归属权威。
func TestDualWriteTurnIdentityOnRunLevelEvents(t *testing.T) {
	s := testContinuationServer()
	s.harness = harness.NewWithSender(nil, nil, nil)
	s.harness.EnsureGoal("goal-c1", "run-c1", "检查当前工程有什么问题")

	s.emitChatTurnTrajectoryStarted("conversation-c1", "goal-c1", "run-c1")
	s.emitChatTurnTrajectoryTerminal("conversation-c1", "goal-c1", "run-c1", trajectory.StatusCompleted, "")

	events, _ := s.agentEventsSince("conversation-c1", 0, 64)
	if len(events) == 0 {
		t.Fatal("run-level turn trajectory events missing")
	}
	for _, event := range events {
		if event.TurnID != "run-c1" {
			t.Fatalf("legacy turn_id must stay untouched, got %q", event.TurnID)
		}
		if event.TrajectoryTurnID != "run-c1" {
			t.Fatalf("run-level event trajectory_turn_id: want run-c1, got %q", event.TrajectoryTurnID)
		}
		if event.SourceTurnID != "run-c1" {
			t.Fatalf("run-level event source_turn_id: want run-c1, got %q", event.SourceTurnID)
		}
	}
}

func TestDualWriteTurnIdentityOnExperimentLevelEvents(t *testing.T) {
	s := testContinuationServer()
	event := trajectory.Event{
		Type: trajectory.EventIntentFramed, ConversationID: "conversation-c1x",
		GoalID: "goal-c1x", RunID: "run-c1x", ItemID: "turn:free_state_c1x:intent",
		Payload: trajectory.Payload{
			SchemaVersion: trajectory.SchemaVersion,
			TurnID:        "turn:free_state_c1x", TraceNodeID: "turn:free_state_c1x:intent",
			NodeKind: trajectory.NodeIntent, Status: trajectory.StatusCompleted,
		},
	}
	normalized := event.Normalize()
	if normalized.Payload.TrajectoryTurnID != "turn:free_state_c1x" {
		t.Fatalf("experiment payload trajectory_turn_id: want turn:free_state_c1x, got %q", normalized.Payload.TrajectoryTurnID)
	}
	if normalized.Payload.SourceTurnID != "run-c1x" {
		t.Fatalf("experiment payload source_turn_id: want run-c1x (the owning run), got %q", normalized.Payload.SourceTurnID)
	}

	emitted, err := s.emitTrajectoryEvent("conversation-c1x", event)
	if err != nil {
		t.Fatalf("emitTrajectoryEvent: %v", err)
	}
	if emitted.TurnID != "turn:free_state_c1x" {
		t.Fatalf("legacy turn_id must stay untouched, got %q", emitted.TurnID)
	}
	if emitted.TrajectoryTurnID != "turn:free_state_c1x" {
		t.Fatalf("experiment event trajectory_turn_id: want turn:free_state_c1x, got %q", emitted.TrajectoryTurnID)
	}
	if emitted.SourceTurnID != "run-c1x" {
		t.Fatalf("experiment event source_turn_id: want run-c1x (cross-domain join key), got %q", emitted.SourceTurnID)
	}
}

// CONTRACT-1 RED②：调度链收尾切片（settle slice）的 turn.completed 由服务端
// 显式标记 turn_kind=settle_slice——替代 UI 猜测式隐藏（M12 定案推荐方向）。
func TestSchedulerChainResultEventMarksSettleSlice(t *testing.T) {
	s := testContinuationServer()
	s.harness = harness.NewWithSender(nil, nil, nil)
	s.harness.EnsureGoal("goal-c1s", "run-c1s", "检查当前工程有什么问题")
	s.emitChatTurnTrajectoryStarted("conversation-c1s", "goal-c1s", "run-c1s")

	s.emitSchedulerChainResultEvent("conversation-c1s", ChatResponse{
		ConversationID: "conversation-c1s", GoalID: "goal-c1s", RunID: "run-c1s",
		GoalStatus: string(agentruntime.StatusCompleted), Reply: "本轮观察结束。",
	}, nil)

	events, _ := s.agentEventsSince("conversation-c1s", 0, 64)
	var marked bool
	for _, event := range events {
		if event.Type != "turn.completed" || event.ItemID != "chain_result" {
			continue
		}
		if kind, _ := event.Payload["turn_kind"].(string); kind != "settle_slice" {
			t.Fatalf("chain settle slice turn.completed must carry turn_kind=settle_slice, got %q", kind)
		}
		marked = true
	}
	if !marked {
		t.Fatal("chain settle slice turn.completed missing")
	}
}

// CONTRACT-1 RED③：scheduler_chain 的 turn.stopped 终局必须带 scheduler_chain
// 标记与非空终局 body（CONTRACT-2 已知缺口：UI 从未提取 stopped——服务端先
// 保证事件形态完整，UI 消费归 GUI-1）。
func TestSchedulerChainResultEventStoppedTerminalComplete(t *testing.T) {
	s := testContinuationServer()
	s.harness = harness.NewWithSender(nil, nil, nil)
	s.harness.EnsureGoal("goal-c1t", "run-c1t", "检查当前工程有什么问题")
	s.emitChatTurnTrajectoryStarted("conversation-c1t", "goal-c1t", "run-c1t")

	s.emitSchedulerChainResultEvent("conversation-c1t", ChatResponse{
		ConversationID: "conversation-c1t", GoalID: "goal-c1t", RunID: "run-c1t",
		GoalStatus: string(agentruntime.StatusStopped),
	}, nil)

	events, _ := s.agentEventsSince("conversation-c1t", 0, 64)
	var sawStopped bool
	for _, event := range events {
		if event.Type != "turn.stopped" {
			continue
		}
		sawStopped = true
		if marked, _ := event.Payload["scheduler_chain"].(bool); !marked {
			t.Fatalf("stopped terminal must carry the scheduler_chain marker: %+v", event.Payload)
		}
		if strings.TrimSpace(event.Body) == "" {
			t.Fatal("stopped terminal must carry a non-empty terminal body")
		}
	}
	if !sawStopped {
		t.Fatal("stopped chain end did not emit turn.stopped")
	}
}

// CONTRACT-1 RED⑤：按域路由开关默认全旧路径（放行条件 2：服务端、按域、
// 可观测、预定义移除条件；AGENT-1 的 static_eq/broadband_compression 迁移
// 铺轨）。
func TestDomainRouteDefaultsToLegacyPath(t *testing.T) {
	t.Setenv(domainRoutingEnvName, "")
	s := testContinuationServer()
	for _, domain := range []string{"static_eq", "broadband_compression", "track_gain", "pan", "unknown_domain"} {
		if route := s.domainRouteFor(domain); route != DomainRouteLegacyNative {
			t.Fatalf("domain %q must default to %s, got %q", domain, DomainRouteLegacyNative, route)
		}
	}
}

func TestDomainRouteOverridesPerDomain(t *testing.T) {
	s := testContinuationServer()
	t.Setenv(domainRoutingEnvName, "static_eq=processor_selection")
	if route := s.domainRouteFor("static_eq"); route != DomainRouteProcessorSelection {
		t.Fatalf("static_eq override: want processor_selection, got %q", route)
	}
	if route := s.domainRouteFor("broadband_compression"); route != DomainRouteLegacyNative {
		t.Fatalf("broadband_compression must stay legacy unless listed, got %q", route)
	}
	// 非法路由值不得放行——回落旧路径。
	t.Setenv(domainRoutingEnvName, "static_eq=processor_selection,broadband_compression=yolo")
	if route := s.domainRouteFor("broadband_compression"); route != DomainRouteLegacyNative {
		t.Fatalf("invalid route value must fall back to legacy, got %q", route)
	}
}

// 开关可观测性要件：每次路由决策记录实际路径到运行日志。
func TestDomainRouteDecisionIsLogged(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "domain-route.log")
	s := testContinuationServer()
	s.logger = newTestLogger(logPath)
	t.Setenv(domainRoutingEnvName, "static_eq=processor_selection")

	_ = s.domainRouteFor("static_eq")
	_ = s.domainRouteFor("broadband_compression")

	logged := readTestLog(t, logPath)
	for _, want := range []string{
		`domain=static_eq route=processor_selection`,
		`domain=broadband_compression route=legacy_native`,
	} {
		if !strings.Contains(logged, want) {
			t.Fatalf("route decision log must contain %q, log:\n%s", want, logged)
		}
	}
}
