package agentloop

import (
	"testing"

	"vit-daw-agent/internal/planner"
)

func TestEqualizerCapabilityPlanIsNotBlockedByBroadMixObservationGuard(t *testing.T) {
	state := &runState{input: Input{UserText: "我想通过当前加载的均衡器调节3400Hz频段增益3dB"}}
	call := planner.ToolCall{Tool: "capability.equalizer.plan", Args: map[string]any{"task": "spectral_region_adjust", "frequency_hz": 3400.0, "gain_db": 3.0}}
	if issue := messageLoopToolGuardIssue(state, call, false); issue != "" {
		t.Fatalf("equalizer capability tool was blocked before the Agent could query the capability layer: %s", issue)
	}
}
