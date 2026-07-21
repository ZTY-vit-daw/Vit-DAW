package vps

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const pluginLearningV3Source = "plugin_learning_v3"

// LearningInput is the fresh, user-authorized Plugin Learning observation
// used to update a VPS. It deliberately contains no project binding in the
// resulting document: TrackID and PluginID are used only by the caller while
// acquiring this observation and are stripped from persisted evidence.
type LearningInput struct {
	Skill                   plugingrabber.PluginSkillDocument
	Digest                  plugingrabber.ParameterDigest
	InstallationFingerprint string
	EvidenceRefs            []string
	ObservedAt              time.Time
}

// LearningBuildResult makes non-dispatchable gaps explicit. A missing
// installation/display fingerprint or an incomplete static-EQ mapping still
// produces a useful VPS record, but never a usable Provider Credential.
type LearningBuildResult struct {
	Document              VPSDocument
	Warnings              []string
	StaticEQBinding       *StaticEQBinding
	CandidateCredentialID string
	StateChanges          []CredentialStateChange
}

// StaticEQBinding is the limited implementation-facing mapping needed by the
// first VPS v3 conformance profile. It is intentionally not a SPAL semantic
// instruction and contains no project instance or lease information.
type StaticEQBinding struct {
	ComponentID           string `json:"component_id"`
	FilterTypeParameterID string `json:"filter_type_parameter_id"`
	FrequencyParameterID  string `json:"frequency_parameter_id"`
	GainParameterID       string `json:"gain_parameter_id"`
	QParameterID          string `json:"q_parameter_id"`
	EnabledParameterID    string `json:"enabled_parameter_id"`
}

// Validate checks that a persisted static-EQ conformance binding still names
// one unambiguous, complete raw control surface. Display ranges and scales are
// retained by the corresponding VPS ControlSurface mappings.
func (b StaticEQBinding) Validate() error {
	if strings.TrimSpace(b.ComponentID) == "" {
		return fmt.Errorf("static EQ binding requires a component_id")
	}
	ids := []string{
		b.FilterTypeParameterID,
		b.FrequencyParameterID,
		b.GainParameterID,
		b.QParameterID,
		b.EnabledParameterID,
	}
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return fmt.Errorf("static EQ binding requires five distinct parameter ids")
		}
		seen[id] = true
	}
	return nil
}

// ConformanceResult is the durable, capability-scoped result of a controlled
// conformance transaction. Details are archived as raw VPS evidence while the
// compact fields are what the Credential validation model relies on.
type ConformanceResult struct {
	CapabilityID string   `json:"capability_id"`
	Operations   []string `json:"operations,omitempty"`
	Parameters   []string `json:"parameters,omitempty"`
	FilterTypes  []string `json:"filter_types,omitempty"`
	// StaticEQBinding is the exact, confirmed control surface that the bounded
	// spectral.static_eq.v0 conformance exercised. It is retained with the
	// Credential so a later runtime resolver never has to guess which learned
	// EQ band or raw parameter IDs were qualified.
	StaticEQBinding     *StaticEQBinding `json:"static_eq_binding,omitempty"`
	WriteReadbackPassed bool             `json:"write_readback_passed"`
	BoundaryTestsPassed bool             `json:"boundary_tests_passed"`
	RollbackTestPassed  bool             `json:"rollback_test_passed"`
	EvidenceRefs        []string         `json:"evidence_refs,omitempty"`
	CompletedAt         time.Time        `json:"completed_at"`
	Details             json.RawMessage  `json:"details,omitempty"`
}

