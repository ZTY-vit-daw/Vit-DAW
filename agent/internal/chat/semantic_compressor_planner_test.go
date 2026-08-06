package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/semanticeffect"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func compressorPlannerCard(t *testing.T) semanticeffect.AudioProcessorIdentityCard {
	t.Helper()
	digest := plugingrabber.ParameterDigest{PluginName: "CLA-2A Stereo", Parameters: []plugingrabber.ParameterInfo{
		{ID: "peak", Name: "Peak Reduction", HostControllable: true, NormalizedValue: .4, ValueText: "40 %"},
		{ID: "gain", Name: "Output Gain", HostControllable: true, NormalizedValue: .5, ValueText: "0 dB"},
	}}
	card, boundary := plugingrabber.BuildAudioProcessorIdentityCard(digest)
	if card == nil || boundary != "" {
		t.Fatalf("card=%+v boundary=%q", card, boundary)
	}
	return *card
}

func compressorIntentJSON(goal string) string {
	data, _ := json.Marshal(semanticeffect.CompressorIntentPlan{SchemaVersion: semanticeffect.CompressorIntentPlanSchema, UserGoal: goal,
		SelectedAxes: []string{"activation_intensity"}, EvidenceRequest: semanticeffect.CompressorEvidenceRequest{PreferredMode: "paired_io", FallbackMode: "source_only", Dimensions: []string{"gain_action"}, Reason: "observe current action"},
		Reason: "amount-driven processor can serve this goal", NeedsControlBrief: true})
	return string(data)
}

func compressorDecisionJSON(value float64) string {
	data, _ := json.Marshal(compressorControlDecision{SchemaVersion: compressorControlDecisionSchema,
		Controls: []compressorControlDecisionControl{{Axis: "activation_intensity", PathKey: "main", Role: "reduction_amount",
			Target: semanticeffect.CompressorControlTarget{Percent: &value}, Purpose: "increase compression amount", Confidence: "medium"}},
		EvaluationAxes: []semanticeffect.CompressorAxisEvaluation{{Axis: "activation_intensity", DesiredDirection: "more gain action",
			COMDimensions: []string{"gain_action"}, AcceptanceCondition: "future change_delta shows bounded increase"}}})
	return string(data)
}

func TestCompressorIntentPlannerSeesIdentityCardBeforeCOM(t *testing.T) {
	goal := "compress a little more while staying natural"
	server, cfg, calls, bodies := semanticEQPlannerTestServer(t, []string{compressorIntentJSON(goal)})
	card := compressorPlannerCard(t)
	plan, err := server.planOrdinaryAgentCompressorIntent(context.Background(), "conversation-1", goal, card, cfg)
	if err != nil || plan == nil || *calls != 1 {
		t.Fatalf("plan=%+v calls=%d err=%v", plan, *calls, err)
	}
	body := (*bodies)[0]
	if !strings.Contains(body, "processor_identity_card") || !strings.Contains(body, "CLA-2A Stereo") || strings.Contains(body, "com_observation") {
		t.Fatalf("phase-1 disclosure order is wrong: %s", body)
	}
}

