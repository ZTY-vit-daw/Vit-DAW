package spallab

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/spal"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

// ConformTDRNova turns a learned Plugin Skill plus an explicit static-Bell
// attestation into a lab-only verified Provider instance. It does not load a
// plug-in, alter the project, or grant execution authority.
func ConformTDRNova(request TDRNovaConformanceRequest) (ProviderRecord, error) {
	if strings.TrimSpace(request.TargetRef) == "" || strings.TrimSpace(request.TrackID) == "" || strings.TrimSpace(request.PluginID) == "" {
		return ProviderRecord{}, fmt.Errorf("target_ref, track_id and plugin_id are required")
	}
	if !request.StaticBellConfirmed {
		return ProviderRecord{}, fmt.Errorf("TDR Nova static Bell must be explicitly confirmed in the lab")
	}
	if refs := canonicalStrings(request.StaticBellEvidence); len(refs) == 0 {
		return ProviderRecord{}, fmt.Errorf("static Bell confirmation requires evidence")
	}
	if len(canonicalStrings(request.ObservedParamIDs)) == 0 {
		return ProviderRecord{}, fmt.Errorf("a current observed parameter set is required for conformance")
	}
	if request.PluginSkill.SchemaVersion != plugingrabber.PluginSkillSchemaVersion {
		return ProviderRecord{}, fmt.Errorf("Plugin Skill schema_version must be %d", plugingrabber.PluginSkillSchemaVersion)
	}
	if strings.TrimSpace(request.PluginSkill.Identity.ParamSignatureHash) == "" {
		return ProviderRecord{}, fmt.Errorf("Plugin Skill parameter signature is required for SPAL lab conformance")
	}
	if observed := strings.TrimSpace(request.ObservedParamHash); observed != "" && observed != strings.TrimSpace(request.PluginSkill.Identity.ParamSignatureHash) {
		return ProviderRecord{}, fmt.Errorf("Plugin Skill parameter signature does not match the current plug-in readback")
	}
	if !request.StaticBellInvariant.Valid() {
		return ProviderRecord{}, fmt.Errorf("static Bell conformance requires the current verified filter-type parameter invariant")
	}

	bandSlot := strings.ToLower(strings.TrimSpace(request.BandSlot))
	profile, err := tdrNovaProfileFromSkill(request.PluginSkill, bandSlot, request.StaticBellConfirmed)
	if err != nil {
		return ProviderRecord{}, err
	}
	if err := requireObservedMappings(profile, bandSlot, request.ObservedParamIDs, request.StaticBellInvariant.ParameterID); err != nil {
		return ProviderRecord{}, err
	}
	adapter, err := spal.NewExperimentalTDRNovaAdapter()
	if err != nil {
		return ProviderRecord{}, err
	}
	report, err := adapter.Conform(profile, bandSlot)
	if err != nil {
		return ProviderRecord{}, fmt.Errorf("TDR Nova SPAL conformance: %w", err)
	}

	fingerprintValues := []string{request.TargetRef, request.TrackID, request.PluginID, bandSlot, request.PluginSkill.Identity.ParamSignatureHash}
	if projectUUID := strings.TrimSpace(request.ProjectUUID); projectUUID != "" {
		fingerprintValues = append(fingerprintValues, projectUUID)
	}
	instanceID := "spal_lab_tdr_nova_" + shortFingerprint(fingerprintValues...)
	metadata := map[string]string{
		"band_slot":              bandSlot,
		"static_bell_ready":      "true",
		"spal_lab_only":          "true",
		"plugin_skill_signature": strings.TrimSpace(request.PluginSkill.Identity.ParamSignatureHash),
	}
	if projectUUID := strings.TrimSpace(request.ProjectUUID); projectUUID != "" {
		metadata["project_uuid"] = projectUUID
	}
	instance := spal.ProviderInstance{
		ID:              instanceID,
		ProviderID:      report.ProviderID,
		TargetRef:       strings.TrimSpace(request.TargetRef),
		TrackID:         strings.TrimSpace(request.TrackID),
		PluginID:        strings.TrimSpace(request.PluginID),
		PluginSignature: spal.ExperimentalTDRNovaSignature,
		Status:          spal.InstanceVerified,
		Metadata:        metadata,
	}
	now := time.Now().UTC()
	record := ProviderRecord{
		SchemaVersion:             SchemaVersion,
		ID:                        instanceID,
		ProjectUUID:               strings.TrimSpace(request.ProjectUUID),
		LabOnly:                   true,
		ProviderID:                report.ProviderID,
		Instance:                  instance,
		PluginSkillSignature:      strings.TrimSpace(request.PluginSkill.Identity.ParamSignatureHash),
		CurrentParameterSignature: firstNonEmpty(strings.TrimSpace(request.ObservedParamHash), strings.TrimSpace(request.PluginSkill.Identity.ParamSignatureHash)),
		PluginProfileKey:          strings.TrimSpace(request.PluginSkill.Identity.ProfileKey),
		PluginVersion:             strings.TrimSpace(request.PluginSkill.Identity.Version),
		StaticBellInvariant:       request.StaticBellInvariant,
		StaticBellEvidence:        canonicalStrings(request.StaticBellEvidence),
		ConformanceEvidence:       canonicalStrings(append(report.EvidenceRefs, request.StaticBellEvidence...)),
		ObservedParameterIDs:      canonicalStrings(request.ObservedParamIDs),
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := record.Valid(); err != nil {
		return ProviderRecord{}, err
	}
	return record, nil
}

// ConformTDRNovaFromPluginReply is the live-readback variant used by the CLI.
// It prevents an old Skill document from being bound to a differently shaped
// current plug-in instance.
func ConformTDRNovaFromPluginReply(request TDRNovaConformanceRequest, reply map[string]any) (ProviderRecord, error) {
	skill, err := PluginSkillFromReply(reply)
	if err != nil {
		return ProviderRecord{}, err
	}
	request.PluginSkill = skill
	request.ObservedParamIDs = observedParameterIDs(reply)
	request.ObservedParamHash = firstString(reply, "current_param_signature_hash")
	if request.ObservedParamHash == "" {
		request.ObservedParamHash = firstString(reply, "param_signature_hash")
	}
	storedSignature := firstString(reply, "profile_param_signature_hash")
	if storedSignature != "" && request.ObservedParamHash != "" && storedSignature != request.ObservedParamHash {
		return ProviderRecord{}, fmt.Errorf("current plug-in parameter signature does not match its saved Plugin Skill profile")
	}
	invariant, err := staticBellInvariantFromReply(skill, request.BandSlot, reply)
	if err != nil {
		return ProviderRecord{}, err
	}
	request.StaticBellInvariant = invariant
	return ConformTDRNova(request)
}

func (r ProviderRecord) Valid() error {
	if r.SchemaVersion != SchemaVersion || strings.TrimSpace(r.ID) == "" || !r.LabOnly {
		return fmt.Errorf("SPAL lab record schema_version, id and lab_only are required")
	}
	if r.ProviderID != spal.ExperimentalTDRNovaProviderID || r.Instance.ProviderID != r.ProviderID {
		return fmt.Errorf("SPAL lab record must use the experimental TDR Nova Provider")
	}
	if err := r.Instance.Valid(); err != nil {
		return err
	}
	if r.Instance.Status != spal.InstanceVerified || r.Instance.PluginSignature != spal.ExperimentalTDRNovaSignature {
		return fmt.Errorf("SPAL lab record is not a verified TDR Nova fixture instance")
	}
	if strings.TrimSpace(r.PluginSkillSignature) == "" || strings.TrimSpace(r.CurrentParameterSignature) == "" || !r.StaticBellInvariant.Valid() || len(canonicalStrings(r.StaticBellEvidence)) == 0 || len(canonicalStrings(r.ConformanceEvidence)) == 0 || len(canonicalStrings(r.ObservedParameterIDs)) == 0 {
		return fmt.Errorf("SPAL lab record omits conformance provenance")
	}
	if !strings.EqualFold(r.Instance.Metadata["static_bell_ready"], "true") || !strings.EqualFold(r.Instance.Metadata["spal_lab_only"], "true") {
		return fmt.Errorf("SPAL lab record is missing static Bell or lab-only invariants")
	}
	if r.ProjectUUID != "" && r.Instance.Metadata["project_uuid"] != r.ProjectUUID {
		return fmt.Errorf("SPAL provider record project scope does not match the instance metadata")
	}
	return nil
}

// PluginSkillFromReply extracts the v2 learned document emitted by the
// existing Plugin Grabber `get_plugin_parameters` path.
func PluginSkillFromReply(reply map[string]any) (plugingrabber.PluginSkillDocument, error) {
	raw := any(nil)
	if reply != nil {
		raw = reply["plugin_skill"]
		if raw == nil {
			if profile := mapValue(reply["global_profile"]); profile != nil {
				raw = profile["plugin_skill"]
			}
		}
	}
	if raw == nil {
		return plugingrabber.PluginSkillDocument{}, fmt.Errorf("current plug-in has no learned v2 Plugin Skill; run Plugin Learning first")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return plugingrabber.PluginSkillDocument{}, fmt.Errorf("encode learned Plugin Skill: %w", err)
	}
	var skill plugingrabber.PluginSkillDocument
	if err := json.Unmarshal(data, &skill); err != nil {
		return plugingrabber.PluginSkillDocument{}, fmt.Errorf("decode learned Plugin Skill: %w", err)
	}
	if skill.SchemaVersion != plugingrabber.PluginSkillSchemaVersion {
		return plugingrabber.PluginSkillDocument{}, fmt.Errorf("current plug-in does not expose Plugin Skill schema v%d", plugingrabber.PluginSkillSchemaVersion)
	}
	return skill, nil
}

func tdrNovaProfileFromSkill(skill plugingrabber.PluginSkillDocument, bandSlot string, staticBellReady bool) (spal.TDRNovaConformanceProfile, error) {
	if strings.TrimSpace(skill.Identity.ProfileKey) != "plugin_f9788adda4203df8" || !strings.EqualFold(strings.TrimSpace(skill.Identity.Name), "TDR Nova") || !strings.EqualFold(strings.TrimSpace(skill.Identity.Format), "VST3") || strings.TrimSpace(skill.Identity.Version) != "2.2.2" {
		return spal.TDRNovaConformanceProfile{}, fmt.Errorf("Plugin Skill identity is not the TDR Nova 2.2.2 experimental fixture")
	}
	component, ok := tdrNovaSkillComponent(skill.Components, bandSlot)
	if !ok {
		return spal.TDRNovaConformanceProfile{}, fmt.Errorf("Plugin Skill does not expose component %s", bandSlot)
	}
	params := make(map[string]spal.TDRNovaParameterConformance, 5)
	for _, slot := range []string{"enable", "dyn_enable", "frequency", "gain", "q"} {
		mapping, ok := component.Params[slot]
		if !ok {
			return spal.TDRNovaConformanceProfile{}, fmt.Errorf("Plugin Skill component %s omits %s", bandSlot, slot)
		}
		if !mapping.Confirmed || mapping.DisplayDomain == nil || mapping.DisplayDomain.Min == nil || mapping.DisplayDomain.Max == nil {
			return spal.TDRNovaConformanceProfile{}, fmt.Errorf("Plugin Skill component %s %s mapping is not confirmed with a bounded display domain", bandSlot, slot)
		}
		params[slot] = spal.TDRNovaParameterConformance{
			ParameterID: strings.TrimSpace(mapping.ParamID), Unit: strings.TrimSpace(mapping.DisplayDomain.Unit),
			Min: *mapping.DisplayDomain.Min, Max: *mapping.DisplayDomain.Max, Confirmed: mapping.Confirmed,
		}
	}
	return spal.TDRNovaConformanceProfile{
		ProfileKey:         skill.Identity.ProfileKey,
		PluginName:         skill.Identity.Name,
		PluginFormat:       skill.Identity.Format,
		PluginVersion:      skill.Identity.Version,
		ParameterSignature: spal.ExperimentalTDRNovaSignature,
		Bands: map[string]spal.TDRNovaBandConformance{
			bandSlot: {StaticBellReady: staticBellReady, Parameters: params},
		},
	}, nil
}

func requireObservedMappings(profile spal.TDRNovaConformanceProfile, bandSlot string, observed []string, extraIDs ...string) error {
	known := make(map[string]bool, len(observed))
	for _, id := range observed {
		if id = strings.TrimSpace(id); id != "" {
			known[id] = true
		}
	}
	band, ok := profile.Bands[bandSlot]
	if !ok {
		return fmt.Errorf("conformance profile omits %s", bandSlot)
	}
	for slot, mapping := range band.Parameters {
		if !known[mapping.ParameterID] {
			return fmt.Errorf("current plug-in readback omits conformed %s parameter %s", slot, mapping.ParameterID)
		}
	}
	for _, id := range extraIDs {
		if id = strings.TrimSpace(id); id != "" && !known[id] {
			return fmt.Errorf("current plug-in readback omits conformed static Bell parameter %s", id)
		}
	}
	return nil
}

func skillComponent(components []plugingrabber.PluginSkillComponent, id string) (plugingrabber.PluginSkillComponent, bool) {
	for _, component := range components {
		if strings.EqualFold(strings.TrimSpace(component.ID), strings.TrimSpace(id)) {
			return component, true
		}
	}
	return plugingrabber.PluginSkillComponent{}, false
}

// tdrNovaSkillComponent accepts the short b1..b4 IDs emitted by the generic
// Plugin Learning EQ-band draft as aliases for the adapter's explicit
// band1..band4 conformance slots.  This is intentionally scoped to the known
// TDR Nova fixture; it is not a generic fuzzy component resolver.
func tdrNovaSkillComponent(components []plugingrabber.PluginSkillComponent, bandSlot string) (plugingrabber.PluginSkillComponent, bool) {
	if component, ok := skillComponent(components, bandSlot); ok {
		return component, true
	}
	bandSlot = strings.ToLower(strings.TrimSpace(bandSlot))
	if len(bandSlot) == len("band1") && strings.HasPrefix(bandSlot, "band") {
		return skillComponent(components, "b"+bandSlot[len("band"):])
	}
	return plugingrabber.PluginSkillComponent{}, false
}

func observedParameterIDs(reply map[string]any) []string {
	rows := rowsValue(reply["parameters"])
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if id := firstString(row, "id", "param_id", "parameter_id", "raw_param_id"); id != "" {
			out = append(out, id)
		}
	}
	return canonicalStrings(out)
}

