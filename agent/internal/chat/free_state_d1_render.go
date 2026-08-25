package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/kernel"
)

const d1RenderSchema = "vit.free_state_d1_render.v1"

func d1AuditionCandidates(before, after map[string]any, baselineCommit, treatmentCommit, projectRef, projectUUID string) ([]kernel.AuditionCandidate, error) {
	beforeRevision := firstStringFromMap(before, "project_revision")
	afterRevision := firstStringFromMap(after, "project_revision")
	if beforeRevision == "" || afterRevision == "" || beforeRevision == afterRevision {
		return nil, fmt.Errorf("D1-S1 A/B renders require distinct before and after project revisions")
	}
	for phase, row := range map[string]map[string]any{"before": before, "after": after} {
		if firstStringFromMap(row, "status") != "ready" || !validD1RenderFile(firstStringFromMap(row, "file_path")) || firstStringFromMap(row, "render_revision") == "" || firstStringFromMap(row, "sha256") == "" {
			return nil, fmt.Errorf("D1-S1 %s A/B render provenance is incomplete", phase)
		}
	}
	return []kernel.AuditionCandidate{
		{ID: "candidate-a", Label: "A", SourceKind: "audio_file", SourceRef: firstStringFromMap(before, "file_path"), PreviewRef: "audio_file:" + firstStringFromMap(before, "sha256"), CheckpointRef: baselineCommit, CommitID: baselineCommit, ProjectPath: projectRef, ProjectUUID: projectUUID, ProjectRevision: beforeRevision, RenderRevision: firstStringFromMap(before, "render_revision"), PreviewRevision: firstStringFromMap(before, "preview_revision"), Scope: "target"},
		{ID: "candidate-b", Label: "B", SourceKind: "audio_file", SourceRef: firstStringFromMap(after, "file_path"), PreviewRef: "audio_file:" + firstStringFromMap(after, "sha256"), CheckpointRef: treatmentCommit, CommitID: treatmentCommit, ProjectPath: projectRef, ProjectUUID: projectUUID, ProjectRevision: afterRevision, RenderRevision: firstStringFromMap(after, "render_revision"), PreviewRevision: firstStringFromMap(after, "preview_revision"), Scope: "target"},
	}, nil
}

func (s *Server) ensureD1Render(ctx context.Context, loop *freeStateReasoningLoop, phase, projectRevision, checkpointRef string) (map[string]any, error) {
	if s == nil || s.kernel == nil || s.harness == nil || loop == nil || loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() {
		return nil, fmt.Errorf("D1-S1 render dependencies are unavailable")
	}
	phase = strings.ToLower(strings.TrimSpace(phase))
	if phase != "before" && phase != "after" {
		return nil, fmt.Errorf("D1-S1 render phase must be before or after")
	}
	projectRevision = strings.TrimSpace(projectRevision)
	if projectRevision == "" {
		return nil, fmt.Errorf("D1-S1 render must be revision-bound")
	}
	key := phase + "_render"
	row := firstMapFromAny(loop.D1State[key])
	if firstStringFromMap(row, "project_revision") == projectRevision {
		if path := firstStringFromMap(row, "file_path"); validD1RenderFile(path) {
			return s.finishD1Render(loop, key, row, path, checkpointRef)
		}
		if jobID := firstStringFromMap(row, "job_id"); jobID != "" {
			return s.waitD1Render(ctx, loop, key, row, jobID, checkpointRef)
		}
	}

	root := filepath.Join(s.artifactStore().Root, "d1_s1", sanitizeCanaryID(loop.Experiment.ID))
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create D1-S1 render directory: %w", err)
	}
	path := filepath.Join(root, phase+"_revision_"+sanitizeCanaryID(projectRevision)+".wav")
	if validD1RenderFile(path) {
		row = map[string]any{"schema_version": d1RenderSchema, "phase": phase, "status": "ready", "file_path": path, "project_revision": projectRevision, "checkpoint_ref": checkpointRef}
		return s.finishD1Render(loop, key, row, path, checkpointRef)
	}
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove incomplete D1-S1 render: %w", err)
		}
	}
	reply, _, err := s.kernel.SendCommand(ctx, map[string]any{"cmd": "start_render", "file_path": path, "bit_depth": 24, "use_master_plugins": true})
	if err != nil {
		return nil, fmt.Errorf("start D1-S1 %s render: %w", phase, err)
	}
	if strings.EqualFold(firstStringFromMap(reply, "status"), "error") {
		return nil, fmt.Errorf("start D1-S1 %s render: %s", phase, firstNonEmpty(firstStringFromMap(reply, "message"), "kernel rejected render"))
	}
	jobID := firstStringFromMap(reply, "job_id")
	if jobID == "" {
		return nil, fmt.Errorf("start D1-S1 %s render returned no job_id", phase)
	}
	row = map[string]any{
		"schema_version": d1RenderSchema, "phase": phase, "status": "rendering", "job_id": jobID, "file_path": path,
		"project_revision": projectRevision, "checkpoint_ref": checkpointRef, "range": "full_project", "bit_depth": 24,
		"use_master_plugins": true, "started_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	s.storeD1RenderState(loop, key, row)
	return s.waitD1Render(ctx, loop, key, row, jobID, checkpointRef)
}

