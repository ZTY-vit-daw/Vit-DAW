package plugingrabber

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	displayProbeSourceRaw      = "plugin_parameter_display_probe"
	displayProbeSourceInferred = "display_probe_inferred"
)

var displayProbeNumberPattern = regexp.MustCompile(`[-+]?\d+(?:\.\d+)?`)

func displayProbeFromAny(value any) *ParameterDisplayProbe {
	row := mapValue(value)
	if len(row) == 0 {
		return nil
	}
	probe := &ParameterDisplayProbe{
		Mode:         firstNonEmptyText(row, "mode"),
		CurrentText:  firstNonEmptyText(row, "current_text"),
		Label:        firstNonEmptyText(row, "label"),
		AllLabels:    stringSliceValue(row["all_labels"]),
		Capabilities: stringSliceValue(row["capabilities"]),
		Issues:       stringSliceValue(row["issues"]),
	}
	for _, sampleRow := range mapRowsValue(row["samples"]) {
		probe.Samples = append(probe.Samples, ParameterDisplayProbeSample{
			NormalizedValue: floatNumber(sampleRow["normalized_value"]),
			Value:           sampleRow["value"],
			Text:            firstNonEmptyText(sampleRow, "text"),
		})
	}
	for _, labelRow := range mapRowsValue(row["discrete_labels"]) {
		probe.DiscreteLabels = append(probe.DiscreteLabels, ParameterDisplayProbeLabel{
			Index: intNumber(labelRow["index"]),
			Value: labelRow["value"],
			Label: firstNonEmptyText(labelRow, "label"),
		})
	}
	if probe.Mode == "" && probe.CurrentText == "" && probe.Label == "" && len(probe.Samples) == 0 && len(probe.DiscreteLabels) == 0 {
		return nil
	}
	return probe
}

func DisplayDomainCandidateForParameter(param ParameterInfo) *PluginDisplayDomain {
	if param.DisplayProbe == nil {
		return nil
	}
	if domain := displayDomainFromDiscreteProbe(param); domain != nil {
		return domain
	}
	return displayDomainFromProbeSamples(param)
}

func DisplayProbeSummary(digest ParameterDigest) map[string]any {
	out := map[string]any{
		"parameter_count":                 digest.ParameterCount,
		"parameters_with_display_probe":   0,
		"display_domain_candidate_count":  0,
		"high_confidence_candidate_count": 0,
		"probe_issue_count":               0,
	}
	units := map[string]int{}
	for _, param := range digest.Parameters {
		if param.DisplayProbe != nil {
			out["parameters_with_display_probe"] = intNumber(out["parameters_with_display_probe"]) + 1
			out["probe_issue_count"] = intNumber(out["probe_issue_count"]) + len(param.DisplayProbe.Issues)
		}
		if param.DisplayDomainCandidate != nil {
			out["display_domain_candidate_count"] = intNumber(out["display_domain_candidate_count"]) + 1
			if param.DisplayDomainCandidate.Confidence >= 0.80 {
				out["high_confidence_candidate_count"] = intNumber(out["high_confidence_candidate_count"]) + 1
			}
			if unit := strings.TrimSpace(param.DisplayDomainCandidate.Unit); unit != "" {
				units[unit]++
			}
		}
	}
	if len(units) > 0 {
		out["units"] = units
	}
	return out
}

func displayDomainFromDiscreteProbe(param ParameterInfo) *PluginDisplayDomain {
	probe := param.DisplayProbe
	if probe == nil {
		return nil
	}
	labels := compactProbeLabels(probe)
	if len(labels) == 0 {
		return nil
	}
	if param.IsBoolean || len(labels) == 2 && looksLikeToggleLabels(labels) {
		return displayDomain("0~1 toggle", "toggle", 0, 1, "enum", displayDomainStatusInferred, displayProbeSourceInferred, 0.92)
	}
	text := "enum: " + strings.Join(firstStringLimit(labels, 8), " / ")
	return &PluginDisplayDomain{
		Text:       text,
		Unit:       "enum",
		Scale:      "enum",
		Status:     displayDomainStatusInferred,
		Source:     displayProbeSourceInferred,
		Confidence: 0.88,
	}
}

func displayDomainFromProbeSamples(param ParameterInfo) *PluginDisplayDomain {
	probe := param.DisplayProbe
	if probe == nil {
		return nil
	}
	values := make([]float64, 0, len(probe.Samples))
	units := map[string]bool{}
	for _, sample := range probe.Samples {
		value, unit, ok := parseProbeDisplayNumber(sample.Text, probe.Label)
		if !ok {
			continue
		}
		values = append(values, value)
		if unit != "" {
			units[unit] = true
		}
	}
	if len(values) < 2 {
		return nil
	}
	minValue, maxValue := values[0], values[0]
	for _, value := range values[1:] {
		minValue = math.Min(minValue, value)
		maxValue = math.Max(maxValue, value)
	}
	if nearlyEqual(minValue, maxValue) {
		return nil
	}
	unit := ""
	for candidate := range units {
		unit = candidate
		break
	}
	confidence := 0.84
	if unit != "" {
		confidence = 0.90
	}
	if len(units) > 1 {
		confidence = 0.66
	}
	if len(probe.Issues) > 0 && unit == "" {
		confidence -= 0.08
	}
	if confidence < 0.50 {
		confidence = 0.50
	}
	scale := inferProbeScale(unit, minValue, values)
	domain := displayDomain(displayDomainRangeText(minValue, maxValue, unit), unit, minValue, maxValue, scale, displayDomainStatusInferred, displayProbeSourceInferred, confidence)
	return domain
}

func parseProbeDisplayNumber(text, label string) (float64, string, bool) {
	text = strings.TrimSpace(text)
	match := displayProbeNumberPattern.FindString(text)
	if match == "" {
		return 0, "", false
	}
	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, "", false
	}
	unitSource := text
	unit := inferDisplayDomainUnit(unitSource)
	lower := strings.ToLower(unitSource)
	if strings.Contains(lower, "khz") {
		unit = "Hz"
		value *= 1000
	}
	if unit == "" {
		unit = inferDisplayDomainUnit(label)
	}
	return value, unit, true
}

func compactProbeLabels(probe *ParameterDisplayProbe) []string {
	if probe == nil {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	add := func(label string) {
		label = strings.TrimSpace(label)
		if label == "" || seen[label] {
			return
		}
		seen[label] = true
		out = append(out, label)
	}
	for _, label := range probe.AllLabels {
		add(label)
	}
	for _, row := range probe.DiscreteLabels {
		add(row.Label)
	}
	return out
}

func looksLikeToggleLabels(labels []string) bool {
	if len(labels) != 2 {
		return false
	}
	joined := strings.ToLower(strings.Join(labels, " "))
	return strings.Contains(joined, "off") && strings.Contains(joined, "on")
}

func inferProbeScale(unit string, minValue float64, values []float64) string {
	if strings.EqualFold(unit, "Hz") && minValue > 0 {
		return "log"
	}
	if len(values) >= 5 && values[0] > 0 && values[len(values)-1] > values[0] {
		firstStep := values[1] - values[0]
		lastStep := values[len(values)-1] - values[len(values)-2]
		if firstStep > 0 && lastStep > firstStep*2.0 {
			return "log"
		}
	}
	return "linear"
}

func displayDomainRangeText(minValue, maxValue float64, unit string) string {
	if strings.TrimSpace(unit) == "" {
		return fmt.Sprintf("%g~%g", minValue, maxValue)
	}
	return fmt.Sprintf("%g~%g %s", minValue, maxValue, strings.TrimSpace(unit))
}

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.000001
}
