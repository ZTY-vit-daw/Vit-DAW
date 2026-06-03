package chat

import "testing"

func TestSynthesizeLocalAddTrackCommands(t *testing.T) {
	cases := []string{
		"新建一条轨道",
		"新建一个轨道",
		"添加一个音频轨道",
		"add a new track",
	}
	for _, text := range cases {
		cmds := synthesizeLocalDAWCommands(text, map[string]any{})
		if len(cmds) != 1 {
			t.Fatalf("%q cmds = %+v", text, cmds)
		}
		if cmds[0]["tool"] != "track.add" {
			t.Fatalf("%q tool = %#v", text, cmds[0]["tool"])
		}
	}
}

func TestSynthesizeLocalAddTrackIgnoresPluginAndImportRequests(t *testing.T) {
	for _, text := range []string{
		"加载混响到当前轨道",
		"add reverb to the current track",
		"添加音频到当前轨道",
	} {
		if cmds := synthesizeLocalDAWCommands(text, map[string]any{"selected_track_id": "1007"}); len(cmds) != 0 {
			t.Fatalf("%q should not synthesize track.add: %+v", text, cmds)
		}
	}
}

func TestSynthesizeLocalTrackMuteCommands(t *testing.T) {
	cases := []struct {
		text  string
		want  bool
		index int
	}{
		{"静音当前轨道", true, 0},
		{"静音轨道", true, 0},
		{"mute current track", true, 0},
		{"取消静音", false, 0},
		{"取消mute", false, 0},
		{"unmute track 1", false, 1},
	}
	for _, tc := range cases {
		cmds := synthesizeLocalDAWCommands(tc.text, map[string]any{
			"selected_track_id": "1007",
		})
		if len(cmds) != 1 {
			t.Fatalf("%q cmds = %+v", tc.text, cmds)
		}
		if cmds[0]["tool"] != "track.mute" {
			t.Fatalf("%q tool = %#v", tc.text, cmds[0]["tool"])
		}
		args := cmds[0]["args"].(map[string]any)
		if args["mute"] != tc.want {
			t.Fatalf("%q args = %+v", tc.text, args)
		}
		if tc.index > 0 {
			if args["user_track_index"] != tc.index {
				t.Fatalf("%q args = %+v", tc.text, args)
			}
			if _, ok := args["track_id"]; ok {
				t.Fatalf("%q explicit track index should override selected track: %+v", tc.text, args)
			}
		} else if args["track_id"] != "1007" {
			t.Fatalf("%q args = %+v", tc.text, args)
		}
	}
}

func TestSynthesizeLocalTrackMuteCommandByUserTrackIndex(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("取消静音 track 1", map[string]any{
		"selected_track_id": "1010",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	args := cmds[0]["args"].(map[string]any)
	if args["mute"] != false || args["user_track_index"] != 1 {
		t.Fatalf("args = %+v", args)
	}
	if _, ok := args["track_id"]; ok {
		t.Fatalf("explicit track index should override selected track: %+v", args)
	}
}

func TestSynthesizeLocalTrackSoloCommands(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"solo 当前轨道", true},
		{"取消solo", false},
		{"取消独奏", false},
	}
	for _, tc := range cases {
		cmds := synthesizeLocalDAWCommands(tc.text, map[string]any{
			"selected_track_id": "1007",
		})
		if len(cmds) != 1 {
			t.Fatalf("%q cmds = %+v", tc.text, cmds)
		}
		if cmds[0]["tool"] != "track.solo" {
			t.Fatalf("%q tool = %#v", tc.text, cmds[0]["tool"])
		}
		args := cmds[0]["args"].(map[string]any)
		if args["solo"] != tc.want || args["track_id"] != "1007" {
			t.Fatalf("%q args = %+v", tc.text, args)
		}
	}
}

func TestSynthesizeLocalClipMoveCommand(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("将选中的clip移动20s", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.move" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_id"] != "1011" || args["source_track_id"] != "1007" || args["target_track_id"] != "1007" {
		t.Fatalf("args = %+v", args)
	}
	if args["new_start"] != 20.0 || args["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", args)
	}
}

