package experiment

import (
	"fmt"
	"math"
	"strings"

	"vit-daw-agent/internal/agentprotocol"
)

// D1S1JournalField maps one plan Args key into the durable journal Command
// map for a recorded mutation. ArgFallback covers domains whose identity key
// degrades gracefully (plugin_id falling back to plugin_identifier).
type D1S1JournalField struct {
	Arg         string
	ArgFallback string
	CommandKey  string
}

// D1S1JournalShape is the audit wording of one executed domain action as it
// lands in the harness journal while running. It mirrors wording only: the
// budget/attempts/idempotency semantics stay outside this table.
type D1S1JournalShape struct {
	Summary      string
	Tool         string
	CommandLabel string
	Fields       []D1S1JournalField
}

// D1S1WriteBinding describes how one domain's admitted gain parameter reaches
// the kernel. It is routing metadata for the plan builder; validators never
// consult it.
type D1S1WriteBinding struct {
	// PluginBound marks domains that write plugin band parameters instead of
	// a track fader.
	PluginBound bool
	// StubParamIDFormat assembles a param id from the admission's band_index
	// without any machine-local whitelist ("%d"). The static_eq row keeps its
	// D2-1 stub format so the table-driven builder can reproduce the
	// historical plan byte-for-byte when no whitelisted plugin resolves;
	// production resolution hard-fails before reaching that path.
	StubParamIDFormat string
	// Channels is how many gain parameters one action writes (1 = single,
	// 2 = paired dual-channel batch write under one idempotency key).
	Channels int
}

// D1S1DomainSpec describes one action domain admitted by the narrow bounded
// experiment gate (Phase D1-S1 and its D2-1 extension). Each domain carries
// its own equally tight parameter bounds; admitting another domain never
// relaxes the shared invariants (budget 1, one round, one forward mutation,
// observation-bound track target).
//
// The execution descriptor fields below (ActionIDSuffix onward) are consumed
// by the chat plan/journal builders so per-domain shapes derive from this
// table instead of parallel hand-written mirrors. They carry no enforcement
// authority.
type D1S1DomainSpec struct {
	ActionDomain string
	ActionKind   string
	// PromptParameterHint is the model-facing description of this domain's
	// parameter_bounds shape and absolute bounds. It is derived wording only:
	// admitting a domain or editing this hint never changes what
	// ValidateTypedAction/ValidateDoseBounds enforce.
	PromptParameterHint string
	// ValidateTypedAction checks the domain-specific typed action parameters.
	ValidateTypedAction func(a Admission) error
	// ValidateDoseBounds checks one dose-bounds map ("diagnostic" or
	// "retained") against the domain's acoustic parameter bounds.
	ValidateDoseBounds func(scope string, bounds map[string]any) error

	// ActionIDSuffix appends to the "d1_<experiment>_..." action id.
	ActionIDSuffix string
	// CapabilityID identifies the domain in the frozen ActionSet/Proposal.
	CapabilityID string
	// ContractVersions lists the ProjectCut contract versions verbatim.
	ContractVersions []string
	// Fingerprint templates use {track}, {db}, and {param} markers; the
	// builder substitutes the values each domain owns.
	TargetFingerprintTemplate string
	BeforeFingerprintTemplate string
	// ObservationViewIDs bind the post-action CCB observation projection.
	ObservationViewIDs []string
	// Journal shapes the running audit record.
	Journal D1S1JournalShape
	// WriteBinding routes the parameter write.
	WriteBinding D1S1WriteBinding

	// AdmissionValueKey names the typed-action key carrying this domain's
	// single bounded dB move ("delta_db"/"gain_db"/"threshold_db"). The chat
	// admission assembler reads it to shape TypedAction and both dose-bounds
	// maps; the per-row validators remain the bounds authority.
	AdmissionValueKey string
	// AdmissionPassthroughKeys lists optional proposal ParameterBounds keys
	// that are copied verbatim into TypedAction (e.g. a plugin_identifier
	// pinning, which the whitelist gate still validates).
	AdmissionPassthroughKeys []string
	// AppliedReplyText is the human reply wording used once this domain's
	// single action is applied and read back. It is wording only: execution
	// outcomes never depend on it.
	AppliedReplyText string
	// TargetSemantics declares how the admitted value maps to a physical
	// write target: empty means the historical absolute dB target (static_eq
	// gain), "delta_db" means a bounded physical delta on top of the current
	// value (broadband compression threshold, whose display probe is
	// structurally degenerate and admits no absolute inversion). The chat
	// plan builder forwards it as target_semantics so the execution port can
	// pick the matching conversion machine.
	TargetSemantics string
	// SemanticIntentFamily and SemanticIntentCoverage anchor the server-owned
	// structured processor intent for this row: when a free-state admitted
	// experiment of this domain drives semantic dynamic execution, the server
	// freezes the intent axis from these fields instead of trusting the
	// model's live axis wording (D2-SEMINT1; FAM1-S1 ruling 2 anchors
	// de_esser on sibilance_reduction). Rows without an anchor never ride the
	// semantic dynamic chain from this table.
	SemanticIntentFamily   string
	SemanticIntentCoverage []string
}

