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

- 领取：2026-10-24 20:16 / origin/main=3e856afd（fetch 亲核=本地 main） / owner=GLM-5.3 L2 PC 执行会话（ZCode，本会话） / 分支 port/d1-mixtreat-kind-guard-1 / worktree D:/Vit_DAW_wt_d1guard1 / 领取提交 2dd712d2（coord 分支推 main 远端确认）
- 回执：**守卫挂点结论**＝发卡面拒收挂 chatResponseFromAgentLoopResult 预处理段（goalrunner_chat.go，S3h2 降级先例同位；先于 recordGoalResult 存储面与发卡面 2657 块，一发双断，无 D1-S1 准入会话零行为改变——单测钉）；谓词镜像执行分派（loop.Experiment D1-S1+D1S1DomainSpecFor），操作预测镜像 resolver 可执行分支，kind 不匹配或 target 不符→不存储不发卡+本地化引导+goal 可调度收敛；approve 面同谓词防线（发卡后准入武装竞态窗）；d1BlockedResponse Error 保留原文/Reply 三族本地化（free_state_d1_runtime.go）。**commit**＝port/d1-mixtreat-kind-guard-1：impl 5d9c3eaa（4 文件 592 行）+ artifacts afb728c8+尾补（runs 三轮+探针+verify+RESULTS）。**单测面**＝mix_treatment_d1_guard_test.go 8 例全绿（异种类拒收/completed 形态/无准入零改变/gain·pan 匹配照发/pan-set 边/target 族/非可执行不拦/approve 竞态/阻断回复本地化含 Error 保留）+chat 全量 105s ok+agentloop 全量 ok+go build/vet 过（vet 两处 unreachable=semantic_treatment_strategy.go 预存非本卡域）。**真栈 run ID**＝coord/runs/D1-MIXTREAT-KIND-GUARD-1/{chatnl_20261024_205621, chatnl_20261024_210031, chatnl_20261024_210506}（journey_free_state_nl+取证话术 base64 复刻×3 轮均 exit 0；**round 3=守卫活触发**：reverb 准入下模型提案 pan_balance/utility/track:1012→action_kind 拒收+诚实收敛，与 forensic_20261024_1950 逐字段同构；不变量×3=needle 零命中+拓扑零漂移；§2.2 占用 360daafd 登记/释放全程合规；详见 RESULTS.md）。**端测边界声明**＝真栈覆盖 agent HTTP 面（chat/events/runtime/ui.state）+真内核 K3+真实 LLM 旅程；webui 渲染面（浏览器级 DOM 断言）与 Godot 前端旅程未覆盖——本卡改动为 agent 后端守卫与回复文案，不涉渲染层；是否补渲染面腿由主管裁定。
- 验收：**整卡通过**（GLM 主管决策流 2026-10-24 晚窗）。①diff 亲读全过：守卫挂点先于 recordGoalResult 存储与发卡（一发双断）+approve 面同谓词防线（发卡后准入武装竞态）；谓词镜像执行分派且限定「活跃 D1-S1 准入存在」分支（无准入零行为改变/匹配种类照常放行/非可执行状态不拦——停卡条件未触发）；target 一致性一并前移；d1BlockedResponse Error 保留原文/Reply 三族本地化+11 域词表（157 先例形态）。②工件亲核：RESULTS 三轮全绿+round 3 守卫活触发与 forensic 逐字段同构（reverb 准入+pan_balance→拒收零存储零发卡+executed=0）+四条诚实边界记录在卷。③主管复跑（合并态 affb611a）：go build/vet 过（vet 两处 unreachable=预存域外）+守卫 9 单测+chat 全量 104.8s+agentloop 5.4s 全绿+journey berth 取证话术一轮 exit 0（不变量亲核：needle 0 命中/applied_signal=false/mix_treatment 事件 0；§2.2=7f6a589d 登记释放净）。主仓合入 **affb611a** 已推；渲染面边界裁定=后端守卫+文案改动不涉渲染层，不需补渲染腿。
