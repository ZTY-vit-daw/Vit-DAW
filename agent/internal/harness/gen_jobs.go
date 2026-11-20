package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/genapi"
)

// Generation background jobs (GEN-YUE-WIRE-1, audit H6 item 5).
//
// gen.jobs.start launches the same conditioning+generation+ingest pipeline as
// gen.generate but returns immediately: submit, polling and ingest run on a
// background goroutine, and progress/failure publish through gen.jobs.status
// / gen.jobs.list instead of blocking the conversation turn. Every job
// archives the version record (original student MIDI notes, source dialect
// score, dispatched request summary, replay + audio artifact paths) as JSON
// under the jobs dir so the version list survives agent restarts.

// envGenJobsDir overrides the job archive location (tests and run-scoped
// smoke workdirs use this so ~/.vit is never touched).
const envGenJobsDir = "VIT_GENAPI_JOBS_DIR"

const (
	genJobStateStarting  = "starting"  // registered, background leg not yet running
	genJobStatePreparing = "preparing" // engine ensure + score read + conditioning map
	genJobStateRunning   = "running"   // submitted to the engine, polling
	genJobStateDone      = "done"
	genJobStateFailed    = "failed"
)

// genJobRecord is one version of a generated arrangement.
type genJobRecord struct {
	ID           string         `json:"id"`
	VersionLabel string         `json:"version_label,omitempty"`
	ProviderID   string         `json:"provider_id"`
	Dialect      string         `json:"dialect"`
	State        string         `json:"state"`
	Phase        string         `json:"phase,omitempty"` // human progress phase
	Error        string         `json:"error,omitempty"`
	ErrorClass   string         `json:"error_class,omitempty"`
	TaskID       string         `json:"task_id,omitempty"`
	StartedAt    time.Time      `json:"started_at"`
	EndedAt      *time.Time     `json:"ended_at,omitempty"`
	Dispatched   map[string]any `json:"dispatched,omitempty"`
	Actions      []string       `json:"conditioning_actions,omitempty"`
	// Version archive faces (audit H6 item 5: 原 MIDI + 源谱 + 请求 + replay).
	SourceScore *genapi.YueScoreInput `json:"source_score,omitempty"`
	SourceABC   string                `json:"source_abc,omitempty"`
	Artifacts   []map[string]any      `json:"artifacts,omitempty"`
	ReplayPath  string                `json:"replay_path,omitempty"`
	StatusTrail []string              `json:"status_trail,omitempty"`
	TrackID     string                `json:"track_id,omitempty"`
	// TwoStage archives the two-stage score-transfer receipt
	// (YUE-INSTRUMENTAL-SCORE-TRANSFER-1: pass1/pass2 job ids + the
	// vocal/ins conservation table). Present on instrumental yue_local jobs.
	TwoStage map[string]any `json:"two_stage,omitempty"`
}

// genJobStore is the in-memory index plus the JSON archive dir.
type genJobStore struct {
	mu   sync.Mutex
	dir  string
	jobs []*genJobRecord // append order == creation order
}

func defaultGenJobsDir() (string, error) {
	if p := strings.TrimSpace(os.Getenv(envGenJobsDir)); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("gen jobs: user home: %w", err)
	}
	return filepath.Join(home, ".vit", "genapi_jobs"), nil
}

// genJobStore lazily builds the harness-wide store and loads the archive.
func (h *Harness) genJobStore() (*genJobStore, error) {
	h.genJobsMu.Lock()
	defer h.genJobsMu.Unlock()
	if h.genJobs != nil {
		return h.genJobs, nil
	}
	dir, err := defaultGenJobsDir()
	if err != nil {
		return nil, err
	}
	store := &genJobStore{dir: dir}
	if err := store.load(); err != nil {
		return nil, err
	}
	h.genJobs = store
	return store, nil
}

