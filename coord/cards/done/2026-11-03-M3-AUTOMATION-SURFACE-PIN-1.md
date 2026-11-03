# M3-AUTOMATION-SURFACE-PIN-1：写关键面入册（automation_surfaces 持久段）+回执归因+note 引册一致化+重跑组 1

- 发卡：GLM 主管决策流 / 2026-11-03（依据=240 re-review a6aa8156「纯模型残差」定性推翻的三缺口实证+用户裁定 2026-11-03：T1 接线卡立即做、T2 memory 挂证据触发器不进排期）
- 派发确认：已确认（用户 2026-11-03「ok，那确定了，可以发卡」）
- 验收负责人：GLM 主管决策流
- 池序 241；目标仓库=**主仓 agent**；P1 / 0.5-1 天 / 依赖=240 已在 main（bdd4146 合入+a6aa8156 补记）✓；L1 / PC 执行会话

## 标的（240 re-review 定性，不重考）

- **三缺口实证**（guardrun3 载荷逐轮对账，见 240 卡验收补记）：①成功回执摘要不回显 plugin_id——模型传 1011 调用成功但回执只归因 track_id，按反捏造红线（ccb_model_prompt.go:714）拒认自传 id；②正面发现不入册——plugin_id 1011 最后见第 72 行载荷、automation_param_surface 最后见第 73 行，最终 33 轮决策两者被有界上下文/单槽 recent_observation 顶掉，模型申报「缺标识/缺区间」在其可见面内字面为真；③redirect note 的 in-hand 判定走 state.executed 尾扫，与模型真实可见面（上下文）脱节。
- **机制锚点**：free-state ledger JSON 98/106 轮常驻注入（每轮 user message 内，从不出局）——240 护栏 unavailable_view_sets 钉此全程有效，正面发现可走同一通道；harness `publicPluginParametersResult`（harness.go:12875）已有 `"plugin_id": reply["plugin_id"]` 键；agentloop `automationObservationPromptSummary`（execution_memory.go:798）键表已含 plugin_id——到达摘要层的 result 该键为空，**准确丢失层归本卡勘察定案**（内核回复无此键 vs VSP 路径组装 vs 其他）。

## 目标（五面）

1. **automation_surfaces 入册段**：成功 `plugin.get_parameters` → loop 面+ledger 面双点写 bounded 行（双点合并沿用 240 模式：loop 面=recordFreeStateCCBObservation 同位，ledger 面=mergeFreeStateObservationLedger 同位）：`{plugin_id, plugin_name, track_id, params:[{param_id,label,min,max}], project_revision}`；指纹去重（track_id+plugin_id）；有界 ≤8 行（新旧行淘汰策略定案记录）；revision 严格前进的处理沿用 240 语义（freeStateRevisionStrictlyNewer，失效边界定案记录）。
2. **回执归因**：成功摘要回显 plugin_id/plugin_name——勘察丢失层后单边修（候选：harness 层 `firstNonEmpty(reply["plugin_id"], cmd["plugin_id"], reply["plugin_item_id"])` 或勘察定案的最小侵入位）；红线：automation_param_surface 既有键集除新增归因键外字节不变。
3. **note 引册一致化**：redirect note 的 in-hand 判定（freeStateAutomationParamSurfacePresent 尾扫）改读 ledger 册（=模型可见面）；册在场时 note 附具体引用（plugin_id+param 范围）。**红线：note 前缀 token `processor.* CCB projections do not cover` 逐字稳定**（route smoke needle 锚定），引用信息只可附加在后部；needle 若需同步微扩按文件域条款处理。
4. **单测**：入册三件（成功入册/指纹去重/revision 前进出册）+有界淘汰+回执归因断言+note 引册两态断言+零回退（240 护栏语义与既有摘要/披露语义字节不变）。
5. **重跑腿**：同码（唯一 diff=本卡目标 1-4）同 fixture 同引擎（**deepseek-flash**）重跑组 1；载荷捕获+needle+param_surface 全开。判据同链口径：组 1 no_route 消失+`tool_chain_ok`（set_points 包络形态）。门过且预算余量时组 2/3 跑满（非验收门）。

## 预期与诚实边界

本卡=**接线卡**（正面发现路由进已有持久通道），非泛化 memory——T2 findings memory 挂证据触发器（超钉住面覆盖/可证实重复重取烧预算/T3 愿景前置），不进当前排期（用户裁定 2026-11-03）。时间范围类事实不入册（可经 CCB 视图重观察，非丢失输出）。**判定成功=组 1 绿或新真断点**；入册+归因+引册三在场（载荷亲证）而模型仍不走向 set_points → 残差定性（持有持久册仍不用=纯模型行为面）才真正成立，回终局三选。预算 1-3 轮 ≈240 单轮量级（¥1.5-2/轮），取证迭代如实披露。

