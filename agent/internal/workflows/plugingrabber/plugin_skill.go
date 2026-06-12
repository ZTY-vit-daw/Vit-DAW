package plugingrabber

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"unicode"
)

const (
	pluginSkillSourceProfilePatch       = "auto_learn_low_confidence"
	pluginSkillDefaultMappingConfidence = 0.65
	pluginSkillRuntimeConfidenceFloor   = 0.70
)

type PluginSkillValidationResult struct {
	Warnings        []string          `json:"warnings,omitempty"`
	Coverage        map[string]string `json:"coverage,omitempty"`
	CoverageSummary map[string]int    `json:"coverage_summary,omitempty"`
}

func BuildPluginSkillDocument(digest ParameterDigest, patch ProfilePatch) PluginSkillDocument {
	projectDefault := legacyProjectDefaultMap(patch)
	identity := buildPluginSkillIdentity(digest, patch)
	components := buildPluginSkillComponents(digest, patch)
	operations := buildPluginSkillOperations(patch)
	safety := SanitizeProfileSafety(patch.Safety)
	if safety == nil {
		safety = map[string]any{}
	}
	flags := map[string]bool{
		"legacy_project_default": true,
		"profile_patch_backed":   true,
	}
	if len(projectDefault) > 0 {
		flags["quick_controls"] = true
	}
	if len(components) > 0 {
		flags["components"] = true
	}
	if len(operations) > 0 {
		flags["operations"] = true
	}
	capabilities := PluginSkillCapabilities{
		Types: pluginSkillCapabilityTypes(identity.PrimaryClass),
		Flags: flags,
	}
	return PluginSkillDocument{
		SchemaVersion: PluginSkillSchemaVersion,
		Identity:      identity,
		Capabilities:  capabilities,
		Components:    components,
		Operations:    operations,
		Safety:        safety,
		Preferences: PluginSkillPreferences{
			Explicit:     pluginSkillExplicitPreferences(patch),
			UsageSummary: map[string]any{},
		},
		Legacy: PluginSkillLegacy{
			ProjectDefault: projectDefault,
			ProfilePatch:   &patch,
		},
	}
}

func ValidatePluginSkillDocument(doc PluginSkillDocument, digest ParameterDigest) (PluginSkillValidationResult, error) {
	result := PluginSkillValidationResult{Coverage: map[string]string{}}
	if doc.SchemaVersion != PluginSkillSchemaVersion {
		return result, fmt.Errorf("plugin skill schema_version must be %d", PluginSkillSchemaVersion)
	}
	validIDs := map[string]bool{}
	for _, param := range digest.Parameters {
		id := strings.TrimSpace(param.ID)
		if id != "" {
			validIDs[id] = true
			result.Coverage[id] = coverageForParameter(param)
		}
	}
	var invalid []string
	eqOperationComponents := eqRuntimeOperationComponents(doc.Operations)
	for _, component := range doc.Components {
		isRuntimeEQComponent := componentLooksLikeEQBand(component) || eqOperationComponents[component.ID]
		if isRuntimeEQComponent {
			if _, ok := component.Params["frequency"]; !ok {
				result.Warnings = append(result.Warnings, component.ID+" missing EQ frequency mapping")
			}
			if _, ok := component.Params["gain"]; !ok {
				result.Warnings = append(result.Warnings, component.ID+" missing EQ gain mapping")
			}
		}
		for slot, mapping := range component.Params {
			paramID := strings.TrimSpace(mapping.ParamID)
			where := firstNonEmptyString(component.ID, "component") + "." + strings.TrimSpace(slot)
			if paramID == "" {
				result.Warnings = append(result.Warnings, where+" has empty param_id")
				continue
			}
			if !validIDs[paramID] {
				invalid = append(invalid, paramID)
				continue
			}
			result.Coverage[paramID] = "used_in_component"
			if strings.TrimSpace(mapping.Source) == "" {
				result.Warnings = append(result.Warnings, where+" missing source")
			}
			if mapping.Confidence <= 0 {
				result.Warnings = append(result.Warnings, where+" missing confidence")
			}
			if isRuntimeEQComponent && isCriticalEQSlot(slot) && !pluginSkillMappingRuntimeEligible(mapping) {
				return result, fmt.Errorf("plugin skill low-confidence EQ runtime mapping cannot be used for automatic apply: %s", where)
			}
		}
		if isRuntimeEQComponent {
			if _, ok := component.Params["frequency"]; !ok {
				return result, fmt.Errorf("plugin skill EQ component %s is missing frequency mapping", component.ID)
			}
			if _, ok := component.Params["gain"]; !ok {
				return result, fmt.Errorf("plugin skill EQ component %s is missing gain mapping", component.ID)
			}
		}
	}
	if len(invalid) > 0 {
		sort.Strings(invalid)
		return result, fmt.Errorf("plugin skill references unknown plugin parameter IDs: %s", strings.Join(firstStringLimit(uniqueStrings(invalid), profileInvalidIDListLimit), ", "))
	}
	result.CoverageSummary = pluginSkillCoverageSummary(result.Coverage)
	sort.Strings(result.Warnings)
	return result, nil
}

