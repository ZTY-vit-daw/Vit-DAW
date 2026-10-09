# L1-5-IMPL-B 回执：冷启动底座+披露位迁移+真实退场接线

- 卡：coord/cards/doing/2026-10-09-L1-5-IMPL-B.md（PC 执行会话，worktree D:/Vit_DAW_wt_l5b，分支 port/l1-5-impl-b）
- 领取基线：origin/main=a2b82564（领取 commit a44030bf）；实现 commit 见下方"提交"节
- 设计权威：docs/HARNESS_V1_DESIGN.md §4（冷启动+披露位迁移表）+ §2/§3（轮次语义/骨架）；旧面（agentloop/chat/harness）零改动

## 1. 交付内容（按卡面三目标）

### 1.1 冷启动底座（agent/internal/pullharness/coldstart.go，369 行）

- 会话首装一次性 session 层 Section（Stable=true，Kind=SectionSession），固定序 TOM→总线→交付→规则；run 内首装一次后同值重挂（字节恒定）。
- 三事实组与 L4 genesis 头部段（harness/project_genesis.go，L4-GENESIS-1）同源，经既有只读消费面重渲染，**不复制 harness 包私有函数、不改 harness 包**（文本化在本接线层完成，与 genesis 接线层同责）：
  - TOM 概览：`capabilitycontext.BuildProjectTOMProjection(engine, "", "")` → llm_context.summary_md + full_assignment_manifest 逐轨行（组序/行内序即快照序，封顶 32 行+`+N more` 注记）；
  - 交付目标：`rlm.BuiltinDeliveryProfiles()`（固定目录序，封顶 8 行；工程态 render→profile 绑定面归 IMPL-D 接线供给，不在本渲染面）；
  - 总线拓扑：同源内核快照 parent_track_id/parent_folder_track_id/vit_type/is_submix_folder/routing_bus_enabled 文本化（tracks 数组序即输出序，封顶 32 边）。
- 缺失投影→该事实组 absent 不臆造：缺席组不渲染 Section，经 `PrefixRequest.LayerStates` 声明 "absent" 进装配报告（§2.0 fail-open 但显式）。
- 输入缝：`ColdStartSource` 接口（+ColdStartSourceFunc 适配）——engine_snapshot 同源面，harness 侧由 shadow.Snapshot()["engine_snapshot"] 供给，pull 侧真实接线归 IMPL-D。
- 确定性说明：BuildProjectTOMProjection 内部 CreatedAt 时间戳不进所消费的 summary/manifest 字段（亲核 tom/projection.go summaryMD 与 compactFullManifestFact），字节恒定不依赖时钟。

### 1.2 披露位迁移（agent/internal/pullharness/disclosure.go，67 行；§4.2 表四行）

| 行 | 现役披露位 | 去向 | 落点 |
|---|---|---|---|
| 1 | GATE PATH 预披露（ccb_model_prompt.go:220/:273） | 冷启动底座规则段（一次性指引，不再逐轮复现） | coldStartRulesContent 锚 `GATE PATH (standing rule disclosure):` |
| 2 | G1-G8 证据门指引（:582） | 冷启动底座规则段（L1 规则域） | 同上锚 `EVIDENCE GATES (G1-G8, standing rule disclosure):` |
| 3 | TERMINAL TURN（:358） | 动态区逐轮状态指令 | terminalTurnRow 锚 `TERMINAL TURN (mechanical runtime state):`（剩余循环节 counters+终态轮指令语义） |
| 4 | 预算指令 | 动态区 BudgetState 披露+预算指令行 | budget_state 行（A 面 Disclosure 既有机械事实+预警）+ budgetInstructionRow 锚 `BUDGET (mechanical runtime state):` |

- 语义沿既有文本，旧 harness 零改动（复用语义不复用其内部函数）；行 1 的 machine-readable gap 结构沿响应面既有语义保留（不属装配面）。
- 四行正置+错位双断言（各行在指定段、不在他段；"不再逐轮复现"由错位断言锁定）。

### 1.3 真实退场接线（agent/internal/pullharness/exit_wiring.go，100 行 + loop.go 槽位）

