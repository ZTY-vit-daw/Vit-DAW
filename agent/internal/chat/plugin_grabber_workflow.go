package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/artifacts"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/policy"
	"vit-daw-agent/internal/promptruntime"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/tools"
	"vit-daw-agent/internal/webtools"

	daw "vit-daw-agent/internal/daw"

	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	pluginGrabberLearnCommand        = "plugin_grabber_learn_project_profile"
	pluginGrabberLearnTool           = "plugin_grabber.learn_project_profile"
	pluginGrabberLoadCommand         = "plugin_grabber_load_and_get_params"
	pluginGrabberLoadTool            = "plugin_grabber.load_and_get_params"
	pluginLearningDigestMaxBytes     = 180000
	pluginLearningInvalidIDListLimit = 10
	pluginLearningWebDigestTimeout   = 4 * time.Minute
	pluginLearningTypeTimeout        = 4 * time.Minute
	pluginLearningMatchTimeout       = 4 * time.Minute
	pluginLearningDraftTimeout       = 6 * time.Minute
	pluginLearningUIReferencePurpose = "plugin_ui_reference"
	pluginLearningUIReferenceStage   = "ui_reference_request"
	pluginLearningUIReferenceType    = "plugin_learning_ui_reference_request"
	pluginLearningSessionSchema      = "vit.plugin_learning_session.v1"
	pluginLearningWebReferenceSchema = "plugin_web_reference_digest.v2"
)

var errPluginUIReferenceNoValidImage = errors.New("plugin_ui_reference_no_valid_image")

var pluginUIReferenceBandTokenPattern = regexp.MustCompile(`^b([0-9]+)$`)
var pluginUIReferenceSecondUnitPattern = regexp.MustCompile(`(^|[^a-z])s([^a-z]|$)`)

type pluginLearningTarget = plugingrabber.LearningTarget
type pluginLoadTarget = plugingrabber.LoadTarget
type pluginLoadCandidate = plugingrabber.LoadCandidate
type pluginParameterDigest = plugingrabber.ParameterDigest
type pluginParameterInfo = plugingrabber.ParameterInfo
type pluginQuickControlDigest = plugingrabber.QuickControlDigest
type pluginRecommendedGroupInfo = plugingrabber.RecommendedGroupInfo
type pluginProfilePatch = plugingrabber.ProfilePatch
type pluginSkillValidationResult = plugingrabber.PluginSkillValidationResult

func synthesizePluginGrabberLearningCommands(userText string, requestContext map[string]any) []map[string]any {
	return plugingrabber.SynthesizeLearningCommands(userText, requestContext)
}

func synthesizePluginGrabberLoadCommands(userText string, requestContext map[string]any) []map[string]any {
	return plugingrabber.SynthesizeLoadCommands(userText, requestContext)
}

func extractPluginLoadQuery(text string) string {
	return plugingrabber.ExtractLoadQuery(text)
}

func synthesizePluginLibraryCommands(userText string) []map[string]any {
	return plugingrabber.SynthesizeLibraryCommands(userText)
}

func firstPluginGrabberLearningCommand(commands []map[string]any) (map[string]any, bool) {
	return plugingrabber.FirstLearningCommand(commands)
}

func pluginGrabberLearningInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	toolName := strings.TrimSpace(req.Tool)
	if toolName == pluginGrabberLearnTool || toolName == "plugin.learn_project_profile" || toolName == "plugin_learn_project_profile" {
		cmd := map[string]any{"cmd": pluginGrabberLearnCommand}
		for key, value := range req.Args {
			cmd[key] = value
		}
		return cmd, true
	}
	args := workflowCommandArgs(req.Command)
	name := strings.TrimSpace(fmt.Sprint(args["cmd"]))
	if name == "" || name == "<nil>" {
		name = strings.TrimSpace(fmt.Sprint(args["command"]))
	}
	if name == pluginGrabberLearnCommand {
		return args, true
	}
	return nil, false
}

func (s *Server) invokePluginGrabberLearningWorkflow(ctx context.Context, req harness.InvokeRequest, workflowCmd map[string]any, cfg config.EngineConfig) (harness.InvokeResponse, error) {
	if req.Confirmed {
		key := pluginGrabberLearningInvokePendingKey(req)
		plan, ok, stale := s.takePendingPlan(key)
		if !ok {
			if stale {
				return harness.InvokeResponse{
					Status:      "ok",
					Tool:        pluginGrabberLearnTool,
					CommandName: pluginGrabberLearnCommand,
					RiskLevel:   tools.RiskConfirm,
					Result: map[string]any{
						"message": "这个 Plugin Grabber 学习确认已处理或已过期。",
						"plan_id": key,
					},
				}, nil
			}
			out := harness.InvokeResponse{
				Status:      "error",
				Tool:        pluginGrabberLearnTool,
				CommandName: pluginGrabberLearnCommand,
				RiskLevel:   tools.RiskConfirm,
				Error:       "Plugin Grabber 学习确认缺少待保存的档案更新。",
			}
			return out, fmt.Errorf("%s", out.Error)
		}
		replies, err := s.executeDecisions(ctx, plan.Decisions, true, plan.Context)
		result := map[string]any{
			"replies": replies,
			"plan_id": plan.ID,
		}
		if data := pluginGrabberLearningCompletionData(plan, true); data != nil {
			result["workflow"] = plan.Workflow
			result["workflow_data"] = data
			result["plugin_learning"] = data
		}
		out := harness.InvokeResponse{
			Status:      "ok",
			Tool:        pluginGrabberLearnTool,
			CommandName: pluginGrabberLearnCommand,
			RiskLevel:   tools.RiskConfirm,
			Result:      result,
		}
		if err != nil {
			out.Status = "error"
			out.Error = err.Error()
			return out, err
		}
		return out, nil
	}

	intent := firstNonEmptyText(workflowCmd, "intent", "user_intent")
	resp := s.runPluginGrabberLearningWorkflow(ctx, "invoke_"+randomID(), intent, req.Context, cfg, workflowCmd)
	s.attachInteractionRequests(&resp)
	resultPayload := map[string]any{
		"reply":                resp.Reply,
		"needs_confirmation":   resp.NeedsConfirmation,
		"plan_id":              resp.PlanID,
		"preview":              resp.Preview,
		"workflow":             resp.Workflow,
		"workflow_data":        resp.WorkflowData,
		"plugin_learning":      resp.PluginLearning,
		"interaction_requests": resp.InteractionRequests,
	}
	if len(resp.Artifacts) > 0 {
		resultPayload["artifacts"] = resp.Artifacts
	}
	if resp.SidePanelRequest != nil {
		resultPayload["side_panel_request"] = resp.SidePanelRequest
	}
	out := harness.InvokeResponse{
		Status:               "ok",
		Tool:                 pluginGrabberLearnTool,
		CommandName:          pluginGrabberLearnCommand,
		RiskLevel:            tools.RiskDirect,
		RequiresConfirmation: resp.NeedsConfirmation,
		Preview:              resp.Preview,
		Result:               resultPayload,
	}
	if resp.NeedsConfirmation && strings.TrimSpace(resp.PlanID) != "" {
		s.mu.Lock()
		if plan, ok := s.pending[resp.PlanID]; ok {
			s.pending[pluginGrabberLearningInvokePendingKey(req)] = plan
		}
		s.mu.Unlock()
		out.RiskLevel = tools.RiskConfirm
	}
	if resp.Error != "" {
		out.Status = "error"
		out.Error = resp.Error
		return out, fmt.Errorf("%s", resp.Error)
	}
	return out, nil
}

func pluginGrabberLearningInvokePendingKey(req harness.InvokeRequest) string {
	parts := []string{
		"plugin_grabber_learning",
		strings.TrimSpace(req.GoalID),
		strings.TrimSpace(req.RunID),
		strings.TrimSpace(req.ToolCallID),
	}
	if parts[1] == "" && parts[2] == "" && parts[3] == "" {
		parts = append(parts, strings.TrimSpace(req.Source), strings.TrimSpace(req.Tool))
	}
	return strings.Join(parts, ":")
}

func firstPluginGrabberLoadCommand(commands []map[string]any) (map[string]any, bool) {
	return plugingrabber.FirstLoadCommand(commands)
}

func coercePluginGrabberLoadCommand(commands []map[string]any, userText string, requestContext map[string]any) (map[string]any, bool) {
	return plugingrabber.CoerceLoadCommand(commands, userText, requestContext)
}

func looksLikePluginGrabberLoadIntent(userText string) bool {
	return plugingrabber.LooksLikeLoadIntent(userText)
}

func copyWorkflowField(dst map[string]any, src map[string]any, target string, keys ...string) {
	plugingrabber.CopyWorkflowField(dst, src, target, keys...)
}
func (s *Server) runPluginGrabberLoadWorkflow(ctx context.Context, conversationID, userText string, requestContext map[string]any, workflowCmd map[string]any) ChatResponse {
	if !boolValue(requestContext["mix_treatment_preparation"]) && legacyChatBroadMixRequestNeedsObservation(userText) && !legacyChatExplicitPluginOrRawRequest(userText) {
		reply := "我不会因为宽泛的混音目标直接加载效果器。先做一次 mix.request_observation，基于当前轨道/音频的实际观察给出建议；你确认一个具体小动作后，我再执行可撤回的单步调整。"
		return ChatResponse{
			ConversationID: conversationID,
			Reply:          reply,
			GoalStatus:     string(agentruntime.StatusCompleted),
		}
	}
	target, err := s.resolvePluginLoadTarget(ctx, workflowCmd, requestContext, userText)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}
	if s.kernel == nil && s.harness == nil {
		err := fmt.Errorf("kernel client is nil")
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}

	candidate := pluginLoadCandidate{Name: target.PluginName, Path: target.PluginPath}
	if target.PluginPath == "" {
		candidates, err := s.searchPluginLoadCandidates(ctx, target.PluginQuery, strings.TrimSpace(target.Intent+" "+target.IntentKind))
		if err != nil {
			return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
		}
		if len(candidates) == 0 {
			err := fmt.Errorf("no indexed plugin matched %q; run plugin.scan first if the plugin has not been scanned", target.PluginQuery)
			return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
		}
		candidate = candidates[0]
		target.PluginPath = candidate.Path
		target.PluginName = candidate.Name
	}
	if strings.TrimSpace(target.PluginPath) == "" {
		err := fmt.Errorf("plugin search matched %q but did not return a loadable plugin_path", target.PluginQuery)
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}

	zone := "Z3"
	if candidate.IsInstrument {
		zone = "Z2"
	}
	pluginKind := "Effect"
	if candidate.IsInstrument {
		pluginKind = "Instrument"
	}
	loadCommand := map[string]any{
		"cmd":         "rack_add_node",
		"track_id":    target.TrackID,
		"plugin_path": target.PluginPath,
		"plugin_name": target.PluginName,
		"plugin_kind": pluginKind,
		"x":           360.0,
		"y":           260.0,
		"zone_id":     zone,
	}
	decisions := policy.Analyze([]map[string]any{loadCommand})
	preview, err := s.confirmationPreview(ctx, decisions, requestContext)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error(), Commands: decisions}
	}
	plan := PendingPlan{
		ID:        "plan_" + randomID(),
		CreatedAt: time.Now(),
		Decisions: decisions,
		Context:   requestContext,
		Preview:   preview,
		Workflow:  pluginGrabberLoadCommand,
		WorkflowData: map[string]any{
			"track_id":     target.TrackID,
			"plugin_query": target.PluginQuery,
			"plugin_name":  target.PluginName,
			"plugin_path":  target.PluginPath,
			"plugin_kind":  pluginKind,
			"user_message": userText,
			"intent":       target.Intent,
		},
	}
	if boolValue(requestContext["mix_treatment_preparation"]) {
		plan.WorkflowData["mix_treatment_preparation"] = true
		if preparationPlan := cloneContext(mapValue(requestContext["mix_treatment_preparation_plan"])); len(preparationPlan) > 0 {
			plan.WorkflowData["mix_treatment_preparation_plan"] = preparationPlan
		}
	}
	s.mu.Lock()
	s.pending[plan.ID] = plan
	s.mu.Unlock()

	name := target.PluginName
	if name == "" {
		name = filepath.Base(target.PluginPath)
	}
	reply := fmt.Sprintf("将加载%s %s 到当前轨道。确认后我会抓取参数摘要。", pluginKindForUser(pluginKind), name)
	return ChatResponse{
		ConversationID:    conversationID,
		Reply:             reply + "\n\n" + confirmationReply(decisions),
		NeedsConfirmation: true,
		PlanID:            plan.ID,
		Preview:           preview,
		Workflow:          pluginGrabberLoadCommand,
		WorkflowData:      copyStringAnyMap(plan.WorkflowData),
		Commands:          decisions,
	}
}

func (s *Server) resolvePluginLoadTarget(ctx context.Context, workflowCmd map[string]any, requestContext map[string]any, userText string) (pluginLoadTarget, error) {
	args := workflowCommandArgs(workflowCmd)
	target := pluginLoadTarget{
		TrackID:     firstNonEmptyText(args, "track_id", "selected_track_id", "selected_plugin_track_id"),
		PluginQuery: firstNonEmptyText(args, "plugin_query", "query", "plugin_name", "plugin", "name"),
		PluginPath:  firstNonEmptyText(args, "plugin_path", "path", "file_path"),
		PluginName:  firstNonEmptyText(args, "plugin_name", "name"),
		Intent:      firstNonEmptyText(args, "intent", "user_intent"),
		IntentKind:  firstNonEmptyText(args, "plugin_intent_kind", "intent_kind", "plugin_type", "type"),
	}
	if target.Intent == "" {
		target.Intent = strings.TrimSpace(userText)
	}
	if target.IntentKind == "" {
		target.IntentKind = plugingrabber.PluginIntentKind(target.Intent + " " + target.PluginQuery)
	}
	if target.TrackID == "" {
		target.TrackID = firstNonEmptyText(requestContext, "selected_track_id", "selected_plugin_track_id", "track_id")
	}
	if target.PluginQuery == "" && target.PluginPath == "" {
		target.PluginQuery = extractPluginLoadQuery(userText)
	}
	if target.PluginQuery == "" && target.PluginPath != "" {
		target.PluginQuery = filepath.Base(target.PluginPath)
	}
	if target.TrackID == "" {
		resolved, err := resolveChatTrackIDForPluginLoad(s.harness.UserStateSummary(ctx), requestContext)
		if err != nil {
			return target, err
		}
		target.TrackID = resolved
	}
	if target.TrackID == "" {
		return target, fmt.Errorf("plugin load target track is ambiguous; select or name a track")
	}
	if target.PluginQuery == "" && target.PluginPath == "" {
		return target, fmt.Errorf("plugin load requires plugin_query or plugin_path")
	}
	return target, nil
}

func resolveChatTrackIDForPluginLoad(state map[string]any, requestContext map[string]any) (string, error) {
	return daw.ResolveTrackIDForPluginLoad(state, requestContext)
}
func (s *Server) searchPluginLoadCandidates(ctx context.Context, query, intent string) ([]pluginLoadCandidate, error) {
	var semanticCandidates []pluginLoadCandidate
	var semanticErr error
	semanticCandidates, semanticErr = semanticIndexPluginLoadCandidates(query, intent)
	reply, err := s.searchPluginInventory(ctx, map[string]any{
		"cmd":   "plugin_search",
		"query": query,
		"limit": 8,
	})
	if err != nil {
		if len(semanticCandidates) > 0 {
			return semanticCandidates, nil
		}
		return nil, firstNonNilErr(semanticErr, err)
	}
	if !kernelReplyOK(reply) {
		message := firstNonEmptyText(reply, "message", "error")
		if message == "" {
			message = "plugin_search failed"
		}
		if len(semanticCandidates) > 0 {
			return semanticCandidates, nil
		}
		return nil, fmt.Errorf("%s", message)
	}
	out := mergePluginLoadCandidates(pluginLoadCandidatesFromRows(mapRowsValue(reply["plugins"])), semanticCandidates)
	if ranked := rankPluginLoadCandidates(query, intent, out); len(ranked) > 0 {
		return ranked, nil
	}
	if len(out) == 0 {
		if fallback, err := s.semanticPluginLoadFallbackCandidates(ctx, query, intent); err == nil && len(fallback) > 0 {
			return fallback, nil
		}
	}
	if len(semanticCandidates) > 0 {
		return semanticCandidates, nil
	}
	return out, nil
}

func semanticIndexPluginLoadCandidates(query, intent string) ([]pluginLoadCandidate, error) {
	idx, err := pluginsemantics.Load("")
	if err != nil {
		if pluginsemantics.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	opts := pluginsemantics.SearchOptions{
		Query:              query,
		Limit:              16,
		IncludeInstruments: true,
	}
	if pluginLoadIntentKind(query, intent) == "instrument" {
		opts.Type = "synth"
	}
	results := pluginsemantics.Search(idx, opts)
	out := make([]pluginLoadCandidate, 0, len(results))
	for _, entry := range results {
		if strings.TrimSpace(entry.PluginPath) == "" {
			continue
		}
		out = append(out, pluginLoadCandidate{
			Name:         firstNonEmptyText(map[string]any{"name": entry.Name, "descriptive_name": entry.DescriptiveName}, "name", "descriptive_name"),
			Path:         entry.PluginPath,
			Format:       entry.Format,
			Manufacturer: entry.Manufacturer,
			Category:     entry.Category,
			Score:        entry.SearchScore,
			IsInstrument: entry.IsInstrument,
		})
	}
	return rankPluginLoadCandidates(query, intent, out), nil
}

func (s *Server) semanticPluginLoadFallbackCandidates(ctx context.Context, query, intent string) ([]pluginLoadCandidate, error) {
	reply, err := s.searchPluginInventory(ctx, map[string]any{
		"cmd":   "plugin_list_available",
		"limit": 500,
	})
	if err != nil {
		return nil, err
	}
	if !kernelReplyOK(reply) {
		message := firstNonEmptyText(reply, "message", "error")
		if message == "" {
			message = "plugin_list_available failed"
		}
		return nil, fmt.Errorf("%s", message)
	}
	return rankPluginLoadCandidates(query, intent, pluginLoadCandidatesFromRows(mapRowsValue(reply["plugins"]))), nil
}

func (s *Server) searchPluginInventory(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if s != nil && s.harness != nil {
		resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
			Tool:    "daw.invoke",
			Command: cmd,
			Source:  "plugin_grabber_inventory",
		})
		if err != nil {
			return nil, err
		}
		if resp.Status != "ok" {
			return nil, fmt.Errorf("%s", firstNonEmpty(resp.Error, "plugin inventory command failed"))
		}
		return resp.Result, nil
	}
	if s == nil || s.kernel == nil {
		return nil, fmt.Errorf("plugin inventory requires a kernel client")
	}
	reply, _, err := s.kernel.SendCommand(ctx, cmd)
	return reply, err
}

func pluginLoadCandidatesFromRows(rows []map[string]any) []pluginLoadCandidate {
	var out []pluginLoadCandidate
	for _, row := range rows {
		candidate := pluginLoadCandidate{
			Name:         firstNonEmptyText(row, "name", "descriptive_name"),
			Path:         firstNonEmptyText(row, "plugin_path", "path", "file_or_identifier", "file_path"),
			Format:       firstNonEmptyText(row, "format"),
			Manufacturer: firstNonEmptyText(row, "manufacturer"),
			Category:     firstNonEmptyText(row, "category"),
			Score:        intNumber(row["match_score"]),
			IsInstrument: boolValue(row["is_instrument"]),
		}
		if candidate.Path == "" {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func mergePluginLoadCandidates(primary []pluginLoadCandidate, secondary []pluginLoadCandidate) []pluginLoadCandidate {
	if len(primary) == 0 {
		return append([]pluginLoadCandidate(nil), secondary...)
	}
	if len(secondary) == 0 {
		return append([]pluginLoadCandidate(nil), primary...)
	}
	merged := make([]pluginLoadCandidate, 0, len(primary)+len(secondary))
	seen := map[string]bool{}
	add := func(candidate pluginLoadCandidate) {
		key := pluginLoadCandidateKey(candidate)
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(candidate.Name))
		}
		if key != "" && seen[key] {
			return
		}
		if key != "" {
			seen[key] = true
		}
		merged = append(merged, candidate)
	}
	for _, candidate := range primary {
		add(candidate)
	}
	for _, candidate := range secondary {
		add(candidate)
	}
	return merged
}

func pluginLoadCandidateKey(candidate pluginLoadCandidate) string {
	for _, value := range []string{candidate.Path, candidate.Name} {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonNilErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func rankPluginLoadCandidates(query, intent string, candidates []pluginLoadCandidate) []pluginLoadCandidate {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || len(candidates) == 0 {
		return nil
	}
	scored := make([]pluginLoadCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		score := candidate.Score + semanticPluginLoadScore(query, intent, candidate) + pluginLoadNameScore(query, intent, candidate)
		if score < 300 {
			continue
		}
		candidate.Score = score
		scored = append(scored, candidate)
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return strings.ToLower(scored[i].Name) < strings.ToLower(scored[j].Name)
	})
	if len(scored) > 8 {
		scored = scored[:8]
	}
	return scored
}

func pluginLoadNameScore(query, intent string, candidate pluginLoadCandidate) int {
	queryName := normalizePluginLoadQueryText(query)
	if queryName == "" {
		return 0
	}
	name := normalizePluginCandidateText(candidate.Name)
	basePath := strings.TrimSuffix(filepath.Base(candidate.Path), filepath.Ext(filepath.Base(candidate.Path)))
	baseName := normalizePluginCandidateText(basePath)
	haystack := normalizePluginCandidateText(strings.Join([]string{
		candidate.Name,
		candidate.Manufacturer,
		candidate.Category,
		basePath,
		candidate.Path,
	}, " "))
	score := 0
	for _, candidateName := range []string{name, baseName} {
		if candidateName == "" {
			continue
		}
		switch {
		case candidateName == queryName:
			score += 2200
		case trimPluginEffectSuffix(candidateName) == queryName:
			score += 1450
		case strings.Contains(candidateName, queryName):
			score += 700
		}
	}
	for _, token := range strings.Fields(queryName) {
		if len(token) >= 2 && strings.Contains(haystack, token) {
			score += 130
		}
	}
	intentKind := pluginLoadIntentKind(query, intent)
	if intentKind == "instrument" {
		if candidate.IsInstrument {
			score += 1300
		} else {
			score -= 550
		}
		if strings.Contains(haystack, "instrument") || strings.Contains(haystack, "synth") {
			score += 720
		}
		if pluginLoadCandidateLooksEffect(candidate) {
			score -= 1200
		}
	} else if intentKind == "effect" {
		if candidate.IsInstrument {
			score -= 700
		} else {
			score += 420
		}
	} else if pluginLoadCandidateLooksEffect(candidate) && (trimPluginEffectSuffix(name) == queryName || trimPluginEffectSuffix(baseName) == queryName) {
		score -= 320
	}
	return score
}

func semanticPluginLoadCandidates(query, intent string, candidates []pluginLoadCandidate) []pluginLoadCandidate {
	return rankPluginLoadCandidates(query, intent, candidates)
}

func pluginKindForUser(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "instrument", "synth":
		return "乐器"
	case "effect", "fx":
		return "效果器"
	default:
		return "插件"
	}
}

func pluginLoadIntentKind(query, intent string) string {
	kind := plugingrabber.PluginIntentKind(strings.TrimSpace(query + " " + intent))
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "instrument", "synth":
		return "instrument"
	case "effect", "fx":
		return "effect"
	default:
		return ""
	}
}

func pluginLoadCandidateLooksEffect(candidate pluginLoadCandidate) bool {
	text := strings.ToLower(strings.Join([]string{
		candidate.Name,
		candidate.Category,
		filepath.Base(candidate.Path),
	}, " "))
	return strings.Contains(text, "effect") ||
		strings.Contains(text, "effects") ||
		strings.Contains(text, " fx") ||
		strings.Contains(text, "|fx") ||
		strings.Contains(text, "fx|")
}

func normalizePluginLoadQueryText(text string) string {
	text = normalizePluginCandidateText(text)
	replacer := strings.NewReplacer(
		"plugin", " ", "vst", " ", "vst3", " ",
		"synth", " ", "synthesizer", " ", "instrument", " ", "vsti", " ", "midi", " ",
		"\u5408\u6210\u5668", " ", "\u4e50\u5668", " ", "\u97f3\u6e90", " ",
		"\u63d2\u4ef6", " ", "\u6548\u679c\u5668", " ",
	)
	return strings.Join(strings.Fields(replacer.Replace(text)), " ")
}

func normalizePluginCandidateText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer(
		".vst3", " ", ".vst", " ", ".dll", " ",
		"\\", " ", "/", " ", "_", " ", "-", " ",
		"(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ",
		",", " ", ".", " ", ";", " ", ":", " ",
	)
	return strings.Join(strings.Fields(replacer.Replace(text)), " ")
}

func trimPluginEffectSuffix(text string) string {
	for _, suffix := range []string{" effects", " effect", " fx"} {
		if strings.HasSuffix(text, suffix) {
			return strings.TrimSpace(strings.TrimSuffix(text, suffix))
		}
	}
	return text
}

func semanticPluginLoadScore(query, intent string, candidate pluginLoadCandidate) int {
	haystack := strings.ToLower(strings.Join([]string{
		candidate.Name,
		candidate.Manufacturer,
		candidate.Category,
		filepath.Base(candidate.Path),
		candidate.Path,
	}, " "))
	score := 0
	if strings.Contains(query, "reverb") || strings.Contains(query, "verb") || strings.Contains(query, "\u6df7\u54cd") {
		score += semanticTermScore(haystack, "reverb", 700)
		score += semanticTermScore(haystack, "verb", 520)
		score += semanticTermScore(haystack, "room", 360)
		score += semanticTermScore(haystack, "hall", 360)
		score += semanticTermScore(haystack, "plate", 340)
		score += semanticTermScore(haystack, "space", 320)
		score += semanticTermScore(haystack, "shimmer", 320)
		score += semanticTermScore(haystack, "valhalla", 720)
		score += semanticTermScore(haystack, "supermassive", 720)
	}
	if strings.Contains(query, "delay") {
		score += semanticTermScore(haystack, "delay", 700)
		score += semanticTermScore(haystack, "echo", 420)
		score += semanticTermScore(haystack, "space", 220)
		score += semanticTermScore(haystack, "valhalla", 220)
	}
	if strings.Contains(query, "compressor") || strings.Contains(query, "comp") {
		score += semanticTermScore(haystack, "compressor", 700)
		score += semanticTermScore(haystack, "comp", 420)
		score += semanticTermScore(haystack, "dynamics", 360)
	}
	if strings.Contains(query, "eq") || strings.Contains(query, "equalizer") {
		score += semanticTermScore(haystack, "eq", 700)
		score += semanticTermScore(haystack, "equalizer", 700)
		score += semanticTermScore(haystack, "nova", 260)
	}
	if pluginLoadIntentKind(query, intent) == "instrument" {
		score += semanticTermScore(haystack, "synth", 700)
		score += semanticTermScore(haystack, "instrument", 680)
		score += semanticTermScore(haystack, "sampler", 360)
		score += semanticTermScore(haystack, "surge", 220)
		if candidate.IsInstrument {
			score += 900
		} else {
			score -= 650
		}
		if pluginLoadCandidateLooksEffect(candidate) {
			score -= 900
		}
	}
	if score > 0 {
		if strings.Contains(haystack, "fx") {
			score += 40
		}
		if candidate.IsInstrument && pluginLoadIntentKind(query, intent) != "instrument" {
			score -= 300
		} else if !candidate.IsInstrument {
			score += 80
		}
	}
	return score
}

func semanticTermScore(haystack, term string, score int) int {
	if strings.Contains(haystack, term) {
		return score
	}
	return 0
}

func (s *Server) finishPluginGrabberLoadWorkflow(ctx context.Context, plan PendingPlan, replies []map[string]any, fallbackMessage string) (string, []map[string]any) {
	trackID, pluginID, pluginName := pluginLoadResultIDs(replies)
	if trackID == "" {
		trackID = firstNonEmptyText(plan.WorkflowData, "track_id")
	}
	if pluginName == "" {
		pluginName = firstNonEmptyText(plan.WorkflowData, "plugin_name")
	}
	if pluginID == "" {
		return strings.TrimSpace(fallbackMessage + "\n插件已加载，但内核没有返回 plugin_id，所以还不能自动抓参数。"), replies
	}
	if s.kernel == nil {
		return strings.TrimSpace(fallbackMessage + "\n插件已加载，但 VitAgent 当前没有 kernel client，不能继续抓参数。"), replies
	}
	paramsReply, _, err := s.kernel.SendCommand(ctx, map[string]any{
		"cmd":       "get_plugin_parameters",
		"track_id":  trackID,
		"plugin_id": pluginID,
	})
	if err != nil || !kernelReplyOK(paramsReply) {
		message := ""
		if err != nil {
			message = err.Error()
		} else {
			message = firstNonEmptyText(paramsReply, "message", "error")
		}
		return strings.TrimSpace(fallbackMessage + "\n插件已加载，但抓参数失败：" + message), replies
	}
	s.observePluginParametersReply(paramsReply)
	digest := buildPluginParameterDigest(paramsReply)
	if pluginName == "" {
		pluginName = digest.PluginName
	}
	if pluginName == "" {
		pluginName = pluginID
	}
	compact := compactPluginParameterDigestResult(digest)
	replies = append(replies, map[string]any{
		"status":       "ok",
		"command_name": "get_plugin_parameters",
		"result":       compact,
	})
	return formatPluginLoadGrabReply(pluginName, digest), replies
}

func pluginLoadResultIDs(replies []map[string]any) (string, string, string) {
	for i := len(replies) - 1; i >= 0; i-- {
		row := replies[i]
		if name := firstNonEmptyText(row, "command_name"); name != "" && name != "rack_add_node" {
			continue
		}
		result, _ := row["result"].(map[string]any)
		if result == nil {
			continue
		}
		pluginID := firstNonEmptyText(result, "plugin_id", "plugin_item_id", "item_id")
		if pluginID == "" {
			continue
		}
		return firstNonEmptyText(result, "track_id"), pluginID, firstNonEmptyText(result, "plugin_name", "name")
	}
	return "", "", ""
}

func compactPluginParameterDigestResult(digest pluginParameterDigest) map[string]any {
	quick := make([]map[string]any, 0, len(digest.QuickControls))
	for i, row := range digest.QuickControls {
		if i >= 12 {
			break
		}
		quick = append(quick, map[string]any{
			"param_id":        row.ParamID,
			"label":           row.Label,
			"display_group":   row.DisplayGroup,
			"normalized_role": row.NormalizedRole,
		})
	}
	groups := make([]map[string]any, 0, len(digest.RecommendedGroups))
	for i, row := range digest.RecommendedGroups {
		if i >= 12 {
			break
		}
		groups = append(groups, map[string]any{
			"name":            row.Name,
			"parameter_count": row.ParameterCount,
			"sample_ids":      row.SampleIDs,
		})
	}
	return map[string]any{
		"status":                  "ok",
		"track_id":                digest.TrackID,
		"plugin_id":               digest.PluginID,
		"plugin_name":             digest.PluginName,
		"template_role":           digest.TemplateRole,
		"profile_source":          digest.ProfileSource,
		"profile_applied":         digest.ProfileApplied,
		"parameter_count":         digest.ParameterCount,
		"quick_control_count":     len(digest.QuickControls),
		"quick_controls":          quick,
		"recommended_group_count": len(digest.RecommendedGroups),
		"recommended_groups":      groups,
		"display_probe_summary":   plugingrabber.DisplayProbeSummary(digest),
	}
}

func formatPluginLoadGrabReply(pluginName string, digest pluginParameterDigest) string {
	labels := make([]string, 0, 8)
	for _, row := range digest.QuickControls {
		label := strings.TrimSpace(row.Label)
		if label == "" {
			label = row.ParamID
		}
		if label != "" {
			labels = append(labels, label)
		}
		if len(labels) >= 8 {
			break
		}
	}
	groups := make([]string, 0, 8)
	for _, row := range digest.RecommendedGroups {
		if strings.TrimSpace(row.Name) != "" {
			groups = append(groups, row.Name)
		}
		if len(groups) >= 8 {
			break
		}
	}
	parts := []string{fmt.Sprintf("已加载 %s，并抓到 %d 个可控参数。", pluginName, digest.ParameterCount)}
	if len(labels) > 0 {
		parts = append(parts, "Quick controls: "+strings.Join(labels, ", ")+"。")
	}
	if len(groups) > 0 {
		parts = append(parts, "Groups: "+strings.Join(groups, ", ")+"。")
	}
	return strings.Join(parts, "\n")
}

