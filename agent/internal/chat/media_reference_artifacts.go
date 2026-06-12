package chat

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/artifacts"
	"vit-daw-agent/internal/resourceintake"
)

var mediaReferenceFilePattern = regexp.MustCompile(`(?i)[^\\/:*?"<>|\r\n]+?\.(wav|mp3|flac|aif|aiff|ogg|oga|m4a|wma|mp4|mov|mkv|webm|avi|m4v|png|jpg|jpeg|gif|webp|bmp|tif|tiff|pdf|txt|md|rtf|html|htm|doc|docx|mid|midi)\b`)

func (s *Server) artifactsFromDialogueMediaReferences(userText, reply, conversationID, goalID, runID string, scope artifacts.Scope) []artifacts.Summary {
	if s == nil {
		return nil
	}
	combined := strings.TrimSpace(userText + "\n" + reply)
	if combined == "" {
		return nil
	}
	started := time.Now()
	store := s.artifactStore()
	args := mapWithArtifactScope(map[string]any{
		"conversation_id": conversationID,
		"goal_id":         goalID,
		"run_id":          runID,
		"source":          "dialogue_media_reference",
		"limit":           80,
	}, scope)

	var out []artifacts.Summary
	userPaths := existingLocalPathsFromText(userText)
	replyPaths := existingLocalPathsFromText(reply)
	replyNames := previewableFileNamesFromText(reply)
	allPaths := uniqueStrings(append(append([]string{}, userPaths...), replyPaths...))
	var filePaths []string
	var folderPaths []string
	for _, path := range allPaths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			folderPaths = append(folderPaths, path)
			continue
		}
		if previewablePath(path) {
			filePaths = append(filePaths, path)
		}
	}
	if len(filePaths) > 0 {
		fileArgs := cloneStringAnyMap(args)
		fileArgs["file_paths"] = filePaths
		if result, err := resourceintake.RegisterAssets(store, fileArgs); err == nil {
			out = mergeArtifactSummaries(out, artifactSummariesFromCommandResult(result))
		} else if s.logger != nil {
			s.logger.Warn("[media_refs] register_assets_failed conversation=%s files=%d err=%s", conversationID, len(filePaths), err)
		}
	}

	mentionsMedia := dialogueMentionsMedia(userText, reply)
	if len(folderPaths) == 0 || !mentionsMedia {
		if s.logger != nil {
			s.logger.Info("[media_refs] ms=%d conversation=%s user_paths=%d reply_names=%d files=%d folders=%d mentions_media=%t artifacts=%d",
				time.Since(started).Milliseconds(), conversationID, len(userPaths), len(replyNames), len(filePaths), len(folderPaths), mentionsMedia, len(out))
		}
		return out
	}
	for _, folder := range folderPaths {
		var matched []string
		for _, name := range replyNames {
			path := filepath.Join(folder, name)
			if st, err := os.Stat(path); err == nil && !st.IsDir() && previewablePath(path) {
				matched = append(matched, path)
			}
		}
		if len(matched) > 0 {
			fileArgs := cloneStringAnyMap(args)
			fileArgs["file_paths"] = uniqueStrings(matched)
			if result, err := resourceintake.RegisterAssets(store, fileArgs); err == nil {
				out = mergeArtifactSummaries(out, artifactSummariesFromCommandResult(result))
			} else if s.logger != nil {
				s.logger.Warn("[media_refs] register_matched_failed conversation=%s folder=%s matched=%d err=%s", conversationID, filepath.Base(folder), len(matched), err)
			}
			continue
		}
		indexArgs := cloneStringAnyMap(args)
		indexArgs["asset_location"] = folder
		indexArgs["media_kinds"] = mediaKindsFromDialogue(userText + "\n" + reply)
		indexArgs["recursive"] = dialogueAsksRecursive(userText)
		if result, err := resourceintake.IndexAuthorizedFolder(store, indexArgs); err == nil {
			out = mergeArtifactSummaries(out, artifactSummariesFromCommandResult(result))
		} else if s.logger != nil {
			s.logger.Warn("[media_refs] index_folder_failed conversation=%s folder=%s err=%s", conversationID, filepath.Base(folder), err)
		}
	}
	if s.logger != nil {
		s.logger.Info("[media_refs] ms=%d conversation=%s user_paths=%d reply_names=%d files=%d folders=%d mentions_media=%t artifacts=%d",
			time.Since(started).Milliseconds(), conversationID, len(userPaths), len(replyNames), len(filePaths), len(folderPaths), mentionsMedia, len(out))
	}
	return out
}

func artifactSummariesFromCommandResult(result map[string]any) []artifacts.Summary {
	var out []artifacts.Summary
	appendArtifactSummariesFromValue(&out, result)
	return mergeArtifactSummaries(nil, out)
}

func existingLocalPathsFromText(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	candidates := quotedLocalPathsFromText(text)
	candidates = append(candidates, discoveredLocalPathsFromText(text)...)
	return uniqueStrings(candidates)
}

func quotedLocalPathsFromText(text string) []string {
	var out []string
	start := -1
	quote := rune(0)
	for i, r := range text {
		if quote == 0 {
			if r == '"' || r == '\'' || r == '“' || r == '‘' {
				start = i + len(string(r))
				quote = r
			}
			continue
		}
		if matchingQuote(r, quote) {
			value := strings.TrimSpace(text[start:i])
			if path := existingPath(value); path != "" {
				out = append(out, path)
			}
			quote = 0
			start = -1
		}
	}
	return out
}

