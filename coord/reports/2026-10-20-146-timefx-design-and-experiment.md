# 146 设计定稿建议 + 软引导实验方案（§8 预写，交主管确认）

- 前置：取证结论见 [2026-10-20-146-timefx-bus-forensics.md](2026-10-20-146-timefx-bus-forensics.md)（下称《取证》）
- 性质：本件为设计建议与实验协议预写；**实验未跑**——按卡面实验阶梯第 3 层，方案经主管确认后另腿执行（占真栈须登记 `coord/resources/PC-RUNTIME-STACK.md`）

## 一、软引导实验方案（SEMANTIC-TIMEFX-SOFTGUIDE-EXP-1）

### A. 实验问题与隔离边界

**问题**：在系统提示词声明「时域族默认总线拓扑」惯例后（观察披露维持现状），模型能否把时域族请求自行路由为总线拓扑提案？

**隔离边界**（据《取证》①②划定）：
- 只测**路由面**（提案拓扑形态），不测门——每变体以新会话+观察前置话术运行，规避 L0 延续陷阱与 L1 结转缺口；
- 回复面 L3 缺陷与 L1 首轮观察缺口**不在本实验修复/验证域**，其修复另行立项（见 §三卡 E/D）；
- 交互卡在 full access 下仍会弹出（F1/147 未修）——由驱动脚本自动应答（选第一候选/approve）并留痕，不引入人工点击方差。

### B. 运行环境（跑前登记）

- 真实三件套：`dev_agent_smoke -StartKernel -StartUI` 模式自起（AGENTS §5）；fixture=spv1_p06 六轨（bass/drums/guitar/other/piano/vocals，与 M2 场次同源）；模型=deepseek-flash 思考档；authority=full_project_access。
- 驱动：python NL 驱动（b9_free_state_chat_trace_smoke.py / FS-LARGEPROJECT-SMOKE nl_chat 模式同族），每变体一个新会话（新 conversation/goal），自动应答交互卡。
- 工件/变体：conversation_graph.json、agent_last.log 切片、遥测行、mixboard_interaction_diag.jsonl、运行后 `track.send.list` 读回、loop 状态 json；run 目录 `coord/runs/146-SOFT-GUIDE-EXP/<run_id>/`。
- B2 臂的惯例文本经**一次性实验分支补丁**注入 `internal/agentloop/ccb_model_prompt.go` 组装处（临时 build，不入生产；生产注入走卡 A）。

### C. 话术集（10 变体，Tier A 族词显式 / Tier B 全模糊；前缀中性）

前缀模板（准入门先过+轨道身份可获，不含任何族/拓扑词）：**「请先观察工程，再给一个有界的小步改进建议：」**

| # | 变体话术（接前缀后） |
|---|---|
| A1 | 给人声轨加一点混响，衰减短些，不要动整体增益或 EQ |
| A2 | 给钢琴加一点混响 |
| A3 | 给鼓组加一点房间感（room） |
| A4 | 给这条吉他轨加一个轻微的延迟 |
| A5 | 给人声加一点板式混响 |
| B1 | 给人声加点空间感（M2 场次原话术） |
| B2 | 这条钢琴听着太干了，帮我弄润一点 |
| B3 | 让鼓组听起来更有空间 |
| B4 | 人声有点贴脸，往后放一点 |
| B5 | 让这条吉他有点远近层次 |

### D. B2 软引导惯例文本（草案，注入系统提示词）

「时域效果惯例（默认拓扑）：当计划的处理属于时域族（reverb/delay/echo/room/hall，及空间感/氛围/远近/干湿类请求），且用户未明说插入方式时，默认规划为总线拓扑：效果器装载到一条 FX/return 轨（新建时默认命名 Reverb Bus / Delay Bus，工程已有可用时域 FX 轨时复用之），源轨以 send 连接该轨并设默认发送量；inline 插入源轨仅当用户明说插入、单轨快速 A/B 或实验探测。提案必须呈现拓扑计划（建什么轨或复用哪条、装载位置、send 连接与默认发送量）。只能引用观察披露过的真实轨道与结构，不得发明轨道或拓扑。」

