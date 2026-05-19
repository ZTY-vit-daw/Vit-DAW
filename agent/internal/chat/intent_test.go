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