// ValidateProviderRecordCurrent verifies the live profile layout and the
// manually conformed Bell-type state immediately before planning. It does not
// mutate the plug-in; it only prevents a stale registration from being used.
func ValidateProviderRecordCurrent(record ProviderRecord, reply map[string]any) error {
	if err := record.Valid(); err != nil {
		return err
	}
	if strings.EqualFold(firstString(reply, "status"), "error") {
		return fmt.Errorf("current plug-in readback failed: %s", firstString(reply, "message", "error"))
	}
	if trackID := firstString(reply, "track_id"); trackID != "" && trackID != record.Instance.TrackID {
		return fmt.Errorf("current plug-in readback track does not match verified Provider instance")
	}
	if pluginID := firstString(reply, "plugin_id", "plugin_item_id"); pluginID != "" && pluginID != record.Instance.PluginID {
		return fmt.Errorf("current plug-in readback instance does not match verified Provider instance")
	}
	currentSignature := firstString(reply, "current_param_signature_hash", "param_signature_hash")
	if currentSignature == "" || currentSignature != record.CurrentParameterSignature {
		return fmt.Errorf("current plug-in parameter signature does not match the verified SPAL lab instance")
	}
	if profileSignature := firstString(reply, "profile_param_signature_hash"); profileSignature != "" && profileSignature != currentSignature {
		return fmt.Errorf("current plug-in parameter signature does not match the persisted Plugin Skill profile")
	}
	skill, err := PluginSkillFromReply(reply)
	if err != nil {
		return err
	}
	if strings.TrimSpace(skill.Identity.ParamSignatureHash) != record.PluginSkillSignature {
		return fmt.Errorf("current Plugin Skill signature does not match the verified SPAL lab instance")
	}
	bandSlot := strings.TrimSpace(record.Instance.Metadata["band_slot"])
	profile, err := tdrNovaProfileFromSkill(skill, bandSlot, true)
	if err != nil {
		return err
	}
	if err := requireObservedMappings(profile, strings.ToLower(bandSlot), observedParameterIDs(reply), record.StaticBellInvariant.ParameterID); err != nil {
		return err
	}
	invariant, err := staticBellInvariantFromReply(skill, bandSlot, reply)
	if err != nil {
		return err
	}
	if invariant.ParameterID != record.StaticBellInvariant.ParameterID || !sameFloat(invariant.Value, record.StaticBellInvariant.Value) || !sameFloat(invariant.NormalizedValue, record.StaticBellInvariant.NormalizedValue) {
		return fmt.Errorf("current filter-type state no longer matches the verified static Bell invariant")
	}
	if record.StaticBellInvariant.ValueText != "" && invariant.ValueText != "" && invariant.ValueText != record.StaticBellInvariant.ValueText {
		return fmt.Errorf("current filter-type display text no longer matches the verified static Bell invariant")
	}
	return nil
}

