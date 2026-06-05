package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tools"
	"vit-daw-agent/internal/workflows/plugingrabber"
)

type fakeKernelClient struct {
	replies  []map[string]any
	commands []map[string]any
}

func (f *fakeKernelClient) SendCommand(_ context.Context, cmd map[string]any) (map[string]any, string, error) {
	f.commands = append(f.commands, tools.CloneCommand(cmd))
	if len(f.replies) == 0 {
		return map[string]any{"status": "ok"}, `{"status":"ok"}`, nil
	}
	reply := f.replies[0]
	f.replies = f.replies[1:]
	if errText := strings.TrimSpace(fmt.Sprint(reply["error"])); errText != "" && errText != "<nil>" {
		return reply, "", fmt.Errorf("%s", errText)
	}
	return reply, "", nil
}

func TestInvokeConfirmCommandDoesNotNeedKernelBeforeApproval(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{"cmd": "delete_track", "track_id": "1007"},
		Source:  "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "needs_confirmation" {
		t.Fatalf("status = %q, want needs_confirmation", resp.Status)
	}
	if resp.AgentActionID == "" || resp.Preview == "" {
		t.Fatalf("missing action id or preview: %+v", resp)
	}
}

func TestResolveRejectsUnknownCommand(t *testing.T) {
	h := New(nil, nil, nil)
	_, _, err := h.resolveCommand(InvokeRequest{
		Tool: "daw.invoke",
		Args: map[string]any{"cmd": "totally_not_registered"},
	})
	if err == nil {
		t.Fatal("expected unknown command error")
	}
}

func TestRollbackWorkspaceApplyEditUsesReversePatch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("hello vit"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "workspace.apply_edit",
		Args: map[string]any{
			"path":            path,
			"old_text":        "vit",
			"new_text":        "history",
			"workspace_roots": []any{root},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("apply edit: %v", err)
	}
	if resp.AgentActionID == "" {
		t.Fatalf("missing action id: %+v", resp)
	}
	if b, _ := os.ReadFile(path); string(b) != "hello history" {
		t.Fatalf("after apply = %q", string(b))
	}
	_, err = h.Invoke(context.Background(), InvokeRequest{
		Tool:      "agent.rollback_action",
		Args:      map[string]any{"target_action_id": resp.AgentActionID},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "hello vit" {
		t.Fatalf("after rollback = %q", string(b))
	}
	action, ok := h.journal.Get(resp.AgentActionID)
	if !ok {
		t.Fatalf("missing journal action %s", resp.AgentActionID)
	}
	if action.Status != journal.StatusRolledBack || action.RollbackState != "succeeded" {
		t.Fatalf("rollback journal = %+v", action)
	}
}

func TestRollbackDAWActionCallsProjectUndo(t *testing.T) {
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{"status": "ok", "agent_action_id": "undo_1", "message": "undone"},
		},
	}
	h := New(nil, nil, nil)
	h.kernel = kernel
	h.journal.Record(journal.Action{
		AgentActionID: "act_daw",
		Domain:        "daw",
		Source:        "test",
		Tool:          "track.mute",
		CommandName:   "set_mute",
		Command:       map[string]any{"cmd": "set_mute", "track_id": "1007", "mute": true},
		Status:        journal.StatusSucceeded,
	})

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "agent.rollback_action",
		Args:      map[string]any{"target_action_id": "act_daw"},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if resp.Status != "ok" || len(kernel.commands) != 1 {
		t.Fatalf("resp=%+v commands=%+v", resp, kernel.commands)
	}
	if kernel.commands[0]["cmd"] != "undo" || kernel.commands[0]["target_action_id"] != "act_daw" {
		t.Fatalf("undo command = %+v", kernel.commands[0])
	}
	action, ok := h.journal.Get("act_daw")
	if !ok {
		t.Fatal("target action missing")
	}
	if action.Status != journal.StatusRolledBack || action.RollbackActionID != "undo_1" || action.RollbackState != "succeeded" {
		t.Fatalf("rollback journal = %+v", action)
	}
}

