package agentloop

import (
	"strings"
	"testing"
)

// FIX-POSTACTION-GATE-FEEDBACK-1（FORENSICS-LEG-E §一 run8 子形态）：post-action
// 观察门反馈面三锚点的钉。run8 形态=mutation 前录的三视图 partial receipt
// （revision 6），mutation 后 revision 8，模型收窄请求两视图/一视图——整集
// fingerprint 豁免够不着、per-view 可用性分支无 revision 感知、弹回消息教被
// settle/blocked/needs_action/satisfied 四门封死的"decide from the recorded
// evidence"。三锚点=弹回教学阶段感知、豁免子集+新 revision 匹配、prompt 教学
// 与门行为对齐；组合钉=三者在同一 run8 形态下一致。

// postActionRun8LedgerFixture 复刻 run8 的账面：track 1032 三视图 partial
// receipt（revision 由参数给出）+ per-track available_views 行。
func postActionRun8LedgerFixture(receiptRevision string) (*runState, map[string]any) {
	loopCtx := map[string]any{
		"schema_version":                   "free_state_reasoning_loop.v1",
		"status":                           "observing",
		"decision_phase":                   "post_action_evaluation",
		"requires_post_action_observation": true,
		"observation_ledger": map[string]any{
			"receipts": []any{map[string]any{
				"status":           "partial",
				"requested_views":  []any{"track.stereo_space", "track.time_dynamics", "track.activity_structure"},
				"target_ref":       map[string]any{"kind": "track", "id": "1032", "label": "vocals"},
				"project_revision": receiptRevision,
				"observation_id":   "obs_e091826f6b60",
			}},
			"available_views": map[string]any{
				"track:1032::track.stereo_space":       map[string]any{"view_id": "track.stereo_space", "status": "partial", "observation_id": "obs_e091826f6b60"},
				"track:1032::track.time_dynamics":      map[string]any{"view_id": "track.time_dynamics", "status": "partial", "observation_id": "obs_e091826f6b60"},
				"track:1032::track.activity_structure": map[string]any{"view_id": "track.activity_structure", "status": "partial", "observation_id": "obs_e091826f6b60"},
			},
		},
	}
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": loopCtx}}}
	return state, loopCtx
}

// 钉 1（锚点 1）：requires_post_action_observation=true 时 per-track 弹回消息
// 不含"decide from the recorded evidence"（四门封死的不可达出口），只教两条
// 可达出口（豁免形状重取整集/子集、换 view/target）；阶段外该出口真实可达，
// 旧教学保持。
func TestPostActionPerTrackBounceTeachesReachableExitsOnly(t *testing.T) {
	state, loopCtx := postActionRun8LedgerFixture("6")
	loopCtx["project_revision"] = "6" // revision 未前进：豁免不成立，弹回必须发生
	target := map[string]any{"kind": "track", "id": "1032", "label": "vocals"}
	requested := []string{"track.stereo_space"}

	issue := messageLoopFreeStateAlreadyObservedIssue(state, requested, target)
	if issue == "" {
		t.Fatal("same-revision narrowed repeat stopped bouncing in the post-action phase")
	}
	if strings.Contains(issue, "decide from the recorded evidence") {
		t.Fatalf("post-action bounce still teaches the structurally unreachable recorded-evidence exit: %q", issue)
	}
	for _, reachable := range []string{"the full set or a narrowed subset", "different cataloged view or target"} {
		if !strings.Contains(issue, reachable) {
			t.Fatalf("post-action bounce missing reachable exit %q: %q", reachable, issue)
		}
	}

	// 阶段外（requires_post_action_observation=false）recorded evidence 出口
	// 真实可达（四门只在 post-action 阶段要求本周期新观察），旧教学必须原样。
	loopCtx["requires_post_action_observation"] = false
	issue = messageLoopFreeStateAlreadyObservedIssue(state, requested, target)
	if !strings.Contains(issue, "decide from the recorded evidence") {
		t.Fatalf("non-post-action bounce lost the legal recorded-evidence teaching: %q", issue)
	}
}

