# PORT-HARNESS-FIX-1：rackPluginSemanticEntry 内核兜底移除（被拒命令零内核副作用）

- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（FLAKY-1 取证转修复卡）
- 模型分级：L2 / GLM-5.3（改动小但涉 PCA 门控邻近语义，谨慎执行）
- 决策定案（方案 A·生产根治，FLAKY-1 回执三案择一，决策侧 2026-09-17 裁定）：删除 `rackPluginSemanticEntry` 的内核兜底（`agent/internal/harness/harness.go:8996-9012` 索引缺失时 `listAvailablePlugins`→`scan_plugins` 路径），索引未命中走既有默认 Z3 分支（harness.go:8969-8971）。**接受 tradeoff**：无索引机器上乐器 zone（Z2）不再经内核清单解析、默认落 Z3——zone 解析的正路是校准（C2/C3）或显式装载流建立索引，不是规范化阶段的内核副作用
- 目标：恢复「被 PCA 门控拒绝的命令在规范化阶段零内核副作用」不变量；FLAKY-1 两测试（`TestFullProjectAccessDoesNotDisplaceExplicitSelectionAuthorization` / `TestAgentProcessorLoadGateRechecksPCAAndBlocksBypass`）在**无任何夹具的干净机器**上通过
- 文件域：`agent/internal/harness/harness.go`（仅该兜底块删除及其必要收尾）；**不改动** `pluginsemantics/index.go` 的模糊兜底（另行决策）；不改两测试文件（方案 A 下应原样通过）
- 验收标准：① 干净机器（无 `~/.vit/plugin_semantics.json`、不设 `VIT_PLUGIN_SEMANTICS_PATH`）上述两测试 `-count=1` PASS；② `go test ./internal/harness -count=1` 整包**零失败**（FLAKY-1 两例消失、无新增）；③ `go build ./...` 与 `go vet ./internal/harness/` exit 0；④ 失败输出与命令入回执
- 停止条件：删除兜底引发其它测试依赖该路径的失败且非夹具可解 → 停下上报（不许为凑通过改测试语义）；发现 zone 解析在别处依赖同一兜底 → 列锚点上交
- 领取：
- 回执：
- 验收：
- 关联：取证见 `coord/cards/done/2026-09-16-PORT-MAC-HARNESS-FLAKY-1.md`（证据矩阵与三变体对照）；`Get()` 模糊兜底跨插件错配隐患（index.go:253）**不在本卡**，登记为独立待裁项
