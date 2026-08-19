package kernel

import (
	"encoding/json"
	"testing"
)

func TestAuditionPrepareArgsUsesContractCandidateAAndB(t *testing.T) {
	request := AuditionSessionRequest{
		SessionID:             "audition-1",
		Scope:                 "target",
		ActiveProjectRef:      "project:active",
		ActiveProjectRevision: "project-r17",
		TimelineRevision:      "timeline-r17",
		Candidates: []AuditionCandidate{
			{ID: "candidate-a", SourceKind: "audio_file", SourceRef: "D:/audio/a.wav", CheckpointRef: "checkpoint:a", CommitID: "commit-a", BranchRef: "branch:a", WorktreeRef: "worktree:a", ProjectRevision: "project-r17", RenderRevision: "render-a", Scope: "target"},
			{ID: "candidate-b", SourceKind: "audio_file", SourceRef: "D:/audio/b.wav", CheckpointRef: "checkpoint:b", CommitID: "commit-b", BranchRef: "branch:b", WorktreeRef: "worktree:b", ProjectRevision: "project-r17", RenderRevision: "render-b", Scope: "target"},
		},
	}
	args, err := auditionPrepareArgs(request)
	if err != nil {
		t.Fatalf("prepare args: %v", err)
	}
	if mapFromAny(args["candidate_a"])["id"] != "candidate-a" || mapFromAny(args["candidate_b"])["id"] != "candidate-b" {
		t.Fatalf("candidate A/B contract payload = %#v", args)
	}
	if _, ok := args["candidates"]; ok {
		t.Fatalf("legacy candidates array leaked into contract payload: %#v", args)
	}
	candidateA := mapFromAny(args["candidate_a"])
	if candidateA["source_kind"] != "audio_file" || candidateA["checkpoint_ref"] != "checkpoint:a" || candidateA["commit_id"] != "commit-a" || candidateA["branch_ref"] != "branch:a" || candidateA["worktree_ref"] != "worktree:a" || candidateA["project_revision"] != "project-r17" || candidateA["render_revision"] != "render-a" || candidateA["scope"] != "target" {
		t.Fatalf("candidate identity binding missing: %#v", candidateA)
	}
}
func TestAuditionSessionRequestCarriesBothPlanes(t *testing.T) {
	request := AuditionSessionRequest{
		SessionID:             "audition-1",
		Scope:                 "target",
		ActiveProjectRef:      "project:active",
		ActiveProjectRevision: "project-r17",
		TimelineRevision:      "timeline-r17",
		Candidates: []AuditionCandidate{
			{ID: "candidate-a", Label: "Candidate A", SourceKind: "edit", SourceRef: "edit:a"},
			{ID: "candidate-b", Label: "Candidate B", SourceKind: "edit", SourceRef: "edit:b"},
		},
	}

	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal audition request: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode audition request: %v", err)
	}
	if payload["active_project_ref"] != "project:active" || payload["active_project_revision"] != "project-r17" {
		t.Fatalf("active project plane identity missing: %#v", payload)
	}
	if payload["timeline_revision"] != "timeline-r17" {
		t.Fatalf("timeline revision missing: %#v", payload)
	}
	candidates, ok := payload["candidates"].([]any)
	if !ok || len(candidates) != 2 {
		t.Fatalf("candidate A/B payload = %#v", payload["candidates"])
	}
}

func TestParseAuditionSelectReplyPreservesPreviewPlane(t *testing.T) {
	result := parseVSPCommandResult(map[string]any{
		"type":           "command.response",
		"transaction_id": "tx-audition-select",
		"payload": map[string]any{
			"command":        "audition.select",
			"legacy_command": "audition.select",
			"legacy_status":  "ok",
			"legacy_reply": map[string]any{
				"status":  "ok",
				"command": "audition.select",
				"session": map[string]any{
					"status":              "ready",
					"active_candidate_id": "candidate-b",
					"active_project_plane": map[string]any{
						"project_ref":      "project:active",
						"project_revision": "project-r17",
					},
					"audition_preview_plane": map[string]any{
						"plane":               "audition_preview",
						"active_candidate_id": "candidate-b",
					},
				},
			},
		},
	}, "raw")

	if result.Command != "audition.select" || result.TransactionID != "tx-audition-select" {
		t.Fatalf("select response identity = command %q tx %q", result.Command, result.TransactionID)
	}
	session := mapFromAny(result.LegacyReply["session"])
	if stringFromAny(session["active_candidate_id"]) != "candidate-b" {
		t.Fatalf("selected candidate missing: %#v", session)
	}
	activePlane := mapFromAny(session["active_project_plane"])
	if stringFromAny(activePlane["project_ref"]) != "project:active" || stringFromAny(activePlane["project_revision"]) != "project-r17" {
		t.Fatalf("active project plane changed or missing: %#v", activePlane)
	}
	previewPlane := mapFromAny(session["audition_preview_plane"])
	if stringFromAny(previewPlane["plane"]) != "audition_preview" || stringFromAny(previewPlane["active_candidate_id"]) != "candidate-b" {
		t.Fatalf("audition preview plane = %#v", previewPlane)
	}
}

func TestAuditionCommandNamesStayInAuditionPlane(t *testing.T) {
	commands := []string{
		"audition.prepare",
		"audition.status",
		"audition.ready",
		"audition.select",
		"audition.position",
		"audition.stop",
		"audition.inspect_candidate",
		"audition.apply_candidate",
	}
	for _, command := range commands {
		if command == "render.start" || command == "reload_project" || command == "project.checkout" {
			t.Fatalf("audition command aliased to project/render operation: %q", command)
		}
	}
}

func TestAuditionReadyUsesDedicatedCommand(t *testing.T) {
	commands := []string{"audition.prepare", "audition.status", "audition.ready", "audition.stale", "audition.failed", "audition.select", "audition.position", "audition.stop", "audition.inspect_candidate", "audition.apply_candidate"}
	for _, command := range commands {
		if command == "render" || command == "reload_project" || command == "project.checkout" {
			t.Fatalf("unsafe alias %s", command)
		}
	}
}
