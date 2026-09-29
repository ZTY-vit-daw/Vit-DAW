package capabilitycontext

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/orchestration"
)

const (
	FreeStateObservationCatalogSchema = "ccb_observation_catalog.v1"
	FreeStateObservationRequestSchema = "ccb_observation_request.v1"
	FreeStateObservationBundleSchema  = "ccb_observation_bundle.v1"
	FreeStateObservationReceiptSchema = "ccb_observation_receipt.v1"
)

// QUERY_ENGINE_V1_DESIGN §2.6 freshness_class 枚举（缺口 5）。
const (
	FreeStateFreshnessClassCurrentObservation = "current_observation"
	FreeStateFreshnessClassPostAction         = "post_action"
	FreeStateFreshnessClassMaterialReuse      = "material_reuse"
)

var FreeStateObservationFreshnessClasses = []string{
	FreeStateFreshnessClassCurrentObservation,
	FreeStateFreshnessClassPostAction,
	FreeStateFreshnessClassMaterialReuse,
}

type FreeStateObservationView struct {
	ViewID                 string                     `json:"view_id"`
	Questions              []string                   `json:"questions"`
	SupportedTargetKinds   []string                   `json:"supported_target_kinds"`
	TemporalResolution     string                     `json:"temporal_resolution"`
	Availability           string                     `json:"availability"`
	CostLatencyClass       string                     `json:"cost_latency_class"`
	QualityCeiling         string                     `json:"quality_ceiling"`
	Limitations            []string                   `json:"limitations,omitempty"`
	RequiredDependencies   []string                   `json:"required_dependencies,omitempty"`
	DiagnosticDimensions   []string                   `json:"diagnostic_dimensions,omitempty"`
	SortableFields         []string                   `json:"sortable_fields,omitempty"`
	InterpretationGuidance *CCBInterpretationGuidance `json:"interpretation_guidance,omitempty"`
}

type CCBInterpretationGuidance struct {
	Patterns []CCBEvidencePattern `json:"patterns,omitempty"`
}

type CCBEvidencePattern struct {
	Name        string   `json:"name"`
	Signal      string   `json:"signal"`
	Suggests    []string `json:"suggests"`
	Description string   `json:"description"`
}

// FreeStateObservationTimeWindow 是 observe 路径的 §2.3 双标尺观察窗（§2.6 缺口 1）：
// nil = 默认全窗（"all"）；显式区间以 seconds（通用人类层）或 samples（权威层）
// 计。应用面 v1 = masking 预备探测区间（seconds 标尺）；其余投影在内核窗口化
// 落地前保持全窗（bundle limitation 如实记录）。
type FreeStateObservationTimeWindow struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Units        string  `json:"units,omitempty"`
}

// FreeStateObservationTopK 是 observe 路径的 §2.6 top_k 选择（缺口 4）：field
// 必须 ⊆ 请求中声明了 SortableFields 的每个 view 的声明集，越界拒绝（不静默
// 乱序）；k 与 max_items 独立（top_k 决定选哪些行，max_items 保持粗粒度闸）。
type FreeStateObservationTopK struct {
	Field string `json:"field"`
	Dir   string `json:"dir,omitempty"`
	K     int    `json:"k"`
}

type FreeStateObservationCatalog struct {
	SchemaVersion string                     `json:"schema_version"`
	Boundary      string                     `json:"boundary"`
	TargetRef     mixboard.TargetRef         `json:"target_ref,omitempty"`
	Dimension     string                     `json:"dimension,omitempty"`
	Dimensions    []string                   `json:"dimensions,omitempty"`
	Views         []FreeStateObservationView `json:"views"`
	Exclusions    []string                   `json:"exclusions"`
}

type FreeStateObservationRequest struct {
	SchemaVersion      string                          `json:"schema_version"`
	RequestID          string                          `json:"request_id,omitempty"`
	ObservationID      string                          `json:"observation_id,omitempty"`
	MixSessionID       string                          `json:"mix_session_id,omitempty"`
	ViewIDs            []string                        `json:"view_ids"`
	OriginalViewIDs    []string                        `json:"-"`
	TargetRef          mixboard.TargetRef              `json:"target_ref,omitempty"`
	Targets            []mixboard.TargetRef            `json:"targets,omitempty"`
	TimeWindow         *FreeStateObservationTimeWindow `json:"time_window,omitempty"`
	TopK               *FreeStateObservationTopK       `json:"top_k,omitempty"`
	FreshnessClass     string                          `json:"freshness_class,omitempty"`
	RequestedBy        string                          `json:"-"`
	Scope              string                          `json:"-"`
	MaxDisclosureBytes int                             `json:"max_disclosure_bytes,omitempty"`
	MaxItems           int                             `json:"max_items,omitempty"`
}

// FreeStateObservationAuditReceipt is the immutable audit projection for one
// CCB observation request. It records what the model asked for separately from
// what the server actually attempted, so a later consumer can prove that no
// acoustic view was added, removed, or inferred.
type FreeStateObservationAuditReceipt struct {
	SchemaVersion         string                          `json:"schema_version"`
	ReceiptID             string                          `json:"receipt_id"`
	RequestedBy           string                          `json:"requested_by"`
	ModelRequestedViewIDs []string                        `json:"model_requested_view_ids"`
	ActualExecutedViewIDs []string                        `json:"actual_executed_view_ids"`
	ViewSetMatches        bool                            `json:"view_set_matches"`
	Scope                 string                          `json:"scope"`
	Targets               []mixboard.TargetRef            `json:"targets,omitempty"`
	TimeWindow            *FreeStateObservationTimeWindow `json:"time_window,omitempty"`
	TopK                  *FreeStateObservationTopK       `json:"top_k,omitempty"`
	Freshness             map[string]any                  `json:"freshness"`
	ProjectBinding        map[string]any                  `json:"project_binding,omitempty"`
	ProjectRevision       string                          `json:"project_revision,omitempty"`
	Status                string                          `json:"status"`
	RejectionReasons      []string                        `json:"rejection_reasons,omitempty"`
	RejectionScope        string                          `json:"rejection_scope,omitempty"`
	BlockingViewIDs       []string                        `json:"blocking_view_ids,omitempty"`
	NonBlockingViewIDs    []string                        `json:"non_blocking_view_ids,omitempty"`
}

type FreeStateObservationBundle struct {
	SchemaVersion      string                                  `json:"schema_version"`
	BundleID           string                                  `json:"bundle_id"`
	RequestID          string                                  `json:"request_id,omitempty"`
	Status             string                                  `json:"status"`
	ReadOnly           bool                                    `json:"read_only"`
	MutationAuthority  bool                                    `json:"mutation_authority"`
	ObservationID      string                                  `json:"observation_id"`
	MixSessionID       string                                  `json:"mix_session_id,omitempty"`
	TargetRef          map[string]any                          `json:"target_ref,omitempty"`
	ProjectBinding     map[string]any                          `json:"project_binding,omitempty"`
	Freshness          map[string]any                          `json:"freshness"`
	RequestedViews     []string                                `json:"requested_views"`
	Targets            []mixboard.TargetRef                    `json:"targets,omitempty"`
	TimeWindow         *FreeStateObservationTimeWindow         `json:"time_window,omitempty"`
	TopK               *FreeStateObservationTopK               `json:"top_k,omitempty"`
	BatchObservations  map[string]string                       `json:"batch_observations,omitempty"`
	Views              map[string]any                          `json:"views"`
	EvidenceRefs       []string                                `json:"evidence_refs,omitempty"`
	Limitations        []string                                `json:"limitations,omitempty"`
	OmissionReasons    []string                                `json:"omission_reasons,omitempty"`
	Omissions          map[string]orchestration.OmissionStatus `json:"omissions,omitempty"`
	DisclosureBytes    int                                     `json:"disclosure_bytes"`
	MaxDisclosureBytes int                                     `json:"max_disclosure_bytes"`
	AuditReceipt       FreeStateObservationAuditReceipt        `json:"audit_receipt"`
	RejectionScope     string                                  `json:"rejection_scope,omitempty"`
	BlockingViewIDs    []string                                `json:"blocking_view_ids,omitempty"`
	NonBlockingViewIDs []string                                `json:"non_blocking_view_ids,omitempty"`
}

type freeStateViewDefinition struct {
	view FreeStateObservationView
	keys []string
}

func FreeStateObservationCatalogFor(target mixboard.TargetRef) FreeStateObservationCatalog {
	defs := freeStateViewDefinitions(target.ID)
	views := make([]FreeStateObservationView, 0, len(defs))
	for _, def := range defs {
		views = append(views, def.view)
	}
	return FreeStateObservationCatalog{
		SchemaVersion: FreeStateObservationCatalogSchema,
		Boundary:      "semantic_views_only",
		TargetRef:     target,
		Dimensions:    FreeStateObservationDimensions(),
		Views:         views,
		Exclusions: []string{
			"raw PCM or audio buffers",
			"waveform and spectrogram arrays",
			"full DAD packages or evidence blobs",
			"processor selection, parameter decisions, and mutation authority",
		},
	}
}

