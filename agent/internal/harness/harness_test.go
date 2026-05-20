package harness

import (
	"context"
	"testing"

	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tools"
)

func TestInvokeConfirmCommandDoesNotNeedKernelBeforeApproval(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{"cmd": "delete_track", "track_id": "1007"},
		Source:  "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "needs_confirmation" {
		t.Fatalf("status = %q, want needs_confirmation", resp.Status)
	}
	if resp.AgentActionID == "" || resp.Preview == "" {
		t.Fatalf("missing action id or preview: %+v", resp)
	}
}

func TestResolveRejectsUnknownCommand(t *testing.T) {
	h := New(nil, nil, nil)
	_, _, err := h.resolveCommand(InvokeRequest{
		Tool: "daw.invoke",
		Args: map[string]any{"cmd": "totally_not_registered"},
	})
	if err == nil {
		t.Fatal("expected unknown command error")
	}
}

func TestResolveToolBuildsKernelCommand(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.mute",
		Args: map[string]any{"track_id": "1007", "mute": true},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if spec.CommandName != "set_mute" || cmd["cmd"] != "set_mute" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v spec = %+v", cmd, spec)
	}
}

func TestResolveToolFormCommandBuildsKernelCommand(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"tool": "track.solo",
			"args": map[string]any{
				"track_id": "1007",
				"enabled":  "true",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if spec.CommandName != "set_solo" || cmd["cmd"] != "set_solo" || cmd["track_id"] != "1007" || cmd["solo"] != true {
		t.Fatalf("cmd = %+v spec = %+v", cmd, spec)
	}
}

func TestNormalizeTrackBooleanAliases(t *testing.T) {
	h := New(nil, nil, nil)
	cases := []struct {
		tool  string
		args  map[string]any
		field string
		want  bool
	}{
		{"track.mute", map[string]any{"track_id": "1007", "enabled": "on"}, "mute", true},
		{"track.mute", map[string]any{"track_id": "1007", "muted": "0"}, "mute", false},
		{"track.solo", map[string]any{"track_id": "1007", "value": 1}, "solo", true},
		{"track.arm", map[string]any{"track_id": "1007", "armed": "false"}, "is_armed", false},
	}
	for _, tc := range cases {
		cmd, spec, err := h.resolveCommand(InvokeRequest{Tool: tc.tool, Args: tc.args})
		if err != nil {
			t.Fatalf("%s resolveCommand: %v", tc.tool, err)
		}
		if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
			t.Fatalf("%s resolveImplicitTargets: %v", tc.tool, err)
		}
		if got := cmd[tc.field]; got != tc.want {
			t.Fatalf("%s %s = %#v, want %#v; cmd=%+v", tc.tool, tc.field, got, tc.want, cmd)
		}
	}
}

func TestInferMissingTrackBooleanFromUserMessage(t *testing.T) {
	h := New(nil, nil, nil)
	cases := []struct {
		name    string
		command map[string]any
		context map[string]any
		field   string
		want    bool
	}{
		{
			name:    "mute on",
			command: map[string]any{"cmd": "set_mute", "track_id": "1007"},
			context: map[string]any{"user_message": "我想让track1静音"},
			field:   "mute",
			want:    true,
		},
		{
			name:    "mute off",
			command: map[string]any{"cmd": "set_mute", "track_id": "1007"},
			context: map[string]any{"user_message": "取消静音"},
			field:   "mute",
			want:    false,
		},
		{
			name:    "solo on",
			command: map[string]any{"cmd": "set_solo", "track_id": "1012"},
			context: map[string]any{"user_message": "track 2 solo"},
			field:   "solo",
			want:    true,
		},
	}
	for _, tc := range cases {
		cmd, spec, err := h.resolveCommand(InvokeRequest{Command: tc.command})
		if err != nil {
			t.Fatalf("%s resolveCommand: %v", tc.name, err)
		}
		if err := h.resolveImplicitTargets(context.Background(), spec, cmd, tc.context); err != nil {
			t.Fatalf("%s resolveImplicitTargets: %v", tc.name, err)
		}
		if got := cmd[tc.field]; got != tc.want {
			t.Fatalf("%s %s = %#v, want %#v; cmd=%+v", tc.name, tc.field, got, tc.want, cmd)
		}
	}
}

func TestInvokeRejectsMissingRequiredTargetID(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:   "track.rename",
		Args:   map[string]any{"name": "Lead Vocal"},
		Source: "test",
	})
	if err == nil {
		t.Fatal("expected missing target id error")
	}
	if resp.Status != "error" || resp.CommandName != "rename_track" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestResolveImplicitTrackIDFromSingleVisibleTrack(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Track 1", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.rename",
		Args: map[string]any{"new_name": "Lead Vocal"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1007" {
		t.Fatalf("track_id = %#v, want 1007", cmd["track_id"])
	}
	if cmd["name"] != "Lead Vocal" {
		t.Fatalf("name = %#v, want Lead Vocal", cmd["name"])
	}
}