var d1s1Domains = []D1S1DomainSpec{
	{
		ActionDomain:        D1S1ActionDomain,
		ActionKind:          D1S1ActionKind,
		PromptParameterHint: `parameter_bounds={"delta_db":<nonzero number within +/-2>}`,
		ValidateDoseBounds: func(scope string, bounds map[string]any) error {
			delta, ok := mapNumber(bounds, "delta_db")
			if !ok || delta == 0 || math.Abs(delta) > 2 {
				return fmt.Errorf("D1-S1 %s delta_db must be non-zero and within +/-2 dB", scope)
			}
			return nil
		},
		ActionIDSuffix:            "_gain",
		CapabilityID:              "static_mix.static_balance.v0",
		ContractVersions:          []string{"free_state:d1_s1", "action:track_gain_adjust"},
		TargetFingerprintTemplate: "track:{track}:fader:{db}",
		BeforeFingerprintTemplate: "track:{track}:fader_db:{db}",
		ObservationViewIDs:        []string{"mix.multitrack_relationship"},
		Journal: D1S1JournalShape{
			Summary:      "D1-S1 bounded track gain adjustment",
			Tool:         "track_gain_adjust",
			CommandLabel: "set_volume",
			Fields: []D1S1JournalField{
				{Arg: "target_db", CommandKey: "db"},
			},
		},
		WriteBinding:      D1S1WriteBinding{Channels: 1},
		AdmissionValueKey: "delta_db",
		AppliedReplyText:  "D1-S1 track gain parameter was applied and read back. Fresh post-action evidence is recorded separately; acoustic materiality, target response, and human judgment remain pending.",
	},
	{
		// static_eq adjusts one EQ band of one track. D2-1 admits it with the
		// same single-mutation tightness as track_gain: one band, one bounded
		// gain move, no frequency outside the audible range, no resonant Q.
		ActionDomain:        agentprotocol.ImprovementActionDomainStaticEQ,
		ActionKind:          "static_eq_band_adjust",
		PromptParameterHint: `parameter_bounds={"gain_db":<nonzero number within +/-2>,"frequency_hz":<number within 20-20000>,"q":<optional number within 0.1-18>,"band_index":<optional non-negative integer>}`,
		ValidateTypedAction: func(a Admission) error {
			frequency, ok := mapNumber(a.TypedAction, "frequency_hz")
			if !ok || frequency < 20 || frequency > 20000 {
				return fmt.Errorf("D1-S1 static_eq frequency_hz must be within 20-20000 Hz")
			}
			if index, present := a.TypedAction["band_index"]; present {
				parsed, ok := mapNumber(map[string]any{"band_index": index}, "band_index")
				if !ok || parsed < 0 || parsed != math.Trunc(parsed) {
					return fmt.Errorf("D1-S1 static_eq band_index must be a non-negative integer")
				}
			}
			if value, present := a.TypedAction["q"]; present {
				parsed, ok := mapNumber(map[string]any{"q": value}, "q")
				if !ok || parsed < 0.1 || parsed > 18 {
					return fmt.Errorf("D1-S1 static_eq q must be within 0.1-18")
				}
			}
			return nil
		},
		ValidateDoseBounds: func(scope string, bounds map[string]any) error {
			gain, ok := mapNumber(bounds, "gain_db")
			if !ok || gain == 0 || math.Abs(gain) > 2 {
				return fmt.Errorf("D1-S1 %s gain_db must be non-zero and within +/-2 dB", scope)
			}
			return nil
		},
		ActionIDSuffix:            "_eq",
		CapabilityID:              "static_mix.static_eq.v0",
		ContractVersions:          []string{"free_state:d1_s1", "action:static_eq_band_adjust"},
		TargetFingerprintTemplate: "track:{track}:eq:{param}:pending",
		BeforeFingerprintTemplate: "track:{track}:eq:{param}:pending",
		ObservationViewIDs:        []string{"mix.multitrack_relationship"},
		Journal: D1S1JournalShape{
			Summary:      "D2-1 bounded static EQ band adjustment",
			Tool:         "set_plugin_param",
			CommandLabel: "set_plugin_param",
			Fields: []D1S1JournalField{
				{Arg: "plugin_id", ArgFallback: "plugin_identifier", CommandKey: "plugin_id"},
				{Arg: "param_id", CommandKey: "param_id"},
				{Arg: "target_value", CommandKey: "value"},
			},
		},
		WriteBinding:             D1S1WriteBinding{PluginBound: true, StubParamIDFormat: "band_%d_gain", Channels: 2},
		AdmissionValueKey:        "gain_db",
		AdmissionPassthroughKeys: []string{"frequency_hz", "q", "band_index", "plugin_identifier"},
		AppliedReplyText:         "D2-1 static EQ band parameter was applied and read back. Fresh post-action evidence is recorded separately; acoustic materiality, target response, and human judgment remain pending.",
	},
	{
		// broadband_compression adjusts one compressor instance's dual-channel
		// threshold by one bounded move. D2-1.5 admits it with the same
		// single-mutation tightness as the other rows: one plugin instance, one
		// parameter pair (ch A/B under one idempotency key), attempts and
		// budget pinned at 1 elsewhere. Acoustic observation binds the COM
		// time-dynamics view instead of any static-level projection. There is
		// no stub form: production resolves the machine-local whitelist first
		// and hard-fails without it.
		ActionDomain:        agentprotocol.ImprovementActionDomainBroadbandCompression,
		ActionKind:          "broadband_threshold_adjust",
		PromptParameterHint: `parameter_bounds={"threshold_db":<nonzero number within +/-2>}`,
		ValidateDoseBounds: func(scope string, bounds map[string]any) error {
			threshold, ok := mapNumber(bounds, "threshold_db")
			if !ok || threshold == 0 || math.Abs(threshold) > 2 {
				return fmt.Errorf("D1-S1 %s threshold_db must be non-zero and within +/-2 dB", scope)
			}
			return nil
		},
		ActionIDSuffix:            "_comp",
		CapabilityID:              "static_mix.broadband_compression.v0",
		ContractVersions:          []string{"free_state:d1_s1", "action:broadband_threshold_adjust"},
		TargetFingerprintTemplate: "track:{track}:comp:{param}:pending",
		BeforeFingerprintTemplate: "track:{track}:comp:{param}:pending",
		ObservationViewIDs:        []string{"track.time_dynamics"},
		Journal: D1S1JournalShape{
			Summary:      "D2-1.5 bounded broadband compression threshold adjustment",
			Tool:         "set_plugin_param",
			CommandLabel: "set_plugin_param",
			Fields: []D1S1JournalField{
				{Arg: "plugin_id", ArgFallback: "plugin_identifier", CommandKey: "plugin_id"},
				{Arg: "param_id", CommandKey: "param_id"},
				{Arg: "target_value", CommandKey: "value"},
			},
		},
		WriteBinding:             D1S1WriteBinding{PluginBound: true, Channels: 2},
		AdmissionValueKey:        "threshold_db",
		AdmissionPassthroughKeys: []string{"plugin_identifier"},
		TargetSemantics:          "delta_db",
		AppliedReplyText:         "D2-1.5 broadband compression threshold was applied and read back. Fresh post-action evidence is recorded separately; acoustic materiality, target response, and human judgment remain pending.",
	},
	{
		// de_esser_adjusts one de-esser instance's threshold by one bounded
		// move. FAM1-S1 admits it with the same single-mutation tightness as
		// the other PluginBound rows. Unlike the PA compression carriers, the
		// whitelisted de-esser (FabFilter Pro-DS, pluginprobe 2026-08-31)
		// exposes ONE shared automatable threshold parameter (id "1", stereo
		// in/out) rather than a ch A/B pair, so the write is single-channel.
		// Acoustic observation binds the DOM frequency-time events view, whose
		// sibilance events carry the contrast the domain acts on. There is no
		// stub form: production resolves the machine-local whitelist first and
		// hard-fails without it.
		ActionDomain:        agentprotocol.ImprovementActionDomainDeEsser,
		ActionKind:          "de_esser_threshold_adjust",
		PromptParameterHint: `parameter_bounds={"threshold_db":<nonzero number within +/-2>}`,
		ValidateDoseBounds: func(scope string, bounds map[string]any) error {
			threshold, ok := mapNumber(bounds, "threshold_db")
			if !ok || threshold == 0 || math.Abs(threshold) > 2 {
				return fmt.Errorf("D1-S1 %s threshold_db must be non-zero and within +/-2 dB", scope)
			}
			return nil
		},
		ActionIDSuffix:            "_deess",
		CapabilityID:              "static_mix.de_ess.v0",
		ContractVersions:          []string{"free_state:d1_s1", "action:de_esser_threshold_adjust"},
		TargetFingerprintTemplate: "track:{track}:deess:{param}:pending",
		BeforeFingerprintTemplate: "track:{track}:deess:{param}:pending",
		ObservationViewIDs:        []string{"track.frequency_time_events"},
		Journal: D1S1JournalShape{
			Summary:      "FAM1-S1 bounded de-esser threshold adjustment",
			Tool:         "set_plugin_param",
			CommandLabel: "set_plugin_param",
			Fields: []D1S1JournalField{
				{Arg: "plugin_id", ArgFallback: "plugin_identifier", CommandKey: "plugin_id"},
				{Arg: "param_id", CommandKey: "param_id"},
				{Arg: "target_value", CommandKey: "value"},
			},
		},
		WriteBinding:             D1S1WriteBinding{PluginBound: true, Channels: 1},
		AdmissionValueKey:        "threshold_db",
		AdmissionPassthroughKeys: []string{"plugin_identifier"},
		TargetSemantics:          "delta_db",
	// The domain's certified semantic axis (FAM1-S1 ruling 2): a
	// free-state admitted de_esser experiment rides the semantic dynamic
	// chain on sibilance_reduction, not on the parameter-centric axis an
	// LLM would freeze from the literal threshold parameter name.
	SemanticIntentFamily:   "de_esser",
	SemanticIntentCoverage: []string{"sibilance_reduction"},
	AppliedReplyText:       "FAM1-S1 de-esser threshold was applied and read back. Fresh post-action evidence is recorded separately; acoustic materiality, target response, and human judgment remain pending.",
	},
	{
		// transient_attack_adjusts one transient-shaper instance's attack by one
		// bounded move. FAM2-S1 admits it with the same single-mutation
		// tightness as the other PluginBound rows. Like the whitelisted
		// de-esser, the whitelisted transient shaper (SPL Transient Designer
		// Plus, pluginprobe 2026-09-01) exposes ONE shared automatable attack
		// parameter (id "1098151019", stereo in/out, Link default On) rather
		// than a ch A/B pair, so the write is single-channel. Acoustic
		// observation binds the DOM transient-structure view, whose onset/body
		// contrast carries the domain quantity. There is no stub form:
		// production resolves the machine-local whitelist first and hard-fails
		// without it.
		ActionDomain:        agentprotocol.ImprovementActionDomainTransientShaper,
		ActionKind:          "transient_attack_adjust",
		PromptParameterHint: `parameter_bounds={"attack_db":<nonzero number within +/-2>}`,
		ValidateDoseBounds: func(scope string, bounds map[string]any) error {
			attack, ok := mapNumber(bounds, "attack_db")
			if !ok || attack == 0 || math.Abs(attack) > 2 {
				return fmt.Errorf("D1-S1 %s attack_db must be non-zero and within +/-2 dB", scope)
			}
			return nil
		},
		ActionIDSuffix:            "_trans",
		CapabilityID:              "static_mix.transient.v0",
		ContractVersions:          []string{"free_state:d1_s1", "action:transient_attack_adjust"},
		TargetFingerprintTemplate: "track:{track}:trans:{param}:pending",
		BeforeFingerprintTemplate: "track:{track}:trans:{param}:pending",
		ObservationViewIDs:        []string{"track.transient_structure"},
		Journal: D1S1JournalShape{
			Summary:      "FAM2-S1 bounded transient attack adjustment",
			Tool:         "set_plugin_param",
			CommandLabel: "set_plugin_param",
			Fields: []D1S1JournalField{
				{Arg: "plugin_id", ArgFallback: "plugin_identifier", CommandKey: "plugin_id"},
				{Arg: "param_id", CommandKey: "param_id"},
				{Arg: "target_value", CommandKey: "value"},
			},
		},
		WriteBinding:             D1S1WriteBinding{PluginBound: true, Channels: 1},
		AdmissionValueKey:        "attack_db",
		AdmissionPassthroughKeys: []string{"plugin_identifier"},
		TargetSemantics:          "delta_db",
		// The domain's certified semantic axis: a free-state admitted
		// transient_shaper experiment rides the semantic dynamic chain on
		// envelope_emphasis (SPL TD+ attestation coverage), not on the
		// parameter-centric axis an LLM would freeze from the literal attack
		// parameter name.
		SemanticIntentFamily:   "transient_shaper",
		SemanticIntentCoverage: []string{"envelope_emphasis"},
		AppliedReplyText:       "FAM2-S1 transient attack parameter was applied and read back. Fresh post-action evidence is recorded separately; acoustic materiality, target response, and human judgment remain pending.",
	},
}

