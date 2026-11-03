# M3-LOOP-UNAVAILABLE-VIEW-GUARD-1：不可用视图重试机械执法+重定向披露（237 残差的执法面收口）+flash 重跑组 1

- 发卡：GLM 主管决策流 / 2026-11-01 深夜（依据=237 残差终局定性的执法面缺口主管亲核+**用户裁定 2026-11-01**：模型排序修正 flash(1017)>v4-pro→pro 判别撤销，方向=执法护栏+重跑；flash=可用最强引擎，残差指向架构/执法面）
- 派发确认：已确认（用户 2026-11-01 深夜裁定「执法护栏+重跑」）
- 验收负责人：GLM 主管决策流
- 池序 240；目标仓库=**主仓 agent**；P1 / 0.5 天 / 依赖=237 已在 main（79443422）✓（238 工件无生产代码依赖）；L1 / PC 执行会话（锚点已定死；重跑判读承链测量面）

## 标的（237 验收+主管亲核，不重考）

- 237 残差形态：模型对 `processor.identity_and_controls`（对内建轨级插件返回 `unsupported_projection`/`source evidence is missing`）**反复请求 ~30 次**（237 载荷 50 行上下文反复携带 unavailable 回执，主管亲核），骨架规则明文「If a CCB observation returns rejected, unavailable, or deferred evidence for a requested view set, do not retry the same view set」**但循环层零机械执法**——规则只存在于 prompt 自觉层。允许自家规则被违反 30 次无后果的循环，对任何模型都不是公平测试。
- 同时：unavailable 回执的 omission_reason 不带重定向——模型不知道 processor.* 投影不覆盖内建轨级插件，也不知道上下文里已有的 automation 面结果（plugin.get_parameters 参数面）即权威参数面。

## 目标（四面）

1. **机械执法护栏**：同一 exact view set+同 target 在此前已返回 unavailable/unsupported/rejected 后**第二次请求起，循环机械拒绝执行**（返回机器可读拒绝：该视图集本轮已判定不可用+引用规则出处+不再消耗观察轮）。勘察定案拦截位（agentloop 观察请求执行面 vs CCB 服务面，选最小侵入+对全部消费路径生效者）。红线：只拦「完全同视图集+同 target+无 revision 变化」的重试——不同视图集/不同 target/project revision 前进**不拦**（规则原文 exact_view_set 语义）；拒绝不消耗观察预算（拒绝不是观察）。
2. **重定向披露**：unavailable 回执的 omission_reason 补 closed-template 机制说明：processor.* 投影不覆盖内建轨级插件→其权威参数面=automation 面（plugin.get_parameters 结果，若 recent_observation 已携带则指明在场）。承 237 验收「次要事实」条。
3. **单测**：拦截三件（同集+同 target 第二次拒/不同视图集放行/revision 前进放行）+拒绝回执形态断言（机器可读+规则引用）+重定向文本断言+零回退（正常观察路径与既有 rejection/declared_contradiction 语义字节不变）。
4. **重跑腿（flash 回归最强引擎）**：同码（唯一 diff=本卡目标 1/2/3）同 fixture 同引擎（**deepseek-flash**，撤 v4-pro）重跑组 1；载荷捕获+needle+param_surface 全开。判据同链口径：组 1 no_route 消失+`tool_chain_ok`（set_points 包络形态）。门过且预算余量时组 2/3 跑满（非验收门）。

## 预期与诚实边界

本卡=执法面收口（把已有明文规则从自觉层升到机械层）+披露补全，非新增判断逻辑。**终局分叉预埋**：护栏+重定向在场（载荷亲证）flash 仍三连同型止损 → flash=可用最强+执法+披露三全下仍不收敛=**架构面终局证据包**上呈用户裁定（接更强引擎 vs 架构重构 vs 收线接受边界申报形态），不再折卡。预算 ≈237 量级（¥1.5 档）。单 run 存在性判别（§8）。

## 停止条件

- 护栏拦截正常合法观察（异集/target/revision 放行路径误伤，既有观察测试红）→回退两案上交；
- 重跑仍三连同型止损→架构面终局证据包上交（载荷+收敛面+护栏触发计数三件套），转用户裁定不开新卡；
- LLM 面栈不可用→环境中断如实记录。

