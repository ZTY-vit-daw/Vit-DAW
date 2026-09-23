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

// d1PluginParamWhitelistBinding is the normalized plugin-parameter write
// resolution shared by every PluginBound domain: which whitelisted plugin file
// to instantiate and which parameter ids to write under one idempotency key —
// a ch1/ch2 pair for dual-channel carriers, a single shared parameter id for
// single-channel ones (empty ParamIDCH2: FAM1-S1 de_esser and
// FIX-BROADBAND-SHARED-1 shared-form broadband entries). static_eq
// resolutions are mapped into this shape after the legacy band resolver ran;
// broadband_compression and de_esser resolve directly.
type d1PluginParamWhitelistBinding struct {
	Section          string  // whitelist section label, e.g. "static_eq"
	PluginName       string  `json:"-"`
	PluginPath       string  `json:"-"`
	PluginIdentifier string  `json:"-"` // whitelist CID; kernel resolves shell paths by this, fail-closed without one (FIX-D1-PLUGIDENT-1)
	ParamID          string  // ch1 parameter id (fingerprint + journal identity)
	ParamIDCH2       string  // paired channel written in the same batch
	FrequencyHz      float64 // static_eq provenance only; 0 for other domains
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
	if domainLabel(typedAction) == d1DeEsserDomain {
		return resolveD1DeEsserWhitelistBinding(typedAction)
	}
	if domainLabel(typedAction) == d1TransientShaperDomain {
		return resolveD1TransientShaperWhitelistBinding(typedAction)
	}
	if domainLabel(typedAction) == d1LimiterDomain {
		return resolveD1LimiterWhitelistBinding(typedAction)
	}
	if domainLabel(typedAction) == d1GateExpanderDomain {
		return resolveD1GateExpanderWhitelistBinding(typedAction)
	}
	if domainLabel(typedAction) == d1MultibandDomain {
		return resolveD1MultibandWhitelistBinding(typedAction)
	}
	binding, err := resolveD1StaticEQWhitelistBinding(typedAction)
	if err != nil {
		return nil, err
	}
	return &d1PluginParamWhitelistBinding{
		Section:          "static_eq",
		PluginName:       binding.Plugin.PluginName,
		PluginPath:       binding.Plugin.PluginPath,
		PluginIdentifier: binding.Plugin.PluginIdentifier,
		ParamID:          binding.Band.GainParamIDCH1,
		ParamIDCH2:       binding.Band.GainParamIDCH2,
		FrequencyHz:      binding.FrequencyHz,
	}, nil
}

// resolveD1BroadbandCompressionWhitelistBinding mirrors the static_eq gate for
// the compression whitelist section with its own label so upstream can tell
// the domains apart while keeping the class wording aligned. FIX-PLUGIN-
// SELECT-1: the pinned plugin_identifier is a membership selection over the
// v6 candidate list (non-members fail closed) and the binding resolves to the
// selected entry. FIX-BROADBAND-SHARED-1: the selected entry's threshold form
// decides the write shape — dual ch pair or the single shared parameter id
// (one-entry batch, FAM1-S1 semantics).
func resolveD1BroadbandCompressionWhitelistBinding(typedAction map[string]any) (*d1PluginParamWhitelistBinding, error) {
	const label = d1BroadbandCompressionDomain
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) || errors.Is(err, experimentplugins.ErrCompressionNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrCompressionNotConfigured)
		}
		return nil, fmt.Errorf("%s experiment whitelist is invalid: %w", label, err)
	}
	compression, err := whitelist.SelectBroadbandCompression(firstStringFromMap(typedAction, "plugin_identifier"))
	if err != nil {
		if errors.Is(err, experimentplugins.ErrCompressionNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrCompressionNotConfigured)
		}
		return nil, err
	}
	library, err := d1StaticEQAttestationReader()
	if err != nil {
		return nil, fmt.Errorf("%s experiment could not evaluate its PCA admission: %w", label, err)
	}
	if err := whitelist.ValidateCompressionAdmission(library, compression.PluginIdentifier); err != nil {
		return nil, fmt.Errorf("%s experiment was refused by the PCA admission check: %w", label, err)
	}
	// FIX-BROADBAND-SHARED-1: a shared-form entry (single threshold_param_id)
	// resolves to a one-entry batch write (empty ch2), a dual-form entry keeps
	// the ch pair — the normalized binding carries both shapes the port
	// already accepts.
	paramID, paramIDCH2 := compression.ThresholdParamPair()
	return &d1PluginParamWhitelistBinding{
		Section:          label,
		PluginName:       compression.PluginName,
		PluginPath:       compression.PluginPath,
		PluginIdentifier: compression.PluginIdentifier,
		ParamID:          paramID,
		ParamIDCH2:       paramIDCH2,
	}, nil
}

