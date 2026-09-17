# 裁定：PORT-HARNESS-FIX-1 pass（2026-09-17，决策侧，rework 后终验）

- 对象：`port/harness-fix-1`（实现 `e4dc881` 兜底删除 + rework `c7e901c` 规格测试改写）
- rework 对照裁定逐条达成：仅 :1925-1961 一例改写并更名（`DefaultsToZ3WithoutSemanticIndex`）；fakeKernel 零应答哨兵 + `kernel.commands` 为空断言（"want zero kernel side effects during normalization"）；其余测试零触碰（diff 核实 +6/−18 单文件）
- 决策侧独立复验（隔离 worktree @c7e901c，干净机器：无 `~/.vit/plugin_semantics.json`、未设 `VIT_PLUGIN_SEMANTICS_PATH`）：改写测试 PASS；FLAKY-1 两例 PASS；**`go test ./internal/harness -count=1` 整包 ok 零失败**（该包 mac 上自开荒以来首次全绿，FLAKY-1 慢性失败根除）；`go build ./...` + `go vet ./internal/harness/` 双 exit 0
- 处置：`e4dc881` + `c7e901c` 依次 cherry-pick 入 main；「被拒命令零内核副作用」不变量自此固化于测试（哨兵断言）
- 遗留登记：`Get()` 模糊兜底跨插件错配隐患（pluginsemantics/index.go:253）仍为独立待裁项；真栈 zone 解析行为验证并入 C3（先前裁定不变）