// load reads every archived job JSON back into the index (best effort per
// file: a corrupt single record never hides the rest of the version list).
func (s *genJobStore) load() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if rerr != nil {
			continue
		}
		var rec genJobRecord
		if jerr := json.Unmarshal(raw, &rec); jerr != nil {
			continue
		}
		s.jobs = append(s.jobs, &rec)
	}
	sort.Slice(s.jobs, func(i, j int) bool { return s.jobs[i].StartedAt.Before(s.jobs[j].StartedAt) })
	return nil
}

// persist writes one record's JSON archive atomically enough for a version
// list (temp file + rename).
func (s *genJobStore) persist(rec *genJobRecord) error {
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, rec.ID+".tmp")
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, rec.ID+".json"))
}

func (s *genJobStore) add(rec *genJobRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, rec)
	return s.persist(rec)
}

// update mutates the record under the store lock and re-archives it.
func (s *genJobStore) update(id string, mutate func(*genJobRecord)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range s.jobs {
		if rec.ID == id {
			mutate(rec)
			return s.persist(rec)
		}
	}
	return fmt.Errorf("gen job %q not found", id)
}

func (s *genJobStore) get(id string) *genJobRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range s.jobs {
		if rec.ID == id {
			return rec
		}
	}
	return nil
}

func (s *genJobStore) list() []*genJobRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*genJobRecord, len(s.jobs))
	copy(out, s.jobs)
	return out
}

// genJobsStart implements gen.jobs.start: register the version record and
// run the whole pipeline (engine ensure + conditioning + generate + ingest)
// on a background goroutine. The conversation turn returns immediately with
// the job id.
func (h *Harness) genJobsStart(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	store, serr := h.genJobStore()
	if serr != nil {
		return map[string]any{"status": "error", "error": serr.Error()}, serr
	}
	providerID := firstString(cmd, "provider_id", "provider")
	if providerID == "" {
		return nil, fmt.Errorf("gen.jobs.start needs provider_id (see gen.providers for configured entries)")
	}
	provider, _, err := genapi.FindProvider(providerID)
	if err != nil {
		gerr := genapi.AsError(err)
		return map[string]any{"status": "error", "error": gerr.Error()}, gerr
	}
	versionLabel := firstString(cmd, "version_label", "version")
	rec := &genJobRecord{
		ID:           sanitizeJobToken(fmt.Sprintf("genjob-%s", time.Now().Format("20060102-150405.000"))),
		VersionLabel: versionLabel,
		ProviderID:   provider.ID,
		Dialect:      provider.Dialect,
		State:        genJobStateStarting,
		StartedAt:    time.Now().UTC(),
		TrackID:      firstString(cmd, "track_id", "target_track_id"),
	}
	if aerr := store.add(rec); aerr != nil {
		return map[string]any{"status": "error", "error": "archive job record failed: " + aerr.Error()}, aerr
	}

	// The background leg must survive the request context (the tool call
	// returns now); generation length stays bounded by provider.Timeout().
	runCtx := context.WithoutCancel(ctx)
	go h.genJobRun(runCtx, store, rec.ID, provider, cmd)

	out := map[string]any{
		"status":        "ok",
		"job_id":        rec.ID,
		"state":         rec.State,
		"provider_id":   provider.ID,
		"dialect":       provider.Dialect,
		"version_label": rec.VersionLabel,
		"hint":          "generation runs in the background; poll gen.jobs.status {job_id} — progress, dispatched request summary and artifacts land there",
	}
	return out, nil
}