// domainLabel reads the canonical action_domain of an admitted typed action.
func domainLabel(typedAction map[string]any) string {
	return strings.ToLower(strings.TrimSpace(firstNonEmpty(firstStringFromMap(typedAction, "action_domain", "domain"), "")))
}

// resolveD1DeEsserWhitelistBinding mirrors the broadband_compression gate for
// the de_esser whitelist section with its own label so upstream can tell the
// domains apart. The whitelisted de-esser carries ONE shared threshold
// parameter (Pro-DS probe 2026-08-31), so the binding resolves a single
// channel; admission runs against the PCA v2 library (de_esser is a v2
// family, GLM ruling on D2-FAM1-S1: Form A dispatch).
func resolveD1DeEsserWhitelistBinding(typedAction map[string]any) (*d1PluginParamWhitelistBinding, error) {
	const label = d1DeEsserDomain
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) || errors.Is(err, experimentplugins.ErrDeEsserNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrDeEsserNotConfigured)
		}
		return nil, fmt.Errorf("%s experiment whitelist is invalid: %w", label, err)
	}
	deEsser, err := whitelist.SelectDeEsser(firstStringFromMap(typedAction, "plugin_identifier"))
	if err != nil {
		if errors.Is(err, experimentplugins.ErrDeEsserNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrDeEsserNotConfigured)
		}
		return nil, err
	}
	library, err := d1DeEsserAttestationReader()
	if err != nil {
		return nil, fmt.Errorf("%s experiment could not evaluate its PCA admission: %w", label, err)
	}
	if err := whitelist.ValidateDeEsserAdmission(library, deEsser.PluginIdentifier); err != nil {
		return nil, fmt.Errorf("%s experiment was refused by the PCA admission check: %w", label, err)
	}
	return &d1PluginParamWhitelistBinding{
		Section:          label,
		PluginName:       deEsser.PluginName,
		PluginPath:       deEsser.PluginPath,
		PluginIdentifier: deEsser.PluginIdentifier,
		ParamID:          deEsser.ThresholdParamID,
	}, nil
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

// d1DeEsserAttestationReader is the v2-generation companion of
// d1StaticEQAttestationReader: de_esser attestations live in the PCA v2
// store. A missing store decodes to an empty library, which makes every
// admission query answer not-promoted.
var d1DeEsserAttestationReader = func() (processorattestation.LibraryV2, error) {
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		return processorattestation.LibraryV2{}, err
	}
	library, report, err := store.Read()
	if err != nil {
		return processorattestation.LibraryV2{}, fmt.Errorf("processor attestation v2 store is unreadable at %s: %w", report.Path, err)
	}
	return library, nil
}

// d1TransientShaperAttestationReader is the transient_shaper companion of
// d1DeEsserAttestationReader: transient_shaper attestations live in the same
// PCA v2 store. A missing store decodes to an empty library, which makes
// every admission query answer not-promoted.
var d1TransientShaperAttestationReader = func() (processorattestation.LibraryV2, error) {
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		return processorattestation.LibraryV2{}, err
	}
	library, report, err := store.Read()
	if err != nil {
		return processorattestation.LibraryV2{}, fmt.Errorf("processor attestation v2 store is unreadable at %s: %w", report.Path, err)
	}
	return library, nil
}

