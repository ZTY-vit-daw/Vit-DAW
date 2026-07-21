package chat

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vps"
)

const vpsEQV2ProviderIDPrefix = "vps.eq_v2:"

type vpsEQV2ResolvedProvider struct {
	CatalogEntry vps.ProviderCatalogEntry
	Document     vps.VPSDocument
	Credential   vps.ProviderCredential
	Definition   spal.EQV2ProviderDefinition
	Instance     spal.ProviderInstance
	Registry     *spal.Registry
	EvidenceRefs []string
	LiveDigest   pluginParameterDigest
	// HostFingerprint is Vit's current controllable projection guard. It is
	// distinct from the independent VST3 identity in the Credential.
	HostFingerprint vps.PluginFingerprint
}

type vpsEQV2ResolveRequest struct {
	TargetRef            string
	PluginID             string
	ProviderCredentialID string
	Instruction          spal.Instruction
}

func (s *Server) buildVPSEQV2Cut(state *kernel.VSPStateResult, provider vpsEQV2ResolvedProvider, req ChatRequest) (orchestration.ProjectCut, error) {
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
			"vps.vit_host_projection:" + provider.HostFingerprint.ParameterSurface,
			"vps.vit_host_display_projection:" + provider.HostFingerprint.DisplaySurface,
		},
		TargetFingerprints: []string{"track:" + provider.Instance.TrackID, "plugin:" + provider.Instance.TrackID + ":" + provider.Instance.PluginID, "vps.binding:" + provider.Instance.ID},
		ArtifactRefs:       artifacts,
		ContractVersions:   []string{"capability:" + spalEQV2CapabilityID, "spal:eq.v2", "vps:v3", "vps:" + vps.EqualizerCapabilityID},
	})
}

// matchVPSEQV2ProviderCandidates narrows observed candidates by target ref
// and plugin_id. Rack slot IDs (plugin_id) get reused/reassigned across a
// project's lifetime, and the calling model can keep citing a stale
// plugin_id across turns even after a fresher one was already reported back
// to it (observed live: a project reload moved Pro-Q 3 from slot "1013" to
// "1018", and the model kept citing "1013" for several turns afterward). If
// the requested plugin_id matches nothing within the targeted scope but that
// scope has exactly one plug-in loaded, that is unambiguously the instance
// the user means - fall back to it instead of reporting "not found".
func matchVPSEQV2ProviderCandidates(candidates []spalReferenceEQProviderCandidate, targetRef, pluginID string) []spalReferenceEQProviderCandidate {
	targetMatching := make([]spalReferenceEQProviderCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if targetRef != "" && !spalReferenceEQProviderTargetMatches(candidate, targetRef) {
			continue
		}
		targetMatching = append(targetMatching, candidate)
	}
	matching := make([]spalReferenceEQProviderCandidate, 0, len(targetMatching))
	for _, candidate := range targetMatching {
		if pluginID != "" && !strings.EqualFold(candidate.PluginID, pluginID) {
			continue
		}
		matching = append(matching, candidate)
	}
	if len(matching) == 0 && pluginID != "" && len(targetMatching) == 1 {
		matching = targetMatching
	}
	return matching
}