- `NewWiredExitExecutor(cfg)` 生产构造：真实 `contextruntime.NewExitExecutor`（既有机械判据零新造）；IMPL-D 经此注入，不再用测试 fake。
- TurnBoundaryEvent.Window/Budget 构造落位：`BatchTurnEvent`（T1，TurnID=`<RunID>:cycle:<n>`，单元=工具结果行）与 `FinalTurnEvent`（T2，TurnID=RunID，exit_retain 先例；累计窗复制）两构造函数收拢并测试锁定；线位经 `ExitWiring{HistoryLimit, HotLimitBytes, Now}`（零值=A 阶段形态不变）。
- T1 三态（retain/ref/drop）走既有机械判据零新造——真实执行器双层验证（构造函数层+整场 Run 层），见 §3 测试清单。
- 结论级 retain 供给通道：`ToolResult.RetainedStatement`（上游显式提供，WindowUnit 同名语义）——观察结论经 T1 批轮进三态判据。
- **边界声明（生产消费切换归 IMPL-D）**：报告消费（WriteRetains 落盘/遥测 WARN 接线）维持 advisory 形态不属本卡——exit_executor.go 头注先例"生产消费切换归 IMPL-D"；T2 终局事件窗面=run 累计工具单元，单元 TurnID 标批轮次≠T2 TurnID，既有判据不重复判定（结论供给在 T1 批轮完成，无双重入账）；"终局回复是否作为结论单元供给 T2"属 IMPL-D 生产消费接线时的裁定面，本卡不臆造。

## 2. loop.go 槽位填充清单（卡面授权范围：冷启动空槽与必要接口槽）

1. `ToolResult` 增 `RetainedStatement` 字段（T1 三态经批面的必要通道；加法式，A 测试零改动全过）。
2. `PullLoop` 增 `ColdStart ColdStartSource` / `ExitWiring ExitWiring` 字段（nil/零值=A 阶段形态逐字节保持，TestColdStartNilSourceKeepsAForm 锁定）。
3. `Run()`：冷启动首装一次（renderColdStart），产物线程至 assemble。
4. `assemble()`：SystemSections=底座 Section 族+LayerStates 缺席声明。
5. `dynamicZone()`：dynamicStateRows（披露位行 3/4 落点）。
6. `fireTurnBoundary()`/`finish()`：事件构造委托 BatchTurnEvent/FinalTurnEvent。
7. 移除 A 阶段空槽方法 assembleColdStart 与 time import（time.Now 移入 exit_wiring.go 构造函数）。

## 3. 新增测试（15 项，全 PASS）

| 测试 | 验收点 |
|---|---|
| TestColdStartByteConstantAcrossHundredAssemblies | 同输入 ×100 渲染 digest 全等+×100 PrefixService 装配逐层 content_hash 全等（卡面"×100 装配 content_hash 全等"） |
| TestColdStartThreeFactGroupsPresent | 三事实组内容就位（TOM summary+逐轨行/总线 routes+bus/交付 builtin profile 行） |
| TestColdStartMissingProjectionsAbsent | 缺失投影 absent 不臆造（nil 快照/无路由快照两形态+LayerStates+报告行双面断言） |
| TestColdStartFirstInstallOnceAndFrozen | 首装一次性：快照源只读一次，中途变异不改变底座，跨轮装配逐 Section 全等 |
| TestColdStartNilSourceKeepsAForm | nil 源=A 阶段形态保持（空 Section/空声明） |
| TestDisclosureFourRowsInPlace | 四行就位+错位零+行 1/2 语义锚（ccb.observation_catalog/observation_request/machine-readable structured gap；G1-G8/Do not rush the proposal/no default view sequence） |
| TestTerminalTurnRowMechanicalCounters | 行 3 counters（capped/uncapped）+终态轮指令 |
| TestBudgetDisclosureWarningInDynamicZone | 行 4 预警随机械判据（剩余≤1）出现/不出现 |
| TestDisclosureRowsMountedViaColdStartRender | 规则段经 RenderColdStart 实际挂载 |
| TestColdStartAndDisclosureContentBlind | 内容盲审查：审查键封闭枚举（facts/discipline/catalog/output_format；规则段=discipline、事实段=facts）+27 项域处理禁词扫描+阳性对照防词表空转 |
| TestBatchTurnEventConstructionPlaced | T1 事件字段透传+线位/时钟落位 |
| TestFinalTurnEventConstructionPlaced | T2 事件 TurnID=RunID+累计窗所有权分离 |
| TestNewWiredExitExecutorRealCriteria | 真实执行器语义冒针：retain 产 LedgerEntry/ref 带 HandleRef/drop/证据负担无结论无句柄→拒绝退场（不变式） |
| TestExitWiringBudgetLineEnforced | HotLimitBytes 落位后 budget 判据既有规则执法（T2 累计窗场景，最旧优先淘汰） |
| TestPullLoopT1ThreeStatesWithRealExitExecutor | 整场 Run：T1×2+T2 TurnID 序列、retain（LedgerEntry.Statement 全等）/drop/ref 三态、T2 窗面累计（3 单元/120 字节）不重复判定 |

