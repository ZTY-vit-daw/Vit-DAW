package chat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
)

func rangeGoalTestContext() map[string]any {
	ctx := map[string]any{
		"selected_track_id":   "track-1",
		"selected_track_name": "Lead Vocal",
		"goal_id":             "goal-range-1",
		"run_id":              "run-range-1",
		"selected_clip_ranges": []map[string]any{{
			"range_id":      "range-1",
			"clip_id":       "clip-1",
			"track_id":      "track-1",
			"start_seconds": 4.0,
			"end_seconds":   8.0,
		}},
	}
	return contextWithSemanticEntryDecision(ctx, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteOpenSemantic,
		Controller:  "minimal_audio_closure",
		TargetScope: semanticEntryScopeCurrentSelection, ControlMode: semanticEntryControlSemanticLoop,
		UserAuthorization: semanticEntryAuthorizationAction, Confidence: 0.94, Reason: "open acoustic outcome",
	})
}

func rangeSplitExecRow(clipID, leftID, rightID string, splitTime, leftStart, leftLength, rightStart, rightLength float64, status string) map[string]any {
	if status == "" {
		status = "ok"
	}
	resultStatus := status
	if status == "error" {
		resultStatus = "error"
	}
	return map[string]any{
		"tool": "clip.split", "status": status, "error": "",
		"result": map[string]any{
			"status":               resultStatus,
			"clip_id":              clipID,
			"left_clip_id":         leftID,
			"right_clip_id":        rightID,
			"split_time_seconds":   splitTime,
			"left_start_seconds":   leftStart,
			"left_length_seconds":  leftLength,
			"right_start_seconds":  rightStart,
			"right_length_seconds": rightLength,
		},
	}
}

func TestFreeStateRangeGoalOldStateJSONDecodesFailOpen(t *testing.T) {
	// AGENTS §11 兼容义务：旧 loop 状态无 range_goal 键，反序列化必须 fail-open
	// ——普通 goal 零影响。
	var stored map[string]any
	if err := json.Unmarshal([]byte(`{
		"schema_version": "free_state_reasoning_loop.v1",
		"loop_id": "free_state_old",
		"conversation_id": "conversation-old",
		"status": "reasoning",
		"original_intent": "把选中的这段处理到不闷",
		"cycle": 1,
		"max_cycles": 6
	}`), &stored); err != nil {
		t.Fatalf("fixture json: %v", err)
	}
	loop, ok := freeStateLoopFromAny(stored)
	if !ok || !freeStateLoopActive(loop) {
		t.Fatalf("legacy loop did not decode as active: ok=%v loop=%#v", ok, loop)
	}
	if loop.RangeGoal != nil {
		t.Fatalf("legacy loop must decode with nil range_goal, got %#v", loop.RangeGoal)
	}
}

func TestFreeStateRangeGoalJSONRoundTripPreservesField(t *testing.T) {
	loop := freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "free_state_range",
		ConversationID: "conversation-range",
		Status:         "reasoning",
		OriginalIntent: "把这段处理到不闷",
		RangeGoal: &freeStateRangeGoal{
			ClipID: "clip-1", TrackID: "track-1",
			RangeStartS: 4, RangeEndS: 8,
			Phase: freeStateRangePhaseSplitDone, SubClipID: "clip-sub",
		},
	}
	restored, ok := freeStateLoopFromAny(freeStateLoopMap(loop))
	if !ok {
		t.Fatalf("round trip lost the loop schema")
	}
	if restored.RangeGoal == nil {
		t.Fatalf("round trip dropped range_goal")
	}
	if restored.RangeGoal.ClipID != "clip-1" || restored.RangeGoal.TrackID != "track-1" ||
		restored.RangeGoal.RangeStartS != 4 || restored.RangeGoal.RangeEndS != 8 ||
		restored.RangeGoal.Phase != freeStateRangePhaseSplitDone || restored.RangeGoal.SubClipID != "clip-sub" {
		t.Fatalf("round trip mutated range_goal: %#v", restored.RangeGoal)
	}
}

