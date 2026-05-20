package harness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tools"
)

type Harness struct {
	kernel  *kernel.Client
	shadow  *shadow.Project
	catalog *tools.Catalog
	journal *journal.Journal
	logger  *logx.Logger
}

type InvokeRequest struct {
	Tool      string         `json:"tool"`
	Args      map[string]any `json:"args"`
	Command   map[string]any `json:"command"`
	Context   map[string]any `json:"context,omitempty"`
	Source    string         `json:"source"`
	Confirmed bool           `json:"confirmed"`
}

type InvokeResponse struct {
	Status               string          `json:"status"`
	AgentActionID        string          `json:"agent_action_id,omitempty"`
	Tool                 string          `json:"tool,omitempty"`
	CommandName          string          `json:"command_name,omitempty"`
	RiskLevel            tools.RiskLevel `json:"risk_level,omitempty"`
	RequiresConfirmation bool            `json:"requires_confirmation"`
	Preview              string          `json:"preview,omitempty"`
	UndoLabel            string          `json:"undo_label,omitempty"`
	Result               map[string]any  `json:"result,omitempty"`
	Error                string          `json:"error,omitempty"`
}

func New(kernelClient *kernel.Client, shadowProject *shadow.Project, logger *logx.Logger) *Harness {
	return &Harness{
		kernel:  kernelClient,
		shadow:  shadowProject,
		catalog: tools.DefaultCatalog(),
		journal: journal.New(300),
		logger:  logger,
	}
}

func (h *Harness) Tools() []tools.Tool {
	if h == nil || h.catalog == nil {
		return nil
	}
	return h.catalog.Tools()
}

func (h *Harness) Commands() []tools.CommandSpec {
	if h == nil || h.catalog == nil {
		return nil
	}
	return h.catalog.Commands()
}

func (h *Harness) Actions(limit int) []journal.Action {
	if h == nil || h.journal == nil {
		return nil
	}
	return h.journal.Recent(limit)
}

func (h *Harness) DirectCommandNames() []string {
	if h == nil || h.catalog == nil {
		return nil
	}
	return h.catalog.DirectCommandNames()
}

func (h *Harness) StateSummary(ctx context.Context) map[string]any {
	if h == nil || h.shadow == nil {
		return map[string]any{"initialized": false}
	}
	if !h.shadow.Initialized() {
		h.refreshShadow(ctx, "state_summary")
	}
	return h.shadow.Summary()
}

func (h *Harness) UserStateSummary(ctx context.Context) map[string]any {
	return userVisibleState(h.StateSummary(ctx))
}

func (h *Harness) ModelCatalogSummary() string {
	if h == nil || h.catalog == nil {
		return ""
	}
	return h.catalog.ModelSummary()
}

func (h *Harness) Invoke(ctx context.Context, req InvokeRequest) (InvokeResponse, error) {
	if h == nil {
		return InvokeResponse{Status: "error", Error: "harness is nil"}, fmt.Errorf("harness is nil")
	}
	cmd, spec, err := h.resolveCommand(req)
	if err != nil {
		resp := InvokeResponse{Status: "error", Error: err.Error()}
		return resp, err
	}
	if err := h.resolveImplicitTargets(ctx, spec, cmd, req.Context); err != nil {
		resp := InvokeResponse{
			Status:      "error",
			Tool:        spec.ToolName,
			CommandName: spec.CommandName,
			RiskLevel:   spec.RiskLevel,
			Error:       err.Error(),
		}
		return resp, err
	}
	if err := validateRequiredTargetIDs(spec, cmd); err != nil {
		resp := InvokeResponse{
			Status:      "error",
			Tool:        spec.ToolName,
			CommandName: spec.CommandName,
			RiskLevel:   spec.RiskLevel,
			Error:       err.Error(),
		}
		return resp, err
	}

	actionID := "act_" + randomID()
	undoLabel := buildUndoLabel(spec, cmd)
	cmd["agent_action_id"] = actionID
	if undoLabel != "" {
		cmd["undo_label"] = undoLabel
	}

	needsConfirmation := spec.RequiresConfirmation && !req.Confirmed
	action := journal.Action{
		AgentActionID:        actionID,
		Source:               fallback(req.Source, "agent"),
		Summary:              spec.Description,
		Tool:                 spec.ToolName,
		CommandName:          spec.CommandName,
		Command:              cmd,
		RiskLevel:            string(spec.RiskLevel),
		RequiresConfirmation: spec.RequiresConfirmation,
		ConfirmationStatus:   confirmationStatus(needsConfirmation, req.Confirmed),
		Status:               journal.StatusRunning,
		UndoLabel:            undoLabel,
	}
	if needsConfirmation {
		action.Status = journal.StatusPendingConfirmation
		h.journal.Record(action)
		resp := InvokeResponse{
			Status:               "needs_confirmation",
			AgentActionID:        actionID,
			Tool:                 spec.ToolName,
			CommandName:          spec.CommandName,
			RiskLevel:            spec.RiskLevel,
			RequiresConfirmation: true,
			Preview:              PreviewCommand(spec, cmd),
			UndoLabel:            undoLabel,
		}
		return resp, nil
	}

	h.journal.Record(action)
	if result, ok := h.invokeLocal(spec, cmd); ok {
		h.journal.MarkResult(actionID, journal.StatusSucceeded, result, nil)
		resp := InvokeResponse{
			Status:               "ok",
			AgentActionID:        actionID,
			Tool:                 spec.ToolName,
			CommandName:          spec.CommandName,
			RiskLevel:            spec.RiskLevel,
			RequiresConfirmation: false,
			Result:               result,
			UndoLabel:            undoLabel,
		}
		return resp, nil
	}
	if h.kernel == nil {
		err := fmt.Errorf("kernel client is nil")
		h.journal.MarkResult(actionID, journal.StatusFailed, nil, err)
		resp := InvokeResponse{
			Status:        "error",
			AgentActionID: actionID,
			Tool:          spec.ToolName,
			CommandName:   spec.CommandName,
			RiskLevel:     spec.RiskLevel,
			UndoLabel:     undoLabel,
			Error:         err.Error(),
		}
		return resp, err
	}
	reply, _, err := h.kernel.SendCommand(ctx, cmd)
	if err != nil {
		h.journal.MarkResult(actionID, journal.StatusFailed, nil, err)
		resp := InvokeResponse{
			Status:        "error",
			AgentActionID: actionID,
			Tool:          spec.ToolName,
			CommandName:   spec.CommandName,
			RiskLevel:     spec.RiskLevel,
			UndoLabel:     undoLabel,
			Error:         err.Error(),
		}
		return resp, err
	}

	h.afterKernelReply(ctx, spec, reply)
	status := "ok"
	journalStatus := journal.StatusSucceeded
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
		status = "kernel_error"
		journalStatus = journal.StatusFailed
		err = fmt.Errorf("%s", firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), "kernel command failed"))
	}
	h.journal.MarkResult(actionID, journalStatus, reply, err)
	result := h.publicResult(spec, reply)
	resp := InvokeResponse{
		Status:               status,
		AgentActionID:        actionID,
		Tool:                 spec.ToolName,
		CommandName:          spec.CommandName,
		RiskLevel:            spec.RiskLevel,
		RequiresConfirmation: false,
		Result:               result,
		UndoLabel:            undoLabel,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp, err
}

