package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
	"vit-daw-agent/internal/shadow"
)

// D2-SEMINT1 RED form 1: a free-state admitted experiment that drives the
// semantic dynamic chain must carry a server-derived structured intent whose
// axis comes from the admitted domain-table row (de_esser -> sibilance_
// reduction), never from the model's live axis wording.

func semintAdmittedDeEsserLoop(conversationID string) freeStateReasoningLoop {
	now := time.Now().UTC()
	return freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "loop-semint",
		ConversationID: conversationID,
		GoalID:         "goal-semint",
		RunID:          "run-semint",
		Status:         "awaiting_experiment",
		OriginalIntent: "减少人声齿音",
		Experiment: &experiment.Turn{
			SchemaVersion:  "experiment.turn.v1",
			ID:             "exp-semint",
			ConversationID: conversationID,
			Status:         experiment.StatusRunning,
			CurrentRoundID: "r_semint",
			Admission: experiment.Admission{
				SchemaVersion: experiment.SchemaVersion,
				TargetRef:     map[string]any{"kind": "track", "id": "vocals"},
				EvidenceRefs:  []string{"obs-vocal", "obs-vocal", "obs-mix"},
				Hypothesis:    "bounded threshold move reduces sibilance",
				TypedAction: map[string]any{
					"action_domain": "de_esser", "action_kind": "de_esser_threshold_adjust",
					"threshold_db": -1.5,
				},
				DiagnosticDoseBounds: map[string]any{"max": 2},
				RetainedDoseBounds:   map[string]any{"max": 2},
				ExperimentBudget:     1,
				ExpectedEffect:       "less sibilance at same loudness",
				VerificationPlan:     map[string]any{"views": []string{"track.frequency_time_events"}},
				CheckpointRef:        "ckpt-semint",
				RollbackPlan:         map[string]any{"restore": true},
			},
			Rounds: []experiment.Round{{
				ID: "r_semint", Number: 1, Status: experiment.RoundObserving,
				StartedAt: now, UpdatedAt: now,
			}},
		},
		LatestObservation: &agentloop.RecentObservation{
			Tool: "ccb.observation_request", Status: "ok",
			Summary: map[string]any{
				"observation_id": "obs-vocal",
				"audit_receipt": map[string]any{
					"schema_version":           "ccb_observation_receipt.v1",
					"requested_by":             "model",
					"model_requested_view_ids": []string{"track.frequency_time_events"},
					"actual_executed_view_ids": []string{"track.frequency_time_events"},
					"view_set_matches":         true,
					"scope":                    "selected_track",
					"freshness":                map[string]any{"status": "fresh"},
					"status":                   "executed",
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestFreeStateAdmittedSemanticIntentDerivesAxisFromAdmittedDomainRow(t *testing.T) {
	loop := semintAdmittedDeEsserLoop("chat-semint")
	intent, err := freeStateAdmittedSemanticProcessorIntent(loop)
	if err != nil {
		t.Fatalf("admitted de_esser experiment failed intent derivation: %v", err)
	}
	if intent == nil {
		t.Fatal("admitted de_esser experiment derived no semantic processor intent")
	}
	if family := firstStringFromMap(intent, "family"); family != processorintent.FamilyDeEsser {
		t.Fatalf("derived family = %q, want %q", family, processorintent.FamilyDeEsser)
	}
	coverage := stringListValue(intent["required_coverage"])
	if len(coverage) != 1 || coverage[0] != "sibilance_reduction" {
		t.Fatalf("derived required_coverage = %#v, want the FAM1-S1 ruling-2 domain axis [sibilance_reduction]", coverage)
	}
	// The hypothesis text says "threshold"; the axis must not follow it.
	if strings.Contains(strings.Join(coverage, ","), "threshold") {
		t.Fatalf("derived axis followed model wording: %#v", coverage)
	}
	encoded, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := processorintent.Decode(string(encoded))
	if err != nil {
		t.Fatalf("derived intent is not a valid semantic_processor_intent.v1: %v", err)
	}
	if decoded.Status != processorintent.StatusResolved || decoded.ControlMode != processorintent.ControlModeSemantic {
		t.Fatalf("derived intent shape = %+v", decoded)
	}
	if refs := stringListValue(intent["evidence_refs"]); len(refs) != 2 || refs[0] != "obs-vocal" || refs[1] != "obs-mix" {
		t.Fatalf("derived evidence_refs = %#v, want admission refs deduplicated in order", refs)
	}
}

func TestFreeStateAdmittedSemanticIntentLeavesNonSemanticDomainsUntouched(t *testing.T) {
	if intent, err := freeStateAdmittedSemanticProcessorIntent(freeStateReasoningLoop{}); err != nil || intent != nil {
		t.Fatalf("loop without experiment derived intent=%#v err=%v", intent, err)
	}
	loop := semintAdmittedDeEsserLoop("chat-semint-native")
	loop.Experiment.Admission.TypedAction = map[string]any{
		"action_domain": "track_gain", "action_kind": "track_gain_adjust", "delta_db": 0.8,
	}
	if intent, err := freeStateAdmittedSemanticProcessorIntent(loop); err != nil || intent != nil {
		t.Fatalf("native mix-tick admission derived intent=%#v err=%v", intent, err)
	}
}

func TestAcceptedAdmittedDeEsserProposalCarriesDomainAxisIntentIntoTreatmentRouter(t *testing.T) {
	strategyJSON := `{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct","user_goal":"减少人声齿音","summary":"de-ess the vocal","choices":[` +
		`{"choice_key":"deess","role":"recommended","title":"de-esser","processor_type":"de_esser","target_mode":"load_required","reason":"sibilance is the named problem","expected_effect":"calmer highs","tradeoff":"narrow band only","confidence":"high","next_planner":"plugin_recommendation"}],` +
		`"global_constraints":[],"evidence_refs":[],"limitations":[]}`
	var strategyRequestBodies []map[string]any
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		strategyRequestBodies = append(strategyRequestBodies, payload)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": strategyJSON}}}})
	}))
	defer model.Close()
	t.Setenv("VIT_AGENT_LLM_BASE_URL", model.URL)
	t.Setenv("VIT_AGENT_LLM_API_KEY", "test")
	t.Setenv("VIT_AGENT_LLM_MODEL", "test")

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: model.Client()}
	loop := semintAdmittedDeEsserLoop("chat-semint-route")
	server.storeFreeStateLoop(loop)
	proposal := agentprotocol.ImprovementProposal{
		SchemaVersion:     agentprotocol.ImprovementProposalSchema,
		Target:            map[string]any{"kind": "track", "id": "vocals"},
		EvidenceRefs:      []string{"obs-vocal"},
		ImprovementIntent: "减少人声齿音",
		Hypothesis:        "bounded threshold move reduces sibilance",
		ExpectedEffect:    "less sibilance at same loudness",
		ActionDomain:      agentprotocol.ImprovementActionDomainDeEsser,
		ActionKind:        "de_esser_threshold_adjust",
		ParameterBounds:   map[string]any{"threshold_db": -1.5},
		Confidence:        0.5,
	}
	response := server.continueImprovementProposalInteraction(context.Background(), PendingInteraction{
		ConversationID: "chat-semint-route", GoalID: "goal-semint", RunID: "run-semint", Workflow: improvementProposalWorkflow,
		Payload: map[string]any{"proposal": agentprotocol.ToMap(proposal), "request_context": map[string]any{
			"selected_track_id": "vocals", "free_state_reasoning_loop": freeStateLoopMap(loop),
		}},
	}, "approve")
	if response.WorkflowData["action_domain_router"] != true {
		t.Fatalf("accepted de_esser proposal did not reach the governed router: %+v", response)
	}
	var routedIntent map[string]any
	for _, body := range strategyRequestBodies {
		for _, message := range mapRowsValue(body["messages"]) {
			if firstStringFromMap(message, "role") != "user" {
				continue
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(firstStringFromMap(message, "content")), &decoded); err != nil {
				continue
			}
			if intent := firstMapFromAny(decoded["semantic_processor_intent"]); len(intent) > 0 {
				routedIntent = intent
			}
		}
	}
	if len(routedIntent) == 0 {
		t.Fatalf("treatment strategy planner request carried no admitted-domain semantic intent: %#v", strategyRequestBodies)
	}
	if family := firstStringFromMap(routedIntent, "family"); family != processorintent.FamilyDeEsser {
		t.Fatalf("routed intent family = %q", family)
	}
	if coverage := stringListValue(routedIntent["required_coverage"]); len(coverage) != 1 || coverage[0] != "sibilance_reduction" {
		t.Fatalf("routed intent required_coverage = %#v, want [sibilance_reduction]", coverage)
	}
}

