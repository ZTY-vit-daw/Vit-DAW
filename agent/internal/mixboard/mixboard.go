package mixboard

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/fxm"
	"vit-daw-agent/internal/mom"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/tim"
)

const (
	ObservationSchemaVersion   = "mix_observation.v2"
	BoardSchemaVersion         = "mixboard.v1"
	ContextPackSchemaVersion   = "mixboard_context_pack.v1"
	ObservationCatalogVersion  = "mix_observation_catalog.v1"
	ObservationDigestVersion   = "mix_observation_digest.v1"
	RelationshipPackageVersion = "mix_relationship_package.v1"
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
	SchemaVersion         string            `json:"schema_version"`
	ProjectUUID           string            `json:"project_uuid,omitempty"`
	ObservationID         string            `json:"observation_id"`
	MixSessionID          string            `json:"mix_session_id"`
	Round                 int               `json:"round"`
	Status                string            `json:"status"`
	TargetRef             TargetRef         `json:"target_ref"`
	MixObjects            []MixObject       `json:"mix_objects"`
	ListenScope           ListenScope       `json:"listen_scope"`
	TimeRuler             TimeRuler         `json:"time_ruler"`
	GlobalSummary         map[string]any    `json:"global_summary"`
	Digest                map[string]any    `json:"digest,omitempty"`
	Catalog               Catalog           `json:"catalog,omitempty"`
	EnvironmentPackage    map[string]any    `json:"environment_package"`
	ProjectPackage        map[string]any    `json:"project_package"`
	MixPackage            map[string]any    `json:"mix_package"`
	DeepPackage           map[string]any    `json:"deep_package"`
	COMProjection         *com.Projection   `json:"com_projection,omitempty"`
	DOMProjection         *dom.Projection   `json:"dom_projection,omitempty"`
	FXMProjection         *fxm.Projection   `json:"fxm_projection,omitempty"`
	MOMProjection         *mom.Projection   `json:"mom_projection,omitempty"`
	TIMProjection         *tim.Projection   `json:"tim_projection,omitempty"`
	SectionCandidates     []map[string]any  `json:"section_candidates"`
	TimelineDigest        []map[string]any  `json:"timeline_digest"`
	Hotspots              []map[string]any  `json:"hotspots"`
	SourceCapabilities    map[string]string `json:"source_capabilities"`
	AcousticPackageStatus map[string]any    `json:"acoustic_package_status,omitempty"`
	ProjectChange         map[string]any    `json:"project_change,omitempty"`
	Notes                 []string          `json:"notes,omitempty"`
	EvidenceRefs          []string          `json:"evidence_refs,omitempty"`
	CreatedAt             string            `json:"created_at"`
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
	LatestChange          map[string]any    `json:"latest_change,omitempty"`
	OpenBlockers          []string          `json:"open_blockers,omitempty"`
	UpdatedAt             string            `json:"updated_at"`
}

type ContextPack struct {
	SchemaVersion     string           `json:"schema_version"`
	MixSessionID      string           `json:"mix_session_id"`
	GeneratedAt       string           `json:"generated_at"`
	SessionHeader     map[string]any   `json:"session_header"`
	LatestObservation map[string]any   `json:"latest_observation"`
	ChangeWindow      []map[string]any `json:"change_window,omitempty"`
	ActiveProblemMap  []map[string]any `json:"active_problem_map"`
	RelevantSections  []map[string]any `json:"relevant_sections"`
	OpenBlockers      []string         `json:"open_blockers,omitempty"`
}

type Request struct {
	MixSessionID  string
	Round         int
	GoalText      string
	TargetRef     TargetRef
	MixObjects    []MixObject
	ListenScope   ListenScope
	ProjectState  map[string]any
	ProjectChange map[string]any
	ChangeWindow  []map[string]any
	Args          map[string]any
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

// FrequencyContext is the compact, non-persisted observation surface used by
// project specialists that only need an existing MOM projection. It keeps
// acoustic materialization and MixBoard persistence out of the CCB read path.
type FrequencyContext struct {
	ObservationID string         `json:"observation_id"`
	Status        string         `json:"status"`
	MOMProjection map[string]any `json:"mom_projection"`
	Digest        map[string]any `json:"digest,omitempty"`
	Catalog       Catalog        `json:"catalog,omitempty"`
	Assembly      map[string]any `json:"assembly,omitempty"`
}

// AssembleFrequencyContext projects the current feature snapshot and project
// state without requesting new analysis and without writing an Observation,
// MixBoard, Context Pack, action, or journal payload.
func AssembleFrequencyContext(projectState map[string]any, mixSessionID, goalText, featureSnapshotPath string) FrequencyContext {
	started := time.Now()
	args := map[string]any{
		"scope":            "full_project",
		"project_context":  true,
		"observation_only": true,
		"disclosure":       "digest_catalog",
		"mom_intent":       mom.IntentProjectFrequencyObservation,
	}
	if strings.TrimSpace(featureSnapshotPath) != "" {
		args["feature_snapshot_path"] = filepath.Clean(featureSnapshotPath)
	}
	featureSnapshot := loadFeatureSnapshot(args)
	recovery := recoverFrequencySnapshotFromPriorObservations(&featureSnapshot, projectState, mixSessionID)
	assembly := hydrateFrequencySnapshotFromAcousticPackages(&featureSnapshot, projectState, args)
	// C1 assembles an existing-evidence-only snapshot. Normalize the hydrated
	// rows before the generic observation projection so a stale primary row from
	// the shared feature snapshot cannot hide current acoustic-package rows.
	// buildObservation repeats this normalization defensively; doing it here
	// also makes the assembly contract observable before MOM projection.
	normalizeProjectFeatureMaterialFreshness(&featureSnapshot, projectState)
	promoteBestL3FeatureRows(&featureSnapshot)
	promoteBestRealtimeFeatureRows(&featureSnapshot)
	preferUniformProjectL3FrequencyEvidence(&featureSnapshot, projectState)
	assembly["normalized_l2_row_count"] = usableFrequencyL2TrackCount(featureSnapshot.L2RenderProbes)
	for key, value := range recovery {
		assembly["prior_observation_"+key] = value
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	observation := buildObservation(Request{
		MixSessionID: strings.TrimSpace(mixSessionID),
		Round:        1,
		GoalText:     strings.TrimSpace(goalText),
		TargetRef:    TargetRef{Kind: "project", ID: "current", Label: "Current project", Source: "c1_ccb_existing_evidence", Confidence: "high"},
		ListenScope:  ListenScope{Time: ListenTimeScope{Mode: "full_song", Source: "scope_full_project"}, Source: ListenSourceScope{Mode: "full_project"}},
		ProjectState: projectState,
		Args:         args,
	}, now, &featureSnapshot)
	projection := map[string]any{}
	if observation.MOMProjection != nil {
		projection = mom.ContextProjection(*observation.MOMProjection)
	}
	frequency := mapValue(projection["frequency_relationship"])
	coverage := mapValue(frequency["coverage"])
	assembly["projected_profile_count"] = len(mapRowsAny(frequency["track_profiles"]))
	assembly["projected_eligible_track_count"] = int(numberFromMap(coverage, "eligible_track_count"))
	assembly["projected_missing_track_count"] = int(numberFromMap(coverage, "missing_track_count"))
	assembly["projected_tap_point"] = cleanAnyString(frequency["tap_point"])
	if int(numberFromMap(assembly, "matched_l2_track_count")) > 0 && int(numberFromMap(coverage, "eligible_track_count")) == 0 {
		assembly["status"] = "projection_mismatch"
		assembly["reason"] = "matched_acoustic_l2_was_not_projected_to_mom"
	}
	return FrequencyContext{
		ObservationID: observation.ObservationID,
		Status:        observation.Status,
		MOMProjection: projection,
		Digest:        observation.Digest,
		Catalog:       observation.Catalog,
		Assembly:      mergeFrequencyAssemblyTiming(assembly, time.Since(started)),
	}
}

// C1's project-frequency diagnosis is a source-file relationship analysis.
// B4 may have left a small set of target post-fader probes in the shared
// feature snapshot; allowing those rows to win per-track would produce a
// mixed tap point and make otherwise complete C1 evidence incomparable. Once
// any frequency-relevant project track has a valid L3 row, the general
// frequency observation stays on the uniform source-file tap: L2 render-probe
// rows never mix into it, and tracks without L3 remain explicit missing
// instead of degrading the comparison to a mixed tap. Target post-fader
// baselines are collected separately after C1 selects exact mutation targets.
// When NO track has L3 evidence at all, L2 rows are kept so a uniform
// post-fader tap can still support same-tap comparison.
func preferUniformProjectL3FrequencyEvidence(snap *featureSnapshot, projectState map[string]any) {
	if snap == nil || len(snap.L2RenderProbes) == 0 {
		return
	}
	hasAnyL3 := false
	for _, row := range snap.BandEnergySummaries {
		status := featureStatus(row)
		if (status == "ready" || status == "partial") && len(mapValue(row["bands"])) > 0 {
			hasAnyL3 = true
			break
		}
	}
	if !hasAnyL3 {
		return
	}
	snap.L2RenderProbe = nil
	snap.L2RenderProbes = nil
	snap.RealtimeBandEnergySummary = nil
	snap.RealtimeBandEnergySummaries = nil
	snap.RealtimeStereoRelationSummary = nil
	snap.RealtimeStereoRelationSummaries = nil
}

func usableFrequencyL2TrackCount(rows []map[string]any) int {
	tracks := map[string]bool{}
	for _, row := range rows {
		status := featureStatus(row)
		if status != "ready" && status != "partial" {
			continue
		}
		if len(mapValue(row["bands"])) == 0 || !strings.EqualFold(cleanAnyString(row["tap_point"]), "track_post_fader") {
			continue
		}
		if trackID := cleanAnyString(row["track_id"]); trackID != "" {
			tracks[trackID] = true
		}
	}
	return len(tracks)
}

var frequencyAcousticSnapshotCache struct {
	sync.Mutex
	entries map[string]frequencyAcousticSnapshotCacheEntry
}

type frequencyAcousticSnapshotCacheEntry struct {
	size    int64
	modTime time.Time
	snap    acousticpackage.Snapshot
}

var frequencyObservationRecoveryCache struct {
	sync.Mutex
	key       string
	signature string
	rows      featureSnapshot
}

func recoverFrequencySnapshotFromPriorObservations(snap *featureSnapshot, projectState map[string]any, mixSessionID string) map[string]any {
	result := map[string]any{"status": "missing", "file_count": 0, "matched_track_count": 0}
	if snap == nil || !strings.HasPrefix(strings.TrimSpace(mixSessionID), "cap_v1_c1_") {
		return result
	}
	family := c1SessionFamily(mixSessionID)
	if family == "" {
		return result
	}
	paths, signature := c1ObservationFamilyFiles(DefaultRoot(), family)
	result["file_count"] = len(paths)
	if len(paths) == 0 {
		return result
	}
	frequencyObservationRecoveryCache.Lock()
	var recovered featureSnapshot
	if frequencyObservationRecoveryCache.key == family && frequencyObservationRecoveryCache.signature == signature {
		recovered = frequencyObservationRecoveryCache.rows
	} else {
		recovered = readFrequencyRowsFromObservations(paths)
		frequencyObservationRecoveryCache.key = family
		frequencyObservationRecoveryCache.signature = signature
		frequencyObservationRecoveryCache.rows = recovered
	}
	frequencyObservationRecoveryCache.Unlock()
	snap.BandEnergySummaries = append(snap.BandEnergySummaries, recovered.BandEnergySummaries...)
	snap.StereoRelationSummaries = append(snap.StereoRelationSummaries, recovered.StereoRelationSummaries...)
	snap.LoudnessSummaries = append(snap.LoudnessSummaries, recovered.LoudnessSummaries...)
	matched := map[string]bool{}
	for _, row := range recovered.BandEnergySummaries {
		if trackWaveformRowMatchesProjectState(row, projectState) {
			matched[cleanAnyString(row["track_id"])] = true
		}
	}
	result["matched_track_count"] = len(matched)
	if len(matched) > 0 {
		result["status"] = "ready"
	}
	return result
}

func c1SessionFamily(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	index := strings.LastIndex(sessionID, "_")
	if index <= 0 || index == len(sessionID)-1 {
		return sessionID
	}
	if _, err := strconv.Atoi(sessionID[index+1:]); err == nil {
		return sessionID[:index]
	}
	return sessionID
}

func c1ObservationFamilyFiles(root, family string) ([]string, string) {
	root, family = strings.TrimSpace(root), strings.TrimSpace(family)
	if root == "" || family == "" {
		return nil, ""
	}
	dirs, _ := filepath.Glob(filepath.Join(root, safePathName(family)+"_*"))
	paths := []string{}
	var totalSize int64
	var newest int64
	for _, dir := range dirs {
		matches, _ := filepath.Glob(filepath.Join(dir, "observations", "*.json"))
		if data, err := os.ReadFile(filepath.Join(dir, "current.json")); err == nil {
			var board Board
			if json.Unmarshal(data, &board) == nil && strings.TrimSpace(board.LatestObservationPath) != "" {
				matches = append(matches, filepath.Clean(board.LatestObservationPath))
			}
		}
		for _, path := range matches {
			info, err := os.Stat(path)
			if err != nil || info.Size() <= 0 || info.Size() > 32*1024*1024 {
				continue
			}
			paths = append(paths, path)
			totalSize += info.Size()
			if modified := info.ModTime().UnixNano(); modified > newest {
				newest = modified
			}
		}
	}
	paths = uniquePaths(paths)
	sort.Strings(paths)
	return paths, fmt.Sprintf("%d:%d:%d", len(paths), totalSize, newest)
}

func uniquePaths(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		key := strings.ToLower(path)
		if path == "." || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, path)
	}
	return out
}

func readFrequencyRowsFromObservations(paths []string) featureSnapshot {
	out := featureSnapshot{}
	bandByKey := map[string]map[string]any{}
	stereoByKey := map[string]map[string]any{}
	loudnessByKey := map[string]map[string]any{}
	for _, path := range paths {
		observation, err := readObservationFile(path)
		if err != nil {
			continue
		}
		snapshot := mapValue(observation.GlobalSummary["feature_snapshot"])
		collectRecoveredFrequencyRows(bandByKey, snapshot["band_energy_summary"], snapshot["band_energy_summaries"])
		collectRecoveredFrequencyRows(stereoByKey, snapshot["stereo_relation_summary"], snapshot["stereo_relation_summaries"])
		collectRecoveredFrequencyRows(loudnessByKey, snapshot["loudness_summary"], snapshot["loudness_summaries"])
	}
	out.BandEnergySummaries = sortedRecoveredFrequencyRows(bandByKey)
	out.StereoRelationSummaries = sortedRecoveredFrequencyRows(stereoByKey)
	out.LoudnessSummaries = sortedRecoveredFrequencyRows(loudnessByKey)
	return out
}

func collectRecoveredFrequencyRows(out map[string]map[string]any, values ...any) {
	for _, value := range values {
		rows := mapRowsAny(value)
		if row := mapValue(value); len(row) > 0 {
			rows = append(rows, row)
		}
		for _, row := range rows {
			status := featureStatus(row)
			if status != "ready" && status != "partial" {
				continue
			}
			if bridgeFeatureUsesRenderRevision(row) || !featureRowHasMaterialIdentity(row) {
				continue
			}
			key := strings.Join([]string{cleanAnyString(row["track_id"]), cleanAnyString(row["clip_id"]), firstNonEmpty(cleanAnyString(row["source_revision"]), cleanAnyString(row["source_fingerprint"]))}, "\x00")
			if key == "\x00\x00" {
				continue
			}
			if current := out[key]; len(current) == 0 || recoveredFrequencyRowScore(row) > recoveredFrequencyRowScore(current) {
				out[key] = row
			}
		}
	}
}

func recoveredFrequencyRowScore(row map[string]any) int {
	score := 0
	if featureStatus(row) == "ready" {
		score += 100
	}
	if len(mapValue(row["bands"])) > 0 {
		score += 20
	}
	for _, key := range []string{"evidence_ref", "source_revision", "source_fingerprint", "quality_status", "updated_at"} {
		if cleanAnyString(row[key]) != "" {
			score++
		}
	}
	return score
}

func sortedRecoveredFrequencyRows(rows map[string]map[string]any) []map[string]any {
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		out = append(out, rows[key])
	}
	return out
}

func readFrequencyAcousticSnapshot(args map[string]any) (acousticpackage.Snapshot, string, error) {
	primaryPath := acousticpackage.DefaultStorePath(args)
	merged, err := readFrequencyAcousticSnapshotPath(primaryPath)
	if err != nil {
		return acousticpackage.Snapshot{}, primaryPath, err
	}
	for _, fallbackPath := range frequencyAcousticFallbackPaths(args, primaryPath) {
		fallback, fallbackErr := readFrequencyAcousticSnapshotPath(fallbackPath)
		if fallbackErr != nil {
			continue
		}
		merged.Packages = append(merged.Packages, fallback.Packages...)
	}
	return merged, primaryPath, nil
}

func readFrequencyAcousticSnapshotPath(path string) (acousticpackage.Snapshot, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion}, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion}, nil
		}
		return acousticpackage.Snapshot{}, err
	}
	frequencyAcousticSnapshotCache.Lock()
	defer frequencyAcousticSnapshotCache.Unlock()
	key := strings.ToLower(path)
	if cached, ok := frequencyAcousticSnapshotCache.entries[key]; ok && cached.size == info.Size() && cached.modTime.Equal(info.ModTime()) {
		return cached.snap, nil
	}
	snap, err := acousticpackage.NewStore(path).Read()
	if err != nil {
		return acousticpackage.Snapshot{}, err
	}
	if frequencyAcousticSnapshotCache.entries == nil {
		frequencyAcousticSnapshotCache.entries = map[string]frequencyAcousticSnapshotCacheEntry{}
	}
	frequencyAcousticSnapshotCache.entries[key] = frequencyAcousticSnapshotCacheEntry{size: info.Size(), modTime: info.ModTime(), snap: snap}
	return snap, nil
}

// Source-file L3 facts are portable across project identities when their file
// descriptor still matches the current material. Project Package v2 uses a
// project-local acoustic store, while older installations kept the same facts
// in the workspace store. Read that legacy store as a fallback so opening a
// folder snapshot does not strand C1 merely because the local store contains
// only rows materialized after Save As. The hydration path below still applies
// exact source/path validation before any fallback row can enter the model.
func frequencyAcousticFallbackPaths(args map[string]any, primaryPath string) []string {
	paths := []string{}
	for _, raw := range anySlice(args["acoustic_package_fallback_paths"]) {
		if path := strings.TrimSpace(fmt.Sprint(raw)); path != "" && path != "<nil>" {
			paths = append(paths, path)
		}
	}
	if path := strings.TrimSpace(os.Getenv("VIT_ACOUSTIC_PACKAGE_FALLBACK_PATH")); path != "" {
		paths = append(paths, path)
	}
	if devRoot := strings.TrimSpace(os.Getenv("VIT_DAW_DEV_ROOT")); devRoot != "" {
		legacyPath := filepath.Join(filepath.Clean(devRoot), "VitApp", "Workspace", "Artifacts", "acoustic_package_status.json")
		if _, err := os.Stat(legacyPath); err == nil {
			paths = append(paths, legacyPath)
		}
	}
	primaryKey := strings.ToLower(filepath.Clean(strings.TrimSpace(primaryPath)))
	out := []string{}
	for _, path := range uniquePaths(paths) {
		if strings.ToLower(filepath.Clean(path)) != primaryKey {
			out = append(out, path)
		}
	}
	return out
}

