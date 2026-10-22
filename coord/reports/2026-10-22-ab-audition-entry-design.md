# 设计稿：动态链完成后的 A/B 对照试听接通（UX-AB-AUDITION-ENTRY-1 / 池序 172）

- 产出：GLM-5.3 L2 PC 执行会话（ZCode，172 卡会话）/ 2026-10-22 晚
- 领取基线：origin/main=2b3a9b05（领取提交 coord/aba172-claim=28406f51）；主仓工作树=主管本地 main ee0050de
- 性质：勘察设计卡产出，零生产码改动；终案交主管与用户裁定

## 0. 结论摘要（TL;DR）

1. **既有面盘点结论：主要面全部可用，停止条件（全不可用）未触发**。内核 AuditionPreview 状态机、agent audition 事件链、webui 判定卡消费链、render 生产（start_render）、盲测面全部生产就绪；缺的只有一处——动态链执行腿零挂接（卡面定性实证成立）。
2. **终案推荐：主案（系统自动兑现）**，具体形态=在 `executeSemanticDynamicTicket` 内挂接 **mix-tick 形态的 audition bracket**（mutation 前拍 before render → 执行 → after render → mount 会话 → 发 `audition.ready` 事件），全程复用既有机制，零内核、零 webui 改动。四维对比见 §2；备案在触发确定性上与 159/170 已实证的「依赖模型确定性供应」缺口同族，不推荐。
3. **一处卡面预设需修正**：「对照粒度=被处理轨 bypass vs processed（FXM 既有形态）」在现有内核面**不可生产**——`start_render` 是整工程离线渲染（无轨级/插件 bypass 参数，TransportAudioService.cpp:1185 实证）。最小可感路径首版用**整工程 before/after 对照**（对「加空间感」类整曲效果完全可感）；轨级 bypass 对照需内核新参数，建议独立后置卡。**此修正需用户裁定确认**。
4. **判定落账是唯一需要新增的身份通道**：judgment POST 现有双路由（mix-tick handled 分流 → free-state fallback）都不认识动态链会话身份；free-state 校验绑定 `loop.Experiment.ID`（动态链必 409）。按 mix-tick 第三通道同模式扩展，不触碰实验身份校验（实验契约不变）。

## 1. 既有面盘点（可用性判定 + 证据锚点）

