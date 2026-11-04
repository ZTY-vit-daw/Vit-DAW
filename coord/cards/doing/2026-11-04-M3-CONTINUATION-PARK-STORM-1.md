# M3-CONTINUATION-PARK-STORM-1：continuation.park 空转风暴修复（无退避循环）+settle 记账面勘察

- 发卡：GLM 主管决策流 / 2026-11-04（依据=247 验收裁定 [2026-11-04-M3-AUTOMATION-GUARD-ADMIT-1-pass.md](../../rulings/2026-11-04-M3-AUTOMATION-GUARD-ADMIT-1-pass.md) r3 再分类+245 裁定挂账并入）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 248；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=247 已在 main（7b06c019）✓；L1 / PC 执行会话；栈空闲

## 标的（247 工件亲证+主管验收后代码定位 2026-11-04）

- **代码锚点（主管亲读）**：continuation_scheduler.go:150 `continuationSchedulerTick=250ms` 固定节奏 ticker（:758-775 循环）+ `s.schedulerWake` 通道**无节流并入**（每 wake 即时 drive，风暴实测 ≈6 次/秒>ticker 4 次/秒=wake 加成证据）；:963-975 park 日志分支无退避无节流（answerable park=有意形态每 tick 必打一行）；每 tick `drive()` 还执行 recoverContinuationWorkspace（workspace 身份检查+互斥锁）。注释自证历史：CONT-STALL-1 dead park 告警路径同函数。
- **风暴机制假说（供验证）**：parked 链存在期间 scheduler 全速空转（tick+wake）→每轮持 s.mu 做检查→HTTP handler 同锁竞争→listener backlog 拒绝（10061）。**wake 是否自激**（drive→状态变化→wake→drive）须判定。

guardrun1 r3：模型面直调完成后（22:38 telemetry 亲证），agent 陷入 continuation.park 空转风暴——agent_last_full_tail.log 22:40:59 起 729 条/约 2 分钟（≈6 条/秒「answerable park holds the chain non_terminal=1 skipped_status=4」），期间 HTTP 轮询遭 WinError 10061 拒绝（疑 park 循环打满 listener backlog），无 g1_r3.json/summary→ps1 exit 1。r2 settle 停 needs_confirmation 后 continuation parked——风暴疑与 settle/park 生命周期相关。**park 循环无退避**是直接观察。

## 目标

1. **修复 park 空转**（锚点已定）：①parked 无可领记录时 drive() 降频或 park 日志节流（如每 N 秒一行/状态变化才打）；②wake 通道自激判定+如有则去自激（drive 内不再触发 wake 的路径，或 wake 合并/最小间隔）；③保留 answerable 及时恢复（退避只作用于「无可领」分支，不作用于 claim 路径）。
2. **listener 拒绝面判定**：核实风暴期间 10061 是否 park 循环自身 HTTP 打满（或无关并发因）；修复后组 1 全组复验不复现即证。
3. **245 挂账勘察（并入）**：r2 终局 run 无 [continuation.arm] 行=recordGoalResult 未执行、durable=cancelled vs goal=completed 记账矛盾——定位 settle/记账分支缺口，产出锚点+修复方向（同域顺手修则修，跨域则只交锚点）。

## 判据与验收标准

- **单测**：park 退避/上限语义测试（含 answerable 及时恢复断言）+既有 continuation 测试零红；
- **真栈腿**：-Groups 1 全组复验——**三轮全过+ps1 exit 0（3/3 稳定 run，链收口前置件）**；park 日志零风暴（无连续高频 park 行）；组 1 判据维持 tool_chain_ok 口径；栈登记/释放全流程+TEARDOWN_ALL_ZERO。
- §8 预算 ≈¥1.5-2.5（组 1 直调路径单轮 ≈¥0.4 亲证）；计费披露。

## 停止条件

- 修复破坏 park 语义/既有测试红→回退两案上交；
- 全组复验仍风暴→取证包上交（含 park 行时间序列+listener 状态 dump），不原样重跑；
- 勘察发现 settle 记账缺口超出文件域→只交锚点上交；
- 越域即停。

## 文件域

park 循环与 settle 记账所在 chat/agentruntime 包文件（执行侧勘察定位后回填具体文件清单到领取栏；修复限退避/上限/记账分支类，不动治理/准入/执行器）。

## 领取

- 领取时间：2026-11-04（晚窗）/ 领取基线 origin/main=d00c159d（fetch 核对卡在 todo 无 owner，doing 仅 GEN-SUNO 卡无文件域冲突）
- owner：ZCode GLM-5.3-Flash L1 PC 执行会话（248 卡会话）/ 机器=PC / worktree=D:/Vit_DAW-coord-248 @ coord/m3-continuation-park-storm-1-claim（领取提交）；实现分支=port/m3-continuation-park-storm-1（worktree D:/Vit_DAW_wt_248impl）
- 领取时主检出 status：既有未提交改动属其他会话/历史（M3-CHAT-AUTH-STAMP 卡面、UX-AB-AUDITION-IMPL-1 工件、webui design 草稿、VitApp Workspace 运行态）——非本卡输入，本卡零触碰；本卡全部改动仅在独立 worktree。
- **勘察定位后的文件清单**（修复限退避/上限/记账分支类）：
  - `agent/internal/chat/continuation_scheduler.go`——①scheduler 循环 :756-775 无可领降频（idle backoff 只门控重型扫描=文件锁+全量 reload+整树恢复，workspace 恢复 recoverContinuationWorkspace 保持 tick 节奏不降频——保发现延迟）；②park 日志 :969-971 节流（镜像 :47-73 stall 节流形态，签名变更立即打）；③wake 自激判定=**不存在**（风暴窗口 agent_last.log+full_tail `continuation.wake`=0 条，风暴=纯 ticker 稳定 4/s；wake 生产调用点全 5 处均事件驱动一次性，drive→claim 失败→无状态变化→无 wake 回路），无需去自激代码。
  - `agent/internal/chat/server.go`——Server struct :113 邻接新增 park 日志节流状态字段（镜像 continuationStallLog* 三字段形态）。
  - `agent/internal/chat/goalrunner_chat.go`——245 记账面 observability：`clearGoalContinuation` :4094-4108 无痕取消（零日志零 goal 状态写）补一行取消日志+计数（durable=cancelled vs goal=completed 矛盾的头号无痕写入者候选）。
  - `agent/internal/chat/continuation_scheduler_test.go`——新增 park 节流/backoff/answerable 即时恢复测试。
  - 仅锚点不改动（勘察上交）：recordGoalResult 全仓唯一调用点=chatResponseFromAgentLoopResult(goalrunner_chat.go:2570)，GoalID 空即静默早退 :3271；goal=completed 写点=settleGoalAfterContinuationEnd→turn_close_guard.go:105；245 形态精确写入者需复验轮时间序列日志定案（carryrun1 原始 agent log 未入库，仅 telemetry）。
- 10061 归因（勘察结论，卡面目标 2 前置）：风暴窗口 10061 全部为 agent→vsphub:8787 出站 POST（VSP publish+hub registration），**vsphub 不在 dev_agent_smoke 栈内=既有 WARN 形态**（guardrun1 RECEIPT 亲证），与 park 循环无关；python 轮询腿 10061（agent HTTP 面）与风暴窗口重合，机制候选=每 tick 文件锁+全量 reload+mu 下整树恢复饿死 HTTP 面——修复后组 1 复验不复现即证。

## 回执

- （执行侧回填）

## 验收

- （主管回填）
