package chat

import (
	"vit-daw-agent/internal/policy"
	"vit-daw-agent/internal/presenter"
)

func confirmationReply(decisions []policy.Decision) string {
	return presenter.ConfirmationReply(decisions)
}

func friendlyExecutionError(err error) string {
	return presenter.FriendlyExecutionError(err)
}

func sanitizeUserReply(reply string, state map[string]any, userText string) string {
	return presenter.SanitizeUserReply(reply, state, userText)
}

func executedReply(before, after map[string]any, decisions []policy.Decision, replies []map[string]any) string {
	return presenter.ExecutedReply(before, after, decisions, replies)
}

func mapRowsValue(value any) []map[string]any {
	return presenter.MapRowsValue(value)
}

func firstBool(row map[string]any, keys ...string) (bool, bool) {
	return presenter.FirstBool(row, keys...)
}
