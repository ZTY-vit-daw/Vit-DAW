package agentloop

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMixObservationPromptSummaryCarriesCompactCOMContext(t *testing.T) {
	result := map[string]any{
		"status": "ready", "observation_id": "obs-com-1", "mix_session_id": "mix-com-1",
		"observation": map[string]any{
			"com_projection": map[string]any{
				"schema_version": "com.projection.v1", "com_version": "v1", "projection_id": "com-1",
				"mode": "paired_io", "status": "ready", "observation_id": "obs-com-1",
				"trust_quality":           map[string]any{"overall_status": "ready", "can_support_behavior_observation": true},
				"llm_context":             map[string]any{"do_not_include_raw_package": true, "compact_facts": []any{map[string]any{"layer": "gain_action", "status": "ready"}}},
				"aligned_envelope_frames": []any{map[string]any{"raw": "must-not-survive"}},
			},
		},
	}
	summary := mixObservationPromptSummary(result)
	projection := messageLoopMapValue(summary["com_projection"])
	if firstMapText(projection, "projection_id") != "com-1" || firstMapText(projection, "mode") != "paired_io" {
		t.Fatalf("COM context missing: %+v", summary)
	}
	data, _ := json.Marshal(summary)
	if strings.Contains(string(data), "aligned_envelope_frames") || strings.Contains(string(data), "must-not-survive") {
		t.Fatalf("COM raw trace leaked to AgentLoop context: %s", data)
	}
}
