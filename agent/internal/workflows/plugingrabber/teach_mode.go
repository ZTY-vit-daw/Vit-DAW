package plugingrabber

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	teachModeSourceUserDemonstrated = "user_demonstrated"
	teachModeDiffEpsilon            = 0.000001
)

func BuildParameterSnapshot(digest ParameterDigest, capturedAt string) PluginParameterSnapshot {
	if strings.TrimSpace(capturedAt) == "" {
		capturedAt = time.Now().UTC().Format(time.RFC3339)
	}
	values := make([]PluginParameterSnapshotValue, 0, len(digest.Parameters))
	for _, param := range digest.Parameters {
		id := strings.TrimSpace(param.ID)
		if id == "" {
			continue
		}
		normalized := floatNumber(param.NormalizedValue)
		values = append(values, PluginParameterSnapshotValue{
			ParamID:         id,
			Name:            parameterLabel(param, id),
			NormalizedValue: normalized,
			ValueText:       param.ValueText,
		})
	}
	return PluginParameterSnapshot{
		CapturedAt: capturedAt,
		TrackID:    digest.TrackID,
		PluginID:   digest.PluginID,
		PluginName: digest.PluginName,
		Parameters: values,
	}
}

func DiffParameterSnapshots(before, after PluginParameterSnapshot) []PluginParameterChange {
	beforeByID := map[string]PluginParameterSnapshotValue{}
	for _, value := range before.Parameters {
		if strings.TrimSpace(value.ParamID) != "" {
			beforeByID[value.ParamID] = value
		}
	}
	changes := []PluginParameterChange{}
	for _, next := range after.Parameters {
		prev, ok := beforeByID[next.ParamID]
		if !ok {
			continue
		}
		delta := next.NormalizedValue - prev.NormalizedValue
		if math.Abs(delta) <= teachModeDiffEpsilon {
			continue
		}
		changes = append(changes, PluginParameterChange{
			ParamID:          next.ParamID,
			Name:             firstNonEmptyString(next.Name, prev.Name),
			BeforeNormalized: prev.NormalizedValue,
			AfterNormalized:  next.NormalizedValue,
			Delta:            delta,
		})
	}
	sort.SliceStable(changes, func(i, j int) bool {
		return changes[i].ParamID < changes[j].ParamID
	})
	return changes
}

