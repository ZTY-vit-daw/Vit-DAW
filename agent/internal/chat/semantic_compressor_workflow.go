package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/semanticeffect"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

var compressorExactParameterRequest = regexp.MustCompile(`(?i)(threshold|ratio|attack|release|knee|makeup|input|output|阈值|压缩比|启动|释放|拐点|补偿增益).{0,20}[-+]?\d+(?:\.\d+)?`)

// routeOrdinaryAgentSemanticCompressorPlanning owns the selected-compressor
// fast path. It intercepts only when the selected live instance is actually a
// qualified broadband compressor; otherwise open method arbitration continues.
func (s *Server) routeOrdinaryAgentSemanticCompressorPlanning(ctx context.Context, conversationID, userText string,
	requestContext map[string]any, cfg config.EngineConfig) (ChatResponse, bool) {
	if contextBool(requestContext, "semantic_entry_unavailable") {
		return ChatResponse{}, false
	}
	if !ordinaryAgentSemanticCompressorPlanningRequest(userText, requestContext) {
		return ChatResponse{}, false
	}
	trackID := firstStringFromMap(requestContext, "selected_plugin_track_id", "selected_track_id")
	pluginID := firstStringFromMap(requestContext, "selected_plugin_id")
	digest, liveSummary, err := s.readLiveCompressorControlSurface(ctx, trackID, pluginID)
	if err != nil {
		if semanticCompressorTopologyBoundary(err) {
			return ChatResponse{}, false
		}
		return semanticCompressorPlanningFailure(conversationID, requestContext, "processor_identity_unavailable", err), true
	}
	return s.planBoundSemanticCompressorFromLive(ctx, conversationID, userText, requestContext, digest, liveSummary, cfg), true
}

func semanticCompressorTopologyBoundary(err error) bool {
	failure, ok := err.(*compressorControlFailure)
	return ok && (failure.Code == "not_compressor" || strings.HasPrefix(failure.Code, "unsupported_"))
}

// planBoundSemanticCompressor is the shared processor-specific planning entry.
// Callers must bind an exact live instance before entering it. The ordinary
// selected-plugin route owns its lexical gate; treatment-strategy handoffs do
// not repeat that gate after the LLM has already selected compression.
func (s *Server) planBoundSemanticCompressor(ctx context.Context, conversationID, userText string,
	requestContext map[string]any, cfg config.EngineConfig) ChatResponse {
	trackID := firstStringFromMap(requestContext, "selected_plugin_track_id", "selected_track_id")
	pluginID := firstStringFromMap(requestContext, "selected_plugin_id")
	digest, liveSummary, err := s.readLiveCompressorControlSurface(ctx, trackID, pluginID)
	if err != nil {
		return semanticCompressorPlanningFailure(conversationID, requestContext, "processor_identity_unavailable", err)
	}
	return s.planBoundSemanticCompressorFromLive(ctx, conversationID, userText, requestContext, digest, liveSummary, cfg)
}

