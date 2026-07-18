package preview

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"vit-daw-agent/internal/tools"
)

func Command(spec tools.CommandSpec, cmd map[string]any) string {
	switch spec.CommandName {
	case "apply_midi_note_patch":
		return MidiPatch(spec, cmd)
	case "add_midi_notes", "add_midi_notes_bulk", "mutate_midi_notes", "delete_midi_notes":
		return LegacyMidiNotes(spec, cmd)
	case "import_midi_to_track":
		return ImportMidiFile(spec, cmd)
	case "project.import_folder_as_stems", "project.import_audio_files":
		return ImportProjectAudioFiles(spec, cmd)
	case "project.apply_track_organization":
		return ApplyTrackOrganization(spec, cmd)
	case "track.group.create", "track.group.update", "track.group.set_members":
		return TrackGroupMutation(spec, cmd)
	case "track.group.delete":
		return TrackGroupDelete(spec, cmd)
	case "track.group.apply_control":
		return TrackGroupApplyControl(spec, cmd)
	case "rack_add_node":
		return RackAddNode(spec, cmd)
	case "control_add_macro":
		return MacroControl(spec, cmd)
	case "control_rename_macro":
		return MacroRename(spec, cmd)
	case "plugin_grabber_upsert_project_profile":
		return PluginGrabberProfileUpsert(spec, cmd)
	case "plugin_grabber_remove_project_profile":
		return PluginGrabberProfileRemove(spec, cmd)
	}
	raw, _ := json.Marshal(cmd)
	return fmt.Sprintf("%s [%s]: %s\n%s", spec.CommandName, spec.RiskLevel, spec.Description, string(raw))
}

func ImportProjectAudioFiles(spec tools.CommandSpec, cmd map[string]any) string {
	folderPath := firstString(cmd, "folder_path", "folder", "directory")
	folderName := firstNonEmpty(filepath.Base(folderPath), folderPath)
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	if folderName != "" {
		fmt.Fprintf(&b, "Folder %s\n", folderName)
	}
	if tracks := firstNonEmpty(firstString(cmd, "tracks_to_create"), firstString(cmd, "readable_file_count")); tracks != "" {
		fmt.Fprintf(&b, "Tracks to create %s\n", tracks)
	}
	decision := mapFromAny(cmd["sample_rate_decision"])
	patch := mapFromAny(cmd["project_audio_settings_patch"])
	if len(decision) > 0 {
		projectSR := firstString(decision, "project_sample_rate_hz")
		sourceSR := firstString(decision, "source_sample_rate_hz")
		if newSR := firstString(patch, "sample_rate_hz"); newSR != "" {
			fmt.Fprintf(&b, "Project sample rate %s Hz -> %s Hz before import\n", firstNonEmpty(projectSR, "<current>"), newSR)
		} else if sourceSR != "" && projectSR != "" && sourceSR != projectSR {
			fmt.Fprintf(&b, "Sample-rate mismatch: source %s Hz, project %s Hz; keep project rate unless changed separately\n", sourceSR, projectSR)
		}
	}
	fmt.Fprintf(&b, "Start %s", firstNonEmpty(firstString(cmd, "start_time_seconds", "start_time"), "0"))
	return strings.TrimSpace(b.String())
}

