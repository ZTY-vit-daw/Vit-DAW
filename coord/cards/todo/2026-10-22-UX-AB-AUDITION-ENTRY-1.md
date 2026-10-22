# UX-AB-AUDITION-ENTRY-1：动态链执行完成后的 A/B 对照试听接通（勘察设计先行）

- 发卡：GLM 主管决策流 / 2026-10-22 晚（依据=用户 2026-09-12 裁定「时域效果器控制的第一个可感功能落地时体验」节点已到（场次④实证 send+混响装载落地）+场次④用户反馈「agent 完成没有向我提供 AB 试听」+主管取证：本轮会话 agent_last.log 零 audition/AB 痕迹、workspace 零试听资产=链路未接非入口丢失）
- 派发确认：已确认（用户裁定节点+现场反馈）
- 验收负责人：GLM 主管决策流
- 池序 172；目标仓库=D:\Vit_DAW（勘察）→实现卡另拆（agent/前端按设计定）；P1（M2 可感收口）/ 0.5 天勘察+设计；模型分级：L2 / GLM 执行会话

## 现状取证（2026-10-22 场次④）

- 用户旅程完成（19:44:36 semantic_dynamic_execution_completed）后无任何 A/B 试听提供；agent 日志/工作区零 audition 面。
- **既有基础设施（盘点起点）**：agent 侧 MOM/mixboard ABResult（run_ab_result_smoke.ps1 健康检查在跑）、FXM（bypass/processed render 对照投影）、webui audition.ts+auditionJudgmentIdentity（盲测判断面）、内核 VitAuditionPreview（VitAuditionPreviewStateTests）、VIT_DAW_AUDITION_BLIND 环境变量（盲测模式，AGENTS §5）。Godot 前端仅 startup/wav_audition_loader（导入面，非效果对照）。

## 范围（两步走，本卡=第一步勘察+设计，零生产码改动）

①盘点：上述既有 A/B 面与「动态链 completed（semantic_dynamic_execution_completed/162-族终态）」的最近接通点——ABResult 生产链的触发条件、资产形态、消费面；FXM 对照 render 的可用时机；②设计最小可感路径：任务完成后用户**一键听到 before/after**——触发时机（completed 事件→生产 or 惰性按需）、资产生产（bypass/processed 两版 render 或实时 bypass 切换）、呈现入口（PlanBar 完成态挂试听入口/机架/混音台——与 163/171 呈现链协调）、盲测选用（VIT_DAW_AUDITION_BLIND 既有面）；③输出设计稿（含分卡建议：agent 生产卡+前端入口卡+真栈验收腿）入 coord/reports/。

## 文件域

勘察零改动；设计稿=coord/reports/2026-10-22-ab-audition-entry-design.md（或 runs/）。实现卡按设计另开。

## 验收标准

设计稿含：既有面盘点结论（每面可用性判定+证据锚点）、接通路径图、分卡建议（每卡文件域/验收/真栈腿）、与 171（send 拓扑链）的依赖关系判定。上交主管裁定后开实现卡。

## 停止条件

盘点发现既有 AB 面全部不可用（需从零建渲染管线）→ 设计稿如实标注+最小可行面（如仅 bypass 开关实时对照）上交用户裁定范围。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（设计稿路径 / 关键结论）
- 验收：（裁定文件 / 验收 commit）
