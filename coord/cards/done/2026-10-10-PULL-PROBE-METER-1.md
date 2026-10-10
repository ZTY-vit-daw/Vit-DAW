# PULL-PROBE-METER-1：probe 物理成本计量接入（execRecord elapsed_ms + settleBatch D5 分级累加）

- 发卡：GLM 主管决策侧 / 2026-10-10
- 派发确认：已确认（用户 2026-10-10 对话裁定推进超大工程自由态烟测线；依据=[G3-PROBE-METER-RECON-1 报告](../../runs/G3-PROBE-METER-RECON-1/report.md) 层(a)结论：有源、接线点已预声明）
- 验收负责人：GLM 主管决策流
- 池序 44；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（main ≥171fb223）
- 模型分级：L1 / flash 可接（两文件接线+测试；锚点全在勘察报告里）

## 要解决的问题（勘察报告 §1 已核）

pull 账本的账户/执法/披露三面俱全（pullLedger.probeSpent/probeCostKnown + budget.go），唯缺逐笔成本生产者：execRecord 九字段无计量键（runner.go:558-567）、InvokeResponse 无耗时字段 → settleBatch 只能计数并把 probeCostKnown 翻 false（max_probe_cost 执法停用，成本面读数为空）。**接线点已在 pull_session.go:411-430 注释预声明**："有真实计量源后在此接入"。

## 目标（勘察报告层(a)，充分层）

1. **execRecord 增 `elapsed_ms` 键**（agent/internal/agentloop/runner.go）：工具执行墙钟（executeTool 处 started/time.Since，与 harness.go:879 A1 源同粒度）；零成本路径不填 0 冒充——异常/未执行不写键。
2. **settleBatch 按 D5 分级累加**（agent/internal/agentloop/pull_session.go）：probe 类工具（render/probe 级：ccb.observation_request / mix.observe / mix_request_observation 等观察与渲染面）的 elapsed_ms 逐笔累加进 `ledger.probeSpent`；index 级（ref.query/ref.diff/catalog 读）不计（D5 语义 budget.go:17-19）；有计量回执不再翻 `probeCostKnown=false`（无键回执维持现状翻 false——诚实语义保留）。
3. **披露行真实化**：budget_state 的 probe_spent=实测累计值（budget.go Disclosure 零改动，数据源变真）。
4. **测试**：已知成本累加+预算越线执法（probeCostKnown=true 路径）+无键回执翻 false 回归+index 级不计+披露行渲染实测值。

## 工具分级表（执行侧核实，争议即停）

| 级 | 工具/命令 | 计量 |
|---|---|---|
| probe（render/observe） | ccb.observation_request、mix.observe/mix_request_observation、观察类命令 | 逐笔累加 |
| index | ref.query、ref.diff、catalog/state 读 | 不计 |

分级争议（如某工具两属性）→ 上交裁定，不私定。

## 文件域

`agent/internal/agentloop/runner.go` + `agent/internal/agentloop/pull_session.go` + 新增/扩展测试。≤4 文件；越域即停。

## 约束与红线

诚实红线：未计量不填 0（勘察报告"层(a)含调度噪音，勿冒称纯内核成本"注记随卡生效——账本记的是 agent 观测墙钟）；push 模式行为零变化（本卡只动 pull 结算面+通用 execRecord 加法键）；既有测试零改动全绿。

## 验收标准

`go build ./...` + `go test ./internal/agentloop -count=1` 全绿 + 全量 `go test ./... -count=1` 0 FAIL + 触碰文件 blob 级 gofmt 净（等价证明口径=HEAD blob==gofmt(父 blob)+CR=0）；回执附分级表实测锚点。

## 停止条件

execRecord 加键破坏既有序列化/快照消费方（发现消费方对未知键 fail-closed）→ 锚点+形态上交；分级表争议 → 上交。

## 并行与资源

