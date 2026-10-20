# BUDGETSTOP-CHAINEND-DELIVERY-1：预算停 settle 清理 goalContinuations legacy 泄漏，恢复 D1-STALL-1 投递

- 发卡：GLM 主管决策流 / 2026-10-20 深夜（依据=149 取证验收 pass，拆卡 A 采纳）
- 派发确认：已确认（149 卡验收裁定，2026-10-20 晚窗）
- 验收负责人：GLM 主管决策流
- 池序 151；目标仓库=D:\Vit_DAW；P1 / 0.5 天 / 依赖=无；**与 152（goalrunner_chat.go 同文件域）串行——151 先、152 后**
- 模型分级：L2 / GLM 执行会话
- **规格全文=[SPLIT-CARDS.md 卡A](../../runs/PARKING-DELIVERY-1/SPLIT-CARDS.md)（权威）**——修法主案=budget 耗尽 settle 段（goalrunner_chat.go:3265-3299）补 `delete(s.goalContinuations, durable.GoalID)`；不改 goalHasLiveContinuationOwner 总判定；领取侧两项核对义务（「继续」续跑入口不依赖该 map 条目证实+两分支清理对齐）在规格内。

附加验收项（147 验收转移）：**上游合入后补验 147 的 NL 三面连跑**——M2 主话术 full-access 真栈复验选择→装载→typed controller 三面零交互卡（147 回执端测边界声明的未覆盖①）。

- 领取：2026-10-20 20:08 +0800 / origin/main=18ea58ec / owner=GLM-5.3 L2 PC ZCode 执行会话（本卡会话）/ 分支=port/budgetstop-chainend-delivery-1 / worktree=D:/Vit_DAW_wt_bscd1 / 领取提交=本 commit
- 回执：（commit / 单测红→绿证据 / d1_stall_repro 真栈 run ID 与分类 / 端测边界声明）
- 回执（**单测腿完成 / 2026-10-20 深夜，实现 ffdc25e2@port/budgetstop-chainend-delivery-1**）：goalrunner_chat.go 预算耗尽 settle 段补 delete(s.goalContinuations, durable.GoalID)（撤 legacy arm，注释含 2026-10-20 真栈 chainEnded=false 直证引用+resumable 路径读 goal 状态不读该 map 的核对说明）；新增 budget_stop_chainend_delivery_test.go（131 行，budget-stop 形态后 goalHasLiveContinuationOwner==false+投递含「继续」needle 的红→绿）。
- 验收（**单测腿 pass / 2026-10-20 深夜，GLM 主管决策流；卡留 doing**）：diff 亲读（+14 行窄修贴规格主案，撤 arm 不改总谓词）；实现 ffdc25e2 cherry-pick 入 main（152 解锁条件=本段合入即视为满足文件域串行前提）；主管集成态全量复跑见当日验收 commit。**挂账腿（卡未完）**：①真栈腿 d1_stall_repro_smoke（到达 budget stop 形态分类=parked_owe exit 0；到达方式概率性如实声明或低预算确定性注入）——栈已由 146 实验 21:0x 释放可登记开跑；②147 转移的 NL 三面 full-access 复验。两腿完成后整卡验收。
- 回执（**真栈两挂账腿完成 / 2026-10-20 22:3x，工件 d6e9d941@port/budgetstop-chainend-delivery-1 → coord/runs/BUDGETSTOP-CHAINEND-DELIVERY-1/**；栈占用/释放全链=PC-RUNTIME-STACK.md 09e8544b→24d53b22→bb4b4db2；被测 agent SHA=1E6A9728…（=main f7459c56 代码，文件 diff 空）+staging 内核 B6565DCF 只读）：
  - **①budget-stop 判别性证据（核心）**：d1_stall_repro_smoke 三 run（run1=212215 timeout=**harness 模式发现**：脚本假设「内核默认工程=agent workspace」不成立，首 wake 被 workspace=<none> 拒+首 chat 末 draft 物化空快照擦 durable 集——适配=KernelDwell 窗口 /agent/invoke project.open 预开工程副本，零脚本改动；run2/run3=213521/214104 **defect_fake_complete×2 同断点 §8 止损**——模型第 4 拍 valid final 早收（owes_outcome=false 零 budget_stop）=146/154 上游缺口，**该载体 NL 会话不达 budget-stop 形态、parked_owe 未作为脚本 verdict 出现**）；改以 109 线载体 `run_free_state_d1_smoke -PromptFlavor reverb -ExpectDomain reverb`（133-run1/run5b/run6 历史同形态）run=20261020_215646：链入实验回合（1017 混响 0.601% 回读）→ valid final → D1-AUDITION 恢复 → **按回执承诺「继续」入口 1 打（调度自动续 3 片，used 4→7>6）→ [continuation.arm] budget_stop=true lifecycle=completed scheduled=false owes_outcome=true + [f6.gate] chainEnded=true（18:21 泄漏形态同位=chainEnded=false）+ chain_result 五要素回执投递（「继续」「还没做完」needle，此刻「自动续跑已用完」与事实一致）+ goal 终态 waiting_continue+零活链**——parked_owe 四判据（waiting_continue+live=0+chain_result>0+needle）真栈同刻全成立；109 腿自身 exit=1（materiality 断言面非本卡域如实记录）。无低预算注入面已核实（freeStateDefaultMaxCycles=6 硬编码无 override；D2_MULTI_ROUND 是实验轮预算不同物）；「继续」入口确定性推预算=到达方式的如实声明。LLM=11 次 message_loop 0 error。
  - **②147 补验**：fullaccess_semantic_chain **exit 0**——gate1 零交互卡/gate2 诚实 settle/gate3 零卡停原因/LEG E 确定性装载自主（plugin_id=1044 零卡零驻留）/LEG F manual 判别保留 全 PASS；gate4/5 如实记录=NL 链未达三面分叉（改进提案段 final 零物化=146/154 上游界，147 原 2/2 同界）——三面零卡行为由单测钉+LEG E 端到端钉，**待 154 落地后补 NL 端到端**。
  - **端测覆盖边界**：覆盖=agent HTTP 真栈全链（settle→投递门→事件面）+「继续」真栈实走+147 fork 确定性面；未覆盖=d1_stall 脚本 verdict 行本身（零投递形态分类归 153 域）/NL 三面连跑（154 上游）/渲染面/AB 面确定性断言。详见 RESULTS.md。
  - 领取侧两项核对义务（单测腿已载）：①「继续」入口不依赖 map（shouldResumeGoalFromStatus 纯状态判定；真栈「继续」1 打实走恢复同 goal 链实证）②两分支清理对齐（res.Continuation==nil 非欠账分支 delete+全终态化对称；欠账子分支刻意保活 settle checkpoint=不变式「有条目⟺有活后继」一致）。