func (s *Server) planBoundSemanticCompressorFromLive(ctx context.Context, conversationID, userText string,
	requestContext map[string]any, digest plugingrabber.ParameterDigest, liveSummary map[string]any, cfg config.EngineConfig) ChatResponse {
	trackID := firstStringFromMap(requestContext, "selected_plugin_track_id", "selected_track_id")
	pluginID := firstStringFromMap(requestContext, "selected_plugin_id")
	trackName := firstStringFromMap(requestContext, "selected_track_name")
	pluginName := firstStringFromMap(requestContext, "selected_plugin_name")
	if digest.PluginName == "" {
		digest.PluginName = pluginName
	}
	card, boundary := plugingrabber.BuildAudioProcessorIdentityCard(digest)
	if card == nil {
		return semanticCompressorPlanningFailure(conversationID, requestContext, firstNonEmpty(boundary, "processor_identity_unavailable"), fmt.Errorf("%s", boundary))
	}
	intent, err := s.c2FrozenCompressorIntent(requestContext)
	if err != nil {
		return semanticCompressorPlanningFailure(conversationID, requestContext, "c2_intent_rejected", err)
	}
	if intent == nil {
		intent, err = s.planOrdinaryAgentCompressorIntent(ctx, conversationID, userText, *card, cfg)
		if err != nil {
			return semanticCompressorPlanningFailure(conversationID, requestContext, "intent_planning_failed", err)
		}
	}
	registry, registryErr := processorregistry.Default()
	if registryErr != nil {
		return semanticCompressorPlanningFailure(conversationID, requestContext, "pca_registry_unavailable", registryErr)
	}
	pcaCoverage, pcaErr := registry.PCARequiredCoverage(processorintent.FamilyBroadbandCompressor, intent.SelectedAxes)
	if pcaErr != nil {
		return semanticCompressorPlanningFailure(conversationID, requestContext, "pca_coverage_unproven", pcaErr)
	}
	if _, pcaErr = s.semanticLoadedInstancePCAAdmissionForContext(ctx, requestContext, trackID, pluginID, semanticTreatmentPCAInput{
		Family: processorintent.FamilyBroadbandCompressor, RequiredCoverage: pcaCoverage,
	}); pcaErr != nil {
		return semanticCompressorPlanningFailure(conversationID, requestContext, "pca_admission_rejected", pcaErr)
	}
	comContext := s.observeSemanticCompressorContext(ctx, conversationID, trackID, pluginID, requestContext, *card, *intent)
	brief, briefBoundary := plugingrabber.BuildCompressorControlBrief(digest, intent.SelectedAxes)
	if brief == nil {
		// A PCA-covered semantic axis must have a corresponding live Typed
		// Executor role. Preserve both sides of that contract when it does not:
		// C2 can then distinguish a stale certification from a topology adapter
		// regression instead of reporting an opaque unreachable-axis failure.
		diagnostic := fmt.Sprintf("%s; selected_axes=%s; live_signature_roles=%s; topology=%s; parameter_count=%d",
			firstNonEmpty(briefBoundary, "selected_axes_unreachable"), strings.Join(intent.SelectedAxes, ","),
			strings.Join(card.SignatureControls, ","), card.TopologyEvidence.Classification, digest.ParameterCount)
		return semanticCompressorPlanningFailure(conversationID, requestContext, firstNonEmpty(briefBoundary, "selected_axes_unreachable"), fmt.Errorf("%s", diagnostic))
	}
	plan, err := s.planOrdinaryAgentCompressorControls(ctx, conversationID, userText, trackID, pluginID, trackName, pluginName,
		*card, *intent, comContext, *brief, nil, cfg)
	if err != nil {
		return semanticCompressorPlanningFailure(conversationID, requestContext, "control_planning_failed", err)
	}
	rejection := validateCompressorPlanAgainstBrief(*plan, *brief)
	if rejection != nil {
		if err := rejection.Validate(); err != nil {
			return semanticCompressorPlanningFailure(conversationID, requestContext, "invalid_materialization_rejection", err)
		}
		plan, err = s.planOrdinaryAgentCompressorControls(ctx, conversationID, userText, trackID, pluginID, trackName, pluginName,
			*card, *intent, comContext, *brief, rejection, cfg)
		if err != nil {
			return semanticCompressorPlanningFailure(conversationID, requestContext, "constrained_revision_failed", err)
		}
		if second := validateCompressorPlanAgainstBrief(*plan, *brief); second != nil {
			return ChatResponse{ConversationID: conversationID, Reply: "压缩器语义方案在一次受约束修订后仍无法由当前控制表面实现；已停止在只读规划阶段，没有修改任何参数。",
				Workflow: "semantic_compressor_planning", WorkflowData: semanticCompressorWorkflowData(requestContext, map[string]any{"schema_version": "semantic_compressor.workflow.v1", "status": "rejected", "planning_only": true,
					"mutation_performed": false, "processor_identity_card": card, "intent_plan": intent, "com_observation": comContext, "control_brief": brief, "first_rejection": rejection, "final_rejection": second}),
				GoalStatus: "completed", StopReason: "compressor_plan_unreachable_after_revision"}
		}
	}
	if boolValue(requestContext["semantic_compressor_planning_only"]) {
		return semanticCompressorPlanningResponse(conversationID, requestContext, *card, *intent, comContext, *brief, *plan, rejection)
	}
	goalID, runID := goalIDsFromContext(requestContext)
	ticket, preview, err := materializeSemanticCompressorPlan(*plan, digest, liveSummary, comContext, conversationID, goalID, runID,
		firstNonEmpty(digest.PluginName, pluginName), requestContext)
	if err != nil {
		return semanticCompressorExecutionMaterializationFailure(conversationID, requestContext, *card, *intent,
			comContext, *brief, *plan, err)
	}
	return s.semanticCompressorWaitingResponse(conversationID, requestContext, *card, *intent, comContext, *brief, *plan,
		ticket, preview)
}

