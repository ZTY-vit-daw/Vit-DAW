# L1-5 取消修复与适配提案核查（辅助决策侧）

- 日期：2026-10-09（Asia/Shanghai）；用户交回两卡要求检查，沿既定辅助决策分工核查。
- 共享卡池已 fast-forward 至 7f421cba；源码已有 XML/history_refs.go 改动保留。本文不出最终 ruling、不合入 A/C/修复、不解锁 B/D。

## 核查结论

| 交付 | 建议 | 理由与边界 |
|---|---|---|
| L1-5-IMPL-A-CANCEL-FIX-1，47ce5d75 | 建议主管通过本窄范围修复 | 五文件在域，原接口/预算/模式/既有测试无改动；六种同步取消反例修前红、修后绿；已返回事实与成本保留，无新增取消批 T1/T2。未发现本卡范围内阻断问题。 |
| L1-5-ADAPTER-DESIGN-1，26ba7edc | 建议主管接受提案取证，采纳 S1 方向；六项设计裁决后才能定版 | 两文件在域，状态矩阵、三方案、签名草案、生命周期 owner、成本恢复边界与后续验收表齐全；真实锚点可复查。交付完成不等于接口规格已冻结或 A+C 集成通过。 |

## 我方复跑与原始证据

使用本会话已附属的独立 worktree `C:/Users/timoz/.codex/worktrees/harness-review-evidence/Vit_DAW`，切至提交 47ce5d75 原代码测试；未复用实现作者 worktree，未修改源码。上一轮本会话生成的未跟踪工件与目标提交路径冲突，先完整移至同 runs 下 `L1-5-REVIEW-AUDIT-20261009-local-preserved` 保留，再切换；主树工件已有完整副本。没有丢弃或覆盖未知改动。

新工件目录：`coord/runs/L1-5-CANCEL-ADAPTER-AUDIT-20261009/`。独立测试后拷回主树保全，未覆盖执行侧红绿日志。

| 检查 | 实际结果 |
|---|---|
| 修复提交生产 diff 亲读 | loop.go +47/-7；新增 loop_cancel_test.go；其余三文件为本卡证据。原 loop_test.go 和冻结三包零 diff，ToolExecutor/FastPathRouter/GoalInput/Result 未改。 |
| 修前 overlay（只把 loop.go 映回原 A blob） | original-loop.go.txt 的 Git hash=939b3b354af44721ac2f722ba5803901d590636c，与 de1d876c:loop.go 相等。新取消测试 exit 1，六子路径及此前批次回归为断言级失败，不是编译失败；red.txt。 |
| go test ./internal/pullharness -count=1 -v | exit 0，21 个顶层测试（原19+新增2，含六种取消子路径）；green.txt。 |
| 上一轮独立三取消反例 overlay | exit 0：取消入场 Router=0，模型返回取消后 Execute=0，工具取消不记完成轮次且边界=0；original-counterexamples.txt。 |
| 提案 inspect.go 复跑 | exit 0，41 声明、13状态、108生命周期调用点；anchors.json。工具仅定位和列举，不证明新方案运行正确。 |
| 构建与全量 Go | 本轮未重复运行；执行侧 validation.txt 内 BUILD_EXIT_CODE/FULL_TEST_EXIT_CODE 均0，完整包输出无 FAIL；被测底座+修复 diff 已留档，实际提交 diff 相符。 |

命令：红相 `go test '-overlay=../coord/runs/L1-5-CANCEL-ADAPTER-AUDIT-20261009/red-overlay.json' ./internal/pullharness -run 'TestPullLoopCancellation' -count=1 -v`；绿相为上表包级命令；旧反例 `go test '-overlay=../coord/runs/L1-5-CANCEL-ADAPTER-AUDIT-20261009/probe-overlay.json' ./internal/pullharness -run '^TestReview' -count=1 -v`。提案工具 `go run ./coord/runs/L1-5-CANCEL-ADAPTER-AUDIT-20261009/inspect.go`。红绿退出码与被测 HEAD 在 results.json。

## 取消修复评估

- 入场、Route 返回、Prefix 返回、Complete 返回、Plan 返回、Execute 返回和 T1 返回均检查取消；finish 入口也检查，避免分类回调内取消后触发 T2。
- 模型返回事实保存后再中断；工具结果的 ModelLine 进入 Conversation，成本累计，ID/Tool/Status/单笔成本进入 Trace。取消批不计 Cycles，不发 T1/T2；此前已完成批次不撤销、不退款。修复符合卡面定义，并不声称能撤销已执行动作或消除所有外部竞态。
- 新增正式回归覆盖六路径及“首批正常、第二批取消”；额外 Prefix/分类/T1 回调内取消检查由代码审读核查，未据此声称全部竞态穷尽。
- 已超限 Router 成本仍能旁路预算，上一轮观察用例依旧可见；这是卡面明确未修的契约待定项，不可将其写为本修复已解决。nil Prefix 跨轮实例、真实退出消费与 continuation 恢复也未在本卡解决。
- 原 A 仍未最终验收。主管如采信，按原 A de1d876c → 修复47ce5d75 顺序合入；231a4ff3 是原 A 本地副本，不重复合入。

## 适配提案评估与下一关口

S1 的“宿主持有单一 state、pull 驱动循环”推荐成立：旧十个 handler 依赖私有 state/Runner，miss 会更新观察与会话，暂停与完成均可 stopped=true，无法把当前布尔协议直接包装成 hit。提案尤其正确区分 runtime EndTurn 与 run 终局 T2，并识别 clip split 在 handler 返回前补写 Continuation.PendingToolQueue 的事实。

关键锚点亲核：C runner.go:704-787 的 complete/fail/pause/result 已在返回前调用 Runtime.Complete/SetStatus/EndTurn/retain；C message_loop.go:1199-1258 的拆分确认队列补写；因此提案要求 pull-only draft 返回策略有依据。草案改变的是生命周期提交方式，涉及 runner 生产共享面，不能以新增包或零行为改变验收。

主管需定版的六组事项（提案 §8 已列明）：

1. S1 及 C 阶段边界：宿主执行体保留、三副本、配置漂移 panic 的接受范围。
2. A 占位协议解冻及 B 配合顺序；原 B 填 loop 槽与接口准备卡重叠，不要在未定契约前并发领取。
3. pull-only draft/Return 的唯一终局 owner；暂停仍单次 EndTurn、零 T2；明确用户 cancel 与 ctx 中断的差异。
4. run预算与slice额度分开、Router计量不重复、可靠 probe 成本来源/单位及 unknown 分类；不能用缺字段填0。
5. 第一版恢复能力边界与持久化字段：同进程/跨进程、旧状态缺省push、新未知枚举fail-closed、已执行unknown apply 的回读与停止策略。内存 DraftID 不能冒充持久化恢复。
6. 结构化质量 Outcome 与遥测，completed不直接判断judgment_ok；chat 迁移仍在 G3 后。

提案本身明确上述未定，不构成交付缺失。建议主管先选择最小可验收的 v1 边界，再从接口准备/状态适配两小卡开始；不要一张 D 同时承接全部接口、恢复、执行计量、终局重构和真栈 G3。暂停确认是现役行为，不能以缩减范围而静默丢弃。

这轮没有真实栈、渲染面或用户手测；只证明取消骨架修复与提案取证。终审、规格更新及后续卡授权继续由主管负责。
