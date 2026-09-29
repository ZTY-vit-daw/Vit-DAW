package capabilitycontext

// free_state_observation_param_test.go — §2.6 observe 参数化增量契约测试
// （CCB-PARAM；规格=docs/QUERY_ENGINE_V1_DESIGN.md §2.6 表左列）：
//  1. freshness_class 枚举（缺口 5）：三值合法、未知拒绝；
//  2. time_window 双标尺（缺口 1）：缺省全窗、seconds/samples 合法、units/start-end 非法拒绝；
//  3. targets 批量护栏（缺口 2）：≤8、kind 域、mix.*/project.* 缩窄拒绝、replay 拒绝；
//  4. top_k 声明集（缺口 4）：合法字段排序截断生效、越界/k/dir/无声明 view 拒绝；
//  5. dimension 目录过滤（缺口 6）：GetViewsForDimension 接线一致性、未知维度 ok=false；
//  6. targets 批量合并：后缀视图键、回执执行集、批量观察血缘。

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/mixboard"
)

func paramTestRequest(viewIDs ...string) FreeStateObservationRequest {
	return NormalizeFreeStateObservationRequest(FreeStateObservationRequest{ViewIDs: viewIDs})
}

func paramTestReasons(t *testing.T, req FreeStateObservationRequest) []string {
	t.Helper()
	return ValidateFreeStateObservationParameters(req)
}

func reasonsContain(reasons []string, fragment string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, fragment) {
			return true
		}
	}
	return false
}

// —— 缺口 5：freshness_class 枚举 ——

func TestFreeStateFreshnessClassEnumContract(t *testing.T) {
	for _, class := range FreeStateObservationFreshnessClasses {
		req := paramTestRequest("track.basic_energy")
		req.FreshnessClass = class
		if reasons := paramTestReasons(t, req); len(reasons) != 0 {
			t.Fatalf("freshness_class %q should be accepted, got %v", class, reasons)
		}
	}
	req := paramTestRequest("track.basic_energy")
	req.FreshnessClass = "bogus_freshness"
	reasons := paramTestReasons(t, req)
	if !reasonsContain(reasons, "freshness_class must be one of") {
		t.Fatalf("unknown freshness_class should be rejected with enum reason, got %v", reasons)
	}
	if !reasonsContain(reasons, "current_observation|post_action|material_reuse") {
		t.Fatalf("rejection reason should list the enum vocabulary, got %v", reasons)
	}
	// 缺省归一为 current_observation（既有行为不回退）。
	if got := paramTestRequest("track.basic_energy").FreshnessClass; got != FreeStateFreshnessClassCurrentObservation {
		t.Fatalf("default freshness_class = %q", got)
	}
}

// —— 缺口 1：time_window 双标尺 ——

func TestFreeStateTimeWindowContract(t *testing.T) {
	if req := paramTestRequest("track.basic_energy"); req.TimeWindow != nil {
		t.Fatalf("absent time_window must stay nil (full window default)")
	}
	valid := []*FreeStateObservationTimeWindow{
		{StartSeconds: 0, EndSeconds: 12.5, Units: "seconds"},
		{StartSeconds: 3, EndSeconds: 9, Units: "samples"},
		{StartSeconds: 1, EndSeconds: 2}, // units 缺省归一 seconds
	}
	for _, window := range valid {
		// 窗口先设再 normalize（对齐 harness 真实路径：units 缺省在 normalize 归一）。
		req := NormalizeFreeStateObservationRequest(FreeStateObservationRequest{
			ViewIDs:    []string{"track.basic_energy"},
			TimeWindow: window,
		})
		if reasons := ValidateFreeStateObservationParameters(req); len(reasons) != 0 {
			t.Fatalf("time_window %+v should be accepted, got %v", window, reasons)
		}
	}
	invalid := []struct {
		name   string
		window *FreeStateObservationTimeWindow
		frag   string
	}{
		{"units", &FreeStateObservationTimeWindow{StartSeconds: 0, EndSeconds: 1, Units: "beats"}, "time_window.units"},
		{"end_le_start", &FreeStateObservationTimeWindow{StartSeconds: 5, EndSeconds: 5, Units: "seconds"}, "end_seconds greater than start_seconds"},
		{"empty_shape", &FreeStateObservationTimeWindow{}, "end_seconds greater than start_seconds"},
	}
	for _, tt := range invalid {
		req := paramTestRequest("track.basic_energy")
		req.TimeWindow = tt.window
		if reasons := paramTestReasons(t, req); !reasonsContain(reasons, tt.frag) {
			t.Fatalf("%s window %+v should be rejected (%q), got %v", tt.name, tt.window, tt.frag, reasons)
		}
	}
}