func TestFreeStateRangeGoalMergePhaseNeverRegresses(t *testing.T) {
	done := &freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", RangeStartS: 4, RangeEndS: 8,
		Phase: freeStateRangePhaseSplitDone, SubClipID: "clip-sub"}
	pending := &freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", RangeStartS: 4, RangeEndS: 8,
		Phase: freeStateRangePhaseSplitPending}
	base := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, UpdatedAt: time.Now().UTC(), RangeGoal: done}
	merged := mergeFreeStateLoops(base, base, false)
	if merged.RangeGoal == nil || *merged.RangeGoal != *done {
		t.Fatalf("no-overlay merge must keep the durable range goal: %#v", merged.RangeGoal)
	}
	stale := base
	stale.RangeGoal = pending
	merged = mergeFreeStateLoops(base, stale, true)
	if merged.RangeGoal.Phase != freeStateRangePhaseSplitDone || merged.RangeGoal.SubClipID != "clip-sub" {
		t.Fatalf("older transport copy regressed the durable phase: %#v", merged.RangeGoal)
	}
	// 服务端在信封铸出后写入的 split_done 过渡必须胜过 split_pending 基线。
	overlay := pendingLoop(t)
	overlay.RangeGoal = done
	merged = mergeFreeStateLoops(pendingLoop(t), overlay, true)
	if merged.RangeGoal.Phase != freeStateRangePhaseSplitDone || merged.RangeGoal.SubClipID != "clip-sub" {
		t.Fatalf("split_done overlay lost to split_pending base: %#v", merged.RangeGoal)
	}
	// B2/快照统摄留位：未知 phase（未来终态）fail-open 保留原文，且不回退已知相位。
	future := base
	future.RangeGoal = &freeStateRangeGoal{Phase: "snapshot_written"}
	merged = mergeFreeStateLoops(base, future, true)
	if merged.RangeGoal.Phase != freeStateRangePhaseSplitDone {
		t.Fatalf("unknown-phase overlay regressed a known phase: %#v", merged.RangeGoal)
	}
	unknownBase := pendingLoop(t)
	unknownBase.RangeGoal = &freeStateRangeGoal{Phase: "snapshot_written"}
	merged = mergeFreeStateLoops(unknownBase, unknownBase, false)
	if merged.RangeGoal.Phase != "snapshot_written" {
		t.Fatalf("unknown phase must round-trip fail-open: %#v", merged.RangeGoal)
	}
}

func pendingLoop(t *testing.T) freeStateReasoningLoop {
	t.Helper()
	return freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, UpdatedAt: time.Now().UTC(),
		RangeGoal: &freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", RangeStartS: 4, RangeEndS: 8,
			Phase: freeStateRangePhaseSplitPending}}
}

func TestFreeStateRangeGoalArmsOnFreshLoopWithTreatmentSpeechAndRange(t *testing.T) {
	server := New(nil, nil, nil)
	prepared, active := server.prepareFreeStateReasoningContext("conversation-range-arm", "把框选的这段处理到不闷一点", rangeGoalTestContext())
	if !active {
		t.Fatalf("free-state loop did not start")
	}
	loop, ok := server.freeStateLoop("conversation-range-arm")
	if !ok {
		t.Fatalf("loop not stored")
	}
	_ = prepared
	if loop.RangeGoal == nil {
		t.Fatalf("treatment speech with a boxed range must arm the range goal")
	}
	if loop.RangeGoal.Phase != freeStateRangePhaseSplitPending {
		t.Fatalf("fresh range goal phase = %q", loop.RangeGoal.Phase)
	}
	if loop.RangeGoal.ClipID != "clip-1" || loop.RangeGoal.TrackID != "track-1" ||
		loop.RangeGoal.RangeStartS != 4 || loop.RangeGoal.RangeEndS != 8 {
		t.Fatalf("range goal fields = %#v", loop.RangeGoal)
	}
	if firstStringFromMap(loop.TargetRef, "id") != "track-1" {
		t.Fatalf("target ref = %#v", loop.TargetRef)
	}
}

func TestFreeStateRangeGoalDoesNotArmWithoutTreatmentVerbOrRange(t *testing.T) {
	server := New(nil, nil, nil)
	// 纯结构话术：即使 ranges 在场也不武装（设计问 3 分界——IMPL-2 精化）。
	server.prepareFreeStateReasoningContext("conversation-range-structural", "把这段拆出来", rangeGoalTestContext())
	if loop, ok := server.freeStateLoop("conversation-range-structural"); ok && loop.RangeGoal != nil {
		t.Fatalf("pure structural speech must not arm a range goal: %#v", loop.RangeGoal)
	}
	// 处理话术但无 ranges：不武装。
	noRange := rangeGoalTestContext()
	delete(noRange, "selected_clip_ranges")
	server.prepareFreeStateReasoningContext("conversation-range-norange", "把这段处理到不闷", noRange)
	if loop, ok := server.freeStateLoop("conversation-range-norange"); ok && loop.RangeGoal != nil {
		t.Fatalf("treatment speech without a boxed range must not arm: %#v", loop.RangeGoal)
	}
}

