# L1-5-IMPL-A-REVIEW-1 独立复核报告

日期：2026-10-09（Asia/Shanghai）。复核人：Codex GPT-6，Windows PC ZTY，会话 `01a11f26-bbcd-7d71-8a9d-909d10107633`。授权：用户明确要求执行本卡，只提交报告与证据，不修代码、不出最终裁定。

**建议返工**：A 的原有验收命令全绿，六文件提交内容格式正确，旧面零改动与零生产消费均成立；但“ctx 取消=暂停、零 T2”的实现只覆盖模型调用前/调用报错两处。三条确定性取消反例均已通过只读 overlay 复现，属于 A 当前骨架缺陷，不是 B/D 尚未填槽所造成。请主管决定 A 的修订范围与最终验收。本报告不产生 ruling，不解锁 B，不合入 A。

## 1. 版本、领取与集成证据

| 项目 | 实际值 |
|---|---|
| 领取前 main / 干净复验基线 | `a25d3b9d71f38eb8ffbec7019e5f986ac9871b83`；独立 worktree 的 `git status --short`、`git diff --stat` 均为空 |
| 领取提交 | `2986df4f`；只移动本复核卡 todo→doing 并写 owner，普通 fast-forward push 到共享 main，fetch 后核对远端 owner |
| 被审实现 | `de1d876c8b8e3e453d69b138e22bf7425df4f558`；原执行基线 `4c2f75596e3571ab9f1ef3eccdaeb6fd25bbdbc8` |
| 集成复验 HEAD | `eca150926b47573f012c5c6faffb65ea7e5ebff6`，由领取提交上 `git cherry-pick de1d876c` 产生，无冲突、无修改 |
| 复验分支 / worktree | `port/l1-5-impl-a-review-1` / `C:/Users/timoz/.codex/worktrees/l1-5-a-review-coord/Vit_DAW`；未复用作者 worktree |
| 原 A 状态 | 初次及测试后 fetch 均确认仍在 done、验收字段空、无最终 ruling；远端实现分支仍指向被审提交 |
| 并发 | 领取前 doing/blocked 为空；收口前 C-REVIEW-1 已独立领取，其报告域不同。本卡不占真实栈 |

领取卡中的 `13:38` 是本流手填时间误差；实际领取移动时间为 13:35:35+08:00，随后领取提交推送成功。收口卡面纠正为 13:35，保留原领取提交历史，不 amend。

实际集成新增六个 Go 文件和原 A 回执，7 文件、1409 行。`integration-diff.txt` 保存完整差异；`integration-evidence.json` 保存七文件被审/复验 blob ID，逐一相同。冻结三包 promptruntime/contextruntime/agentprotocol 以及 agentloop/chat 的集成 diff 为空。生产 Go 文件排除 pullharness 后搜索 `pullharness|ResolveHarnessMode|VIT_DAW_HARNESS`，无命中（rg exit 1 表示无匹配）。主工作树既有 Settings.xml/default_project.xml/history_refs.go 未带入、未处理。

## 2. 命令与直接退出码

所有 Go 命令 cwd 为上述 worktree 的 `agent/`，Go `go1.26.2 windows/amd64`。每个 `*-result.json` 保存命令、直接 `$LASTEXITCODE`、HEAD 和完成时间；原始输出独立保留。

| 命令 | 退出码与结果 | 原始证据 |
|---|---|---|
| `go build ./...` | 0 | `build.txt`、`build-result.json` |
| `go test ./internal/pullharness -count=1 -v` | 0，19 个顶层测试 / 19 PASS，loop 10 + budget 4 + mode 5 | `package.txt`、`package-result.json`，含全部实际名称 |
| `go test ./... -count=1` | 0，91 包 ok，0 FAIL；无 flaky 复跑 | `full.txt`、`full-result.json` |
| `gofmt -l internal/pullharness` | 0，工作副本列出全部六文件 | `gofmt.txt`、`gofmt-result.json` |
| 对 `git show de1d876c:<六文件>` 原始字节逐文件运行 gofmt 并比较 | 六文件均字节相等；工作副本仅 CRLF 转换，归一 LF 后均等于提交 blob | `gofmt-committed.json`，每文件 `committed_is_gofmt=true`、`committed_diff=""` |
| `go test '-overlay=../coord/runs/L1-5-IMPL-A-REVIEW-1/probe-overlay.json' ./internal/pullharness -run '^TestReview' -count=1 -v` | 1；三条取消契约断言失败，一条 Router 预算观测完成 | `cancellation-probe-output.txt`、`probe-result.json` |