func (h *Harness) resolveCommand(req InvokeRequest) (map[string]any, tools.CommandSpec, error) {
	if h == nil || h.catalog == nil {
		return nil, tools.CommandSpec{}, fmt.Errorf("command catalog is unavailable")
	}
	if len(req.Command) > 0 {
		cmd := tools.CloneCommand(req.Command)
		flattenCommandParams(cmd)
		name := tools.CommandName(cmd)
		if name == "" {
			if toolName := firstString(cmd, "tool"); toolName != "" {
				return h.resolveToolCall(toolName, commandArgs(cmd))
			}
			return nil, tools.CommandSpec{}, fmt.Errorf("command is missing cmd/action/command or tool")
		}
		spec, ok := h.catalog.LookupCommand(name)
		if !ok {
			return nil, tools.CommandSpec{}, fmt.Errorf("unknown or unregistered DAW command: %s", name)
		}
		return cmd, spec, nil
	}

	toolName := strings.TrimSpace(req.Tool)
	if toolName == "" {
		return nil, tools.CommandSpec{}, fmt.Errorf("tool or command is required")
	}
	if toolName == "daw.invoke" {
		cmd := tools.CloneCommand(req.Args)
		flattenCommandParams(cmd)
		name := tools.CommandName(cmd)
		if name == "" {
			return nil, tools.CommandSpec{}, fmt.Errorf("daw.invoke args must include cmd/action/command")
		}
		spec, ok := h.catalog.LookupCommand(name)
		if !ok {
			return nil, tools.CommandSpec{}, fmt.Errorf("unknown or unregistered DAW command: %s", name)
		}
		return cmd, spec, nil
	}

	spec, ok := h.catalog.LookupTool(toolName)
	if !ok {
		return nil, tools.CommandSpec{}, fmt.Errorf("unknown tool: %s", toolName)
	}
	cmd := tools.BuildCommand(spec.CommandName, req.Args)
	flattenCommandParams(cmd)
	return cmd, spec, nil
}

func (h *Harness) resolveToolCall(toolName string, args map[string]any) (map[string]any, tools.CommandSpec, error) {
	toolName = strings.TrimSpace(toolName)
	spec, ok := h.catalog.LookupTool(toolName)
	if !ok {
		return nil, tools.CommandSpec{}, fmt.Errorf("unknown tool: %s", toolName)
	}
	cmd := tools.BuildCommand(spec.CommandName, args)
	flattenCommandParams(cmd)
	return cmd, spec, nil
}

func commandArgs(cmd map[string]any) map[string]any {
	if args, ok := cmd["args"].(map[string]any); ok {
		return tools.CloneCommand(args)
	}
	out := tools.CloneCommand(cmd)
	delete(out, "tool")
	delete(out, "args")
	return out
}

func (h *Harness) invokeLocal(spec tools.CommandSpec, cmd map[string]any) (map[string]any, bool) {
	switch spec.CommandName {
	case "select_clip":
		return map[string]any{
			"status":     "ok",
			"ui_action":  "select_clip",
			"track_id":   firstString(cmd, "track_id"),
			"track_name": firstString(cmd, "track_name"),
			"clip_id":    firstString(cmd, "clip_id"),
			"clip_name":  firstString(cmd, "clip_name", "name"),
		}, true
	default:
		return nil, false
	}
}

func flattenCommandParams(cmd map[string]any) {
	for _, nestedKey := range []string{"params", "args"} {
		params, ok := cmd[nestedKey].(map[string]any)
		if !ok {
			continue
		}
		for k, v := range params {
			if isEmptyValue(cmd[k]) && !isEmptyValue(v) {
				cmd[k] = v
			}
		}
		delete(cmd, nestedKey)
	}
}

func (h *Harness) resolveImplicitTargets(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) error {
	normalizeCommandArgs(spec, cmd, requestContext)

	if err := h.resolveImportAudioSource(ctx, spec, cmd, requestContext); err != nil {
		return fmt.Errorf("%s could not resolve import source: %w", spec.ToolName, err)
	}

	if err := h.resolveClipTargets(ctx, spec, cmd, requestContext); err != nil {
		return fmt.Errorf("%s could not resolve clip target: %w", spec.ToolName, err)
	}

	for _, field := range spec.RequiredTargetIDs {
		if field != "track_id" || !isEmptyValue(cmd[field]) {
			continue
		}
		trackID, err := h.resolveTrackID(ctx, spec, cmd, requestContext)
		if err != nil {
			return fmt.Errorf("%s could not resolve track target: %w", spec.ToolName, err)
		}
		if trackID != "" {
			cmd[field] = trackID
		}
	}
	return nil
}

