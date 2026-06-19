package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tools"
	"vit-daw-agent/internal/workflows/plugingrabber"
)

type fakeKernelClient struct {
	replies  []map[string]any
	commands []map[string]any
}

func testMap(t *testing.T, value any) map[string]any {
	t.Helper()
	row, ok := value.(map[string]any)
	if ok {
		return row
	}
	data, err := json.Marshal(value)
	if err == nil {
		if err := json.Unmarshal(data, &row); err == nil && row != nil {
			return row
		}
	}
	if !ok {
		t.Fatalf("value is %T, want map[string]any: %+v", value, value)
	}
	return row
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

func TestResolveRackAddMacroAlias(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd":   "rack.add_macro",
			"name":  "通用宏控件",
			"value": 0.5,
		},
	})
	if err != nil {
		t.Fatalf("resolve rack.add_macro: %v", err)
	}
	if spec.CommandName != "control_add_macro" {
		t.Fatalf("command = %q, want control_add_macro", spec.CommandName)
	}
	if cmd["cmd"] != "control_add_macro" {
		t.Fatalf("resolved cmd = %v, want control_add_macro", cmd["cmd"])
	}
}

func TestInvokeRackAddMacroAliasUsesSelectedTrack(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":   "rack.add_macro",
			"name":  "通用宏控件",
			"value": 0.5,
		},
		Context: map[string]any{
			"selected_track_id": "1007",
		},
		Source: "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "needs_confirmation" {
		t.Fatalf("status = %q, want needs_confirmation", resp.Status)
	}
	if resp.CommandName != "control_add_macro" || resp.Tool != "rack.add_macro" {
		t.Fatalf("resolved response = %+v", resp)
	}
	if !strings.Contains(resp.Preview, "Macro 通用宏控件") || !strings.Contains(resp.Preview, "Track 1007") {
		t.Fatalf("preview did not include macro/track: %q", resp.Preview)
	}
}

func TestInvokeControlAddMacroConfirmedCreatesRackMacroMutation(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":   "control_add_macro",
			"name":  "Agent Macro",
			"value": 0.25,
		},
		Context: map[string]any{
			"selected_track_id": "1007",
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok; error=%q result=%+v", resp.Status, resp.Error, resp.Result)
	}
	if resp.CommandName != "control_add_macro" {
		t.Fatalf("command = %q, want control_add_macro", resp.CommandName)
	}
	if resp.Result["ui_action"] != "rack_macro_upserted" || resp.Result["kind"] != "rack_macro_upserted" {
		t.Fatalf("result did not request rack macro upsert: %+v", resp.Result)
	}
	macroID := strings.TrimSpace(fmt.Sprint(resp.Result["macro_id"]))
	if macroID == "" {
		t.Fatalf("missing macro_id: %+v", resp.Result)
	}
	macro, ok := resp.Result["macro"].(map[string]any)
	if !ok {
		t.Fatalf("macro payload missing: %+v", resp.Result)
	}
	if macro["macro_id"] != macroID || macro["track_id"] != "1007" || macro["name"] != "Agent Macro" {
		t.Fatalf("macro payload mismatch: id=%q macro=%+v", macroID, macro)
	}
	if got := fmt.Sprint(macro["value"]); got != "0.25" {
		t.Fatalf("macro value = %v, want 0.25; macro=%+v", got, macro)
	}
}

func TestInvokeControlAddBindingConfirmedCreatesRackMacroBindingMutation(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":               "control_add_binding",
			"source_node_id":    "macro_ccc50053d5f770bf",
			"target_track_id":   "1007",
			"target_plugin_id":  "1012",
			"target_param_id":   "2",
			"target_param_name": "B1 Gain",
			"target_min":        -12,
			"target_max":        12,
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok; error=%q result=%+v", resp.Status, resp.Error, resp.Result)
	}
	if resp.CommandName != "control_add_binding" {
		t.Fatalf("command = %q, want control_add_binding", resp.CommandName)
	}
	if resp.Result["ui_action"] != "rack_macro_binding_added" || resp.Result["kind"] != "rack_macro_binding_added" {
		t.Fatalf("result did not request rack macro binding: %+v", resp.Result)
	}
	if resp.Result["macro_id"] != "macro_ccc50053d5f770bf" {
		t.Fatalf("macro_id = %v", resp.Result["macro_id"])
	}
	binding, ok := resp.Result["binding"].(map[string]any)
	if !ok {
		t.Fatalf("binding payload missing: %+v", resp.Result)
	}
	if binding["track_id"] != "1007" || binding["plugin_id"] != "1012" || binding["param_id"] != "2" || binding["param_name"] != "B1 Gain" {
		t.Fatalf("binding payload mismatch: %+v", binding)
	}
}

func TestInvokeControlSetMacroValuesAppliesTrackVolumeBinding(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, nil, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":      "control_set_macro_values",
			"macro_id": "macro_volume",
			"value":    -5.25,
			"macro": map[string]any{
				"macro_id": "macro_volume",
				"name":     "轨道电平",
				"track_id": "track_1",
				"value":    -6.0,
				"min":      -60.0,
				"max":      12.0,
				"bindings": []map[string]any{{
					"control":    "track.volume",
					"track_id":   "track_1",
					"param_id":   "track.volume",
					"param_name": "轨道音量",
					"target_min": -60.0,
					"target_max": 12.0,
				}},
			},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, result=%+v error=%q", resp.Status, resp.Result, resp.Error)
	}
	if len(kernel.commands) != 1 {
		t.Fatalf("commands = %+v", kernel.commands)
	}
	cmd := kernel.commands[0]
	if cmd["cmd"] != "set_volume" || cmd["track_id"] != "track_1" || fmt.Sprint(cmd["db"]) != "-5.25" {
		t.Fatalf("kernel command = %+v", cmd)
	}
	if resp.Result["ui_action"] != "rack_macro_value_changed" {
		t.Fatalf("result = %+v", resp.Result)
	}
}