func TestSynthesizeLocalClipRemoveCommand(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("删除选中的clip", map[string]any{
		"selected_clip_ids": []any{"1011"},
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.remove" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	ids := args["clip_ids"].([]string)
	if len(ids) != 1 || ids[0] != "1011" {
		t.Fatalf("clip_ids = %#v", ids)
	}
}

func TestSynthesizeLocalClipRemoveThisAudio(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("把这个音频删掉", map[string]any{
		"selected_clip_ids": []any{"1011"},
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.remove" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	ids := args["clip_ids"].([]string)
	if len(ids) != 1 || ids[0] != "1011" {
		t.Fatalf("clip_ids = %#v", ids)
	}
}

func TestSynthesizeLocalClipSelectCommandByName(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("select clip Loop A", map[string]any{
		"selected_clip_track_id": "1007",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.select" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_name"] != "Loop A" || args["track_id"] != "1007" {
		t.Fatalf("args = %+v", args)
	}
}

func TestSynthesizeLocalClipResizeCommand(t *testing.T) {
	cases := []string{
		"把选中的clip裁到2秒",
		"把选中的clip改成2s",
		"选中的clip时长改为2秒",
	}
	for _, text := range cases {
		cmds := synthesizeLocalDAWCommands(text, map[string]any{
			"selected_clip_id":       "1011",
			"selected_clip_track_id": "1007",
		})
		if len(cmds) != 1 {
			t.Fatalf("%q cmds = %+v", text, cmds)
		}
		if cmds[0]["tool"] != "clip.resize" {
			t.Fatalf("%q tool = %#v", text, cmds[0]["tool"])
		}
		args := cmds[0]["args"].(map[string]any)
		if args["clip_id"] != "1011" || args["track_id"] != "1007" {
			t.Fatalf("%q args = %+v", text, args)
		}
		if args["new_length"] != 2.0 || args["time_unit"] != "seconds" {
			t.Fatalf("%q time args = %+v", text, args)
		}
	}
}

func TestSynthesizeLocalClipResizeThisAudio(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("把这个音频裁到3秒", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.resize" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_id"] != "1011" || args["track_id"] != "1007" || args["new_length"] != 3.0 {
		t.Fatalf("args = %+v", args)
	}
}

func TestSynthesizeLocalClipResizeDoesNotCatchMovePosition(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("把选中的clip起点改成2秒", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
	})
	if len(cmds) != 1 || cmds[0]["tool"] != "clip.move" {
		t.Fatalf("cmds = %+v", cmds)
	}
}

func TestSynthesizeLocalClipMoveCommandToPlayhead(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("把这个音频移动到播放头", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       4.5,
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.move" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_id"] != "1011" || args["source_track_id"] != "1007" || args["target_track_id"] != "1007" {
		t.Fatalf("args = %+v", args)
	}
	if args["new_start"] != 4.5 || args["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", args)
	}
}

func TestSynthesizeLocalClipSplitCommandAtTime(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("把选中的clip在1秒处切开", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.split" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_id"] != "1011" || args["track_id"] != "1007" {
		t.Fatalf("args = %+v", args)
	}
	if args["split_time"] != 1.0 || args["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", args)
	}
}

func TestSynthesizeLocalClipSplitThisAudioAtPlayhead(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("在这里切开这个音频", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       1.25,
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.split" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_id"] != "1011" || args["track_id"] != "1007" || args["playhead_seconds"] != "1.25" {
		t.Fatalf("args = %+v", args)
	}
}

func TestSynthesizeLocalClipSplitCommandAtPlayhead(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("在播放头这里切开选中的clip", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       1.25,
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_id"] != "1011" || args["track_id"] != "1007" || args["playhead_seconds"] != "1.25" {
		t.Fatalf("args = %+v", args)
	}
}

func TestSynthesizeLocalClipCloneCommandAtTime(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("复制选中的clip到20s", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.clone" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_id"] != "1011" || args["target_track_id"] != "1007" {
		t.Fatalf("args = %+v", args)
	}
	if args["new_start"] != 20.0 || args["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", args)
	}
}

func TestSynthesizeLocalClipCloneCommandAfterSource(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("复制选中的clip到后面", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.clone" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_id"] != "1011" || args["target_track_id"] != "1007" {
		t.Fatalf("args = %+v", args)
	}
	if _, ok := args["new_start"]; ok {
		t.Fatalf("new_start should be inferred from project state by harness: %+v", args)
	}
}

func TestSynthesizeLocalClipCloneThisAudioToPlayhead(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("把这个音频复制到播放头", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       6.25,
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.clone" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_id"] != "1011" || args["target_track_id"] != "1007" {
		t.Fatalf("args = %+v", args)
	}
	if args["new_start"] != 6.25 || args["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", args)
	}
}

