package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/tools"
	"vit-daw-agent/internal/vps"
)

const (
	equalizerCapabilityInspectTool    = "capability.equalizer.inspect"
	equalizerCapabilityPlanTool       = "capability.equalizer.plan"
	equalizerCapabilityInspectCommand = "capability_equalizer_inspect"
	equalizerCapabilityPlanCommand    = "capability_equalizer_plan"
)

type equalizerCapabilityPlan struct {
	Request          spalEQV2Request
	Task             string
	MissingSemantic  []string
	MissingTechnical []string
}

type equalizerPluginEffectPlan struct {
	Task             string
	ApplyArgs        map[string]any
	MissingSemantic  []string
	MissingTechnical []string
}

func isEqualizerCapabilityToolCall(call planner.ToolCall) bool {
	tool := strings.ToLower(strings.TrimSpace(call.Tool))
	if tool == equalizerCapabilityInspectTool || tool == equalizerCapabilityPlanTool || tool == equalizerCapabilityInspectCommand || tool == equalizerCapabilityPlanCommand {
		return true
	}
	command := workflowCommandArgs(call.Command)
	name := strings.ToLower(firstNonEmpty(cleanContextText(command["cmd"]), cleanContextText(command["command"]), cleanContextText(command["tool"])))
	return name == equalizerCapabilityInspectTool || name == equalizerCapabilityPlanTool || name == equalizerCapabilityInspectCommand || name == equalizerCapabilityPlanCommand
}

// equalizerCapabilityInvokeRequest recognizes the capability seam at the
// stable HTTP invocation boundary as well as inside AgentLoop.  The generic
// Harness must not forward this name to Vit as a legacy host command: it is an
// Agent-owned compatibility planner which creates a governed B4 Proposal.
func equalizerCapabilityInvokeRequest(req harness.InvokeRequest) bool {
	return isEqualizerCapabilityToolCall(planner.ToolCall{
		Tool:    strings.TrimSpace(req.Tool),
		Args:    cloneStringAnyMap(req.Args),
		Command: cloneStringAnyMap(req.Command),
	})
}

// invokeEqualizerCapabilityHTTP adapts a direct local API call to the same
// capability executor used by AgentLoop.  It only creates a Proposal; the
// normal /agent/confirm endpoint remains the sole path that can execute a
// staging transaction.  Keeping this adapter here prevents the API boundary
// from bypassing the action matrix and accidentally emitting an unknown Vit
// command named capability_equalizer_plan.
func (s *Server) invokeEqualizerCapabilityHTTP(ctx context.Context, req harness.InvokeRequest) (harness.InvokeResponse, error) {
	callID := strings.TrimSpace(req.ToolCallID)
	if callID == "" {
		callID = "http_equalizer_" + randomID()
	}
	call := planner.ToolCall{
		ID:      callID,
		Tool:    strings.TrimSpace(req.Tool),
		Args:    cloneStringAnyMap(req.Args),
		Command: cloneStringAnyMap(req.Command),
	}
	out, err := s.invokeEqualizerCapabilityTool(ctx, executorpkg.Input{
		GoalID:    strings.TrimSpace(req.GoalID),
		RunID:     strings.TrimSpace(req.RunID),
		ToolCall:  call,
		Context:   cloneStringAnyMap(req.Context),
		Confirmed: req.Confirmed,
		Source:    firstNonEmpty(strings.TrimSpace(req.Source), "http"),
	})
	response := harness.InvokeResponse{
		Status:               firstNonEmpty(strings.TrimSpace(out.Status), "ok"),
		AgentActionID:        out.AgentActionID,
		Tool:                 firstNonEmpty(strings.TrimSpace(out.Tool), equalizerCapabilityToolName(call)),
		CommandName:          firstNonEmpty(strings.TrimSpace(out.CommandName), equalizerCapabilityPlanCommand),
		RiskLevel:            tools.RiskUndoable,
		RequiresConfirmation: out.RequiresConfirmation,
		Preview:              out.Preview,
		UndoLabel:            out.UndoLabel,
		Result:               out.Result,
		ProjectHistory:       out.ProjectHistory,
		Error:                out.Error,
	}
	if err != nil && strings.TrimSpace(response.Error) == "" {
		response.Error = err.Error()
	}
	return response, err
}

func equalizerCapabilityToolName(call planner.ToolCall) string {
	tool := strings.ToLower(strings.TrimSpace(call.Tool))
	if tool == equalizerCapabilityInspectTool || tool == equalizerCapabilityInspectCommand {
		return equalizerCapabilityInspectTool
	}
	if tool == equalizerCapabilityPlanTool || tool == equalizerCapabilityPlanCommand {
		return equalizerCapabilityPlanTool
	}
	command := workflowCommandArgs(call.Command)
	name := strings.ToLower(firstNonEmpty(cleanContextText(command["cmd"]), cleanContextText(command["command"]), cleanContextText(command["tool"])))
	if name == equalizerCapabilityInspectTool || name == equalizerCapabilityInspectCommand {
		return equalizerCapabilityInspectTool
	}
	return equalizerCapabilityPlanTool
}

func equalizerCapabilityToolArgs(call planner.ToolCall) map[string]any {
	out := cloneStringAnyMap(call.Args)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range workflowCommandArgs(call.Command) {
		if key == "cmd" || key == "command" || key == "tool" {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = value
		}
	}
	if desired := mapValue(out["desired"]); len(desired) > 0 {
		for key, value := range desired {
			if _, exists := out[key]; !exists {
				out[key] = value
			}
		}
	}
	return out
}