func (s *Server) storePluginProfilePatchArtifact(conversationID string, target pluginLearningTarget, patch pluginProfilePatch, workflowData map[string]any) []artifacts.Summary {
	if firstNonEmptyText(workflowData, "plan_id") == "" {
		return nil
	}
	patch = canonicalPluginSkillPatch(patch)
	payload := map[string]any{
		"schema":         "vit.plugin_skill.v1",
		"legacy_schema":  "plugin_profile_patch.v1",
		"target":         map[string]any{"track_id": target.TrackID, "plugin_id": target.PluginID, "plugin_name": target.PluginName},
		"profile_patch":  patch,
		"plugin_skill":   patch,
		"file_extension": ".vps",
	}
	for _, key := range []string{"mode", "stage", "strategy", "class", "parameter_count", "component_count", "operation_count", "validation_summary", "validation_warnings", "plan_id"} {
		if value, ok := workflowData[key]; ok {
			payload[key] = value
		}
	}
	uiReference := mapValue(workflowData["ui_reference"])
	if len(uiReference) > 0 {
		payload["ui_reference_status"] = firstNonEmptyText(uiReference, "status")
		payload["ui_reference_artifact_ids"] = stringListValue(firstPresentAny(uiReference, "artifact_ids", "ui_reference_artifact_ids"))
		if warnings := compactStringList(stringListValue(uiReference["warnings"])); len(warnings) > 0 {
			payload["ui_reference_warnings"] = warnings
		}
		if visualDigest := mapValue(uiReference["visual_digest"]); len(visualDigest) > 0 {
			if matchingStatus := firstNonEmptyText(visualDigest, "matching_status"); matchingStatus != "" {
				payload["ui_reference_matching_status"] = matchingStatus
			}
		}
		if audit := mapValue(uiReference["parameter_coverage_audit"]); len(audit) > 0 {
			payload["ui_reference_parameter_coverage"] = audit
		}
	}
	if typeHypothesis := mapValue(workflowData["plugin_type_hypothesis"]); len(typeHypothesis) > 0 {
		payload["plugin_type_hypothesis"] = typeHypothesis
	}
	if webReference := mapValue(workflowData["plugin_web_reference"]); len(webReference) > 0 {
		payload["plugin_web_reference"] = webReference
	}
	if coreCoverage := mapValue(workflowData["core_control_coverage"]); len(coreCoverage) > 0 {
		payload["core_control_coverage"] = coreCoverage
	}
	text, _ := json.MarshalIndent(payload, "", "  ")
	a := artifacts.New(time.Now())
	a.ID = pluginSkillArtifactID(target, workflowData)
	a.Kind = "plugin_skill"
	a.Source = "plugin_grabber"
	a.Title = pluginSkillArtifactTitle(target.PluginName)
	a.Summary = fmt.Sprintf("Plugin Skill: %d components, %d virtual controls", len(patch.Groups), len(patch.VirtualControls))
	a.MIME = "application/vnd.vit.plugin-skill+json"
	a.Text = string(text)
	a.ConversationID = conversationID
	a.Metadata = map[string]any{
		"artifact_schema": "vit.plugin_skill.v1",
		"legacy_schema":   "plugin_profile_patch.v1",
		"legacy_kind":     "plugin_profile_patch",
		"file_extension":  ".vps",
		"display_name":    firstNonEmpty(target.PluginName, "Plugin") + " Plugin Skill",
		"track_id":        target.TrackID,
		"plugin_id":       target.PluginID,
		"plugin_name":     target.PluginName,
		"class":           patch.Class,
		"component_count": len(patch.Groups),
		"operation_count": len(patch.VirtualControls),
		"stage":           workflowData["stage"],
		"strategy":        workflowData["strategy"],
		"plan_id":         workflowData["plan_id"],
	}
	if len(uiReference) > 0 {
		a.Metadata["ui_reference_status"] = firstNonEmptyText(uiReference, "status")
		a.Metadata["ui_reference_artifact_ids"] = stringListValue(firstPresentAny(uiReference, "artifact_ids", "ui_reference_artifact_ids"))
		if warnings := compactStringList(stringListValue(uiReference["warnings"])); len(warnings) > 0 {
			a.Metadata["ui_reference_warnings"] = warnings
		}
		if visualDigest := mapValue(uiReference["visual_digest"]); len(visualDigest) > 0 {
			if matchingStatus := firstNonEmptyText(visualDigest, "matching_status"); matchingStatus != "" {
				a.Metadata["ui_reference_matching_status"] = matchingStatus
			}
		}
		if audit := mapValue(uiReference["parameter_coverage_audit"]); len(audit) > 0 {
			a.Metadata["ui_reference_parameter_coverage"] = audit
		}
	}
	if typeHypothesis := mapValue(workflowData["plugin_type_hypothesis"]); len(typeHypothesis) > 0 {
		a.Metadata["plugin_type_hypothesis_status"] = firstNonEmptyText(typeHypothesis, "status")
		a.Metadata["plugin_type_hypothesis_primary_type"] = firstNonEmptyText(typeHypothesis, "primary_type")
	}
	if webReference := mapValue(workflowData["plugin_web_reference"]); len(webReference) > 0 {
		a.Metadata["plugin_web_reference_status"] = firstNonEmptyText(webReference, "status")
	}
	if coreCoverage := mapValue(workflowData["core_control_coverage"]); len(coreCoverage) > 0 {
		a.Metadata["plugin_core_control_coverage"] = map[string]any{
			"plugin_type":     firstNonEmptyText(coreCoverage, "plugin_type"),
			"covered_count":   len(mapRowsValue(coreCoverage["covered"])),
			"uncertain_count": len(mapRowsValue(coreCoverage["uncertain"])),
			"missing_count":   len(mapRowsValue(coreCoverage["missing"])),
		}
	}
	scope := artifacts.ScopeFromMap(workflowData)
	if scope.Empty() {
		scope = artifacts.ScopeFromMap(mapValue(workflowData["request_context"]))
	}
	if scope.Empty() {
		scope = s.artifactScopeFromArgs(context.Background(), nil)
	}
	a = artifacts.ApplyScope(a, scope)
	stored, err := s.artifactStore().Upsert(a)
	if err != nil {
		return nil
	}
	return []artifacts.Summary{stored.CompactSummary()}
}

func (s *Server) recordPluginLearningStage(conversationID string, requestContext map[string]any, sessionID string, target pluginLearningTarget, stage, status string, payload map[string]any) artifacts.Summary {
	sessionID = strings.TrimSpace(sessionID)
	stage = strings.TrimSpace(stage)
	if s == nil || sessionID == "" || stage == "" {
		return artifacts.Summary{}
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "recorded"
	}
	now := time.Now()
	doc := map[string]any{
		"schema":                     pluginLearningSessionSchema,
		"plugin_learning_session_id": sessionID,
		"stage":                      stage,
		"status":                     status,
		"target":                     pluginLearningTargetMap(target),
		"updated_at":                 now.UTC().Format(time.RFC3339Nano),
	}
	if len(payload) > 0 {
		doc["payload"] = payload
	}
	text, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return artifacts.Summary{}
	}
	a := artifacts.New(now)
	a.ID = pluginLearningStageArtifactID(sessionID, stage)
	a.Kind = "plugin_learning_stage"
	a.Source = "plugin_grabber"
	a.Title = pluginLearningStageArtifactTitle(target.PluginName, stage)
	a.Summary = fmt.Sprintf("Plugin Skill learning stage %s: %s", stage, status)
	a.MIME = "application/vnd.vit.plugin-learning-stage+json"
	a.Text = string(text)
	a.ConversationID = conversationID
	a.Metadata = map[string]any{
		"artifact_schema":            pluginLearningSessionSchema,
		"plugin_learning_session_id": sessionID,
		"plugin_learning_stage":      stage,
		"plugin_learning_status":     status,
		"track_id":                   target.TrackID,
		"plugin_id":                  target.PluginID,
		"plugin_name":                target.PluginName,
	}
	if scope := artifacts.ScopeFromMap(requestContext); !scope.Empty() {
		a = artifacts.ApplyScope(a, scope)
	}
	stored, err := s.artifactStore().Upsert(a)
	if err != nil {
		return artifacts.Summary{}
	}
	return stored.CompactSummary()
}

func pluginLearningStageArtifactID(sessionID, stage string) string {
	return "pl_stage_" + pluginLearningSafeArtifactToken(sessionID) + "_" + pluginLearningSafeArtifactToken(stage)
}

func pluginLearningStageArtifactTitle(pluginName, stage string) string {
	name := strings.TrimSpace(pluginName)
	if name == "" {
		name = "Plugin"
	}
	label := strings.ReplaceAll(strings.TrimSpace(stage), "_", " ")
	if label == "" {
		label = "stage"
	}
	return name + " Plugin Skill learning - " + label
}

func pluginLearningSafeArtifactToken(text string) string {
	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range text {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	clean := strings.Trim(b.String(), "_")
	if clean == "" {
		return "unknown"
	}
	if len(clean) > 80 {
		clean = clean[:80]
	}
	return clean
}

func appendPluginLearningStageArtifact(rows []artifacts.Summary, summary artifacts.Summary) []artifacts.Summary {
	if strings.TrimSpace(summary.ID) == "" {
		return rows
	}
	return append(rows, summary)
}

func pluginSkillArtifactTitle(pluginName string) string {
	base := strings.TrimSpace(pluginName)
	if base == "" {
		base = "Plugin Skill"
	}
	base = strings.TrimSuffix(base, ".vps")
	base = strings.TrimSuffix(base, ".VPS")
	clean := strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, base)
	clean = strings.Trim(clean, " ._-")
	if clean == "" {
		clean = "Plugin Skill"
	}
	return clean + ".vps"
}

func pluginSkillArtifactID(target pluginLearningTarget, workflowData map[string]any) string {
	key := firstNonEmptyText(mapValue(workflowData["plugin_identity"]), "profile_key")
	if key == "" {
		key = firstNonEmptyText(mapValue(workflowData["target"]), "profile_key")
	}
	if key == "" {
		key = firstNonEmpty(target.PluginID, target.PluginName)
	}
	hash := strings.TrimSpace(firstNonEmptyText(workflowData, "param_signature_hash"))
	parts := []string{"plugin_skill", pluginLearningSafeArtifactToken(key)}
	if hash != "" {
		parts = append(parts, pluginLearningSafeArtifactToken(hash))
	}
	return strings.Join(parts, "_")
}

func canonicalPluginSkillPatch(patch pluginProfilePatch) pluginProfilePatch {
	out := patch
	out.Groups = canonicalPluginSkillRows(patch.Groups, "group")
	out.VirtualControls = canonicalPluginSkillRows(patch.VirtualControls, "operation")
	out.Safety = plugingrabber.SanitizeProfileSafety(patch.Safety)
	return out
}

func canonicalPluginSkillRows(rows []map[string]any, rowKind string) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := map[string]any{}
		keys := []string{"id", "role", "label", "name", "description", "resolver", "component_id", "component", "status"}
		if rowKind == "operation" {
			keys = append(keys, "operation")
		}
		for _, key := range keys {
			if value := firstPresentAny(row, key); value != nil {
				next[key] = value
			}
		}
		if inputs := stringListValue(row["inputs"]); len(inputs) > 0 {
			next["inputs"] = inputs
		}
		if params := mapValue(row["params"]); len(params) > 0 {
			next["params"] = canonicalPluginSkillParams(params)
		}
		if len(next) > 0 {
			out = append(out, next)
		}
	}
	return out
}

func canonicalPluginSkillParams(params map[string]any) map[string]any {
	out := map[string]any{}
	for slot, raw := range params {
		slot = strings.TrimSpace(slot)
		if slot == "" {
			continue
		}
		mapping := canonicalPluginSkillMapping(raw)
		if len(mapping) > 0 {
			out[slot] = mapping
		}
	}
	return out
}

func canonicalPluginSkillMapping(raw any) map[string]any {
	mapping := mapValue(raw)
	paramID := firstNonEmptyText(mapping, "param_id", "id")
	if paramID == "" {
		paramID = strings.TrimSpace(fmt.Sprint(raw))
	}
	if paramID == "" || paramID == "<nil>" {
		return nil
	}
	out := map[string]any{"param_id": paramID}
	for _, key := range []string{
		"label",
		"source",
		"confidence",
		"confirmed",
		"status",
		"locked",
		"display_domain",
		"display_domain_text",
		"visual_label",
		"visual_group",
		"visual_confidence",
		"visual_match_score",
		"visual_backend_name",
		"visual_match_reason",
		"visual_display_domain_text",
	} {
		if value := firstPresentAny(mapping, key); value != nil {
			out[key] = canonicalPluginSkillValue(key, value)
		}
	}
	if missing := pluginLearningCompactStringList(stringListValue(mapping["missing_evidence"]), 6, 140); len(missing) > 0 {
		out["missing_evidence"] = missing
	}
	if warnings := pluginLearningCompactStringList(stringListValue(mapping["validator_warnings"]), 6, 140); len(warnings) > 0 {
		out["validator_warnings"] = warnings
	}
	return out
}

func canonicalPluginSkillValue(key string, value any) any {
	switch typed := value.(type) {
	case string:
		return pluginLearningPromptClip(typed, 220)
	case []string:
		return pluginLearningCompactStringList(typed, 8, 160)
	case []any:
		return pluginLearningCompactAnyList(typed, 160)
	case map[string]any:
		switch key {
		case "display_domain":
			return pluginLearningCompactRow(typed, 160, "text", "unit", "min", "max", "scale", "status", "source", "confidence")
		default:
			return pluginLearningCompactRow(typed, 160, "schema", "status", "summary", "source", "confidence")
		}
	default:
		return value
	}
}

func canonicalPluginSkillEvidenceRows(rows []map[string]any, limit int) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, minInt(len(rows), limit))
	seen := map[string]bool{}
	for _, row := range rows {
		kind := firstNonEmptyText(row, "kind", "source")
		if kind == "" {
			kind = "evidence"
		}
		summary := firstNonEmptyText(row, "summary", "reason", "text")
		switch kind {
		case "plugin_ui_reference_image":
			summary = "Current or prior visual learning contributed distilled UI display evidence."
		case "user_review":
			summary = firstNonEmpty(summary, "User confirmed this mapping or display domain.")
		case "plugin_parameter_display_probe", "display_probe_inferred", "value_to_string_probe":
			summary = firstNonEmpty(summary, "Display-domain evidence inferred from plugin-exposed display text.")
		case "active_roundtrip_probe":
			summary = firstNonEmpty(summary, "Display domain confirmed by active roundtrip probe.")
		}
		next := map[string]any{
			"kind":    kind,
			"summary": pluginLearningPromptClip(summary, 180),
		}
		key := kind + "\x00" + firstNonEmptyText(next, "summary")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, next)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func canonicalPluginSkillProvenanceRows(rows []map[string]any, limit int) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, minInt(len(rows), limit))
	seen := map[string]bool{}
	for _, row := range rows {
		kind := firstNonEmptyText(row, "kind", "source")
		source := firstNonEmptyText(row, "source")
		summary := firstNonEmptyText(row, "summary", "reason", "text")
		switch kind {
		case "plugin_ui_reference_image":
			summary = "Distilled visual UI evidence; raw session image and artifact IDs are stored only in the learning ledger."
		case "user_review":
			summary = firstNonEmpty(summary, "User review confirmed this mapping or display domain.")
		case "plugin_parameter_display_probe", "display_probe_inferred", "value_to_string_probe":
			summary = firstNonEmpty(summary, "Plugin display probe supplied display-domain evidence.")
		}
		next := map[string]any{
			"kind":    kind,
			"source":  firstNonEmpty(source, kind),
			"summary": pluginLearningPromptClip(summary, 180),
		}
		key := firstNonEmptyText(next, "kind") + "\x00" + firstNonEmptyText(next, "source") + "\x00" + firstNonEmptyText(next, "summary")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, next)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func pluginGrabberUIReferenceRequestResponse(conversationID, userText string, requestContext map[string]any, target pluginLearningTarget, sessionID, startedAt string) ChatResponse {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = "pl_session_" + randomID()
	}
	startedAt = strings.TrimSpace(startedAt)
	if startedAt == "" {
		startedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	targetMap := pluginLearningTargetMap(target)
	preparationPlan := cloneContext(mapValue(requestContext["mix_treatment_preparation_plan"]))
	data := map[string]any{
		"mode":                               string(plugingrabber.LearningModeAutoLearn),
		"stage":                              pluginLearningUIReferenceStage,
		"type":                               pluginLearningUIReferenceType,
		"plugin_learning_session_id":         sessionID,
		"plugin_learning_session_started_at": startedAt,
		"target":                             targetMap,
		"track_id":                           target.TrackID,
		"plugin_id":                          target.PluginID,
		"plugin_name":                        target.PluginName,
		"intent":                             strings.TrimSpace(userText),
		"request_context":                    requestContext,
		"ui_reference_optional":              true,
		"web_reference_optional":             true,
		"web_reference_default":              "skipped",
		"ui_reference": map[string]any{
			"status":  "requested",
			"purpose": pluginLearningUIReferencePurpose,
		},
		"web_reference": map[string]any{
			"status":   "offered",
			"optional": true,
			"purpose":  "plugin_docs_reference",
		},
	}
	if len(preparationPlan) > 0 {
		data["mix_treatment_preparation"] = true
		data["mix_treatment_preparation_plan"] = preparationPlan
	}
	reply := "可以先提供一份本次学习专用的插件界面图样。这个步骤是可选的；如果没有合适图样，也可以直接跳过继续自动学习。只有现在上传并绑定到本次学习会话的图样会被使用，之前发送过的图片不会计入。"
	return ChatResponse{
		ConversationID: conversationID,
		Reply:          reply,
		Workflow:       "plugin_grabber_auto_learn",
		WorkflowData:   data,
		PluginLearning: data,
	}
}

func pluginLearningTargetMap(target pluginLearningTarget) map[string]any {
	return map[string]any{
		"track_id":    target.TrackID,
		"plugin_id":   target.PluginID,
		"plugin_name": target.PluginName,
	}
}

func (s *Server) resolvePluginUIReference(ctx context.Context, cfg config.EngineConfig, args map[string]any, target pluginLearningTarget, parameterDigest pluginParameterDigest, learningEvidence ...map[string]any) (map[string]any, []artifacts.Summary, error) {
	decision := strings.TrimSpace(strings.ToLower(firstNonEmptyText(args, "ui_reference_decision")))
	ids := stringListValue(args["ui_reference_artifact_ids"])
	if decision == "" && len(ids) > 0 {
		decision = "provided"
	}
	sessionID := strings.TrimSpace(firstNonEmptyText(args, "plugin_learning_session_id"))
	startedAt := strings.TrimSpace(firstNonEmptyText(args, "plugin_learning_session_started_at"))
	out := map[string]any{
		"status":                      "skipped",
		"decision":                    firstNonEmpty(decision, "skipped"),
		"purpose":                     pluginLearningUIReferencePurpose,
		"plugin_learning_session_id":  sessionID,
		"session_started_at":          startedAt,
		"target":                      pluginLearningTargetMap(target),
		"ui_reference_artifact_ids":   ids,
		"visual_digest_schema":        "plugin_ui_reference_digest.v1",
		"visual_digest_source_policy": "image-only evidence; never final param_id authority",
	}
	if decision == "" || decision == "skipped" || decision == "skip" {
		out["status"] = "skipped"
		out["decision"] = "skipped"
		return out, nil, nil
	}
	valid, summaries, warnings := s.validPluginUIReferenceArtifacts(args, target)
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	if len(valid) == 0 {
		return nil, nil, fmt.Errorf("%w: no current-session plugin interface image pattern was provided", errPluginUIReferenceNoValidImage)
	}
	out["status"] = "provided"
	out["decision"] = "provided"
	out["artifact_ids"] = artifactIDsFromSummaries(summaries)
	out["artifacts"] = artifactSummaryRows(summaries)
	digest, warning, err := s.buildPluginUIReferenceDigest(ctx, cfg, target, valid, parameterDigest, learningEvidence...)
	if err != nil {
		warnings = append(warnings, warning)
		status := "vision_failed"
		if errors.Is(err, llm.ErrImageUnderstandingRouteUnavailable) {
			status = "vision_unavailable"
		}
		out["status"] = status
		out["warnings"] = compactStringList(warnings)
		focusSummary := pluginUIReferenceVisualFocusSummary(parameterDigest)
		out["visual_focus_summary"] = focusSummary
		out["parameter_coverage_audit"] = pluginUIReferenceCoverageAudit(parameterDigest, focusSummary, nil)
		return out, summaries, nil
	}
	out["visual_digest"] = digest
	if focusSummary := mapValue(digest["visual_focus_summary"]); len(focusSummary) > 0 {
		out["visual_focus_summary"] = focusSummary
	}
	if audit := mapValue(digest["parameter_coverage_audit"]); len(audit) > 0 {
		out["parameter_coverage_audit"] = audit
	}
	out["warnings"] = compactStringList(append(warnings, stringListValue(digest["warnings"])...))
	return out, summaries, nil
}

func (s *Server) validPluginUIReferenceArtifacts(args map[string]any, target pluginLearningTarget) ([]artifacts.Artifact, []artifacts.Summary, []string) {
	ids := stringListValue(args["ui_reference_artifact_ids"])
	sessionID := strings.TrimSpace(firstNonEmptyText(args, "plugin_learning_session_id"))
	startedAt := strings.TrimSpace(firstNonEmptyText(args, "plugin_learning_session_started_at"))
	var sessionStart time.Time
	if startedAt != "" {
		sessionStart, _ = time.Parse(time.RFC3339Nano, startedAt)
	}
	store := s.artifactStore()
	valid := make([]artifacts.Artifact, 0, len(ids))
	summaries := make([]artifacts.Summary, 0, len(ids))
	warnings := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		a, err := store.Get(id)
		if err != nil {
			warnings = append(warnings, "Ignored missing plugin interface image pattern artifact: "+id)
			continue
		}
		if !artifactIsImage(a) {
			warnings = append(warnings, "Ignored non-image plugin interface pattern artifact: "+id)
			continue
		}
		if !pluginUIReferenceArtifactMatchesSession(a, sessionID, sessionStart, target) {
			warnings = append(warnings, "Ignored image artifact outside the current plugin learning session: "+id)
			continue
		}
		valid = append(valid, a)
		summaries = append(summaries, a.CompactSummary())
	}
	return valid, summaries, compactStringList(warnings)
}

func artifactIsImage(a artifacts.Artifact) bool {
	return strings.EqualFold(strings.TrimSpace(a.Kind), "image") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(a.MIME)), "image/")
}

func pluginUIReferenceArtifactMatchesSession(a artifacts.Artifact, sessionID string, sessionStart time.Time, target pluginLearningTarget) bool {
	meta := a.Metadata
	if sessionID == "" || strings.TrimSpace(fmt.Sprint(meta["plugin_learning_session_id"])) != sessionID {
		return false
	}
	if strings.TrimSpace(fmt.Sprint(meta["plugin_learning_purpose"])) != pluginLearningUIReferencePurpose {
		return false
	}
	if target.TrackID != "" && strings.TrimSpace(fmt.Sprint(meta["track_id"])) != target.TrackID {
		return false
	}
	if target.PluginID != "" && strings.TrimSpace(fmt.Sprint(meta["plugin_id"])) != target.PluginID {
		return false
	}
	if !sessionStart.IsZero() {
		createdAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(a.CreatedAt))
		if err == nil && createdAt.Before(sessionStart) {
			return false
		}
	}
	return true
}

func pluginLearningEvidenceContext(sources ...map[string]any) map[string]any {
	out := map[string]any{
		"schema": "plugin_learning_evidence_context.v1",
		"policy": "Evidence may guide attention and confidence, but final param_id authority remains get_plugin_parameters plus the Plugin Skill validator.",
	}
	for _, source := range sources {
		for key, value := range source {
			if strings.TrimSpace(key) == "" || value == nil {
				continue
			}
			if rows := mapRowsValue(value); len(rows) > 0 {
				out[key] = value
				continue
			}
			if m := mapValue(value); len(m) > 0 {
				out[key] = value
				continue
			}
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				out[key] = value
			}
		}
	}
	return out
}

func (s *Server) buildPluginUIReferenceDigest(ctx context.Context, cfg config.EngineConfig, target pluginLearningTarget, images []artifacts.Artifact, parameterDigest pluginParameterDigest, learningEvidence ...map[string]any) (map[string]any, string, error) {
	if s == nil || s.llm == nil {
		err := llm.ErrImageUnderstandingRouteUnavailable
		return nil, "Image understanding route is unavailable; continuing without plugin interface image pattern digest.", err
	}
	inputs := make([]llm.ImageInput, 0, len(images))
	for _, a := range images {
		path := strings.TrimSpace(a.Path)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		mime := strings.TrimSpace(a.MIME)
		if mime == "" {
			mime = "image/png"
		}
		inputs = append(inputs, llm.ImageInput{
			MIME:       mime,
			DataBase64: base64.StdEncoding.EncodeToString(data),
			Detail:     "high",
		})
		if len(inputs) >= 3 {
			break
		}
	}
	if len(inputs) == 0 {
		return nil, "No readable plugin interface image pattern files were available; continuing without visual digest.", fmt.Errorf("no readable plugin interface image pattern")
	}

	focusSummary := pluginUIReferenceVisualFocusSummary(parameterDigest)
	evidenceContext := pluginLearningEvidenceContext(learningEvidence...)
	targetFocus := pluginUIReferenceVisualTargetFocus(parameterDigest, focusSummary, evidenceContext)
	imageDigest, warning, err := s.buildPluginUIReferenceTargetProbe(ctx, cfg, target, inputs, targetFocus, evidenceContext)
	if err != nil {
		return nil, warning, err
	}
	digest := mergePluginUIReferenceDigests(imageDigest, nil)
	attachPluginUIReferenceFocusAndCoverage(digest, focusSummary, parameterDigest)
	if pluginUIReferenceVisualDigestHasUsableEvidence(digest) {
		digest["visual_target_focus"] = pluginLearningCompactVisualTargetFocus(targetFocus)
	} else if value := firstPresentAny(targetFocus, "target_count"); value != nil {
		digest["visual_target_count"] = value
	}
	digest["visual_matching_mode"] = "type_guided_single_pass"
	return digest, "", nil
}

func (s *Server) buildPluginUIReferenceTargetProbe(ctx context.Context, cfg config.EngineConfig, target pluginLearningTarget, inputs []llm.ImageInput, targetFocus map[string]any, learningEvidence map[string]any) (map[string]any, string, error) {
	targetFocusJSON, err := json.MarshalIndent(targetFocus, "", "  ")
	if err != nil {
		return nil, "Could not prepare type-guided visual targets for plugin interface image probing; continuing without visual digest.", err
	}
	evidenceJSON, err := json.MarshalIndent(pluginUIReferenceCompactLearningEvidence(learningEvidence), "", "  ")
	if err != nil {
		return nil, "Could not prepare plugin learning evidence context for plugin interface image probing; continuing without visual digest.", err
	}
	prompt := fmt.Sprintf(`The provided image is a plugin interface pattern for the current target DAW plugin.

Target plugin:
- track_id: %s
- plugin_id: %s
- plugin_name: %s

Type-guided visual targets and legal backend candidates:
%s

Optional compact learning evidence context:
%s

Your job is NOT to inventory the whole plugin UI. Probe the image for the requested core controls and explain how those controls are displayed to a human user.
Return exactly one target_results row for every supplied target. If the target is not visible or cannot be read, still return a row with status "not_visible" or "uncertain".

Return ONLY one JSON object. Do not use markdown fences. Do not add prose before or after the JSON.
{
  "schema": "plugin_ui_reference_target_probe.v1",
  "target_results": [{"target_id": "copied target id", "control": "copied target control", "priority": "primary|secondary|utility", "status": "visible|partly_visible|not_visible|uncertain", "visible_label": "literal UI label/readout if visible", "visible_group": "visible section/module/channel", "value_text": "current visible value if any", "visible_unit": "Hz|dB|%%|ms|s|note|ratio|toggle|unknown", "display_domain": "visible range/options/current value style", "tick_labels": ["literal tick/axis labels if visible"], "scale_hint": "linear|log|stepped|bipolar|unipolar|unknown", "display_style": "knob|slider|fader|switch|button|menu|graph_node|meter|numeric_readout|unknown", "ui_region": "top-left/center/bottom panel/etc", "state": "active|selected|disabled|dimmed|unknown", "evidence_tile_or_region": "visible image region such as left panel/center graph/top-right", "confidence": 0.0}],
  "extra_visible_core_controls": [{"label": "important visible processing control not requested", "group": "visible section", "unit": "visible unit if any", "reason": "why it matters"}],
  "layout": {"summary": "short layout description", "notable_regions": ["major processing regions only"]},
  "confidence": 0.0,
  "warnings": ["uncertainties, unreadable text, or if the image does not look like the target plugin"]
}

Rules:
- First inspect the requested targets; do not spend effort listing every visible knob/button.
- target_results is required and must include exactly one row for each supplied target_id.
- It is acceptable and useful to return status "not_visible" or "uncertain".
- Do not output backend param_id mappings. Backend candidates are attention aids only.
- Focus on display-domain evidence: visible units, readouts, tick labels, ranges, scale shape, graph axes, and state.
- Use plugin type and web evidence as attention priors only. The image is the source of visible UI evidence.
- Do not infer hidden controls or invisible ranges. Prefer uncertainty over guessing.
- Stop immediately after the JSON object.`, target.TrackID, target.PluginID, target.PluginName, string(targetFocusJSON), string(evidenceJSON))
	resp, err := s.completePluginUIReferenceImageUnderstanding(ctx, cfg, llm.VisionRequest{
		Prompt: prompt,
		Images: inputs,
		Metadata: llm.RequestMetadata{
			Source: "plugin_learning_visual_evidence",
		},
	})
	if err != nil {
		return nil, "Image understanding failed while probing type-guided plugin interface targets; continuing without visual digest: " + err.Error(), err
	}
	digest, err := parsePluginUIReferenceTargetProbe(resp.Text)
	if err != nil {
		return nil, "Image understanding returned an invalid type-guided plugin interface probe; continuing without it: " + err.Error(), err
	}
	if len(mapRowsValue(digest["target_results"])) == 0 {
		digest["matching_status"] = "target_probe_empty"
		appendPluginUIReferenceWarning(digest, "vision_probe_empty_targets: image understanding returned no target_results rows.")
	} else if firstNonEmptyText(digest, "matching_status") == "" {
		digest["matching_status"] = "target_probe_observed"
	}
	return digest, "", nil
}

func (s *Server) buildPluginUIReferenceImageExtract(ctx context.Context, cfg config.EngineConfig, target pluginLearningTarget, inputs []llm.ImageInput, focusSummary map[string]any, learningEvidence map[string]any) (map[string]any, string, error) {
	focusJSON, err := json.MarshalIndent(focusSummary, "", "  ")
	if err != nil {
		return nil, "Could not prepare backend controllable parameter focus summary for plugin interface image extraction; continuing without visual digest.", err
	}
	evidenceJSON, err := json.MarshalIndent(learningEvidence, "", "  ")
	if err != nil {
		return nil, "Could not prepare plugin learning evidence context for plugin interface image extraction; continuing without visual digest.", err
	}
	prompt := fmt.Sprintf(`The provided image is a plugin interface pattern for the current target DAW plugin.

Target plugin:
- track_id: %s
- plugin_id: %s
- plugin_name: %s

Backend controllable parameter visual focus summary from get_plugin_parameters:
%s

Optional learning evidence context (plugin type hypothesis, public docs digest, and policies):
%s

Build a detailed visual inventory of the plugin UI. The focus summary and learning evidence are attention priors only; they are not proof that a control appears in the image. The image is the source of visible UI evidence. This must be generic across audio effects and instruments: do not assume the plugin is an EQ, compressor, reverb, delay, modulation effect, saturator, synth, or utility unless the UI itself makes that clear.

Inspect the whole image before answering. Do not stop at the title bar, preset menu, transport/header controls, or first readable label. Prioritize the plugin's signal-processing body: main panels, module strips, graphs, draggable nodes/handles, knobs, sliders, faders, switches, buttons, menus, tabs, numeric readouts, meters with adjacent controls, and visible labels. Header/preset/global controls can be included, but they must not crowd out the main processing controls.

For repeated modules or channels, record each visible instance separately when position, index, color, selection, or label distinguishes it. For graph-based UIs, record visible handles/nodes/markers and any visible axis labels or scales as controls/readouts without assuming what they mean beyond the visible text/geometry. Include dimmed, disabled, greyed, or inactive controls when visible, and mark their state instead of ignoring them.

	Return ONLY one JSON object. Do not use markdown fences. Do not add prose before or after the JSON.
	{
	  "schema": "plugin_ui_reference_image_extract.v1",
	  "visible_controls": [{"label": "literal visible label/readout", "value_text": "current visible value if any", "display_value": "visible numeric/text display value if any", "control_type": "knob|slider|fader|button|switch|toggle|menu|tab|graph_node|meter|text|unknown", "control_kind": "continuous_parameter|discrete_parameter|toggle_parameter|meter|template_or_preset|navigation|decorative|unknown", "semantic_class": "generic role inferred from visible evidence, e.g. gain/time/rate/mix/tone/enable/mode/unknown", "ui_salience": "primary|secondary|utility|background|unknown", "group": "visible section/module/channel label if any", "region": "title/header/main graph/bottom panel/left panel/right panel/etc", "position": "short location such as top-left/center/lower-right", "state": "active|selected|disabled|dimmed|unknown", "unit": "Hz|dB|%%|ms|s|note|ratio|toggle|unknown", "display_unit": "literal unit shown near the value if any", "visible_range": "visible min/max or span if shown", "tick_labels": ["literal tick/axis labels if visible"], "scale_hint": "linear|log|stepped|bipolar|unipolar|unknown", "range_or_options": "visible range/options if shown", "is_parameter_like": true, "is_template_or_navigation_like": false, "visual_role_hint": "short UI role inferred from visible text/layout only", "confidence": 0.0}],
	  "groups": [{"label": "visible group/module/channel label", "description": "short visual grouping/layout notes", "region": "where it appears", "confidence": 0.0}],
	  "units_and_display_domains": [{"label": "visible label or readout", "unit": "Hz|dB|%%|ms|s|note|ratio|toggle|unknown", "range_or_values": "visible range/options/current value style if shown", "group": "visible section/module/channel label if any", "confidence": 0.0}],
	  "possible_focus_hits": [{"visible_label": "visible UI label/readout", "visible_group": "visible group/module/channel", "visible_unit": "Hz|dB|%%|ms|s|ratio|toggle|unknown", "focus_term": "term or group from the focus summary", "candidate_param_ids": ["representative ids copied from the focus summary only"], "reason": "short visible evidence, not a final mapping", "confidence": 0.0}],
	  "extra_visible_controls": [{"label": "visible UI text/control not predicted by the focus summary", "group": "visible section", "reason": "why it appears relevant"}],
	  "layout": {"summary": "short layout description", "notable_regions": ["short notes about major UI regions"]},
	  "confidence": 0.0,
	  "warnings": ["uncertainties, occlusion, unreadable text, or if the image does not look like the target plugin"]
	}

	Rules:
	- This step is visual observation, not final parameter matching. The text-only matching step and final Plugin Skill validator decide final mappings.
	- possible_focus_hits may only use representative IDs that appear in the focus summary. Do not invent backend parameter IDs.
	- Do not claim a focus hit from backend terms alone; require visible UI evidence such as label, group, value/readout, unit, graph node, position, or state.
	- The Plugin Skill learning priority is how a parameter is displayed/read/constrained in the human UI: visible units, value text, tick labels, ranges, scales, and state. Names are secondary evidence.
	- Distinguish parameter-like controls from preset/template/navigation controls when visible, but do not discard either category; mark it explicitly.
	- Treat visually prominent processing controls as important evidence even when labels are unfamiliar or absent.
	- Report visible controls even when they are not suggested by the focus summary.
	- Do not infer hidden controls or invisible ranges. Prefer uncertainty over guessing.
	- Copy visible text literally when possible; use concise descriptions only when text is unreadable.
	- A busy plugin UI that yields only a preset/header/menu entry is probably under-extracted; revisit the main body and extract the visible processing controls/readouts.
	- Include enough detail for later matching accuracy, but keep this extraction compact. Limit visible_controls to 90 items, groups to 30 items, units_and_display_domains to 60 items, possible_focus_hits to 60 items, extra_visible_controls to 30 items, and warnings to 10 items.
	- Stop immediately after the JSON object.`, target.TrackID, target.PluginID, target.PluginName, string(focusJSON), string(evidenceJSON))
	resp, err := s.completePluginUIReferenceImageUnderstanding(ctx, cfg, llm.VisionRequest{
		Prompt: prompt,
		Images: inputs,
		Metadata: llm.RequestMetadata{
			Source: "plugin_ui_reference_image_extract",
		},
	})
	if err != nil {
		return nil, "Image understanding failed while extracting plugin interface image pattern; continuing without visual digest: " + err.Error(), err
	}
	digest, err := parsePluginUIReferenceImageExtract(resp.Text)
	if err != nil {
		return nil, "Image understanding returned an invalid plugin interface image extraction; continuing without it: " + err.Error(), err
	}
	return digest, "", nil
}

