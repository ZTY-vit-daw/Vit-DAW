package capabilitycontext

import (
	"math"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/rlm"
)

type GainStagingInput struct {
	UserIntent          string
	ProjectState        map[string]any
	MixObservation      map[string]any
	AudioAnalysisStatus map[string]any
	ContextSnapshot     map[string]any
	RequestContext      map[string]any
	ExecutionMemory     map[string]any
	GeneratedAt         time.Time
	Budget              Budget
}

type TrackGainRow struct {
	TrackID        string         `json:"track_id,omitempty"`
	TrackName      string         `json:"track_name,omitempty"`
	UserTrackIndex any            `json:"user_track_index,omitempty"`
	RoleGuess      string         `json:"role_guess,omitempty"`
	TrackType      string         `json:"track_type,omitempty"`
	Selected       bool           `json:"selected,omitempty"`
	Mute           bool           `json:"mute,omitempty"`
	Solo           bool           `json:"solo,omitempty"`
	VolumeDB       *float64       `json:"volume_db,omitempty"`
	Pan            *float64       `json:"pan,omitempty"`
	PeakDBFS       *float64       `json:"peak_dbfs,omitempty"`
	ActiveRMSDBFS  *float64       `json:"active_rms_dbfs,omitempty"`
	RMSDBFS        *float64       `json:"rms_dbfs,omitempty"`
	IntegratedLUFS *float64       `json:"integrated_lufs,omitempty"`
	ApproxLUFS     *float64       `json:"approximate_lufs,omitempty"`
	HeadroomDB     *float64       `json:"headroom_db,omitempty"`
	CrestDB        *float64       `json:"crest_db,omitempty"`
	AcousticStatus string         `json:"acoustic_status,omitempty"`
	ClipCount      int            `json:"clip_count,omitempty"`
	AudioClipCount int            `json:"audio_clip_count,omitempty"`
	PrimaryClip    *ClipGainRow   `json:"primary_clip,omitempty"`
	ClipGainStats  map[string]any `json:"clip_gain_stats,omitempty"`
	Risks          []string       `json:"risks,omitempty"`
	EvidenceRefs   []string       `json:"evidence_refs,omitempty"`
}