// c2FrozenCompressorIntent translates the already evidence-bound C2 target
// into the compressor leaf's representation. It fixes scope and PCA coverage
// while leaving only concrete controller values to the semantic leaf planner.
// A nil result means this is not a C2-owned request and preserves ordinary
// open-agent behavior.
func (s *Server) c2FrozenCompressorIntent(requestContext map[string]any) (*semanticeffect.CompressorIntentPlan, error) {
	raw := firstMapFromAny(requestContext["c2_semantic_processor_intent"])
	if len(raw) == 0 {
		return nil, nil
	}
	encoded, _ := json.Marshal(raw)
	frozen, err := processorintent.Decode(string(encoded))
	if err != nil {
		return nil, err
	}
	if frozen.Status != processorintent.StatusResolved || frozen.Family != processorintent.FamilyBroadbandCompressor {
		return nil, fmt.Errorf("C2 compressor intent is not a resolved broadband compressor intent")
	}
	dimensions := []string{}
	for _, axis := range frozen.RequiredCoverage {
		switch axis {
		case "activation_intensity", "transfer_severity":
			dimensions = appendUniqueStrings(dimensions, "gain_action")
		case "transient_timing":
			dimensions = appendUniqueStrings(dimensions, "transient_response")
		case "recovery_motion":
			dimensions = appendUniqueStrings(dimensions, "recovery_motion")
		case "detector_focus":
			dimensions = appendUniqueStrings(dimensions, "trigger_relation")
		case "output_normalization":
			dimensions = appendUniqueStrings(dimensions, "level_effect")
		case "parallel_balance":
			dimensions = appendUniqueStrings(dimensions, "gain_action")
		case "character":
			dimensions = appendUniqueStrings(dimensions, "source_dynamics")
		default:
			return nil, fmt.Errorf("C2 compressor coverage contains unsupported axis %q", axis)
		}
	}
	intent := &semanticeffect.CompressorIntentPlan{SchemaVersion: semanticeffect.CompressorIntentPlanSchema, UserGoal: frozen.Intent, SelectedAxes: append([]string(nil), frozen.RequiredCoverage...), EvidenceRequest: semanticeffect.CompressorEvidenceRequest{PreferredMode: "paired_io", FallbackMode: "source_only", Dimensions: dimensions, Reason: "C2 fixed project observation selected these evidence-backed compressor axes."}, Reason: "Frozen from the C2 full-project target decision.", NeedsControlBrief: true}
	if err := intent.Validate(); err != nil {
		return nil, fmt.Errorf("translated C2 compressor intent: %w", err)
	}
	return intent, nil
}

