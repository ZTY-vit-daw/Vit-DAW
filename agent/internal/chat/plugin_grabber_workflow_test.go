package chat

import (
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/pluginsemantics"
)

func TestSemanticPluginLoadFallbackFindsValhallaForReverb(t *testing.T) {
	candidates := semanticPluginLoadCandidates("reverb", "", []pluginLoadCandidate{
		{
			Name:         "SPAN",
			Manufacturer: "Voxengo",
			Category:     "Fx|Analyzer",
			Path:         `C:\Program Files\Common Files\VST3\SPAN.vst3`,
		},
		{
			Name:         "ValhallaSupermassive",
			Manufacturer: "Valhalla DSP, LLC",
			Category:     "Fx",
			Path:         `C:\Program Files\Common Files\VST3\ValhallaSupermassive.vst3`,
		},
		{
			Name:         "Surge XT",
			Manufacturer: "Surge Synth Team",
			Category:     "Instrument|Synth",
			Path:         `C:\Program Files\Common Files\VST3\Surge XT.vst3`,
			IsInstrument: true,
		},
	})
	if len(candidates) == 0 {
		t.Fatal("expected semantic reverb fallback candidate")
	}
	if candidates[0].Name != "ValhallaSupermassive" {
		t.Fatalf("top candidate = %+v", candidates[0])
	}
}

func TestPluginGrabberMappingNeedsDisplayReviewWhenUserReviewIsMissing(t *testing.T) {
	mapping := map[string]any{
		"confirmed":        false,
		"missing_evidence": []string{"user_review"},
		"display_domain": map[string]any{
			"status":     "inferred",
			"confidence": 0.95,
		},
	}
	if !pluginGrabberMappingNeedsDisplayReview(mapping) {
		t.Fatalf("unconfirmed mapping with missing user review was hidden: %+v", mapping)
	}
	mapping["confirmed"] = true
	if pluginGrabberMappingNeedsDisplayReview(mapping) {
		t.Fatalf("confirmed mapping should not be reviewed again: %+v", mapping)
	}
}

func TestPluginGrabberDisplayDomainFormKeepsSubmittedSlotsStable(t *testing.T) {
	payload := map[string]any{
		"profile_patch": map[string]any{
			"groups": []any{map[string]any{
				"id": "b1", "label": "B1",
				"params": map[string]any{
					"gain": map[string]any{
						"param_id": "2", "confirmed": false, "missing_evidence": []string{"user_review"},
						"display_domain_text": "-18~18 dB", "display_domain": map[string]any{"status": "inferred", "confidence": 0.95},
					},
					"frequency": map[string]any{
						"param_id": "4", "confirmed": false, "missing_evidence": []string{"user_review"},
						"display_domain_text": "10~40000 Hz", "display_domain": map[string]any{"status": "inferred", "confidence": 0.95},
					},
				},
			}},
		},
	}
	fields := pluginGrabberDisplayDomainFields(payload)
	if len(fields) != 2 {
		t.Fatalf("fields = %+v", fields)
	}
	if fields[0].Payload["slot"] != "frequency" || fields[1].Payload["slot"] != "gain" {
		t.Fatalf("field order must be deterministic: %+v", fields)
	}
	submitted := map[string]any{
		fields[0].ID: "10~40000 Hz",
		fields[1].ID: "-18~18 dB",
	}
	reviews := pluginGrabberDisplayDomainReviews(payload, submitted)
	if len(reviews) != 2 || reviews[0]["slot"] != "frequency" || reviews[0]["display_domain_text"] != "10~40000 Hz" || reviews[1]["slot"] != "gain" || reviews[1]["display_domain_text"] != "-18~18 dB" {
		t.Fatalf("submitted field bindings changed: %+v", reviews)
	}
}

func TestRankPluginLoadCandidatesPrefersInstrumentForInstrumentIntent(t *testing.T) {
	candidates := rankPluginLoadCandidates("Surge XT", "load an instrument", []pluginLoadCandidate{
		{
			Name:     "Surge XT Effects",
			Category: "Fx|Effects",
			Path:     `C:\Program Files\Common Files\VST3\Surge XT Effects.vst3`,
		},
		{
			Name:         "Surge XT",
			Manufacturer: "Surge Synth Team",
			Category:     "Instrument|Synth",
			Path:         `C:\Program Files\Common Files\VST3\Surge XT.vst3`,
			IsInstrument: true,
		},
	})
	if len(candidates) == 0 {
		t.Fatal("expected ranked candidates")
	}
	if candidates[0].Name != "Surge XT" || !candidates[0].IsInstrument {
		t.Fatalf("top candidate = %+v", candidates[0])
	}
}

func TestSemanticIndexPluginLoadCandidates(t *testing.T) {
	idx := pluginsemantics.Build([]map[string]any{
		{
			"name":         "ValhallaSupermassive",
			"manufacturer": "Valhalla DSP, LLC",
			"category":     "Fx",
			"plugin_path":  `C:\Program Files\Common Files\VST3\ValhallaSupermassive.vst3`,
		},
	}, time.Unix(10, 0).UTC())
	path := filepath.Join(t.TempDir(), "plugin_semantics.json")
	if _, err := pluginsemantics.Save(path, idx); err != nil {
		t.Fatalf("save index: %v", err)
	}
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", path)

	candidates, err := semanticIndexPluginLoadCandidates("reverb", "")
	if err != nil {
		t.Fatalf("semantic index candidates: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected semantic index candidate")
	}
	if candidates[0].Name != "ValhallaSupermassive" || candidates[0].Path == "" {
		t.Fatalf("candidate = %+v", candidates[0])
	}
}

func TestMergePluginLoadCandidatesKeepsLiveSearchCandidates(t *testing.T) {
	merged := mergePluginLoadCandidates([]pluginLoadCandidate{
		{
			Name:     "Live Compressor",
			Category: "Fx|Dynamics",
			Path:     `VST3-Live Compressor-1234`,
		},
	}, []pluginLoadCandidate{
		{
			Name:     "Old Reverb",
			Category: "Fx|Reverb",
			Path:     `C:\Program Files\Common Files\VST3\Old Reverb.vst3`,
		},
	})
	ranked := rankPluginLoadCandidates("compressor", "effect", merged)
	if len(ranked) == 0 {
		t.Fatal("expected ranked candidates")
	}
	if ranked[0].Name != "Live Compressor" {
		t.Fatalf("top candidate = %+v", ranked[0])
	}
}
