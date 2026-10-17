package genshell

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/genapi"
)

// ---------------------------------------------------------------- fakes

type fakeKernel struct {
	mu      sync.Mutex
	sent    []map[string]any
	resolve func(cmd map[string]any) (map[string]any, error)
}

func (f *fakeKernel) send(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	f.mu.Lock()
	f.sent = append(f.sent, cmd)
	resolve := f.resolve
	f.mu.Unlock()
	if resolve == nil {
		return map[string]any{"status": "ok"}, nil
	}
	return resolve(cmd)
}

func (f *fakeKernel) commands(name string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []map[string]any{}
	for _, cmd := range f.sent {
		if s, _ := cmd["cmd"].(string); s == name {
			out = append(out, cmd)
		}
	}
	return out
}

func (f *fakeKernel) commandsWhere(name, key, value string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []map[string]any{}
	for _, cmd := range f.sent {
		if s, _ := cmd["cmd"].(string); s != name {
			continue
		}
		if v, _ := cmd[key].(string); v == value {
			out = append(out, cmd)
		}
	}
	return out
}

func waitFor(t *testing.T, pred func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if pred() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

// stateScript answers gen_shell.pending / get_project_state from mutable
// test-scripted state so tests can advance the queue like the kernel would.
type stateScript struct {
	mu      sync.Mutex
	pending []PendingJob
	jobs    map[string]map[string]any
}

func (s *stateScript) upsert(jobID, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[jobID]; ok {
		job["job_state"] = state
	}
	for i := range s.pending {
		if s.pending[i].JobID == jobID {
			s.pending[i].State = state
			s.pending[i].CancelRequested = state == StateCancelRequested
		}
	}
}

func (s *stateScript) reply(cmd map[string]any) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch c, _ := cmd["cmd"].(string); c {
	case "gen_shell.pending":
		jobs := []map[string]any{}
		for _, j := range s.pending {
			entry := map[string]any{"job_id": j.JobID, "state": j.State, "cancel_requested": j.CancelRequested}
			if j.Request != nil {
				raw, _ := json.Marshal(j.Request)
				var decoded map[string]any
				_ = json.Unmarshal(raw, &decoded)
				entry["request"] = decoded
			}
			jobs = append(jobs, entry)
		}
		return map[string]any{"status": "ok", "jobs": jobs, "providers": []any{}}, nil
	case "get_project_state":
		jobs := []any{}
		for _, job := range s.jobs {
			jobs = append(jobs, job)
		}
		return map[string]any{
			"status":         "ok",
			"graph_revision": "r42",
			"music_context": map[string]any{
				"tempo_bpm": 96.0, "tempo_setting_count": 1,
				"time_signature": "4/4", "time_signature_numerator": 4, "time_signature_denominator": 4,
			},
			"markers": []any{},
			"jobs":    jobs,
		}, nil
	default:
		return map[string]any{"status": "ok", "clip_id": "clip-x", "asset_ref": "asset-x", "active_take_id": "take-x"}, nil
	}
}

func scriptWithQueuedJob(jobID string, request *SubmitRequest) *stateScript {
	script := &stateScript{jobs: map[string]map[string]any{}}
	script.pending = []PendingJob{{JobID: jobID, State: StateQueued, Request: request}}
	script.jobs[jobID] = map[string]any{
		"job_id": jobID, "node_id": "node-1", "job_state": StateQueued,
		"ghost_state": "mute", "request_hash": "hash-1", "source": JobSourcePrefix + "node-1",
	}
	return script
}

func baseRequest(jobID string) *SubmitRequest {
	return &SubmitRequest{
		SchemaVersion: RequestSchemaVersion,
		JobID:         jobID,
		ShellNodeID:   "node-1",
		TrackID:       "track-1",
		StartTime:     1.5,
		Scene:         "in_project",
		ProviderID:    "sim",
		FreeText:      "a warm pad for the verse",
		GenParams:     GenParams{DurationSeconds: 8, Count: 1, StyleTags: "ambient"},
	}
}

func wavArtifact(path string) genapi.Artifact {
	return genapi.Artifact{Kind: "audio", Format: "wav", LocalPath: path, Bytes: 42}
}