func pluginSkillCoverageSummary(coverage map[string]string) map[string]int {
	if len(coverage) == 0 {
		return nil
	}
	out := map[string]int{}
	for _, status := range coverage {
		status = strings.TrimSpace(status)
		if status == "" {
			status = "unknown"
		}
		out[status]++
	}
	return out
}

func BuildPluginSkillUpsertCommand(target LearningTarget, patch ProfilePatch, digest ParameterDigest) (map[string]any, PluginSkillValidationResult, error) {
	skill := BuildPluginSkillDocument(digest, patch)
	validation, err := ValidatePluginSkillDocument(skill, digest)
	if err != nil {
		return nil, validation, err
	}
	cmd := BuildProfileUpsertCommand(target, patch)
	cmd["param_signature_hash"] = skill.Identity.ParamSignatureHash
	cmd["parameter_snapshot"] = map[string]any{
		"track_id":             digest.TrackID,
		"plugin_id":            digest.PluginID,
		"plugin_name":          digest.PluginName,
		"param_signature_hash": skill.Identity.ParamSignatureHash,
		"parameter_count":      digest.ParameterCount,
		"parameters":           digest.Parameters,
	}
	cmd["schema_version"] = PluginSkillSchemaVersion
	cmd["plugin_skill"] = skill
	if len(validation.Warnings) > 0 {
		cmd["plugin_skill_validator_warnings"] = validation.Warnings
	}
	return cmd, validation, nil
}

func buildPluginSkillIdentity(digest ParameterDigest, patch ProfilePatch) PluginSkillIdentity {
	identity := digest.PluginIdentity
	primaryClass := firstNonEmptyString(
		patch.Class,
		firstNonEmptyText(identity, "primary_class", "class"),
		digest.TemplateRole,
	)
	paramHash := firstNonEmptyText(identity, "param_signature_hash")
	if paramHash == "" {
		paramHash = pluginParamSignatureHash(digest.Parameters)
	}
	return PluginSkillIdentity{
		Name:               firstNonEmptyString(digest.PluginName, firstNonEmptyText(identity, "plugin_name", "name")),
		Manufacturer:       firstNonEmptyText(identity, "manufacturer", "maker"),
		Format:             firstNonEmptyText(identity, "plugin_format", "format"),
		Version:            firstNonEmptyText(identity, "version"),
		Path:               firstNonEmptyText(identity, "plugin_path", "path"),
		ProfileKey:         firstNonEmptyText(identity, "profile_key"),
		PluginID:           firstNonEmptyString(digest.PluginID, firstNonEmptyText(identity, "plugin_id")),
		ParamSignatureHash: paramHash,
		PrimaryClass:       pluginSkillSlug("", primaryClass),
	}
}