// resolveD1TransientShaperWhitelistBinding mirrors the de_esser gate for the
// transient_shaper whitelist section with its own label so upstream can tell
// the domains apart. The whitelisted transient shaper carries ONE shared
// attack parameter (SPL TD+ probe 2026-09-01), so the binding resolves a
// single channel; admission runs against the PCA v2 library (transient_shaper
// is a v2 family, same Form A dispatch as de_esser).
func resolveD1TransientShaperWhitelistBinding(typedAction map[string]any) (*d1PluginParamWhitelistBinding, error) {
	const label = d1TransientShaperDomain
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) || errors.Is(err, experimentplugins.ErrTransientShaperNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrTransientShaperNotConfigured)
		}
		return nil, fmt.Errorf("%s experiment whitelist is invalid: %w", label, err)
	}
	transient, err := whitelist.SelectTransientShaper(firstStringFromMap(typedAction, "plugin_identifier"))
	if err != nil {
		if errors.Is(err, experimentplugins.ErrTransientShaperNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrTransientShaperNotConfigured)
		}
		return nil, err
	}
	library, err := d1TransientShaperAttestationReader()
	if err != nil {
		return nil, fmt.Errorf("%s experiment could not evaluate its PCA admission: %w", label, err)
	}
	if err := whitelist.ValidateTransientShaperAdmission(library, transient.PluginIdentifier); err != nil {
		return nil, fmt.Errorf("%s experiment was refused by the PCA admission check: %w", label, err)
	}
	return &d1PluginParamWhitelistBinding{
		Section:          label,
		PluginName:       transient.PluginName,
		PluginPath:       transient.PluginPath,
		PluginIdentifier: transient.PluginIdentifier,
		ParamID:          transient.AttackParamID,
	}, nil
}

// d1LimiterAttestationReader is the limiter companion of
// d1TransientShaperAttestationReader: limiter attestations live in the same
// PCA v2 store. A missing store decodes to an empty library, which makes
// every admission query answer not-promoted.
var d1LimiterAttestationReader = func() (processorattestation.LibraryV2, error) {
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		return processorattestation.LibraryV2{}, err
	}
	library, report, err := store.Read()
	if err != nil {
		return processorattestation.LibraryV2{}, fmt.Errorf("processor attestation v2 store is unreadable at %s: %w", report.Path, err)
	}
	return library, nil
}

// resolveD1LimiterWhitelistBinding mirrors the transient_shaper gate for the
// limiter whitelist section with its own label so upstream can tell the
// domains apart. The whitelisted limiter carries ONE shared ceiling
// parameter (FabFilter Pro-L 2 probe 2026-09-02), so the binding resolves a
// single channel; admission runs against the PCA v2 library (limiter is a
// v2 family, same Form A dispatch as de_esser/transient_shaper).
func resolveD1LimiterWhitelistBinding(typedAction map[string]any) (*d1PluginParamWhitelistBinding, error) {
	const label = d1LimiterDomain
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) || errors.Is(err, experimentplugins.ErrLimiterNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrLimiterNotConfigured)
		}
		return nil, fmt.Errorf("%s experiment whitelist is invalid: %w", label, err)
	}
	limiter, err := whitelist.SelectLimiter(firstStringFromMap(typedAction, "plugin_identifier"))
	if err != nil {
		if errors.Is(err, experimentplugins.ErrLimiterNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrLimiterNotConfigured)
		}
		return nil, err
	}
	library, err := d1LimiterAttestationReader()
	if err != nil {
		return nil, fmt.Errorf("%s experiment could not evaluate its PCA admission: %w", label, err)
	}
	if err := whitelist.ValidateLimiterAdmission(library, limiter.PluginIdentifier); err != nil {
		return nil, fmt.Errorf("%s experiment was refused by the PCA admission check: %w", label, err)
	}
	return &d1PluginParamWhitelistBinding{
		Section:          label,
		PluginName:       limiter.PluginName,
		PluginPath:       limiter.PluginPath,
		PluginIdentifier: limiter.PluginIdentifier,
		ParamID:          limiter.CeilingParamID,
	}, nil
}

// d1GateExpanderAttestationReader is the gate_expander companion of
// d1LimiterAttestationReader: gate_expander attestations live in the same
// PCA v2 store. A missing store decodes to an empty library, which makes
// every admission query answer not-promoted.
var d1GateExpanderAttestationReader = func() (processorattestation.LibraryV2, error) {
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		return processorattestation.LibraryV2{}, err
	}
	library, report, err := store.Read()
	if err != nil {
		return processorattestation.LibraryV2{}, fmt.Errorf("processor attestation v2 store is unreadable at %s: %w", report.Path, err)
	}
	return library, nil
}

