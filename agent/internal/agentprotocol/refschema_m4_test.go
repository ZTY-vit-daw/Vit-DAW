package agentprotocol

// refschema_m4_test.go — REFSCHEMA-M4（2026-10-10）capabilitycontext 批次注册
// 词条的解析面测试：ccbobs_ 三形态 + cap_pack_ 翻译表、注册表首组前缀包含对
// 的最长匹配行为、ccbr_ 族停止条件缓注册的 opaque 钉住、legacy 分解往返恒等。
// 证据与承载裁定见 coord/runs/REFSCHEMA-M4/receipt.md。

import (
	"reflect"
	"strings"
	"testing"
)

// TestRefSchemaM4LegacyTranslationTable 钉住四个新词条的翻译五元组。样本取
// webui trace fixture 实录形态（ccbr_ 前缀除外——该族缓注册，见 opaque 钉住
// 测试）。
func TestRefSchemaM4LegacyTranslationTable(t *testing.T) {
	cases := []struct {
		raw    string
		prefix string
		family string
		kind   string
		slot   string
		value  string
	}{
		{"ccbobs_obs_20260911T115419_a2ef6022376c", "ccbobs_", RefFamilyEvidenceSchemeURI, "", RefSlotSnapshot, "obs_20260911T115419_a2ef6022376c"},
		{"ccbobs_rejected_obs_20260911T115500_be4567ef", "ccbobs_rejected_", RefFamilyEvidenceSchemeURI, "", RefSlotSnapshot, "obs_20260911T115500_be4567ef"},
		{"ccbobs_rejected_unbound", "ccbobs_rejected_", RefFamilyEvidenceSchemeURI, "", RefSlotSnapshot, "unbound"},
		{"ccbobs_batch_obs_20260911T115510_c78901ab", "ccbobs_batch_", RefFamilyEvidenceSchemeURI, "", RefSlotSnapshot, "obs_20260911T115510_c78901ab"},
		{"cap_pack_0123456789abcdef", "cap_pack_", RefFamilyEvidenceSchemeURI, "", RefSlotSnapshot, "0123456789abcdef"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseRef(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.State != RefStateLegacy {
				t.Fatalf("state = %q, want legacy", got.State)
			}
			if got.Legacy == nil {
				t.Fatalf("legacy translation missing")
			}
			l := got.Legacy
			if l.LegacyPrefix != tc.prefix || l.Family != tc.family || l.TargetKind != tc.kind || l.Slot != tc.slot || l.Value != tc.value {
				t.Errorf("translation = %+v, want prefix=%q family=%q kind=%q slot=%q value=%q", l, tc.prefix, tc.family, tc.kind, tc.slot, tc.value)
			}
			// 往返恒等：分解必须无损（旧记录 legacy refs 可解析的 M4 承载面）。
			if l.LegacyPrefix+l.Value != tc.raw {
				t.Errorf("round-trip broken: prefix+value = %q, raw = %q", l.LegacyPrefix+l.Value, tc.raw)
			}
			again, err := ParseRef(tc.raw)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Errorf("re-parse not idempotent: %+v vs %+v (err=%v)", got, again, err)
			}
		})
	}
}

// TestRefSchemaM4LongestMatchPrefixContainment 动态扫描注册表全部前缀包含对
// （M4 新增 ccbobs_/ccbobs_rejected_ 与 ccbobs_/ccbobs_batch_ 是本表首批），
// 断言命中取最长前缀、与注册序无关（主条目先于变体登记）。
func TestRefSchemaM4LongestMatchPrefixContainment(t *testing.T) {
	registry := LegacyPrefixRegistry()
	pairs := 0
	for _, short := range registry {
		for _, long := range registry {
			if short.LegacyPrefix == long.LegacyPrefix || !strings.HasPrefix(long.LegacyPrefix, short.LegacyPrefix) {
				continue
			}
			pairs++
			sample := long.LegacyPrefix + "0123456789abcdef"
			got, err := ParseRef(sample)
			if err != nil {
				t.Fatalf("parse %q: %v", sample, err)
			}
			if got.State != RefStateLegacy || got.Legacy == nil {
				t.Fatalf("parse %q: state=%v, want legacy translation", sample, got.State)
			}
			if got.Legacy.LegacyPrefix != long.LegacyPrefix {
				t.Errorf("containment pair %q<%q: matched %q, want longest %q", short.LegacyPrefix, long.LegacyPrefix, got.Legacy.LegacyPrefix, long.LegacyPrefix)
			}
			if got.Legacy.Value != "0123456789abcdef" {
				t.Errorf("containment pair %q<%q: value = %q, want clean remainder without infix", short.LegacyPrefix, long.LegacyPrefix, got.Legacy.Value)
			}
		}
	}
	if pairs == 0 {
		t.Fatalf("no containment pairs found — ccbobs_ 三形态词条丢失")
	}
}

// TestRefSchemaM4CCBRFamilyStaysUnregistered 钉住停止条件上交的现状：
// ccbr_/ccbr_rejected_/ccbr_batch_ 缓注册，保持 opaque WARN-once 透传。若未来
// 重裁后注册该族，本测试必须随裁定同步更新（fail-visible，不许静默改语义）。
func TestRefSchemaM4CCBRFamilyStaysUnregistered(t *testing.T) {
	for _, entry := range LegacyPrefixRegistry() {
		if strings.HasPrefix(entry.LegacyPrefix, "ccbr") {
			t.Fatalf("ccbr_ 族出现注册词条 %q —— 与 REFSCHEMA-M4 停止条件上交记录冲突（coord/runs/REFSCHEMA-M4/receipt.md），须先经 G1 重裁并同步本测试", entry.LegacyPrefix)
		}
	}
	ResetOpaqueWarnState()
	samples := []string{
		"ccbr_obs_20260911T115419_a2ef6022376c",
		"ccbr_rejected_obs_20260911T115500_be4567ef",
		"ccbr_batch_obs_20260911T115510_c78901ab",
	}
	for _, raw := range samples {
		got, err := ParseRef(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if got.State != RefStateOpaque {
			t.Errorf("parse %q: state = %q, want opaque (缓注册)", raw, got.State)
		}
		if got.Legacy != nil {
			t.Errorf("parse %q: unexpected legacy translation %+v", raw, got.Legacy)
		}
	}
	// 无冒号无斜杠的字面量，opaque 计数键=整串（scheme head 兜底分支）。
	counts := OpaqueWarnCounts()
	if len(counts) != len(samples) {
		t.Fatalf("opaque counts = %v, want exactly %d keys", counts, len(samples))
	}
	for _, raw := range samples {
		if counts[raw] != 1 {
			t.Errorf("count[%q] = %d, want 1", raw, counts[raw])
		}
	}
	ResetOpaqueWarnState()
}