// FreeStateObservationDimensions 返回目录声明的诊断维度词表（排序去重）。
func FreeStateObservationDimensions() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, def := range freeStateViewDefinitions("") {
		for _, dimension := range def.view.DiagnosticDimensions {
			if dimension == "" || seen[dimension] {
				continue
			}
			seen[dimension] = true
			out = append(out, dimension)
		}
	}
	sort.Strings(out)
	return out
}

// FreeStateObservationCatalogForDimension 按诊断维度过滤目录（§2.6 缺口 6 的目录
// 参数面）。接线 GetViewsForDimension——此前它是全库唯一无生产接线的目录谓词
// 钩子（F7）。dimension 是目录过滤不是推荐：CCB 不做自动 view 选择。ok=false
// 表示 dimension 不在词表内（调用方应拒绝而非静默返回空目录）。
func FreeStateObservationCatalogForDimension(target mixboard.TargetRef, dimension string) (FreeStateObservationCatalog, bool) {
	catalog := FreeStateObservationCatalogFor(target)
	dimension = strings.TrimSpace(dimension)
	known := map[string]bool{}
	for _, candidate := range catalog.Dimensions {
		known[candidate] = true
	}
	if dimension == "" || !known[dimension] {
		return FreeStateObservationCatalog{}, false
	}
	allowed := map[string]bool{}
	for _, viewID := range GetViewsForDimension(dimension, target.ID) {
		allowed[viewID] = true
	}
	views := make([]FreeStateObservationView, 0, len(catalog.Views))
	for _, view := range catalog.Views {
		if allowed[view.ViewID] {
			views = append(views, view)
		}
	}
	catalog.Dimension = dimension
	catalog.Views = views
	return catalog, true
}

func NormalizeFreeStateObservationRequest(req FreeStateObservationRequest) FreeStateObservationRequest {
	req.SchemaVersion = FreeStateObservationRequestSchema
	if len(req.OriginalViewIDs) == 0 {
		req.OriginalViewIDs = append([]string(nil), req.ViewIDs...)
	}
	if strings.TrimSpace(req.FreshnessClass) == "" {
		req.FreshnessClass = "current_observation"
	}
	if req.TimeWindow != nil {
		req.TimeWindow.Units = strings.ToLower(strings.TrimSpace(req.TimeWindow.Units))
		if req.TimeWindow.Units == "" {
			req.TimeWindow.Units = "seconds"
		}
	}
	if req.TopK != nil {
		req.TopK.Field = strings.TrimSpace(req.TopK.Field)
		req.TopK.Dir = strings.ToLower(strings.TrimSpace(req.TopK.Dir))
		if req.TopK.Dir == "" {
			req.TopK.Dir = "desc"
		}
	}
	req.Targets = uniqueFreeStateTargets(req.Targets)
	if req.MaxDisclosureBytes <= 0 {
		// Keep the default within the existing bounded ceiling so a model-owned
		// multi-view request is not silently reduced to a partial observation.
		req.MaxDisclosureBytes = 64 * 1024
	}
	if req.MaxDisclosureBytes < 256 {
		req.MaxDisclosureBytes = 256
	}
	if req.MaxDisclosureBytes > 64*1024 {
		req.MaxDisclosureBytes = 64 * 1024
	}
	if req.MaxItems <= 0 {
		req.MaxItems = 12
	}
	if req.MaxItems > 24 {
		req.MaxItems = 24
	}
	req.ViewIDs = uniqueNonEmpty(req.ViewIDs)
	return req
}

func uniqueFreeStateTargets(targets []mixboard.TargetRef) []mixboard.TargetRef {
	seen := map[string]bool{}
	out := make([]mixboard.TargetRef, 0, len(targets))
	for _, target := range targets {
		target.ID = strings.TrimSpace(target.ID)
		target.Kind = strings.TrimSpace(target.Kind)
		if target.ID == "" || seen[target.ID] {
			continue
		}
		seen[target.ID] = true
		out = append(out, target)
	}
	return out
}

func ValidateFreeStateObservationViewIDs(viewIDs []string) []string {
	reasons := []string{}
	seen := map[string]bool{}
	if len(viewIDs) == 0 {
		return []string{"view_ids must contain at least one identifier"}
	}
	for _, value := range viewIDs {
		viewID := strings.TrimSpace(value)
		if viewID == "" {
			reasons = append(reasons, "view_ids must contain only non-empty identifiers")
			continue
		}
		if seen[viewID] {
			reasons = append(reasons, viewID+": duplicate view identifier")
			continue
		}
		seen[viewID] = true
	}
	return uniqueNonEmpty(reasons)
}

// ValidateFreeStateObservationParameters 校验 §2.6 observe 参数化增量：
// freshness_class 枚举（缺口 5）、time_window 双标尺（缺口 1）、targets 批量
// 护栏（缺口 2——mix.*/project.* 保持 project 域硬定不缩窄）、top_k 声明集
// （缺口 4——越界字段拒绝，不静默乱序）。view 级护栏的 reason 带 "<viewID>: "
// 前缀，供 blocking/non-blocking 拆分复用。
func ValidateFreeStateObservationParameters(req FreeStateObservationRequest) []string {
	reasons := []string{}
	if !containsString(FreeStateObservationFreshnessClasses, strings.TrimSpace(req.FreshnessClass)) {
		reasons = append(reasons, "freshness_class must be one of "+strings.Join(FreeStateObservationFreshnessClasses, "|")+" (got "+strconv.Quote(strings.TrimSpace(req.FreshnessClass))+")")
	}
	if window := req.TimeWindow; window != nil {
		if window.Units != "seconds" && window.Units != "samples" {
			reasons = append(reasons, "time_window.units must be seconds or samples (got "+strconv.Quote(window.Units)+")")
		}
		if window.EndSeconds <= window.StartSeconds {
			reasons = append(reasons, "time_window requires end_seconds greater than start_seconds")
		}
	}
	if len(req.Targets) > 8 {
		reasons = append(reasons, fmt.Sprintf("targets must contain at most 8 entries (got %d)", len(req.Targets)))
	}
	for i, target := range req.Targets {
		if target.ID == "" {
			reasons = append(reasons, fmt.Sprintf("targets[%d].id must be non-empty", i))
		}
		switch strings.ToLower(target.Kind) {
		case "track", "clip", "selection":
		default:
			reasons = append(reasons, fmt.Sprintf("targets[%d].kind must be one of track|clip|selection (got %q)", i, target.Kind))
		}
	}
	if len(req.Targets) > 0 {
		if strings.TrimSpace(req.ObservationID) != "" {
			reasons = append(reasons, "targets are not supported on observation replay: the observation binding is authoritative")
		}
		for _, viewID := range req.ViewIDs {
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(viewID)), "track.") {
				reasons = append(reasons, viewID+": targets batching applies to track.* views only; project-domain views stay project-scoped")
			}
		}
	}
	if topK := req.TopK; topK != nil {
		if topK.Field == "" {
			reasons = append(reasons, "top_k.field must be non-empty")
		}
		switch topK.Dir {
		case "asc", "desc":
		default:
			reasons = append(reasons, "top_k.dir must be asc or desc (got "+strconv.Quote(topK.Dir)+")")
		}
		if topK.K < 1 || topK.K > 24 {
			reasons = append(reasons, "top_k.k must be between 1 and 24")
		}
		reasons = append(reasons, validateFreeStateTopKField(req)...)
	}
	return uniqueNonEmpty(reasons)
}

