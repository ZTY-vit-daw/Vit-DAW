# M3-CONTINUATION-PARK-STORM-1：continuation.park 空转风暴修复（无退避循环）+settle 记账面勘察

- 发卡：GLM 主管决策流 / 2026-11-04（依据=247 验收裁定 [2026-11-04-M3-AUTOMATION-GUARD-ADMIT-1-pass.md](../../rulings/2026-11-04-M3-AUTOMATION-GUARD-ADMIT-1-pass.md) r3 再分类+245 裁定挂账并入）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 248；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=247 已在 main（7b06c019）✓；L1 / PC 执行会话；栈空闲

## 标的（247 工件亲证）

guardrun1 r3：模型面直调完成后（22:38 telemetry 亲证），agent 陷入 continuation.park 空转风暴——agent_last_full_tail.log 22:40:59 起 729 条/约 2 分钟（≈6 条/秒「answerable park holds the chain non_terminal=1 skipped_status=4」），期间 HTTP 轮询遭 WinError 10061 拒绝（疑 park 循环打满 listener backlog），无 g1_r3.json/summary→ps1 exit 1。r2 settle 停 needs_confirmation 后 continuation parked——风暴疑与 settle/park 生命周期相关。**park 循环无退避**是直接观察。

## 目标

1. **定位+修复 park 空转**：勘察 park 循环代码路径（settle 后 continuation parked 状态的 answerability 轮询），修复=加退避（同类 scheduler 循环惯例：指数/固定间隔+上限，具体值执行侧按现有惯例定）+如适用加空闲上限终止条件；修复不改变 park 语义（answerable 时仍须及时恢复——退避不得引入可感延迟，上限值给依据）。
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

- （执行侧回填：会话/worktree/分支/领取基线 HEAD+status+diff --stat/栈登记/**勘察定位后的文件清单**）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
