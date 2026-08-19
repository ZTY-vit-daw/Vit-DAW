package mixboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/tim"
)

type Catalog struct {
	SchemaVersion string         `json:"schema_version"`
	ObservationID string         `json:"observation_id"`
	GeneratedAt   string         `json:"generated_at"`
	Entries       []CatalogEntry `json:"entries"`
}

type CatalogEntry struct {
	Key          string   `json:"key"`
	Kind         string   `json:"kind"`
	Freshness    string   `json:"freshness"`
	Cost         string   `json:"cost"`
	Summary      string   `json:"summary,omitempty"`
	ReadHint     string   `json:"read_hint,omitempty"`
	TargetKind   string   `json:"target_kind,omitempty"`
	TargetID     string   `json:"target_id,omitempty"`
	AvailableOps []string `json:"available_ops,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

type ReadRequest struct {
	ObservationID string
	MixSessionID  string
	Keys          []string
	RangeStart    float64
	RangeEnd      float64
	Detail        string
	MaxItems      int
}

type DeriveRequest struct {
	ObservationID string
	MixSessionID  string
	Type          string
	A             map[string]any
	B             map[string]any
	Focus         map[string]any
	Dimensions    []string
	MaxItems      int
}

func FinalizeObservationContext(obs *ObservationPacket, req Request, now string) {
	if obs == nil {
		return
	}
	if strings.TrimSpace(now) == "" {
		now = time.Now().UTC().Format(time.RFC3339Nano)
	}
	wantsDOM := strings.TrimSpace(cleanAnyString(req.Args["dom_mode"])) != ""
	if wantsDOM {
		finalizeDOMProjection(obs, req)
	}
	if observationWantsBandStereoProjection(req.Args) {
		applyBandStereoProjection(obs, req)
	}
	finalizeMOMProjection(obs, req)
	finalizeTIMProjection(obs, req)
	finalizeFXMProjection(obs, req)
	finalizeCOMProjection(obs, req)
	if !wantsDOM && !observationWantsBandStereoProjection(req.Args) {
		finalizeDOMProjection(obs, req)
	}
	obs.Digest = BuildDigest(*obs, req)
	obs.Catalog = BuildCatalog(*obs, req, now)
	if observationWantsBandStereoProjection(req.Args) {
		filterBandStereoProjectionCatalog(obs)
		stripObservationRawKeys(obs)
	}
}

func BuildDigest(obs ObservationPacket, req Request) map[string]any {
	out := map[string]any{
		"schema_version": ObservationDigestVersion,
		"observation_id": obs.ObservationID,
		"status":         obs.Status,
		"scope":          firstNonEmpty(cleanAnyString(req.Args["scope"]), obs.ListenScope.Source.Mode),
		"listen_scope": map[string]any{
			"time_mode":   obs.ListenScope.Time.Mode,
			"source_mode": obs.ListenScope.Source.Mode,
			"focus_ids":   obs.ListenScope.Source.FocusIDs,
			"context_ids": obs.ListenScope.Source.ContextIDs,
		},
		"target": map[string]any{
			"kind":  obs.TargetRef.Kind,
			"id":    obs.TargetRef.ID,
			"label": obs.TargetRef.Label,
		},
		"available_detail": map[string]any{},
	}
	if obs.TimeRuler.DurationSeconds > 0 {
		out["duration_seconds"] = obs.TimeRuler.DurationSeconds
	}
	for _, key := range []string{"peak_dbfs", "rms_dbfs", "crest_db", "dominant_problem_tags"} {
		if value, ok := obs.GlobalSummary[key]; ok && value != nil {
			out[key] = value
		}
	}
	metrics, _ := obs.MixPackage["current_metrics"].(map[string]any)
	if waveform, _ := metrics["waveform"].(map[string]any); len(waveform) > 0 {
		out["waveform"] = compactKeys(waveform, []string{"status", "peak_dbfs", "rms_dbfs", "headroom_db", "crest_db"})
	}
	if rows := mapRowsAny(metrics["time_energy"]); len(rows) > 0 {
		out["time_energy_excerpt"] = capRows(rows, 8)
	}
	if band, _ := metrics["band_energy"].(map[string]any); len(band) > 0 {
		out["band_energy_status"] = band["status"]
	}
	if stereo, _ := metrics["stereo_relation"].(map[string]any); len(stereo) > 0 {
		out["stereo_status"] = stereo["status"]
		for _, key := range []string{"balance_db", "balance_state", "correlation_estimate", "correlation_state"} {
			if value, ok := stereo[key]; ok && value != nil {
				out[key] = value
			}
		}
	}
	if len(obs.AcousticPackageStatus) > 0 {
		out["acoustic_package_status"] = compactAcousticStatusForDigest(obs.AcousticPackageStatus)
	}
	if obs.MOMProjection != nil {
		out["mom_projection"] = map[string]any{
			"mom_version":   obs.MOMProjection.MOMVersion,
			"intent":        obs.MOMProjection.Intent,
			"trust_quality": obs.MOMProjection.TrustQuality,
			"llm_context":   obs.MOMProjection.LLMContext,
		}
	}
	if obs.TIMProjection != nil {
		out["tim_projection"] = map[string]any{
			"schema_version":    obs.TIMProjection.SchemaVersion,
			"tim_version":       obs.TIMProjection.TIMVersion,
			"status":            obs.TIMProjection.Status,
			"technical_summary": compactTIMSummary(obs.TIMProjection.TechnicalSummary),
			"coverage":          obs.TIMProjection.Coverage,
			"risk_summary":      obs.TIMProjection.RiskSummary,
		}
	}
	if obs.COMProjection != nil {
		out["com_projection"] = map[string]any{
			"schema_version": obs.COMProjection.SchemaVersion,
			"com_version":    obs.COMProjection.COMVersion,
			"projection_id":  obs.COMProjection.ProjectionID,
			"mode":           obs.COMProjection.Mode,
			"status":         obs.COMProjection.Status,
			"trust_quality":  obs.COMProjection.TrustQuality,
			"llm_context":    obs.COMProjection.LLMContext,
		}
	}
	if obs.DOMProjection != nil {
		out["dom_projection"] = map[string]any{
			"schema_version": obs.DOMProjection.SchemaVersion,
			"dom_version":    obs.DOMProjection.DOMVersion,
			"projection_id":  obs.DOMProjection.ProjectionID,
			"mode":           obs.DOMProjection.Mode,
			"status":         obs.DOMProjection.Status,
			"trust_quality":  obs.DOMProjection.TrustQuality,
			"llm_context":    obs.DOMProjection.LLMContext,
		}
	}
	if sourceIdentity := observationSourceIdentity(obs); len(sourceIdentity) > 0 {
		out["source_identity"] = sourceIdentity
	}
	if missing := removeStringFromAnySlice(obs.MixPackage["missing_metrics"], ""); len(missing) > 0 {
		out["missing_metrics"] = missing
	}
	if len(obs.Hotspots) > 0 {
		out["hotspots"] = capRows(obs.Hotspots, 6)
	}
	if summary, _ := obs.ProjectPackage["summary"].(map[string]any); len(summary) > 0 {
		out["project_summary"] = summary
	}
	for _, key := range []string{"acoustic_track_count", "active_acoustic_track_count"} {
		if value, ok := obs.ProjectPackage[key]; ok && value != nil {
			out[key] = value
		}
	}
	if target, _ := obs.ProjectPackage["likely_first_attention_target"].(map[string]any); len(target) > 0 {
		out["likely_first_attention_target"] = target
	}
	if limitations := removeStringFromAnySlice(obs.ProjectPackage["limitations"], ""); len(limitations) > 0 {
		out["project_limitations"] = limitations
	}
	if ranking := mapRowsAny(obs.ProjectPackage["loudness_ranking"]); len(ranking) > 0 {
		out["project_loudness_ranking_excerpt"] = capRows(ranking, 6)
	}
	if ranking := mapRowsAny(obs.ProjectPackage["level_ranking"]); len(ranking) > 0 {
		out["project_level_ranking_excerpt"] = capRows(ranking, 6)
	}
	if ranking := mapRowsAny(obs.ProjectPackage["peak_ranking"]); len(ranking) > 0 {
		out["project_peak_ranking_excerpt"] = capRows(ranking, 6)
	}
	if ranking := mapRowsAny(obs.ProjectPackage["peak_risk_ranking"]); len(ranking) > 0 {
		out["project_peak_risk_ranking_excerpt"] = capRows(ranking, 6)
	}
	if ranking := mapRowsAny(obs.ProjectPackage["headroom_risk"]); len(ranking) > 0 {
		out["project_headroom_risk_excerpt"] = capRows(ranking, 6)
	}
	available, _ := out["available_detail"].(map[string]any)
	available["catalog"] = "ready"
	available["project_context"] = sourceStatus(obs, "project_context")
	available["project_track_waveforms"] = sourceStatus(obs, "track_waveform_envelopes")
	available["time_energy"] = sourceStatus(obs, "time_energy")
	available["band_energy"] = sourceStatus(obs, "band_energy")
	available["stereo_relation"] = sourceStatus(obs, "stereo_correlation")
	available["full_project_acoustic_render"] = sourceStatus(obs, "track_waveform_envelopes")
	available["raw_ranges"] = "available_through_mix_read_when_source_ready"
	return out
}

func compactTIMSummary(summary tim.TechnicalSummary) map[string]any {
	return map[string]any{
		"track_count":             summary.TrackCount,
		"clip_count":              summary.ClipCount,
		"empty_track_count":       summary.EmptyTrackCount,
		"source_missing_count":    summary.SourceMissingCount,
		"playback_invalid_count":  summary.PlaybackInvalidCount,
		"compressed_source_count": summary.CompressedSourceCount,
		"format_family_counts":    summary.FormatFamilyCounts,
	}
}

func BuildCatalog(obs ObservationPacket, req Request, now string) Catalog {
	targetID := firstNonEmpty(obs.TargetRef.ID, "target")
	targetKind := firstNonEmpty(obs.TargetRef.Kind, "selection")
	entries := []CatalogEntry{
		catalogEntry("observation.digest", "derived", "fresh", "cheap", "Default acoustic digest for LLM context.", "mix_read key=observation.digest", targetKind, targetID, now),
		catalogEntry("observation.binding", "derived", "fresh", "cheap", "Observation target, project revision, scope, and evidence lineage. No raw audio payload is included.", "mix_read key=observation.binding", targetKind, targetID, now),
		catalogEntry("observation.com_projection", "com_projection", comProjectionFreshness(obs), "medium", "COM v1 compact compression observation. Raw envelope and event evidence remains external.", "mix_read key=observation.com_projection", targetKind, targetID, now),
		catalogEntry("observation.dom_projection", "dom_projection", domProjectionFreshness(obs), "medium", "DOM v1 compact family-neutral dynamics observation. Raw time, event, and spectral payloads remain external.", "mix_read key=observation.dom_projection", targetKind, targetID, now),
		catalogEntry("observation.mom_projection", "mom_projection", "fresh", "cheap", "MOM v1 compact projection with trust quality, task layers, and evidence refs for LLM context.", "mix_read key=observation.mom_projection", targetKind, targetID, now),
		catalogEntry("observation.tim_projection", "tim_projection", "fresh", "cheap", "TIM v0 technical integrity projection with source, format, metadata coverage, and import risk checks.", "mix_read key=observation.tim_projection", targetKind, targetID, now),
		catalogEntry("observation.fxm_projection", "fxm_projection", fxmProjectionFreshness(obs), "medium", "FXM v0 counterfactual plug-in-chain transformation projection. Raw renders remain external evidence.", "mix_read key=observation.fxm_projection", targetKind, targetID, now),
		catalogEntry("project.static.summary", "static", "fresh", "cheap", projectSummaryText(req.ProjectState), "mix_read key=project.static.summary", "project", "current", now),
		catalogEntry("project.change_delta", "project_change", projectChangeFreshness(obs), "cheap", "Deterministic Shadow Project change receipt for the current engineering state; it does not establish an acoustic consequence.", "mix_read key=project.change_delta", "project", "current", now),
		catalogEntry("project.tracks.summary", "static", sourceFreshness(obs, "project_context"), "cheap", trackSummaryText(req.ProjectState), "mix_read key=project.tracks.summary", "project", "current", now),
		catalogEntry("project.acoustic.tracks", "fast_acoustic", sourceFreshness(obs, "track_waveform_envelopes"), "cheap", "Per-track lightweight waveform packages for visible audio tracks.", "mix_read key=project.acoustic.tracks", "project", "current", now),
		catalogEntry("project.relationship_inputs", "derived", sourceFreshness(obs, "project_context"), "cheap", "Readiness summary for project-level relationship derivation.", "mix_read key=project.relationship_inputs", "project", "current", now),
		catalogEntry("project.frequency_relationship_inputs", "derived", sourceFreshness(obs, "band_energy"), "medium", "Compact project band-occupancy inputs for MOM frequency-relationship observation.", "mix_read key=project.frequency_relationship_inputs", "project", "current", now),
		catalogEntry("project.masking_relationship_inputs", "derived", sourceFreshness(obs, "masking_analysis"), "expensive", "Directional same-window, same-tap masking-risk measurements. Candidate evidence only.", "mix_read key=project.masking_relationship_inputs", "project", "current", now),
		catalogEntry("project.rankings.loudness", "derived", sourceFreshness(obs, "project_context"), "cheap", "Track ranking by acoustic RMS/loudness when available.", "mix_read key=project.rankings.loudness", "project", "current", now),
		catalogEntry("project.rankings.level", "derived", sourceFreshness(obs, "project_context"), "cheap", "Track ranking by live/shadow level_db when available.", "mix_read key=project.rankings.level", "project", "current", now),
		catalogEntry("project.rankings.peak", "derived", sourceFreshness(obs, "project_context"), "cheap", "Track ranking by peak_dbfs when available.", "mix_read key=project.rankings.peak", "project", "current", now),
		catalogEntry("project.risks.peak", "derived", sourceFreshness(obs, "project_context"), "cheap", "Tracks sorted by highest known peak with risk labels.", "mix_read key=project.risks.peak", "project", "current", now),
		catalogEntry("project.risks.headroom", "derived", sourceFreshness(obs, "project_context"), "cheap", "Tracks sorted by smallest known headroom.", "mix_read key=project.risks.headroom", "project", "current", now),
		catalogEntry("project.attention.first", "derived", sourceFreshness(obs, "project_context"), "cheap", "Likely first track to inspect based on headroom risk, peak, focus, and loudness.", "mix_read key=project.attention.first", "project", "current", now),
		catalogEntry("project.limitations", "derived", "fresh", "cheap", "Known missing, partial, or v1-limited project observation capabilities; use this before interpreting silence as absence.", "mix_read key=project.limitations", "project", "current", now),
		catalogEntry("track."+targetID+".static.identity", "static", "fresh", "cheap", "Target identity, listen scope, and mix object metadata.", "mix_read key=track."+targetID+".static.identity", targetKind, targetID, now),
		catalogEntry("track."+targetID+".fast.levels", "fast_realtime", sourceFreshness(obs, "waveform_envelope"), "cheap", waveformSummaryText(obs), "mix_read key=track."+targetID+".fast.levels", targetKind, targetID, now),
		catalogEntry("track."+targetID+".slow.time_energy.summary", "slow_acoustic", sourceFreshness(obs, "time_energy"), "medium", timeEnergySummaryText(obs), "mix_read key=track."+targetID+".slow.time_energy.summary", targetKind, targetID, now),
		catalogEntry("track."+targetID+".slow.band_energy.summary", "slow_acoustic", sourceFreshness(obs, "band_energy"), "medium", "Band energy summary for sub/bass/low_mid/mid/presence/air when available.", "mix_read key=track."+targetID+".slow.band_energy.summary", targetKind, targetID, now),
		catalogEntry("track."+targetID+".slow.stereo.summary", "slow_acoustic", sourceFreshness(obs, "stereo_correlation"), "medium", "Stereo balance and correlation summary when available.", "mix_read key=track."+targetID+".slow.stereo.summary", targetKind, targetID, now),
		catalogEntry("track."+targetID+".realtime.band_energy.summary", "l2_realtime", sourceFreshness(obs, "realtime_band_energy"), "cheap", "Playback-captured realtime band energy when live telemetry has provided it.", "mix_read key=track."+targetID+".realtime.band_energy.summary", targetKind, targetID, now),
		catalogEntry("track."+targetID+".realtime.stereo.summary", "l2_realtime", sourceFreshness(obs, "realtime_stereo_relation"), "cheap", "Playback-captured realtime stereo balance/correlation when live telemetry has provided it.", "mix_read key=track."+targetID+".realtime.stereo.summary", targetKind, targetID, now),
		catalogEntry("track."+targetID+".raw.time_energy.range", "slow_acoustic", sourceFreshness(obs, "time_energy"), "medium", "Range-readable time energy rows; use range_sec for long audio.", "mix_read key=track."+targetID+".raw.time_energy.range range_sec=[start,end]", targetKind, targetID, now),
		catalogEntry("observation.before_after.latest", "derived", sourceFreshness(obs, "before_after_delta"), "cheap", "Delta against the previous observation in the same mix session.", "mix_derive type=before_after", targetKind, targetID, now),
		catalogEntry("observation.ab_result.latest", "derived", sourceFreshness(obs, "ab_result"), "cheap", "Same-tap L2 Render Probe AB result with compact deltas and evidence refs.", "mix_derive type=ab_result", targetKind, targetID, now),
		catalogEntry("relationships.available", "derived", "fresh", "cheap", "On-demand relationship package types: before_after, ab_result, rank_tracks, focus_vs_project, a_vs_b, group_overlap.", "mix_derive type=...", "relationship", "available", now),
	}
	for i := range entries {
		switch entries[i].Key {
		case "relationships.available":
			entries[i].AvailableOps = []string{"before_after", "ab_result", "rank_tracks", "focus_vs_project", "a_vs_b", "group_overlap"}
		case "track." + targetID + ".raw.time_energy.range":
			entries[i].AvailableOps = []string{"read_range", "read_summary"}
		default:
			entries[i].AvailableOps = []string{"read"}
		}
	}
	return Catalog{
		SchemaVersion: ObservationCatalogVersion,
		ObservationID: obs.ObservationID,
		GeneratedAt:   now,
		Entries:       entries,
	}
}

func (s Store) Read(req ReadRequest) (map[string]any, error) {
	obs, path, err := s.findObservation(req.MixSessionID, req.ObservationID)
	if err != nil {
		return nil, err
	}
	if req.MaxItems <= 0 {
		req.MaxItems = 48
	}
	if len(req.Keys) == 0 {
		req.Keys = []string{"observation.digest", "observation.catalog"}
	}
	items := map[string]any{}
	var evidenceSnapshot map[string]any
	for _, key := range req.Keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if strings.Contains(key, ".raw.") && evidenceSnapshot == nil {
			evidenceSnapshot = lazyFeatureSnapshot(obs)
		}
		value, ok := readObservationKey(obs, key, req, evidenceSnapshot)
		if !ok {
			items[key] = map[string]any{"status": "missing", "reason": "catalog_key_not_found"}
			continue
		}
		items[key] = value
	}
	return map[string]any{
		"status":           "ok",
		"observation_id":   obs.ObservationID,
		"mix_session_id":   obs.MixSessionID,
		"observation_path": path,
		"items":            items,
	}, nil
}

func (s Store) Derive(req DeriveRequest) (map[string]any, error) {
	obs, path, err := s.findObservation(req.MixSessionID, req.ObservationID)
	if err != nil {
		return nil, err
	}
	deriveType := strings.ToLower(strings.TrimSpace(req.Type))
	if deriveType == "" {
		deriveType = "before_after"
	}
	if req.MaxItems <= 0 {
		req.MaxItems = 12
	}
	var pkg map[string]any
	switch deriveType {
	case "before_after":
		pkg = deriveBeforeAfter(obs)
	case "ab_result":
		pkg = deriveABResult(obs)
	case "rank_tracks":
		pkg = deriveRankTracks(obs, req)
	case "focus_vs_project", "a_vs_b", "group_overlap":
		pkg = derivePartialRelationship(obs, req, deriveType)
	default:
		pkg = map[string]any{"status": "error", "reason": "unsupported_derive_type", "type": deriveType}
	}
	pkg["schema_version"] = RelationshipPackageVersion
	pkg["relationship_id"] = "rel_" + strings.ReplaceAll(obs.ObservationID, "obs_", "") + "_" + safePathName(deriveType)
	pkg["observation_id"] = obs.ObservationID
	return map[string]any{
		"status":           firstNonEmpty(cleanAnyString(pkg["status"]), "ok"),
		"observation_id":   obs.ObservationID,
		"mix_session_id":   obs.MixSessionID,
		"observation_path": path,
		"relationship":     pkg,
	}, nil
}

func (s Store) findObservation(sessionID, observationID string) (ObservationPacket, string, error) {
	sessionID = strings.TrimSpace(sessionID)
	observationID = strings.TrimSpace(observationID)
	if sessionID != "" {
		sessionDir := filepath.Join(s.Root, safePathName(sessionID))
		if observationID == "" {
			if obs, ok := readLatestObservation(sessionDir); ok {
				return obs, latestObservationPath(sessionDir), nil
			}
		}
		path := filepath.Join(sessionDir, "observations", safePathName(observationID)+".json")
		if obs, err := readObservationFile(path); err == nil {
			return obs, path, nil
		}
		if root := canonicalObservationRoot(s.Root); root != "" {
			path = filepath.Join(root, safePathName(observationID)+".json")
			if obs, err := readObservationFile(path); err == nil && obs.MixSessionID == sessionID {
				return obs, path, nil
			}
		}
	}
	if observationID != "" {
		if root := canonicalObservationRoot(s.Root); root != "" {
			path := filepath.Join(root, safePathName(observationID)+".json")
			if obs, err := readObservationFile(path); err == nil {
				return obs, path, nil
			}
		}
		var foundObs ObservationPacket
		var foundPath string
		err := filepath.WalkDir(s.Root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			if filepath.Base(path) != safePathName(observationID)+".json" {
				return nil
			}
			obs, readErr := readObservationFile(path)
			if readErr != nil {
				return nil
			}
			foundObs, foundPath = obs, path
			return filepath.SkipAll
		})
		if err != nil {
			return ObservationPacket{}, "", err
		}
		if foundPath != "" {
			return foundObs, foundPath, nil
		}
	}
	return ObservationPacket{}, "", fmt.Errorf("mix observation not found")
}

func canonicalObservationRoot(storeRoot string) string {
	root := filepath.Clean(strings.TrimSpace(storeRoot))
	if root == "." || !strings.EqualFold(filepath.Base(root), "sessions") {
		return ""
	}
	mixboardRoot := filepath.Dir(root)
	if !strings.EqualFold(filepath.Base(mixboardRoot), "mixboard") {
		return ""
	}
	return filepath.Join(filepath.Dir(mixboardRoot), "observations")
}

func readObservationFile(path string) (ObservationPacket, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ObservationPacket{}, err
	}
	var obs ObservationPacket
	if err := json.Unmarshal(data, &obs); err != nil {
		return ObservationPacket{}, err
	}
	return obs, nil
}

func latestObservationPath(sessionDir string) string {
	data, err := os.ReadFile(filepath.Join(sessionDir, "current.json"))
	if err != nil {
		return ""
	}
	var board Board
	if json.Unmarshal(data, &board) != nil {
		return ""
	}
	return board.LatestObservationPath
}

func readObservationKey(obs ObservationPacket, key string, req ReadRequest, evidenceSnapshots ...map[string]any) (any, bool) {
	var evidenceSnapshot map[string]any
	if len(evidenceSnapshots) > 0 {
		evidenceSnapshot = evidenceSnapshots[0]
	}
	targetID := firstNonEmpty(obs.TargetRef.ID, "target")
	metrics, _ := obs.MixPackage["current_metrics"].(map[string]any)
	realtimeMetrics, _ := obs.MixPackage["realtime_metrics"].(map[string]any)
	switch key {
	case "observation.digest":
		return obs.Digest, true
	case "observation.binding":
		return observationBinding(obs), true
	case "observation.com_projection":
		if obs.COMProjection == nil {
			return map[string]any{"status": "missing", "reason": "com_projection_unavailable"}, true
		}
		return obs.COMProjection, true
	case "observation.dom_projection":
		if obs.DOMProjection == nil {
			return map[string]any{"status": "missing", "reason": "dom_projection_unavailable"}, true
		}
		return obs.DOMProjection, true
	case "observation.mom_projection":
		if obs.MOMProjection == nil {
			return map[string]any{"status": "missing", "reason": "mom_projection_unavailable"}, true
		}
		return obs.MOMProjection, true
	case "observation.tim_projection":
		if obs.TIMProjection == nil {
			return map[string]any{"status": "missing", "reason": "tim_projection_unavailable"}, true
		}
		return obs.TIMProjection, true
	case "observation.fxm_projection":
		if obs.FXMProjection == nil {
			return map[string]any{"status": "missing", "reason": "fxm_projection_unavailable"}, true
		}
		return obs.FXMProjection, true
	case "observation.catalog":
		return obs.Catalog, true
	case "project.static.summary":
		return compactKeys(obs.EnvironmentPackage, []string{"schema_version", "status", "target_ref", "listen_scope", "time_ruler", "source_capabilities"}), true
	case "project.change_delta":
		return projectChangeDelta(obs), true
	case "project.tracks.summary":
		return compactProjectTracks(obs.ProjectPackage, req.MaxItems), true
	case "project.acoustic.tracks":
		return compactProjectAcousticTracks(obs.ProjectPackage, req.MaxItems), true
	case "project.relationship_inputs":
		return obs.ProjectPackage["relationship_inputs"], true
	case "project.frequency_relationship_inputs":
		return obs.ProjectPackage["frequency_relationship_inputs"], true
	case "project.masking_relationship_inputs":
		return obs.ProjectPackage["masking_relationship_inputs"], true
	case "project.rankings.loudness":
		return map[string]any{"status": projectRankingStatus(obs.ProjectPackage, "loudness_ranking"), "rows": capRows(mapRowsAny(obs.ProjectPackage["loudness_ranking"]), req.MaxItems)}, true
	case "project.rankings.level":
		return map[string]any{"status": projectRankingStatus(obs.ProjectPackage, "level_ranking"), "rows": capRows(mapRowsAny(obs.ProjectPackage["level_ranking"]), req.MaxItems)}, true
	case "project.rankings.peak":
		return map[string]any{"status": projectRankingStatus(obs.ProjectPackage, "peak_ranking"), "rows": capRows(mapRowsAny(obs.ProjectPackage["peak_ranking"]), req.MaxItems)}, true
	case "project.risks.peak":
		return map[string]any{"status": projectRankingStatus(obs.ProjectPackage, "peak_risk_ranking"), "rows": capRows(mapRowsAny(obs.ProjectPackage["peak_risk_ranking"]), req.MaxItems)}, true
	case "project.risks.headroom":
		return map[string]any{"status": projectRankingStatus(obs.ProjectPackage, "headroom_risk"), "rows": capRows(mapRowsAny(obs.ProjectPackage["headroom_risk"]), req.MaxItems)}, true
	case "project.attention.first":
		return obs.ProjectPackage["likely_first_attention_target"], true
	case "project.limitations":
		return map[string]any{
			"status":      firstNonEmpty(cleanAnyString(obs.ProjectPackage["status"]), "partial"),
			"limitations": obs.ProjectPackage["limitations"],
			"available_detail": map[string]any{
				"project_context":                 sourceStatus(obs, "project_context"),
				"project_track_waveforms":         sourceStatus(obs, "track_waveform_envelopes"),
				"target_waveform_envelope":        sourceStatus(obs, "waveform_envelope"),
				"target_time_energy":              sourceStatus(obs, "time_energy"),
				"target_band_energy":              sourceStatus(obs, "band_energy"),
				"target_stereo_relation":          sourceStatus(obs, "stereo_correlation"),
				"target_realtime_band_energy":     sourceStatus(obs, "realtime_band_energy"),
				"target_realtime_stereo_relation": sourceStatus(obs, "realtime_stereo_relation"),
				"full_project_acoustic_render":    sourceStatus(obs, "track_waveform_envelopes"),
			},
		}, true
	case "track." + targetID + ".static.identity":
		return map[string]any{
			"target_ref":     obs.TargetRef,
			"track_identity": targetTrackIdentity(obs),
			"mix_objects":    obs.MixObjects,
			"listen_scope":   obs.ListenScope,
		}, true
	case "track." + targetID + ".fast.levels":
		return metrics["waveform"], true
	case "track." + targetID + ".slow.time_energy.summary":
		rows := mapRowsAny(metrics["time_energy"])
		status := sourceStatus(obs, "time_energy")
		if len(rows) == 0 {
			if evidenceRows := evidenceTimeEnergyRows(evidenceSnapshot); len(evidenceRows) > 0 {
				rows, status = evidenceRows, "ready"
			}
		}
		return map[string]any{"status": status, "rows": capRows(rows, req.MaxItems)}, true
	case "track." + targetID + ".raw.time_energy.range":
		rows := mapRowsAny(metrics["time_energy"])
		if evidenceRows := evidenceTimeEnergyRows(evidenceSnapshot); len(evidenceRows) > 0 {
			rows = evidenceRows
		}
		return readTimeEnergyRange(rows, req), true
	case "track." + targetID + ".slow.band_energy.summary":
		return metrics["band_energy"], true
	case "track." + targetID + ".slow.stereo.summary":
		return metrics["stereo_relation"], true
	case "track." + targetID + ".realtime.band_energy.summary":
		return realtimeMetrics["band_energy"], true
	case "track." + targetID + ".realtime.stereo.summary":
		return realtimeMetrics["stereo_relation"], true
	case "observation.before_after.latest":
		return deriveBeforeAfter(obs), true
	case "observation.ab_result.latest":
		return deriveABResult(obs), true
	case "relationships.available":
		return map[string]any{"status": "ready", "types": []string{"before_after", "ab_result", "rank_tracks", "focus_vs_project", "a_vs_b", "group_overlap"}}, true
	default:
		return nil, false
	}
}

func lazyFeatureSnapshot(obs ObservationPacket) map[string]any {
	roots, ok := projectstore.Current()
	if !ok || obs.ProjectUUID == "" || !strings.EqualFold(projectstore.SafeName(obs.ProjectUUID), roots.ProjectUUID) {
		return nil
	}
	for _, ref := range obs.EvidenceRefs {
		if !strings.HasPrefix(strings.TrimSpace(ref), "evidence://") {
			continue
		}
		blob, err := projectstore.GetEvidence(roots, ref)
		if err != nil || blob.Kind != "feature_snapshot" {
			continue
		}
		if snapshot, ok := blob.Content.(map[string]any); ok {
			return snapshot
		}
		data, _ := json.Marshal(blob.Content)
		var snapshot map[string]any
		if json.Unmarshal(data, &snapshot) == nil {
			return snapshot
		}
	}
	return nil
}

func evidenceTimeEnergyRows(snapshot map[string]any) []map[string]any {
	for _, container := range []map[string]any{snapshot, mapValue(snapshot["waveform_envelope"])} {
		for _, key := range []string{"time_segments", "time_energy", "time_energy_rows"} {
			if rows := mapRowsAny(container[key]); len(rows) > 0 {
				return rows
			}
		}
	}
	return nil
}

func fxmProjectionFreshness(obs ObservationPacket) string {
	if obs.FXMProjection == nil {
		return "missing"
	}
	switch strings.ToLower(strings.TrimSpace(obs.FXMProjection.Status)) {
	case "ready":
		return "fresh"
	case "partial":
		return "partial"
	case "stale":
		return "stale"
	default:
		return "missing"
	}
}

func comProjectionFreshness(obs ObservationPacket) string {
	if obs.COMProjection == nil {
		return "missing"
	}
	switch strings.ToLower(strings.TrimSpace(obs.COMProjection.Status)) {
	case com.StatusReady:
		return "fresh"
	case com.StatusPartial, com.StatusApproximate:
		return "partial"
	case com.StatusStale:
		return "stale"
	case com.StatusSuspect:
		return "suspect"
	default:
		return "missing"
	}
}

func domProjectionFreshness(obs ObservationPacket) string {
	if obs.DOMProjection == nil {
		return "missing"
	}
	switch strings.ToLower(strings.TrimSpace(obs.DOMProjection.Status)) {
	case dom.StatusReady:
		return "fresh"
	case dom.StatusPartial:
		return "partial"
	case dom.StatusStale:
		return "stale"
	case dom.StatusSuspect:
		return "suspect"
	default:
		return "missing"
	}
}

func projectChangeFreshness(obs ObservationPacket) string {
	if len(obs.ProjectChange) == 0 {
		return "missing"
	}
	if strings.EqualFold(cleanAnyString(obs.ProjectChange["freshness"]), "current_snapshot") {
		return "fresh"
	}
	return "partial"
}

func readTimeEnergyRange(rows []map[string]any, req ReadRequest) map[string]any {
	out := []map[string]any{}
	for _, row := range rows {
		start := numberFromMap(row, "start_seconds")
		end := numberFromMap(row, "end_seconds")
		if req.RangeEnd > req.RangeStart && (end < req.RangeStart || start > req.RangeEnd) {
			continue
		}
		out = append(out, row)
	}
	truncated := false
	if req.MaxItems > 0 && len(out) > req.MaxItems {
		out = out[:req.MaxItems]
		truncated = true
	}
	return map[string]any{
		"status":        "ready",
		"range_start":   req.RangeStart,
		"range_end":     req.RangeEnd,
		"rows":          out,
		"truncated":     truncated,
		"total_rows":    len(rows),
		"returned_rows": len(out),
	}
}

func deriveBeforeAfter(obs ObservationPacket) map[string]any {
	metrics, _ := obs.MixPackage["current_metrics"].(map[string]any)
	delta, _ := metrics["before_after_delta"].(map[string]any)
	if len(delta) == 0 {
		return map[string]any{"status": "missing", "type": "before_after", "reason": "before_after_delta_missing"}
	}
	return map[string]any{
		"status":        featureStatus(delta),
		"type":          "before_after",
		"summary":       delta["summary"],
		"facts":         delta,
		"evidence_refs": []string{"observation.before_after.latest"},
	}
}

func deriveABResult(obs ObservationPacket) map[string]any {
	metrics, _ := obs.MixPackage["current_metrics"].(map[string]any)
	result, _ := metrics["ab_result"].(map[string]any)
	if len(result) == 0 {
		return map[string]any{"status": "missing", "type": "ab_result", "reason": "ab_result_missing"}
	}
	return map[string]any{
		"status":        featureStatus(result),
		"type":          "ab_result",
		"summary":       result["summary"],
		"facts":         result,
		"evidence_refs": []string{"observation.ab_result.latest"},
	}
}

func deriveRankTracks(obs ObservationPacket, req DeriveRequest) map[string]any {
	loudnessRows := capRows(mapRowsAny(obs.ProjectPackage["loudness_ranking"]), req.MaxItems)
	levelRows := capRows(mapRowsAny(obs.ProjectPackage["level_ranking"]), req.MaxItems)
	peakRows := capRows(mapRowsAny(obs.ProjectPackage["peak_ranking"]), req.MaxItems)
	peakRiskRows := capRows(mapRowsAny(obs.ProjectPackage["peak_risk_ranking"]), req.MaxItems)
	headroomRows := capRows(mapRowsAny(obs.ProjectPackage["headroom_risk"]), req.MaxItems)
	if len(loudnessRows) > 0 || len(levelRows) > 0 || len(peakRows) > 0 || len(peakRiskRows) > 0 || len(headroomRows) > 0 {
		return map[string]any{
			"status": "ready",
			"type":   "rank_tracks",
			"rankings": map[string]any{
				"loudness":      loudnessRows,
				"level":         levelRows,
				"peak":          peakRows,
				"peak_risk":     peakRiskRows,
				"headroom_risk": headroomRows,
			},
			"relationship_inputs": obs.ProjectPackage["relationship_inputs"],
			"limitations":         obs.ProjectPackage["limitations"],
			"evidence_refs":       []string{"project.rankings.loudness", "project.rankings.level", "project.rankings.peak", "project.risks.peak", "project.risks.headroom"},
		}
	}
	waveform, _ := mixCurrentMetrics(obs)["waveform"].(map[string]any)
	row := map[string]any{
		"rank":        1,
		"target_ref":  obs.TargetRef,
		"peak_dbfs":   waveform["peak_dbfs"],
		"rms_dbfs":    waveform["rms_dbfs"],
		"headroom_db": waveform["headroom_db"],
	}
	return map[string]any{
		"status":        "partial",
		"type":          "rank_tracks",
		"reason":        "project track level/peak fields are missing; falling back to focused object waveform only",
		"rankings":      []map[string]any{row},
		"evidence_refs": []string{"track." + firstNonEmpty(obs.TargetRef.ID, "target") + ".fast.levels"},
	}
}

func derivePartialRelationship(obs ObservationPacket, req DeriveRequest, deriveType string) map[string]any {
	projectTracks := compactProjectTracks(obs.ProjectPackage, req.MaxItems)
	focusTrack := focusTrackSummary(projectTracks, req.Focus, req.A, req.B)
	peerTrack := peerTrackSummary(projectTracks, focusTrack, req.A, req.B)
	levelRanking := capRows(mapRowsAny(obs.ProjectPackage["level_ranking"]), req.MaxItems)
	loudnessRanking := capRows(mapRowsAny(obs.ProjectPackage["loudness_ranking"]), req.MaxItems)
	peakRanking := capRows(mapRowsAny(obs.ProjectPackage["peak_ranking"]), req.MaxItems)
	peakRiskRanking := capRows(mapRowsAny(obs.ProjectPackage["peak_risk_ranking"]), req.MaxItems)
	headroomRisk := capRows(mapRowsAny(obs.ProjectPackage["headroom_risk"]), req.MaxItems)
	if len(levelRanking) == 0 {
		levelRanking = loudnessRanking
	}
	return map[string]any{
		"status":            relationshipStatusForFocus(projectTracks, focusTrack),
		"type":              deriveType,
		"inputs":            map[string]any{"a": req.A, "b": req.B, "focus": req.Focus},
		"dimensions":        req.Dimensions,
		"summary":           relationshipSummaryText(deriveType, focusTrack, peerTrack),
		"derived_judgement": relationshipJudgement(deriveType, projectTracks, focusTrack, peerTrack, levelRanking, peakRanking, headroomRisk),
		"facts": map[string]any{
			"focus_track":    focusTrack,
			"peer_track":     peerTrack,
			"project_tracks": projectTracks,
			"rankings": map[string]any{
				"loudness":      loudnessRanking,
				"level":         levelRanking,
				"peak":          peakRanking,
				"peak_risk":     peakRiskRanking,
				"headroom_risk": headroomRisk,
			},
		},
		"limitations":   appendStringIfMissing(obs.ProjectPackage["limitations"], "project_minus_focus_acoustic_render_not_available"),
		"evidence_refs": relationshipEvidenceRefs(deriveType, focusTrack, peerTrack),
		"confidence":    relationshipConfidence(focusTrack, peerTrack),
	}
}

func relationshipJudgement(deriveType string, project, focus, peer map[string]any, levelRanking, peakRanking, headroomRisk []map[string]any) map[string]any {
	out := map[string]any{
		"status": "partial",
	}
	focusID := cleanAnyString(focus["track_id"])
	if focusID == "" {
		out["reason"] = "focus_track_not_identified"
		return out
	}
	out["status"] = "ready"
	if rank, total, ok := rankingPosition(levelRanking, focusID); ok {
		out["focus_level_rank"] = rank
		out["focus_level_percentile"] = rankPercentile(rank, total)
	}
	if rank, total, ok := rankingPosition(peakRanking, focusID); ok {
		out["focus_peak_rank"] = rank
		out["focus_peak_percentile"] = rankPercentile(rank, total)
	}
	if loudest := loudestNonFocus(levelRanking, focusID); len(loudest) > 0 {
		out["loudest_non_focus"] = loudest
		if focusLevel, okA := numberField(focus, "level_db"); okA {
			if peerLevel, okB := numberField(loudest, "value"); okB {
				out["focus_vs_loudest_level_delta_db"] = round3(focusLevel - peerLevel)
			}
		}
	}
	if risk := matchingRankingRow(headroomRisk, focusID); len(risk) > 0 {
		out["focus_headroom_risk"] = risk
	}
	if deriveType == "group_overlap" {
		out["candidate_group_tracks"] = lowFrequencyCandidateTracks(mapRowsAny(project["tracks"]))
		out["dimensions"] = []string{"role_guess", "level_rank", "peak_rank", "headroom", "time_overlap"}
	}
	out["first_attention_candidate"] = firstAttentionCandidate(focus, levelRanking, headroomRisk)
	return out
}

func rankingPosition(rows []map[string]any, trackID string) (int, int, bool) {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return 0, len(rows), false
	}
	for i, row := range rows {
		if strings.EqualFold(cleanAnyString(row["track_id"]), trackID) {
			if rank := int(numberFromMap(row, "rank")); rank > 0 {
				return rank, len(rows), true
			}
			return i + 1, len(rows), true
		}
	}
	return 0, len(rows), false
}

func rankPercentile(rank, total int) float64 {
	if rank <= 0 || total <= 0 {
		return 0
	}
	if total == 1 {
		return 1
	}
	return round3(1 - float64(rank-1)/float64(total-1))
}

func loudestNonFocus(rows []map[string]any, focusID string) map[string]any {
	for _, row := range rows {
		if !strings.EqualFold(cleanAnyString(row["track_id"]), focusID) {
			return row
		}
	}
	return nil
}

func matchingRankingRow(rows []map[string]any, trackID string) map[string]any {
	for _, row := range rows {
		if strings.EqualFold(cleanAnyString(row["track_id"]), trackID) {
			return row
		}
	}
	return nil
}

func lowFrequencyCandidateTracks(tracks []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, track := range tracks {
		role := strings.ToLower(cleanAnyString(track["role_guess"]))
		name := strings.ToLower(cleanAnyString(track["name"]))
		if role == "bass" || role == "kick" ||
			strings.Contains(name, "bass") || strings.Contains(name, "kick") || strings.Contains(name, "808") || strings.Contains(name, "sub") ||
			strings.Contains(name, "\u8d1d\u65af") || strings.Contains(name, "\u4f4e\u97f3") || strings.Contains(name, "\u4f4e\u9891") ||
			strings.Contains(name, "\u5e95\u9f13") || strings.Contains(name, "\u5927\u9f13") {
			out = append(out, compactKeys(track, []string{"track_id", "name", "role_guess", "level_db", "peak_dbfs", "headroom_db", "focused"}))
		}
	}
	return out
}

func firstAttentionCandidate(focus map[string]any, levelRanking, headroomRisk []map[string]any) map[string]any {
	if risk := firstHighHeadroomRisk(headroomRisk); len(risk) > 0 {
		return map[string]any{"reason": "smallest_headroom", "track": risk}
	}
	if len(focus) > 0 && cleanAnyString(focus["track_id"]) != "" {
		return map[string]any{"reason": "user_focus", "track": compactKeys(focus, []string{"track_id", "name", "role_guess", "level_db", "peak_dbfs", "headroom_db"})}
	}
	if len(levelRanking) > 0 {
		return map[string]any{"reason": "loudest_known_track", "track": levelRanking[0]}
	}
	return map[string]any{"status": "missing"}
}

func firstHighHeadroomRisk(rows []map[string]any) map[string]any {
	for _, row := range rows {
		risk := strings.ToLower(cleanAnyString(row["risk"]))
		if risk == "critical" || risk == "high" {
			return row
		}
	}
	return nil
}

func compactProjectTracks(project map[string]any, max int) map[string]any {
	tracks := capRows(mapRowsAny(project["tracks"]), max)
	return map[string]any{
		"schema_version":              project["schema_version"],
		"status":                      project["status"],
		"summary":                     project["summary"],
		"track_count":                 project["track_count"],
		"active_track_count":          project["active_track_count"],
		"acoustic_track_count":        project["acoustic_track_count"],
		"active_acoustic_track_count": project["active_acoustic_track_count"],
		"tracks":                      tracks,
		"relationship_inputs":         project["relationship_inputs"],
		"limitations":                 project["limitations"],
	}
}

func compactProjectAcousticTracks(project map[string]any, max int) map[string]any {
	rows := []map[string]any{}
	for _, track := range mapRowsAny(project["tracks"]) {
		acoustic, _ := track["acoustic"].(map[string]any)
		row := map[string]any{
			"track_id":   track["track_id"],
			"name":       track["name"],
			"role_guess": track["role_guess"],
			"focused":    track["focused"],
			"acoustic":   compactKeys(acoustic, []string{"status", "track_id", "clip_id", "primary_clip_id", "primary_clip_name", "source", "source_path", "file_path", "rms_dbfs", "peak_dbfs", "headroom_db", "crest_db", "time_energy_status", "reason", "updated_at"}),
		}
		rows = append(rows, row)
	}
	return map[string]any{
		"schema_version":              project["schema_version"],
		"status":                      sourceLikeStatusForRows(rows),
		"track_count":                 project["track_count"],
		"acoustic_track_count":        project["acoustic_track_count"],
		"active_acoustic_track_count": project["active_acoustic_track_count"],
		"tracks":                      capRows(rows, max),
	}
}

func sourceLikeStatusForRows(rows []map[string]any) string {
	if len(rows) == 0 {
		return "missing"
	}
	ready := 0
	partial := 0
	requested := 0
	blocked := 0
	for _, row := range rows {
		acoustic, _ := row["acoustic"].(map[string]any)
		switch featureStatus(acoustic) {
		case "ready":
			ready++
		case "partial":
			partial++
		case "requested":
			requested++
		case "blocked", "unavailable":
			blocked++
		}
	}
	switch {
	case ready == len(rows):
		return "ready"
	case ready > 0 || partial > 0:
		return "partial"
	case requested > 0:
		return "requested"
	case blocked > 0:
		return "blocked"
	default:
		return "missing"
	}
}

func targetTrackIdentity(obs ObservationPacket) map[string]any {
	targetID := strings.TrimSpace(obs.TargetRef.ID)
	out := map[string]any{
		"kind":       obs.TargetRef.Kind,
		"track_id":   targetID,
		"name":       obs.TargetRef.Label,
		"track_name": obs.TargetRef.Label,
		"user_label": obs.TargetRef.Label,
		"source":     obs.TargetRef.Source,
		"confidence": obs.TargetRef.Confidence,
	}
	for _, track := range mapRowsAny(obs.ProjectPackage["tracks"]) {
		if targetID == "" || !strings.EqualFold(cleanAnyString(track["track_id"]), targetID) {
			continue
		}
		for _, key := range []string{"track_id", "name", "track_name", "user_label", "user_track_index", "track_type", "role_guess", "active_state", "selected", "focused"} {
			if value, ok := track[key]; ok && value != nil {
				out[key] = value
			}
		}
		break
	}
	return out
}

func observationBinding(obs ObservationPacket) map[string]any {
	project := mapValue(obs.ProjectPackage)
	out := map[string]any{
		"schema_version": "mix_observation_binding.v1",
		"observation_id": obs.ObservationID,
		"mix_session_id": obs.MixSessionID,
		"status":         obs.Status,
		"created_at":     obs.CreatedAt,
		"target_ref":     obs.TargetRef,
		"listen_scope":   obs.ListenScope,
		"project_binding": map[string]any{
			"project_uuid":       firstNonEmpty(cleanAnyString(project["project_uuid"]), obs.ProjectUUID),
			"project_epoch":      cleanAnyString(project["project_epoch"]),
			"project_revision":   cleanAnyString(project["project_revision"]),
			"project_state_hash": cleanAnyString(project["project_state_hash"]),
		},
		"evidence_refs": append([]string(nil), obs.EvidenceRefs...),
	}
	if len(obs.ProjectChange) > 0 {
		out["project_change_ref"] = compactKeys(obs.ProjectChange, []string{"schema_version", "change_id", "source", "authoritative", "freshness", "from_state_epoch", "to_state_epoch"})
	}
	return out
}

func projectChangeDelta(obs ObservationPacket) map[string]any {
	if len(obs.ProjectChange) == 0 {
		return map[string]any{"status": "missing", "reason": "project_change_monitor_unavailable"}
	}
	out := copyAnyMap(obs.ProjectChange)
	if out["status"] == nil {
		out["status"] = "partial"
		if strings.EqualFold(cleanAnyString(out["freshness"]), "current_snapshot") {
			out["status"] = "ready"
		}
	}
	out["observation_id"] = obs.ObservationID
	out["limitations"] = []string{"Engineering delta does not establish an acoustic outcome; request a current acoustic view for sound-level evaluation."}
	return out
}

func projectRankingStatus(project map[string]any, key string) string {
	if len(mapRowsAny(project[key])) > 0 {
		return "ready"
	}
	if status := cleanAnyString(project["status"]); status != "" {
		return "partial"
	}
	return "missing"
}

func focusTrackSummary(project map[string]any, focus, a, b map[string]any) map[string]any {
	tracks := mapRowsAny(project["tracks"])
	if track := findTrackByHint(tracks, focus); len(track) > 0 {
		return track
	}
	if track := findTrackByHint(tracks, a); len(track) > 0 {
		return track
	}
	if track := findTrackByHint(tracks, b); len(track) > 0 {
		return track
	}
	if len(tracks) == 1 {
		return tracks[0]
	}
	return map[string]any{"status": "missing"}
}

func peerTrackSummary(project map[string]any, focus map[string]any, a, b map[string]any) map[string]any {
	tracks := mapRowsAny(project["tracks"])
	if track := findTrackByHint(tracks, a); len(track) > 0 && track["track_id"] != focus["track_id"] {
		return track
	}
	if track := findTrackByHint(tracks, b); len(track) > 0 && track["track_id"] != focus["track_id"] {
		return track
	}
	for _, track := range tracks {
		if track["track_id"] != focus["track_id"] {
			return track
		}
	}
	return map[string]any{"status": "missing"}
}

func findTrackByHint(tracks []map[string]any, hint map[string]any) map[string]any {
	if len(tracks) == 0 || len(hint) == 0 {
		return nil
	}
	id := firstNonEmpty(cleanAnyString(hint["track_id"]), cleanAnyString(hint["id"]))
	name := strings.ToLower(firstNonEmpty(cleanAnyString(hint["track_name"]), cleanAnyString(hint["name"])))
	role := strings.ToLower(firstNonEmpty(cleanAnyString(hint["role"]), cleanAnyString(hint["role_guess"])))
	for _, track := range tracks {
		if id != "" && strings.EqualFold(cleanAnyString(track["track_id"]), id) {
			return track
		}
		if name != "" && strings.Contains(strings.ToLower(cleanAnyString(track["name"])), name) {
			return track
		}
		if role != "" && strings.EqualFold(cleanAnyString(track["role_guess"]), role) {
			return track
		}
	}
	return nil
}

func relationshipStatusForFocus(project, focus map[string]any) string {
	if len(project) == 0 || len(focus) == 0 {
		return "partial"
	}
	if cleanAnyString(focus["track_id"]) == "" {
		return "partial"
	}
	if project["status"] == "ready" {
		return "ready"
	}
	return "partial"
}

func relationshipSummaryText(deriveType string, focus, peer map[string]any) string {
	focusName := firstNonEmpty(cleanAnyString(focus["name"]), cleanAnyString(focus["track_id"]), "focus track")
	peerName := firstNonEmpty(cleanAnyString(peer["name"]), cleanAnyString(peer["track_id"]), "project track")
	switch deriveType {
	case "focus_vs_project":
		return fmt.Sprintf("%s can be judged against project-level ranking and headroom context; use it to decide whether to raise level, trim masking, or widen space.", focusName)
	case "a_vs_b", "group_overlap":
		return fmt.Sprintf("%s compared with %s using project-level track summaries and ranking context.", focusName, peerName)
	default:
		return fmt.Sprintf("%s comparison with %s is available from project summaries.", focusName, peerName)
	}
}

func relationshipEvidenceRefs(deriveType string, focus, peer map[string]any) []string {
	refs := []string{"observation.digest", "project.tracks.summary", "project.rankings.level"}
	if deriveType == "focus_vs_project" {
		return append(refs, "project.risks.headroom")
	}
	if cleanAnyString(focus["track_id"]) != "" {
		refs = append(refs, "track."+safePathName(cleanAnyString(focus["track_id"]))+".fast.levels")
	}
	if cleanAnyString(peer["track_id"]) != "" {
		refs = append(refs, "track."+safePathName(cleanAnyString(peer["track_id"]))+".fast.levels")
	}
	return refs
}

func relationshipConfidence(focus, peer map[string]any) float64 {
	score := 0.35
	if cleanAnyString(focus["track_id"]) != "" {
		score += 0.2
	}
	if _, ok := focus["level_db"]; ok {
		score += 0.15
	}
	if cleanAnyString(peer["track_id"]) != "" {
		score += 0.15
	}
	if _, ok := peer["level_db"]; ok {
		score += 0.15
	}
	if score > 0.95 {
		score = 0.95
	}
	return round3(score)
}

func catalogEntry(key, kind, freshness, cost, summary, readHint, targetKind, targetID, now string) CatalogEntry {
	return CatalogEntry{
		Key:        key,
		Kind:       kind,
		Freshness:  freshness,
		Cost:       cost,
		Summary:    summary,
		ReadHint:   readHint,
		TargetKind: targetKind,
		TargetID:   targetID,
		UpdatedAt:  now,
	}
}

func sourceFreshness(obs ObservationPacket, key string) string {
	switch sourceStatus(obs, key) {
	case "ready":
		return "fresh"
	case "partial":
		return "partial"
	case "requested":
		return "pending"
	case "blocked":
		return "blocked"
	case "missing", "":
		return "missing"
	default:
		return sourceStatus(obs, key)
	}
}

func sourceStatus(obs ObservationPacket, key string) string {
	if key == "project_context" {
		return firstNonEmpty(cleanAnyString(obs.ProjectPackage["status"]), "missing")
	}
	if obs.SourceCapabilities != nil {
		if value := strings.TrimSpace(obs.SourceCapabilities[key]); value != "" {
			return value
		}
	}
	if caps, _ := obs.MixPackage["source_capabilities"].(map[string]string); caps != nil {
		if value := strings.TrimSpace(caps[key]); value != "" {
			return value
		}
	}
	return "missing"
}

func waveformSummaryText(obs ObservationPacket) string {
	waveform, _ := mixCurrentMetrics(obs)["waveform"].(map[string]any)
	if len(waveform) == 0 {
		return "Waveform metrics are missing."
	}
	return fmt.Sprintf("Peak %v dBFS, RMS %v dBFS, headroom %v dB.", waveform["peak_dbfs"], waveform["rms_dbfs"], waveform["headroom_db"])
}

func timeEnergySummaryText(obs ObservationPacket) string {
	rows := mapRowsAny(mixCurrentMetrics(obs)["time_energy"])
	if len(rows) == 0 {
		return "Time energy summary is missing."
	}
	return fmt.Sprintf("%d time-energy rows available; use mix_read range for detail.", len(rows))
}

func projectSummaryText(state map[string]any) string {
	tracks := anySlice(state["tracks"])
	if len(tracks) == 0 {
		return "Project summary available from observation environment; track list missing from shadow."
	}
	return fmt.Sprintf("%d visible track rows in shadow summary.", len(tracks))
}

func trackSummaryText(state map[string]any) string {
	tracks := anySlice(state["tracks"])
	if len(tracks) == 0 {
		return "No track summary rows available."
	}
	names := []string{}
	for _, raw := range tracks {
		row, _ := raw.(map[string]any)
		name := firstNonEmpty(cleanAnyString(row["track_name"]), cleanAnyString(row["name"]), cleanAnyString(row["track_id"]))
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) > 6 {
		names = append(names[:6], fmt.Sprintf("+%d more", len(names)-6))
	}
	return "Visible tracks: " + strings.Join(names, ", ")
}

func compactKeys(row map[string]any, keys []string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := row[key]; ok {
			out[key] = value
		}
	}
	return out
}

func mapRowsAny(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, raw := range rows {
			if row, ok := raw.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}
