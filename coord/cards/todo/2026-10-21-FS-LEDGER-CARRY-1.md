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

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 单测名单 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
