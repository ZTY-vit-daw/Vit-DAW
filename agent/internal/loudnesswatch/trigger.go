// Loudness watch trigger wiring (card LOUDNESS-WATCH-TRIGGER-1, design file
// 01 of LOUDNESS-WATCH-DESIGN-1). This file owns the mechanical runtime the
// three after-processing hooks drive: config gate, measurement-row freshness
// partition, the pure Compare call, stale-row bake fill (capped), verdict
// persistence with dedupe, and the optional periodic ticker.
//
// Design-pinned invariants (card face "设计钉死项"):
//   - the watch is an observation parasite: check failures only WARN through
//     the injected logger and never unwind a settlement or an observation;
//   - the piggyback path (T2) never issues new bakes;
//   - one trigger bakes at most MaxBakeClipsPerTrigger clips; beyond that the
//     verdict is unmeasurable with reason scope_capped (bake-storm guard);
//   - dedupe key = project_uuid + project_revision + verdict_kind; a repeat
//     check under the same key rechecks (counter bump) but never duplicates
//     the recorded verdict, so alert consumers stay noise-free;
//   - periodic mode is config-driven and default-off; with the watch disabled
//     every entry point returns without touching disk (silent zero cost).
package loudnesswatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Trigger kinds carried on verdict records (design 01 §2.4 trigger field).
const (
	TriggerKindSettle          = "settle"
	TriggerKindProjectChange   = "project_change"
	TriggerKindObservationRide = "observation_piggyback"
	TriggerKindPeriodic        = "periodic"
)

// MaxBakeClipsPerTrigger is the per-trigger stale-fill bake cap (design 01
// §4: a large-project settle must not amplify the bake queue).
const MaxBakeClipsPerTrigger = 4

// VerdictStoreSchemaVersion is the only accepted verdict-store schema.
const VerdictStoreSchemaVersion = "loudness_watch_verdicts.v0"

// MaxKeptVerdicts bounds the verdict history (status face reads recent
// verdicts; older entries drop off the tail).
const MaxKeptVerdicts = 50

// Unmeasurable reasons specific to the trigger layer (Compare's own reasons
// stay authoritative when fresh rows exist).
const (
	UnmeasurableScopeCapped     = "scope_capped"
	UnmeasurableStaleBakeIssued = "stale_rows_bake_issued"
)

func sprintScopeCapped(staleCount int) string {
	return fmt.Sprintf("scope_capped: %d stale clips exceed the %d-clip bake cap for one trigger; no bake issued", staleCount, MaxBakeClipsPerTrigger)
}

func sprintStaleBake(issued int) string {
	return fmt.Sprintf("stale_rows_bake_issued: %d clips re-baked in the background; verdict deferred to the next trigger", issued)
}

func errForeignSchema(found string) error {
	return errors.New("loudness watch verdict store: schema version " + found + " is not " + VerdictStoreSchemaVersion + " (fail closed)")
}

// Logger is the minimal WARN surface the runtime needs (logx.Logger satisfies
// it); nil disables logging without changing behavior.
type Logger interface {
	Warn(format string, args ...any)
}

// BakeTarget addresses one clip for a background loudness bake.
type BakeTarget struct {
	TrackID  string `json:"track_id"`
	ClipID   string `json:"clip_id"`
	FilePath string `json:"file_path,omitempty"`
}

// Deps are the closure-supplied seams. Every func must be safe to call from a
// background goroutine; nil Deps funcs degrade to honest no-ops.
type Deps struct {
	// Config returns the effective watch config; absent (nil) means the
	// project never configured the watch — every entry point exits silently.
	Config func() (Config, bool)
	// Rows returns the raw loudness_summaries feature rows for the current
	// project binding.
	Rows func() []map[string]any
	// Bake issues background loudness_summary bakes for the given targets and
	// returns how many it actually issued. The runtime always enforces the
	// cap before calling; nil means no bake face is wired (T2 piggyback and
	// unit tests).
	Bake func(targets []BakeTarget) int
	// Identity resolves the current project uuid for verdict records.
	Identity func() string
	// StorePath is the verdict store file location.
	StorePath string
	Logger    Logger
	Now       func() time.Time
}