func hydrateFrequencySnapshotFromAcousticPackages(snap *featureSnapshot, projectState map[string]any, args map[string]any) map[string]any {
	result := map[string]any{"source": "existing_acoustic_package", "status": "missing", "matched_track_count": 0, "matched_l2_track_count": 0, "matched_l3_track_count": 0, "package_read_count": 0}
	if snap == nil {
		result["reason"] = "feature_snapshot_unavailable"
		return result
	}
	packages, path, err := readFrequencyAcousticSnapshot(args)
	result["acoustic_package_path"] = path
	result["package_read_count"] = len(packages.Packages)
	if err != nil {
		result["status"], result["reason"] = "unavailable", err.Error()
		return result
	}
	projectID := projectIdentityFromState(projectState)
	currentByTarget := map[string]acousticpackage.Status{}
	currentCandidatesByTarget := map[string][]acousticpackage.Status{}
	allCandidatesByTarget := map[string][]acousticpackage.Status{}
	readyBySource := map[string][]acousticpackage.Status{}
	readyBySourcePath := map[string][]acousticpackage.Status{}
	for _, pkg := range packages.Packages {
		if pkg.TrackID != "" || pkg.ClipID != "" {
			key := pkg.TrackID + "\x00" + pkg.ClipID
			allCandidatesByTarget[key] = append(allCandidatesByTarget[key], pkg)
			if packageProjectMatches(pkg.ProjectID, projectID) {
				currentCandidatesByTarget[key] = append(currentCandidatesByTarget[key], pkg)
				if newerAcousticPackage(pkg, currentByTarget[key]) {
					currentByTarget[key] = pkg
				}
			}
		}
		if sourceKey := acousticSourceIdentityKey(pkg.SourceRevision, pkg.SourceFingerprint); sourceKey != "" && packageHasReadySourceFileL3(pkg) {
			readyBySource[sourceKey] = append(readyBySource[sourceKey], pkg)
		}
		if sourcePath := normalizedFeatureMaterialPath(pkg.SourcePath); sourcePath != "" && packageHasReadySourceFileL3(pkg) {
			readyBySourcePath[sourcePath] = append(readyBySourcePath[sourcePath], pkg)
		}
	}
	matchedTracks := map[string]bool{}
	matchedL2Tracks := map[string]bool{}
	matchedL3Tracks := map[string]bool{}
	for _, track := range mapRowsAny(projectState["tracks"]) {
		trackID := firstNonEmpty(cleanAnyString(track["track_id"]), cleanAnyString(track["id"]))
		for _, rawClip := range anySlice(firstPresent(track, "clips", "clip_summaries")) {
			clip := mapValue(rawClip)
			clipID := firstNonEmpty(cleanAnyString(clip["clip_id"]), cleanAnyString(clip["id"]), cleanAnyString(clip["item_id"]))
			targetKey := trackID + "\x00" + clipID
			current := currentByTarget[targetKey]
			currentL2 := newestCurrentAcousticL2(currentCandidatesByTarget[targetKey], projectID, trackID, clipID, track, clip)
			l2Added := appendFrequencyL2PackageRow(snap, currentL2, projectID, trackID, clipID, false)
			if !l2Added {
				portableL2 := newestPortableAcousticL2(allCandidatesByTarget[targetKey], trackID, clipID, track, clip)
				l2Added = appendFrequencyL2PackageRow(snap, portableL2, projectID, trackID, clipID, true)
			}
			if l2Added {
				matchedTracks[trackID] = true
				matchedL2Tracks[trackID] = true
			}
			sourceRevision := firstNonEmpty(
				cleanAnyString(clip["source_revision"]), cleanAnyString(clip["source_fingerprint"]),
				current.SourceRevision, current.SourceFingerprint,
			)
			candidates := readyBySource[acousticSourceIdentityKey(sourceRevision, sourceRevision)]
			stateSourcePath := firstNonEmpty(
				cleanAnyString(clip["current_source_path"]), cleanAnyString(clip["source_path"]), cleanAnyString(clip["file_path"]),
				cleanAnyString(track["source_path"]), cleanAnyString(track["file_path"]),
			)
			if len(candidates) == 0 && stateSourcePath != "" {
				for _, candidate := range readyBySourcePath[normalizedFeatureMaterialPath(stateSourcePath)] {
					packageRevision := firstNonEmpty(candidate.SourceRevision, candidate.SourceFingerprint, candidate.SourceHash)
					if acousticpackage.SourceRevisionMatchesIdentity(packageRevision, acousticpackage.Identity{SourcePath: stateSourcePath}) {
						candidates = append(candidates, candidate)
					}
				}
			}
			if len(candidates) == 0 {
				continue
			}
			best := newestReadySourceFileL3(candidates)
			matchedRevision := firstNonEmpty(sourceRevision, best.SourceRevision, best.SourceFingerprint)
			added := appendFrequencyL3PackageRows(snap, best, projectID, trackID, clipID, clip, matchedRevision)
			if added > 0 {
				matchedTracks[trackID] = true
				matchedL3Tracks[trackID] = true
			}
		}
	}
	result["matched_track_count"] = len(matchedTracks)
	result["matched_l2_track_count"] = len(matchedL2Tracks)
	result["matched_l3_track_count"] = len(matchedL3Tracks)
	if len(matchedTracks) > 0 {
		result["status"] = "ready"
	}
	return result
}

func newestCurrentAcousticL2(candidates []acousticpackage.Status, projectID, trackID, clipID string, track, clip map[string]any) acousticpackage.Status {
	best := acousticpackage.Status{}
	for _, candidate := range candidates {
		if currentAcousticL2MatchesProjectMaterial(candidate, projectID, trackID, clipID, track, clip) && newerAcousticPackage(candidate, best) {
			best = candidate
		}
	}
	return best
}

func newestPortableAcousticL2(candidates []acousticpackage.Status, trackID, clipID string, track, clip map[string]any) acousticpackage.Status {
	best := acousticpackage.Status{}
	for _, candidate := range candidates {
		if portableAcousticL2MatchesCurrentState(candidate, trackID, clipID, track, clip) && newerAcousticPackage(candidate, best) {
			best = candidate
		}
	}
	return best
}

// Save As changes project identity while preserving the underlying Tracktion
// ValueTrees. A prior-project L2 row is portable only when the current source
// file and the exact deterministic track/clip state revisions all match the
// revisions embedded by the existing L2 renderer. This is intentionally much
// stricter than matching track IDs or source paths alone.
func portableAcousticL2MatchesCurrentState(pkg acousticpackage.Status, trackID, clipID string, track, clip map[string]any) bool {
	if strings.TrimSpace(pkg.TrackID) != strings.TrimSpace(trackID) || strings.TrimSpace(pkg.ClipID) != strings.TrimSpace(clipID) {
		return false
	}
	layer, ok := pkg.PackageLayers["l2_realtime"]
	if !ok {
		return false
	}
	feature, ok := layer.Features["render_probe"]
	if !ok || (feature.Status != acousticpackage.StatusReady && feature.Status != acousticpackage.StatusPartial) || len(feature.Ref) == 0 {
		return false
	}
	if !strings.EqualFold(cleanAnyString(feature.Ref["tap_point"]), "track_post_fader") || len(mapValue(feature.Ref["bands"])) == 0 {
		return false
	}
	stateSourcePath := firstNonEmpty(cleanAnyString(clip["current_source_path"]), cleanAnyString(clip["source_path"]), cleanAnyString(clip["file_path"]), cleanAnyString(track["source_path"]), cleanAnyString(track["file_path"]))
	packageSourceRevision := firstNonEmpty(pkg.SourceRevision, pkg.SourceFingerprint, pkg.SourceHash, cleanAnyString(feature.Ref["source_revision"]), cleanAnyString(feature.Ref["source_fingerprint"]), cleanAnyString(feature.Ref["source_hash"]))
	if stateSourcePath == "" || packageSourceRevision == "" || (pkg.SourcePath != "" && normalizedFeatureMaterialPath(pkg.SourcePath) != normalizedFeatureMaterialPath(stateSourcePath)) {
		return false
	}
	if !acousticpackage.SourceRevisionMatchesIdentity(packageSourceRevision, acousticpackage.Identity{SourcePath: stateSourcePath}) {
		return false
	}
	trackStateRevision := cleanAnyString(track["track_state_revision"])
	clipStateRevision := cleanAnyString(clip["clip_state_revision"])
	renderRevision := firstNonEmpty(cleanAnyString(feature.Ref["render_revision"]), pkg.RenderRevision)
	if trackStateRevision == "" || clipStateRevision == "" || renderRevision == "" {
		return false
	}
	return strings.EqualFold(renderRevisionIdentityField(renderRevision, "track_state"), trackStateRevision) &&
		strings.EqualFold(renderRevisionIdentityField(renderRevision, "clip_state"), clipStateRevision)
}

func renderRevisionIdentityField(revision, field string) string {
	prefix := strings.TrimSpace(field) + "="
	if prefix == "=" {
		return ""
	}
	for _, part := range strings.Split(revision, "|") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(part, prefix))
		}
	}
	return ""
}

// Diagnosis may reuse a ready post-fader package row without turning it into a
// mutation cache hit. The package must describe the exact current project,
// track, clip, and source material. A later mutation preflight still requires
// an exact track_state_fingerprint through CollectL2RenderProbeBatch.
func currentAcousticL2MatchesProjectMaterial(pkg acousticpackage.Status, projectID, trackID, clipID string, track, clip map[string]any) bool {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(pkg.ProjectID) == "" || !strings.EqualFold(strings.TrimSpace(pkg.ProjectID), strings.TrimSpace(projectID)) {
		return false
	}
	if strings.TrimSpace(pkg.TrackID) != strings.TrimSpace(trackID) || strings.TrimSpace(pkg.ClipID) != strings.TrimSpace(clipID) {
		return false
	}
	stateSourceRevision := firstNonEmpty(cleanAnyString(clip["source_revision"]), cleanAnyString(clip["source_fingerprint"]), cleanAnyString(clip["source_hash"]))
	packageSourceRevision := firstNonEmpty(pkg.SourceRevision, pkg.SourceFingerprint, pkg.SourceHash)
	stateSourcePath := firstNonEmpty(cleanAnyString(clip["current_source_path"]), cleanAnyString(clip["source_path"]), cleanAnyString(clip["file_path"]), cleanAnyString(track["source_path"]), cleanAnyString(track["file_path"]))
	if packageSourceRevision == "" || (pkg.SourcePath != "" && stateSourcePath != "" && normalizedFeatureMaterialPath(pkg.SourcePath) != normalizedFeatureMaterialPath(stateSourcePath)) {
		return false
	}
	if !acousticpackage.SourceRevisionMatchesIdentity(packageSourceRevision, acousticpackage.Identity{SourcePath: stateSourcePath, SourceRevision: stateSourceRevision, SourceFingerprint: stateSourceRevision, SourceHash: cleanAnyString(clip["source_hash"])}) {
		return false
	}
	stateClipRevision := firstNonEmpty(cleanAnyString(clip["clip_revision"]), cleanAnyString(clip["revision"]))
	if stateClipRevision != "" && pkg.ClipRevision != "" && stateClipRevision != pkg.ClipRevision {
		return false
	}
	stateRenderRevision := firstNonEmpty(cleanAnyString(track["render_revision"]), cleanAnyString(track["track_render_revision"]), cleanAnyString(clip["render_revision"]))
	if stateRenderRevision != "" && pkg.RenderRevision != "" && stateRenderRevision != pkg.RenderRevision {
		return false
	}
	return true
}

func appendFrequencyL2PackageRow(snap *featureSnapshot, pkg acousticpackage.Status, projectID, trackID, clipID string, allowProjectRebind bool) bool {
	if snap == nil || pkg.TrackID == "" || pkg.ClipID == "" {
		return false
	}
	layer, ok := pkg.PackageLayers["l2_realtime"]
	if !ok {
		return false
	}
	feature, ok := layer.Features["render_probe"]
	silenceConfirmed := acousticPackageConfirmsDeterministicSilence(pkg, feature)
	if !ok || ((feature.Status != acousticpackage.StatusReady && feature.Status != acousticpackage.StatusPartial) && !silenceConfirmed) || len(feature.Ref) == 0 {
		return false
	}
	row := make(map[string]any, len(feature.Ref)+8)
	for key, value := range feature.Ref {
		row[key] = value
	}
	if rowProjectID := cleanAnyString(row["project_id"]); rowProjectID != "" && !strings.EqualFold(rowProjectID, projectID) && !allowProjectRebind {
		return false
	}
	if rowTrackID := cleanAnyString(row["track_id"]); rowTrackID != "" && rowTrackID != trackID {
		return false
	}
	if rowClipID := cleanAnyString(row["clip_id"]); rowClipID != "" && rowClipID != clipID {
		return false
	}
	packageSourceRevision := firstNonEmpty(pkg.SourceRevision, pkg.SourceFingerprint, pkg.SourceHash)
	if rowSourceRevision := firstNonEmpty(cleanAnyString(row["source_revision"]), cleanAnyString(row["source_fingerprint"]), cleanAnyString(row["source_hash"])); rowSourceRevision != "" && acousticSourceIdentityKey(rowSourceRevision, rowSourceRevision) != acousticSourceIdentityKey(packageSourceRevision, packageSourceRevision) {
		return false
	}
	if len(mapValue(row["bands"])) == 0 || !strings.EqualFold(cleanAnyString(row["tap_point"]), "track_post_fader") {
		return false
	}
	row["status"] = feature.Status
	if silenceConfirmed {
		// A current exact source that is all-zero both before and after the track
		// chain is valid C1 evidence: it proves that no spectral action is
		// available. Keep it comparable at the real post-fader tap while marking
		// the row partial so the decision layer must classify it as no_change.
		row["status"] = acousticpackage.StatusPartial
		row["silence_confirmed"] = true
		row["silence_reason"] = "source_and_post_fader_all_zero"
		row["original_quality_status"] = feature.Status
	}
	row["feature_type"] = "l2_render_probe"
	row["source"] = firstNonEmpty(cleanAnyString(row["source"]), feature.Source, "l2_render_probe")
	row["tap_point"] = "track_post_fader"
	row["project_id"], row["track_id"], row["clip_id"] = projectID, trackID, clipID
	row["source_revision"] = firstNonEmpty(cleanAnyString(row["source_revision"]), pkg.SourceRevision, pkg.SourceFingerprint)
	row["clip_revision"] = firstNonEmpty(cleanAnyString(row["clip_revision"]), pkg.ClipRevision)
	row["render_revision"] = firstNonEmpty(cleanAnyString(row["render_revision"]), pkg.RenderRevision)
	snap.L2RenderProbes = mergeFeatureRowsByCurrentMaterial(snap.L2RenderProbes, row)
	return true
}

func acousticPackageConfirmsDeterministicSilence(pkg acousticpackage.Status, renderProbe acousticpackage.FeatureStatus) bool {
	if !strings.EqualFold(strings.TrimSpace(renderProbe.Status), acousticpackage.StatusSuspect) ||
		!strings.EqualFold(firstNonEmpty(strings.TrimSpace(renderProbe.Reason), cleanAnyString(renderProbe.Ref["reason"])), "all_zero_render") ||
		!strings.EqualFold(cleanAnyString(renderProbe.Ref["tap_point"]), "track_post_fader") {
		return false
	}
	layer, ok := pkg.PackageLayers["l1_static"]
	if !ok {
		return false
	}
	waveform, ok := layer.Features["waveform_envelope"]
	if !ok || len(waveform.Ref) == 0 {
		return false
	}
	qualityReason := firstNonEmpty(
		strings.TrimSpace(waveform.Reason),
		cleanAnyString(waveform.Ref["quality_reason"]),
		cleanAnyString(waveform.Ref["reason"]),
	)
	return strings.EqualFold(qualityReason, "input_all_zero") || strings.EqualFold(qualityReason, "all_zero_source")
}

func packageProjectMatches(packageID, projectID string) bool {
	if !concreteProjectIdentity(projectID) || !concreteProjectIdentity(packageID) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(packageID), strings.TrimSpace(projectID))
}

func projectIdentityFromState(state map[string]any) string {
	project := mapValue(state["project"])
	return firstNonEmpty(cleanAnyString(state["project_uuid"]), cleanAnyString(state["project_id"]), cleanAnyString(project["project_uuid"]), cleanAnyString(project["uuid"]), cleanAnyString(project["project_id"]), cleanAnyString(project["id"]))
}

func acousticSourceIdentityKey(revision, fingerprint string) string {
	value := firstNonEmpty(strings.TrimSpace(revision), strings.TrimSpace(fingerprint))
	if value == "" {
		return ""
	}
	return strings.ToLower(strings.ReplaceAll(value, "\\", "/"))
}

func newerAcousticPackage(candidate, current acousticpackage.Status) bool {
	if current.TrackID == "" && current.ClipID == "" {
		return true
	}
	return candidate.UpdatedAt > current.UpdatedAt
}

func packageHasReadySourceFileL3(pkg acousticpackage.Status) bool {
	layer, ok := pkg.PackageLayers["l3_deep"]
	if !ok {
		return false
	}
	for _, key := range []string{"band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		feature := layer.Features[key]
		if ((feature.Status == acousticpackage.StatusReady || feature.Status == acousticpackage.StatusPartial) || l3FeatureConfirmsDeterministicSilence(feature)) && len(feature.Ref) > 0 {
			return true
		}
	}
	return false
}

func newestReadySourceFileL3(candidates []acousticpackage.Status) acousticpackage.Status {
	best := acousticpackage.Status{}
	for _, candidate := range candidates {
		if packageHasReadySourceFileL3(candidate) && newerAcousticPackage(candidate, best) {
			best = candidate
		}
	}
	return best
}

func appendFrequencyL3PackageRows(snap *featureSnapshot, pkg acousticpackage.Status, projectID, trackID, clipID string, clip map[string]any, sourceRevision string) int {
	layer := pkg.PackageLayers["l3_deep"]
	added := 0
	for _, spec := range []struct {
		key  string
		rows *[]map[string]any
	}{
		{key: "band_energy_summary", rows: &snap.BandEnergySummaries},
		{key: "stereo_relation_summary", rows: &snap.StereoRelationSummaries},
		{key: "loudness_summary", rows: &snap.LoudnessSummaries},
	} {
		feature := layer.Features[spec.key]
		silenceConfirmed := l3FeatureConfirmsDeterministicSilence(feature)
		if (feature.Status != acousticpackage.StatusReady && feature.Status != acousticpackage.StatusPartial && !silenceConfirmed) || len(feature.Ref) == 0 {
			continue
		}
		ref := mapValue(feature.Ref)
		if len(ref) == 0 {
			continue
		}
		row := make(map[string]any, len(ref)+8)
		for key, value := range ref {
			row[key] = value
		}
		row["status"] = feature.Status
		if silenceConfirmed {
			row["status"] = acousticpackage.StatusPartial
			row["silence_confirmed"] = true
			row["silence_reason"] = "source_file_all_zero"
			row["original_quality_status"] = feature.Status
		}
		row["feature_type"] = spec.key
		row["source"] = firstNonEmpty(cleanAnyString(row["source"]), feature.Source, "kernel_l3_offline_analyzer")
		row["source_kind"] = "source_file"
		row["tap_point"] = "source_file_pre_fx"
		row["project_id"], row["track_id"], row["clip_id"] = projectID, trackID, clipID
		row["source_revision"], row["source_fingerprint"] = sourceRevision, sourceRevision
		if path := firstNonEmpty(cleanAnyString(clip["current_source_path"]), cleanAnyString(clip["source_path"]), cleanAnyString(clip["file_path"]), pkg.SourcePath); path != "" {
			row["source_path"], row["file_path"] = path, path
		}
		*spec.rows = mergeFeatureRowsByCurrentMaterial(*spec.rows, row)
		added++
	}
	return added
}

func l3FeatureConfirmsDeterministicSilence(feature acousticpackage.FeatureStatus) bool {
	if !strings.EqualFold(strings.TrimSpace(feature.Status), acousticpackage.StatusSuspect) || len(feature.Ref) == 0 {
		return false
	}
	reason := firstNonEmpty(strings.TrimSpace(feature.Reason), cleanAnyString(feature.Ref["quality_reason"]), cleanAnyString(feature.Ref["reason"]))
	if !strings.EqualFold(reason, "all_zero_audio") && !strings.EqualFold(reason, "all_zero_source") && !strings.EqualFold(reason, "input_all_zero") {
		return false
	}
	return numberFromMap(feature.Ref, "coverage_ratio") >= 0.999
}

func mergeFeatureRowsByCurrentMaterial(rows []map[string]any, update map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows)+1)
	for _, row := range rows {
		if cleanAnyString(row["track_id"]) == cleanAnyString(update["track_id"]) && cleanAnyString(row["clip_id"]) == cleanAnyString(update["clip_id"]) {
			continue
		}
		out = append(out, row)
	}
	return append(out, update)
}

func mergeWaveformRowsPreservingTimeSegments(rows []map[string]any, update map[string]any) []map[string]any {
	candidates := []map[string]any{}
	for _, existing := range rows {
		if cleanAnyString(existing["track_id"]) != cleanAnyString(update["track_id"]) || cleanAnyString(existing["clip_id"]) != cleanAnyString(update["clip_id"]) {
			continue
		}
		if len(waveformTimeSegments(existing)) > 0 {
			candidates = append(candidates, existing)
		}
	}
	if len(candidates) == 0 {
		return mergeFeatureRowsByCurrentMaterial(rows, update)
	}
	chosen := candidates[0]
	bestMtime := waveformRevisionMtime(chosen)
	for _, candidate := range candidates[1:] {
		if mtime := waveformRevisionMtime(candidate); mtime > bestMtime {
			chosen = candidate
			bestMtime = mtime
		}
	}
	merged := copyAnyMap(chosen)
	for key, value := range update {
		if _, present := merged[key]; !present {
			merged[key] = value
		}
	}
	return mergeFeatureRowsByCurrentMaterial(rows, merged)
}

func waveformRevisionMtime(row map[string]any) int64 {
	revision := firstNonEmpty(cleanAnyString(row["source_revision"]), cleanAnyString(row["source_fingerprint"]))
	index := strings.Index(revision, "mtime=")
	if index < 0 {
		return 0
	}
	value := revision[index+len("mtime="):]
	if end := strings.IndexAny(value, "|"); end >= 0 {
		value = value[:end]
	}
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}

func mergeFrequencyAssemblyTiming(result map[string]any, elapsed time.Duration) map[string]any {
	if result == nil {
		result = map[string]any{}
	}
	result["elapsed_ms"] = elapsed.Milliseconds()
	result["persistence_writes"] = 0
	result["mix_observe_calls"] = 0
	return result
}

type featureSnapshot struct {
	SchemaVersion                   string           `json:"schema_version"`
	UpdatedAt                       string           `json:"updated_at"`
	LatestRequest                   map[string]any   `json:"latest_request"`
	WaveformEnvelope                map[string]any   `json:"waveform_envelope"`
	TrackWaveformEnvelopes          []map[string]any `json:"track_waveform_envelopes"`
	SpectrogramTiles                map[string]any   `json:"spectrogram_tiles"`
	SpectrogramTileRows             []map[string]any `json:"spectrogram_tile_rows,omitempty"`
	BandEnergySummary               map[string]any   `json:"band_energy_summary"`
	BandEnergySummaries             []map[string]any `json:"band_energy_summaries,omitempty"`
	StereoRelationSummary           map[string]any   `json:"stereo_relation_summary"`
	StereoRelationSummaries         []map[string]any `json:"stereo_relation_summaries,omitempty"`
	LoudnessSummary                 map[string]any   `json:"loudness_summary,omitempty"`
	LoudnessSummaries               []map[string]any `json:"loudness_summaries,omitempty"`
	RealtimeBandEnergySummary       map[string]any   `json:"realtime_band_energy_summary,omitempty"`
	RealtimeBandEnergySummaries     []map[string]any `json:"realtime_band_energy_summaries,omitempty"`
	RealtimeStereoRelationSummary   map[string]any   `json:"realtime_stereo_relation_summary,omitempty"`
	RealtimeStereoRelationSummaries []map[string]any `json:"realtime_stereo_relation_summaries,omitempty"`
	L2RenderProbe                   map[string]any   `json:"l2_render_probe,omitempty"`
	L2RenderProbes                  []map[string]any `json:"l2_render_probes,omitempty"`
	MaskingMeasurement              map[string]any   `json:"masking_measurement,omitempty"`
	// TransientEvents carries the DAD L3 frame-level transient block
	// (status/events/coverage/window_ms/hop_ms/evidence_refs) that feeds
	// source-only micro-transient statistics.
	TransientEvents map[string]any `json:"transient_events,omitempty"`
}

