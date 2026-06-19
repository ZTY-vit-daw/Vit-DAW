package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/policy"
	agentruntime "vit-daw-agent/internal/runtime"
)

type mixResolverDecision struct {
	SchemaVersion   string         `json:"schema_version,omitempty"`
	Status          string         `json:"status,omitempty"`
	ActionKind      string         `json:"action_kind,omitempty"`
	ProcessorType   string         `json:"processor_type,omitempty"`
	TargetRef       string         `json:"target_ref,omitempty"`
	ToolRoute       []string       `json:"tool_route,omitempty"`
	Needs           []string       `json:"needs,omitempty"`
	PrepSteps       []string       `json:"prep_steps,omitempty"`
	PreparationPlan map[string]any `json:"preparation_plan,omitempty"`
	Reason          string         `json:"reason,omitempty"`
	Pending         map[string]any `json:"pending,omitempty"`
	Command         map[string]any `json:"command,omitempty"`
	PrepCommand     map[string]any `json:"prep_command,omitempty"`
}

func (s *Server) handlePendingMixTreatmentChat(ctx context.Context, conversationID string, req ChatRequest, mode string) (ChatResponse, bool) {
	treatment, ok := s.pendingMixTreatmentForConversation(conversationID)
	if !ok {
		return ChatResponse{}, false
	}
	switch {
	case messageKeepsMixTickDiscussion(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.treatment.pending] discussion continued without resolution conversation=%s message=%q action=%s processor=%s target=%s", conversationID, req.Message, treatment.ActionKind, treatment.ProcessorType, treatment.TargetRef)
		}
		return ChatResponse{}, false
	case messageClearlyShiftsMixTickContext(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.treatment.pending] expired on context shift conversation=%s message=%q action=%s processor=%s target=%s", conversationID, req.Message, treatment.ActionKind, treatment.ProcessorType, treatment.TargetRef)
		}
		s.expirePendingMixTreatment(conversationID)
		return ChatResponse{}, false
	case messageExplicitMixTickApply(req.Message):
		decision := s.resolveMixTreatment(ctx, treatment, req.Context)
		s.expirePendingMixTreatment(conversationID)
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.treatment.pending] resolver decision conversation=%s status=%s action=%s processor=%s target=%s",
				conversationID, decision.Status, decision.ActionKind, decision.ProcessorType, decision.TargetRef)
		}
		s.emitAgentEvent(conversationID, AgentEvent{
			Type:     "mix_treatment.resolver_decision",
			ItemType: "mix_treatment",
			Status:   decision.Status,
			Title:    "Mix treatment resolver decision",
			Body:     decision.Reason,
			Payload: map[string]any{
				"schema_version":   decision.SchemaVersion,
				"status":           decision.Status,
				"action_kind":      decision.ActionKind,
				"processor_type":   decision.ProcessorType,
				"target_ref":       decision.TargetRef,
				"tool_route":       decision.ToolRoute,
				"needs":            decision.Needs,
				"prep_steps":       decision.PrepSteps,
				"preparation_plan": decision.PreparationPlan,
				"pending":          decision.Pending,
				"command":          decision.Command,
				"prep_command":     decision.PrepCommand,
			},
		})
		if decision.Status == "ready_gain_tick" {
			return s.executeResolvedMixTreatmentMixTick(ctx, conversationID, req, mode, treatment, decision), true
		}
		if decision.Status == "ready_pan_tick" {
			return s.executeResolvedMixTreatmentMixTick(ctx, conversationID, req, mode, treatment, decision), true
		}
		if decision.Status == "ready_plugin_control" {
			return s.executeResolvedMixTreatment(ctx, conversationID, req, mode, treatment, decision), true
		}
		if decision.Status == "needs_preparation" && len(decision.PrepCommand) > 0 {
			return s.startResolvedMixTreatmentPreparation(ctx, conversationID, req, mode, treatment, decision), true
		}
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          mixTreatmentResolverReply(decision),
			GoalStatus:     "completed",
			StopReason:     "mix_treatment_resolved_" + decision.Status,
		}, true
	case messageAmbiguousMixTickApproval(req.Message):
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          "I have not executed anything yet. The pending item is a mix treatment direction, not an immediate parameter write. Say \"execute it\" or \"continue\" to proceed; otherwise I will keep discussing without changing the DAW.",
			GoalStatus:     "completed",
			StopReason:     "ambiguous_mix_treatment_confirmation",
		}, true
	default:
		s.expirePendingMixTreatment(conversationID)
		return ChatResponse{}, false
	}
}

func (s *Server) pendingMixTreatmentForConversation(conversationID string) (agentloop.MixTreatmentPending, bool) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return agentloop.MixTreatmentPending{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	treatment, ok := s.pendingTreatments[conversationID]
	if !ok || !strings.EqualFold(strings.TrimSpace(treatment.Status), "pending_confirmation") {
		return agentloop.MixTreatmentPending{}, false
	}
	return treatment, true
}

func (s *Server) expirePendingMixTreatment(conversationID string) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pendingTreatments, conversationID)
}