func staticBellInvariantFromReply(skill plugingrabber.PluginSkillDocument, bandSlot string, reply map[string]any) (StaticBellInvariant, error) {
	component, ok := tdrNovaSkillComponent(skill.Components, strings.ToLower(strings.TrimSpace(bandSlot)))
	if !ok {
		return StaticBellInvariant{}, fmt.Errorf("Plugin Skill does not expose component %s for static Bell verification", bandSlot)
	}
	typeMapping, ok := component.Params["type"]
	if !ok || strings.TrimSpace(typeMapping.ParamID) == "" {
		return StaticBellInvariant{}, fmt.Errorf("Plugin Skill component %s does not expose a filter-type mapping for static Bell verification", bandSlot)
	}
	row, ok := parameterRow(reply, typeMapping.ParamID)
	if !ok {
		return StaticBellInvariant{}, fmt.Errorf("current plug-in readback omits static Bell filter-type parameter %s", typeMapping.ParamID)
	}
	value, ok := numberFromAny(row["value"])
	if !ok {
		return StaticBellInvariant{}, fmt.Errorf("static Bell filter-type parameter %s has no numeric current value", typeMapping.ParamID)
	}
	normalized, ok := numberFromAny(firstPresent(row, "normalized_value", "normalised_value"))
	if !ok {
		return StaticBellInvariant{}, fmt.Errorf("static Bell filter-type parameter %s has no numeric normalized value", typeMapping.ParamID)
	}
	return StaticBellInvariant{
		ParameterID: typeMapping.ParamID, Value: value, NormalizedValue: normalized,
		ValueText: firstString(row, "value_text", "display_text", "current_text"),
	}, nil
}

func parameterRow(reply map[string]any, id string) (map[string]any, bool) {
	for _, row := range rowsValue(reply["parameters"]) {
		if firstString(row, "id", "param_id", "parameter_id", "raw_param_id") == strings.TrimSpace(id) {
			return row, true
		}
	}
	return nil, false
}

func numberFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case float32:
		return numberFromAny(float64(typed))
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
	default:
		return 0, false
	}
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return nil
}

func sameFloat(left, right float64) bool { return math.Abs(left-right) <= 0.0001 }

func rowsValue(value any) []map[string]any {
	if rows, ok := value.([]map[string]any); ok {
		return rows
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var rows []map[string]any
	if json.Unmarshal(data, &rows) != nil {
		return nil
	}
	return rows
}

func mapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var row map[string]any
	if json.Unmarshal(data, &row) != nil {
		return nil
	}
	return row
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(row[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func canonicalStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func shortFingerprint(values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return hex.EncodeToString(sum[:])[:20]
}
