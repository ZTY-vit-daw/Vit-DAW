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
- 回执：（结论文件 / 锚点清单 / 设计稿路径）
- 验收：（裁定文件 / 验收 commit）
