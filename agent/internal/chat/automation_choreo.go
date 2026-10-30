package chat

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"vit-daw-agent/internal/actionworkflow"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
)

// Section choreography chain (M3-C4-AUTO-INTENT-1): natural-language
// time-varying mix phrase -> deterministic intent routing (vocabulary
// families in automation_intent.go) -> region resolution (three channels, no
// guessing) -> shape generator -> confirmation card (disclosing the chosen
// parameter/shape/values, editable) -> governed execution through the
// D1-S1 domain-table section_automation row -> set_points write (the only
// forward mutation) -> get_curve per-point readback settle -> terminal note
// (no automatic revert) -> C4 ledger (mix_decision_record.v1 + workflow
// queue node advance).
//
// Governance shape: the write is issued by the domain-table materializer face
// (executionports.AutomationEnvelopeVSPPort under the orchestration runtime's
// authorize/execute discipline). The public invoke guard is untouched —
// automation.* stays rejected there for active free-state loops (STOP-NOTE
// red line: no guard exceptions, the table row is the governed surface).

const (
	automationChoreoWorkflow       = "section_automation_choreo"
	automationChoreoCapabilityID   = "fine_mix.section_automation.v1"
	automationChoreoMaxRegionSpanS = experiment.D1S1AutomationMaxRegionSpanS
)

// automationChoreoCandidate is the pending confirmation payload: the resolved
// plan (parameter/shape/region/points) awaiting the user's confirm/edit/cancel
// decision, mirrored after the mix-tick pending surface.
type automationChoreoCandidate struct {
	ID          string
	CreatedAt   time.Time
	Status      string
	GoalID      string
	RunID       string
	UserText    string
	TrackID     string
	TrackLabel  string
	Category    string
	PluginID    string
	PluginName  string
	ParamID     string
	Shape       automationShape
	RegionStart float64
	RegionEnd   float64
	Points      []map[string]any
	// Clarify holds the pending clarification state (category or region ask);
	// a non-empty Reason means the candidate is a question, not a proposal.
	Clarify map[string]any
}

func (c *automationChoreoCandidate) regionMap() map[string]any {
	if c.RegionEnd <= c.RegionStart {
		return map[string]any{}
	}
	return map[string]any{"start": c.RegionStart, "end": c.RegionEnd}
}

// automationChoreoHumanSummary is the one-line disclosure of the plan.
func automationChoreoHumanSummary(c automationChoreoCandidate) string {
	param := c.ParamID
	if c.PluginName != "" {
		param = c.PluginName + " · " + c.ParamID
	}
	return fmt.Sprintf("对 %s 的「%s」参数写一条 %s 包络：区间 %.1fs-%.1fs，%s（%.2f → %.2f，曲率 %.2f）",
		automationChoreoTrackLabel(c.TrackID, c.TrackLabel), param, shapeWordForCategory(c.Category),
		c.RegionStart, c.RegionEnd, c.Shape.Label, c.Shape.StartValue, c.Shape.EndValue, c.Shape.Curve)
}

func automationChoreoTrackLabel(trackID, label string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	if strings.TrimSpace(trackID) == "" {
		return "目标轨"
	}
	return "Track " + trackID
}

func shapeWordForCategory(category string) string {
	switch category {
	case automationCategorySpace:
		return "空间类"
	case automationCategoryTone:
		return "音色类"
	default:
		return "电平类"
	}
}

// automationChoreoForConversation reads the live pending candidate.
func (s *Server) automationChoreoForConversation(conversationID string) (*automationChoreoCandidate, bool) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	candidate, ok := s.pendingAutomationChoreos[conversationID]
	if !ok || candidate == nil || !strings.EqualFold(strings.TrimSpace(candidate.Status), "pending_confirmation") {
		return nil, false
	}
	stored := *candidate
	return &stored, true
}

func (s *Server) storeAutomationChoreoCandidate(conversationID string, candidate *automationChoreoCandidate) {
	if s == nil || candidate == nil {
		return
	}
	s.mu.Lock()
	if s.pendingAutomationChoreos == nil {
		s.pendingAutomationChoreos = map[string]*automationChoreoCandidate{}
	}
	stored := *candidate
	s.pendingAutomationChoreos[conversationID] = &stored
	s.mu.Unlock()
}

func (s *Server) expireAutomationChoreo(conversationID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.pendingAutomationChoreos, conversationID)
	s.mu.Unlock()
}

// ---- Region channel 2: project section markers ----

// automationMarkerRegion resolves a section word against the project's
// section markers (project.markers.list). The optional spanSeconds narrows
// the window from the marker start. No marker, no guess: the caller falls to
// the clarification channel.
func (s *Server) automationMarkerRegion(ctx context.Context, sectionName string, spanSeconds float64) (start, end float64, ok bool) {
	if s == nil || s.kernel == nil || strings.TrimSpace(sectionName) == "" {
		return 0, 0, false
	}
	markerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	reply, _, err := s.kernel.SendCommand(markerCtx, map[string]any{"cmd": "project.markers.list"})
	if err != nil {
		return 0, 0, false
	}
	if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "ok") {
		return 0, 0, false
	}
	for _, row := range mapRowsFromAny(reply["markers"]) {
		name := firstNonEmpty(firstStringFromMap(row, "name", "label", "title"), "")
		if name == "" || !automationMarkerNameMatches(name, sectionName) {
			continue
		}
		markerStart, okStart := automationMarkerSeconds(row, "start", "start_seconds", "time", "position_seconds")
		markerEnd, okEnd := automationMarkerSeconds(row, "end", "end_seconds", "length_seconds")
		if !okStart {
			continue
		}
		if spanSeconds > 0 {
			windowEnd := markerStart + spanSeconds
			if okEnd && markerEnd > markerStart && windowEnd > markerEnd {
				windowEnd = markerEnd
			}
			if windowEnd <= markerStart || windowEnd-markerStart > automationChoreoMaxRegionSpanS {
				return 0, 0, false
			}
			return markerStart, windowEnd, true
		}
		if !okEnd || markerEnd <= markerStart || markerEnd-markerStart > automationChoreoMaxRegionSpanS {
			return 0, 0, false
		}
		return markerStart, markerEnd, true
	}
	return 0, 0, false
}

func automationMarkerNameMatches(markerName, sectionName string) bool {
	marker := strings.ToLower(strings.TrimSpace(markerName))
	section := strings.ToLower(strings.TrimSpace(sectionName))
	if marker == "" || section == "" {
		return false
	}
	return strings.Contains(marker, section) || strings.Contains(section, marker)
}

func automationMarkerSeconds(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			if parsed, parseOK := automationChoreoNumber(value); parseOK {
				return parsed, true
			}
		}
	}
	return 0, false
}

func automationChoreoNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

// ---- Target resolution: track + plugin/parameter ----

// automationResolveTrack resolves the target track: the explicit selection in
// the request context ("这条轨"), else a named track in the phrase, else the
// single-track project. Ambiguity is returned as a clarification need.
func (s *Server) automationResolveTrack(ctx context.Context, message string, requestContext map[string]any) (trackID, label string, ok bool) {
	if id := firstStringFromMap(requestContext, "selected_track_id", "selected_scene_track_id", "selected_plugin_track_id"); strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id), "", true
	}
	if s == nil || s.harness == nil {
		return "", "", false
	}
	state := s.harness.UserStateSummary(ctx)
	rows := chatVisibleTrackRows(state)
	if len(rows) == 0 {
		return "", "", false
	}
	if len(rows) == 1 {
		row := rows[0]
		return firstNonEmpty(firstStringFromMap(row, "track_id"), firstStringFromMap(row, "id")), firstNonEmpty(firstStringFromMap(row, "name"), firstStringFromMap(row, "track_name")), true
	}
	// Named track in the phrase wins over ambiguity.
	lower := strings.ToLower(message)
	for _, row := range rows {
		name := firstNonEmpty(firstStringFromMap(row, "name"), firstStringFromMap(row, "track_name"), firstStringFromMap(row, "track_id"), firstStringFromMap(row, "id"))
		if strings.TrimSpace(name) != "" && strings.Contains(lower, strings.ToLower(strings.TrimSpace(name))) {
			return firstNonEmpty(firstStringFromMap(row, "track_id"), firstStringFromMap(row, "id")), name, true
		}
	}
	return "", "", false
}