func (s *Server) buildPluginUIReferenceParameterMatches(ctx context.Context, cfg config.EngineConfig, target pluginLearningTarget, imageDigest map[string]any, parameterDigest pluginParameterDigest, learningEvidence map[string]any) (map[string]any, string, error) {
	if s == nil || s.llm == nil {
		err := llm.ErrImageUnderstandingRouteUnavailable
		return nil, "Text matching route is unavailable; continuing with image-only plugin interface evidence.", err
	}
	backendCandidates := pluginUIReferenceBackendCandidateDigest(parameterDigest)
	focusSummary := pluginUIReferenceVisualFocusSummary(parameterDigest)
	batches := pluginUIReferenceMatchBatches(imageDigest, backendCandidates, focusSummary, learningEvidence)
	if len(batches) == 0 {
		return map[string]any{
			"schema":          "plugin_ui_reference_match.v1",
			"matching_status": "no_visible_controls",
			"warnings":        []string{"No visible plugin UI controls were available for visual/backend matching."},
		}, "", nil
	}
	evidenceJSON, err := json.MarshalIndent(pluginUIReferenceCompactLearningEvidence(learningEvidence), "", "  ")
	if err != nil {
		return nil, "Could not prepare plugin learning evidence context for visual matching; continuing with image-only plugin interface evidence.", err
	}
	system := `You align extracted DAW plugin UI information with backend controllable parameters.
Return ONLY strict JSON.`
	digests := make([]map[string]any, 0, len(batches))
	batchSummaries := make([]map[string]any, 0, len(batches))
	warnings := []string{}
	var firstErr error
	for _, batch := range batches {
		batchJSON, err := json.MarshalIndent(batch.PromptInput, "", "  ")
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			warnings = append(warnings, fmt.Sprintf("Batch %s could not be prepared: %s", batch.ID, compactPluginLearningError(err)))
			batchSummaries = append(batchSummaries, pluginUIReferenceMatchBatchSummary(batch, "prepare_failed", err))
			continue
		}
		user := fmt.Sprintf(`The image has already been converted into text. Do not ask for or use image input in this step.

Use the visual extraction as evidence for labels, units, grouping, layout, and display-domain hints. Use the backend candidate list as the only legal source of controllable parameter IDs.
This is one small matching batch from a larger Plugin Skill learning session. Candidate filtering is only an attention aid; it is not a final decision and it does not change the complete backend parameter ledger used by the validator.

Target plugin:
- track_id: %s
- plugin_id: %s
- plugin_name: %s

Batch input:
%s

Optional learning evidence context:
%s

Return ONLY strict JSON with this schema:
{
  "schema": "plugin_ui_reference_match.v1",
  "candidate_parameter_matches": [{"visible_label": "UI label/readout", "visible_group": "UI section/band", "visible_unit": "Hz|dB|%%|ms|ratio|toggle|unknown", "display_domain": "visible display range/value style if any", "ui_salience": "primary|secondary|utility|background|unknown", "semantic_class": "role inferred from combined evidence", "control_kind": "continuous_parameter|discrete_parameter|toggle_parameter|unknown", "backend_param_id": "must be one of the supplied candidate param_id values", "backend_param_name": "candidate name", "match_reason": "short reason using visual and backend evidence", "evidence": ["ui_label", "backend_name", "display_probe"], "confidence": 0.0}],
  "ambiguous_matches": [{"visible_label": "UI label", "candidate_param_ids": ["id1", "id2"], "reason": "why ambiguous"}],
  "unmatched_visible_controls": [{"label": "visible text", "reason": "not enough backend evidence or likely non-controllable"}],
  "unmatched_backend_parameters": [{"param_id": "candidate id", "reason": "not visible or not enough evidence"}],
  "conflicts": [{"param_id": "candidate id", "issue": "visual unit conflicts with backend/display probe"}],
  "confidence": 0.0,
  "warnings": ["uncertainties or matching limitations"]
}

Rules:
- Do not invent plugin parameter IDs. backend_param_id MUST be copied from the supplied candidate list.
- Do not output a candidate_parameter_match unless visual UI evidence and backend candidate evidence are plausibly connected.
- Prioritize display-domain evidence: visible units, value text, tick labels, ranges, scales, and state. Names are secondary evidence.
- Prefer exact/near label matches, group/module matches, unit/display-domain matches, display_probe matches, and cautious audio-control synonyms.
- Use plugin type/core-control expectations as priors for what deserves attention, not as hard requirements and not as proof of a mapping.
- Preset/template/navigation controls should usually stay unmatched unless a backend parameter clearly represents that choice.
- Use possible_focus_hits as hints, not proof; verify against visible_controls and backend candidates before matching.
- If a visible UI control is clearly present but no backend candidate is safe, put it in unmatched_visible_controls.
- If multiple backend candidates could match one visible control, put it in ambiguous_matches unless one is clearly best.
- The final Plugin Skill validator will decide final mappings; you only provide evidence and confidence.
- Prefer uncertainty over guessing.
- Keep match_reason under 160 characters and limit candidate_parameter_matches to the most useful 24 matches for this batch.`, target.TrackID, target.PluginID, target.PluginName, string(batchJSON), string(evidenceJSON))
		assembly := promptruntime.Build(promptruntime.AssemblyInput{
			SystemSections: []promptruntime.Section{
				promptruntime.TextSection(promptruntime.SectionStatic, "plugin_ui_reference_match_system", "", system, true),
			},
			UserSections: []promptruntime.Section{
				promptruntime.TextSection(promptruntime.SectionRuntime, "plugin_ui_reference_match_runtime_"+batch.ID, "", user, false),
			},
		})
		req := llm.Request{
			Messages: assembly.Messages,
			Timeout:  pluginLearningMatchTimeout,
			Metadata: llm.RequestMetadata{
				Source:            "plugin_ui_reference_match",
				PromptFingerprint: assembly.Fingerprint,
				PromptStats:       assembly.Stats.Map(),
			},
		}
		digest, err := s.completePluginUIReferenceMatchBatch(ctx, cfg, req)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			warnings = append(warnings, fmt.Sprintf("Batch %s (%s) visual/backend matching failed: %s", batch.ID, batch.Label, compactPluginLearningError(err)))
			batchSummaries = append(batchSummaries, pluginUIReferenceMatchBatchSummary(batch, "failed", err))
			continue
		}
		digest["batch_id"] = batch.ID
		digest["batch_label"] = batch.Label
		digests = append(digests, digest)
		batchSummaries = append(batchSummaries, pluginUIReferenceMatchBatchSummary(batch, "matched", nil))
	}
	merged := pluginUIReferenceMergeMatchBatchDigests(parameterDigest, batches, digests, batchSummaries, warnings)
	if len(digests) > 0 {
		if len(warnings) > 0 {
			return merged, "Some visual/backend matching batches failed; continuing with partial plugin interface matches.", nil
		}
		return merged, "", nil
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("all plugin UI reference matching batches failed")
	}
	return nil, fmt.Sprintf("Text-only visual/backend matching failed for all %d batches; continuing with image-only plugin interface evidence: %s", len(batches), compactPluginLearningError(firstErr)), firstErr
}

type pluginUIReferenceMatchBatch struct {
	ID                     string
	Label                  string
	PromptInput            map[string]any
	VisibleControlCount    int
	CandidateCount         int
	OmittedCandidateCount  int
	SourceCandidateCount   int
	PossibleFocusHitCount  int
	UnitsDisplayRowCount   int
	ExtraVisibleRowCount   int
	EstimatedPromptPayload int
}

func (s *Server) completePluginUIReferenceMatchBatch(ctx context.Context, cfg config.EngineConfig, req llm.Request) (map[string]any, error) {
	var lastParseErr error
	for attempt := 0; attempt < pluginUIReferenceParseRetryAttempts(); attempt++ {
		resp, err := s.completePluginUIReferenceRequest(ctx, cfg, req)
		if err != nil {
			return nil, err
		}
		digest, err := parsePluginUIReferenceMatchDigest(resp.Text)
		if err == nil {
			return digest, nil
		}
		lastParseErr = err
		if !pluginUIReferenceRetryableParseFailure(ctx, resp.Text, err) {
			break
		}
	}
	if lastParseErr == nil {
		lastParseErr = fmt.Errorf("plugin UI reference matching returned no parseable JSON")
	}
	return nil, lastParseErr
}

func pluginUIReferenceMatchBatches(imageDigest, backendCandidates, focusSummary, learningEvidence map[string]any) []pluginUIReferenceMatchBatch {
	controls := mapRowsValue(imageDigest["visible_controls"])
	if len(controls) == 0 {
		controls = mapRowsValue(imageDigest["possible_focus_hits"])
	}
	if len(controls) == 0 {
		return nil
	}
	candidateRows := mapRowsValue(backendCandidates["candidate_parameters"])
	groups := pluginUIReferenceControlGroups(controls)
	out := make([]pluginUIReferenceMatchBatch, 0, len(groups))
	for groupIndex, group := range groups {
		chunks := pluginUIReferenceChunkRows(group.Rows, 12)
		for chunkIndex, chunk := range chunks {
			label := group.Label
			if len(chunks) > 1 {
				label = fmt.Sprintf("%s part %d", label, chunkIndex+1)
			}
			batch := pluginUIReferenceBuildMatchBatch(groupIndex, chunkIndex, label, chunk, imageDigest, candidateRows, backendCandidates, focusSummary, learningEvidence)
			out = append(out, batch)
		}
	}
	return out
}

type pluginUIReferenceControlGroup struct {
	Key   string
	Label string
	Rows  []map[string]any
}

func pluginUIReferenceControlGroups(controls []map[string]any) []pluginUIReferenceControlGroup {
	index := map[string]int{}
	out := []pluginUIReferenceControlGroup{}
	for i, row := range controls {
		label := firstNonEmptyText(row, "group", "section", "region", "ui_salience")
		if label == "" || strings.EqualFold(label, "unknown") {
			label = fmt.Sprintf("visible controls %d", len(out)+1)
		}
		key := pluginLearningSafeArtifactToken(label)
		if key == "unknown" {
			key = fmt.Sprintf("visible_%d", i+1)
		}
		pos, ok := index[key]
		if !ok {
			pos = len(out)
			index[key] = pos
			out = append(out, pluginUIReferenceControlGroup{Key: key, Label: label})
		}
		out[pos].Rows = append(out[pos].Rows, row)
	}
	return out
}

func pluginUIReferenceChunkRows(rows []map[string]any, limit int) [][]map[string]any {
	if limit <= 0 || len(rows) <= limit {
		return [][]map[string]any{rows}
	}
	out := [][]map[string]any{}
	for start := 0; start < len(rows); start += limit {
		end := start + limit
		if end > len(rows) {
			end = len(rows)
		}
		out = append(out, rows[start:end])
	}
	return out
}

func pluginUIReferenceBuildMatchBatch(groupIndex, chunkIndex int, label string, controls []map[string]any, imageDigest map[string]any, candidateRows []map[string]any, backendCandidates, focusSummary, learningEvidence map[string]any) pluginUIReferenceMatchBatch {
	batchID := fmt.Sprintf("batch_%02d_%02d_%s", groupIndex+1, chunkIndex+1, pluginLearningSafeArtifactToken(label))
	batchTerms := pluginUIReferenceTermsFromRows(controls, "label", "name", "text", "value_text", "display_value", "group", "section", "region", "unit", "display_unit", "visible_range", "range_or_options", "semantic_class", "control_kind", "visual_role_hint")
	unitsRows := pluginUIReferenceRelatedRows(mapRowsValue(imageDigest["units_and_display_domains"]), batchTerms, 14, "label", "group", "section", "unit", "range_or_values")
	focusRows := pluginUIReferenceRelatedRows(mapRowsValue(imageDigest["possible_focus_hits"]), batchTerms, 18, "visible_label", "visible_group", "visible_unit", "focus_term", "reason")
	extraRows := pluginUIReferenceRelatedRows(mapRowsValue(imageDigest["extra_visible_controls"]), batchTerms, 10, "label", "group", "reason")
	groupRows := pluginUIReferenceRelatedRows(mapRowsValue(imageDigest["groups"]), batchTerms, 8, "label", "name", "description", "region")
	candidates, omitted := pluginUIReferenceCandidateRowsForBatch(batchTerms, focusRows, candidateRows, 36)
	promptInput := map[string]any{
		"schema": "plugin_ui_reference_match_batch_input.v1",
		"batch": map[string]any{
			"id":                      batchID,
			"label":                   label,
			"visible_control_count":   len(controls),
			"candidate_count":         len(candidates),
			"source_candidate_count":  len(candidateRows),
			"omitted_candidate_count": omitted,
			"candidate_policy":        "Only listed candidate_parameters are legal for this batch. Candidate filtering is a weak attention filter; final validation still uses the complete backend parameter ledger.",
		},
		"visual_extraction_schema": firstNonEmpty(firstNonEmptyText(imageDigest, "schema"), "plugin_ui_reference_image_extract.v1"),
		"visible_controls":         pluginLearningCompactRows(controls, 12, 120, "label", "value_text", "display_value", "control_type", "control_kind", "semantic_class", "ui_salience", "group", "region", "position", "state", "unit", "display_unit", "visible_range", "tick_labels", "scale_hint", "range_or_options", "is_parameter_like", "is_template_or_navigation_like", "visual_role_hint", "confidence"),
		"groups":                   pluginLearningCompactRows(groupRows, 8, 160, "label", "name", "description", "region", "confidence"),
		"units_and_display_domains": pluginLearningCompactRows(unitsRows, 14, 120,
			"label", "unit", "range_or_values", "group", "confidence"),
		"possible_focus_hits":    pluginLearningCompactRows(focusRows, 18, 120, "visible_label", "visible_group", "visible_unit", "focus_term", "candidate_param_ids", "reason", "confidence"),
		"extra_visible_controls": pluginLearningCompactRows(extraRows, 10, 120, "label", "group", "reason"),
		"visual_focus_summary":   pluginUIReferenceFocusSummaryForBatch(focusSummary, batchTerms),
		"backend_parameter_candidates": map[string]any{
			"schema":                  "plugin_backend_parameter_candidates.batch.v1",
			"parameter_count":         backendCandidates["parameter_count"],
			"source_candidate_count":  len(candidateRows),
			"candidate_count":         len(candidates),
			"omitted_candidate_count": omitted,
			"candidate_parameters":    candidates,
			"param_id_source_policy":  "backend_param_id values must be copied from candidate_parameters[].param_id only.",
		},
	}
	data, _ := json.Marshal(promptInput)
	return pluginUIReferenceMatchBatch{
		ID:                     batchID,
		Label:                  label,
		PromptInput:            promptInput,
		VisibleControlCount:    len(controls),
		CandidateCount:         len(candidates),
		OmittedCandidateCount:  omitted,
		SourceCandidateCount:   len(candidateRows),
		PossibleFocusHitCount:  len(focusRows),
		UnitsDisplayRowCount:   len(unitsRows),
		ExtraVisibleRowCount:   len(extraRows),
		EstimatedPromptPayload: len(data),
	}
}

func pluginUIReferenceTermsFromRows(rows []map[string]any, keys ...string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(values ...string) {
		for _, term := range pluginUIReferenceTerms(values...) {
			if !seen[term] {
				seen[term] = true
				out = append(out, term)
			}
		}
	}
	for _, row := range rows {
		for _, key := range keys {
			add(firstNonEmptyText(row, key))
			for _, value := range stringListValue(row[key]) {
				add(value)
			}
		}
	}
	return out
}

func pluginUIReferenceRelatedRows(rows []map[string]any, batchTerms []string, limit int, keys ...string) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	type scored struct {
		row   map[string]any
		score int
		index int
	}
	scoredRows := []scored{}
	for i, row := range rows {
		terms := pluginUIReferenceTermsFromRows([]map[string]any{row}, keys...)
		score := pluginUIReferenceTermOverlapScore(batchTerms, terms)
		if score == 0 && len(scoredRows) > 0 {
			continue
		}
		scoredRows = append(scoredRows, scored{row: row, score: score, index: i})
	}
	sort.SliceStable(scoredRows, func(i, j int) bool {
		if scoredRows[i].score == scoredRows[j].score {
			return scoredRows[i].index < scoredRows[j].index
		}
		return scoredRows[i].score > scoredRows[j].score
	})
	if limit >= 0 && len(scoredRows) > limit {
		scoredRows = scoredRows[:limit]
	}
	out := make([]map[string]any, 0, len(scoredRows))
	for _, row := range scoredRows {
		out = append(out, row.row)
	}
	return out
}

func pluginUIReferenceCandidateRowsForBatch(batchTerms []string, focusRows, candidateRows []map[string]any, limit int) ([]map[string]any, int) {
	if limit <= 0 || len(candidateRows) <= limit {
		return pluginUIReferenceCompactCandidateRows(candidateRows), 0
	}
	focusIDs := map[string]bool{}
	for _, row := range focusRows {
		for _, id := range stringListValue(row["candidate_param_ids"]) {
			if strings.TrimSpace(id) != "" {
				focusIDs[strings.TrimSpace(id)] = true
			}
		}
	}
	type scored struct {
		row   map[string]any
		score int
		index int
	}
	scoredRows := make([]scored, 0, len(candidateRows))
	for i, row := range candidateRows {
		id := firstNonEmptyText(row, "param_id", "id")
		terms := pluginUIReferenceCandidateTerms(row)
		score := pluginUIReferenceTermOverlapScore(batchTerms, terms) * 4
		if focusIDs[id] {
			score += 100
		}
		if boolValue(row["quick_control"]) {
			score += 5
		}
		if boolValue(row["host_controllable"]) {
			score += 2
		}
		if firstNonEmptyText(row, "control_relevance") != "" {
			score += 2
		}
		if pluginUIReferenceUnitTermsOverlap(batchTerms, terms) {
			score += 12
		}
		scoredRows = append(scoredRows, scored{row: row, score: score, index: i})
	}
	sort.SliceStable(scoredRows, func(i, j int) bool {
		if scoredRows[i].score == scoredRows[j].score {
			return scoredRows[i].index < scoredRows[j].index
		}
		return scoredRows[i].score > scoredRows[j].score
	})
	if len(scoredRows) > limit {
		scoredRows = scoredRows[:limit]
	}
	out := make([]map[string]any, 0, len(scoredRows))
	for _, row := range scoredRows {
		out = append(out, row.row)
	}
	return pluginUIReferenceCompactCandidateRows(out), len(candidateRows) - len(out)
}

func pluginUIReferenceCandidateTerms(row map[string]any) []string {
	values := []string{}
	for _, key := range []string{"param_id", "id", "name", "raw_name", "alias", "display_group", "normalized_role", "control_relevance", "unit_hint", "current_value_text"} {
		values = append(values, firstNonEmptyText(row, key))
	}
	if probe := mapValue(row["display_probe"]); len(probe) > 0 {
		for _, key := range []string{"current_text", "label"} {
			values = append(values, firstNonEmptyText(probe, key))
		}
		for _, sample := range mapRowsValue(probe["samples"]) {
			values = append(values, firstNonEmptyText(sample, "text", "label"))
		}
		for _, label := range mapRowsValue(probe["discrete_labels"]) {
			values = append(values, firstNonEmptyText(label, "label", "text"))
		}
		values = append(values, stringListValue(probe["labels"])...)
	}
	if domain := mapValue(row["display_domain_candidate"]); len(domain) > 0 {
		values = append(values, firstNonEmptyText(domain, "text", "unit", "scale", "source"))
	}
	return pluginUIReferenceTerms(values...)
}

func pluginUIReferenceTermOverlapScore(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := map[string]bool{}
	for _, term := range b {
		if len(term) >= 2 {
			set[term] = true
		}
	}
	score := 0
	for _, term := range a {
		if set[term] {
			score++
		}
	}
	return score
}

func pluginUIReferenceUnitTermsOverlap(a, b []string) bool {
	units := map[string]bool{"hz": true, "khz": true, "db": true, "ms": true, "s": true, "%": true, "percent": true, "ratio": true, "toggle": true}
	aSet := map[string]bool{}
	for _, term := range a {
		if units[strings.ToLower(term)] {
			aSet[strings.ToLower(term)] = true
		}
	}
	for _, term := range b {
		if aSet[strings.ToLower(term)] {
			return true
		}
	}
	return false
}

func pluginUIReferenceCompactCandidateRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := pluginLearningCompactRow(row, 120, "param_id", "name", "raw_name", "alias", "display_group", "normalized_role", "control_relevance", "quick_control", "host_controllable", "is_boolean", "is_discrete", "unit_hint", "current_value_text", "backend_range")
		if domain := mapValue(row["display_domain_candidate"]); len(domain) > 0 {
			next["display_domain_candidate"] = pluginLearningCompactRow(domain, 100, "text", "unit", "min", "max", "scale", "confidence", "source", "status")
		}
		if probe := mapValue(row["display_probe"]); len(probe) > 0 {
			compactProbe := pluginLearningCompactRow(probe, 100, "mode", "current_text", "label", "capabilities")
			if samples := mapRowsValue(probe["samples"]); len(samples) > 0 {
				compactProbe["samples"] = pluginLearningCompactRows(samples, 3, 80, "normalized_value", "value", "text")
			}
			if labels := mapRowsValue(probe["discrete_labels"]); len(labels) > 0 {
				compactProbe["discrete_labels"] = pluginLearningCompactRows(labels, 5, 80, "index", "value", "label")
			}
			if labels := stringListValue(probe["labels"]); len(labels) > 0 {
				compactProbe["labels"] = pluginLearningCompactStringList(labels, 5, 80)
			}
			if len(compactProbe) > 0 {
				next["display_probe"] = compactProbe
			}
		}
		if len(next) > 0 {
			out = append(out, next)
		}
	}
	return out
}

func pluginUIReferenceFocusSummaryForBatch(focusSummary map[string]any, batchTerms []string) map[string]any {
	if len(focusSummary) == 0 {
		return nil
	}
	return map[string]any{
		"schema":                 "plugin_ui_reference_visual_focus_summary.batch.v1",
		"source_schema":          firstNonEmptyText(focusSummary, "schema"),
		"source_parameter_count": focusSummary["source_parameter_count"],
		"parameters_retained":    focusSummary["parameters_retained"],
		"dropped_param_count":    focusSummary["dropped_param_count"],
		"role_summary":           pluginUIReferenceRelatedRows(mapRowsValue(focusSummary["role_summary"]), batchTerms, 10, "text", "candidate_param_ids"),
		"unit_summary":           pluginUIReferenceRelatedRows(mapRowsValue(focusSummary["unit_summary"]), batchTerms, 10, "text", "candidate_param_ids"),
		"group_summary":          pluginUIReferenceRelatedRows(mapRowsValue(focusSummary["group_summary"]), batchTerms, 12, "text", "candidate_param_ids"),
		"visual_search_terms":    pluginUIReferenceRelatedRows(mapRowsValue(focusSummary["visual_search_terms"]), batchTerms, 20, "text", "candidate_param_ids"),
		"focus_summary_policy":   "Batch focus is a compact attention guide only; final param_id mappings must use backend candidates and validator.",
	}
}

func pluginUIReferenceCompactLearningEvidence(learningEvidence map[string]any) map[string]any {
	if len(learningEvidence) == 0 {
		return nil
	}
	out := map[string]any{
		"schema": "plugin_learning_evidence_context.compact.v1",
		"policy": firstNonEmpty(firstNonEmptyText(learningEvidence, "policy"),
			"Evidence may guide attention and confidence, but final param_id authority remains get_plugin_parameters plus the Plugin Skill validator."),
	}
	if typeHypothesis := mapValue(learningEvidence["plugin_type_hypothesis"]); len(typeHypothesis) > 0 {
		out["plugin_type_hypothesis"] = pluginLearningCompactTypeHypothesis(typeHypothesis)
	}
	if webReference := mapValue(learningEvidence["plugin_web_reference"]); len(webReference) > 0 {
		out["plugin_web_reference"] = pluginLearningCompactWebReference(webReference)
	}
	for key, value := range learningEvidence {
		if key == "plugin_type_hypothesis" || key == "plugin_web_reference" || key == "policy" {
			continue
		}
		if rows := mapRowsValue(value); len(rows) > 0 {
			out[key] = pluginLearningCompactRows(rows, 12, 160, "kind", "summary", "control", "label", "type", "unit", "range_or_values", "confidence")
			continue
		}
		if m := mapValue(value); len(m) > 0 {
			out[key] = pluginLearningCompactRow(m, 160, "schema", "status", "summary", "primary_type", "confidence")
			continue
		}
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
			out[key] = pluginLearningPromptClip(text, 220)
		}
	}
	return out
}

func pluginUIReferenceMatchBatchSummary(batch pluginUIReferenceMatchBatch, status string, err error) map[string]any {
	out := map[string]any{
		"id":                       batch.ID,
		"label":                    batch.Label,
		"status":                   status,
		"visible_control_count":    batch.VisibleControlCount,
		"candidate_count":          batch.CandidateCount,
		"source_candidate_count":   batch.SourceCandidateCount,
		"omitted_candidate_count":  batch.OmittedCandidateCount,
		"possible_focus_hit_count": batch.PossibleFocusHitCount,
		"units_display_row_count":  batch.UnitsDisplayRowCount,
		"extra_visible_row_count":  batch.ExtraVisibleRowCount,
		"estimated_payload_bytes":  batch.EstimatedPromptPayload,
	}
	if err != nil {
		out["error"] = compactPluginLearningError(err)
	}
	return out
}

func pluginUIReferenceMergeMatchBatchDigests(parameterDigest pluginParameterDigest, batches []pluginUIReferenceMatchBatch, digests []map[string]any, batchSummaries []map[string]any, warnings []string) map[string]any {
	out := map[string]any{
		"schema":                 "plugin_ui_reference_match.v1",
		"matching_mode":          "batched",
		"batch_count":            len(batches),
		"successful_batch_count": len(digests),
		"failed_batch_count":     len(batches) - len(digests),
		"match_batch_summaries":  batchSummaries,
	}
	if len(digests) == 0 {
		out["matching_status"] = "match_failed"
		out["warnings"] = compactStringList(warnings)
		return out
	}
	if len(digests) == len(batches) {
		out["matching_status"] = "matched"
	} else {
		out["matching_status"] = "partial_matched"
	}
	matchesByKey := map[string]map[string]any{}
	matchOrder := []string{}
	ambiguous := []map[string]any{}
	unmatchedVisible := []map[string]any{}
	conflicts := []map[string]any{}
	allWarnings := append([]string{}, warnings...)
	confidenceSum := 0.0
	confidenceCount := 0
	for _, digest := range digests {
		batchID := firstNonEmptyText(digest, "batch_id")
		batchLabel := firstNonEmptyText(digest, "batch_label")
		if c := floatNumber(digest["confidence"]); c > 0 {
			confidenceSum += c
			confidenceCount++
		}
		for _, row := range mapRowsValue(digest["candidate_parameter_matches"]) {
			paramID := firstNonEmptyText(row, "backend_param_id", "param_id")
			if paramID == "" {
				continue
			}
			next := copyStringAnyMap(row)
			if batchID != "" {
				next["match_batch_id"] = batchID
			}
			if batchLabel != "" {
				next["match_batch_label"] = batchLabel
			}
			key := strings.Join([]string{paramID, firstNonEmptyText(next, "visible_label", "label"), firstNonEmptyText(next, "visible_group", "group")}, "\x00")
			if existing, ok := matchesByKey[key]; ok {
				if floatNumber(next["confidence"]) <= floatNumber(existing["confidence"]) {
					continue
				}
			} else {
				matchOrder = append(matchOrder, key)
			}
			matchesByKey[key] = next
		}
		ambiguous = append(ambiguous, pluginUIReferenceRowsWithBatch(mapRowsValue(digest["ambiguous_matches"]), batchID, batchLabel)...)
		unmatchedVisible = append(unmatchedVisible, pluginUIReferenceRowsWithBatch(mapRowsValue(digest["unmatched_visible_controls"]), batchID, batchLabel)...)
		conflicts = append(conflicts, pluginUIReferenceRowsWithBatch(mapRowsValue(digest["conflicts"]), batchID, batchLabel)...)
		allWarnings = append(allWarnings, stringListValue(digest["warnings"])...)
	}
	matches := make([]map[string]any, 0, len(matchOrder))
	matchedIDs := map[string]bool{}
	for _, key := range matchOrder {
		row := matchesByKey[key]
		matches = append(matches, row)
		if id := firstNonEmptyText(row, "backend_param_id", "param_id"); id != "" {
			matchedIDs[id] = true
		}
	}
	out["candidate_parameter_matches"] = matches
	if len(ambiguous) > 0 {
		out["ambiguous_matches"] = pluginUIReferenceLimitRows(ambiguous, 80)
	}
	if len(unmatchedVisible) > 0 {
		out["unmatched_visible_controls"] = pluginUIReferenceLimitRows(unmatchedVisible, 80)
	}
	if len(conflicts) > 0 {
		out["conflicts"] = pluginUIReferenceLimitRows(conflicts, 60)
	}
	unmatchedBackend := []map[string]any{}
	for _, param := range parameterDigest.Parameters {
		id := strings.TrimSpace(param.ID)
		if id == "" || matchedIDs[id] {
			continue
		}
		row := map[string]any{"param_id": id, "reason": "not confidently matched by any visual/backend matching batch"}
		if param.Name != "" {
			row["name"] = param.Name
		}
		if param.DisplayGroup != "" {
			row["display_group"] = param.DisplayGroup
		}
		if param.NormalizedRole != "" {
			row["normalized_role"] = param.NormalizedRole
		}
		unmatchedBackend = append(unmatchedBackend, row)
	}
	if len(unmatchedBackend) > 0 {
		out["unmatched_backend_parameters"] = pluginUIReferenceLimitRows(unmatchedBackend, 120)
	}
	if confidenceCount > 0 {
		out["confidence"] = confidenceSum / float64(confidenceCount)
	}
	if len(matches) == 0 && len(digests) > 0 {
		out["matching_status"] = "no_confident_matches"
	}
	if len(allWarnings) > 0 {
		out["warnings"] = compactStringList(allWarnings)
	}
	return out
}

func pluginUIReferenceRowsWithBatch(rows []map[string]any, batchID, batchLabel string) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := copyStringAnyMap(row)
		if batchID != "" {
			next["match_batch_id"] = batchID
		}
		if batchLabel != "" {
			next["match_batch_label"] = batchLabel
		}
		out = append(out, next)
	}
	return out
}

