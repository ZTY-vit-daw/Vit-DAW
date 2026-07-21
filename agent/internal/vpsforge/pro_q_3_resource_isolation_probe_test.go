package vpsforge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/vps"
)

func TestProQ3IsolationStateReceiptOmitsRawState(t *testing.T) {
	receipt, state, err := proQ3IsolationStateReceiptFromJSON(json.RawMessage(`{"state_sha256":"sha256:test","state_bytes":12,"state_base64":"sensitive-state-bytes"}`))
	if err != nil {
		t.Fatal(err)
	}
	if state != "sensitive-state-bytes" || !receipt.Serialized || !receipt.RawStateOmitted {
		t.Fatalf("unexpected state receipt: %#v state=%q", receipt, state)
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "state_base64") || strings.Contains(string(encoded), state) {
		t.Fatalf("serialized evidence leaked raw state: %s", encoded)
	}
}

func TestProQ3IsolationComparisonChecksCompleteParameterMap(t *testing.T) {
	expected := map[string]float64{"a": 0.25, "b": 0.75}
	matching := map[string]float64{"b": 0.75, "a": 0.25}
	comparison := proQ3IsolationCompare(expected, matching)
	if !comparison.Matches || comparison.ExpectedParameterCount != 2 || comparison.ActualParameterCount != 2 {
		t.Fatalf("matching comparison = %#v", comparison)
	}
	if comparison.ExpectedValueSHA256 != comparison.ActualValueSHA256 {
		t.Fatalf("matching parameter maps had different digests: %#v", comparison)
	}
	changed := proQ3IsolationCompare(expected, map[string]float64{"a": 0.25, "b": 0.5, "extra": 1})
	if changed.Matches || len(changed.Mismatches) == 0 {
		t.Fatalf("changed complete parameter map was accepted: %#v", changed)
	}
}

