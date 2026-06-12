package macrocontrols

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Kind = "macro_panel"

type Panel struct {
	Kind      string    `json:"kind"`
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	TrackID   string    `json:"track_id,omitempty"`
	PluginID  string    `json:"plugin_id,omitempty"`
	Source    string    `json:"source,omitempty"`
	Controls  []Control `json:"controls,omitempty"`
	CreatedAt string    `json:"created_at,omitempty"`
}

type Control struct {
	ID             string         `json:"id"`
	Label          string         `json:"label,omitempty"`
	Type           string         `json:"type"`
	Min            float64        `json:"min,omitempty"`
	Max            float64        `json:"max,omitempty"`
	Default        any            `json:"default,omitempty"`
	Unit           string         `json:"unit,omitempty"`
	Control        string         `json:"control,omitempty"`
	TargetTemplate map[string]any `json:"target_template,omitempty"`
}

func NormalizeList(raw any) []map[string]any {
	rows := macroRows(raw)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		normalized := NormalizeControl(row)
		if strings.TrimSpace(fmt.Sprint(normalized["macro_id"])) == "" {
			continue
		}
		out = append(out, normalized)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(fmt.Sprint(out[i]["name"])))
		right := strings.ToLower(strings.TrimSpace(fmt.Sprint(out[j]["name"])))
		if left == right {
			return strings.TrimSpace(fmt.Sprint(out[i]["macro_id"])) < strings.TrimSpace(fmt.Sprint(out[j]["macro_id"]))
		}
		return left < right
	})
	return out
}

func NormalizeControl(raw map[string]any) map[string]any {
	id := firstText(raw, "macro_id", "id", "control_id")
	minValue := numberValue(firstPresent(raw, "min", "minimum"), 0)
	maxValue := numberValue(firstPresent(raw, "max", "maximum"), 1)
	if maxValue <= minValue {
		minValue = 0
		maxValue = 1
	}
	value := clamp(numberValue(firstPresent(raw, "value", "default"), minValue), minValue, maxValue)
	bindings := normalizeBindings(firstPresent(raw, "bindings", "targets"))
	status := strings.ToLower(firstText(raw, "status", "state"))
	if status == "" {
		status = "ready"
	}
	source := firstText(raw, "source", "created_by")
	if source == "" {
		source = "godot_rack"
	}
	out := map[string]any{
		"macro_id":      id,
		"id":            id,
		"name":          firstNonEmpty(firstText(raw, "name", "label", "title"), id),
		"track_id":      firstText(raw, "track_id", "track"),
		"plugin_id":     firstText(raw, "plugin_id", "plugin", "node_id"),
		"control_type":  firstNonEmpty(firstText(raw, "control_type", "type"), "slider"),
		"value":         value,
		"min":           minValue,
		"max":           maxValue,
		"unit":          firstText(raw, "unit"),
		"bindings":      bindings,
		"binding_count": len(bindings),
		"source":        source,
		"status":        status,
		"enabled":       boolValue(firstPresent(raw, "enabled"), true),
	}
	for _, key := range []string{"created_at", "updated_at", "created_at_unix", "updated_at_unix", "position"} {
		if value, ok := raw[key]; ok {
			out[key] = value
		}
	}
	return out
}

func NewPanel(title, trackID, pluginID, source string) Panel {
	return Panel{
		Kind:      Kind,
		ID:        "macro_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "_"),
		Title:     strings.TrimSpace(title),
		TrackID:   strings.TrimSpace(trackID),
		PluginID:  strings.TrimSpace(pluginID),
		Source:    strings.TrimSpace(source),
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func macroRows(raw any) []map[string]any {
	switch value := raw.(type) {
	case []map[string]any:
		return value
	case []any:
		out := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	case map[string]any:
		for _, key := range []string{"macros", "macro_controls", "controls", "items"} {
			if rows := macroRows(value[key]); len(rows) > 0 {
				return rows
			}
		}
		if firstText(value, "macro_id", "id", "control_id") != "" {
			return []map[string]any{value}
		}
	}
	return nil
}

func normalizeBindings(raw any) []map[string]any {
	switch value := raw.(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(value))
		for _, row := range value {
			out = append(out, normalizeBinding(row))
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if row, ok := item.(map[string]any); ok {
				out = append(out, normalizeBinding(row))
			}
		}
		return out
	}
	return []map[string]any{}
}

func normalizeBinding(raw map[string]any) map[string]any {
	out := map[string]any{
		"binding_id": firstText(raw, "binding_id", "id"),
		"track_id":   firstText(raw, "track_id", "track"),
		"plugin_id":  firstText(raw, "plugin_id", "plugin", "node_id"),
		"param_id":   firstText(raw, "param_id", "parameter_id", "key"),
		"param_name": firstText(raw, "param_name", "parameter_name", "label", "name"),
		"source_min": numberValue(firstPresent(raw, "source_min"), 0),
		"source_max": numberValue(firstPresent(raw, "source_max"), 1),
		"target_min": numberValue(firstPresent(raw, "target_min", "min"), 0),
		"target_max": numberValue(firstPresent(raw, "target_max", "max"), 1),
		"enabled":    boolValue(firstPresent(raw, "enabled"), true),
	}
	for _, key := range []string{"unit", "value_text", "param_path"} {
		if value, ok := raw[key]; ok {
			out[key] = value
		}
	}
	return out
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return nil
}

func firstText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := row[key]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func numberValue(raw any, fallback float64) float64 {
	switch value := raw.(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fallback
		}
		return value
	case float32:
		return numberValue(float64(value), fallback)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case jsonNumber:
		if n, err := value.Float64(); err == nil {
			return n
		}
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return n
		}
	}
	return fallback
}

type jsonNumber interface {
	Float64() (float64, error)
}

func boolValue(raw any, fallback bool) bool {
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1", "yes", "on", "enabled":
			return true
		case "false", "0", "no", "off", "disabled":
			return false
		}
	}
	return fallback
}

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