// automationResolveLevelPlugin resolves the level-category write target: the
// track's built-in Volume & Pan plugin id (type/name containing "volume",
// 196/217 kernel face) with param_id "volume" (raw range [0,1]).
func (s *Server) automationResolveLevelPlugin(ctx context.Context, trackID string) (pluginID, pluginName, paramID string, ok bool) {
	if s == nil || s.kernel == nil || strings.TrimSpace(trackID) == "" {
		return "", "", "", false
	}
	stateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	reply, _, err := s.kernel.SendCommand(stateCtx, map[string]any{"cmd": "get_project_state"})
	if err != nil {
		return "", "", "", false
	}
	if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "ok") {
		return "", "", "", false
	}
	for _, trackRow := range mapRowsFromAny(reply["tracks"]) {
		rowID := firstNonEmpty(firstStringFromMap(trackRow, "track_id"), firstStringFromMap(trackRow, "id"), firstStringFromMap(trackRow, "item_id"))
		if rowID != trackID {
			continue
		}
		for _, pluginRow := range mapRowsFromAny(trackRow["plugins"]) {
			kind := strings.ToLower(firstStringFromMap(pluginRow, "name") + "/" + firstStringFromMap(pluginRow, "type"))
			if !strings.Contains(kind, "volume") {
				continue
			}
			id := firstNonEmpty(firstStringFromMap(pluginRow, "id"), firstStringFromMap(pluginRow, "plugin_item_id"), firstStringFromMap(pluginRow, "plugin_id"))
			if strings.TrimSpace(id) != "" {
				return strings.TrimSpace(id), firstNonEmpty(firstStringFromMap(pluginRow, "name"), "Volume & Pan"), "volume", true
			}
		}
		return "", "", "", false
	}
	return "", "", "", false
}

// automationTrackHasPlugins reports whether the track carries any external
// plugin rows at all (space/tone honest-boundary check input).
func (s *Server) automationTrackPluginNames(ctx context.Context, trackID string) []string {
	if s == nil || s.kernel == nil || strings.TrimSpace(trackID) == "" {
		return nil
	}
	stateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	reply, _, err := s.kernel.SendCommand(stateCtx, map[string]any{"cmd": "get_project_state"})
	if err != nil {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "ok") {
		return nil
	}
	for _, trackRow := range mapRowsFromAny(reply["tracks"]) {
		rowID := firstNonEmpty(firstStringFromMap(trackRow, "track_id"), firstStringFromMap(trackRow, "id"), firstStringFromMap(trackRow, "item_id"))
		if rowID != trackID {
			continue
		}
		names := []string{}
		for _, pluginRow := range mapRowsFromAny(trackRow["plugins"]) {
			if name := firstNonEmpty(firstStringFromMap(pluginRow, "name"), firstStringFromMap(pluginRow, "type")); strings.TrimSpace(name) != "" {
				names = append(names, strings.TrimSpace(name))
			}
		}
		return names
	}
	return nil
}

// automationCategoryPluginNames filters the track plugin rows down to the
// category-relevant effect plugins: the space class needs a reverb/delay
// style processor and the tone class an EQ/filter — the track built-ins
// (volume & pan, level meter) are not effect targets for either, so a
// track carrying only built-ins still reports the honest no-plugin
// boundary (nothing fabricated).
func automationCategoryPluginNames(names []string, category string) []string {
	var needles []string
	switch category {
	case automationCategorySpace:
		needles = []string{"reverb", "verb", "space", "delay", "echo", "supermassive", "ambience", "hall", "room"}
	case automationCategoryTone:
		needles = []string{"eq", "filter", "tone", "shelf", "boost", "exciter", "q3"}
	default:
		return nil
	}
	out := []string{}
	for _, name := range names {
		lower := strings.ToLower(name)
		for _, needle := range needles {
			if strings.Contains(lower, needle) {
				out = append(out, name)
				break
			}
		}
	}
	return out
}

// ---- Entry: deterministic pre-model takeover ----

// handleAutomationChoreoChat is the deterministic intent router: a
// time-varying phrase with a semantic-result word (or treatment verb) takes
// over before any LLM turn, mirroring the INTENT-WIRE boxed-range takeover.
// Non-armed phrases fall through untouched (static treatment keeps its path).
func (s *Server) handleAutomationChoreoChat(ctx context.Context, conversationID string, req ChatRequest, mode string) (ChatResponse, bool) {
	message := strings.TrimSpace(req.Message)
	if message == "" || strings.HasPrefix(message, "/") {
		return ChatResponse{}, false
	}
	// A live pending choreography owns the turn: confirmation, cancellation,
	// revision or re-ask routing (before any new intent parsing).
	if response, handled := s.handlePendingAutomationChoreoChat(ctx, conversationID, req, mode); handled {
		return response, true
	}
	if !automationIntentArmed(message) {
		return ChatResponse{}, false
	}
	intent := parseAutomationIntent(message)
	goalID, runID := goalIDsFromContext(req.Context)

	// Target track: explicit selection or single-track project, else clarify.
	trackID, trackLabel, trackOK := s.automationResolveTrack(ctx, message, req.Context)
	if !trackOK {
		return s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent,
			"要给哪条轨做这段自动化？请指名轨道（或先在 DAW 里选中它），再说一次完整话术。", "track"), true
	}

	// Category: known -> resolve parameters; unknown -> clarify (never guess).
	if intent.Category == "" {
		return s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent,
			"这段变化想调哪一类参数？电平类（存在感/能量/音量）、空间类（远近/湿度）还是音色类（亮闷薄）？说「电平」「空间」或「音色」，或直接重说完整话术。", "category"), true
	}

	pluginID, pluginName, paramID := "", "", ""
	switch intent.Category {
	case automationCategoryLevel:
		resolvedPlugin, resolvedName, resolvedParam, resolvedOK := s.automationResolveLevelPlugin(ctx, trackID)
		if !resolvedOK {
			return ChatResponse{
				ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
				Reply:      fmt.Sprintf("没有在 %s 上找到轨级音量插件，电平类包络当前不可写。工程状态可能未同步，可稍后重试或改用显式包络指令。", automationChoreoTrackLabel(trackID, trackLabel)),
				GoalStatus: string(agentruntime.StatusFailed), StopReason: "automation_level_plugin_unavailable",
				Workflow: automationChoreoWorkflow, WorkflowData: map[string]any{"mutation_performed": false},
			}, true
		}
		pluginID, pluginName, paramID = resolvedPlugin, resolvedName, resolvedParam
	default:
		// Space/tone honest boundary: 挂了才有，没挂如实报边界不虚写。
		names := automationCategoryPluginNames(s.automationTrackPluginNames(ctx, trackID), intent.Category)
		if len(names) == 0 {
			categoryWord := map[string]string{automationCategorySpace: "空间", automationCategoryTone: "音色"}[intent.Category]
			return ChatResponse{
				ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
				Reply: fmt.Sprintf("「%s」属于%s类变化，但 %s 上没有挂任何效果插件，没有可写的空间/音色参数——如实说明，不虚写。这条轨的电平类（音量包络）可以做；要做%s类请先在轨上挂对应插件（如混响/EQ）。",
					intent.CategoryWord, categoryWord, automationChoreoTrackLabel(trackID, trackLabel), categoryWord),
				GoalStatus: string(agentruntime.StatusCompleted), StopReason: "automation_category_boundary_no_plugin",
				Workflow: automationChoreoWorkflow,
				WorkflowData: map[string]any{
					"mutation_performed": false, "category": intent.Category, "boundary": "no_plugin_on_track",
					"available_fallback": automationCategoryLevel,
				},
			}, true
		}
		return s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent,
			fmt.Sprintf("%s 上挂着这些插件：%s。%s类包络需要指名具体插件和参数（例如挂了混响可以写 wet 参数）；请指名参数，或改用电平类（音量）话术。",
				automationChoreoTrackLabel(trackID, trackLabel), strings.Join(names, "、"), shapeWordForCategory(intent.Category)), "param"), true
	}

	// Region: three channels, no guessing.
	region := parseAutomationRegionIntent(message)
	choreoTarget := automationChoreoTarget{TrackID: trackID, TrackLabel: trackLabel, PluginID: pluginID, PluginName: pluginName, ParamID: paramID}
	switch region.Source {
	case automationRegionSourceAbsolute:
		// window resolved inline
	case automationRegionSourceMarker:
		start, end, markerOK := s.automationMarkerRegion(ctx, region.SectionName, region.SectionSeconds)
		if !markerOK {
			// Face 1: a span the user already said ("副歌那 30 秒") is offered
			// back as a confirmable candidate instead of being dropped.
			if region.SectionSeconds > 0 {
				return s.automationChoreoClarifySpanCandidateResponse(conversationID, mode, goalID, runID, message, intent, choreoTarget, region.SectionName, region.SectionSeconds), true
			}
			return s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent,
				fmt.Sprintf("话术里的「%s」在工程段落标记里没有找到对应区间（不猜测段落位置）。请直接说绝对区间，例如「0 到 30 秒」，或先在工程里打上段落标记。", region.SectionName), "region"), true
		}
		region.Start, region.End, region.Source = start, end, automationRegionSourceMarker
	default:
		// Face 3: a bare "N 秒" carries a confirmable candidate ("是从 0 开始
		// 的 N 秒吗？"), never a silent default.
		if region.BareSpanSeconds > 0 {
			return s.automationChoreoClarifySpanCandidateResponse(conversationID, mode, goalID, runID, message, intent, choreoTarget, "", region.BareSpanSeconds), true
		}
		return s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent,
			"这段变化从哪一秒到哪一秒？请给出区间（例如「0 到 30 秒」），或指名工程里已有的段落标记（如「副歌」）。", "region"), true
	}
	if region.End-region.Start > automationChoreoMaxRegionSpanS {
		return ChatResponse{
			ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
			Reply:      fmt.Sprintf("区间 %.0fs-%.0fs 超出单条包络 %.0f 秒上限，请缩小区间或拆成多段。", region.Start, region.End, automationChoreoMaxRegionSpanS),
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "automation_region_span_overflow",
			Workflow: automationChoreoWorkflow, WorkflowData: map[string]any{"mutation_performed": false},
		}, true
	}

	// Shape: explicit utterance values win over the default library.
	shape := automationDefaultShape(intent.DirectionUp)
	if startValue, endValue, ok := parseAutomationExplicitValues(message); ok {
		shape = automationShape{Name: "explicit", Label: "按话术数值", StartValue: startValue, EndValue: endValue, Curve: 0}
	}

	candidate := &automationChoreoCandidate{
		ID:          "automation_choreo_" + randomID(),
		CreatedAt:   time.Now().UTC(),
		Status:      "pending_confirmation",
		GoalID:      goalID,
		RunID:       runID,
		UserText:    message,
		TrackID:     trackID,
		TrackLabel:  trackLabel,
		Category:    intent.Category,
		PluginID:    pluginID,
		PluginName:  pluginName,
		ParamID:     paramID,
		Shape:       shape,
		RegionStart: region.Start,
		RegionEnd:   region.End,
		Points:      automationBuildPoints(shape, region.Start, region.End),
	}
	if fullAccessChoreoConfirmationContext(req.Context) {
		return s.automationChoreoFullAccessExecution(ctx, conversationID, mode, *candidate), true
	}
	s.storeAutomationChoreoCandidate(conversationID, candidate)
	return s.automationChoreoConfirmationResponse(conversationID, mode, *candidate), true
}

