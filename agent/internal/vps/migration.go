package vps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/spallab"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	migrationSourcePluginSkillV2 = "plugin_skill_document_v2"
	migrationSourceProfilePatch  = "profile_patch"
	migrationSourceLegacyVPS     = "legacy_vps"
	migrationSourceSPALV0        = "spal_v0_provider_record"
)

// MigratePluginSkillDocumentV2 imports a learned PluginSkillDocument as a VPS
// draft. The v2 signature is retained as legacy evidence only: it cannot
// become a v3 Verified Credential without a full fresh fingerprint and
// conformance run.
func MigratePluginSkillDocumentV2(skill plugingrabber.PluginSkillDocument, now time.Time) (VPSDocument, error) {
	if skill.SchemaVersion != plugingrabber.PluginSkillSchemaVersion {
		return VPSDocument{}, fmt.Errorf("Plugin Skill schema_version must be %d", plugingrabber.PluginSkillSchemaVersion)
	}
	identity := PluginIdentity{
		Manufacturer: strings.TrimSpace(skill.Identity.Manufacturer),
		Name:         strings.TrimSpace(skill.Identity.Name),
		Format:       strings.TrimSpace(skill.Identity.Format),
		Version:      strings.TrimSpace(skill.Identity.Version),
		ProfileKey:   strings.TrimSpace(skill.Identity.ProfileKey),
		InstallPath:  strings.TrimSpace(skill.Identity.Path),
		Fingerprint: PluginFingerprint{
			LegacyParameterSignature: strings.TrimSpace(skill.Identity.ParamSignatureHash),
		},
	}
	if err := identity.validDraft(); err != nil {
		return VPSDocument{}, fmt.Errorf("Plugin Skill identity: %w", err)
	}
	legacyID := firstNonEmpty(skill.Identity.ProfileKey, skill.Identity.ParamSignatureHash, skill.Identity.Name)
	document := newMigrationDraft(identity, migrationSourcePluginSkillV2, legacyID, now)
	document.RawObservations = []RawObservation{rawObservation(migrationSourcePluginSkillV2, "Imported PluginSkillDocument v2; requires VPS v3 requalification.", skill, now)}
	document.ControlSurface, document.Topology = controlSurfaceAndTopologyFromSkill(skill)
	document.SemanticCapabilities = semanticCapabilitiesFromSkill(skill)
	document.CapabilityProfiles = profilesForCapabilities(document.SemanticCapabilities)
	document.SafetyAndRollback = SafetyAndRollback{
		Preconditions: []string{"Legacy Plugin Skill evidence must be revalidated against a fresh VPS v3 fingerprint."},
		Invalidators:  []string{"plugin installation fingerprint changed", "parameter-surface signature changed", "display-surface signature changed"},
		LegacyPolicy:  rawJSON(skill.Safety),
	}
	document.ConformanceEvidence = []string{"migration:" + migrationSourcePluginSkillV2}
	if err := document.Validate(); err != nil {
		return VPSDocument{}, err
	}
	return document, nil
}

// MigrateProfilePatch imports the older UI/semantic patch shape as evidence.
// The patch is useful for control-surface learning, but contains no authority
// to issue a Provider Credential.
func MigrateProfilePatch(identity PluginIdentity, patch plugingrabber.ProfilePatch, now time.Time) (VPSDocument, error) {
	if strings.TrimSpace(identity.Name) == "" && strings.TrimSpace(identity.ProfileKey) == "" {
		identity.Name = "Legacy Plugin Profile"
		identity.ProfileKey = "legacy_profile_" + shortFingerprint(patch)
	}
	document := newMigrationDraft(identity, migrationSourceProfilePatch, firstNonEmpty(identity.ProfileKey, identity.Name), now)
	document.RawObservations = []RawObservation{rawObservation(migrationSourceProfilePatch, "Imported legacy ProfilePatch; parameter mappings remain candidates.", patch, now)}
	document.ControlSurface = controlSurfaceFromProfilePatch(patch)
	document.SemanticCapabilities = semanticCapabilitiesFromProfilePatch(patch)
	document.CapabilityProfiles = profilesForCapabilities(document.SemanticCapabilities)
	document.SafetyAndRollback = SafetyAndRollback{
		Preconditions: []string{"Legacy profile patch is evidence only until VPS v3 conformance completes."},
		Invalidators:  []string{"parameter-surface signature changed", "display-surface signature changed"},
		LegacyPolicy:  rawJSON(patch.Safety),
	}
	document.ConformanceEvidence = []string{"migration:" + migrationSourceProfilePatch}
	if err := document.Validate(); err != nil {
		return VPSDocument{}, err
	}
	return document, nil
}