// —— 缺口 2：targets 批量与 mix.*/project.* 护栏 ——

func TestFreeStateTargetsContract(t *testing.T) {
	targets := func(n int) []mixboard.TargetRef {
		out := make([]mixboard.TargetRef, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, mixboard.TargetRef{Kind: "track", ID: string(rune('A' + i))})
		}
		return out
	}
	req := paramTestRequest("track.basic_energy")
	req.Targets = targets(8)
	if reasons := paramTestReasons(t, req); len(reasons) != 0 {
		t.Fatalf("8 targets on track view should be accepted, got %v", reasons)
	}

	req = paramTestRequest("track.basic_energy")
	req.Targets = targets(9)
	if reasons := paramTestReasons(t, req); !reasonsContain(reasons, "at most 8") {
		t.Fatalf("9 targets should be rejected, got %v", reasons)
	}

	req = paramTestRequest("track.basic_energy")
	req.Targets = []mixboard.TargetRef{{Kind: "project", ID: "p1"}}
	if reasons := paramTestReasons(t, req); !reasonsContain(reasons, "kind must be one of track|clip|selection") {
		t.Fatalf("project-kind target should be rejected, got %v", reasons)
	}

	// 护栏反例：mix.* 保持 project 域硬定，targets 不得缩窄。
	req = paramTestRequest("mix.multitrack_relationship")
	req.Targets = targets(1)
	reasons := paramTestReasons(t, req)
	if !reasonsContain(reasons, "mix.multitrack_relationship: targets batching applies to track.* views only") {
		t.Fatalf("mix view narrowed by targets should be rejected per-view, got %v", reasons)
	}

	// project.* 域同样不可缩窄。
	req = paramTestRequest("project.structure")
	req.Targets = targets(1)
	if reasons := paramTestReasons(t, req); !reasonsContain(reasons, "project.structure: targets batching applies to track.* views only") {
		t.Fatalf("project view narrowed by targets should be rejected, got %v", reasons)
	}

	// replay 的目标继承自权威绑定，targets 不得重指。
	req = paramTestRequest("track.basic_energy")
	req.ObservationID = "obs_existing"
	req.Targets = targets(1)
	if reasons := paramTestReasons(t, req); !reasonsContain(reasons, "not supported on observation replay") {
		t.Fatalf("targets on replay should be rejected, got %v", reasons)
	}

	// normalize 去重去空（与 view_ids 同口径）。
	req = NormalizeFreeStateObservationRequest(FreeStateObservationRequest{
		ViewIDs: []string{"track.basic_energy"},
		Targets: []mixboard.TargetRef{{Kind: "track", ID: "1007"}, {Kind: "track", ID: "1007"}, {Kind: "track", ID: ""}},
	})
	if len(req.Targets) != 1 || req.Targets[0].ID != "1007" {
		t.Fatalf("targets normalize = %+v, want deduped [1007]", req.Targets)
	}
}

// —— 缺口 4：top_k 声明集与选择 ——

