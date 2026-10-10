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

- 领取：（时间 / origin/main hash / owner 模型+机器+会话 / 分支 / worktree / 领取提交）
- 回执：（commit hash / 分级表锚点 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