func (s *Server) invokeEqualizerCapabilityTool(ctx context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	toolCallID := strings.TrimSpace(in.ToolCall.ID)
	if toolCallID == "" {
		toolCallID = "equalizer_capability_step"
	}
	tool := equalizerCapabilityToolName(in.ToolCall)
	command := equalizerCapabilityPlanCommand
	if tool == equalizerCapabilityInspectTool {
		command = equalizerCapabilityInspectCommand
	}
	base := executorpkg.Result{ToolCallID: toolCallID, Tool: tool, CommandName: command, Status: "ok"}
	// Inspect remains read-only. Complete plans delegate directly to the B4
	// semantic plug-in control owner and never enter SPAL/Catalog execution.
	if s == nil || s.harness == nil {
		base.Status = "error"
		base.Error = "equalizer capability runtime is unavailable"
		return base, fmt.Errorf("%s", base.Error)
	}
	args := equalizerCapabilityToolArgs(in.ToolCall)
	if tool == equalizerCapabilityInspectTool {
		base.Result = s.inspectEqualizerCapability(ctx, args, in.Context)
		return base, nil
	}

	plan := buildEqualizerPluginEffectPlan(args, in.Context)
	if len(plan.MissingSemantic) > 0 || len(plan.MissingTechnical) > 0 {
		payload := map[string]any{
			"status":                   "needs_agent_input",
			"role":                     "equalizer",
			"work_card_capability_id":  vps.EqualizerCapabilityID,
			"task":                     plan.Task,
			"missing_semantic_fields":  append([]string(nil), plan.MissingSemantic...),
			"missing_technical_fields": append([]string(nil), plan.MissingTechnical...),
			"execution_route":          pluginEffectControlCapabilityID,
			"reply":                    equalizerPluginEffectGapReply(plan),
		}
		base.Result = payload
		return base, nil
	}

	conversationID := firstNonEmpty(cleanContextText(in.Context["conversation_id"]), "agent_equalizer_"+in.GoalID)
	message := firstNonEmpty(cleanContextText(in.Context["user_message"]), cleanContextText(args["intent"]), "equalizer capability action")
	requestContext := pluginEffectControlInvocationContext(
		contextWithGoal(contextWithConversationID(cloneStringAnyMap(in.Context), conversationID), in.GoalID, in.RunID),
		plan.ApplyArgs,
		toolCallID,
	)
	if in.Confirmed {
		requestContext["raw_tool_confirmation_ignored"] = true
	}
	response := s.runPluginEffectControlRuntime(ctx, conversationID, ChatRequest{
		ConversationID: conversationID,
		Message:        message,
		Context:        requestContext,
	}, agentruntime.Goal{GoalID: in.GoalID, RunID: in.RunID, Summary: message, Status: agentruntime.StatusRunning})
	s.attachInteractionRequests(&response)
	base.Result = equalizerCapabilityResponsePayload(response)
	base.Preview = response.Preview
	base.ProjectHistory = response.ProjectHistory
	// The delegated B4 Proposal owns the durable confirmation interaction.
	base.RequiresConfirmation = false
	if strings.TrimSpace(response.Error) != "" {
		base.Status = "error"
		base.Error = response.Error
		return base, fmt.Errorf("%s", response.Error)
	}
	return base, nil
}

func equalizerCapabilityResponsePayload(response ChatResponse) map[string]any {
	executionRoute := firstNonEmpty(cleanContextText(response.WorkflowData["execution_route"]), pluginEffectControlCapabilityID)
	source := firstNonEmpty(cleanContextText(response.WorkflowData["source"]), "b4_plugin_effect_control")
	payload := map[string]any{
		"status":                  response.GoalStatus,
		"reply":                   response.Reply,
		"needs_confirmation":      response.NeedsConfirmation,
		"plan_id":                 response.PlanID,
		"preview":                 response.Preview,
		"workflow":                response.Workflow,
		"workflow_data":           response.WorkflowData,
		"proposal_presentation":   response.ProposalPresentation,
		"interaction_requests":    response.InteractionRequests,
		"role":                    "equalizer",
		"work_card_capability_id": vps.EqualizerCapabilityID,
		"execution_route":         executionRoute,
		"source":                  source,
	}
	if len(response.ProjectHistory) > 0 {
		payload["project_history"] = response.ProjectHistory
	}
	if strings.TrimSpace(response.Error) != "" {
		payload["error"] = response.Error
	}
	return payload
}

