package chat

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/loudnesswatch"
)

// FIX-LW-CONFIG-PERSIST-1 regression evidence.
//
// The D1 post-run context keeps a durable runtime snapshot on disk that the
// continuation scheduler replays into memory on every drive
// (runContinuationSchedulerOnce -> reloadActiveRuntimeState, tick = 250ms).
// /agent/invoke writes its HTTP response BEFORE the deferred
// syncCurrentProjectWorkspace (server.go handleInvoke: `defer` + writeJSON in
// the handler body), so an explicit loudness_watch_configure that lands in
// memory can be reverted by the scheduler's disk replay before the deferred
// persist writes it — and the next status/observe then reads enabled=true again.
// The persisted state and the reload both belong to the same project session,
// so a second request cannot tell the revert from the durable truth.
//
// These tests model the two paths named on the card:
//  1. the durable-snapshot replay (restore surface), and
//  2. the deferred sync persist (write-back surface).
//
// Both must keep an explicit in-process config selection over the stale
// persisted snapshot, in every direction (on->off and off->on).

func seedLoudnessWatchRuntimeStateForTest(t *testing.T, projectPath, projectUUID string, persisted loudnesswatch.Config) {
	t.Helper()
	state := projectAgentRuntimeState{
		SchemaVersion:       "vit_project_agent_runtime.v2",
		ProjectPath:         projectPath,
		ProjectUUID:         projectUUID,
		LoudnessWatchConfig: loudnessWatchConfigCopyForTest(persisted),
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := history.WriteAgentRuntimeState(projectPath, projectUUID, data); err != nil {
		// Windows can transiently refuse the replace-rename on a file the
		// server just wrote; retry briefly instead of failing the fixture
		// (mirrors TestSchedulerWaitsForActiveInvocationBeforeReloadingRuntimeState).
		deadline := time.Now().Add(2 * time.Second)
		for err != nil && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
			err = history.WriteAgentRuntimeState(projectPath, projectUUID, data)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func readPersistedLoudnessWatchEnabledForTest(t *testing.T, projectPath, projectUUID string) bool {
	t.Helper()
	data, err := history.ReadAgentRuntimeState(projectPath, projectUUID)
	if err != nil {
		t.Fatalf("read persisted runtime state: %v", err)
	}
	state := projectAgentRuntimeState{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode persisted runtime state: %v", err)
	}
	return state.LoudnessWatchConfig != nil && state.LoudnessWatchConfig.Enabled
}

func assertLoudnessWatchEnabledForTest(t *testing.T, s *Server, want bool, label string) {
	t.Helper()
	status := s.invokeLoudnessWatchStatus(nil)
	if status.Status != "ok" {
		t.Fatalf("%s: status failed: %+v", label, status)
	}
	got, ok := status.Result["enabled"].(bool)
	if !ok {
		t.Fatalf("%s: status has no boolean enabled: %+v", label, status.Result)
	}
	if got != want {
		t.Fatalf("%s: enabled=%t, want %t (explicit configure was reverted by the durable snapshot)", label, got, want)
	}
}

// Path 1 (restore surface): persisted enabled=true, explicit configure off, then
// the continuation scheduler's durable-snapshot replay. The replay must not
// resurrect the stale enabled=true, and the following persist must keep false.
func TestLoudnessWatchExplicitOffSurvivesSchedulerDiskReplay(t *testing.T) {
	s := newChatServerForTest(t, nil, nil, nil)
	dir := t.TempDir()
	const projectUUID = "vitproj_lw_explicit_off_replay"
	s.mu.Lock()
	s.activeWorkspacePath = dir
	s.activeWorkspaceUUID = projectUUID
	s.mu.Unlock()

	seedLoudnessWatchRuntimeStateForTest(t, dir, projectUUID, loudnesswatch.Normalize(loudnesswatch.Config{
		Enabled:       true,
		ProfileSource: "builtin:ebu_r128",
	}))
	// Workspace activation restore: the effective config starts as the
	// persisted enabled=true (the D1 post-run shape).
	if err := s.reloadActiveRuntimeState(); err != nil {
		t.Fatalf("activation restore failed: %v", err)
	}

	if resp := s.invokeLoudnessWatchConfigure(map[string]any{"enabled": false}); resp.Status != "ok" {
		t.Fatalf("configure off failed: %+v", resp)
	}

	// D1 post-run shape: the scheduler replays the durable snapshot (the
	// configure persist has not landed yet at this point of the request).
	if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
		t.Fatalf("scheduler drive failed: %v", err)
	}
	assertLoudnessWatchEnabledForTest(t, s, false, "after scheduler replay")

	// The deferred sync persist must write off, not the replayed on.
	if err := s.persistCurrentProjectWorkspaceChecked(); err != nil {
		t.Fatalf("persist after configure failed: %v", err)
	}
	if enabled := readPersistedLoudnessWatchEnabledForTest(t, dir, projectUUID); enabled {
		t.Fatal("deferred persist wrote enabled=true back over the explicit off")
	}
	if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
		t.Fatalf("second scheduler drive failed: %v", err)
	}
	assertLoudnessWatchEnabledForTest(t, s, false, "after second replay")
}

// Path 2 (deferred-sync write-back surface): the durable replay lands between
// the configure mutation and the handleInvoke deferred sync (activate+persist),
// exactly the D1 post-run request window. The delayed sync must persist the
// explicit value, not the replayed snapshot, in both directions.
func TestLoudnessWatchExplicitConfigSurvivesDeferredSyncAfterReplay(t *testing.T) {
	cases := []struct {
		name             string
		persistedEnabled bool
		explicitEnabled  bool
	}{
		{name: "explicit_off_over_stale_on", persistedEnabled: true, explicitEnabled: false},
		{name: "explicit_on_over_stale_off", persistedEnabled: false, explicitEnabled: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newChatServerForTest(t, nil, nil, nil)
			dir := t.TempDir()
			const projectUUID = "vitproj_lw_explicit_defer"
			s.mu.Lock()
			s.activeWorkspacePath = dir
			s.activeWorkspaceUUID = projectUUID
			s.mu.Unlock()

			seedLoudnessWatchRuntimeStateForTest(t, dir, projectUUID, loudnesswatch.Normalize(loudnesswatch.Config{
				Enabled:       tc.persistedEnabled,
				ProfileSource: "builtin:ebu_r128",
			}))
			// Workspace activation restore: the effective config starts as the
			// persisted value; the explicit configure below is the user intent.
			if err := s.reloadActiveRuntimeState(); err != nil {
				t.Fatalf("activation restore failed: %v", err)
			}
			if resp := s.invokeLoudnessWatchConfigure(map[string]any{"enabled": tc.explicitEnabled}); resp.Status != "ok" {
				t.Fatalf("configure failed: %+v", resp)
			}

			// Request window: the scheduler's disk replay races the explicit
			// mutation before handleInvoke's deferred sync persist.
			if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
				t.Fatalf("scheduler drive failed: %v", err)
			}
			// What handleInvoke's `defer syncCurrentProjectWorkspace` does.
			s.syncCurrentProjectWorkspace(context.Background())

			assertLoudnessWatchEnabledForTest(t, s, tc.explicitEnabled, "after deferred sync")
			if enabled := readPersistedLoudnessWatchEnabledForTest(t, dir, projectUUID); enabled != tc.explicitEnabled {
				t.Fatalf("deferred sync persisted enabled=%t, want %t (explicit configure was reverted)", enabled, tc.explicitEnabled)
			}

			// A later replay must not flip the now-persisted explicit value.
			if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
				t.Fatalf("post-persist scheduler drive failed: %v", err)
			}
			assertLoudnessWatchEnabledForTest(t, s, tc.explicitEnabled, "after post-persist replay")
		})
	}
}
