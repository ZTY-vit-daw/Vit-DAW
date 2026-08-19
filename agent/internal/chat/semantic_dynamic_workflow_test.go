package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
	"vit-daw-agent/internal/shadow"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func dynamicTestBinding(role, ref string) map[string]any {
	return map[string]any{"param_id": "vendor_param_should_not_leak", "role": role, "control_ref": ref, "current_text": "-12 dB", "physical_unit": "dB", "current_physical": -12.0}
}

func dynamicTestSummary(family string) map[string]any {
	base := map[string]any{"control_topology": map[string]any{"generation": "generation-test"}}
	rows := func(bindings ...map[string]any) []map[string]any { return bindings }
	switch family {
	case processorintent.FamilyLimiter:
		base["limiter_stages"] = []map[string]any{{"stage_key": "limiter_main", "operating_point": rows(dynamicTestBinding("threshold", "l-threshold")), "safety": rows(dynamicTestBinding("ceiling", "l-ceiling")), "timing": rows(dynamicTestBinding("release", "l-release"))}}
	case processorintent.FamilyGateExpander:
		base["gate_expander_stage"] = map[string]any{"stage_key": "gate_main", "operating_point": rows(dynamicTestBinding("threshold", "g-threshold")), "gain_action": rows(dynamicTestBinding("range", "g-range")), "timing": rows(dynamicTestBinding("release", "g-release"))}
	case processorintent.FamilyDeEsser:
		base["de_esser_stages"] = []map[string]any{{"stage_key": "deesser_main", "operating_point": rows(dynamicTestBinding("threshold", "d-threshold"), dynamicTestBinding("reduction_range", "d-range")), "frequency_selectivity": rows(dynamicTestBinding("focus_frequency", "d-frequency")), "timing": rows(dynamicTestBinding("release", "d-release"))}}
	case processorintent.FamilyTransientShaper:
		base["transient_shaper_stage"] = map[string]any{"stage_key": "transient_main", "envelope_action": rows(dynamicTestBinding("attack_amount", "t-attack")), "detector": rows(dynamicTestBinding("focus_frequency", "t-focus")), "timing": rows(dynamicTestBinding("duration", "t-duration"))}
	case processorintent.FamilyMultibandDynamics:
		base["filterbank"] = map[string]any{"crossovers": rows(dynamicTestBinding("crossover", "m-crossover"))}
		base["band_cells"] = []map[string]any{{"band_key": "low", "operating_point": rows(dynamicTestBinding("threshold", "m-threshold")), "timing": rows(dynamicTestBinding("attack", "m-attack"))}}
		base["shared_controls"] = rows(dynamicTestBinding("mix", "m-mix"))
	}
	return base
}

func TestSemanticDynamicBoundaryMatrixKeepsSpectralAndClipperInspectOnly(t *testing.T) {
	registry, err := processorregistry.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{processorintent.FamilySpectralDynamics, processorintent.FamilyClipper} {
		definition, ok := registry.Resolve(family)
		if !ok || !definition.InspectOnly || definition.Planner != "inspect_only" || definition.TypedExecutor != "none" {
			t.Fatalf("boundary family %s is executable: %#v", family, definition)
		}
		if _, ok := semanticDynamicSpecForFamily(family); ok {
			t.Fatalf("boundary family %s entered generic dynamic workflow", family)
		}
		if _, _, ok := semanticPostLoadAdapterForProcessorType(family); ok {
			t.Fatalf("boundary family %s entered post-load adapter", family)
		}
		if _, err := registry.PCARequiredCoverage(family, []string{"inspect_only"}); err == nil {
			t.Fatalf("boundary family %s received executable PCA coverage", family)
		}
		if processorAttestationFamily(family) != "" || semanticTreatmentInstanceExecutable(
			semanticTreatmentInstance{ProcessorType: family}, "limiter", "semantic_limiter") {
			t.Fatalf("boundary family %s crossed Limiter candidate admission", family)
		}
	}
}

