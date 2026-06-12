package artifacts

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Scope struct {
	ProjectPath     string
	RootProjectPath string
	ActiveWorktree  string
	ActiveBranch    string
	ActiveNodeID    string
	HistoryScopeKey string
	MediaScopeKey   string
}

func ScopeFromMap(args map[string]any) Scope {
	scope := Scope{}
	if args == nil {
		return scope
	}
	scope.ProjectPath = scopePath(firstScopeString(args, "project_path", "current_project_path"))
	scope.RootProjectPath = scopePath(firstScopeString(args, "root_project_path"))
	scope.ActiveWorktree = cleanScopeValue(firstScopeString(args, "active_worktree", "worktree"))
	scope.ActiveBranch = cleanScopeValue(firstScopeString(args, "active_branch", "branch"))
	scope.ActiveNodeID = cleanScopeValue(firstScopeString(args, "active_node_id", "node_id"))
	scope.HistoryScopeKey = cleanScopeValue(firstScopeString(args, "history_scope_key"))
	scope.MediaScopeKey = cleanScopeValue(firstScopeString(args, "media_scope_key"))
	if scope.MediaScopeKey == "" {
		scope.MediaScopeKey = scope.StableKey()
	}
	return scope
}

func (s Scope) StableKey() string {
	if key := cleanScopeValue(s.MediaScopeKey); key != "" {
		return key
	}
	root := scopePath(s.RootProjectPath)
	project := scopePath(s.ProjectPath)
	worktree := cleanScopeValue(s.ActiveWorktree)
	branch := cleanScopeValue(s.ActiveBranch)
	if root != "" {
		return strings.Join([]string{"root", root, firstScopeNonEmpty(worktree, "main"), firstScopeNonEmpty(branch, "main")}, "::")
	}
	if project != "" {
		return "project::" + project
	}
	if history := cleanScopeValue(s.HistoryScopeKey); history != "" {
		return "history::" + history
	}
	return ""
}

func (s Scope) Empty() bool {
	return s.StableKey() == "" &&
		scopePath(s.ProjectPath) == "" &&
		scopePath(s.RootProjectPath) == "" &&
		cleanScopeValue(s.HistoryScopeKey) == ""
}

func ApplyScope(a Artifact, scope Scope) Artifact {
	if scope.Empty() {
		return a
	}
	a.ProjectPath = scopePath(scope.ProjectPath)
	a.RootProjectPath = scopePath(scope.RootProjectPath)
	a.ActiveWorktree = cleanScopeValue(scope.ActiveWorktree)
	a.ActiveBranch = cleanScopeValue(scope.ActiveBranch)
	a.ActiveNodeID = cleanScopeValue(scope.ActiveNodeID)
	a.HistoryScopeKey = cleanScopeValue(scope.HistoryScopeKey)
	a.MediaScopeKey = scope.StableKey()
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}
	for key, value := range map[string]string{
		"project_path":      a.ProjectPath,
		"root_project_path": a.RootProjectPath,
		"active_worktree":   a.ActiveWorktree,
		"active_branch":     a.ActiveBranch,
		"active_node_id":    a.ActiveNodeID,
		"history_scope_key": a.HistoryScopeKey,
		"media_scope_key":   a.MediaScopeKey,
	} {
		if value != "" {
			a.Metadata[key] = value
		}
	}
	return a
}

func FilterByScope(items []Artifact, scope Scope) []Artifact {
	if len(items) == 0 || scope.Empty() {
		return items
	}
	key := scope.StableKey()
	out := make([]Artifact, 0, len(items))
	for _, item := range items {
		if artifactMatchesScope(item, scope, key) {
			out = append(out, item)
		}
	}
	return out
}

func RetagScope(store Store, from, to Scope) (int, error) {
	if from.Empty() || to.Empty() {
		return 0, nil
	}
	items, err := store.List()
	if err != nil {
		return 0, err
	}
	fromKey := from.StableKey()
	count := 0
	for _, item := range items {
		if !artifactMatchesScope(item, from, fromKey) {
			continue
		}
		if _, err := store.Upsert(ApplyScope(item, to)); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func artifactMatchesScope(a Artifact, scope Scope, key string) bool {
	if scopePath(a.ProjectPath) == "" && scopePath(a.RootProjectPath) == "" && cleanScopeValue(a.HistoryScopeKey) == "" {
		return false
	}
	root := scopePath(scope.RootProjectPath)
	itemRoot := scopePath(a.RootProjectPath)
	if root != "" && itemRoot != "" {
		if itemRoot != root {
			return false
		}
		if !scopeFieldCompatible(a.ActiveWorktree, scope.ActiveWorktree) {
			return false
		}
		if !scopeFieldCompatible(a.ActiveBranch, scope.ActiveBranch) {
			return false
		}
		return true
	}
	if key != "" && cleanScopeValue(a.MediaScopeKey) != "" && cleanScopeValue(a.MediaScopeKey) == key {
		return true
	}
	if project := scopePath(scope.ProjectPath); project != "" && scopePath(a.ProjectPath) == project {
		return true
	}
	if history := cleanScopeValue(scope.HistoryScopeKey); history != "" {
		return cleanScopeValue(a.HistoryScopeKey) == history
	}
	return false
}

func scopeFieldCompatible(item, wanted string) bool {
	item = cleanScopeValue(item)
	wanted = cleanScopeValue(wanted)
	return item == "" || wanted == "" || item == wanted
}

func firstScopeString(args map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := cleanScopeValue(fmt.Sprint(args[key])); value != "" {
			return value
		}
	}
	return ""
}

func cleanScopeValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" || value == "null" || value == "undefined" {
		return ""
	}
	return value
}

func scopePath(value string) string {
	value = cleanScopeValue(value)
	if value == "" {
		return ""
	}
	clean := filepath.Clean(value)
	return filepath.ToSlash(clean)
}

func firstScopeNonEmpty(values ...string) string {
	for _, value := range values {
		if value = cleanScopeValue(value); value != "" {
			return value
		}
	}
	return ""
}