func TestInvokeControlSetMacroValuesTreatsTrackVolumeAsDBWhenBindingHasDefaultRange(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, nil, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":      "control_set_macro_values",
			"macro_id": "macro_volume",
			"value":    -1.5,
			"macro": map[string]any{
				"macro_id": "macro_volume",
				"name":     "Track volume",
				"track_id": "track_1",
				"value":    0.0,
				"min":      -60.0,
				"max":      12.0,
				"unit":     "dB",
				"bindings": []map[string]any{{
					"control":    "track.volume",
					"track_id":   "track_1",
					"param_id":   "track.volume",
					"target_min": 0.0,
					"target_max": 1.0,
				}},
			},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" || len(kernel.commands) != 1 {
		t.Fatalf("resp=%+v commands=%+v", resp, kernel.commands)
	}
	if got := fmt.Sprint(kernel.commands[0]["db"]); got != "-1.5" {
		t.Fatalf("db = %s, want -1.5; command=%+v", got, kernel.commands[0])
	}
}

func TestMixTickProposeApplyRollbackTrackGainAdjust(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, shadow.New(nil), nil)
	h.kernel = kernel
	h.shadow.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true, "volume_db": -6.0},
		},
	})

	propose, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.propose_tick",
		Args: map[string]any{
			"operation":      "track_gain_adjust",
			"track_id":       "track_1",
			"delta_db":       3.75,
			"observation_id": "obs_1",
		},
		Source: "test",
	})
	if err != nil {
		t.Fatalf("propose returned error: %v", err)
	}
	if propose.Status != "ok" {
		t.Fatalf("propose status = %q result=%+v error=%q", propose.Status, propose.Result, propose.Error)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("propose should not send kernel command: %+v", kernel.commands)
	}
	tickID := strings.TrimSpace(fmt.Sprint(propose.Result["tick_id"]))
	if tickID == "" || fmt.Sprint(propose.Result["delta_db"]) != "2" || fmt.Sprint(propose.Result["after_db"]) != "-4" {
		t.Fatalf("unexpected proposal: %+v", propose.Result)
	}

	rejected, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": tickID},
	})
	if err == nil || rejected.Status != "error" || !strings.Contains(rejected.Error, "confirmation") {
		t.Fatalf("unconfirmed apply should fail with confirmation error: resp=%+v err=%v", rejected, err)
	}

	applied, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": tickID, "confirmation": true},
	})
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if applied.Status != "ok" {
		t.Fatalf("apply status = %q result=%+v error=%q", applied.Status, applied.Result, applied.Error)
	}
	volumeCommands := testCommandsByName(kernel.commands, "set_volume")
	if len(volumeCommands) != 1 {
		t.Fatalf("set_volume commands = %+v all=%+v", volumeCommands, kernel.commands)
	}
	if cmd := volumeCommands[0]; cmd["cmd"] != "set_volume" || cmd["track_id"] != "track_1" || fmt.Sprint(cmd["db"]) != "-4" {
		t.Fatalf("apply kernel command = %+v", cmd)
	}

	duplicate, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": tickID, "confirmation": true},
	})
	if err == nil || duplicate.Status != "error" || !strings.Contains(duplicate.Error, "already applied") {
		t.Fatalf("duplicate apply should fail: resp=%+v err=%v", duplicate, err)
	}

	rolledBack, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.rollback_tick",
		Args: map[string]any{"tick_id": tickID},
	})
	if err != nil {
		t.Fatalf("rollback returned error: %v", err)
	}
	if rolledBack.Status != "ok" {
		t.Fatalf("rollback status = %q result=%+v error=%q", rolledBack.Status, rolledBack.Result, rolledBack.Error)
	}
	volumeCommands = testCommandsByName(kernel.commands, "set_volume")
	if len(volumeCommands) != 2 {
		t.Fatalf("set_volume commands after rollback = %+v all=%+v", volumeCommands, kernel.commands)
	}
	if cmd := volumeCommands[1]; cmd["cmd"] != "set_volume" || cmd["track_id"] != "track_1" || fmt.Sprint(cmd["db"]) != "-6" {
		t.Fatalf("rollback kernel command = %+v", cmd)
	}
}

func TestMixTickProposeApplyRollbackTrackPanAdjust(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, shadow.New(nil), nil)
	h.kernel = kernel
	h.shadow.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true, "pan": 0.0},
		},
	})

	propose, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.propose_tick",
		Args: map[string]any{
			"operation":      "track_pan_adjust",
			"track_id":       "track_1",
			"delta_pan":      0.30,
			"observation_id": "obs_1",
		},
		Source: "test",
	})
	if err != nil {
		t.Fatalf("propose returned error: %v", err)
	}
	if propose.Status != "ok" {
		t.Fatalf("propose status = %q result=%+v error=%q", propose.Status, propose.Result, propose.Error)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("propose should not send kernel command: %+v", kernel.commands)
	}
	tickID := strings.TrimSpace(fmt.Sprint(propose.Result["tick_id"]))
	if tickID == "" || fmt.Sprint(propose.Result["delta_pan"]) != "0.15" || fmt.Sprint(propose.Result["after_pan"]) != "0.15" {
		t.Fatalf("unexpected proposal: %+v", propose.Result)
	}

	applied, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": tickID, "confirmation": true},
	})
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if applied.Status != "ok" {
		t.Fatalf("apply status = %q result=%+v error=%q", applied.Status, applied.Result, applied.Error)
	}
	panCommands := testCommandsByName(kernel.commands, "set_pan")
	if len(panCommands) != 1 {
		t.Fatalf("set_pan commands = %+v all=%+v", panCommands, kernel.commands)
	}
	if cmd := panCommands[0]; cmd["cmd"] != "set_pan" || cmd["track_id"] != "track_1" || fmt.Sprint(cmd["pan"]) != "0.15" {
		t.Fatalf("apply kernel command = %+v", cmd)
	}

	rolledBack, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.rollback_tick",
		Args: map[string]any{"tick_id": tickID},
	})
	if err != nil {
		t.Fatalf("rollback returned error: %v", err)
	}
	if rolledBack.Status != "ok" {
		t.Fatalf("rollback status = %q result=%+v error=%q", rolledBack.Status, rolledBack.Result, rolledBack.Error)
	}
	panCommands = testCommandsByName(kernel.commands, "set_pan")
	if len(panCommands) != 2 {
		t.Fatalf("set_pan commands after rollback = %+v all=%+v", panCommands, kernel.commands)
	}
	if cmd := panCommands[1]; cmd["cmd"] != "set_pan" || cmd["track_id"] != "track_1" || fmt.Sprint(cmd["pan"]) != "0" {
		t.Fatalf("rollback kernel command = %+v", cmd)
	}
}

