package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/policy"
)

func TestExecutedReplyListsTrackNamesOnly(t *testing.T) {
	after := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Track 1"},
		},
	}

	reply := executedReply(nil, after, []policy.Decision{
		{Name: "list_tracks", Risk: policy.RiskDirect, Command: map[string]any{"cmd": "list_tracks"}},
	}, nil)

	if reply != "当前有 1 条轨道：Track 1。" {
		t.Fatalf("reply = %q", reply)
	}
	if strings.Contains(reply, "1007") || strings.Contains(reply, "即将") {
		t.Fatalf("reply leaked internal wording/id: %q", reply)
	}
}

func TestExecutedReplyRenamesTrackAfterExecution(t *testing.T) {
	before := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Track 1"},
		},
	}
	after := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Lead Vocal"},
		},
	}

	reply := executedReply(before, after, []policy.Decision{
		{
			Name:    "rename_track",
			Risk:    policy.RiskUndoable,
			Command: map[string]any{"cmd": "rename_track", "track_id": "1007", "name": "Lead Vocal"},
		},
	}, nil)

	if reply != "已把 Track 1 重命名为 Lead Vocal，可撤销。" {
		t.Fatalf("reply = %q", reply)
	}
	if strings.Contains(reply, "1007") || strings.Contains(reply, "即将") {
		t.Fatalf("reply leaked internal wording/id: %q", reply)
	}
}

func TestExecutedReplyRenamesTrackFromKernelReply(t *testing.T) {
	reply := executedReply(nil, nil, []policy.Decision{
		{
			Name:    "rename_track",
			Risk:    policy.RiskUndoable,
			Command: map[string]any{"cmd": "rename_track", "track_id": "1007", "name": "A"},
		},
	}, []map[string]any{
		{
			"result": map[string]any{
				"old_name":   "Track 1",
				"track_name": "A",
			},
		},
	})

	if reply != "已把 Track 1 重命名为 A，可撤销。" {
		t.Fatalf("reply = %q", reply)
	}
}

func TestExecutedReplyUsesKernelMuteStateForCancel(t *testing.T) {
	before := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Track 1"},
		},
	}

	reply := executedReply(before, before, []policy.Decision{
		{
			Name: "set_mute",
			Risk: policy.RiskUndoable,
			Command: map[string]any{
				"tool": "track.mute",
				"args": map[string]any{
					"track_id": "1007",
					"enabled":  true,
				},
			},
		},
	}, []map[string]any{
		{
			"result": map[string]any{
				"track_id": "1007",
				"mute":     false,
			},
		},
	})

	if !strings.Contains(reply, "已取消 Track 1 的静音") {
		t.Fatalf("reply = %q", reply)
	}
	if strings.Contains(reply, "1007") {
		t.Fatalf("reply leaked track id: %q", reply)
	}
}

func TestExecutedReplyExpandsToolArgsForSoloCancel(t *testing.T) {
	before := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Track 1"},
		},
	}

	reply := executedReply(before, before, []policy.Decision{
		{
			Name: "set_solo",
			Risk: policy.RiskUndoable,
			Command: map[string]any{
				"tool": "track.solo",
				"args": map[string]any{
					"track_id": "1007",
					"enabled":  false,
				},
			},
		},
	}, nil)

	if !strings.Contains(reply, "已取消 Track 1 的独奏") {
		t.Fatalf("reply = %q", reply)
	}
	if strings.Contains(reply, "1007") {
		t.Fatalf("reply leaked track id: %q", reply)
	}
}

func TestSanitizeUserReplyReplacesTrackIDsUnlessAsked(t *testing.T) {
	state := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Lead Vocal"},
		},
	}

	reply := sanitizeUserReply("当前轨道是 1007。", state, "列出当前轨道")
	if reply != "当前轨道是 Lead Vocal。" {
		t.Fatalf("reply = %q", reply)
	}

	technical := sanitizeUserReply("当前轨道 ID 是 1007。", state, "轨道 ID 是多少")
	if technical != "当前轨道 ID 是 1007。" {
		t.Fatalf("technical reply = %q", technical)
	}
}