func buildPluginSkillComponents(digest ParameterDigest, patch ProfilePatch) []PluginSkillComponent {
	paramIndex := map[string]ParameterInfo{}
	for _, param := range digest.Parameters {
		if strings.TrimSpace(param.ID) != "" {
			paramIndex[param.ID] = param
		}
	}
	componentsByID := map[string]*PluginSkillComponent{}
	order := []string{}
	addComponent := func(id, role, label string) *PluginSkillComponent {
		id = pluginSkillSlug("component", id)
		if existing := componentsByID[id]; existing != nil {
			return existing
		}
		component := &PluginSkillComponent{
			ID:     id,
			Role:   pluginSkillSlug("", role),
			Label:  firstNonEmptyString(label, id),
			Params: map[string]PluginSkillParamMap{},
		}
		componentsByID[id] = component
		order = append(order, id)
		return component
	}
	for idx, group := range patch.Groups {
		id := firstNonEmptyText(group, "id", "name", "label")
		if id == "" {
			id = fmt.Sprintf("group_%d", idx+1)
		}
		component := addComponent(id, firstNonEmptyText(group, "role", "class", "name"), firstNonEmptyText(group, "label", "name", "id"))
		for slot, mapping := range componentParamMappings(group["params"], paramIndex) {
			component.Params[slot] = mapping
		}
	}
	quickIDs := patch.QuickControlIDs
	if len(quickIDs) == 0 {
		for _, quick := range digest.QuickControls {
			if strings.TrimSpace(quick.ParamID) != "" {
				quickIDs = append(quickIDs, quick.ParamID)
			}
		}
	}
	covered := map[string]bool{}
	for _, component := range componentsByID {
		for _, mapping := range component.Params {
			covered[mapping.ParamID] = true
		}
	}
	for _, paramID := range quickIDs {
		paramID = strings.TrimSpace(paramID)
		if paramID == "" || covered[paramID] {
			continue
		}
		param := paramIndex[paramID]
		group := firstNonEmptyString(patch.DisplayGroups[paramID], param.DisplayGroup, "Other")
		component := addComponent(group, group, group)
		slot := pluginSkillParamSlot(firstNonEmptyString(patch.NormalizedRoles[paramID], param.NormalizedRole, patch.Aliases[paramID], param.Name, paramID))
		mapping := pluginSkillParamMapping(paramID, firstNonEmptyString(patch.Aliases[paramID], param.Alias, param.Name, paramID), pluginSkillSourceProfilePatch, pluginSkillDefaultMappingConfidence)
		applyParameterDisplayCandidateToMapping(&mapping, param)
		component.Params[slot] = mapping
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i] < order[j] })
	out := make([]PluginSkillComponent, 0, len(order))
	for _, id := range order {
		component := componentsByID[id]
		if component == nil {
			continue
		}
		out = append(out, *component)
	}
	return out
}

func componentParamMappings(value any, paramIndex map[string]ParameterInfo) map[string]PluginSkillParamMap {
	out := map[string]PluginSkillParamMap{}
	switch rows := value.(type) {
	case map[string]any:
		for slot, raw := range rows {
			mapping := pluginSkillParamMappingFromAny(raw, slot, paramIndex)
			if strings.TrimSpace(mapping.ParamID) != "" {
				out[pluginSkillParamSlot(slot)] = mapping
			}
		}
	case []any:
		for i, raw := range rows {
			slot := fmt.Sprintf("param_%d", i+1)
			mapping := pluginSkillParamMappingFromAny(raw, slot, paramIndex)
			if strings.TrimSpace(mapping.ParamID) != "" {
				out[pluginSkillParamSlot(slot)] = mapping
			}
		}
	case []string:
		for i, paramID := range rows {
			slot := fmt.Sprintf("param_%d", i+1)
			if strings.TrimSpace(paramID) != "" {
				mapping := pluginSkillParamMapping(paramID, parameterLabel(paramIndex[paramID], paramID), pluginSkillSourceProfilePatch, pluginSkillDefaultMappingConfidence)
				applyParameterDisplayCandidateToMapping(&mapping, paramIndex[paramID])
				out[slot] = mapping
			}
		}
	}
	return out
}

