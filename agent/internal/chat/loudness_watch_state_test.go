package chat

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/loudnesswatch"
	"vit-daw-agent/internal/rlm"
)

func loudnessWatchCustomForTest() *loudnesswatch.CustomThresholds {
	minLUFS := -23.5
	return &loudnesswatch.CustomThresholds{
		Integrated:      rlm.LoudnessBand{Min: &minLUFS, Max: -22.5},
		TruePeakMaxDBTP: -1,
	}
}

func loudnessWatchConfigCopyForTest(config loudnesswatch.Config) *loudnesswatch.Config {
	cloned := config
	return &cloned
}

// Test ① (card acceptance list): legacy records carry no loudness_watch_config
// field — restore must leave the watch unconfigured with the dropped counter
// at zero, and the persisted form must not gain the key (zero behavior change,
// AGENTS.md §11).
func TestLoudnessWatchAbsentFieldRestoresDisabled(t *testing.T) {
	restarted := New(nil, nil, nil)
	restarted.mu.Lock()
	restarted.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{})
	restarted.mu.Unlock()

	restarted.mu.Lock()
	config := restarted.loudnessWatchConfigCopyLocked()
	dropped := restarted.loudnessWatchRestoreDropped
	restarted.mu.Unlock()
	if config != nil || dropped != 0 {
		t.Fatalf("legacy restore must leave the watch unconfigured and clean, got %+v dropped=%d", config, dropped)
	}

	server := New(nil, nil, nil)
	server.mu.Lock()
	state := server.projectAgentRuntimeStateLocked()
	server.mu.Unlock()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, present := decoded["loudness_watch_config"]; present {
		t.Fatal("unconfigured state must not persist a loudness_watch_config key")
	}
}

// Test ② (card acceptance list): configured watch survives the full persist
// path — write (through the configure handler) -> snapshot -> on-disk round
// trip -> restore on a fresh server -> read back equal, including the custom
// pointer thresholds.
func TestLoudnessWatchConfigRoundTripThroughRestore(t *testing.T) {
	server := New(nil, nil, nil)
	resp := server.invokeLoudnessWatchConfigure(map[string]any{
		"enabled":        true,
		"trigger_mode":   "periodic",
		"profile_source": "custom",
		"custom":         *loudnessWatchCustomForTest(),
	})
	if resp.Status != "ok" {
		t.Fatalf("configure failed: %+v", resp)
	}

	server.mu.Lock()
	state := server.projectAgentRuntimeStateLocked()
	server.mu.Unlock()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	decoded := projectAgentRuntimeState{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.LoudnessWatchConfig == nil || !decoded.LoudnessWatchConfig.Enabled {
		t.Fatalf("decoded state lost the config: %+v", decoded.LoudnessWatchConfig)
	}

	restarted := New(nil, nil, nil)
	restarted.mu.Lock()
	restarted.restoreProjectAgentRuntimeStateLocked(decoded)
	restarted.mu.Unlock()

	restarted.mu.Lock()
	restored := restarted.loudnessWatchConfigCopyLocked()
	restarted.mu.Unlock()
	if restored == nil {
		t.Fatal("config did not survive restart")
	}
	if !reflect.DeepEqual(*restored, *decoded.LoudnessWatchConfig) {
		t.Fatalf("restored config differs:\n want: %+v\n got:  %+v", *decoded.LoudnessWatchConfig, *restored)
	}
	if restored.Custom == nil || restored.Custom.Integrated.Min == nil || *restored.Custom.Integrated.Min != -23.5 {
		t.Fatalf("custom min pointer lost across restart: %+v", restored.Custom)
	}
	if restored.Custom.ShortTermMax != nil || restored.Custom.MomentaryMax != nil {
		t.Fatalf("nil ST/M pointers must stay nil: %+v", restored.Custom)
	}
	if restored.PeriodicIntervalSeconds != loudnesswatch.DefaultPeriodicIntervalSeconds {
		t.Fatalf("periodic default interval lost: %+v", restored)
	}
}

// Test ③ (card acceptance list): bad schema versions and unknown enums are
// dropped and counted at restore — the watch stays disabled and the status
// face discloses the drop instead of silently rewriting the record.
func TestLoudnessWatchInvalidRecordsDropAndCount(t *testing.T) {
	badSchema := loudnesswatch.Normalize(loudnesswatch.Config{Enabled: true, ProfileSource: "builtin:ebu_r128"})
	badSchema.SchemaVersion = "loudness_watch_config.v9"
	badTrigger := loudnesswatch.Normalize(loudnesswatch.Config{Enabled: true, TriggerMode: "every_render", ProfileSource: "builtin:ebu_r128"})
	unknownBuiltin := loudnesswatch.Normalize(loudnesswatch.Config{Enabled: true, ProfileSource: "builtin:no_such_profile"})

	for name, record := range map[string]loudnesswatch.Config{"bad_schema": badSchema, "bad_trigger": badTrigger, "unknown_builtin": unknownBuiltin} {
		t.Run(name, func(t *testing.T) {
			restarted := New(nil, nil, nil)
			restarted.mu.Lock()
			restarted.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{LoudnessWatchConfig: loudnessWatchConfigCopyForTest(record)})
			restarted.mu.Unlock()

			restarted.mu.Lock()
			config := restarted.loudnessWatchConfigCopyLocked()
			dropped := restarted.loudnessWatchRestoreDropped
			restarted.mu.Unlock()
			if config != nil {
				t.Fatalf("invalid record must leave the watch disabled, got %+v", config)
			}
			if dropped != 1 {
				t.Fatalf("invalid record must count one drop, got %d", dropped)
			}

			status := restarted.invokeLoudnessWatchStatus(nil)
			if status.Status != "ok" {
				t.Fatalf("status failed: %+v", status)
			}
			if value, ok := status.Result["restore_dropped"].(int); !ok || value != 1 {
				t.Fatalf("status must disclose the dropped counter: %+v", status.Result)
			}
			if enabled, ok := status.Result["enabled"].(bool); !ok || enabled {
				t.Fatalf("status must report the watch disabled: %+v", status.Result)
			}
		})
	}
}

