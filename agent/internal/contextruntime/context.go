package contextruntime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/pluginsemantics"
)

const SchemaVersion = "vit_context_snapshot.v1"

type Options struct {
	RecentTurns             int
	RecentTraceEvents       int
	MaxTextRunes            int
	MaxListItems            int
	MaxPreviewBytes         int
	Now                     func() time.Time
	SkipPluginSemanticLoad  bool
	PluginSemanticIndexPath string
}

type Input struct {
	ConversationID        string
	GoalID                string
	RunID                 string
	UserText              string
	GoalSummary           string
	Conversation          []llm.Message
	GoalTrace             []planner.TraceEvent
	Context               map[string]any
	State                 map[string]any
	ToolResults           []planner.ToolResult
	PluginContextPack     map[string]any
	PluginSemanticSummary map[string]any
	ProjectHistorySummary map[string]any
	PlanItems             []planner.PlanItem
	PendingToolCall       *planner.ToolCall
	PendingToolQueue      []planner.ToolCall
	ExecutionMemory       map[string]any
	RecentObservation     map[string]any
	PreviousSnapshot      map[string]any
}

type Snapshot struct {
	SchemaVersion         string           `json:"schema_version"`
	CreatedAt             string           `json:"created_at,omitempty"`
	ConversationID        string           `json:"conversation_id,omitempty"`
	GoalID                string           `json:"goal_id,omitempty"`
	RunID                 string           `json:"run_id,omitempty"`
	UserText              string           `json:"user_text,omitempty"`
	GoalSummary           string           `json:"goal_summary,omitempty"`
	ConversationSummary   map[string]any   `json:"conversation_summary"`
	GoalTraceSummary      map[string]any   `json:"goal_trace_summary,omitempty"`
	RecentTurns           []llm.Message    `json:"recent_turns,omitempty"`
	CurrentSelection      map[string]any   `json:"current_selection"`
	DAWStateSummary       map[string]any   `json:"daw_state_summary"`
	DAWSemanticSummary    map[string]any   `json:"daw_semantic_summary,omitempty"`
	RecentGoalContext     map[string]any   `json:"recent_goal_context,omitempty"`
	ProjectHistorySummary map[string]any   `json:"project_history_summary,omitempty"`
	PluginContextSummary  map[string]any   `json:"plugin_context_summary,omitempty"`
	ToolResultSummary     []map[string]any `json:"tool_result_summary,omitempty"`
	Warnings              []string         `json:"warnings,omitempty"`
}

func Build(in Input, opts Options) Snapshot {
	opts = normalizeOptions(opts)
	now := opts.Now()
	recentTurns, conversationSummary := summarizeConversation(in.Conversation, opts)
	goalTraceSummary, toolResultSummary := summarizeTrace(in.GoalTrace, opts)
	for _, result := range in.ToolResults {
		toolResultSummary = append(toolResultSummary, summarizeToolResult(result, opts))
	}
	if len(toolResultSummary) > opts.MaxListItems {
		toolResultSummary = toolResultSummary[len(toolResultSummary)-opts.MaxListItems:]
	}

	selection := summarizeSelection(in.Context, opts)
	dawStateSummary := summarizeDAWState(in.State, opts)
	projectHistorySummary := summarizeProjectHistory(in.ProjectHistorySummary, in.State, opts)
	pluginContextSummary := map[string]any{}
	warnings := []string{}

	pack := findPluginContextPack(in.PluginContextPack, in.Context, in.State)
	if pack == nil {
		for _, event := range in.GoalTrace {
			if event.ToolResult != nil && event.ToolResult.Result != nil {
				pack = findPluginContextPack(event.ToolResult.Result)
				if pack != nil {
					break
				}
			}
		}
	}
	if pack != nil {
		var packWarnings []string
		pluginContextSummary, packWarnings = summarizePluginContextPack(pack, opts)
		warnings = append(warnings, packWarnings...)
	}

	semantic := cloneMap(in.PluginSemanticSummary)
	if len(semantic) == 0 && !opts.SkipPluginSemanticLoad {
		semantic = LoadPluginSemanticSummary(opts.PluginSemanticIndexPath, opts.MaxListItems)
	}
	if len(semantic) > 0 {
		if pluginContextSummary == nil {
			pluginContextSummary = map[string]any{}
		}
		pluginContextSummary["semantic_library"] = semantic
	}
	if len(pluginContextSummary) == 0 {
		pluginContextSummary = nil
	}
	if len(in.PreviousSnapshot) > 0 {
		goalTraceSummary["inherited_snapshot"] = summarizeInheritedSnapshot(in.PreviousSnapshot)
	}
	dawSemanticSummary := summarizeDAWSemantic(selection, dawStateSummary, projectHistorySummary, in.ProjectHistorySummary, pluginContextSummary, toolResultSummary, in.Context, opts)
	recentGoalContext := summarizeRecentGoalContext(in.GoalTrace, in.PlanItems, in.PendingToolCall, in.PendingToolQueue, in.ExecutionMemory, in.RecentObservation, in.PreviousSnapshot, opts)

	return Snapshot{
		SchemaVersion:         SchemaVersion,
		CreatedAt:             now.UTC().Format(time.RFC3339Nano),
		ConversationID:        strings.TrimSpace(in.ConversationID),
		GoalID:                strings.TrimSpace(in.GoalID),
		RunID:                 strings.TrimSpace(in.RunID),
		UserText:              compactText(in.UserText, opts.MaxTextRunes),
		GoalSummary:           compactText(in.GoalSummary, opts.MaxTextRunes),
		ConversationSummary:   conversationSummary,
		GoalTraceSummary:      goalTraceSummary,
		RecentTurns:           recentTurns,
		CurrentSelection:      selection,
		DAWStateSummary:       dawStateSummary,
		DAWSemanticSummary:    dawSemanticSummary,
		RecentGoalContext:     recentGoalContext,
		ProjectHistorySummary: projectHistorySummary,
		PluginContextSummary:  pluginContextSummary,
		ToolResultSummary:     toolResultSummary,
		Warnings:              warnings,
	}
}

