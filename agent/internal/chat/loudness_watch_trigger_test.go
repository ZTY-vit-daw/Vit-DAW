package chat

// LOUDNESS-WATCH-TRIGGER-1 wiring tests: the three after-processing hooks
// (T1 settle bridge, T3 loop context assembly, T2 harness piggyback), the
// settle-receipt-before-verdict discipline, and the watch failure isolation.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/loudnesswatch"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/projectstore"
)

// fakeWatchHook replaces the adapter for call-path tests: it records every
// trigger and can block on demand to prove backgrounding.
type fakeWatchHook struct {
	mu        sync.Mutex
	settles   []string
	changes   []string
	piggyback int
	block     chan struct{}
}

func (f *fakeWatchHook) CheckSettle(projectUUID, projectRevision, conclusionID string) {
	f.mu.Lock()
	f.settles = append(f.settles, projectUUID+"|"+projectRevision+"|"+conclusionID)
	f.mu.Unlock()
	if f.block != nil {
		<-f.block
	}
}

func (f *fakeWatchHook) CheckProjectChange(change map[string]any) {
	f.mu.Lock()
	f.changes = append(f.changes, firstStringFromMap(change, "project_revision", "revision"))
	f.mu.Unlock()
	if f.block != nil {
		<-f.block
	}
}

func (f *fakeWatchHook) PiggybackCheck(cmd map[string]any, covered []map[string]any, revision string) {
	f.mu.Lock()
	f.piggyback++
	f.mu.Unlock()
}

func (f *fakeWatchHook) Configure(args map[string]any) (map[string]any, error) {
	return map[string]any{"status": "ok"}, nil
}

func (f *fakeWatchHook) Status() map[string]any    { return map[string]any{} }
func (f *fakeWatchHook) Disclosure() map[string]any { return map[string]any{} }

func settleTestLoop() *freeStateReasoningLoop {
	loop := &freeStateReasoningLoop{
		ConversationID: "conv-lw", GoalID: "goal-lw", RunID: "run-lw", LoopID: "loop-lw",
	}
	loop.LatestProjectChange = map[string]any{
		"project_uuid": "proj-lw", "checkpoint_ref": "ckpt-lw", "project_revision": "rev-lw",
	}
	loop.Experiment = &experiment.Turn{ID: "turn-lw"}
	return loop
}

func TestSettleHookFiresWatchCheck(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	projectRoot := t.TempDir()
	if _, _, err := projectstore.Activate(filepath.Join(projectRoot, "song.vitproj"), "proj-lw"); err != nil {
		t.Fatalf("activate projectstore: %v", err)
	}
	fake := &fakeWatchHook{}
	server := &Server{harness: harness.New(nil, nil, nil)}
	server.harness.SetLoudnessWatchHook(fake)

	server.recordFreeStateSettlement(settleTestLoop(), mixboard.FreeStateConclusionKindExperimentSettle, "improved", "summary", nil, "")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		fake.mu.Lock()
		count := len(fake.settles)
		fake.mu.Unlock()
		if count == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.settles) != 1 {
		t.Fatalf("settle did not fire the watch check: %+v", fake.settles)
	}
	if fake.settles[0] != "proj-lw|rev-lw|fsc_turn-lw_experiment_settle" {
		t.Fatalf("settle trigger identity drifted: %q", fake.settles[0])
	}
}

func TestSettleReceiptPrecedesVerdict(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	projectRoot := t.TempDir()
	roots, _, err := projectstore.Activate(filepath.Join(projectRoot, "song.vitproj"), "proj-lw")
	if err != nil {
		t.Fatalf("activate projectstore: %v", err)
	}
	block := make(chan struct{})
	fake := &fakeWatchHook{block: block}
	server := &Server{harness: harness.New(nil, nil, nil)}
	server.harness.SetLoudnessWatchHook(fake)

	// The check blocks until released. recordFreeStateSettlement must still
	// return and the settle conclusion must already be durable: the receipt
	// closes before the verdict can exist (the backgrounding discipline).
	done := make(chan struct{})
	go func() {
		server.recordFreeStateSettlement(settleTestLoop(), mixboard.FreeStateConclusionKindExperimentSettle, "improved", "summary", nil, "")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("settle path blocked on the watch check (watch must be backgrounded)")
	}
	board, err := mixboard.NewStore("").ReadFreeStateConclusionBoard("proj-lw")
	if err != nil {
		t.Fatalf("read conclusion board: %v", err)
	}
	if board.ConclusionCount != 1 {
		t.Fatalf("settle conclusion not persisted while the watch was still running: %+v", board)
	}
	close(block)
	_ = roots
}

