package chat

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"vit-daw-agent/internal/artifacts"
)

func artifactSummariesFromExecuted(records []map[string]any) []artifacts.Summary {
	var out []artifacts.Summary
	for _, record := range records {
		appendArtifactSummariesFromValue(&out, record["artifact"])
		appendArtifactSummariesFromValue(&out, record["artifacts"])
		if result, ok := record["result"]; ok {
			appendArtifactSummariesFromValue(&out, result)
		}
	}
	return mergeArtifactSummaries(nil, out)
}

func appendArtifactSummariesFromValue(out *[]artifacts.Summary, value any) {
	switch v := value.(type) {
	case nil:
		return
	case artifacts.Summary:
		*out = append(*out, v)
	case artifacts.Artifact:
		*out = append(*out, v.CompactSummary())
	case []artifacts.Summary:
		*out = append(*out, v...)
	case []artifacts.Artifact:
		for _, item := range v {
			*out = append(*out, item.CompactSummary())
		}
	case []map[string]any:
		for _, item := range v {
			appendArtifactSummariesFromValue(out, item)
		}
	case []any:
		for _, item := range v {
			appendArtifactSummariesFromValue(out, item)
		}
	case map[string]any:
		if summary, ok := artifactSummaryFromMap(v); ok {
			*out = append(*out, summary)
			return
		}
		appendArtifactSummariesFromValue(out, v["artifact"])
		appendArtifactSummariesFromValue(out, v["artifacts"])
	}
}

func artifactSummaryFromMap(row map[string]any) (artifacts.Summary, bool) {
	id := artifactPromotionString(row["id"])
	if id == "" {
		return artifacts.Summary{}, false
	}
	metadata, _ := row["metadata"].(map[string]any)
	return artifacts.Summary{
		ID:              id,
		Kind:            artifactPromotionString(row["kind"]),
		Title:           artifactPromotionString(row["title"]),
		Source:          artifactPromotionString(row["source"]),
		Path:            artifactPromotionString(row["path"]),
		URL:             artifactPromotionString(row["url"]),
		MIME:            artifactPromotionString(row["mime"]),
		SizeBytes:       artifactPromotionInt64(row["size_bytes"]),
		Status:          artifactPromotionString(row["status"]),
		Summary:         artifactPromotionString(row["summary"]),
		Metadata:        metadata,
		CreatedAt:       artifactPromotionString(row["created_at"]),
		ConversationID:  artifactPromotionString(row["conversation_id"]),
		GoalID:          artifactPromotionString(row["goal_id"]),
		RunID:           artifactPromotionString(row["run_id"]),
		ProjectPath:     artifactPromotionString(row["project_path"]),
		RootProjectPath: artifactPromotionString(row["root_project_path"]),
		ActiveWorktree:  artifactPromotionString(row["active_worktree"]),
		ActiveBranch:    artifactPromotionString(row["active_branch"]),
		ActiveNodeID:    artifactPromotionString(row["active_node_id"]),
		HistoryScopeKey: artifactPromotionString(row["history_scope_key"]),
		MediaScopeKey:   artifactPromotionString(row["media_scope_key"]),
	}, true
}

func artifactPromotionString(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func artifactPromotionInt64(value any) int64 {
	switch n := value.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case json.Number:
		i, _ := strconv.ParseInt(n.String(), 10, 64)
		return i
	default:
		i, _ := strconv.ParseInt(artifactPromotionString(value), 10, 64)
		return i
	}
}