func TestSemanticDynamicSpecsCoverAllFiveFamilies(t *testing.T) {
	families := []string{processorintent.FamilyLimiter, processorintent.FamilyGateExpander, processorintent.FamilyDeEsser, processorintent.FamilyTransientShaper, processorintent.FamilyMultibandDynamics}
	for _, family := range families {
		spec, ok := semanticDynamicSpecForFamily(family)
		if !ok || spec.Planner == "" || spec.ApplyTool == "" || len(spec.CoverageAxes) == 0 {
			t.Fatalf("family %s spec=%+v ok=%v", family, spec, ok)
		}
	}
}

func TestSemanticDynamicBriefIsStructuralAndDoesNotExposeParameterIDs(t *testing.T) {
	cases := []struct {
		family string
		axes   []string
	}{
		{processorintent.FamilyLimiter, []string{"threshold", "ceiling", "release"}},
		{processorintent.FamilyGateExpander, []string{"threshold", "range", "release"}},
		{processorintent.FamilyDeEsser, []string{"threshold", "frequency_focus", "release"}},
		{processorintent.FamilyTransientShaper, []string{"attack", "duration"}},
		{processorintent.FamilyMultibandDynamics, []string{"threshold", "attack", "crossover", "mix"}},
	}
	for _, test := range cases {
		spec, _ := semanticDynamicSpecForFamily(test.family)
		brief, bindings, err := semanticDynamicBriefFor(spec, dynamicTestSummary(test.family), test.axes)
		if err != nil || len(brief.Controls) == 0 || len(bindings) != len(brief.Controls) {
			t.Fatalf("family %s brief=%+v bindings=%+v err=%v", test.family, brief, bindings, err)
		}
		encoded, _ := json.Marshal(brief)
		text := string(encoded)
		if strings.Contains(text, "param_id") || strings.Contains(text, "control_ref") || strings.Contains(text, "vendor_param") {
			t.Fatalf("family %s leaked execution identity: %s", test.family, text)
		}
	}
}

func TestSemanticDynamicDecisionBindsOnlyDisclosedPathAndPhysicalTarget(t *testing.T) {
	spec, _ := semanticDynamicSpecForFamily(processorintent.FamilyLimiter)
	brief, _, err := semanticDynamicBriefFor(spec, dynamicTestSummary(spec.Family), []string{"threshold"})
	if err != nil {
		t.Fatal(err)
	}
	decisionJSON := `{"schema_version":"semantic_dynamic_control_decision.v1","controls":[{"axis":"threshold","path_key":"stage/limiter_main/operating_point/threshold/0","role":"threshold","target":{"value_db":-14},"purpose":"reduce peak drive","confidence":"high"}]}`
	intent := processorintent.Intent{SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved, Family: spec.Family, Intent: "protect peaks", RequiredCoverage: []string{"threshold"}, Scope: processorintent.ScopeCurrentTrack, ControlMode: processorintent.ControlModeSemantic, Confidence: .9}
	decision, err := decodeSemanticDynamicDecision(decisionJSON, intent, brief)
	if err != nil || len(decision.Controls) != 1 {
		t.Fatalf("valid decision=%+v err=%v", decision, err)
	}
	for _, bad := range []string{
		`{"schema_version":"semantic_dynamic_control_decision.v1","controls":[{"axis":"threshold","path_key":"unknown","role":"threshold","target":{"value_db":-14},"purpose":"x","confidence":"high"}]}`,
		`{"schema_version":"semantic_dynamic_control_decision.v1","controls":[{"axis":"threshold","path_key":"stage/limiter_main/operating_point/threshold/0","role":"threshold","target":{"value_db":-14,"control_ref":"x"},"purpose":"x","confidence":"high"}]}`,
	} {
		if _, err := decodeSemanticDynamicDecision(bad, intent, brief); err == nil {
			t.Fatalf("out-of-bound decision was accepted: %s", bad)
		}
	}
}