// The continuation must feed the progressive-disclosure orchestrator the
// authoritative CCB audit receipt. The loop's durable LatestObservation is a
// compact ledger projection whose audit keeps only identity fields; the
// authoritative model-requested receipt lives in the loop's receipt list.
// Restoring it by receipt_id keeps the orchestrator's fail-closed contract
// intact: an observation whose recorded receipt was not model-requested (or
// is missing) still fails closed exactly as before.
func TestAdmittedExperimentContinuationRestoresAuthoritativeObservationAudit(t *testing.T) {
	loop := semintAdmittedDeEsserLoop("chat-semint-audit")
	const receiptID = "ccbr_obs_20260831T105741_30a32e3a812b"
	loop.ObservationReceipts = []map[string]any{{
		"receipt_id": receiptID, "schema_version": "ccb_observation_receipt.v1",
		"requested_by": "model", "model_requested_view_ids": []string{"track.timbre_frequency"},
		"actual_executed_view_ids": []string{"track.timbre_frequency"}, "view_set_matches": true,
		"scope": "selected_track", "freshness": map[string]any{"status": "fresh"}, "status": "executed",
	}}
	loop.LatestObservation.Summary["audit_receipt"] = map[string]any{
		"receipt_id": receiptID, "schema_version": "ccb_observation_receipt.v1", "status": "ready",
	}
	restored := freeStateObservationWithAuthoritativeAudit(loop, loop.LatestObservation)
	audit := firstMapFromAny(restored.Summary["audit_receipt"])
	if firstStringFromMap(audit, "requested_by") != "model" || !boolValue(audit["view_set_matches"]) {
		t.Fatalf("authoritative audit was not restored by receipt_id: %#v", audit)
	}
	// No id-matched authoritative receipt: the compact projection stays
	// untouched so downstream boundaries keep rejecting it.
	loop.ObservationReceipts = []map[string]any{{
		"receipt_id": "ccbr_obs_other", "schema_version": "ccb_observation_receipt.v1",
		"requested_by": "model", "view_set_matches": true,
	}}
	unchanged := freeStateObservationWithAuthoritativeAudit(loop, loop.LatestObservation)
	if audit := firstMapFromAny(unchanged.Summary["audit_receipt"]); firstStringFromMap(audit, "requested_by") != "" {
		t.Fatalf("unmatched receipt fabricated an audit upgrade: %#v", audit)
	}
}

