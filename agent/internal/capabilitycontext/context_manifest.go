package capabilitycontext

const (
	ContextManifestSchemaVersion    = "capability_context_manifest.v0"
	CapabilityContextBuilderVersion = "capability_context_builder.v0"
)

type ContextManifest struct {
	SchemaVersion  string              `json:"schema_version"`
	ManifestID     string              `json:"manifest_id"`
	CapabilityID   string              `json:"capability_id"`
	CapabilityName string              `json:"capability_name,omitempty"`
	PackSchema     string              `json:"pack_schema,omitempty"`
	DefaultBudget  Budget              `json:"default_budget,omitempty"`
	Sources        []ContextSourceSpec `json:"sources,omitempty"`
	RowSets        []ContextRowSetSpec `json:"row_sets,omitempty"`
	Merges         []ContextMergeSpec  `json:"merges,omitempty"`
	Fields         []ContextFieldSpec  `json:"fields,omitempty"`
	FollowUpTools  []string            `json:"follow_up_tools,omitempty"`
	Excluded       []string            `json:"excluded,omitempty"`
	Guidance       []string            `json:"guidance,omitempty"`
	Metadata       map[string]any      `json:"metadata,omitempty"`
}

type ContextSourceSpec struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type ContextRowSetSpec struct {
	ID          string     `json:"id"`
	SourceID    string     `json:"source_id"`
	Paths       [][]string `json:"paths,omitempty"`
	KeyAliases  []string   `json:"key_aliases,omitempty"`
	EvidenceRef string     `json:"evidence_ref,omitempty"`
	Description string     `json:"description,omitempty"`
}

type ContextMergeSpec struct {
	ID          string   `json:"id"`
	RowSetIDs   []string `json:"row_set_ids,omitempty"`
	KeyAliases  []string `json:"key_aliases,omitempty"`
	MergePolicy string   `json:"merge_policy,omitempty"`
	Description string   `json:"description,omitempty"`
}

type ContextFieldSpec struct {
	ID            string   `json:"id"`
	Aliases       []string `json:"aliases,omitempty"`
	Semantic      string   `json:"semantic,omitempty"`
	EvidenceKind  string   `json:"evidence_kind,omitempty"`
	Usage         string   `json:"usage,omitempty"`
	Primary       bool     `json:"primary,omitempty"`
	Writable      bool     `json:"writable,omitempty"`
	Approximate   bool     `json:"approximate,omitempty"`
	ControlTarget bool     `json:"control_target,omitempty"`
}

func normalizeContextManifest(manifest ContextManifest) ContextManifest {
	if manifest.SchemaVersion == "" {
		manifest.SchemaVersion = ContextManifestSchemaVersion
	}
	if manifest.PackSchema == "" {
		manifest.PackSchema = SchemaVersion
	}
	manifest.DefaultBudget = NormalizeBudget(manifest.DefaultBudget)
	return manifest
}