// automationChoreoClarifyResponse stores the clarification context and answers
// with the ask (never a mutation, never a guess).
func (s *Server) automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message string, intent automationIntent, question, kind string) ChatResponse {
	candidate := &automationChoreoCandidate{
		ID:        "automation_choreo_" + randomID(),
		CreatedAt: time.Now().UTC(),
		Status:    "pending_confirmation",
		GoalID:    goalID,
		RunID:     runID,
		UserText:  message,
		Category:  intent.Category,
		Clarify:   map[string]any{"kind": kind, "question": question, "intent_time_varying": true},
	}
	s.storeAutomationChoreoCandidate(conversationID, candidate)
	return s.automationChoreoClarifyChatResponse(conversationID, mode, goalID, runID, intent, question, kind)
}

// automationChoreoClarifyChatResponse renders the clarify card body shared by
// the plain ask and the span-candidate ask.
func (s *Server) automationChoreoClarifyChatResponse(conversationID, mode, goalID, runID string, intent automationIntent, question, kind string) ChatResponse {
	resp := ChatResponse{
		ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
		Reply:      question,
		GoalStatus: string(agentruntime.StatusWaitingContinue),
		StopReason: "automation_choreo_clarify_" + kind,
		Workflow:   automationChoreoWorkflow,
		WorkflowData: map[string]any{
			"mutation_performed": false, "clarify_kind": kind, "category_hint": intent.Category,
			"time_varying_intent": intent.TimeVarying,
		},
		InteractionRequests: []AgentInteractionRequest{{
			ID: "interaction_" + randomID(), Kind: "automation_choreo_clarify", Type: "automation_choreo_clarify",
			Workflow: automationChoreoWorkflow, Stage: "clarify", ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Title: "段落自动化：需要补充信息", Body: question, Status: "waiting",
			Questions: []AgentInteractionField{{ID: "clarify_" + kind, Label: "补充说明", Kind: "text", Required: false, Placeholder: "例如：0 到 30 秒 / 电平"}},
			Actions:   []AgentInteractionAction{{ID: "cancel", Label: "取消", Style: "secondary"}},
		}},
	}
	if s.logger != nil {
		s.logger.Info("[automation.choreo] clarify conversation=%s kind=%s", conversationID, kind)
	}
	return resp
}

// automationChoreoTarget carries the resolved write target across the clarify
// hand-off (track + level plugin): a span-candidate adoption rebuilds the
// proposal from the stored context without re-resolving against the kernel.
type automationChoreoTarget struct {
	TrackID, TrackLabel         string
	PluginID, PluginName, ParamID string
}

// automationChoreoClarifySpanCandidateResponse offers a confirmable fallback
// region instead of dropping the span the user already said
// (M3-C4-AUTO-CLARIFY-UX-1 faces 1+3): "副歌那 30 秒" with the project marker
// missing, or a bare "30 秒" with no window at all. The candidate rides the
// full resolved target context, so an affirmative answer adopts region
// 0..span without re-running the marker channel that just failed here.
// 不猜原则不破：系统只提供候选；采纳必须来自用户的显式肯定回复，永不默认。
func (s *Server) automationChoreoClarifySpanCandidateResponse(conversationID, mode, goalID, runID, message string, intent automationIntent, target automationChoreoTarget, sectionName string, spanSeconds float64) ChatResponse {
	question := fmt.Sprintf("是从 0 开始的 %.0f 秒吗？回「可以」采纳候选区间；或直接给完整区间（例如「0 到 30 秒」）。", spanSeconds)
	if strings.TrimSpace(sectionName) != "" {
		question = fmt.Sprintf("「%s」在工程段落标记里没有找到对应区间（不猜测段落位置）。用开头 %.0f 秒可以吗？回「可以」采纳候选区间；或直接给绝对区间（例如「0 到 30 秒」）。", sectionName, spanSeconds)
	}
	candidate := &automationChoreoCandidate{
		ID:         "automation_choreo_" + randomID(),
		CreatedAt:  time.Now().UTC(),
		Status:     "pending_confirmation",
		GoalID:     goalID,
		RunID:      runID,
		UserText:   message,
		TrackID:    target.TrackID,
		TrackLabel: target.TrackLabel,
		Category:   intent.Category,
		PluginID:   target.PluginID,
		PluginName: target.PluginName,
		ParamID:    target.ParamID,
		Clarify: map[string]any{"kind": "region_span_candidate", "question": question, "intent_time_varying": true,
			"span_seconds": spanSeconds, "section_name": sectionName},
	}
	s.storeAutomationChoreoCandidate(conversationID, candidate)
	return s.automationChoreoClarifyChatResponse(conversationID, mode, goalID, runID, intent, question, "region_span_candidate")
}

// automationChoreoAffirmAnswer reports whether the message is a bare
// affirmative/confirmation word with no content of its own
// (M3-C4-AUTO-CLARIFY-UX-1 face 2: 「确认写入」「写入」「可以吗」... while a
// clarification is pending). Deliberately tight — a length cap rejects whole
// sentences so a real region/category answer always reaches the merge path.
func automationChoreoAffirmAnswer(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" || len([]rune(text)) > 8 {
		return false
	}
	switch text {
	case "可以", "可以吧", "可以吗", "好的", "好", "好吧", "行", "行吧", "同意", "采纳",
		"确认", "确认写入", "写入", "写", "就这个", "用这个", "就这样", "嗯", "嗯嗯",
		"ok", "okay", "yes":
		return true
	}
	return textHasAny(text, "确认写入", "开始写入")
}

// automationChoreoCandidateAffirm is the explicit-acceptance family that may
// adopt an offered span candidate (face 1). The candidate question tells the
// user to answer 「可以」 verbatim, so that exact phrase is a mechanical accept
// (CONFIRM-CHAT-1 lesson: the ask's own verbs are the accept table).
// Interrogative 「可以吗」 is a question, not an acceptance — it never adopts.
func automationChoreoCandidateAffirm(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" || len([]rune(text)) > 8 {
		return false
	}
	switch text {
	case "可以", "可以吧", "可以呀", "好的", "好", "好吧", "行", "行吧", "同意", "采纳",
		"确认", "确认采纳", "确认写入", "写入", "写", "开始写入", "就这个", "用这个",
		"就按这个", "就这样", "嗯", "嗯嗯", "ok", "okay", "yes":
		return true
	}
	return textHasAny(text, "确认写入", "开始写入", "采纳这个", "就按这个")
}

