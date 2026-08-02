package rlm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/levelsafety"
)

const (
	minActionDeltaDB = 0.1
	clipGainBoundDB  = 24.0
)

type metricDefinition struct {
	ID          string
	Unit        string
	Mode        string
	Approximate bool
}

type metricCandidate struct {
	Row   int
	Value float64
}

func Build(input Input) Projection {
	generatedAt := strings.TrimSpace(input.GeneratedAt)
	if generatedAt == "" {
		generatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	rows := buildReferenceRows(input)
	profiles := buildMetricProfiles(rows)
	selected, hasSelected := selectMetricProfile(profiles)
	status := StatusMissing
	mode := ""
	unit := ""
	var reference *float64
	limitations := []string{}
	actionable := false
	if hasSelected {
		status = StatusPartial
		mode = selected.Mode
		unit = selected.Unit
		if selected.CandidateCount >= 2 &&
			selected.MissingMetricCount == 0 &&
			selected.NotCalibratableCount == 0 &&
			selected.CandidateCount == selected.CalibrationReadyCount {
			status = StatusReady
			values := make([]float64, 0, selected.CandidateCount)
			for _, candidate := range candidatesForMetric(rows, selected.Metric) {
				values = append(values, candidate.Value)
			}
			ref := median(values)
			reference = &ref
			actionable = true
			if selected.Mode == ModeCoarse {
				limitations = append(limitations, "rlm_strict_active_or_lufs_missing_coarse_reference")
			}
		} else {
			limitations = append(limitations, "rlm_reference_level_partial_metric_coverage")
			if selected.MissingMetricCount > 0 {
				limitations = append(limitations, "rlm_missing_selected_metric_for_some_tracks")
			}
			if selected.NotCalibratableCount > 0 {
				limitations = append(limitations, "rlm_some_tracks_missing_writable_clip_gain")
			}
		}
	} else {
		limitations = append(limitations, "rlm_no_comparable_reference_level_metric")
	}

	calibration := []CalibrationRow{}
	if actionable && reference != nil {
		calibration = buildCalibrationRows(rows, selected, *reference)
		applySelectedReferenceToRows(rows, selected, *reference, calibration)
	}
	if actionable && len(calibration) == 0 {
		limitations = append(limitations, "rlm_reference_level_already_within_tolerance")
	}

	summary := buildSummary(rows, selected, hasSelected, status, mode, unit, len(calibration))
	proj := Projection{
		SchemaVersion:  SchemaVersion,
		RLMVersion:     Version,
		Status:         status,
		Mode:           mode,
		SelectedMetric: selected.Metric,
		SelectedUnit:   unit,
		ReferenceLevel: reference,
		Actionable:     actionable && len(calibration) > 0,
		Summary:        summary,
		MetricProfiles: profiles,
		Rows:           rows,
		Calibration:    calibration,
		EvidenceRefs:   evidenceRefsFromRows(rows),
		Limitations:    uniqueStrings(limitations...),
		GeneratedAt:    generatedAt,
	}
	proj.ProjectionID = stableProjectionID(proj)
	return proj
}

func ContextProjectionMap(proj Projection) map[string]any {
	compact := proj
	if len(compact.Rows) > 24 {
		compact.Rows = append([]ReferenceLevelRow(nil), compact.Rows[:24]...)
	}
	if len(compact.Calibration) > 24 {
		compact.Calibration = append([]CalibrationRow(nil), compact.Calibration[:24]...)
	}
	data, err := json.Marshal(compact)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func buildReferenceRows(input Input) []ReferenceLevelRow {
	merged := mergeSourceRows(
		sourceRows(projectTrackRows(input.ProjectState), "project.state:tracks"),
		sourceRows(mixTrackRows(input.MixObservation), "mix.observe:project_package.tracks"),
		sourceRows(audioAnalysisRows(input.AudioAnalysisStatus), "project.audio_analysis_status:track_waveform_envelopes"),
	)
	rows := make([]ReferenceLevelRow, 0, len(merged))
	for _, raw := range merged {
		row := referenceRowFromMap(raw.Data, raw.EvidenceRefs)
		if row.TrackID == "" && row.TrackName == "" {
			continue
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rowLabel(rows[i]) < rowLabel(rows[j])
	})
	return rows
}

type rawRow struct {
	Key          string
	Data         map[string]any
	EvidenceRefs []string
}

func sourceRows(rows []map[string]any, evidenceRef string) []rawRow {
	out := make([]rawRow, 0, len(rows))
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		data := cloneMap(row)
		key := firstText(data, "track_id", "source_track_id", "id", "track_name", "name")
		if key == "" {
			key = fmt.Sprintf("%s#%d", evidenceRef, i)
		}
		out = append(out, rawRow{Key: key, Data: data, EvidenceRefs: []string{evidenceRef}})
	}
	return out
}

func mergeSourceRows(groups ...[]rawRow) []rawRow {
	out := []rawRow{}
	byKey := map[string]int{}
	for _, rows := range groups {
		for _, row := range rows {
			key := strings.TrimSpace(row.Key)
			if key == "" {
				key = fmt.Sprintf("row_%d", len(out)+1)
			}
			if idx, ok := byKey[key]; ok {
				mergeMap(out[idx].Data, row.Data)
				out[idx].EvidenceRefs = uniqueStrings(append(out[idx].EvidenceRefs, row.EvidenceRefs...)...)
				continue
			}
			next := rawRow{
				Key:          key,
				Data:         cloneMap(row.Data),
				EvidenceRefs: uniqueStrings(row.EvidenceRefs...),
			}
			byKey[key] = len(out)
			out = append(out, next)
		}
	}
	return out
}

func mergeMap(dst map[string]any, src map[string]any) {
	if dst == nil || len(src) == 0 {
		return
	}
	if sourceClipID, targetClipID := sourceClipIdentity(src), sourceClipIdentity(dst); sourceClipID != "" && targetClipID != "" && sourceClipID != targetClipID {
		// Track IDs are not sufficient after A4 clip splitting. Keep writable
		// primary-clip identity from project.state. A DAD track aggregate may be
		// attached to another representative clip, though, so use its acoustic
		// facts only where an earlier mix-level aggregate did not already supply
		// that same field.
		for key, value := range src {
			if rlmClipIdentityField(key) {
				continue
			}
			if rlmAcousticField(key) {
				if existing, exists := dst[key]; exists && cleanText(existing) != "" {
					continue
				}
				if cleanText(value) != "" {
					dst[key] = value
				}
				continue
			}
			if cleanText(value) != "" {
				dst[key] = value
			}
		}
		return
	}
	for key, value := range src {
		if cleanText(value) == "" {
			continue
		}
		dst[key] = value
	}
}

func sourceClipIdentity(row map[string]any) string {
	if len(row) == 0 {
		return ""
	}
	if value := firstText(row, "clip_id", "source_clip_id", "primary_clip_id", "bake_key"); value != "" {
		return value
	}
	clip := mapValue(row["primary_clip"])
	if len(clip) > 0 {
		return firstText(clip, "clip_id", "id", "item_id")
	}
	if clip = primaryClip(row); len(clip) > 0 {
		return firstText(clip, "clip_id", "id", "item_id")
	}
	return ""
}

func rlmAcousticField(key string) bool {
	switch key {
	case "rms", "rms_db", "rms_dbfs", "active_rms_dbfs", "gated_rms_dbfs", "silence_gated_rms_dbfs", "peak", "peak_abs", "peak_db", "peak_dbfs", "headroom_db", "integrated_lufs", "approximate_lufs", "lufs", "lufs_estimate", "crest_db":
		return true
	default:
		return false
	}
}

func rlmClipIdentityField(key string) bool {
	switch key {
	case "clip_id", "source_clip_id", "primary_clip_id", "bake_key", "clip_name", "clip_gain_db", "gain_db", "start_seconds", "end_seconds", "duration_seconds", "length_seconds":
		return true
	default:
		return false
	}
}

func projectTrackRows(projectState map[string]any) []map[string]any {
	for _, candidate := range []map[string]any{
		projectState,
		mapValue(projectState["project_state"]),
		mapValue(projectState["daw_state_summary"]),
		mapValue(projectState["shadow"]),
	} {
		if rows := firstRows(candidate, "tracks", "visible_tracks", "track_summaries"); len(rows) > 0 {
			return rows
		}
	}
	return nil
}

func mixTrackRows(mixObservation map[string]any) []map[string]any {
	best := bestMixObservation(mixObservation)
	for _, path := range [][]string{
		{"project_package", "tracks"},
		{"observation", "project_package", "tracks"},
		{"context_pack", "latest_observation", "project_package", "tracks"},
	} {
		if rows := rowsValue(valueAtPath(best, path)); len(rows) > 0 {
			return rows
		}
	}
	return nil
}

func audioAnalysisRows(audioStatus map[string]any) []map[string]any {
	for _, path := range [][]string{
		{"track_waveform_envelopes"},
		{"analysis_job", "track_waveform_envelopes"},
		{"result", "track_waveform_envelopes"},
	} {
		if rows := rowsValue(valueAtPath(audioStatus, path)); len(rows) > 0 {
			return rows
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

func referenceRowFromMap(data map[string]any, refs []string) ReferenceLevelRow {
	row := ReferenceLevelRow{
		TrackID:      firstText(data, "track_id", "source_track_id", "id"),
		TrackName:    firstText(data, "track_name", "name", "user_label"),
		TrackType:    firstText(data, "track_type", "type", "kind"),
		Muted:        boolValue(firstPresent(data, "mute", "muted", "is_muted")),
		EvidenceRefs: uniqueStrings(refs...),
	}
	if value, ok := firstNumber(data, "volume_db", "fader_db", "track_gain_db"); ok {
		row.TrackFaderDB = ptrRound(value)
	}
	if value, ok := firstNumber(data, "pan", "pan_value", "balance"); ok {
		_ = value
	}
	applyAcousticFields(&row, data)
	if acoustic := mapValue(data["acoustic"]); len(acoustic) > 0 {
		applyAcousticFields(&row, acoustic)
	}
	clip := primaryClip(data)
	if len(clip) > 0 {
		row.PrimaryClipID = firstText(clip, "clip_id", "id", "item_id")
		row.PrimaryClipName = firstText(clip, "clip_name", "name", "file_name")
		if value, ok := firstNumber(clip, "clip_gain_db", "gain_db", "db"); ok {
			row.ClipGainDB = ptrRound(value)
		}
	}
	if row.PrimaryClipID == "" {
		row.PrimaryClipID = firstText(data, "clip_id", "primary_clip_id", "item_id")
		row.PrimaryClipName = firstText(data, "clip_name", "primary_clip_name", "file_name")
		if value, ok := firstNumber(data, "clip_gain_db"); ok {
			row.ClipGainDB = ptrRound(value)
		}
	}
	row.Eligible, row.EligibilityReason = rowEligible(row)
	row.CalibrationReady, row.CalibrationBlocker = rowCalibrationReady(row)
	return row
}

func applyAcousticFields(row *ReferenceLevelRow, data map[string]any) {
	if row == nil || len(data) == 0 {
		return
	}
	if peakAbs, peakOK := firstNumber(data, "peak_abs"); peakOK {
		if rmsAbs, rmsOK := firstNumber(data, "rms"); rmsOK && peakAbs <= 0 && rmsAbs <= 0 {
			row.SilentSource = true
		}
	}
	if value, ok := firstNumber(data, "peak_dbfs", "peak_db"); ok {
		row.PeakDBFS = ptrRound(value)
	} else if value, ok := firstNumber(data, "peak_abs", "peak"); ok {
		if db, dbOK := dbfsFromAbs(value); dbOK {
			row.PeakDBFS = &db
		}
	}
	if value, ok := firstNumber(data, "active_rms_dbfs", "gated_rms_dbfs", "silence_gated_rms_dbfs"); ok {
		row.ActiveRMSDBFS = ptrRound(value)
	}
	if value, ok := firstNumber(data, "rms_dbfs", "level_db", "rms_db"); ok {
		row.RMSDBFS = ptrRound(value)
	} else if value, ok := firstNumber(data, "rms"); ok {
		if db, dbOK := dbfsFromAbs(value); dbOK {
			row.RMSDBFS = &db
		}
	}
	if value, ok := firstNumber(data, "integrated_lufs"); ok {
		row.IntegratedLUFS = ptrRound(value)
	}
	if value, ok := firstNumber(data, "approximate_lufs", "lufs", "lufs_estimate"); ok {
		row.ApproximateLUFS = ptrRound(value)
	}
	if value, ok := firstNumber(data, "headroom_db"); ok {
		row.HeadroomDB = ptrRound(value)
	} else if row.PeakDBFS != nil {
		headroom := round3(-*row.PeakDBFS)
		row.HeadroomDB = &headroom
	}
	if value, ok := firstNumber(data, "crest_db"); ok {
		row.CrestDB = ptrRound(value)
	}
}

func primaryClip(data map[string]any) map[string]any {
	best := map[string]any(nil)
	bestDuration := -1.0
	for _, key := range []string{"clips", "clip_summaries", "audio_clips"} {
		for _, clip := range rowsValue(data[key]) {
			if !clipLooksAudio(clip) {
				continue
			}
			duration := 0.0
			if value, ok := firstNumber(clip, "duration_seconds", "length_seconds", "duration", "length"); ok {
				duration = value
			}
			if best == nil || duration > bestDuration {
				best = clip
				bestDuration = duration
			}
		}
	}
	if best == nil {
		best = mapValue(data["primary_clip"])
	}
	return best
}

func clipLooksAudio(row map[string]any) bool {
	text := strings.ToLower(firstText(row, "type", "clip_type", "media_kind", "kind"))
	return text == "" || strings.Contains(text, "audio") || strings.Contains(text, "wave") || strings.Contains(text, "clip")
}

func rowEligible(row ReferenceLevelRow) (bool, string) {
	if row.Muted {
		return false, "muted"
	}
	if row.SilentSource {
		return false, "silent_source_no_level_evidence"
	}
	trackType := strings.ToLower(strings.TrimSpace(row.TrackType))
	if strings.Contains(trackType, "master") {
		return false, "master_track"
	}
	if strings.Contains(trackType, "folder") && row.PrimaryClipID == "" && !rowHasLevel(row) {
		return false, "folder_without_source_level"
	}
	if row.PrimaryClipID != "" || rowHasLevel(row) {
		return true, "audio_source_or_level_evidence"
	}
	if strings.Contains(trackType, "audio") || strings.Contains(trackType, "track") || trackType == "" {
		return true, "audio_track_without_level_evidence"
	}
	return false, "non_audio_track"
}

func rowCalibrationReady(row ReferenceLevelRow) (bool, string) {
	if !row.Eligible {
		return false, row.EligibilityReason
	}
	if strings.TrimSpace(row.PrimaryClipID) == "" {
		return false, "primary_clip_id_missing"
	}
	if row.ClipGainDB == nil {
		return false, "clip_gain_missing"
	}
	return true, ""
}

func rowHasLevel(row ReferenceLevelRow) bool {
	return row.IntegratedLUFS != nil || row.ActiveRMSDBFS != nil || row.RMSDBFS != nil || row.ApproximateLUFS != nil || row.PeakDBFS != nil
}

func metricDefinitions() []metricDefinition {
	return []metricDefinition{
		{ID: "integrated_lufs", Unit: "LUFS", Mode: ModeStrict},
		{ID: "active_rms_dbfs", Unit: "dBFS", Mode: ModeStrict},
		{ID: "rms_dbfs", Unit: "dBFS", Mode: ModeCoarse},
		{ID: "approximate_lufs", Unit: "LUFS", Mode: ModeCoarse, Approximate: true},
	}
}

func buildMetricProfiles(rows []ReferenceLevelRow) []MetricProfile {
	defs := metricDefinitions()
	out := make([]MetricProfile, 0, len(defs))
	for _, def := range defs {
		profile := MetricProfile{
			Metric:      def.ID,
			Unit:        def.Unit,
			Mode:        def.Mode,
			Approximate: def.Approximate,
		}
		for _, row := range rows {
			if !row.Eligible {
				continue
			}
			profile.EligibleTrackCount++
			value, ok := rowMetric(row, def.ID)
			if ok {
				profile.KnownTrackCount++
			}
			if !row.CalibrationReady {
				profile.NotCalibratableCount++
				profile.NotCalibratableTracks = append(profile.NotCalibratableTracks, rowLabel(row))
				if !ok {
					profile.MissingMetricCount++
					profile.MissingTracks = append(profile.MissingTracks, rowLabel(row))
				}
				continue
			}
			profile.CalibrationReadyCount++
			if !ok {
				profile.MissingMetricCount++
				profile.MissingTracks = append(profile.MissingTracks, rowLabel(row))
				continue
			}
			_ = value
			profile.CandidateCount++
		}
		profile.MissingTracks = capStrings(profile.MissingTracks, 12)
		profile.NotCalibratableTracks = capStrings(profile.NotCalibratableTracks, 12)
		out = append(out, profile)
	}
	return out
}

func selectMetricProfile(profiles []MetricProfile) (MetricProfile, bool) {
	for _, profile := range profiles {
		if profile.CandidateCount >= 2 &&
			profile.MissingMetricCount == 0 &&
			profile.NotCalibratableCount == 0 &&
			profile.CandidateCount == profile.CalibrationReadyCount {
			return profile, true
		}
	}
	best := MetricProfile{}
	has := false
	for _, profile := range profiles {
		if profile.CandidateCount == 0 {
			continue
		}
		if !has || profile.CandidateCount > best.CandidateCount ||
			(profile.CandidateCount == best.CandidateCount && profile.MissingMetricCount < best.MissingMetricCount) {
			best = profile
			has = true
		}
	}
	return best, has
}

func candidatesForMetric(rows []ReferenceLevelRow, metric string) []metricCandidate {
	out := []metricCandidate{}
	for i, row := range rows {
		if !row.Eligible || !row.CalibrationReady {
			continue
		}
		value, ok := rowMetric(row, metric)
		if !ok {
			continue
		}
		out = append(out, metricCandidate{Row: i, Value: value})
	}
	return out
}

func rowMetric(row ReferenceLevelRow, metric string) (float64, bool) {
	switch metric {
	case "integrated_lufs":
		if row.IntegratedLUFS != nil {
			return *row.IntegratedLUFS, true
		}
	case "active_rms_dbfs":
		if row.ActiveRMSDBFS != nil {
			return *row.ActiveRMSDBFS, true
		}
	case "rms_dbfs":
		if row.RMSDBFS != nil {
			return *row.RMSDBFS, true
		}
	case "approximate_lufs":
		if row.ApproximateLUFS != nil {
			return *row.ApproximateLUFS, true
		}
	}
	return 0, false
}

func buildCalibrationRows(rows []ReferenceLevelRow, profile MetricProfile, reference float64) []CalibrationRow {
	out := []CalibrationRow{}
	for _, candidate := range candidatesForMetric(rows, profile.Metric) {
		row := rows[candidate.Row]
		if row.ClipGainDB == nil {
			continue
		}
		requestedDelta := round3(reference - candidate.Value)
		if math.Abs(requestedDelta) < minActionDeltaDB {
			continue
		}
		constraint := levelsafety.ConstrainSourceClipGain(*row.ClipGainDB, requestedDelta, clipGainBoundDB, row.PeakDBFS)
		target := round3(constraint.TargetClipGainDB)
		appliedDelta := round3(constraint.AppliedDeltaDB)
		out = append(out, CalibrationRow{
			TrackID:            row.TrackID,
			TrackName:          row.TrackName,
			ClipID:             row.PrimaryClipID,
			ClipName:           row.PrimaryClipName,
			Metric:             profile.Metric,
			Unit:               profile.Unit,
			Mode:               profile.Mode,
			ObservedLevel:      round3(candidate.Value),
			ReferenceLevel:     round3(reference),
			CurrentClipGainDB:  round3(*row.ClipGainDB),
			TargetClipGainDB:   target,
			RequestedDeltaDB:   requestedDelta,
			AppliedDeltaDB:     appliedDelta,
			TargetClipped:      constraint.ClipGainBoundClamped,
			PeakSafetyClipped:  constraint.PeakSafetyClamped,
			SourcePeakDBFS:     ptrRoundOptional(row.PeakDBFS),
			ProjectedPeakDBFS:  ptrRoundOptional(constraint.ProjectedPeakDBFS),
			PeakSafetyAchieved: cloneBoolPointer(constraint.PeakSafetyAchieved),
			Risk:               calibrationRisk(requestedDelta),
			EvidenceRefs:       append([]string(nil), row.EvidenceRefs...),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return math.Abs(out[i].RequestedDeltaDB) > math.Abs(out[j].RequestedDeltaDB)
	})
	return out
}

func ptrRoundOptional(value *float64) *float64 {
	if value == nil {
		return nil
	}
	rounded := round3(*value)
	return &rounded
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func applySelectedReferenceToRows(rows []ReferenceLevelRow, profile MetricProfile, reference float64, calibration []CalibrationRow) {
	byClip := map[string]CalibrationRow{}
	for _, row := range calibration {
		byClip[row.ClipID] = row
	}
	for i := range rows {
		if !rows[i].Eligible {
			continue
		}
		value, ok := rowMetric(rows[i], profile.Metric)
		if !ok {
			continue
		}
		delta := round3(reference - value)
		ref := round3(reference)
		observed := round3(value)
		rows[i].SelectedMetric = profile.Metric
		rows[i].ObservedLevel = &observed
		rows[i].ReferenceLevel = &ref
		rows[i].DeltaDB = &delta
		if action, ok := byClip[rows[i].PrimaryClipID]; ok {
			target := action.TargetClipGainDB
			rows[i].TargetClipGainDB = &target
			rows[i].TargetClipped = action.TargetClipped
			rows[i].RequiresAction = true
		}
	}
}

func buildSummary(rows []ReferenceLevelRow, selected MetricProfile, hasSelected bool, status, mode, unit string, actionCount int) Summary {
	summary := Summary{
		TrackCount:       len(rows),
		ReferenceBasis:   "median",
		MinActionDeltaDB: minActionDeltaDB,
	}
	for _, row := range rows {
		if row.Eligible {
			summary.EligibleTrackCount++
		}
		if row.CalibrationReady {
			summary.CalibrationReadyCount++
		}
		if row.ActiveRMSDBFS != nil {
			summary.ActiveRMSCount++
		}
		if row.RMSDBFS != nil {
			summary.RMSCount++
		}
		if row.IntegratedLUFS != nil || row.ApproximateLUFS != nil {
			summary.LUFSCount++
		}
	}
	if hasSelected {
		summary.CandidateCount = selected.CandidateCount
		summary.MissingMetricCount = selected.MissingMetricCount
		summary.NotCalibratableCount = selected.NotCalibratableCount
		summary.SelectedMetric = selected.Metric
		summary.SelectedUnit = unit
		summary.Mode = mode
	}
	if status == StatusMissing {
		summary.Mode = ""
	}
	summary.ActionCount = actionCount
	return summary
}

func calibrationRisk(delta float64) string {
	switch {
	case math.Abs(delta) >= 9:
		return "high"
	case math.Abs(delta) >= 3:
		return "medium"
	default:
		return "low"
	}
}

func evidenceRefsFromRows(rows []ReferenceLevelRow) []string {
	refs := []string{}
	for _, row := range rows {
		refs = append(refs, row.EvidenceRefs...)
	}
	return uniqueStrings(refs...)
}

func stableProjectionID(proj Projection) string {
	seed := strings.Join([]string{
		proj.SchemaVersion,
		proj.SelectedMetric,
		proj.Mode,
		fmt.Sprintf("%d", proj.Summary.TrackCount),
		fmt.Sprintf("%d", proj.Summary.ActionCount),
		strings.Join(proj.EvidenceRefs, "|"),
		proj.GeneratedAt,
	}, "\x00")
	sum := sha256.Sum256([]byte(seed))
	return "rlm_" + hex.EncodeToString(sum[:])[:16]
}

func valueAtPath(root map[string]any, path []string) any {
	var current any = root
	for _, key := range path {
		mapped := mapValue(current)
		if len(mapped) == 0 {
			return nil
		}
		current = mapped[key]
	}
	return current
}

func firstRows(row map[string]any, keys ...string) []map[string]any {
	for _, key := range keys {
		if rows := rowsValue(row[key]); len(rows) > 0 {
			return rows
		}
	}
	return nil
}

func mapValue(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case map[string]string:
		out := make(map[string]any, len(v))
		for key, value := range v {
			out[key] = value
		}
		return out
	default:
		data, err := json.Marshal(v)
		if err != nil || len(data) == 0 || string(data) == "null" {
			return nil
		}
		out := map[string]any{}
		if err := json.Unmarshal(data, &out); err != nil || len(out) == 0 {
			return nil
		}
		return out
	}
}

func rowsValue(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if mapped := mapValue(row); len(mapped) > 0 {
				out = append(out, mapped)
			}
		}
		return out
	default:
		return nil
	}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	data, _ := json.Marshal(in)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok && cleanText(value) != "" {
			return value
		}
	}
	return nil
}

func firstText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := cleanText(row[key]); text != "" {
			return text
		}
	}
	return ""
}

func firstNumber(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := numberValue(row[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func numberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		text := strings.TrimSpace(strings.TrimSuffix(v, "dB"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "db"))
		if text == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(text, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "yes", "1", "on":
			return true
		default:
			return false
		}
	case int:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func cleanText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func ptrRound(value float64) *float64 {
	next := round3(value)
	return &next
}

func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func dbfsFromAbs(value float64) (float64, bool) {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return round3(20 * math.Log10(value)), true
}

func median(values []float64) float64 {
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

func rowLabel(row ReferenceLevelRow) string {
	if strings.TrimSpace(row.TrackID) != "" {
		return row.TrackID
	}
	if strings.TrimSpace(row.TrackName) != "" {
		return row.TrackName
	}
	if strings.TrimSpace(row.PrimaryClipID) != "" {
		return row.PrimaryClipID
	}
	return "unknown_track"
}

func capStrings(values []string, limit int) []string {
	values = uniqueStrings(values...)
	if limit > 0 && len(values) > limit {
		return append([]string(nil), values[:limit]...)
	}
	return values
}

func uniqueStrings(values ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