func TestCompressorControlPlannerUsesProgressiveBriefAndNoExecutionIdentity(t *testing.T) {
	goal := "compress a little more"
	card := compressorPlannerCard(t)
	brief := plugingrabber.CompressorControlBrief{SchemaVersion: plugingrabber.CompressorControlBriefSchema, TopologyGeneration: card.TopologyEvidence.Generation,
		SelectedAxes: []string{"activation_intensity"}, Controls: []plugingrabber.CompressorControlBriefControl{{PathKey: "main", Section: "operating_point", Role: "reduction_amount", CurrentText: "40 %", PhysicalUnit: "%", Reachable: plugingrabber.CompressorReachableSummary{Kind: "continuous", Status: "ready"}}}}
	intent := semanticeffect.CompressorIntentPlan{SchemaVersion: semanticeffect.CompressorIntentPlanSchema, UserGoal: goal, SelectedAxes: []string{"activation_intensity"},
		EvidenceRequest: semanticeffect.CompressorEvidenceRequest{PreferredMode: "source_only", Dimensions: []string{"source_dynamics"}, Reason: "material context"}, Reason: "amount driven", NeedsControlBrief: true}
	server, cfg, _, bodies := semanticEQPlannerTestServer(t, []string{compressorDecisionJSON(55)})
	plan, err := server.planOrdinaryAgentCompressorControls(context.Background(), "conversation-1", goal, "track-1", "plugin-1", "Vocal", "CLA-2A Stereo", card, intent,
		map[string]any{"mode": "source_only", "status": "ready"}, brief, nil, cfg)
	if err != nil || plan == nil {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	body := (*bodies)[0]
	for _, forbidden := range []string{"\\\"control_ref\\\"", "\\\"param_id\\\"", "\\\"current_normalized\\\""} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("planner disclosure leaked %q: %s", forbidden, body)
		}
	}
	if strings.Contains(body, "\\\"com_observation\\\"") || !strings.Contains(body, "\\\"decision_evidence\\\"") {
		t.Fatalf("planner did not receive the compact decision projection: %s", body)
	}
	if plan.Target.TrackID != "track-1" || plan.Target.PluginID != "plugin-1" || plan.IdentityCardID != card.CardID ||
		plan.TopologyGeneration != brief.TopologyGeneration || !strings.HasPrefix(plan.Controls[0].ProposalID, "cp1_") ||
		plan.Revision.SchemaVersion != semanticeffect.CompressorRevisionSchema || plan.Revision.MaxAttempts != 1 {
		t.Fatalf("deterministic plan envelope was not assembled: %+v", plan)
	}
}

func TestCompressorMaterializationRejectsUnreachableTargetAndAllowsOneRevision(t *testing.T) {
	card := compressorPlannerCard(t)
	brief := plugingrabber.CompressorControlBrief{SchemaVersion: plugingrabber.CompressorControlBriefSchema, TopologyGeneration: card.TopologyEvidence.Generation,
		Controls: []plugingrabber.CompressorControlBriefControl{{PathKey: "main", Role: "reduction_amount", PhysicalUnit: "%", Reachable: plugingrabber.CompressorReachableSummary{Kind: "continuous", Status: "ready", Minimum: floatPtr(0), Maximum: floatPtr(50)}}}}
	intent := semanticeffect.CompressorIntentPlan{SchemaVersion: semanticeffect.CompressorIntentPlanSchema, UserGoal: "compress more",
		SelectedAxes: []string{"activation_intensity"}, NegativeConstraints: []string{"stay natural"},
		EvidenceRequest: semanticeffect.CompressorEvidenceRequest{PreferredMode: "source_only", Dimensions: []string{"gain_action"}, Reason: "observe action"},
		Reason:          "serve the goal", NeedsControlBrief: true}
	comContext := map[string]any{"projection_id": "com-1", "mode": "source_only", "status": "ready", "evidence_refs": []string{"evidence-1"},
		"llm_context": map[string]any{"summary_md": "source dynamics only"}}
	plan, err := decodeCompressorControlDecisionForContext(compressorDecisionJSON(55), "track-1", "plugin-1", "compress more",
		card.CardID, brief.TopologyGeneration, intent, comContext, nil)
	if err != nil {
		t.Fatal(err)
	}
	rejection := validateCompressorPlanAgainstBrief(plan, brief)
	if rejection == nil || !rejection.RevisionAllowed || rejection.MutationPerformed {
		t.Fatalf("rejection=%+v", rejection)
	}
	revised, err := decodeCompressorControlDecisionForContext(compressorDecisionJSON(45), "track-1", "plugin-1", "compress more",
		card.CardID, brief.TopologyGeneration, intent, comContext, rejection)
	if err != nil || validateCompressorPlanAgainstBrief(revised, brief) != nil {
		t.Fatalf("revised=%+v err=%v", revised, err)
	}
	if revised.Revision.Attempt != 1 || revised.Revision.RejectionID != rejection.RejectionID ||
		len(revised.Controls) != 1 || len(revised.Controls[0].EvidenceRefs) != 1 {
		t.Fatalf("deterministic revision/evidence binding failed: %+v", revised)
	}
}