func (s *Server) waitD1Render(ctx context.Context, loop *freeStateReasoningLoop, key string, row map[string]any, jobID, checkpointRef string) (map[string]any, error) {
	result, err := s.harness.WaitRender(ctx, jobID)
	path := firstStringFromMap(row, "file_path")
	if err != nil && !validD1RenderFile(path) {
		return nil, fmt.Errorf("wait for D1-S1 render %s: %w", jobID, err)
	}
	if err == nil {
		if result.Status != "ready" {
			return nil, fmt.Errorf("D1-S1 render %s failed: %s", jobID, firstNonEmpty(result.Error, result.Status))
		}
		if result.FilePath != "" && !sameD1RenderPath(result.FilePath, path) {
			return nil, fmt.Errorf("D1-S1 render %s completed for an unexpected file", jobID)
		}
	}
	if !validD1RenderFile(path) {
		return nil, fmt.Errorf("D1-S1 render %s produced no valid WAV", jobID)
	}
	return s.finishD1Render(loop, key, row, path, checkpointRef)
}

func (s *Server) finishD1Render(loop *freeStateReasoningLoop, key string, row map[string]any, path, checkpointRef string) (map[string]any, error) {
	digest, size, err := hashD1RenderFile(path)
	if err != nil {
		return nil, err
	}
	row = cloneContext(row)
	row["status"] = "ready"
	row["file_path"] = filepath.Clean(path)
	row["checkpoint_ref"] = firstNonEmpty(checkpointRef, firstStringFromMap(row, "checkpoint_ref"))
	row["sha256"] = digest
	row["size_bytes"] = size
	row["render_revision"] = "d1_render:" + sanitizeCanaryID(loop.Experiment.ID) + ":" + firstStringFromMap(row, "phase") + ":" + firstStringFromMap(row, "project_revision") + ":" + digest[:16]
	row["preview_revision"] = "sha256:" + digest
	row["completed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	s.storeD1RenderState(loop, key, row)
	return cloneContext(row), nil
}

func (s *Server) storeD1RenderState(loop *freeStateReasoningLoop, key string, row map[string]any) {
	if loop.D1State == nil {
		loop.D1State = map[string]any{}
	}
	loop.D1State[key] = cloneContext(row)
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(*loop)
	s.persistCurrentProjectWorkspace()
}

func validD1RenderFile(path string) bool {
	if strings.TrimSpace(path) == "" || !strings.EqualFold(filepath.Ext(path), ".wav") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 44
}

func sameD1RenderPath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(strings.TrimSpace(left)), filepath.Clean(strings.TrimSpace(right)))
}

func hashD1RenderFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open D1-S1 render: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, fmt.Errorf("hash D1-S1 render: %w", err)
	}
	if size <= 44 {
		return "", 0, fmt.Errorf("D1-S1 render is not a valid WAV payload")
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}