func (s *Server) resolveMixTreatment(ctx context.Context, treatment agentloop.MixTreatmentPending, requestContext map[string]any) mixResolverDecision {
	decision := mixResolverDecision{
		SchemaVersion: "mix_resolver_decision.v0",
		Status:        "needs_preparation",
		ActionKind:    strings.TrimSpace(treatment.ActionKind),
		ProcessorType: strings.TrimSpace(treatment.ProcessorType),
		TargetRef:     strings.TrimSpace(treatment.TargetRef),
		Needs:         append([]string(nil), treatment.NeedsResolution...),
		Reason:        "resolver must verify executability before this treatment can run.",
	}
	if decision.ActionKind == "" {
		decision.ActionKind = "observation_only"
	}
	if decision.ProcessorType == "" {
		decision.ProcessorType = "unknown"
	}
	if decision.TargetRef == "" {
		decision.TargetRef = "unknown"
	}
	if targetNeedsClarification(decision.TargetRef, decision.Needs) {
		decision.Status = "needs_clarification"
		decision.Reason = "Target track is still unclear; confirm the treatment target first."
		decision.Needs = appendUniqueStrings(decision.Needs, "target_track")
		return decision
	}
	switch strings.ToLower(decision.ActionKind) {
	case "gain_balance":
		return s.resolveGainBalanceTreatment(treatment, decision)
	case "pan_balance":
		return s.resolvePanBalanceTreatment(treatment, decision)
	case "plugin_treatment":
		if ready, ok := s.resolveReadyPluginTreatment(ctx, treatment, decision, requestContext); ok {
			return ready
		}
		return s.resolvePluginTreatmentPreparation(ctx, treatment, decision, requestContext)
	case "ask_clarification":
		decision.Status = "needs_clarification"
		decision.Reason = "This pending treatment asks for clarification first."
		return decision
	case "observation_only":
		decision.Status = "observation_only"
		decision.Reason = "This pending treatment is observation-only and will not mutate the DAW."
		return decision
	default:
		decision.Status = "needs_preparation"
		decision.Reason = fmt.Sprintf("unknown treatment action %q; resolver v0 will not execute it.", decision.ActionKind)
		return decision
	}
}

func (s *Server) resolvePluginTreatmentPreparation(ctx context.Context, treatment agentloop.MixTreatmentPending, base mixResolverDecision, requestContext map[string]any) mixResolverDecision {
	decision := base
	decision.Status = "needs_preparation"
	decision.ToolRoute = []string{"plugin_grabber.load_and_get_params", "plugin_grabber.learn_project_profile", "plugin_grabber.apply_control"}
	decision.Needs = appendUniqueStrings(decision.Needs, "plugin_instance", "plugin_profile", "exact_control")
	decision.PrepSteps = []string{
		"choose or load a suitable plugin instance",
		"read plugin parameters",
		"learn or confirm a plugin grabber profile",
		"resolve the treatment into a plugin_grabber.apply_control command",
	}
	decision.Reason = "plugin treatment requires preparation before any plugin write; resolver will not fall back to raw plugin.set_parameter or daw.invoke."
	decision.PreparationPlan = mixTreatmentPreparationPlan(treatment, decision, nil)

	trackID := trackIDFromTreatmentTarget(treatment.TargetRef)
	if trackID == "" {
		decision.Needs = appendUniqueStrings(decision.Needs, "target_track")
		decision.PreparationPlan = mixTreatmentPreparationPlan(treatment, decision, nil)
		return decision
	}
	plugin := s.resolveTreatmentPluginInstance(ctx, trackID, treatment, requestContext)
	if len(plugin) == 0 {
		decision.PreparationPlan = mixTreatmentPreparationPlan(treatment, decision, nil)
		return decision
	}
	pluginID := firstNonEmpty(cleanContextText(plugin["plugin_id"]), cleanContextText(plugin["plugin_item_id"]), cleanContextText(plugin["id"]), strings.TrimSpace(treatment.PluginID))
	if pluginID == "" {
		decision.PreparationPlan = mixTreatmentPreparationPlan(treatment, decision, plugin)
		return decision
	}
	pluginName := firstNonEmpty(cleanContextText(plugin["plugin_name"]), cleanContextText(plugin["name"]), strings.TrimSpace(treatment.PluginName))
	decision.ToolRoute = []string{"plugin_grabber.learn_project_profile", "plugin_grabber.apply_control"}
	decision.Needs = appendUniqueStrings(withoutString(decision.Needs, "plugin_instance"), "plugin_profile", "exact_control")
	decision.PrepSteps = []string{
		"read full parameters from the existing plugin instance",
		"draft and confirm a plugin grabber profile",
		"resolve this treatment into a plugin_grabber.apply_control command",
	}
	decision.Reason = "resolver found an existing plugin instance, but profile/control are not ready; the next safe step is plugin_grabber.learn_project_profile, not a parameter write."
	decision.Pending = map[string]any{
		"track_id":    trackID,
		"plugin_id":   pluginID,
		"plugin_name": pluginName,
	}
	decision.PrepCommand = map[string]any{
		"cmd":         pluginGrabberLearnCommand,
		"track_id":    trackID,
		"plugin_id":   pluginID,
		"plugin_name": pluginName,
		"mode":        "auto_learn",
		"intent":      firstNonEmpty(strings.TrimSpace(treatment.Intent), strings.TrimSpace(treatment.ReasoningSummary)),
		"source":      "mix_treatment_resolver",
	}
	removeEmptyTreatmentValues(decision.Pending)
	removeEmptyTreatmentValues(decision.PrepCommand)
	decision.PreparationPlan = mixTreatmentPreparationPlan(treatment, decision, plugin)
	return decision
}

