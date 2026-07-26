package plugingrabber

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// EQ band structure detection.
//
// This replaces two heuristics that both keyed on developer-chosen strings:
//
//	grouping  used to require a literal "Band " prefix, so ZamEQ2
//	          ("Frequency 1" / "Boost/Cut L") and ZamGEQ31 ("32Hz") produced no
//	          bands at all.
//	roles     used to require DisplayDomain.Unit == "Hz" to call a frequency
//	          adjustable. That unit string only exists when the plugin either
//	          declares getLabel() or prints the unit inside its value text.
//	          Measured: TDR Nova declares "Hz"; Pro-Q 3 declares nothing but
//	          prints "10.000 Hz"; FreeEQ8 does neither (0 of 129 parameters
//	          carry a label, and its value text is a bare "80.000"). FreeEQ8 was
//	          therefore classified fixed_freq, and a request for 3400 Hz moved
//	          only the gain of the band sitting at 4000 Hz — reported as success.
//
// Grouping is now structural: a band family is a set of parameter-name
// templates that share one varying index token. Roles are decided from the
// shape of the probe-measured value range, which is a property of the audio
// control rather than of the plugin's metadata hygiene.
//
// Names have not disappeared entirely, and pretending otherwise would be
// dishonest about the coverage: they still supply the grouping key, and they
// break ties between same-shaped candidates (Pro-Q 3 exposes "Band 1 Gain" and
// "Band 1 Dynamic Range" with an identical -30..+30 range; FreeEQ8 exposes
// "Band 1 Q" and "Band 1 Attack" both starting at 0.1). The load-bearing change
// is that a *missing* unit string no longer alters any decision.

const (
	// A frequency control has to reach at least this far up the spectrum. The
	// lowest real maximum measured across the fixture set is ZamEQ2's
	// "Frequency L" at 600 Hz.
	eqFreqPlausibleMax = 200.0

	// Q conventionally starts below 1 (measured lows: 0.025 Pro-Q 3, 0.1 TDR
	// Nova and FreeEQ8, 0.7 ZamEQ2 "Bandwidth"). The bound excludes FreeEQ8's
	// "Band 1 Slope" (12..48), which is otherwise Q-shaped.
	eqQPlausibleLow  = 10.0
	eqQPlausibleHigh = 100.0

	// A gain control straddles zero with usable travel on both sides.
	eqGainMinMagnitude = 3.0
	eqGainMaxAsymmetry = 4.0

	// Bands whose index token is itself the centre frequency (ZamGEQ31's
	// "32Hz".."20000Hz") must land inside the audible band to be believed.
	eqIndexFreqMinHz = 15.0
	eqIndexFreqMaxHz = 25000.0
	eqIndexFreqMin   = 5
)

// eqParamShape is the probe-measured value range of one parameter.
type eqParamShape struct {
	Lo       float64
	Hi       float64
	HasRange bool
}

// eqShapeOfParam reads the range the display probe actually observed by
// rendering the parameter at five normalised positions. Enum and toggle domains
// carry a synthetic 0..1 range and are reported as having no usable range.
func eqShapeOfParam(param ParameterInfo) eqParamShape {
	domain := param.DisplayDomainCandidate
	if domain == nil || domain.Min == nil || domain.Max == nil {
		return eqParamShape{}
	}
	if strings.EqualFold(strings.TrimSpace(domain.Scale), "enum") {
		return eqParamShape{}
	}
	lo, hi := *domain.Min, *domain.Max
	if hi <= lo {
		return eqParamShape{}
	}
	return eqParamShape{Lo: lo, Hi: hi, HasRange: true}
}

// looksLikeGain reports a range straddling zero with comparable travel each way.
// Measured hits: ±18 (TDR Nova), ±24 (FreeEQ8), ±30 (Pro-Q 3), ±20 (ZamEQ2
// "Boost/Cut"), ±12 (ZamGEQ31, Marvel GEQ). Note that two of those are named
// nothing like "gain" — which is the point of not consulting the name.
func (s eqParamShape) looksLikeGain() bool {
	if !s.HasRange || s.Lo >= 0 || s.Hi <= 0 {
		return false
	}
	negative, positive := -s.Lo, s.Hi
	if negative < eqGainMinMagnitude || positive < eqGainMinMagnitude {
		return false
	}
	asymmetry := negative / positive
	if asymmetry < 1 {
		asymmetry = 1 / asymmetry
	}
	return asymmetry <= eqGainMaxAsymmetry
}

