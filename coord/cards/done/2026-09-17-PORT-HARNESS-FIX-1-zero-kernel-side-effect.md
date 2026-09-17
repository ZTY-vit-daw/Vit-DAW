# PORT-HARNESS-FIX-1：rackPluginSemanticEntry 内核兜底移除（被拒命令零内核副作用）

- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（FLAKY-1 取证转修复卡）
- 模型分级：L2 / GLM-5.3（改动小但涉 PCA 门控邻近语义，谨慎执行）
- 决策定案（方案 A·生产根治，FLAKY-1 回执三案择一，决策侧 2026-09-17 裁定）：删除 `rackPluginSemanticEntry` 的内核兜底（`agent/internal/harness/harness.go:8996-9012` 索引缺失时 `listAvailablePlugins`→`scan_plugins` 路径），索引未命中走既有默认 Z3 分支（harness.go:8969-8971）。**接受 tradeoff**：无索引机器上乐器 zone（Z2）不再经内核清单解析、默认落 Z3——zone 解析的正路是校准（C2/C3）或显式装载流建立索引，不是规范化阶段的内核副作用
- 目标：恢复「被 PCA 门控拒绝的命令在规范化阶段零内核副作用」不变量；FLAKY-1 两测试（`TestFullProjectAccessDoesNotDisplaceExplicitSelectionAuthorization` / `TestAgentProcessorLoadGateRechecksPCAAndBlocksBypass`）在**无任何夹具的干净机器**上通过
- 文件域：`agent/internal/harness/harness.go`（仅该兜底块删除及其必要收尾）；**不改动** `pluginsemantics/index.go` 的模糊兜底（另行决策）；~~不改两测试文件~~ → **rework 扩域（裁定 [2026-09-17-HARNESSFIX1-rework.md](../../rulings/2026-09-17-HARNESSFIX1-rework.md)）**：`harness_test.go` **仅限 :1925-1961 一例**按方案 A 新语义改写规格断言并更名（索引缺失→默认 Z3 + `kernel.commands` 为空；scan 应答桩改"不应被调用"哨兵或移除），其余测试不触碰
- 验收标准：① 干净机器（无 `~/.vit/plugin_semantics.json`、不设 `VIT_PLUGIN_SEMANTICS_PATH`）上述两测试 `-count=1` PASS；② `go test ./internal/harness -count=1` 整包**零失败**（FLAKY-1 两例消失、改写后规格测试 PASS、无新增）；③ `go build ./...` 与 `go vet ./internal/harness/` exit 0；④ 失败输出与命令入回执；⑤ 改写后测试固化新语义（Z3 + 零内核命令）
- 返工意见（2026-09-17 决策侧）：blocked 处置=维持方案 A + 扩域改写不删（决策侧已独立复验红绿对照与整包恰 1 例）；同分支 `port/harness-fix-1` 续作；端测边界裁定：单元级验收足够（真栈 zone 解析验证并入 C3）
- 停止条件：删除兜底引发其它测试依赖该路径的失败且非夹具可解 → 停下上报（不许为凑通过改测试语义）；发现 zone 解析在别处依赖同一兜底 → 列锚点上交
- 领取：2026-09-17（Mac 执行侧，用户口令「开工 HARNESS-FIX-1」）/ origin/main=04fa36d49d7c4866520b3990ddd9c0a863bca630（工作树干净，无并行流，主工作树）/ 分支 port/harness-fix-1
- 续作领取（rework，2026-09-17）：用户口令「开工 HARNESS-FIX-1」/ origin/main=b1117c5（工作树干净，无并行流，主工作树）/ 同分支 port/harness-fix-1（基于已推实现 e4dc881），按裁定 [2026-09-17-HARNESSFIX1-rework.md](../../rulings/2026-09-17-HARNESSFIX1-rework.md) 扩域改写 harness_test.go:1925-1961 一例
- 回执：**blocked（2026-09-17，Mac 执行侧）——方案 A 实现完成且不变量恢复（FLAKY-1 两例干净机器 PASS、build/vet exit 0），但整包验收②出现 1 例新增失败：`TestPluginLoadToRackInstrumentDefaultsToZ2FromScannedPluginInventory`（harness_test.go:1925-1961）恰是被删兜底的功能规格测试，非夹具可解，命中停止条件上交。实现 commit e4dc881（分支 port/harness-fix-1，已推）待决策扩域处置。**

  ## 验收对照（本机 darwin/arm64 干净机器：无 `~/.vit/plugin_semantics.json`、`VIT_PLUGIN_SEMANTICS_PATH` 未设）

  | 验收项 | 结果 |
  |---|---|
  | ① 两测试 `-count=1` | **PASS**（exit 0；改动前同机 FAIL，红→绿证据齐） |
  | ② 整包 `go test ./internal/harness -count=1` | **FAIL，有且仅有 1 例新增失败**（见下）；FLAKY-1 两例消失 |
  | ③ `go build ./...` / `go vet ./internal/harness/` | 双 exit 0 |
  | ④ 原始输出 | `Export/harness-fix-1-20260917/`（baseline-two-tests / baseline-scanned-inventory / post-two-tests / post-scanned-inventory / post-full-package / go-build / go-vet .txt；Export/ gitignored，可 SSH 只读核查） |

  ## 阻塞点（停止条件「其它测试依赖该路径且非夹具可解」+「zone 解析在别处依赖同一兜底」同时命中）

  - `harness_test.go:1925` 该测试**功能规格化被删除的内核兜底**：故意把 `VIT_PLUGIN_SEMANTICS_PATH` 指向缺失文件（:1926）隔离出兜底路径，fakeKernel 以含 Surge XT（is_instrument）的清单应答 scan_plugins，断言 `zone_id=="Z2"`（:1955）且 `kernel.commands` 恰为 `[scan_plugins]`（:1958-1960）。方案 A 裁定语义（无索引→默认 Z3）与之直接矛盾，必然失败。
  - 基线对照：04fa36d 改动前经独立 git worktree 复跑该测试 **PASS**（baseline-scanned-inventory.txt）——失败系删除新增，非既有失败。
  - 非夹具可解：测试自身已用 `t.Setenv` 隔离索引；机器级 `~/.vit` 夹具会破坏验收①的干净机器前提、违反 AGENTS §10（且正是 FLAKY-1 诊断出的机器状态依赖缺陷本身）；改断言/删测试=改测试语义且 harness_test.go 在本卡文件域外。
  - 其它 scan_plugins 测试不受影响：`TestPluginSemanticSearchMergesLivePluginSearch`（:1963）/ `TestPluginSemanticBuildIndexUsesScannedPluginInventory`（:2007）走**显式工具调用**路径（searchPluginSemanticIndex / buildPluginSemanticIndex），非规范化阶段副作用，仍 PASS——与不变量恢复语义一致。
  - 实现细节：兜底块删除 + `rackPluginSemanticEntry` 改纯函数（去死参 h/ctx，仅本地索引查找）+ `resolveRackAddNodeZone` 去 ctx（调用点 :8793 同步），净 +5/−21 行，全部在 harness.go 文件域内；无 import 变化（time/pluginsemantics 均有其它使用点）。

  ## 上交决策项

  1. **扩域删/改 harness_test.go:1925**（执行侧建议：删——兄弟例 :1844 `FromSemanticIndex` 已覆盖索引在场的 Z2 解析；或改断言 Z3 固化新语义）→ 授权后本分支续作，整包即零失败；
  2. 或推翻方案 A 改走方案 B（两测试夹具隔离，固化现状语义）→ 本分支作废。

  - 端测边界声明：本卡验收口径为单元级（①②③④），未含真实栈烟测；改动限于命令规范化阶段 zone 解析路径，HTTP/ZMQ 接口面无变化，是否补真栈烟测由决策侧裁定。
