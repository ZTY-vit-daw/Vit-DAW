package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	agentruntime "vit-daw-agent/internal/runtime"
)

// FS-LEDGER-PERSIST-1 minimal counter-example: the SMOKE runs 20261010_185519
// and 20261010_201713 delivered two CCB observation requests through one pull
// session (mix scan first, track views last), but the terminal observation
// ledger kept only the track-level receipt and the admission gate G3 failed
// for a missing project scan (capability_blocked misfire).
//
// Root cause: the harness CCB executor embeds the typed
// capabilitycontext.FreeStateObservationBundle struct inside the
// map[string]any tool result, so the strict map assertion in
// freeStateCCBObservations read an empty bundle for every executed row and
// only the last observation survived via Result.RecentObservation. The
// counter-example feeds struct-valued bundles through the production
// recorder: both cross-round receipts must survive and gate G3's scan
// receipt must be in the ledger.

// forensicCCBBundleStruct mirrors the runtime shape: a typed struct with
// json tags carried inside the map result (what harness.ccbObservationRequest
// actually returns; json.Marshal renders the same view the HTTP response
// shows, which is why the artifact face looked well-formed).
type forensicCCBBundleStruct struct {
	SchemaVersion   string                    `json:"schema_version"`
	ObservationID   string                    `json:"observation_id"`
	Status          string                    `json:"status"`
	RequestedViews  []string                  `json:"requested_views"`
	TargetRef       map[string]any            `json:"target_ref"`
	ProjectBinding  map[string]any            `json:"project_binding"`
	Freshness       map[string]any            `json:"freshness"`
	OmissionReasons []string                  `json:"omission_reasons,omitempty"`
	Views           map[string]map[string]any `json:"views"`
	AuditReceipt    forensicCCBAuditStruct    `json:"audit_receipt"`
}

type forensicCCBAuditStruct struct {
	SchemaVersion    string   `json:"schema_version"`
	ReceiptID        string   `json:"receipt_id"`
	RequestedBy      string   `json:"requested_by"`
	Scope            string   `json:"scope"`
	Status           string   `json:"status"`
	ProjectRevision  string   `json:"project_revision"`
	RejectionReasons []string `json:"rejection_reasons,omitempty"`
}

func forensicStructExecRecord(toolCallID, obsID string, targetKind, targetID string, requestedViews, rejectionReasons []string) map[string]any {
	views := map[string]map[string]any{}
	omissions := append([]string(nil), rejectionReasons...)
	delivered := append([]string(nil), requestedViews...)
	if len(rejectionReasons) > 0 {
		delivered = delivered[:len(delivered)-1]
	}
	for _, viewID := range delivered {
		views[viewID] = map[string]any{"status": "partial"}
	}
	bundle := forensicCCBBundleStruct{
		SchemaVersion:   "ccb_observation_bundle.v1",
		ObservationID:   obsID,
		Status:          "partial",
		RequestedViews:  requestedViews,
		TargetRef:       map[string]any{"kind": targetKind, "id": targetID},
		ProjectBinding:  map[string]any{"project_revision": "4", "project_uuid": "vitproj_forensic", "project_epoch": "epoch_forensic"},
		Freshness:       map[string]any{"class": "current_observation", "project_revision": "4", "observed_at": "2026-10-10T10:56:49Z", "status": "partial"},
		OmissionReasons: omissions,
		Views:           views,
		AuditReceipt: forensicCCBAuditStruct{
			SchemaVersion:    "ccb_observation_receipt.v1",
			ReceiptID:        "ccbr_" + obsID,
			RequestedBy:      "model",
			Scope:            "full_project",
			Status:           "partial",
			ProjectRevision:  "4",
			RejectionReasons: rejectionReasons,
		},
	}
	return map[string]any{
		"status":       "ok",
		"tool_call_id": toolCallID,
		"tool":         "ccb.observation_request",
		"command_name": "ccb_observation_request",
		"result":       map[string]any{"bundle": bundle, "status": "partial"},
	}
}

func forensicStoreActiveLoop(t *testing.T) (*Server, freeStateReasoningLoop) {
	t.Helper()
	s := testContinuationServer()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "loop-fslp-forensic",
		ConversationID: "conversation-fslp-forensic",
		GoalID:         "goal-fslp-forensic",
		RunID:          "run-fslp-forensic",
		Status:         "observing",
		DecisionPhase:  freeStatePhaseProcessorSelection,
		OriginalIntent: "全曲诊断并给出有界改进提案",
		ActiveIntent:   "全曲诊断并给出有界改进提案",
		MaxCycles:      freeStateDefaultMaxCycles,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.storeFreeStateLoop(loop)
	return s, loop
}

func forensicNeedsExperimentResult(loop freeStateReasoningLoop, executed []map[string]any) agentloop.Result {
	return agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID,
		Status:     agentruntime.StatusCompleted,
		StopReason: agentloop.StopReasonDone,
		Reply:      "已完成全曲诊断并形成提案。",
		Executed:   executed,
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema,
			Status:        agentloop.FreeStateNeedsExperiment,
			Summary:       "candidate diagnosis established",
			ImprovementProposal: &agentprotocol.ImprovementProposal{
				SchemaVersion:     agentprotocol.ImprovementProposalSchema,
				Target:            map[string]any{"kind": "track", "id": "1037"},
				EvidenceRefs:      []string{"obs_track_forensic"},
				ImprovementIntent: "bounded high-frequency attenuation on the target track",
				Hypothesis:        "a bounded attenuation improves the harshness",
				ExpectedEffect:    "A/B comparable candidate",
				ActionDomain:      agentprotocol.ImprovementActionDomainTrackGain,
				ActionKind:        "gain_adjust",
				ParameterBounds:   map[string]any{"delta_db": -1.5},
				Confidence:        0.5,
			},
		},
	}
}

