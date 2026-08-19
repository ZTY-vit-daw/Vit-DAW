package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"vit-daw-agent/internal/dynamiccontrol"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/processorintent"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

func TestC2ProjectObservationDigestBoundsModelContext(t *testing.T) {
	raw := map[string]any{"tracks": []any{map[string]any{"track_id": "bass", "name": "Bass", "clips": []any{map[string]any{"sample_buffer": strings.Repeat("x", c2ProjectDigestMaxBytes)}}, "time_dynamics": map[string]any{"peak_db": -3.0}}}}
	digest := c2ProjectObservationDigest(raw)
	encoded, err := json.Marshal(digest)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > c2ProjectDigestMaxBytes+2048 {
		t.Fatalf("digest bytes=%d exceeds bounded context projection", len(encoded))
	}
	projection := firstMapFromAny(digest["projection"])
	tracks := mapRowsValue(projection["tracks"])
	if len(tracks) != 1 || firstStringFromMap(tracks[0], "track_id") != "bass" || tracks[0]["clips"] != nil {
		t.Fatalf("digest did not retain track identity while omitting clip payload: %#v", projection)
	}
}

func TestC2VerificationViewsMatchTheExecutedFamily(t *testing.T) {
	cases := map[string][]any{
		processorintent.FamilyBroadbandCompressor: {"track.time_dynamics"},
		processorintent.FamilyLimiter:             {"track.peak_structure"},
		processorintent.FamilyGateExpander:        {"track.activity_structure", "track.time_dynamics"},
		processorintent.FamilyDeEsser:             {"track.frequency_time_events"},
		processorintent.FamilyTransientShaper:     {"track.transient_structure"},
		processorintent.FamilyMultibandDynamics:   {"track.band_dynamics"},
	}
	for family, want := range cases {
		if got := c2VerificationViews(family); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s views=%#v want=%#v", family, got, want)
		}
	}
	if got := c2VerificationViews("unsupported"); got != nil {
		t.Fatalf("unsupported family received C2 verification views: %#v", got)
	}
}

func TestTransientEnvelopeEmphasisIncludesPCACertifiedRangeRole(t *testing.T) {
	roles := semanticDynamicRoleAliases(processorintent.FamilyTransientShaper, "envelope_emphasis")
	for _, role := range []string{"attack_amount", "sustain_amount", "transient_range"} {
		if !roles[role] {
			t.Fatalf("envelope_emphasis omitted typed role %q: %#v", role, roles)
		}
	}
}

func TestC2ProjectObservationTrackScopeDoesNotDependOnPlugins(t *testing.T) {
	state := map[string]any{"tracks": []any{
		map[string]any{"track_id": "vocal", "name": "Lead Vocal"},
		map[string]any{"track_id": "bass", "name": "Bass"},
	}}
	if got := c2ProjectTrackIDs(state, ""); !reflect.DeepEqual(got, []string{"vocal", "bass"}) {
		t.Fatalf("C2 project tracks=%#v, want every project track regardless of plugin inventory", got)
	}
	if got := c2ProjectTrackIDs(state, "bass"); !reflect.DeepEqual(got, []string{"bass"}) {
		t.Fatalf("C2 explicit target scope=%#v", got)
	}
}

func TestC2ProjectBatchIgnoresUIPresentationSelection(t *testing.T) {
	requestContext := map[string]any{
		"selected_track_id":        "ui-track",
		"selected_plugin_track_id": "ui-plugin-track",
	}
	if got := c2ExplicitProjectTargetID(requestContext); got != "" {
		t.Fatalf("C2 project scope leaked UI selection %q", got)
	}
	requestContext["c2_target_track_id"] = "explicit-c2-track"
	if got := c2ExplicitProjectTargetID(requestContext); got != "explicit-c2-track" {
		t.Fatalf("C2 explicit target = %q", got)
	}
}