func matchingQuote(r, quote rune) bool {
	switch quote {
	case '"':
		return r == '"'
	case '\'':
		return r == '\''
	case '“':
		return r == '”' || r == '"'
	case '‘':
		return r == '’' || r == '\''
	default:
		return false
	}
}

func discoveredLocalPathsFromText(text string) []string {
	var out []string
	drivePattern := regexp.MustCompile(`[A-Za-z]:[\\/]`)
	for _, loc := range drivePattern.FindAllStringIndex(text, -1) {
		start := loc[0]
		end := start
		for end < len(text) && text[end] != '\r' && text[end] != '\n' && text[end] != '"' && text[end] != '\'' {
			end++
		}
		segment := text[start:end]
		if path := longestExistingPathPrefix(segment); path != "" {
			out = append(out, path)
		}
	}
	return out
}

func longestExistingPathPrefix(segment string) string {
	segment = strings.TrimSpace(segment)
	for end := len(segment); end > 2; end-- {
		if end < len(segment) {
			r := rune(segment[end])
			if r > 127 && !isPathBoundaryRune(r) {
				continue
			}
		}
		candidate := trimPathCandidate(segment[:end])
		if path := existingPath(candidate); path != "" {
			return path
		}
	}
	return ""
}

func trimPathCandidate(path string) string {
	return strings.Trim(strings.TrimSpace(path), " \t\r\n\"'“”‘’`*_。，,;；!！?？)）]】}")
}

func existingPath(path string) string {
	path = trimPathCandidate(path)
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if st, err := os.Stat(clean); err == nil && (st.IsDir() || previewablePath(clean)) {
		if abs, absErr := filepath.Abs(clean); absErr == nil {
			return abs
		}
		return clean
	}
	return ""
}

func isPathBoundaryRune(r rune) bool {
	switch r {
	case ' ', '\t', '/', '\\', '.', '-', '_', '(', ')', '[', ']', '{', '}', '（', '）', '【', '】':
		return true
	default:
		return false
	}
}

func previewableFileNamesFromText(text string) []string {
	matches := mediaReferenceFilePattern.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		name := cleanupMediaFileName(match)
		if name != "" && previewablePath(name) {
			out = append(out, filepath.Base(name))
		}
	}
	return uniqueStrings(out)
}

func cleanupMediaFileName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n\"'“”‘’`*_。，,;；!！?？)）]】}")
	for {
		next := strings.TrimLeft(value, " \t-*`0123456789.、)）")
		if next == value {
			break
		}
		value = next
	}
	return strings.TrimSpace(value)
}

func previewablePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	if ext == "" {
		return false
	}
	_, ok := previewableMediaExtensions()[ext]
	return ok
}

func previewableMediaExtensions() map[string]bool {
	return map[string]bool{
		".wav": true, ".mp3": true, ".flac": true, ".aif": true, ".aiff": true, ".ogg": true, ".oga": true, ".m4a": true, ".wma": true,
		".mp4": true, ".mov": true, ".mkv": true, ".webm": true, ".avi": true, ".m4v": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".tif": true, ".tiff": true,
		".pdf": true, ".txt": true, ".md": true, ".rtf": true, ".html": true, ".htm": true, ".doc": true, ".docx": true,
		".mid": true, ".midi": true,
	}
}

func dialogueMentionsMedia(parts ...string) bool {
	text := strings.ToLower(strings.Join(parts, "\n"))
	return strings.Contains(text, "\u7d20\u6750") || strings.Contains(text, "\u5a92\u4f53") ||
		strings.Contains(text, "\u97f3\u9891") || strings.Contains(text, "\u89c6\u9891") ||
		strings.Contains(text, "\u56fe\u7247") || strings.Contains(text, "\u6587\u6863") ||
		strings.Contains(text, "media") || strings.Contains(text, "asset") ||
		strings.Contains(text, "audio") || strings.Contains(text, "video") ||
		strings.Contains(text, "image") || strings.Contains(text, "document")
}

func mediaKindsFromDialogue(text string) []any {
	lower := strings.ToLower(text)
	var out []string
	add := func(kind string) {
		for _, existing := range out {
			if existing == kind {
				return
			}
		}
		out = append(out, kind)
	}
	if strings.Contains(lower, "\u89c6\u9891") || strings.Contains(lower, "video") {
		add("video")
	}
	if strings.Contains(lower, "\u97f3\u9891") || strings.Contains(lower, "\u97f3\u4e50") || strings.Contains(lower, "audio") || strings.Contains(lower, "music") {
		add("audio")
	}
	if strings.Contains(lower, "\u56fe\u7247") || strings.Contains(lower, "\u56fe\u50cf") || strings.Contains(lower, "image") || strings.Contains(lower, "picture") {
		add("image")
	}
	if strings.Contains(lower, "\u6587\u6863") || strings.Contains(lower, "document") || strings.Contains(lower, "pdf") {
		add("document")
	}
	if strings.Contains(lower, "midi") || strings.Contains(lower, ".mid") {
		add("midi")
	}
	if len(out) == 0 {
		add("audio")
		add("video")
		add("image")
		add("document")
		add("midi")
	}
	values := make([]any, 0, len(out))
	for _, kind := range out {
		values = append(values, kind)
	}
	return values
}

func dialogueAsksRecursive(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "\u5b50\u6587\u4ef6\u5939") || strings.Contains(lower, "\u9012\u5f52") ||
		strings.Contains(lower, "subfolder") || strings.Contains(lower, "recursive")
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