func TestSemanticDynamicDecisionCompletesOnlyAnUnambiguousServerBinding(t *testing.T) {
	spec, _ := semanticDynamicSpecForFamily(processorintent.FamilyLimiter)
	brief, _, err := semanticDynamicBriefFor(spec, dynamicTestSummary(spec.Family), []string{"threshold"})
	if err != nil {
		t.Fatal(err)
	}
	intent := processorintent.Intent{SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved, Family: spec.Family, Intent: "protect peaks", RequiredCoverage: []string{"threshold"}, Scope: processorintent.ScopeCurrentTrack, ControlMode: processorintent.ControlModeSemantic, Confidence: .9}
	decision, err := decodeSemanticDynamicDecision(`{"schema_version":"semantic_dynamic_control_decision.v1","controls":[{"axis":"threshold","role":"threshold","target":{"value_db":-14},"purpose":"reduce peak drive","confidence":"high"}]}`, intent, brief)
	if err != nil || len(decision.Controls) != 1 || decision.Controls[0].PathKey != "stage/limiter_main/operating_point/threshold/0" {
		t.Fatalf("unique server binding was not completed: decision=%+v err=%v", decision, err)
	}
	ambiguous := semanticDynamicBrief{Family: spec.Family, Controls: append(append([]semanticDynamicBriefRow(nil), brief.Controls...), semanticDynamicBriefRow{PathKey: "stage/limiter_alt/operating_point/threshold/0", Role: "threshold"})}
	if _, err := decodeSemanticDynamicDecision(`{"schema_version":"semantic_dynamic_control_decision.v1","controls":[{"axis":"threshold","role":"threshold","target":{"value_db":-14},"purpose":"reduce peak drive","confidence":"high"}]}`, intent, ambiguous); err == nil || !strings.Contains(err.Error(), "path_key is missing") {
		t.Fatalf("ambiguous server binding did not fail closed: %v", err)
	}
}

func TestSemanticDynamicFamilyMatrixHasPCAProofAndStructuralObservation(t *testing.T) {
	registry, err := processorregistry.Default()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		family string
		axis   string
		view   string
		role   string
	}{
		{processorintent.FamilyLimiter, "output_ceiling", "track.peak_structure", "ceiling"},
		{processorintent.FamilyGateExpander, "activation_threshold", "track.activity_structure", "threshold"},
		{processorintent.FamilyDeEsser, "detector_focus", "track.frequency_time_events", "focus_frequency"},
		{processorintent.FamilyTransientShaper, "envelope_emphasis", "track.transient_structure", "attack_amount"},
		{processorintent.FamilyMultibandDynamics, "band_dynamics", "track.band_dynamics", "threshold"},
	}
	for _, test := range cases {
		t.Run(test.family, func(t *testing.T) {
			spec, ok := semanticDynamicSpecForFamily(test.family)
			if !ok {
				t.Fatal("dynamic spec is missing")
			}
			definition, ok := registry.Resolve(test.family)
			if !ok {
				t.Fatal("registry definition is missing")
			}
			if !containsString(definition.ObservationViews, test.view) {
				t.Fatalf("observation view %q missing from %+v", test.view, definition.ObservationViews)
			}
			if !containsString(spec.CoverageAxes, test.axis) {
				t.Fatalf("semantic axis %q missing from %+v", test.axis, spec.CoverageAxes)
			}
			proof, err := registry.PCARequiredCoverage(test.family, []string{test.axis})
			if err != nil || len(proof) == 0 {
				t.Fatalf("axis %q did not resolve to PCA proof: %v %+v", test.axis, err, proof)
			}
			brief, bindings, err := semanticDynamicBriefFor(spec, dynamicTestSummary(test.family), []string{semanticAxisForRole(test.family, test.role)})
			if err != nil || len(brief.Controls) == 0 || len(bindings) == 0 {
				t.Fatalf("structural brief unavailable: err=%v brief=%+v bindings=%+v", err, brief, bindings)
			}
			for _, row := range brief.Controls {
				if row.Role == test.role && row.PathKey == "" {
					t.Fatal("structural control path is empty")
				}
			}
		})
	}
}