func normalizeCommandArgs(spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) {
	switch spec.CommandName {
	case "rename_track":
		copyFirstNonEmpty(cmd, "name", "new_name", "track_name")
	case "set_mute":
		copyBoolAlias(cmd, "mute", "muted", "enabled", "value")
		inferBoolFromUserMessage(cmd, "mute", requestContext,
			[]string{"unmute", "取消静音", "解除静音", "取消mute", "取消 mute", "关闭静音"},
			[]string{"mute", "静音"})
	case "set_solo":
		copyBoolAlias(cmd, "solo", "is_solo", "enabled", "value")
		inferBoolFromUserMessage(cmd, "solo", requestContext,
			[]string{"unsolo", "取消独奏", "解除独奏", "取消solo", "取消 solo", "关闭独奏"},
			[]string{"solo", "独奏"})
	case "arm_track":
		copyBoolAlias(cmd, "is_armed", "arm", "armed", "enabled", "value")
		inferBoolFromUserMessage(cmd, "is_armed", requestContext,
			[]string{"disarm", "取消录音准备", "解除录音准备", "取消arm", "取消 arm"},
			[]string{"arm", "录音准备"})
	case "set_click":
		copyBoolAlias(cmd, "enabled", "click", "value")
	case "set_tempo":
		copyFirstNonEmpty(cmd, "bpm", "tempo", "value")
	case "seek":
		copyFirstNonEmpty(cmd, "time", "position_seconds", "seconds", "value")
	case "move_clip":
		copyFirstNonEmpty(cmd, "clip_id", "source_clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "new_start", "new_start_seconds", "new_start_beats", "new_start_beat", "start", "start_seconds", "start_beats", "position_seconds", "time")
		inferTimeUnit(cmd)
	case "resize_clip":
		copyFirstNonEmpty(cmd, "clip_id", "source_clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "new_start", "new_start_seconds", "new_start_beats", "new_start_beat", "start", "start_seconds", "start_beats", "position_seconds")
		copyFirstNonEmpty(cmd, "new_length", "new_length_seconds", "new_length_beats", "new_length_beat", "length", "length_seconds", "length_beats", "duration", "duration_seconds", "duration_beats")
		copyFirstNonEmpty(cmd, "offset_in_source", "offset_in_source_seconds", "offset_in_source_beats", "offset_in_source_beat")
		inferTimeUnit(cmd)
	case "split_clip":
		copyFirstNonEmpty(cmd, "clip_id", "source_clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "split_time", "split_time_seconds", "split_time_beats", "split_time_beat", "position_seconds", "time", "at", "start", "start_seconds", "start_beats")
		inferTimeUnit(cmd)
	case "clone_clip":
		copyFirstNonEmpty(cmd, "source_clip_id", "clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "target_track_id", "track_id")
		copyFirstNonEmpty(cmd, "new_start", "new_start_seconds", "new_start_beats", "new_start_beat", "start", "start_seconds", "start_beats", "position_seconds", "time")
		inferTimeUnit(cmd)
	case "remove_clips":
		normalizeClipIDsArray(cmd)
	case "import_audio":
		copyFirstNonEmpty(cmd, "file_path", "path", "absolute_path", "audio_path", "source_path", "source_file", "selected_library_file_path", "library_file_path")
		copyFirstNonEmpty(cmd, "track_id", "target_track_id", "selected_track_id")
		copyFirstNonEmpty(cmd, "offset_time", "start_time", "start", "start_seconds", "position_seconds", "time")
	case "import_media_to_track":
		copyFirstNonEmpty(cmd, "file_path", "path", "absolute_path", "audio_path", "source_path", "source_file", "selected_library_file_path", "library_file_path")
		copyFirstNonEmpty(cmd, "track_id", "target_track_id", "selected_track_id")
		copyFirstNonEmpty(cmd, "start_time", "offset_time", "start", "start_seconds", "position_seconds", "time")
		if isEmptyValue(cmd["media_type"]) {
			cmd["media_type"] = "audio"
		}
		if isEmptyValue(cmd["mode"]) {
			cmd["mode"] = "non_destructive"
		}
	}
}

func inferTimeUnit(cmd map[string]any) {
	if !isEmptyValue(cmd["time_unit"]) {
		return
	}
	for _, key := range []string{"new_start_beats", "new_start_beat", "start_beats", "new_length_beats", "new_length_beat", "length_beats", "duration_beats", "offset_in_source_beats", "offset_in_source_beat", "split_time_beats", "split_time_beat"} {
		if !isEmptyValue(cmd[key]) {
			cmd["time_unit"] = "beats"
			return
		}
	}
	for _, key := range []string{"new_start", "new_start_seconds", "start", "start_seconds", "position_seconds", "time", "new_length", "new_length_seconds", "length", "length_seconds", "duration", "duration_seconds", "offset_in_source", "offset_in_source_seconds", "split_time", "split_time_seconds"} {
		if !isEmptyValue(cmd[key]) {
			cmd["time_unit"] = "seconds"
			return
		}
	}
}

func normalizeClipIDsArray(cmd map[string]any) {
	if ids := stringSliceFromAny(cmd["clip_ids"]); len(ids) > 0 {
		cmd["clip_ids"] = ids
		return
	}
	if id := firstString(cmd, "clip_id", "source_clip_id", "target_clip_id"); id != "" {
		cmd["clip_ids"] = []string{id}
	}
}

var (
	audioPathPattern       = regexp.MustCompile(`(?i)((?:[a-z]:|\\\\[^\\/]+[\\/][^\\/]+)[\\/][^\r\n"<>|?*]+?\.(?:wav|mp3|flac|ogg|oga|aif|aiff|m4a|wma))`)
	quotedAudioPathPattern = regexp.MustCompile(`(?i)["'“”‘’]([^"'“”‘’]+?\.(?:wav|mp3|flac|ogg|oga|aif|aiff|m4a|wma))["'“”‘’]?`)
	errAudioSearchLimit    = errors.New("audio search limit reached")
)

var audioExtensions = map[string]bool{
	".wav":  true,
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".oga":  true,
	".aif":  true,
	".aiff": true,
	".m4a":  true,
	".wma":  true,
}

func (h *Harness) resolveImportAudioSource(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) error {
	_ = ctx
	if spec.CommandName != "import_audio" && spec.CommandName != "import_media_to_track" {
		return nil
	}

	if isEmptyValue(cmd["file_path"]) {
		if fp := selectedLibraryFilePathFromContext(requestContext); fp != "" {
			cmd["file_path"] = fp
		}
	}
	if isEmptyValue(cmd["file_path"]) {
		if fp := firstAudioPathInText(firstString(requestContext, "user_message", "message", "prompt", "utterance")); fp != "" {
			cmd["file_path"] = fp
		}
	}
	if isEmptyValue(cmd["file_path"]) {
		query := importSearchQuery(cmd, requestContext)
		if query != "" {
			fp, err := findAudioFileInRoots(query, importSearchRoots(cmd, requestContext))
			if err != nil {
				return err
			}
			cmd["file_path"] = fp
		}
	}
	if isEmptyValue(cmd["file_path"]) {
		return fmt.Errorf("missing audio file path; provide an absolute path, select an audio file in the library, or include a library search query")
	}

	resolved, err := resolveExistingAudioPath(firstString(cmd, "file_path"), cmd, requestContext)
	if err != nil {
		return err
	}
	cmd["file_path"] = resolved

	if spec.CommandName == "import_media_to_track" {
		if isEmptyValue(cmd["media_type"]) {
			cmd["media_type"] = "audio"
		}
		if isEmptyValue(cmd["mode"]) {
			cmd["mode"] = "non_destructive"
		}
		if isEmptyValue(cmd["start_time"]) {
			cmd["start_time"] = 0.0
		}
	}
	return nil
}

func selectedLibraryFilePathFromContext(requestContext map[string]any) string {
	if fp := firstString(requestContext, "selected_library_file_path", "library_file_path"); fp != "" {
		return fp
	}
	for _, key := range []string{"selected_library_item", "selected_library_resource", "library_item"} {
		item, ok := requestContext[key].(map[string]any)
		if !ok {
			continue
		}
		if fp := firstString(item, "file_path", "path"); fp != "" {
			return fp
		}
	}
	return ""
}

func resolveExistingAudioPath(raw string, cmd map[string]any, requestContext map[string]any) (string, error) {
	path := normalizeAudioPath(raw)
	if path == "" {
		return "", fmt.Errorf("audio file path is empty")
	}
	if !isSupportedAudioPath(path) {
		return "", fmt.Errorf("unsupported audio file extension: %s", path)
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		abs, absErr := filepath.Abs(path)
		if absErr == nil {
			return filepath.Clean(abs), nil
		}
		return filepath.Clean(path), nil
	}
	if !filepath.IsAbs(path) {
		if found, err := findAudioFileInRoots(path, importSearchRoots(cmd, requestContext)); err == nil {
			return found, nil
		}
	}
	return "", fmt.Errorf("audio file does not exist: %s", path)
}

func normalizeAudioPath(path string) string {
	path = trimPathPunctuation(path)
	if path == "" {
		return ""
	}
	path = filepath.FromSlash(path)
	return filepath.Clean(path)
}

func trimPathPunctuation(path string) string {
	return strings.Trim(path, " \t\r\n\"'`“”‘’.,，。;；:：)）]】")
}

func isSupportedAudioPath(path string) bool {
	return audioExtensions[strings.ToLower(filepath.Ext(path))]
}

func firstAudioPathInText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for _, pattern := range []*regexp.Regexp{quotedAudioPathPattern, audioPathPattern} {
		match := pattern.FindStringSubmatch(text)
		if len(match) > 1 {
			return trimPathPunctuation(match[1])
		}
	}
	return ""
}

func importSearchQuery(cmd map[string]any, requestContext map[string]any) string {
	if q := firstString(cmd, "asset_query", "search_query", "file_query", "query", "search", "file_name"); q != "" {
		return q
	}
	return inferImportSearchQuery(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
}

func inferImportSearchQuery(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || firstAudioPathInText(text) != "" {
		return ""
	}
	if !containsTextAnyFold(text, "搜索", "查找", "找", "search", "find") {
		return ""
	}
	cleaned := text
	for _, phrase := range []string{
		"搜索", "查找", "找一下", "找到", "找", "并导入", "然后导入", "导入", "放到", "放进", "拖入",
		"资料库", "素材库", "当前轨道", "选中轨道", "这个轨道", "这条轨道", "音频", "素材",
		"search", "find", "import", "add", "audio", "sample", "current track", "selected track", "track", "and", "to",
	} {
		cleaned = strings.ReplaceAll(cleaned, phrase, " ")
		cleaned = strings.ReplaceAll(cleaned, strings.Title(phrase), " ")
	}
	replacer := strings.NewReplacer("，", " ", "。", " ", ",", " ", ".", " ", "；", " ", ";", " ", "：", " ", ":", " ", "（", " ", "）", " ", "(", " ", ")", " ", "并", " ", "到", " ", "给", " ")
	cleaned = replacer.Replace(cleaned)
	return strings.Join(strings.Fields(cleaned), " ")
}

func importSearchRoots(cmd map[string]any, requestContext map[string]any) []string {
	var roots []string
	for _, key := range []string{"search_roots", "library_places", "library_roots", "places"} {
		roots = append(roots, stringSliceFromAny(cmd[key])...)
		roots = append(roots, stringSliceFromAny(requestContext[key])...)
	}
	for _, key := range []string{"selected_library_root", "library_root"} {
		if root := firstString(cmd, key); root != "" {
			roots = append(roots, root)
		}
		if root := firstString(requestContext, key); root != "" {
			roots = append(roots, root)
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		root = normalizeAudioPath(root)
		if root == "" || seen[strings.ToLower(root)] {
			continue
		}
		seen[strings.ToLower(root)] = true
		out = append(out, root)
	}
	return out
}

type audioFileMatch struct {
	path  string
	score int
}

func findAudioFileInRoots(query string, roots []string) (string, error) {
	terms := importSearchTerms(query)
	if len(terms) == 0 {
		return "", fmt.Errorf("empty audio search query")
	}
	if len(roots) == 0 {
		return "", fmt.Errorf("library search needs at least one Places root")
	}

	matches := make([]audioFileMatch, 0)
	const maxScannedFiles = 6000
	scanned := 0
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		root = filepath.Clean(root)
		scanErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d == nil {
				return nil
			}
			if d.IsDir() {
				if path != root && strings.HasPrefix(filepath.Base(path), ".") {
					return filepath.SkipDir
				}
				if directoryDepth(root, path) > 8 {
					return filepath.SkipDir
				}
				return nil
			}
			scanned++
			if scanned > maxScannedFiles {
				return errAudioSearchLimit
			}
			if !isSupportedAudioPath(path) {
				return nil
			}
			if score := audioSearchScore(filepath.Base(path), terms); score > 0 {
				matches = append(matches, audioFileMatch{path: filepath.Clean(path), score: score})
			}
			return nil
		})
		if scanErr != nil && !errors.Is(scanErr, errAudioSearchLimit) {
			return "", scanErr
		}
		if errors.Is(scanErr, errAudioSearchLimit) {
			break
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no audio file matched %q in library Places", query)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return len(matches[i].path) < len(matches[j].path)
	})
	if len(matches) > 1 && matches[0].score == matches[1].score {
		return "", fmt.Errorf("multiple audio files matched %q; specify one path, e.g. %s or %s", query, matches[0].path, matches[1].path)
	}
	return matches[0].path, nil
}

func importSearchTerms(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	query = strings.TrimSuffix(query, strings.ToLower(filepath.Ext(query)))
	replacer := strings.NewReplacer("_", " ", "-", " ", ".", " ", ",", " ", "，", " ", "。", " ", "/", " ", "\\", " ")
	query = replacer.Replace(query)
	fields := strings.Fields(query)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func audioSearchScore(fileName string, terms []string) int {
	name := strings.ToLower(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
	score := 0
	for _, term := range terms {
		if !strings.Contains(name, term) {
			return 0
		}
		score += 10 + len(term)
		if name == term {
			score += 50
		}
	}
	joined := strings.Join(terms, " ")
	if name == joined {
		score += 100
	} else if strings.HasPrefix(name, joined) {
		score += 30
	}
	return score
}

func directoryDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return len(strings.Split(rel, string(os.PathSeparator)))
}

func inferBoolFromUserMessage(cmd map[string]any, target string, requestContext map[string]any, negativePhrases, positivePhrases []string) {
	if _, ok := boolValue(cmd[target]); ok {
		return
	}
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	if text == "" {
		return
	}
	if value, ok := inferBoolFromText(text, negativePhrases, positivePhrases); ok {
		cmd[target] = value
	}
}

func inferBoolFromText(text string, negativePhrases, positivePhrases []string) (bool, bool) {
	lower := strings.ToLower(text)
	stripped := lower
	hasNegative := false
	for _, phrase := range negativePhrases {
		p := strings.ToLower(strings.TrimSpace(phrase))
		if p == "" {
			continue
		}
		if strings.Contains(lower, p) {
			hasNegative = true
			stripped = strings.ReplaceAll(stripped, p, " ")
		}
	}
	hasPositive := false
	for _, phrase := range positivePhrases {
		p := strings.ToLower(strings.TrimSpace(phrase))
		if p != "" && strings.Contains(stripped, p) {
			hasPositive = true
			break
		}
	}
	if hasPositive && hasNegative {
		return false, false
	}
	if hasPositive {
		return true, true
	}
	if hasNegative {
		return false, true
	}
	return false, false
}

func copyFirstNonEmpty(cmd map[string]any, target string, aliases ...string) {
	if !isEmptyValue(cmd[target]) {
		return
	}
	for _, key := range aliases {
		if !isEmptyValue(cmd[key]) {
			cmd[target] = cmd[key]
			return
		}
	}
}

func copyBoolAlias(cmd map[string]any, target string, aliases ...string) {
	if value, ok := boolValue(cmd[target]); ok {
		cmd[target] = value
		return
	}
	for _, key := range aliases {
		if value, ok := boolValue(cmd[key]); ok {
			cmd[target] = value
			return
		}
	}
}

func boolValue(v any) (bool, bool) {
	if isEmptyValue(v) {
		return false, false
	}
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "1", "yes", "on", "enable", "enabled":
			return true, true
		case "false", "0", "no", "off", "disable", "disabled":
			return false, true
		}
	case int:
		return x != 0, true
	case int64:
		return x != 0, true
	case float64:
		return x != 0, true
	case json.Number:
		n, err := strconv.Atoi(x.String())
		if err == nil {
			return n != 0, true
		}
	}
	return false, false
}

type clipRef struct {
	ID             string
	Name           string
	TrackID        string
	TrackName      string
	UserTrackIndex int
	Row            map[string]any
}

func (h *Harness) resolveClipTargets(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) error {
	switch spec.CommandName {
	case "move_clip":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_id"] = ref.ID
		if isEmptyValue(cmd["new_start"]) {
			if seconds, ok := inferMoveStartSeconds(ref, requestContext); ok {
				cmd["new_start"] = seconds
				if isEmptyValue(cmd["time_unit"]) {
					cmd["time_unit"] = "seconds"
				}
			}
		}
		if isEmptyValue(cmd["source_track_id"]) {
			cmd["source_track_id"] = ref.TrackID
		}
		if isEmptyValue(cmd["target_track_id"]) {
			targetTrackID, err := h.resolveTargetTrackID(ctx, spec, cmd, requestContext)
			if err != nil {
				return err
			}
			if targetTrackID == "" {
				targetTrackID = ref.TrackID
			}
			cmd["target_track_id"] = targetTrackID
		}
	case "resize_clip":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_id"] = ref.ID
		if isEmptyValue(cmd["new_length"]) {
			if seconds, ok := inferLengthSeconds(requestContext); ok {
				cmd["new_length"] = seconds
				if isEmptyValue(cmd["time_unit"]) {
					cmd["time_unit"] = "seconds"
				}
			}
		}
		if isEmptyValue(cmd["track_id"]) && ref.TrackID != "" {
			cmd["track_id"] = ref.TrackID
		}
	case "split_clip":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_id"] = ref.ID
		if mentionsPlayheadSplit(firstString(requestContext, "user_message", "message", "prompt", "utterance")) {
			if seconds, ok := firstPlayheadSeconds(cmd, requestContext); ok {
				cmd["split_time"] = seconds
				if isEmptyValue(cmd["time_unit"]) {
					cmd["time_unit"] = "seconds"
				}
			}
		}
		if isEmptyValue(cmd["split_time"]) {
			if seconds, ok := inferSplitTimeSeconds(ref, requestContext); ok {
				cmd["split_time"] = seconds
				if isEmptyValue(cmd["time_unit"]) {
					cmd["time_unit"] = "seconds"
				}
			}
		}
		if isEmptyValue(cmd["track_id"]) && ref.TrackID != "" {
			cmd["track_id"] = ref.TrackID
		}
		if isEmptyValue(cmd["split_time"]) {
			return fmt.Errorf("split_clip requires split_time; say a time like 在 2 秒处切开, or move the playhead and say 在播放头切开")
		}
		if isEmptyValue(cmd["time_unit"]) {
			cmd["time_unit"] = "seconds"
		}
	case "clone_clip":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "source_clip_id")
		if err != nil {
			return err
		}
		cmd["source_clip_id"] = ref.ID
		if isEmptyValue(cmd["new_start"]) {
			if seconds, ok := inferCloneStartSeconds(ref, requestContext); ok {
				cmd["new_start"] = seconds
				if isEmptyValue(cmd["time_unit"]) {
					cmd["time_unit"] = "seconds"
				}
			}
		}
		if isEmptyValue(cmd["target_track_id"]) {
			targetTrackID, err := h.resolveTargetTrackID(ctx, spec, cmd, requestContext)
			if err != nil {
				return err
			}
			if targetTrackID == "" {
				targetTrackID = ref.TrackID
			}
			cmd["target_track_id"] = targetTrackID
		}
		if isEmptyValue(cmd["new_start"]) {
			return fmt.Errorf("clone_clip requires new_start; say where to place the copy, e.g. 复制到 20 秒")
		}
		if isEmptyValue(cmd["time_unit"]) {
			cmd["time_unit"] = "seconds"
		}
	case "remove_clips":
		if ids := stringSliceFromAny(cmd["clip_ids"]); len(ids) > 0 {
			cmd["clip_ids"] = ids
			return nil
		}
		if ids := selectedClipIDsFromContext(requestContext); len(ids) > 0 {
			cmd["clip_ids"] = ids
			return nil
		}
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_ids"] = []string{ref.ID}
	case "select_clip":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_id"] = ref.ID
		if isEmptyValue(cmd["track_id"]) && ref.TrackID != "" {
			cmd["track_id"] = ref.TrackID
		}
		if isEmptyValue(cmd["clip_name"]) && ref.Name != "" {
			cmd["clip_name"] = ref.Name
		}
		if isEmptyValue(cmd["track_name"]) && ref.TrackName != "" {
			cmd["track_name"] = ref.TrackName
		}
	default:
		if specRequiresTarget(spec, "clip_id") && isEmptyValue(cmd["clip_id"]) {
			ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
			if err != nil {
				return err
			}
			cmd["clip_id"] = ref.ID
			if specRequiresTarget(spec, "track_id") && isEmptyValue(cmd["track_id"]) && ref.TrackID != "" {
				cmd["track_id"] = ref.TrackID
			}
		}
	}
	return nil
}

