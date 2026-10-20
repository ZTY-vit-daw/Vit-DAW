package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/orchestration"
)

// assertCCBJSONShaped 契约：CCB 工具结果 map 内任意嵌套值必须是 JSON 基元/
// 容器形态。结构体与指针零容忍——严格 map 断言消费者读到它们即得空
// （缺陷链：harness 曾把 FreeStateObservationBundle 结构体直接嵌入结果 map）。
func assertCCBJSONShaped(t *testing.T, path string, value any) {
	t.Helper()
	if value == nil {
		return
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			assertCCBJSONShaped(t, fmt.Sprintf("%s[%d]", path, i), rv.Index(i).Interface())
		}
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			t.Fatalf("non-string map key at %s: %s", path, rv.Type().Key())
		}
		for _, key := range rv.MapKeys() {
			assertCCBJSONShaped(t, path+"."+key.String(), rv.MapIndex(key).Interface())
		}
	default:
		t.Fatalf("non-JSON shape at %s: %T", path, value)
	}
}

// ccbNormalizeContractBundle 覆盖 bundle 全部字段族：字符串/布尔/计数、map、
// 结构体切片、指针区间、嵌套 audit_receipt、枚举 omissions。
func ccbNormalizeContractBundle() capabilitycontext.FreeStateObservationBundle {
	return capabilitycontext.FreeStateObservationBundle{
		SchemaVersion:      "fs.observation_bundle.v1",
		BundleID:           "bundle-normalize-1",
		RequestID:          "req-normalize-1",
		Status:             "ok",
		ReadOnly:           true,
		MutationAuthority:  false,
		ObservationID:      "obs-normalize-1",
		MixSessionID:       "mix-normalize-1",
		TargetRef:          map[string]any{"kind": "track", "id": "1007"},
		ProjectBinding:     map[string]any{"project_uuid": "proj-1", "revision": 7},
		Freshness:          map[string]any{"class": "fresh", "age_ms": 12},
		RequestedViews:     []string{"track.peak_structure"},
		Targets:            []mixboard.TargetRef{{Kind: "track", ID: "1007", Label: "Lead", Source: "ccb_request"}},
		TimeWindow:         &capabilitycontext.FreeStateObservationTimeWindow{StartSeconds: 0.5, EndSeconds: 12.25, Units: "seconds"},
		TopK:               &capabilitycontext.FreeStateObservationTopK{Field: "energy", Dir: "desc", K: 5},
		BatchObservations:  map[string]string{"1007": "obs-normalize-1"},
		Views:              map[string]any{"track.peak_structure": map[string]any{"peak_dbfs": -3.5}},
		EvidenceRefs:       []string{"evidence-normalize-1"},
		Limitations:        []string{"digest only"},
		OmissionReasons:    []string{"budget"},
		Omissions:          map[string]orchestration.OmissionStatus{"track.waveform": orchestration.OmissionByReference},
		DisclosureBytes:    2048,
		MaxDisclosureBytes: 4096,
		AuditReceipt: capabilitycontext.FreeStateObservationAuditReceipt{
			SchemaVersion:         "fs.observation_audit_receipt.v1",
			ReceiptID:             "receipt-normalize-1",
			RequestedBy:           "model",
			ModelRequestedViewIDs: []string{"track.peak_structure"},
			ActualExecutedViewIDs: []string{"track.peak_structure"},
			ViewSetMatches:        true,
			Scope:                 "selected_track",
			Targets:               []mixboard.TargetRef{{Kind: "track", ID: "1007"}},
			TimeWindow:            &capabilitycontext.FreeStateObservationTimeWindow{StartSeconds: 0.5, EndSeconds: 12.25},
			TopK:                  &capabilitycontext.FreeStateObservationTopK{Field: "energy", Dir: "desc", K: 5},
			Freshness:             map[string]any{"class": "fresh"},
			ProjectBinding:        map[string]any{"project_uuid": "proj-1"},
			ProjectRevision:       "rev-7",
			Status:                "ok",
		},
	}
}

func TestCCBObservationBundleMapIsJSONShapedAndLossless(t *testing.T) {
	bundle := ccbNormalizeContractBundle()
	normalized := ccbObservationBundleMap(bundle)
	assertCCBJSONShaped(t, "bundle", normalized)

	if got := normalized["bundle_id"]; got != "bundle-normalize-1" {
		t.Fatalf("bundle_id = %v, want bundle-normalize-1", got)
	}
	if got := normalized["schema_version"]; got != "fs.observation_bundle.v1" {
		t.Fatalf("schema_version = %v", got)
	}
	receipt, ok := normalized["audit_receipt"].(map[string]any)
	if !ok || receipt["receipt_id"] != "receipt-normalize-1" {
		t.Fatalf("audit_receipt round-trip lost receipt_id: %T %v", normalized["audit_receipt"], normalized["audit_receipt"])
	}
	views, ok := normalized["views"].(map[string]any)
	if !ok || len(views) != 1 {
		t.Fatalf("views round-trip lost entries: %T", normalized["views"])
	}
	omissions, ok := normalized["omissions"].(map[string]any)
	if !ok || omissions["track.waveform"] != string(orchestration.OmissionByReference) {
		t.Fatalf("omissions round-trip lost enum: %T %v", normalized["omissions"], normalized["omissions"])
	}
	if got, want := normalized["disclosure_bytes"], float64(2048); got != want {
		t.Fatalf("disclosure_bytes = %v, want %v", got, want)
	}
}

