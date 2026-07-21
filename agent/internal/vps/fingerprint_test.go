package vps

import (
	"math"
	"testing"
)

func TestBuildPluginFingerprintIncludesTypeRangeEnumAndDisplayDomain(t *testing.T) {
	base := testParameterDescriptors()
	first, err := BuildPluginFingerprint("binary:example-1.0.0", base)
	if err != nil {
		t.Fatalf("BuildPluginFingerprint: %v", err)
	}
	reversed := []ParameterSurfaceDescriptor{base[1], base[0]}
	second, err := BuildPluginFingerprint("binary:example-1.0.0", reversed)
	if err != nil {
		t.Fatalf("BuildPluginFingerprint reversed: %v", err)
	}
	if !first.Equal(second) {
		t.Fatalf("descriptor ordering changed fingerprint: first=%#v second=%#v", first, second)
	}

	cases := []struct {
		name             string
		mutate           func([]ParameterSurfaceDescriptor)
		parameterChanges bool
		displayChanges   bool
	}{
		{"type", func(rows []ParameterSurfaceDescriptor) { rows[0].Type = "enum" }, true, false},
		{"range", func(rows []ParameterSurfaceDescriptor) { *rows[0].Max = 24 }, true, false},
		{"enum", func(rows []ParameterSurfaceDescriptor) { rows[1].EnumValues = []string{"bell", "high_pass"} }, true, false},
		{"display domain", func(rows []ParameterSurfaceDescriptor) { rows[0].DisplayDomain = "-24..24 dB" }, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := cloneParameterDescriptors(base)
			tc.mutate(rows)
			updated, err := BuildPluginFingerprint("binary:example-1.0.0", rows)
			if err != nil {
				t.Fatalf("BuildPluginFingerprint: %v", err)
			}
			if got := updated.ParameterSurface != first.ParameterSurface; got != tc.parameterChanges {
				t.Fatalf("parameter signature changed=%v, want %v: before=%s after=%s", got, tc.parameterChanges, first.ParameterSurface, updated.ParameterSurface)
			}
			if got := updated.DisplaySurface != first.DisplaySurface; got != tc.displayChanges {
				t.Fatalf("display signature changed=%v, want %v: before=%s after=%s", got, tc.displayChanges, first.DisplaySurface, updated.DisplaySurface)
			}
		})
	}
}

func TestBuildPluginFingerprintRejectsPartialOrInvalidDescriptors(t *testing.T) {
	if _, err := BuildPluginFingerprint("", testParameterDescriptors()); err == nil {
		t.Fatalf("missing installation fingerprint was accepted")
	}
	missingDisplay := testParameterDescriptors()
	missingDisplay[0].DisplayDomain = ""
	if _, err := BuildPluginFingerprint("binary:example", missingDisplay); err == nil {
		t.Fatalf("missing display domain was accepted")
	}
	nanRange := testParameterDescriptors()
	*nanRange[0].Min = math.NaN()
	if _, err := BuildPluginFingerprint("binary:example", nanRange); err == nil {
		t.Fatalf("NaN range was accepted")
	}
}

func testParameterDescriptors() []ParameterSurfaceDescriptor {
	minimumGain, maximumGain := -18.0, 18.0
	minimumType, maximumType := 0.0, 2.0
	return []ParameterSurfaceDescriptor{
		{ID: "gain", Type: "continuous", Min: &minimumGain, Max: &maximumGain, DisplayDomain: "-18..18 dB", Unit: "dB", Scale: "linear"},
		{ID: "type", Type: "enum", Min: &minimumType, Max: &maximumType, EnumValues: []string{"bell", "low_shelf", "high_shelf"}, DisplayDomain: "filter type", Scale: "enum"},
	}
}

func cloneParameterDescriptors(input []ParameterSurfaceDescriptor) []ParameterSurfaceDescriptor {
	out := make([]ParameterSurfaceDescriptor, len(input))
	for index, row := range input {
		out[index] = row
		out[index].EnumValues = append([]string(nil), row.EnumValues...)
		if row.Min != nil {
			value := *row.Min
			out[index].Min = &value
		}
		if row.Max != nil {
			value := *row.Max
			out[index].Max = &value
		}
	}
	return out
}
