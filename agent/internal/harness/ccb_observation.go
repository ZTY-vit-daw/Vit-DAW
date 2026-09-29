package harness

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/mixboard"
)

func (h *Harness) ccbObservationCatalog(cmd map[string]any) map[string]any {
	target := ccbObservationTarget(cmd)
	dimension := strings.TrimSpace(firstString(cmd, "dimension"))
	if dimension == "" {
		return map[string]any{"status": "ok", "catalog": capabilitycontext.FreeStateObservationCatalogFor(target)}
	}
	// §2.6 缺口 6：目录 dimension 过滤（接线 GetViewsForDimension，F7）。
	// 未知维度拒绝并回词表——dimension 是目录过滤不是推荐，CCB 不做自动 view 选择。
	catalog, ok := capabilitycontext.FreeStateObservationCatalogForDimension(target, dimension)
	if !ok {
		return map[string]any{
			"status":     "rejected",
			"reasons":    []string{"dimension " + dimension + " is not a known diagnostic dimension"},
			"dimensions": capabilitycontext.FreeStateObservationDimensions(),
		}
	}
	return map[string]any{"status": "ok", "catalog": catalog}
}

func (h *Harness) ccbObservationRequest(ctx context.Context, cmd map[string]any, requestContext map[string]any, source string) (map[string]any, error) {
	rawViewIDs := ccbObservationViewIDs(cmd)
	req := capabilitycontext.FreeStateObservationRequest{
		RequestID:          firstString(cmd, "request_id"),
		ObservationID:      firstString(cmd, "observation_id"),
		MixSessionID:       firstString(cmd, "mix_session_id", "session_id"),
		ViewIDs:            rawViewIDs,
		OriginalViewIDs:    append([]string(nil), rawViewIDs...),
		TargetRef:          ccbObservationTarget(cmd),
		Targets:            ccbObservationTargets(cmd),
		TimeWindow:         ccbObservationTimeWindow(cmd),
		TopK:               ccbObservationTopK(cmd),
		FreshnessClass:     firstString(cmd, "freshness_class", "freshness"),
		MaxDisclosureBytes: int(numberFromAny(cmd["max_disclosure_bytes"])),
		MaxItems:           int(numberFromAny(cmd["max_items"])),
		RequestedBy:        ccbObservationRequestedBy(requestContext, source),
	}
	req = capabilitycontext.NormalizeFreeStateObservationRequest(req)
	req.Scope = ccbObservationScope(req)
	if reasons := capabilitycontext.ValidateFreeStateObservationViewIDs(rawViewIDs); len(reasons) > 0 {
		return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, ccbObservationBlockingViewsFromReasons(req, reasons), reasons...)}, nil
	}
	if reasons := capabilitycontext.ValidateFreeStateObservationParameters(req); len(reasons) > 0 {
		return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, ccbObservationBlockingViewsFromReasons(req, reasons), reasons...)}, nil
	}
	if reasons := ccbObservationUnknownViewReasons(req); len(reasons) > 0 {
		return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, ccbObservationBlockingViewsFromReasons(req, reasons), reasons...)}, nil
	}
	// A track.* view observes exactly one track. A fresh request without an
	// explicit target materializes project-wide and yields the track-less
	// evidence shell the DIAG1 failure chain produced, so it is rejected with
	// guidance (reject + guide, never auto-select) and the model re-picks a
	// visible track next turn. A replay carrying an existing observation_id
	// inherits its target from the authoritative binding and stays legal.
	if blocking, reasons := ccbObservationTrackTargetReasons(req); len(reasons) > 0 {
		return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, blocking, reasons...)}, nil
	}
	// Catalog entries marked deferred/unavailable are explicit capability
	// boundaries. Reject before materializing an observation so the model gets
	// a durable receipt and can deterministically choose another view or block.
	if blocking, reasons := ccbObservationUnavailableViewDetails(req); len(reasons) > 0 {
		return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, blocking, reasons...)}, nil
	}
	// §2.6 targets 批量：多目标 fan-out 到逐目标观察并合并（参数验证已保证
	// 全部 view 为 track.*、无 replay）。
	if len(req.Targets) > 1 {
		return h.ccbObservationBatchRequest(ctx, cmd, req)
	}
	if len(req.Targets) == 1 && strings.TrimSpace(req.TargetRef.ID) == "" {
		req.TargetRef = req.Targets[0]
	}
	onFields := map[string]any{}
	if req.ObservationID == "" {
		observeCmd := ccbObservationCommand(cmd, req)
		if containsCCBView(req.ViewIDs, "mix.masking_relationship") {
			if _, err := h.prepareCCBMaskingObservation(ctx, observeCmd, req.MixSessionID); err != nil {
				reason := "mix.masking_relationship: " + err.Error()
				return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, []string{"mix.masking_relationship"}, reason)}, nil
			}
		}
		observed, err := h.requestMixObservation(ctx, observeCmd)
		if err != nil {
			return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, req.ViewIDs, err.Error())}, nil
		}
		req.ObservationID = firstString(observed, "observation_id")
		req.MixSessionID = firstString(observed, "mix_session_id")
		if req.ObservationID == "" {
			reason := firstNonEmpty(firstString(observed, "reason", "error", "message"), "authoritative observation was not materialized")
			return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, req.ViewIDs, reason)}, nil
		}
		// T11 半边（QUERY_ENGINE §2.6 响应字段）：物化 on 态的 observe 响应携带
		// actual_cost_class/recomputed（MAT-E 已落的字段面）——"observe 便宜"的
		// 预设被打破时模型必须看得见。off/shadow 态字段不出现（响应面不变）。
		onFields = ccbObservationOnFields(observed)
	}
	bundle, err := h.ccbObservationReadAndAssemble(req)
	if err != nil {
		return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, req.ViewIDs, err.Error())}, nil
	}
	response := map[string]any{
		"status": bundle.Status,
		"bundle": bundle,
	}
	for key, value := range onFields {
		response[key] = value
	}
	return response, nil
}

