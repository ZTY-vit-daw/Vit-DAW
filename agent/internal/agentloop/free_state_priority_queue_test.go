package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/planner"
)

// Matrix M03/M04/M05 (docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md): the
// diagnostic priority queue, same-target/view-set re-request admission, and
// the queue-exhaustion terminal boundary.

func TestM03PriorityQueueDefaultOrderReorderAndSkipReasons(t *testing.T) {
	queue := audioclosure.DefaultPriorityQueue()
	if err := queue.Validate(); err != nil {
		t.Fatalf("default queue invalid: %v", err)
	}
	for i, want := range audioclosure.DefaultDimensionOrder {
		if queue.Entries[i].Dimension != want {
			t.Fatalf("default order position %d = %s, want %s", i, queue.Entries[i].Dimension, want)
		}
	}
	// Reordering is legal only with a recorded priority_reason.
	reordered := audioclosure.DefaultPriorityQueue()
	reordered.Entries[0], reordered.Entries[1] = reordered.Entries[1], reordered.Entries[0]
	if err := reordered.Validate(); err == nil || !strings.Contains(err.Error(), "priority_reason") {
		t.Fatalf("unexplained reorder accepted: %v", err)
	}
	reordered.Entries[0].PriorityReason = audioclosure.PriorityProjectEvidence
	reordered.Entries[1].PriorityReason = audioclosure.PriorityCost
	if err := reordered.Validate(); err != nil {
		t.Fatalf("explained reorder rejected: %v", err)
	}
	// A skip without a reason is rejected; a reason outside the five-value
	// enumeration is rejected.
	skipNoReason := audioclosure.DefaultPriorityQueue()
	skipNoReason.Entries[2].Status = audioclosure.QueueSkipped
	if err := skipNoReason.Validate(); err == nil || !strings.Contains(err.Error(), "without a valid enum reason") {
		t.Fatalf("reason-less skip accepted: %v", err)
	}
	badReason := audioclosure.DefaultPriorityQueue()
	badReason.Entries[2].Status = audioclosure.QueueSkipped
	badReason.Entries[2].SkipReason = audioclosure.SkipReason("bored")
	if err := badReason.Validate(); err == nil || !strings.Contains(err.Error(), "without a valid enum reason") {
		t.Fatalf("out-of-enum skip reason accepted: %v", err)
	}
	validSkip := audioclosure.DefaultPriorityQueue()
	validSkip.Entries[2].Status = audioclosure.QueueSkipped
	validSkip.Entries[2].SkipReason = audioclosure.SkipAlreadyCovered
	if err := validSkip.Validate(); err != nil {
		t.Fatalf("valid skip rejected: %v", err)
	}
	if validSkip.HasOpen() != true {
		t.Fatal("queue with open entries reported exhausted")
	}
	exhausted := audioclosure.PriorityQueue{SchemaVersion: audioclosure.PriorityQueueSchema}
	for _, dim := range audioclosure.DefaultDimensionOrder {
		exhausted.Entries = append(exhausted.Entries, audioclosure.PriorityQueueEntry{
			Dimension: dim, PriorityReason: audioclosure.PriorityDefaultOrder, Status: audioclosure.QueueSkipped, SkipReason: audioclosure.SkipNotApplicable,
		})
	}
	if err := exhausted.Validate(); err != nil {
		t.Fatalf("fully skipped queue invalid: %v", err)
	}
	if exhausted.HasOpen() {
		t.Fatal("fully skipped queue still reports open dimensions")
	}
	// Round records: views must stay inside the dimension→view mapping and
	// skip reasons inside the enumeration.
	validRound := audioclosure.DiagnosticRoundRecord{
		SchemaVersion: audioclosure.DiagnosticRoundSchema, RoundID: "r_abc123def456",
		PrimaryDimension: audioclosure.DimensionLevelHeadroom, PriorityReason: audioclosure.PriorityDefaultOrder,
		ViewsRequested: []string{"track.basic_energy", "mix.multitrack_relationship"},
		EvidenceStatus: audioclosure.RoundEvidenceReady, ProjectRevision: "rev-7",
	}
	if err := validRound.Validate(); err != nil {
		t.Fatalf("valid round rejected: %v", err)
	}
	outside := validRound
	outside.ViewsRequested = []string{"track.stereo_space"}
	if err := outside.Validate(); err == nil || !strings.Contains(err.Error(), "outside the allowed views") {
		t.Fatalf("cross-dimension view accepted: %v", err)
	}
	badSkip := validRound
	badSkip.SkippedDimensions = []audioclosure.SkippedDimension{{Dimension: audioclosure.DimensionDynamics, Reason: "later"}}
	if err := badSkip.Validate(); err == nil || !strings.Contains(err.Error(), "skip-reason enumeration") {
		t.Fatalf("out-of-enum round skip accepted: %v", err)
	}
}

