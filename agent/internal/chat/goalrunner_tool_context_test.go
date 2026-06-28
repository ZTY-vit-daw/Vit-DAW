package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
)

func TestAgentLoopToolContextNarrowsTrackRequest(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "新建一条轨道", nil)

	if !containsToolName(ctx.AllowedTools, "track.add") {
		t.Fatalf("track.add missing from allowed tools: %+v", ctx.AllowedTools)
	}
	for _, unwanted := range []string{"midi.apply_note_patch", "plugin.search"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("unexpected %s in track tool context: %+v", unwanted, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "add_track tool=track.add") || strings.Contains(ctx.CatalogSummary, "plugin_search") {
		t.Fatalf("unexpected track catalog summary:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextIncludesMidiPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "输入 8 个 MIDI 音符", nil)

	for _, want := range []string{"midi.apply_note_patch", "midi.create_clip", "track.add"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from MIDI tool context: %+v", want, ctx.AllowedTools)
		}
	}
	if containsToolName(ctx.AllowedTools, "plugin.search") {
		t.Fatalf("plugin.search should not be in MIDI-only context: %+v", ctx.AllowedTools)
	}
	if !strings.Contains(ctx.CatalogSummary, "apply_midi_note_patch tool=midi.apply_note_patch") {
		t.Fatalf("MIDI catalog summary missing note patch:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextRoutesRealChineseTrackRequestToPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "\u65b0\u5efa\u4e00\u6761\u8f68\u9053", nil)

	if !containsToolName(ctx.AllowedTools, "track.add") {
		t.Fatalf("track.add missing from allowed tools: %+v", ctx.AllowedTools)
	}
	if containsToolName(ctx.AllowedTools, "plugin.search") || containsToolName(ctx.AllowedTools, "midi.apply_note_patch") {
		t.Fatalf("track request leaked unrelated tools: %+v", ctx.AllowedTools)
	}
	if !strings.Contains(ctx.CatalogSummary, "Capability pack: track") || !strings.Contains(ctx.CatalogSummary, "State slices:") {
		t.Fatalf("track request did not receive track capability pack:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextRoutesRealChineseMidiRequestToPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "\u8f93\u5165 8 \u4e2a MIDI \u97f3\u7b26", nil)

	for _, want := range []string{"midi.apply_note_patch", "midi.create_clip", "track.add"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from MIDI tool context: %+v", want, ctx.AllowedTools)
		}
	}
	if containsToolName(ctx.AllowedTools, "plugin.search") {
		t.Fatalf("MIDI request leaked plugin tools: %+v", ctx.AllowedTools)
	}
	if !strings.Contains(ctx.CatalogSummary, "Capability pack: midi") || !strings.Contains(ctx.CatalogSummary, "midi.create_clip") {
		t.Fatalf("MIDI request did not receive MIDI capability pack:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextRoutesPanRequestToMixPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "Track 2 pan left a little", nil)

	for _, want := range []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from pan mix tool context: %+v", want, ctx.AllowedTools)
		}
	}
	for _, unwanted := range []string{"daw.invoke", "track.pan", "plugin.set_parameter"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("pan request leaked %s into tool context: %+v", unwanted, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "Capability pack: mix") || !strings.Contains(ctx.CatalogSummary, "mix.observe") {
		t.Fatalf("pan request did not receive mix capability pack:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextRoutesChineseVolumeMixRequestToMixPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "这条轨道太响了，稍微压低一点", map[string]any{"selected_track_id": "1007"})

	for _, want := range []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from Chinese volume mix tool context: %+v", want, ctx.AllowedTools)
		}
	}
	for _, unwanted := range []string{"track.volume", "plugin.set_parameter", "daw.invoke"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("Chinese volume mix request leaked %s into tool context: %+v", unwanted, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "Capability pack: mix") || !strings.Contains(ctx.CatalogSummary, "mix.observe") {
		t.Fatalf("Chinese volume mix request did not receive mix capability pack:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextRoutesAcousticObservationRequestsToMixPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	cases := []string{
		"\u5e2e\u6211\u770b\u4e00\u4e0b\u9891\u8c31",
		"\u9700\u8981\u7528\u58f0\u5b66\u89c2\u5bdf\u5668\u8bfb\u53d6\u9891\u8c31",
		"\u4e3a\u4ec0\u4e48\u6ca1\u6709\u83b7\u53d6\u5230\u5176\u5b83\u58f0\u5b66\u6570\u636e",
		"please run mix.observe and read the spectral band projection",
	}
	for _, userText := range cases {
		ctx := s.agentLoopToolContext(agentModeDefault, userText, nil)
		for _, want := range []string{"mix.observe", "mix.read", "mix.derive"} {
			if !containsToolName(ctx.AllowedTools, want) {
				t.Fatalf("%s missing from acoustic observation context for %q: %+v", want, userText, ctx.AllowedTools)
			}
		}
		if containsToolName(ctx.AllowedTools, "plugin.set_parameter") || containsToolName(ctx.AllowedTools, "daw.invoke") {
			t.Fatalf("acoustic observation request leaked mutation-oriented tools for %q: %+v", userText, ctx.AllowedTools)
		}
		if !strings.Contains(ctx.CatalogSummary, "Capability pack: mix") || !strings.Contains(ctx.CatalogSummary, "mix.observe") {
			t.Fatalf("acoustic observation request did not receive mix capability pack for %q:\n%s", userText, ctx.CatalogSummary)
		}
	}
}

func containsToolName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