// looksLikeFreq is deliberately permissive: it only screens candidates, and the
// winner is chosen by comparing maxima within the band. An absolute rule cannot
// work here — ZamEQ2's "Frequency L" (40..600) and FreeEQ8's "Band 1 Release"
// (1..1000) are not separable by range alone, but the release loses because its
// band also contains a control reaching 20000.
func (s eqParamShape) looksLikeFreq() bool {
	return s.HasRange && s.Lo > 0 && s.Hi >= eqFreqPlausibleMax
}

func (s eqParamShape) looksLikeQ() bool {
	return s.HasRange && s.Lo > 0 && s.Lo < eqQPlausibleLow && s.Hi <= eqQPlausibleHigh
}

// eqNameHint maps the slot text of a parameter template to a slot, and is only
// ever consulted to break a tie between candidates that already passed the
// range-shape screen, or to pick up the non-numeric slots (used, shape) whose
// domains are enums and therefore carry no range at all.
func eqNameHint(slotText string) string {
	text := strings.ToLower(strings.TrimSpace(slotText))
	if text == "" {
		return ""
	}
	fields := strings.Fields(text)
	hasField := func(want string) bool {
		for _, field := range fields {
			if strings.Trim(field, "()[]-_/") == want {
				return true
			}
		}
		return false
	}
	switch {
	case strings.Contains(text, "freq") || strings.Contains(text, "cutoff"):
		return eqSlotFreq
	case strings.Contains(text, "bandwidth") || strings.Contains(text, "width") || hasField("q"):
		return eqSlotQ
	case strings.Contains(text, "used") || strings.Contains(text, "in use"):
		return eqSlotUsed
	case strings.Contains(text, "shape") || strings.Contains(text, "type"):
		return eqSlotShape
	case strings.Contains(text, "gain") || strings.Contains(text, "boost") ||
		strings.Contains(text, "cut") || strings.Contains(text, "level") ||
		strings.Contains(text, "trim"):
		return eqSlotGain
	}
	return ""
}

// ---- structural grouping ----

const eqIndexPlaceholder = "\x00"

var eqDigitRunPattern = regexp.MustCompile(`\d+`)

// eqDecomposition splits a parameter name into a template plus the index token
// that identifies which band it belongs to.
type eqDecomposition struct {
	Template string
	Index    string
}

// eqDecompositionsFor enumerates every way a name could be read as
// "template with one varying index". A name usually yields one reading, but
// Marvel GEQ's "1EQ0" yields two ("<n>EQ0" and "1EQ<n>"); the caller picks the
// reading whose template is shared by the most siblings.
func eqDecompositionsFor(name string) []eqDecomposition {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	out := make([]eqDecomposition, 0, 3)
	for _, span := range eqDigitRunPattern.FindAllStringIndex(name, -1) {
		out = append(out, eqDecomposition{
			Template: name[:span[0]] + eqIndexPlaceholder + name[span[1]:],
			Index:    name[span[0]:span[1]],
		})
	}
	// Console-style and shelf-style EQs index bands by letter rather than
	// number: ZamEQ2 exposes "Boost/Cut 1", "Boost/Cut 2", "Boost/Cut L" and
	// "Boost/Cut H" as one family, so the letter reading has to unify with the
	// numeric one.
	if fields := strings.Fields(name); len(fields) >= 2 {
		last := fields[len(fields)-1]
		if len(last) == 1 && isASCIILetter(last[0]) {
			out = append(out, eqDecomposition{
				Template: strings.Join(fields[:len(fields)-1], " ") + " " + eqIndexPlaceholder,
				Index:    strings.ToUpper(last),
			})
		}
	}
	return out
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func eqSlotTextForTemplate(template string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(template, eqIndexPlaceholder, " ")), " ")
}