func buildEqualizerPluginEffectPlan(args, requestContext map[string]any) equalizerPluginEffectPlan {
	values := cloneStringAnyMap(args)
	if values == nil {
		values = map[string]any{}
	}
	task := normalizeEqualizerTask(firstNonEmpty(cleanContextText(values["task"]), cleanContextText(values["action"]), cleanContextText(values["operation"])))
	trackID := firstNonEmpty(
		cleanContextText(values["track_id"]), cleanContextText(values["target_ref"]),
		spalReferenceEQContextText(requestContext, "selected_track_id"), spalReferenceEQContextText(requestContext, "selected_plugin_track_id"),
		spalReferenceEQContextText(requestContext, "primary_selected_track_id"),
	)
	pluginID := firstNonEmpty(cleanContextText(values["plugin_id"]), spalReferenceEQContextText(requestContext, "selected_plugin_id"), spalReferenceEQContextText(requestContext, "primary_selected_plugin_id"))
	result := equalizerPluginEffectPlan{Task: task}
	if task == "" {
		result.MissingSemantic = append(result.MissingSemantic, "task")
	}
	if trackID == "" {
		result.MissingTechnical = append(result.MissingTechnical, "track_id")
	}
	if pluginID == "" {
		result.MissingTechnical = append(result.MissingTechnical, "plugin_id")
	}
	target := map[string]any{}
	control := ""
	switch task {
	case "spectral_region_adjust":
		frequency, frequencyOK := equalizerNumber(values["frequency_hz"])
		gain, gainOK := equalizerNumber(values["gain_db"])
		q, qOK := equalizerNumber(values["q"])
		if !frequencyOK || frequency <= 0 {
			result.MissingSemantic = append(result.MissingSemantic, "frequency_hz")
		} else {
			target["freq_hz"] = frequency
		}
		if !gainOK {
			result.MissingSemantic = append(result.MissingSemantic, "gain_db")
		} else {
			target["gain_db"] = gain
			switch {
			case gain < 0:
				control = "eq.cut_region"
			case gain > 0:
				control = "eq.boost_region"
			default:
				control = "eq.set_region"
			}
		}
		if !qOK || q <= 0 {
			result.MissingSemantic = append(result.MissingSemantic, "q")
		} else {
			target["q"] = q
		}
		if shape := normalizeEqualizerShape(cleanContextText(values["response_shape"])); shape != "" {
			target["response_shape"] = shape
		}
		if _, exists := values["enabled"]; exists {
			target["enabled"] = equalizerBooleanNumber(values["enabled"], 1)
		}
	case "highpass", "lowpass":
		control = "eq." + task
		frequency, frequencyOK := equalizerNumber(firstNonNil(values["cutoff_frequency_hz"], values["frequency_hz"]))
		if !frequencyOK || frequency <= 0 {
			result.MissingSemantic = append(result.MissingSemantic, "cutoff_frequency_hz")
		} else {
			target["freq_hz"] = frequency
		}
		if slope, ok := equalizerNumber(values["slope_db_per_octave"]); ok && slope > 0 {
			target["slope_db_per_octave"] = slope
		} else {
			result.MissingSemantic = append(result.MissingSemantic, "slope_db_per_octave")
		}
	case "output_control":
		control = "eq.output_control"
		if _, exists := values["bypass"]; exists {
			target["bypass"] = equalizerBooleanNumber(values["bypass"], 0)
		}
		if value, ok := equalizerNumber(values["dry_mix_percent"]); ok {
			target["dry_mix_percent"] = value
		}
		if value, ok := equalizerNumber(values["output_gain_db"]); ok {
			target["output_gain_db"] = value
		}
		if len(target) == 0 {
			result.MissingSemantic = append(result.MissingSemantic, "bypass_or_dry_mix_percent_or_output_gain_db")
		}
	case "":
	default:
		result.MissingSemantic = append(result.MissingSemantic, "supported_task(spectral_region_adjust|highpass|lowpass|output_control)")
	}
	if len(result.MissingSemantic) == 0 && len(result.MissingTechnical) == 0 {
		result.ApplyArgs = map[string]any{"track_id": trackID, "plugin_id": pluginID, "control": control, "target": target}
	}
	return result
}

func equalizerPluginEffectGapReply(plan equalizerPluginEffectPlan) string {
	missing := append(append([]string(nil), plan.MissingSemantic...), plan.MissingTechnical...)
	return "equalizer 兼容工具还不能形成 B4 插件控制提案；请补充：" + strings.Join(missing, "、") + "。"
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
			return value
		}
	}
	return nil
}