func TestMixTickProposeApplyTrackPanSet(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, shadow.New(nil), nil)
	h.kernel = kernel
	h.shadow.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true, "pan": 0.25},
		},
	})

	propose, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.propose_tick",
		Args: map[string]any{
			"operation": "track_pan_set",
			"track_id":  "track_1",
			"pan":       -0.25,
		},
		Source: "test",
	})
	if err != nil {
		t.Fatalf("propose returned error: %v", err)
	}
	if propose.Status != "ok" || fmt.Sprint(propose.Result["before_pan"]) != "0.25" || fmt.Sprint(propose.Result["after_pan"]) != "-0.25" {
		t.Fatalf("unexpected proposal: status=%q result=%+v error=%q", propose.Status, propose.Result, propose.Error)
	}
	tickID := strings.TrimSpace(fmt.Sprint(propose.Result["tick_id"]))
	applied, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": tickID, "confirmation": true},
	})
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if applied.Status != "ok" {
		t.Fatalf("apply status = %q result=%+v error=%q", applied.Status, applied.Result, applied.Error)
	}
	panCommands := testCommandsByName(kernel.commands, "set_pan")
	if len(panCommands) != 1 || fmt.Sprint(panCommands[0]["pan"]) != "-0.25" {
		t.Fatalf("set_pan commands = %+v all=%+v", panCommands, kernel.commands)
	}
}

func TestMixTickApplyPanReportsObservedTrackPan(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "ok"},
		{"status": "ok", "tracks": []map[string]any{
			{"track_id": "track_1", "track_name": "Lead", "pan": -0.7},
		}},
		{"status": "ok", "tracks": []map[string]any{
			{"track_id": "track_1", "track_name": "Lead", "pan": -0.7},
		}},
		{"status": "ok", "tracks": []map[string]any{
			{"track_id": "track_1", "track_name": "Lead", "pan": -0.7},
		}},
	}}
	h := NewWithSender(kernel, shadow.New(nil), nil)
	h.shadow.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true, "pan": 0.0},
		},
	})

	propose, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.propose_tick",
		Args: map[string]any{
			"operation": "track_pan_set",
			"track_id":  "track_1",
			"pan":       -0.7,
		},
		Source: "test",
	})
	if err != nil || propose.Status != "ok" {
		t.Fatalf("propose = status=%q result=%+v err=%v", propose.Status, propose.Result, err)
	}

	applied, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": strings.TrimSpace(fmt.Sprint(propose.Result["tick_id"])), "confirmation": true},
	})
	if err != nil || applied.Status != "ok" {
		t.Fatalf("apply = status=%q result=%+v err=%v", applied.Status, applied.Result, err)
	}
	if fmt.Sprint(applied.Result["observed_pan"]) != "-0.7" || applied.Result["observed_pan_matches"] != true {
		t.Fatalf("observed pan result = %+v", applied.Result)
	}
	observed := testMap(t, applied.Result["observed_track"])
	if fmt.Sprint(observed["pan"]) != "-0.7" {
		t.Fatalf("observed track = %+v", observed)
	}
}

func TestDirectTrackPanReplyUpdatesShadowPan(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "ok", "track_id": "track_1", "track_name": "Lead", "pan": 0.75, "pan_value": 0.75},
		{"status": "ok", "tracks": []map[string]any{
			{"track_id": "track_1", "track_name": "Lead", "track_type": "audio", "is_audio_track": true, "pan": 0.0},
		}},
	}}
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "audio", "is_audio_track": true, "pan": 0.0},
		},
	})
	h := NewWithSender(kernel, project, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "track.pan",
		Args:      map[string]any{"track_id": "track_1", "pan": 0.75},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("track pan = status=%q result=%+v err=%v", resp.Status, resp.Result, err)
	}
	tracks := visibleTrackRows(h.UserStateSummary(context.Background()))
	if len(tracks) != 1 {
		t.Fatalf("tracks = %+v", tracks)
	}
	if fmt.Sprint(tracks[0]["pan"]) != "0.75" || fmt.Sprint(tracks[0]["pan_value"]) != "0.75" {
		t.Fatalf("track pan did not update shadow: %+v", tracks[0])
	}
}

func TestMixTickProposeRequiresExplicitDelta(t *testing.T) {
	h := New(nil, shadow.New(nil), nil)
	h.shadow.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true, "volume_db": -6.0},
		},
	})

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.propose_tick",
		Args: map[string]any{
			"operation":      "track_gain_adjust",
			"track_id":       "track_1",
			"observation_id": "obs_1",
		},
		Source: "test",
	})
	if err == nil {
		t.Fatalf("expected error when delta_db is missing, got resp=%+v", resp)
	}
	if resp.Status != "error" || !strings.Contains(resp.Error, "delta_db is required") {
		t.Fatalf("unexpected response: %+v err=%v", resp, err)
	}
}

func testCommandsByName(commands []map[string]any, name string) []map[string]any {
	var out []map[string]any
	for _, cmd := range commands {
		if fmt.Sprint(cmd["cmd"]) == name {
			out = append(out, cmd)
		}
	}
	return out
}

func TestInvokeControlAddBindingResolvesExistingMacroByName(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":               "control_add_binding",
			"source_node_id":    "Macro 1",
			"target_plugin_id":  "1012",
			"target_param_id":   "2",
			"target_param_name": "B1 Gain",
		},
		Context: map[string]any{
			"user_message": "把 B1 Gain 绑定到 Macro 1 上",
			"available_macro_controls": []any{
				map[string]any{"macro_id": "macro_existing", "name": "Macro 1", "track_id": "1007", "bindings": []any{}},
			},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok; error=%q result=%+v", resp.Status, resp.Error, resp.Result)
	}
	if resp.Result["macro_id"] != "macro_existing" {
		t.Fatalf("macro_id = %v, want macro_existing; result=%+v", resp.Result["macro_id"], resp.Result)
	}
	binding, ok := resp.Result["binding"].(map[string]any)
	if !ok {
		t.Fatalf("binding payload missing: %+v", resp.Result)
	}
	if binding["plugin_id"] != "1012" || binding["param_id"] != "2" || binding["track_id"] != "1007" {
		t.Fatalf("binding payload mismatch: %+v", binding)
	}
}

