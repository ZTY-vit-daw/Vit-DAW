package chat

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vps"
)

const vpsStaticEQProviderIDPrefix = "vps.static_eq:"

// vpsStaticEQResolvedProvider is the project-scoped result of resolving one
// reusable, verified VPS Credential against one currently loaded plug-in
// instance. Nothing in this value is persisted back into the user-level VPS
// Library as a project binding.
type vpsStaticEQResolvedProvider struct {
	CatalogEntry vps.ProviderCatalogEntry
	Document     vps.VPSDocument
	Credential   vps.ProviderCredential
	Definition   spal.StaticEQProviderDefinition
	Instance     spal.ProviderInstance
	Registry     *spal.Registry
	EvidenceRefs []string
}

func (s *Server) buildVPSStaticEQCut(state *kernel.VSPStateResult, provider vpsStaticEQResolvedProvider, req ChatRequest) (orchestration.ProjectCut, error) {
	artifacts := append([]string(nil), req.ArtifactRefs...)
	artifacts = append(artifacts, canaryStringSlice(req.Context["artifact_refs"])...)
	return projectcut.Build(projectcut.BuildRequest{
		State: state, Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: []string{
			"vsp.snapshot:" + state.SnapshotHash,
			"vps.document:" + provider.Document.ID + ":" + strconv.Itoa(provider.Document.Revision),
			"vps.credential:" + provider.Credential.ID + ":" + strconv.Itoa(provider.Credential.Revision),
			"vps.parameter_surface:" + provider.Credential.PluginFingerprint.ParameterSurface,
			"vps.display_surface:" + provider.Credential.PluginFingerprint.DisplaySurface,
		},
		TargetFingerprints: []string{
			"track:" + provider.Instance.TrackID,
			"plugin:" + provider.Instance.TrackID + ":" + provider.Instance.PluginID,
			"vps.binding:" + provider.Instance.ID,
		},
		ArtifactRefs: artifacts,
		ContractVersions: []string{
			"capability:" + spalReferenceEQTestCapabilityID,
			"spal:" + spal.SchemaVersion,
			"vps:v3",
			"vps:spectral.static_eq.v0",
		},
	})
}

