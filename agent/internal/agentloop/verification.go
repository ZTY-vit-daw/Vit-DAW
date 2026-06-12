package agentloop

import (
	"fmt"
	"path"
	"strings"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

const (
	verificationVerified                      = "verified"
	verificationUnverified                    = "unverified"
	verificationFailed                        = "failed"
	verificationNotRequired                   = "not_required"
	verificationPendingConfirmation           = "not_executed_pending_confirmation"
	postconditionTrackPresent                 = "track_present"
	postconditionClipPresent                  = "clip_present"
	postconditionMIDINotesPresent             = "midi_notes_present"
	postconditionRackNodePresent              = "rack_node_present"
	postconditionNoObservableDawPostcondition = "no_observable_daw_postcondition"
)

func verificationForPendingConfirmation(call planner.ToolCall, result executorpkg.Result) planner.VerificationResult {
	return planner.VerificationResult{
		ToolCallID:    firstNonEmpty(result.ToolCallID, call.ID),
		PlanItemID:    call.PlanItemID,
		Tool:          firstNonEmpty(result.Tool, call.Tool),
		CommandName:   strings.TrimSpace(result.CommandName),
		Status:        verificationPendingConfirmation,
		Postcondition: postconditionFor(call, result),
		Message:       "tool has not executed yet because it is waiting for user confirmation",
		Evidence: map[string]any{
			"requires_confirmation": true,
		},
	}
}

func verifyToolExecution(call planner.ToolCall, result executorpkg.Result) planner.VerificationResult {
	ver := planner.VerificationResult{
		ToolCallID:    firstNonEmpty(result.ToolCallID, call.ID),
		PlanItemID:    strings.TrimSpace(call.PlanItemID),
		Tool:          firstNonEmpty(result.Tool, call.Tool),
		CommandName:   strings.TrimSpace(result.CommandName),
		Postcondition: postconditionFor(call, result),
		Evidence:      map[string]any{},
	}
	if result.Error != "" || toolStatusFailed(result.Status) {
		ver.Status = verificationFailed
		ver.Message = firstNonEmpty(result.Error, "tool execution failed")
		return ver
	}
	switch ver.Postcondition {
	case postconditionTrackPresent:
		return verifyTrackPresent(ver, call, result)
	case postconditionClipPresent:
		return verifyClipPresent(ver, call, result)
	case postconditionMIDINotesPresent:
		return verifyMIDINotesPresent(ver, call, result)
	case postconditionRackNodePresent:
		return verifyRackNodePresent(ver, call, result)
	default:
		ver.Status = verificationNotRequired
		ver.Message = "tool result recorded; no additional DAW state postcondition is required"
		return ver
	}
}

func postconditionFor(call planner.ToolCall, result executorpkg.Result) string {
	name := normalizedActionName(call, result)
	switch name {
	case "add_track", "add_audio_track", "track.add", "track.add_audio":
		return postconditionTrackPresent
	case "create_midi_clip", "insert_midi_clip", "import_midi_to_track", "midi.create_clip", "midi.insert_clip", "midi.import_file",
		"import_audio", "import_media_to_track", "add_audio_clip", "clip.import_audio", "clip.import_media_to_track", "clip.add_audio":
		return postconditionClipPresent
	case "apply_midi_note_patch", "midi.apply_note_patch", "add_midi_notes", "add_midi_notes_bulk", "midi.add_notes", "midi.add_notes_bulk",
		"midi.legacy_add_notes", "midi.legacy_add_notes_bulk":
		if midiNoteInsertionExpected(call, result) {
			return postconditionMIDINotesPresent
		}
		return postconditionNoObservableDawPostcondition
	case "rack_add_node", "rack.add_node", "plugin.load_to_rack":
		return postconditionRackNodePresent
	default:
		return postconditionNoObservableDawPostcondition
	}
}

func verifyTrackPresent(ver planner.VerificationResult, call planner.ToolCall, result executorpkg.Result) planner.VerificationResult {
	trackID := firstMapText(result.Result, "track_id", "id", "item_id")
	if trackID == "" {
		trackID = firstMapText(call.Args, "track_id", "selected_track_id", "target_track_id")
	}
	trackName := firstMapText(result.Result, "track_name", "name")
	if trackName == "" {
		trackName = firstMapText(call.Args, "name", "track_name")
	}
	tracks := stateTracks(result.ObservedState)
	ver.Evidence["expected_track_id"] = trackID
	ver.Evidence["expected_track_name"] = trackName
	ver.Evidence["observed_track_count"] = len(tracks)
	if len(tracks) == 0 {
		ver.Status = verificationUnverified
		ver.Message = "could not observe the newly created track in refreshed project state"
		return ver
	}
	for _, track := range tracks {
		observedID := firstMapText(track, "track_id", "id", "item_id")
		observedName := firstMapText(track, "track_name", "name")
		if trackID != "" && observedID == trackID {
			ver.Status = verificationVerified
			ver.Message = fmt.Sprintf("observed track %s in refreshed project state", firstNonEmpty(observedName, observedID))
			ver.Evidence["observed_track_id"] = observedID
			ver.Evidence["observed_track_name"] = observedName
			return ver
		}
		if trackID == "" && trackName != "" && strings.EqualFold(observedName, trackName) {
			ver.Status = verificationVerified
			ver.Message = fmt.Sprintf("observed track %s in refreshed project state", observedName)
			ver.Evidence["observed_track_id"] = observedID
			ver.Evidence["observed_track_name"] = observedName
			return ver
		}
	}
	if trackID == "" && trackName == "" && len(tracks) > 0 {
		ver.Status = verificationVerified
		ver.Message = "observed at least one user-visible track after track creation"
		return ver
	}
	ver.Status = verificationUnverified
	ver.Message = "refreshed project state does not contain the expected track"
	return ver
}

func verifyClipPresent(ver planner.VerificationResult, call planner.ToolCall, result executorpkg.Result) planner.VerificationResult {
	clipID := firstNonEmpty(
		firstMapText(result.Result, "clip_id", "id", "item_id"),
		firstMapText(call.Args, "clip_id"),
	)
	clipName := firstNonEmpty(
		firstMapText(result.Result, "clip_name", "name"),
		firstMapText(call.Args, "clip_name", "name"),
	)
	trackID := firstNonEmpty(
		firstMapText(result.Result, "track_id", "target_track_id", "selected_track_id"),
		firstMapText(call.Args, "track_id", "target_track_id", "selected_track_id"),
	)
	clips := stateClips(result.ObservedState, trackID)
	ver.Evidence["expected_track_id"] = trackID
	ver.Evidence["expected_clip_id"] = clipID
	ver.Evidence["expected_clip_name"] = clipName
	ver.Evidence["observed_clip_count"] = len(clips)
	if len(result.ObservedState) == 0 {
		ver.Status = verificationUnverified
		ver.Message = "could not observe refreshed project state after clip creation or import"
		return ver
	}
	for _, clip := range clips {
		observedID := firstMapText(clip, "clip_id", "id", "item_id")
		observedName := firstMapText(clip, "clip_name", "name")
		if clipID != "" && observedID == clipID {
			ver.Status = verificationVerified
			ver.Message = fmt.Sprintf("observed clip %s in refreshed project state", firstNonEmpty(observedName, observedID))
			ver.Evidence["observed_clip_id"] = observedID
			ver.Evidence["observed_clip_name"] = observedName
			return ver
		}
		if clipID == "" && clipName != "" && strings.EqualFold(observedName, clipName) {
			ver.Status = verificationVerified
			ver.Message = fmt.Sprintf("observed clip %s in refreshed project state", observedName)
			ver.Evidence["observed_clip_id"] = observedID
			ver.Evidence["observed_clip_name"] = observedName
			return ver
		}
	}
	if clipID == "" && clipName == "" && len(clips) > 0 {
		ver.Status = verificationVerified
		ver.Message = "observed at least one user-visible clip after clip creation or import"
		return ver
	}
	ver.Status = verificationUnverified
	ver.Message = "refreshed project state does not contain the expected clip"
	return ver
}

func verifyMIDINotesPresent(ver planner.VerificationResult, call planner.ToolCall, result executorpkg.Result) planner.VerificationResult {
	clipID := firstNonEmpty(
		firstMapText(result.Result, "clip_id", "id", "item_id"),
		firstMapText(call.Args, "clip_id", "selected_clip_id", "primary_selected_clip_id"),
	)
	trackID := firstNonEmpty(
		firstMapText(result.Result, "track_id", "target_track_id", "selected_track_id"),
		firstMapText(call.Args, "track_id", "target_track_id", "selected_track_id"),
	)
	expectedIDs := noteIDsFromAny(result.Result["inserted_note_ids"])
	if len(expectedIDs) == 0 {
		expectedIDs = noteIDsFromAny(result.Result["note_ids"])
	}
	expectedCount := firstPositiveMapInt(result.Result, "inserted_count", "added_count", "written_count", "note_count")
	if expectedCount == 0 {
		expectedCount = expectedInsertedMIDINoteCount(call)
	}
	if expectedCount == 0 && len(expectedIDs) > 0 {
		expectedCount = len(expectedIDs)
	}
	observedNotes := stateClipNotes(result.ObservedState, trackID, clipID)
	ver.Evidence["expected_track_id"] = trackID
	ver.Evidence["expected_clip_id"] = clipID
	ver.Evidence["expected_inserted_count"] = expectedCount
	ver.Evidence["expected_note_ids"] = expectedIDs
	ver.Evidence["observed_note_count"] = len(observedNotes)
	if len(result.ObservedState) == 0 {
		ver.Status = verificationUnverified
		ver.Message = "could not observe refreshed project state after MIDI note write"
		return ver
	}
	if clipID != "" && len(observedNotes) == 0 && !stateHasClip(result.ObservedState, trackID, clipID) {
		ver.Status = verificationUnverified
		ver.Message = "refreshed project state does not contain the MIDI clip targeted by the note write"
		return ver
	}
	if len(expectedIDs) > 0 && observedNotesContainIDs(observedNotes, expectedIDs) {
		ver.Status = verificationVerified
		ver.Message = fmt.Sprintf("observed %d inserted MIDI notes in refreshed project state", len(expectedIDs))
		return ver
	}
	if expectedCount > 0 && len(observedNotes) >= expectedCount {
		ver.Status = verificationVerified
		ver.Message = fmt.Sprintf("observed %d MIDI notes in refreshed project state", len(observedNotes))
		return ver
	}
	ver.Status = verificationUnverified
	ver.Message = "refreshed project state does not show the expected MIDI notes"
	return ver
}

func verifyRackNodePresent(ver planner.VerificationResult, call planner.ToolCall, result executorpkg.Result) planner.VerificationResult {
	trackID := firstNonEmpty(
		firstMapText(result.Result, "track_id", "target_track_id", "selected_track_id"),
		firstMapText(call.Args, "track_id", "target_track_id", "selected_track_id"),
	)
	pluginID := firstMapText(result.Result, "plugin_id", "plugin_item_id", "node_id", "item_id", "id")
	pluginName := firstNonEmpty(
		firstMapText(result.Result, "plugin_name", "name", "descriptive_name"),
		firstMapText(call.Args, "plugin_name", "name"),
	)
	pluginPath := firstNonEmpty(
		firstMapText(result.Result, "plugin_path", "path", "file_path"),
		firstMapText(call.Args, "plugin_path", "path", "file_path"),
	)
	if pluginName == "" && pluginPath != "" {
		pluginName = pluginNameFromPath(pluginPath)
	}
	ver.Evidence["expected_track_id"] = trackID
	ver.Evidence["expected_plugin_id"] = pluginID
	ver.Evidence["expected_plugin_name"] = pluginName
	ver.Evidence["expected_plugin_path"] = pluginPath
	if len(result.ObservedState) == 0 {
		ver.Status = verificationUnverified
		ver.Message = "could not observe refreshed project state after plugin load"
		return ver
	}
	if node, ok := findRackNode(result.ObservedState, trackID, pluginID, pluginName, pluginPath); ok {
		if rackNodeIsOrphanedEffect(node) {
			ver.Status = verificationUnverified
			ver.Message = fmt.Sprintf("observed rack node %s, but it is not connected to the main audio path", firstNonEmpty(firstMapText(node, "plugin_name", "name", "descriptive_name"), pluginName, pluginID))
			ver.Evidence["observed_node_id"] = firstMapText(node, "plugin_id", "plugin_item_id", "node_id", "item_id", "id")
			ver.Evidence["observed_plugin_name"] = firstMapText(node, "plugin_name", "name", "descriptive_name")
			ver.Evidence["zone_id"] = firstMapText(node, "zone_id")
			if reachable, ok := firstMapBool(node, "audio_reachable_from_rack_input"); ok {
				ver.Evidence["audio_reachable_from_rack_input"] = reachable
			}
			if outputPath, ok := firstMapBool(node, "vit_effective_in_output_path"); ok {
				ver.Evidence["vit_effective_in_output_path"] = outputPath
			}
			if orphan, ok := firstMapBool(node, "vit_orphan_bypass_candidate"); ok {
				ver.Evidence["vit_orphan_bypass_candidate"] = orphan
			}
			return ver
		}
		ver.Status = verificationVerified
		ver.Message = fmt.Sprintf("observed rack node %s in refreshed project state", firstNonEmpty(firstMapText(node, "plugin_name", "name", "descriptive_name"), pluginName, pluginID))
		ver.Evidence["observed_node_id"] = firstMapText(node, "plugin_id", "plugin_item_id", "node_id", "item_id", "id")
		ver.Evidence["observed_plugin_name"] = firstMapText(node, "plugin_name", "name", "descriptive_name")
		return ver
	}
	ver.Status = verificationUnverified
	ver.Message = "refreshed project state does not contain the expected rack node"
	return ver
}

func normalizedActionName(call planner.ToolCall, result executorpkg.Result) string {
	for _, value := range []string{
		result.CommandName,
		commandNameFromMap(call.Command),
		commandNameFromMap(call.Args),
		result.Tool,
		call.Tool,
	} {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			return value
		}
	}
	return ""
}

