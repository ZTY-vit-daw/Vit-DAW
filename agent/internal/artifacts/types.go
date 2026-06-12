package artifacts

import "time"

const SchemaVersion = "vit_artifact.v1"

type Artifact struct {
	ID              string         `json:"id"`
	Kind            string         `json:"kind"`
	Title           string         `json:"title,omitempty"`
	Source          string         `json:"source,omitempty"`
	Path            string         `json:"path,omitempty"`
	URL             string         `json:"url,omitempty"`
	MIME            string         `json:"mime,omitempty"`
	SizeBytes       int64          `json:"size_bytes,omitempty"`
	Status          string         `json:"status"`
	Summary         string         `json:"summary,omitempty"`
	Text            string         `json:"text,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       string         `json:"created_at,omitempty"`
	ConversationID  string         `json:"conversation_id,omitempty"`
	GoalID          string         `json:"goal_id,omitempty"`
	RunID           string         `json:"run_id,omitempty"`
	ProjectPath     string         `json:"project_path,omitempty"`
	RootProjectPath string         `json:"root_project_path,omitempty"`
	ActiveWorktree  string         `json:"active_worktree,omitempty"`
	ActiveBranch    string         `json:"active_branch,omitempty"`
	ActiveNodeID    string         `json:"active_node_id,omitempty"`
	HistoryScopeKey string         `json:"history_scope_key,omitempty"`
	MediaScopeKey   string         `json:"media_scope_key,omitempty"`
}

type Summary struct {
	ID              string         `json:"id"`
	Kind            string         `json:"kind"`
	Title           string         `json:"title,omitempty"`
	Source          string         `json:"source,omitempty"`
	Path            string         `json:"path,omitempty"`
	URL             string         `json:"url,omitempty"`
	MIME            string         `json:"mime,omitempty"`
	SizeBytes       int64          `json:"size_bytes,omitempty"`
	Status          string         `json:"status"`
	Summary         string         `json:"summary,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       string         `json:"created_at,omitempty"`
	ConversationID  string         `json:"conversation_id,omitempty"`
	GoalID          string         `json:"goal_id,omitempty"`
	RunID           string         `json:"run_id,omitempty"`
	ProjectPath     string         `json:"project_path,omitempty"`
	RootProjectPath string         `json:"root_project_path,omitempty"`
	ActiveWorktree  string         `json:"active_worktree,omitempty"`
	ActiveBranch    string         `json:"active_branch,omitempty"`
	ActiveNodeID    string         `json:"active_node_id,omitempty"`
	HistoryScopeKey string         `json:"history_scope_key,omitempty"`
	MediaScopeKey   string         `json:"media_scope_key,omitempty"`
}

func (a Artifact) CompactSummary() Summary {
	return Summary{
		ID:              a.ID,
		Kind:            a.Kind,
		Title:           a.Title,
		Source:          a.Source,
		Path:            a.Path,
		URL:             a.URL,
		MIME:            a.MIME,
		SizeBytes:       a.SizeBytes,
		Status:          a.Status,
		Summary:         a.Summary,
		Metadata:        cloneMap(a.Metadata),
		CreatedAt:       a.CreatedAt,
		ConversationID:  a.ConversationID,
		GoalID:          a.GoalID,
		RunID:           a.RunID,
		ProjectPath:     a.ProjectPath,
		RootProjectPath: a.RootProjectPath,
		ActiveWorktree:  a.ActiveWorktree,
		ActiveBranch:    a.ActiveBranch,
		ActiveNodeID:    a.ActiveNodeID,
		HistoryScopeKey: a.HistoryScopeKey,
		MediaScopeKey:   a.MediaScopeKey,
	}
}

func New(now time.Time) Artifact {
	return Artifact{
		Status:    "ready",
		CreatedAt: now.UTC().Format(time.RFC3339Nano),
		Metadata:  map[string]any{},
	}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
