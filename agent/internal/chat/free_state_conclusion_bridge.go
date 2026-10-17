package chat

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/loudnesswatch"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/projectstore"
)

// MIXLAYER-M1-1 conversation→project bridge. Settle state is keyed by
// conversation_id in the live process, while the durable stores (decision
// ledger, this conclusion layer) are keyed by project_uuid. The bridge reuses
// the project-history checkpoint anchor the free-state loop already carries:
// ensureFreeStateExperimentCheckpoint reads checkpoint_ref/commit_id (and
// project_uuid/project_revision) off loop.LatestProjectChange, so the same
// map is the primary identity source here; the judgment evidence, the
// audio-closure state, the bound projectstore roots and the harness shadow
// summary act as fallbacks.
type freeStateConclusionBridge struct {
	ProjectUUID     string
	CheckpointRef   string
	ProjectRevision string
}

func freeStateConclusionBridgeFromLoop(loop *freeStateReasoningLoop, projectUUIDHint string) freeStateConclusionBridge {
	bridge := freeStateConclusionBridge{}
	if loop != nil {
		change := loop.LatestProjectChange
		bridge.ProjectUUID = firstStringFromMap(change, "project_uuid", "project_id")
		bridge.CheckpointRef = firstStringFromMap(change, "checkpoint_ref", "commit_id")
		bridge.ProjectRevision = firstStringFromMap(change, "project_revision", "revision")
	}
	bridge.ProjectUUID = firstNonEmpty(bridge.ProjectUUID, strings.TrimSpace(projectUUIDHint))
	if bridge.ProjectUUID == "" {
		if roots, ok := projectstore.Current(); ok {
			bridge.ProjectUUID = strings.TrimSpace(roots.ProjectUUID)
		}
	}
	return bridge
}

func (s *Server) freeStateConclusionBridgeWithHarness(ctx context.Context, loop *freeStateReasoningLoop, projectUUIDHint string) freeStateConclusionBridge {
	bridge := freeStateConclusionBridgeFromLoop(loop, projectUUIDHint)
	if bridge.ProjectUUID == "" && s != nil && s.harness != nil {
		if _, projectUUID := s.harness.CurrentProjectIdentity(ctx); strings.TrimSpace(projectUUID) != "" {
			bridge.ProjectUUID = strings.TrimSpace(projectUUID)
		}
	}
	return bridge
}

// recordFreeStateSettlement persists one terminal free-state conclusion unit
// into the project-keyed conclusion store. projectUUIDHint carries an
// out-of-band identity when the caller holds a fresher one (the judgment
// evidence's project_uuid, the closure state's project uuid). The settle
// itself stays authoritative: a persistence failure only WARNs and never
// unwinds the settlement (same discipline as attachMixboardDecisionProjection).
// A bridge that cannot resolve a project uuid skips the write — the conclusion
// stays honest-by-omission rather than landing in an unaddressable location.
func (s *Server) recordFreeStateSettlement(loop *freeStateReasoningLoop, kind, outcome, summary string, judgment *mixboard.FreeStateJudgmentOutcome, projectUUIDHint string) {
	if loop == nil {
		return
	}
	bridge := s.freeStateConclusionBridgeWithHarness(context.Background(), loop, projectUUIDHint)
	if bridge.ProjectUUID == "" {
		if s != nil && s.logger != nil {
			s.logger.Warn("[free-state-conclusion] settle conclusion skipped: no project uuid resolvable for conversation=%s", loop.ConversationID)
		}
		return
	}
	record := freeStateConclusionRecordFromLoop(*loop, bridge, kind, outcome, summary, judgment, time.Now().UTC())
	if _, err := (mixboard.NewStore("")).RecordFreeStateConclusion(record); err != nil {
		if s != nil && s.logger != nil {
			s.logger.Warn("[free-state-conclusion] settle conclusion persistence failed conversation=%s conclusion=%s: %v", loop.ConversationID, record.ConclusionID, err)
		}
		return
	}
	// LOUDNESS-WATCH-TRIGGER-1 T1: the settlement landed durably, which is
	// the design's "after processing" completion fact. The watch check runs
	// in the background and only ever WARNs — the settle receipt is already
	// closed above and can never wait on, or be rolled back by, the watch.
	// The panic guard lives at the spawn point: any hook implementation
	// failure is contained here, never in the settle flow.
	if hook := s.loudnessWatchHook(); hook != nil {
		projectUUID, projectRevision, conclusionID := bridge.ProjectUUID, firstNonEmpty(record.ProjectRevision, bridge.ProjectRevision), record.ConclusionID
		go func() {
			defer func() {
				if err := recover(); err != nil {
					if s != nil && s.logger != nil {
						s.logger.Warn("[loudness-watch] settle-triggered check panicked conversation=%s err=%v", record.ConversationID, err)
					}
				}
			}()
			hook.CheckSettle(projectUUID, projectRevision, conclusionID)
		}()
	}
}

