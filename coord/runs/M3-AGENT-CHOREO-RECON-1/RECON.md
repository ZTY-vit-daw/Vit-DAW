# M3-AGENT-CHOREO-RECON-1：M3 步骤④勘察报告——意图层包络指令+范围 goal 编舞单轮终态+披露面解锁现状

- 卡：coord/cards/（池序 197）；执行：ZCode GLM-5.3 PC 执行会话 @ D:\Vit_DAW / 2026-10-27
- 基线：origin/main=2fa99bd3（领取基线）；领取提交 c1c6a63c（doing）
- 方法：grep+读源码（主仓 agent/ 只读、VitApp/ 只读、前端仓只读），零代码改动、零真栈启动、零探针；全部锚点当日亲核，标 文件:行号
- 端测边界声明：纯只读勘察零代码零真栈，不适用端侧烟测门槛
- 结论一句话：**披露面四处 Excluded 与 catalog 三工具现状与 M3-RECON/M3-KERNEL-1 记录零漂移（工具已在册且普通回合已可调，缺的是指引与 Excluded 摘词）；意图层对 automation 只有只读计数词表（写入词表与时变限定词零存在）；FREESTATE 保全件完整可读、其揭示的发令面三缺口今日全部仍在——单轮终态编舞的必要条件是 D1-S1 域表扩 automation 动作行；DAD 曲线回流断在内核零生产者（agent 侧管道完备但休眠）**

---

## 一、问1：意图层包络指令词表现状+「这段处理到 X」映射缺口清单

### 1.1 词表盘点（M3-RECON §4 问 4 锚点核对+186-195 新增面）

| 面 | 锚点（现址） | 事实 | 与 M3-RECON 对比 |
|---|---|---|---|
| goalrunner 话术 | `agent/internal/chat/goalrunner_chat.go`（全文件 grep automation=**0 命中**） | 零 automation/自动化/包络触发词 | 不变（RECON §4.2 同款零命中） |
| 波形包络词族 | `agent/internal/agentloop/message_loop.go:9626-9627`（`waveform_envelope`/`track_waveform_envelopes`） | 波形包络（展示域），非参数自动化 | **锚点漂移**：文件由 internal/chat/ 迁至 internal/agentloop/，行号不变 |
| 黑板词表 | `agent/internal/agentloop/project_blackboard_report.go:94`（"包络"） | 名词词表（报告域），非触发词 | **锚点漂移**：同上迁移，行号不变 |
| 本地意图合成 | `agent/internal/conversation/intent.go:16`（SynthesizeLocalDAWCommands） | 动词族=track.add/mute/solo/import/clip.remove/split/clone/move/resize/select——**零 automation.* 写动词** | 新核（RECON 未单列） |
| treatment 意图路由 | `agent/internal/agentloop/message_loop.go:4867`（messageLoopTextHasAny：处理/调整/更亮/…） | treatment 类话术→closure machine（静态处理路径） | 新核 |
| **只读计数面（新）** | `agent/internal/chat/capability_routing.go:148`（自动化/automation→"automation"）+`:170`（AutomationLaneCount 计数答） | AGENT-F3 保守短路：仅「多少/有没有」疑问+事实名词，**无诊断/处理意图** | **186-195 后新增**（RECON 时零命中，现有一处只读路由） |
| 队列标签（新） | `agent/internal/chat/mix_workflow_queue_view.go:91`（C4 "段落自动化"） | 清理规划 C4 分类在队列视图的呈现标签 | 186-195 面新增 |
| 需求侧实证 | `agent/internal/chat/c1_frequency_cleanup_planner.go:31`（schema：defer_automation） | 模型已能产出「该用自动化」判定而无执行出口 | **锚点漂移**：文件由 internal/frequencycleanup/ 迁至 internal/chat/，行号不变 |
| 相邻能力词面 | `agent/internal/chat/clip_fade_gain_guard.go:22-24`（fade/淡入/淡出/增益） | 淡入淡出词路由 clip.fade（clip 边界淡变工具）——**非参数自动化**，词面撞车源 | 新核 |
| 时变限定词 | agent/ 全仓非测试 grep 慢慢/渐强/渐弱/逐渐/ramp=**0 命中** | 意图层无任何时变语义词 | 新核 |
| 域表 | `agent/internal/experiment/d1s1_domains.go`（723 行） | 零 automation/包络/曲线行动行 | 与 RECON 一致（无变化） |

