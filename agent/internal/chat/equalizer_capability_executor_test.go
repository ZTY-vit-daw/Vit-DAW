package chat

import (
	"context"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
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