// BuildLearningVPS creates a current VPS v3 record from a freshly saved
// Plugin Skill. Passing the prior record preserves its audit history and
// invalidates former verified Credentials before the new conformance result
// can issue a replacement.
func BuildLearningVPS(input LearningInput, prior *VPSDocument) (LearningBuildResult, error) {
	at := input.ObservedAt.UTC()
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if input.Skill.SchemaVersion != plugingrabber.PluginSkillSchemaVersion {
		return LearningBuildResult{}, fmt.Errorf("Plugin Skill schema_version must be %d", plugingrabber.PluginSkillSchemaVersion)
	}
	identity := learningIdentity(input.Skill, input.Digest)
	if err := identity.validDraft(); err != nil {
		return LearningBuildResult{}, fmt.Errorf("learned plugin identity: %w", err)
	}
	identity.Fingerprint.LegacyParameterSignature = strings.TrimSpace(input.Skill.Identity.ParamSignatureHash)
	warnings := []string{}
	if fingerprint := strings.TrimSpace(input.InstallationFingerprint); fingerprint == "" {
		warnings = append(warnings, "Installation package fingerprint is unavailable; Credential issuance is blocked.")
	} else if current, err := BuildPluginFingerprintFromDigest(fingerprint, input.Digest); err != nil {
		warnings = append(warnings, "Fresh VPS fingerprint is incomplete: "+err.Error())
	} else {
		current.LegacyParameterSignature = identity.Fingerprint.LegacyParameterSignature
		identity.Fingerprint = current
	}

	var document VPSDocument
	stateChanges := []CredentialStateChange{}
	if prior != nil {
		document = cloneVPS(*prior)
		if err := document.Validate(); err != nil {
			return LearningBuildResult{}, fmt.Errorf("existing VPS %s: %w", document.ID, err)
		}
		if identity.Fingerprint.Complete() {
			changes, err := document.ReconcileCredentialFingerprints(identity.Fingerprint, at)
			if err != nil {
				return LearningBuildResult{}, err
			}
			stateChanges = append(stateChanges, changes...)
		}
		// A newly saved Skill can change the semantic-to-parameter mapping even
		// when the host parameter fingerprint is unchanged. It therefore always
		// requires a fresh conformance transaction before dispatch can continue.
		stateChanges = append(stateChanges, invalidateVerifiedCredentials(&document, "Plugin Learning mapping was updated and requires fresh conformance.", at)...)
	} else {
		document = NewDraft(identity, at)
	}

	document.PluginIdentity = identity
	document.ControlSurface, document.Topology = controlSurfaceAndTopologyFromSkill(sanitizedLearningSkill(input.Skill))
	document.RawObservations = append(document.RawObservations, learningRawObservation(input, identity, at))
	document.SafetyAndRollback = SafetyAndRollback{
		Preconditions: []string{
			"A fresh host parameter readback must match the learned control surface.",
			"Conformance writes are allowed only inside the user-authorized Plugin Learning transaction.",
		},
		RollbackMode: "restore_previous_state",
		Invalidators: []string{
			"plugin installation fingerprint changed",
			"plugin parameter-surface signature changed",
			"plugin display-surface signature changed",
			"learned control mapping changed",
		},
	}

	learningRef := "plugin_learning_v3:" + shortFingerprint(map[string]any{
		"identity": identity,
		"skill":    sanitizedLearningSkill(input.Skill),
		"observed": at.UTC().Format(time.RFC3339Nano),
	})
	document.ConformanceEvidence = uniqueSorted(append(document.ConformanceEvidence, append([]string{learningRef}, input.EvidenceRefs...)...))

	// Candidate Credentials are never dispatchable. Replacing an earlier
	// candidate is safe, while stale/revoked/pending Credentials remain part of
	// the audit record.
	document.ProviderCredentials = filterReplaceableCandidates(document.ProviderCredentials, StaticEQCapabilityID)
	document.SemanticCapabilities = removeCapability(document.SemanticCapabilities, StaticEQCapabilityID)
	document.CapabilityProfiles = removeProfile(document.CapabilityProfiles, StaticEQCapabilityID)

	result := LearningBuildResult{Document: document, Warnings: warnings, StateChanges: stateChanges}
	binding, bindingErr := StaticEQBindingForSkill(input.Skill)
	if bindingErr == nil {
		profile := StaticEQConformanceProfileV0()
		document.SemanticCapabilities = append(document.SemanticCapabilities, SemanticCapability{
			ID:           profile.ID,
			Operations:   append([]string(nil), profile.RequiredOperations...),
			Parameters:   append([]string(nil), profile.RequiredParameters...),
			FilterTypes:  append([]string(nil), profile.RequiredFilterTypes...),
			Status:       string(CredentialCandidate),
			EvidenceRefs: []string{learningRef},
		})
		document.CapabilityProfiles = append(document.CapabilityProfiles, profile)
		candidate := ProviderCredential{
			ID:                nextCredentialID(document.ProviderCredentials, learningCredentialID(document.ID, profile.ID, at)),
			Revision:          1,
			CapabilityID:      profile.ID,
			Status:            CredentialCandidate,
			PluginFingerprint: identity.Fingerprint,
			Safety:            document.SafetyAndRollback,
			EvidenceRefs:      []string{learningRef},
			IssuedAt:          at,
			UpdatedAt:         at,
		}
		document.ProviderCredentials = append(document.ProviderCredentials, candidate)
		result.StaticEQBinding = &binding
		result.CandidateCredentialID = candidate.ID
	} else {
		warnings = append(warnings, "spectral.static_eq.v0 remains a draft: "+bindingErr.Error())
		result.Warnings = warnings
	}

	sort.Slice(document.SemanticCapabilities, func(i, j int) bool { return document.SemanticCapabilities[i].ID < document.SemanticCapabilities[j].ID })
	sort.Slice(document.CapabilityProfiles, func(i, j int) bool { return document.CapabilityProfiles[i].ID < document.CapabilityProfiles[j].ID })
	sort.Slice(document.ProviderCredentials, func(i, j int) bool { return document.ProviderCredentials[i].ID < document.ProviderCredentials[j].ID })
	if document.hasDispatchableCredential() {
		document.Status = VPSStatusVerified
	} else {
		document.Status = VPSStatusMapped
	}
	document.UpdatedAt = at
	if err := document.Validate(); err != nil {
		return LearningBuildResult{}, err
	}
	result.Document = document
	return result, nil
}