// validateFreeStateTopKField 强制 top_k.field ⊆ 声明了 SortableFields 的每个
// 请求 view 的声明集（§2.6 缺口 4 右列护栏：任意字段 topK 归 query 侧）。
func validateFreeStateTopKField(req FreeStateObservationRequest) []string {
	byID := map[string]FreeStateObservationView{}
	for _, def := range freeStateViewDefinitions(req.TargetRef.ID) {
		byID[def.view.ViewID] = def.view
	}
	declaring := []string{}
	for _, viewID := range req.ViewIDs {
		if view, ok := byID[viewID]; ok && len(view.SortableFields) > 0 {
			declaring = append(declaring, viewID)
		}
	}
	if len(declaring) == 0 {
		return []string{"top_k requires at least one requested view with declared sortable fields (mix.masking_relationship or mix.multitrack_relationship)"}
	}
	reasons := []string{}
	for _, viewID := range declaring {
		if !containsString(byID[viewID].SortableFields, req.TopK.Field) {
			reasons = append(reasons, viewID+": top_k.field must be one of the view's declared sortable fields ("+strings.Join(byID[viewID].SortableFields, "|")+")")
		}
	}
	return reasons
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func RejectedFreeStateObservation(req FreeStateObservationRequest, reasons ...string) FreeStateObservationBundle {
	return RejectedFreeStateObservationScoped(req, nil, reasons...)
}

func RejectedFreeStateObservationScoped(req FreeStateObservationRequest, blockingViews []string, reasons ...string) FreeStateObservationBundle {
	req = NormalizeFreeStateObservationRequest(req)
	modelViewIDs := append([]string(nil), req.OriginalViewIDs...)
	if len(modelViewIDs) == 0 {
		modelViewIDs = append([]string(nil), req.ViewIDs...)
	}
	rejectionReasons := uniqueNonEmpty(append(ValidateFreeStateObservationViewIDs(modelViewIDs), reasons...))
	blocking := uniqueNonEmpty(blockingViews)
	nonBlocking := subtractStrings(modelViewIDs, blocking)
	return FreeStateObservationBundle{
		SchemaVersion:      FreeStateObservationBundleSchema,
		BundleID:           "ccbobs_rejected_" + compactID(req.ObservationID, req.RequestID),
		RequestID:          req.RequestID,
		Status:             "rejected",
		ReadOnly:           true,
		MutationAuthority:  false,
		ObservationID:      req.ObservationID,
		MixSessionID:       req.MixSessionID,
		RequestedViews:     append([]string(nil), req.ViewIDs...),
		Targets:            append([]mixboard.TargetRef(nil), req.Targets...),
		TimeWindow:         cloneFreeStateTimeWindow(req.TimeWindow),
		TopK:               cloneFreeStateTopK(req.TopK),
		Views:              map[string]any{},
		Freshness:          map[string]any{"class": req.FreshnessClass, "status": "rejected"},
		OmissionReasons:    append([]string(nil), rejectionReasons...),
		RejectionScope:     "exact_view_set",
		BlockingViewIDs:    blocking,
		NonBlockingViewIDs: nonBlocking,
		AuditReceipt: FreeStateObservationAuditReceipt{
			SchemaVersion:         FreeStateObservationReceiptSchema,
			ReceiptID:             "ccbr_rejected_" + compactID(req.ObservationID, req.RequestID),
			RequestedBy:           firstNonEmptyString(req.RequestedBy, "caller"),
			ModelRequestedViewIDs: modelViewIDs,
			ActualExecutedViewIDs: nil,
			ViewSetMatches:        false,
			Scope:                 firstNonEmptyString(req.Scope, observationScopeForRequest(req)),
			Targets:               append([]mixboard.TargetRef(nil), req.Targets...),
			TimeWindow:            cloneFreeStateTimeWindow(req.TimeWindow),
			TopK:                  cloneFreeStateTopK(req.TopK),
			Freshness:             map[string]any{"class": req.FreshnessClass, "status": "rejected"},
			Status:                "rejected",
			RejectionReasons:      rejectionReasons,
			RejectionScope:        "exact_view_set",
			BlockingViewIDs:       blocking,
			NonBlockingViewIDs:    nonBlocking,
		},
	}
}

func subtractStrings(values, remove []string) []string {
	blocked := map[string]bool{}
	for _, value := range remove {
		blocked[strings.TrimSpace(value)] = true
	}
	out := make([]string, 0, len(values))
	for _, value := range uniqueNonEmpty(values) {
		if !blocked[value] {
			out = append(out, value)
		}
	}
	return out
}

func FreeStateObservationReadKeys(req FreeStateObservationRequest, targetID string) []string {
	req = NormalizeFreeStateObservationRequest(req)
	defs := freeStateViewDefinitions(targetID)
	byID := map[string]freeStateViewDefinition{}
	for _, def := range defs {
		byID[def.view.ViewID] = def
	}
	keys := []string{"observation.binding"}
	for _, viewID := range req.ViewIDs {
		keys = append(keys, byID[viewID].keys...)
	}
	return uniqueNonEmpty(keys)
}

func AssembleFreeStateObservation(req FreeStateObservationRequest, readResult map[string]any) FreeStateObservationBundle {
	req = NormalizeFreeStateObservationRequest(req)
	items := anyMap(readResult["items"])
	binding := anyMap(items["observation.binding"])
	observationID := firstNonEmptyString(stringValue(binding["observation_id"]), stringValue(readResult["observation_id"]), req.ObservationID)
	mixSessionID := firstNonEmptyString(stringValue(binding["mix_session_id"]), stringValue(readResult["mix_session_id"]), req.MixSessionID)
	defs := freeStateViewDefinitions(targetIDFromBinding(binding, req.TargetRef.ID))
	byID := map[string]freeStateViewDefinition{}
	for _, def := range defs {
		byID[def.view.ViewID] = def
	}
	bundle := FreeStateObservationBundle{
		SchemaVersion:     FreeStateObservationBundleSchema,
		BundleID:          "ccbobs_" + compactID(observationID, req.RequestID),
		RequestID:         req.RequestID,
		Status:            "ready",
		ReadOnly:          true,
		MutationAuthority: false,
		ObservationID:     observationID,
		MixSessionID:      mixSessionID,
		TargetRef:         anyMap(binding["target_ref"]),
		ProjectBinding:    anyMap(binding["project_binding"]),
		Freshness: map[string]any{
			"class":       req.FreshnessClass,
			"status":      firstNonEmptyString(stringValue(binding["status"]), "unknown"),
			"observed_at": stringValue(binding["created_at"]),
		},
		RequestedViews:     append([]string(nil), req.ViewIDs...),
		Targets:            append([]mixboard.TargetRef(nil), req.Targets...),
		TimeWindow:         cloneFreeStateTimeWindow(req.TimeWindow),
		TopK:               cloneFreeStateTopK(req.TopK),
		Views:              map[string]any{},
		EvidenceRefs:       stringSlice(binding["evidence_refs"]),
		Omissions:          map[string]orchestration.OmissionStatus{},
		MaxDisclosureBytes: req.MaxDisclosureBytes,
	}
	canonicalBound := strings.TrimSpace(stringValue(bundle.ProjectBinding["project_uuid"])) != "" &&
		strings.TrimSpace(stringValue(bundle.ProjectBinding["project_epoch"])) != "" &&
		strings.TrimSpace(stringValue(bundle.ProjectBinding["project_revision"])) != ""
	if !canonicalBound {
		if bundle.ProjectBinding == nil {
			bundle.ProjectBinding = map[string]any{}
		}
		bundle.ProjectBinding["binding_status"] = "unbound"
		bundle.Limitations = append(bundle.Limitations, "canonical VSP project binding unavailable")
		bundle.Freshness["status"] = "unbound"
	}
	// The binding is authoritative lineage.  Do not infer a revision from the
	// MixBoard session/round counters; an observation without a kernel/VSP
	// project binding remains unbound and therefore cannot satisfy G7.
	if projectBinding := anyMap(binding["project_binding"]); len(projectBinding) > 0 {
		for _, key := range []string{"project_uuid", "project_epoch", "project_revision"} {
			if value := stringValue(projectBinding[key]); value != "" {
				bundle.Freshness[key] = value
			}
		}
	}
	knownViewIDs := map[string]bool{}
	for _, def := range defs {
		knownViewIDs[def.view.ViewID] = true
	}
	actualViewIDs := make([]string, 0, len(req.ViewIDs))
	for _, viewID := range req.ViewIDs {
		if knownViewIDs[viewID] && len(byID[viewID].keys) > 0 {
			actualViewIDs = append(actualViewIDs, viewID)
		}
	}
	modelViewIDs := append([]string(nil), req.OriginalViewIDs...)
	if len(modelViewIDs) == 0 {
		modelViewIDs = append([]string(nil), req.ViewIDs...)
	}
	receiptReasons := ValidateFreeStateObservationViewIDs(modelViewIDs)
	if len(receiptReasons) == 0 && !sameStringSet(modelViewIDs, actualViewIDs) {
		receiptReasons = append(receiptReasons, "actual acoustic view set did not exactly match the model request")
	}
	receiptStatus := "executed"
	if len(receiptReasons) > 0 {
		receiptStatus = "rejected"
	}
	bundle.AuditReceipt = FreeStateObservationAuditReceipt{
		SchemaVersion:         FreeStateObservationReceiptSchema,
		ReceiptID:             "ccbr_" + compactID(observationID, req.RequestID),
		RequestedBy:           firstNonEmptyString(req.RequestedBy, "caller"),
		ModelRequestedViewIDs: modelViewIDs,
		ActualExecutedViewIDs: actualViewIDs,
		ViewSetMatches:        len(receiptReasons) == 0,
		Scope:                 firstNonEmptyString(req.Scope, observationScopeForRequest(req)),
		Targets:               append([]mixboard.TargetRef(nil), req.Targets...),
		TimeWindow:            cloneFreeStateTimeWindow(req.TimeWindow),
		TopK:                  cloneFreeStateTopK(req.TopK),
		Freshness:             nil,
		ProjectBinding:        cloneAnyMap(bundle.ProjectBinding),
		ProjectRevision:       stringValue(bundle.ProjectBinding["project_revision"]),
		Status:                receiptStatus,
		RejectionReasons:      receiptReasons,
	}
	for _, viewID := range req.ViewIDs {
		def, known := byID[viewID]
		if !known {
			bundle.Omissions[viewID] = orchestration.OmissionForbidden
			bundle.OmissionReasons = append(bundle.OmissionReasons, viewID+": semantic view is not in the CCB catalog")
			continue
		}
		if len(def.keys) == 0 {
			bundle.Omissions[viewID] = orchestration.OmissionUnavailable
			bundle.OmissionReasons = append(bundle.OmissionReasons, viewID+": "+firstNonEmptyString(firstStringSlice(def.view.Limitations), "view is unavailable"))
			continue
		}
		view, status := assembleFreeStateView(viewID, def, items)
		if status == "missing" || status == "deferred" || status == "unavailable" {
			bundle.Omissions[viewID] = orchestration.OmissionUnavailable
			bundle.OmissionReasons = append(bundle.OmissionReasons, viewID+": source evidence is "+status)
			continue
		}
		if status == "stale" {
			bundle.Omissions[viewID] = orchestration.OmissionStale
			bundle.OmissionReasons = append(bundle.OmissionReasons, viewID+": source evidence is stale")
			continue
		}
		candidate := cloneAnyMap(bundle.Views)
		sanitized := sanitizeFreeStateValue(view, req.MaxItems, 0)
		if req.TopK != nil {
			// §2.6 top_k 选择在通用 sanitize（max_items 粗闸）之后、披露预算之前
			// 应用：top_k 决定选中哪些行（声明集内字段排序），max_items 保持
			// 全局粗粒度闸——两者独立组合。
			applyFreeStateTopKToView(viewID, sanitized, req.TopK)
		}
		candidate[viewID] = sanitized
		if jsonSize(candidate) > req.MaxDisclosureBytes {
			bundle.Omissions[viewID] = orchestration.OmissionBudget
			bundle.OmissionReasons = append(bundle.OmissionReasons, viewID+": omitted by disclosure budget")
			continue
		}
		bundle.Views = candidate
		if status == "partial" || status == "suspect" || status == "approximate" {
			bundle.Limitations = append(bundle.Limitations, viewID+": evidence status is "+status)
		}
	}
	bundle.DisclosureBytes = jsonSize(bundle.Views)
	if req.TimeWindow != nil {
		// 诚实边界：显式观察窗 v1 仅作用于 masking 预备探测区间（seconds 标尺）；
		// 其余投影在内核窗口化落地前保持全窗。该边界必须对模型可见，不得静默
		// 把全窗结果当作已开窗结果。
		bundle.Limitations = append(bundle.Limitations, "time_window is applied to the masking probe range only (seconds scale); other projections remain whole-window until kernel windowing lands")
	}
	if len(bundle.Views) == 0 {
		bundle.Status = "insufficient"
	} else if len(bundle.Omissions) > 0 || len(bundle.Limitations) > 0 {
		bundle.Status = "partial"
	}
	bundle.EvidenceRefs = uniqueNonEmpty(bundle.EvidenceRefs)
	bundle.Limitations = uniqueNonEmpty(bundle.Limitations)
	bundle.OmissionReasons = uniqueNonEmpty(bundle.OmissionReasons)
	bundle.AuditReceipt.Freshness = cloneAnyMap(bundle.Freshness)
	bundle.AuditReceipt.ProjectBinding = cloneAnyMap(bundle.ProjectBinding)
	bundle.AuditReceipt.ProjectRevision = stringValue(bundle.ProjectBinding["project_revision"])
	if len(bundle.OmissionReasons) > 0 {
		bundle.AuditReceipt.RejectionReasons = uniqueNonEmpty(append(bundle.AuditReceipt.RejectionReasons, bundle.OmissionReasons...))
		if bundle.AuditReceipt.Status == "executed" {
			bundle.AuditReceipt.Status = "partial"
		}
	}
	if !canonicalBound && bundle.AuditReceipt.Status == "executed" {
		bundle.AuditReceipt.Status = "partial"
	}
	if len(bundle.AuditReceipt.RejectionReasons) > 0 {
		bundle.AuditReceipt.ViewSetMatches = len(receiptReasons) == 0
	}
	if len(bundle.Omissions) == 0 {
		bundle.Omissions = nil
	}
	return bundle
}

func freeStateViewDefinitions(targetID string) []freeStateViewDefinition {
	if strings.TrimSpace(targetID) == "" {
		targetID = "target"
	}
	track := "track." + targetID
	defs := []freeStateViewDefinition{
		{view: semanticView("project.structure", []string{"What tracks and sources are present?", "What is the current project/selection structure?"}, []string{"project", "track", "selection"}, "state snapshot", "ready_on_observation", "cheap", "compact project and TOM-adjacent identity summary", []string{"Does not disclose a full TOM tree."}, []string{"project state", "MixBoard observation"}), keys: []string{"project.static.summary", "project.tracks.summary", "observation.tim_projection", "observation.mom_projection"}},
		{view: semanticView("project.change_delta", []string{"What deterministic engineering changes occurred since the prior state?", "Which current observations must be refreshed after the latest project change?"}, []string{"project", "track", "clip", "processor"}, "latest state transition", "conditional", "cheap", "bounded Shadow Project change receipt", []string{"Change receipts confirm engineering mutations occurred. Acoustic evaluation requires fresh observation of the new state."}, []string{"Shadow Project change monitor", "MixBoard observation binding"}), keys: []string{"project.change_delta"}},
		{view: semanticViewWithDimensions("track.basic_energy", []string{"How loud and peaky is the target?", "Is headroom or crest factor unusual?"}, []string{"track", "clip", "selection"}, "whole window", "ready_on_observation", "cheap", "bounded level summary", nil, []string{"waveform envelope summary"}, []string{"level_headroom"}, nil), keys: []string{track + ".static.identity", track + ".fast.levels"}},
		{view: semanticViewWithDimensions("track.time_dynamics", []string{"How does energy evolve over time?", "What transient and macro-dynamic structure is observable?"}, []string{"track", "clip", "selection"}, "macro and short-window summary", "conditional", "medium", "COM plus bounded time-energy summaries", []string{"Fine envelopes and event lists stay in evidence storage."}, []string{"time-energy summary", "COM projection"}, []string{"dynamics"}, nil), keys: []string{track + ".slow.time_energy.summary", "observation.com_projection"}},
		{view: semanticViewWithDimensions("track.timbre_frequency", []string{"Where is energy concentrated by band?", "Which broad tonal regions need inspection?"}, []string{"track", "clip", "selection"}, "whole-window band summary", "conditional", "medium", "broad-band energy only", []string{"Not a raw spectrum or a static-EQ decision."}, []string{"band-energy summary"}, []string{"frequency_occupancy"}, nil), keys: []string{track + ".slow.band_energy.summary"}},
		{view: semanticView("track.peak_structure", []string{"How are sample peaks, headroom, and crest distributed?", "Are peak events concentrated or broadly elevated?"}, []string{"track", "clip", "selection"}, "whole-window and bounded segment summary", "conditional", "medium", "sample-peak structure with explicit true-peak limitation", []string{"Sample-peak structure provides bounded clipping-risk evidence. Extreme values suggest peak-management improvement candidates, though true-peak confirmation requires additional measurement."}, []string{"DOM source-only projection"}), keys: []string{"observation.dom_projection"}},
		{view: semanticView("track.activity_structure", []string{"How are active, low-energy, and silent intervals distributed?", "How long are observed low-energy runs?"}, []string{"track", "clip", "selection"}, "bounded segment summary", "conditional", "medium", "declared activity states and interval coverage", []string{"Noise floor and a control threshold are not inferred from coarse states."}, []string{"DOM source-only projection"}), keys: []string{"observation.dom_projection"}},
		{view: semanticView("track.frequency_time_events", []string{"Is frequency energy localized to events over time?", "Which time-localized frequency facts are actually available?"}, []string{"track", "clip", "selection"}, "frequency-time event summary", "conditional", "medium", "bounded time-frequency evidence with explicit omissions", []string{"Whole-window bands never become time-localized events."}, []string{"DOM source-only projection"}), keys: []string{"observation.dom_projection"}},
		{view: semanticViewWithDimensions("track.transient_structure", []string{"What onset, body, and sustain structure is observable?", "How consistent are transient contrasts across events?"}, []string{"track", "clip", "selection"}, "event and envelope summary", "conditional", "medium", "bounded transient evidence with macro-only fallback", []string{"Macro crest does not establish onset or sustain behavior."}, []string{"DOM source-only projection"}, []string{"transient_event"}, nil), keys: []string{"observation.dom_projection"}},
		{view: semanticView("track.band_dynamics", []string{"How does dynamic behavior differ by frequency band?", "Are per-band crest and time variation actually available?"}, []string{"track", "clip", "selection"}, "per-band time summary", "conditional", "medium", "bounded band-dynamics evidence with whole-window fallback", []string{"Whole-window band energy does not establish per-band dynamics."}, []string{"DOM source-only projection"}), keys: []string{"observation.dom_projection"}},
		{view: semanticViewWithDimensions("track.stereo_space", []string{"How wide or correlated is the target?", "Is left-right balance unusual?"}, []string{"track", "clip", "selection"}, "whole-window stereo summary", "conditional", "medium", "balance/correlation summary", nil, []string{"stereo-relation summary"}, []string{"stereo_space"}, nil), keys: []string{track + ".slow.stereo.summary"}},
		{view: semanticViewWithDimensions("mix.multitrack_relationship", []string{"How do track levels and risks relate?", "Which track deserves attention first?"}, []string{"project", "track_group"}, "project snapshot", "conditional", "medium", "bounded MOM and project relationship summaries", []string{"This is observation, not a B2/B3 solver result."}, []string{"MOM multitrack projection", "project acoustic summaries"}, []string{"level_headroom"}, &CCBInterpretationGuidance{
			Patterns: []CCBEvidencePattern{
				{
					Name:        "level_difference",
					Signal:      "consistent RMS or peak differences across tracks",
					Suggests:    []string{"level_imbalance", "track_gain"},
					Description: "Consistent level differences suggest gain adjustment candidates",
				},
			},
		}), keys: []string{"project.relationship_inputs", "project.rankings.level", "project.rankings.peak", "project.risks.headroom", "project.attention.first", "observation.mom_projection"}},
		{view: semanticViewWithDimensions("mix.frequency_relationship", []string{"How do track band occupancies relate?", "Where are broad frequency conflicts plausible?"}, []string{"project", "track_group"}, "project snapshot", "conditional", "medium", "compact MOM frequency relationship projection", []string{"Band overlaps suggest frequency-relationship improvement candidates. This is plausible evidence for bounded experiments, not deterministic masking proof."}, []string{"MOM frequency relationship", "project band-energy coverage"}, []string{"frequency_occupancy"}, nil), keys: []string{"project.frequency_relationship_inputs", "observation.mom_projection"}},
		{view: semanticViewWithDimensions("mix.masking_relationship", []string{"Which directional source pairs and frequency bands are plausible masking-risk improvement candidates?"}, []string{"project", "track_group"}, "synchronized project range", "ready_on_request", "expensive", "compact MOM directional masking-risk projection", []string{"Candidates are relative energetic-risk evidence showing directional energy relationships. Large consistent margins often indicate level-imbalance improvement opportunities; band-specific patterns may indicate frequency considerations. This is plausible evidence for bounded improvement hypotheses, not proof of defects."}, []string{"same-window track_post_fader probes", "DAD masking measurement", "MOM masking projection"}, []string{"level_headroom", "frequency_occupancy"}, &CCBInterpretationGuidance{
			Patterns: []CCBEvidencePattern{
				{
					Name:        "large_consistent_margin",
					Signal:      "median_margin_db > 20 and risk_coverage_ratio > 0.9",
					Suggests:    []string{"level_imbalance", "track_gain"},
					Description: "Large consistent margins across time and bands suggest level-imbalance improvement candidates",
				},
				{
					Name:        "band_specific_margin",
					Signal:      "margin concentrated in specific frequency bands",
					Suggests:    []string{"frequency_conflict", "eq"},
					Description: "Band-specific patterns may indicate frequency-domain considerations",
				},
			},
		}), keys: []string{"observation.mom_projection"}},
		{view: semanticView("processor.identity_and_controls", []string{"Which processor is bound to this observation?", "Which semantic controls are available?"}, []string{"processor"}, "processor state snapshot", "conditional", "cheap", "processor scope only", []string{"COM does not disclose a live control surface; use the processor inspector separately."}, []string{"COM processor scope", "read-only processor inspector"}), keys: []string{"observation.com_projection"}},
		{view: semanticView("processor.behavior", []string{"What gain action and transient/recovery behavior is observed?"}, []string{"processor", "track"}, "paired input/output summary", "conditional", "medium", "compact COM behavior projection", []string{"Requires paired evidence for processor-caused behavior."}, []string{"COM paired_io projection"}), keys: []string{"observation.com_projection"}},
		{view: semanticView("processor.change_delta", []string{"How did processor behavior change between observations?"}, []string{"processor", "track"}, "change delta", "conditional", "medium", "compact COM change projection", []string{"Requires compatible before/after COM evidence."}, []string{"COM change_delta projection"}), keys: []string{"observation.com_projection"}},
		{view: semanticView("comparison.before_after", []string{"What changed between the current and previous observation?"}, []string{"project", "track", "processor"}, "observation pair", "conditional", "cheap", "bounded delta summary", []string{"Requires a prior observation in the same mix session."}, []string{"MixBoard observation history"}), keys: []string{"observation.before_after.latest"}},
	}
	// §2.6 缺口 4：view 声明的可排序字段（top_k.field 的合法集）。声明集对齐
	// 既有硬编码截断列表（CCB-VIEW §3.4）——masking candidates ≤12 与
	// multitrack band_conflict 每行 tracks ≤4 正是 top_k 参数化要解锁的。
	sortableByView := map[string][]string{
		"mix.masking_relationship":    {"median_margin_db", "p90_margin_db", "max_margin_db", "risk_coverage_ratio"},
		"mix.multitrack_relationship": {"unit_energy", "energy_db"},
	}
	for i := range defs {
		if fields, ok := sortableByView[defs[i].view.ViewID]; ok {
			defs[i].view.SortableFields = fields
		}
	}
	return defs
}

func semanticView(id string, questions, targets []string, temporal, availability, cost, ceiling string, limitations, dependencies []string) FreeStateObservationView {
	return semanticViewWithDimensions(id, questions, targets, temporal, availability, cost, ceiling, limitations, dependencies, nil, nil)
}

func semanticViewWithDimensions(id string, questions, targets []string, temporal, availability, cost, ceiling string, limitations, dependencies, dimensions []string, guidance *CCBInterpretationGuidance) FreeStateObservationView {
	return FreeStateObservationView{
		ViewID:                 id,
		Questions:              questions,
		SupportedTargetKinds:   targets,
		TemporalResolution:     temporal,
		Availability:           availability,
		CostLatencyClass:       cost,
		QualityCeiling:         ceiling,
		Limitations:            limitations,
		RequiredDependencies:   dependencies,
		DiagnosticDimensions:   dimensions,
		InterpretationGuidance: guidance,
	}
}

func sameStringSet(left, right []string) bool {
	leftSet := map[string]bool{}
	rightSet := map[string]bool{}
	for _, value := range left {
		leftSet[strings.TrimSpace(value)] = true
	}
	for _, value := range right {
		rightSet[strings.TrimSpace(value)] = true
	}
	if len(leftSet) != len(rightSet) {
		return false
	}
	for value := range leftSet {
		if !rightSet[value] {
			return false
		}
	}
	return true
}

func observationScopeForRequest(req FreeStateObservationRequest) string {
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
	default:
		return "selected_track"
	}
}