// resolveVPSStaticEQProvider is the VPS v3 product bridge for the existing
// Reference EQ task. It resolves only a Credential that is visible in the
// derived Catalog, then binds it to a live project instance after fresh
// fingerprint and static-Bell checks. It intentionally has no v0 Provider
// Record fallback.
func (s *Server) resolveVPSStaticEQProvider(ctx context.Context, state *kernel.VSPStateResult, request spalReferenceEQRequest) (vpsStaticEQResolvedProvider, []spalReferenceEQProviderCandidate, error) {
	if s == nil || s.harness == nil {
		return vpsStaticEQResolvedProvider{}, nil, fmt.Errorf("VPS v3 Provider resolution requires the command harness")
	}
	library, err := s.userVPSLibrary()
	if err != nil {
		return vpsStaticEQResolvedProvider{}, nil, err
	}
	catalog, err := library.Catalog()
	if err != nil {
		return vpsStaticEQResolvedProvider{}, nil, err
	}
	entries := vpsStaticEQCatalogEntries(catalog, request.ProviderCredentialID)
	if len(entries) == 0 {
		return vpsStaticEQResolvedProvider{}, nil, fmt.Errorf("no_verified_vps_provider")
	}

	candidates := observedSPALReferenceEQProviderCandidates(state)
	matchingCandidates := make([]spalReferenceEQProviderCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if request.PluginID != "" && !strings.EqualFold(candidate.PluginID, request.PluginID) {
			continue
		}
		if request.TargetRef != "" && !spalReferenceEQProviderTargetMatches(candidate, request.TargetRef) {
			continue
		}
		matchingCandidates = append(matchingCandidates, candidate)
	}
	if len(matchingCandidates) == 0 {
		return vpsStaticEQResolvedProvider{}, candidates, fmt.Errorf("no_observed_vps_provider_instance")
	}

	documents := make(map[string]vps.VPSDocument, len(entries))
	resolved := make([]vpsStaticEQResolvedProvider, 0, len(matchingCandidates))
	reasons := []string{}
	for _, candidate := range matchingCandidates {
		fresh, readErr := s.readVPSV3ParameterSurface(ctx, map[string]any{
			"track_id":  candidate.TrackID,
			"plugin_id": candidate.PluginID,
		}, vpsRuntimeReadContext())
		if readErr != nil {
			reasons = append(reasons, candidate.PluginID+": "+readErr.Error())
			continue
		}
		digest := buildPluginParameterDigest(fresh)
		if digest.TrackID != "" && !strings.EqualFold(digest.TrackID, candidate.TrackID) {
			reasons = append(reasons, candidate.PluginID+": parameter readback track does not match selected instance")
			continue
		}
		if digest.PluginID != "" && !strings.EqualFold(digest.PluginID, candidate.PluginID) {
			reasons = append(reasons, candidate.PluginID+": parameter readback plugin does not match selected instance")
			continue
		}

		currentFingerprint, fingerprintErr := vpsRuntimeFingerprint(digest)
		if fingerprintErr != nil {
			reasons = append(reasons, candidate.PluginID+": "+fingerprintErr.Error())
			continue
		}
		for _, entry := range entries {
			document, ok := documents[entry.VPSID]
			if !ok {
				var found bool
				document, found, err = library.Get(entry.VPSID)
				if err != nil {
					return vpsStaticEQResolvedProvider{}, matchingCandidates, err
				}
				if !found {
					reasons = append(reasons, entry.CredentialID+": Catalog VPS is no longer available")
					continue
				}
				documents[entry.VPSID] = document
			}
			credential, found := vpsRuntimeCredential(document, entry.CredentialID, entry.CredentialRevision)
			if !found || !credential.Dispatchable() || credential.CapabilityID != vps.StaticEQCapabilityID {
				reasons = append(reasons, entry.CredentialID+": Catalog Credential is no longer dispatchable")
				continue
			}
			if !vpsRuntimeIdentityMatches(document.PluginIdentity, digest) {
				continue
			}
			if matched, reason, stale := vpsRuntimeCredentialMatchesCurrentVitHost(document, credential, currentFingerprint); !matched {
				if stale {
					if staleErr := s.markVPSCredentialStale(library, document, currentFingerprint); staleErr != nil {
						return vpsStaticEQResolvedProvider{}, matchingCandidates, staleErr
					}
				}
				reasons = append(reasons, entry.CredentialID+": "+reason)
				continue
			}
			definition, definitionErr := vpsStaticEQDefinitionWithObservedDisplaySurface(document, credential, digest)
			if definitionErr != nil {
				reasons = append(reasons, entry.CredentialID+": "+definitionErr.Error())
				continue
			}
			if liveErr := validateVPSStaticEQLiveSurface(digest, credential.Conformance.StaticEQBinding); liveErr != nil {
				reasons = append(reasons, entry.CredentialID+": "+liveErr.Error())
				continue
			}
			adapter, adapterErr := spal.NewVPSStaticEQAdapter(definition)
			if adapterErr != nil {
				reasons = append(reasons, entry.CredentialID+": "+adapterErr.Error())
				continue
			}
			registry := spal.NewRegistry()
			if err := registry.Register(adapter); err != nil {
				return vpsStaticEQResolvedProvider{}, matchingCandidates, err
			}
			instance := vpsStaticEQProjectInstance(definition, document, credential, candidate, spalReferenceEQProjectUUID(state))
			resolved = append(resolved, vpsStaticEQResolvedProvider{
				CatalogEntry: entry,
				Document:     document,
				Credential:   credential,
				Definition:   definition,
				Instance:     instance,
				Registry:     registry,
				EvidenceRefs: appendUniqueSPALRefs(appendUniqueSPALRefs(document.ConformanceEvidence, credential.EvidenceRefs...), credential.Conformance.EvidenceRefs...),
			})
		}
	}
	if len(resolved) == 0 {
		cause := "no_usable_vps_provider"
		if len(reasons) > 0 {
			cause += ": " + strings.Join(reasons, "; ")
		}
		return vpsStaticEQResolvedProvider{}, matchingCandidates, fmt.Errorf("%s", cause)
	}
	sort.Slice(resolved, func(i, j int) bool {
		if resolved[i].Credential.ID == resolved[j].Credential.ID {
			return resolved[i].Instance.ID < resolved[j].Instance.ID
		}
		return resolved[i].Credential.ID < resolved[j].Credential.ID
	})
	if len(resolved) > 1 {
		return vpsStaticEQResolvedProvider{}, matchingCandidates, fmt.Errorf("vps_provider_selection_required")
	}
	return resolved[0], matchingCandidates, nil
}

