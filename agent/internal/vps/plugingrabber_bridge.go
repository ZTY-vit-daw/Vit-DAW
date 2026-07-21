package vps

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

// BuildPluginFingerprintFromDigest is the explicit bridge from the current
// Plugin Grabber observation model to the stronger VPS v3 fingerprint. It
// fails closed when the current digest cannot prove a finite host range and a
// host-visible display surface. Semantic display domains are preferred, while
// non-capability controls may use a raw host-display descriptor so an opaque
// UI-state parameter cannot block qualification of an otherwise verified
// capability mapping.
func BuildPluginFingerprintFromDigest(installationFingerprint string, digest plugingrabber.ParameterDigest) (PluginFingerprint, error) {
	descriptors, err := ParameterDescriptorsFromPluginDigest(digest)
	if err != nil {
		return PluginFingerprint{}, err
	}
	return BuildPluginFingerprint(installationFingerprint, descriptors)
}

func ParameterDescriptorsFromPluginDigest(digest plugingrabber.ParameterDigest) ([]ParameterSurfaceDescriptor, error) {
	if len(digest.Parameters) == 0 {
		return nil, fmt.Errorf("Plugin Grabber digest has no parameters")
	}
	descriptors := make([]ParameterSurfaceDescriptor, 0, len(digest.Parameters))
	for _, parameter := range digest.Parameters {
		id := strings.TrimSpace(parameter.ID)
		if id == "" {
			return nil, fmt.Errorf("Plugin Grabber digest contains a parameter without id")
		}
		minimum, minOK := numberFromDigest(parameter.Min)
		maximum, maxOK := numberFromDigest(parameter.Max)
		if !minOK || !maxOK {
			if domain := parameter.DisplayDomainCandidate; domain != nil && domain.Min != nil && domain.Max != nil {
				minimum, maximum, minOK, maxOK = *domain.Min, *domain.Max, true, true
			}
		}
		if !minOK || !maxOK || !finiteFingerprint(minimum) || !finiteFingerprint(maximum) || maximum < minimum {
			return nil, fmt.Errorf("Plugin Grabber parameter %s has no finite complete range", id)
		}
		displayDomain := ""
		unit := ""
		scale := ""
		if domain := parameter.DisplayDomainCandidate; domain != nil && strings.TrimSpace(domain.Text) != "" {
			displayDomain = strings.TrimSpace(domain.Text)
			unit = strings.TrimSpace(domain.Unit)
			scale = strings.TrimSpace(domain.Scale)
		} else {
			var ok bool
			displayDomain, unit, scale, ok = rawHostDisplayFingerprintDomain(parameter, minimum, maximum)
			if !ok {
				return nil, fmt.Errorf("Plugin Grabber parameter %s has no confirmed display-domain candidate", id)
			}
		}
		kind := "continuous"
		if parameter.IsBoolean {
			kind = "boolean"
		} else if parameter.IsDiscrete {
			kind = "enum"
		}
		descriptors = append(descriptors, ParameterSurfaceDescriptor{
			ID:            id,
			Type:          kind,
			Min:           &minimum,
			Max:           &maximum,
			EnumValues:    enumLabels(parameter),
			DisplayDomain: displayDomain,
			Unit:          unit,
			Scale:         scale,
		})
	}
	return descriptors, nil
}

// rawHostDisplayFingerprintDomain is deliberately not a semantic mapping.
// It records only stable information exposed by the host parameter surface:
// normalized bounds, labels and fixed probe samples. CurrentText remains
// useful observation evidence, but is a mutable control state rather than a
// surface fact; including it here would make ordinary parameter changes
// invalidate a VPS Credential. Capability dispatch still requires the
// separate confirmed Plugin Skill mapping and the bounded conformance
// transaction.
func rawHostDisplayFingerprintDomain(parameter plugingrabber.ParameterInfo, minimum, maximum float64) (string, string, string, bool) {
	probe := parameter.DisplayProbe
	if probe == nil {
		return "", "", "", false
	}
	samples := make([]string, 0, len(probe.Samples))
	for _, sample := range probe.Samples {
		text := strings.TrimSpace(sample.Text)
		if text == "" {
			continue
		}
		samples = append(samples, strconv.FormatFloat(sample.NormalizedValue, 'g', -1, 64)+":"+text)
	}
	// A current value alone is mutable state, not a stable description of the
	// parameter surface.  Retaining it as the sole qualification evidence
	// would both undermine fail-closed fingerprinting and reintroduce the
	// Bell/Low-S false-staleness bug this descriptor prevents.
	if strings.TrimSpace(probe.Label) == "" && len(samples) == 0 {
		return "", "", "", false
	}
	sort.Strings(samples)
	parts := []string{
		"host_range=" + strconv.FormatFloat(minimum, 'g', -1, 64) + ".." + strconv.FormatFloat(maximum, 'g', -1, 64),
	}
	if label := strings.TrimSpace(probe.Label); label != "" {
		parts = append(parts, "host_label="+label)
	}
	if len(samples) > 0 {
		parts = append(parts, "host_samples="+strings.Join(samples, "|"))
	}
	unit := strings.TrimSpace(parameter.Unit)
	if unit == "" {
		unit = strings.TrimSpace(probe.Label)
	}
	scale := "host_normalized"
	if parameter.IsBoolean || parameter.IsDiscrete {
		scale = "enum"
	}
	return strings.Join(parts, ";"), unit, scale, true
}

func enumLabels(parameter plugingrabber.ParameterInfo) []string {
	if !parameter.IsDiscrete && !parameter.IsBoolean {
		return nil
	}
	labels := []string{}
	if parameter.DisplayProbe != nil {
		for _, label := range parameter.DisplayProbe.DiscreteLabels {
			if text := strings.TrimSpace(label.Label); text != "" {
				labels = append(labels, text)
			}
		}
	}
	if parameter.IsBoolean && len(labels) == 0 {
		labels = []string{"off", "on"}
	}
	return uniqueSorted(labels)
}

func numberFromDigest(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, finiteFingerprint(typed)
	case float32:
		return float64(typed), finiteFingerprint(float64(typed))
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil && finiteFingerprint(parsed)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil && finiteFingerprint(parsed)
	default:
		return 0, false
	}
}