func TestResolveFlattensNestedParams(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "rename_track",
			"params": map[string]any{
				"track_id": "1007",
				"new_name": "A",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if _, ok := cmd["params"]; ok {
		t.Fatalf("params was not flattened: %+v", cmd)
	}
	if cmd["track_id"] != "1007" || cmd["name"] != "A" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveFlattensNestedArgsOnCommand(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "move_clip",
			"args": map[string]any{
				"clip_id":         "clip_a",
				"source_track_id": "1007",
				"target_track_id": "1007",
				"new_start":       10.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if _, ok := cmd["args"]; ok {
		t.Fatalf("args was not flattened: %+v", cmd)
	}
	if cmd["source_track_id"] != "1007" || cmd["target_track_id"] != "1007" || cmd["new_start"] != 10.0 {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveToolFormRemoveClipStringID(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"tool": "clip.remove",
			"args": map[string]any{"clip_ids": "clip_a"},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	got := cmd["clip_ids"].([]string)
	if len(got) != 1 || got[0] != "clip_a" {
		t.Fatalf("clip_ids = %#v", got)
	}
}

func TestResolveImplicitTrackIDByUserTrackIndex(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Track 1", "track_type": "hybrid", "is_audio_track": true},
			map[string]any{"track_id": "1010", "track_name": "Track 2", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.mute",
		Args: map[string]any{"user_track_index": 2, "mute": true},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1010" {
		t.Fatalf("track_id = %#v, want 1010", cmd["track_id"])
	}
}

func TestResolveImplicitTrackIDFromSelectedContext(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Drums", "track_type": "hybrid", "is_audio_track": true},
			map[string]any{"track_id": "1010", "track_name": "Bass", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.rename",
		Args: map[string]any{"new_name": "Low End"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id":   "1010",
		"selected_track_name": "Bass",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1010" {
		t.Fatalf("track_id = %#v, want 1010", cmd["track_id"])
	}
}

func TestResolveClipResizeFromSelectedContext(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.resize",
		Args: map[string]any{"new_length": 4.5},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_b" || cmd["track_id"] != "1010" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveClipResizeInfersLengthFromUserMessage(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "resize_clip",
			"args": map[string]any{
				"clip_id": "clip_b",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "把选中的clip裁到2秒",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_b" || cmd["track_id"] != "1010" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["new_length"] != 2.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("length args = %+v", cmd)
	}
}

func TestResolveRemoveClipsFromSelectedIDs(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.remove",
		Args: map[string]any{},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_ids": []any{"clip_a", "clip_b"},
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	got := cmd["clip_ids"].([]string)
	if len(got) != 2 || got[0] != "clip_a" || got[1] != "clip_b" {
		t.Fatalf("clip_ids = %#v", got)
	}
}

func TestResolveMoveClipDefaultsTracksFromSelectedClip(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.move",
		Args: map[string]any{"new_start_seconds": 2.0},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_a" || cmd["source_track_id"] != "1007" || cmd["target_track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["new_start"] != 2.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestResolveMoveClipInfersNewStartFromUserMessage(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "move_clip",
			"args": map[string]any{
				"clip_id":         "clip_a",
				"source_track_id": "1007",
				"target_track_id": "1007",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message": "移动选中的clip移动10s",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["new_start"] != 10.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveCloneClipAliasesAndTargetTrack(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{
			"clip_id":            "clip_a",
			"target_track_index": 2,
			"new_start":          8.0,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["source_clip_id"] != "clip_a" || cmd["target_track_id"] != "1010" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["time_unit"] != "seconds" {
		t.Fatalf("time_unit = %#v", cmd["time_unit"])
	}
}

func TestPublicResultSanitizesProjectState(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1002", "track_name": "Arranger", "track_type": "track", "is_audio_track": false},
			map[string]any{"track_id": "1007", "track_name": "Track 1", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)

	result := h.publicResult(tools.CommandSpec{CommandName: "get_project_state"}, map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1002"},
			map[string]any{"track_id": "1007"},
		},
	})

	if result["status"] != "ok" || result["track_count"] != 1 {
		t.Fatalf("result counts = %+v", result)
	}
	if _, ok := result["engine_track_count"]; ok {
		t.Fatalf("public result leaked engine_track_count: %+v", result)
	}
	if _, ok := result["internal_track_count"]; ok {
		t.Fatalf("public result leaked internal_track_count: %+v", result)
	}
	tracks := result["tracks"].([]map[string]any)
	if len(tracks) != 1 || tracks[0]["track_id"] != "1007" {
		t.Fatalf("tracks = %+v", tracks)
	}
}

func shadowProjectWithClips() *shadow.Project {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Drums",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{"id": "clip_a", "name": "Loop A", "start_seconds": 0.0, "length_seconds": 2.0},
				},
			},
			map[string]any{
				"track_id":       "1010",
				"track_name":     "Bass",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{"id": "clip_b", "name": "Loop B", "start_seconds": 4.0, "length_seconds": 2.0},
				},
			},
		},
	})
	return project
}