func mixTreatmentPreparationPlan(treatment agentloop.MixTreatmentPending, decision mixResolverDecision, plugin map[string]any) map[string]any {
	if !strings.EqualFold(strings.TrimSpace(decision.Status), "needs_preparation") {
		return nil
	}
	plan := map[string]any{
		"schema_version": "mix_treatment_preparation.v0",
		"source":         "mix_treatment_resolver",
		"action_kind":    strings.TrimSpace(decision.ActionKind),
		"processor_type": strings.TrimSpace(decision.ProcessorType),
		"target_ref":     strings.TrimSpace(decision.TargetRef),
		"intent":         strings.TrimSpace(treatment.Intent),
		"safe_route":     append([]string(nil), decision.ToolRoute...),
		"blocked_routes": []string{"daw.invoke", "plugin.set_parameter", "track.volume", "track.pan"},
		"needs":          append([]string(nil), decision.Needs...),
		"steps":          append([]string(nil), decision.PrepSteps...),
	}
	if treatment.ObservationID != "" {
		plan["observation_id"] = treatment.ObservationID
	}
	if len(treatment.EvidenceRefs) > 0 {
		plan["evidence_refs"] = append([]string(nil), treatment.EvidenceRefs...)
	}
	if len(decision.PrepCommand) > 0 {
		plan["next_command"] = cloneStringAnyMap(decision.PrepCommand)
	}
	if len(decision.Pending) > 0 {
		plan["pending"] = cloneStringAnyMap(decision.Pending)
	}
	if len(plugin) > 0 {
		candidate := map[string]any{
			"track_id":     firstNonEmpty(cleanContextText(plugin["track_id"]), trackIDFromTreatmentTarget(treatment.TargetRef)),
			"plugin_id":    firstNonEmpty(cleanContextText(plugin["plugin_id"]), cleanContextText(plugin["plugin_item_id"]), cleanContextText(plugin["id"]), strings.TrimSpace(treatment.PluginID)),
			"plugin_name":  firstNonEmpty(cleanContextText(plugin["plugin_name"]), cleanContextText(plugin["name"]), strings.TrimSpace(treatment.PluginName)),
			"profile_key":  firstNonEmpty(cleanContextText(plugin["profile_key"]), cleanContextText(plugin["plugin_profile_key"])),
			"manufacturer": cleanContextText(plugin["manufacturer"]),
		}
		removeEmptyTreatmentValues(candidate)
		if len(candidate) > 0 {
			plan["plugin_candidate"] = candidate
		}
	}
	removeEmptyTreatmentValues(plan)
	return plan
}

func (s *Server) resolveReadyPluginTreatment(ctx context.Context, treatment agentloop.MixTreatmentPending, base mixResolverDecision, requestContext map[string]any) (mixResolverDecision, bool) {
	trackID := trackIDFromTreatmentTarget(treatment.TargetRef)
	if trackID == "" {
		return mixResolverDecision{}, false
	}
	control := strings.TrimSpace(treatment.Control)
	target := cloneContext(treatment.Target)
	if control == "" || len(target) == 0 {
		return mixResolverDecision{}, false
	}
	plugin, profile, ok := s.resolvePluginTreatmentRuntime(ctx, trackID, treatment, requestContext)
	if !ok {
		return mixResolverDecision{}, false
	}
	pluginID := firstNonEmpty(cleanContextText(plugin["plugin_id"]), cleanContextText(plugin["plugin_item_id"]), cleanContextText(plugin["id"]), strings.TrimSpace(treatment.PluginID))
	if pluginID == "" {
		return mixResolverDecision{}, false
	}
	if !profileAllowsVirtualControl(profile, control) {
		return mixResolverDecision{}, false
	}
	command := map[string]any{
		"cmd":       "plugin_grabber_apply_control",
		"track_id":  trackID,
		"plugin_id": pluginID,
		"control":   control,
		"target":    target,
		"intent":    strings.TrimSpace(treatment.Intent),
	}
	if pluginName := firstNonEmpty(cleanContextText(plugin["plugin_name"]), cleanContextText(plugin["name"]), strings.TrimSpace(treatment.PluginName)); pluginName != "" {
		command["plugin_name"] = pluginName
	}
	decision := base
	decision.Status = "ready_plugin_control"
	decision.ToolRoute = []string{"plugin_grabber.apply_control"}
	decision.Needs = nil
	decision.PrepSteps = nil
	decision.Reason = "resolver found explicit target track, existing plugin instance, usable plugin grabber profile, and exact control; confirmation will use only plugin_grabber.apply_control."
	decision.Command = command
	decision.Pending = map[string]any{
		"track_id":  trackID,
		"plugin_id": pluginID,
		"control":   control,
		"target":    target,
	}
	return decision, true
}