func TestCompressorPlanRejectsEnumTimingAsMillisecondsBeforeMaterialization(t *testing.T) {
	valueMS := 3.0
	brief := plugingrabber.CompressorControlBrief{TopologyGeneration: "generation-1", Controls: []plugingrabber.CompressorControlBriefControl{{
		PathKey: "main", Role: "attack", PhysicalUnit: "enum",
		Reachable: plugingrabber.CompressorReachableSummary{Kind: "discrete", Status: "ready", Labels: []string{"0.03 ms", "0.1 ms", "3 ms", "10 ms"}},
	}}}
	plan := semanticeffect.CompressorPlan{Controls: []semanticeffect.CompressorControlProposal{{
		ProposalID: "attack-ms", PathKey: "main", Role: "attack", Target: semanticeffect.CompressorControlTarget{ValueMS: &valueMS},
	}}}
	rejection := validateCompressorPlanAgainstBrief(plan, brief)
	if rejection == nil || !strings.Contains(rejection.Reason, "target type ms is incompatible") {
		t.Fatalf("unit mismatch was not rejected during planning: %+v", rejection)
	}
	plan.Controls[0].ProposalID = "attack-enum"
	plan.Controls[0].Target = semanticeffect.CompressorControlTarget{EnumLabel: "3 ms"}
	if rejection := validateCompressorPlanAgainstBrief(plan, brief); rejection != nil {
		t.Fatalf("reachable enum timing target was rejected: %+v", rejection)
	}
}

func TestSemanticCompressorDecisionProjectionFiltersAndDeduplicatesCOM(t *testing.T) {
	full := map[string]any{
		"schema_version": "com.projection.v1", "projection_id": "com-1", "mode": "paired_io", "status": "ready",
		"gain_action":        map[string]any{"status": "ready", "facts": map[string]any{"depth": 3.0}},
		"transient_response": map[string]any{"status": "ready", "facts": map[string]any{"contrast": .2}},
		"identifiability":    map[string]any{"duplicated": true}, "trust_quality": map[string]any{"overall_status": "ready", "duplicated": true},
		"evidence_refs": []string{"evidence-1"},
		"llm_context": map[string]any{
			"summary_md": "bounded behavior", "evidence_refs": []string{"evidence-1"},
			"limitation_notes": []string{"numeric_parameter_inference_forbidden"},
			"quality_summary":  map[string]any{"overall_status": "ready", "can_support_semantic_planning": true},
			"compact_facts": []map[string]any{
				{"layer": "trust_quality", "status": "ready"},
				{"layer": "identifiability", "dimension": "gain_action", "status": "identified"},
				{"layer": "identifiability", "dimension": "transient_response", "status": "identified"},
				{"layer": "gain_action", "status": "ready", "depth": 3.0},
				{"layer": "transient_response", "status": "ready", "contrast": .2},
			},
		},
	}
	decision := semanticCompressorDecisionProjection(full, []string{"gain_action"})
	encoded, err := json.Marshal(decision)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"transient_response", "\"gain_action\":{", "\"identifiability\":{", "\"trust_quality\":{"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("decision projection retained duplicate/unrequested layer %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, "\"layer\":\"gain_action\"") || !strings.Contains(text, "\"dimension\":\"gain_action\"") ||
		len(encoded) >= encodedJSONSize(full) {
		t.Fatalf("decision projection did not retain compact requested facts: %s", text)
	}
}

