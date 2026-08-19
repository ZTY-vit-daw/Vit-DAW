package kernel

import (
	"context"
	"fmt"
)

// AuditionCandidate is the protocol shape for one Kernel-owned preview.
type AuditionCandidate struct {
	ID              string  `json:"id"`
	Label           string  `json:"label,omitempty"`
	SourceKind      string  `json:"source_kind,omitempty"`
	SourceRef       string  `json:"source_ref"`
	PreviewRef      string  `json:"preview_ref,omitempty"`
	CheckpointRef   string  `json:"checkpoint_ref,omitempty"`
	CommitID        string  `json:"commit_id,omitempty"`
	BranchRef       string  `json:"branch_ref,omitempty"`
	WorktreeRef     string  `json:"worktree_ref,omitempty"`
	ProjectUUID     string  `json:"project_uuid,omitempty"`
	ProjectRevision string  `json:"project_revision,omitempty"`
	RenderRevision  string  `json:"render_revision,omitempty"`
	PreviewRevision string  `json:"preview_revision,omitempty"`
	Scope           string  `json:"scope,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	SampleRate      float64 `json:"sample_rate,omitempty"`
	ChannelCount    int     `json:"channel_count,omitempty"`
}

// AuditionSessionRequest creates a session spanning the Active Project Plane
// and a separate Audition Preview Plane.
type AuditionSessionRequest struct {
	ConversationID        string              `json:"conversation_id,omitempty"`
	SessionID             string              `json:"session_id"`
	Scope                 string              `json:"scope"`
	ActiveProjectRef      string              `json:"active_project_ref"`
	ActiveProjectRevision string              `json:"active_project_revision"`
	TimelineRevision      string              `json:"timeline_revision"`
	Candidates            []AuditionCandidate `json:"candidates"`
}

func auditionPrepareArgs(request AuditionSessionRequest) (map[string]any, error) {
	if len(request.Candidates) != 2 {
		return nil, fmt.Errorf("audition.prepare requires exactly two candidates")
	}
	return map[string]any{
		"conversation_id":         request.ConversationID,
		"session_id":              request.SessionID,
		"scope":                   request.Scope,
		"active_project_ref":      request.ActiveProjectRef,
		"active_project_revision": request.ActiveProjectRevision,
		"timeline_revision":       request.TimelineRevision,
		"candidate_a":             request.Candidates[0],
		"candidate_b":             request.Candidates[1],
	}, nil
}

func (c *Client) AuditionPrepare(ctx context.Context, request AuditionSessionRequest) (*VSPCommandResult, error) {
	args, err := auditionPrepareArgs(request)
	if err != nil {
		return nil, err
	}
	return c.SendVSPCommand(ctx, "audition.prepare", args)
}
func (c *Client) AuditionStatus(ctx context.Context, sessionID string) (*VSPCommandResult, error) {
	return c.SendVSPCommand(ctx, "audition.status", map[string]any{"session_id": sessionID})
}

func (c *Client) AuditionReady(ctx context.Context, sessionID, candidateID, previewRef string) (*VSPCommandResult, error) {
	return c.SendVSPCommand(ctx, "audition.ready", map[string]any{
		"session_id": sessionID, "candidate_id": candidateID, "preview_ref": previewRef,
	})
}

func (c *Client) AuditionSelect(ctx context.Context, sessionID, candidateID string) (*VSPCommandResult, error) {
	return c.SendVSPCommand(ctx, "audition.select", map[string]any{
		"session_id":   sessionID,
		"candidate_id": candidateID,
	})
}

func (c *Client) AuditionPosition(ctx context.Context, sessionID string, positionSeconds *float64, isPlaying *bool) (*VSPCommandResult, error) {
	args := map[string]any{"session_id": sessionID}
	if positionSeconds != nil {
		args["position_seconds"] = *positionSeconds
	}
	if isPlaying != nil {
		args["is_playing"] = *isPlaying
	}
	return c.SendVSPCommand(ctx, "audition.position", args)
}

func (c *Client) AuditionStop(ctx context.Context, sessionID string) (*VSPCommandResult, error) {
	return c.SendVSPCommand(ctx, "audition.stop", map[string]any{"session_id": sessionID})
}

func (c *Client) AuditionInspectCandidate(ctx context.Context, sessionID, candidateID string) (*VSPCommandResult, error) {
	return c.SendVSPCommand(ctx, "audition.inspect_candidate", map[string]any{
		"session_id": sessionID, "candidate_id": candidateID,
	})
}

func (c *Client) AuditionApplyCandidate(ctx context.Context, sessionID, candidateID string) (*VSPCommandResult, error) {
	return c.SendVSPCommand(ctx, "audition.apply_candidate", map[string]any{
		"session_id": sessionID, "candidate_id": candidateID,
	})
}

func (c *Client) AuditionStale(ctx context.Context, sessionID string) (*VSPCommandResult, error) {
	return c.SendVSPCommand(ctx, "audition.stale", map[string]any{"session_id": sessionID})
}

func (c *Client) AuditionFailed(ctx context.Context, sessionID string) (*VSPCommandResult, error) {
	return c.SendVSPCommand(ctx, "audition.failed", map[string]any{"session_id": sessionID})
}
