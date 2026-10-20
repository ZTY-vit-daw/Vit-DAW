# BUDGETSTOP-CHAINEND-DELIVERY-1 真栈腿结果（151，2026-10-20 晚窗）

- owner=GLM-5.3 L2 PC ZCode 执行会话；worktree D:/Vit_DAW_wt_bscd1 @ port/budgetstop-chainend-delivery-1
- 被测 agent=worktree 构建 SHA256=1E6A972836B17F40E7392C34EB5C6CB46C1DD6E04F73（=main f7459c56，文件 diff 空）；内核=主检出 staging B6565DCF…（只读，零内核改动）
- 栈占用/释放全链见 coord/resources/PC-RUNTIME-STACK.md（登记 09e8544b→24d53b22，释放 bb4b4db2）

## 判别性证据（本卡修复的真栈 delta）——d109_budgetstop/

到达路径：`run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb`（109 线载体，133-run1/run5b/run6 历史同形态）→ 链入实验回合（吉他轨 1017 混响衰减 0.601% 回读在卷）→ post-action 观察部分返回 → 模型 valid final 收束 → D1-AUDITION-GAP-1 恢复（[continuation.owed]，goal 还原 waiting_continue）→ **按回执承诺的「继续」入口打 1 次 bare 继续**（调度链自动续 3 片，会话级 ContinuationUsed 4→7 越过 budget=6）→ **budget-stop 形态**：

- `[continuation.arm] 22:06:55 … lifecycle=completed scheduled=false budget_stop=true owes_outcome=true`
- `[f6.gate] 22:06:55 chainEnded=true status=completed respGoal=waiting_continue runtimeGoal=waiting_continue`（**18:21 泄漏形态同位证据为 chainEnded=false**，FORENSICS agent_last.log:491）
- chain_result 事件送达（events_final.json）：五要素回执全文，含「继续」「还没做完」needle；此刻「自动续跑也已经用完」与事实一致（续跑预算真实耗尽）
- 终态：goal=waiting_continue（回执承诺的续跑入口在）、零活链（runtime_status_final.json）、task=improvement_proposal 非终态

即 d1_stall_repro_smoke 的 parked_owe 四判据（goal=waiting_continue && live==0 && chain_result>0 && body needle）在真栈同一时刻全部成立；109 腿自身 exit=1（其 "D1 requires one post-action CCB observation" 断言面非本卡域，如实记录）。LLM=deepseek-flash 11 次 message_loop 调用 0 error。

## d1_stall_repro_smoke 三 run（d1_stall_runs/）——载体边界如实记录

- run1=212215 timeout(exit2)：**harness 模式发现**——脚本假设「内核默认工程=agent workspace」在当前架构不成立：首片 arm 后 wake 被 workspace=<none> 拒 + 首 chat 结束 draft 物化用空快照替换 durable 集（activateProjectWorkspace→restoreProjectAgentRuntimeStateLocked），链死零事件。适配=KernelDwell 窗口经 /agent/invoke project.open 预开工程副本（零脚本改动；147 LEG A 同款面）。
- run2=213521 / run3=214104：**defect_fake_complete(exit1)×2 同断点，§8 止损**。签名=模型观察后第 4 拍 valid final 收束（arm 行 status=completed stop=done、owes_outcome=false、零 budget_stop、零物化）——146/147/154 域已知上游缺口（改进提案段 final 零物化），非本卡修复面。**该载体的 NL 会话在当前模型分支下不达 budget-stop 形态，parked_owe 未作为脚本 verdict 出现**；验收语义由上述 109 判别性证据覆盖。
- 无低预算确定性注入面（已核实）：freeStateDefaultMaxCycles=6 硬编码常量，无 env/context override；VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET 是实验轮预算，不同物。

## 147 转移补验（fasc_reverify/）——fullaccess_semantic_chain exit 0

`dev_agent_smoke -StartKernel -RestartAgent -Scenario fullaccess_semantic_chain -KernelExe <staging>`：gate1 零交互卡 PASS / gate2 settle_reason=goal_terminal goal_status=completed PASS / gate3 零卡停原因 PASS / LEG E 确定性装载自主（plugin_id=1044、零卡零驻留）PASS / LEG F manual 判别保留（plan_8838734801135dd6）PASS。gate4/5 如实记录（NL 链未达选择/装载/typed 分叉面、无 fresh-observation receipt）=改进提案段 final 零物化上游缺口（146/154 域，147 原 2/2 同界）——三面零卡行为由单测钉（full_access_semantic_chain_test.go 7 件）+LEG E 端到端钉。

## 端测覆盖边界声明（照 109 卡先例）

- 覆盖：agent HTTP 面（chat/invoke/authority/runtime/events）真实栈；budget-stop settle→投递门→事件面全链（判别性证据）；「继续」入口真栈一次实走（guard 应答+调度链续跑）；147 fork 的确定性装载/判别面端到端。
- 未覆盖：①d1_stall_repro_smoke 自身 verdict 行未输出 parked_owe（其 NL 会话不达形态，见上）——脚本的零投递形态分类（parked_owe_no_delivery）归 153 域；②NL 主话术三面连跑（选择→装载→typed controller）仍被 146/154 上游缺口挡（gate4 记录在卷），待 154 落地后补验；③渲染面/webui 用户旅程（本卡零渲染面改动）；④A/B 试听面保留为模型所有面未做确定性断言。
