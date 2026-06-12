package resourceintake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/artifacts"
)

type ScanOptions struct {
	DownloadDirs  []string
	Since         time.Duration
	MinModifiedAt time.Time
	Limit         int
	Now           time.Time
}

const (
	defaultMediaIndexLimit = 80
	maxMediaIndexLimit     = 200
)

func RegisterURL(store artifacts.Store, args map[string]any) (map[string]any, error) {
	raw := strings.TrimSpace(stringValue(args["url"]))
	if raw == "" {
		return nil, errors.New("url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return nil, errors.New("valid absolute url is required")
	}
	now := time.Now()
	a := artifacts.New(now)
	a.ID = "art_url_" + stableID(raw)
	a.Kind = "web_page"
	a.Source = firstNonEmpty(stringValue(args["source"]), "external_browser")
	a.URL = raw
	a.Title = firstNonEmpty(stringValue(args["title"]), raw)
	a.Text = raw
	a.Summary = "Saved URL from external browser."
	a.ConversationID = stringValue(args["conversation_id"])
	a.GoalID = stringValue(args["goal_id"])
	a.RunID = stringValue(args["run_id"])
	a.Metadata = map[string]any{
		"resource_origin": "url_bar",
		"user_approved":   true,
	}
	a = artifacts.ApplyScope(a, artifacts.ScopeFromMap(args))
	a, err = store.Upsert(a)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":   "ok",
		"artifact": a.CompactSummary(),
	}, nil
}

func ScanDownloads(store artifacts.Store, args map[string]any, opts ScanOptions) (map[string]any, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	since := opts.Since
	if since <= 0 {
		since = durationFromMinutes(args["since_minutes"], 30)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = intValue(args["limit"], 80)
	}
	dirs := downloadDirsForScan(args, opts)
	if len(dirs) == 0 {
		return map[string]any{"status": "ok", "artifacts": []artifacts.Summary{}, "download_dirs": []string{}}, nil
	}
	snapshot := downloadsSnapshot(dirs)
	changeToken := stringValue(args["change_token"])
	changed := changeToken == "" || snapshot.Token != changeToken
	if boolValue(args["only_if_changed"]) && changeToken != "" && !changed {
		return map[string]any{
			"status":        "ok",
			"artifacts":     []artifacts.Summary{},
			"download_dirs": dirs,
			"count":         0,
			"changed":       false,
			"change_token":  snapshot.Token,
		}, nil
	}
	cutoff := now.Add(-since)
	if !opts.MinModifiedAt.IsZero() && cutoff.Before(opts.MinModifiedAt) {
		cutoff = opts.MinModifiedAt
	}
	files := recentDownloadFiles(dirs, cutoff, limit)
	out := make([]artifacts.Summary, 0, len(files))
	scope := artifacts.ScopeFromMap(args)
	for _, row := range files {
		a := artifactFromDownload(row, args, scope, now)
		if _, err := store.Get(a.ID); err == nil {
			continue
		}
		stored, err := store.Upsert(a)
		if err != nil {
			return nil, err
		}
		out = append(out, stored.CompactSummary())
	}
	return map[string]any{
		"status":        "ok",
		"artifacts":     out,
		"download_dirs": dirs,
		"count":         len(out),
		"changed":       changed,
		"change_token":  snapshot.Token,
	}, nil
}

