package genshell

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/genapi"
	"vit-daw-agent/internal/genctx"
	"vit-daw-agent/internal/logx"
)

// DefaultPollInterval is the 2s queue-poll cadence (件二 §4: polling beats
// PUB events — at-least-once with self-heal; the delay is negligible against
// minute-scale generation jobs).
const DefaultPollInterval = 2 * time.Second

// sweepStates are the in-flight states an agent restart interrupts (件二 §7).
var sweepStates = map[string]bool{
	StateRunning:  true,
	"downloading": true,
	StateLanding:  true,
}

// Runner is the background execution leg. Exactly one Runner lives per agent
// process (started at chat server startup); the kernel queue plus the
// aigc_register_job upsert face are its only kernel surfaces.
type Runner struct {
	send     KernelCommand
	generate GenerateFunc
	log      *logx.Logger

	pollInterval time.Duration

	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	startOnce sync.Once

	mu       sync.Mutex
	enabled  bool
	inflight map[string]*inflightJob
	stats    Stats
}

// Stats is the management-plane status payload (secret-free).
type Stats struct {
	Started          bool    `json:"started"`
	Enabled          bool    `json:"enabled"`
	PollIntervalSecs float64 `json:"poll_interval_seconds"`
	Polls            int     `json:"polls"`
	Claimed          int     `json:"claimed"`
	Done             int     `json:"done"`
	Failed           int     `json:"failed"`
	Cancelled        int     `json:"cancelled"`
	Interrupted      int     `json:"interrupted"`
	InFlight         int     `json:"in_flight"`
	LastPollAt       string  `json:"last_poll_at,omitempty"`
	LastPollError    string  `json:"last_poll_error,omitempty"`
}

// inflightJob tracks one claimed job across the execution goroutine. A
// placeholder entry (request nil) marks "claiming" so the next tick cannot
// double-claim while the claim's kernel round-trips are in flight.
type inflightJob struct {
	jobID      string
	cancel     context.CancelFunc
	request    *SubmitRequest
	record     jobRecord
	cancelSeen bool
}

// jobRecord carries the whole-record fields every aigc_register_job upsert
// must repeat (whole-record replacement semantics — losing them would wipe
// the audit anchor and flip the ghost).
type jobRecord struct {
	NodeID      string
	GhostState  string
	RequestHash string
	Source      string
}

// NewRunner builds a runner around the kernel command face. A nil send face
// yields a runner that refuses to start (embedded/test harnesses); the
// management tools report that state honestly.
func NewRunner(send KernelCommand, logger *logx.Logger) *Runner {
	return &Runner{
		send:         send,
		generate:     genapi.Generate,
		log:          logger,
		pollInterval: DefaultPollInterval,
		enabled:      true,
		inflight:     map[string]*inflightJob{},
	}
}

// SetGenerateFunc swaps the generation seam (tests only; nil restores the
// real genapi.Generate).
func (r *Runner) SetGenerateFunc(f GenerateFunc) {
	if f == nil {
		f = genapi.Generate
	}
	r.generate = f
}

// SetPollInterval overrides the cadence (tests only; before Start).
func (r *Runner) SetPollInterval(d time.Duration) {
	if d > 0 {
		r.pollInterval = d
	}
}

// Start launches the runner goroutine: one orphan sweep, then the poll loop.
// It is the product-host single wiring point (chat server Start); tests and
// embedded callers that never call Start get no goroutine.
func (r *Runner) Start() {
	if r == nil || r.send == nil {
		return
	}
	r.startOnce.Do(func() {
		r.ctx, r.cancel = context.WithCancel(context.Background())
		r.done = make(chan struct{})
		r.mu.Lock()
		r.stats.Started = true
		r.mu.Unlock()
		go func() {
			defer close(r.done)
			r.sweepOrphans(r.ctx)
			ticker := time.NewTicker(r.pollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-r.ctx.Done():
					return
				case <-ticker.C:
					r.PollOnce(r.ctx)
				}
			}
		}()
	})
}

// Stop halts the loop and cancels in-flight jobs (process shutdown face).
func (r *Runner) Stop() {
	if r == nil || r.cancel == nil {
		return
	}
	r.cancel()
	<-r.done
	r.mu.Lock()
	for _, job := range r.inflight {
		if job.cancel != nil {
			job.cancel()
		}
	}
	r.inflight = map[string]*inflightJob{}
	r.mu.Unlock()
}

// Enable toggles the management-plane pause (gen_shell.stop / start): while
// disabled the runner keeps polling (cancel observation must not stall) but
// claims nothing.
func (r *Runner) Enable(on bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.enabled = on
	r.mu.Unlock()
}

