package artifacts

import (
	"errors"
	"fmt"
	"strings"
)

func ListCommand(store Store, args map[string]any) (map[string]any, error) {
	items, err := store.List()
	if err != nil {
		return nil, err
	}
	items = FilterByScope(items, ScopeFromMap(args))
	if !boolValue(args["include_internal"], false) {
		items = FilterUserVisible(items)
	}
	if conversationID := stringValue(args["conversation_id"]); conversationID != "" {
		filtered := items[:0]
		for _, item := range items {
			if item.ConversationID == conversationID {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	for _, filter := range []struct {
		argKey      string
		metadataKey string
	}{
		{"kind", ""},
		{"source", ""},
		{"artifact_schema", "artifact_schema"},
	} {
		value := stringValue(args[filter.argKey])
		if value == "" {
			continue
		}
		filtered := items[:0]
		for _, item := range items {
			if artifactMatchesFilter(item, filter.argKey, filter.metadataKey, value) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	limit := intValue(args["limit"], 50)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return map[string]any{
		"status":    "ok",
		"artifacts": Summaries(items),
		"root":      store.Root,
	}, nil
}

func artifactMatchesFilter(item Artifact, argKey, metadataKey, value string) bool {
	switch argKey {
	case "kind":
		return strings.EqualFold(strings.TrimSpace(item.Kind), value)
	case "source":
		return strings.EqualFold(strings.TrimSpace(item.Source), value)
	}
	if metadataKey == "" || item.Metadata == nil {
		return false
	}
	return stringValue(item.Metadata[metadataKey]) == value
}

func FilterUserVisible(items []Artifact) []Artifact {
	if len(items) == 0 {
		return items
	}
	filtered := items[:0]
	for _, item := range items {
		if IsInternal(item) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func IsInternal(a Artifact) bool {
	if a.Metadata == nil {
		return false
	}
	for _, key := range []string{"internal", "hidden", "hide_from_media_pool", "hidden_from_media_pool"} {
		if boolValue(a.Metadata[key], false) {
			return true
		}
	}
	return false
}

func ReadCommand(store Store, args map[string]any) (map[string]any, error) {
	id := stringValue(args["artifact_id"])
	if id == "" {
		id = stringValue(args["id"])
	}
	if id == "" {
		return nil, errors.New("artifact_id is required")
	}
	a, err := store.Get(id)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":   "ok",
		"artifact": a,
	}, nil
}

func RenameCommand(store Store, args map[string]any) (map[string]any, error) {
	id := stringValue(args["artifact_id"])
	if id == "" {
		id = stringValue(args["id"])
	}
	if id == "" {
		return nil, errors.New("artifact_id is required")
	}
	title := firstStringValue(args, "title", "name")
	a, err := store.Rename(id, title)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":   "ok",
		"artifact": a,
	}, nil
}

func DeleteCommand(store Store, args map[string]any) (map[string]any, error) {
	id := stringValue(args["artifact_id"])
	if id == "" {
		id = stringValue(args["id"])
	}
	if id == "" {
		return nil, errors.New("artifact_id is required")
	}
	a, err := store.Delete(id)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":   "ok",
		"artifact": a.CompactSummary(),
	}, nil
}

func ExtractCommand(store Store, args map[string]any) (map[string]any, error) {
	id := stringValue(args["artifact_id"])
	if id == "" {
		id = stringValue(args["id"])
	}
	if id == "" {
		path := firstStringValue(args, "file_path", "path", "absolute_path")
		if path == "" {
			return nil, errors.New("artifact_id or file_path is required")
		}
		id = artifactIDFromPath(path)
		return extractPathCommand(store, args, id, path)
	}
	a, err := store.Get(id)
	if err != nil {
		path := firstStringValue(args, "file_path", "path", "absolute_path")
		if path == "" {
			return nil, err
		}
		return extractPathCommand(store, args, id, path)
	}
	a = Extract(a, intValue(args["max_text_runes"], DefaultTextLimit))
	a, err = store.Upsert(a)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":   "ok",
		"artifact": a,
	}, nil
}

func extractPathCommand(store Store, args map[string]any, id, path string) (map[string]any, error) {
	a := ArtifactFromFile(path, id, stringValue(args["conversation_id"]), stringValue(args["goal_id"]), stringValue(args["run_id"]))
	a = ApplyScope(a, ScopeFromMap(args))
	if kind := stringValue(args["kind"]); kind != "" {
		a.Kind = kind
	}
	a = Extract(a, intValue(args["max_text_runes"], DefaultTextLimit))
	a, err := store.Upsert(a)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":   "ok",
		"artifact": a,
	}, nil
}

func firstStringValue(args map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(args[key]); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func artifactIDFromPath(path string) string {
	id := strings.Trim(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, strings.TrimSpace(path)), "_")
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, "art_") {
		return id
	}
	return "art_" + id
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
	case jsonNumber:
		i, err := n.Int64()
		if err == nil {
			return int(i)
		}
	}
	return fallback
}

func boolValue(v any, fallback bool) bool {
	switch value := v.(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "on", "include", "included":
			return true
		case "0", "false", "no", "off", "exclude", "excluded":
			return false
		}
	case int:
		return value != 0
	case int64:
		return value != 0
	case float64:
		return value != 0
	}
	return fallback
}

type jsonNumber interface {
	Int64() (int64, error)
}