func TestSemanticDynamicTransientDetectorFocusUsesCanonicalPCAControllerRole(t *testing.T) {
	spec, ok := semanticDynamicSpecForFamily(processorintent.FamilyTransientShaper)
	if !ok {
		t.Fatal("transient-shaper spec is missing")
	}
	brief, bindings, err := semanticDynamicBriefFor(spec, dynamicTestSummary(spec.Family), []string{"detector_focus"})
	if err != nil {
		t.Fatalf("canonical transient detector role was rejected: %v", err)
	}
	if len(brief.Controls) != 1 || len(bindings) != 1 || brief.Controls[0].Role != "focus_frequency" {
		t.Fatalf("unexpected transient detector brief: brief=%+v bindings=%+v", brief, bindings)
	}
}

func semanticAxisForRole(family, role string) string {
	switch family {
	case processorintent.FamilyLimiter:
		if role == "ceiling" {
			return "ceiling"
		}
	case processorintent.FamilyGateExpander:
		if role == "threshold" {
			return "threshold"
		}
	case processorintent.FamilyDeEsser:
		if role == "focus_frequency" {
			return "frequency_focus"
		}
	case processorintent.FamilyTransientShaper:
		if role == "attack_amount" {
			return "attack"
		}
	case processorintent.FamilyMultibandDynamics:
		if role == "threshold" {
			return "threshold"
		}
	}
	return ""
}

func TestSemanticDynamicUnresolvedIntentIsAStableRejection(t *testing.T) {
	spec, ok := semanticDynamicSpecForFamily(processorintent.FamilyDeEsser)
	if !ok {
		t.Fatal("de-esser spec missing")
	}
	raw := `{"schema_version":"semantic_processor_intent.v1","status":"unresolved","control_mode":"unresolved","confidence":0.21,"rejection":{"code":"ambiguous_acoustic_goal","reason":"the request does not identify a defensible processor intent"}}`
	_, err := semanticDynamicDecodeIntent(raw, "make it better", spec, nil)
	if err == nil || !strings.Contains(err.Error(), "ambiguous_acoustic_goal") {
		t.Fatalf("unresolved intent lost structured rejection: %v", err)
	}
}

func TestSemanticDynamicConfirmationPayloadDoesNotExposeExecutionBinding(t *testing.T) {
	s := New(nil, nil, nil)
	requestContext := map[string]any{"conversation_id": "conv-1", "goal_id": "goal-1", "run_id": "run-1"}
	spec, _ := semanticDynamicSpecForFamily(processorintent.FamilyLimiter)
	intent := processorintent.Intent{SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved, Family: spec.Family, Intent: "protect peaks", RequiredCoverage: []string{"ceiling"}, Scope: processorintent.ScopeCurrentTrack, ControlMode: processorintent.ControlModeSemantic, Confidence: .9}
	brief, bindings, err := semanticDynamicBriefFor(spec, dynamicTestSummary(spec.Family), []string{"ceiling"})
	if err != nil {
		t.Fatal(err)
	}
	path := "stage/limiter_main/safety/ceiling/0"
	binding := bindings[path]
	ticket := semanticDynamicTicket{SchemaVersion: semanticDynamicSchema, TicketID: "dyn-test", Family: spec.Family, ProcessorType: spec.Processor, TrackID: "track-1", PluginID: "plugin-1", TopologyGeneration: "generation-test", Intent: intent, Controls: []semanticDynamicTicketRow{{Axis: "ceiling", PathKey: path, Role: binding.Role, ControlRef: binding.ControlRef, Target: map[string]any{"value_db": -1.0}, Purpose: "protect peaks", Confidence: "high"}}}
	decision := semanticDynamicDecision{SchemaVersion: semanticDynamicDecisionSchema, Controls: []semanticDynamicDecisionRow{{Axis: "ceiling", PathKey: path, Role: binding.Role, Target: map[string]any{"value_db": -1.0}, Purpose: "protect peaks", Confidence: "high"}}}
	resp := s.semanticDynamicWaitingResponse("conv-1", requestContext, ticket, brief, decision, plugingrabber.ParameterDigest{})
	encoded, _ := json.Marshal(resp.WorkflowData)
	text := string(encoded)
	if strings.Contains(text, "control_ref") || strings.Contains(text, "param_id") {
		t.Fatalf("public confirmation payload exposed execution binding: %s", text)
	}
	if firstStringFromMap(resp.WorkflowData, "ticket_id") != ticket.TicketID {
		t.Fatalf("public payload omitted ticket id: %+v", resp.WorkflowData)
	}
	if len(resp.InteractionRequests) != 1 {
		t.Fatal("confirmation interaction was not created")
	}
	interactionData, _ := json.Marshal(resp.InteractionRequests[0].Payload)
	if strings.Contains(string(interactionData), "control_ref") {
		t.Fatalf("interaction payload exposed execution binding: %s", interactionData)
	}
}