func TestInvokeControlAddMacroBindingIntentSelectsExistingMacro(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":   "control_add_macro",
			"name":  "Agent Macro",
			"value": 0.5,
		},
		Context: map[string]any{
			"user_message": "把 B1 Gain 绑定到 Macro 1 上",
			"available_macro_controls": []any{
				map[string]any{"macro_id": "macro_existing", "name": "Macro 1", "track_id": "1007", "bindings": []any{}},
			},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok; error=%q result=%+v", resp.Status, resp.Error, resp.Result)
	}
	if resp.Result["kind"] != "rack_macro_selected" || resp.Result["selected_existing_macro"] != true {
		t.Fatalf("result did not select existing macro: %+v", resp.Result)
	}
	if resp.Result["macro_id"] != "macro_existing" {
		t.Fatalf("macro_id = %v, want macro_existing; result=%+v", resp.Result["macro_id"], resp.Result)
	}
}

func TestInvokeControlRenameMacroResolvesExistingMacroByName(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":      "control_rename_macro",
			"macro_id": "Macro 1",
			"name":     "Filter Sweep",
		},
		Context: map[string]any{
			"user_message": "rename Macro 1 to Filter Sweep",
			"available_macro_controls": []any{
				map[string]any{"macro_id": "macro_existing", "name": "Macro 1", "track_id": "1007", "bindings": []any{}},
			},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok; error=%q result=%+v", resp.Status, resp.Error, resp.Result)
	}
	if resp.CommandName != "control_rename_macro" {
		t.Fatalf("command = %q, want control_rename_macro", resp.CommandName)
	}
	if resp.Result["kind"] != "rack_macro_renamed" || resp.Result["ui_action"] != "rack_macro_renamed" {
		t.Fatalf("result did not request macro rename: %+v", resp.Result)
	}
	if resp.Result["macro_id"] != "macro_existing" || resp.Result["name"] != "Filter Sweep" || resp.Result["old_name"] != "Macro 1" {
		t.Fatalf("rename payload mismatch: %+v", resp.Result)
	}
	macro, ok := resp.Result["macro"].(map[string]any)
	if !ok || macro["macro_id"] != "macro_existing" || macro["name"] != "Filter Sweep" {
		t.Fatalf("macro payload mismatch: %+v", resp.Result)
	}
}

func TestResolveToolAcceptsCommandName(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "get_project_state",
	})
	if err != nil {
		t.Fatalf("resolve command-name tool: %v", err)
	}
	if spec.CommandName != "get_project_state" || cmd["cmd"] != "get_project_state" {
		t.Fatalf("resolved = spec:%+v cmd:%+v", spec, cmd)
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

func TestBroadMixRequestCannotLoadPluginThroughHarness(t *testing.T) {
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(t.TempDir(), "missing_plugin_semantics.json"))
	kernel := &fakeKernelClient{}
	h := New(nil, nil, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{
			"path":     `C:\Program Files\Common Files\VST3\TDR Nova.vst3`,
			"track_id": "1007",
		},
		Context: map[string]any{
			"user_message": "我想你帮我对这段音频进行缩混可以吗",
		},
		Confirmed: true,
	})
	if err == nil {
		t.Fatal("expected broad mix plugin load to be blocked")
	}
	if resp.Status != "error" || !strings.Contains(resp.Error, "mix.request_observation") {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("blocked command reached kernel: %+v", kernel.commands)
	}
}

func TestExplicitPluginLoadStillReachesHarness(t *testing.T) {
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(t.TempDir(), "missing_plugin_semantics.json"))
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "plugin_id": "1015"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{
			"path":     `C:\Program Files\Common Files\VST3\TDR Nova.vst3`,
			"track_id": "1007",
		},
		Context: map[string]any{
			"user_message": "请直接加载 TDR Nova 插件",
		},
		Confirmed: true,
	})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("explicit plugin load should be allowed, resp=%+v err=%v", resp, err)
	}
	foundRackAdd := false
	for _, cmd := range kernel.commands {
		if cmd["cmd"] == "rack_add_node" {
			foundRackAdd = true
			break
		}
	}
	if !foundRackAdd {
		t.Fatalf("explicit plugin load did not reach kernel: %+v", kernel.commands)
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

func TestPluginLoadToRackInstrumentDefaultsToZ2FromScannedPluginInventory(t *testing.T) {
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
	if len(kernel.commands) != 1 || kernel.commands[0]["cmd"] != "scan_plugins" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
}

func TestPluginSemanticSearchMergesLivePluginSearch(t *testing.T) {
	semanticsPath := filepath.Join(t.TempDir(), "plugin_semantics.json")
	idx := pluginsemantics.Build([]map[string]any{
		{
			"name":        "Old Reverb",
			"category":    "Fx|Reverb",
			"plugin_path": `C:\Program Files\Common Files\VST3\Old Reverb.vst3`,
		},
	}, time.Unix(10, 0).UTC())
	if _, err := pluginsemantics.Save(semanticsPath, idx); err != nil {
		t.Fatalf("save semantics: %v", err)
	}
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)

	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status": "ok",
			"plugins": []any{
				map[string]any{
					"name":               "Live Compressor",
					"category":           "Fx|Dynamics",
					"file_or_identifier": "VST3-Live Compressor-1234",
				},
			},
		},
	}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	result, err := h.searchPluginSemanticIndex(map[string]any{"query": "compressor", "type": "compressor", "limit": 4})
	if err != nil {
		t.Fatalf("searchPluginSemanticIndex: %v", err)
	}
	entries, _ := result["entries"].([]pluginsemantics.Entry)
	if len(entries) == 0 {
		t.Fatalf("expected live semantic entry, result=%+v", result)
	}
	if entries[0].Name != "Live Compressor" {
		t.Fatalf("top entry = %+v", entries[0])
	}
	if len(kernel.commands) != 1 || kernel.commands[0]["cmd"] != "scan_plugins" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
}

