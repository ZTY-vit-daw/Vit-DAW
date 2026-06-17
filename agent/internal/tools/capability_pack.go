package tools

import (
	"fmt"
	"sort"
	"strings"
)

type CapabilityPack struct {
	Name          string
	Title         string
	Domain        string
	Description   string
	StateSlices   []string
	Preconditions []string
	Effects       []string
	ResultHints   []string
	Verification  []string
	Tools         []string
	Examples      []string
}

func (c *Catalog) ModelSummaryForCapabilityPacks(packNames, toolNames []string) string {
	if c == nil {
		return ""
	}
	packs := c.CapabilityPacks(packNames)
	if len(packs) == 0 {
		return c.ModelSummaryForTools(toolNames)
	}

	allowed := stringSet(toolNames)
	var covered []string
	var parts []string
	for _, pack := range packs {
		parts = append(parts, c.capabilityPackSummary(pack, allowed))
		covered = append(covered, pack.Tools...)
	}
	if shared := differenceStrings(toolNames, append(covered, "daw.invoke")); len(shared) > 0 {
		if summary := c.ModelSummaryForTools(shared); strings.TrimSpace(summary) != "" {
			parts = append(parts, "Shared support tools:\n"+summary)
		}
	}
	return strings.Join(nonEmptyStrings(parts), "\n\n")
}

func (c *Catalog) CapabilityPacks(names []string) []CapabilityPack {
	if c == nil {
		return nil
	}
	available := defaultCapabilityPacks()
	byName := map[string]CapabilityPack{}
	for _, pack := range available {
		if pack.Name != "" {
			byName[pack.Name] = pack
		}
	}
	seen := map[string]bool{}
	var out []CapabilityPack
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" || seen[name] {
			continue
		}
		pack, ok := byName[name]
		if !ok {
			continue
		}
		seen[name] = true
		out = append(out, pack)
	}
	return out
}

func (c *Catalog) capabilityPackSummary(pack CapabilityPack, allowed map[string]bool) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Capability pack: %s (%s)", pack.Name, pack.Title))
	lines = append(lines, "Domain: "+pack.Domain)
	lines = append(lines, "Purpose: "+pack.Description)
	lines = appendList(lines, "State slices", pack.StateSlices)
	lines = appendList(lines, "Preconditions", pack.Preconditions)
	lines = appendList(lines, "Effects", pack.Effects)
	lines = appendList(lines, "Result hints", pack.ResultHints)
	lines = appendList(lines, "Verification", pack.Verification)
	if summary := c.ModelSummaryForTools(intersectStrings(pack.Tools, allowed)); strings.TrimSpace(summary) != "" {
		lines = append(lines, "Tools:\n"+summary)
	}
	lines = appendList(lines, "Examples", pack.Examples)
	return strings.Join(lines, "\n")
}

