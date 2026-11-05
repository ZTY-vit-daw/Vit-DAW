package loudnesswatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// d1SettleConfig is a valid non-realtime config for CheckWithConfig tests.
func d1SettleConfig() Config {
	return Normalize(Config{Enabled: true, TriggerMode: TriggerAfterProcessing, ProfileSource: "builtin:spotify"})
}

func TestCheckWithConfigRecordsVerdictWithoutConfiguredSwitch(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "loudness_watch_verdicts.json")
	rows := []map[string]any{
		{"track_id": "trk_a", "clip_id": "clip_1", "integrated_lufs": -8.0, "sample_peak_dbfs": -3.0},
	}
	runtime := NewRuntime(Deps{
		// Config seam reports "never configured": ordinary Check would exit
		// silently; CheckWithConfig must still evaluate.
		Config: func() (Config, bool) { return Config{}, false },
		Rows:   func() []map[string]any { return rows },
		Identity: func() string { return "proj_d1" },
		StorePath: storePath,
	})
	runtime.CheckWithConfig(TriggerContext{TriggerKind: TriggerKindD1Settle, ProjectUUID: "proj_d1", ProjectRevision: "r1", AllowBake: true}, d1SettleConfig()) // return value asserted via the store below

	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("verdict store missing after CheckWithConfig: %v", err)
	}
	store := VerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatalf("verdict store decode failed: %v", err)
	}
	if store.ChecksTotal != 1 || len(store.Verdicts) != 1 {
		t.Fatalf("expected exactly one recorded verdict, got checks=%d verdicts=%d", store.ChecksTotal, len(store.Verdicts))
	}
	record := store.Verdicts[0]
	if record.TriggerKind != TriggerKindD1Settle {
		t.Fatalf("expected trigger kind %s, got %s", TriggerKindD1Settle, record.TriggerKind)
	}
	// -8 LUFS against the spotify ceiling (-14) is a breach.
	if record.VerdictKind != VerdictBreach {
		t.Fatalf("expected breach verdict, got %s", record.VerdictKind)
	}
	if record.Verdict.ProjectUUID != "proj_d1" || record.FreshRows != 1 {
		t.Fatalf("unexpected record telemetry: %+v", record)
	}
}

func TestCheckWithConfigRejectsInvalidAndRealtimeConfigs(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "loudness_watch_verdicts.json")
	rows := []map[string]any{{"track_id": "trk_a", "clip_id": "clip_1", "integrated_lufs": -12.0}}
	runtime := NewRuntime(Deps{
		Rows:     func() []map[string]any { return rows },
		Identity: func() string { return "proj_d1" },
		StorePath: storePath,
	})
	// Realtime: stored-but-inactive capability gap — no verdict may land.
	runtime.CheckWithConfig(TriggerContext{TriggerKind: TriggerKindD1Settle}, Normalize(Config{Enabled: true, TriggerMode: TriggerRealtime, ProfileSource: "builtin:spotify"}))
	// Invalid profile: fail closed.
	runtime.CheckWithConfig(TriggerContext{TriggerKind: TriggerKindD1Settle}, Normalize(Config{Enabled: true, ProfileSource: "builtin:does_not_exist"}))
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("rejected configs must not touch the verdict store (stat err=%v)", err)
	}
}

func TestCheckBehaviorUnchangedForConfiguredPath(t *testing.T) {
	// Regression guard for the Check refactor: the configured-switch path
	// still records with the same key shape, and a disabled switch stays
	// silent zero cost.
	dir := t.TempDir()
	storePath := filepath.Join(dir, "loudness_watch_verdicts.json")
	rows := []map[string]any{{"track_id": "trk_a", "clip_id": "clip_1", "integrated_lufs": -8.0}}
	config := d1SettleConfig()
	runtime := NewRuntime(Deps{
		Config: func() (Config, bool) { return config, true },
		Rows:   func() []map[string]any { return rows },
		Identity: func() string { return "proj_d1" },
		StorePath: storePath,
	})
	runtime.Check(TriggerContext{TriggerKind: TriggerKindSettle, ProjectUUID: "proj_d1", ProjectRevision: "r1"})
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("configured Check must still record: %v", err)
	}
	store := VerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if store.Verdicts[0].TriggerKind != TriggerKindSettle || store.Verdicts[0].VerdictKind != VerdictBreach {
		t.Fatalf("configured path drifted: %+v", store.Verdicts[0])
	}
	// Disabled switch: silent, no disk write, no counter.
	config.Enabled = false
	dir2 := t.TempDir()
	runtime2 := NewRuntime(Deps{
		Config: func() (Config, bool) { return config, true },
		Rows:   func() []map[string]any { return rows },
		StorePath: filepath.Join(dir2, "loudness_watch_verdicts.json"),
	})
	runtime2.Check(TriggerContext{TriggerKind: TriggerKindSettle})
	runtime2.CheckWithConfig(TriggerContext{TriggerKind: TriggerKindD1Settle}, d1SettleConfig())
	data2, err := os.ReadFile(filepath.Join(dir2, "loudness_watch_verdicts.json"))
	if err != nil {
		t.Fatalf("CheckWithConfig must record even while the configured switch is off: %v", err)
	}
	store2 := VerdictStoreFile{}
	if err := json.Unmarshal(data2, &store2); err != nil {
		t.Fatal(err)
	}
	if store2.ChecksTotal != 1 || store2.Verdicts[0].TriggerKind != TriggerKindD1Settle {
		t.Fatalf("expected only the d1_settle record on the second store: %+v", store2)
	}
}