func (h *Harness) resolveTargetTrackID(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) (string, error) {
	if targetTrackID := firstString(cmd, "target_track_id"); targetTrackID != "" {
		return targetTrackID, nil
	}
	if trackID := firstString(cmd, "track_id"); trackID != "" {
		return trackID, nil
	}
	if hasTargetTrackHint(cmd) {
		return h.resolveTrackID(ctx, spec, cmd, requestContext)
	}
	return "", nil
}

func hasTargetTrackHint(cmd map[string]any) bool {
	if firstString(cmd, "target_track_name", "target_track") != "" {
		return true
	}
	_, ok := firstPositiveInt(cmd, "target_track_index", "target_track_number", "target_user_track_index")
	return ok
}

func (h *Harness) resolveSingleClipRef(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any, field string) (clipRef, error) {
	state := h.UserStateSummary(ctx)
	refs := visibleClipRefs(state)
	if id := firstString(cmd, field, "clip_id", "source_clip_id", "target_clip_id"); id != "" {
		return clipRefByIDOrContext(id, refs, requestContext)
	}

	selected := selectedClipIDsFromContext(requestContext)
	if len(selected) == 1 {
		return clipRefByIDOrContext(selected[0], refs, requestContext)
	}
	if len(selected) > 1 {
		return clipRef{}, fmt.Errorf("multiple clips are selected; specify which clip or use a multi-clip command")
	}

	if name := firstString(cmd, "clip_name", "target_clip_name", "source_clip_name", "clip"); name != "" {
		matches := filterClipRefsByName(refs, name)
		switch len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			return clipRef{}, fmt.Errorf("no user-visible clip named %q", name)
		default:
			return clipRef{}, fmt.Errorf("multiple user-visible clips named %q; specify track or clip_index", name)
		}
	}

	if index, ok := firstPositiveInt(cmd, "clip_index", "clip_number", "user_clip_index"); ok {
		scope := filterClipRefsByTrack(refs, clipScopeTrackID(ctx, cmd, requestContext))
		if len(scope) == 0 {
			scope = refs
		}
		if index < 1 || index > len(scope) {
			return clipRef{}, fmt.Errorf("clip_index %d is out of range", index)
		}
		return scope[index-1], nil
	}

	if trackID := clipScopeTrackID(ctx, cmd, requestContext); trackID != "" {
		scope := filterClipRefsByTrack(refs, trackID)
		if len(scope) == 1 {
			return scope[0], nil
		}
		if len(scope) > 1 {
			return clipRef{}, fmt.Errorf("multiple clips exist on the selected track; select a clip or specify clip_index")
		}
	}

	switch len(refs) {
	case 0:
		return clipRef{}, fmt.Errorf("there are no user-visible clips")
	case 1:
		return refs[0], nil
	default:
		return clipRef{}, fmt.Errorf("multiple user-visible clips exist; select a clip or specify clip name/clip_index")
	}
}

