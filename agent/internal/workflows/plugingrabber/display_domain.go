package plugingrabber

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	displayDomainStatusConfirmed         = "confirmed"
	displayDomainStatusInferred          = "inferred"
	displayDomainStatusNeedsConfirmation = "needs_confirmation"
	displayDomainStatusUnknown           = "unknown"
)

var displayDomainRangePattern = regexp.MustCompile(`(?i)([-+]?\d+(?:\.\d+)?)\s*(?:db|hz|khz|%|ms|sec|s)?\s*(?:~|〜|～|至|到|to|\.{2,}|[-–—]+)\s*([-+]?\d+(?:\.\d+)?)`)

func parseDisplayDomainText(text, source string, confirmed bool) *PluginDisplayDomain {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	domain := &PluginDisplayDomain{
		Text:       text,
		Unit:       inferDisplayDomainUnit(text),
		Scale:      inferDisplayDomainScale(text),
		Source:     strings.TrimSpace(source),
		Confidence: 1.0,
	}
	if domain.Scale == "" {
		if strings.EqualFold(domain.Unit, "Hz") {
			domain.Scale = "log"
		} else {
			domain.Scale = "linear"
		}
	}
	if matches := displayDomainRangePattern.FindStringSubmatch(text); len(matches) == 3 {
		if minValue, err := strconv.ParseFloat(matches[1], 64); err == nil {
			domain.Min = &minValue
		}
		if maxValue, err := strconv.ParseFloat(matches[2], 64); err == nil {
			domain.Max = &maxValue
		}
		if domain.Min != nil && domain.Max != nil && *domain.Min > *domain.Max {
			*domain.Min, *domain.Max = *domain.Max, *domain.Min
		}
	}
	if domain.Unit == "" && strings.Contains(strings.ToLower(text), "toggle") {
		domain.Unit = "toggle"
	}
	if confirmed && domain.Min != nil && domain.Max != nil {
		domain.Status = displayDomainStatusConfirmed
	} else if confirmed && domain.Unit == "toggle" {
		domain.Status = displayDomainStatusConfirmed
	} else if confirmed {
		domain.Status = displayDomainStatusNeedsConfirmation
	} else {
		domain.Status = displayDomainStatusInferred
		domain.Confidence = 0.70
	}
	return domain
}

func ParseDisplayDomainText(text, source string, confirmed bool) *PluginDisplayDomain {
	return parseDisplayDomainText(text, source, confirmed)
}

func InferDisplayDomainForSlot(slot string, param ParameterInfo) *PluginDisplayDomain {
	return inferredDisplayDomainForSlot(slot, param)
}

func DisplayDomainText(domain *PluginDisplayDomain) string {
	return displayDomainTextForSummary(domain)
}

func inferDisplayDomainUnit(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case strings.Contains(lower, "db") || strings.Contains(text, "分贝"):
		return "dB"
	case strings.Contains(lower, "khz"):
		return "Hz"
	case strings.Contains(lower, "hz") || strings.Contains(text, "赫兹"):
		return "Hz"
	case strings.Contains(lower, "%") || strings.Contains(text, "百分比"):
		return "%"
	case strings.Contains(lower, "ms") || strings.Contains(text, "毫秒"):
		return "ms"
	case strings.Contains(lower, "sec") || strings.Contains(lower, "second") || strings.Contains(text, "秒"):
		return "s"
	}
	return ""
}

func inferDisplayDomainScale(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if strings.Contains(lower, "log") || strings.Contains(text, "对数") {
		return "log"
	}
	if strings.Contains(lower, "linear") || strings.Contains(text, "线性") {
		return "linear"
	}
	return ""
}

func displayDomainFromAny(value any) *PluginDisplayDomain {
	row := mapValue(value)
	if len(row) == 0 {
		return nil
	}
	domain := &PluginDisplayDomain{
		Text:       firstNonEmptyText(row, "text", "description", "display_domain_text"),
		Unit:       firstNonEmptyText(row, "unit"),
		Scale:      firstNonEmptyText(row, "scale", "curve"),
		Status:     firstNonEmptyText(row, "status"),
		Source:     firstNonEmptyText(row, "source"),
		Confidence: floatNumber(row["confidence"]),
	}
	if minValue := floatNumber(row["min"]); minValue != 0 || row["min"] != nil {
		domain.Min = &minValue
	}
	if maxValue := floatNumber(row["max"]); maxValue != 0 || row["max"] != nil {
		domain.Max = &maxValue
	}
	if domain.Text == "" && domain.Unit == "" && domain.Min == nil && domain.Max == nil {
		return nil
	}
	if domain.Scale == "" {
		domain.Scale = "linear"
	}
	if domain.Status == "" {
		domain.Status = displayDomainStatusUnknown
	}
	return domain
}

