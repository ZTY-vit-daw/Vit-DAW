package plugingrabber

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	autoLearnSourceEQBandPattern = "auto_learn_eq_band_pattern"
	autoLearnConfidence          = 0.94
	// maxEQBandIndex bounds how many band numbers auto-learn scans for.
	// Simple parametric EQs (TDR Nova) have 3-4 bands; graphic/multiband EQs
	// (Fabfilter Pro-Q 3) go up to 24, so this needs real headroom above that.
	maxEQBandIndex = 32
)

type eqBandDraft struct {
	ID     string
	Label  string
	Params map[string]ParameterInfo
}

// BuildAutoLearnProfilePatch creates a deterministic profile patch when the
// parameter list is clear enough to avoid asking the model to guess mappings.
func BuildAutoLearnProfilePatch(digest ParameterDigest) (ProfilePatch, map[string]any, bool) {
	bands := detectEQBands(digest)
	if len(bands) == 0 {
		return ProfilePatch{}, nil, false
	}

	source := autoLearnSourceEQBandPattern
	preserved := demonstratedMappingsFromPluginSkill(digest.PluginSkill)
	quickIDs := []string{}
	aliases := map[string]string{}
	displayGroups := map[string]string{}
	roles := map[string]string{}
	groups := []map[string]any{}
	warnings := []string{}

	for _, band := range bands {
		params := map[string]any{}
		// Keep these implementation-facing slots inside Plugin Learning.  They
		// are not exposed as semantic SPAL controls: a later Provider adapter
		// decides whether a learned EQ can prove a safe static-Bell binding.
		// In particular, dyn_enable and type let a conformance workflow verify
		// that it will not silently enable dynamic processing or change shape.
		for _, slot := range []string{"enable", "dyn_enable", "frequency", "gain", "q", "type"} {
			param, ok := band.Params[slot]
			if !ok {
				if slot == "frequency" || slot == "gain" {
					warnings = append(warnings, fmt.Sprintf("%s missing %s", band.Label, slot))
				}
				continue
			}
			label := firstNonEmptyString(parameterLabel(param, param.ID), slot)
			mapping := autoLearnParamMapping(param.ID, label, source, fmt.Sprintf("%s %s matched by parameter name", band.Label, slot), inferredDisplayDomainForSlot(slot, param))
			if demonstrated := preserved.ForSlot(band.ID, slot); len(demonstrated) > 0 {
				mapping = demonstrated
			}
			params[slot] = mapping
			quickIDs = appendUniqueString(quickIDs, param.ID)
			aliases[param.ID] = fmt.Sprintf("%s %s", band.Label, slotLabel(slot))
			displayGroups[param.ID] = "EQ"
			roles[param.ID] = "eq_" + slot
		}
		if len(params) == 0 {
			continue
		}
		groups = append(groups, map[string]any{
			"id":     band.ID,
			"role":   "eq_band",
			"label":  band.Label,
			"params": params,
		})
	}

	for _, group := range preserved.Groups {
		id := firstNonEmptyText(group, "id")
		if id == "" || !containsProfileGroupID(groups, id) {
			groups = append(groups, group)
		}
	}

	virtualControls := []map[string]any{
		{
			"name":     "eq.cut_region",
			"inputs":   []string{"freq_hz", "gain_db", "q", "amount"},
			"resolver": "choose_nearest_or_free_band",
		},
		{
			"name":     "eq.boost_region",
			"inputs":   []string{"freq_hz", "gain_db", "q", "amount"},
			"resolver": "choose_nearest_or_free_band",
		},
		{
			"name":     "eq.set_region",
			"inputs":   []string{"freq_hz", "gain_db", "q"},
			"resolver": "choose_nearest_or_free_band",
		},
	}
	for _, op := range preserved.Operations {
		name := firstNonEmptyText(op, "name", "operation")
		if name == "" || !containsVirtualControlName(virtualControls, name) {
			virtualControls = append(virtualControls, op)
		}
	}

	patch := ProfilePatch{
		Reply:           autoLearnReply(digest, len(groups), len(warnings)),
		QuickControlIDs: quickIDs,
		Aliases:         aliases,
		DisplayGroups:   displayGroups,
		NormalizedRoles: roles,
		Class:           "eq",
		Groups:          groups,
		VirtualControls: virtualControls,
		Safety: map[string]any{
			"max_gain_change_db": 6.0,
			"min_q":              0.25,
			"max_q":              8.0,
			"notes":              "EQ runtime controls clamp musical gain and Q requests before writing mapped parameters.",
		},
	}

	summary := map[string]any{
		"mode":                   string(LearningModeAutoLearn),
		"state":                  "profile_draft",
		"strategy":               source,
		"class":                  patch.Class,
		"plugin_name":            firstNonEmptyString(digest.PluginName, firstNonEmptyText(digest.PluginIdentity, "plugin_name", "name")),
		"parameter_count":        digest.ParameterCount,
		"component_count":        len(groups),
		"operation_count":        len(virtualControls),
		"components":             groupSummaries(groups),
		"operations":             operationNames(virtualControls),
		"safety":                 patch.Safety,
		"warnings":               warnings,
		"preserved_teach_map":    preserved.Count,
		"confidence_summary":     mappingConfidenceSummary(groups),
		"evidence_summary":       mappingEvidenceSummary(groups),
		"display_domain_summary": displayDomainSummary(groups),
		"display_probe_summary":  DisplayProbeSummary(digest),
		"profile_preview": map[string]any{
			"class":           patch.Class,
			"components":      groupSummaries(groups),
			"operations":      operationNames(virtualControls),
			"parameter_count": digest.ParameterCount,
		},
	}
	return patch, summary, true
}

