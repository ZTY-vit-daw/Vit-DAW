package agentprotocol

import (
	"encoding/json"
	"testing"
)

func TestPendingCandidateSerializesStableKindAndStatus(t *testing.T) {
	pending := PendingCandidate{
		ID:              "pending_1",
		Kind:            KindPendingCandidate,
		Domain:          "mix",
		CandidateType:   "mix_treatment",
		TargetRef:       "track:bass",
		ActionKind:      "plugin_treatment",
		ProcessorType:   "eq",
		EvidenceRefs:    []string{"observation:obs_1"},
		NeedsResolution: []string{"live_parameter_surface"},
		Confidence:      "medium",
		Status:          PendingStatusWaitingUser,
		Source:          Source{ConversationID: "chat_1", LegacySchema: "mix_treatment_pending.v0"},
	}
	raw, err := json.Marshal(pending)
	if err != nil {
		t.Fatal(err)
	}
	var decoded PendingCandidate
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Kind != KindPendingCandidate || decoded.Status != PendingStatusWaitingUser || decoded.Source.LegacySchema == "" {
		t.Fatalf("decoded pending = %+v", decoded)
	}
	if decoded.ActionKind != "plugin_treatment" || decoded.ProcessorType != "eq" || decoded.Confidence != "medium" {
		t.Fatalf("decoded workflow fields = %+v", decoded)
	}
	if len(decoded.EvidenceRefs) != 1 || decoded.EvidenceRefs[0] != "observation:obs_1" || len(decoded.NeedsResolution) != 1 {
		t.Fatalf("decoded refs/needs = %+v", decoded)
	}
	event := NewEvent(decoded, decoded.Source)
	if event.EventType != KindPendingCandidate || event.StateKind != KindPendingCandidate || event.StateID != "pending_1" {
		t.Fatalf("event = %+v", event)
	}
}

func TestLegacyStatusAdapters(t *testing.T) {
	if got := LegacyPendingStatus("pending_confirmation"); got != PendingStatusWaitingUser {
		t.Fatalf("pending status = %q", got)
	}
	if got := LegacyApprovalStatus("needs_confirmation"); got != ApprovalStatusWaitingUser {
		t.Fatalf("approval status = %q", got)
	}
	if got := LegacyUserInputStatus("waiting_for_user"); got != UserInputStatusWaitingUser {
		t.Fatalf("input status = %q", got)
	}
}

func TestAcousticPackageStatusEventSerializes(t *testing.T) {
	status := AcousticPackageStatus{
		ID:             "acoustic_1",
		Kind:           KindAcousticPackageStatus,
		SchemaVersion:  "acoustic_package_status.v0",
		Status:         "building",
		ProjectID:      "current",
		TrackID:        "track_1",
		ClipID:         "clip_1",
		SourceRevision: "rev_1",
		PackageLayers: map[string]any{
			"l3_deep": map[string]any{"status": "building"},
		},
		Source: Source{LegacySchema: "acoustic_package_status.v0"},
	}
	event := NewEvent(status, status.Source)
	if event.EventType != KindAcousticPackageStatus || event.StateKind != KindAcousticPackageStatus || event.Status != "building" {
		t.Fatalf("event = %+v", event)
	}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Event
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.EventType != KindAcousticPackageStatus || decoded.StateID != "acoustic_1" {
		t.Fatalf("decoded = %+v", decoded)
	}
}
