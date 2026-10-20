package chat

import (
	"testing"

	"vit-daw-agent/internal/agentloop"
)

// The admission receipt must bind to the CCB bundle's revision wherever the
// bundle actually carries it. FS-LARGEPROJECT-SMOKE-1 showed receipts coming
// up empty when the revision lived only under freshness.project_revision, so
// this construction point shares the ledger's wide fallback chain
// (freeStateObservationProjectRevision) instead of reading project_binding
// alone.
func TestFreeStateAdmissionReceiptProjectRevisionFallbacks(t *testing.T) {
	cases := []struct {
		name         string
		latestChange map[string]any
		observation  map[string]any
		want         string
	}{
		{
			name:        "revision under freshness key",
			observation: map[string]any{"freshness": map[string]any{"class": "current_observation", "project_revision": "rev-4"}},
			want:        "rev-4",
		},
		{
			name:        "revision under project binding stays valid",
			observation: map[string]any{"project_binding": map[string]any{"project_revision": "rev-7"}},
			want:        "rev-7",
		},
		{
			name:         "latest project change keeps primacy over observation",
			latestChange: map[string]any{"project_revision": "rev-9"},
			observation:  map[string]any{"freshness": map[string]any{"project_revision": "rev-4"}},
			want:         "rev-9",
		},
		{
			name: "audit receipt freshness nesting",
			observation: map[string]any{
				"audit_receipt": map[string]any{"freshness": map[string]any{"project_revision": "rev-11"}},
			},
			want: "rev-11",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &Server{}
			loop := &freeStateReasoningLoop{
				ConversationID:      "conv-receipt-revision",
				LatestProjectChange: tc.latestChange,
				LatestObservation:   &agentloop.RecentObservation{ToolCallID: "obs-tool-1", Summary: tc.observation},
			}
			decision := &agentloop.FreeStateDecision{
				SchemaVersion: agentloop.FreeStateDecisionSchema,
				Status:        agentloop.FreeStateNeedsExperiment,
			}
			srv.recordFreeStateAdmissionReceipt(loop, decision, map[string]any{})
			if len(loop.AdmissionReceipt) == 0 {
				t.Fatalf("admission receipt was not recorded")
			}
			if got := firstStringFromMap(loop.AdmissionReceipt, "project_revision"); got != tc.want {
				t.Fatalf("receipt project_revision = %q, want %q (receipt: %+v)", got, tc.want, loop.AdmissionReceipt)
			}
		})
	}
}