func StaticMixGainStagingContextManifest() ContextManifest {
	return normalizeContextManifest(ContextManifest{
		ManifestID:     "static_mix.gain_staging.context_manifest.v0",
		CapabilityID:   GainStagingCapabilityID,
		CapabilityName: "B1 Gain Staging",
		Sources: []ContextSourceSpec{
			{ID: "project_state", Description: "Current user-visible DAW project state, including track faders and clip gain.", Required: true},
			{ID: "mix_observation", Description: "Latest mix.observe result with compact project acoustic evidence."},
			{ID: "audio_analysis_status", Description: "Latest read-only project.audio_analysis_status result with DAD waveform metrics."},
			{ID: "reference_level_model", Description: "RLM Reference Level Model projection for B1.2 strict/coarse full-project source-level calibration."},
			{ID: "context_snapshot", Description: "Runtime context snapshot, including UI selection and compact projections."},
			{ID: "request_context", Description: "Current chat/request context."},
			{ID: "execution_memory", Description: "Recent agent execution memory and verification facts."},
		},
		RowSets: []ContextRowSetSpec{
			{
				ID:          "mix_project_tracks",
				SourceID:    "mix_observation",
				Paths:       [][]string{{"observation", "project_package", "tracks"}, {"project_package", "tracks"}, {"context_pack", "latest_observation", "project_package", "tracks"}},
				KeyAliases:  []string{"track_id", "id"},
				EvidenceRef: "mix.observe:project_package.tracks",
				Description: "Project-level track acoustic and relationship rows from mix.observe.",
			},
			{
				ID:          "audio_analysis_track_waveforms",
				SourceID:    "audio_analysis_status",
				Paths:       [][]string{{"track_waveform_envelopes"}, {"analysis_job", "track_waveform_envelopes"}},
				KeyAliases:  []string{"track_id", "source_track_id", "id"},
				EvidenceRef: "project.audio_analysis_status:track_waveform_envelopes",
				Description: "DAD waveform rows from project.audio_analysis_status, including full-project RMS/peak/headroom metrics when available.",
			},
			{
				ID:          "rlm_reference_level_rows",
				SourceID:    "reference_level_model",
				Paths:       [][]string{{"rows"}},
				KeyAliases:  []string{"track_id", "id"},
				EvidenceRef: "rlm:reference_level_rows",
				Description: "RLM per-track reference-level rows for B1.2 strict/coarse source-level calibration.",
			},
			{
				ID:          "project_state_tracks",
				SourceID:    "project_state",
				Paths:       [][]string{{"tracks"}, {"visible_tracks"}, {"track_summaries"}, {"daw_state_summary", "tracks"}, {"project_state", "tracks"}, {"shadow", "tracks"}},
				KeyAliases:  []string{"track_id", "id"},
				EvidenceRef: "project.state:tracks",
				Description: "Writable current track and clip state.",
			},
			{
				ID:          "context_snapshot_tracks",
				SourceID:    "context_snapshot",
				Paths:       [][]string{{"tracks"}, {"visible_tracks"}, {"track_summaries"}, {"daw_state_summary", "tracks"}, {"project_state", "tracks"}, {"shadow", "tracks"}},
				KeyAliases:  []string{"track_id", "id"},
				EvidenceRef: "context.snapshot:tracks",
				Description: "Compact fallback track rows already present in the runtime context snapshot.",
			},
		},
		Merges: []ContextMergeSpec{
			{
				ID:          "tracks",
				RowSetIDs:   []string{"context_snapshot_tracks", "mix_project_tracks", "audio_analysis_track_waveforms", "project_state_tracks"},
				KeyAliases:  []string{"track_id", "id", "track_name", "name"},
				MergePolicy: "override",
				Description: "Merged B1 track table. context snapshot is fallback, mix.observe and DAD status contribute acoustic facts, and project.state wins for current writable track/clip state.",
			},
		},
		Fields: []ContextFieldSpec{
			{ID: "track_id", Aliases: []string{"track_id", "id"}, Semantic: "stable user-visible track identifier", EvidenceKind: "identity", Usage: "row_key"},
			{ID: "track_name", Aliases: []string{"track_name", "name", "user_label"}, Semantic: "user-visible track label", EvidenceKind: "identity", Usage: "display"},
			{ID: "volume_db", Aliases: []string{"volume_db", "fader_db", "track_gain_db", "gain_db", "db"}, Semantic: "track fader level in dB", EvidenceKind: "track_gain", Usage: "B1.1 fader unity evidence; not B1.2 source calibration"},
			{ID: "pan", Aliases: []string{"pan", "pan_value", "balance"}, Semantic: "track pan/balance", EvidenceKind: "track_state", Usage: "read-only B1 state context"},
			{ID: "mute", Aliases: []string{"mute", "muted", "is_muted"}, Semantic: "mute state", EvidenceKind: "track_state", Usage: "eligibility"},
			{ID: "solo", Aliases: []string{"solo", "is_solo"}, Semantic: "solo state", EvidenceKind: "track_state", Usage: "state context"},
			{ID: "clip_gain_db", Aliases: []string{"clip_gain_db", "gain_db", "db"}, Semantic: "static clip gain before track processing", EvidenceKind: "clip_gain", Usage: "B1.2 writable source calibration target", Writable: true, ControlTarget: true},
			{ID: "integrated_lufs", Aliases: []string{"integrated_lufs"}, Semantic: "integrated loudness", EvidenceKind: "loudness", Usage: "primary B1.2 loudness reference", Primary: true},
			{ID: "approximate_lufs", Aliases: []string{"approximate_lufs", "lufs", "lufs_estimate"}, Semantic: "approximate loudness", EvidenceKind: "loudness", Usage: "approximate B1.2 loudness fallback", Approximate: true},
			{ID: "active_rms_dbfs", Aliases: []string{"active_rms_dbfs", "gated_rms_dbfs", "silence_gated_rms_dbfs"}, Semantic: "silence-gated RMS", EvidenceKind: "loudness", Usage: "preferred fallback when available"},
			{ID: "rms_dbfs", Aliases: []string{"rms_dbfs", "level_db", "rms_db"}, Semantic: "RMS level", EvidenceKind: "level", Usage: "B1.2 fallback loudness proxy"},
			{ID: "peak_dbfs", Aliases: []string{"peak_dbfs", "peak_db"}, Semantic: "sample peak level", EvidenceKind: "headroom", Usage: "headroom/safety evidence; last-resort source-level fallback"},
			{ID: "effective_static_peak_dbfs", Aliases: []string{"effective_static_peak_dbfs"}, Semantic: "source peak after current static clip and track gain", EvidenceKind: "headroom", Usage: "B1 completion and safety verification"},
			{ID: "headroom_db", Aliases: []string{"headroom_db"}, Semantic: "peak headroom", EvidenceKind: "headroom", Usage: "B1 safety risk"},
			{ID: "crest_db", Aliases: []string{"crest_db"}, Semantic: "crest factor", EvidenceKind: "dynamics", Usage: "transient/source-shape hint"},
			{ID: "effective_static_rms_dbfs", Aliases: []string{"effective_static_rms_dbfs"}, Semantic: "source RMS after current static clip and track gain", EvidenceKind: "level", Usage: "B1 post-write result verification"},
			{ID: "reference_level", Aliases: []string{"reference_level"}, Semantic: "RLM selected median reference level", EvidenceKind: "reference_level", Usage: "B1.2 strict/coarse calibration basis"},
			{ID: "target_clip_gain_db", Aliases: []string{"target_clip_gain_db"}, Semantic: "RLM target writable clip gain", EvidenceKind: "clip_gain", Usage: "B1.2 pending clip.gain.set_batch target", Writable: true, ControlTarget: true},
		},
		FollowUpTools: []string{
			"project.state",
			"mix.observe",
			"mix.read",
			"mix.derive",
			"clip.gain.read",
			"clip.gain.set",
			"clip.gain.set_batch",
			"track.group.list",
			"track.group.apply_control",
		},
		Excluded: []string{
			"raw waveform arrays",
			"time_segments",
			"spectrogram tiles",
			"full TOM tree",
			"long absolute file paths",
			"full project JSON",
		},
		Guidance: []string{
			"This pack is a default B1 starting context, not a restriction on follow-up tool use.",
			"Use B1 for reference-level calibration from the current project state: within a full B1 run, first reset target track faders to 0 dB, then calibrate source/clip/trim levels from objective meters.",
			"Track fader reset is an engineering state reset and may use track.group.apply_control mode=absolute db=0 after confirmation; it is not limited by small B2 mix-tick deltas.",
			"B1.2 source calibration uses same-metric LUFS/RMS/peak references and pending clip.gain.set/trim-style actions; it must not use track faders, track groups, or mix ticks as source calibration controls.",
			"RLM is the B1.2 reference-level projection. Strict mode uses LUFS or active/gated RMS when available; full-clip RMS is marked coarse, not silently treated as strict loudness.",
			"Do not turn B1 into B2 static balance: musical foreground/background volume relationships belong to static_mix.static_balance.",
			"Any project mutation must remain pending until the user confirms.",
		},
	})
}

