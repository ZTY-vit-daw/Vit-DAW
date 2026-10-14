# 2026-10-14 批量三裁定（112 / 113 双卡验收 + 109 rerun5 解锁）

> 验收人：GLM 主管决策流（ZCode 主会话）。触发：用户"任务卡已完成可以检查"。
> 方法：diff 亲读（含接线/锚点逐处核）+两分支独立亲跑+gofmt blob 核；main 起点=5a6d5c99。

## ① 卡 112 FIX-SHADOW-BINDING-IDENTITY-1 —— **pass，合入 26757b34**

- diff 亲读：preserveVSPIdentityFromShadow=同 uuid 守卫（stateUUID==lastUUID 才回填）+仅回填 lastKnown 非空值+revision 正整数解析守卫+跨工程/未绑定/无可解析值 fail-closed；**四处接线逐一核**（harness.go:1605 before 快照/:1683 observed/:1689 get_project_state 回退（直接调用形态）/:12855 refreshShadowWithStatus=run9 实锤路径）——与回执一致。
- 单测 5 钉（三形态+部分缺失子形态+跨工程护栏+日志标记断言）随 95 包亲跑通过。
- 读方冲突核查采信（回执停止条件段）：checkpoint 迟到不变式/change receipts uuid 比较器/confirm epoch 校验/materialize 伪通知减少——论证与 diff 一致。
- 覆盖边界=仅 VSP 快照通道，非 VSP legacy 通道按不扩域纪律未动（无 run9 形态实证）——采信。
- 端测边界=纯单测（卡面约定）；端到端效果归 109 rerun5，**复跑核验锚点=identity_preserved_on_incomplete_snapshot 日志+refresh status identity_preserved 键**。

## ② 卡 113 FIX-POSTACTION-GATE-FEEDBACK-1 —— **pass（域外 rules.txt 双源同步采认），合入 1f4e2919**

- diff 亲读三锚点：①弹回消息阶段感知（阶段外文案字节不变的后半钉设计好）；②freeStateNarrowedReObservationAdmitted=请求集⊆**最新覆盖** receipt 集+当前 revision **严格新于**（尾整数数值比较，不可排序 fail-closed 弹回）+子集永不以 new_view_set 放行+旧 receipt 不能解锁新 receipt 已录视图——与卡面硬约束逐条对上；③prompt 教学只换被弹形状为豁免可收形状，FIRST-action 与 settle/needs_action 禁令原样。
- **域外 rules.txt double-source 采认**：agent/rules/ruleset/resources/sections/neutral_family.discipline.rules.txt 是 ccb_model_prompt 模板的字节权威镜像（agentloop/ruleset_parity_test.go 双向字节锁亲核在场，生产 prompt 从 embed 渲染）——单改 .go 侧 parity 即红且生产面不生效，同步该文件是锚点 3 唯一完整实现路径；先例 aee05996（109 C 腿同款）。**裁定：采认为卡内必要实现面，非越域**；此双源同步模式今后涉 prompt 模板的卡在卡面文件域直接列明。
- 证据门不弱化逐断言论证与 diff 相符：settle/final gate 断言、MaterialityEvaluation、audioclosure.AdmitReObservation 零改动亲核；豁免只放行请求执行，fresh 证据仍须本周期真实返回。
- 端测边界=零真栈（卡面约定）；实链行为归 109 rerun5。

## ③ 109 rerun5 解锁

- 裁定四既定梯子条件满足（112+113 均验收合入）：**rerun5 可发**——flash 思考档 2 轮，worktree fsdc1r2、新分支自当前 main 建；核验锚点新增=identity_preserved 标记（112）+收窄重取豁免放行（113）+全字段拒收教学（腿 E）。exit 0=三账同销+M2 体验场次解锁；同签名 2/2→升 v4-pro。
- 三笔挂账（M2-TIMEFX-1 111841/M2-TIMEFX-2 195448/FIX-106）维持未销至 rerun5。

## 工件

- 合入：26757b34（cherry-pick -x 6ae5525a）+1f4e2919（cherry-pick -x 51e5ca9a）；裁定文件=本文件。
- 亲跑工件：/tmp/gotest_112.log+/tmp/gotest_113.log（各 95 包 exit 0）；gofmt blob 核六文件净。
