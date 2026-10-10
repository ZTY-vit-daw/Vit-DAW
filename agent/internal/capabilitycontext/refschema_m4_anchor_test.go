package capabilitycontext

// refschema_m4_anchor_test.go — REFSCHEMA-M4（2026-10-10）承载核实的生成侧锚
// 点钉住：ccbobs_/ccbr_ 的 remainder=compactID 对观察 id 的清洗透传（非哈希、
// 非内容寻址——ccbr_ 族停止条件上交的证据基线）、cap_pack_ 的实例身份语义
// （种子含 generatedAt，同内容异时刻异 id）、bundle 身份字段不进 EvidenceRefs
// 数组（纯响应面→注册即完成的依据）。消费链清单：
// coord/runs/REFSCHEMA-M4/receipt.md。

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestFreeStateBundleAndReceiptIDsAreObservationIDPassthrough(t *testing.T) {
	const obsID = "obs_20260911T115419_a2ef6022376c"
	read := testFreeStateReadResult()
	items := read["items"].(map[string]any)
	binding := items["observation.binding"].(map[string]any)
	binding["observation_id"] = obsID
	bundle := AssembleFreeStateObservation(FreeStateObservationRequest{
		RequestID: "req-m4", ViewIDs: []string{"track.basic_energy"},
	}, read)
	// remainder=观察 id 透传（compactID 取首个非空输入；请求 id 不是成分）。
	// ccbr_ 族缓注册的承载证据即此形态：实含时序戳，非内容寻址哈希短串。
	if want := "ccbobs_" + obsID; bundle.BundleID != want {
		t.Errorf("BundleID = %q, want %q (observation id passthrough)", bundle.BundleID, want)
	}
	if want := "ccbr_" + obsID; bundle.AuditReceipt.ReceiptID != want {
		t.Errorf("ReceiptID = %q, want %q (compactID takes first non-empty input only)", bundle.AuditReceipt.ReceiptID, want)
	}
	// 身份字段不进 EvidenceRefs 数组（纯响应面，注册即完成的依据）。
	for _, ref := range bundle.EvidenceRefs {
		if strings.HasPrefix(ref, "ccbobs_") || strings.HasPrefix(ref, "ccbr_") {
			t.Errorf("bundle identity leaked into EvidenceRefs: %q", ref)
		}
	}
}

func TestRejectedBundleUsesRejectedSubfamilyPrefixes(t *testing.T) {
	const obsID = "obs_20260911T115500_be4567ef"
	bundle := RejectedFreeStateObservation(FreeStateObservationRequest{
		ObservationID: obsID, RequestID: "req-m4-reject",
		ViewIDs: []string{"mix.nonexistent_view"},
	}, "mix.nonexistent_view: semantic view is not in the CCB catalog")
	if !strings.HasPrefix(bundle.BundleID, "ccbobs_rejected_") || !strings.HasSuffix(bundle.BundleID, obsID) {
		t.Errorf("rejected BundleID = %q, want ccbobs_rejected_<observation id>", bundle.BundleID)
	}
	if !strings.HasPrefix(bundle.AuditReceipt.ReceiptID, "ccbr_rejected_") || !strings.HasSuffix(bundle.AuditReceipt.ReceiptID, obsID) {
		t.Errorf("rejected ReceiptID = %q, want ccbr_rejected_<observation id>", bundle.AuditReceipt.ReceiptID)
	}
}

func TestStablePackIDIsInstanceIdentityNotContentFingerprint(t *testing.T) {
	refs := []string{"evidence://one", "obs-1"}
	at := time.Date(2026, 10, 10, 3, 30, 0, 0, time.UTC)
	first := stablePackID(GainStagingCapabilityID, "tighten vocal levels", refs, at)
	if !strings.HasPrefix(first, "cap_pack_") {
		t.Fatalf("PackID = %q, want cap_pack_ prefix", first)
	}
	if !regexp.MustCompile(`^cap_pack_[0-9a-f]{16}$`).MatchString(first) {
		t.Errorf("PackID = %q, want cap_pack_<16 hex>", first)
	}
	same := stablePackID(GainStagingCapabilityID, "tighten vocal levels", refs, at)
	if same != first {
		t.Errorf("same inputs+generatedAt: %q vs %q, want deterministic", same, first)
	}
	// generatedAt 在种子内：同内容异时刻异 id——实例身份，非可复现内容指纹
	// （承载预裁定 identity→slot=snapshot 的核实依据）。
	later := stablePackID(GainStagingCapabilityID, "tighten vocal levels", refs, at.Add(time.Second))
	if later == first {
		t.Errorf("different generatedAt produced identical PackID %q — instance identity semantics broken", first)
	}
}