- 回执（rework 续作，2026-09-17，Mac 执行侧）：**done——按裁定扩域改写 harness_test.go:1925-1961 一例并整包零失败达成**。更名 `TestPluginLoadToRackInstrumentDefaultsToZ2FromScannedPluginInventory` → `TestPluginLoadToRackInstrumentDefaultsToZ3WithoutSemanticIndex`：索引缺失 → 默认 Z3；scan 应答桩移除（fakeKernel 零应答即"不应被调用"哨兵，`SendCommand` 落 commands 记录，断言 `kernel.commands` 为空）；净 +6/−18 仅该一例，其余测试未触碰。分支 `port/harness-fix-1` 新增 commit **c7e901c**（已推，基于实现 e4dc881）。

  ## 验收对照（rework 轮，本机 darwin/arm64 干净机器：无 `~/.vit/plugin_semantics.json`、`VIT_PLUGIN_SEMANTICS_PATH` 未设）

  | 验收项 | 结果 |
  |---|---|
  | ① FLAKY-1 两测试 `-count=1` | **PASS**（exit 0） |
  | ② `go test ./internal/harness -count=1` 整包 | **PASS（exit 0，零失败）**——blocked 轮唯一失败例已由改写消解，无新增 |
  | ③ `go build ./...` / `go vet ./internal/harness/` | 双 exit 0 |
  | ④ 原始输出 | `Export/harness-fix-1-20260917/rework-*.txt`（two-tests / full-package / go-build / go-vet / red-baseline；Export/ gitignored，可 SSH 只读核查） |
  | ⑤ 新语义固化 | 改写后测试断言 Z3 + `kernel.commands` 为空；红绿对照：基线 04fa36d（兜底在场，独立 worktree）上该测试 **FAIL**（exit 1，`scan_plugins` 副作用被哨兵捕获，rework-red-baseline.txt）→ 本分支 PASS |

  - 端测边界声明：rework 轮沿用裁定三（单元级验收足够），未含真实栈烟测；真栈 zone 解析行为验证并入 C3。
- 验收：
- 关联：取证见 `coord/cards/done/2026-09-16-PORT-MAC-HARNESS-FLAKY-1.md`（证据矩阵与三变体对照）；`Get()` 模糊兜底跨插件错配隐患（index.go:253）**不在本卡**，登记为独立待裁项
