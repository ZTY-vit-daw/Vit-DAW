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
