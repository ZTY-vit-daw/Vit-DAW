package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/planner"
)

// PLUGIN-CONTROL-EVAL-BAREFREE-1 (card 318) paired seam tests. The B-R1
// payload dissection proved the T1 write gate blocks plugin.get_parameters
// and plugin.set_parameter under a natural mixing request after
// mix.observe ("mix.observe is complete; broad mixing requests must stop
// here, ..." — the exact tool_gate reason the model saw). These tests pin
// both directions:
//  1. seam unset / non-"1" → the gate keeps blocking the plugin parameter
//     read/write victims byte-for-byte (production behavior unchanged);
//  2. seam exactly "1" → the same calls pass the gate;
//  3. the open seam exempts ONLY plugin parameter read/write — plugin
//     loading and daw.invoke stay gated, so the barewrite arm measures
//     raw-write capability, not an ungated loop.

const barefreeT1UserText = "请把 Pro-R 混响的 Mix 设为 35%。"

func barefreeT1State() *runState {
	return &runState{input: Input{UserText: barefreeT1UserText}}
}

func barefreeParamWriteCall() planner.ToolCall {
	return planner.ToolCall{Tool: "plugin.set_parameter", Args: map[string]any{
		"track_id": "1138", "plugin_id": "1144", "param_id": "1", "value": 0.7}}
}

func barefreeParamReadCall() planner.ToolCall {
	return planner.ToolCall{Tool: "plugin.get_parameters", Args: map[string]any{
		"track_id": "1138", "plugin_id": "1144"}}
}

const barefreeT1GateReason = "mix.observe is complete; broad mixing requests must stop here, summarize the observation, propose one concrete next move, and wait for explicit user confirmation before loading plugins, changing volume, applying controls, or writing parameters"

func TestBarefreeSeamClosedKeepsT1GateBlockingParamWrite(t *testing.T) {
	state := barefreeT1State()
	if !messageLoopNaturalMixRequest(state.input.UserText) {
		t.Fatal("test premise: T1 user text must classify as a natural mixing request")
	}
	if got := messageLoopToolGuardIssue(state, barefreeParamWriteCall(), true); got != barefreeT1GateReason {
		t.Fatalf("closed seam must keep the T1 write gate; got %q", got)
	}
	if got := messageLoopToolGuardIssue(state, barefreeParamReadCall(), true); got != barefreeT1GateReason {
		t.Fatalf("closed seam must keep the T1 read gate (plugin.get_parameters was the payload-proven victim); got %q", got)
	}
}

func TestBarefreeSeamOpenPassesParamReadWriteThroughT1Gate(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBareWriteEnv, "1")
	state := barefreeT1State()
	if got := messageLoopToolGuardIssue(state, barefreeParamWriteCall(), true); got != "" {
		t.Fatalf("open seam must pass plugin.set_parameter; got %q", got)
	}
	if got := messageLoopToolGuardIssue(state, barefreeParamReadCall(), true); got != "" {
		t.Fatalf("open seam must pass plugin.get_parameters; got %q", got)
	}
	// Command-name variants resolve through the same matcher.
	for _, tool := range []string{"set_plugin_param", "plugin_set_parameter", "get_plugin_parameters"} {
		call := planner.ToolCall{Tool: tool}
		if got := messageLoopToolGuardIssue(state, call, true); got != "" {
			t.Fatalf("open seam must pass variant %q; got %q", tool, got)
		}
	}
}

func TestBarefreeSeamOpenStillGatesNonParamTools(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBareWriteEnv, "1")
	state := barefreeT1State()
	pluginLoad := planner.ToolCall{Tool: "plugin.load_to_rack"}
	if got := messageLoopToolGuardIssue(state, pluginLoad, true); got == "" {
		t.Fatal("open seam must keep plugin loading gated by the observe-first requirement")
	}
	dawInvoke := planner.ToolCall{Tool: "daw.invoke"}
	if got := messageLoopToolGuardIssue(state, dawInvoke, true); got == "" {
		t.Fatal("open seam must keep daw.invoke gated by the observe-first requirement")
	}
	trackVolume := planner.ToolCall{Tool: "track.volume"}
	if got := messageLoopToolGuardIssue(state, trackVolume, true); got == "" ||
		!strings.Contains(got, "direct track volume or pan writes") {
		t.Fatalf("open seam must keep the track volume/pan gate; got %q", got)
	}
}

func TestBarefreeSeamOnlyExactValueOpens(t *testing.T) {
	for _, value := range []string{"", "0", "true", "off", "1x"} {
		t.Setenv(pluginControlEvalSeamBareWriteEnv, value)
		if pluginControlEvalSeamBareWriteOpen() {
			t.Fatalf("value %q must not open the seam (only exactly \"1\")", value)
		}
		got := messageLoopToolGuardIssue(barefreeT1State(), barefreeParamWriteCall(), true)
		if got != barefreeT1GateReason {
			t.Fatalf("value %q must keep the gate closed; got %q", value, got)
		}
	}
	t.Setenv(pluginControlEvalSeamBareWriteEnv, "1")
	if !pluginControlEvalSeamBareWriteOpen() {
		t.Fatal("exactly \"1\" must open the seam")
	}
}

func TestBarefreeParamReadWriteToolMatcher(t *testing.T) {
	positive := []string{"plugin.get_parameters", "plugin_get_parameters", "get_plugin_parameters",
		"get_plugin_param", "plugin.set_parameter", "plugin_set_parameter", "set_plugin_param", "set_plugin_parameter"}
	for _, tool := range positive {
		if !messageLoopIsPluginParameterReadWriteTool(planner.ToolCall{Tool: tool}) {
			t.Fatalf("%q must match the plugin parameter read/write family", tool)
		}
	}
	negative := []string{"plugin.load_to_rack", "track.volume", "daw.invoke", "plugin_grabber.apply_eq_edits", "mix.observe"}
	for _, tool := range negative {
		if messageLoopIsPluginParameterReadWriteTool(planner.ToolCall{Tool: tool}) {
			t.Fatalf("%q must not match the plugin parameter read/write family", tool)
		}
	}
}