func (s *Server) resolvePluginTreatmentRuntime(ctx context.Context, trackID string, treatment agentloop.MixTreatmentPending, requestContext map[string]any) (map[string]any, map[string]any, bool) {
	plugin := s.resolveTreatmentPluginInstance(ctx, trackID, treatment, requestContext)
	if len(plugin) == 0 {
		return nil, nil, false
	}
	profile := s.resolveTreatmentPluginProfile(ctx, plugin, treatment, requestContext)
	if len(profile) == 0 {
		return nil, nil, false
	}
	return plugin, profile, true
}

func (s *Server) resolveTreatmentPluginInstance(ctx context.Context, trackID string, treatment agentloop.MixTreatmentPending, requestContext map[string]any) map[string]any {
	wantedID := strings.TrimSpace(treatment.PluginID)
	wantedName := strings.TrimSpace(treatment.PluginName)
	selectedTrackID := firstNonEmpty(cleanContextText(requestContext["selected_plugin_track_id"]), cleanContextText(requestContext["selected_track_id"]))
	selectedID := cleanContextText(requestContext["selected_plugin_id"])
	if selectedID != "" && selectedTrackID == trackID && (wantedID == "" || selectedID == wantedID) {
		selected := map[string]any{
			"track_id":     trackID,
			"plugin_id":    selectedID,
			"plugin_name":  cleanContextText(requestContext["selected_plugin_name"]),
			"profile_key":  cleanContextText(requestContext["selected_plugin_profile_key"]),
			"plugin_path":  cleanContextText(requestContext["selected_plugin_path"]),
			"manufacturer": cleanContextText(requestContext["selected_plugin_manufacturer"]),
		}
		removeEmptyTreatmentValues(selected)
		return selected
	}
	plugins := treatmentVisiblePlugins(s.visibleStateForTreatment(ctx), trackID)
	for _, plugin := range plugins {
		id := firstNonEmpty(cleanContextText(plugin["plugin_id"]), cleanContextText(plugin["plugin_item_id"]), cleanContextText(plugin["id"]))
		name := firstNonEmpty(cleanContextText(plugin["plugin_name"]), cleanContextText(plugin["name"]))
		if wantedID != "" && id != wantedID {
			continue
		}
		if wantedID == "" && wantedName != "" && !strings.EqualFold(name, wantedName) {
			continue
		}
		if wantedID == "" && wantedName == "" && len(plugins) != 1 {
			continue
		}
		if plugin["track_id"] == nil {
			plugin["track_id"] = trackID
		}
		return plugin
	}
	return nil
}

func (s *Server) resolveTreatmentPluginProfile(ctx context.Context, plugin map[string]any, treatment agentloop.MixTreatmentPending, requestContext map[string]any) map[string]any {
	profiles := treatmentProfileRows(requestContext["plugin_grabber_profiles"])
	profiles = append(profiles, treatmentProfileRows(requestContext["profiles"])...)
	if stateProfiles := treatmentProfileRows(s.visibleStateForTreatment(ctx)["plugin_grabber_profiles"]); len(stateProfiles) > 0 {
		profiles = append(profiles, stateProfiles...)
	}
	if s != nil && s.harness != nil {
		resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
			Tool:      "plugin_grabber.get_project_profiles",
			Args:      map[string]any{},
			Context:   requestContext,
			Source:    "mix_treatment_resolver",
			Confirmed: true,
		})
		if err == nil && resp.Status == "ok" {
			profiles = append(profiles, treatmentProfileRows(resp.Result["plugin_grabber_profiles"])...)
			profiles = append(profiles, treatmentProfileRows(resp.Result["profiles"])...)
		}
	}
	for _, profile := range profiles {
		if treatmentProfileMatchesPlugin(profile, plugin, treatment) {
			return profile
		}
	}
	return nil
}

func (s *Server) visibleStateForTreatment(ctx context.Context) map[string]any {
	if s == nil {
		return nil
	}
	if s.harness != nil {
		return s.harness.UserStateSummary(ctx)
	}
	if s.shadow != nil {
		return s.shadow.Summary()
	}
	return nil
}

func treatmentVisiblePlugins(state map[string]any, trackID string) []map[string]any {
	var out []map[string]any
	for _, track := range mapRowsValue(state["tracks"]) {
		id := firstNonEmpty(cleanContextText(track["track_id"]), cleanContextText(track["id"]))
		if id != trackID {
			continue
		}
		for _, plugin := range mapRowsValue(track["plugins"]) {
			row := copyStringAnyMap(plugin)
			row["track_id"] = trackID
			out = append(out, row)
		}
	}
	return out
}

func treatmentProfileRows(value any) []map[string]any {
	rows := mapRowsValue(value)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	return out
}

func treatmentProfileMatchesPlugin(profile, plugin map[string]any, treatment agentloop.MixTreatmentPending) bool {
	if len(profile) == 0 || len(plugin) == 0 {
		return false
	}
	identity := mapValue(profile["plugin_identity"])
	pluginID := firstNonEmpty(cleanContextText(plugin["plugin_id"]), cleanContextText(plugin["plugin_item_id"]), cleanContextText(plugin["id"]))
	profilePluginID := firstNonEmpty(cleanContextText(profile["plugin_id"]), cleanContextText(identity["plugin_id"]))
	if profilePluginID != "" && pluginID != "" && profilePluginID == pluginID {
		return true
	}
	wantedName := firstNonEmpty(strings.TrimSpace(treatment.PluginName), cleanContextText(plugin["plugin_name"]), cleanContextText(plugin["name"]))
	profileName := firstNonEmpty(cleanContextText(profile["plugin_name"]), cleanContextText(identity["plugin_name"]), cleanContextText(identity["name"]))
	if wantedName != "" && profileName != "" && strings.EqualFold(wantedName, profileName) {
		return true
	}
	profileKey := firstNonEmpty(cleanContextText(profile["profile_key"]), cleanContextText(profile["profile_id"]), cleanContextText(identity["profile_key"]))
	pluginProfileKey := firstNonEmpty(cleanContextText(plugin["profile_key"]), cleanContextText(plugin["plugin_profile_key"]))
	return profileKey != "" && pluginProfileKey != "" && profileKey == pluginProfileKey
}

