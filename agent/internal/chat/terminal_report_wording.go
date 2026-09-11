package chat

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"vit-daw-agent/internal/experiment"
)

// B6（2026-09-11 手测点 3 尝试 #2 用户质问三连）：原生微调（mix-tick / D1
// 域）终局用户交付治理，三类纪律集中在本文件。
//
//  1. 五要素：应用类终局正文必须让用户不看日志就知道——动的是哪条轨（用
//     户可辨名）、哪个参数、变更量与方向（含回读前后值）、针对的发现、去
//     DAW 哪里查看。
//  2. 术语黑名单：仓库内部代号（FAM/FS/D 阶段编号）与观察投影缩写（DAD/
//     DOM/MOM/TIM/TOM/FXM/COM/EPM/RLM/CCB 等）不得出现在用户面终局文本
//     （AGENT-W2 中文化纪律同族）。L1/L3 听感分层词汇是既有用户面措辞，
//     不在黑名单内。
//  3. 判定一致性：goal 已收口为完成态的终局不得再声明「人工判定仍待完
//     成」——可服务的判定入口（audition 卡 / waiting park）由结算评估的
//     user_judgment_pending 决定建立（F1 语义，audition_events 判定 POST
//     可达），应用轮静态模板不得预支这个承诺。B6 勘察选定路线：不要求判
//     定则终局文案禁用待判定表述（完成态路径剥除该承诺，可应答 park 的
//     「等待试听确认」措辞不受影响）。

// terminalInternalTermPattern matches the warehouse-internal codes that must
// never surface in user-facing terminal text: FAM family ids (FAM3-S1), free
// state tiers (FS0/FS2), D-phase ids (D1-S1/D2-1.5/D2-2), and the observation
// projection/coordination abbreviations. Uppercase-only with word boundaries:
// lowercase protocol tokens (stop_reason values such as d1_post_action_)
// never match.
var terminalInternalTermPattern = regexp.MustCompile(`\b(?:FAM\d+(?:-S\d+(?:\.\d+)?)?|FS\d+(?:-[0-9A-Za-z.]+)?|D\d+(?:-\d+(?:\.\d+)?)?|DAD|DOM|MOM|TIM|TOM|FXM|COM|EPM|RLM|CCB|PCA|VSP)\b`)

// stripInternalTerminalTerms removes internal codes from a user-facing
// terminal text and collapses the whitespace the removal leaves behind. The
// informative wording around the code always survives.
func stripInternalTerminalTerms(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	stripped := terminalInternalTermPattern.ReplaceAllString(text, "")
	stripped = collapseTerminalWhitespace(stripped)
	return strings.TrimSpace(stripped)
}

// terminalPendingJudgmentClaimPattern matches the legacy applied-template tail
// ("；声学实质性、目标响应与人工判定仍待完成。") down to a bare
// 「人工判定仍待…完成」 clause inside its own sentence, so a settled terminal
// stops promising a judgment entry it does not own.
var terminalPendingJudgmentClaimPattern = regexp.MustCompile(`[；;，,]?[^。；;]*人工判定[^。]*。?`)

// stripPendingHumanJudgmentClaims removes pending-human-judgment promises from
// a settled terminal reply. Answerable waiting parks (「等待试听确认」) never
// match: they are the servicable judgment entry itself, not a hollow claim.
func stripPendingHumanJudgmentClaims(text string) string {
	if !strings.Contains(text, "人工判定") {
		return text
	}
	stripped := terminalPendingJudgmentClaimPattern.ReplaceAllString(text, "")
	stripped = collapseTerminalWhitespace(stripped)
	return strings.TrimSpace(stripped)
}

// sanitizeSettledTerminalReply is the full guardrail for completed-form chain
// terminals: internal-term blacklist first, then the judgment-claim strip.
func sanitizeSettledTerminalReply(text string) string {
	return stripPendingHumanJudgmentClaims(stripInternalTerminalTerms(text))
}

// sanitizeSettledChainReply applies the settled guardrail on the scheduler
// delivery path and leaves a log line whenever it rewrites (B6 forensic
// observability: the rewrite is a user-facing wording change, not a drop).
func (s *Server) sanitizeSettledChainReply(current DurableContinuation, reply string) string {
	sanitized := sanitizeSettledTerminalReply(reply)
	if s != nil && s.logger != nil && sanitized != reply {
		s.logger.Info("[b6.wording] settled terminal reply sanitized conversation=%s goal=%s before_len=%d after_len=%d",
			current.ConversationID, current.GoalID, len([]rune(reply)), len([]rune(sanitized)))
	}
	return sanitized
}

