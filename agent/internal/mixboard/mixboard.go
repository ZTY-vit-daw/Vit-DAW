package mixboard

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	ObservationSchemaVersion = "mix_observation.v1"
	BoardSchemaVersion       = "mixboard.v1"
	ContextPackSchemaVersion = "mixboard_context_pack.v1"
)

type TargetRef struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Label      string `json:"label,omitempty"`
	Source     string `json:"source,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

type MixObject struct {
	Mode        string `json:"mode,omitempty"`
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	Label       string `json:"label,omitempty"`
	EffectScope string `json:"effect_scope,omitempty"`
	Source      string `json:"source,omitempty"`
}

type ListenTimeScope struct {
	Mode         string  `json:"mode"`
	StartSeconds float64 `json:"start_seconds,omitempty"`
	EndSeconds   float64 `json:"end_seconds,omitempty"`
	Locked       bool    `json:"locked,omitempty"`
	Source       string  `json:"source,omitempty"`
}

type ListenSourceScope struct {
	Mode       string   `json:"mode"`
	FocusIDs   []string `json:"focus_ids,omitempty"`
	ContextIDs []string `json:"context_ids,omitempty"`
}

type ListenScope struct {
	Time   ListenTimeScope   `json:"time"`
	Source ListenSourceScope `json:"source"`
}

type TimeRuler struct {
	DurationSeconds float64  `json:"duration_seconds"`
	SegmentSeconds  float64  `json:"segment_seconds"`
	FrameSeconds    float64  `json:"frame_seconds"`
	TempoBPM        *float64 `json:"tempo_bpm"`
	BarMapAvailable bool     `json:"bar_map_available"`
}

type ObservationPacket struct {
	SchemaVersion      string            `json:"schema_version"`
	ObservationID      string            `json:"observation_id"`
	MixSessionID       string            `json:"mix_session_id"`
	Round              int               `json:"round"`
	Status             string            `json:"status"`
	TargetRef          TargetRef         `json:"target_ref"`
	MixObjects         []MixObject       `json:"mix_objects"`
	ListenScope        ListenScope       `json:"listen_scope"`
	TimeRuler          TimeRuler         `json:"time_ruler"`
	GlobalSummary      map[string]any    `json:"global_summary"`
	EnvironmentPackage map[string]any    `json:"environment_package"`
	MixPackage         map[string]any    `json:"mix_package"`
	DeepPackage        map[string]any    `json:"deep_package"`
	SectionCandidates  []map[string]any  `json:"section_candidates"`
	TimelineDigest     []map[string]any  `json:"timeline_digest"`
	Hotspots           []map[string]any  `json:"hotspots"`
	SourceCapabilities map[string]string `json:"source_capabilities"`
	Notes              []string          `json:"notes,omitempty"`
	CreatedAt          string            `json:"created_at"`
}

type Board struct {
	SchemaVersion         string            `json:"schema_version"`
	MixSessionID          string            `json:"mix_session_id"`
	Status                string            `json:"status"`
	GoalText              string            `json:"goal_text,omitempty"`
	TargetRef             TargetRef         `json:"target_ref"`
	MixObjects            []MixObject       `json:"mix_objects"`
	ListenScope           ListenScope       `json:"listen_scope"`
	ObservationCount      int               `json:"observation_count"`
	LatestObservationID   string            `json:"latest_observation_id,omitempty"`
	LatestObservationPath string            `json:"latest_observation_path,omitempty"`
	ActiveProblemMap      []map[string]any  `json:"active_problem_map"`
	SectionCandidates     []map[string]any  `json:"section_candidates"`
	PackageStatus         map[string]string `json:"package_status"`
	OpenBlockers          []string          `json:"open_blockers,omitempty"`
	UpdatedAt             string            `json:"updated_at"`
}

type ContextPack struct {
	SchemaVersion     string           `json:"schema_version"`
	MixSessionID      string           `json:"mix_session_id"`
	GeneratedAt       string           `json:"generated_at"`
	SessionHeader     map[string]any   `json:"session_header"`
	LatestObservation map[string]any   `json:"latest_observation"`
	ActiveProblemMap  []map[string]any `json:"active_problem_map"`
	RelevantSections  []map[string]any `json:"relevant_sections"`
	OpenBlockers      []string         `json:"open_blockers,omitempty"`
}

type Request struct {
	MixSessionID string
	Round        int
	GoalText     string
	TargetRef    TargetRef
	MixObjects   []MixObject
	ListenScope  ListenScope
	ProjectState map[string]any
	Args         map[string]any
}

type WriteResult struct {
	Status          string            `json:"status"`
	BoardPath       string            `json:"board_path"`
	ObservationPath string            `json:"observation_path"`
	ContextPackPath string            `json:"context_pack_path"`
	Board           Board             `json:"board"`
	Observation     ObservationPacket `json:"observation"`
	ContextPack     ContextPack       `json:"context_pack"`
}

type Store struct {
	Root string
	Now  func() time.Time
}

type featureSnapshot struct {
	SchemaVersion         string         `json:"schema_version"`
	UpdatedAt             string         `json:"updated_at"`
	LatestRequest         map[string]any `json:"latest_request"`
	WaveformEnvelope      map[string]any `json:"waveform_envelope"`
	SpectrogramTiles      map[string]any `json:"spectrogram_tiles"`
	BandEnergySummary     map[string]any `json:"band_energy_summary"`
	StereoRelationSummary map[string]any `json:"stereo_relation_summary"`
}

func DefaultRoot() string {
	if override := strings.TrimSpace(os.Getenv("VIT_MIXBOARD_ROOT")); override != "" {
		return filepath.Clean(override)
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join("VitApp", "Workspace", "Artifacts", "mixboard")
	}
	for dir := wd; dir != ""; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "agent" {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return filepath.Join(filepath.Dir(dir), "VitApp", "Workspace", "Artifacts", "mixboard")
			}
		}
		if _, err := os.Stat(filepath.Join(dir, "VitApp", "Workspace")); err == nil {
			return filepath.Join(dir, "VitApp", "Workspace", "Artifacts", "mixboard")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return filepath.Join(wd, "VitApp", "Workspace", "Artifacts", "mixboard")
}

func NewStore(root string) Store {
	if strings.TrimSpace(root) == "" {
		root = DefaultRoot()
	}
	return Store{Root: filepath.Clean(root), Now: time.Now}
}

func (s Store) RequestObservation(req Request) (WriteResult, error) {
	now := s.now().UTC().Format(time.RFC3339Nano)
	req.MixSessionID = strings.TrimSpace(req.MixSessionID)
	if req.MixSessionID == "" {
		req.MixSessionID = "mix_" + randomID()
	}
	if req.Round <= 0 {
		req.Round = 1
	}
	req.TargetRef = normalizeTarget(req.TargetRef)
	req.MixObjects = normalizeMixObjects(req.MixObjects, req.TargetRef, req.Args)
	req.ListenScope = normalizeListenScope(req.ListenScope, req.MixObjects, req.Args)

	sessionDir := filepath.Join(s.Root, safePathName(req.MixSessionID))
	previousObservation, hasPreviousObservation := readLatestObservation(sessionDir)
	observation := BuildObservation(req, now)
	applyBeforeAfterDelta(&observation, previousObservation, hasPreviousObservation, now)
	obsDir := filepath.Join(sessionDir, "observations")
	actionsDir := filepath.Join(sessionDir, "actions")
	if err := os.MkdirAll(obsDir, 0o755); err != nil {
		return WriteResult{}, err
	}
	if err := os.MkdirAll(actionsDir, 0o755); err != nil {
		return WriteResult{}, err
	}

	observationPath := filepath.Join(obsDir, observation.ObservationID+".json")
	if err := writeJSON(observationPath, observation); err != nil {
		return WriteResult{}, err
	}
	board := buildBoard(req, observation, observationPath, now)
	boardPath := filepath.Join(sessionDir, "current.json")
	if err := writeJSON(boardPath, board); err != nil {
		return WriteResult{}, err
	}
	contextPack := buildContextPack(req, board, observation, now)
	contextPackPath := filepath.Join(sessionDir, "context_pack.json")
	if err := writeJSON(contextPackPath, contextPack); err != nil {
		return WriteResult{}, err
	}

	return WriteResult{
		Status:          observation.Status,
		BoardPath:       boardPath,
		ObservationPath: observationPath,
		ContextPackPath: contextPackPath,
		Board:           board,
		Observation:     observation,
		ContextPack:     contextPack,
	}, nil
}

func BuildObservation(req Request, createdAt string) ObservationPacket {
	duration := firstPositiveFloat(req.Args, "duration_seconds", "duration")
	if duration <= 0 {
		duration = projectDuration(req.ProjectState)
	}
	segment := firstPositiveFloat(req.Args, "segment_seconds")
	if segment <= 0 {
		segment = 2.0
	}
	frame := firstPositiveFloat(req.Args, "frame_seconds")
	if frame <= 0 {
		frame = 0.01
	}
	var tempo *float64
	if bpm := firstPositiveFloat(req.Args, "tempo_bpm", "bpm"); bpm > 0 {
		tempo = &bpm
	} else if bpm := firstPositiveFloat(req.ProjectState, "tempo_bpm", "bpm"); bpm > 0 {
		tempo = &bpm
	}
	featureSnapshot := loadFeatureSnapshot(req.Args)
	waveformStatus := featureStatus(featureSnapshot.WaveformEnvelope)
	spectrogramStatus := featureStatus(featureSnapshot.SpectrogramTiles)
	bandEnergyStatus := featureStatus(featureSnapshot.BandEnergySummary)
	stereoRelationStatus := featureStatus(featureSnapshot.StereoRelationSummary)

	caps := map[string]string{
		"waveform_envelope": waveformStatus,
		"spectrogram_tiles": spectrogramStatus,
		"band_energy":       bandEnergyStatus,
		"stereo_relation":   stereoRelationStatus,
		"post_fx_probe":     "unavailable",
	}
	notes := []string{
		"MixBoard observation initialized with project/shadow facts.",
	}
	if waveformStatus == "ready" {
		notes = append(notes, "MixBoard consumed the latest lightweight acoustic feature snapshot.")
	} else {
		notes = append(notes, "Agent-side lightweight acoustic feature reader is not connected yet; acoustic fields are partial.")
	}
	status := observationStatusFromFeatures(duration, waveformStatus, spectrogramStatus)
	if duration <= 0 {
		notes = append(notes, "duration_seconds is unavailable; timeline uses a zero-length ruler until audio feature data is connected.")
	}
	waveformMetrics := buildWaveformMetrics(featureSnapshot.WaveformEnvelope)
	timeSegments := waveformTimeSegments(featureSnapshot.WaveformEnvelope)
	timeline := buildTimelineDigest(duration, segment, timeSegments)
	sections := buildSectionCandidates(duration)
	hotspots := buildFeatureHotspots(featureSnapshot, waveformStatus, spectrogramStatus)
	problemTags := []string{}
	if status == "unavailable" {
		problemTags = append(problemTags, "feature_source_unavailable")
	}
	if waveformStatus == "ready" {
		problemTags = append(problemTags, "waveform_observed")
	}
	if spectrogramStatus == "ready" || spectrogramStatus == "partial" {
		problemTags = append(problemTags, "spectrum_observed")
	}
	environmentPackage := buildEnvironmentPackage(req, duration, segment, frame, tempo, caps, featureSnapshot)
	bandEnergy := buildBandEnergySummary(featureSnapshot.BandEnergySummary)
	stereoRelation := buildStereoRelationSummary(featureSnapshot.StereoRelationSummary)
	mixPackage := buildMixPackage(req, status, waveformStatus, waveformMetrics, timeSegments, bandEnergy, stereoRelation, caps)
	deepPackage := buildDeepPackage(spectrogramStatus, featureSnapshot)

	return ObservationPacket{
		SchemaVersion: ObservationSchemaVersion,
		ObservationID: "obs_" + time.Now().UTC().Format("20060102T150405") + "_" + randomID(),
		MixSessionID:  req.MixSessionID,
		Round:         req.Round,
		Status:        status,
		TargetRef:     normalizeTarget(req.TargetRef),
		MixObjects:    normalizeMixObjects(req.MixObjects, req.TargetRef, req.Args),
		ListenScope:   normalizeListenScope(req.ListenScope, req.MixObjects, req.Args),
		TimeRuler: TimeRuler{
			DurationSeconds: duration,
			SegmentSeconds:  segment,
			FrameSeconds:    frame,
			TempoBPM:        tempo,
			BarMapAvailable: tempo != nil,
		},
		GlobalSummary: map[string]any{
			"rms_dbfs":              waveformMetrics["rms_dbfs"],
			"peak_dbfs":             waveformMetrics["peak_dbfs"],
			"crest_db":              waveformMetrics["crest_db"],
			"dominant_problem_tags": problemTags,
			"feature_snapshot":      compactFeatureSnapshot(featureSnapshot),
		},
		EnvironmentPackage: environmentPackage,
		MixPackage:         mixPackage,
		DeepPackage:        deepPackage,
		SectionCandidates:  sections,
		TimelineDigest:     timeline,
		Hotspots:           hotspots,
		SourceCapabilities: caps,
		Notes:              notes,
		CreatedAt:          createdAt,
	}
}

func buildBoard(req Request, obs ObservationPacket, obsPath, now string) Board {
	blockers := []string{}
	if obs.SourceCapabilities["waveform_envelope"] != "ready" {
		if obs.SourceCapabilities["waveform_envelope"] == "requested" {
			blockers = append(blockers, "audio_feature_request_pending")
		} else if obs.SourceCapabilities["waveform_envelope"] == "blocked" {
			blockers = append(blockers, "audio_feature_request_blocked")
		} else {
			blockers = append(blockers, "audio_feature_reader_not_connected")
		}
	}
	return Board{
		SchemaVersion:         BoardSchemaVersion,
		MixSessionID:          req.MixSessionID,
		Status:                obs.Status,
		GoalText:              strings.TrimSpace(req.GoalText),
		TargetRef:             obs.TargetRef,
		MixObjects:            obs.MixObjects,
		ListenScope:           obs.ListenScope,
		ObservationCount:      obs.Round,
		LatestObservationID:   obs.ObservationID,
		LatestObservationPath: obsPath,
		ActiveProblemMap:      obs.Hotspots,
		SectionCandidates:     obs.SectionCandidates,
		PackageStatus:         packageStatus(obs),
		OpenBlockers:          blockers,
		UpdatedAt:             now,
	}
}

func featureSnapshotPath(args map[string]any) string {
	if path := strings.TrimSpace(fmt.Sprint(args["feature_snapshot_path"])); path != "" && path != "<nil>" {
		return filepath.Clean(path)
	}
	root := DefaultRoot()
	return filepath.Join(filepath.Dir(root), "mixboard_feature_snapshot.json")
}

func FeatureSnapshotPath(args map[string]any) string {
	return featureSnapshotPath(args)
}

func loadFeatureSnapshot(args map[string]any) featureSnapshot {
	path := featureSnapshotPath(args)
	data, err := os.ReadFile(path)
	if err != nil {
		return featureSnapshot{
			SchemaVersion:         "mixboard_feature_snapshot.v1",
			WaveformEnvelope:      map[string]any{"status": "missing"},
			SpectrogramTiles:      map[string]any{"status": "missing"},
			BandEnergySummary:     map[string]any{"status": "missing"},
			StereoRelationSummary: map[string]any{"status": "missing"},
		}
	}
	var snap featureSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return featureSnapshot{
			SchemaVersion:         "mixboard_feature_snapshot.v1",
			WaveformEnvelope:      map[string]any{"status": "invalid"},
			SpectrogramTiles:      map[string]any{"status": "invalid"},
			BandEnergySummary:     map[string]any{"status": "invalid"},
			StereoRelationSummary: map[string]any{"status": "invalid"},
		}
	}
	if snap.WaveformEnvelope == nil {
		snap.WaveformEnvelope = map[string]any{"status": "missing"}
	}
	if snap.SpectrogramTiles == nil {
		snap.SpectrogramTiles = map[string]any{"status": "missing"}
	}
	if snap.BandEnergySummary == nil {
		snap.BandEnergySummary = map[string]any{"status": "missing"}
	}
	if snap.StereoRelationSummary == nil {
		snap.StereoRelationSummary = map[string]any{"status": "missing"}
	}
	return snap
}

func featureStatus(row map[string]any) string {
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(row["status"])))
	switch status {
	case "ready", "partial", "requested", "blocked", "missing", "unavailable", "invalid":
		return status
	case "":
		return "missing"
	default:
		return status
	}
}

func observationStatusFromFeatures(duration float64, waveformStatus, spectrogramStatus string) string {
	if duration <= 0 {
		return "unavailable"
	}
	if waveformStatus == "ready" {
		return "ready"
	}
	if spectrogramStatus == "ready" || spectrogramStatus == "partial" {
		return "partial"
	}
	return "partial"
}

func compactFeatureSnapshot(snap featureSnapshot) map[string]any {
	return map[string]any{
		"schema_version":          snap.SchemaVersion,
		"updated_at":              snap.UpdatedAt,
		"latest_request":          compactFeatureRequest(snap.LatestRequest),
		"waveform_envelope":       compactFeatureRow(snap.WaveformEnvelope),
		"spectrogram_tiles":       compactFeatureRow(snap.SpectrogramTiles),
		"band_energy_summary":     compactFeatureRow(snap.BandEnergySummary),
		"stereo_relation_summary": compactFeatureRow(snap.StereoRelationSummary),
	}
}

func compactFeatureRow(row map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"status", "track_id", "clip_id", "request_id", "reason", "float_count", "tile_count_seen", "tile_count_expected", "total_duration", "rms", "peak_abs", "updated_at", "time_segments", "source", "bands", "band_count", "left_level_db", "right_level_db", "balance_db", "balance_unit", "balance_state", "phase_deviation", "phase_negative_ratio", "correlation_estimate", "correlation_state", "bin_count"} {
		if value, ok := row[key]; ok {
			out[key] = value
		}
	}
	return out
}

func compactFeatureRequest(row map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"schema_version", "request_id", "status", "reason", "resolved_target", "created_at"} {
		if value, ok := row[key]; ok {
			out[key] = value
		}
	}
	if requested, ok := row["requested_features"]; ok {
		out["requested_features"] = requested
	}
	return out
}

func buildWaveformMetrics(row map[string]any) map[string]any {
	rms := numberFromMap(row, "rms")
	peak := numberFromMap(row, "peak_abs")
	metrics := map[string]any{
		"status":      featureStatus(row),
		"rms":         nullablePositive(rms),
		"peak_abs":    nullablePositive(peak),
		"rms_dbfs":    approxDbfs(rms),
		"peak_dbfs":   approxDbfs(peak),
		"headroom_db": nil,
		"crest_db":    nil,
	}
	if peak > 0 {
		metrics["headroom_db"] = round3(-20 * math.Log10(peak))
	}
	if rms > 0 && peak > 0 {
		metrics["crest_db"] = round3(20 * math.Log10(peak/rms))
	}
	if v, ok := row["float_count"]; ok {
		metrics["sample_count"] = v
	}
	if v, ok := row["updated_at"]; ok {
		metrics["updated_at"] = v
	}
	return metrics
}

func buildBandEnergySummary(row map[string]any) map[string]any {
	status := featureStatus(row)
	out := map[string]any{
		"status": status,
	}
	if status != "ready" {
		return out
	}
	out["source"] = cleanAnyString(row["source"])
	out["updated_at"] = row["updated_at"]
	bandsIn, _ := row["bands"].(map[string]any)
	bands := map[string]any{}
	for _, id := range []string{"sub", "bass", "low_mid", "mid", "presence", "air"} {
		bandRow, _ := bandsIn[id].(map[string]any)
		if len(bandRow) == 0 {
			continue
		}
		bands[id] = map[string]any{
			"unit_energy": nullablePositive(numberFromMap(bandRow, "unit_energy")),
			"energy_db":   firstNonNil(bandRow["energy_db"], approxDbfs(numberFromMap(bandRow, "unit_energy"))),
			"min_hz":      numberFromMap(bandRow, "min_hz"),
			"max_hz":      numberFromMap(bandRow, "max_hz"),
		}
	}
	out["bands"] = bands
	return out
}

func buildStereoRelationSummary(row map[string]any) map[string]any {
	status := featureStatus(row)
	out := map[string]any{
		"status": status,
	}
	if status != "ready" {
		return out
	}
	for _, key := range []string{"source", "updated_at", "balance_state", "correlation_state"} {
		if value, ok := row[key]; ok {
			out[key] = value
		}
	}
	for _, key := range []string{"left_level_db", "right_level_db", "balance_db", "balance_unit", "phase_deviation", "phase_negative_ratio", "correlation_estimate"} {
		out[key] = round3(numberFromMap(row, key))
	}
	if binCount := numberFromMap(row, "bin_count"); binCount > 0 {
		out["bin_count"] = int(binCount)
	}
	return out
}

func buildEnvironmentPackage(req Request, duration, segment, frame float64, tempo *float64, caps map[string]string, snap featureSnapshot) map[string]any {
	status := "ready"
	if duration <= 0 || caps["waveform_envelope"] != "ready" {
		status = "partial"
	}
	return map[string]any{
		"schema_version": "mixboard_environment.v1",
		"status":         status,
		"role":           "environment",
		"target_ref":     normalizeTarget(req.TargetRef),
		"mix_objects":    normalizeMixObjects(req.MixObjects, req.TargetRef, req.Args),
		"listen_scope":   normalizeListenScope(req.ListenScope, req.MixObjects, req.Args),
		"time_ruler": TimeRuler{
			DurationSeconds: duration,
			SegmentSeconds:  segment,
			FrameSeconds:    frame,
			TempoBPM:        tempo,
			BarMapAvailable: tempo != nil,
		},
		"source_capabilities": caps,
		"feature_snapshot":    compactFeatureSnapshot(snap),
	}
}

func buildMixPackage(req Request, observationStatus, waveformStatus string, waveformMetrics map[string]any, timeSegments []map[string]any, bandEnergy map[string]any, stereoRelation map[string]any, caps map[string]string) map[string]any {
	status := "limited"
	if observationStatus == "ready" && waveformStatus == "ready" {
		status = "baseline_ready"
	}
	missing := []string{}
	if waveformStatus != "ready" {
		missing = append(missing, "waveform_envelope")
	}
	if cleanAnyString(bandEnergy["status"]) != "ready" {
		missing = append(missing, "band_energy_summary")
	}
	if cleanAnyString(stereoRelation["status"]) != "ready" {
		missing = append(missing, "stereo_correlation")
	}
	missing = append(missing, "before_after_delta")
	timeEnergyStatus := "missing"
	if len(timeSegments) > 0 {
		timeEnergyStatus = "ready"
	}
	return map[string]any{
		"schema_version": "mixboard_mix_packet.v1",
		"status":         status,
		"role":           "realtime_mix_loop",
		"round":          req.Round,
		"goal_text":      strings.TrimSpace(req.GoalText),
		"current_metrics": map[string]any{
			"waveform":        waveformMetrics,
			"time_energy":     capRows(timeSegments, 48),
			"band_energy":     bandEnergy,
			"stereo_relation": stereoRelation,
		},
		"available_controls": []string{
			"gain_staging_observation",
			"headroom_check",
			"coarse_dynamic_observation",
			"time_energy_observation",
			"band_energy_observation",
			"stereo_image_observation",
		},
		"missing_metrics": missing,
		"source_capabilities": map[string]string{
			"waveform_envelope":  caps["waveform_envelope"],
			"time_energy":        timeEnergyStatus,
			"band_energy":        cleanAnyString(bandEnergy["status"]),
			"stereo_correlation": cleanAnyString(stereoRelation["status"]),
			"before_after_delta": "missing",
		},
	}
}

func applyBeforeAfterDelta(obs *ObservationPacket, before ObservationPacket, ok bool, now string) {
	if obs == nil {
		return
	}
	delta := buildBeforeAfterDelta(before, *obs, ok, now)
	metrics, _ := obs.MixPackage["current_metrics"].(map[string]any)
	if metrics == nil {
		metrics = map[string]any{}
		obs.MixPackage["current_metrics"] = metrics
	}
	metrics["before_after_delta"] = delta
	caps, _ := obs.MixPackage["source_capabilities"].(map[string]string)
	if caps == nil {
		caps = map[string]string{}
		obs.MixPackage["source_capabilities"] = caps
	}
	caps["before_after_delta"] = featureStatus(delta)
	obs.MixPackage["missing_metrics"] = removeStringFromAnySlice(obs.MixPackage["missing_metrics"], "before_after_delta")
	if featureStatus(delta) != "ready" {
		obs.MixPackage["missing_metrics"] = appendStringIfMissing(obs.MixPackage["missing_metrics"], "before_after_delta")
	}
}

func buildBeforeAfterDelta(before, after ObservationPacket, ok bool, now string) map[string]any {
	if !ok || strings.TrimSpace(before.ObservationID) == "" {
		return map[string]any{
			"status":     "pending",
			"reason":     "no_previous_mix_observation",
			"updated_at": now,
		}
	}
	beforeMetrics := mixCurrentMetrics(before)
	afterMetrics := mixCurrentMetrics(after)
	if len(beforeMetrics) == 0 || len(afterMetrics) == 0 {
		return map[string]any{
			"status":                "missing",
			"reason":                "mix_metrics_unavailable",
			"before_observation_id": before.ObservationID,
			"after_observation_id":  after.ObservationID,
			"updated_at":            now,
		}
	}
	out := map[string]any{
		"status":                "ready",
		"source":                "mixboard_observation_pair",
		"before_observation_id": before.ObservationID,
		"after_observation_id":  after.ObservationID,
		"before_round":          before.Round,
		"after_round":           after.Round,
		"updated_at":            now,
	}
	if waveform := metricDeltaBlock(beforeMetrics["waveform"], afterMetrics["waveform"], []string{"rms_dbfs", "peak_dbfs", "crest_db", "headroom_db"}); len(waveform) > 0 {
		out["waveform"] = waveform
	}
	if bands := bandEnergyDelta(beforeMetrics["band_energy"], afterMetrics["band_energy"]); len(bands) > 0 {
		out["band_energy"] = bands
	}
	if stereo := metricDeltaBlock(beforeMetrics["stereo_relation"], afterMetrics["stereo_relation"], []string{"balance_db", "phase_deviation", "phase_negative_ratio", "correlation_estimate"}); len(stereo) > 0 {
		out["stereo_relation"] = stereo
	}
	tags := beforeAfterDeltaTags(out)
	out["summary_tags"] = tags
	out["summary"] = beforeAfterDeltaSummary(tags)
	return out
}

func mixCurrentMetrics(obs ObservationPacket) map[string]any {
	metrics, _ := obs.MixPackage["current_metrics"].(map[string]any)
	return metrics
}

func metricDeltaBlock(beforeValue, afterValue any, keys []string) map[string]any {
	before, _ := beforeValue.(map[string]any)
	after, _ := afterValue.(map[string]any)
	if len(before) == 0 || len(after) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, key := range keys {
		beforeNumber := numberFromMap(before, key)
		afterNumber := numberFromMap(after, key)
		if beforeNumber == 0 && afterNumber == 0 {
			continue
		}
		out[key] = map[string]any{
			"before": beforeNumber,
			"after":  afterNumber,
			"delta":  round3(afterNumber - beforeNumber),
		}
	}
	for _, key := range []string{"status", "balance_state", "correlation_state"} {
		beforeText := cleanAnyString(before[key])
		afterText := cleanAnyString(after[key])
		if beforeText != "" || afterText != "" {
			out[key] = map[string]any{"before": beforeText, "after": afterText, "changed": beforeText != afterText}
		}
	}
	return out
}

func bandEnergyDelta(beforeValue, afterValue any) map[string]any {
	before, _ := beforeValue.(map[string]any)
	after, _ := afterValue.(map[string]any)
	beforeBands, _ := before["bands"].(map[string]any)
	afterBands, _ := after["bands"].(map[string]any)
	if len(beforeBands) == 0 || len(afterBands) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, id := range []string{"sub", "bass", "low_mid", "mid", "presence", "air"} {
		beforeBand, _ := beforeBands[id].(map[string]any)
		afterBand, _ := afterBands[id].(map[string]any)
		if len(beforeBand) == 0 || len(afterBand) == 0 {
			continue
		}
		beforeDB := numberFromMap(beforeBand, "energy_db")
		afterDB := numberFromMap(afterBand, "energy_db")
		beforeEnergy := numberFromMap(beforeBand, "unit_energy")
		afterEnergy := numberFromMap(afterBand, "unit_energy")
		if beforeDB == 0 && afterDB == 0 && beforeEnergy == 0 && afterEnergy == 0 {
			continue
		}
		out[id] = map[string]any{
			"energy_db": map[string]any{
				"before": beforeDB,
				"after":  afterDB,
				"delta":  round3(afterDB - beforeDB),
			},
			"unit_energy": map[string]any{
				"before": beforeEnergy,
				"after":  afterEnergy,
				"delta":  round3(afterEnergy - beforeEnergy),
			},
		}
	}
	return out
}

func beforeAfterDeltaTags(delta map[string]any) []string {
	tags := []string{}
	if waveform, _ := delta["waveform"].(map[string]any); len(waveform) > 0 {
		if row, _ := waveform["peak_dbfs"].(map[string]any); math.Abs(numberFromMap(row, "delta")) >= 1.0 {
			tags = append(tags, "peak_changed")
		}
		if row, _ := waveform["rms_dbfs"].(map[string]any); math.Abs(numberFromMap(row, "delta")) >= 1.0 {
			tags = append(tags, "rms_changed")
		}
	}
	if bands, _ := delta["band_energy"].(map[string]any); len(bands) > 0 {
		for id, raw := range bands {
			row, _ := raw.(map[string]any)
			energyDB, _ := row["energy_db"].(map[string]any)
			if math.Abs(numberFromMap(energyDB, "delta")) >= 1.5 {
				tags = append(tags, "band_"+id+"_changed")
			}
		}
	}
	if stereo, _ := delta["stereo_relation"].(map[string]any); len(stereo) > 0 {
		if row, _ := stereo["correlation_estimate"].(map[string]any); math.Abs(numberFromMap(row, "delta")) >= 0.1 {
			tags = append(tags, "stereo_correlation_changed")
		}
		if row, _ := stereo["balance_db"].(map[string]any); math.Abs(numberFromMap(row, "delta")) >= 1.0 {
			tags = append(tags, "stereo_balance_changed")
		}
	}
	if len(tags) == 0 {
		tags = append(tags, "no_significant_change")
	}
	return tags
}

func beforeAfterDeltaSummary(tags []string) string {
	if len(tags) == 0 || (len(tags) == 1 && tags[0] == "no_significant_change") {
		return "本轮与上一轮观察差异很小。"
	}
	return "本轮相对上一轮已有可测声学变化。"
}

func readLatestObservation(sessionDir string) (ObservationPacket, bool) {
	boardPath := filepath.Join(sessionDir, "current.json")
	data, err := os.ReadFile(boardPath)
	if err != nil || len(data) == 0 {
		return ObservationPacket{}, false
	}
	var board Board
	if err := json.Unmarshal(data, &board); err != nil {
		return ObservationPacket{}, false
	}
	path := strings.TrimSpace(board.LatestObservationPath)
	if path == "" {
		return ObservationPacket{}, false
	}
	obsData, err := os.ReadFile(path)
	if err != nil || len(obsData) == 0 {
		return ObservationPacket{}, false
	}
	var obs ObservationPacket
	if err := json.Unmarshal(obsData, &obs); err != nil {
		return ObservationPacket{}, false
	}
	return obs, true
}

func removeStringFromAnySlice(value any, target string) []string {
	out := []string{}
	for _, raw := range anySlice(value) {
		text := cleanAnyString(raw)
		if text == "" || text == target {
			continue
		}
		out = append(out, text)
	}
	return out
}

func appendStringIfMissing(value any, item string) []string {
	out := removeStringFromAnySlice(value, "")
	for _, existing := range out {
		if existing == item {
			return out
		}
	}
	return append(out, item)
}

func buildDeepPackage(spectrogramStatus string, snap featureSnapshot) map[string]any {
	status := "async_missing"
	if spectrogramStatus == "ready" || spectrogramStatus == "partial" {
		status = "async_available"
	}
	return map[string]any{
		"schema_version": "mixboard_deep_packet.v1",
		"status":         status,
		"role":           "slow_context_refresh",
		"source_capabilities": map[string]string{
			"deep_band_observation": spectrogramStatus,
			"section_detection":     "heuristic",
			"masking_analysis":      "missing",
			"reference_match":       "missing",
			"lufs_analysis":         "missing",
		},
		"feature_snapshot": map[string]any{
			"spectrogram_tiles": compactFeatureRow(snap.SpectrogramTiles),
		},
	}
}

func packageStatus(obs ObservationPacket) map[string]string {
	return map[string]string{
		"environment": cleanAnyString(obs.EnvironmentPackage["status"]),
		"mix":         cleanAnyString(obs.MixPackage["status"]),
		"deep":        cleanAnyString(obs.DeepPackage["status"]),
	}
}

func buildFeatureHotspots(snap featureSnapshot, waveformStatus, spectrogramStatus string) []map[string]any {
	out := []map[string]any{}
	if waveformStatus == "ready" {
		if peak := numberFromMap(snap.WaveformEnvelope, "peak_abs"); peak > 0.95 {
			out = append(out, map[string]any{
				"tag":        "peak_near_full_scale",
				"summary":    "波形峰值接近满刻度，后续调控需留意削波风险",
				"confidence": 0.45,
			})
		}
	}
	if spectrogramStatus == "ready" || spectrogramStatus == "partial" {
		out = append(out, map[string]any{
			"tag":        "deep_band_observation_available",
			"summary":    "深度频段观察可用，可作为后续精修参考",
			"confidence": 0.35,
		})
	}
	return out
}

func waveformTimeSegments(row map[string]any) []map[string]any {
	rows := anySlice(row["time_segments"])
	out := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		segment, _ := raw.(map[string]any)
		if len(segment) == 0 {
			continue
		}
		start := numberFromMap(segment, "start_seconds")
		end := numberFromMap(segment, "end_seconds")
		if end <= start {
			continue
		}
		rms := numberFromMap(segment, "rms")
		peak := numberFromMap(segment, "peak_abs")
		rowOut := map[string]any{
			"start_seconds": round3(start),
			"end_seconds":   round3(end),
			"rms":           nullablePositive(rms),
			"rms_dbfs":      firstNonNil(segment["rms_dbfs"], approxDbfs(rms)),
			"peak_abs":      nullablePositive(peak),
			"peak_dbfs":     firstNonNil(segment["peak_dbfs"], approxDbfs(peak)),
			"crest_db":      firstNonNil(segment["crest_db"], crestDb(peak, rms)),
			"energy_state":  firstNonEmpty(cleanAnyString(segment["energy_state"]), energyState(rms)),
		}
		out = append(out, rowOut)
	}
	return out
}

func numberFromMap(row map[string]any, key string) float64 {
	value, ok := row[key]
	if !ok || value == nil {
		return 0
	}
	switch x := value.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		v, _ := x.Float64()
		return v
	case string:
		v, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return v
	default:
		return 0
	}
}

func approxDbfs(value float64) any {
	if value <= 0 {
		return nil
	}
	return round3(20 * math.Log10(value))
}

func crestDb(peak, rms float64) any {
	if peak <= 0 || rms <= 0 {
		return nil
	}
	return round3(20 * math.Log10(peak/rms))
}

func energyState(rms float64) string {
	if rms <= 0.0001 {
		return "silent"
	}
	if rms < 0.02 {
		return "low"
	}
	if rms < 0.12 {
		return "medium"
	}
	return "high"
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func nullablePositive(value float64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func buildContextPack(req Request, board Board, obs ObservationPacket, now string) ContextPack {
	latest := map[string]any{
		"observation_id":      obs.ObservationID,
		"status":              obs.Status,
		"time_ruler":          obs.TimeRuler,
		"global_summary":      obs.GlobalSummary,
		"environment_package": obs.EnvironmentPackage,
		"mix_package":         obs.MixPackage,
		"deep_package":        obs.DeepPackage,
		"timeline_digest":     capRows(obs.TimelineDigest, 12),
		"hotspots":            capRows(obs.Hotspots, 12),
		"source_capabilities": obs.SourceCapabilities,
	}
	return ContextPack{
		SchemaVersion: ContextPackSchemaVersion,
		MixSessionID:  req.MixSessionID,
		GeneratedAt:   now,
		SessionHeader: map[string]any{
			"mix_session_id": req.MixSessionID,
			"round":          obs.Round,
			"goal_text":      strings.TrimSpace(req.GoalText),
			"target_ref":     obs.TargetRef,
			"mix_objects":    obs.MixObjects,
			"listen_scope":   obs.ListenScope,
		},
		LatestObservation: latest,
		ActiveProblemMap:  capRows(board.ActiveProblemMap, 12),
		RelevantSections:  capRows(board.SectionCandidates, 8),
		OpenBlockers:      board.OpenBlockers,
	}
}

func buildTimelineDigest(duration, segment float64, timeSegments []map[string]any) []map[string]any {
	if len(timeSegments) > 0 {
		out := make([]map[string]any, 0, minInt(len(timeSegments), 48))
		for _, row := range timeSegments {
			out = append(out, map[string]any{
				"start_seconds": row["start_seconds"],
				"end_seconds":   row["end_seconds"],
				"energy_state":  row["energy_state"],
				"rms_dbfs":      row["rms_dbfs"],
				"peak_dbfs":     row["peak_dbfs"],
				"crest_db":      row["crest_db"],
				"band_energy":   map[string]any{},
				"tags":          energyTags(row),
			})
			if len(out) >= 48 {
				break
			}
		}
		return out
	}
	if duration <= 0 || segment <= 0 {
		return nil
	}
	count := int(duration / segment)
	if float64(count)*segment < duration {
		count++
	}
	if count > 48 {
		count = 48
	}
	out := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		start := float64(i) * segment
		end := start + segment
		if end > duration {
			end = duration
		}
		out = append(out, map[string]any{
			"start_seconds": round3(start),
			"end_seconds":   round3(end),
			"energy_state":  "unknown",
			"band_energy":   map[string]any{},
			"tags":          []string{},
		})
		if end >= duration {
			break
		}
	}
	return out
}

func energyTags(row map[string]any) []string {
	tags := []string{}
	state := strings.ToLower(cleanAnyString(row["energy_state"]))
	if state == "silent" || state == "low" {
		tags = append(tags, "low_energy")
	}
	if peak := numberFromMap(row, "peak_abs"); peak > 0.95 {
		tags = append(tags, "peak_near_full_scale")
	}
	return tags
}

func buildSectionCandidates(duration float64) []map[string]any {
	if duration <= 0 {
		return nil
	}
	if duration <= 16 {
		return []map[string]any{{"label_hint": "full", "start_seconds": 0, "end_seconds": round3(duration), "confidence": 0.25}}
	}
	introEnd := minFloat(duration, maxFloat(8, duration*0.12))
	out := []map[string]any{
		{"label_hint": "intro", "start_seconds": 0, "end_seconds": round3(introEnd), "confidence": 0.25},
	}
	if introEnd < duration {
		out = append(out, map[string]any{"label_hint": "body", "start_seconds": round3(introEnd), "end_seconds": round3(duration), "confidence": 0.2})
	}
	return out
}

func normalizeTarget(target TargetRef) TargetRef {
	target.Kind = strings.TrimSpace(target.Kind)
	if target.Kind == "" {
		target.Kind = "selection"
	}
	target.ID = strings.TrimSpace(target.ID)
	target.Label = strings.TrimSpace(target.Label)
	target.Source = strings.TrimSpace(target.Source)
	target.Confidence = strings.TrimSpace(target.Confidence)
	return target
}

func normalizeMixObjects(objects []MixObject, target TargetRef, args map[string]any) []MixObject {
	if len(objects) == 0 {
		if rows := anySlice(args["mix_objects"]); len(rows) > 0 {
			for _, rowValue := range rows {
				row, _ := rowValue.(map[string]any)
				obj := MixObject{
					Mode:        cleanAnyString(row["mode"]),
					Kind:        cleanAnyString(firstPresent(row, "kind", "type")),
					ID:          cleanAnyString(row["id"]),
					Label:       cleanAnyString(firstPresent(row, "label", "name")),
					EffectScope: cleanAnyString(row["effect_scope"]),
					Source:      cleanAnyString(row["source"]),
				}
				if obj.Kind != "" || obj.ID != "" {
					objects = append(objects, obj)
				}
			}
		}
	}
	if len(objects) == 0 {
		target = normalizeTarget(target)
		effectScope := "track_rack"
		if target.Kind == "clip" {
			effectScope = "clip_scope"
		}
		objects = append(objects, MixObject{
			Mode:        "target_ref",
			Kind:        target.Kind,
			ID:          target.ID,
			Label:       target.Label,
			EffectScope: effectScope,
			Source:      firstNonEmpty(target.Source, "target_ref"),
		})
	}
	out := make([]MixObject, 0, len(objects))
	for _, obj := range objects {
		obj.Kind = strings.TrimSpace(obj.Kind)
		if obj.Kind == "" {
			obj.Kind = "track"
		}
		obj.ID = strings.TrimSpace(obj.ID)
		obj.Label = strings.TrimSpace(obj.Label)
		obj.Mode = strings.TrimSpace(obj.Mode)
		obj.EffectScope = strings.TrimSpace(obj.EffectScope)
		if obj.EffectScope == "" {
			if obj.Kind == "clip" {
				obj.EffectScope = "clip_scope"
			} else {
				obj.EffectScope = "track_rack"
			}
		}
		obj.Source = strings.TrimSpace(obj.Source)
		out = append(out, obj)
	}
	return out
}

func normalizeListenScope(scope ListenScope, objects []MixObject, args map[string]any) ListenScope {
	if scope.Time.Mode == "" {
		if ts, ok := args["time_selection"].(map[string]any); ok && contextBool(ts, "active") {
			scope.Time = ListenTimeScope{
				Mode:         "time_selection",
				StartSeconds: firstPositiveFloat(ts, "start_seconds"),
				EndSeconds:   firstPositiveFloat(ts, "end_seconds"),
				Locked:       contextBool(ts, "locked"),
				Source:       firstNonEmpty(cleanAnyString(ts["source"]), "time_selection"),
			}
		}
	}
	if scope.Time.Mode == "" {
		start := firstPositiveFloat(args, "listen_time_start_seconds", "start_seconds")
		end := firstPositiveFloat(args, "listen_time_end_seconds", "end_seconds")
		if end > start && end > 0 {
			scope.Time = ListenTimeScope{Mode: "manual_time", StartSeconds: start, EndSeconds: end, Locked: true, Source: "context"}
		}
	}
	if scope.Time.Mode == "" {
		scope.Time = ListenTimeScope{Mode: "full_song", Source: "default"}
	}
	if scope.Source.Mode == "" {
		scope.Source = ListenSourceScope{Mode: "full_mix_context"}
	}
	if len(scope.Source.FocusIDs) == 0 {
		for _, obj := range objects {
			if strings.TrimSpace(obj.ID) != "" {
				scope.Source.FocusIDs = append(scope.Source.FocusIDs, obj.ID)
			}
		}
	}
	return scope
}

func cleanAnyString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func contextBool(row map[string]any, key string) bool {
	switch v := row[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func projectDuration(state map[string]any) float64 {
	if v := firstPositiveFloat(state, "duration_seconds", "project_duration_seconds", "length_seconds"); v > 0 {
		return v
	}
	maxEnd := 0.0
	for _, item := range anySlice(state["tracks"]) {
		track, _ := item.(map[string]any)
		for _, clipValue := range anySlice(firstPresent(track, "clips", "clip_summaries")) {
			clip, _ := clipValue.(map[string]any)
			start := firstPositiveFloat(clip, "start_seconds", "start")
			length := firstPositiveFloat(clip, "length_seconds", "length", "duration_seconds", "duration")
			if end := start + length; end > maxEnd {
				maxEnd = end
			}
		}
	}
	return maxEnd
}

func firstPositiveFloat(row map[string]any, keys ...string) float64 {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		switch x := value.(type) {
		case int:
			if x > 0 {
				return float64(x)
			}
		case int64:
			if x > 0 {
				return float64(x)
			}
		case float64:
			if x > 0 {
				return x
			}
		case json.Number:
			f, err := x.Float64()
			if err == nil && f > 0 {
				return f
			}
		default:
			f, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(x)), 64)
			if err == nil && f > 0 {
				return f
			}
		}
	}
	return 0
}

func anySlice(value any) []any {
	switch x := value.(type) {
	case []any:
		return x
	case []string:
		out := make([]any, 0, len(x))
		for _, row := range x {
			out = append(out, row)
		}
		return out
	case []map[string]any:
		out := make([]any, 0, len(x))
		for _, row := range x {
			out = append(out, row)
		}
		return out
	default:
		return nil
	}
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return nil
}

func capRows(rows []map[string]any, max int) []map[string]any {
	if max <= 0 || len(rows) <= max {
		return rows
	}
	return append([]map[string]any(nil), rows[:max]...)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func safePathName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "mix_unknown"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func randomID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("150405.000000")))
	}
	return hex.EncodeToString(b[:])
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