// TriggerContext carries the hook identity for one check.
type TriggerContext struct {
	ProjectUUID     string
	ProjectRevision string
	TriggerKind     string
	TriggerRef      string
	// AllowBake is false on the T2 piggyback path (design 01 §2.2: the
	// piggyback consumes already-baked rows and never issues new bakes).
	AllowBake bool
	// CoveredTargets limits the comparison scope to the observed clips
	// (T2); nil means project scope (T1/T3/periodic).
	CoveredTargets []map[string]any
}

// VerdictRecord is one stored verdict plus the trigger-layer telemetry (E1
// cost accounting: per-check wall time, bake count, fresh-row reuse).
type VerdictRecord struct {
	Key             string  `json:"key"`
	ProjectUUID     string  `json:"project_uuid"`
	ProjectRevision string  `json:"project_revision"`
	VerdictKind     string  `json:"verdict_kind"`
	Verdict         Verdict `json:"verdict"`
	RecordedAt      string  `json:"recorded_at"`
	LastRecheckedAt string  `json:"last_rechecked_at,omitempty"`
	RecheckCount    int     `json:"recheck_count,omitempty"`
	TriggerKind     string  `json:"trigger_kind"`
	BakesIssued     int     `json:"bakes_issued"`
	FreshRows       int     `json:"fresh_rows"`
	StaleRows       int     `json:"stale_rows"`
	CheckDurationMs float64 `json:"check_duration_ms"`
}

// VerdictStoreFile is the on-disk verdict history plus the process-lifetime
// check counter the status face discloses.
type VerdictStoreFile struct {
	SchemaVersion string          `json:"schema_version"`
	UpdatedAt     string          `json:"updated_at"`
	ChecksTotal   int64           `json:"checks_total"`
	Verdicts      []VerdictRecord `json:"verdicts"`
}

// Runtime is the trigger wiring core. All exported methods are safe for
// concurrent use; none of them ever returns an error to a caller — failures
// WARN through Deps.Logger and leave the triggering flow untouched.
type Runtime struct {
	deps Deps

	mu           sync.Mutex
	store        VerdictStoreFile
	storeLoaded  bool
	periodicStop chan struct{}
	periodicOn   bool
}

// NewRuntime builds the watch runtime over the given seams.
func NewRuntime(deps Deps) *Runtime {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Runtime{deps: deps}
}

func (r *Runtime) now() time.Time {
	if r == nil || r.deps.Now == nil {
		return time.Now()
	}
	return r.deps.Now()
}

func (r *Runtime) warn(format string, args ...any) {
	if r != nil && r.deps.Logger != nil {
		r.deps.Logger.Warn("[loudness-watch] "+format, args...)
	}
}

// Check runs one watch evaluation for the given trigger. It is synchronous
// and bounded (file read + pure compare + at most MaxBakeClipsPerTrigger
// bake sends); hook call sites wrap it in a goroutine.
func (r *Runtime) Check(trigger TriggerContext) {
	if r == nil {
		return
	}
	started := r.now()
	config, ok := r.config()
	if !ok || !config.Enabled {
		// Silent zero cost: no counter, no disk write, no goroutine spawn.
		return
	}
	if config.TriggerMode == TriggerRealtime {
		// Capability-gap semantics (design 01 §3): stored but never active;
		// the status face already discloses unavailable/no_realtime_producer.
		return
	}
	r.EnsurePeriodic()

	rows := r.rows()
	if len(trigger.CoveredTargets) > 0 {
		rows = rowsCoveredByTargets(rows, trigger.CoveredTargets)
	}
	projectUUID := firstNonEmptyString(trigger.ProjectUUID, r.identity())
	fresh, stale := partitionRows(rows, projectUUID)

	bakesIssued := 0
	if trigger.AllowBake && len(stale) > 0 && len(stale) <= MaxBakeClipsPerTrigger && r.deps.Bake != nil {
		bakesIssued = r.deps.Bake(bakeTargetsFromRows(stale))
	}

	verdict := Compare(fresh, config)
	verdict.ProjectUUID = projectUUID
	verdict.ProjectRevision = trigger.ProjectRevision
	verdict.TriggerKind = trigger.TriggerKind
	verdict.TriggerRef = trigger.TriggerRef
	verdict.CheckedAt = r.now().UTC().Format(time.RFC3339Nano)
	if len(fresh) == 0 && len(rows) > 0 {
		// Rows exist but none is fresh: the trigger layer owns the honest
		// reason (Compare only sees the empty fresh set).
		verdict.Verdict = VerdictUnmeasurable
		verdict.Measured = nil
		verdict.BreachDetails = nil
		switch {
		case len(stale) > MaxBakeClipsPerTrigger:
			verdict.UnmeasurableReason = sprintScopeCapped(len(stale))
			verdict.Summary = approximateSummary(verdict.Verdict, verdict.UnmeasurableReason, 0, "")
		case bakesIssued > 0:
			verdict.UnmeasurableReason = sprintStaleBake(bakesIssued)
			verdict.Summary = approximateSummary(verdict.Verdict, verdict.UnmeasurableReason, 0, "")
		default:
			verdict.UnmeasurableReason = "stale_rows: all covered loudness rows are stale and no bake face is wired"
			verdict.Summary = approximateSummary(verdict.Verdict, verdict.UnmeasurableReason, 0, "")
		}
	}
	r.recordVerdict(trigger, verdict, RecordMetrics{
		DurationMs: durationMs(started, r.now()),
		BakesIssued: bakesIssued,
		FreshRows:  len(fresh),
		StaleRows:  len(stale),
	})
}

