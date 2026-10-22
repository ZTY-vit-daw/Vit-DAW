# UX-AB-AUDITION-ENTRY-1：动态链执行完成后的 A/B 对照试听接通（勘察设计先行）

- 发卡：GLM 主管决策流 / 2026-10-22 晚（依据=用户 2026-09-12 裁定「时域效果器控制的第一个可感功能落地时体验」节点已到（场次④实证 send+混响装载落地）+场次④用户反馈「agent 完成没有向我提供 AB 试听」+主管取证：本轮会话 agent_last.log 零 audition/AB 痕迹、workspace 零试听资产=链路未接非入口丢失）
- 派发确认：已确认（用户裁定节点+现场反馈；2026-10-22 晚用户确认：按修正定性出稿——两案评估入设计稿，终案交用户裁定）
- 验收负责人：GLM 主管决策流
- 池序 172；目标仓库=D:\Vit_DAW（勘察）→实现卡另拆（agent/前端按设计定）；P1（M2 可感收口）/ 0.5 天勘察+设计；模型分级：L2 / GLM 执行会话

## 现状取证（2026-10-22 场次④）

- 用户旅程完成（19:44:36 semantic_dynamic_execution_completed）后无任何 A/B 试听提供；agent 日志/工作区零 audition 面。
- **既有基础设施（盘点起点）**：agent 侧 MOM/mixboard ABResult（run_ab_result_smoke.ps1 健康检查在跑）、FXM（bypass/processed render 对照投影）、webui audition.ts+auditionJudgmentIdentity（盲测判断面）、内核 VitAuditionPreview（VitAuditionPreviewStateTests）、VIT_DAW_AUDITION_BLIND 环境变量（盲测模式，AGENTS §5）。Godot 前端仅 startup/wav_audition_loader（导入面，非效果对照）。

## 现状机制取证（2026-10-22 晚补充，主管代码勘察）

**AB 试听按执行路径挂接，动态链路径零挂接**：
- 实验路径（free_state_experiment_runtime.go:782 prepareFreeStateAudition）：needs_experiment 提案→用户确认→执行实验→**自动准备 A/B 试听**→判定落账。用户此前触发过试听的操作=此路径（显式实验/提案确认形态）。
- 动态链路径（semantic_dynamic_workflow.go / plugin_grabber_workflow.go：**0 处 audition**）：模糊意图→模型自主决定并直接执行→completed，无任何试听面。场次④「给人声加空间感」全访问工程默认路由=此路径——「之前的操作会触发、混响处理不触发」的机制解释。

**设计定性修正（2026-10-22 深夜，用户追问后二次取证）**：判断标准「需要用户试听」**已在提示词层**——full_access_autonomy.go:69/:82 明示「execute…and hand the result to the user's A/B listening. In this mode the user's only duty is the A/B audition」。缺陷不在模型判断（模型被明确告知链末=交付试听），而在**系统未兑现承诺**：①模型动作面零试听工具（dynamic workflow 0 处 audition，无从交付）；②动态链 runtime 无 prepareFreeStateAudition 自动挂接（实验路径有：free_state_experiment_runtime.go:782）；③呈现入口无事件可消费。**「hand to A/B audition」=空头支票**。
**设计两案（172 设计卡评估后交用户裁定）**：主案=系统自动兑现——动态链 completed 且含效果类 mutation→复用实验路径 prepareFreeStateAudition 同机制自动挂试听入口（提示词承诺系统兜底交付，模型零新动作）；备案=模型显式试听动作（工具/结构化回复字段）按请求 prepare（更语义化但引入模型漏提供风险）。两案共同面：对照粒度=被处理轨 bypass vs processed（FXM 既有形态）、惰性生产、盲测可选、实验路径契约不变。

## 范围（两步走，本卡=第一步勘察+设计，零生产码改动）

