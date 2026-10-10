package agentprotocol

// refschema_m5_test.go — REFSCHEMA-M5（2026-10-10）E 类回执族注册词条的解析面
// 测试：b4/c1 semantic_eq_batch 主/reconcile 四词条 + c2.dynamic_plugin_load_batch
// 的翻译五元组、往返恒等、尾冒号前缀与 .governed 命令常量及主/变体词条互不
// 劫持、同端口兄弟族 plugin_load_batch{,.reconcile}: 维持 opaque 的 fail-visible
// 钉住（M4 对 ccbr_ 缓注册钉住的先例，随域触碰翻面）。承载裁定依据（identity
// 族 slot=snapshot、TargetKind 留空）与消费链清单见 coord/runs/REFSCHEMA-M5/receipt.md。

import (
	"reflect"
	"testing"
)

// TestRefSchemaM5ReceiptFamilyRegistered 钉住五个新词条的注册表形态（含 M5
// 锚点）。b4/c1 四词条同构（共享 projectEQBatchMutationPort，SourcePrefix 分
// 族），c2 单词条。
func TestRefSchemaM5ReceiptFamilyRegistered(t *testing.T) {
	byPrefix := map[string]LegacyPrefixEntry{}
	for _, entry := range LegacyPrefixRegistry() {
		byPrefix[entry.LegacyPrefix] = entry
	}
	wantEntries := []LegacyPrefixEntry{
		{LegacyPrefix: "b4.semantic_eq_batch:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "REFSCHEMA-M5 2026-10-10 chat/b4_eq_runtime.go:618"},
		{LegacyPrefix: "b4.semantic_eq_batch.reconcile:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "REFSCHEMA-M5 2026-10-10 chat/b4_eq_runtime.go:525"},
		{LegacyPrefix: "c1.semantic_eq_batch:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "REFSCHEMA-M5 2026-10-10 chat/b4_eq_runtime.go:618 c1 端口同源"},
		{LegacyPrefix: "c1.semantic_eq_batch.reconcile:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "REFSCHEMA-M5 2026-10-10 chat/b4_eq_runtime.go:525 c1 端口同源"},
		{LegacyPrefix: "c2.dynamic_plugin_load_batch:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "REFSCHEMA-M5 2026-10-10 chat/c2_dynamic_batch.go:686"},
	}
	for _, want := range wantEntries {
		got, ok := byPrefix[want.LegacyPrefix]
		if !ok {
			t.Fatalf("prefix %q missing from registry —— M5 E 类回执族词条丢失", want.LegacyPrefix)
		}
		if got != want {
			t.Errorf("entry for %q = %+v, want %+v (M4 批次同款：family=evidence_scheme_uri kind=\"\" slot=snapshot + M5 锚点)", want.LegacyPrefix, got, want)
		}
	}
}

// TestRefSchemaM5ReceiptTranslationTable 钉住五个词条的翻译五元组与往返恒等。
// 样本按生成点实录形态拼接：remainder=IdempotencyKey（executionruntime/
// coordinator.go:115/:130）="exec:"+sessionID("plan_"+随机量)+":"+actionSetHash
// (sha256 64hex)+":"+actionID。
func TestRefSchemaM5ReceiptTranslationTable(t *testing.T) {
	const setHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cases := []struct {
		raw    string
		prefix string
		value  string
	}{
		{"b4.semantic_eq_batch:exec:plan_a1b2c3d4e5f60718:" + setHash + ":b4_semantic_eq_batch", "b4.semantic_eq_batch:", "exec:plan_a1b2c3d4e5f60718:" + setHash + ":b4_semantic_eq_batch"},
		{"b4.semantic_eq_batch.reconcile:exec:plan_a1b2c3d4e5f60718:" + setHash + ":b4_semantic_eq_batch", "b4.semantic_eq_batch.reconcile:", "exec:plan_a1b2c3d4e5f60718:" + setHash + ":b4_semantic_eq_batch"},
		{"c1.semantic_eq_batch:exec:plan_a1b2c3d4e5f60718:" + setHash + ":c1_semantic_eq_batch", "c1.semantic_eq_batch:", "exec:plan_a1b2c3d4e5f60718:" + setHash + ":c1_semantic_eq_batch"},
		{"c1.semantic_eq_batch.reconcile:exec:plan_a1b2c3d4e5f60718:" + setHash + ":c1_semantic_eq_batch", "c1.semantic_eq_batch.reconcile:", "exec:plan_a1b2c3d4e5f60718:" + setHash + ":c1_semantic_eq_batch"},
		{"c2.dynamic_plugin_load_batch:exec:plan_a1b2c3d4e5f60718:" + setHash + ":c2_dynamic_plugin_load_batch", "c2.dynamic_plugin_load_batch:", "exec:plan_a1b2c3d4e5f60718:" + setHash + ":c2_dynamic_plugin_load_batch"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseRef(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.State != RefStateLegacy {
				t.Fatalf("state = %q, want legacy (M5 已注册)", got.State)
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
			again, err := ParseRef(tc.raw)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Errorf("re-parse not idempotent: %+v vs %+v (err=%v)", got, again, err)
			}
		})
	}
}

// TestRefSchemaM5NoCrossHijack 钉住尾冒号前缀的互不劫持面：主/reconcile 词条
// 各归其主（':' 与 '.' 分隔，注册表内无新增包含对）；.governed 命令常量（命
// 令名非 ref）不被回执词条吞入 legacy 态。
func TestRefSchemaM5NoCrossHijack(t *testing.T) {
	const setHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	pairs := []struct {
		raw, wantPrefix string
	}{
		{"b4.semantic_eq_batch:exec:plan_x:" + setHash + ":a", "b4.semantic_eq_batch:"},
		{"b4.semantic_eq_batch.reconcile:exec:plan_x:" + setHash + ":a", "b4.semantic_eq_batch.reconcile:"},
		{"c1.semantic_eq_batch:exec:plan_x:" + setHash + ":a", "c1.semantic_eq_batch:"},
		{"c1.semantic_eq_batch.reconcile:exec:plan_x:" + setHash + ":a", "c1.semantic_eq_batch.reconcile:"},
	}
	for _, tc := range pairs {
		got, err := ParseRef(tc.raw)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.raw, err)
		}
		if got.State != RefStateLegacy || got.Legacy == nil || got.Legacy.LegacyPrefix != tc.wantPrefix {
			t.Errorf("parse %q: got (%v, %+v), want legacy prefix %q —— 主/reconcile 词条互不劫持被破坏", tc.raw, got.State, got.Legacy, tc.wantPrefix)
		}
	}
	// 命令常量（b4/c1/c2 三族 .governed 形态）不是 ref：不得被回执词条匹配成
	// legacy，维持 opaque 原样透传。
	for _, command := range []string{"b4.semantic_eq_batch.governed", "c1.semantic_eq_batch.governed", "b4.eq_plugin_load_batch.governed", "c2.dynamic_plugin_load_batch.governed"} {
		got, err := ParseRef(command)
		if err != nil {
			t.Fatalf("parse command %q: %v", command, err)
		}
		if got.State == RefStateLegacy {
			t.Errorf("command %q parsed as legacy prefix %q —— 命令名被回执词条劫持", command, got.Legacy.LegacyPrefix)
		}
	}
}

// TestRefSchemaM5LoadBatchSiblingsStayUnregistered fail-visible 钉住：同端口
// 兄弟族 plugin_load_batch{,.reconcile}:（b4_eq_runtime.go:678/:564，b4/c1 两
// 前缀）不在 G1 M5 行点名两族内，维持 opaque 登记余量。随域触碰注册时翻面本
// 测试（对齐 M4 对 ccbr_ 缓注册钉住→M4B 翻面的先例）。
func TestRefSchemaM5LoadBatchSiblingsStayUnregistered(t *testing.T) {
	const setHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, raw := range []string{
		"b4.plugin_load_batch:exec:plan_x:" + setHash + ":a",
		"b4.plugin_load_batch.reconcile:exec:plan_x:" + setHash + ":a",
		"c1.plugin_load_batch:exec:plan_x:" + setHash + ":a",
		"c1.plugin_load_batch.reconcile:exec:plan_x:" + setHash + ":a",
	} {
		got, err := ParseRef(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if got.State == RefStateLegacy {
			t.Errorf("sibling %q parsed as legacy prefix %q —— 余量族被擅自注册，须先过 G1 行裁决再翻面本测试", raw, got.Legacy.LegacyPrefix)
		}
	}
}