func ordinaryAgentSemanticCompressorPlanningRequest(userText string, requestContext map[string]any) bool {
	if !contextHasAnyValue(requestContext, "selected_plugin_id") || !contextHasAnyValue(requestContext, "selected_plugin_track_id", "selected_track_id") {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || compressorExactParameterRequest.MatchString(text) {
		return false
	}
	if agentLoopTextHasAny(text, "limiter", "限制器", "multiband", "多段压缩", "多频段压缩") {
		return false
	}
	compressorTopic := agentLoopTextHasAny(text, "compress", "compression", "compressor", "压缩", "动态", "punch", "transient", "pumping", "泵感", "瞬态", "起音", "恢复", "更稳", "收紧")
	semanticAction := agentLoopTextHasAny(text, "more", "less", "preserve", "reduce", "increase", "tight", "smooth", "gentle", "aggressive", "punch", "control", "match", "parallel",
		"多一点", "少一点", "再压", "保留", "削弱", "减少", "增加", "收紧", "稳定", "平滑", "温和", "激进", "有力", "控制", "匹配", "并行", "自然")
	discussionOnly := agentLoopTextHasAny(text, "为什么", "是什么", "怎么理解", "解释一下", "why", "what is", "explain") && !semanticAction
	return compressorTopic && semanticAction && !discussionOnly
}

func (s *Server) observeSemanticCompressorContext(ctx context.Context, conversationID, trackID, pluginID string,
	requestContext map[string]any, card semanticeffect.AudioProcessorIdentityCard, intent semanticeffect.CompressorIntentPlan) map[string]any {
	mode := intent.EvidenceRequest.PreferredMode
	if mode == "not_needed" {
		return map[string]any{"status": "not_observed", "mode": "not_needed", "reason": intent.EvidenceRequest.Reason}
	}
	type observation struct {
		projection map[string]any
		result     map[string]any
	}
	baseContext := semanticCompressorObservationRequestContext(requestContext)
	call := func(requestedMode string, observationContext map[string]any) (observation, error) {
		args := map[string]any{"scope": "selected_track", "project_context": true, "observation_only": true,
			"track_id": trackID, "plugin_id": pluginID, "com_mode": requestedMode,
			"mix_session_id": fmt.Sprintf("compressor_planning_%s_%d", sanitizeCanaryID(conversationID), time.Now().UnixNano()),
			"goal_text":      intent.UserGoal,
			"target_ref":     map[string]any{"kind": "plugin", "id": pluginID, "track_id": trackID, "plugin_id": pluginID},
		}
		for _, key := range []string{"selected_clip_id", "clip_id", "feature_snapshot", "start_sample", "end_sample",
			"selected_clip_time_range", "selected_clip_range", "selected_clip_ranges", "time_selection",
			"listen_time_start_seconds", "listen_time_end_seconds", "current_playhead_seconds", "playhead_seconds", "transport_position_seconds"} {
			if value, ok := observationContext[key]; ok {
				args[key] = value
			}
		}
		if clipID := firstStringFromMap(observationContext, "selected_clip_id", "clip_id"); clipID != "" {
			args["clip_id"] = clipID
		}
		if requestedMode == "paired_io" {
			args["topology_class"] = card.TopologyEvidence.Classification
			args["topology_generation"] = card.TopologyEvidence.Generation
			args["resolve_semantic_compressor_window"] = true
		}
		response, err := s.harness.Invoke(ctx, harness.InvokeRequest{Tool: "mix.observe", Args: args,
			Context: map[string]any{"observation_only": true, "ordinary_agent_semantic_compressor": true}, Source: "semantic_compressor_planning", Confirmed: true,
			ToolCallID: "compressor_planning:" + sanitizeCanaryID(conversationID) + ":mix.observe"})
		if err != nil || !strings.EqualFold(response.Status, "ok") {
			return observation{}, fmt.Errorf("%s", firstNonEmpty(errorText(err), response.Error, response.Status))
		}
		projection := semanticCompressorProjectionContext(response.Result["com_projection"])
		if len(projection) == 0 {
			if observation := firstMapFromAny(response.Result["observation"]); len(observation) > 0 {
				projection = semanticCompressorProjectionContext(observation["com_projection"])
			}
		}
		if len(projection) == 0 {
			return observation{}, fmt.Errorf("mix.observe omitted COM projection")
		}
		return observation{projection: projection, result: response.Result}, nil
	}
	observed, err := call(mode, baseContext)
	if err == nil {
		// C2 parameter leaves are source-only reversible by contract. A paired
		// observation may return a usable partial projection without an error,
		// but that projection cannot authorize a C2 materialization. Explicitly
		// fall back to the source-only projection in that case instead of
		// passing the non-ready paired result into the execution ticket guard.
		if mode == com.ModePairedIO && contextBool(requestContext, "c2_source_only_reversible") &&
			!strings.EqualFold(firstNonEmptyText(observed.projection, "status"), com.StatusReady) {
			if fallback := intent.EvidenceRequest.FallbackMode; fallback != "" && fallback != "not_needed" && fallback != mode {
				if fallbackObservation, fallbackErr := call(fallback, baseContext); fallbackErr == nil {
					fallbackMode := firstNonEmptyText(fallbackObservation.projection, "mode")
					fallbackStatus := firstNonEmptyText(fallbackObservation.projection, "status")
					if c2ReversibleCompressorEvidence(requestContext, fallbackMode, fallbackStatus) {
						fallbackObservation.projection["requested_mode"] = mode
						fallbackObservation.projection["fallback_reason"] = "C2 source-only reversible leaf requires a non-paired execution projection"
						return fallbackObservation.projection
					}
				}
			}
		}
		if mode == com.ModePairedIO {
			initial := semanticCompressorObservationCandidate{Projection: observed.projection, Result: observed.result}
			selected, audit := selectSemanticCompressorEventCoverageWindow(baseContext, initial, func(retryContext map[string]any) (semanticCompressorObservationCandidate, error) {
				retry, retryErr := call(mode, retryContext)
				return semanticCompressorObservationCandidate{Projection: retry.projection, Result: retry.result}, retryErr
			})
			observed.projection, observed.result = selected.Projection, selected.Result
			if len(audit) > 0 {
				observed.projection["observation_window_selection"] = audit
			}
			adoptSemanticCompressorCaptureContext(requestContext, observed.result)
		}
		return observed.projection
	}
	if fallback := intent.EvidenceRequest.FallbackMode; fallback != "" && fallback != "not_needed" && fallback != mode {
		if fallbackObservation, fallbackErr := call(fallback, baseContext); fallbackErr == nil {
			fallbackObservation.projection["requested_mode"] = mode
			fallbackObservation.projection["fallback_reason"] = err.Error()
			return fallbackObservation.projection
		} else {
			err = fmt.Errorf("%v; fallback %s failed: %v", err, fallback, fallbackErr)
		}
	}
	return map[string]any{"status": "missing", "mode": "not_observed", "reason": err.Error(), "can_support_semantic_planning": false}
}

const (
	semanticCompressorRetryWindowSeconds = 10.0
	semanticCompressorMaxWindowRetries   = 3
)

type semanticCompressorObservationCandidate struct {
	Projection map[string]any
	Result     map[string]any
}

func selectSemanticCompressorEventCoverageWindow(baseContext map[string]any, initial semanticCompressorObservationCandidate,
	call func(map[string]any) (semanticCompressorObservationCandidate, error)) (semanticCompressorObservationCandidate, map[string]any) {
	if semanticCompressorHasExplicitObservationWindow(baseContext) || !semanticCompressorNeedsEventCoverageRetry(initial) {
		return initial, nil
	}
	windows := semanticCompressorRetryWindows(initial.Result)
	if len(windows) == 0 {
		return initial, nil
	}
	attempts := []map[string]any{semanticCompressorObservationAttempt(0, initial, nil)}
	selected := initial
	selectedAttempt := 0
	for index, window := range windows {
		retryContext := semanticCompressorRetryObservationContext(baseContext, window)
		candidate, err := call(retryContext)
		attempts = append(attempts, semanticCompressorObservationAttempt(index+1, candidate, err))
		if err == nil && strings.EqualFold(firstNonEmptyText(candidate.Projection, "mode"), com.ModePairedIO) &&
			strings.EqualFold(firstNonEmptyText(candidate.Projection, "status"), com.StatusReady) {
			selected = candidate
			selectedAttempt = index + 1
			break
		}
	}
	return selected, map[string]any{
		"strategy": "bounded_event_coverage_retry_v1", "read_only": true,
		"attempt_count": len(attempts), "selected_attempt": selectedAttempt,
		"ready_found": selectedAttempt > 0, "attempts": attempts,
	}
}

func semanticCompressorObservationRequestContext(requestContext map[string]any) map[string]any {
	out := make(map[string]any, len(requestContext))
	for key, value := range requestContext {
		out[key] = value
	}
	selectedClipWindow := mapValue(out["selected_clip_time_range"])
	if strings.EqualFold(firstNonEmptyText(selectedClipWindow, "source"), "selected_clip") {
		delete(out, "selected_clip_time_range")
	}
	return out
}

func semanticCompressorHasExplicitObservationWindow(context map[string]any) bool {
	if floatNumber(context["end_sample"]) > floatNumber(context["start_sample"]) {
		return true
	}
	for _, key := range []string{"time_selection", "selected_clip_range"} {
		if semanticCompressorActiveSecondsRange(mapValue(context[key])) {
			return true
		}
	}
	for _, value := range []any{context["selected_clip_ranges"]} {
		for _, row := range anyMapRows(value) {
			if semanticCompressorActiveSecondsRange(row) {
				return true
			}
		}
	}
	selectedClipWindow := mapValue(context["selected_clip_time_range"])
	if semanticCompressorActiveSecondsRange(selectedClipWindow) &&
		!strings.EqualFold(firstNonEmptyText(selectedClipWindow, "source"), "selected_clip") {
		return true
	}
	return floatNumber(context["listen_time_end_seconds"]) > floatNumber(context["listen_time_start_seconds"])
}

func semanticCompressorActiveSecondsRange(row map[string]any) bool {
	if len(row) == 0 || (row["active"] != nil && !boolValue(row["active"])) {
		return false
	}
	return floatNumber(row["end_seconds"]) > floatNumber(row["start_seconds"])
}

func semanticCompressorNeedsEventCoverageRetry(candidate semanticCompressorObservationCandidate) bool {
	if !strings.EqualFold(firstNonEmptyText(candidate.Projection, "mode"), com.ModePairedIO) ||
		!strings.EqualFold(firstNonEmptyText(candidate.Projection, "status"), com.StatusPartial) {
		return false
	}
	capture := mapValue(candidate.Result["com_evidence_capture"])
	if !strings.EqualFold(firstNonEmptyText(capture, "window_source"), "resolved_clip_default_10s") {
		return false
	}
	trust := mapValue(candidate.Projection["trust_quality"])
	for _, field := range contextStringSlice(trust["missing_fields"]) {
		if strings.EqualFold(strings.TrimSpace(field), "event_coverage") {
			return true
		}
	}
	return false
}

func semanticCompressorRetryWindows(result map[string]any) []map[string]any {
	resolved := mapValue(result["resolved_target"])
	clipStart := floatNumber(resolved["clip_start_seconds"])
	duration := floatNumber(resolved["duration_seconds"])
	if duration <= semanticCompressorRetryWindowSeconds {
		return nil
	}
	clipEnd := clipStart + duration
	capture := mapValue(result["com_evidence_capture"])
	initialStart := floatNumber(capture["start_seconds"])
	initialEnd := floatNumber(capture["end_seconds"])
	out := make([]map[string]any, 0, semanticCompressorMaxWindowRetries)
	for _, fraction := range []float64{0.25, 0.5, 0.75} {
		start := clipStart + duration*fraction - semanticCompressorRetryWindowSeconds/2
		start = math.Max(clipStart, math.Min(start, clipEnd-semanticCompressorRetryWindowSeconds))
		start = math.Round(start*1000) / 1000
		end := math.Min(clipEnd, start+semanticCompressorRetryWindowSeconds)
		if math.Abs(start-initialStart) < 0.001 && math.Abs(end-initialEnd) < 0.001 {
			continue
		}
		duplicate := false
		for _, existing := range out {
			if math.Abs(floatNumber(existing["start_seconds"])-start) < 0.001 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, map[string]any{"active": true, "start_seconds": start, "end_seconds": end,
				"source": "semantic_compressor_event_coverage_retry"})
		}
		if len(out) == semanticCompressorMaxWindowRetries {
			break
		}
	}
	return out
}