// D1S1AdmittedDomains lists the action domains admitted by the bounded
// experiment gate, in registration order.
func D1S1AdmittedDomains() []string {
	out := make([]string, 0, len(d1s1Domains))
	for _, domain := range d1s1Domains {
		out = append(out, domain.ActionDomain)
	}
	return out
}

// D1S1DomainSpecs returns a copy of the admitted domain specs in registration
// order for read-only consumers (the model prompt layer derives its domain
// wording from this table; the validators remain the enforcement authority).
func D1S1DomainSpecs() []D1S1DomainSpec {
	return append([]D1S1DomainSpec(nil), d1s1Domains...)
}

// D1S1SpecForAction resolves the domain spec by explicit action_domain and
// action_kind names, for builders that hold the constants rather than a full
// admission.
func D1S1SpecForAction(actionDomain, actionKind string) (D1S1DomainSpec, bool) {
	for _, domain := range d1s1Domains {
		if domain.ActionDomain == actionDomain && domain.ActionKind == actionKind {
			return domain, true
		}
	}
	return D1S1DomainSpec{}, false
}

// D1S1DomainSpecFor resolves the admitted domain spec for an admission's
// typed action. A domain must match on both action_domain and action_kind so
// a hybrid action cannot borrow a domain's bounds.
func D1S1DomainSpecFor(a Admission) (D1S1DomainSpec, bool) {
	domainValue := strings.ToLower(mapString(a.TypedAction, "action_domain", "domain"))
	kindValue := strings.ToLower(mapString(a.TypedAction, "action_kind", "kind"))
	for _, domain := range d1s1Domains {
		if domainValue == domain.ActionDomain && kindValue == domain.ActionKind {
			return domain, true
		}
	}
	return D1S1DomainSpec{}, false
}

// D1S1SemanticIntentSpecFor resolves the admitted domain row when that row
// carries the server-owned structured intent anchor. The admission — which
// passed the domain validators — is the axis authority; callers must never
// substitute the model's live wording for the returned coverage axes.
func D1S1SemanticIntentSpecFor(a Admission) (D1S1DomainSpec, bool) {
	spec, found := D1S1DomainSpecFor(a)
	if !found || strings.TrimSpace(spec.SemanticIntentFamily) == "" || len(spec.SemanticIntentCoverage) == 0 {
		return D1S1DomainSpec{}, false
	}
	return spec, true
}
