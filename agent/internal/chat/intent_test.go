package chat

import "testing"

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

func TestSynthesizeLocalClipResizeDoesNotCatchMovePosition(t *testing.T) {
	cmds := synthesizeLocalDAWCommands("把选中的clip起点改成2秒", map[string]any{
		"selected_clip_id":       "1011",
		"selected_clip_track_id": "1007",
	})
	if len(cmds) != 1 || cmds[0]["tool"] != "clip.move" {
		t.Fatalf("cmds = %+v", cmds)
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