func DefaultRoot() string {
	if override := strings.TrimSpace(os.Getenv("VIT_MIXBOARD_ROOT")); override != "" {
		return filepath.Clean(override)
	}
	if roots, ok := projectstore.Current(); ok {
		return filepath.Join(roots.Agent, "mixboard", "sessions")
	}
	if devRoot := strings.TrimSpace(os.Getenv("VIT_DAW_DEV_ROOT")); devRoot != "" {
		root := filepath.Clean(devRoot)
		if _, err := os.Stat(filepath.Join(root, "VitApp", "Workspace")); err == nil {
			return filepath.Join(root, "VitApp", "Workspace", "Artifacts", "mixboard")
		}
	}
	return filepath.Join(os.TempDir(), "vit-daw-unbound", fmt.Sprint(os.Getpid()), "mixboard")
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
	previousObservation, hasPreviousObservation := s.readPreviousObservation(sessionDir, req.Args)
	featureSnapshot := loadFeatureSnapshot(req.Args)
	observation := buildObservation(req, now, &featureSnapshot)
	applyBeforeAfterDelta(&observation, previousObservation, hasPreviousObservation, now)
	FinalizeObservationContext(&observation, req, now)
	persistedObservation, canonicalObservationPath, persistenceErr := projectObservationForPersistence(req, observation, featureSnapshot)
	if persistenceErr != nil {
		return WriteResult{}, persistenceErr
	}
	actionsDir := filepath.Join(sessionDir, "actions")
	if err := os.MkdirAll(actionsDir, 0o755); err != nil {
		return WriteResult{}, err
	}

	observationPath := canonicalObservationPath
	if observationPath == "" {
		// Legacy/dev stores keep their self-contained session layout. In v2 the
		// canonical project observation is written once under
		// .vit_agent/<uuid>/observations and the board references it directly.
		obsDir := filepath.Join(sessionDir, "observations")
		if err := os.MkdirAll(obsDir, 0o755); err != nil {
			return WriteResult{}, err
		}
		observationPath = filepath.Join(obsDir, persistedObservation.ObservationID+".json")
		if err := writeJSON(observationPath, persistedObservation); err != nil {
			return WriteResult{}, err
		}
	}
	board := buildBoard(req, persistedObservation, observationPath, now)
	boardPath := filepath.Join(sessionDir, "current.json")
	if err := writeJSON(boardPath, board); err != nil {
		return WriteResult{}, err
	}
	contextPack := buildContextPack(req, board, persistedObservation, now)
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
		Observation:     persistedObservation,
		ContextPack:     contextPack,
	}, nil
}

func BuildObservation(req Request, createdAt string) ObservationPacket {
	snapshot := loadFeatureSnapshot(req.Args)
	return buildObservation(req, createdAt, &snapshot)
}

func buildObservation(req Request, createdAt string, featureSnapshot *featureSnapshot) ObservationPacket {
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
	hydrateFeatureSnapshotFromAnalysisManifest(featureSnapshot, req.ProjectState)
	normalizeTrackWaveformFeatureFreshness(featureSnapshot, req.ProjectState, listenScopeAllowsLegacyProjectWaveformRows(req.ListenScope))
	acousticPackageStatus := compactAcousticPackageStatus(mapValue(req.Args["acoustic_package_status"]))
	if !acousticPackageMatchesLatestRequest(acousticPackageStatus, featureSnapshot.LatestRequest) {
		acousticPackageStatus = nil
	}
	applyAcousticPackageStatusToFeatureSnapshot(featureSnapshot, acousticPackageStatus)
	normalizeProjectFeatureMaterialFreshness(featureSnapshot, req.ProjectState)
	preserveTargetWaveformTimeSegments(featureSnapshot, req)
	promoteBestL3FeatureRows(featureSnapshot)
	promoteBestRealtimeFeatureRows(featureSnapshot)
	// The general observation path (CCB mix_request_observation included) is a
	// source-file analysis: once any L3 frequency row exists, post-fader L2
	// render-probe rows never mix into per-track frequency evidence. Without
	// this, a target track observed earlier by a post-fader workflow would
	// produce a mixed tap and make the project relationship incomparable.
	preferUniformProjectL3FrequencyEvidence(featureSnapshot, req.ProjectState)
	waveformStatus := featureStatus(featureSnapshot.WaveformEnvelope)
	trackWaveformStatus := trackFeatureRowsStatus(featureSnapshot.TrackWaveformEnvelopes)
	spectrogramStatus := featureStatus(featureSnapshot.SpectrogramTiles)
	bandEnergyStatus := featureStatus(featureSnapshot.BandEnergySummary)
	stereoRelationStatus := featureStatus(featureSnapshot.StereoRelationSummary)
	loudnessStatus := featureStatus(featureSnapshot.LoudnessSummary)
	realtimeBandEnergyStatus := featureStatus(featureSnapshot.RealtimeBandEnergySummary)
	realtimeStereoRelationStatus := featureStatus(featureSnapshot.RealtimeStereoRelationSummary)
	l2RenderProbeStatus := featureStatus(featureSnapshot.L2RenderProbe)
	maskingStatus := featureStatus(featureSnapshot.MaskingMeasurement)

	caps := map[string]string{
		"waveform_envelope":        waveformStatus,
		"track_waveform_envelopes": trackWaveformStatus,
		"spectrogram_tiles":        spectrogramStatus,
		"band_energy":              bandEnergyStatus,
		"stereo_relation":          stereoRelationStatus,
		"loudness_summary":         loudnessStatus,
		"realtime_band_energy":     realtimeBandEnergyStatus,
		"realtime_stereo_relation": realtimeStereoRelationStatus,
		"l2_render_probe":          l2RenderProbeStatus,
		"masking_analysis":         maskingStatus,
		"reference_match":          "deferred",
		"lufs_analysis":            "deferred",
		"post_fx_probe":            postFXProbeCapability(l2RenderProbeStatus),
	}
	applyAcousticPackageCapabilities(caps, acousticPackageStatus)
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
	if len(acousticPackageStatus) > 0 {
		notes = append(notes, "MixBoard consumed acoustic_package_status.v0 readiness before optional background feature fill.")
	}
	waveformMetrics := buildWaveformMetrics(featureSnapshot.WaveformEnvelope)
	timeSegments := waveformTimeSegments(featureSnapshot.WaveformEnvelope)
	timeline := buildTimelineDigest(duration, segment, timeSegments)
	sections := buildSectionCandidates(duration)
	hotspots := buildFeatureHotspots(*featureSnapshot, waveformStatus, spectrogramStatus)
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
	target := normalizeTarget(req.TargetRef)
	objects := normalizeMixObjects(req.MixObjects, req.TargetRef, req.Args)
	scope := normalizeListenScope(req.ListenScope, req.MixObjects, req.Args)
	environmentPackage := buildEnvironmentPackage(req, duration, segment, frame, tempo, caps, *featureSnapshot)
	projectPackage := buildProjectPackage(req.ProjectState, target, scope, *featureSnapshot)
	bandEnergy := buildBandEnergySummary(featureSnapshot.BandEnergySummary)
	stereoRelation := buildStereoRelationSummary(featureSnapshot.StereoRelationSummary)
	loudness := buildLoudnessSummary(featureSnapshot.LoudnessSummary)
	realtimeBandEnergy := buildBandEnergySummary(featureSnapshot.RealtimeBandEnergySummary)
	realtimeStereoRelation := buildStereoRelationSummary(featureSnapshot.RealtimeStereoRelationSummary)
	l2RenderProbe := buildL2RenderProbeSummary(featureSnapshot.L2RenderProbe)
	mixPackage := buildMixPackage(req, status, waveformStatus, waveformMetrics, timeSegments, bandEnergy, stereoRelation, loudness, realtimeBandEnergy, realtimeStereoRelation, l2RenderProbe, caps)
	deepPackage := buildDeepPackage(spectrogramStatus, *featureSnapshot)
	globalSummary := map[string]any{
		"rms_dbfs":              waveformMetrics["rms_dbfs"],
		"peak_dbfs":             waveformMetrics["peak_dbfs"],
		"crest_db":              waveformMetrics["crest_db"],
		"dominant_problem_tags": problemTags,
		"feature_snapshot":      compactFeatureSnapshot(*featureSnapshot),
	}
	if len(acousticPackageStatus) > 0 {
		globalSummary["acoustic_package_status"] = acousticPackageStatus
		environmentPackage["acoustic_package_status"] = acousticPackageStatus
		mixPackage["acoustic_package_status"] = acousticPackageStatus
		deepPackage["acoustic_package_status"] = acousticPackageStatus
	}

	obs := ObservationPacket{
		SchemaVersion: ObservationSchemaVersion,
		ObservationID: "obs_" + time.Now().UTC().Format("20060102T150405") + "_" + randomID(),
		MixSessionID:  req.MixSessionID,
		Round:         req.Round,
		Status:        status,
		TargetRef:     target,
		MixObjects:    objects,
		ListenScope:   scope,
		TimeRuler: TimeRuler{
			DurationSeconds: duration,
			SegmentSeconds:  segment,
			FrameSeconds:    frame,
			TempoBPM:        tempo,
			BarMapAvailable: tempo != nil,
		},
		GlobalSummary:         globalSummary,
		EnvironmentPackage:    environmentPackage,
		ProjectPackage:        projectPackage,
		MixPackage:            mixPackage,
		DeepPackage:           deepPackage,
		SectionCandidates:     sections,
		TimelineDigest:        timeline,
		Hotspots:              hotspots,
		SourceCapabilities:    caps,
		AcousticPackageStatus: acousticPackageStatus,
		ProjectChange:         copyAnyMap(req.ProjectChange),
		Notes:                 notes,
		CreatedAt:             createdAt,
	}
	FinalizeObservationContext(&obs, req, createdAt)
	return obs
}

func hydrateFeatureSnapshotFromAnalysisManifest(snap *featureSnapshot, projectState map[string]any) {
	if snap == nil {
		return
	}
	manifest := mapValue(projectState["analysis_manifest"])
	for _, sourceRow := range mapRowsAny(manifest["l1_waveform_rows"]) {
		row := copyAnyMap(sourceRow)
		if cleanAnyString(row["track_id"]) == "" || cleanAnyString(row["clip_id"]) == "" {
			continue
		}
		if cleanAnyString(row["feature_type"]) == "" {
			row["feature_type"] = "waveform_envelope"
		}
		if cleanAnyString(row["source"]) == "" {
			row["source"] = "project_analysis_manifest"
		}
		snap.TrackWaveformEnvelopes = mergeWaveformRowsPreservingTimeSegments(snap.TrackWaveformEnvelopes, row)
	}
}

func buildBoard(req Request, obs ObservationPacket, obsPath, now string) Board {
	blockers := []string{}
	if obs.SourceCapabilities["waveform_envelope"] != "ready" && !observationHasReadyAcousticFeature(obs) {
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
		LatestChange:          copyAnyMap(obs.ProjectChange),
		OpenBlockers:          blockers,
		UpdatedAt:             now,
	}
}

func observationHasReadyAcousticFeature(obs ObservationPacket) bool {
	for _, key := range []string{
		"waveform_envelope",
		"track_waveform_envelopes",
		"spectrogram_tiles",
		"band_energy",
		"band_energy_summary",
		"stereo_relation",
		"stereo_relation_summary",
		"loudness_summary",
		"realtime_band_energy",
		"realtime_band_energy_summary",
		"realtime_stereo_relation",
		"realtime_stereo_relation_summary",
		"l2_render_probe",
		"post_fx_probe",
	} {
		if observationStatusReady(obs.SourceCapabilities[key]) {
			return true
		}
	}
	for _, row := range []map[string]any{
		obs.GlobalSummary,
		obs.MixPackage,
		mapValue(obs.MixPackage["current_metrics"]),
		mapValue(obs.MixPackage["realtime_metrics"]),
	} {
		if observationAnyFeatureReady(row, "band_energy_summary", "stereo_relation_summary", "loudness_summary", "realtime_band_energy_summary", "realtime_stereo_relation_summary", "l2_render_probe", "render_probe", "spectrogram_tiles", "waveform", "band_energy", "stereo_relation", "loudness") {
			return true
		}
	}
	return observationAcousticPackageHasReadyEvidence(obs.AcousticPackageStatus)
}

func observationAnyFeatureReady(row map[string]any, keys ...string) bool {
	if len(row) == 0 {
		return false
	}
	for _, key := range keys {
		if observationFeatureReady(row[key]) {
			return true
		}
	}
	return false
}

func observationFeatureReady(value any) bool {
	row := mapValue(value)
	if len(row) > 0 {
		if observationStatusReady(cleanAnyString(row["status"])) {
			return true
		}
		if observationStatusReady(cleanAnyString(mapValue(row["ref"])["status"])) {
			return true
		}
	}
	return observationStatusReady(cleanAnyString(value))
}

func observationAcousticPackageHasReadyEvidence(status map[string]any) bool {
	if len(status) == 0 || !observationStatusReady(cleanAnyString(status["status"])) {
		return false
	}
	layers := mapValue(status["package_layers"])
	for _, layerName := range []string{"l1_static", "l2_realtime", "l3_deep"} {
		layer := mapValue(layers[layerName])
		if !observationStatusReady(cleanAnyString(layer["status"])) {
			continue
		}
		if observationAnyFeatureReady(mapValue(layer["features"]),
			"waveform_envelope",
			"peak_rms_summary",
			"time_energy",
			"live_meter",
			"realtime_spectrum",
			"render_probe",
			"realtime_stereo_correlation",
			"spectrogram_tiles",
			"band_energy_summary",
			"stereo_relation_summary",
			"loudness_summary",
		) {
			return true
		}
	}
	return false
}

func observationStatusReady(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "baseline_ready", "fresh", "available", "ok":
		return true
	default:
		return false
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
	if snap, ok := featureSnapshotFromAny(args["feature_snapshot"]); ok {
		return normalizeFeatureSnapshot(snap)
	}
	path := featureSnapshotPath(args)
	data, err := os.ReadFile(path)
	if err != nil {
		return featureSnapshot{
			SchemaVersion:                 "mixboard_feature_snapshot.v1",
			WaveformEnvelope:              map[string]any{"status": "missing"},
			SpectrogramTiles:              map[string]any{"status": "missing"},
			BandEnergySummary:             map[string]any{"status": "missing"},
			StereoRelationSummary:         map[string]any{"status": "missing"},
			LoudnessSummary:               map[string]any{"status": "missing"},
			RealtimeBandEnergySummary:     map[string]any{"status": "missing"},
			RealtimeStereoRelationSummary: map[string]any{"status": "missing"},
			L2RenderProbe:                 map[string]any{"status": "missing"},
		}
	}
	var snap featureSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return featureSnapshot{
			SchemaVersion:                 "mixboard_feature_snapshot.v1",
			WaveformEnvelope:              map[string]any{"status": "invalid"},
			SpectrogramTiles:              map[string]any{"status": "invalid"},
			BandEnergySummary:             map[string]any{"status": "invalid"},
			StereoRelationSummary:         map[string]any{"status": "invalid"},
			LoudnessSummary:               map[string]any{"status": "invalid"},
			RealtimeBandEnergySummary:     map[string]any{"status": "invalid"},
			RealtimeStereoRelationSummary: map[string]any{"status": "invalid"},
			L2RenderProbe:                 map[string]any{"status": "invalid"},
		}
	}
	return normalizeFeatureSnapshot(snap)
}

func featureSnapshotFromAny(value any) (featureSnapshot, bool) {
	if value == nil {
		return featureSnapshot{}, false
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 || string(data) == "null" {
		return featureSnapshot{}, false
	}
	var snap featureSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return featureSnapshot{}, false
	}
	return snap, true
}