func TestFreeStateRangeSplitOutcomeTwoCutPlanVerifies(t *testing.T) {
	goal := freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", RangeStartS: 4, RangeEndS: 8,
		Phase: freeStateRangePhaseSplitPending}
	// clip-1 [2,12]；先终点切 8（左 [2,8] 原 id / 右 [8,12]），再起点切 4（左 [2,4] 原 id / 右 [4,8] 子段）。
	executed := []map[string]any{
		rangeSplitExecRow("clip-1", "clip-1", "clip-r1", 8, 2, 6, 8, 4, ""),
		rangeSplitExecRow("clip-1", "clip-1", "clip-sub", 4, 2, 2, 4, 4, ""),
	}
	sawSplit, sub, failures, ok := freeStateRangeSplitOutcome(goal, executed)
	if !sawSplit || !ok || len(failures) != 0 {
		t.Fatalf("two-cut plan must verify: saw=%v ok=%v failures=%v", sawSplit, ok, failures)
	}
	if sub != "clip-sub" {
		t.Fatalf("sub clip = %q, want the start cut's right half", sub)
	}
}

func TestFreeStateRangeSplitOutcomeDegenerateSingleCutsVerify(t *testing.T) {
	// 贴终点退化单切：range [6,10] 触及 clip 尾 [2,10]，只切 6，子段=右半。
	endTouch := freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", RangeStartS: 6, RangeEndS: 10,
		Phase: freeStateRangePhaseSplitPending}
	sawSplit, sub, failures, ok := freeStateRangeSplitOutcome(endTouch, []map[string]any{
		rangeSplitExecRow("clip-1", "clip-1", "clip-sub", 6, 2, 4, 6, 4, ""),
	})
	if !sawSplit || !ok || sub != "clip-sub" || len(failures) != 0 {
		t.Fatalf("end-touch single cut must verify: saw=%v ok=%v sub=%q failures=%v", sawSplit, ok, sub, failures)
	}
	// 贴起点退化单切：range [3,7] 触及 clip 头 [3,11]，只切 7，子段=左半（原 id 留左）。
	startTouch := freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", RangeStartS: 3, RangeEndS: 7,
		Phase: freeStateRangePhaseSplitPending}
	sawSplit, sub, failures, ok = freeStateRangeSplitOutcome(startTouch, []map[string]any{
		rangeSplitExecRow("clip-1", "clip-1", "clip-r1", 7, 3, 4, 7, 4, ""),
	})
	if !sawSplit || !ok || sub != "clip-1" || len(failures) != 0 {
		t.Fatalf("start-touch single cut must verify with the original id: saw=%v ok=%v sub=%q failures=%v", sawSplit, ok, sub, failures)
	}
}

func TestFreeStateRangeSplitOutcomeRejectsWrongGeometryAndFailedExecution(t *testing.T) {
	goal := freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", RangeStartS: 4, RangeEndS: 8,
		Phase: freeStateRangePhaseSplitPending}
	// 子段宽度超出 0.02s 容差。
	sawSplit, _, failures, ok := freeStateRangeSplitOutcome(goal, []map[string]any{
		rangeSplitExecRow("clip-1", "clip-1", "clip-r1", 8, 2, 6, 8, 4, ""),
		rangeSplitExecRow("clip-1", "clip-1", "clip-sub", 4, 2, 2, 4, 4.5, ""),
	})
	if !sawSplit || ok || len(failures) == 0 {
		t.Fatalf("width out of tolerance must fail with evidence: ok=%v failures=%v", ok, failures)
	}
	// 内核回报失败。
	sawSplit, _, failures, ok = freeStateRangeSplitOutcome(goal, []map[string]any{
		rangeSplitExecRow("clip-1", "", "", 8, 0, 0, 0, 0, "error"),
	})
	if !sawSplit || ok || len(failures) == 0 {
		t.Fatalf("failed split execution must fail with evidence: ok=%v failures=%v", ok, failures)
	}
	// 计划外的切点=意外结构变更。
	sawSplit, _, failures, ok = freeStateRangeSplitOutcome(goal, []map[string]any{
		rangeSplitExecRow("clip-1", "clip-1", "clip-x", 5, 2, 3, 5, 7, ""),
	})
	if !sawSplit || ok || len(failures) == 0 {
		t.Fatalf("out-of-bounds split must fail: ok=%v failures=%v", ok, failures)
	}
	// 无 split 回执：编舞原地不动。
	if sawSplit, _, failures, ok = freeStateRangeSplitOutcome(goal, nil); sawSplit || ok || failures != nil {
		t.Fatalf("no receipts must be a no-op: saw=%v ok=%v failures=%v", sawSplit, ok, failures)
	}
}