// automationChoreoClarifyAffirmGuidance answers an affirmative word received
// while a clarification is still pending: no plan exists yet, so a
// confirmation word has nothing to confirm — say what is actually awaited
// instead of replaying the ask verbatim (2026-10-29 hand-test: 「确认写入」
// replayed the region question word for word and felt like dead input). The
// pending clarification stays alive; the user can still answer it properly.
func (s *Server) automationChoreoClarifyAffirmGuidance(conversationID, mode string, candidate automationChoreoCandidate) ChatResponse {
	kind := firstStringFromMap(candidate.Clarify, "kind")
	guidance := ""
	switch kind {
	case "region":
		guidance = "还在等时间区间，尚未生成写入方案——例如「0 到 30 秒」，或指名工程段落（如「副歌」）。确认类回复要等方案卡出来后才生效。"
	case "category":
		guidance = "还在等参数类别，尚未生成写入方案——回「电平」「空间」或「音色」。确认类回复要等方案卡出来后才生效。"
	case "param":
		guidance = "还在等具体插件与参数，尚未生成写入方案——请指名参数（例如挂了混响可以写 wet）。确认类回复要等方案卡出来后才生效。"
	default:
		guidance = "还在等目标轨道，尚未生成写入方案——请指名轨道（或先在 DAW 里选中它）。确认类回复要等方案卡出来后才生效。"
	}
	goalID, runID := candidate.GoalID, candidate.RunID
	if s.logger != nil {
		s.logger.Info("[automation.choreo] clarify affirm guidance conversation=%s kind=%s", conversationID, kind)
	}
	return ChatResponse{
		ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
		Reply:      guidance,
		GoalStatus: string(agentruntime.StatusWaitingContinue),
		StopReason: "automation_choreo_clarify_affirm_guidance",
		Workflow:   automationChoreoWorkflow,
		WorkflowData: map[string]any{
			"mutation_performed": false, "clarify_kind": kind, "affirm_guidance": true,
			"time_varying_intent": true,
		},
	}
}

// automationChoreoAdoptSpanCandidate turns an accepted span candidate into
// the confirmation proposal: region 0..span over the stored target context
// (M3-C4-AUTO-CLARIFY-UX-1 face 1). The user's affirmative is the explicit
// acceptance the 不猜 principle requires — nothing is adopted by default, and
// the proposal still waits on the confirmation card before any write.
func (s *Server) automationChoreoAdoptSpanCandidate(ctx context.Context, conversationID string, req ChatRequest, mode string, clarified automationChoreoCandidate, span float64) (ChatResponse, bool) {
	goalID, runID := clarified.GoalID, clarified.RunID
	if strings.TrimSpace(goalID) == "" || strings.TrimSpace(runID) == "" {
		goalID, runID = goalIDsFromContext(req.Context)
	}
	s.expireAutomationChoreo(conversationID)
	if span > automationChoreoMaxRegionSpanS {
		return ChatResponse{
			ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
			Reply:      fmt.Sprintf("候选区间 %.0f 秒超出单条包络 %.0f 秒上限，没有采纳；请给一个更短的绝对区间（例如「0 到 30 秒」）。", span, automationChoreoMaxRegionSpanS),
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "automation_region_span_overflow",
			Workflow: automationChoreoWorkflow, WorkflowData: map[string]any{"mutation_performed": false},
		}, true
	}
	intent := parseAutomationIntent(clarified.UserText)
	shape := automationDefaultShape(intent.DirectionUp)
	if startValue, endValue, ok := parseAutomationExplicitValues(clarified.UserText); ok {
		shape = automationShape{Name: "explicit", Label: "按话术数值", StartValue: startValue, EndValue: endValue, Curve: 0}
	}
	candidate := &automationChoreoCandidate{
		ID: "automation_choreo_" + randomID(), CreatedAt: time.Now().UTC(), Status: "pending_confirmation",
		GoalID: goalID, RunID: runID, UserText: clarified.UserText,
		TrackID: clarified.TrackID, TrackLabel: clarified.TrackLabel,
		Category: clarified.Category, PluginID: clarified.PluginID, PluginName: clarified.PluginName, ParamID: clarified.ParamID,
		Shape: shape, RegionStart: 0, RegionEnd: span,
		Points: automationBuildPoints(shape, 0, span),
	}
	if fullAccessChoreoConfirmationContext(req.Context) {
		return s.automationChoreoFullAccessExecution(ctx, conversationID, mode, *candidate), true
	}
	s.storeAutomationChoreoCandidate(conversationID, candidate)
	return s.automationChoreoConfirmationResponse(conversationID, mode, *candidate), true
}

// automationChoreoConfirmationResponse renders the answerable confirmation
// card: review items disclose the chosen parameter/shape/values/region and
// editable fields let the user revise before confirming.
func (s *Server) automationChoreoConfirmationResponse(conversationID, mode string, candidate automationChoreoCandidate) ChatResponse {
	paramLabel := candidate.ParamID
	if candidate.PluginName != "" {
		paramLabel = candidate.PluginName + " · " + candidate.ParamID
	}
	request := AgentInteractionRequest{
		ID: "interaction_" + randomID(), Kind: "automation_choreo_confirmation", Type: "automation_choreo_confirmation",
		Workflow: automationChoreoWorkflow, Stage: "pending_confirmation", ConversationID: conversationID,
		GoalID: candidate.GoalID, RunID: candidate.RunID, Title: "段落包络写入方案（确认后执行）", Status: "waiting",
		Body: automationChoreoHumanSummary(candidate) + "。确认前不会修改工程；形状、幅度和区间都可以在下面改。",
		ReviewItems: []AgentInteractionReview{
			{ID: "target", Title: "目标参数", Body: fmt.Sprintf("%s：%s（%s类）", automationChoreoTrackLabel(candidate.TrackID, candidate.TrackLabel), paramLabel, shapeWordForCategory(candidate.Category)), Status: "info"},
			{ID: "region", Title: "区间", Body: fmt.Sprintf("%.1fs → %.1fs", candidate.RegionStart, candidate.RegionEnd), Status: "info"},
			{ID: "shape", Title: "形状", Body: automationShapeSummary(candidate.Shape), Status: "info"},
			{ID: "points", Title: "写入点", Body: automationChoreoPointsSummary(candidate.Points), Status: "info"},
		},
		Fields: []AgentInteractionField{
			{ID: "shape", Label: "形状", Kind: "choice", Value: candidate.Shape.Name, Options: automationShapeLibrary(), Description: "渐入/渐出/保持/推进缺省形状库"},
			{ID: "start_value", Label: "起点值", Kind: "text", Value: fmt.Sprintf("%.2f", candidate.Shape.StartValue), Description: "电平类为音量 raw 值 [0,1]"},
			{ID: "end_value", Label: "终点值", Kind: "text", Value: fmt.Sprintf("%.2f", candidate.Shape.EndValue)},
			{ID: "curve", Label: "曲率 c", Kind: "text", Value: fmt.Sprintf("%.2f", candidate.Shape.Curve), Description: "[-1,1]"},
			{ID: "region_start", Label: "区间起点（秒）", Kind: "text", Value: fmt.Sprintf("%.1f", candidate.RegionStart)},
			{ID: "region_end", Label: "区间终点（秒）", Kind: "text", Value: fmt.Sprintf("%.1f", candidate.RegionEnd)},
		},
		Actions: []AgentInteractionAction{
			{ID: "approve", Label: "确认写入", Style: "primary", Recommended: true},
			{ID: "cancel", Label: "取消", Style: "secondary"},
		},
		Payload: map[string]any{
			"candidate_id": candidate.ID, "track_id": candidate.TrackID, "plugin_id": candidate.PluginID,
			"param_id": candidate.ParamID, "category": candidate.Category,
			"region_start_s": candidate.RegionStart, "region_end_s": candidate.RegionEnd,
			"shape": candidate.Shape.Name, "points": candidate.Points,
			"display": map[string]any{"title": "段落包络待确认", "summary": automationChoreoHumanSummary(candidate)},
		},
	}
	s.storePendingInteraction(request, request.Payload)
	if s.logger != nil {
		s.logger.Info("[automation.choreo] proposal card conversation=%s track=%s param=%s region=%.1f-%.1f shape=%s",
			conversationID, candidate.TrackID, candidate.ParamID, candidate.RegionStart, candidate.RegionEnd, candidate.Shape.Name)
	}
	s.emitAgentEvent(conversationID, AgentEvent{
		Type: "automation_choreo.pending", GoalID: candidate.GoalID, RunID: candidate.RunID,
		ItemType: "automation_choreo", Status: "pending_confirmation",
		Title: "段落包络方案待确认", Body: automationChoreoHumanSummary(candidate) + "，确认前不会修改工程。",
		Payload: request.Payload,
	})
	return ChatResponse{
		ConversationID: conversationID, AgentMode: mode, GoalID: candidate.GoalID, RunID: candidate.RunID,
		Reply: "我准备这样写这条包络：" + automationChoreoHumanSummary(candidate) +
			"。方案里的形状、幅度、区间都可以改；确认后我会写入并逐点读回验证，写完汇报混音进展。",
		GoalStatus: string(agentruntime.StatusWaitingConfirmation), StopReason: "automation_choreo_awaiting_confirmation",
		Workflow: automationChoreoWorkflow,
		WorkflowData: map[string]any{
			"mutation_performed": false, "candidate_id": candidate.ID, "category": candidate.Category,
			"region_start_s": candidate.RegionStart, "region_end_s": candidate.RegionEnd,
			"shape": candidate.Shape.Name, "points": candidate.Points,
			"plugin_id": candidate.PluginID, "param_id": candidate.ParamID, "track_id": candidate.TrackID,
		},
		InteractionRequests: []AgentInteractionRequest{request},
	}
}

