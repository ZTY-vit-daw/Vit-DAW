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

func TestImprovementProposalValidatesAndBecomesPendingCandidate(t *testing.T) {
	proposal := ImprovementProposal{
		SchemaVersion:     ImprovementProposalSchema,
		Target:            map[string]any{"kind": "track", "id": "vox"},
		EvidenceRefs:      []string{"obs_17"},
		ImprovementIntent: "让人声更靠前且保持自然",
		Hypothesis:        "小幅改变目标域可能改善前后层次，而不宣称当前混音存在确定性错误",
		ExpectedEffect:    "人声可懂度和前后感改善",
		ActionDomain:      ImprovementActionDomainCompressor,
		ActionKind:        "bounded_treatment",
		Confidence:        0.62,
		RiskClass:         "reversible_bounded_experiment",
	}
	if err := proposal.Validate(); err != nil {
		t.Fatalf("proposal should validate: %v", err)
	}
	candidate := proposal.ToPendingCandidate("chat_1", "goal_1", "run_1", "now")
	if candidate.CandidateType != "improvement_proposal" || candidate.Domain != ImprovementActionDomainCompressor {
		t.Fatalf("unexpected candidate routing: %+v", candidate)
	}
	if candidate.TargetRef != "track:vox" || candidate.Status != PendingStatusWaitingUser {
		t.Fatalf("candidate lost target or confirmation boundary: %+v", candidate)
	}
	if candidate.CandidateAction["hypothesis"] == nil || candidate.CandidateAction["expected_effect"] == nil {
		t.Fatalf("candidate lost L3 semantics: %+v", candidate.CandidateAction)
	}
}

func TestImprovementProposalRejectsUnsupportedDomain(t *testing.T) {
	proposal := ImprovementProposal{
		SchemaVersion:     ImprovementProposalSchema,
		Target:            map[string]any{"kind": "track", "id": "vox"},
		EvidenceRefs:      []string{"obs_17"},
		ImprovementIntent: "improve",
		Hypothesis:        "bounded hypothesis",
		ExpectedEffect:    "better",
		ActionDomain:      "deterministic_truth",
		ActionKind:        "bounded_treatment",
		Confidence:        0.5,
	}
	if err := proposal.Validate(); err == nil {
		t.Fatal("unsupported action domain should be rejected")
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

// TestImprovementProposalAdmitsStaticEQ locks the D2-1 root cause fix: the
// protocol vocabulary must include static_eq so a bounded static_eq proposal
// from the model reaches the experiment admission gate instead of dying at
// the final gate with "unsupported action_domain". Parameters mirror the
// rejected proposal observed in artifacts run 20260826_210107.
func TestImprovementProposalAdmitsStaticEQ(t *testing.T) {
	proposal := ImprovementProposal{
		SchemaVersion:     ImprovementProposalSchema,
		Target:            map[string]any{"kind": "track", "id": "1027"},
		EvidenceRefs:      []string{"obs_20260826T130325_a3ef41152c2f"},
		ImprovementIntent: "衰减人声 200-400Hz 的中低频堆积",
		Hypothesis:        "在 300Hz 附近进行小幅静态 EQ 衰减可在保持整体电平平衡的同时减少中低频能量堆积",
		ExpectedEffect:    "200-400Hz 区域浑浊感减弱，人声音色更清晰透亮",
		ActionDomain:      ImprovementActionDomainStaticEQ,
		ActionKind:        "static_eq_band_adjust",
		Confidence:        0.5,
		ParameterBounds:   map[string]any{"gain_db": -1.0, "frequency_hz": 300.0, "q": 1.0},
		RiskClass:         "reversible_bounded_experiment",
	}
	if err := proposal.Validate(); err != nil {
		t.Fatalf("static_eq proposal should validate: %v", err)
	}
	candidate := proposal.ToPendingCandidate("chat_1", "goal_1", "run_1", "now")
	if candidate.Domain != ImprovementActionDomainStaticEQ || candidate.ActionKind != "static_eq_band_adjust" {
		t.Fatalf("unexpected candidate routing: %+v", candidate)
	}
}
