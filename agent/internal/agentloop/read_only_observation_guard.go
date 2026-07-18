package agentloop

import (
	"strings"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

func messageLoopMutationBarrierActive(state *runState) bool {
	return state != nil && messageLoopReadOnlyObservationRequest(state.input.UserText)
}

func messageLoopApplyReadOnlyMutationBarrier(state *runState) {
	if !messageLoopMutationBarrierActive(state) {
		return
	}
	state.executionMemory.PendingMixTickCandidate = nil
	state.executionMemory.PendingMixTreatment = nil
}

func messageLoopReadOnlyObservationRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if messageLoopTextHasAny(text,
		"read only", "read-only", "readonly", "observe only", "observation only", "analysis only", "analyze only", "analyse only",
		"no changes", "no change", "do not modify", "don't modify", "do not execute", "don't execute",
		"do not apply", "don't apply", "without modifying",
		"\u53ea\u8bfb", "\u53ea\u770b", "\u53ea\u89c2\u5bdf", "\u53ea\u5206\u6790", "\u53ea\u68c0\u67e5",
		"\u4e0d\u8981\u6267\u884c", "\u4e0d\u6267\u884c", "\u4e0d\u8981\u4fee\u6539", "\u4e0d\u4fee\u6539",
		"\u4e0d\u505a\u4efb\u4f55\u6539\u52a8", "\u4e0d\u8981\u505a\u4efb\u4f55\u6539\u52a8",
		"\u4e0d\u6539\u5de5\u7a0b", "\u4e0d\u8981\u6539\u5de5\u7a0b", "\u4e0d\u8981\u6539", "\u4e0d\u8981\u52a8",
		"\u4e0d\u5199\u5165", "\u4e0d\u8981\u5199\u5165", "\u4e0d\u5e94\u7528", "\u4e0d\u8981\u5e94\u7528",
		"\u4e0d\u52a0\u8f7d\u63d2\u4ef6", "\u4e0d\u8981\u52a0\u8f7d\u63d2\u4ef6",
	) {
		return true
	}
	hasObserveIntent := messageLoopTextHasAny(text,
		"\u89c2\u5bdf", "\u89c2\u5bdf\u4e00\u4e0b", "\u770b\u4e00\u4e0b", "\u770b\u4e0b", "\u770b\u4e00\u770b", "\u770b\u770b", "\u67e5\u770b", "\u67e5\u770b\u4e00\u4e0b",
		"\u5e2e\u6211\u770b", "\u5e2e\u6211\u770b\u770b", "\u5e2e\u5fd9\u770b", "\u7ed9\u6211\u770b",
		"\u5206\u6790", "\u5206\u6790\u4e00\u4e0b", "\u68c0\u67e5", "\u68c0\u67e5\u4e00\u4e0b", "\u8bca\u65ad", "\u8bc4\u4f30",
		"\u542c\u4e00\u4e0b", "\u542c\u542c",
	)
	if !hasObserveIntent {
		return false
	}
	hasAudioSubject := messageLoopAudioObservationRequest(text) || messageLoopNaturalMixRequest(text) || messageLoopTextHasAny(text,
		"mix", "audio", "acoustic", "stereo", "phase", "correlation", "spectrum", "spectral", "frequency", "band energy", "waveform", "envelope", "loudness", "rms", "peak",
		"\u6df7\u97f3", "\u97f3\u9891", "\u58f0\u5b66", "\u58f0\u50cf", "\u58f0\u76f8", "\u58f0\u573a", "\u76f8\u4f4d", "\u76f8\u5173\u5ea6",
		"\u9891\u6bb5", "\u9891\u8c31", "\u4f4e\u9891", "\u4e2d\u9891", "\u9ad8\u9891", "\u6ce2\u5f62", "\u5305\u7edc", "\u54cd\u5ea6", "\u5cf0\u503c",
	)
	if !hasAudioSubject {
		return false
	}
	return !messageLoopReadOnlyObservationHasActionIntent(text)
}