第一次 overlay 命令未给整个 `-overlay=...` 参数加引号，PowerShell 分词导致 Go 把 JSON 路径当包，exit 1、`no tests to run`。此为本流命令设置错误，不是被审功能失败；原始记录保留在 `probe-setup-failure.txt` / `probe-setup-result.json`。纠正引号后才获得上表有效反例，未对功能失败原样重跑。

`cancellation-probe.txt` 是完整复现输入，`probe-overlay.json` 只把它映射成虚拟新增测试文件；源码树无实际 `review_probe_test.go`，原有源码/测试均未修改。测试使用 fake，不启停 VitApp/Godot/agent，不执行真实工具，不写真实工程。全量复跑后 tracked diff 仍为空；未观察到 fixture 或样例改写。本卡不声称具备 fixture 全量前后哈希证据。

## 3. 八项实现选择逐条复核

下列行号均为被审提交及复验 HEAD 的原始文件行号。设计依据为现行 `docs/HARNESS_V1_DESIGN.md`，未采用历史文档替代。

| # / 选择 | 源码锚点 | 设计依据 | 已有测试覆盖 | 复核结论与缺口 |
|---|---|---|---|---|
| 1 LLM 接口 | `loop.go:134`；`llm/client.go:104` 的 Completer | §3.1 既有 LLM 面、冻结接口；设计伪码 Client 为 struct | OverflowFailsClosedWithoutRequest、LLMErrorFiresT2、InjectionDefaults | **合理接口适配**：`*llm.Client` 现成实现 Completer，fake 可注入，llm 包零改动。真实协议适配仍归 D |
| 2 GoalInput/Result 最小面 | `loop.go:87`、`:112`、`:163` | §10 OQ-H1/H3 允许 A 定 goal 最小面、主管复核；chat 切换不在四卡内 | T1OncePerBatchWithTurnIDSequence、PauseSurfaceNeverFiresT2 | **骨架形态可用，恢复未证**：Conversation 有载体，不等于 continuation 完整；循环号/窗口局部重置，Result 不回传 Context/退场状态。不得宣称恢复幂等 |
| 3 ClassifyTerminal | `loop.go:105`、`:314` | §3.4 无工具轮诚实分类；真实执行面接线归 D | ClassifyTerminalSeam 只验默认和 capability_blocked | **仅占位**：nil 或空返回均 judgment_ok，不分析事实成功，Tools=nil 也直接终态。A 零消费可保留槽位；D 必须真实分类，补 no_candidate_found/异常协议等分记 |
| 4 暂停/取消 | `loop.go:169`、`:203`、`:214`、`:240`、`:260`、`:397` | §2、设计行 68：暂停/续跑零 T2；A 卡暂停零触发验收 | 原 PauseSurfaceNeverFiresT2 只在第二模型调用返回 ctx.Err；overlay 三反例补证 | **应修，见 F1**：其他取消时点仍会执行 Router/工具或落入 budget_exhausted/T2。保留 Conversation 不能抵消此违约 |
| 5 T1/T2 与溢出时序 | `loop.go:192`、`:253`、`:350`、`:382` | §3.1 ③溢出→④模型→⑤工具→⑥T1→⑦预算；§2 T1/T2 | T1OncePerBatchWithTurnIDSequence、T2ExactlyOnceOnTerminals、OverflowFailsClosedWithoutRequest、BudgetExhaustedOnCycleCap | **正常路径时序符合**：每批 T1、终局 T2。T1 返回值被丢弃、历史不出窗；T2 累计单元仍标 cycle ID，无结论供给和持久化，不能称 retain 收口；真实消费归 B |
| 6 预算检查点 | `budget.go:51`、`:56`、`:63`；`loop.go:163`、`:234`、`:260` | §3.1 ⑦批后；§3.4 预算止损 | BudgetExhaustedOnCycleCap、BudgetExhaustedOnProbeAccount、四 budget 测试 | **工具路径批后止损成立**，>= 上限结束，<=0 不设限；不是执行前成本限额。已超限入场仍多执行首批，无工具终局跳过⑦。Router 完全旁路⑦，见 F2，请主管明确例外 |
| 7 Router 短路 | `loop.go:73`、`:81`、`:169` | §3.1 ①、§5 命中短路；§10 OQ-H2 独立分类归 C | T2ExactlyOnceOnTerminals：一个首轮命中，零模型/零 T1、计 probe | **短路占位符合，取消/预算不完整**：任意 Outcome 透传，空→fastpath；Route 每循环取同一个原始 GoalInput，无当前历史/预算。D 适配时需核对 C 分类、diagnostic 旁路、作用域 |
| 8 非法 env | `mode.go:66`、`:72`、`:84` | §6 env wins、默认 push；原卡 BOM 容错 | EnvWins、ConfigKey、DefaultPush、BOMTolerated、FailVisible | **保守回退合理**：非空非法 env 阻断配置并返回 push+environment_invalid；空白 env 视未设。Source 是可见载体，当前无调用者，是否用户可见/启动仅读一次留 D 验证 |