func profileAllowsVirtualControl(profile map[string]any, control string) bool {
	control = strings.TrimSpace(control)
	if control == "" {
		return false
	}
	if controlRowsContainName(mapRowsValue(profile["virtual_controls"]), control) {
		return true
	}
	if controlRowsContainName(mapRowsValue(mapValue(profile["plugin_skill"])["operations"]), control) {
		return true
	}
	if legacy := mapValue(profile["legacy"]); len(legacy) > 0 {
		if patch := mapValue(legacy["profile_patch"]); controlRowsContainName(mapRowsValue(patch["virtual_controls"]), control) {
			return true
		}
	}
	return false
}

func controlRowsContainName(rows []map[string]any, control string) bool {
	for _, row := range rows {
		name := firstNonEmpty(cleanContextText(row["name"]), cleanContextText(row["control"]), cleanContextText(row["id"]))
		if strings.EqualFold(name, control) {
			return true
		}
	}
	return false
}

func removeEmptyTreatmentValues(row map[string]any) {
	for key, value := range row {
		if strings.TrimSpace(fmt.Sprint(value)) == "" || strings.TrimSpace(fmt.Sprint(value)) == "<nil>" {
			delete(row, key)
		}
	}
}

func trackIDFromTreatmentTarget(targetRef string) string {
	targetRef = strings.TrimSpace(targetRef)
	lower := strings.ToLower(targetRef)
	if strings.HasPrefix(lower, "track:") {
		return strings.TrimSpace(targetRef[len("track:"):])
	}
	if strings.HasPrefix(lower, "track_id:") {
		return strings.TrimSpace(targetRef[len("track_id:"):])
	}
	return ""
}

func (s *Server) resolveGainBalanceTreatment(treatment agentloop.MixTreatmentPending, base mixResolverDecision) mixResolverDecision {
	decision := base
	decision.ToolRoute = []string{"mix.propose_tick", "mix.apply_tick", "mix.observe"}
	trackID := trackIDFromTreatmentTarget(treatment.TargetRef)
	deltaDB := treatmentDeltaDB(treatment)
	if trackID == "" || deltaDB == 0 || math.Abs(deltaDB) > 2 {
		decision.Status = "needs_preparation"
		decision.Needs = appendUniqueStrings(decision.Needs, "exact_control")
		decision.Reason = "gain_balance can use the existing gain tick route, but it still needs one explicit nonzero delta within +/-2 dB."
		return decision
	}
	decision.Status = "ready_gain_tick"
	decision.Needs = nil
	decision.PrepSteps = nil
	decision.Reason = "gain_balance has an explicit track target and small delta, so confirmation can execute through the existing gain tick route."
	decision.Pending = map[string]any{
		"operation": "track_gain_adjust",
		"track_id":  trackID,
		"delta_db":  deltaDB,
	}
	return decision
}

func (s *Server) resolvePanBalanceTreatment(treatment agentloop.MixTreatmentPending, base mixResolverDecision) mixResolverDecision {
	decision := base
	decision.ToolRoute = []string{"mix.propose_tick", "mix.apply_tick", "mix.observe"}
	trackID := trackIDFromTreatmentTarget(treatment.TargetRef)
	deltaPan := treatmentDeltaPan(treatment)
	targetPan := treatmentTargetPan(treatment)
	if trackID == "" {
		decision.Status = "needs_preparation"
		decision.Needs = appendUniqueStrings(decision.Needs, "target_track")
		decision.Reason = "pan_balance can use the typed pan tick route, but it needs one clear target track."
		return decision
	}
	if targetPan != nil {
		if *targetPan < -1 || *targetPan > 1 {
			decision.Status = "needs_preparation"
			decision.Needs = appendUniqueStrings(decision.Needs, "exact_control")
			decision.Reason = "pan_balance target pan must be within -1.0 left to +1.0 right."
			return decision
		}
		decision.Status = "ready_pan_tick"
		decision.Needs = nil
		decision.PrepSteps = nil
		decision.Reason = "pan_balance has an explicit track target and target pan, so confirmation can execute through the typed pan tick route."
		decision.Pending = map[string]any{
			"operation":  "track_pan_set",
			"track_id":   trackID,
			"target_pan": *targetPan,
		}
		return decision
	}
	if deltaPan == 0 || math.Abs(deltaPan) > 0.15 {
		decision.Status = "needs_preparation"
		decision.Needs = appendUniqueStrings(decision.Needs, "exact_control")
		decision.Reason = "pan_balance can use the typed pan tick route, but it still needs one explicit nonzero pan delta within +/-0.15 or a target pan within -1.0..+1.0."
		return decision
	}
	decision.Status = "ready_pan_tick"
	decision.Needs = nil
	decision.PrepSteps = nil
	decision.Reason = "pan_balance has an explicit track target and small pan delta, so confirmation can execute through the typed pan tick route."
	decision.Pending = map[string]any{
		"operation": "track_pan_adjust",
		"track_id":  trackID,
		"delta_pan": deltaPan,
	}
	return decision
}

