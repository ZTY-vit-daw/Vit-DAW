package loudnesswatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func validEnabledConfig() Config {
	config := Normalize(Config{ProfileSource: "builtin:ebu_r128"})
	config.Enabled = true
	return config
}

type recordingLogger struct {
	mu     sync.Mutex
	warned []string
}

func (l *recordingLogger) Warn(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warned = append(l.warned, format)
}

func (l *recordingLogger) warnCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.warned)
}

type fixture struct {
	runtime  *Runtime
	storeDir string
	config   atomicConfig
	rows     atomicRows
	bakes    atomicBakes
}

type atomicConfig struct {
	mu     sync.Mutex
	config Config
	ok     bool
}

func (c *atomicConfig) get() (Config, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.config, c.ok
}

func (c *atomicConfig) set(config Config, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config, c.ok = config, ok
}

func (c *atomicConfig) modify(fn func(*Config)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(&c.config)
}

type atomicRows struct {
	mu   sync.Mutex
	rows []map[string]any
}

func (r *atomicRows) get() []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rows
}

func (r *atomicRows) set(rows []map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows = rows
}

type atomicBakes struct {
	mu     sync.Mutex
	target []BakeTarget
}

func (b *atomicBakes) bake(targets []BakeTarget) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.target = append(b.target, targets...)
	return len(targets)
}

func (b *atomicBakes) issued() []BakeTarget {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]BakeTarget(nil), b.target...)
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	storeDir := t.TempDir()
	f := &fixture{storeDir: storeDir}
	f.runtime = NewRuntime(Deps{
		Config:    f.config.get,
		Rows:      f.rows.get,
		Bake:      f.bakes.bake,
		Identity:  func() string { return "proj-test" },
		StorePath: filepath.Join(storeDir, "loudness_watch_verdicts.json"),
		Logger:    &recordingLogger{},
		Now:       time.Now,
	})
	return f
}

func (f *fixture) readStore(t *testing.T) VerdictStoreFile {
	t.Helper()
	data, err := os.ReadFile(f.runtime.deps.StorePath)
	if err != nil {
		t.Fatalf("read verdict store: %v", err)
	}
	store := VerdictStoreFile{}
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatalf("parse verdict store: %v", err)
	}
	return store
}

func freshRow(trackID, clipID string, integrated float64) map[string]any {
	return map[string]any{
		"track_id": trackID, "clip_id": clipID,
		"integrated_lufs": integrated, "sample_peak_dbfs": -3.5,
		"status": "ready", "quality_status": "ready",
	}
}

func staleRow(trackID, clipID string) map[string]any {
	return map[string]any{
		"track_id": trackID, "clip_id": clipID,
		"integrated_lufs": -18.0, "status": "building",
	}
}

func TestCheckDisabledConfigIsSilentZeroCost(t *testing.T) {
	f := newFixture(t)
	// Never configured: no file, no counter, no bake.
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", ProjectRevision: "rev-1", TriggerKind: TriggerKindSettle, AllowBake: true})
	if _, err := os.Stat(f.runtime.deps.StorePath); !os.IsNotExist(err) {
		t.Fatalf("disabled watch wrote the verdict store: %v", err)
	}
	// Configured but disabled: still zero surface.
	f.config.set(validEnabledConfig(), true)
	f.config.modify(func(config *Config) { config.Enabled = false })
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", TriggerKind: TriggerKindSettle, AllowBake: true})
	if _, err := os.Stat(f.runtime.deps.StorePath); !os.IsNotExist(err) {
		t.Fatalf("disabled watch wrote the verdict store: %v", err)
	}
	if bakes := f.bakes.issued(); len(bakes) != 0 {
		t.Fatalf("disabled watch issued bakes: %+v", bakes)
	}
}

func TestCheckRealtimeModeNeverProducesVerdicts(t *testing.T) {
	f := newFixture(t)
	config := validEnabledConfig()
	config.TriggerMode = TriggerRealtime
	f.config.set(config, true)
	f.rows.set([]map[string]any{freshRow("t1", "c1", -23.0)})
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", ProjectRevision: "rev-1", TriggerKind: TriggerKindSettle})
	if _, err := os.Stat(f.runtime.deps.StorePath); !os.IsNotExist(err) {
		t.Fatalf("realtime mode produced a verdict (capability gap must stay quiet): %v", err)
	}
}

