package vps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// ParameterSurfaceDescriptor is the complete parameter fact required before a
// VPS v3 Credential can be verified. It deliberately includes more than the
// older Plugin Skill ID-only hash: type, range, enum and display semantics all
// participate in the resulting fingerprints.
type ParameterSurfaceDescriptor struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Min           *float64 `json:"min,omitempty"`
	Max           *float64 `json:"max,omitempty"`
	EnumValues    []string `json:"enum_values,omitempty"`
	DisplayDomain string   `json:"display_domain"`
	Unit          string   `json:"unit,omitempty"`
	Scale         string   `json:"scale,omitempty"`
}

// BuildPluginFingerprint produces the v3 installation, parameter-surface and
// display-surface signatures from a fresh host observation. The caller owns
// acquisition of the installation fingerprint; this helper makes the rest
// deterministic and rejects partial descriptor data.
func BuildPluginFingerprint(installationFingerprint string, parameters []ParameterSurfaceDescriptor) (PluginFingerprint, error) {
	installationFingerprint = strings.TrimSpace(installationFingerprint)
	if installationFingerprint == "" {
		return PluginFingerprint{}, fmt.Errorf("plugin installation fingerprint is required")
	}
	if len(parameters) == 0 {
		return PluginFingerprint{}, fmt.Errorf("at least one parameter descriptor is required")
	}
	canonical := make([]parameterFingerprintRow, 0, len(parameters))
	seen := map[string]bool{}
	for _, parameter := range parameters {
		id := strings.TrimSpace(parameter.ID)
		kind := strings.TrimSpace(strings.ToLower(parameter.Type))
		display := strings.TrimSpace(parameter.DisplayDomain)
		if id == "" || kind == "" || display == "" {
			return PluginFingerprint{}, fmt.Errorf("parameter descriptor requires id, type and display_domain")
		}
		if seen[id] {
			return PluginFingerprint{}, fmt.Errorf("duplicate parameter descriptor id %s", id)
		}
		seen[id] = true
		if parameter.Min == nil || parameter.Max == nil || !finiteFingerprint(*parameter.Min) || !finiteFingerprint(*parameter.Max) || *parameter.Max < *parameter.Min {
			return PluginFingerprint{}, fmt.Errorf("parameter descriptor %s requires finite min <= max", id)
		}
		canonical = append(canonical, parameterFingerprintRow{
			ID:            id,
			Type:          kind,
			Min:           *parameter.Min,
			Max:           *parameter.Max,
			EnumValues:    uniqueSorted(parameter.EnumValues),
			DisplayDomain: display,
			Unit:          strings.TrimSpace(parameter.Unit),
			Scale:         strings.TrimSpace(parameter.Scale),
		})
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].ID < canonical[j].ID })
	parameterRows := make([]parameterSurfaceHashRow, 0, len(canonical))
	displayRows := make([]displaySurfaceHashRow, 0, len(canonical))
	for _, row := range canonical {
		parameterRows = append(parameterRows, parameterSurfaceHashRow{ID: row.ID, Type: row.Type, Min: row.Min, Max: row.Max, EnumValues: row.EnumValues})
		displayRows = append(displayRows, displaySurfaceHashRow{ID: row.ID, DisplayDomain: row.DisplayDomain, Unit: row.Unit, Scale: row.Scale})
	}
	parameterSignature, err := signature("parameter_surface", parameterRows)
	if err != nil {
		return PluginFingerprint{}, err
	}
	displaySignature, err := signature("display_surface", displayRows)
	if err != nil {
		return PluginFingerprint{}, err
	}
	return PluginFingerprint{
		Installation:     installationFingerprint,
		ParameterSurface: parameterSignature,
		DisplaySurface:   displaySignature,
	}, nil
}

func finiteFingerprint(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

type parameterFingerprintRow struct {
	ID            string
	Type          string
	Min           float64
	Max           float64
	EnumValues    []string
	DisplayDomain string
	Unit          string
	Scale         string
}

type parameterSurfaceHashRow struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Min        float64  `json:"min"`
	Max        float64  `json:"max"`
	EnumValues []string `json:"enum_values,omitempty"`
}

type displaySurfaceHashRow struct {
	ID            string `json:"id"`
	DisplayDomain string `json:"display_domain"`
	Unit          string `json:"unit,omitempty"`
	Scale         string `json:"scale,omitempty"`
}

func signature(kind string, value any) (string, error) {
	payload, err := json.Marshal(struct {
		Kind  string `json:"kind"`
		Value any    `json:"value"`
	}{Kind: kind, Value: value})
	if err != nil {
		return "", fmt.Errorf("encode %s signature: %w", kind, err)
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
