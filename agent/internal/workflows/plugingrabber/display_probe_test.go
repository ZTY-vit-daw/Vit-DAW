package plugingrabber

import "testing"

func TestDisplayProbeInfersCommonDisplayDomains(t *testing.T) {
	reply := map[string]any{
		"track_id":  "track_1",
		"plugin_id": "delay_1",
		"parameters": []any{
			probedParam("delay", "Delay", "ms", []string{"0 ms", "500 ms", "1000 ms", "1500 ms", "2000 ms"}),
			probedParam("mix", "Mix", "%", []string{"0 %", "25 %", "50 %", "75 %", "100 %"}),
			probedParam("freq", "Frequency", "Hz", []string{"20 Hz", "200 Hz", "2 kHz", "8 kHz", "20 kHz"}),
			probedParam("gain", "Gain", "dB", []string{"-18 dB", "-9 dB", "0 dB", "9 dB", "18 dB"}),
			map[string]any{
				"id":                "mode",
				"name":              "Mode",
				"host_controllable": true,
				"is_discrete":       true,
				"num_steps":         3,
				"display_probe": map[string]any{
					"mode": "read_only_value_to_string",
					"discrete_labels": []any{
						map[string]any{"index": 0, "value": 0.0, "label": "Echo"},
						map[string]any{"index": 1, "value": 0.5, "label": "Reverb"},
						map[string]any{"index": 2, "value": 1.0, "label": "Warp"},
					},
				},
			},
		},
	}
	digest := BuildParameterDigest(reply)
	byID := map[string]ParameterInfo{}
	for _, param := range digest.Parameters {
		byID[param.ID] = param
	}
	assertDomain(t, byID["delay"].DisplayDomainCandidate, "ms", 0, 2000)
	assertDomain(t, byID["mix"].DisplayDomainCandidate, "%", 0, 100)
	assertDomain(t, byID["freq"].DisplayDomainCandidate, "Hz", 20, 20000)
	assertDomain(t, byID["gain"].DisplayDomainCandidate, "dB", -18, 18)
	if domain := byID["mode"].DisplayDomainCandidate; domain == nil || domain.Unit != "enum" || domain.Scale != "enum" {
		t.Fatalf("mode domain = %+v", domain)
	}
}

func TestDisplayProbeInfersBooleanToggle(t *testing.T) {
	reply := map[string]any{
		"parameters": []any{map[string]any{
			"id":                "bypass",
			"name":              "Bypass",
			"host_controllable": true,
			"is_boolean":        true,
			"is_discrete":       true,
			"num_steps":         2,
			"display_probe": map[string]any{
				"mode":       "read_only_value_to_string",
				"all_labels": []any{"Off", "On"},
			},
		}},
	}
	digest := BuildParameterDigest(reply)
	domain := digest.Parameters[0].DisplayDomainCandidate
	if domain == nil || domain.Unit != "toggle" || domain.Scale != "enum" {
		t.Fatalf("toggle domain = %+v", domain)
	}
}

func probedParam(id, name, label string, texts []string) map[string]any {
	samples := make([]any, 0, len(texts))
	for i, text := range texts {
		samples = append(samples, map[string]any{
			"normalized_value": float64(i) / float64(len(texts)-1),
			"value":            float64(i) / float64(len(texts)-1),
			"text":             text,
		})
	}
	return map[string]any{
		"id":                id,
		"name":              name,
		"host_controllable": true,
		"display_probe": map[string]any{
			"mode":         "read_only_value_to_string",
			"label":        label,
			"current_text": texts[len(texts)/2],
			"samples":      samples,
		},
	}
}

func assertDomain(t *testing.T, domain *PluginDisplayDomain, unit string, minValue, maxValue float64) {
	t.Helper()
	if domain == nil {
		t.Fatalf("domain is nil")
	}
	if domain.Unit != unit {
		t.Fatalf("unit = %q, want %q; domain=%+v", domain.Unit, unit, domain)
	}
	if domain.Min == nil || domain.Max == nil || *domain.Min != minValue || *domain.Max != maxValue {
		t.Fatalf("range = %+v..%+v, want %g..%g; domain=%+v", domain.Min, domain.Max, minValue, maxValue, domain)
	}
	if domain.Source != displayProbeSourceInferred || domain.Confidence < 0.80 {
		t.Fatalf("source/confidence = %+v", domain)
	}
}