| # | 面 | 可用性 | 证据锚点 | 复用判定 |
|---|---|---|---|---|
| 1 | 内核 VitAuditionPreview | **可用（生产就绪）** | `VitApp/Source/Service/AuditionPreviewState.h`（Session/Candidate 状态机 preparing→ready→playing→stopped→stale→failed）；`AuditionPreviewService.h:21-28`（audition.select 自动开播+previewPlaying gate）；`VspKernelReference.cpp:130-133`（VSP 命令面 audition.prepare/status/ready/stale）；VitAuditionPreviewStateTests 在场 | 直接复用，零改动 |
| 2 | agent audition 事件链 | **可用** | `agent/internal/chat/audition_events.go:157` emitAuditionEvent→AgentEvent 流（webui 轮询消费）；`:592` prepareFreeStateAudition（实验形态）；`:560` judgment POST 双路由（mix-tick handled 分流→free-state fallback） | 直接复用；judgment 需扩第三路由（§3.4） |
| 3 | webui 消费链 | **可用（零改动候选）** | `agent/webui/src/audition.ts` reduceAuditionEvents（audition.* 事件→session 归约，含盲测/解盲/判定态）；`App.tsx:1531-1549` handleAuditionSelect/Stop；`lib/api.ts` selectAudition/stopAudition/submitAuditionJudgment；`trajectory/TrajectoryAuditionPanel.tsx:385-391` 逐 session 渲染 AuditionJudgeCard——**session 存在卡即出现**；AUDITION-UNSTICK-1 准备期/禁用原因 UX 在场 | 零改动：agent 发 audition.ready → 卡自动出现 |
| 4 | render 生产 | **可用（整工程粒度）** | `free_state_d1_render.go:99-136`（start_render：file_path/bit_depth=24/use_master_plugins，range=full_project→job_id→harness.WaitRender→wav+sha256/render_revision 溯源）；`mix_tick_audition.go` renderMixTickAuditionPhase（注释明示复用 D1 start_render 等待模式+低层 helpers verbatim 共享）；**约束=TransportAudioService.cpp:1185 handleStartRender 参数仅 file_path/range[startSec,endSec]/bit_depth/use_master_plugins，startOfflineRender(edit,…) 整工程离线渲染，无轨级/插件 bypass 参数** | 直接复用；轨级粒度需内核增强（后置卡） |
| 5 | mix-tick 无实验试听先例 | **可用（主案模板）** | `mix_tick_audition.go` runMixTickAuditionBracket（**before render fail-open → mutate() → after render → mount**；每步降级开放，「用户确认的小步绝不被人呈层挟持」）；mountMixTickAudition（**自造 session/turn/round 身份**+复用 d1AuditionCandidatesForAssignment+盲测抽签+AuditionPrepare）；判定走 mix-tick 自己的身份通道 | **主案完全复刻此形态** |
| 6 | 实验路径 prepareFreeStateAudition | 可用但强耦合实验状态机 | `audition_events.go:592-720`：sessionID=audition:experimentID:roundID，候选 A=checkpoint/B=treatment，D1S1 绑 before/after render（ensureD1Render），触发条件=EvaluationHumanAuditionReady；判定校验 `loop.Experiment.ID != request.TurnID → 409` | 复用其机制（候选构造/盲测/事件），不字面复用函数 |
| 7 | MOM ABResult 生产链 | 可用（数值面非试听面） | `mom/projection.go:284` buildABResultComparison、`mixboard/catalog.go:733` deriveABResult、`mixboard.go:3601` abRenderProbeDelta（L2 render probe 数值 delta）；消费=PendingMixTickReport/ProjectResultCard；run_ab_result_smoke.ps1 健康检查在跑 | 角色=判定落账后的数值佐证/结果卡，**不是可听对照的生产者** |
| 8 | FXM bypass/processed | 可用（测量面非可听面） | `fxm/types.go:27/105`（Processed Measurement 测量对形态）；不生产可听资产 | 轨级 bypass 对照的**语义模板**，非生产机制 |
| 9 | 盲测面 | **可用** | auditionBlindSettingsFor（env VIT_DAW_AUDITION_BLIND 优先→agent_runtime_config.json，逐会话读）；drawAuditionBlind 物理抽签；d1AuditionCandidatesForAssignment blindSwap（标签恒 A/B，物理交换）；webui blind/blindDisclosure 判定前中立措辞+判定后解盲披露 | mount 时同一配置面，零新增通道 |
| 10 | Godot 前端 | 不适用（卡面定性成立） | `D:/Godot/project/vit-daw-frontend/app/startup/wav_audition_loader.gd`=导入面；场次④会话=webui chat 面（webui_mvjh2p5b） | 效果对照呈现归 webui，Godot 零改动 |
| 11 | 动态链路径现状 | **确认零挂接（卡面定性实证）** | `semantic_dynamic_workflow.go` / `plugin_grabber_workflow.go` grep 0 处 audition；executeSemanticDynamicTicket（:856-945）settlement bracket 仅 D1 实验所有权（semanticDynamicSettlementOwner→IsD1S1 门），**C2 动态批/全访问会话无 bracket、无 render、无 audition**；完成面 `:948` StopReason=semantic_dynamic_execution_completed | 挂接点（§3.1） |
| 12 | 提示词承诺 | 在卷（空头支票定性成立） | `agentloop/full_access_autonomy.go:69`（fullAccessAutonomyDirective："execute…and hand the result to the user's A/B listening. In this mode the user's only duty is the A/B audition"）；`:82`（bounce text 同句）；`chat/full_access_semantic_chain.go:1-6` 引用户 2026-09-13 裁定原文「完全访问权限应该是它能够自主进行所有执行，我只负责AB试听」 | 主案兑现该承诺；提示词零改动 |

补充勘定：动态链执行腿的调用者族=c2_dynamic_batch.go:408 / c2_dynamic_control_runtime.go:144 / semantic_treatment_strategy.go:1104/1380/1688（147 全访问链经此）/ server.go:4524（确认续行）——挂接在 executeSemanticDynamicTicket 单点即覆盖全族。

## 2. 两案评估（四维对比 + 终案推荐）

**主案=系统自动兑现**：动态链 completed 且含效果类 mutation → 系统在执行腿内自动挂 mix-tick 形态 audition bracket。
**备案=模型显式试听动作**：新增试听工具/结构化回复字段，模型 completed 时显式请求 prepare，系统按请求挂。