## 文件域

`agent/internal/agentloop/`（观察请求执行面拦截+omission 披露）或 CCB 服务面（勘察定案单边）+测试+`scripts/automation_llm_route_smoke.py`（如判读需护栏触发计数微扩）。越域即停上交。

## 验收标准

- 单测四组+相关包全绿；
- 真栈重跑腿：场景腿 COMPLETED+载荷三件套+护栏触发与重定向亲证（或护栏零触发+模型一次即弃的形态如实记录）+组 1 判据或终局证据包；栈登记/释放全流程；§8 计费披露。

## 领取

- 2026-11-03 上午 +0800 / origin/main=15de9627 / owner=GLM-5.3 PC 执行会话（ZCode，L1，240 卡会话）/ 分支=port/m3-loop-unavailable-view-guard-1 / worktree=D:\Vit_DAW_wt_240impl
- 领取基线三件：HEAD=15de9627；impl worktree `git status --short` 空、`git diff --stat` 空（干净基线，无领取前 diff）
- 主工作树 D:\Vit_DAW 既有未提交改动（VitApp Workspace/coord runs 工件/webui design 等）与本卡文件域（agentloop/scripts）不相交，不动
- 真栈腿：重跑前按 §2.2 登记 PC-RUNTIME-STACK.md（fetch 重读 main+进程/端口亲核全零后登记，push 后 ≤60s 即时复核）

## 回执