func buildEqualizerCapabilityPlan(args, requestContext map[string]any) equalizerCapabilityPlan {
	values := cloneStringAnyMap(args)
	if values == nil {
		values = map[string]any{}
	}
	task := normalizeEqualizerTask(firstNonEmpty(cleanContextText(values["task"]), cleanContextText(values["action"]), cleanContextText(values["operation"])))
	target := firstNonEmpty(
		cleanContextText(values["target_ref"]), cleanContextText(values["track_id"]),
		spalReferenceEQContextText(requestContext, "selected_track_id"), spalReferenceEQContextText(requestContext, "selected_plugin_track_id"),
		spalReferenceEQContextText(requestContext, "primary_selected_track_id"),
	)
	pluginID := firstNonEmpty(cleanContextText(values["plugin_id"]), spalReferenceEQContextText(requestContext, "selected_plugin_id"), spalReferenceEQContextText(requestContext, "primary_selected_plugin_id"))
	credentialID := firstNonEmpty(cleanContextText(values["provider_credential_id"]), spalReferenceEQContextText(requestContext, "spal_provider_credential_id"))
	result := equalizerCapabilityPlan{Task: task, Request: spalEQV2Request{TargetRef: target, PluginID: pluginID, ProviderCredentialID: credentialID}}
	if task == "" {
		result.MissingSemantic = append(result.MissingSemantic, "task")
		return result
	}
	instruction := spal.Instruction{TargetRef: target, Parameters: map[string]float64{}, StringParameters: map[string]string{}}
	if target == "" {
		result.MissingTechnical = append(result.MissingTechnical, "target_ref")
	}

	switch task {
	case "spectral_region_adjust":
		instruction.SchemaID = spal.EQBandPatchControlID
		band := normalizeEqualizerBandRef(cleanContextText(values["band_ref"]))
		if band == "" {
			result.MissingTechnical = append(result.MissingTechnical, "band_ref")
		} else {
			instruction.StringParameters["band_ref"] = band
		}
		shape := normalizeEqualizerShape(cleanContextText(values["response_shape"]))
		if shape == "" {
			result.MissingSemantic = append(result.MissingSemantic, "response_shape")
		} else {
			instruction.StringParameters["response_shape"] = shape
		}
		if value, ok := equalizerNumber(values["frequency_hz"]); ok && value > 0 {
			instruction.Parameters["frequency_hz"] = value
		} else {
			result.MissingSemantic = append(result.MissingSemantic, "frequency_hz")
		}
		if value, ok := equalizerNumber(values["gain_db"]); ok {
			instruction.Parameters["gain_db"] = value
		} else {
			result.MissingSemantic = append(result.MissingSemantic, "gain_db")
		}
		if value, ok := equalizerNumber(values["q"]); ok && value > 0 {
			instruction.Parameters["q"] = value
		} else {
			result.MissingSemantic = append(result.MissingSemantic, "q")
		}
		instruction.Parameters["enabled"] = equalizerBooleanNumber(values["enabled"], 1)
		maxGain := 18.0
		instruction.SafetyBounds.MaxAbsoluteGainDB = &maxGain
	case "highpass", "lowpass":
		instruction.SchemaID = spal.EQPassFilterPatchControlID
		instruction.StringParameters["filter_kind"] = task
		instruction.Parameters["enabled"] = equalizerBooleanNumber(values["enabled"], 1)
		if value, ok := equalizerNumber(firstPresentAny(values, "cutoff_frequency_hz", "frequency_hz")); ok && value > 0 {
			instruction.Parameters["cutoff_frequency_hz"] = value
		} else {
			result.MissingSemantic = append(result.MissingSemantic, "cutoff_frequency_hz")
		}
		if value, ok := equalizerNumber(values["slope_db_per_octave"]); ok && value > 0 {
			instruction.Parameters["slope_db_per_octave"] = value
		} else {
			result.MissingSemantic = append(result.MissingSemantic, "slope_db_per_octave")
		}
	case "output_control":
		instruction.SchemaID = spal.EQOutputPatchControlID
		if _, exists := values["bypass"]; exists {
			instruction.Parameters["bypass"] = equalizerBooleanNumber(values["bypass"], 0)
		}
		if value, ok := equalizerNumber(values["dry_mix_percent"]); ok {
			instruction.Parameters["dry_mix_percent"] = value
		}
		if value, ok := equalizerNumber(values["output_gain_db"]); ok {
			instruction.Parameters["output_gain_db"] = value
		}
		if len(instruction.Parameters) == 0 {
			result.MissingSemantic = append(result.MissingSemantic, "bypass_or_dry_mix_percent_or_output_gain_db")
		}
	default:
		result.MissingSemantic = append(result.MissingSemantic, "supported_task(spectral_region_adjust|highpass|lowpass|output_control)")
	}
	result.Request.Instruction = instruction
	result.Request.Missing = append(append([]string(nil), result.MissingSemantic...), result.MissingTechnical...)
	if len(result.Request.Missing) == 0 {
		if err := instruction.Validate(); err != nil {
			result.MissingSemantic = append(result.MissingSemantic, err.Error())
			result.Request.Missing = append(result.Request.Missing, err.Error())
		}
	}
	return result
}

func normalizeEqualizerTask(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	switch value {
	case "spectral_region_adjust", "static_eq", "static_parametric", "band_patch", "bell", "boost_region", "cut_region":
		return "spectral_region_adjust"
	case "highpass", "high_pass", "hp", "lowcut", "low_cut":
		return "highpass"
	case "lowpass", "low_pass", "lp", "highcut", "high_cut":
		return "lowpass"
	case "output", "output_control", "plugin_output":
		return "output_control"
	default:
		return ""
	}
}

func normalizeEqualizerBandRef(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "band", "")
	value = strings.TrimSpace(value)
	switch value {
	case "1", "i", "b1":
		return "b1"
	case "2", "ii", "b2":
		return "b2"
	case "3", "iii", "b3":
		return "b3"
	case "4", "iv", "b4":
		return "b4"
	default:
		return ""
	}
}

func normalizeEqualizerShape(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	switch value {
	case "bell", "peaking", "peak":
		return "bell"
	case "low_shelf", "lowshelf":
		return "low_shelf"
	case "high_shelf", "highshelf":
		return "high_shelf"
	default:
		return ""
	}
}

func equalizerNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case float32:
		v := float64(typed)
		return v, !math.IsNaN(v) && !math.IsInf(v, 0)
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		v, err := typed.Float64()
		return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" || text == "<nil>" {
			return 0, false
		}
		v, err := strconv.ParseFloat(text, 64)
		return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
	}
}

func equalizerBooleanNumber(value any, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		if typed {
			return 1
		}
		return 0
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "on", "enabled", "enable", "open", "1":
			return 1
		case "false", "off", "disabled", "disable", "close", "0":
			return 0
		}
	}
	if number, ok := equalizerNumber(value); ok {
		if number == 0 {
			return 0
		}
		return 1
	}
	return fallback
}