func detectEQBands(digest ParameterDigest) []eqBandDraft {
	byID := map[string]*eqBandDraft{}
	order := []string{}
	for _, param := range digest.Parameters {
		if strings.TrimSpace(param.ID) == "" {
			continue
		}
		bandIndex, slot, score := classifyEQBandParam(param)
		if bandIndex <= 0 || slot == "" || score <= 0 {
			continue
		}
		id := fmt.Sprintf("b%d", bandIndex)
		band := byID[id]
		if band == nil {
			band = &eqBandDraft{ID: id, Label: strings.ToUpper(id), Params: map[string]ParameterInfo{}}
			byID[id] = band
			order = append(order, id)
		}
		if existing, ok := band.Params[slot]; !ok || classifyEQBandSlotScore(param, bandIndex, slot) > classifyEQBandSlotScore(existing, bandIndex, slot) {
			band.Params[slot] = param
		}
	}
	out := []eqBandDraft{}
	sort.Slice(order, func(i, j int) bool { return eqBandIndex(order[i]) < eqBandIndex(order[j]) })
	for _, id := range order {
		band := byID[id]
		if band == nil {
			continue
		}
		if _, ok := band.Params["frequency"]; !ok {
			continue
		}
		if _, ok := band.Params["gain"]; !ok {
			continue
		}
		out = append(out, *band)
	}
	if len(out) >= 4 {
		return out
	}
	if looksLikeEQ(digest) && len(out) >= 1 {
		return out
	}
	return nil
}

// eqBandIndex parses the numeric suffix of a "b<N>" band ID for numeric
// sorting (band IDs must otherwise sort as strings: b1, b10..b19, b2, ...).
func eqBandIndex(id string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(id, "b"))
	if err != nil {
		return 0
	}
	return n
}

func classifyEQBandParam(param ParameterInfo) (int, string, int) {
	bestBand := 0
	bestSlot := ""
	bestScore := 0
	for band := 1; band <= maxEQBandIndex; band++ {
		if !paramTextHasBand(param, band) {
			continue
		}
		for _, slot := range []string{"frequency", "gain", "q", "enable", "dyn_enable", "type"} {
			score := classifyEQBandSlotScore(param, band, slot)
			if score > bestScore {
				bestBand = band
				bestSlot = slot
				bestScore = score
			}
		}
	}
	return bestBand, bestSlot, bestScore
}