func (s *Server) resolveVPSStaticEQProviderForFrozen(ctx context.Context, instance spal.ProviderInstance, projectUUID string) (vpsStaticEQResolvedProvider, error) {
	if s == nil || s.kernel == nil {
		return vpsStaticEQResolvedProvider{}, fmt.Errorf("Project Kernel is unavailable")
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		if err != nil {
			return vpsStaticEQResolvedProvider{}, err
		}
		return vpsStaticEQResolvedProvider{}, fmt.Errorf("current project state is unavailable")
	}
	if currentProjectUUID := spalReferenceEQProjectUUID(state); projectUUID == "" || currentProjectUUID != projectUUID {
		return vpsStaticEQResolvedProvider{}, fmt.Errorf("frozen VPS Provider belongs to a different project")
	}
	resolved, _, err := s.resolveVPSStaticEQProvider(ctx, state, spalReferenceEQRequest{
		TargetRef:            instance.TargetRef,
		PluginID:             instance.PluginID,
		ProviderCredentialID: strings.TrimSpace(instance.Metadata["vps_credential_id"]),
	})
	if err != nil {
		return vpsStaticEQResolvedProvider{}, err
	}
	if resolved.Instance.ID != instance.ID || resolved.Instance.ProviderID != instance.ProviderID {
		return vpsStaticEQResolvedProvider{}, fmt.Errorf("frozen VPS Provider Instance no longer matches the live Credential binding")
	}
	return resolved, nil
}

func vpsStaticEQCatalogEntries(catalog vps.ProviderCatalog, requestedCredentialID string) []vps.ProviderCatalogEntry {
	entries := make([]vps.ProviderCatalogEntry, 0, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		if entry.CapabilityID != vps.StaticEQCapabilityID || !vpsStaticEQOperationsContain(entry.Operations, vps.OperationBellCut) {
			continue
		}
		if requestedCredentialID != "" && !strings.EqualFold(entry.CredentialID, requestedCredentialID) {
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CredentialID == entries[j].CredentialID {
			return entries[i].VPSID < entries[j].VPSID
		}
		return entries[i].CredentialID < entries[j].CredentialID
	})
	return entries
}