type ClipGainRow struct {
	ClipID          string   `json:"clip_id,omitempty"`
	ClipName        string   `json:"clip_name,omitempty"`
	TrackID         string   `json:"track_id,omitempty"`
	GainDB          *float64 `json:"gain_db,omitempty"`
	StartSeconds    *float64 `json:"start_seconds,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	FadeInSeconds   *float64 `json:"fade_in_seconds,omitempty"`
	FadeOutSeconds  *float64 `json:"fade_out_seconds,omitempty"`
}

type RankRow struct {
	TrackID      string         `json:"track_id,omitempty"`
	TrackName    string         `json:"track_name,omitempty"`
	ClipID       string         `json:"clip_id,omitempty"`
	ClipName     string         `json:"clip_name,omitempty"`
	Kind         string         `json:"kind"`
	Value        float64        `json:"value"`
	Unit         string         `json:"unit,omitempty"`
	Risk         string         `json:"risk,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	EvidenceRefs []string       `json:"evidence_refs,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

func BuildGainStagingPack(in GainStagingInput) Pack {
	budget := NormalizeBudget(in.Budget)
	now := in.GeneratedAt
	if now.IsZero() {
		now = time.Now()
	}
	manifest := StaticMixGainStagingContextManifest()
	rlmProjection := rlm.Build(rlm.Input{
		UserIntent:          in.UserIntent,
		ProjectState:        in.ProjectState,
		MixObservation:      in.MixObservation,
		AudioAnalysisStatus: in.AudioAnalysisStatus,
		GeneratedAt:         now.UTC().Format(time.RFC3339Nano),
	})
	rlmContext := rlm.ContextProjectionMap(rlmProjection)
	contextBuild := BuildCapabilityContext(ContextInput{
		Manifest: manifest,
		Sources: map[string]any{
			"project_state":         in.ProjectState,
			"mix_observation":       in.MixObservation,
			"audio_analysis_status": in.AudioAnalysisStatus,
			"reference_level_model": rlmContext,
			"context_snapshot":      in.ContextSnapshot,
			"request_context":       in.RequestContext,
			"execution_memory":      in.ExecutionMemory,
		},
		Budget: budget,
	})
	projectState := bestProjectState(contextBuild.SourceMap("project_state"), contextBuild.SourceMap("context_snapshot"), contextBuild.SourceMap("request_context"))
	mixObservation := bestMixObservation(contextBuild.SourceMap("mix_observation"))
	mixProject := mapValue(mixObservation["project_package"])
	scope := gainStagingScope(in, projectState, mixObservation)

	contextRows := contextBuild.Rows("tracks")
	tracks := make([]TrackGainRow, 0, len(contextRows))
	for _, row := range contextRows {
		track := buildTrackGainRow(row.Data, nil, budget)
		if track.TrackID == "" && track.TrackName == "" {
			continue
		}
		track.EvidenceRefs = addUnique(track.EvidenceRefs, row.EvidenceRefs...)
		tracks = append(tracks, track)
	}
	for i := range tracks {
		tracks[i].Risks = gainStagingRisks(tracks[i])
	}
	allTracks := append([]TrackGainRow(nil), tracks...)
	sourceAdmission := gainStagingSourceLevelAdmission(allTracks)
	rankings := gainStagingRankings(allTracks, budget.MaxRankingRows, sourceAdmission, rlmProjection, gainStagingStrictReferenceCalibrationRequest(in.UserIntent))
	sortTrackRowsForPack(tracks)
	tracks = capRows(tracks, budget.MaxTracks)

	evidenceRefs := addUnique(gainStagingEvidenceRefs(projectState, mixObservation, mixProject), contextBuild.EvidenceRefs()...)
	evidenceRefs = addUnique(evidenceRefs, rlmProjection.EvidenceRefs...)
	evidenceStatus := gainStagingEvidenceStatus(projectState, mixObservation, allTracks, sourceAdmission)
	evidenceStatus["b1_2_reference_level_model"] = gainStagingRLMEvidenceStatus(rlmProjection)
	limitations := gainStagingLimitations(evidenceStatus, mixProject, budget, len(allTracks), len(tracks), sourceAdmission)
	limitations = addUnique(limitations, rlmProjection.Limitations...)
	pack := Pack{
		SchemaVersion:  SchemaVersion,
		CapabilityID:   GainStagingCapabilityID,
		CapabilityName: "B1 Gain Staging",
		GeneratedAt:    now.UTC().Format(time.RFC3339Nano),
		UserIntent:     compactText(in.UserIntent, budget.MaxStringRunes*2),
		Scope:          scope,
		Budget:         budget,
		EvidenceRefs:   evidenceRefs,
		EvidenceStatus: evidenceStatus,
		Summary: map[string]any{
			"track_count":                         len(allTracks),
			"returned_track_count":                len(tracks),
			"tracks_with_acoustics":               countTracksWithAcoustics(allTracks),
			"tracks_with_loudness":                countTracksWithLoudness(allTracks),
			"tracks_with_track_gain":              countTracksWithTrackGain(allTracks),
			"clips_with_clip_gain":                countClipsWithGain(allTracks),
			"b1_2_source_level_admission":         sourceAdmission.Summary(),
			"b1_2_reference_level_model":          gainStagingRLMSummary(rlmProjection),
			"headroom_risk_count":                 len(rankings["headroom_risk"]),
			"low_level_count":                     len(rankings["low_level"]),
			"source_level_outlier_count":          len(rankings["source_level_outliers"]),
			"source_level_strict_candidate_count": len(rankings["source_level_reference_calibration"]),
		},
		Tracks:          tracks,
		ReferenceLevel:  rlmContext,
		Rankings:        rankings,
		ClipGainSummary: clipGainSummary(allTracks),
		Limitations:     limitations,
		FollowUpTools:   append([]string(nil), manifest.FollowUpTools...),
		Excluded:        append([]string(nil), manifest.Excluded...),
		Guidance:        append([]string(nil), manifest.Guidance...),
	}
	applyContextBuilderMetadata(&pack, manifest)
	pack.PackID = stablePackID(pack.CapabilityID, in.UserIntent, pack.EvidenceRefs, now)
	return pack
}

func bestProjectState(candidates ...map[string]any) map[string]any {
	for _, candidate := range candidates {
		if len(candidate) == 0 {
			continue
		}
		for _, row := range []map[string]any{
			candidate,
			mapValue(candidate["project_state"]),
			mapValue(candidate["daw_state_summary"]),
			mapValue(candidate["shadow"]),
		} {
			if len(bestTrackRows(row)) > 0 {
				return row
			}
		}
	}
	for _, candidate := range candidates {
		if len(candidate) > 0 {
			return candidate
		}
	}
	return nil
}

func bestTrackRows(candidates ...map[string]any) []map[string]any {
	for _, candidate := range candidates {
		if len(candidate) == 0 {
			continue
		}
		for _, key := range []string{"tracks", "visible_tracks", "track_summaries"} {
			if rows := rowsValue(candidate[key]); len(rows) > 0 {
				return rows
			}
		}
		if daw := mapValue(candidate["daw_state_summary"]); len(daw) > 0 {
			if rows := bestTrackRows(daw); len(rows) > 0 {
				return rows
			}
		}
	}
	return nil
}

func bestMixObservation(result map[string]any) map[string]any {
	if len(result) == 0 {
		return nil
	}
	if observation := mapValue(result["observation"]); len(observation) > 0 {
		return observation
	}
	if pack := mapValue(result["context_pack"]); len(pack) > 0 {
		if observation := mapValue(pack["latest_observation"]); len(observation) > 0 {
			return observation
		}
	}
	return result
}

func tracksByID(rows []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range rows {
		id := firstText(row, "track_id", "id")
		if id == "" {
			continue
		}
		out[id] = row
	}
	return out
}

func buildTrackGainRow(stateRow map[string]any, mixTracks map[string]map[string]any, budget Budget) TrackGainRow {
	trackID := firstText(stateRow, "track_id", "id")
	mixRow := map[string]any(nil)
	if trackID != "" && mixTracks != nil {
		mixRow = mixTracks[trackID]
	}
	row := mergeTrackRows(stateRow, mixRow)
	track := TrackGainRow{
		TrackID:        firstText(row, "track_id", "id"),
		TrackName:      compactText(firstText(row, "track_name", "name", "user_label"), budget.MaxStringRunes),
		UserTrackIndex: row["user_track_index"],
		RoleGuess:      compactText(firstText(row, "role_guess", "role", "folder_role"), budget.MaxStringRunes),
		TrackType:      compactText(firstText(row, "track_type", "type"), budget.MaxStringRunes),
		Selected:       boolValue(row["selected"]),
		Mute:           boolValue(firstPresent(row, "mute", "muted", "is_muted")),
		Solo:           boolValue(firstPresent(row, "solo", "is_solo")),
	}
	if value, ok := firstNumber(row, "volume_db", "fader_db", "track_gain_db", "gain_db", "db"); ok {
		track.VolumeDB = ptrRound(value)
	}
	if value, ok := firstNumber(row, "pan", "pan_value", "balance"); ok {
		track.Pan = ptrRound(value)
	}
	applyAcousticFields(&track, row)
	acoustic := mapValue(row["acoustic"])
	if len(acoustic) > 0 {
		applyAcousticFields(&track, acoustic)
		track.AcousticStatus = firstText(acoustic, "status")
	}
	if track.AcousticStatus == "" {
		track.AcousticStatus = firstText(row, "acoustic_status", "status")
	}
	clips := clipRows(row)
	track.ClipCount = len(clips)
	clipGainRows := make([]ClipGainRow, 0, len(clips))
	for _, clip := range clips {
		if !clipLooksAudio(clip) {
			continue
		}
		track.AudioClipCount++
		clipRow := buildClipGainRow(clip, track.TrackID, budget)
		if clipRow.ClipID == "" && clipRow.ClipName == "" {
			continue
		}
		clipGainRows = append(clipGainRows, clipRow)
	}
	if len(clipGainRows) > 0 {
		primary := primaryClipGainRow(clipGainRows)
		track.PrimaryClip = &primary
		track.ClipGainStats = compactClipGainStats(clipGainRows)
	}
	return track
}

func mergeTrackRows(primary, secondary map[string]any) map[string]any {
	out := cloneMap(secondary)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range primary {
		if _, exists := out[key]; !exists || out[key] == nil || cleanText(out[key]) == "" {
			out[key] = value
		}
	}
	return out
}

func applyAcousticFields(track *TrackGainRow, row map[string]any) {
	if track == nil || len(row) == 0 {
		return
	}
	if value, ok := firstNumber(row, "peak_dbfs", "peak_db"); ok {
		track.PeakDBFS = ptrRound(value)
	} else if value, ok := firstNumber(row, "peak_abs", "peak"); ok {
		if db, dbOK := dbfsFromAbs(*value); dbOK {
			track.PeakDBFS = db
		}
	}
	if value, ok := firstNumber(row, "active_rms_dbfs", "gated_rms_dbfs", "silence_gated_rms_dbfs"); ok {
		track.ActiveRMSDBFS = ptrRound(value)
	}
	if value, ok := firstNumber(row, "rms_dbfs", "level_db", "rms_db"); ok {
		track.RMSDBFS = ptrRound(value)
	} else if value, ok := firstNumber(row, "rms"); ok {
		if db, dbOK := dbfsFromAbs(*value); dbOK {
			track.RMSDBFS = db
		}
	}
	if value, ok := firstNumber(row, "integrated_lufs"); ok {
		track.IntegratedLUFS = ptrRound(value)
	}
	if value, ok := firstNumber(row, "approximate_lufs", "lufs", "lufs_estimate"); ok {
		track.ApproxLUFS = ptrRound(value)
	}
	if value, ok := firstNumber(row, "headroom_db"); ok {
		track.HeadroomDB = ptrRound(value)
	} else if track.PeakDBFS != nil {
		track.HeadroomDB = headroomFromPeakDBFS(*track.PeakDBFS)
	}
	if value, ok := firstNumber(row, "crest_db"); ok {
		track.CrestDB = ptrRound(value)
	}
}

func ptrRound(value *float64) *float64 {
	if value == nil {
		return nil
	}
	next := round3(*value)
	return &next
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok && cleanText(value) != "" {
			return value
		}
	}
	return nil
}

