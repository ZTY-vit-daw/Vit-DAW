package chat

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/projectcut"
)

// d1StaticEQWhitelistBinding is the machine-local plugin resolution for one
// real-plugin static_eq execution: the whitelisted plugin plus the band the
// admission's frequency_hz maps onto. Machine-specific paths never enter
// agent code; they travel exclusively through this binding, which originates
// from ~/.vit/free_state_experiment_plugins.json.
type d1StaticEQWhitelistBinding struct {
	Plugin      experimentplugins.StaticEQPlugin
	Band        experimentplugins.Band
	FrequencyHz float64
}

// d1PluginParamWhitelistBinding is the normalized single-parameter-pair write
// resolution shared by every PluginBound domain: which whitelisted plugin file
// to instantiate and which ch1/ch2 parameter ids to write under one
// idempotency key. static_eq resolutions are mapped into this shape after the
// legacy band resolver ran; broadband_compression resolves directly.
type d1PluginParamWhitelistBinding struct {
	Section     string  // whitelist section label, e.g. "static_eq"
	PluginName  string  `json:"-"`
	PluginPath  string  `json:"-"`
	ParamID     string  // ch1 parameter id (fingerprint + journal identity)
	ParamIDCH2  string  // paired channel written in the same batch
	FrequencyHz float64 // static_eq provenance only; 0 for other domains
}

// resolveD1PluginParamWhitelistBinding turns any admitted PluginBound typed
// action into the normalized whitelist binding. Error classes stay the same
// distinguishable boundaries per section: not configured, corrupt whitelist,
// pinned identifier outside the whitelist, unreadable attestation store, and
// a PCA admission refusal. An eligibility ruling here gates execution only;
// it never changes what the domain validators enforce.
func resolveD1PluginParamWhitelistBinding(typedAction map[string]any) (*d1PluginParamWhitelistBinding, error) {
	if domainLabel(typedAction) == d1BroadbandCompressionDomain {
		return resolveD1BroadbandCompressionWhitelistBinding(typedAction)
	}
	binding, err := resolveD1StaticEQWhitelistBinding(typedAction)
	if err != nil {
		return nil, err
	}
	return &d1PluginParamWhitelistBinding{
		Section:     "static_eq",
		PluginName:  binding.Plugin.PluginName,
		PluginPath:  binding.Plugin.PluginPath,
		ParamID:     binding.Band.GainParamIDCH1,
		ParamIDCH2:  binding.Band.GainParamIDCH2,
		FrequencyHz: binding.FrequencyHz,
	}, nil
}

// resolveD1BroadbandCompressionWhitelistBinding mirrors the static_eq gate for
// the compression whitelist section with its own label so upstream can tell
// the domains apart while keeping the class wording aligned.
func resolveD1BroadbandCompressionWhitelistBinding(typedAction map[string]any) (*d1PluginParamWhitelistBinding, error) {
	const label = d1BroadbandCompressionDomain
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) || errors.Is(err, experimentplugins.ErrCompressionNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrCompressionNotConfigured)
		}
		return nil, fmt.Errorf("%s experiment whitelist is invalid: %w", label, err)
	}
	compression := whitelist.BroadbandCompression
	if compression == nil {
		return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrCompressionNotConfigured)
	}
	pinned := firstStringFromMap(typedAction, "plugin_identifier")
	if pinned != "" && !strings.EqualFold(pinned, compression.PluginIdentifier) {
		return nil, fmt.Errorf("%s admission pinned plugin_identifier %q but the experiment plugin whitelist admits %q", label, pinned, compression.PluginIdentifier)
	}
	library, err := d1StaticEQAttestationReader()
	if err != nil {
		return nil, fmt.Errorf("%s experiment could not evaluate its PCA admission: %w", label, err)
	}
	if err := whitelist.ValidateCompressionAdmission(library); err != nil {
		return nil, fmt.Errorf("%s experiment was refused by the PCA admission check: %w", label, err)
	}
	return &d1PluginParamWhitelistBinding{
		Section:    label,
		PluginName: compression.PluginName,
		PluginPath: compression.PluginPath,
		ParamID:    compression.ThresholdParamIDCH1,
		ParamIDCH2: compression.ThresholdParamIDCH2,
	}, nil
}