func normalizeFeatureSnapshot(snap featureSnapshot) featureSnapshot {
	if snap.SchemaVersion == "" {
		snap.SchemaVersion = "mixboard_feature_snapshot.v1"
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
	if snap.LoudnessSummary == nil {
		snap.LoudnessSummary = map[string]any{"status": "missing"}
	}
	if snap.RealtimeBandEnergySummary == nil {
		snap.RealtimeBandEnergySummary = map[string]any{"status": "missing"}
	}
	if snap.RealtimeStereoRelationSummary == nil {
		snap.RealtimeStereoRelationSummary = map[string]any{"status": "missing"}
	}
	if snap.L2RenderProbe == nil {
		snap.L2RenderProbe = map[string]any{"status": "missing"}
	}
	promoteBestL3FeatureRows(&snap)
	normalizeBridgeFeatureFreshness(&snap)
	return snap
}

func applyAcousticPackageStatusToFeatureSnapshot(snap *featureSnapshot, status map[string]any) {
	if snap == nil || len(status) == 0 || cleanAnyString(status["schema_version"]) != "acoustic_package_status.v0" {
		return
	}
	if !acousticPackageMatchesLatestRequest(status, snap.LatestRequest) {
		return
	}
	latestRequestID := cleanAnyString(snap.LatestRequest["request_id"])
	if row := featureRowFromAcousticPackage(status, "l1_static", "waveform_envelope", "waveform_envelope", latestRequestID); len(row) > 0 {
		row = waveformRowWithPreservedTimeSegments(row, snap.WaveformEnvelope)
		snap.WaveformEnvelope = row
		if cleanAnyString(row["track_id"]) != "" {
			snap.TrackWaveformEnvelopes = mergeAcousticPackageTrackWaveformRows(snap.TrackWaveformEnvelopes, row)
		}
	}
	if row := featureRowFromAcousticPackage(status, "l3_deep", "spectrogram_tiles", "spectral_field", latestRequestID); len(row) > 0 {
		snap.SpectrogramTiles = row
		snap.SpectrogramTileRows = mergeFeatureRowsByIdentity(snap.SpectrogramTileRows, row)
	}
	if row := featureRowFromAcousticPackage(status, "l3_deep", "band_energy_summary", "band_energy_summary", latestRequestID); len(row) > 0 {
		snap.BandEnergySummary = row
		snap.BandEnergySummaries = mergeFeatureRowsByIdentity(snap.BandEnergySummaries, row)
	}
	if row := featureRowFromAcousticPackage(status, "l3_deep", "stereo_relation_summary", "stereo_relation_summary", latestRequestID); len(row) > 0 {
		snap.StereoRelationSummary = row
		snap.StereoRelationSummaries = mergeFeatureRowsByIdentity(snap.StereoRelationSummaries, row)
	}
	if row := featureRowFromAcousticPackage(status, "l3_deep", "loudness_summary", "loudness_summary", latestRequestID); len(row) > 0 {
		snap.LoudnessSummary = row
		snap.LoudnessSummaries = mergeFeatureRowsByIdentity(snap.LoudnessSummaries, row)
	}
	if row := featureRowFromAcousticPackage(status, "l2_realtime", "realtime_spectrum", "realtime_band_energy_summary", latestRequestID); len(row) > 0 {
		snap.RealtimeBandEnergySummaries = mergeFeatureRowsByIdentity(snap.RealtimeBandEnergySummaries, row)
	}
	if row := featureRowFromAcousticPackage(status, "l2_realtime", "realtime_stereo_correlation", "realtime_stereo_relation_summary", latestRequestID); len(row) > 0 {
		snap.RealtimeStereoRelationSummaries = mergeFeatureRowsByIdentity(snap.RealtimeStereoRelationSummaries, row)
	}
	if row := featureRowFromAcousticPackage(status, "l2_realtime", "render_probe", "l2_render_probe", latestRequestID); len(row) > 0 {
		snap.L2RenderProbe = row
		snap.L2RenderProbes = mergeFeatureRowsByIdentity(snap.L2RenderProbes, row)
	}
	promoteBestRealtimeFeatureRows(snap)
}

func waveformRowWithPreservedTimeSegments(summary, evidence map[string]any) map[string]any {
	if len(summary) == 0 || len(waveformTimeSegments(summary)) > 0 || len(waveformTimeSegments(evidence)) == 0 ||
		!waveformRowsDescribeSameMaterial(summary, evidence) {
		return summary
	}
	out := copyAnyMap(summary)
	out["time_segments"] = evidence["time_segments"]
	for _, key := range []string{"total_duration", "duration_seconds", "coverage_seconds", "coverage_ratio"} {
		if _, present := out[key]; !present && evidence[key] != nil {
			out[key] = evidence[key]
		}
	}
	return out
}

func promoteBestRealtimeFeatureRows(snap *featureSnapshot) {
	if snap == nil {
		return
	}
	target := latestRequestResolvedTarget(snap.LatestRequest)
	if row := bestRealtimeFeatureRow(snap.RealtimeBandEnergySummary, snap.RealtimeBandEnergySummaries, target); len(row) > 0 {
		snap.RealtimeBandEnergySummary = row
	}
	if row := bestRealtimeFeatureRow(snap.RealtimeStereoRelationSummary, snap.RealtimeStereoRelationSummaries, target); len(row) > 0 {
		snap.RealtimeStereoRelationSummary = row
	}
	if row := bestRealtimeFeatureRow(snap.L2RenderProbe, snap.L2RenderProbes, target); len(row) > 0 {
		snap.L2RenderProbe = row
	}
}

func promoteBestL3FeatureRows(snap *featureSnapshot) {
	if snap == nil {
		return
	}
	target := latestRequestResolvedTarget(snap.LatestRequest)
	if row := bestRealtimeFeatureRow(snap.BandEnergySummary, snap.BandEnergySummaries, target); len(row) > 0 {
		snap.BandEnergySummary = row
	}
	if row := bestRealtimeFeatureRow(snap.StereoRelationSummary, snap.StereoRelationSummaries, target); len(row) > 0 {
		snap.StereoRelationSummary = row
	}
	if row := bestRealtimeFeatureRow(snap.LoudnessSummary, snap.LoudnessSummaries, target); len(row) > 0 {
		snap.LoudnessSummary = row
	}
}

func latestRequestResolvedTarget(latestRequest map[string]any) map[string]any {
	target := mapValue(latestRequest["resolved_target"])
	if len(target) > 0 {
		return target
	}
	return nil
}

func bestRealtimeFeatureRow(primary map[string]any, rows []map[string]any, target map[string]any) map[string]any {
	candidates := make([]map[string]any, 0, len(rows)+1)
	if len(primary) > 0 {
		candidates = append(candidates, primary)
	}
	candidates = append(candidates, rows...)
	if len(candidates) == 0 {
		return primary
	}
	hasNonMismatch := false
	for _, row := range candidates {
		if len(row) > 0 && !featureRowMismatchesTarget(row, target) {
			hasNonMismatch = true
			break
		}
	}
	var best map[string]any
	for _, row := range candidates {
		if len(row) == 0 {
			continue
		}
		if hasNonMismatch && featureRowMismatchesTarget(row, target) {
			continue
		}
		if len(best) == 0 || realtimeFeatureRowBeats(row, best, target) {
			best = row
		}
	}
	return best
}

func realtimeFeatureRowBeats(candidate, current map[string]any, target map[string]any) bool {
	candidateStatus := featureStatusRank(featureStatus(candidate))
	currentStatus := featureStatusRank(featureStatus(current))
	if candidateStatus != currentStatus {
		return candidateStatus > currentStatus
	}
	candidateTarget := featureRowTargetScore(candidate, target)
	currentTarget := featureRowTargetScore(current, target)
	if candidateTarget != currentTarget {
		return candidateTarget > currentTarget
	}
	return featureRowProjectionCompleteness(candidate) > featureRowProjectionCompleteness(current)
}

func featureRowMismatchesTarget(row map[string]any, target map[string]any) bool {
	if len(row) == 0 || len(target) == 0 {
		return false
	}
	for _, key := range []string{"track_id", "clip_id"} {
		rowValue := cleanAnyString(row[key])
		targetValue := cleanAnyString(target[key])
		if key == "clip_id" && targetValue == "" {
			targetValue = cleanAnyString(target["id"])
		}
		if rowValue != "" && targetValue != "" && rowValue != targetValue {
			return true
		}
	}
	rowRevision := firstNonEmpty(cleanAnyString(row["source_revision"]), cleanAnyString(row["source_fingerprint"]), cleanAnyString(row["source_hash"]))
	targetRevision := firstNonEmpty(cleanAnyString(target["source_revision"]), cleanAnyString(target["source_fingerprint"]), cleanAnyString(target["source_hash"]))
	if rowRevision != "" && targetRevision != "" && rowRevision != targetRevision {
		return true
	}
	rowRenderRevision := cleanAnyString(row["render_revision"])
	targetRenderRevision := cleanAnyString(target["render_revision"])
	if bridgeFeatureUsesRenderRevision(row) && rowRenderRevision != "" && targetRenderRevision != "" && rowRenderRevision != targetRenderRevision {
		return true
	}
	rowPath := firstNonEmpty(cleanAnyString(row["file_path"]), cleanAnyString(row["source_path"]), cleanAnyString(row["current_source_path"]))
	targetPath := firstNonEmpty(cleanAnyString(target["source_path"]), cleanAnyString(target["file_path"]), cleanAnyString(target["current_source_path"]))
	return rowPath != "" && targetPath != "" && !pathsMatchForObservation(rowPath, targetPath)
}

func featureRowTargetScore(row map[string]any, target map[string]any) int {
	if len(row) == 0 || len(target) == 0 {
		return 0
	}
	score := 0
	for _, key := range []string{"track_id", "clip_id"} {
		rowValue := cleanAnyString(row[key])
		targetValue := cleanAnyString(target[key])
		if key == "clip_id" && targetValue == "" {
			targetValue = cleanAnyString(target["id"])
		}
		if rowValue != "" && targetValue != "" && rowValue == targetValue {
			score += 2
		}
	}
	rowRevision := firstNonEmpty(cleanAnyString(row["source_revision"]), cleanAnyString(row["source_fingerprint"]), cleanAnyString(row["source_hash"]))
	targetRevision := firstNonEmpty(cleanAnyString(target["source_revision"]), cleanAnyString(target["source_fingerprint"]), cleanAnyString(target["source_hash"]))
	if rowRevision != "" && targetRevision != "" && rowRevision == targetRevision {
		score++
	}
	rowRenderRevision := cleanAnyString(row["render_revision"])
	targetRenderRevision := cleanAnyString(target["render_revision"])
	if bridgeFeatureUsesRenderRevision(row) && rowRenderRevision != "" && targetRenderRevision != "" && rowRenderRevision == targetRenderRevision {
		score++
	}
	rowPath := firstNonEmpty(cleanAnyString(row["file_path"]), cleanAnyString(row["source_path"]), cleanAnyString(row["current_source_path"]))
	targetPath := firstNonEmpty(cleanAnyString(target["source_path"]), cleanAnyString(target["file_path"]), cleanAnyString(target["current_source_path"]))
	if rowPath != "" && targetPath != "" && pathsMatchForObservation(rowPath, targetPath) {
		score++
	}
	return score
}

func featureRowProjectionCompleteness(row map[string]any) int {
	if len(row) == 0 {
		return 0
	}
	score := 0
	score += len(mapValue(row["bands"])) * 4
	for _, key := range []string{"left_level_db", "right_level_db", "balance_db", "balance_state", "correlation_estimate", "correlation_state", "quality_status", "quality_reason", "capture_mode", "tap_point", "updated_at", "source_revision", "clip_revision", "render_revision"} {
		if value, ok := row[key]; ok && !isEmptyFeatureValue(value) {
			score++
		}
	}
	if len(mapValue(row["quality_evidence"])) > 0 {
		score += 3
	}
	// Fine L3 evidence is materially more complete than a later whole-window
	// refresh of the same source. Preserve it for DOM family-selection views;
	// freshness and material identity are still checked before this tie-break.
	for _, key := range []string{"band_dynamics", "noise_floor_evidence", "frequency_time_events", "transient_events"} {
		if len(mapValue(row[key])) > 0 {
			score += 8
		}
	}
	return score
}

func compactAcousticPackageStatus(status map[string]any) map[string]any {
	if len(status) == 0 || cleanAnyString(status["schema_version"]) != "acoustic_package_status.v0" {
		return nil
	}
	out := map[string]any{}
	for _, key := range []string{"schema_version", "status", "project_id", "session_id", "track_id", "clip_id", "source_hash", "source_fingerprint", "source_revision", "clip_revision", "render_revision", "analyzer_revision", "source_path", "duration_seconds", "clip_start_seconds", "source_identity", "package_layers", "updated_at", "audit"} {
		if value, ok := status[key]; ok && !isEmptyFeatureValue(value) {
			out[key] = value
		}
	}
	return out
}

func acousticPackageMatchesLatestRequest(status map[string]any, latestRequest map[string]any) bool {
	if len(status) == 0 || len(latestRequest) == 0 {
		return true
	}
	target := mapValue(latestRequest["resolved_target"])
	if len(target) == 0 {
		return true
	}
	identity := mapValue(status["source_identity"])
	for _, key := range []string{"track_id", "clip_id"} {
		statusValue := firstNonEmpty(cleanAnyString(status[key]), cleanAnyString(identity[key]))
		targetValue := cleanAnyString(target[key])
		if key == "clip_id" && targetValue == "" {
			targetValue = cleanAnyString(target["id"])
		}
		if statusValue != "" && targetValue != "" && statusValue != targetValue {
			return false
		}
	}
	statusSource := firstNonEmpty(cleanAnyString(status["source_path"]), cleanAnyString(identity["source_path"]), cleanAnyString(identity["file_path"]))
	targetSource := firstNonEmpty(cleanAnyString(target["source_path"]), cleanAnyString(target["file_path"]), cleanAnyString(target["current_source_path"]))
	if statusSource != "" && targetSource != "" && !pathsMatchForObservation(statusSource, targetSource) {
		return false
	}
	statusRevision := firstNonEmpty(cleanAnyString(status["source_revision"]), cleanAnyString(status["source_fingerprint"]), cleanAnyString(identity["source_revision"]), cleanAnyString(identity["source_fingerprint"]))
	targetRevision := firstNonEmpty(cleanAnyString(target["source_revision"]), cleanAnyString(target["source_fingerprint"]))
	if statusRevision != "" && targetRevision != "" && statusRevision != targetRevision {
		return false
	}
	return true
}

func pathsMatchForObservation(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return true
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func featureRowFromAcousticPackage(status map[string]any, layerName, featureName, featureType, latestRequestID string) map[string]any {
	feature := acousticPackageFeature(status, layerName, featureName)
	if len(feature) == 0 {
		return nil
	}
	featureStatus := cleanAnyString(feature["status"])
	row := map[string]any{}
	if featureStatus == "ready" || featureStatus == "partial" || featureStatus == "suspect" {
		row = mapValue(feature["ref"])
		if len(row) == 0 {
			row = map[string]any{}
		}
	}
	row["status"] = featureStatus
	row["feature_type"] = featureType
	if source := cleanAnyString(feature["source"]); source != "" {
		row["source"] = source
	}
	if reason := firstNonEmpty(cleanAnyString(feature["reason"]), cleanAnyString(mapValue(feature["progress"])["reason"])); reason != "" {
		row["reason"] = reason
	}
	if updated := firstNonEmpty(cleanAnyString(feature["updated_at"]), cleanAnyString(mapValue(feature["progress"])["updated_at"]), cleanAnyString(status["updated_at"])); updated != "" {
		row["updated_at"] = updated
	}
	for _, key := range []string{"project_id", "session_id", "track_id", "clip_id", "source_hash", "source_fingerprint", "source_revision", "clip_revision", "render_revision", "analyzer_revision", "source_path", "duration_seconds", "clip_start_seconds"} {
		if value := cleanAnyString(status[key]); value != "" {
			targetKey := key
			if key == "source_path" {
				targetKey = "file_path"
			}
			if key == "duration_seconds" {
				row[targetKey] = status[key]
				continue
			}
			if featureStatus != "ready" && featureStatus != "partial" {
				row[targetKey] = value
			} else if cleanAnyString(row[targetKey]) == "" {
				row[targetKey] = value
			}
		}
	}
	if latestRequestID != "" && featureStatus != "ready" && featureStatus != "partial" && featureStatus != "suspect" {
		row["request_id"] = latestRequestID
	}
	progress := mapValue(feature["progress"])
	for _, key := range []string{"tile_count_seen", "tile_count_expected", "coverage_seconds", "coverage_ratio"} {
		if value, ok := progress[key]; ok && !isEmptyFeatureValue(value) {
			row[key] = value
		}
	}
	return row
}

func acousticPackageFeature(status map[string]any, layerName, featureName string) map[string]any {
	layers := mapValue(status["package_layers"])
	layer := mapValue(layers[layerName])
	features := mapValue(layer["features"])
	return mapValue(features[featureName])
}

func applyAcousticPackageCapabilities(caps map[string]string, status map[string]any) {
	if len(caps) == 0 || len(status) == 0 {
		return
	}
	for key, ref := range map[string][2]string{
		"waveform_envelope":        {"l1_static", "waveform_envelope"},
		"spectrogram_tiles":        {"l3_deep", "spectrogram_tiles"},
		"band_energy":              {"l3_deep", "band_energy_summary"},
		"stereo_relation":          {"l3_deep", "stereo_relation_summary"},
		"loudness_summary":         {"l3_deep", "loudness_summary"},
		"realtime_band_energy":     {"l2_realtime", "realtime_spectrum"},
		"realtime_stereo_relation": {"l2_realtime", "realtime_stereo_correlation"},
		"l2_render_probe":          {"l2_realtime", "render_probe"},
		"post_fx_probe":            {"l2_realtime", "render_probe"},
		"masking_analysis":         {"l3_deep", "masking_analysis"},
		"reference_match":          {"l3_deep", "reference_match"},
		"lufs_analysis":            {"l3_deep", "lufs_analysis"},
	} {
		if feature := acousticPackageFeature(status, ref[0], ref[1]); len(feature) > 0 {
			if value := cleanAnyString(feature["status"]); value != "" {
				if featureStatusRank(value) > featureStatusRank(caps[key]) {
					caps[key] = value
				}
			}
		}
	}
}

func shouldReplaceFeatureRow(existing, candidate map[string]any) bool {
	if len(candidate) == 0 {
		return false
	}
	return featureStatusRank(featureStatus(candidate)) > featureStatusRank(featureStatus(existing))
}

func featureStatusRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready":
		return 7
	case "partial":
		return 6
	case "building", "requested":
		return 5
	case "suspect":
		return 4
	case "stale":
		return 3
	case "failed", "invalid", "blocked", "unavailable":
		return 2
	case "missing":
		return 1
	case "deferred":
		return 0
	default:
		return 0
	}
}

func mergeAcousticPackageTrackWaveformRows(rows []map[string]any, row map[string]any) []map[string]any {
	if len(row) == 0 {
		return rows
	}
	trackID := cleanAnyString(row["track_id"])
	clipID := cleanAnyString(row["clip_id"])
	replaced := false
	out := make([]map[string]any, 0, len(rows)+1)
	for _, existing := range rows {
		if cleanAnyString(existing["track_id"]) == trackID && cleanAnyString(existing["clip_id"]) == clipID {
			if shouldReplaceFeatureRow(existing, row) {
				out = append(out, row)
			} else {
				out = append(out, existing)
			}
			replaced = true
			continue
		}
		out = append(out, existing)
	}
	if !replaced {
		out = append(out, row)
	}
	return out
}

func preserveTargetWaveformTimeSegments(snap *featureSnapshot, req Request) {
	if snap == nil {
		return
	}
	trackID := cleanAnyString(req.Args["track_id"])
	clipID := cleanAnyString(req.Args["clip_id"])
	switch strings.ToLower(strings.TrimSpace(req.TargetRef.Kind)) {
	case "track":
		trackID = firstNonEmpty(req.TargetRef.ID, trackID)
	case "clip":
		clipID = firstNonEmpty(req.TargetRef.ID, clipID)
	}
	var targetRow map[string]any
	for _, row := range snap.TrackWaveformEnvelopes {
		if waveformRowMatchesRequestedTarget(row, trackID, clipID) && len(waveformTimeSegments(row)) > 0 {
			targetRow = row
			break
		}
	}
	if len(targetRow) == 0 {
		return
	}
	if !waveformRowMatchesRequestedTarget(snap.WaveformEnvelope, trackID, clipID) {
		snap.WaveformEnvelope = copyAnyMap(targetRow)
		return
	}
	if !waveformRowsDescribeSameMaterial(snap.WaveformEnvelope, targetRow) {
		return
	}
	if len(waveformTimeSegments(snap.WaveformEnvelope)) > 0 {
		return
	}
	merged := copyAnyMap(snap.WaveformEnvelope)
	merged["time_segments"] = targetRow["time_segments"]
	for _, key := range []string{"total_duration", "duration_seconds", "coverage_seconds", "coverage_ratio"} {
		if _, present := merged[key]; !present && targetRow[key] != nil {
			merged[key] = targetRow[key]
		}
	}
	snap.WaveformEnvelope = merged
}

func waveformRowMatchesRequestedTarget(row map[string]any, trackID, clipID string) bool {
	if trackID != "" && cleanAnyString(row["track_id"]) != trackID {
		return false
	}
	if clipID != "" && cleanAnyString(row["clip_id"]) != clipID {
		return false
	}
	return trackID != "" || clipID != ""
}

func waveformRowsDescribeSameMaterial(a, b map[string]any) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	matchedIdentity := false
	for _, key := range []string{"track_id", "clip_id", "source_revision", "source_fingerprint", "clip_revision"} {
		left, right := cleanAnyString(a[key]), cleanAnyString(b[key])
		if left == "" || right == "" {
			continue
		}
		matchedIdentity = true
		if !strings.EqualFold(left, right) {
			return false
		}
	}
	return matchedIdentity
}

func mergeFeatureRowsByIdentity(rows []map[string]any, row map[string]any) []map[string]any {
	if len(row) == 0 {
		return rows
	}
	key := featureIdentityKey(row)
	if key == "" {
		key = fmt.Sprintf("row_%d", len(rows))
	}
	out := make([]map[string]any, 0, len(rows)+1)
	replaced := false
	for _, existing := range rows {
		if featureIdentityKey(existing) == key {
			if shouldReplaceFeatureRow(existing, row) || cleanAnyString(existing["request_id"]) == cleanAnyString(row["request_id"]) {
				out = append(out, row)
			} else {
				out = append(out, existing)
			}
			replaced = true
			continue
		}
		out = append(out, existing)
	}
	if !replaced {
		out = append(out, row)
	}
	return out
}

func featureIdentityKey(row map[string]any) string {
	if len(row) == 0 {
		return ""
	}
	parts := []string{
		cleanAnyString(row["feature_type"]),
		cleanAnyString(row["project_id"]),
		cleanAnyString(row["session_id"]),
		cleanAnyString(row["track_id"]),
		cleanAnyString(row["clip_id"]),
		cleanAnyString(row["clip_revision"]),
		cleanAnyString(row["source_revision"]),
		cleanAnyString(row["source_hash"]),
		strings.ToLower(filepath.Clean(firstNonEmpty(cleanAnyString(row["file_path"]), cleanAnyString(row["source_path"]), cleanAnyString(row["current_source_path"])))),
	}
	has := false
	for _, part := range parts {
		if strings.TrimSpace(part) != "" && strings.TrimSpace(part) != "." {
			has = true
			break
		}
	}
	if !has {
		return ""
	}
	return strings.Join(parts, "::")
}

func featureStatus(row map[string]any) string {
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(row["status"])))
	switch status {
	case "ready", "partial", "building", "stale", "suspect", "missing", "deferred", "failed", "requested", "blocked", "unavailable", "invalid":
		return status
	case "", "<nil>":
		return "missing"
	default:
		return status
	}
}

func postFXProbeCapability(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "partial", "building", "stale", "suspect":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "unavailable"
	}
}

func normalizeBridgeFeatureFreshness(snap *featureSnapshot) {
	if snap == nil || len(snap.LatestRequest) == 0 {
		return
	}
	requestID := cleanAnyString(snap.LatestRequest["request_id"])
	if requestID == "" {
		return
	}
	target, _ := snap.LatestRequest["resolved_target"].(map[string]any)
	snap.SpectrogramTiles = freshBridgeRowOrMissing(snap.SpectrogramTiles, requestID, target, "spectral_field")
	snap.BandEnergySummary = freshBridgeRowOrMissing(snap.BandEnergySummary, requestID, target, "band_energy_summary")
	snap.StereoRelationSummary = freshBridgeRowOrMissing(snap.StereoRelationSummary, requestID, target, "stereo_relation_summary")
	snap.LoudnessSummary = freshBridgeRowOrMissing(snap.LoudnessSummary, requestID, target, "loudness_summary")
}

func normalizeTrackWaveformFeatureFreshness(snap *featureSnapshot, projectState map[string]any, allowLegacyProjectRows bool) {
	if snap == nil || len(snap.LatestRequest) == 0 {
		return
	}
	requestID := cleanAnyString(snap.LatestRequest["request_id"])
	if requestID == "" {
		return
	}
	if allowLegacyProjectRows && trackWaveformLatestRequestIsNonAuthoritativeBlocked(snap.LatestRequest) {
		return
	}
	target, _ := snap.LatestRequest["resolved_target"].(map[string]any)
	snap.WaveformEnvelope = freshWaveformRowOrMissing(snap.WaveformEnvelope, requestID, target, snap.LatestRequest, projectState)
	rows := make([]map[string]any, 0, len(snap.TrackWaveformEnvelopes))
	for _, row := range snap.TrackWaveformEnvelopes {
		if allowLegacyProjectRows && featureRowHasMaterialIdentity(row) && trackWaveformRowMatchesProjectState(row, projectState) {
			rows = append(rows, row)
			continue
		}
		if trackWaveformRowMatchesCurrentRequest(row, requestID, target, snap.LatestRequest, projectState) {
			rows = append(rows, row)
		}
	}
	snap.TrackWaveformEnvelopes = rows
}

func listenScopeAllowsLegacyProjectWaveformRows(scope ListenScope) bool {
	switch strings.ToLower(strings.TrimSpace(scope.Source.Mode)) {
	case "full_project", "full_project_with_focus_track", "track_group":
		return true
	default:
		return false
	}
}

func trackWaveformLatestRequestIsNonAuthoritativeBlocked(latestRequest map[string]any) bool {
	if len(latestRequest) == 0 {
		return false
	}
	status := strings.ToLower(cleanAnyString(latestRequest["status"]))
	reason := strings.ToLower(cleanAnyString(latestRequest["reason"]))
	if status != "blocked" || reason != "clip_source_required_for_current_feature_bakers" {
		return false
	}
	return len(mapRowsAny(latestRequest["requested_features"])) == 0 && len(mapRowsAny(latestRequest["track_feature_targets"])) == 0
}

func freshWaveformRowOrMissing(row map[string]any, requestID string, target map[string]any, latestRequest map[string]any, projectState map[string]any) map[string]any {
	status := featureStatus(row)
	if status == "missing" || status == "invalid" {
		return row
	}
	if trackWaveformRowMatchesCurrentRequest(row, requestID, target, latestRequest, projectState) {
		return row
	}
	out := map[string]any{
		"status":       "missing",
		"feature_type": "waveform_envelope",
		"request_id":   requestID,
		"reason":       "stale_feature_snapshot_for_current_request",
	}
	if trackID := cleanAnyString(target["track_id"]); trackID != "" {
		out["track_id"] = trackID
	}
	if clipID := cleanAnyString(target["clip_id"]); clipID != "" {
		out["clip_id"] = clipID
	}
	return out
}

