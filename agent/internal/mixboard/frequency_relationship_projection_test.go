package mixboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/mom"
)

func TestProjectPackageDerivesFrequencyInputsFromExistingL2ProbeWithoutTrackCap(t *testing.T) {
	state := map[string]any{
		"project_uuid": "project-frequency", "project_epoch": "epoch-1", "revision": 23, "snapshot_hash": "hash-23",
		"tracks": []any{},
	}
	snap := featureSnapshot{}
	for i := 1; i <= 6; i++ {
		trackID := "track_" + cleanAnyString(i)
		state["tracks"] = append(anySlice(state["tracks"]), map[string]any{"track_id": trackID, "track_name": "Track " + cleanAnyString(i)})
		snap.L2RenderProbes = append(snap.L2RenderProbes, map[string]any{
			"status": "ready", "track_id": trackID, "request_id": "probe_" + trackID,
			"tap_point": "track_post_fader", "source": "l2_render_probe", "render_revision": "render_" + trackID,
			"bands": map[string]any{"presence": map[string]any{"unit_energy": .2 + float64(i)*.01, "energy_db": -14 + float64(i)}},
		})
	}
	project := buildProjectPackage(state, TargetRef{Kind: "project", ID: "current"}, ListenScope{Source: ListenSourceScope{Mode: "full_project"}}, snap)
	inputs := mapValue(project["frequency_relationship_inputs"])
	if cleanAnyString(inputs["schema_version"]) != "dad.project_frequency_relationship_inputs.v1" || cleanAnyString(inputs["status"]) != "ready" {
		t.Fatalf("frequency inputs = %#v", inputs)
	}
	if int(numberFromMap(inputs, "usable_track_count")) != 6 || len(mapRowsAny(inputs["tracks"])) != 6 || boolFromAny(inputs["decision_tracks_truncated"]) {
		t.Fatalf("frequency input coverage was capped: %#v", inputs)
	}
	if cleanAnyString(project["project_revision"]) != "23" || cleanAnyString(project["project_state_hash"]) != "hash-23" {
		t.Fatalf("project cut facts missing: %#v", project)
	}
	for _, track := range mapRowsAny(project["tracks"]) {
		evidence := mapValue(track["frequency_evidence"])
		if cleanAnyString(evidence["evidence_layer"]) != "l2_realtime.render_probe" || cleanAnyString(evidence["tap_point"]) != "track_post_fader" {
			t.Fatalf("did not reuse existing L2 render probe: %#v", evidence)
		}
	}
}

