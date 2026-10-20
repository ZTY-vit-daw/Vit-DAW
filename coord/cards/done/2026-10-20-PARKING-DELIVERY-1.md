# PARKING-DELIVERY-1：停泊/A/B 交接在 final gate 驳回+预算耗尽时被静默吞掉（取证+设计）

- 发卡：GLM 主管决策流 / 2026-10-20 晚窗（依据=M2 体验场次①实录 F5：[2026-10-20-m2-experience-session-1.md](../../reports/2026-10-20-m2-experience-session-1.md)）
- 派发确认：已确认（用户 2026-10-20 晚窗「可以发卡」）
- 验收负责人：GLM 主管决策流
- 池序 149；目标仓库=D:\Vit_DAW；P2 / 取证 0.5 天（实现另拆卡）/ 依赖=无
- 模型分级：L2 / GLM 执行会话（continuation/预算面核心逻辑）

## 缺陷（场次实录实证）

M2 主旅程断在停泊/试听交付（清单 §3 步骤 4-6 未达用户）：执行完成后 turn6-9 连续 4 turn executed=0；18:21:34 与 18:21:45 **两次 free_state_final_gate 驳回**（agent_message_loop_debug.jsonl，全文在录）：

- 第一次：experiment_materiality 形状违规（state=""、attempt=0）——错误信息已含完整 shape 教学；
- 第二次：status=needs_action 携带 improvement_proposal（schema 要求 improvement_proposal|needs_experiment）；
- 两次驳回时模型的 reply 正文均为**完整合格的 A/B 试听交接文案**（「请在 DAW 里对 bass 轨做整体开关对比（旁通/启用这条轨的混响）…交由你的耳朵判断」）——被整条驳回、未达用户；
- 18:21:45 budget_stop=true / lifecycle=completed / **owes_outcome=true**，用户面零交付，静默 95 秒后用户主动发话。

109 线双 exit 0 验证轮与今晚**逐字同构话术+同模型档**（deepseek-flash 思考档）——验证面未覆盖该形状失败路径。

## 目标（取证+设计卡，本卡不改生产代码）

1. **归因**：final gate 驳回后为何无第三次修复机会——bounce/预算/门教学机制在哪一层断（第二次驳回后 budget_stop 的触发链；对照 FULLACCESS-AUTONOMY-1 的单 bounce 设计先例）；
2. **owed_outcome 交付义务**：预算耗尽+gate 驳回时用户面应交付什么（模型 reply 正文兜底 / 标准停泊文案 / 静默的理由）——owes_outcome=true 的消费面在哪，为何没消费；
3. **109 验证面盲区**：双 exit 0 断言面在事件/状态还是用户面文本，为何形状失败路径不可见；
4. 实现卡拆分建议（含文件域与验收面）交主管定稿。

## 文件域

取证只读（free_state final gate / continuation 预算面 / goalrunner 响应路由）；产出设计件入 `coord/runs/PARKING-DELIVERY-1/`。

## 验收标准

三问各有代码锚点+工件回指的归因结论+实现卡建议，交主管定稿后拆实现卡。

## 停止条件

- 发现 continuation 预算/budget_stop 行为已有更晚用户裁定在卷 → 引用裁定收窄范围，不重开已裁定面；
- 归因牵出 FS 状态机统一收敛（T10 答辩后任务）级别的结构性问题 → 上交边界，本卡只覆盖停泊交付面。

- 领取：2026-10-20 19:02 +0800 / origin/main=c08de366（用户提示基线 a958fc40 已被 146 回执与 150 领取推进；首推被 150 并发领取所拒，按 §2.1-3 从新 origin/main 重放重推，本卡全程无人竞争） / owner=GLM-5.3 L2 PC ZCode 执行会话（parking-delivery-1）/ 设计件分支 port/parking-delivery-1（待 cherry-pick）/ worktree D:/Vit_DAW_wt_parkdeli1 / 领取提交=协调分支 coord/parkdeli1-claim 直推 main
- 回执（2026-10-20 晚窗，GLM-5.3 L2 执行会话 PC/ZCode）：**三问归因闭环，零生产代码改动**。①bounce 通道无上限无保底、content-shape 驳回不进 TIMING-1 计数（`free_state_reasoning.go:699-713` 注释自认 never count；工件实为 **4 次**驳回非 2 次——18:20:59/18:21:09/18:21:34/18:21:45，全 Validate() 层、reply 全是合格 A/B 交接文案，jsonl 23751-23754），terminal lock 从未触发，bounce 活到 continuation 预算耗尽（`goalrunner_chat.go:3265-3299`；末片强化重试指令成死信）；对照 `full_access_autonomy.go:40-52` 单 bounce 先例缺「上限耗尽后原样投递」对称物。②owes_outcome 三消费面：goal 修正正常（3443-3458，goal 实停 waiting_continue）；**链尾投递门 D1-STALL-1 分支被挡死=根因**——`goalrunner_chat.go:3214` legacy arm 写入后预算停 settle（3282-3299）不清理→`goalHasLiveContinuationOwner` 把 completed 记录判活→`continuation_scheduler.go:1146,1205` chainEnded=false 静默 return；**直证=agent_last.log:491 `[f6.gate] chainEnded=false`**；应交付物=五要素欠账回执（`d1OwedEvaluationReply` 1342-1354，文案本就为预算耗尽撰写）未送达。③109 断言面在状态/记录（materiality/d1_receipt/停泊存活），预算耗尽形状=materiality missing exit 1（rerun2 同签名实证），用户面不在判据；停摆专项 `d1_stall_repro_smoke.ps1:223` 有 parked_owe_silent→exit 1 用户面文本断言但前提 chain_result>0 被本缺陷短路→落 timeout(exit 2 环境类)；单元层 `d1_stall_settle_test.go:156-167` 显式传 chainEnded=true 绕过谓词求值——三层全不可见。**拆卡建议 A（根修 P1）/B（烟测分类 P2）/C（裁定备忘）**：[SPLIT-CARDS.md](../../runs/PARKING-DELIVERY-1/SPLIT-CARDS.md)（A 文件域 goalrunner_chat.go 与 148④腿相交，立卡时主管排串行）。结论全文=[FORENSICS.md](../../runs/PARKING-DELIVERY-1/FORENSICS.md)；设计件=port/parking-delivery-1@3590f310（已推 origin 待 cherry-pick）。停止条件核对：无更晚 budget_stop 裁定在卷；未牵出 T10 级结构性问题。
- 验收：（裁定文件 / 验收 commit）