func eqParamDisplayName(param ParameterInfo) string {
	for _, candidate := range []string{param.Name, param.Alias, param.RawName} {
		if text := strings.TrimSpace(candidate); text != "" {
			return text
		}
	}
	return ""
}

// eqBandFamily is one detected set of bands: which templates act as slots, and
// which index tokens act as bands.
type eqBandFamily struct {
	Templates []string
	Indices   []string
	Members   map[string]map[string]ParameterInfo // index -> template -> param
}

// detectEQBandFamily finds the dominant band family in a parameter list.
//
// A template only counts as a slot if at least two bands share it, which is
// what keeps single-instance controls out: Marvel GEQ's "1OGain" has the exact
// same dB range as its sixteen band gains and can only be told apart by the
// fact that it does not belong to an index series.
func detectEQBandFamily(params []ParameterInfo) *eqBandFamily {
	templateIndices := map[string]map[string]bool{}
	perParam := map[int][]eqDecomposition{}

	for i, param := range params {
		name := eqParamDisplayName(param)
		decompositions := eqDecompositionsFor(name)
		if len(decompositions) == 0 {
			continue
		}
		perParam[i] = decompositions
		for _, decomposition := range decompositions {
			if templateIndices[decomposition.Template] == nil {
				templateIndices[decomposition.Template] = map[string]bool{}
			}
			templateIndices[decomposition.Template][decomposition.Index] = true
		}
	}

	// Each parameter commits to a single reading — the one whose template has
	// the widest index series — so that a rejected reading cannot also
	// contribute a phantom slot.
	winners := map[string]map[string]ParameterInfo{}
	for i, decompositions := range perParam {
		best := eqDecomposition{}
		bestCount := 0
		for _, decomposition := range decompositions {
			count := len(templateIndices[decomposition.Template])
			if count > bestCount ||
				(count == bestCount && len(decomposition.Template) > len(best.Template)) {
				best, bestCount = decomposition, count
			}
		}
		if bestCount < 2 {
			continue
		}
		if winners[best.Template] == nil {
			winners[best.Template] = map[string]ParameterInfo{}
		}
		winners[best.Template][best.Index] = params[i]
	}
	if len(winners) == 0 {
		return nil
	}

	anchor := ""
	for template, byIndex := range winners {
		if anchor == "" || len(byIndex) > len(winners[anchor]) ||
			(len(byIndex) == len(winners[anchor]) && template < anchor) {
			anchor = template
		}
	}
	anchorIndices := winners[anchor]
	if len(anchorIndices) < 2 {
		return nil
	}

	family := &eqBandFamily{Members: map[string]map[string]ParameterInfo{}}
	for template, byIndex := range winners {
		if template != anchor && !eqIndexSetsRelated(byIndex, anchorIndices) {
			continue
		}
		family.Templates = append(family.Templates, template)
		for index, param := range byIndex {
			if family.Members[index] == nil {
				family.Members[index] = map[string]ParameterInfo{}
			}
			family.Members[index][template] = param
		}
	}
	for index := range family.Members {
		family.Indices = append(family.Indices, index)
	}
	sort.Strings(family.Templates)
	eqSortBandIndices(family.Indices)
	return family
}

// eqIndexSetsRelated keeps a template in the family when its bands line up with
// the anchor's. ZamEQ2's "Bandwidth <n>" covers only {1,2} while "Frequency <n>"
// covers {1,2,L,H}, because its shelves have no width control — a subset must
// still join. An unrelated indexed series (a modulation matrix, say) will not.
func eqIndexSetsRelated(candidate, anchor map[string]ParameterInfo) bool {
	if len(candidate) == 0 {
		return false
	}
	shared := 0
	for index := range candidate {
		if _, ok := anchor[index]; ok {
			shared++
		}
	}
	return shared*2 >= len(candidate)
}

func eqSortBandIndices(indices []string) {
	sort.SliceStable(indices, func(i, j int) bool {
		left, leftErr := strconv.ParseFloat(indices[i], 64)
		right, rightErr := strconv.ParseFloat(indices[j], 64)
		switch {
		case leftErr == nil && rightErr == nil:
			return left < right
		case leftErr == nil:
			return true
		case rightErr == nil:
			return false
		default:
			return indices[i] < indices[j]
		}
	})
}