func clipRows(row map[string]any) []map[string]any {
	for _, key := range []string{"clips", "clip_summaries", "audio_clips"} {
		if rows := rowsValue(row[key]); len(rows) > 0 {
			return rows
		}
	}
	if primary := mapValue(row["primary_clip"]); len(primary) > 0 {
		return []map[string]any{primary}
	}
	return nil
}

func clipLooksAudio(row map[string]any) bool {
	text := strings.ToLower(firstText(row, "type", "clip_type", "media_kind", "kind"))
	if text == "" {
		return true
	}
	return strings.Contains(text, "audio") || strings.Contains(text, "wave") || strings.Contains(text, "clip")
}

func buildClipGainRow(row map[string]any, trackID string, budget Budget) ClipGainRow {
	clip := ClipGainRow{
		ClipID:   firstText(row, "clip_id", "id", "item_id"),
		ClipName: compactText(firstText(row, "clip_name", "name", "file_name"), budget.MaxStringRunes),
		TrackID:  firstText(row, "track_id", "parent_track_id"),
	}
	if clip.TrackID == "" {
		clip.TrackID = trackID
	}
	if value, ok := firstNumber(row, "clip_gain_db", "gain_db", "db"); ok {
		clip.GainDB = ptrRound(value)
	}
	if value, ok := firstNumber(row, "start_seconds", "start", "start_time"); ok {
		clip.StartSeconds = ptrRound(value)
	}
	if value, ok := firstNumber(row, "duration_seconds", "length_seconds", "duration", "length"); ok {
		clip.DurationSeconds = ptrRound(value)
	}
	if value, ok := firstNumber(row, "fade_in_seconds", "fade_in"); ok {
		clip.FadeInSeconds = ptrRound(value)
	}
	if value, ok := firstNumber(row, "fade_out_seconds", "fade_out"); ok {
		clip.FadeOutSeconds = ptrRound(value)
	}
	return clip
}

func primaryClipGainRow(rows []ClipGainRow) ClipGainRow {
	if len(rows) == 0 {
		return ClipGainRow{}
	}
	best := rows[0]
	bestDuration := -1.0
	for _, row := range rows {
		duration := -1.0
		if row.DurationSeconds != nil {
			duration = *row.DurationSeconds
		}
		if duration > bestDuration {
			best = row
			bestDuration = duration
		}
	}
	return best
}

func compactClipGainStats(rows []ClipGainRow) map[string]any {
	values := []float64{}
	for _, row := range rows {
		if row.GainDB != nil {
			values = append(values, *row.GainDB)
		}
	}
	out := map[string]any{
		"audio_clip_count": len(rows),
		"known_gain_count": len(values),
	}
	if len(values) == 0 {
		out["status"] = "missing"
		return out
	}
	minValue, maxValue, sumAbs := values[0], values[0], 0.0
	for _, value := range values {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
		sumAbs += math.Abs(value)
	}
	out["status"] = "ready"
	out["min_db"] = round3(minValue)
	out["max_db"] = round3(maxValue)
	out["avg_abs_db"] = round3(sumAbs / float64(len(values)))
	return out
}

func gainStagingRisks(row TrackGainRow) []string {
	risks := []string{}
	if row.PeakDBFS != nil {
		switch {
		case *row.PeakDBFS >= -0.1:
			risks = append(risks, "possible_clipping_or_no_headroom")
		case *row.PeakDBFS >= -1.0:
			risks = append(risks, "peak_near_full_scale")
		}
	}
	if row.HeadroomDB != nil {
		switch {
		case *row.HeadroomDB <= 0.1:
			risks = append(risks, "possible_clipping_or_no_headroom")
		case *row.HeadroomDB < 1.0:
			risks = append(risks, "low_headroom")
		}
	}
	if row.ActiveRMSDBFS != nil && *row.ActiveRMSDBFS <= -42 {
		risks = append(risks, "very_low_active_rms")
	}
	if row.RMSDBFS != nil && *row.RMSDBFS <= -42 {
		risks = append(risks, "very_low_rms")
	}
	if row.PeakDBFS != nil && *row.PeakDBFS <= -60 {
		risks = append(risks, "near_silent_or_empty_source")
	}
	if row.VolumeDB != nil && math.Abs(*row.VolumeDB) >= 6 {
		risks = append(risks, "large_track_gain_offset")
	}
	if row.ClipGainStats != nil {
		if maxDB, ok := numberValue(row.ClipGainStats["max_db"]); ok && math.Abs(maxDB) >= 6 {
			risks = append(risks, "large_clip_gain_offset")
		}
		if minDB, ok := numberValue(row.ClipGainStats["min_db"]); ok && math.Abs(minDB) >= 6 {
			risks = append(risks, "large_clip_gain_offset")
		}
	}
	if row.PeakDBFS == nil && row.ActiveRMSDBFS == nil && row.RMSDBFS == nil && row.HeadroomDB == nil {
		risks = append(risks, "missing_acoustic_level_evidence")
	}
	return addUnique(nil, risks...)
}