func automationChoreoPointsSummary(points []map[string]any) string {
	parts := make([]string, 0, len(points))
	for _, point := range points {
		parts = append(parts, fmt.Sprintf("(%.1fs, %.2f)", point["t"], point["v"]))
	}
	return strings.Join(parts, " → ")
}

// ---- Pending-face confirmation routing ----

// handlePendingAutomationChoreoChat routes the answer to a live pending
// choreography: explicit/natural confirmation executes, rejection cancels,
// revision requests regenerate the card, clarification answers merge into a
// fresh resolve pass.
func (s *Server) handlePendingAutomationChoreoChat(ctx context.Context, conversationID string, req ChatRequest, mode string) (ChatResponse, bool) {
	candidate, ok := s.automationChoreoForConversation(conversationID)
	if !ok {
		return ChatResponse{}, false
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return ChatResponse{}, false
	}
	// Clarification answers: the stored intent context plus the user's new
	// region/category wording re-runs the resolve pass.
	if len(candidate.Clarify) > 0 {
		clarifyKind := firstStringFromMap(candidate.Clarify, "kind")
		// Face 2: an affirmative/confirmation word with no content of its own
		// gets the differentiated guidance (what is still awaited, no plan
		// exists yet) instead of a verbatim replay of the ask. The pending
		// clarification stays alive.
		if clarifyKind != "region_span_candidate" && automationChoreoAffirmAnswer(message) {
			return s.automationChoreoClarifyAffirmGuidance(conversationID, mode, *candidate), true
		}
		// Face 1: the offered span candidate is adopted only by the user's
		// explicit acceptance; an embedded absolute region still wins and
		// falls through to the merge path.
		if clarifyKind == "region_span_candidate" {
			if parseAutomationRegionIntent(message).Source != automationRegionSourceAbsolute && automationChoreoCandidateAffirm(message) {
				if span, ok := automationChoreoNumber(candidate.Clarify["span_seconds"]); ok && span > 0 {
					return s.automationChoreoAdoptSpanCandidate(ctx, conversationID, req, mode, *candidate, span)
				}
			}
			// A non-adopting answer to a span candidate re-enters the resolve
			// pass with the merged wording (absolute region, different section,
			// or a rejection of the candidate in words).
		}
		merged := candidate.UserText + " " + message
		if !automationIntentArmed(merged) {
			// The answer alone may carry the missing piece; keep the pending
			// clarify alive only when the merged phrase still arms. Otherwise
			// the user moved on — retire the clarify state honestly.
			s.expireAutomationChoreo(conversationID)
			return ChatResponse{}, false
		}
		s.expireAutomationChoreo(conversationID)
		req.Message = merged
		return s.handleAutomationChoreoEntry(ctx, conversationID, req, mode)
	}
	confirmation := actionworkflow.ClassifyConfirmation(message, true)
	switch {
	case confirmation.Kind == actionworkflow.DecisionRevision || messageRevisesPendingMixTreatment(message):
		revised, reviseErr := automationChoreoApplyRevision(*candidate, message)
		if reviseErr != nil {
			return ChatResponse{
				ConversationID: conversationID, AgentMode: mode, GoalID: candidate.GoalID, RunID: candidate.RunID,
				Reply:      "这轮修改没有读出可用的数值：" + reviseErr.Error(),
				GoalStatus: string(agentruntime.StatusWaitingConfirmation), StopReason: "automation_choreo_revision_unreadable",
				Workflow: automationChoreoWorkflow, WorkflowData: map[string]any{"mutation_performed": false},
			}, true
		}
		revised.Status = "pending_confirmation"
		s.storeAutomationChoreoCandidate(conversationID, &revised)
		return s.automationChoreoConfirmationResponse(conversationID, mode, revised), true
	case confirmation.Kind == actionworkflow.DecisionReject || messageNaturalMixTickCancel(message):
		s.expireAutomationChoreo(conversationID)
		s.expirePendingInteractionsForConversation(conversationID, "automation_choreo")
		return ChatResponse{
			ConversationID: conversationID, AgentMode: mode, GoalID: candidate.GoalID, RunID: candidate.RunID,
			Reply: "已取消这条段落包络方案，没有修改工程。", GoalStatus: string(agentruntime.StatusCancelled),
			StopReason: "automation_choreo_cancelled", Workflow: automationChoreoWorkflow,
			WorkflowData: map[string]any{"mutation_performed": false},
		}, true
	case confirmation.Kind == actionworkflow.DecisionAccept || messageNaturalMixTickApproval(message) || messageExplicitMixTickApply(message) || automationChoreoExplicitApply(message):
		s.expireAutomationChoreo(conversationID)
		return s.executeAutomationChoreo(ctx, conversationID, mode, *candidate, map[string]any{}), true
	default:
		// Unreadable/other phrasing while a confirmation is pending: hold the
		// card and answer truthfully (fail-closed, no silent expiry).
		return ChatResponse{
			ConversationID: conversationID, AgentMode: mode, GoalID: candidate.GoalID, RunID: candidate.RunID,
			Reply:      "我还没有写入。待确认的方案是：" + automationChoreoHumanSummary(*candidate) + "。确认请回「确认写入」或「需要」；要改请直接说改成什么（例如「改成 0.4 到 0.9」）；取消回「取消」。",
			GoalStatus: string(agentruntime.StatusWaitingConfirmation), StopReason: "automation_choreo_confirmation_held",
			Workflow: automationChoreoWorkflow, WorkflowData: map[string]any{"mutation_performed": false},
		}, true
	}
}

// automationChoreoExplicitApply is the choreography confirmation’s own
// accepted wording (CONFIRM-CHAT-1 lesson: the ask’s own verbs must be in
// the accept table — the card tells the user to answer it verbatim, so
// that exact phrase is a mechanical accept).
func automationChoreoExplicitApply(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	switch text {
	case "确认写入", "写入", "开始写入", "写", "确认写", "写吧", "写入吧", "按这个写", "就按这个写":
		return true
	}
	return textHasAny(text, "确认写入", "开始写入", "写入吧", "按这个写", "就按这个写", "write it", "confirm write")
}

