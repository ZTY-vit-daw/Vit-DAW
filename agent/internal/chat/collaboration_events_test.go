package chat

import (
	"testing"

	"vit-daw-agent/internal/harness"
)

func TestCollaborationCreationProjectsTrajectoryEvents(t *testing.T) {
	server := New(nil, nil, nil)
	server.emitInvokeCollaborationEvents(harness.InvokeRequest{Tool: "version.branch_create", GoalID: "goal-1", RunID: "run-1", Context: map[string]any{"conversation_id": "conversation-1"}}, harness.InvokeResponse{Status: "ok", CommandName: "version_branch_create", Result: map[string]any{"branch": "vocal-natural", "source_commit_id": "commit-1", "activated": false}})
	server.emitInvokeCollaborationEvents(harness.InvokeRequest{Tool: "version.worktree_create", GoalID: "goal-1", RunID: "run-1", Context: map[string]any{"conversation_id": "conversation-1"}}, harness.InvokeResponse{Status: "ok", CommandName: "version_worktree_create", Result: map[string]any{"name": "drums-transient", "project_file_path": `D:\Worktrees\drums.vit`, "origin_commit_id": "commit-1"}})
	events, _ := server.agentEventsSince("conversation-1", 0, 20)
	foundBranch, foundWorktree := false, false
	for _, event := range events {
		if event.Type == "trajectory.branch.created" {
			foundBranch = true
		}
		if event.Type == "trajectory.worktree.created" {
			foundWorktree = true
		}
	}
	if !foundBranch || !foundWorktree {
		t.Fatalf("events=%+v", events)
	}
}

func TestCollaborationAuditionPrepareBindsFreeStateSession(t *testing.T) {
	server := New(nil, nil, nil)
	loop := boundAdoptionLoopForTest(t)
	loop.AuditionSessionID = ""
	loop.AuditionSessionSnapshot = nil
	server.storeFreeStateLoop(loop)
	request := harness.InvokeRequest{
		Tool: "collaboration.audition_prepare", GoalID: loop.GoalID, RunID: loop.RunID,
		Context: map[string]any{"conversation_id": loop.ConversationID},
	}
	response := harness.InvokeResponse{
		Status: "ok", CommandName: "collaboration_audition_prepare",
		Result: map[string]any{"session": map[string]any{
			"session_id": "cross-worktree-session", "conversation_id": loop.ConversationID,
			"status": "ready", "project_revision": "rev-7",
			"candidates": []any{
				map[string]any{"id": "candidate-a", "status": "ready", "source_kind": "audio_file", "source_ref": "a.wav", "preview_ref": "audio-buffer:a"},
				map[string]any{"id": "candidate-b", "status": "ready", "source_kind": "audio_file", "source_ref": "b.wav", "preview_ref": "audio-buffer:b"},
			},
		}},
	}
	server.emitInvokeCollaborationEvents(request, response)
	stored, ok := server.freeStateLoop(loop.ConversationID)
	if !ok || stored.AuditionSessionID != "cross-worktree-session" || firstStringFromMap(stored.AuditionSessionSnapshot, "status") != "ready" {
		t.Fatalf("stored=%+v ok=%v", stored, ok)
	}
	events, _ := server.agentEventsSince(loop.ConversationID, 0, 20)
	foundReady := false
	for _, event := range events {
		if event.Type == "audition.ready" {
			foundReady = true
		}
	}
	if !foundReady {
		t.Fatalf("events=%+v", events)
	}
}