func gainStagingRankings(tracks []TrackGainRow, limit int, sourceAdmission sourceLevelAdmission, rlmProjection rlm.Projection, strictReference bool) map[string][]RankRow {
	out := map[string][]RankRow{}
	for _, track := range tracks {
		if track.HeadroomDB != nil {
			risk := ""
			if *track.HeadroomDB <= 0.1 {
				risk = "high"
			} else if *track.HeadroomDB < 1 {
				risk = "medium"
			}
			if risk != "" {
				out["headroom_risk"] = append(out["headroom_risk"], rankFromTrack(track, "headroom_db", *track.HeadroomDB, "dB", risk, "smallest headroom"))
			}
		} else if track.PeakDBFS != nil && *track.PeakDBFS >= -1 {
			out["headroom_risk"] = append(out["headroom_risk"], rankFromTrack(track, "peak_dbfs", *track.PeakDBFS, "dBFS", "medium", "peak near full scale"))
		}
		if track.PeakDBFS != nil && *track.PeakDBFS >= -1 {
			risk := "medium"
			if *track.PeakDBFS >= -0.1 {
				risk = "high"
			}
			out["peak_risk"] = append(out["peak_risk"], rankFromTrack(track, "peak_dbfs", *track.PeakDBFS, "dBFS", risk, "highest peak"))
		}
		if track.ActiveRMSDBFS != nil && *track.ActiveRMSDBFS <= -42 {
			out["low_level"] = append(out["low_level"], rankFromTrack(track, "active_rms_dbfs", *track.ActiveRMSDBFS, "dBFS", "medium", "very low active RMS"))
		} else if track.RMSDBFS != nil && *track.RMSDBFS <= -42 {
			out["low_level"] = append(out["low_level"], rankFromTrack(track, "rms_dbfs", *track.RMSDBFS, "dBFS", "medium", "very low RMS"))
		}
		if track.VolumeDB != nil && math.Abs(*track.VolumeDB) >= 6 {
			out["track_gain_outliers"] = append(out["track_gain_outliers"], rankFromTrack(track, "volume_db", *track.VolumeDB, "dB", "medium", "large track gain offset"))
		}
		if track.PrimaryClip != nil && track.PrimaryClip.GainDB != nil && math.Abs(*track.PrimaryClip.GainDB) >= 6 {
			row := rankFromTrack(track, "clip_gain_db", *track.PrimaryClip.GainDB, "dB", "medium", "large primary clip gain offset")
			row.ClipID = track.PrimaryClip.ClipID
			row.ClipName = track.PrimaryClip.ClipName
			out["clip_gain_outliers"] = append(out["clip_gain_outliers"], row)
		}
	}
	out["source_level_outliers"] = gainStagingSourceLevelOutlierRows(sourceAdmission, 0)
	if strictReference {
		out["source_level_reference_calibration"] = gainStagingRLMCalibrationRows(rlmProjection)
	}
	sort.SliceStable(out["headroom_risk"], func(i, j int) bool { return out["headroom_risk"][i].Value < out["headroom_risk"][j].Value })
	sort.SliceStable(out["peak_risk"], func(i, j int) bool { return out["peak_risk"][i].Value > out["peak_risk"][j].Value })
	sort.SliceStable(out["low_level"], func(i, j int) bool { return out["low_level"][i].Value < out["low_level"][j].Value })
	sortByAbsDesc(out["track_gain_outliers"])
	sortByAbsDesc(out["clip_gain_outliers"])
	for key, rows := range out {
		if key == "source_level_outliers" || key == "source_level_reference_calibration" {
			continue
		}
		out[key] = capRows(rows, limit)
	}
	return out
}

func gainStagingRLMCalibrationRows(projection rlm.Projection) []RankRow {
	if !projection.Actionable || len(projection.Calibration) == 0 {
		return nil
	}
	rows := make([]RankRow, 0, len(projection.Calibration))
	for _, candidate := range projection.Calibration {
		if strings.TrimSpace(candidate.ClipID) == "" {
			continue
		}
		row := RankRow{
			TrackID:      candidate.TrackID,
			TrackName:    candidate.TrackName,
			ClipID:       candidate.ClipID,
			ClipName:     candidate.ClipName,
			Kind:         candidate.Metric + "_delta_to_reference",
			Value:        round3(candidate.RequestedDeltaDB),
			Unit:         "dB",
			Risk:         candidate.Risk,
			Reason:       "RLM strict/coarse reference-level calibration candidate",
			EvidenceRefs: append([]string(nil), candidate.EvidenceRefs...),
			Metadata: map[string]any{
				"reference_metric":        candidate.Metric,
				"reference_level":         round3(candidate.ReferenceLevel),
				"reference_unit":          candidate.Unit,
				"observed_level":          round3(candidate.ObservedLevel),
				"current_clip_gain_db":    round3(candidate.CurrentClipGainDB),
				"target_clip_gain_db":     round3(candidate.TargetClipGainDB),
				"requested_delta_db":      round3(candidate.RequestedDeltaDB),
				"applied_delta_db":        round3(candidate.AppliedDeltaDB),
				"clip_gain_bound_db":      24.0,
				"target_clipped_to_bound": candidate.TargetClipped,
				"same_metric_comparison":  true,
				"comparison_status":       projection.Status,
				"calibration_mode":        projection.Mode,
				"rlm_projection_id":       projection.ProjectionID,
				"candidate_count":         projection.Summary.CandidateCount,
				"eligible_track_count":    projection.Summary.EligibleTrackCount,
				"known_track_count":       projection.Summary.CandidateCount,
				"covered_track_count":     projection.Summary.CandidateCount,
				"calibration_ready_count": projection.Summary.CalibrationReadyCount,
				"metric_approximate":      projection.Mode == rlm.ModeCoarse,
				"metric_last_resort":      false,
				"strict_reference_model":  true,
			},
		}
		rows = append(rows, row)
	}
	sortByAbsDesc(rows)
	return rows
}

type sourceLevelCandidate struct {
	Track      TrackGainRow
	LevelKind  string
	LevelValue float64
	Unit       string
	ClipGainDB float64
}

type sourceLevelMetricDefinition struct {
	ID          string
	Unit        string
	Approximate bool
	LastResort  bool
}

type sourceLevelMetricProfile struct {
	Metric                 string
	Unit                   string
	Approximate            bool
	LastResort             bool
	TotalTrackCount        int
	NonMutedTrackCount     int
	EligibleTrackCount     int
	KnownTrackCount        int
	CalibrationReadyCount  int
	CandidateCount         int
	MissingCandidateCount  int
	MissingCandidateTracks []string
	NotCalibratableCount   int
	NotCalibratableTracks  []string
	Candidates             []sourceLevelCandidate
}

type sourceLevelAdmission struct {
	Status      string
	Reason      string
	Profiles    []sourceLevelMetricProfile
	Selected    sourceLevelMetricProfile
	HasSelected bool
}