func semanticCompressorRetryObservationContext(base map[string]any, window map[string]any) map[string]any {
	out := semanticCompressorObservationRequestContext(base)
	for _, key := range []string{"start_sample", "end_sample", "selected_clip_time_range", "selected_clip_range", "selected_clip_ranges",
		"time_selection", "listen_time_start_seconds", "listen_time_end_seconds", "current_playhead_seconds", "playhead_seconds", "transport_position_seconds"} {
		delete(out, key)
	}
	retryWindow := make(map[string]any, len(window)+1)
	for key, value := range window {
		retryWindow[key] = value
	}
	if clipID := firstStringFromMap(base, "selected_clip_id", "clip_id"); clipID != "" {
		retryWindow["clip_id"] = clipID
	}
	out["selected_clip_range"] = retryWindow
	return out
}

func semanticCompressorObservationAttempt(index int, candidate semanticCompressorObservationCandidate, err error) map[string]any {
	row := map[string]any{"attempt": index}
	if err != nil {
		row["status"], row["error"] = "error", err.Error()
		return row
	}
	row["status"] = firstNonEmptyText(candidate.Projection, "status")
	capture := mapValue(candidate.Result["com_evidence_capture"])
	for _, key := range []string{"window_source", "start_seconds", "end_seconds"} {
		if value, ok := capture[key]; ok {
			row[key] = value
		}
	}
	trust := mapValue(candidate.Projection["trust_quality"])
	if missing := contextStringSlice(trust["missing_fields"]); len(missing) > 0 {
		row["missing_fields"] = missing
	}
	return row
}

func anyMapRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, value := range rows {
			if row := mapValue(value); len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func adoptSemanticCompressorCaptureContext(requestContext, result map[string]any) {
	capture := mapValue(result["com_evidence_capture"])
	if len(capture) == 0 {
		return
	}
	for _, key := range []string{"clip_id", "start_sample", "end_sample", "sample_rate", "channel_count"} {
		if value, ok := capture[key]; ok {
			requestContext[key] = value
		}
	}
	if clipID := firstNonEmptyText(capture, "clip_id"); clipID != "" {
		requestContext["selected_clip_id"] = clipID
	}
}

func semanticCompressorProjectionContext(value any) map[string]any {
	switch projection := value.(type) {
	case *com.Projection:
		if projection == nil {
			return nil
		}
		return com.ContextProjection(*projection)
	case com.Projection:
		return com.ContextProjection(projection)
	case map[string]any:
		return projection
	default:
		return nil
	}
}

func semanticCompressorPlanningResponse(conversationID string, requestContext map[string]any, card semanticeffect.AudioProcessorIdentityCard,
	intent semanticeffect.CompressorIntentPlan, comContext map[string]any, brief plugingrabber.CompressorControlBrief,
	plan semanticeffect.CompressorPlan, rejection *semanticeffect.CompressorMaterializationRejection) ChatResponse {
	data := map[string]any{"schema_version": "semantic_compressor.workflow.v1", "status": "planned", "planning_only": true,
		"mutation_authorized": false, "mutation_performed": false, "processor_identity_card": card, "intent_plan": intent,
		"com_observation": comContext, "control_brief": brief, "compressor_plan": plan, "progressive_disclosure": []string{"processor_identity_card", "com_observation", "candidate_control_brief"}}
	if rejection != nil {
		data["materialization_rejection"] = rejection
		data["constrained_revision_used"] = true
	}
	data = semanticCompressorWorkflowData(requestContext, data)
	goalID, runID := goalIDsFromContext(requestContext)
	return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
		Reply: fmt.Sprintf("已为 %s 生成压缩器语义控制方案：选择了 %s；方案已结合处理器名片、COM 证据和当前可达控制，但仍停留在只读规划阶段，没有修改任何参数。",
			card.Identity.Name, strings.Join(plan.SelectedAxes, "、")), Workflow: "semantic_compressor_planning", WorkflowData: data, GoalStatus: "completed", StopReason: "compressor_plan_ready"}
}