func TestSettleWatchFailureNeverRollsBackSettlement(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	projectRoot := t.TempDir()
	if _, _, err := projectstore.Activate(filepath.Join(projectRoot, "song.vitproj"), "proj-lw"); err != nil {
		t.Fatalf("activate projectstore: %v", err)
	}
	panicking := &panickingWatchHook{}
	server := &Server{harness: harness.New(nil, nil, nil)}
	server.harness.SetLoudnessWatchHook(panicking)

	server.recordFreeStateSettlement(settleTestLoop(), mixboard.FreeStateConclusionKindExperimentSettle, "improved", "summary", nil, "")
	// Give the panicking goroutine a moment; the settle record must survive.
	time.Sleep(50 * time.Millisecond)
	board, err := mixboard.NewStore("").ReadFreeStateConclusionBoard("proj-lw")
	if err != nil {
		t.Fatalf("read conclusion board: %v", err)
	}
	if board.ConclusionCount != 1 {
		t.Fatalf("watch panic unwound the settlement: %+v", board)
	}
}

type panickingWatchHook struct{ fakeWatchHook }

func (p *panickingWatchHook) CheckSettle(projectUUID, projectRevision, conclusionID string) {
	panic("watch blew up")
}

func TestSettleWithoutWatcherStaysSilent(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	projectRoot := t.TempDir()
	if _, _, err := projectstore.Activate(filepath.Join(projectRoot, "song.vitproj"), "proj-lw"); err != nil {
		t.Fatalf("activate projectstore: %v", err)
	}
	// A server with a harness but no watch config yet: the lazily built
	// adapter is registered but the disabled config gate keeps every check
	// silent — and nothing lands on disk.
	server := &Server{harness: harness.New(nil, nil, nil)}
	server.recordFreeStateSettlement(settleTestLoop(), mixboard.FreeStateConclusionKindExperimentSettle, "improved", "summary", nil, "")
	time.Sleep(50 * time.Millisecond)
	verdictPath := loudnessWatchVerdictStorePath()
	if _, err := os.Stat(verdictPath); !os.IsNotExist(err) {
		t.Fatalf("unconfigured watch wrote a verdict store: %v (%s)", err, verdictPath)
	}
}

func TestSettleWithEnabledWatchRecordsVerdict(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	projectRoot := t.TempDir()
	roots, _, err := projectstore.Activate(filepath.Join(projectRoot, "song.vitproj"), "proj-lw")
	if err != nil {
		t.Fatalf("activate projectstore: %v", err)
	}
	config := loudnesswatch.Normalize(loudnesswatch.Config{ProfileSource: "builtin:ebu_r128"})
	config.Enabled = true
	server := &Server{harness: harness.New(nil, nil, nil)}
	server.mu.Lock()
	server.loudnessWatchConfig = &config
	server.mu.Unlock()

	// Seed the feature snapshot with one fresh in-band row and one breach row.
	snapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"loudness_summaries": []any{
			map[string]any{"track_id": "t1", "clip_id": "c1", "integrated_lufs": -23.0, "sample_peak_dbfs": -3.0, "status": "ready", "quality_status": "ready"},
			map[string]any{"track_id": "t2", "clip_id": "c2", "integrated_lufs": -12.0, "sample_peak_dbfs": -3.0, "status": "ready", "quality_status": "ready"},
		},
	}
	snapshotDir := filepath.Dir(mixboard.FeatureSnapshotPath(map[string]any{}))
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatalf("mkdir snapshot dir: %v", err)
	}
	snapshotData, _ := json.Marshal(snapshot)
	if err := os.WriteFile(mixboard.FeatureSnapshotPath(map[string]any{}), snapshotData, 0o644); err != nil {
		t.Fatalf("seed feature snapshot: %v", err)
	}

	server.recordFreeStateSettlement(settleTestLoop(), mixboard.FreeStateConclusionKindExperimentSettle, "improved", "summary", nil, "")

	verdictPath := filepath.Join(snapshotDir, "loudness_watch_verdicts.json")
	deadline := time.Now().Add(2 * time.Second)
	var store loudnesswatch.VerdictStoreFile
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(verdictPath)
		if err == nil {
			_ = json.Unmarshal(data, &store)
			if len(store.Verdicts) > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(store.Verdicts) == 0 {
		t.Fatalf("settle-triggered verdict never landed in %s", verdictPath)
	}
	record := store.Verdicts[0]
	if record.TriggerKind != loudnesswatch.TriggerKindSettle {
		t.Fatalf("verdict trigger kind: %+v", record)
	}
	if record.VerdictKind != loudnesswatch.VerdictBreach {
		t.Fatalf("expected breach from the seeded rows, got %+v", record.Verdict)
	}
	if record.FreshRows != 2 {
		t.Fatalf("fresh row accounting: %+v", record)
	}
	_ = roots
}