// loudnessWatcherAdapter binds the chat server's config state to the pure
// loudnesswatch runtime and exposes the hook face both the harness (T2
// piggyback, agent-loop tool cases, mix.observe disclosure) and the chat
// trigger points (T1 settle, T3 project change) consume.
type loudnessWatcherAdapter struct {
	server  *Server
	runtime *loudnesswatch.Runtime
}

// CheckSettle serves the T1 hook: settle conclusion persisted -> background
// check. The spawn site owns the panic guard; implementations may still add
// their own containment.
func (a *loudnessWatcherAdapter) CheckSettle(projectUUID, projectRevision, conclusionID string) {
	if a == nil {
		return
	}
	a.runtime.Check(loudnesswatch.TriggerContext{
		ProjectUUID:     projectUUID,
		ProjectRevision: projectRevision,
		TriggerKind:     loudnesswatch.TriggerKindSettle,
		TriggerRef:      conclusionID,
		AllowBake:       true,
	})
}

// CheckProjectChange serves the T3 hook: the free-state loop observed a
// project change. Honesty boundary (design 01 §2.3): LatestProjectChange
// updates ride the loop cadence — a purely manual user edit with no agent
// session activity is not guaranteed an immediate check; the periodic mode
// and the status tool are the explicit fallbacks.
func (a *loudnessWatcherAdapter) CheckProjectChange(change map[string]any) {
	if a == nil {
		return
	}
	a.runtime.Check(loudnesswatch.TriggerContext{
		ProjectUUID:     firstStringFromMap(change, "project_uuid", "project_id"),
		ProjectRevision: firstStringFromMap(change, "project_revision", "revision"),
		TriggerKind:     loudnesswatch.TriggerKindProjectChange,
		TriggerRef:      firstStringFromMap(change, "checkpoint_ref", "commit_id"),
		AllowBake:       true,
	})
}

// PiggybackCheck serves the harness T2 hook: comparison over the covered
// clips, never any new bake.
func (a *loudnessWatcherAdapter) PiggybackCheck(cmd map[string]any, coveredTargets []map[string]any, projectRevision string) {
	if a == nil {
		return
	}
	a.runtime.Check(loudnesswatch.TriggerContext{
		ProjectRevision: projectRevision,
		TriggerKind:     loudnesswatch.TriggerKindObservationRide,
		TriggerRef:      firstStringFromMap(cmd, "mixboard_request_id", "request_id"),
		AllowBake:       false,
		CoveredTargets:  coveredTargets,
	})
}

// Configure routes the agent-loop tool case onto the same chat handler the
// HTTP /agent/invoke face uses (card A semantics: merge-update, fail-closed).
func (a *loudnessWatcherAdapter) Configure(args map[string]any) (map[string]any, error) {
	if a == nil || a.server == nil {
		return nil, errors.New("loudness watch adapter is not bound to a server")
	}
	resp := a.server.invokeLoudnessWatchConfigure(args)
	if resp.Status != "ok" {
		return resultMapOf(resp), errors.New(firstNonEmpty(resp.Error, "configure failed"))
	}
	return resultMapOf(resp), nil
}