## 4. 问题分级与可复现事实

### F1，应修（P1）：取消检查覆盖不全，违反暂停零 T2

锚点：`loop.go:169` Router 在 ctx 检查前调用；`:214` 只在 llmErr 非 nil 时检查模型阶段取消；`:240` Execute 后立即累计轮数并在 `:260` 检查预算，无取消分流。`:382` finish 无条件执行 T2（非空 RunID、Exit 在位）。

有效 overlay 结果：

| 触发 | 实际输出 | 与契约的冲突 |
|---|---|---|
| ctx 在 Run 入场前取消，Router 命中 | `outcome=fastpath router_calls=1 boundary_ids=[cancel-router]` | 取消后仍调用可有执行副作用的 Router，并触发 T2 |
| Execute 取消 ctx 并返回空结果，MaxCycles=1 | `outcome=budget_exhausted cycles=1 boundary_ids=[cancel-tool:cycle:1 cancel-tool]` | 中断批被计为完成轮次，预算终态触发 T2 |
| Complete 在取消 ctx 后返回响应且 error=nil，MaxCycles=1 | `outcome=budget_exhausted tool_executions=1 boundary_ids=[cancel-model:cycle:1 cancel-model]` | 已取消的模型返回仍触发工具执行及 T2 |

这是确定性接口反例，证明 PullLoop 自身未实现其承诺；不声称发生过真实生产副作用。Router/Tools 适配器也应响应 ctx，但不能用未来适配器替代当前 loop 的暂停契约。建议由主管要求补齐取消分流与这些路径的回归断言，并明确批中断/批完成与取消竞争的边界。本流未修代码。

### F2，建议（P2，需主管定边界）：Router probe 成本绕过止损

锚点：`loop.go:169` 命中后成本累计直接 finish，`:266` 的 ProbeExhausted 只在模型工具批路径执行。有效只读观测：入场 ProbeCost=2、MaxProbeCost=1，Router 返回 ProbeCost=2，结果为 `fastpath`、ProbeSpent=4。不是 budget_exhausted。

设计 §3.1 明确快路径直接短路，同时 §3.4 要求超限独立预算分类，两者对快路径成本例外未定死。本报告不把“缺少执行前限额”整体判为 A 失败：原步骤本来就在批后检查；但 Router 连批后分类也没有。请主管明确 Router 是否预算豁免、已超限入场能否继续执行，以及超限后的终态归属，再交 C/D 接线落实。此项不新增本流最终裁定。

### F3，建议（P2）：nil Prefix 缺省实例无法跨 turn 比对

锚点：`loop.go:286` 每次 assemble 在 Prefix=nil 时新建 PrefixService；`promptruntime/prefix_service.go:145` 定义服务按 SessionKey 保存上一轮快照。当前测试 InjectionDefaults 只跑一个模型轮，AssemblyConsumesPrefixService 用 fake，未验证 nil 缺省的跨轮断裂/缓存异常。B/D 应复用同一实例或禁止 nil，不得把当前 Trace 的 breaks=0 当作 P1/P2 成功证据。当前冷启动槽为空，未据此否定预留槽位。