func toolStatusFailed(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "error", "kernel_error", "failed", "failure":
		return true
	default:
		return false
	}
}

func commandNameFromMap(row map[string]any) string {
	return firstMapText(row, "cmd", "command", "action")
}

func findRackNode(state map[string]any, trackID, pluginID, pluginName, pluginPath string) (map[string]any, bool) {
	for _, track := range stateTracks(state) {
		if trackID != "" && firstMapText(track, "track_id", "id", "item_id") != trackID {
			continue
		}
		nodes := append(rackNodes(track), mapRows(track["rack_nodes"])...)
		for _, node := range nodes {
			if rackNodeMatches(node, pluginID, pluginName, pluginPath) {
				return node, true
			}
		}
	}
	return nil, false
}

func rackNodes(track map[string]any) []map[string]any {
	rack := firstMap(track, "rack", "plugin_rack")
	if rack == nil {
		return nil
	}
	return mapRows(rack["nodes"])
}

func rackNodeMatches(node map[string]any, pluginID, pluginName, pluginPath string) bool {
	nodeID := firstMapText(node, "plugin_id", "plugin_item_id", "node_id", "item_id", "id", "uid")
	if pluginID != "" && nodeID == pluginID {
		return true
	}
	nodeName := firstMapText(node, "plugin_name", "name", "descriptive_name")
	nodePath := firstMapText(node, "plugin_path", "path", "file_path")
	if pluginName != "" && pluginNamesMatch(nodeName, pluginName) {
		return true
	}
	if pluginPath != "" && (pluginNamesMatch(nodeName, pluginNameFromPath(pluginPath)) || cleanPath(nodePath) == cleanPath(pluginPath)) {
		return true
	}
	return false
}