func TestC2ObservationReadyTimeoutIsResumable(t *testing.T) {
	bundle := map[string]any{
		"status": "rejected",
		"audit_receipt": map[string]any{
			"rejection_reasons": []any{"observation_ready_gate_timeout"},
		},
	}
	if got := c2ObservationPendingReason(harness.InvokeResponse{Status: "ok"}, bundle); got != "observation_ready_gate_timeout" {
		t.Fatalf("C2 readiness timeout reason = %q", got)
	}
	response := c2ObservationPendingResponse("conversation", agentruntime.Goal{GoalID: "goal", RunID: "run"}, "session", &c2ObservationPendingError{Reason: "track-1: observation_ready_gate_timeout"})
	if response.GoalStatus != string(agentruntime.StatusWaitingContinue) || response.Error != "" || response.StopReason != "c2_observation_pending" {
		t.Fatalf("C2 readiness timeout was not resumable: %#v", response)
	}
	if !boolValue(response.WorkflowData["resumable"]) || boolValue(response.WorkflowData["mutation_performed"]) {
		t.Fatalf("C2 readiness response lost its no-mutation resume contract: %#v", response.WorkflowData)
	}
}

func TestC2PermanentObservationRejectionIsNotMisclassifiedAsPending(t *testing.T) {
	bundle := map[string]any{
		"status": "rejected",
		"audit_receipt": map[string]any{
			"rejection_reasons": []any{"target_not_found"},
		},
	}
	if got := c2ObservationPendingReason(harness.InvokeResponse{Status: "ok"}, bundle); got != "" {
		t.Fatalf("C2 permanent rejection was classified as pending: %q", got)
	}
}

func TestC2ProjectTrackRowsKeepNamesForCandidatePlanning(t *testing.T) {
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "bass", "name": "Sattelites Bass"}}}
	rows := c2ProjectTrackRows(state, []string{"bass"})
	if len(rows) != 1 || firstStringFromMap(rows[0], "track_id") != "bass" || firstStringFromMap(rows[0], "track_name") != "Sattelites Bass" {
		t.Fatalf("C2 candidate track rows=%#v", rows)
	}
}

func TestC2ProjectDiscoverySessionsAreTrackIsolated(t *testing.T) {
	first := c2DiscoveryMixSessionID("c2 product smoke", "vocal")
	second := c2DiscoveryMixSessionID("c2 product smoke", "bass")
	if first == second || first == "" || second == "" {
		t.Fatalf("C2 parallel track discovery reused a Mixboard session: %q, %q", first, second)
	}
}

func TestC2ProjectCandidateSelectionUsesObservationNotPluginInventory(t *testing.T) {
	project := map[string]any{"global_summary": map[string]any{"feature_snapshot": map[string]any{"band_energy_summaries": []any{
		map[string]any{"source_identity": map[string]any{"track_id": "quiet"}, "peak_dbfs": -20.0},
		map[string]any{"source_identity": map[string]any{"track_id": "loud"}, "peak_dbfs": -2.0, "crest_db": 12.0, "band_dynamics": map[string]any{"status": "ready"}},
		map[string]any{"source_identity": map[string]any{"track_id": "medium"}, "peak_dbfs": -8.0},
	}}}}
	got := c2ProjectDynamicCandidateTrackIDs(project, []string{"quiet", "loud", "medium"})
	if len(got) != 3 || got[0] != "loud" {
		t.Fatalf("C2 dynamic candidates = %#v, want observation-ranked loud track first", got)
	}
}

func TestC2CoverageVocabularyRejectsInventedAxes(t *testing.T) {
	vocabulary, err := c2CoverageVocabulary(processorintent.FamilyTransientShaper)
	if err != nil {
		t.Fatal(err)
	}
	options := map[string][][]string{processorintent.FamilyTransientShaper: {{"attack", "sustain"}}}
	target := dynamiccontrol.TargetHypothesis{TrackID: "drums", Intent: processorintent.Intent{Family: processorintent.FamilyTransientShaper, RequiredCoverage: []string{"attack"}}}
	if err := c2ValidateRequiredCoverage(vocabulary, options, target); err != nil {
		t.Fatal(err)
	}
	target.Intent.RequiredCoverage = []string{"attack_transient_control"}
	if err := c2ValidateRequiredCoverage(vocabulary, options, target); err == nil {
		t.Fatal("C2 accepted an unregistered transient-shaper coverage axis")
	}
	target.Intent.RequiredCoverage = []string{"attack", "sustain", "mix", "output", "attack"}
	if err := c2ValidateRequiredCoverage(vocabulary, options, target); err == nil {
		t.Fatal("C2 accepted more than four semantic coverage axes")
	}
}

