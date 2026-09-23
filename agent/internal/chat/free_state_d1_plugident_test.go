package chat

import (
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/processorattestation"
)

// FIX-D1-PLUGIDENT-1: whitelist-bound normalized-batch plans must pin the
// whitelisted plugin identifier. WaveShell carriers (mac) expose hundreds of
// members behind one plugin_path, and the kernel resolves a non-empty
// identifier exclusively against its known-plugin list, fail-closing on shell
// paths without one — a plan that ships only path+name cannot instantiate
// there. One red test per plan builder (static_eq legacy shape and the
// table-driven PluginBound shape); the stub path (no binding) keeps the
// historical juce_eq schema and is asserted unchanged alongside.

func TestD1StaticEQBindingPlanArgsCarryWhitelistPluginIdentifier(t *testing.T) {
	loop := d1StaticEQLoopForTest(t, "7")
	binding := &d1StaticEQWhitelistBinding{
		Plugin: experimentplugins.StaticEQPlugin{
			PluginName: "Fixture EQ", Manufacturer: "Fixture", Format: "VST3",
			PluginIdentifier: "fixture-eq-shell-member", PluginPath: "/Library/Audio/Plug-Ins/VST3/Fixture Shell.vst3",
		},
		Band:        experimentplugins.Band{CenterHz: 315, GainParamIDCH1: "p315_c1", GainParamIDCH2: "p315_c2"},
		FrequencyHz: 400,
	}
	plan, err := d1StaticEQPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, binding)
	if err != nil {
		t.Fatal(err)
	}
	args := plan.ActionSet.Actions[0].Args
	if got := args["plugin_identifier"]; got != binding.Plugin.PluginIdentifier {
		t.Fatalf("whitelisted static_eq plan must pin the whitelist plugin identifier for shell carriers: args=%+v", args)
	}

	// Stub path (no binding) keeps the historical juce_eq schema byte-for-byte.
	stubArgs, stubParamID := d1StaticEQActionArgs(map[string]any{"band_index": 2}, -1.5, nil)
	if stubArgs["plugin_identifier"] != defaultD1StaticEQPluginIdentifier || stubArgs["param_id"] != "band_2_gain" ||
		stubArgs["target_value"] != -1.5 || stubParamID != "band_2_gain" {
		t.Fatalf("stub args drifted: %+v param=%q", stubArgs, stubParamID)
	}
}

func TestPluginBoundPlanArgsCarryWhitelistPluginIdentifier(t *testing.T) {
	fixture := d1CompressionWhitelistFixtureOnDisk(t)
	subject := processorattestation.Subject{Name: "Fixture Comp", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-comp", InstalledPath: fixture.PluginPath}
	d1TableOverrideLoaders(t, fixture.Whitelist, nil, d1CompressionPromotedLibrary(t, subject, fixture.Fingerprint), nil)

	// Resolve through the production resolver so the binding's identifier
	// provably originates from the whitelist section, not the test literal.
	binding, err := resolveD1PluginParamWhitelistBinding(map[string]any{"action_domain": d1BroadbandCompressionDomain, "threshold_db": 1.5})
	if err != nil {
		t.Fatal(err)
	}
	loop := d1CompressionLoopForTest(t, "7")
	plan, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1BroadbandCompressionKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, binding, "")
	if err != nil {
		t.Fatal(err)
	}
	args := plan.ActionSet.Actions[0].Args
	if got := args["plugin_identifier"]; got != fixture.Whitelist.BroadbandCompression.PluginIdentifier {
		t.Fatalf("PluginBound plan must pin the whitelist plugin identifier for shell carriers: args=%+v", args)
	}

	// Stub path (no binding) keeps the historical juce_eq default identifier;
	// only static_eq carries a stub form, mirroring pluginParamWriteArgs.
	stubSpec, ok := experiment.D1S1SpecForAction(d1StaticEQDomain, d1StaticEQKind)
	if !ok {
		t.Fatal("static_eq domain spec missing")
	}
	stubArgs, stubParamID := pluginParamWriteArgs(map[string]any{"band_index": 1}, stubSpec, 1.0, nil)
	if stubArgs["plugin_identifier"] != defaultD1StaticEQPluginIdentifier || stubArgs["param_id"] != "band_1_gain" ||
		stubArgs["target_value"] != 1.0 || stubParamID != "band_1_gain" {
		t.Fatalf("stub args drifted: %+v param=%q", stubArgs, stubParamID)
	}
}