func assembleFreeStateView(viewID string, def freeStateViewDefinition, items map[string]any) (map[string]any, string) {
	facts := map[string]any{}
	useful := 0
	degraded := false
	stale := false
	for _, key := range def.keys {
		optionalSupplemental := optionalSupplementalProjection(viewID, key)
		value, ok := items[key]
		if !ok || value == nil {
			// Supplemental projections remain visible as missing nested facts, but
			// must not make the authoritative view unobservable on their own.
			if !optionalSupplemental {
				degraded = true
			}
			continue
		}
		itemStatus := semanticItemStatus(viewID, key, value)
		facts[key] = compactProjectionForView(viewID, key, value)
		if optionalSupplemental {
			continue
		}
		switch itemStatus {
		case "missing", "deferred", "unavailable", "blocked", "requested", "pending":
			degraded = true
		case "stale":
			stale = true
		case "partial", "suspect", "approximate":
			useful++
			degraded = true
		default:
			useful++
		}
	}
	status := "ready"
	if len(facts) == 0 {
		status = "missing"
	} else if useful == 0 && stale {
		status = "stale"
	} else if useful == 0 {
		status = "missing"
	} else if stale || degraded {
		status = "partial"
	}
	return map[string]any{
		"view_id":     viewID,
		"status":      status,
		"facts":       facts,
		"limitations": def.view.Limitations,
	}, status
}