func pluginSkillParamMappingFromAny(value any, fallbackSlot string, paramIndex map[string]ParameterInfo) PluginSkillParamMap {
	switch v := value.(type) {
	case map[string]any:
		paramID := firstNonEmptyText(v, "param_id", "id")
		if paramID == "" {
			paramID = strings.TrimSpace(fmt.Sprint(v["parameter"]))
		}
		label := firstNonEmptyText(v, "label", "name")
		if label == "" {
			label = parameterLabel(paramIndex[paramID], paramID)
		}
		source := firstNonEmptyText(v, "source")
		if source == "" {
			source = pluginSkillSourceProfilePatch
		}
		confidence := floatNumber(v["confidence"])
		if confidence <= 0 {
			confidence = pluginSkillDefaultMappingConfidence
		}
		mapping := pluginSkillParamMapping(paramID, label, source, confidence)
		mapping.Confirmed = boolValue(v["confirmed"])
		mapping.Locked = boolValue(v["locked"])
		mapping.DisplayDomainText = firstNonEmptyText(v, "display_domain_text", "display_range", "display_unit", "unit", "range")
		mapping.DisplayDomain = displayDomainFromAny(v["display_domain"])
		if mapping.DisplayDomain == nil {
			if mapping.DisplayDomainText != "" {
				mapping.DisplayDomain = parseDisplayDomainText(mapping.DisplayDomainText, source, mapping.Confirmed)
			}
		}
		if mapping.DisplayDomain == nil {
			applyParameterDisplayCandidateToMapping(&mapping, paramIndex[paramID])
		}
		if mapping.DisplayDomain != nil && strings.TrimSpace(mapping.DisplayDomain.Text) == "" {
			mapping.DisplayDomain.Text = displayDomainTextForSummary(mapping.DisplayDomain)
		}
		if mapping.DisplayDomainText == "" {
			mapping.DisplayDomainText = displayDomainTextForSummary(mapping.DisplayDomain)
		}
		if evidence := pluginEvidenceFromAny(v["evidence"]); len(evidence) > 0 {
			mapping.Evidence = evidence
		}
		if provenance := pluginProvenanceFromAny(v["provenance"]); len(provenance) > 0 {
			mapping.Provenance = provenance
		}
		mapping.Status = pluginSkillStatusFromAny(v["status"], mapping)
		mapping.MissingEvidence = stringSliceValue(v["missing_evidence"])
		mapping.ValidatorWarnings = stringSliceValue(v["validator_warnings"])
		return mapping
	default:
		paramID := strings.TrimSpace(fmt.Sprint(value))
		if paramID == "" || paramID == "<nil>" {
			return PluginSkillParamMap{}
		}
		return pluginSkillParamMapping(paramID, parameterLabel(paramIndex[paramID], fallbackSlot), pluginSkillSourceProfilePatch, pluginSkillDefaultMappingConfidence)
	}
}

func applyParameterDisplayCandidateToMapping(mapping *PluginSkillParamMap, param ParameterInfo) {
	if mapping == nil || param.DisplayDomainCandidate == nil {
		return
	}
	mapping.DisplayDomain = param.DisplayDomainCandidate
	mapping.DisplayDomainText = displayDomainTextForSummary(param.DisplayDomainCandidate)
	mapping.Evidence = append(mapping.Evidence, PluginEvidence{
		Kind:    displayProbeSourceRaw,
		Summary: "Display domain inferred from plugin-exposed display text samples.",
	})
	mapping.Provenance = append(mapping.Provenance,
		PluginProvenance{
			Kind:    displayProbeSourceRaw,
			Source:  displayProbeSourceRaw,
			Summary: "Read-only valueToString/label/enum probe from get_plugin_parameters.",
		},
		PluginProvenance{
			Kind:    displayProbeSourceInferred,
			Source:  displayProbeSourceInferred,
			Summary: mapping.DisplayDomainText,
		},
	)
}