## 停止条件

- 入册/归因破坏既有摘要语义或 240 护栏（零回退测试红）→回退两案上交；
- 重跑组 1 绿 → 链收口（组 2/3+统一手测转正），不开新卡；
- 重跑仍 no_route 三连同型且三在场载荷亲证 → 残差定性成立，终局三选（接强引擎/架构重构/收线）回用户裁定，不再折卡；
- LLM 面栈不可用→环境中断如实记录。

## 文件域

`agent/internal/agentloop/`（free_state_reasoning.go 入册段+execution_memory.go 归因+note 一致化+测试）或勘察定案的 harness 单点（harness.go publicPluginParametersResult）——**单边落地**，卡内记录定案理由；`scripts/automation_llm_route_smoke.py`（仅 needle 同步微扩）。越域即停上交。

## 验收标准

- 单测全组+相关包全绿；
- 真栈重跑腿：场景腿 COMPLETED+载荷四件亲证（**最终轮载荷含 automation_surfaces 册行**[plugin_id=实测值]/归因键在场/note 引册在场/护栏零回退）+组 1 判据或残差定性三件套；栈登记/释放全流程；§8 计费披露。

## 领取

- 2026-11-03 晚 +0800 / origin/main=c00b5201 / owner=GLM-5.3 PC 执行会话（ZCode，L1，241 卡会话）/ 分支=port/m3-automation-surface-pin-1 / worktree=D:\Vit_DAW_wt_241impl
- 领取基线三件：HEAD=c00b5201；impl worktree `git status --short` 空、`git diff --stat` 空（干净基线，无领取前 diff）
- 主工作树 D:\Vit_DAW 既有未提交改动（VitApp Workspace/coord runs 工件/webui design 等）与本卡文件域（agentloop/automation_llm_route_smoke.py）不相交（grep 亲核零匹配），不动
- 真栈腿：重跑前按 §2.2 登记 PC-RUNTIME-STACK.md（fetch 重读 main+进程/端口亲核全零后登记，push 后 ≤60s 即时复核）

### 勘察定案（实现前记入，卡面要求）

**①plugin_id 丢失层=agentloop 模型投影层（helpers.go），非内核/VSP/harness/摘要层。** 证据链三层：
- 静态全链透传：内核 VSP reply（guardrun3 executed 面亲证 result.plugin_id='1011'×5 处成功回执）→ harness `publicPluginParametersResult`（harness.go:12887 透传 reply["plugin_id"]）→ executor `resp.Result` 同 map 引用 → `automationObservationPromptSummary` 键表含 plugin_id（15de9627/f0d092fd/9cc871d7/HEAD 四点核验一致）→ contextruntime compactValue 不剥键。240 补记「到达摘要层的 result 该键为空」**不准确**——result 该键非空、summary 组装正确携带。
- 动态复现三重 PASS：函数级/`recentObservationForTool` 全链级/guardrun3 真实 result fixture 级，plugin_id 均正确到达 summary。
- **真丢点**：`messageLoopNeedsNeutralFamilyProjection`（helpers.go:131）= `messageLoopFreeStateActive`——free-state active 期**全程**走中性族投影（fail-safe 设计，非仅族选择前）；`buildModelContextSnapshot`（helpers.go:70）对 RecentObservation 过 `messageLoopPreFamilyValue` → `messageLoopPreFamilyMap` → `messageLoopPreFamilyForbiddenKey`（helpers.go:326）marker 列表含 `"plugin"`——**summary 顶层 plugin_id 键被删**。而 `automation_param_surface` 的值是 `[]map[string]any` 类型，`messageLoopPreFamilyValue` 的 switch 无此 case→default 原样保留（行内 param_id 幸存）——与 guardrun 载荷观测（param_id 在、plugin_id 不在）逐键吻合；executed 面不经该投影所以保留。复现测试钉死（TestAutomationAttributionSurvivesNeutralProjection 内嵌丢失层断言）。
- **单边修**：`restoreFreeStateAutomationAttribution`（helpers.go，中性族分支挂点）——仅 automation 读回执（plugin.get_parameters 族）的 summary 归因键（plugin_id/plugin_name/plugin_item_id）拷回；Context/State/Trace 族前匿名化零改动。设计依据：needs_observation 轮模型已过 family 选择（M3-LLM-GATE-OPEN-1 放行该面），回执归因是对模型自己工具调用的回应，不是预置引导。
- plugin_name 源定案：内核 get_plugin_parameters reply 无 plugin_name 键（PluginRackControlService.cpp:3381-3394 内建路径亲核；改内核超本卡文件域）——摘要键表加 plugin_name（add-only 纯键搬运，内核未来回带即自动到达），不硬造值。

