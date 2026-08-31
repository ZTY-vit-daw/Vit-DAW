package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/kernel"
)

// semanticSettlementState is the read-only VSP state channel the semantic
// dynamic settlement bracket captures its revision evidence through.
// Production resolves it to the kernel client; tests narrow it the same way
// eqKernelOverride narrows the EQ transport.
type semanticSettlementState interface {
	VSPStateSnapshot(ctx context.Context, scope string) (*kernel.VSPStateResult, error)
}

func (s *Server) semanticSettlementStateClient() semanticSettlementState {
	if s == nil {
		return nil
	}
	if s.semanticSettlementStateOverride != nil {
		return s.semanticSettlementStateOverride
	}
	if s.kernel == nil {
		return nil
	}
	return s.kernel
}

// semanticDynamicSettlementBracket brackets one semantic dynamic execution
// with the channel-level settlement evidence the D1 machine requires
// (D2-SEMREC1 ruling 8, option (a)). The semantic typed controller's writes
// advance the kernel project revision exactly like port writes do — plugin
// parameters live in the track ValueTree hash the compact VSP snapshot
// copies as track_state_revision — so bracketing the mutation with real
// state snapshots and the kernel-derived transaction identity is captured
// evidence, never fabrication.
type semanticDynamicSettlementBracket struct {
	ConversationID string
	GoalID         string
	RunID          string
	BeforeRevision int64
	ProjectEpoch   string
	SnapshotHash   string
	RequestID      string
	ActionID       string
}

// semanticDynamicSettlementOwner reports whether this interaction's semantic
// dynamic execution belongs to a free-state D1 experiment that is waiting for
// its single forward mutation. Only that owner gets the settlement bridge:
// C2 dynamic batch leaves and capability-owned sessions carry the c2 intent
// channel (or none) and keep the historical unbracketed receipt shape.
func (s *Server) semanticDynamicSettlementOwner(interaction PendingInteraction) (freeStateReasoningLoop, bool) {
	if s == nil {
		return freeStateReasoningLoop{}, false
	}
	if len(firstMapFromAny(interaction.RequestContext["free_state_semantic_processor_intent"])) == 0 {
		return freeStateReasoningLoop{}, false
	}
	loop, ok := s.freeStateLoop(interaction.ConversationID)
	if !ok {
		loop, ok = freeStateLoopFromAny(interaction.RequestContext["free_state_reasoning_loop"])
		if !ok {
			return freeStateReasoningLoop{}, false
		}
	}
	if loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return freeStateReasoningLoop{}, false
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil || len(round.Interventions) != 0 {
		return freeStateReasoningLoop{}, false
	}
	return loop, true
}

// beginSemanticDynamicSettlement opens the bracket before any parameter is
// written: one VSP state snapshot as the kernel-real "before" revision, the
// persisted before render at that revision (the audition settlement requires
// it and only the native D1 path created it before this bridge), and a
// re-snapshot proving the kernel state did not drift while the render was
// produced. A bracket that cannot open fails the execution before any
// mutation happens, mirroring the port path's pre-write render discipline.
func (s *Server) beginSemanticDynamicSettlement(ctx context.Context, interaction PendingInteraction, ticket semanticDynamicTicket) (*semanticDynamicSettlementBracket, error) {
	loop, owned := s.semanticDynamicSettlementOwner(interaction)
	if !owned {
		return nil, nil
	}
	state := s.semanticSettlementStateClient()
	if state == nil {
		return nil, fmt.Errorf("settlement state channel unavailable")
	}
	before, err := state.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || before == nil || !before.OK() {
		return nil, fmt.Errorf("settlement before snapshot unavailable: %v", canaryErrorText(err))
	}
	beforeRevision := before.Revision
	if beforeRevision <= 0 {
		return nil, fmt.Errorf("settlement before snapshot has no project revision")
	}
	round, roundErr := loop.Experiment.CurrentRound()
	if roundErr != nil {
		return nil, fmt.Errorf("settlement round unavailable: %v", roundErr)
	}
	if _, renderErr := s.ensureD1Render(ctx, &loop, "before", strconv.FormatInt(beforeRevision, 10), round.CheckpointRef); renderErr != nil {
		return nil, fmt.Errorf("settlement before render failed: %v", renderErr)
	}
	recheck, err := state.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || recheck == nil || !recheck.OK() {
		return nil, fmt.Errorf("settlement post-render recheck unavailable: %v", canaryErrorText(err))
	}
	if recheck.ProjectEpoch != before.ProjectEpoch || recheck.Revision != beforeRevision || recheck.SnapshotHash != before.SnapshotHash {
		return nil, fmt.Errorf("kernel state drifted while producing the before render")
	}
	return &semanticDynamicSettlementBracket{
		ConversationID: interaction.ConversationID,
		GoalID:         firstNonEmpty(interaction.GoalID, loop.GoalID),
		RunID:          firstNonEmpty(interaction.RunID, loop.RunID),
		BeforeRevision: beforeRevision,
		ProjectEpoch:   before.ProjectEpoch,
		SnapshotHash:   before.SnapshotHash,
		RequestID:      ticket.TicketID,
		ActionID:       "d1_semantic_" + sanitizeCanaryID(ticket.TicketID),
	}, nil
}