- 实现：port/m3-loop-unavailable-view-guard-1 @ **9cc871d7**（三 commit：f0d092fd 护栏+披露+单测四组 →206b0169 修复 1 披露下沉 raw bundle 层 →9cc871d7 修复 2 披露/执法解耦按逐视图引证；全部在卡面文件域 agent/internal/agentloop/ 三文件：free_state_reasoning.go+execution_memory.go+free_state_view_guard_test.go 纯 Go 零回退）。工件同分支 coord/runs/M3-LOOP-UNAVAILABLE-VIEW-GUARD-1/（guardrun1/2/3+EVIDENCE.md）。
- **拦截位定案（勘察记由）**：agentloop 观察请求执行面——既有 rejected 护栏同位扩展（messageLoopFreeStateRejectedViewSetIssue 尾接 unavailable 检查，message_loop.go:5435 与决策验证 :1007 两调用点自动生效单边落地）；bounce 前置于 executeTool=拒绝不是观察天然不消耗预算；ledger 记忆（observation_ledger opaque map）本层已双点合并跨轮存活；harness CCB 服务面为无跨请求状态纯函数面，落地需跨层状态管道=更大侵入。缺口定性：现有护栏只认 bundle status=="rejected"，237 形态=视图级不可用（AssembleFreeStateObservation len(Views)==0→"insufficient"）零执法。
- **链路进度（目标 1/2/3）：全达**——①机械护栏：freeStateUnavailableLedgerEntry 记 exact fingerprint（status=unavailable/retry_policy=do_not_retry/project_revision 随行）入 ledger unavailable_view_sets+loop unavailable_observation_requests；guard 机器可读拒绝（view_set_retry_denied/status=unavailable/rule=ccb_model_prompt.go:720 原文/observation_round_consumed=false 四稳定 token）；红线三件全守：异集/异 target 异指纹放行（单测+真栈）、严格新 revision 放行（freeStateRevisionStrictlyNewer，单测三态）、拒绝零观察物化（guardrun1 rows100-124 denial=6 而新 unavailable 回执=0 实证）。②重定向披露：closed-template note（processor.* 不覆盖内建轨级插件→权威参数面=plugin.get_parameters+已在场自查指引）挂 raw bundle 层（recentObservationForTool clone 前就地附加）三面同达（trace→model_projection/executed record→ccb_observation_summary/recentObservation clone）；ledger 行 reasons 经 rawCCBDeliveryReasons 过滤 note 保 raw；denial 对 processor.* 集附 redirect 句+在场标记（state.executed plugin.get_parameters 成功尾扫）。③单测四组（free_state_view_guard_test.go 8 测试）：拦截三件+denial 机器可读四 token+redirect/在场两态+bundle 层幂等+ledger 零污染+混合集正例/预算裁剪负例+零回退（rejected 语义字节断言+usable 去重/declared_contradiction 准入不动）；三轮 go test ./... exit 0（96 包）。
- **真实栈三轮（目标 4，无原样重跑：三轮 HEAD 两两不同唯一 diff=披露面）**：三轮 ps1 均 exit 0+场景腿 COMPLETED+引擎三面亲证 deepseek-flash（console env 行+telemetry 逐行+payload model）；kernel F9E3DA0A 三轮零漂移。**护栏触发亲证**：guardrun1 denial=21 行载荷（237 的 ~30 次违规重试被机械截断）+guardrun2/3 各 5 行（guardrun3 全带 redirect=执法+披露同回执）；**重定向亲证**：guardrun3 note=45/106 行（两次取证迭代如实记录：guardrun1 note 挂 clone 面零到达→修复 1；guardrun2 模型改混合集（bundle=partial=usable）使 bundle 级前置不成立→修复 2 披露/执法解耦，拦截红线维持窄域不动）。
- **最终验收（组 1 门三件）：未过——停止条件第二条触发，架构面终局证据包上呈**：guardrun1 [no_route,no_route,**boundary_ok**,no_route,no_route] 1/5（boundary_ok=新形态首现：模型引用 unavailable 回执申报能力边界「不能靠猜测填补…不会自动重试」=骨架期望的诚实行为）；guardrun2/3 [no_route×3] 三连同型止损。**形态质变于 237**：无限同集兜圈→被机械拦截+边界申报+settled 停止（r4/r5 分型=模型申报参数标识/区间缺失后 continuation settled 无能力面，持有 param_surface 载荷行仍宣称缺失——237 残差在执法+披露双全下收窄为「不走向 set_points 工具链」的纯模型行为面）。组 2（仅 guardrun1 跑到）[no_route,confirm_broken×3] 止损 stop_reason=plugin_selection_required（非本卡标的）。EVIDENCE.md 三件套（载荷+收敛面+护栏触发计数）成文，终局三选（接更强引擎/架构重构/收线）转用户裁定，不开新卡。
- 栈登记/释放全流程：占用 da0406ee（登记前 fetch 重读 main=a1e3ed20+进程/端口亲核全零+push 后 19:47:22 ≤60s 即时复核全零）→三轮间 owner 手拆核零→释放 0db08f85（TEARDOWN_ALL_ZERO 21:12:40：PROCS=0+TCP 5555/5556/7878/8787/18787/5178=0+UDP 4444/4445=0 亲核）。
- §8 计费披露：三轮合计 360 计费行 in 5,212,447/out 533,367 ≈**¥6.3 上界**（flash ¥1/M in+¥2/M out 全价上界口径，含 cache_read 高估）；超卡面 ¥1.5 档预估如实披露=三轮取证迭代的诚实成本（单轮均 ≈237 量级）；每轮独立止损存在性判别。

## 验收

