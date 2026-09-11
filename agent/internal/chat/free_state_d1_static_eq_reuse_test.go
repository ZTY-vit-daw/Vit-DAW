package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/executionports"
)

// B10 instance-reuse nails. Discovery runs on the live plugin graph read
// (get_project_state — the same legacy surface the bridge uses for shadow
// refreshes), because the revision-bound VSP compact snapshot strips plugin
// rows entirely; the discovered instance id travels into the plan builder as
// an explicit reference and becomes the plugin_id action arg, so the port
// writes parameters instead of stacking another instance. The graph shape
// below mirrors the 2026-09-11 forensics: plugin rows carry only the display
// identity (name + type), and Track 1007 showed two bx_hybrid V2 instances
// after two same-day runs.

func d1ReuseBindingForTest() *d1PluginParamWhitelistBinding {
	return &d1PluginParamWhitelistBinding{
		Section: d1StaticEQDomain, PluginName: "Fixture EQ", PluginPath: "C:/plugins/Fixture EQ.vst3",
		ParamID: "p315_c1", ParamIDCH2: "p315_c2", FrequencyHz: 400,
	}
}

// d1ReuseGraphReply reproduces the get_project_state reply family the
// discovery consumes: status plus track rows whose plugins array holds the
// live chain.
func d1ReuseGraphReply(extraPlugins ...map[string]any) map[string]any {
	plugins := []any{
		map[string]any{"plugin_item_id": "1008", "item_id": "1008", "id": "1008", "name": "Volume & Pan Plugin", "type": "volume"},
		map[string]any{"plugin_item_id": "1009", "item_id": "1009", "id": "1009", "name": "Level Meter", "type": "level"},
	}
	for _, plugin := range extraPlugins {
		plugins = append(plugins, any(plugin))
	}
	return map[string]any{
		"status": "ok",
		"tracks": []any{map[string]any{
			"track_id": "1007", "track_name": "bass", "plugins": plugins,
		}},
	}
}

func d1ReusePluginRow(id string) map[string]any {
	return map[string]any{"plugin_item_id": id, "item_id": id, "id": id, "name": "Fixture EQ", "type": "vst"}
}

// Nail 1a: discovery resolves the first same-identity instance on the target
// track from the live graph shape (name folded, builtins never match, other
// tracks never match, no match stays empty).
func TestD1StaticEQDiscoveryResolvesLiveGraphInstance(t *testing.T) {
	graph := d1ReuseGraphReply(d1ReusePluginRow("1040"))
	if got := d1ExistingPluginInstanceID(graph, "1007", "Fixture EQ"); got != "1040" {
		t.Fatalf("discovery must resolve the live instance: got %q", got)
	}
	if got := d1ExistingPluginInstanceID(graph, "1007", "  fixture eq  "); got != "1040" {
		t.Fatalf("discovery folds name case and surrounding whitespace: got %q", got)
	}
	// Builtin exclusion is carried by the whitelist identity, not by the
	// walker: asking for the whitelisted name must resolve the EQ row, never
	// the volume/level chain rows around it.
	if got := d1ExistingPluginInstanceID(graph, "1007", "Level Meter"); got != "1009" {
		t.Fatalf("discovery is name-driven; the whitelist decides identity: got %q", got)
	}
	if got := d1ExistingPluginInstanceID(graph, "1012", "Fixture EQ"); got != "" {
		t.Fatalf("other tracks never match: got %q", got)
	}
	if got := d1ExistingPluginInstanceID(d1ReuseGraphReply(), "1007", "Fixture EQ"); got != "" {
		t.Fatalf("no same-identity instance means no reuse: got %q", got)
	}
	if got := d1ExistingPluginInstanceID(d1ReuseGraphReply(d1ReusePluginRow("1040")), "1007", ""); got != "" {
		t.Fatalf("an empty plugin identity never reuses: got %q", got)
	}
}

