package harness

// L1-3-IMPL-C 工具面测试（QUERY_ENGINE §6.4 T10/T11）。
//
// T10：ref.query/ref.diff 的 schema 校验——未知字段 / 越界 limit / 空谓词
// 全空集（至少一段谓词必填，防"全库拉取"）三组反例全部拒绝；
// T11：降级路径响应必带 degraded+compile（query 半边）＋ observe 半边
// （actual_cost_class+recomputed）联合断言（CCB-PARAM 已合入，§2.6 响应字段）。

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/queryengine"
	"vit-daw-agent/internal/tools"
)

// newRefQueryTestHarness 构造指向隔离临时扫描面的 harness（不触源码树，
// AGENTS §10；空目录=合法盘面：bootstrap 如实空行集）。
func newRefQueryTestHarness(t *testing.T) *Harness {
	t.Helper()
	h := NewWithSender(nil, nil, nil)
	h.refQueryConfigOverride = &queryengine.BootstrapConfig{
		AgentRoot: t.TempDir(),
	}
	return h
}

func refQueryResultRowMap(result map[string]any, key string) any {
	if result == nil {
		return nil
	}
	return result[key]
}

// ---- CommandSpec 注册与同源 description（T9 口径延伸）----

func TestRefQueryCommandSpecRegistered(t *testing.T) {
	catalog := tools.DefaultCatalog()
	spec, ok := catalog.LookupCommand("ref_query")
	if !ok {
		t.Fatal("ref_query CommandSpec is not registered in the default catalog")
	}
	if spec.ToolName != "ref.query" {
		t.Fatalf("ref_query tool name = %q, want ref.query", spec.ToolName)
	}
	if spec.Category != "mix" {
		t.Fatalf("ref_query category = %q, want mix", spec.Category)
	}
	if spec.RiskLevel != tools.RiskDirect {
		t.Fatalf("ref_query risk = %q, want direct (read-only)", spec.RiskLevel)
	}
	if spec.MutatesProject {
		t.Fatal("ref_query must not mutate the project")
	}
	if _, ok := catalog.LookupTool("ref.query"); !ok {
		t.Fatal("ref.query tool lookup failed")
	}

	diffSpec, ok := catalog.LookupCommand("ref_diff")
	if !ok {
		t.Fatal("ref_diff CommandSpec is not registered in the default catalog")
	}
	if diffSpec.ToolName != "ref.diff" {
		t.Fatalf("ref_diff tool name = %q, want ref.diff", diffSpec.ToolName)
	}
	if diffSpec.RiskLevel != tools.RiskDirect || diffSpec.MutatesProject {
		t.Fatal("ref.diff must be registered read-only/direct")
	}
	if _, ok := catalog.LookupTool("ref.diff"); !ok {
		t.Fatal("ref.diff tool lookup failed")
	}
}

// TestRefQueryDescriptionSameSourceAsEngineCatalog description 必须是
// BuildRefQuerySchemaDescription 的产物（不得手写第二份目录文案，双写漂移）。
func TestRefQueryDescriptionSameSourceAsEngineCatalog(t *testing.T) {
	spec, ok := tools.DefaultCatalog().LookupCommand("ref_query")
	if !ok {
		t.Fatal("ref_query not registered")
	}
	want := queryengine.BuildRefQuerySchemaDescription(queryengine.NewEngine(nil, nil).Catalog())
	if !strings.Contains(spec.Description, want) {
		t.Fatalf("ref.query description is not generated from Engine.Catalog (T9 same-source); want substring:\n%s", want)
	}
}

// ---- T10：三组反例全红绿 ----