func gainStagingSourceLevelOutlierRows(admission sourceLevelAdmission, limit int) []RankRow {
	if !admission.HasSelected {
		return nil
	}
	candidates := admission.Selected.Candidates
	if len(candidates) < 2 {
		return nil
	}
	values := make([]float64, 0, len(candidates))
	for _, candidate := range candidates {
		values = append(values, candidate.LevelValue)
	}
	reference := medianFloat64(values)
	rows := []RankRow{}
	for _, candidate := range candidates {
		delta := round3(reference - candidate.LevelValue)
		if math.Abs(delta) < 3.0 {
			continue
		}
		targetClipGain := round3(candidate.ClipGainDB + delta)
		appliedDelta := delta
		clamped := false
		if targetClipGain > 24 {
			targetClipGain = 24
			appliedDelta = round3(targetClipGain - candidate.ClipGainDB)
			clamped = true
		} else if targetClipGain < -24 {
			targetClipGain = -24
			appliedDelta = round3(targetClipGain - candidate.ClipGainDB)
			clamped = true
		}
		risk := "medium"
		if math.Abs(delta) >= 9 {
			risk = "high"
		}
		track := candidate.Track
		row := rankFromTrack(track, candidate.LevelKind+"_delta_to_reference", delta, "dB", risk, "source level differs from project technical reference")
		row.ClipID = track.PrimaryClip.ClipID
		row.ClipName = track.PrimaryClip.ClipName
		row.Metadata = map[string]any{
			"reference_metric":        candidate.LevelKind,
			"reference_level":         round3(reference),
			"reference_unit":          candidate.Unit,
			"observed_level":          round3(candidate.LevelValue),
			"current_clip_gain_db":    round3(candidate.ClipGainDB),
			"target_clip_gain_db":     targetClipGain,
			"requested_delta_db":      delta,
			"applied_delta_db":        appliedDelta,
			"clip_gain_bound_db":      24.0,
			"target_clipped_to_bound": clamped,
			"same_metric_comparison":  true,
			"comparison_status":       admission.Status,
			"candidate_count":         admission.Selected.CandidateCount,
			"eligible_track_count":    admission.Selected.EligibleTrackCount,
			"known_track_count":       admission.Selected.KnownTrackCount,
			"covered_track_count":     admission.Selected.CandidateCount,
			"calibration_ready_count": admission.Selected.CalibrationReadyCount,
			"metric_approximate":      admission.Selected.Approximate,
			"metric_last_resort":      admission.Selected.LastResort,
		}
		rows = append(rows, row)
	}
	sortByAbsDesc(rows)
	return capRows(rows, limit)
}

func gainStagingSourceLevelAdmission(tracks []TrackGainRow) sourceLevelAdmission {
	profiles := sourceLevelMetricProfiles(tracks)
	admission := sourceLevelAdmission{
		Status:   "missing",
		Reason:   "source_level_tracks_missing",
		Profiles: profiles,
	}
	if len(tracks) == 0 {
		return admission
	}
	admission.Status = "missing"
	admission.Reason = "no_comparable_source_level_candidates"
	for _, profile := range profiles {
		if profile.CandidateCount >= 2 && profile.MissingCandidateCount == 0 && profile.CandidateCount == profile.EligibleTrackCount {
			admission.Status = "ready"
			admission.Reason = "same_metric_full_eligible_coverage"
			admission.Selected = profile
			admission.HasSelected = true
			return admission
		}
	}
	best := bestSourceLevelMetricProfile(profiles)
	if best.CandidateCount > 0 {
		admission.Status = "partial"
		if best.CandidateCount < 2 {
			admission.Reason = "same_metric_candidate_count_below_2"
		} else if best.MissingCandidateCount > 0 {
			admission.Reason = "same_metric_candidate_coverage_partial"
		} else if best.CandidateCount < best.EligibleTrackCount {
			admission.Reason = "same_metric_candidate_coverage_partial"
		} else if best.NotCalibratableCount > 0 {
			admission.Reason = "source_level_tracks_not_calibratable"
		} else {
			admission.Reason = "same_metric_candidate_not_ready"
		}
		admission.Selected = best
	}
	return admission
}

func sourceLevelMetricDefinitions() []sourceLevelMetricDefinition {
	return []sourceLevelMetricDefinition{
		{ID: "integrated_lufs", Unit: "LUFS"},
		{ID: "active_rms_dbfs", Unit: "dBFS"},
		{ID: "rms_dbfs", Unit: "dBFS"},
		{ID: "approximate_lufs", Unit: "LUFS", Approximate: true},
		{ID: "peak_dbfs", Unit: "dBFS", LastResort: true},
	}
}

func sourceLevelMetricProfiles(tracks []TrackGainRow) []sourceLevelMetricProfile {
	defs := sourceLevelMetricDefinitions()
	out := make([]sourceLevelMetricProfile, 0, len(defs))
	for _, def := range defs {
		profile := sourceLevelMetricProfile{
			Metric:          def.ID,
			Unit:            def.Unit,
			Approximate:     def.Approximate,
			LastResort:      def.LastResort,
			TotalTrackCount: len(tracks),
		}
		for _, track := range tracks {
			if track.Mute {
				continue
			}
			profile.NonMutedTrackCount++
			if !trackEligibleForSourceLevelObservation(track) {
				continue
			}
			profile.EligibleTrackCount++
			value, unit, hasMetric := trackSourceLevelValue(track, def.ID)
			if hasMetric {
				profile.KnownTrackCount++
				if unit != "" {
					profile.Unit = unit
				}
			}
			if !trackCanReceiveSourceLevelCalibration(track) {
				profile.NotCalibratableCount++
				profile.NotCalibratableTracks = append(profile.NotCalibratableTracks, sourceLevelTrackLabel(track))
				if !hasMetric {
					profile.MissingCandidateCount++
					profile.MissingCandidateTracks = append(profile.MissingCandidateTracks, sourceLevelTrackLabel(track))
				}
				continue
			}
			profile.CalibrationReadyCount++
			if !hasMetric {
				profile.MissingCandidateCount++
				profile.MissingCandidateTracks = append(profile.MissingCandidateTracks, sourceLevelTrackLabel(track))
				continue
			}
			profile.CandidateCount++
			profile.Candidates = append(profile.Candidates, sourceLevelCandidate{
				Track:      track,
				LevelKind:  def.ID,
				LevelValue: value,
				Unit:       profile.Unit,
				ClipGainDB: *track.PrimaryClip.GainDB,
			})
		}
		out = append(out, profile)
	}
	return out
}

func bestSourceLevelMetricProfile(profiles []sourceLevelMetricProfile) sourceLevelMetricProfile {
	best := sourceLevelMetricProfile{}
	for _, profile := range profiles {
		if profile.CandidateCount > best.CandidateCount {
			best = profile
			continue
		}
		if profile.CandidateCount == best.CandidateCount && profile.MissingCandidateCount < best.MissingCandidateCount {
			best = profile
		}
	}
	return best
}

func trackCanReceiveSourceLevelCalibration(track TrackGainRow) bool {
	return track.PrimaryClip != nil &&
		track.PrimaryClip.GainDB != nil &&
		strings.TrimSpace(track.PrimaryClip.ClipID) != ""
}

