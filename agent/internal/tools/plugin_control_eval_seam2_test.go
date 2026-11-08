package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

// PLUGIN-CONTROL-EVAL-SEAM2-1 S5 (tool-catalog trimming), B4/C4
// ablation-hook test pattern (305 seam tests same style): default unset
// keeps the production model catalog byte-for-byte (plugin_grabber typed
// adapters on the menu); VIT_PLUGIN_CONTROL_EVAL_SEAM=1 trims the
// plugin_grabber family from the model catalog faces while the bare
// parameter tools and the execution-facing specs stay; only the exact
// value "1" arms the hook.

// seamS5BlueprintTools are the plugin_grabber call forms the 307 A-arm
// observed as write_path=typed_adapter (arm_a_summary.json, 10/18 rounds),
// used here as the trimming-surface blueprint.
var seamS5BlueprintTools = []string{
	"plugin_grabber.inspect_reverb",
	"plugin_grabber_inspect_reverb",
	"plugin_grabber.apply_reverb_controls",
	"plugin_grabber_apply_reverb_controls",
	"plugin_grabber.apply_delay_controls",
	"plugin_grabber_apply_delay_controls",
	"plugin_grabber.explain_controls",
	"plugin_grabber_explain_controls",
	"plugin_explain_controls",
	"plugin.explain_controls",
}

// seamS5BareParameterTools: menu forms (tool name + alias) of the bare
// parameter read/write pair; the command names get_plugin_parameters /
// set_plugin_param stay reachable through the execution-facing lookups.
var seamS5BareParameterTools = []string{
	"plugin.get_parameters",
	"plugin_get_parameters",
	"plugin.set_parameter",
	"plugin_set_parameter",
}

var seamS5PackList = []string{
	"plugin.get_parameters",
	"plugin.set_parameter",
	"plugin_grabber.explain_controls",
	"plugin_grabber.inspect_reverb",
	"plugin_grabber.apply_reverb_controls",
	"plugin_grabber.apply_eq_edits",
	"plugin_grabber.set_eq_point",
}

// A-arm condition proof, S5 face: seam open → the model tool catalog carries
// no typed adapter under any naming form while every bare parameter tool
// stays on the menu and execution lookups keep the specs (governance faces
// not weakened).
func TestSeamS5OpenCatalogTrimsTypedAdapters(t *testing.T) {
	t.Setenv(pluginControlEvalSeamEnv, "1")
	c := DefaultCatalog()

	onMenu := map[string]bool{}
	for _, tool := range c.Tools() {
		if strings.Contains(tool.Name, "plugin_grabber") {
			t.Fatalf("open seam tool list must not carry a plugin_grabber adapter: %s", tool.Name)
		}
		if strings.HasPrefix(tool.KernelCommand, "plugin_grabber") {
			t.Fatalf("open seam tool list must not carry a plugin_grabber kernel command: tool=%s kernel=%s", tool.Name, tool.KernelCommand)
		}
		onMenu[tool.Name] = true
	}
	for _, name := range seamS5BlueprintTools {
		if onMenu[name] {
			t.Fatalf("open seam blueprint tool must be off the menu: %s", name)
		}
	}
	for _, name := range seamS5BareParameterTools {
		if !onMenu[name] {
			t.Fatalf("open seam must keep the bare parameter tool on the menu: %s", name)
		}
	}
	for _, command := range []string{"get_plugin_parameters", "set_plugin_param"} {
		if _, ok := c.LookupCommand(command); !ok {
			t.Fatalf("open seam must keep the bare parameter command reachable: %s", command)
		}
	}

	if summary := c.ModelSummary(); strings.Contains(summary, "plugin_grabber") {
		t.Fatal("open seam ModelSummary must not disclose plugin_grabber lines")
	}
	if summary := c.ModelSummaryForTools(seamS5PackList); strings.Contains(summary, "plugin_grabber") {
		t.Fatal("open seam ModelSummaryForTools must not disclose plugin_grabber lines even when a pack list names them")
	}

	for _, command := range []string{
		"plugin_grabber_explain_controls",
		"plugin_grabber_inspect_reverb",
		"plugin_grabber_apply_reverb_controls",
		"plugin_grabber_apply_delay_controls",
		"plugin_grabber_apply_eq_edits",
		"plugin_grabber_set_eq_point",
	} {
		if _, ok := c.LookupCommand(command); !ok {
			t.Fatalf("open seam must keep the execution-facing spec for %s", command)
		}
	}
}

// Closed seam keeps the production catalog byte-for-byte across Tools(),
// ModelSummary() and ModelSummaryForTools(); only the exact value "1" arms
// the trimming hook.
func TestSeamS5ClosedKeepsCatalogByteForByte(t *testing.T) {
	baseline := DefaultCatalog()
	baselineToolsJSON, err := json.Marshal(baseline.Tools())
	if err != nil {
		t.Fatal(err)
	}
	baselineSummary := baseline.ModelSummary()
	baselinePackSummary := baseline.ModelSummaryForTools(seamS5PackList)
	if !strings.Contains(baselineSummary, "plugin_grabber") {
		t.Fatal("closed seam catalog summary must still carry the plugin_grabber lines (production behavior)")
	}
	baselineOnMenu := map[string]bool{}
	for _, tool := range baseline.Tools() {
		baselineOnMenu[tool.Name] = true
	}
	for _, name := range seamS5BlueprintTools {
		if !baselineOnMenu[name] {
			t.Fatalf("closed seam blueprint tool must be on the menu (blueprint sanity): %s", name)
		}
	}

	for _, value := range []string{"", "0", "true", "off"} {
		t.Setenv(pluginControlEvalSeamEnv, value)
		c := DefaultCatalog()
		toolsJSON, err := json.Marshal(c.Tools())
		if err != nil {
			t.Fatal(err)
		}
		if string(toolsJSON) != string(baselineToolsJSON) {
			t.Fatalf("env %q must keep the tool list byte-for-byte", value)
		}
		if c.ModelSummary() != baselineSummary {
			t.Fatalf("env %q must keep ModelSummary byte-for-byte", value)
		}
		if c.ModelSummaryForTools(seamS5PackList) != baselinePackSummary {
			t.Fatalf("env %q must keep ModelSummaryForTools byte-for-byte", value)
		}
	}
}