func WatchDownloads(ctx context.Context, args map[string]any, opts ScanOptions) (map[string]any, error) {
	dirs := downloadDirsForScan(args, opts)
	if len(dirs) == 0 {
		return map[string]any{"status": "ok", "changed": false, "change_token": "", "download_dirs": []string{}}, nil
	}
	changeToken := stringValue(args["change_token"])
	timeout := durationFromMillis(args["timeout_ms"], 25000, 1000, 60000)
	poll := durationFromMillis(args["poll_ms"], 1500, 100, 10000)
	snapshot := downloadsSnapshot(dirs)
	if changeToken == "" || snapshot.Token != changeToken {
		return map[string]any{
			"status":        "ok",
			"changed":       true,
			"change_token":  snapshot.Token,
			"download_dirs": dirs,
			"file_count":    snapshot.FileCount,
		}, nil
	}
	timer := time.NewTimer(timeout)
	ticker := time.NewTicker(poll)
	defer timer.Stop()
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return map[string]any{
				"status":        "ok",
				"changed":       false,
				"change_token":  changeToken,
				"download_dirs": dirs,
				"file_count":    snapshot.FileCount,
			}, nil
		case <-timer.C:
			snapshot = downloadsSnapshot(dirs)
			return map[string]any{
				"status":        "ok",
				"changed":       false,
				"change_token":  snapshot.Token,
				"download_dirs": dirs,
				"file_count":    snapshot.FileCount,
			}, nil
		case <-ticker.C:
			snapshot = downloadsSnapshot(dirs)
			if snapshot.Token != changeToken {
				return map[string]any{
					"status":        "ok",
					"changed":       true,
					"change_token":  snapshot.Token,
					"download_dirs": dirs,
					"file_count":    snapshot.FileCount,
				}, nil
			}
		}
	}
}

func RegisterAssets(store artifacts.Store, args map[string]any) (map[string]any, error) {
	paths := assetPathsFromArgs(args)
	if len(paths) == 0 {
		return nil, errors.New("at least one asset path is required")
	}
	limit := boundedLimit(args["limit"], defaultMediaIndexLimit, maxMediaIndexLimit)
	out := make([]artifacts.Summary, 0, minInt(len(paths), limit))
	skipped := make([]map[string]any, 0)
	scope := artifacts.ScopeFromMap(args)
	for _, raw := range paths {
		if len(out) >= limit {
			skipped = appendLimitedSkip(skipped, raw, "limit reached")
			continue
		}
		summary, reason, err := registerAssetPath(store, raw, args, scope, "")
		if err != nil {
			return nil, err
		}
		if reason != "" {
			skipped = appendLimitedSkip(skipped, raw, reason)
			continue
		}
		out = append(out, summary)
	}
	return map[string]any{
		"status":        "ok",
		"artifacts":     out,
		"count":         len(out),
		"skipped_count": len(paths) - len(out),
		"skipped":       skipped,
	}, nil
}

func IndexAuthorizedFolder(store artifacts.Store, args map[string]any) (map[string]any, error) {
	locations := assetLocationsFromArgs(args)
	if len(locations) == 0 {
		return nil, errors.New("asset_location or folder_path is required")
	}
	limit := boundedLimit(args["limit"], defaultMediaIndexLimit, maxMediaIndexLimit)
	recursive := boolValue(args["recursive"])
	allowedKinds := mediaKindSet(args)
	out := make([]artifacts.Summary, 0, limit)
	skipped := make([]map[string]any, 0)
	scope := artifacts.ScopeFromMap(args)
	for _, location := range locations {
		if len(out) >= limit {
			break
		}
		clean := cleanAssetPath(location)
		info, err := os.Stat(clean)
		if err != nil {
			skipped = appendLimitedSkip(skipped, location, err.Error())
			continue
		}
		if !info.IsDir() {
			summary, reason, err := registerAssetPath(store, clean, args, scope, filepath.Dir(clean))
			if err != nil {
				return nil, err
			}
			if reason != "" {
				skipped = appendLimitedSkip(skipped, clean, reason)
				continue
			}
			if !mediaKindAllowed(summary.Kind, allowedKinds) {
				skipped = appendLimitedSkip(skipped, clean, "kind filtered")
				continue
			}
			out = append(out, summary)
			continue
		}
		files := mediaFilesInFolder(clean, recursive, limit-len(out), allowedKinds)
		for _, path := range files {
			if len(out) >= limit {
				break
			}
			summary, reason, err := registerAssetPath(store, path, args, scope, clean)
			if err != nil {
				return nil, err
			}
			if reason != "" {
				skipped = appendLimitedSkip(skipped, path, reason)
				continue
			}
			out = append(out, summary)
		}
	}
	return map[string]any{
		"status":        "ok",
		"artifacts":     out,
		"count":         len(out),
		"skipped":       skipped,
		"skipped_count": len(skipped),
		"recursive":     recursive,
		"limit":         limit,
		"locations":     locations,
	}, nil
}