func optionalSupplementalProjection(viewID, key string) bool {
	if viewID == "project.structure" && key == "observation.tim_projection" {
		return true
	}
	if viewID != "mix.multitrack_relationship" {
		return false
	}
	// MOM's multitrack relation is the authoritative project-wide comparison.
	// Rankings, headroom rows, and first-attention hints are useful supplements;
	// a missing one must not downgrade an otherwise ready relation to partial.
	return strings.HasPrefix(key, "project.rankings.") || key == "project.risks.headroom" || key == "project.attention.first"
}

func semanticItemStatus(viewID, key string, value any) string {
	row := anyMap(value)
	status := strings.ToLower(strings.TrimSpace(stringValue(row["status"])))
	if key == "observation.mom_projection" {
		switch viewID {
		case "project.structure":
			status = stringValue(anyMap(row["project_structure"])["status"])
		case "mix.multitrack_relationship":
			status = stringValue(anyMap(row["multitrack_relation"])["status"])
		case "mix.frequency_relationship":
			status = stringValue(anyMap(row["frequency_relationship"])["status"])
		case "mix.masking_relationship":
			status = stringValue(anyMap(row["masking_relationship"])["status"])
		}
	}
	if key == "observation.com_projection" {
		switch viewID {
		case "track.time_dynamics":
			status = stringValue(anyMap(row["source_dynamics"])["status"])
		case "processor.identity_and_controls":
			if len(anyMap(row["processor_scope"])) == 0 {
				return "missing"
			}
			return "partial"
		case "processor.behavior":
			if !hasAnyField(row, "gain_action", "transient_response", "recovery_motion", "level_effect", "stereo_behavior", "trigger_relation") {
				return "missing"
			}
		case "processor.change_delta":
			if len(anyMap(row["behavior_change"])) == 0 {
				return "missing"
			}
		}
	}
	if key == "observation.dom_projection" {
		field := map[string]string{
			"track.peak_structure":        "peak_structure",
			"track.activity_structure":    "activity_structure",
			"track.frequency_time_events": "frequency_time_events",
			"track.transient_structure":   "transient_structure",
			"track.band_dynamics":         "band_dynamics",
		}[viewID]
		if field != "" {
			rootStatus := strings.ToLower(strings.TrimSpace(status))
			switch rootStatus {
			case "missing", "stale", "suspect", "unsupported", "blocked":
				return rootStatus
			default:
				status = stringValue(anyMap(row[field])["status"])
			}
		}
	}
	if strings.TrimSpace(status) == "" {
		return "ready"
	}
	return strings.ToLower(strings.TrimSpace(status))
}

