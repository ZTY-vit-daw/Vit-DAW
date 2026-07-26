package plugingrabber

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const profileInvalidIDListLimit = 10

func BuildParameterDigest(reply map[string]any) ParameterDigest {
	identity, _ := reply["plugin_identity"].(map[string]any)
	globalProfile := mapValue(reply["global_profile"])
	pluginSkill := mapValue(reply["plugin_skill"])
	if pluginSkill == nil && globalProfile != nil {
		pluginSkill = mapValue(globalProfile["plugin_skill"])
	}
	pluginGroups := mapRowsValue(reply["plugin_groups"])
	if len(pluginGroups) == 0 && globalProfile != nil {
		pluginGroups = mapRowsValue(globalProfile["groups"])
	}
	virtualControls := mapRowsValue(reply["virtual_controls"])
	if len(virtualControls) == 0 && globalProfile != nil {
		virtualControls = mapRowsValue(globalProfile["virtual_controls"])
	}
	safetyLimits := mapValue(reply["safety_limits"])
	if safetyLimits == nil && globalProfile != nil {
		safetyLimits = mapValue(globalProfile["safety"])
	}
	safetyLimits = SanitizeProfileSafety(safetyLimits)
	pluginClass := firstNonEmptyText(reply, "plugin_class")
	if pluginClass == "" && globalProfile != nil {
		pluginClass = firstNonEmptyText(globalProfile, "class")
	}
	params := mapRowsValue(reply["parameters"])
	digest := ParameterDigest{
		TrackID:                   firstNonEmptyText(reply, "track_id"),
		PluginID:                  firstNonEmptyText(reply, "plugin_id"),
		PluginName:                firstNonEmptyText(identity, "plugin_name"),
		PluginIdentity:            identity,
		TemplateRole:              firstNonEmptyText(reply, "template_role"),
		ProfileSource:             firstNonEmptyText(reply, "profile_source"),
		ProfileApplied:            boolValue(reply["profile_applied"]),
		ProfileStaleParamIDs:      stringSliceValue(reply["profile_stale_param_ids"]),
		GlobalProfileApplied:      boolValue(reply["global_profile_applied"]),
		GlobalProfileSource:       firstNonEmptyText(reply, "global_profile_source"),
		GlobalProfile:             globalProfile,
		PluginClass:               pluginClass,
		PluginGroups:              pluginGroups,
		VirtualControls:           virtualControls,
		CurrentParamSignatureHash: firstNonEmptyText(reply, "current_param_signature_hash"),
		ProfileParamSignatureHash: firstNonEmptyText(reply, "profile_param_signature_hash"),
		SafetyLimits:              safetyLimits,
		PluginSkill:               pluginSkill,
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
		displayProbe := displayProbeFromAny(row["display_probe"])
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
			DisplayProbe:     displayProbe,
		}
		param.DisplayDomainCandidate = DisplayDomainCandidateForParameter(param)
		digest.Parameters = append(digest.Parameters, param)
	}
	return digest
}

func ValidateProfilePatch(patch ProfilePatch, digest ParameterDigest) (ProfilePatch, error) {
	validIDs := map[string]bool{}
	for _, param := range digest.Parameters {
		validIDs[param.ID] = true
	}
	var invalid []string
	quick := cleanIDList(patch.QuickControlIDs, validIDs, &invalid)
	aliases := cleanPatchMap(patch.Aliases, validIDs, &invalid)
	groups := cleanPatchMap(patch.DisplayGroups, validIDs, &invalid)
	roles := cleanPatchMap(patch.NormalizedRoles, validIDs, &invalid)
	if len(invalid) > 0 {
		sort.Strings(invalid)
		return ProfilePatch{}, fmt.Errorf("AI proposed unknown plugin parameter IDs: %s", strings.Join(firstStringLimit(invalid, profileInvalidIDListLimit), ", "))
	}
	if len(quick) == 0 && len(aliases) == 0 && len(groups) == 0 && len(roles) == 0 {
		return ProfilePatch{}, fmt.Errorf("AI did not propose any plugin grabber profile changes")
	}
	patch.QuickControlIDs = quick
	patch.Aliases = aliases
	patch.DisplayGroups = groups
	patch.NormalizedRoles = roles
	patch.Safety = SanitizeProfileSafety(patch.Safety)
	return patch, nil
}

func BuildProfileUpsertCommand(target LearningTarget, patch ProfilePatch) map[string]any {
	cmd := map[string]any{
		"cmd":       "plugin_grabber_upsert_project_profile",
		"track_id":  target.TrackID,
		"plugin_id": target.PluginID,
	}
	cmd["global"] = true

	if len(patch.QuickControlIDs) > 0 {
		cmd["quick_control_ids"] = patch.QuickControlIDs
	}
	if len(patch.Aliases) > 0 {
		cmd["aliases"] = patch.Aliases
	}
	if len(patch.DisplayGroups) > 0 {
		cmd["display_groups"] = patch.DisplayGroups
	}
	if len(patch.NormalizedRoles) > 0 {
		cmd["normalized_roles"] = patch.NormalizedRoles
	}
	if strings.TrimSpace(patch.Class) != "" {
		cmd["class"] = strings.TrimSpace(patch.Class)
	}
	if len(patch.Groups) > 0 {
		cmd["groups"] = patch.Groups
	}
	if len(patch.VirtualControls) > 0 {
		cmd["virtual_controls"] = patch.VirtualControls
	}
	if safety := SanitizeProfileSafety(patch.Safety); len(safety) > 0 {
		cmd["safety"] = safety
	}
	return cmd
}

func ParseProfilePatch(raw string) (ProfilePatch, error) {
	var patch ProfilePatch
	text := strings.TrimSpace(raw)
	if err := json.Unmarshal([]byte(text), &patch); err == nil {
		if nested := nestedProfilePatch(text); nested != nil {
			return *nested, nil
		}
		return patch, nil
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		snippet := text[start : end+1]
		if nested := nestedProfilePatch(snippet); nested != nil {
			return *nested, nil
		}
		if err := json.Unmarshal([]byte(snippet), &patch); err == nil {
			return patch, nil
		}
	}
	return ProfilePatch{}, fmt.Errorf("AI returned invalid plugin profile patch JSON")
}

func nestedProfilePatch(text string) *ProfilePatch {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &outer); err != nil {
		return nil
	}
	for _, key := range []string{"profile_patch", "patch", "project_default"} {
		if raw, ok := outer[key]; ok {
			var patch ProfilePatch
			if err := json.Unmarshal(raw, &patch); err == nil {
				if replyRaw, ok := outer["reply"]; ok && patch.Reply == "" {
					_ = json.Unmarshal(replyRaw, &patch.Reply)
				}
				return &patch
			}
		}
	}
	return nil
}

func cleanIDList(values []string, validIDs map[string]bool, invalid *[]string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if !validIDs[id] {
			*invalid = append(*invalid, id)
			continue
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func cleanPatchMap(values map[string]string, validIDs map[string]bool, invalid *[]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range values {
		id := strings.TrimSpace(key)
		text := strings.TrimSpace(value)
		if id == "" || text == "" {
			continue
		}
		if !validIDs[id] {
			*invalid = append(*invalid, id)
			continue
		}
		out[id] = text
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
		for key, value := range x {
			out[key] = value
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