type downloadFile struct {
	Path     string
	Dir      string
	Name     string
	Size     int64
	Modified time.Time
}

type downloadSnapshot struct {
	Token     string
	FileCount int
}

func downloadDirsForScan(args map[string]any, opts ScanOptions) []string {
	dirs := cleanDirs(opts.DownloadDirs)
	if len(dirs) == 0 {
		dirs = dirsFromArgs(args)
	}
	if len(dirs) == 0 {
		dirs = defaultDownloadDirs()
	}
	return dirs
}

func downloadsSnapshot(dirs []string) downloadSnapshot {
	rows := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			rows = append(rows, dir+"|error")
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || isTemporaryDownloadName(entry.Name()) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				rows = append(rows, filepath.Join(dir, entry.Name())+"|error")
				continue
			}
			rows = append(rows, fmt.Sprintf("%s|%s|%d|%d", dir, entry.Name(), info.Size(), info.ModTime().UnixNano()))
		}
	}
	sort.Strings(rows)
	return downloadSnapshot{
		Token:     stableID(strings.Join(rows, "\n")),
		FileCount: len(rows),
	}
}

func recentDownloadFiles(dirs []string, cutoff time.Time, limit int) []downloadFile {
	rows := []downloadFile{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if isTemporaryDownloadName(name) {
				continue
			}
			path := filepath.Join(dir, name)
			info, err := entry.Info()
			if err != nil || info.Size() <= 0 || info.ModTime().Before(cutoff) {
				continue
			}
			rows = append(rows, downloadFile{
				Path:     path,
				Dir:      dir,
				Name:     name,
				Size:     info.Size(),
				Modified: info.ModTime(),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Modified.After(rows[j].Modified)
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func artifactFromDownload(row downloadFile, args map[string]any, scope artifacts.Scope, now time.Time) artifacts.Artifact {
	a := artifacts.New(now)
	a.ID = "art_download_" + stableID(strings.ToLower(filepath.Clean(row.Path)))
	a.Kind = kindForPath(row.Path)
	a.Source = "downloads"
	a.Path = row.Path
	a.Title = row.Name
	a.MIME = firstNonEmpty(mime.TypeByExtension(strings.ToLower(filepath.Ext(row.Path))), "")
	a.SizeBytes = row.Size
	a.Summary = fmt.Sprintf("Downloaded file from %s.", row.Dir)
	a.ConversationID = stringValue(args["conversation_id"])
	a.GoalID = stringValue(args["goal_id"])
	a.RunID = stringValue(args["run_id"])
	a.Metadata = map[string]any{
		"resource_origin": "download_folder",
		"download_dir":    row.Dir,
		"modified_at":     row.Modified.UTC().Format(time.RFC3339Nano),
		"extension":       strings.ToLower(filepath.Ext(row.Path)),
	}
	return artifacts.ApplyScope(a, scope)
}

func registerAssetPath(store artifacts.Store, raw string, args map[string]any, scope artifacts.Scope, sourceRoot string) (artifacts.Summary, string, error) {
	path := cleanAssetPath(raw)
	if path == "" {
		return artifacts.Summary{}, "empty path", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return artifacts.Summary{}, err.Error(), nil
	}
	if info.IsDir() {
		return artifacts.Summary{}, "path is a folder", nil
	}
	if info.Size() <= 0 {
		return artifacts.Summary{}, "file is empty", nil
	}
	kind := kindForPath(path)
	if !isPreviewableMediaKind(kind) {
		return artifacts.Summary{}, "unsupported media kind", nil
	}
	id := firstNonEmpty(stringValue(args["artifact_id"]), stringValue(args["id"]))
	explicitSingleFile := strings.TrimSpace(sourceRoot) == "" && len(assetPathsFromArgs(args)) <= 1
	if id == "" || !explicitSingleFile {
		id = "art_media_" + stableID(strings.ToLower(filepath.Clean(path)))
	}
	a := artifacts.ArtifactFromFile(path, id, stringValue(args["conversation_id"]), stringValue(args["goal_id"]), stringValue(args["run_id"]))
	if a.Kind == "" || a.Kind == "unknown" {
		a.Kind = kind
	}
	if kindOverride := firstNonEmpty(stringValue(args["kind"]), stringValue(args["media_kind"])); kindOverride != "" && explicitSingleFile {
		a.Kind = kindOverride
	}
	if title := stringValue(args["title"]); title != "" && explicitSingleFile {
		a.Title = title
	}
	a.Source = firstNonEmpty(stringValue(args["source"]), "authorized_media")
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}
	a.Metadata["resource_origin"] = "authorized_asset_location"
	a.Metadata["user_authorized"] = true
	a.Metadata["extension"] = strings.ToLower(filepath.Ext(path))
	a.Metadata["modified_at"] = info.ModTime().UTC().Format(time.RFC3339Nano)
	if strings.TrimSpace(sourceRoot) != "" {
		a.Metadata["source_root"] = filepath.Clean(sourceRoot)
	}
	a = artifacts.ApplyScope(a, scope)
	a = artifacts.Extract(a, intValue(args["max_text_runes"], artifacts.DefaultTextLimit))
	stored, err := store.Upsert(a)
	if err != nil {
		return artifacts.Summary{}, "", err
	}
	return stored.CompactSummary(), "", nil
}

func assetPathsFromArgs(args map[string]any) []string {
	return uniqueCleanStrings(valuesFromArgs(args,
		"file_path", "path", "absolute_path", "asset_path", "asset_location",
		"file_paths", "paths", "absolute_paths", "asset_paths", "asset_locations", "files",
	))
}

func assetLocationsFromArgs(args map[string]any) []string {
	return uniqueCleanStrings(valuesFromArgs(args,
		"asset_location", "folder_path", "directory", "dir", "path",
		"asset_locations", "folder_paths", "directories", "dirs", "paths",
	))
}

func valuesFromArgs(args map[string]any, keys ...string) []string {
	if args == nil {
		return nil
	}
	var out []string
	for _, key := range keys {
		switch value := args[key].(type) {
		case string:
			out = append(out, value)
		case []string:
			out = append(out, value...)
		case []any:
			for _, row := range value {
				if text := stringValue(row); text != "" {
					out = append(out, text)
				}
			}
		}
	}
	return out
}

func uniqueCleanStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, row := range in {
		clean := cleanAssetPath(row)
		if clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, clean)
	}
	return out
}

func cleanAssetPath(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "\"")
	if path == "" || path == "<nil>" {
		return ""
	}
	clean := filepath.Clean(path)
	if abs, err := filepath.Abs(clean); err == nil {
		return abs
	}
	return clean
}