func TestFrequencyRelationshipReadOnlyStoreSmoke(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "frequency_features.json")
	snapshot := featureSnapshot{BandEnergySummaries: []map[string]any{
		{"status": "ready", "source": "kernel_l3_offline_analyzer", "track_id": "t1", "request_id": "l3_t1", "bands": map[string]any{"bass": map[string]any{"unit_energy": .31}}},
		{"status": "ready", "source": "kernel_l3_offline_analyzer", "track_id": "t2", "request_id": "l3_t2", "bands": map[string]any{"bass": map[string]any{"unit_energy": .29}}},
	}}
	if err := writeJSON(snapshotPath, snapshot); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(root, "mixboard"))
	result, err := store.RequestObservation(Request{
		MixSessionID: "frequency-read-only-smoke", Round: 1, GoalText: "observe project frequency relationships",
		TargetRef:   TargetRef{Kind: "project", ID: "current", Label: "Current project"},
		ListenScope: ListenScope{Time: ListenTimeScope{Mode: "full_song"}, Source: ListenSourceScope{Mode: "full_project"}},
		ProjectState: map[string]any{
			"project_uuid": "p-smoke", "project_epoch": "e-smoke", "revision": 4, "snapshot_hash": "h4", "duration_seconds": 30.0,
			"tracks": []any{map[string]any{"track_id": "t1", "track_name": "One"}, map[string]any{"track_id": "t2", "track_name": "Two"}},
		},
		Args: map[string]any{"mom_intent": mom.IntentProjectFrequencyObservation, "scope": "full_project", "observation_only": true, "feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Observation.MOMProjection == nil || result.Observation.MOMProjection.FrequencyRelationship == nil {
		t.Fatalf("persisted observation missing frequency projection: %#v", result.Observation.MOMProjection)
	}
	relation := result.Observation.MOMProjection.FrequencyRelationship
	if relation.SchemaVersion != mom.FrequencyRelationshipSchema || len(relation.TrackProfiles) != 2 || relation.TapPoint != "source_file_pre_fx" {
		t.Fatalf("read-only smoke projection = %#v", relation)
	}
	if _, err := os.Stat(result.ObservationPath); err != nil {
		t.Fatalf("observation artifact missing: %v", err)
	}
	actionDir := filepath.Join(root, "mixboard", "frequency-read-only-smoke", "actions")
	actions, err := os.ReadDir(actionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Fatalf("read-only smoke created action artifacts: %#v", actions)
	}
}

func TestAssembleFrequencyContextUsesExistingSnapshotWithoutPersistence(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", filepath.Join(root, "missing_acoustic_package.json"))
	snapshotPath := filepath.Join(root, "frequency_features.json")
	snapshot := featureSnapshot{BandEnergySummaries: []map[string]any{
		{"status": "ready", "source": "kernel_l3_offline_analyzer", "track_id": "t1", "request_id": "l3_t1", "bands": map[string]any{"bass": map[string]any{"unit_energy": .31}}},
		{"status": "ready", "source": "kernel_l3_offline_analyzer", "track_id": "t2", "request_id": "l3_t2", "bands": map[string]any{"bass": map[string]any{"unit_energy": .29}}},
	}}
	if err := writeJSON(snapshotPath, snapshot); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"project_uuid": "p-fast", "project_epoch": "e-fast", "revision": 7, "snapshot_hash": "h7", "duration_seconds": 30.0,
		"tracks": []any{map[string]any{"track_id": "t1", "track_name": "One"}, map[string]any{"track_id": "t2", "track_name": "Two"}},
	}
	context := AssembleFrequencyContext(state, "c1-fast", "run C1", snapshotPath)
	frequency := mapValue(context.MOMProjection["frequency_relationship"])
	if context.ObservationID == "" || cleanAnyString(frequency["schema_version"]) != mom.FrequencyRelationshipSchema {
		t.Fatalf("compact C1 context = %#v", context)
	}
	if len(mapRowsAny(frequency["track_profiles"])) != 2 || cleanAnyString(frequency["tap_point"]) != "source_file_pre_fx" {
		t.Fatalf("frequency relationship = %#v", frequency)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(snapshotPath) {
		t.Fatalf("CCB assembly persisted unexpected artifacts: %#v", entries)
	}
}

func TestAssembleFrequencyContextReusesExactSourceL3AcrossProjectWithoutObservation(t *testing.T) {
	root := t.TempDir()
	featurePath := filepath.Join(root, "frequency_features.json")
	packagePath := filepath.Join(root, "acoustic_package_status.json")
	if err := writeJSON(featurePath, featureSnapshot{}); err != nil {
		t.Fatal(err)
	}
	const sourceRevision = "D:/audio/one.wav|size=100|mtime=1|length=8.0000"
	readyFeatures := map[string]acousticpackage.FeatureStatus{
		"band_energy_summary": {
			Status: acousticpackage.StatusReady, Source: "kernel_l3_offline_analyzer",
			Ref: map[string]any{"status": "ready", "source": "kernel_l3_offline_analyzer", "source_revision": sourceRevision, "bands": map[string]any{"bass": map[string]any{"unit_energy": .31}}},
		},
		"stereo_relation_summary": {
			Status: acousticpackage.StatusReady, Source: "kernel_l3_offline_analyzer",
			Ref: map[string]any{"status": "ready", "source": "kernel_l3_offline_analyzer", "source_revision": sourceRevision, "correlation_estimate": .8},
		},
	}
	packages := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion, Packages: []acousticpackage.Status{
		{SchemaVersion: acousticpackage.SchemaVersion, ProjectID: "project-current", TrackID: "t1", ClipID: "c1", SourceRevision: sourceRevision, SourceFingerprint: sourceRevision, SourcePath: "D:/audio/one.wav", UpdatedAt: "2026-07-30T01:00:00Z", PackageLayers: map[string]acousticpackage.LayerStatus{"l3_deep": {Status: acousticpackage.StatusStale, Features: map[string]acousticpackage.FeatureStatus{"band_energy_summary": {Status: acousticpackage.StatusStale, Reason: "project_mismatch"}}}}},
		{SchemaVersion: acousticpackage.SchemaVersion, ProjectID: "older-project", TrackID: "old-t", ClipID: "old-c", SourceRevision: sourceRevision, SourceFingerprint: sourceRevision, SourcePath: "D:/audio/one.wav", UpdatedAt: "2026-07-29T01:00:00Z", PackageLayers: map[string]acousticpackage.LayerStatus{"l3_deep": {Status: acousticpackage.StatusReady, Features: readyFeatures}}},
	}}
	if err := writeJSON(packagePath, packages); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", packagePath)
	state := map[string]any{
		"project_uuid": "project-current", "tracks": []any{map[string]any{
			"track_id": "t1", "track_name": "One", "clips": []any{map[string]any{"clip_id": "c1", "source_path": "D:/audio/one.wav", "source_revision": sourceRevision, "duration_seconds": 8.0}},
		}},
	}
	context := AssembleFrequencyContext(state, "c1-package", "run C1", featurePath)
	frequency := mapValue(context.MOMProjection["frequency_relationship"])
	profiles := mapRowsAny(frequency["track_profiles"])
	if len(profiles) != 1 || cleanAnyString(profiles[0]["status"]) != "ready" || cleanAnyString(profiles[0]["tap_point"]) != "source_file_pre_fx" {
		t.Fatalf("exact source L3 was not assembled: context=%#v profiles=%#v", context.Assembly, profiles)
	}
	if int(numberFromMap(context.Assembly, "matched_track_count")) != 1 || int(numberFromMap(context.Assembly, "mix_observe_calls")) != 0 || int(numberFromMap(context.Assembly, "persistence_writes")) != 0 {
		t.Fatalf("assembly contract = %#v", context.Assembly)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("CCB assembly persisted unexpected artifacts: %#v", entries)
	}
}