// Nail 1b: an explicit instance reference turns the plan into a parameter
// write on that instance — plugin_id lands in the action args (the port's
// Apply skips the instantiate command exactly when this arg is present) while
// the normalized whitelist payload stays intact.
func TestD1StaticEQPlanReusesExistingWhitelistInstance(t *testing.T) {
	loop := d1StaticEQLoopForTest(t, "7")
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}
	plan, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"},
		7, "project-1", "epoch-1", "snapshot-7", state, d1ReuseBindingForTest(), "1040")
	if err != nil {
		t.Fatal(err)
	}
	args := plan.ActionSet.Actions[0].Args
	if args["plugin_id"] != "1040" {
		t.Fatalf("existing whitelist instance must be reused as the write target: args=%+v", args)
	}
	if args["write_mode"] != executionports.WriteModeNormalizedBatchV1 || args["param_id"] != "p315_c1" ||
		args["param_id_ch2"] != "p315_c2" || args["target_value"] != -1.0 || args["plugin_path"] != "C:/plugins/Fixture EQ.vst3" {
		t.Fatalf("reuse must keep the normalized write payload intact: args=%+v", args)
	}
	if _, present := args["plugin_identifier"]; present {
		t.Fatalf("real-plugin action must not carry a known-list identifier: %+v", args)
	}
}

// Nail 2 (no-regression): an empty instance reference keeps the historical
// instantiate shape byte-for-byte — plugin_id absent, whitelist payload
// unchanged.
func TestD1StaticEQPlanWithoutExistingInstanceKeepsInstantiatePath(t *testing.T) {
	loop := d1StaticEQLoopForTest(t, "7")
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}
	plan, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"},
		7, "project-1", "epoch-1", "snapshot-7", state, d1ReuseBindingForTest(), "")
	if err != nil {
		t.Fatal(err)
	}
	args := plan.ActionSet.Actions[0].Args
	if _, present := args["plugin_id"]; present {
		t.Fatalf("no reusable instance means the instantiate path must stay: args=%+v", args)
	}
	if args["write_mode"] != executionports.WriteModeNormalizedBatchV1 || args["plugin_path"] != "C:/plugins/Fixture EQ.vst3" ||
		args["plugin_name"] != "Fixture EQ" || args["param_id"] != "p315_c1" || args["param_id_ch2"] != "p315_c2" ||
		args["target_value"] != -1.0 || args["frequency_hz"] != 400.0 {
		t.Fatalf("instantiate-path args drifted: args=%+v", args)
	}
}

// Nail 3 (dose guard, reuse semantics): against the accident shape — the
// track already carrying two same-identity instances from two stacking runs —
// discovery deterministically resolves exactly one of them, replays converge
// on the same instance, the write stays the admitted absolute band value (no
// value stacking on top of prior runs), and the plan's admission bound still
// rejects an out-of-band gain even when reuse applies.
func TestD1StaticEQPlanReuseDoseGuardPinsOneBoundedInstance(t *testing.T) {
	loop := d1StaticEQLoopForTest(t, "7")
	graph := d1ReuseGraphReply(d1ReusePluginRow("1040"), d1ReusePluginRow("1039"))
	if first := d1ExistingPluginInstanceID(graph, "1007", "Fixture EQ"); first != "1040" {
		t.Fatalf("discovery must resolve exactly one instance deterministically (first match): got %q", first)
	}
	if second := d1ExistingPluginInstanceID(graph, "1007", "Fixture EQ"); second != "1040" {
		t.Fatalf("replays must converge on the same instance instead of stacking: got %q", second)
	}
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}
	candidate := agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"}
	plan, err := d1PluginParamPlanWithBinding(loop, candidate, 7, "project-1", "epoch-1", "snapshot-7", state, d1ReuseBindingForTest(), "1040")
	if err != nil {
		t.Fatal(err)
	}
	args := plan.ActionSet.Actions[0].Args
	if args["plugin_id"] != "1040" || args["target_value"] != -1.0 {
		t.Fatalf("the admitted absolute band value is the steady state on the reused instance: args=%+v", args)
	}
	loop.Experiment.Admission.TypedAction["gain_db"] = 3.0
	if _, err := d1PluginParamPlanWithBinding(loop, candidate, 7, "project-1", "epoch-1", "snapshot-7", state, d1ReuseBindingForTest(), "1040"); err == nil || !strings.Contains(err.Error(), "within +/-2 dB") {
		t.Fatalf("reuse must not bypass the admitted dose bound: err=%v", err)
	}
}