// domainLabel reads the canonical action_domain of an admitted typed action.
func domainLabel(typedAction map[string]any) string {
	return strings.ToLower(strings.TrimSpace(firstNonEmpty(firstStringFromMap(typedAction, "action_domain", "domain"), "")))
}

// d1StaticEQWhitelistLoader loads the experiment plugin whitelist. Package
// variables so tests inject t.TempDir fixtures instead of the developer
// machine's real ~/.vit state.
var d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
	path, err := experimentplugins.DefaultPath()
	if err != nil {
		return experimentplugins.Whitelist{}, err
	}
	return experimentplugins.Load(path)
}

// d1StaticEQAttestationReader reads the v1 processor-control attestation
// library backing the PCA admission predicate. A missing store decodes to an
// empty library, which makes every admission query answer not-promoted.
var d1StaticEQAttestationReader = func() (processorattestation.Library, error) {
	store, err := processorattestation.NewStore("")
	if err != nil {
		return processorattestation.Library{}, err
	}
	library, report, err := store.Read()
	if err != nil {
		return processorattestation.Library{}, fmt.Errorf("processor attestation store is unreadable at %s: %w", report.Path, err)
	}
	return library, nil
}

// resolveD1StaticEQWhitelistBinding turns one admitted static_eq typed action
// into the whitelisted band binding. The returned errors carry deliberately
// distinct boundaries so upstream can tell apart: not configured, corrupt
// whitelist, not PCA-promoted, attestation store unreadable, and an admission
// that pins an identifier outside the whitelist. An eligibility ruling here
// gates execution only; it never changes what the domain validators enforce.
func resolveD1StaticEQWhitelistBinding(typedAction map[string]any) (*d1StaticEQWhitelistBinding, error) {
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) {
			return nil, fmt.Errorf("static_eq experiment is not configured: %w", err)
		}
		return nil, fmt.Errorf("static_eq experiment whitelist is invalid: %w", err)
	}
	if whitelist.StaticEQ == nil {
		return nil, fmt.Errorf("static_eq experiment is not configured: %w", experimentplugins.ErrNotConfigured)
	}
	pinned := firstStringFromMap(typedAction, "plugin_identifier")
	if pinned != "" && !strings.EqualFold(pinned, whitelist.StaticEQ.PluginIdentifier) {
		return nil, fmt.Errorf("static_eq admission pinned plugin_identifier %q but the experiment plugin whitelist admits %q", pinned, whitelist.StaticEQ.PluginIdentifier)
	}
	frequency, ok := treatmentNumber(typedAction, "frequency_hz")
	if !ok || frequency < 20 || frequency > 20000 {
		return nil, fmt.Errorf("static_eq execution requires frequency_hz within 20-20000 Hz in the admitted typed action")
	}
	library, err := d1StaticEQAttestationReader()
	if err != nil {
		return nil, fmt.Errorf("static_eq experiment could not evaluate its PCA admission: %w", err)
	}
	if err := whitelist.ValidateStaticEQAdmission(library); err != nil {
		return nil, fmt.Errorf("static_eq experiment was refused by the PCA admission check: %w", err)
	}
	band, err := whitelist.NearestStaticEQBand(frequency)
	if err != nil {
		return nil, fmt.Errorf("static_eq experiment whitelist has no usable band: %w", err)
	}
	return &d1StaticEQWhitelistBinding{Plugin: *whitelist.StaticEQ, Band: band, FrequencyHz: frequency}, nil
}