func TestProQ3ReviewSummaryStaysNonConformingAndLeavesDraftUntouched(t *testing.T) {
	root := filepath.Join(t.TempDir(), "staging", "preflight", "fabfilter-pro-q-3-test")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 17, 15, 0, 0, 0, time.UTC)
	if _, err := Init(InitRequest{
		Root: root,
		Identity: vps.PluginIdentity{
			Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3", Version: "test",
		},
		Capabilities: []string{"equalizer.v2"},
		Now:          now,
	}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, draftFile))
	if err != nil {
		t.Fatal(err)
	}
	result, err := WriteProQ3EqualizerReviewSummary(ProQ3EqualizerReviewSummaryRequest{Root: root, Now: now.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "evidence_summary_not_conformed" {
		t.Fatalf("summary status = %q", result.Status)
	}
	after, err := os.ReadFile(filepath.Join(root, draftFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("review summary changed vps_draft.json")
	}
	var summary proQ3EqualizerReviewSummary
	if err := readJSON(filepath.Join(root, result.Artifact), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Trust != "agent-inferred" || summary.Decision.CredentialIssuable || summary.Decision.CatalogMutation != "none" || summary.Decision.SPALRouteMutation != "none" {
		t.Fatalf("review authority boundary was weakened: %#v", summary.Decision)
	}
	if summary.CandidateBadge.RoutingEligible {
		t.Fatalf("candidate badge became routeable: %#v", summary.CandidateBadge)
	}
}

func TestEqualizerV2ProposalDraftStaysLocalAndNonSubmitting(t *testing.T) {
	root := filepath.Join(t.TempDir(), "staging", "preflight", "fabfilter-pro-q-3-proposal-test")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 17, 16, 0, 0, 0, time.UTC)
	if _, err := Init(InitRequest{
		Root: root,
		Identity: vps.PluginIdentity{
			Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3", Version: "test",
		},
		Capabilities: []string{"equalizer.v2"},
		Now:          now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(root, candidateBadgesFile), map[string]any{
		"schema_version": preflightSchema,
		"trust":          "agent-inferred",
		"candidates": []any{map[string]any{
			"badge_id": "equalizer.v2", "category_id": "spectral_processing", "task": "equalizer.correct_tone",
			"status": "candidate_not_granted", "routing_eligible": false,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteProQ3EqualizerReviewSummary(ProQ3EqualizerReviewSummaryRequest{Root: root, Now: now.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, draftFile))
	if err != nil {
		t.Fatal(err)
	}
	result, err := WriteEqualizerV2ConformanceProposalDraft(EqualizerV2ConformanceProposalDraftRequest{Root: root, Now: now.Add(2 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "draft_not_submitted_not_conformed" {
		t.Fatalf("proposal status = %q", result.Status)
	}
	after, err := os.ReadFile(filepath.Join(root, draftFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("proposal draft changed vps_draft.json")
	}
	var proposal equalizerV2ConformanceProposalDraft
	if err := readJSON(filepath.Join(root, result.ResultsArtifact), &proposal); err != nil {
		t.Fatal(err)
	}
	if proposal.Trust != "agent-inferred" || proposal.Target.BadgeID != "equalizer.v2" || proposal.Target.RoutingEligible {
		t.Fatalf("unexpected proposal target: %#v", proposal.Target)
	}
	if proposal.RequestedReview.SubmissionAuthorization != "not_requested" || proposal.RequestedReview.BadgeFreezeAuthorized || proposal.RequestedReview.CredentialIssuable {
		t.Fatalf("proposal draft weakened authority boundaries: %#v", proposal.RequestedReview)
	}
	if len(proposal.RequestedReview.ProposedStandardActions) != 0 {
		t.Fatalf("proposal draft invented standard actions: %#v", proposal.RequestedReview.ProposedStandardActions)
	}
}

func TestEqualizerV2ScopeReviewRecordsUserBoundaryWithoutPromotingMappings(t *testing.T) {
	root := filepath.Join(t.TempDir(), "staging", "preflight", "fabfilter-pro-q-3-scope-test")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 18, 1, 0, 0, 0, time.UTC)
	if _, err := Init(InitRequest{
		Root: root,
		Identity: vps.PluginIdentity{
			Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3", Version: "test",
		},
		Capabilities: []string{"equalizer.v2"},
		Now:          now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(root, candidateBadgesFile), map[string]any{
		"schema_version": preflightSchema,
		"trust":          "agent-inferred",
		"candidates": []any{map[string]any{
			"badge_id": "equalizer.v2", "status": "candidate_not_granted", "routing_eligible": false,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, draftFile))
	if err != nil {
		t.Fatal(err)
	}
	result, err := RecordEqualizerV2ScopeReview(EqualizerV2ScopeReviewRequest{
		Root: root, UserStatement: "确认：第一版只审核静态参量 EQ 核心，延后未单独审核扩展功能。", Now: now.Add(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, draftFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("scope review changed vps_draft.json")
	}
	var scope equalizerV2ScopeReview
	if err := readJSON(filepath.Join(root, result.ResultsArtifact), &scope); err != nil {
		t.Fatal(err)
	}
	if scope.Trust != "user-confirmed" || scope.Status != "scope_confirmed_not_conformed" || scope.CandidateTarget.RoutingEligible {
		t.Fatalf("unexpected scope record: %#v", scope)
	}
	if scope.ExecutionBoundary.ExecutableActionMappings != 0 || scope.ExecutionBoundary.BadgeFreezeAuthorized || scope.ExecutionBoundary.CredentialIssuable {
		t.Fatalf("scope record promoted authority: %#v", scope.ExecutionBoundary)
	}
	if len(scope.IncludedScope) == 0 || len(scope.ExcludedScope) == 0 || len(scope.VendorSpecial) == 0 {
		t.Fatalf("scope boundary missing expected partitions: %#v", scope)
	}
	if _, err := WriteProQ3EqualizerReviewSummary(ProQ3EqualizerReviewSummaryRequest{Root: root, Now: now.Add(2 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	var summary proQ3EqualizerReviewSummary
	if err := readJSON(filepath.Join(root, proQ3EqualizerReviewSummaryFile), &summary); err != nil {
		t.Fatal(err)
	}
	var scopeGap *proQ3ReviewGap
	for index := range summary.ReviewGaps {
		if summary.ReviewGaps[index].ID == "equalizer_v2_semantic_scope" {
			scopeGap = &summary.ReviewGaps[index]
			break
		}
	}
	if scopeGap == nil || scopeGap.Status != "scope_confirmed_not_conformed" {
		t.Fatalf("review summary did not retain the user-confirmed scope boundary: %#v", summary.ReviewGaps)
	}
}

func TestStaticEQMatrixCoverageNeverPromotesMappings(t *testing.T) {
	core := equalizerV2MatrixEvidence{Availability: "present", Trust: "observed", Status: "observed_completed"}
	if got := equalizerV2MatrixStaticCoreStatus(core, []string{"witness"}); got != "user_and_machine_evidence_assembled_not_conformed" {
		t.Fatalf("static core coverage = %q", got)
	}
	if got := equalizerV2MatrixStaticCoreStatus(core, nil); got != "evidence_incomplete_not_conformed" {
		t.Fatalf("static core without witness coverage = %q", got)
	}
	if got := equalizerV2MatrixLifecycleEligibility([]string{"witness"}); !strings.Contains(got, "non-destructive staging") {
		t.Fatalf("lifecycle eligibility unexpectedly implied execution: %q", got)
	}
}

func TestLifecycleFreshReadbackUsesActualCandidateDisplay(t *testing.T) {
	change := proQ3LifecycleChange{Role: "band_enabled", ID: "1", RequestedNormalized: 0}
	raw := json.RawMessage(`{"fresh_readback":{"parameters":[{"id":"1","normalized_value":0,"display_value":"Disabled"}]}}`)
	readback := proQ3LifecycleReadbackChanges(raw, []proQ3LifecycleChange{change})
	if len(readback) != 1 || readback[0].ObservedNormalized != 0 || readback[0].ObservedDisplay != "Disabled" {
		t.Fatalf("candidate readback = %#v", readback)
	}
}
