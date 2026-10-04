package chat

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
)

// Free-state range goal protocol phases (FREESTATE-REGION-GOAL-1 DESIGN 问 1/问 2):
// split_pending → split_done → treating. The enum is deliberately neutral so a
// future B2 materialization (envelope write instead of a split) can replace
// round 1 without reshaping the protocol state.
const (
	freeStateRangePhaseSplitPending = "split_pending"
	freeStateRangePhaseSplitDone    = "split_done"
	freeStateRangePhaseTreating     = "treating"
)

// freeStateRangeSplitToleranceSeconds is the round-1 structural assertion
// tolerance: the surviving sub clip's measured span must match the boxed
// range width within this bound (the kernel split response carries exact
// start/length seconds, so the residual is float noise plus boundary slack).
const freeStateRangeSplitToleranceSeconds = 0.02

// freeStateRangeGoal is the per-goal payload of the two-round range protocol
// riding free_state_reasoning_loop (DESIGN 问 1): the boxed-range structural
// operation target of this conversation. It is orchestration memory — never a
// Goal identity and never a mutation authority.
type freeStateRangeGoal struct {
	ClipID      string  `json:"clip_id,omitempty"`
	TrackID     string  `json:"track_id,omitempty"`
	RangeStartS float64 `json:"range_start_s,omitempty"`
	RangeEndS   float64 `json:"range_end_s,omitempty"`
	Phase       string  `json:"phase,omitempty"`
	SubClipID   string  `json:"sub_clip_id,omitempty"`
}

func cloneFreeStateRangeGoal(goal *freeStateRangeGoal) *freeStateRangeGoal {
	if goal == nil {
		return nil
	}
	out := *goal
	return &out
}

func freeStateRangePhaseRank(phase string) int {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case freeStateRangePhaseSplitPending:
		return 0
	case freeStateRangePhaseSplitDone:
		return 1
	case freeStateRangePhaseTreating:
		return 2
	default:
		return -1
	}
}

// freeStateRangeTreatmentVerbs is the card's simplest treatment-intent gate
// (IMPL-2 refines the speech boundary): a boxed range only arms the two-round
// protocol when the speech carries a treatment-class verb. Pure structural
// phrasing ("把这段拆出来") keeps riding the intent layer's ordinary
// confirmation path and never arms a range goal.
var freeStateRangeTreatmentVerbs = []string{
	"处理", "调一", "调整", "调改", "改善", "优化", "修一", "修改",
	"不闷", "更亮", "亮一点", "暗一点", "更厚", "更薄", "干净", "好听",
	"混响", "均衡", "压缩", "激励", "响一点",
	"treat", "process", "adjust", "improve", "fix", "brighten", "eq", "compress", "reverb",
}

// freeStateRangeGoalFromRequest arms the protocol state on a fresh loop:
// treatment-class speech plus at least one boxed range. v1 supports a single
// range — the first well-formed row wins, the rest are ignored (DESIGN 问 4).
func freeStateRangeGoalFromRequest(userText string, requestContext map[string]any) *freeStateRangeGoal {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || !textHasAny(text, freeStateRangeTreatmentVerbs...) {
		return nil
	}
	for _, row := range contextClipRangeRows(requestContext["selected_clip_ranges"]) {
		goal := freeStateRangeGoalFromRow(row)
		if goal != nil {
			return goal
		}
	}
	return nil
}

func freeStateRangeGoalFromRow(row map[string]any) *freeStateRangeGoal {
	clipID := strings.TrimSpace(firstStringFromMap(row, "clip_id"))
	trackID := strings.TrimSpace(firstStringFromMap(row, "track_id"))
	start, okStart := freeStateRowSeconds(row, "start_seconds")
	end, okEnd := freeStateRowSeconds(row, "end_seconds")
	if clipID == "" || trackID == "" || !okStart || !okEnd || end-start <= 0 {
		return nil
	}
	return &freeStateRangeGoal{
		ClipID:      clipID,
		TrackID:     trackID,
		RangeStartS: start,
		RangeEndS:   end,
		Phase:       freeStateRangePhaseSplitPending,
	}
}

