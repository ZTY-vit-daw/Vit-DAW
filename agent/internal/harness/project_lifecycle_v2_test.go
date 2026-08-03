package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/artifacts"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/projectworkspace"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tools"
)

func TestOrdinarySaveDoesNotCreateProjectPackageOrCopyAgentStore(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_save_v2"
	if _, err := history.EnsureWorkingSession(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	prepared, err := history.PrepareWorkingSessionSave(projectPath, projectUUID, "save")
	if err != nil {
		t.Fatal(err)
	}
	h := New(nil, nil, nil)
	if _, err := h.ActivateProjectStore(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	workspace, err := h.applyProjectLifecycle(context.Background(), tools.CommandSpec{CommandName: "save_project"}, map[string]any{
		"file_path": projectPath, "history_prepare_id": firstString(prepared, "prepare_id"),
		"agent_history_generation": firstString(prepared, "generation_id"),
	}, map[string]any{
		"status": "ok", "project_lifecycle": "save", "project_path": projectPath,
		"project_uuid": projectUUID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !boolValueDefault(workspace["working_session_committed"], false) {
		t.Fatalf("workspace=%#v", workspace)
	}
	for _, legacy := range []string{
		filepath.Join(root, "song.vit_project"),
		filepath.Join(root, ".vit_projects", projectUUID),
	} {
		if _, err := os.Stat(legacy); !os.IsNotExist(err) {
			t.Fatalf("ordinary save created legacy package %s: %v", legacy, err)
		}
	}
}

func TestSaveAsProjectForksAndActivatesV2StoreWithoutMediaCopy(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.vit")
	targetPath := filepath.Join(root, "target.vit")
	if err := os.WriteFile(sourcePath, []byte("source-project"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("target-project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sourceUUID = "vitproj_save_as_source"
	const targetUUID = "vitproj_save_as_target"
	if _, err := history.EnsureWorkingSession(sourcePath, sourceUUID); err != nil {
		t.Fatal(err)
	}
	prepared, err := history.PrepareWorkingSessionSave(sourcePath, sourceUUID, "save_as")
	if err != nil {
		t.Fatal(err)
	}
	h := New(nil, nil, nil)
	sourceRoots, err := h.ActivateProjectStore(sourcePath, sourceUUID)
	if err != nil {
		t.Fatal(err)
	}
	observationPath := filepath.Join(sourceRoots.Agent, "observations", "obs_source.json")
	if err := os.MkdirAll(filepath.Dir(observationPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(observationPath, []byte(`{"schema_version":"mix_observation.v2","project_uuid":"vitproj_save_as_source","project_path":"`+filepath.ToSlash(sourcePath)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sourceManifestBefore, err := os.ReadFile(filepath.Join(sourceRoots.Agent, projectstore.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := h.applyProjectLifecycle(context.Background(), tools.CommandSpec{CommandName: "save_as_project"}, map[string]any{
		"source_project_path": sourcePath, "source_project_uuid": sourceUUID,
		"file_path": targetPath, "history_prepare_id": firstString(prepared, "prepare_id"),
		"agent_history_generation": firstString(prepared, "generation_id"),
	}, map[string]any{
		"status": "ok", "project_lifecycle": "save_as", "project_path": targetPath,
		"project_uuid": targetUUID, "source_project_uuid": sourceUUID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !boolValueDefault(workspace["forked_agent_store"], false) {
		t.Fatalf("workspace=%#v", workspace)
	}
	current, ok := projectstore.Current()
	if !ok || current.ProjectUUID != targetUUID || !samePath(current.ProjectPath, targetPath) {
		t.Fatalf("active store=%+v ok=%v", current, ok)
	}
	targetRoots, err := projectstore.Resolve(targetPath, targetUUID)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := projectstore.Load(targetRoots)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ProjectUUID != targetUUID || !samePath(manifest.ProjectPath, targetPath) || manifest.OriginProjectUUID != sourceUUID {
		t.Fatalf("target manifest=%+v", manifest)
	}
	targetObservation, err := os.ReadFile(filepath.Join(targetRoots.Agent, "observations", "obs_source.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(targetObservation) == "" || string(targetObservation) == string([]byte(`{"project_uuid":"`+sourceUUID+`"}`)) {
		t.Fatalf("target observation was not forked/rebound: %s", targetObservation)
	}
	sourceManifestAfter, err := os.ReadFile(filepath.Join(sourceRoots.Agent, projectstore.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(sourceManifestAfter) != string(sourceManifestBefore) {
		t.Fatal("source v2 manifest changed during Save As")
	}
	if _, err := os.Stat(filepath.Join(root, "target_Media")); !os.IsNotExist(err) {
		t.Fatalf("Save As copied media unexpectedly: %v", err)
	}
}

func TestRestoreProjectPersistencePrefersV2OverLegacyPackage(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_restore_v2"
	if _, _, err := projectstore.Ensure(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "song.vit_project")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "manifest.json"), []byte(`{"schema_version":"corrupt_legacy"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := restoreProjectPersistence(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status != "v2" || restored.Reason != "agent_store_v2_authoritative" {
		t.Fatalf("restore=%+v", restored)
	}
}

func TestRestoreProjectPersistenceRebindsCopiedV2StorePath(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source", "song.vit")
	targetPath := filepath.Join(root, "target", "song.vit")
	const projectUUID = "vitproj_restore_relocated"
	sourceRoots, _, err := projectstore.Ensure(sourcePath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := os.ReadFile(filepath.Join(sourceRoots.Agent, projectstore.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	targetRoots, err := projectstore.Resolve(targetPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetRoots.Agent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetRoots.Agent, projectstore.ManifestFile), manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := restoreProjectPersistence(targetPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status != "v2" {
		t.Fatalf("restore=%+v", restored)
	}
	manifest, err := projectstore.Load(targetRoots)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(manifest.ProjectPath, targetPath) {
		t.Fatalf("copied store path was not rebound: %+v", manifest)
	}
}

func TestExternalProjectOpenAcceptsSelfContainedV2SnapshotWithoutParentPath(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	projectPath := filepath.Join(root, "snapshot", "song.vit")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_self_contained_snapshot"
	if _, _, err := projectstore.Ensure(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}

	h := New(nil, nil, nil)
	result, err := h.applyExternalProjectOpened(context.Background(), map[string]any{
		"project_path":        projectPath,
		"project_uuid":        projectUUID,
		"parent_project_uuid": "vitproj_missing_unsaved_parent",
	})
	if err != nil {
		t.Fatalf("self-contained v2 snapshot required its missing parent: %v", err)
	}
	if firstString(result, "status") != "ok" || !samePath(firstString(result, "project_path"), projectPath) {
		t.Fatalf("external open result=%+v", result)
	}
}

func TestActiveV2WorkflowDoesNotWriteLegacyWorkspaceStores(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	devRoot := filepath.Join(root, "dev")
	legacyWorkspace := filepath.Join(devRoot, "VitApp", "Workspace")
	if err := os.MkdirAll(legacyWorkspace, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_DAW_DEV_ROOT", devRoot)
	t.Setenv("VIT_MIXBOARD_ROOT", "")
	t.Setenv("VIT_AGENT_JOURNAL_PATH", "")
	projectPath := filepath.Join(root, "song.vit")
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_zero_workspace"
	h := New(nil, nil, nil)
	if _, err := h.ActivateProjectStore(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	h.journal.Record(journal.Action{AgentActionID: "act_v2", Tool: "track_gain_adjust", Status: journal.StatusSucceeded})
	if _, err := artifacts.NewStore("").Upsert(artifacts.Artifact{ID: "art_v2", Kind: "text", Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	if _, err := mixboard.NewStore("").RequestObservation(mixboard.Request{
		MixSessionID: "mix_zero_workspace", TargetRef: mixboard.TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"project_uuid": projectUUID, "project_path": projectPath, "duration_seconds": 1.0},
		Args: map[string]any{"feature_snapshot": map[string]any{
			"waveform_envelope": map[string]any{"status": "ready", "track_id": "track_1", "rms": 0.1, "peak_abs": 0.2},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(legacyWorkspace, "Logs", "agent_journal.json"),
		filepath.Join(legacyWorkspace, "Artifacts", "mixboard"),
		filepath.Join(legacyWorkspace, "Artifacts", "manifest.jsonl"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("legacy workspace path was written: %s (%v)", path, err)
		}
	}
}

func TestKernelL3TelemetryIsPersistedOnceByAgent(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_l3_owner"
	h := New(nil, nil, nil)
	if _, err := h.ActivateProjectStore(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	event := map[string]any{
		"command": "audio_feature_data_ready", "feature_type": "band_energy_summary",
		"status": "ready", "quality_status": "ready", "project_uuid": projectUUID,
		"track_id": "track_1", "clip_id": "clip_1", "file_path": filepath.Join(root, "audio.wav"),
		"source_revision": "source_rev_1", "analyzer_revision": "dad_l3_offline_analyzer.v1",
		"bands": map[string]any{"low": map[string]any{"energy": 1.0}},
	}
	h.IngestKernelTelemetry(event)
	h.IngestKernelTelemetry(event)
	rows, path, err := projectworkspace.ReadL3Features(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("agent-owned L3 rows=%d want=1 path=%s rows=%#v", len(rows), path, rows)
	}
	if firstString(rows[0], "project_uuid", "project_id") != projectUUID {
		t.Fatalf("L3 row identity=%#v", rows[0])
	}
}

func TestSaveAsProjectDefersAuditToActivatedTargetStore(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.vit")
	targetPath := filepath.Join(root, "target.vit")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sourceUUID = "vitproj_invoke_source"
	const targetUUID = "vitproj_invoke_target"
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": sourcePath, "project_uuid": sourceUUID})
	kernel := &fakeKernelClient{replies: []map[string]any{{
		"status": "ok", "project_lifecycle": "save_as", "project_path": targetPath,
		"project_uuid": targetUUID, "source_project_uuid": sourceUUID,
	}}}
	h := NewWithSender(kernel, project, nil)
	if _, err := history.EnsureWorkingSession(sourcePath, sourceUUID); err != nil {
		t.Fatal(err)
	}
	sourceRoots, err := h.ActivateProjectStore(sourcePath, sourceUUID)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(sourceRoots.Agent, projectstore.ManifestFile)
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	response, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "project.save_as", Confirmed: true, Args: map[string]any{"file_path": targetPath},
	})
	if err != nil || response.Status != "ok" {
		t.Fatalf("response=%+v err=%v", response, err)
	}
	manifestAfter, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(manifestAfter) != string(manifestBefore) {
		t.Fatal("source manifest changed during Save As")
	}
	if entries, readErr := os.ReadDir(filepath.Join(sourceRoots.Agent, "journal")); readErr == nil && len(entries) > 0 {
		t.Fatalf("source journal changed during Save As: %#v", entries)
	} else if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	current, ok := projectstore.Current()
	if !ok || current.ProjectUUID != targetUUID {
		t.Fatalf("target store not active: %+v ok=%v", current, ok)
	}
	targetJournal, err := journal.NewSharded(20, filepath.Join(current.Agent, "journal"), targetUUID)
	if err != nil {
		t.Fatal(err)
	}
	actions := targetJournal.Recent(10)
	if len(actions) != 1 || actions[0].CommandName != "save_as_project" || actions[0].Status != journal.StatusSucceeded {
		t.Fatalf("target audit=%#v", actions)
	}
}

func TestV2ProjectStoreSurvivesRestartAndContinues(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_restart"
	h1 := New(nil, nil, nil)
	roots, err := h1.ActivateProjectStore(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	h1.journal.Record(journal.Action{AgentActionID: "act_before_restart", Tool: "track_gain_adjust", Status: journal.StatusSucceeded})
	store1 := mixboard.NewStore("")
	observation, err := store1.RequestObservation(mixboard.Request{
		MixSessionID: "mix_restart", TargetRef: mixboard.TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"project_uuid": projectUUID, "project_path": projectPath, "duration_seconds": 2.0},
		Args: map[string]any{"feature_snapshot": map[string]any{
			"waveform_envelope": map[string]any{"status": "ready", "track_id": "track_1", "rms": 0.1, "peak_abs": 0.2},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	decisionTime := time.Unix(1_800_000_000, 0).UTC()
	if _, err := store1.RecordCapabilitySession(orchestration.PlanningSession{
		SchemaVersion: orchestration.SchemaVersion, ID: "session_restart", ProjectUUID: projectUUID,
		EngineOwner: orchestration.EngineV1, Status: orchestration.StatusCompleted,
		Goal: "restart persistence", Invocation: orchestration.CapabilityInvocation{CapabilityID: "static_mix.static_balance.v0", CapabilityVer: "v0"},
		CreatedAt: decisionTime.Add(-time.Minute), UpdatedAt: decisionTime,
	}); err != nil {
		t.Fatal(err)
	}
	if len(observation.Observation.EvidenceRefs) != 1 {
		t.Fatalf("evidence refs=%#v", observation.Observation.EvidenceRefs)
	}
	evidenceRef := observation.Observation.EvidenceRefs[0]

	projectstore.Deactivate()
	h2 := New(nil, nil, nil)
	if _, err := h2.ActivateProjectStore(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	if action, ok := h2.JournalGet("act_before_restart"); !ok || action.Status != journal.StatusSucceeded {
		t.Fatalf("reloaded action=%+v ok=%v", action, ok)
	}
	store2 := mixboard.NewStore("")
	read, err := store2.Read(mixboard.ReadRequest{MixSessionID: "mix_restart", ObservationID: observation.Observation.ObservationID})
	if err != nil || firstString(read, "observation_id") != observation.Observation.ObservationID {
		t.Fatalf("reloaded observation=%#v err=%v", read, err)
	}
	if _, err := projectstore.GetEvidence(roots, evidenceRef); err != nil {
		t.Fatalf("reloaded evidence: %v", err)
	}
	board, err := store2.ReadProjectDecisionBoard(projectUUID)
	if err != nil || board.DecisionCount != 1 {
		t.Fatalf("reloaded decision board=%+v err=%v", board, err)
	}
	h2.journal.Record(journal.Action{AgentActionID: "act_after_restart", Tool: "track_gain_adjust", Status: journal.StatusSucceeded})
	if _, ok := h2.JournalGet("act_after_restart"); !ok {
		t.Fatal("journal could not continue after restart")
	}
}