### 1.2 「这段处理到 X」→包络指令的映射缺口清单

现状该话术走向：treatment 动词命中（message_loop.go:4867）→自由态 closure machine→D1-S1 标量参数静态写入。缺口按链路序：

1. **时变语义不可见**：无时变限定词表——「这段低频慢慢降下来」与「把低频降下来」在意图层不可区分，前者本应是包络指令却落静态写入。
2. **写入动词缺位**：intent.go 本地合成与 goalrunner 武装词表均无「画/写/加 包络/自动化」动词族。
3. **判定出口断裂**：planner 的 defer_automation 分类（:31）无动作出口——RECON 已记，今日仍断。
4. **路由指引缺位**：`chat/server.go:6198-6212` 路由指引段有 clip.fade/set_plugin_param/mix 专行、**零 automation 行**——模型收到包络类请求无既定路由（且「淡出」词面有 clip.fade 撞车先占）。
5. **能力视图矛盾**：四处 Excluded（问3）说能力面不含 automation，而工具面已在册可调——指引与能力矛盾。

## 二、问2：goal 编舞单轮终态设计输入

### 2.1 保全件复用评估（runs/FREESTATE-REGION-IMPL-1-STOP/，5 文件全部在场可读，停止条件未触发）

| 保全面 | 件 | 单轮终态下的处置 |
|---|---|---|
| range_goal 状态字段 | free_state_range_goal.go:33-40（struct，omitempty fail-open） | **直接复用**——clip/track/range 界字段与单轮编舞同构 |
| 相位单调守卫 | :50-61 freeStateRangePhaseRank+diff 中 mergeFreeStateLoops 补丁 | **直接复用**（枚举值重定义后守卫逻辑原样） |
| 旧 JSON 兼容 | 测试 TestFreeStateRangeGoalOldStateJSONDecodesFailOpen/RoundTrip/MergePhaseNeverRegresses（:58/:82/:109） | **直接复用**（fail-open 语义与相位值无关） |
| 武装判定 | :68-107（treatment 动词族+ranges 在场最简门） | **复用+扩**——动词族保留，需加时变限定词维度（问1 缺口1） |
| split 回执几何断言 | :117-257（freeStateSplitReceipt/Outcome：两切序/贴界退化/容差 0.02s） | **按单轮终态重写**——断言对象从 clip.split 回执换为 automation.set_points 回执+get_curve 逐点读回（回执字段 mode/written/point_count/region echo 在 catalog.go:896 OutputSchema 在册） |
| 轮1 settle 挂点 | applyFreeStateRangeSplitOutcome（挂 recordFreeStateDecision 后） | **改写**——挂写回执后，settle=写回执+读回断言（结构 settle 性质同源：确定性断言不占判定资源） |
| 轮2 衔接 | applyFreeStateRangeGoalTreatmentTarget（:328-344，sub_clip 绑 TargetRef） | **删除**——单轮无衔接 |
| 轮间中止注记 | :350-373（「已拆分待调改」） | **改写**——单轮终态注记（如「已写入包络待判定」），保留零自动 revert 姿态 |
| 测试骨架 | 10 组测试（:58-367） | 3 组兼容类原样+7 组按新相位/新断言对象改写 |

补丁可套性：impl_tracked.diff 基线 de833667，今日 HEAD 挂点函数全部在位但行号漂移（recordFreeStateDecision :1003/1105→:1193；struct :182→:221）——**逻辑可复用、字面不可直套**，按函数名重放。

### 2.2 goalrunner 挂点锚点核对（DESIGN D1-D7 + STOP-NOTE 三缺口）