// MigrateLegacyVPS recognizes the historical .vps artifact envelope. It
// accepts either a v2 Plugin Skill or a ProfilePatch payload and always emits
// a non-dispatchable VPS draft.
func MigrateLegacyVPS(data []byte, now time.Time) (VPSDocument, error) {
	data = bytesTrimSpace(data)
	if len(data) == 0 {
		return VPSDocument{}, fmt.Errorf("legacy .vps payload is empty")
	}
	var directSkill plugingrabber.PluginSkillDocument
	if err := json.Unmarshal(data, &directSkill); err == nil && directSkill.SchemaVersion == plugingrabber.PluginSkillSchemaVersion {
		return MigratePluginSkillDocumentV2(directSkill, now)
	}

	var envelope legacyVPSEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return VPSDocument{}, fmt.Errorf("decode legacy .vps: %w", err)
	}
	if len(envelope.PluginSkill) > 0 {
		var skill plugingrabber.PluginSkillDocument
		if err := json.Unmarshal(envelope.PluginSkill, &skill); err == nil && skill.SchemaVersion == plugingrabber.PluginSkillSchemaVersion {
			return MigratePluginSkillDocumentV2(skill, now)
		}
	}
	patchPayload := envelope.ProfilePatch
	if len(patchPayload) == 0 {
		patchPayload = envelope.PluginSkill
	}
	var patch plugingrabber.ProfilePatch
	if len(patchPayload) == 0 || json.Unmarshal(patchPayload, &patch) != nil {
		return VPSDocument{}, fmt.Errorf("legacy .vps does not contain a supported Plugin Skill v2 or ProfilePatch")
	}
	identity := identityFromLegacyEnvelope(envelope)
	document, err := MigrateProfilePatch(identity, patch, now)
	if err != nil {
		return VPSDocument{}, err
	}
	document.ID = migrationDocumentID(migrationSourceLegacyVPS, document.PluginIdentity, firstNonEmpty(envelope.Schema, envelope.Target.PluginID, envelope.Target.PluginName))
	document.Migration.Sources = uniqueSorted(append(document.Migration.Sources, migrationSourceLegacyVPS))
	document.RawObservations = append(document.RawObservations, rawObservation(migrationSourceLegacyVPS, "Imported historical .vps artifact envelope.", json.RawMessage(data), now))
	return document, nil
}

