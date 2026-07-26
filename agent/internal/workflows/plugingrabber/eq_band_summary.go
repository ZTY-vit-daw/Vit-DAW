package plugingrabber

import (
	"fmt"
	"strconv"
	"strings"
)

// EQ band slot kinds recognised inside a "Band N <slot>" parameter name.
const (
	eqSlotFreq  = "frequency"
	eqSlotGain  = "gain"
	eqSlotUsed  = "used"
	eqSlotShape = "shape"
	eqSlotQ     = "q"
)

// Marvel GEQ exposes its 16 fixed graphic bands as channel-1 parameters
// named 1EQ0..1EQ15 and does not expose frequency parameters at all. These
// are the standard ISO graphic-EQ center frequencies shown by its UI.
var marvelGEQFixedFrequenciesHz = []float64{
	20, 31.5, 50, 80, 125, 200, 315, 500,
	800, 1250, 2000, 3150, 5000, 8000, 12500, 20000,
}

type eqBandSlot struct {
	ParamID   string
	ValueText string
	Domain    *PluginDisplayDomain
	// Curve holds the (normalised, displayed value) pairs the kernel measured by
	// rendering this parameter at five positions. It is the parameter's actual
	// response, and the only sound basis for turning a target value into a
	// normalised one — see eqNormalizedFromCurve.
	Curve [][2]float64
}

type eqBandEntry struct {
	Slots map[string]eqBandSlot
}

// BuildEQBandSummary detects the EQ operation model and emits a compact band
// list so the model can make a correct band-selection decision without having
// to re-derive band structure from a flat 78+ parameter list on every turn.
//
// Two operation models exist in practice, and they need opposite handling:
//
//	fixed_slot     bands always exist (TDR Nova, SSL/Neve-style EQs). Pick the
//	               band whose current_freq_hz is closest to the target and move
//	               its gain; only retune frequency when nothing is near.
//
//	free_floating  bands are empty slots that must be created (FabFilter Pro-Q 3,
//	               Kirchhoff). Take an entry from available_slots and write the
//	               whole create sequence — omitting the "Used" write leaves the
//	               band inaudible and invisible in the plugin UI, which is a
//	               silent failure.
//
// Detection is structural, not per-plugin: a "Band N Used" parameter sitting at
// an unused-looking value means slots are created on demand. Nothing here is
// cached or persisted — it is recomputed from the live parameter read each time,
// which is what keeps it from becoming another profile format that can go stale.
//
// Note: template_role is only stored after an explicit learn step, so it is
// empty for freshly loaded plugins. The function also accepts the raw parameter
// list as evidence so it works with any EQ regardless of whether it has been
// profiled.
func BuildEQBandSummary(digest ParameterDigest) map[string]any {
	if summary := buildMarvelGEQSummary(digest); summary != nil {
		return summary
	}
	bands, order, indexFrequencies := collectEQBandsStructural(digest)
	if len(bands) == 0 {
		return nil
	}

	role := strings.ToLower(strings.TrimSpace(digest.TemplateRole))
	class := strings.ToLower(strings.TrimSpace(digest.PluginClass))
	isEQByMeta := role == "eq" || class == "eq"
	if !isEQByMeta && !eqBandsLookStructural(bands, indexFrequencies) {
		return nil
	}

	if eqBandsAreFreeFloating(bands) {
		return buildFreeFloatingSummary(bands, order)
	}
	if eqBandsHaveAdjustableFreq(bands) {
		return buildFixedSlotAdjustableSummary(bands, order)
	}
	return buildFixedFreqSummary(bands, order, indexFrequencies)
}