func TestFreeStateTopKContract(t *testing.T) {
	req := paramTestRequest("mix.masking_relationship")
	req.TopK = &FreeStateObservationTopK{Field: "median_margin_db", Dir: "desc", K: 5}
	if reasons := paramTestReasons(t, req); len(reasons) != 0 {
		t.Fatalf("declared top_k field should be accepted, got %v", reasons)
	}

	req = paramTestRequest("mix.masking_relationship")
	req.TopK = &FreeStateObservationTopK{Field: "not_a_field", Dir: "desc", K: 5}
	if reasons := paramTestReasons(t, req); !reasonsContain(reasons, "mix.masking_relationship: top_k.field must be one of the view's declared sortable fields") {
		t.Fatalf("out-of-declaration top_k field should be rejected, got %v", reasons)
	}

	for _, k := range []int{0, 25} {
		req = paramTestRequest("mix.masking_relationship")
		req.TopK = &FreeStateObservationTopK{Field: "median_margin_db", K: k}
		if reasons := paramTestReasons(t, req); !reasonsContain(reasons, "top_k.k must be between 1 and 24") {
			t.Fatalf("top_k.k=%d should be rejected, got %v", k, reasons)
		}
	}

	req = paramTestRequest("mix.masking_relationship")
	req.TopK = &FreeStateObservationTopK{Field: "median_margin_db", Dir: "up", K: 5}
	if reasons := paramTestReasons(t, req); !reasonsContain(reasons, "top_k.dir must be asc or desc") {
		t.Fatalf("invalid top_k.dir should be rejected, got %v", reasons)
	}

	// 无任何请求 view 声明可排序字段：拒绝而非静默忽略。
	req = paramTestRequest("track.basic_energy")
	req.TopK = &FreeStateObservationTopK{Field: "median_margin_db", K: 5}
	if reasons := paramTestReasons(t, req); !reasonsContain(reasons, "requires at least one requested view with declared sortable fields") {
		t.Fatalf("top_k without sortable view should be rejected, got %v", reasons)
	}

	// 混合两个声明 view 但字段只在其中一个：拒绝（拆请求，不静默乱序）。
	req = paramTestRequest("mix.masking_relationship", "mix.multitrack_relationship")
	req.TopK = &FreeStateObservationTopK{Field: "median_margin_db", K: 5}
	if reasons := paramTestReasons(t, req); !reasonsContain(reasons, "mix.multitrack_relationship: top_k.field must be one of the view's declared sortable fields") {
		t.Fatalf("top_k field missing from one declaring view should be rejected, got %v", reasons)
	}

	// normalize：dir 缺省 desc。
	req = NormalizeFreeStateObservationRequest(FreeStateObservationRequest{
		ViewIDs: []string{"mix.masking_relationship"},
		TopK:    &FreeStateObservationTopK{Field: "median_margin_db", K: 3},
	})
	if req.TopK.Dir != "desc" {
		t.Fatalf("top_k.dir default = %q, want desc", req.TopK.Dir)
	}
}

func maskingCandidatesFixture(count int) []any {
	candidates := make([]any, 0, count)
	for i := 0; i < count; i++ {
		candidates = append(candidates, map[string]any{
			"masker_track_id": "T1", "target_track_id": "T2", "band_id": "mid",
			"min_hz": 250.0, "max_hz": 2000.0,
			"risk_frame_count": float64(i), "risk_coverage_ratio": float64(i) / float64(count),
			"median_margin_db": float64(i + 1), // 升序：desc 首名应为最大
		})
	}
	return candidates
}