// StaticEQBindingForSkill requires a fully confirmed mapping for the finite
// v3 Static EQ profile. The filter type is read as a Bell invariant during
// conformance; it is never guessed or rewritten merely to make a Credential
// issue succeed.
func StaticEQBindingForSkill(skill plugingrabber.PluginSkillDocument) (StaticEQBinding, error) {
	if skill.SchemaVersion != plugingrabber.PluginSkillSchemaVersion {
		return StaticEQBinding{}, fmt.Errorf("Plugin Skill schema_version must be %d", plugingrabber.PluginSkillSchemaVersion)
	}
	for _, component := range skill.Components {
		if !learningComponentLooksLikeEQBand(component) {
			continue
		}
		keys := map[string]string{
			"filter_type": "type",
			"frequency":   "frequency",
			"gain":        "gain",
			"q":           "q",
			"enabled":     "enable",
		}
		ids := map[string]string{}
		valid := true
		for semantic, slot := range keys {
			mapping, ok := component.Params[slot]
			if !ok || strings.TrimSpace(mapping.ParamID) == "" || !mapping.Confirmed {
				valid = false
				break
			}
			if semantic == "filter_type" {
				// The host's current readback is the authoritative Bell
				// invariant checked during conformance. A user-confirmed enum
				// label is enough to select that mapping here; unlike a numeric
				// control, its raw display text need not encode a range.
				if !learningFilterTypeMappingHasDisplayEvidence(mapping) {
					valid = false
					break
				}
			} else if !learningMappingHasBoundedDomain(mapping) {
				valid = false
				break
			}
			ids[semantic] = strings.TrimSpace(mapping.ParamID)
		}
		if !valid || !uniqueBindingParameterIDs(ids) {
			continue
		}
		return StaticEQBinding{
			ComponentID:           strings.TrimSpace(component.ID),
			FilterTypeParameterID: ids["filter_type"],
			FrequencyParameterID:  ids["frequency"],
			GainParameterID:       ids["gain"],
			QParameterID:          ids["q"],
			EnabledParameterID:    ids["enabled"],
		}, nil
	}
	return StaticEQBinding{}, fmt.Errorf("no confirmed EQ band exposes type, enable, frequency, gain and Q mappings with bounded display domains")
}

