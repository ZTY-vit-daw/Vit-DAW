package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (h *Harness) prepareCollaborationCandidate(cmd, requestContext map[string]any) (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	scope := firstNonEmpty(firstString(cmd, "scope"), "target")
	if scope != "target" {
		return nil, fmt.Errorf("full_project_preview_unsupported: collaboration candidate preparation supports target scope only")
	}
	engineeringKind := strings.ToLower(strings.TrimSpace(firstString(cmd, "engineering_source_kind", "source_kind")))
	if engineeringKind != "branch" && engineeringKind != "worktree" && engineeringKind != "checkpoint" && engineeringKind != "preview_state" {
		return nil, fmt.Errorf("candidate engineering source_kind must be branch, worktree, checkpoint, or preview_state")
	}
	engineeringRef := firstString(cmd, "engineering_source_ref", "source_ref")
	audioFile := firstString(cmd, "audio_file", "audio_file_path", "preview_audio_file")
	if engineeringRef == "" || audioFile == "" {
		return nil, fmt.Errorf("candidate requires engineering source_ref and audio_file")
	}
	info, err := os.Stat(audioFile)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("candidate_audio_missing: %s", audioFile)
	}
	projectUUID := firstNonEmpty(firstString(cmd, "project_uuid"), firstString(requestContext, "project_uuid", "project_id"))
	projectPath := firstString(cmd, "project_path", "source_project_path")
	projectRevision := firstString(cmd, "project_revision", "source_project_revision")
	commitID := firstString(cmd, "commit_id", "checkpoint_ref")
	branchRef := firstString(cmd, "branch_ref")
	worktreeRef := firstString(cmd, "worktree_ref")
	reservationID := firstString(cmd, "reservation_id")
	if commitID == "" || projectRevision == "" {
		return nil, fmt.Errorf("candidate commit_id and project_revision are required")
	}
	if engineeringKind == "worktree" && reservationID == "" {
		return nil, fmt.Errorf("worktree candidate requires reservation_id")
	}
	ownerAgentID := firstNonEmpty(firstString(cmd, "owner_agent_id", "agent_id"), firstString(requestContext, "owner_agent_id", "agent_id"))
	if reservationID != "" {
		reservation, ok := h.collaboration.Get(reservationID)
		if !ok {
			return nil, fmt.Errorf("reservation not found: %s", reservationID)
		}
		if projectUUID != "" && reservation.ProjectUUID != projectUUID {
			return nil, fmt.Errorf("candidate project_uuid does not match reservation")
		}
		if worktreeRef != "" && reservation.WorktreeRef != worktreeRef {
			return nil, fmt.Errorf("candidate worktree_ref does not match reservation")
		}
		if projectPath != "" && reservation.WorktreePath != "" && !samePath(projectPath, reservation.WorktreePath) {
			return nil, fmt.Errorf("candidate project_path does not match reserved worktree")
		}
		if ownerAgentID != "" && reservation.OwnerAgentID != ownerAgentID {
			return nil, fmt.Errorf("candidate owner_agent_id does not match reservation")
		}
		projectUUID = firstNonEmpty(projectUUID, reservation.ProjectUUID)
		projectPath = firstNonEmpty(projectPath, reservation.WorktreePath)
		worktreeRef = firstNonEmpty(worktreeRef, reservation.WorktreeRef)
		ownerAgentID = firstNonEmpty(ownerAgentID, reservation.OwnerAgentID)
	}
	if engineeringKind == "worktree" && worktreeRef == "" {
		worktreeRef = engineeringRef
	}
	if engineeringKind == "branch" && branchRef == "" {
		branchRef = engineeringRef
	}
	if projectUUID == "" || projectPath == "" {
		return nil, fmt.Errorf("candidate project_uuid and project_path are required")
	}
	hash := sha256.New()
	file, err := os.Open(audioFile)
	if err != nil {
		return nil, err
	}
	_, copyErr := file.WriteTo(hash)
	closeErr := file.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	candidateID := firstNonEmpty(firstString(cmd, "candidate_id", "id"), "candidate-"+checksum[:8])
	renderRevision := firstNonEmpty(firstString(cmd, "render_revision"), "audio-file:"+checksum)
	previewRevision := firstNonEmpty(firstString(cmd, "preview_revision"), renderRevision+":"+fmt.Sprint(info.Size())+":"+info.ModTime().UTC().Format(time.RFC3339Nano))
	artifactRef := "audio-sha256:" + checksum
	candidate := map[string]any{
		"id": candidateID, "label": firstNonEmpty(firstString(cmd, "label"), candidateID), "status": "preview_prepared", "scope": scope,
		"source_kind": "audio_file", "source_ref": filepath.Clean(audioFile), "engineering_source_kind": engineeringKind, "engineering_source_ref": engineeringRef,
		"branch_ref": branchRef, "worktree_ref": worktreeRef, "checkpoint_ref": firstString(cmd, "checkpoint_ref"), "commit_id": commitID,
		"project_path": projectPath, "project_uuid": projectUUID, "project_revision": projectRevision, "render_revision": renderRevision, "preview_revision": previewRevision,
		"preview_ref": "file://" + filepath.Clean(audioFile), "reservation_id": reservationID, "owner_agent_id": ownerAgentID,
		"artifact_ref": artifactRef, "audio_file_size": info.Size(), "prepared_at": time.Now().UTC(),
	}
	var reservation map[string]any
	if reservationID != "" {
		updated, updateErr := h.collaboration.RecordCandidate(reservationID, candidateID, artifactRef, "")
		if updateErr != nil {
			return nil, updateErr
		}
		reservation, _ = structMap(map[string]any{"reservation": updated})
		reservation = mapFromAny(reservation["reservation"])
	}
	return map[string]any{"status": "ok", "candidate": candidate, "reservation": reservation, "preview_preparation": map[string]any{"scope": scope, "source_kind": "audio_file", "audio_file": filepath.Clean(audioFile), "checksum_sha256": checksum, "render_revision": renderRevision, "preview_revision": previewRevision, "kernel_prepare_required": true}}, nil
}
