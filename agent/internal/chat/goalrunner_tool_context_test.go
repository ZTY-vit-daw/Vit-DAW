package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/tools"
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

func TestAgentLoopToolContextExposesEqualizerCapabilityBridge(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}
	ctx := s.agentLoopToolContext(agentModeDefault, "我想通过当前加载的均衡器调节3400Hz频段增益3dB", map[string]any{"selected_track_id": "1007", "selected_plugin_id": "1013"})
	for _, want := range []string{"capability.equalizer.inspect", "capability.equalizer.plan"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from equalizer capability context: %+v", want, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "capability_equalizer_plan tool=capability.equalizer.plan") {
		t.Fatalf("equalizer capability contract missing from catalog:\n%s", ctx.CatalogSummary)
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

func TestAgentLoopToolContextRoutesTrackFolderOrganizationToTrackPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "\u5c31\u7528\u4f60\u7684\u5efa\u8bae\u5206\u7ec4\u5e2e\u6211\u6574\u7406\u6210\u6587\u4ef6\u5939", nil)

	for _, want := range []string{"project.apply_track_organization", "track.folder.create", "track.move_to_folder"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from track organization context: %+v\nsummary:\n%s", want, ctx.AllowedTools, ctx.CatalogSummary)
		}
	}
	for _, unwanted := range []string{"project.import_folder_as_stems", "media.index_authorized_folder"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("track organization leaked %s into tool context: %+v", unwanted, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "project.apply_track_organization") || !strings.Contains(ctx.CatalogSummary, "track.folder.create") {
		t.Fatalf("track organization guidance missing folder tools:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextRoutesCreateTrackFolderToTrackPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "\u5e2e\u6211\u65b0\u5efa\u4e00\u4e2a\u8f68\u9053\u6587\u4ef6\u5939", nil)

	if !containsToolName(ctx.AllowedTools, "track.folder.create") {
		t.Fatalf("track.folder.create missing from create folder context: %+v\nsummary:\n%s", ctx.AllowedTools, ctx.CatalogSummary)
	}
	if containsToolName(ctx.AllowedTools, "project.import_folder_as_stems") {
		t.Fatalf("create track folder leaked project folder import tools: %+v", ctx.AllowedTools)
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

func TestAgentLoopToolContextRoutesB1GainStagingToStaticMixPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	cases := []string{
		"\u5e2e\u6211\u6267\u884cB1",
		"\u8fdb\u884cB1.2",
		"\u68c0\u67e5 B1 \u589e\u76ca\u7ed3\u6784\u548c headroom \u662f\u5426\u5065\u5eb7",
		"\u5e2e\u6211\u8fdb\u884cB1\u9636\u6bb5\u7684\u97f3\u91cf\u68c0\u67e5",
		"\u6211\u60f3\u8ba9\u4f60\u5bf9\u6574\u4e2a\u5de5\u7a0b\u7684\u7535\u5e73\u8fdb\u884c\u68c0\u67e5\u505a\u5f97\u5230\u5417\uff1f",
	}
	for _, userText := range cases {
		ctx := s.agentLoopToolContext(agentModeDefault, userText, nil)

		for _, want := range []string{"project.audio_analysis_status", "mix.observe", "mix.read", "clip.gain.read", "clip.gain.set", "clip.gain.set_batch", "track.group.list", "track.group.apply_control"} {
			if !containsToolName(ctx.AllowedTools, want) {
				t.Fatalf("%s missing from B1 gain staging context for %q: %+v\nsummary:\n%s", want, userText, ctx.AllowedTools, ctx.CatalogSummary)
			}
		}
		for _, unwanted := range []string{"mix.propose_tick", "mix.apply_tick", "track.volume", "plugin.set_parameter", "daw.invoke", "track.group.create"} {
			if containsToolName(ctx.AllowedTools, unwanted) {
				t.Fatalf("B1 gain staging leaked %s for %q: %+v", unwanted, userText, ctx.AllowedTools)
			}
		}
		if !strings.Contains(ctx.CatalogSummary, "Capability pack: static_mix_gain_staging") || !strings.Contains(ctx.CatalogSummary, "static_mix.gain_staging.v0") {
			t.Fatalf("B1 gain staging did not receive static mix capability pack for %q:\n%s", userText, ctx.CatalogSummary)
		}
	}
}

func TestAgentLoopToolContextRoutesB2WithoutB1MutationTools(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}
	ctx := s.agentLoopToolContext(agentModeDefault, "基于 B1 后的健康基准，按现代流行做 B2 静态平衡", nil)
	for _, want := range []string{"project.state", "mix.observe", "mix.propose_tick", "mix.apply_tick", "mix.rollback_tick"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from B2 context: %+v", want, ctx.AllowedTools)
		}
	}
	for _, unwanted := range []string{"clip.gain.read", "clip.gain.set", "clip.gain.set_batch", "track.group.apply_control", "track.volume", "track.pan", "plugin.set_parameter"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("B2 leaked %s: %+v", unwanted, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "Capability pack: static_mix_static_balance") || !strings.Contains(ctx.CatalogSummary, "vit.mix_style.v1") {
		t.Fatalf("B2 capability pack missing:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextRoutesB3ToDedicatedPanLayoutPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}
	for _, userText := range []string{"帮我进行 B3", "为全工程做静态声像布局", "build a full project pan layout"} {
		ctx := s.agentLoopToolContext(agentModeDefault, userText, nil)
		for _, want := range []string{"project.state", "project.audio_analysis_status", "mix.observe", "mix.read", "mix.derive", "mix.apply_pan_layout_batch"} {
			if !containsToolName(ctx.AllowedTools, want) {
				t.Fatalf("%s missing from B3 context for %q: %+v", want, userText, ctx.AllowedTools)
			}
		}
		for _, unwanted := range []string{"mix.propose_tick", "mix.apply_tick", "track.pan", "track.volume", "clip.gain.set", "plugin.set_parameter"} {
			if containsToolName(ctx.AllowedTools, unwanted) {
				t.Fatalf("B3 leaked %s for %q: %+v", unwanted, userText, ctx.AllowedTools)
			}
		}
		if !strings.Contains(ctx.CatalogSummary, "Capability pack: static_mix_pan_layout") || !strings.Contains(ctx.CatalogSummary, "pan_layout.model.v1") || !strings.Contains(ctx.CatalogSummary, "vit.mix_style.v1") {
			t.Fatalf("B3 capability pack missing for %q:\n%s", userText, ctx.CatalogSummary)
		}
	}
}

func TestAgentLoopToolContextKeepsLocalPanMoveInGenericMixPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}
	ctx := s.agentLoopToolContext(agentModeDefault, "Move the guitar slightly left", map[string]any{"selected_track_id": "guitar"})
	if !containsToolName(ctx.AllowedTools, "mix.propose_tick") || !containsToolName(ctx.AllowedTools, "mix.apply_tick") {
		t.Fatalf("local pan move did not keep generic mix tools: %+v", ctx.AllowedTools)
	}
	if containsToolName(ctx.AllowedTools, "mix.apply_pan_layout_batch") || strings.Contains(ctx.CatalogSummary, "Capability pack: static_mix_pan_layout") {
		t.Fatalf("local pan move was incorrectly routed to B3: %+v\n%s", ctx.AllowedTools, ctx.CatalogSummary)
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

func TestAgentLoopToolContextRoutesProjectAudioSettingsToPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "\u8bf7\u67e5\u770b\u5de5\u7a0b\u91c7\u6837\u7387\u548c\u5f55\u97f3\u4f4d\u6df1", nil)

	for _, want := range []string{"project.get_audio_settings", "project.validate_audio_settings_change", "project.set_audio_settings"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from project audio context: %+v", want, ctx.AllowedTools)
		}
	}
	for _, unwanted := range []string{"daw.invoke", "clip.import_audio", "plugin.search"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("project audio request leaked %s into tool context: %+v", unwanted, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "Capability pack: project_audio") || !strings.Contains(ctx.CatalogSummary, "project.get_audio_settings") {
		t.Fatalf("project audio request did not receive project_audio capability pack:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextRoutesSectionMarkerRequestToProjectMarkerPack(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "确认把 A5 段落地图写入 marker", nil)

	for _, want := range []string{"project.markers.apply_section_markers", "project.markers.list", "project.markers.upsert"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from project marker context: %+v", want, ctx.AllowedTools)
		}
	}
	for _, unwanted := range []string{"project.import_folder_as_stems", "plugin.search"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("project marker request leaked %s into tool context: %+v", unwanted, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "Capability pack: project_marker") || !strings.Contains(ctx.CatalogSummary, "project.markers.apply_section_markers") {
		t.Fatalf("project marker request did not receive marker capability pack:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextRoutesStemsPreflightWithoutSingleTrackImport(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "\u5bf9\u8fd9\u4e2a stems \u6587\u4ef6\u5939\u505a\u5bfc\u5165\u9884\u68c0\uff0c\u5148\u751f\u6210\u5bfc\u5165\u8ba1\u5212", nil)

	for _, want := range []string{"project.import_preflight", "project.import_folder_as_stems", "media.inspect_files", "project.get_audio_settings"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from stems preflight context: %+v", want, ctx.AllowedTools)
		}
	}
	for _, unwanted := range []string{"daw.invoke", "clip.import_audio", "clip.import_media_to_track"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("stems preflight leaked %s into tool context: %+v", unwanted, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "do not simulate folder import by repeated clip.import_audio") {
		t.Fatalf("stems preflight guidance missing single-track import guard:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextRoutesChineseMultitrackFolderImportToProjectAudioOnly(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "\u5e2e\u6211\u5bfc\u5165\u8fd9\u4e2a\u5de5\u7a0b\"E:\\BaiduNetdiskDownload\\yingge - sattelites tracks out\"", nil)

	for _, want := range []string{"project.import_preflight", "project.import_folder_as_stems", "media.inspect_files", "project.get_audio_settings"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from Chinese multitrack import context: %+v", want, ctx.AllowedTools)
		}
	}
	for _, unwanted := range []string{"daw.invoke", "track.add", "track.add_audio", "clip.import_audio", "clip.import_media_to_track", "media.index_authorized_folder"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("Chinese multitrack import leaked %s into tool context: %+v", unwanted, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "project.import_preflight") || !strings.Contains(ctx.CatalogSummary, "project.import_folder_as_stems") {
		t.Fatalf("Chinese multitrack import did not receive project_audio import guidance:\n%s", ctx.CatalogSummary)
	}
}

func TestAgentLoopToolContextKeepsSingleAudioProjectImportOnClipTools(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "\u5e2e\u6211\u628a E:\\Samples\\Lead Vocal.wav \u8fd9\u4e2a\u97f3\u9891\u5bfc\u5165\u5de5\u7a0b", nil)

	for _, want := range []string{"clip.import_media_to_track", "clip.import_audio"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from single audio import context: %+v", want, ctx.AllowedTools)
		}
	}
	for _, unwanted := range []string{"project.import_preflight", "project.import_folder_as_stems"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("single audio import leaked %s into tool context: %+v", unwanted, ctx.AllowedTools)
		}
	}
}

func TestAgentLoopToolContextKeepsDraggedAudioProjectImportOnClipTools(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "\u5e2e\u6211\u628a\u8fd9\u4e2a\u97f3\u9891\u5bfc\u5165\u5de5\u7a0b", map[string]any{
		"attachment_import_file_path": `D:\Samples\Loop.mp3`,
		"attachment_import_kind":      "audio",
		"selected_attachment_kind":    "audio",
	})

	if !containsToolName(ctx.AllowedTools, "clip.import_media_to_track") {
		t.Fatalf("dragged audio import missing clip.import_media_to_track: %+v", ctx.AllowedTools)
	}
	for _, unwanted := range []string{"project.import_preflight", "project.import_folder_as_stems"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("dragged audio import leaked %s into tool context: %+v", unwanted, ctx.AllowedTools)
		}
	}
}

func TestAgentLoopToolContextRoutesMediaFolderInspectionToMediaPool(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "\u770b\u770b E:\\BaiduNetdiskDownload\\yingge - sattelites tracks out \u8fd9\u4e2a\u6587\u4ef6\u5939\u91cc\u6709\u4ec0\u4e48\u97f3\u9891\u7d20\u6750", nil)

	if !containsToolName(ctx.AllowedTools, "media.index_authorized_folder") {
		t.Fatalf("media folder inspection missing media.index_authorized_folder: %+v", ctx.AllowedTools)
	}
	if !containsToolName(ctx.AllowedTools, "media.register_assets") {
		t.Fatalf("media folder inspection missing media.register_assets: %+v", ctx.AllowedTools)
	}
}

func TestAgentLoopToolContextClipFadeGainIncludesTypedClipTools(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "clip fade gain", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
	})

	for _, want := range []string{"clip.fade.read", "clip.gain.read", "clip.fade.set", "clip.gain.set"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from clip fade/gain context: %+v", want, ctx.AllowedTools)
		}
	}
	for _, unwanted := range []string{"mix.observe", "mix.apply_tick"} {
		if containsToolName(ctx.AllowedTools, unwanted) {
			t.Fatalf("clip fade/gain context leaked %s: %+v", unwanted, ctx.AllowedTools)
		}
	}
}