func TestAssembleFrequencyContextUsesPortableLegacyAcousticFallback(t *testing.T) {
	root := t.TempDir()
	featurePath := filepath.Join(root, "frequency_features.json")
	primaryPath := filepath.Join(root, "project", "acoustic_package_status.json")
	fallbackPath := filepath.Join(root, "workspace", "acoustic_package_status.json")
	if err := writeJSON(featurePath, featureSnapshot{L2RenderProbes: []map[string]any{{
		"status": "ready", "track_id": "t1", "clip_id": "c1", "tap_point": "track_post_fader",
		"source_path": "D:/audio/portable.wav", "bands": map[string]any{"bass": map[string]any{"unit_energy": .9}},
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(primaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(fallbackPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(primaryPath, acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion}); err != nil {
		t.Fatal(err)
	}
	const sourceRevision = "D:/audio/portable.wav|size=100|mtime=1|length=8.0000"
	fallback := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion, Packages: []acousticpackage.Status{{
		SchemaVersion: acousticpackage.SchemaVersion, ProjectID: "older-project", TrackID: "old-track", ClipID: "old-clip",
		SourceRevision: sourceRevision, SourceFingerprint: sourceRevision, SourcePath: "D:/audio/portable.wav", UpdatedAt: "2026-07-30T01:00:00Z",
		PackageLayers: map[string]acousticpackage.LayerStatus{"l3_deep": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{
			"band_energy_summary": {Status: acousticpackage.StatusReady, Source: "kernel_l3_offline_analyzer", Ref: map[string]any{
				"status": "ready", "source": "kernel_l3_offline_analyzer", "source_revision": sourceRevision,
				"bands": map[string]any{"bass": map[string]any{"unit_energy": .42}},
			}},
		}}},
	}}}
	if err := writeJSON(fallbackPath, fallback); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{"project_uuid": "current-project", "tracks": []any{map[string]any{
		"track_id": "t1", "clips": []any{map[string]any{
			"clip_id": "c1", "source_path": "D:/audio/portable.wav", "source_revision": sourceRevision, "duration_seconds": 8.0,
		}},
	}}}
	assembled := loadFeatureSnapshot(map[string]any{"feature_snapshot_path": featurePath})
	assembly := hydrateFrequencySnapshotFromAcousticPackages(&assembled, state, map[string]any{
		"acoustic_package_status_path":    primaryPath,
		"acoustic_package_fallback_paths": []any{fallbackPath},
	})
	if int(numberFromMap(assembly, "matched_l3_track_count")) != 1 || len(assembled.BandEnergySummaries) != 1 {
		t.Fatalf("portable fallback was not hydrated: assembly=%#v rows=%#v", assembly, assembled.BandEnergySummaries)
	}
	if cleanAnyString(assembled.BandEnergySummaries[0]["project_id"]) != "current-project" || cleanAnyString(assembled.BandEnergySummaries[0]["track_id"]) != "t1" {
		t.Fatalf("portable fallback was not rebound to the current project target: %#v", assembled.BandEnergySummaries[0])
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", primaryPath)
	t.Setenv("VIT_ACOUSTIC_PACKAGE_FALLBACK_PATH", fallbackPath)
	context := AssembleFrequencyContext(state, "c1-portable-fallback", "run C1", featurePath)
	frequency := mapValue(context.MOMProjection["frequency_relationship"])
	if cleanAnyString(frequency["tap_point"]) != "source_file_pre_fx" {
		t.Fatalf("C1 frequency diagnosis mixed L2 and portable L3 taps: assembly=%#v frequency=%#v", context.Assembly, frequency)
	}
}

func TestAssembleFrequencyContextMatchesDescriptorL3AcrossProjectWhenStateHasOnlySourcePath(t *testing.T) {
	root := t.TempDir()
	featurePath := filepath.Join(root, "frequency_features.json")
	packagePath := filepath.Join(root, "acoustic_package_status.json")
	sourcePath := filepath.Join(root, "one.wav")
	if err := os.WriteFile(sourcePath, []byte("current exact source"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := fmt.Sprintf("%s|size=%d|mtime=%d|length=8.0000", sourcePath, info.Size(), info.ModTime().UTC().UnixMilli())
	packages := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion, Packages: []acousticpackage.Status{{
		SchemaVersion: acousticpackage.SchemaVersion, ProjectID: "older-project", TrackID: "old-track", ClipID: "old-clip",
		SourcePath: sourcePath, SourceRevision: descriptor, SourceFingerprint: descriptor, UpdatedAt: "2026-07-30T01:00:00Z",
		PackageLayers: map[string]acousticpackage.LayerStatus{"l3_deep": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{
			"band_energy_summary": {Status: acousticpackage.StatusReady, Source: "kernel_l3_offline_analyzer", Ref: map[string]any{"status": "ready", "source_path": sourcePath, "source_revision": descriptor, "bands": map[string]any{"bass": map[string]any{"unit_energy": .31}}}},
		}}},
	}}}
	if err := writeJSON(featurePath, featureSnapshot{}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(packagePath, packages); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", packagePath)
	state := map[string]any{"project_uuid": "project-current", "tracks": []any{map[string]any{
		"track_id": "t1", "track_name": "One", "clips": []any{map[string]any{"clip_id": "c1", "current_source_path": sourcePath}},
	}}}
	context := AssembleFrequencyContext(state, "c1-path-only-l3", "run C1", featurePath)
	frequency := mapValue(context.MOMProjection["frequency_relationship"])
	profiles := mapRowsAny(frequency["track_profiles"])
	if len(profiles) != 1 || cleanAnyString(profiles[0]["status"]) != "ready" || cleanAnyString(profiles[0]["tap_point"]) != "source_file_pre_fx" {
		t.Fatalf("path/file descriptor L3 was not assembled: assembly=%#v profiles=%#v", context.Assembly, profiles)
	}
	if int(numberFromMap(context.Assembly, "matched_l3_track_count")) != 1 || int(numberFromMap(context.Assembly, "mix_observe_calls")) != 0 {
		t.Fatalf("L3 path fallback assembly contract = %#v", context.Assembly)
	}
}

func TestFrequencyPackageAssemblyRejectsDescriptorL3WhenSourceFileChanged(t *testing.T) {
	root := t.TempDir()
	packagePath := filepath.Join(root, "acoustic_package_status.json")
	sourcePath := filepath.Join(root, "one.wav")
	if err := os.WriteFile(sourcePath, []byte("original source"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := fmt.Sprintf("%s|size=%d|mtime=%d|length=8.0000", sourcePath, info.Size(), info.ModTime().UTC().UnixMilli())
	packages := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion, Packages: []acousticpackage.Status{{
		ProjectID: "older-project", TrackID: "old-track", ClipID: "old-clip", SourcePath: sourcePath,
		SourceRevision: descriptor, SourceFingerprint: descriptor, UpdatedAt: "2026-07-30T01:00:00Z",
		PackageLayers: map[string]acousticpackage.LayerStatus{"l3_deep": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{
			"band_energy_summary": {Status: acousticpackage.StatusReady, Source: "kernel_l3_offline_analyzer", Ref: map[string]any{"status": "ready", "source_path": sourcePath, "source_revision": descriptor, "bands": map[string]any{"bass": map[string]any{"unit_energy": .9}}}},
		}}},
	}}}
	if err := writeJSON(packagePath, packages); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("changed source with a different size"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", packagePath)
	snap := featureSnapshot{}
	state := map[string]any{"project_uuid": "project-current", "tracks": []any{map[string]any{
		"track_id": "t1", "clips": []any{map[string]any{"clip_id": "c1", "current_source_path": sourcePath}},
	}}}
	assembly := hydrateFrequencySnapshotFromAcousticPackages(&snap, state, nil)
	if len(snap.BandEnergySummaries) != 0 || int(numberFromMap(assembly, "matched_l3_track_count")) != 0 {
		t.Fatalf("changed source file reused stale L3: assembly=%#v rows=%#v", assembly, snap.BandEnergySummaries)
	}
}

func TestFrequencyPackageAssemblyRejectsDifferentSourceRevision(t *testing.T) {
	root := t.TempDir()
	packagePath := filepath.Join(root, "acoustic_package_status.json")
	packages := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion, Packages: []acousticpackage.Status{{
		ProjectID: "older-project", TrackID: "old-t", ClipID: "old-c", SourceRevision: "old-revision", SourceFingerprint: "old-revision", UpdatedAt: "2026-07-29T01:00:00Z",
		PackageLayers: map[string]acousticpackage.LayerStatus{"l3_deep": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{"band_energy_summary": {Status: acousticpackage.StatusReady, Source: "kernel_l3_offline_analyzer", Ref: map[string]any{"status": "ready", "source_revision": "old-revision", "bands": map[string]any{"bass": map[string]any{"unit_energy": .9}}}}}}},
	}}}
	if err := writeJSON(packagePath, packages); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", packagePath)
	snap := featureSnapshot{}
	state := map[string]any{"project_uuid": "new-project", "tracks": []any{map[string]any{"track_id": "t1", "clips": []any{map[string]any{"clip_id": "c1", "source_revision": "new-revision"}}}}}
	assembly := hydrateFrequencySnapshotFromAcousticPackages(&snap, state, nil)
	if len(snap.BandEnergySummaries) != 0 || int(numberFromMap(assembly, "matched_track_count")) != 0 {
		t.Fatalf("different source revision was reused: assembly=%#v rows=%#v", assembly, snap.BandEnergySummaries)
	}
}

func TestAssembleFrequencyContextUsesExactCurrentAcousticL2ForDiagnosis(t *testing.T) {
	root := t.TempDir()
	featurePath := filepath.Join(root, "frequency_features.json")
	packagePath := filepath.Join(root, "acoustic_package_status.json")
	if err := writeJSON(featurePath, featureSnapshot{}); err != nil {
		t.Fatal(err)
	}
	const projectID = "project-current"
	packages := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion}
	stateTracks := []any{}
	for index, trackID := range []string{"t1", "t2"} {
		clipID := "c" + cleanAnyString(index+1)
		sourceRevision := "source-" + cleanAnyString(index+1)
		stateTracks = append(stateTracks, map[string]any{"track_id": trackID, "track_name": trackID, "clips": []any{map[string]any{"clip_id": clipID, "source_revision": sourceRevision, "clip_revision": "clip-" + trackID}}})
		packages.Packages = append(packages.Packages, acousticpackage.Status{
			SchemaVersion: acousticpackage.SchemaVersion, ProjectID: projectID, TrackID: trackID, ClipID: clipID,
			SourceRevision: sourceRevision, SourceFingerprint: sourceRevision, ClipRevision: "clip-" + trackID, RenderRevision: "render-" + trackID, UpdatedAt: "2026-07-30T01:00:00Z",
			PackageLayers: map[string]acousticpackage.LayerStatus{"l2_realtime": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{"render_probe": {
				Status: acousticpackage.StatusReady, Source: "l2_render_probe", Ref: map[string]any{"status": "ready", "tap_point": "track_post_fader", "source_revision": sourceRevision, "bands": map[string]any{"bass": map[string]any{"unit_energy": .31 + float64(index)*.01}}, "evidence_ref": "dad.l2:" + trackID},
			}}}},
		})
	}
	packages.Packages = append(packages.Packages, acousticpackage.Status{
		SchemaVersion: acousticpackage.SchemaVersion, ProjectID: projectID, TrackID: "t1", ClipID: "c1", SourceRevision: "stale-source", SourceFingerprint: "stale-source", UpdatedAt: "2026-07-30T02:00:00Z",
		PackageLayers: map[string]acousticpackage.LayerStatus{"l2_realtime": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{"render_probe": {Status: acousticpackage.StatusReady, Ref: map[string]any{"status": "ready", "tap_point": "track_post_fader", "bands": map[string]any{"bass": map[string]any{"unit_energy": .99}}}}}}},
	})
	if err := writeJSON(packagePath, packages); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", packagePath)
	context := AssembleFrequencyContext(map[string]any{"project_uuid": projectID, "tracks": stateTracks}, "c1-package-l2", "run C1", featurePath)
	frequency := mapValue(context.MOMProjection["frequency_relationship"])
	profiles := mapRowsAny(frequency["track_profiles"])
	if len(profiles) != 2 || cleanAnyString(frequency["tap_point"]) != "track_post_fader" || cleanAnyString(profiles[0]["status"]) != "ready" || cleanAnyString(profiles[1]["status"]) != "ready" {
		t.Fatalf("exact package L2 was not assembled: assembly=%#v frequency=%#v", context.Assembly, frequency)
	}
	if int(numberFromMap(context.Assembly, "matched_l2_track_count")) != 2 || int(numberFromMap(context.Assembly, "mix_observe_calls")) != 0 || int(numberFromMap(context.Assembly, "persistence_writes")) != 0 {
		t.Fatalf("L2 diagnosis assembly contract = %#v", context.Assembly)
	}
}

func TestAssembleFrequencyContextPreservesReadyL2AndExcludesFolderContainers(t *testing.T) {
	root := t.TempDir()
	featurePath := filepath.Join(root, "frequency_features.json")
	packagePath := filepath.Join(root, "acoustic_package_status.json")
	if err := writeJSON(featurePath, featureSnapshot{}); err != nil {
		t.Fatal(err)
	}
	const projectID = "project-current"
	stateTracks := []any{
		map[string]any{"track_id": "folder", "track_name": "Drums", "track_type": "folder", "is_folder_track": true, "is_folder_container": true, "has_child_tracks": true, "child_track_count": 3, "clips": []any{}},
		map[string]any{"track_id": "internal", "track_name": "Internal", "track_type": "track", "is_audio": false, "is_audio_track": false, "clips": []any{}},
		map[string]any{"track_id": "t1", "track_name": "Kick", "is_audio": true, "is_audio_track": true, "clips": []any{map[string]any{"clip_id": "c1", "source_revision": "source-1"}}},
		map[string]any{"track_id": "t2", "track_name": "Bass", "is_audio": true, "is_audio_track": true, "clips": []any{map[string]any{"clip_id": "c2", "source_revision": "source-2"}}},
		map[string]any{"track_id": "t3", "track_name": "Perc", "is_audio": true, "is_audio_track": true, "clips": []any{map[string]any{"clip_id": "c3", "source_revision": "source-3"}}},
	}
	packages := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion}
	for index, trackID := range []string{"t1", "t2"} {
		clipID := "c" + cleanAnyString(index+1)
		sourceRevision := "source-" + cleanAnyString(index+1)
		packages.Packages = append(packages.Packages, acousticpackage.Status{
			SchemaVersion: acousticpackage.SchemaVersion, ProjectID: projectID, TrackID: trackID, ClipID: clipID,
			SourceRevision: sourceRevision, SourceFingerprint: sourceRevision, UpdatedAt: "2026-07-30T01:00:00Z",
			PackageLayers: map[string]acousticpackage.LayerStatus{"l2_realtime": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{"render_probe": {
				Status: acousticpackage.StatusReady, Source: "l2_render_probe", Ref: map[string]any{
					"status": "ready", "tap_point": "track_post_fader", "source_revision": sourceRevision,
					"bands": map[string]any{"bass": map[string]any{"unit_energy": .31 + float64(index)*.01}}, "evidence_ref": "dad.l2:" + trackID,
				},
			}}}},
		})
	}
	packages.Packages = append(packages.Packages, acousticpackage.Status{
		SchemaVersion: acousticpackage.SchemaVersion, ProjectID: projectID, TrackID: "t3", ClipID: "c3",
		SourceRevision: "source-3", SourceFingerprint: "source-3", UpdatedAt: "2026-07-30T01:00:00Z",
		PackageLayers: map[string]acousticpackage.LayerStatus{
			"l1_static": {Status: acousticpackage.StatusPartial, Features: map[string]acousticpackage.FeatureStatus{"waveform_envelope": {
				Status: acousticpackage.StatusPartial, Ref: map[string]any{"status": "partial", "quality_status": "suspect", "quality_reason": "input_all_zero"},
			}}},
			"l2_realtime": {Status: acousticpackage.StatusSuspect, Features: map[string]acousticpackage.FeatureStatus{"render_probe": {
				Status: acousticpackage.StatusSuspect, Reason: "all_zero_render", Ref: map[string]any{"status": "suspect", "reason": "all_zero_render", "tap_point": "track_post_fader", "source_revision": "source-3", "bands": map[string]any{"bass": map[string]any{"unit_energy": 0.0}}},
			}}},
		},
	})
	if !acousticPackageConfirmsDeterministicSilence(packages.Packages[len(packages.Packages)-1], packages.Packages[len(packages.Packages)-1].PackageLayers["l2_realtime"].Features["render_probe"]) {
		t.Fatal("all-zero source plus all-zero post-fader render was not recognized as deterministic silence")
	}
	if err := writeJSON(packagePath, packages); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", packagePath)
	context := AssembleFrequencyContext(map[string]any{"project_uuid": projectID, "tracks": stateTracks}, "c1-folder-roster", "run C1", featurePath)
	frequency := mapValue(context.MOMProjection["frequency_relationship"])
	coverage := mapValue(frequency["coverage"])
	profiles := mapRowsAny(frequency["track_profiles"])
	if len(profiles) != 3 || int(numberFromMap(coverage, "project_track_count")) != 3 || int(numberFromMap(coverage, "eligible_track_count")) != 3 || int(numberFromMap(coverage, "missing_track_count")) != 0 {
		t.Fatalf("frequency roster/coverage = %#v profiles=%#v assembly=%#v", coverage, profiles, context.Assembly)
	}
	if cleanAnyString(frequency["tap_point"]) != "track_post_fader" {
		t.Fatalf("ready L2 tap was lost: frequency=%#v assembly=%#v", frequency, context.Assembly)
	}
	for _, profile := range profiles {
		if cleanAnyString(profile["track_id"]) == "folder" || cleanAnyString(profile["track_id"]) == "internal" {
			t.Fatalf("non-audio container/internal track entered C1 frequency roster: %#v", profiles)
		}
		if cleanAnyString(profile["track_id"]) == "t3" && (!boolFromAny(profile["silence_confirmed"]) || cleanAnyString(profile["status"]) != "partial") {
			t.Fatalf("deterministic silence was not projected as partial no-change evidence: %#v", profile)
		}
	}
	if int(numberFromMap(context.Assembly, "matched_l2_track_count")) != 3 || int(numberFromMap(context.Assembly, "normalized_l2_row_count")) != 3 || int(numberFromMap(context.Assembly, "projected_eligible_track_count")) != 3 {
		t.Fatalf("ready L2 evidence was lost between package and MOM: %#v", context.Assembly)
	}
	if cleanAnyString(context.Assembly["status"]) == "projection_mismatch" {
		t.Fatalf("unexpected package/MOM projection mismatch: %#v", context.Assembly)
	}
}