func TestM04SameTargetReRequestThreeAdmissions(t *testing.T) {
	base := audioclosure.ReObservationRequest{
		RequestedViews:       []string{"track.basic_energy"},
		PriorViews:           []string{"track.basic_energy"},
		PriorEvidenceStatus:  audioclosure.RoundEvidenceReady,
		PriorProjectRevision: "rev-7",
		NewProjectRevision:   "rev-7",
	}
	if admitted, _ := audioclosure.AdmitReObservation(base); admitted {
		t.Fatal("plain repeat without any admission was accepted")
	}
	// Admission 1: new evidence — a NEW unresolved question (not present in the
	// prior round) plus a contradiction-priority revisit.
	newEvidence := base
	newEvidence.PriorUnresolved = []string{"level mismatch between views"}
	newEvidence.NewUnresolvedQuestions = []string{"level mismatch between views", "which track drives the masking"}
	newEvidence.NewPriorityReason = audioclosure.PriorityContradiction
	if admitted, reason := audioclosure.AdmitReObservation(newEvidence); !admitted || reason != "new_evidence_contradiction" {
		t.Fatalf("new-evidence admission rejected: %v/%s", admitted, reason)
	}
	// A contradiction declaration over the same unresolved questions is not
	// new evidence and must not re-admit the repeat.
	sameQuestions := newEvidence
	sameQuestions.NewUnresolvedQuestions = []string{"level mismatch between views"}
	if admitted, _ := audioclosure.AdmitReObservation(sameQuestions); admitted {
		t.Fatal("contradiction priority without a new question re-admitted a repeat")
	}
	// Admission 2: new project revision.
	newRevision := base
	newRevision.NewProjectRevision = "rev-8"
	if admitted, reason := audioclosure.AdmitReObservation(newRevision); !admitted || reason != "new_project_revision" {
		t.Fatalf("new-revision admission rejected: %v/%s", admitted, reason)
	}
	// Admission 3: recorded contradiction — prior stale/partial plus an
	// explicit conflict declaration.
	contradiction := base
	contradiction.PriorEvidenceStatus = audioclosure.RoundEvidencePartial
	contradiction.DeclaredContradiction = true
	if admitted, reason := audioclosure.AdmitReObservation(contradiction); !admitted || reason != "recorded_contradiction" {
		t.Fatalf("contradiction admission rejected: %v/%s", admitted, reason)
	}
	// A different view set is always its own request.
	newViewSet := base
	newViewSet.RequestedViews = []string{"track.peak_structure"}
	if admitted, _ := audioclosure.AdmitReObservation(newViewSet); !admitted {
		t.Fatal("distinct view set was treated as a repeat")
	}
	// Declared contradiction without a stale/partial prior status is not an
	// admission by itself.
	weakContradiction := base
	weakContradiction.DeclaredContradiction = true
	if admitted, _ := audioclosure.AdmitReObservation(weakContradiction); admitted {
		t.Fatal("contradiction declaration alone re-admitted a ready repeat")
	}

	// End-to-end through the message-loop gate: a repeated usable view set is
	// rejected (generalizing TestOpenSemanticRejectsRepeatingUsableProjectObservation),
	// but a changed project revision re-admits it.
	fingerprintViews := []string{"project.structure"}
	target := map[string]any{"kind": "project", "id": "current"}
	loopCtx := map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect",
		"observation_ledger": map[string]any{"receipts": []any{map[string]any{
			"status": "ready", "requested_views": []any{"project.structure"},
			"target_ref":       map[string]any{"kind": "project", "id": "current"},
			"project_revision": "rev-7",
		}}},
	}
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": loopCtx}}}
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, fingerprintViews, target); !strings.Contains(issue, "already returned usable evidence") {
		t.Fatalf("repeated usable view set accepted: %q", issue)
	}
	loopCtx["project_revision"] = "rev-8"
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, fingerprintViews, target); issue != "" {
		t.Fatalf("new-revision re-request was not admitted: %q", issue)
	}
	// The production message-loop gate also consumes the other two admissions
	// from durable/current round metadata, rather than relying on the library
	// helper alone.
	loopCtx["project_revision"] = "rev-7"
	loopCtx["reobservation"] = map[string]any{"priority_reason": "contradiction", "new_unresolved_questions": []any{"new routing conflict"}}
	loopCtx["observation_ledger"].(map[string]any)["receipts"].([]any)[0].(map[string]any)["unresolved_questions"] = []any{"old question"}
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, fingerprintViews, target); issue != "" {
		t.Fatalf("new-question contradiction re-request was not admitted: %q", issue)
	}
	loopCtx["reobservation"] = map[string]any{"declared_contradiction": true}
	loopCtx["observation_ledger"].(map[string]any)["receipts"].([]any)[0].(map[string]any)["status"] = "partial"
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, fingerprintViews, target); issue != "" {
		t.Fatalf("stale/partial contradiction re-request was not admitted: %q", issue)
	}
	// Rejected view sets remain fail-closed even when contradiction metadata is
	// present; this check is intentionally on the complete output path.
	loopCtx["observation_ledger"].(map[string]any)["rejected_view_sets"] = []any{map[string]any{"requested_views": []any{"project.structure"}, "target_ref": target}}
	if issue := messageLoopFreeStateRejectedViewSetIssue(state, fingerprintViews, target); issue == "" {
		t.Fatal("rejected view set became retryable after contradiction admission metadata")
	}
	_ = planner.ToolCall{}
}