func clipRefByIDOrContext(id string, refs []clipRef, requestContext map[string]any) (clipRef, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return clipRef{}, fmt.Errorf("clip_id is empty")
	}
	for _, ref := range refs {
		if ref.ID == id {
			return ref, nil
		}
	}
	if trackID := firstString(requestContext, "selected_clip_track_id", "focused_track_id", "selected_track_id", "track_id"); trackID != "" {
		return clipRef{ID: id, TrackID: trackID}, nil
	}
	return clipRef{}, fmt.Errorf("clip_id %q is not present in the current user-visible project state", id)
}

func clipScopeTrackID(ctx context.Context, cmd map[string]any, requestContext map[string]any) string {
	if trackID := firstString(cmd, "track_id", "source_track_id", "target_track_id"); trackID != "" {
		return trackID
	}
	if trackID := firstString(requestContext, "selected_clip_track_id", "focused_track_id", "selected_track_id", "track_id"); trackID != "" {
		return trackID
	}
	_ = ctx
	return ""
}

func visibleClipRefs(state map[string]any) []clipRef {
	tracks := visibleTrackRows(state)
	out := make([]clipRef, 0)
	for _, track := range tracks {
		trackID := visibleTrackID(track)
		trackName := visibleTrackName(track)
		userIndex, _ := firstPositiveInt(track, "user_track_index")
		for _, clip := range mapRowsFromAny(track["clips"]) {
			id := firstString(clip, "clip_id", "id", "item_id")
			if id == "" {
				continue
			}
			out = append(out, clipRef{
				ID:             id,
				Name:           firstString(clip, "name", "clip_name"),
				TrackID:        trackID,
				TrackName:      trackName,
				UserTrackIndex: userIndex,
				Row:            clip,
			})
		}
	}
	return out
}