// resolveD1GateExpanderWhitelistBinding mirrors the limiter gate for the
// gate_expander whitelist section with its own label so upstream can tell the
// domains apart. The whitelisted gate carries ONE shared range parameter
// (FabFilter Pro-G probe 2026-09-02), so the binding resolves a single
// channel; admission runs against the PCA v2 library (gate_expander is a v2
// family, same Form A dispatch as the other v2 sections).
func resolveD1GateExpanderWhitelistBinding(typedAction map[string]any) (*d1PluginParamWhitelistBinding, error) {
	const label = d1GateExpanderDomain
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) || errors.Is(err, experimentplugins.ErrGateExpanderNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrGateExpanderNotConfigured)
		}
		return nil, fmt.Errorf("%s experiment whitelist is invalid: %w", label, err)
	}
	gate, err := whitelist.SelectGateExpander(firstStringFromMap(typedAction, "plugin_identifier"))
	if err != nil {
		if errors.Is(err, experimentplugins.ErrGateExpanderNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrGateExpanderNotConfigured)
		}
		return nil, err
	}
	library, err := d1GateExpanderAttestationReader()
	if err != nil {
		return nil, fmt.Errorf("%s experiment could not evaluate its PCA admission: %w", label, err)
	}
	if err := whitelist.ValidateGateExpanderAdmission(library, gate.PluginIdentifier); err != nil {
		return nil, fmt.Errorf("%s experiment was refused by the PCA admission check: %w", label, err)
	}
	return &d1PluginParamWhitelistBinding{
		Section:          label,
		PluginName:       gate.PluginName,
		PluginPath:       gate.PluginPath,
		PluginIdentifier: gate.PluginIdentifier,
		ParamID:          gate.RangeParamID,
	}, nil
}

// d1MultibandAttestationReader is the multiband companion of
// d1GateExpanderAttestationReader: multiband attestations live in the same
// PCA v2 store. A missing store decodes to an empty library, which makes
// every admission query answer not-promoted.
var d1MultibandAttestationReader = func() (processorattestation.LibraryV2, error) {
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		return processorattestation.LibraryV2{}, err
	}
	library, report, err := store.Read()
	if err != nil {
		return processorattestation.LibraryV2{}, fmt.Errorf("processor attestation v2 store is unreadable at %s: %w", report.Path, err)
	}
	return library, nil
}

// resolveD1MultibandWhitelistBinding mirrors the gate_expander gate for the
// multiband whitelist section with its own label so upstream can tell the
// domains apart. The whitelisted multiband carries ONE threshold parameter
// per band (Lindell MBC probe 2026-09-02, no ch pair), so the binding
// resolves the single band threshold named by the admission's band_index
// (mirroring the static_eq band selection; 0 = the carrier's first band);
// admission runs against the PCA v2 library (multiband_dynamics is a v2
// family, same Form A dispatch as the other v2 sections).
func resolveD1MultibandWhitelistBinding(typedAction map[string]any) (*d1PluginParamWhitelistBinding, error) {
	const label = d1MultibandDomain
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) || errors.Is(err, experimentplugins.ErrMultibandNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrMultibandNotConfigured)
		}
		return nil, fmt.Errorf("%s experiment whitelist is invalid: %w", label, err)
	}
	multiband, err := whitelist.SelectMultiband(firstStringFromMap(typedAction, "plugin_identifier"))
	if err != nil {
		if errors.Is(err, experimentplugins.ErrMultibandNotConfigured) {
			return nil, fmt.Errorf("%s experiment is not configured: %w", label, experimentplugins.ErrMultibandNotConfigured)
		}
		return nil, err
	}
	bandIndex := 0
	if value, present := typedAction["band_index"]; present {
		parsed, ok := treatmentNumber(map[string]any{"band_index": value}, "band_index")
		if !ok || math.IsInf(parsed, 0) || math.IsNaN(parsed) || parsed != math.Trunc(parsed) {
			return nil, fmt.Errorf("%s admission requires band_index to be a whole number", label)
		}
		bandIndex = int(parsed)
	}
	if bandIndex < 0 || bandIndex >= len(multiband.BandThresholdParamIDs) {
		return nil, fmt.Errorf("%s admission band_index %d is outside the whitelisted band threshold ids (0-%d)", label, bandIndex, len(multiband.BandThresholdParamIDs)-1)
	}
	library, err := d1MultibandAttestationReader()
	if err != nil {
		return nil, fmt.Errorf("%s experiment could not evaluate its PCA admission: %w", label, err)
	}
	if err := whitelist.ValidateMultibandAdmission(library, multiband.PluginIdentifier); err != nil {
		return nil, fmt.Errorf("%s experiment was refused by the PCA admission check: %w", label, err)
	}
	return &d1PluginParamWhitelistBinding{
		Section:          label,
		PluginName:       multiband.PluginName,
		PluginPath:       multiband.PluginPath,
		PluginIdentifier: multiband.PluginIdentifier,
		ParamID:          multiband.BandThresholdParamIDs[bandIndex],
	}, nil
}