func pluginUIReferenceLimitRows(rows []map[string]any, limit int) []map[string]any {
	if limit >= 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func parsePluginUIReferenceDigest(raw string) (map[string]any, error) {
	return parsePluginUIReferenceJSON(raw, "plugin_ui_reference_digest.v1")
}

func parsePluginUIReferenceImageExtract(raw string) (map[string]any, error) {
	return parsePluginUIReferenceJSON(raw, "plugin_ui_reference_image_extract.v1")
}

func parsePluginUIReferenceTargetProbe(raw string) (map[string]any, error) {
	return parsePluginUIReferenceJSON(raw, "plugin_ui_reference_target_probe.v1")
}

func parsePluginUIReferenceMatchDigest(raw string) (map[string]any, error) {
	return parsePluginUIReferenceJSON(raw, "plugin_ui_reference_match.v1")
}

func pluginUIReferenceParseRetryAttempts() int {
	return 2
}

func pluginUIReferenceRetryableParseFailure(ctx context.Context, raw string, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	return true
}

func parsePluginUIReferenceJSON(raw, defaultSchema string) (map[string]any, error) {
	text := strings.TrimSpace(raw)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err == nil && len(out) > 0 {
		out["schema"] = firstNonEmpty(firstNonEmptyText(out, "schema"), defaultSchema)
		return out, nil
	}
	if out, err := parseFirstPluginUIReferenceJSONObject(text); err == nil && len(out) > 0 {
		out["schema"] = firstNonEmpty(firstNonEmptyText(out, "schema"), defaultSchema)
		return out, nil
	}
	return nil, fmt.Errorf("invalid %s JSON; response_preview=%q", defaultSchema, pluginUIReferenceResponsePreview(text, 1200))
}

func parseFirstPluginUIReferenceJSONObject(text string) (map[string]any, error) {
	for offset := 0; offset < len(text); {
		relative := strings.Index(text[offset:], "{")
		if relative < 0 {
			break
		}
		start := offset + relative
		decoder := json.NewDecoder(strings.NewReader(text[start:]))
		decoder.UseNumber()
		var out map[string]any
		if err := decoder.Decode(&out); err == nil && len(out) > 0 {
			return out, nil
		}
		offset = start + 1
	}
	return nil, fmt.Errorf("no JSON object found")
}

func pluginUIReferenceResponsePreview(text string, max int) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func mergePluginUIReferenceDigests(imageDigest, matchDigest map[string]any) map[string]any {
	out := map[string]any{"schema": "plugin_ui_reference_digest.v1"}
	if len(imageDigest) > 0 {
		out["visual_extraction_schema"] = firstNonEmpty(firstNonEmptyText(imageDigest, "schema"), "plugin_ui_reference_image_extract.v1")
		for _, key := range []string{
			"visible_controls",
			"groups",
			"units_and_display_domains",
			"possible_focus_hits",
			"extra_visible_controls",
			"target_results",
			"extra_visible_core_controls",
			"candidate_parameter_matches",
			"ambiguous_matches",
			"layout",
			"confidence",
			"matching_status",
		} {
			if value := firstPresentAny(imageDigest, key); value != nil {
				out[key] = value
			}
		}
		if warnings := stringListValue(imageDigest["warnings"]); len(warnings) > 0 {
			out["warnings"] = warnings
		}
	}
	if len(matchDigest) > 0 {
		out["matching_schema"] = firstNonEmpty(firstNonEmptyText(matchDigest, "schema"), "plugin_ui_reference_match.v1")
		out["matching_status"] = firstNonEmpty(firstNonEmptyText(matchDigest, "matching_status"), "matched")
		for _, key := range []string{
			"matching_mode",
			"batch_count",
			"successful_batch_count",
			"failed_batch_count",
			"match_batch_summaries",
			"candidate_parameter_matches",
			"ambiguous_matches",
			"unmatched_visible_controls",
			"unmatched_backend_parameters",
			"conflicts",
		} {
			if value := firstPresentAny(matchDigest, key); value != nil {
				out[key] = value
			}
		}
		if value := firstPresentAny(matchDigest, "confidence"); value != nil {
			out["matching_confidence"] = value
		}
		warnings := append(stringListValue(out["warnings"]), stringListValue(matchDigest["warnings"])...)
		if len(warnings) > 0 {
			out["warnings"] = compactStringList(warnings)
		}
	} else {
		if firstNonEmptyText(out, "matching_status") == "" {
			if pluginUIReferenceHasCandidateMatches(out) {
				out["matching_status"] = "image_guided"
			} else {
				out["matching_status"] = "image_extract_only"
			}
		}
	}
	return out
}

func pluginUIReferenceHasCandidateMatches(visualDigest map[string]any) bool {
	return len(mapRowsValue(visualDigest["candidate_parameter_matches"])) > 0
}

func pluginUIReferenceVisualDigestHasUsableEvidence(visualDigest map[string]any) bool {
	if len(visualDigest) == 0 {
		return false
	}
	status := strings.TrimSpace(strings.ToLower(firstNonEmptyText(visualDigest, "matching_status", "status")))
	if status == "target_probe_empty" || status == "vision_empty" || status == "empty" {
		return false
	}
	if len(mapRowsValue(visualDigest["target_results"])) > 0 {
		return true
	}
	if len(mapRowsValue(visualDigest["candidate_parameter_matches"])) > 0 {
		return true
	}
	if len(mapRowsValue(visualDigest["visible_controls"])) > 0 {
		return true
	}
	if len(mapRowsValue(visualDigest["units_and_display_domains"])) > 0 {
		return true
	}
	return false
}

func appendPluginUIReferenceWarning(digest map[string]any, warning string) {
	if len(digest) == 0 || strings.TrimSpace(warning) == "" {
		return
	}
	digest["warnings"] = compactStringList(append(stringListValue(digest["warnings"]), warning))
}

func (s *Server) completePluginUIReferenceImageUnderstanding(ctx context.Context, cfg config.EngineConfig, req llm.VisionRequest) (llm.Response, error) {
	return retryPluginUIReferenceLLMCall(ctx, func() (llm.Response, error) {
		return s.llm.CompleteImageUnderstanding(ctx, cfg, req)
	})
}

func (s *Server) completePluginUIReferenceRequest(ctx context.Context, cfg config.EngineConfig, req llm.Request) (llm.Response, error) {
	return retryPluginUIReferenceLLMCall(ctx, func() (llm.Response, error) {
		return s.llm.CompleteRequest(ctx, cfg, req)
	})
}

func retryPluginUIReferenceLLMCall(ctx context.Context, run func() (llm.Response, error)) (llm.Response, error) {
	const attempts = 2
	var last llm.Response
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		last, err = run()
		if err == nil || attempt == attempts-1 || !pluginUIReferenceRetryableLLMError(ctx, err) {
			return last, err
		}
	}
	return last, err
}

func pluginUIReferenceRetryableLLMError(ctx context.Context, err error) bool {
	if err == nil || errors.Is(err, llm.ErrImageUnderstandingRouteUnavailable) || errors.Is(err, context.Canceled) {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"stream error",
		"internal_error",
		"internal error",
		"http error 502",
		"http error 503",
		"http error 504",
		"http error 524",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"connection reset",
		"connection aborted",
		"connection refused",
		"connection closed",
		"client connection lost",
		"server closed idle connection",
		"unexpected eof",
		"eof",
		"timeout",
		"timed out",
		"deadline exceeded",
		"temporarily unavailable",
		"temporary failure",
		"upstream request failed",
		"upstream_error",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func (s *Server) resolvePluginWebReference(ctx context.Context, cfg config.EngineConfig, args map[string]any, target pluginLearningTarget, digest pluginParameterDigest) map[string]any {
	out := map[string]any{
		"schema":   pluginLearningWebReferenceSchema,
		"status":   "skipped",
		"decision": pluginWebReferenceDecision(args),
		"policy":   "Public documentation is optional evidence for type/core-control discovery; it never creates final param_id mappings.",
	}
	if !pluginWebReferenceRequested(args) {
		out["decision"] = "skipped"
		return out
	}
	out["status"] = "searching"
	query := pluginWebReferenceSearchQuery(target, digest)
	if query == "" {
		out["status"] = "unavailable"
		out["warnings"] = []string{"No public-search-safe plugin name was available for web reference lookup."}
		return out
	}
	out["query"] = query
	hostedWarnings := []string{}
	if hosted, warning := s.buildPluginHostedWebReference(ctx, cfg, target, digest, query); len(hosted) > 0 {
		for key, value := range hosted {
			out[key] = value
		}
		out["schema"] = pluginLearningWebReferenceSchema
		out["decision"] = pluginWebReferenceDecision(args)
		out["query"] = query
		out["search_source"] = firstNonEmpty(firstNonEmptyText(out, "search_source"), "openai_hosted_web_search")
		return pluginWebReferencePublicDigest(out)
	} else if warning != "" {
		hostedWarnings = append(hostedWarnings, warning)
	}
	searchCtx, cancel := context.WithTimeout(ctx, 24*time.Second)
	defer cancel()
	searchResult, err := webtools.Search(searchCtx, map[string]any{
		"query":       query,
		"max_results": 6,
		"timeout_ms":  9000,
		"max_bytes":   320 * 1024,
	})
	if err != nil {
		out["status"] = "failed"
		out["warnings"] = compactStringList(append(hostedWarnings, "Web reference search failed: "+compactPluginLearningError(err)))
		return out
	}
	results := pluginWebReferenceRankedResults(mapRowsValue(searchResult["results"]), target, digest)
	if len(results) == 0 {
		out["status"] = "not_found"
		out["warnings"] = compactStringList(append(hostedWarnings, "Web reference search returned no useful public documentation candidates."))
		return out
	}
	out["search_source"] = firstNonEmptyText(searchResult, "source")
	out["search_results"] = pluginLearningCompactRows(results, 6, 260, "title", "url", "snippet", "web_reference_score")
	sources := make([]map[string]any, 0, 3)
	warnings := []string{}
	for _, result := range results {
		if len(sources) >= 3 {
			break
		}
		url := firstNonEmptyText(result, "url")
		if url == "" {
			continue
		}
		source := map[string]any{
			"title":       firstNonEmptyText(result, "title"),
			"url":         url,
			"snippet":     firstNonEmptyText(result, "snippet"),
			"source_rank": len(sources) + 1,
		}
		fetchCtx, fetchCancel := context.WithTimeout(ctx, 12*time.Second)
		fetchResult, err := webtools.Fetch(fetchCtx, map[string]any{
			"url":        url,
			"timeout_ms": 9000,
			"max_bytes":  220 * 1024,
		})
		fetchCancel()
		if err != nil {
			warnings = append(warnings, "Could not fetch web reference source "+url+": "+compactPluginLearningError(err))
			sources = append(sources, source)
			continue
		}
		source["final_url"] = firstNonEmptyText(fetchResult, "final_url")
		source["content_type"] = firstNonEmptyText(fetchResult, "content_type")
		source["status_code"] = intNumber(fetchResult["status_code"])
		body := firstNonEmpty(firstNonEmptyText(fetchResult, "body"), firstNonEmptyText(fetchResult, "body_excerpt"), firstNonEmptyText(fetchResult, "text_digest"))
		source["text_excerpt"] = pluginWebReferenceTextExcerpt(body, 1800)
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		out["status"] = "not_found"
		out["warnings"] = compactStringList(append(append(hostedWarnings, warnings...), "No web reference source could be fetched."))
		return out
	}
	digestOut := s.buildPluginWebResearchDigest(ctx, cfg, target, digest, query, results, sources)
	for key, value := range digestOut {
		out[key] = value
	}
	out["schema"] = pluginLearningWebReferenceSchema
	out["decision"] = pluginWebReferenceDecision(args)
	out["query"] = query
	out["search_source"] = firstNonEmptyText(searchResult, "source")
	out["search_results"] = pluginLearningCompactRows(results, 6, 260, "title", "url", "snippet", "web_reference_score")
	out["warnings"] = compactStringList(append(append(hostedWarnings, stringListValue(out["warnings"])...), warnings...))
	return pluginWebReferencePublicDigest(out)
}

func (s *Server) buildPluginHostedWebReference(ctx context.Context, cfg config.EngineConfig, target pluginLearningTarget, digest pluginParameterDigest, query string) (map[string]any, string) {
	if s == nil || s.llm == nil || !cfg.Complete() {
		return nil, ""
	}
	hostedCfg, ok := pluginHostedWebSearchConfig(cfg)
	if !ok {
		return nil, ""
	}
	input := map[string]any{
		"schema":                    "plugin_hosted_web_reference_request.v1",
		"target":                    pluginLearningTargetMap(target),
		"query":                     query,
		"plugin_identity":           digest.PluginIdentity,
		"plugin_name":               firstNonEmpty(digest.PluginName, target.PluginName),
		"backend_parameter_summary": pluginWebReferenceBackendSummary(digest),
		"policy":                    "Use hosted web search for public plugin documentation. Web evidence may guide type and human-facing controls; it must not create final backend param_id mappings.",
	}
	inputJSON, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return nil, "OpenAI hosted web search input could not be prepared: " + compactPluginLearningError(err)
	}
	system := `You build compact public-documentation evidence for audio Plugin Skill auto-learning. Use web search when helpful. Return ONLY strict JSON.`
	user := fmt.Sprintf(`Find concise public evidence for this plugin and infer a lightweight type prior for learning.

Request:
%s

Return ONLY strict JSON with this schema:
{
  "schema": "plugin_web_reference_digest.v2",
  "status": "provided|not_found",
  "search_source": "openai_hosted_web_search",
  "sources": [{"title": "source title", "url": "https://...", "source_type": "official_manual|official_product_page|support_doc|third_party_reference|search_result|unknown", "confidence": 0.0}],
  "type_hints": [{"type": "eq|dynamic_eq|compressor|limiter|gate|expander|de_esser|reverb|delay|modulation|chorus|flanger|phaser|saturation|distortion|transient_shaper|pitch|utility|meter|instrument|hybrid|unknown", "summary": "brief evidence", "source_url": "https://...", "confidence": 0.0}],
  "documented_core_controls": [{"control": "human-facing control concept", "description": "what the docs say it controls", "display_clues": ["units, labels, ranges, sections, or states mentioned by docs"], "priority": "primary|secondary|utility|unknown", "source_url": "https://...", "confidence": 0.0}],
  "documented_units_and_ranges": [{"control": "human-facing control concept", "unit": "Hz|dB|%%|ms|s|ratio|note|toggle|unknown", "range_or_values": "documented range/options if present", "source_url": "https://...", "confidence": 0.0}],
  "ui_sections": [{"label": "documented section/module name", "description": "short summary", "source_url": "https://...", "confidence": 0.0}],
  "warnings": ["uncertainties, weak docs, source conflicts, or if web search found little"]
}

Rules:
- Prefer official product/manual/support pages when available.
- If search finds no relevant plugin documentation, return the same JSON schema with "status": "not_found", empty evidence arrays, and a warning. Never return an empty response.
- Keep this lightweight; do not copy long source text.
- Do not output backend param_id values.
- Treat type as a prior for later visual/Skill reasoning, not as a hard certainty.`, string(inputJSON))
	assembly := promptruntime.Build(promptruntime.AssemblyInput{
		SystemSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionStatic, "plugin_hosted_web_reference_system", "", system, true),
		},
		UserSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionRuntime, "plugin_hosted_web_reference_runtime", "", user, false),
		},
	})
	req := llm.Request{
		Messages:   assembly.Messages,
		Timeout:    pluginLearningWebDigestTimeout,
		Tools:      []map[string]any{{"type": "web_search"}},
		ToolChoice: "auto",
		Metadata: llm.RequestMetadata{
			Source:            "plugin_learning_web_digest_search",
			PromptFingerprint: assembly.Fingerprint,
			PromptStats:       assembly.Stats.Map(),
		},
	}
	resp, err := s.llm.CompleteRequest(ctx, hostedCfg, req)
	if err != nil && pluginHostedWebSearchShouldRetryJSON(err) {
		retryReq := req
		retryReq.PreferJSON = true
		retryReq.Metadata.Source = "plugin_learning_web_digest_search_json_retry"
		resp, err = s.llm.CompleteRequest(ctx, hostedCfg, retryReq)
	}
	if err != nil {
		return nil, "OpenAI hosted web search failed; falling back to local web search: " + compactPluginLearningError(err)
	}
	parsed, err := parsePluginUIReferenceJSON(resp.Text, pluginLearningWebReferenceSchema)
	if err != nil {
		return nil, "OpenAI hosted web search returned invalid JSON; falling back to local web search: " + compactPluginLearningError(err)
	}
	parsed["schema"] = pluginLearningWebReferenceSchema
	parsed["status"] = firstNonEmpty(firstNonEmptyText(parsed, "status"), "provided")
	parsed["search_source"] = firstNonEmpty(firstNonEmptyText(parsed, "search_source"), "openai_hosted_web_search")
	parsed["policy"] = "Hosted web search is optional evidence for type/core-control discovery; it never creates final param_id mappings."
	return pluginWebReferencePublicDigest(parsed), ""
}

func pluginHostedWebSearchShouldRetryJSON(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "responses stream returned no output") || strings.Contains(text, "responses stream returned no events")
}

func pluginHostedWebSearchConfig(cfg config.EngineConfig) (config.EngineConfig, bool) {
	cfg.Normalize()
	model := strings.ToLower(strings.TrimSpace(cfg.DefaultModel))
	if !strings.HasPrefix(model, "gpt") {
		return config.EngineConfig{}, false
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return config.EngineConfig{}, false
	}
	switch {
	case strings.HasSuffix(base, "/responses"):
	case strings.HasSuffix(base, "/chat/completions"):
		base = strings.TrimSuffix(base, "/chat/completions") + "/responses"
	case strings.HasSuffix(base, "/v1"):
		base += "/responses"
	default:
		base += "/responses"
	}
	cfg.BaseURL = base
	return cfg, cfg.Complete()
}

func (s *Server) buildPluginWebResearchDigest(ctx context.Context, cfg config.EngineConfig, target pluginLearningTarget, digest pluginParameterDigest, query string, searchResults, sources []map[string]any) map[string]any {
	if s == nil || s.llm == nil || !cfg.Complete() {
		return pluginWebReferenceFallbackDigest("provided_without_research_summary", query, searchResults, sources, []string{"LLM route is unavailable; using bounded source metadata without web research summary."})
	}
	input := pluginWebReferenceResearchInput(target, digest, query, searchResults, sources)
	inputJSON, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return pluginWebReferenceFallbackDigest("summarize_failed", query, searchResults, sources, []string{"Could not prepare web research digest input: " + compactPluginLearningError(err)})
	}
	system := `You build compact public-documentation research digests for audio plugin learning. Return ONLY strict JSON.`
	user := fmt.Sprintf(`Summarize the supplied public web evidence for Plugin Skill auto-learning.

Evidence:
%s

Return ONLY strict JSON with this schema:
{
  "schema": "plugin_web_reference_digest.v2",
  "status": "provided",
  "sources": [{"title": "source title", "url": "https://...", "source_type": "official_manual|official_product_page|support_doc|third_party_reference|search_result|unknown", "confidence": 0.0}],
  "type_hints": [{"type": "eq|dynamic_eq|compressor|limiter|gate|expander|de_esser|reverb|delay|modulation|chorus|flanger|phaser|saturation|distortion|transient_shaper|pitch|utility|meter|instrument|hybrid|unknown", "summary": "brief evidence", "source_url": "https://...", "confidence": 0.0}],
  "documented_core_controls": [{"control": "human-facing control concept", "description": "what the docs say it controls", "display_clues": ["units, labels, ranges, sections, or states mentioned by docs"], "priority": "primary|secondary|utility|unknown", "source_url": "https://...", "confidence": 0.0}],
  "documented_units_and_ranges": [{"control": "human-facing control concept", "unit": "Hz|dB|%%|ms|s|ratio|note|toggle|unknown", "range_or_values": "documented range/options if present", "source_url": "https://...", "confidence": 0.0}],
  "ui_sections": [{"label": "documented section/module name", "description": "short summary", "source_url": "https://...", "confidence": 0.0}],
  "warnings": ["uncertainties, source conflicts, missing manual, or if evidence is weak"]
}

Rules:
- Use only facts supported by the supplied public evidence. Do not invent product details.
- Do not output backend param_id values; public docs can guide type, UI terms, display units/ranges, and experiment priorities only.
- Prefer official manuals/product/support pages when present, but keep third-party evidence if clearly labeled.
- The core Plugin Skill learning objective is how human UI controls are displayed, read, and constrained: units, ranges, labels, sections, states, and options.
- Summarize facts; do not copy long source text verbatim.
- This must stay generic across plugin types; do not assume one product family or one experiment.`, string(inputJSON))
	assembly := promptruntime.Build(promptruntime.AssemblyInput{
		SystemSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionStatic, "plugin_web_reference_digest_system", "", system, true),
		},
		UserSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionRuntime, "plugin_web_reference_digest_runtime", "", user, false),
		},
	})
	resp, err := s.completePluginUIReferenceRequest(ctx, cfg, llm.Request{
		Messages: assembly.Messages,
		Timeout:  pluginLearningWebDigestTimeout,
		Metadata: llm.RequestMetadata{
			Source:            "plugin_web_reference_digest",
			PromptFingerprint: assembly.Fingerprint,
			PromptStats:       assembly.Stats.Map(),
		},
	})
	if err != nil {
		return pluginWebReferenceFallbackDigest("summarize_failed", query, searchResults, sources, []string{"Web research digest failed: " + compactPluginLearningError(err)})
	}
	parsed, err := parsePluginUIReferenceJSON(resp.Text, pluginLearningWebReferenceSchema)
	if err != nil {
		return pluginWebReferenceFallbackDigest("summarize_failed", query, searchResults, sources, []string{"Web research digest returned invalid JSON: " + compactPluginLearningError(err)})
	}
	parsed["schema"] = pluginLearningWebReferenceSchema
	parsed["status"] = firstNonEmpty(firstNonEmptyText(parsed, "status"), "provided")
	parsed["policy"] = "Public documentation is optional evidence for type/core-control discovery; it never creates final param_id mappings."
	if len(mapRowsValue(parsed["sources"])) == 0 {
		parsed["sources"] = pluginWebReferencePublicSources(sources, 3)
	}
	return pluginWebReferencePublicDigest(parsed)
}

func pluginWebReferenceResearchInput(target pluginLearningTarget, digest pluginParameterDigest, query string, searchResults, sources []map[string]any) map[string]any {
	return map[string]any{
		"schema":                     "plugin_web_reference_research_input.v1",
		"target":                     pluginLearningTargetMap(target),
		"query":                      query,
		"search_results":             pluginLearningCompactRows(searchResults, 6, 260, "title", "url", "snippet", "web_reference_score"),
		"sources":                    pluginWebReferenceResearchSources(sources, 3),
		"plugin_identity":            digest.PluginIdentity,
		"plugin_name":                firstNonEmpty(digest.PluginName, target.PluginName),
		"backend_parameter_summary":  pluginWebReferenceBackendSummary(digest),
		"public_doc_evidence_policy": "Docs may identify plugin type, human-facing controls, units, ranges, and UI sections. They must not choose backend param_id mappings.",
	}
}

func pluginWebReferenceResearchSources(sources []map[string]any, limit int) []map[string]any {
	if limit >= 0 && len(sources) > limit {
		sources = sources[:limit]
	}
	out := make([]map[string]any, 0, len(sources))
	for _, source := range sources {
		row := pluginLearningCompactRow(source, 260, "title", "url", "final_url", "content_type", "status_code", "snippet", "source_rank")
		if excerpt := firstNonEmptyText(source, "text_excerpt", "body_excerpt", "text_digest"); excerpt != "" {
			row["source_text_excerpt"] = pluginLearningPromptClip(excerpt, 1800)
		}
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	return out
}

func pluginWebReferenceBackendSummary(digest pluginParameterDigest) map[string]any {
	focus := pluginUIReferenceVisualFocusSummary(digest)
	return map[string]any{
		"parameter_count":       digest.ParameterCount,
		"template_role_hint":    digest.TemplateRole,
		"plugin_class_hint":     digest.PluginClass,
		"role_summary":          focus["role_summary"],
		"unit_summary":          focus["unit_summary"],
		"group_summary":         firstRowsLimit(mapRowsValue(focus["group_summary"]), 12),
		"representative_params": pluginTypeHypothesisRepresentativeParams(digest, 45),
		"param_id_policy":       "Representative params are backend evidence only; do not output final param_id mappings in the web digest.",
	}
}

func pluginWebReferenceFallbackDigest(status, query string, searchResults, sources []map[string]any, warnings []string) map[string]any {
	return pluginWebReferencePublicDigest(map[string]any{
		"schema":         pluginLearningWebReferenceSchema,
		"status":         firstNonEmpty(status, "provided_without_research_summary"),
		"query":          query,
		"search_results": pluginLearningCompactRows(searchResults, 6, 260, "title", "url", "snippet", "web_reference_score"),
		"sources":        pluginWebReferencePublicSources(sources, 3),
		"warnings":       compactStringList(warnings),
		"policy":         "Public documentation is optional evidence for type/core-control discovery; it never creates final param_id mappings.",
	})
}

func pluginWebReferencePublicSources(sources []map[string]any, limit int) []map[string]any {
	if limit >= 0 && len(sources) > limit {
		sources = sources[:limit]
	}
	out := make([]map[string]any, 0, len(sources))
	for _, source := range sources {
		row := pluginLearningCompactRow(source, 260, "title", "url", "final_url", "content_type", "status_code", "snippet", "source_rank")
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	return out
}

func pluginWebReferencePublicDigest(in map[string]any) map[string]any {
	out := copyStringAnyMap(in)
	delete(out, "body")
	delete(out, "body_excerpt")
	delete(out, "text_excerpt")
	delete(out, "source_text_excerpt")
	if sources := mapRowsValue(out["sources"]); len(sources) > 0 {
		clean := make([]map[string]any, 0, len(sources))
		for _, source := range sources {
			row := copyStringAnyMap(source)
			delete(row, "body")
			delete(row, "body_excerpt")
			delete(row, "text_excerpt")
			delete(row, "source_text_excerpt")
			clean = append(clean, row)
		}
		out["sources"] = clean
	}
	return out
}

func pluginWebReferenceDecision(args map[string]any) string {
	decision := strings.TrimSpace(strings.ToLower(firstNonEmptyText(args, "web_reference_decision")))
	if decision == "" && boolValue(firstPresentAny(args, "web_reference_enabled")) {
		decision = "enabled"
	}
	if decision == "" {
		return "skipped"
	}
	switch decision {
	case "enabled", "enable", "provided", "use", "true", "yes":
		return "enabled"
	default:
		return "skipped"
	}
}

func pluginWebReferenceRequested(args map[string]any) bool {
	return pluginWebReferenceDecision(args) == "enabled"
}

func pluginWebReferenceSearchQuery(target pluginLearningTarget, digest pluginParameterDigest) string {
	name := firstNonEmpty(target.PluginName, digest.PluginName, firstNonEmptyText(digest.PluginIdentity, "plugin_name", "name"))
	name = pluginWebReferenceSafeQueryText(name)
	if name == "" {
		return ""
	}
	maker := pluginWebReferenceSafeQueryText(firstNonEmptyText(digest.PluginIdentity, "manufacturer", "maker", "vendor"))
	parts := []string{}
	if maker != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(maker)) {
		parts = append(parts, `"`+maker+`"`)
	}
	parts = append(parts, `"`+name+`"`, "audio plugin manual controls")
	return strings.Join(parts, " ")
}

func pluginWebReferenceSafeQueryText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, `\`, " ")
	text = strings.ReplaceAll(text, `/`, " ")
	text = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '-' || r == '_' || r == '.' || r == '+':
			return r
		default:
			return ' '
		}
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

func pluginWebReferenceRankedResults(rows []map[string]any, target pluginLearningTarget, digest pluginParameterDigest) []map[string]any {
	nameTerms := pluginUIReferenceTerms(target.PluginName, digest.PluginName, firstNonEmptyText(digest.PluginIdentity, "manufacturer", "maker", "vendor"))
	relevanceTerms := pluginWebReferenceRelevanceTerms(target, digest)
	type scored struct {
		row   map[string]any
		score int
	}
	scoredRows := []scored{}
	seen := map[string]bool{}
	for _, row := range rows {
		url := firstNonEmptyText(row, "url")
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		title := firstNonEmptyText(row, "title")
		snippet := firstNonEmptyText(row, "snippet")
		haystack := strings.ToLower(title + " " + snippet + " " + url)
		relevanceHits := 0
		for _, term := range relevanceTerms {
			if strings.Contains(haystack, strings.ToLower(term)) {
				relevanceHits++
			}
		}
		if len(relevanceTerms) > 0 && relevanceHits == 0 {
			continue
		}
		score := 0
		for _, term := range nameTerms {
			if len(term) >= 3 && strings.Contains(haystack, strings.ToLower(term)) {
				score += 10
			}
		}
		hasDocTerm := false
		for _, term := range []string{"manual", "documentation", "docs", "user guide", "product", "support", "pdf"} {
			if strings.Contains(haystack, term) {
				score += 8
				hasDocTerm = true
			}
		}
		hasAudioTerm := false
		for _, term := range []string{"plugin", "vst", "vst3", "audio", "daw", "eq", "compressor", "reverb", "delay"} {
			if strings.Contains(haystack, term) {
				score += 3
				hasAudioTerm = true
			}
		}
		for _, term := range []string{"forum", "reddit", "youtube", "review", "coupon", "download"} {
			if strings.Contains(haystack, term) {
				score -= 6
			}
		}
		if len(relevanceTerms) > 1 && relevanceHits < 2 && !hasDocTerm && !hasAudioTerm {
			continue
		}
		if score < 10 {
			continue
		}
		next := copyStringAnyMap(row)
		next["web_reference_score"] = score
		scoredRows = append(scoredRows, scored{row: next, score: score})
	}
	sort.SliceStable(scoredRows, func(i, j int) bool {
		if scoredRows[i].score == scoredRows[j].score {
			return strings.ToLower(firstNonEmptyText(scoredRows[i].row, "title")) < strings.ToLower(firstNonEmptyText(scoredRows[j].row, "title"))
		}
		return scoredRows[i].score > scoredRows[j].score
	})
	if len(scoredRows) > 6 {
		scoredRows = scoredRows[:6]
	}
	out := make([]map[string]any, 0, len(scoredRows))
	for _, row := range scoredRows {
		out = append(out, row.row)
	}
	return out
}

func pluginWebReferenceRelevanceTerms(target pluginLearningTarget, digest pluginParameterDigest) []string {
	name := firstNonEmpty(target.PluginName, digest.PluginName, firstNonEmptyText(digest.PluginIdentity, "plugin_name", "name"))
	terms := pluginUIReferenceTerms(pluginWebReferenceSafeQueryText(name))
	stop := map[string]bool{
		"plugin": true, "audio": true, "effect": true, "effects": true, "vst": true, "vst3": true, "au": true, "aax": true,
		"the": true, "and": true, "for": true, "with": true, "free": true, "trial": true, "demo": true,
	}
	out := []string{}
	for _, term := range terms {
		if len(term) < 3 || stop[term] {
			continue
		}
		out = append(out, term)
	}
	return compactStringList(out)
}

func pluginWebReferenceTextExcerpt(body string, max int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	body = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>|<noscript[^>]*>.*?</noscript>`).ReplaceAllString(body, " ")
	body = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(body, " ")
	body = strings.ReplaceAll(body, "&nbsp;", " ")
	body = strings.ReplaceAll(body, "&amp;", "&")
	body = strings.ReplaceAll(body, "&lt;", "<")
	body = strings.ReplaceAll(body, "&gt;", ">")
	body = strings.Join(strings.Fields(body), " ")
	if max > 0 && len(body) > max {
		body = body[:max]
	}
	return body
}

func (s *Server) buildPluginTypeHypothesis(ctx context.Context, cfg config.EngineConfig, target pluginLearningTarget, digest pluginParameterDigest, webReference map[string]any) map[string]any {
	out := map[string]any{
		"schema": "plugin_type_hypothesis.v1",
		"status": "unavailable",
		"policy": "LLM inference from backend parameters and optional public docs; never a hard-coded local type decision.",
	}
	if s == nil || s.llm == nil {
		out["warnings"] = []string{"LLM route is unavailable; plugin type hypothesis was skipped."}
		return out
	}
	input := pluginTypeHypothesisInput(target, digest, webReference)
	inputJSON, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		out["status"] = "failed"
		out["warnings"] = []string{"Could not prepare plugin type hypothesis input: " + compactPluginLearningError(err)}
		return out
	}
	system := `You infer the functional type of an audio plugin from evidence. Return ONLY strict JSON.`
	user := fmt.Sprintf(`Infer the likely plugin type and the core controls a human mixer would need to operate this plugin effectively.

Evidence:
%s

Return ONLY strict JSON with this schema:
{
  "schema": "plugin_type_hypothesis.v1",
  "primary_type": "eq|dynamic_eq|compressor|limiter|gate|expander|de_esser|reverb|delay|modulation|chorus|flanger|phaser|saturation|distortion|transient_shaper|pitch|utility|meter|instrument|hybrid|unknown",
  "secondary_types": ["optional additional likely types"],
  "confidence": 0.0,
  "evidence": [{"kind": "backend_parameter|display_probe|plugin_identity|web_reference|semantic_hint", "summary": "short reason"}],
  "missing_evidence": ["what remains uncertain"],
  "core_control_expectations": [{"control": "human-facing control concept, not a backend ID", "why_it_matters": "short reason for this plugin type", "expected_display_clues": ["units, ranges, readouts, sections, states to look for"], "priority": "primary|secondary|utility", "confidence": 0.0}],
  "warnings": ["uncertainties"]
}

Rules:
- Do not hard-code from one product or one experiment. Infer from the supplied evidence.
- The core learning objective is not merely what a parameter is called; it is how the human UI displays, reads, and constrains it.
- Use public docs as evidence only when present. Do not invent facts not supported by the evidence.
- Allow hybrid or unknown when evidence is mixed or weak.
- Core control expectations should guide visual matching and later active experiments; they are not final parameter mappings and must not include backend param_id values.`, string(inputJSON))
	assembly := promptruntime.Build(promptruntime.AssemblyInput{
		SystemSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionStatic, "plugin_type_hypothesis_system", "", system, true),
		},
		UserSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionRuntime, "plugin_type_hypothesis_runtime", "", user, false),
		},
	})
	resp, err := s.completePluginUIReferenceRequest(ctx, cfg, llm.Request{
		Messages: assembly.Messages,
		Timeout:  pluginLearningTypeTimeout,
		Metadata: llm.RequestMetadata{
			Source:            "plugin_type_hypothesis",
			PromptFingerprint: assembly.Fingerprint,
			PromptStats:       assembly.Stats.Map(),
		},
	})
	if err != nil {
		out["status"] = "failed"
		out["warnings"] = []string{"Plugin type hypothesis failed: " + compactPluginLearningError(err)}
		return out
	}
	parsed, err := parsePluginUIReferenceJSON(resp.Text, "plugin_type_hypothesis.v1")
	if err != nil {
		out["status"] = "failed"
		out["warnings"] = []string{"Plugin type hypothesis returned invalid JSON: " + compactPluginLearningError(err)}
		return out
	}
	parsed["status"] = "provided"
	if firstNonEmptyText(parsed, "policy") == "" {
		parsed["policy"] = out["policy"]
	}
	return parsed
}

func pluginTypeHypothesisInput(target pluginLearningTarget, digest pluginParameterDigest, webReference map[string]any) map[string]any {
	focus := pluginUIReferenceVisualFocusSummary(digest)
	out := map[string]any{
		"schema":                "plugin_type_hypothesis_input.v1",
		"target":                pluginLearningTargetMap(target),
		"plugin_name":           firstNonEmpty(digest.PluginName, target.PluginName),
		"plugin_identity":       digest.PluginIdentity,
		"template_role_hint":    digest.TemplateRole,
		"plugin_class_hint":     digest.PluginClass,
		"parameter_count":       digest.ParameterCount,
		"quick_controls":        digest.QuickControls,
		"recommended_groups":    digest.RecommendedGroups,
		"display_probe_summary": plugingrabber.DisplayProbeSummary(digest),
		"role_summary":          focus["role_summary"],
		"unit_summary":          focus["unit_summary"],
		"group_summary":         firstRowsLimit(mapRowsValue(focus["group_summary"]), 18),
		"visual_search_terms":   firstRowsLimit(mapRowsValue(focus["visual_search_terms"]), 32),
		"representative_params": pluginTypeHypothesisRepresentativeParams(digest, 90),
		"param_id_policy":       "Representative params may be cited as backend evidence only; core_control_expectations must not choose final param_id mappings.",
	}
	if len(webReference) > 0 && firstNonEmptyText(webReference, "status") != "skipped" {
		out["plugin_web_reference"] = webReference
	}
	if pack := plugingrabber.BuildContextPack(digest); len(pack) > 0 {
		if runtimeProfile := mapValue(pack["runtime_profile"]); len(runtimeProfile) > 0 {
			out["runtime_profile_evidence"] = runtimeProfile
		}
		if components := mapRowsValue(pack["components"]); len(components) > 0 {
			out["semantic_component_hints"] = firstRowsLimit(components, 16)
		}
	}
	return out
}