func trackEligibleForSourceLevelObservation(track TrackGainRow) bool {
	if track.Mute {
		return false
	}
	trackType := strings.ToLower(strings.TrimSpace(track.TrackType))
	if strings.Contains(trackType, "master") {
		return false
	}
	if track.AudioClipCount > 0 {
		return true
	}
	if track.PrimaryClip != nil && strings.TrimSpace(track.PrimaryClip.ClipID) != "" {
		return true
	}
	if track.ClipCount > 0 && strings.Contains(trackType, "audio") {
		return true
	}
	if trackType == "" && (track.PeakDBFS != nil || track.ActiveRMSDBFS != nil || track.RMSDBFS != nil || track.IntegratedLUFS != nil || track.ApproxLUFS != nil) {
		return true
	}
	return false
}

func sourceLevelTrackLabel(track TrackGainRow) string {
	if strings.TrimSpace(track.TrackID) != "" {
		return track.TrackID
	}
	if strings.TrimSpace(track.TrackName) != "" {
		return track.TrackName
	}
	if track.PrimaryClip != nil && strings.TrimSpace(track.PrimaryClip.ClipID) != "" {
		return track.PrimaryClip.ClipID
	}
	return "unknown_track"
}

func (a sourceLevelAdmission) Summary() map[string]any {
	out := map[string]any{
		"status":                                 a.Status,
		"reason":                                 a.Reason,
		"requires_same_metric":                   true,
		"minimum_candidate_count":                2,
		"full_eligible_metric_coverage_required": true,
		"metric_counts":                          map[string]any{},
	}
	if a.HasSelected || strings.TrimSpace(a.Selected.Metric) != "" {
		out["selected_metric"] = a.Selected.Metric
		out["selected_unit"] = a.Selected.Unit
		out["candidate_count"] = a.Selected.CandidateCount
		out["eligible_track_count"] = a.Selected.EligibleTrackCount
		out["known_track_count"] = a.Selected.KnownTrackCount
		out["covered_track_count"] = a.Selected.CandidateCount
		out["calibration_ready_count"] = a.Selected.CalibrationReadyCount
		out["missing_candidate_count"] = a.Selected.MissingCandidateCount
		out["not_calibratable_count"] = a.Selected.NotCalibratableCount
		out["metric_approximate"] = a.Selected.Approximate
		out["metric_last_resort"] = a.Selected.LastResort
		if len(a.Selected.MissingCandidateTracks) > 0 {
			out["missing_candidate_tracks"] = capRows(a.Selected.MissingCandidateTracks, 12)
		}
		if len(a.Selected.NotCalibratableTracks) > 0 {
			out["not_calibratable_tracks"] = capRows(a.Selected.NotCalibratableTracks, 12)
		}
	}
	counts := out["metric_counts"].(map[string]any)
	for _, profile := range a.Profiles {
		counts[profile.Metric] = map[string]any{
			"unit":                    profile.Unit,
			"approximate":             profile.Approximate,
			"last_resort":             profile.LastResort,
			"non_muted_track_count":   profile.NonMutedTrackCount,
			"eligible_track_count":    profile.EligibleTrackCount,
			"known_track_count":       profile.KnownTrackCount,
			"covered_track_count":     profile.CandidateCount,
			"calibration_ready_count": profile.CalibrationReadyCount,
			"candidate_count":         profile.CandidateCount,
			"missing_candidate_count": profile.MissingCandidateCount,
			"not_calibratable_count":  profile.NotCalibratableCount,
		}
	}
	return out
}

func (a sourceLevelAdmission) EvidenceStatus() EvidenceStatus {
	total := 0
	known := 0
	if a.HasSelected || strings.TrimSpace(a.Selected.Metric) != "" {
		total = a.Selected.EligibleTrackCount
		known = a.Selected.CandidateCount
	}
	status := a.Status
	if status == "" {
		status = "missing"
	}
	return EvidenceStatus{
		Status:     status,
		KnownCount: known,
		TotalCount: total,
		Reason:     a.Reason,
	}
}

func (a sourceLevelAdmission) Limitations() []string {
	limits := []string{}
	switch a.Status {
	case "ready":
		if a.Selected.Approximate {
			limits = append(limits, "b1_2_source_level_selected_metric_approximate")
		}
		if a.Selected.LastResort {
			limits = append(limits, "b1_2_source_level_selected_metric_peak_last_resort")
		}
	case "partial", "missing":
		limits = append(limits, "b1_2_source_level_admission_"+a.Status)
		if a.Reason != "" {
			limits = append(limits, "b1_2_source_level_"+a.Reason)
		}
		if a.Selected.MissingCandidateCount > 0 {
			limits = append(limits, "b1_2_source_level_same_metric_partial_coverage")
		}
		if sourceLevelHasMultipleIncompleteMetrics(a.Profiles) {
			limits = append(limits, "b1_2_source_level_metrics_not_mixable")
		}
	}
	return addUnique(nil, limits...)
}

func sourceLevelHasMultipleIncompleteMetrics(profiles []sourceLevelMetricProfile) bool {
	count := 0
	for _, profile := range profiles {
		if profile.CandidateCount > 0 {
			count++
		}
	}
	return count > 1
}

func trackSourceLevelValue(track TrackGainRow, metric string) (float64, string, bool) {
	switch metric {
	case "integrated_lufs":
		if track.IntegratedLUFS != nil {
			return *track.IntegratedLUFS, "LUFS", true
		}
	case "active_rms_dbfs":
		if track.ActiveRMSDBFS != nil {
			return *track.ActiveRMSDBFS, "dBFS", true
		}
	case "approximate_lufs":
		if track.ApproxLUFS != nil {
			return *track.ApproxLUFS, "LUFS", true
		}
	case "rms_dbfs":
		if track.RMSDBFS != nil {
			return *track.RMSDBFS, "dBFS", true
		}
	case "peak_dbfs":
		if track.PeakDBFS != nil {
			return *track.PeakDBFS, "dBFS", true
		}
	}
	return 0, "", false
}