func TestPluginSemanticBuildIndexUsesScannedPluginInventory(t *testing.T) {
	semanticsPath := filepath.Join(t.TempDir(), "plugin_semantics.json")
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status": "ok",
			"plugins": []any{
				map[string]any{
					"name":               "Live Compressor",
					"category":           "Fx|Dynamics",
					"file_or_identifier": "VST3-Live Compressor-1234",
				},
			},
		},
	}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	result, err := h.buildPluginSemanticIndex(map[string]any{"index_path": semanticsPath})
	if err != nil {
		t.Fatalf("buildPluginSemanticIndex: %v", err)
	}
	if result["plugin_count"] != 1 || result["source"] != "scan_plugins" {
		t.Fatalf("unexpected result = %+v", result)
	}
	if len(kernel.commands) != 1 || kernel.commands[0]["cmd"] != "scan_plugins" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	idx, err := pluginsemantics.Load(semanticsPath)
	if err != nil {
		t.Fatalf("load semantic index: %v", err)
	}
	if len(idx.Entries) != 1 || idx.Entries[0].Name != "Live Compressor" {
		t.Fatalf("index entries = %+v", idx.Entries)
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

func TestTranslateInsertNotePatchToLegacyCommand(t *testing.T) {
	cmd := map[string]any{
		"cmd":       "apply_midi_note_patch",
		"clip_id":   "clip_b",
		"track_id":  "1010",
		"time_unit": "beats",
		"operations": []map[string]any{
			{"op": "insert_note", "pitch": 60, "start": 0.0, "length": 1.0, "velocity": 100},
			{"action": "insert_note", "pitch": 62, "start": 1.0, "duration": 0.5, "velocity": 90},
		},
	}
	if !translateInsertNotePatchToLegacyCommand(cmd) {
		t.Fatalf("translation failed: %+v", cmd)
	}
	if cmd["cmd"] != "add_midi_notes" {
		t.Fatalf("cmd = %v", cmd["cmd"])
	}
	if _, ok := cmd["operations"]; ok {
		t.Fatalf("operations should be removed: %+v", cmd)
	}
	notes := operationRowsFromAny(cmd["notes"])
	if len(notes) != 2 {
		t.Fatalf("notes = %#v", notes)
	}
	if notes[0]["pitch"] != 60 || notes[0]["length"] != 1.0 || notes[1]["pitch"] != 62 || notes[1]["length"] != 0.5 {
		t.Fatalf("notes not translated: %#v", notes)
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

func TestInvokeMixRequestObservationWritesMixBoardWithoutKernel(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	h := New(nil, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_test",
			"round":          1,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "1007",
				"label": "Drums",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" || resp.CommandName != "mix_request_observation" {
		t.Fatalf("resp = %+v", resp)
	}
	if firstString(resp.Result, "status") != "partial" {
		t.Fatalf("result status = %+v", resp.Result)
	}
	for _, key := range []string{"board_path", "observation_path", "context_pack_path"} {
		path := firstString(resp.Result, key)
		if path == "" {
			t.Fatalf("%s missing from result: %+v", key, resp.Result)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s does not exist: %v", key, err)
		}
	}
}

func TestInvokeMixObserveAliasReturnsDigestAndCatalog(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	h := New(nil, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_observe",
			"round":          1,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "1007",
				"label": "Drums",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.CommandName != "mix_observe" {
		t.Fatalf("command = %q", resp.CommandName)
	}
	if resp.Result["digest"] == nil || resp.Result["catalog"] == nil {
		t.Fatalf("missing digest/catalog: %+v", resp.Result)
	}
}

func TestInvokeMixObserveFullProjectScopeKeepsProjectTarget(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	h := New(nil, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_full_project",
			"scope":          "full_project",
			"goal_text":      "帮我看一下整体混音",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	obs := testMap(t, resp.Result["observation"])
	target := testMap(t, obs["target_ref"])
	if target["kind"] != "project" || target["id"] != "current" {
		t.Fatalf("target = %+v", target)
	}
	digest := testMap(t, resp.Result["digest"])
	if digest["scope"] != "full_project" {
		t.Fatalf("digest = %+v", digest)
	}
	listen := testMap(t, obs["listen_scope"])
	source := testMap(t, listen["source"])
	if source["mode"] != "full_project" {
		t.Fatalf("listen scope = %+v", listen)
	}
	if _, ok := source["focus_ids"]; ok {
		t.Fatalf("full project scope should not carry focus ids: %+v", listen)
	}
}

func TestInvokeMixObserveFullProjectWritesBlockedPerTrackAcousticsWithoutKernel(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	h := New(nil, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_full_project_blocked_acoustic",
			"scope":          "full_project",
			"goal_text":      "whole mix",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	featureRequest, _ := resp.Result["feature_request"].(map[string]any)
	if firstString(featureRequest, "status") != "blocked" || firstString(featureRequest, "reason") == "" {
		t.Fatalf("feature request = %+v", featureRequest)
	}
	obs, _ := resp.Result["observation"].(mixboard.ObservationPacket)
	if got := obs.SourceCapabilities["track_waveform_envelopes"]; got != "blocked" {
		t.Fatalf("track waveform capability = %q observation=%+v", got, obs)
	}
	project := obs.ProjectPackage
	if project["acoustic_track_count"] != 2 || project["active_acoustic_track_count"] != 0 {
		t.Fatalf("project package counts = %+v", project)
	}
	tracks := mapRowsFromAny(project["tracks"])
	if len(tracks) != 2 {
		t.Fatalf("tracks = %+v", tracks)
	}
	for _, track := range tracks {
		acoustic := testMap(t, track["acoustic"])
		if firstString(acoustic, "status") != "blocked" || firstString(acoustic, "reason") == "" {
			t.Fatalf("acoustic = %+v track=%+v", acoustic, track)
		}
	}
	digest := testMap(t, resp.Result["digest"])
	available := testMap(t, digest["available_detail"])
	if available["project_track_waveforms"] != "blocked" || available["full_project_acoustic_render"] != "blocked" {
		t.Fatalf("available detail = %+v", available)
	}
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	rows := mapRowsFromAny(snapshot["track_waveform_envelopes"])
	if len(rows) != 2 {
		t.Fatalf("snapshot rows = %+v\n%s", rows, string(data))
	}
}

func TestInvokeMixObserveFocusHintResolvesVocalTrack(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "vocal_1",
				"track_name":     "Lead Vocal",
				"is_audio_track": true,
				"level_db":       -12.0,
				"peak_dbfs":      -3.0,
				"clips":          []any{map[string]any{"clip_id": "clip_v", "length_seconds": 8.0}},
			},
			map[string]any{
				"track_id":       "bass_1",
				"track_name":     "Bass",
				"is_audio_track": true,
				"level_db":       -8.0,
				"peak_dbfs":      -2.0,
				"clips":          []any{map[string]any{"clip_id": "clip_b", "length_seconds": 8.0}},
			},
		},
	})
	h := New(nil, project, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_focus_vocal",
			"scope":          "full_project_with_focus_track",
			"focus_hint":     map[string]any{"role": "vocal"},
			"goal_text":      "bring lead vocal forward",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	obs := testMap(t, resp.Result["observation"])
	target := testMap(t, obs["target_ref"])
	if target["kind"] != "track" || target["id"] != "vocal_1" {
		t.Fatalf("target = %+v", target)
	}
	listen := testMap(t, obs["listen_scope"])
	source := testMap(t, listen["source"])
	if source["mode"] != "full_project_with_focus_track" {
		t.Fatalf("listen scope = %+v", listen)
	}
	focusIDs := stringSliceFromAny(source["focus_ids"])
	if len(focusIDs) != 1 || focusIDs[0] != "vocal_1" {
		t.Fatalf("focus ids = %+v", source["focus_ids"])
	}
	projectPackage := testMap(t, obs["project_package"])
	tracks := mapRowsFromAny(projectPackage["tracks"])
	if len(tracks) != 2 || tracks[0]["focused"] != true {
		t.Fatalf("project tracks = %+v", tracks)
	}
}

func TestInvokeMixObserveFocusHintResolvesUserTrackIndex(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":         "1007",
				"track_name":       "Track 1",
				"user_track_index": 1,
				"is_audio_track":   true,
				"level_db":         -12.0,
				"peak_dbfs":        -3.0,
				"clips":            []any{map[string]any{"clip_id": "clip_1", "length_seconds": 8.0}},
			},
			map[string]any{
				"track_id":         "1012",
				"track_name":       "Track 2",
				"user_track_index": 2,
				"is_audio_track":   true,
				"level_db":         -8.0,
				"peak_dbfs":        -2.0,
				"clips":            []any{map[string]any{"clip_id": "clip_2", "length_seconds": 8.0}},
			},
		},
	})
	h := New(nil, project, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_focus_index",
			"scope":          "full_project_with_focus_track",
			"focus_hint":     map[string]any{"role": "vocal", "user_track_index": 1},
			"goal_text":      "让主唱更靠前",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	obs := testMap(t, resp.Result["observation"])
	target := testMap(t, obs["target_ref"])
	if target["kind"] != "track" || target["id"] != "1007" {
		t.Fatalf("target = %+v", target)
	}
	listen := testMap(t, obs["listen_scope"])
	source := testMap(t, listen["source"])
	focusIDs := stringSliceFromAny(source["focus_ids"])
	if len(focusIDs) != 1 || focusIDs[0] != "1007" {
		t.Fatalf("focus ids = %+v", source["focus_ids"])
	}
}