func classifyEQBandSlotScore(param ParameterInfo, band int, slot string) int {
	text := normalizedParamText(param)
	compact := compactTokenText(text)
	score := 0
	if hasBoundedBandToken(compact, "b", band) {
		score += 4
	}
	if hasBoundedBandToken(compact, "band", band) {
		score += 5
	}
	switch slot {
	case "frequency":
		if strings.Contains(compact, "freq") || strings.Contains(compact, "frequency") {
			score += 8
		}
		if strings.Contains(text, " hz") || strings.HasSuffix(text, "hz") {
			score += 2
		}
	case "gain":
		if strings.Contains(compact, "gain") {
			score += 8
		}
		if strings.Contains(compact, "level") {
			score += 3
		}
	case "q":
		if strings.Contains(compact, fmt.Sprintf("b%dq", band)) || strings.Contains(compact, fmt.Sprintf("band%dq", band)) {
			score += 9
		}
		if strings.Contains(text, " q ") || strings.HasSuffix(text, " q") || strings.Contains(compact, "quality") || strings.Contains(compact, "bandwidth") {
			score += 7
		}
	case "enable":
		if strings.Contains(compact, "enable") || strings.Contains(compact, "enabled") || strings.Contains(compact, "active") {
			score += 8
		}
		if strings.Contains(compact, fmt.Sprintf("b%don", band)) || strings.Contains(compact, fmt.Sprintf("band%don", band)) {
			score += 7
		}
		score += booleanToggleBonus(param)
	case "dyn_enable":
		if strings.Contains(compact, fmt.Sprintf("b%ddyn", band)) || strings.Contains(compact, fmt.Sprintf("band%ddyn", band)) {
			score += 10
		}
		if strings.Contains(compact, "dynamic") || strings.Contains(compact, "dyn") {
			score += 4
		}
		score += booleanToggleBonus(param)
	case "type":
		if strings.Contains(compact, fmt.Sprintf("b%dtype", band)) || strings.Contains(compact, fmt.Sprintf("band%dtype", band)) ||
			strings.Contains(compact, fmt.Sprintf("b%dshape", band)) || strings.Contains(compact, fmt.Sprintf("band%dshape", band)) {
			score += 10
		}
		if strings.Contains(compact, "type") || strings.Contains(compact, "shape") {
			score += 4
		}
	}
	if !param.HostControllable {
		score -= 1
	}
	return int(math.Max(0, float64(score)))
}

// booleanToggleBonus rewards parameters that are actually a two-state
// toggle over ones that merely share an "enable"/"dyn" keyword. Without this,
// a continuous amount parameter (e.g. "Band 1 Dynamic Range", -30~30dB) and
// the real boolean toggle (e.g. "Band 1 Dynamics Enabled") score identically
// on keyword match alone, and the strict-greater-than tie-break in
// detectEQBands then keeps whichever one was encountered first — which is
// not necessarily the toggle.
func booleanToggleBonus(param ParameterInfo) int {
	if param.IsBoolean || (param.IsDiscrete && param.NumSteps == 2) {
		return 6
	}
	return 0
}

func paramTextHasBand(param ParameterInfo, band int) bool {
	text := normalizedParamText(param)
	compact := compactTokenText(text)
	return hasBoundedBandToken(compact, "b", band) ||
		hasBoundedBandToken(compact, "band", band) ||
		strings.Contains(text, fmt.Sprintf("band %d", band))
}

// hasBoundedBandToken reports whether compact contains prefix+strconv.Itoa(band)
// as a whole token — i.e. not immediately followed by another digit. Without
// this check, "band1" matches inside "band10".."band19" (and "b1" inside
// "b10".."b19"), silently merging a 24-band plugin's higher bands into band 1.
func hasBoundedBandToken(compact, prefix string, band int) bool {
	token := prefix + strconv.Itoa(band)
	idx := 0
	for {
		pos := strings.Index(compact[idx:], token)
		if pos < 0 {
			return false
		}
		pos += idx
		end := pos + len(token)
		if end >= len(compact) || !unicode.IsDigit(rune(compact[end])) {
			return true
		}
		idx = pos + 1
	}
}

