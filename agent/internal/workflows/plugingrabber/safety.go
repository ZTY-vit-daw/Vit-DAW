package plugingrabber

import (
	"regexp"
	"strings"
)

var numericSafetyNotePattern = regexp.MustCompile(`(?i)(?:^|[\s,.;:，。；：])(?:up\s*to|maximum|max|min|range|limit|限制|上限|下限|范围|到|至|~)?\s*[-+]?\d+(?:\.\d+)?\s*(?:ms|msec|s|sec|second|hz|khz|db|%)`)

func SanitizeProfileSafety(safety map[string]any) map[string]any {
	if len(safety) == 0 {
		return nil
	}
	out := mapStringAnyCopy(safety)
	if out == nil {
		return nil
	}
	for _, key := range []string{"notes", "note", "description"} {
		text := strings.TrimSpace(firstNonEmptyText(out, key))
		if text == "" {
			continue
		}
		if numericSafetyNotePattern.MatchString(text) {
			out[key] = "数值范围以已确认的显示域和实际控制回包为准。"
			out["numeric_notes_sanitized"] = true
		}
	}
	return out
}