func TestAssembleFrequencyContextMatchesDescriptorL2WhenStateHasOnlySourcePath(t *testing.T) {
	root := t.TempDir()
	featurePath := filepath.Join(root, "frequency_features.json")
	packagePath := filepath.Join(root, "acoustic_package_status.json")
	sourcePath := filepath.Join(root, "one.wav")
	if err := os.WriteFile(sourcePath, []byte("current exact source"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := fmt.Sprintf("%s|size=%d|mtime=%d|length=8.0000", sourcePath, info.Size(), info.ModTime().UTC().UnixMilli())
	packages := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion, Packages: []acousticpackage.Status{{
		SchemaVersion: acousticpackage.SchemaVersion, ProjectID: "project-current", TrackID: "t1", ClipID: "c1", SourcePath: sourcePath, SourceRevision: descriptor, SourceFingerprint: descriptor, UpdatedAt: "2026-07-30T01:00:00Z",
		PackageLayers: map[string]acousticpackage.LayerStatus{"l2_realtime": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{"render_probe": {Status: acousticpackage.StatusReady, Source: "l2_render_probe", Ref: map[string]any{"status": "ready", "tap_point": "track_post_fader", "source_path": sourcePath, "source_revision": descriptor, "bands": map[string]any{"bass": map[string]any{"unit_energy": .31}}}}}}},
	}}}
	if err := writeJSON(featurePath, featureSnapshot{}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(packagePath, packages); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", packagePath)
	state := map[string]any{"project_uuid": "project-current", "tracks": []any{map[string]any{"track_id": "t1", "clips": []any{map[string]any{"clip_id": "c1", "current_source_path": sourcePath}}}}}
	context := AssembleFrequencyContext(state, "c1-path-only", "run C1", featurePath)
	frequency := mapValue(context.MOMProjection["frequency_relationship"])
	profiles := mapRowsAny(frequency["track_profiles"])
	if len(profiles) != 1 || cleanAnyString(profiles[0]["status"]) != "ready" || cleanAnyString(profiles[0]["tap_point"]) != "track_post_fader" {
		t.Fatalf("path/file descriptor identity was not assembled: assembly=%#v profiles=%#v", context.Assembly, profiles)
	}
}

func TestAssembleFrequencyContextReusesPriorProjectL2AfterSaveAsWithExactStateRevisions(t *testing.T) {
	root := t.TempDir()
	featurePath := filepath.Join(root, "frequency_features.json")
	packagePath := filepath.Join(root, "acoustic_package_status.json")
	sourcePath := filepath.Join(root, "one.wav")
	if err := os.WriteFile(sourcePath, []byte("current exact source"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := fmt.Sprintf("%s|size=%d|mtime=%d|length=8.0000", sourcePath, info.Size(), info.ModTime().UTC().UnixMilli())
	renderRevision := "l2rp.v1|tap=track_post_fader|mode=offline_probe|track=t1|clip=c1|source=" + descriptor + "|track_state=track-state-1|clip_state=clip-state-1"
	packages := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion, Packages: []acousticpackage.Status{{
		SchemaVersion: acousticpackage.SchemaVersion, ProjectID: "project-before-save-as", TrackID: "t1", ClipID: "c1",
		SourcePath: sourcePath, SourceRevision: descriptor, SourceFingerprint: descriptor, RenderRevision: renderRevision, UpdatedAt: "2026-07-30T01:00:00Z",
		PackageLayers: map[string]acousticpackage.LayerStatus{"l2_realtime": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{"render_probe": {
			Status: acousticpackage.StatusReady, Source: "l2_render_probe", Ref: map[string]any{
				"status": "ready", "project_id": "project-before-save-as", "track_id": "t1", "clip_id": "c1", "tap_point": "track_post_fader",
				"source_path": sourcePath, "source_revision": descriptor, "render_revision": renderRevision,
				"bands": map[string]any{"bass": map[string]any{"unit_energy": .31}}, "evidence_ref": "dad.l2:save-as-exact",
			},
		}}}},
	}}}
	if err := writeJSON(featurePath, featureSnapshot{}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(packagePath, packages); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", packagePath)
	state := map[string]any{"project_uuid": "project-after-save-as", "tracks": []any{map[string]any{
		"track_id": "t1", "track_name": "One", "track_state_revision": "track-state-1",
		"clips": []any{map[string]any{"clip_id": "c1", "current_source_path": sourcePath, "clip_state_revision": "clip-state-1"}},
	}}}
	context := AssembleFrequencyContext(state, "c1-save-as-l2", "run C1", featurePath)
	frequency := mapValue(context.MOMProjection["frequency_relationship"])
	profiles := mapRowsAny(frequency["track_profiles"])
	if len(profiles) != 1 || cleanAnyString(profiles[0]["status"]) != "ready" || cleanAnyString(profiles[0]["tap_point"]) != "track_post_fader" {
		t.Fatalf("exact Save As L2 was not assembled: assembly=%#v profiles=%#v", context.Assembly, profiles)
	}
	if int(numberFromMap(context.Assembly, "matched_l2_track_count")) != 1 {
		t.Fatalf("Save As L2 assembly contract = %#v", context.Assembly)
	}
}

func TestFrequencyPackageAssemblyRejectsPriorProjectL2WhenStateRevisionChanged(t *testing.T) {
	root := t.TempDir()
	packagePath := filepath.Join(root, "acoustic_package_status.json")
	sourcePath := filepath.Join(root, "one.wav")
	if err := os.WriteFile(sourcePath, []byte("current exact source"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := fmt.Sprintf("%s|size=%d|mtime=%d|length=8.0000", sourcePath, info.Size(), info.ModTime().UTC().UnixMilli())
	renderRevision := "l2rp.v1|tap=track_post_fader|track=t1|clip=c1|source=" + descriptor + "|track_state=old-track-state|clip_state=clip-state-1"
	packages := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion, Packages: []acousticpackage.Status{{
		ProjectID: "project-before-save-as", TrackID: "t1", ClipID: "c1", SourcePath: sourcePath, SourceRevision: descriptor, RenderRevision: renderRevision, UpdatedAt: "2026-07-30T01:00:00Z",
		PackageLayers: map[string]acousticpackage.LayerStatus{"l2_realtime": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{"render_probe": {
			Status: acousticpackage.StatusReady, Ref: map[string]any{"status": "ready", "project_id": "project-before-save-as", "track_id": "t1", "clip_id": "c1", "tap_point": "track_post_fader", "source_revision": descriptor, "render_revision": renderRevision, "bands": map[string]any{"bass": map[string]any{"unit_energy": .9}}},
		}}}},
	}}}
	if err := writeJSON(packagePath, packages); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", packagePath)
	snap := featureSnapshot{}
	state := map[string]any{"project_uuid": "project-after-save-as", "tracks": []any{map[string]any{
		"track_id": "t1", "track_state_revision": "changed-track-state",
		"clips": []any{map[string]any{"clip_id": "c1", "current_source_path": sourcePath, "clip_state_revision": "clip-state-1"}},
	}}}
	assembly := hydrateFrequencySnapshotFromAcousticPackages(&snap, state, nil)
	if len(snap.L2RenderProbes) != 0 || int(numberFromMap(assembly, "matched_l2_track_count")) != 0 {
		t.Fatalf("state-mismatched prior-project L2 was reused: assembly=%#v rows=%#v", assembly, snap.L2RenderProbes)
	}
}

func TestFrequencyPackageAssemblyRejectsMismatchedCurrentL2Identity(t *testing.T) {
	root := t.TempDir()
	packagePath := filepath.Join(root, "acoustic_package_status.json")
	packages := acousticpackage.Snapshot{SchemaVersion: acousticpackage.SchemaVersion, Packages: []acousticpackage.Status{{
		ProjectID: "project-current", TrackID: "t1", ClipID: "c1", SourceRevision: "old-source", SourceFingerprint: "old-source", UpdatedAt: "2026-07-30T01:00:00Z",
		PackageLayers: map[string]acousticpackage.LayerStatus{"l2_realtime": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{"render_probe": {Status: acousticpackage.StatusReady, Ref: map[string]any{"status": "ready", "tap_point": "track_post_fader", "bands": map[string]any{"bass": map[string]any{"unit_energy": .9}}}}}}},
	}}}
	if err := writeJSON(packagePath, packages); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", packagePath)
	snap := featureSnapshot{}
	state := map[string]any{"project_uuid": "project-current", "tracks": []any{map[string]any{"track_id": "t1", "clips": []any{map[string]any{"clip_id": "c1", "source_revision": "new-source"}}}}}
	assembly := hydrateFrequencySnapshotFromAcousticPackages(&snap, state, nil)
	if len(snap.L2RenderProbes) != 0 || int(numberFromMap(assembly, "matched_l2_track_count")) != 0 {
		t.Fatalf("mismatched current L2 was reused: assembly=%#v rows=%#v", assembly, snap.L2RenderProbes)
	}
}

func TestAssembleFrequencyContextRecoversExactL3FromPriorC1Observations(t *testing.T) {
	root := t.TempDir()
	mixboardRoot := filepath.Join(root, "mixboard")
	sessionDir := filepath.Join(mixboardRoot, "cap_v1_c1_webui_1", "observations")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	featurePath := filepath.Join(root, "frequency_features.json")
	if err := writeJSON(featurePath, featureSnapshot{}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_MIXBOARD_ROOT", mixboardRoot)
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", filepath.Join(root, "missing_acoustic_package.json"))
	for index, row := range []map[string]any{
		{"status": "ready", "feature_type": "band_energy_summary", "source": "kernel_l3_offline_analyzer", "project_id": "current", "track_id": "t1", "clip_id": "c1", "source_path": "D:/audio/one.wav", "source_revision": "source-1", "bands": map[string]any{"bass": map[string]any{"unit_energy": .31}}},
		{"status": "ready", "feature_type": "band_energy_summary", "source": "kernel_l3_offline_analyzer", "project_id": "current", "track_id": "t2", "clip_id": "c2", "source_path": "D:/audio/two.wav", "source_revision": "source-2", "bands": map[string]any{"bass": map[string]any{"unit_energy": .29}}},
	} {
		observation := ObservationPacket{SchemaVersion: ObservationSchemaVersion, ObservationID: "old-" + cleanAnyString(index), GlobalSummary: map[string]any{"feature_snapshot": map[string]any{"band_energy_summaries": []any{row}}}}
		if err := writeJSON(filepath.Join(sessionDir, observation.ObservationID+".json"), observation); err != nil {
			t.Fatal(err)
		}
	}
	state := map[string]any{"project_uuid": "project-current", "tracks": []any{
		map[string]any{"track_id": "t1", "track_name": "One", "clips": []any{map[string]any{"clip_id": "c1", "source_path": "D:/audio/one.wav", "source_revision": "source-1"}}},
		map[string]any{"track_id": "t2", "track_name": "Two", "clips": []any{map[string]any{"clip_id": "c2", "source_path": "D:/audio/two.wav", "source_revision": "source-2"}}},
	}}
	context := AssembleFrequencyContext(state, "cap_v1_c1_webui_2", "run C1", featurePath)
	frequency := mapValue(context.MOMProjection["frequency_relationship"])
	if profiles := mapRowsAny(frequency["track_profiles"]); len(profiles) != 2 {
		t.Fatalf("prior C1 L3 evidence was not recovered: assembly=%#v profiles=%#v", context.Assembly, profiles)
	}
	if int(numberFromMap(context.Assembly, "prior_observation_matched_track_count")) != 2 || int(numberFromMap(context.Assembly, "prior_observation_file_count")) != 2 {
		t.Fatalf("prior observation recovery contract = %#v", context.Assembly)
	}
	if matches, _ := filepath.Glob(filepath.Join(mixboardRoot, "cap_v1_c1_webui_2", "observations", "*.json")); len(matches) != 0 {
		t.Fatalf("recovery wrote a new observation: %#v", matches)
	}
}

func TestFrequencyInputFallbackLabelsL3AsSourceFilePreFX(t *testing.T) {
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "t1", "track_name": "One"}, map[string]any{"track_id": "t2", "track_name": "Two"}}}
	snap := featureSnapshot{BandEnergySummaries: []map[string]any{
		{"status": "ready", "source": "kernel_l3_offline_analyzer", "track_id": "t1", "bands": map[string]any{"bass": map[string]any{"unit_energy": .3}}},
		{"status": "ready", "source": "kernel_l3_offline_analyzer", "track_id": "t2", "bands": map[string]any{"bass": map[string]any{"unit_energy": .2}}},
	}}
	project := buildProjectPackage(state, TargetRef{Kind: "project", ID: "current"}, ListenScope{Source: ListenSourceScope{Mode: "full_project"}}, snap)
	for _, track := range mapRowsAny(project["tracks"]) {
		if tap := cleanAnyString(mapValue(track["frequency_evidence"])["tap_point"]); tap != "source_file_pre_fx" {
			t.Fatalf("L3 fallback tap = %q track=%#v", tap, track)
		}
	}
}

