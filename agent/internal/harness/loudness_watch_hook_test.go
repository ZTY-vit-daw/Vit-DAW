package harness

// LOUDNESS-WATCH-TRIGGER-1 harness-side wiring tests: the T2 piggyback
// spawn, the additive mix.observe disclosure, the agent-loop tool cases
// (card A boundary obligation), and the capped stale-fill bake face.

import (
	"context"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/tools"
)

type recordingWatchHook struct {
	mu        sync.Mutex
	piggy     int
	covered   []map[string]any
	revision  string
	status    map[string]any
	disclose  map[string]any
	configure map[string]any
}

func (r *recordingWatchHook) CheckSettle(projectUUID, projectRevision, conclusionID string) {}
func (r *recordingWatchHook) CheckProjectChange(change map[string]any)                     {}

func (r *recordingWatchHook) PiggybackCheck(cmd map[string]any, coveredTargets []map[string]any, projectRevision string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.piggy++
	r.covered = coveredTargets
	r.revision = projectRevision
}

func (r *recordingWatchHook) Configure(args map[string]any) (map[string]any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configure = args
	return map[string]any{"status": "ok", "configured": true}, nil
}

func (r *recordingWatchHook) Status() map[string]any {
	if r.status != nil {
		return r.status
	}
	return map[string]any{"verdict_count": 0}
}

func (r *recordingWatchHook) Disclosure() map[string]any {
	if r.disclose != nil {
		return r.disclose
	}
	return map[string]any{"enabled": true}
}