// Status layers the runtime's verdict-history disclosure onto card A's
// config/status result (same storage, single truth).
func (a *loudnessWatcherAdapter) Status() map[string]any {
	out := map[string]any{}
	if a != nil && a.server != nil {
		out = resultMapOf(a.server.invokeLoudnessWatchStatus(map[string]any{}))
	}
	if a != nil {
		for key, value := range a.runtime.Status() {
			out[key] = value
		}
	}
	return out
}

// Disclosure is the additive mix.observe response section (the M1-4 D1
// read-only carrier): config summary + counters + latest verdict.
func (a *loudnessWatcherAdapter) Disclosure() map[string]any {
	if a == nil {
		return map[string]any{}
	}
	return a.runtime.Status()
}

func resultMapOf(resp harness.InvokeResponse) map[string]any {
	if resp.Result != nil {
		return resp.Result
	}
	if resp.Status == "ok" {
		return map[string]any{"status": "ok"}
	}
	out := map[string]any{"status": firstNonEmpty(resp.Status, "error")}
	if resp.Error != "" {
		out["error"] = resp.Error
	}
	return out
}

// loudnessWatchHook lazily builds and registers the watch adapter on the
// harness (set-if-absent, race-free) and returns the effective hook. The
// registration points are the three hooks themselves; before the first
// free-state surface runs, the harness hook stays nil and every watch touch
// point is a no-op. A pre-registered hook (tests, future wiring) wins.
func (s *Server) loudnessWatchHook() harness.LoudnessWatchHook {
	if s == nil || s.harness == nil {
		return nil
	}
	return s.harness.EnsureLoudnessWatchHook(func() harness.LoudnessWatchHook {
		return s.buildLoudnessWatcher()
	})
}

func (s *Server) buildLoudnessWatcher() *loudnessWatcherAdapter {
	runtime := loudnesswatch.NewRuntime(loudnesswatch.Deps{
		Config: func() (loudnesswatch.Config, bool) {
			if s == nil {
				return loudnesswatch.Config{}, false
			}
			s.mu.Lock()
			config := s.loudnessWatchConfigCopyLocked()
			s.mu.Unlock()
			if config == nil {
				return loudnesswatch.Config{}, false
			}
			return *config, true
		},
		Rows: readLoudnessSummariesFromFeatureSnapshot,
		Bake: func(targets []loudnesswatch.BakeTarget) int {
			if s == nil || s.harness == nil {
				return 0
			}
			rows := make([]map[string]any, 0, len(targets))
			for _, target := range targets {
				rows = append(rows, map[string]any{
					"track_id":  target.TrackID,
					"clip_id":   target.ClipID,
					"file_path": target.FilePath,
				})
			}
			return s.harness.RequestLoudnessWatchBake(rows)
		},
		Identity: func() string {
			if roots, ok := projectstore.Current(); ok {
				return roots.ProjectUUID
			}
			return ""
		},
		StorePath: loudnessWatchVerdictStorePath(),
		Logger:    s.logger,
	})
	return &loudnessWatcherAdapter{server: s, runtime: runtime}
}

// loudnessWatchVerdictStorePath colocates the verdict history with the
// mixboard feature snapshot of the current project binding.
func loudnessWatchVerdictStorePath() string {
	return filepath.Join(filepath.Dir(mixboard.FeatureSnapshotPath(map[string]any{})), "loudness_watch_verdicts.json")
}