// exhaustedQueueFixture is a valid, fully-closed/skipped queue: the shape the
// production loop derives once every diagnostic dimension has been closed.
func exhaustedQueueFixture() map[string]any {
	entries := []any{}
	for i, dim := range audioclosure.DefaultDimensionOrder {
		entry := map[string]any{"dimension": string(dim), "priority_reason": "default_order"}
		if i%2 == 0 {
			entry["status"] = "closed"
		} else {
			entry["status"] = "skipped"
			entry["skip_reason"] = "not_applicable"
		}
		entries = append(entries, entry)
	}
	return map[string]any{"schema_version": audioclosure.PriorityQueueSchema, "entries": entries}
}

func TestM05QueueExhaustionNoCandidateFoundNotPerfection(t *testing.T) {
	// An exhausted queue settles as no_candidate_found without implying the
	// project is flawless (pattern assertion, not fixed copy).
	state := &runState{input: Input{Context: map[string]any{
		"task_contract": map[string]any{"kind": "improvement"},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "inspect the project",
			"priority_queue": exhaustedQueueFixture(),
		},
	}}}
	bounded := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNoCandidateFound, EvidenceStatus: "sufficient",
		Summary:    "the diagnostic queue is exhausted; no bounded improvement candidate was established within the searched dimensions",
		Diagnostic: &FreeStateDiagnostic{SchemaVersion: FreeStateDiagnosticSchema, Status: "unresolved", Findings: nil, Limitations: []string{"queue exhausted"}},
	}}
	if issue := messageLoopFreeStateOutputIssue(state, bounded); issue != "" {
		t.Fatalf("bounded no_candidate_found rejected: %q", issue)
	}
	for _, claim := range []string{
		"the project looks perfect after inspection",
		"没有发现任何问题，工程完美",
		"nothing wrong with the mix",
	} {
		perfection := bounded
		perfection.FreeStateDecision = &FreeStateDecision{
			SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNoCandidateFound, EvidenceStatus: "sufficient",
			Summary:    claim,
			Diagnostic: &FreeStateDiagnostic{SchemaVersion: FreeStateDiagnosticSchema, Status: "unresolved", Findings: nil, Limitations: []string{"queue exhausted"}},
		}
		if issue := messageLoopFreeStateOutputIssue(state, perfection); !strings.Contains(issue, "not a clean bill of health") {
			t.Fatalf("perfection claim %q accepted: %q", claim, issue)
		}
	}
}