func filterClipRefsByName(refs []clipRef, name string) []clipRef {
	name = strings.TrimSpace(name)
	out := make([]clipRef, 0, len(refs))
	for _, ref := range refs {
		if strings.EqualFold(ref.Name, name) {
			out = append(out, ref)
		}
	}
	return out
}

func filterClipRefsByTrack(refs []clipRef, trackID string) []clipRef {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return refs
	}
	out := make([]clipRef, 0, len(refs))
	for _, ref := range refs {
		if ref.TrackID == trackID {
			out = append(out, ref)
		}
	}
	return out
}

func selectedClipIDsFromContext(requestContext map[string]any) []string {
	if len(requestContext) == 0 {
		return nil
	}
	if ids := stringSliceFromAny(requestContext["selected_clip_ids"]); len(ids) > 0 {
		return ids
	}
	if id := firstString(requestContext, "selected_clip_id", "primary_selected_clip_id", "clip_id"); id != "" {
		return []string{id}
	}
	return nil
}

func specRequiresTarget(spec tools.CommandSpec, field string) bool {
	for _, id := range spec.RequiredTargetIDs {
		if id == field {
			return true
		}
	}
	return false
}

var secondsInTextPattern = regexp.MustCompile(`(?i)([-+]?\d+(?:\.\d+)?)\s*(?:s|sec|secs|second|seconds|秒)`)

