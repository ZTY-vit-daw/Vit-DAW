package plugingrabber

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAudioProcessorIdentityCardIsCompactAndExecutionFree(t *testing.T) {
	digest := ParameterDigest{PluginName: "CLA-2A Stereo", PluginIdentity: map[string]any{"manufacturer": "Waves"}, Parameters: []ParameterInfo{
		compressorParam("peak", "Peak Reduction", "31.5"),
		compressorParam("gain", "Output Gain", "0 dB"),
		compressorParam("mode", "Compress Limit", "Compress"),
	}}
	card, boundary := BuildAudioProcessorIdentityCard(digest)
	if card == nil || boundary != "" {
		t.Fatalf("card=%+v boundary=%q", card, boundary)
	}
	if card.Identity.Name != "CLA-2A Stereo" || card.Identity.Manufacturer != "Waves" || card.Archetype.InteractionStyle != "amount_driven_leveler" {
		t.Fatalf("identity/archetype=%+v", card)
	}
	joined := strings.Join(card.HardBoundaries, "|")
	for _, required := range []string{"no independent threshold", "no independent ratio", "no independent attack", "output or makeup gain is separate"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("missing hard boundary %q in %q", required, joined)
		}
	}
	encoded, _ := json.Marshal(card)
	for _, forbidden := range []string{"param_id", "control_ref", "normalized", "curve"} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("identity card leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestAudioProcessorIdentityCardUnknownNameStillUsesArchetype(t *testing.T) {
	digest := ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("threshold", "Threshold", "-18 dB"), compressorParam("ratio", "Ratio", "4:1"),
	}}
	card, boundary := BuildAudioProcessorIdentityCard(digest)
	if card == nil || boundary != "" || card.IdentityStatus != "unknown" || card.Identity.Name != "Unknown broadband compressor" {
		t.Fatalf("card=%+v boundary=%q", card, boundary)
	}
}

func TestCompressorControlBriefDisclosesOnlySelectedAxesWithoutExecutionRefs(t *testing.T) {
	digest := ParameterDigest{PluginName: "Pro-C 2", Parameters: []ParameterInfo{
		compressorParam("threshold-secret", "Threshold", "-18 dB"),
		compressorParam("ratio-secret", "Ratio", "4:1"),
		compressorParam("attack-secret", "Attack", "10 ms"),
		compressorParam("output-secret", "Output Gain", "0 dB"),
		compressorParam("mix-secret", "Mix", "75 %"),
	}}
	brief, boundary := BuildCompressorControlBrief(digest, []string{"activation_intensity"})
	if brief == nil || boundary != "" || len(brief.Controls) != 1 || brief.Controls[0].Role != "threshold" {
		t.Fatalf("brief=%+v boundary=%q", brief, boundary)
	}
	encoded, _ := json.Marshal(brief)
	text := string(encoded)
	for _, forbidden := range []string{"threshold-secret", "ratio-secret", "control_ref", "current_normalized"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("brief leaked %q: %s", forbidden, text)
		}
	}
}

func TestCompressorControlBriefOmitsAmbiguousSemanticBindings(t *testing.T) {
	digest := ParameterDigest{PluginName: "structural fixture", Parameters: []ParameterInfo{
		compressorParam("threshold", "Threshold", "-18 dB"),
		compressorParam("attack", "Attack", "3 ms"),
		compressorParam("release-continuous", "Release", "0.2 s"),
		compressorParam("release-mode", "Release", "Auto"),
	}}
	brief, boundary := BuildCompressorControlBrief(digest, []string{"transient_timing", "recovery_motion"})
	if brief == nil || boundary != "" {
		t.Fatalf("brief=%+v boundary=%q", brief, boundary)
	}
	for _, control := range brief.Controls {
		if control.PathKey == "main" && control.Role == "release" {
			t.Fatalf("ambiguous release leaked into executable brief: %+v", brief.Controls)
		}
	}
	if len(brief.Controls) != 1 || brief.Controls[0].Role != "attack" || !strings.Contains(strings.Join(brief.Boundaries, "|"), "ambiguous semantic binding unavailable: main/release") {
		t.Fatalf("brief did not preserve the unique control and ambiguity boundary: %+v", brief)
	}
}

