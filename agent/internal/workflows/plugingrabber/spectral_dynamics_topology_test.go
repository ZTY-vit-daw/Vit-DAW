package plugingrabber

import (
	"fmt"
	"testing"
)

func spectralParam(id, name, value string) ParameterInfo {
	return ParameterInfo{ID: id, Name: name, RawName: name, HostControllable: true, NormalizedValue: 0.5, ValueText: value}
}

func dsmSpectralFixture() ParameterDigest {
	params := []ParameterInfo{
		spectralParam("mode", "Dynamic Mode", "Adaptive"),
		spectralParam("cr", "Compressor Ratio", "2:1"),
		spectralParam("er", "Expander Ratio", "1:2"),
		spectralParam("gt", "Threshold", "-18 dB"),
		spectralParam("a", "Attack", "10 ms"),
		spectralParam("r", "Release", "100 ms"),
		spectralParam("k", "Knee", "Soft"),
		spectralParam("capture", "Capture", "Off"),
	}
	for i := 1; i <= 3; i++ {
		params = append(params,
			spectralParam(fmt.Sprintf("f%d", i), fmt.Sprintf("Frequency %d", i), "1 kHz"),
			spectralParam(fmt.Sprintf("t%d", i), fmt.Sprintf("Threshold %d", i), "-12 dB"),
			spectralParam(fmt.Sprintf("q%d", i), fmt.Sprintf("Q %d", i), "1.0"),
		)
	}
	return ParameterDigest{PluginName: "identity ignored", Parameters: params}
}

func TestDetectSpectralDynamicsRequiresFieldAndGlobalLaw(t *testing.T) {
	model, code := DetectSpectralDynamicsModelWithBoundary(dsmSpectralFixture())
	if model == nil || code != "" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if model.Classification != "spectral_field_global_dynamics" || !model.Field.Dense || model.Field.IndexedCount != 3 {
		t.Fatalf("unexpected model=%+v", model)
	}
	if len(model.GlobalLaw.OperatingPoint) == 0 || len(model.GlobalLaw.Transfer) == 0 || len(model.GlobalLaw.Timing) != 2 {
		t.Fatalf("incomplete global law=%+v", model.GlobalLaw)
	}
	if model.Generation == "" || model.Observation.Status != "inspect_only_observation_required" {
		t.Fatalf("missing generation/observation=%+v", model)
	}
}

func TestSpectralGenerationIgnoresIdentityAndCurrentValues(t *testing.T) {
	a, b := dsmSpectralFixture(), dsmSpectralFixture()
	a.PluginName, b.PluginName = "A", "B"
	a.Parameters[0].NormalizedValue, b.Parameters[0].NormalizedValue = 0.1, 0.9
	a.Parameters[0].ValueText, b.Parameters[0].ValueText = "Static", "Learned"
	ma, ca := DetectSpectralDynamicsModelWithBoundary(a)
	mb, cb := DetectSpectralDynamicsModelWithBoundary(b)
	if ma == nil || mb == nil || ca != "" || cb != "" || ma.Generation != mb.Generation {
		t.Fatalf("generation mismatch ma=%+v ca=%q mb=%+v cb=%q", ma, ca, mb, cb)
	}
}