func TestCompressorDecisionProposalIDIgnoresExplanatoryWording(t *testing.T) {
	value := 45.0
	control := compressorControlDecisionControl{Axis: "activation_intensity", PathKey: "main", Role: "reduction_amount",
		Target: semanticeffect.CompressorControlTarget{Percent: &value}, Purpose: "first wording", Confidence: "medium"}
	first := compressorDecisionProposalID(0, control)
	control.Purpose, control.Confidence = "different wording", "high"
	if second := compressorDecisionProposalID(0, control); second != first {
		t.Fatalf("proposal identity changed with explanatory wording: %s != %s", second, first)
	}
	other := 46.0
	control.Target.Percent = &other
	if compressorDecisionProposalID(0, control) == first {
		t.Fatal("proposal identity did not change with the absolute target")
	}
}

func TestCompressorSemanticIntentRoutingKeepsExactParameterControlSeparate(t *testing.T) {
	ctx := map[string]any{"selected_track_id": "track-1", "selected_plugin_id": "plugin-1"}
	if !ordinaryAgentSemanticCompressorPlanningRequest("compress more but preserve the transient", ctx) {
		t.Fatal("abstract compressor request did not enter semantic workflow")
	}
	if ordinaryAgentSemanticCompressorPlanningRequest("set Threshold to -12 dB and Ratio to 4:1", ctx) {
		t.Fatal("exact parameter request was captured by semantic planning")
	}
	if ordinaryAgentSemanticCompressorPlanningRequest("explain why compressor pumping happens", ctx) {
		t.Fatal("discussion-only request was captured by semantic planning")
	}
	if ordinaryAgentSemanticCompressorPlanningRequest("compress the multiband compressor harder", ctx) {
		t.Fatal("multiband request crossed COM-6 boundary")
	}
}

