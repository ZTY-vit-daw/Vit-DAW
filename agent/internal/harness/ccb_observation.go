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
	return map[string]any{
		"status":  "ok",
		"catalog": capabilitycontext.FreeStateObservationCatalogFor(target),
	}
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
	if reasons := ccbObservationUnknownViewReasons(req); len(reasons) > 0 {
		return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, ccbObservationBlockingViewsFromReasons(req, reasons), reasons...)}, nil
	}
	// Catalog entries marked deferred/unavailable are explicit capability
	// boundaries. Reject before materializing an observation so the model gets
	// a durable receipt and can deterministically choose another view or block.
	if blocking, reasons := ccbObservationUnavailableViewDetails(req); len(reasons) > 0 {
		return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, blocking, reasons...)}, nil
	}
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
	}
	store := mixboard.NewStore("")
	bindingRead, err := store.Read(mixboard.ReadRequest{
		ObservationID: req.ObservationID,
		MixSessionID:  req.MixSessionID,
		Keys:          []string{"observation.binding"},
		MaxItems:      1,
	})
	if err != nil {
		return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, req.ViewIDs, err.Error())}, nil
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
		return map[string]any{"status": "rejected", "bundle": capabilitycontext.RejectedFreeStateObservationScoped(req, req.ViewIDs, err.Error())}, nil
	}
	bundle := capabilitycontext.AssembleFreeStateObservation(req, readResult)
	return map[string]any{
		"status": bundle.Status,
		"bundle": bundle,
	}, nil
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

func ccbObservationCommand(cmd map[string]any, req capabilitycontext.FreeStateObservationRequest) map[string]any {
	out := cloneAnyMap(cmd)
	for _, key := range []string{"view_ids", "views", "view_id", "view", "max_disclosure_bytes", "max_items", "freshness_class"} {
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
