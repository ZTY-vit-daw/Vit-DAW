package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

func TestMixTreatmentPendingToPendingCandidate(t *testing.T) {
	treatment := MixTreatmentPending{
		SchemaVersion:    "mix_treatment_pending.v0",
		Status:           "pending_confirmation",
		ConversationID:   "chat_1",
		ObservationID:    "obs_1",
		Intent:           "reduce low mids",
		TargetRef:        "track:bass",
		ActionKind:       "plugin_treatment",
		ProcessorType:    "eq",
		ReasoningSummary: "low-mid buildup",
		NeedsResolution:  []string{"plugin_profile"},
	}
	pending := treatment.ToPendingCandidate("chat_1", "goal_1", "run_1", "now")
	if pending.Kind != agentprotocol.KindPendingCandidate || pending.Status != agentprotocol.PendingStatusWaitingUser {
		t.Fatalf("pending identity = %+v", pending)
	}
	if pending.Domain != "mix" || pending.CandidateType != "mix_treatment" || pending.TargetRef != "track:bass" {
		t.Fatalf("pending routing = %+v", pending)
	}
	if len(pending.RequiredPermissionDomains) == 0 || pending.RequiredPermissionDomains[0] != "plugin.load" {
		t.Fatalf("permission domains = %+v", pending.RequiredPermissionDomains)
	}
	if pending.Source.LegacySchema != "mix_treatment_pending.v0" || pending.Source.GoalID != "goal_1" {
		t.Fatalf("source = %+v", pending.Source)
	}
	if strings.Contains(pending.Rationale, "low-mid") || !strings.Contains(pending.Rationale, "插件处理候选") {
		t.Fatalf("rationale was not localized: %q", pending.Rationale)
	}
	if strings.Contains(pending.Summary, "plugin_treatment") || !strings.Contains(pending.Summary, "准备 EQ 插件处理") {
		t.Fatalf("summary was not localized: %q", pending.Summary)
	}
}

func TestPendingMixTickCandidateToPendingCandidate(t *testing.T) {
	candidate := PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       "1007",
		DeltaDB:       -1,
		ObservationID: "obs_1",
		Status:        "pending_confirmation",
		Evidence:      map[string]any{"reason": "headroom_risk"},
	}
	pending := candidate.ToPendingCandidate("chat_1", "goal_1", "run_1", "")
	if pending.Kind != agentprotocol.KindPendingCandidate || pending.CandidateType != "mix_tick" {
		t.Fatalf("pending = %+v", pending)
	}
	if pending.TargetRef != "track:1007" || pending.CandidateAction["delta_db"] != -1.0 {
		t.Fatalf("candidate action = %+v target=%q", pending.CandidateAction, pending.TargetRef)
	}
	if len(pending.RequiredPermissionDomains) != 1 || pending.RequiredPermissionDomains[0] != "mix.commit" {
		t.Fatalf("permission domains = %+v", pending.RequiredPermissionDomains)
	}
}

func TestObservationOnlyNeverBecomesPendingCandidate(t *testing.T) {
	memory := ExecutionMemory{
		PendingMixTreatment: &MixTreatmentPending{
			SchemaVersion: "mix_treatment_pending.v0",
			Status:        "pending_confirmation",
			ActionKind:    "observation_only",
			TargetRef:     "project",
		},
	}
	if pending := PendingCandidateFromExecutionMemory(memory, "chat_1", "goal_1", "run_1", "now"); pending != nil {
		t.Fatalf("observation_only converted to pending candidate: %+v", pending)
	}
}

func TestObservationResultDoesNotEmitApprovalRequest(t *testing.T) {
	memory := ExecutionMemory{
		PendingMixTreatment: &MixTreatmentPending{
			SchemaVersion: "mix_treatment_pending.v0",
			Status:        "observation_only",
			ActionKind:    "observation_only",
			TargetRef:     "project",
		},
	}
	if pending := PendingCandidateFromExecutionMemory(memory, "chat_1", "goal_1", "run_1", "now"); pending != nil {
		t.Fatalf("observation result produced pending/approval path: %+v", pending)
	}
}