// IssueCredential promotes only the fresh Candidate produced by Plugin
// Learning (or creates an equivalent new revision) after all profile-required
// conformance evidence has passed. There is intentionally no API that turns a
// draft, stale or imported Credential directly into a verified one.
func (d *VPSDocument) IssueCredential(result ConformanceResult, now time.Time) (ProviderCredential, error) {
	if d == nil {
		return ProviderCredential{}, fmt.Errorf("VPS document is required")
	}
	profile, ok := ProfileFor(result.CapabilityID)
	if !ok {
		return ProviderCredential{}, fmt.Errorf("unknown capability profile %q", result.CapabilityID)
	}
	if err := d.PluginIdentity.validForCredential(); err != nil {
		return ProviderCredential{}, err
	}
	conformance := result.credentialConformance()
	if err := conformance.validates(profile); err != nil {
		return ProviderCredential{}, err
	}
	at := now.UTC()
	if at.IsZero() {
		at = conformance.CompletedAt.UTC()
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if conformance.CompletedAt.IsZero() {
		conformance.CompletedAt = at
	}

	credentialIndex := -1
	for index := range d.ProviderCredentials {
		credential := d.ProviderCredentials[index]
		if credential.CapabilityID == profile.ID && credential.Status == CredentialCandidate && credential.PluginFingerprint.Equal(d.PluginIdentity.Fingerprint) {
			credentialIndex = index
			break
		}
	}
	credential := ProviderCredential{
		ID:                nextCredentialID(d.ProviderCredentials, learningCredentialID(d.ID, profile.ID, at)),
		Revision:          1,
		CapabilityID:      profile.ID,
		Status:            CredentialVerified,
		PluginFingerprint: d.PluginIdentity.Fingerprint,
		Conformance:       conformance,
		Safety:            d.SafetyAndRollback,
		EvidenceRefs:      uniqueSorted(result.EvidenceRefs),
		IssuedAt:          at,
		UpdatedAt:         at,
	}
	if credentialIndex >= 0 {
		previous := d.ProviderCredentials[credentialIndex]
		credential.ID = previous.ID
		credential.Revision = previous.Revision
		credential.IssuedAt = previous.IssuedAt
		if credential.IssuedAt.IsZero() {
			credential.IssuedAt = at
		}
		d.ProviderCredentials[credentialIndex] = credential
	} else {
		d.ProviderCredentials = append(d.ProviderCredentials, credential)
	}
	d.ensureCredentialCapabilities([]ProviderCredential{credential})
	for index := range d.SemanticCapabilities {
		if d.SemanticCapabilities[index].ID != profile.ID {
			continue
		}
		d.SemanticCapabilities[index].Status = string(CredentialVerified)
		d.SemanticCapabilities[index].EvidenceRefs = uniqueSorted(append(d.SemanticCapabilities[index].EvidenceRefs, append(credential.EvidenceRefs, conformance.EvidenceRefs...)...))
	}
	if !hasProfile(d.CapabilityProfiles, profile.ID) {
		d.CapabilityProfiles = append(d.CapabilityProfiles, profile)
	}
	d.ConformanceEvidence = uniqueSorted(append(d.ConformanceEvidence, append(credential.EvidenceRefs, conformance.EvidenceRefs...)...))
	d.RawObservations = append(d.RawObservations, rawObservation("vps_v3_conformance", "Controlled VPS v3 conformance passed and issued a Credential.", result, at))
	d.UpdatedAt = at
	if d.Migration.RequiresRequalification {
		if err := d.Requalify(d.PluginIdentity, d.ProviderCredentials, at); err != nil {
			return ProviderCredential{}, err
		}
	} else {
		d.Status = VPSStatusVerified
		if err := d.Validate(); err != nil {
			return ProviderCredential{}, err
		}
	}
	return credential, nil
}

// RecordConformanceAttempt keeps failed or incomplete controlled experiments in
// the VPS evidence trail without allowing them to affect catalog eligibility.
func (d *VPSDocument) RecordConformanceAttempt(result ConformanceResult, status, failure string, now time.Time) {
	if d == nil {
		return
	}
	at := now.UTC()
	if at.IsZero() {
		at = time.Now().UTC()
	}
	payload := map[string]any{
		"status":  strings.TrimSpace(status),
		"failure": strings.TrimSpace(failure),
		"result":  result,
	}
	d.RawObservations = append(d.RawObservations, rawObservation("vps_v3_conformance", "Controlled VPS v3 conformance attempt.", payload, at))
	d.UpdatedAt = at
}

func (r ConformanceResult) credentialConformance() CredentialConformance {
	var binding *StaticEQBinding
	if r.StaticEQBinding != nil {
		copy := *r.StaticEQBinding
		binding = &copy
	}
	return CredentialConformance{
		ProfileID:           strings.TrimSpace(r.CapabilityID),
		ProfileVersion:      StaticEQProfileVersion,
		Operations:          uniqueSorted(r.Operations),
		Parameters:          uniqueSorted(r.Parameters),
		FilterTypes:         uniqueSorted(r.FilterTypes),
		StaticEQBinding:     binding,
		WriteReadbackPassed: r.WriteReadbackPassed,
		BoundaryTestsPassed: r.BoundaryTestsPassed,
		RollbackTestPassed:  r.RollbackTestPassed,
		EvidenceRefs:        uniqueSorted(r.EvidenceRefs),
		CompletedAt:         r.CompletedAt.UTC(),
	}
}

func learningIdentity(skill plugingrabber.PluginSkillDocument, digest plugingrabber.ParameterDigest) PluginIdentity {
	raw := digest.PluginIdentity
	lookup := func(keys ...string) string {
		for _, key := range keys {
			if raw == nil {
				continue
			}
			if value, ok := raw[key]; ok {
				if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
					return text
				}
			}
		}
		return ""
	}
	return PluginIdentity{
		Manufacturer: firstNonEmpty(lookup("manufacturer", "maker", "vendor"), skill.Identity.Manufacturer),
		Name:         firstNonEmpty(lookup("plugin_name", "name"), skill.Identity.Name, digest.PluginName),
		Format:       firstNonEmpty(lookup("plugin_format", "format"), skill.Identity.Format),
		Version:      firstNonEmpty(lookup("version"), skill.Identity.Version),
		ProfileKey:   firstNonEmpty(lookup("profile_key"), skill.Identity.ProfileKey),
		InstallPath:  firstNonEmpty(lookup("plugin_path", "path"), skill.Identity.Path),
	}
}

