package agentloop

import (
	"strings"
	"testing"
)

// MIXTICK-CONFIRM-CONTRACT-1 裁定 A 配对单测（2026-11-06）。裁定：:292 闭合句族在
// 「观察台账中最新 needs_observation 请求的 view set 已返回 usable 证据」——与去重门
// （messageLoopFreeStateAlreadyObservedIssue，free_state_reasoning.go:1024 调用点）同一
// 判据——条件下提前进入 turn_directives，不再等待排干切片续跑注入（262 取证：T32 证明
// 该句注入即提案）。三项断言面：①条件满足→directives 含闭合句 ②条件不满足→不含
// （现行为不变）③判据与去重门同源——指令触发与去重门判定对同一状态族真值表等价，
// 复制粘贴第二套判据会被本文件钉破。

const duplicateViewSetClosureLeadIn = "The CCB view set requested in the latest needs_observation decision has already returned usable evidence in the observation ledger; the runtime refuses to repeat it."

func duplicateViewSetClosureLoop(mutate func(loop map[string]any)) map[string]any {
	loop := map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
		"continuation_budget": 6, "continuation_used": 2,
	}
	if mutate != nil {
		mutate(loop)
	}
	return loop
}

func duplicateViewSetClosureState(loop map[string]any, ctxMutate func(ctx map[string]any)) *runState {
	ctx := map[string]any{
		"task_contract":             map[string]any{"kind": "improvement"},
		"free_state_reasoning_loop": loop,
		"minimal_audio_closure": map[string]any{
			"project_uuid": "proj-1", "project_revision": "rev-7", "phase": "fs5_candidates",
			"rounds_started": 4,
			"policy":         map[string]any{"max_closure_rounds": 6},
		},
	}
	if ctxMutate != nil {
		ctxMutate(ctx)
	}
	return &runState{input: Input{Context: ctx}}
}

// ①条件满足（取证形态：T5 之后模型重请求 project 级 view set，台账已 usable）：
// 生产装配面 messageLoopNeutralFamilyTurnDirectives 提前携带闭合句族——引导句、
// 冻结预算句（含 "The evidence you hold is sufficient for a bounded proposal"）与
// needs_experiment handoff 句同时在场。
func TestDuplicateViewSetClosureDirectiveRidesTurnDirectivesWhenLedgerUsable(t *testing.T) {
	state := duplicateViewSetClosureState(duplicateViewSetClosureLoop(func(loop map[string]any) {
		loop["latest_decision"] = map[string]any{
			"schema_version": "free_state_decision.v1", "status": "needs_observation",
			"requested_view_ids": []any{"project.structure", "mix.multitrack_relationship"},
		}
		loop["observation_ledger"] = map[string]any{
			"available_views": map[string]any{
				"project.structure":            map[string]any{"status": "ready", "observation_id": "obs-1"},
				"mix.multitrack_relationship": map[string]any{"status": "ready", "observation_id": "obs-2"},
			},
		}
	}), nil)
	directives := messageLoopNeutralFamilyTurnDirectives(state)
	if !strings.Contains(directives, duplicateViewSetClosureLeadIn) {
		t.Fatalf("directives missing the duplicate-view-set closure lead-in:\n%s", directives)
	}
	if !strings.Contains(directives, "The evidence you hold is sufficient for a bounded proposal.") {
		t.Fatal("directives missing the frozen sufficiency sentence (262 T32 decisive needle)")
	}
	if !strings.Contains(directives, "use needs_experiment with one bounded improvement_proposal.v1 citing the returned observation") {
		t.Fatal("directives missing the needs_experiment handoff sentence")
	}
	if !strings.Contains(directives, "(closure rounds remaining: 2/6; continuations remaining: 4/6)") {
		t.Fatal("closure guidance must carry the authoritative budget digits, not invented numbers")
	}
}