// handleAutomationChoreoEntry re-runs the resolve pass after a clarification
// answer (separated so the pending-face check does not recurse).
func (s *Server) handleAutomationChoreoEntry(ctx context.Context, conversationID string, req ChatRequest, mode string) (ChatResponse, bool) {
	message := strings.TrimSpace(req.Message)
	if !automationIntentArmed(message) {
		return ChatResponse{}, false
	}
	intent := parseAutomationIntent(message)
	goalID, runID := goalIDsFromContext(req.Context)
	trackID, trackLabel, trackOK := s.automationResolveTrack(ctx, message, req.Context)
	if !trackOK {
		return s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent,
			"要给哪条轨做这段自动化？请指名轨道（或先在 DAW 里选中它），再说一次完整话术。", "track"), true
	}
	if intent.Category == "" {
		return s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent,
			"这段变化想调哪一类参数？电平类（存在感/能量/音量）、空间类（远近/湿度）还是音色类（亮闷薄）？", "category"), true
	}
	pluginID, pluginName, paramID := "", "", ""
	if intent.Category != automationCategoryLevel {
		names := automationCategoryPluginNames(s.automationTrackPluginNames(ctx, trackID), intent.Category)
		if len(names) == 0 {
			categoryWord := map[string]string{automationCategorySpace: "空间", automationCategoryTone: "音色"}[intent.Category]
			return ChatResponse{
				ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
				Reply:      fmt.Sprintf("这条轨上没有挂任何效果插件，%s类参数没有可写目标——如实说明，不虚写。电平类（音量包络）可以做。", categoryWord),
				GoalStatus: string(agentruntime.StatusCompleted), StopReason: "automation_category_boundary_no_plugin",
				Workflow: automationChoreoWorkflow, WorkflowData: map[string]any{"mutation_performed": false, "category": intent.Category},
			}, true
		}
		return s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent,
			fmt.Sprintf("轨上插件：%s。%s类包络请指名具体插件和参数；或改用电平类话术。", strings.Join(names, "、"), shapeWordForCategory(intent.Category)), "param"), true
	}
	resolvedPlugin, resolvedName, resolvedParam, resolvedOK := s.automationResolveLevelPlugin(ctx, trackID)
	if !resolvedOK {
		return ChatResponse{
			ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
			Reply:      "没有找到轨级音量插件，电平类包络当前不可写；可稍后重试或改用显式包络指令。",
			GoalStatus: string(agentruntime.StatusFailed), StopReason: "automation_level_plugin_unavailable",
			Workflow: automationChoreoWorkflow, WorkflowData: map[string]any{"mutation_performed": false},
		}, true
	}
	pluginID, pluginName, paramID = resolvedPlugin, resolvedName, resolvedParam
	region := parseAutomationRegionIntent(message)
	choreoTarget := automationChoreoTarget{TrackID: trackID, TrackLabel: trackLabel, PluginID: pluginID, PluginName: pluginName, ParamID: paramID}
	switch region.Source {
	case automationRegionSourceAbsolute:
	case automationRegionSourceMarker:
		start, end, markerOK := s.automationMarkerRegion(ctx, region.SectionName, region.SectionSeconds)
		if !markerOK {
			if region.SectionSeconds > 0 {
				return s.automationChoreoClarifySpanCandidateResponse(conversationID, mode, goalID, runID, message, intent, choreoTarget, region.SectionName, region.SectionSeconds), true
			}
			return s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent,
				fmt.Sprintf("「%s」在工程段落标记里没有找到对应区间（不猜测段落位置）。请直接说绝对区间，例如「0 到 30 秒」。", region.SectionName), "region"), true
		}
		region.Start, region.End, region.Source = start, end, automationRegionSourceMarker
	default:
		if region.BareSpanSeconds > 0 {
			return s.automationChoreoClarifySpanCandidateResponse(conversationID, mode, goalID, runID, message, intent, choreoTarget, "", region.BareSpanSeconds), true
		}
		return s.automationChoreoClarifyResponse(conversationID, mode, goalID, runID, message, intent,
			"这段变化从哪一秒到哪一秒？请给出区间（例如「0 到 30 秒」），或指名工程里已有的段落标记。", "region"), true
	}
	if region.End-region.Start > automationChoreoMaxRegionSpanS {
		return ChatResponse{
			ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
			Reply:      fmt.Sprintf("区间 %.0fs-%.0fs 超出单条包络 %.0f 秒上限，请缩小区间或拆成多段。", region.Start, region.End, automationChoreoMaxRegionSpanS),
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "automation_region_span_overflow",
			Workflow: automationChoreoWorkflow, WorkflowData: map[string]any{"mutation_performed": false},
		}, true
	}
	shape := automationDefaultShape(intent.DirectionUp)
	if startValue, endValue, ok := parseAutomationExplicitValues(message); ok {
		shape = automationShape{Name: "explicit", Label: "按话术数值", StartValue: startValue, EndValue: endValue, Curve: 0}
	}
	candidate := &automationChoreoCandidate{
		ID: "automation_choreo_" + randomID(), CreatedAt: time.Now().UTC(), Status: "pending_confirmation",
		GoalID: goalID, RunID: runID, UserText: message, TrackID: trackID, TrackLabel: trackLabel,
		Category: intent.Category, PluginID: pluginID, PluginName: pluginName, ParamID: paramID,
		Shape: shape, RegionStart: region.Start, RegionEnd: region.End,
		Points: automationBuildPoints(shape, region.Start, region.End),
	}
	if fullAccessChoreoConfirmationContext(req.Context) {
		return s.automationChoreoFullAccessExecution(ctx, conversationID, mode, *candidate), true
	}
	s.storeAutomationChoreoCandidate(conversationID, candidate)
	return s.automationChoreoConfirmationResponse(conversationID, mode, *candidate), true
}

// automationChoreoApplyRevision merges a revision phrase ("改成 0.4 到 0.9",
// "换成渐出", "改成 10 到 40 秒") into the candidate.
func automationChoreoApplyRevision(candidate automationChoreoCandidate, message string) (automationChoreoCandidate, error) {
	lower := strings.ToLower(message)
	updated := candidate
	if startValue, endValue, ok := parseAutomationExplicitValues(lower); ok {
		updated.Shape.StartValue, updated.Shape.EndValue = startValue, endValue
		updated.Shape.Name, updated.Shape.Label = "explicit", "按修改数值"
	}
	for _, name := range []string{automationShapeFadeIn, automationShapeFadeOut, automationShapeHold, automationShapePush} {
		shape, ok := automationShapeByID(name)
		if !ok {
			continue
		}
		if strings.Contains(lower, strings.ToLower(shape.Label)) {
			updated.Shape = shape
			break
		}
	}
	region := parseAutomationRegionIntent(lower)
	if region.Source == automationRegionSourceAbsolute && region.End > region.Start {
		updated.RegionStart, updated.RegionEnd = region.Start, region.End
	}
	if updated.Shape.StartValue == candidate.Shape.StartValue && updated.Shape.EndValue == candidate.Shape.EndValue &&
		updated.Shape.Name == candidate.Shape.Name && updated.RegionStart == candidate.RegionStart && updated.RegionEnd == candidate.RegionEnd {
		return candidate, fmt.Errorf("请在修改话术里带上新的数值、形状名（渐入/渐出/保持/推进）或区间")
	}
	if updated.RegionEnd-updated.RegionStart <= 0 {
		return candidate, fmt.Errorf("修改后的区间无效")
	}
	updated.Points = automationBuildPoints(updated.Shape, updated.RegionStart, updated.RegionEnd)
	return updated, nil
}

// ---- Governed execution (single-round terminal choreography) ----

type automationChoreoVerifier struct {
	Candidate automationChoreoCandidate
}

func (v automationChoreoVerifier) Verify(ctx context.Context, set orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	evidence := []string{}
	applied := false
	readbackVerified := false
	for _, receipt := range receipts {
		evidence = append(evidence, receipt.EvidenceRefs...)
		if strings.EqualFold(receipt.Status, "applied") {
			applied = true
			if detail, ok := receipt.Details["readback_verified"]; ok {
				if verified, ok := detail.(bool); ok && verified {
					readbackVerified = true
				}
			}
		}
	}
	if !applied {
		return orchestration.VerificationResult{Status: "fail", Structural: "not_applied", EvidenceRefs: evidence,
			Summary: "the automation envelope write was not applied"}, fmt.Errorf("automation envelope write was not applied")
	}
	if !readbackVerified {
		return orchestration.VerificationResult{Status: "fail", Structural: "readback_unverified", EvidenceRefs: evidence,
			Summary: "the automation envelope readback verification is missing"}, fmt.Errorf("automation envelope readback verification is missing")
	}
	return orchestration.VerificationResult{Status: "pass", Structural: "get_curve_per_point_match", EvidenceRefs: evidence,
		Summary: fmt.Sprintf("envelope written and read back per-point on %s (%d points)",
			v.Candidate.ParamID, len(v.Candidate.Points))}, nil
}