func BuildTeachModeProfilePatch(digest ParameterDigest, rows []TeachModeRowRecord) (ProfilePatch, map[string]any, error) {
	capturedRows := []TeachModeRowRecord{}
	for _, row := range rows {
		if row.State != TeachModeRowCaptured {
			continue
		}
		if len(row.Changes) == 0 && row.BeforeSnapshot != nil && row.AfterSnapshot != nil {
			row.Changes = DiffParameterSnapshots(*row.BeforeSnapshot, *row.AfterSnapshot)
		}
		if len(row.Changes) == 0 {
			continue
		}
		capturedRows = append(capturedRows, row)
	}
	if len(capturedRows) == 0 {
		return ProfilePatch{}, nil, fmt.Errorf("teach mode needs at least one learned row with parameter changes")
	}

	validIDs := map[string]ParameterInfo{}
	for _, param := range digest.Parameters {
		if strings.TrimSpace(param.ID) != "" {
			validIDs[param.ID] = param
		}
	}

	quickIDs := []string{}
	aliases := map[string]string{}
	displayGroups := map[string]string{}
	roles := map[string]string{}
	groups := []map[string]any{}
	operations := []map[string]any{}
	warnings := []string{}

	for idx, row := range capturedRows {
		componentID := teachComponentID(row, idx)
		label := firstNonEmptyString(strings.TrimSpace(row.Description), fmt.Sprintf("Demonstration %d", idx+1))
		displayDomain := row.DisplayDomain
		if displayDomain == nil {
			displayDomain = parseDisplayDomainText(row.DisplayDomainText, "teach_mode_user_input", true)
		}
		params := map[string]any{}
		opParams := map[string]any{}
		for changeIndex, change := range row.Changes {
			paramID := strings.TrimSpace(change.ParamID)
			param, ok := validIDs[paramID]
			if !ok {
				warnings = append(warnings, fmt.Sprintf("%s references missing param_id %s", label, paramID))
				continue
			}
			slot := fmt.Sprintf("param_%d", changeIndex+1)
			mapping := map[string]any{
				"param_id":   paramID,
				"label":      firstNonEmptyString(change.Name, parameterLabel(param, paramID)),
				"source":     teachModeSourceUserDemonstrated,
				"confidence": 1.0,
				"confirmed":  true,
				"status":     PluginSkillStatusActive,
				"locked":     true,
				"evidence": []map[string]any{{
					"kind": "demonstration_diff",
					"summary": fmt.Sprintf("User demonstrated %.6f -> %.6f normalized (delta %.6f).",
						change.BeforeNormalized, change.AfterNormalized, change.Delta),
				}},
			}
			if domainMap := displayDomainMap(displayDomain); len(domainMap) > 0 {
				mapping["display_domain"] = domainMap
				mapping["display_domain_text"] = displayDomainTextForSummary(displayDomain)
			}
			params[slot] = mapping
			opParams[slot] = mapping
			quickIDs = appendUniqueString(quickIDs, paramID)
			aliases[paramID] = firstNonEmptyString(change.Name, parameterLabel(param, paramID))
			displayGroups[paramID] = "User demonstrations"
			roles[paramID] = "user_demonstrated"
		}
		if len(params) == 0 {
			continue
		}
		groups = append(groups, map[string]any{
			"id":     componentID,
			"role":   "user_demonstrated",
			"label":  label,
			"params": params,
		})
		operations = append(operations, map[string]any{
			"name":         teachOperationName(row, idx),
			"inputs":       []string{"amount"},
			"resolver":     "user_demonstrated_delta",
			"component_id": componentID,
			"params":       opParams,
			"changes":      pluginChangesAsMaps(row.Changes),
		})
	}
	if len(groups) == 0 {
		return ProfilePatch{}, nil, fmt.Errorf("teach mode demonstrations did not reference any current plugin parameters")
	}

	patch := ProfilePatch{
		Reply:           fmt.Sprintf("已根据 %d 条教学行为生成 Plugin Grabber 草案。", len(groups)),
		QuickControlIDs: quickIDs,
		Aliases:         aliases,
		DisplayGroups:   displayGroups,
		NormalizedRoles: roles,
		Class:           firstNonEmptyString(digest.PluginClass, digest.TemplateRole, "unknown"),
		Groups:          groups,
		VirtualControls: operations,
		Safety: map[string]any{
			"notes": "Teach Mode mappings are locked user demonstrations and should not be overwritten by automatic learning.",
		},
		TeachModeRows: capturedRows,
	}
	summary := map[string]any{
		"mode":                   string(LearningModeTeach),
		"state":                  "profile_draft",
		"plugin_name":            firstNonEmptyString(digest.PluginName, firstNonEmptyText(digest.PluginIdentity, "plugin_name", "name")),
		"parameter_count":        digest.ParameterCount,
		"row_count":              len(capturedRows),
		"component_count":        len(groups),
		"operations":             operationNames(operations),
		"warnings":               warnings,
		"confidence_summary":     mappingConfidenceSummary(groups),
		"evidence_summary":       mappingEvidenceSummary(groups),
		"missing_evidence":       mappingMissingEvidence(groups),
		"display_domain_summary": displayDomainSummary(groups),
		"profile_preview": map[string]any{
			"class":           patch.Class,
			"components":      groupSummaries(groups),
			"operations":      operationNames(operations),
			"parameter_count": digest.ParameterCount,
		},
	}
	return patch, summary, nil
}

