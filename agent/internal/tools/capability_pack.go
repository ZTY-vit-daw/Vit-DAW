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
			Name:        "project_marker",
			Title:       "Project timeline markers and A5 section marker writing",
			Domain:      "Project-level timeline markers. Position markers have start_seconds only; range/section markers have start_seconds and end_seconds.",
			Description: "Read, create, rename, delete, and apply confirmed A5/EPM section-map recommendations as timeline range markers without moving clips.",
			StateSlices: []string{
				"project.timeline snapshots may expose markers[] with marker_id, name, kind, start_seconds, optional end_seconds, duration_seconds, color, and confidence",
				"recent_goal_context.execution_memory.pending_section_markers stores complete A5/EPM section candidates after an import or section-map recommendation",
			},
			Preconditions: []string{
				"When the user confirms an A5/EPM section-map proposal, use project.markers.apply_section_markers with the complete pending_section_markers.sections manifest.",
				"Do not move clips, trim clips, or reorganize tracks when writing markers.",
				"Use project.markers.upsert for one manual marker; omit end_seconds for a position marker.",
			},
			Effects: []string{
				"project.markers.apply_section_markers writes range markers and may replace existing epm_a5 markers by source.",
				"project.markers.upsert creates or updates one marker; rename/delete affect marker metadata only.",
			},
			ResultHints: []string{
				"After a successful A5 marker write, reply with the written marker count and avoid exposing marker IDs unless asked.",
			},
			Verification: []string{
				"Marker writes are evidenced by status=ok and written_count or returned markers.",
				"Refreshed project state should include markers[] for GUI marker lane/list rendering.",
			},
			Tools: []string{
				"project.state",
				"project.markers.list", "project.markers.upsert", "project.markers.apply_section_markers",
				"project.markers.rename", "project.markers.delete",
			},
			Examples: []string{
				"Write confirmed A5 sections -> project.markers.apply_section_markers {sections: pending_section_markers.sections, replace_existing:true, source:\"epm_a5\"}",
				"Add a position marker -> project.markers.upsert {name:\"Chorus\", start_seconds:64.0}",
			},
		},
		{
			Name:        "project_audio",
			Title:       "Project audio settings, import preflight, stems import, and automatic DAD status",
			Domain:      "Project-level audio specification metadata, local media preflight, single-command stems import, and read-only deferred analysis status; not audio-device configuration.",
			Description: "Read, validate, update, and persist project audio settings, inspect stems folders, generate import plans, import stems as one track and clip per file, then inspect the automatic DAD/deferred audio-analysis queue without starting it.",
			StateSlices: []string{
				"project.get_audio_settings returns audio_settings with sample_rate_hz, record_bit_depth, record_file_type, pcm_format, import policies, and field/capability status",
				"project.import_preflight returns summary and import_plan for stems_folder planning without mutating the project",
				"media.inspect_files returns local metadata only; it does not register artifacts or import media",
				"project.import_folder_as_stems creates user-visible audio tracks and one aligned clip per readable source file",
				"project.audio_analysis_status returns lightweight submitted/pending progress for deferred stems analysis",
			},
			Preconditions: []string{
				"Use project.get_audio_settings before explaining or changing project sample-rate/bit-depth/record-format policy.",
				"Use project.validate_audio_settings_change before project.set_audio_settings when changing sample_rate_hz or record_bit_depth.",
				"Use project.import_preflight before importing stems folders, then use project.import_folder_as_stems for the actual write; do not simulate folder import by repeated clip.import_audio or clip.import_media_to_track calls.",
				"After stems import, DAD analysis is automatic; use project.audio_analysis_status only to read queue progress, and do not start or cancel DAD analysis from the agent.",
				"Do not treat project sample_rate_hz as the audio device sample rate.",
				"Do not read full audio into the model; rely on kernel metadata rows and compact summaries.",
			},
			Effects: []string{
				"project.set_audio_settings updates project metadata and saves it with the project; it does not rewrite existing media.",
				"project.import_preflight and media.inspect_files are read-only metadata probes and do not create tracks or clips.",
				"project.import_folder_as_stems mutates the project and creates tracks/clips in one kernel action; DAD feature generation is owned by the automatic fact layer.",
				"project.audio_analysis_status only reports pending background-analysis submissions; it does not prove final acoustic bake completion.",
				"Warnings should be presented as user-facing risks, not raw internal JSON dumps.",
			},
			ResultHints: []string{
				"Summarize sample rate, bit depth, file type, import policy, device-rate warning, and migration/defaulted state.",
				"For preflight, report discovered/readable/unreadable counts, tracks_to_create, mismatch counts, confirmation need, copy policy, and a few example files/tracks.",
				"For stems import, report tracks_created, clips_created, edit_length_seconds, mismatch counts, copy policy, analysis_job_id, and a few imported track/clip names.",
				"For background analysis, report analysis_queue_status plus submitted_clips/total_clips; avoid claiming bottom-level bake completion when only submissions are tracked.",
			},
			Verification: []string{
				"Audio settings read/change is evidenced by status=ok and returned audio_settings matching the requested fields.",
				"Preflight completion is evidenced by status=ok, summary.discovered_audio_file_count, import_plan.tracks_to_create, and mismatch/unreadable counts.",
				"Actual stems import is evidenced by status=ok, summary.tracks_created/clips_created, created IDs, and refreshed project state containing the new tracks/clips.",
				"Deferred analysis queue progress is evidenced by status=ok, analysis_job_id, analysis_queue_status, and submitted counts; final A2/TIM readiness requires DAD acoustic facts, not queue status alone.",
			},
			Tools: []string{
				"project.state",
				"project.get_audio_settings", "project.validate_audio_settings_change", "project.set_audio_settings",
				"project.import_preflight", "project.import_folder_as_stems", "project.import_audio_files",
				"project.audio_analysis_status",
				"media.inspect_files",
			},
			Examples: []string{
				"Read project audio spec -> project.get_audio_settings {}",
				"Set 48 kHz / 24-bit WAV-BWF metadata -> project.validate_audio_settings_change, then project.set_audio_settings with audio_settings",
				"Preflight a stems folder -> project.import_preflight {folder_path:\"...\", intended_mode:\"stems_folder\", target_policy:\"create_tracks\", start_time_seconds:0}",
				"Import a confirmed stems folder -> project.import_folder_as_stems {folder_path:\"...\", target_policy:\"create_tracks\", start_time_seconds:0}",
				"Read automatic DAD queue status -> project.audio_analysis_status {analysis_job_id:\"...\"}",
			},
		},
		{
			Name:        "track",
			Title:       "Track operations",
			Domain:      "User-visible Vit-DAW hybrid tracks and track-folder containers only; hidden engine tracks are not valid targets.",
			Description: "Create, list, rename, select, mute, solo, arm, volume-adjust, freeze, delete, organize tracks into folders, create track control groups, or enable folder bus routing.",
			StateSlices: []string{
				"daw_state_summary.tracks[]: track_id, track_name/name, user_track_index, mute, solo, is_armed, volume_db, pan",
				"daw_state_summary.track_groups[]: group_id, name, member_track_ids, linked_controls, enabled, suspended",
				"tracks[] expose track_group_ids/track_group_names when they belong to persistent control groups",
				"Folder rows expose track_type=folder, is_folder_container, parent_track_id, depth, child_track_ids, and routing_bus_enabled",
				"current_selection.selected_track_id and selected_track_name for this/current/selected track",
				"recent_execution_result and execution_memory for last_created_track / active_work_target_track bindings",
			},
			Preconditions: []string{
				"Use explicit user-named or indexed tracks first; then selected_track_id; then a single unambiguous default.",
				"Ask for clarification before destructive or ambiguous track-targeted edits.",
				"Do not invent track_id values; use state, current_selection, or produced bindings.",
				"Treat folders as containers around hybrid tracks. Create ordinary folder containers by default; enable folder bus routing only when the user asks for routing/submix behavior.",
			},
			Effects: []string{
				"track.add and track.add_audio create a user-visible track and refresh state.",
				"track.folder.create creates a folder container; track.move_to_folder changes the track tree without moving clips or media.",
				"project.apply_track_organization applies a confirmed TOM grouping proposal by creating folder containers and moving the listed tracks.",
				"track.folder.set_routing_bus_enabled turns a folder container into a bus/submix folder or back to a plain container, with plugin-clear protection when disabling.",
				"track.group.create/update/set_members/delete manage persistent control groups; track.group.apply_control applies a confirmed volume operation directly to member track faders and can create/reuse a group from explicit track_ids.",
				"rename/mute/solo/arm/volume/pan mutate an existing track and are undoable.",
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
				"project.state", "project.audio_analysis_status", "project.audio_analysis_start", "track.list",
				"track.add", "track.add_audio", "track.rename", "track.delete",
				"track.folder.create", "track.move_to_folder", "track.folder.set_routing_bus_enabled",
				"project.apply_track_organization",
				"track.group.list", "track.group.create", "track.group.update", "track.group.set_members", "track.group.delete", "track.group.apply_control",
				"track.mute", "track.solo", "track.arm", "track.volume", "track.pan",
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
				"Use project.import_preflight for stems folders or folder-level audio import planning, then project.import_folder_as_stems for actual stems import; do not use repeated single-track imports to simulate stems import.",
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
				"project.import_preflight", "project.import_folder_as_stems", "media.inspect_files",
			},
			Examples: []string{
				"Find audio/video assets in a provided folder -> media.index_authorized_folder {asset_location:\"...\",media_kinds:[\"audio\",\"video\"],limit:80}",
				"Register an explicit file for preview -> media.register_assets {file_path:\"...\"}",
			},
		},
		{
			Name:        "static_mix_gain_staging",
			Title:       "B1 Gain Staging default context and pending gain actions",
			Domain:      "Static mix capability layer for source/track gain health, clipping/headroom risk, very low source level, and gain outliers.",
			Description: "Use a deterministic B1 capability context pack as the default starting observation, then let the model request additional allowed observations or prepare pending gain-staging actions.",
			StateSlices: []string{
				"capability_context_pack schema static_mix.gain_staging.v0 contains compact track peak/RMS/LUFS/headroom, track volume, primary clip gain, source-level calibration rankings, evidence status, and limitations",
				"mix.observe returns project-level acoustic digest/catalog; mix.read can fetch project.tracks.summary, project.risks.headroom, project.rankings.peak, and project.acoustic.tracks",
				"project.state exposes current track/clip state including track volume and clip gain when available",
			},
			Preconditions: []string{
				"The B1 context pack is a default observation pack, not a restriction on follow-up tool use.",
				"Keep B1 separate from B2 static balance: do not make musical foreground/background volume decisions under B1.",
				"B1 fader unity reset is an engineering state reset: use a target track group and a pending track.group.apply_control absolute volume=0 dB action, not a +/-2 dB mix tick.",
				"B1.2 source calibration starts after fader unity: compare same-metric full-project references, then use pending clip.gain.set_batch or clip.gain.set/trim-style actions; do not use track faders, track groups, or mix ticks for B1.2 source calibration.",
				"Use clip.gain.read before explaining or changing a specific clip's static source gain when project.state does not already expose it.",
				"All project mutations must remain pending until the user confirms.",
			},
			Effects: []string{
				"Context pack generation is read-only and should avoid raw waveform arrays, spectrogram tiles, full TOM trees, full project dumps, and long absolute paths.",
				"clip.gain.set and clip.gain.set_batch change static clip gain before track processing and require confirmation.",
				"track.group.apply_control mode=absolute db=0 can create/reuse a B1 reference-level group and directly reset member track faders after confirmation.",
			},
			ResultHints: []string{
				"Report B1 findings as reference-level calibration facts: fader unity status, source-level reference candidates, clipping/headroom risk, very low levels, large track offsets, and large clip gain offsets.",
				"State missing/partial evidence explicitly and call additional allowed tools when the pack is too narrow.",
				"Suggested actions should explain whether the next B1 step is fader unity reset, clip gain/trim calibration, or more observation.",
			},
			Verification: []string{
				"Read-only B1 observation is evidenced by a capability_context_pack with capability_id static_mix.gain_staging.v0 and non-empty evidence_status.",
				"Confirmed B1.2 clip gain changes should be verified through refreshed project.state and, when available, a fresh mix.observe evidence read.",
				"Confirmed B1 fader reset should be verified through track.group.apply_control members[].after_db and refreshed project.state track volume_db=0 dB.",
			},
			Tools: []string{
				"project.state", "track.list",
				"mix.observe", "mix.read", "mix.derive", "mix.report", "mix.request_observation",
				"clip.gain.read", "clip.gain.set", "clip.gain.set_batch",
				"track.group.list", "track.group.apply_control",
				"project.undo", "project.redo",
			},
			Examples: []string{
				"Check gain staging -> use the B1 capability_context_pack, then mix.read project.risks.headroom if more detail is needed",
				"B1 reset all track faders to unity -> track.group.apply_control {track_ids:[...], create_group_if_missing:true, replace_members:true, control:\"volume\", mode:\"absolute\", db:0}",
				"B1.2 source-level offset -> compare LUFS/RMS/peak reference rows, then propose one pending clip.gain.set_batch that applies each track delta to every surviving audio clip; never use track.volume or mix.propose_tick for this calibration",
			},
		},
		{
			Name:        "static_mix_static_balance",
			Title:       "B2 Static Balance with Mix Style",
			Domain:      "Whole-project musical level relationships when the current project satisfies the B2 readiness contract.",
			Description: "Evaluate B2 readiness, build an all-track StaticBalanceModel from project.state, TOM, and the typed MOM static-level relationship projection, apply a bounded Vit Mix Style (.vms), solve deterministic candidates, then disclose a compact CCB for LLM candidate selection.",
			StateSlices: []string{
				"TOM role assignments and confidence establish track content hypotheses such as lead vocal, drums, bass, strings, harmonic beds, and support layers",
				"MOM multitrack_relation provides the full-project relationship inventory; its generic L3-aware action-preflight flag is advisory for B2",
				"MOM static_level_relationship provides observation-layer clip aggregation, effective RMS/peak values, metric/tap identity, freshness, evidence references, and project-cut identity; raw DAD rows remain behind MOM",
				"project.state provides current track fader values and the project fingerprint used to expire stale plans",
				"Mix Style schema vit.mix_style.v1 (.vms) supplies bounded weights over fixed relationship dimensions",
			},
			Preconditions: []string{
				"Evaluate current-state B2 readiness before evidence assembly and solving. B1 is one remediation path for an unhealthy technical baseline, not a required history predecessor.",
				"B2 is musical static balance, not B1 source calibration: never change clip gain or trim under B2.",
				"Do not assume universal instrument loudness laws; reason over foreground, rhythm anchor, low-end anchor, harmonic bed, support, and effects functions.",
				"If B2-specific role/function coverage, effective L1 levels, current faders, technical baseline, or MOM multitrack relationships are insufficient, report assumptions and limitations without creating an executable pending plan.",
				"Every fader action must be pending until explicit user confirmation; each action is bounded to +/-2 dB, but all-track analysis and complete pending plans have no disclosure-derived track cap.",
			},
			Effects: []string{
				"Readiness, all-track modeling, deterministic solving, compact CCB disclosure, and VMS selection are read-only.",
				"The LLM may select only a disclosed candidate_plan_id; the local Action Compiler resolves and validates the complete action list.",
				"After confirmation, each plan action runs through mix.propose_tick then mix.apply_tick at the track fader layer.",
				"B2 v1 does not change clip gain, group/VCA controls, plugins, pan, automation, or direct primitive track.volume commands.",
			},
			ResultHints: []string{
				"Present the selected Mix Style, role/function assignment, per-track delta, before/target fader values, evidence source, and limitations.",
				"Treat automatic style matching as advisory unless the user explicitly names a style; otherwise use neutral.vms.",
				"One confirmation applies the complete multi-track plan; do not turn B2 into a chain of separately confirmed single-track suggestions.",
			},
			Verification: []string{
				"After all actions, read project.state and verify every target fader within tolerance.",
				"Run one fresh full-project mix.observe/MOM read after the plan and report whether both state and acoustic re-observation succeeded.",
			},
			Tools: []string{
				"project.state", "mix.observe", "mix.read", "mix.derive", "mix.request_observation",
				"mix.propose_tick", "mix.apply_tick", "mix.rollback_tick", "project.undo", "project.redo",
			},
			Examples: []string{
				"基于 B1 做现代流行 B2 静态平衡 -> read TOM/MOM/project.state, load modern_pop.vms, prepare one multi-track pending fader plan",
				"做全工程静态音量平衡 -> use neutral.vms when no style is explicit; block pending when role or MOM evidence is insufficient",
			},
		},
		{
			Name:        "static_mix_pan_layout",
			Title:       "B3 Static Pan Layout with Mix Style",
			Domain:      "Whole-project musical pan relationships when the current project satisfies the B3 readiness contract.",
			Description: "Evaluate B3 readiness, build the full-project pan_layout.model.v1 from TOM, MOM, project.state, channel identity, and stereo evidence, apply Vit Mix Style (.vms) constraints, solve deterministic candidates, then disclose a compact CCB for LLM candidate selection.",
			StateSlices: []string{
				"TOM role/function assignments identify center anchors, complementary pairs, support layers, rhythm peripherals, and unresolved tracks",
				"MOM multitrack_relation provides full-project relationship evidence; generic projection summaries are inputs rather than candidate actions",
				"project.state provides current track pan, track/channel identity, parent routing, and stale-plan fingerprints",
				"stereo balance and correlation evidence constrain risky stereo sources and mono compatibility; B3 v1 does not execute width changes",
				"Mix Style schema vit.mix_style.v1 (.vms), capability vit.mix_style.pan_layout.v1, supplies bounded pan-layout dimensions",
			},
			Preconditions: []string{
				"Evaluate current-state B3 Readiness before solving. B1 or B2 history is not required; their current-state outcomes may satisfy relevant conditions.",
				"Analyze every current track in pan_layout.model.v1. Unknown or risky tracks may be conservatively excluded from movement without truncating the project model.",
				"Center anchors must remain within the VMS center constraint; paired/support roles may move only within solver and stereo-safety bounds.",
				"If current pan, functional relationship coverage, or full-project MOM evidence cannot support any safe candidate, report failed readiness conditions without pending actions.",
				"Every action remains pending until explicit user confirmation; CCB disclosure size must never cap the complete candidate action list.",
			},
			Effects: []string{
				"Readiness, all-track modeling, deterministic solving, compact CCB disclosure, and VMS selection are read-only.",
				"The LLM may select only one disclosed candidate_plan_id; the local Action Compiler resolves and validates the complete solver-owned action list.",
				"One confirmation executes the complete plan through mix.apply_pan_layout_batch, backed by one atomic track.pan.set_batch Kernel transaction.",
				"B3 v1 changes track pan only; it never changes track fader, clip gain, plugins, automation, stereo width, or primitive single-track pan commands.",
			},
			ResultHints: []string{
				"Present the selected Mix Style, center anchors, pair/group layout, action count, important exclusions, assumptions, and limitations.",
				"Treat automatic style matching as advisory unless the user explicitly names a style; otherwise use neutral.vms.",
				"Keep local requests such as moving one guitar left in the generic mix-tick capability; B3 is a coherent full-project layout.",
			},
			Verification: []string{
				"After the atomic batch, read project.state and verify every target pan within 0.01.",
				"Run one fresh full-project mix.observe/MOM read and report state verification and acoustic re-observation separately.",
				"Verify non-target controls were not changed and expire the pending plan after success, failure, rejection, or stale-baseline detection.",
			},
			Tools: []string{
				"project.state", "project.audio_analysis_status",
				"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
				"mix.apply_pan_layout_batch", "project.undo", "project.redo",
			},
			Examples: []string{
				"进行 B3 -> read TOM/MOM/project.state/channel/stereo evidence, build pan_layout.model.v1, select one deterministic pending candidate",
				"按现代流行做全工程声像布局 -> load modern_pop.vms pan_layout rules; one confirmation applies one complete track-pan batch",
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
				"mix.propose_tick does not mutate the project; mix.apply_tick can run one small track_gain_adjust through set_volume or one track_pan_adjust/track_pan_set through set_pan, and mix.rollback_tick restores the previous value.",
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