func TestInvokeMixReadAndDeriveUseStoredObservation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	h := New(nil, shadowProjectWithClips(), nil)

	observation, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_read_derive",
			"round":          1,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "1007",
				"label": "Drums",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	obsID := firstString(observation.Result, "observation_id")
	if obsID == "" {
		t.Fatalf("observation id missing: %+v", observation.Result)
	}
	readResp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.read",
		Args: map[string]any{
			"observation_id": obsID,
			"keys":           []any{"observation.digest"},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if readResp.CommandName != "mix_read" {
		t.Fatalf("command = %q", readResp.CommandName)
	}
	if readResp.Result["items"] == nil {
		t.Fatalf("read result missing items: %+v", readResp.Result)
	}
	deriveResp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.derive",
		Args: map[string]any{
			"observation_id": obsID,
			"type":           "before_after",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if deriveResp.CommandName != "mix_derive" {
		t.Fatalf("command = %q", deriveResp.CommandName)
	}
	if firstString(deriveResp.Result, "status") == "" {
		t.Fatalf("derive result missing status: %+v", deriveResp.Result)
	}
}

func TestProjectSnapshotExportFallsBackWhenKernelCommandMissing(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "error", "message": "Unknown command: project_snapshot_export"},
	}}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "project.snapshot_export",
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if firstString(resp.Result, "source") != "agent_shadow_snapshot_compat" {
		t.Fatalf("expected compat snapshot, got %+v", resp.Result)
	}
	if resp.Result["project_state"] == nil {
		t.Fatalf("project_state missing: %+v", resp.Result)
	}
}

func TestInvokeMixRequestObservationRequestsAudioFeatures(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	kernel := &fakeKernelClient{}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_feature_request",
			"round":          1,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "1007",
				"label": "Drums",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 1 {
		t.Fatalf("kernel commands = %+v resp=%+v", kernel.commands, resp.Result)
	}
	for i, feature := range []string{"waveform_envelope"} {
		if kernel.commands[i]["cmd"] != "warm_waveform_bake" || kernel.commands[i]["feature_type"] != feature {
			t.Fatalf("command[%d] = %+v", i, kernel.commands[i])
		}
		if kernel.commands[i]["track_id"] != "1007" || kernel.commands[i]["clip_id"] != "clip_a" {
			t.Fatalf("command[%d] target = %+v", i, kernel.commands[i])
		}
	}
	featureRequest, _ := resp.Result["feature_request"].(map[string]any)
	if firstString(featureRequest, "status") != "requested" {
		t.Fatalf("feature_request = %+v", featureRequest)
	}
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if !strings.Contains(string(data), `"latest_request"`) || !strings.Contains(string(data), `"waveform_envelope"`) {
		t.Fatalf("snapshot = %s", string(data))
	}
}