func ApplyTrackOrganization(spec tools.CommandSpec, cmd map[string]any) string {
	groups := operationRowsFromAny(cmd["groups"])
	if len(groups) == 0 {
		groups = operationRowsFromAny(cmd["group_proposals"])
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Folders to create %d", len(groups))

	totalTracks := 0
	for i, group := range groups {
		trackIDs := stringSliceFromAny(group["track_ids"])
		if len(trackIDs) == 0 {
			for _, assignment := range operationRowsFromAny(group["assignments"]) {
				if id := firstString(assignment, "track_id", "id"); id != "" {
					trackIDs = append(trackIDs, id)
				}
			}
		}
		totalTracks += len(trackIDs)
		if i >= 6 {
			continue
		}
		folder := firstNonEmpty(firstString(group, "folder_name", "proposed_folder", "label", "name", "group_id"), "Folder")
		fmt.Fprintf(&b, "\n%d. %s: %d tracks", i+1, folder, len(trackIDs))
		if enabled, ok := boolValue(group["routing_bus_enabled"]); ok && enabled {
			b.WriteString(" / bus routing")
		}
	}
	if len(groups) > 6 {
		fmt.Fprintf(&b, "\n... %d more folders", len(groups)-6)
	}
	fmt.Fprintf(&b, "\nTracks to move %d", totalTracks)
	return strings.TrimSpace(b.String())
}

func TrackGroupMutation(spec tools.CommandSpec, cmd map[string]any) string {
	trackIDs := stringSliceFromAny(firstPresent(cmd, "track_ids", "member_track_ids", "members"))
	name := firstNonEmpty(firstString(cmd, "name", "group_name", "label"), firstString(cmd, "group_id", "id"), "Track group")
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Group %s\n", name)
	if groupID := firstString(cmd, "group_id", "id"); groupID != "" {
		fmt.Fprintf(&b, "ID %s\n", groupID)
	}
	fmt.Fprintf(&b, "Members %d", len(trackIDs))
	if len(trackIDs) > 0 {
		fmt.Fprintf(&b, ": %s", strings.Join(firstNStrings(trackIDs, 10), ", "))
	}
	return strings.TrimSpace(b.String())
}

func TrackGroupDelete(spec tools.CommandSpec, cmd map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Group %s", firstNonEmpty(firstString(cmd, "group_id", "id"), "<group_id>"))
	return strings.TrimSpace(b.String())
}

func TrackGroupApplyControl(spec tools.CommandSpec, cmd map[string]any) string {
	mode := strings.ToLower(firstNonEmpty(firstString(cmd, "mode", "operation"), "absolute"))
	control := firstNonEmpty(firstString(cmd, "control", "param", "parameter"), "volume")
	trackIDs := stringSliceFromAny(firstPresent(cmd, "track_ids", "member_track_ids", "members"))
	value := firstString(cmd, "db", "target_db", "value_db", "volume_db")
	if mode == "relative" || mode == "delta" || mode == "volume_relative" {
		value = firstString(cmd, "delta_db", "db_delta", "amount_db")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Group %s\n", firstNonEmpty(firstString(cmd, "group_id", "id"), "<group_id>"))
	if len(trackIDs) > 0 {
		fmt.Fprintf(&b, "Members %d: %s\n", len(trackIDs), strings.Join(firstNStrings(trackIDs, 10), ", "))
	}
	fmt.Fprintf(&b, "Control %s %s", control, mode)
	if value != "" {
		fmt.Fprintf(&b, " %s dB", value)
	}
	return strings.TrimSpace(b.String())
}

func ImportMidiFile(spec tools.CommandSpec, cmd map[string]any) string {
	midiPath := firstString(cmd, "file_path", "path")
	midiName := firstNonEmpty(filepath.Base(midiPath), midiPath)
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Track %s\n", firstString(cmd, "track_id"))
	fmt.Fprintf(&b, "MIDI %s\n", midiName)
	fmt.Fprintf(&b, "Start beat %s\n", firstNonEmpty(firstString(cmd, "start_time_beats", "start_beats"), "0"))
	return strings.TrimSpace(b.String())
}

func RackAddNode(spec tools.CommandSpec, cmd map[string]any) string {
	pluginPath := firstString(cmd, "plugin_path", "path", "file_path")
	pluginName := firstNonEmpty(filepath.Base(pluginPath), pluginPath)
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Track %s\n", firstString(cmd, "track_id"))
	fmt.Fprintf(&b, "Plugin %s\n", pluginName)
	if zone := firstString(cmd, "zone_id"); zone != "" {
		fmt.Fprintf(&b, "Zone %s\n", zone)
	}
	return strings.TrimSpace(b.String())
}

func MacroControl(spec tools.CommandSpec, cmd map[string]any) string {
	bindings := operationRowsFromAny(cmd["bindings"])
	name := firstNonEmpty(firstString(cmd, "name", "label", "title", "macro_name"), firstString(cmd, "macro_id", "id"), "Macro")
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Track %s\n", firstString(cmd, "track_id"))
	fmt.Fprintf(&b, "Macro %s\n", name)
	fmt.Fprintf(&b, "Control %s\n", firstNonEmpty(firstString(cmd, "control_type", "type"), "slider"))
	if len(bindings) == 0 {
		b.WriteString("Bindings: none")
		return strings.TrimSpace(b.String())
	}
	fmt.Fprintf(&b, "Bindings: %d", len(bindings))
	for i, binding := range bindings {
		if i >= 4 {
			fmt.Fprintf(&b, "\n... %d more", len(bindings)-i)
			break
		}
		fmt.Fprintf(&b, "\n%d. %s", i+1, firstNonEmpty(firstString(binding, "param_name", "label", "name"), firstString(binding, "param_id"), "<param>"))
		if targetMin := firstString(binding, "target_min", "min"); targetMin != "" {
			fmt.Fprintf(&b, " min=%s", targetMin)
		}
		if targetMax := firstString(binding, "target_max", "max"); targetMax != "" {
			fmt.Fprintf(&b, " max=%s", targetMax)
		}
	}
	return strings.TrimSpace(b.String())
}

func MacroRename(spec tools.CommandSpec, cmd map[string]any) string {
	name := firstNonEmpty(firstString(cmd, "new_name", "name", "label", "title"), "<name>")
	macro := firstNonEmpty(firstString(cmd, "macro_name", "macro_label", "macro_id", "id"), "<macro>")
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Macro %s\n", macro)
	fmt.Fprintf(&b, "New name %s", name)
	return strings.TrimSpace(b.String())
}

func PluginGrabberProfileUpsert(spec tools.CommandSpec, cmd map[string]any) string {
	quickIDs := stringSliceFromAny(cmd["quick_control_ids"])
	aliases := mapStringStringFromAny(cmd["aliases"])
	groups := mapStringStringFromAny(cmd["display_groups"])
	roles := mapStringStringFromAny(cmd["normalized_roles"])
	semanticGroups := operationRowsFromAny(cmd["groups"])
	virtualControls := operationRowsFromAny(cmd["virtual_controls"])
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Track %s, plugin %s\n", firstString(cmd, "track_id"), firstString(cmd, "plugin_id"))
	fmt.Fprintf(&b, "Quick controls (%d): %s\n", len(quickIDs), strings.Join(firstNStrings(quickIDs, 16), ", "))
	if pluginClass := firstString(cmd, "class"); pluginClass != "" {
		fmt.Fprintf(&b, "Class: %s\n", pluginClass)
	}
	if len(aliases) > 0 {
		fmt.Fprintf(&b, "Aliases: %d\n", len(aliases))
	}
	if len(groups) > 0 {
		fmt.Fprintf(&b, "Display groups: %d\n", len(groups))
	}
	if len(roles) > 0 {
		fmt.Fprintf(&b, "Normalized roles: %d\n", len(roles))
	}
	if len(semanticGroups) > 0 {
		fmt.Fprintf(&b, "Semantic groups: %d\n", len(semanticGroups))
	}
	if len(virtualControls) > 0 {
		fmt.Fprintf(&b, "Virtual controls: %d\n", len(virtualControls))
	}
	if cmd["plugin_skill"] != nil {
		fmt.Fprintf(&b, "Plugin skill: yes\n")
	}
	return strings.TrimSpace(b.String())
}

func PluginGrabberProfileRemove(spec tools.CommandSpec, cmd map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	if profileID := firstString(cmd, "profile_id"); profileID != "" {
		fmt.Fprintf(&b, "Profile %s", profileID)
	} else {
		fmt.Fprintf(&b, "Track %s, plugin %s", firstString(cmd, "track_id"), firstString(cmd, "plugin_id"))
	}
	return b.String()
}

func LegacyMidiNotes(spec tools.CommandSpec, cmd map[string]any) string {
	notes := operationRowsFromAny(cmd["notes"])
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Clip %s, time_unit=%s\n", firstString(cmd, "clip_id"), firstNonEmpty(firstString(cmd, "time_unit"), "beats"))
	if len(notes) == 0 {
		if ids := stringSliceFromAny(cmd["note_ids"]); len(ids) > 0 {
			fmt.Fprintf(&b, "Notes: %s", strings.Join(ids, ","))
			return b.String()
		}
		b.WriteString("No MIDI notes.")
		return b.String()
	}
	for i, note := range notes {
		fmt.Fprintf(&b, "%d. note", i+1)
		if id := firstString(note, "id", "note_id"); id != "" {
			fmt.Fprintf(&b, " id=%s", id)
		}
		for _, key := range []string{"pitch", "start", "length", "velocity"} {
			if !isEmptyValue(note[key]) {
				fmt.Fprintf(&b, " %s=%v", key, note[key])
			}
		}
		if isEmptyValue(note["length"]) && !isEmptyValue(note["duration"]) {
			fmt.Fprintf(&b, " duration=%v", note["duration"])
		}
		if i != len(notes)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func MidiPatch(spec tools.CommandSpec, cmd map[string]any) string {
	ops := operationRowsFromAny(cmd["operations"])
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]: %s\n", spec.CommandName, spec.RiskLevel, spec.Description)
	fmt.Fprintf(&b, "Clip %s, time_unit=%s\n", firstString(cmd, "clip_id"), firstNonEmpty(firstString(cmd, "time_unit"), "beats"))
	if len(ops) == 0 {
		b.WriteString("No MIDI operations.")
		return b.String()
	}
	for i, op := range ops {
		fmt.Fprintf(&b, "%d. %s", i+1, firstNonEmpty(firstString(op, "op"), "<unknown>"))
		if id := firstString(op, "note_id", "id"); id != "" {
			fmt.Fprintf(&b, " note=%s", id)
		}
		if ids := stringSliceFromAny(op["note_ids"]); len(ids) > 0 {
			fmt.Fprintf(&b, " notes=%s", strings.Join(ids, ","))
		}
		for _, key := range []string{"pitch", "start", "length", "velocity", "semitones", "grid"} {
			if !isEmptyValue(op[key]) {
				fmt.Fprintf(&b, " %s=%v", key, op[key])
			}
		}
		if notes := operationRowsFromAny(op["notes"]); len(notes) > 0 {
			fmt.Fprintf(&b, " insert_notes=%d", len(notes))
		}
		if i != len(ops)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok && !isEmptyValue(v) {
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := row[key]; ok && !isEmptyValue(v) {
			return v
		}
	}
	return nil
}

func stringSliceFromAny(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case []string:
		out := make([]string, 0, len(x))
		for _, it := range x {
			s := strings.TrimSpace(it)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, it := range x {
			s := strings.TrimSpace(fmt.Sprint(it))
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(x))
		for key, enabled := range x {
			if value, ok := boolValue(enabled); ok && !value {
				continue
			}
			s := strings.TrimSpace(key)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "<nil>" {
			return nil
		}
		return []string{s}
	}
}

func mapStringStringFromAny(v any) map[string]string {
	switch x := v.(type) {
	case map[string]string:
		return x
	case map[string]any:
		out := make(map[string]string, len(x))
		for key, value := range x {
			if s := strings.TrimSpace(fmt.Sprint(value)); s != "" && s != "<nil>" {
				out[key] = s
			}
		}
		return out
	default:
		return nil
	}
}

func operationRowsFromAny(v any) []map[string]any {
	switch rows := v.(type) {
	case map[string]any:
		return []map[string]any{rows}
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, it := range rows {
			if row, ok := it.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func mapFromAny(v any) map[string]any {
	if row, ok := v.(map[string]any); ok {
		return row
	}
	return nil
}

func boolValue(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "1", "yes", "on", "enable", "enabled":
			return true, true
		case "false", "0", "no", "off", "disable", "disabled":
			return false, true
		}
	}
	return false, false
}

func firstNStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) == 0 {
		return nil
	}
	if len(values) > limit {
		values = values[:limit]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s == "" || s == "<nil>"
}