// ccbObservationOnFields 提取物化 on 态 observe 响应字段（actual_cost_class/
// recomputed；仅 MAT-E on 态可见）。off/shadow 或无字段时返回空 map。
func ccbObservationOnFields(observed map[string]any) map[string]any {
	fields := map[string]any{}
	if class := firstString(observed, "actual_cost_class"); class != "" {
		fields["actual_cost_class"] = class
	}
	if _, ok := observed["recomputed"]; ok {
		fields["recomputed"] = int(numberFromAny(observed["recomputed"]))
	}
	return fields
}

// ccbObservationReadAndAssemble 读取观察绑定与 view 读键并装配 bundle。
func (h *Harness) ccbObservationReadAndAssemble(req capabilitycontext.FreeStateObservationRequest) (capabilitycontext.FreeStateObservationBundle, error) {
	store := mixboard.NewStore("")
	bindingRead, err := store.Read(mixboard.ReadRequest{
		ObservationID: req.ObservationID,
		MixSessionID:  req.MixSessionID,
		Keys:          []string{"observation.binding"},
		MaxItems:      1,
	})
	if err != nil {
		return capabilitycontext.FreeStateObservationBundle{}, err
	}
	binding := mapAnyFromAny(mapAnyFromAny(bindingRead["items"])["observation.binding"])
	targetID := ccbObservationBindingTargetID(binding["target_ref"])
	readResult, err := store.Read(mixboard.ReadRequest{
		ObservationID: req.ObservationID,
		MixSessionID:  req.MixSessionID,
		Keys:          capabilitycontext.FreeStateObservationReadKeys(req, targetID),
		MaxItems:      req.MaxItems,
	})
	if err != nil {
		return capabilitycontext.FreeStateObservationBundle{}, err
	}
	return capabilitycontext.AssembleFreeStateObservation(req, readResult), nil
}