// MigrateSPALV0ProviderRecord preserves the existing TDR Nova v0 record as a
// pending-requalification sample. The source record is project/instance scoped
// and therefore cannot become a reusable v3 Credential automatically.
func MigrateSPALV0ProviderRecord(record spallab.ProviderRecord, now time.Time) (VPSDocument, error) {
	if err := record.Valid(); err != nil {
		return VPSDocument{}, fmt.Errorf("SPAL v0 provider record: %w", err)
	}
	identity := PluginIdentity{
		Manufacturer: "Tokyo Dawn Labs",
		Name:         "TDR Nova",
		Format:       "VST3",
		Version:      strings.TrimSpace(record.PluginVersion),
		ProfileKey:   strings.TrimSpace(record.PluginProfileKey),
		Fingerprint: PluginFingerprint{
			LegacyParameterSignature: strings.TrimSpace(record.CurrentParameterSignature),
		},
	}
	document := newMigrationDraft(identity, migrationSourceSPALV0, record.ID, now)
	if !record.CreatedAt.IsZero() {
		document.CreatedAt = record.CreatedAt.UTC()
	}
	document.RawObservations = []RawObservation{rawObservation(migrationSourceSPALV0, "Imported SPAL v0 TDR Nova ProviderRecord; it is not a v3 Provider Credential.", record, now)}
	bandSlot := strings.TrimSpace(record.Instance.Metadata["band_slot"])
	document.Topology = Topology{
		Components: []TopologyComponent{{ID: firstNonEmpty(bandSlot, "legacy_band"), Role: "eq_band", Label: "Legacy SPAL v0 observed band", Slots: []string{"filter_type", "frequency_hz", "gain_db", "q", "enabled"}}},
		Resources:  []PhysicalResource{{ID: "legacy:" + firstNonEmpty(bandSlot, "band"), Kind: "eq_band", ComponentID: firstNonEmpty(bandSlot, "legacy_band"), Capacity: 1, Constraints: []string{"legacy observation only; no v3 lease or dispatch authority"}}},
	}
	document.SemanticCapabilities = []SemanticCapability{{
		ID:           StaticEQCapabilityID,
		Operations:   []string{OperationBellCut},
		Parameters:   []string{"filter_type", "frequency_hz", "gain_db", "q", "enabled"},
		FilterTypes:  []string{"bell"},
		Status:       string(CredentialPendingRequalification),
		EvidenceRefs: uniqueSorted(record.ConformanceEvidence),
	}}
	document.CapabilityProfiles = []CapabilityConformanceProfile{StaticEQConformanceProfileV0()}
	document.ProviderCredentials = []ProviderCredential{{
		ID:                 "legacy_spal_v0_" + record.ID,
		Revision:           1,
		CapabilityID:       StaticEQCapabilityID,
		Status:             CredentialPendingRequalification,
		PluginFingerprint:  identity.Fingerprint,
		EvidenceRefs:       uniqueSorted(append(append([]string(nil), record.ConformanceEvidence...), record.StaticBellEvidence...)),
		MigrationSource:    migrationSourceSPALV0,
		InvalidationReason: "SPAL v0 ProviderRecord is an instance-scoped laboratory record and requires VPS v3 requalification.",
	}}
	document.ConformanceEvidence = uniqueSorted(append([]string{"migration:" + migrationSourceSPALV0}, record.ConformanceEvidence...))
	document.SafetyAndRollback = SafetyAndRollback{
		Preconditions: []string{"Retain v0 record only as evidence; re-run v3 control-surface, boundary and rollback conformance."},
		RollbackMode:  "legacy_reference_eq_v0",
		Invalidators:  []string{"plugin installation fingerprint changed", "parameter-surface signature changed", "display-surface signature changed"},
	}
	if err := document.Validate(); err != nil {
		return VPSDocument{}, err
	}
	return document, nil
}

type legacyVPSEnvelope struct {
	Schema         string          `json:"schema"`
	Target         legacyVPSTarget `json:"target"`
	PluginIdentity map[string]any  `json:"plugin_identity"`
	ProfilePatch   json.RawMessage `json:"profile_patch"`
	PluginSkill    json.RawMessage `json:"plugin_skill"`
}

type legacyVPSTarget struct {
	PluginID   string `json:"plugin_id"`
	PluginName string `json:"plugin_name"`
}