func TestRollbackNonAutomaticDomainIsRejected(t *testing.T) {
	h := New(nil, nil, nil)
	h.journal.Record(journal.Action{
		AgentActionID: "act_web",
		Domain:        "web",
		Source:        "test",
		Tool:          "web.fetch",
		CommandName:   "web_fetch",
		Command:       map[string]any{"cmd": "web_fetch", "url": "https://example.com"},
		Status:        journal.StatusSucceeded,
	})
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "agent.rollback_action",
		Args:      map[string]any{"target_action_id": "act_web"},
		Confirmed: true,
		Source:    "test",
	})
	if err == nil {
		t.Fatalf("expected rollback rejection, resp=%+v", resp)
	}
	if !strings.Contains(err.Error(), `domain "web"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveToolBuildsKernelCommand(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.mute",
		Args: map[string]any{"track_id": "1007", "mute": true},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if spec.CommandName != "set_mute" || cmd["cmd"] != "set_mute" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v spec = %+v", cmd, spec)
	}
}

func TestResolveToolFormCommandBuildsKernelCommand(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"tool": "track.solo",
			"args": map[string]any{
				"track_id": "1007",
				"enabled":  "true",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if spec.CommandName != "set_solo" || cmd["cmd"] != "set_solo" || cmd["track_id"] != "1007" || cmd["solo"] != true {
		t.Fatalf("cmd = %+v spec = %+v", cmd, spec)
	}
}

func TestResolvePluginTargetFromContext(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd":               "plugin_grabber_upsert_project_profile",
			"quick_control_ids": []any{"dry", "wet"},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_plugin_id":       "plugin_a",
		"selected_plugin_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1007" || cmd["plugin_id"] != "plugin_a" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestPublicPluginParametersResultIncludesRuntimeProfile(t *testing.T) {
	out := publicPluginParametersResult(nil, map[string]any{
		"status":                 "ok",
		"track_id":               "1007",
		"plugin_id":              "plugin_a",
		"global_profile_applied": true,
		"global_profile_source":  "global_profile",
		"plugin_class":           "compressor",
		"plugin_groups": []any{
			map[string]any{"id": "main_dynamics", "role": "compressor", "label": "Main dynamics"},
		},
		"virtual_controls": []any{
			map[string]any{"name": "tighten dynamics", "component_id": "main_dynamics", "resolver": "local_profile_mapping"},
		},
		"safety_limits": map[string]any{"max_gain_change_db": 6},
		"global_profile": map[string]any{
			"plugin_skill": map[string]any{
				"schema_version": 2,
				"components": []any{
					map[string]any{
						"id":     "main_dynamics",
						"role":   "compressor",
						"params": map[string]any{"threshold": map[string]any{"param_id": "threshold", "confidence": 0.9}},
					},
				},
				"operations": []any{
					map[string]any{"name": "tighten dynamics", "component_id": "main_dynamics", "params": map[string]any{"threshold": "threshold"}},
				},
			},
		},
		"quick_controls": []any{
			map[string]any{"param_id": "threshold", "label": "Threshold"},
		},
		"recommended_groups": []any{},
		"parameters": []any{
			map[string]any{"id": "threshold"},
		},
	})
	if out["plugin_class"] != "compressor" || out["global_profile_applied"] != true {
		t.Fatalf("runtime profile flags missing: %+v", out)
	}
	if out["plugin_group_count"] != 1 || out["virtual_control_count"] != 1 {
		t.Fatalf("runtime profile counts missing: %+v", out)
	}
	skill, ok := out["plugin_skill"].(map[string]any)
	if !ok || skill["component_count"] != 1 || skill["operation_count"] != 1 {
		t.Fatalf("plugin_skill summary missing: %+v", out["plugin_skill"])
	}
	components := mapRowsFromAny(skill["components"])
	if len(components) != 1 || len(mapRowsFromAny(components[0]["params"])) != 1 {
		t.Fatalf("plugin_skill params missing: %+v", skill["components"])
	}
}

func TestPluginLoadToRackNormalizesArgs(t *testing.T) {
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(t.TempDir(), "missing_plugin_semantics.json"))
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{
			"path": "C:/Program Files/Common Files/VST3/TDR Nova.vst3",
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "rack_add_node" || cmd["track_id"] != "1007" || cmd["plugin_path"] == "" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["x"] == nil || cmd["y"] == nil || cmd["zone_id"] != "Z3" {
		t.Fatalf("rack defaults missing: %+v", cmd)
	}
}

func TestPluginLoadToRackInstrumentDefaultsToZ2FromSemanticIndex(t *testing.T) {
	surgePath := `C:\Program Files\Common Files\VST3\Surge Synth Team\Surge XT.vst3\Contents\x86_64-win\Surge XT.vst3`
	semanticsPath := filepath.Join(t.TempDir(), "plugin_semantics.json")
	idx := pluginsemantics.Build([]map[string]any{
		{
			"name":          "Surge XT Effects",
			"category":      "Fx",
			"plugin_path":   `C:\Program Files\Common Files\VST3\Surge Synth Team\Surge XT Effects.vst3\Contents\x86_64-win\Surge XT Effects.vst3`,
			"is_instrument": false,
		},
		{
			"name":          "Surge XT",
			"category":      "Instrument|Synth",
			"plugin_path":   surgePath,
			"is_instrument": true,
		},
	}, time.Now().UTC())
	if _, err := pluginsemantics.Save(semanticsPath, idx); err != nil {
		t.Fatalf("save semantics: %v", err)
	}
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)

	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{"path": surgePath},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if got := cmd["zone_id"]; got != "Z2" {
		t.Fatalf("zone_id = %#v, want Z2; cmd=%+v", got, cmd)
	}
}

func TestPluginLoadToRackInstrumentOverridesPlannerZ3(t *testing.T) {
	surgePath := `C:\Program Files\Common Files\VST3\Surge Synth Team\Surge XT.vst3\Contents\x86_64-win\Surge XT.vst3`
	semanticsPath := filepath.Join(t.TempDir(), "plugin_semantics.json")
	idx := pluginsemantics.Build([]map[string]any{
		{
			"name":          "Surge XT",
			"category":      "Instrument|Synth",
			"plugin_path":   surgePath,
			"is_instrument": true,
		},
	}, time.Now().UTC())
	if _, err := pluginsemantics.Save(semanticsPath, idx); err != nil {
		t.Fatalf("save semantics: %v", err)
	}
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)

	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{
			"path":    surgePath,
			"zone_id": "Z3",
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if got := cmd["zone_id"]; got != "Z2" {
		t.Fatalf("zone_id = %#v, want corrected Z2; cmd=%+v", got, cmd)
	}
	preview := PreviewCommand(spec, cmd)
	if strings.Contains(preview, "Zone Z3") || !strings.Contains(preview, "Zone Z2") {
		t.Fatalf("preview did not show corrected zone:\n%s", preview)
	}
}

func TestPluginLoadToRackInstrumentDefaultsToZ2FromKernelPluginList(t *testing.T) {
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(t.TempDir(), "missing_plugin_semantics.json"))
	surgePath := `C:\Program Files\Common Files\VST3\Surge Synth Team\Surge XT.vst3\Contents\x86_64-win\Surge XT.vst3`
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status": "ok",
			"plugins": []any{
				map[string]any{
					"name":          "Surge XT",
					"category":      "Instrument|Synth",
					"path":          surgePath,
					"is_instrument": true,
				},
			},
		},
	}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{"path": surgePath},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if got := cmd["zone_id"]; got != "Z2" {
		t.Fatalf("zone_id = %#v, want Z2; cmd=%+v", got, cmd)
	}
	if len(kernel.commands) != 1 || kernel.commands[0]["cmd"] != "plugin_list_available" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
}

func TestPluginParametersPublicResultIsCompact(t *testing.T) {
	h := New(nil, nil, nil)
	result := h.publicResult(tools.CommandSpec{CommandName: "get_plugin_parameters"}, nil, map[string]any{
		"status":     "ok",
		"track_id":   "1007",
		"plugin_id":  "plugin_a",
		"parameters": []any{map[string]any{"id": "a"}, map[string]any{"id": "b"}},
		"quick_controls": []any{
			map[string]any{"param_id": "a", "label": "A", "display_group": "Mix"},
		},
		"recommended_groups": []any{
			map[string]any{"name": "Mix", "parameter_ids": []any{"a", "b"}},
		},
	})
	if result["parameter_count"] != 2 || result["quick_control_count"] != 1 {
		t.Fatalf("result counts = %+v", result)
	}
	if _, ok := result["parameters"]; ok {
		t.Fatalf("public result leaked full parameters: %+v", result)
	}
	if summary, ok := result["display_probe_summary"].(map[string]any); !ok || summary["parameter_count"] != 2 {
		t.Fatalf("display probe summary = %+v", result["display_probe_summary"])
	}
}

func TestPluginParametersPublicResultKeepsCompactDisplayProbeWhenIncluded(t *testing.T) {
	h := New(nil, nil, nil)
	result := h.publicResult(tools.CommandSpec{CommandName: "get_plugin_parameters"}, map[string]any{"include_parameters": true}, map[string]any{
		"status":    "ok",
		"track_id":  "1007",
		"plugin_id": "plugin_a",
		"parameters": []any{map[string]any{
			"id":                "delay",
			"name":              "Delay",
			"host_controllable": true,
			"display_probe": map[string]any{
				"mode":  "read_only_value_to_string",
				"label": "ms",
				"samples": []any{
					map[string]any{"normalized_value": 0.0, "value": 0.0, "text": "0 ms"},
					map[string]any{"normalized_value": 1.0, "value": 1.0, "text": "2000 ms"},
				},
			},
		}},
	})
	params := mapRowsFromAny(result["parameters"])
	if len(params) != 1 {
		t.Fatalf("parameters = %+v", result["parameters"])
	}
	if _, ok := params[0]["display_probe"]; !ok {
		t.Fatalf("compact parameter missing display_probe: %+v", params[0])
	}
	if domain, ok := params[0]["display_domain_candidate"].(*plugingrabber.PluginDisplayDomain); !ok || domain.Unit != "ms" {
		t.Fatalf("display_domain_candidate = %+v", params[0]["display_domain_candidate"])
	}
}

func TestNormalizeTrackBooleanAliases(t *testing.T) {
	h := New(nil, nil, nil)
	cases := []struct {
		tool  string
		args  map[string]any
		field string
		want  bool
	}{
		{"track.mute", map[string]any{"track_id": "1007", "enabled": "on"}, "mute", true},
		{"track.mute", map[string]any{"track_id": "1007", "muted": "0"}, "mute", false},
		{"track.solo", map[string]any{"track_id": "1007", "value": 1}, "solo", true},
		{"track.arm", map[string]any{"track_id": "1007", "armed": "false"}, "is_armed", false},
	}
	for _, tc := range cases {
		cmd, spec, err := h.resolveCommand(InvokeRequest{Tool: tc.tool, Args: tc.args})
		if err != nil {
			t.Fatalf("%s resolveCommand: %v", tc.tool, err)
		}
		if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
			t.Fatalf("%s resolveImplicitTargets: %v", tc.tool, err)
		}
		if got := cmd[tc.field]; got != tc.want {
			t.Fatalf("%s %s = %#v, want %#v; cmd=%+v", tc.tool, tc.field, got, tc.want, cmd)
		}
	}
}

func TestInferMissingTrackBooleanFromUserMessage(t *testing.T) {
	h := New(nil, nil, nil)
	cases := []struct {
		name    string
		command map[string]any
		context map[string]any
		field   string
		want    bool
	}{
		{
			name:    "mute on",
			command: map[string]any{"cmd": "set_mute", "track_id": "1007"},
			context: map[string]any{"user_message": "我想让track1静音"},
			field:   "mute",
			want:    true,
		},
		{
			name:    "mute off",
			command: map[string]any{"cmd": "set_mute", "track_id": "1007"},
			context: map[string]any{"user_message": "取消静音"},
			field:   "mute",
			want:    false,
		},
		{
			name:    "mute off overrides model true",
			command: map[string]any{"cmd": "set_mute", "track_id": "1007", "mute": true},
			context: map[string]any{"user_message": "取消静音"},
			field:   "mute",
			want:    false,
		},
		{
			name:    "solo on",
			command: map[string]any{"cmd": "set_solo", "track_id": "1012"},
			context: map[string]any{"user_message": "track 2 solo"},
			field:   "solo",
			want:    true,
		},
		{
			name:    "solo off overrides model true",
			command: map[string]any{"cmd": "set_solo", "track_id": "1012", "solo": true},
			context: map[string]any{"user_message": "取消solo"},
			field:   "solo",
			want:    false,
		},
	}
	for _, tc := range cases {
		cmd, spec, err := h.resolveCommand(InvokeRequest{Command: tc.command})
		if err != nil {
			t.Fatalf("%s resolveCommand: %v", tc.name, err)
		}
		if err := h.resolveImplicitTargets(context.Background(), spec, cmd, tc.context); err != nil {
			t.Fatalf("%s resolveImplicitTargets: %v", tc.name, err)
		}
		if got := cmd[tc.field]; got != tc.want {
			t.Fatalf("%s %s = %#v, want %#v; cmd=%+v", tc.name, tc.field, got, tc.want, cmd)
		}
	}
}

func TestInvokeRejectsMissingRequiredTargetID(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:   "track.rename",
		Args:   map[string]any{"name": "Lead Vocal"},
		Source: "test",
	})
	if err == nil {
		t.Fatal("expected missing target id error")
	}
	if resp.Status != "error" || resp.CommandName != "rename_track" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestResolveImplicitTrackIDFromSingleVisibleTrack(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Track 1", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.rename",
		Args: map[string]any{"new_name": "Lead Vocal"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1007" {
		t.Fatalf("track_id = %#v, want 1007", cmd["track_id"])
	}
	if cmd["name"] != "Lead Vocal" {
		t.Fatalf("name = %#v, want Lead Vocal", cmd["name"])
	}
}

func TestResolveFlattensNestedParams(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "rename_track",
			"params": map[string]any{
				"track_id": "1007",
				"new_name": "A",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if _, ok := cmd["params"]; ok {
		t.Fatalf("params was not flattened: %+v", cmd)
	}
	if cmd["track_id"] != "1007" || cmd["name"] != "A" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveFlattensNestedArgsOnCommand(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "move_clip",
			"args": map[string]any{
				"clip_id":         "clip_a",
				"source_track_id": "1007",
				"target_track_id": "1007",
				"new_start":       10.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if _, ok := cmd["args"]; ok {
		t.Fatalf("args was not flattened: %+v", cmd)
	}
	if cmd["source_track_id"] != "1007" || cmd["target_track_id"] != "1007" || cmd["new_start"] != 10.0 {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveToolFormRemoveClipStringID(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"tool": "clip.remove",
			"args": map[string]any{"clip_ids": "clip_a"},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	got := cmd["clip_ids"].([]string)
	if len(got) != 1 || got[0] != "clip_a" {
		t.Fatalf("clip_ids = %#v", got)
	}
}

func TestResolveImplicitTrackIDByUserTrackIndex(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Track 1", "track_type": "hybrid", "is_audio_track": true},
			map[string]any{"track_id": "1010", "track_name": "Track 2", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.mute",
		Args: map[string]any{"user_track_index": 2, "mute": true},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1010" {
		t.Fatalf("track_id = %#v, want 1010", cmd["track_id"])
	}
}

func TestResolveImplicitTrackIDFromSelectedContext(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Drums", "track_type": "hybrid", "is_audio_track": true},
			map[string]any{"track_id": "1010", "track_name": "Bass", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.rename",
		Args: map[string]any{"new_name": "Low End"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id":   "1010",
		"selected_track_name": "Bass",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1010" {
		t.Fatalf("track_id = %#v, want 1010", cmd["track_id"])
	}
}

func TestResolveClipResizeFromSelectedContext(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.resize",
		Args: map[string]any{"new_length": 4.5},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_b" || cmd["track_id"] != "1010" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveClipResizeInfersLengthFromUserMessage(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "resize_clip",
			"args": map[string]any{
				"clip_id": "clip_b",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "把选中的clip裁到2秒",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_b" || cmd["track_id"] != "1010" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["new_length"] != 2.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("length args = %+v", cmd)
	}
}

func TestResolveClipSplitFromSelectedContext(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.split",
		Args: map[string]any{"split_time": 1.0},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["split_time"] != 1.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestResolveClipSplitInfersPlayheadFromContext(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.split",
		Args: map[string]any{},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "在播放头这里切开选中的clip",
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       1.25,
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["split_time"] != 1.25 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestResolveClipSplitOverridesModelZeroForPlayhead(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.split",
		Args: map[string]any{"split_time": 0.0},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "split the selected clip at the playhead",
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       1.25,
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["split_time"] != 1.25 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestInvokeClipSelectIsLocalUIAction(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.select",
		Args: map[string]any{"clip_name": "Loop B"},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Status != "ok" || resp.CommandName != "select_clip" {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Result["ui_action"] != "select_clip" || resp.Result["clip_id"] != "clip_b" || resp.Result["track_id"] != "1010" {
		t.Fatalf("result = %+v", resp.Result)
	}
}

func TestVersionCheckpointUsesDraftProjectWhenProjectPathMissing(t *testing.T) {
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", t.TempDir())
	h := New(nil, shadow.New(nil), nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.checkpoint",
		Args:      map[string]any{"message": "smoke"},
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if resp.Status != "ok" || resp.Tool != "version.checkpoint" {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Result["project_path"] == "" || resp.Result["commit_id"] == "" || resp.Result["draft"] != true {
		t.Fatalf("draft result = %+v", resp.Result)
	}
	if resp.ProjectHistory["draft"] != true || resp.ProjectHistory["project_path"] == "" {
		t.Fatalf("project history = %+v", resp.ProjectHistory)
	}
}

func TestConfirmedMutatingToolCreatesOneGoalBaseline(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(projectPath, []byte("<EDIT disk=\"one\"/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Drums", "clips": []any{
				map[string]any{"id": "clip_a", "name": "Loop A"},
			}},
		},
	})
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{"status": "ok", "snapshot_xml": "<EDIT memory=\"baseline\"/>", "project_path": projectPath},
			{"status": "ok", "saved": true},
			{"status": "ok", "saved": true},
		},
	}
	h := New(nil, project, nil)
	h.kernel = kernel
	first, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "project.save",
		Confirmed: true,
		GoalID:    "goal_baseline",
		RunID:     "run_one",
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("first invoke: %v", err)
	}
	if first.Status != "ok" || first.ProjectHistory["baseline_commit"] == "" {
		t.Fatalf("first response = %+v", first)
	}
	second, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "project.save",
		Confirmed: true,
		GoalID:    "goal_baseline",
		RunID:     "run_two",
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("second invoke: %v", err)
	}
	baselineID := fmt.Sprint(first.ProjectHistory["baseline_commit"])
	if fmt.Sprint(second.ProjectHistory["baseline_commit"]) != baselineID {
		t.Fatalf("baseline changed: first=%+v second=%+v", first.ProjectHistory, second.ProjectHistory)
	}
	if len(kernel.commands) != 3 || kernel.commands[0]["cmd"] != "project_snapshot_export" || kernel.commands[1]["cmd"] != "save_project" || kernel.commands[2]["cmd"] != "save_project" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	actions := h.Actions(2)
	if len(actions) != 2 || actions[0].VersionCommitID != baselineID || actions[1].VersionCommitID != baselineID {
		t.Fatalf("actions = %+v baseline=%s", actions, baselineID)
	}
	status, err := history.Status(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("history status: %v", err)
	}
	if status["commit_count"] != 1 {
		t.Fatalf("history status = %+v", status)
	}
	goal := h.RuntimeStatus("goal_baseline")
	if goal.ProjectHistory == nil || goal.ProjectHistory.BaselineCommitID != baselineID {
		t.Fatalf("goal project history = %+v", goal.ProjectHistory)
	}
}

func TestProjectHistoryCheckoutReloadsKernelAndShadow(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(projectPath, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstResult, err := history.Checkpoint(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("checkpoint first: %v", err)
	}
	firstCommit := firstResult["commit"].(history.Commit)
	if err := os.WriteFile(projectPath, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Checkpoint(map[string]any{"project_path": projectPath}); err != nil {
		t.Fatalf("checkpoint second: %v", err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath})
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{"status": "ok"},
			{"status": "ok", "project_path": projectPath},
		},
	}
	h := New(nil, project, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.checkout",
		Args:      map[string]any{"commit_id": firstCommit.ID},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("checkout invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["commit_id"] != firstCommit.ID {
		t.Fatalf("resp = %+v", resp)
	}
	if got, err := os.ReadFile(projectPath); err != nil || string(got) != "one" {
		t.Fatalf("project after checkout = %q err=%v", string(got), err)
	}
	if len(kernel.commands) != 2 || kernel.commands[0]["cmd"] != "open_project" || kernel.commands[0]["file_path"] != projectPath || kernel.commands[1]["cmd"] != "get_project_state" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	refresh := resp.Result["refresh"].(map[string]any)
	if refresh["kernel_reloaded"] != true || refresh["shadow_refreshed"] != true {
		t.Fatalf("refresh = %+v", refresh)
	}
}

func TestProjectHistoryWorktreeCheckoutOpensTargetProject(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(projectPath, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := history.Checkpoint(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	commit := checkpoint["commit"].(history.Commit)
	worktree, err := history.WorktreeCreate(map[string]any{"project_path": projectPath, "commit_id": commit.ID, "name": "wt-1"})
	if err != nil {
		t.Fatalf("worktree create: %v", err)
	}
	targetProject := fmt.Sprint(worktree["project_file_path"])
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath})
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{"status": "ok"},
			{"status": "ok", "project_path": targetProject},
		},
	}
	h := New(nil, project, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.worktree_checkout",
		Args:      map[string]any{"name": "wt-1"},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("worktree checkout invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["project_file_path"] != targetProject {
		t.Fatalf("resp = %+v target=%s", resp, targetProject)
	}
	if len(kernel.commands) != 3 ||
		kernel.commands[0]["cmd"] != "project_snapshot_export" ||
		kernel.commands[1]["cmd"] != "open_project" ||
		kernel.commands[1]["file_path"] != targetProject ||
		kernel.commands[2]["cmd"] != "get_project_state" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
}

func TestProjectHistoryWorktreeCheckoutAutosaveUsesKernelProjectPath(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(projectPath, []byte("root-one"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := history.Checkpoint(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	commit := checkpoint["commit"].(history.Commit)
	worktree, err := history.WorktreeCreate(map[string]any{"project_path": projectPath, "commit_id": commit.ID, "name": "wt-1"})
	if err != nil {
		t.Fatalf("worktree create: %v", err)
	}
	targetProject := fmt.Sprint(worktree["project_file_path"])
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath})
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{"status": "ok", "snapshot_xml": "worktree-live", "project_path": targetProject},
			{"status": "ok"},
			{"status": "ok", "project_path": projectPath},
		},
	}
	h := New(nil, project, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.worktree_checkout",
		Args:      map[string]any{"name": "main"},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("worktree checkout invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["project_file_path"] != projectPath {
		t.Fatalf("resp = %+v", resp)
	}
	rootList, err := history.List(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("root list: %v", err)
	}
	if commits := rootList["commits"].([]history.Commit); len(commits) != 1 || commits[0].ID != commit.ID {
		t.Fatalf("root history was polluted by worktree autosave: %+v", commits)
	}
	targetList, err := history.List(map[string]any{"project_path": targetProject})
	if err != nil {
		t.Fatalf("target list: %v", err)
	}
	foundAutosave := false
	for _, item := range targetList["commits"].([]history.Commit) {
		if item.Source == "worktree_checkout" && item.ProjectPath == targetProject {
			foundAutosave = true
			break
		}
	}
	if !foundAutosave {
		t.Fatalf("worktree autosave missing from target history: %+v", targetList["commits"])
	}
}

func TestResolveClipSelectFromCurrentTrackScope(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.select",
		Args: map[string]any{"track_id": "1007"},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["clip_id"] != "clip_a" || resp.Result["track_id"] != "1007" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestResolveClipSelectFromUserTrackIndexScope(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.select",
		Args: map[string]any{"user_track_index": 1},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["clip_id"] != "clip_a" || resp.Result["track_id"] != "1007" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestResolveClipSelectAmbiguousTrackScopeRequiresClipIndex(t *testing.T) {
	project := shadowProjectWithClips()
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Drums",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{"id": "clip_a", "name": "Loop A"},
					map[string]any{"id": "clip_c", "name": "Loop C"},
				},
			},
		},
	})
	h := New(nil, project, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.select",
		Args: map[string]any{"track_id": "1007"},
	})
	if err == nil {
		t.Fatalf("expected ambiguity error, resp=%+v", resp)
	}
	if resp.Status != "error" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestResolveRemoveClipsFromSelectedIDs(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.remove",
		Args: map[string]any{},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_ids": []any{"clip_a", "clip_b"},
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	got := cmd["clip_ids"].([]string)
	if len(got) != 2 || got[0] != "clip_a" || got[1] != "clip_b" {
		t.Fatalf("clip_ids = %#v", got)
	}
}

func TestRemoveClipsPublicResultIncludesUIAction(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.remove",
		Args: map[string]any{},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"clip_ids": []string{"clip_a", "clip_b"}},
		map[string]any{"status": "ok", "missing_ids": []any{"clip_b"}},
	)
	removed := result["removed_clip_ids"].([]string)
	if result["ui_action"] != "remove_clips" || len(removed) != 1 || removed[0] != "clip_a" {
		t.Fatalf("result = %+v", result)
	}
	requested := result["requested_clip_ids"].([]string)
	if len(requested) != 2 || requested[0] != "clip_a" || requested[1] != "clip_b" {
		t.Fatalf("requested = %#v", requested)
	}
}

func TestResolveMoveClipDefaultsTracksFromSelectedClip(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.move",
		Args: map[string]any{"new_start_seconds": 2.0},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_a" || cmd["source_track_id"] != "1007" || cmd["target_track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["new_start"] != 2.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestResolveMoveClipInfersNewStartFromUserMessage(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "move_clip",
			"args": map[string]any{
				"clip_id":         "clip_a",
				"source_track_id": "1007",
				"target_track_id": "1007",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message": "移动选中的clip移动10s",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["new_start"] != 10.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveMoveClipInfersPlayheadTarget(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.move",
		Args: map[string]any{"clip_id": "clip_a"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "把这个音频移动到播放头",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       4.5,
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["new_start"] != 4.5 || cmd["time_unit"] != "seconds" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveCloneClipAliasesAndTargetTrack(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{
			"clip_id":            "clip_a",
			"target_track_index": 2,
			"new_start":          8.0,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["source_clip_id"] != "clip_a" || cmd["target_track_id"] != "1010" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["time_unit"] != "seconds" {
		t.Fatalf("time_unit = %#v", cmd["time_unit"])
	}
}

func TestCloneClipPublicResultSelectsNewClip(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{"clip_id": "clip_a", "target_track_id": "1010", "new_start": 8.0},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"source_clip_id": "clip_a", "target_track_id": "1010"},
		map[string]any{"status": "ok", "new_clip_id": "clip_new"},
	)
	created := result["created_clip_ids"].([]string)
	if result["ui_action"] != "select_clip" || result["clip_id"] != "clip_new" || result["track_id"] != "1010" {
		t.Fatalf("result = %+v", result)
	}
	if result["source_clip_id"] != "clip_a" || len(created) != 1 || created[0] != "clip_new" {
		t.Fatalf("clone metadata = %+v", result)
	}
}

func TestResolveMidiPatchFromSelectedClipDefaultsBeats(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.insert_notes",
		Args: map[string]any{
			"notes": []any{
				map[string]any{"pitch": 60, "start": 0.0, "length": 0.5, "velocity": 96},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "apply_midi_note_patch" || cmd["clip_id"] != "clip_b" || cmd["track_id"] != "1010" || cmd["time_unit"] != "beats" {
		t.Fatalf("cmd = %+v", cmd)
	}
	ops := cmd["operations"].([]map[string]any)
	if len(ops) != 1 || ops[0]["op"] != "insert_note" || ops[0]["pitch"] != 60 {
		t.Fatalf("ops = %#v", ops)
	}
}

func TestMidiPatchRequiresOperations(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{"clip_id": "clip_a"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	err = h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_track_id": "1007",
	})
	if err == nil {
		t.Fatalf("expected missing operations error, cmd=%+v", cmd)
	}
}

func TestMidiPatchInfersInsertOpForBareNoteOperations(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{
			"operations": []any{
				map[string]any{"pitch": 51, "start": 0.0, "duration": 0.95, "velocity": 84},
				map[string]any{"pitch": 58, "start": 0.0, "length": 0.95, "velocity": 84},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	ops := cmd["operations"].([]map[string]any)
	if len(ops) != 2 {
		t.Fatalf("ops = %#v", ops)
	}
	for i, op := range ops {
		if op["op"] != "insert_note" {
			t.Fatalf("op %d missing insert_note: %#v", i, op)
		}
		if op["length"] != 0.95 {
			t.Fatalf("op %d length not normalized: %#v", i, op)
		}
	}
	preview := PreviewCommand(spec, cmd)
	if strings.Contains(preview, "<unknown>") || !strings.Contains(preview, "1. insert_note pitch=51 start=0 length=0.95 velocity=84") {
		t.Fatalf("preview =\n%s", preview)
	}
}

func TestMidiPatchNormalizesOperationAliasesAndNestedNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{
			"operations": []any{
				map[string]any{
					"operation": "insert",
					"note": map[string]any{
						"pitch":    60,
						"start":    0.0,
						"duration": 1.0,
						"velocity": 100,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	ops := cmd["operations"].([]map[string]any)
	if len(ops) != 1 || ops[0]["op"] != "insert_note" || ops[0]["pitch"] != 60 || ops[0]["length"] != 1.0 {
		t.Fatalf("ops = %#v", ops)
	}
	preview := PreviewCommand(spec, cmd)
	if strings.Contains(preview, "<unknown>") || !strings.Contains(preview, "1. insert_note pitch=60 start=0 length=1 velocity=100") {
		t.Fatalf("preview =\n%s", preview)
	}
}

func TestMidiPatchInfersSingleTopLevelInsertNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{
			"pitch":    60,
			"start":    0.0,
			"length":   1.0,
			"velocity": 100,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "apply_midi_note_patch" || cmd["clip_id"] != "clip_b" || cmd["track_id"] != "1010" || cmd["time_unit"] != "beats" {
		t.Fatalf("cmd = %+v", cmd)
	}
	ops := operationRowsFromAny(cmd["operations"])
	if len(ops) != 1 || ops[0]["op"] != "insert_note" || ops[0]["pitch"] != 60 || ops[0]["start"] != 0.0 || ops[0]["length"] != 1.0 {
		t.Fatalf("ops = %#v", ops)
	}
	preview := PreviewCommand(spec, cmd)
	if !strings.Contains(preview, "1. insert_note pitch=60 start=0 length=1 velocity=100") {
		t.Fatalf("preview did not show inferred top-level insert:\n%s", preview)
	}
}

func TestMidiReplaceRegionNormalizesTopLevelReplacementNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.replace_region",
		Args: map[string]any{
			"region_start":  0.0,
			"region_length": 1.0,
			"pitch":         65,
			"note_start":    0.0,
			"note_length":   0.5,
			"velocity":      90,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	ops := operationRowsFromAny(cmd["operations"])
	if len(ops) != 1 || ops[0]["op"] != "replace_region" || ops[0]["start"] != 0.0 || ops[0]["length"] != 1.0 {
		t.Fatalf("ops = %#v", ops)
	}
	notes := operationRowsFromAny(ops[0]["notes"])
	if len(notes) != 1 || notes[0]["pitch"] != 65 || notes[0]["start"] != 0.0 || notes[0]["length"] != 0.5 || notes[0]["velocity"] != 90 {
		t.Fatalf("notes = %#v ops=%#v", notes, ops)
	}
}

func TestMidiReplaceRegionNormalizesNestedReplacementNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.replace_region",
		Args: map[string]any{
			"region_start":  0.0,
			"region_length": 1.0,
			"replacement_note": map[string]any{
				"pitch":          65,
				"relative_start": 0.25,
				"duration":       0.5,
				"velocity":       90,
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	ops := operationRowsFromAny(cmd["operations"])
	if len(ops) != 1 || ops[0]["op"] != "replace_region" || ops[0]["start"] != 0.0 || ops[0]["length"] != 1.0 {
		t.Fatalf("ops = %#v", ops)
	}
	notes := operationRowsFromAny(ops[0]["notes"])
	if len(notes) != 1 || notes[0]["pitch"] != 65 || notes[0]["start"] != 0.25 || notes[0]["length"] != 0.5 || notes[0]["velocity"] != 90 {
		t.Fatalf("notes = %#v ops=%#v", notes, ops)
	}
}

func TestMidiPatchPreviewIsReadable(t *testing.T) {
	spec := tools.CommandSpec{
		CommandName: "apply_midi_note_patch",
		Description: "Apply a beat-based MIDI note patch to a clip.",
		RiskLevel:   tools.RiskConfirm,
	}
	preview := PreviewCommand(spec, map[string]any{
		"clip_id":   "clip_a",
		"time_unit": "beats",
		"operations": []map[string]any{
			{"op": "insert_note", "pitch": 64, "start": 0.5, "length": 0.25, "velocity": 88},
			{"op": "quantize_region", "start": 0.0, "length": 4.0, "grid": "1/16"},
		},
	})
	for _, want := range []string{"apply_midi_note_patch [confirm]", "Clip clip_a", "1. insert_note pitch=64 start=0.5 length=0.25 velocity=88", "2. quantize_region start=0 length=4 grid=1/16"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview missing %q in:\n%s", want, preview)
		}
	}
}

func TestLegacyMidiBulkNormalizesDurationDefaultsBeats(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.add_notes_bulk",
		Args: map[string]any{
			"notes": []any{
				map[string]any{"pitch": 55, "start": 0.0, "duration": 0.75, "velocity": 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "add_midi_notes_bulk" || cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" || cmd["time_unit"] != "beats" {
		t.Fatalf("cmd = %+v", cmd)
	}
	notes := cmd["notes"].([]map[string]any)
	if len(notes) != 1 || notes[0]["length"] != 0.75 || notes[0]["duration"] != 0.75 {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestLegacyMidiAddNormalizesSingleTopLevelNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.legacy_add_notes",
		Args: map[string]any{
			"pitch":    67,
			"start":    1.0,
			"duration": 0.5,
			"velocity": 88,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "add_midi_notes" || cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" || cmd["time_unit"] != "beats" {
		t.Fatalf("cmd = %+v", cmd)
	}
	notes := operationRowsFromAny(cmd["notes"])
	if len(notes) != 1 || notes[0]["pitch"] != 67 || notes[0]["start"] != 1.0 || notes[0]["length"] != 0.5 || notes[0]["velocity"] != 88 {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestLegacyMidiMutateNormalizesSingleTopLevelNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.legacy_mutate_notes",
		Args: map[string]any{
			"note_id":  "note_a",
			"velocity": 72,
			"duration": 0.25,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "mutate_midi_notes" || cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" || cmd["time_unit"] != "beats" {
		t.Fatalf("cmd = %+v", cmd)
	}
	notes := operationRowsFromAny(cmd["notes"])
	if len(notes) != 1 || notes[0]["id"] != "note_a" || notes[0]["length"] != 0.25 || notes[0]["velocity"] != 72 {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestLegacyMidiDeleteNormalizesSingleNoteID(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.legacy_delete_notes",
		Args: map[string]any{"note_id": "note_a"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "delete_midi_notes" || cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	ids := stringSliceFromAny(cmd["note_ids"])
	if len(ids) != 1 || ids[0] != "note_a" {
		t.Fatalf("note_ids = %#v cmd=%+v", ids, cmd)
	}
}

func TestLegacyMidiPublicResultSyncsMidiUI(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.legacy_add_notes",
		Args: map[string]any{"clip_id": "clip_a", "notes": []any{map[string]any{"pitch": 67, "start": 1.0, "length": 0.5}}},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"clip_id": "clip_a", "track_id": "1007"},
		map[string]any{
			"status":         "ok",
			"clip_id":        "clip_a",
			"track_id":       "1007",
			"inserted_count": 1,
			"notes": []any{
				map[string]any{"id": "note_legacy", "pitch": 67, "start": 1.0, "length": 0.5, "velocity": 88},
			},
		},
	)
	if result["ui_action"] != "midi_note_patch" || result["clip_id"] != "clip_a" || result["track_id"] != "1007" || result["inserted_count"] != 1 {
		t.Fatalf("result = %+v", result)
	}
	notes := operationRowsFromAny(result["notes"])
	if len(notes) != 1 || notes[0]["id"] != "note_legacy" {
		t.Fatalf("notes = %#v result=%+v", notes, result)
	}
}

func TestLegacyMidiBulkRejectsUnknownExplicitClipID(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.add_notes_bulk",
		Args: map[string]any{
			"clip_id": "1010",
			"notes": []any{
				map[string]any{"pitch": 55, "start": 0.0, "duration": 0.75, "velocity": 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	err = h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1007",
	})
	if err == nil {
		t.Fatalf("expected unknown clip_id error, cmd=%+v", cmd)
	}
	if !strings.Contains(err.Error(), `clip_id "1010" is not present`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLegacyMidiPreviewIsReadable(t *testing.T) {
	spec := tools.CommandSpec{
		CommandName: "add_midi_notes_bulk",
		Description: "Add many MIDI notes to a clip.",
		RiskLevel:   tools.RiskConfirm,
	}
	preview := PreviewCommand(spec, map[string]any{
		"clip_id":   "clip_a",
		"time_unit": "beats",
		"notes": []map[string]any{
			{"pitch": 55, "start": 0.0, "duration": 0.75, "velocity": 100},
		},
	})
	for _, want := range []string{"add_midi_notes_bulk [confirm]", "Clip clip_a", "1. note pitch=55 start=0 velocity=100 duration=0.75"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview missing %q in:\n%s", want, preview)
		}
	}
	if strings.Contains(preview, `"notes"`) {
		t.Fatalf("preview should be readable, got raw JSON:\n%s", preview)
	}
}

func TestMidiPatchPublicResultIncludesUIAction(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{"clip_id": "clip_a", "operations": []any{map[string]any{"op": "quantize_region", "start": 0.0, "length": 1.0, "grid": "1/16"}}},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"clip_id": "clip_a", "track_id": "1007"},
		map[string]any{"status": "ok", "clip_id": "clip_a", "quantized_count": 3},
	)
	if result["ui_action"] != "midi_note_patch" || result["clip_id"] != "clip_a" || result["track_id"] != "1007" || result["quantized_count"] != 3 {
		t.Fatalf("result = %+v", result)
	}
}

func TestMidiPatchPublicResultPreservesInsertedNotes(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{
			"clip_id": "clip_a",
			"operations": []any{
				map[string]any{"op": "insert_note", "pitch": 60, "start": 0.0, "length": 1.0, "velocity": 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"clip_id": "clip_a", "track_id": "1007"},
		map[string]any{
			"status":            "ok",
			"clip_id":           "clip_a",
			"track_id":          "1007",
			"inserted_count":    1,
			"inserted_note_ids": []any{"note_a"},
			"notes": []any{
				map[string]any{"id": "note_a", "pitch": 60, "start": 0.0, "length": 1.0, "velocity": 100},
			},
		},
	)
	if result["ui_action"] != "midi_note_patch" || result["inserted_count"] != 1 {
		t.Fatalf("result = %+v", result)
	}
	ids := stringSliceFromAny(result["inserted_note_ids"])
	notes := operationRowsFromAny(result["notes"])
	if len(ids) != 1 || ids[0] != "note_a" {
		t.Fatalf("inserted ids = %#v result=%+v", ids, result)
	}
	if len(notes) != 1 || notes[0]["id"] != "note_a" || notes[0]["pitch"] != 60 {
		t.Fatalf("notes = %#v result=%+v", notes, result)
	}
}

func TestMidiClipCreatePublicResultSelectsNewClip(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.create_clip",
		Args: map[string]any{"track_id": "1007"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"track_id": "1007"},
		map[string]any{"status": "ok", "clip_id": "clip_new"},
	)
	created := result["created_clip_ids"].([]string)
	if result["ui_action"] != "select_clip" || result["clip_id"] != "clip_new" || result["new_clip_id"] != "clip_new" {
		t.Fatalf("result = %+v", result)
	}
	if result["track_id"] != "1007" || result["target_track_id"] != "1007" || len(created) != 1 || created[0] != "clip_new" {
		t.Fatalf("create metadata = %+v", result)
	}
}

func TestResolveCloneClipInfersPlayheadTarget(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{"clip_id": "clip_a"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "把这个音频复制到播放头",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       6.25,
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["new_start"] != 6.25 || cmd["time_unit"] != "seconds" || cmd["target_track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveCloneClipInfersStartAfterSource(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{"clip_id": "clip_a"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "复制选中的clip到后面",
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["source_clip_id"] != "clip_a" || cmd["target_track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["new_start"] != 2.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestResolveCloneClipRequiresStartWhenUnknownClipBounds(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{"clip_id": "external_clip"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	err = h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_track_id": "1007",
	})
	if err == nil {
		t.Fatalf("expected missing new_start error, cmd=%+v", cmd)
	}
}

func TestResolveImportMediaFromSelectedLibraryAndTrack(t *testing.T) {
	audioPath := writeTempAudioFile(t, "Loop A.wav")
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.import_media_to_track",
		Args: map[string]any{},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id":          "1007",
		"selected_library_file_path": audioPath,
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1007" || cmd["file_path"] != audioPath {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["media_type"] != "audio" || cmd["mode"] != "non_destructive" || cmd["start_time"] != 0.0 {
		t.Fatalf("defaults = %+v", cmd)
	}
}

func TestResolveImportAudioPathAliases(t *testing.T) {
	audioPath := writeTempAudioFile(t, "Kick.wav")
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.import_audio",
		Args: map[string]any{
			"path":            audioPath,
			"target_track_id": "1010",
			"start_time":      3.5,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1010" || cmd["file_path"] != audioPath || cmd["offset_time"] != 3.5 {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveImportMediaSearchesLibraryPlaces(t *testing.T) {
	root := t.TempDir()
	audioPath := filepath.Join(root, "Deep Kick Loop.wav")
	if err := os.WriteFile(audioPath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write temp audio: %v", err)
	}
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.import_media_to_track",
		Args: map[string]any{"asset_query": "kick loop"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1007",
		"library_places":    []any{root},
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["file_path"] != audioPath || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestPublicResultSanitizesProjectState(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1002", "track_name": "Arranger", "track_type": "track", "is_audio_track": false},
			map[string]any{"track_id": "1007", "track_name": "Track 1", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)

	result := h.publicResult(tools.CommandSpec{CommandName: "get_project_state"}, nil, map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1002"},
			map[string]any{"track_id": "1007"},
		},
	})

	if result["status"] != "ok" || result["track_count"] != 1 {
		t.Fatalf("result counts = %+v", result)
	}
	if _, ok := result["engine_track_count"]; ok {
		t.Fatalf("public result leaked engine_track_count: %+v", result)
	}
	if _, ok := result["internal_track_count"]; ok {
		t.Fatalf("public result leaked internal_track_count: %+v", result)
	}
	tracks := result["tracks"].([]map[string]any)
	if len(tracks) != 1 || tracks[0]["track_id"] != "1007" {
		t.Fatalf("tracks = %+v", tracks)
	}
}

func shadowProjectWithClips() *shadow.Project {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Drums",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{"id": "clip_a", "name": "Loop A", "start_seconds": 0.0, "length_seconds": 2.0},
				},
			},
			map[string]any{
				"track_id":       "1010",
				"track_name":     "Bass",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{"id": "clip_b", "name": "Loop B", "start_seconds": 4.0, "length_seconds": 2.0},
				},
			},
		},
	})
	return project
}

func writeTempAudioFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write temp audio: %v", err)
	}
	return path
}