func rackNodeIsOrphanedEffect(node map[string]any) bool {
	zoneID := strings.ToUpper(strings.TrimSpace(firstMapText(node, "zone_id", "zone")))
	if zoneID != "Z3" {
		return false
	}
	if orphan, ok := firstMapBool(node, "vit_orphan_bypass_candidate"); ok && orphan {
		return true
	}
	if inOutputPath, ok := firstMapBool(node, "vit_effective_in_output_path"); ok {
		return !inOutputPath
	}
	if reachable, ok := firstMapBool(node, "audio_reachable_from_rack_input"); ok {
		return !reachable
	}
	return false
}

func pluginNamesMatch(a, b string) bool {
	a = normalizePluginName(a)
	b = normalizePluginName(b)
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

func normalizePluginName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, ".vst3")
	value = strings.TrimSuffix(value, ".dll")
	value = strings.TrimSuffix(value, ".component")
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func pluginNameFromPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	base := path.Base(value)
	ext := path.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return strings.TrimSpace(base)
}

func cleanPath(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
}

func stateTracks(state map[string]any) []map[string]any {
	tracks := mapRows(state["tracks"])
	if len(tracks) > 0 {
		return tracks
	}
	if daw, ok := state["daw_state_summary"].(map[string]any); ok {
		return mapRows(daw["tracks"])
	}
	return nil
}

