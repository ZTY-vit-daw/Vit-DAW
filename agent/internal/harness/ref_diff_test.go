package harness

// L1-3-IMPL-D 工具面测试（QUERY_ENGINE §6.2 T7 + content 委托面）。
//
// 三层：
//  1. 引擎级 explicit-refs：added/removed/changed/unchanged 四类精确命中
//     （T7 本体——显式 ref 集不受 bootstrap 结构约束，removed 可命中）；
//  2. harness fixture（真实 BootstrapStore 扫隔离临时面，AGENTS §10）：三类
//     命中 + 委托映射（承载者/句柄/matches_base/实例身份注记）。bootstrap
//     单盘态语义下 removed 结构性恒空（base exact 视图=同一次 latest 扫描的
//     子集）——removed 类由第 1 层覆盖，此处如实断言恒空并注记；
//  3. 空 latest 短路（content 深度响应契约：compile/delegated 空面如实）。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/queryengine"
)

func newRefDiffContentHarness(t *testing.T) (*Harness, string, string) {
	t.Helper()
	h := NewWithSender(nil, nil, nil)
	agentRoot := t.TempDir()
	comDir := t.TempDir()
	h.refQueryConfigOverride = &queryengine.BootstrapConfig{
		AgentRoot:      agentRoot,
		COMEvidenceDir: comDir,
	}
	return h, agentRoot, comDir
}

// writeRefDiffObservationTicket 写最小可扫描观察票（bootstrap 只解析
// observation_id/target_ref/三投影；mix_package 仅供 harness 委托面的
// before_after 扫描——两解析面独立，fixture 双面都给）。
func writeRefDiffObservationTicket(t *testing.T, agentRoot, obsID, trackID, fxmID, comID, domStatus string, beforeAfterFrom string) string {
	t.Helper()
	dir := filepath.Join(agentRoot, "s1", "observations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ticket := map[string]any{
		"schema_version": "mixboard.observation.v1",
		"observation_id": obsID,
		"target_ref":     map[string]any{"kind": "track", "id": trackID, "label": trackID},
	}
	if fxmID != "" {
		ticket["fxm_projection"] = map[string]any{
			"schema_version": "fxm.projection.v0",
			"projection_id":  fxmID,
			"status":         "ready",
			"target_ref":     map[string]any{"kind": "track", "id": trackID},
			"baseline_ref":   map[string]any{"id": "m1", "stage": "bypass", "render_revision": "r1"},
			"processed_ref":  map[string]any{"id": "m2", "stage": "processed", "render_revision": "r2"},
			"effect_delta":   map[string]any{"lufs_delta_db": -1.5},
		}
	}
	if comID != "" {
		ticket["com_projection"] = map[string]any{
			"schema_version": "com.projection.v0",
			"projection_id":  comID,
			"mode":           "change_delta",
			"status":         "ready",
			"target_ref":     map[string]any{"kind": "track", "id": trackID},
			"behavior_change": map[string]any{
				"status":               "changed",
				"before_projection_id": comID + "-before",
				"after_projection_id":  comID + "-after",
			},
		}
	}
	if domStatus != "" {
		ticket["dom_projection"] = map[string]any{"schema_version": "dom.projection.v1", "status": domStatus}
	}
	if beforeAfterFrom != "" {
		ticket["mix_package"] = map[string]any{"current_metrics": map[string]any{
			"before_after_delta": map[string]any{
				"status":                "ready",
				"before_observation_id": beforeAfterFrom,
				"after_observation_id":  obsID,
			},
			"ab_result": map[string]any{
				"schema_version":         "mom_ab_result.v1",
				"status":                 "ready",
				"before_observation_id":  beforeAfterFrom,
				"after_observation_id":   obsID,
				"before_render_revision": "r1",
				"after_render_revision":  "r2",
				"tap_point":              "master",
				"render_mode":            "offline",
				"quality_gates":          map[string]any{"same_tap_point": true, "render_revision_changed": true},
			},
		}}
	}
	return writeRefDiffJSON(t, filepath.Join(dir, obsID+".json"), ticket)
}