func mediaFilesInFolder(root string, recursive bool, limit int, allowedKinds map[string]bool) []string {
	if limit <= 0 {
		return nil
	}
	out := make([]string, 0, limit)
	addFile := func(path string) {
		if len(out) >= limit || isTemporaryDownloadName(filepath.Base(path)) {
			return
		}
		kind := kindForPath(path)
		if !isPreviewableMediaKind(kind) || !mediaKindAllowed(kind, allowedKinds) {
			return
		}
		out = append(out, path)
	}
	if !recursive {
		entries, err := os.ReadDir(root)
		if err != nil {
			return out
		}
		for _, entry := range entries {
			if len(out) >= limit {
				break
			}
			if entry.IsDir() {
				continue
			}
			addFile(filepath.Join(root, entry.Name()))
		}
		return out
	}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(out) >= limit {
			return filepath.SkipAll
		}
		if entry.IsDir() {
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		addFile(path)
		return nil
	})
	return out
}

func mediaKindSet(args map[string]any) map[string]bool {
	values := valuesFromArgs(args, "media_kind", "kind", "media_kinds", "kinds")
	out := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == ';' || r == '|' || r == ' '
		}) {
			kind := strings.ToLower(strings.TrimSpace(part))
			if kind == "" {
				continue
			}
			switch kind {
			case "images":
				kind = "image"
			case "audios":
				kind = "audio"
			case "videos":
				kind = "video"
			case "documents", "docs":
				kind = "document"
			case "midis":
				kind = "midi"
			}
			out[kind] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mediaKindAllowed(kind string, allowed map[string]bool) bool {
	if len(allowed) == 0 {
		return true
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	return allowed[kind] || (kind == "text" && allowed["document"])
}

func isPreviewableMediaKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "audio", "video", "image", "document", "text", "midi":
		return true
	default:
		return false
	}
}

