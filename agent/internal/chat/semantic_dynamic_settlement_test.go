package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/processorintent"
)

// semanticSettlementStateForTest feeds the bracket a scripted sequence of VSP
// state snapshots (begin consumes the pre-mutation snapshot plus the
// post-render drift re-check; complete consumes the post-mutation snapshot).
type semanticSettlementStateForTest struct {
	states []*kernel.VSPStateResult
	index  int
}

func (s *semanticSettlementStateForTest) VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error) {
	if s.index >= len(s.states) {
		return nil, fmt.Errorf("no scripted snapshot left")
	}
	out := s.states[s.index]
	s.index++
	return out, nil
}

func semanticSettlementSnapshot(revision int64) *kernel.VSPStateResult {
	return &kernel.VSPStateResult{
		Response:     map[string]any{"type": "state.snapshot"},
		Payload:      map[string]any{"snapshot_hash": fmt.Sprintf("hash-%d", revision)},
		LegacyState:  map[string]any{},
		ProjectEpoch: "epoch-settle",
		Revision:     revision,
		SnapshotHash: fmt.Sprintf("hash-%d", revision),
	}
}

// semanticSettlementDeEsserKernel extends the De-esser fake with the
// ID-carrying VSP transport so the accounted execution can echo a kernel-real
// transaction identity.
type semanticSettlementDeEsserKernel struct {
	fakeDeEsserKernel
	requestIDs []string
}

func (fake *semanticSettlementDeEsserKernel) SendVSPCommandWithIDs(ctx context.Context, command string, args map[string]any, requestID, transactionID string) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	fake.requestIDs = append(fake.requestIDs, requestID)
	result, err := fake.SendVSPCommand(ctx, command, args)
	if err != nil {
		return nil, err
	}
	// Mirrors the kernel's makeBatchTransactionId: the batch transaction id is
	// derived from the request id.
	result.TransactionID = "tx_" + requestID
	return result, nil
}

func semanticSettlementInteraction(loop freeStateReasoningLoop, ticket semanticDynamicTicket, withIntent bool) PendingInteraction {
	requestContext := map[string]any{}
	if withIntent {
		requestContext["free_state_semantic_processor_intent"] = map[string]any{
			"schema_version":    "semantic_processor_intent.v1",
			"family":            processorintent.FamilyDeEsser,
			"status":            "resolved",
			"required_coverage": []any{"sibilance_reduction"},
			"processor_type":    "de_esser",
		}
	}
	return PendingInteraction{
		ID: "interaction-settle", Kind: "confirmation", Workflow: semanticDynamicWorkflow,
		ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID,
		RequestContext: requestContext,
		Data:           map[string]any{"ticket": ticket},
	}
}

func semanticSettlementReadyBeforeRender(t *testing.T, revision string) map[string]any {
	t.Helper()
	path := filepath.Join(t.TempDir(), "before_revision_"+revision+".wav")
	payload := make([]byte, 128)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return map[string]any{"schema_version": d1RenderSchema, "phase": "before", "status": "ready",
		"file_path": path, "project_revision": revision, "checkpoint_ref": "checkpoint-settle"}
}