func inferMoveStartSeconds(ref clipRef, requestContext map[string]any) (float64, bool) {
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	seconds, ok := firstSecondsInText(text)
	if !ok {
		return 0, false
	}
	lower := strings.ToLower(text)
	current := numberFromAny(ref.Row["start_seconds"])
	switch {
	case strings.Contains(text, "向前") || strings.Contains(text, "前移") || strings.Contains(text, "提前") || strings.Contains(lower, "earlier") || strings.Contains(lower, "left") || strings.Contains(lower, "backward"):
		next := current - seconds
		if next < 0 {
			next = 0
		}
		return next, true
	case strings.Contains(text, "向后") || strings.Contains(text, "后移") || strings.Contains(text, "推后") || strings.Contains(lower, "later") || strings.Contains(lower, "right") || strings.Contains(lower, "forward"):
		return current + seconds, true
	default:
		return seconds, true
	}
}

func inferCloneStartSeconds(ref clipRef, requestContext map[string]any) (float64, bool) {
	if seconds, ok := inferMoveStartSeconds(ref, requestContext); ok {
		return seconds, true
	}
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	if strings.TrimSpace(text) == "" || isCloneAfterText(text) || containsTextAnyFold(text, "复制", "克隆", "拷贝", "duplicate", "clone", "copy") {
		start := numberFromAny(ref.Row["start_seconds"])
		length := numberFromAny(ref.Row["length_seconds"])
		if length <= 0 {
			end := numberFromAny(ref.Row["end_seconds"])
			if end > start {
				length = end - start
			}
		}
		if length > 0 {
			return start + length, true
		}
	}
	return 0, false
}

func inferSplitTimeSeconds(ref clipRef, requestContext map[string]any) (float64, bool) {
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	if seconds, ok := firstSecondsInText(text); ok {
		return splitTimeFromTextSeconds(ref, seconds, text), true
	}
	if mentionsPlayheadSplit(text) {
		if seconds, ok := firstPlayheadSeconds(nil, requestContext); ok {
			return seconds, true
		}
	}
	return 0, false
}

func firstPlayheadSeconds(cmd map[string]any, requestContext map[string]any) (float64, bool) {
	keys := []string{"playhead_seconds", "current_playhead_seconds", "transport_position_seconds", "position_seconds"}
	if seconds, ok := numberValueFromMap(cmd, keys...); ok {
		return seconds, true
	}
	return numberValueFromMap(requestContext, keys...)
}

func splitTimeFromTextSeconds(ref clipRef, seconds float64, text string) float64 {
	lower := strings.ToLower(text)
	if containsTextAnyFold(text, "内部", "里面", "从开头", "从起点", "相对", "offset", "into clip", "from start") {
		return numberFromAny(ref.Row["start_seconds"]) + seconds
	}
	if containsTextAnyFold(text, "后", "之后", "later", "after") && !containsTextAnyFold(text, "秒处", "s mark", "at") {
		return numberFromAny(ref.Row["start_seconds"]) + seconds
	}
	if strings.Contains(lower, "relative") {
		return numberFromAny(ref.Row["start_seconds"]) + seconds
	}
	return seconds
}

func mentionsPlayheadSplit(text string) bool {
	return containsTextAnyFold(text, "播放头", "当前位置", "这里", "此处", "当前时间", "playhead", "cursor", "current position", "here")
}

func isCloneAfterText(text string) bool {
	return containsTextAnyFold(text, "后面", "后边", "后方", "后面一份", "后续", "紧接", "后", "after", "next", "right after")
}

func inferLengthSeconds(requestContext map[string]any) (float64, bool) {
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	if !isResizeLengthText(text) {
		return 0, false
	}
	return firstSecondsInText(text)
}

func isResizeLengthText(text string) bool {
	if containsTextAnyFold(text, "位置", "起点", "开始位置", "移动", "移到", "挪到", "拖到", "move", "position", "start") {
		return false
	}
	return containsTextAnyFold(text,
		"长度", "时长", "持续", "裁到", "裁成", "剪到", "剪成", "剪短",
		"缩到", "缩短", "拉长", "拉到", "伸到", "改成", "改为", "变成",
		"调整到", "调成", "resize", "length", "duration", "trim", "shorten")
}

func containsTextAnyFold(text string, needles ...string) bool {
	lower := strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func firstSecondsInText(text string) (float64, bool) {
	match := secondsInTextPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, false
	}
	if value < 0 {
		value = -value
	}
	return value, true
}

func numberFromAny(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case json.Number:
		n, _ := x.Float64()
		return n
	default:
		n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
		return n
	}
}

func numberValueFromMap(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		v, ok := row[key]
		if !ok || isEmptyValue(v) {
			continue
		}
		switch x := v.(type) {
		case int:
			return float64(x), true
		case int64:
			return float64(x), true
		case float64:
			return x, true
		case json.Number:
			n, err := x.Float64()
			return n, err == nil
		default:
			n, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
			return n, err == nil
		}
	}
	return 0, false
}