// Enabled reports the management-plane switch.
func (r *Runner) Enabled() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enabled
}

// Status returns the secret-free management payload.
func (r *Runner) Status() Stats {
	if r == nil {
		return Stats{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.stats
	st.InFlight = len(r.inflight)
	st.Enabled = r.enabled
	st.PollIntervalSecs = r.pollInterval.Seconds()
	return st
}

// ---------------------------------------------------------------- kernel I/O

func replyStatus(reply map[string]any) string {
	if reply == nil {
		return ""
	}
	s, _ := reply["status"].(string)
	return strings.TrimSpace(s)
}

func sendOK(reply map[string]any, err error) bool {
	return err == nil && replyStatus(reply) == "ok"
}

// pendingAnswer fetches and decodes the queue (H3 shape: jobs carry
// job_id/state/cancel_requested/request; request survives only while the
// kernel service holds the payload in memory).
func (r *Runner) pendingAnswer(ctx context.Context) (*PendingAnswer, error) {
	reply, err := r.send(ctx, map[string]any{"cmd": "gen_shell.pending"})
	if err != nil {
		return nil, err
	}
	if !sendOK(reply, nil) {
		return nil, fmt.Errorf("gen_shell.pending status=%s", replyStatus(reply))
	}
	answer := &PendingAnswer{}
	if raw, ok := reply["jobs"]; ok && raw != nil {
		encoded, merr := json.Marshal(raw)
		if merr != nil {
			return nil, fmt.Errorf("encode gen_shell.pending jobs: %w", merr)
		}
		if err := json.Unmarshal(encoded, &answer.Jobs); err != nil {
			return nil, fmt.Errorf("decode gen_shell.pending jobs: %w", err)
		}
	}
	return answer, nil
}

// jobRecordsFromState reads the whole-record fields the upserts must carry.
func (r *Runner) jobRecordsFromState(ctx context.Context) (map[string]jobRecord, error) {
	reply, err := r.send(ctx, map[string]any{"cmd": "get_project_state"})
	if err != nil || !sendOK(reply, nil) {
		return nil, fmt.Errorf("get_project_state failed (err=%v status=%s)", err, replyStatus(reply))
	}
	out := map[string]jobRecord{}
	jobs, _ := reply["jobs"].([]any)
	for _, item := range jobs {
		job, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := stringField(job, "job_id")
		if id == "" {
			continue
		}
		out[id] = jobRecord{
			NodeID:      stringField(job, "node_id"),
			GhostState:  stringField(job, "ghost_state"),
			RequestHash: stringField(job, "request_hash"),
			Source:      stringField(job, "source"),
		}
	}
	return out, nil
}

// upsertJob pushes one job-state transition through the aigc_register_job
// face with the whole-record fields repeated (H4 precedent).
func (r *Runner) upsertJob(ctx context.Context, jobID string, rec jobRecord, state, statusMessage string) error {
	reply, err := r.send(ctx, map[string]any{
		"cmd":            "aigc_register_job",
		"job_id":         jobID,
		"node_id":        rec.NodeID,
		"job_state":      state,
		"ghost_state":    rec.GhostState,
		"status_message": statusMessage,
		"request_hash":   rec.RequestHash,
		"source":         rec.Source,
	})
	if err != nil || !sendOK(reply, nil) {
		return fmt.Errorf("aigc_register_job upsert %s->%s failed (err=%v status=%s)", jobID, state, err, replyStatus(reply))
	}
	return nil
}

// terminalContext detaches a one-shot terminal upsert from a cancelled job
// context: the cancelled/cancel-during-failure transitions must still reach
// the kernel after the job's own context is gone.
func terminalContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}

// ---------------------------------------------------------------- poll loop

// PollOnce runs one queue pass; exported for tests and for an immediate
// first tick after management-plane resume.
func (r *Runner) PollOnce(ctx context.Context) {
	answer, err := r.pendingAnswer(ctx)
	r.mu.Lock()
	r.stats.Polls++
	r.stats.LastPollAt = time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		r.stats.LastPollError = err.Error()
	} else {
		r.stats.LastPollError = ""
	}
	enabled := r.enabled
	r.mu.Unlock()
	if err != nil {
		if r.log != nil {
			r.log.Warn("[genshell] poll failed: %v", err)
		}
		return
	}
	for i := range answer.Jobs {
		job := answer.Jobs[i]
		if job.JobID == "" {
			continue
		}
		switch {
		case job.State == StateCancelRequested || (job.State == StateQueued && job.CancelRequested):
			r.observeCancel(ctx, job.JobID)
		case job.State == StateQueued && enabled:
			r.claim(ctx, job)
		}
	}
}

