# FREESTATE-REGION-GOAL-1 · DESIGN：自由态范围目标两轮制协议（设计产出，2026-10-04 晚）

- 决策侧亲自收口设计段（先例 CONTAINER-3）；基线 8cd98a60。
- 依据：[decisions/2026-10-04-region-op-path-and-freestate.md](../../decisions/2026-10-04-region-op-path-and-freestate.md) 裁定 2/3（自由态默认方向+两轮制仅 a/B1）+[RECON.md](../REGION-OP-RECON-1/RECON.md)+REGION-INTENT-WIRE-1（已合 main c2edb0d6）。
- 红线复述：D1 prompt 规则文案（ccb_model_prompt.go:131/133）**零改动**；两轮制=轮次编舞不是规则变更。

## 锚点地图（勘察实证）

| # | 锚点 | 事实 |
|---|---|---|
| D1 | `chat/goalrunner_chat.go:32` | `beginChatGoal(conversationID, message, requestContext)` 整体收 requestContext——`selected_clip_ranges` 已随 server 白名单（server.go:2171-2175）+结构校验（:5119-5128）进入 goal 运行时 |
| D2 | `agentloop/helpers.go:143-145` | agentloop 上下文键白名单已含 `selected_clip_ranges`——循环组装层可见，**零扩展即可达 LLM 上下文** |
| D3 | `runtime/runtime.go:45` | `Goal` struct（GoalID/Task/Status/Summary…）**无结构化操作目标字段**——范围目标 v1 不扩 Goal struct（见问 1 决定） |
| D4 | `chat/goalrunner_chat.go:280-330,766,1916-1942` | 自由态轮状态机：`free_state_reasoning_loop`（chatContext 键）携带轮次；`freeStateLoopRoundPendingSettlement/ NeverActed/ SettleRefused` 分支在位——轮间编舞的挂点 |
| D5 | `chat/audio_closure_controller.go` | 轮 admit/record（`admitAudioClosureRound/recordAudioClosureRound`）——每轮取证与结算的执行面 |
| D6 | `agentloop/ccb_model_prompt.go:131/133` | D1-S1/多轮面单 mutation 规则原文——不动 |
| D7 | `tools/catalog.go:987`+RECON A2 | split_clip RiskConfirm+内核子 id 回传（left/right_clip_id）——**轮间交接对象的来源** |

## 问 1：范围操作目标进 goal——v1=chatContext 透传+loop 状态显式化，不扩 Goal struct

- **数据流现状已通**：框选 ranges 经白名单进 requestContext（D1）→ agentloop 组装可见（D2）。缺的不是"看得见"，是**协议层的显式引用**——两轮制要求轮 1 结束后轮 2 知道"操作目标=刚拆出的子 clip"。
- **设计决定**：`free_state_reasoning_loop` 增可选字段 `range_goal`（fail-open，旧 loop 状态无此键=普通 goal 零影响，AGENTS §11 兼容义务）：

```go
range_goal: {
  clip_id, track_id,          // 目标 clip（框选命中）
  range_start_s, range_end_s, // 全局时间界
  phase: "split_pending" | "split_done" | "treating",
  sub_clip_id: "",            // 轮 1 split 回传（RECON A2：right/left_clip_id）
  // split_done 后轮 2 的操作目标=sub_clip_id
}
```

- **为什么不动 Goal struct（D3）**：范围目标是**会话内协议状态**（随 loop 生灭），不是 goal 持久身份；写进 Goal struct 会引入持久化兼容义务且跨 goal 复用无场景。loop 状态已是 chatContext 持久投影（D4），字段随它走。

## 问 2：两轮制协议（轮次编舞）

