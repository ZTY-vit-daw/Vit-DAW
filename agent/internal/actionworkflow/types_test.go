package actionworkflow

import (
	"strings"
	"testing"
)

func TestClassifyConfirmationRequiresActivePendingForPlainApproval(t *testing.T) {
	if got := ClassifyConfirmation("可以", false); got.Kind != DecisionNone {
		t.Fatalf("approval without pending = %+v, want none", got)
	}
	if got := ClassifyConfirmation("可以", true); got.Kind != DecisionAccept || got.Intent != IntentConfirmationAccept {
		t.Fatalf("approval with pending = %+v, want accept", got)
	}
}

func TestRouteIntentSeparatesReadOnlyAndActionPreflight(t *testing.T) {
	readOnly := RouteIntent(IntentInput{UserText: "观察当前工程的频段和声像状态，不要修改。"})
	if readOnly.Intent != IntentReadOnlyObservation {
		t.Fatalf("read-only intent = %+v", readOnly)
	}
	action := RouteIntent(IntentInput{UserText: "帮我把低频稍微收一点，但先告诉我依据。"})
	if action.Intent != IntentActionPreflight {
		t.Fatalf("action intent = %+v", action)
	}
}

func TestGateFromMOMProjectionBlocksUntrustedActionPreflight(t *testing.T) {
	gate := GateFromMOMProjection(map[string]any{
		"mom_version": "v1.4",
		"intent":      "action_preflight_observation",
		"trust_quality": map[string]any{
			"overall_status":               "partial",
			"can_support_suggestion":       true,
			"can_support_action_preflight": false,
			"evidence_refs":                []any{"observation:obs_1"},
			"blocked_reasons":              []any{"action_preflight_requires_ready_band_stereo_evidence"},
		},
	})
	if gate.CanCreateExecutablePending {
		t.Fatalf("gate should block executable pending: %+v", gate)
	}
	if !gate.CanSuggest || len(gate.EvidenceRefs) != 1 || len(gate.BlockedReasons) == 0 {
		t.Fatalf("gate lost suggestion/evidence/blockers: %+v", gate)
	}
}

func TestRenderPendingUsesStableFourPartChineseDisplay(t *testing.T) {
	spec := PendingSpec{
		TargetRef:       "track:1007",
		ActionKind:      "plugin_treatment",
		ProcessorType:   "eq",
		Confidence:      "medium",
		EvidenceRefs:    []string{"observation:obs_1", "mix.read:track.1007.band"},
		NeedsResolution: []string{"plugin_instance", "exact_control"},
	}
	gate := PreflightGate{CanCreateExecutablePending: true, CanSuggest: true}
	reply := RenderReply(spec, gate)
	for _, want := range []string{"结论：", "依据：", "待确认动作：", "限制："} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q: %s", want, reply)
		}
	}
	display := RenderPending(spec, gate)
	if display.CardTitle != "混音建议待确认" || display.CardBody == "" {
		t.Fatalf("display = %+v", display)
	}
	for _, forbidden := range []string{"schema_version", "mix_treatment_pending", "raw JSON"} {
		if strings.Contains(reply, forbidden) || strings.Contains(display.CardBody, forbidden) {
			t.Fatalf("display leaked internal marker %q: reply=%s card=%s", forbidden, reply, display.CardBody)
		}
	}
}
