package chat

import "vit-daw-agent/internal/conversation"

func synthesizeLocalDAWCommands(userText string, requestContext map[string]any) []map[string]any {
	return conversation.SynthesizeLocalDAWCommands(userText, requestContext)
}