func buildMarvelGEQSummary(digest ParameterDigest) map[string]any {
	name := strings.ToLower(strings.TrimSpace(firstNonEmptyText(digest.PluginIdentity, "plugin_name")))
	if !strings.Contains(name, "marvel geq") {
		return nil
	}
	byID := map[string]ParameterInfo{}
	for _, param := range digest.Parameters {
		byID[strings.TrimSpace(param.ID)] = param
	}
	rows := make([]map[string]any, 0, len(marvelGEQFixedFrequenciesHz))
	for index, frequency := range marvelGEQFixedFrequenciesHz {
		paramID := ""
		for _, param := range digest.Parameters {
			if strings.EqualFold(strings.TrimSpace(param.Name), fmt.Sprintf("1EQ%d", index)) {
				paramID = strings.TrimSpace(param.ID)
				break
			}
		}
		param, ok := byID[paramID]
		if !ok || paramID == "" {
			return nil
		}
		row := map[string]any{
			"band":          fmt.Sprintf("B%d", index+1),
			"gain_param_id": paramID,
			"fixed_freq_hz": frequency,
		}
		if d := eqDomainFields(param.DisplayDomainCandidate); d != nil {
			row["gain_domain"] = d
		}
		if db, ok := parseDisplayDB(param.ValueText); ok {
			row["current_gain_db"] = db
		}
		rows = append(rows, row)
	}
	return map[string]any{
		"eq_model":           "fixed_freq",
		"band_count":         len(rows),
		"bands":              rows,
		"how_to_pick_a_band": "This is a fixed-frequency 16-band graphic EQ. Select the band whose fixed_freq_hz is closest to the target and write only its gain_param_id. Frequency is selection metadata, not a writable parameter; Q is not supported.",
	}
}

// eqBandsAreFreeFloating reports whether any band exposes a "Used" slot parked
// at an unused value, which is how create-on-demand EQs present empty slots.
func eqBandsAreFreeFloating(bands map[string]*eqBandEntry) bool {
	for _, entry := range bands {
		used, ok := entry.Slots[eqSlotUsed]
		if !ok {
			continue
		}
		if eqSlotIsUnused(used.ValueText) {
			return true
		}
	}
	return false
}

// eqBandsHaveAdjustableFreq reports whether any band owns a frequency slot.
//
// A frequency slot is only assigned when some control in that band has a
// frequency-shaped measured range, so its mere presence means the frequency can
// be repositioned. This deliberately no longer inspects Domain.Unit: that string
// is absent whenever a plugin neither declares getLabel() nor prints the unit in
// its value text, and requiring it made FreeEQ8 — a fully parametric 8-band EQ —
// report as fixed-frequency.
func eqBandsHaveAdjustableFreq(bands map[string]*eqBandEntry) bool {
	for _, entry := range bands {
		if _, ok := entry.Slots[eqSlotFreq]; ok {
			return true
		}
	}
	return false
}

func eqSlotIsUnused(valueText string) bool {
	switch strings.ToLower(strings.TrimSpace(valueText)) {
	case "unused", "off", "false", "0", "":
		return true
	}
	return false
}