// 钉 2（锚点 2）：子集+新 revision=豁免放行；子集+同/旧 revision=维持弹回；
// 非子集（真换 view）不需要豁免；豁免以"最新覆盖 receipt"为锚防重复浪费。
func TestNarrowedViewSetReObservationAdmittedOnStrictlyNewerRevision(t *testing.T) {
	state, loopCtx := postActionRun8LedgerFixture("6")
	target := map[string]any{"kind": "track", "id": "1032"}
	narrowed := []string{"track.stereo_space", "track.time_dynamics"}

	// 子集 + 新 revision（run8 形态 6→8）→ 放行，无弹回。
	loopCtx["project_revision"] = "8"
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, narrowed, target); issue != "" {
		t.Fatalf("narrowed subset on newer revision was not admitted: %q", issue)
	}
	// rev- 前缀形状同样按尾数排序（测试与既有 fixture 的 "rev-7"/"rev-8" 形状对齐）。
	loopCtx["observation_ledger"].(map[string]any)["receipts"].([]any)[0].(map[string]any)["project_revision"] = "rev-6"
	loopCtx["project_revision"] = "rev-8"
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, narrowed, target); issue != "" {
		t.Fatalf("rev-prefixed narrowed subset on newer revision was not admitted: %q", issue)
	}
	// 子集 + 同 revision / 旧 revision → 维持弹回。
	for _, current := range []string{"rev-6", "rev-5"} {
		loopCtx["project_revision"] = current
		if issue := messageLoopFreeStateAlreadyObservedIssue(state, narrowed, target); issue == "" {
			t.Fatalf("narrowed subset on non-newer revision %q was admitted", current)
		}
	}
	// 非子集（band_dynamics 无任何 receipt 覆盖）→ 真换 view，不需要豁免。
	different := []string{"track.stereo_space", "track.band_dynamics"}
	loopCtx["project_revision"] = "rev-6"
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, different, target); issue != "" {
		t.Fatalf("view outside the recorded receipts was not its own request: %q", issue)
	}

	// 防浪费锚：两个覆盖 receipt（rev-5 三视图、rev-9 两视图）时豁免以最新
	// 覆盖为准——rev-9 已在当前 revision 录过 stereo_space，重取它=重复观察，
	// 必须弹回；revision 前进到 rev-10 才放行。
	state, loopCtx = postActionRun8LedgerFixture("rev-5")
	ledger := loopCtx["observation_ledger"].(map[string]any)
	ledger["receipts"] = []any{
		ledger["receipts"].([]any)[0],
		map[string]any{
			"status":           "ready",
			"requested_views":  []any{"track.stereo_space", "track.time_dynamics"},
			"target_ref":       map[string]any{"kind": "track", "id": "1032"},
			"project_revision": "rev-9",
			"observation_id":   "obs_newest_covering",
		},
	}
	single := []string{"track.stereo_space"}
	loopCtx["project_revision"] = "rev-9"
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, single, target); issue == "" {
		t.Fatal("older covering receipt unlocked a set the newest covering receipt already recorded at the current revision")
	}
	loopCtx["project_revision"] = "rev-10"
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, single, target); issue != "" {
		t.Fatalf("narrowed subset on revision newer than the newest covering receipt was not admitted: %q", issue)
	}
	// 无 revision 绑定的覆盖 receipt 不可排序 → fail-closed 维持弹回。
	state, loopCtx = postActionRun8LedgerFixture("")
	loopCtx["project_revision"] = "8"
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, single, target); issue == "" {
		t.Fatal("narrowed subset over an order-less covering receipt was admitted")
	}
}

// 钉 3（锚点 3）：生产 prompt（ruleset embed 渲染）的 post_action 教学必须只
// 教豁免可收的形状（整集重取或子集收窄+新 revision，明说收窄允许），不得再
// 教被门弹回的"experiment's view set"指令；新鲜观察要求本身不弱化。
func TestPostActionPromptTeachesAdmittedReObservationShapes(t *testing.T) {
	state := neutralFamilyTestState(nil)
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	skeleton := messageLoopNeutralFamilySystemSkeleton(state)
	if skeleton == "" {
		t.Fatal("neutral-family skeleton rendered empty")
	}
	if strings.Contains(skeleton, "for the admitted experiment's view set") {
		t.Fatal("post-action teaching still names the pre-narrowing 'experiment view set' directive the gate bounces")
	}
	for _, anchor := range []string{
		"full recorded view set",
		"narrow it to a subset",
		"narrowing to a subset is allowed",
		"newer than that receipt",
		"A view outside the recorded receipts is a different view set",
	} {
		if !strings.Contains(skeleton, anchor) {
			t.Fatalf("post-action teaching lacks exemption-shape anchor %q", anchor)
		}
	}
	// 证据门不弱化：post-action 段首动作仍是本周期必须返回的观察请求。
	if !strings.Contains(skeleton, "requires_post_action_observation=true your FIRST action must be a ccb.observation_request") {
		t.Fatal("post-action teaching lost the mandatory fresh-observation requirement")
	}
	if !strings.Contains(skeleton, "No runtime outcome, needs_action, or settle report is legal until that request has returned in this reasoning cycle") {
		t.Fatal("post-action teaching lost the settle/needs_action ban before the observation returns")
	}
}

// 钉 4（组合）：run8 完整形态下三锚点一致——revision 前进后收窄请求与整集
// 重取都放行（run8 死锁解除）；revision 未前进时弹回只教可达出口，且弹回与
// prompt 教的是同一组出口。
func TestPostActionNarrowedRequestCombinedShape(t *testing.T) {
	state, loopCtx := postActionRun8LedgerFixture("6")
	target := map[string]any{"kind": "track", "id": "1032", "label": "vocals"}
	narrowed := []string{"track.stereo_space"}
	fullSet := []string{"track.stereo_space", "track.time_dynamics", "track.activity_structure"}

	// mutation 后（6→8）：收窄请求经子集豁免放行；整集重取经既有新 revision
	// 豁免放行——两条可达出口都真实可达。
	loopCtx["project_revision"] = "8"
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, narrowed, target); issue != "" {
		t.Fatalf("combined post-action narrowed request still bounced: %q", issue)
	}
	if issue := messageLoopFreeStateAlreadyObservedIssue(state, fullSet, target); issue != "" {
		t.Fatalf("combined post-action full-set re-request still bounced: %q", issue)
	}

	// revision 未前进：弹回只教可达出口；prompt 与弹回教同一组出口形状。
	loopCtx["project_revision"] = "6"
	issue := messageLoopFreeStateAlreadyObservedIssue(state, narrowed, target)
	if issue == "" || strings.Contains(issue, "decide from the recorded evidence") {
		t.Fatalf("combined same-revision bounce missing or teaching the unreachable exit: %q", issue)
	}
	promptState := neutralFamilyTestState(nil)
	promptState.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	skeleton := messageLoopNeutralFamilySystemSkeleton(promptState)
	for _, shared := range []string{"subset", "full"} {
		if !strings.Contains(issue, shared) || !strings.Contains(skeleton, shared) {
			t.Fatalf("bounce and prompt disagree on the %q exemption shape (bounce=%q)", shared, issue)
		}
	}
}