| 锚 | DESIGN/STOP 记录 | 现址 | 判定 |
|---|---|---|---|
| D1 ranges 白名单 | server.go:2171-2175 | server.go:2458（+5466-5478 校验族） | 在位，行漂移 |
| D2 agentloop 上下文键 | helpers.go:143-145 | helpers.go:145 | 在位 |
| D3 Goal struct 无结构化操作字段 | runtime/runtime.go:45 | internal/runtime/runtime.go:45 | 在位（零漂移） |
| D4 轮状态机分支族 | goalrunner_chat.go:280-330,766,1916-1942 | :280-330 核心族在位（:302 prepareFreeStateReasoningContext、:306+ free_state_internal_resume/PendingSettlement 分支）；轮函数族迁至 **free_state_experiment_runtime.go:970/990/1010/1038/1062**（SpentMutation/PendingSettlement/SettleRefused/OwesIntervention/NeverActed） | 部分漂移（文件迁移非重构，未触发停止条件） |
| D5 轮 admit/record | audio_closure_controller.go | admitAudioClosureRound:453/recordAudioClosureRound:559 | 在位 |
| D6 D1 prompt 单 mutation 规则 | ccb_model_prompt.go:131/133 | :131 段在位（单轮终态编舞写=唯一前向 mutation，与该规则同构兼容——每轮恰一次的极限即单轮一次） | 在位 |
| D7 split 回传 | catalog.go:987 | **catalog.go:1133**（目录增长行漂移，spec 同形 RiskConfirm） | 在位，行漂移 |
| 缺口①loop 内披露面 | goalrunner_chat.go:3519-3531（仅 ccb 观察） | **:4137-4145**（agentLoopToolContext：freeStateLoopActiveContext→仅 ccb.observation_catalog/request） | 仍在，行漂移 |
| 缺口②invoke guard | free_state_invoke_guard.go:14-43（MutatesProject 一律拒） | :14-43 在位（automation.set_points=MutatesProject=true 必被拒） | 仍在 |
| 缺口③D1-S1 域表动作种类 | free_state_d1_plan_table.go:636-638（插件/参数域标量绑定） | 域表=internal/experiment/d1s1_domains.go（:58 D1S1DomainSpec；行族 :128-641 全部**标量 gain 参数写入绑定** D1S1WriteBinding） | 仍在——**且为结构性缺口：automation 行需点数组载荷，非加一行标量行可解** |

### 2.3 原 DESIGN.md 过时条款清单（186-195 落地后）

1. 问 2 两轮制协议整体（轮1 split/轮2 treatment）——**作废**：用户 2026-10-04 裁定（b8a7e27e）推迟 B2 批次按单轮终态实现。
2. 问 3 用户侧形态（两轮预告卡/轮1 报告卡/轮2 既有卡）——**作废**：单轮终态无轮间；写前确认卡=RiskConfirm 通用面已在。
3. 问 1 range_goal 字段设计（不扩 Goal struct、随 loop 走）——**保留有效**：单轮终态同样只需要会话内协议状态。
4. 问 4 条 2 B2 留位（「轮 1 具现从 split 替换为写包络」）——**转正为主设计**：具现=写包络，但不再是轮 1 而是唯一一轮。
5. 问 4 条 3 快照统摄留位——**提前兑现**：单轮写包络即快照统摄的最小形态（phase 加终态枚举的设计预留直接启用）。
6. 问 4 条 1 v1 显式不支持清单（多 range 取首/不跨 clip/note 不发起）——**保留有效**（单轮形态约束面更小）。
7. 新增事实（186-195）：呈现面已有车道三态+在场性自动展开（LANE-AUTO）+closeup（200 在飞）——DESIGN 问 3 的「用户侧形态」在 Godot 侧已大半存在，编舞卡的呈现义务收窄为 webui/goal 摘要面。

## 三、问3：披露面解锁清单

### 3.1 四处 Excluded 现状（M3-KERNEL-1 裁定 5 摘除点）

`agent/internal/capabilitycontext/context_manifest.go`——**四处零漂移，与 M3-RECON §4.2 记录逐字一致**：

| 行 | 视图 | Excluded 内容（automation 所在行） |
|---|---|---|
| :215 | （B1 域） | clip gain/plugins/pan/**automation**/raw waveform arrays/spectrogram tiles/full TOM tree/direct track.volume |
| :257 | （B2 域） | track fader/clip gain/plugins/stereo width mutation/**automation**/… |
| :301 | （B3 域） | clip gain/plugins/stereo width/**automation**/raw waveform arrays |
| :336 | （诊断域） | dynamic EQ/compression/space processing/**automation**/vendor parameter rules/… |

解锁动作=四处摘词+各视图 Guidance 是否补 automation 指引行（披露契约变更=显式行为，M3-KERNEL-1 裁定 5 已预留本卡解锁）。

