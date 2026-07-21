package chat

import (
	"context"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestAgentLoopExecutorBlocksImplicitPluginLearningFallback(t *testing.T) {
	executor := pluginGrabberWorkflowExecutor{}
	out, err := executor.RunToolCall(context.Background(), executorpkg.Input{
		ToolCall: planner.ToolCall{Tool: pluginGrabberLearnTool},
		Context:  map[string]any{"user_message": "我想通过当前加载的均衡器调节3400Hz频段增益3dB"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cleanContextText(out.Result["blocker"]) != "plugin_learning_requires_explicit_user_intent" || cleanContextText(out.Result["required_capability_tool"]) != equalizerCapabilityPlanTool {
		t.Fatalf("implicit learning was not blocked at the execution boundary: %#v", out)
	}
}

func TestAgentLoopRecognizesExplicitPluginLearningAsSeparateWorkflow(t *testing.T) {
	if !agentLoopExplicitPluginLearningRequest(executorpkg.Input{Context: map[string]any{"user_message": "请学习这个插件并保存常用控制"}}) {
		t.Fatal("explicit user-initiated learning request was not recognized")
	}
	if agentLoopExplicitPluginLearningRequest(executorpkg.Input{Context: map[string]any{"user_message": "把当前均衡器的3400Hz提高3dB"}}) {
		t.Fatal("ordinary EQ control was mistaken for user-initiated learning")
	}
}

func TestEqualizerPlanThinDelegatesCompleteRequestToB4(t *testing.T) {
	server := &Server{harness: harness.New(nil, nil, nil), interactions: make(map[string]PendingInteraction)}
	server.pluginEffectControlRuntimeOverride = func(_ context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
		apply := pluginEffectApplyArgs(req.Context)
		if cleanContextText(apply["track_id"]) != "1007" || cleanContextText(apply["plugin_id"]) != "1013" {
			t.Fatalf("delegated identity = %#v", apply)
		}
		if cleanContextText(apply["control"]) != "eq.cut_region" {
			t.Fatalf("delegated control = %#v", apply["control"])
		}
		target := mapValue(apply["target"])
		if target["freq_hz"] != 3400.0 || target["gain_db"] != -3.0 || target["q"] != 1.2 {
			t.Fatalf("delegated target = %#v", target)
		}
		if _, exists := apply["band_ref"]; exists {
			t.Fatalf("SPAL band_ref leaked into B4 request: %#v", apply)
		}
		return pluginEffectProposalTestResponse(conversationID, goal)
	}
	out, err := server.invokeEqualizerCapabilityTool(context.Background(), executorpkg.Input{
		GoalID: "goal_eq", RunID: "run_eq",
		ToolCall: planner.ToolCall{ID: "call_eq", Tool: equalizerCapabilityPlanTool, Args: map[string]any{
			"task": "spectral_region_adjust", "track_id": "1007", "plugin_id": "1013", "frequency_hz": 3400.0, "gain_db": -3.0, "q": 1.2,
		}},
		Context: map[string]any{"conversation_id": "chat_eq", "user_message": "cut harshness"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cleanContextText(out.Result["execution_route"]) != pluginEffectControlCapabilityID || out.Result["plugin_learning_fallback"] != nil {
		t.Fatalf("equalizer compatibility payload = %#v", out.Result)
	}
}

func TestEqualizerPlanNoLongerRequiresBandRefOrCredential(t *testing.T) {
	plan := buildEqualizerPluginEffectPlan(map[string]any{
		"task": "spectral_region_adjust", "track_id": "1007", "plugin_id": "1013", "frequency_hz": 200.0, "gain_db": 2.0, "q": 0.8,
	}, nil)
	if len(plan.MissingSemantic) != 0 || len(plan.MissingTechnical) != 0 {
		t.Fatalf("unexpected B4 gaps: %#v", plan)
	}
	if cleanContextText(plan.ApplyArgs["control"]) != "eq.boost_region" {
		t.Fatalf("control = %#v", plan.ApplyArgs)
	}
}
