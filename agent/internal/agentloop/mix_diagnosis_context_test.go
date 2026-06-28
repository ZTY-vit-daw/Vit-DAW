package agentloop

import (
	"strings"
	"testing"
	"time"

	agentruntime "vit-daw-agent/internal/runtime"
)

func TestMessageLoopAttachDiagnosisToTreatmentLowMudMissingBand(t *testing.T) {
	state := &runState{
		input: Input{UserText: "低频有点糊，先清一下"},
		goal:  agentruntime.Goal{GoalID: "goal_1", RunID: "run_1", Summary: "mix"},
		recentObservation: &RecentObservation{
			Tool:   "mix.observe",
			Status: "ok",
			Summary: map[string]any{
				"observation_id": "obs_1",
				"mix_session_id": "mix_1",
				"target_ref":     map[string]any{"kind": "track", "id": "1007"},
				"mix_package": map[string]any{
					"missing_metrics": []any{"band_energy_summary"},
					"source_capabilities": map[string]any{
						"band_energy": "missing",
					},
				},
				"deep_package": map[string]any{
					"source_capabilities": map[string]any{"deep_band_observation": "missing"},
				},
			},
		},
		startedAt: time.Unix(100, 0),
	}
	treatment := &MixTreatmentPending{
		SchemaVersion: "mix_treatment_pending.v0",
		Status:        "pending_confirmation",
		ObservationID: "obs_1",
		Intent:        "低频有点糊，先清一下",
		TargetRef:     "track:1007",
		ActionKind:    "plugin_treatment",
		ProcessorType: "eq",
		EvidenceRefs:  []string{"obs_1"},
	}
	messageLoopAttachDiagnosisToTreatment(state, treatment)
	if treatment.DiagnosisContextID == "" || state.executionMemory.MixDiagnosisContextID != treatment.DiagnosisContextID {
		t.Fatalf("diagnosis ids treatment=%q memory=%q", treatment.DiagnosisContextID, state.executionMemory.MixDiagnosisContextID)
	}
	if treatment.DiagnosisContext["schema_version"] != "mix_diagnosis_context.v0" || treatment.DiagnosisContext["problem_kind"] != "low_mud" {
		t.Fatalf("diagnosis context = %+v", treatment.DiagnosisContext)
	}
	recommendation := messageLoopMapValue(treatment.DiagnosisContext["recommendation"])
	if recommendation["strategy"] != "conservative_probe" {
		t.Fatalf("recommendation = %+v", recommendation)
	}
	if !messageLoopDiagnosisHasMissing(treatment.DiagnosisContext, "band_energy_summary") {
		t.Fatalf("missing evidence = %+v", treatment.DiagnosisContext["missing_evidence"])
	}
	if !messageLoopDiagnosisRefsContain(treatment.EvidenceRefs, treatment.DiagnosisContextID) {
		t.Fatalf("evidence refs = %+v", treatment.EvidenceRefs)
	}
	for _, row := range messageLoopMapRows(treatment.DiagnosisContext["observed_facts"]) {
		if strings.Contains(strings.ToLower(messageLoopText(row["summary"])), "buildup") || strings.Contains(messageLoopText(row["summary"]), "堆积") {
			t.Fatalf("missing band evidence must not become observed buildup: %+v", row)
		}
	}
}

func TestMessageLoopAttachDiagnosisToTreatmentPanUsesSmallPanAdjust(t *testing.T) {
	state := &runState{
		input: Input{UserText: "把吉他稍微靠左一点"},
		goal:  agentruntime.Goal{GoalID: "goal_1", RunID: "run_1"},
		recentObservation: &RecentObservation{
			Tool:   "mix.observe",
			Status: "ok",
			Summary: map[string]any{
				"observation_id": "obs_pan",
				"target_ref":     map[string]any{"kind": "track", "id": "guitar"},
				"mix_package": map[string]any{
					"missing_metrics": []any{"band_energy_summary"},
				},
			},
		},
	}
	treatment := &MixTreatmentPending{
		SchemaVersion: "mix_treatment_pending.v0",
		Status:        "pending_confirmation",
		ObservationID: "obs_pan",
		Intent:        "把吉他稍微靠左一点",
		TargetRef:     "track:guitar",
		ActionKind:    "pan_balance",
		ProcessorType: "utility",
		DeltaPan:      -0.1,
	}
	messageLoopAttachDiagnosisToTreatment(state, treatment)
	recommendation := messageLoopMapValue(treatment.DiagnosisContext["recommendation"])
	if treatment.DiagnosisContext["problem_kind"] != "pan_balance" || recommendation["strategy"] != "small_pan_adjust" {
		t.Fatalf("diagnosis context = %+v", treatment.DiagnosisContext)
	}
	if messageLoopDiagnosisHasMissing(treatment.DiagnosisContext, "band_energy_summary") {
		t.Fatalf("pan diagnosis should not block on band energy: %+v", treatment.DiagnosisContext["missing_evidence"])
	}
}

func messageLoopDiagnosisHasMissing(ctx map[string]any, key string) bool {
	for _, row := range messageLoopMapRows(ctx["missing_evidence"]) {
		if messageLoopText(row["key"]) == key {
			return true
		}
	}
	return false
}

func messageLoopDiagnosisRefsContain(refs []string, needle string) bool {
	for _, ref := range refs {
		if strings.Contains(ref, needle) {
			return true
		}
	}
	return false
}
