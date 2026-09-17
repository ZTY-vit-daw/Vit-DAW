# PORT-MAC-HARNESS-FLAKY-1：harness 2 例 darwin 失败根因排查

- 优先级 / 预估 / 依赖：P2 / 0.5-1 天（取证卡）/ 无
- 背景：Mac darwin/arm64 上 `TestFullProjectAccessDoesNotDisplaceExplicitSelectionAuthorization` 与 `TestAgentProcessorLoadGateRechecksPCAAndBlocksBypass` 失败；干净 worktree @`ee0fac4` 复跑同样失败（与 C1 无关）；Windows 同版本 PASS（决策侧 2026-09-16 复验）
- 目标：定位 mac 特有根因（候选方向：路径分隔符 / 大小写敏感 / 文件锁语义 / 时序 / TempDir 行为差异），产出取证报告；根因在共享代码 → 附最小修复建议转修复卡；属 mac 环境语义 → 文档化到 PORT_AUDIT §7 或独立风险条目
- 文件域：只读取证为主；如需实验性补丁，放独立分支不进 main
- 验收标准：失败原因有代码锚点或最小复现；原始失败输出已保存（AGENTS §11 不稳定测试纪律）
- 停止条件：2×20 分钟仍无法缩小假设面 → 上交已收集证据转 blocked
- 领取：2026-09-17（Mac 执行侧，用户口令「开工 FLAKY-1」）/ origin/main=30307e14c2a6cad40f853c7c9bd1ed07fb1103b6（工作树干净）/ 取证只读无实现分支；实验性补丁（如需）→ port/flaky-1-forensics
- 回执：**取证完成（2026-09-17，Mac 执行侧）——根因定位：非 darwin 平台语义、非时序 flaky；是测试对机器全局状态 `~/.vit/plugin_semantics.json` 的隐性依赖，叠加共享代码在命令规范化阶段的内核副作用。属共享代码缺陷，附修复建议转修复卡。**

  ## 机制链（代码锚点）

  1. `Harness.Invoke`（harness.go:880）→ `resolveImplicitTargets` → `resolveRackAddNodeZone`（harness.go:8957）→ `rackPluginSemanticEntry`（harness.go:8986）。
  2. `rackPluginSemanticEntry` 先读本地语义索引 `pluginsemantics.Load("")` → 默认路径 `~/.vit/plugin_semantics.json`（pluginsemantics/index.go:66-75）。
  3. **索引缺失（干净机器）时落到内核兜底**（harness.go:8996-9012）→ `listAvailablePlugins` → `scanAvailablePluginRows` → `h.kernel.SendCommand({"cmd":"scan_plugins"})`（harness.go:8510）——即：**被 PCA 门控即将拒绝的命令，在规范化阶段就已对内核产生了 scan_plugins 副作用**（早于 enforceAgentProcessorLoadGate，harness.go:891）。
  4. 两测试断言被拒 Invoke 前后 `len(kernel.commands)` 不变（processor_load_gate_fullaccess_test.go:254 / processor_load_gate_test.go:54-55）；mac 上每次带 `plugin_path` 的 rack.add_node Invoke 都多派一条 scan_plugins → 计数被污染 → FAIL。失败输出实锤：kernel 收到 `[scan_plugins, rack_add_node, scan_plugins]` 三条（repro-run1.txt）。
  5. **Windows 为何 PASS**：Windows 开发机经真实栈使用已生成非空 `~/.vit/plugin_semantics.json`；`pluginsemantics.Get`（index.go:246-258）精确匹配失败后**用整个插件路径做模糊搜索兜底（Limit 1）**，TempDir 路径含 "EQ" token → 命中索引中任一 EQ 类条目 → 提前返回、不碰内核 → 计数守恒。Mac 为新机无索引文件 → 走内核兜底。

  ## 证据矩阵（本机 darwin/arm64，`-count=1`，工件 Export/flaky1-forensics-20260917/）

  | 变体 | 结果 |
  |---|---|
  A. 机器默认（无 `~/.vit/plugin_semantics.json`）| FAIL ×2（repro-run1.txt；3 次重复全 FAIL=repro-3x.txt，确定性，非时序）
  B. `VIT_PLUGIN_SEMANTICS_PATH`→含 1 个 EQ 条目索引 | **PASS ×2**（variant-populated-index.txt，exit 0）
  C. `VIT_PLUGIN_SEMANTICS_PATH`→空条目索引 | FAIL ×2（variant-empty-index.txt，exit 1）
  整包 `go test ./internal/harness -count=1` | exit 1，失败**有且仅有这 2 例**，各 0.01s（harness-full-package.txt；与 A2 验收回执口径一致）

  B/C 对照证明：通过条件=「索引文件存在**且**含可被路径 token 模糊命中的条目」——Windows 通过是开发机状态的巧合，任何干净机器/CI 都会失败。"FLAKY" 是误称，实际是机器状态确定性依赖。

  ## 为何恰是这 2 例（一致性核对）

  - `plugin.load_to_rack` 系测试（RechecksV2PCA / LeavesManualPath）：不断言内核命令计数，且 fakeKernelClient 回复耗尽返回默认 ok（harness_test.go:302-313）→ scan 副作用不触发断言 → PASS。
  - fullaccess 兄弟例 Probe-1/2/2b/3：或不传 `plugin_path`（rackPluginSemanticEntry 对空路径直接返回，harness.go:8988-8990），或用 `rackAddNodeCommands` 过滤后断言（只数 rack_add_node，scan_plugins 不计入）→ PASS。
  - 仅这 2 例「args 带 plugin_path + 断言原始内核计数」→ 恰好命中。

  ## 修复建议（转修复卡，供决策侧择案）

  - **方案 A（生产修复·根治）**：删除 `rackPluginSemanticEntry` 的内核兜底（harness.go:8996-9012），索引未命中走既有默认 Z3 分支（harness.go:8969-8971）。恢复「被拒命令零内核副作用」不变量，两测试在任何机器无需夹具即过。代价：无索引机器上乐器 zone（Z2）无法从内核清单解析，乐器节点默认落 Z3——zone 正确性 tradeoff 需决策裁定。
  - **方案 B（测试隔离·最小 diff）**：两测试内 `t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", <指向临时含 EQ 条目索引>)`，全部机器确定性复现 Windows 现状通过。固化现状语义（规范化阶段读索引合法），但把「被拒命令可先触发内核 scan」的现状一并固化，不修复分层缺陷。
  - **方案 C（分层修复·中间态）**：保留内核兜底但改为只读 `plugin_list_available`——仍破坏零副作用不变量，单独采用不解决测试失败，仅作 A 的补充（scan→list 降载）。
  - 附带发现（建议随修复卡处置）：`Get()` 的模糊兜底（index.go:253）使 zone 解析可能拿**另一个插件**的条目回答"本路径是否乐器"——精确查找意图下静默错配，正确性隐患独立于本失败。

  ## 工件与边界

  - 原始输出（AGENTS §11）：Export/flaky1-forensics-20260917/{repro-run1,repro-3x,variant-populated-index,variant-empty-index,harness-full-package}.txt（Export/ gitignored，决策侧可 SSH 只读核查，PROTOCOL §9）
  - 本卡零源码改动（工作树除卡片外干净）；实验经 `VIT_PLUGIN_SEMANTICS_PATH` 环境变量隔离，未触碰机器全局 `~/.vit/`；无实现 commit
  - 测试命令：`go test ./internal/harness -run 'TestFullProjectAccessDoesNotDisplaceExplicitSelectionAuthorization|TestAgentProcessorLoadGateRechecksPCAAndBlocksBypass' -count=1`（A: exit 1 / B: exit 0 / C: exit 1）+ 整包 exit 1（仅既有 2 例）
  - 端测边界声明：本卡为纯单元级取证，无生产改动，不涉渲染面/旅程/真实栈烟测
- 验收：**pass**（裁定 [2026-09-17-FLAKY1-pass.md](../../rulings/2026-09-17-FLAKY1-pass.md)，决策侧复现变体 A + 三锚点核实 + 工件抽查）；修复方案 A 定案转卡 [PORT-HARNESS-FIX-1](../todo/2026-09-17-PORT-HARNESS-FIX-1-zero-kernel-side-effect.md)；模糊兜底错配隐患登记独立待裁