### E. 运行次数 / 成功条件 / 失败分类 / 止损（§8 预声明，跑前不得改）

- **臂与次数**：B2（惯例 ON）=10 变体×1 run；B1（对照，现状提示词）=A1/B1/B2/B3/B4 共 5 run。合计 15 run（LLM 参与面；环境中断须原始日志/退出证据方可出有效轮次，AGENTS §8）。
- **主判据（B2）**：≥8/10 run 的**提案**（提案卡/typed controller/终答计划文本）含总线拓扑：装载位置=FX/return 轨（新建或复用）且源轨 send 连接在计划内——非 inline 于源轨。
- **硬判据（两臂通用）**：0 幻觉拓扑（提案引用的轨/send 目标必须存在于观察披露或本提案新建清单；运行后 track.send.list 读回亲核）。任一幻觉=该 run 硬失败。
- **失败分类**：F-inline（inline 装载源轨）/F-notopo（提案无位置主张）/F-halluc（幻觉拓扑，硬失败）/F-gate（未达提案：门拒或自报边界——含 target_identity 类）/F-env（环境中断，须证据）。
- **分母规则**：F-gate 计入失败（生产同样是失败），单列计数。
- **止损 ×2**：①同签名失败连续 ×2（如 2×F-inline）→ 停，收口报告；②F-gate 累计 ×2 → 停（实验前提破：前置话术不足以稳定过门——上交 L1/L3 修复优先，实验在其后重排）。
- **判读→第 4 层厚度**（卡面实验阶梯 4 预绑定）：B2 ≥80% 且 0 幻觉 → 硬规则只守写门（确认时拓扑校验）；<80% 或任何幻觉签名再现 → 硬规则接管默认拓扑计划生成（确定性模板，136 posture：软的管解释、硬的管落笔）。
- B1 臂不设通过线，仅作归因对照（B2−B1 差分=惯例文本净效应）。

## 二、设计定稿建议（待主管裁定后拆卡）

### 1. 注入点三件套（对应《取证》② 推论）

- **软（解释与路由引导）**：`internal/agentloop/ccb_model_prompt.go` 自由态系统提示词加入 §一.D 惯例段（词表复用 `c3SpacePluginPathMarkers`/`c3SpaceParamMarkers`，抽公共引用避免双份维护）。例外路由解释（inline 三场景）同段声明。
- **硬（落笔面）**：**拓扑计划对象**（新）：

```go
type semanticTopologyPlan struct {
    Mode      string  // "bus" | "inline_exception"
    FXTrack   fxTrackRef // {action: create|reuse, name, track_id}
    Loads     []loadRef  // {plugin_id, track: fx_track}（inline 例外时=源轨+reason_code）
    Sends     []sendRef  // {source_track_id, default_amount_db}
    Exception struct{ Code string; Reason string } // user_explicit | single_track_ab | experiment_probe（inline 时必填）
}
```

  组装点=`plugin_recommendation.go` 提案组装处（装载目标从 selected_track_id 直通改为消费 topology plan；inline 例外须带 reason code）；呈现=提案确认卡拓扑计划区（建什么轨/连谁/默认发送量——卡面合同遵 148 修复后的指名合同）。
- **硬（写门）**：确认/执行时拓扑校验（引用轨必须存在于观察披露或本提案新建；send 目标必须存在）+ `track.send.connect` 执行接线（K1/K2 命令面，无需内核改动）。厚度由 B2 实验判读决定（§一.E）。

### 2. 默认规则（确定性模板，B2<80% 时启用全量）

时域族词表命中（用户请求或所选族）且无显式拓扑指示 → bus 模板：复用既有可用时域 FX 轨或新建（默认名 Reverb Bus/Delay Bus）→ 装载 → 源轨 send.connect（默认发送量 −6dB 起步，写入提案供用户改）→ 提案卡呈现完整拓扑计划。inline 仅凭例外码放行（§《取证》④ 清单）。

### 3. 接口位与不动项