func TestApplyFreeStateRangeSplitOutcomeThreeRoundStates(t *testing.T) {
	buildLoop := func() freeStateReasoningLoop {
		return freeStateReasoningLoop{
			SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free_state_range",
			ConversationID: "conversation-rounds", GoalID: "goal-range-1", RunID: "run-range-1",
			Status: "reasoning", OriginalIntent: "把这段处理到不闷", ActiveIntent: "把这段处理到不闷",
			MaxCycles: freeStateDefaultMaxCycles,
			RangeGoal: &freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", RangeStartS: 4, RangeEndS: 8,
				Phase: freeStateRangePhaseSplitPending},
		}
	}
	executed := []map[string]any{
		rangeSplitExecRow("clip-1", "clip-1", "clip-r1", 8, 2, 6, 8, 4, ""),
		rangeSplitExecRow("clip-1", "clip-1", "clip-sub", 4, 2, 2, 4, 4, ""),
	}

	t.Run("断言过→split_done+记sub_clip_id", func(t *testing.T) {
		server := New(nil, nil, nil)
		loop := buildLoop()
		server.storeFreeStateLoop(loop)
		loop = server.applyFreeStateRangeSplitOutcome("conversation-rounds", loop, agentloop.Result{Executed: executed})
		if loop.RangeGoal.Phase != freeStateRangePhaseSplitDone || loop.RangeGoal.SubClipID != "clip-sub" {
			t.Fatalf("pass state = %#v", loop.RangeGoal)
		}
		if !freeStateLoopActive(loop) {
			t.Fatalf("verified split must keep the loop active for round 2")
		}
		stored, ok := server.freeStateLoop("conversation-rounds")
		if !ok || stored.RangeGoal.SubClipID != "clip-sub" {
			t.Fatalf("durable loop lost the transition: %#v", stored.RangeGoal)
		}
		found := false
		for _, action := range stored.Actions {
			if action.Workflow == "free_state_range_split" && action.Status == "applied" {
				found = true
			}
		}
		if !found {
			t.Fatalf("structural settle action not recorded: %#v", stored.Actions)
		}
	})

	t.Run("断言不过→blocked+证据上交", func(t *testing.T) {
		server := New(nil, nil, nil)
		loop := buildLoop()
		server.storeFreeStateLoop(loop)
		badExecuted := []map[string]any{
			rangeSplitExecRow("clip-1", "clip-1", "clip-r1", 8, 2, 6, 8, 4, ""),
			rangeSplitExecRow("clip-1", "clip-1", "clip-sub", 4, 2, 2, 4, 4.5, ""),
		}
		loop = server.applyFreeStateRangeSplitOutcome("conversation-rounds", loop, agentloop.Result{Executed: badExecuted})
		if loop.Status != "blocked" {
			t.Fatalf("failed assertion must block the loop, status=%q", loop.Status)
		}
		if !strings.Contains(loop.LastError, "range split assertion failed") {
			t.Fatalf("blocked settle must carry the evidence: %q", loop.LastError)
		}
		if loop.RangeGoal.Phase != freeStateRangePhaseSplitPending || loop.RangeGoal.SubClipID != "" {
			t.Fatalf("failed split must not claim split_done: %#v", loop.RangeGoal)
		}
	})

	t.Run("无回执→编舞原地不动", func(t *testing.T) {
		server := New(nil, nil, nil)
		loop := buildLoop()
		after := server.applyFreeStateRangeSplitOutcome("conversation-rounds", loop, agentloop.Result{})
		if after.RangeGoal.Phase != freeStateRangePhaseSplitPending {
			t.Fatalf("no receipts must not move the phase: %#v", after.RangeGoal)
		}
	})

	t.Run("中止留存→子clip留存+注记，不自动revert", func(t *testing.T) {
		server := New(nil, nil, nil)
		loop := buildLoop()
		server.storeFreeStateLoop(loop)
		loop = server.applyFreeStateRangeSplitOutcome("conversation-rounds", loop, agentloop.Result{Executed: executed})
		// 会话中断：loop 终端化（用户停止路径写 stopped；此处以 cancelled 同形覆盖）。
		loop.Status = "cancelled"
		loop.LastError = "user cancelled"
		loop.UpdatedAt = time.Now().UTC()
		server.storeFreeStateLoop(loop)
		stored, ok := server.freeStateLoop("conversation-rounds")
		if !ok {
			t.Fatalf("terminal loop must stay durable")
		}
		if stored.RangeGoal == nil || stored.RangeGoal.Phase != freeStateRangePhaseSplitDone || stored.RangeGoal.SubClipID != "clip-sub" {
			t.Fatalf("abort must retain the sub clip state without revert: %#v", stored.RangeGoal)
		}
		note := freeStateRangeGoalRetentionNote(stored)
		if !strings.Contains(note, "已拆分待调改") || !strings.Contains(note, "clip-sub") {
			t.Fatalf("retention note = %q", note)
		}
		reply := freeStateRangeGoalReplyNote(stored, "free-state reasoning loop has ended.")
		if !strings.Contains(reply, "已拆分待调改") {
			t.Fatalf("terminal reply must carry the retention note: %q", reply)
		}
		if freeStateRangeGoalRetentionNote(freeStateReasoningLoop{}) != "" {
			t.Fatalf("plain goals must not grow a retention note")
		}
	})
}