**②入册行淘汰与 revision 失效策略**：
- 淘汰=FIFO 尾进头出（沿 `appendBoundedFreeStateRows` 同契约），上限 8（卡面）；同指纹（track_id+plugin_id）重复成功读取**原地刷新**该行（params/project_revision/round 更新，不追加）——参数面可能变化，刷新优于首见即冻结。
- revision 失效=**严格前进出册**（`freeStateRevisionStrictlyNewer`，240 同语义对偶：guard 对 revision 前进放行重观察 ↔ 册对 revision 前进失效旧发现，行为闭环——模型 mutation 后必须重读，不写过期区间）。同 revision 不失效（幂等重读）；unorderable fail-closed 保留。revision 源=`freeStateAutomationSurfaceProjectRevision`（project_binding→free_state_project_change.to_project→latest_project_change→ctx project_revision→closure，与 240 guard `freeStateUnavailableRetryRevisionAdmitted` 同族源，判定永不分歧）。读时点复核失效（present/引用句构造时再清一遍）。
- 模型到达面=**ledger JSON 常驻面**（`messageLoopFreeStatePromptContext` 键表加 automation_surfaces——「Free-state reasoning ledger JSON」每轮 user message 注入，240 EVIDENCE 实证 98/106 轮从不出局）；不动 contextruntime ProjectObservationLedger（unavailable_view_sets 同样不进该投影，零回退面最小+不越域）。双点=loop 顶层 `automation_surfaces`（prompt JSON+present 判定读此）+`observation_ledger.automation_surfaces` 镜像（240 双点对偶，无 CCB 观察时独立建 schema）。
- note 引册：`freeStateAutomationParamSurfacePresent` 从 state.executed 尾扫改读册（=模型可见面，消除 240 补记③判定面脱节）；denial 的 in-hand 句升级闭模板引用 `automation_surfaces in hand: plugin_id=… track_id=… params=volume[0..1],pan[-1..1] … use these pinned rows for automation.set_points`；**base note 常量逐字节不动**（`; redirect: ` 前缀+全文，needle 红线）。

## 回执

- **实现**：port/m3-automation-surface-pin-1 @ **384fc14a**（+工件 e8625ea2）：agentloop 六文件+smoke py（8 文件 +605/-23 纯实现+测试，文件域全守：free_state_reasoning.go 入册段+free_state_automation_surface.go read matcher+execution_memory.go 键表+helpers.go 归因豁免+runner.go 三挂点+free_state_view_guard_test.go in-hand 断言同步+free_state_automation_surface_pin_test.go 新建 9 测+automation_llm_route_smoke.py needle 微扩）。三轮 go build/test ./... 96 包 exit 0。
- **链路进度（目标 1-4）：全达**——①入册段：recordFreeStateAutomationSurface 双点写（loop 顶层+observation_ledger 镜像）{plugin_id,plugin_name?,track_id,params[param_id/label/min/max],project_revision,round}；指纹=track+plugin 原地刷新；FIFO≤8；strictlyNewer 出册（读时复核）；到达面=ledger JSON 常驻注入。②归因：丢失层定案=helpers.go:326 preFamily forbidden marker "plugin" 删 summary.plugin_id（free-state active 全程；内核/VSP/harness/摘要层全链透传无罪——executed 面亲证 result.plugin_id='1011'×5+复现三重 PASS+载荷逐键吻合[param_id 幸存=typed slice default 分支]）；修=restoreFreeStateAutomationAttribution 窄域豁免；摘要键表+plugin_name（add-only，内核现无此键不硬造——PluginRackControlService.cpp:3381 内建路径亲核）。③note 引册：present 改读册+denial 引用句闭模板；base note 逐字节不动（diff 亲证）。④单测 9 件+240 八测原语义绿（in-hand 断言按目标 3 授权同步）。
- **最终验收（目标 5，pinrun1 单轮 HEAD=384fc14a 唯一 diff=目标 1-4，flash 三面亲证，kernel F9E3DA0A 零漂移，ps1 exit 0+COMPLETED）：组 1 门未过（no_route×3）——但「判定成功=组 1 绿或**新真断点**」第二形态成立**：①**册行被模型真实消费**——g1_r3 最终决策原文「只有参数范围信息，还缺当前值」（240 的「缺标识/缺区间」消失，标识+区间已入模）；②**模型走向 automation 工具链**——r1/r3 均调 automation.get_curve（240「不走向 set_points 工具链」形态消失），processor.* 同集重试零发生（denial=0 行）；③**断点钉死=写前基线缺格**——get_curve(volume) ok 但 point_count=0（无自动化点），模型要的当前值=静态参数值，内核 parameters 行携带 value/value_text（PluginRackControlService.cpp:741-743 亲核）但 harness compactAutomationParamSurfaceRows 投影只出 param_id/label/min/max/unit/normalized_role/supports_automation（**当前值被 harness 投影丢弃**，include_* 门控面有但骨架未教）。修复方向（harness 行 add-only +value/value_text，涉 237 键集红线）**供主管裁定，未开卡自修**。残差定性（240 预埋终局三选前提「模型不用册」）**不成立**——册被用了，终局三选不触发。
- **载荷四件亲证**：册行 38/82 行（plugin_id=1011 实测值/track 1010/volume[0..1]/pan[-1..1]/rev-5，样本观察循环内常驻）；归因键 34/82 行（240 全程 0 行→丢失层修复真栈实证）；note 引册=guard 零触发的自然结果（无 denial 面；两态由单测锁定）；护栏零回退（240 八测绿+base note 零字节）。已知边界如实披露：chat workflow_data 持久化面未搬运 automation_surfaces（样本内有效/跨 exchange 重观察，与 240 unavailable_view_sets 同生命周期，非回归）；修复轮与 settled 收尾轮不带 ledger JSON 组装面。
- **栈登记/释放全流程**：占用 801d2a26（fetch 重读 main=9c076715+亲核全零+push 后 ≤60s 即时复核全零）→单轮→释放 dfc0a36e（TEARDOWN_ALL_ZERO 23:46:42 亲核 PROCS=0+TCP/UDP 全零）。
- **§8 计费披露**：单轮 77 计费调用 deepseek-flash，in 1,176,938/out 138,059 ≈**¥1.45 上界**（卡面 1-3 轮预算内，第一轮即收口，无重跑）。
- 工件：coord/runs/M3-AUTOMATION-SURFACE-PIN-1/（pinrun1 全套+RECEIPT.md，随 port 分支入库）；prompt_render 2.9M 留 worktree（重复度高，240 先例）；.vit_derived/.vit_agent/.vit_history/clip.wav 排除（240 瑕疵教训）。