// executeAutomationChoreo is the governed single-round execution: admission
// (domain-table validation) -> frozen plan -> durable session ->
// authorize -> execute via the AutomationEnvelopeVSPPort (the only forward
// mutation) -> settle -> terminal note + C4 ledger.
func (s *Server) executeAutomationChoreo(ctx context.Context, conversationID, mode string, candidate automationChoreoCandidate, fieldEdits map[string]any) ChatResponse {
	goalID, runID := candidate.GoalID, candidate.RunID
	blocked := func(reason string) ChatResponse {
		return ChatResponse{
			ConversationID: conversationID, AgentMode: mode, GoalID: goalID, RunID: runID,
			Reply: "这条包络没有写入：" + reason, GoalStatus: string(agentruntime.StatusFailed),
			StopReason: "automation_choreo_blocked", Workflow: automationChoreoWorkflow,
			WorkflowData: map[string]any{"mutation_performed": false, "block_reason": reason},
		}
	}
	if s.kernel == nil || s.harness == nil || s.orchestrationRuntime == nil || !s.orchestrationRuntime.HasDurableStore() {
		return blocked("编排运行时或内核依赖不可用")
	}
	// Field edits from the confirmation card (user-revised values win).
	if len(fieldEdits) > 0 {
		edited, editErr := automationChoreoApplyFieldEdits(candidate, fieldEdits)
		if editErr != nil {
			return blocked("确认卡上的修改值无效：" + editErr.Error())
		}
		candidate = edited
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return blocked("工程状态快照不可用")
	}
	projectUUID := firstNonEmpty(firstStringFromMap(state.LegacyState, "project_uuid", "id"), "automation_choreo")
	points := []map[string]any{}
	for _, point := range candidate.Points {
		points = append(points, point)
	}
	region := map[string]any{}
	if candidate.RegionEnd > candidate.RegionStart {
		region = map[string]any{"start": candidate.RegionStart, "end": candidate.RegionEnd}
	}
	// Admission: the domain-table row is the bounds authority (point array,
	// region structure, id presence); the raw value range is port-side.
	admission := experiment.Admission{
		SchemaVersion: experiment.SchemaVersion,
		TargetRef:     map[string]any{"kind": "track", "id": candidate.TrackID},
		EvidenceRefs:  []string{"automation_intent:" + candidate.ID},
		Hypothesis:    "自然话术段落编舞：" + candidate.UserText,
		TypedAction: map[string]any{
			"action_domain": experiment.SectionAutomationActionDomain,
			"action_kind":   experiment.AutomationEnvelopeWriteKind,
			"plugin_id":     candidate.PluginID,
			"param_id":      candidate.ParamID,
			"points":        points,
			"region":        region,
		},
		DiagnosticDoseBounds: map[string]any{"max_action_attempts": 1, "point_count": float64(len(points))},
		RetainedDoseBounds:   map[string]any{"max_action_attempts": 1, "point_count": float64(len(points))},
		ExperimentBudget:     1,
		ExpectedEffect:       fmt.Sprintf("%s 参数在 %.1fs-%.1fs 内 %s", candidate.ParamID, candidate.RegionStart, candidate.RegionEnd, candidate.Shape.Label),
		VerificationPlan:     map[string]any{"kind": "automation_get_curve_readback", "tolerance": experiment.D1S1AutomationReadbackTol},
		CheckpointRef:        fmt.Sprint(state.Revision),
		RollbackPlan:         map[string]any{"kind": "automation_clear_region", "region": region},
		AuthorityMode:        experiment.AuthorityOrdinary,
	}
	if err := admission.ValidateD1S1(); err != nil {
		return blocked("域表准入未通过：" + err.Error())
	}
	spec, specOK := experiment.D1S1DomainSpecFor(admission)
	if !specOK {
		return blocked("域表行未解析")
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{
		State:                  d1StateResult(projectUUID, state.ProjectEpoch, state.SnapshotHash, state.Revision, state.LegacyState),
		Guarantee:              projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: admission.EvidenceRefs,
		TargetFingerprints:     []string{d1SubstituteFingerprint(spec.TargetFingerprintTemplate, candidate.TrackID, "", candidate.ParamID)},
		ContractVersions:       append([]string(nil), spec.ContractVersions...),
	})
	if err != nil {
		return blocked("工程基线锚定失败：" + err.Error())
	}
	choreoID := sanitizeCanaryID(candidate.ID)
	action := orchestration.Action{
		ID: "choreo_" + choreoID + spec.ActionIDSuffix, Command: spec.ActionKind, TargetRef: candidate.TrackID,
		BeforeFingerprint: d1SubstituteFingerprint(spec.BeforeFingerprintTemplate, candidate.TrackID, "", candidate.ParamID),
		Args:              admission.TypedAction, Compensatable: true, IdempotencyClass: "effectively_once",
	}
	set := orchestration.ActionSet{ID: "choreo_set_" + choreoID, CapabilityID: spec.CapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	set.Hash = set.ComputeHash()
	proposal := orchestration.Proposal{ID: "choreo_proposal_" + choreoID, Revision: 1, CapabilityID: spec.CapabilityID,
		CapabilityVer: "v1", ProjectCutHash: cut.Hash, ActionSetHash: set.Hash, TargetScope: []string{candidate.TrackID},
		Risk: "reversible", VerificationRef: "automation_get_curve_readback", Summary: admission.Hypothesis, CreatedAt: time.Now().UTC()}
	plan := orchestration.FrozenPlan{Proposal: proposal, ActionSet: set, ProjectCut: cut,
		ContextBundleID: "choreo_context_" + choreoID, FrozenAt: time.Now().UTC()}
	sessionID := "choreo_session_" + choreoID
	session, err := s.orchestrationRuntime.StartB2ChatSession(sessionID, conversationID, projectUUID, admission.Hypothesis, orchestration.InteractionPropose)
	if err == nil {
		session, err = s.orchestrationRuntime.AttachFrozenPlan(sessionID, plan)
	}
	if err == nil {
		decision := orchestration.ApprovalDecision{SchemaVersion: orchestration.ApprovalDecisionSchema, Kind: orchestration.ApprovalApprove,
			ProposalID: plan.Proposal.ID, ProposalRevision: 1, ActionSetHash: set.Hash, ProjectCutHash: cut.Hash,
			ApprovedScope: []string{candidate.TrackID}, SourceTurnID: candidate.ID, UserText: "用户确认卡：确认写入", Confidence: "explicit"}
		session, err = s.orchestrationRuntime.AuthorizeProposal(sessionID, orchestration.Authorization{ProposalID: plan.Proposal.ID, ProposalRevision: 1,
			ActionSetHash: set.Hash, ProjectCutHash: cut.Hash, Scope: []string{candidate.TrackID}, SourceTurnID: candidate.ID,
			Sequence: session.Revision + 1, Decision: &decision})
	}
	if err != nil {
		return blocked("编排会话授权失败：" + err.Error())
	}
	port := &d1JournalMutationPort{inner: &executionports.AutomationEnvelopeVSPPort{Client: s.kernel}, harness: s.harness, goalID: goalID, runID: runID, journalSpec: &spec}
	verifier := automationChoreoVerifier{Candidate: candidate}
	session, execErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(ctx, sessionID, set, cut, port, verifier,
		executionports.ProjectHistory{Harness: s.harness, GoalID: goalID, RunID: runID})
	if execErr != nil || session.Status != orchestration.StatusCompleted {
		reason := firstNonEmpty(execErr.Error(), string(session.Status))
		return blocked("执行或验证失败：" + reason)
	}
	// Settle: the port's per-point readback plus the verifier's pass verdict
	// are the structural settle; the reply carries the terminal note (no
	// automatic revert).
	resp := s.automationChoreoSettleResponse(conversationID, mode, candidate, session)
	s.recordLastAutomationChoreo(conversationID, candidate)
	return resp
}

// automationChoreoApplyFieldEdits merges the confirmation card's editable
// fields (shape/start/end/curve/region) into the candidate before execution.
func automationChoreoApplyFieldEdits(candidate automationChoreoCandidate, edits map[string]any) (automationChoreoCandidate, error) {
	updated := candidate
	changed := false
	if shapeName := strings.TrimSpace(firstStringFromMap(edits, "shape")); shapeName != "" && !strings.EqualFold(shapeName, candidate.Shape.Name) {
		shape, ok := automationShapeByID(shapeName)
		if !ok {
			return candidate, fmt.Errorf("形状 %q 不在缺省形状库", shapeName)
		}
		updated.Shape = shape
		changed = true
	}
	if value, err := automationChoreoParseEditFloat(edits, "start_value"); err == nil && math.Abs(value-candidate.Shape.StartValue) > 1e-9 {
		updated.Shape.StartValue = value
		updated.Shape.Name, updated.Shape.Label = "edited", "按确认卡修改"
		changed = true
	}
	if value, err := automationChoreoParseEditFloat(edits, "end_value"); err == nil && math.Abs(value-candidate.Shape.EndValue) > 1e-9 {
		updated.Shape.EndValue = value
		updated.Shape.Name, updated.Shape.Label = "edited", "按确认卡修改"
		changed = true
	}
	if value, err := automationChoreoParseEditFloat(edits, "curve"); err == nil && math.Abs(value-candidate.Shape.Curve) > 1e-9 {
		if value < -1 || value > 1 {
			return candidate, fmt.Errorf("曲率 %.2f 超出 [-1,1]", value)
		}
		updated.Shape.Curve = value
		changed = true
	}
	regionStart := candidate.RegionStart
	regionEnd := candidate.RegionEnd
	if value, err := automationChoreoParseEditFloat(edits, "region_start"); err == nil {
		regionStart = value
	}
	if value, err := automationChoreoParseEditFloat(edits, "region_end"); err == nil {
		regionEnd = value
	}
	if regionStart != candidate.RegionStart || regionEnd != candidate.RegionEnd {
		if regionStart < 0 || regionEnd <= regionStart || regionEnd-regionStart > automationChoreoMaxRegionSpanS {
			return candidate, fmt.Errorf("区间 %.1f-%.1f 无效", regionStart, regionEnd)
		}
		updated.RegionStart, updated.RegionEnd = regionStart, regionEnd
		changed = true
	}
	if !changed {
		return candidate, nil
	}
	if updated.Shape.StartValue == updated.Shape.EndValue && updated.Shape.Name != automationShapeHold {
		return candidate, fmt.Errorf("起点与终点值相同（保持形状除外）")
	}
	updated.Points = automationBuildPoints(updated.Shape, updated.RegionStart, updated.RegionEnd)
	return updated, nil
}

func automationChoreoParseEditFloat(edits map[string]any, key string) (float64, error) {
	raw := strings.TrimSpace(firstStringFromMap(edits, key))
	if raw == "" {
		return 0, fmt.Errorf("absent")
	}
	return automationChoreoParseFloatText(raw)
}

func automationChoreoParseFloatText(text string) (float64, error) {
	var value float64
	if _, err := fmt.Sscanf(strings.TrimSpace(text), "%g", &value); err != nil {
		return 0, err
	}
	return value, nil
}

// recordLastAutomationChoreo remembers the last executed choreography for the
// readback entry ("读回刚才那条线").
func (s *Server) recordLastAutomationChoreo(conversationID string, candidate automationChoreoCandidate) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastAutomationChoreos == nil {
		s.lastAutomationChoreos = map[string]automationChoreoCandidate{}
	}
	s.lastAutomationChoreos[conversationID] = candidate
}