func (s *Server) executeResolvedMixTreatmentMixTick(ctx context.Context, conversationID string, req ChatRequest, mode string, treatment agentloop.MixTreatmentPending, decision mixResolverDecision) ChatResponse {
	trackID := cleanContextText(decision.Pending["track_id"])
	operation := firstNonEmpty(cleanContextText(decision.Pending["operation"]), operationFromMixTreatmentDecision(treatment, decision))
	candidate := agentloop.PendingMixTickCandidate{
		Operation:                 operation,
		TrackID:                   trackID,
		ObservationID:             treatment.ObservationID,
		Evidence:                  map[string]any{"source": "mix_treatment_resolver", "intent": treatment.Intent, "action_kind": treatment.ActionKind},
		CreatedFromReply:          treatment.CreatedFromReply,
		ExpiresAfterContextChange: treatment.ExpiresAfterContextChange,
		Status:                    "pending_confirmation",
		Fingerprint:               cloneContext(treatment.Fingerprint),
	}
	switch operation {
	case "track_pan_adjust":
		candidate.DeltaPan = treatmentDeltaPan(treatment)
		if candidate.DeltaPan == 0 {
			if value, ok := treatmentNumber(decision.Pending, "delta_pan", "pan_delta"); ok {
				candidate.DeltaPan = value
			}
		}
	case "track_pan_set":
		candidate.TargetPan = treatmentTargetPan(treatment)
		if candidate.TargetPan == nil {
			if value, ok := treatmentNumber(decision.Pending, "target_pan", "pan", "pan_value"); ok {
				candidate.TargetPan = &value
			}
		}
	default:
		candidate.Operation = "track_gain_adjust"
		candidate.DeltaDB = treatmentDeltaDB(treatment)
		if candidate.DeltaDB == 0 {
			if value, ok := treatmentNumber(decision.Pending, "delta_db"); ok {
				candidate.DeltaDB = value
			}
		}
	}
	if candidate.Fingerprint == nil {
		candidate.Fingerprint = map[string]any{}
	}
	if candidate.Fingerprint["target_track_id"] == nil {
		candidate.Fingerprint["target_track_id"] = trackID
	}
	if candidate.Fingerprint["observation_id"] == nil {
		candidate.Fingerprint["observation_id"] = treatment.ObservationID
	}
	return s.executePendingMixTickCandidate(ctx, conversationID, req, mode, candidate)
}

func operationFromMixTreatmentDecision(treatment agentloop.MixTreatmentPending, decision mixResolverDecision) string {
	if strings.EqualFold(strings.TrimSpace(decision.Status), "ready_pan_tick") || strings.EqualFold(strings.TrimSpace(treatment.ActionKind), "pan_balance") {
		if treatmentTargetPan(treatment) != nil {
			return "track_pan_set"
		}
		return "track_pan_adjust"
	}
	return "track_gain_adjust"
}

func (s *Server) startResolvedMixTreatmentPreparation(ctx context.Context, conversationID string, req ChatRequest, mode string, treatment agentloop.MixTreatmentPending, decision mixResolverDecision) ChatResponse {
	prepCommand := cloneStringAnyMap(decision.PrepCommand)
	if cleanContextText(prepCommand["cmd"]) != pluginGrabberLearnCommand {
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          mixTreatmentResolverReply(decision),
			GoalStatus:     string(agentruntime.StatusCompleted),
			StopReason:     "mix_treatment_resolved_" + decision.Status,
		}
	}
	prepContext := mergeContext(req.Context, map[string]any{
		"mix_treatment_preparation":      true,
		"mix_treatment_preparation_plan": cloneContext(decision.PreparationPlan),
		"mix_treatment_intent":           treatment.Intent,
		"mix_treatment_action_kind":      treatment.ActionKind,
		"mix_treatment_processor":        treatment.ProcessorType,
		"mix_treatment_target_ref":       treatment.TargetRef,
	})
	resp := s.runPluginGrabberLearningWorkflow(ctx, conversationID, firstNonEmpty(strings.TrimSpace(treatment.Intent), strings.TrimSpace(req.Message)), prepContext, config.EngineConfig{}, prepCommand)
	resp.AgentMode = firstNonEmpty(resp.AgentMode, mode)
	resp.StopReason = firstNonEmpty(resp.StopReason, "mix_treatment_preparation_started")
	if strings.TrimSpace(resp.GoalStatus) == "" {
		resp.GoalStatus = string(agentruntime.StatusCompleted)
	}
	resolverReply := "No plugin parameter write has run. Resolver found an existing plugin instance; the next step only opens the Plugin Grabber learning/confirmation flow to prepare a profile and executable control."
	if len(decision.PrepCommand) == 0 {
		resolverReply = strings.TrimSpace(mixTreatmentResolverReply(decision))
	}
	if strings.TrimSpace(resp.Reply) == "" {
		resp.Reply = resolverReply
	} else if resolverReply != "" {
		resp.Reply = resolverReply + "\n\n" + resp.Reply
	}
	s.attachInteractionRequests(&resp)
	return resp
}

