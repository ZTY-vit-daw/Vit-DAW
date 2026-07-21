package capabilitycontext

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

func mapValue(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case map[string]string:
		out := make(map[string]any, len(v))
		for key, value := range v {
			out[key] = value
		}
		return out
	default:
		data, err := json.Marshal(v)
		if err != nil || len(data) == 0 || string(data) == "null" {
			return nil
		}
		out := map[string]any{}
		if err := json.Unmarshal(data, &out); err != nil || len(out) == 0 {
			return nil
		}
		return out
	}
}

func rowsValue(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if mapped := mapValue(row); len(mapped) > 0 {
				out = append(out, mapped)
			}
		}
		return out
	default:
		return nil
	}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	data, _ := json.Marshal(in)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func cleanText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func firstText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := cleanText(row[key]); text != "" {
			return text
		}
	}
	return ""
}

func firstNumber(row map[string]any, keys ...string) (*float64, bool) {
	for _, key := range keys {
		if value, ok := numberValue(row[key]); ok {
			return &value, true
		}
	}
	return nil, false
}

func numberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return 0, false
		}
		return float64(v), true
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		text := strings.TrimSpace(strings.TrimSuffix(v, "dB"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "db"))
		if text == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(text, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "yes", "1", "on":
			return true
		default:
			return false
		}
	case int:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func dbfsFromAbs(value float64) (*float64, bool) {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, false
	}
	db := round3(20 * math.Log10(value))
	return &db, true
}

func headroomFromPeakDBFS(peak float64) *float64 {
	headroom := round3(-peak)
	return &headroom
}

func capRows[T any](rows []T, limit int) []T {
	if limit > 0 && len(rows) > limit {
		return append([]T(nil), rows[:limit]...)
	}
	return append([]T(nil), rows...)
}

func stringsFromAny(value any, max int, maxRunes int) []string {
	var raw []any
	switch v := value.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text := compactText(item, maxRunes); text != "" {
				out = append(out, text)
			}
		}
		if max > 0 && len(out) > max {
			return out[:max]
		}
		return out
	case []any:
		raw = v
	default:
		if text := compactText(value, maxRunes); text != "" {
			return []string{text}
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if text := compactText(item, maxRunes); text != "" {
			out = append(out, text)
		}
	}
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return out
}

func sortByAbsDesc(rows []RankRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		return math.Abs(rows[i].Value) > math.Abs(rows[j].Value)
	})
}
