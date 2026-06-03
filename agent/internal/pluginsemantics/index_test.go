package pluginsemantics

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBuildClassifiesMetadataCategories(t *testing.T) {
	idx := Build([]map[string]any{
		{"name": "TDR Nova", "manufacturer": "Tokyo Dawn Labs", "category": "Fx|EQ", "plugin_path": `C:\VST3\TDR Nova.vst3`},
		{"name": "SPAN", "manufacturer": "Voxengo", "category": "Fx|Analyzer", "plugin_path": `C:\VST3\SPAN.vst3`},
		{"name": "Surge XT", "manufacturer": "Surge Synth Team", "category": "Instrument|Synth", "plugin_path": `C:\VST3\Surge XT.vst3`, "is_instrument": true},
	}, time.Unix(10, 0).UTC())
	if len(idx.Entries) != 3 {
		t.Fatalf("entries = %d", len(idx.Entries))
	}
	got := map[string]string{}
	for _, entry := range idx.Entries {
		got[entry.Name] = entry.PrimaryType
	}
	if got["TDR Nova"] != "eq" {
		t.Fatalf("TDR Nova primary type = %q", got["TDR Nova"])
	}
	if got["SPAN"] != "analyzer" {
		t.Fatalf("SPAN primary type = %q", got["SPAN"])
	}
	if got["Surge XT"] != "synth" {
		t.Fatalf("Surge XT primary type = %q", got["Surge XT"])
	}
}

func TestSearchFindsValhallaForReverb(t *testing.T) {
	idx := Build([]map[string]any{
		{"name": "SPAN", "manufacturer": "Voxengo", "category": "Fx|Analyzer", "plugin_path": `C:\VST3\SPAN.vst3`},
		{"name": "ValhallaSupermassive", "manufacturer": "Valhalla DSP, LLC", "category": "Fx", "plugin_path": `C:\VST3\ValhallaSupermassive.vst3`},
		{"name": "Surge XT", "manufacturer": "Surge Synth Team", "category": "Instrument|Synth", "plugin_path": `C:\VST3\Surge XT.vst3`, "is_instrument": true},
	}, time.Unix(10, 0).UTC())
	results := Search(idx, SearchOptions{Query: "reverb", Limit: 5})
	if len(results) == 0 {
		t.Fatal("expected reverb result")
	}
	if results[0].Name != "ValhallaSupermassive" {
		t.Fatalf("top result = %+v", results[0])
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	idx := Build([]map[string]any{
		{"name": "TDR Nova", "manufacturer": "Tokyo Dawn Labs", "category": "Fx|EQ", "plugin_path": `C:\VST3\TDR Nova.vst3`},
	}, time.Unix(10, 0).UTC())
	path := filepath.Join(t.TempDir(), "plugin_semantics.json")
	if _, err := Save(path, idx); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Entries) != 1 || loaded.Entries[0].Name != "TDR Nova" {
		t.Fatalf("loaded = %+v", loaded)
	}
	if loaded.Summary["eq"] != 1 {
		t.Fatalf("summary = %+v", loaded.Summary)
	}
}