func defaultCapabilityPacks() []CapabilityPack {
	return []CapabilityPack{
		{
			Name:        "track",
			Title:       "Track operations",
			Domain:      "User-visible Vit-DAW tracks only; hidden engine tracks are not valid targets.",
			Description: "Create, list, rename, select, mute, solo, arm, volume-adjust, freeze, or delete tracks.",
			StateSlices: []string{
				"daw_state_summary.tracks[]: track_id, track_name/name, user_track_index, mute, solo, is_armed, volume_db",
				"current_selection.selected_track_id and selected_track_name for this/current/selected track",
				"recent_execution_result and execution_memory for last_created_track / active_work_target_track bindings",
			},
			Preconditions: []string{
				"Use explicit user-named or indexed tracks first; then selected_track_id; then a single unambiguous default.",
				"Ask for clarification before destructive or ambiguous track-targeted edits.",
				"Do not invent track_id values; use state, current_selection, or produced bindings.",
			},
			Effects: []string{
				"track.add and track.add_audio create a user-visible track and refresh state.",
				"rename/mute/solo/arm/volume mutate an existing track and are undoable.",
				"delete/freeze/unfreeze are higher-risk and require confirmation.",
			},
			ResultHints: []string{
				"Created tracks usually return track_id and track_name/name.",
				"Mutations may return status plus refreshed observed state; prefer user-facing names in replies.",
			},
			Verification: []string{
				"For creation, verify the returned track_id or track_name appears in refreshed tracks[].",
				"For existing-track mutations, verify status=ok and refreshed state when available.",
			},
			Tools: []string{
				"project.state", "track.list",
				"track.add", "track.add_audio", "track.rename", "track.delete",
				"track.mute", "track.solo", "track.arm", "track.volume",
				"track.freeze", "track.unfreeze",
			},
			Examples: []string{
				"\u65b0\u5efa\u4e00\u6761\u8f68\u9053 -> track.add {}",
				"\u628a\u7b2c\u4e00\u6761\u8f68\u9053\u9759\u97f3 -> track.mute {track_id: tracks[0].track_id, mute:true}",
			},
		},
		{
			Name:        "midi",
			Title:       "MIDI clip and note editing",
			Domain:      "Beat-based MIDI clips and notes inside user-visible tracks.",
			Description: "Create MIDI clips, import MIDI files, read notes, and apply note patches such as insert, delete, move, resize, transpose, quantize, or velocity edits.",
			StateSlices: []string{
				"daw_state_summary.tracks[].clips[]: clip_id/id, clip_name/name, type/clip_type, start, length, note_count, notes when available",
				"current_selection.selected_clip_id / selected_clip_ids / selected_clip_track_id",
				"current_selection.selected_track_id when a new MIDI clip should be created on the selected track",
				"execution_memory.last_created_clip and active_work_target_clip for dependent note writes",
			},
			Preconditions: []string{
				"Prefer selected_clip_id when the user says this/current MIDI clip.",
				"If writing notes and no MIDI clip exists, create one first with midi.create_clip on the selected or explicit track.",
				"Use time_unit:\"beats\" for note patch operations unless the user explicitly gives another unit.",
				"MIDI write/import/create operations require confirmation; the preview must describe the musical change.",
			},
			Effects: []string{
				"midi.create_clip creates a MIDI clip and produces last_created_clip binding.",
				"midi.apply_note_patch mutates notes using operations[] and refreshes state after confirmation.",
				"midi.read_notes is read-only and should be used to verify uncertain note writes before retrying.",
			},
			ResultHints: []string{
				"Note writes may return inserted_count, inserted_note_ids, track_id, clip_id, and notes.",
				"Readback results return notes[] or midi_notes[]; compare IDs/count before claiming completion.",
			},
			Verification: []string{
				"For note insertion, verify expected inserted_note_ids or inserted_count are present in refreshed clip notes or readback.",
				"Do not finish a create-clip-and-write-notes goal after only creating the clip.",
				"If verification is unverified, read current clip notes before retrying to avoid duplicates.",
			},
			Tools: []string{
				"project.state", "track.list",
				"midi.create_clip", "midi.insert_clip", "midi.import_file",
				"midi.apply_note_patch", "midi.write_clip_notes", "midi.read_notes", "midi.read_clip_notes", "midi.read_clip_data",
				"midi.insert_notes", "midi.delete_notes", "midi.move_notes", "midi.resize_notes", "midi.quantize", "midi.transpose", "midi.set_velocity", "midi.replace_region",
				"clip.select",
			},
			Examples: []string{
				"\u65b0\u5efa MIDI clip \u5e76\u8f93\u5165 8 \u4e2a\u97f3\u7b26 -> midi.create_clip, then midi.apply_note_patch with 8 insert_note operations",
				"\u628a\u5f53\u524d MIDI \u7247\u6bb5\u91cf\u5316 -> midi.apply_note_patch {clip_id:selected_clip_id,time_unit:\"beats\",operations:[{op:\"quantize_region\",...}]}",
			},
		},
		{
			Name:        "media",
			Title:       "Media pool artifacts",
			Domain:      "User-authorized media, document, and MIDI files that should become Vit media pool artifacts.",
			Description: "Register explicit local asset paths or index one user-provided asset folder so the assistant can return clickable preview cards.",
			StateSlices: []string{
				"current_selection.artifacts[] and current_selection.attachments[] contain already attached media context",
				"media_scope_key and project_path keep media pool records scoped to the active project",
			},
			Preconditions: []string{
				"Only use folder or file paths that the user supplied in the current request or existing context.",
				"Use media.register_assets for explicit file paths; use media.index_authorized_folder for a user-provided folder.",
				"Keep indexing bounded with limit; recursive should only be true when the user asks to include subfolders.",
			},
			Effects: []string{
				"Registered files become Vit artifacts and can be previewed from the media pool.",
				"The tools do not modify DAW tracks or clips; importing a media artifact into a track is a separate clip tool.",
			},
			ResultHints: []string{
				"Tool results return artifacts[] with id, kind, title, path, mime, summary, and scope fields.",
				"Replies should mention the count and let artifact cards carry paths/previews instead of listing long raw paths.",
			},
			Verification: []string{
				"Completion is evidenced by status=ok and one or more returned artifact summaries.",
				"If no previewable files are found, say that clearly and do not invent assets.",
			},
			Tools: []string{
				"artifact.list", "artifact.read", "artifact.extract",
				"media.register_assets", "media.index_authorized_folder",
			},
			Examples: []string{
				"Find audio/video assets in a provided folder -> media.index_authorized_folder {asset_location:\"...\",media_kinds:[\"audio\",\"video\"],limit:80}",
				"Register an explicit file for preview -> media.register_assets {file_path:\"...\"}",
			},
		},
		{
			Name:        "mix",
			Title:       "Observation-first mixing",
			Domain:      "Natural Ask Vit mixing conversations over selected clips/tracks, named tracks, focus tracks, track groups, or the full project.",
			Description: "Build a Mix Observation Context before proposing or executing mix moves. The mix tool family is read-only in this phase.",
			StateSlices: []string{
				"current_selection.selected_track_id / selected_clip_id for current or selected scope",
				"daw_state_summary.tracks[] for project context, role guesses, plugin chains, levels, peaks, and focus candidates",
				"mix.observe returns digest + catalog; mix.read fetches catalog entries; mix.derive computes local relationship packages",
			},
			Preconditions: []string{
				"For broad acoustic requests, call mix.observe before plugin search/load, volume changes, plugin profile learning, or parameter writes.",
				"Choose scope from intent: selected_track, selected_clip, named_track, track_group, full_project, or full_project_with_focus_track.",
				"Use project_context for current-track mixing; use full_project for overall mix questions; use full_project_with_focus_track when the user asks for vocal/lead/focus relationships.",
				"After observing, read only the catalog entries needed for the answer and derive relationships locally when comparing tracks or focus vs project.",
				"After the user confirms a concrete small mix move, use mix.propose_tick then mix.apply_tick rather than calling primitive track/plugin writes directly.",
			},
			Effects: []string{
				"mix.observe, mix.read, and mix.derive do not mutate the project.",
				"mix.propose_tick does not mutate the project; mix.apply_tick v1 can only run one small track_gain_adjust through set_volume, and mix.rollback_tick restores the previous dB.",
				"Observation may request fast audio features and may return partial/pending/missing slow packages; missing data must be stated as uncertainty.",
				"Do not load plugins, write parameters, or change volume in the same broad mixing request. Propose one small next move and wait for explicit confirmation.",
			},
			ResultHints: []string{
				"digest contains the default acoustic summary and available_detail readiness.",
				"catalog entries are stable keys such as observation.digest, project.tracks.summary, project.rankings.level, project.risks.headroom, and track.{id}.slow.time_energy.summary.",
				"relationship package types include rank_tracks, focus_vs_project, a_vs_b, group_overlap, and before_after.",
			},
			Verification: []string{
				"For Scenario A, response should include target acoustic facts, basic relation to project context, and one safe suggestion.",
				"For Scenario B, response should include project/master summary where available, track summaries, rankings, issue candidates, and first attention target.",
				"For Scenario C, identify a focus track or ask for clarification; if found, derive focus_vs_project before proposing a small move.",
			},
			Tools: []string{
				"project.state", "track.list",
				"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
				"mix.propose_tick", "mix.apply_tick", "mix.rollback_tick",
				"project.undo", "project.redo",
			},
			Examples: []string{
				"混一下当前轨道 -> mix.observe {scope:\"selected_track\", project_context:true}, then mix.read/derive as needed, then suggest one safe move",
				"帮我看整体混音 -> mix.observe {scope:\"full_project\"}, then mix.read project rankings and summaries",
				"让主唱更靠前 -> mix.observe {scope:\"full_project_with_focus_track\", focus_hint:{role:\"vocal\"}}, then mix.derive {type:\"focus_vs_project\"}",
			},
		},
	}
}

func appendList(lines []string, title string, items []string) []string {
	items = nonEmptyStrings(items)
	if len(items) == 0 {
		return lines
	}
	lines = append(lines, title+":")
	for _, item := range items {
		lines = append(lines, "- "+item)
	}
	return lines
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func intersectStrings(values []string, allowed map[string]bool) []string {
	if len(values) == 0 {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		if len(allowed) > 0 && !allowed[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func differenceStrings(values []string, excluded []string) []string {
	blocked := stringSet(excluded)
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] || blocked[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