// buildFixedSlotAdjustableSummary builds the summary for parametric EQs where
// each band slot can be repositioned anywhere in the frequency spectrum (e.g.
// TDR Nova). The correct professional operation is: select the band whose
// current frequency is CLOSEST to the target, then write BOTH freq_param_id
// (tuning it to the exact target Hz) and gain_param_id (target dB). This is
// standard parametric EQ usage — you're not randomly moving bands; you're
// tuning the nearest available band to precisely where you need it.
func buildFixedSlotAdjustableSummary(bands map[string]*eqBandEntry, order []string) map[string]any {
	rows := make([]map[string]any, 0, len(order))
	for _, number := range order {
		entry := bands[number]
		freq := entry.Slots[eqSlotFreq]
		if freq.ParamID == "" {
			continue
		}
		row := map[string]any{
			"band":          "B" + number,
			"freq_param_id": freq.ParamID,
		}
		if hz, ok := parseDisplayHz(freq.ValueText); ok {
			row["current_freq_hz"] = hz
		}
		if d := eqDomainFields(freq.Domain); d != nil {
			row["freq_domain"] = d
		}
		if len(freq.Curve) > 0 {
			row["freq_curve"] = freq.Curve
		}
		if gain := entry.Slots[eqSlotGain]; gain.ParamID != "" {
			row["gain_param_id"] = gain.ParamID
			if db, ok := parseDisplayDB(gain.ValueText); ok {
				row["current_gain_db"] = db
			}
			if d := eqDomainFields(gain.Domain); d != nil {
				row["gain_domain"] = d
			}
			if len(gain.Curve) > 0 {
				row["gain_curve"] = gain.Curve
			}
		}
		if q := entry.Slots[eqSlotQ]; q.ParamID != "" {
			row["q_param_id"] = q.ParamID
			if d := eqDomainFields(q.Domain); d != nil {
				row["q_domain"] = d
			}
			if len(q.Curve) > 0 {
				row["q_curve"] = q.Curve
			}
		}
		if shape := entry.Slots[eqSlotShape]; shape.ParamID != "" {
			row["shape_param_id"] = shape.ParamID
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	return map[string]any{
		"eq_model":   "fixed_slot_adjustable",
		"band_count": len(rows),
		"bands":      rows,
		"how_to_pick_a_band": "Parametric EQ. MANDATORY ALGORITHM — follow exactly: " +
			"(1) Extract target_hz and target_db from the user request. " +
			"(2) For EACH band, compute abs(target_hz - current_freq_hz). " +
			"(3) Select the band with the SMALLEST computed distance. " +
			"(4) Write that band's freq_param_id to normalized target_hz and its gain_param_id to normalized target_db. " +
			"WARNING: Band 1 is NOT automatically the right choice. " +
			"EXAMPLE: user wants 3400 Hz; bands are 69/400/2200/6000 Hz; distances are 3331/3000/1200/2600 → pick B3 (smallest=1200). " +
			"Do NOT ask clarification. Do NOT skip steps.",
	}
}

// buildFixedFreqSummary builds the summary for EQs where band frequencies are
// fixed and cannot be changed — only the gain at each fixed frequency can be
// adjusted (e.g. 30-band graphic EQ, classic SSL-style fixed-frequency EQ).
// buildFixedFreqSummary is only reached when no band owns a frequency slot, so
// every band here is a gain fader at a frequency the host cannot move.
//
// fixed_freq_hz is populated strictly from indexFrequencies — band labels that
// carry a real frequency, such as ZamGEQ31's "32Hz".."20000Hz". It must never be
// back-filled from a frequency parameter's current reading: doing that reported
// the position a movable band happened to be resting at as though it were fixed,
// which is what let a 3400 Hz request land on 4000 Hz and still return success.
//
// When indexFrequencies is empty the bands are structurally sound but
// unlabelled (Marvel GEQ's "1EQ0".."1EQ15"). Band selection by frequency is not
// possible from parameter data alone in that case, and eqWritePlanFixedFreq
// refuses rather than guessing.
func buildFixedFreqSummary(bands map[string]*eqBandEntry, order []string, indexFrequencies map[string]float64) map[string]any {
	rows := make([]map[string]any, 0, len(order))
	labelled := 0
	for _, number := range order {
		entry := bands[number]
		gain := entry.Slots[eqSlotGain]
		if gain.ParamID == "" {
			continue
		}
		row := map[string]any{
			"band":          "B" + number,
			"gain_param_id": gain.ParamID,
		}
		if db, ok := parseDisplayDB(gain.ValueText); ok {
			row["current_gain_db"] = db
		}
		if d := eqDomainFields(gain.Domain); d != nil {
			row["gain_domain"] = d
		}
		if len(gain.Curve) > 0 {
			row["gain_curve"] = gain.Curve
		}
		if hz, ok := indexFrequencies[number]; ok {
			row["fixed_freq_hz"] = hz
			labelled++
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	if labelled == 0 {
		return map[string]any{
			"eq_model":   "fixed_freq",
			"band_count": len(rows),
			"bands":      rows,
			"frequency_labels": "unknown — this plugin exposes its bands as opaque gain " +
				"controls and publishes no centre frequency for any of them. Band order is " +
				"almost certainly ascending in frequency, but the actual values are not " +
				"recoverable from parameter data.",
			"how_to_pick_a_band": "Frequency-targeted requests cannot be served: there is no " +
				"way to tell which band sits at a given Hz. Requests naming a band position " +
				"directly (\"the 5th band\") can be served by writing that band's gain_param_id.",
		}
	}
	return map[string]any{
		"eq_model":   "fixed_freq",
		"band_count": len(rows),
		"bands":      rows,
		"how_to_pick_a_band": "This EQ has fixed frequency bands — only the gain at each band can be changed. " +
			"Find the band whose fixed_freq_hz is CLOSEST to the target frequency. " +
			"Write ONLY gain_param_id. Do not write any frequency parameter.",
	}
}

func buildFreeFloatingSummary(bands map[string]*eqBandEntry, order []string) map[string]any {
	active := make([]map[string]any, 0, len(order))
	available := make([]map[string]any, 0, len(order))

	for _, number := range order {
		entry := bands[number]
		freq := entry.Slots[eqSlotFreq]
		gain := entry.Slots[eqSlotGain]
		used := entry.Slots[eqSlotUsed]
		if freq.ParamID == "" || used.ParamID == "" {
			continue
		}
		row := map[string]any{
			"band":          "B" + number,
			"used_param_id": used.ParamID,
			"freq_param_id": freq.ParamID,
		}
		if gain.ParamID != "" {
			row["gain_param_id"] = gain.ParamID
		}
		if shape := entry.Slots[eqSlotShape]; shape.ParamID != "" {
			row["shape_param_id"] = shape.ParamID
		}
		if q := entry.Slots[eqSlotQ]; q.ParamID != "" {
			row["q_param_id"] = q.ParamID
			if d := eqDomainFields(q.Domain); d != nil {
				row["q_domain"] = d
			}
			if len(q.Curve) > 0 {
				row["q_curve"] = q.Curve
			}
		}
		if d := eqDomainFields(freq.Domain); d != nil {
			row["freq_domain"] = d
		}
		if len(freq.Curve) > 0 {
			row["freq_curve"] = freq.Curve
		}
		if d := eqDomainFields(gain.Domain); d != nil {
			row["gain_domain"] = d
		}
		if len(gain.Curve) > 0 {
			row["gain_curve"] = gain.Curve
		}

		if eqSlotIsUnused(used.ValueText) {
			available = append(available, row)
			continue
		}
		if fv, ok := parseDisplayHz(freq.ValueText); ok {
			row["current_freq_hz"] = fv
		}
		if db, ok := parseDisplayDB(gain.ValueText); ok {
			row["current_gain_db"] = db
		}
		active = append(active, row)
	}

	if len(active) == 0 && len(available) == 0 {
		return nil
	}

	// The full slot list is long (Pro-Q 3 exposes 24); a handful is enough to
	// choose from and keeps the context pack small.
	shown := available
	if len(shown) > 4 {
		shown = shown[:4]
	}

	return map[string]any{
		"eq_model":              "free_floating",
		"active_band_count":     len(active),
		"available_slot_count":  len(available),
		"active_bands":          active,
		"available_slots":       shown,
		"how_to_pick_a_band":    "Bands are created on demand. If an active_bands entry is already near the requested frequency, just write its gain_param_id. Otherwise take the first available_slots entry and create the band.",
		"create_sequence_notes": "To create a band you must write used_param_id (to Used/On), freq_param_id (target Hz) and gain_param_id (target dB); set shape_param_id to Bell for an ordinary peaking move. Writing frequency and gain WITHOUT used_param_id is a silent failure: the parameters accept the values but the band stays inactive and nothing is audible.",
	}
}

// eqDomainFields extracts just unit/min/max/scale from a display domain,
// dropping the verbose Text/Status/Source/Confidence fields that are only
// useful for diagnostics and would bloat the context pack.
func eqDomainFields(d *PluginDisplayDomain) map[string]any {
	if d == nil {
		return nil
	}
	out := map[string]any{}
	if d.Unit != "" {
		out["unit"] = d.Unit
	}
	if d.Scale != "" {
		out["scale"] = d.Scale
	}
	if d.Min != nil {
		out["min"] = *d.Min
	}
	if d.Max != nil {
		out["max"] = *d.Max
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseDisplayHz reads a frequency display string ("80", "3400", "3.40 kHz",
// "15 Hz") into Hz.
func parseDisplayHz(valueText string) (float64, bool) {
	text := strings.ToLower(strings.TrimSpace(valueText))
	if text == "" {
		return 0, false
	}
	multiplier := 1.0
	switch {
	case strings.HasSuffix(text, "khz"):
		text = strings.TrimSpace(strings.TrimSuffix(text, "khz"))
		multiplier = 1000
	case strings.HasSuffix(text, "hz"):
		text = strings.TrimSpace(strings.TrimSuffix(text, "hz"))
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0, false
	}
	return value * multiplier, true
}

// parseDisplayDB reads a gain display string ("0.0", "-3.0", "+6.00 dB") into dB.
func parseDisplayDB(valueText string) (float64, bool) {
	text := strings.ToLower(strings.TrimSpace(valueText))
	if text == "" {
		return 0, false
	}
	text = strings.TrimSpace(strings.TrimSuffix(text, "db"))
	text = strings.TrimPrefix(text, "+")
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