func StaticMixStaticBalanceContextManifest() ContextManifest {
	return normalizeContextManifest(ContextManifest{
		ManifestID:     "static_mix.static_balance.context_manifest.v1",
		CapabilityID:   StaticBalanceCapabilityID,
		CapabilityName: "B2 Static Balance",
		PackSchema:     StaticBalanceContextPackSchema,
		DefaultBudget:  Budget{MaxDisclosedTracks: 12, MaxRankingRows: 8, MaxStringRunes: 96},
		Sources: []ContextSourceSpec{
			{ID: "project_state", Description: "Current full-project user-track faders and project structure.", Required: true},
			{ID: "tom_projection", Description: "Track organization and role hypotheses with confidence; deterministically rebuilt from current project state when absent."},
			{ID: "mom_projection", Description: "Whole-project relationship inventory plus typed static-level relationship projection; raw acoustic packages remain behind the observation boundary.", Required: true},
			{ID: "mix_style", Description: "Validated Vit Mix Style weights from a built-in or future external .vms module.", Required: true},
		},
		RowSets: []ContextRowSetSpec{
			{ID: "project_tracks", SourceID: "project_state", Paths: [][]string{{"tracks"}, {"visible_tracks"}, {"daw_state_summary", "tracks"}}, KeyAliases: []string{"track_id", "id"}, EvidenceRef: "project.state:tracks"},
			{ID: "tom_assignments", SourceID: "tom_projection", Paths: [][]string{{"group_proposals", "assignment_excerpt"}, {"full_assignment_manifest", "groups", "assignments"}}, KeyAliases: []string{"track_id", "id"}, EvidenceRef: "TOM:role_assignments"},
			{ID: "mom_compared_tracks", SourceID: "mom_projection", Paths: [][]string{{"multitrack_relation", "compared_tracks"}}, KeyAliases: []string{"track_id", "id"}, EvidenceRef: "MOM:multitrack_relation.compared_tracks"},
			{ID: "mom_static_levels", SourceID: "mom_projection", Paths: [][]string{{"static_level_relationship", "tracks"}}, KeyAliases: []string{"track_id", "id"}, EvidenceRef: "MOM:static_level_relationship.tracks"},
		},
		Merges: []ContextMergeSpec{{ID: "static_balance_tracks", RowSetIDs: []string{"project_tracks", "tom_assignments", "mom_compared_tracks", "mom_static_levels"}, KeyAliases: []string{"track_id", "id"}, MergePolicy: "project_identity_and_fader_plus_tom_role_plus_mom_relation_and_typed_static_level"}},
		Fields: []ContextFieldSpec{
			{ID: "track_id", Aliases: []string{"track_id", "id"}, Semantic: "stable track identifier", EvidenceKind: "identity", Usage: "row_key"},
			{ID: "role", Aliases: []string{"role_guess", "role_hypothesis", "role"}, Semantic: "track content/function hypothesis", EvidenceKind: "TOM", Usage: "functional relationship classification", Primary: true},
			{ID: "volume_db", Aliases: []string{"volume_db", "fader_db", "track_gain_db"}, Semantic: "current track fader", EvidenceKind: "project_state", Usage: "B2 writable target and stale-plan fingerprint", Writable: true, ControlTarget: true},
			{ID: "level_db", Aliases: []string{"effective_static_rms_dbfs"}, Semantic: "typed MOM static-level relationship value", EvidenceKind: "MOM static-level projection", Usage: "bounded relationship correction", Primary: true},
			{ID: "headroom_db", Aliases: []string{"effective_static_peak_dbfs"}, Semantic: "typed MOM effective static peak", EvidenceKind: "MOM static-level projection", Usage: "headroom gate"},
		},
		FollowUpTools: []string{"project.state", "mix.observe", "mix.read", "mix.derive", "mix.propose_tick", "mix.apply_tick", "mix.rollback_tick"},
		Excluded:      []string{"clip gain", "plugins", "pan", "automation", "raw waveform arrays", "spectrogram tiles", "full TOM tree", "direct track.volume"},
		Guidance: []string{
			"Run deterministic B2 readiness before solving; capability history is not a prerequisite.",
			"Use B2-specific MOM relationship, role, effective L1, fader, and technical-baseline readiness; do not reuse MOM's generic L3-aware action gate as a hard block.",
			"Classify functional relationships; do not encode universal instrument loudness laws.",
			"Mix Style changes bounded weights only and cannot bypass evidence or execution safety gates.",
			"Analyze every project track, then disclose only compact summaries and examples to the LLM.",
			"The LLM selects one solver candidate ID; the Action Compiler preserves its complete unique track_gain_adjust list, each within +/-2 dB.",
			"After confirmation, verify target faders through project.state and refresh MOM through one full-project mix.observe.",
		},
	})
}

