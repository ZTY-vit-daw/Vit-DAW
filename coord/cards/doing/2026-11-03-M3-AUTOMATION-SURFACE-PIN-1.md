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

## 回执

- （执行侧回填）

## 验收

- （主管回填）