func identityFromLegacyEnvelope(envelope legacyVPSEnvelope) PluginIdentity {
	value := func(keys ...string) string {
		for _, key := range keys {
			if raw, ok := envelope.PluginIdentity[key]; ok {
				if text := strings.TrimSpace(fmt.Sprint(raw)); text != "" && text != "<nil>" {
					return text
				}
			}
		}
		return ""
	}
	return PluginIdentity{
		Manufacturer: value("manufacturer", "maker"),
		Name:         firstNonEmpty(value("plugin_name", "name"), envelope.Target.PluginName),
		Format:       value("plugin_format", "format"),
		Version:      value("version"),
		ProfileKey:   value("profile_key"),
		InstallPath:  value("plugin_path", "path"),
	}
}

func newMigrationDraft(identity PluginIdentity, source, legacyID string, now time.Time) VPSDocument {
	document := NewDraft(identity, now)
	document.ID = migrationDocumentID(source, identity, legacyID)
	document.Status = VPSStatusDraft
	document.Migration = MigrationMetadata{
		Sources:                 []string{source},
		LegacyIDs:               uniqueSorted([]string{legacyID}),
		RequiresRequalification: true,
		ImportedAt:              now.UTC(),
	}
	return document
}

func controlSurfaceAndTopologyFromSkill(skill plugingrabber.PluginSkillDocument) (ControlSurface, Topology) {
	mappings := []ControlSurfaceMapping{}
	components := []TopologyComponent{}
	resources := []PhysicalResource{}
	for _, component := range skill.Components {
		slots := make([]string, 0, len(component.Params))
		for slot, mapping := range component.Params {
			slots = append(slots, slot)
			mappings = append(mappings, ControlSurfaceMapping{
				ComponentID:   component.ID,
				SemanticSlot:  slot,
				ParameterID:   mapping.ParamID,
				Label:         mapping.Label,
				DisplayDomain: displayDomainFromPluginSkill(mapping.DisplayDomain),
				Confirmed:     mapping.Confirmed,
				EvidenceRefs:  evidenceRefs(mapping.Evidence),
			})
		}
		sort.Strings(slots)
		components = append(components, TopologyComponent{ID: component.ID, Role: component.Role, Label: component.Label, Slots: slots})
		if componentHasStaticEQShape(component) {
			resources = append(resources, PhysicalResource{ID: "slot:" + component.ID, Kind: "eq_band", ComponentID: component.ID, Capacity: 1, Constraints: []string{"candidate topology; allocation requires v3 conformance"}})
		}
	}
	sort.Slice(mappings, func(i, j int) bool {
		if mappings[i].ComponentID == mappings[j].ComponentID {
			return mappings[i].SemanticSlot < mappings[j].SemanticSlot
		}
		return mappings[i].ComponentID < mappings[j].ComponentID
	})
	sort.Slice(components, func(i, j int) bool { return components[i].ID < components[j].ID })
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	return ControlSurface{Mappings: mappings, MacroRelations: macroRelationsFromSkill(skill)}, Topology{Components: components, Resources: resources}
}

func semanticCapabilitiesFromSkill(skill plugingrabber.PluginSkillDocument) []SemanticCapability {
	capabilities := []SemanticCapability{}
	operations := operationNamesFromSkill(skill)
	for _, capability := range uniqueSorted(skill.Capabilities.Types) {
		capabilities = append(capabilities, SemanticCapability{ID: capability, Operations: operations, Status: string(CredentialCandidate), EvidenceRefs: []string{"migration:" + migrationSourcePluginSkillV2}})
	}
	for _, component := range skill.Components {
		if !componentHasStaticEQShape(component) {
			continue
		}
		capabilities = append(capabilities, SemanticCapability{
			ID:           StaticEQCapabilityID,
			Operations:   uniqueSorted(append([]string{OperationBellCut}, operations...)),
			Parameters:   []string{"filter_type", "frequency_hz", "gain_db", "q", "enabled"},
			FilterTypes:  []string{"bell"},
			Status:       string(CredentialCandidate),
			EvidenceRefs: []string{"migration:" + migrationSourcePluginSkillV2},
		})
		break
	}
	return uniqueCapabilities(capabilities)
}