// observeCancel cancels an in-flight execution, or terminal-cancels a job
// this process never claimed (ghost clearing on cancelled belongs to the
// kernel service).
func (r *Runner) observeCancel(ctx context.Context, jobID string) {
	r.mu.Lock()
	job, inflight := r.inflight[jobID]
	if inflight {
		job.cancelSeen = true
	}
	r.mu.Unlock()
	if inflight {
		job.cancel()
		return
	}
	records, err := r.jobRecordsFromState(ctx)
	if err != nil {
		if r.log != nil {
			r.log.Warn("[genshell] cancel record fetch failed for %s: %v", jobID, err)
		}
		return
	}
	tctx, tcancel := terminalContext(ctx)
	defer tcancel()
	if err := r.upsertJob(tctx, jobID, records[jobID], StateCancelled,
		classMessage("none", "cancelled before claim (agent runner)")); err != nil {
		if r.log != nil {
			r.log.Warn("[genshell] cancel upsert failed for %s: %v", jobID, err)
		}
		return
	}
	r.count(func(st *Stats) { st.Cancelled++ })
}

// claim takes one queued job: placeholder-register, upsert running, spawn
// execution.
func (r *Runner) claim(ctx context.Context, job PendingJob) {
	r.mu.Lock()
	if _, dup := r.inflight[job.JobID]; dup {
		r.mu.Unlock()
		return
	}
	entry := &inflightJob{jobID: job.JobID}
	r.inflight[job.JobID] = entry // claiming placeholder
	r.mu.Unlock()
	unclaim := func() {
		r.mu.Lock()
		delete(r.inflight, job.JobID)
		r.mu.Unlock()
	}

	records, err := r.jobRecordsFromState(ctx)
	if err != nil {
		if r.log != nil {
			r.log.Warn("[genshell] claim record fetch failed for %s: %v", job.JobID, err)
		}
		unclaim()
		return
	}
	rec := records[job.JobID]
	if rec.Source != "" && !strings.HasPrefix(rec.Source, JobSourcePrefix) {
		unclaim() // not a shell job (queue contract drift — leave untouched)
		return
	}

	if job.Request == nil {
		// Kernel restarted (or payload evicted): the request body lives in
		// kernel service memory until the shell chunk persistence leg (卡4)
		// lands. Honest terminal, retryable by resubmit.
		tctx, tcancel := terminalContext(ctx)
		defer tcancel()
		_ = r.upsertJob(tctx, job.JobID, rec, StateInterrupted,
			classMessage("interrupted", "request payload unavailable on queue (kernel restart?); resubmit from the shell"))
		unclaim()
		r.count(func(st *Stats) { st.Interrupted++ })
		return
	}

	if err := r.upsertJob(ctx, job.JobID, rec, StateRunning,
		classMessage("none", fmt.Sprintf("claimed by agent runner (scene=%s)", job.Request.Scene))); err != nil {
		if r.log != nil {
			r.log.Warn("[genshell] claim upsert failed for %s: %v", job.JobID, err)
		}
		unclaim()
		return
	}
	r.count(func(st *Stats) { st.Claimed++ })

	jobCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	entry.request = job.Request
	entry.record = rec
	entry.cancel = cancel
	r.mu.Unlock()
	go func() {
		defer func() {
			cancel()
			r.mu.Lock()
			delete(r.inflight, job.JobID)
			r.mu.Unlock()
		}()
		r.execute(jobCtx, entry)
	}()
}

func (r *Runner) count(f func(*Stats)) {
	r.mu.Lock()
	f(&r.stats)
	r.mu.Unlock()
}

// ---------------------------------------------------------------- execution