func trackWaveformRowMatchesCurrentRequest(row map[string]any, requestID string, target map[string]any, latestRequest map[string]any, projectState map[string]any) bool {
	if len(row) == 0 || requestID == "" {
		return false
	}
	rowRequestID := cleanAnyString(row["request_id"])
	if rowRequestID != "" && !trackWaveformRequestIDMatches(rowRequestID, requestID, row, latestRequest) {
		if !featureRowHasMaterialIdentity(row) {
			return false
		}
	}
	if !trackWaveformRowMatchesLatestTarget(row, requestID, target, latestRequest) {
		return false
	}
	return trackWaveformRowMatchesProjectState(row, projectState)
}

func trackWaveformRequestIDMatches(rowRequestID string, requestID string, row map[string]any, latestRequest map[string]any) bool {
	if rowRequestID == requestID {
		return true
	}
	for _, requested := range mapRowsAny(latestRequest["requested_features"]) {
		featureType := cleanAnyString(requested["feature_type"])
		if featureType != "" && featureType != "waveform_envelope" {
			continue
		}
		if cleanAnyString(requested["request_id"]) != rowRequestID {
			continue
		}
		if sameTrackClip(row, requested) {
			return true
		}
	}
	return false
}

func trackWaveformRowMatchesLatestTarget(row map[string]any, requestID string, target map[string]any, latestRequest map[string]any) bool {
	targets := mapRowsAny(latestRequest["track_feature_targets"])
	if len(targets) > 0 {
		for _, candidate := range targets {
			if sameTrackClip(row, candidate) {
				return true
			}
		}
		return false
	}
	if cleanAnyString(row["request_id"]) == "" && sameTrackClip(row, target) {
		return true
	}
	if featureRowHasMaterialIdentity(row) && sameTrackClip(row, target) {
		return true
	}
	if bridgeRowMatchesRequest(row, requestID, target) {
		return true
	}
	if strings.EqualFold(cleanAnyString(target["kind"]), "project") || strings.EqualFold(cleanAnyString(target["scope"]), "full_project") {
		return true
	}
	return cleanAnyString(target["track_id"]) == "" && cleanAnyString(target["clip_id"]) == ""
}

func sameTrackClip(row map[string]any, target map[string]any) bool {
	rowTrack := cleanAnyString(row["track_id"])
	rowClip := cleanAnyString(row["clip_id"])
	targetTrack := cleanAnyString(target["track_id"])
	targetClip := cleanAnyString(target["clip_id"])
	if targetTrack != "" && rowTrack != targetTrack {
		return false
	}
	if targetClip != "" && rowClip != targetClip {
		return false
	}
	return targetTrack != "" || targetClip != ""
}

func trackWaveformRowMatchesProjectState(row map[string]any, projectState map[string]any) bool {
	tracks := mapRowsAny(projectState["tracks"])
	if len(tracks) == 0 {
		return true
	}
	rowTrack := cleanAnyString(row["track_id"])
	if rowTrack == "" {
		return true
	}
	var track map[string]any
	for _, candidate := range tracks {
		if rowTrack == firstNonEmpty(cleanAnyString(candidate["track_id"]), cleanAnyString(candidate["id"])) {
			track = candidate
			break
		}
	}
	if len(track) == 0 {
		return false
	}
	if !featureRowMatchesProjectIdentity(row, projectState) {
		return false
	}
	rowClip := cleanAnyString(row["clip_id"])
	clips := anySlice(firstPresent(track, "clips", "clip_summaries"))
	if len(clips) == 0 {
		return true
	}
	if rowClip == "" {
		if len(clips) == 1 {
			clip, _ := clips[0].(map[string]any)
			return featureRowMatchesClipMaterial(row, clip)
		}
		return !featureRowHasPathIdentity(row)
	}
	for _, raw := range clips {
		clip, _ := raw.(map[string]any)
		if rowClip == firstNonEmpty(cleanAnyString(clip["clip_id"]), cleanAnyString(clip["id"]), cleanAnyString(clip["item_id"])) {
			return featureRowMatchesClipMaterial(row, clip)
		}
	}
	return false
}

func normalizeProjectFeatureMaterialFreshness(snap *featureSnapshot, projectState map[string]any) {
	if snap == nil || len(mapRowsAny(projectState["tracks"])) == 0 {
		return
	}
	filter := func(rows []map[string]any) []map[string]any {
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if trackWaveformRowMatchesProjectState(row, projectState) {
				out = append(out, row)
			}
		}
		return out
	}
	snap.TrackWaveformEnvelopes = filter(snap.TrackWaveformEnvelopes)
	snap.SpectrogramTileRows = filter(snap.SpectrogramTileRows)
	snap.BandEnergySummaries = filter(snap.BandEnergySummaries)
	snap.StereoRelationSummaries = filter(snap.StereoRelationSummaries)
	snap.LoudnessSummaries = filter(snap.LoudnessSummaries)
	snap.RealtimeBandEnergySummaries = filter(snap.RealtimeBandEnergySummaries)
	snap.RealtimeStereoRelationSummaries = filter(snap.RealtimeStereoRelationSummaries)
	snap.L2RenderProbes = filter(snap.L2RenderProbes)
	snap.WaveformEnvelope = currentProjectFeatureRowOrMissing(snap.WaveformEnvelope, projectState, "waveform_envelope")
	snap.SpectrogramTiles = currentProjectFeatureRowOrMissing(snap.SpectrogramTiles, projectState, "spectral_field")
	snap.BandEnergySummary = currentProjectFeatureRowOrMissing(snap.BandEnergySummary, projectState, "band_energy_summary")
	snap.StereoRelationSummary = currentProjectFeatureRowOrMissing(snap.StereoRelationSummary, projectState, "stereo_relation_summary")
	snap.LoudnessSummary = currentProjectFeatureRowOrMissing(snap.LoudnessSummary, projectState, "loudness_summary")
	snap.RealtimeBandEnergySummary = currentProjectFeatureRowOrMissing(snap.RealtimeBandEnergySummary, projectState, "realtime_band_energy_summary")
	snap.RealtimeStereoRelationSummary = currentProjectFeatureRowOrMissing(snap.RealtimeStereoRelationSummary, projectState, "realtime_stereo_relation_summary")
	snap.L2RenderProbe = currentProjectFeatureRowOrMissing(snap.L2RenderProbe, projectState, "l2_render_probe")
}

func currentProjectFeatureRowOrMissing(row map[string]any, projectState map[string]any, featureType string) map[string]any {
	if len(row) == 0 || cleanAnyString(row["track_id"]) == "" || trackWaveformRowMatchesProjectState(row, projectState) {
		return row
	}
	out := map[string]any{
		"status":       "missing",
		"feature_type": featureType,
		"reason":       "project_material_identity_mismatch",
	}
	for _, key := range []string{"request_id", "track_id", "clip_id"} {
		if value := cleanAnyString(row[key]); value != "" {
			out[key] = value
		}
	}
	return out
}

func featureRowMatchesProjectIdentity(row map[string]any, projectState map[string]any) bool {
	current := firstNonEmpty(
		cleanAnyString(projectState["project_id"]),
		cleanAnyString(projectState["project_uuid"]),
	)
	identity := mapValue(row["source_identity"])
	rowProject := firstNonEmpty(cleanAnyString(row["project_id"]), cleanAnyString(identity["project_id"]))
	if !concreteProjectIdentity(current) || !concreteProjectIdentity(rowProject) {
		return true
	}
	return strings.EqualFold(current, rowProject)
}

func concreteProjectIdentity(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value != "" && value != "current" && value != "project_current" && value != "project"
}

func featureRowMatchesClipMaterial(row map[string]any, clip map[string]any) bool {
	currentPath := normalizedFeatureMaterialPath(firstNonEmpty(
		cleanAnyString(clip["current_source_path"]),
		cleanAnyString(clip["source_path"]),
		cleanAnyString(clip["file_path"]),
	))
	if currentPath == "" {
		return true
	}
	if rowPath := normalizedFeatureMaterialPath(featureRowSourcePath(row)); rowPath != "" {
		return rowPath == currentPath
	}
	for _, key := range []string{"source_revision", "source_fingerprint", "clip_revision"} {
		identity := normalizedFeatureMaterialIdentity(cleanAnyString(row[key]))
		if identity == "" {
			continue
		}
		if strings.Contains(identity, currentPath) {
			return true
		}
		if featureMaterialIdentityContainsPath(identity) {
			return false
		}
	}
	identity := mapValue(row["source_identity"])
	for _, key := range []string{"source_revision", "source_fingerprint", "clip_revision"} {
		value := normalizedFeatureMaterialIdentity(cleanAnyString(identity[key]))
		if value == "" {
			continue
		}
		if strings.Contains(value, currentPath) {
			return true
		}
		if featureMaterialIdentityContainsPath(value) {
			return false
		}
	}
	return true
}

func featureRowHasPathIdentity(row map[string]any) bool {
	if normalizedFeatureMaterialPath(featureRowSourcePath(row)) != "" {
		return true
	}
	identity := mapValue(row["source_identity"])
	for _, source := range []map[string]any{row, identity} {
		for _, key := range []string{"source_revision", "source_fingerprint", "clip_revision"} {
			if featureMaterialIdentityContainsPath(normalizedFeatureMaterialIdentity(cleanAnyString(source[key]))) {
				return true
			}
		}
	}
	return false
}

func featureRowSourcePath(row map[string]any) string {
	identity := mapValue(row["source_identity"])
	return firstNonEmpty(
		cleanAnyString(row["source_path"]),
		cleanAnyString(row["file_path"]),
		cleanAnyString(identity["source_path"]),
		cleanAnyString(identity["file_path"]),
		cleanAnyString(identity["source_id"]),
	)
}

func normalizedFeatureMaterialPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = filepath.ToSlash(filepath.Clean(value))
	return strings.ToLower(value)
}

func normalizedFeatureMaterialIdentity(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
}

func featureMaterialIdentityContainsPath(value string) bool {
	return strings.Contains(value, ":/") || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//")
}

func freshBridgeRowOrMissing(row map[string]any, requestID string, target map[string]any, featureType string) map[string]any {
	status := featureStatus(row)
	if status != "ready" && status != "partial" {
		if bridgeRowMatchesRequest(row, requestID, target) {
			return row
		}
		if sameTrackClip(row, target) || (len(target) == 0 && len(row) > 0) {
			return currentBridgeStatusRow(row, requestID, target, featureType)
		}
		return currentBridgeMissingRow(requestID, target, featureType, "stale_feature_snapshot_for_current_request")
	}
	if bridgeRowMatchesRequest(row, requestID, target) {
		return row
	}
	if featureRowHasMaterialIdentity(row) && sameTrackClip(row, target) {
		return materialBridgeStatusRow(row, requestID, target, featureType)
	}
	return currentBridgeMissingRow(requestID, target, featureType, "stale_feature_snapshot_for_current_request")
}

func currentBridgeMissingRow(requestID string, target map[string]any, featureType string, reason string) map[string]any {
	out := map[string]any{
		"status":       "missing",
		"feature_type": featureType,
		"request_id":   requestID,
		"reason":       reason,
	}
	if trackID := cleanAnyString(target["track_id"]); trackID != "" {
		out["track_id"] = trackID
	}
	if clipID := cleanAnyString(target["clip_id"]); clipID != "" {
		out["clip_id"] = clipID
	}
	return out
}

func currentBridgeStatusRow(row map[string]any, requestID string, target map[string]any, featureType string) map[string]any {
	out := make(map[string]any, len(row)+4)
	for key, value := range row {
		out[key] = value
	}
	status := featureStatus(row)
	out["status"] = status
	out["feature_type"] = featureType
	out["request_id"] = requestID
	if status == "missing" && cleanAnyString(out["reason"]) == "" {
		out["reason"] = "stale_feature_snapshot_for_current_request"
	}
	if trackID := cleanAnyString(target["track_id"]); trackID != "" {
		out["track_id"] = trackID
	}
	if clipID := cleanAnyString(target["clip_id"]); clipID != "" {
		out["clip_id"] = clipID
	}
	return out
}

func materialBridgeStatusRow(row map[string]any, requestID string, target map[string]any, featureType string) map[string]any {
	out := make(map[string]any, len(row)+4)
	for key, value := range row {
		out[key] = value
	}
	out["status"] = featureStatus(row)
	out["feature_type"] = featureType
	rowRequestID := cleanAnyString(out["request_id"])
	if requestID != "" && (rowRequestID == "" || materialBridgeRequestIDBelongsToLatest(rowRequestID, requestID, target)) {
		out["request_id"] = requestID
	}
	if trackID := cleanAnyString(target["track_id"]); trackID != "" {
		out["track_id"] = trackID
	}
	if clipID := cleanAnyString(target["clip_id"]); clipID != "" {
		out["clip_id"] = clipID
	}
	return out
}

func materialBridgeRequestIDBelongsToLatest(rowRequestID string, latestRequestID string, target map[string]any) bool {
	rowRequestID = strings.TrimSpace(rowRequestID)
	latestRequestID = strings.TrimSpace(latestRequestID)
	if rowRequestID == "" || latestRequestID == "" || rowRequestID == latestRequestID {
		return false
	}
	if !strings.HasPrefix(rowRequestID, "kernel_prepared_") || !strings.HasPrefix(latestRequestID, "kernel_prepared_") {
		return false
	}
	suffix := safeKernelPreparedRequestIDSuffix(target)
	return suffix != "" && strings.HasSuffix(rowRequestID, "_"+suffix) && strings.HasSuffix(latestRequestID, "_"+suffix)
}

func safeKernelPreparedRequestIDSuffix(target map[string]any) string {
	for _, value := range []string{cleanAnyString(target["clip_id"]), cleanAnyString(target["track_id"])} {
		value = safeBridgeRequestIDPart(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func safeBridgeRequestIDPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return strings.Trim(b.String(), "_")
}

func featureRowHasMaterialIdentity(row map[string]any) bool {
	return cleanAnyString(row["source_revision"]) != "" ||
		cleanAnyString(row["source_fingerprint"]) != "" ||
		cleanAnyString(row["source_hash"]) != "" ||
		cleanAnyString(row["clip_revision"]) != "" ||
		cleanAnyString(row["source_path"]) != "" ||
		cleanAnyString(row["file_path"]) != ""
}

func bridgeFeatureUsesRenderRevision(row map[string]any) bool {
	featureType := strings.ToLower(strings.TrimSpace(firstNonEmpty(cleanAnyString(row["feature_type"]), cleanAnyString(row["type"]))))
	switch featureType {
	case "spectral_field", "spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary", "l3_acoustic_summary":
		return false
	}
	source := strings.ToLower(cleanAnyString(row["source"]))
	if strings.Contains(source, "kernel_l3_offline_analyzer") || strings.Contains(source, "spectral_tile_derived") {
		return false
	}
	return true
}

func bridgeRowMatchesRequest(row map[string]any, requestID string, target map[string]any) bool {
	if len(row) == 0 || requestID == "" {
		return false
	}
	if rowRequestID := cleanAnyString(row["request_id"]); rowRequestID != requestID {
		return false
	}
	if strings.EqualFold(cleanAnyString(row["scope"]), "full_project") {
		return true
	}
	if len(target) == 0 {
		return true
	}
	targetTrack := cleanAnyString(target["track_id"])
	targetClip := cleanAnyString(target["clip_id"])
	rowTrack := cleanAnyString(row["track_id"])
	rowClip := cleanAnyString(row["clip_id"])
	if targetTrack != "" && rowTrack == "" {
		return false
	}
	if targetTrack != "" && rowTrack != targetTrack {
		return false
	}
	if targetClip != "" && rowClip == "" {
		return false
	}
	if targetClip != "" && rowClip != targetClip {
		return false
	}
	return true
}

func trackFeatureRowsStatus(rows []map[string]any) string {
	if len(rows) == 0 {
		return "missing"
	}
	ready := 0
	partial := 0
	requested := 0
	blocked := 0
	building := 0
	stale := 0
	for _, row := range rows {
		switch featureStatus(row) {
		case "ready":
			ready++
		case "partial":
			partial++
		case "building", "requested":
			building++
			requested++
		case "stale":
			stale++
		case "failed", "blocked", "unavailable", "invalid":
			blocked++
		}
	}
	switch {
	case ready == len(rows):
		return "ready"
	case ready > 0 || partial > 0:
		return "partial"
	case building > 0 || requested > 0:
		return "building"
	case stale > 0:
		return "stale"
	case blocked > 0:
		return "blocked"
	default:
		return "missing"
	}
}

func compactFeatureSnapshot(snap featureSnapshot) map[string]any {
	latestRequest := compactFeatureRequest(snap.LatestRequest)
	if len(latestRequest) == 0 {
		latestRequest = inferredLatestRequestFromFeatureRows(snap)
	}
	return map[string]any{
		"schema_version":                     snap.SchemaVersion,
		"updated_at":                         snap.UpdatedAt,
		"latest_request":                     latestRequest,
		"waveform_envelope":                  compactFeatureRow(snap.WaveformEnvelope),
		"track_waveform_envelopes":           compactTrackFeatureRows(snap.TrackWaveformEnvelopes),
		"spectrogram_tiles":                  compactFeatureRow(snap.SpectrogramTiles),
		"spectrogram_tile_rows":              compactTrackFeatureRows(snap.SpectrogramTileRows),
		"band_energy_summary":                compactFeatureRow(snap.BandEnergySummary),
		"band_energy_summaries":              compactTrackFeatureRows(snap.BandEnergySummaries),
		"stereo_relation_summary":            compactFeatureRow(snap.StereoRelationSummary),
		"stereo_relation_summaries":          compactTrackFeatureRows(snap.StereoRelationSummaries),
		"loudness_summary":                   compactFeatureRow(snap.LoudnessSummary),
		"loudness_summaries":                 compactTrackFeatureRows(snap.LoudnessSummaries),
		"realtime_band_energy_summary":       compactFeatureRow(snap.RealtimeBandEnergySummary),
		"realtime_band_energy_summaries":     compactTrackFeatureRows(snap.RealtimeBandEnergySummaries),
		"realtime_stereo_relation_summary":   compactFeatureRow(snap.RealtimeStereoRelationSummary),
		"realtime_stereo_relation_summaries": compactTrackFeatureRows(snap.RealtimeStereoRelationSummaries),
		"l2_render_probe":                    compactFeatureRow(snap.L2RenderProbe),
		"l2_render_probes":                   compactTrackFeatureRows(snap.L2RenderProbes),
		"masking_measurement":                compactMaskingMeasurement(snap.MaskingMeasurement),
		"transient_events":                   snap.TransientEvents,
	}
}

func compactMaskingMeasurement(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	out := compactKeys(row, []string{"schema_version", "measurement_id", "status", "freshness", "model", "project_binding", "conditions", "coverage", "pair_bands", "evidence_refs", "limitations"})
	if pairs := mapRowsAny(out["pair_bands"]); len(pairs) > 96 {
		out["pair_bands"] = pairs[:96]
		coverage := mapValue(out["coverage"])
		coverage["pair_bands_truncated"] = true
		out["coverage"] = coverage
	}
	return out
}

func inferredLatestRequestFromFeatureRows(snap featureSnapshot) map[string]any {
	anchor := firstMaterializedFeatureRowWithRequest(
		snap.SpectrogramTiles,
		snap.BandEnergySummary,
		snap.StereoRelationSummary,
		snap.LoudnessSummary,
		snap.WaveformEnvelope,
	)
	requestID := cleanAnyString(anchor["request_id"])
	if requestID == "" {
		return nil
	}
	target := compactKeys(anchor, []string{"track_id", "clip_id", "source_revision", "source_fingerprint", "source_hash", "source_path", "file_path", "duration_seconds"})
	if len(target) == 0 {
		target = map[string]any{}
	}
	requested := inferredRequestedFeaturesFromFeatureRows(requestID, snap)
	out := map[string]any{
		"schema_version":     "mixboard_feature_request.v1",
		"request_id":         requestID,
		"status":             "materialized",
		"reason":             "inferred_from_feature_rows",
		"source_kind":        "feature_snapshot_rows",
		"resolved_target":    target,
		"requested_features": requested,
	}
	return out
}

func firstMaterializedFeatureRowWithRequest(rows ...map[string]any) map[string]any {
	for _, row := range rows {
		if materializedFeatureRowHasRequest(row) {
			return row
		}
	}
	return nil
}

func materializedFeatureRowHasRequest(row map[string]any) bool {
	if cleanAnyString(row["request_id"]) == "" {
		return false
	}
	switch featureStatus(row) {
	case "ready", "partial", "suspect":
		return true
	default:
		return false
	}
}

func inferredRequestedFeaturesFromFeatureRows(requestID string, snap featureSnapshot) []any {
	out := make([]any, 0, 2)
	if materializedFeatureRowHasRequest(snap.WaveformEnvelope) || len(snap.TrackWaveformEnvelopes) > 0 {
		out = append(out, inferredRequestedFeature("waveform_envelope", requestID, snap.WaveformEnvelope))
	}
	if materializedFeatureRowHasRequest(snap.SpectrogramTiles) ||
		materializedFeatureRowHasRequest(snap.BandEnergySummary) ||
		materializedFeatureRowHasRequest(snap.StereoRelationSummary) ||
		materializedFeatureRowHasRequest(snap.LoudnessSummary) {
		if materializedFeatureRowHasRequest(snap.SpectrogramTiles) {
			out = append(out, inferredRequestedFeature("spectral_field", requestID, snap.SpectrogramTiles))
		}
		if materializedFeatureRowHasRequest(snap.BandEnergySummary) ||
			materializedFeatureRowHasRequest(snap.StereoRelationSummary) ||
			materializedFeatureRowHasRequest(snap.LoudnessSummary) {
			out = append(out, inferredRequestedFeature("l3_acoustic_summary", requestID, firstMaterializedFeatureRowWithRequest(snap.BandEnergySummary, snap.StereoRelationSummary, snap.LoudnessSummary)))
		}
	}
	return out
}

func inferredRequestedFeature(featureType, requestID string, row map[string]any) map[string]any {
	item := map[string]any{
		"feature_type": featureType,
		"request_id":   firstNonEmpty(cleanAnyString(row["request_id"]), requestID),
	}
	for _, key := range []string{"track_id", "clip_id"} {
		if value := cleanAnyString(row[key]); value != "" {
			item[key] = value
		}
	}
	return item
}

func compactFeatureRow(row map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"schema_version", "status", "feature_type", "layer", "source_kind", "track_id", "clip_id", "target", "file_path", "source_path", "source_identity", "source_revision", "source_fingerprint", "source_hash", "clip_revision", "render_revision", "plugin_chain_revision", "fader_revision", "analyzer_revision", "analyzer_version", "clip_start_seconds", "request_id", "reason", "scope", "capture_mode", "tap_point", "render_mode", "tail_seconds", "capture_time", "time_basis", "quality_status", "quality_reason", "quality_reasons", "quality_evidence", "target_count", "track_ids", "float_count", "shm_bytes", "stride", "producer_format", "channels_semantics", "frequency_mapping", "derivation_status", "parse_failure_count", "parse_failures", "tile_count_seen", "tile_count_expected", "tile_count_parsed", "coverage_seconds", "coverage_ratio", "last_tile_index", "tile_duration", "tile_content_start_seconds", "frame_duration_seconds", "total_duration", "duration_seconds", "sample_rate", "channel_count", "channels", "expected_sample_count", "analyzed_sample_count", "nonzero_count", "sum_abs", "max_abs", "nan_count", "inf_count", "analyzed_range", "window_ms", "hop_ms", "evidence_ref", "resolution_frame_width", "resolution_frequency_bins", "first_received_at", "last_received_at", "rms", "peak", "peak_abs", "rms_dbfs", "peak_dbfs", "headroom_db", "crest_factor", "crest_db", "integrated_lufs", "approximate_lufs", "approximate", "algorithm", "updated_at", "time_segments", "source", "bands", "band_count", "band_dynamics", "noise_floor_evidence", "frequency_time_events", "transient_events", "left_unit_energy", "right_unit_energy", "left_level_db", "right_level_db", "balance_db", "balance_unit", "balance_state", "phase_deviation", "phase_negative_ratio", "correlation_estimate", "correlation_state", "bin_count", "sample_count", "phase_sample_count"} {
		if value, ok := row[key]; ok {
			out[key] = value
		}
	}
	return out
}

func compactTrackFeatureRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		out = append(out, compactFeatureRow(row))
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
	if requested, ok := row["requested_features"]; ok && len(mapRowsAny(requested)) > 0 {
		out["requested_features"] = requested
	} else if requested := inferredRequestedFeaturesForMaterializedRequest(row); len(requested) > 0 {
		out["requested_features"] = requested
	}
	return out
}