func TestFreeStateTopKSelectionOnMaskingView(t *testing.T) {
	readResult := map[string]any{"items": map[string]any{
		"observation.binding": map[string]any{
			"observation_id": "obs_topk", "mix_session_id": "sess_topk", "status": "ready",
			"project_binding": map[string]any{"project_uuid": "u1", "project_epoch": "e1", "project_revision": "r1"},
		},
		"observation.mom_projection": map[string]any{
			"mom_version": "mom.projection.v1", "intent": "project_masking_relationship_observation",
			"masking_relationship": map[string]any{
				"status": "ready", "candidate_only": true,
				"coverage":      map[string]any{"track_count": 2, "candidate_count": float64(10)},
				"candidates":    maskingCandidatesFixture(10),
				"limitations":   []any{},
				"evidence_refs": []any{},
			},
		},
	}}
	req := NormalizeFreeStateObservationRequest(FreeStateObservationRequest{
		ViewIDs: []string{"mix.masking_relationship"},
		TopK:    &FreeStateObservationTopK{Field: "median_margin_db", Dir: "desc", K: 3},
	})
	bundle := AssembleFreeStateObservation(req, readResult)
	view, ok := bundle.Views["mix.masking_relationship"].(map[string]any)
	if !ok {
		t.Fatalf("masking view missing from bundle: %+v", bundle.Views)
	}
	facts, _ := view["facts"].(map[string]any)
	mom, _ := facts["observation.mom_projection"].(map[string]any)
	relation, _ := mom["masking_relationship"].(map[string]any)
	rows := rowsValue(relation["candidates"])
	if len(rows) != 3 {
		t.Fatalf("top_k.k=3 should truncate candidates to 3, got %d", len(rows))
	}
	if first := rows[0]["median_margin_db"].(float64); first != 10 {
		t.Fatalf("desc order first candidate margin = %v, want 10", first)
	}
	if second := rows[1]["median_margin_db"].(float64); second != 9 {
		t.Fatalf("desc order second candidate margin = %v, want 9", second)
	}
	marker, _ := relation["top_k"].(map[string]any)
	if marker == nil || marker["field"] != "median_margin_db" || marker["dir"] != "desc" || marker["applied_to"] != "candidates" {
		t.Fatalf("top_k audit marker = %+v", marker)
	}
	// 回执与 bundle 回显 top_k。
	if bundle.TopK == nil || bundle.TopK.Field != "median_margin_db" {
		t.Fatalf("bundle top_k echo = %+v", bundle.TopK)
	}
	if bundle.AuditReceipt.TopK == nil || bundle.AuditReceipt.TopK.K != 3 {
		t.Fatalf("receipt top_k echo = %+v", bundle.AuditReceipt.TopK)
	}
}

// —— 缺口 6：dimension 目录过滤接线 GetViewsForDimension ——

func TestFreeStateDimensionCatalogContract(t *testing.T) {
	expectedDimensions := []string{"dynamics", "frequency_occupancy", "level_headroom", "stereo_space", "transient_event"}
	dimensions := FreeStateObservationDimensions()
	if len(dimensions) != len(expectedDimensions) {
		t.Fatalf("dimension vocabulary = %v, want %v", dimensions, expectedDimensions)
	}
	for i := range dimensions {
		if dimensions[i] != expectedDimensions[i] {
			t.Fatalf("dimension vocabulary = %v, want %v", dimensions, expectedDimensions)
		}
	}

	target := mixboard.TargetRef{Kind: "project", ID: ""}
	catalog, ok := FreeStateObservationCatalogForDimension(target, "level_headroom")
	if !ok {
		t.Fatal("known dimension should filter successfully")
	}
	if catalog.Dimension != "level_headroom" || len(catalog.Views) == 0 {
		t.Fatalf("filtered catalog = %+v", catalog)
	}
	for _, view := range catalog.Views {
		if !containsString(view.DiagnosticDimensions, "level_headroom") {
			t.Fatalf("view %q returned by dimension filter does not declare it", view.ViewID)
		}
	}
	// 接线一致性：过滤结果与 GetViewsForDimension（F7 钩子）同集。
	filtered := map[string]bool{}
	for _, view := range catalog.Views {
		filtered[view.ViewID] = true
	}
	hook := map[string]bool{}
	for _, viewID := range GetViewsForDimension("level_headroom", "") {
		hook[viewID] = true
	}
	if len(filtered) != len(hook) {
		t.Fatalf("catalog filter %v disagrees with GetViewsForDimension %v", filtered, hook)
	}
	for viewID := range filtered {
		if !hook[viewID] {
			t.Fatalf("catalog filter %v disagrees with GetViewsForDimension %v", filtered, hook)
		}
	}

	if _, ok := FreeStateObservationCatalogForDimension(target, "bogus_dimension"); ok {
		t.Fatal("unknown dimension must report ok=false (caller rejects with vocabulary)")
	}
	// 全量目录携带词表（模型可见的 dimension 发现面）。
	full := FreeStateObservationCatalogFor(target)
	if len(full.Dimensions) != len(expectedDimensions) {
		t.Fatalf("unfiltered catalog dimensions = %v", full.Dimensions)
	}
}

