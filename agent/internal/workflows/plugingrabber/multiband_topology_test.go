package plugingrabber

import (
	"fmt"
	"testing"
)

func multibandParam(id, name, value string) ParameterInfo {
	return ParameterInfo{ID: id, Name: name, RawName: name, HostControllable: true, NormalizedValue: 0.5, ValueText: value}
}

func c6Fixture() ParameterDigest {
	params := []ParameterInfo{
		multibandParam("x1", "Low Crossover", "92 Hz"),
		multibandParam("x2", "Mid Crossover", "4000 Hz"),
		multibandParam("x3", "High Crossover", "11071 Hz"),
		multibandParam("shared", "Knee", "0.50"),
	}
	for band := 1; band <= 6; band++ {
		params = append(params,
			multibandParam(fmt.Sprintf("t%d", band), fmt.Sprintf("Band %d Threshold", band), "-12 dB"),
			multibandParam(fmt.Sprintf("g%d", band), fmt.Sprintf("Band %d Gain", band), "0 dB"),
			multibandParam(fmt.Sprintf("r%d", band), fmt.Sprintf("Band %d Range", band), "-8 dB"),
			multibandParam(fmt.Sprintf("a%d", band), fmt.Sprintf("Band %d Attack", band), "10 ms"),
			multibandParam(fmt.Sprintf("l%d", band), fmt.Sprintf("Band %d Release", band), "100 ms"),
		)
	}
	return ParameterDigest{Parameters: params}
}

func TestDetectMultibandModelC6StructuralSurface(t *testing.T) {
	model, code := DetectMultibandModelWithBoundary(c6Fixture())
	if model == nil || code != "" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if model.Classification != "filterbank_repeated_dynamics" || len(model.Crossovers) != 3 || len(model.Bands) != 6 {
		t.Fatalf("unexpected model=%+v", model)
	}
	for _, band := range model.Bands {
		if !multibandBandComplete(band) || len(band.OperatingPoint) == 0 || len(band.GainAction) == 0 || len(band.Timing) < 2 {
			t.Fatalf("incomplete band=%+v", band)
		}
	}
	if model.Generation == "" {
		t.Fatal("missing topology generation")
	}
}

func TestDetectMultibandModelC4ThreshAndRangeIntersection(t *testing.T) {
	params := []ParameterInfo{
		multibandParam("x1", "Low Crossover", "92 Hz"),
		multibandParam("x2", "Mid Crossover", "4000 Hz"),
		multibandParam("x3", "High Crossover", "11071 Hz"),
		multibandParam("shared", "Behavior", "Electro"),
		multibandParam("shared_knee", "Knee", "0.50"),
	}
	for band := 1; band <= 4; band++ {
		params = append(params,
			multibandParam(fmt.Sprintf("gain_%d", band), fmt.Sprintf("Band %d Gain", band), "0 dB"),
			multibandParam(fmt.Sprintf("range_%d", band), fmt.Sprintf("Band %d Range", band), "-8 dB"),
			multibandParam(fmt.Sprintf("attack_%d", band), fmt.Sprintf("Band %d Attack", band), "10 ms"),
			multibandParam(fmt.Sprintf("release_%d", band), fmt.Sprintf("Band %d Release", band), "100 ms"),
		)
		if band == 1 {
			params = append(params, multibandParam("thresh_1", "Band 1 Thresh", "-20 dB"))
		}
	}
	model, code := DetectMultibandModelWithBoundary(ParameterDigest{Parameters: params})
	if model == nil || code != "" || len(model.Crossovers) != 3 || len(model.Bands) != 4 {
		t.Fatalf("C4 model=%+v code=%q", model, code)
	}
}

func TestDetectMultibandModelNamedMBCAnd354ECluster(t *testing.T) {
	params := []ParameterInfo{
		multibandParam("xl", "Low-Mid Freq", "120 Hz"),
		multibandParam("xh", "Mid-High Freq", "4000 Hz"),
		multibandParam("shared", "Knee", "Soft"),
	}
	for _, name := range []string{"Low", "Mid", "High"} {
		params = append(params,
			multibandParam("th_"+name, name+" Threshold", "-18 dB"),
			multibandParam("ra_"+name, name+" Ratio", "2:1"),
			multibandParam("at_"+name, name+" Attack", "10 ms"),
			multibandParam("re_"+name, name+" Release", "100 ms"),
		)
	}
	model, code := DetectMultibandModelWithBoundary(ParameterDigest{Parameters: params})
	if model == nil || code != "" || len(model.Bands) != 3 || len(model.Crossovers) != 2 {
		t.Fatalf("named model=%+v code=%q", model, code)
	}

	params = []ParameterInfo{
		multibandParam("xl", "Low XOverParameter", "200 Hz"),
		multibandParam("xh", "High XOverParameter", "5000 Hz"),
		multibandParam("shared", "Master Gain", "0 dB"),
	}
	for _, name := range []string{"Low", "Mid", "High"} {
		params = append(params,
			multibandParam("th_354e_"+name, name+" Threshold", "-4 dB"),
			multibandParam("ra_354e_"+name, name+" Ratio", "3:1"),
			multibandParam("at_354e_"+name, name+" Attack", "2 ms"),
			multibandParam("re_354e_"+name, name+" Recovery", "Auto"),
		)
	}
	model, code = DetectMultibandModelWithBoundary(ParameterDigest{Parameters: params})
	if model == nil || code != "" || len(model.Bands) != 3 || len(model.Crossovers) != 2 {
		t.Fatalf("354E model=%+v code=%q", model, code)
	}
}