// ---------------------------------------------------------------- lifecycle

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "genshell-test")
	if err != nil {
		panic(err)
	}
	cfgPath := filepath.Join(dir, "providers.json")
	cfg := `{"providers":[{"id":"sim","dialect":"sync_binary","base_url":"http://127.0.0.1:1","model":"sim-model"}]}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		panic(err)
	}
	os.Setenv("VIT_GENAPI_CONFIG", cfgPath)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func newRunner(script *stateScript, gen GenerateFunc) (*Runner, *fakeKernel) {
	kernel := &fakeKernel{resolve: script.reply}
	r := NewRunner(kernel.send, nil)
	if gen != nil {
		r.SetGenerateFunc(gen)
	}
	return r, kernel
}

// ---------------------------------------------------------------- tests

func TestClaimRenderGenerateLandDone(t *testing.T) {
	jobID := "job-happy"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	var gotPrompt, gotModel string
	r, kernel := newRunner(script, func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error) {
		gotPrompt = req.Prompt
		gotModel = provider.Model
		return &genapi.Result{ProviderID: provider.ID, Dialect: provider.Dialect, Model: provider.Model, TaskID: "task-1",
			Artifacts: []genapi.Artifact{wavArtifact("/tmp/artifact-1.wav")}}, nil
	})
	script.pending[0].Request.Model = "request-model-override"

	ctx := context.Background()
	r.sweepOrphans(ctx)
	r.PollOnce(ctx)

	waitFor(t, func() bool {
		return len(kernel.commandsWhere("aigc_register_job", "job_state", StateDone)) > 0
	}, "done upsert")

	// prompt evidence: auto blocks injected by the genctx engine (profile=gen_plugin)
	if !strings.Contains(gotPrompt, "== MUSIC CONTEXT") {
		t.Fatalf("prompt missing MUSIC CONTEXT block:\n%s", gotPrompt)
	}
	if !strings.Contains(gotPrompt, "== CONSTRAINTS") {
		t.Fatalf("prompt missing CONSTRAINTS block:\n%s", gotPrompt)
	}
	if !strings.Contains(gotPrompt, "tempo: 96.0") {
		t.Fatalf("prompt missing snapshot tempo:\n%s", gotPrompt)
	}
	if !strings.HasSuffix(strings.TrimSpace(gotPrompt), "a warm pad for the verse") {
		t.Fatalf("user free text not verbatim at the tail:\n%s", gotPrompt)
	}
	if gotModel != "request-model-override" {
		t.Fatalf("request model override not applied: %q", gotModel)
	}

	// claim upsert carries the whole-record fields (H4 face semantics)
	claim := kernel.commandsWhere("aigc_register_job", "job_state", StateRunning)
	if len(claim) != 1 {
		t.Fatalf("want 1 running upsert, got %d", len(claim))
	}
	for key, want := range map[string]string{"node_id": "node-1", "ghost_state": "mute", "request_hash": "hash-1", "source": JobSourcePrefix + "node-1"} {
		if got, _ := claim[0][key].(string); got != want {
			t.Fatalf("claim upsert %s=%q, want %q", key, got, want)
		}
	}
	// landing leg: ingest with track/start/take_stack/job from the request
	ingests := kernel.commands("bridge_ingest_generated_asset")
	if len(ingests) != 1 {
		t.Fatalf("want 1 ingest, got %d", len(ingests))
	}
	ing := ingests[0]
	if got, _ := ing["track_id"].(string); got != "track-1" {
		t.Fatalf("ingest track_id=%v", ing["track_id"])
	}
	if got, _ := ing["start_time"].(float64); got != 1.5 {
		t.Fatalf("ingest start_time=%v", ing["start_time"])
	}
	if got, _ := ing["job_id"].(string); got != jobID {
		t.Fatalf("ingest job_id=%v", ing["job_id"])
	}
	stack, _ := ing["take_stack_id"].(string)
	if !strings.HasPrefix(stack, "genshell-"+jobID) {
		t.Fatalf("ingest take_stack_id=%q", stack)
	}
	done := kernel.commandsWhere("aigc_register_job", "job_state", StateDone)[0]
	msg, _ := done["status_message"].(string)
	for _, want := range []string{"class=none", "clip_id=clip-x", "asset_ref=asset-x", stack} {
		if !strings.Contains(msg, want) {
			t.Fatalf("done status missing %q: %s", want, msg)
		}
	}
}

func TestFreeCreationZeroInjectionNoLanding(t *testing.T) {
	jobID := "job-free"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	script.pending[0].Request.Scene = "free_creation"
	var gotPrompt string
	r, kernel := newRunner(script, func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error) {
		gotPrompt = req.Prompt
		return &genapi.Result{Artifacts: []genapi.Artifact{wavArtifact("/tmp/free.wav")}}, nil
	})

	ctx := context.Background()
	r.PollOnce(ctx)
	waitFor(t, func() bool { return len(kernel.commandsWhere("aigc_register_job", "job_state", StateDone)) > 0 }, "done upsert")

	if strings.Contains(gotPrompt, "MUSIC CONTEXT") || strings.Contains(gotPrompt, "CONSTRAINTS") {
		t.Fatalf("exempt scene must inject zero auto blocks:\n%s", gotPrompt)
	}
	if !strings.Contains(gotPrompt, "a warm pad for the verse") {
		t.Fatalf("user text missing:\n%s", gotPrompt)
	}
	if got := len(kernel.commands("bridge_ingest_generated_asset")); got != 0 {
		t.Fatalf("free_creation must not land, saw %d ingests", got)
	}
	done := kernel.commandsWhere("aigc_register_job", "job_state", StateDone)[0]
	msg, _ := done["status_message"].(string)
	if !strings.Contains(msg, "/tmp/free.wav") {
		t.Fatalf("done receipt missing artifact path: %s", msg)
	}
}

func TestCancelDuringGeneration(t *testing.T) {
	jobID := "job-cancel"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	started := make(chan struct{})
	released := make(chan struct{})
	r, kernel := newRunner(script, func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error) {
		close(started)
		select {
		case <-ctx.Done():
			close(released)
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return &genapi.Result{Artifacts: []genapi.Artifact{wavArtifact("/tmp/never.wav")}}, nil
		}
	})

	ctx := context.Background()
	r.PollOnce(ctx) // claim + spawn execution
	<-started
	script.upsert(jobID, StateCancelRequested)
	r.PollOnce(ctx) // observeCancel cancels the in-flight job
	<-released

	waitFor(t, func() bool { return len(kernel.commandsWhere("aigc_register_job", "job_state", StateCancelled)) > 0 }, "cancelled upsert")
	if got := len(kernel.commands("bridge_ingest_generated_asset")); got != 0 {
		t.Fatalf("cancelled job must not land, saw %d ingests", got)
	}
	if got := len(kernel.commandsWhere("aigc_register_job", "job_state", StateDone)); got != 0 {
		t.Fatalf("cancelled job must not reach done")
	}
	cancelled := kernel.commandsWhere("aigc_register_job", "job_state", StateCancelled)[0]
	msg, _ := cancelled["status_message"].(string)
	if !strings.Contains(msg, "class=none") {
		t.Fatalf("cancel terminal should carry class=none: %s", msg)
	}
}

func TestCancelQueuedBeforeClaim(t *testing.T) {
	jobID := "job-early-cancel"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	script.upsert(jobID, StateCancelRequested)
	generated := false
	r, kernel := newRunner(script, func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error) {
		generated = true
		return &genapi.Result{Artifacts: []genapi.Artifact{wavArtifact("/tmp/x.wav")}}, nil
	})

	r.PollOnce(context.Background())
	waitFor(t, func() bool { return len(kernel.commandsWhere("aigc_register_job", "job_state", StateCancelled)) > 0 }, "cancelled upsert")
	if generated {
		t.Fatalf("queued-cancel job must not generate")
	}
	if got := len(kernel.commandsWhere("aigc_register_job", "job_state", StateRunning)); got != 0 {
		t.Fatalf("queued-cancel job must not be claimed")
	}
}

func TestAuthErrorClassification(t *testing.T) {
	jobID := "job-auth"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	r, kernel := newRunner(script, func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error) {
		return nil, &genapi.Error{Class: genapi.ClassAuthError, HTTPStatus: 401, ProviderMessage: "invalid api key"}
	})

	r.PollOnce(context.Background())
	waitFor(t, func() bool { return len(kernel.commandsWhere("aigc_register_job", "job_state", StateFailed)) > 0 }, "failed upsert")
	failed := kernel.commandsWhere("aigc_register_job", "job_state", StateFailed)[0]
	msg, _ := failed["status_message"].(string)
	if !strings.Contains(msg, "class=auth_error") {
		t.Fatalf("failure must classify auth_error: %s", msg)
	}
}

func TestOrphanSweepOnStart(t *testing.T) {
	jobID := "job-orphan"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	script.upsert(jobID, StateRunning) // in-flight from a previous agent life
	r, kernel := newRunner(script, nil)

	r.sweepOrphans(context.Background())
	if got := len(kernel.commandsWhere("aigc_register_job", "job_state", StateInterrupted)); got != 1 {
		t.Fatalf("orphan sweep must mark interrupted, got %d", got)
	}
	msg, _ := kernel.commandsWhere("aigc_register_job", "job_state", StateInterrupted)[0]["status_message"].(string)
	if !strings.Contains(msg, "class=interrupted") {
		t.Fatalf("sweep message must carry class=interrupted: %s", msg)
	}
	// queued jobs are NOT swept (still claimable)
	script2 := scriptWithQueuedJob("job-queued", baseRequest("job-queued"))
	kernel2 := &fakeKernel{resolve: script2.reply}
	r2 := NewRunner(kernel2.send, nil)
	r2.sweepOrphans(context.Background())
	if got := len(kernel2.commandsWhere("aigc_register_job", "job_state", StateInterrupted)); got != 0 {
		t.Fatalf("queued jobs must not be swept, got %d", got)
	}
}

func TestQueuedWithoutRequestInterrupted(t *testing.T) {
	jobID := "job-norequest"
	script := scriptWithQueuedJob(jobID, nil) // payload gone (kernel restart shape)
	script.jobs[jobID]["job_state"] = StateQueued
	r, kernel := newRunner(script, nil)

	r.PollOnce(context.Background())
	waitFor(t, func() bool { return len(kernel.commandsWhere("aigc_register_job", "job_state", StateInterrupted)) > 0 }, "interrupted upsert")
	if got := len(kernel.commandsWhere("aigc_register_job", "job_state", StateRunning)); got != 0 {
		t.Fatalf("payload-less job must not be claimed")
	}
}

func TestMultiCandidateTwoTakesSameStack(t *testing.T) {
	jobID := "job-multi"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	script.pending[0].Request.GenParams.Count = 2
	r, kernel := newRunner(script, func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error) {
		if req.Count != 2 {
			t.Errorf("count not mapped: %d", req.Count)
		}
		return &genapi.Result{TaskID: "task-m", Artifacts: []genapi.Artifact{
			wavArtifact("/tmp/cand-a.wav"), wavArtifact("/tmp/cand-b.wav"),
		}}, nil
	})

	r.PollOnce(context.Background())
	waitFor(t, func() bool { return len(kernel.commandsWhere("aigc_register_job", "job_state", StateDone)) > 0 }, "done upsert")
	ingests := kernel.commands("bridge_ingest_generated_asset")
	if len(ingests) != 2 {
		t.Fatalf("want 2 takes ingested, got %d", len(ingests))
	}
	stackA, _ := ingests[0]["take_stack_id"].(string)
	stackB, _ := ingests[1]["take_stack_id"].(string)
	if stackA == "" || stackA != stackB {
		t.Fatalf("candidates must share one take stack: %q vs %q", stackA, stackB)
	}
	done := kernel.commandsWhere("aigc_register_job", "job_state", StateDone)[0]
	msg, _ := done["status_message"].(string)
	if !strings.Contains(msg, "2 take(s)") {
		t.Fatalf("done message must record both takes: %s", msg)
	}
}

func TestRenderOnlyPreviewNoProviderCall(t *testing.T) {
	jobID := "job-preview"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	script.pending[0].Request.RenderOnly = true
	generated := false
	r, kernel := newRunner(script, func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error) {
		generated = true
		return &genapi.Result{Artifacts: []genapi.Artifact{wavArtifact("/tmp/x.wav")}}, nil
	})

	r.PollOnce(context.Background())
	waitFor(t, func() bool { return len(kernel.commandsWhere("aigc_register_job", "job_state", StateDone)) > 0 }, "done upsert")
	if generated {
		t.Fatalf("render_only must not call the provider")
	}
	if got := len(kernel.commands("bridge_ingest_generated_asset")); got != 0 {
		t.Fatalf("render_only must not land")
	}
	done := kernel.commandsWhere("aigc_register_job", "job_state", StateDone)[0]
	msg, _ := done["status_message"].(string)
	for _, want := range []string{"render_only preview:", "== MUSIC CONTEXT", "a warm pad for the verse"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("preview text missing %q: %s", want, msg)
		}
	}
}

func TestNonWavOnlyUnsupportedForLanding(t *testing.T) {
	jobID := "job-mid"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	r, kernel := newRunner(script, func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error) {
		return &genapi.Result{Artifacts: []genapi.Artifact{
			{Kind: "midi", Format: "mid", LocalPath: "/tmp/beats.mid"},
		}}, nil
	})

	r.PollOnce(context.Background())
	waitFor(t, func() bool { return len(kernel.commandsWhere("aigc_register_job", "job_state", StateFailed)) > 0 }, "failed upsert")
	failed := kernel.commandsWhere("aigc_register_job", "job_state", StateFailed)[0]
	msg, _ := failed["status_message"].(string)
	if !strings.Contains(msg, "class=unsupported_format_for_landing") {
		t.Fatalf("mid-only must fail as unsupported_format_for_landing: %s", msg)
	}
	if !strings.Contains(msg, "/tmp/beats.mid") {
		t.Fatalf("artifact path must stay visible: %s", msg)
	}
}

func TestLandingErrorClassified(t *testing.T) {
	jobID := "job-badland"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	kernel := &fakeKernel{}
	kernel.resolve = func(cmd map[string]any) (map[string]any, error) {
		if c, _ := cmd["cmd"].(string); c == "bridge_ingest_generated_asset" {
			return map[string]any{"status": "error", "message": "track missing"}, nil
		}
		return script.reply(cmd)
	}
	r := NewRunner(kernel.send, nil)
	r.SetGenerateFunc(func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error) {
		return &genapi.Result{Artifacts: []genapi.Artifact{wavArtifact("/tmp/land.wav")}}, nil
	})

	r.PollOnce(context.Background())
	waitFor(t, func() bool { return len(kernel.commandsWhere("aigc_register_job", "job_state", StateFailed)) > 0 }, "failed upsert")
	failed := kernel.commandsWhere("aigc_register_job", "job_state", StateFailed)[0]
	msg, _ := failed["status_message"].(string)
	if !strings.Contains(msg, "class=landing_error") {
		t.Fatalf("ingest failure must classify landing_error: %s", msg)
	}
	if !strings.Contains(msg, "/tmp/land.wav") {
		t.Fatalf("artifact must remain visible for retry: %s", msg)
	}
}

func TestManagementPauseKeepsPollingButClaimsNothing(t *testing.T) {
	jobID := "job-paused"
	script := scriptWithQueuedJob(jobID, baseRequest(jobID))
	generated := false
	r, kernel := newRunner(script, func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error) {
		generated = true
		return &genapi.Result{Artifacts: []genapi.Artifact{wavArtifact("/tmp/x.wav")}}, nil
	})

	r.Enable(false)
	r.PollOnce(context.Background())
	r.PollOnce(context.Background())
	if generated || len(kernel.commandsWhere("aigc_register_job", "job_state", StateRunning)) > 0 {
		t.Fatalf("paused runner must not claim")
	}
	if r.Status().Polls != 2 {
		t.Fatalf("paused runner must keep polling (cancels), polls=%d", r.Status().Polls)
	}
	// pause does not stall cancel observation
	script.upsert(jobID, StateCancelRequested)
	r.PollOnce(context.Background())
	waitFor(t, func() bool { return len(kernel.commandsWhere("aigc_register_job", "job_state", StateCancelled)) > 0 }, "cancelled while paused")
}

func TestSubmitConversation(t *testing.T) {
	shellsPending := map[string]any{
		"status": "ok",
		"jobs":   []any{},
		"shells": []any{map[string]any{"plugin_id": "shell-1", "instance_ready": true}},
	}
	t.Run("happy", func(t *testing.T) {
		kernel := &fakeKernel{}
		kernel.resolve = func(cmd map[string]any) (map[string]any, error) {
			if c, _ := cmd["cmd"].(string); c == "gen_shell.pending" {
				if _, hasSubmit := cmd["debug_submit"]; hasSubmit {
					sub, _ := cmd["debug_submit"].(map[string]any)
					id, _ := sub["job_id"].(string)
					return map[string]any{"status": "ok",
						"jobs": []any{map[string]any{"job_id": id, "state": "queued", "cancel_requested": false}}}, nil
				}
				return shellsPending, nil
			}
			return map[string]any{"status": "ok"}, nil
		}
		out, err := SubmitConversation(context.Background(), kernel.send, SubmitArgs{
			FreeText: "generate a warm pad", TrackID: "track-9", ProviderID: "sim",
		})
		if err != nil {
			t.Fatalf("submit failed: %v", err)
		}
		if out["queued"] != true || out["shell_node_id"] != "shell-1" {
			t.Fatalf("submit receipt wrong: %v", out)
		}
		submitted := kernel.commandsWhere("gen_shell.pending", "cmd", "gen_shell.pending")
		var payload map[string]any
		for _, cmd := range submitted {
			if s, ok := cmd["debug_submit"].(map[string]any); ok {
				payload = s
			}
		}
		if payload == nil {
			t.Fatalf("no debug_submit payload recorded")
		}
		if payload["schema_version"] != RequestSchemaVersion || payload["free_text"] != "generate a warm pad" || payload["shell_node_id"] != "shell-1" {
			t.Fatalf("payload wrong: %v", payload)
		}
	})
	t.Run("needs free text and track", func(t *testing.T) {
		kernel := &fakeKernel{}
		if _, err := SubmitConversation(context.Background(), kernel.send, SubmitArgs{TrackID: "t"}); err == nil {
			t.Fatalf("empty free_text must fail")
		}
		if _, err := SubmitConversation(context.Background(), kernel.send, SubmitArgs{FreeText: "x"}); err == nil {
			t.Fatalf("missing track_id must fail for in_project")
		}
	})
	t.Run("no shell loaded", func(t *testing.T) {
		kernel := &fakeKernel{}
		kernel.resolve = func(cmd map[string]any) (map[string]any, error) {
			return map[string]any{"status": "ok", "jobs": []any{}, "shells": []any{}}, nil
		}
		if _, err := SubmitConversation(context.Background(), kernel.send, SubmitArgs{FreeText: "x", TrackID: "t", ProviderID: "sim"}); err == nil || !strings.Contains(err.Error(), "no loaded GenShell") {
			t.Fatalf("want no-shell error, got %v", err)
		}
	})
	t.Run("submit rejected", func(t *testing.T) {
		kernel := &fakeKernel{}
		kernel.resolve = func(cmd map[string]any) (map[string]any, error) {
			if _, hasSubmit := cmd["debug_submit"]; hasSubmit {
				return map[string]any{"status": "error", "message": "job_id already submitted: x"}, nil
			}
			return shellsPending, nil
		}
		_, err := SubmitConversation(context.Background(), kernel.send, SubmitArgs{FreeText: "x", TrackID: "t", ProviderID: "sim"})
		if err == nil || !strings.Contains(err.Error(), "already submitted") {
			t.Fatalf("want reject passthrough, got %v", err)
		}
	})
}

func TestClassMessageFormat(t *testing.T) {
	if got := classMessage("auth_error", "bad key"); got != "class=auth_error bad key" {
		t.Fatalf("format: %s", got)
	}
	if got := classMessage("", ""); got != "class=none" {
		t.Fatalf("default class: %s", got)
	}
}

func TestJobIDSanitize(t *testing.T) {
	if got := sanitizeToken("job/with spaces!!"); got != "job-with-spaces" {
		t.Fatalf("sanitize: %q", got)
	}
	long := strings.Repeat("a", 300)
	if got := sanitizeToken(long); len(got) != 96 {
		t.Fatalf("sanitize cap: %d", len(got))
	}
}

func TestNewJobIDShape(t *testing.T) {
	id := NewJobID()
	if !strings.HasPrefix(id, "genshell-") || len(id) != len("genshell-")+24 {
		t.Fatalf("job id shape: %q", id)
	}
	if NewJobID() == id {
		t.Fatalf("job ids must be unique")
	}
}

func TestStartStopLifecycleWithNilSender(t *testing.T) {
	r := NewRunner(nil, nil)
	r.Start() // must no-op, not panic
	if r.Status().Started {
		t.Fatalf("nil-sender runner must not start")
	}
	r.Stop()

	kernel := &fakeKernel{}
	kernel.resolve = func(cmd map[string]any) (map[string]any, error) {
		return map[string]any{"status": "error", "message": "kernel down"}, nil
	}
	r2 := NewRunner(kernel.send, nil)
	r2.SetPollInterval(time.Hour)
	r2.Start()
	waitFor(t, func() bool { return r2.Status().Started }, "started")
	r2.PollOnce(context.Background()) // kernel-down pass: honest error, no panic
	st := r2.Status()
	if st.LastPollError == "" || st.Polls < 1 {
		t.Fatalf("kernel-down poll must record error: %+v", st)
	}
	r2.Stop()
	if r2.Status().InFlight != 0 {
		t.Fatalf("stop must clear in-flight")
	}
}