func inferredRequestedFeaturesForMaterializedRequest(row map[string]any) []any {
	requestID := cleanAnyString(row["request_id"])
	if requestID == "" {
		return nil
	}
	status := strings.ToLower(cleanAnyString(row["status"]))
	if status != "materialized" && status != "ready" {
		return nil
	}
	if !strings.HasPrefix(requestID, "kernel_prepared_waveform_envelope") {
		return nil
	}
	target := mapValue(row["resolved_target"])
	out := make([]any, 0, 2)
	for _, featureType := range []string{"waveform_envelope", "spectral_field"} {
		item := map[string]any{
			"feature_type": featureType,
			"request_id":   requestID,
		}
		if trackID := cleanAnyString(target["track_id"]); trackID != "" {
			item["track_id"] = trackID
		}
		if clipID := cleanAnyString(target["clip_id"]); clipID != "" {
			item["clip_id"] = clipID
		}
		out = append(out, item)
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
		"rms_dbfs":    firstNonNil(row["rms_dbfs"], approxDbfs(rms)),
		"peak_dbfs":   firstNonNil(row["peak_dbfs"], approxDbfs(peak)),
		"headroom_db": row["headroom_db"],
		"crest_db":    row["crest_db"],
	}
	if metrics["headroom_db"] == nil && peak > 0 {
		metrics["headroom_db"] = round3(-20 * math.Log10(peak))
	}
	if metrics["crest_db"] == nil && rms > 0 && peak > 0 {
		metrics["crest_db"] = round3(20 * math.Log10(peak/rms))
	}
	if v, ok := row["file_path"]; ok {
		metrics["file_path"] = v
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
	silenceConfirmed := status == "suspect" && l3BandRowConfirmsDeterministicSilence(row)
	if silenceConfirmed {
		status = "partial"
	}
	out := map[string]any{
		"status": status,
	}
	copyOptionalFeatureFields(out, row, "schema_version", "feature_type", "layer", "source_kind", "reason", "source", "updated_at", "track_id", "clip_id", "target", "request_id", "capture_mode", "tap_point", "capture_time", "time_basis", "quality_status", "quality_reason", "quality_evidence", "source_identity", "source_revision", "clip_revision", "render_revision", "plugin_chain_revision", "fader_revision", "tile_count_seen", "tile_count_expected", "tile_count_parsed", "coverage_seconds", "coverage_ratio", "total_duration", "derivation_status", "silence_confirmed", "silence_reason", "original_quality_status", "noise_floor_evidence", "frequency_time_events", "transient_events", "band_dynamics",
		// Measurement conditions consumed by MOM cross-track comparability
		// (frequencyMeasurementConditions): sample format, analyzer, time
		// window, and render mode must survive the band-energy projection or
		// every multi-track frequency relationship reports incomplete
		// measurement conditions and fails closed.
		"sample_rate", "channel_count", "channels", "analyzer_version", "analyzer_revision", "window_ms", "hop_ms", "render_mode", "analyzed_range", "duration_seconds")
	if silenceConfirmed {
		out["silence_confirmed"] = true
		out["silence_reason"] = "source_file_all_zero"
		out["original_quality_status"] = "suspect"
	}
	if status != "ready" && status != "partial" {
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
			"status":            firstNonEmpty(cleanAnyString(bandRow["status"]), status),
			"unit_energy":       nullablePositive(numberFromMap(bandRow, "unit_energy")),
			"energy_db":         firstNonNil(bandRow["energy_db"], approxDbfs(numberFromMap(bandRow, "unit_energy"))),
			"min_hz":            numberFromMap(bandRow, "min_hz"),
			"max_hz":            numberFromMap(bandRow, "max_hz"),
			"sample_count":      firstNonNil(bandRow["sample_count"], nil),
			"left_unit_energy":  firstNonNil(bandRow["left_unit_energy"], nil),
			"right_unit_energy": firstNonNil(bandRow["right_unit_energy"], nil),
		}
	}
	out["bands"] = bands
	return out
}

func l3BandRowConfirmsDeterministicSilence(row map[string]any) bool {
	reason := firstNonEmpty(cleanAnyString(row["quality_reason"]), cleanAnyString(row["reason"]))
	if !strings.EqualFold(reason, "all_zero_audio") && !strings.EqualFold(reason, "all_zero_source") && !strings.EqualFold(reason, "input_all_zero") {
		return false
	}
	return numberFromMap(row, "coverage_ratio") >= 0.999 && len(mapValue(row["bands"])) > 0
}

