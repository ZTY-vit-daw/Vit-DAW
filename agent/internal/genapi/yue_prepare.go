package genapi

import (
	"fmt"
	"sort"
	"strings"
)

// yue_local request preparation (GEN-YUE-WIRE-1, audit H6 items 2-4).
//
// The harness requires a non-empty user prompt for every generation call,
// but the yue_local builder has no free-prompt channel: the ABC score +
// lyrics + English style tags are the input face. These two facts together
// meant the prompt the user was forced to write was silently discarded.
// YuePrepareRequest closes that gap deterministically (no LLM round-trip):
//   - style: explicit style_tags win; otherwise the raw user prompt text is
//     mapped verbatim into the style channel (translation to English tags is
//     the caller's runtime concern, disclosed in the actions list).
//   - instrumental: the style gains an "instrumental" tag and the ABC score's
//     Vocal voice is rested (equal-length rests, chords preserved) — REQUEST-
//     SIDE conditioning only. The YuE2 engine has no instrumental switch and
//     un-lyriced generation may still produce sung vocals (run_20261116_
//     000009 forensics: SheetSage2 readback found sustained Vocal melody in
//     the vocal-rested render); engine-side muting is NOT guaranteed.
//   - score: a YueScoreInput (student clip notes) converts to the dialect
//     score when no explicit ABC was provided.
//
// DispatchedSummary mirrors the exact body the builder will send so the
// harness can show the user what actually left the building.

// YuePrepareOptions carries the inputs beyond the normalized request.
type YuePrepareOptions struct {
	// UserPrompt is the raw user free-text prompt (pre-render, pre-auto
	// blocks). Empty = no prompt-derived style.
	UserPrompt string
	// Score converts to ABC when Request.ABC is empty. Nil = no score input.
	Score *YueScoreInput
	// StyleExtra appends extra style tags (e.g. engine-known descriptors)
	// after the derived or explicit style.
	StyleExtra []string
}

// YuePrepareResult is the mapped request plus its disclosure surface.
type YuePrepareResult struct {
	Request    GenerationRequest
	Dispatched map[string]any
	Actions    []string
}

// YueProtectedExtraKeys are the request fields a provider's fixed
// extra_params must never override: they carry the student's own score and
// style (audit H6 item 4). Engine knobs (cot/steps/output_format/seed/...)
// remain overridable.
var YueProtectedExtraKeys = []string{"abc", "style", "lyrics"}