func TestCheckFreshRowsRecordVerdictAndDedupe(t *testing.T) {
	f := newFixture(t)
	f.config.set(validEnabledConfig(), true)
	f.rows.set([]map[string]any{
		freshRow("t1", "c1", -23.0),
		freshRow("t2", "c2", -12.0), // above the R128 ceiling: breach
	})
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", ProjectRevision: "rev-1", TriggerKind: TriggerKindSettle, TriggerRef: "fsc_a"})
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", ProjectRevision: "rev-1", TriggerKind: TriggerKindObservationRide})

	store := f.readStore(t)
	if len(store.Verdicts) != 1 {
		t.Fatalf("dedupe failed: %d records for the same project+revision+kind", len(store.Verdicts))
	}
	record := store.Verdicts[0]
	if record.VerdictKind != VerdictBreach {
		t.Fatalf("expected breach verdict, got %q", record.VerdictKind)
	}
	if record.TriggerKind != TriggerKindObservationRide {
		t.Fatalf("recheck should refresh the trigger audit, got %q", record.TriggerKind)
	}
	if record.RecheckCount != 1 {
		t.Fatalf("recheck count not bumped: %+v", record)
	}
	if record.FreshRows != 2 || record.StaleRows != 0 {
		t.Fatalf("row accounting drifted: %+v", record)
	}
	if record.CheckDurationMs < 0 {
		t.Fatalf("duration telemetry broken: %+v", record)
	}
	// A different verdict kind under the same revision is a new record (state
	// changes re-alert, by design).
	f.rows.set([]map[string]any{freshRow("t1", "c1", -23.0)})
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", ProjectRevision: "rev-1", TriggerKind: TriggerKindSettle})
	store = f.readStore(t)
	if len(store.Verdicts) != 2 {
		t.Fatalf("kind change must record a new verdict: %d records", len(store.Verdicts))
	}
	// A different revision is a new record.
	f.rows.set([]map[string]any{freshRow("t1", "c1", -12.0)})
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", ProjectRevision: "rev-2", TriggerKind: TriggerKindSettle})
	store = f.readStore(t)
	if len(store.Verdicts) != 3 {
		t.Fatalf("revision change must record a new verdict: %d records", len(store.Verdicts))
	}
	if store.ChecksTotal != 4 {
		t.Fatalf("checks counter drifted: %d", store.ChecksTotal)
	}
}

func TestCheckStaleRowsBakeWithinCap(t *testing.T) {
	f := newFixture(t)
	f.config.set(validEnabledConfig(), true)
	f.rows.set([]map[string]any{
		staleRow("t1", "c1"), staleRow("t2", "c2"),
	})
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", ProjectRevision: "rev-1", TriggerKind: TriggerKindSettle, AllowBake: true})
	bakes := f.bakes.issued()
	if len(bakes) != 2 {
		t.Fatalf("stale fill should bake both clips: %+v", bakes)
	}
	store := f.readStore(t)
	record := store.Verdicts[len(store.Verdicts)-1]
	if record.VerdictKind != VerdictUnmeasurable {
		t.Fatalf("stale-only check must be unmeasurable, got %q", record.VerdictKind)
	}
	if record.BakesIssued != 2 {
		t.Fatalf("bake count not recorded: %+v", record)
	}
	if want := DedupeKey("proj-test", "rev-1", VerdictUnmeasurable); record.Key != want {
		t.Fatalf("dedupe key mismatch: %q vs %q", record.Key, want)
	}
	if record.Verdict.UnmeasurableReason == "" {
		t.Fatalf("unmeasurable reason missing: %+v", record.Verdict)
	}
}

func TestCheckStaleRowsBeyondCapAreScopeCapped(t *testing.T) {
	f := newFixture(t)
	f.config.set(validEnabledConfig(), true)
	rows := make([]map[string]any, 0, MaxBakeClipsPerTrigger+2)
	for index := 0; index < MaxBakeClipsPerTrigger+2; index++ {
		rows = append(rows, staleRow("t", "c"))
		rows[index]["clip_id"] = "c" + string(rune('a'+index))
	}
	f.rows.set(rows)
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", TriggerKind: TriggerKindSettle, AllowBake: true})
	if bakes := f.bakes.issued(); len(bakes) != 0 {
		t.Fatalf("bake storm guard failed: %+v", bakes)
	}
	store := f.readStore(t)
	record := store.Verdicts[len(store.Verdicts)-1]
	if record.VerdictKind != VerdictUnmeasurable || record.Verdict.UnmeasurableReason == "" {
		t.Fatalf("scope_capped verdict missing: %+v", record)
	}
	if got := record.Verdict.UnmeasurableReason; len(got) == 0 || got[:13] != "scope_capped:" {
		t.Fatalf("expected scope_capped reason, got %q", got)
	}
}