func gainStagingStrictReferenceCalibrationRequest(userIntent string) bool {
	text := strings.ToLower(strings.TrimSpace(userIntent))
	if text == "" {
		return false
	}
	if strings.Contains(text, "b1.2") ||
		strings.Contains(text, "b 1.2") ||
		strings.Contains(text, "b1-2") ||
		strings.Contains(text, "b 1-2") ||
		strings.Contains(text, "b1_2") {
		return true
	}
	if gainStagingStrictReferenceCalibrationCleanRequest(text) {
		return true
	}
	if !strings.Contains(text, "b1.2") &&
		!strings.Contains(text, "b 1.2") &&
		!strings.Contains(text, "b1-2") &&
		!strings.Contains(text, "b 1-2") &&
		!strings.Contains(text, "b1_2") &&
		!strings.Contains(text, "reference level") &&
		!strings.Contains(text, "source level calibration") &&
		!strings.Contains(text, "reference-level") &&
		!strings.Contains(text, "参考电平") &&
		!strings.Contains(text, "统一电平") &&
		!strings.Contains(text, "相同范围") &&
		!strings.Contains(text, "同一参考") &&
		!strings.Contains(text, "贴近") {
		return false
	}
	return strings.Contains(text, "strict") ||
		strings.Contains(text, "reference level") ||
		strings.Contains(text, "reference-level") ||
		strings.Contains(text, "same reference") ||
		strings.Contains(text, "same range") ||
		strings.Contains(text, "full-project reference") ||
		strings.Contains(text, "统一") ||
		strings.Contains(text, "参考电平") ||
		strings.Contains(text, "相同范围") ||
		strings.Contains(text, "同一参考") ||
		strings.Contains(text, "贴近") ||
		strings.Contains(text, "严格")
}

func gainStagingStrictReferenceCalibrationCleanRequest(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	hasScope := strings.Contains(text, "b1.2") ||
		strings.Contains(text, "b 1.2") ||
		strings.Contains(text, "b1-2") ||
		strings.Contains(text, "b 1-2") ||
		strings.Contains(text, "b1_2") ||
		strings.Contains(text, "reference level") ||
		strings.Contains(text, "source level calibration") ||
		strings.Contains(text, "reference-level") ||
		strings.Contains(text, "\u53c2\u8003\u7535\u5e73") ||
		strings.Contains(text, "\u7edf\u4e00\u7535\u5e73") ||
		strings.Contains(text, "\u76f8\u540c\u8303\u56f4") ||
		strings.Contains(text, "\u540c\u4e00\u53c2\u8003") ||
		strings.Contains(text, "\u8d34\u8fd1")
	if !hasScope {
		return false
	}
	return strings.Contains(text, "strict") ||
		strings.Contains(text, "reference level") ||
		strings.Contains(text, "reference-level") ||
		strings.Contains(text, "same reference") ||
		strings.Contains(text, "same range") ||
		strings.Contains(text, "full-project reference") ||
		strings.Contains(text, "\u7edf\u4e00") ||
		strings.Contains(text, "\u53c2\u8003\u7535\u5e73") ||
		strings.Contains(text, "\u76f8\u540c\u8303\u56f4") ||
		strings.Contains(text, "\u540c\u4e00\u53c2\u8003") ||
		strings.Contains(text, "\u8d34\u8fd1") ||
		strings.Contains(text, "\u4e25\u683c")
}

func gainStagingRLMSummary(projection rlm.Projection) map[string]any {
	out := map[string]any{
		"schema_version":          projection.SchemaVersion,
		"projection_id":           projection.ProjectionID,
		"status":                  projection.Status,
		"mode":                    projection.Mode,
		"selected_metric":         projection.SelectedMetric,
		"selected_unit":           projection.SelectedUnit,
		"track_count":             projection.Summary.TrackCount,
		"eligible_track_count":    projection.Summary.EligibleTrackCount,
		"calibration_ready_count": projection.Summary.CalibrationReadyCount,
		"candidate_count":         projection.Summary.CandidateCount,
		"action_count":            projection.Summary.ActionCount,
		"missing_metric_count":    projection.Summary.MissingMetricCount,
		"not_calibratable_count":  projection.Summary.NotCalibratableCount,
		"active_rms_count":        projection.Summary.ActiveRMSCount,
		"rms_count":               projection.Summary.RMSCount,
		"lufs_count":              projection.Summary.LUFSCount,
		"reference_basis":         projection.Summary.ReferenceBasis,
		"min_action_delta_db":     projection.Summary.MinActionDeltaDB,
		"actionable":              projection.Actionable,
	}
	if projection.ReferenceLevel != nil {
		out["reference_level"] = *projection.ReferenceLevel
	}
	if len(projection.Limitations) > 0 {
		out["limitations"] = append([]string(nil), projection.Limitations...)
	}
	return out
}

func gainStagingRLMEvidenceStatus(projection rlm.Projection) EvidenceStatus {
	status := strings.TrimSpace(projection.Status)
	if status == "" {
		status = "missing"
	}
	reason := ""
	if len(projection.Limitations) > 0 {
		reason = projection.Limitations[0]
	}
	return EvidenceStatus{
		Status:     status,
		KnownCount: projection.Summary.CandidateCount,
		TotalCount: projection.Summary.EligibleTrackCount,
		Reason:     reason,
	}
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return round3(sorted[mid])
	}
	return round3((sorted[mid-1] + sorted[mid]) / 2)
}

func rankFromTrack(track TrackGainRow, kind string, value float64, unit, risk, reason string) RankRow {
	return RankRow{
		TrackID:      track.TrackID,
		TrackName:    track.TrackName,
		Kind:         kind,
		Value:        round3(value),
		Unit:         unit,
		Risk:         risk,
		Reason:       reason,
		EvidenceRefs: append([]string(nil), track.EvidenceRefs...),
	}
}

func sortTrackRowsForPack(rows []TrackGainRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		iScore := trackRiskSortScore(rows[i])
		jScore := trackRiskSortScore(rows[j])
		if iScore != jScore {
			return iScore > jScore
		}
		return rows[i].TrackName < rows[j].TrackName
	})
}

func trackRiskSortScore(row TrackGainRow) int {
	score := len(row.Risks)
	if row.HeadroomDB != nil && *row.HeadroomDB <= 0.1 {
		score += 5
	}
	if row.PeakDBFS != nil && *row.PeakDBFS >= -0.1 {
		score += 4
	}
	if row.ActiveRMSDBFS != nil && *row.ActiveRMSDBFS <= -42 {
		score += 2
	}
	if row.RMSDBFS != nil && *row.RMSDBFS <= -42 {
		score += 2
	}
	return score
}

func gainStagingScope(in GainStagingInput, projectState, mixObservation map[string]any) Scope {
	scope := Scope{Kind: "project", Source: "default"}
	if target := mapValue(mixObservation["target_ref"]); len(target) > 0 {
		kind := firstText(target, "kind", "type")
		id := firstText(target, "id", "track_id", "clip_id")
		label := firstText(target, "label", "name")
		if kind != "" {
			scope.Kind = kind
			scope.Source = "mix.observe.target_ref"
		}
		if id != "" {
			scope.IDs = []string{id}
		}
		if label != "" {
			scope.Labels = []string{label}
		}
	}
	if selection := currentSelection(in.RequestContext, in.ContextSnapshot, projectState); len(selection) > 0 {
		if id := firstText(selection, "selected_track_id", "track_id"); id != "" && len(scope.IDs) == 0 {
			scope.Kind = "selected_track"
			scope.IDs = []string{id}
			scope.Source = "current_selection"
		}
		if id := firstText(selection, "selected_clip_id", "clip_id"); id != "" && len(scope.IDs) == 0 {
			scope.Kind = "selected_clip"
			scope.IDs = []string{id}
			scope.Source = "current_selection"
		}
	}
	if strings.TrimSpace(scope.Kind) == "" {
		scope.Kind = "project"
	}
	return scope
}