func vpsStaticEQOperationsContain(operations []string, want string) bool {
	for _, operation := range operations {
		if strings.EqualFold(strings.TrimSpace(operation), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func vpsRuntimeReadContext() map[string]any {
	return map[string]any{
		"user_message":               "VPS v3 dispatch validation",
		"vps_v3_dispatch_validation": true,
	}
}

func vpsRuntimeFingerprint(digest pluginParameterDigest) (vps.PluginFingerprint, error) {
	installationFingerprint, err := vpsRuntimeInstallationFingerprint(digest)
	if err != nil {
		return vps.PluginFingerprint{}, fmt.Errorf("current plugin installation fingerprint is unavailable: %w", err)
	}
	fingerprint, err := vps.BuildPluginFingerprintFromDigest(installationFingerprint, digest)
	if err != nil {
		return vps.PluginFingerprint{}, fmt.Errorf("current plugin surface cannot prove VPS compatibility: %w", err)
	}
	return fingerprint, nil
}

// vpsRuntimeInstallationFingerprint first uses the same raw-file grammar as
// the independent Windows VST3 adapter.  The host only supplies an observed
// path, not an Adapter hash, so comparing it with the older package-hash
// grammar would falsely stale a Credential for the exact same single-file
// VST3. Directory bundles retain the package fingerprint fallback.
func vpsRuntimeInstallationFingerprint(digest pluginParameterDigest) (string, error) {
	identity := digest.PluginIdentity
	for _, key := range []string{"installation_fingerprint", "plugin_installation_fingerprint"} {
		if value := strings.TrimSpace(fmt.Sprint(identity[key])); value != "" && value != "<nil>" && vpsSHA256Fingerprint(value) {
			return value, nil
		}
	}
	path := firstNonEmptyText(identity, "plugin_path", "path")
	if path == "" {
		return "", fmt.Errorf("plugin installation path is required")
	}
	if fingerprint, fileErr := vps.BuildFileFingerprint(path); fileErr == nil {
		return fingerprint, nil
	}
	return vps.BuildInstallationFingerprint(path)
}

// vpsRuntimeCredentialMatchesCurrentVitHost distinguishes an independent
// plug-in identity invalidation from a Vit-only projection change. A
// projection mismatch blocks dispatch but cannot stale a Credential, because
// it says nothing about the independently observed VST3 installation.
func vpsRuntimeCredentialMatchesCurrentVitHost(document vps.VPSDocument, credential vps.ProviderCredential, current vps.PluginFingerprint) (matched bool, reason string, stale bool) {
	if !current.Complete() {
		return false, "current_host_fingerprint_unavailable", false
	}
	if strings.TrimSpace(credential.PluginFingerprint.Installation) == "" ||
		!strings.EqualFold(strings.TrimSpace(credential.PluginFingerprint.Installation), strings.TrimSpace(current.Installation)) {
		return false, "provider_installation_fingerprint_stale", true
	}
	if binding, found := document.HostProjectionBindingFor(vps.VitHostProjectionID); found {
		if !binding.Fingerprint.Equal(current) {
			return false, "vit_host_projection_binding_mismatch", false
		}
		return true, "", false
	}
	if credential.PluginFingerprint.Equal(current) {
		return true, "", false
	}
	// Existing VPS documents predate host-projection bindings. Preserve their
	// strict full-surface invalidation behavior.
	return false, "provider_fingerprint_stale", true
}

func vpsRuntimeCredential(document vps.VPSDocument, credentialID string, revision int) (vps.ProviderCredential, bool) {
	for _, credential := range document.ProviderCredentials {
		if credential.ID == credentialID && credential.Revision == revision {
			return credential, true
		}
	}
	return vps.ProviderCredential{}, false
}

func vpsRuntimeIdentityMatches(identity vps.PluginIdentity, digest pluginParameterDigest) bool {
	if identity.ProfileKey != "" {
		current := firstNonEmptyText(digest.PluginIdentity, "profile_key")
		if current != "" {
			return strings.EqualFold(strings.TrimSpace(identity.ProfileKey), current)
		}
	}
	name := firstNonEmptyText(digest.PluginIdentity, "plugin_name", "name")
	format := firstNonEmptyText(digest.PluginIdentity, "plugin_format", "format")
	if name == "" {
		name = digest.PluginName
	}
	if name == "" || identity.Name == "" || !strings.EqualFold(strings.TrimSpace(identity.Name), name) {
		return false
	}
	return identity.Format == "" || format == "" || strings.EqualFold(strings.TrimSpace(identity.Format), format)
}

func (s *Server) markVPSCredentialStale(library *vps.Library, document vps.VPSDocument, current vps.PluginFingerprint) error {
	changes, err := document.ReconcileCredentialFingerprints(current, time.Now().UTC())
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	if _, err := library.Upsert(document); err != nil {
		return fmt.Errorf("persist stale VPS Credential: %w", err)
	}
	return nil
}

func vpsStaticEQDefinition(document vps.VPSDocument, credential vps.ProviderCredential) (spal.StaticEQProviderDefinition, error) {
	binding := credential.Conformance.StaticEQBinding
	if binding == nil {
		return spal.StaticEQProviderDefinition{}, fmt.Errorf("Credential predates the persisted static-EQ binding; rerun Plugin Learning")
	}
	if strings.TrimSpace(binding.ComponentID) == "" || !vpsStaticEQHasResource(document, binding.ComponentID) {
		return spal.StaticEQProviderDefinition{}, fmt.Errorf("Credential static-EQ component is not a conformed VPS resource")
	}
	mappings := make(map[string]vps.ControlSurfaceMapping, len(document.ControlSurface.Mappings))
	for _, mapping := range document.ControlSurface.Mappings {
		if mapping.ComponentID == binding.ComponentID && mapping.Confirmed && mapping.ParameterID != "" {
			mappings[mapping.ParameterID] = mapping
		}
	}
	filter, ok := mappings[binding.FilterTypeParameterID]
	if !ok {
		return spal.StaticEQProviderDefinition{}, fmt.Errorf("Credential filter-type mapping is no longer confirmed")
	}
	_ = filter
	definition := spal.StaticEQProviderDefinition{
		Descriptor: spal.ProviderDescriptor{
			ID:                  vpsStaticEQProviderID(credential.ID),
			AdapterVersion:      "v3",
			PluginName:          document.PluginIdentity.Name,
			PluginFormat:        document.PluginIdentity.Format,
			ProfileSignature:    document.PluginIdentity.Fingerprint.ParameterSurface,
			Status:              spal.ProviderVerified,
			SupportedSchemas:    []string{spal.StaticBellControlID},
			ConformanceEvidence: appendUniqueSPALRefs(appendUniqueSPALRefs(document.ConformanceEvidence, credential.EvidenceRefs...), credential.Conformance.EvidenceRefs...),
		},
		VPSID:                 document.ID,
		CredentialID:          credential.ID,
		ComponentID:           binding.ComponentID,
		FilterTypeParameterID: binding.FilterTypeParameterID,
		ParameterBindings:     map[string]spal.ParameterBinding{},
	}
	for _, item := range []struct {
		semantic  string
		parameter string
	}{
		{"center_frequency_hz", binding.FrequencyParameterID},
		{"gain_db", binding.GainParameterID},
		{"q", binding.QParameterID},
		{"enabled", binding.EnabledParameterID},
	} {
		mapping, ok := mappings[item.parameter]
		if !ok {
			return spal.StaticEQProviderDefinition{}, fmt.Errorf("Credential %s mapping is no longer confirmed", item.semantic)
		}
		parameter, err := vpsStaticEQParameterBinding(item.semantic, mapping)
		if err != nil {
			return spal.StaticEQProviderDefinition{}, err
		}
		definition.ParameterBindings[item.semantic] = parameter
	}
	return definition, nil
}

// vpsStaticEQDefinitionWithObservedDisplaySurface keeps a Credential's
// persisted mapping authoritative, while allowing the fresh host display
// probe to correct a demonstrably wrong transport scale for this invocation.
// It never writes to the VPS Library or changes a Credential revision: a
// current instance must expose the same semantic range and an explicit
// linear/log scale before the runtime definition is adjusted.
func vpsStaticEQDefinitionWithObservedDisplaySurface(document vps.VPSDocument, credential vps.ProviderCredential, digest pluginParameterDigest) (spal.StaticEQProviderDefinition, error) {
	definition, err := vpsStaticEQDefinition(document, credential)
	if err != nil {
		return spal.StaticEQProviderDefinition{}, err
	}
	binding := credential.Conformance.StaticEQBinding
	if binding == nil {
		return definition, nil
	}
	parameters := vpsParameterIndex(digest)
	for _, item := range []struct {
		semantic  string
		parameter string
	}{
		{"center_frequency_hz", binding.FrequencyParameterID},
		{"gain_db", binding.GainParameterID},
		{"q", binding.QParameterID},
		{"enabled", binding.EnabledParameterID},
	} {
		parameter, found := parameters[item.parameter]
		if !found {
			continue
		}
		persisted, found := definition.ParameterBindings[item.semantic]
		if !found {
			continue
		}
		if scale, ok := vpsStaticEQObservedDisplayScale(item.semantic, persisted, parameter); ok {
			persisted.Scale = scale
			definition.ParameterBindings[item.semantic] = persisted
		}
	}
	return definition, nil
}

func vpsStaticEQObservedDisplayScale(semantic string, persisted spal.ParameterBinding, parameter pluginParameterInfo) (string, bool) {
	if !parameter.HostControllable || parameter.DisplayDomainCandidate == nil {
		return "", false
	}
	domain := parameter.DisplayDomainCandidate
	if domain.Min == nil || domain.Max == nil || !vpsStaticEQDisplayRangesMatch(persisted.Min, persisted.Max, *domain.Min, *domain.Max) {
		return "", false
	}
	if unit := strings.TrimSpace(domain.Unit); unit != "" {
		if !vpsStaticEQUnitMatches(semantic, unit) {
			return "", false
		}
	} else if semantic != "q" {
		// Q is the one conformed semantic whose host display probe is known to
		// omit its unit.  Its Credential-bound parameter ID and matching range
		// keep this exception tied to the already verified control.
		return "", false
	}
	switch strings.ToLower(strings.TrimSpace(domain.Scale)) {
	case "linear":
		return "linear", true
	case "log", "logarithmic":
		return "log", true
	default:
		return "", false
	}
}

func vpsStaticEQDisplayRangesMatch(persistedMin, persistedMax, observedMin, observedMax float64) bool {
	return vpsStaticEQDisplayBoundMatches(persistedMin, observedMin) && vpsStaticEQDisplayBoundMatches(persistedMax, observedMax)
}

func vpsStaticEQDisplayBoundMatches(left, right float64) bool {
	if math.IsNaN(left) || math.IsInf(left, 0) || math.IsNaN(right) || math.IsInf(right, 0) {
		return false
	}
	tolerance := math.Max(1e-9, math.Max(math.Abs(left), math.Abs(right))*1e-6)
	return math.Abs(left-right) <= tolerance
}

func vpsStaticEQHasResource(document vps.VPSDocument, componentID string) bool {
	for _, resource := range document.Topology.Resources {
		if resource.ComponentID == componentID && resource.Kind == "eq_band" {
			return true
		}
	}
	return false
}

func vpsStaticEQParameterBinding(semantic string, mapping vps.ControlSurfaceMapping) (spal.ParameterBinding, error) {
	if mapping.DisplayDomain.Min == nil || mapping.DisplayDomain.Max == nil {
		return spal.ParameterBinding{}, fmt.Errorf("Credential %s mapping has no confirmed display range", semantic)
	}
	unit := strings.TrimSpace(mapping.DisplayDomain.Unit)
	if !vpsStaticEQUnitMatches(semantic, unit) {
		return spal.ParameterBinding{}, fmt.Errorf("Credential %s mapping has incompatible display unit %q", semantic, unit)
	}
	return spal.ParameterBinding{
		ParameterID: mapping.ParameterID,
		Unit:        unit,
		Min:         *mapping.DisplayDomain.Min,
		Max:         *mapping.DisplayDomain.Max,
		Scale:       strings.TrimSpace(mapping.DisplayDomain.Scale),
	}, nil
}

func vpsStaticEQUnitMatches(semantic, unit string) bool {
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch semantic {
	case "center_frequency_hz":
		return unit == "hz" || unit == "hertz"
	case "gain_db":
		return unit == "db" || unit == "decibel" || unit == "decibels"
	case "q":
		return unit == "q"
	case "enabled":
		return unit == "toggle" || unit == "bool" || unit == "boolean"
	default:
		return false
	}
}

func validateVPSStaticEQLiveSurface(digest pluginParameterDigest, binding *vps.StaticEQBinding) error {
	if binding == nil {
		return fmt.Errorf("Credential has no persisted static-EQ binding")
	}
	parameters := vpsParameterIndex(digest)
	for _, item := range []struct {
		semantic string
		id       string
	}{
		{"filter_type", binding.FilterTypeParameterID},
		{"frequency_hz", binding.FrequencyParameterID},
		{"gain_db", binding.GainParameterID},
		{"q", binding.QParameterID},
		{"enabled", binding.EnabledParameterID},
	} {
		parameter, ok := parameters[item.id]
		if !ok || !parameter.HostControllable {
			return fmt.Errorf("current plugin surface no longer exposes conformed %s mapping", item.semantic)
		}
		if item.semantic == "filter_type" && !strings.Contains(strings.ToLower(parameter.ValueText), "bell") {
			return fmt.Errorf("current conformed filter type is not Bell")
		}
	}
	return nil
}

func vpsStaticEQProviderID(credentialID string) string {
	return vpsStaticEQProviderIDPrefix + strings.TrimSpace(credentialID)
}

func vpsStaticEQProjectInstance(definition spal.StaticEQProviderDefinition, document vps.VPSDocument, credential vps.ProviderCredential, candidate spalReferenceEQProviderCandidate, projectUUID string) spal.ProviderInstance {
	instanceID := "vps_static_eq:" + credential.ID + ":" + candidate.TrackID + ":" + candidate.PluginID
	return spal.ProviderInstance{
		ID:              instanceID,
		ProviderID:      definition.Descriptor.ID,
		TargetRef:       candidate.TargetRef,
		TrackID:         candidate.TrackID,
		PluginID:        candidate.PluginID,
		PluginSignature: document.PluginIdentity.Fingerprint.ParameterSurface,
		Status:          spal.InstanceVerified,
		Metadata: map[string]string{
			"project_uuid":            projectUUID,
			"vps_id":                  document.ID,
			"vps_credential_id":       credential.ID,
			"vps_credential_revision": strconv.Itoa(credential.Revision),
			"vps_component_id":        definition.ComponentID,
			"static_bell_ready":       "true",
		},
	}
}

func vpsSPALProviderResponse(conversationID string, goal agentruntime.Goal, targetRef string, cause error, candidates []spalReferenceEQProviderCandidate) ChatResponse {
	stage := "no_verified_vps_provider"
	reply := "当前项目没有能与已验证 VPS v3 Credential 绑定的 Static EQ Provider。Vit 不会回退到旧 SPAL v0 ProviderRecord，也不会猜测插件参数；请先对当前已加载实例完成 Plugin Learning。"
	blockers := []string{"verified_vps_provider_required", "plugin_learning_required"}
	pluginLearningRequired := true
	if cause != nil {
		switch {
		case strings.Contains(cause.Error(), "vps_provider_selection_required"):
			stage = "vps_provider_selection_required"
			reply = "当前目标存在多个符合 VPS v3 Credential 的已加载实例。请在 Godot 中选择具体插件，或在请求中提供 provider_credential_id 后重试。"
		case strings.Contains(cause.Error(), "provider_fingerprint_stale"):
			stage = "vps_credential_stale"
			reply = "当前插件的安装包、参数面或显示面已与已签发的 VPS Credential 不一致。该 Credential 已停止派遣；请重新学习并完成 conformance。"
		case strings.Contains(cause.Error(), "static-Bell") || strings.Contains(cause.Error(), "not Bell"):
			stage = "vps_static_bell_not_ready"
			reply = "当前实例没有保持已验证的静态 Bell 状态。请先恢复为 Bell 后重试当前请求；Credential 仍保留，系统不会自行写入滤波器类型。"
			blockers = []string{"static_bell_state_required"}
			pluginLearningRequired = false
		}
	}
	available := make([]map[string]string, 0, len(candidates))
	for _, candidate := range candidates {
		available = append(available, map[string]string{
			"track_id":    candidate.TrackID,
			"plugin_id":   candidate.PluginID,
			"plugin_name": candidate.PluginName,
			"target_ref":  candidate.TargetRef,
		})
	}
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    reply,
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification),
		WorkflowData: map[string]any{
			"capability_id":                spalReferenceEQTestCapabilityID,
			"canary_stage":                 stage,
			"target_ref":                   targetRef,
			"provider_source":              "vps_v3_catalog",
			"blockers":                     blockers,
			"available_observed_instances": available,
			"cause":                        errorText(cause),
			"plugin_learning_required":     pluginLearningRequired,
		},
	}
}