func (s *Server) resolveVPSEQV2Provider(ctx context.Context, state *kernel.VSPStateResult, request vpsEQV2ResolveRequest) (vpsEQV2ResolvedProvider, []spalReferenceEQProviderCandidate, error) {
	if s == nil || s.harness == nil {
		return vpsEQV2ResolvedProvider{}, nil, fmt.Errorf("VPS EQ v2 Provider resolution requires the command harness")
	}
	if err := request.Instruction.Validate(); err != nil {
		return vpsEQV2ResolvedProvider{}, nil, err
	}
	library, err := s.userVPSLibrary()
	if err != nil {
		return vpsEQV2ResolvedProvider{}, nil, err
	}
	catalog, err := library.Catalog()
	if err != nil {
		return vpsEQV2ResolvedProvider{}, nil, err
	}
	entries := vpsEQV2CatalogEntries(catalog, request.Instruction.SchemaID, request.ProviderCredentialID)
	if len(entries) == 0 {
		return vpsEQV2ResolvedProvider{}, nil, fmt.Errorf("no_verified_eq_v2_provider")
	}

	candidates := observedVPSProviderCandidates(state)
	matching := matchVPSEQV2ProviderCandidates(candidates, request.TargetRef, request.PluginID)
	if len(matching) == 0 {
		return vpsEQV2ResolvedProvider{}, candidates, fmt.Errorf("no_observed_eq_v2_provider_instance")
	}

	documents := map[string]vps.VPSDocument{}
	resolved := []vpsEQV2ResolvedProvider{}
	reasons := []string{}
	for _, candidate := range matching {
		fresh, readErr := s.readVPSV3ParameterSurface(ctx, map[string]any{"track_id": candidate.TrackID, "plugin_id": candidate.PluginID}, vpsRuntimeReadContext())
		if readErr != nil {
			reasons = append(reasons, candidate.PluginID+": "+readErr.Error())
			continue
		}
		digest := buildPluginParameterDigest(fresh)
		if digest.TrackID != "" && !strings.EqualFold(digest.TrackID, candidate.TrackID) {
			reasons = append(reasons, candidate.PluginID+": parameter readback track mismatch")
			continue
		}
		if digest.PluginID != "" && !strings.EqualFold(digest.PluginID, candidate.PluginID) {
			reasons = append(reasons, candidate.PluginID+": parameter readback plugin mismatch")
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
					return vpsEQV2ResolvedProvider{}, matching, err
				}
				if !found {
					reasons = append(reasons, entry.CredentialID+": Catalog VPS missing")
					continue
				}
				documents[entry.VPSID] = document
			}
			credential, found := vpsRuntimeCredential(document, entry.CredentialID, entry.CredentialRevision)
			if !found || !credential.Dispatchable() || credential.CapabilityID != vps.EqualizerCapabilityID {
				reasons = append(reasons, entry.CredentialID+": Credential is no longer dispatchable")
				continue
			}
			if !containsIgnoreCase(credential.Conformance.ConformedSchemas, request.Instruction.SchemaID) {
				continue
			}
			if !vpsRuntimeIdentityMatches(document.PluginIdentity, digest) {
				continue
			}
			if matched, reason, stale := vpsRuntimeCredentialMatchesCurrentVitHost(document, credential, currentFingerprint); !matched {
				if stale {
					if staleErr := s.markVPSCredentialStale(library, document, currentFingerprint); staleErr != nil {
						return vpsEQV2ResolvedProvider{}, matching, staleErr
					}
				}
				reasons = append(reasons, entry.CredentialID+": "+reason)
				continue
			}
			definition, definitionErr := vpsEQV2Definition(document, credential)
			if definitionErr != nil {
				reasons = append(reasons, entry.CredentialID+": "+definitionErr.Error())
				continue
			}
			if liveErr := validateVPSEQV2LiveSurface(digest, definition.Binding); liveErr != nil {
				reasons = append(reasons, entry.CredentialID+": "+liveErr.Error())
				continue
			}
			adapter, adapterErr := spal.NewVPSEQV2Adapter(definition)
			if adapterErr != nil {
				reasons = append(reasons, entry.CredentialID+": "+adapterErr.Error())
				continue
			}
			registry := spal.NewRegistry()
			if err := registry.Register(adapter); err != nil {
				return vpsEQV2ResolvedProvider{}, matching, err
			}
			instance := vpsEQV2ProjectInstance(definition, document, credential, candidate, spalReferenceEQProjectUUID(state))
			if _, bindErr := adapter.Bind(instance, request.Instruction); bindErr != nil {
				reasons = append(reasons, entry.CredentialID+": "+bindErr.Error())
				continue
			}
			resolved = append(resolved, vpsEQV2ResolvedProvider{CatalogEntry: entry, Document: document, Credential: credential, Definition: definition, Instance: instance, Registry: registry, EvidenceRefs: appendUniqueSPALRefs(appendUniqueSPALRefs(document.ConformanceEvidence, credential.EvidenceRefs...), credential.Conformance.EvidenceRefs...), LiveDigest: digest, HostFingerprint: currentFingerprint})
		}
	}
	if len(resolved) == 0 {
		cause := "no_usable_eq_v2_provider"
		if len(reasons) > 0 {
			cause += ": " + strings.Join(reasons, "; ")
		}
		return vpsEQV2ResolvedProvider{}, matching, fmt.Errorf("%s", cause)
	}
	sort.Slice(resolved, func(i, j int) bool {
		if resolved[i].Credential.ID == resolved[j].Credential.ID {
			return resolved[i].Instance.ID < resolved[j].Instance.ID
		}
		return resolved[i].Credential.ID < resolved[j].Credential.ID
	})
	if len(resolved) > 1 {
		return vpsEQV2ResolvedProvider{}, matching, fmt.Errorf("eq_v2_provider_selection_required")
	}
	return resolved[0], matching, nil
}

