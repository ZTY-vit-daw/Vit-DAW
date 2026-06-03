package workspace

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	DefaultMaxReadBytes = 256 * 1024
	DefaultMaxResults   = 200
	DefaultMaxScanBytes = 2 * 1024 * 1024
)

type Root struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type Context struct {
	Roots []Root `json:"roots"`
}

func NewContext(requestContext map[string]any, state map[string]any) Context {
	roots := []Root{}
	add := func(id, path string) {
		path = cleanPath(path)
		if id == "" || path == "" {
			return
		}
		for _, root := range roots {
			if strings.EqualFold(root.ID, id) || samePath(root.Path, path) {
				return
			}
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			roots = append(roots, Root{ID: id, Path: path})
		}
	}

	add("agent", `D:\Vit_DAW\agent`)
	add("vitapp", `D:\Vit_DAW\VitApp`)
	add("godot", `D:\Godot\project\vit-daw-frontend`)
	add("logs", `D:\Vit_DAW\VitApp\Workspace\Logs`)

	for _, key := range []string{"project_path", "current_project_path"} {
		if path := stringValue(requestContext[key]); path != "" {
			addProjectRoots(add, path)
		}
		if path := stringValue(state[key]); path != "" {
			addProjectRoots(add, path)
		}
	}
	if shadow, ok := state["shadow"].(map[string]any); ok {
		if path := stringValue(shadow["project_path"]); path != "" {
			addProjectRoots(add, path)
		}
	}
	if rootsAny, ok := requestContext["workspace_roots"].([]any); ok {
		for i, raw := range rootsAny {
			add(fmt.Sprintf("context_%d", i+1), stringValue(raw))
		}
	}
	if roots, ok := requestContext["workspace_roots"].([]string); ok {
		for i, raw := range roots {
			add(fmt.Sprintf("context_%d", i+1), raw)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].ID < roots[j].ID })
	return Context{Roots: roots}
}

func addProjectRoots(add func(string, string), projectPath string) {
	projectPath = cleanPath(projectPath)
	if projectPath == "" {
		return
	}
	projectDir := filepath.Dir(projectPath)
	add("project", projectDir)
	base := strings.TrimSuffix(filepath.Base(projectPath), filepath.Ext(projectPath))
	if base != "" {
		add("project_media", filepath.Join(projectDir, base+"_Media"))
	}
}

func (c Context) Resolve(rootID, rawPath string, allowMissing bool) (string, Root, error) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", Root{}, errors.New("path is required")
	}
	if filepath.IsAbs(rawPath) {
		path := cleanPath(rawPath)
		for _, root := range c.Roots {
			if isWithin(root.Path, path) {
				if !allowMissing {
					if _, err := os.Stat(path); err != nil {
						return "", root, err
					}
				}
				return path, root, nil
			}
		}
		return "", Root{}, fmt.Errorf("path is outside allowed workspace roots: %s", path)
	}
	root, ok := c.RootByID(rootID)
	if !ok {
		if len(c.Roots) == 0 {
			return "", Root{}, errors.New("no workspace roots are available")
		}
		root = c.Roots[0]
	}
	path := cleanPath(filepath.Join(root.Path, rawPath))
	if !isWithin(root.Path, path) {
		return "", root, fmt.Errorf("path escapes workspace root %s", root.ID)
	}
	if !allowMissing {
		if _, err := os.Stat(path); err != nil {
			return "", root, err
		}
	}
	return path, root, nil
}

func (c Context) RootByID(id string) (Root, bool) {
	id = strings.TrimSpace(id)
	for _, root := range c.Roots {
		if root.ID == id {
			return root, true
		}
	}
	return Root{}, false
}