func StaticMixPanLayoutContextManifest() ContextManifest {
	return normalizeContextManifest(ContextManifest{
		ManifestID:     "static_mix.pan_layout.context_manifest.v1",
		CapabilityID:   PanLayoutCapabilityID,
		CapabilityName: "B3 Pan Layout",
		PackSchema:     PanLayoutContextPackSchema,
		DefaultBudget:  Budget{MaxDisclosedTracks: 14, MaxRankingRows: 8, MaxStringRunes: 96},
		Sources: []ContextSourceSpec{
			{ID: "project_state", Description: "Current full-project track pan, channel, routing, and parent/group state.", Required: true},
			{ID: "tom_projection", Description: "Full track role/function assignments and confidence."},
			{ID: "mom_projection", Description: "Whole-project multitrack and stereo relationships.", Required: true},
			{ID: "stereo_evidence", Description: "Per-track balance/correlation evidence used as a mono-safety constraint."},
			{ID: "mix_style", Description: "Validated vit.mix_style.v1 pan_layout dimensions.", Required: true},
		},
		RowSets: []ContextRowSetSpec{
			{ID: "project_tracks", SourceID: "project_state", Paths: [][]string{{"tracks"}, {"visible_tracks"}, {"daw_state_summary", "tracks"}}, KeyAliases: []string{"track_id", "id"}, EvidenceRef: "project.state:tracks"},
			{ID: "tom_assignments", SourceID: "tom_projection", Paths: [][]string{{"group_proposals", "assignment_excerpt"}, {"full_assignment_manifest", "groups", "assignments"}}, KeyAliases: []string{"track_id", "id"}, EvidenceRef: "TOM:role_assignments"},
			{ID: "mom_compared_tracks", SourceID: "mom_projection", Paths: [][]string{{"multitrack_relation", "compared_tracks"}}, KeyAliases: []string{"track_id", "id"}, EvidenceRef: "MOM:multitrack_relation.compared_tracks"},
		},
		Merges: []ContextMergeSpec{{ID: "pan_layout_tracks", RowSetIDs: []string{"project_tracks", "tom_assignments", "mom_compared_tracks"}, KeyAliases: []string{"track_id", "id"}, MergePolicy: "project_identity_pan_routing_plus_tom_role_plus_mom_stereo_relation"}},
		Fields: []ContextFieldSpec{
			{ID: "track_id", Aliases: []string{"track_id", "id"}, Semantic: "stable track identifier", EvidenceKind: "identity", Usage: "row_key"},
			{ID: "role", Aliases: []string{"role_guess", "role_hypothesis", "role"}, Semantic: "track content/function hypothesis", EvidenceKind: "TOM", Usage: "center/pair/group classification", Primary: true},
			{ID: "pan", Aliases: []string{"pan", "pan_value", "track_pan"}, Semantic: "current normalized track pan", EvidenceKind: "project_state", Usage: "B3 writable target and stale-plan fingerprint", Writable: true, ControlTarget: true},
			{ID: "channel_count", Aliases: []string{"channel_count", "channels", "source_channel_count"}, Semantic: "mono/stereo source identity", EvidenceKind: "source_identity", Usage: "pan-bound selection"},
			{ID: "correlation", Aliases: []string{"correlation_estimate", "correlation", "stereo_correlation"}, Semantic: "stereo mono-compatibility evidence", EvidenceKind: "stereo_relation", Usage: "risk gate"},
			{ID: "parent_track_id", Aliases: []string{"parent_track_id", "parent_folder_track_id"}, Semantic: "routing/group parent", EvidenceKind: "routing", Usage: "double-pan and topology guard"},
		},
		FollowUpTools: []string{"project.state", "mix.observe", "mix.read", "mix.derive", "mix.apply_pan_layout_batch", "mix.rollback_tick"},
		Excluded:      []string{"track fader", "clip gain", "plugins", "stereo width mutation", "automation", "raw waveform arrays", "full TOM tree", "direct primitive track.pan"},
		Guidance: []string{
			"Run deterministic B3 readiness before solving; B1/B2 history is not a prerequisite.",
			"Analyze every project track, but leave unknown-role or stereo-risk tracks unchanged instead of truncating analysis.",
			"Center placement is a weighted prior, not a universal instrument law.",
			"The LLM selects an existing candidate_plan_id and never generates track IDs or pan values.",
			"B3 v1 changes track pan only; stereo width and mono compatibility are constraints and observations.",
			"After confirmation, verify every target through project.state and refresh MOM through full-project mix.observe.",
		},
	})
}