| 维度 | 主案（系统自动兑现） | 备案（模型显式动作） |
|---|---|---|
| 触发确定性 | **高**：挂接在系统侧单点（executeSemanticDynamicTicket），与模型行为解耦——效果类 mutation 落地则试听面必然出现 | **低**：依赖模型每轮记得调用——159/170 系列已实证「依赖模型确定性供应」是本系统反复失血的缺口同族（A5 披露行非确定性供应）；漏调用=现状复发（用户再次拿到零试听） |
| 成本 | 双渲染（before/after 各一次整工程离线渲染，与 D1/mix-tick 同级——用户已在实验路径体验过此成本）+ 判定路由扩展 | 同级渲染成本 + 一次额外模型往返（工具调用轮）+ 新模型动作面（工具 schema/提示词修订）——改动面更大 |
| 提示词承诺一致性 | **完全一致**：「链末交付 A/B audition」由系统兜底兑现，提示词不再空头；与 full_access_autonomy 设计哲学（Never ask the user…execute the entire chain yourself + 运行时兜底）同构 | 形式一致（模型显式 hand off）但把兑现责任留在概率性行为上——把「承诺交付」变「承诺调用」，与现状同构 |
| 风险 | ①渲染时长延迟 completed 回复（解法=降级开放：渲染失败不阻塞执行与回复，degraded 原因如实披露，mix-tick 同款纪律）；②判定身份通道需扩展（§3.4）；③效果类触发条件需定义（§3.5）；④与实验会话命名域隔离（§3.6）——均有明确解法 | 模型漏提供风险无兜底；工具面本身也要系统侧实现（成本并未省），只多不少 |

**终案推荐：主案**。触发确定性与承诺一致性两维压倒性优势；成本同级；风险均有先例解法。备案唯一优势（模型可在纯观察轮省略试听）可作为主案的**抑制豁免**后置二期（结构化回复声明 skip_audition），首版全触发——不引入概率性依赖。

## 3. 最小可感路径设计

### 3.1 触发时机：bracket 挂执行腿（非 completed 事件后）

在 `executeSemanticDynamicTicket` 内、PCA admission 与 surface/拓扑校验通过后挂 bracket，完全复刻 mix-tick 时序：

```
PCA admission + surface/topology 校验通过
  → before render（start_render @ 写前 revision；失败=fail-open 继续执行，记 degraded）
  → applySemanticDynamicController（typed controller 写参数，既有逻辑零改动）
  → after render（start_render @ after_revision）
  → mount audition session（复用 d1AuditionCandidatesForAssignment + 盲测抽签 + AuditionPrepare）
  → audition.ready 事件随 semanticDynamicExecutedResponse 同轮发出
```

**为什么挂执行腿而非 completed 事件后**：before render 必须在参数写前拍（时序不可逆）；写后回退拍 before 需两次额外状态变更且有内核状态漂移面（D1 settlement bracket 的 drift 复核就是防此族）。单点挂接覆盖全部四类调用者（C2 批/C2 运行时/147 全访问链/确认续行）。

### 3.2 资产生产：同步 bracket 首版基线，异步 after 为降级备案

- **基线=同步 bracket**（mix-tick/D1 同款）：completed 回复前完成双渲染。渲染失败降级开放（degraded 原因写进回复与日志，不承诺不存在的入口——mix-tick「用户确认的小步绝不被人呈层挟持」同款纪律，这里换主语为「自主链的执行」）。
- **降级备案=异步 after**：before 同步（mutation 前必然要等），after 在回复后异步渲，完成发 audition.ready（webui 事件驱动本就支持，零前端改动）。仅当真栈实测整工程渲染时长不可接受（回复延迟体感差）时启用——实现卡内按实测数据定，不预先复杂化。
- **不采用「用户点按钮才生产」的惰性形态**：before 时机不可逆（参数已写），惰性=回退重渲染（漂移面+双倍状态变更），且判定链（轨迹回合组锚定）依赖会话与 mutation 同轮建立。

### 3.3 入口呈现：webui 轨迹流判定卡（零 webui 改动）+ 与 163/171 协调

- **唯一持久入口=TrajectoryAuditionPanel 判定卡**：audition.ready 事件→session 归约→卡自动出现（A/B 选择播放/停止/判定提交/盲测中立措辞/解盲披露全链在卷）。**webui 零改动**。
- **与 163（PlanBar）协调**：PlanBar terminal 态 6.4s 后自卸载（planBarDwellMs=6000+fade=400）——**试听入口不挂 PlanBar**（会被时间清场），PlanBar 契约不动。
- **与 171（send 拓扑呈现链）协调**：171=Godot 前端仓（rack 视图），172 首版=agent Go 侧（零前端/零 webui 改动）——文件域天然不相交，**无依赖，可并行**。
- **Godot 面**：零改动（wav_audition_loader 是导入面；效果对照呈现归 webui chat 面——场次④实证用户会话在 webui）。

### 3.4 判定落账：第三身份通道（唯一新增服务端面）

judgment POST 现有路由（audition_events.go:560）：先 mix-tick（handled 分流）→ fallback free-state（校验 `loop.Experiment.ID`，动态链会话必 409 "experiment session not found"）。动态链需要按 mix-tick 同模式扩展第三通道：