// readLoudnessSummariesFromFeatureSnapshot reads the loudness_summaries rows
// from the current project's mixboard feature snapshot. Read-only; a missing
// or unreadable snapshot yields no rows (the check then reports its honest
// unmeasurable reason).
func readLoudnessSummariesFromFeatureSnapshot() []map[string]any {
	path := mixboard.FeatureSnapshotPath(map[string]any{})
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	snapshot := map[string]any{}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil
	}
	rawRows, _ := snapshot["loudness_summaries"].([]any)
	rows := make([]map[string]any, 0, len(rawRows))
	for _, raw := range rawRows {
		if row, ok := raw.(map[string]any); ok && len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

func freeStateConclusionRecordFromLoop(loop freeStateReasoningLoop, bridge freeStateConclusionBridge, kind, outcome, summary string, judgment *mixboard.FreeStateJudgmentOutcome, settledAt time.Time) mixboard.FreeStateConclusionRecord {
	experimentID := ""
	if loop.Experiment != nil {
		experimentID = loop.Experiment.ID
	}
	identityKey := firstNonEmpty(experimentID, loop.LoopID, loop.ConversationID)
	record := mixboard.FreeStateConclusionRecord{
		ProjectUUID:     bridge.ProjectUUID,
		ConversationID:  loop.ConversationID,
		GoalID:          loop.GoalID,
		RunID:           loop.RunID,
		LoopID:          loop.LoopID,
		ExperimentID:    experimentID,
		Kind:            kind,
		Outcome:         outcome,
		Summary:         firstNonEmpty(strings.TrimSpace(summary), outcome),
		CheckpointRef:   bridge.CheckpointRef,
		ProjectRevision: bridge.ProjectRevision,
		TargetRef:       cloneContext(loop.TargetRef),
		Judgment:        judgment,
		SettledAt:       settledAt,
	}
	if loop.LatestDecision != nil {
		record.DecisionStatus = loop.LatestDecision.Status
	}
	if judgment != nil {
		record.EvidenceRefs = append(record.EvidenceRefs, judgment.EvidenceID)
	}
	record.ConclusionID = "fsc_" + safeConclusionPathName(identityKey) + "_" + kind
	return record
}

// safeConclusionPathName narrows an identity key into a filename-safe token
// (letters/digits/dash/underscore/dot kept, everything else collapsed to
// underscore), mirroring mixboard's safePathName discipline without exporting
// it across packages.
func safeConclusionPathName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	return builder.String()
}

// settleFreeStateExperimentTurn settles the experiment Turn, emits its
// trajectory events and records the durable conclusion unit. Every
// experiment-side settle funnel routes through here so the persisted
// conclusion cannot diverge from the settle that actually landed.
func (s *Server) settleFreeStateExperimentTurn(loop *freeStateReasoningLoop, outcome experiment.SettlementOutcome, summary string, judgment *mixboard.FreeStateJudgmentOutcome, projectUUIDHint string) error {
	if loop == nil || loop.Experiment == nil {
		return nil
	}
	events, err := loop.Experiment.Settle(outcome, summary, time.Now().UTC())
	if err != nil {
		return err
	}
	s.emitFreeStateExperimentEvents(events)
	s.recordFreeStateSettlement(loop, mixboard.FreeStateConclusionKindExperimentSettle, string(outcome), summary, judgment, projectUUIDHint)
	return nil
}

// judgmentConclusionFromEvidence projects the user's A/B judgment evidence
// into the audit metadata the conclusion record carries. The blind flag
// follows the loop's audition session snapshot when it states one; the
// authoritative blind disclosure stays in the evidence event stream.
func judgmentConclusionFromEvidence(evidence experiment.UserJudgmentEvidence, loop *freeStateReasoningLoop) *mixboard.FreeStateJudgmentOutcome {
	blind := false
	if loop != nil {
		if value, ok := loop.AuditionSessionSnapshot["blind"].(bool); ok {
			blind = value
		}
	}
	return &mixboard.FreeStateJudgmentOutcome{
		EvidenceID:        evidence.ID,
		AuditionSessionID: evidence.AuditionSessionID,
		Preference:        string(evidence.Preference),
		HeardDifference:   string(evidence.HeardDifference),
		Blind:             blind,
	}
}