func (s *Server) resolveVPSEQV2ProviderForFrozen(ctx context.Context, instance spal.ProviderInstance, projectUUID string, instruction spal.Instruction) (vpsEQV2ResolvedProvider, error) {
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		if err != nil {
			return vpsEQV2ResolvedProvider{}, err
		}
		return vpsEQV2ResolvedProvider{}, fmt.Errorf("current project state is unavailable")
	}
	if current := spalReferenceEQProjectUUID(state); projectUUID == "" || current != projectUUID {
		return vpsEQV2ResolvedProvider{}, fmt.Errorf("frozen EQ v2 Provider belongs to a different project")
	}
	resolved, _, err := s.resolveVPSEQV2Provider(ctx, state, vpsEQV2ResolveRequest{TargetRef: instance.TargetRef, PluginID: instance.PluginID, ProviderCredentialID: instance.Metadata["vps_credential_id"], Instruction: instruction})
	if err != nil {
		return vpsEQV2ResolvedProvider{}, err
	}
	if resolved.Instance.ID != instance.ID || resolved.Instance.ProviderID != instance.ProviderID {
		return vpsEQV2ResolvedProvider{}, fmt.Errorf("frozen EQ v2 Provider no longer matches the live Credential binding")
	}
	return resolved, nil
}

func vpsEQV2CatalogEntries(catalog vps.ProviderCatalog, schemaID, credentialID string) []vps.ProviderCatalogEntry {
	out := []vps.ProviderCatalogEntry{}
	for _, entry := range catalog.Entries {
		if entry.CapabilityID != vps.EqualizerCapabilityID || !containsIgnoreCase(entry.SupportedSchemas, schemaID) {
			continue
		}
		if credentialID != "" && !strings.EqualFold(entry.CredentialID, credentialID) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func vpsEQV2Definition(document vps.VPSDocument, credential vps.ProviderCredential) (spal.EQV2ProviderDefinition, error) {
	if credential.Conformance.EQV2Binding == nil {
		return spal.EQV2ProviderDefinition{}, fmt.Errorf("Credential has no persisted EQ v2 binding")
	}
	schemas := append([]string(nil), credential.Conformance.ConformedSchemas...)
	return spal.EQV2ProviderDefinition{
		Descriptor: spal.ProviderDescriptor{ID: vpsEQV2ProviderID(credential.ID), AdapterVersion: "v2", PluginName: document.PluginIdentity.Name, PluginFormat: document.PluginIdentity.Format, ProfileSignature: document.PluginIdentity.Fingerprint.ParameterSurface, Status: spal.ProviderVerified, SupportedSchemas: schemas, ConformanceEvidence: appendUniqueSPALRefs(appendUniqueSPALRefs(document.ConformanceEvidence, credential.EvidenceRefs...), credential.Conformance.EvidenceRefs...)},
		VPSID:      document.ID, CredentialID: credential.ID, Binding: *credential.Conformance.EQV2Binding,
	}, nil
}

func validateVPSEQV2LiveSurface(digest pluginParameterDigest, binding spal.EQV2Binding) error {
	parameters := vpsParameterIndex(digest)
	ids := map[string]string{}
	add := func(role, id string) {
		if strings.TrimSpace(id) != "" {
			ids[id] = role
		}
	}
	for ref, band := range binding.Bands {
		add(ref+".enabled", band.Enabled.ParameterID)
		add(ref+".response_shape", band.ResponseShape.ParameterID)
		add(ref+".frequency_hz", band.FrequencyHz.ParameterID)
		add(ref+".gain_db", band.GainDB.ParameterID)
		add(ref+".q", band.Q.ParameterID)
		if band.Dynamic != nil {
			add(ref+".dynamics_mode", band.Dynamic.Mode.ParameterID)
			add(ref+".routing_scope", band.Dynamic.Routing.ParameterID)
			add(ref+".threshold_db", band.Dynamic.ThresholdDB.ParameterID)
			if band.Dynamic.Ratio != nil {
				add(ref+".ratio", band.Dynamic.Ratio.ParameterID)
			}
			if band.Dynamic.AttackMS != nil {
				add(ref+".attack_ms", band.Dynamic.AttackMS.ParameterID)
			}
			if band.Dynamic.ReleaseMS != nil {
				add(ref+".release_ms", band.Dynamic.ReleaseMS.ParameterID)
			}
		}
	}
	for prefix, filter := range map[string]*spal.EQV2PassFilterBinding{"hp": binding.HighPass, "lp": binding.LowPass} {
		if filter != nil {
			add(prefix+".enabled", filter.Enabled.ParameterID)
			add(prefix+".frequency", filter.CutoffFrequencyHz.ParameterID)
			add(prefix+".slope", filter.SlopeDBPerOctave.ParameterID)
		}
	}
	if binding.Output != nil {
		if binding.Output.Bypass != nil {
			add("output.bypass", binding.Output.Bypass.ParameterID)
		}
		if binding.Output.DryMix != nil {
			add("output.dry_mix", binding.Output.DryMix.ParameterID)
		}
		if binding.Output.OutputGainDB != nil {
			add("output.gain", binding.Output.OutputGainDB.ParameterID)
		}
	}
	for id, role := range ids {
		parameter, ok := parameters[id]
		if !ok || !parameter.HostControllable {
			return fmt.Errorf("current plugin surface no longer exposes conformed %s mapping", role)
		}
	}
	return nil
}

func vpsEQV2ProjectInstance(definition spal.EQV2ProviderDefinition, document vps.VPSDocument, credential vps.ProviderCredential, candidate spalReferenceEQProviderCandidate, projectUUID string) spal.ProviderInstance {
	return spal.ProviderInstance{ID: "vps_eq_v2:" + credential.ID + ":" + candidate.TrackID + ":" + candidate.PluginID, ProviderID: definition.Descriptor.ID, TargetRef: candidate.TargetRef, TrackID: candidate.TrackID, PluginID: candidate.PluginID, PluginSignature: document.PluginIdentity.Fingerprint.ParameterSurface, Status: spal.InstanceVerified, Metadata: map[string]string{"project_uuid": projectUUID, "vps_id": document.ID, "vps_credential_id": credential.ID, "vps_credential_revision": strconv.Itoa(credential.Revision), "eq_v2_ready": "true"}}
}

func vpsEQV2ProviderID(credentialID string) string {
	return vpsEQV2ProviderIDPrefix + strings.TrimSpace(credentialID)
}
func containsIgnoreCase(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}
