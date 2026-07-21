package agentloop

import (
	"fmt"
	"strings"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

func TestBindPendingTrackOrganizationStoresFullTOMManifest(t *testing.T) {
	rows := make([]map[string]any, 0, 25)
	for i := 1; i <= 25; i++ {
		rows = append(rows, map[string]any{
			"track_id":         fmt.Sprintf("track_kick_%02d", i),
			"track_name":       fmt.Sprintf("Kick %02d", i),
			"clip_id":          fmt.Sprintf("clip_kick_%02d", i),
			"clip_name":        fmt.Sprintf("Kick %02d.wav", i),
			"source_file_path": fmt.Sprintf("E:/stems/Kick %02d.wav", i),
			"duration_seconds": 180.0,
			"sample_rate_hz":   48000,
			"bit_depth":        24,
			"channel_count":    1,
		})
	}
	state := &runState{}
	result := executorpkg.Result{
		Status:     "ok",
		Tool:       "project.import_folder_as_stems",
		ToolCallID: "import_1",
		Result: map[string]any{
			"summary":             map[string]any{"tracks_created": 25, "edit_length_seconds": 180.0},
			"imported_track_refs": rows,
		},
	}

	bindPendingTrackOrganization(state, planner.ToolCall{ID: "import_1", Tool: "project.import_folder_as_stems"}, result)

	pending := state.executionMemory.PendingTrackOrganization
	if len(pending) == 0 {
		t.Fatal("pending TOM organization memory missing")
	}
	if pending["coverage_status"] != "complete" || pending["assignment_coverage_count"] != 25 {
		t.Fatalf("pending coverage mismatch: %#v", pending)
	}
	groups := messageLoopMapRows(pending["groups"])
	if len(groups) != 1 {
		t.Fatalf("expected one stored group, got %#v", groups)
	}
	idsCSV := firstMapText(groups[0], "track_ids_csv")
	if !strings.Contains(idsCSV, "track_kick_01") || !strings.Contains(idsCSV, "track_kick_25") {
		t.Fatalf("stored group did not keep full ID line: %q", idsCSV)
	}
	memoryMap := executionMemoryMap(state.executionMemory)
	if messageLoopMapValue(memoryMap["pending_track_organization"]) == nil {
		t.Fatalf("execution memory map omitted pending organization: %#v", memoryMap)
	}
	report := strings.Join(messageLoopTOMOrganizationReportBullets(result.Result), "\n")
	if !strings.Contains(report, "覆盖 25/25") || !strings.Contains(report, "当前依据：命名/ID 阶段") {
		t.Fatalf("TOM report should expose coverage and disclosure stage, got:\n%s", report)
	}

	truncatedIDs := messageLoopSplitTrackIDsCSV(idsCSV)
	truncatedIDs = truncatedIDs[:20]
	applyCall := planner.ToolCall{
		Tool: "project.apply_track_organization",
		Args: map[string]any{
			"groups": []map[string]any{
				{"folder_name": "Drums", "track_ids": truncatedIDs},
			},
		},
		Command: map[string]any{"cmd": "project.apply_track_organization"},
	}
	resolved := resolveMessageLoopBindings(state, applyCall)
	resolvedGroups := messageLoopMapRows(resolved.Args["groups"])
	if len(resolvedGroups) != 1 {
		t.Fatalf("resolved apply organization groups mismatch: %#v", resolved.Args["groups"])
	}
	resolvedIDs := messageLoopStringSliceFromAny(resolvedGroups[0]["track_ids"])
	if len(resolvedIDs) != 25 || resolvedIDs[24] != "track_kick_25" {
		t.Fatalf("pending TOM manifest should replace incomplete apply args, got %d ids: %#v", len(resolvedIDs), resolvedIDs)
	}
}

func TestProjectStateRebuildsPendingTOMManifestForOrganizationIntent(t *testing.T) {
	tracks := make([]map[string]any, 0, 25)
	for i := 1; i <= 25; i++ {
		trackID := fmt.Sprintf("track_bgv_%02d", i)
		clipID := fmt.Sprintf("clip_bgv_%02d", i)
		tracks = append(tracks, map[string]any{
			"track_id":   trackID,
			"id":         trackID,
			"name":       fmt.Sprintf("BGV %02d", i),
			"track_type": "hybrid",
			"clips": []map[string]any{
				{
					"clip_id":             clipID,
					"id":                  clipID,
					"name":                fmt.Sprintf("BGV %02d", i),
					"current_source_path": fmt.Sprintf("E:/stems/BGV %02d.wav", i),
					"length_seconds":      180.0,
					"start_seconds":       0.0,
				},
			},
		})
	}
	state := &runState{input: Input{UserText: "按刚才建议把轨道整理成文件夹"}}
	result := executorpkg.Result{
		Status:     "ok",
		Tool:       "project.state",
		ToolCallID: "state_1",
		Result:     map[string]any{"track_count": 25, "tracks": tracks},
	}

	bindPendingTrackOrganizationFromProjectState(state, planner.ToolCall{ID: "state_1", Tool: "project.state"}, result)

	pending := state.executionMemory.PendingTrackOrganization
	if len(pending) == 0 {
		t.Fatal("project state organization fallback did not create pending TOM manifest")
	}
	if pending["source"] != "project_state_rebuilt_tom" || pending["assignment_coverage_count"] != 25 {
		t.Fatalf("fallback pending manifest mismatch: %#v", pending)
	}
	groups := messageLoopMapRows(pending["groups"])
	if len(groups) != 1 {
		t.Fatalf("expected one BGV group, got %#v", groups)
	}
	if idsCSV := firstMapText(groups[0], "track_ids_csv"); !strings.Contains(idsCSV, "track_bgv_01") || !strings.Contains(idsCSV, "track_bgv_25") {
		t.Fatalf("fallback manifest did not keep full project state IDs: %q", idsCSV)
	}
}

func TestBindPendingSectionMarkersStoresFullEPMManifest(t *testing.T) {
	state := &runState{}
	result := executorpkg.Result{
		Status:     "ok",
		Tool:       "project.import_folder_as_stems",
		ToolCallID: "import_1",
		Result: map[string]any{
			"epm_projection": map[string]any{
				"schema_version": "epm.projection.v0",
				"epm_version":    "v0",
				"section_map": map[string]any{
					"status":             "ready",
					"candidate_count":    3,
					"duration_seconds":   90.0,
					"coverage_seconds":   90.0,
					"confidence":         "medium",
					"reference_strategy": "template",
					"sections": []map[string]any{
						{"section_id": "section_01", "label": "Intro", "start_seconds": 0.0, "end_seconds": 30.0, "confidence": "medium"},
						{"section_id": "section_02", "label": "Section A", "start_seconds": 30.0, "end_seconds": 60.0, "confidence": "medium"},
						{"section_id": "section_03", "label": "Outro", "start_seconds": 60.0, "end_seconds": 90.0, "confidence": "low"},
					},
				},
			},
		},
	}

	bindPendingSectionMarkers(state, planner.ToolCall{ID: "import_1", Tool: "project.import_folder_as_stems"}, result)

	pending := state.executionMemory.PendingSectionMarkers
	if len(pending) == 0 {
		t.Fatal("pending section marker memory missing")
	}
	if pending["section_count"] != 3 || pending["source"] != "epm_a5" {
		t.Fatalf("pending section marker memory mismatch: %#v", pending)
	}
	memoryMap := executionMemoryMap(state.executionMemory)
	if messageLoopMapValue(memoryMap["pending_section_markers"]) == nil {
		t.Fatalf("execution memory map omitted pending section markers: %#v", memoryMap)
	}

	applyCall := planner.ToolCall{
		Tool: "project.markers.apply_section_markers",
		Args: map[string]any{
			"sections": []map[string]any{
				{"name": "Intro", "start_seconds": 0.0, "end_seconds": 30.0},
			},
		},
		Command: map[string]any{"cmd": "project.markers.apply_section_markers"},
	}
	resolved := resolveMessageLoopBindings(state, applyCall)
	sections := messageLoopMapRows(resolved.Args["sections"])
	if len(sections) != 3 {
		t.Fatalf("pending section marker manifest should replace incomplete apply args, got %#v", sections)
	}
	if firstMapText(sections[2], "name") != "Outro" || firstPresentNumber(sections[2], "end_seconds") != 90.0 {
		t.Fatalf("resolved sections lost final marker: %#v", sections[2])
	}
	if resolved.Args["source"] != "epm_a5" {
		t.Fatalf("resolved marker source = %#v", resolved.Args["source"])
	}
}