// d1SubstituteFingerprint fills a domain fingerprint template's markers. A
// marker whose value is empty stays visible, which keeps malformed templates
// loud instead of silently producing ambiguous provenance strings.
func d1SubstituteFingerprint(template, trackID, dbValue, paramValue string) string {
	replacer := strings.NewReplacer("{track}", trackID, "{db}", dbValue, "{param}", paramValue)
	return replacer.Replace(template)
}

// d1AssembleFrozenPlan finishes the shared scaffolding of both bounded D1
// plans: hashed action set and proposal under the domain capability,
// revision-bound observation walk, and the frozen envelope. All per-domain
// variation arrives through the arguments, sourced from the domain table.
func d1AssembleFrozenPlan(loop freeStateReasoningLoop, spec experiment.D1S1DomainSpec, action orchestration.Action, targetID string, cut orchestration.ProjectCut) (orchestration.FrozenPlan, error) {
	experimentID := sanitizeCanaryID(loop.Experiment.ID)
	set := orchestration.ActionSet{ID: "d1_set_" + experimentID, CapabilityID: spec.CapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	set.Hash = set.ComputeHash()
	round, _ := loop.Experiment.CurrentRound()
	previousObservationID := ""
	for _, observation := range round.Observations {
		if !observation.PostAction {
			previousObservationID = observation.ID
		}
	}
	proposal := orchestration.Proposal{ID: "d1_proposal_" + experimentID, Revision: 1, CapabilityID: spec.CapabilityID,
		CapabilityVer: "v0", ProjectCutHash: cut.Hash, ActionSetHash: set.Hash, TargetScope: []string{targetID}, Risk: "reversible", VerificationRef: "fresh_revision_bound_ccb", Summary: loop.Experiment.Admission.Hypothesis, CreatedAt: time.Now().UTC()}
	return orchestration.FrozenPlan{Proposal: proposal, ActionSet: set, ProjectCut: cut, ContextBundleID: "d1_context_" + experimentID, PreviousObservationID: previousObservationID, FrozenAt: time.Now().UTC()}, nil
}

// d1ObservationViewIDsFor binds the post-action CCB observation views from
// the domain table; an unknown domain keeps the historical default view.
func d1ObservationViewIDsFor(admission experiment.Admission) []string {
	if spec, ok := experiment.D1S1DomainSpecFor(admission); ok && len(spec.ObservationViewIDs) > 0 {
		return append([]string(nil), spec.ObservationViewIDs...)
	}
	return []string{"mix.multitrack_relationship"}
}

// d1JournalRecordForAction builds the running audit record for one D1
// mutation from the domain table. Identity keys (everything except the raw
// numeric payload keys "value"/"db") are coerced through the string reader so
// a deleted arg cannot masquerade as an id, matching the historical journal
// shape byte-for-byte for both admitted domains.
func d1JournalRecordForAction(staticEQ bool, actionID, goalID, runID, targetRef string, args map[string]any) journal.Action {
	domainName, kindName := experiment.D1S1ActionDomain, experiment.D1S1ActionKind
	if staticEQ {
		domainName, kindName = d1StaticEQDomain, d1StaticEQKind
	}
	spec, _ := experiment.D1S1SpecForAction(domainName, kindName)
	return d1JournalRecordForSpec(spec, actionID, goalID, runID, targetRef, args)
}

// d1JournalRecordForSpec builds the same running audit record from any
// resolved domain spec. It is the shape authority behind
// d1JournalRecordForAction, which keeps its historical bool signature.
func d1JournalRecordForSpec(spec experiment.D1S1DomainSpec, actionID, goalID, runID, targetRef string, args map[string]any) journal.Action {
	command := map[string]any{"cmd": spec.Journal.CommandLabel, "track_id": targetRef}
	for _, field := range spec.Journal.Fields {
		switch field.CommandKey {
		case "value", "db":
			command[field.CommandKey] = args[field.Arg]
		default:
			keys := []string{field.Arg}
			if field.ArgFallback != "" {
				keys = append(keys, field.ArgFallback)
			}
			command[field.CommandKey] = firstNonEmpty(firstStringFromMap(args, keys...))
		}
	}
	return journal.Action{
		AgentActionID: actionID, GoalID: goalID, RunID: runID, Domain: "daw", Source: "free_state_d1_s1",
		Summary: spec.Journal.Summary, Tool: spec.Journal.Tool, CommandName: spec.Journal.Tool,
		Command:   command,
		RiskLevel: "confirm", RequiresConfirmation: true, ConfirmationStatus: "confirmed", Status: journal.StatusRunning,
	}
}

// d1StaticEQActionArgs assembles the write payload for one admitted band
// adjustment. Without a whitelist binding it falls back to the historical
// stub schema (juce_eq identifier plus band_%d_gain), preserved verbatim so
// the table-driven builder reproduces past plans; with a binding it targets
// the real plugin through normalized_batch_v1.
func d1StaticEQActionArgs(typedAction map[string]any, gainDB float64, binding *d1StaticEQWhitelistBinding) (map[string]any, string) {
	if binding == nil {
		bandIndex := 0
		if value, present := typedAction["band_index"]; present {
			if parsed, ok := treatmentNumber(map[string]any{"band_index": value}, "band_index"); ok {
				bandIndex = int(parsed)
			}
		}
		paramID := fmt.Sprintf("band_%d_gain", bandIndex)
		pluginIdentifier := firstStringFromMap(typedAction, "plugin_identifier")
		if pluginIdentifier == "" {
			pluginIdentifier = defaultD1StaticEQPluginIdentifier
		}
		return map[string]any{"plugin_identifier": pluginIdentifier, "param_id": paramID, "target_value": gainDB}, paramID
	}
	paramID := binding.Band.GainParamIDCH1
	return map[string]any{
		"write_mode":   executionports.WriteModeNormalizedBatchV1,
		"plugin_path":  binding.Plugin.PluginPath,
		"plugin_name":  binding.Plugin.PluginName,
		"param_id":     paramID,
		"param_id_ch2": binding.Band.GainParamIDCH2,
		"target_value": gainDB,
		"frequency_hz": binding.FrequencyHz,
	}, paramID
}

// d1PluginParamPlanWithBinding is the table-driven plan builder shared by
// every PluginBound domain with a resolved whitelist binding: one admitted
// action, one forward mutation, revision-bound cut, fingerprint templates and
// contract versions sourced from the domain table. Plugin parameter before
// values are read by the port's Preflight; the plan layer never invokes
// kernel commands. Legacy static_eq shapes stay locked by
// TestD1S1TableDrivenBuildersReproduceLegacyPlans through the untouched
// stub path.
func d1PluginParamPlanWithBinding(loop freeStateReasoningLoop, candidate agentloop.PendingMixTickCandidate, stateRevision int64, projectUUID, projectEpoch, snapshotHash string, state map[string]any, writeBinding *d1PluginParamWhitelistBinding) (orchestration.FrozenPlan, error) {
	if loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return orchestration.FrozenPlan{}, fmt.Errorf("active D1-S1 experiment is required")
	}
	if err := loop.Experiment.Admission.ValidateD1S1(); err != nil {
		return orchestration.FrozenPlan{}, err
	}
	spec, ok := experiment.D1S1DomainSpecFor(loop.Experiment.Admission)
	if !ok || !spec.WriteBinding.PluginBound {
		return orchestration.FrozenPlan{}, fmt.Errorf("D1-S1 plugin parameter plan requires a plugin-bound domain admission")
	}
	if candidate.Operation != spec.ActionKind || strings.TrimSpace(candidate.TrackID) == "" {
		return orchestration.FrozenPlan{}, fmt.Errorf("D1-S1 requires one bounded %s", spec.ActionKind)
	}
	valueDB, ok := treatmentNumber(loop.Experiment.Admission.TypedAction, spec.AdmissionValueKey)
	if !ok || valueDB == 0 || math.Abs(valueDB) > 2 {
		return orchestration.FrozenPlan{}, fmt.Errorf("D1-S1 requires one bounded %s move within +/-2 dB", spec.AdmissionValueKey)
	}
	targetID := firstStringFromMap(loop.Experiment.Admission.TargetRef, "id", "track_id")
	if candidate.TrackID != targetID {
		return orchestration.FrozenPlan{}, fmt.Errorf("D1-S1 candidate target does not match admitted observation target")
	}
	if writeBinding == nil && spec.WriteBinding.StubParamIDFormat == "" {
		return orchestration.FrozenPlan{}, fmt.Errorf("%s execution requires a resolved experiment plugin whitelist binding", spec.ActionDomain)
	}
	args, paramID := pluginParamWriteArgs(loop.Experiment.Admission.TypedAction, spec, valueDB, writeBinding)
	if stateRevision <= 0 || projectUUID == "" || projectEpoch == "" || snapshotHash == "" {
		return orchestration.FrozenPlan{}, fmt.Errorf("D1-S1 requires a revision-bound VSP snapshot")
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{
		State: d1StateResult(projectUUID, projectEpoch, snapshotHash, stateRevision, state), Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: append([]string(nil), loop.Experiment.Admission.EvidenceRefs...),
		TargetFingerprints:     []string{d1SubstituteFingerprint(spec.TargetFingerprintTemplate, targetID, "", paramID)},
		ContractVersions:       append([]string(nil), spec.ContractVersions...),
	})
	if err != nil {
		return orchestration.FrozenPlan{}, err
	}
	action := orchestration.Action{ID: "d1_" + sanitizeCanaryID(loop.Experiment.ID) + spec.ActionIDSuffix, Command: spec.ActionKind, TargetRef: targetID,
		BeforeFingerprint: d1SubstituteFingerprint(spec.BeforeFingerprintTemplate, targetID, "", paramID),
		Args:              args, Compensatable: true, IdempotencyClass: "effectively_once"}
	return d1AssembleFrozenPlan(loop, spec, action, targetID, cut)
}