func (h *Harness) resolveTrackID(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) (string, error) {
	if h == nil || h.shadow == nil {
		return "", nil
	}
	state := h.UserStateSummary(ctx)
	tracks := visibleTrackRows(state)
	if index, ok := firstPositiveInt(cmd, "user_track_index", "track_index", "track_number", "target_track_index", "target_track_number", "target_user_track_index", "index"); ok {
		if index < 1 || index > len(tracks) {
			return "", fmt.Errorf("user_track_index %d is out of range", index)
		}
		return visibleTrackID(tracks[index-1]), nil
	}

	nameKeys := []string{"target_track_name", "target_track", "track"}
	if spec.CommandName != "rename_track" {
		nameKeys = append(nameKeys, "track_name")
	}
	if name := firstString(cmd, nameKeys...); name != "" {
		for _, row := range tracks {
			if strings.EqualFold(visibleTrackName(row), name) {
				return visibleTrackID(row), nil
			}
		}
		return "", fmt.Errorf("no user-visible track named %q", name)
	}
	if trackID, ok := trackIDFromContext(tracks, requestContext); ok {
		return trackID, nil
	}

	switch len(tracks) {
	case 0:
		return "", fmt.Errorf("there are no user-visible tracks")
	case 1:
		return visibleTrackID(tracks[0]), nil
	default:
		return "", fmt.Errorf("multiple user-visible tracks exist; specify track name or user_track_index")
	}
}

func trackIDFromContext(tracks []map[string]any, requestContext map[string]any) (string, bool) {
	if len(tracks) == 0 || len(requestContext) == 0 {
		return "", false
	}
	if selectedID := firstString(requestContext, "selected_track_id", "track_id"); selectedID != "" {
		for _, row := range tracks {
			if visibleTrackID(row) == selectedID {
				return selectedID, true
			}
		}
	}
	if selectedName := firstString(requestContext, "selected_track_name", "track_name", "name"); selectedName != "" {
		for _, row := range tracks {
			if strings.EqualFold(visibleTrackName(row), selectedName) {
				return visibleTrackID(row), true
			}
		}
	}
	return "", false
}

func (h *Harness) afterKernelReply(ctx context.Context, spec tools.CommandSpec, reply map[string]any) {
	if !kernelReplySucceeded(reply) {
		return
	}
	if spec.CommandName == "get_project_state" {
		if h.shadow != nil {
			h.shadow.Initialize(reply)
		}
		return
	}
	if spec.RefreshAfter {
		h.refreshShadow(ctx, spec.CommandName)
	}
}

func kernelReplySucceeded(reply map[string]any) bool {
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(reply["status"])))
	return status == "ok" || status == "success"
}

func (h *Harness) publicResult(spec tools.CommandSpec, reply map[string]any) map[string]any {
	switch spec.CommandName {
	case "get_project_state":
		if h.shadow != nil {
			return withOKStatus(userVisibleState(h.shadow.Summary()))
		}
	case "list_tracks":
		if h.shadow != nil {
			summary := h.shadow.Summary()
			return map[string]any{
				"status":           "ok",
				"track_count":      summary["track_count"],
				"user_track_count": summary["user_track_count"],
				"tracks":           summary["tracks"],
			}
		}
	}
	return reply
}

func userVisibleState(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch k {
		case "engine_track_count", "internal_track_count":
			continue
		default:
			out[k] = v
		}
	}
	return out
}

func withOKStatus(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	out["status"] = "ok"
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (h *Harness) refreshShadow(ctx context.Context, reason string) {
	if h.kernel == nil || h.shadow == nil {
		return
	}
	reply, _, err := h.kernel.SendCommand(ctx, map[string]any{"cmd": "get_project_state"})
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("[harness] shadow refresh after %s failed: %v", reason, err)
		}
		return
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "ok") {
		h.shadow.Initialize(reply)
	}
}

func PreviewCommand(spec tools.CommandSpec, cmd map[string]any) string {
	raw, _ := json.Marshal(cmd)
	return fmt.Sprintf("%s [%s]: %s\n%s", spec.CommandName, spec.RiskLevel, spec.Description, string(raw))
}

func validateRequiredTargetIDs(spec tools.CommandSpec, cmd map[string]any) error {
	for _, field := range spec.RequiredTargetIDs {
		if strings.TrimSpace(field) == "" {
			continue
		}
		v, ok := cmd[field]
		if !ok || isEmptyValue(v) {
			return fmt.Errorf("%s requires non-empty %s; use stable IDs from /agent/state user-visible tracks", spec.ToolName, field)
		}
	}
	return nil
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s == "" || s == "<nil>"
}

func visibleTrackRows(state map[string]any) []map[string]any {
	switch rows := state["tracks"].(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, it := range rows {
			if row, ok := it.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func mapRowsFromAny(v any) []map[string]any {
	switch rows := v.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, it := range rows {
			if row, ok := it.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func visibleTrackID(row map[string]any) string {
	if id := firstString(row, "track_id", "id"); id != "" {
		return id
	}
	return ""
}

func visibleTrackName(row map[string]any) string {
	if name := firstString(row, "name", "track_name"); name != "" {
		return name
	}
	return visibleTrackID(row)
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok && !isEmptyValue(v) {
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}

func stringSliceFromAny(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case []string:
		out := make([]string, 0, len(x))
		for _, it := range x {
			s := strings.TrimSpace(it)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, it := range x {
			s := strings.TrimSpace(fmt.Sprint(it))
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(x))
		for key, enabled := range x {
			if value, ok := boolValue(enabled); ok && !value {
				continue
			}
			s := strings.TrimSpace(key)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "<nil>" {
			return nil
		}
		return []string{s}
	}
}

func firstPositiveInt(row map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		v, ok := row[key]
		if !ok || isEmptyValue(v) {
			continue
		}
		switch x := v.(type) {
		case int:
			return x, x > 0
		case int64:
			return int(x), x > 0
		case float64:
			n := int(x)
			return n, n > 0
		case json.Number:
			n, err := strconv.Atoi(x.String())
			return n, err == nil && n > 0
		default:
			n, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
			return n, err == nil && n > 0
		}
	}
	return 0, false
}

func buildUndoLabel(spec tools.CommandSpec, cmd map[string]any) string {
	if !spec.SupportsUndo {
		return ""
	}
	if v := strings.TrimSpace(fmt.Sprint(cmd["undo_label"])); v != "" && v != "<nil>" {
		return v
	}
	switch {
	case spec.MutatesProject:
		return "Agent: " + spec.CommandName
	default:
		return ""
	}
}

func confirmationStatus(needsConfirmation, confirmed bool) string {
	switch {
	case needsConfirmation:
		return "pending"
	case confirmed:
		return "confirmed"
	default:
		return "not_required"
	}
}

func fallback(v, fb string) string {
	if strings.TrimSpace(v) == "" {
		return fb
	}
	return strings.TrimSpace(v)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && v != "<nil>" {
			return v
		}
	}
	return ""
}

func randomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