```
goal 创建（框选在场+目标话术"把这段处理到不闷"）
  └─ range_goal.phase = split_pending
轮 1【拆分轮】
  mutation = clip.split ×2（先终点后起点，INTENT-WIRE 同序；贴界退化单切）
  post 取证 = ①结构核对：split 回传子 clip id + clip 清单时长=range 宽（确定性断言）
              ②听感基线：l2_render_probe range=[range_start,range_end]（RECON A11/A12——range 渲染在位）
  settle 形态 = 结构 settle：断言过→phase=split_done+记 sub_clip_id，自动进入轮 2；
               断言不过→blocked（拆分失败证据上交，不进轮 2）
轮 2【调改轮】
  mutation = treatment（参数变更或 A/B 双候选）——操作目标=sub_clip_id
  判定/结算 = 全复用既有链（JUDGMENT-SETTLE/AB-JUDGMENT 已转正面），零新建
  settle = 既有 settle 报告（实验材料性/目标响应/轮决策）
```

- **D1 合规论证**：每轮恰好一次前向 mutation（轮 1=split，轮 2=treatment），轮内无第二次——与 ccb prompt 131/133 现行文案**逐字兼容**，规则零改动即合规。
- **轮 1 的 settle 为什么不是判定**：轮 1 产物是结构事实（子 clip 存在且边界正确），可确定性断言，不需要听感判定——判定资源（用户耳朵/AB）留给轮 2 的 treatment。这是两轮制的效率设计。
- **轮间中止语义（诚实形态）**：轮 1 后用户停在 split_done（不自动进入轮 2）→ 子 clip 留存，goal 报告"已拆分待调改"，可用既有 undo 回退结构。**不自动 revert**——结构变更已过确认制，静默回滚反而制造不可见变更。

## 问 3：用户侧形态

1. **发起**：框选在场+目标话术 → goal 创建卡显示「范围操作目标：clip X 0:03–0:08 · 两轮制（先拆分，后调改）」——两轮预告在创建时可见（用户知情权）。
2. **轮 1 卡**：split 确认（RiskConfirm 两条，INTENT-WIRE 既有面）→ 结构 settle 报告卡（子 clip id/时长/基线探针产物引用）。
3. **轮 2 卡**：既有 treatment 确认卡+A/B 判定卡（零新建，JUDGMENT 链原样）。
4. **话术边界**：纯结构话术（"把这段拆出来"）不进自由态——走 INTENT-WIRE 普通确认制（意图层已接）；**带处理目标的话术**（"处理到不闷"“更亮一点”）才触发 range_goal 自由态编舞。分界词=是否存在 treatment 意图（复用既有 goal 话术分类先例，实锚 goalrunner 话术路由后定，预计 ≤20 行判定）。

## 问 4：边界与留位

1. **v1 显式不支持**：多 range（取首个+注记，其余忽略）、跨 clip 范围、note 会话发起（note 只读，范围 goal 只从主流发起）。
2. **B2 留位**：`phase` 抽象为"范围具现化"——将来 B2 落地时轮 1 的具现从"split"替换为"写包络"（phase 枚举已中性命名，协议形状不变）。
3. **快照统摄留位（2026-10-04 晚新输入）**：远期"范围快照"若落地，拆分+调改可缩为一次原子写入=单轮——本协议的 range_goal 字段设计不排斥该演进（phase 可加 "snapshot_written" 终态）。
4. **与意图层的关系**：INTENT-WIRE（普通确认制）与本协议（自由态）**共用同一 ranges 上下文源**（D1/D2），互不干扰——普通话术走意图层短路，目标话术走 goalrunner。

## 实现卡拆分建议（供决策侧出卡，本卡不开实现）

| 卡 | 内容 | 粒度 | 引擎 |
|---|---|---|---|
| FREESTATE-REGION-IMPL-1 | loop 状态 `range_goal` 字段+两轮编舞（split_done 自动衔接/断言 settle/中止语义）+测试 | 0.5 天，chat/goalrunner+audio_closure 域 | flash 可接（先例充分） |
| FREESTATE-REGION-IMPL-2 | 话术分界判定+goal 创建卡两轮预告+轮 1 报告卡 | 0.25 天，chat 域 | flash 可接 |

依赖：IMPL-1 → IMPL-2；真栈烟测=E2E-WEBUI-1 加组（范围 goal 创建→轮 1 卡形态）+用户手测（框选+目标话术→两轮走通）。

## 停止条件回顾

未触发：goalrunner 轮状态机（D4）与 split 命令面（D7）均支撑本协议，无接口缺失。