func TestApplyFreeStateRangeGoalTreatmentTargetBindsSubClip(t *testing.T) {
	loop := freeStateReasoningLoop{
		RangeGoal: &freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", RangeStartS: 4, RangeEndS: 8,
			Phase: freeStateRangePhaseSplitDone, SubClipID: "clip-sub"},
		TargetRef: map[string]any{"kind": "track", "id": "track-1", "track_id": "track-1"},
	}
	applyFreeStateRangeGoalTreatmentTarget(&loop)
	if firstStringFromMap(loop.TargetRef, "clip_id") != "clip-sub" {
		t.Fatalf("treatment target must prefer the sub clip: %#v", loop.TargetRef)
	}
	if loop.RangeGoal.Phase != freeStateRangePhaseTreating {
		t.Fatalf("phase after the round-2 handoff = %q", loop.RangeGoal.Phase)
	}
	// 单次幂等：treating 后再次绑定不改写。
	first := loop.TargetRef["clip_id"]
	applyFreeStateRangeGoalTreatmentTarget(&loop)
	if loop.TargetRef["clip_id"] != first || loop.RangeGoal.Phase != freeStateRangePhaseTreating {
		t.Fatalf("handoff must be idempotent: %#v", loop.RangeGoal)
	}
	// 非本轨目标不绑定；phase 未到 split_done 不绑定。
	wrongTrack := freeStateReasoningLoop{
		RangeGoal: &freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", Phase: freeStateRangePhaseSplitDone, SubClipID: "clip-sub"},
		TargetRef: map[string]any{"kind": "track", "id": "track-other", "track_id": "track-other"},
	}
	applyFreeStateRangeGoalTreatmentTarget(&wrongTrack)
	if _, exists := wrongTrack.TargetRef["clip_id"]; exists {
		t.Fatalf("foreign track target must not take the sub clip preference")
	}
	pending := freeStateReasoningLoop{
		RangeGoal: &freeStateRangeGoal{ClipID: "clip-1", TrackID: "track-1", Phase: freeStateRangePhaseSplitPending},
		TargetRef: map[string]any{"kind": "track", "id": "track-1", "track_id": "track-1"},
	}
	applyFreeStateRangeGoalTreatmentTarget(&pending)
	if _, exists := pending.TargetRef["clip_id"]; exists {
		t.Fatalf("split_pending must not bind a treatment target")
	}
}