func (s Snapshot) Map() map[string]any {
	b, err := json.Marshal(s)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func (s Snapshot) JSON() string {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func AppendJSONL(path string, snapshot Snapshot) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(snapshot)
}

func AppendDefault(snapshot Snapshot) error {
	if override := strings.TrimSpace(os.Getenv("VIT_CONTEXT_SNAPSHOT_PATH")); override != "" {
		if strings.EqualFold(override, "off") || strings.EqualFold(override, "disabled") {
			return nil
		}
		return AppendJSONL(override, snapshot)
	}
	return AppendJSONL(DefaultSnapshotPath(), snapshot)
}

func ReadJSONL(path string, limit int) ([]Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Snapshot
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var snap Snapshot
		if err := json.Unmarshal([]byte(line), &snap); err != nil {
			return out, err
		}
		out = append(out, snap)
		if limit > 0 && len(out) > limit {
			out = out[len(out)-limit:]
		}
	}
	if err := scanner.Err(); err != nil {
		return out, err
	}
	return out, nil
}

func DefaultSnapshotPath() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if root := detectWorkspaceRoot(wd); root != "" {
		return filepath.Join(root, "VitApp", "Workspace", "Logs", "agent_context_snapshots.jsonl")
	}
	return filepath.Join(wd, "VitApp", "Workspace", "Logs", "agent_context_snapshots.jsonl")
}

