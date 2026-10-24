# D1-MIXTREAT-KIND-GUARD-1：D1-S1 活跃期异种类治疗守卫+阻断回复面修复

- 发卡：GLM 主管决策流 / 2026-10-24 晚窗（依据=用户手测现场取证 coord/runs/D1-MIXTREAT-KIND-GUARD-1/forensic_20261024_1950/FORENSICS.md）
- 派发确认：已确认（主管发卡；主旅程标准话术死路=用户可感缺陷）
- 验收负责人：GLM 主管决策流
- 池序 184；目标仓库=D:\Vit_DAW（agent/ Go）；P2（主旅程话术「给人声加点空间感」死路+内部错误直漏用户面）/ 0.5 天 / 依赖=无；模型分级：L2 / PC 执行会话
- 并行：与 180（前端仓）不同仓可并行；真栈腿按 §2.2 与在飞卡错峰。

## 现象（取证已固化）

用户以标准话术「给人声加点空间感」开 chat 会话：D1-S1 准入正确武装（reverb/reverb_decay_adjust/decay_pct=5/target=vocals 1032），但模型第 4 轮提案 pan_balance（utility）治疗 → 边界层无种类校验照存 pending+发交互卡（ready_pan_tick）→ 用户 approve → 执行口才发现 candidate.Operation=track_pan_adjust ≠ spec.ActionKind=reverb_decay_adjust → fail-closed 拦截（正确）但 d1BlockedResponse 把内部英文 reason 直接作为 ChatReply（"D1-S1 requires one bounded reverb_decay_adjust"）。零 mutation，工程无损。

## 范围

①**存储/发卡前守卫**（主修）：pending mix treatment 存储（goalrunner_chat.go:3179 邻域）或交互卡发布前，若会话存在活跃 D1-S1 准入且治疗 action_kind ≠ 准入 spec.ActionKind——不发布可执行确认卡；回复改为诚实说明（本地化：当前实验轮限定 <准入域> 剂量动作，声像/电平类调整不在本轮剂量内）+引导（等实验轮收口或撤销实验另开）。同族 target 一致性门若同点可检则一并前移（复用 free_state_gate_target_consistency_test 先例）。②**阻断回复面**（必改）：d1BlockedResponse（free_state_d1_runtime.go:790）Reply 不得直漏内部 reason——结构化 Error 保留原文，Reply 用本地化可行动文案（157「invalid_referral 边界面」先例同款形态）。③探针：单测钉两类形态（异种类治疗不产生可执行卡+阻断回复本地化）+既有 chat/agentloop 包零回退（177 六族+free_state_gate_target_consistency 等）。④真栈 chat 腿：复现话术旅程收敛（要么可执行 reverb 剂量卡、要么诚实引导卡），断言=无内部错误文本漏出（needle 扫描英文错误签名）+零非法 mutation。

## 文件域

以勘察固定（预期 agent/internal/chat/{goalrunner_chat.go 存储邻域, mix_treatment_confirmation.go, mix_tick_confirmation.go 分派邻域, free_state_d1_runtime.go 回复面} + 对应 _test）。越域即停。

## 验收标准

新增单测绿+chat/agentloop 全量零回退+真栈 chat 腿 exit 0（取证话术复现+英文错误签名零出现+拓扑读回零非法变更）；工件入 coord/runs/D1-MIXTREAT-KIND-GUARD-1/（取证件已在）；§2.2 登记。

## 停止条件

勘察发现守卫前移会破坏既有 mix tick 确认语义（如无准入会话的正常 pan/gain 卡）→ 锚点上交再裁（守卫必须限定「活跃 D1-S1 准入存在」分支）。

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（守卫挂点结论 / commit / 单测面 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
