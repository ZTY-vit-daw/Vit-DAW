package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/taskstate"
)

func shortCircuitAssessment(trackCount, activeTracks int, scope, revision string) FreeStateCapacityAssessment {
	facts := CapacityObservedFacts{
		TrackCount: trackCount, ActiveAudioTrackCount: activeTracks,
		PluginCount: 4, ProjectRevision: revision, ProjectUUID: "project-fact",
		RequestScope: scope, EvidenceRefs: []string{"project.state:project-fact:" + revision},
	}
	return assessFreeStateCapacity(facts, false, time.Now().UTC())
}

// 测试①（路由面）：空工程 + 事实性问句 → 短路直答，不指派闭包控制器、
// 不开切片。
func TestCapacityShortCircuitEmptyProjectAnswersDirectly(t *testing.T) {
	assessment := shortCircuitAssessment(0, 0, semanticEntryScopeProjectContext, "rev-1")
	circuit := capacityFactShortCircuit("检查当前工程有多少轨道", assessment)
	if circuit == nil {
		t.Fatal("empty project must short-circuit the closure route")
	}
	if circuit.Kind != "empty_project" || !strings.Contains(circuit.Answer, "轨道") {
		t.Fatalf("empty project answer is not an honest fact answer: %+v", circuit)
	}
	if len(circuit.EvidenceRefs) == 0 {
		t.Fatalf("empty project answer must cite the observed state: %+v", circuit)
	}
}

// 测试①（端到端路由）：空工程上走 planObservationFirstCapabilityRoute 的
// 真实观察路径，record 带短路标记、entry 无控制器、无闭包准入。
func TestRoutePlanningShortCircuitsEmptyProjectObservation(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_uuid": "project-empty", "project_revision": "rev-empty"})
	server := New(nil, project, nil)
	request := "检查当前工程有多少轨道"
	contextWithTask, identity := server.ensureCapabilityRoutingTask(request, nil)
	entry := semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteObservation,
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlObserveOnly,
		UserAuthorization: semanticEntryAuthorizationObserve, Confidence: 0.95, Reason: "count question",
	}
	entry, record, err := server.planObservationFirstCapabilityRoute(context.Background(), "conversation-f3-empty", request, contextWithTask, entry, identity)
	if err != nil {
		t.Fatal(err)
	}
	if record.ShortCircuit == nil || record.ShortCircuit.Kind != "empty_project" {
		t.Fatalf("empty project observation must short-circuit: %+v", record)
	}
	if entry.Controller != "" || record.Controller != "" {
		t.Fatalf("short-circuited route must not own a closure controller: entry=%+v record=%+v", entry, record)
	}
	if record.Assessment == nil || record.Assessment.ObservedFacts.TrackCount != 0 {
		t.Fatalf("short-circuit must ride on observed facts: %+v", record.Assessment)
	}

	resp := server.capacityFactShortCircuitResponse("conversation-f3-empty", "chat", record)
	if resp.GoalStatus != string(agentruntime.StatusCompleted) || resp.StopReason != "capacity_fact_short_circuit" {
		t.Fatalf("short-circuit response must settle the goal honestly: %+v", resp)
	}
	if !strings.Contains(resp.Reply, "轨道") || strings.Contains(resp.Reply, "上限") {
		t.Fatalf("short-circuit reply must be the fact answer: %q", resp.Reply)
	}
	goal := server.harness.RuntimeStatus(record.GoalID)
	if goal.Status != agentruntime.StatusCompleted {
		t.Fatalf("short-circuit must complete the routing goal: %s", goal.Status)
	}
	if goal.Task.Run.CurrentSliceID != "" || len(goal.Task.Run.Slices) != 0 {
		t.Fatalf("short-circuit must not open any slice: %+v", goal.Task.Run)
	}
}

// 测试②：6 轨工程的事实问句回归——直接以观察到的事实作答。
func TestCapacityShortCircuitFactQuestionOnNonEmptyProject(t *testing.T) {
	assessment := shortCircuitAssessment(6, 6, semanticEntryScopeProjectContext, "rev-6")
	circuit := capacityFactShortCircuit("检查当前工程有多少轨道", assessment)
	if circuit == nil || circuit.Kind != "fact_question" {
		t.Fatalf("six-track fact question must answer from observed facts: %+v", circuit)
	}
	if !strings.Contains(circuit.Answer, "6") {
		t.Fatalf("answer must state the observed count: %q", circuit.Answer)
	}
}

// 测试③：真诊断意图不得被短路（防短路过宽）。
func TestCapacityShortCircuitSparesDiagnosticIntent(t *testing.T) {
	assessment := shortCircuitAssessment(6, 6, semanticEntryScopeProjectContext, "rev-6")
	for _, request := range []string{
		"检查一下当前工程有什么混音问题",
		"有几轨的动态起伏偏大，请给一个有界的小步改进建议",
		"帮我检查并改善当前工程",
	} {
		if circuit := capacityFactShortCircuit(request, assessment); circuit != nil {
			t.Fatalf("diagnostic/improvement request %q must still route into the closure machine: %+v", request, circuit)
		}
	}
}

// 测试③补充：观察不可信（revision 未绑定）时不得断言工程为空。
func TestCapacityShortCircuitRequiresBoundFacts(t *testing.T) {
	assessment := shortCircuitAssessment(0, 0, semanticEntryScopeProjectContext, "unknown")
	if circuit := capacityFactShortCircuit("检查当前工程有多少轨道", assessment); circuit != nil {
		t.Fatalf("unobserved structure must not be asserted empty: %+v", circuit)
	}
}