func TestCompressorControlBriefKeepsSingleReleaseBinding(t *testing.T) {
	digest := ParameterDigest{PluginName: "structural fixture", Parameters: []ParameterInfo{
		compressorParam("threshold", "Threshold", "-18 dB"), compressorParam("release", "Release", "120 ms"),
	}}
	brief, boundary := BuildCompressorControlBrief(digest, []string{"recovery_motion"})
	if brief == nil || boundary != "" || len(brief.Controls) != 1 || brief.Controls[0].PathKey != "main" || brief.Controls[0].Role != "release" {
		t.Fatalf("single release binding was not disclosed: brief=%+v boundary=%q", brief, boundary)
	}
}

func TestCompressorControlBriefKeepsStableEquivalentContinuousBinding(t *testing.T) {
	minimum, maximum := 0.0, 20.0
	first := compressorParam("threshold-1", "Threshold 1", "0.0")
	second := compressorParam("threshold-2", "Threshold 2", "0.0")
	first.DisplayDomainCandidate = &PluginDisplayDomain{Min: &minimum, Max: &maximum, Unit: "dB"}
	second.DisplayDomainCandidate = &PluginDisplayDomain{Min: &minimum, Max: &maximum, Unit: "dB"}
	brief, boundary := BuildCompressorControlBrief(ParameterDigest{Parameters: []ParameterInfo{
		first, second, compressorParam("ratio", "Ratio", "2:1"),
	}}, []string{"activation_intensity"})
	if brief == nil || boundary != "" || len(brief.Controls) != 1 || brief.Controls[0].Role != "threshold" {
		t.Fatalf("equivalent continuous controls were not deterministically reduced: brief=%+v boundary=%q", brief, boundary)
	}
}

func TestCompressorControlBriefPrefersUniqueContinuousInputDrive(t *testing.T) {
	minimum, maximum := 0.0, 10.0
	inputGain := compressorParam("input-gain", "Input Gain", "4.0")
	inputGain.DisplayDomainCandidate = &PluginDisplayDomain{Min: &minimum, Max: &maximum, Unit: "dB"}
	inputPad := compressorParam("input-pad", "Input Pad", "Off")
	inputPad.DisplayProbe = &ParameterDisplayProbe{DiscreteLabels: []ParameterDisplayProbeLabel{
		{Index: 0, Label: "Off"}, {Index: 1, Label: "On"},
	}}
	brief, boundary := BuildCompressorControlBrief(ParameterDigest{Parameters: []ParameterInfo{
		inputGain, inputPad, compressorParam("response", "Response", "Normal"),
		compressorParam("output", "Output Gain", "4.0"), compressorParam("mix", "Dry/Wet Mix", "100 %"),
		compressorParam("link", "Stereo Link", "On"),
	}}, []string{"activation_intensity"})
	if brief == nil || boundary != "" || len(brief.Controls) != 1 || brief.Controls[0].Role != "input_drive" || brief.Controls[0].CurrentText != "4.0" {
		t.Fatalf("continuous input drive was not selected: brief=%+v boundary=%q", brief, boundary)
	}
}

func TestCompressorSemanticSurfacesPreserveLimiterBoundary(t *testing.T) {
	digest := ParameterDigest{PluginName: "L2", Parameters: []ParameterInfo{
		compressorParam("threshold", "Limiter Threshold", "-6 dB"), compressorParam("ceiling", "Ceiling", "-0.1 dB"),
	}}
	if card, boundary := BuildAudioProcessorIdentityCard(digest); card != nil || boundary != "unsupported_limiter" {
		t.Fatalf("card=%+v boundary=%q", card, boundary)
	}
}
