package acousticpackage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/projectstore"
)

const SchemaVersion = "acoustic_package_status.v0"

const (
	StatusReady    = "ready"
	StatusPartial  = "partial"
	StatusBuilding = "building"
	StatusStale    = "stale"
	StatusSuspect  = "suspect"
	StatusMissing  = "missing"
	StatusDeferred = "deferred"
	StatusFailed   = "failed"
)

type Identity struct {
	ProjectID         string  `json:"project_id,omitempty"`
	SessionID         string  `json:"session_id,omitempty"`
	TrackID           string  `json:"track_id,omitempty"`
	ClipID            string  `json:"clip_id,omitempty"`
	SourcePath        string  `json:"source_path,omitempty"`
	SourceHash        string  `json:"source_hash,omitempty"`
	SourceFingerprint string  `json:"source_fingerprint,omitempty"`
	SourceRevision    string  `json:"source_revision,omitempty"`
	ClipRevision      string  `json:"clip_revision,omitempty"`
	RenderRevision    string  `json:"render_revision,omitempty"`
	AnalyzerRevision  string  `json:"analyzer_revision,omitempty"`
	DurationSec       float64 `json:"duration_seconds,omitempty"`
	ClipStartSec      float64 `json:"clip_start_seconds,omitempty"`
}

type Snapshot struct {
	SchemaVersion string   `json:"schema_version"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
	Packages      []Status `json:"packages,omitempty"`
}

type Status struct {
	SchemaVersion     string                 `json:"schema_version"`
	Status            string                 `json:"status,omitempty"`
	ProjectID         string                 `json:"project_id,omitempty"`
	SessionID         string                 `json:"session_id,omitempty"`
	TrackID           string                 `json:"track_id,omitempty"`
	ClipID            string                 `json:"clip_id,omitempty"`
	SourceHash        string                 `json:"source_hash,omitempty"`
	SourceFingerprint string                 `json:"source_fingerprint,omitempty"`
	SourceRevision    string                 `json:"source_revision,omitempty"`
	ClipRevision      string                 `json:"clip_revision,omitempty"`
	RenderRevision    string                 `json:"render_revision,omitempty"`
	AnalyzerRevision  string                 `json:"analyzer_revision,omitempty"`
	SourcePath        string                 `json:"source_path,omitempty"`
	DurationSec       float64                `json:"duration_seconds,omitempty"`
	ClipStartSec      float64                `json:"clip_start_seconds,omitempty"`
	SourceIdentity    map[string]any         `json:"source_identity,omitempty"`
	PackageLayers     map[string]LayerStatus `json:"package_layers"`
	UpdatedAt         string                 `json:"updated_at,omitempty"`
	Audit             []AuditEntry           `json:"audit,omitempty"`
}

type LayerStatus struct {
	Status    string                   `json:"status"`
	Features  map[string]FeatureStatus `json:"features"`
	UpdatedAt string                   `json:"updated_at,omitempty"`
	Reason    string                   `json:"reason,omitempty"`
}

type FeatureStatus struct {
	Status    string         `json:"status"`
	Source    string         `json:"source,omitempty"`
	Progress  Progress       `json:"progress,omitempty"`
	Ref       map[string]any `json:"ref,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
	Reason    string         `json:"reason,omitempty"`
}

type Progress struct {
	TileCountSeen     int     `json:"tile_count_seen,omitempty"`
	TileCountExpected int     `json:"tile_count_expected,omitempty"`
	CoverageSeconds   float64 `json:"coverage_seconds,omitempty"`
	CoverageRatio     float64 `json:"coverage_ratio,omitempty"`
	UpdatedAt         string  `json:"updated_at,omitempty"`
	Reason            string  `json:"reason,omitempty"`
}