func TestProjectChangeHookFiresOnLoopContextAssembly(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	fake := &fakeWatchHook{}
	server := &Server{harness: harness.New(nil, nil, nil)}
	server.harness.SetLoudnessWatchHook(fake)
	server.freeStateLoops = map[string]freeStateReasoningLoop{}

	loop := settleTestLoop()
	loop.SchemaVersion = freeStateReasoningLoopSchema
	loop.OriginalIntent = "把人声推前一点"
	loop.Status = "reasoning"
	server.storeFreeStateLoop(*loop)

	requestContext := map[string]any{
		"free_state_reasoning_loop": freeStateLoopMap(*loop),
	}
	_, ok := server.prepareFreeStateReasoningContext("conv-lw", "继续", requestContext)
	if !ok {
		t.Fatalf("context assembly rejected the active loop")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		fake.mu.Lock()
		count := len(fake.changes)
		fake.mu.Unlock()
		if count == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.changes) != 1 {
		t.Fatalf("loop context assembly did not fire the project-change check: %+v", fake.changes)
	}
	if fake.changes[0] != "rev-lw" {
		t.Fatalf("project-change revision drifted: %q", fake.changes[0])
	}
}

func TestProjectChangeHookSilentWithoutChange(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	fake := &fakeWatchHook{}
	server := &Server{harness: harness.New(nil, nil, nil)}
	server.harness.SetLoudnessWatchHook(fake)

	loop := settleTestLoop()
	loop.SchemaVersion = freeStateReasoningLoopSchema
	loop.OriginalIntent = "把人声推前一点"
	loop.LatestProjectChange = nil
	loop.Status = "reasoning"
	server.storeFreeStateLoop(*loop)
	_, ok := server.prepareFreeStateReasoningContext("conv-lw", "继续", map[string]any{
		"free_state_reasoning_loop": freeStateLoopMap(*loop),
	})
	if !ok {
		t.Fatalf("context assembly rejected the active loop")
	}
	time.Sleep(50 * time.Millisecond)
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.changes) != 0 {
		t.Fatalf("project-change check fired without a change map: %+v", fake.changes)
	}
}

func TestConfigureRoutesThroughChatHandler(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)

	server := &Server{harness: harness.New(nil, nil, nil)}
	hook := server.loudnessWatchHook()
	if hook == nil {
		t.Fatalf("watch hook not built")
	}
	result, err := hook.Configure(map[string]any{"enabled": true, "profile_source": "builtin:ebu_r128"})
	if err != nil {
		t.Fatalf("configure through the agent-loop face failed: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("configure result: %+v", result)
	}
	server.mu.Lock()
	config := server.loudnessWatchConfigCopyLocked()
	server.mu.Unlock()
	if config == nil || !config.Enabled {
		t.Fatalf("configure did not reach the chat runtime state: %+v", config)
	}
	if _, err := hook.Configure(map[string]any{"profile_source": "builtin:unknown"}); err == nil {
		t.Fatalf("unknown builtin id must fail closed on the agent-loop face")
	}
}