// execute walks one job from running to a terminal state. Every exit path
// writes exactly one terminal upsert (done / failed / cancelled); failures
// classify per 件二 §3 and never stay silent.
func (r *Runner) execute(ctx context.Context, job *inflightJob) {
	jobID := job.jobID
	req := job.request

	terminal := func(state, class, detail string) {
		tctx, tcancel := terminalContext(ctx)
		defer tcancel()
		if err := r.upsertJob(tctx, jobID, job.record, state, classMessage(class, detail)); err != nil && r.log != nil {
			r.log.Warn("[genshell] terminal upsert failed for %s (%s): %v", jobID, state, err)
		}
		switch state {
		case StateDone:
			r.count(func(st *Stats) { st.Done++ })
		case StateFailed:
			r.count(func(st *Stats) { st.Failed++ })
		case StateCancelled:
			r.count(func(st *Stats) { st.Cancelled++ })
		}
	}
	cancelled := func() bool {
		if ctx.Err() != nil {
			return true
		}
		r.mu.Lock()
		seen := job.cancelSeen
		r.mu.Unlock()
		return seen
	}

	// scene (件三 §5: explicit per call; unknown fails closed)
	scene := genctx.SceneInProject
	if strings.TrimSpace(req.Scene) != "" {
		parsed, err := genctx.ParseScene(req.Scene)
		if err != nil {
			terminal(StateFailed, "context_error", err.Error())
			return
		}
		scene = parsed
	}

	// render through the single genctx engine (profile=gen_plugin)
	rendered, rerr := r.renderPrompt(ctx, scene, req.FreeText)
	if rerr != nil {
		terminal(StateFailed, "context_error", rerr.Error())
		return
	}

	if req.RenderOnly {
		// dry-run preview (件三 §4): same render path, no provider call, no
		// landing. The rendered text rides the status message back to the
		// GUI leg (卡4 owns the echo surface).
		terminal(StateDone, "none", "render_only preview:\n"+rendered)
		return
	}

	provider, _, err := genapi.FindProvider(req.ProviderID)
	if err != nil {
		terminal(StateFailed, string(genapi.AsError(err).Class),
			fmt.Sprintf("provider %q: %v", req.ProviderID, err))
		return
	}
	if model := strings.TrimSpace(req.Model); model != "" {
		provider.Model = model
	}

	count := req.GenParams.Count
	if count <= 0 {
		count = 1
	}
	genReq := genapi.GenerationRequest{
		Prompt:          rendered,
		StyleTags:       req.GenParams.StyleTags,
		Title:           req.GenParams.Title,
		Instrumental:    req.GenParams.Instrumental,
		DurationSeconds: req.GenParams.DurationSeconds,
		Count:           count,
		// TempoBPM / MusicalKey deliberately zero: project features enter
		// through the snapshot-injected auto blocks only (件二 §2).
	}

	result, err := r.generate(ctx, provider, genReq)
	if err != nil {
		if cancelled() {
			terminal(StateCancelled, "none", "cancelled during generation (provider spend may already exist)")
			return
		}
		gerr := genapi.AsError(err)
		terminal(StateFailed, string(gerr.Class), gerr.Error())
		return
	}
	if cancelled() {
		terminal(StateCancelled, "none", "cancelled after generation (artifacts left on disk; no landing)")
		return
	}

	// artifact split: only WAV lands in v1 (件二 §3 — non-WAV products are
	// neither silently transcoded nor landed).
	var wav, other []genapi.Artifact
	for _, a := range result.Artifacts {
		if strings.EqualFold(a.Format, "wav") {
			wav = append(wav, a)
		} else {
			other = append(other, a)
		}
	}
	otherNote := artifactNote("unsupported_format_for_landing", other)

	if scene != genctx.SceneInProject {
		// Exempt scenes produce zero project writes (04 §3 landing
		// criterion); artifact paths come back as the receipt.
		terminal(StateDone, "none",
			fmt.Sprintf("scene=%s products stay off the timeline; artifacts: %s", scene, artifactPaths(result.Artifacts)))
		return
	}
	if serr := genctx.GuardWriteCommand(scene, "bridge_ingest_generated_asset"); serr != nil {
		terminal(StateFailed, "context_error", serr.Error())
		return
	}
	if len(wav) == 0 {
		detail := "no WAV artifact to land"
		if otherNote != "" {
			detail = otherNote
		}
		terminal(StateFailed, "unsupported_format_for_landing", detail)
		return
	}

	// landing leg (fast path: direct ingest, 件二 §6)
	if err := r.upsertJob(ctx, jobID, job.record, StateLanding,
		classMessage("none", "landing artifacts")); err != nil && r.log != nil {
		r.log.Warn("[genshell] landing upsert failed for %s: %v", jobID, err)
	}

	stackID := "genshell-" + sanitizeToken(jobID)
	var landed []string
	for _, artifact := range wav {
		if cancelled() {
			terminal(StateCancelled, "none",
				fmt.Sprintf("cancelled mid-landing (already landed: %s)", strings.Join(landed, ", ")))
			return
		}
		absPath := artifact.LocalPath
		if expanded, aerr := filepath.Abs(absPath); aerr == nil {
			absPath = expanded
		}
		reply, ierr := r.send(ctx, map[string]any{
			"cmd":           "bridge_ingest_generated_asset",
			"file_path":     absPath,
			"track_id":      req.TrackID,
			"start_time":    req.StartTime,
			"job_id":        jobID,
			"bucket":        "generated",
			"take_stack_id": stackID,
		})
		if ierr != nil || !sendOK(reply, nil) {
			terminal(StateFailed, "landing_error",
				fmt.Sprintf("ingest %s failed (err=%v status=%s); artifacts remain on disk: %s",
					filepath.Base(absPath), ierr, replyStatus(reply), artifactPaths(result.Artifacts)))
			return
		}
		clipID := stringField(reply, "clip_id")
		if clipID == "" {
			clipID = stringField(reply, "id")
		}
		takeID := stringField(reply, "active_take_id")
		if takeID == "" {
			takeID = stringField(reply, "take_id")
		}
		landed = append(landed, fmt.Sprintf("clip_id=%s asset_ref=%s take_id=%s", clipID, stringField(reply, "asset_ref"), takeID))
	}

	detail := fmt.Sprintf("landed %d take(s) on stack %s: %s", len(landed), stackID, strings.Join(landed, "; "))
	if otherNote != "" {
		detail += "; " + otherNote
	}
	terminal(StateDone, "none", detail)
}