// resolveD1StaticEQWhitelistBinding turns one admitted static_eq typed action
// into the whitelisted band binding. The returned errors carry deliberately
// distinct boundaries so upstream can tell apart: not configured, corrupt
// whitelist, not PCA-promoted, attestation store unreadable, and an admission
// that pins an identifier outside the whitelist. An eligibility ruling here
// gates execution only; it never changes what the domain validators enforce.
// FIX-PLUGIN-SELECT-1: the pinned plugin_identifier selects one member of the
// v6 candidate list (non-members fail closed) and the band resolves inside
// the selected entry's bands.
func resolveD1StaticEQWhitelistBinding(typedAction map[string]any) (*d1StaticEQWhitelistBinding, error) {
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) {
			return nil, fmt.Errorf("static_eq experiment is not configured: %w", err)
		}
		return nil, fmt.Errorf("static_eq experiment whitelist is invalid: %w", err)
	}
	plugin, err := whitelist.SelectStaticEQ(firstStringFromMap(typedAction, "plugin_identifier"))
	if err != nil {
		if errors.Is(err, experimentplugins.ErrNotConfigured) {
			return nil, fmt.Errorf("static_eq experiment is not configured: %w", experimentplugins.ErrNotConfigured)
		}
		return nil, err
	}
	frequency, ok := treatmentNumber(typedAction, "frequency_hz")
	if !ok || frequency < 20 || frequency > 20000 {
		return nil, fmt.Errorf("static_eq execution requires frequency_hz within 20-20000 Hz in the admitted typed action")
	}
	library, err := d1StaticEQAttestationReader()
	if err != nil {
		return nil, fmt.Errorf("static_eq experiment could not evaluate its PCA admission: %w", err)
	}
	if err := whitelist.ValidateStaticEQAdmission(library, plugin.PluginIdentifier); err != nil {
		return nil, fmt.Errorf("static_eq experiment was refused by the PCA admission check: %w", err)
	}
	band, err := plugin.NearestBand(frequency)
	if err != nil {
		return nil, fmt.Errorf("static_eq experiment whitelist has no usable band: %w", err)
	}
	return &d1StaticEQWhitelistBinding{Plugin: plugin, Band: band, FrequencyHz: frequency}, nil
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
		case "value", "db", "pan":
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
		"write_mode":        executionports.WriteModeNormalizedBatchV1,
		"plugin_path":       binding.Plugin.PluginPath,
		"plugin_name":       binding.Plugin.PluginName,
		"plugin_identifier": binding.Plugin.PluginIdentifier,
		"param_id":          paramID,
		"param_id_ch2":      binding.Band.GainParamIDCH2,
		"target_value":      gainDB,
		"frequency_hz":      binding.FrequencyHz,
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
// d1PluginParamPlanWithBinding freezes one bounded plugin-parameter mutation.
// existingPluginInstanceID is the caller's discovery result from the live
// plugin graph (get_project_state): non-empty means a same-identity instance
// already sits on the target track, so the plan targets it and the port skips
// the instantiate command — reruns rewrite that instance's parameters
// instead of stacking another one (the kernel persists the plugin graph, so
// without this every session adds a fresh instance to the same track). Empty
// keeps the historical instantiate path. The revision-bound VSP state itself
// never carries plugin rows (the kernel's compact snapshot strips them), so
// discovery cannot live inside this builder.
func d1PluginParamPlanWithBinding(loop freeStateReasoningLoop, candidate agentloop.PendingMixTickCandidate, stateRevision int64, projectUUID, projectEpoch, snapshotHash string, state map[string]any, writeBinding *d1PluginParamWhitelistBinding, existingPluginInstanceID string) (orchestration.FrozenPlan, error) {
	if loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return orchestration.FrozenPlan{}, fmt.Errorf("active D1-S1 experiment is required")
	}
	if err := validateD1TierAdmission(loop.Experiment.Admission); err != nil {
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
	if writeBinding != nil && strings.TrimSpace(existingPluginInstanceID) != "" {
		args["plugin_id"] = strings.TrimSpace(existingPluginInstanceID)
	}
	if loop.Experiment.Admission.IsD2MultiRound() {
		if err := d2MultiRoundCheckProjectedCumulativeDelta(loop.Experiment, spec.AdmissionValueKey, valueDB); err != nil {
			return orchestration.FrozenPlan{}, err
		}
	}
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
	action := orchestration.Action{ID: "d1_" + sanitizeCanaryID(loop.Experiment.ID) + d1MultiRoundRoundScopeSuffix(loop) + spec.ActionIDSuffix, Command: spec.ActionKind, TargetRef: targetID,
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
		"write_mode":        executionports.WriteModeNormalizedBatchV1,
		"plugin_path":       writeBinding.PluginPath,
		"plugin_name":       writeBinding.PluginName,
		"plugin_identifier": writeBinding.PluginIdentifier,
		"param_id":          paramID,
		"target_value":      valueDB,
	}
	// Single-channel domains (FAM1-S1 de_esser) carry no ch2: the action args
	// omit the key entirely (GLM ruling on D2-FAM1-S1 ③, correction b); the
	// execution port records the empty second channel in its receipt details.
	if writeBinding.ParamIDCH2 != "" {
		args["param_id_ch2"] = writeBinding.ParamIDCH2
	}
	if writeBinding.Section == d1StaticEQDomain && writeBinding.FrequencyHz > 0 {
		args["frequency_hz"] = writeBinding.FrequencyHz
	}
	if spec.TargetSemantics != "" {
		args["target_semantics"] = spec.TargetSemantics
	}
	return args, paramID
}