func TestFreeStateLedgerKeepsStructValuedCCBReceiptsAcrossRounds(t *testing.T) {
	s, loop := forensicStoreActiveLoop(t)
	mix := forensicStructExecRecord("tool_step_1", "obs_mix_forensic", "project", "",
		[]string{"mix.multitrack_relationship", "mix.frequency_relationship"},
		[]string{"mix.frequency_relationship: omitted by disclosure budget"})
	track := forensicStructExecRecord("tool_step_1", "obs_track_forensic", "track", "1037",
		[]string{"track.basic_energy", "track.timbre_frequency"}, nil)
	res := forensicNeedsExperimentResult(loop, []map[string]any{mix, track})

	observations := freeStateCCBObservations(res)
	if len(observations) != 2 {
		t.Fatalf("freeStateCCBObservations returned %d observations, want 2 (mix+track): %+v", len(observations), observations)
	}

	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("recordFreeStateDecision did not retain the decision")
	}
	receipts := freeStateMapRows(stored.ObservationLedger["receipts"])
	if len(receipts) != 2 {
		t.Fatalf("ledger receipts = %d, want 2 (both cross-round CCB receipts must survive): %+v", len(receipts), receipts)
	}
	scanSurvived := false
	for _, row := range receipts {
		for _, viewID := range freeStateStringSlice(row["requested_views"]) {
			if viewID == "mix.multitrack_relationship" {
				scanSurvived = true
			}
		}
	}
	if !scanSurvived {
		t.Fatal("the delivered mix scan receipt did not survive into the terminal ledger (G3 would misfire as capability_blocked)")
	}
	if len(stored.ObservationIDs) != 2 {
		t.Fatalf("observation ids = %v, want both", stored.ObservationIDs)
	}
}

// The delivered scan must also carry the facts gate G3's row predicate needs
// (agentloop/free_state_gate.go freeStateReceiptUsable + requested scan view
// not marked omitted by the disclosure budget): partial status, requested
// scan view present, and the other view's disclosure-budget omission must
// not disqualify the delivered one.
func TestFreeStateLedgerScanReceiptCarriesG3Facts(t *testing.T) {
	s, loop := forensicStoreActiveLoop(t)
	mix := forensicStructExecRecord("tool_step_1", "obs_mix_forensic", "project", "",
		[]string{"mix.multitrack_relationship", "mix.frequency_relationship"},
		[]string{"mix.frequency_relationship: omitted by disclosure budget"})
	track := forensicStructExecRecord("tool_step_1", "obs_track_forensic", "track", "1037",
		[]string{"track.basic_energy", "track.timbre_frequency"}, nil)
	res := forensicNeedsExperimentResult(loop, []map[string]any{mix, track})

	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("recordFreeStateDecision did not retain the decision")
	}
	receipts := freeStateMapRows(stored.ObservationLedger["receipts"])
	foundUsableScan := false
	for _, row := range receipts {
		status := firstStringFromMap(row, "status")
		if status != "ready" && status != "partial" {
			continue
		}
		for _, viewID := range freeStateStringSlice(row["requested_views"]) {
			if viewID != "mix.multitrack_relationship" && viewID != "mix.frequency_relationship" {
				continue
			}
			disqualified := false
			for _, reason := range freeStateStringSlice(row["rejection_reasons"]) {
				if strings.Contains(reason, "omitted by disclosure budget") {
					owner := reason
					if idx := strings.Index(reason, ":"); idx >= 0 {
						owner = reason[:idx]
					}
					if strings.EqualFold(strings.TrimSpace(owner), viewID) {
						disqualified = true
					}
				}
			}
			if !disqualified {
				foundUsableScan = true
			}
		}
	}
	if !foundUsableScan {
		t.Fatal("no ledger receipt carries a usable delivered scan view (G3 row predicate would fail)")
	}
}

// Cycle attribution pin (SMOKE 20261010_185519 secondary finding): loop.Cycle
// counts APPLIED processor actions (the interaction-answer path), not model
// rounds. A loop that settles at the admission boundary with delivered
// observations and zero applied actions legitimately keeps cycle=0 — the
// counter is not stuck; the ledger starvation above was the defect.
func TestFreeStateLoopCycleCountsActionsNotModelRounds(t *testing.T) {
	s, loop := forensicStoreActiveLoop(t)
	mix := forensicStructExecRecord("tool_step_1", "obs_mix_forensic", "project", "",
		[]string{"mix.multitrack_relationship"}, nil)
	res := forensicNeedsExperimentResult(loop, []map[string]any{mix})
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("recordFreeStateDecision did not retain the decision")
	}
	if stored.Cycle != 0 {
		t.Fatalf("a loop with zero applied actions must keep cycle=0, got %d", stored.Cycle)
	}
	if len(stored.ObservationIDs) == 0 {
		t.Fatal("observations were recorded, so the zero cycle is not an empty-run artifact")
	}
}