func TestC2CoverageRejectsCrossBinaryAxisCombination(t *testing.T) {
	vocabulary := map[string][]string{"broadband_compressor": {"activation_intensity", "recovery_motion"}}
	options := map[string][][]string{"broadband_compressor": {{"activation_intensity"}, {"recovery_motion"}}}
	target := dynamiccontrol.TargetHypothesis{TrackID: "vocal", Intent: processorintent.Intent{Family: "broadband_compressor", RequiredCoverage: []string{"activation_intensity", "recovery_motion"}}}
	if err := c2ValidateRequiredCoverage(vocabulary, options, target); err == nil || !strings.Contains(err.Error(), "no single PCA-certified") {
		t.Fatalf("C2 accepted an axis combination split across PCA binaries: %v", err)
	}
}

func TestC2FamiliesWithExecutableCoverageExcludesEmptyFamilies(t *testing.T) {
	got := c2FamiliesWithExecutableCoverage(map[string][]string{"limiter": nil, "transient_shaper": {"envelope_emphasis"}, "broadband_compressor": {"activation_intensity"}})
	want := []string{"broadband_compressor", "transient_shaper"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("C2 executable families=%#v want=%#v", got, want)
	}
}

func TestC2ProjectTreatmentRetriesOnlyTransientServiceErrors(t *testing.T) {
	for _, test := range []struct {
		err  error
		want bool
	}{
		{errors.New("Internal server error"), true},
		{errors.New("LLM HTTP error 503"), true},
		{errors.New("context deadline exceeded while awaiting headers"), true},
		{errors.New("C2 target cited an unexpected observation"), false},
		{errors.New("invalid project treatment JSON"), false},
	} {
		if got := c2TransientProjectTreatmentError(test.err); got != test.want {
			t.Fatalf("c2 transient classification for %q = %v, want %v", test.err, got, test.want)
		}
	}
}

func TestC2LeafSurfaceRetryIsLimitedToTopologyReadiness(t *testing.T) {
	if !c2LeafSurfaceMayBeUnready(ChatResponse{Error: "selected_axes_unreachable"}) {
		t.Fatal("C2 did not retry a transient post-load control-surface boundary")
	}
	if !c2LeafSurfaceMayBeUnready(ChatResponse{StopReason: "required_coverage_unreachable:activation_intensity"}) {
		t.Fatal("C2 did not retry a transient post-load coverage binding boundary")
	}
	if c2LeafSurfaceMayBeUnready(ChatResponse{Error: "pca_admission_rejected"}) {
		t.Fatal("C2 retried a PCA admission failure")
	}
	if c2LeafSurfaceMayBeUnready(ChatResponse{Error: "control_planning_failed"}) {
		t.Fatal("C2 retried a model control-plan failure")
	}
}

func TestC2LeafPlanningTransportRetryRequiresAnExplicitTransientError(t *testing.T) {
	if !c2LeafPlanningTransportMayRetry(ChatResponse{Error: "context deadline exceeded while awaiting headers", StopReason: "control_planning_failed"}) {
		t.Fatal("C2 did not retry a leaf request that failed before returning a model decision")
	}
	if c2LeafPlanningTransportMayRetry(ChatResponse{StopReason: "control_planning_failed"}) {
		t.Fatal("C2 retried an opaque model planning failure")
	}
	if c2LeafPlanningTransportMayRetry(ChatResponse{Error: "invalid semantic dynamic control decision", StopReason: "control_planning_failed"}) {
		t.Fatal("C2 retried a returned invalid model decision")
	}
}

func TestC2LeafPlanningFailureReasonPreservesMaterializationDetail(t *testing.T) {
	response := ChatResponse{StopReason: "execution_materialization_rejected", WorkflowData: map[string]any{
		"execution": map[string]any{"message": "proposal drive cannot be bound: ambiguous input drive"},
	}}
	got := c2LeafPlanningFailureReason(response)
	if !strings.Contains(got, "execution_materialization_rejected") || !strings.Contains(got, "ambiguous input drive") {
		t.Fatalf("materialization detail was lost: %q", got)
	}
}

