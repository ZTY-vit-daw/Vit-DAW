package chat

import "vit-daw-agent/internal/conversation"

func synthesizeLocalDAWCommands(userText string, requestContext map[string]any) []map[string]any {
	return conversation.SynthesizeLocalDAWCommands(userText, requestContext)
}

// synthesizeClipRangeSplitCommandsPreModel exposes the boxed-range split gate
// for the pre-LLM takeover in server.go: when it returns a plan, the two-cut
// proposal is deterministic (end cut before start cut) and must not race the
// model envelope.
func synthesizeClipRangeSplitCommandsPreModel(userText string, requestContext map[string]any) []map[string]any {
	return conversation.SynthesizeClipRangeSplitCommands(userText, requestContext)
}