func freeStateRowSeconds(row map[string]any, key string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(row[key])), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// freeStateSplitReceipt is the kernel-facing shape of one clip.split execution
// receipt (VitApp ClipService split response: the original clip_id stays on
// the left half, the freshly created clip is the right half).
type freeStateSplitReceipt struct {
	ClipID      string
	SplitTime   float64
	LeftClipID  string
	RightClipID string
	LeftStart   float64
	LeftLength  float64
	RightStart  float64
	RightLength float64
	OK          bool
}

func freeStateSplitReceiptFromExecuted(row map[string]any) (freeStateSplitReceipt, bool) {
	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		firstStringFromMap(row, "tool"),
		firstStringFromMap(row, "command_name"),
	)))
	if name != "clip.split" && name != "split_clip" {
		return freeStateSplitReceipt{}, false
	}
	result := firstMapFromAny(row["result"])
	if result == nil {
		return freeStateSplitReceipt{}, false
	}
	receipt := freeStateSplitReceipt{
		ClipID:      strings.TrimSpace(firstStringFromMap(result, "clip_id")),
		LeftClipID:  strings.TrimSpace(firstStringFromMap(result, "left_clip_id")),
		RightClipID: strings.TrimSpace(firstStringFromMap(result, "right_clip_id", "new_clip_id")),
		OK: strings.EqualFold(strings.TrimSpace(firstStringFromMap(result, "status")), "ok") &&
			strings.TrimSpace(firstStringFromMap(row, "error")) == "",
	}
	splitTime, okTime := freeStateRowSeconds(result, "split_time_seconds")
	receipt.SplitTime = splitTime
	receipt.LeftStart, _ = freeStateRowSeconds(result, "left_start_seconds")
	receipt.LeftLength, _ = freeStateRowSeconds(result, "left_length_seconds")
	receipt.RightStart, _ = freeStateRowSeconds(result, "right_start_seconds")
	receipt.RightLength, _ = freeStateRowSeconds(result, "right_length_seconds")
	if receipt.ClipID == "" || !okTime {
		return freeStateSplitReceipt{}, false
	}
	return receipt, true
}