// RED: the D1-gated semantic dynamic execution must be bracketed with
// kernel-real settlement evidence — before/after revisions from live VSP
// snapshots, the kernel-derived transaction identity of the net write, the
// persisted before render at the pre-mutation revision, and the journaled
// forward mutation — mirroring the channel-level guarantee the port machine
// produces for native D1 executions (D2-SEMREC1 ruling 8, option (a)).
func TestSemanticDynamicSettlementBracketCapturesKernelRealEvidence(t *testing.T) {
	loop := d1DeEsserLoopForTest(t, "5")
	loop.D1State = map[string]any{"before_render": semanticSettlementReadyBeforeRender(t, "5")}
	server := New(nil, nil, nil)
	server.semanticSettlementStateOverride = &semanticSettlementStateForTest{states: []*kernel.VSPStateResult{
		semanticSettlementSnapshot(5), semanticSettlementSnapshot(5), semanticSettlementSnapshot(6),
	}}
	server.storeFreeStateLoop(loop)

	ticket := semanticDynamicTicket{SchemaVersion: semanticDynamicSchema, TicketID: "dyn_settle_test", Family: processorintent.FamilyDeEsser,
		ProcessorType: "de_esser", TrackID: "vocal", PluginID: "deesser-1", TopologyGeneration: "gen-1",
		Controls: []semanticDynamicTicketRow{{Axis: "sibilance_reduction", PathKey: "stage/de_esser_main/operating_point/reduction_range/0",
			Role: "reduction_range", Target: map[string]any{"value_db": 8}}}}
	interaction := semanticSettlementInteraction(loop, ticket, true)

	bracket, err := server.beginSemanticDynamicSettlement(context.Background(), interaction, ticket)
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if bracket == nil {
		t.Fatal("D1-gated interaction produced no settlement bracket")
	}
	if bracket.BeforeRevision != 5 || bracket.RequestID == "" || bracket.ActionID == "" {
		t.Fatalf("bracket=%+v", bracket)
	}
	restored, ok := server.freeStateLoop(loop.ConversationID)
	if !ok {
		t.Fatal("loop was not persisted")
	}
	before := firstMapFromAny(restored.D1State["before_render"])
	if firstStringFromMap(before, "status") != "ready" || firstStringFromMap(before, "project_revision") != "5" {
		t.Fatalf("before_render=%v", before)
	}

	controller := map[string]any{
		"status": "exact", "atomic": true, "track_id": "vocal", "plugin_id": "deesser-1",
		"topology_generation": "gen-1", "restore_ref": "d1rr1_settle",
		"transaction_id": "tx_kernel_real", "idempotency_key": bracket.RequestID,
		"writes": []map[string]any{{"param_id": "range", "role": "reduction_range", "requested_normalized": 1.0,
			"actual_normalized": 1.0, "actual_physical": 24.0}},
	}
	settlement, ok := server.completeSemanticDynamicSettlement(context.Background(), bracket, ticket, controller)
	if !ok {
		t.Fatal("settlement completion failed")
	}
	if settlement["before_revision"] != "5" || settlement["after_revision"] != "6" {
		t.Fatalf("revisions=%v/%v", settlement["before_revision"], settlement["after_revision"])
	}
	if settlement["transaction_id"] != "tx_kernel_real" || settlement["idempotency_key"] != bracket.RequestID {
		t.Fatalf("transaction identity=%v/%v", settlement["transaction_id"], settlement["idempotency_key"])
	}
	if settlement["param_id"] != "range" || settlement["actual_readback_value"] != 24.0 || settlement["readback_verified"] != true {
		t.Fatalf("settlement=%v", settlement)
	}
	actions := server.harness.Actions(10)
	d1 := 0
	for _, action := range actions {
		if action.Source == "free_state_d1_s1" {
			d1++
			if action.AgentActionID != bracket.ActionID || action.Status != journal.StatusSucceeded {
				t.Fatalf("journal action=%+v", action)
			}
		}
	}
	if d1 != 1 {
		t.Fatalf("expected exactly one journaled D1 forward mutation, got %d (%+v)", d1, actions)
	}
}