func TestPiggybackNeverBakes(t *testing.T) {
	f := newFixture(t)
	f.config.set(validEnabledConfig(), true)
	f.rows.set([]map[string]any{staleRow("t1", "c1")})
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", TriggerKind: TriggerKindObservationRide, AllowBake: false})
	if bakes := f.bakes.issued(); len(bakes) != 0 {
		t.Fatalf("piggyback path issued bakes: %+v", bakes)
	}
	store := f.readStore(t)
	record := store.Verdicts[len(store.Verdicts)-1]
	if record.VerdictKind != VerdictUnmeasurable {
		t.Fatalf("piggyback over stale rows must stay unmeasurable: %+v", record)
	}
}

func TestCoveredTargetsScopeTheComparison(t *testing.T) {
	f := newFixture(t)
	f.config.set(validEnabledConfig(), true)
	f.rows.set([]map[string]any{
		freshRow("t1", "c1", -23.0),
		freshRow("t2", "c2", -12.0), // breach, but outside the covered scope
	})
	f.runtime.Check(TriggerContext{
		ProjectUUID:    "proj-test",
		ProjectRevision: "rev-1",
		TriggerKind:    TriggerKindObservationRide,
		CoveredTargets: []map[string]any{{"track_id": "t1", "clip_id": "c1"}},
	})
	store := f.readStore(t)
	record := store.Verdicts[len(store.Verdicts)-1]
	if record.VerdictKind != VerdictPass {
		t.Fatalf("out-of-scope clip leaked into the piggyback verdict: %+v", record)
	}
	if len(record.Verdict.Measured) != 1 {
		t.Fatalf("covered scope must compare exactly the observed clip: %+v", record.Verdict.Measured)
	}
}

func TestForeignProjectRowsAreNotConsumed(t *testing.T) {
	f := newFixture(t)
	f.config.set(validEnabledConfig(), true)
	foreign := freshRow("t1", "c1", -12.0)
	foreign["project_uuid"] = "proj-other"
	f.rows.set([]map[string]any{foreign})
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", TriggerKind: TriggerKindSettle, AllowBake: true})
	if bakes := f.bakes.issued(); len(bakes) != 1 {
		t.Fatalf("foreign row should count as stale (bake candidate), got %+v", bakes)
	}
}