// ---- slot assignment ----

type eqSlotCandidate struct {
	SlotText string
	Param    ParameterInfo
	Shape    eqParamShape
	Hint     string
}

// assignEQSlotsForBand decides which parameter plays which role inside one band.
//
// Frequency and gain — the two roles a wrong answer would silently corrupt — are
// decided by range shape. Q additionally requires name agreement, because
// FreeEQ8 exposes "Band 1 Q" (0.1..24) and "Band 1 Attack" (0.1..100) with
// indistinguishable shapes; Q is the optional slot, so demanding agreement
// costs a skipped write rather than a wrong one.
func assignEQSlotsForBand(candidates []eqSlotCandidate) map[string]eqBandSlot {
	slots := map[string]eqBandSlot{}
	claimed := map[string]bool{}

	put := func(slot string, candidate eqSlotCandidate) {
		slots[slot] = eqBandSlot{
			ParamID:   candidate.Param.ID,
			ValueText: candidate.Param.ValueText,
			Domain:    candidate.Param.DisplayDomainCandidate,
			Curve:     eqCurveFromProbe(candidate.Param),
		}
		claimed[candidate.Param.ID] = true
	}

	// Frequency: the highest-reaching positive control in the band.
	best, found := eqSlotCandidate{}, false
	for _, candidate := range candidates {
		if !candidate.Shape.looksLikeFreq() {
			continue
		}
		if !found || candidate.Shape.Hi > best.Shape.Hi ||
			(candidate.Shape.Hi == best.Shape.Hi && candidate.Hint == eqSlotFreq && best.Hint != eqSlotFreq) {
			best, found = candidate, true
		}
	}
	if found {
		put(eqSlotFreq, best)
	}

	// Gain: straddles zero. Name agreement only breaks ties (Pro-Q 3's
	// "Gain" vs "Dynamic Range", both -30..+30).
	best, found = eqSlotCandidate{}, false
	for _, candidate := range candidates {
		if claimed[candidate.Param.ID] || !candidate.Shape.looksLikeGain() {
			continue
		}
		if !found || (candidate.Hint == eqSlotGain && best.Hint != eqSlotGain) {
			best, found = candidate, true
		}
	}
	if found {
		put(eqSlotGain, best)
	}

	// Q / bandwidth: shape plus name agreement.
	for _, candidate := range candidates {
		if claimed[candidate.Param.ID] || candidate.Hint != eqSlotQ || !candidate.Shape.looksLikeQ() {
			continue
		}
		put(eqSlotQ, candidate)
		break
	}

	// Used and shape are enum or boolean controls with no numeric range, so the
	// slot text is the only signal available for them.
	for _, slot := range []string{eqSlotUsed, eqSlotShape} {
		for _, candidate := range candidates {
			if claimed[candidate.Param.ID] || candidate.Hint != slot {
				continue
			}
			put(slot, candidate)
			break
		}
	}

	// A graphic-EQ band is a single dB fader whose template text names a
	// frequency ("32Hz") or nothing at all ("1EQ<n>"), so no slot text can
	// identify it. The range still can.
	if _, ok := slots[eqSlotGain]; !ok && len(candidates) == 1 && candidates[0].Shape.looksLikeGain() {
		put(eqSlotGain, candidates[0])
	}
	return slots
}