func (s *Server) inspectEqualizerCapability(ctx context.Context, args, requestContext map[string]any) map[string]any {
	result := map[string]any{
		"role":                    "equalizer",
		"work_card_capability_id": vps.EqualizerCapabilityID,
		"execution_route":         pluginEffectControlCapabilityID,
		"status":                  "blocked",
	}
	if staging := s.inspectVPSForgeStagingEQ(ctx, args, requestContext); len(staging) > 0 {
		result["vpsforge_staging"] = staging
	}
	library, err := s.userVPSLibrary()
	if err != nil {
		result["reason"] = err.Error()
		return result
	}
	catalog, err := library.Catalog()
	if err != nil {
		result["reason"] = err.Error()
		return result
	}
	entries := make([]map[string]any, 0)
	schemas := []string{}
	for _, entry := range catalog.Entries {
		if entry.CapabilityID != vps.EqualizerCapabilityID {
			continue
		}
		entries = append(entries, map[string]any{
			"plugin_name":          entry.PluginIdentity.Name,
			"manufacturer":         entry.PluginIdentity.Manufacturer,
			"format":               entry.PluginIdentity.Format,
			"version":              entry.PluginIdentity.Version,
			"credential_id":        entry.CredentialID,
			"vps_id":               entry.VPSID,
			"supported_schemas":    append([]string(nil), entry.SupportedSchemas...),
			"special_capabilities": entry.SpecialCapabilities,
		})
		schemas = appendUniqueSPALRefs(schemas, entry.SupportedSchemas...)
	}
	result["generic_actions"] = schemas
	result["catalog_providers"] = entries
	if len(entries) == 0 {
		if staging := mapValue(result["vpsforge_staging"]); cleanContextText(staging["status"]) == "selected_staging_candidate" {
			result["status"] = "staging_candidate"
			result["reason"] = "explicit_nonrouteable_vpsforge_staging_slice_available"
			return result
		}
		result["reason"] = "no_verified_equalizer_provider"
		return result
	}
	state, stateErr := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if stateErr != nil || state == nil || !state.OK() {
		result["status"] = "catalog_only"
		result["reason"] = "project_state_unavailable"
		return result
	}
	targetRefs := compactContextStrings(
		cleanContextText(args["target_ref"]), cleanContextText(args["track_id"]),
		spalReferenceEQContextText(requestContext, "selected_track_id"), spalReferenceEQContextText(requestContext, "selected_plugin_track_id"),
	)
	target := firstNonEmpty(targetRefs...)
	pluginID := firstNonEmpty(cleanContextText(args["plugin_id"]), spalReferenceEQContextText(requestContext, "selected_plugin_id"))
	candidates := observedVPSProviderCandidates(state)
	// The model sometimes passes an unresolved context-field name (e.g.
	// "current_selection") instead of an actual track ref in target_ref.
	// Rather than let that literal value shadow a perfectly good track_id or
	// selected_track_id, accept a match against any supplied ref.
	targetMatched := make([]*spalReferenceEQProviderCandidate, 0, len(candidates))
	for i := range candidates {
		if len(targetRefs) == 0 || spalReferenceEQProviderTargetMatchesAny(candidates[i], targetRefs) {
			targetMatched = append(targetMatched, &candidates[i])
		}
	}
	loaded := []map[string]any{}
	var selectedCandidate *spalReferenceEQProviderCandidate
	if pluginID != "" {
		for _, candidate := range targetMatched {
			if strings.EqualFold(pluginID, candidate.PluginID) {
				selectedCandidate = candidate
				break
			}
		}
		// Rack slot IDs (plugin_id) get reused/reassigned across a project's
		// lifetime, and the calling model can keep citing a stale plugin_id
		// across several turns even after a fresher one was already reported
		// back to it (observed live). If the stale id matches nothing but the
		// targeted scope has exactly one plug-in loaded, that is unambiguously
		// the instance the user means - fall back to it instead of reporting
		// "not found".
		if selectedCandidate == nil && len(targetMatched) == 1 {
			selectedCandidate = targetMatched[0]
		}
	}
	for _, candidate := range targetMatched {
		if pluginID != "" && (selectedCandidate == nil || candidate != selectedCandidate) {
			continue
		}
		for _, entry := range catalog.Entries {
			if entry.CapabilityID != vps.EqualizerCapabilityID || !strings.EqualFold(strings.TrimSpace(entry.PluginIdentity.Name), strings.TrimSpace(candidate.PluginName)) {
				continue
			}
			loaded = append(loaded, map[string]any{"track_id": candidate.TrackID, "track_name": candidate.TrackName, "target_ref": candidate.TargetRef, "plugin_id": candidate.PluginID, "plugin_name": candidate.PluginName, "format": candidate.Format, "credential_id": entry.CredentialID})
		}
	}
	result["loaded_provider_candidates"] = loaded
	if len(loaded) > 0 {
		result["status"] = "loaded_provider_candidate"
		result["reason"] = "planning_will_fresh_validate_identity_fingerprint_and_schema"
	} else {
		if pluginID != "" {
			// The caller explicitly selected a real plug-in instance.  Absence
			// of a verified Provider for that instance is not permission to
			// insert a different plug-in such as TDR Nova.
			result["status"] = "selected_plugin_not_routable"
			result["selected_plugin_fallback_forbidden"] = true
			// Rack slot IDs (plugin_id) get reused across the lifetime of a
			// project once a plug-in is removed and another is loaded into the
			// same slot, and a Plugin Grabber profile's saved plugin_id can
			// likewise drift after the plug-in it belongs to gets reloaded into
			// a different slot. Match the grabber fallback on the currently
			// observed plug-in's name/path instead of plugin_id.
			//
			// Use selectedCandidate's plugin_id (the slot actually observed
			// right now), not the caller-supplied pluginID: the caller's
			// value may itself be stale (see the single-instance fallback
			// above), and the reported learned_grabber_fallback.plugin_id is
			// what the model will cite in the immediate follow-up
			// plugin_grabber_apply_control call - if it echoes back a slot
			// that no longer exists, that follow-up call fails the same way
			// get_plugin_parameters did.
			var pluginName, pluginPath, resolvedPluginID string
			if selectedCandidate != nil {
				pluginName, pluginPath = selectedCandidate.PluginName, selectedCandidate.PluginPath
				resolvedPluginID = selectedCandidate.PluginID
			} else {
				resolvedPluginID = pluginID
			}
			if fallback := s.eqLearnedGrabberFallback(ctx, requestContext, target, resolvedPluginID, pluginName, pluginPath); len(fallback) > 0 {
				result["learned_grabber_fallback"] = fallback
				// selected_plugin_fallback_forbidden above forbids silently
				// loading or swapping in a *different* plug-in - it does not
				// forbid controlling the plug-in the user already has selected
				// through its own learned Plugin Grabber profile. Say so
				// explicitly: observed live, a model read the two adjacent
				// "forbidden"/"no_..._fallback" strings in this payload as
				// "nothing can be done" and asked the user to swap plugins
				// instead of calling plugin_grabber_apply_control, even though
				// a fresh learned profile for the selected plug-in was right
				// there in learned_grabber_fallback.
				result["reason"] = "selected_plugin_has_no_verified_provider; a_learned_grabber_fallback_is_available_below_use_plugin_grabber_apply_control_do_not_ask_to_swap_plugin"
				result["next_action"] = "plugin_grabber_apply_control"
			} else {
				result["reason"] = "selected_plugin_has_no_verified_provider; no_plugin_loading_or_provider_fallback"
			}
			return result
		}
		result["status"] = "requires_provisioning"
		result["reason"] = "verified_equalizer_exists_but_no_matching_loaded_instance"
	}
	return result
}

