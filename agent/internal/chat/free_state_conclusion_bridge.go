package chat

import (
	"context"
	"strings"
	"time"

	"vit-daw-agent/internal/experiment"
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
	}
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