func hasAnyField(row map[string]any, fields ...string) bool {
	for _, field := range fields {
		if value, ok := row[field]; ok && value != nil && len(anyMap(value)) > 0 {
			return true
		}
	}
	return false
}

func compactProjectionForView(viewID, key string, value any) any {
	row := anyMap(value)
	if len(row) == 0 {
		return value
	}
	if key == "project.tracks.summary" && viewID == "project.structure" {
		return compactProjectTracksSummaryForStructure(row)
	}
	if key == "observation.com_projection" {
		switch viewID {
		case "processor.identity_and_controls":
			return selectFields(row, "schema_version", "com_version", "projection_id", "mode", "status", "processor_scope", "identifiability", "trust_quality", "evidence_refs", "limitations")
		case "processor.behavior":
			return selectFields(row, "schema_version", "com_version", "projection_id", "mode", "status", "processor_scope", "gain_action", "transient_response", "recovery_motion", "level_effect", "stereo_behavior", "trigger_relation", "identifiability", "trust_quality", "llm_context", "evidence_refs", "limitations")
		case "processor.change_delta":
			return selectFields(row, "schema_version", "com_version", "projection_id", "mode", "status", "processor_scope", "behavior_change", "trust_quality", "llm_context", "evidence_refs", "limitations")
		case "track.time_dynamics":
			return compactCOMSourceDynamicsForSelection(row)
		}
	}
	if key == "observation.dom_projection" {
		field := map[string]string{
			"track.peak_structure":        "peak_structure",
			"track.activity_structure":    "activity_structure",
			"track.frequency_time_events": "frequency_time_events",
			"track.transient_structure":   "transient_structure",
			"track.band_dynamics":         "band_dynamics",
		}[viewID]
		if field != "" {
			return compactDOMDimension(row, field)
		}
	}
	if key == "observation.tim_projection" {
		return selectFields(row, "schema_version", "tim_version", "status", "technical_summary", "coverage", "risk_summary", "evidence_refs", "limitations")
	}
	if key == "observation.mom_projection" {
		switch viewID {
		case "project.structure":
			return map[string]any{
				"mom_version":       row["mom_version"],
				"intent":            row["intent"],
				"project_structure": compactMOMProjectStructureForView(anyMap(row["project_structure"])),
				"trust_quality":     compactMOMTrustQualityForView(anyMap(row["trust_quality"])),
				"llm_context":       row["llm_context"],
			}
		case "mix.multitrack_relationship":
			return map[string]any{
				"mom_version":         row["mom_version"],
				"intent":              row["intent"],
				"multitrack_relation": compactMOMMultitrackRelation(anyMap(row["multitrack_relation"])),
				"llm_context":         row["llm_context"],
			}
		case "mix.frequency_relationship":
			return selectFields(row, "mom_version", "intent", "frequency_relationship", "llm_context")
		case "mix.masking_relationship":
			// Masking is an already-compact MOM projection. Do not carry the
			// full generic LLMContext beside it: that duplicates policy text and
			// can push a valid masking view out of a mixed-request disclosure
			// budget. The bounded directional candidates and their conditions are
			// the authoritative model-facing payload here.
			return map[string]any{
				"mom_version":          row["mom_version"],
				"intent":               row["intent"],
				"masking_relationship": compactMOMMaskingRelationshipForView(anyMap(row["masking_relationship"])),
			}
		}
	}
	if key == "project.frequency_relationship_inputs" {
		return compactFrequencyRelationshipInputsForView(row)
	}
	return value
}

func compactProjectTracksSummaryForStructure(row map[string]any) map[string]any {
	out := selectFields(row, "status", "track_count", "active_track_count")
	tracks := rowsValue(row["tracks"])
	compactTracks := make([]any, 0, len(tracks))
	for _, track := range tracks {
		if len(track) == 0 {
			continue
		}
		compactTracks = append(compactTracks, selectFields(track, "track_id", "name", "track_name", "role_guess", "active_state", "source_status"))
	}
	if len(compactTracks) > 0 {
		out["tracks"] = compactTracks
	}
	return out
}

func compactMOMProjectStructureForView(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	return selectFields(row,
		"status", "freshness", "project_id", "session_id", "track_id", "clip_id", "gui_id",
		"source_revision_status", "clip_revision_status", "render_revision_status", "duration_seconds",
		"sample_rate", "channel_count", "target_ref", "listen_scope", "evidence_refs", "limitations",
	)
}

func compactMOMTrustQualityForView(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	return selectFields(row, "schema_version", "overall_status", "source_revision_status", "clip_revision_status", "render_revision_status", "limitations", "evidence_refs")
}

// compactFrequencyRelationshipInputsForView keeps the project-level facts
// needed to interpret a frequency relationship without leaking the source
// project package or allowing a large track payload to consume the CCB budget.
func compactFrequencyRelationshipInputsForView(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	out := selectFields(row, "schema_version", "status", "track_count", "usable_track_count", "tap_points", "decision_tracks_truncated")
	tracks := []map[string]any{}
	for _, track := range rowsValue(row["tracks"]) {
		compact := selectFields(track, "track_id", "name", "status", "tap_point")
		if len(compact) > 0 {
			tracks = append(tracks, compact)
		}
	}
	if len(tracks) > 0 {
		out["tracks"] = tracks
	}
	return out
}

func compactDOMDimension(row map[string]any, dimension string) map[string]any {
	out := selectFields(row, "schema_version", "dom_version", "projection_id", "mode", "status", "freshness", dimension, "evidence_refs", "limitations")
	if conditions := anyMap(row["conditions"]); len(conditions) > 0 {
		out["conditions"] = conditions
	}
	readiness := []any{}
	switch values := row["dimension_readiness"].(type) {
	case []any:
		for _, value := range values {
			if stringValue(anyMap(value)["dimension"]) == dimension {
				readiness = append(readiness, value)
			}
		}
	case []map[string]any:
		for _, value := range values {
			if stringValue(value["dimension"]) == dimension {
				readiness = append(readiness, value)
			}
		}
	}
	if len(readiness) > 0 {
		out["dimension_readiness"] = readiness
	}
	if trust := anyMap(row["trust_quality"]); len(trust) > 0 {
		out["trust_quality"] = selectFields(trust,
			"overall_status", "can_support_source_description", "can_support_family_selection",
			"can_support_behavior_observation", "can_support_post_action_evaluation",
			"coverage", "blocked_reasons", "limitations",
		)
	}
	return out
}

func compactCOMSourceDynamicsForSelection(row map[string]any) map[string]any {
	source := anyMap(row["source_dynamics"])
	trust := anyMap(row["trust_quality"])
	compactSource := selectFields(source,
		"schema_version", "status", "declared_scale", "analyzed_duration_seconds",
		"levels", "activity", "macro_dynamics", "events", "evidence_refs",
	)
	if scales, ok := source["time_scale_coverage"].([]any); ok {
		bounded := make([]any, 0, 2)
		for _, value := range scales {
			scale := anyMap(value)
			switch strings.ToLower(stringValue(scale["scale"])) {
			case "macro_program", "micro_transient":
				bounded = append(bounded, scale)
			}
		}
		if len(bounded) > 0 {
			compactSource["time_scale_coverage"] = bounded
		}
	}
	canDescribe, _ := trust["can_support_source_description"].(bool)
	selectionStatus := "unavailable"
	if canDescribe {
		selectionStatus = "supported_with_limitations"
	}
	return map[string]any{
		"schema_version":  row["schema_version"],
		"com_version":     row["com_version"],
		"projection_id":   row["projection_id"],
		"mode":            row["mode"],
		"status":          row["status"],
		"source_dynamics": compactSource,
		"decision_support": map[string]any{
			"current_decision":                   "processor_family_selection",
			"status":                             selectionStatus,
			"can_describe_source_dynamics":       canDescribe,
			"supports":                           []string{"source_macro_dynamics", "treatment_family_selection"},
			"does_not_support":                   []string{"compressor_behavior_claims", "compressor_parameter_values", "post_action_evaluation"},
			"non_blocking_constraints":           []string{"micro_transient_detail_unresolved", "preserve_transients_conservatively"},
			"downstream_evidence_owner":          "governed_processor_workflow",
			"paired_processor_io_required_after": "processor_selection",
		},
		"evidence_refs": row["evidence_refs"],
	}
}