func TestCCBObservationBundleNormalizationKeepsWireValues(t *testing.T) {
	bundle := ccbNormalizeContractBundle()
	oldResult := map[string]any{"status": bundle.Status, "bundle": bundle}
	newResult := map[string]any{"status": bundle.Status, "bundle": ccbObservationBundleMap(bundle)}

	// 缺陷钉住：旧形态（结构体直嵌）在严格 map 断言下确实读到空。
	if _, ok := oldResult["bundle"].(map[string]any); ok {
		t.Fatal("precondition failed: struct embed unexpectedly satisfies strict map assertion")
	}
	// 修复面：归一后严格 map 断言读到完整 bundle。
	newBundle, ok := newResult["bundle"].(map[string]any)
	if !ok || len(newBundle) == 0 {
		t.Fatal("normalized bundle fails strict map assertion or is empty")
	}

	// 值面零变化：两种形态经 json.Marshal 渲染后 unmarshal 深度相等。
	oldJSON, err := json.Marshal(oldResult)
	if err != nil {
		t.Fatal(err)
	}
	newJSON, err := json.Marshal(newResult)
	if err != nil {
		t.Fatal(err)
	}
	var oldAny, newAny any
	if err := json.Unmarshal(oldJSON, &oldAny); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(newJSON, &newAny); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(oldAny, newAny) {
		t.Fatal("normalized result values diverge from struct-embedded result values")
	}
	// 字节面对照（回执证据）：差异只允许来自 bundle 对象内键序（结构体字段序
	// → map 字典序），值面必须一致。
	if string(oldJSON) != string(newJSON) {
		t.Logf("byte-level diff is key order only (struct field order vs sorted map keys)")
		t.Logf("old: %s", oldJSON)
		t.Logf("new: %s", newJSON)
	}
}

func TestCCBObservationRequestRejectedResultIsJSONShapedMap(t *testing.T) {
	h := &Harness{}
	result, err := h.ccbObservationRequest(context.Background(), map[string]any{
		"view_ids": []string{"track.peak_structure", "track.peak_structure"},
	}, nil, "test")
	if err != nil {
		t.Fatal(err)
	}
	assertCCBJSONShaped(t, "result", result)
	if result["status"] != "rejected" {
		t.Fatalf("status = %v, want rejected", result["status"])
	}
	bundle, ok := result["bundle"].(map[string]any)
	if !ok || len(bundle) == 0 {
		t.Fatal("rejected bundle fails strict map assertion or is empty")
	}
	if bundle["status"] != "rejected" {
		t.Fatalf("bundle status = %v, want rejected", bundle["status"])
	}
	blocking, ok := bundle["blocking_view_ids"].([]any)
	if !ok || len(blocking) == 0 || blocking[0] != "track.peak_structure" {
		t.Fatalf("blocking_view_ids round-trip lost: %T %v", bundle["blocking_view_ids"], bundle["blocking_view_ids"])
	}
	receipt, ok := bundle["audit_receipt"].(map[string]any)
	if !ok || receipt["status"] != "rejected" {
		t.Fatalf("rejected audit_receipt round-trip lost: %T %v", bundle["audit_receipt"], bundle["audit_receipt"])
	}
}

func TestCCBObservationCatalogResultIsJSONShapedMap(t *testing.T) {
	h := &Harness{}
	result := h.ccbObservationCatalog(map[string]any{})
	assertCCBJSONShaped(t, "catalog_result", result)
	catalog, ok := result["catalog"].(map[string]any)
	if !ok || len(catalog) == 0 {
		t.Fatal("catalog fails strict map assertion or is empty")
	}
	if catalog["schema_version"] == "" {
		t.Fatal("catalog schema_version lost in normalization")
	}

	dimensions := capabilitycontext.FreeStateObservationDimensions()
	if len(dimensions) == 0 {
		t.Fatal("catalog dimensions unexpectedly empty")
	}
	filtered := h.ccbObservationCatalog(map[string]any{"dimension": dimensions[0]})
	assertCCBJSONShaped(t, "filtered_catalog_result", filtered)
	if filteredCatalog, ok := filtered["catalog"].(map[string]any); !ok || len(filteredCatalog) == 0 {
		t.Fatal("dimension-filtered catalog fails strict map assertion or is empty")
	}

	unknown := h.ccbObservationCatalog(map[string]any{"dimension": "not-a-dimension"})
	assertCCBJSONShaped(t, "unknown_dimension_result", unknown)
	if unknown["status"] != "rejected" {
		t.Fatalf("unknown dimension status = %v, want rejected", unknown["status"])
	}
}