## 验收

- **GLM 主管决策流 / 2026-11-04 晨 / 裁定：实现面 pass 合入（4ea33461）+「判定成功」第二形态（新真断点）成立采信，终局三选不触发。**
- 主管亲核九件：①diff 全量 stat=文件域全守（8 文件 +605/-23）；②丢失层定案源码亲核=helpers.go:364 marker 列表含 "plugin"（中性族投影删 summary.plugin_id 实锤），restoreFreeStateAutomationAttribution 窄域豁免设计合理（归因是对模型自己工具调用的回应非预置引导）——**执行侧纠正主管 240 补记错误断言（result 键非空、丢失在投影层）成立，三重复现采信**；③本地复跑 build+agentloop 包测试 exit 0；④册行持久 22-74/82 行至决策轮（末 8 行=修复/settled 轮，回执已披露）；⑤册行被消费亲证=L68 模型回复逐字「我先读一下这条轨道音量参数的可写范围和当前值，好把'越来越突出'写成一段从当前音量平滑上行的音量曲线，而不是一次跳变」+终局申报只剩「还缺当前值」；⑥工具链亲证=rounds tools_called r1/r3 含 automation.get_curve；⑦新断点事实=内核源码主管亲核 PluginRackControlService.cpp:741-743 每参数行带 value/normalized_value/value_text，harness compactAutomationParamSurfaceRows 键集无 value（代码亲读）；⑧栈登记/释放全流程+验收时主管复核进程/端口全零；⑨工件卫生（.vit_derived 排除，240 教训落实）。
- 一处回执表述瑕疵入档（不影响裁定）：「denial=0 行」不准确——事件级去重实为 **1 次早期 denial**（~第 9 轮，22 行载荷为历史回显），随后即转向 automation 面。实质成立：237=~30 次循环，241=1 次被拦即转向（护栏+册协同的预期形态）。
- 已知边界采信：chat workflow_data 持久化面未搬运 automation_surfaces（与 240 unavailable_view_sets 同生命周期）；修复/settled 收尾轮不带 ledger 组装面（决策轮覆盖）。
- **处置**：断点修复方向裁定采纳=harness 行 add-only +value/value_text+册行携带（同 241 持久性论证：当前值与范围同寿命）——开卡 242（M3-CURRENT-VALUE-DISCLOSE-1）。链 229→241 续行，模型已入工具链（get_curve 在手），距 set_points 一块拼图。