// eqLearnedGrabberFallback reports whether the explicitly selected plugin
// (which has no verified SPAL Provider) has a usable learned Plugin Grabber
// profile with eq-shaped virtual controls, so the chat model can fall back
// to plugin_grabber_apply_control instead of asking the user to swap plugins.
//
// Rack slot IDs (plugin_id) are recycled across a project's lifetime: once a
// plug-in is removed and a different plug-in loaded into the same slot, that
// slot's plugin_id is reused, and a Plugin Grabber profile's own saved
// plugin_id reflects whichever slot it was learned in at save time, which can
// drift from the plugin_id it currently occupies after a reload. Matching on
// plugin_id is therefore unreliable; identity here is resolved the same way
// mixcontrolsurface.samePlugin resolves it elsewhere in this codebase - by
// plugin_path first, then by normalized plugin name. pluginName/pluginPath
// are the identity of the plug-in actually observed in the target rack slot
// right now.
func (s *Server) eqLearnedGrabberFallback(ctx context.Context, requestContext map[string]any, target, pluginID, pluginName, pluginPath string) map[string]any {
	if s == nil || s.harness == nil || target == "" || pluginID == "" || (pluginName == "" && pluginPath == "") {
		return nil
	}
	args := map[string]any{}
	if target != "" {
		args["track_id"] = target
	}
	if pluginID != "" {
		args["plugin_id"] = pluginID
	}
	resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:      "plugin_grabber.get_project_profiles",
		Args:      args,
		Context:   requestContext,
		Source:    "equalizer_inspect_grabber_fallback",
		Confirmed: true,
	})
	if err != nil || resp.Status != "ok" {
		return nil
	}
	profiles := mapRowsValue(resp.Result["plugin_grabber_profiles"])
	if len(profiles) == 0 {
		profiles = mapRowsValue(resp.Result["profiles"])
	}
	return matchEqGrabberFallbackProfile(profiles, target, pluginID, pluginName, pluginPath)
}

// matchEqGrabberFallbackProfile finds the profile whose plugin_identity
// matches the plug-in actually loaded right now, by plugin_path first (most
// stable) then by normalized plugin name - never by the recycled rack slot
// plugin_id. See eqLearnedGrabberFallback for why plugin_id is untrustworthy.
func matchEqGrabberFallbackProfile(profiles []map[string]any, trackID, pluginID, pluginName, pluginPath string) map[string]any {
	if trackID == "" || pluginID == "" || (pluginName == "" && pluginPath == "") {
		return nil
	}
	normalizedExpectName := strings.ToLower(strings.TrimSpace(pluginName))
	normalizedExpectPath := strings.ToLower(strings.TrimSpace(pluginPath))
	for _, profile := range profiles {
		identity := mapValue(profile["plugin_identity"])
		if len(identity) == 0 {
			identity = mapValue(mapValue(profile["plugin_skill"])["identity"])
		}
		profilePath := strings.ToLower(strings.TrimSpace(firstNonEmpty(cleanContextText(identity["plugin_path"]), cleanContextText(identity["path"]))))
		profileName := strings.ToLower(strings.TrimSpace(firstNonEmpty(cleanContextText(identity["plugin_name"]), cleanContextText(identity["name"]))))
		pathMatches := normalizedExpectPath != "" && profilePath != "" && profilePath == normalizedExpectPath
		nameMatches := normalizedExpectName != "" && profileName != "" && profileName == normalizedExpectName
		if !pathMatches && !nameMatches {
			continue
		}
		if !eqGrabberProfileHasEQControls(profile) {
			continue
		}
		status := "fresh"
		if eqGrabberProfileIsStale(profile) {
			status = "stale"
		}
		return map[string]any{
			"status":           status,
			"track_id":         trackID,
			"plugin_id":        pluginID,
			"profile_key":      firstNonEmpty(cleanContextText(identity["profile_key"]), cleanContextText(profile["profile_id"])),
			"grabber_controls": []string{"eq.cut_region", "eq.boost_region", "eq.set_region"},
		}
	}
	return nil
}