func learningRawObservation(input LearningInput, identity PluginIdentity, at time.Time) RawObservation {
	skill := sanitizedLearningSkill(input.Skill)
	digest := input.Digest
	digest.TrackID = ""
	digest.PluginID = ""
	digest.GlobalProfile = nil
	digest.PluginSkill = nil
	digest.PluginIdentity = map[string]any{
		"manufacturer":  identity.Manufacturer,
		"plugin_name":   identity.Name,
		"plugin_format": identity.Format,
		"version":       identity.Version,
		"profile_key":   identity.ProfileKey,
		"plugin_path":   identity.InstallPath,
	}
	return rawObservation(pluginLearningV3Source, "Fresh Plugin Learning observation for VPS v3.", map[string]any{
		"plugin_identity":  identity,
		"plugin_skill":     skill,
		"parameter_digest": digest,
	}, at)
}

func sanitizedLearningSkill(skill plugingrabber.PluginSkillDocument) plugingrabber.PluginSkillDocument {
	skill.Identity.PluginID = ""
	return skill
}

func learningComponentLooksLikeEQBand(component plugingrabber.PluginSkillComponent) bool {
	text := strings.ToLower(strings.Join([]string{component.ID, component.Role, component.Label}, " "))
	if strings.Contains(text, "eq_band") || (strings.Contains(text, "eq") && strings.Contains(text, "band")) {
		return true
	}
	id := strings.TrimSpace(strings.ToLower(component.ID))
	return len(id) >= 2 && id[0] == 'b' && id[1] >= '0' && id[1] <= '9'
}