// renderPrompt is the single injection face (件一 §6): in_project assembles
// the snapshot from kernel project truth, exempt scenes render the user text
// alone.
func (r *Runner) renderPrompt(ctx context.Context, scene genctx.Scene, userFreeText string) (string, error) {
	profile := genctx.RenderProfile{ConsumerID: "gen_plugin"}
	if scene != genctx.SceneInProject {
		return genctx.RenderGenerationPrompt(genctx.Snapshot{
			SchemaVersion: genctx.SchemaVersion,
			Scene:         scene,
		}, userFreeText, profile)
	}
	reply, err := r.send(ctx, map[string]any{"cmd": "get_project_state"})
	if err != nil || !sendOK(reply, nil) {
		return "", fmt.Errorf("in_project needs open project truth: get_project_state failed (err=%v status=%s)", err, replyStatus(reply))
	}
	snap, err := genctx.AssembleSnapshot(reply, scene, genctx.CallParams{})
	if err != nil {
		return "", fmt.Errorf("in_project needs open project truth: %w", err)
	}
	return genctx.RenderGenerationPrompt(snap, userFreeText, profile)
}

// sweepOrphans marks in-flight shell jobs interrupted at runner start
// (件二 §7 — agent restart; the request payloads survive on the kernel side
// or in the shell chunk, resubmit is the retry).
func (r *Runner) sweepOrphans(ctx context.Context) {
	reply, err := r.send(ctx, map[string]any{"cmd": "get_project_state"})
	if err != nil || !sendOK(reply, nil) {
		if r.log != nil {
			r.log.Info("[genshell] orphan sweep skipped (kernel not reachable yet: err=%v)", err)
		}
		return
	}
	jobs, _ := reply["jobs"].([]any)
	for _, item := range jobs {
		job, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := stringField(job, "job_id")
		state := stringField(job, "job_state")
		source := stringField(job, "source")
		if id == "" || !strings.HasPrefix(source, JobSourcePrefix) || !sweepStates[state] {
			continue
		}
		rec := jobRecord{
			NodeID:      stringField(job, "node_id"),
			GhostState:  stringField(job, "ghost_state"),
			RequestHash: stringField(job, "request_hash"),
			Source:      source,
		}
		if err := r.upsertJob(ctx, id, rec, StateInterrupted,
			classMessage("interrupted", "agent runner restart swept in-flight job; resubmit from the shell")); err != nil && r.log != nil {
			r.log.Warn("[genshell] orphan sweep upsert failed for %s: %v", id, err)
			continue
		}
		r.count(func(st *Stats) { st.Interrupted++ })
		if r.log != nil {
			r.log.Info("[genshell] orphan swept: %s (%s -> interrupted)", id, state)
		}
	}
}

func artifactPaths(artifacts []genapi.Artifact) string {
	parts := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		parts = append(parts, a.LocalPath)
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, ", ")
}

func artifactNote(marker string, artifacts []genapi.Artifact) string {
	if len(artifacts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		parts = append(parts, fmt.Sprintf("%s(%s)", a.LocalPath, a.Format))
	}
	return fmt.Sprintf("%s: %s", marker, strings.Join(parts, ", "))
}

var tokenUnsafe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func sanitizeToken(s string) string {
	out := strings.Trim(tokenUnsafe.ReplaceAllString(strings.TrimSpace(s), "-"), "-")
	if len(out) > 96 {
		out = out[:96]
	}
	return out
}
