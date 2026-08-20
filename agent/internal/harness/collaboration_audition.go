package harness

import (
	"context"
	"fmt"
	"strings"
)

func (h *Harness) prepareCollaborationAudition(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	vsp, ok := h.vspKernel()
	if !ok {
		return nil, fmt.Errorf("kernel_audition_unavailable")
	}
	candidateA := collaborationMapFromAny(cmd["candidate_a"])
	candidateB := collaborationMapFromAny(cmd["candidate_b"])
	if len(candidateA) == 0 || len(candidateB) == 0 {
		return nil, fmt.Errorf("collaboration.audition_prepare requires candidate_a and candidate_b")
	}
	scope := firstNonEmpty(firstString(cmd, "scope"), "target")
	if scope != "target" {
		return nil, fmt.Errorf("full_project_preview_unsupported: collaboration audition supports target scope only")
	}
	for label, candidate := range map[string]map[string]any{"candidate_a": candidateA, "candidate_b": candidateB} {
		if firstString(candidate, "source_kind") != "audio_file" || firstString(candidate, "source_ref") == "" {
			return nil, fmt.Errorf("%s must use source_kind=audio_file with a real source_ref", label)
		}
		if firstString(candidate, "project_uuid") == "" || firstString(candidate, "project_revision") == "" {
			return nil, fmt.Errorf("%s project identity and revision are required", label)
		}
	}
	if samePath(firstString(candidateA, "source_ref"), firstString(candidateB, "source_ref")) {
		return nil, fmt.Errorf("audition candidates must use distinct audio sources")
	}
	request := map[string]any{"conversation_id": firstString(cmd, "conversation_id"), "session_id": firstString(cmd, "session_id", "audition_session_id"), "scope": scope, "active_project_ref": firstString(cmd, "active_project_ref", "project_path"), "active_project_revision": firstString(cmd, "active_project_revision", "project_revision"), "timeline_revision": firstString(cmd, "timeline_revision", "project_revision"), "candidate_a": candidateA, "candidate_b": candidateB}
	if strings.TrimSpace(fmt.Sprint(request["session_id"])) == "" {
		return nil, fmt.Errorf("audition session_id is required")
	}
	result, err := vsp.SendVSPCommand(ctx, "audition.prepare", request)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("kernel returned empty audition response")
	}
	out := collaborationCloneMap(result.LegacyReply)
	if len(out) == 0 {
		out = collaborationCloneMap(result.Payload)
	}
	if len(out) == 0 {
		out = collaborationCloneMap(result.Response)
	}
	if out == nil {
		out = map[string]any{}
	}
	out["status"] = firstNonEmpty(firstString(out, "status"), "ok")
	out["candidate_a"] = candidateA
	out["candidate_b"] = candidateB
	out["active_project_plane_unchanged"] = true
	return out, nil
}

func collaborationMapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if row, ok := value.(map[string]any); ok {
		return collaborationCloneMap(row)
	}
	return nil
}

func collaborationCloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}