func currentSelection(candidates ...map[string]any) map[string]any {
	for _, candidate := range candidates {
		if len(candidate) == 0 {
			continue
		}
		if sel := mapValue(candidate["current_selection"]); len(sel) > 0 {
			return sel
		}
		if sel := mapValue(candidate["ui_context"]); len(sel) > 0 {
			return sel
		}
		if firstText(candidate, "selected_track_id", "selected_clip_id") != "" {
			return candidate
		}
	}
	return nil
}

func gainStagingEvidenceRefs(projectState, mixObservation, mixProject map[string]any) []string {
	refs := []string{}
	if len(projectState) > 0 {
		refs = addUnique(refs, "project.state")
	}
	observationID := firstText(mixObservation, "observation_id")
	if observationID != "" {
		refs = addUnique(refs, "mix.observe:"+observationID)
	} else if len(mixObservation) > 0 {
		refs = addUnique(refs, "mix.observe")
	}
	if len(mixProject) > 0 {
		refs = addUnique(refs, "mix.read:project.tracks.summary", "mix.read:project.risks.headroom", "mix.read:project.rankings.peak")
	}
	return refs
}

func gainStagingEvidenceStatus(projectState, mixObservation map[string]any, tracks []TrackGainRow, sourceAdmission sourceLevelAdmission) map[string]EvidenceStatus {
	out := map[string]EvidenceStatus{}
	total := len(tracks)
	out["project_state"] = EvidenceStatus{Status: "missing", Reason: "project_state_tracks_missing"}
	if len(bestTrackRows(projectState)) > 0 {
		out["project_state"] = EvidenceStatus{Status: "ready", KnownCount: total, TotalCount: total}
	}
	out["mix_observation"] = EvidenceStatus{Status: "missing", Reason: "mix_observation_missing"}
	if len(mixObservation) > 0 {
		out["mix_observation"] = EvidenceStatus{Status: "ready", TotalCount: total}
	}
	acoustic := countTracksWithAcoustics(tracks)
	status := "missing"
	if acoustic == total && total > 0 {
		status = "ready"
	} else if acoustic > 0 {
		status = "partial"
	}
	out["track_acoustic"] = EvidenceStatus{Status: status, KnownCount: acoustic, TotalCount: total}
	loudness := countTracksWithLoudness(tracks)
	status = "missing"
	if loudness == total && total > 0 {
		status = "ready"
	} else if loudness > 0 {
		status = "partial"
	}
	out["track_loudness"] = EvidenceStatus{Status: status, KnownCount: loudness, TotalCount: total}
	out["b1_2_source_level_admission"] = sourceAdmission.EvidenceStatus()
	trackGain := countTracksWithTrackGain(tracks)
	status = "missing"
	if trackGain == total && total > 0 {
		status = "ready"
	} else if trackGain > 0 {
		status = "partial"
	}
	out["track_gain"] = EvidenceStatus{Status: status, KnownCount: trackGain, TotalCount: total}
	clipGain := countClipsWithGain(tracks)
	audioClips := countAudioClips(tracks)
	status = "missing"
	if audioClips > 0 && clipGain == audioClips {
		status = "ready"
	} else if clipGain > 0 {
		status = "partial"
	}
	out["clip_gain"] = EvidenceStatus{Status: status, KnownCount: clipGain, TotalCount: audioClips}
	roleCount := 0
	for _, track := range tracks {
		if track.RoleGuess != "" || track.TrackType != "" {
			roleCount++
		}
	}
	status = "missing"
	if roleCount == total && total > 0 {
		status = "ready"
	} else if roleCount > 0 {
		status = "partial"
	}
	out["tom_role_summary"] = EvidenceStatus{Status: status, KnownCount: roleCount, TotalCount: total}
	return out
}

func gainStagingLimitations(status map[string]EvidenceStatus, mixProject map[string]any, budget Budget, total, returned int, sourceAdmission sourceLevelAdmission) []string {
	limits := []string{}
	for key, row := range status {
		if row.Status == "missing" || row.Status == "partial" {
			limits = append(limits, key+"_"+row.Status)
		}
	}
	limits = append(limits, stringsFromAny(mixProject["limitations"], 10, budget.MaxStringRunes)...)
	if returned < total {
		limits = append(limits, "tracks_truncated_by_context_budget")
	}
	limits = append(limits, sourceAdmission.Limitations()...)
	return addUnique(nil, limits...)
}

func countTracksWithAcoustics(rows []TrackGainRow) int {
	count := 0
	for _, row := range rows {
		if row.PeakDBFS != nil || row.ActiveRMSDBFS != nil || row.RMSDBFS != nil || row.HeadroomDB != nil || row.IntegratedLUFS != nil || row.ApproxLUFS != nil {
			count++
		}
	}
	return count
}

func countTracksWithLoudness(rows []TrackGainRow) int {
	count := 0
	for _, row := range rows {
		if row.IntegratedLUFS != nil || row.ApproxLUFS != nil {
			count++
		}
	}
	return count
}

func countTracksWithTrackGain(rows []TrackGainRow) int {
	count := 0
	for _, row := range rows {
		if row.VolumeDB != nil {
			count++
		}
	}
	return count
}

func countAudioClips(rows []TrackGainRow) int {
	count := 0
	for _, row := range rows {
		count += row.AudioClipCount
	}
	return count
}

func countClipsWithGain(rows []TrackGainRow) int {
	count := 0
	for _, row := range rows {
		if row.PrimaryClip != nil && row.PrimaryClip.GainDB != nil {
			count++
		}
	}
	return count
}

func clipGainSummary(rows []TrackGainRow) map[string]any {
	out := map[string]any{
		"audio_clip_count": countAudioClips(rows),
		"known_gain_count": countClipsWithGain(rows),
	}
	maxAbs := 0.0
	for _, row := range rows {
		if row.PrimaryClip != nil && row.PrimaryClip.GainDB != nil {
			if abs := math.Abs(*row.PrimaryClip.GainDB); abs > maxAbs {
				maxAbs = abs
			}
		}
	}
	if maxAbs > 0 {
		out["max_abs_primary_clip_gain_db"] = round3(maxAbs)
	}
	return out
}