// d1RackWrappedGraphReply reproduces the rack-wrapped track row the kernel
// emits after a rack_add_node load (F4 forensics artifacts/F4/20260912_min/f1,
// raw get_project_state track 1017): the EQ disappears from the flat plugins
// array (only the "New Rack" wrapper row stays) and lives under the nested
// rack.nodes with the full plugin_item_id/plugin_name key family.
func d1RackWrappedGraphReply() map[string]any {
	return map[string]any{
		"status": "ok",
		"tracks": []any{map[string]any{
			"track_id": "1007", "track_name": "bass",
			"plugins": []any{
				map[string]any{"plugin_item_id": "1008", "item_id": "1008", "id": "1008", "name": "Volume & Pan Plugin", "type": "volume"},
				map[string]any{"plugin_item_id": "1052", "item_id": "1052", "id": "1052", "name": "New Rack", "type": "rack"},
			},
			"rack": map[string]any{
				"rack_item_id": "1052", "scope": "track",
				"nodes": []any{map[string]any{
					"node_id": "1053", "plugin_item_id": "1053", "item_id": "1053", "id": "1053", "plugin_id": "1053",
					"name": "Fixture EQ", "plugin_name": "Fixture EQ", "type": "vst",
					"zone_id": "Z3", "x": 40.0, "y": 500.0,
				}},
			},
		}},
	}
}

// Nail F4A-2: on a rack-wrapped project (the shape every governed load now
// produces) discovery must still resolve the EQ instance — the read walks all
// three faces (plugins, rack_nodes, nested rack.nodes) like VisiblePluginRefs,
// so reuse keeps converging on the rack node instead of stacking another one.
func TestD1StaticEQDiscoveryResolvesRackWrappedInstance(t *testing.T) {
	graph := d1RackWrappedGraphReply()
	if got := d1ExistingPluginInstanceID(graph, "1007", "Fixture EQ"); got != "1053" {
		t.Fatalf("discovery must resolve the rack-wrapped instance: got %q", got)
	}
	// Discovery stays name-driven: the wrapper row itself is just another row.
	if got := d1ExistingPluginInstanceID(graph, "1007", "New Rack"); got != "1052" {
		t.Fatalf("wrapper rows resolve by their own name: got %q", got)
	}
	if got := d1ExistingPluginInstanceID(graph, "1012", "Fixture EQ"); got != "" {
		t.Fatalf("other tracks never match: got %q", got)
	}
	// The flat rack_nodes face still resolves when a consumer reports that
	// shape (mirroring the shadow projection's alternate row key).
	flat := map[string]any{
		"status": "ok",
		"tracks": []any{map[string]any{
			"track_id": "1007",
			"rack_nodes": []any{map[string]any{"plugin_item_id": "1054", "id": "1054", "name": "Fixture EQ"}},
		}},
	}
	if got := d1ExistingPluginInstanceID(flat, "1007", "Fixture EQ"); got != "1054" {
		t.Fatalf("flat rack_nodes face must resolve: got %q", got)
	}
	// Mixed shape: a legacy bare instance and a rack-wrapped one on the same
	// track resolve deterministically to the first face's match.
	mixed := d1RackWrappedGraphReply()
	bare := mapRowsFromAny(mixed["tracks"])[0]
	bare["plugins"] = append(mapRowsFromAny(bare["plugins"]),
		map[string]any{"plugin_item_id": "1040", "item_id": "1040", "id": "1040", "name": "Fixture EQ", "type": "vst"})
	if got := d1ExistingPluginInstanceID(mixed, "1007", "Fixture EQ"); got != "1040" {
		t.Fatalf("mixed shapes resolve deterministically from the plugins face first: got %q", got)
	}
}