// freeStateRangeSplitOutcome replays the round-1 cut plan against the executed
// receipts: the boxed span must have survived as its own clip with the
// expected id geometry and width (tolerance 0.02s). sawSplit=false means no
// round-1 split receipt for the target clip appeared at all and the
// choreography stays put; sawSplit with ok=false carries the assertion
// failures as the blocked-settle evidence.
func freeStateRangeSplitOutcome(goal freeStateRangeGoal, executed []map[string]any) (sawSplit bool, subClipID string, failures []string, ok bool) {
	width := goal.RangeEndS - goal.RangeStartS
	fail := func(format string, args ...any) {
		failures = append(failures, fmt.Sprintf(format, args...))
	}
	var startCuts, endCuts []freeStateSplitReceipt
	for _, row := range executed {
		receipt, match := freeStateSplitReceiptFromExecuted(row)
		if !match || receipt.ClipID != goal.ClipID {
			continue
		}
		sawSplit = true
		if !receipt.OK {
			fail("split at %.3fs did not execute cleanly", receipt.SplitTime)
			continue
		}
		switch {
		case math.Abs(receipt.SplitTime-goal.RangeEndS) <= freeStateRangeSplitToleranceSeconds:
			endCuts = append(endCuts, receipt)
		case math.Abs(receipt.SplitTime-goal.RangeStartS) <= freeStateRangeSplitToleranceSeconds:
			startCuts = append(startCuts, receipt)
		default:
			fail("unexpected split at %.3fs outside the boxed bounds [%.3f, %.3f]",
				receipt.SplitTime, goal.RangeStartS, goal.RangeEndS)
		}
	}
	if !sawSplit {
		return false, "", nil, false
	}
	if len(failures) > 0 {
		return true, "", failures, false
	}
	var sub string
	switch {
	case len(startCuts) > 0 && len(endCuts) > 0:
		// INTENT-WIRE order: the end cut runs first and the start cut second,
		// so the boxed span is the start cut's right half.
		end := endCuts[0]
		start := startCuts[len(startCuts)-1]
		if start.RightClipID == "" {
			fail("start cut returned no sub clip id")
		}
		if math.Abs(end.RightStart-goal.RangeEndS) > freeStateRangeSplitToleranceSeconds {
			fail("end cut right start %.3fs expected %.3fs", end.RightStart, goal.RangeEndS)
		}
		if math.Abs(start.RightStart-goal.RangeStartS) > freeStateRangeSplitToleranceSeconds {
			fail("start cut right start %.3fs expected %.3fs", start.RightStart, goal.RangeStartS)
		}
		if math.Abs(start.RightLength-width) > freeStateRangeSplitToleranceSeconds {
			fail("sub clip length %.3fs expected %.3fs", start.RightLength, width)
		}
		sub = start.RightClipID
	case len(startCuts) > 0:
		// Degenerate single cut: the range reaches the clip end, so the boxed
		// span is the start cut's right half.
		start := startCuts[len(startCuts)-1]
		if start.RightClipID == "" {
			fail("start cut returned no sub clip id")
		}
		if math.Abs(start.RightStart-goal.RangeStartS) > freeStateRangeSplitToleranceSeconds {
			fail("sub clip start %.3fs expected %.3fs", start.RightStart, goal.RangeStartS)
		}
		if math.Abs(start.RightLength-width) > freeStateRangeSplitToleranceSeconds {
			fail("sub clip length %.3fs expected %.3fs", start.RightLength, width)
		}
		sub = start.RightClipID
	case len(endCuts) > 0:
		// Degenerate single cut: the range reaches the clip start, so the
		// boxed span is the end cut's left half (the kernel keeps the original
		// clip id on the left).
		end := endCuts[len(endCuts)-1]
		if end.LeftClipID == "" {
			fail("end cut returned no sub clip id")
		}
		if math.Abs(end.LeftStart-goal.RangeStartS) > freeStateRangeSplitToleranceSeconds {
			fail("sub clip start %.3fs expected %.3fs", end.LeftStart, goal.RangeStartS)
		}
		if math.Abs(end.LeftLength-width) > freeStateRangeSplitToleranceSeconds {
			fail("sub clip length %.3fs expected %.3fs", end.LeftLength, width)
		}
		sub = end.LeftClipID
	default:
		fail("no split landed on either range bound")
	}
	if len(failures) > 0 {
		return true, "", failures, false
	}
	return true, sub, nil, true
}

