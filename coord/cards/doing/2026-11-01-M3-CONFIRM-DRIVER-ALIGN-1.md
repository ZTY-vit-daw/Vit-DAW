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

## 回执（执行侧自验完成，留 doing 待主管验收）

**commit**（port 分支 `port/m3-confirm-driver-align-1`，已 push，待主管合入）：13f69c28（短语对齐+超时窗+测量面）→ 65ecba63（叠卡驱动勘察扩展）→ e43ed18f（双 run 工件+EVIDENCE.md）。领取 41d0dd7d / 栈登记 90ea7bb1→fbd8284e / 释放 63e6fb79（main）。

**测试命令及退出码**：
- `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_234impl -RunArtifactsDir …\alignrun1 -StartKernel -RestartAgent -KernelExe <wt_228 f9e3da0a> -AutomationLLMRouteSmoke` → **exit 0**
- 同型 alignrun2（kernel 38132 原栈复用+`-RestartAgent -SkipBuild`，VitAgent.exe sha256 1167f56c 本会话自建）→ **exit 0**
- py_compile+测量面/叠卡驱动合成数据自验全过（alignrun1 g1_r2 实况形状回放）

### 链路进度（三里程碑，全部真栈亲证在卷）

1. **「确认提案」路由修通**：a1 g1_r2 ex1+a2 g1_r4 ex1 双样本——驱动后 stop_reason=`improvement_proposal_native_tool_confirmation_required`+「已进入原生控制工具链」，确认文本首次 settle pending interaction（233 wire2 断点消除）。
2. **叠卡勘察**：settle 后链路叠出第二层 `mix_tick_confirmation` 卡（卡面「确认执行」）——卡面「分卡种各用其卡面词（勘察后最小改）」条款延伸，65ecba63 补 `confirm_phrase_for` 严格面分词（mix_tick→「确认执行」/improvement→「确认提案」/其余→裸「确认」），判定逻辑零改动。
3. **确认链端到端首绿**（a2 g1_r4 ex0→ex2）：确认卡→「确认提案」→mix_tick 卡→「确认执行」→**写入+回读验证（+1 dB，回读 1.5→2.5 dB）**=229 五卡链以来第一次治理面全绿写入。choreo 腿「确认写入」pending_intact。

### 最终验收（229 口径第六承）——未达，如实分列

- 组 1 no_route 消失：**未达**（a1 g1=[no_route×4,confirm_broken]、a2 g1=[capability_blocked,no_route×3]，两 run 均 3 连 no_route 止损）；
- set_points 确认链端到端首绿：**部分**——确认链四环首绿 ✔，写入形态=静态 mix tick 非 set_points 曲线（两次到达提案的轮模型均选静态小幅步）；
- 组 2/3 跑满：**未跑**（组 1 止损连带跳过；§8 同断点两连禁原样重跑，不开第三 run）。

### 模型能力面终局证据包（<4/5 停止条件触发，核心交付）

详见 `coord/runs/M3-CONFIRM-DRIVER-ALIGN-1/EVIDENCE.md` §C/§D：9 采样轮逐轮收敛数据（turn 16-42/观察 0-27/LLM 17-43 调用每轮）+**探针 9/9 view_present+ready 事实**（rms_dbfs=-11.889 等）=观察面有事实而不收敛到提案 ⇒ **模型行为面非证据面**；终局分型=观察重复型×6/中性族空转型×3（其中提案到达型×2 选静态步）/能力申报型×1。五卡链（230→234）生产环全修通后剩余形态独占模型行为面→按卡面预埋转主管裁定（决策轮换引擎 or 指引效率 nudge），不再折生产修复卡。

### §8 计费披露（deepseek-flash 真实计费）

alignrun1：135 调用 1,976,924 in/179,201 out；alignrun2：118 调用 1,757,967 in/170,416 out；**合计 253 调用 3,734,891 in/349,617 out**（0 env_interrupt 0 canceled）。

### 端测覆盖边界声明

两 run 均=真实三件套栈（VitApp f9e3da0a+agent 1167f56c+HTTP/ZMQ 面驱动），ps1 exit 0×2；渲染面/用户旅程不涉（scripts 单域驱动卡）；工件在 port 分支 `coord/runs/M3-CONFIRM-DRIVER-ALIGN-1/{alignrun1,alignrun2}`（排除 .vit_agent 101M 状态树与可再生 wav）。

### 诚实边界

① round 后观察探针=同面确定性状态读非模型自身返回体回放（该数据不落盘）；② a2 g1_r4 ex3 D1 事后评估面收到既有 REGION_FOLLOWUP→goal failed→no_route，判定逻辑零改动未动，形态入卷；③ alignrun2 默认腿一次 strip.chat durable_checkpoint_persist_failed 瞬态（kernel 复用锁残留，非本腿面）；④ 组 2/3 未采样，跑满需求归主管随证据包处置。