// RecordMetrics carries one check's cost telemetry into the store (E1).
type RecordMetrics struct {
	DurationMs  float64
	BakesIssued int
	FreshRows   int
	StaleRows   int
}

func (r *Runtime) recordVerdict(trigger TriggerContext, verdict Verdict, metrics RecordMetrics) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.loadStoreLocked(); err != nil {
		r.warn("verdict store load failed path=%s err=%v (starting an empty history)", r.deps.StorePath, err)
	}
	r.store.SchemaVersion = VerdictStoreSchemaVersion
	r.store.UpdatedAt = r.now().UTC().Format(time.RFC3339Nano)
	r.store.ChecksTotal++
	key := DedupeKey(verdict.ProjectUUID, verdict.ProjectRevision, verdict.Verdict)
	nowStamp := r.now().UTC().Format(time.RFC3339Nano)
	for index := range r.store.Verdicts {
		if r.store.Verdicts[index].Key == key {
			// Dedupe: same project+revision+kind never duplicates the
			// recorded verdict; later checks only bump the recheck audit.
			r.store.Verdicts[index].LastRecheckedAt = nowStamp
			r.store.Verdicts[index].RecheckCount++
			r.store.Verdicts[index].TriggerKind = trigger.TriggerKind
			r.store.Verdicts[index].BakesIssued = metrics.BakesIssued
			r.store.Verdicts[index].FreshRows = metrics.FreshRows
			r.store.Verdicts[index].StaleRows = metrics.StaleRows
			r.store.Verdicts[index].CheckDurationMs = metrics.DurationMs
			r.writeStoreLocked()
			return
		}
	}
	r.store.Verdicts = append(r.store.Verdicts, VerdictRecord{
		Key:             key,
		ProjectUUID:     verdict.ProjectUUID,
		ProjectRevision: verdict.ProjectRevision,
		VerdictKind:     verdict.Verdict,
		Verdict:         verdict,
		RecordedAt:      nowStamp,
		TriggerKind:     trigger.TriggerKind,
		BakesIssued:     metrics.BakesIssued,
		FreshRows:       metrics.FreshRows,
		StaleRows:       metrics.StaleRows,
		CheckDurationMs: metrics.DurationMs,
	})
	if len(r.store.Verdicts) > MaxKeptVerdicts {
		r.store.Verdicts = r.store.Verdicts[len(r.store.Verdicts)-MaxKeptVerdicts:]
	}
	r.writeStoreLocked()
}

// DedupeKey builds the pinned dedupe key: project uuid + project revision +
// verdict kind (design 01 §2.4).
func DedupeKey(projectUUID, projectRevision, verdictKind string) string {
	return projectUUID + "|" + projectRevision + "|" + verdictKind
}