func Glob(ctx Context, args map[string]any) (map[string]any, error) {
	rootID := stringValue(args["root_id"])
	pattern := firstNonEmpty(stringValue(args["pattern"]), "*")
	maxResults := intValue(args["max_results"], DefaultMaxResults)
	root, ok := ctx.RootByID(rootID)
	if !ok {
		if len(ctx.Roots) == 0 {
			return nil, errors.New("no workspace roots are available")
		}
		root = ctx.Roots[0]
	}
	matches := []map[string]any{}
	err := filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path != root.Path && d.IsDir() && skipDir(d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root.Path, path)
		if err != nil {
			return nil
		}
		if !matchPattern(pattern, filepath.ToSlash(rel)) {
			return nil
		}
		info, _ := d.Info()
		matches = append(matches, map[string]any{
			"path":          path,
			"relative_path": filepath.ToSlash(rel),
			"size":          info.Size(),
		})
		if len(matches) >= maxResults {
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return map[string]any{"root": root, "matches": matches, "truncated": len(matches) >= maxResults}, nil
}

func Grep(ctx Context, args map[string]any) (map[string]any, error) {
	query := stringValue(args["query"])
	if query == "" {
		return nil, errors.New("query is required")
	}
	rootID := stringValue(args["root_id"])
	pattern := firstNonEmpty(stringValue(args["pattern"]), "*")
	maxResults := intValue(args["max_results"], DefaultMaxResults)
	root, ok := ctx.RootByID(rootID)
	if !ok {
		if len(ctx.Roots) == 0 {
			return nil, errors.New("no workspace roots are available")
		}
		root = ctx.Roots[0]
	}
	matches := []map[string]any{}
	err := filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path != root.Path && d.IsDir() && skipDir(d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root.Path, path)
		if err != nil || !matchPattern(pattern, filepath.ToSlash(rel)) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > DefaultMaxScanBytes {
			return nil
		}
		fileMatches, err := grepFile(path, query, root.Path)
		if err != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		if len(matches) >= maxResults {
			matches = matches[:maxResults]
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return map[string]any{"root": root, "matches": matches, "truncated": len(matches) >= maxResults}, nil
}

func ReadFile(ctx Context, args map[string]any) (map[string]any, error) {
	path, root, err := ctx.Resolve(stringValue(args["root_id"]), stringValue(args["path"]), false)
	if err != nil {
		return nil, err
	}
	maxBytes := int64(intValue(args["max_bytes"], DefaultMaxReadBytes))
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("file is too large for text read: %d > %d", info.Size(), maxBytes)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if isBinary(b) {
		return nil, errors.New("file appears to be binary; use workspace.blob_info")
	}
	return map[string]any{"root": root, "path": path, "size": info.Size(), "content": string(b)}, nil
}

func FileInfo(ctx Context, args map[string]any) (map[string]any, error) {
	path, root, err := ctx.Resolve(stringValue(args["root_id"]), stringValue(args["path"]), false)
	if err != nil {
		return nil, err
	}
	return fileInfo(root, path, boolValue(args["hash"], false))
}

func EditPreview(ctx Context, args map[string]any) (map[string]any, error) {
	path, root, err := ctx.Resolve(stringValue(args["root_id"]), stringValue(args["path"]), false)
	if err != nil {
		return nil, err
	}
	if err := ensureTextWritablePath(path); err != nil {
		return nil, err
	}
	oldText := stringValue(args["old_text"])
	newText := stringValue(args["new_text"])
	if oldText == "" {
		return nil, errors.New("old_text is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if isBinary(b) {
		return nil, errors.New("file appears to be binary; text edit refused")
	}
	content := string(b)
	count := strings.Count(content, oldText)
	if count == 0 {
		return nil, errors.New("old_text was not found")
	}
	if !boolValue(args["replace_all"], false) {
		count = 1
	}
	return map[string]any{
		"root":         root,
		"path":         path,
		"replacements": count,
		"preview":      previewReplacement(content, oldText, newText),
	}, nil
}

func ApplyEdit(ctx Context, args map[string]any) (map[string]any, error) {
	path, root, err := ctx.Resolve(stringValue(args["root_id"]), stringValue(args["path"]), false)
	if err != nil {
		return nil, err
	}
	if err := ensureTextWritablePath(path); err != nil {
		return nil, err
	}
	oldText := stringValue(args["old_text"])
	newText := stringValue(args["new_text"])
	if oldText == "" {
		return nil, errors.New("old_text is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if isBinary(b) {
		return nil, errors.New("file appears to be binary; text edit refused")
	}
	content := string(b)
	count := strings.Count(content, oldText)
	if count == 0 {
		return nil, errors.New("old_text was not found")
	}
	replaceAll := boolValue(args["replace_all"], false)
	if replaceAll {
		content = strings.ReplaceAll(content, oldText, newText)
	} else {
		content = strings.Replace(content, oldText, newText, 1)
		count = 1
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return map[string]any{"root": root, "path": path, "replacements": count, "reverse_old_text": newText, "reverse_new_text": oldText}, nil
}

func WriteFile(ctx Context, args map[string]any) (map[string]any, error) {
	path, root, err := ctx.Resolve(stringValue(args["root_id"]), stringValue(args["path"]), true)
	if err != nil {
		return nil, err
	}
	if err := ensureTextWritablePath(path); err != nil {
		return nil, err
	}
	content := stringValue(args["content"])
	if !utf8.ValidString(content) {
		return nil, errors.New("content is not valid UTF-8 text")
	}
	if !boolValue(args["overwrite"], false) {
		if _, err := os.Stat(path); err == nil {
			return nil, errors.New("target exists; set overwrite:true to replace it")
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return map[string]any{"root": root, "path": path, "bytes": len(content)}, nil
}

func BlobInfo(ctx Context, args map[string]any) (map[string]any, error) {
	path, root, err := ctx.Resolve(stringValue(args["root_id"]), stringValue(args["path"]), false)
	if err != nil {
		return nil, err
	}
	return fileInfo(root, path, true)
}

func BlobCopy(ctx Context, args map[string]any) (map[string]any, error) {
	src, srcRoot, err := ctx.Resolve(stringValue(args["source_root_id"]), stringValue(args["source_path"]), false)
	if err != nil {
		return nil, err
	}
	dst, dstRoot, err := ctx.Resolve(stringValue(args["target_root_id"]), stringValue(args["target_path"]), true)
	if err != nil {
		return nil, err
	}
	if isLiveProjectPath(dst) {
		return nil, errors.New("refusing to write live project files through workspace blob_copy")
	}
	if !boolValue(args["overwrite"], false) {
		if _, err := os.Stat(dst); err == nil {
			return nil, errors.New("target exists; set overwrite:true to replace it")
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}
	in, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return nil, err
	}
	n, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return map[string]any{"source_root": srcRoot, "target_root": dstRoot, "source_path": src, "target_path": dst, "bytes": n}, nil
}

func grepFile(path, query, root string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	head := make([]byte, 1024)
	n, _ := f.Read(head)
	if isBinary(head[:n]) {
		return nil, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(f)
	lineNo := 0
	out := []map[string]any{}
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
			rel, _ := filepath.Rel(root, path)
			out = append(out, map[string]any{
				"path":          path,
				"relative_path": filepath.ToSlash(rel),
				"line":          lineNo,
				"text":          line,
			})
		}
	}
	return out, nil
}

func fileInfo(root Root, path string, hash bool) (map[string]any, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"root":     root,
		"path":     path,
		"size":     info.Size(),
		"mode":     info.Mode().String(),
		"mod_time": info.ModTime(),
		"is_dir":   info.IsDir(),
	}
	if hash && !info.IsDir() {
		sum, err := sha256File(path)
		if err != nil {
			return nil, err
		}
		out["sha256"] = sum
	}
	return out, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func matchPattern(pattern, rel string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" || pattern == "*" || pattern == "**" || pattern == "**/*" {
		return true
	}
	if ok, _ := filepath.Match(pattern, rel); ok {
		return true
	}
	if !strings.Contains(pattern, "/") {
		if ok, _ := filepath.Match(pattern, filepath.Base(rel)); ok {
			return true
		}
	}
	return strings.Contains(strings.ToLower(rel), strings.ToLower(strings.Trim(pattern, "*")))
}

func skipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".vit_history", "node_modules", "build_release", ".godot", ".cache":
		return true
	default:
		return false
	}
}

func ensureTextWritablePath(path string) error {
	if isLiveProjectPath(path) {
		return errors.New("refusing to write live project files through workspace text tools")
	}
	return nil
}

func isLiveProjectPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".vit" || ext == ".tracktionedit"
}

func isBinary(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	if bytesIndexByte(b, 0) >= 0 {
		return true
	}
	return !utf8.Valid(b)
}

func bytesIndexByte(b []byte, needle byte) int {
	for i, v := range b {
		if v == needle {
			return i
		}
	}
	return -1
}

func previewReplacement(content, oldText, newText string) string {
	idx := strings.Index(content, oldText)
	if idx < 0 {
		return ""
	}
	start := idx - 160
	if start < 0 {
		start = 0
	}
	end := idx + len(oldText) + 160
	if end > len(content) {
		end = len(content)
	}
	return content[start:idx] + "[-" + oldText + "-][+" + newText + "+]" + content[idx+len(oldText):end]
}

func isWithin(root, path string) bool {
	root = cleanPath(root)
	path = cleanPath(path)
	if samePath(root, path) {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != "" && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

func samePath(a, b string) bool {
	return strings.EqualFold(cleanPath(a), cleanPath(b))
}

func cleanPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

func stringValue(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	default:
		if v == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func intValue(v any, fallback int) int {
	switch t := v.(type) {
	case int:
		if t > 0 {
			return t
		}
	case int64:
		if t > 0 {
			return int(t)
		}
	case float64:
		if t > 0 {
			return int(t)
		}
	case string:
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(t), "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func boolValue(v any, fallback bool) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" && strings.TrimSpace(v) != "<nil>" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
