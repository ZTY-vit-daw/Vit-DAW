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