func detectWorkspaceRoot(start string) string {
	for dir := start; dir != ""; dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "agent", "go.mod")) && dirExists(filepath.Join(dir, "VitApp")) {
			return dir
		}
		if filepath.Base(dir) == "agent" && fileExists(filepath.Join(dir, "go.mod")) {
			root := filepath.Dir(dir)
			if dirExists(filepath.Join(root, "VitApp")) {
				return root
			}
		}
		if filepath.Base(dir) == "VitApp" && dirExists(filepath.Join(dir, "Workspace")) {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func LoadPluginSemanticSummary(path string, maxCandidates int) map[string]any {
	if maxCandidates <= 0 {
		maxCandidates = 8
	}
	defaultPath, _ := pluginsemantics.DefaultPath()
	actualPath := firstNonEmpty(path, defaultPath)
	idx, err := pluginsemantics.Load(path)
	if err != nil {
		out := map[string]any{
			"index_exists": false,
			"index_path":   actualPath,
		}
		if !pluginsemantics.IsNotExist(err) {
			out["error"] = err.Error()
		}
		return out
	}
	entries := append([]pluginsemantics.Entry(nil), idx.Entries...)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Confidence != entries[j].Confidence {
			return entries[i].Confidence > entries[j].Confidence
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	if len(entries) > maxCandidates {
		entries = entries[:maxCandidates]
	}
	candidates := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		candidates = append(candidates, map[string]any{
			"id":            entry.ID,
			"name":          firstNonEmpty(entry.Name, entry.DescriptiveName, filepath.Base(entry.PluginPath)),
			"manufacturer":  entry.Manufacturer,
			"format":        entry.Format,
			"primary_type":  entry.PrimaryType,
			"confidence":    entry.Confidence,
			"is_instrument": entry.IsInstrument,
		})
	}
	out := map[string]any{
		"index_exists":            true,
		"index_path":              actualPath,
		"entry_count":             len(idx.Entries),
		"type_counts":             idx.Summary,
		"top_semantic_candidates": candidates,
	}
	if !idx.BuiltAt.IsZero() {
		out["built_at"] = idx.BuiltAt.Format(time.RFC3339)
	}
	return out
}

func normalizeOptions(opts Options) Options {
	if opts.RecentTurns <= 0 {
		opts.RecentTurns = 8
	}
	if opts.RecentTraceEvents <= 0 {
		opts.RecentTraceEvents = 12
	}
	if opts.MaxTextRunes <= 0 {
		opts.MaxTextRunes = 900
	}
	if opts.MaxListItems <= 0 {
		opts.MaxListItems = 20
	}
	if opts.MaxPreviewBytes <= 0 {
		opts.MaxPreviewBytes = 12 * 1024
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return opts
}

func summarizeConversation(messages []llm.Message, opts Options) ([]llm.Message, map[string]any) {
	userCount := 0
	assistantCount := 0
	for _, msg := range messages {
		switch strings.ToLower(strings.TrimSpace(msg.Role)) {
		case "user":
			userCount++
		case "assistant":
			assistantCount++
		}
	}
	recentStart := 0
	if len(messages) > opts.RecentTurns {
		recentStart = len(messages) - opts.RecentTurns
	}
	recent := make([]llm.Message, 0, len(messages)-recentStart)
	for _, msg := range messages[recentStart:] {
		recent = append(recent, llm.Message{
			Role:    strings.TrimSpace(msg.Role),
			Content: compactText(msg.Content, opts.MaxTextRunes),
		})
	}
	older := messages[:recentStart]
	summary := map[string]any{
		"message_count":           len(messages),
		"omitted_message_count":   len(older),
		"recent_message_count":    len(recent),
		"user_message_count":      userCount,
		"assistant_message_count": assistantCount,
	}
	if first := firstMessageByRole(older, "user"); first != "" {
		summary["first_user_message"] = compactText(first, opts.MaxTextRunes/2)
	}
	if last := lastMessageByRole(older, "user"); last != "" {
		summary["last_omitted_user_message"] = compactText(last, opts.MaxTextRunes/2)
	}
	if last := lastMessageByRole(older, "assistant"); last != "" {
		summary["last_omitted_assistant_message"] = compactText(last, opts.MaxTextRunes/2)
	}
	return recent, summary
}

func summarizeTrace(trace []planner.TraceEvent, opts Options) (map[string]any, []map[string]any) {
	summary := map[string]any{
		"event_count":         len(trace),
		"omitted_event_count": 0,
	}
	if len(trace) > opts.RecentTraceEvents {
		summary["omitted_event_count"] = len(trace) - opts.RecentTraceEvents
	}
	toolCounts := map[string]int{}
	errorCount := 0
	var lastAssistant, lastError string
	toolResults := []map[string]any{}
	for _, ev := range trace {
		if strings.TrimSpace(ev.Reply) != "" {
			lastAssistant = compactText(ev.Reply, opts.MaxTextRunes/2)
		}
		if ev.ToolCall != nil {
			tool := strings.TrimSpace(ev.ToolCall.Tool)
			if tool != "" {
				toolCounts[tool]++
			}
		}
		if ev.ToolResult != nil {
			if strings.TrimSpace(ev.ToolResult.Tool) != "" {
				toolCounts[ev.ToolResult.Tool]++
			}
			if strings.TrimSpace(ev.ToolResult.Error) != "" || strings.EqualFold(ev.ToolResult.Status, "error") || strings.EqualFold(ev.ToolResult.Status, "kernel_error") {
				errorCount++
				lastError = compactText(firstNonEmpty(ev.ToolResult.Error, ev.ToolResult.Status), opts.MaxTextRunes/2)
			}
			toolResults = append(toolResults, summarizeToolResult(*ev.ToolResult, opts))
		}
		if strings.EqualFold(ev.Kind, "planner_error") && strings.TrimSpace(ev.Message) != "" {
			errorCount++
			lastError = compactText(ev.Message, opts.MaxTextRunes/2)
		}
	}
	if len(toolCounts) > 0 {
		summary["tool_counts"] = toolCounts
	}
	if errorCount > 0 {
		summary["error_count"] = errorCount
		summary["last_error"] = lastError
	}
	if lastAssistant != "" {
		summary["last_assistant_reply"] = lastAssistant
	}
	if len(trace) > 0 {
		start := 0
		if len(trace) > opts.RecentTraceEvents {
			start = len(trace) - opts.RecentTraceEvents
		}
		recent := make([]map[string]any, 0, len(trace)-start)
		for _, ev := range trace[start:] {
			recent = append(recent, compactTraceEvent(ev, opts))
		}
		summary["recent_events"] = recent
	}
	return summary, toolResults
}

func compactTraceEvent(ev planner.TraceEvent, opts Options) map[string]any {
	out := map[string]any{"kind": ev.Kind}
	if ev.Message != "" {
		out["message"] = compactText(ev.Message, opts.MaxTextRunes)
	}
	if ev.Reply != "" {
		out["reply"] = compactText(ev.Reply, opts.MaxTextRunes)
	}
	if ev.ToolCall != nil {
		out["tool_call"] = map[string]any{
			"id":           ev.ToolCall.ID,
			"tool":         ev.ToolCall.Tool,
			"reason":       compactText(ev.ToolCall.Reason, opts.MaxTextRunes/2),
			"plan_item_id": ev.ToolCall.PlanItemID,
			"args":         compactValue(ev.ToolCall.Args, opts, 0),
			"command":      compactValue(ev.ToolCall.Command, opts, 0),
		}
	}
	if ev.ToolResult != nil {
		out["tool_result"] = summarizeToolResult(*ev.ToolResult, opts)
	}
	if ev.Verification != nil {
		out["verification"] = map[string]any{
			"tool_call_id":  ev.Verification.ToolCallID,
			"plan_item_id":  ev.Verification.PlanItemID,
			"tool":          ev.Verification.Tool,
			"command_name":  ev.Verification.CommandName,
			"status":        ev.Verification.Status,
			"postcondition": ev.Verification.Postcondition,
			"message":       compactText(ev.Verification.Message, opts.MaxTextRunes),
			"evidence":      compactValue(ev.Verification.Evidence, opts, 0),
		}
	}
	if len(ev.PlanItems) > 0 {
		items := make([]map[string]any, 0, len(ev.PlanItems))
		limit := len(ev.PlanItems)
		if limit > opts.MaxListItems {
			limit = opts.MaxListItems
		}
		for i := 0; i < limit; i++ {
			item := ev.PlanItems[i]
			row := map[string]any{
				"id":          item.ID,
				"description": compactText(item.Description, opts.MaxTextRunes/2),
				"status":      item.Status,
				"evidence":    compactText(item.Evidence, opts.MaxTextRunes/2),
			}
			removeEmpty(row)
			items = append(items, row)
		}
		out["plan_items"] = items
	}
	return out
}

func SummarizeToolResult(result planner.ToolResult, opts Options) map[string]any {
	return summarizeToolResult(result, normalizeOptions(opts))
}

func CompactValue(value any, opts Options) any {
	return compactValue(value, normalizeOptions(opts), 0)
}

func summarizeToolResult(result planner.ToolResult, opts Options) map[string]any {
	out := map[string]any{
		"tool_call_id": result.ToolCallID,
		"tool":         result.Tool,
		"status":       result.Status,
	}
	if strings.TrimSpace(result.Error) != "" {
		out["error"] = compactText(result.Error, opts.MaxTextRunes)
	}
	if len(result.Result) > 0 {
		out["result_keys"] = sortedKeys(result.Result)
		important := map[string]any{}
		collectImportantFields(important, "", result.Result, 0, opts.MaxListItems)
		if len(important) > 0 {
			out["important_fields"] = important
		}
		out["result_preview"] = compactResultPreview(result.Result, compactValue(result.Result, opts, 0), opts)
	}
	return out
}

func summarizeSelection(ctx map[string]any, opts Options) map[string]any {
	out := map[string]any{}
	keys := []string{
		"selected_track_id", "selected_track_name", "selected_scene_track_id",
		"selected_clip_id", "selected_clip_track_id", "selected_clip_name",
		"piano_roll_focus_clip_id", "piano_roll_focus_track_id",
		"selected_plugin_id", "selected_plugin_name", "selected_plugin_track_id", "selected_plugin_source",
		"playhead_seconds", "current_playhead_seconds", "transport_position_seconds",
		"selected_library_file_path", "selected_library_item_name", "selected_library_kind", "library_search_query",
	}
	for _, key := range keys {
		if value, ok := ctx[key]; ok && !isEmptyValue(value) {
			out[key] = compactValue(value, opts, 0)
		}
	}
	if ids := stringSlice(ctx["selected_clip_ids"]); len(ids) > 0 {
		out["selected_clip_ids"] = ids
	}
	if places := stringSlice(ctx["library_places"]); len(places) > 0 {
		out["library_places"] = firstStrings(places, opts.MaxListItems)
	}
	if attachments := mapRows(ctx["attachments"]); len(attachments) > 0 {
		out["attachments"] = compactValue(attachments, opts, 0)
	}
	if artifacts := mapRows(ctx["artifacts"]); len(artifacts) > 0 {
		out["artifacts"] = compactRows(artifacts, []string{"id", "kind", "title", "source", "status", "summary", "mime", "size_bytes", "url", "path"}, opts)
	}
	if ids := stringSlice(ctx["artifact_ids"]); len(ids) > 0 {
		out["artifact_ids"] = firstStrings(ids, opts.MaxListItems)
	}
	if id := strings.TrimSpace(fmt.Sprint(ctx["selected_artifact_id"])); id != "" && id != "<nil>" {
		out["selected_artifact_id"] = id
	}
	if len(out) == 0 {
		return map[string]any{}
	}
	return out
}

func summarizeDAWState(state map[string]any, opts Options) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"initialized", "project_path", "track_count", "user_track_count", "engine_track_count", "internal_track_count", "graph_revision"} {
		if value, ok := state[key]; ok && !isEmptyValue(value) {
			out[key] = compactValue(value, opts, 0)
		}
	}
	tracks := mapRows(state["tracks"])
	if len(tracks) > 0 {
		out["tracks"] = summarizeTracks(tracks, opts)
		out["track_count"] = len(tracks)
		plugins := summarizeLoadedPlugins(tracks, opts)
		if len(plugins) > 0 {
			out["loaded_plugins"] = plugins
		}
	}
	if transport := firstNonEmptyMap(state, "transport", "transport_state"); transport != nil {
		out["transport"] = compactValue(transport, opts, 0)
	}
	if obs := firstNonEmptyMap(state, "observability"); obs != nil {
		out["observability"] = compactValue(obs, opts, 0)
	}
	for _, key := range []string{"macros", "rack_control_macros", "control_macros", "project_health", "pending_job_data"} {
		if value, ok := state[key]; ok && !isEmptyValue(value) {
			out[key] = compactValue(value, opts, 0)
		}
	}
	if len(out) == 0 {
		return map[string]any{"initialized": false}
	}
	return out
}