func controlSurfaceFromProfilePatch(patch plugingrabber.ProfilePatch) ControlSurface {
	ids := map[string]bool{}
	for _, id := range patch.QuickControlIDs {
		ids[strings.TrimSpace(id)] = true
	}
	for id := range patch.Aliases {
		ids[strings.TrimSpace(id)] = true
	}
	for id := range patch.DisplayGroups {
		ids[strings.TrimSpace(id)] = true
	}
	for id := range patch.NormalizedRoles {
		ids[strings.TrimSpace(id)] = true
	}
	keys := make([]string, 0, len(ids))
	for id := range ids {
		if id != "" {
			keys = append(keys, id)
		}
	}
	sort.Strings(keys)
	mappings := make([]ControlSurfaceMapping, 0, len(keys))
	for _, id := range keys {
		mappings = append(mappings, ControlSurfaceMapping{
			ComponentID:  strings.TrimSpace(patch.DisplayGroups[id]),
			SemanticSlot: strings.TrimSpace(patch.NormalizedRoles[id]),
			ParameterID:  id,
			Label:        strings.TrimSpace(patch.Aliases[id]),
			EvidenceRefs: []string{"migration:" + migrationSourceProfilePatch},
		})
	}
	return ControlSurface{Mappings: mappings, MacroRelations: macroRelationsFromProfilePatch(patch)}
}

func semanticCapabilitiesFromProfilePatch(patch plugingrabber.ProfilePatch) []SemanticCapability {
	class := strings.ToLower(strings.TrimSpace(patch.Class))
	if !strings.Contains(class, "eq") {
		return nil
	}
	return []SemanticCapability{{
		ID:           StaticEQCapabilityID,
		Operations:   uniqueSorted(append([]string{OperationBellCut}, operationNamesFromProfilePatch(patch)...)),
		Parameters:   []string{"filter_type", "frequency_hz", "gain_db", "q", "enabled"},
		FilterTypes:  []string{"bell"},
		Status:       string(CredentialCandidate),
		EvidenceRefs: []string{"migration:" + migrationSourceProfilePatch},
	}}
}

func macroRelationsFromSkill(skill plugingrabber.PluginSkillDocument) []MacroRelation {
	relations := make([]MacroRelation, 0, len(skill.Operations))
	for _, operation := range skill.Operations {
		id := strings.TrimSpace(operation.Name)
		if id == "" {
			continue
		}
		relations = append(relations, MacroRelation{
			ID:          id,
			Inputs:      uniqueSorted(operation.Inputs),
			Resolver:    strings.TrimSpace(operation.Resolver),
			ComponentID: strings.TrimSpace(operation.ComponentID),
			Parameters:  rawJSON(operation.Params),
		})
	}
	sort.Slice(relations, func(i, j int) bool { return relations[i].ID < relations[j].ID })
	return relations
}

func macroRelationsFromProfilePatch(patch plugingrabber.ProfilePatch) []MacroRelation {
	relations := make([]MacroRelation, 0, len(patch.VirtualControls))
	for _, control := range patch.VirtualControls {
		id := firstNonEmpty(textFromMap(control, "name", "operation"))
		if id == "" {
			continue
		}
		relations = append(relations, MacroRelation{
			ID:          id,
			Inputs:      stringsFromAny(control["inputs"]),
			Resolver:    textFromMap(control, "resolver"),
			ComponentID: textFromMap(control, "component_id", "component"),
			Parameters:  rawJSON(control["params"]),
		})
	}
	sort.Slice(relations, func(i, j int) bool { return relations[i].ID < relations[j].ID })
	return relations
}

func operationNamesFromSkill(skill plugingrabber.PluginSkillDocument) []string {
	operations := make([]string, 0, len(skill.Operations))
	for _, operation := range skill.Operations {
		operations = append(operations, operation.Name)
	}
	return uniqueSorted(operations)
}