// 测试④：对话任务 limit 边界的用户可见 Reply 不含"思考步数已到上限"，
// 换中性收尾；runner 侧语义字段原样透传。无续跑的 waiting_continue 会被
// 上游诚实转为 failed（durable_continuation_missing），预算文案同样不得
// 泄漏到该错误回复。
func TestLimitBoundaryReplyIsNeutralForConversationalTurns(t *testing.T) {
	server := &Server{harness: harness.NewWithSender(nil, nil, nil)}
	t.Run("chain continues", func(t *testing.T) {
		res := agentloop.Result{
			Reply: "本轮思考步数已到上限；任务会从已保存的检查点自动继续。",
			Status: agentruntime.StatusWaitingContinue, StopReason: agentloop.StopReasonLimitReached,
			LimitType: agentloop.LimitTypeTurns, RunID: "run-f3", SliceID: "slice-f3", TurnID: "turn-f3",
			Continuation: &agentloop.Continuation{RunID: "run-f3", SliceID: "slice-f3"},
		}
		resp := server.chatResponseFromAgentLoopResult("conversation-f3-limit", "chat", res)
		if strings.Contains(resp.Reply, "思考步数已到上限") || strings.Contains(resp.Reply, "检查点") {
			t.Fatalf("conversational limit boundary leaked the runner budget text: %q", resp.Reply)
		}
		if !strings.Contains(resp.Reply, "继续处理") {
			t.Fatalf("neutral reply must tell the user work continues: %q", resp.Reply)
		}
		if resp.StopReason != agentloop.StopReasonLimitReached || resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
			t.Fatalf("runner boundary semantics must pass through unchanged: %+v", resp)
		}
	})
	t.Run("checkpoint missing converts honestly", func(t *testing.T) {
		res := agentloop.Result{
			Reply: "本轮思考步数已到上限；任务会从已保存的检查点自动继续。",
			Status: agentruntime.StatusWaitingContinue, StopReason: agentloop.StopReasonLimitReached,
			LimitType: agentloop.LimitTypeTurns, RunID: "run-f3", SliceID: "slice-f3", TurnID: "turn-f3",
		}
		resp := server.chatResponseFromAgentLoopResult("conversation-f3-limit", "chat", res)
		if strings.Contains(resp.Reply, "思考步数已到上限") {
			t.Fatalf("budget text must not leak through the honest failure path: %q", resp.Reply)
		}
		if resp.GoalStatus != string(agentruntime.StatusFailed) || resp.StopReason != "durable_continuation_missing" {
			t.Fatalf("missing checkpoint must convert to the honest failure: %+v", resp)
		}
	})
}

// 测试⑥：续跑完成边界孤儿收口——链条停止（无活续跑持有者）时任务语义收
// closed、goal 落 completed、running 切片置终态；有后续切片时不动。
func TestContinuationEndGuardClosesOrphanTaskAndSettlesSlices(t *testing.T) {
	s, goal, _ := orphanTaskServer(t)
	if _, slice, ok := s.harness.Runtime().BeginSlice(goal.GoalID, 1, 1); !ok {
		t.Fatal("slice was not opened")
	} else if slice.Status != "running" {
		t.Fatalf("slice must start running: %+v", slice)
	}
	s.durableContinuations = map[string]DurableContinuation{
		"cont-f3": {ContinuationID: "cont-f3", TaskID: goal.Task.TaskID, GoalID: goal.GoalID, RunID: goal.RunID,
			ConversationID: "conversation-orphan", OriginalIntent: goal.Task.OriginalIntent, Status: ContinuationCompleted},
	}

	s.settleGoalAfterContinuationEnd("conversation-orphan", goal.GoalID)

	after := s.harness.RuntimeStatus(goal.GoalID)
	if after.Task == nil || after.Task.SemanticState == nil {
		t.Fatalf("task disappeared at continuation end: %+v", after)
	}
	if state := after.Task.SemanticState; state.State != taskstate.StateClosed || !state.Terminal {
		t.Fatalf("orphan observation task was not closed at continuation end: %+v", state)
	}
	if after.Status != agentruntime.StatusCompleted {
		t.Fatalf("goal must settle completed when the chain stops: %s", after.Status)
	}
	if len(after.Task.Run.Slices) != 1 || after.Task.Run.Slices[0].Status != "settled" || after.Task.Run.Slices[0].EndedAt.IsZero() {
		t.Fatalf("running slice must finalize with the closed task: %+v", after.Task.Run.Slices)
	}
}

// 测试⑥补充：链条还有活续跑（下一片 pending）或待答交互时，完成边界守卫
// 不得收口。
func TestContinuationEndGuardSparesLiveContinuationOwners(t *testing.T) {
	s, goal, _ := orphanTaskServer(t)
	s.durableContinuations = map[string]DurableContinuation{
		"cont-f3-next": {ContinuationID: "cont-f3-next", TaskID: goal.Task.TaskID, GoalID: goal.GoalID, RunID: goal.RunID,
			ConversationID: "conversation-orphan", OriginalIntent: goal.Task.OriginalIntent, Status: ContinuationPending},
	}

	s.settleGoalAfterContinuationEnd("conversation-orphan", goal.GoalID)

	after := s.harness.RuntimeStatus(goal.GoalID)
	if after.Task.SemanticState.Terminal {
		t.Fatalf("task with a pending next slice must not be closed: %+v", after.Task.SemanticState)
	}
	if after.Status == agentruntime.StatusCompleted {
		t.Fatalf("goal with live continuation owner must not settle: %s", after.Status)
	}
}
