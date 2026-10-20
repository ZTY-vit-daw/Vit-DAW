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
- 验收：（裁定文件 / 验收 commit）