func stateClips(state map[string]any, trackID string) []map[string]any {
	var out []map[string]any
	for _, clip := range mapRows(state["clips"]) {
		if trackID == "" || firstMapText(clip, "track_id", "target_track_id") == trackID {
			out = append(out, clip)
		}
	}
	for _, track := range stateTracks(state) {
		if trackID != "" && firstMapText(track, "track_id", "id", "item_id") != trackID {
			continue
		}
		for _, key := range []string{"clips", "midi_clips", "audio_clips"} {
			out = append(out, mapRows(track[key])...)
		}
	}
	return out
}

func stateHasClip(state map[string]any, trackID, clipID string) bool {
	if clipID == "" {
		return false
	}
	for _, clip := range stateClips(state, trackID) {
		if firstMapText(clip, "clip_id", "id", "item_id") == clipID {
			return true
		}
	}
	return false
}

func stateClipNotes(state map[string]any, trackID, clipID string) []map[string]any {
	for _, clip := range stateClips(state, trackID) {
		observedID := firstMapText(clip, "clip_id", "id", "item_id")
		if clipID != "" && observedID != clipID {
			continue
		}
		notes := mapRows(clip["notes"])
		if len(notes) == 0 {
			notes = mapRows(clip["midi_notes"])
		}
		if len(notes) > 0 || clipID != "" {
			return notes
		}
	}
	return nil
}