func TestFullCoverageL3SilenceIsPartialNoChangeEvidence(t *testing.T) {
	state := map[string]any{"project_uuid": "project-silence", "tracks": []any{
		map[string]any{"track_id": "t1", "track_name": "Music", "is_audio": true, "is_audio_track": true, "clips": []any{map[string]any{"clip_id": "c1"}}},
		map[string]any{"track_id": "t2", "track_name": "Silent Stem", "is_audio": true, "is_audio_track": true, "clips": []any{map[string]any{"clip_id": "c2"}}},
	}}
	snap := featureSnapshot{BandEnergySummaries: []map[string]any{
		{"status": "ready", "source": "kernel_l3_offline_analyzer", "track_id": "t1", "clip_id": "c1", "coverage_ratio": 1.0, "bands": map[string]any{"bass": map[string]any{"unit_energy": .3}}},
		{"status": "suspect", "quality_status": "suspect", "quality_reason": "all_zero_audio", "source": "kernel_l3_offline_analyzer", "track_id": "t2", "clip_id": "c2", "coverage_ratio": 1.0, "bands": map[string]any{"bass": map[string]any{"unit_energy": 0.0}}},
	}}
	project := buildProjectPackage(state, TargetRef{Kind: "project", ID: "current"}, ListenScope{Source: ListenSourceScope{Mode: "full_project"}}, snap)
	projection := mom.Build(mom.Input{
		ObservationID: "obs-silence", MixSessionID: "mix-silence", Intent: mom.IntentProjectFrequencyObservation,
		Args: map[string]any{"mom_intent": mom.IntentProjectFrequencyObservation}, ProjectPackage: project,
		ListenScope: map[string]any{"source": map[string]any{"mode": "full_project"}, "time": map[string]any{"mode": "full_song"}},
	})
	if projection.FrequencyRelationship == nil {
		t.Fatal("frequency relationship missing")
	}
	coverage := projection.FrequencyRelationship.Coverage
	if int(numberFromMap(coverage, "eligible_track_count")) != 2 || int(numberFromMap(coverage, "missing_track_count")) != 0 {
		t.Fatalf("silent source blocked full coverage: %#v", coverage)
	}
	for _, profile := range projection.FrequencyRelationship.TrackProfiles {
		if cleanAnyString(profile["track_id"]) == "t2" && (!boolFromAny(profile["silence_confirmed"]) || cleanAnyString(profile["status"]) != "partial") {
			t.Fatalf("silent L3 row was not projected as partial no-change evidence: %#v", profile)
		}
	}
}