func StaticMixLowEndRelationContextManifest() ContextManifest {
	return normalizeContextManifest(ContextManifest{
		ManifestID:     "static_mix.low_end_relation.context_manifest.v0",
		CapabilityID:   LowEndRelationCapabilityID,
		CapabilityName: "B4 Low-End Relation",
		DefaultBudget:  Budget{MaxDisclosedTracks: 10, MaxRankingRows: 6, MaxStringRunes: 96},
		Sources: []ContextSourceSpec{
			{ID: "project_state", Description: "Current full-project track faders and structure.", Required: true},
			{ID: "mom_projection", Description: "Whole-project MOM with band occupancy and conflict candidates.", Required: true},
			{ID: "tom_projection", Description: "Track role assignments; enriches low-end track identification."},
		},
		RowSets: []ContextRowSetSpec{
			{ID: "project_tracks", SourceID: "project_state",
				Paths:      [][]string{{"tracks"}, {"visible_tracks"}, {"daw_state_summary", "tracks"}},
				KeyAliases: []string{"track_id", "id"}, EvidenceRef: "project.state:tracks"},
			{ID: "mom_band_occupancy", SourceID: "mom_projection",
				Paths:       [][]string{{"multitrack_relation", "band_occupancy"}},
				KeyAliases:  []string{"band"},
				EvidenceRef: "MOM:multitrack_relation.band_occupancy"},
			{ID: "tom_assignments", SourceID: "tom_projection",
				Paths:       [][]string{{"group_proposals", "assignment_excerpt"}},
				KeyAliases:  []string{"track_id", "id"},
				EvidenceRef: "TOM:role_assignments"},
		},
		Fields: []ContextFieldSpec{
			{ID: "track_id", Aliases: []string{"track_id", "id"}, Semantic: "stable track identifier", EvidenceKind: "identity", Usage: "row_key"},
			{ID: "role", Aliases: []string{"role_guess", "role_hypothesis", "role"}, Semantic: "track content/function hypothesis", EvidenceKind: "TOM", Usage: "low-end source identification", Primary: true},
			{ID: "fader_db", Aliases: []string{"volume_db", "fader_db", "track_gain_db"}, Semantic: "current track fader", EvidenceKind: "project_state", Usage: "B4 state context"},
			{ID: "sub_unit_energy", Aliases: []string{"sub_unit_energy"}, Semantic: "normalized sub-band energy", EvidenceKind: "MOM_band", Usage: "low-end dominance evidence", Primary: true},
			{ID: "bass_unit_energy", Aliases: []string{"bass_unit_energy"}, Semantic: "normalized bass-band energy", EvidenceKind: "MOM_band", Usage: "low-end dominance evidence", Primary: true},
		},
		FollowUpTools: []string{"mix.observe", "mix.read", "mix.derive"},
		Excluded:      []string{"clip gain", "plugins", "stereo width", "automation", "raw waveform arrays"},
		Guidance: []string{
			"Evaluate B4 from current project and MOM evidence; B1, B2, or B3 history is not a prerequisite.",
			"B4 v0 produces analysis and observations only; no pending action or execution is created.",
			"Low-end identification is based on MOM band energy occupancy, not assumed role names.",
			"Masking conflicts are detected by MOM band_conflict_candidates narrowed to sub/bass bands.",
			"Suggested remediation directions (EQ, fader, pan) belong to B2/B3/SPAL; B4 does not execute them.",
		},
	})
}

