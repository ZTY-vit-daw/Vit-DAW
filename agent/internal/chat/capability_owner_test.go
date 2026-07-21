package chat

import (
	"context"
	"testing"

	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestCapabilityOwnerRolloutRoutesNaturalB2IntentToV1(t *testing.T) {
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT", "b2")
	server := &Server{orchestrationRuntime: orchestrationruntime.New()}
	response, handled := server.handleCapabilityRuntimeCanary(context.Background(), "chat-rollout", ChatRequest{
		Message: "请做 B2 静态平衡",
	}, agentruntime.Goal{})
	if !handled || response.Workflow != "capability_runtime_v1" {
		t.Fatalf("rollout did not claim new B2 Session: handled=%v response=%#v", handled, response)
	}
}

func TestCapabilityOwnerRemainsV1AfterLegacyRollbackToggleRetired(t *testing.T) {
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT", "off")
	server := &Server{orchestrationRuntime: orchestrationruntime.New()}
	resolution := server.resolveCapabilityOwner("chat-cutover", ChatRequest{Message: "B2 static balance"})
	if resolution.Decision.Owner != orchestration.EngineV1 || resolution.Decision.Conflict {
		t.Fatalf("retired legacy owner was selected: %#v", resolution)
	}
}

func TestProjectAwareCapabilityInferenceSupportsChineseProductIntent(t *testing.T) {
	if got := inferProjectAwareCapability("请先做整首工程的静态平衡"); got != staticBalanceCapabilityID {
		t.Fatalf("Chinese B2 intent routed to %q", got)
	}
	if got := inferProjectAwareCapability("重新安排所有轨道的声像布局"); got != panLayoutCapabilityID {
		t.Fatalf("Chinese B3 intent routed to %q", got)
	}
}

func TestProjectAwareCapabilityLeavesOrdinaryEQForAgentLoop(t *testing.T) {
	message := "在当前 TDR Nova 上做 92Hz、-2.5dB、Q 1.2 的静态 Bell EQ。"
	if got := inferProjectAwareCapability(message); got != "" {
		t.Fatalf("ordinary static EQ bypassed AgentLoop and routed to %q", got)
	}
	if got := inferProjectAwareCapability("帮我把低频浑浊减一点"); got != "" {
		t.Fatalf("generic EQ request bypassed AgentLoop: %q", got)
	}
	if got := inferProjectAwareCapability("我想把当前轨道上的均衡器低切打开"); got != "" {
		t.Fatalf("pass-filter request bypassed AgentLoop: %q", got)
	}
	if got := inferProjectAwareCapability("我想通过当前加载的均衡器调节3400Hz频段增益3dB"); got != "" {
		t.Fatalf("concrete EQ request bypassed AgentLoop: %q", got)
	}
}

func TestCapabilityOwnerHonorsExactRecoveredInteractionSession(t *testing.T) {
	runtime := orchestrationruntime.New()
	if _, err := runtime.StartPluginEffectControlChatSession("cap_v1_plugin_effect_replay_1", "chat-replay", "p1", "first", orchestration.InteractionPropose); err != nil {
		t.Fatal(err)
	}
	wanted, err := runtime.StartPluginEffectControlChatSession("cap_v1_plugin_effect_replay_2", "chat-replay", "p1", "second", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{orchestrationRuntime: runtime}
	resolution := server.resolveCapabilityOwner("chat-replay", ChatRequest{
		Message: "可以执行",
		Context: map[string]any{
			"capability_id":         pluginEffectControlCapabilityID,
			"capability_session_id": wanted.ID,
		},
	})
	if resolution.CapabilityID != pluginEffectControlCapabilityID || resolution.SessionID != wanted.ID || resolution.Decision.Owner != orchestration.EngineV1 {
		t.Fatalf("recovered interaction did not retain its exact session: %#v", resolution)
	}
}
func TestCapabilitySessionSequenceAdvancesAfterTerminalInvocation(t *testing.T) {
	runtime := orchestrationruntime.New()
	session, err := runtime.StartB2ChatSession("cap_v1_b2_chat-sequence_1", "chat-sequence", "p1", "B2", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	loaded, _ := runtime.Store.Load(session.ID)
	loaded.Status = orchestration.StatusCompleted
	loaded.Revision++
	if err := runtime.Store.Save(loaded, session.Revision); err != nil {
		t.Fatal(err)
	}
	server := &Server{orchestrationRuntime: runtime}
	if next := server.nextCapabilitySessionID("chat-sequence", staticBalanceCapabilityID); next != "cap_v1_b2_chat-sequence_2" {
		t.Fatalf("next Session ID = %q", next)
	}
	if active := server.activeCapabilitySessions("chat-sequence"); len(active) != 0 {
		t.Fatalf("terminal Session remained active: %#v", active)
	}
}

func TestV1PlanningSessionCanDrainByCancellationWithoutKernel(t *testing.T) {
	runtime := orchestrationruntime.New()
	session, err := runtime.StartB2ChatSession("cap_v1_b2_chat-cancel_1", "chat-cancel", "p1", "B2", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{orchestrationRuntime: runtime}
	response, handled := server.handleCapabilityRuntimeCanary(context.Background(), "chat-cancel", ChatRequest{Message: "取消这个方案"}, agentruntime.Goal{})
	if !handled || response.GoalStatus != string(agentruntime.StatusCancelled) {
		t.Fatalf("v1 cancellation was not handled without Kernel: handled=%v response=%#v", handled, response)
	}
	loaded, _ := runtime.Store.Load(session.ID)
	if loaded.Status != orchestration.StatusCancelled || loaded.EngineOwner != orchestration.EngineV1 {
		t.Fatalf("cancel did not preserve fixed owner/terminal state: %#v", loaded)
	}
}