// d1ExistingPluginInstanceID resolves the instance id of a plugin already
// loaded on one track of a live plugin-graph read (the get_project_state
// legacy reply — the same surface the bridge uses for shadow refreshes;
// its track rows carry the plugin chain, unlike the revision-bound VSP
// compact snapshot). Since governed loads issue rack_add_node (F4A), a
// loaded EQ no longer appears in the flat plugins array (only the rack
// wrapper row does) — it lives under the nested rack.nodes — so the read
// walks all three faces (plugins, rack_nodes, rack.nodes) like
// daw.VisiblePluginRefs. Rows expose only the display identity (name plus
// type), never the whitelist's identifier or path, so the folded name
// comparison is the strongest identity the state can offer; a mismatched
// same-named plugin still fails closed downstream because the port reads the
// bound parameter ids off the resolved instance. The first matching row wins
// deterministically, making reuse a pure function of the graph read: replays
// converge on one instance instead of stacking new ones.
func d1ExistingPluginInstanceID(state map[string]any, trackID, pluginName string) string {
	pluginName = strings.TrimSpace(pluginName)
	if pluginName == "" {
		return ""
	}
	for _, row := range mapRowsFromAny(state["tracks"]) {
		if firstStringFromMap(row, "track_id", "id") != trackID {
			continue
		}
		pluginRows := [][]map[string]any{
			mapRowsFromAny(row["plugins"]),
			mapRowsFromAny(row["rack_nodes"]),
		}
		if rack, ok := row["rack"].(map[string]any); ok {
			pluginRows = append(pluginRows, mapRowsFromAny(rack["nodes"]))
		}
		for _, rows := range pluginRows {
			for _, plugin := range rows {
				name := strings.TrimSpace(firstStringFromMap(plugin, "plugin_name", "name", "display_name", "label"))
				if !strings.EqualFold(name, pluginName) {
					continue
				}
				if instanceID := firstStringFromMap(plugin, "plugin_item_id", "item_id", "plugin_id", "id", "node_id"); instanceID != "" {
					return instanceID
				}
			}
		}
		return ""
	}
	return ""
}