- **GLM 主管决策流 / 2026-11-03 / 裁定：实现面 pass 合入（bdd4141e）+ 组 1 门未过成立，停止条件 2 触发，终局证据包采信，终局三选转用户裁定。**
- 主管亲核六件：①diff 全量 stat=文件域全守（agentloop 三文件 +611 行纯 Go，scripts 未动[可选项]）；②护栏红线逐行读代码=指纹集+target/`freeStateRevisionStrictlyNewer` 严格前进放行（复用 free_state_gate.go:1297）/guard 前置 executeTool 零观察物化/rejected 语义同位尾接零侵入；③fix2 披露执法解耦红线维持窄域（note 逐视图引证触发，拦截纯不可用整集不动）；④本地复跑 build+agentloop 包测试 exit 0；⑤工件抽查全中（denial 21/5/5、guardrun3 note 45/106+redirect 5/5、telemetry 106/106 deepseek-flash、三轮 PS1_EXIT=0 COMPLETED、boundary_ok 引文逐字在卷、guardrun2/3 verdict no_route×3 亲读）；⑥栈登记/释放全流程+主管验收时进程/端口复核全零。
- 两点入档观察（不影响通过）：①「三连同型」为 verdict 级同型，detail 存在族内变体（settled completed/failed、纯文本）——止损判据以 verdict 族为准成立，终局结论不依赖逐字同型；②计费 ¥6.3 超卡面 ¥1.5 档=三轮取证迭代诚实成本，披露合规，三轮 HEAD 两两不同非原样重跑。
- 瑕疵留档（同 238 先例不重写历史）：工件 merge 混入 guardrun2/3 `.vit_derived/` 运行态目录（各 2 小文件，与 rounds/setup_acoustic_package_status.json 同源冗余）——不影响证据链，后续卡工件清单模板补「.vit_derived 排除」条目。
- 两次修复（206b0169/9cc871d7）均为取证驱动且在目标 2 意图内（披露必须真实到达模型面），非越域。
- **终局分叉兑现**：flash（用户裁定可用最强）+执法+披露三全在场（载荷亲证）组 1 仍 no_route 三连同型止损，形态已质变于 237（无限重试→被拦+boundary_ok 诚实边界申报首现+settled 停止）——剩余残差=持有 param_surface 载荷行仍不走向 set_points 工具链的纯模型行为面。三选（接更强引擎/架构重构/收线接受边界申报形态）转用户裁定，不开新卡。239 timeout 适配维持 P2：若用户选「接更强引擎」则 239 升前置（reasoning 引擎 60s 硬线先适配）。

### 验收补记（2026-11-03 用户质疑驱动复审——「纯模型残差」定性推翻，改判披露持久性缺口）

用户质询：模型宣称缺失的信息到底缺没缺？是提供了没用，还是格式/持久性问题？主管对 guardrun3 载荷逐轮对账（payload 106 行+summary 执行记录+源码三层），证据链：

1. **模型读对了参数面且行动正确**：第 20 行推理原文亲证它读到 volume 0..1/pan -1..1；第 6 行它明确知道 set_points 需要 plugin_id 而「I don't have it yet」；第 8-11 行它尝试 track_id-only 与 plugin_type=volume 解析；第 12 行起从歧义报错（`multiple visible plugins match: [1011 Volume & Pan Plugin, 1012 Level Meter]`）学到 1011 并成功读到参数面。**它没有浪费轮次，没有不使用已提供数据。**
2. **成功回执摘要不回显 plugin_id**（载荷第 20 行 recent_observation.summary={track_id,parameter_count,automation_param_surface}，无 plugin 身份归因；harness 层 publicPluginParametersResult 有 plugin_id 键、agentloop automationObservationPromptSummary 键表也有——到达摘要层的 result 该键为空，准确丢失层归修复卡勘察）。模型按反捏造红线（ccb_model_prompt.go:714）不认自己传入的 1011 为「returned evidence 解析出的标识」——回执只归因到 track 1010，未确认 1011↔该参数面。
3. **持久性双缺（定性核心）**：`1011` 最后出现在第 72 行载荷、`automation_param_surface` 最后出现在第 73 行——**最终 33 轮决策里两者都被有界上下文/单槽 recent_observation 顶掉**（末轮 recent_observation=track.timbre_frequency CCB 观察）。模型最终申报「缺参数标识、缺可写区间」在其可见面内**字面为真**。
4. **redirect note 与可见面脱节**：240 的 note（45 行在场）说「参数面若已在 recent observations 就直接用」——但 in-hand 判定走 state.executed 尾扫，与模型真实可见面（上下文）不一致；模型后续合理观察（定包络形状）反而加速顶掉参数面。
5. **修正裁定**：237「持有权威数据仍宣称缺失」的「持有」实为时间窗持有——参数面与身份在其后观察轮中被逐出。**残差主因=我方披露持久性/归因缺口（回执不归因身份+单槽顶掉+note in-hand 判定面不一致），模型行为在规则内正确**（不捏造、申报边界、settled 停止）。原「纯模型行为面」定性撤销；终局三选暂缓，修复持久性缺口后重跑组 1 优先。教训入档：「模型持有数据」类断言的验收必须核**最终决策轮载荷**的持续在场，不能以任一轮在场为准。
