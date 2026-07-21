package chat

import (
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

func TestSelectedPluginEQRequestBlocksImplicitProviderFallbackLoad(t *testing.T) {
	loadTDR := planner.ToolCall{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{"plugin_query": "TDR Nova"},
	}
	for _, tc := range []struct {
		name string
		text string
		want bool
	}{
		{name: "selected Chinese low-cut request", text: "帮我把现在加载的均衡的低切打开", want: true},
		{name: "selected English EQ request", text: "enable a high-pass filter on the selected EQ", want: true},
		{name: "explicit TDR load remains allowed", text: "请加载 TDR Nova 后打开低切", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := agentLoopSelectedPluginEQProviderFallbackLoadBlocked(executorpkg.Input{
				ToolCall: loadTDR,
				Context: map[string]any{
					"selected_plugin_id": "proq3-instance",
					"user_message":       tc.text,
				},
			})
			if got != tc.want {
				t.Fatalf("fallback-load block = %v, want %v for %q", got, tc.want, tc.text)
			}
		})
	}
}