func TestSemanticDynamicPostActionVerificationIsModelOwned(t *testing.T) {
	server := New(nil, nil, nil)
	spec, ok := semanticDynamicSpecForFamily(processorintent.FamilyDeEsser)
	if !ok {
		t.Fatal("de-esser spec is missing")
	}
	intent := processorintent.Intent{
		SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved,
		Family: spec.Family, Intent: "reduce sibilance", RequiredCoverage: []string{"threshold"},
		Scope: processorintent.ScopeCurrentTrack, ControlMode: processorintent.ControlModeSemantic, Confidence: .9,
	}
	brief, bindings, err := semanticDynamicBriefFor(spec, dynamicTestSummary(spec.Family), []string{"threshold"})
	if err != nil || len(bindings) == 0 {
		t.Fatalf("brief=%+v bindings=%+v err=%v", brief, bindings, err)
	}
	path := "stage/deesser_main/operating_point/threshold/0"
	binding, ok := bindings[path]
	if !ok {
		for candidatePath, candidate := range bindings {
			if candidate.Role == "threshold" {
				path, binding, ok = candidatePath, candidate, true
				break
			}
		}
	}
	if !ok {
		t.Fatal("threshold binding is missing")
	}
	ticket := semanticDynamicTicket{
		SchemaVersion: semanticDynamicSchema, TicketID: "dyn-first", Family: spec.Family, ProcessorType: spec.Processor,
		TrackID: "track-1", PluginID: "deesser-1", TopologyGeneration: "generation-test", Intent: intent,
		Controls: []semanticDynamicTicketRow{{Axis: "threshold", PathKey: path, Role: binding.Role, ControlRef: binding.ControlRef,
			Target: map[string]any{"value_db": -18.0}, Purpose: "reduce sibilance", Confidence: "high"}},
	}
	decision := semanticDynamicDecision{SchemaVersion: semanticDynamicDecisionSchema, Controls: []semanticDynamicDecisionRow{{
		Axis: "threshold", PathKey: path, Role: binding.Role, Target: map[string]any{"value_db": -18.0}, Purpose: "reduce sibilance", Confidence: "high",
	}}}
	first := server.semanticDynamicWaitingResponse("conversation-1", nil, ticket, brief, decision, plugingrabber.ParameterDigest{})
	post := firstMapFromAny(first.WorkflowData["post_action_verification"])
	if first.Workflow != semanticDynamicWorkflow || !first.NeedsConfirmation ||
		firstStringFromMap(post, "view_selection") != "model_owned" ||
		boolValue(post["server_injected_view"]) || !boolValue(post["requires_fresh_observation"]) {
		t.Fatalf("first confirmation injected a review view: %+v", first.WorkflowData)
	}

	// A second same-family plan is a new confirmation boundary, not a reuse of
	// the prior ticket or confirmation interaction.
	ticket.TicketID = "dyn-second"
	second := server.semanticDynamicWaitingResponse("conversation-1", nil, ticket, brief, decision, plugingrabber.ParameterDigest{})
	if !second.NeedsConfirmation || second.PlanID == first.PlanID || len(second.InteractionRequests) != 1 ||
		second.InteractionRequests[0].ID == first.InteractionRequests[0].ID {
		t.Fatalf("same-family repeat reused confirmation state: first=%+v second=%+v", first, second)
	}
}