func pluginSkillParamMapping(paramID, label, source string, confidence float64) PluginSkillParamMap {
	mapping := PluginSkillParamMap{
		ParamID:    strings.TrimSpace(paramID),
		Label:      strings.TrimSpace(label),
		Source:     strings.TrimSpace(source),
		Confidence: confidence,
		Evidence: []PluginEvidence{{
			Kind:    "plugin_skill_learning",
			Summary: "Generated from a Plugin Skill learning draft.",
		}},
		Provenance: []PluginProvenance{{
			Kind:    source,
			Source:  source,
			Summary: "Generated from a Plugin Skill learning draft.",
		}},
	}
	mapping.Status = pluginSkillStatusFromAny(nil, mapping)
	return mapping
}

func pluginSkillExplicitPreferences(patch ProfilePatch) map[string]any {
	out := map[string]any{}
	if len(patch.TeachModeRows) > 0 {
		out["teach_mode_rows"] = patch.TeachModeRows
	}
	return out
}

func eqRuntimeOperationComponents(operations []PluginSkillOperation) map[string]bool {
	out := map[string]bool{}
	for _, op := range operations {
		if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(op.Name)), "eq.") {
			continue
		}
		if strings.TrimSpace(op.ComponentID) != "" {
			out[strings.TrimSpace(op.ComponentID)] = true
		}
	}
	return out
}

func componentLooksLikeEQBand(component PluginSkillComponent) bool {
	text := strings.ToLower(strings.Join([]string{component.ID, component.Role, component.Label}, " "))
	if strings.Contains(text, "eq_band") || (strings.Contains(text, "eq") && strings.Contains(text, "band")) {
		return true
	}
	id := strings.TrimSpace(strings.ToLower(component.ID))
	return len(id) >= 2 && id[0] == 'b' && id[1] >= '0' && id[1] <= '9'
}

func isCriticalEQSlot(slot string) bool {
	slot = pluginSkillParamSlot(slot)
	return slot == "frequency" || slot == "freq" || slot == "gain" || slot == "q"
}

func pluginSkillMappingRuntimeEligible(mapping PluginSkillParamMap) bool {
	source := strings.ToLower(strings.TrimSpace(mapping.Source))
	if mapping.Locked || strings.Contains(source, teachModeSourceUserDemonstrated) {
		return true
	}
	status := strings.TrimSpace(mapping.Status)
	if status == "" {
		status = pluginSkillStatusFromAny(nil, mapping)
	}
	return status == PluginSkillStatusActive && mapping.Confidence >= pluginSkillRuntimeConfidenceFloor
}

func pluginSkillStatusFromAny(value any, mapping PluginSkillParamMap) string {
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
	if status == "<nil>" {
		status = ""
	}
	switch status {
	case PluginSkillStatusActive, PluginSkillStatusCandidate, PluginSkillStatusUnresolved:
		return status
	}
	source := strings.ToLower(strings.TrimSpace(mapping.Source))
	if mapping.Locked || strings.Contains(source, teachModeSourceUserDemonstrated) {
		return PluginSkillStatusActive
	}
	if mapping.Confidence >= pluginSkillRuntimeConfidenceFloor {
		return PluginSkillStatusActive
	}
	if mapping.Confidence > 0 {
		return PluginSkillStatusCandidate
	}
	return PluginSkillStatusUnresolved
}

func pluginEvidenceFromAny(value any) []PluginEvidence {
	out := []PluginEvidence{}
	for _, row := range mapRowsValue(value) {
		kind := firstNonEmptyText(row, "kind", "type")
		summary := firstNonEmptyText(row, "summary", "text", "description")
		if kind == "" && summary == "" {
			continue
		}
		out = append(out, PluginEvidence{Kind: kind, Summary: summary})
	}
	return out
}