func TestDetectMultibandModelLinMBFourCrossoverOrder(t *testing.T) {
	params := []ParameterInfo{
		multibandParam("x1", "Low Crossover", "92 Hz"),
		multibandParam("x2", "LoMid Crossover", "545 Hz"),
		multibandParam("x3", "HiMid Crossover", "4000 Hz"),
		multibandParam("x4", "High Crossover", "11071 Hz"),
		multibandParam("shared", "Release Mode", "ARC"),
	}
	for band := 1; band <= 5; band++ {
		params = append(params,
			multibandParam(fmt.Sprintf("range_%d", band), fmt.Sprintf("Band %d Range", band), "-6 dB"),
			multibandParam(fmt.Sprintf("gain_%d", band), fmt.Sprintf("Band %d Gain", band), "0 dB"),
			multibandParam(fmt.Sprintf("attack_%d", band), fmt.Sprintf("Band %d Attack", band), "10 ms"),
		)
	}
	model, code := DetectMultibandModelWithBoundary(ParameterDigest{Parameters: params})
	if model == nil || code != "" || len(model.Crossovers) != 4 || len(model.Bands) != 5 {
		t.Fatalf("LinMB model=%+v code=%q", model, code)
	}
}

func TestDetectMultibandRejectsBandLocalCrossoverPairs(t *testing.T) {
	params := []ParameterInfo{multibandParam("shared", "Bypass", "Off")}
	for band := 1; band <= 3; band++ {
		params = append(params,
			multibandParam(fmt.Sprintf("lo_%d", band), fmt.Sprintf("Band %d Low Crossover", band), "30 Hz"),
			multibandParam(fmt.Sprintf("hi_%d", band), fmt.Sprintf("Band %d High Crossover", band), "30000 Hz"),
			multibandParam(fmt.Sprintf("threshold_%d", band), fmt.Sprintf("Band %d Threshold", band), "-18 dB"),
			multibandParam(fmt.Sprintf("ratio_%d", band), fmt.Sprintf("Band %d Ratio", band), "4:1"),
			multibandParam(fmt.Sprintf("attack_%d", band), fmt.Sprintf("Band %d Attack", band), "20%"),
		)
	}
	model, code := DetectMultibandModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "unresolved_band_local_crossovers" {
		t.Fatalf("band-local model=%+v code=%q", model, code)
	}
}