func TeachModeRowsFromAny(value any) []TeachModeRowRecord {
	out := []TeachModeRowRecord{}
	for _, row := range mapRowsValue(value) {
		record := TeachModeRowRecord{
			RowID:             firstNonEmptyText(row, "row_id", "id"),
			Description:       firstNonEmptyText(row, "description", "label", "name"),
			DisplayDomainText: firstNonEmptyText(row, "display_domain_text", "display_range", "display_unit", "unit", "range"),
			DisplayDomain:     displayDomainFromAny(row["display_domain"]),
			State:             TeachModeRowState(firstNonEmptyText(row, "state")),
			Changes:           pluginChangesFromAny(row["changes"]),
		}
		if record.DisplayDomain == nil {
			record.DisplayDomain = parseDisplayDomainText(record.DisplayDomainText, "teach_mode_user_input", true)
		}
		if record.State == "" {
			record.State = TeachModeRowCaptured
		}
		if before := pluginSnapshotFromAny(row["before_snapshot"]); before != nil {
			record.BeforeSnapshot = before
		}
		if after := pluginSnapshotFromAny(row["after_snapshot"]); after != nil {
			record.AfterSnapshot = after
		}
		out = append(out, record)
	}
	return out
}

func teachComponentID(row TeachModeRowRecord, idx int) string {
	seed := firstNonEmptyString(row.RowID, row.Description, fmt.Sprintf("row_%d", idx+1))
	return pluginSkillSlug("teach", seed)
}

func teachOperationName(row TeachModeRowRecord, idx int) string {
	label := firstNonEmptyString(row.Description, fmt.Sprintf("Demonstration %d", idx+1))
	return "teach." + pluginSkillSlug("behavior", label)
}

func pluginSnapshotFromAny(value any) *PluginParameterSnapshot {
	row := mapValue(value)
	if len(row) == 0 {
		return nil
	}
	snapshot := PluginParameterSnapshot{
		CapturedAt: firstNonEmptyText(row, "captured_at"),
		TrackID:    firstNonEmptyText(row, "track_id"),
		PluginID:   firstNonEmptyText(row, "plugin_id"),
		PluginName: firstNonEmptyText(row, "plugin_name"),
	}
	for _, valueRow := range mapRowsValue(row["parameters"]) {
		paramID := firstNonEmptyText(valueRow, "param_id", "id")
		if paramID == "" {
			continue
		}
		snapshot.Parameters = append(snapshot.Parameters, PluginParameterSnapshotValue{
			ParamID:         paramID,
			Name:            firstNonEmptyText(valueRow, "name", "label"),
			NormalizedValue: floatNumber(firstPresent(valueRow, "normalized_value", "value")),
			ValueText:       firstNonEmptyText(valueRow, "value_text"),
		})
	}
	return &snapshot
}

func pluginChangesFromAny(value any) []PluginParameterChange {
	out := []PluginParameterChange{}
	for _, row := range mapRowsValue(value) {
		paramID := firstNonEmptyText(row, "param_id", "id")
		if paramID == "" {
			continue
		}
		out = append(out, PluginParameterChange{
			ParamID:          paramID,
			Name:             firstNonEmptyText(row, "name", "label"),
			BeforeNormalized: floatNumber(firstPresent(row, "before_normalized", "before")),
			AfterNormalized:  floatNumber(firstPresent(row, "after_normalized", "after")),
			Delta:            floatNumber(row["delta"]),
		})
	}
	return out
}

func pluginChangesAsMaps(changes []PluginParameterChange) []map[string]any {
	out := make([]map[string]any, 0, len(changes))
	for _, change := range changes {
		out = append(out, map[string]any{
			"param_id":          change.ParamID,
			"name":              change.Name,
			"before_normalized": change.BeforeNormalized,
			"after_normalized":  change.AfterNormalized,
			"delta":             change.Delta,
		})
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

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
