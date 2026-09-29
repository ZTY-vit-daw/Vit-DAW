package queryengine

// catalog_schema.go — ref.query 工具面 schema description 的同源生成器
// （QUERY_ENGINE §6.3 T9，IMPL-B）。
//
// 单一事实源：description 是 EngineCatalog 的纯函数。IMPL-C 工具面把本函数
// 产物编译进 ref.query 的 schema description——不得手写第二份目录文案（双写
// 漂移在编译面不可能）；路由表/注册索引变化自动带进工具面。模型在发 query
// 前就能看到可查字段面（grep 前先有 --help）。

import (
	"fmt"
	"strings"
)

// BuildRefQuerySchemaDescription 从引擎目录生成 ref.query 的 schema
// description 文案。确定性输出（目录序即路由表序）；空目录产出说明性占位。
func BuildRefQuerySchemaDescription(c EngineCatalog) string {
	var b strings.Builder
	b.WriteString("Ad-hoc read-only ref query (audio-grep) over materialized observations. ")
	b.WriteString("Returns refs plus declared payload scalars; never triggers computation. ")
	b.WriteString("Registered kinds and their filterable/sortable payload fields:")
	if len(c.Kinds) == 0 {
		b.WriteString(" (none registered)\n")
	} else {
		b.WriteString("\n")
	}
	for _, kind := range c.Kinds {
		b.WriteString("- ")
		b.WriteString(kind.Kind)
		if kind.LegacyPrefix != "" {
			fmt.Fprintf(&b, " (legacy prefix %q)", kind.LegacyPrefix)
		}
		b.WriteString("\n")
		if len(kind.PayloadFields) == 0 {
			b.WriteString("    (no payload fields declared)\n")
			continue
		}
		for _, f := range kind.PayloadFields {
			fmt.Fprintf(&b, "    %s: type=%s, ops=[%s]", f.Field, f.Type, opsList(f.Ops))
			if f.Sortable {
				b.WriteString(", sortable")
			}
			b.WriteString("\n")
		}
	}
	if len(c.SortAliases) > 0 {
		fmt.Fprintf(&b, "Ref-segment sort aliases: %s\n", strings.Join(c.SortAliases, ", "))
	}
	return b.String()
}

func opsList(ops []PayloadOp) string {
	parts := make([]string, 0, len(ops))
	for _, op := range ops {
		parts = append(parts, string(op))
	}
	return strings.Join(parts, ",")
}
