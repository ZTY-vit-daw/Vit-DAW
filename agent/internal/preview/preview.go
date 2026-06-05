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
	case "rack_add_node":
		return RackAddNode(spec, cmd)
	case "plugin_grabber_upsert_project_profile":
		return PluginGrabberProfileUpsert(spec, cmd)
	case "plugin_grabber_remove_project_profile":
		return PluginGrabberProfileRemove(spec, cmd)
	}
	raw, _ := json.Marshal(cmd)
	return fmt.Sprintf("%s [%s]: %s\n%s", spec.CommandName, spec.RiskLevel, spec.Description, string(raw))
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