func pluginProvenanceFromAny(value any) []PluginProvenance {
	out := []PluginProvenance{}
	for _, row := range mapRowsValue(value) {
		kind := firstNonEmptyText(row, "kind", "type")
		source := firstNonEmptyText(row, "source")
		summary := firstNonEmptyText(row, "summary", "text", "description")
		if kind == "" && source == "" && summary == "" {
			continue
		}
		out = append(out, PluginProvenance{
			Kind:    kind,
			Source:  source,
			Summary: summary,
			Data:    mapValue(row["data"]),
		})
	}
	return out
}

func buildPluginSkillOperations(patch ProfilePatch) []PluginSkillOperation {
	out := make([]PluginSkillOperation, 0, len(patch.VirtualControls))
	for _, row := range patch.VirtualControls {
		name := firstNonEmptyText(row, "name", "operation")
		if name == "" {
			continue
		}
		op := PluginSkillOperation{
			Name:        strings.TrimSpace(name),
			Inputs:      stringSliceValue(row["inputs"]),
			Resolver:    firstNonEmptyText(row, "resolver"),
			ComponentID: firstNonEmptyText(row, "component_id", "component"),
		}
		if params, ok := row["params"].(map[string]any); ok {
			op.Params = mapStringAnyCopy(params)
		}
		out = append(out, op)
	}
	return out
}

func legacyProjectDefaultMap(patch ProfilePatch) map[string]any {
	out := map[string]any{}
	if len(patch.QuickControlIDs) > 0 {
		out["quick_control_ids"] = append([]string(nil), patch.QuickControlIDs...)
	}
	if len(patch.Aliases) > 0 {
		out["aliases"] = mapStringStringCopy(patch.Aliases)
	}
	if len(patch.DisplayGroups) > 0 {
		out["display_groups"] = mapStringStringCopy(patch.DisplayGroups)
	}
	if len(patch.NormalizedRoles) > 0 {
		out["normalized_roles"] = mapStringStringCopy(patch.NormalizedRoles)
	}
	return out
}

func pluginSkillCapabilityTypes(primaryClass string) []string {
	primaryClass = pluginSkillSlug("", primaryClass)
	if primaryClass == "" || primaryClass == "unknown" {
		return nil
	}
	return []string{primaryClass}
}

func pluginParamSignatureHash(params []ParameterInfo) string {
	ids := make([]string, 0, len(params))
	for _, param := range params {
		if id := strings.TrimSpace(param.ID); id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.Join(ids, "|")))
	return fmt.Sprintf("p_%016x", h.Sum64())
}

func coverageForParameter(param ParameterInfo) string {
	group := strings.ToLower(strings.TrimSpace(param.DisplayGroup))
	role := strings.ToLower(strings.TrimSpace(param.NormalizedRole))
	relevance := strings.ToLower(strings.TrimSpace(param.ControlRelevance))
	name := strings.ToLower(firstNonEmptyString(param.Name, param.RawName, param.ID))
	if !param.HostControllable && relevance == "" {
		return "display_only"
	}
	if strings.Contains(group, "analy") || strings.Contains(role, "analy") || strings.Contains(name, "analy") || strings.Contains(name, "meter") || strings.Contains(name, "spectrum") {
		return "analyzer_only"
	}
	if strings.Contains(group, "utility") || strings.Contains(relevance, "utility") || strings.Contains(role, "bypass") {
		return "utility"
	}
	return "unknown_needs_review"
}

func parameterLabel(param ParameterInfo, fallback string) string {
	return firstNonEmptyString(param.Alias, param.Name, param.RawName, fallback)
}

func mapStringStringCopy(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func mapStringAnyCopy(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
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

func pluginSkillParamSlot(value string) string {
	slot := pluginSkillSlug("param", value)
	if slot == "" {
		return "param"
	}
	return slot
}

func pluginSkillSlug(prefix, value string) string {
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
		return prefix
	}
	if prefix != "" && (out[0] >= '0' && out[0] <= '9') {
		return prefix + "_" + out
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