func TestL3PackageSilenceRequiresCompleteCoverage(t *testing.T) {
	feature := acousticpackage.FeatureStatus{Status: acousticpackage.StatusSuspect, Reason: "all_zero_audio", Ref: map[string]any{"quality_reason": "all_zero_audio", "coverage_ratio": 1.0}}
	if !l3FeatureConfirmsDeterministicSilence(feature) {
		t.Fatal("complete all-zero L3 source was not accepted as deterministic silence")
	}
	feature.Ref["coverage_ratio"] = .5
	if l3FeatureConfirmsDeterministicSilence(feature) {
		t.Fatal("partial all-zero coverage must not prove deterministic silence")
	}
}

func TestDedicatedFrequencyIntentGetsCompactProjectionAndLegacyProjectionDoesNot(t *testing.T) {
	project := map[string]any{"tracks": []any{map[string]any{
		"track_id": "t1", "name": "One", "frequency_evidence": map[string]any{
			"status": "ready", "source": "kernel_l3_offline_analyzer", "tap_point": "source_file_pre_fx",
			"bands": map[string]any{"bass": map[string]any{"unit_energy": .3}},
		},
	}}}
	legacy := projectedProjectTracks(project, false)
	if _, ok := legacy[0]["frequency_evidence"]; ok {
		t.Fatalf("legacy compact projection changed shape: %#v", legacy)
	}
	dedicated := projectedProjectTracks(project, true)
	if _, ok := dedicated[0]["frequency_evidence"]; !ok {
		t.Fatalf("dedicated compact projection lost frequency evidence: %#v", dedicated)
	}

	input := mom.Input{
		ObservationID: "obs-frequency", MixSessionID: "mix-frequency", Intent: mom.IntentProjectFrequencyObservation,
		Args:           map[string]any{"mom_intent": mom.IntentProjectFrequencyObservation},
		ProjectPackage: map[string]any{"status": "ready", "project_uuid": "p1", "project_epoch": "e1", "project_revision": "1", "tracks": dedicated},
		ListenScope:    map[string]any{"source": map[string]any{"mode": "full_project"}, "time": map[string]any{"mode": "full_song"}},
	}
	projection := mom.Build(input)
	data, err := json.Marshal(mom.ContextProjection(projection))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{`"raw_samples":`, `"quality_evidence":`, `"render_file_path":`, `"spectrogram_tiles":`, `"time_segments":`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("dedicated context leaked %s: %s", forbidden, text)
		}
	}
	if projection.FrequencyRelationship == nil || projection.FrequencyRelationship.SchemaVersion != mom.FrequencyRelationshipSchema {
		t.Fatalf("dedicated MOM projection missing: %#v", projection.FrequencyRelationship)
	}
}