func TestSemanticCompressorProjectionContextAcceptsTypedProjectionWithoutRawEvidence(t *testing.T) {
	projection := com.Build(com.Input{
		Mode: com.ModeSourceOnly,
		Source: com.SourceEvidence{
			ID: "source-1", SchemaVersion: "dad.com.source_projection_input.v1", Status: com.StatusReady,
			DurationSeconds: 1, EvidenceRefs: []string{"source:evidence:1"},
			Quality:      com.QualityEvidence{QualityStatus: com.StatusReady, Nonzero: true, Coverage: 1},
			TimeSegments: []com.TimeSegment{{StartSeconds: 0, EndSeconds: 1, EnergyState: "active"}},
		},
		Conditions: com.Conditions{StartSample: 0, EndSample: 48000, SampleRate: 48000, ChannelCount: 2},
	})
	context := semanticCompressorProjectionContext(&projection)
	status := context["status"]
	if context["mode"] != com.ModeSourceOnly || (status != com.StatusReady && status != com.StatusPartial) {
		t.Fatalf("typed COM projection was not adapted: %+v", context)
	}
	encoded, err := json.Marshal(context)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"time_segments", "raw_samples", "aligned_envelope_frames"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("COM context leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestSemanticCompressorCaptureContextFreezesAuthoritativeWindow(t *testing.T) {
	requestContext := map[string]any{"selected_plugin_id": "plugin-1"}
	adoptSemanticCompressorCaptureContext(requestContext, map[string]any{"com_evidence_capture": map[string]any{
		"clip_id": "clip-1", "start_sample": int64(480000), "end_sample": int64(960000),
		"sample_rate": 48000.0, "channel_count": 2,
	}})
	if requestContext["selected_clip_id"] != "clip-1" || requestContext["clip_id"] != "clip-1" ||
		requestContext["start_sample"] != int64(480000) || requestContext["end_sample"] != int64(960000) ||
		requestContext["sample_rate"] != 48000.0 || requestContext["channel_count"] != 2 {
		t.Fatalf("capture context was not frozen: %+v", requestContext)
	}
}

func TestSemanticCompressorEventCoverageRetrySelectsAndFreezesReadyWindow(t *testing.T) {
	initial := compressorObservationCandidate("partial", "resolved_clip_default_10s", 0, 10, 100, 0, 480000)
	calls := 0
	selected, audit := selectSemanticCompressorEventCoverageWindow(map[string]any{
		"selected_clip_id": "clip-1", "playhead_seconds": 0.0,
	}, initial, func(context map[string]any) (semanticCompressorObservationCandidate, error) {
		calls++
		window := mapValue(context["selected_clip_range"])
		if floatNumber(window["start_seconds"]) != 20 || floatNumber(window["end_seconds"]) != 30 {
			t.Fatalf("first retry window = %+v", window)
		}
		return compressorObservationCandidate("ready", "selected_clip_range", 20, 30, 100, 960000, 1440000), nil
	})
	if calls != 1 || !boolValue(audit["ready_found"]) || intNumber(audit["selected_attempt"]) != 1 {
		t.Fatalf("calls=%d audit=%+v", calls, audit)
	}
	requestContext := map[string]any{"selected_plugin_id": "plugin-1"}
	adoptSemanticCompressorCaptureContext(requestContext, selected.Result)
	if requestContext["start_sample"] != int64(960000) || requestContext["end_sample"] != int64(1440000) ||
		requestContext["selected_clip_id"] != "clip-1" {
		t.Fatalf("ready retry window was not frozen: %+v", requestContext)
	}
}

func TestSemanticCompressorEventCoverageRetryRespectsExplicitTimeSelection(t *testing.T) {
	initial := compressorObservationCandidate("partial", "time_selection", 12, 18, 100, 576000, 864000)
	calls := 0
	selected, audit := selectSemanticCompressorEventCoverageWindow(map[string]any{
		"time_selection": map[string]any{"active": true, "start_seconds": 12.0, "end_seconds": 18.0},
	}, initial, func(map[string]any) (semanticCompressorObservationCandidate, error) {
		calls++
		return semanticCompressorObservationCandidate{}, nil
	})
	if calls != 0 || len(audit) != 0 || selected.Result["com_evidence_capture"] == nil {
		t.Fatalf("explicit selection was retried: calls=%d audit=%+v selected=%+v", calls, audit, selected)
	}
}

func TestSemanticCompressorEventCoverageRetryKeepsInitialPartialWhenAllCandidatesFail(t *testing.T) {
	initial := compressorObservationCandidate("partial", "resolved_clip_default_10s", 0, 10, 100, 0, 480000)
	calls := 0
	selected, audit := selectSemanticCompressorEventCoverageWindow(map[string]any{"selected_clip_id": "clip-1"}, initial,
		func(context map[string]any) (semanticCompressorObservationCandidate, error) {
			calls++
			window := mapValue(context["selected_clip_range"])
			start := floatNumber(window["start_seconds"])
			return compressorObservationCandidate("partial", "selected_clip_range", start, start+10, 100,
				int64(start*48000), int64((start+10)*48000)), nil
		})
	if calls != 3 || boolValue(audit["ready_found"]) || intNumber(audit["selected_attempt"]) != 0 ||
		firstNonEmptyText(selected.Projection, "status") != "partial" {
		t.Fatalf("partial retry boundary changed: calls=%d audit=%+v selected=%+v", calls, audit, selected)
	}
	attempts, ok := audit["attempts"].([]map[string]any)
	if !ok || len(attempts) != 4 {
		t.Fatalf("retry attempts = %#v", audit["attempts"])
	}
}

func compressorObservationCandidate(status, windowSource string, start, end, duration float64,
	startSample, endSample int64) semanticCompressorObservationCandidate {
	missing := []string{}
	if status == "partial" {
		missing = []string{"event_coverage", "transient_response", "recovery_motion"}
	}
	return semanticCompressorObservationCandidate{
		Projection: map[string]any{
			"mode": "paired_io", "status": status,
			"trust_quality": map[string]any{"missing_fields": missing},
		},
		Result: map[string]any{
			"resolved_target": map[string]any{"clip_start_seconds": 0.0, "duration_seconds": duration},
			"com_evidence_capture": map[string]any{
				"clip_id": "clip-1", "window_source": windowSource,
				"start_seconds": start, "end_seconds": end,
				"start_sample": startSample, "end_sample": endSample,
				"sample_rate": 48000.0, "channel_count": 2,
			},
		},
	}
}

func floatPtr(value float64) *float64 { return &value }