func buildStereoRelationSummary(row map[string]any) map[string]any {
	status := featureStatus(row)
	out := map[string]any{
		"status": status,
	}
	copyOptionalFeatureFields(out, row, "schema_version", "feature_type", "layer", "source_kind", "reason", "source", "updated_at", "track_id", "clip_id", "target", "request_id", "capture_mode", "tap_point", "capture_time", "time_basis", "quality_status", "quality_reason", "quality_evidence", "source_identity", "source_revision", "clip_revision", "render_revision", "plugin_chain_revision", "fader_revision", "tile_count_seen", "tile_count_expected", "tile_count_parsed", "coverage_seconds", "coverage_ratio", "total_duration", "derivation_status")
	if status != "ready" && status != "partial" {
		return out
	}
	for _, key := range []string{"source", "updated_at", "track_id", "clip_id", "request_id", "balance_state", "correlation_state"} {
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

func buildLoudnessSummary(row map[string]any) map[string]any {
	status := featureStatus(row)
	out := map[string]any{
		"status": status,
	}
	copyOptionalFeatureFields(out, row,
		"schema_version", "feature_type", "layer", "source_kind", "reason", "source", "updated_at",
		"track_id", "clip_id", "target", "request_id", "quality_status", "quality_reason",
		"quality_reasons", "quality_evidence", "source_identity", "source_revision", "clip_revision",
		"render_revision", "analyzer_revision", "analyzer_version", "coverage_seconds", "coverage_ratio",
		"duration_seconds", "total_duration", "sample_rate", "channel_count", "channels",
		"expected_sample_count", "analyzed_sample_count", "nonzero_count", "sum_abs", "max_abs",
		"nan_count", "inf_count", "evidence_ref", "algorithm", "approximate")
	if status != "ready" && status != "partial" && status != "suspect" {
		return out
	}
	for _, key := range []string{"peak", "peak_abs", "peak_dbfs", "rms", "rms_dbfs", "integrated_lufs", "approximate_lufs", "crest_factor", "crest_db"} {
		if value, ok := row[key]; ok && !isEmptyFeatureValue(value) {
			out[key] = round3(numberFromMap(row, key))
		}
	}
	return out
}

func buildL2RenderProbeSummary(row map[string]any) map[string]any {
	status := featureStatus(row)
	out := map[string]any{
		"status": status,
	}
	copyOptionalFeatureFields(out, row,
		"schema_version", "feature_type", "layer", "source_kind", "reason", "source", "updated_at",
		"track_id", "clip_id", "target", "request_id", "tap_point", "render_mode", "quality_status",
		"quality_reason", "quality_evidence", "source_identity", "source_revision", "clip_revision",
		"render_revision", "analyzer_revision", "duration_seconds", "sample_rate", "channel_count",
		"analyzed_range", "evidence_ref", "silence_confirmed", "silence_reason", "original_quality_status")
	if status != "ready" && status != "partial" && status != "suspect" {
		return out
	}
	copyOptionalFeatureFields(out, row, "bands", "balance_state", "correlation_state")
	for _, key := range []string{"peak_abs", "peak_dbfs", "rms", "rms_dbfs", "headroom_db", "crest_db", "left_level_db", "right_level_db", "balance_db", "correlation_estimate"} {
		if value, ok := row[key]; ok && !isEmptyFeatureValue(value) {
			out[key] = round3(numberFromMap(row, key))
		}
	}
	return out
}

func copyOptionalFeatureFields(dst, src map[string]any, keys ...string) {
	for _, key := range keys {
		if value, ok := src[key]; ok && !isEmptyFeatureValue(value) {
			dst[key] = value
		}
	}
}

func isEmptyFeatureValue(value any) bool {
	if value == nil {
		return true
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text == "" || text == "<nil>"
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

func buildMixPackage(req Request, observationStatus, waveformStatus string, waveformMetrics map[string]any, timeSegments []map[string]any, bandEnergy map[string]any, stereoRelation map[string]any, loudness map[string]any, realtimeBandEnergy map[string]any, realtimeStereoRelation map[string]any, l2RenderProbe map[string]any, caps map[string]string) map[string]any {
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
	if cleanAnyString(loudness["status"]) != "ready" {
		missing = append(missing, "loudness_summary")
	}
	missing = append(missing, "before_after_delta", "ab_result")
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
			"loudness":        loudness,
		},
		"realtime_metrics": map[string]any{
			"band_energy":     realtimeBandEnergy,
			"stereo_relation": realtimeStereoRelation,
			"render_probe":    l2RenderProbe,
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
			"waveform_envelope":        caps["waveform_envelope"],
			"time_energy":              timeEnergyStatus,
			"band_energy":              cleanAnyString(bandEnergy["status"]),
			"stereo_correlation":       cleanAnyString(stereoRelation["status"]),
			"loudness_summary":         cleanAnyString(loudness["status"]),
			"realtime_band_energy":     cleanAnyString(realtimeBandEnergy["status"]),
			"realtime_stereo_relation": cleanAnyString(realtimeStereoRelation["status"]),
			"l2_render_probe":          cleanAnyString(l2RenderProbe["status"]),
			"post_fx_probe":            caps["post_fx_probe"],
			"before_after_delta":       "missing",
			"ab_result":                "missing",
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
	abResult := buildABResultComparison(before, *obs, ok, now)
	metrics["ab_result"] = abResult
	caps, _ := obs.MixPackage["source_capabilities"].(map[string]string)
	if caps == nil {
		caps = map[string]string{}
		obs.MixPackage["source_capabilities"] = caps
	}
	caps["before_after_delta"] = featureStatus(delta)
	caps["ab_result"] = featureStatus(abResult)
	if obs.SourceCapabilities == nil {
		obs.SourceCapabilities = map[string]string{}
	}
	obs.SourceCapabilities["before_after_delta"] = featureStatus(delta)
	obs.SourceCapabilities["ab_result"] = featureStatus(abResult)
	obs.MixPackage["missing_metrics"] = removeStringFromAnySlice(obs.MixPackage["missing_metrics"], "before_after_delta")
	if featureStatus(delta) != "ready" {
		obs.MixPackage["missing_metrics"] = appendStringIfMissing(obs.MixPackage["missing_metrics"], "before_after_delta")
	}
	obs.MixPackage["missing_metrics"] = removeStringFromAnySlice(obs.MixPackage["missing_metrics"], "ab_result")
	if featureStatus(abResult) != "ready" {
		obs.MixPackage["missing_metrics"] = appendStringIfMissing(obs.MixPackage["missing_metrics"], "ab_result")
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

func buildABResultComparison(before, after ObservationPacket, ok bool, now string) map[string]any {
	if !ok || strings.TrimSpace(before.ObservationID) == "" {
		return map[string]any{
			"schema_version": "mom_ab_result.v1",
			"status":         "missing",
			"reason":         "no_previous_mix_observation",
			"updated_at":     now,
		}
	}
	beforeProbe := observationRenderProbe(before)
	afterProbe := observationRenderProbe(after)
	out := map[string]any{
		"schema_version":         "mom_ab_result.v1",
		"source":                 "mixboard.l2_render_probe_ab",
		"before_observation_id":  before.ObservationID,
		"after_observation_id":   after.ObservationID,
		"before_round":           before.Round,
		"after_round":            after.Round,
		"before_evidence_ref":    renderProbeEvidenceRef(beforeProbe),
		"after_evidence_ref":     renderProbeEvidenceRef(afterProbe),
		"before_render_revision": cleanAnyString(beforeProbe["render_revision"]),
		"after_render_revision":  cleanAnyString(afterProbe["render_revision"]),
		"tap_point":              cleanAnyString(afterProbe["tap_point"]),
		"render_mode":            cleanAnyString(afterProbe["render_mode"]),
		"updated_at":             now,
	}
	if goal := cleanAnyString(after.MixPackage["goal_text"]); goal != "" {
		out["action_summary"] = goal
	}
	status, reason, gates := abResultQualityGate(beforeProbe, afterProbe)
	out["status"] = status
	if reason != "" {
		out["reason"] = reason
	}
	out["quality_gates"] = gates
	if status == "ready" {
		out["delta"] = abRenderProbeDelta(beforeProbe, afterProbe)
		out["summary_tags"] = abResultSummaryTags(mapValue(out["delta"]))
		out["summary"] = abResultSummary(stringsFromAny(out["summary_tags"]))
	} else if len(beforeProbe) == 0 || len(afterProbe) == 0 {
		out["delta"] = map[string]any{}
	} else {
		out["delta"] = abRenderProbeDelta(beforeProbe, afterProbe)
	}
	return out
}

func observationRenderProbe(obs ObservationPacket) map[string]any {
	realtime, _ := obs.MixPackage["realtime_metrics"].(map[string]any)
	if row := mapValue(realtime["render_probe"]); len(row) > 0 {
		return row
	}
	current, _ := obs.MixPackage["current_metrics"].(map[string]any)
	if row := mapValue(current["render_probe"]); len(row) > 0 {
		return row
	}
	if row := mapValue(mapValue(obs.GlobalSummary["feature_snapshot"])["l2_render_probe"]); len(row) > 0 {
		return buildL2RenderProbeSummary(row)
	}
	return nil
}

func renderProbeEvidenceRef(row map[string]any) string {
	if len(row) == 0 {
		return ""
	}
	if ref := cleanAnyString(row["evidence_ref"]); ref != "" {
		return ref
	}
	if renderRevision := cleanAnyString(row["render_revision"]); renderRevision != "" {
		return "dad.l2_render_probe:" + renderRevision
	}
	return ""
}

func abResultQualityGate(beforeProbe, afterProbe map[string]any) (string, string, map[string]any) {
	gates := map[string]any{
		"before_probe_present":    len(beforeProbe) > 0,
		"after_probe_present":     len(afterProbe) > 0,
		"before_status_ready":     featureStatus(beforeProbe) == "ready",
		"after_status_ready":      featureStatus(afterProbe) == "ready",
		"same_tap_point":          false,
		"same_render_mode":        false,
		"render_revision_changed": false,
		"before_evidence_ref":     renderProbeEvidenceRef(beforeProbe) != "",
		"after_evidence_ref":      renderProbeEvidenceRef(afterProbe) != "",
		"before_quality_trusted":  renderProbeQualityTrusted(beforeProbe),
		"after_quality_trusted":   renderProbeQualityTrusted(afterProbe),
		"raw_payload_excluded":    renderProbeRawPayloadExcluded(beforeProbe) && renderProbeRawPayloadExcluded(afterProbe),
	}
	if len(beforeProbe) == 0 || len(afterProbe) == 0 {
		return "missing", "render_probe_missing", gates
	}
	beforeTap := cleanAnyString(beforeProbe["tap_point"])
	afterTap := cleanAnyString(afterProbe["tap_point"])
	gates["same_tap_point"] = beforeTap != "" && beforeTap == afterTap
	beforeMode := cleanAnyString(beforeProbe["render_mode"])
	afterMode := cleanAnyString(afterProbe["render_mode"])
	gates["same_render_mode"] = beforeMode != "" && beforeMode == afterMode
	beforeRevision := cleanAnyString(beforeProbe["render_revision"])
	afterRevision := cleanAnyString(afterProbe["render_revision"])
	gates["render_revision_changed"] = beforeRevision != "" && afterRevision != "" && beforeRevision != afterRevision
	if featureStatus(beforeProbe) == "stale" || featureStatus(afterProbe) == "stale" {
		return "stale", "render_probe_stale", gates
	}
	if beforeRevision == "" || afterRevision == "" {
		return "missing", "render_revision_missing", gates
	}
	if beforeRevision == afterRevision {
		return "stale", "render_revision_not_changed", gates
	}
	if beforeTap == "" || afterTap == "" || beforeTap != afterTap {
		return "suspect", "tap_point_mismatch", gates
	}
	if beforeMode == "" || afterMode == "" || beforeMode != afterMode {
		return "suspect", "render_mode_mismatch", gates
	}
	if renderProbeEvidenceRef(beforeProbe) == "" || renderProbeEvidenceRef(afterProbe) == "" {
		return "suspect", "evidence_ref_missing", gates
	}
	if featureStatus(beforeProbe) != "ready" || featureStatus(afterProbe) != "ready" {
		return "suspect", "render_probe_not_ready", gates
	}
	if !renderProbeQualityTrusted(beforeProbe) || !renderProbeQualityTrusted(afterProbe) {
		return "suspect", "quality_evidence_failed", gates
	}
	if !renderProbeRawPayloadExcluded(beforeProbe) || !renderProbeRawPayloadExcluded(afterProbe) {
		return "suspect", "raw_payload_present", gates
	}
	return "ready", "", gates
}

func renderProbeQualityTrusted(row map[string]any) bool {
	if len(row) == 0 {
		return false
	}
	qualityStatus := strings.ToLower(cleanAnyString(row["quality_status"]))
	if qualityStatus != "" && qualityStatus != "ready" {
		return false
	}
	evidence := mapValue(row["quality_evidence"])
	if len(evidence) == 0 {
		return false
	}
	if value, ok := boolFromAnyOK(evidence["nonzero"]); ok && !value {
		return false
	}
	if numberFromMap(evidence, "sum_abs") <= 0 || numberFromMap(evidence, "max_abs") <= 0 {
		return false
	}
	if numberFromMap(evidence, "nan_inf_count") > 0 {
		return false
	}
	if coverage := numberFromMap(evidence, "coverage"); coverage <= 0 {
		return false
	}
	for _, key := range []string{"latency_compensated", "tail_captured", "deterministic"} {
		if value, ok := boolFromAnyOK(evidence[key]); ok && !value {
			return false
		}
	}
	return true
}

func renderProbeRawPayloadExcluded(row map[string]any) bool {
	if len(row) == 0 {
		return true
	}
	for _, key := range []string{"raw_waveform", "raw_samples", "time_segments", "spectral_tiles", "shared_memory", "render_file_path", "render_path"} {
		if value, ok := row[key]; ok && !isEmptyFeatureValue(value) {
			return false
		}
	}
	return true
}

func abRenderProbeDelta(beforeProbe, afterProbe map[string]any) map[string]any {
	out := map[string]any{}
	if levels := metricDeltaBlock(beforeProbe, afterProbe, []string{"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db", "peak_abs", "rms"}); len(levels) > 0 {
		out["levels"] = levels
	}
	if bands := bandEnergyDelta(beforeProbe, afterProbe); len(bands) > 0 {
		out["bands"] = bands
	}
	if stereo := metricDeltaBlock(beforeProbe, afterProbe, []string{"balance_db", "correlation_estimate", "left_level_db", "right_level_db"}); len(stereo) > 0 {
		out["stereo"] = stereo
	}
	if risk := abRiskDelta(beforeProbe, afterProbe); len(risk) > 0 {
		out["risk"] = risk
	}
	return out
}

func abRiskDelta(beforeProbe, afterProbe map[string]any) map[string]any {
	beforePeak := numberFromMap(beforeProbe, "peak_dbfs")
	afterPeak := numberFromMap(afterProbe, "peak_dbfs")
	beforeRisk := peakRiskLabel(beforePeak)
	afterRisk := peakRiskLabel(afterPeak)
	if beforeRisk == "" && afterRisk == "" {
		return nil
	}
	return map[string]any{
		"peak_risk_before": beforeRisk,
		"peak_risk_after":  afterRisk,
		"changed":          beforeRisk != afterRisk,
	}
}

func peakRiskLabel(peakDBFS float64) string {
	if peakDBFS == 0 {
		return ""
	}
	switch {
	case peakDBFS >= -0.1:
		return "clip_risk"
	case peakDBFS >= -1.0:
		return "hot"
	case peakDBFS <= -18.0:
		return "very_low"
	default:
		return "normal"
	}
}

func abResultSummaryTags(delta map[string]any) []string {
	tags := []string{}
	levels := mapValue(delta["levels"])
	if row := mapValue(levels["peak_dbfs"]); math.Abs(numberFromMap(row, "delta")) >= 0.5 {
		tags = append(tags, "peak_changed")
	}
	if row := mapValue(levels["rms_dbfs"]); math.Abs(numberFromMap(row, "delta")) >= 0.5 {
		tags = append(tags, "rms_changed")
	}
	bands := mapValue(delta["bands"])
	for id, raw := range bands {
		row := mapValue(raw)
		if energyDB := mapValue(row["energy_db"]); math.Abs(numberFromMap(energyDB, "delta")) >= 1.0 {
			tags = append(tags, "band_"+id+"_changed")
		}
	}
	stereo := mapValue(delta["stereo"])
	if row := mapValue(stereo["balance_db"]); math.Abs(numberFromMap(row, "delta")) >= 0.5 {
		tags = append(tags, "stereo_balance_changed")
	}
	if row := mapValue(stereo["correlation_estimate"]); math.Abs(numberFromMap(row, "delta")) >= 0.05 {
		tags = append(tags, "stereo_correlation_changed")
	}
	if len(tags) == 0 {
		tags = append(tags, "no_significant_change")
	}
	return tags
}

func abResultSummary(tags []string) string {
	if len(tags) == 0 || (len(tags) == 1 && tags[0] == "no_significant_change") {
		return "同一 tap point 的 L2 Render Probe AB 对比显示变化很小。"
	}
	return "同一 tap point 的 L2 Render Probe AB 对比显示已产生可测变化。"
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

func (s Store) readPreviousObservation(sessionDir string, args map[string]any) (ObservationPacket, bool) {
	previousID := cleanAnyString(args["previous_observation"])
	if previousID == "" {
		previousID = cleanAnyString(args["previous_observation_id"])
	}
	previousID = normalizeObservationRef(previousID)
	if previousID != "" {
		if obs, ok := readObservationByID(sessionDir, previousID); ok {
			return obs, true
		}
		if obs, ok := readObservationByIDAcrossRoot(s.Root, previousID); ok {
			return obs, true
		}
	}
	return readLatestObservation(sessionDir)
}

func normalizeObservationRef(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimPrefix(ref, "observation:")
	ref = strings.TrimPrefix(ref, "observation_id:")
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(ref), ".json") {
		ref = strings.TrimSuffix(filepath.Base(ref), filepath.Ext(ref))
	}
	return ref
}

func readObservationByID(sessionDir, observationID string) (ObservationPacket, bool) {
	observationID = normalizeObservationRef(observationID)
	if observationID == "" {
		return ObservationPacket{}, false
	}
	return readObservationPacket(filepath.Join(sessionDir, "observations", observationID+".json"))
}

func readObservationByIDAcrossRoot(root, observationID string) (ObservationPacket, bool) {
	observationID = normalizeObservationRef(observationID)
	if strings.TrimSpace(root) == "" || observationID == "" {
		return ObservationPacket{}, false
	}
	if canonicalRoot := canonicalObservationRoot(root); canonicalRoot != "" {
		if obs, ok := readObservationPacket(filepath.Join(canonicalRoot, observationID+".json")); ok {
			return obs, true
		}
	}
	matches, err := filepath.Glob(filepath.Join(root, "*", "observations", observationID+".json"))
	if err != nil {
		return ObservationPacket{}, false
	}
	for _, path := range matches {
		if obs, ok := readObservationPacket(path); ok {
			return obs, true
		}
	}
	return ObservationPacket{}, false
}

func readObservationPacket(path string) (ObservationPacket, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ObservationPacket{}, false
	}
	obs, err := readObservationFile(path)
	if err != nil {
		return ObservationPacket{}, false
	}
	return obs, true
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
	return readObservationPacket(path)
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
	bandStatus := featureStatus(snap.BandEnergySummary)
	stereoStatus := featureStatus(snap.StereoRelationSummary)
	loudnessStatus := featureStatus(snap.LoudnessSummary)
	l2RenderProbeStatus := featureStatus(snap.L2RenderProbe)
	maskingStatus := featureStatus(snap.MaskingMeasurement)
	status := "async_missing"
	if spectrogramStatus == "ready" {
		status = "async_available"
	} else if spectrogramStatus == "partial" {
		status = "async_partial"
	} else if bandStatus == "ready" || stereoStatus == "ready" || loudnessStatus == "ready" || maskingStatus == "ready" {
		status = "async_available"
	} else if bandStatus == "partial" || stereoStatus == "partial" || loudnessStatus == "partial" || loudnessStatus == "suspect" {
		status = "async_partial"
	}
	return map[string]any{
		"schema_version": "mixboard_deep_packet.v1",
		"status":         status,
		"role":           "slow_context_refresh",
		"source_capabilities": map[string]string{
			"spectrogram_tiles":     spectrogramStatus,
			"deep_band_observation": bandStatus,
			"stereo_relation":       stereoStatus,
			"loudness_summary":      loudnessStatus,
			"section_detection":     "heuristic",
			"masking_analysis":      maskingStatus,
			"reference_match":       "deferred",
			"lufs_analysis":         "deferred",
			"l2_render_probe":       l2RenderProbeStatus,
			"post_fx_probe":         postFXProbeCapability(l2RenderProbeStatus),
		},
		"feature_snapshot": map[string]any{
			"spectrogram_tiles":       compactFeatureRow(snap.SpectrogramTiles),
			"band_energy_summary":     compactFeatureRow(snap.BandEnergySummary),
			"stereo_relation_summary": compactFeatureRow(snap.StereoRelationSummary),
			"loudness_summary":        compactFeatureRow(snap.LoudnessSummary),
			"masking_analysis":        compactMaskingMeasurement(snap.MaskingMeasurement),
		},
	}
}

func observationWantsBandStereoProjection(args map[string]any) bool {
	if mom.ResolveIntent(args, firstNonEmpty(cleanAnyString(args["mom_intent"]), cleanAnyString(args["intent"]))) == mom.IntentProjectFrequencyObservation {
		return true
	}
	projection := strings.ToLower(strings.TrimSpace(cleanAnyString(args["projection"])))
	switch projection {
	case "frequency_stereo", "band_stereo", "band_stereo_status", "frequency_stereo_status":
		return !argBoolDefault(args["include_raw"], true)
	}
	keys := stringSetFromAny(args["feature_keys"])
	if len(keys) == 0 {
		return false
	}
	hasBand := keys["band_energy_summary"] || keys["band_energy"]
	hasStereo := keys["stereo_relation_summary"] || keys["stereo_relation"]
	hasSpectral := keys["spectrogram_tiles"] || keys["spectral_field"]
	return hasBand && hasStereo && hasSpectral && !argBoolDefault(args["include_raw"], true)
}

func applyBandStereoProjection(obs *ObservationPacket, req Request) {
	if obs == nil {
		return
	}
	sourceIdentity := observationSourceIdentity(*obs)
	metrics := mapValue(obs.MixPackage["current_metrics"])
	realtimeMetrics := mapValue(obs.MixPackage["realtime_metrics"])
	band := compactProjectedBandEnergy(mapValue(metrics["band_energy"]))
	stereo := compactProjectedStereoRelation(mapValue(metrics["stereo_relation"]))
	loudness := compactProjectedLoudness(mapValue(metrics["loudness"]))
	realtimeBand := compactProjectedBandEnergy(mapValue(realtimeMetrics["band_energy"]))
	realtimeStereo := compactProjectedStereoRelation(mapValue(realtimeMetrics["stereo_relation"]))
	spectral := projectedSpectrogramStatus(*obs)
	projectedFeatureSnapshot := projectedFeatureSnapshot(*obs, spectral, band, stereo, loudness, realtimeBand, realtimeStereo)
	acousticStatus := compactAcousticStatusForDigest(obs.AcousticPackageStatus)
	projectedCaps := projectedSourceCapabilities(obs.SourceCapabilities)
	obs.SourceCapabilities = projectedCaps

	obs.GlobalSummary = map[string]any{
		"projection":                       "frequency_stereo",
		"source_identity":                  sourceIdentity,
		"acoustic_package_status":          acousticStatus,
		"spectrogram_tiles":                spectral,
		"band_energy_summary":              band,
		"stereo_relation_summary":          stereo,
		"loudness_summary":                 loudness,
		"realtime_band_energy_summary":     realtimeBand,
		"realtime_stereo_relation_summary": realtimeStereo,
		"feature_snapshot":                 projectedFeatureSnapshot,
	}

	currentMetrics := map[string]any{
		"band_energy":     band,
		"stereo_relation": stereo,
		"loudness":        loudness,
	}
	obs.MixPackage["current_metrics"] = currentMetrics
	obs.MixPackage["realtime_metrics"] = map[string]any{
		"band_energy":     realtimeBand,
		"stereo_relation": realtimeStereo,
	}
	obs.MixPackage["projection"] = "frequency_stereo"
	obs.MixPackage["goal_text"] = strings.TrimSpace(req.GoalText)
	obs.MixPackage["missing_metrics"] = projectedMissingMetrics(obs.MixPackage["missing_metrics"])

	obs.EnvironmentPackage = map[string]any{
		"schema_version":          "mixboard_environment.v1",
		"status":                  obs.EnvironmentPackage["status"],
		"role":                    "environment",
		"target_ref":              obs.TargetRef,
		"listen_scope":            obs.ListenScope,
		"time_ruler":              obs.TimeRuler,
		"source_capabilities":     projectedCaps,
		"source_identity":         sourceIdentity,
		"acoustic_package_status": acousticStatus,
	}
	projectPackage := map[string]any{
		"schema_version":              "mixboard_project_packet.v1",
		"status":                      obs.ProjectPackage["status"],
		"role":                        "project_context",
		"project_uuid":                obs.ProjectPackage["project_uuid"],
		"project_epoch":               obs.ProjectPackage["project_epoch"],
		"project_revision":            obs.ProjectPackage["project_revision"],
		"project_state_hash":          obs.ProjectPackage["project_state_hash"],
		"duration_seconds":            obs.ProjectPackage["duration_seconds"],
		"track_count":                 obs.ProjectPackage["track_count"],
		"active_track_count":          obs.ProjectPackage["active_track_count"],
		"acoustic_track_count":        obs.ProjectPackage["acoustic_track_count"],
		"active_acoustic_track_count": obs.ProjectPackage["active_acoustic_track_count"],
		"summary":                     obs.ProjectPackage["summary"],
		"source_identity":             sourceIdentity,
		"relationship_inputs":         compactKeys(mapValue(obs.ProjectPackage["relationship_inputs"]), []string{"status", "track_count", "tracks_with_acoustic"}),
		"limitations":                 obs.ProjectPackage["limitations"],
	}
	frequencyIntent := mom.ResolveIntent(req.Args, firstNonEmpty(cleanAnyString(req.Args["mom_intent"]), cleanAnyString(req.Args["intent"]))) == mom.IntentProjectFrequencyObservation
	if frequencyIntent {
		projectPackage["frequency_relationship_inputs"] = compactFrequencyRelationshipInputs(mapValue(obs.ProjectPackage["frequency_relationship_inputs"]))
	}
	if projectionShouldIncludeProjectTracks(*obs) {
		projectPackage["tracks"] = projectedProjectTracks(obs.ProjectPackage, frequencyIntent)
	}
	obs.ProjectPackage = projectPackage
	obs.DeepPackage = map[string]any{
		"schema_version":          "mixboard_deep_packet.v1",
		"status":                  obs.DeepPackage["status"],
		"role":                    "slow_context_refresh",
		"source_capabilities":     obs.DeepPackage["source_capabilities"],
		"source_identity":         sourceIdentity,
		"acoustic_package_status": acousticStatus,
		"feature_snapshot":        projectedFeatureSnapshot,
	}
	obs.TimelineDigest = nil
	obs.SectionCandidates = nil
	obs.Hotspots = projectedHotspots(obs.Hotspots)
	obs.Notes = append(obs.Notes, "Observation projection frequency_stereo returned band/stereo/package readiness without raw waveform time segments.")
}

func finalizeMOMProjection(obs *ObservationPacket, req Request) {
	if obs == nil {
		return
	}
	projection := mom.Build(momInputFromObservation(*obs, req))
	obs.MOMProjection = &projection
}

func finalizeTIMProjection(obs *ObservationPacket, req Request) {
	if obs == nil {
		return
	}
	projection := tim.Build(timInputFromObservation(*obs, req))
	obs.TIMProjection = &projection
}

func finalizeFXMProjection(obs *ObservationPacket, req Request) {
	if obs == nil {
		return
	}
	input := fxm.Input{}
	if raw, ok := req.Args["fxm_measurement"]; ok {
		data, err := json.Marshal(raw)
		if err == nil {
			_ = json.Unmarshal(data, &input)
		}
	}
	input.ObservationID = obs.ObservationID
	input.MixSessionID = obs.MixSessionID
	input.CreatedAt = obs.CreatedAt
	if len(input.TargetRef) == 0 {
		input.TargetRef = map[string]any{"kind": obs.TargetRef.Kind, "id": obs.TargetRef.ID, "label": obs.TargetRef.Label}
	}
	projection := fxm.Build(input)
	obs.FXMProjection = &projection
}

func momInputFromObservation(obs ObservationPacket, req Request) mom.Input {
	args := copyAnyMap(req.Args)
	if goal := strings.TrimSpace(req.GoalText); goal != "" {
		args["goal_text"] = goal
	}
	if _, ok := args["projection"]; !ok {
		if projection := cleanAnyString(obs.MixPackage["projection"]); projection != "" {
			args["projection"] = projection
		}
	}
	return mom.Input{
		ObservationID:         obs.ObservationID,
		MixSessionID:          obs.MixSessionID,
		CreatedAt:             obs.CreatedAt,
		Args:                  args,
		TargetRef:             mom.ToMap(obs.TargetRef),
		ListenScope:           mom.ToMap(obs.ListenScope),
		TimeRuler:             mom.ToMap(obs.TimeRuler),
		GlobalSummary:         obs.GlobalSummary,
		EnvironmentPackage:    obs.EnvironmentPackage,
		ProjectPackage:        obs.ProjectPackage,
		MixPackage:            obs.MixPackage,
		DeepPackage:           obs.DeepPackage,
		AcousticPackageStatus: obs.AcousticPackageStatus,
		SourceCapabilities:    stringMapToAnyMap(obs.SourceCapabilities),
	}
}

func timInputFromObservation(obs ObservationPacket, req Request) tim.Input {
	args := copyAnyMap(req.Args)
	if goal := strings.TrimSpace(req.GoalText); goal != "" {
		args["goal_text"] = goal
	}
	return tim.Input{
		ObservationID:         obs.ObservationID,
		MixSessionID:          obs.MixSessionID,
		CreatedAt:             obs.CreatedAt,
		Args:                  args,
		ProjectPackage:        obs.ProjectPackage,
		AcousticPackageStatus: obs.AcousticPackageStatus,
		SourceCapabilities:    stringMapToAnyMap(obs.SourceCapabilities),
		AuthoritativeState:    authoritativeTIMState(obs, req),
	}
}

// authoritativeTIMState is deliberately a small internal summary. The full
// project state and DAD package remain in their owning/audit stores; TIM only
// receives binding, topology, and explicit source-state facts for consistency
// checks.
func authoritativeTIMState(obs ObservationPacket, req Request) map[string]any {
	state := copyAnyMap(req.ProjectState)
	project := mapValue(state["project"])
	out := map[string]any{
		"project_uuid":       firstNonEmpty(cleanAnyString(state["project_uuid"]), cleanAnyString(project["project_uuid"]), cleanAnyString(project["uuid"]), obs.ProjectUUID),
		"project_epoch":      firstNonEmpty(cleanAnyString(state["project_epoch"]), cleanAnyString(project["project_epoch"]), cleanAnyString(obs.ProjectPackage["project_epoch"])),
		"project_revision":   firstNonEmpty(cleanAnyString(state["project_revision"]), cleanAnyString(state["revision"]), cleanAnyString(project["revision"]), cleanAnyString(obs.ProjectPackage["project_revision"])),
		"project_state_hash": firstNonEmpty(cleanAnyString(state["snapshot_hash"]), cleanAnyString(state["project_state_hash"]), cleanAnyString(project["snapshot_hash"]), cleanAnyString(obs.ProjectPackage["project_state_hash"])),
		"track_count":        len(anySlice(state["tracks"])),
	}
	rows := []map[string]any{}
	for _, raw := range anySlice(state["tracks"]) {
		row := mapValue(raw)
		if len(row) == 0 {
			continue
		}
		primary := primaryTrackClip(anySlice(firstPresent(row, "clips", "clip_summaries")))
		item := map[string]any{
			"track_id": firstNonEmpty(cleanAnyString(row["track_id"]), cleanAnyString(row["id"])),
			"clip_id":  firstNonEmpty(cleanAnyString(primary["clip_id"]), cleanAnyString(primary["id"])),
		}
		for _, key := range []string{"source_status", "source_state", "source_availability", "playback_source_valid", "source_path", "current_source_path", "file_path"} {
			if value, ok := row[key]; ok && !isEmptyFeatureValue(value) {
				item[key] = value
			}
		}
		for _, key := range []string{"source_status", "source_state", "source_availability", "playback_source_valid", "source_path", "current_source_path", "file_path"} {
			if value, ok := primary[key]; ok && !isEmptyFeatureValue(value) {
				item[key] = value
			}
		}
		rows = append(rows, item)
	}
	// The public project-state summary may intentionally omit source identity
	// for compactness.  The persisted L1 analysis manifest is authoritative for
	// the currently bound project and carries the exact track/clip material
	// identity used by DAD.  Merge only those read-only facts into TIM's
	// internal state; they are never copied into the model-visible projection.
	mergeAuthoritativeTIMAnalysisRows(rows, mapValue(state["analysis_manifest"]), out)
	if len(rows) > 0 {
		out["tracks"] = rows
		clipCount := 0
		for _, row := range rows {
			if cleanAnyString(row["clip_id"]) != "" {
				clipCount++
			}
		}
		out["clip_count"] = clipCount
	}
	if status := mapValue(obs.AcousticPackageStatus); len(status) > 0 {
		if value := firstNonEmpty(cleanAnyString(status["dad_fact_status"]), cleanAnyString(status["analysis_status"])); value != "" {
			out["dad_fact_status"] = value
		}
		if value := firstNonNil(status["dad_fact_ready_count"], status["ready_count"]); value != nil {
			out["dad_fact_ready_count"] = value
		}
		if value := firstNonNil(status["dad_fact_total_count"], status["total_count"]); value != nil {
			out["dad_fact_total_count"] = value
		}
		out["dad_source_state"] = compactKeys(status, []string{"status", "source_identity", "source_path", "source_revision", "track_id", "clip_id"})
	}
	for _, key := range []string{"dad_fact_status", "dad_fact_ready_count", "dad_fact_total_count"} {
		if value, ok := state[key]; ok && !isEmptyFeatureValue(value) {
			out[key] = value
		}
	}
	return out
}

func mergeAuthoritativeTIMAnalysisRows(rows []map[string]any, manifest map[string]any, out map[string]any) {
	if len(rows) == 0 || len(manifest) == 0 {
		return
	}
	byKey := map[string]map[string]any{}
	byTrack := map[string][]map[string]any{}
	for _, row := range rows {
		trackID := cleanAnyString(row["track_id"])
		clipID := cleanAnyString(row["clip_id"])
		if trackID == "" {
			continue
		}
		byTrack[trackID] = append(byTrack[trackID], row)
		if clipID != "" {
			byKey[trackID+"::"+clipID] = row
		}
	}
	for _, evidence := range mapRowsAny(manifest["l1_waveform_rows"]) {
		trackID := firstNonEmpty(cleanAnyString(evidence["track_id"]), cleanAnyString(evidence["source_track_id"]))
		clipID := firstNonEmpty(cleanAnyString(evidence["clip_id"]), cleanAnyString(evidence["item_id"]))
		if trackID == "" {
			continue
		}
		row := byKey[trackID+"::"+clipID]
		if len(row) == 0 {
			candidates := byTrack[trackID]
			if len(candidates) == 1 {
				row = candidates[0]
			}
		}
		if len(row) == 0 {
			continue
		}
		for _, key := range []string{"source_path", "file_path", "source_revision", "source_fingerprint", "clip_revision"} {
			if value := evidence[key]; value != nil && !isEmptyFeatureValue(value) {
				row[key] = value
			}
		}
		for _, key := range []string{"sample_rate", "sample_rate_hz", "channel_count", "channels", "bit_depth", "bits_per_sample", "duration_seconds", "length_seconds"} {
			if value := evidence[key]; value != nil && !isEmptyFeatureValue(value) {
				row[key] = value
			}
		}
		if cleanAnyString(row["source_path"]) == "" && cleanAnyString(row["file_path"]) != "" {
			row["source_path"] = row["file_path"]
		}
		if strings.EqualFold(cleanAnyString(evidence["status"]), "ready") {
			acoustic := mapValue(row["acoustic"])
			if len(acoustic) == 0 {
				acoustic = map[string]any{}
				row["acoustic"] = acoustic
			}
			acoustic["status"] = "ready"
			for _, key := range []string{"rms_dbfs", "peak_dbfs", "headroom_db", "crest_db"} {
				if value := evidence[key]; value != nil && !isEmptyFeatureValue(value) {
					acoustic[key] = value
				}
			}
		}
		if cleanAnyString(row["source_status"]) == "" && cleanAnyString(row["source_path"]) != "" {
			row["source_status"] = "present"
		}
		if _, ok := row["playback_source_valid"]; !ok && cleanAnyString(row["source_path"]) != "" {
			row["playback_source_valid"] = true
		}
	}
	if status := cleanAnyString(manifest["status"]); status != "" {
		out["dad_manifest_status"] = status
	}
}

func copyAnyMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneChangeWindow(in []map[string]any) []map[string]any {
	if len(in) == 0 {
		return nil
	}
	limit := len(in)
	if limit > 4 {
		limit = 4
	}
	out := make([]map[string]any, 0, limit)
	for index := 0; index < limit; index++ {
		out = append(out, copyAnyMap(in[index]))
	}
	return out
}

func stringMapToAnyMap(in map[string]string) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func filterBandStereoProjectionCatalog(obs *ObservationPacket) {
	if obs == nil {
		return
	}
	entries := make([]CatalogEntry, 0, len(obs.Catalog.Entries))
	for _, entry := range obs.Catalog.Entries {
		if entry.Key == "observation.digest" ||
			entry.Key == "observation.mom_projection" ||
			entry.Key == "observation.tim_projection" ||
			(entry.Key == "observation.dom_projection" && obs.DOMProjection != nil) ||
			entry.Key == "project.limitations" ||
			strings.Contains(entry.Key, ".static.identity") ||
			strings.Contains(entry.Key, ".slow.band_energy.summary") ||
			strings.Contains(entry.Key, ".slow.stereo.summary") ||
			strings.Contains(entry.Key, ".realtime.band_energy.summary") ||
			strings.Contains(entry.Key, ".realtime.stereo.summary") {
			entries = append(entries, entry)
		}
	}
	obs.Catalog.Entries = entries
}

func stripObservationRawKeys(obs *ObservationPacket) {
	if obs == nil {
		return
	}
	stripMapRawKeys(obs.GlobalSummary)
	stripMapRawKeys(obs.EnvironmentPackage)
	stripMapRawKeys(obs.ProjectPackage)
	stripMapRawKeys(obs.MixPackage)
	stripMapRawKeys(obs.DeepPackage)
	stripMapRawKeys(obs.Digest)
}

func stripMapRawKeys(row map[string]any) {
	for key, value := range row {
		if key == "time_segments" || key == "time_energy" || key == "track_waveform_envelopes" || key == "waveform_envelope" ||
			key == "project_track_waveforms" || key == "full_project_acoustic_render" || key == "raw_ranges" ||
			key == "raw_waveform" || key == "raw_samples" || key == "spectral_tiles" || key == "render_file_path" || key == "render_path" || key == "shared_memory" {
			delete(row, key)
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			stripMapRawKeys(typed)
		case []map[string]any:
			for _, child := range typed {
				stripMapRawKeys(child)
			}
		case []any:
			for _, child := range typed {
				if childMap, ok := child.(map[string]any); ok {
					stripMapRawKeys(childMap)
				}
			}
		}
	}
}

func projectedSourceCapabilities(caps map[string]string) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"spectrogram_tiles", "band_energy", "stereo_relation", "loudness_summary", "realtime_band_energy", "realtime_stereo_relation", "l2_render_probe", "lufs_analysis", "masking_analysis", "reference_match", "post_fx_probe"} {
		if value := strings.TrimSpace(caps[key]); value != "" {
			out[key] = value
		}
	}
	return out
}

func observationSourceIdentity(obs ObservationPacket) map[string]any {
	if status := mapValue(obs.AcousticPackageStatus); len(status) > 0 {
		if identity := mapValue(status["source_identity"]); len(identity) > 0 {
			return compactKeys(identity, []string{"project_id", "session_id", "track_id", "clip_id", "source_path", "source_hash", "source_fingerprint", "source_revision", "clip_revision", "render_revision", "analyzer_revision", "duration_seconds", "clip_start_seconds"})
		}
		out := compactKeys(status, []string{"project_id", "session_id", "track_id", "clip_id", "source_path", "source_hash", "source_fingerprint", "source_revision", "clip_revision", "render_revision", "analyzer_revision", "duration_seconds", "clip_start_seconds"})
		if len(out) > 0 {
			return out
		}
	}
	return map[string]any{
		"track_id":    obs.TargetRef.ID,
		"target_kind": obs.TargetRef.Kind,
	}
}

func projectedSpectrogramStatus(obs ObservationPacket) map[string]any {
	if status := mapValue(obs.AcousticPackageStatus); len(status) > 0 {
		layers := mapValue(status["package_layers"])
		l3 := mapValue(layers["l3_deep"])
		features := mapValue(l3["features"])
		if spectral := mapValue(features["spectrogram_tiles"]); len(spectral) > 0 {
			out := compactKeys(spectral, []string{"status", "reason", "updated_at"})
			if progress := mapValue(spectral["progress"]); len(progress) > 0 {
				out["progress"] = compactKeys(progress, []string{"tile_count_seen", "tile_count_expected", "coverage_seconds", "coverage_ratio", "updated_at", "reason"})
			}
			if ref := mapValue(spectral["ref"]); len(ref) > 0 {
				out["ref"] = compactKeys(ref, []string{"request_id", "track_id", "clip_id", "file_path", "source_revision", "source_fingerprint", "source_hash", "clip_revision", "tile_count_seen", "tile_count_expected", "coverage_seconds", "coverage_ratio", "total_duration", "updated_at"})
			}
			return out
		}
	}
	featureSnap := mapValue(obs.DeepPackage["feature_snapshot"])
	return compactFeatureRow(mapValue(featureSnap["spectrogram_tiles"]))
}

func projectedFeatureSnapshot(obs ObservationPacket, spectral, band, stereo, loudness, realtimeBand, realtimeStereo map[string]any) map[string]any {
	out := map[string]any{
		"spectrogram_tiles":                spectral,
		"band_energy_summary":              band,
		"stereo_relation_summary":          stereo,
		"loudness_summary":                 loudness,
		"realtime_band_energy_summary":     realtimeBand,
		"realtime_stereo_relation_summary": realtimeStereo,
	}
	source := mapValue(obs.GlobalSummary["feature_snapshot"])
	if latest := compactProjectedFeatureRequest(mapValue(source["latest_request"])); len(latest) > 0 {
		out["latest_request"] = latest
	}
	return out
}

func compactProjectedFeatureRequest(row map[string]any) map[string]any {
	return compactKeys(row, []string{"schema_version", "status", "request_id", "requested_features", "resolved_target", "source_kind", "lifecycle", "project_id", "updated_at"})
}

func projectionShouldIncludeProjectTracks(obs ObservationPacket) bool {
	if strings.EqualFold(strings.TrimSpace(obs.TargetRef.Kind), "project") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(obs.ListenScope.Source.Mode)) {
	case "full_project", "full_project_with_focus_track", "track_group":
		return true
	default:
		return false
	}
}

func projectedProjectTracks(project map[string]any, includeFrequencyEvidence bool) []map[string]any {
	rows := mapRowsAny(project["tracks"])
	out := make([]map[string]any, 0, len(rows))
	for _, track := range rows {
		if len(track) == 0 {
			continue
		}
		row := compactKeys(track, []string{
			"track_id", "name", "track_name", "user_label", "user_track_index", "role_guess", "active_state",
			"track_type", "clip_count", "plugin_count", "focused", "selected", "mute", "solo", "is_armed",
			"is_audio", "is_audio_track", "is_folder_track", "is_folder_container", "is_submix_folder",
			"has_child_tracks", "child_track_count", "descendant_track_count",
			"volume_db", "pan", "level_db", "rms_dbfs", "effective_static_rms_dbfs", "peak_dbfs", "effective_static_peak_dbfs", "headroom_db", "crest_db", "left_level_db", "right_level_db",
		})
		if level := compactProjectedStaticLevel(mapValue(track["static_level"])); len(level) > 0 {
			row["static_level"] = level
		}
		if acoustic := compactProjectedTrackAcoustic(mapValue(track["acoustic"])); len(acoustic) > 0 {
			row["acoustic"] = acoustic
		}
		if band := compactProjectedBandEnergy(mapValue(track["band_energy"])); len(band) > 0 {
			row["band_energy"] = band
		}
		if includeFrequencyEvidence {
			if frequency := compactProjectedFrequencyEvidence(mapValue(track["frequency_evidence"])); len(frequency) > 0 {
				row["frequency_evidence"] = frequency
			}
		}
		if stereo := compactProjectedStereoRelation(mapValue(track["stereo_relation"])); len(stereo) > 0 {
			row["stereo_relation"] = stereo
		}
		if loudness := compactProjectedLoudness(mapValue(track["loudness"])); len(loudness) > 0 {
			row["loudness"] = loudness
		}
		out = append(out, row)
	}
	return out
}

func compactProjectedTrackAcoustic(row map[string]any) map[string]any {
	return compactKeys(row, []string{
		"status", "track_id", "clip_id", "primary_clip_id", "primary_clip_name",
		"source", "source_path", "file_path", "request_id", "rms_dbfs", "peak_dbfs", "headroom_db", "crest_db",
		"time_energy_status", "reason", "updated_at",
	})
}

func compactProjectedStaticLevel(row map[string]any) map[string]any {
	return compactKeys(row, []string{
		"schema_version", "track_id", "status", "metric", "tap_point", "source_rms_dbfs",
		"effective_static_rms_dbfs", "effective_static_peak_dbfs", "fader_db",
		"included_clip_ids", "missing_clip_ids", "aggregation_method", "evidence_refs", "limitations",
	})
}

func compactStaticLevelRelationshipInputs(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	out := compactKeys(row, []string{"schema_version", "status", "project_cut_ref", "track_count", "usable_track_count", "ready_track_count", "evidence_refs", "limitations"})
	tracks := mapRowsAny(row["tracks"])
	compactTracks := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		if compact := compactProjectedStaticLevel(track); len(compact) > 0 {
			compactTracks = append(compactTracks, compact)
		}
	}
	out["tracks"] = compactTracks
	return out
}

func compactProjectedBandEnergy(row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": "missing"}
	}
	out := compactKeys(row, []string{"schema_version", "status", "reason", "source", "source_kind", "layer", "capture_mode", "tap_point", "capture_time", "time_basis", "quality_status", "quality_reason", "quality_evidence", "source_identity", "source_revision", "clip_revision", "render_revision", "plugin_chain_revision", "fader_revision", "updated_at", "track_id", "clip_id", "target", "request_id", "tile_count_seen", "tile_count_expected", "coverage_seconds", "coverage_ratio", "total_duration", "duration_seconds", "sample_rate", "channel_count", "analyzed_range", "frame_duration_seconds", "tile_duration", "window_ms", "hop_ms", "analyzer_revision", "analyzer_version", "derivation_status"})
	if bands := mapValue(row["bands"]); len(bands) > 0 {
		out["bands"] = bands
	}
	// Dense event arrays remain available through the Observation evidence
	// reference. Their readiness is still useful to fixed project workflows
	// (including C2 candidate ranking), so retain the bounded status surface.
	if events := mapValue(row["frequency_time_events"]); len(events) > 0 {
		out["frequency_time_events"] = compactKeys(events, []string{"schema_version", "status", "coverage", "event_count", "event_count_available", "reason"})
	}
	if dynamics := mapValue(row["band_dynamics"]); len(dynamics) > 0 {
		out["band_dynamics"] = compactKeys(dynamics, []string{"schema_version", "status", "coverage", "band_count", "reason"})
	}
	if transients := mapValue(row["transient_events"]); len(transients) > 0 {
		out["transient_events"] = compactKeys(transients, []string{"schema_version", "status", "coverage", "event_count", "event_count_available", "reason"})
	}
	return out
}