// Test ⑥ (card acceptance list, chat half): realtime mode survives the round
// trip and the status face discloses the honest unavailable reason.
func TestLoudnessWatchRealtimeStatusDisclosure(t *testing.T) {
	server := New(nil, nil, nil)
	resp := server.invokeLoudnessWatchConfigure(map[string]any{
		"enabled":        true,
		"trigger_mode":   "realtime",
		"profile_source": "builtin:spotify",
	})
	if resp.Status != "ok" {
		t.Fatalf("realtime configure failed: %+v", resp)
	}

	server.mu.Lock()
	state := server.projectAgentRuntimeStateLocked()
	server.mu.Unlock()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	decoded := projectAgentRuntimeState{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	restarted := New(nil, nil, nil)
	restarted.mu.Lock()
	restarted.restoreProjectAgentRuntimeStateLocked(decoded)
	restarted.mu.Unlock()

	status := restarted.invokeLoudnessWatchStatus(nil)
	realtime, ok := status.Result["realtime"].(map[string]any)
	if !ok {
		t.Fatalf("status must carry the realtime face: %+v", status.Result)
	}
	if realtime["status"] != loudnesswatch.RealtimeUnavailableStatus || realtime["reason"] != loudnesswatch.RealtimeUnavailableReason {
		t.Fatalf("realtime disclosure drifted: %+v", realtime)
	}
	if enabled := status.Result["enabled"].(bool); !enabled {
		t.Fatal("realtime config is stored as requested and must read back enabled")
	}
}

// Configure face: partial merge, fail-closed rejections, and the returned
// effective config.
func TestLoudnessWatchConfigureFace(t *testing.T) {
	server := New(nil, nil, nil)
	if resp := server.invokeLoudnessWatchConfigure(map[string]any{"enabled": true}); resp.Status != "error" {
		t.Fatalf("enabled-only on an empty config has no profile source and must fail closed: %+v", resp)
	}
	resp := server.invokeLoudnessWatchConfigure(map[string]any{
		"enabled":        true,
		"profile_source": "builtin:ebu_r128",
	})
	if resp.Status != "ok" {
		t.Fatalf("baseline configure failed: %+v", resp)
	}
	if _, ok := resp.Result["config_digest"].(string); !ok {
		t.Fatalf("configure result must pin the digest: %+v", resp.Result)
	}

	// Partial merge: flip enabled off without touching the rest.
	resp = server.invokeLoudnessWatchConfigure(map[string]any{"enabled": false})
	if resp.Status != "ok" {
		t.Fatalf("partial configure failed: %+v", resp)
	}
	config := resp.Result["config"].(loudnesswatch.Config)
	if config.Enabled || config.ProfileSource != "builtin:ebu_r128" {
		t.Fatalf("partial merge lost current values: %+v", config)
	}

	// Invalid input never mutates stored state.
	if resp := server.invokeLoudnessWatchConfigure(map[string]any{"trigger_mode": "every_render"}); resp.Status != "error" {
		t.Fatalf("unknown trigger mode must be rejected: %+v", resp)
	}
	if resp := server.invokeLoudnessWatchConfigure(map[string]any{"enabeld": true}); resp.Status != "error" || !strings.Contains(resp.Error, "enabeld") {
		t.Fatalf("unknown key must be rejected naming it: %+v", resp)
	}
	status := server.invokeLoudnessWatchStatus(nil)
	if enabled := status.Result["enabled"].(bool); enabled {
		t.Fatalf("failed configure must not mutate stored state: %+v", status.Result)
	}
}

// Recognition covers all three invoke forms (dotted tool, snake command, cmd
// in the command map) and nothing else.
func TestLoudnessWatchInvokeRecognition(t *testing.T) {
	matches := 0
	for _, tool := range []string{"loudness.watch.configure", "loudness_watch_configure", "LOUDNESS.WATCH.CONFIGURE"} {
		if _, ok := loudnessWatchInvokeCommand(harness.InvokeRequest{Tool: tool}, loudnessWatchConfigureCommand, loudnessWatchConfigureTool); ok {
			matches++
		}
	}
	if matches != 3 {
		t.Fatalf("configure tool recognition matched %d/3 forms", matches)
	}
	if _, ok := loudnessWatchInvokeCommand(harness.InvokeRequest{Tool: "loudness.watch.other"}, loudnessWatchStatusCommand, loudnessWatchStatusTool); ok {
		t.Fatal("unrelated tool must not match")
	}
	if _, ok := loudnessWatchInvokeCommand(harness.InvokeRequest{Command: map[string]any{"cmd": "loudness_watch_status"}}, loudnessWatchStatusCommand, loudnessWatchStatusTool); !ok {
		t.Fatal("cmd-form status invoke must be recognized")
	}
}