func summarizeProjectHistory(projectHistory map[string]any, state map[string]any, opts Options) map[string]any {
	src := cloneMap(projectHistory)
	if len(src) == 0 {
		if nested := firstNonEmptyMap(state, "project_history", "project_history_summary"); nested != nil {
			src = cloneMap(nested)
		}
	}
	if len(src) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, key := range []string{
		"available", "active_branch", "detached", "head", "baseline_commit", "baseline_created",
		"commit_count", "worktree_count", "warnings",
	} {
		if value, ok := src[key]; ok && !isEmptyValue(value) {
			out[key] = compactValue(value, opts, 0)
		}
	}
	if branches, ok := src["branches"]; ok && !isEmptyValue(branches) {
		out["branches"] = compactValue(branches, opts, 0)
	}
	if recent, ok := src["recent_checkpoints"]; ok && !isEmptyValue(recent) {
		out["recent_checkpoints"] = compactValue(recent, opts, 0)
	}
	if worktrees, ok := src["worktrees"]; ok && !isEmptyValue(worktrees) {
		out["worktrees"] = compactValue(worktrees, opts, 0)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func summarizeDAWSemantic(selection, dawStateSummary, projectHistorySummary, rawProjectHistory, pluginContextSummary map[string]any, toolResultSummary []map[string]any, ctx map[string]any, opts Options) map[string]any {
	out := map[string]any{}
	if len(selection) > 0 {
		out["current_selection"] = compactValue(selection, opts, 0)
	}
	tracks := mapRows(dawStateSummary["tracks"])
	if len(tracks) > 0 {
		out["visible_tracks"] = summarizeVisibleTracks(tracks, opts)
		if clips := collectClipTimeRanges(tracks, opts); len(clips) > 0 {
			out["clip_time_ranges"] = clips
		}
		if midi := collectMIDIClipOverview(tracks, opts); len(midi) > 0 {
			out["midi_clip_overview"] = midi
		}
	}
	if loaded := mapRows(dawStateSummary["loaded_plugins"]); len(loaded) > 0 {
		out["loaded_plugins"] = compactValue(loaded, opts, 0)
	}
	if len(pluginContextSummary) > 0 {
		out["plugin_context"] = compactValue(pluginContextSummary, opts, 0)
	}
	if branch := firstNonEmpty(contextText(ctx, "active_branch"), mapText(projectHistorySummary, "active_branch")); branch != "" {
		out["active_branch"] = branch
	}
	if worktree := firstNonEmpty(contextText(ctx, "active_worktree"), contextText(ctx, "project_path"), contextText(ctx, "root_project_path"), mapText(projectHistorySummary, "active_worktree")); worktree != "" {
		out["active_worktree"] = worktree
	}
	if nodeID := firstNonEmpty(contextText(ctx, "active_node_id"), mapText(projectHistorySummary, "active_node_id")); nodeID != "" {
		out["active_node_id"] = nodeID
	}
	if nodes := recentAskVitNodes(rawProjectHistory, opts); len(nodes) > 0 {
		out["recent_ask_vit_nodes"] = nodes
	}
	if len(toolResultSummary) > 0 {
		out["recent_execution_result"] = toolResultSummary[len(toolResultSummary)-1]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func summarizeVisibleTracks(tracks []map[string]any, opts Options) []map[string]any {
	limit := len(tracks)
	if limit > opts.MaxListItems {
		limit = opts.MaxListItems
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := tracks[i]
		item := map[string]any{
			"track_id":         firstPresent(row, "track_id", "id"),
			"track_name":       firstPresent(row, "track_name", "name"),
			"track_type":       firstPresent(row, "track_type", "type"),
			"user_track_index": firstPresent(row, "user_track_index"),
			"clip_count":       firstPresent(row, "clip_count"),
			"plugin_count":     firstPresent(row, "plugin_count"),
			"rack_node_count":  firstPresent(row, "rack_node_count"),
		}
		removeEmpty(item)
		out = append(out, item)
	}
	return out
}

func collectClipTimeRanges(tracks []map[string]any, opts Options) []map[string]any {
	out := []map[string]any{}
	for _, track := range tracks {
		trackID := firstPresent(track, "track_id", "id")
		trackName := firstPresent(track, "track_name", "name")
		for _, clip := range mapRows(track["clips"]) {
			row := map[string]any{
				"track_id":       trackID,
				"track_name":     trackName,
				"clip_id":        firstPresent(clip, "clip_id", "id"),
				"clip_name":      firstPresent(clip, "clip_name", "name"),
				"type":           firstPresent(clip, "type", "clip_type"),
				"start_seconds":  firstPresent(clip, "start_seconds", "start"),
				"length_seconds": firstPresent(clip, "length_seconds", "length"),
			}
			removeEmpty(row)
			if len(row) > 0 {
				out = append(out, row)
			}
			if len(out) >= opts.MaxListItems {
				return out
			}
		}
	}
	return out
}

func collectMIDIClipOverview(tracks []map[string]any, opts Options) []map[string]any {
	out := []map[string]any{}
	for _, track := range tracks {
		trackID := firstPresent(track, "track_id", "id")
		trackName := firstPresent(track, "track_name", "name")
		for _, clip := range mapRows(track["clips"]) {
			kind := strings.ToLower(firstNonEmpty(fmt.Sprint(firstPresent(clip, "type", "clip_type")), fmt.Sprint(firstPresent(clip, "clip_name", "name"))))
			if !strings.Contains(kind, "midi") {
				continue
			}
			row := map[string]any{
				"track_id":       trackID,
				"track_name":     trackName,
				"clip_id":        firstPresent(clip, "clip_id", "id"),
				"clip_name":      firstPresent(clip, "clip_name", "name"),
				"start_seconds":  firstPresent(clip, "start_seconds", "start"),
				"length_seconds": firstPresent(clip, "length_seconds", "length"),
				"note_count":     firstPresent(clip, "note_count", "notes_count"),
			}
			removeEmpty(row)
			out = append(out, row)
			if len(out) >= opts.MaxListItems {
				return out
			}
		}
	}
	return out
}

func recentAskVitNodes(projectHistory map[string]any, opts Options) []map[string]any {
	rows := mapRows(projectHistory["conversation_messages"])
	if len(rows) == 0 {
		return nil
	}
	start := 0
	if len(rows) > opts.MaxListItems {
		start = len(rows) - opts.MaxListItems
	}
	out := make([]map[string]any, 0, len(rows)-start)
	for _, row := range rows[start:] {
		item := map[string]any{
			"role":       firstPresent(row, "role"),
			"content":    compactText(fmt.Sprint(firstPresent(row, "content")), opts.MaxTextRunes/2),
			"node_id":    firstPresent(row, "node_id"),
			"commit_id":  firstPresent(row, "commit_id"),
			"branch":     firstPresent(row, "branch"),
			"created_at": firstPresent(row, "created_at"),
		}
		removeEmpty(item)
		if len(item) > 0 {
			out = append(out, item)
		}
	}
	return out
}

func summarizeRecentGoalContext(trace []planner.TraceEvent, planItems []planner.PlanItem, pendingToolCall *planner.ToolCall, pendingToolQueue []planner.ToolCall, executionMemory map[string]any, recentObservation map[string]any, previous map[string]any, opts Options) map[string]any {
	out := map[string]any{}
	steps := planItems
	if len(steps) == 0 {
		steps = latestPlanItemsFromTrace(trace)
	}
	if len(steps) > 0 {
		out["plan_steps"] = summarizePlanSteps(steps, opts)
	}
	if ver := latestVerificationFromTrace(trace, opts); len(ver) > 0 {
		out["recent_verification"] = ver
	}
	if pendingToolCall != nil {
		out["pending_confirmation"] = summarizeToolCall(*pendingToolCall, opts)
	}
	if len(pendingToolQueue) > 0 {
		out["pending_tool_queue"] = summarizeToolCalls(pendingToolQueue, opts)
	}
	if len(executionMemory) > 0 {
		out["execution_memory"] = compactValue(executionMemory, opts, 0)
	}
	if len(recentObservation) > 0 {
		out["recent_observation"] = compactValue(recentObservation, opts, 0)
	}
	if result := latestToolResultFromTrace(trace, opts); len(result) > 0 {
		out["recent_tool_result"] = result
	}
	if inherited, ok := previous["recent_goal_context"].(map[string]any); ok && len(inherited) > 0 {
		out["inherited_recent_goal_context"] = compactValue(inherited, opts, 0)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func summarizePlanSteps(items []planner.PlanItem, opts Options) []map[string]any {
	limit := len(items)
	if limit > opts.MaxListItems {
		limit = opts.MaxListItems
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		item := items[i]
		row := map[string]any{
			"id":          item.ID,
			"description": compactText(item.Description, opts.MaxTextRunes/2),
			"status":      item.Status,
			"evidence":    compactText(item.Evidence, opts.MaxTextRunes/2),
		}
		if len(item.Metadata) > 0 {
			row["metadata"] = compactValue(item.Metadata, opts, 0)
		}
		removeEmpty(row)
		out = append(out, row)
	}
	return out
}

func latestPlanItemsFromTrace(trace []planner.TraceEvent) []planner.PlanItem {
	for i := len(trace) - 1; i >= 0; i-- {
		if len(trace[i].PlanItems) > 0 {
			return append([]planner.PlanItem(nil), trace[i].PlanItems...)
		}
	}
	return nil
}

func latestVerificationFromTrace(trace []planner.TraceEvent, opts Options) map[string]any {
	for i := len(trace) - 1; i >= 0; i-- {
		ver := trace[i].Verification
		if ver == nil {
			continue
		}
		row := map[string]any{
			"tool_call_id":  ver.ToolCallID,
			"plan_item_id":  ver.PlanItemID,
			"tool":          ver.Tool,
			"command_name":  ver.CommandName,
			"status":        ver.Status,
			"postcondition": ver.Postcondition,
			"message":       compactText(ver.Message, opts.MaxTextRunes),
			"evidence":      compactValue(ver.Evidence, opts, 0),
		}
		removeEmpty(row)
		return row
	}
	return nil
}

func latestToolResultFromTrace(trace []planner.TraceEvent, opts Options) map[string]any {
	for i := len(trace) - 1; i >= 0; i-- {
		if trace[i].ToolResult != nil {
			return summarizeToolResult(*trace[i].ToolResult, opts)
		}
	}
	return nil
}

func summarizeToolCalls(calls []planner.ToolCall, opts Options) []map[string]any {
	limit := len(calls)
	if limit > opts.MaxListItems {
		limit = opts.MaxListItems
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, summarizeToolCall(calls[i], opts))
	}
	return out
}

func summarizeToolCall(call planner.ToolCall, opts Options) map[string]any {
	row := map[string]any{
		"id":           call.ID,
		"tool":         call.Tool,
		"plan_item_id": call.PlanItemID,
		"reason":       compactText(call.Reason, opts.MaxTextRunes/2),
		"args":         compactValue(call.Args, opts, 0),
		"command":      compactValue(call.Command, opts, 0),
	}
	removeEmpty(row)
	return row
}

func summarizeTracks(tracks []map[string]any, opts Options) []map[string]any {
	limit := len(tracks)
	if limit > opts.MaxListItems {
		limit = opts.MaxListItems
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := tracks[i]
		plugins := mapRows(row["plugins"])
		rackNodes := rackNodeRows(row)
		clips := mapRows(row["clips"])
		item := map[string]any{
			"user_track_index": firstPresent(row, "user_track_index"),
			"track_id":         firstPresent(row, "track_id", "id"),
			"track_name":       firstPresent(row, "track_name", "name"),
			"track_type":       firstPresent(row, "track_type", "type"),
			"mute":             firstPresent(row, "mute", "muted"),
			"solo":             firstPresent(row, "solo", "is_solo"),
			"is_armed":         firstPresent(row, "is_armed", "armed"),
			"clip_count":       len(clips),
			"plugin_count":     len(plugins),
			"rack_node_count":  len(rackNodes),
		}
		removeEmpty(item)
		if len(plugins) > 0 {
			item["plugins"] = summarizePluginRows(plugins, opts)
		}
		if len(rackNodes) > 0 {
			item["rack_nodes"] = summarizePluginRows(rackNodes, opts)
		}
		if len(clips) > 0 {
			item["clips"] = summarizeClipRows(clips, opts)
		}
		out = append(out, item)
	}
	return out
}

func summarizeLoadedPlugins(tracks []map[string]any, opts Options) []map[string]any {
	var out []map[string]any
	for _, track := range tracks {
		trackID := fmt.Sprint(firstPresent(track, "track_id", "id"))
		trackName := fmt.Sprint(firstPresent(track, "track_name", "name"))
		for _, plugin := range mapRows(track["plugins"]) {
			row := summarizePluginIdentity(plugin)
			row["source"] = "track_plugin_chain"
			if trackID != "" && trackID != "<nil>" {
				row["track_id"] = trackID
			}
			if trackName != "" && trackName != "<nil>" {
				row["track_name"] = trackName
			}
			removeEmpty(row)
			out = append(out, row)
			if len(out) >= opts.MaxListItems {
				return out
			}
		}
		for _, plugin := range rackNodeRows(track) {
			row := summarizePluginIdentity(plugin)
			row["source"] = "rack_node"
			if rack := firstNonEmptyMap(track, "rack"); rack != nil {
				if rackID := firstPresent(rack, "rack_item_id", "id"); rackID != nil {
					row["rack_item_id"] = rackID
				}
			}
			if trackID != "" && trackID != "<nil>" {
				row["track_id"] = trackID
			}
			if trackName != "" && trackName != "<nil>" {
				row["track_name"] = trackName
			}
			removeEmpty(row)
			out = append(out, row)
			if len(out) >= opts.MaxListItems {
				return out
			}
		}
	}
	return out
}

func rackNodeRows(track map[string]any) []map[string]any {
	rack := firstNonEmptyMap(track, "rack")
	if rack == nil {
		return nil
	}
	return mapRows(rack["nodes"])
}

func summarizePluginRows(plugins []map[string]any, opts Options) []map[string]any {
	limit := len(plugins)
	if limit > opts.MaxListItems {
		limit = opts.MaxListItems
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, summarizePluginIdentity(plugins[i]))
	}
	return out
}

func summarizePluginIdentity(row map[string]any) map[string]any {
	out := map[string]any{
		"plugin_id":       firstPresent(row, "plugin_id", "plugin_item_id", "node_id", "item_id", "id", "uid"),
		"plugin_name":     firstPresent(row, "plugin_name", "name", "descriptive_name"),
		"manufacturer":    firstPresent(row, "manufacturer", "vendor"),
		"format":          firstPresent(row, "format"),
		"category":        firstPresent(row, "category"),
		"bypass":          firstPresent(row, "bypass", "is_bypassed"),
		"enabled":         firstPresent(row, "enabled"),
		"parameter_count": firstPresent(row, "parameter_count"),
	}
	removeEmpty(out)
	return out
}

func summarizeClipRows(clips []map[string]any, opts Options) []map[string]any {
	limit := len(clips)
	if limit > opts.MaxListItems {
		limit = opts.MaxListItems
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := clips[i]
		item := map[string]any{
			"clip_id":        firstPresent(row, "clip_id", "id"),
			"clip_name":      firstPresent(row, "clip_name", "name"),
			"start_seconds":  firstPresent(row, "start_seconds", "start"),
			"length_seconds": firstPresent(row, "length_seconds", "length"),
			"type":           firstPresent(row, "type", "clip_type"),
		}
		removeEmpty(item)
		out = append(out, item)
	}
	return out
}

func summarizePluginContextPack(pack map[string]any, opts Options) (map[string]any, []string) {
	warnings := []string{}
	out := map[string]any{}
	for _, key := range []string{
		"schema_version", "context_strategy", "track_id", "plugin_id", "plugin_name",
		"plugin_identity", "template_role", "profile_source", "profile_applied",
		"parameters_retained", "parameter_count", "quick_control_count",
		"recommended_group_count", "profile_stale_param_ids",
		"display_probe_summary",
	} {
		if value, ok := pack[key]; ok && !isEmptyValue(value) {
			out[key] = compactValue(value, opts, 0)
		}
	}
	if rows := mapRows(pack["quick_controls"]); len(rows) > 0 {
		out["quick_controls"] = compactRows(rows, []string{"param_id", "label", "widget", "display_group", "normalized_role", "value", "normalized_value", "value_text", "host_controllable", "control_relevance", "display_domain_candidate", "display_probe", "explanation_hint", "full_parameter_ref"}, opts)
	}
	if rows := mapRows(pack["groups"]); len(rows) > 0 {
		out["groups"] = compactRows(rows, []string{"name", "parameter_count", "quick_control_ids", "sample_param_ids"}, opts)
	}
	if rows := mapRows(pack["role_summary"]); len(rows) > 0 {
		out["role_summary"] = compactRows(rows, []string{"normalized_role", "parameter_count"}, opts)
	}
	if rows := mapRows(pack["macro_candidates"]); len(rows) > 0 {
		out["macro_candidates"] = compactRows(rows, []string{"param_id", "label", "display_group", "normalized_role", "confidence", "candidate_reason", "requires_user_review"}, opts)
	}
	if access, ok := pack["full_parameter_access"].(map[string]any); ok {
		out["full_parameter_access"] = compactValue(access, opts, 0)
	}
	if n := listLen(pack["parameters"]); n > 0 {
		if _, ok := out["parameter_count"]; !ok {
			out["parameter_count"] = n
		}
		warnings = append(warnings, fmt.Sprintf("plugin context pack contained %d full parameters; snapshot kept only summary metadata", n))
	}
	return out, warnings
}

func compactRows(rows []map[string]any, keys []string, opts Options) []map[string]any {
	limit := opts.MaxListItems
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		item := map[string]any{}
		for _, key := range keys {
			if value, ok := row[key]; ok && !isEmptyValue(value) {
				item[key] = compactValue(value, opts, 0)
			}
		}
		out = append(out, item)
	}
	return out
}

func compactResultPreview(original map[string]any, preview any, opts Options) any {
	if opts.MaxPreviewBytes <= 0 || jsonByteLen(preview) <= opts.MaxPreviewBytes {
		return preview
	}
	out := map[string]any{
		"preview_omitted": fmt.Sprintf("compacted result preview exceeded %d bytes", opts.MaxPreviewBytes),
		"top_level_keys":  compactKeyList(sortedKeys(original), opts.MaxListItems),
	}
	for _, key := range []string{
		"status", "track_id", "track_name", "clip_id", "clip_name",
		"plugin_id", "plugin_item_id", "plugin_name", "plugin_class",
		"parameter_count", "quick_control_count", "recommended_group_count",
		"schema_version", "context_strategy", "profile_source", "profile_applied",
		"command_name", "agent_action_id", "goal_id", "run_id",
	} {
		if value, ok := original[key]; ok && !isEmptyValue(value) {
			out[key] = compactValue(value, opts, 0)
		}
	}
	collections := []map[string]any{}
	collectCollectionSummaries(&collections, "", original, 0, opts.MaxListItems)
	if len(collections) > 0 {
		out["collection_summaries"] = collections
	}
	removeEmpty(out)
	return out
}

func compactKeyList(keys []string, limit int) []string {
	if limit <= 0 || len(keys) <= limit {
		return append([]string(nil), keys...)
	}
	out := append([]string(nil), keys[:limit]...)
	out = append(out, fmt.Sprintf("<omitted %d keys>", len(keys)-limit))
	return out
}

func collectCollectionSummaries(out *[]map[string]any, path string, value any, depth int, limit int) {
	if out == nil || len(*out) >= limit || depth > 4 {
		return
	}
	switch v := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(v) {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			child := v[key]
			if n := listLen(child); n > 0 {
				*out = append(*out, map[string]any{"path": childPath, "item_count": n})
				if len(*out) >= limit {
					return
				}
			}
			collectCollectionSummaries(out, childPath, child, depth+1, limit)
			if len(*out) >= limit {
				return
			}
		}
	case []any:
		for i, child := range v {
			if i >= 3 || len(*out) >= limit {
				return
			}
			collectCollectionSummaries(out, fmt.Sprintf("%s[%d]", path, i), child, depth+1, limit)
		}
	case []map[string]any:
		for i, child := range v {
			if i >= 3 || len(*out) >= limit {
				return
			}
			collectCollectionSummaries(out, fmt.Sprintf("%s[%d]", path, i), child, depth+1, limit)
		}
	}
}

func jsonByteLen(value any) int {
	data, err := json.Marshal(value)
	if err != nil {
		return len(fmt.Sprint(value))
	}
	return len(data)
}

func compactValue(value any, opts Options, depth int) any {
	if depth > 5 {
		return fmt.Sprintf("<%T>", value)
	}
	switch v := value.(type) {
	case nil:
		return nil
	case map[string]any:
		out := map[string]any{}
		for key, child := range v {
			if strings.EqualFold(key, "snapshot_xml") {
				out[key] = fmt.Sprintf("<omitted %d chars>", len(fmt.Sprint(child)))
				continue
			}
			if strings.EqualFold(key, "parameters") {
				if n := listLen(child); n > 0 {
					out["parameter_count"] = n
					out[key] = fmt.Sprintf("<omitted %d parameters>", n)
					continue
				}
			}
			out[key] = compactValue(child, opts, depth+1)
		}
		return out
	case []map[string]any:
		limit := len(v)
		if limit > opts.MaxListItems {
			limit = opts.MaxListItems
		}
		out := make([]any, 0, limit+1)
		for i := 0; i < limit; i++ {
			out = append(out, compactValue(v[i], opts, depth+1))
		}
		if len(v) > limit {
			out = append(out, fmt.Sprintf("<omitted %d items>", len(v)-limit))
		}
		return out
	case []any:
		limit := len(v)
		if limit > opts.MaxListItems {
			limit = opts.MaxListItems
		}
		out := make([]any, 0, limit+1)
		for i := 0; i < limit; i++ {
			out = append(out, compactValue(v[i], opts, depth+1))
		}
		if len(v) > limit {
			out = append(out, fmt.Sprintf("<omitted %d items>", len(v)-limit))
		}
		return out
	case []string:
		if len(v) > opts.MaxListItems {
			return append(firstStrings(v, opts.MaxListItems), fmt.Sprintf("<omitted %d items>", len(v)-opts.MaxListItems))
		}
		return append([]string(nil), v...)
	case string:
		return compactText(v, opts.MaxTextRunes)
	default:
		return v
	}
}

func findPluginContextPack(values ...any) map[string]any {
	for _, value := range values {
		if pack := findPluginContextPackDepth(value, 0); pack != nil {
			return pack
		}
	}
	return nil
}

func findPluginContextPackDepth(value any, depth int) map[string]any {
	if depth > 5 || value == nil {
		return nil
	}
	switch v := value.(type) {
	case map[string]any:
		if isPluginContextPack(v) {
			return v
		}
		for _, key := range sortedKeys(v) {
			child := v[key]
			if pack := findPluginContextPackDepth(child, depth+1); pack != nil {
				return pack
			}
		}
	case []any:
		for _, child := range v {
			if pack := findPluginContextPackDepth(child, depth+1); pack != nil {
				return pack
			}
		}
	case []map[string]any:
		for _, child := range v {
			if pack := findPluginContextPackDepth(child, depth+1); pack != nil {
				return pack
			}
		}
	}
	return nil
}

func isPluginContextPack(m map[string]any) bool {
	schema := strings.TrimSpace(fmt.Sprint(m["schema_version"]))
	if schema == "plugin_grabber_context_pack.v1" {
		return true
	}
	strategy := strings.TrimSpace(fmt.Sprint(m["context_strategy"]))
	return strategy == "summary_first_full_parameters_available" && !isEmptyValue(m["parameter_count"])
}

func summarizeInheritedSnapshot(snapshot map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"schema_version", "created_at", "conversation_id", "goal_id", "run_id", "goal_summary"} {
		if value, ok := snapshot[key]; ok && !isEmptyValue(value) {
			out[key] = value
		}
	}
	return out
}

func collectImportantFields(out map[string]any, prefix string, value any, depth, limit int) {
	if depth > 4 || len(out) >= limit {
		return
	}
	switch v := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(v) {
			child := v[key]
			if len(out) >= limit {
				return
			}
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if isImportantKey(key) && !isEmptyValue(child) {
				out[path] = compactText(fmt.Sprint(child), 400)
				continue
			}
			collectImportantFields(out, path, child, depth+1, limit)
		}
	case []any:
		for i, child := range v {
			if i >= 5 || len(out) >= limit {
				return
			}
			collectImportantFields(out, fmt.Sprintf("%s[%d]", prefix, i), child, depth+1, limit)
		}
	case []map[string]any:
		for i, child := range v {
			if i >= 5 || len(out) >= limit {
				return
			}
			collectImportantFields(out, fmt.Sprintf("%s[%d]", prefix, i), child, depth+1, limit)
		}
	}
}

func isImportantKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "id" || strings.HasSuffix(k, "_id") || strings.Contains(k, "path") {
		return true
	}
	switch k {
	case "status", "tool", "command_name", "track_name", "clip_name", "plugin_name", "index_path", "built_at", "agent_action_id", "tool_call_id", "goal_id", "run_id":
		return true
	default:
		return false
	}
}

func mapRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if m, ok := row.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func firstNonEmptyMap(row map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if m, ok := row[key].(map[string]any); ok && len(m) > 0 {
			return m
		}
	}
	return nil
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok && !isEmptyValue(value) {
			return value
		}
	}
	return nil
}

func mapText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok && !isEmptyValue(value) {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func contextText(ctx map[string]any, key string) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx[key]; ok && !isEmptyValue(value) {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}

func removeEmpty(row map[string]any) {
	for key, value := range row {
		if isEmptyValue(value) {
			delete(row, key)
		}
	}
}

func isEmptyValue(value any) bool {
	if value == nil {
		return true
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text == "" || text == "<nil>"
}

func stringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func firstStrings(values []string, limit int) []string {
	if limit <= 0 || limit > len(values) {
		limit = len(values)
	}
	out := make([]string, limit)
	copy(out, values[:limit])
	return out
}

func listLen(value any) int {
	switch v := value.(type) {
	case []any:
		return len(v)
	case []map[string]any:
		return len(v)
	case []string:
		return len(v)
	default:
		return 0
	}
}

func sortedKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func firstMessageByRole(messages []llm.Message, role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	for _, msg := range messages {
		if strings.ToLower(strings.TrimSpace(msg.Role)) == role {
			return msg.Content
		}
	}
	return ""
}

func lastMessageByRole(messages []llm.Message, role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.ToLower(strings.TrimSpace(messages[i].Role)) == role {
			return messages[i].Content
		}
	}
	return ""
}

func compactText(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "...<truncated>"
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "<nil>" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