## 4. 验收命令与退出码（cd agent 后执行）

| 命令 | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go test ./... -count=1` | **exit 0；92 包 ok、0 FAIL**（日志 /tmp/l5b_fulltest.log） |
| `go test ./internal/pullharness/ -count=1` | exit 0（A 卡既有测试零改动全过+本卡 15 项新测试） |
| gofmt | 净（口径说明：本机 git autocrlf 检出为 CRLF，gofmt -l 对全树含 A 卡原文件报行尾；按 LF 归一化内容断言 gofmt 语义净，8 个本卡涉及文件逐一过检；未对 A 卡文件做行尾重写，diff 零噪音） |

## 5. 旧面零改动证明

```
$ git diff --stat -- agent/internal/agentloop agent/internal/chat agent/internal/harness
（空输出）
$ git diff -- agent/internal/agentloop agent/internal/chat agent/internal/harness | wc -l
0
```

本卡 diff 全集：`agent/internal/pullharness/loop.go`（58+/65-，§2 槽位清单）+ 7 个新增文件（3 实现+4 测试，1224 行）。

## 6. 内容盲审查结论

**通过**。底座三事实段（机械事实行）+规则段（GATE PATH/G1-G8 语义迁移文本）+动态区披露段（TERMINAL TURN/BUDGET 指令）经机械审查：审查键封闭枚举全有效（规则段=discipline，事实段=facts）；27 项域处理禁词（track_gain/static_eq/compressor/equalizer/reverb/fade/stems/silence/crest/headroom/"consider "/"apply " 等处理知识与引导词族，取自 push 侧 Pattern Recognition/preflight 域动词族）零命中；阳性对照（"often indicates level-imbalance; consider track_gain adjustment"）确认扫描器有效。注：CCB 视图 ID（mix.* / track.*）与 LUFS/dBTP 参数事实不属禁列——目录结构与引用面，非处理知识（同 ccb_model_prompt.go 对 G3 行的既有裁定）。

## 7. 停止条件核验

- GENESIS 三事实组只读面 import 可达性：**可达，未触发**。TOM=capabilitycontext.BuildProjectTOMProjection（导出）；交付目标=rlm.BuiltinDeliveryProfiles（导出）；总线拓扑=同源内核快照字段文本化（接线层职责，与 genesis 接线层同责，无需导出 harness 私有函数）。
- 披露语义与循环时序冲突：**未发现**。TERMINAL TURN/预算指令落动态区（每轮重算，随计数/预算账户变化）；GATE PATH/G1-G8 落一次性底座（不逐轮复现，错位断言锁定）。

## 8. 端测覆盖边界声明（AGENTS §5）

本卡为纯新增包+包内槽位填充，不触碰任何生产入口/flag 面/webui/旅程面（缺省形态逐字节保持由 TestColdStartNilSourceKeepsAForm 等锁定）；真实栈烟测腿按排程归 L1-5-IMPL-D（G3 A/B 执行卡，双模入口接线+真栈 exit 0 门槛）。故本卡未执行真实栈烟测，验收=卡面单测级门槛（全过）。

## 9. 提交

- 分支：port/l1-5-impl-b（基于 a2b82564）
- 实现 commit：见 git log（本回执随实现同 commit 提交）
- 工件：本回执 coord/runs/L1-5-IMPL-B/receipt.md