func eqGrabberProfileHasEQControls(profile map[string]any) bool {
	class := strings.ToLower(cleanContextText(profile["class"]))
	if class == "eq" || class == "equalizer" {
		return true
	}
	for _, control := range mapRowsValue(profile["virtual_controls"]) {
		if strings.HasPrefix(strings.ToLower(cleanContextText(control["name"])), "eq.") {
			return true
		}
	}
	return false
}

func eqGrabberProfileIsStale(profile map[string]any) bool {
	if boolFromAny(profile["stale"]) || boolFromAny(profile["profile_stale"]) {
		return true
	}
	if len(anyListToStrings(profile["profile_stale_param_ids"])) > 0 {
		return true
	}
	snapshot := mapValue(profile["parameter_snapshot"])
	if len(anyListToStrings(snapshot["profile_stale_param_ids"])) > 0 {
		return true
	}
	status := strings.ToLower(strings.Join([]string{
		cleanContextText(profile["status"]),
		cleanContextText(profile["profile_status"]),
		cleanContextText(profile["signature_status"]),
	}, " "))
	return strings.Contains(status, "stale")
}

func boolFromAny(value any) bool {
	b, ok := value.(bool)
	return ok && b
}

func anyListToStrings(value any) []string {
	rows, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		text := strings.TrimSpace(fmt.Sprint(row))
		if text != "" && text != "<nil>" {
			out = append(out, text)
		}
	}
	return out
}

// inspectVPSForgeStagingEQ deliberately reports the staging slice beside the
// verified Catalog rather than inside it.  This lets the Agent elect the
// explicit local staging path while preserving the fact that it is not a
// SPAL-routable Provider.
func (s *Server) inspectVPSForgeStagingEQ(ctx context.Context, args, requestContext map[string]any) map[string]any {
	config, enabled, err := s.loadVPSForgeStagingEQConfig()
	if !enabled {
		return nil
	}
	result := map[string]any{
		"source":              vpsForgeStagingEQSource,
		"routing_eligible":    false,
		"catalog_visible":     false,
		"credential_issuance": false,
		"spal_dispatch":       false,
		"supported_task":      "equalizer.v2 staging action matrix",
	}
	if err != nil {
		result["status"] = "configuration_error"
		result["reason"] = err.Error()
		return result
	}
	result["vps_id"] = config.Artifact.VPSID
	result["vps_revision"] = config.Artifact.VPSRevision
	result["action_implementations"] = config.Artifact.BadgeActionImplementations
	target := vpsForgeStagingTargetFromRequest(spalEQV2Request{
		TargetRef: firstNonEmpty(cleanContextText(args["target_ref"]), cleanContextText(args["track_id"]), spalReferenceEQContextText(requestContext, "selected_track_id"), spalReferenceEQContextText(requestContext, "selected_plugin_track_id")),
		PluginID:  firstNonEmpty(cleanContextText(args["plugin_id"]), spalReferenceEQContextText(requestContext, "selected_plugin_id"), spalReferenceEQContextText(requestContext, "primary_selected_plugin_id")),
	})
	if target.valid() != nil {
		result["status"] = "requires_selected_plugin_instance"
		return result
	}
	fresh, readErr := s.readVPSDraftTestParameterSurface(ctx, target, vpsForgeStagingRequestContext(config.Artifact.VPSID, "inspect"))
	if readErr != nil {
		result["status"] = "selected_plugin_unreadable"
		result["reason"] = readErr.Error()
		return result
	}
	digest := buildPluginParameterDigest(fresh)
	if matchErr := vpsForgeStagingTargetMatchesDigest(target, config.Document, digest); matchErr != nil {
		result["status"] = "configured_for_different_plugin"
		result["reason"] = matchErr.Error()
		return result
	}
	result["status"] = "selected_staging_candidate"
	result["target"] = target
	return result
}

func (s *Server) inspectEqualizerBandResources(ctx context.Context, request spalEQV2Request) (map[string]any, error) {
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("project state is unavailable")
	}
	probe := request
	probe.Instruction.Parameters = cloneFloatMap(request.Instruction.Parameters)
	probe.Instruction.StringParameters = cloneStringMap(request.Instruction.StringParameters)
	var resolved vpsEQV2ResolvedProvider
	var resolveErr error
	for _, bandRef := range []string{"b1", "b2", "b3", "b4"} {
		probe.Instruction.StringParameters["band_ref"] = bandRef
		resolved, _, resolveErr = s.resolveVPSEQV2Provider(ctx, state, vpsEQV2ResolveRequest{TargetRef: probe.TargetRef, PluginID: probe.PluginID, ProviderCredentialID: probe.ProviderCredentialID, Instruction: probe.Instruction})
		if resolveErr == nil {
			break
		}
	}
	if resolveErr != nil {
		return nil, resolveErr
	}
	rows := equalizerBandResourceRows(resolved, request.Instruction.Parameters["frequency_hz"])
	result := map[string]any{
		"provider_status":          "verified_loaded",
		"provider_plugin_name":     resolved.Document.PluginIdentity.Name,
		"provider_credential_id":   resolved.Credential.ID,
		"band_resource_candidates": rows,
	}
	for _, row := range rows {
		stateName := cleanContextText(row["resource_state"])
		if stateName == "neutral" || stateName == "disabled" {
			result["recommended_band_ref"] = row["band_ref"]
			result["recommendation_reason"] = "currently non-processing Band closest to the requested frequency; Agent must still disclose and choose it"
			break
		}
	}
	if result["recommended_band_ref"] == nil {
		result["resource_blocker"] = "all_conformed_bands_appear_in_use; Agent should ask which Band may be replaced"
	}
	return result, nil
}

