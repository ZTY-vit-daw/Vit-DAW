package mixcontrolsurface

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const SchemaVersion = "goal_control_surface.v1"

const (
	ReadinessPlanReady         = "plan_ready"
	ReadinessBlocked           = "blocked"
	ReadinessNeedsConfirmation = "needs_confirmation"

	InstanceExisting    = "existing"
	InstanceNeedsLoad   = "needs_load"
	InstanceUnavailable = "unavailable"
	InstanceUnknown     = "unknown"

	ProfileReady       = "ready"
	ProfileMissing     = "missing"
	ProfileStale       = "stale"
	ProfileNotRequired = "not_required"
)

type Target struct {
	Kind       string `json:"kind,omitempty"`
	ID         string `json:"id,omitempty"`
	Label      string `json:"label,omitempty"`
	Source     string `json:"source,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

type Request struct {
	MixSessionID     string
	Mode             string
	Goal             string
	Target           Target
	Observation      map[string]any
	PluginCandidates []map[string]any
	ProjectProfiles  []map[string]any
	RackPlugins      []map[string]any
	Warnings         []string
	Now              time.Time
}

type rolePlan struct {
	Role     string
	Type     string
	Purpose  string
	Priority int
	Required bool
}

func RequiredRoleTypes(goal string) []string {
	roles := inferRoles(goal)
	out := make([]string, 0, len(roles))
	seen := map[string]bool{}
	for _, role := range roles {
		if role.Type == "" || seen[role.Type] {
			continue
		}
		seen[role.Type] = true
		out = append(out, role.Type)
	}
	return out
}

func Build(req Request) map[string]any {
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	roles := inferRoles(req.Goal)
	profiles := normalizeProfiles(req.ProjectProfiles)
	candidates := mergeCandidates(req.PluginCandidates, profiles)
	selected := selectChain(roles, candidates, profiles, req.RackPlugins)
	blockers, readiness, nextAction := readinessFor(req.Target, selected)
	effectPlan := effectChainPlan(roles)

	out := map[string]any{
		"schema_version":       SchemaVersion,
		"mix_session_id":       strings.TrimSpace(req.MixSessionID),
		"mode":                 firstNonEmpty(req.Mode, "auto_mix"),
		"goal":                 strings.TrimSpace(req.Goal),
		"target":               targetMap(req.Target),
		"required_roles":       roleRows(roles),
		"effect_chain_plan":    effectPlan,
		"plugin_candidates":    candidates,
		"selected_chain":       selected,
		"instance_status":      aggregateStatus(selected, "instance_status", InstanceUnavailable),
		"profile_status":       aggregateStatus(selected, "profile_status", ProfileMissing),
		"proposed_controls":    proposedControls(selected),
		"readiness":            readiness,
		"blockers":             blockers,
		"next_required_action": nextAction,
		"updated_at":           now.UTC().Format(time.RFC3339Nano),
	}
	if len(req.Warnings) > 0 {
		out["warnings"] = compactStrings(req.Warnings)
	}
	return out
}

func inferRoles(goal string) []rolePlan {
	text := strings.ToLower(strings.TrimSpace(goal))
	roles := []rolePlan{}
	add := func(role rolePlan) {
		for _, existing := range roles {
			if existing.Type == role.Type {
				return
			}
		}
		roles = append(roles, role)
	}
	add(rolePlan{
		Role:     "tone_balance",
		Type:     "eq",
		Purpose:  "Shape tone, clean masking, and create presence before level automation.",
		Priority: 1,
		Required: true,
	})
	add(rolePlan{
		Role:     "dynamic_stability",
		Type:     "dynamics",
		Purpose:  "Stabilize level and density with an auditable dynamics stage.",
		Priority: 2,
		Required: true,
	})
	if containsAny(text, "de-ess", "deess", "sibil", "harsh", "sharp", "\u9f7f", "\u523a\u8033", "\u6bdb\u523a") {
		add(rolePlan{
			Role:     "sibilance_control",
			Type:     "de_ess",
			Purpose:  "Tame sibilance or harsh high-frequency peaks if observation confirms them.",
			Priority: 3,
			Required: false,
		})
	}
	if containsAny(text, "space", "ambience", "ambient", "reverb", "depth", "\u7a7a\u95f4", "\u6df7\u54cd", "\u6c1b\u56f4") {
		add(rolePlan{
			Role:     "depth_space",
			Type:     "reverb",
			Purpose:  "Place the source in depth after corrective tone and dynamics decisions.",
			Priority: 4,
			Required: false,
		})
	}
	sort.SliceStable(roles, func(i, j int) bool { return roles[i].Priority < roles[j].Priority })
	return roles
}

func effectChainPlan(roles []rolePlan) []map[string]any {
	out := make([]map[string]any, 0, len(roles))
	for slot, role := range roles {
		out = append(out, map[string]any{
			"slot":     slot + 1,
			"role":     role.Role,
			"type":     role.Type,
			"purpose":  role.Purpose,
			"required": role.Required,
		})
	}
	return out
}

func roleRows(roles []rolePlan) []map[string]any {
	out := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		out = append(out, map[string]any{
			"role":     role.Role,
			"type":     role.Type,
			"purpose":  role.Purpose,
			"priority": role.Priority,
			"required": role.Required,
		})
	}
	return out
}

func mergeCandidates(raw []map[string]any, profiles []map[string]any) []map[string]any {
	out := []map[string]any{}
	seen := map[string]bool{}
	add := func(row map[string]any, source string) {
		normalized := normalizeCandidate(row)
		if len(normalized) == 0 {
			return
		}
		keys := candidateDedupeKeys(normalized)
		duplicate := false
		for _, key := range keys {
			if seen[key] {
				duplicate = true
				break
			}
		}
		if duplicate {
			return
		}
		for _, key := range keys {
			seen[key] = true
		}
		if text(normalized, "source") == "" {
			normalized["source"] = source
		}
		out = append(out, normalized)
	}
	for _, row := range raw {
		add(row, "plugin_semantic_search")
	}
	for _, profile := range profiles {
		add(candidateFromProfile(profile), "plugin_grabber_profile")
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := strings.ToLower(text(out[i], "name"))
		right := strings.ToLower(text(out[j], "name"))
		if left == right {
			return text(out[i], "source") < text(out[j], "source")
		}
		return left < right
	})
	return out
}

func selectChain(roles []rolePlan, candidates, profiles, rackPlugins []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(roles))
	for slot, role := range roles {
		candidate := bestCandidateForRole(role.Type, candidates, profiles)
		row := map[string]any{
			"slot":    slot + 1,
			"role":    role.Role,
			"type":    role.Type,
			"purpose": role.Purpose,
		}
		if len(candidate) == 0 {
			row["status"] = "blocked"
			row["instance_status"] = InstanceUnavailable
			row["profile_status"] = ProfileMissing
			row["reason"] = "No plugin candidate was available for this role."
			out = append(out, row)
			continue
		}
		profile, profileStatus := matchProfile(candidate, profiles)
		instance := matchRackInstance(candidate, rackPlugins)
		instanceStatus := InstanceNeedsLoad
		if len(instance) > 0 {
			instanceStatus = InstanceExisting
		}
		row["status"] = "planned"
		row["selected_plugin"] = compactPlugin(candidate)
		row["selection_reason"] = selectionReason(role.Type, candidate)
		row["instance_status"] = instanceStatus
		row["profile_status"] = profileStatus
		if len(instance) > 0 {
			row["instance"] = compactInstance(instance)
		}
		if len(profile) > 0 {
			row["profile_id"] = firstNonEmpty(text(profile, "profile_id"), text(profile, "id"))
			row["profile_class"] = firstNonEmpty(text(profile, "class"), text(mapValue(profile["plugin_skill"]), "primary_class"))
			row["proposed_controls"] = profileControls(profile)
		}
		out = append(out, row)
	}
	return out
}

func readinessFor(target Target, selected []map[string]any) ([]string, string, string) {
	blockers := []string{}
	if strings.TrimSpace(target.ID) == "" {
		blockers = append(blockers, "mix target is not resolved")
	}
	if len(selected) == 0 {
		blockers = append(blockers, "no effect roles were planned")
	}
	hasNeedsLoad := false
	for _, row := range selected {
		switch text(row, "instance_status") {
		case InstanceUnavailable, InstanceUnknown:
			blockers = append(blockers, fmt.Sprintf("%s has no available plugin candidate", text(row, "type")))
		case InstanceNeedsLoad:
			hasNeedsLoad = true
		}
		switch text(row, "profile_status") {
		case ProfileMissing:
			blockers = append(blockers, fmt.Sprintf("%s requires Plugin Grabber learning", text(row, "type")))
		case ProfileStale:
			blockers = append(blockers, fmt.Sprintf("%s has a stale Plugin Grabber profile", text(row, "type")))
		}
	}
	blockers = compactStrings(blockers)
	if len(blockers) > 0 {
		next := "learn_plugin_profile"
		for _, blocker := range blockers {
			if strings.Contains(blocker, "target") {
				next = "observe_again"
				break
			}
			if strings.Contains(blocker, "candidate") {
				next = "select_plugin"
			}
		}
		return blockers, ReadinessBlocked, next
	}
	if hasNeedsLoad {
		return nil, ReadinessNeedsConfirmation, "confirm_control_surface"
	}
	return nil, ReadinessPlanReady, "confirm_control_surface"
}

func aggregateStatus(rows []map[string]any, key, fallback string) string {
	if len(rows) == 0 {
		return fallback
	}
	hasNeedsLoad := false
	for _, row := range rows {
		status := text(row, key)
		if status == ProfileMissing || status == ProfileStale || status == InstanceUnavailable || status == InstanceUnknown {
			return status
		}
		if status == InstanceNeedsLoad {
			hasNeedsLoad = true
		}
	}
	if hasNeedsLoad {
		return InstanceNeedsLoad
	}
	return text(rows[0], key)
}

func proposedControls(selected []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, row := range selected {
		for _, control := range rows(row["proposed_controls"]) {
			control["role"] = text(row, "role")
			control["type"] = text(row, "type")
			if plugin := mapValue(row["selected_plugin"]); len(plugin) > 0 {
				control["plugin_name"] = text(plugin, "name")
			}
			out = append(out, control)
			if len(out) >= 12 {
				return out
			}
		}
	}
	return out
}

func bestCandidateForRole(roleType string, candidates, profiles []map[string]any) map[string]any {
	best := map[string]any{}
	bestScore := -1
	bestTier := -1
	for _, candidate := range candidates {
		score := roleMatchScore(roleType, candidate)
		profile, profileStatus := matchProfile(candidate, profiles)
		if len(profile) > 0 {
			if profileScore := roleMatchScore(roleType, candidateFromProfile(profile)); profileScore > score {
				score = profileScore
			}
		}
		if score <= 0 {
			continue
		}
		tier := profileSelectionTier(profileStatus)
		if tier > bestTier || (tier == bestTier && score > bestScore) {
			bestTier = tier
			bestScore = score
			best = candidate
		}
	}
	if bestScore <= 0 {
		return nil
	}
	return best
}

func profileSelectionTier(profileStatus string) int {
	switch profileStatus {
	case ProfileReady, ProfileNotRequired:
		return 3
	case ProfileStale:
		return 2
	default:
		return 1
	}
}

func roleMatchScore(roleType string, candidate map[string]any) int {
	score := 0
	primary := canonicalType(text(candidate, "primary_type", "class", "type"))
	if primary == roleType {
		score += 80
	}
	for _, tag := range rows(candidate["semantic_types"]) {
		if canonicalType(text(tag, "type")) == roleType {
			score += intNumber(tag["score"], 40)
		}
	}
	class := canonicalType(text(candidate, "class"))
	if class == roleType {
		score += 70
	}
	name := strings.ToLower(strings.Join([]string{
		text(candidate, "name"),
		text(candidate, "descriptive_name"),
		text(candidate, "category"),
		text(candidate, "manufacturer"),
		text(candidate, "profile_controls_text"),
	}, " "))
	switch roleType {
	case "eq":
		if containsAny(name, "eq", "equalizer", "nova", "filter") {
			score += 40
		}
	case "dynamics":
		if containsAny(name, "comp", "compress", "dynamics", "limiter", "nova", "threshold", "ratio", "wideband", "dynamic eq", "dyn") {
			score += 40
		}
		if containsAny(name, "wideband compression", "threshold", "ratio") {
			score += 60
		}
	case "de_ess":
		if containsAny(name, "de-ess", "deess", "sibil", "nova", "dynamic eq") {
			score += 40
		}
	case "reverb":
		if containsAny(name, "reverb", "space", "supermassive", "delay") {
			score += 40
		}
	case "analyzer":
		if containsAny(name, "analyzer", "spectrum", "meter", "span") {
			score += 40
		}
	}
	if score > 0 && hasProfileKey(candidate) {
		score += 10
	}
	return score
}

func normalizeCandidate(row map[string]any) map[string]any {
	row = mapValue(row)
	if len(row) == 0 {
		return nil
	}
	out := map[string]any{
		"id":                    firstNonEmpty(text(row, "id"), text(row, "profile_id")),
		"name":                  firstNonEmpty(text(row, "name"), text(row, "plugin_name"), text(row, "descriptive_name")),
		"descriptive_name":      text(row, "descriptive_name"),
		"manufacturer":          firstNonEmpty(text(row, "manufacturer"), text(row, "vendor")),
		"format":                firstNonEmpty(text(row, "format"), text(row, "plugin_format")),
		"category":              text(row, "category"),
		"identifier":            firstNonEmpty(text(row, "identifier"), text(row, "uid")),
		"plugin_path":           firstNonEmpty(text(row, "plugin_path"), text(row, "path"), text(row, "file_path")),
		"primary_type":          canonicalType(firstNonEmpty(text(row, "primary_type"), text(row, "class"), text(row, "type"))),
		"semantic_types":        row["semantic_types"],
		"confidence":            row["confidence"],
		"source":                text(row, "source"),
		"profile_id":            text(row, "profile_id"),
		"profile_status":        text(row, "profile_status"),
		"profile_controls_text": text(row, "profile_controls_text"),
		"search_score":          row["search_score"],
		"selection_context":     text(row, "selection_context"),
	}
	for key, value := range out {
		if value == nil || fmt.Sprint(value) == "" || fmt.Sprint(value) == "<nil>" {
			delete(out, key)
		}
	}
	return out
}

func candidateFromProfile(profile map[string]any) map[string]any {
	identity := mapValue(profile["plugin_identity"])
	if len(identity) == 0 {
		identity = mapValue(mapValue(profile["plugin_skill"])["identity"])
	}
	return map[string]any{
		"id":                    firstNonEmpty(text(profile, "profile_id"), text(identity, "profile_key")),
		"profile_id":            text(profile, "profile_id"),
		"name":                  firstNonEmpty(text(identity, "plugin_name"), text(identity, "name")),
		"manufacturer":          firstNonEmpty(text(identity, "manufacturer"), text(identity, "vendor")),
		"format":                firstNonEmpty(text(identity, "plugin_format"), text(identity, "format")),
		"plugin_path":           firstNonEmpty(text(identity, "plugin_path"), text(identity, "path")),
		"primary_type":          canonicalType(firstNonEmpty(text(profile, "class"), text(identity, "primary_class"))),
		"class":                 canonicalType(firstNonEmpty(text(profile, "class"), text(identity, "primary_class"))),
		"profile_key":           firstNonEmpty(text(identity, "profile_key"), text(profile, "profile_id")),
		"profile_controls_text": profileControlSearchText(profile),
	}
}

func candidateDedupeKeys(candidate map[string]any) []string {
	keys := []string{}
	add := func(parts ...string) {
		cleaned := []string{}
		for _, part := range parts {
			part = strings.ToLower(strings.TrimSpace(part))
			if part != "" {
				cleaned = append(cleaned, part)
			}
		}
		if len(cleaned) > 0 {
			keys = append(keys, strings.Join(cleaned, "|"))
		}
	}
	add(text(candidate, "plugin_path"))
	add(text(candidate, "identifier"))
	add(text(candidate, "id"))
	add(normalizeName(text(candidate, "name")), normalizeName(text(candidate, "manufacturer")), canonicalType(text(candidate, "primary_type")))
	add(normalizeName(text(candidate, "name")), canonicalType(text(candidate, "primary_type")))
	return compactStrings(keys)
}

func normalizeProfiles(raw []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(raw))
	for _, row := range raw {
		normalized := mapValue(row)
		if len(normalized) > 0 {
			out = append(out, normalized)
		}
	}
	return out
}

func matchProfile(candidate map[string]any, profiles []map[string]any) (map[string]any, string) {
	if text(candidate, "profile_status") == ProfileNotRequired {
		return nil, ProfileNotRequired
	}
	for _, profile := range profiles {
		if samePlugin(candidate, candidateFromProfile(profile)) {
			if profileIsStale(profile) {
				return profile, ProfileStale
			}
			return profile, ProfileReady
		}
	}
	return nil, ProfileMissing
}

func matchRackInstance(candidate map[string]any, rackPlugins []map[string]any) map[string]any {
	for _, plugin := range rackPlugins {
		if samePlugin(candidate, normalizeCandidate(plugin)) {
			return plugin
		}
	}
	return nil
}

func samePlugin(left, right map[string]any) bool {
	for _, key := range []string{"plugin_path", "identifier", "profile_id", "profile_key"} {
		l := strings.ToLower(text(left, key))
		r := strings.ToLower(text(right, key))
		if l != "" && r != "" && l == r {
			return true
		}
	}
	ln := normalizeName(text(left, "name"))
	rn := normalizeName(text(right, "name"))
	if ln != "" && rn != "" && ln == rn {
		return true
	}
	return false
}

func selectionReason(roleType string, candidate map[string]any) string {
	name := firstNonEmpty(text(candidate, "name"), text(candidate, "plugin_path"), "candidate plugin")
	source := firstNonEmpty(text(candidate, "source"), "project context")
	return fmt.Sprintf("%s matches %s via %s.", name, roleType, source)
}

func compactPlugin(candidate map[string]any) map[string]any {
	keys := []string{"id", "name", "manufacturer", "format", "category", "identifier", "plugin_path", "primary_type", "source", "profile_id", "confidence", "search_score"}
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := candidate[key]; ok && fmt.Sprint(value) != "" && fmt.Sprint(value) != "<nil>" {
			out[key] = value
		}
	}
	return out
}

func compactInstance(instance map[string]any) map[string]any {
	keys := []string{"track_id", "track", "plugin_id", "plugin_item_id", "id", "name", "plugin_name", "slot", "path", "plugin_path"}
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := instance[key]; ok && fmt.Sprint(value) != "" && fmt.Sprint(value) != "<nil>" {
			out[key] = value
		}
	}
	return out
}

func profileControls(profile map[string]any) []map[string]any {
	out := []map[string]any{}
	skill := mapValue(profile["plugin_skill"])
	for _, op := range rows(firstNonNil(skill["operations"], profile["virtual_controls"])) {
		row := map[string]any{
			"name":         firstNonEmpty(text(op, "name"), text(op, "id")),
			"component_id": text(op, "component_id"),
			"resolver":     text(op, "resolver"),
			"inputs":       op["inputs"],
			"params":       paramKeys(op["params"]),
		}
		out = append(out, row)
		if len(out) >= 8 {
			return out
		}
	}
	for _, group := range rows(firstNonNil(skill["components"], profile["groups"])) {
		row := map[string]any{
			"name":         firstNonEmpty(text(group, "label"), text(group, "id")),
			"component_id": text(group, "id"),
			"params":       paramKeys(group["params"]),
		}
		out = append(out, row)
		if len(out) >= 8 {
			return out
		}
	}
	return out
}

func profileControlSearchText(profile map[string]any) string {
	parts := []string{
		text(profile, "class"),
	}
	skill := mapValue(profile["plugin_skill"])
	for _, op := range rows(firstNonNil(skill["operations"], profile["virtual_controls"])) {
		parts = append(parts, text(op, "name"), text(op, "component_id"), strings.Join(paramKeys(op["params"]), " "))
	}
	for _, group := range rows(firstNonNil(skill["components"], profile["groups"])) {
		parts = append(parts, text(group, "id"), text(group, "label"), text(group, "role"), strings.Join(paramKeys(group["params"]), " "))
	}
	return strings.Join(compactStrings(parts), " ")
}

func paramKeys(value any) []string {
	row := mapValue(value)
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 8 {
		return keys[:8]
	}
	return keys
}

func profileIsStale(profile map[string]any) bool {
	if boolValue(profile["stale"]) || boolValue(profile["profile_stale"]) {
		return true
	}
	if len(stringList(profile["profile_stale_param_ids"])) > 0 || len(stringList(mapValue(profile["parameter_snapshot"])["profile_stale_param_ids"])) > 0 {
		return true
	}
	status := strings.ToLower(strings.Join([]string{
		text(profile, "status"),
		text(profile, "profile_status"),
		text(profile, "signature_status"),
	}, " "))
	return strings.Contains(status, "stale")
}

func stringList(value any) []string {
	switch x := value.(type) {
	case []string:
		return compactStrings(x)
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return compactStrings(out)
	default:
		return nil
	}
}

func hasProfileKey(candidate map[string]any) bool {
	return firstNonEmpty(text(candidate, "profile_id"), text(candidate, "profile_key")) != ""
}

func targetMap(target Target) map[string]any {
	return map[string]any{
		"kind":       target.Kind,
		"id":         target.ID,
		"label":      target.Label,
		"source":     target.Source,
		"confidence": target.Confidence,
	}
}

func canonicalType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case "compressor", "compression", "dynamics_processor", "dynamic", "dynamic_eq":
		return "dynamics"
	case "equalizer":
		return "eq"
	case "deesser", "de_esser":
		return "de_ess"
	case "utility", "meter", "metering":
		return "analyzer"
	default:
		return normalized
	}
}

func normalizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "")
	return replacer.Replace(value)
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func compactStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
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

func rows(value any) []map[string]any {
	switch x := value.(type) {
	case []map[string]any:
		return x
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, item := range x {
			if row := mapValue(item); len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		var out []map[string]any
		if json.Unmarshal(b, &out) == nil {
			return out
		}
		return nil
	}
}

func mapValue(value any) map[string]any {
	switch x := value.(type) {
	case map[string]any:
		return x
	case map[string]string:
		out := make(map[string]any, len(x))
		for key, val := range x {
			out[key] = val
		}
		return out
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		var out map[string]any
		if json.Unmarshal(b, &out) == nil {
			return out
		}
		return nil
	}
}

func text(row map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func intNumber(value any, fallback int) int {
	switch x := value.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		n, err := x.Int64()
		if err == nil {
			return int(n)
		}
	case string:
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(x), "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}

func boolValue(value any) bool {
	switch x := value.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
