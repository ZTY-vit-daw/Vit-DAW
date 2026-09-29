package harness

// ccb_observation_param_test.go — §2.6 observe 参数化增量的 harness 端到端
// 契约测试（CCB-PARAM）。覆盖：time_window 接受与回显、freshness enum 拒绝、
// mix.* 缩窄护栏拒绝、top_k 越界拒绝、catalog dimension 过滤与 schema enum
// 一致性、targets 批量 fan-out、物化 on 态响应字段（T11 半边断言）。
// 端测边界：单测面（harness+shadow 夹具）；真栈烟测归 IMPL-C/D 收口。

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/tools"
)

func ccbParamHarness(t *testing.T) *Harness {
	t.Helper()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(t.TempDir(), "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	return New(nil, shadowProjectWithClips(), nil)
}

func ccbParamInvoke(t *testing.T, h *Harness, args map[string]any) map[string]any {
	t.Helper()
	response, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "ccb.observation_request",
		Args: args,
		// agentloop 语境：RequestedBy=model（回执口径对齐既有测试）。
		Source: "agentloop",
	})
	if err != nil {
		t.Fatal(err)
	}
	return testMap(t, response.Result)
}

func ccbParamContains(values []string, fragment string) bool {
	for _, value := range values {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}

// —— 缺口 1：time_window 接受、回显与诚实 limitation ——

func TestCCBObservationRequestTimeWindowEchoAndLimitation(t *testing.T) {
	h := ccbParamHarness(t)
	result := ccbParamInvoke(t, h, map[string]any{
		"request_id": "ccb-tw", "mix_session_id": "ccb-tw-session",
		"view_ids":    []any{"track.basic_energy"},
		"target_kind": "track", "target_id": "1007",
		"time_window": map[string]any{"start_seconds": 0.0, "end_seconds": 2.0, "units": "seconds"},
	})
	if result["status"] == "rejected" {
		t.Fatalf("windowed request rejected: %+v", result)
	}
	bundle := testMap(t, result["bundle"])
	window := testMap(t, bundle["time_window"])
	if window["start_seconds"] != 0.0 || window["end_seconds"] != 2.0 || window["units"] != "seconds" {
		t.Fatalf("bundle time_window echo = %+v", window)
	}
	echoed := false
	for _, limitation := range stringSliceFromAny(bundle["limitations"]) {
		if strings.Contains(limitation, "time_window is applied to the masking probe range only") {
			echoed = true
		}
	}
	if !echoed {
		t.Fatalf("explicit window must carry the whole-window honesty limitation: %+v", bundle["limitations"])
	}
	receipt := testMap(t, bundle["audit_receipt"])
	if receiptWindow := testMap(t, receipt["time_window"]); receiptWindow == nil || receiptWindow["units"] != "seconds" {
		t.Fatalf("receipt time_window echo = %+v", receipt["time_window"])
	}
	// "all" 与缺省：不回显窗、不加 limitation（响应面与既有行为一致）。
	result = ccbParamInvoke(t, h, map[string]any{
		"request_id": "ccb-tw-all", "mix_session_id": "ccb-tw-session-2",
		"view_ids":    []any{"track.basic_energy"},
		"target_kind": "track", "target_id": "1007",
		"time_window": "all",
	})
	if result["status"] == "rejected" {
		t.Fatalf("\"all\" window request rejected: %+v", result)
	}
	bundle = testMap(t, result["bundle"])
	if bundle["time_window"] != nil {
		t.Fatalf("\"all\" window must not echo an explicit window: %+v", bundle["time_window"])
	}
}

// —— 缺口 5：freshness enum 端到端拒绝 ——

func TestCCBObservationRequestRejectsUnknownFreshnessClass(t *testing.T) {
	h := ccbParamHarness(t)
	result := ccbParamInvoke(t, h, map[string]any{
		"request_id": "ccb-fresh", "mix_session_id": "ccb-fresh-session",
		"view_ids":    []any{"track.basic_energy"},
		"target_kind": "track", "target_id": "1007",
		"freshness_class": "bogus_freshness",
	})
	if result["status"] != "rejected" {
		t.Fatalf("unknown freshness_class must be rejected, got %+v", result["status"])
	}
	bundle := testMap(t, result["bundle"])
	if !ccbParamContains(stringSliceFromAny(bundle["omission_reasons"]), "freshness_class must be one of current_observation|post_action|material_reuse (got \"bogus_freshness\")") {
		t.Fatalf("rejection reason = %+v", bundle["omission_reasons"])
	}
	// 合法枚举值通过。
	result = ccbParamInvoke(t, h, map[string]any{
		"request_id": "ccb-fresh-ok", "mix_session_id": "ccb-fresh-session-2",
		"view_ids":    []any{"track.basic_energy"},
		"target_kind": "track", "target_id": "1007",
		"freshness_class": "post_action",
	})
	if result["status"] == "rejected" {
		t.Fatalf("post_action freshness_class should be accepted: %+v", result)
	}
	if class := testMap(t, testMap(t, result["bundle"])["freshness"])["class"]; class != "post_action" {
		t.Fatalf("bundle freshness.class = %+v", class)
	}
}

// —— 缺口 2 护栏反例：mix.* 保持 project 域硬定，targets 缩窄拒绝 ——

func TestCCBObservationRequestRejectsMixViewNarrowedByTargets(t *testing.T) {
	h := ccbParamHarness(t)
	result := ccbParamInvoke(t, h, map[string]any{
		"request_id": "ccb-narrow", "mix_session_id": "ccb-narrow-session",
		"view_ids": []any{"mix.multitrack_relationship"},
		"targets":  []any{map[string]any{"kind": "track", "id": "1007"}},
	})
	if result["status"] != "rejected" {
		t.Fatalf("mix view narrowed by targets must be rejected, got %+v", result["status"])
	}
	bundle := testMap(t, result["bundle"])
	blocking := stringSliceFromAny(bundle["blocking_view_ids"])
	if len(blocking) != 1 || blocking[0] != "mix.multitrack_relationship" {
		t.Fatalf("blocking views = %v, want the mix view", blocking)
	}
	found := false
	for _, reason := range stringSliceFromAny(bundle["omission_reasons"]) {
		if strings.Contains(reason, "targets batching applies to track.* views only") {
			found = true
		}
	}
	if !found {
		t.Fatalf("guardrail reason missing: %+v", bundle["omission_reasons"])
	}
	receipt := testMap(t, bundle["audit_receipt"])
	if len(stringSliceFromAny(receipt["targets"])) != 1 {
		t.Fatalf("receipt should echo the requested targets for audit: %+v", receipt["targets"])
	}
}

// —— 缺口 4 反例：top_k 越界字段与 k 越界拒绝（不静默乱序） ——

func TestCCBObservationRequestRejectsTopKViolations(t *testing.T) {
	h := ccbParamHarness(t)
	result := ccbParamInvoke(t, h, map[string]any{
		"request_id": "ccb-topk-field", "mix_session_id": "ccb-topk-session",
		"view_ids": []any{"mix.masking_relationship"},
		"top_k":    map[string]any{"field": "not_a_field", "dir": "desc", "k": 5},
	})
	if result["status"] != "rejected" {
		t.Fatalf("out-of-declaration top_k field must be rejected, got %+v", result["status"])
	}
	reasons := stringSliceFromAny(testMap(t, result["bundle"])["omission_reasons"])
	found := false
	for _, reason := range reasons {
		if strings.Contains(reason, "declared sortable fields") {
			found = true
		}
	}
	if !found {
		t.Fatalf("sortable-fields rejection reason missing: %v", reasons)
	}

	result = ccbParamInvoke(t, h, map[string]any{
		"request_id": "ccb-topk-k", "mix_session_id": "ccb-topk-session-2",
		"view_ids": []any{"mix.masking_relationship"},
		"top_k":    map[string]any{"field": "median_margin_db", "dir": "desc", "k": 25},
	})
	if result["status"] != "rejected" {
		t.Fatalf("top_k.k=25 must be rejected, got %+v", result["status"])
	}
	reasons = stringSliceFromAny(testMap(t, result["bundle"])["omission_reasons"])
	found = false
	for _, reason := range reasons {
		if strings.Contains(reason, "top_k.k must be between 1 and 24") {
			found = true
		}
	}
	if !found {
		t.Fatalf("k-bounds rejection reason missing: %v", reasons)
	}
}

// —— 缺口 6：catalog dimension 过滤 + schema enum 与词表一致性 ——

func TestCCBObservationCatalogDimensionFilter(t *testing.T) {
	h := ccbParamHarness(t)
	response, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "ccb.observation_catalog",
		Args: map[string]any{"dimension": "level_headroom"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := testMap(t, response.Result)
	if result["status"] != "ok" {
		t.Fatalf("dimension catalog status = %+v", result)
	}
	catalog := testMap(t, result["catalog"])
	if catalog["dimension"] != "level_headroom" {
		t.Fatalf("catalog.dimension = %+v", catalog["dimension"])
	}
	viewRows := mapRowsFromAny(catalog["views"])
	if len(viewRows) == 0 {
		t.Fatalf("filtered catalog has no views: %+v", catalog["views"])
	}
	for _, view := range viewRows {
		dimensions := stringSliceFromAny(view["diagnostic_dimensions"])
		if !ccbParamContains(dimensions, "level_headroom") {
			t.Fatalf("view %v does not declare level_headroom", view["view_id"])
		}
	}

	// 未知维度：拒绝并回词表。
	response, err = h.Invoke(context.Background(), InvokeRequest{
		Tool: "ccb.observation_catalog",
		Args: map[string]any{"dimension": "bogus_dimension"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result = testMap(t, response.Result)
	if result["status"] != "rejected" {
		t.Fatalf("unknown dimension must be rejected, got %+v", result["status"])
	}
	if len(stringSliceFromAny(result["dimensions"])) == 0 {
		t.Fatalf("rejection must carry the known vocabulary: %+v", result["dimensions"])
	}

	// schema enum 与运行时词表一致性（防 schema 漂移）。
	catalogSpec, ok := tools.DefaultCatalog().LookupTool("ccb.observation_catalog")
	if !ok {
		t.Fatal("ccb.observation_catalog missing from tool catalog")
	}
	properties, _ := catalogSpec.InputSchema["properties"].(map[string]any)
	dimensionSpec, _ := properties["dimension"].(map[string]any)
	enum := stringSliceFromAny(dimensionSpec["enum"])
	runtime := capabilitycontext.FreeStateObservationDimensions()
	if len(enum) != len(runtime) {
		t.Fatalf("schema enum %v != runtime vocabulary %v", enum, runtime)
	}
	for i := range enum {
		if enum[i] != runtime[i] {
			t.Fatalf("schema enum %v != runtime vocabulary %v", enum, runtime)
		}
	}
}

// —— 缺口 2：targets 批量 fan-out（track.* view 批量） ——

func TestCCBObservationBatchTargetsFanOut(t *testing.T) {
	h := ccbParamHarness(t)
	result := ccbParamInvoke(t, h, map[string]any{
		"request_id": "ccb-batch", "mix_session_id": "ccb-batch-session",
		"view_ids": []any{"track.basic_energy"},
		"targets": []any{
			map[string]any{"kind": "track", "id": "1007", "label": "Drums"},
			map[string]any{"kind": "track", "id": "1010", "label": "Bass"},
		},
	})
	if result["status"] == "rejected" {
		t.Fatalf("batch targets request rejected: %+v", result)
	}
	bundle := testMap(t, result["bundle"])
	views := testMap(t, bundle["views"])
	if views["track.basic_energy@1007"] == nil || views["track.basic_energy@1010"] == nil {
		t.Fatalf("batch views missing per-target keys: %+v", views)
	}
	if views["track.basic_energy"] != nil {
		t.Fatalf("batch must not carry unsuffixed view keys: %+v", views)
	}
	observations := testMap(t, bundle["batch_observations"])
	if len(observations) != 2 {
		t.Fatalf("batch observation lineage = %+v", observations)
	}
	receipt := testMap(t, bundle["audit_receipt"])
	executed := stringSliceFromAny(receipt["actual_executed_view_ids"])
	if len(executed) != 2 {
		t.Fatalf("receipt executed set = %v", executed)
	}
	if receipt["view_set_matches"] != true {
		t.Fatalf("receipt view set must match: %+v", receipt)
	}
	if len(stringSliceFromAny(receipt["targets"])) != 2 {
		t.Fatalf("receipt targets echo = %+v", receipt["targets"])
	}
}

// —— T11 半边：物化 on 态 observe 参数化路径的响应字段断言 ——

func TestCCBObservationOnStateServesCostFields(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", "")
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	if _, _, err := projectstore.Activate(root+"/song.vit", "vitproj_ccb_param_t11"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_DAW_MATERIALIZATION", "on")
	h := New(nil, shadowProjectWithClips(), nil)

	// miss 轮（空物化库）：compile + recomputed=1（QUERY_ENGINE §2.6 响应字段）。
	response, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "ccb.observation_request",
		Args: map[string]any{
			"request_id": "ccb-t11", "mix_session_id": "ccb-t11-session",
			"view_ids":    []any{"track.peak_structure"},
			"target_kind": "track", "target_id": "1007",
		},
		Source: "agentloop",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := testMap(t, response.Result)
	if result["status"] == "rejected" {
		t.Fatalf("on-state ccb request rejected: %+v", result)
	}
	if result["actual_cost_class"] != "compile" || result["recomputed"] != 1 {
		t.Fatalf("on-state miss round must surface compile/1, got cost=%+v recomputed=%+v", result["actual_cost_class"], result["recomputed"])
	}

	// 命中轮（回填行装配）：index + recomputed=0。
	response, err = h.Invoke(context.Background(), InvokeRequest{
		Tool: "ccb.observation_request",
		Args: map[string]any{
			"request_id": "ccb-t11-hit", "mix_session_id": "ccb-t11-session-2",
			"view_ids":    []any{"track.peak_structure"},
			"target_kind": "track", "target_id": "1007",
		},
		Source: "agentloop",
	})
	if err != nil {
		t.Fatal(err)
	}
	result = testMap(t, response.Result)
	if result["status"] == "rejected" {
		t.Fatalf("on-state hit round rejected: %+v", result)
	}
	if result["actual_cost_class"] != "index" || result["recomputed"] != 0 {
		t.Fatalf("on-state hit round must surface index/0, got cost=%+v recomputed=%+v", result["actual_cost_class"], result["recomputed"])
	}

	// off 态（默认）：响应面不变——字段不出现。
	t.Setenv("VIT_DAW_MATERIALIZATION", "off")
	hOff := ccbParamHarness(t)
	result = ccbParamInvoke(t, hOff, map[string]any{
		"request_id": "ccb-t11-off", "mix_session_id": "ccb-t11-off-session",
		"view_ids":    []any{"track.peak_structure"},
		"target_kind": "track", "target_id": "1007",
	})
	if result["actual_cost_class"] != nil || result["recomputed"] != nil {
		t.Fatalf("off-state response surface must be unchanged, got cost=%+v recomputed=%+v", result["actual_cost_class"], result["recomputed"])
	}
}