// genJobRun is the background pipeline body. It reuses genGenerate end to
// end (engine ensure + conditioning + submit/poll + optional ingest) and
// mirrors the outcome into the job record.
func (h *Harness) genJobRun(ctx context.Context, store *genJobStore, jobID string, provider genapi.ProviderConfig, cmd map[string]any) {
	fail := func(class string, err error) {
		now := time.Now().UTC()
		_ = store.update(jobID, func(rec *genJobRecord) {
			rec.State = genJobStateFailed
			rec.Error = err.Error()
			rec.ErrorClass = class
			rec.EndedAt = &now
		})
	}
	_ = store.update(jobID, func(rec *genJobRecord) {
		rec.State = genJobStatePreparing
		rec.Phase = "engine + conditioning"
	})
	result, err := h.genGenerate(ctx, cmd)
	if err != nil {
		fail("generation_error", err)
		return
	}
	if status := firstString(result, "status"); status != "ok" {
		fail("generation_error", fmt.Errorf("generation returned status=%s error=%v", status, firstString(result, "error")))
		return
	}
	now := time.Now().UTC()
	uerr := store.update(jobID, func(rec *genJobRecord) {
		rec.State = genJobStateDone
		rec.Phase = "complete"
		rec.EndedAt = &now
		rec.TaskID = firstString(result, "task_id")
		if d, ok := result["dispatched"].(map[string]any); ok {
			rec.Dispatched = d
		}
		if a, ok := result["conditioning_actions"].([]string); ok {
			rec.Actions = a
		} else if a, ok := result["conditioning_actions"].([]any); ok {
			for _, v := range a {
				if s, ok := v.(string); ok {
					rec.Actions = append(rec.Actions, s)
				}
			}
		}
		if s, ok := result["source_score"].(map[string]any); ok {
			rec.SourceScore = sourceScoreFromPayload(s)
		}
		if a, ok := result["artifacts"].([]map[string]any); ok {
			rec.Artifacts = a
		} else if a, ok := result["artifacts"].([]any); ok {
			for _, v := range a {
				if m, ok := v.(map[string]any); ok {
					rec.Artifacts = append(rec.Artifacts, m)
				}
			}
		}
		if t, ok := result["status_seen"].([]any); ok {
			for _, v := range t {
				if s, ok := v.(string); ok {
					rec.StatusTrail = append(rec.StatusTrail, s)
				}
			}
		} else if t, ok := result["status_seen"].([]string); ok {
			rec.StatusTrail = t
		}
		for _, a := range rec.Artifacts {
			if s, ok := a["kind"].(string); ok && s == "replay" {
				if p, ok := a["local_path"].(string); ok {
					rec.ReplayPath = p
				}
			}
		}
		// The source dialect score actually dispatched (the definitive
		// version record face).
		if d, ok := result["dispatched"].(map[string]any); ok {
			if head, ok := d["abc_head"].(string); ok {
				rec.SourceABC = head
			}
		}
		if ts, ok := result["two_stage"].(map[string]any); ok {
			rec.TwoStage = ts
		}
	})
	if uerr != nil {
		fail("archive_error", uerr)
	}
}

// sourceScoreFromPayload rebuilds the archived YueScoreInput from the
// disclosure payload (the full original note list + meter + tempo + key —
// the version archive must survive agent restarts and stay auditable).
func sourceScoreFromPayload(m map[string]any) *genapi.YueScoreInput {
	in := &genapi.YueScoreInput{}
	if v, ok := m["time_signature"].(string); ok && strings.Contains(v, "/") {
		parts := strings.SplitN(v, "/", 2)
		fmt.Sscanf(parts[0], "%d", &in.Numerator)
		fmt.Sscanf(parts[1], "%d", &in.Denominator)
	}
	if v, ok := m["tempo_bpm"].(float64); ok {
		in.TempoBPM = v
	}
	if v, ok := m["key"].(string); ok {
		in.Key = v
	}
	if rows, ok := m["notes"].([]any); ok {
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			note := genapi.YueNote{}
			if p, ok := row["pitch"].(float64); ok {
				note.Pitch = int(p)
			}
			if s, ok := row["start"].(float64); ok {
				note.Start = s
			}
			if l, ok := row["length"].(float64); ok {
				note.Length = l
			}
			in.Notes = append(in.Notes, note)
		}
	}
	return in
}