func TestSemanticDynamicControllerReceiptRequiresAtomicReadbackAndRestore(t *testing.T) {
	ticket := semanticDynamicTicket{TrackID: "track-1", PluginID: "plugin-1", TopologyGeneration: "generation-test", Controls: []semanticDynamicTicketRow{{}}}
	base := map[string]any{"status": "exact", "atomic": true, "track_id": "track-1", "plugin_id": "plugin-1", "topology_generation": "generation-test", "restore_ref": "restore-1", "controls": []map[string]any{{"status": "exact", "actual_readback": []any{map[string]any{"value": -1.0}}}}}
	if err := validateSemanticDynamicControllerResult(base, ticket); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"non_atomic":       func(result map[string]any) { result["atomic"] = false },
		"missing_restore":  func(result map[string]any) { delete(result, "restore_ref") },
		"missing_readback": func(result map[string]any) { result["controls"] = []map[string]any{{"status": "exact"}} },
	} {
		t.Run(name, func(t *testing.T) {
			result := cloneContext(base)
			mutate(result)
			if err := validateSemanticDynamicControllerResult(result, ticket); err == nil {
				t.Fatal("invalid controller receipt was accepted")
			}
		})
	}
}

func TestSemanticDynamicTargetMaterializerMatchesTypedFrequencyFields(t *testing.T) {
	for _, family := range []string{processorintent.FamilyGateExpander, processorintent.FamilyDeEsser, processorintent.FamilyTransientShaper} {
		t.Run(family, func(t *testing.T) {
			spec, _ := semanticDynamicSpecForFamily(family)
			got, err := materializeSemanticDynamicTarget(spec, semanticDynamicBinding{Role: "frequency_focus"}, map[string]any{"value_hz": 4200.0})
			if err != nil {
				t.Fatal(err)
			}
			if got["frequency_hz"] != 4200.0 || got["value_hz"] != nil {
				t.Fatalf("typed frequency target was not materialized: %+v", got)
			}
		})
	}
	spec, _ := semanticDynamicSpecForFamily(processorintent.FamilyMultibandDynamics)
	got, err := materializeSemanticDynamicTarget(spec, semanticDynamicBinding{Role: "crossover"}, map[string]any{"value_hz": 1200.0})
	if err != nil || got["value_hz"] != 1200.0 || got["frequency_hz"] != nil {
		t.Fatalf("multiband frequency target changed unexpectedly: got=%+v err=%v", got, err)
	}
}

func TestSemanticDynamicLimiterRejectsUnsupportedFrequencyTarget(t *testing.T) {
	spec, _ := semanticDynamicSpecForFamily(processorintent.FamilyLimiter)
	if _, err := materializeSemanticDynamicTarget(spec, semanticDynamicBinding{Role: "ceiling"}, map[string]any{"value_hz": 1000.0}); err == nil {
		t.Fatal("limiter accepted an unsupported frequency target")
	}
}