func learningMappingHasBoundedDomain(mapping plugingrabber.PluginSkillParamMap) bool {
	return mapping.DisplayDomain != nil && mapping.DisplayDomain.Min != nil && mapping.DisplayDomain.Max != nil && *mapping.DisplayDomain.Max >= *mapping.DisplayDomain.Min
}

func learningFilterTypeMappingHasDisplayEvidence(mapping plugingrabber.PluginSkillParamMap) bool {
	if mapping.DisplayDomain == nil {
		return false
	}
	return strings.TrimSpace(mapping.DisplayDomain.Text) != ""
}

func uniqueBindingParameterIDs(ids map[string]string) bool {
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func invalidateVerifiedCredentials(document *VPSDocument, reason string, at time.Time) []CredentialStateChange {
	if document == nil {
		return nil
	}
	changes := []CredentialStateChange{}
	for index := range document.ProviderCredentials {
		credential := &document.ProviderCredentials[index]
		if credential.Status != CredentialVerified {
			continue
		}
		previous := credential.Status
		credential.Status = CredentialStale
		credential.InvalidatedAt = &at
		credential.InvalidationReason = strings.TrimSpace(reason)
		credential.UpdatedAt = at
		changes = append(changes, CredentialStateChange{CredentialID: credential.ID, From: previous, To: credential.Status, Reason: credential.InvalidationReason})
	}
	if len(changes) > 0 {
		document.Status = VPSStatusStale
		document.UpdatedAt = at
	}
	return changes
}

func filterReplaceableCandidates(credentials []ProviderCredential, capabilityID string) []ProviderCredential {
	out := make([]ProviderCredential, 0, len(credentials))
	for _, credential := range credentials {
		if credential.CapabilityID == capabilityID && credential.Status == CredentialCandidate {
			continue
		}
		out = append(out, credential)
	}
	return out
}

func removeCapability(capabilities []SemanticCapability, capabilityID string) []SemanticCapability {
	out := make([]SemanticCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		if capability.ID != capabilityID {
			out = append(out, capability)
		}
	}
	return out
}

func removeProfile(profiles []CapabilityConformanceProfile, capabilityID string) []CapabilityConformanceProfile {
	out := make([]CapabilityConformanceProfile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.ID != capabilityID {
			out = append(out, profile)
		}
	}
	return out
}

func hasProfile(profiles []CapabilityConformanceProfile, capabilityID string) bool {
	for _, profile := range profiles {
		if profile.ID == capabilityID {
			return true
		}
	}
	return false
}

func learningCredentialID(vpsID, capabilityID string, at time.Time) string {
	return "credential_" + shortFingerprint(map[string]any{
		"vps_id":        strings.TrimSpace(vpsID),
		"capability_id": strings.TrimSpace(capabilityID),
		"observed_at":   at.UTC().Format(time.RFC3339Nano),
	})
}

func nextCredentialID(credentials []ProviderCredential, base string) string {
	used := map[string]bool{}
	for _, credential := range credentials {
		used[credential.ID] = true
	}
	if !used[base] {
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s_%d", base, suffix)
		if !used[candidate] {
			return candidate
		}
	}
}