// sanitizeWaitingParkChainReply applies the internal-term strip only: an
// answerable park's pending-judgment wording is a servicable entry, so the
// judgment-claim strip deliberately does not run here.
func (s *Server) sanitizeWaitingParkChainReply(current DurableContinuation, reply string) string {
	sanitized := stripInternalTerminalTerms(reply)
	if s != nil && s.logger != nil && sanitized != reply {
		s.logger.Info("[b6.wording] waiting-park terminal reply sanitized conversation=%s goal=%s before_len=%d after_len=%d",
			current.ConversationID, current.GoalID, len([]rune(reply)), len([]rune(sanitized)))
	}
	return sanitized
}

// collapseTerminalWhitespace trims the doubled punctuation/space runs a token
// strip can leave behind, keeping single CJK punctuation intact.
func collapseTerminalWhitespace(text string) string {
	if !strings.Contains(text, "  ") && !strings.Contains(text, "， ") && !strings.Contains(text, "。 ") && !strings.Contains(text, "： ") {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	space := false
	for _, r := range text {
		if r == ' ' || r == '\t' {
			space = true
			continue
		}
		if space {
			b.WriteRune(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// d1UserFacingWording is the per-domain user-facing vocabulary for the applied
// report: the parameter label, its display unit, where in the DAW the user
// verifies the move, and the receipt detail keys carrying the before/after
// readbacks. Mirrors the chat routing constants' domain set; the bounds
// authority stays in the experiment domain table.
type d1UserFacingWording struct {
	Parameter   string
	Unit        string
	WhereToLook string
	BeforeKey   string
	AfterKey    string
}

func d1UserFacingWordingFor(actionKind string) d1UserFacingWording {
	switch strings.TrimSpace(actionKind) {
	case d1PanKind, "track_pan_set":
		return d1UserFacingWording{Parameter: "声像（pan）", Unit: "", WhereToLook: "轨道通道条的声像（pan）位置",
			BeforeKey: "before_readback_pan", AfterKey: "actual_readback_pan"}
	case experiment.D1S1ActionKind:
		return d1UserFacingWording{Parameter: "音量", Unit: " dB", WhereToLook: "轨道通道条的音量推子/电平读数",
			BeforeKey: "before_readback_db", AfterKey: "actual_readback_db"}
	case d1StaticEQKind:
		return d1UserFacingWording{Parameter: "静态 EQ 频段增益", Unit: " dB", WhereToLook: "该轨 EQ 效果器对应频段的增益",
			BeforeKey: "before_readback_value", AfterKey: "actual_readback_value"}
	case d1BroadbandCompressionKind:
		return d1UserFacingWording{Parameter: "压缩器阈值", Unit: " dB", WhereToLook: "该轨压缩器效果的阈值",
			BeforeKey: "before_readback_value", AfterKey: "actual_readback_value"}
	case d1DeEsserKind:
		return d1UserFacingWording{Parameter: "齿音处理器阈值", Unit: " dB", WhereToLook: "该轨齿音处理器效果的阈值",
			BeforeKey: "before_readback_value", AfterKey: "actual_readback_value"}
	case d1TransientShaperKind:
		return d1UserFacingWording{Parameter: "瞬态处理器起振参数", Unit: "", WhereToLook: "该轨瞬态处理器效果的起振参数",
			BeforeKey: "before_readback_value", AfterKey: "actual_readback_value"}
	case d1LimiterKind:
		return d1UserFacingWording{Parameter: "限幅器输出上限", Unit: " dB", WhereToLook: "该轨限幅器效果的输出上限",
			BeforeKey: "before_readback_value", AfterKey: "actual_readback_value"}
	case d1GateExpanderKind:
		return d1UserFacingWording{Parameter: "门限处理器范围", Unit: " dB", WhereToLook: "该轨门限处理器效果的范围",
			BeforeKey: "before_readback_value", AfterKey: "actual_readback_value"}
	case d1MultibandKind:
		return d1UserFacingWording{Parameter: "多段压限器频段阈值", Unit: " dB", WhereToLook: "该轨多段压限器效果对应频段的阈值",
			BeforeKey: "before_readback_value", AfterKey: "actual_readback_value"}
	default:
		return d1UserFacingWording{Parameter: "参数", Unit: "", WhereToLook: "目标轨道的效果器/通道条参数",
			BeforeKey: "before_readback_value", AfterKey: "actual_readback_value"}
	}
}

// d1AppliedReportReply composes the five-element applied report for a D1
// bounded native adjustment: target track (user-recognizable label), parameter,
// signed change plus readback before/after, the finding the move targets (the
// admission hypothesis in the model's own words), and where to look in the
// DAW. The reply deliberately carries no pending-human-judgment promise (B6
// defect 2, chosen route: the settle evaluation decides the judgment boundary
// later; the applied turn must not pre-commit one) and no internal codes.
func d1AppliedReportReply(loop freeStateReasoningLoop, receipt map[string]any) string {
	if loop.Experiment == nil {
		return stripInternalTerminalTerms("这一步参数调整已应用并回读验证。")
	}
	admission := loop.Experiment.Admission
	spec, specOK := experiment.D1S1DomainSpecFor(admission)
	wording := d1UserFacingWording{Parameter: "参数", Unit: "", WhereToLook: "目标轨道的效果器/通道条参数", BeforeKey: "before_readback_value", AfterKey: "actual_readback_value"}
	if specOK {
		wording = d1UserFacingWordingFor(spec.ActionKind)
	} else if kind := firstStringFromMap(admission.TypedAction, "action_kind", "kind"); kind != "" {
		wording = d1UserFacingWordingFor(kind)
	}

	targetID := firstStringFromMap(admission.TargetRef, "id", "track_id")
	targetName := firstStringFromMap(admission.TargetRef, "label", "name", "track_name")
	trackLabel := pendingMixTickTrackLabel(targetID)
	if targetName != "" && targetName != targetID {
		trackLabel += "（" + targetName + "）"
	}

	delta, deltaOK := treatmentNumber(admission.TypedAction, "delta_pan", "delta_db", "gain_db", "threshold_db", "attack_db", "ceiling_db", "range_db", "band_threshold_db")
	changeText := ""
	if deltaOK && delta != 0 {
		changeText = signedTerminalNumber(delta, wording.Unit)
	} else if after, ok := receiptNumber(receipt, wording.AfterKey); ok {
		changeText = "目标值 " + plainTerminalNumber(after, wording.Unit)
	}

	readbackText := ""
	before, beforeOK := receiptNumber(receipt, wording.BeforeKey)
	after, afterOK := receiptNumber(receipt, wording.AfterKey)
	switch {
	case beforeOK && afterOK:
		readbackText = "（回读 " + plainTerminalNumber(before, wording.Unit) + " → " + plainTerminalNumber(after, wording.Unit) + "）"
	case afterOK:
		readbackText = "（回读 " + plainTerminalNumber(after, wording.Unit) + "）"
	}

	finding := strings.TrimSpace(firstNonEmpty(admission.Hypothesis, admission.ExpectedEffect, loop.OriginalIntent))
	if finding == "" {
		finding = "基于动作前观察证据选定的一个小步调整"
	}

	head := "已应用并回读验证：" + trackLabel + wording.Parameter + " " + firstNonEmpty(changeText, "已调整") + readbackText + "。"
	body := strings.Join([]string{
		head,
		"针对的发现：" + truncateTerminalFinding(finding) + "。",
		"去哪看：在 DAW 里选中 " + trackLabel + "，查看" + wording.WhereToLook + "即可确认这一步。",
		"这一步是有界、可回滚的小步调整；如果听感不对，告诉我，我可以回滚。",
	}, "\n")
	return stripInternalTerminalTerms(body)
}

// truncateTerminalFinding bounds the model-authored finding clause; the
// admission hypothesis is free text and can occasionally run long.
func truncateTerminalFinding(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= 80 {
		return string(runes)
	}
	return string(runes[:80]) + "…"
}

func receiptNumber(receipt map[string]any, keys ...string) (float64, bool) {
	if receipt == nil {
		return 0, false
	}
	return treatmentNumber(receipt, keys...)
}

// signedTerminalNumber renders a signed bounded delta with its unit, trimming
// trailing zeros (+0.05, +0.8 dB, -1.5 dB).
func signedTerminalNumber(value float64, unit string) string {
	rounded := math.Round(value*1000) / 1000
	text := strconv.FormatFloat(rounded, 'f', -1, 64)
	sign := ""
	if rounded >= 0 {
		sign = "+"
	}
	return sign + text + unit
}

// plainTerminalNumber renders a readback value with its unit (no forced sign).
func plainTerminalNumber(value float64, unit string) string {
	rounded := math.Round(value*1000) / 1000
	text := strconv.FormatFloat(rounded, 'f', -1, 64)
	if text == "-0" {
		text = "0"
	}
	return text + unit
}