### 3.2 catalog 三工具与 argspec 现状复核（M3-KERNEL-1 已落，全旗在位）

`agent/internal/tools/catalog.go`：

- 别名：:78-80（automation_get_curve/set_points/clear_region）。
- `automation.get_curve` :862-882：RiskDirect，必填 plugin_id+param_id，OutputSchema=points[{t,v,c}]+bypass+point_count+time_base（引擎 TimeBase 原样披露）。
- `automation.set_points` :884-924：**RiskUndoable+MutatesProject+SupportsUndo+RequiresConfirmation+RefreshAfter 全旗**；points 数组 minItems 1（t≥0、c∈[-1,1] 缺省 0、v 需在 valueRange 内）、可选 region[start,end]（区域替换语义：区域内点移除+新点须全在域内）、fail-closed 整令拒绝。
- `automation.clear_region` :926-955：同写旗族；region 必填、无整曲线形态（宽区域承担）。
- harness pre-journal：`harness.go:1079-1089` validateAutomationWrite（写命令参数校验+失败留痕 logPreJournalInvokeFailure）。

**可调用性现状**：普通（非自由态）回合 AllowedTools=toolNamesForAgentLoop（goalrunner_chat.go:4171-4192，无 automation 过滤）
——**模型今日已能在普通回合调用三工具**；自由态 loop 活跃时被 :4137-4145 收窄为仅 ccb 观察。

### 3.3 解锁后 agent 侧最小演示路径（纸面推演）

1. 用户（webui，普通模式）：「给这条轨 Supermassive 的 Mix 参数画一条 30s 到 60s 从 0.2 涨到 0.8 的包络」。
2. 话术无本地合成/事实短路命中（问1 词表盘点）→ 进普通模型回合。
3. 模型 catalog 可见 automation.*（3.2）——**但路由指引零 automation 行（server.go:6198-6212），模型行为不保证**（解锁项：补指引行，对位 :6203 clip.fade 行式样）。
4. 模型先 get_plugin_parameters 读 valueRange → automation.set_points（pre-journal 校验）→ RiskConfirm 确认卡（webui 通用确认面）→ 用户确认 → VSP 信封（base_revision=plugin state 级，裁定 2）→ 内核 CommandDispatcher.cpp:3131-3141 handler（亲核在位）→ PluginRackControlService 写曲线。
5. 回执 mode/written/point_count → 模型 automation.get_curve 逐点读回（RiskDirect+renderAllowlist）。
6. Godot 侧：auto 档车道在既有重扫触发（目录变化/进档）时展开呈现 envelope_point_count（190 字段）。

**链路缺口（即解锁清单）**：①四处 Excluded 摘词（3.1）；②server 路由指引补 automation 行+淡出词面与 clip.fade 的分界指引；③M3-KERNEL-1 回执自认「未走 /agent/invoke 驱动新工具（authority 面不在本卡腿内）」——**agent 侧全链真栈零证据**，解锁卡须含真栈腿。

## 四、问4：编舞呈现面

- **webui 现状**：通用轨迹块 TraceBlock（`webui/src/App.tsx:3288-3300`，自由态实验块跟随+判定卡挂靠）+确认卡（:5078 confirmationReviewItems/:1334 is_confirmation）+TurnReceiptRow。**零编舞专用卡**（DESIGN 问 3 形态从未建——两轮制作废后其单轮退化形态也未建）。
- **Godot 现状**：车道三态+在场性自动展开（186-195）+特写窗曲线（200 在飞）——**曲线本体的用户可见面已大半在场**，呈现义务主要在 webui/goal 侧。
- **单轮终态可感知面缺口**：
  1. agent ZMQ 写不入 Godot kernel_command_signal_bus（`agent/internal/bridge/bridge.go:123-128`=UDP 请求-回执模式；LANE-AUTO 重扫触发=bus 回执/目录变化/进档）——agent 侧写后**无即时重扫信号**，感知延迟到下一触发。边界标注：bus「全量回执面」语义未逐行核（LANE-AUTO 回执措辞），实施卡内核。
  2. webui 无包络写回执摘要形态（region/written/point_count）——通用确认卡+轨迹步可承载但不直观。
  3. goal 摘要与编舞结果无关联呈现（单轮终态注记需进 goal summary 文本面）。