- 会话身份自造（不碰实验命名域）：`audition:dynamic:ticketID` / `dynamic_turn:…` / `dynamic_round:…`（类比 mixTickAuditionSessionID 族）；
- 判定落账到 conversation/trajectory（user_judgment 节点），不进实验任务状态机（human_judgment_required 状态机是实验的，动态链没有也不需要）；
- 建议与生产挂接同卡实现（判定不可用=只能播不能判的半功能）。

### 3.5 效果类 mutation 触发条件：首版=executeSemanticDynamicTicket 成功单条件

这条腿本身就是语义参数控制（EQ/压缩/混响/时间效果全族经此）；「只装载不调参」的链（plugin_grabber 装载后无参数写腿）首版不触发，如实记录。场次④形态（send connect+reverb 装载→4s 内参数写腿 completed）在此条件下**覆盖**。白名单细化/模型豁免（skip_audition）留二期。

### 3.6 实验契约不变（红线核对）

- prepareFreeStateAudition / loop.Experiment / D1 settlement 链零改动；
- 会话命名域隔离：audition:experimentID:roundID（实验）/ audition:mix_tick:tickID（mix-tick）/ audition:dynamic:ticketID（新）——三者互不冲突；
- 盲测配置同一来源（auditionBlindSettingsFor），实验路径盲测语义零影响；
- 复用件（ensureD1Render 低层 helpers、d1AuditionCandidatesForAssignment、AuditionPrepare、emitAuditionEvent）按 mix-tick 先例 verbatim 共享，不复制不修改。

## 4. 分卡建议

| 卡 | 内容 | 文件域 | 验收 | 真栈腿 |
|---|---|---|---|---|
| A（主实现，生产+判定同卡） | 动态链 audition bracket 挂接+第三判定通道 | `agent/internal/chat/semantic_dynamic_workflow.go`（executeSemanticDynamicTicket 挂点）+ 新文件 `agent/internal/chat/semantic_dynamic_audition.go`（bracket/mount/身份，mix_tick_audition.go 模式）+ `audition_events.go`（判定路由扩展邻域）；零内核/webui/Godot 改动 | 单测：bracket 时序（before fail-open 不阻塞执行/after 失败降级/mount 失败降级）+身份域隔离+盲测复用+**mix-tick 与 D1 既有测试零回退**（run_ab_result_smoke.ps1 + mix_tick/d1 测试族全绿）+全量 go test 0 FAIL | 真栈 chat 腿：场次④话术复刻「给人声加空间感」→completed 后 audition.ready 事件在卷+webui 判定卡出现+A/B 选择播放+判定 POST 落账 exit 0；PC-RUNTIME-STACK 登记 |
| B（可选后置，独立裁定） | start_render 轨级/插件 bypass 参数（FXM 粒度对照） | VitApp TransportAudioService + VspKernelReference + agent render 调用邻域 | 内核 VitAuditionPreviewStateTests 扩展+轨级对照冒烟 | 真栈渲染腿 |
| C（可选后置） | skip_audition 结构化豁免+白名单细化 | agent chat 邻域 | 单测+真栈对照 | 免真栈（纯语义面） |

**卡间依赖**：A 无依赖（与 171 并行，文件域不相交）；B/C 依赖 A 合入且用户裁定确认。

## 5. 需用户裁定的点

1. **终案=主案（系统自动兑现）**：按 §2 四维对比推荐；备案不推荐的理由=159/170 同族概率性供应缺口。
2. **对照粒度修正**：首版=整工程 before/after（内核 start_render 无轨级参数面，§1#4 实证）；卡面预设「被处理轨 bypass vs processed（FXM 粒度）」转后置卡 B。对「加空间感」类整曲效果，整工程对照可感性足够（用户需求=一键听到 before/after）；若用户坚持轨级粒度，卡 B 升为一期。
3. **同步 bracket 基线**（completed 回复等双渲染，mix-tick 同款成本）vs 异步 after 降级——授权实现卡按真栈实测时长自定降级线。

## 6. 勘察边界声明

- 本卡零生产码改动（工作树唯一新增=本设计稿）；全部结论来自代码阅读+grep 锚点，无运行时取证（无真栈 run）。
- 渲染时长未实测（同步/异步形态的降级线留给实现卡真栈腿）。
- webui 判定卡呈现的 judgmentRequested 置位依赖 trajectory.user_judgment.requested 事件与否——实现卡需核对动态链会话是否需要补发该事件（卡本体出现不依赖它，但判定提交按钮的 pending 态语义要一致）；锚点=audition.ts reduceAuditionEvents + TrajectoryAuditionPanel pending 判定。