// D2-SEMINT1 RED form 2: the generic dynamic path keeps its LLM-frozen
// parameter-centric axis vocabulary byte-for-byte. The structured ride-along
// only exists on the free-state channel.
func TestGenericDynamicPathKeepsLLMParameterAxisVocabularyUnchanged(t *testing.T) {
	spec, ok := semanticDynamicSpecForFamily(processorintent.FamilyDeEsser)
	if !ok {
		t.Fatal("de_esser dynamic spec is unavailable")
	}
	allowed := false
	for _, axis := range spec.CoverageAxes {
		if axis == "threshold" {
			allowed = true
		}
	}
	if !allowed {
		t.Fatal("generic de_esser vocabulary dropped the LLM parameter-centric threshold axis")
	}
	llmFrozen := `{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"de_esser","intent":"reduce sibilance","required_coverage":["threshold"],"scope":"current_track","control_mode":"semantic_loop","confidence":0.7}`
	intent, err := semanticDynamicDecodeIntent(llmFrozen, "reduce sibilance", spec, nil)
	if err != nil {
		t.Fatalf("generic LLM-frozen threshold intent was rejected: %v", err)
	}
	if len(intent.RequiredCoverage) != 1 || intent.RequiredCoverage[0] != "threshold" {
		t.Fatalf("generic LLM-frozen axis was rewritten: %#v", intent.RequiredCoverage)
	}
	registry, err := processorregistry.Default()
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := registry.PCARequiredCoverage(processorintent.FamilyDeEsser, []string{"threshold"})
	if err != nil || len(frozen) != 1 || frozen[0].Action != "adjust" || frozen[0].Axis != "threshold_sensitivity" {
		t.Fatalf("generic parameter-axis coverage proof drifted: %#v err=%v", frozen, err)
	}
	derived, err := registry.PCARequiredCoverage(processorintent.FamilyDeEsser, []string{"sibilance_reduction"})
	if err != nil || len(derived) != 1 || derived[0].Action != "adjust" || derived[0].Axis != "sibilance_reduction" {
		t.Fatalf("domain-axis coverage proof drifted: %#v err=%v", derived, err)
	}
}