// ②条件不满足：各现行为分支均不得渲染（view set 未在台账、latest 决策非
// needs_observation、无 requested_view_ids、非 improvement 契约、diagnostic-only、
// 终局轮锁定、experiment 运行中）。判 absence 用本指令唯一的引导句作 marker，
// 不借用预算句（预算句在别处合法在场）。
func TestDuplicateViewSetClosureDirectiveStaysSilentWithoutCondition(t *testing.T) {
	baseLoop := func(loop map[string]any) {
		loop["latest_decision"] = map[string]any{
			"schema_version": "free_state_decision.v1", "status": "needs_observation",
			"requested_view_ids": []any{"mix.multitrack_relationship"},
		}
	}
	cases := []struct {
		name string
		loop func(loop map[string]any)
		ctx  func(ctx map[string]any)
	}{
		{"view set not in ledger", func(loop map[string]any) {
			baseLoop(loop)
			loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
				"mix.frequency_relationship": map[string]any{"status": "ready"},
			}}
		}, nil},
		{"latest decision not needs_observation", func(loop map[string]any) {
			baseLoop(loop)
			decision := messageLoopMapValue(loop["latest_decision"])
			decision["status"] = "needs_experiment"
			loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
				"mix.multitrack_relationship": map[string]any{"status": "ready"},
			}}
		}, nil},
		{"no requested view ids", func(loop map[string]any) {
			baseLoop(loop)
			delete(messageLoopMapValue(loop["latest_decision"]), "requested_view_ids")
			loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
				"mix.multitrack_relationship": map[string]any{"status": "ready"},
			}}
		}, nil},
		{"no latest decision", func(loop map[string]any) {
			loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
				"mix.multitrack_relationship": map[string]any{"status": "ready"},
			}}
		}, nil},
		{"diagnostic-only loop", func(loop map[string]any) {
			baseLoop(loop)
			loop["diagnostic_only"] = true
			loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
				"mix.multitrack_relationship": map[string]any{"status": "ready"},
			}}
		}, nil},
		{"terminal turn locked", func(loop map[string]any) {
			baseLoop(loop)
			loop["terminal_turn_locked"] = true
			loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
				"mix.multitrack_relationship": map[string]any{"status": "ready"},
			}}
		}, nil},
		{"experiment running", func(loop map[string]any) {
			baseLoop(loop)
			loop["experiment"] = map[string]any{"status": "running"}
			loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
				"mix.multitrack_relationship": map[string]any{"status": "ready"},
			}}
		}, nil},
	}
	for _, tc := range cases {
		state := duplicateViewSetClosureState(duplicateViewSetClosureLoop(tc.loop), tc.ctx)
		if got := messageLoopFreeStateDuplicateViewSetClosureDirective(state); got != "" {
			t.Fatalf("%s: directive must stay silent, got %q", tc.name, got)
		}
		if directives := messageLoopNeutralFamilyTurnDirectives(state); strings.Contains(directives, duplicateViewSetClosureLeadIn) {
			t.Fatalf("%s: turn directives must not carry the duplicate-view-set lead-in", tc.name)
		}
	}
	// 非 improvement 契约（plain 组）同样不渲染。
	plain := duplicateViewSetClosureState(duplicateViewSetClosureLoop(func(loop map[string]any) {
		baseLoop(loop)
		loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
			"mix.multitrack_relationship": map[string]any{"status": "ready"},
		}}
	}), func(ctx map[string]any) {
		delete(ctx, "task_contract")
	})
	if got := messageLoopFreeStateDuplicateViewSetClosureDirective(plain); got != "" {
		t.Fatalf("plain contract must not render the closure directive, got %q", got)
	}
}