func semanticCompressorPlanningFailure(conversationID string, requestContext map[string]any, code string, err error) ChatResponse {
	goalID, runID := goalIDsFromContext(requestContext)
	if freeStateLoopActiveContext(requestContext) && freeStateTransientServiceError(errorText(err)) {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    "Compressor semantic planning encountered a transient model-service error. No parameters were changed.",
			Workflow: "semantic_compressor_planning", WorkflowData: semanticCompressorWorkflowData(requestContext, map[string]any{
				"schema_version": "semantic_compressor.workflow.v1", "status": "unavailable", "code": code,
				"planning_only": true, "mutation_performed": false, "error": errorText(err),
			}), GoalStatus: string(agentruntime.StatusFailed), StopReason: code, Error: errorText(err)}
	}
	return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
		Reply:    "压缩器语义规划无法在当前证据和控制边界内安全完成；已停止在只读阶段，没有修改任何参数。",
		Workflow: "semantic_compressor_planning", WorkflowData: semanticCompressorWorkflowData(requestContext, map[string]any{"schema_version": "semantic_compressor.workflow.v1", "status": "rejected", "code": code,
			"planning_only": true, "mutation_performed": false, "error": errorText(err)}), GoalStatus: "completed", StopReason: code, Error: errorText(err)}
}

func semanticCompressorWorkflowData(requestContext, data map[string]any) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	data["request_context"] = cloneContext(requestContext)
	if loop := firstMapFromAny(requestContext["free_state_reasoning_loop"]); len(loop) > 0 {
		data["free_state_reasoning_loop"] = cloneContext(loop)
	}
	return data
}