func TestSpectralRejectsDynamicEQRepeatedGainCells(t *testing.T) {
	params := []ParameterInfo{
		spectralParam("gt", "Threshold", "-18 dB"), spectralParam("ratio", "Ratio", "2:1"),
		spectralParam("a", "Attack", "10 ms"), spectralParam("r", "Release", "100 ms"),
	}
	for i := 1; i <= 3; i++ {
		params = append(params,
			spectralParam(fmt.Sprintf("f%d", i), fmt.Sprintf("Frequency %d", i), "1 kHz"),
			spectralParam(fmt.Sprintf("g%d", i), fmt.Sprintf("Gain %d", i), "0 dB"),
			spectralParam(fmt.Sprintf("t%d", i), fmt.Sprintf("Dynamic Threshold %d", i), "-12 dB"),
		)
	}
	model, code := DetectSpectralDynamicsModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "unsupported_dynamic_eq" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestSpectralDoesNotCallStaticEQNodesDynamicEQ(t *testing.T) {
	params := []ParameterInfo{}
	for i := 1; i <= 8; i++ {
		params = append(params,
			spectralParam(fmt.Sprintf("f%d", i), fmt.Sprintf("Node %d Frequency", i), "1 kHz"),
			spectralParam(fmt.Sprintf("g%d", i), fmt.Sprintf("Node %d Gain", i), "0 dB"),
			spectralParam(fmt.Sprintf("q%d", i), fmt.Sprintf("Node %d Q", i), "1.0"),
		)
	}
	model, code := DetectSpectralDynamicsModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "unresolved_spectral_dynamics_surface" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestSpectralRejectsDynamicRangeCellsAsDynamicEQ(t *testing.T) {
	params := []ParameterInfo{}
	for i := 1; i <= 3; i++ {
		params = append(params,
			spectralParam(fmt.Sprintf("f%d", i), fmt.Sprintf("Band %d Frequency", i), "1 kHz"),
			spectralParam(fmt.Sprintf("g%d", i), fmt.Sprintf("Band %d Gain", i), "0 dB"),
			spectralParam(fmt.Sprintf("dr%d", i), fmt.Sprintf("Band %d Dynamic Range", i), "6 dB"),
		)
	}
	model, code := DetectSpectralDynamicsModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "unsupported_dynamic_eq" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestSpectralReturnsUnresolvedWithoutGlobalLaw(t *testing.T) {
	params := []ParameterInfo{}
	for i := 1; i <= 3; i++ {
		params = append(params,
			spectralParam(fmt.Sprintf("f%d", i), fmt.Sprintf("Frequency %d", i), "1 kHz"),
			spectralParam(fmt.Sprintf("t%d", i), fmt.Sprintf("Threshold %d", i), "-12 dB"),
		)
	}
	model, code := DetectSpectralDynamicsModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "unresolved_spectral_dynamics_surface" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestSpectralRequiresMatchingFrequencyAndThresholdIndices(t *testing.T) {
	params := []ParameterInfo{}
	for i := 1; i <= 3; i++ {
		params = append(params, spectralParam(fmt.Sprintf("f%d", i), fmt.Sprintf("Frequency %d", i), "1 kHz"))
	}
	for i := 4; i <= 6; i++ {
		params = append(params, spectralParam(fmt.Sprintf("t%d", i), fmt.Sprintf("Threshold %d", i), "-12 dB"))
	}
	model, code := DetectSpectralDynamicsModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "unresolved_spectral_dynamics_surface" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestSpectralAdjacentVetoesRequireMatchingCellIndices(t *testing.T) {
	params := []ParameterInfo{
		spectralParam("f1", "Band 1 Frequency", "1 kHz"),
		spectralParam("f2", "Band 2 Frequency", "2 kHz"),
		spectralParam("g3", "Band 3 Gain", "0 dB"),
		spectralParam("g4", "Band 4 Gain", "0 dB"),
		spectralParam("t5", "Band 5 Threshold", "-12 dB"),
		spectralParam("t6", "Band 6 Threshold", "-12 dB"),
	}
	model, code := DetectSpectralDynamicsModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "unresolved_spectral_dynamics_surface" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestSpectralRejectsRepeatedBandTimingCellsAsMultiband(t *testing.T) {
	params := []ParameterInfo{}
	for i := 1; i <= 3; i++ {
		params = append(params,
			spectralParam(fmt.Sprintf("t%d", i), fmt.Sprintf("Band %d Threshold", i), "-12 dB"),
			spectralParam(fmt.Sprintf("a%d", i), fmt.Sprintf("Band %d Attack", i), "10 ms"),
			spectralParam(fmt.Sprintf("r%d", i), fmt.Sprintf("Band %d Release", i), "100 ms"),
		)
	}
	model, code := DetectSpectralDynamicsModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "unsupported_multiband_dynamics" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestSpectralKeepsLimiterAndClipperBoundariesSeparate(t *testing.T) {
	tests := []struct {
		name string
		row  ParameterInfo
		want string
	}{
		{"limiter", spectralParam("ceiling", "True Peak Ceiling", "-1 dB"), "unsupported_limiter"},
		{"clipper", spectralParam("clip", "Soft Clip Shape", "Hard"), "unsupported_clipper"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, code := DetectSpectralDynamicsModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{test.row}})
			if model != nil || code != test.want {
				t.Fatalf("model=%+v code=%q", model, code)
			}
		})
	}
}

func TestSpectralDoesNotUseCaptureAsPositiveEvidence(t *testing.T) {
	model, code := DetectSpectralDynamicsModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		spectralParam("capture", "Capture", "On"), spectralParam("freeze", "Freeze Gain", "Off"),
	}})
	if model != nil || code != "not_spectral_dynamics" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestBuildSpectralSummaryIsIdentityIndependent(t *testing.T) {
	one, two := dsmSpectralFixture(), dsmSpectralFixture()
	one.PluginName, two.PluginName = "DSM", "Curves"
	a, ca := BuildSpectralDynamicsSummaryWithBoundary(one)
	b, cb := BuildSpectralDynamicsSummaryWithBoundary(two)
	if a == nil || b == nil || ca != "" || cb != "" {
		t.Fatalf("summary rejected ca=%q cb=%q", ca, cb)
	}
	if a["classification"] != b["classification"] || a["schema_version"] != spectralDynamicsTopologySchema {
		t.Fatalf("unexpected summary a=%v b=%v", a, b)
	}
}

func TestSpectralDisclosedShapeRegressionProjections(t *testing.T) {
	curves := []ParameterInfo{}
	for i := 1; i <= 8; i++ {
		curves = append(curves,
			spectralParam(fmt.Sprintf("f%d", i), fmt.Sprintf("Node %d Frequency", i), "2000 Hz"),
			spectralParam(fmt.Sprintf("g%d", i), fmt.Sprintf("Node %d Gain", i), "0 dB"),
			spectralParam(fmt.Sprintf("q%d", i), fmt.Sprintf("Node %d Q", i), "1.0"),
		)
	}
	dynamicEQ := []ParameterInfo{}
	for i := 1; i <= 24; i++ {
		dynamicEQ = append(dynamicEQ,
			spectralParam(fmt.Sprintf("f%d", i), fmt.Sprintf("Band %d Frequency", i), "1000 Hz"),
			spectralParam(fmt.Sprintf("g%d", i), fmt.Sprintf("Band %d Gain", i), "0 dB"),
			spectralParam(fmt.Sprintf("dr%d", i), fmt.Sprintf("Band %d Dynamic Range", i), "6 dB"),
		)
	}
	multiband := []ParameterInfo{}
	for i := 1; i <= 6; i++ {
		multiband = append(multiband,
			spectralParam(fmt.Sprintf("t%d", i), fmt.Sprintf("Band %d Threshold", i), "-12 dB"),
			spectralParam(fmt.Sprintf("a%d", i), fmt.Sprintf("Band %d Attack", i), "10 ms"),
			spectralParam(fmt.Sprintf("r%d", i), fmt.Sprintf("Band %d Release", i), "100 ms"),
		)
	}
	checks := []struct {
		name string
		rows []ParameterInfo
		want string
	}{
		{"static_node_field", curves, "unresolved_spectral_dynamics_surface"},
		{"dynamic_eq_cells", dynamicEQ, "unsupported_dynamic_eq"},
		{"multiband_cells", multiband, "unsupported_multiband_dynamics"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			model, code := DetectSpectralDynamicsModelWithBoundary(ParameterDigest{Parameters: check.rows})
			if model != nil || code != check.want {
				t.Fatalf("model=%+v code=%q want=%q", model, code, check.want)
			}
		})
	}
}