// YuePrepareRequest applies the deterministic yue_local conditioning map.
func YuePrepareRequest(req GenerationRequest, opts YuePrepareOptions) (YuePrepareResult, error) {
	res := YuePrepareResult{Request: req}

	// 1) Score face: student notes -> dialect ABC when no explicit score.
	if strings.TrimSpace(res.Request.ABC) == "" && opts.Score != nil {
		abc, cerr := ConvertMidiToYueABC(*opts.Score)
		if cerr != nil {
			return res, cerr
		}
		res.Request.ABC = abc
		res.Actions = append(res.Actions, fmt.Sprintf("abc: converted %d student notes to the YuE2 ABC dialect (deterministic program conversion)", len(opts.Score.Notes)))
	}

	// 2) Style face: explicit tags win; otherwise the raw user prompt rides
	// the style channel verbatim (yue_local has no free-prompt channel).
	style := strings.TrimSpace(res.Request.StyleTags)
	if style == "" && strings.TrimSpace(opts.UserPrompt) != "" {
		style = strings.TrimSpace(opts.UserPrompt)
		res.Actions = append(res.Actions, "style: derived verbatim from the user prompt (yue_local has no free-prompt channel; translate to English tags upstream if needed)")
	}

	// 3) Instrumental conditioning: style tag + Vocal voice resting.
	if res.Request.Instrumental {
		if !yueStyleHasTag(style, "instrumental") {
			style = strings.TrimSpace(strings.TrimSpace(style) + ", instrumental")
		}
		res.Actions = append(res.Actions, "style: instrumental tag added (instrumental switch requested; yue_local has no instrumental parameter, engine-side muting not guaranteed — un-lyriced generation may still produce vocals)")
		if strings.TrimSpace(res.Request.ABC) != "" {
			rested, changed, rerr := RestYueVocalPart(res.Request.ABC)
			if rerr != nil {
				return res, fmt.Errorf("instrumental conditioning failed: %w", rerr)
			}
			if changed {
				res.Request.ABC = rested
				res.Actions = append(res.Actions, "abc: Vocal voice rested (equal-length rests, chords preserved) — request-side score conditioning; the engine may still sing (no instrumental switch, YUE-VOCAL-FORENSICS-1)")
			} else if strings.Contains(res.Request.ABC, "V: Vocal") {
				res.Actions = append(res.Actions, "abc: Vocal voice already silent (no sounding notes to rest; request-side conditioning only — the engine may still sing, YUE-VOCAL-FORENSICS-1)")
			}
		}
	}
	for _, extra := range opts.StyleExtra {
		tag := strings.TrimSpace(extra)
		if tag != "" && !yueStyleHasTag(style, tag) {
			style = strings.TrimSpace(style + ", " + tag)
		}
	}
	res.Request.StyleTags = style

	// 4) Version semantics: the local engine runs one song per request;
	// requesting count>1 cannot produce multiple candidates in one job.
	if res.Request.Count > 1 {
		res.Actions = append(res.Actions, "count: the local engine serves one candidate per job; run one job per version instead of count>1")
		res.Request.Count = 1
	}

	res.Dispatched = YueDispatchedSummary(res.Request, ProviderConfig{})
	return res, nil
}

func yueStyleHasTag(style, tag string) bool {
	for _, part := range strings.Split(style, ",") {
		if strings.EqualFold(strings.TrimSpace(part), tag) {
			return true
		}
	}
	return false
}

// YueDispatchedSummary builds the "what actually got sent" disclosure for a
// prepared yue_local request: the style string in full, score/lyrics sizes
// and head preview, the pinned engine baseline with effective extra_params,
// and the protected keys dropped from the provider's fixed extra_params.
func YueDispatchedSummary(req GenerationRequest, cfg ProviderConfig) map[string]any {
	abc := strings.TrimSpace(req.ABC)
	abcHead := abc
	if len([]rune(abcHead)) > 120 {
		abcHead = string([]rune(abcHead)[:120]) + "…"
	}
	effective := map[string]any{}
	var dropped []string
	for k, v := range cfg.ExtraParams {
		if yueKeyProtected(k) {
			dropped = append(dropped, k)
			continue
		}
		effective[k] = v
	}
	sort.Strings(dropped)
	cot, steps, format := any("full"), any(32), any("wav16")
	if v, ok := effective["cot"]; ok {
		cot = v
	}
	if v, ok := effective["steps"]; ok {
		steps = v
	}
	if v, ok := effective["output_format"]; ok {
		format = v
	}
	summary := map[string]any{
		"style":       req.StyleTags,
		"lyrics":      strings.TrimSpace(req.Lyrics) != "",
		"lyrics_chars": len(strings.TrimSpace(req.Lyrics)),
		"abc_chars":   len([]rune(abc)),
		"abc_head":    abcHead,
		"duration":    req.DurationSeconds,
		"cot":         cot,
		"steps":       steps,
		"output_format": format,
		"not_forwarded": []string{"prompt", "tempo_bpm", "musical_key", "title", "instrumental (as a switch)"},
	}
	keys := make([]string, 0, len(effective))
	for k := range effective {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	summary["extra_params_effective"] = keys
	if len(dropped) > 0 {
		summary["extra_params_dropped_protected"] = dropped
	}
	return summary
}

func yueKeyProtected(k string) bool {
	for _, p := range YueProtectedExtraKeys {
		if strings.EqualFold(strings.TrimSpace(k), p) {
			return true
		}
	}
	return false
}
