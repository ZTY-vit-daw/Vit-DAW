package agentprotocol

// refschema_m4_test.go — REFSCHEMA-M4（2026-10-10）capabilitycontext 批次注册
// 词条的解析面测试：ccbobs_ 三形态 + cap_pack_ 翻译表、注册表首组前缀包含对
// 的最长匹配行为、legacy 分解往返恒等。M4B（2026-10-10）追加了 ccbr_ 族三前缀
// 的重裁落地翻面测试（原 opaque 缓注册钉住翻转为注册+翻译形态断言）。
// 证据与承载裁定见 coord/runs/REFSCHEMA-M4/receipt.md 与
// coord/runs/REFSCHEMA-M4B/receipt.md。

import (
	"reflect"
	"strings"
	"testing"
)

// TestRefSchemaM4LegacyTranslationTable 钉住四个新词条的翻译五元组。样本取
// webui trace fixture 实录形态（ccbr_ 族三前缀由 M4B 翻面测试覆盖）。
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
// （M4 新增 ccbobs_/ccbobs_rejected_ 与 ccbobs_/ccbobs_batch_ 是本表首批；
// M4B 追加 ccbr_/ccbr_rejected_ 与 ccbr_/ccbr_batch_ 两对，同机制覆盖），
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

// TestRefSchemaM4BCCBRFamilyRegistered 翻面测试（原 TestRefSchemaM4CCBRFamilyStaysUnregistered，
// REFSCHEMA-M4B 2026-10-10 落地）：按承载重裁（coord/rulings/2026-10-10-MORNING-BATCH-rulings.md
// §3）ccbr_ 族三前缀注册进注册表——compactID=首个非空输入清洗透传（RequestID
// 在场仍取 observationID，capabilitycontext 锚点测试复证），M8 报告 D4「哈希
// 短串/内容寻址」描述失准，与 ccbobs_ 族同构 → identity 族承载 slot=snapshot、
// TargetKind 留空。断言：注册表三词条形态（含 M4B 锚点）、翻译五元组与往返
// 恒等、族内最长匹配（主条目 ccbr_ 不劫持 rejected/batch 变体）、与 ccbobs_
// 族互不劫持。
func TestRefSchemaM4BCCBRFamilyRegistered(t *testing.T) {
	byPrefix := map[string]LegacyPrefixEntry{}
	for _, entry := range LegacyPrefixRegistry() {
		byPrefix[entry.LegacyPrefix] = entry
	}
	wantEntries := []LegacyPrefixEntry{
		{LegacyPrefix: "ccbr_", Family: RefFamilyEvidenceSchemeURI, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "REFSCHEMA-M4B 2026-10-10 capabilitycontext/free_state_observation.go:567"},
		{LegacyPrefix: "ccbr_rejected_", Family: RefFamilyEvidenceSchemeURI, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "REFSCHEMA-M4B 2026-10-10 capabilitycontext/free_state_observation.go:439"},
		{LegacyPrefix: "ccbr_batch_", Family: RefFamilyEvidenceSchemeURI, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "REFSCHEMA-M4B 2026-10-10 capabilitycontext/free_state_observation.go:1636"},
	}
	for _, want := range wantEntries {
		got, ok := byPrefix[want.LegacyPrefix]
		if !ok {
			t.Fatalf("prefix %q missing from registry —— M4B 重裁落地词条丢失", want.LegacyPrefix)
		}
		if got != want {
			t.Errorf("entry for %q = %+v, want %+v (ccbobs_ 同款：family=evidence_scheme_uri kind=\"\" slot=snapshot + M4B 锚点)", want.LegacyPrefix, got, want)
		}
	}
	// 翻译形态：样本取 webui trace fixture 实录（ccbr_obs_<UTC 时序戳>_<随机量>）
	// 与 compactID 空输入兜底（unbound，与 ccbobs_rejected_unbound 同款）。
	cases := []struct {
		raw    string
		prefix string
		value  string
	}{
		{"ccbr_obs_20260911T115419_a2ef6022376c", "ccbr_", "obs_20260911T115419_a2ef6022376c"},
		{"ccbr_rejected_obs_20260911T115500_be4567ef", "ccbr_rejected_", "obs_20260911T115500_be4567ef"},
		{"ccbr_batch_obs_20260911T115510_c78901ab", "ccbr_batch_", "obs_20260911T115510_c78901ab"},
		{"ccbr_rejected_unbound", "ccbr_rejected_", "unbound"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseRef(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.State != RefStateLegacy {
				t.Fatalf("state = %q, want legacy (M4B 已注册)", got.State)
			}
			if got.Legacy == nil {
				t.Fatalf("legacy translation missing")
			}
			l := got.Legacy
			if l.LegacyPrefix != tc.prefix || l.Family != RefFamilyEvidenceSchemeURI || l.TargetKind != "" || l.Slot != RefSlotSnapshot || l.Value != tc.value {
				t.Errorf("translation = %+v, want prefix=%q family=evidence_scheme_uri kind=\"\" slot=snapshot value=%q", l, tc.prefix, tc.value)
			}
			// 往返恒等：Value 不带中缀，prefix+value 无损还原原字面量。
			if l.LegacyPrefix+l.Value != tc.raw {
				t.Errorf("round-trip broken: prefix+value = %q, raw = %q", l.LegacyPrefix+l.Value, tc.raw)
			}
		})
	}
	// 与 ccbobs_ 族互不劫持：族内最长匹配（主条目不吞变体）+ 跨族各归其主。
	pairs := []struct {
		raw, wantPrefix string
	}{
		{"ccbr_rejected_obs_20260911T115500_be4567ef", "ccbr_rejected_"},
		{"ccbr_batch_obs_20260911T115510_c78901ab", "ccbr_batch_"},
		{"ccbobs_obs_20260911T115419_a2ef6022376c", "ccbobs_"},
		{"ccbobs_rejected_obs_20260911T115500_be4567ef", "ccbobs_rejected_"},
		{"ccbobs_batch_obs_20260911T115510_c78901ab", "ccbobs_batch_"},
	}
	for _, tc := range pairs {
		got, err := ParseRef(tc.raw)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.raw, err)
		}
		if got.State != RefStateLegacy || got.Legacy == nil || got.Legacy.LegacyPrefix != tc.wantPrefix {
			t.Errorf("parse %q: got (%v, %+v), want legacy prefix %q —— 族内最长匹配/跨族互不劫持被破坏", tc.raw, got.State, got.Legacy, tc.wantPrefix)
		}
	}
}