func TestSynthesizeLocalClipSelectThisClipUsesSelection(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("选中这个 clip", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.select" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["clip_id"] != "1011" || args["track_id"] != "1007" {
		t.Fatalf("args = %+v", args)
	}
	if _, ok := args["clip_name"]; ok {
		t.Fatalf("should not infer clip_name for current clip reference: %+v", args)
	}
}

func TestSynthesizeLocalClipSelectCurrentTrackOneClipUsesTrackScope(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("选中当前轨道的一个clip", map[string]any{
		"selected_track_id": "1007",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.select" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["track_id"] != "1007" {
		t.Fatalf("args = %+v", args)
	}
	if _, ok := args["clip_name"]; ok {
		t.Fatalf("generic reference should not become clip_name: %+v", args)
	}
	if _, ok := args["clip_id"]; ok {
		t.Fatalf("generic track-scoped reference should be resolved by harness: %+v", args)
	}
}

func TestSynthesizeLocalClipSelectTrackIndexOneClipUsesTrackScope(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("选中track 1轨道的一个clip", map[string]any{
		"selected_clip_id": "1011",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	args := cmds[0]["args"].(map[string]any)
	if args["user_track_index"] != 1 {
		t.Fatalf("args = %+v", args)
	}
	if _, ok := args["clip_name"]; ok {
		t.Fatalf("generic reference should not become clip_name: %+v", args)
	}
	if _, ok := args["clip_id"]; ok {
		t.Fatalf("explicit track scope should be resolved by harness: %+v", args)
	}
}

func TestSynthesizeLocalImportAbsolutePathCommand(t *testing.T) {
	cmds := synthesizeLocalDAWCommands(`import C:\Samples\Kick Loop.wav to current track`, map[string]any{
		"selected_track_id": "1007",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.import_media_to_track" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["track_id"] != "1007" || args["file_path"] != `C:\Samples\Kick Loop.wav` {
		t.Fatalf("args = %+v", args)
	}
	if args["media_type"] != "audio" || args["mode"] != "non_destructive" || args["start_time"] != 0.0 {
		t.Fatalf("defaults = %+v", args)
	}
}

func TestSynthesizeLocalImportSelectedLibraryCommand(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("把资料库选中的音频导入当前轨道", map[string]any{
		"selected_track_id":          "1007",
		"selected_library_file_path": `D:\Samples\Loop.wav`,
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	args := cmds[0]["args"].(map[string]any)
	if args["track_id"] != "1007" || args["selected_library_file_path"] != `D:\Samples\Loop.wav` {
		t.Fatalf("args = %+v", args)
	}
}

func TestSynthesizeLocalImportAudioAttachmentCommand(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("把这个放到当前轨道", map[string]any{
		"selected_track_id":             "1007",
		"attachment_import_kind":        "audio",
		"attachment_import_file_path":   `D:\Samples\Loop.mp3`,
		"selected_attachment_file_path": `D:\Samples\Loop.mp3`,
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "clip.import_media_to_track" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["track_id"] != "1007" || args["file_path"] != `D:\Samples\Loop.mp3` {
		t.Fatalf("args = %+v", args)
	}
}

func TestSynthesizeLocalImportMidiAttachmentCommand(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("导入这个 MIDI", map[string]any{
		"selected_track_id":             "1007",
		"attachment_import_kind":        "midi",
		"attachment_import_file_path":   `D:\Samples\Part.mid`,
		"selected_attachment_file_path": `D:\Samples\Part.mid`,
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0]["tool"] != "midi.import_file" {
		t.Fatalf("tool = %#v", cmds[0]["tool"])
	}
	args := cmds[0]["args"].(map[string]any)
	if args["track_id"] != "1007" || args["file_path"] != `D:\Samples\Part.mid` {
		t.Fatalf("args = %+v", args)
	}
}

func TestSynthesizeLocalImportSearchCommand(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("search kick loop and import to current track", map[string]any{
		"selected_track_id": "1007",
	})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	args := cmds[0]["args"].(map[string]any)
	if args["track_id"] != "1007" || args["asset_query"] != "kick loop" {
		t.Fatalf("args = %+v", args)
	}
}
