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

## 实验阶梯（用户 2026-10-19 晚窗裁定：先烟测→找原因→软引导实验→强约束，每层实验后再定下一层实现量）

1. **烟测现状**（=取证腿行为半边，明日 M2 体验场次兼实录）：模糊意图实际提案拓扑/装载位置；顺带观察有无偶发自发总线制（有则说明观察披露已在起作用）。
2. **归因**（架构层已答 80%：规划层零拓扑概念+控制面未建+底座今日才齐）：烟测补行为半边后收口。
3. **软引导实验**（§8 协议预写）：prompt 契约草案声明惯例+观察披露维持现状，测模型能否自行路由到总线拓扑。固定话术集（模糊意图 8-10 变体）；成功条件=≥80% 提案含总线拓扑+100% 不发明未知拓扑；失败分类=inline 偏好/无拓扑主张/幻觉拓扑；止损=同签名失败×2。**实验产出决定第 4 步硬规则厚度**。
4. **强约束落笔**（生产终态 posture 同 136：软的管解释、硬的管落笔）：默认拓扑计划由确定性规则生成进提案卡；prompt 契约承担解释与例外路由。软引导成功率≥80% → 硬规则只守写门（确认时拓扑校验）；<80% → 硬规则接管整个拓扑计划生成。

## 设计方向（草案，待取证+软引导实验后定稿）

- **默认规则**：时域族（词表复用 ⑥）→ 默认 send/return 拓扑（新建 FX 轨默认名 Reverb Bus 类+装载+track.send.connect），inline 为显式例外（用户明说/实验探测）；
- 提案确认卡呈现**拓扑计划**（建什么轨、连谁、默认发送量），用户确认后执行；
- 与 129 宏卡衔接（FX 轨 return/发送量可绑宏）留接口位不做实现；
- 观察侧（C3）无需改动（已能读）；
- prompt 契约同步：模型获知惯例规则但不擅自发明拓扑（解释/遵循已给选项，与 136 同 posture）。

## 验收标准

探查+设计卡：现状结论（含体验场次实录互证）+注入点设计+实现卡拆分建议（含文件域与验收面），交主管定稿后拆实现卡。本卡自身不改生产代码。

## 2026-10-20 体验场次①实录补记（待证 1 预答）

实录=[2026-10-20-m2-experience-session-1.md](../../reports/2026-10-20-m2-experience-session-1.md)。模糊意图（「给人声加点空间感」）实际路径：**到不了任何提案**——server-side 自由态准入门拒绝（failed_gate_ids=G3/G5/G6/G7/G8，`internal/chat/goalrunner_chat.go:2096-2104`），模型 prose 终答被扣，用户收到 G 门代号文案；且该 turn 零观察调用（模型把新话术当前轮延续）。取证腿据此转向：①准入门拒绝根因分层（模型未做本轮观察 vs 门语义过严）；②注入点设计必须回答「语义意图路由在 G 门评估之前还是之后分流」；③软引导实验话术集需含「准入门先过」前置，否则实验测的是门不是路由。另注：主话术（显式混响）路径走 plugin_recommendation 链 inline 装载 bass 轨、无 send 结构——显式路径的拓扑行为与 F1/F2/F3 finding 卡（147/148/149）相关但不属本卡修复域。

## 停止条件

- 取证发现惯例已部分存在（某层已有拓扑决策）→ 如实报告并收窄设计范围；
- 注入需要内核新命令（现有 track.send.* 不够表达）→ 上交缺口锚点。

- 领取：2026-10-20 18:50 +0800 / origin/main=03fea178（基线=本地 main 358b3de7，主管 18:39 findings 提交尚未推送，本领取推送连带 fast-forward 入远端）/ owner=GLM-5.3 执行会话（ZCode，PC，本会话，L2 agent 语义面）/ 分支=port/semantic-timefx-bus-1（报告与设计稿）+coord/timefx146-claim（卡态） / worktree=D:/Vit_DAW_wt_timefx146 + D:/Vit_DAW_wt_timefx146_coord / 领取提交=3c4176b7（经主线 merge 2e693df1 并入）
- 回执：**结论文件**=[coord/reports/2026-10-20-146-timefx-bus-forensics.md]（取证①-④：根因四层分层 L0-L3——**F4 修正：模型自报 capability_blocked（人声轨身份未披露 target_identity_not_disclosed）、无提案、boundary=proposal_missing；用户面门代号文案是回复面 goalrunner_chat.go:2098-2107 failed_gate_ids 劫持（新 finding，建议并入 147 族或小卡 D）**；路由在 G 门前且已工作（入口分类器内容盲 semantic_entry.go:96-102）；**零层持装载位置决策**=plugin_recommendation 链 selected_track_id 直通（:125/286/528/568/814/843）；inline 例外清单×5）/ **锚点清单**=报告内嵌（E1-E8 工件互证+行号锚）/ **设计稿**=[coord/reports/2026-10-20-146-timefx-design-and-experiment.md]（§8 实验方案预写：10 变体+B1 对照 5run/≥80% 成功线/失败分类×5/止损×2/准入门前置话术——**实验未跑，按卡面交主管确认后另腿执行**；设计三件套=软提示词+拓扑计划对象+写门校验；拆卡 A-E+优先序 D→A→B/C、E 单独）/ **实录关联**=取证报告 §① 证据链互证+§「对实录 F4 的修正建议」（与实录增订 Q2 勘误方向一致）/ 报告 commit=**port/semantic-timefx-bus-1@dff52757 待主管验收 cherry-pick**；本卡零生产代码改动；停止条件两条均未触发（拓扑惯例零层存在≠已部分存在；track.send.* 足够表达，无需内核新命令）；并行注记：147 已领取（4dbcda83），设计稿卡 B 文件域与之相交已标注"147 合入后再领 B"
- 验收：（裁定文件 / 验收 commit）