// ccbObservationBatchRequest 执行 §2.6 targets 批量 fan-out：逐目标物化观察
// 并装配，再合并为一个 bundle（视图键带 @<targetID> 后缀）。任一目标物化失败
// 即整请求拒绝（exact-set 纪律，不静默丢目标）。on 态字段跨目标聚合：cost
// class 取最坏（compile 支配），recomputed 求和。
func (h *Harness) ccbObservationBatchRequest(ctx context.Context, cmd map[string]any, req capabilitycontext.FreeStateObservationRequest) (map[string]any, error) {
	entries := make([]capabilitycontext.FreeStateObservationBatchEntry, 0, len(req.Targets))
	onFields := map[string]any{}
	for _, target := range req.Targets {
		sub := req
		sub.Targets = nil
		sub.TargetRef = target
		observed, err := h.requestMixObservation(ctx, ccbObservationCommand(cmd, sub))
		if err != nil {
			reason := "targets[" + target.ID + "]: " + err.Error()
			return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, req.ViewIDs, reason)}, nil
		}
		sub.ObservationID = firstString(observed, "observation_id")
		sub.MixSessionID = firstString(observed, "mix_session_id")
		if sub.ObservationID == "" {
			reason := "targets[" + target.ID + "]: " + firstNonEmpty(firstString(observed, "reason", "error", "message"), "authoritative observation was not materialized")
			return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, req.ViewIDs, reason)}, nil
		}
		ccbObservationMergeOnFields(onFields, observed)
		bundle, err := h.ccbObservationReadAndAssemble(sub)
		if err != nil {
			reason := "targets[" + target.ID + "]: " + err.Error()
			return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, req.ViewIDs, reason)}, nil
		}
		entries = append(entries, capabilitycontext.FreeStateObservationBatchEntry{Target: target, Bundle: bundle})
	}
	merged := capabilitycontext.MergeFreeStateObservationBundles(req, entries)
	response := map[string]any{
		"status": merged.Status,
		"bundle": merged,
	}
	for key, value := range onFields {
		response[key] = value
	}
	return response, nil
}

// ccbObservationMergeOnFields 聚合批量路径的 on 态响应字段（compile 支配
// index；recomputed 求和）。
func ccbObservationMergeOnFields(dst map[string]any, observed map[string]any) {
	if class := firstString(observed, "actual_cost_class"); class != "" {
		if previous, ok := dst["actual_cost_class"].(string); !ok || (previous != "compile" && class == "compile") {
			dst["actual_cost_class"] = class
		}
	}
	if _, ok := observed["recomputed"]; ok {
		dst["recomputed"] = int(numberFromAny(dst["recomputed"])) + int(numberFromAny(observed["recomputed"]))
	}
}

// ccbObservationBlockingViewsFromReasons extracts only view-specific causes
// from a rejection. A mixed request can therefore retain individually usable
// views as non-blocking instead of turning exact-set do_not_retry into a ban
// on every member of the set.
func ccbObservationBlockingViewsFromReasons(req capabilitycontext.FreeStateObservationRequest, reasons []string) []string {
	requested := map[string]bool{}
	for _, viewID := range req.OriginalViewIDs {
		viewID = strings.TrimSpace(viewID)
		if viewID != "" {
			requested[viewID] = true
		}
	}
	if len(requested) == 0 {
		for _, viewID := range req.ViewIDs {
			viewID = strings.TrimSpace(viewID)
			if viewID != "" {
				requested[viewID] = true
			}
		}
	}
	blocking := []string{}
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		for viewID := range requested {
			if strings.HasPrefix(reason, viewID+":") {
				blocking = append(blocking, viewID)
			}
		}
	}
	return uniqueStrings(blocking)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func ccbObservationRequestedBy(requestContext map[string]any, source string) string {
	if strings.EqualFold(strings.TrimSpace(source), "agentloop") || strings.EqualFold(strings.TrimSpace(source), "agent") {
		return "model"
	}
	if requestContext != nil {
		if _, ok := requestContext["free_state_reasoning_loop"]; ok {
			return "model"
		}
		if value, ok := boolValue(requestContext["free_state_route_authorized"]); ok && value {
			return "model"
		}
	}
	return "caller"
}

func ccbObservationUnknownViewReasons(req capabilitycontext.FreeStateObservationRequest) []string {
	catalog := capabilitycontext.FreeStateObservationCatalogFor(req.TargetRef)
	known := map[string]bool{}
	for _, view := range catalog.Views {
		known[view.ViewID] = true
	}
	reasons := []string{}
	for _, viewID := range req.ViewIDs {
		if !known[viewID] {
			reasons = append(reasons, viewID+": semantic view is not in the CCB catalog")
		}
	}
	return reasons
}

// ccbObservationTrackTargetReasons enforces the DIAG2 observation guardrail:
// every track.* view in a fresh materialization requires an explicit,
// non-empty target. Reasons are per-view prefixed so the blocking/non-blocking
// split keeps the non-track views of the same request retryable.
func ccbObservationTrackTargetReasons(req capabilitycontext.FreeStateObservationRequest) ([]string, []string) {
	if req.ObservationID != "" || strings.TrimSpace(req.TargetRef.ID) != "" || len(req.Targets) > 0 {
		return nil, nil
	}
	blocking := []string{}
	reasons := []string{}
	for _, viewID := range req.ViewIDs {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(viewID)), "track.") {
			blocking = append(blocking, viewID)
			reasons = append(reasons, viewID+": track view requires explicit target; pick from visible tracks and resend with target_ref kind=track id=<exact visible track id> (request project.structure first if track identities are not yet visible)")
		}
	}
	return uniqueStrings(blocking), reasons
}

