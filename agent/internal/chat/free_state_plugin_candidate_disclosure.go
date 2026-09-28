package chat

import (
	"fmt"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/experimentplugins"
)

// FIX-PLUGIN-SELECT-1 proposal-time bounded disclosure: when a whitelist
// family carries MORE THAN one certified candidate (schema v6), the model
// must choose among them for its needs_experiment proposal. The candidate set
// is disclosed as machine whitelist state through the same mechanical
// disclosure channel the observation-saturation notice uses: structural
// fields only (name / manufacturer / format / identifier plus the family
// capability face), never the machine-local plugin_path or any parameter id.
// Single-candidate (v5-shaped) families disclose nothing — their proposal
// behavior is unchanged and the prompt stays byte-identical on
// single-candidate machines. Selection is enforced fail-closed at the
// admission (whitelist membership), never by this disclosure.

const freeStatePluginCandidateDisclosureSchema = "free_state_plugin_candidate_disclosure.v1"

// freeStatePluginCandidateCapability describes one family's capability face:
// the bounded parameter surface every candidate of the family is certified to
// expose. All candidates inside one family share the family face; the
// per-candidate differentiators are the structural identity fields alone.
// The dose ceiling wording derives from the experiment-package constants
// (DOSE-AUDIBLE-1) so the disclosure cannot drift from the admission gate.
var freeStatePluginCandidateCapabilities = map[string]string{
	d1StaticEQDomain:             fmt.Sprintf("one bounded static EQ band gain move within %s at the family's pinned band centers", experiment.D1S1DBDoseRangeText()),
	d1BroadbandCompressionDomain: fmt.Sprintf("one bounded broadband compressor threshold move within %s on a dual-channel surface", experiment.D1S1DBDoseRangeText()),
	d1DeEsserDomain:              fmt.Sprintf("one bounded de-esser threshold move within %s on a single shared parameter", experiment.D1S1DBDoseRangeText()),
	d1TransientShaperDomain:      fmt.Sprintf("one bounded transient attack move within %s on a single shared parameter", experiment.D1S1DBDoseRangeText()),
	d1LimiterDomain:              fmt.Sprintf("one bounded output ceiling move within %s on a single shared parameter", experiment.D1S1DBDoseRangeText()),
	d1GateExpanderDomain:         fmt.Sprintf("one bounded gate range move within %s on a single shared parameter", experiment.D1S1DBDoseRangeText()),
	d1MultibandDomain:            fmt.Sprintf("one bounded per-band threshold move within %s indexed by band_index", experiment.D1S1DBDoseRangeText()),
}

// buildFreeStatePluginCandidateDisclosure loads the machine whitelist and
// returns the bounded candidate disclosure for every family with more than
// one certified candidate. Any loader failure (including a missing whitelist
// file) yields a nil disclosure: the disclosure is optional guidance for the
// proposal turn, while the admission keeps failing closed on the real
// whitelist at execution time.
func buildFreeStatePluginCandidateDisclosure() map[string]any {
	whitelist, err := d1StaticEQWhitelistLoader()
	if err != nil {
		return nil
	}
	var families []map[string]any
	addFamily := func(domain string, identities []map[string]any) {
		if len(identities) < 2 {
			return
		}
		families = append(families, map[string]any{
			"action_domain": domain,
			"capability":    freeStatePluginCandidateCapabilities[domain],
			"candidates":    identities,
		})
	}
	addFamily(d1StaticEQDomain, identitiesOf(whitelist.StaticEQ, func(e experimentplugins.StaticEQPlugin) (string, string, string, string) {
		return e.PluginName, e.Manufacturer, e.Format, e.PluginIdentifier
	}))
	addFamily(d1BroadbandCompressionDomain, identitiesOf(whitelist.BroadbandCompression, func(e experimentplugins.BroadbandCompressionPlugin) (string, string, string, string) {
		return e.PluginName, e.Manufacturer, e.Format, e.PluginIdentifier
	}))
	addFamily(d1DeEsserDomain, identitiesOf(whitelist.DeEsser, func(e experimentplugins.DeEsserPlugin) (string, string, string, string) {
		return e.PluginName, e.Manufacturer, e.Format, e.PluginIdentifier
	}))
	addFamily(d1TransientShaperDomain, identitiesOf(whitelist.TransientShaper, func(e experimentplugins.TransientShaperPlugin) (string, string, string, string) {
		return e.PluginName, e.Manufacturer, e.Format, e.PluginIdentifier
	}))
	addFamily(d1LimiterDomain, identitiesOf(whitelist.Limiter, func(e experimentplugins.LimiterPlugin) (string, string, string, string) {
		return e.PluginName, e.Manufacturer, e.Format, e.PluginIdentifier
	}))
	addFamily(d1GateExpanderDomain, identitiesOf(whitelist.GateExpander, func(e experimentplugins.GateExpanderPlugin) (string, string, string, string) {
		return e.PluginName, e.Manufacturer, e.Format, e.PluginIdentifier
	}))
	addFamily(d1MultibandDomain, identitiesOf(whitelist.Multiband, func(e experimentplugins.MultibandPlugin) (string, string, string, string) {
		return e.PluginName, e.Manufacturer, e.Format, e.PluginIdentifier
	}))
	if len(families) == 0 {
		return nil
	}
	return map[string]any{
		"schema_version": freeStatePluginCandidateDisclosureSchema,
		"families":       families,
	}
}

// identitiesOf projects one family's entries into the disclosure's structural
// candidate rows.
func identitiesOf[T any](entries []T, fields func(T) (name, manufacturer, formatV, identifier string)) []map[string]any {
	rows := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		name, manufacturer, formatV, identifier := fields(entry)
		rows = append(rows, map[string]any{
			"name":         name,
			"manufacturer": manufacturer,
			"format":       formatV,
			"identifier":   identifier,
		})
	}
	return rows
}