纯单测域，不需要真栈；与 REFSCHEMA-M4B/M5 文件域不相交可并行；与 FS-LARGEPROJECT-SMOKE-1 无硬依赖（烟测验收口径为行为面，本卡并行落地、合入后烟测成本面即有读数）。

- 领取：2026-10-10 18:20 / origin/main 491555c8f703d94244285f2ffc61dc2b0fb21178（依赖 ≥171fb223 满足，merge-base --is-ancestor 实核） / owner=GLM-5.3-Flash（ZCode flash 会话）/ PC (Windows) / 分支 port/pull-probe-meter-1（基线 491555c8） / worktree D:/Vit_DAW_wt_pull_probe_meter_1（独立 worktree，状态净）/ 领取提交=3cd4efa4（card mv 协调提交，已推 main）
- 回执：实现 commit **12792d0a**（port/pull-probe-meter-1，已推 origin；3 文件 +237/−3：runner.go +5 / pull_session.go +72−3 / 新增 pull_probe_meter_test.go +163，≤4 文件域内）。
  - **验收命令与退出码**：`go build ./...` exit 0；`go test ./internal/agentloop -count=1` ok；全量 `go test ./... -count=1` exit 0（0 FAIL，既有测试零改动全绿）；AGENTS §6 五组健康测试全 ok。
  - **gofmt blob 级口径**：三文件暂存 blob GOFMT-STABLE（`git show :file | gofmt` 恒等）+ `git ls-files --eol` 实证 i/lf（blob CR=0；工作树 w/crlf 为 autocrlf 检出形态，非 blob 属性）。
  - **分级表锚点（实现面）**：分类函数 `probeToolTier`（pull_session.go，名称归一=下划线视作点号，一处 switch 可扩表）。probe 级=`ccb.observation_request`/`ccb_observation_request`、`mix.observe`/`mix_observe`、`mix.request_observation`/`mix_request_observation`（elapsed_ms 逐笔累加）；index 级=`ref.query`、`ref.diff`、`ccb.observation_catalog`、`project.state`（D5 定义零成本，不计不翻，budget.go:17-19 语义）；**未列名=probeTierUnlisted：不入账且翻 probeCostKnown=false（维持既有 unknown 语义——既有测试 TestPullSessionUnknownProbeCostNotZero/TestPullContinuationUnknownCostCarried 因此保持绿）**。争议上交件：`clip.warm_waveform_bake`（变异+烘焙两属性，卡面"两属性"例）未私定，归 unlisted 待裁定扩表；同族候选 `mix.read`/`mix.derive`/`mix.report`/audition 面亦未列名未入账。
  - **计量键锚点**：runner.go executeTool `started:=time.Now()` + execRecord `"elapsed_ms": time.Since(started).Milliseconds()`（与 harness.invoke :879 计时同粒度）；结算面 settleBatch→settleRecordProbeCost（单次结算去重面沿用 ledger.settle，仅对本批 fresh 回执分级结算）。**probe_spent 单位=ms**（elapsed_ms 原子），max_probe_cost 注入方（G3 复跑）需按 ms 口径设线。粗时钟实测 ~0ms 属真实计量非冒充（键存在=已计量）。
  - **诚实红线核对**：未计量不填 0——无键/不可解析翻 false 不入账（elapsedMSFromRecord ok=false 路径）；账本记 agent 观测墙钟（含调度噪音，代码注释明示勿冒称纯内核成本）；push 模式零变化——settleBatch 仅 pullSession.Execute 调用，runner.go 仅通用加法键（push 侧消费方全为内存 map 读特定键，grep 实证无严格 schema 反序列化、无 T1 Bytes 精确断言）。
  - **端测边界声明**：本卡按卡面"纯单测域，不需要真栈"口径交付，未跑真栈烟测与 webui 渲染面（无 webui 改动）；probe 成本面真栈读数归 FS-LARGEPROJECT-SMOKE-1 行为面验收（合入后 budget_state 披露行即有实测值）。
- 验收：（裁定文件 / 验收 commit）
