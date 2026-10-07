package agentloop

// observe_presentation.go — OPT-IMPL-2 P2（2026-10-07）：观察问答路由最终回复的
// 可选 presentation 块。设计锚：docs/OBSERVE_OUTPUT_LAYERING_V1_DESIGN.md §4.1/§4.3。
//
// schema（设计 §4.3 定版，webui fail-open 消费）：
//
//	presentation := {summary: ≤3 行结论, evidence_entries: [{text, source, ref?}], detail_mode: "layered"}
//
// 保守原则（沿 P1 §4.2-1 宁漏勿滥）：summary 提取失败（无自洽首段/超 3 行）或
// evidence 为空 → 不挂块，消息回落 P1 谓词呈现。summary 是 LLM 按指令产出的
// 结论首段（chat 侧 observe_reply_shape 动态段；不碰 IMPL-B 字节锁定的骨架面），
// 此处只做整段提取，禁止句中机械截断。

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	observePresentationDetailMode      = "layered"
	observePresentationMaxSummaryLines = 3
	observePresentationMaxEntries      = 8
	observePresentationEntryTextMax    = 200
)

// ObservePresentation is the optional structured reply block attached to
// read-only observation turns (design §4.3).
type ObservePresentation struct {
	Summary         string                 `json:"summary"`
	EvidenceEntries []ObserveEvidenceEntry `json:"evidence_entries"`
	DetailMode      string                 `json:"detail_mode"`
}

// ObserveEvidenceEntry is one observation citation: a one-line observation,
// its source tool/projection family, and an optional observation id / ref.
type ObserveEvidenceEntry struct {
	Text   string `json:"text"`
	Source string `json:"source"`
	Ref    string `json:"ref,omitempty"`
}

// ReadOnlyObservationIntent reports whether the user text reads as a
// read-only observation request. Exported for the chat-side reply-shape
// prompt gate; same heuristic family as the message-loop read-only guard.
func ReadOnlyObservationIntent(userText string) bool {
	return messageLoopReadOnlyObservationRequest(userText)
}

// observePresentationFromTurn assembles the block for a read-only
// observation turn. Nil whenever a conservative quality gate fails.
func observePresentationFromTurn(state *runState, cleanReply string) *ObservePresentation {
	if state == nil || !messageLoopMutationBarrierActive(state) {
		return nil
	}
	reply := strings.TrimSpace(cleanReply)
	if reply == "" {
		return nil
	}
	summary := observePresentationSummary(reply)
	if summary == "" {
		return nil
	}
	entries := observePresentationEvidence(state)
	if len(entries) == 0 {
		return nil
	}
	return &ObservePresentation{
		Summary:         summary,
		EvidenceEntries: entries,
		DetailMode:      observePresentationDetailMode,
	}
}

// observePresentationSummary takes the reply's first paragraph when it is a
// self-contained block of at most three non-empty lines. Overlong or empty
// leads degrade to "" (P1 path) — never mid-paragraph truncation.
func observePresentationSummary(reply string) string {
	paragraph := firstParagraph(reply)
	if paragraph == "" {
		return ""
	}
	if nonEmptyLineCount(paragraph) > observePresentationMaxSummaryLines {
		return ""
	}
	return paragraph
}

func firstParagraph(reply string) string {
	for _, block := range strings.Split(strings.TrimSpace(reply), "\n\n") {
		block = strings.TrimSpace(block)
		if block != "" {
			return block
		}
	}
	return ""
}

func nonEmptyLineCount(block string) int {
	count := 0
	for _, line := range strings.Split(block, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// observePresentationEvidence derives the evidence layer from the turn's
// observation-tool records (the same allowlist family as the read-only guard).
func observePresentationEvidence(state *runState) []ObserveEvidenceEntry {
	entries := make([]ObserveEvidenceEntry, 0, 4)
	for _, record := range state.executed {
		if len(entries) >= observePresentationMaxEntries {
			break
		}
		tool := firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))
		name := strings.ToLower(strings.TrimSpace(tool))
		if !observePresentationEvidenceTool(name) {
			continue
		}
		if status := strings.TrimSpace(messageLoopText(record["status"])); status != "" &&
			!strings.EqualFold(status, "ok") && !strings.EqualFold(status, "success") {
			continue
		}
		result, _ := record["result"].(map[string]any)
		text := observeEvidenceText(record, result)
		if strings.TrimSpace(text) == "" {
			continue
		}
		entry := ObserveEvidenceEntry{Text: text, Source: tool}
		if result != nil {
			if id := strings.TrimSpace(messageLoopText(result["observation_id"])); id != "" {
				entry.Ref = id
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

func observePresentationEvidenceTool(name string) bool {
	switch name {
	case "mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation",
		"mix.read", "mix_read", "mix.derive", "mix_derive", "mix.report", "mix_report",
		"ccb.observation_catalog", "ccb_observation_catalog",
		"ccb.observation_request", "ccb_observation_request",
		"ref.query", "ref_query", "ref.diff", "ref_diff":
		return true
	default:
		return false
	}
}

// observeEvidenceText flattens the record's existing compact summary (the same
// extractors the loop uses for tool-result notes) into one citation line; any
// other observation-family result map falls back to a generic key/value
// flatten so every usable observation record yields a deterministic line.
func observeEvidenceText(record, result map[string]any) string {
	if len(result) > 0 {
		if summary := messageLoopCCBObservationSummary(record, result); len(summary) > 0 {
			return flattenEvidenceSummary(summary)
		}
		if summary := mixObservationPromptSummary(result); len(summary) > 0 {
			return flattenEvidenceSummary(summary)
		}
		if flat := flattenEvidenceSummary(result); flat != "" {
			return flat
		}
	}
	if preview := strings.TrimSpace(messageLoopText(record["preview"])); preview != "" {
		return clipEvidenceText(preview)
	}
	return ""
}

func flattenEvidenceSummary(summary map[string]any) string {
	keys := make([]string, 0, len(summary))
	for key := range summary {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := summary[key]
		// 只取标量：嵌套 map/slice 的 Go 字符串化（"map[...]"）不是可读引用行。
		switch value.(type) {
		case string, bool, int, int64, float64, float32, int32, json.Number:
		default:
			continue
		}
		text := strings.TrimSpace(messageLoopText(value))
		if text == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", key, text))
	}
	if len(parts) == 0 {
		return ""
	}
	return clipEvidenceText(strings.Join(parts, "; "))
}

func clipEvidenceText(text string) string {
	if len(text) <= observePresentationEntryTextMax {
		return text
	}
	return text[:observePresentationEntryTextMax] + "…"
}