func pluginTypeHypothesisRepresentativeParams(digest pluginParameterDigest, limit int) []map[string]any {
	if limit <= 0 {
		return nil
	}
	rows := make([]map[string]any, 0, minInt(len(digest.Parameters), limit))
	for _, param := range digest.Parameters {
		id := strings.TrimSpace(param.ID)
		if id == "" {
			continue
		}
		row := map[string]any{"param_id": id}
		if param.Name != "" {
			row["name"] = param.Name
		}
		if param.RawName != "" && param.RawName != param.Name {
			row["raw_name"] = param.RawName
		}
		if param.DisplayGroup != "" {
			row["display_group"] = param.DisplayGroup
		}
		if param.NormalizedRole != "" {
			row["normalized_role"] = param.NormalizedRole
		}
		if param.ControlRelevance != "" {
			row["control_relevance"] = param.ControlRelevance
		}
		if param.Unit != "" {
			row["unit_hint"] = param.Unit
		}
		if param.ValueText != "" {
			row["current_value_text"] = param.ValueText
		}
		if param.IsBoolean {
			row["is_boolean"] = true
		}
		if param.IsDiscrete {
			row["is_discrete"] = true
		}
		if param.DisplayProbe != nil {
			row["display_probe"] = pluginUIReferenceCompactDisplayProbe(param.DisplayProbe)
		}
		rows = append(rows, row)
		if len(rows) >= limit {
			break
		}
	}
	return rows
}

func firstRowsLimit(rows []map[string]any, limit int) []map[string]any {
	if limit >= 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func compactPluginLearningError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}

func pluginLearningShouldBuildTypeHypothesis(args map[string]any, webReference map[string]any) bool {
	if pluginUIReferenceDecisionProvided(args) {
		return true
	}
	return pluginWebReferenceHasUsableEvidence(webReference)
}

func pluginUIReferenceDecisionProvided(args map[string]any) bool {
	decision := strings.TrimSpace(strings.ToLower(firstNonEmptyText(args, "ui_reference_decision")))
	if decision == "" && len(stringListValue(args["ui_reference_artifact_ids"])) > 0 {
		decision = "provided"
	}
	switch decision {
	case "provided", "continue", "continue_with_ui_reference", "use":
		return true
	default:
		return false
	}
}

func pluginLearningEvidenceShouldForceLLMPatch(uiReference, webReference, pluginTypeHypothesis map[string]any) bool {
	if pluginUIReferenceVisualDigestHasUsableEvidence(mapValue(uiReference["visual_digest"])) {
		return true
	}
	if pluginWebReferenceHasUsableEvidence(webReference) {
		return true
	}
	if strings.EqualFold(firstNonEmptyText(pluginTypeHypothesis, "status"), "provided") {
		return true
	}
	return false
}

func pluginLearningLLMStrategy(uiReference, webReference, pluginTypeHypothesis map[string]any) string {
	parts := []string{"llm_profile_patch"}
	if strings.EqualFold(firstNonEmptyText(pluginTypeHypothesis, "status"), "provided") {
		parts = append(parts, "type_hypothesis")
	}
	if pluginWebReferenceHasUsableEvidence(webReference) {
		parts = append(parts, "web_reference")
	}
	if pluginUIReferenceVisualDigestHasUsableEvidence(mapValue(uiReference["visual_digest"])) {
		parts = append(parts, "ui_reference_visual_digest")
	}
	return strings.Join(parts, "+")
}

func pluginWebReferenceHasUsableEvidence(webReference map[string]any) bool {
	status := strings.TrimSpace(strings.ToLower(firstNonEmptyText(webReference, "status")))
	if status == "" || status == "skipped" || status == "searching" || status == "failed" || status == "not_found" || status == "unavailable" || status == "unusable" {
		return false
	}
	if len(mapRowsValue(webReference["sources"])) > 0 || len(mapRowsValue(webReference["type_hints"])) > 0 || len(mapRowsValue(webReference["documented_core_controls"])) > 0 {
		return true
	}
	return status == "provided"
}

func buildPluginCoreControlCoverage(pluginTypeHypothesis, uiReference map[string]any, patch pluginProfilePatch) map[string]any {
	expectations := mapRowsValue(pluginTypeHypothesis["core_control_expectations"])
	if len(expectations) == 0 {
		return nil
	}
	visualDigest := mapValue(uiReference["visual_digest"])
	visualRows := mapRowsValue(visualDigest["candidate_parameter_matches"])
	patchRows := pluginPatchMappingEvidenceRows(patch)
	covered := []map[string]any{}
	uncertain := []map[string]any{}
	missing := []map[string]any{}
	probes := []map[string]any{}
	for _, expectation := range expectations {
		control := firstNonEmptyText(expectation, "control", "name", "label")
		if control == "" {
			continue
		}
		row := copyStringAnyMap(expectation)
		hit, hitReason, paramIDs := pluginCoreControlCoverageHit(expectation, visualRows, patchRows)
		if hit {
			row["status"] = "covered"
			row["matched_param_ids"] = paramIDs
			row["reason"] = hitReason
			covered = append(covered, row)
			continue
		}
		row["status"] = "uncertain"
		row["reason"] = "No confident visual/backend mapping was found for this expected core control."
		if len(visualRows) == 0 {
			row["status"] = "missing"
			row["reason"] = "No candidate visual/backend match currently supports this expected core control."
			missing = append(missing, row)
		} else {
			uncertain = append(uncertain, row)
		}
		if len(probes) < 5 {
			probes = append(probes, map[string]any{
				"control":  control,
				"question": "Confirm the visible UI label, unit/range, and audible/display response for this core control.",
				"reason":   row["reason"],
			})
		}
	}
	return map[string]any{
		"schema":                      "plugin_core_control_coverage.v1",
		"plugin_type":                 firstNonEmptyText(pluginTypeHypothesis, "primary_type"),
		"covered":                     covered,
		"uncertain":                   uncertain,
		"missing":                     missing,
		"recommended_probe_questions": probes,
		"policy":                      "Coverage guides active experiments; it does not create final param_id mappings.",
	}
}

func pluginPatchMappingEvidenceRows(patch pluginProfilePatch) []map[string]any {
	rows := []map[string]any{}
	addParams := func(container map[string]any, params map[string]any) {
		groupLabel := firstNonEmptyText(container, "label", "name", "id", "role", "component_id", "operation")
		for slot, raw := range params {
			mapping := mapValue(raw)
			paramID := firstNonEmptyText(mapping, "param_id", "id")
			if paramID == "" {
				paramID = strings.TrimSpace(fmt.Sprint(raw))
			}
			if paramID == "" || paramID == "<nil>" {
				continue
			}
			rows = append(rows, map[string]any{
				"param_id":     paramID,
				"slot":         strings.TrimSpace(fmt.Sprint(slot)),
				"label":        firstNonEmptyText(mapping, "label", "visual_label"),
				"group":        groupLabel,
				"display_text": firstNonEmptyText(mapping, "display_domain_text", "visual_display_domain_text"),
			})
		}
	}
	for _, group := range patch.Groups {
		if params := mapValue(group["params"]); len(params) > 0 {
			addParams(group, params)
		}
	}
	for _, control := range patch.VirtualControls {
		if params := mapValue(control["params"]); len(params) > 0 {
			addParams(control, params)
		}
	}
	return rows
}

func pluginCoreControlCoverageHit(expectation map[string]any, visualRows, patchRows []map[string]any) (bool, string, []string) {
	terms := pluginCoreControlExpectationTerms(expectation)
	if len(terms) == 0 {
		return false, "", nil
	}
	paramIDs := []string{}
	for _, row := range visualRows {
		rowTerms := pluginUIReferenceTerms(
			firstNonEmptyText(row, "visible_label", "label"),
			firstNonEmptyText(row, "visible_group", "group"),
			firstNonEmptyText(row, "backend_param_name", "backend_name"),
			firstNonEmptyText(row, "semantic_class", "control_kind", "display_domain", "visible_unit"),
		)
		if pluginCoreControlTermOverlap(terms, rowTerms) {
			if id := firstNonEmptyText(row, "backend_param_id", "param_id"); id != "" {
				paramIDs = append(paramIDs, id)
			}
		}
	}
	if len(paramIDs) > 0 {
		return true, "Matched by param-bound visual/backend evidence.", compactStringList(paramIDs)
	}
	for _, row := range patchRows {
		rowTerms := pluginUIReferenceTerms(
			firstNonEmptyText(row, "slot", "label", "group", "display_text"),
		)
		if pluginCoreControlTermOverlap(terms, rowTerms) {
			if id := firstNonEmptyText(row, "param_id"); id != "" {
				paramIDs = append(paramIDs, id)
			}
		}
	}
	if len(paramIDs) > 0 {
		return true, "Mapped in the current Plugin Skill draft; display-domain evidence may still need probing.", compactStringList(paramIDs)
	}
	return false, "", nil
}

func pluginCoreControlExpectationTerms(expectation map[string]any) []string {
	values := []string{
		firstNonEmptyText(expectation, "control", "name", "label"),
		firstNonEmptyText(expectation, "why_it_matters", "summary"),
		firstNonEmptyText(expectation, "priority"),
	}
	values = append(values, stringListValue(expectation["expected_display_clues"])...)
	return pluginUIReferenceTerms(values...)
}

func pluginCoreControlTermOverlap(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := map[string]bool{}
	for _, term := range b {
		if len(term) >= 3 {
			set[term] = true
		}
	}
	matches := 0
	for _, term := range a {
		if len(term) >= 3 && set[term] {
			matches++
		}
	}
	return matches >= 1
}

func attachPluginUIReferenceFocusAndCoverage(digest map[string]any, focusSummary map[string]any, parameterDigest pluginParameterDigest) {
	if len(digest) == 0 {
		return
	}
	if len(focusSummary) > 0 {
		digest["visual_focus_summary"] = focusSummary
	}
	digest["parameter_coverage_audit"] = pluginUIReferenceCoverageAudit(parameterDigest, focusSummary, digest)
}

func pluginUIReferenceCoverageAudit(parameterDigest pluginParameterDigest, focusSummary map[string]any, visualDigest map[string]any) map[string]any {
	total := parameterDigest.ParameterCount
	if total <= 0 {
		total = len(parameterDigest.Parameters)
	}
	matched := map[string]bool{}
	for _, row := range mapRowsValue(visualDigest["candidate_parameter_matches"]) {
		id := firstNonEmptyText(row, "backend_param_id", "param_id")
		if id != "" {
			matched[id] = true
		}
	}
	unmatched := total - len(matched)
	if unmatched < 0 {
		unmatched = 0
	}
	return map[string]any{
		"schema":                          "plugin_ui_reference_parameter_coverage.v1",
		"backend_param_total":             total,
		"backend_parameter_rows":          len(parameterDigest.Parameters),
		"parameters_retained":             true,
		"dropped_param_count":             0,
		"focus_source_parameter_count":    intNumber(focusSummary["source_parameter_count"]),
		"focus_group_count":               len(mapRowsValue(focusSummary["group_summary"])),
		"focus_role_count":                len(mapRowsValue(focusSummary["role_summary"])),
		"focus_unit_count":                len(mapRowsValue(focusSummary["unit_summary"])),
		"focus_search_term_count":         len(mapRowsValue(focusSummary["visual_search_terms"])),
		"vision_observed_control_count":   len(mapRowsValue(visualDigest["visible_controls"])),
		"vision_observed_group_count":     len(mapRowsValue(visualDigest["groups"])),
		"vision_possible_focus_hit_count": len(mapRowsValue(visualDigest["possible_focus_hits"])),
		"matched_param_count":             len(matched),
		"unmatched_param_count":           unmatched,
		"matching_status":                 firstNonEmptyText(visualDigest, "matching_status"),
	}
}

type pluginUIReferenceFocusBucket struct {
	Name  string
	Count int
	IDs   []string
}

func pluginUIReferenceVisualFocusSummary(digest pluginParameterDigest) map[string]any {
	total := digest.ParameterCount
	if total <= 0 {
		total = len(digest.Parameters)
	}
	quickIDs := map[string]bool{}
	quickRows := make([]map[string]any, 0, len(digest.QuickControls))
	for _, quick := range digest.QuickControls {
		id := strings.TrimSpace(quick.ParamID)
		if id == "" {
			continue
		}
		quickIDs[id] = true
		row := map[string]any{"param_id": id}
		if quick.Label != "" {
			row["label"] = quick.Label
		}
		if quick.DisplayGroup != "" {
			row["display_group"] = quick.DisplayGroup
		}
		if quick.NormalizedRole != "" {
			row["normalized_role"] = quick.NormalizedRole
		}
		quickRows = append(quickRows, row)
		if len(quickRows) >= 24 {
			break
		}
	}

	unitBuckets := map[string]*pluginUIReferenceFocusBucket{}
	roleBuckets := map[string]*pluginUIReferenceFocusBucket{}
	groupBuckets := map[string]*pluginUIReferenceFocusBucket{}
	termBuckets := map[string]*pluginUIReferenceFocusBucket{}
	for _, param := range digest.Parameters {
		id := strings.TrimSpace(param.ID)
		if id == "" {
			continue
		}
		for _, unit := range pluginUIReferenceFocusUnits(param) {
			pluginUIReferenceAddFocusBucket(unitBuckets, unit, id)
			pluginUIReferenceAddFocusBucket(termBuckets, unit, id)
		}
		role := strings.TrimSpace(param.NormalizedRole)
		if role == "" {
			role = "other"
		}
		pluginUIReferenceAddFocusBucket(roleBuckets, role, id)
		group := strings.TrimSpace(param.DisplayGroup)
		if group == "" {
			group = "Other"
		}
		pluginUIReferenceAddFocusBucket(groupBuckets, group, id)
		for _, term := range pluginUIReferenceFocusTerms(param, quickIDs[id]) {
			pluginUIReferenceAddFocusBucket(termBuckets, term, id)
		}
	}

	out := map[string]any{
		"schema":                   "plugin_ui_reference_visual_focus_summary.v1",
		"parameter_count":          total,
		"source_parameter_count":   len(digest.Parameters),
		"parameters_retained":      true,
		"dropped_param_count":      0,
		"quick_control_count":      len(digest.QuickControls),
		"quick_controls":           quickRows,
		"unit_summary":             pluginUIReferenceFocusRows(unitBuckets, 18, 8),
		"role_summary":             pluginUIReferenceFocusRows(roleBuckets, 18, 8),
		"group_summary":            pluginUIReferenceFocusRows(groupBuckets, 28, 10),
		"visual_search_terms":      pluginUIReferenceFocusRows(termBuckets, 48, 8),
		"display_probe_summary":    plugingrabber.DisplayProbeSummary(digest),
		"focus_policy":             "Statistics and representative IDs guide visual attention only; they never filter the full parameter ledger.",
		"param_id_source_policy":   "Representative IDs are hints for possible_focus_hits only. Final matching may only use the full backend candidate list.",
		"open_discovery_required":  true,
		"final_param_id_authority": "get_plugin_parameters plus Plugin Skill validator",
	}
	if digest.PluginClass != "" {
		out["plugin_class_hint"] = digest.PluginClass
	}
	return out
}

func pluginUIReferenceVisualTargetFocus(digest pluginParameterDigest, focusSummary, learningEvidence map[string]any) map[string]any {
	typeHypothesis := mapValue(learningEvidence["plugin_type_hypothesis"])
	webReference := mapValue(learningEvidence["plugin_web_reference"])
	out := map[string]any{
		"schema":                   "plugin_ui_reference_visual_target_focus.v1",
		"parameter_count":          digest.ParameterCount,
		"focus_policy":             "Probe only likely core controls and their visual display domains; do not inventory the whole UI.",
		"final_param_id_authority": "get_plugin_parameters plus Plugin Skill validator",
	}
	if digest.PluginClass != "" {
		out["plugin_class_hint"] = digest.PluginClass
	}
	if primaryType := firstNonEmptyText(typeHypothesis, "primary_type"); primaryType != "" {
		out["plugin_type_prior"] = map[string]any{
			"primary_type": primaryType,
			"confidence":   typeHypothesis["confidence"],
		}
	}
	if displayProbeSummary := firstPresentAny(focusSummary, "display_probe_summary"); displayProbeSummary != nil {
		out["display_probe_summary"] = displayProbeSummary
	}

	targets := []map[string]any{}
	addTarget := func(source string, row map[string]any) {
		if len(targets) >= 8 || len(row) == 0 {
			return
		}
		control := firstNonEmptyText(row, "control", "label", "name", "type")
		if control == "" {
			return
		}
		target := map[string]any{
			"target_id": fmt.Sprintf("target_%02d", len(targets)+1),
			"control":   pluginLearningPromptClip(control, 120),
			"source":    source,
			"priority":  firstNonEmpty(firstNonEmptyText(row, "priority"), "primary"),
		}
		if why := firstNonEmptyText(row, "why_it_matters", "description", "summary", "reason"); why != "" {
			target["why_it_matters"] = pluginLearningPromptClip(why, 220)
		}
		if clues := stringListValue(row["expected_display_clues"]); len(clues) > 0 {
			target["expected_display_clues"] = pluginLearningCompactStringList(clues, 6, 120)
		} else if clues := stringListValue(row["display_clues"]); len(clues) > 0 {
			target["expected_display_clues"] = pluginLearningCompactStringList(clues, 6, 120)
		}
		candidates := pluginUIReferenceTargetCandidateRows(digest, target, 4)
		if len(candidates) > 0 {
			target["candidate_parameters"] = candidates
			ids := make([]string, 0, len(candidates))
			for _, candidate := range candidates {
				if id := firstNonEmptyText(candidate, "param_id"); id != "" {
					ids = append(ids, id)
				}
			}
			target["candidate_param_ids"] = ids
		}
		targets = append(targets, target)
	}

	for _, row := range mapRowsValue(typeHypothesis["core_control_expectations"]) {
		addTarget("type_hypothesis", row)
	}
	for _, row := range mapRowsValue(webReference["documented_core_controls"]) {
		if len(targets) >= 8 {
			break
		}
		addTarget("web_reference", row)
	}
	if len(targets) == 0 {
		for _, quick := range digest.QuickControls {
			if len(targets) >= 8 {
				break
			}
			row := map[string]any{
				"control":  firstNonEmpty(quick.Label, quick.NormalizedRole, quick.ParamID),
				"priority": "primary",
				"reason":   "Quick control from backend parameter inventory.",
			}
			addTarget("quick_controls", row)
		}
	}
	if len(targets) == 0 {
		for _, param := range digest.Parameters {
			if len(targets) >= 8 {
				break
			}
			if strings.TrimSpace(param.ID) == "" {
				continue
			}
			row := map[string]any{
				"control":  firstNonEmpty(param.Name, param.RawName, param.ID),
				"priority": "primary",
				"reason":   "Fallback controllable parameter from backend inventory.",
			}
			addTarget("backend_parameters", row)
		}
	}
	out["targets"] = targets
	out["target_count"] = len(targets)
	if quickRows := mapRowsValue(focusSummary["quick_controls"]); len(quickRows) > 0 {
		out["quick_controls"] = pluginLearningCompactRows(quickRows, 10, 100, "param_id", "label", "display_group", "normalized_role")
	}
	return out
}

func pluginUIReferenceTargetCandidateRows(digest pluginParameterDigest, target map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(digest.Parameters) == 0 {
		return nil
	}
	quickIDs := map[string]bool{}
	for _, quick := range digest.QuickControls {
		if id := strings.TrimSpace(quick.ParamID); id != "" {
			quickIDs[id] = true
		}
	}
	targetTerms := pluginUIReferenceVisualTargetTerms(target)
	type scoredParam struct {
		param pluginParameterInfo
		score int
		index int
	}
	scored := make([]scoredParam, 0, len(digest.Parameters))
	for i, param := range digest.Parameters {
		id := strings.TrimSpace(param.ID)
		if id == "" {
			continue
		}
		score := pluginUIReferenceTermOverlapScore(targetTerms, pluginUIReferenceParameterTerms(param)) * 10
		if quickIDs[id] {
			score += 8
		}
		if param.HostControllable {
			score += 3
		}
		if param.ControlRelevance != "" {
			score += 3
		}
		if param.DisplayProbe != nil {
			score += 2
		}
		if param.DisplayDomainCandidate != nil {
			score += 2
		}
		if param.IsBoolean && pluginUIReferenceTargetSuggestsToggle(targetTerms) {
			score += 10
		}
		if pluginUIReferenceUnitTermsOverlap(targetTerms, pluginUIReferenceParameterTerms(param)) {
			score += 8
		}
		if score <= 0 && len(targetTerms) > 0 {
			continue
		}
		scored = append(scored, scoredParam{param: param, score: score, index: i})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].index < scored[j].index
		}
		return scored[i].score > scored[j].score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	rows := make([]map[string]any, 0, len(scored))
	for _, item := range scored {
		rows = append(rows, pluginUIReferenceTargetCandidateRow(item.param, quickIDs[item.param.ID]))
	}
	return rows
}

func pluginUIReferenceVisualTargetTerms(target map[string]any) []string {
	values := []string{
		firstNonEmptyText(target, "control", "label", "name", "type"),
		firstNonEmptyText(target, "why_it_matters", "description", "summary", "reason"),
		firstNonEmptyText(target, "priority"),
	}
	values = append(values, stringListValue(target["expected_display_clues"])...)
	values = append(values, stringListValue(target["display_clues"])...)
	return pluginUIReferenceTerms(values...)
}

func pluginUIReferenceParameterTerms(param pluginParameterInfo) []string {
	values := []string{param.ID, param.Name, param.RawName, param.Alias, param.DisplayGroup, param.NormalizedRole, param.ControlRelevance, param.Unit, param.ValueText}
	if param.DisplayProbe != nil {
		values = append(values, param.DisplayProbe.CurrentText, param.DisplayProbe.Label)
		for _, sample := range param.DisplayProbe.Samples {
			values = append(values, sample.Text)
			if len(values) > 20 {
				break
			}
		}
		for _, label := range param.DisplayProbe.DiscreteLabels {
			values = append(values, label.Label)
			if len(values) > 28 {
				break
			}
		}
		values = append(values, param.DisplayProbe.AllLabels...)
	}
	if param.DisplayDomainCandidate != nil {
		domain := mapFromJSONStruct(param.DisplayDomainCandidate)
		values = append(values, firstNonEmptyText(domain, "text", "unit", "scale", "source"))
	}
	return pluginUIReferenceTerms(values...)
}

func pluginUIReferenceTargetSuggestsToggle(terms []string) bool {
	for _, term := range terms {
		switch term {
		case "toggle", "switch", "enable", "enabled", "bypass", "active", "on", "off":
			return true
		}
	}
	return false
}

func pluginUIReferenceTargetCandidateRow(param pluginParameterInfo, quick bool) map[string]any {
	row := map[string]any{"param_id": param.ID}
	if param.Name != "" {
		row["name"] = pluginLearningPromptClip(param.Name, 120)
	}
	if param.RawName != "" && param.RawName != param.Name {
		row["raw_name"] = pluginLearningPromptClip(param.RawName, 120)
	}
	if param.Alias != "" {
		row["alias"] = pluginLearningPromptClip(param.Alias, 100)
	}
	if param.DisplayGroup != "" {
		row["display_group"] = pluginLearningPromptClip(param.DisplayGroup, 100)
	}
	if param.NormalizedRole != "" {
		row["normalized_role"] = pluginLearningPromptClip(param.NormalizedRole, 80)
	}
	if param.ControlRelevance != "" {
		row["control_relevance"] = pluginLearningPromptClip(param.ControlRelevance, 80)
	}
	if quick {
		row["quick_control"] = true
	}
	if param.HostControllable {
		row["host_controllable"] = true
	}
	if param.IsBoolean {
		row["is_boolean"] = true
	}
	if param.IsDiscrete {
		row["is_discrete"] = true
	}
	if param.Unit != "" {
		row["unit_hint"] = pluginLearningPromptClip(param.Unit, 40)
	}
	if param.ValueText != "" {
		row["current_value_text"] = pluginLearningPromptClip(param.ValueText, 80)
	}
	if param.DisplayProbe != nil {
		row["display_probe"] = pluginLearningCompactDisplayProbe(param.DisplayProbe)
	}
	if param.DisplayDomainCandidate != nil {
		row["display_domain_candidate"] = mapFromJSONStruct(param.DisplayDomainCandidate)
	}
	return row
}

func pluginUIReferenceAddFocusBucket(buckets map[string]*pluginUIReferenceFocusBucket, name, id string) {
	name = strings.TrimSpace(name)
	id = strings.TrimSpace(id)
	if name == "" || id == "" {
		return
	}
	bucket := buckets[name]
	if bucket == nil {
		bucket = &pluginUIReferenceFocusBucket{Name: name}
		buckets[name] = bucket
	}
	bucket.Count++
	if !pluginUIReferenceContainsString(bucket.IDs, id) && len(bucket.IDs) < 16 {
		bucket.IDs = append(bucket.IDs, id)
	}
}

func pluginUIReferenceContainsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func pluginUIReferenceFocusRows(buckets map[string]*pluginUIReferenceFocusBucket, limit, idLimit int) []map[string]any {
	rows := make([]*pluginUIReferenceFocusBucket, 0, len(buckets))
	for _, bucket := range buckets {
		rows = append(rows, bucket)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Count == rows[j].Count {
			return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
		}
		return rows[i].Count > rows[j].Count
	})
	if limit >= 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, bucket := range rows {
		out = append(out, map[string]any{
			"text":                bucket.Name,
			"parameter_count":     bucket.Count,
			"candidate_param_ids": firstStringLimit(bucket.IDs, idLimit),
		})
	}
	return out
}

func pluginUIReferenceFocusTerms(param pluginParameterInfo, quick bool) []string {
	values := []string{param.Name, param.RawName, param.Alias, param.DisplayGroup, param.NormalizedRole}
	if param.DisplayProbe != nil {
		values = append(values, param.DisplayProbe.Label, param.DisplayProbe.CurrentText)
		for _, sample := range param.DisplayProbe.Samples {
			values = append(values, sample.Text)
			if len(values) > 18 {
				break
			}
		}
	}
	seen := map[string]bool{}
	terms := []string{}
	for _, term := range pluginUIReferenceTerms(values...) {
		term = pluginUIReferenceNormalizeFocusTerm(term)
		if term == "" || seen[term] || pluginUIReferenceFocusStopWord(term) {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
	}
	if quick {
		for _, term := range []string{"quick_control", "front_panel"} {
			if !seen[term] {
				seen[term] = true
				terms = append(terms, term)
			}
		}
	}
	return terms
}

func pluginUIReferenceFocusUnits(param pluginParameterInfo) []string {
	values := []string{param.Unit, param.ValueText}
	if param.DisplayProbe != nil {
		values = append(values, param.DisplayProbe.CurrentText, param.DisplayProbe.Label)
		for _, sample := range param.DisplayProbe.Samples {
			values = append(values, sample.Text)
			if len(values) > 12 {
				break
			}
		}
	}
	seen := map[string]bool{}
	units := []string{}
	for _, value := range values {
		text := strings.ToLower(strings.TrimSpace(value))
		if text == "" {
			continue
		}
		for _, unit := range []string{"khz", "hz", "db", "ms", "sec", "s", "%", "percent", "ratio", "x", "bpm", "note", "semitone", "st"} {
			if !pluginUIReferenceTextContainsUnit(text, unit) {
				continue
			}
			normalized := pluginUIReferenceNormalizeUnit(unit)
			if normalized != "" && !seen[normalized] {
				seen[normalized] = true
				units = append(units, normalized)
			}
		}
	}
	return units
}

func pluginUIReferenceTextContainsUnit(text, unit string) bool {
	if unit == "%" {
		return strings.Contains(text, "%")
	}
	if unit == "s" {
		return pluginUIReferenceSecondUnitPattern.MatchString(text)
	}
	if unit == "x" {
		return strings.Contains(text, ":1") || strings.Contains(text, "ratio")
	}
	return strings.Contains(text, unit)
}

func pluginUIReferenceNormalizeUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "khz":
		return "kHz"
	case "hz":
		return "Hz"
	case "db":
		return "dB"
	case "ms":
		return "ms"
	case "sec", "s":
		return "s"
	case "%", "percent":
		return "%"
	case "ratio", "x":
		return "ratio"
	case "bpm":
		return "BPM"
	case "note":
		return "note"
	case "semitone", "st":
		return "semitone"
	default:
		return strings.TrimSpace(unit)
	}
}

func pluginUIReferenceNormalizeFocusTerm(term string) string {
	term = strings.TrimSpace(strings.ToLower(term))
	term = strings.Trim(term, "_- ")
	if len(term) < 2 {
		return ""
	}
	switch term {
	case "db":
		return "dB"
	case "hz":
		return "Hz"
	case "khz":
		return "kHz"
	default:
		return term
	}
}

func pluginUIReferenceFocusStopWord(term string) bool {
	switch term {
	case "param", "parameter", "plugin", "control", "controls", "value", "default", "current", "normalized", "host", "automation", "auto", "other", "unknown", "true", "false":
		return true
	default:
		return false
	}
}

func pluginUIReferenceBackendVisualGuide(digest pluginParameterDigest) map[string]any {
	const maxCandidates = 160
	candidates := make([]map[string]any, 0, len(digest.Parameters))
	for _, param := range digest.Parameters {
		id := strings.TrimSpace(param.ID)
		if id == "" {
			continue
		}
		row := map[string]any{"param_id": id}
		if param.Name != "" {
			row["name"] = param.Name
		}
		if param.RawName != "" && param.RawName != param.Name {
			row["raw_name"] = param.RawName
		}
		if param.Alias != "" {
			row["alias"] = param.Alias
		}
		if param.DisplayGroup != "" {
			row["display_group"] = param.DisplayGroup
		}
		if param.NormalizedRole != "" {
			row["normalized_role"] = param.NormalizedRole
		}
		if param.ControlRelevance != "" {
			row["control_relevance"] = param.ControlRelevance
		}
		if param.Unit != "" {
			row["unit_hint"] = param.Unit
		}
		if param.ValueText != "" {
			row["current_value_text"] = param.ValueText
		}
		if param.IsBoolean {
			row["is_boolean"] = true
		}
		if param.IsDiscrete {
			row["is_discrete"] = true
		}
		if param.DisplayProbe != nil {
			probe := map[string]any{}
			if param.DisplayProbe.CurrentText != "" {
				probe["current_text"] = param.DisplayProbe.CurrentText
			}
			if param.DisplayProbe.Label != "" {
				probe["label"] = param.DisplayProbe.Label
			}
			samples := make([]string, 0, len(param.DisplayProbe.Samples))
			for _, sample := range param.DisplayProbe.Samples {
				if strings.TrimSpace(sample.Text) == "" {
					continue
				}
				samples = append(samples, sample.Text)
				if len(samples) >= 3 {
					break
				}
			}
			if len(samples) > 0 {
				probe["sample_texts"] = samples
			}
			if len(probe) > 0 {
				row["display_probe"] = probe
			}
		}
		candidates = append(candidates, row)
		if len(candidates) >= maxCandidates {
			break
		}
	}
	out := map[string]any{
		"schema":                 "plugin_backend_visual_search_guide.v1",
		"parameter_count":        digest.ParameterCount,
		"candidate_count":        len(candidates),
		"candidates_truncated":   len(digest.Parameters) > len(candidates),
		"candidate_parameters":   candidates,
		"param_id_source_policy": "candidate matches may only use param_id values listed here",
	}
	if digest.PluginClass != "" {
		out["plugin_class_hint"] = digest.PluginClass
	}
	return out
}

func pluginUIReferenceBackendCandidateDigest(digest pluginParameterDigest) map[string]any {
	quickSet := map[string]bool{}
	quickRows := make([]map[string]any, 0, len(digest.QuickControls))
	for _, quick := range digest.QuickControls {
		id := strings.TrimSpace(quick.ParamID)
		if id == "" {
			continue
		}
		quickSet[id] = true
		row := map[string]any{"param_id": id}
		if quick.Label != "" {
			row["label"] = quick.Label
		}
		if quick.DisplayGroup != "" {
			row["display_group"] = quick.DisplayGroup
		}
		if quick.NormalizedRole != "" {
			row["normalized_role"] = quick.NormalizedRole
		}
		quickRows = append(quickRows, row)
		if len(quickRows) >= 32 {
			break
		}
	}

	candidates := make([]map[string]any, 0, len(digest.Parameters))
	for _, param := range digest.Parameters {
		id := strings.TrimSpace(param.ID)
		if id == "" {
			continue
		}
		row := map[string]any{"param_id": id}
		if param.Name != "" {
			row["name"] = param.Name
		}
		if param.RawName != "" && param.RawName != param.Name {
			row["raw_name"] = param.RawName
		}
		if param.Alias != "" {
			row["alias"] = param.Alias
		}
		if param.DisplayGroup != "" {
			row["display_group"] = param.DisplayGroup
		}
		if param.NormalizedRole != "" {
			row["normalized_role"] = param.NormalizedRole
		}
		if param.ControlRelevance != "" {
			row["control_relevance"] = param.ControlRelevance
		}
		if quickSet[id] {
			row["quick_control"] = true
		}
		if param.HostControllable {
			row["host_controllable"] = true
		}
		if param.IsBoolean {
			row["is_boolean"] = true
		}
		if param.IsDiscrete {
			row["is_discrete"] = true
		}
		if param.Unit != "" {
			row["unit_hint"] = param.Unit
		}
		if param.ValueText != "" {
			row["current_value_text"] = param.ValueText
		}
		if param.Min != nil || param.Max != nil {
			row["backend_range"] = map[string]any{"min": param.Min, "max": param.Max}
		}
		if param.DisplayDomainCandidate != nil {
			row["display_domain_candidate"] = mapFromJSONStruct(param.DisplayDomainCandidate)
		}
		if param.DisplayProbe != nil {
			row["display_probe"] = pluginUIReferenceCompactDisplayProbe(param.DisplayProbe)
		}
		candidates = append(candidates, row)
	}
	out := map[string]any{
		"schema":                  "plugin_backend_parameter_candidates.v1",
		"parameter_count":         digest.ParameterCount,
		"candidate_count":         len(candidates),
		"parameters_retained":     true,
		"candidates_truncated":    false,
		"quick_controls":          quickRows,
		"candidate_parameters":    candidates,
		"display_probe_summary":   plugingrabber.DisplayProbeSummary(digest),
		"param_id_source_policy":  "Only param_id values listed in candidate_parameters are legal; this list is the complete backend parameter ledger for this matching step.",
		"visual_matching_purpose": "Connect visible UI labels/units/groups to backend controllable parameters with confidence; do not create new parameters.",
	}
	if digest.PluginClass != "" {
		out["plugin_class"] = digest.PluginClass
	}
	return out
}