func TestAgentLoopToolContextStripSilenceIncludesTypedClipTools(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}

	ctx := s.agentLoopToolContext(agentModeDefault, "对选中范围做 strip silence 片段清理", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
		"selected_clip_ranges": []any{
			map[string]any{
				"range_id":         "range_1",
				"clip_id":          "1011",
				"track_id":         "1007",
				"start_seconds":    1.0,
				"end_seconds":      2.0,
				"duration_seconds": 1.0,
			},
		},
	})

	for _, want := range []string{"clip.strip_silence.analyze", "clip.strip_silence.suggest", "clip.strip_silence.apply", "clip.strip_silence.apply_batch"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from strip silence context: %+v", want, ctx.AllowedTools)
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

func TestAgentLoopNeverExposesRawPluginParameterMutation(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}
	for _, tc := range []struct {
		name string
		ctx  agentLoopToolContext
	}{
		{name: "broad", ctx: s.agentLoopToolContext(agentModeDefault, "inspect the project", nil)},
		{name: "plugin", ctx: s.agentLoopToolContext(agentModeDefault, "adjust the selected TDR Nova EQ", map[string]any{"selected_plugin_name": "TDR Nova"})},
	} {
		if containsToolName(tc.ctx.AllowedTools, "plugin.set_parameter") {
			t.Fatalf("%s model tools expose plugin.set_parameter: %#v", tc.name, tc.ctx.AllowedTools)
		}
	}
	if _, ok := tools.DefaultCatalog().LookupTool("plugin.set_parameter"); !ok {
		t.Fatal("internal catalog must retain plugin.set_parameter for compensation/internal execution")
	}
}