func operationNamesFromProfilePatch(patch plugingrabber.ProfilePatch) []string {
	operations := make([]string, 0, len(patch.VirtualControls))
	for _, control := range patch.VirtualControls {
		operations = append(operations, textFromMap(control, "name", "operation"))
	}
	return uniqueSorted(operations)
}

func textFromMap(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := strings.TrimSpace(fmt.Sprint(values[key])); text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func stringsFromAny(value any) []string {
	values := []string{}
	switch typed := value.(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		for _, item := range typed {
			values = append(values, fmt.Sprint(item))
		}
	}
	return uniqueSorted(values)
}

func componentHasStaticEQShape(component plugingrabber.PluginSkillComponent) bool {
	if component.Params == nil {
		return false
	}
	_, frequency := component.Params["frequency"]
	_, gain := component.Params["gain"]
	_, q := component.Params["q"]
	return frequency && gain && q
}

func displayDomainFromPluginSkill(input *plugingrabber.PluginDisplayDomain) DisplayDomain {
	if input == nil {
		return DisplayDomain{}
	}
	out := DisplayDomain{Text: input.Text, Unit: input.Unit, Scale: input.Scale}
	if input.Min != nil {
		value := *input.Min
		out.Min = &value
	}
	if input.Max != nil {
		value := *input.Max
		out.Max = &value
	}
	return out
}

func evidenceRefs(evidence []plugingrabber.PluginEvidence) []string {
	refs := make([]string, 0, len(evidence))
	for _, item := range evidence {
		if text := strings.TrimSpace(item.Kind); text != "" {
			refs = append(refs, "plugin_skill:"+text)
		}
	}
	return uniqueSorted(refs)
}

func uniqueCapabilities(input []SemanticCapability) []SemanticCapability {
	byID := map[string]SemanticCapability{}
	for _, capability := range input {
		if strings.TrimSpace(capability.ID) == "" {
			continue
		}
		current, exists := byID[capability.ID]
		if !exists {
			byID[capability.ID] = capability
			continue
		}
		current.Operations = uniqueSorted(append(current.Operations, capability.Operations...))
		current.Parameters = uniqueSorted(append(current.Parameters, capability.Parameters...))
		current.FilterTypes = uniqueSorted(append(current.FilterTypes, capability.FilterTypes...))
		current.EvidenceRefs = uniqueSorted(append(current.EvidenceRefs, capability.EvidenceRefs...))
		byID[capability.ID] = current
	}
	keys := make([]string, 0, len(byID))
	for id := range byID {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	out := make([]SemanticCapability, 0, len(keys))
	for _, id := range keys {
		out = append(out, byID[id])
	}
	return out
}

func profilesForCapabilities(capabilities []SemanticCapability) []CapabilityConformanceProfile {
	profiles := []CapabilityConformanceProfile{}
	for _, capability := range capabilities {
		if profile, ok := ProfileFor(capability.ID); ok {
			profiles = append(profiles, profile)
		}
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	unique := profiles[:0]
	for _, profile := range profiles {
		if len(unique) == 0 || unique[len(unique)-1].ID != profile.ID {
			unique = append(unique, profile)
		}
	}
	return unique
}

func rawObservation(kind, summary string, value any, now time.Time) RawObservation {
	data := rawJSON(value)
	return RawObservation{Kind: kind, Source: kind, Summary: summary, CapturedAt: now.UTC(), Data: data}
}

func rawJSON(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func migrationDocumentID(source string, identity PluginIdentity, legacyID string) string {
	parts := []string{source, NewDocumentID(identity), strings.TrimSpace(legacyID)}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "vps_migration_" + hex.EncodeToString(sum[:])[:20]
}

func shortFingerprint(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func bytesTrimSpace(value []byte) []byte {
	start, end := 0, len(value)
	for start < end && (value[start] == ' ' || value[start] == '\n' || value[start] == '\r' || value[start] == '\t') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\n' || value[end-1] == '\r' || value[end-1] == '\t') {
		end--
	}
	return value[start:end]
}