未发现源码编译或原测试失败 blocker；建议返工由 F1 的已证契约违约支撑。F2/F3 为主管应明确或后续接线应核对事项，未擅自要求跨域修复。

## 5. 原卡验收覆盖与七步落地

| 原卡要求 | 复核结果 |
|---|---|
| 六注入 / 七步循环 | 字段与①至⑦齐；冷启动槽空符合 A/B 分工，Router nil-safe 符合 C/D 分工 |
| T1 每工具批一次、TurnID 序列 | 原测试通过；只证明边界报告输入/调用，未证明模型动态窗实际出窗 |
| T2 正常终态一次、暂停零触发 | 正常无工具、Router、溢出、LLM 报错、工具预算路径通过；暂停“所有取消路径零 T2”被 F1 反例推翻 |
| 预算独立分类 | 工具路径成立，预算常量与其他失败类区分；Router 及执行前边界如 F2 所列 |
| flag 四态 / BOM | 原 5 测试通过；配置写入在 t.TempDir 内，不触及真实配置 |
| 溢出 fail-closed | 提供显式 overflow marker 时零 LLM、零工具；静态 ContextSnapshotJSON 不测不断增长的实际 assembly/history，D 须建立真实预检来源 |
| build / 全量 / gofmt | build 0，全量 0 FAIL，提交六文件 gofmt 相等；工作副本 CRLF 列名已单列解释 |
| 冻结接口零改动 / 零生产消费 | 以实际提交 diff、逐 blob 比较和全仓生产源码搜索证实；不是仅以“tracked diff 空”替代已提交差异核查 |

链路进度：骨架可构建、19 原测试通过、91 包全量通过；取消扩展诊断失败。最终验收：未裁定，交主管；原 A 生产接线、G3 真栈成功与稳定性均未由本卡证明。

## 6. B/D 接线前必核项

1. 先处置 F1 的取消问题；统一 Router、模型、工具阶段的中断分类，明确中断批如何留在 continuation，避免取消触发 T2。
2. B 消费真实 ExitReport，落实 retain/ref/drop 到模型窗及持久化；当前 ToolResult 不携 RetainedStatement，T2 累计工具单元 ID 不匹配 run 终局，不能沿用 fake 的次数断言宣称真实 retain。确认 invariant violation 与写入失败显式可见。
3. OQ-H1 的 continuation 必须承接循环号、probe spent、Context/窗口与已执行批，验证同 RunID 恢复不重用 cycle ID、不重复执行/入账；当前 Conversation 载体只证明可携文本。
4. D 必须注入真实工具协议解析与 ClassifyTerminal，区分无调用成功、no_candidate_found、capability_blocked、预算与基础设施失败；不把 nil classifier 的 judgment_ok 当质量指标。
5. 明确 F2 Router 预算例外、成本记账与批前/批后执法口径；对齐 C 的独立 fastpath 分类及 diagnostic 旁路。明确 Router 每循环拿原 GoalInput 的行为是否满足真实运行状态。
6. B/D 使用有状态 PrefixService，冷启动事实段会话首装一次、稳定四层字节恒定；记录完整 AssemblyReport 的前缀指纹/断裂原因/异常，当前 Trace 只有 breaks 数量，不能满足 G3 指标三层。
7. D 基于实际装配及历史更新溢出预检，模式配置只在启动读一次并显示非法来源；真实工具只传安全 LLMContext/摘要，不展开 raw package，不自动选择 CCB view。
8. G3 按既有 smoke 体系完成真实栈及旅程双模式验收、预写重复轮数与失败分类。本卡零真实栈，未新增任何烟测豁免，也不新增用户手测义务。

## 7. 交付范围

本流新交付仅此报告与本目录证据；本卡状态变更同步共享 main。A 的集成提交只保留在本地复验分支，报告发布分支仅承载本复核目录，供主管审阅/cherry-pick。实现、原 A/B/C 卡、docs、rulings、资源记录均未修改。报告提交 hash 与发布分支回填本复核卡回执，避免报告自引用 hash。
