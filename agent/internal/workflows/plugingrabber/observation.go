package plugingrabber

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// BuildParameterDigest converts a live, read-only parameter response into the
// compact form shared by context explanation and deterministic EQ planning.
func BuildParameterDigest(reply map[string]any) ParameterDigest {
	params := mapRowsValue(reply["parameters"])
	digest := ParameterDigest{
		TrackID:                   firstNonEmptyText(reply, "track_id"),
		PluginID:                  firstNonEmptyText(reply, "plugin_id"),
		PluginName:                firstNonEmptyText(reply, "plugin_name"),
		PluginIdentity:            mapValue(reply["plugin_identity"]),
		TemplateRole:              firstNonEmptyText(reply, "template_role"),
		PluginClass:               firstNonEmptyText(reply, "plugin_class"),
		CurrentParamSignatureHash: firstNonEmptyText(reply, "current_param_signature_hash"),
		ParameterCount:            len(params),
		Parameters:                make([]ParameterInfo, 0, len(params)),
	}
	for _, row := range mapRowsValue(reply["quick_controls"]) {
		digest.QuickControls = append(digest.QuickControls, QuickControlDigest{
			ParamID:        firstNonEmptyText(row, "param_id", "id"),
			Label:          firstNonEmptyText(row, "label", "name"),
			Widget:         firstNonEmptyText(row, "widget"),
			DisplayGroup:   firstNonEmptyText(row, "display_group"),
			NormalizedRole: firstNonEmptyText(row, "normalized_role"),
		})
	}
	for _, row := range mapRowsValue(reply["recommended_groups"]) {
		ids := stringSliceValue(row["parameter_ids"])
		digest.RecommendedGroups = append(digest.RecommendedGroups, RecommendedGroupInfo{
			Name:           firstNonEmptyText(row, "name", "group"),
			ParameterCount: len(ids),
			SampleIDs:      firstStringLimit(ids, 8),
		})
	}
	for _, row := range params {
		id := firstNonEmptyText(row, "id", "param_id", "raw_param_id")
		if id == "" {
			continue
		}
		param := ParameterInfo{
			ID:               id,
			Name:             firstNonEmptyText(row, "name", "label"),
			RawName:          firstNonEmptyText(row, "raw_param_name", "raw_name"),
			Alias:            firstNonEmptyText(row, "alias"),
			DisplayGroup:     firstNonEmptyText(row, "display_group"),
			NormalizedRole:   firstNonEmptyText(row, "normalized_role"),
			ControlRelevance: firstNonEmptyText(row, "control_relevance"),
			HostControllable: boolValue(row["host_controllable"]),
			IsBoolean:        boolValue(row["is_boolean"]),
			IsDiscrete:       boolValue(row["is_discrete"]),
			NumSteps:         intNumber(row["num_steps"]),
			Unit:             firstNonEmptyText(row, "unit", "label_unit"),
			Value:            row["value"],
			NormalizedValue:  row["normalized_value"],
			ValueText:        firstNonEmptyText(row, "value_text"),
			Min:              row["min"],
			Max:              row["max"],
			DisplayProbe:     displayProbeFromAny(row["display_probe"]),
		}
		param.DisplayDomainCandidate = DisplayDomainCandidateForParameter(param)
		digest.Parameters = append(digest.Parameters, param)
	}
	return digest
}

func stringSliceValue(value any) []string {
	switch x := value.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func mapValue(value any) map[string]any {
	switch x := value.(type) {
	case map[string]any:
		return x
	case map[string]string:
		out := make(map[string]any, len(x))
		for key, item := range x {
			out[key] = item
		}
		return out
	default:
		return nil
	}
}

func intNumber(value any) int {
	switch x := value.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	default:
		return 0
	}
}

func floatNumber(value any) float64 {
	switch x := value.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		var out float64
		if _, err := fmt.Sscanf(strings.TrimSpace(x), "%f", &out); err == nil {
			return out
		}
	}
	return 0
}

func normaliseParameterSlot(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "parameter"
	}
	if out[0] >= '0' && out[0] <= '9' {
		return "parameter_" + out
	}
	return out
}
