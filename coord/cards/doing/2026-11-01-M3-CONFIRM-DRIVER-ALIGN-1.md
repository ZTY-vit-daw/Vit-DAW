# M3-CONFIRM-DRIVER-ALIGN-1：确认驱动短语对齐卡面词表+驱动超时窗扩容+模型收敛测量面（229 验收门第六承，233 两案主管裁定 B 承接）

- 发卡：GLM 主管决策流 / 2026-11-01（依据=233 验收定案+主管裁定 B 案：驱动短语换卡面词。主管亲核词表锚点——actionworkflow acceptText 精确表含「确认」「可以」「确认执行」且 isImprovementProposalConfirmationText 收「确认+提案/建议/这个」，**卡面按钮印的「确认提案」与裸「确认」均可过生产词表**，smoke CONFIRM_PHRASE=「确认写入」（218 choreo 卡按钮短语）恰为唯一不兼容族；生产词表覆盖真实用户话术面已足，为测试短语扩生产词表=尾巴摇狗，A 案否决；233 milestone：五卡以来第一次 G1-G8 全过+提案接受+确认卡弹出）
- 派发确认：已确认（主管裁定域内，scripts 单域小卡）
- 验收负责人：GLM 主管决策流
- 池序 234；目标仓库=**主仓 scripts 单域**；P2 / 0.2 天 / 依赖=233 已在 main（42134f36）✓；L1 / PC 执行会话

## 标的（233 定案+主管裁定，不重考）

smoke 的 CONFIRM_PHRASE=「确认写入」（automation_llm_route_smoke.py:107，218 choreo 时代短语）对 improvement_proposal 确认卡不路由（types.go acceptText 无该族+isImprovementProposalConfirmationText 需「确认+提案/建议/这个」）；CHAT_TIMEOUT_SEC=300 硬编码（:105）与门开后链长错配（wire1 中断亲证）；残留观察循环不收敛形态需测量面数据。

## 目标

1. **驱动短语对齐**：CONFIRM_PHRASE 改为卡面按钮词「确认提案」（improvement_proposal_workflow.go:239 的 Label，用户所见即所输入的最忠实形态）；如 choreo-pending 腿的确认短语与 choreo 卡词表有同族耦合，分卡种用各自卡面词（勘察后最小改）。**生产词表零改动**（A 案已否决）。
2. **超时窗扩容**：CHAT_TIMEOUT_SEC 提至覆盖确认链修通后的单轮全长（写入+读回在窗内；建议 600s 起勘察定，推导成文）。
3. **模型收敛测量面（越测量不越修复）**：重验腿逐轮采集——每轮 turn 预算消耗/观察请求次数与返回内容（ready 事实 or missing/partial 重复）/提案到达与否/终局形态分布；对「观察循环耗尽 48 轮不收敛」的轮，摘录观察返回内容判别：观察返回 ready 事实仍重复请求=模型行为面；观察反复 missing/partial=证据面仍有缺（锚点上交）。
4. **重验腿（本卡验收门，229 原口径第六承）**：组 1「这段让它越来越突出一点」——组 1 no_route 消失+set_points 确认链（确认卡→确认→写入→get_curve 读回）首次端到端全绿；组 2/3 按 220 口径跑满。**判定逻辑其余零改动**。

## 文件域

`scripts/automation_llm_route_smoke.py` 单文件。越域即停上交。

## 预期与诚实边界（发卡时即声明）

wire2 组 1 形态=[no_route(观察耗尽), confirm_broken×2→本卡修后应为 success, capability_blocked(模型申报), no_route(观察耗尽)]——确认驱动修通后组级 ≥4/5 **大概率仍受观察循环不收敛制约**（预期 2-3/5）。若 <4/5：本卡按停止条件**整理模型能力面终局证据包**（五卡证据链+逐轮收敛数据+观察返回内容判别）上交主管转模型/指引设计讨论（决策轮换引擎 or 指引效率 nudge——届时带数据），不再折卡。

## 停止条件

- 确认短语换卡面词后仍不路由→生产路由缺陷锚点上交（非本卡域）；
- 确认 settle 后写入/读回任一环失败→write_fail 如实锚点上交；
- 组级 <4/5→模型能力面终局证据包上交（本卡核心交付之一，非失败）；
- LLM 面栈不可用→环境中断如实记录。

## 领取

- 领取会话：PC 执行会话（GLM-5.3，ZCode）/ 2026-11-01
- worktree：`D:\Vit_DAW_wt_234impl`，分支 `port/m3-confirm-driver-align-1`（impl 走 port 分支+主管合入）
- 领取基线：`git rev-parse HEAD` = `4e03face07bc54b278025e8ee7545f0d522169c7`（=origin/main，本卡发卡提交）；`git status --short` 空；`git diff --stat` 空（干净起点）
- 勘察锚点亲核（2026-11-01）：improvement_proposal 卡按钮词=「确认提案」（`agent/internal/chat/improvement_proposal_workflow.go:239`，过 `isImprovementProposalConfirmationText` 的 确认+提案）；choreo 卡按钮词=「确认写入」（`agent/internal/chat/automation_choreo.go:713`，其 settle 词表 :584/:601 含该词）→ 分卡种各用其卡面词：主确认链（run_round/settle_from_terminal_wait）改「确认提案」，choreo-pending 腿保持「确认写入」（该腿与 choreo 卡词表耦合，即卡面 233 所述「唯一不兼容族」仅影响 improvement_proposal 面）；CHAT_TIMEOUT 推导数据=wire2 12 轮 telemetry 逐轮 wall（max 276.4s）+wire1 g1_r2 超 300s 客户端窗亲证
- 真栈腿登记：PC-RUNTIME-STACK.md（见回执；登记先于启动）