// applyFreeStateRangeSplitOutcome is the round-1 structural settle hook. It
// runs on the freshly recorded loop after every free-state turn result: while
// the range goal waits in split_pending, the first clean split receipt set
// promotes the phase to split_done and records the sub clip the treatment
// round must target; a landed-but-failed split set blocks the loop with the
// assertion evidence on the durable record. No receipts → no-op: the split
// simply has not happened yet. The returned loop carries the transition for
// the same-turn response envelope.
func (s *Server) applyFreeStateRangeSplitOutcome(conversationID string, loop freeStateReasoningLoop, res agentloop.Result) freeStateReasoningLoop {
	if loop.RangeGoal == nil || !freeStateLoopActive(loop) {
		return loop
	}
	if !strings.EqualFold(strings.TrimSpace(loop.RangeGoal.Phase), freeStateRangePhaseSplitPending) {
		return loop
	}
	sawSplit, subClipID, failures, ok := freeStateRangeSplitOutcome(*loop.RangeGoal, res.Executed)
	if !sawSplit {
		return loop
	}
	now := time.Now().UTC()
	record := freeStateActionRecord{
		Cycle:         loop.Cycle,
		ProcessorType: "range_split",
		Workflow:      "free_state_range_split",
		RecordedAt:    now,
	}
	if ok {
		loop.RangeGoal.Phase = freeStateRangePhaseSplitDone
		loop.RangeGoal.SubClipID = subClipID
		record.Status = "applied"
		record.Summary = fmt.Sprintf("range split verified: sub clip %s spans %.3fs-%.3fs",
			subClipID, loop.RangeGoal.RangeStartS, loop.RangeGoal.RangeEndS)
		record.Receipt = map[string]any{
			"sub_clip_id":   subClipID,
			"range_start_s": loop.RangeGoal.RangeStartS,
			"range_end_s":   loop.RangeGoal.RangeEndS,
			"phase":         loop.RangeGoal.Phase,
			"settle":        "structural",
		}
	} else {
		record.Status = "failed"
		record.Summary = strings.Join(failures, "; ")
		record.Receipt = map[string]any{
			"failures":      failures,
			"range_start_s": loop.RangeGoal.RangeStartS,
			"range_end_s":   loop.RangeGoal.RangeEndS,
		}
		loop.Status = "blocked"
		loop.LastError = "range split assertion failed: " + strings.Join(failures, "; ")
	}
	loop.Actions = append(loop.Actions, record)
	loop.UpdatedAt = now
	s.storeFreeStateLoop(loop)
	if s.logger != nil {
		if ok {
			s.logger.Info("[free-state-range] round-1 split verified conversation=%s goal=%s sub_clip=%s", conversationID, loop.GoalID, subClipID)
		} else {
			s.logger.Warn("[free-state-range] round-1 split assertion failed conversation=%s goal=%s: %s", conversationID, loop.GoalID, loop.LastError)
		}
	}
	return loop
}

// applyFreeStateRangeGoalTreatmentTarget is the round-2 handoff: once the
// split is verified, the treatment target the loop binds must be the sub clip
// the split produced. v1 binds the preference onto the loop's TargetRef — the
// track id stays untouched so the D1-S1 admission target checks keep holding,
// and clip_id records the clip-scoped operation target — then advances the
// phase to treating. One-shot and idempotent.
func applyFreeStateRangeGoalTreatmentTarget(loop *freeStateReasoningLoop) {
	if loop == nil || loop.RangeGoal == nil || loop.TargetRef == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(loop.RangeGoal.Phase), freeStateRangePhaseSplitDone) {
		return
	}
	if strings.TrimSpace(loop.RangeGoal.SubClipID) == "" {
		return
	}
	trackID := strings.TrimSpace(firstStringFromMap(loop.TargetRef, "track_id", "id"))
	if trackID == "" || !strings.EqualFold(trackID, strings.TrimSpace(loop.RangeGoal.TrackID)) {
		return
	}
	loop.TargetRef["clip_id"] = loop.RangeGoal.SubClipID
	loop.RangeGoal.Phase = freeStateRangePhaseTreating
}

// freeStateRangeGoalRetentionNote is the inter-round abort semantic (DESIGN 问 2
// 轮间中止): a loop that terminalizes after the split landed leaves the sub clip
// in the project — no automatic revert — and the reply faces carry the
// split-awaiting-treatment note.
func freeStateRangeGoalRetentionNote(loop freeStateReasoningLoop) string {
	if loop.RangeGoal == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(loop.RangeGoal.Phase)) {
	case freeStateRangePhaseSplitDone, freeStateRangePhaseTreating:
	default:
		return ""
	}
	subClipID := strings.TrimSpace(loop.RangeGoal.SubClipID)
	if subClipID == "" {
		return ""
	}
	return fmt.Sprintf("已拆分待调改：子 clip %s 已留存，未自动回退；继续即可对该段调改，或用撤销回退结构。", subClipID)
}

// freeStateRangeGoalReplyNote appends the retention note to a terminal reply
// face when the loop stopped between the two rounds.
func freeStateRangeGoalReplyNote(loop freeStateReasoningLoop, reply string) string {
	if note := freeStateRangeGoalRetentionNote(loop); note != "" {
		return strings.TrimSpace(reply + " " + note)
	}
	return reply
}