func (r *Runtime) loadStoreLocked() error {
	if r.storeLoaded {
		return nil
	}
	r.storeLoaded = true
	data, err := os.ReadFile(r.deps.StorePath)
	if err != nil {
		if os.IsNotExist(err) {
			r.store = VerdictStoreFile{SchemaVersion: VerdictStoreSchemaVersion}
			return nil
		}
		return err
	}
	loaded := VerdictStoreFile{}
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	if loaded.SchemaVersion != "" && loaded.SchemaVersion != VerdictStoreSchemaVersion {
		// Foreign schema: fail closed to an empty history rather than
		// guessing an upgrade (AGENTS.md §11 discipline).
		return errForeignSchema(loaded.SchemaVersion)
	}
	if loaded.SchemaVersion == "" {
		loaded.SchemaVersion = VerdictStoreSchemaVersion
	}
	r.store = loaded
	return nil
}

func (r *Runtime) writeStoreLocked() {
	if r.deps.StorePath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(r.deps.StorePath), 0o755); err != nil {
		r.warn("verdict store mkdir failed path=%s err=%v", r.deps.StorePath, err)
		return
	}
	data, err := json.MarshalIndent(r.store, "", "  ")
	if err != nil {
		r.warn("verdict store marshal failed err=%v", err)
		return
	}
	if err := os.WriteFile(r.deps.StorePath, append(data, '\n'), 0o644); err != nil {
		r.warn("verdict store write failed path=%s err=%v", r.deps.StorePath, err)
	}
}