func TestSemanticDynamicControllerDispatchCoversAllFiveFamilies(t *testing.T) {
	tests := []struct {
		name       string
		family     string
		trackID    string
		pluginID   string
		makeServer func() (*Server, func() int, string, map[string]any)
	}{
		{
			name:     "limiter",
			family:   processorintent.FamilyLimiter,
			trackID:  "track-1",
			pluginID: "limit-1",
			makeServer: func() (*Server, func() int, string, map[string]any) {
				fake := newFakeLimiterKernel()
				server := New(nil, shadow.New(nil), nil)
				server.eqKernelOverride = fake
				_, summary, err := server.readLiveLimiterControlSurface(context.Background(), "track-1", "limit-1")
				if err != nil {
					panic(err)
				}
				return server, func() int { return len(fake.batchCalls) }, limiterTestControlRef(summary, "threshold"), map[string]any{"value_db": -15.0}
			},
		},
		{
			name:     "gate_expander",
			family:   processorintent.FamilyGateExpander,
			trackID:  "track-1",
			pluginID: "gate-1",
			makeServer: func() (*Server, func() int, string, map[string]any) {
				fake := newFakeGateExpanderKernel()
				server := New(nil, shadow.New(nil), nil)
				server.eqKernelOverride = fake
				_, summary, err := server.readLiveGateExpanderControlSurface(context.Background(), "track-1", "gate-1")
				if err != nil {
					panic(err)
				}
				return server, func() int { return len(fake.batchCalls) }, gateTestControlRef(summary, "threshold"), map[string]any{"value_db": -15.0}
			},
		},
		{
			name:     "de_esser",
			family:   processorintent.FamilyDeEsser,
			trackID:  "track-1",
			pluginID: "deesser-1",
			makeServer: func() (*Server, func() int, string, map[string]any) {
				fake := newFakeDeEsserKernel()
				server := New(nil, shadow.New(nil), nil)
				server.eqKernelOverride = fake
				_, summary, err := server.readLiveDeEsserControlSurface(context.Background(), "track-1", "deesser-1")
				if err != nil {
					panic(err)
				}
				return server, func() int { return len(fake.batchCalls) }, deEsserTestControlRef(summary, "threshold"), map[string]any{"value_db": -15.0}
			},
		},
		{
			name:     "transient_shaper",
			family:   processorintent.FamilyTransientShaper,
			trackID:  "track-1",
			pluginID: "transient-1",
			makeServer: func() (*Server, func() int, string, map[string]any) {
				fake := newFakeTransientShaperKernel()
				server := New(nil, shadow.New(nil), nil)
				server.eqKernelOverride = fake
				_, summary, err := server.readLiveTransientShaperControlSurface(context.Background(), "track-1", "transient-1")
				if err != nil {
					panic(err)
				}
				return server, func() int { return len(fake.batchCalls) }, transientTestControlRef(summary, "attack_amount"), map[string]any{"percent": 25.0}
			},
		},
		{
			name:     "multiband_dynamics",
			family:   processorintent.FamilyMultibandDynamics,
			trackID:  "track-mb",
			pluginID: "mb-1",
			makeServer: func() (*Server, func() int, string, map[string]any) {
				fake := newFakeMultibandKernel()
				server := New(nil, shadow.New(nil), nil)
				server.eqKernelOverride = fake
				_, summary, err := server.readLiveMultibandControlSurface(context.Background(), "track-mb", "mb-1")
				if err != nil {
					panic(err)
				}
				return server, func() int { return len(fake.batches) }, multibandTestRef(summary, "gain_action", "band_1", "gain"), map[string]any{"value_db": -6.0}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, batchCount, ref, target := test.makeServer()
			if ref == "" {
				t.Fatal("typed surface did not disclose a control_ref")
			}
			spec, ok := semanticDynamicSpecForFamily(test.family)
			if !ok {
				t.Fatal("dynamic family spec is missing")
			}
			control := cloneContext(target)
			control["control_ref"] = ref
			result, err := server.applySemanticDynamicController(context.Background(), spec, map[string]any{
				"track_id": test.trackID, "plugin_id": test.pluginID, "atomic": true,
				"controls": []map[string]any{control},
			}, nil)
			if err != nil {
				t.Fatalf("generic family dispatch failed: %v", err)
			}
			if firstStringFromMap(result, "status") != "exact" && firstStringFromMap(result, "status") != "quantized" {
				t.Fatalf("unexpected typed status: %+v", result)
			}
			if !boolValue(result["atomic"]) || firstStringFromMap(result, "restore_ref") == "" || len(mapRowsValue(result["controls"])) != 1 {
				t.Fatalf("typed receipt is incomplete: %+v", result)
			}
			if batchCount() != 1 {
				t.Fatalf("generic dispatch did not produce exactly one typed batch: %d", batchCount())
			}
		})
	}
}