- 129 宏卡接口位：topology plan 的 fx_track/sends 字段预留 binding 目标（宏绑 send level 已在 K3 验证过链路），本线不实现；
- 观察侧（C3）零改动（已能读）；入口分类器（semantic_entry.go）零改动（内容盲契约保持——惯例不进门）；
- prompt 契约 posture 同 136：模型获知惯例、解释与遵循已给选项，不得发明拓扑。

### 4. 前置依赖（非本卡域，阻断项如实声明）

| 依赖 | 缺口 | 影响 | 建议归属 |
|---|---|---|---|
| D-回复面 | boundary=proposal_missing 时 failed_gate_ids 劫持文案（goalrunner_chat.go:2098-2107） | 模糊意图失败时用户面仍是误导性门代号；软引导实验的 F-gate 判读也依赖诚实文案 | 并入 147 族或单独小卡（改动面极小：按 receipt.boundary 选分支） |
| E-观察强制/结转 | 新循环 ledger 零结转+无首轮强制观察（free_state_reasoning_loop.go:265-270） | 惯例只在"模型自觉观察"场景生效（延续式模糊意图仍断） | 单独探查+设计卡（涉 FS 状态机语义，建议先行） |

## 三、实现卡拆分建议（文件域+验收面）

| 卡 | 内容 | 文件域 | 验收面 |
|---|---|---|---|
| A（146-IMPL-SOFT） | 软引导惯例段进自由态提示词+词表抽公共 | `agent/internal/agentloop/ccb_model_prompt.go`、`agent/internal/chat/c3_space_depth_runtime.go`（词表导出，只动导出不动逻辑） | 单测（提示词组装含惯例段+例外码）；B2 实验腿复跑判读 |
| B（146-IMPL-TOPO-PLAN） | 拓扑计划对象+提案卡拓扑呈现+装载链消费（去 selected_track_id 直通） | `agent/internal/chat/plugin_recommendation.go`、`agent/internal/chat/semantic_treatment_strategy.go`（plan 结构） | 单测+真栈烟测（提案卡含拓扑计划；扩展 dev_agent_smoke 或同模式新腿）+渲染面边界声明（AGENTS §5） |
| C（146-IMPL-WRITE-GATE） | 写门拓扑校验+send.connect 执行接线 | `agent/internal/chat/send_route_workflow.go` 邻域、执行面（vsphub 派发处） | 真栈烟测 exit 0（建轨/连接/读回/幻觉拒载四断言） |
| D（146-IMPL-REPLY-FACE） | 回复面 boundary 分层修复 | `agent/internal/chat/goalrunner_chat.go`（freeStateCapabilityBlockedReply） | 单测（proposal_missing→summary 分支）+真栈 chat 腿 |
| E（146-IMPL-OBSERVE-FORCE） | 新循环首轮强制观察/ledger 结转设计+实现 | `agent/internal/chat/free_state_reasoning_loop.go`、`agent/internal/agentloop/`（FS 语义，先探查后定） | 单独探查卡先行；验收含旧循环延续回归（§11 持久化兼容） |

**优先序建议**：D（最小、独立、先解用户面误导）→ A（软，可与 B2 实验腿合并验收）→ B/C（硬，C 厚度由 B2 判读绑定）；E 单独排期（涉 FS 语义，不与 B/C 同批）。B 与 C 的文件域与 147（plugin_recommendation.go:536-640 权限分叉）**相交**——按 §2.1 领取前核对，建议 147 合入后再领 B。

## 四、诚实边界

- 本设计基于单场次行为证据（M2 场次①）+静态代码取证；B2 实验前的行为断言（模型能否自行路由）**未经检验**——这正是实验的目的。
- 「零层持有拓扑决策」结论覆盖 chat/agentloop/semanticorchestrator/planner 四层的现行实现；若存在未读到的旁路装载路径（如 UI 直操作），不在本取证范围。
- 例外面第 5 条（复用既有 FX 轨）依赖 C3 披露质量，实验变体未覆盖"已有总线"场景（fixture 无既有 FX 轨）——留作 B/C 卡烟测场景补。