func ccbObservationUnavailableViewReasons(req capabilitycontext.FreeStateObservationRequest) []string {
	_, reasons := ccbObservationUnavailableViewDetails(req)
	return reasons
}

func ccbObservationUnavailableViewDetails(req capabilitycontext.FreeStateObservationRequest) ([]string, []string) {
	catalog := capabilitycontext.FreeStateObservationCatalogFor(req.TargetRef)
	byID := map[string]capabilitycontext.FreeStateObservationView{}
	for _, view := range catalog.Views {
		byID[view.ViewID] = view
	}
	blocking := []string{}
	reasons := []string{}
	for _, viewID := range req.ViewIDs {
		view, ok := byID[viewID]
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(view.Availability)) {
		case "deferred", "unavailable":
			blocking = append(blocking, viewID)
			reasons = append(reasons, viewID+": availability="+view.Availability)
		}
	}
	return blocking, reasons
}

func ccbObservationBindingTargetID(value any) string {
	if target, ok := value.(mixboard.TargetRef); ok {
		return strings.TrimSpace(target.ID)
	}
	return firstString(mapAnyFromAny(value), "id", "track_id", "clip_id")
}

func ccbObservationTarget(cmd map[string]any) mixboard.TargetRef {
	row := mapAnyFromAny(cmd["target_ref"])
	return mixboard.TargetRef{
		Kind:       firstNonEmpty(firstString(cmd, "target_kind"), firstString(row, "kind", "target_kind")),
		ID:         firstNonEmpty(firstString(cmd, "target_id", "track_id", "clip_id"), firstString(row, "id", "target_id", "track_id", "clip_id")),
		Label:      firstNonEmpty(firstString(cmd, "target_label", "track_name", "clip_name"), firstString(row, "label", "name")),
		Source:     firstNonEmpty(firstString(row, "source"), "ccb_request"),
		Confidence: firstString(row, "confidence"),
	}
}

func ccbObservationViewIDs(cmd map[string]any) []string {
	values := stringSliceFromAny(cmd["view_ids"])
	if len(values) == 0 {
		values = stringSliceFromAny(cmd["views"])
	}
	if len(values) == 0 {
		if value := firstString(cmd, "view_id", "view"); value != "" {
			values = []string{value}
		}
	}
	return values
}

// ccbObservationTimeWindow 解析 §2.3 双标尺 time_window："all"/缺省 → nil（全
// 窗）；对象 → 显式区间。形状非法（非 "all" 字符串、空对象）返回零值区间，
// 由 ValidateFreeStateObservationParameters 拒绝（end<=start）。
func ccbObservationTimeWindow(cmd map[string]any) *capabilitycontext.FreeStateObservationTimeWindow {
	value := cmd["time_window"]
	if value == nil {
		return nil
	}
	if text, ok := value.(string); ok {
		if strings.EqualFold(strings.TrimSpace(text), "all") {
			return nil
		}
		return &capabilitycontext.FreeStateObservationTimeWindow{}
	}
	row := mapAnyFromAny(value)
	if len(row) == 0 {
		return &capabilitycontext.FreeStateObservationTimeWindow{}
	}
	return &capabilitycontext.FreeStateObservationTimeWindow{
		StartSeconds: numberFromAny(row["start_seconds"]),
		EndSeconds:   numberFromAny(row["end_seconds"]),
		Units:        firstString(row, "units"),
	}
}

// ccbObservationTargets 解析 §2.6 targets 批量列表。normalize 会去重去空 id
// （与 view_ids 同口径）；≤8 上限按去重后数量校验。
func ccbObservationTargets(cmd map[string]any) []mixboard.TargetRef {
	rows := mapRowsFromAny(cmd["targets"])
	out := make([]mixboard.TargetRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, mixboard.TargetRef{
			Kind:   firstString(row, "kind", "target_kind"),
			ID:     firstString(row, "id", "target_id", "track_id", "clip_id"),
			Label:  firstString(row, "label", "name"),
			Source: "ccb_request_targets",
		})
	}
	return out
}