## 五、问5：DAD 曲线回流现状

- **摄入白名单（M3-KERNEL-1 裁定 4，已落）**：`harness.go:4572-4574` isAutomationCurveSummaryFeature——只认 automation_curve_summary 一种 feature_type（fail-closed 注释在册）；分发 `IngestKernelTelemetry:4411→:4426-4427`（command=audio_feature_data_ready+feature_type 匹配）。
- **落盘/证据链形态**：`ingestKernelAutomationCurveTelemetry:4579-4598`→`writeMixboardReadyAutomationCurveSnapshot:6776-6798`——mixboard feature snapshot 行（mixboard_feature_snapshot.v1，行身份=plugin_id+param_id，automation_curve_summaries 合并行数组）=DAD 证据层扩型，无新 peer（裁定 4 遵守，拓扑未动）。
- **生产者：内核零存在**——VitApp/Source 全仓 grep automation_curve_summary=0 命中；audio_feature_data_ready 生产者仅 L3AcousticAnalyzer.cpp:559 与 WaveformEnvelopeBaker.cpp:927。**回流断在生产者侧：agent 管道完备但休眠**。
- **消费面：零消费**——automation_curve_summary 非测试消费全仓 0 命中（编舞链不消费、CCB 无 view、观察投影不读）。编舞 settle 若需曲线证据，现役可得路径=get_curve 主动读回（3.3 步5），非遥测回流。

## 六、问6：形态选项包（交用户面——本卡核心交付）

三维度：意图层指令词表形态 / 编舞触发形态 / 呈现形态。

### 案一「确认制直达」（最小步）

- 词表：不加武装词表——capability_routing 只读计数段维持；server 路由指引补 automation 行（:6203 clip.fade 行式样+淡出分界）。
- 触发：无编舞——普通回合单工具 RiskConfirm 确认制（M3-KERNEL-1 工具面原样），settle=读回。
- 呈现：既有确认卡+轨迹步+goal 摘要；Godot 既有车道。
- 成本：低——context_manifest 四处摘词+server 指引段+manifest 单测+真栈腿（补 agent invoke authority 面首证），约 0.25 天，agent 侧 2 文件+冒测脚本。
- 边界：「这段处理到X」类时变话术仍进自由态静态处理（不产生包络）；时变语义解析全交模型；Godot 即时感知依赖既有重扫触发（问4 缺口1 不解）。

### 案二「单轮终态编舞」（原裁定方向完整形）

- 词表：武装词表=保全件 treatment 动词族（free_state_range_goal.go:68-73）+新增时变限定词维度（慢慢/逐渐/渐强/渐弱等）+ranges 在场三条件。
- 触发：range_goal 单轮终态——arm→**D1-S1 域表新增 automation 包络动作行（点数组载荷=结构性新增）**→materializer 治理发令→写=唯一前向 mutation（D6 同构合规）→写回执+读回断言 settle（复用保全件断言骨架）→终端注记（零自动 revert）。
- 呈现：goal 摘要带范围目标预告+写确认卡+结构 settle 报告（webui 既有 WorkflowData 卡面零新建）。
- 成本：中高——chat 域 3-4 文件（新 range_goal 单轮版+loop 挂点+experiment 域表扩行）+保全件测试改造+真栈冒测（话术→编舞→写→读回→注记），约 1-1.5 天。
- 边界：仅覆盖「框选+目标话术+时变」形态，无框选包络请求回落案一路径；域表扩行动作语义与 fail-closed 门禁边界须主管裁定（STOP-NOTE 红线：不得为发令开口子削弱门禁——automation 行必须是治理面扩行而非 guard 例外）。

### 案三「两段混排」＝**推荐案**

- 形态：案一先行（披露解锁+路由指引+最小链真栈首证）→案二随后（编舞卡复用保全件按单轮终态重写）；三维度形态=案一为底座+案二为增量。
- 成本：案一 0.25 天+案二 1-1.5 天分两卡，各带独立验收；两段文件域不相交（manifest+server 指引段 vs chat free_state 族+experiment 域表）可串行。
- 边界：案二全部边界继承；案一先行不阻塞案二设计（词表/域表均在案二卡内）；DAD 生产者（问5）与 Godot 即时感知（问4 缺口1）两缺口本包不解、另裁后置。