func TestMultibandRejectsAdjacentSurfaces(t *testing.T) {
	tests := []struct {
		name string
		rows []ParameterInfo
		want string
	}{
		{"dynamic eq", []ParameterInfo{
			multibandParam("x1", "Crossover 1", "100 Hz"), multibandParam("x2", "Crossover 2", "2 kHz"),
			multibandParam("shared", "Knee", "Soft"),
			multibandParam("f1", "Band 1 Frequency", "100 Hz"), multibandParam("t1", "Band 1 Dynamic EQ Threshold", "-12 dB"),
			multibandParam("g1", "Band 1 Gain", "0 dB"), multibandParam("a1", "Band 1 Attack", "10 ms"),
			multibandParam("f2", "Band 2 Frequency", "2 kHz"), multibandParam("t2", "Band 2 Dynamic EQ Threshold", "-12 dB"),
			multibandParam("g2", "Band 2 Gain", "0 dB"), multibandParam("a2", "Band 2 Attack", "10 ms"),
		}, "unsupported_dynamic_eq"},
		{"sidechain eq", []ParameterInfo{
			multibandParam("x", "SC Crossover 1", "400 Hz"),
			multibandParam("f1", "SC EQ Band 1 Frequency", "100 Hz"),
			multibandParam("t1", "SC EQ Band 1 Threshold", "-12 dB"),
		}, "not_multiband_dynamics"},
		{"multiband maximizer", []ParameterInfo{
			multibandParam("x", "Crossover 1", "400 Hz"),
			multibandParam("y", "Crossover 2", "4 kHz"),
			multibandParam("m", "Maximizer Enable", "On"),
			multibandParam("t1", "Band 1 Threshold", "-12 dB"),
			multibandParam("c1", "Band 1 Ceiling", "-1 dB"),
			multibandParam("r1", "Band 1 Release", "100 ms"),
			multibandParam("t2", "Band 2 Threshold", "-12 dB"),
			multibandParam("c2", "Band 2 Ceiling", "-1 dB"),
			multibandParam("r2", "Band 2 Release", "100 ms"),
		}, "unsupported_multiband_maximizer"},
		{"de-esser", []ParameterInfo{
			multibandParam("x1", "Crossover 1", "400 Hz"), multibandParam("x2", "Crossover 2", "4 kHz"),
			multibandParam("shared", "Knee", "Soft"), multibandParam("ds", "De-esser Mode", "Wide"),
			multibandParam("t1", "Band 1 Threshold", "-12 dB"), multibandParam("g1", "Band 1 Gain", "0 dB"), multibandParam("a1", "Band 1 Attack", "10 ms"),
			multibandParam("t2", "Band 2 Threshold", "-12 dB"), multibandParam("g2", "Band 2 Gain", "0 dB"), multibandParam("a2", "Band 2 Attack", "10 ms"),
		}, "unsupported_de_esser"},
		{"spectral dynamics", []ParameterInfo{
			multibandParam("x1", "Crossover 1", "400 Hz"), multibandParam("x2", "Crossover 2", "4 kHz"),
			multibandParam("shared", "Knee", "Soft"), multibandParam("sd", "Spectral Dynamics Mode", "Adaptive"),
			multibandParam("t1", "Band 1 Threshold", "-12 dB"), multibandParam("g1", "Band 1 Gain", "0 dB"), multibandParam("a1", "Band 1 Attack", "10 ms"),
			multibandParam("t2", "Band 2 Threshold", "-12 dB"), multibandParam("g2", "Band 2 Gain", "0 dB"), multibandParam("a2", "Band 2 Attack", "10 ms"),
		}, "unsupported_spectral_dynamics"},
		{"clipper adjacent", []ParameterInfo{
			multibandParam("x1", "Crossover 1", "400 Hz"), multibandParam("x2", "Crossover 2", "4 kHz"),
			multibandParam("shared", "Knee", "Soft"), multibandParam("clip", "Soft Clip Shape", "Hard"),
			multibandParam("t1", "Band 1 Threshold", "-12 dB"), multibandParam("g1", "Band 1 Gain", "0 dB"), multibandParam("a1", "Band 1 Attack", "10 ms"),
			multibandParam("t2", "Band 2 Threshold", "-12 dB"), multibandParam("g2", "Band 2 Gain", "0 dB"), multibandParam("a2", "Band 2 Attack", "10 ms"),
		}, "unsupported_clipper"},
		{"unordered crossovers", []ParameterInfo{
			multibandParam("x1", "Crossover 1", "4000 Hz"), multibandParam("x2", "Crossover 2", "400 Hz"),
			multibandParam("t1", "Band 1 Threshold", "-12 dB"), multibandParam("g1", "Band 1 Gain", "0 dB"), multibandParam("a1", "Band 1 Attack", "10 ms"),
			multibandParam("t2", "Band 2 Threshold", "-12 dB"), multibandParam("g2", "Band 2 Gain", "0 dB"), multibandParam("a2", "Band 2 Attack", "10 ms"),
		}, "unresolved_crossover_order"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, code := DetectMultibandModelWithBoundary(ParameterDigest{Parameters: test.rows})
			if model != nil || code != test.want {
				t.Fatalf("model=%+v code=%q want=%q", model, code, test.want)
			}
		})
	}
}

func TestMultibandGenerationIgnoresIdentityAndCurrentValues(t *testing.T) {
	a := c6Fixture()
	b := c6Fixture()
	a.PluginName, b.PluginName = "A", "B"
	a.Parameters[3].NormalizedValue, b.Parameters[3].NormalizedValue = 0.1, 0.9
	a.Parameters[3].ValueText, b.Parameters[3].ValueText = "-20 dB", "-3 dB"
	ma, _ := DetectMultibandModelWithBoundary(a)
	mb, _ := DetectMultibandModelWithBoundary(b)
	if ma == nil || mb == nil || ma.Generation != mb.Generation {
		t.Fatalf("generation differs: %q vs %q", ma.Generation, mb.Generation)
	}
}

func TestDetectMultibandRejectsUnorderedFreeBandSurface(t *testing.T) {
	params := []ParameterInfo{}
	for band := 1; band <= 2; band++ {
		params = append(params,
			multibandParam(fmt.Sprintf("f%d", band), fmt.Sprintf("Band %d Frequency", band), "1 kHz"),
			multibandParam(fmt.Sprintf("w%d", band), fmt.Sprintf("Band %d Width", band), "1 octave"),
			multibandParam(fmt.Sprintf("t%d", band), fmt.Sprintf("Band %d Threshold", band), "-18 dB"),
			multibandParam(fmt.Sprintf("a%d", band), fmt.Sprintf("Band %d Attack", band), "10 ms"),
		)
	}
	model, code := DetectMultibandModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "not_multiband_dynamics" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}
