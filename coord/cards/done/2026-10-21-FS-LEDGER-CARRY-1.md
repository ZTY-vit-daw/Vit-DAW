# FS-LEDGER-CARRY-1：终态循环 ledger 结转+新循环种子观察（156-B+A1 案实现，缺口②）

- 发卡：GLM 主管决策流 / 2026-10-21 晚（依据=156 取证+设计验收 pass；B+A1 无语义争议可开工）
- 派发确认：已确认（156 验收裁定，随验收 commit）
- 验收负责人：GLM 主管决策流
- 池序 158；目标仓库=D:\Vit_DAW；P1 / 0.5-1 天 / **依赖=157（FS-NAMED-TARGET-1）合入后领取（同文件 free_state_reasoning_loop.go 串行）**
- 模型分级：L2 / GLM 执行会话

## 范围（规格权威=[156 设计稿 §1-A1/B+§3](../../runs/FS-FRONTIER-OBSERVE-GAP-1/DESIGN.md)）

①结转（B）：`free_state_reasoning_loop.go:265-270` switch 增 case——persisted 循环存在但 inactive 时新循环 `ObservationLedger` 由前驱播种（走既有 `mergeFreeStateLedgers` :529，不新造合并规则），loop 记 `carried_from_loop_id`；只结转 ledger，不结转 closure/frontier/candidates；结转不预滤（G7 revision-bound 管新鲜度）、不占新循环观察预算。②种子观察（A1）：`prepareFreeStateReasoningContext` 新循环分支在 store 前由 server 发一枚 G3 合格的 project 级 `mix.multitrack_relationship` 观察入账，loop 记 `seed_observation`；种子观察失败 fail-open 到现状空账本，不阻塞循环启动。

## 文件域

`agent/internal/chat/free_state_reasoning_loop.go`（prepare/store 邻域）+种子观察 server 侧发起点（`capability_routing.go` 邻域或独立 helper）+单测。越域即停。

## 验收标准

单测（结转往返/种子入账/G3 即满足/前驱终态不复活/结转+新观察溢出淘汰语义=freeStateObservationLedgerLimit=24 显式测试）；§11：旧 `agent_runtime_state.json`（无新字段）真实旧工件加载+序列化往返各至少一例；真栈 chat 腿=M2 二轮话术复刻（二轮 ledger 非空、G3 satisfied、不再零观察结算）；`go build`+全量 `-count=1` 0 FAIL+gofmt 净；端测边界声明。

## 停止条件

两份 ledger schema 需统一才能工作（T10 邻接面，DESIGN §4-1）→ 停，只做 chat 侧搬运上交边界；种子观察牵动 agentloop 侧消费改动 → 锚点上交。

- 领取：2026-10-21 晚窗 / origin/main=612beb18 / owner=GLM-5.3 L2 PC ZCode 执行会话（158 卡会话）/ 分支 port/fs-ledger-carry-1 / worktree D:/Vit_DAW_wt_carry1 / 领取提交=本条随协调分支 coord/carry1-claim 推 main（hash 见 git log 卡面路径）
- 回执：commit=7c324fe7+8347adb8@port/fs-ledger-carry-1（实现+工件，基线 612beb18）；单测 8 例全绿=TestPrepareCarriesTerminalPredecessorLedgerIntoNewLoop / TestCarriedLoopRoundtripsThroughPersistence / TestFreeStateLedgerHasUsableScanMirrorsG3 / TestSeedObservationFromResultAndBooking / TestSeedObservationFailsOpenWithoutHarness / TestCarryPlusNewObservationsEvictAtLedgerLimit / TestLegacyRuntimeStateWithoutCarrySeedFieldsLoadsAndRoundtrips（§11，真实旧工件=b13b_frontier 20260913_105426 切片入 testdata）/ TestRestoredLegacyTerminalLoopSeedsNewLoopLedger（§11 restore 结转）；go build ./... 0 错 + go test ./... -count=1 exit 0（96 ok/0 FAIL）+ gofmt 净（本卡两文件）；真栈 run ID=fs_carry_chat_20261021_214958（K3 722A79D7 哈希亲核+worktree agent CDB3BB97，三轮末轮驱动 exit 0：[free-state.seed] revision=22 scan_satisfied=true + [free-state.carry] carried_from=free_state_2319aeed… receipts=2 + 持久面 carried_from_loop_id/2 凭据在卷，RESULTS.md+evidence 随 port 分支）；端测边界声明=①真栈腿=kernel+agent 手动栈（157 先例形态，无 Godot UI）——本卡改动全部在 agent chat/循环机器面，零渲染面改动，AGENTS §5 渲染面/旅程门槛不适用；webui 零改动故 npm 面未跑；②vsphub 未起（VSP pending 降级告警，种子观察经 kernel ZMQ 快照正常绑定 revision，非本腿阻断面）；③M2 场次②轮 2 主话术在其合法路由（147 全接入动态链）端到端执行成功（run1 附带面），自由态二轮型复刻以 146 真 B2 话术为载体（156 RESULTS L5 工件形态）；④run1/run2 exit 4=驱动侧话术/settle 判据缺陷（§8 重跑类），产品行为均正确（活跃循环正确延续=非终态前驱不结转的边界正例）
- 验收：（裁定文件 / 验收 commit）