func treatmentDeltaDB(treatment agentloop.MixTreatmentPending) float64 {
	if treatment.DeltaDB != 0 {
		return treatment.DeltaDB
	}
	if value, ok := treatmentNumber(treatment.Target, "delta_db", "db_delta", "gain_delta_db", "volume_delta_db"); ok {
		return value
	}
	return 0
}

func treatmentDeltaPan(treatment agentloop.MixTreatmentPending) float64 {
	if treatment.DeltaPan != 0 {
		return treatment.DeltaPan
	}
	if value, ok := treatmentNumber(treatment.Target, "delta_pan", "pan_delta"); ok {
		return value
	}
	return 0
}

func treatmentTargetPan(treatment agentloop.MixTreatmentPending) *float64 {
	if treatment.TargetPan != nil {
		value := *treatment.TargetPan
		return &value
	}
	if value, ok := treatmentNumber(treatment.Target, "target_pan", "pan", "pan_value"); ok {
		return &value
	}
	return nil
}

func treatmentNumber(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if row == nil {
			return 0, false
		}
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed, true
		case float32:
			return float64(typed), true
		case int:
			return float64(typed), true
		case int64:
			return float64(typed), true
		case json.Number:
			if n, err := typed.Float64(); err == nil {
				return n, true
			}
		case string:
			if n, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

func (s *Server) executeResolvedMixTreatment(ctx context.Context, conversationID string, req ChatRequest, mode string, treatment agentloop.MixTreatmentPending, decision mixResolverDecision) ChatResponse {
	if len(decision.Command) == 0 {
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          mixTreatmentResolverReply(decision),
			GoalStatus:     string(agentruntime.StatusCompleted),
			StopReason:     "mix_treatment_resolved_" + decision.Status,
		}
	}
	projectPath := projectPathFromChatContext(req.Context)
	goal := s.beginChatGoal(conversationID, req.Message, req.Context)
	toolContext := s.agentLoopToolContext(mode, req.Message, req.Context)
	exec := executor.New(s.harness)
	execContext := mergeContext(req.Context, map[string]any{
		"mix_treatment_confirmation": true,
		"mix_treatment_intent":       treatment.Intent,
		"user_message":               req.Message,
		"original_user_message":      treatment.Intent,
	})
	var executed []map[string]any
	runTool := func(call planner.ToolCall, confirmed bool, source string, callContext map[string]any) (executor.Result, error) {
		out, err := exec.RunToolCall(ctx, executor.Input{
			GoalID:    goal.GoalID,
			RunID:     goal.RunID,
			ToolCall:  call,
			Context:   callContext,
			Confirmed: confirmed,
			Source:    source,
		})
		executed = append(executed, agentLoopExecutionRecord(out))
		return out, err
	}
	resp, err := runTool(planner.ToolCall{
		ID:      "apply_resolved_mix_treatment",
		Tool:    "plugin_grabber.apply_control",
		Command: cloneStringAnyMap(decision.Command),
		Args:    cloneStringAnyMap(decision.Command),
		Reason:  "apply the confirmed resolved mix treatment through Plugin Grabber",
	}, true, "mix_treatment_resolver", execContext)
	reply := mixTreatmentApplyReply(decision, harness.InvokeResponse{
		Status:        resp.Status,
		AgentActionID: resp.AgentActionID,
		Tool:          resp.Tool,
		CommandName:   resp.CommandName,
		RiskLevel:     resp.Response.RiskLevel,
		UndoLabel:     resp.UndoLabel,
		Result:        resp.Result,
		Error:         resp.Error,
	}, err)
	status := "completed"
	stopReason := "mix_treatment_applied_plugin_control"
	errText := ""
	if err != nil || resultFailed(resp) {
		stopReason = "mix_treatment_plugin_control_failed"
		errText = firstNonEmpty(resp.Error, fmt.Sprint(err))
	}
	commandDecision := policy.Decision{
		Command: decision.Command,
		Name:    "plugin_grabber_apply_control",
		Risk:    policy.RiskUndoable,
		Reason:  "confirmed mix treatment resolver route",
	}
	completedSteps := 1
	if err == nil && !resultFailed(resp) {
		trackID := firstNonEmpty(cleanContextText(decision.Command["track_id"]), trackIDFromTreatmentTarget(treatment.TargetRef))
		observeArgs := map[string]any{
			"track_id":              trackID,
			"scope":                 "selected_track",
			"project_context":       true,
			"observation_only":      true,
			"disclosure":            "digest_catalog",
			"goal_text":             firstNonEmpty(strings.TrimSpace(treatment.Intent), strings.TrimSpace(req.Message)),
			"previous_observation":  treatment.ObservationID,
			"mix_session_id":        firstNonEmpty(cleanContextText(treatment.Fingerprint["mix_session_id"]), "mix_"+goal.GoalID),
			"source":                "mix_treatment_resolver",
			"post_plugin_treatment": true,
		}
		if trackID == "" {
			delete(observeArgs, "track_id")
			observeArgs["scope"] = "full_project"
		}
		observe, observeErr := runTool(planner.ToolCall{
			ID:     "reobserve_after_plugin_treatment",
			Tool:   preferredMixObservationTool(toolContext.AllowedTools),
			Args:   observeArgs,
			Reason: "re-observe after applying the confirmed plugin treatment",
		}, true, "mix_treatment_resolver_reobserve", req.Context)
		if observeErr != nil || resultFailed(observe) {
			stopReason = "mix_treatment_applied_plugin_control_reobserve_failed"
			errText = firstNonEmpty(observe.Error, fmt.Sprint(observeErr))
			reply += " Re-observe after the plugin move failed, so listen/check the project before continuing."
		} else {
			stopReason = "mix_treatment_applied_plugin_control_reobserved"
			completedSteps = 2
			if observationID := firstNonEmpty(cleanContextText(observe.Result["observation_id"]), cleanContextText(mapValue(observe.Result["observation"])["observation_id"]), cleanContextText(mapValue(observe.Result["digest"])["observation_id"])); observationID != "" {
				reply += " Re-observed after the plugin move: " + observationID + "."
			} else {
				reply += " Re-observed after the plugin move."
			}
		}
	}
	return ChatResponse{
		ConversationID:      conversationID,
		AgentMode:           mode,
		GoalID:              goal.GoalID,
		RunID:               goal.RunID,
		Reply:               reply,
		Commands:            []policy.Decision{commandDecision},
		ExecutedKernelReply: executed,
		ProjectResultCards:  projectResultCardsFromExecuted(executed),
		Artifacts:           artifactSummariesFromExecuted(executed),
		GoalStatus:          status,
		GoalSummary:         req.Message,
		CompletedSteps:      completedSteps,
		StopReason:          stopReason,
		ProjectHistory:      s.harness.ProjectHistorySummaryForProject(ctx, goal.GoalID, projectPath),
		Error:               errText,
	}
}