// ③判据同源：对同一状态族，指令触发 ⇔ 去重门本体（messageLoopFreeStateAlready-
// ObservedIssue）对同一 requested view set 判非空。真值表由门函数现场求值——指令侧
// 若复制粘贴出第二套 view-set 判定并发生漂移，本表即红。含再观察准入 carve-out：
// 台账 prior=partial + 决策 declared_contradiction 时门放行，指令必须同样沉默。
func TestDuplicateViewSetClosureDirectiveFollowsDedupGatePredicate(t *testing.T) {
	partialReceiptLedger := func(loop map[string]any) {
		loop["observation_ledger"] = map[string]any{
			"available_views": map[string]any{
				"mix.multitrack_relationship": map[string]any{"status": "ready", "observation_id": "obs-2"},
			},
			"receipts": []any{map[string]any{
				"requested_views": []any{"mix.multitrack_relationship"},
				"status":          "partial", "project_revision": "rev-7",
			}},
		}
	}
	cases := []struct {
		name      string
		loop      func(loop map[string]any)
		wantFires bool
	}{
		{"forensics project set bounced", func(loop map[string]any) {
			loop["latest_decision"] = map[string]any{
				"status": "needs_observation",
				"requested_view_ids": []any{"project.structure", "mix.multitrack_relationship"},
			}
			loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
				"project.structure":            map[string]any{"status": "ready"},
				"mix.multitrack_relationship": map[string]any{"status": "ready"},
			}}
		}, true},
		{"single mix view usable", func(loop map[string]any) {
			loop["latest_decision"] = map[string]any{
				"status": "needs_observation", "requested_view_ids": []any{"mix.multitrack_relationship"},
			}
			loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
				"mix.multitrack_relationship": map[string]any{"status": "ready"},
			}}
		}, true},
		{"track view target not disclosed", func(loop map[string]any) {
			loop["latest_decision"] = map[string]any{
				"status": "needs_observation", "requested_view_ids": []any{"track.basic_energy"},
			}
			loop["observation_ledger"] = map[string]any{"available_views": map[string]any{
				"track:1007::track.basic_energy": map[string]any{"status": "ready"},
			}}
		}, false},
		{"view unknown to ledger", func(loop map[string]any) {
			loop["latest_decision"] = map[string]any{
				"status": "needs_observation", "requested_view_ids": []any{"mix.frequency_relationship"},
			}
		}, false},
		{"repeat receipt without admission", func(loop map[string]any) {
			loop["latest_decision"] = map[string]any{
				"status": "needs_observation", "requested_view_ids": []any{"mix.multitrack_relationship"},
			}
			partialReceiptLedger(loop)
		}, true},
		{"repeat receipt with declared contradiction admitted", func(loop map[string]any) {
			loop["latest_decision"] = map[string]any{
				"status": "needs_observation", "requested_view_ids": []any{"mix.multitrack_relationship"},
				"declared_contradiction": true,
			}
			partialReceiptLedger(loop)
		}, false},
	}
	for _, tc := range cases {
		state := duplicateViewSetClosureState(duplicateViewSetClosureLoop(tc.loop), nil)
		directive := messageLoopFreeStateDuplicateViewSetClosureDirective(state)
		fires := directive != ""
		if fires != tc.wantFires {
			t.Fatalf("%s: fires=%v want %v (directive=%q)", tc.name, fires, tc.wantFires, directive)
		}
		// Same-source check: the gate's own verdict on the same requested set
		// (unbound target, decision metadata mirrored) must agree with the
		// directive's firing on every battery row.
		decision := messageLoopMapValue(messageLoopFreeStateContext(state)["latest_decision"])
		meta := map[string]any{}
		if reason := strings.TrimSpace(messageLoopText(decision["priority_reason"])); reason != "" {
			meta["priority_reason"] = reason
		}
		if questions := messageLoopStringList(decision["unresolved_questions"]); len(questions) > 0 {
			meta["new_unresolved_questions"] = questions
		}
		if freeStateBool(decision["declared_contradiction"]) {
			meta["declared_contradiction"] = true
		}
		gateIssue := messageLoopFreeStateAlreadyObservedIssue(
			state, messageLoopStringList(decision["requested_view_ids"]), nil, meta)
		if fires != (gateIssue != "") {
			t.Fatalf("%s: directive fires=%v but dedup gate verdict=%q — predicate forked from the gate", tc.name, fires, gateIssue)
		}
		if fires && !strings.Contains(directive, "The evidence you hold is sufficient for a bounded proposal.") {
			t.Fatalf("%s: firing directive must carry the closure guidance, got %q", tc.name, directive)
		}
	}
}

// 字节恒等锚（needle 面零回退）：:292 提炼为共享 helper 后，candidate-frontier
// 路径的输出必须与提炼前逐字节相同（取证 excerpts.md §四的 T32 注入块即此文）。
func TestCandidateFrontierClosureGuidanceByteIdenticalAfterRefactor(t *testing.T) {
	state := duplicateViewSetClosureState(duplicateViewSetClosureLoop(func(loop map[string]any) {
		loop["observation_ledger"] = map[string]any{"available_views": map[string]any{}}
	}), func(ctx map[string]any) {
		closure := messageLoopMapValue(ctx["minimal_audio_closure"])
		closure["hypothesis_frontier"] = map[string]any{
			"candidates": []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007"}}},
		}
	})
	state.recentObservation = &RecentObservation{
		Tool: "ccb.observation_request",
		Summary: map[string]any{
			"status": "ready",
			"target_ref": map[string]any{"kind": "track", "id": "1007"},
		},
	}
	want := "A target-level observation for the closure candidate has just returned in this turn. " +
		"Further observations consume window budget (closure rounds remaining: 2/6; continuations remaining: 4/6). " +
		"The evidence you hold is sufficient for a bounded proposal. " +
		"Use needs_action only when the returned target evidence supports a deterministic governed action; " +
		"when it plausibly relates to the user's listening goal but cannot prove an objective defect, " +
		"use needs_experiment with one bounded improvement_proposal.v1 citing the returned observation. " +
		"Use blocked only for a concrete capability, freshness, authorization, or observation boundary.\n\n"
	if got := messageLoopCandidateFrontierDirective(state); got != want {
		t.Fatalf("candidate-frontier closure guidance drifted from the pre-refactor bytes:\ngot  %q\nwant %q", got, want)
	}
}