func compactMOMMultitrackRelation(relation map[string]any) map[string]any {
	if len(relation) == 0 {
		return nil
	}
	level := anyMap(relation["level_distribution"])
	out := selectFields(relation, "status", "freshness", "track_count", "limitations", "evidence_refs")
	out["coverage"] = map[string]any{
		"compared_track_count": anyListLength(relation["compared_tracks"]),
		"missing_track_count":  anyListLength(relation["missing_tracks"]),
	}
	if len(level) > 0 {
		out["level_summary"] = selectFields(level, "status", "track_count", "tracks_with_level", "spread_db", "highest", "lowest")
	}
	out["risk_summary"] = map[string]any{
		"band_occupancy_count": anyListLength(relation["band_occupancy"]),
		"band_conflict_count":  anyListLength(relation["band_conflict_candidates"]),
		"phase_risk_count":     anyListLength(relation["phase_risk_tracks"]),
	}
	// Counts alone are not actionable. Keep a bounded, family-neutral view of
	// the actual candidate rows so the model can choose a focused follow-up
	// observation without receiving a processor or treatment recommendation.
	if candidates := rowsValue(relation["band_conflict_candidates"]); len(candidates) > 0 {
		rows := make([]any, 0, len(candidates))
		for _, candidate := range candidates {
			row := selectFields(candidate, "type", "status", "band", "evidence_ref", "tracks")
			tracks := rowsValue(row["tracks"])
			if len(tracks) > 0 {
				compactTracks := make([]any, 0, len(tracks))
				for i, track := range tracks {
					if i >= 4 {
						break
					}
					compactTracks = append(compactTracks, selectFields(track, "track_id", "name", "role_guess", "unit_energy", "energy_db"))
				}
				row["tracks"] = compactTracks
			}
			rows = append(rows, row)
			if len(rows) >= 12 {
				break
			}
		}
		out["band_conflict_candidates"] = rows
	}
	return out
}

func compactMOMMaskingRelationshipForView(relation map[string]any) map[string]any {
	if len(relation) == 0 {
		return nil
	}
	out := selectFields(relation,
		"schema_version", "status", "freshness", "measurement_id", "model_version", "candidate_only", "evidence_refs", "limitations")
	out["conditions"] = selectFields(anyMap(relation["conditions"]),
		"tap_point", "range_start_seconds", "range_end_seconds", "tail_seconds", "sample_rate", "analyzer_revision", "synchronized", "frame_count_per_track")
	coverage := selectFields(anyMap(relation["coverage"]),
		"track_count", "eligible_track_count", "candidate_count", "projected_candidate_count", "candidates_truncated", "complete_coverage")
	if len(coverage) > 0 {
		out["coverage"] = coverage
	}
	candidates := rowsValue(relation["candidates"])
	if len(candidates) > 12 {
		candidates = candidates[:12]
		coverage["candidates_truncated"] = true
		out["coverage"] = coverage
	}
	compactCandidates := make([]any, 0, len(candidates))
	for _, candidate := range candidates {
		compact := selectFields(candidate,
			"masker_track_id", "masker_track_name", "target_track_id", "target_track_name", "band_id",
			"min_hz", "max_hz", "active_frame_count", "risk_frame_count", "risk_coverage_ratio",
			"median_margin_db", "p90_margin_db", "max_margin_db")
		if len(compact) > 0 {
			compactCandidates = append(compactCandidates, compact)
		}
	}
	if len(compactCandidates) > 0 {
		out["candidates"] = compactCandidates
	}
	return out
}

func anyListLength(value any) int {
	if rows, ok := value.([]any); ok {
		return len(rows)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	var rows []any
	if json.Unmarshal(data, &rows) != nil {
		return 0
	}
	return len(rows)
}

func selectFields(row map[string]any, fields ...string) map[string]any {
	out := map[string]any{}
	for _, field := range fields {
		if value, ok := row[field]; ok && value != nil {
			out[field] = value
		}
	}
	return out
}

func sanitizeFreeStateValue(value any, maxItems, depth int) any {
	if depth > 10 {
		return map[string]any{"status": "omitted", "reason": "depth_limit"}
	}
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, child := range typed {
			if freeStateForbiddenField(key) {
				continue
			}
			out[key] = sanitizeFreeStateValue(child, maxItems, depth+1)
		}
		return out
	case []any:
		if maxItems > 0 && len(typed) > maxItems {
			typed = typed[:maxItems]
		}
		out := make([]any, 0, len(typed))
		for _, child := range typed {
			out = append(out, sanitizeFreeStateValue(child, maxItems, depth+1))
		}
		return out
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprint(value)
		}
		var normalized any
		if json.Unmarshal(data, &normalized) == nil {
			if _, changed := normalized.(map[string]any); changed {
				return sanitizeFreeStateValue(normalized, maxItems, depth+1)
			}
			if _, changed := normalized.([]any); changed {
				return sanitizeFreeStateValue(normalized, maxItems, depth+1)
			}
		}
		return value
	}
}

func freeStateForbiddenField(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if strings.HasPrefix(key, "raw_") || strings.HasSuffix(key, "_samples") {
		return true
	}
	switch key {
	case "pcm", "pcm_data", "samples", "audio_buffer", "waveform_array", "spectrogram_tiles", "time_segments", "event_list", "observation_path", "board_path", "context_pack_path", "file_path", "source_path":
		return true
	default:
		return false
	}
}

func targetIDFromBinding(binding map[string]any, fallback string) string {
	return firstNonEmptyString(stringValue(anyMap(binding["target_ref"])["id"]), fallback)
}

func anyMap(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	data, _ := json.Marshal(value)
	var row map[string]any
	_ = json.Unmarshal(data, &row)
	return row
}

func cloneAnyMap(row map[string]any) map[string]any {
	out := make(map[string]any, len(row)+1)
	for key, value := range row {
		out[key] = value
	}
	return out
}