func appendLimitedSkip(rows []map[string]any, path, reason string) []map[string]any {
	if len(rows) >= 20 {
		return rows
	}
	return append(rows, map[string]any{
		"path":   path,
		"reason": reason,
	})
}

func defaultDownloadDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}
	out := []string{filepath.Join(home, "Downloads")}
	if runtime.GOOS == "windows" {
		if profile := strings.TrimSpace(os.Getenv("USERPROFILE")); profile != "" && profile != home {
			out = append(out, filepath.Join(profile, "Downloads"))
		}
	}
	return cleanDirs(out)
}

func dirsFromArgs(args map[string]any) []string {
	out := []string{}
	if value := stringValue(args["download_dir"]); value != "" {
		out = append(out, value)
	}
	if value := stringValue(args["download_path"]); value != "" {
		out = append(out, value)
	}
	switch rows := args["download_dirs"].(type) {
	case []string:
		out = append(out, rows...)
	case []any:
		for _, row := range rows {
			if value := stringValue(row); value != "" {
				out = append(out, value)
			}
		}
	}
	return cleanDirs(out)
}

func cleanDirs(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, row := range in {
		dir := strings.TrimSpace(row)
		if dir == "" {
			continue
		}
		clean := filepath.Clean(dir)
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		if st, err := os.Stat(clean); err == nil && st.IsDir() {
			seen[key] = true
			out = append(out, clean)
		}
	}
	return out
}

func isTemporaryDownloadName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return true
	}
	for _, suffix := range []string{".crdownload", ".part", ".tmp", ".download", ".opdownload"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return strings.HasPrefix(lower, ".")
}

func kindForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav", ".mp3", ".flac", ".aif", ".aiff", ".ogg", ".oga", ".m4a", ".wma":
		return "audio"
	case ".mid", ".midi":
		return "midi"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		return "image"
	case ".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v":
		return "video"
	case ".pdf", ".txt", ".md", ".rtf", ".html", ".htm", ".doc", ".docx":
		return "document"
	case ".zip", ".rar", ".7z", ".tar", ".gz":
		return "archive"
	default:
		return "file"
	}
}

func stableID(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])[:24]
}

func durationFromMinutes(value any, fallbackMinutes int) time.Duration {
	minutes := intValue(value, fallbackMinutes)
	if minutes <= 0 {
		minutes = fallbackMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func durationFromMillis(value any, fallbackMillis, minMillis, maxMillis int) time.Duration {
	millis := intValue(value, fallbackMillis)
	if millis <= 0 {
		millis = fallbackMillis
	}
	if minMillis > 0 && millis < minMillis {
		millis = minMillis
	}
	if maxMillis > 0 && millis > maxMillis {
		millis = maxMillis
	}
	return time.Duration(millis) * time.Millisecond
}

func boolValue(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		clean := strings.ToLower(strings.TrimSpace(b))
		return clean == "1" || clean == "true" || clean == "yes" || clean == "on"
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func intValue(v any, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return fallback
	}
}

func boundedLimit(value any, fallback, max int) int {
	limit := intValue(value, fallback)
	if limit <= 0 {
		limit = fallback
	}
	if max > 0 && limit > max {
		limit = max
	}
	return limit
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