// RED: a mutation whose settlement cannot be proven (the after snapshot did
// not advance the revision) must report no settlement fields — the D1 chain
// refuses downstream instead of trusting an unproven revision.
func TestSemanticDynamicSettlementBracketRequiresRevisionAdvance(t *testing.T) {
	loop := d1DeEsserLoopForTest(t, "5")
	loop.D1State = map[string]any{"before_render": semanticSettlementReadyBeforeRender(t, "5")}
	server := New(nil, nil, nil)
	server.semanticSettlementStateOverride = &semanticSettlementStateForTest{states: []*kernel.VSPStateResult{
		semanticSettlementSnapshot(5), semanticSettlementSnapshot(5), semanticSettlementSnapshot(5),
	}}
	server.storeFreeStateLoop(loop)

	ticket := semanticDynamicTicket{SchemaVersion: semanticDynamicSchema, TicketID: "dyn_settle_stall", Family: processorintent.FamilyDeEsser,
		ProcessorType: "de_esser", TrackID: "vocal", PluginID: "deesser-1", TopologyGeneration: "gen-1"}
	interaction := semanticSettlementInteraction(loop, ticket, true)

	bracket, err := server.beginSemanticDynamicSettlement(context.Background(), interaction, ticket)
	if err != nil || bracket == nil {
		t.Fatalf("begin err=%v bracket=%v", err, bracket)
	}
	controller := map[string]any{"transaction_id": "tx_kernel_real", "writes": []map[string]any{{"param_id": "range", "actual_normalized": 1.0}}}
	if settlement, ok := server.completeSemanticDynamicSettlement(context.Background(), bracket, ticket, controller); ok {
		t.Fatalf("stalled revision produced settlement: %v", settlement)
	}
	if actions := server.harness.Actions(10); len(actions) != 0 {
		t.Fatalf("unproven settlement was journaled: %+v", actions)
	}
}

// RED: interactions without the free-state intent channel (C2 dynamic batch
// leaves, capability-owned sessions) keep the historical unbracketed receipt
// shape — the bridge is scoped to the free-state D1 settlement flow only.
func TestSemanticDynamicSettlementBracketIgnoredWithoutFreeStateIntent(t *testing.T) {
	loop := d1DeEsserLoopForTest(t, "5")
	loop.D1State = map[string]any{"before_render": semanticSettlementReadyBeforeRender(t, "5")}
	server := New(nil, nil, nil)
	server.semanticSettlementStateOverride = &semanticSettlementStateForTest{}
	server.storeFreeStateLoop(loop)

	ticket := semanticDynamicTicket{SchemaVersion: semanticDynamicSchema, TicketID: "dyn_settle_c2", Family: processorintent.FamilyDeEsser,
		ProcessorType: "de_esser", TrackID: "vocal", PluginID: "deesser-1", TopologyGeneration: "gen-1"}
	interaction := semanticSettlementInteraction(loop, ticket, false)

	bracket, err := server.beginSemanticDynamicSettlement(context.Background(), interaction, ticket)
	if err != nil {
		t.Fatalf("non-free-state interaction failed to skip the bracket: %v", err)
	}
	if bracket != nil {
		t.Fatalf("non-free-state interaction produced a bracket: %+v", bracket)
	}
	if override := server.semanticSettlementStateOverride.(*semanticSettlementStateForTest); override.index != 0 {
		t.Fatalf("non-free-state interaction consumed snapshots: %d", override.index)
	}
}

// semanticSettlementTransientShaperKernel extends the transient-shaper fake
// with the ID-carrying VSP transport so the accounted execution can echo a
// kernel-real transaction identity.
type semanticSettlementTransientShaperKernel struct {
	fakeTransientShaperKernel
	requestIDs []string
}

func (fake *semanticSettlementTransientShaperKernel) SendVSPCommandWithIDs(ctx context.Context, command string, args map[string]any, requestID, transactionID string) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	fake.requestIDs = append(fake.requestIDs, requestID)
	result, err := fake.SendVSPCommand(ctx, command, args)
	if err != nil {
		return nil, err
	}
	// Mirrors the kernel's makeBatchTransactionId: the batch transaction id is
	// derived from the request id.
	result.TransactionID = "tx_" + requestID
	return result, nil
}