func TestInvokeMixRequestObservationWritesRequestedSnapshotBeforeKernelBake(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	kernel := &fakeKernelClient{}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	_, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_feature_order",
			"target_ref": map[string]any{
				"kind": "track",
				"id":   "1007",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kernel.commands) != 1 {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"status": "requested"`) || !strings.Contains(text, `"feature_type": "waveform_envelope"`) {
		t.Fatalf("snapshot was not prepared before bake fallback: %s", text)
	}
}

func TestInvokeMixRequestObservationResolvesVisibleTrackClipSource(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	kernel := &fakeKernelClient{}
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Track 1",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{
						"id":                  "1011",
						"name":                "Paper Crown",
						"file_path":           "D:\\Vit_DAW\\Paper Crown.mp3",
						"current_source_path": "D:\\Vit_DAW\\Paper Crown.mp3",
						"length_seconds":      219.384,
					},
				},
			},
		},
	})
	h := New(nil, project, nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_resolve_clip",
			"round":          1,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "track_1",
				"label": "Vocal",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 1 {
		t.Fatalf("kernel commands = %+v resp=%+v", kernel.commands, resp.Result)
	}
	if kernel.commands[0]["track_id"] != "1007" || kernel.commands[0]["clip_id"] != "1011" {
		t.Fatalf("resolved command target = %+v", kernel.commands[0])
	}
	acoustic, _ := resp.Result["acoustic_digest"].(map[string]any)
	if acoustic["clip_id"] != "1011" || acoustic["file_path"] == "" {
		t.Fatalf("acoustic digest missing resolved clip data: %+v", acoustic)
	}
}

func TestInvokeMixRequestObservationResolvesTrackAliasAmongMultipleTracks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	kernel := &fakeKernelClient{}
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":         "1007",
				"track_name":       "Track 1",
				"track_type":       "hybrid",
				"user_track_index": 1,
				"is_audio_track":   true,
				"clips": []any{
					map[string]any{
						"id":                  "1011",
						"name":                "Paper Crown",
						"current_source_path": "D:\\Vit_DAW\\Paper Crown.mp3",
						"length_seconds":      219.384,
					},
				},
			},
			map[string]any{
				"track_id":         "1012",
				"track_name":       "Track 2",
				"track_type":       "hybrid",
				"user_track_index": 2,
				"is_audio_track":   true,
				"clips":            []any{},
			},
		},
	})
	h := New(nil, project, nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_track_alias",
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "track_1",
				"label": "Vocal",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 1 {
		t.Fatalf("kernel commands = %+v resp=%+v", kernel.commands, resp.Result)
	}
	if kernel.commands[0]["track_id"] != "1007" || kernel.commands[0]["clip_id"] != "1011" {
		t.Fatalf("track alias did not resolve to audio clip source: %+v", kernel.commands[0])
	}
	board, _ := resp.Result["mixboard"].(mixboard.Board)
	if board.TargetRef.ID != "1007" {
		t.Fatalf("board target was not canonicalized: %+v", board.TargetRef)
	}
	if len(board.MixObjects) == 0 || board.MixObjects[0].ID != "1007" {
		t.Fatalf("board mix objects were not canonicalized: %+v", board.MixObjects)
	}
}

func TestInvokeMixRequestObservationUsesRenamedVisibleTrackLabel(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":         "1012",
				"track_name":       "Track 2",
				"track_type":       "hybrid",
				"user_track_index": 2,
				"is_audio_track":   true,
				"clips": []any{
					map[string]any{
						"id":                  "2012",
						"name":                "test_100hz_10s",
						"current_source_path": "D:\\Vit_DAW\\test_100hz_10s.wav",
						"length_seconds":      10.0,
					},
				},
			},
		},
	})
	project.ApplyDelta(map[string]any{
		"type":       "delta_update",
		"seq_id":     float64(1),
		"target_uid": "1012",
		"action":     "property_changed:name",
		"value":      "vocal",
	})
	kernel := &fakeKernelClient{}
	h := New(nil, project, nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_renamed_label",
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "track_2",
				"label": "Track 2",
			},
		},
		Context: map[string]any{
			"selected_track_id":   "1012",
			"selected_track_name": "Track 2",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	board, _ := resp.Result["mixboard"].(mixboard.Board)
	if board.TargetRef.ID != "1012" || board.TargetRef.Label != "vocal" {
		t.Fatalf("board target did not use renamed visible label: %+v", board.TargetRef)
	}
	acoustic, _ := resp.Result["acoustic_digest"].(map[string]any)
	if acoustic["track_name"] != "vocal" || acoustic["user_label"] != "vocal" {
		t.Fatalf("acoustic digest missing renamed label: %+v", acoustic)
	}
	obs := resp.Result["observation"].(mixboard.ObservationPacket)
	read, err := mixboard.NewStore(filepath.Join(root, "mixboard")).Read(mixboard.ReadRequest{
		ObservationID: obs.ObservationID,
		Keys:          []string{"project.tracks.summary", "track.1012.static.identity"},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := read["items"].(map[string]any)
	tracks := items["project.tracks.summary"].(map[string]any)
	rows := tracks["tracks"].([]map[string]any)
	if rows[0]["track_name"] != "vocal" || rows[0]["user_label"] != "vocal" {
		t.Fatalf("project track summary missing renamed label: %+v", rows[0])
	}
	identity := items["track.1012.static.identity"].(map[string]any)
	trackIdentity := identity["track_identity"].(map[string]any)
	if trackIdentity["track_name"] != "vocal" || trackIdentity["user_label"] != "vocal" {
		t.Fatalf("identity missing renamed label: %+v", trackIdentity)
	}
}

func TestInvokeMixRequestObservationRefreshesStaleAliasTarget(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "old_track",
				"track_name":     "Old Track",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips":          []any{},
			},
		},
	})
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status": "ok",
			"tracks": []any{
				map[string]any{
					"track_id":         "1007",
					"track_name":       "Track 1",
					"track_type":       "hybrid",
					"user_track_index": 1,
					"is_audio_track":   true,
					"clips": []any{
						map[string]any{
							"id":                  "1011",
							"name":                "Paper Crown",
							"file_path":           "D:\\Vit_DAW\\Paper Crown.mp3",
							"current_source_path": "D:\\Vit_DAW\\Paper Crown.mp3",
							"length_seconds":      219.384,
						},
					},
				},
			},
		},
		{"status": "ok"},
	}}
	h := New(nil, project, nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_stale_alias",
			"goal_text":      "auto mix",
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "track_1",
				"label": "Vocal",
			},
			"mix_objects": []any{map[string]any{
				"mode":         "target_ref",
				"kind":         "track",
				"id":           "track_1",
				"label":        "Vocal",
				"effect_scope": "track_rack",
				"source":       "target_ref",
			}},
			"listen_scope": map[string]any{
				"source": map[string]any{"focus_ids": []any{"track_1"}},
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 2 {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	if kernel.commands[0]["cmd"] != "get_project_state" {
		t.Fatalf("first command should refresh shadow: %+v", kernel.commands)
	}
	if kernel.commands[1]["cmd"] != "warm_waveform_bake" || kernel.commands[1]["track_id"] != "1007" || kernel.commands[1]["clip_id"] != "1011" {
		t.Fatalf("feature command target = %+v", kernel.commands[1])
	}
	featureRequest, _ := resp.Result["feature_request"].(map[string]any)
	resolved, _ := featureRequest["resolved_target"].(map[string]any)
	if resolved["track_id"] != "1007" || resolved["clip_id"] != "1011" {
		t.Fatalf("feature request target = %+v", featureRequest)
	}
	board, _ := resp.Result["mixboard"].(mixboard.Board)
	if board.TargetRef.ID != "1007" {
		t.Fatalf("board target = %+v", board.TargetRef)
	}
	if len(board.MixObjects) == 0 || board.MixObjects[0].ID != "1007" {
		t.Fatalf("mix objects = %+v", board.MixObjects)
	}
	if len(board.ListenScope.Source.FocusIDs) != 1 || board.ListenScope.Source.FocusIDs[0] != "1007" {
		t.Fatalf("listen scope = %+v", board.ListenScope)
	}
}

func TestWaveformFeatureCollectorWritesReadySnapshot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	cmd := map[string]any{
		"mix_session_id":   "mix_waveform_snapshot",
		"duration_seconds": 10.0,
	}
	packet := newMixboardFeatureRequestPacket(cmd, mixboard.TargetRef{Kind: "track", ID: "1007"})
	packet["status"] = "requested"
	packet["resolved_target"] = map[string]any{"track_id": "1007", "clip_id": "1011"}
	collector := &waveformFeatureCollector{
		TrackID:        "1007",
		ClipID:         "1011",
		FilePath:       "D:\\Vit_DAW\\Paper Crown.mp3",
		TotalDuration:  10,
		ExpectedTiles:  2,
		TilesSeen:      2,
		PeakAbs:        0.5,
		SumSquares:     0.01 + 0.04,
		SampleFrames:   2,
		FloatCount:     24,
		TimeSegments:   []map[string]any{{"start_seconds": 0.0, "end_seconds": 5.0, "rms": 0.1, "peak_abs": 0.3}, {"start_seconds": 5.0, "end_seconds": 10.0, "rms": 0.2, "peak_abs": 0.5}},
		LastReceivedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	writeMixboardReadyWaveformSnapshot(cmd, packet, collector.SnapshotRow(firstString(packet, "request_id")))
	result, err := mixboard.NewStore("").RequestObservation(mixboard.Request{
		MixSessionID: "mix_waveform_snapshot",
		TargetRef:    mixboard.TargetRef{Kind: "track", ID: "1007"},
		ProjectState: map[string]any{"duration_seconds": 10.0},
		Args:         cmd,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready" {
		t.Fatalf("status = %q observation=%+v", result.Status, result.Observation)
	}
	metrics := result.Observation.MixPackage["current_metrics"].(map[string]any)
	waveform := metrics["waveform"].(map[string]any)
	if waveform["peak_dbfs"] == nil || waveform["rms_dbfs"] == nil || waveform["headroom_db"] == nil {
		t.Fatalf("waveform metrics missing: %+v", waveform)
	}
	if got := result.Observation.SourceCapabilities["waveform_envelope"]; got != "ready" {
		t.Fatalf("waveform capability = %q", got)
	}
}

func TestMixRequestObservationKeepsReadyFeatureSnapshot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"waveform_envelope":{"status":"ready","track_id":"1007","clip_id":"clip_a","rms":0.2,"peak_abs":0.7},
		"band_energy_summary":{"status":"ready","track_id":"1007","clip_id":"clip_a","source":"live_level_meter_spectrum"},
		"stereo_relation_summary":{"status":"ready","track_id":"1007","clip_id":"clip_a","source":"live_level_meter_stereo","correlation_state":"stable"},
		"spectrogram_tiles":{"status":"ready","track_id":"1007","clip_id":"clip_a","tile_count_seen":2,"tile_count_expected":2}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	kernel := &fakeKernelClient{}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_ready_snapshot",
			"target_ref": map[string]any{
				"kind": "track",
				"id":   "1007",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstString(resp.Result, "status") != "ready" {
		t.Fatalf("result = %+v", resp.Result)
	}
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	waveform, _ := snapshot["waveform_envelope"].(map[string]any)
	if firstString(waveform, "status") != "ready" || firstString(waveform, "track_id") != "1007" || firstString(waveform, "clip_id") != "clip_a" {
		t.Fatalf("snapshot downgraded ready waveform: %+v\n%s", waveform, string(data))
	}
	metrics, _ := resp.Result["observation"].(mixboard.ObservationPacket)
	current, _ := metrics.MixPackage["current_metrics"].(map[string]any)
	observedWaveform, _ := current["waveform"].(map[string]any)
	if observedWaveform["peak_dbfs"] == nil || observedWaveform["rms_dbfs"] == nil || observedWaveform["headroom_db"] == nil {
		t.Fatalf("observation did not consume ready waveform metrics: %+v", observedWaveform)
	}
	if !strings.Contains(string(data), `"stereo_relation_summary"`) || !strings.Contains(string(data), `"correlation_state": "stable"`) {
		t.Fatalf("snapshot did not preserve stereo relation: %s", string(data))
	}
}

func TestMixRequestObservationTreatsWaveformReadyAsSufficient(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"waveform_envelope":{"status":"ready","track_id":"1007","clip_id":"clip_a","rms":0.2,"peak_abs":0.7,
			"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.2,"peak_abs":0.7,"energy_state":"high"}]},
		"band_energy_summary":{"status":"missing"},
		"stereo_relation_summary":{"status":"missing"},
		"spectrogram_tiles":{"status":"missing"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	kernel := &fakeKernelClient{}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_waveform_ready_sufficient",
			"target_ref": map[string]any{
				"kind": "track",
				"id":   "1007",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstString(resp.Result, "status") != "ready" {
		t.Fatalf("result = %+v", resp.Result)
	}
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	waveform, _ := snapshot["waveform_envelope"].(map[string]any)
	if firstString(waveform, "status") != "ready" {
		t.Fatalf("waveform should remain ready: %+v\n%s", waveform, string(data))
	}
	obs, _ := resp.Result["observation"].(mixboard.ObservationPacket)
	if got := obs.SourceCapabilities["waveform_envelope"]; got != "ready" {
		t.Fatalf("waveform capability = %q observation=%+v", got, obs)
	}
	if len(resp.Result["mixboard"].(mixboard.Board).OpenBlockers) != 0 {
		t.Fatalf("board blockers = %+v", resp.Result["mixboard"].(mixboard.Board).OpenBlockers)
	}
}
