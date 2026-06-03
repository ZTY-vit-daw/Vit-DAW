package rollback

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/workspace"
)

type Dependencies interface {
	JournalGet(targetID string) (journal.Action, bool)
	JournalMarkRollback(targetActionID, rollbackActionID, state string)
	KernelSendCommand(ctx context.Context, command map[string]any) (map[string]any, string, error)
	WorkspaceContext(cmd map[string]any) workspace.Context
	RefreshShadow(ctx context.Context, reason string)
}

func Action(ctx context.Context, deps Dependencies, cmd map[string]any) (map[string]any, error) {
	if deps == nil {
		return nil, fmt.Errorf("rollback dependencies are nil")
	}
	targetID := firstString(cmd, "target_action_id", "agent_action_id", "action_id")
	if targetID == "" {
		return nil, fmt.Errorf("target_action_id is required")
	}
	action, ok := deps.JournalGet(targetID)
	if !ok {
		return nil, fmt.Errorf("journal action not found: %s", targetID)
	}
	switch action.Domain {
	case "daw", "":
		reply, _, err := deps.KernelSendCommand(ctx, map[string]any{
			"cmd":              "undo",
			"target_action_id": targetID,
		})
		if err != nil {
			deps.JournalMarkRollback(targetID, "", "failed")
			return nil, err
		}
		rollbackID := firstString(reply, "agent_action_id")
		deps.JournalMarkRollback(targetID, rollbackID, "succeeded")
		deps.RefreshShadow(ctx, "rollback_action")
		return map[string]any{"target_action_id": targetID, "domain": action.Domain, "reply": reply}, nil
	case "workspace":
		if action.CommandName != "workspace_apply_edit" {
			return nil, fmt.Errorf("workspace rollback is only automatic for workspace.apply_edit")
		}
		reverseOld := firstString(action.Result, "reverse_old_text")
		reverseNew := firstString(action.Result, "reverse_new_text")
		path := firstString(action.Result, "path")
		if reverseOld == "" || path == "" {
			return nil, fmt.Errorf("workspace reverse patch data is missing")
		}
		result, err := workspace.ApplyEdit(deps.WorkspaceContext(action.Command), map[string]any{
			"path":     path,
			"old_text": reverseOld,
			"new_text": reverseNew,
		})
		if err != nil {
			deps.JournalMarkRollback(targetID, "", "failed")
			return nil, err
		}
		deps.JournalMarkRollback(targetID, firstString(cmd, "agent_action_id"), "succeeded")
		return map[string]any{"target_action_id": targetID, "domain": action.Domain, "result": result}, nil
	default:
		return nil, fmt.Errorf("rollback is not automatic for domain %q", action.Domain)
	}
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok && !isEmptyValue(v) {
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s == "" || s == "<nil>"
}