// ---- Settle response: receipt summary + terminal note + C4 ledger ----

func (s *Server) automationChoreoSettleResponse(conversationID, mode string, candidate automationChoreoCandidate, session orchestration.PlanningSession) ChatResponse {
	resp := ChatResponse{
		ConversationID: conversationID, AgentMode: mode, GoalID: candidate.GoalID, RunID: candidate.RunID,
		Reply: strings.Join([]string{
			fmt.Sprintf("已写入：%s（%s）。", automationChoreoHumanSummary(candidate), automationChoreoPointsSummary(candidate.Points)),
			fmt.Sprintf("逐点读回验证通过（容差 %g），共 %d 点；曲线已留存，未自动回退——不满意可用撤销或让我清除这段区间。", experiment.D1S1AutomationReadbackTol, len(candidate.Points)),
			"混音进展（C4 段落自动化）：" + s.automationChoreoC4Summary(conversationID, session),
		}, "\n"),
		GoalStatus: string(agentruntime.StatusCompleted), StopReason: "automation_choreo_settled",
		Workflow: automationChoreoWorkflow,
		WorkflowData: map[string]any{
			"mutation_performed": true, "settle": "structural", "candidate_id": candidate.ID,
			"plugin_id": candidate.PluginID, "param_id": candidate.ParamID, "track_id": candidate.TrackID,
			"region": map[string]any{"start": candidate.RegionStart, "end": candidate.RegionEnd},
			"points": candidate.Points, "written": len(candidate.Points),
			"readback_tolerance": experiment.D1S1AutomationReadbackTol,
			"capability_id":      automationChoreoCapabilityID,
		},
	}
	// C4 ledger: decision record (authority references only) + queue node.
	attachMixboardDecisionProjection(&resp, session)
	if annotation, annErr := s.writeAutomationChoreoQueueNode(conversationID, candidate); annErr == nil {
		resp.WorkflowData["workflow_queue_c4"] = map[string]any{
			"node_id": annotation.NodeID, "state": annotation.State, "reason": annotation.Reason,
			"evidence_refs": annotation.EvidenceRefs,
		}
	}
	return resp
}

// automationChoreoC4Summary reports the C4 workflow state for the settle
// reply ("写完汇报一下现在的混音进展").
func (s *Server) automationChoreoC4Summary(conversationID string, session orchestration.PlanningSession) string {
	recorded := "账面记录失败"
	if status, ok := sessionStatusDecisionProjection(session); ok {
		recorded = status
	}
	return fmt.Sprintf("已记入混音决策账（%s，能力 %s），C4 节点已推进到进行中；后续自动化的进展会随队列视图更新。", recorded, automationChoreoCapabilityID)
}

func sessionStatusDecisionProjection(session orchestration.PlanningSession) (string, bool) {
	if session.Execution == nil {
		return "", false
	}
	for _, receipt := range session.Execution.Receipts {
		if strings.EqualFold(receipt.Status, "applied") {
			return "已应用并读回验证", true
		}
	}
	return "", false
}

// writeAutomationChoreoQueueNode advances the workflow queue's C4 node to
// in_progress with the choreography evidence (completed stays ledger-owned).
func (s *Server) writeAutomationChoreoQueueNode(conversationID string, candidate automationChoreoCandidate) (MixWorkflowQueueNodeAnnotation, error) {
	if s == nil {
		return MixWorkflowQueueNodeAnnotation{}, fmt.Errorf("server unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	annotation, err := s.WriteMixWorkflowQueueNodeState(ctx, "C4", "in_progress",
		fmt.Sprintf("段落自动化包络写入并读回验证（%s，%d 点）", candidate.ParamID, len(candidate.Points)),
		[]string{"automation_choreo:" + candidate.ID}, conversationID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[automation.choreo] queue node annotation failed conversation=%s err=%v", conversationID, err)
		}
		return MixWorkflowQueueNodeAnnotation{}, err
	}
	if s.logger != nil {
		s.logger.Info("[automation.choreo] C4 node advanced conversation=%s node=%s state=%s", conversationID, annotation.NodeID, annotation.State)
	}
	return annotation, nil
}

// ---- Readback entry ("读回刚才那条线") ----

var automationChoreoReadbackWords = []string{"读回", "读一下刚才", "刚才那条线", "那条包络", "包络读回", "read back"}

// handleAutomationChoreoReadback answers a readback request with the live
// curve of the last executed choreography.
func (s *Server) handleAutomationChoreoReadback(ctx context.Context, conversationID string, req ChatRequest, mode string) (ChatResponse, bool) {
	message := strings.ToLower(strings.TrimSpace(req.Message))
	if message == "" || !textHasAny(message, automationChoreoReadbackWords...) {
		return ChatResponse{}, false
	}
	s.mu.Lock()
	candidate, ok := s.lastAutomationChoreos[conversationID]
	s.mu.Unlock()
	if !ok {
		return ChatResponse{
			ConversationID: conversationID, AgentMode: mode,
			Reply: "这个对话里还没有已写入的段落包络可读回。", GoalStatus: string(agentruntime.StatusCompleted),
			StopReason: "automation_choreo_readback_none",
		}, true
	}
	if s.kernel == nil {
		return ChatResponse{
			ConversationID: conversationID, AgentMode: mode,
			Reply: "内核连接不可用，暂时读不了曲线。", GoalStatus: string(agentruntime.StatusFailed),
			StopReason: "automation_choreo_readback_kernel_unavailable",
		}, true
	}
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	reply, _, err := s.kernel.SendCommand(readCtx, map[string]any{
		"cmd": "automation.get_curve", "plugin_id": candidate.PluginID, "param_id": candidate.ParamID,
	})
	if err != nil || !strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "ok") {
		return ChatResponse{
			ConversationID: conversationID, AgentMode: mode, GoalID: candidate.GoalID, RunID: candidate.RunID,
			Reply:      "读回失败：" + firstNonEmpty(fmt.Sprint(reply["message"]), err.Error()),
			GoalStatus: string(agentruntime.StatusFailed), StopReason: "automation_choreo_readback_failed",
			Workflow: automationChoreoWorkflow,
		}, true
	}
	points := []map[string]any{}
	for _, row := range mapRowsFromAny(reply["points"]) {
		points = append(points, row)
	}
	matches := experiment.AutomationEnvelopeCurveMatches(automationChoreoPointsTyped(candidate.Points), points) == nil
	replyText := fmt.Sprintf("「%s」当前曲线（%d 点）：%s。", candidate.ParamID, len(points), automationChoreoPointsSummary(points))
	if matches {
		replyText += "与刚才写入的方案逐点一致。"
	} else {
		replyText += "注意：与刚才写入的方案不完全一致（曲线可能被后续操作改过）。"
	}
	return ChatResponse{
		ConversationID: conversationID, AgentMode: mode, GoalID: candidate.GoalID, RunID: candidate.RunID,
		Reply: replyText, GoalStatus: string(agentruntime.StatusCompleted), StopReason: "automation_choreo_readback_ok",
		Workflow: automationChoreoWorkflow,
		WorkflowData: map[string]any{
			"mutation_performed": false, "plugin_id": candidate.PluginID, "param_id": candidate.ParamID,
			"point_count": len(points), "matches_written": matches,
		},
	}, true
}

// automationChoreoPointsTyped converts the candidate's point maps into the
// typed shape for the readback matcher.
func automationChoreoPointsTyped(points []map[string]any) []experiment.AutomationEnvelopePoint {
	out := make([]experiment.AutomationEnvelopePoint, 0, len(points))
	for _, point := range points {
		t, _ := automationChoreoNumber(point["t"])
		v, _ := automationChoreoNumber(point["v"])
		c := 0.0
		if parsed, ok := automationChoreoNumber(point["c"]); ok {
			c = parsed
		}
		out = append(out, experiment.AutomationEnvelopePoint{T: t, V: v, C: c})
	}
	return out
}