func FineMixFrequencyCleanupContextManifest() ContextManifest {
	return normalizeContextManifest(ContextManifest{
		ManifestID:     "fine_mix.frequency_cleanup.context_manifest.v1",
		CapabilityID:   FrequencyCleanupCapabilityID,
		CapabilityName: "C1 Frequency Cleanup",
		DefaultBudget:  Budget{MaxStringRunes: 160, MaxRankingRows: 8},
		Sources: []ContextSourceSpec{
			{ID: "project_state", Description: "Current full-project identity, routing and plug-in state.", Required: true},
			{ID: "mom_frequency_relationship", Description: "Whole-project six-region frequency relationship projection with explicit tap and coverage.", Required: true},
			{ID: "tom_projection", Description: "Track role hypotheses used as context, never as acoustic truth."},
			{ID: "mixboard_decisions", Description: "Prior capability decisions and dimensions that require revalidation."},
		},
		RowSets: []ContextRowSetSpec{
			{ID: "project_tracks", SourceID: "project_state", Paths: [][]string{{"tracks"}, {"visible_tracks"}, {"daw_state_summary", "tracks"}}, KeyAliases: []string{"track_id", "id"}, EvidenceRef: "project.state:tracks"},
			{ID: "frequency_track_profiles", SourceID: "mom_frequency_relationship", Paths: [][]string{{"frequency_relationship", "track_profiles"}}, KeyAliases: []string{"track_id"}, EvidenceRef: "MOM:frequency_relationship.track_profiles"},
			{ID: "frequency_conflicts", SourceID: "mom_frequency_relationship", Paths: [][]string{{"frequency_relationship", "conflict_candidates"}}, KeyAliases: []string{"conflict_id"}, EvidenceRef: "MOM:frequency_relationship.conflict_candidates"},
		},
		Fields: []ContextFieldSpec{
			{ID: "track_id", Aliases: []string{"track_id", "id"}, Semantic: "stable project track identifier", EvidenceKind: "identity", Usage: "whole-project decision key"},
			{ID: "role", Aliases: []string{"role_guess", "role_hypothesis", "role"}, Semantic: "track function hypothesis", EvidenceKind: "TOM", Usage: "context only"},
			{ID: "tap_point", Aliases: []string{"tap_point"}, Semantic: "frequency evidence observation point", EvidenceKind: "MOM", Usage: "readiness and before/after comparability", Primary: true},
			{ID: "frequency_bands", Aliases: []string{"bands"}, Semantic: "six-region normalized frequency evidence", EvidenceKind: "MOM", Usage: "C1 diagnosis", Primary: true},
		},
		FollowUpTools: []string{"project.state", "mix.observe", "mix.read", "mix.derive"},
		Excluded:      []string{"dynamic EQ parameters", "compression parameters", "space processing", "automation", "vendor parameter rules", "raw waveform arrays", "observer duplication"},
		Guidance: []string{
			"Evaluate C1 diagnosis and planning from current full-project frequency evidence; B1-B4 history is not a prerequisite.",
			"Classify every project track explicitly as static_eq, a C2/C3/C4 deferral, arrangement_or_source, or no_change.",
			"Energy overlap is a diagnostic candidate, not proof of psychoacoustic masking.",
			"Diagnosis may use source_file_pre_fx evidence, but mutation requires target-scoped same-tap post-FX L2 evidence selected after classification.",
			"Only static_eq items may enter the shared generic static-EQ execution path.",
			"C1 must reuse existing EQ qualification, topology, materialization, preimage, execution, rollback and recovery.",
		},
	})
}