// completeSemanticDynamicSettlement closes the bracket after the typed
// controller succeeded. The after snapshot must show the same project epoch
// and a strictly advanced revision — the parameter write changed the track
// state hash, so a stall means no real forward mutation happened — and the
// controller result must carry the kernel-derived transaction identity plus
// its verified readback. ok=false means the mutation executed but its
// settlement could not be proven; the caller marks the receipt unverified and
// the D1 chain refuses downstream rather than trusting an unproven revision.
func (s *Server) completeSemanticDynamicSettlement(ctx context.Context, bracket *semanticDynamicSettlementBracket, ticket semanticDynamicTicket, controllerResult map[string]any) (map[string]any, bool) {
	if s == nil || bracket == nil {
		return nil, false
	}
	settlementGate := func(reason string) (map[string]any, bool) {
		if s.logger != nil {
			s.logger.Warn("[semantic-settlement] bracket %s could not be proven: %s (before=%d)", bracket.ActionID, reason, bracket.BeforeRevision)
		}
		return nil, false
	}
	state := s.semanticSettlementStateClient()
	if state == nil {
		return settlementGate("state channel unavailable")
	}
	after, err := state.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || after == nil || !after.OK() {
		return settlementGate("after snapshot unavailable: " + canaryErrorText(err))
	}
	if after.ProjectEpoch != bracket.ProjectEpoch {
		return settlementGate(fmt.Sprintf("epoch changed %s -> %s", bracket.ProjectEpoch, after.ProjectEpoch))
	}
	if after.Revision <= bracket.BeforeRevision {
		return settlementGate(fmt.Sprintf("revision did not advance (%d -> %d)", bracket.BeforeRevision, after.Revision))
	}
	transactionID := firstStringFromMap(controllerResult, "transaction_id")
	if transactionID == "" {
		return settlementGate("kernel transaction id missing from the controller result")
	}
	primary := semanticDynamicPrimaryWrite(controllerResult)
	if primary == nil {
		return settlementGate("controller result has no write rows")
	}
	readback, readbackOK := firstNumericAny(primary, "actual_physical")
	if !readbackOK {
		readback, readbackOK = firstNumericAny(primary, "actual_normalized")
	}
	if !readbackOK {
		return settlementGate("primary write has no readback value")
	}
	settlement := map[string]any{
		"before_revision":  strconv.FormatInt(bracket.BeforeRevision, 10),
		"after_revision":   strconv.FormatInt(after.Revision, 10),
		"applied_revision": strconv.FormatInt(after.Revision, 10),
		"transaction_id":   transactionID,
		"idempotency_key":  bracket.RequestID,
		"param_id":         firstStringFromMap(primary, "param_id"),
		// The typed controller's atomic transaction already fails closed on
		// any readback mismatch (normalized, enum, and activation checks in
		// executeEQTransaction), so reaching here means the readback was
		// verified against the written parameters.
		"readback_verified":     true,
		"settlement_source":     "semantic_dynamic_settlement_bracket.v1",
		"actual_readback_value": readback,
	}
	s.journalSemanticDynamicSettlement(bracket, ticket, settlement)
	return settlement, true
}

// semanticDynamicPrimaryWrite selects the receipt's primary write row — the
// first executed parameter — mirroring the port receipt's primary-channel
// param_id/actual_readback_value convention for multi-channel writes.
func semanticDynamicPrimaryWrite(controllerResult map[string]any) map[string]any {
	rows := mapRowsValue(controllerResult["writes"])
	if len(rows) == 0 {
		return nil
	}
	primary := rows[0]
	if firstStringFromMap(primary, "param_id") == "" {
		for _, row := range rows {
			if firstStringFromMap(row, "param_id") != "" {
				return row
			}
		}
		return nil
	}
	return primary
}

// journalSemanticDynamicSettlement records the forward mutation in the
// running journal under the free_state_d1_s1 source, describing what
// actually executed (the semantic typed controller with its confirmed
// controls) rather than fabricating a native port command shape.
func (s *Server) journalSemanticDynamicSettlement(bracket *semanticDynamicSettlementBracket, ticket semanticDynamicTicket, settlement map[string]any) {
	if s == nil || s.harness == nil || bracket == nil || strings.TrimSpace(bracket.ActionID) == "" {
		return
	}
	if _, exists := s.harness.JournalGet(bracket.ActionID); exists {
		return
	}
	controls := make([]map[string]any, 0, len(ticket.Controls))
	for _, control := range ticket.Controls {
		controls = append(controls, map[string]any{"axis": control.Axis, "path_key": control.PathKey,
			"role": control.Role, "target": cloneContext(control.Target)})
	}
	record := journal.Action{
		AgentActionID: bracket.ActionID, GoalID: bracket.GoalID, RunID: bracket.RunID, Domain: "daw", Source: "free_state_d1_s1",
		Summary: fmt.Sprintf("free-state %s semantic adjustment applied via the typed controller (settlement bracketed)", ticket.ProcessorType),
		Tool:    "semantic_dynamic_execution", CommandName: "semantic_dynamic_execution",
		Command: map[string]any{"cmd": "semantic_dynamic_execution", "track_id": ticket.TrackID, "plugin_id": ticket.PluginID,
			"ticket_id": ticket.TicketID, "family": ticket.Family, "controls": controls},
		RiskLevel: "confirm", RequiresConfirmation: true, ConfirmationStatus: "confirmed", Status: journal.StatusRunning,
	}
	s.harness.JournalRecord(record)
	s.harness.JournalMarkResult(bracket.ActionID, journal.StatusSucceeded, map[string]any{"execution_receipt": cloneContext(settlement)}, nil)
}