func pluginUIReferenceCompactDisplayProbe(probe *plugingrabber.ParameterDisplayProbe) map[string]any {
	if probe == nil {
		return nil
	}
	row := map[string]any{}
	if probe.Mode != "" {
		row["mode"] = probe.Mode
	}
	if probe.CurrentText != "" {
		row["current_text"] = probe.CurrentText
	}
	if probe.Label != "" {
		row["label"] = probe.Label
	}
	if len(probe.Capabilities) > 0 {
		row["capabilities"] = firstStringLimit(probe.Capabilities, 6)
	}
	samples := make([]map[string]any, 0, len(probe.Samples))
	for _, sample := range probe.Samples {
		if strings.TrimSpace(sample.Text) == "" && sample.Value == nil {
			continue
		}
		samples = append(samples, map[string]any{
			"normalized_value": sample.NormalizedValue,
			"value":            sample.Value,
			"text":             sample.Text,
		})
		if len(samples) >= 5 {
			break
		}
	}
	if len(samples) > 0 {
		row["samples"] = samples
	}
	labels := make([]map[string]any, 0, len(probe.DiscreteLabels))
	for _, label := range probe.DiscreteLabels {
		if strings.TrimSpace(label.Label) == "" {
			continue
		}
		labels = append(labels, map[string]any{
			"index": label.Index,
			"value": label.Value,
			"label": label.Label,
		})
		if len(labels) >= 8 {
			break
		}
	}
	if len(labels) > 0 {
		row["discrete_labels"] = labels
	}
	if len(probe.AllLabels) > 0 {
		row["labels"] = firstStringLimit(probe.AllLabels, 12)
	}
	if len(probe.Issues) > 0 {
		row["issues"] = firstStringLimit(probe.Issues, 4)
	}
	return row
}

func applyPluginUIReferenceEvidence(patch pluginProfilePatch, uiReference map[string]any) pluginProfilePatch {
	visualDigest := mapValue(uiReference["visual_digest"])
	if len(visualDigest) == 0 {
		return patch
	}
	sessionID := strings.TrimSpace(firstNonEmptyText(uiReference, "plugin_learning_session_id"))
	artifactIDs := stringListValue(firstPresentAny(uiReference, "artifact_ids", "ui_reference_artifact_ids"))
	visualHints := pluginUIReferenceVisualHints(visualDigest)
	allowVisualHintApplication := pluginUIReferenceAllowsVisualHintApplication(visualDigest)
	matches := []map[string]any{}
	evidence := map[string]any{
		"kind":    "plugin_ui_reference_image",
		"summary": "User-provided plugin interface image pattern was used as visual evidence for labels, grouping, layout, and display-domain hints.",
	}
	provenance := map[string]any{
		"kind":    "plugin_ui_reference_image",
		"source":  "plugin_ui_reference_image",
		"summary": "Visual digest schema plugin_ui_reference_digest.v1 was considered during automatic learning.",
		"data": map[string]any{
			"plugin_learning_session_id": sessionID,
			"artifact_ids":               artifactIDs,
		},
	}
	addToParams := func(groupLabel string, params map[string]any) {
		for slot, raw := range params {
			mapping := mapValue(raw)
			paramID := firstNonEmptyText(mapping, "param_id", "id")
			if paramID == "" {
				paramID = strings.TrimSpace(fmt.Sprint(raw))
			}
			if paramID == "" || paramID == "<nil>" {
				continue
			}
			if len(mapping) == 0 {
				mapping = map[string]any{"param_id": paramID}
			}
			if allowVisualHintApplication {
				match, ok := bestPluginUIReferenceVisualHint(visualHints, slot, paramID, firstNonEmptyText(mapping, "label"), groupLabel)
				if ok {
					mapping["visual_confidence"] = match.Confidence
					mapping["visual_match_score"] = match.Score
					if match.ParamID != "" {
						mapping["visual_backend_param_id"] = match.ParamID
					}
					if match.BackendName != "" {
						mapping["visual_backend_name"] = match.BackendName
					}
					if match.MatchReason != "" {
						mapping["visual_match_reason"] = match.MatchReason
					}
					if len(match.Evidence) > 0 {
						mapping["visual_match_evidence"] = match.Evidence
					}
					if match.Label != "" {
						mapping["visual_label"] = match.Label
						if firstNonEmptyText(mapping, "label") == "" || pluginUIReferenceShouldPreferVisualLabel(mapping, match.Score) {
							mapping["label"] = match.Label
						}
						if patch.Aliases == nil {
							patch.Aliases = map[string]string{}
						}
						if patch.Aliases[paramID] == "" || pluginUIReferenceShouldPreferVisualLabel(mapping, match.Score) {
							patch.Aliases[paramID] = match.Label
						}
					}
					if match.Group != "" {
						mapping["visual_group"] = match.Group
						if patch.DisplayGroups == nil {
							patch.DisplayGroups = map[string]string{}
						}
						if patch.DisplayGroups[paramID] == "" || pluginUIReferenceShouldPreferVisualGroup(patch.DisplayGroups[paramID], match.Group, match.Score) {
							patch.DisplayGroups[paramID] = match.Group
						}
					}
					if match.DisplayDomainText != "" {
						mapping["visual_display_domain_text"] = match.DisplayDomainText
						if pluginUIReferenceShouldUseVisualDisplayDomain(mapping, match) {
							mapping["display_domain_text"] = match.DisplayDomainText
							if domain := plugingrabber.ParseDisplayDomainText(match.DisplayDomainText, "plugin_ui_reference_image", false); domain != nil {
								domainMap := mapFromJSONStruct(domain)
								if match.Confidence > 0 {
									domainMap["confidence"] = pluginUIReferenceDisplayDomainConfidence(match.Confidence)
								}
								mapping["display_domain"] = domainMap
							}
						}
					}
					matches = append(matches, map[string]any{
						"slot":                strings.TrimSpace(fmt.Sprint(slot)),
						"param_id":            paramID,
						"visual_label":        match.Label,
						"visual_group":        match.Group,
						"backend_name":        match.BackendName,
						"display_domain_text": match.DisplayDomainText,
						"match_reason":        match.MatchReason,
						"evidence":            match.Evidence,
						"score":               match.Score,
						"confidence":          match.Confidence,
					})
				}
			}
			mapping["evidence"] = append(mapRowsValue(mapping["evidence"]), evidence)
			mapping["provenance"] = append(mapRowsValue(mapping["provenance"]), provenance)
			params[slot] = mapping
		}
	}
	for _, group := range patch.Groups {
		params := mapValue(group["params"])
		if len(params) == 0 {
			continue
		}
		groupLabel := firstNonEmptyText(group, "label", "name", "id", "role")
		if label := pluginUIReferencePreferredGroupLabel(visualHints, groupLabel); allowVisualHintApplication && label != "" {
			group["visual_label"] = label
			if groupLabel == "" || strings.HasPrefix(strings.ToLower(groupLabel), "b") {
				group["label"] = label
			}
		}
		addToParams(groupLabel, params)
		group["params"] = params
	}
	for _, control := range patch.VirtualControls {
		params := mapValue(control["params"])
		if len(params) == 0 {
			continue
		}
		addToParams(firstNonEmptyText(control, "label", "name", "operation", "component_id"), params)
		control["params"] = params
	}
	if len(matches) > 0 {
		uiReference["visual_matches"] = matches
	}
	return patch
}

type pluginUIReferenceVisualHint struct {
	ParamID           string
	Label             string
	Group             string
	BackendName       string
	DisplayDomainText string
	MatchReason       string
	Evidence          []string
	Confidence        float64
	Terms             []string
}

func pluginUIReferenceAllowsVisualHintApplication(visualDigest map[string]any) bool {
	status := strings.ToLower(strings.TrimSpace(firstNonEmptyText(visualDigest, "matching_status")))
	switch status {
	case "matched":
		return true
	case "", "image_guided", "image_guided_match_failed", "match_failed", "image_extract_only":
		return false
	default:
		return false
	}
}

func pluginUIReferenceVisualHints(visualDigest map[string]any) []pluginUIReferenceVisualHint {
	hints := []pluginUIReferenceVisualHint{}
	add := func(paramID, label, group, backendName, displayDomain, matchReason string, evidence []string, confidence float64) {
		paramID = strings.TrimSpace(paramID)
		label = strings.TrimSpace(label)
		group = strings.TrimSpace(group)
		backendName = strings.TrimSpace(backendName)
		displayDomain = strings.TrimSpace(displayDomain)
		matchReason = strings.TrimSpace(matchReason)
		if paramID == "" {
			return
		}
		if confidence <= 0 {
			confidence = 0.5
		}
		hint := pluginUIReferenceVisualHint{
			ParamID:           paramID,
			Label:             label,
			Group:             group,
			BackendName:       backendName,
			DisplayDomainText: displayDomain,
			MatchReason:       matchReason,
			Evidence:          compactStringList(evidence),
			Confidence:        confidence,
			Terms:             pluginUIReferenceTerms(paramID, label, group, backendName, displayDomain),
		}
		hints = append(hints, hint)
	}
	for _, row := range mapRowsValue(visualDigest["candidate_parameter_matches"]) {
		displayDomain := firstNonEmptyText(row, "display_domain", "display_domain_hint", "range_or_values", "range", "values")
		unit := firstNonEmptyText(row, "visible_unit", "unit")
		if displayDomain != "" && unit != "" && !strings.Contains(strings.ToLower(displayDomain), strings.ToLower(unit)) && unit != "unknown" {
			displayDomain = strings.TrimSpace(displayDomain + " " + unit)
		}
		if displayDomain == "" && unit != "" && unit != "unknown" {
			displayDomain = unit
		}
		add(
			firstNonEmptyText(row, "backend_param_id", "param_id"),
			firstNonEmptyText(row, "visible_label", "ui_label", "label", "name", "text"),
			firstNonEmptyText(row, "visible_group", "ui_group", "group", "section"),
			firstNonEmptyText(row, "backend_param_name", "backend_name", "candidate_name"),
			displayDomain,
			firstNonEmptyText(row, "match_reason", "reason", "summary"),
			stringListValue(row["evidence"]),
			floatNumber(row["confidence"]),
		)
	}
	for _, row := range mapRowsValue(visualDigest["visible_controls"]) {
		add(
			"",
			firstNonEmptyText(row, "label", "name", "text"),
			firstNonEmptyText(row, "group", "section"),
			"",
			firstNonEmptyText(row, "display_domain", "display_domain_text", "range", "unit", "value"),
			"",
			nil,
			floatNumber(row["confidence"]),
		)
	}
	for _, row := range mapRowsValue(visualDigest["units_and_display_domains"]) {
		displayDomain := firstNonEmptyText(row, "range_or_values", "display_domain", "display_domain_text", "range", "values")
		unit := firstNonEmptyText(row, "unit")
		if displayDomain != "" && unit != "" && !strings.Contains(strings.ToLower(displayDomain), strings.ToLower(unit)) && unit != "unknown" {
			displayDomain = strings.TrimSpace(displayDomain + " " + unit)
		}
		add(
			"",
			firstNonEmptyText(row, "label", "name", "text"),
			firstNonEmptyText(row, "group", "section"),
			"",
			displayDomain,
			"",
			nil,
			floatNumber(row["confidence"]),
		)
	}
	for _, row := range mapRowsValue(visualDigest["groups"]) {
		add(
			"",
			firstNonEmptyText(row, "label", "name"),
			firstNonEmptyText(row, "label", "name"),
			"",
			"",
			"",
			nil,
			floatNumber(row["confidence"]),
		)
	}
	return hints
}

type pluginUIReferenceVisualMatch struct {
	pluginUIReferenceVisualHint
	Score float64
}

func bestPluginUIReferenceVisualHint(hints []pluginUIReferenceVisualHint, slot any, paramID, mappingLabel, groupLabel string) (pluginUIReferenceVisualMatch, bool) {
	contextTerms := pluginUIReferenceTerms(strings.TrimSpace(fmt.Sprint(slot)), paramID, mappingLabel, groupLabel)
	if len(contextTerms) == 0 {
		return pluginUIReferenceVisualMatch{}, false
	}
	best := pluginUIReferenceVisualMatch{}
	for _, hint := range hints {
		if hint.ParamID != "" && hint.ParamID != paramID {
			continue
		}
		score := pluginUIReferenceHintScore(contextTerms, hint)
		if hint.ParamID != "" && hint.ParamID == paramID {
			if score == 0 {
				score = 0.70
			}
			if score < 0.78 {
				score = 0.78
			}
			if hint.Confidence > 0 {
				score = score * (0.75 + 0.25*hint.Confidence)
			}
		}
		if score > best.Score {
			best = pluginUIReferenceVisualMatch{pluginUIReferenceVisualHint: hint, Score: score}
		}
	}
	if best.Score < 0.34 {
		return pluginUIReferenceVisualMatch{}, false
	}
	return best, true
}

func pluginUIReferenceHintScore(contextTerms []string, hint pluginUIReferenceVisualHint) float64 {
	if len(hint.Terms) == 0 {
		return 0
	}
	hintSet := map[string]bool{}
	for _, term := range hint.Terms {
		hintSet[term] = true
	}
	matches := 0
	for _, term := range contextTerms {
		if hintSet[term] {
			matches++
		}
	}
	if matches == 0 {
		return 0
	}
	denom := float64(len(contextTerms))
	if len(hint.Terms) < len(contextTerms) {
		denom = float64(len(hint.Terms))
	}
	score := float64(matches) / denom
	if hint.Confidence > 0 {
		score = score * (0.65 + 0.35*hint.Confidence)
	}
	if pluginUIReferenceSharesBandAndRole(contextTerms, hintSet) && score < 0.82 {
		score = 0.82
	}
	return score
}

func pluginUIReferenceTerms(values ...string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(term string) {
		term = strings.Trim(strings.ToLower(term), " \t\r\n_-:;,.()[]{}")
		if term == "" || seen[term] || len(term) <= 1 && term < "0" || len(term) <= 1 && term > "9" {
			return
		}
		seen[term] = true
		out = append(out, term)
	}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		add(value)
		var b strings.Builder
		for _, r := range value {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				b.WriteRune(r)
			default:
				b.WriteRune(' ')
			}
		}
		for _, term := range strings.Fields(b.String()) {
			add(term)
			if matches := pluginUIReferenceBandTokenPattern.FindStringSubmatch(term); len(matches) == 2 {
				add("band")
				add(matches[1])
				add("band" + matches[1])
			}
			if term == "band" {
				add("b")
			}
			if strings.HasPrefix(term, "frequency") {
				add("freq")
			}
			if term == "freq" {
				add("frequency")
			}
			if term == "db" {
				add("gain")
			}
		}
		terms := strings.Fields(b.String())
		for i := 0; i+1 < len(terms); i++ {
			if terms[i] == "band" && pluginUIReferenceIsDigits(terms[i+1]) {
				add("b" + terms[i+1])
				add("band" + terms[i+1])
			}
		}
	}
	return out
}

func pluginUIReferenceSharesBandAndRole(contextTerms []string, hintSet map[string]bool) bool {
	sharedRole := false
	sharedBand := false
	for _, term := range contextTerms {
		if !hintSet[term] {
			continue
		}
		switch term {
		case "gain", "frequency", "freq", "q", "enable", "active", "bypass":
			sharedRole = true
		}
		if strings.HasPrefix(term, "band") && pluginUIReferenceIsDigits(strings.TrimPrefix(term, "band")) {
			sharedBand = true
		}
		if strings.HasPrefix(term, "b") && pluginUIReferenceIsDigits(strings.TrimPrefix(term, "b")) {
			sharedBand = true
		}
	}
	return sharedRole && sharedBand
}

func pluginUIReferenceIsDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func pluginUIReferenceShouldPreferVisualLabel(mapping map[string]any, score float64) bool {
	source := strings.ToLower(firstNonEmptyText(mapping, "source"))
	return strings.Contains(source, "auto_learn") && score >= 0.75
}

func pluginUIReferenceShouldPreferVisualGroup(existing, visual string, score float64) bool {
	existing = strings.TrimSpace(existing)
	visual = strings.TrimSpace(visual)
	if visual == "" {
		return false
	}
	if existing == "" {
		return true
	}
	return score >= 0.75 && (strings.EqualFold(existing, "eq") || strings.EqualFold(existing, "other") || strings.HasPrefix(strings.ToLower(existing), "b"))
}

func pluginUIReferenceShouldUseVisualDisplayDomain(mapping map[string]any, match pluginUIReferenceVisualMatch) bool {
	if match.DisplayDomainText == "" {
		return false
	}
	if pluginGrabberMappingDomainText(mapping) == "" {
		return true
	}
	if match.Score < 0.50 || match.Confidence < 0.70 {
		return false
	}
	domain := mapValue(mapping["display_domain"])
	source := strings.ToLower(strings.Join([]string{
		firstNonEmptyText(mapping, "source"),
		firstNonEmptyText(domain, "source"),
	}, " "))
	confidence := floatNumber(domain["confidence"])
	if confidence > 0 && confidence < 0.80 {
		return true
	}
	return strings.Contains(source, "auto_learn_eq_slot") || strings.Contains(source, "auto_learn_parameter_shape")
}

func pluginUIReferenceDisplayDomainConfidence(visualConfidence float64) float64 {
	if visualConfidence <= 0 {
		return 0.75
	}
	if visualConfidence < 0.75 {
		return 0.75
	}
	if visualConfidence > 0.90 {
		return 0.90
	}
	return visualConfidence
}

func pluginUIReferencePreferredGroupLabel(hints []pluginUIReferenceVisualHint, groupLabel string) string {
	groupTerms := pluginUIReferenceTerms(groupLabel)
	if len(groupTerms) == 0 {
		return ""
	}
	bestScore := 0.0
	best := ""
	for _, hint := range hints {
		if hint.Group == "" && hint.Label == "" {
			continue
		}
		score := pluginUIReferenceHintScore(groupTerms, hint)
		if score > bestScore {
			bestScore = score
			best = firstNonEmpty(hint.Group, hint.Label)
		}
	}
	if bestScore < 0.5 {
		return ""
	}
	return best
}

func artifactIDsFromSummaries(summaries []artifacts.Summary) []string {
	out := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		if id := strings.TrimSpace(summary.ID); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func compactStringList(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	return out
}

func (s *Server) runPluginGrabberLearningWorkflow(ctx context.Context, conversationID, userText string, requestContext map[string]any, cfg config.EngineConfig, workflowCmd map[string]any) ChatResponse {
	target, err := s.resolvePluginLearningTarget(ctx, workflowCmd, requestContext, userText)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}
	args := workflowCommandArgs(workflowCmd)
	mode := strings.TrimSpace(strings.ToLower(firstNonEmptyText(args, "mode", "learning_mode")))
	if mode == "" {
		mode = string(plugingrabber.LearningModeAutoLearn)
	}
	reviewedAutoLearn := mode == "auto_learn_reviewed" || args["reviewed_profile_patch"] != nil
	if reviewedAutoLearn {
		mode = string(plugingrabber.LearningModeAutoLearn)
	}
	learningSessionID := strings.TrimSpace(firstNonEmptyText(args, "plugin_learning_session_id"))
	uiReferenceDecision := strings.TrimSpace(strings.ToLower(firstNonEmptyText(args, "ui_reference_decision")))
	if mode == string(plugingrabber.LearningModeAutoLearn) && !reviewedAutoLearn && uiReferenceDecision == "" && len(stringListValue(args["ui_reference_artifact_ids"])) == 0 {
		return pluginGrabberUIReferenceRequestResponse(conversationID, userText, requestContext, target, "", "")
	}
	if extra := strings.TrimSpace(firstNonEmptyText(args, "extra_instructions", "supplement", "notes")); extra != "" {
		userText = strings.TrimSpace(userText + "\nAdditional user notes: " + extra)
	}
	if s.kernel == nil {
		err := fmt.Errorf("kernel client is nil")
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}

	stageArtifacts := []artifacts.Summary{}
	autoLearningSession := mode == string(plugingrabber.LearningModeAutoLearn) && learningSessionID != ""
	if autoLearningSession {
		s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "parameters_extracted", "running", map[string]any{
			"operation": "get_plugin_parameters",
		})
	}
	paramsReply, _, err := s.kernel.SendCommand(ctx, map[string]any{
		"cmd":       "get_plugin_parameters",
		"track_id":  target.TrackID,
		"plugin_id": target.PluginID,
	})
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}
	if !kernelReplyOK(paramsReply) {
		message := firstNonEmptyText(paramsReply, "message", "error")
		if message == "" {
			message = "get_plugin_parameters failed"
		}
		err := fmt.Errorf("%s", message)
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}

	s.observePluginParametersReply(paramsReply)
	digest := buildPluginParameterDigest(paramsReply)
	if autoLearningSession {
		stageArtifacts = appendPluginLearningStageArtifact(stageArtifacts, s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "parameters_extracted", "provided", map[string]any{
			"parameter_digest": pluginLearningCompactParameterDigest(digest),
		}))
	}
	var uiReference map[string]any
	var uiReferenceArtifacts []artifacts.Summary
	var webReference map[string]any
	var pluginTypeHypothesis map[string]any
	if mode == string(plugingrabber.LearningModeAutoLearn) && !reviewedAutoLearn {
		if autoLearningSession {
			s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "web_reference", "running", map[string]any{
				"decision": firstNonEmptyText(args, "web_reference_decision"),
			})
		}
		webReference = s.resolvePluginWebReference(ctx, cfg, args, target, digest)
		stageArtifacts = appendPluginLearningStageArtifact(stageArtifacts, s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "web_reference", firstNonEmpty(firstNonEmptyText(webReference, "status"), "skipped"), pluginLearningCompactWebReference(webReference)))
		if pluginLearningShouldBuildTypeHypothesis(args, webReference) {
			if autoLearningSession {
				s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "plugin_type_hypothesis", "running", nil)
			}
			pluginTypeHypothesis = s.buildPluginTypeHypothesis(ctx, cfg, target, digest, webReference)
			stageArtifacts = appendPluginLearningStageArtifact(stageArtifacts, s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "plugin_type_hypothesis", firstNonEmpty(firstNonEmptyText(pluginTypeHypothesis, "status"), "skipped"), pluginLearningCompactTypeHypothesis(pluginTypeHypothesis)))
		} else if autoLearningSession {
			stageArtifacts = appendPluginLearningStageArtifact(stageArtifacts, s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "plugin_type_hypothesis", "skipped", map[string]any{
				"reason": "type hypothesis was not required for this learning pass",
			}))
		}
		evidence := map[string]any{}
		if len(pluginTypeHypothesis) > 0 {
			evidence["plugin_type_hypothesis"] = pluginTypeHypothesis
		}
		if len(webReference) > 0 {
			evidence["plugin_web_reference"] = webReference
		}
		if autoLearningSession {
			s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "ui_reference", "running", map[string]any{
				"decision": firstNonEmptyText(args, "ui_reference_decision"),
			})
		}
		uiReference, uiReferenceArtifacts, err = s.resolvePluginUIReference(ctx, cfg, args, target, digest, evidence)
		if err != nil {
			return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
		}
		uiStagePayload := map[string]any{
			"status":                      firstNonEmptyText(uiReference, "status"),
			"decision":                    firstNonEmptyText(uiReference, "decision"),
			"ui_reference_artifact_ids":   stringListValue(firstPresentAny(uiReference, "artifact_ids", "ui_reference_artifact_ids")),
			"visual_digest_schema":        firstNonEmptyText(uiReference, "visual_digest_schema"),
			"visual_digest_source_policy": firstNonEmptyText(uiReference, "visual_digest_source_policy"),
			"warnings":                    firstStringLimit(stringListValue(uiReference["warnings"]), 8),
		}
		if visualDigest := mapValue(uiReference["visual_digest"]); len(visualDigest) > 0 {
			uiStagePayload["visual_digest"] = pluginLearningCompactUIReferenceDigest(visualDigest)
		}
		stageArtifacts = appendPluginLearningStageArtifact(stageArtifacts, s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "ui_reference", firstNonEmpty(firstNonEmptyText(uiReference, "status"), "skipped"), uiStagePayload))
	}
	if len(uiReference) == 0 {
		uiReference = mapValue(args["ui_reference"])
	}
	if autoLearningSession && !reviewedAutoLearn {
		s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "draft_skill", "running", nil)
	}
	var patch pluginProfilePatch
	var workflowData map[string]any
	switch mode {
	case string(plugingrabber.LearningModeTeach):
		rows := plugingrabber.TeachModeRowsFromAny(args["teach_mode_rows"])
		patch, workflowData, err = plugingrabber.BuildTeachModeProfilePatch(digest, rows)
		if err != nil {
			return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
		}
	default:
		if reviewedAutoLearn {
			patch, err = reviewedProfilePatchFromArgs(args)
			if err != nil {
				return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
			}
			applyReviewedDisplayDomains(&patch, args["display_domain_reviews"])
			workflowData = map[string]any{
				"mode":                  string(plugingrabber.LearningModeAutoLearn),
				"stage":                 "final_confirmation",
				"strategy":              "reviewed_profile_patch",
				"class":                 patch.Class,
				"plugin_name":           firstNonEmpty(digest.PluginName, target.PluginName),
				"parameter_count":       digest.ParameterCount,
				"component_count":       len(patch.Groups),
				"operation_count":       len(patch.VirtualControls),
				"display_probe_summary": plugingrabber.DisplayProbeSummary(digest),
			}
		} else if deterministicPatch, summary, ok := plugingrabber.BuildAutoLearnProfilePatch(digest); ok && !pluginLearningEvidenceShouldForceLLMPatch(uiReference, webReference, pluginTypeHypothesis) {
			patch = deterministicPatch
			workflowData = summary
			if len(mapValue(uiReference["visual_digest"])) > 0 {
				workflowData["strategy"] = firstNonEmptyText(workflowData, "strategy") + "+ui_reference_visual_digest"
			}
		} else {
			promptDigest := pluginLearningPromptDigestForDraft(digest, uiReference, webReference, pluginTypeHypothesis, false)
			digestJSON, err := json.MarshalIndent(promptDigest, "", "  ")
			if err != nil {
				return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
			}
			if len(digestJSON) > pluginLearningDigestMaxBytes {
				promptDigest = pluginLearningPromptDigestForDraft(digest, uiReference, webReference, pluginTypeHypothesis, true)
				promptDigest["prompt_compaction"] = map[string]any{
					"schema": "plugin_learning_prompt_compaction.v1",
					"reason": "Draft prompt exceeded the controlled learning budget after optional evidence was added.",
					"policy": "All backend parameter IDs are retained; verbose evidence and display probes are compacted.",
				}
				digestJSON, err = json.MarshalIndent(promptDigest, "", "  ")
				if err != nil {
					return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
				}
			}
			if len(digestJSON) > pluginLearningDigestMaxBytes {
				err := fmt.Errorf("插件参数摘要仍然过大，无法进入受控学习流程：%d bytes", len(digestJSON))
				return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
			}
			patch, err = s.proposePluginProfilePatch(ctx, cfg, target, userText, string(digestJSON))
			if err != nil {
				return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
			}
			workflowData = map[string]any{
				"mode":                  string(plugingrabber.LearningModeAutoLearn),
				"strategy":              pluginLearningLLMStrategy(uiReference, webReference, pluginTypeHypothesis),
				"class":                 patch.Class,
				"plugin_name":           firstNonEmpty(digest.PluginName, target.PluginName),
				"parameter_count":       digest.ParameterCount,
				"component_count":       len(patch.Groups),
				"operation_count":       len(patch.VirtualControls),
				"components":            plugingrabber.BuildContextPack(digest)["components"],
				"display_probe_summary": plugingrabber.DisplayProbeSummary(digest),
			}
		}
	}
	patch = applyPluginUIReferenceEvidence(patch, uiReference)
	patch, err = validatePluginProfilePatch(patch, digest)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}
	patch = enrichPluginProfilePatchDisplayDomains(patch, digest)
	patch = reconcilePluginProfilePatchVisualDisplayDomains(patch, digest, uiReference)
	patch = canonicalPluginSkillPatch(patch)

	upsert, validation, err := buildPluginProfileUpsertCommand(target, patch, digest)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}
	if workflowData == nil {
		workflowData = map[string]any{}
	}
	workflowData["target"] = map[string]any{
		"track_id":    target.TrackID,
		"plugin_id":   target.PluginID,
		"plugin_name": firstNonEmpty(target.PluginName, digest.PluginName),
	}
	workflowData["conversation_id"] = conversationID
	workflowData["request_context"] = requestContext
	workflowData["plan_id"] = ""
	workflowData["validation_warnings"] = validation.Warnings
	workflowData["validation_summary"] = validation.CoverageSummary
	workflowData["profile_patch"] = patch
	workflowData["plugin_identity"] = digest.PluginIdentity
	if value := firstPresentAny(upsert, "param_signature_hash"); value != nil {
		workflowData["param_signature_hash"] = value
	}
	workflowData["experiments"] = buildPluginLearningExperiments(digest, patch, 3)
	if learningSessionID != "" {
		workflowData["plugin_learning_session_id"] = learningSessionID
	}
	if len(pluginTypeHypothesis) > 0 {
		workflowData["plugin_type_hypothesis"] = pluginTypeHypothesis
	}
	if len(webReference) > 0 && firstNonEmptyText(webReference, "status") != "skipped" {
		workflowData["plugin_web_reference"] = webReference
	}
	if len(uiReference) > 0 {
		workflowData["ui_reference"] = uiReference
	}
	if coverage := buildPluginCoreControlCoverage(pluginTypeHypothesis, uiReference, patch); len(coverage) > 0 {
		workflowData["core_control_coverage"] = coverage
	}
	if len(uiReferenceArtifacts) > 0 {
		workflowData["ui_reference_artifacts"] = artifactSummaryRows(uiReferenceArtifacts)
	} else if inheritedArtifacts := firstPresentAny(args, "ui_reference_artifacts"); inheritedArtifacts != nil {
		workflowData["ui_reference_artifacts"] = inheritedArtifacts
	}
	if mode == string(plugingrabber.LearningModeAutoLearn) && learningSessionID != "" {
		draftPayload := map[string]any{
			"stage":                 firstNonEmpty(firstNonEmptyText(workflowData, "stage"), "candidate_review"),
			"strategy":              firstNonEmptyText(workflowData, "strategy"),
			"class":                 patch.Class,
			"component_count":       len(patch.Groups),
			"operation_count":       len(patch.VirtualControls),
			"validation_summary":    validation.CoverageSummary,
			"validation_warnings":   validation.Warnings,
			"profile_patch":         patch,
			"experiments":           workflowData["experiments"],
			"core_control_coverage": workflowData["core_control_coverage"],
		}
		stageArtifacts = appendPluginLearningStageArtifact(stageArtifacts, s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "draft_skill", "candidate_review", draftPayload))
		if len(stageArtifacts) > 0 {
			workflowData["plugin_learning_session_artifacts"] = stageArtifacts
		}
	}
	if mode == string(plugingrabber.LearningModeAutoLearn) && !reviewedAutoLearn {
		workflowData["stage"] = "candidate_review"
		workflowData["needs_user_review"] = true
		reply := strings.TrimSpace(patch.Reply)
		if reply == "" {
			reply = "已生成插件抓手候选，请先测试并确认显示域。"
		}
		return ChatResponse{
			ConversationID: conversationID,
			Reply:          reply,
			Workflow:       "plugin_grabber_" + mode,
			WorkflowData:   workflowData,
			PluginLearning: workflowData,
		}
	}
	decisions := policy.Analyze([]map[string]any{upsert})
	preview, err := s.confirmationPreview(ctx, decisions, requestContext)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error(), Commands: decisions}
	}
	plan := PendingPlan{
		ID:           "plan_" + randomID(),
		CreatedAt:    time.Now(),
		Decisions:    decisions,
		Context:      requestContext,
		Preview:      preview,
		Workflow:     "plugin_grabber_" + mode,
		WorkflowData: workflowData,
	}
	s.mu.Lock()
	s.pending[plan.ID] = plan
	s.mu.Unlock()
	workflowData["plan_id"] = plan.ID
	if mode == string(plugingrabber.LearningModeAutoLearn) && learningSessionID != "" {
		stageArtifacts = appendPluginLearningStageArtifact(stageArtifacts, s.recordPluginLearningStage(conversationID, requestContext, learningSessionID, target, "final_confirmation", "pending_confirmation", map[string]any{
			"plan_id":             plan.ID,
			"strategy":            firstNonEmptyText(workflowData, "strategy"),
			"class":               patch.Class,
			"component_count":     len(patch.Groups),
			"operation_count":     len(patch.VirtualControls),
			"validation_summary":  validation.CoverageSummary,
			"validation_warnings": validation.Warnings,
			"profile_patch":       patch,
		}))
		if len(stageArtifacts) > 0 {
			workflowData["plugin_learning_session_artifacts"] = stageArtifacts
		}
	}

	reply := strings.TrimSpace(patch.Reply)
	if reply == "" {
		reply = "已准备保存 Plugin Skill。"
	}
	artifactSummaries := s.storePluginProfilePatchArtifact(conversationID, target, patch, workflowData)
	var sidePanelRequest *SidePanelRequest
	if len(artifactSummaries) > 0 {
		sidePanelRequest = &SidePanelRequest{View: "artifact", Tab: "media", ArtifactID: artifactSummaries[0].ID}
		workflowData["artifacts"] = artifactSummaries
		workflowData["profile_patch_artifact"] = artifactSummaries[0]
	}
	if sidePanelRequest != nil {
		workflowData["side_panel_request"] = sidePanelRequest
	}
	return ChatResponse{
		ConversationID:    conversationID,
		Reply:             reply + "\n\n" + confirmationReply(decisions),
		NeedsConfirmation: true,
		PlanID:            plan.ID,
		Preview:           preview,
		Workflow:          plan.Workflow,
		WorkflowData:      workflowData,
		PluginLearning:    workflowData,
		Commands:          decisions,
		Artifacts:         artifactSummaries,
		SidePanelRequest:  sidePanelRequest,
	}
}

