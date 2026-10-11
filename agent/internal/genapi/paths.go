package genapi

import (
	"strings"
)

// lookupPath resolves a dotted JSON path ("error.message") against a decoded
// document. Object keys only; no array indexing (use [] collection forms).
func lookupPath(doc map[string]any, path string) (any, bool) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	var current any = doc
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// stringAt returns the string value at a (possibly dotted) path.
func stringAt(doc map[string]any, field string) string {
	if field == "" {
		return ""
	}
	if v, ok := lookupPath(doc, field); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// firstStringPath returns the first non-empty string among paths.
func firstStringPath(doc map[string]any, paths ...string) (string, bool) {
	for _, p := range paths {
		if s := stringAt(doc, p); strings.TrimSpace(s) != "" {
			return s, true
		}
	}
	return "", false
}

// taskIDFrom extracts the task identifier from a submit reply. Bare string
// documents and {"data":"<id>"} relay envelopes are both accepted.
func taskIDFrom(doc map[string]any, fields []string) string {
	for _, f := range fields {
		if s := stringAt(doc, f); strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// collectStrings gathers values for field patterns across a document.
// Pattern forms:
//
//	audio_url          object key
//	data.audio_url     nested object key
//	data[].audio_url   the same key on every element of the data array
//	output             the value itself when it is a string, or every
//	                   element when it is an array of strings
func collectStrings(doc map[string]any, fields []string) []string {
	var out []string
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			continue
		}
		if idx := strings.Index(field, "[]"); idx >= 0 {
			arrayPath := field[:idx]
			member := strings.TrimPrefix(field[idx+2:], ".")
			var arr any = doc
			if arrayPath != "" {
				v, ok := lookupPath(doc, arrayPath)
				if !ok {
					continue
				}
				arr = v
			}
			if list, ok := arr.([]any); ok {
				for _, element := range list {
					m, ok := element.(map[string]any)
					if !ok {
						continue
					}
					if s := stringAt(m, member); strings.TrimSpace(s) != "" {
						out = append(out, strings.TrimSpace(s))
					}
				}
			}
			continue
		}
		v, ok := lookupPath(doc, field)
		if !ok {
			continue
		}
		switch typed := v.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				out = append(out, strings.TrimSpace(typed))
			}
		case []any:
			for _, element := range typed {
				if s, ok := element.(string); ok && strings.TrimSpace(s) != "" {
					out = append(out, strings.TrimSpace(s))
				}
			}
		}
	}
	return dedupePreserveOrder(out)
}

// titleNear finds a plausible artifact title near the harvested URL field
// (same object), used for artifact labels.
func titleNear(doc map[string]any, field string) string {
	if idx := strings.Index(field, "[]."); idx >= 0 {
		arrayPath := field[:idx]
		member := strings.TrimPrefix(field[idx+3:], ".")
		if member == "url" || member == "audio_url" {
			// derive from the sibling fields of the same array elements
			var arr any = doc
			if arrayPath != "" {
				if v, ok := lookupPath(doc, arrayPath); ok {
					arr = v
				}
			}
			if list, ok := arr.([]any); ok && len(list) > 0 {
				if m, ok := list[0].(map[string]any); ok {
					for _, candidate := range []string{"title", "name", "id"} {
						if s := stringAt(m, candidate); strings.TrimSpace(s) != "" {
							return strings.TrimSpace(s)
						}
					}
				}
			}
		}
		return ""
	}
	if field == "url" || field == "audio_url" || strings.HasSuffix(field, ".url") || strings.HasSuffix(field, ".audio_url") {
		for _, candidate := range []string{"title", "name"} {
			if s := stringAt(doc, candidate); strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// durationNear finds a reported duration near the artifacts, if any.
func durationNear(doc map[string]any) float64 {
	for _, field := range []string{"duration", "duration_seconds", "metadata.duration", "data[].metadata.duration"} {
		if idx := strings.Index(field, "[]."); idx >= 0 {
			arrayPath := field[:idx]
			member := strings.TrimPrefix(field[idx+3:], ".")
			var arr any = doc
			if arrayPath != "" {
				if v, ok := lookupPath(doc, arrayPath); ok {
					arr = v
				}
			}
			if list, ok := arr.([]any); ok && len(list) > 0 {
				if m, ok := list[0].(map[string]any); ok {
					if f := numberAt(m, member); f > 0 {
						return f
					}
				}
			}
			continue
		}
		if f := numberAt(doc, field); f > 0 {
			return f
		}
	}
	return 0
}

func numberAt(doc map[string]any, field string) float64 {
	v, ok := lookupPath(doc, field)
	if !ok {
		return 0
	}
	switch typed := v.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	}
	return 0
}

func dedupePreserveOrder(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