func TestLoudnessWatchPiggybackSpawnsBackgroundCheck(t *testing.T) {
	h := &Harness{}
	hook := &recordingWatchHook{}
	h.SetLoudnessWatchHook(hook)

	cmd := map[string]any{"track_id": "t1", "clip_id": "c1"}
	state := map[string]any{"project_revision": "rev-9"}
	h.triggerLoudnessWatchPiggyback(cmd, state, mixboardTargetForTest("t1", "c1"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hook.mu.Lock()
		count := hook.piggy
		hook.mu.Unlock()
		if count == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	hook.mu.Lock()
	defer hook.mu.Unlock()
	if hook.piggy != 1 {
		t.Fatalf("piggyback check never fired: %d", hook.piggy)
	}
	if hook.revision != "rev-9" {
		t.Fatalf("piggyback revision not carried: %q", hook.revision)
	}
	// Single-target scope must cover the resolved clip target.
	if len(hook.covered) == 0 {
		t.Fatalf("piggyback covered targets missing")
	}
	if got := firstString(hook.covered[0], "clip_id"); got != "c1" {
		t.Fatalf("covered target drifted: %+v", hook.covered[0])
	}
}

func TestLoudnessWatchPiggybackSilentWithoutHook(t *testing.T) {
	h := &Harness{}
	// No hook registered: the spawn site must not panic and must not block.
	h.triggerLoudnessWatchPiggyback(map[string]any{}, nil, mixboardTargetForTest("t", "c"))
}

func TestLoudnessWatchPiggybackContainsPanics(t *testing.T) {
	h := &Harness{}
	h.SetLoudnessWatchHook(&panickingPiggybackHook{})
	h.triggerLoudnessWatchPiggyback(map[string]any{}, nil, mixboardTargetForTest("t", "c"))
	// The goroutine recovers; give it a beat and confirm the process lives.
	time.Sleep(50 * time.Millisecond)
}

type panickingPiggybackHook struct{ recordingWatchHook }

func (p *panickingPiggybackHook) PiggybackCheck(cmd map[string]any, covered []map[string]any, revision string) {
	panic("piggyback blew up")
}

func mixboardTargetForTest(trackID, clipID string) mixboard.TargetRef {
	return mixboard.TargetRef{Kind: "clip", ID: clipID, Label: trackID}
}

func TestLoudnessWatchInvokeCasesRouteThroughHook(t *testing.T) {
	h := &Harness{}
	spec := tools.CommandSpec{}
	cmd := map[string]any{"enabled": true}

	// Nil hook: honest unavailable error, marked handled.
	result, handled := h.invokeLocal(context.Background(), specWithName(spec, "loudness_watch_configure"), cmd, nil, "test")
	if !handled {
		t.Fatalf("configure case not handled")
	}
	if result["status"] != "error" {
		t.Fatalf("nil hook configure must error honestly: %+v", result)
	}
	result, handled = h.invokeLocal(context.Background(), specWithName(spec, "loudness_watch_status"), nil, nil, "test")
	if !handled || result["status"] != "error" {
		t.Fatalf("nil hook status must error honestly: %+v handled=%v", result, handled)
	}

	// Registered hook: both tool names (underscore and dotted aliases) route.
	hook := &recordingWatchHook{}
	h.SetLoudnessWatchHook(hook)
	result, handled = h.invokeLocal(context.Background(), specWithName(spec, "loudness_watch_configure"), cmd, nil, "test")
	if !handled || result["status"] != "ok" || result["configured"] != true {
		t.Fatalf("configure routing broke: %+v handled=%v", result, handled)
	}
	hook.mu.Lock()
	configured := hook.configure
	hook.mu.Unlock()
	if configured == nil {
		t.Fatalf("configure args never reached the hook")
	}
	result, handled = h.invokeLocal(context.Background(), specWithName(spec, "loudness.watch.status"), nil, nil, "test")
	if !handled {
		t.Fatalf("dotted status alias not handled")
	}
	if _, ok := result["verdict_count"]; !ok {
		t.Fatalf("status routing broke: %+v", result)
	}
}

func specWithName(spec tools.CommandSpec, name string) tools.CommandSpec {
	spec.CommandName = name
	return spec
}

type fakeKernelSender struct {
	mu        sync.Mutex
	commands  []map[string]any
	replyWith map[string]any
}

func (f *fakeKernelSender) SendCommand(ctx context.Context, cmd map[string]any) (map[string]any, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, cmd)
	if f.replyWith != nil {
		return f.replyWith, "", nil
	}
	return map[string]any{"status": "ok"}, "", nil
}

func TestRequestLoudnessWatchBakeIssuesCappedBackgroundCommands(t *testing.T) {
	kernel := &fakeKernelSender{}
	h := &Harness{kernel: kernel}
	targets := []map[string]any{
		{"track_id": "t1", "clip_id": "c1"},
		{"track_id": "t2", "clip_id": "c2"},
		{"track_id": "", "clip_id": ""}, // skipped: no identity
	}
	issued := h.RequestLoudnessWatchBake(targets)
	if issued != 2 {
		t.Fatalf("issued count: %d", issued)
	}
	kernel.mu.Lock()
	defer kernel.mu.Unlock()
	if len(kernel.commands) != 2 {
		t.Fatalf("kernel commands: %d", len(kernel.commands))
	}
	for _, cmd := range kernel.commands {
		if cmd["feature_type"] != "loudness_summary" {
			t.Fatalf("bake command feature type: %+v", cmd)
		}
		if cmd["source_kind"] != "loudness_watch_stale_fill" {
			t.Fatalf("bake command source kind: %+v", cmd)
		}
	}
}

func TestRequestLoudnessWatchBakeFailsSoft(t *testing.T) {
	kernel := &fakeKernelSender{replyWith: map[string]any{"status": "error", "message": "no clip"}}
	h := &Harness{kernel: kernel}
	if issued := h.RequestLoudnessWatchBake([]map[string]any{{"track_id": "t1", "clip_id": "c1"}}); issued != 0 {
		t.Fatalf("failed bake must not count as issued: %d", issued)
	}
	if h.RequestLoudnessWatchBake(nil) != 0 {
		t.Fatalf("nil targets must issue nothing")
	}
	bare := &Harness{}
	if bare.RequestLoudnessWatchBake([]map[string]any{{"track_id": "t1", "clip_id": "c1"}}) != 0 {
		t.Fatalf("kernelless harness must issue nothing")
	}
}