// —— 缺口 2：targets 批量合并 ——

func batchBundleFixture(t *testing.T, targetID string) FreeStateObservationBundle {
	t.Helper()
	track := "track." + targetID
	readResult := map[string]any{"items": map[string]any{
		"observation.binding": map[string]any{
			"observation_id": "obs_" + targetID, "mix_session_id": "sess_batch", "status": "ready",
			"target_ref":      map[string]any{"kind": "track", "id": targetID},
			"project_binding": map[string]any{"project_uuid": "u1", "project_epoch": "e1", "project_revision": "r1"},
		},
		track + ".static.identity": map[string]any{"status": "ready", "track_id": targetID},
		track + ".fast.levels":     map[string]any{"status": "ready", "rms_dbfs": -18.0},
	}}
	return AssembleFreeStateObservation(NormalizeFreeStateObservationRequest(FreeStateObservationRequest{
		ViewIDs:   []string{"track.basic_energy"},
		TargetRef: mixboard.TargetRef{Kind: "track", ID: targetID},
	}), readResult)
}

func TestFreeStateBatchMergeContract(t *testing.T) {
	req := NormalizeFreeStateObservationRequest(FreeStateObservationRequest{
		ViewIDs: []string{"track.basic_energy"},
		Targets: []mixboard.TargetRef{{Kind: "track", ID: "1007"}, {Kind: "track", ID: "1010"}},
	})
	merged := MergeFreeStateObservationBundles(req, []FreeStateObservationBatchEntry{
		{Target: mixboard.TargetRef{Kind: "track", ID: "1007"}, Bundle: batchBundleFixture(t, "1007")},
		{Target: mixboard.TargetRef{Kind: "track", ID: "1010"}, Bundle: batchBundleFixture(t, "1010")},
	})
	if _, ok := merged.Views["track.basic_energy@1007"]; !ok {
		t.Fatalf("batch merge missing suffixed view key: %v", merged.Views)
	}
	if _, ok := merged.Views["track.basic_energy@1010"]; !ok {
		t.Fatalf("batch merge missing suffixed view key: %v", merged.Views)
	}
	if _, ok := merged.Views["track.basic_energy"]; ok {
		t.Fatalf("batch merge must not carry unsuffixed keys: %v", merged.Views)
	}
	if merged.Status == "rejected" || merged.Status == "insufficient" {
		t.Fatalf("batch merge status = %q", merged.Status)
	}
	if len(merged.RequestedViews) != 1 || merged.RequestedViews[0] != "track.basic_energy" {
		t.Fatalf("requested views = %v (model asked plain view IDs)", merged.RequestedViews)
	}
	if len(merged.Targets) != 2 || merged.Targets[0].ID != "1007" {
		t.Fatalf("bundle targets echo = %+v", merged.Targets)
	}
	if len(merged.BatchObservations) != 2 || merged.BatchObservations["1007"] != "obs_1007" || merged.BatchObservations["1010"] != "obs_1010" {
		t.Fatalf("batch observation lineage = %+v", merged.BatchObservations)
	}
	executed := map[string]bool{}
	for _, viewID := range merged.AuditReceipt.ActualExecutedViewIDs {
		executed[viewID] = true
	}
	if !executed["track.basic_energy@1007"] || !executed["track.basic_energy@1010"] {
		t.Fatalf("receipt executed set = %v", merged.AuditReceipt.ActualExecutedViewIDs)
	}
	if !merged.AuditReceipt.ViewSetMatches {
		t.Fatalf("receipt view set should match: model=%v executed=%v", merged.AuditReceipt.ModelRequestedViewIDs, merged.AuditReceipt.ActualExecutedViewIDs)
	}
	if len(merged.AuditReceipt.Targets) != 2 {
		t.Fatalf("receipt targets echo = %+v", merged.AuditReceipt.Targets)
	}

	// 空批拒绝（不应发生，防御面）。
	if rejected := MergeFreeStateObservationBundles(req, nil); rejected.Status != "rejected" {
		t.Fatalf("empty batch merge status = %q, want rejected", rejected.Status)
	}
}
