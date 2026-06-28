package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
)

func TestMixTreatmentInteractionPayloadIncludesDiagnosisContext(t *testing.T) {
	diagnosis := map[string]any{
		"schema_version": "mix_diagnosis_context.v0",
		"id":             "diag_1",
		"problem_kind":   "low_mud",
		"recommendation": map[string]any{"strategy": "conservative_probe"},
	}
	payload := mixTreatmentInteractionPayload(agentloop.MixTreatmentPending{
		SchemaVersion:      "mix_treatment_pending.v0",
		Status:             "pending_confirmation",
		ConversationID:     "chat_1",
		ObservationID:      "obs_1",
		TargetRef:          "track:1007",
		ActionKind:         "plugin_treatment",
		ProcessorType:      "eq",
		DiagnosisContextID: "diag_1",
		DiagnosisContext:   diagnosis,
		EvidenceRefs:       []string{"obs_1", "diag_1"},
	})
	if payload["diagnosis_context_id"] != "diag_1" {
		t.Fatalf("payload = %+v", payload)
	}
	typed := mapValue(payload["typed_state"])
	if typed["kind"] != agentprotocol.KindPendingCandidate {
		t.Fatalf("typed state = %+v", typed)
	}
	action := mapValue(typed["candidate_action"])
	if action["diagnosis_context_id"] != "diag_1" {
		t.Fatalf("candidate action = %+v", action)
	}
}

func TestPluginPrepWorkerEvidenceSnapshotUsesDiagnosisContextMissingBand(t *testing.T) {
	diagnosis := map[string]any{
		"schema_version": "mix_diagnosis_context.v0",
		"id":             "diag_missing",
		"problem_kind":   "low_mud",
		"recommendation": map[string]any{"strategy": "conservative_probe", "confidence": "medium"},
		"evidence_status": map[string]any{
			"band_energy": "missing",
			"strategy":    "conservative_probe",
			"missing":     []any{"band_energy_summary"},
		},
		"missing_evidence": []any{map[string]any{"key": "band_energy_summary"}},
		"evidence_refs":    []any{"diag_missing.missing.band_energy_summary"},
	}
	requestContext := map[string]any{"diagnosis_context": diagnosis}
	snapshot := pluginPrepWorkerEvidenceSnapshotFromContext(nil, requestContext, nil)
	if snapshot.DiagnosisContextID != "diag_missing" || snapshot.Status["strategy"] != "conservative_probe" || snapshot.Status["band_energy"] != "missing" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	action := pluginPrepWorkerCandidateAction(
		"1007",
		"plug_1",
		"TDR Nova",
		"track:1007",
		pluginPrepWorkerControl{ParamID: "b1_gain", Label: "B1 Gain", Role: "eq_gain", Band: "B1"},
		map[string]any{},
		requestContext,
		nil,
		"低频有点糊",
		"digest_1",
	)
	if action["diagnosis_context_id"] != "diag_missing" {
		t.Fatalf("action diagnosis = %+v", action)
	}
	if !chatRefsContain(pluginPrepWorkerStringList(action["evidence_refs"]), "diag_missing") {
		t.Fatalf("evidence refs = %+v", action["evidence_refs"])
	}
}

func chatRefsContain(refs []string, needle string) bool {
	for _, ref := range refs {
		if strings.Contains(ref, needle) {
			return true
		}
	}
	return false
}