func TestVerdictStoreCapsHistory(t *testing.T) {
	f := newFixture(t)
	f.config.set(validEnabledConfig(), true)
	f.rows.set([]map[string]any{freshRow("t1", "c1", -23.0)})
	for index := 0; index < MaxKeptVerdicts+7; index++ {
		f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", ProjectRevision: "rev-" + string(rune('a'+index%26)) + itoa(index), TriggerKind: TriggerKindSettle})
	}
	store := f.readStore(t)
	if len(store.Verdicts) != MaxKeptVerdicts {
		t.Fatalf("history cap failed: %d records", len(store.Verdicts))
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

func TestVerdictStoreForeignSchemaFailsClosed(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(filepath.Dir(f.runtime.deps.StorePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	foreign := []byte(`{"schema_version":"loudness_watch_verdicts.v9","verdicts":[{"key":"x"}]}`)
	if err := os.WriteFile(f.runtime.deps.StorePath, foreign, 0o644); err != nil {
		t.Fatalf("seed foreign store: %v", err)
	}
	f.config.set(validEnabledConfig(), true)
	f.rows.set([]map[string]any{freshRow("t1", "c1", -23.0)})
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", ProjectRevision: "rev-1", TriggerKind: TriggerKindSettle})
	// The foreign history is not upgraded or consumed: the new record starts
	// a fresh history under the pinned schema.
	store := f.readStore(t)
	if store.SchemaVersion != VerdictStoreSchemaVersion {
		t.Fatalf("schema not pinned: %q", store.SchemaVersion)
	}
	if len(store.Verdicts) != 1 {
		t.Fatalf("fresh history expected: %+v", store.Verdicts)
	}
}

func TestStatusDisclosesCountersAndLatestVerdict(t *testing.T) {
	f := newFixture(t)
	empty := f.runtime.Status()
	if empty["verdict_count"].(int) != 0 {
		t.Fatalf("empty status verdict_count: %+v", empty)
	}
	if _, ok := empty["config"]; !ok {
		t.Fatalf("status config key missing: %+v", empty)
	}
	f.config.set(validEnabledConfig(), true)
	f.rows.set([]map[string]any{freshRow("t1", "c1", -12.0)})
	f.runtime.Check(TriggerContext{ProjectUUID: "proj-test", ProjectRevision: "rev-1", TriggerKind: TriggerKindSettle})
	status := f.runtime.Status()
	if status["verdict_count"].(int) != 1 {
		t.Fatalf("status verdict_count: %+v", status)
	}
	if status["checks_total"].(int64) != 1 {
		t.Fatalf("status checks_total: %+v", status)
	}
	latest, ok := status["last_verdict"].(map[string]any)
	if !ok || latest["verdict"] != VerdictBreach {
		t.Fatalf("status last_verdict: %+v", status["last_verdict"])
	}
	if status["verdicts_path"] != f.runtime.deps.StorePath {
		t.Fatalf("status verdicts_path: %+v", status)
	}
}

func TestPeriodicLifecycle(t *testing.T) {
	f := newFixture(t)
	// Default off: EnsurePeriodic starts nothing.
	f.config.set(validEnabledConfig(), true)
	f.runtime.EnsurePeriodic()
	f.runtime.mu.Lock()
	started := f.runtime.periodicOn
	f.runtime.mu.Unlock()
	if started {
		t.Fatalf("after_processing mode must not start a ticker")
	}
	// Periodic with a 1s test interval (validated configs enforce >=60s;
	// the runtime trusts the validated value) fires checks until disabled.
	config := validEnabledConfig()
	config.TriggerMode = TriggerPeriodic
	config.PeriodicIntervalSeconds = 1
	f.config.set(config, true)
	f.runtime.EnsurePeriodic()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		store := f.runtime.Status()
		if store["checks_total"].(int64) >= 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if f.runtime.Status()["checks_total"].(int64) < 1 {
		t.Fatalf("periodic ticker never fired a check")
	}
	// Disabling stops the ticker.
	config.Enabled = false
	f.config.set(config, true)
	f.runtime.EnsurePeriodic()
	f.runtime.mu.Lock()
	stillOn := f.runtime.periodicOn
	f.runtime.mu.Unlock()
	if stillOn {
		t.Fatalf("disabled config must stop the ticker")
	}
}

func TestCheckPanicIsContained(t *testing.T) {
	f := newFixture(t)
	f.config.set(validEnabledConfig(), true)
	f.runtime.deps.Rows = func() []map[string]any { panic("rows blew up") }
	f.runtime.recoverCheck(TriggerContext{ProjectUUID: "proj-test", TriggerKind: TriggerKindSettle})
	logger := f.runtime.deps.Logger.(*recordingLogger)
	if logger.warnCount() == 0 {
		t.Fatalf("contained panic did not WARN")
	}
}

func TestRowConsumableFreshnessRules(t *testing.T) {
	cases := []struct {
		name string
		row  map[string]any
		want bool
	}{
		{"ready with integrated", freshRow("t", "c", -20), true},
		{"ready with peak only", map[string]any{"clip_id": "c", "sample_peak_dbfs": -3.0, "status": "ready"}, true},
		{"no measurement value", map[string]any{"clip_id": "c", "status": "ready"}, false},
		{"status building", staleRow("t", "c"), false},
		{"quality stale", map[string]any{"clip_id": "c", "integrated_lufs": -20.0, "status": "ready", "quality_status": "stale"}, false},
		{"empty row", map[string]any{}, false},
	}
	for _, testCase := range cases {
		if got := rowConsumable(testCase.row, "proj-test"); got != testCase.want {
			t.Fatalf("%s: rowConsumable=%v want %v", testCase.name, got, testCase.want)
		}
	}
	if rowConsumable(map[string]any{"clip_id": "c", "integrated_lufs": -20.0, "status": "ready", "project_uuid": "proj-other"}, "proj-test") {
		t.Fatalf("foreign project row consumed")
	}
}