// ccbObservationTopK 解析 §2.6 top_k 选择；缺省/非对象 → nil。
func ccbObservationTopK(cmd map[string]any) *capabilitycontext.FreeStateObservationTopK {
	row := mapAnyFromAny(cmd["top_k"])
	if len(row) == 0 {
		return nil
	}
	return &capabilitycontext.FreeStateObservationTopK{
		Field: firstString(row, "field"),
		Dir:   firstString(row, "dir"),
		K:     int(numberFromAny(row["k"])),
	}
}

func ccbObservationCommand(cmd map[string]any, req capabilitycontext.FreeStateObservationRequest) map[string]any {
	out := cloneAnyMap(cmd)
	// time_window 保留透传（§2.6 缺口 1——masking 预备探测区间消费它）；targets/
	// top_k 是 CCB 层参数化概念，不进物化命令。
	for _, key := range []string{"view_ids", "views", "view_id", "view", "max_disclosure_bytes", "max_items", "freshness_class", "targets", "top_k"} {
		delete(out, key)
	}
	out["cmd"] = "mix_request_observation"
	out["observation_only"] = true
	out["disclosure"] = "digest_catalog"
	out["goal_text"] = firstNonEmpty(firstString(cmd, "goal_text", "goal", "intent"), "ordinary-Agent free-state CCB observation")
	if req.MixSessionID != "" {
		out["mix_session_id"] = req.MixSessionID
	}
	// The semantic view defines the authoritative observation scope. A model may
	// suggest a scope, but it cannot narrow a project relationship view to one track.
	out["scope"] = ccbObservationScope(req)
	if req.TargetRef.Kind != "" {
		out["target_kind"] = req.TargetRef.Kind
	}
	if req.TargetRef.ID != "" {
		out["target_id"] = req.TargetRef.ID
		if strings.EqualFold(req.TargetRef.Kind, "track") || strings.EqualFold(req.TargetRef.Kind, "processor") {
			out["track_id"] = req.TargetRef.ID
		}
	}
	if req.TargetRef.Label != "" {
		out["target_label"] = req.TargetRef.Label
	}
	if containsCCBView(req.ViewIDs, "mix.masking_relationship") {
		out["mom_intent"] = "project_masking_relationship_observation"
	} else if containsCCBView(req.ViewIDs, "mix.frequency_relationship") {
		out["mom_intent"] = "project_frequency_relationship_observation"
	} else if containsCCBView(req.ViewIDs, "mix.multitrack_relationship") {
		out["mom_intent"] = "project_multitrack_relation_observation"
	}
	if containsCCBView(req.ViewIDs, "track.time_dynamics") || containsCCBView(req.ViewIDs, "processor.behavior") || containsCCBView(req.ViewIDs, "processor.identity_and_controls") {
		if firstString(out, "com_mode") == "" {
			out["com_mode"] = "source_only"
		}
	}
	for _, viewID := range []string{"track.peak_structure", "track.activity_structure", "track.frequency_time_events", "track.transient_structure", "track.band_dynamics"} {
		if containsCCBView(req.ViewIDs, viewID) {
			if firstString(out, "dom_mode") == "" {
				out["dom_mode"] = "source_only"
			}
			break
		}
	}
	return out
}

func ccbObservationScope(req capabilitycontext.FreeStateObservationRequest) string {
	hasTrackView := false
	hasMixView := false
	for _, viewID := range req.ViewIDs {
		viewID = strings.ToLower(strings.TrimSpace(viewID))
		hasTrackView = hasTrackView || strings.HasPrefix(viewID, "track.") || strings.HasPrefix(viewID, "processor.") || viewID == "comparison.before_after"
		hasMixView = hasMixView || strings.HasPrefix(viewID, "mix.") || strings.HasPrefix(viewID, "project.")
	}
	if hasMixView && hasTrackView {
		return "full_project_with_focus_track"
	}
	if hasMixView {
		return "full_project"
	}
	switch strings.ToLower(strings.TrimSpace(req.TargetRef.Kind)) {
	case "clip":
		return "selected_clip"
	case "track", "processor":
		return "selected_track"
	case "track_group":
		return "track_group"
	case "project":
		return "full_project"
	}
	return "selected_track"
}

func containsCCBView(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), expected) {
			return true
		}
	}
	return false
}

func validateCCBObservationResult(result map[string]any) error {
	if result == nil {
		return fmt.Errorf("ccb observation result is nil")
	}
	return nil
}
