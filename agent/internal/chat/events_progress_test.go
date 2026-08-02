package chat

import (
	"strings"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

func TestAudioAnalysisToolEventShowsDADProgress(t *testing.T) {
	result := executorpkg.Result{
		Tool:        "project.audio_analysis_status",
		CommandName: "project.audio_analysis_status",
		Status:      "ok",
		Result: map[string]any{
			"analysis_job": map[string]any{
				"dad_fact_ready_count":   9,
				"dad_fact_total_count":   61,
				"dad_fact_pending_count": 52,
			},
		},
	}
	title := toolResultTitle(planner.ToolCall{Tool: "project.audio_analysis_status"}, result)
	body := toolResultBody(result, nil)
	if title != "素材分析 9/61" {
		t.Fatalf("title = %q", title)
	}
	if !strings.Contains(body, "9/61") || !strings.Contains(body, "52") || !strings.Contains(body, "B1") {
		t.Fatalf("body = %q", body)
	}
}