func compactProjectedFrequencyEvidence(row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": "missing"}
	}
	out := compactKeys(row, []string{"schema_version", "feature_type", "status", "reason", "source", "source_kind", "layer", "evidence_layer", "capture_mode", "tap_point", "render_mode", "capture_time", "time_basis", "quality_status", "quality_reason", "track_id", "clip_id", "request_id", "source_revision", "clip_revision", "render_revision", "coverage_seconds", "coverage_ratio", "total_duration", "duration_seconds", "sample_rate", "channel_count", "analyzed_range", "frame_duration_seconds", "tile_duration", "window_ms", "hop_ms", "analyzer_revision", "analyzer_version", "evidence_ref", "silence_confirmed", "silence_reason", "original_quality_status"})
	if bands := mapValue(row["bands"]); len(bands) > 0 {
		out["bands"] = bands
	}
	return out
}

func compactFrequencyRelationshipInputs(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	return compactKeys(row, []string{"schema_version", "status", "track_count", "usable_track_count", "tap_points", "tracks", "decision_tracks_truncated"})
}

func compactProjectedStereoRelation(row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": "missing"}
	}
	return compactKeys(row, []string{"schema_version", "status", "reason", "source", "source_kind", "layer", "capture_mode", "tap_point", "capture_time", "time_basis", "quality_status", "quality_reason", "quality_evidence", "source_identity", "source_revision", "clip_revision", "render_revision", "plugin_chain_revision", "fader_revision", "updated_at", "track_id", "clip_id", "target", "request_id", "tile_count_seen", "tile_count_expected", "coverage_seconds", "coverage_ratio", "total_duration", "derivation_status", "left_level_db", "right_level_db", "balance_db", "balance_unit", "balance_state", "phase_deviation", "phase_negative_ratio", "correlation_estimate", "correlation_state", "bin_count", "sample_count", "phase_sample_count"})
}

func compactProjectedLoudness(row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": "missing"}
	}
	return compactKeys(row, []string{
		"schema_version", "status", "reason", "source", "source_kind", "layer", "quality_status",
		"quality_reason", "quality_reasons", "quality_evidence", "source_identity", "source_revision",
		"clip_revision", "render_revision", "analyzer_revision", "analyzer_version", "updated_at",
		"track_id", "clip_id", "target", "request_id", "duration_seconds", "sample_rate",
		"channel_count", "channels", "expected_sample_count", "analyzed_sample_count",
		"coverage_ratio", "nonzero_count", "sum_abs", "max_abs", "nan_count", "inf_count",
		"peak", "peak_abs", "peak_dbfs", "rms", "rms_dbfs", "integrated_lufs",
		"approximate_lufs", "approximate", "algorithm", "crest_factor", "crest_db", "evidence_ref",
	})
}

func compactAcousticStatusForDigest(status map[string]any) map[string]any {
	if len(status) == 0 {
		return nil
	}
	out := compactKeys(status, []string{"schema_version", "status", "project_id", "session_id", "track_id", "clip_id", "source_path", "source_hash", "source_fingerprint", "source_revision", "clip_revision", "render_revision", "analyzer_revision", "duration_seconds", "clip_start_seconds", "source_identity", "updated_at"})
	layers := mapValue(status["package_layers"])
	if len(layers) == 0 {
		return out
	}
	layerOut := map[string]any{}
	for _, layerName := range []string{"l1_static", "l2_realtime", "l3_deep"} {
		layer := mapValue(layers[layerName])
		if len(layer) == 0 {
			continue
		}
		compactLayer := compactKeys(layer, []string{"status", "reason", "updated_at"})
		features := mapValue(layer["features"])
		featureOut := map[string]any{}
		for name, raw := range features {
			feature := mapValue(raw)
			if len(feature) == 0 {
				continue
			}
			row := compactKeys(feature, []string{"status", "reason", "updated_at"})
			if progress := mapValue(feature["progress"]); len(progress) > 0 {
				row["progress"] = compactKeys(progress, []string{"tile_count_seen", "tile_count_expected", "coverage_seconds", "coverage_ratio", "updated_at", "reason"})
			}
			featureOut[name] = row
		}
		if len(featureOut) > 0 {
			compactLayer["features"] = featureOut
		}
		layerOut[layerName] = compactLayer
	}
	if len(layerOut) > 0 {
		out["package_layers"] = layerOut
	}
	return out
}

func projectedMissingMetrics(value any) []string {
	missing := removeStringFromAnySlice(value, "")
	out := make([]string, 0, len(missing))
	for _, item := range missing {
		switch item {
		case "band_energy_summary", "stereo_correlation":
			out = append(out, item)
		}
	}
	return out
}

func projectedHotspots(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		tag := cleanAnyString(row["tag"])
		if strings.Contains(tag, "spectral") || strings.Contains(tag, "band") || strings.Contains(tag, "stereo") {
			out = append(out, row)
		}
	}
	return capRows(out, 6)
}

func argBoolDefault(value any, fallback bool) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		text := strings.ToLower(strings.TrimSpace(v))
		if text == "true" || text == "1" || text == "yes" {
			return true
		}
		if text == "false" || text == "0" || text == "no" {
			return false
		}
	}
	return fallback
}

func stringSetFromAny(value any) map[string]bool {
	out := map[string]bool{}
	for _, raw := range anySlice(value) {
		text := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw)))
		if text != "" && text != "<nil>" {
			out[text] = true
		}
	}
	return out
}

func packageStatus(obs ObservationPacket) map[string]string {
	out := map[string]string{
		"environment": cleanAnyString(obs.EnvironmentPackage["status"]),
		"project":     cleanAnyString(obs.ProjectPackage["status"]),
		"mix":         cleanAnyString(obs.MixPackage["status"]),
		"deep":        cleanAnyString(obs.DeepPackage["status"]),
	}
	if obs.COMProjection != nil {
		out["com"] = obs.COMProjection.Status
	}
	if obs.DOMProjection != nil {
		out["dom"] = obs.DOMProjection.Status
	}
	return out
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
	bandStatus := featureStatus(snap.BandEnergySummary)
	if spectrogramStatus == "partial" && bandStatus != "ready" && bandStatus != "partial" {
		out = append(out, map[string]any{
			"tag":        "spectral_tiles_partial_band_summary_missing",
			"summary":    "spectral tiles are partial; band-energy diagnosis remains unavailable until a fresh band_energy_summary is ready",
			"confidence": 0.2,
		})
		return out
	}
	if bandStatus == "partial" {
		out = append(out, map[string]any{
			"tag":        "spectral_tile_derived_band_summary_partial",
			"summary":    "spectral tiles provide partial band/stereo evidence; coverage is limited to returned tiles",
			"confidence": 0.28,
		})
		return out
	}
	if bandStatus == "ready" {
		out = append(out, map[string]any{
			"tag":        "deep_band_observation_available",
			"summary":    "深度频段观察可用，可作为后续精修参考",
			"confidence": 0.35,
		})
		return out
	}
	if spectrogramStatus == "ready" {
		out = append(out, map[string]any{
			"tag":        "spectral_tiles_ready_band_summary_missing",
			"summary":    "spectral tiles are ready, but band-energy diagnosis remains unavailable until a fresh band_energy_summary is ready",
			"confidence": 0.24,
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
		"observation_id": obs.ObservationID,
		"status":         obs.Status,
		"time_ruler": map[string]any{
			"duration_seconds": obs.TimeRuler.DurationSeconds,
			"segment_seconds":  obs.TimeRuler.SegmentSeconds,
			"frame_seconds":    obs.TimeRuler.FrameSeconds,
		},
		"source_capabilities": projectedSourceCapabilities(obs.SourceCapabilities),
		"read_hints": []string{
			"mix_read key=observation.com_projection",
			"mix_read key=observation.dom_projection",
			"mix_read key=observation.mom_projection",
			"mix_read key=observation.tim_projection",
			"mix_read key=observation.fxm_projection",
			"mix_read key=observation.digest",
			"mix_read key=observation.catalog",
		},
	}
	if len(obs.ProjectChange) > 0 {
		latest["project_change"] = copyAnyMap(obs.ProjectChange)
	}
	if obs.COMProjection != nil {
		latest["com_projection"] = com.ContextProjection(*obs.COMProjection)
		latest["compression_observation_context"] = obs.COMProjection.LLMContext
	}
	if obs.DOMProjection != nil {
		latest["dom_projection"] = dom.ContextProjection(*obs.DOMProjection)
		latest["dynamics_observation_context"] = obs.DOMProjection.LLMContext
	}
	if obs.MOMProjection != nil {
		latest["mom_projection"] = mom.ContextProjection(*obs.MOMProjection)
		latest["llm_context"] = obs.MOMProjection.LLMContext
	}
	if obs.TIMProjection != nil {
		latest["tim_projection"] = tim.ContextProjection(*obs.TIMProjection)
		latest["technical_integrity_context"] = obs.TIMProjection.LLMContext
	}
	if obs.FXMProjection != nil {
		latest["fxm_projection"] = fxm.ContextProjection(*obs.FXMProjection)
		latest["effects_transformation_context"] = obs.FXMProjection.LLMContext
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
		ChangeWindow:      cloneChangeWindow(req.ChangeWindow),
		ActiveProblemMap:  nil,
		RelevantSections:  nil,
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
	if len(scope.Source.FocusIDs) == 0 && listenScopeShouldInferFocusIDs(scope.Source.Mode) {
		for _, obj := range objects {
			if strings.TrimSpace(obj.ID) != "" {
				scope.Source.FocusIDs = append(scope.Source.FocusIDs, obj.ID)
			}
		}
	}
	return scope
}

func listenScopeShouldInferFocusIDs(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "full_project", "track_group":
		return false
	default:
		return true
	}
}

func cleanAnyString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func boolFromAnyOK(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		text := strings.ToLower(strings.TrimSpace(v))
		switch text {
		case "true", "yes", "1":
			return true, true
		case "false", "no", "0":
			return false, true
		default:
			return false, false
		}
	case float64:
		return v != 0, true
	case int:
		return v != 0, true
	case json.Number:
		n, err := v.Float64()
		if err == nil {
			return n != 0, true
		}
	}
	return false, false
}

func stringsFromAny(value any) []string {
	out := []string{}
	for _, raw := range anySlice(value) {
		text := cleanAnyString(raw)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func mapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 || string(data) == "null" {
		return nil
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		return nil
	}
	return row
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