func stringSlice(value any) []string {
	values := []string{}
	switch typed := value.(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		for _, item := range typed {
			values = append(values, stringValue(item))
		}
	}
	return uniqueNonEmpty(values)
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstStringSlice(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
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

func compactID(values ...string) string {
	value := firstNonEmptyString(values...)
	if value == "" {
		return "unbound"
	}
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, value)
	if len(value) > 80 {
		value = value[:80]
	}
	return value
}

func jsonSize(value any) int {
	data, _ := json.Marshal(value)
	return len(data)
}

func SortedFreeStateObservationViewIDs() []string {
	ids := []string{}
	for _, def := range freeStateViewDefinitions("target") {
		ids = append(ids, def.view.ViewID)
	}
	sort.Strings(ids)
	return ids
}

// GetViewsForDimension returns view IDs that support the specified diagnostic dimension.
// This allows the priority queue to recommend views based on the current dimension focus.
func GetViewsForDimension(dimension string, targetID string) []string {
	defs := freeStateViewDefinitions(targetID)
	var views []string
	for _, def := range defs {
		for _, dim := range def.view.DiagnosticDimensions {
			if dim == dimension {
				views = append(views, def.view.ViewID)
				break
			}
		}
	}
	return views
}

// ——— §2.6 observe 参数化增量（CCB-PARAM）：top_k 应用与 targets 批量合并 ———

func cloneFreeStateTimeWindow(window *FreeStateObservationTimeWindow) *FreeStateObservationTimeWindow {
	if window == nil {
		return nil
	}
	copied := *window
	return &copied
}

func cloneFreeStateTopK(topK *FreeStateObservationTopK) *FreeStateObservationTopK {
	if topK == nil {
		return nil
	}
	copied := *topK
	return &copied
}

// applyFreeStateTopKToView 在已 sanitize 的 view 装配产物上应用 top_k 选择。
// 只作用于该 view 声明了 SortableFields 的列表路径；路径缺失（view 本就
// omitted/missing）时静默跳过。选择结果以 top_k 标记字段写回，保持可审计。
func applyFreeStateTopKToView(viewID string, value any, topK *FreeStateObservationTopK) {
	view := anyMap(value)
	if len(view) == 0 || topK == nil {
		return
	}
	facts, _ := view["facts"].(map[string]any)
	if len(facts) == 0 {
		return
	}
	mom := anyMap(facts["observation.mom_projection"])
	if len(mom) == 0 {
		return
	}
	marker := map[string]any{"field": topK.Field, "dir": topK.Dir, "k": topK.K}
	switch viewID {
	case "mix.masking_relationship":
		relation := anyMap(mom["masking_relationship"])
		if len(relation) == 0 {
			return
		}
		if rows, ok := anyList(relation["candidates"]); ok && len(rows) > 0 {
			relation["candidates"] = topKFreeStateRows(rows, topK)
			marker["applied_to"] = "candidates"
			relation["top_k"] = marker
		}
	case "mix.multitrack_relationship":
		relation := anyMap(mom["multitrack_relation"])
		if len(relation) == 0 {
			return
		}
		rows, ok := anyList(relation["band_conflict_candidates"])
		if !ok || len(rows) == 0 {
			return
		}
		applied := false
		for _, row := range rows {
			candidate := anyMap(row)
			if len(candidate) == 0 {
				continue
			}
			if tracks, ok := anyList(candidate["tracks"]); ok && len(tracks) > 0 {
				candidate["tracks"] = topKFreeStateRows(tracks, topK)
				applied = true
			}
		}
		if applied {
			marker["applied_to"] = "band_conflict_candidates.tracks"
			relation["top_k"] = marker
		}
	}
}

// topKFreeStateRows 按声明字段排序并截断到 k：数值不可比较的行稳定排在末尾
// （截断优先淘汰），不静默乱序。
func topKFreeStateRows(rows []any, topK *FreeStateObservationTopK) []any {
	numeric := make([]float64, len(rows))
	comparable := make([]bool, len(rows))
	for i, row := range rows {
		numeric[i], comparable[i] = numericFreeStateField(anyMap(row), topK.Field)
	}
	ordered := make([]int, 0, len(rows))
	tail := make([]int, 0)
	for i := range rows {
		if comparable[i] {
			ordered = append(ordered, i)
		} else {
			tail = append(tail, i)
		}
	}
	ascending := topK.Dir == "asc"
	sort.SliceStable(ordered, func(a, b int) bool {
		if ascending {
			return numeric[ordered[a]] < numeric[ordered[b]]
		}
		return numeric[ordered[a]] > numeric[ordered[b]]
	})
	ordered = append(ordered, tail...)
	if topK.K > 0 && len(ordered) > topK.K {
		ordered = ordered[:topK.K]
	}
	out := make([]any, 0, len(ordered))
	for _, i := range ordered {
		out = append(out, rows[i])
	}
	return out
}

func numericFreeStateField(row map[string]any, field string) (float64, bool) {
	if row == nil {
		return 0, false
	}
	value, ok := row[field]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	}
	return 0, false
}

func anyList(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, row := range typed {
			out = append(out, row)
		}
		return out, true
	}
	return nil, false
}

// FreeStateObservationBatchEntry 配对一个 fan-out 目标与其装配 bundle。
type FreeStateObservationBatchEntry struct {
	Target mixboard.TargetRef
	Bundle FreeStateObservationBundle
}

// MergeFreeStateObservationBundles 合并 targets 批量请求的逐目标 bundle
// （§2.6 缺口 2：track.* view 批量）。批量视图键带 "@<targetID>" 后缀
// （单目标请求不经此路径、键形不变）；回执记录模型请求的 view 集，执行集
// 为后缀键——targets 在回执中原样可见，保证精确集合审计可解释。
func MergeFreeStateObservationBundles(req FreeStateObservationRequest, entries []FreeStateObservationBatchEntry) FreeStateObservationBundle {
	req = NormalizeFreeStateObservationRequest(req)
	if len(entries) == 0 {
		return RejectedFreeStateObservationScoped(req, req.ViewIDs, "targets batch produced no observation bundles")
	}
	if len(entries) == 1 {
		merged := entries[0].Bundle
		merged.RequestedViews = append([]string(nil), req.ViewIDs...)
		merged.Targets = append([]mixboard.TargetRef(nil), req.Targets...)
		merged.TimeWindow = cloneFreeStateTimeWindow(req.TimeWindow)
		merged.TopK = cloneFreeStateTopK(req.TopK)
		merged.BatchObservations = map[string]string{compactID(entries[0].Target.ID): entries[0].Bundle.ObservationID}
		merged.AuditReceipt.Targets = append([]mixboard.TargetRef(nil), req.Targets...)
		merged.AuditReceipt.TimeWindow = cloneFreeStateTimeWindow(req.TimeWindow)
		merged.AuditReceipt.TopK = cloneFreeStateTopK(req.TopK)
		return merged
	}
	merged := entries[0].Bundle
	merged.BundleID = "ccbobs_batch_" + compactID(req.ObservationID, req.RequestID)
	merged.RequestedViews = append([]string(nil), req.ViewIDs...)
	merged.Targets = append([]mixboard.TargetRef(nil), req.Targets...)
	merged.TimeWindow = cloneFreeStateTimeWindow(req.TimeWindow)
	merged.TopK = cloneFreeStateTopK(req.TopK)
	views := map[string]any{}
	omissions := map[string]orchestration.OmissionStatus{}
	omissionReasons := []string{}
	limitations := append([]string(nil), entries[0].Bundle.Limitations...)
	evidenceRefs := append([]string(nil), entries[0].Bundle.EvidenceRefs...)
	executed := []string{}
	observations := map[string]string{}
	rejected := false
	for _, entry := range entries {
		suffix := "@" + compactID(entry.Target.ID)
		observations[compactID(entry.Target.ID)] = entry.Bundle.ObservationID
		if entry.Bundle.Status == "rejected" {
			rejected = true
		}
		for viewID, content := range entry.Bundle.Views {
			views[viewID+suffix] = content
			executed = append(executed, viewID+suffix)
		}
		for viewID, status := range entry.Bundle.Omissions {
			omissions[viewID+suffix] = status
		}
		omissionReasons = append(omissionReasons, rekeyFreeStateReasons(entry.Bundle.OmissionReasons, req.ViewIDs, suffix)...)
		limitations = append(limitations, rekeyFreeStateReasons(entry.Bundle.Limitations, req.ViewIDs, suffix)...)
		evidenceRefs = append(evidenceRefs, entry.Bundle.EvidenceRefs...)
	}
	merged.Views = views
	merged.Omissions = omissions
	merged.OmissionReasons = uniqueNonEmpty(omissionReasons)
	merged.Limitations = uniqueNonEmpty(limitations)
	merged.EvidenceRefs = uniqueNonEmpty(evidenceRefs)
	merged.BatchObservations = observations
	merged.DisclosureBytes = jsonSize(views)
	merged.MaxDisclosureBytes = req.MaxDisclosureBytes
	merged.BlockingViewIDs = nil
	merged.NonBlockingViewIDs = nil
	merged.RejectionScope = ""
	switch {
	case rejected:
		merged.Status = "rejected"
	case len(views) == 0:
		merged.Status = "insufficient"
	case len(omissions) > 0 || len(merged.Limitations) > 0:
		merged.Status = "partial"
	default:
		merged.Status = "ready"
	}
	receipt := merged.AuditReceipt
	receipt.ReceiptID = "ccbr_batch_" + compactID(req.ObservationID, req.RequestID)
	receipt.ModelRequestedViewIDs = append([]string(nil), req.OriginalViewIDs...)
	receipt.ActualExecutedViewIDs = uniqueNonEmpty(executed)
	receipt.Targets = append([]mixboard.TargetRef(nil), req.Targets...)
	receipt.TimeWindow = cloneFreeStateTimeWindow(req.TimeWindow)
	receipt.TopK = cloneFreeStateTopK(req.TopK)
	stripped := make([]string, 0, len(executed))
	for _, key := range executed {
		if at := strings.LastIndex(key, "@"); at > 0 {
			stripped = append(stripped, key[:at])
		} else {
			stripped = append(stripped, key)
		}
	}
	receipt.ViewSetMatches = sameStringSet(receipt.ModelRequestedViewIDs, stripped) && !rejected
	receipt.Status = "executed"
	if rejected || len(merged.OmissionReasons) > 0 {
		receipt.Status = "partial"
		if rejected {
			receipt.Status = "rejected"
		}
	}
	receipt.RejectionReasons = uniqueNonEmpty(append(append([]string(nil), receipt.RejectionReasons...), merged.OmissionReasons...))
	merged.AuditReceipt = receipt
	if len(merged.Omissions) == 0 {
		merged.Omissions = nil
	}
	return merged
}

// rekeyFreeStateReasons 把子 bundle 的 view 级 reason 前缀（"<viewID>: ..."）
// 重写为批量键（"<viewID>@<target>: ..."）；无法匹配前缀的 reason 原样保留。
func rekeyFreeStateReasons(reasons []string, viewIDs []string, suffix string) []string {
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		rekeyed := false
		for _, viewID := range viewIDs {
			if strings.HasPrefix(reason, viewID+":") {
				out = append(out, viewID+suffix+reason[len(viewID):])
				rekeyed = true
				break
			}
		}
		if !rekeyed {
			out = append(out, reason)
		}
	}
	return out
}