// pluginParamWriteArgs assembles the write payload for any PluginBound domain.
// With a whitelist binding it targets the real plugin through
// normalized_batch_v1; without one it falls back to the historical stub schema
// for domains that carry a StubParamIDFormat (only static_eq keeps one), so
// table-driven builders can reproduce past plans byte-for-byte. Domains
// without a stub form require a binding and fail loudly otherwise.
func pluginParamWriteArgs(typedAction map[string]any, spec experiment.D1S1DomainSpec, valueDB float64, writeBinding *d1PluginParamWhitelistBinding) (map[string]any, string) {
	if writeBinding == nil {
		bandIndex := 0
		if value, present := typedAction["band_index"]; present {
			if parsed, ok := treatmentNumber(map[string]any{"band_index": value}, "band_index"); ok {
				bandIndex = int(parsed)
			}
		}
		paramID := fmt.Sprintf(spec.WriteBinding.StubParamIDFormat, bandIndex)
		pluginIdentifier := firstStringFromMap(typedAction, "plugin_identifier")
		if pluginIdentifier == "" {
			pluginIdentifier = defaultD1StaticEQPluginIdentifier
		}
		return map[string]any{"plugin_identifier": pluginIdentifier, "param_id": paramID, "target_value": valueDB}, paramID
	}
	paramID := writeBinding.ParamID
	args := map[string]any{
		"write_mode":   executionports.WriteModeNormalizedBatchV1,
		"plugin_path":  writeBinding.PluginPath,
		"plugin_name":  writeBinding.PluginName,
		"param_id":     paramID,
		"param_id_ch2": writeBinding.ParamIDCH2,
		"target_value": valueDB,
	}
	if writeBinding.Section == d1StaticEQDomain && writeBinding.FrequencyHz > 0 {
		args["frequency_hz"] = writeBinding.FrequencyHz
	}
	if spec.TargetSemantics != "" {
		args["target_semantics"] = spec.TargetSemantics
	}
	return args, paramID
}
