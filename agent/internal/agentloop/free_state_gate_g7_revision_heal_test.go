package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// D1-DISCLOSURE-FORENSICS-1 regression (verifyrun2 form): a mid-request
// external writer (a concurrent PCA certification's guard-track add/delete)
// advanced the authoritative project revision 2→3→4 while the conversation's
// audio closure kept its request-entry revision 2. G7 judged the model's
// kernel-minted evidence at the real current revision 4 against the frozen
// baseline 2 — an admission the model could never satisfy, because every new
// observation lands at the current revision. The healed baseline takes the
// newest ledger receipt revision when no active capability pins the tracked
// revision and no supersede booking covers it.
func g7RevisionHealState(closureExtras map[string]any) *runState {
	closure := map[string]any{"project_revision": "2"}
	for k, v := range closureExtras {
		closure[k] = v
	}
	return &runState{input: Input{Context: map[string]any{
		"minimal_audio_closure": closure,
		"free_state_reasoning_loop": map[string]any{"observation_ledger": map[string]any{
			"receipts": []any{
				map[string]any{
					"observation_id": "obs-seed", "receipt_id": "ccbr-seed", "status": "ready",
					"project_revision": "2", "evidence_refs": []any{"obs-seed"},
				},
				map[string]any{
					"observation_id": "obs-fresh", "receipt_id": "ccbr-fresh", "status": "ready",
					"freshness":     map[string]any{"class": "current_observation", "status": "ready", "project_revision": "4"},
					"evidence_refs": []any{"obs-fresh"},
				},
			},
		}},
	}}}
}

func TestG7HealsFrozenClosureRevisionFromNewerKernelReceipts(t *testing.T) {
	state := g7RevisionHealState(nil)
	if !gateG7(state, []string{"obs-fresh"}) {
		t.Fatal("evidence at the kernel-current revision must pass against the healed baseline")
	}
	if !gateG7(state, []string{"ccbr-fresh"}) {
		t.Fatal("receipt-id citations must resolve through the same healed walk")
	}
	if required := freeStateAdmissionRequiredRevision(state); required != "4" {
		t.Fatalf("required revision = %q, want the healed 4", required)
	}
	// Evidence older than the healed baseline still fails fail-closed: the seed
	// observation at revision 2 is stale against the current project state.
	if gateG7(state, []string{"obs-seed"}) {
		t.Fatal("stale seed evidence must keep failing against the healed baseline")
	}
}

func TestG7HealGuardsKeepTrackedBaseline(t *testing.T) {
	// A superseded revision never advances the baseline (governed-mutation
	// history the tracked revision already replaced).
	superseded := g7RevisionHealState(map[string]any{
		"superseded_project_revisions": map[string]any{"4": true},
	})
	if required := freeStateAdmissionRequiredRevision(superseded); required != "2" {
		t.Fatalf("superseded revision advanced the baseline to %q", required)
	}
	if gateG7(superseded, []string{"obs-fresh"}) {
		t.Fatal("a supersede booking must keep the frozen baseline verdict")
	}
	// An active capability pins the tracked revision: settlement must reconcile
	// the drift, not the admission gate.
	pinned := g7RevisionHealState(map[string]any{
		"active_capability": map[string]any{"session_id": "cap-1"},
	})
	if required := freeStateAdmissionRequiredRevision(pinned); required != "2" {
		t.Fatalf("active capability advanced the baseline to %q", required)
	}
	if gateG7(pinned, []string{"obs-fresh"}) {
		t.Fatal("an active capability must pin the tracked baseline")
	}
}

func TestG7GapNamesHealedRevision(t *testing.T) {
	state := g7RevisionHealState(nil)
	decision := &FreeStateDecision{ImprovementProposal: &agentprotocol.ImprovementProposal{
		EvidenceRefs: []string{"obs-unknown"},
	}}
	missing := gateG7MissingConditions(state, decision, []string{freeStateGateG7})
	if len(missing) == 0 {
		t.Fatal("an unresolved ref must report a missing condition")
	}
	if missing[0].Condition != "unresolved_evidence_ref" {
		t.Fatalf("condition = %q, want unresolved_evidence_ref", missing[0].Condition)
	}
	if !strings.Contains(missing[0].Freshness, "project_revision=4") {
		t.Fatalf("the gap must require the healed revision, got %q", missing[0].Freshness)
	}
}

func TestFreeStateRevisionNewerConservative(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"4", "2", true},
		{"2", "4", false},
		{"2", "2", false},
		{"", "2", false},
		{"4", "", false},
		{"rev-4", "rev-2", false},
		{"10", "9", true},
	}
	for _, tc := range cases {
		if got := freeStateRevisionNewer(tc.a, tc.b); got != tc.want {
			t.Fatalf("freeStateRevisionNewer(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
