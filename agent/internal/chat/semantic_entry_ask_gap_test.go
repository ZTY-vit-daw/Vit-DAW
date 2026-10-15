package chat

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

// FIX-ASK-SILENT-GAP-1 ①：语义入口分类器的一次 LLM 调用（含重试环）必须在
// 短预算内收敛——退化服务（挂死请求）下限时失败返回，走 fail-open，而不是
// 独占 60s×2+退避 ≈ 121s（M2 现场 122s 静默根因）。注入式假客户端 +
// 短超时，不真烧长时限。
func TestPlanSemanticEntryConvergesWithinBudgetUnderDegradedLLM(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_TELEMETRY_PATH", "disabled")
	original := semanticEntryRequestBudget
	semanticEntryRequestBudget = 300 * time.Millisecond
	defer func() { semanticEntryRequestBudget = original }()

	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_uuid": "project-asg1", "project_revision": "rev-asg1"})
	server := New(nil, project, nil)
	server.llm = &llm.Client{HTTPClient: &http.Client{Transport: hangingLLMTransport{}}}
	cfg := config.EngineConfig{BaseURL: "http://llm.invalid/v1", APIKey: "test-key", DefaultModel: "test-model"}

	started := time.Now()
	_, err := server.planSemanticEntry(context.Background(), "conversation-asg1-budget", "帮我把整体听感调得开阔一些", nil, cfg)
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("degraded LLM must fail the semantic entry request")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("semantic entry must converge within its request budget (300ms), took %s", elapsed)
	}
}

// hangingLLMTransport simulates the M2 degraded service: every attempt hangs
// until its context is done.
type hangingLLMTransport struct{}

func (hangingLLMTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, context.DeadlineExceeded
}

// FIX-ASK-SILENT-GAP-1 ③：入口分类失败（fail-open）时空工程仍要走本地容量
// 事实短路——秒级直接作答并声明入口分类不可用，不再被放进 slice 管线
// （M2 现场 0 轨工程在 122s 静默后才进管线）。经 runAgentLoopChat 真实分类块
// 端到端驱动（llm 置空 = 分类器不可用）。
func TestSemanticEntryFailureShortCircuitsEmptyProject(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_uuid": "project-asg1-empty", "project_revision": "rev-asg1-empty"})
	server := New(nil, project, nil)

	resp, handled := server.runAgentLoopChat(context.Background(), "conversation-asg1-empty", ChatRequest{
		Message: "帮我把整体听感调得开阔一些",
	}, config.EngineConfig{})
	if !handled {
		t.Fatal("empty-project ask under entry failure must be handled by the capacity short-circuit")
	}
	if resp.StopReason != "capacity_fact_short_circuit" {
		t.Fatalf("empty project must short-circuit instead of entering the slice pipeline: stop=%s", resp.StopReason)
	}
	if !strings.Contains(resp.Reply, "还没有任何轨道") {
		t.Fatalf("short-circuit reply must state the observed fact: %q", resp.Reply)
	}
	if !strings.Contains(resp.Reply, "意图分类服务本次不可用") {
		t.Fatalf("short-circuit reply must disclose the unavailable entry classifier: %q", resp.Reply)
	}
	if resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("short-circuit must settle the goal honestly: %s", resp.GoalStatus)
	}
	if resp.WorkflowData["semantic_entry_unavailable"] != true {
		t.Fatalf("workflow data must record the entry failure: %+v", resp.WorkflowData)
	}
	if resp.GoalID == "" || server.harness.RuntimeStatus(resp.GoalID).Status != agentruntime.StatusCompleted {
		t.Fatalf("short-circuit must complete its routing goal: %s", resp.GoalID)
	}
}

// ④：分类失败时 ask 受理忙碌事件必须收口为失败态事件（用户可见），而不是
// 悬挂成永久 spinner。
func TestSemanticEntryFailureSettlesAskActivityEvent(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_uuid": "project-asg1-ev", "project_revision": "rev-asg1-ev"})
	server := New(nil, project, nil)
	conversationID := "conversation-asg1-events"
	server.emitSemanticEntryActivityStarted(conversationID)

	_, handled := server.runAgentLoopChat(context.Background(), conversationID, ChatRequest{
		Message: "帮我把整体听感调得开阔一些",
	}, config.EngineConfig{})
	if !handled {
		t.Fatal("empty-project ask under entry failure must be handled")
	}
	events, _ := server.agentEventsSince(conversationID, 0, 50)
	started := false
	settled := false
	for _, event := range events {
		if event.ItemID != semanticEntryActivityItemID {
			continue
		}
		switch event.Type {
		case "item.started":
			started = true
			if event.Status != "running" {
				t.Fatalf("ask activity must start running: %+v", event)
			}
		case "item.completed":
			settled = true
			if event.Status != "failed" {
				t.Fatalf("entry failure must settle the ask activity as failed: %+v", event)
			}
		}
	}
	if !started || !settled {
		t.Fatalf("ask activity lifecycle events missing: started=%v settled=%v (%d events)", started, settled, len(events))
	}
}

// ③负例：有轨工程上入口失败不做容量短路（诊断意图进既有 fail-open 管线）；
// revision 未绑定时不得断言空工程（诚实边界保持）。
func TestSemanticEntryFailureSparesTrackedAndUnobservedProjects(t *testing.T) {
	tracked := shadow.New(nil)
	tracked.Initialize(map[string]any{
		"project_uuid": "project-asg1-6", "project_revision": "rev-asg1-6", "track_count": 6,
		"tracks": []any{map[string]any{"track_id": "t1", "is_audio_track": true}},
	})
	server := New(nil, tracked, nil)
	if _, ok := server.semanticEntryUnavailableCapacityAnswer(context.Background(), "conversation-asg1-6", "chat",
		"帮我把整体听感调得开阔一些", nil); ok {
		t.Fatal("tracked project with a diagnostic request must not be short-circuited")
	}

	unobserved := shadow.New(nil)
	unobserved.Initialize(map[string]any{"project_uuid": "project-asg1-unknown"})
	serverUnobserved := New(nil, unobserved, nil)
	if _, ok := serverUnobserved.semanticEntryUnavailableCapacityAnswer(context.Background(), "conversation-asg1-unknown", "chat",
		"帮我把整体听感调得开阔一些", nil); ok {
		t.Fatal("unobserved structure (no bound revision) must not be asserted empty")
	}
}
