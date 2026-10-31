# 233 真栈重验腿证据锚点（wire1+wire2）

run：coord/runs/M3-FS-GATE-EVIDENCE-WIRE-1/{wire1,wire2}（2026-10-31 21:41-22:23 UTC+8）
被测=wt_233impl @ port/m3-fs-gate-evidence-wire-1（实现 commit 20bcff64）；kernel=wt_228 VitApp.exe sha256 f9e3da0a 只读复用。
腿=AutomationLLMRouteSmoke（229 原口径验收门第五承）。wire2 ps1 exit 0（dev agent smoke completed）；wire1 ps1 exit 1（驱动面超时，见 §C）。

## A. 三合一修复的生产生效证据（链路进度）

1. **A1 G6/G8 证据接线生效——capability_blocked 的 admission gate 形态消失**：
   - 232 同腿 8/10 轮 capability_blocked（G6/G8 fail+admission receipt+capability_boundary 投影）；本轮两 run 共 11 个采样轮 **零 admission_gate_failed**：wire2 g1 四轮与 g2 五轮的 capability_boundary 投影均无 admission receipt（g1_r4 的 capability_blocked 是模型主动能力申报非门拒绝）。
   - wire2 g1_r2/g1_r3/g2_r1/r3/r4/r5 **首次走到 improvement_proposal 确认卡**（stop_reason=improvement_proposal_confirmation_required、typed_event=PendingCandidate、status=waiting_user、InteractionRequests 在场）——229 五卡以来组 1/组 2 第一次 G1-G8 全过+提案被接受。
2. **B taskstate 幂等重入生效——232 的 goal failed 形态消失**：232 g1_r4 曾因 `semantic transition capability_blocked is not allowed from capability_blocked` 终局 failed；本轮所有 capability_blocked 终局轮 goal 状态正常，无一条该错误。
3. **A2 family 断线**：本轮无 blocked 轮携带 capability_boundary 投影（门开后不再触发该面）；family 解析在单测层验证（family_action_domain_test.go 全词表往返+volume 族收敛）。

## B. 验收门第五承未达——确认驱动环断线（confirm_broken，按停止条件上交）

229 口径：组 1 no_route 消失+set_points 确认链端到端首绿+组 2/3 跑满。实际：

- wire2 组 1：[no_route, confirm_broken, confirm_broken, capability_blocked, no_route]，successes=0
- wire2 组 2：[confirm_broken, no_route, confirm_broken, confirm_broken, confirm_broken]，止损（confirm_broken 三连）
- 组 3 skipped（220 原口径）

**confirm_broken 机制（g1_r2 全链在卷）**：ex0 模型提案→确认卡弹出（PendingCandidate/waiting_user）→ex1-3 驱动发「确认写入」×3 全部落入 LLM 纯文本轮（tools=[]、模型重新叙述提案、stop_reason 原地不动）——**确认短语从未路由到 pending interaction 的 settle**。

**锚点（词表错配，两处均在越域文件）**：
- `agent/internal/actionworkflow/types.go:255-263`（acceptText）：switch 精确表含「确认」「确认执行」但**无「确认写入」「写入」「开始写入」**，containsAny 表同样没有；
- `agent/internal/chat/server.go:4035-4050`（takePendingImprovementProposalForChat+isImprovementProposalConfirmationText）：判定链=ClassifyConfirmation∈{accept,reject} 或 文本含「确认」+（「提案」|「建议」|「这个」）——「确认写入」含「确认」但无后三者→false→不路由。
- 对照面：`agent/internal/chat/automation_choreo.go:584` 词表含「确认写入」（section_automation 卡按钮 Label 就印它，automation_choreo.go:713）；improvement_proposal 卡按钮 Label=「确认提案」（improvement_proposal_workflow.go:239），该文本可过 isImprovementProposalConfirmationText。驱动脚本 `scripts/automation_llm_route_smoke.py:107` 固定 `CONFIRM_PHRASE="确认写入"`（218 时代 choreo 卡的按钮短语）。
- 归因：A1 修复生效前该确认卡从未到达（232 全部在 G6/G8 被拒），「确认写入」与 improvement_proposal 确认词表的兼容性从未被任何腿测过。属**确认面词表不一致**：同一句确认短语对 choreo 卡有效、对 improvement_proposal 卡无效。

**两案上交（执行会话不择案）**：
- A=生产侧补词表：acceptText/isImprovementProposalConfirmationText 补「确认写入」族（越域=actionworkflow+chat 路由域，与 218 choreo 词表对齐）；
- B=驱动侧换短语：CONFIRM_PHRASE 改「确认提案」（scripts 域，本卡红线同样禁改）。
两案都不在本卡文件域内→按停止条件（「确认驱动/写入/读回任一环失败→如实锚点上交」）上交。

**no_route 残留形态（模型行为面，非门拒绝）**：wire2 组1 两轮 no_route 均为「观察循环耗尽 turn 预算」（g1_r1 12 步、g1_r5 类似，全 ccb.observation_request 重复请求无提案）→limit_reached→continuation settle goal completed 无写入。与 229 的 no_route（模型纯文本不动工具）不同类：本轮模型在观察、链路活着，但单轮内不收敛到提案。

## C. wire1 中断记录（环境中断如实分类）

wire1 g1_r1 完成后（verdict=no_route，同上观察循环形态），g1_r2 的 /agent/chat HTTP 300 秒未返回（telemetry 尾行 `err=context canceled`=客户端超时断开；此前 LLM 调用全部正常 5-10s/次）→驱动脚本 `CHAT_TIMEOUT_SEC=300`（scripts/automation_llm_route_smoke.py:105 硬编码，--timeout-sec 只管启动等待）与门开后链长错配→python ERROR→ps1 exit 1。属驱动面参数错配非栈/LLM 故障；wire2 复跑未再触发（各轮 HTTP 均在窗内返回）。**注意：若确认链修通（写入+读回），单轮链将更长，CHAT_TIMEOUT_SEC=300 可能结构性不足——两案上交时一并披露。**

## D. §8 计费（真实 LLM，deepseek-flash）

- wire1：61 计费调用，908,608 in / 77,694 out（1 调用 canceled 不计费口径单列）
- wire2：167 调用，2,473,172 in / 236,263 out
- 合计 228 计费调用，约 3.38M in / 314K out

## E. 栈与工件

- wire1（中断）：rounds/g1_r1.json+setup_acoustic_package_status.json+logs/（telemetry 62 行含 canceled 尾行）；ps1_console.txt
- wire2（完成）：automation_llm_route_summary.json（status=COMPLETED）+rounds/g1_r1-r5、g2_r1-r5+logs/+ps1_console_wire2.txt；choreo-pending 腿 g1/g2 均 pending_intact
- 拆栈：PROCS=0（VitApp/VitAgent/VspHub）+TCP 5555/5556/7878/8787/18787/5178 listen=0+UDP 4444/4445=0（TEARDOWN_ALL_ZERO）
