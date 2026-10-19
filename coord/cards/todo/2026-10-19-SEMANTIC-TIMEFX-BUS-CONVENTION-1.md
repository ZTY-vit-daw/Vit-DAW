# SEMANTIC-TIMEFX-BUS-CONVENTION-1：语义层时域总线惯例（"加混响=上 FX 轨"）

- 发卡：GLM 主管决策流 / 2026-10-19 晚窗（依据=用户裁定「加混响本身就是加 FX 轨调控的动作，agent 若无此习惯：先搞清为什么没有，再让它有」——为什么没有已由本卡前置锚点回答，本卡=取证余项+设计注入+拆实现卡）
- 派发确认：已确认（用户 2026-10-19 晚窗明示）
- 验收负责人：GLM 主管决策流
- 池序 146；目标仓库=D:\Vit_DAW；P1 / 探查+设计 0.5-1 天（实现另拆卡）/ 依赖=send 底座已收官（K1 f4926543+K2 d4ea37ae+K3 0e244f0）
- 模型分级：L2 / GLM 执行会话（agent 语义面）

## 已钉事实（勿重复取证）

1. **观察侧已接**：C3 空间深度观察面（`agent/internal/chat/c3_space_depth_runtime.go`，MIXLAYER-M1-3）只读，载体 4=`track.send.list` 读回（K2 接入）——agent 能看见 send/return 结构。
2. **规划侧零概念**：`agent/internal/chat/semantic_treatment_strategy.go` 全文无 send/aux/bus/topology 字样——治疗规划无拓扑决策。
3. **控制面明确缺席**：C3 边界注释原文「语义 reverb/delay 控制面属后续 C3 控制侧卡」——本卡即该卡的探查+设计段。
4. **确定性面仅显式触发**：K2 `send_route_workflow.go` 只拦「具名 ≥2 源轨的显式 send 结构请求」；模糊意图不经过它。
5. **M2 实验走临时轨**：M2-TIMEFX 线实验/认证装载用即建即删探测轨（历史卡在卷）；模糊意图链的施加拓扑细节=本卡待证项。
6. **可复用零件**：时域族词表 `c3SpacePluginPathMarkers`（reverb/delay/echo/room/hall/…）+ `c3SpaceParamMarkers`（wet/decay/…）；内核 `track.send.*` 五命令；MOM send_topology 投影。

## 待证（取证腿）

1. 模糊意图（"给人声加点空间感"）当前 M2 编排链的实际提案形态与施加拓扑：提案确认卡内容、装载位置（inline 人声轨 / 临时轨 / FX 轨）、有无任何 send 结构——以代码锚点+明日 M2 体验场次实录互证；
2. 规划→拓扑决策的注入点：semantic_treatment_strategy / planner / M2 编排器（semanticorchestrator）哪一层持有"装载位置"决策；
3. 例外面清单：哪些场景 inline 是正确的（用户明说插入 / 单轨快速 A/B / 实验探测）。

## 设计方向（草案，待取证后定稿）

- **默认规则**：时域族（词表复用 ⑥）→ 默认 send/return 拓扑（新建 FX 轨默认名 Reverb Bus 类+装载+track.send.connect），inline 为显式例外（用户明说/实验探测）；
- 提案确认卡呈现**拓扑计划**（建什么轨、连谁、默认发送量），用户确认后执行；
- 与 129 宏卡衔接（FX 轨 return/发送量可绑宏）留接口位不做实现；
- 观察侧（C3）无需改动（已能读）；
- prompt 契约同步：模型获知惯例规则但不擅自发明拓扑（解释/遵循已给选项，与 136 同 posture）。

## 验收标准

探查+设计卡：现状结论（含体验场次实录互证）+注入点设计+实现卡拆分建议（含文件域与验收面），交主管定稿后拆实现卡。本卡自身不改生产代码。

## 停止条件

- 取证发现惯例已部分存在（某层已有拓扑决策）→ 如实报告并收窄设计范围；
- 注入需要内核新命令（现有 track.send.* 不够表达）→ 上交缺口锚点。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（结论文件 / 锚点清单 / 设计稿路径 / 体验场次实录关联）
- 验收：（裁定文件 / 验收 commit）