func writeRefDiffPairArtifact(t *testing.T, comDir, pairID, trackID string) string {
	t.Helper()
	dir := filepath.Join(comDir, pairID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return writeRefDiffJSON(t, filepath.Join(dir, pairID+".json"), map[string]any{
		"pair_id":         pairID,
		"schema_version":  "com.paired.v1",
		"processor_scope": map[string]any{"track_id": trackID, "plugin_instance_id": "p_" + pairID},
	})
}

func writeRefDiffJSON(t *testing.T, path string, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func refDiffStringSlice(t *testing.T, raw any) []string {
	t.Helper()
	slice, ok := raw.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", raw)
	}
	return slice
}

func refDiffChangedOfKind(t *testing.T, changed any, kind string) map[string]any {
	t.Helper()
	entries, ok := changed.([]map[string]any)
	if !ok {
		t.Fatalf("changed is not []map[string]any: %T", changed)
	}
	for _, entry := range entries {
		if entry["kind"] == kind {
			return entry
		}
	}
	t.Fatalf("no changed entry for kind %q in %+v", kind, changed)
	return nil
}

// ---- 1. 引擎级 T7：四类精确命中（explicit refs）----

func TestT7DiffIdentityExactClassification(t *testing.T) {
	engine := queryengine.NewEngine(nil, nil)
	const (
		changedBase = "vit://fxm/track:T1/t=all@obs_a#sha256:0000000000000001"
		changedHead = "vit://fxm/track:T1/t=all@obs_b#sha256:0000000000000009"
		unchanged   = "vit://com/track:T2/t=all@obs_a#sha256:0000000000000002"
		removed     = "vit://fxm/track:T3/t=all@obs_a#sha256:0000000000000003"
		added       = "vit://fxm/track:T4/t=all@obs_b#sha256:0000000000000004"
	)
	report, err := engine.DiffEvidence(context.Background(), queryengine.DiffRequest{
		Base:  queryengine.SnapshotRefSet{Refs: []string{changedBase, unchanged, removed}},
		Head:  queryengine.SnapshotRefSet{Refs: []string{changedHead, unchanged, added}},
		Depth: queryengine.DiffIdentity,
	})
	if err != nil {
		t.Fatalf("explicit-refs diff rejected: %v", err)
	}
	if got := report.Added; len(got) != 1 || got[0] != added {
		t.Fatalf("Added = %v, want exactly [%s]", got, added)
	}
	if got := report.Removed; len(got) != 1 || got[0] != removed {
		t.Fatalf("Removed = %v, want exactly [%s]", got, removed)
	}
	if len(report.Changed) != 1 || report.Changed[0].Base != changedBase || report.Changed[0].Head != changedHead {
		t.Fatalf("Changed = %+v, want exactly pair base=%s head=%s", report.Changed, changedBase, changedHead)
	}
	if report.UnchangedCount != 1 {
		t.Fatalf("UnchangedCount = %d, want 1 (%s)", report.UnchangedCount, unchanged)
	}
	if report.CostClass != queryengine.CostClassIndex {
		t.Fatalf("identity diff CostClass = %q, want index", report.CostClass)
	}
}

// ---- 2. harness fixture：content 委托面 + 实例身份注记 ----

// 坐标布局（fixture 确定性依据：coordMap 去重取 canonical 最大——@obs_b/@pair_b
// 字典序大于 @obs_a/@pair_a，head 侧幸存行确定性指向 b 侧）：
//
//	track:T3  fxm+com：obs_a（base 侧）→ obs_b（head 侧）＝ changed ×2（A 轮）
//	track:T3  dad.compressor_dual_tap：pair_a → pair_b ＝ changed（B 轮）
//	track:T9  fxm(obs_c)+dom(obs_c, snapshot=current)＝ added（A 轮；dom 无承载者→unrouted）
//	track:T7  fxm(obs_x) 单行＝ unchanged（C 轮）
func TestRefDiffContentDelegationFixture(t *testing.T) {
	h, agentRoot, comDir := newRefDiffContentHarness(t)
	ctx := context.Background()
	ticketA := writeRefDiffObservationTicket(t, agentRoot, "obs_a", "T3", "fxm_a", "com_a", "", "")
	ticketB := writeRefDiffObservationTicket(t, agentRoot, "obs_b", "T3", "fxm_b", "com_b", "", "obs_a")
	ticketC := writeRefDiffObservationTicket(t, agentRoot, "obs_c", "T9", "fxm_c", "", "ready", "")
	writeRefDiffObservationTicket(t, agentRoot, "obs_x", "T7", "fxm_x", "", "", "")
	pairA := writeRefDiffPairArtifact(t, comDir, "pair_a", "T3")
	pairB := writeRefDiffPairArtifact(t, comDir, "pair_b", "T3")
	_ = ticketC // 落盘即作用（fxm/dom 行）；句柄经坐标断言间接核

	// A 轮：base_observation_id=obs_a，depth=content。
	result, err := h.refDiff(ctx, map[string]any{
		"cmd": "ref_diff", "base_observation_id": "obs_a", "depth": "content",
	})
	if err != nil {
		t.Fatalf("content diff rejected: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("status = %v", result["status"])
	}
	if result["cost_class"] != queryengine.CostClassCompile {
		t.Fatalf("content diff cost_class = %v, want compile (R10)", result["cost_class"])
	}
	if result["degraded"] != "" {
		t.Fatalf("content diff degraded = %v, want empty (委托是设计路径非降级)", result["degraded"])
	}

	// 实例身份注记（T7）：fxm/com 的 changed 带 hash_semantics；内容判定走委托。
	for _, kind := range []string{"fxm", "com"} {
		entry := refDiffChangedOfKind(t, result["changed"], kind)
		if entry["hash_semantics"] != "instance_identity" {
			t.Fatalf("changed %q entry missing instance_identity annotation: %+v", kind, entry)
		}
		if base, _ := entry["base"].(string); !strings.Contains(base, "@obs_a#") {
			t.Fatalf("changed %q base ref = %v, want snapshot @obs_a", kind, entry["base"])
		}
		if head, _ := entry["head"].(string); !strings.Contains(head, "@obs_b#") {
			t.Fatalf("changed %q head ref = %v, want snapshot @obs_b", kind, entry["head"])
		}
	}
	// dom（内容身份 hash）不注记：本 fixture dom 行走 added，unrouted 如实报 dom。
	if got := refDiffStringSlice(t, result["unrouted_kinds"]); len(got) != 1 || got[0] != "dom" {
		t.Fatalf("unrouted_kinds = %v, want exactly [dom]", got)
	}

	delegated, ok := result["delegated"].(map[string]any)
	if !ok {
		t.Fatalf("delegated missing or wrong type: %T", result["delegated"])
	}
	// COM change_delta 承载者：changed 对双句柄=前后票路径。
	comCarrier, ok := delegated[refDiffCarrierComChangeDelta].(map[string]any)
	if !ok {
		t.Fatalf("delegated missing %s carrier: %+v", refDiffCarrierComChangeDelta, delegated)
	}
	comPairs, _ := comCarrier["pairs"].([]map[string]any)
	if len(comPairs) != 1 || comPairs[0]["base_handle"] != ticketA || comPairs[0]["head_handle"] != ticketB {
		t.Fatalf("com.change_delta pairs = %+v, want base_handle=%s head_handle=%s", comPairs, ticketA, ticketB)
	}
	// FXM A/B 承载者：同票对。
	fxmCarrier, ok := delegated[refDiffCarrierFXMAB].(map[string]any)
	if !ok {
		t.Fatalf("delegated missing %s carrier: %+v", refDiffCarrierFXMAB, delegated)
	}
	fxmPairs, _ := fxmCarrier["pairs"].([]map[string]any)
	if len(fxmPairs) != 1 || fxmPairs[0]["base_handle"] != ticketA || fxmPairs[0]["head_handle"] != ticketB {
		t.Fatalf("fxm.ab pairs = %+v, want base_handle=%s head_handle=%s", fxmPairs, ticketA, ticketB)
	}
	// before_after 承载者：obs_b 票（before=obs_a=base → matches_base=true）。
	baCarrier, ok := delegated[refDiffCarrierBeforeAfter].(map[string]any)
	if !ok {
		t.Fatalf("delegated missing %s carrier: %+v", refDiffCarrierBeforeAfter, delegated)
	}
	baTickets, _ := baCarrier["tickets"].([]map[string]any)
	if len(baTickets) != 1 {
		t.Fatalf("before_after tickets = %+v, want exactly the obs_b ticket", baTickets)
	}
	if baTickets[0]["handle"] != ticketB || baTickets[0]["matches_base"] != true {
		t.Fatalf("before_after ticket = %+v, want handle=%s matches_base=true", baTickets[0], ticketB)
	}
	ab, _ := baTickets[0]["ab_result"].(map[string]any)
	if ab["before_observation_id"] != "obs_a" || ab["quality_gates"] == nil {
		t.Fatalf("before_after ab_result anchor = %+v, want before=obs_a + quality_gates", ab)
	}

	// B 轮：base_revision=pair_a → pair 工件 changed 对（双工件句柄）。
	resultB, err := h.refDiff(ctx, map[string]any{
		"cmd": "ref_diff", "base_revision": "pair_a", "depth": "content",
	})
	if err != nil {
		t.Fatalf("content diff (pair base) rejected: %v", err)
	}
	pairEntry := refDiffChangedOfKind(t, resultB["changed"], "dad.compressor_dual_tap")
	if _, annotated := pairEntry["hash_semantics"]; annotated {
		t.Fatalf("pair artifact hash is content identity (file bytes); must not carry instance_identity: %+v", pairEntry)
	}
	if head, _ := pairEntry["head"].(string); !strings.Contains(head, "@pair_b#") {
		t.Fatalf("pair changed head = %v, want surviving @pair_b row", pairEntry["head"])
	}
	delegatedB, _ := resultB["delegated"].(map[string]any)
	pairedCarrier, ok := delegatedB[refDiffCarrierComPaired].(map[string]any)
	if !ok {
		t.Fatalf("delegated missing %s carrier: %+v", refDiffCarrierComPaired, delegatedB)
	}
	paired, _ := pairedCarrier["pairs"].([]map[string]any)
	if len(paired) != 1 || paired[0]["base_handle"] != pairA || paired[0]["head_handle"] != pairB {
		t.Fatalf("com.paired_artifact pairs = %+v, want base_handle=%s head_handle=%s", paired, pairA, pairB)
	}

	// C 轮：base_observation_id=obs_x → T7-fxm 单坐标 unchanged；removed 在
	// bootstrap 单盘态结构性恒空（base exact=latest 同次扫描子集）——removed 类
	// 由引擎级 explicit-refs 测试覆盖，此处锁定该结构语义不静默变化。
	resultC, err := h.refDiff(ctx, map[string]any{
		"cmd": "ref_diff", "base_observation_id": "obs_x",
	})
	if err != nil {
		t.Fatalf("identity diff (obs_x) rejected: %v", err)
	}
	if resultC["unchanged_count"] != 1 {
		t.Fatalf("unchanged_count = %v, want 1 (fxm@T7)", resultC["unchanged_count"])
	}
	if got := refDiffStringSlice(t, resultC["removed"]); len(got) != 0 {
		t.Fatalf("removed = %v, want empty (bootstrap: base exact ⊆ latest)", got)
	}
	if resultC["cost_class"] != queryengine.CostClassIndex {
		t.Fatalf("identity diff cost_class = %v, want index", resultC["cost_class"])
	}

	// A' 轮：同 base、depth=identity——实例身份注记在两深度一致（防误读面），
	// 且 identity 响应不带 delegated/unrouted 面（深度契约分离）。
	resultA, err := h.refDiff(ctx, map[string]any{
		"cmd": "ref_diff", "base_observation_id": "obs_a", "depth": "identity",
	})
	if err != nil {
		t.Fatalf("identity diff (obs_a) rejected: %v", err)
	}
	entryA := refDiffChangedOfKind(t, resultA["changed"], "fxm")
	if entryA["hash_semantics"] != "instance_identity" {
		t.Fatalf("identity-depth changed fxm missing annotation: %+v", entryA)
	}
	if _, present := resultA["delegated"]; present {
		t.Fatalf("identity response must not carry delegated surface: %+v", resultA["delegated"])
	}
	if resultA["cost_class"] != queryengine.CostClassIndex {
		t.Fatalf("identity diff cost_class = %v, want index", resultA["cost_class"])
	}
}

// ---- 3. 空 latest 短路（content 深度契约）----

func TestRefDiffContentEmptyHeadShortCircuit(t *testing.T) {
	h := newRefQueryTestHarness(t) // 空临时扫描面：latest 如实空
	result, err := h.refDiff(context.Background(), map[string]any{
		"cmd": "ref_diff", "base_revision": "current", "depth": "content",
	})
	if err != nil {
		t.Fatalf("content diff on empty store rejected: %v", err)
	}
	if result["status"] != "ok" || result["unchanged_count"] != 0 {
		t.Fatalf("empty content diff = %+v, want ok + unchanged_count=0", result)
	}
	if result["cost_class"] != queryengine.CostClassCompile {
		t.Fatalf("empty content diff cost_class = %v, want compile (R10 按请求深度)", result["cost_class"])
	}
	delegated, ok := result["delegated"].(map[string]any)
	if !ok || len(delegated) != 0 {
		t.Fatalf("empty content diff delegated = %#v, want empty non-nil map", result["delegated"])
	}
	if got := refDiffStringSlice(t, result["unrouted_kinds"]); len(got) != 0 {
		t.Fatalf("empty content diff unrouted_kinds = %v, want empty", got)
	}
	if result["degraded"] != "" {
		t.Fatalf("empty content diff degraded = %v, want empty", result["degraded"])
	}
}