func displayDomainMap(domain *PluginDisplayDomain) map[string]any {
	if domain == nil {
		return nil
	}
	out := map[string]any{}
	if strings.TrimSpace(domain.Text) != "" {
		out["text"] = strings.TrimSpace(domain.Text)
	}
	if strings.TrimSpace(domain.Unit) != "" {
		out["unit"] = strings.TrimSpace(domain.Unit)
	}
	if domain.Min != nil {
		out["min"] = *domain.Min
	}
	if domain.Max != nil {
		out["max"] = *domain.Max
	}
	if strings.TrimSpace(domain.Scale) != "" {
		out["scale"] = strings.TrimSpace(domain.Scale)
	}
	if strings.TrimSpace(domain.Status) != "" {
		out["status"] = strings.TrimSpace(domain.Status)
	}
	if strings.TrimSpace(domain.Source) != "" {
		out["source"] = strings.TrimSpace(domain.Source)
	}
	if domain.Confidence > 0 {
		out["confidence"] = domain.Confidence
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func inferredDisplayDomainForSlot(slot string, param ParameterInfo) *PluginDisplayDomain {
	if param.DisplayDomainCandidate != nil && param.DisplayDomainCandidate.Confidence >= 0.80 {
		return param.DisplayDomainCandidate
	}
	cleanSlot := pluginSkillParamSlot(slot)
	text := strings.ToLower(strings.Join([]string{
		cleanSlot,
		param.ID,
		param.Name,
		param.RawName,
		param.Alias,
		param.Unit,
		param.ValueText,
	}, " "))
	switch {
	case param.IsBoolean || cleanSlot == "enable" || strings.Contains(text, "enable") || strings.Contains(text, "bypass"):
		return displayDomain("0~1 toggle", "toggle", 0, 1, "enum", displayDomainStatusInferred, "auto_learn_parameter_shape", 0.85)
	case strings.Contains(text, "low cut") || strings.Contains(text, "lowcut") || strings.Contains(text, "high pass") || strings.Contains(text, "hipass"):
		return displayDomain("10~2000 Hz 对数", "Hz", 10, 2000, "log", displayDomainStatusInferred, "auto_learn_name_pattern", 0.74)
	case strings.Contains(text, "high cut") || strings.Contains(text, "highcut") || strings.Contains(text, "low pass") || strings.Contains(text, "lopass"):
		return displayDomain("200~20000 Hz 对数", "Hz", 200, 20000, "log", displayDomainStatusInferred, "auto_learn_name_pattern", 0.74)
	case cleanSlot == "frequency" || cleanSlot == "freq" || strings.Contains(text, "frequency") || strings.Contains(text, "freq"):
		return displayDomain("20~20000 Hz 对数", "Hz", 20, 20000, "log", displayDomainStatusInferred, "auto_learn_eq_slot", 0.78)
	case cleanSlot == "q" || strings.Contains(text, "quality") || strings.Contains(text, "bandwidth"):
		return displayDomain("0.1~10 Q", "Q", 0.1, 10, "log", displayDomainStatusInferred, "auto_learn_eq_slot", 0.70)
	case cleanSlot == "time" || cleanSlot == "delay" || strings.Contains(text, "delay") || strings.Contains(text, "time_delay") || strings.Contains(strings.ToLower(param.ValueText), "ms"):
		return displayDomain("0~2000 ms", "ms", 0, 2000, "linear", displayDomainStatusInferred, "auto_learn_name_pattern", 0.72)
	case strings.Contains(text, "mod rate") || strings.Contains(text, "rate") || strings.Contains(text, "lfo") || strings.Contains(strings.ToLower(param.ValueText), "hz"):
		return displayDomain("0.01~10 Hz 对数", "Hz", 0.01, 10, "log", displayDomainStatusInferred, "auto_learn_name_pattern", 0.68)
	case strings.Contains(text, "mix") || strings.Contains(text, "wet") || strings.Contains(text, "dry"):
		return displayDomain("0~100 %", "%", 0, 100, "linear", displayDomainStatusInferred, "auto_learn_name_pattern", 0.72)
	case strings.Contains(text, "feedback") || strings.Contains(text, "depth") || strings.Contains(text, "width") || strings.Contains(text, "density") || strings.Contains(text, "warp") || strings.Contains(text, "amount") || strings.Contains(strings.ToLower(param.ValueText), "%"):
		return displayDomain("0~100 %", "%", 0, 100, "linear", displayDomainStatusInferred, "auto_learn_name_pattern", 0.70)
	case cleanSlot == "gain" || strings.Contains(text, "gain") || strings.Contains(text, "level") || strings.Contains(text, "output"):
		return displayDomain("-18~18 dB", "dB", -18, 18, "linear", displayDomainStatusInferred, "auto_learn_name_pattern", 0.66)
	}
	if param.DisplayDomainCandidate != nil {
		return param.DisplayDomainCandidate
	}
	return &PluginDisplayDomain{
		Status:     displayDomainStatusUnknown,
		Source:     "auto_learn_unknown",
		Confidence: 0.0,
	}
}

func displayDomain(text, unit string, minValue, maxValue float64, scale, status, source string, confidence float64) *PluginDisplayDomain {
	return &PluginDisplayDomain{
		Text:       text,
		Unit:       unit,
		Min:        &minValue,
		Max:        &maxValue,
		Scale:      scale,
		Status:     status,
		Source:     source,
		Confidence: confidence,
	}
}

func displayDomainSummary(groups []map[string]any) map[string]int {
	out := map[string]int{
		displayDomainStatusConfirmed:         0,
		displayDomainStatusInferred:          0,
		displayDomainStatusNeedsConfirmation: 0,
		displayDomainStatusUnknown:           0,
	}
	for _, mapping := range profileMappingsFromGroups(groups) {
		domain := displayDomainFromAny(mapping["display_domain"])
		status := displayDomainStatusUnknown
		if domain != nil && strings.TrimSpace(domain.Status) != "" {
			status = strings.TrimSpace(domain.Status)
		}
		if _, ok := out[status]; !ok {
			out[status] = 0
		}
		out[status]++
	}
	return out
}

func displayDomainTextForSummary(domain *PluginDisplayDomain) string {
	if domain == nil {
		return ""
	}
	if strings.TrimSpace(domain.Text) != "" {
		return strings.TrimSpace(domain.Text)
	}
	if domain.Min != nil && domain.Max != nil && strings.TrimSpace(domain.Unit) != "" {
		return fmt.Sprintf("%g~%g %s", *domain.Min, *domain.Max, strings.TrimSpace(domain.Unit))
	}
	return strings.TrimSpace(domain.Unit)
}