// RED: the transient-shaper typed controller must surface the kernel
// transaction identity when the caller pins a request id, and keep the
// historical result shape when it does not (FAM2-S1 settlement bridge,
// mirroring the de_esser controller contract).
func TestTransientShaperControllerCarriesKernelTransactionWhenRequested(t *testing.T) {
	fake := &semanticSettlementTransientShaperKernel{fakeTransientShaperKernel: *newFakeTransientShaperKernel()}
	server := New(nil, nil, nil)
	server.eqKernelOverride = fake

	_, summary, err := server.readLiveTransientShaperControlSurface(context.Background(), "track-1", "transient-1")
	if err != nil {
		t.Fatal(err)
	}
	ref := transientTestControlRef(summary, "attack_amount")
	if ref == "" {
		t.Fatal("typed surface did not disclose an attack control_ref")
	}

	plain, err := server.applyPluginGrabberTransientShaperControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "transient-1", "atomic": true,
		"controls": []map[string]any{{"control_ref": ref, "percent": 50.0}},
	}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := plain["transaction_id"]; present {
		t.Fatalf("unrequested accounting leaked into the controller result: %v", plain["transaction_id"])
	}
	if _, present := plain["idempotency_key"]; present {
		t.Fatalf("unrequested idempotency leaked into the controller result: %v", plain["idempotency_key"])
	}
	if len(fake.requestIDs) != 0 {
		t.Fatalf("anonymous write rode the accounted transport: %v", fake.requestIDs)
	}

	accounted, err := server.applyPluginGrabberTransientShaperControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "transient-1", "atomic": true, "request_id": "dyn_settle_transient",
		"controls": []map[string]any{{"control_ref": ref, "percent": 50.0}},
	}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if accounted["idempotency_key"] != "dyn_settle_transient" {
		t.Fatalf("idempotency_key=%v", accounted["idempotency_key"])
	}
	transaction, _ := accounted["transaction_id"].(string)
	if strings.TrimSpace(transaction) == "" {
		t.Fatalf("kernel transaction id missing from accounted result: %+v", accounted)
	}
	if len(fake.requestIDs) != 1 || fake.requestIDs[0] != "dyn_settle_transient" {
		t.Fatalf("net write request ids=%v", fake.requestIDs)
	}
}

// RED: the De-esser typed controller must surface the kernel transaction
// identity when the caller pins a request id, and keep the historical result
// shape when it does not.
func TestDeEsserControllerCarriesKernelTransactionWhenRequested(t *testing.T) {
	fake := &semanticSettlementDeEsserKernel{fakeDeEsserKernel: *newFakeDeEsserKernel()}
	server := New(nil, nil, nil)
	server.eqKernelOverride = fake

	_, summary, err := server.readLiveDeEsserControlSurface(context.Background(), "track-1", "deesser-1")
	if err != nil {
		t.Fatal(err)
	}
	ref := deEsserTestControlRef(summary, "reduction_range")
	if ref == "" {
		t.Fatal("typed surface did not disclose a reduction range control_ref")
	}

	plain, err := server.applyPluginGrabberDeEsserControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "deesser-1", "atomic": true,
		"controls": []map[string]any{{"control_ref": ref, "value_db": 12.0}},
	}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := plain["transaction_id"]; present {
		t.Fatalf("unrequested accounting leaked into the controller result: %v", plain["transaction_id"])
	}

	accounted, err := server.applyPluginGrabberDeEsserControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "deesser-1", "atomic": true, "request_id": "dyn_settle_test",
		"controls": []map[string]any{{"control_ref": ref, "value_db": 12.0}},
	}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if accounted["idempotency_key"] != "dyn_settle_test" {
		t.Fatalf("idempotency_key=%v", accounted["idempotency_key"])
	}
	transaction, _ := accounted["transaction_id"].(string)
	if strings.TrimSpace(transaction) == "" {
		t.Fatalf("kernel transaction id missing from accounted result: %+v", accounted)
	}
	if len(fake.requestIDs) != 1 || fake.requestIDs[0] != "dyn_settle_test" {
		t.Fatalf("net write request ids=%v", fake.requestIDs)
	}
}