func TestRefQueryT10RejectsUnknownField(t *testing.T) {
	h := newRefQueryTestHarness(t)
	_, err := h.refQuery(context.Background(), map[string]any{
		"cmd": "ref_query", "kinds": []string{"dom"}, "kind": []string{"dom"},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field must be rejected, got err=%v", err)
	}
}

// TestRefQueryToleratesHarnessManagedKeys 真栈烟测 R3 抓到的集成面：Invoke
// 管线在 invokeLocal 前注入 agent_action_id/tool_call_id 等执行元数据——
// 这些是 harness 管理键不是模型参数，不得触发未知字段拒绝。
func TestRefQueryToleratesHarnessManagedKeys(t *testing.T) {
	h := newRefQueryTestHarness(t)
	result, err := h.refQuery(context.Background(), map[string]any{
		"cmd":             "ref_query",
		"kinds":           []string{"dom"},
		"agent_action_id": "act_1",
		"tool_call_id":    "call_1",
		"run_id":          "run_1",
		"undo_label":      "undo",
		"confirmation":    true,
	})
	if err != nil {
		t.Fatalf("harness-managed metadata keys must not trip the unknown-field check: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("status = %v, want ok", result["status"])
	}
}

func TestRefQueryT10RejectsOutOfRangeLimit(t *testing.T) {
	h := newRefQueryTestHarness(t)
	for _, limit := range []any{501, 0, -3} {
		_, err := h.refQuery(context.Background(), map[string]any{
			"cmd": "ref_query", "kinds": []string{"dom"}, "limit": limit,
		})
		if err == nil || !strings.Contains(err.Error(), "limit") {
			t.Fatalf("limit=%v must be rejected as out of range, got err=%v", limit, err)
		}
	}
}

func TestRefQueryT10RejectsEmptyPredicateSet(t *testing.T) {
	h := newRefQueryTestHarness(t)
	for name, cmd := range map[string]map[string]any{
		"bare":       {"cmd": "ref_query"},
		"limit only": {"cmd": "ref_query", "limit": 10},
		"all window": {"cmd": "ref_query", "time_window": "all"},
	} {
		_, err := h.refQuery(context.Background(), cmd)
		if err == nil || !strings.Contains(err.Error(), "empty predicate") {
			t.Fatalf("%s: full-store pull must be rejected (at least one predicate segment required), got err=%v", name, err)
		}
	}
}

func TestRefQueryPositiveControlPasses(t *testing.T) {
	h := newRefQueryTestHarness(t)
	result, err := h.refQuery(context.Background(), map[string]any{
		"cmd": "ref_query", "kinds": []string{"dom"}, "limit": 10,
	})
	if err != nil {
		t.Fatalf("valid kinds query rejected: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("valid kinds query status = %v, want ok", result["status"])
	}
}

// ---- T11：成本标注（query 半边 + observe 半边联合）----

func TestRefQueryT11DegradedPathCarriesCompileAndDegraded(t *testing.T) {
	h := newRefQueryTestHarness(t)
	result, err := h.refQuery(context.Background(), map[string]any{
		"cmd":   "ref_query",
		"kinds": []string{"dom"},
		"payload_conditions": []any{
			map[string]any{"field": "dom.peak_structure.readiness", "op": "eq", "str": "ready"},
		},
	})
	if err != nil {
		t.Fatalf("payload-conditioned query rejected: %v", err)
	}
	if got := refQueryResultRowMap(result, "cost_class"); got != queryengine.CostClassCompile {
		t.Fatalf("degraded path cost_class = %v, want compile", got)
	}
	if got := refQueryResultRowMap(result, "degraded"); got != queryengine.DegradedPayloadIndexUnavailable {
		t.Fatalf("degraded path degraded = %v, want %s", got, queryengine.DegradedPayloadIndexUnavailable)
	}
}

func TestRefQueryT11IndexPathCarriesIndexAndNoDegradation(t *testing.T) {
	h := newRefQueryTestHarness(t)
	result, err := h.refQuery(context.Background(), map[string]any{
		"cmd": "ref_query", "kinds": []string{"dom"},
	})
	if err != nil {
		t.Fatalf("segment query rejected: %v", err)
	}
	if got := refQueryResultRowMap(result, "cost_class"); got != queryengine.CostClassIndex {
		t.Fatalf("index path cost_class = %v, want index", got)
	}
	if got, _ := refQueryResultRowMap(result, "degraded").(string); got != "" {
		t.Fatalf("index path degraded = %q, want empty", got)
	}
}

func TestRefQueryResponseCarriesFiveCostMetadata(t *testing.T) {
	h := newRefQueryTestHarness(t)
	result, err := h.refQuery(context.Background(), map[string]any{
		"cmd": "ref_query", "kinds": []string{"dom"},
	})
	if err != nil {
		t.Fatalf("query rejected: %v", err)
	}
	for _, key := range []string{"cost_class", "degraded", "snapshot", "next_cursor", "total_matches"} {
		if _, ok := result[key]; !ok {
			t.Fatalf("ref.query response is missing required metadata %q (QUERY_ENGINE §2.3 五元数据)", key)
		}
	}
}

// TestRefQueryInvokeLocalDispatch 工具名分发真接线（不是只测 Go 函数面）。
func TestRefQueryInvokeLocalDispatch(t *testing.T) {
	h := newRefQueryTestHarness(t)
	spec, ok := tools.DefaultCatalog().LookupCommand("ref_query")
	if !ok {
		t.Fatal("ref_query not registered")
	}
	result, handled := h.invokeLocal(context.Background(), spec, map[string]any{
		"cmd": "ref_query", "kinds": []string{"dom"},
	}, nil, "test")
	if !handled {
		t.Fatal("ref_query is not dispatched by invokeLocal")
	}
	if result["status"] != "ok" {
		t.Fatalf("invokeLocal ref_query status = %v, want ok", result["status"])
	}
}

// ---- ref.diff 校验（T10 同族 + identity happy path）----

func TestRefDiffValidation(t *testing.T) {
	h := newRefQueryTestHarness(t)
	ctx := context.Background()

	// base 侧三选一：全空与双设都拒绝（fail-closed）。
	if _, err := h.refDiff(ctx, map[string]any{"cmd": "ref_diff"}); err == nil || !strings.Contains(err.Error(), "base") {
		t.Fatalf("missing base selector must be rejected, got err=%v", err)
	}
	if _, err := h.refDiff(ctx, map[string]any{
		"cmd": "ref_diff", "base_revision": "current", "base_observation_id": "obs_1",
	}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("double base selector must be rejected, got err=%v", err)
	}
	// head v1 恒 latest。
	if _, err := h.refDiff(ctx, map[string]any{
		"cmd": "ref_diff", "base_revision": "current", "head": "yesterday",
	}); err == nil || !strings.Contains(err.Error(), "head") {
		t.Fatalf("non-latest head must be rejected in v1, got err=%v", err)
	}
	// depth=content IMPL-D 已接线（正例归 ref_diff_test.go 委托面）；未知深度
	// 仍显式拒绝（fail-closed，不静默降级为 identity）。
	if _, err := h.refDiff(ctx, map[string]any{
		"cmd": "ref_diff", "base_revision": "current", "depth": "contents",
	}); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("unknown depth must be rejected, got err=%v", err)
	}
	// 未知字段。
	if _, err := h.refDiff(ctx, map[string]any{
		"cmd": "ref_diff", "base_revision": "current", "mystery": 1,
	}); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field must be rejected, got err=%v", err)
	}
}

func TestRefDiffIdentityHappyPath(t *testing.T) {
	h := newRefQueryTestHarness(t)
	result, err := h.refDiff(context.Background(), map[string]any{
		"cmd": "ref_diff", "base_revision": "current",
	})
	if err != nil {
		t.Fatalf("identity diff rejected: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("identity diff status = %v, want ok", result["status"])
	}
	if result["cost_class"] != queryengine.CostClassIndex {
		t.Fatalf("identity diff cost_class = %v, want index", result["cost_class"])
	}
	if _, ok := result["unchanged_count"]; !ok {
		t.Fatal("identity diff response missing unchanged_count")
	}
}

// ---- T11 observe 半边联合断言（CCB-PARAM 已合入：§2.6 响应字段面）----

func TestT11JointObserveAndQueryCostFields(t *testing.T) {
	// query 半边：降级路径（R5/R6）必带 degraded+compile。
	h := newRefQueryTestHarness(t)
	queryResult, err := h.refQuery(context.Background(), map[string]any{
		"cmd":   "ref_query",
		"kinds": []string{"dom"},
		"payload_conditions": []any{
			map[string]any{"field": "dom.peak_structure.readiness", "op": "eq", "str": "ready"},
		},
	})
	if err != nil {
		t.Fatalf("query half rejected: %v", err)
	}
	if queryResult["cost_class"] != queryengine.CostClassCompile || queryResult["degraded"] != queryengine.DegradedPayloadIndexUnavailable {
		t.Fatalf("query half must surface compile+degraded on the degraded path, got %+v/%+v", queryResult["cost_class"], queryResult["degraded"])
	}

	// observe 半边：物化 on 态响应携带 actual_cost_class+recomputed
	// （QUERY_ENGINE §2.6；提取面=ccbObservationOnFields，MAT-E 字段面）。
	observeFields := ccbObservationOnFields(map[string]any{
		"actual_cost_class": "compile",
		"recomputed":        2,
	})
	if observeFields["actual_cost_class"] != "compile" || observeFields["recomputed"] != 2 {
		t.Fatalf("observe half must surface actual_cost_class+recomputed, got %+v", observeFields)
	}
	merged := map[string]any{"actual_cost_class": "index", "recomputed": 1}
	ccbObservationMergeOnFields(merged, map[string]any{"actual_cost_class": "compile", "recomputed": 2})
	if merged["actual_cost_class"] != "compile" || merged["recomputed"] != 3 {
		t.Fatalf("observe half merge must be compile-dominant with summed recomputed, got %+v", merged)
	}

	// off 态响应面不变（字段缺席——两半的诚实边界一致）。
	if fields := ccbObservationOnFields(map[string]any{"observation_id": "obs_1"}); len(fields) != 0 {
		t.Fatalf("off-state observe response surface must be unchanged, got %+v", fields)
	}
}