// Status discloses the runtime face for tools and observation carriers:
// effective config (when present), check counter, verdict count, and the
// most recent verdict summary. Read-only.
func (r *Runtime) Status() map[string]any {
	if r == nil {
		return map[string]any{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.loadStoreLocked(); err != nil {
		r.warn("verdict store load failed path=%s err=%v", r.deps.StorePath, err)
	}
	out := map[string]any{
		"schema_version": VerdictStoreSchemaVersion,
		"checks_total":   r.store.ChecksTotal,
		"verdict_count":  len(r.store.Verdicts),
		"verdicts_path":  r.deps.StorePath,
	}
	if config, ok := r.config(); ok {
		out["config"] = config
		out["config_digest"] = ConfigDigest(config)
		if config.TriggerMode == TriggerRealtime {
			out["realtime_availability"] = map[string]any{
				"status": RealtimeUnavailableStatus,
				"reason": RealtimeUnavailableReason,
			}
		}
	} else {
		out["config"] = nil
	}
	if len(r.store.Verdicts) > 0 {
		latest := r.store.Verdicts[len(r.store.Verdicts)-1]
		out["last_verdict"] = map[string]any{
			"verdict":       latest.VerdictKind,
			"trigger_kind":  latest.TriggerKind,
			"recorded_at":   latest.RecordedAt,
			"breach_count":  len(latest.Verdict.BreachDetails),
			"summary":       latest.Verdict.Summary,
		}
	}
	return out
}

// EnsurePeriodic reconciles the background ticker with the config: periodic
// and enabled starts one ticker, anything else stops it. Default-off means
// no goroutine exists until the user opts in.
func (r *Runtime) EnsurePeriodic() {
	if r == nil {
		return
	}
	config, ok := r.config()
	want := ok && config.Enabled && config.TriggerMode == TriggerPeriodic
	r.mu.Lock()
	defer r.mu.Unlock()
	if want && !r.periodicOn {
		r.periodicStop = make(chan struct{})
		r.periodicOn = true
		stop := r.periodicStop
		go r.runPeriodic(stop)
		return
	}
	if !want && r.periodicOn {
		close(r.periodicStop)
		r.periodicOn = false
		r.periodicStop = nil
	}
}

func (r *Runtime) runPeriodic(stop chan struct{}) {
	for {
		config, ok := r.config()
		if !ok || !config.Enabled || config.TriggerMode != TriggerPeriodic {
			return
		}
		// The interval comes from the validated config (ValidateConfig owns
		// the >=60s floor); zero or negative falls back to the default.
		interval := time.Duration(config.PeriodicIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = time.Duration(DefaultPeriodicIntervalSeconds) * time.Second
		}
		timer := time.NewTimer(interval)
		select {
		case <-stop:
			timer.Stop()
			return
		case <-timer.C:
			r.recoverCheck(TriggerContext{TriggerKind: TriggerKindPeriodic, AllowBake: true})
		}
	}
}

// recoverCheck runs Check with a panic guard: the watch is an observation
// parasite and may never take down a background goroutine (a panic in a bare
// goroutine would kill the process).
func (r *Runtime) recoverCheck(trigger TriggerContext) {
	defer func() {
		if err := recover(); err != nil {
			r.warn("check panicked trigger=%s err=%v", trigger.TriggerKind, err)
		}
	}()
	r.Check(trigger)
}

func (r *Runtime) config() (Config, bool) {
	if r == nil || r.deps.Config == nil {
		return Config{}, false
	}
	return r.deps.Config()
}

func (r *Runtime) rows() []map[string]any {
	if r == nil || r.deps.Rows == nil {
		return nil
	}
	return r.deps.Rows()
}

func (r *Runtime) identity() string {
	if r == nil || r.deps.Identity == nil {
		return ""
	}
	return r.deps.Identity()
}

// partitionRows splits rows into fresh (consumable) and stale (bake
// candidates). Freshness is honest and cheap: a row must carry a measurement
// value, must not self-report building/stale/missing/failed, and — when it
// carries a project identity — must belong to the checked project. Rows
// whose quality face is absent are judged by their status face only.
func partitionRows(rows []map[string]any, projectUUID string) (fresh, stale []map[string]any) {
	fresh, stale = []map[string]any{}, []map[string]any{}
	for _, row := range rows {
		if rowConsumable(row, projectUUID) {
			fresh = append(fresh, row)
		} else {
			stale = append(stale, row)
		}
	}
	return fresh, stale
}

func rowConsumable(row map[string]any, projectUUID string) bool {
	if len(row) == 0 {
		return false
	}
	if _, hasValue := rowNumber(row, "integrated_lufs", "approximate_lufs", "lufs", "lufs_estimate"); !hasValue {
		if _, hasPeak := rowNumber(row, "sample_peak_dbfs", "peak_dbfs"); !hasPeak {
			return false
		}
	}
	status := lowerTrimmed(firstStringFromAny(row, "status"))
	switch status {
	case "building", "stale", "missing", "failed", "blocked":
		return false
	}
	quality := lowerTrimmed(firstStringFromAny(row, "quality_status"))
	switch quality {
	case "building", "stale", "missing", "failed", "blocked":
		return false
	}
	if projectUUID != "" {
		if rowProject := firstStringFromAny(row, "project_uuid", "project_id"); rowProject != "" && rowProject != projectUUID {
			return false
		}
	}
	return true
}

// rowsCoveredByTargets keeps only rows whose (track_id, clip_id) pair is
// covered by the observation targets (design 01 §2.2: the piggyback compares
// only the clips this observation covered).
func rowsCoveredByTargets(rows []map[string]any, targets []map[string]any) []map[string]any {
	covered := map[string]bool{}
	for _, target := range targets {
		key := targetKeyString(firstStringFromAny(target, "track_id"), firstStringFromAny(target, "clip_id"))
		if key != "|" {
			covered[key] = true
		}
	}
	if len(covered) == 0 {
		return rows
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if covered[targetKeyString(firstStringFromAny(row, "track_id"), firstStringFromAny(row, "clip_id"))] {
			out = append(out, row)
		}
	}
	return out
}

func targetKeyString(trackID, clipID string) string {
	return trackID + "|" + clipID
}

func bakeTargetsFromRows(rows []map[string]any) []BakeTarget {
	targets := make([]BakeTarget, 0, len(rows))
	for _, row := range rows {
		target := BakeTarget{
			TrackID:  firstStringFromAny(row, "track_id"),
			ClipID:   firstStringFromAny(row, "clip_id"),
			FilePath: firstStringFromAny(row, "file_path", "source_path", "current_source_path"),
		}
		if target.TrackID == "" && target.ClipID == "" {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstStringFromAny(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			if text, ok := value.(string); ok && text != "" {
				return text
			}
		}
	}
	return ""
}

func lowerTrimmed(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func durationMs(from, to time.Time) float64 {
	ms := float64(to.Sub(from)) / float64(time.Millisecond)
	if ms < 0 {
		return 0
	}
	return ms
}