// D2-SEMINT1 RED form 3: the derived domain axis never weakens the receipt
// coverage check. A Pro-DS-shaped subject (detector_focus+sibilance_
// reduction) passes; a subject certified without the domain axis still dies
// on the same "does not cover frozen controls" wall as run 20260831_111925.
func TestFreeStateDerivedAxisStillRequiresCoveringReceipt(t *testing.T) {
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", t.TempDir()+"\\semint_attestations.v2.json")
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := processorregistry.Default()
	if err != nil {
		t.Fatal(err)
	}
	pcaCoverage, err := registry.PCARequiredCoverage(processorintent.FamilyDeEsser, []string{"sibilance_reduction"})
	if err != nil {
		t.Fatal(err)
	}
	mustReceipt := func(t *testing.T, name, identifier string, coverage []processorattestation.Coverage) semanticPCAAdmissionReceipt {
		t.Helper()
		path := t.TempDir() + "\\" + name + ".vst3"
		if err := os.WriteFile(path, []byte(name+"-semint-binary"), 0o600); err != nil {
			t.Fatal(err)
		}
		fingerprint, err := processorattestation.FingerprintPath(path)
		if err != nil {
			t.Fatal(err)
		}
		subject := processorattestation.Subject{Name: name, Manufacturer: "Test", Format: "VST3", Identifier: identifier, InstalledPath: path}
		attestation, err := store.PromoteCurrent(processorattestation.IssueSpecV2{
			Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyDeEsser,
			Coverage: coverage,
			Evidence: []processorattestation.EvidenceRef{{ReceiptID: "semint-" + name, Kind: "test", SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Now().UTC()}},
		}, "semint_test")
		if err != nil {
			t.Fatal(err)
		}
		key, err := processorattestation.BuildSubjectKey(subject)
		if err != nil {
			t.Fatal(err)
		}
		return semanticPCAAdmissionReceipt{
			ProcessorFamily: processorintent.FamilyDeEsser, Name: subject.Name, Manufacturer: subject.Manufacturer,
			Format: subject.Format, Identifier: subject.Identifier, PluginPath: subject.InstalledPath,
			SubjectKey: key, BinaryFingerprint: fingerprint, AttestationID: attestation.AttestationID,
		}
	}
	proDS := mustReceipt(t, "Pro-DS", "pro-ds-v1", []processorattestation.Coverage{
		{Action: "adjust", Axis: "detector_focus"}, {Action: "adjust", Axis: "sibilance_reduction"},
	})
	if err := semanticValidatePCAAdmissionReceiptForInput(proDS, semanticTreatmentPCAInput{
		Family: processorintent.FamilyDeEsser, RequiredCoverage: pcaCoverage,
	}); err != nil {
		t.Fatalf("Pro-DS-shaped receipt with the domain axis was rejected: %v", err)
	}
	thresholdOnly := mustReceipt(t, "Threshold-Only", "threshold-only-v1", []processorattestation.Coverage{
		{Action: "adjust", Axis: "threshold_sensitivity"},
	})
	err = semanticValidatePCAAdmissionReceiptForInput(thresholdOnly, semanticTreatmentPCAInput{
		Family: processorintent.FamilyDeEsser, RequiredCoverage: pcaCoverage,
	})
	if err == nil || !strings.Contains(err.Error(), "does not cover frozen controls") {
		t.Fatalf("receipt without the domain axis escaped the coverage wall: %v", err)
	}
}