func TestC2FrozenCompressorIntentPreservesCertifiedCoverage(t *testing.T) {
	server := &Server{}
	intent, err := server.c2FrozenCompressorIntent(map[string]any{"c2_semantic_processor_intent": map[string]any{
		"schema_version": "semantic_processor_intent.v1", "status": "resolved", "family": "broadband_compressor",
		"intent": "control peaks", "required_coverage": []any{"activation_intensity", "transfer_severity"},
		"scope": "current_track", "control_mode": "semantic_loop", "evidence_refs": []any{"obs-1"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if intent == nil || !reflect.DeepEqual(intent.SelectedAxes, []string{"activation_intensity", "transfer_severity"}) {
		t.Fatalf("C2 compressor leaf lost frozen PCA coverage: %#v", intent)
	}
}

func TestC2PCAAdmissionBindsStableExactCandidate(t *testing.T) {
	candidates := []pluginRecommendationCandidate{
		{Identifier: "z-vst", PluginPath: "C:/Plugins/Z.vst3", Name: "Z"},
		{Identifier: "a-vst", PluginPath: "C:/Plugins/A.vst3", Name: "A"},
	}
	selected, ok := c2StablePCAAdmittedCandidate(candidates)
	if !ok || selected.Identifier != "a-vst" || selected.PluginPath != "C:/Plugins/A.vst3" {
		t.Fatalf("C2 PCA binding = %#v, ok=%v", selected, ok)
	}
	if candidates[0].Identifier != "z-vst" {
		t.Fatalf("C2 PCA binding mutated admitted candidate order: %#v", candidates)
	}
	if _, ok := c2StablePCAAdmittedCandidate(nil); ok {
		t.Fatal("C2 accepted an empty PCA-admitted candidate set")
	}
}

func TestC2RawFrozenHypothesisCannotSelectC2Family(t *testing.T) {
	hypothesis := dynamiccontrol.TargetHypothesis{SchemaVersion: dynamiccontrol.HypothesisSchema, Status: dynamiccontrol.HypothesisTargeted, TrackID: "vocal", ListeningGoal: "steady lead", Rationale: "dynamic evidence", EvidenceRefs: []string{"obs-vocal"}, Intent: processorintent.Intent{SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved, Family: processorintent.FamilyDeEsser, Intent: "reduce sibilant spikes", RequiredCoverage: []string{"threshold_sensitivity"}, Scope: processorintent.ScopeCurrentTrack, ControlMode: processorintent.ControlModeSemantic, Confidence: .7, EvidenceRefs: []string{"obs-vocal"}}}
	if got := c2FamilyFromContext(map[string]any{"c2_frozen_hypothesis": mapFromJSONStruct(hypothesis)}); got != "" {
		t.Fatalf("raw frozen hypothesis selected C2 family %q", got)
	}
	if got := c2FamilyFromContext(map[string]any{"free_state_semantic_processor_intent": mapFromJSONStruct(hypothesis.Intent)}); got != "" {
		t.Fatalf("free-state intent selected C2 family %q", got)
	}
}

func TestC2HypothesisEvidenceMustComeFromTargetCCBReceipt(t *testing.T) {
	hypothesis := dynamiccontrol.TargetHypothesis{Status: dynamiccontrol.HypothesisTargeted, TrackID: "vocal", EvidenceRefs: []string{"obs-vocal"}, Intent: processorintent.Intent{EvidenceRefs: []string{"obs-vocal"}}}
	observations := []map[string]any{{"track_id": "vocal", "bundle": map[string]any{"observation_id": "obs-vocal", "audit_receipt": map[string]any{"receipt_id": "ccbr-vocal"}}}}
	if err := c2ValidateHypothesisEvidence(hypothesis, observations); err != nil {
		t.Fatal(err)
	}
	hypothesis.EvidenceRefs = []string{"invented-observation"}
	if err := c2ValidateHypothesisEvidence(hypothesis, observations); err == nil {
		t.Fatal("C2 accepted an evidence reference that CCB did not return")
	}
}

func TestC2ParameterBatchInteractionRoutesBeforeGenericPendingPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	interaction := AgentInteractionRequest{
		ID:             "interaction_c2_parameter_batch",
		Kind:           "confirmation",
		Type:           c2DynamicParameterBatchWorkflow,
		Source:         c2DynamicParameterBatchWorkflow,
		Workflow:       c2DynamicParameterBatchWorkflow,
		PlanID:         "c2_batch_not_a_legacy_pending_plan",
		ConversationID: "c2_test",
		GoalID:         "goal_c2_test",
		RunID:          "run_c2_test",
		Data:           map[string]any{"session_id": "c2_session"},
	}
	server.storePendingInteraction(interaction, interaction.Data)
	body, err := json.Marshal(InteractionRespondRequest{InteractionID: interaction.ID, ActionID: "cancel", Decision: "cancel"})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.handleInteractionRespond(recorder, httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response ChatResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}
	if response.Workflow != c2DynamicParameterBatchWorkflow || response.GoalStatus != string(agentruntime.StatusCancelled) {
		t.Fatalf("C2 parameter confirmation was routed as a generic plan: %+v", response)
	}
}

func TestC2MaterializedLeafTicketNormalizesPersistedValue(t *testing.T) {
	pending := PendingInteraction{Data: map[string]any{"ticket": struct {
		SchemaVersion string `json:"schema_version"`
		TicketID      string `json:"ticket_id"`
	}{SchemaVersion: "semantic_effect.compressor_execution_ticket.v1", TicketID: "ticket_1"}}}
	ticket := c2MaterializedLeafTicket(pending)
	if firstStringFromMap(ticket, "schema_version") == "" || firstStringFromMap(ticket, "ticket_id") != "ticket_1" {
		t.Fatalf("C2 did not normalize persisted leaf ticket: %#v", ticket)
	}
}

func TestC2PCAAdmissionReceiptRequiresExactCandidateIdentity(t *testing.T) {
	_, err := c2PCAAdmissionReceiptForLoad("broadband_compressor", nil, map[string]any{
		"name": "Compressor", "format": "VST3", "identifier": "plugin-id", "plugin_path": "C:/missing/plugin.vst3",
	}, map[string]any{"subject_key": "subject", "binary_fingerprint": "fingerprint", "attestation_id": "attestation"})
	if err == nil || !strings.Contains(err.Error(), "fingerprint unavailable") {
		t.Fatalf("C2 accepted a non-revalidated PCA receipt: %v", err)
	}
}

func TestC2ResolvedTargetPersistsLoadAdmissionReceipt(t *testing.T) {
	target := c2ResolvedTarget{PluginID: "rack-node-1", PCAAdmissionReceipt: map[string]any{
		"processor_family": "limiter", "subject_key": "subject", "binary_fingerprint": "fingerprint",
		"attestation_id": "attestation",
	}}
	encoded, err := json.Marshal(target)
	if err != nil {
		t.Fatal(err)
	}
	var restored c2ResolvedTarget
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if got := firstStringFromMap(restored.PCAAdmissionReceipt, "attestation_id"); got != "attestation" {
		t.Fatalf("C2 lost the server-issued PCA receipt across persistence: %#v", restored)
	}
}

func TestC2BatchLeafReceiptsBindEveryConfirmedLeaf(t *testing.T) {
	leaves := []c2BatchLeaf{{TrackID: "kick", PluginID: "compressor-1"}, {TrackID: "bass", PluginID: "compressor-2"}}
	receipts := []map[string]any{
		{"status": "executed", "track_id": "kick", "plugin_id": "compressor-1", "controller_result": map[string]any{"status": "exact", "restore_ref": "restore-kick"}, "parameter_audit": map[string]any{"status": "pass"}},
		{"status": "executed", "track_id": "bass", "plugin_id": "compressor-2", "controller_result": map[string]any{"status": "exact", "restore_ref": "restore-bass"}},
	}
	if err := validateC2BatchLeafReceipts(leaves, receipts); err != nil {
		t.Fatalf("valid multi-leaf receipts rejected: %v", err)
	}
	receipts[1]["track_id"] = "kick"
	if err := validateC2BatchLeafReceipts(leaves, receipts); err == nil {
		t.Fatal("cross-target receipt was accepted")
	}
}

type fakeC2BatchRollbackKernel struct {
	targets map[string]*fakeTransientShaperKernel
	calls   []string
}

func (fake *fakeC2BatchRollbackKernel) target(trackID, pluginID string) (*fakeTransientShaperKernel, string, error) {
	key := trackID + "/" + pluginID
	target := fake.targets[key]
	if target == nil {
		return nil, key, fmt.Errorf("unknown fake C2 target %s", key)
	}
	return target, key, nil
}

func (fake *fakeC2BatchRollbackKernel) SendCommand(ctx context.Context, command map[string]any) (map[string]any, string, error) {
	trackID, pluginID := firstStringFromMap(command, "track_id"), firstStringFromMap(command, "plugin_id")
	target, _, err := fake.target(trackID, pluginID)
	if err != nil {
		return nil, "", err
	}
	reply, _, err := target.SendCommand(ctx, command)
	if err != nil {
		return nil, "", err
	}
	reply["track_id"], reply["plugin_id"] = trackID, pluginID
	encoded, _ := json.Marshal(reply)
	return reply, string(encoded), nil
}

func (fake *fakeC2BatchRollbackKernel) SendVSPCommand(ctx context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	trackID, pluginID := firstStringFromMap(args, "track_id"), firstStringFromMap(args, "plugin_id")
	target, key, err := fake.target(trackID, pluginID)
	if err != nil {
		return nil, err
	}
	fake.calls = append(fake.calls, key)
	return target.SendVSPCommand(ctx, command, args)
}

func TestC2BatchRollbackRestoresMultipleLeavesInReverseOrder(t *testing.T) {
	fake := &fakeC2BatchRollbackKernel{targets: map[string]*fakeTransientShaperKernel{
		"track-a/plugin-a": newFakeTransientShaperKernel(),
		"track-b/plugin-b": newFakeTransientShaperKernel(),
	}}
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake

	leaves := []c2BatchLeaf{
		{Family: processorintent.FamilyTransientShaper, TrackID: "track-a", PluginID: "plugin-a"},
		{Family: processorintent.FamilyTransientShaper, TrackID: "track-b", PluginID: "plugin-b"},
	}
	receipts := make([]map[string]any, 0, len(leaves))
	for index, leaf := range leaves {
		_, summary, err := server.readLiveTransientShaperControlSurface(context.Background(), leaf.TrackID, leaf.PluginID)
		if err != nil {
			t.Fatal(err)
		}
		ref := transientTestControlRef(summary, "attack_amount")
		result, err := server.applyPluginGrabberTransientShaperControls(context.Background(), map[string]any{
			"track_id": leaf.TrackID, "plugin_id": leaf.PluginID, "atomic": true,
			"controls": []map[string]any{{"control_ref": ref, "percent": float64(25 + index*25)}},
		}, nil)
		if err != nil || firstStringFromMap(result, "status") != "exact" {
			t.Fatalf("prepare C2 leaf %d: result=%#v err=%v", index, result, err)
		}
		receipts = append(receipts, map[string]any{"controller_result": map[string]any{"restore_ref": firstStringFromMap(result, "restore_ref")}})
	}

	fake.calls = nil
	rollback, err := server.c2RollbackParameterLeaves(context.Background(), leaves, receipts)
	if err != nil {
		t.Fatalf("C2 multi-leaf rollback: %v (%#v)", err, rollback)
	}
	if !reflect.DeepEqual(fake.calls, []string{"track-b/plugin-b", "track-a/plugin-a"}) {
		t.Fatalf("C2 rollback order=%#v", fake.calls)
	}
	for key, target := range fake.targets {
		if target.params["attack"] != .5 {
			t.Fatalf("C2 rollback left %s attack=%v", key, target.params["attack"])
		}
	}
}