func (s *Server) resolvePluginLearningTarget(ctx context.Context, workflowCmd map[string]any, requestContext map[string]any, userText string) (pluginLearningTarget, error) {
	args := workflowCommandArgs(workflowCmd)
	target := pluginLearningTarget{
		TrackID:    firstNonEmptyText(args, "track_id", "selected_plugin_track_id", "selected_track_id"),
		PluginID:   firstNonEmptyText(args, "plugin_id", "selected_plugin_id", "plugin_item_id"),
		PluginName: firstNonEmptyText(args, "plugin_name", "selected_plugin_name", "plugin"),
		Intent:     firstNonEmptyText(args, "intent", "user_intent"),
	}
	if target.Intent == "" {
		target.Intent = strings.TrimSpace(userText)
	}
	if target.TrackID == "" {
		target.TrackID = firstNonEmptyText(requestContext, "selected_plugin_track_id", "selected_track_id", "track_id")
	}
	if target.PluginID == "" {
		target.PluginID = firstNonEmptyText(requestContext, "selected_plugin_id", "plugin_id")
	}
	if target.PluginName == "" {
		target.PluginName = firstNonEmptyText(requestContext, "selected_plugin_name", "plugin_name")
	}
	if target.TrackID != "" && target.PluginID != "" {
		return target, nil
	}

	refs := chatVisiblePluginRefs(s.harness.UserStateSummary(ctx), target.TrackID)
	if target.PluginID == "" && target.PluginName != "" {
		matches := make([]chatPluginRef, 0, 1)
		for _, ref := range refs {
			if strings.EqualFold(ref.Name, target.PluginName) {
				matches = append(matches, ref)
			}
		}
		switch len(matches) {
		case 1:
			target.PluginID = matches[0].ID
			if target.TrackID == "" {
				target.TrackID = matches[0].TrackID
			}
		case 0:
			return target, fmt.Errorf("plugin target not found: %s", target.PluginName)
		default:
			return target, fmt.Errorf("multiple plugins named %q; select one plugin or specify plugin_id", target.PluginName)
		}
	}
	if target.PluginID == "" && len(refs) == 1 {
		target.PluginID = refs[0].ID
		target.PluginName = refs[0].Name
		if target.TrackID == "" {
			target.TrackID = refs[0].TrackID
		}
	}
	if target.TrackID == "" && target.PluginID != "" {
		for _, ref := range chatVisiblePluginRefs(s.harness.UserStateSummary(ctx), "") {
			if ref.ID == target.PluginID {
				target.TrackID = ref.TrackID
				if target.PluginName == "" {
					target.PluginName = ref.Name
				}
				break
			}
		}
	}
	if target.TrackID == "" || target.PluginID == "" {
		return target, fmt.Errorf("plugin target is ambiguous; open/select a plugin parameter panel or specify track_id and plugin_id")
	}
	return target, nil
}

func (s *Server) proposePluginProfilePatch(ctx context.Context, cfg config.EngineConfig, target pluginLearningTarget, userText, digestJSON string) (pluginProfilePatch, error) {
	system := `You produce project-scoped Vit DAW plugin grabber profile patches.
Return ONLY JSON:
{"reply":"short user-facing explanation","quick_control_ids":["param id"],"aliases":{"param id":"label"},"display_groups":{"param id":"Tone|Dynamics|Mix|Modulation|Utility|Other"},"normalized_roles":{"param id":"role"},"class":"eq|compressor|reverb|delay|modulation|saturator|utility|synth|unknown","groups":[{"id":"short_component_id","role":"component role","label":"label","params":{"slot_name":{"param_id":"param id","label":"label","confidence":0.65}}}],"virtual_controls":[{"name":"musical operation","inputs":["plain language input"],"component_id":"short_component_id","resolver":"local_profile_mapping","params":{"slot_name":"param id"}}],"safety":{"notes":"short limits or precautions"}}
Rules:
- Use only parameter IDs that appear in the digest, including inside groups and virtual_controls.
- Do not omit parameters from your reasoning just because they are not quick controls.
- Prefer display_probe and display_domain_candidate evidence when naming display units/ranges; they are plugin-exposed display information, not guaranteed custom UI truth.
- Never treat a parameter's current value_text as the display range limit.
- quick_control_ids are recommendations, not a filter.
- Prefer 4 to 12 quick controls.
- Keep aliases short and musical.
- groups describe semantic plugin components; include only confident mappings.
- virtual_controls describe reusable musical operations; keep them declarative.
- Use Other when unsure.`
	user := fmt.Sprintf("User request: %s\nTarget track_id=%s plugin_id=%s plugin_name=%s\nParameter digest:\n%s", userText, target.TrackID, target.PluginID, target.PluginName, digestJSON)
	assembly := promptruntime.Build(promptruntime.AssemblyInput{
		SystemSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionStatic, "plugin_grabber_profile_patch_system", "", system, true),
		},
		UserSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionRuntime, "plugin_grabber_profile_patch_runtime", "", user, false),
		},
	})
	resp, err := s.llm.CompleteRequest(ctx, cfg, llm.Request{
		Messages: assembly.Messages,
		Timeout:  pluginLearningDraftTimeout,
		Metadata: llm.RequestMetadata{
			Source:            "plugin_grabber_profile_patch",
			PromptFingerprint: assembly.Fingerprint,
			PromptStats:       assembly.Stats.Map(),
		},
	})
	if err != nil {
		return pluginProfilePatch{}, err
	}
	return parsePluginProfilePatch(resp.Text)
}

func buildPluginParameterDigest(reply map[string]any) pluginParameterDigest {
	digest := plugingrabber.BuildParameterDigest(reply)
	return compactPluginLearningMemoryInDigest(digest)
}

func compactPluginLearningMemoryInDigest(digest pluginParameterDigest) pluginParameterDigest {
	digest.GlobalProfile = compactPluginLearningProfileMemory(digest.GlobalProfile)
	digest.PluginSkill = compactPluginLearningPluginSkillMemory(digest.PluginSkill)
	digest.PluginGroups = canonicalPluginSkillRows(digest.PluginGroups, "group")
	digest.VirtualControls = canonicalPluginSkillRows(digest.VirtualControls, "operation")
	digest.SafetyLimits = plugingrabber.SanitizeProfileSafety(digest.SafetyLimits)
	return digest
}

func compactPluginLearningProfileMemory(profile map[string]any) map[string]any {
	if len(profile) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, key := range []string{"schema_version", "profile_id", "plugin_identity", "project_default", "class", "param_signature_hash", "plugin_skill_validator_warnings", "updated_at"} {
		if value := firstPresentAny(profile, key); value != nil {
			out[key] = value
		}
	}
	if groups := canonicalPluginSkillRows(mapRowsValue(profile["groups"]), "group"); len(groups) > 0 {
		out["groups"] = groups
	}
	if controls := canonicalPluginSkillRows(mapRowsValue(profile["virtual_controls"]), "operation"); len(controls) > 0 {
		out["virtual_controls"] = controls
	}
	if safety := plugingrabber.SanitizeProfileSafety(mapValue(profile["safety"])); len(safety) > 0 {
		out["safety"] = safety
	}
	if skill := compactPluginLearningPluginSkillMemory(mapValue(profile["plugin_skill"])); len(skill) > 0 {
		out["plugin_skill"] = skill
	}
	return out
}

func compactPluginLearningPluginSkillMemory(skill map[string]any) map[string]any {
	if len(skill) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, key := range []string{"schema_version", "identity", "capabilities", "safety"} {
		if value := firstPresentAny(skill, key); value != nil {
			out[key] = value
		}
	}
	if components := canonicalPluginSkillRows(mapRowsValue(skill["components"]), "group"); len(components) > 0 {
		out["components"] = components
	}
	if operations := canonicalPluginSkillRows(mapRowsValue(skill["operations"]), "operation"); len(operations) > 0 {
		out["operations"] = operations
	}
	return out
}

func pluginLearningPromptDigestJSON(digest pluginParameterDigest) ([]byte, error) {
	return json.MarshalIndent(pluginLearningPromptDigest(digest), "", "  ")
}

func pluginLearningPromptDigest(digest pluginParameterDigest) map[string]any {
	out := map[string]any{
		"track_id":              digest.TrackID,
		"plugin_id":             digest.PluginID,
		"plugin_name":           digest.PluginName,
		"plugin_identity":       digest.PluginIdentity,
		"template_role":         digest.TemplateRole,
		"profile_source":        digest.ProfileSource,
		"profile_applied":       digest.ProfileApplied,
		"parameter_count":       digest.ParameterCount,
		"quick_controls":        digest.QuickControls,
		"recommended_groups":    digest.RecommendedGroups,
		"display_probe_summary": plugingrabber.DisplayProbeSummary(digest),
	}
	if digest.PluginClass != "" {
		out["plugin_class"] = digest.PluginClass
	}
	if len(digest.ProfileStaleParamIDs) > 0 {
		out["profile_stale_param_ids"] = digest.ProfileStaleParamIDs
	}
	if digest.GlobalProfileApplied || len(digest.PluginSkill) > 0 || len(digest.PluginGroups) > 0 || len(digest.VirtualControls) > 0 {
		pack := plugingrabber.BuildContextPack(digest)
		if runtimeProfile, ok := pack["runtime_profile"].(map[string]any); ok && len(runtimeProfile) > 0 {
			out["runtime_profile"] = runtimeProfile
		}
	}
	parameters := make([]map[string]any, 0, len(digest.Parameters))
	for _, param := range digest.Parameters {
		row := map[string]any{"id": param.ID}
		if param.Name != "" {
			row["name"] = param.Name
		}
		if param.RawName != "" {
			row["raw_param_name"] = param.RawName
		}
		if param.Alias != "" {
			row["alias"] = param.Alias
		}
		if param.DisplayGroup != "" {
			row["display_group"] = param.DisplayGroup
		}
		if param.NormalizedRole != "" {
			row["normalized_role"] = param.NormalizedRole
		}
		if param.ControlRelevance != "" {
			row["control_relevance"] = param.ControlRelevance
		}
		if param.HostControllable {
			row["host_controllable"] = true
		}
		if param.IsBoolean {
			row["is_boolean"] = true
		}
		if param.IsDiscrete {
			row["is_discrete"] = true
		}
		if param.NumSteps > 0 {
			row["num_steps"] = param.NumSteps
		}
		if param.Unit != "" {
			row["unit"] = param.Unit
		}
		if param.ValueText != "" {
			row["value_text"] = param.ValueText
		}
		if param.DisplayProbe != nil {
			row["display_probe"] = param.DisplayProbe
		}
		if param.DisplayDomainCandidate != nil {
			row["display_domain_candidate"] = param.DisplayDomainCandidate
		}
		parameters = append(parameters, row)
	}
	out["parameters"] = parameters
	return out
}

func pluginLearningPromptDigestForDraft(digest pluginParameterDigest, uiReference, webReference, pluginTypeHypothesis map[string]any, compactBase bool) map[string]any {
	out := pluginLearningPromptDigest(digest)
	if compactBase {
		out = pluginLearningCompactParameterDigest(digest)
	}
	out["learning_evidence_policy"] = "Optional web, type, and visual evidence may guide Plugin Skill grouping, display-domain confidence, and experiment priorities. Final param_id mappings must still use backend parameter IDs from this digest."
	if len(pluginTypeHypothesis) > 0 {
		out["plugin_type_hypothesis"] = pluginLearningCompactTypeHypothesis(pluginTypeHypothesis)
	}
	if len(webReference) > 0 && firstNonEmptyText(webReference, "status") != "skipped" {
		out["plugin_web_reference"] = pluginLearningCompactWebReference(webReference)
	}
	if len(uiReference) > 0 {
		if visualDigest := mapValue(uiReference["visual_digest"]); len(visualDigest) > 0 {
			if pluginUIReferenceVisualDigestHasUsableEvidence(visualDigest) {
				out["plugin_ui_reference_image"] = map[string]any{
					"schema":        "plugin_ui_reference_digest.v1",
					"visual_digest": pluginLearningCompactUIReferenceDigest(visualDigest),
					"status":        uiReference["status"],
					"warnings":      firstStringLimit(stringListValue(uiReference["warnings"]), 8),
				}
			} else {
				out["plugin_ui_reference_image"] = map[string]any{
					"schema":          "plugin_ui_reference_digest.v1",
					"status":          firstNonEmpty(firstNonEmptyText(uiReference, "status"), "provided"),
					"matching_status": firstNonEmptyText(visualDigest, "matching_status"),
					"warnings":        firstStringLimit(compactStringList(append(stringListValue(uiReference["warnings"]), stringListValue(visualDigest["warnings"])...)), 8),
					"policy":          "The UI image was provided, but visual probing produced no usable target observations for draft evidence.",
				}
				if value := firstPresentAny(visualDigest, "visual_target_count"); value != nil {
					mapValue(out["plugin_ui_reference_image"])["visual_target_count"] = value
				}
			}
		} else if status := firstNonEmptyText(uiReference, "status"); status != "" && status != "skipped" {
			out["plugin_ui_reference_image"] = map[string]any{
				"schema":   "plugin_ui_reference_digest.v1",
				"status":   status,
				"warnings": firstStringLimit(stringListValue(uiReference["warnings"]), 8),
			}
		}
	}
	return out
}

func pluginLearningCompactParameterDigest(digest pluginParameterDigest) map[string]any {
	out := map[string]any{
		"track_id":                     digest.TrackID,
		"plugin_id":                    digest.PluginID,
		"plugin_name":                  digest.PluginName,
		"plugin_identity":              digest.PluginIdentity,
		"template_role":                digest.TemplateRole,
		"profile_source":               digest.ProfileSource,
		"profile_applied":              digest.ProfileApplied,
		"parameter_count":              digest.ParameterCount,
		"quick_controls":               digest.QuickControls,
		"recommended_groups":           digest.RecommendedGroups,
		"display_probe_summary":        plugingrabber.DisplayProbeSummary(digest),
		"parameters":                   pluginLearningCompactParameterRows(digest),
		"parameters_retained":          true,
		"parameter_compaction_policy":  "All backend parameter IDs are retained; verbose display probe lists are shortened for draft generation only.",
		"final_param_id_source_policy": "Only IDs in parameters[].id are legal final mappings.",
	}
	if digest.PluginClass != "" {
		out["plugin_class"] = digest.PluginClass
	}
	if len(digest.ProfileStaleParamIDs) > 0 {
		out["profile_stale_param_ids"] = digest.ProfileStaleParamIDs
	}
	if digest.GlobalProfileApplied || len(digest.PluginSkill) > 0 || len(digest.PluginGroups) > 0 || len(digest.VirtualControls) > 0 {
		pack := plugingrabber.BuildContextPack(digest)
		if runtimeProfile, ok := pack["runtime_profile"].(map[string]any); ok && len(runtimeProfile) > 0 {
			out["runtime_profile"] = pluginLearningCompactRuntimeProfile(runtimeProfile)
		}
	}
	return out
}

func pluginLearningCompactParameterRows(digest pluginParameterDigest) []map[string]any {
	rows := make([]map[string]any, 0, len(digest.Parameters))
	for _, param := range digest.Parameters {
		id := strings.TrimSpace(param.ID)
		if id == "" {
			continue
		}
		row := map[string]any{"id": id}
		if param.Name != "" {
			row["name"] = pluginLearningPromptClip(param.Name, 160)
		}
		if param.RawName != "" && param.RawName != param.Name {
			row["raw_param_name"] = pluginLearningPromptClip(param.RawName, 160)
		}
		if param.Alias != "" {
			row["alias"] = pluginLearningPromptClip(param.Alias, 160)
		}
		if param.DisplayGroup != "" {
			row["display_group"] = pluginLearningPromptClip(param.DisplayGroup, 120)
		}
		if param.NormalizedRole != "" {
			row["normalized_role"] = pluginLearningPromptClip(param.NormalizedRole, 80)
		}
		if param.ControlRelevance != "" {
			row["control_relevance"] = pluginLearningPromptClip(param.ControlRelevance, 80)
		}
		if param.HostControllable {
			row["host_controllable"] = true
		}
		if param.IsBoolean {
			row["is_boolean"] = true
		}
		if param.IsDiscrete {
			row["is_discrete"] = true
		}
		if param.NumSteps > 0 {
			row["num_steps"] = param.NumSteps
		}
		if param.Unit != "" {
			row["unit"] = pluginLearningPromptClip(param.Unit, 40)
		}
		if param.ValueText != "" {
			row["value_text"] = pluginLearningPromptClip(param.ValueText, 120)
		}
		if param.DisplayProbe != nil {
			row["display_probe"] = pluginLearningCompactDisplayProbe(param.DisplayProbe)
		}
		if param.DisplayDomainCandidate != nil {
			row["display_domain_candidate"] = mapFromJSONStruct(param.DisplayDomainCandidate)
		}
		rows = append(rows, row)
	}
	return rows
}

func pluginLearningCompactDisplayProbe(probe *plugingrabber.ParameterDisplayProbe) map[string]any {
	if probe == nil {
		return nil
	}
	row := map[string]any{}
	if probe.Mode != "" {
		row["mode"] = pluginLearningPromptClip(probe.Mode, 60)
	}
	if probe.CurrentText != "" {
		row["current_text"] = pluginLearningPromptClip(probe.CurrentText, 80)
	}
	if probe.Label != "" {
		row["label"] = pluginLearningPromptClip(probe.Label, 80)
	}
	if len(probe.Capabilities) > 0 {
		row["capabilities"] = pluginLearningCompactStringList(probe.Capabilities, 4, 80)
	}
	samples := make([]map[string]any, 0, minInt(len(probe.Samples), 3))
	for _, sample := range probe.Samples {
		if strings.TrimSpace(sample.Text) == "" && sample.Value == nil {
			continue
		}
		samples = append(samples, map[string]any{
			"normalized_value": sample.NormalizedValue,
			"text":             pluginLearningPromptClip(sample.Text, 80),
		})
		if len(samples) >= 3 {
			break
		}
	}
	if len(samples) > 0 {
		row["samples"] = samples
	}
	labels := make([]map[string]any, 0, minInt(len(probe.DiscreteLabels), 5))
	for _, label := range probe.DiscreteLabels {
		if strings.TrimSpace(label.Label) == "" {
			continue
		}
		labels = append(labels, map[string]any{
			"index": label.Index,
			"label": pluginLearningPromptClip(label.Label, 80),
		})
		if len(labels) >= 5 {
			break
		}
	}
	if len(labels) > 0 {
		row["discrete_labels"] = labels
	}
	if len(probe.AllLabels) > 0 {
		row["labels"] = pluginLearningCompactStringList(probe.AllLabels, 3, 80)
	}
	if len(probe.Issues) > 0 {
		row["issues"] = pluginLearningCompactStringList(probe.Issues, 2, 120)
	}
	if len(row) == 0 {
		return nil
	}
	return row
}

func pluginLearningCompactRuntimeProfile(runtimeProfile map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"class", "source", "status", "profile_key", "param_signature_hash"} {
		if value := firstPresentAny(runtimeProfile, key); value != nil {
			out[key] = value
		}
	}
	if groups := mapRowsValue(runtimeProfile["groups"]); len(groups) > 0 {
		out["groups"] = pluginLearningCompactRows(groups, 16, 120, "id", "role", "label", "name")
	}
	if controls := mapRowsValue(runtimeProfile["virtual_controls"]); len(controls) > 0 {
		out["virtual_controls"] = pluginLearningCompactRows(controls, 16, 120, "name", "component_id", "resolver")
	}
	return out
}

func pluginLearningCompactTypeHypothesis(typeHypothesis map[string]any) map[string]any {
	out := map[string]any{
		"schema":       firstNonEmpty(firstNonEmptyText(typeHypothesis, "schema"), "plugin_type_hypothesis.v1"),
		"status":       firstNonEmptyText(typeHypothesis, "status"),
		"primary_type": firstNonEmptyText(typeHypothesis, "primary_type"),
		"confidence":   typeHypothesis["confidence"],
	}
	if secondary := stringListValue(typeHypothesis["secondary_types"]); len(secondary) > 0 {
		out["secondary_types"] = pluginLearningCompactStringList(secondary, 6, 80)
	}
	if evidence := mapRowsValue(typeHypothesis["evidence"]); len(evidence) > 0 {
		out["evidence"] = pluginLearningCompactRows(evidence, 10, 240, "kind", "summary")
	}
	if missing := stringListValue(typeHypothesis["missing_evidence"]); len(missing) > 0 {
		out["missing_evidence"] = pluginLearningCompactStringList(missing, 8, 180)
	}
	if controls := mapRowsValue(typeHypothesis["core_control_expectations"]); len(controls) > 0 {
		out["core_control_expectations"] = pluginLearningCompactCoreControlExpectations(controls, 18)
	}
	if warnings := stringListValue(typeHypothesis["warnings"]); len(warnings) > 0 {
		out["warnings"] = pluginLearningCompactStringList(warnings, 8, 180)
	}
	return out
}

func pluginLearningCompactCoreControlExpectations(rows []map[string]any, limit int) []map[string]any {
	if limit >= 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := map[string]any{}
		for _, key := range []string{"control", "priority", "confidence"} {
			if value := firstPresentAny(row, key); value != nil {
				next[key] = value
			}
		}
		if why := firstNonEmptyText(row, "why_it_matters", "summary", "reason"); why != "" {
			next["why_it_matters"] = pluginLearningPromptClip(why, 220)
		}
		if clues := stringListValue(row["expected_display_clues"]); len(clues) > 0 {
			next["expected_display_clues"] = pluginLearningCompactStringList(clues, 6, 120)
		}
		if len(next) > 0 {
			out = append(out, next)
		}
	}
	return out
}

func pluginLearningCompactWebReference(webReference map[string]any) map[string]any {
	out := map[string]any{
		"schema":   firstNonEmpty(firstNonEmptyText(webReference, "schema"), pluginLearningWebReferenceSchema),
		"status":   firstNonEmptyText(webReference, "status"),
		"decision": firstNonEmptyText(webReference, "decision"),
		"query":    pluginLearningPromptClip(firstNonEmptyText(webReference, "query"), 220),
		"policy":   "Compact public-doc evidence for draft generation; source URLs are retained for provenance.",
	}
	if source := firstNonEmptyText(webReference, "search_source"); source != "" {
		out["search_source"] = source
	}
	if rows := mapRowsValue(webReference["search_results"]); len(rows) > 0 {
		out["search_results"] = pluginLearningCompactRows(rows, 5, 260, "title", "url", "snippet", "web_reference_score")
	}
	if sources := mapRowsValue(webReference["sources"]); len(sources) > 0 {
		out["sources"] = pluginLearningCompactWebSources(sources, 3)
	}
	if rows := mapRowsValue(webReference["type_hints"]); len(rows) > 0 {
		out["type_hints"] = pluginLearningCompactRows(rows, 5, 180, "type", "summary", "source_url", "confidence")
	}
	if rows := mapRowsValue(webReference["documented_core_controls"]); len(rows) > 0 {
		out["documented_core_controls"] = pluginLearningCompactRows(rows, 18, 180, "control", "description", "display_clues", "priority", "source_url", "confidence")
	}
	if rows := mapRowsValue(webReference["documented_units_and_ranges"]); len(rows) > 0 {
		out["documented_units_and_ranges"] = pluginLearningCompactRows(rows, 18, 160, "control", "unit", "range_or_values", "source_url", "confidence")
	}
	if rows := mapRowsValue(webReference["ui_sections"]); len(rows) > 0 {
		out["ui_sections"] = pluginLearningCompactRows(rows, 12, 160, "label", "description", "source_url", "confidence")
	}
	if warnings := stringListValue(webReference["warnings"]); len(warnings) > 0 {
		out["warnings"] = pluginLearningCompactStringList(warnings, 8, 180)
	}
	return out
}

func pluginLearningCompactWebSources(rows []map[string]any, limit int) []map[string]any {
	if limit >= 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := pluginLearningCompactRow(row, 260, "title", "url", "final_url", "content_type", "status_code", "snippet", "source_rank", "source_type", "confidence")
		if summary := firstNonEmptyText(row, "summary"); summary != "" {
			next["summary"] = pluginLearningPromptClip(summary, 160)
		}
		if len(next) > 0 {
			out = append(out, next)
		}
	}
	return out
}

func pluginLearningCompactVisualTargetFocus(targetFocus map[string]any) map[string]any {
	out := map[string]any{
		"schema":       firstNonEmpty(firstNonEmptyText(targetFocus, "schema"), "plugin_ui_reference_visual_target_focus.v1"),
		"target_count": targetFocus["target_count"],
		"focus_policy": firstNonEmptyText(targetFocus, "focus_policy"),
	}
	if prior := mapValue(targetFocus["plugin_type_prior"]); len(prior) > 0 {
		out["plugin_type_prior"] = pluginLearningCompactRow(prior, 120, "primary_type", "confidence")
	}
	if rows := mapRowsValue(targetFocus["targets"]); len(rows) > 0 {
		out["targets"] = pluginLearningCompactRows(rows, 14, 120,
			"target_id", "control", "source", "priority", "why_it_matters", "expected_display_clues", "candidate_param_ids")
	}
	if quick := mapRowsValue(targetFocus["quick_controls"]); len(quick) > 0 {
		out["quick_controls"] = pluginLearningCompactRows(quick, 8, 100, "param_id", "label", "display_group", "normalized_role")
	}
	return out
}

func pluginLearningCompactUIReferenceDigest(visualDigest map[string]any) map[string]any {
	out := map[string]any{
		"schema":          firstNonEmpty(firstNonEmptyText(visualDigest, "schema"), "plugin_ui_reference_digest.v1"),
		"matching_status": firstNonEmptyText(visualDigest, "matching_status"),
		"confidence":      visualDigest["confidence"],
	}
	if mode := firstNonEmptyText(visualDigest, "visual_matching_mode"); mode != "" {
		out["visual_matching_mode"] = mode
	}
	if value := firstPresentAny(visualDigest, "matching_confidence"); value != nil {
		out["matching_confidence"] = value
	}
	if targetFocus := mapValue(visualDigest["visual_target_focus"]); len(targetFocus) > 0 {
		out["visual_target_focus"] = pluginLearningCompactVisualTargetFocus(targetFocus)
	}
	if value := firstPresentAny(visualDigest, "visual_target_count"); value != nil {
		out["visual_target_count"] = value
	}
	if audit := mapValue(visualDigest["parameter_coverage_audit"]); len(audit) > 0 {
		out["parameter_coverage_audit"] = pluginLearningCompactCoverageAudit(audit)
	}
	if layout := mapValue(visualDigest["layout"]); len(layout) > 0 {
		out["layout"] = pluginLearningCompactRow(layout, 260, "summary", "notable_regions")
	}
	if rows := mapRowsValue(visualDigest["target_results"]); len(rows) > 0 {
		out["target_results"] = pluginLearningCompactRows(rows, 18, 120,
			"target_id", "control", "priority", "status", "visible_label", "visible_group", "value_text",
			"visible_unit", "display_domain", "tick_labels", "scale_hint", "display_style", "ui_region", "state",
			"backend_candidate_param_ids", "likely_backend_param_id", "mapping_reason", "confidence")
	}
	if rows := mapRowsValue(visualDigest["candidate_parameter_matches"]); len(rows) > 0 {
		out["candidate_parameter_matches"] = pluginLearningCompactRows(rows, 24, 120,
			"visible_label", "visible_group", "visible_unit", "display_domain", "ui_salience", "semantic_class", "control_kind",
			"backend_param_id", "backend_param_name", "match_reason", "evidence", "confidence")
	}
	if rows := mapRowsValue(visualDigest["visible_controls"]); len(rows) > 0 {
		out["visible_controls"] = pluginLearningCompactRows(rows, 16, 100,
			"label", "value_text", "display_value", "control_type", "control_kind", "semantic_class", "ui_salience",
			"group", "region", "state", "unit", "display_unit", "visible_range", "tick_labels", "scale_hint",
			"is_parameter_like", "is_template_or_navigation_like", "confidence")
	}
	if rows := mapRowsValue(visualDigest["extra_visible_core_controls"]); len(rows) > 0 {
		out["extra_visible_core_controls"] = pluginLearningCompactRows(rows, 6, 120, "label", "group", "unit", "reason")
	}
	if rows := mapRowsValue(visualDigest["units_and_display_domains"]); len(rows) > 0 {
		out["units_and_display_domains"] = pluginLearningCompactRows(rows, 12, 100, "label", "unit", "range_or_values", "group", "confidence")
	}
	if rows := mapRowsValue(visualDigest["possible_focus_hits"]); len(rows) > 0 {
		out["possible_focus_hits"] = pluginLearningCompactRows(rows, 20, 100, "visible_label", "visible_group", "visible_unit", "focus_term", "candidate_param_ids", "reason", "confidence")
	}
	if rows := mapRowsValue(visualDigest["ambiguous_matches"]); len(rows) > 0 {
		out["ambiguous_matches"] = pluginLearningCompactRows(rows, 16, 100, "visible_label", "candidate_param_ids", "reason")
	}
	if rows := mapRowsValue(visualDigest["conflicts"]); len(rows) > 0 {
		out["conflicts"] = pluginLearningCompactRows(rows, 16, 100, "param_id", "issue")
	}
	if rows := mapRowsValue(visualDigest["unmatched_visible_controls"]); len(rows) > 0 {
		out["unmatched_visible_controls"] = pluginLearningCompactRows(rows, 16, 100, "label", "reason")
	}
	if rows := mapRowsValue(visualDigest["unmatched_backend_parameters"]); len(rows) > 0 {
		out["unmatched_backend_parameters"] = pluginLearningCompactRows(rows, 40, 100, "param_id", "reason")
	}
	if warnings := stringListValue(visualDigest["warnings"]); len(warnings) > 0 {
		out["warnings"] = pluginLearningCompactStringList(warnings, 8, 180)
	}
	return out
}

func pluginLearningCompactCoverageAudit(audit map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{
		"schema", "backend_param_total", "backend_parameter_rows", "parameters_retained", "dropped_param_count",
		"vision_observed_control_count", "vision_observed_group_count", "vision_possible_focus_hit_count",
		"matched_param_count", "unmatched_param_count", "matching_status",
	} {
		if value := firstPresentAny(audit, key); value != nil {
			out[key] = value
		}
	}
	return out
}

func pluginLearningCompactRows(rows []map[string]any, limit, textLimit int, keys ...string) []map[string]any {
	if limit >= 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := pluginLearningCompactRow(row, textLimit, keys...)
		if len(next) > 0 {
			out = append(out, next)
		}
	}
	return out
}

func pluginLearningCompactRow(row map[string]any, textLimit int, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		value := firstPresentAny(row, key)
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			out[key] = pluginLearningPromptClip(typed, textLimit)
		case []string:
			out[key] = pluginLearningCompactStringList(typed, 8, textLimit)
		case []any:
			out[key] = pluginLearningCompactAnyList(typed, textLimit)
		case []map[string]any:
			out[key] = pluginLearningCompactRows(typed, 8, textLimit, "kind", "summary", "label", "param_id", "id", "reason")
		default:
			out[key] = value
		}
	}
	return out
}

func pluginLearningCompactAnyList(values []any, textLimit int) []any {
	if len(values) > 8 {
		values = values[:8]
	}
	out := make([]any, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			out = append(out, pluginLearningPromptClip(text, textLimit))
			continue
		}
		out = append(out, value)
	}
	return out
}