type AuditEntry struct {
	Source    string `json:"source,omitempty"`
	Reason    string `json:"reason,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type Store struct {
	Path string
	Now  func() time.Time
}

// Status stores are shared by concurrent CCB requests for one project. Keep
// the read-modify-write cycle path-locked in-process; the atomic rename below
// also prevents unrelated readers from observing a partially written JSON file.
var statusStorePathLocks sync.Map // map[string]*sync.RWMutex

func statusStorePathLock(path string) *sync.RWMutex {
	key := filepath.Clean(path)
	lock, _ := statusStorePathLocks.LoadOrStore(key, &sync.RWMutex{})
	return lock.(*sync.RWMutex)
}

func (s Store) resolvedPath() string {
	if path := strings.TrimSpace(s.Path); path != "" {
		return filepath.Clean(path)
	}
	return DefaultStorePath(nil)
}

func NewStore(path string) Store {
	if strings.TrimSpace(path) == "" {
		path = DefaultStorePath(nil)
	}
	return Store{Path: filepath.Clean(path), Now: time.Now}
}

func DefaultStorePath(args map[string]any) string {
	for _, key := range []string{"acoustic_package_status_path", "acoustic_package_store_path"} {
		if path := cleanString(args[key]); path != "" {
			return filepath.Clean(path)
		}
	}
	if path := strings.TrimSpace(os.Getenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH")); path != "" {
		return filepath.Clean(path)
	}
	if roots, ok := projectstore.Current(); ok {
		return filepath.Join(roots.Derived, "acoustic_package_status.json")
	}
	if root := strings.TrimSpace(os.Getenv("VIT_MIXBOARD_ROOT")); root != "" {
		return filepath.Join(filepath.Dir(filepath.Clean(root)), "acoustic_package_status.json")
	}
	if devRoot := strings.TrimSpace(os.Getenv("VIT_DAW_DEV_ROOT")); devRoot != "" {
		root := filepath.Clean(devRoot)
		if _, err := os.Stat(filepath.Join(root, "VitApp", "Workspace")); err == nil {
			return filepath.Join(root, "VitApp", "Workspace", "Artifacts", "acoustic_package_status.json")
		}
	}
	return filepath.Join(os.TempDir(), "vit-daw-unbound", fmt.Sprint(os.Getpid()), "acoustic_package_status.json")
}

func (s Store) Read() (Snapshot, error) {
	path := s.resolvedPath()
	lock := statusStorePathLock(path)
	lock.RLock()
	defer lock.RUnlock()
	return readStatusSnapshot(path)
}

func readStatusSnapshot(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{SchemaVersion: SchemaVersion}, nil
		}
		return Snapshot{}, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, err
	}
	if snap.SchemaVersion == "" {
		snap.SchemaVersion = SchemaVersion
	}
	return snap, nil
}

func (s Store) Upsert(status Status) (Snapshot, error) {
	path := s.resolvedPath()
	lock := statusStorePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	now := s.nowString()
	if status.SchemaVersion == "" {
		status.SchemaVersion = SchemaVersion
	}
	if status.UpdatedAt == "" {
		status.UpdatedAt = now
	}
	status.Status = rollupStatus(status.PackageLayers)
	status.Audit = append(status.Audit, AuditEntry{Source: "acoustic_package_store", Reason: "upsert", UpdatedAt: now})

	snap, err := readStatusSnapshot(path)
	if err != nil {
		return Snapshot{}, err
	}
	snap.SchemaVersion = SchemaVersion
	snap.UpdatedAt = now
	replaced := false
	for i := range snap.Packages {
		if samePackageIdentity(snap.Packages[i], status) {
			snap.Packages[i] = status
			replaced = true
			continue
		}
		if samePackageTarget(snap.Packages[i], status) && snap.Packages[i].SourceRevision != "" && status.SourceRevision != "" && snap.Packages[i].SourceRevision != status.SourceRevision {
			snap.Packages[i] = MarkStale(snap.Packages[i], "source_revision_changed", now)
		}
		if samePackageTarget(snap.Packages[i], status) && snap.Packages[i].ClipRevision != "" && status.ClipRevision != "" && snap.Packages[i].ClipRevision != status.ClipRevision {
			snap.Packages[i] = MarkStale(snap.Packages[i], "clip_revision_changed", now)
		}
		if samePackageTarget(snap.Packages[i], status) && snap.Packages[i].RenderRevision != "" && status.RenderRevision != "" && snap.Packages[i].RenderRevision != status.RenderRevision {
			snap.Packages[i] = MarkStale(snap.Packages[i], "render_revision_changed", now)
		}
	}
	if !replaced {
		snap.Packages = append(snap.Packages, status)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Snapshot{}, err
	}
	data, err := json.MarshalIndent(snap, "", "\t")
	if err != nil {
		return Snapshot{}, err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".acoustic_package_status-*.tmp")
	if err != nil {
		return Snapshot{}, err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		return Snapshot{}, err
	}
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return Snapshot{}, err
	}
	if err := temp.Close(); err != nil {
		return Snapshot{}, err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

func (s Store) Find(identity Identity) (Status, bool, error) {
	snap, err := s.Read()
	if err != nil {
		return Status{}, false, err
	}
	needle := statusFromIdentity(identity, "")
	for _, row := range snap.Packages {
		if samePackageIdentity(row, needle) {
			return row, true, nil
		}
	}
	return Status{}, false, nil
}

func (s Store) nowString() string {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	return now().UTC().Format(time.RFC3339Nano)
}

func IdentityFromMaps(projectState, resolved, args map[string]any) Identity {
	explicitSourceIdentity := firstNonEmpty(
		cleanString(args["file_path"]),
		cleanString(args["source_path"]),
		cleanString(args["source_revision"]),
		cleanString(args["source_fingerprint"]),
		cleanString(args["source_hash"]),
	) != ""
	identity := Identity{
		ProjectID:  firstNonEmpty(cleanString(args["project_id"]), cleanString(projectState["project_id"]), cleanString(projectState["id"]), cleanString(projectState["project_path"]), "current"),
		SessionID:  firstNonEmpty(cleanString(args["mix_session_id"]), cleanString(args["session_id"]), cleanString(projectState["mix_session_id"]), cleanString(projectState["session_id"])),
		TrackID:    firstNonEmpty(cleanString(resolved["track_id"]), cleanString(args["track_id"]), targetTrackID(args)),
		ClipID:     firstNonEmpty(cleanString(resolved["clip_id"]), cleanString(args["clip_id"])),
		SourcePath: firstNonEmpty(cleanString(resolved["file_path"]), cleanString(resolved["source_path"]), cleanString(resolved["current_source_path"]), cleanString(args["file_path"]), cleanString(args["source_path"])),
		SourceHash: firstNonEmpty(cleanString(resolved["source_hash"]), cleanString(args["source_hash"])),
	}
	identity.DurationSec = firstPositive(numberValue(resolved["duration_seconds"]), numberValue(resolved["length_seconds"]), numberValue(resolved["duration"]), numberValue(args["duration_seconds"]), numberValue(args["duration"]))
	identity.ClipStartSec = firstNumber(resolved, "clip_start_seconds", "start_seconds", "start_time_seconds")
	if explicitSourceIdentity {
		identity.TrackID = firstNonEmpty(cleanString(args["track_id"]), targetTrackID(args), cleanString(resolved["track_id"]))
		identity.ClipID = firstNonEmpty(cleanString(args["clip_id"]), cleanString(resolved["clip_id"]))
		identity.SourcePath = firstNonEmpty(cleanString(args["file_path"]), cleanString(args["source_path"]), cleanString(resolved["file_path"]), cleanString(resolved["source_path"]), cleanString(resolved["current_source_path"]))
		identity.SourceHash = firstNonEmpty(cleanString(args["source_hash"]), cleanString(resolved["source_hash"]))
		identity.DurationSec = firstPositive(numberValue(args["duration_seconds"]), numberValue(args["duration"]), numberValue(resolved["duration_seconds"]), numberValue(resolved["length_seconds"]), numberValue(resolved["duration"]))
		identity.ClipStartSec = firstNumber(args, "clip_start_seconds", "start_seconds", "start_time_seconds")
		if identity.ClipStartSec == 0 {
			identity.ClipStartSec = firstNumber(resolved, "clip_start_seconds", "start_seconds", "start_time_seconds")
		}
	}
	if identity.SourcePath == "" || identity.DurationSec <= 0 || identity.ClipID == "" {
		fillIdentityFromProjectState(&identity, projectState)
	}
	if explicitSourceIdentity {
		identity.SourceFingerprint = firstNonEmpty(cleanString(args["source_fingerprint"]), cleanString(resolved["source_fingerprint"]))
		identity.SourceRevision = firstNonEmpty(cleanString(args["source_revision"]), cleanString(resolved["source_revision"]), identity.SourceFingerprint)
	} else {
		identity.SourceFingerprint = firstNonEmpty(cleanString(resolved["source_fingerprint"]), cleanString(args["source_fingerprint"]))
		identity.SourceRevision = firstNonEmpty(cleanString(resolved["source_revision"]), cleanString(args["source_revision"]), identity.SourceFingerprint)
	}
	if identity.SourceRevision == "" {
		identity.SourceRevision = computeSourceRevision(identity)
	}
	if identity.SourceFingerprint == "" {
		identity.SourceFingerprint = identity.SourceRevision
	}
	if explicitSourceIdentity {
		identity.ClipRevision = firstNonEmpty(cleanString(args["clip_revision"]), cleanString(resolved["clip_revision"]))
	} else {
		identity.ClipRevision = firstNonEmpty(cleanString(resolved["clip_revision"]), cleanString(args["clip_revision"]))
	}
	if identity.ClipRevision == "" {
		identity.ClipRevision = ComputeClipRevision(identity)
	}
	identity.RenderRevision = firstNonEmpty(cleanString(resolved["render_revision"]), cleanString(args["render_revision"]))
	identity.AnalyzerRevision = firstNonEmpty(cleanString(resolved["analyzer_revision"]), cleanString(args["analyzer_revision"]), "agent_lightweight_o1")
	return identity
}

func BuildStatus(identity Identity, featureSnapshot map[string]any, now, source string) Status {
	if strings.TrimSpace(now) == "" {
		now = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if strings.TrimSpace(source) == "" {
		source = "mixboard_feature_snapshot"
	}
	identity = canonicalizeIdentityFromFeatureSnapshot(identity, featureSnapshot)
	status := statusFromIdentity(identity, now)
	waveform := bestFeatureRow(identity, featureSnapshot, "waveform_envelope", "track_waveform_envelopes")
	spectral := bestFeatureRow(identity, featureSnapshot, "spectrogram_tiles", "spectrogram_tile_rows", "spectrogram_tiles_rows")
	band := bestFeatureRow(identity, featureSnapshot, "band_energy_summary", "band_energy_summaries")
	stereo := bestFeatureRow(identity, featureSnapshot, "stereo_relation_summary", "stereo_relation_summaries")
	loudness := bestFeatureRow(identity, featureSnapshot, "loudness_summary", "loudness_summaries")
	realtimeBand := bestFeatureRow(identity, featureSnapshot, "realtime_band_energy_summary", "realtime_band_energy_summaries")
	realtimeStereo := bestFeatureRow(identity, featureSnapshot, "realtime_stereo_relation_summary", "realtime_stereo_relation_summaries")
	l2RenderProbe := bestFeatureRow(identity, featureSnapshot, "l2_render_probe", "l2_render_probes")

	status.PackageLayers["l1_static"] = LayerStatus{
		Features: map[string]FeatureStatus{
			"waveform_envelope": featureFromRow("waveform_envelope", waveform, identity, source, now),
			"peak_rms_summary":  peakRMSSummaryFromWaveform(waveform, identity, source, now),
			"time_energy":       timeEnergyFromWaveform(waveform, identity, source, now),
		},
		UpdatedAt: now,
	}
	status.PackageLayers["l2_realtime"] = LayerStatus{
		Features: map[string]FeatureStatus{
			"live_meter":                  liveMeterFromRealtimeRows(realtimeBand, realtimeStereo, identity, source, now),
			"realtime_spectrum":           realtimeFeatureFromRow("realtime_spectrum", realtimeBand, identity, source, now),
			"render_probe":                renderProbeFeatureFromRow(l2RenderProbe, identity, source, now),
			"post_fx_meter":               postFXMeterFromRenderProbe(l2RenderProbe, identity, source, now),
			"realtime_stereo_correlation": realtimeFeatureFromRow("realtime_stereo_correlation", realtimeStereo, identity, source, now),
		},
		UpdatedAt: now,
	}
	status.PackageLayers["l3_deep"] = LayerStatus{
		Features: map[string]FeatureStatus{
			"spectrogram_tiles":       featureFromRow("spectrogram_tiles", spectral, identity, source, now),
			"band_energy_summary":     featureFromRow("band_energy_summary", band, identity, source, now),
			"stereo_relation_summary": featureFromRow("stereo_relation_summary", stereo, identity, source, now),
			"loudness_summary":        featureFromRow("loudness_summary", loudness, identity, source, now),
			"lufs_analysis":           deferredFeature("phase_5_deferred", now),
			"masking_analysis":        deferredFeature("phase_5_deferred", now),
			"reference_match":         deferredFeature("phase_5_deferred", now),
		},
		UpdatedAt: now,
	}
	for key, layer := range status.PackageLayers {
		layer.Status = rollupLayerStatus(layer.Features)
		status.PackageLayers[key] = layer
	}
	status.Status = rollupStatus(status.PackageLayers)
	status.Audit = append(status.Audit, AuditEntry{Source: source, Reason: "derived_from_feature_snapshot", UpdatedAt: now})
	return status
}

func MergeStatus(stored, derived Status) Status {
	if stored.SchemaVersion == "" {
		return derived
	}
	if derived.SchemaVersion == "" {
		return stored
	}
	if !samePackageIdentity(stored, derived) {
		return derived
	}
	out := derived
	for layerName, storedLayer := range stored.PackageLayers {
		layer := out.PackageLayers[layerName]
		if layer.Features == nil {
			layer.Features = map[string]FeatureStatus{}
		}
		for featureName, storedFeature := range storedLayer.Features {
			if shouldMergeStoredFeature(layerName, featureName, storedFeature, layer.Features[featureName], identityFromStatus(derived)) {
				layer.Features[featureName] = storedFeature
			}
		}
		layer.Status = rollupLayerStatus(layer.Features)
		out.PackageLayers[layerName] = layer
	}
	out.Status = rollupStatus(out.PackageLayers)
	out.Audit = append(out.Audit, stored.Audit...)
	return out
}

func shouldMergeStoredFeature(layerName, _ string, storedFeature, derivedFeature FeatureStatus, identity Identity) bool {
	if storedFeature.Status == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(layerName), "l2_realtime") && storedFeature.Status == StatusStale {
		return false
	}
	if featureRefIdentityMismatchReason(storedFeature, identity) != "" {
		return false
	}
	if featureRefLacksSourceIdentity(storedFeature, identity) && featureStatusHasEvidence(derivedFeature) {
		return false
	}
	storedPriority := featurePriority(storedFeature.Status)
	derivedPriority := featurePriority(derivedFeature.Status)
	if storedPriority < derivedPriority {
		return false
	}
	storedScore := featureStatusCompletenessScore(storedFeature)
	derivedScore := featureStatusCompletenessScore(derivedFeature)
	if storedPriority == derivedPriority {
		return storedScore >= derivedScore
	}
	if derivedFeature.Status == StatusPartial || derivedFeature.Status == StatusReady {
		if featureStatusHasEvidence(derivedFeature) && derivedScore > storedScore+0.001 {
			return false
		}
	}
	return true
}

func featureRefIdentityMismatchReason(feature FeatureStatus, identity Identity) string {
	if len(feature.Ref) == 0 {
		return ""
	}
	return rowIdentityMismatchReason(feature.Ref, identity)
}

func featureRefLacksSourceIdentity(feature FeatureStatus, identity Identity) bool {
	if len(feature.Ref) == 0 || !identityHasSourceIdentity(identity) {
		return false
	}
	return !rowHasSourceIdentity(feature.Ref)
}

func featureStatusHasEvidence(feature FeatureStatus) bool {
	if feature.Progress.TileCountSeen > 0 || feature.Progress.TileCountExpected > 0 || feature.Progress.CoverageSeconds > 0 || feature.Progress.CoverageRatio > 0 {
		return true
	}
	if len(feature.Ref) == 0 {
		return false
	}
	if feature.Ref["bands"] != nil || feature.Ref["correlation_estimate"] != nil || feature.Ref["balance_db"] != nil || feature.Ref["quality_evidence"] != nil {
		return true
	}
	if numberValue(firstPresent(feature.Ref, "peak_abs", "rms", "sum_abs", "max_abs")) > 0 {
		return true
	}
	if numberValue(firstPresent(feature.Ref, "tile_count_seen", "tile_count_parsed", "tile_count")) > 0 {
		return true
	}
	return firstPositive(numberValue(feature.Ref["coverage_seconds"]), numberValue(feature.Ref["total_duration"])) > 0
}

func featureStatusCompletenessScore(feature FeatureStatus) float64 {
	seen := float64(feature.Progress.TileCountSeen)
	if seen <= 0 && len(feature.Ref) > 0 {
		seen = numberValue(firstPresent(feature.Ref, "tile_count_parsed", "tile_count_seen", "tile_count"))
	}
	expected := float64(feature.Progress.TileCountExpected)
	if expected <= 0 && len(feature.Ref) > 0 {
		expected = numberValue(firstPresent(feature.Ref, "tile_count_expected", "tile_count_total"))
	}
	coverage := feature.Progress.CoverageSeconds
	if coverage <= 0 && len(feature.Ref) > 0 {
		coverage = firstPositive(numberValue(feature.Ref["coverage_seconds"]), numberValue(feature.Ref["total_duration"]), rowDurationSeconds(feature.Ref))
	}
	score := coverage
	if expected > 0 && seen >= expected {
		score += 10000
	} else if expected > 0 && seen > 0 {
		score += (seen / expected) * 1000
	}
	score += seen * 10
	if feature.Progress.CoverageRatio > 0 {
		score += feature.Progress.CoverageRatio * 100
	}
	if len(feature.Ref) > 0 {
		if cleanString(feature.Ref["source_revision"]) != "" || cleanString(feature.Ref["source_fingerprint"]) != "" || cleanString(feature.Ref["source_hash"]) != "" || rowSourcePath(feature.Ref) != "" {
			score += 100
		}
		if feature.Ref["bands"] != nil || feature.Ref["correlation_estimate"] != nil || feature.Ref["balance_db"] != nil || feature.Ref["quality_evidence"] != nil {
			score += 100
		}
		if numberValue(firstPresent(feature.Ref, "peak_abs", "rms", "sum_abs", "max_abs")) > 0 {
			score += 25
		}
	}
	if feature.Status == StatusReady {
		score += 5
	}
	return score
}

func MarkBackgroundRequested(status Status, source, reason, now string) Status {
	if now == "" {
		now = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if source == "" {
		source = "mix.observe_background_fill"
	}
	if reason == "" {
		reason = "background_fill_requested_after_read_first_status"
	}
	for _, layerName := range []string{"l1_static", "l3_deep"} {
		layer := status.PackageLayers[layerName]
		if layer.Features == nil {
			layer.Features = map[string]FeatureStatus{}
		}
		for featureName, feature := range layer.Features {
			if !backgroundFillEligible(layerName, featureName, feature.Status) {
				continue
			}
			feature.Status = StatusBuilding
			feature.Source = source
			feature.Reason = reason
			feature.UpdatedAt = now
			feature.Progress = Progress{UpdatedAt: now, Reason: reason}
			feature.Ref = nil
			layer.Features[featureName] = feature
		}
		layer.Status = rollupLayerStatus(layer.Features)
		layer.UpdatedAt = now
		status.PackageLayers[layerName] = layer
	}
	status.UpdatedAt = now
	status.Status = rollupStatus(status.PackageLayers)
	status.Audit = append(status.Audit, AuditEntry{Source: source, Reason: reason, UpdatedAt: now})
	return status
}

func NeedsBackgroundFill(status Status) bool {
	for layerName, featureNames := range map[string][]string{
		"l1_static": []string{"waveform_envelope", "peak_rms_summary", "time_energy"},
		"l3_deep":   []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary"},
	} {
		layer := status.PackageLayers[layerName]
		for _, name := range featureNames {
			switch layer.Features[name].Status {
			case "", StatusMissing, StatusStale:
				return true
			}
		}
	}
	return false
}

func MarkStale(status Status, reason, now string) Status {
	if reason == "" {
		reason = "stale"
	}
	for layerName, layer := range status.PackageLayers {
		for name, feature := range layer.Features {
			if feature.Status == StatusDeferred {
				continue
			}
			feature.Status = StatusStale
			feature.Reason = reason
			feature.Progress.Reason = reason
			feature.UpdatedAt = now
			layer.Features[name] = feature
		}
		layer.Status = StatusStale
		layer.Reason = reason
		layer.UpdatedAt = now
		status.PackageLayers[layerName] = layer
	}
	status.Status = StatusStale
	status.UpdatedAt = now
	status.Audit = append(status.Audit, AuditEntry{Source: "acoustic_package_store", Reason: reason, UpdatedAt: now})
	return status
}

func ToMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func statusFromIdentity(identity Identity, now string) Status {
	return Status{
		SchemaVersion:     SchemaVersion,
		Status:            StatusMissing,
		ProjectID:         identity.ProjectID,
		SessionID:         identity.SessionID,
		TrackID:           identity.TrackID,
		ClipID:            identity.ClipID,
		SourceHash:        identity.SourceHash,
		SourceFingerprint: identity.SourceFingerprint,
		SourceRevision:    identity.SourceRevision,
		ClipRevision:      identity.ClipRevision,
		RenderRevision:    identity.RenderRevision,
		AnalyzerRevision:  identity.AnalyzerRevision,
		SourcePath:        identity.SourcePath,
		DurationSec:       round3(identity.DurationSec),
		ClipStartSec:      round3(identity.ClipStartSec),
		SourceIdentity:    sourceIdentityMap(identity),
		PackageLayers:     map[string]LayerStatus{},
		UpdatedAt:         now,
	}
}

func canonicalizeIdentityFromFeatureSnapshot(identity Identity, snapshot map[string]any) Identity {
	if len(snapshot) == 0 {
		return identity
	}
	candidates := []map[string]any{
		bestFeatureRow(identity, snapshot, "waveform_envelope", "track_waveform_envelopes"),
		bestFeatureRow(identity, snapshot, "spectrogram_tiles", "spectrogram_tile_rows", "spectrogram_tiles_rows"),
		bestFeatureRow(identity, snapshot, "band_energy_summary", "band_energy_summaries"),
		bestFeatureRow(identity, snapshot, "stereo_relation_summary", "stereo_relation_summaries"),
		bestFeatureRow(identity, snapshot, "loudness_summary", "loudness_summaries"),
		bestFeatureRow(identity, snapshot, "l2_render_probe", "l2_render_probes"),
	}
	var best map[string]any
	for _, row := range candidates {
		if len(row) == 0 || rowIdentityMismatchReason(row, identity) != "" {
			continue
		}
		if firstNonEmpty(cleanString(row["source_revision"]), cleanString(row["source_fingerprint"]), cleanString(row["source_hash"]), rowSourcePath(row)) == "" {
			continue
		}
		if best == nil || featureRowCompletenessScore(row) > featureRowCompletenessScore(best) {
			best = row
		}
	}
	if best == nil {
		return identity
	}
	previousRevision := identity.SourceRevision
	if sourcePath := rowSourcePath(best); sourcePath != "" {
		identity.SourcePath = sourcePath
	}
	if sourceHash := cleanString(best["source_hash"]); sourceHash != "" {
		identity.SourceHash = sourceHash
	}
	if sourceRevision := cleanString(best["source_revision"]); sourceRevision != "" {
		identity.SourceRevision = sourceRevision
	}
	if sourceFingerprint := cleanString(best["source_fingerprint"]); sourceFingerprint != "" {
		identity.SourceFingerprint = sourceFingerprint
	} else if identity.SourceFingerprint == "" || identity.SourceFingerprint == previousRevision {
		identity.SourceFingerprint = identity.SourceRevision
	}
	if duration := rowDurationSeconds(best); duration > 0 {
		identity.DurationSec = duration
	}
	if clipRevision := cleanString(best["clip_revision"]); clipRevision != "" {
		identity.ClipRevision = clipRevision
	} else if previousRevision != "" && identity.SourceRevision != "" && previousRevision != identity.SourceRevision {
		identity.ClipRevision = ComputeClipRevision(identity)
	}
	if renderRevision := cleanString(best["render_revision"]); renderRevision != "" {
		identity.RenderRevision = renderRevision
	}
	if analyzerRevision := cleanString(best["analyzer_revision"]); analyzerRevision != "" {
		identity.AnalyzerRevision = analyzerRevision
	}
	return identity
}

func sourceIdentityMap(identity Identity) map[string]any {
	out := map[string]any{}
	for key, value := range map[string]string{
		"project_id":         identity.ProjectID,
		"session_id":         identity.SessionID,
		"track_id":           identity.TrackID,
		"clip_id":            identity.ClipID,
		"source_path":        identity.SourcePath,
		"source_hash":        identity.SourceHash,
		"source_fingerprint": identity.SourceFingerprint,
		"source_revision":    identity.SourceRevision,
		"clip_revision":      identity.ClipRevision,
		"render_revision":    identity.RenderRevision,
		"analyzer_revision":  identity.AnalyzerRevision,
	} {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	if identity.DurationSec > 0 {
		out["duration_seconds"] = round3(identity.DurationSec)
	}
	out["clip_start_seconds"] = round3(identity.ClipStartSec)
	return out
}

func bestFeatureRow(identity Identity, snapshot map[string]any, topLevelKey string, arrayKeys ...string) map[string]any {
	var best map[string]any
	candidates := make([]map[string]any, 0, 4)
	if row := rowMap(snapshot[topLevelKey]); len(row) > 0 {
		candidates = append(candidates, row)
	}
	for _, key := range arrayKeys {
		candidates = append(candidates, mapRows(snapshot[key])...)
	}
	for _, row := range candidates {
		if len(row) == 0 {
			continue
		}
		if best == nil {
			best = row
			continue
		}
		rowMatchRank := featureRowIdentityMatchRank(row, identity)
		bestMatchRank := featureRowIdentityMatchRank(best, identity)
		if rowMatchRank > bestMatchRank {
			best = row
			continue
		}
		if rowMatchRank < bestMatchRank {
			continue
		}
		rowMatches := rowIdentityMismatchReason(row, identity) == ""
		bestMatches := rowIdentityMismatchReason(best, identity) == ""
		if rowMatches && !bestMatches {
			best = row
			continue
		}
		if !rowMatches && bestMatches {
			continue
		}
		rowStatus := normalizeFeatureStatus(row, identity)
		bestStatus := normalizeFeatureStatus(best, identity)
		if featurePriority(rowStatus) > featurePriority(bestStatus) {
			best = row
			continue
		}
		if featurePriority(rowStatus) == featurePriority(bestStatus) && featureRowCompletenessScore(row) > featureRowCompletenessScore(best) {
			best = row
			continue
		}
	}
	return best
}

func featureRowIdentityMatchRank(row map[string]any, identity Identity) int {
	if len(row) == 0 {
		return 0
	}
	switch rowIdentityMismatchReason(row, identity) {
	case "":
		return 4
	case "incomplete_source_identity":
		if rowHasCurrentTargetAnchor(row, identity) {
			return 3
		}
		return 1
	default:
		return 0
	}
}

func featureRowCompletenessScore(row map[string]any) float64 {
	if len(row) == 0 {
		return 0
	}
	seen := numberValue(firstPresent(row, "tile_count_seen", "tile_count_parsed", "tile_count"))
	expected := numberValue(firstPresent(row, "tile_count_expected", "tile_count_total"))
	coverage := firstPositive(numberValue(row["coverage_seconds"]), numberValue(row["total_duration"]), rowDurationSeconds(row))
	score := coverage
	if expected > 0 && seen >= expected {
		score += 10000
	} else {
		score += seen
	}
	if cleanString(row["source_revision"]) != "" {
		score += 100
	}
	if cleanString(row["quality_status"]) == StatusReady || cleanString(row["quality_reason"]) == "ok" {
		score += 10
	}
	if cleanString(row["clip_revision"]) != "" {
		score += 1
	}
	if cleanString(row["render_revision"]) != "" {
		score += 1
	}
	return score
}

func featureFromRow(featureName string, row map[string]any, identity Identity, fallbackSource, now string) FeatureStatus {
	status := normalizeFeatureStatus(row, identity)
	progress := progressFromRow(row, identity)
	if progress.UpdatedAt == "" {
		progress.UpdatedAt = firstNonEmpty(cleanString(row["updated_at"]), now)
	}
	if progress.Reason == "" {
		progress.Reason = cleanString(row["reason"])
	}
	feature := FeatureStatus{
		Status:    status,
		Source:    firstNonEmpty(cleanString(row["source"]), fallbackSource),
		Progress:  progress,
		Ref:       compactRef(row),
		UpdatedAt: firstNonEmpty(cleanString(row["updated_at"]), now),
		Reason:    cleanString(row["reason"]),
	}
	if mismatchReason := rowIdentityMismatchReason(row, identity); feature.Status == StatusStale && feature.Reason == "" && mismatchReason != "" {
		feature.Reason = mismatchReason
	}
	if feature.Status == StatusStale && rowIdentityMismatchReason(row, identity) != "" {
		feature.Ref = nil
		feature.Progress = Progress{UpdatedAt: firstNonEmpty(cleanString(row["updated_at"]), now), Reason: feature.Reason}
	}
	if feature.Status == "" {
		feature.Status = StatusMissing
	}
	if feature.Status == StatusBuilding && feature.Reason == "" {
		feature.Reason = "background_fill_in_progress"
		if feature.Progress.Reason == "" {
			feature.Progress.Reason = feature.Reason
		}
	}
	return feature
}

func peakRMSSummaryFromWaveform(row map[string]any, identity Identity, source, now string) FeatureStatus {
	feature := featureFromRow("peak_rms_summary", row, identity, source, now)
	if feature.Status == StatusReady {
		if numberValue(row["rms"]) <= 0 && numberValue(row["peak_abs"]) <= 0 && row["rms_dbfs"] == nil && row["peak_dbfs"] == nil {
			feature.Status = StatusPartial
			feature.Reason = "waveform_ready_without_peak_rms_metrics"
		}
	}
	feature.Ref = compactRef(selectKeys(row, "rms", "peak_abs", "rms_dbfs", "peak_dbfs", "headroom_db", "crest_db", "track_id", "clip_id", "file_path", "request_id", "updated_at"))
	return feature
}

func timeEnergyFromWaveform(row map[string]any, identity Identity, source, now string) FeatureStatus {
	feature := featureFromRow("time_energy", row, identity, source, now)
	segments := mapRows(row["time_segments"])
	switch {
	case len(segments) > 0 && (feature.Status == StatusReady || feature.Status == StatusPartial):
		feature.Status = StatusReady
	case feature.Status == StatusReady:
		feature.Status = StatusPartial
		feature.Reason = "waveform_ready_without_time_segments"
	case feature.Status == StatusMissing:
		feature.Reason = firstNonEmpty(feature.Reason, "waveform_envelope_required_for_time_energy")
	}
	feature.Ref = map[string]any{"time_segment_count": len(segments)}
	if len(segments) > 0 {
		feature.Ref["time_segments"] = capRows(segments, 16)
	}
	return feature
}

func liveMeterFromRealtimeRows(bandRow, stereoRow map[string]any, identity Identity, source, now string) FeatureStatus {
	row := bandRow
	if featureStatusPriority(normalizeFeatureStatus(stereoRow, identity)) > featureStatusPriority(normalizeFeatureStatus(row, identity)) {
		row = stereoRow
	}
	feature := realtimeFeatureFromRow("live_meter", row, identity, source, now)
	if feature.Status == StatusReady || feature.Status == StatusPartial {
		feature.Source = firstNonEmpty(feature.Source, "live_level_meter")
	}
	return feature
}

func realtimeFeatureFromRow(featureName string, row map[string]any, identity Identity, source, now string) FeatureStatus {
	if len(row) == 0 {
		return deferredFeature("phase_4_5_l2_realtime_requires_playback", now)
	}
	feature := featureFromRow(featureName, row, identity, source, now)
	if feature.Status == StatusStale && realtimeRowShouldDefer(row, featureName) {
		return deferredFeature("l2_realtime_requires_current_playback_capture", now)
	}
	if feature.Status == StatusMissing && (feature.Reason == "" || feature.Reason == "awaiting_live_level_meter") {
		return deferredFeature("phase_4_5_l2_realtime_requires_playback", now)
	}
	return feature
}

func postFXMeterFromRenderProbe(row map[string]any, identity Identity, source, now string) FeatureStatus {
	if len(row) == 0 {
		return deferredFeature("l2_render_probe_missing", now)
	}
	feature := featureFromRow("post_fx_meter", row, identity, source, now)
	if feature.Source == "" || feature.Source == source {
		feature.Source = "l2_render_probe"
	}
	return feature
}

func renderProbeFeatureFromRow(row map[string]any, identity Identity, source, now string) FeatureStatus {
	if len(row) == 0 {
		return deferredFeature("l2_render_probe_missing", now)
	}
	return featureFromRow("render_probe", row, identity, source, now)
}

func realtimeRowShouldDefer(row map[string]any, featureName string) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(featureName)), "realtime_") || strings.EqualFold(strings.TrimSpace(featureName), "live_meter") {
		return true
	}
	if strings.EqualFold(cleanString(row["layer"]), "l2_realtime") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(cleanString(row["feature_type"]))), "realtime_") {
		return true
	}
	if strings.EqualFold(cleanString(row["source_kind"]), "realtime_level_meter") {
		return true
	}
	return false
}

func featureStatusPriority(status string) int {
	switch status {
	case StatusReady:
		return 7
	case StatusPartial:
		return 6
	case StatusBuilding:
		return 5
	case StatusSuspect:
		return 4
	case StatusStale:
		return 3
	case StatusFailed:
		return 2
	case StatusMissing:
		return 1
	case StatusDeferred:
		return 0
	default:
		return 0
	}
}

func deferredFeature(reason, now string) FeatureStatus {
	return FeatureStatus{
		Status:    StatusDeferred,
		Source:    "phase_4_5_schema",
		Reason:    reason,
		UpdatedAt: now,
		Progress:  Progress{UpdatedAt: now, Reason: reason},
	}
}

func normalizeFeatureStatus(row map[string]any, identity Identity) string {
	if len(row) == 0 {
		return StatusMissing
	}
	status := strings.ToLower(strings.TrimSpace(cleanString(row["status"])))
	derivation := strings.ToLower(strings.TrimSpace(cleanString(row["derivation_status"])))
	reason := strings.ToLower(strings.TrimSpace(cleanString(row["reason"])))
	mismatchReason := rowIdentityMismatchReason(row, identity)
	if mismatchReason != "" {
		// A generic "current" row can outlive the project that issued it while
		// numeric track/clip IDs are reused by the next project. Treating that
		// incomplete legacy request as still building suppresses the new
		// project's background fill forever. Current requests are stamped with
		// the live project ID before they reach this point; a project mismatch
		// is therefore stale even when the old row has no source path yet.
		if mismatchReason == "project_mismatch" || (mismatchReason == "incomplete_source_identity" && !genericProjectIdentity(identity.ProjectID)) {
			return StatusStale
		}
		if status == StatusBuilding && !rowHasSourceIdentity(row) {
			return StatusBuilding
		}
		if status == "requested" || status == "pending" {
			if rowHasSourceIdentity(row) {
				return StatusStale
			}
			return StatusBuilding
		}
		return StatusStale
	}
	if strings.Contains(derivation, "parse_failed") || strings.Contains(reason, "parse_failed") {
		return StatusFailed
	}
	switch status {
	case StatusReady, StatusPartial, StatusBuilding, StatusStale, StatusSuspect, StatusMissing, StatusDeferred, StatusFailed:
		return status
	case "requested", "pending":
		return StatusBuilding
	case "blocked", "unavailable", "invalid", "error":
		return StatusFailed
	case "":
		return StatusMissing
	default:
		return status
	}
}

func rowMatchesIdentity(row map[string]any, identity Identity) bool {
	return rowIdentityMismatchReason(row, identity) == ""
}

func rowIdentityMismatchReason(row map[string]any, identity Identity) string {
	if len(row) == 0 {
		return ""
	}
	if rowProject := cleanString(row["project_id"]); rowProject != "" && identity.ProjectID != "" && rowProject != identity.ProjectID && !sourceFileL3RowMatchesExactMaterial(row, identity) {
		return "project_mismatch"
	}
	if rowTrack := cleanString(row["track_id"]); rowTrack != "" && identity.TrackID != "" && rowTrack != identity.TrackID {
		return "track_mismatch"
	}
	if rowClip := cleanString(row["clip_id"]); rowClip != "" && identity.ClipID != "" && rowClip != identity.ClipID {
		return "clip_mismatch"
	}
	rowPath := rowSourcePath(row)
	if identityHasSourceIdentity(identity) && !rowHasSourceIdentity(row) {
		if rowHasCurrentTargetAnchor(row, identity) {
			return ""
		}
		return "incomplete_source_identity"
	}
	rowHash := cleanString(row["source_hash"])
	if rowHash != "" && identity.SourceHash != "" && rowHash != identity.SourceHash {
		return "source_hash_mismatch"
	}
	rowRev := firstNonEmpty(cleanString(row["source_revision"]), cleanString(row["source_fingerprint"]))
	identityRev := firstNonEmpty(identity.SourceRevision, identity.SourceFingerprint)
	sourceAliasMatch := false
	if rowRev != "" && identityRev != "" && rowRev != identityRev {
		sourceAliasMatch = sourceRevisionAliasesMatch(rowRev, identityRev, rowPath, rowDurationSeconds(row), identity)
		if !sourceAliasMatch {
			return "source_revision_mismatch"
		}
	}
	if rowClipRevision := cleanString(row["clip_revision"]); rowClipRevision != "" && identity.ClipRevision != "" && rowClipRevision != identity.ClipRevision && !sourceAliasMatch {
		return "clip_revision_mismatch"
	}
	if rowUsesRenderRevision(row) {
		if rowRenderRevision := cleanString(row["render_revision"]); rowRenderRevision != "" && identity.RenderRevision != "" && rowRenderRevision != identity.RenderRevision {
			return "render_revision_mismatch"
		}
	}
	if rowPath != "" && identity.SourcePath != "" && normalizePathForIdentity(rowPath) != normalizePathForIdentity(identity.SourcePath) {
		return "source_path_mismatch"
	}
	if rowDuration := rowDurationSeconds(row); rowDuration > 0 && identity.DurationSec > 0 && math.Abs(rowDuration-identity.DurationSec) > 0.25 {
		return "duration_mismatch"
	}
	return ""
}

// Source-file L3 facts describe the decoded source material before the current
// plug-in/fader render. They may therefore be reused across a session/project
// identity change, but only when the strong source revision/fingerprint and the
// exact current track/clip material agree. Render-dependent L2 rows never take
// this exception.
func sourceFileL3RowMatchesExactMaterial(row map[string]any, identity Identity) bool {
	if rowUsesRenderRevision(row) {
		return false
	}
	if rowTrack := cleanString(row["track_id"]); rowTrack != "" && identity.TrackID != "" && rowTrack != identity.TrackID {
		return false
	}
	if rowClip := cleanString(row["clip_id"]); rowClip != "" && identity.ClipID != "" && rowClip != identity.ClipID {
		return false
	}
	rowRevision := firstNonEmpty(cleanString(row["source_revision"]), cleanString(row["source_fingerprint"]))
	identityRevision := firstNonEmpty(identity.SourceRevision, identity.SourceFingerprint)
	if rowRevision == "" || identityRevision == "" {
		return false
	}
	if rowRevision != identityRevision && !sourceRevisionAliasesMatch(rowRevision, identityRevision, rowSourcePath(row), rowDurationSeconds(row), identity) {
		return false
	}
	if rowHash := cleanString(row["source_hash"]); rowHash != "" && identity.SourceHash != "" && rowHash != identity.SourceHash {
		return false
	}
	if rowPath := rowSourcePath(row); rowPath != "" && identity.SourcePath != "" && normalizePathForIdentity(rowPath) != normalizePathForIdentity(identity.SourcePath) {
		return false
	}
	return true
}

func rowUsesRenderRevision(row map[string]any) bool {
	featureType := strings.ToLower(strings.TrimSpace(cleanString(row["feature_type"])))
	switch featureType {
	case "spectral_field", "spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary", "l3_acoustic_summary":
		return false
	}
	source := strings.ToLower(strings.TrimSpace(cleanString(row["source"])))
	if strings.Contains(source, "kernel_l3_offline_analyzer") || strings.Contains(source, "spectral_tile_derived") {
		return false
	}
	return true
}

func rowHasCurrentTargetAnchor(row map[string]any, identity Identity) bool {
	if len(row) == 0 || cleanString(row["request_id"]) == "" {
		return false
	}
	if !genericProjectIdentity(identity.ProjectID) {
		rowProject := cleanString(row["project_id"])
		if rowProject == "" || rowProject != identity.ProjectID {
			return false
		}
	}
	rowClip := cleanString(row["clip_id"])
	if identity.ClipID == "" || rowClip == "" || rowClip != identity.ClipID {
		return false
	}
	rowTrack := cleanString(row["track_id"])
	if identity.TrackID != "" && (rowTrack == "" || rowTrack != identity.TrackID) {
		return false
	}
	return true
}

func genericProjectIdentity(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "current", "project_current", "current_project":
		return true
	default:
		return false
	}
}

type sourceRevisionDescriptor struct {
	Path        string
	Size        int64
	MTimeMillis int64
	LengthSec   float64
}

func sourceRevisionAliasesMatch(rowRevision, identityRevision, rowPath string, rowDuration float64, identity Identity) bool {
	rowDescriptor, rowOK := parseSourceRevisionDescriptor(rowRevision)
	identityDescriptor, identityOK := parseSourceRevisionDescriptor(identityRevision)
	switch {
	case rowOK && identityOK:
		return sourceRevisionDescriptorsMatch(rowDescriptor, identityDescriptor)
	case rowOK && isLegacySourceRevision(identityRevision):
		return sourceRevisionDescriptorMatchesIdentity(rowDescriptor, rowPath, rowDuration, identity)
	case identityOK && isLegacySourceRevision(rowRevision):
		return sourceRevisionDescriptorMatchesIdentity(identityDescriptor, rowPath, rowDuration, identity)
	default:
		return false
	}
}

func isLegacySourceRevision(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "rev_")
}

func parseSourceRevisionDescriptor(value string) (sourceRevisionDescriptor, bool) {
	value = strings.TrimSpace(value)
	if value == "" || !strings.Contains(value, "|") || !strings.Contains(value, "size=") {
		return sourceRevisionDescriptor{}, false
	}
	parts := strings.Split(value, "|")
	descriptor := sourceRevisionDescriptor{Path: strings.TrimSpace(parts[0])}
	for _, part := range parts[1:] {
		key, rawValue, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(key))
		rawValue = strings.TrimSpace(rawValue)
		switch key {
		case "source":
			if descriptor.Path == "" {
				descriptor.Path = rawValue
			}
		case "size":
			descriptor.Size = int64(numberValue(rawValue))
		case "mtime":
			descriptor.MTimeMillis = int64(numberValue(rawValue))
		case "length":
			descriptor.LengthSec = numberValue(rawValue)
		}
	}
	if descriptor.Path == "" || (descriptor.Size <= 0 && descriptor.MTimeMillis <= 0 && descriptor.LengthSec <= 0) {
		return sourceRevisionDescriptor{}, false
	}
	return descriptor, true
}

func sourceRevisionDescriptorsMatch(a, b sourceRevisionDescriptor) bool {
	if a.Path != "" && b.Path != "" && normalizePathForIdentity(a.Path) != normalizePathForIdentity(b.Path) {
		return false
	}
	if a.Size > 0 && b.Size > 0 && a.Size != b.Size {
		return false
	}
	if a.MTimeMillis > 0 && b.MTimeMillis > 0 && math.Abs(float64(a.MTimeMillis-b.MTimeMillis)) > float64(time.Second/time.Millisecond) {
		return false
	}
	if a.LengthSec > 0 && b.LengthSec > 0 && math.Abs(a.LengthSec-b.LengthSec) > 0.25 {
		return false
	}
	return true
}

func sourceRevisionDescriptorMatchesIdentity(descriptor sourceRevisionDescriptor, rowPath string, rowDuration float64, identity Identity) bool {
	evidence := false
	identityPath := firstNonEmpty(identity.SourcePath, rowPath)
	if descriptor.Path != "" && identityPath != "" {
		if normalizePathForIdentity(descriptor.Path) != normalizePathForIdentity(identityPath) {
			return false
		}
		evidence = true
	}
	identityDuration := firstPositive(identity.DurationSec, rowDuration)
	if descriptor.LengthSec > 0 && identityDuration > 0 {
		if math.Abs(descriptor.LengthSec-identityDuration) > 0.25 {
			return false
		}
		evidence = true
	}
	statPath := firstNonEmpty(identity.SourcePath, rowPath, descriptor.Path)
	if statPath != "" {
		if info, err := os.Stat(statPath); err == nil {
			if descriptor.Size > 0 && descriptor.Size != info.Size() {
				return false
			}
			if descriptor.MTimeMillis > 0 && math.Abs(float64(descriptor.MTimeMillis-info.ModTime().UTC().UnixMilli())) > float64(time.Second/time.Millisecond) {
				return false
			}
			evidence = true
		}
	}
	return evidence
}

// SourceRevisionMatchesIdentity validates a stored source revision against a
// current material identity without requiring callers to duplicate descriptor
// parsing or file-stat semantics. It is intentionally strict: an opaque
// revision needs an exact current revision alias, while a descriptor may be
// proven by its path plus current file metadata.
func SourceRevisionMatchesIdentity(revision string, identity Identity) bool {
	revision = strings.TrimSpace(revision)
	if revision == "" {
		return false
	}
	currentRevision := firstNonEmpty(identity.SourceRevision, identity.SourceFingerprint)
	if currentRevision != "" {
		return revision == currentRevision || sourceRevisionAliasesMatch(revision, currentRevision, identity.SourcePath, identity.DurationSec, identity)
	}
	if descriptor, ok := parseSourceRevisionDescriptor(revision); ok {
		return sourceRevisionDescriptorMatchesIdentity(descriptor, identity.SourcePath, identity.DurationSec, identity)
	}
	return isLegacySourceRevision(revision) && revision == ComputeSourceRevision(identity)
}

func identityHasSourceIdentity(identity Identity) bool {
	return strings.TrimSpace(identity.SourceRevision) != "" ||
		strings.TrimSpace(identity.SourceFingerprint) != "" ||
		strings.TrimSpace(identity.SourceHash) != "" ||
		strings.TrimSpace(identity.SourcePath) != "" ||
		identity.DurationSec > 0
}

func rowHasSourceIdentity(row map[string]any) bool {
	return cleanString(row["source_revision"]) != "" ||
		cleanString(row["source_fingerprint"]) != "" ||
		cleanString(row["source_hash"]) != "" ||
		rowSourcePath(row) != ""
}

func rowSourcePath(row map[string]any) string {
	return firstNonEmpty(cleanString(row["source_path"]), cleanString(row["file_path"]), cleanString(row["current_source_path"]))
}

func rowDurationSeconds(row map[string]any) float64 {
	return firstPositive(numberValue(row["duration_seconds"]), numberValue(row["length_seconds"]), numberValue(row["duration"]), numberValue(row["total_duration"]))
}

func normalizePathForIdentity(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return strings.ToLower(filepath.Clean(path))
}

func progressFromRow(row map[string]any, identity Identity) Progress {
	seen := int(numberValue(firstPresent(row, "tile_count_seen", "tile_count_parsed", "tile_count")))
	expected := int(numberValue(firstPresent(row, "tile_count_expected", "tile_count_total")))
	coverage := firstPositive(numberValue(row["coverage_seconds"]), numberValue(row["total_duration"]))
	ratio := numberValue(row["coverage_ratio"])
	if ratio <= 0 {
		ratio = numberValue(rowMap(row["quality_evidence"])["coverage"])
	}
	if ratio <= 0 && expected > 0 && seen > 0 {
		ratio = float64(seen) / float64(expected)
	}
	if ratio <= 0 && coverage > 0 && identity.DurationSec > 0 {
		ratio = coverage / identity.DurationSec
	}
	if ratio > 1 {
		ratio = 1
	}
	return Progress{
		TileCountSeen:     seen,
		TileCountExpected: expected,
		CoverageSeconds:   round3(coverage),
		CoverageRatio:     round4(ratio),
		UpdatedAt:         cleanString(row["updated_at"]),
		Reason:            cleanString(row["reason"]),
	}
}

func rollupLayerStatus(features map[string]FeatureStatus) string {
	if len(features) == 0 {
		return StatusMissing
	}
	ready, partial, building, missing, failed, deferred, stale, suspect := 0, 0, 0, 0, 0, 0, 0, 0
	for _, feature := range features {
		switch feature.Status {
		case StatusReady:
			ready++
		case StatusPartial:
			partial++
		case StatusBuilding:
			building++
		case StatusSuspect:
			suspect++
		case StatusFailed:
			failed++
		case StatusDeferred:
			deferred++
		case StatusStale:
			stale++
		default:
			missing++
		}
	}
	if ready > 0 && ready+deferred == len(features) {
		return StatusReady
	}
	if stale > 0 && ready+partial+building == 0 {
		return StatusStale
	}
	if building > 0 {
		if ready > 0 || partial > 0 {
			return StatusPartial
		}
		return StatusBuilding
	}
	if partial > 0 || ready > 0 {
		return StatusPartial
	}
	if suspect > 0 {
		return StatusSuspect
	}
	if failed > 0 && failed+deferred == len(features) {
		return StatusFailed
	}
	if deferred == len(features) {
		return StatusDeferred
	}
	_ = missing
	return StatusMissing
}

func rollupStatus(layers map[string]LayerStatus) string {
	if len(layers) == 0 {
		return StatusMissing
	}
	ready, partial, building, missing, failed, deferred, stale, suspect := 0, 0, 0, 0, 0, 0, 0, 0
	for _, layer := range layers {
		switch layer.Status {
		case StatusReady:
			ready++
		case StatusPartial:
			partial++
		case StatusBuilding:
			building++
		case StatusSuspect:
			suspect++
		case StatusFailed:
			failed++
		case StatusDeferred:
			deferred++
		case StatusStale:
			stale++
		default:
			missing++
		}
	}
	if ready > 0 && ready+deferred == len(layers) {
		return StatusReady
	}
	if building > 0 {
		if ready > 0 || partial > 0 {
			return StatusPartial
		}
		return StatusBuilding
	}
	if partial > 0 || ready > 0 {
		return StatusPartial
	}
	if suspect > 0 {
		return StatusSuspect
	}
	if stale > 0 {
		return StatusStale
	}
	if failed > 0 {
		return StatusFailed
	}
	_ = missing
	return StatusMissing
}

func featurePriority(status string) int {
	switch status {
	case StatusReady:
		return 7
	case StatusPartial:
		return 6
	case StatusBuilding:
		return 5
	case StatusSuspect:
		return 4
	case StatusStale:
		return 3
	case StatusFailed:
		return 2
	case StatusMissing:
		return 1
	case StatusDeferred:
		return 0
	default:
		return 0
	}
}

func backgroundFillEligible(layerName, featureName, status string) bool {
	switch status {
	case "", StatusMissing, StatusStale:
	default:
		return false
	}
	if layerName == "l1_static" {
		return true
	}
	if layerName == "l3_deep" {
		switch featureName {
		case "spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary":
			return true
		}
	}
	return false
}

func samePackageIdentity(a, b Status) bool {
	if !samePackageTarget(a, b) {
		return false
	}
	if strings.TrimSpace(a.SourceRevision) == "" || strings.TrimSpace(b.SourceRevision) == "" {
		return false
	}
	row := statusIdentityRow(a)
	delete(row, "render_revision")
	identity := identityFromStatus(b)
	identity.RenderRevision = ""
	if rowIdentityMismatchReason(row, identity) != "" {
		return false
	}
	return true
}

func statusIdentityRow(status Status) map[string]any {
	return map[string]any{
		"project_id":         status.ProjectID,
		"track_id":           status.TrackID,
		"clip_id":            status.ClipID,
		"source_hash":        status.SourceHash,
		"source_fingerprint": status.SourceFingerprint,
		"source_revision":    status.SourceRevision,
		"clip_revision":      status.ClipRevision,
		"render_revision":    status.RenderRevision,
		"source_path":        status.SourcePath,
		"duration_seconds":   status.DurationSec,
	}
}

func identityFromStatus(status Status) Identity {
	return Identity{
		ProjectID:         status.ProjectID,
		TrackID:           status.TrackID,
		ClipID:            status.ClipID,
		SourceHash:        status.SourceHash,
		SourceFingerprint: status.SourceFingerprint,
		SourceRevision:    status.SourceRevision,
		ClipRevision:      status.ClipRevision,
		RenderRevision:    status.RenderRevision,
		SourcePath:        status.SourcePath,
		DurationSec:       status.DurationSec,
	}
}

func samePackageTarget(a, b Status) bool {
	return strings.TrimSpace(a.ProjectID) == strings.TrimSpace(b.ProjectID) &&
		strings.TrimSpace(a.TrackID) == strings.TrimSpace(b.TrackID) &&
		strings.TrimSpace(a.ClipID) == strings.TrimSpace(b.ClipID)
}

func fillIdentityFromProjectState(identity *Identity, projectState map[string]any) {
	if identity == nil {
		return
	}
	for _, track := range mapRows(projectState["tracks"]) {
		trackID := firstNonEmpty(cleanString(track["track_id"]), cleanString(track["id"]))
		if identity.TrackID != "" && trackID != "" && identity.TrackID != trackID {
			continue
		}
		for _, clip := range mapRows(firstPresent(track, "clips", "clip_summaries")) {
			clipID := firstNonEmpty(cleanString(clip["clip_id"]), cleanString(clip["id"]), cleanString(clip["item_id"]))
			if identity.ClipID != "" && clipID != "" && identity.ClipID != clipID {
				continue
			}
			if identity.TrackID == "" {
				identity.TrackID = trackID
			}
			if identity.ClipID == "" {
				identity.ClipID = clipID
			}
			if identity.SourcePath == "" {
				identity.SourcePath = firstNonEmpty(cleanString(clip["file_path"]), cleanString(clip["source_path"]), cleanString(clip["current_source_path"]), cleanString(track["file_path"]), cleanString(track["source_path"]))
			}
			if identity.DurationSec <= 0 {
				identity.DurationSec = firstPositive(numberValue(clip["length_seconds"]), numberValue(clip["duration_seconds"]), numberValue(clip["duration"]))
			}
			if identity.ClipStartSec == 0 {
				identity.ClipStartSec = firstNumber(clip, "clip_start_seconds", "start_seconds", "start_time_seconds")
			}
			return
		}
	}
}

func ComputeSourceRevision(identity Identity) string {
	h := sha256.New()
	wrote := false
	writeHash := func(value any) {
		_, _ = fmt.Fprintf(h, "%v|", value)
	}
	if identity.SourceHash != "" {
		wrote = true
		writeHash("hash")
		writeHash(identity.SourceHash)
	}
	if identity.SourcePath != "" {
		wrote = true
		writeHash("path")
		writeHash(strings.ToLower(filepath.Clean(identity.SourcePath)))
		if info, err := os.Stat(identity.SourcePath); err == nil {
			writeHash("stat")
			writeHash(info.Size())
			writeHash(info.ModTime().UTC().UnixNano())
		}
	}
	if identity.DurationSec > 0 {
		wrote = true
		writeHash("duration")
		writeHash(round3(identity.DurationSec))
	}
	if !wrote {
		return ""
	}
	sum := h.Sum(nil)
	return "rev_" + hex.EncodeToString(sum[:8])
}

func computeSourceRevision(identity Identity) string {
	return ComputeSourceRevision(identity)
}

func ComputeClipRevision(identity Identity) string {
	h := sha256.New()
	writeHash := func(value any) {
		_, _ = fmt.Fprintf(h, "%v|", value)
	}
	writeHash(identity.ProjectID)
	writeHash(identity.TrackID)
	writeHash(identity.ClipID)
	writeHash(firstNonEmpty(identity.SourceRevision, identity.SourceFingerprint))
	writeHash(strings.ToLower(filepath.Clean(identity.SourcePath)))
	writeHash(round3(identity.DurationSec))
	writeHash(round3(identity.ClipStartSec))
	sum := h.Sum(nil)
	return "cliprev_" + hex.EncodeToString(sum[:8])
}

func targetTrackID(args map[string]any) string {
	if args == nil {
		return ""
	}
	target := rowMap(args["target_ref"])
	if len(target) == 0 {
		return ""
	}
	if !strings.EqualFold(cleanString(target["kind"]), "track") {
		return ""
	}
	return cleanString(target["id"])
}

func rowMap(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		return nil
	}
	return row
}

func mapRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, item := range rows {
			if row := rowMap(item); len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func compactRef(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	return selectKeys(row,
		"schema_version", "status", "feature_type", "layer", "project_id", "session_id", "track_id", "clip_id", "target", "file_path", "source_path", "source_identity", "source_fingerprint", "source_revision", "source_hash",
		"clip_revision", "render_revision", "plugin_chain_revision", "fader_revision", "track_state_fingerprint", "analyzer_revision", "analyzer_version", "clip_start_seconds",
		"request_id", "source", "source_kind", "reason", "capture_mode", "tap_point", "render_mode", "capture_time", "time_basis", "quality_status", "quality_reason", "quality_reasons", "quality_evidence", "tile_count_seen", "tile_count_expected", "tile_count_parsed",
		"coverage_seconds", "coverage_ratio", "total_duration", "duration_seconds", "sample_rate", "channel_count", "channels", "expected_sample_count", "analyzed_sample_count", "nonzero_count", "sum_abs", "max_abs", "nan_count", "inf_count", "analyzed_range", "evidence_ref", "updated_at", "rms", "peak", "peak_abs", "rms_dbfs",
		"peak_dbfs", "headroom_db", "crest_factor", "crest_db", "integrated_lufs", "approximate_lufs", "approximate", "algorithm", "bands", "balance_db", "balance_state",
		"correlation_estimate", "correlation_state", "parse_failure_count", "parse_failures")
}

func selectKeys(row map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := row[key]; ok && !emptyValue(value) {
			out[key] = value
		}
	}
	return out
}

func capRows(rows []map[string]any, max int) []map[string]any {
	if max <= 0 || len(rows) <= max {
		return rows
	}
	return rows[:max]
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok && !emptyValue(value) {
			return value
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNumber(row map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := row[key]; ok && !emptyValue(value) {
			return numberValue(value)
		}
	}
	return 0
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func cleanString(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func numberValue(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		n, _ := strconvParseFloat(strings.TrimSpace(v))
		return n
	default:
		return 0
	}
}

func strconvParseFloat(text string) (float64, error) {
	var out float64
	_, err := fmt.Sscanf(text, "%f", &out)
	return out, err
}

func emptyValue(value any) bool {
	if value == nil {
		return true
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text == "" || text == "<nil>"
}

func round3(v float64) float64 {
	if v <= 0 {
		return 0
	}
	return math.Round(v*1000) / 1000
}

func round4(v float64) float64 {
	if v <= 0 {
		return 0
	}
	return math.Round(v*10000) / 10000
}