①盘点：上述既有 A/B 面与「动态链 completed（semantic_dynamic_execution_completed/162-族终态）」的最近接通点——ABResult 生产链的触发条件、资产形态、消费面；FXM 对照 render 的可用时机；②设计最小可感路径：任务完成后用户**一键听到 before/after**——触发时机（completed 事件→生产 or 惰性按需）、资产生产（bypass/processed 两版 render 或实时 bypass 切换）、呈现入口（PlanBar 完成态挂试听入口/机架/混音台——与 163/171 呈现链协调）、盲测选用（VIT_DAW_AUDITION_BLIND 既有面）；③输出设计稿（含分卡建议：agent 生产卡+前端入口卡+真栈验收腿）入 coord/reports/。

## 文件域

勘察零改动；设计稿=coord/reports/2026-10-22-ab-audition-entry-design.md（或 runs/）。实现卡按设计另开。

## 验收标准

设计稿含：既有面盘点结论（每面可用性判定+证据锚点）、接通路径图、分卡建议（每卡文件域/验收/真栈腿）、与 171（send 拓扑链）的依赖关系判定。上交主管裁定后开实现卡。

## 停止条件

盘点发现既有 AB 面全部不可用（需从零建渲染管线）→ 设计稿如实标注+最小可行面（如仅 bypass 开关实时对照）上交用户裁定范围。

- 领取：2026-10-22 20:12 / origin/main=2b3a9b05（领取时远端 tip；本卡发卡面自主管本地 main ee0050de 摘取入共享 main，同 787063ec/2b3a9b05 先例只同步 coord 面）/ owner=GLM-5.3 L2 PC 执行会话（ZCode，172 卡会话）/ 分支=主仓 main（勘察设计卡零生产码，用户明示主仓登记；无 port 分支）+ 协调分支 coord/aba172-claim / worktree=主仓 D:/Vit_DAW（勘察期零生产码改动）/ 领取提交=coord/aba172-claim tip
- 回执：设计稿=coord/reports/2026-10-22-ab-audition-entry-design.md（主仓工作树+协调分支 coord/aba172-claim 双留存）/ 关键结论：①既有面盘点全部可用（内核 AuditionPreview 状态机/agent 事件链/webui 判定卡消费链/render 生产/盲测面均生产就绪），停止条件未触发；缺的只有动态链执行腿零挂接（semantic_dynamic_workflow.go grep 0 处 audition 实证，卡面定性成立）；②终案推荐=主案系统自动兑现——在 executeSemanticDynamicTicket 挂 mix-tick 形态 audition bracket（before render→参数写→after render→mount 会话→audition.ready 事件），复刻 runMixTickAuditionBracket 时序，零内核/零 webui 改动；备案不推荐（触发确定性与 159/170 概率性供应缺口同族）；③对照粒度修正（需裁定）：内核 start_render 无轨级/插件 bypass 参数（TransportAudioService.cpp:1185 实证）→首版整工程 before/after，卡面预设「FXM 轨级粒度」转后置卡 B；④判定落账需第三身份通道（judgment POST 现双路由不识别动态链身份，free-state 校验绑 loop.Experiment.ID 必 409）——按 mix-tick 同模式扩展；⑤与 171 无依赖、文件域不相交（171=Godot 前端仓，172 首版=agent Go 侧）可并行；⑥三处需用户裁定：终案确认/对照粒度/同步 bracket 基线（渲染时长降级线授权实现卡实测定）/ 零生产码改动（工作树唯一新增=设计稿），勘察边界=无真栈 run，结论全部代码锚点取证
- 验收：**通过**（GLM 主管决策流 / 2026-10-22 深夜）。设计稿 12 面盘点证据锚点抽核（full_access_autonomy.go:69 承诺在卷/mix_tick_audition.go bracket 模板/TransportAudioService.cpp:1185 无轨级参数实证）；终案=主案（系统自动兑现）与主管四维修读一致。两修正点主管裁定：①首版整工程 before/after（轨级=内核增强后置卡）②判定第三身份通道——均记入实现卡 174 待用户知情确认。实现卡=**174 UX-AB-AUDITION-IMPL-1**（已发）。验收 commit=本记录链。