func pluginLearningCompactStringList(values []string, limit, textLimit int) []string {
	if limit >= 0 && len(values) > limit {
		values = values[:limit]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		text := pluginLearningPromptClip(value, textLimit)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func pluginLearningPromptClip(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || len(text) <= limit {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

func validatePluginProfilePatch(patch pluginProfilePatch, digest pluginParameterDigest) (pluginProfilePatch, error) {
	return plugingrabber.ValidateProfilePatch(patch, digest)
}

func buildPluginProfileUpsertCommand(target pluginLearningTarget, patch pluginProfilePatch, digest pluginParameterDigest) (map[string]any, pluginSkillValidationResult, error) {
	return plugingrabber.BuildPluginSkillUpsertCommand(target, patch, digest)
}

func enrichPluginProfilePatchDisplayDomains(patch pluginProfilePatch, digest pluginParameterDigest) pluginProfilePatch {
	byID := map[string]pluginParameterInfo{}
	for _, param := range digest.Parameters {
		if id := strings.TrimSpace(param.ID); id != "" {
			byID[id] = param
		}
	}
	enrichParams := func(params map[string]any) {
		for rawSlot, rawMapping := range params {
			slot := strings.TrimSpace(fmt.Sprint(rawSlot))
			if slot == "" {
				continue
			}
			mapping := mapValue(rawMapping)
			paramID := firstNonEmptyText(mapping, "param_id", "id")
			if paramID == "" {
				paramID = strings.TrimSpace(fmt.Sprint(rawMapping))
			}
			param, ok := byID[paramID]
			if !ok || strings.TrimSpace(param.ID) == "" {
				continue
			}
			existingText := pluginGrabberMappingDomainText(mapping)
			existingDomain := mapValue(mapping["display_domain"])
			if existingText != "" || len(existingDomain) > 0 {
				domain := plugingrabber.InferDisplayDomainForSlot(slot, param)
				if !pluginShouldPreferInferredDisplayDomain(mapping, existingText, existingDomain, domain) {
					continue
				}
				text := strings.TrimSpace(plugingrabber.DisplayDomainText(domain))
				if text == "" {
					continue
				}
				if len(mapping) == 0 {
					mapping = map[string]any{
						"param_id": paramID,
						"label":    pluginGrabberParamLabel(param, slot),
					}
				}
				if existingText != "" && existingText != text {
					mapping["display_domain_previous_text"] = existingText
					mapping["display_domain_structured_replacement"] = true
				}
				mapping["display_domain"] = mapFromJSONStruct(domain)
				mapping["display_domain_text"] = text
				mapping["evidence"] = append(mapRowsValue(mapping["evidence"]), map[string]any{
					"kind":    "plugin_parameter_display_probe",
					"summary": "Structured display domain was preferred over non-range visual wording.",
				})
				mapping["provenance"] = append(mapRowsValue(mapping["provenance"]), map[string]any{
					"kind":    "display_probe_inferred",
					"source":  "display_probe_inferred",
					"summary": text,
					"data": map[string]any{
						"previous_display_domain_text": existingText,
					},
				})
				params[rawSlot] = mapping
				continue
			}
			domain := plugingrabber.InferDisplayDomainForSlot(slot, param)
			if domain == nil {
				continue
			}
			text := strings.TrimSpace(plugingrabber.DisplayDomainText(domain))
			if text == "" {
				continue
			}
			if len(mapping) == 0 {
				mapping = map[string]any{
					"param_id": paramID,
					"label":    pluginGrabberParamLabel(param, slot),
				}
			}
			mapping["display_domain"] = mapFromJSONStruct(domain)
			mapping["display_domain_text"] = text
			mapping["evidence"] = append(mapRowsValue(mapping["evidence"]), map[string]any{
				"kind":    "plugin_parameter_display_probe",
				"summary": "Display domain inferred from plugin-exposed display information.",
			})
			mapping["provenance"] = append(mapRowsValue(mapping["provenance"]), map[string]any{
				"kind":    "display_probe_inferred",
				"source":  "display_probe_inferred",
				"summary": text,
			})
			params[rawSlot] = mapping
		}
	}
	for _, group := range patch.Groups {
		if params := mapValue(group["params"]); len(params) > 0 {
			enrichParams(params)
			group["params"] = params
		}
	}
	for _, control := range patch.VirtualControls {
		if params := mapValue(control["params"]); len(params) > 0 {
			enrichParams(params)
			control["params"] = params
		}
	}
	return patch
}

func pluginShouldPreferInferredDisplayDomain(mapping map[string]any, currentText string, currentDomain map[string]any, inferred *plugingrabber.PluginDisplayDomain) bool {
	if inferred == nil {
		return false
	}
	inferredText := strings.TrimSpace(plugingrabber.DisplayDomainText(inferred))
	if inferredText == "" || !pluginDisplayDomainTextHasRange(inferredText) {
		return false
	}
	source := strings.ToLower(firstNonEmptyText(currentDomain, "source"))
	if strings.Contains(source, "teach_mode") || strings.Contains(source, "manual") {
		return false
	}
	if pluginDisplayDomainTextHasRange(currentText) {
		return false
	}
	if _, ok := currentDomain["min"]; ok {
		if _, ok := currentDomain["max"]; ok {
			return false
		}
	}
	currentUnit := pluginNormalizeVisualUnit(pluginDisplayDomainUnitFromTextAndMap(currentText, currentDomain))
	inferredUnit := pluginNormalizeVisualUnit(inferred.Unit)
	if currentUnit != "" && inferredUnit != "" && currentUnit != inferredUnit {
		return false
	}
	status := strings.ToLower(firstNonEmptyText(currentDomain, "status"))
	if status == "confirmed" && !strings.Contains(source, "auto_learn") {
		return false
	}
	return true
}

type pluginVisualDisplayObservation struct {
	Control       string
	Label         string
	Group         string
	Unit          string
	DomainText    string
	ValueText     string
	DisplayStyle  string
	ScaleHint     string
	Status        string
	Confidence    float64
	Terms         []string
	IdentityTerms []string
}

func reconcilePluginProfilePatchVisualDisplayDomains(patch pluginProfilePatch, digest pluginParameterDigest, uiReference map[string]any) pluginProfilePatch {
	visualDigest := mapValue(uiReference["visual_digest"])
	if len(visualDigest) == 0 {
		return patch
	}
	observations := pluginVisualDisplayObservations(visualDigest)
	if len(observations) == 0 {
		return patch
	}
	byID := map[string]pluginParameterInfo{}
	for _, param := range digest.Parameters {
		if id := strings.TrimSpace(param.ID); id != "" {
			byID[id] = param
		}
	}
	reconcileParams := func(groupLabel string, params map[string]any) {
		for rawSlot, rawMapping := range params {
			slot := strings.TrimSpace(fmt.Sprint(rawSlot))
			if slot == "" {
				continue
			}
			mapping := mapValue(rawMapping)
			paramID := firstNonEmptyText(mapping, "param_id", "id")
			if paramID == "" {
				paramID = strings.TrimSpace(fmt.Sprint(rawMapping))
			}
			param, ok := byID[paramID]
			if !ok || strings.TrimSpace(param.ID) == "" {
				continue
			}
			observation, ok := bestPluginVisualDisplayObservation(observations, slot, mapping, param, groupLabel)
			if !ok {
				continue
			}
			text, domainMap := pluginVisualDisplayDomainForMapping(observation, slot, mapping, param)
			if text == "" || len(domainMap) == 0 || !pluginShouldApplyVisualDisplayDomain(mapping, observation, text, domainMap) {
				continue
			}
			if len(mapping) == 0 {
				mapping = map[string]any{
					"param_id": paramID,
					"label":    pluginGrabberParamLabel(param, slot),
				}
			}
			previous := pluginGrabberMappingDomainText(mapping)
			mapping["display_domain_text"] = text
			mapping["display_domain"] = domainMap
			mapping["visual_display_domain_text"] = text
			mapping["visual_display_domain_source"] = "plugin_ui_reference_target_probe"
			if previous != "" && previous != text {
				mapping["display_domain_conflict_resolved"] = true
				mapping["display_domain_previous_text"] = previous
			}
			mapping["evidence"] = append(mapRowsValue(mapping["evidence"]), map[string]any{
				"kind":    "plugin_ui_reference_display_domain",
				"summary": "Visual UI evidence was preferred for the human-facing display domain.",
			})
			mapping["provenance"] = append(mapRowsValue(mapping["provenance"]), map[string]any{
				"kind":    "plugin_ui_reference_display_domain",
				"source":  "plugin_ui_reference_target_probe",
				"summary": text,
				"data": map[string]any{
					"visible_label":  firstNonEmpty(observation.Label, observation.Control),
					"visible_group":  observation.Group,
					"visible_unit":   observation.Unit,
					"display_style":  observation.DisplayStyle,
					"previous_value": previous,
				},
			})
			params[rawSlot] = mapping
		}
	}
	for _, group := range patch.Groups {
		params := mapValue(group["params"])
		if len(params) == 0 {
			continue
		}
		reconcileParams(firstNonEmptyText(group, "label", "name", "id", "role"), params)
		group["params"] = params
	}
	for _, control := range patch.VirtualControls {
		params := mapValue(control["params"])
		if len(params) == 0 {
			continue
		}
		reconcileParams(firstNonEmptyText(control, "label", "name", "operation", "component_id"), params)
		control["params"] = params
	}
	return patch
}

func pluginVisualDisplayObservations(visualDigest map[string]any) []pluginVisualDisplayObservation {
	out := []pluginVisualDisplayObservation{}
	add := func(row map[string]any) {
		status := strings.ToLower(strings.TrimSpace(firstNonEmptyText(row, "status")))
		if status == "not_visible" || status == "hidden" || status == "absent" {
			return
		}
		obs := pluginVisualDisplayObservation{
			Control:      firstNonEmptyText(row, "control", "semantic_class", "name"),
			Label:        firstNonEmptyText(row, "visible_label", "label", "ui_label", "text"),
			Group:        firstNonEmptyText(row, "visible_group", "group", "ui_group", "section"),
			Unit:         pluginNormalizeVisualUnit(firstNonEmptyText(row, "visible_unit", "unit")),
			DomainText:   firstNonEmptyText(row, "display_domain", "display_domain_text", "range_or_values", "range", "values"),
			ValueText:    firstNonEmptyText(row, "value_text", "value", "current_text"),
			DisplayStyle: strings.ToLower(strings.TrimSpace(firstNonEmptyText(row, "display_style", "control_type", "style"))),
			ScaleHint:    strings.ToLower(strings.TrimSpace(firstNonEmptyText(row, "scale_hint", "scale"))),
			Status:       status,
			Confidence:   floatNumber(row["confidence"]),
		}
		if obs.Confidence <= 0 {
			obs.Confidence = 0.65
		}
		if obs.Unit == "" {
			obs.Unit = pluginNormalizeVisualUnit(pluginDisplayDomainUnitFromText(obs.DomainText + " " + obs.ValueText + " " + obs.Label))
		}
		if obs.Control == "" && obs.Label == "" && obs.DomainText == "" && obs.ValueText == "" {
			return
		}
		obs.IdentityTerms = pluginUIReferenceTerms(obs.Control, obs.Label)
		if len(obs.IdentityTerms) == 0 {
			obs.IdentityTerms = pluginUIReferenceTerms(obs.Control, obs.Label, obs.ValueText)
		}
		obs.Terms = pluginUIReferenceTerms(obs.Control, obs.Label, obs.Group, obs.DomainText, obs.ValueText, obs.Unit)
		out = append(out, obs)
	}
	for _, row := range mapRowsValue(visualDigest["target_results"]) {
		add(row)
	}
	for _, row := range mapRowsValue(visualDigest["candidate_parameter_matches"]) {
		add(row)
	}
	for _, row := range mapRowsValue(visualDigest["units_and_display_domains"]) {
		add(row)
	}
	return out
}

func bestPluginVisualDisplayObservation(observations []pluginVisualDisplayObservation, slot string, mapping map[string]any, param pluginParameterInfo, groupLabel string) (pluginVisualDisplayObservation, bool) {
	contextTerms := pluginUIReferenceTerms(slot, firstNonEmptyText(mapping, "label", "name"), param.ID, param.Name, param.RawName, param.Alias, param.NormalizedRole, param.DisplayGroup, groupLabel)
	identityTerms := pluginVisualDisplayIdentityTerms(slot, mapping, param)
	if len(contextTerms) == 0 {
		return pluginVisualDisplayObservation{}, false
	}
	best := pluginVisualDisplayObservation{}
	bestScore := 0.0
	for _, obs := range observations {
		if obs.Confidence < 0.60 || len(obs.Terms) == 0 {
			continue
		}
		if !pluginVisualDisplayObservationIdentityCompatible(identityTerms, obs.IdentityTerms) {
			continue
		}
		if !pluginVisualDisplayObservationUnitCompatible(mapping, param, obs) {
			continue
		}
		if !pluginVisualDisplayObservationRoleCompatible(contextTerms, obs.Terms) {
			continue
		}
		score := pluginUIReferenceHintScore(identityTerms, pluginUIReferenceVisualHint{Terms: obs.IdentityTerms, Confidence: obs.Confidence})
		if score == 0 {
			score = pluginUIReferenceHintScore(contextTerms, pluginUIReferenceVisualHint{Terms: obs.Terms, Confidence: obs.Confidence}) * 0.75
		}
		if pluginVisualDisplayObservationHasCriticalMatch(identityTerms, obs.IdentityTerms) && score < 0.78 {
			score = 0.78
		} else if pluginVisualDisplayObservationHasCriticalMatch(contextTerms, obs.Terms) && score < 0.72 {
			score = 0.72
		}
		if score > bestScore {
			best = obs
			bestScore = score
		}
	}
	if bestScore < 0.45 {
		return pluginVisualDisplayObservation{}, false
	}
	return best, true
}

func pluginVisualDisplayIdentityTerms(slot string, mapping map[string]any, param pluginParameterInfo) []string {
	return pluginUIReferenceTerms(
		slot,
		firstNonEmptyText(mapping, "label", "name"),
		param.Alias,
		param.Name,
		param.RawName,
		param.NormalizedRole,
		param.ID,
	)
}

func pluginVisualDisplayObservationIdentityCompatible(identityTerms, observationIdentityTerms []string) bool {
	identityCritical := pluginVisualDisplayCriticalSet(identityTerms)
	if len(identityCritical) == 0 {
		return true
	}
	observationCritical := pluginVisualDisplayCriticalSet(observationIdentityTerms)
	if len(observationCritical) == 0 {
		return false
	}
	if pluginVisualDisplayHasDiscriminatingConflict(identityCritical, observationCritical) {
		return false
	}
	for term := range identityCritical {
		if observationCritical[term] {
			return true
		}
		switch term {
		case "freq":
			if observationCritical["frequency"] {
				return true
			}
		case "frequency":
			if observationCritical["freq"] {
				return true
			}
		case "time":
			if observationCritical["delay"] {
				return true
			}
		case "delay":
			if observationCritical["time"] {
				return true
			}
		}
	}
	return false
}

func pluginVisualDisplayCriticalSet(terms []string) map[string]bool {
	critical := map[string]bool{
		"sync": true, "note": true, "mode": true, "type": true, "algorithm": true, "preset": true,
		"warp": true, "density": true, "feedback": true, "mix": true, "width": true,
		"dry": true, "wet": true, "rate": true, "depth": true,
		"low": true, "high": true, "cut": true, "time": true, "delay": true,
		"frequency": true, "freq": true, "gain": true, "threshold": true,
		"ratio": true, "attack": true, "release": true,
	}
	out := map[string]bool{}
	for _, term := range terms {
		if critical[term] {
			out[term] = true
		}
	}
	return out
}

func pluginVisualDisplayHasDiscriminatingConflict(identityCritical, observationCritical map[string]bool) bool {
	for _, pair := range [][2]string{
		{"warp", "delay"},
		{"density", "feedback"},
		{"dry", "wet"},
		{"rate", "depth"},
		{"low", "high"},
		{"attack", "release"},
	} {
		a, b := pair[0], pair[1]
		if identityCritical[a] && observationCritical[b] && !observationCritical[a] {
			return true
		}
		if identityCritical[b] && observationCritical[a] && !observationCritical[b] {
			return true
		}
	}
	return false
}

func pluginVisualDisplayObservationUnitCompatible(mapping map[string]any, param pluginParameterInfo, obs pluginVisualDisplayObservation) bool {
	visualUnit := pluginNormalizeVisualUnit(firstNonEmpty(obs.Unit, pluginDisplayDomainUnitFromText(obs.DomainText+" "+obs.ValueText)))
	if visualUnit == "" || visualUnit == "unknown" {
		return true
	}
	paramUnit := pluginNormalizeVisualUnit(firstNonEmpty(
		pluginDisplayDomainUnitFromTextAndMap("", mapFromJSONStruct(param.DisplayDomainCandidate)),
		param.Unit,
		pluginDisplayDomainUnitFromText(param.ValueText),
	))
	if paramUnit == "" || paramUnit == "unknown" || paramUnit == visualUnit {
		return true
	}
	if (paramUnit == "enum" || paramUnit == "toggle") && (visualUnit == "enum" || visualUnit == "toggle") {
		return true
	}
	return false
}

func pluginVisualDisplayObservationRoleCompatible(contextTerms, observationTerms []string) bool {
	context := map[string]bool{}
	for _, term := range contextTerms {
		context[term] = true
	}
	critical := map[string]bool{
		"sync": true, "note": true, "mode": true, "type": true, "algorithm": true,
		"warp": true, "density": true, "feedback": true, "mix": true, "width": true,
		"rate": true, "depth": true, "low": true, "high": true, "cut": true,
		"time": true, "frequency": true, "freq": true, "gain": true, "threshold": true,
		"ratio": true, "attack": true, "release": true,
	}
	seenCritical := false
	for _, term := range observationTerms {
		if !critical[term] {
			continue
		}
		seenCritical = true
		if context[term] {
			return true
		}
	}
	return !seenCritical
}

func pluginVisualDisplayObservationHasCriticalMatch(contextTerms, observationTerms []string) bool {
	context := map[string]bool{}
	for _, term := range contextTerms {
		context[term] = true
	}
	for _, term := range observationTerms {
		switch term {
		case "sync", "note", "mode", "type", "algorithm", "warp", "density", "feedback", "mix", "width", "rate", "depth", "time", "frequency", "freq", "gain":
			if context[term] {
				return true
			}
		}
	}
	return false
}

func pluginVisualDisplayDomainForMapping(obs pluginVisualDisplayObservation, slot string, mapping map[string]any, param pluginParameterInfo) (string, map[string]any) {
	context := strings.ToLower(strings.Join([]string{
		slot,
		firstNonEmptyText(mapping, "label", "name"),
		param.ID,
		param.Name,
		param.RawName,
		param.Alias,
		param.NormalizedRole,
		param.ValueText,
	}, " "))
	unit := pluginNormalizeVisualUnit(obs.Unit)
	displayText := strings.TrimSpace(firstNonEmpty(obs.DomainText, obs.ValueText))
	if pluginVisualObservationIsStepped(obs, context) {
		text := firstNonEmpty(obs.ValueText, obs.Label, obs.DomainText, "stepped values")
		return text, map[string]any{
			"text":       text,
			"unit":       "enum",
			"scale":      "enum",
			"status":     "inferred",
			"source":     "plugin_ui_reference_image",
			"confidence": pluginUIReferenceDisplayDomainConfidence(obs.Confidence),
		}
	}
	if unit != "" && unit != "unknown" {
		if unit == "%" && !pluginDisplayDomainTextHasRange(displayText) && pluginVisualContextLooksPercentRange(context) {
			displayText = "0~100 %"
		} else if displayText == "" || !strings.Contains(strings.ToLower(displayText), strings.ToLower(unit)) {
			displayText = strings.TrimSpace(firstNonEmpty(displayText, obs.ValueText, unit) + " " + unit)
		}
	}
	if displayText == "" {
		return "", nil
	}
	domain := plugingrabber.ParseDisplayDomainText(displayText, "plugin_ui_reference_image", false)
	domainMap := mapFromJSONStruct(domain)
	if len(domainMap) == 0 {
		domainMap = map[string]any{
			"text":   displayText,
			"source": "plugin_ui_reference_image",
			"status": "inferred",
		}
	}
	if unit != "" && unit != "unknown" {
		domainMap["unit"] = unit
	}
	if obs.ScaleHint != "" && obs.ScaleHint != "unknown" {
		domainMap["scale"] = obs.ScaleHint
	}
	domainMap["source"] = "plugin_ui_reference_image"
	domainMap["status"] = "inferred"
	domainMap["confidence"] = pluginUIReferenceDisplayDomainConfidence(obs.Confidence)
	return displayText, domainMap
}

func pluginShouldApplyVisualDisplayDomain(mapping map[string]any, obs pluginVisualDisplayObservation, visualText string, visualDomain map[string]any) bool {
	if obs.Confidence < 0.60 {
		return false
	}
	if source := strings.ToLower(firstNonEmptyText(mapValue(mapping["display_domain"]), "source")); strings.Contains(source, "teach_mode") || strings.Contains(source, "manual") {
		return false
	}
	currentText := pluginGrabberMappingDomainText(mapping)
	currentUnit := pluginNormalizeVisualUnit(pluginDisplayDomainUnitFromTextAndMap(currentText, mapValue(mapping["display_domain"])))
	visualUnit := pluginNormalizeVisualUnit(firstNonEmptyText(visualDomain, "unit"))
	if pluginVisualObservationIsStepped(obs, strings.ToLower(strings.Join([]string{currentText, visualText}, " "))) {
		return currentUnit != "enum" && currentUnit != "toggle"
	}
	if currentText == "" {
		return true
	}
	if visualUnit != "" && visualUnit != "unknown" && currentUnit != "" && currentUnit != "unknown" && visualUnit != currentUnit {
		return true
	}
	currentSource := strings.ToLower(firstNonEmptyText(mapValue(mapping["display_domain"]), "source"))
	if strings.Contains(currentSource, "auto_learn_name_pattern") || strings.Contains(currentSource, "auto_learn_parameter_shape") {
		return visualUnit != "" && visualUnit != "unknown" && obs.Confidence >= 0.80
	}
	return false
}

func pluginVisualObservationIsStepped(obs pluginVisualDisplayObservation, context string) bool {
	style := strings.ToLower(strings.TrimSpace(obs.DisplayStyle + " " + obs.ScaleHint + " " + obs.Unit + " " + obs.DomainText + " " + obs.ValueText + " " + context))
	if strings.Contains(style, "menu") || strings.Contains(style, "stepped") || strings.Contains(style, "enum") || strings.Contains(style, "toggle") {
		return true
	}
	for _, token := range []string{" sync", " note", " mode", " type", " algorithm", " preset"} {
		if strings.Contains(" "+style, token) {
			return true
		}
	}
	return strings.Contains(style, "/") && !strings.Contains(style, "db/")
}

func pluginVisualContextLooksPercentRange(context string) bool {
	for _, token := range []string{"mix", "wet", "dry", "feedback", "density", "warp", "width", "depth", "amount"} {
		if strings.Contains(context, token) {
			return true
		}
	}
	return false
}

func pluginDisplayDomainTextHasRange(text string) bool {
	text = strings.TrimSpace(text)
	return strings.Contains(text, "~") || strings.Contains(strings.ToLower(text), " to ") || strings.Contains(text, "..")
}

func pluginDisplayDomainUnitFromTextAndMap(text string, domain map[string]any) string {
	if unit := firstNonEmptyText(domain, "unit"); unit != "" {
		return unit
	}
	return pluginDisplayDomainUnitFromText(text)
}

func pluginDisplayDomainUnitFromText(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case strings.Contains(lower, "db"):
		return "dB"
	case strings.Contains(lower, "khz") || strings.Contains(lower, "hz"):
		return "Hz"
	case strings.Contains(lower, "%"):
		return "%"
	case strings.Contains(lower, "ms") || strings.Contains(lower, "msec"):
		return "ms"
	case strings.Contains(lower, "sec") || strings.Contains(lower, "second"):
		return "s"
	case strings.Contains(lower, "toggle") || strings.Contains(lower, "on/off"):
		return "toggle"
	case strings.Contains(lower, "enum") || strings.Contains(lower, "stepped"):
		return "enum"
	}
	return ""
}

func pluginNormalizeVisualUnit(unit string) string {
	unit = strings.TrimSpace(unit)
	switch strings.ToLower(unit) {
	case "", "unknown", "none":
		return ""
	case "db":
		return "dB"
	case "hz", "khz":
		return "Hz"
	case "percent", "percentage", "%%":
		return "%"
	case "msec", "millisecond", "milliseconds":
		return "ms"
	case "sec", "second", "seconds":
		return "s"
	case "enum", "stepped", "menu", "note":
		return "enum"
	case "bool", "boolean", "switch":
		return "toggle"
	}
	return unit
}

func pluginGrabberParamLabel(param pluginParameterInfo, fallback string) string {
	for _, text := range []string{param.Name, param.RawName, param.Alias, param.ID, fallback} {
		if clean := strings.TrimSpace(text); clean != "" {
			return clean
		}
	}
	return fallback
}

func buildPluginLearningExperiments(digest pluginParameterDigest, patch pluginProfilePatch, limit int) []map[string]any {
	if limit <= 0 {
		return nil
	}
	byID := map[string]pluginParameterInfo{}
	for _, param := range digest.Parameters {
		id := strings.TrimSpace(param.ID)
		if id != "" {
			byID[id] = param
		}
	}
	seen := map[string]bool{}
	out := make([]map[string]any, 0, limit)
	addExperiment := func(control map[string]any, slot string, mapping map[string]any, param pluginParameterInfo, reason string) bool {
		if len(out) >= limit {
			return false
		}
		paramID := strings.TrimSpace(param.ID)
		if paramID == "" || seen[paramID] {
			return false
		}
		before := floatNumber(param.NormalizedValue)
		after := before + 0.18
		if boolValue(param.IsBoolean) || strings.Contains(strings.ToLower(param.NormalizedRole), "bypass") {
			if before >= 0.5 {
				after = 0
			} else {
				after = 1
			}
		} else if before > 0.70 {
			after = before - 0.18
		}
		if after < 0 {
			after = 0
		}
		if after > 1 {
			after = 1
		}
		if after == before {
			return false
		}
		seen[paramID] = true
		label := firstNonEmptyText(mapping, "label", "name")
		if label == "" {
			label = firstNonEmpty(param.Name, paramID)
		}
		controlName := strings.TrimSpace(firstNonEmptyText(control, "name", "operation", "control", "label", "role", "id"))
		row := map[string]any{
			"id":                "experiment_" + fmt.Sprint(len(out)+1),
			"component_id":      strings.TrimSpace(firstNonEmptyText(control, "component_id", "component", "group_id", "id")),
			"operation_name":    controlName,
			"slot":              slot,
			"param_id":          paramID,
			"label":             label,
			"before_normalized": before,
			"after_normalized":  after,
			"value_text":        param.ValueText,
			"normalized_role":   param.NormalizedRole,
			"sample_reason":     reason,
		}
		if domain := plugingrabber.InferDisplayDomainForSlot(slot, param); domain != nil {
			if text := plugingrabber.DisplayDomainText(domain); strings.TrimSpace(text) != "" {
				row["inferred_display_domain_text"] = strings.TrimSpace(text)
				row["inferred_display_domain"] = mapFromJSONStruct(domain)
			}
		}
		out = append(out, row)
		return true
	}
	scanParams := func(container map[string]any, params map[string]any, reason string, shouldSample func(string, string, map[string]any, pluginParameterInfo) bool) bool {
		controlName := strings.TrimSpace(firstNonEmptyText(container, "name", "operation", "control", "label", "role", "id"))
		slots := make([]string, 0, len(params))
		for rawSlot := range params {
			slots = append(slots, strings.TrimSpace(fmt.Sprint(rawSlot)))
		}
		sort.Strings(slots)
		for _, slot := range slots {
			if len(out) >= limit {
				return false
			}
			if slot == "" {
				continue
			}
			rawMapping := params[slot]
			mapping := mapValue(rawMapping)
			paramID := strings.TrimSpace(firstNonEmptyText(mapping, "param_id", "id"))
			if paramID == "" {
				paramID = strings.TrimSpace(fmt.Sprint(rawMapping))
			}
			if paramID == "" || seen[paramID] {
				continue
			}
			param, ok := byID[paramID]
			if !ok || !param.HostControllable {
				continue
			}
			if !shouldSample(slot, controlName, mapping, param) {
				continue
			}
			addExperiment(container, slot, mapping, param, reason)
		}
		return true
	}
	needsActiveExperiment := func(slot, controlName string, mapping map[string]any, param pluginParameterInfo) bool {
		return pluginGrabberNeedsActiveExperiment(slot, controlName, mapping, param)
	}
	needsSpotCheck := func(slot, controlName string, mapping map[string]any, param pluginParameterInfo) bool {
		return !boolValue(mapping["confirmed"]) && pluginGrabberHighImpactMapping(slot, controlName, param)
	}
	for _, control := range patch.VirtualControls {
		if !scanParams(control, mapValue(control["params"]), "low_confidence_high_impact", needsActiveExperiment) {
			return out
		}
	}
	for _, group := range patch.Groups {
		if !scanParams(group, mapValue(group["params"]), "low_confidence_high_impact", needsActiveExperiment) {
			return out
		}
	}
	for _, control := range patch.VirtualControls {
		if !scanParams(control, mapValue(control["params"]), "spot_check_high_confidence", needsSpotCheck) {
			return out
		}
	}
	for _, group := range patch.Groups {
		if !scanParams(group, mapValue(group["params"]), "spot_check_high_confidence", needsSpotCheck) {
			return out
		}
	}
	return out
}

func pluginGrabberNeedsActiveExperiment(slot, controlName string, mapping map[string]any, param pluginParameterInfo) bool {
	if !pluginGrabberHighImpactMapping(slot, controlName, param) {
		return false
	}
	domain := mapValue(mapping["display_domain"])
	if len(domain) == 0 {
		if param.DisplayDomainCandidate != nil && param.DisplayDomainCandidate.Confidence >= 0.80 {
			return false
		}
		return true
	}
	if boolValue(mapping["confirmed"]) {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(firstNonEmptyText(domain, "status")))
	confidence := floatNumber(domain["confidence"])
	if status == "unknown" || status == "needs_confirmation" {
		return true
	}
	if confidence <= 0 {
		return true
	}
	return confidence < 0.80
}

func pluginGrabberHighImpactMapping(slot, controlName string, param pluginParameterInfo) bool {
	text := strings.ToLower(strings.Join([]string{
		slot,
		controlName,
		param.ID,
		param.Name,
		param.RawName,
		param.Alias,
		param.NormalizedRole,
		param.DisplayGroup,
	}, " "))
	for _, token := range []string{
		"freq", "frequency", "gain", "level", "q", "threshold", "ratio", "attack", "release",
		"delay", "time", "mix", "wet", "dry", "feedback", "rate", "depth", "width", "cut", "bypass",
	} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func parsePluginProfilePatch(raw string) (pluginProfilePatch, error) {
	return plugingrabber.ParseProfilePatch(raw)
}

func reviewedProfilePatchFromArgs(args map[string]any) (pluginProfilePatch, error) {
	raw, ok := args["reviewed_profile_patch"]
	if !ok || raw == nil {
		return pluginProfilePatch{}, fmt.Errorf("auto learn finalization requires reviewed_profile_patch")
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return pluginProfilePatch{}, err
	}
	var patch pluginProfilePatch
	if err := json.Unmarshal(payload, &patch); err != nil {
		return pluginProfilePatch{}, err
	}
	return patch, nil
}

func applyReviewedDisplayDomains(patch *pluginProfilePatch, value any) {
	if patch == nil {
		return
	}
	for _, review := range mapRowsValue(value) {
		componentID := strings.TrimSpace(firstNonEmptyText(review, "component_id", "component", "group_id"))
		operationName := strings.TrimSpace(firstNonEmptyText(review, "operation_name", "operation", "control", "name"))
		slot := strings.TrimSpace(firstNonEmptyText(review, "slot", "param_slot", "key"))
		text := strings.TrimSpace(firstNonEmptyText(review, "display_domain_text", "display_domain", "range", "unit"))
		displayLabel := strings.TrimSpace(firstNonEmptyText(review, "display_label", "label", "name"))
		if slot == "" || text == "" {
			continue
		}
		domain := plugingrabber.ParseDisplayDomainText(text, "auto_learn_user_review", true)
		if domain == nil {
			continue
		}
		domainMap := mapFromJSONStruct(domain)
		if len(domainMap) == 0 {
			continue
		}
		provenanceKind := firstNonEmptyText(review, "provenance_kind", "source")
		if provenanceKind == "" {
			provenanceKind = "user_review"
		}
		observation := firstNonEmptyText(review, "observation")
		for _, group := range patch.Groups {
			if componentID != "" && componentID != strings.TrimSpace(firstNonEmptyText(group, "id", "component_id")) {
				continue
			}
			applyReviewedDisplayDomainToParams(group["params"], slot, text, displayLabel, domainMap, provenanceKind, observation)
		}
		for _, control := range patch.VirtualControls {
			if componentID != "" && componentID != strings.TrimSpace(firstNonEmptyText(control, "component_id", "component", "group_id")) {
				continue
			}
			if operationName != "" && operationName != strings.TrimSpace(firstNonEmptyText(control, "name", "operation", "control")) {
				continue
			}
			applyReviewedDisplayDomainToParams(control["params"], slot, text, displayLabel, domainMap, provenanceKind, observation)
		}
	}
}

func applyReviewedDisplayDomainToParams(paramsAny any, slot, text, displayLabel string, domainMap map[string]any, provenanceKind, observation string) {
	params, ok := paramsAny.(map[string]any)
	if !ok || len(params) == 0 {
		return
	}
	raw, ok := params[slot]
	if !ok {
		return
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		paramID := strings.TrimSpace(fmt.Sprint(raw))
		if paramID == "" || paramID == "<nil>" {
			return
		}
		mapping = map[string]any{"param_id": paramID}
	}
	mapping["display_domain"] = domainMap
	mapping["display_domain_text"] = text
	if strings.TrimSpace(displayLabel) != "" {
		mapping["label"] = strings.TrimSpace(displayLabel)
	}
	mapping["confirmed"] = true
	mapping["evidence"] = append(mapRowsValue(mapping["evidence"]), map[string]any{
		"kind":    provenanceKind,
		"summary": firstNonEmpty(observation, "Display domain confirmed by user review."),
	})
	mapping["provenance"] = append(mapRowsValue(mapping["provenance"]), map[string]any{
		"kind":    provenanceKind,
		"source":  provenanceKind,
		"summary": firstNonEmpty(observation, text),
		"data": map[string]any{
			"display_domain_text": text,
		},
	})
	params[slot] = mapping
}

func mapFromJSONStruct(value any) map[string]any {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil
	}
	return out
}

func workflowCommandArgs(cmd map[string]any) map[string]any {
	out := map[string]any{}
	if args, ok := cmd["args"].(map[string]any); ok {
		for k, v := range args {
			out[k] = v
		}
	}
	for k, v := range cmd {
		if k == "args" {
			continue
		}
		out[k] = v
	}
	return out
}

type chatPluginRef = daw.PluginRef

func chatVisiblePluginRefs(state map[string]any, trackScope string) []chatPluginRef {
	return daw.VisiblePluginRefs(state, trackScope)
}
func kernelReplyOK(reply map[string]any) bool {
	status := strings.ToLower(firstNonEmptyText(reply, "status"))
	return status == "ok" || status == "success"
}

func (s *Server) observePluginParametersReply(reply map[string]any) {
	if s == nil || s.harness == nil {
		return
	}
	s.harness.ObservePluginParametersReply(reply)
}

func boolValue(value any) bool {
	switch x := value.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes" || s == "on"
	case int:
		return x != 0
	case float64:
		return x != 0
	default:
		return false
	}
}

func intNumber(value any) int {
	switch x := value.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	default:
		return 0
	}
}

func floatNumber(value any) float64 {
	switch x := value.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		n, _ := x.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n
	default:
		return 0
	}
}

func firstNonEmptyText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func firstStringLimit(values []string, limit int) []string {
	if limit < 0 {
		limit = 0
	}
	if limit > len(values) {
		limit = len(values)
	}
	out := make([]string, limit)
	copy(out, values[:limit])
	return out
}
