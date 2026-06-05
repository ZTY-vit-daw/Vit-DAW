package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/policy"
	"vit-daw-agent/internal/tools"

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
)

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
	out := harness.InvokeResponse{
		Status:               "ok",
		Tool:                 pluginGrabberLearnTool,
		CommandName:          pluginGrabberLearnCommand,
		RiskLevel:            tools.RiskDirect,
		RequiresConfirmation: resp.NeedsConfirmation,
		Preview:              resp.Preview,
		Result: map[string]any{
			"reply":                resp.Reply,
			"needs_confirmation":   resp.NeedsConfirmation,
			"plan_id":              resp.PlanID,
			"preview":              resp.Preview,
			"workflow":             resp.Workflow,
			"workflow_data":        resp.WorkflowData,
			"plugin_learning":      resp.PluginLearning,
			"interaction_requests": resp.InteractionRequests,
		},
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
	target, err := s.resolvePluginLoadTarget(ctx, workflowCmd, requestContext, userText)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}
	if s.kernel == nil {
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
		},
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
	if semanticCandidates, err := semanticIndexPluginLoadCandidates(query, intent); err == nil && len(semanticCandidates) > 0 {
		return semanticCandidates, nil
	}
	reply, _, err := s.kernel.SendCommand(ctx, map[string]any{
		"cmd":   "plugin_search",
		"query": query,
		"limit": 8,
	})
	if err != nil {
		return nil, err
	}
	if !kernelReplyOK(reply) {
		message := firstNonEmptyText(reply, "message", "error")
		if message == "" {
			message = "plugin_search failed"
		}
		return nil, fmt.Errorf("%s", message)
	}
	out := pluginLoadCandidatesFromRows(mapRowsValue(reply["plugins"]))
	if ranked := rankPluginLoadCandidates(query, intent, out); len(ranked) > 0 {
		return ranked, nil
	}
	if len(out) == 0 {
		if fallback, err := s.semanticPluginLoadFallbackCandidates(ctx, query, intent); err == nil && len(fallback) > 0 {
			return fallback, nil
		}
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
	reply, _, err := s.kernel.SendCommand(ctx, map[string]any{
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

func (s *Server) runPluginGrabberLearningWorkflow(ctx context.Context, conversationID, userText string, requestContext map[string]any, cfg config.EngineConfig, workflowCmd map[string]any) ChatResponse {
	target, err := s.resolvePluginLearningTarget(ctx, workflowCmd, requestContext, userText)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}
	if s.kernel == nil {
		err := fmt.Errorf("kernel client is nil")
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
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
	args := workflowCommandArgs(workflowCmd)
	mode := strings.TrimSpace(strings.ToLower(firstNonEmptyText(args, "mode", "learning_mode")))
	if mode == "" {
		mode = string(plugingrabber.LearningModeAutoLearn)
	}
	reviewedAutoLearn := mode == "auto_learn_reviewed" || args["reviewed_profile_patch"] != nil
	if reviewedAutoLearn {
		mode = string(plugingrabber.LearningModeAutoLearn)
	}
	if extra := strings.TrimSpace(firstNonEmptyText(args, "extra_instructions", "supplement", "notes")); extra != "" {
		userText = strings.TrimSpace(userText + "\nAdditional user notes: " + extra)
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
		} else if deterministicPatch, summary, ok := plugingrabber.BuildAutoLearnProfilePatch(digest); ok {
			patch = deterministicPatch
			workflowData = summary
		} else {
			digestJSON, err := pluginLearningPromptDigestJSON(digest)
			if err != nil {
				return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
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
				"strategy":              "llm_profile_patch",
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
	patch, err = validatePluginProfilePatch(patch, digest)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}
	patch = enrichPluginProfilePatchDisplayDomains(patch, digest)

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
	workflowData["experiments"] = buildPluginLearningExperiments(digest, patch, 3)
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

	reply := strings.TrimSpace(patch.Reply)
	if reply == "" {
		reply = "已准备项目 Plugin Grabber 档案更新。"
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
	raw, err := s.llm.Complete(ctx, cfg, []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	})
	if err != nil {
		return pluginProfilePatch{}, err
	}
	return parsePluginProfilePatch(raw)
}

func buildPluginParameterDigest(reply map[string]any) pluginParameterDigest {
	return plugingrabber.BuildParameterDigest(reply)
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
			if pluginGrabberMappingDomainText(mapping) != "" || len(mapValue(mapping["display_domain"])) > 0 {
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