func targetNeedsClarification(targetRef string, needs []string) bool {
	targetRef = strings.ToLower(strings.TrimSpace(targetRef))
	if targetRef == "" || targetRef == "unknown" || targetRef == "vocal_unknown" || strings.Contains(targetRef, "unknown") {
		return true
	}
	for _, need := range needs {
		need = strings.ToLower(strings.TrimSpace(need))
		if need == "target_track" {
			return true
		}
	}
	return false
}

func appendUniqueStrings(values []string, extra ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values)+len(extra))
	for _, value := range append(values, extra...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func withoutString(values []string, remove string) []string {
	remove = strings.TrimSpace(remove)
	if remove == "" {
		return values
	}
	out := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) == remove {
			continue
		}
		out = append(out, value)
	}
	return out
}

func mixTreatmentResolverReply(decision mixResolverDecision) string {
	switch decision.Status {
	case "needs_clarification":
		return "I have not executed anything yet. This treatment needs a clear target track first: " + decision.TargetRef + "."
	case "ready_gain_tick":
		return "I have not executed anything yet. The resolver can route this through the existing gain tick path once the small dB move is explicit."
	case "ready_pan_tick":
		return "I have not executed anything yet. The resolver can route this through the typed pan tick path once the small pan move is explicit."
	case "ready_plugin_control":
		return "The resolver confirmed this step can run through the safe plugin_grabber.apply_control route."
	case "needs_preparation":
		if len(decision.PrepCommand) > 0 {
			return "No plugin parameter write has run. Resolver found an existing plugin instance, so the next step is only Plugin Grabber learning/confirmation to prepare a profile and executable control."
		}
		reply := "I have not executed anything yet. Resolver says this plugin treatment needs preparation before it can run."
		if len(decision.PrepSteps) > 0 {
			reply += " Prep steps: " + strings.Join(decision.PrepSteps, "; ") + "."
		}
		return reply
	case "observation_only":
		return "I have not executed anything yet. This pending item is observation-only and contains no executable DAW mutation."
	default:
		return "I have not executed anything yet. Resolver did not find a safe executable route."
	}
}

func mixTreatmentApplyReply(decision mixResolverDecision, resp harness.InvokeResponse, err error) string {
	if err != nil || resp.Status == "error" || resp.Status == "kernel_error" {
		detail := firstNonEmpty(resp.Error, fmt.Sprint(err))
		if strings.TrimSpace(detail) == "" || detail == "<nil>" {
			detail = "execution failed"
		}
		return "I did not complete this step. Resolver only tried the safe plugin_grabber.apply_control route, but execution failed: " + detail
	}
	control := cleanContextText(decision.Command["control"])
	if control == "" {
		control = "plugin control"
	}
	applied := mapRowsValue(resp.Result["applied_parameters"])
	if len(applied) > 0 {
		first := applied[0]
		valueText := firstNonEmpty(cleanContextText(first["new_value_text"]), cleanContextText(first["applied_value"]), cleanContextText(first["value_text"]))
		if valueText != "" {
			return "Executed: " + control + ", applied value: " + valueText + ". This step used only plugin_grabber.apply_control, not raw plugin.set_parameter or daw.invoke."
		}
	}
	return "Executed: " + control + ". This step used only plugin_grabber.apply_control, not raw plugin.set_parameter or daw.invoke."
}