// collectEQBandsStructural groups parameters into bands and assigns roles.
// The third return value maps band key to centre frequency for graphic EQs whose
// index token *is* the frequency (ZamGEQ31), and is nil otherwise.
func collectEQBandsStructural(digest ParameterDigest) (map[string]*eqBandEntry, []string, map[string]float64) {
	family := detectEQBandFamily(digest.Parameters)
	if family == nil {
		return nil, nil, nil
	}

	bands := map[string]*eqBandEntry{}
	order := make([]string, 0, len(family.Indices))
	for _, index := range family.Indices {
		members := family.Members[index]
		candidates := make([]eqSlotCandidate, 0, len(members))
		for _, template := range family.Templates {
			param, ok := members[template]
			if !ok {
				continue
			}
			slotText := eqSlotTextForTemplate(template)
			candidates = append(candidates, eqSlotCandidate{
				SlotText: slotText,
				Param:    param,
				Shape:    eqShapeOfParam(param),
				Hint:     eqNameHint(slotText),
			})
		}
		slots := assignEQSlotsForBand(candidates)
		if len(slots) == 0 {
			continue
		}
		bands[index] = &eqBandEntry{Slots: slots}
		order = append(order, index)
	}
	if len(bands) == 0 {
		return nil, nil, nil
	}
	return bands, order, eqIndexDerivedFrequencies(bands, order)
}

// eqIndexDerivedFrequencies recovers the centre frequencies of a graphic EQ
// whose band index is the frequency itself. ZamGEQ31 names its 31 faders
// "32Hz".."20000Hz"; the frequency is a label, never a writable parameter, so it
// has to come from the name. Marvel GEQ is excluded here on purpose — its
// indices are 0..15, which carry no frequency information at all, and that gap
// cannot be closed from parameter data.
func eqIndexDerivedFrequencies(bands map[string]*eqBandEntry, order []string) map[string]float64 {
	if len(order) < eqIndexFreqMin {
		return nil
	}
	frequencies := make(map[string]float64, len(order))
	previous := 0.0
	for _, index := range order {
		entry := bands[index]
		if _, hasFreq := entry.Slots[eqSlotFreq]; hasFreq {
			return nil
		}
		if _, hasGain := entry.Slots[eqSlotGain]; !hasGain {
			return nil
		}
		value, err := strconv.ParseFloat(index, 64)
		if err != nil || value < eqIndexFreqMinHz || value > eqIndexFreqMaxHz || value <= previous {
			return nil
		}
		frequencies[index] = value
		previous = value
	}
	return frequencies
}

// eqCurveFromProbe returns the parameter's measured response as (normalised,
// displayed value) pairs.
//
// Plugins do not agree on the curve they map a normalised value through, and the
// declared unit says nothing about it: TDR Nova's frequency is logarithmic
// (10 / 80 / 632 / 5000 / 40000 — a constant ×8 per step), FreeEQ8's is
// quadratic (20 / 1268.75 / 5015 / 11258.75 / 20000, i.e. n²), and ZamEQ2's is
// plain linear (600 / 2200 / 3800 / 5400 / 7000, +1600 per step) despite
// declaring "Hz". Collapsing that to a log-or-linear flag and then applying a
// formula put a 3400 Hz request at 11064 Hz on FreeEQ8 and 5119 Hz on ZamEQ2.
//
// Returns nil unless the samples are strictly increasing on both axes, which is
// what makes the response invertible.
func eqCurveFromProbe(param ParameterInfo) [][2]float64 {
	probe := param.DisplayProbe
	if probe == nil || len(probe.Samples) < 3 {
		return nil
	}
	curve := make([][2]float64, 0, len(probe.Samples))
	for _, sample := range probe.Samples {
		value, _, ok := parseProbeDisplayNumber(sample.Text, probe.Label)
		if !ok {
			return nil
		}
		curve = append(curve, [2]float64{sample.NormalizedValue, value})
	}
	for i := 1; i < len(curve); i++ {
		if curve[i][0] <= curve[i-1][0] || curve[i][1] <= curve[i-1][1] {
			return nil
		}
	}
	return curve
}

// eqBandsLookStructural reports whether the detected family behaves like an EQ:
// at least two bands carrying both a frequency-shaped and a gain-shaped control,
// or a graphic-EQ layout whose frequencies came from the index tokens.
func eqBandsLookStructural(bands map[string]*eqBandEntry, indexFrequencies map[string]float64) bool {
	if len(indexFrequencies) >= eqIndexFreqMin {
		return true
	}
	complete := 0
	for _, entry := range bands {
		_, hasFreq := entry.Slots[eqSlotFreq]
		_, hasGain := entry.Slots[eqSlotGain]
		if hasFreq && hasGain {
			complete++
		}
	}
	return complete >= 2
}