func normalizedParamText(param ParameterInfo) string {
	parts := []string{
		param.ID,
		param.Name,
		param.RawName,
		param.Alias,
		param.DisplayGroup,
		param.NormalizedRole,
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func compactTokenText(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func autoLearnParamMapping(paramID, label, source, evidence string, domain *PluginDisplayDomain) map[string]any {
	evidenceRows := []map[string]any{{
		"kind":    "coherent_eq_band_pattern",
		"summary": evidence,
	}}
	provenanceRows := []map[string]any{{
		"kind":    source,
		"source":  source,
		"summary": evidence,
	}}
	if domain != nil && strings.EqualFold(strings.TrimSpace(domain.Source), displayProbeSourceInferred) {
		evidenceRows = append(evidenceRows, map[string]any{
			"kind":    displayProbeSourceRaw,
			"summary": "Display domain inferred from plugin-exposed display text samples.",
		})
		provenanceRows = append(provenanceRows,
			map[string]any{
				"kind":    displayProbeSourceRaw,
				"source":  displayProbeSourceRaw,
				"summary": "Read-only valueToString/label/enum probe from get_plugin_parameters.",
			},
			map[string]any{
				"kind":    displayProbeSourceInferred,
				"source":  displayProbeSourceInferred,
				"summary": displayDomainTextForSummary(domain),
			},
		)
	}
	mapping := map[string]any{
		"param_id":         strings.TrimSpace(paramID),
		"label":            strings.TrimSpace(label),
		"source":           source,
		"confidence":       autoLearnConfidence,
		"confirmed":        false,
		"locked":           false,
		"status":           PluginSkillStatusActive,
		"missing_evidence": []string{"user_review"},
		"evidence":         evidenceRows,
		"provenance":       provenanceRows,
	}
	if domainMap := displayDomainMap(domain); len(domainMap) > 0 {
		mapping["display_domain"] = domainMap
		mapping["display_domain_text"] = displayDomainTextForSummary(domain)
	}
	return mapping
}

func looksLikeEQ(digest ParameterDigest) bool {
	text := strings.ToLower(strings.Join([]string{
		digest.PluginName,
		digest.TemplateRole,
		digest.PluginClass,
		firstNonEmptyText(digest.PluginIdentity, "plugin_name", "name"),
		firstNonEmptyText(digest.PluginIdentity, "category", "plugin_category"),
	}, " "))
	return strings.Contains(text, "eq") ||
		strings.Contains(text, "equal") ||
		strings.Contains(text, "nova") ||
		strings.Contains(text, "filter")
}

func autoLearnReply(digest ParameterDigest, componentCount, warningCount int) string {
	name := firstNonEmptyString(digest.PluginName, firstNonEmptyText(digest.PluginIdentity, "plugin_name", "name"), "this plugin")
	if warningCount > 0 {
		return fmt.Sprintf("已为 %s 生成 EQ Plugin Grabber 草案，映射了 %d 个组件。请先检查摘要再保存。", name, componentCount)
	}
	return fmt.Sprintf("已为 %s 生成 EQ Plugin Grabber 草案，映射了 %d 个组件。", name, componentCount)
}

func slotLabel(slot string) string {
	switch slot {
	case "frequency":
		return "Frequency"
	case "gain":
		return "Gain"
	case "q":
		return "Q"
	case "enable":
		return "Enable"
	default:
		return strings.Title(slot)
	}
}

func groupSummaries(groups []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, group := range groups {
		params := mapValue(group["params"])
		out = append(out, map[string]any{
			"id":          firstNonEmptyText(group, "id"),
			"role":        firstNonEmptyText(group, "role"),
			"label":       firstNonEmptyText(group, "label", "name", "id"),
			"param_slots": sortedMapKeys(params),
		})
	}
	return out
}

func operationNames(rows []map[string]any) []string {
	out := []string{}
	for _, row := range rows {
		name := firstNonEmptyText(row, "name", "operation")
		if name != "" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func sortedMapKeys(values map[string]any) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsProfileGroupID(groups []map[string]any, id string) bool {
	id = strings.TrimSpace(id)
	for _, group := range groups {
		if firstNonEmptyText(group, "id") == id {
			return true
		}
	}
	return false
}

func containsVirtualControlName(rows []map[string]any, name string) bool {
	name = strings.TrimSpace(name)
	for _, row := range rows {
		if strings.EqualFold(firstNonEmptyText(row, "name", "operation"), name) {
			return true
		}
	}
	return false
}

func mappingConfidenceSummary(groups []map[string]any) map[string]any {
	out := map[string]any{
		"active":     0,
		"candidate":  0,
		"unresolved": 0,
		"total":      0,
	}
	var minConfidence float64
	var maxConfidence float64
	for _, mapping := range profileMappingsFromGroups(groups) {
		confidence := floatNumber(mapping["confidence"])
		if out["total"].(int) == 0 || confidence < minConfidence {
			minConfidence = confidence
		}
		if out["total"].(int) == 0 || confidence > maxConfidence {
			maxConfidence = confidence
		}
		status := strings.TrimSpace(firstNonEmptyText(mapping, "status"))
		if status == "" {
			status = statusFromMappingMap(mapping)
		}
		switch status {
		case PluginSkillStatusActive:
			out["active"] = out["active"].(int) + 1
		case PluginSkillStatusUnresolved:
			out["unresolved"] = out["unresolved"].(int) + 1
		default:
			out["candidate"] = out["candidate"].(int) + 1
		}
		out["total"] = out["total"].(int) + 1
	}
	out["min_confidence"] = minConfidence
	out["max_confidence"] = maxConfidence
	return out
}

func mappingEvidenceSummary(groups []map[string]any) map[string]any {
	byKind := map[string]int{}
	total := 0
	for _, mapping := range profileMappingsFromGroups(groups) {
		for _, evidence := range mapRowsValue(mapping["evidence"]) {
			kind := firstNonEmptyText(evidence, "kind", "type")
			if kind == "" {
				kind = "unspecified"
			}
			byKind[kind]++
			total++
		}
	}
	return map[string]any{
		"total":   total,
		"by_kind": byKind,
	}
}

func mappingMissingEvidence(groups []map[string]any) []string {
	out := []string{}
	for _, mapping := range profileMappingsFromGroups(groups) {
		for _, item := range stringSliceValue(mapping["missing_evidence"]) {
			out = appendUniqueString(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func profileMappingsFromGroups(groups []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, group := range groups {
		for _, raw := range mapValue(group["params"]) {
			mapping := mapValue(raw)
			if len(mapping) > 0 {
				out = append(out, mapping)
			}
		}
	}
	return out
}

func statusFromMappingMap(mapping map[string]any) string {
	source := strings.ToLower(strings.TrimSpace(firstNonEmptyText(mapping, "source")))
	if boolValue(mapping["locked"]) || strings.Contains(source, teachModeSourceUserDemonstrated) {
		return PluginSkillStatusActive
	}
	confidence := floatNumber(mapping["confidence"])
	if confidence >= pluginSkillRuntimeConfidenceFloor {
		return PluginSkillStatusActive
	}
	if confidence > 0 {
		return PluginSkillStatusCandidate
	}
	return PluginSkillStatusUnresolved
}

type demonstratedMappingSet struct {
	ByComponent map[string]map[string]map[string]any
	Groups      []map[string]any
	Operations  []map[string]any
	Count       int
}

func (s demonstratedMappingSet) ForSlot(componentID, slot string) map[string]any {
	if s.ByComponent == nil {
		return nil
	}
	bySlot := s.ByComponent[strings.TrimSpace(componentID)]
	if bySlot == nil {
		return nil
	}
	value := bySlot[pluginSkillParamSlot(slot)]
	if len(value) == 0 {
		return nil
	}
	return mapStringAnyCopy(value)
}

func demonstratedMappingsFromPluginSkill(skill map[string]any) demonstratedMappingSet {
	out := demonstratedMappingSet{ByComponent: map[string]map[string]map[string]any{}}
	for _, component := range mapRowsValue(skill["components"]) {
		id := firstNonEmptyText(component, "id")
		if id == "" {
			continue
		}
		params := mapValue(component["params"])
		groupParams := map[string]any{}
		used := false
		for slot, raw := range params {
			mapping := mapValue(raw)
			if !strings.Contains(strings.ToLower(firstNonEmptyText(mapping, "source")), "user_demonstrated") {
				continue
			}
			if out.ByComponent[id] == nil {
				out.ByComponent[id] = map[string]map[string]any{}
			}
			slot = pluginSkillParamSlot(slot)
			out.ByComponent[id][slot] = mapping
			groupParams[slot] = mapping
			out.Count++
			used = true
		}
		if used {
			out.Groups = append(out.Groups, map[string]any{
				"id":     id,
				"role":   firstNonEmptyString(firstNonEmptyText(component, "role"), "user_demonstrated"),
				"label":  firstNonEmptyText(component, "label", "id"),
				"params": groupParams,
			})
		}
	}
	for _, op := range mapRowsValue(skill["operations"]) {
		if strings.Contains(strings.ToLower(firstNonEmptyText(op, "resolver", "source")), "user_demonstrated") {
			out.Operations = append(out.Operations, op)
		}
	}
	return out
}
