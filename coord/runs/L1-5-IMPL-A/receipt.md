# L1-5-IMPL-A 回执 — PullLoop 骨架+观察预算+flag 面（pullharness 新包，零旧面改动）

- 日期：2026-10-09；执行侧：PC flash（ZCode 执行会话）
- 领取：origin/main=4c2f7559，分支 **port/l1-5-impl-a**（worktree D:/Vit_DAW_wt_l5a）
- 被测 HEAD（worktree 全程未动）：`4c2f75596e3571ab9f1ef3eccdaeb6fd25bbdbc8`
- 工作树领取基线：干净（`git status --short` 仅本卡新增文件）；主工作树的他人改动（Settings.xml/default_project.xml/history_refs.go）未触碰

## 实现面（全新包 agent/internal/pullharness/，共 1344 行，全部新文件）

| 文件 | 行数 | 内容 |
|---|---|---|
| loop.go | 402 | PullLoop 六注入（LLM/Prefix/Tools/Exit/Router/Budget）+七步循环（①快路径 nil-safe ②PrefixService 装配+动态区 ③溢出预检 fail-closed ④模型调用/终态判定 ⑤工具批 ⑥T1 ⑦预算检查）+T2 终局+interrupt 暂停面；冷启动槽=空操作（IMPL-B 填） |
| budget.go | 92 | ObservationBudget（MaxCycles/ProbeCost/MaxProbeCost）+CycleExhausted/ProbeExhausted（上限<=0=不设限，沿 checkTurnBudget 约定）+Disclosure 动态区披露（剩余<=1 循环节预警，无比例魔法数）+终态分类常量族 |
| mode.go | 122 | flag 面：env VIT_DAW_HARNESS（env wins）> VitApp/Workspace/agent_runtime_config.json 键 harness_mode > 缺省 push；UTF-8 BOM 容错=TrimPrefix 单点（CONFIG-BOM-1 教训）；无效值 fail-visible（config_invalid/environment_invalid 显式 Source）+回落 push |
| *_test.go | 730 | 19 个测试（清单见下） |

消费的冻结接口（全部只读消费，零改动）：`promptruntime.PrefixService/PrefixRequest/AssemblyInput/Build/TextSection`、`contextruntime.ExitExecutor/OnTurnBoundary/TurnBoundaryEvent/WindowState/BudgetState/ExitUnit*/ModelContextOverflow`、`llm.Completer`。

## 验收命令与退出码（worktree 内执行）

| 命令 | 结果 |
|---|---|
| `cd agent && go build ./...` | exit 0 |
| `go test ./... -count=1` | **exit 0**，91 包 ok，0 FAIL（全量日志：本目录 `gotest_full.log`） |
| `gofmt -l internal/pullharness/` | 空输出（净） |
| `go test ./internal/pullharness/ -count=1 -v` | 19/19 PASS |

## 冻结接口零改动证明（验收标准 3）

worktree 相对 HEAD 的 tracked diff **整体为空**（实现全部为新增文件，无任何既有文件被改）：

```
$ git status --short
?? agent/internal/pullharness/
$ git diff HEAD --stat
（空）
$ git diff HEAD --stat -- agent/internal/promptruntime agent/internal/contextruntime agent/internal/agentprotocol
（空）  ← promptruntime/contextruntime/agentprotocol 三包 diff 为空
```

## 新测试名（19）

- loop 面：TestPullLoopT1OncePerBatchWithTurnIDSequence（T1 每批恰一次+TurnID 序列+T2 窗面累计）、TestPullLoopT2ExactlyOnceOnTerminals（终局判定/快路径短路两终态路径各恰一次 T2、零模型轮）、TestPullLoopPauseSurfaceNeverFiresT2（暂停面零 T2）、TestPullLoopBudgetExhaustedOnCycleCap（循环节超限→budget_exhausted，⑥→⑦ 顺序实证）、TestPullLoopBudgetExhaustedOnProbeAccount（probe 账户逐笔计量+入场即越线腿）、TestPullLoopOverflowFailsClosedWithoutRequest（溢出 fail-closed：LLM 零调用+工具零执行+T2 一次）、TestPullLoopClassifyTerminalSeam、TestPullLoopAssemblyConsumesPrefixService（SessionKey= pullharness:<RunID>、冷启动槽空、动态区披露）、TestPullLoopInjectionDefaults、TestPullLoopLLMErrorFiresT2
- budget 面：TestObservationBudgetCycleExhausted / ProbeExhausted / TestBudgetExhaustedIsDistinctFailureClass（§8 分记义务锁定）/ TestObservationBudgetDisclosure
- flag 面（四态+附加态）：TestHarnessModeEnvWins / ConfigKey / DefaultPush / BOMTolerated / FailVisible

## 设计语义决策点（决策侧复核清单——实现中自行裁定的最小面）

1. **LLM 注入类型**（§3.1 伪码写 `llm.Client`）：Client 是 struct 不可注入 fake，取其在 llm 包的既有接口面 **`llm.Completer`**（`*llm.Client` 实现之；`llm.CompleteText` 既有助手同型消费）——llm 包零改动，fail-closed 契约可测。
2. **GoalInput/Result 包内最小面**（OQ-H1/H3 预留的复核点）：代码库无 GoalInput 既有类型，按 agentloop 同构在包内定最小面（RunID/GoalID/UserText/Context/Conversation/ContextSnapshotJSON/Engine/ClassifyTerminal；Result 带 Conversation 载体+Trace 行），不拖 planner/agentruntime 依赖。
3. **ClassifyTerminal 分类缝**：终局判定（无工具调用轮）的失败分类（§3.4"沿 push 既有口径"）在 A 阶段无生产面，挂 GoalInput 函数缝，nil=judgment_ok 缺省；真实分类接线归 IMPL-D。
4. **暂停面载体**：A 阶段无显式 pause 面，以 **ctx 取消=interrupt**（Outcome=interrupted，零 T2，Conversation 随 Result 存续）作为"暂停零触发"的可测面——continuation 完整结构归后续卡。
5. **T1/T2 与溢出预检先后**（卡面歧义示例）：设计 §3.1 步骤序已消歧（③ 溢出预检在模型轮前，⑥ T1 在批后）——**未触发停止条件**，无申报。
6. **预算执法位置**：⑦ 单检查点在批后（设计字面）；终局轮（④ 无工具调用）直接出循环，不消耗/不触发 ⑦——MaxCycles=2 时模型恰有 2 个工具轮机会，终局轮不计入。
7. **Router 命中语义**：短路终态（零模型轮、零 T1、零 cycle 消耗）+T2 照常+probe 成本照记；Outcome 透传 Router 值（空→"fastpath" 占位，分类口径归 IMPL-C 定，OQ-H2）。
8. **env 非法值**（banana）：沿 blind 面"env 设了即赢"语义不回落配置面，push 兜底+`environment_invalid:` 显式留痕（fail-visible）。

## 烟测豁免边界（如实复述）

本卡纯新增包、无生产入口消费（flag 读取器无调用方，loop 无调用方；接线归 IMPL-B/C/D）——按决策侧预裁定豁免真实栈烟测，单元级验收充分。豁免边界：**本包当前对生产运行栈零行为面**；G3 A/B 真栈门槛由 IMPL-D 承担（AGENTS §5 不豁免）。

## 工件

- 全量测试日志：`coord/runs/L1-5-IMPL-A/gotest_full.log`（91 包 ok / 0 FAIL）
- 实现 commit：见 port/l1-5-impl-a 分支（hash 回填至任务卡 done 行）