func messageLoopReadOnlyObservationHasActionIntent(text string) bool {
	return messageLoopTextHasAny(text,
		"apply", "execute", "do it", "go ahead", "modify", "change", "adjust", "fix", "load", "write", "prepare", "candidate", "treatment", "plugin", "eq", "compressor", "reverb", "delay", "low cut", "set parameter", "mix this", "mix it",
		"\u5e2e\u6211\u6df7", "\u6df7\u4e00\u4e0b", "\u5904\u7406\u4e00\u4e0b", "\u8c03\u4e00\u4e0b", "\u8c03\u6574", "\u4fee\u4e00\u4e0b", "\u6539\u4e00\u4e0b",
		"\u600e\u4e48\u8c03", "\u600e\u4e48\u5904\u7406", "\u51c6\u5907", "\u5019\u9009", "\u63d2\u4ef6", "\u5747\u8861", "\u4f4e\u5207",
		"\u6267\u884c", "\u5e94\u7528", "\u52a0\u8f7d", "\u6302\u8f7d", "\u5199\u53c2\u6570", "\u6539\u53c2\u6570", "\u6821\u51c6", "\u8865\u507f",
	)
}

func messageLoopReadOnlyAllowedTool(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	if strings.HasPrefix(name, "get_") || strings.HasPrefix(name, "list_") || strings.Contains(name, ".list") || strings.Contains(name, ".read") || strings.Contains(name, "project.state") || strings.Contains(name, "get_audio_settings") {
		return true
	}
	switch name {
	case "project.state", "get_project_state", "track.list",
		"project.get_audio_settings", "project.validate_audio_settings_change", "project.import_preflight", "media.inspect_files",
		"mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation", "mix.read", "mix_read", "mix.derive", "mix_derive":
		return true
	default:
		return false
	}
}

func messageLoopReadOnlyGuardIssue(call planner.ToolCall) string {
	if messageLoopReadOnlyAllowedTool(call) {
		return ""
	}
	return "read-only acoustic observation allows only mix.observe, mix.read, mix.derive, and read/list/project-state tools; it must not propose/apply mix ticks, write track gain/pan, prepare plugins, or mutate the project"
}

func messageLoopReadOnlyFinalReply(state *runState, reply string) string {
	messageLoopApplyReadOnlyMutationBarrier(state)
	reply = messageLoopStripMixTreatmentPendingMarkup(strings.TrimSpace(reply))
	reply = messageLoopStripReadOnlyExecutionQuestions(reply)
	if reply == "" {
		return "\u672c\u6b21\u662f\u53ea\u8bfb\u89c2\u5bdf\uff0c\u672a\u751f\u6210\u5f85\u6267\u884c\u5019\u9009\uff0c\u4e5f\u6ca1\u6709\u4fee\u6539\u5de5\u7a0b\u3002"
	}
	if messageLoopReadOnlyFinalNoticePresent(reply) {
		return reply
	}
	return reply + "\n\n\u672c\u6b21\u662f\u53ea\u8bfb\u89c2\u5bdf\uff0c\u672a\u751f\u6210\u5f85\u6267\u884c\u5019\u9009\uff0c\u4e5f\u6ca1\u6709\u4fee\u6539\u5de5\u7a0b\u3002"
}

func messageLoopStripReadOnlyExecutionQuestions(reply string) string {
	lines := strings.Split(strings.TrimSpace(reply), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if lower == "" {
			out = append(out, line)
			continue
		}
		if messageLoopTextHasAny(lower,
			"should i execute", "should i apply", "should i continue", "want me to execute", "want me to continue", "shall i execute", "shall i apply",
			"\u8981\u6211\u6267\u884c", "\u8981\u6211\u7ee7\u7eed\u6267\u884c", "\u9700\u8981\u6211\u6267\u884c", "\u9700\u8981\u6211\u7ee7\u7eed\u6267\u884c", "\u73b0\u5728\u6267\u884c",
		) {
			continue
		}
		if strings.Contains(lower, "?") && messageLoopTextHasAny(lower, "execute", "apply", "continue") {
			continue
		}
		if strings.Contains(lower, "?") &&
			messageLoopTextHasAny(lower, "should i", "shall i", "want me to", "would you like me", "\u8981\u6211", "\u9700\u8981\u6211") &&
			messageLoopTextHasAny(lower, "db", "lower", "raise", "move", "pan", "gain", "volume", "set", "\u5206\u8d1d", "\u964d\u4f4e", "\u63d0\u9ad8", "\u79fb", "\u58f0\u50cf", "\u589e\u76ca", "\u97f3\u91cf") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func messageLoopReadOnlyFinalNoticePresent(reply string) bool {
	text := strings.ToLower(strings.TrimSpace(reply))
	return messageLoopTextHasAny(text,
		"no pending", "no changes", "not modified", "did not modify", "read-only",
		"\u672a\u751f\u6210\u5f85\u6267\u884c", "\u6ca1\u6709\u4fee\u6539", "\u672a\u4fee\u6539", "\u53ea\u8bfb",
	)
}