**推荐理由**：案一单独存在时「这段处理到X」用户旅程仍断（时变话术进静态处理）；案二单独先行则披露/指引缺口让普通包络请求继续无路由可走。两段混排每段都有独立用户可见增量，且案一真栈腿为案二编舞卡提供链路证据；与 phase-ordering v2「单轮终态（原 FREESTATE IMPL-1/2 并入）+意图层包络指令」两要素一一对应。

## 七、问7：拆卡建议（单轮终态实施分段）

| 卡序 | 卡 ID（建议） | 内容 | 文件域预划 | 验收形态 |
|---|---|---|---|---|
| 1 | M3-CHOREO-DISCLOSE-1 | 四处 Excluded 摘词+Guidance 补行+server 路由指引 automation 行+淡出分界；真栈腿=agent invoke 驱动 set_points 确认→写→读回（authority 面首证） | capabilitycontext/context_manifest.go+chat/server.go 指引段+冒测脚本扩展 | manifest 单测+真栈腿 exit 0（纸面演示路径 3.3 实证化） |
| 2 | M3-CHOREO-GOAL-1 | 单轮终态编舞：range_goal 单轮化（相位枚举 envelope 族）+武装词表（treatment+时变+ranges）+域表 automation 动作行（materializer 治理发令）+写回执/读回断言 settle+终端注记+保全件测试改造 | chat/free_state_range_goal*.go（新）+free_state_reasoning_loop.go+experiment/d1s1_domains.go | go test 全绿+真栈冒测腿（时变话术+ranges→编舞→写→读回→注记）exit 0 |
| 3（可选后置） | M3-AUTO-TELEM-1 | 内核 automation_curve_summary 生产者（写命令后发 audio_feature_data_ready 遥测）——补问5 生产者缺口；消费面另裁 | VitApp/Source/Service/PluginRackControlService.cpp/CommandDispatcher.cpp | 写命令后 agent 落盘 snapshot 行（harness 摄入已在）冒测 exit 0 |
| 增项（低优先，可并入卡2 或单开） | webui 呈现增项 | 包络写回执摘要卡+goal 摘要范围预告文案 | webui/src/App.tsx 局部 | webui 渲染面端侧烟测（AGENTS §5 渲染门槛） |

依赖：卡1→卡2 串行（文件域不相交但链路证据依赖）；卡3/增项独立。真栈腿按 §2.2 错峰纪律互查。

## 八、停止条件核查

- goalrunner/intent/context_manifest 锚点与卡面引用族：核对完成——核心分支族（:280-330）在位、四处 Excluded 零漂移、intent 面锚点均为**文件迁移+行号保持**的可解释漂移（chat/→agentloop/、frequencycleanup/→chat/），非结构重构 → **未触发**。
- FREESTATE-REGION-IMPL-1-STOP 工件：5 文件（STOP-NOTE/主件/测试/diff）全部在场可读 → **未触发**，复用评估完整（§2.1）。

## 九、锚点漂移总表（供后续卡引用校正）

| 旧锚 | 现锚 | 性质 |
|---|---|---|
| internal/chat/message_loop.go:9626-9627/:4867 | internal/agentloop/message_loop.go 同行号 | 文件迁移 |
| internal/chat/project_blackboard_report.go:94 | internal/agentloop/project_blackboard_report.go:94 | 文件迁移 |
| internal/frequencycleanup/c1_frequency_cleanup_planner.go:31 | internal/chat/c1_frequency_cleanup_planner.go:31 | 文件迁移 |
| goalrunner_chat.go:766,1916-1942（轮分支族） | free_state_experiment_runtime.go:970-1062 | 函数迁移 |
| goalrunner_chat.go:3519-3531（loop 披露面） | goalrunner_chat.go:4137-4145 | 行漂移 |
| catalog.go:987（split_clip） | catalog.go:1133 | 行漂移 |
| server.go:2171-2175（ranges 白名单） | server.go:2458 | 行漂移 |
| free_state_d1_plan_table.go:636-638（域表绑定） | experiment/d1s1_domains.go:58+（行族 128-641） | 域表主体迁移至 experiment 包 |
| harness.go:3964/3977/4111-4113（M3-RECON 摄入锚） | harness.go:4411/4426-4427/4560-4574 | 行漂移（L3 白名单已含 time_energy 四型） |