func equalizerBandResourceRows(provider vpsEQV2ResolvedProvider, targetFrequency float64) []map[string]any {
	refs := make([]string, 0, len(provider.Definition.Binding.Bands))
	for ref := range provider.Definition.Binding.Bands {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	rows := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		binding := provider.Definition.Binding.Bands[ref]
		enabled := equalizerDigestParameter(provider.LiveDigest, binding.Enabled.ParameterID)
		shape := equalizerDigestParameter(provider.LiveDigest, binding.ResponseShape.ParameterID)
		frequency := equalizerDigestParameter(provider.LiveDigest, binding.FrequencyHz.ParameterID)
		gain := equalizerDigestParameter(provider.LiveDigest, binding.GainDB.ParameterID)
		q := equalizerDigestParameter(provider.LiveDigest, binding.Q.ParameterID)
		stateName := "in_use"
		if !equalizerParameterEnabled(enabled) {
			stateName = "disabled"
		} else if equalizerParameterNearZero(gain) {
			stateName = "neutral"
		}
		row := map[string]any{
			"band_ref":       ref,
			"resource_state": stateName,
			"enabled":        equalizerParameterEnabled(enabled),
			"response_shape": equalizerEnumLabel(shape, binding.ResponseShape.Values),
			"frequency":      firstNonEmpty(frequency.ValueText, cleanContextText(frequency.Value)),
			"gain":           firstNonEmpty(gain.ValueText, cleanContextText(gain.Value)),
			"q":              firstNonEmpty(q.ValueText, cleanContextText(q.Value)),
		}
		if currentFrequency, ok := equalizerDisplayNumber(frequency); ok && targetFrequency > 0 {
			row["frequency_distance_octaves"] = math.Abs(math.Log2(currentFrequency / targetFrequency))
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := equalizerResourcePriority(cleanContextText(rows[i]["resource_state"])), equalizerResourcePriority(cleanContextText(rows[j]["resource_state"]))
		if left != right {
			return left < right
		}
		li, _ := equalizerNumber(rows[i]["frequency_distance_octaves"])
		rj, _ := equalizerNumber(rows[j]["frequency_distance_octaves"])
		if li != rj {
			return li < rj
		}
		return cleanContextText(rows[i]["band_ref"]) < cleanContextText(rows[j]["band_ref"])
	})
	return rows
}

func equalizerResourcePriority(state string) int {
	switch state {
	case "neutral":
		return 0
	case "disabled":
		return 1
	default:
		return 2
	}
}

func equalizerDigestParameter(digest pluginParameterDigest, id string) pluginParameterInfo {
	for _, parameter := range digest.Parameters {
		if strings.EqualFold(strings.TrimSpace(parameter.ID), strings.TrimSpace(id)) {
			return parameter
		}
	}
	return pluginParameterInfo{}
}

func equalizerParameterEnabled(parameter pluginParameterInfo) bool {
	if value, ok := equalizerNumber(parameter.NormalizedValue); ok {
		return value >= 0.5
	}
	if value, ok := equalizerNumber(parameter.Value); ok {
		return value >= 0.5
	}
	text := strings.ToLower(strings.TrimSpace(parameter.ValueText))
	return text == "on" || text == "enabled" || text == "true"
}

func equalizerParameterNearZero(parameter pluginParameterInfo) bool {
	value, ok := equalizerDisplayNumber(parameter)
	return ok && math.Abs(value) <= 0.05
}

func equalizerDisplayNumber(parameter pluginParameterInfo) (float64, bool) {
	for _, candidate := range []any{parameter.ValueText, parameter.Value} {
		text := strings.TrimSpace(fmt.Sprint(candidate))
		if text == "" || text == "<nil>" {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) == 0 {
			continue
		}
		if value, err := strconv.ParseFloat(strings.TrimPrefix(fields[0], "+"), 64); err == nil {
			if len(fields) > 1 && strings.EqualFold(strings.TrimSpace(fields[1]), "khz") {
				value *= 1000
			}
			return value, true
		}
	}
	return 0, false
}

func equalizerEnumLabel(parameter pluginParameterInfo, values map[string]float64) string {
	normalized, ok := equalizerNumber(parameter.NormalizedValue)
	if !ok {
		normalized, ok = equalizerNumber(parameter.Value)
	}
	if ok {
		best := ""
		distance := math.MaxFloat64
		for label, value := range values {
			if current := math.Abs(value - normalized); current < distance {
				best, distance = label, current
			}
		}
		if best != "" && distance <= 0.01 {
			return best
		}
	}
	return strings.ToLower(strings.TrimSpace(parameter.ValueText))
}

func equalizerCapabilityGapReply(plan equalizerCapabilityPlan, payload map[string]any) string {
	if cleanContextText(payload["status"]) == "needs_agent_resource_choice" {
		if ref := cleanContextText(payload["recommended_band_ref"]); ref != "" {
			return "equalizer 能力合同已验证语义字段；请结合当前 Band 状态再次推理，并明确选择技术资源 " + ref + " 后重新提交。"
		}
		return "equalizer 能力合同已验证语义字段，但所有 Band 看起来都在使用；请先询问用户允许复用哪个 Band。"
	}
	missing := append(append([]string(nil), plan.MissingSemantic...), plan.MissingTechnical...)
	return "equalizer 能力申请还不能形成 SPAL 指令；请根据能力合同继续推理或向用户澄清：" + strings.Join(missing, "、") + "。"
}

func cloneFloatMap(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