func observedNotesContainIDs(notes []map[string]any, expected []string) bool {
	if len(expected) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, note := range notes {
		if id := firstMapText(note, "id", "note_id", "vit_note_id", "uid"); id != "" {
			seen[id] = true
		}
	}
	for _, id := range expected {
		if !seen[id] {
			return false
		}
	}
	return true
}

func midiNoteInsertionExpected(call planner.ToolCall, result executorpkg.Result) bool {
	if firstPositiveMapInt(result.Result, "inserted_count", "added_count", "written_count") > 0 {
		return true
	}
	if len(noteIDsFromAny(result.Result["inserted_note_ids"])) > 0 {
		return true
	}
	return expectedInsertedMIDINoteCount(call) > 0
}

func expectedInsertedMIDINoteCount(call planner.ToolCall) int {
	if notes := mapRows(call.Args["notes"]); len(notes) > 0 {
		return len(notes)
	}
	count := 0
	for _, op := range mapRows(call.Args["operations"]) {
		opName := strings.ToLower(strings.TrimSpace(firstMapText(op, "op", "type", "operation", "action")))
		switch opName {
		case "insert_note", "add_note":
			count++
		case "replace_region":
			count += len(mapRows(op["notes"]))
		}
	}
	return count
}

func noteIDsFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, id := range typed {
			if id = strings.TrimSpace(id); id != "" {
				out = append(out, id)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, value := range typed {
			id := strings.TrimSpace(fmt.Sprint(value))
			if id != "" && id != "<nil>" {
				out = append(out, id)
			}
		}
		return out
	default:
		return nil
	}
}

func firstPositiveMapInt(row map[string]any, keys ...string) int {
	for _, key := range keys {
		if row == nil {
			return 0
		}
		value, ok := row[key]
		if !ok || isEmptyVerificationValue(value) {
			continue
		}
		switch v := value.(type) {
		case int:
			if v > 0 {
				return v
			}
		case int64:
			if v > 0 {
				return int(v)
			}
		case float64:
			if v > 0 {
				return int(v)
			}
		default:
			var n int
			if _, err := fmt.Sscanf(strings.TrimSpace(fmt.Sprint(value)), "%d", &n); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

func firstMap(row map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if child, ok := row[key].(map[string]any); ok && len(child) > 0 {
			return child
		}
	}
	return nil
}

func mapRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if m, ok := row.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func firstMapText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if row == nil {
			return ""
		}
		if value, ok := row[key]; ok && !isEmptyVerificationValue(value) {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func firstMapBool(row map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		if row == nil {
			return false, false
		}
		value, ok := row[key]
		if !ok || isEmptyVerificationValue(value) {
			continue
		}
		switch v := value.(type) {
		case bool:
			return v, true
		case string:
			text := strings.ToLower(strings.TrimSpace(v))
			switch text {
			case "true", "1", "yes", "y", "ok":
				return true, true
			case "false", "0", "no", "n":
				return false, true
			}
		default:
			text := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
			switch text {
			case "true", "1", "yes", "y", "ok":
				return true, true
			case "false", "0", "no", "n":
				return false, true
			}
		}
	}
	return false, false
}

func isEmptyVerificationValue(value any) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case int:
		return false
	case int64:
		return false
	case float64:
		return false
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		return text == "" || text == "<nil>"
	}
}