// genJobsStatus implements gen.jobs.status: live state, dispatched summary
// and artifact paths for one job.
func (h *Harness) genJobsStatus(cmd map[string]any) (map[string]any, error) {
	store, serr := h.genJobStore()
	if serr != nil {
		return map[string]any{"status": "error", "error": serr.Error()}, serr
	}
	jobID := firstString(cmd, "job_id", "id")
	if jobID == "" {
		return nil, fmt.Errorf("gen.jobs.status needs job_id")
	}
	rec := store.get(jobID)
	if rec == nil {
		return nil, fmt.Errorf("gen job %q not found (see gen.jobs.list)", jobID)
	}
	out := map[string]any{
		"status":        "ok",
		"job_id":        rec.ID,
		"state":         rec.State,
		"phase":         rec.Phase,
		"provider_id":   rec.ProviderID,
		"dialect":       rec.Dialect,
		"version_label": rec.VersionLabel,
		"started_at":    rec.StartedAt,
		"ended_at":      rec.EndedAt,
		"task_id":       rec.TaskID,
	}
	if rec.Error != "" {
		out["error"] = rec.Error
		out["error_class"] = rec.ErrorClass
	}
	if rec.Dispatched != nil {
		out["dispatched"] = rec.Dispatched
	}
	if len(rec.Actions) > 0 {
		out["conditioning_actions"] = rec.Actions
	}
	if rec.SourceScore != nil {
		out["source_score"] = map[string]any{
			"notes":          len(rec.SourceScore.Notes),
			"time_signature": fmt.Sprintf("%d/%d", rec.SourceScore.Numerator, rec.SourceScore.Denominator),
			"tempo_bpm":      rec.SourceScore.TempoBPM,
			"key":            rec.SourceScore.Key,
		}
	}
	if rec.SourceABC != "" {
		out["source_abc_head"] = rec.SourceABC
	}
	if rec.TwoStage != nil {
		out["two_stage"] = rec.TwoStage
	}
	if len(rec.Artifacts) > 0 {
		out["artifacts"] = rec.Artifacts
	}
	if rec.ReplayPath != "" {
		out["replay_path"] = rec.ReplayPath
	}
	if len(rec.StatusTrail) > 0 {
		out["status_trail"] = rec.StatusTrail
	}
	if rec.TrackID != "" {
		out["track_id"] = rec.TrackID
	}
	return out, nil
}

// genJobsList implements gen.jobs.list: the version list with the archive
// faces (original score, request, replay, audio) per job.
func (h *Harness) genJobsList() (map[string]any, error) {
	store, serr := h.genJobStore()
	if serr != nil {
		return map[string]any{"status": "error", "error": serr.Error()}, serr
	}
	jobs := store.list()
	out := make([]map[string]any, 0, len(jobs))
	for _, rec := range jobs {
		entry := map[string]any{
			"job_id":        rec.ID,
			"state":         rec.State,
			"provider_id":   rec.ProviderID,
			"version_label": rec.VersionLabel,
			"started_at":    rec.StartedAt,
			"task_id":       rec.TaskID,
		}
		if rec.Error != "" {
			entry["error"] = rec.Error
		}
		if rec.Dispatched != nil {
			if s, ok := rec.Dispatched["style"]; ok {
				entry["style"] = s
			}
		}
		var audio []string
		for _, a := range rec.Artifacts {
			if k, _ := a["kind"].(string); k == "audio" {
				if p, ok := a["local_path"].(string); ok {
					audio = append(audio, p)
				}
			}
		}
		if len(audio) > 0 {
			entry["audio_paths"] = audio
		}
		if rec.ReplayPath != "" {
			entry["replay_path"] = rec.ReplayPath
		}
		if rec.SourceScore != nil && len(rec.SourceScore.Notes) > 0 {
			entry["source_notes"] = len(rec.SourceScore.Notes)
		}
		out = append(out, entry)
	}
	return map[string]any{
		"status": "ok",
		"count":  len(out),
		"jobs":   out,
		"hint":   "two versions of the same score = two jobs (the local engine serves one candidate per request)",
	}, nil
}
