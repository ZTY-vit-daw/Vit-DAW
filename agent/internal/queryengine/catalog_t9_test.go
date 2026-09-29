package queryengine

// catalog_t9_test.go — T9 Catalog 一致性（QUERY_ENGINE §6.3，IMPL-B）：
//
//	1. PayloadIndexRegistry 声明字段与 Engine.Catalog 输出一致——Catalog 的
//	  PayloadFields 恰为路由表建议字段 ∪ 已注册局部索引字段（注册面变化
//	  立即反映，不漂移）；
//	2. ref.query 的 schema description 与 Catalog 同源生成——
//	  BuildRefQuerySchemaDescription 是 Catalog 的纯函数：目录里每个 kind、
//	  每个可过滤/可排序字段、排序别名都出现；目录变则 description 变
//	  （双写漂移在编译面不可能——IMPL-C 工具面只许消费本函数，不得手写）。

import (
	"strings"
	"testing"
)

func TestT9RegistryFieldsMatchCatalog(t *testing.T) {
	empty := NewEngine(newFakeStore(), nil)
	base := empty.Catalog()
	baseFields := catalogFieldIndex(base)

	// 无注册索引：Catalog=路由表建议字段的逐 kind 精确快照（与 KindRouteTable 同源）。
	for _, entry := range KindRouteTable() {
		got, ok := baseFields[entry.Kind]
		if !ok {
			t.Fatalf("Catalog 缺 kind %q（路由表有条目）", entry.Kind)
		}
		want := map[string]PayloadFieldSpec{}
		for _, f := range entry.PayloadFields {
			want[f.Field] = f
		}
		if len(got) != len(want) {
			t.Fatalf("kind %q：Catalog 字段数 %d ≠ 路由表 %d", entry.Kind, len(got), len(want))
		}
		for field, spec := range want {
			c, ok := got[field]
			if !ok {
				t.Fatalf("kind %q 缺路由表字段 %q", entry.Kind, field)
			}
			if c.Type != spec.Type || c.Sortable != spec.Sortable {
				t.Fatalf("kind %q 字段 %q 形态漂移：catalog %+v ≠ 路由表 %+v", entry.Kind, field, c, spec)
			}
		}
	}

	// 注册局部索引后：Catalog=建议字段 ∪ 索引声明字段（注册面立即反映）。
	rows := []MaterializedRow{
		mrow(allRef("dom", "track", "T1", "current", 1), FreshnessCurrent, nil),
		mrow(allRef("fxm", "track", "T1", "current", 1), FreshnessCurrent, nil),
	}
	engine := NewEngine(newFakeStore(rows...), NewPayloadIndexRegistry(
		newFakePayloadIndex("dom", rows),
		newFakePayloadIndex("fxm", rows),
	))
	extended := engine.Catalog()
	extFields := catalogFieldIndex(extended)
	for _, kind := range []string{"dom", "fxm"} {
		for _, suffix := range []string{"demo.number", "demo.text"} {
			if _, ok := extFields[kind][kind+"."+suffix]; !ok {
				t.Fatalf("注册索引后 Catalog 缺 kind %q 的索引声明字段 %q", kind, kind+"."+suffix)
			}
		}
	}
	// 未注册 kind 不受影响（mom 无索引——字段集与路由表快照一致）。
	if _, ok := extFields["mom"]["mom.demo.number"]; ok {
		t.Fatal("mom 无注册索引，不得出现索引声明字段")
	}
}

func TestT9SchemaDescriptionSameSource(t *testing.T) {
	rows := []MaterializedRow{mrow(allRef("dom", "track", "T1", "current", 1), FreshnessCurrent, nil)}
	engine := NewEngine(newFakeStore(rows...), NewPayloadIndexRegistry(newFakePayloadIndex("dom", rows)))
	catalog := engine.Catalog()
	desc := BuildRefQuerySchemaDescription(catalog)
	if strings.TrimSpace(desc) == "" {
		t.Fatal("description 不得为空")
	}
	// 每个 kind 与其全部声明字段、排序别名都必须出现在 description 中。
	for _, kind := range catalog.Kinds {
		if !strings.Contains(desc, kind.Kind) {
			t.Fatalf("description 缺 kind %q", kind.Kind)
		}
		for _, f := range kind.PayloadFields {
			if !strings.Contains(desc, f.Field) {
				t.Fatalf("description 缺字段 %q（kind %q）", f.Field, kind.Kind)
			}
		}
	}
	for _, alias := range catalog.SortAliases {
		if !strings.Contains(desc, alias) {
			t.Fatalf("description 缺排序别名 %q", alias)
		}
	}

	// 同源性的反证腿：目录变化（注册新索引字段）→ description 必随之变化。
	plain := NewEngine(newFakeStore(rows...), nil)
	plainDesc := BuildRefQuerySchemaDescription(plain.Catalog())
	if plainDesc == desc {
		t.Fatal("注册索引前后 description 不得相同（Catalog→description 须为纯函数直射，无第二事实源）")
	}
	if !strings.Contains(plainDesc, "dom.peak_structure.readiness") {
		t.Fatal("无索引目录的 description 仍须含路由表建议字段")
	}
}

func catalogFieldIndex(c EngineCatalog) map[string]map[string]PayloadFieldSpec {
	out := map[string]map[string]PayloadFieldSpec{}
	for _, kind := range c.Kinds {
		fields := map[string]PayloadFieldSpec{}
		for _, f := range kind.PayloadFields {
			fields[f.Field] = f
		}
		out[kind.Kind] = fields
	}
	return out
}
