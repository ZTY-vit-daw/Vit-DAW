# G3-ATTRIB-1 归因报告：pull/push 行为等价性 + 观察预算校准 + 冷启动供给面（归因卡①）

- 卡：2026-10-09-G3-ATTRIB-1（池序 35；[G3-RULING §3](../../runs/L1-5-IMPL-D/G3-RULING.md) 归因卡①）
- 执行：GLM-5.3 执行会话（PC / ZCode），2026-10-09 晚窗
- 被测代码：port/g3-attrib-1 @ c4c015c5（冷启动供给面 chat/server.go + 场景扩展 dev_agent_smoke.ps1；pullharness/agentloop 既有面零改动）
- run：`coord/runs/G3-ATTRIB-1/20261009_214138_harnessab/`（SCRIPT-LASTEXITCODE=0）
- 栈占用：登记 82a654ad（占用前核对三端口 0 监听/进程 0）；跑后释放（见 PC-RUNTIME-STACK.md 回填）

## 1. 运行设定（§8 纪律执行面）

- base 相每模式 1 轮（G3 首轮已有 3×3 固定话术基线，本卡不重复）；话术族相 3 族×2 模式×1 轮（J3 同源 f1 + 低频清理 f2 + 动态控制 f3，每轮 fixture reopen 归一基线）；确认往返相 pull 1 轮（hop≤3）；预算相 pull 1 轮（`pull_observation_budget {max_cycles:1, max_probe_cost:8}`，hop 有界放行）。全部新会话。
- 成功条件（记录不抛）：判定链分类逐轮记录（judgment_terminal / needs_user_* / budget_exhausted / 分记失败类）。
- 失败分类：infra_error_llm/transport/protocol 分记；环境面（VSP 8787 拒连 WARN）全程存在但两模式同载、不入判定链失败类。
- 止损线：同模式连续 2 轮 infra_error_* → 停该模式；轮后 agent 健康失败 → 止损 run。两线均未触发（stop_loss_run=false）。

## 2. 四目标证据

### 2.1 目标①确认往返真栈面（补齐 ✓）

pull 确认往返在本轮真栈完整行使（工件 `harness_ab_confirm_roundtrip.json`）：

- 首轮话术（J3）→ `needs_user_confirmation`（无 plan_id——挂起面是 **interaction 卡**，mix_tick 工作流，非 agentloop plan 卡；两卡面驱动均已在场景实现）；
- 用户确认（`/agent/interaction/respond` approve）→ goal=**completed**，stop=**`mix_tick_applied_reobserved`**——确认→执行→应用→**重观察**一站到达；
- 会话事件流佐证 A/B 判定站：`audition.*`×4 + `mix_tick*`×2（共 18 事件）。
- 该面此前仅有单测覆盖（TestPullEntryConfirmationResumeSameSession），本轮为真栈首行使。

### 2.2 目标②多话术采样与等价性归因（✓，结论见 §3）

outcome 分层（工件 `../equivalence_stratification.json`；base 相 J3 同话术）：

| 话术族 | push | pull |
|---|---|---|
| f1 响度平衡（J3 同源） | judgment_terminal（no_pending_mix_tick_candidate，2 turn） | needs_user_confirmation（1 turn） |
| f2 低频清理 | judgment_terminal（no_pending_mix_tick_candidate，2 turn） | **judgment_terminal（done，1 turn）** |
| f3 动态控制 | **needs_user_confirmation**（1 turn） | needs_user_confirmation（1 turn） |
| base r1（J3） | judgment_terminal（limit_reached→nudge→no_pending_mix_tick_candidate） | judgment_failed@边界（内部 completed/done，见 §4.3） |

成本面：llm_calls push 6 vs pull 17（含确认/预算相；与 G3 首轮 pull 轮级用量更高的方向一致）。

### 2.3 目标③观察预算校准（✓）

工件 `harness_ab_budget.json`：goal 上下文携带 `pull_observation_budget {max_cycles:1, max_probe_cost:8}` → 首个工具循环节完成后下一边界触发 **`observation_budget_exhausted`**（failed 终态，分类 budget_exhausted）——真栈触发面成立。**分记核对**：该轮 stop 序列零 `no_pending_mix_tick_candidate` 共现（`accounted_separately=true`），预算类与 no_candidate 类单列不混记。max_probe_cost 披露面已注入（动态区 budget_state 行），其执法以 probe 成本计量为前提——执行面仍无计量源（G3-RULING §4 既有披露，ProbeCostKnown=false 如实），本轮 max_cycles 为实际执法量。

### 2.4 目标④冷启动 engine_snapshot 供给面（✓ rendered 实证）

- **注入面**：chat/server.go `contextWithPullEngineSnapshot`——pull 模式 goal 上下文注入 shadow `engine_snapshot`（`pull_entry.go:156-157` 冷启动源消费面闭环）；四门控=push 零变化/会话首装（活 continuation 不再注入，续跑沿 continuation.Context 回带首装快照）/影子未就绪=absent 显式/调用方显式携带优先。单测 5 门（`pull_coldstart_supply_test.go`）。
- **rendered 实证**（工件 `../coldstart_prefix_delta.json`）：注入后 pull 每 goal 首调 prefix_bytes=34,316–34,476，对照 G3 首轮 absent 基线（3 goal 恒 34,027）**最小增量 +289、众数 +449**——TOM 概览+总线拓扑+交付目标三事实组字节实际进入稳定前缀；agent 日志 6×`[chat] pull cold-start engine_snapshot supplied`（tracks=7/2）。首调 breaks=0（底座段会话内首装无断裂）。

## 3. 等价性归因结论（主交付）

**结论：pull 与 push 在"到达结构化判定"上站图等价；G3 首轮的 outcome 分布位移是话术条件化的风格差异（distribution shift），不是 pull 行为缺陷。停止条件（方向性缺陷证据）未触发。**

依据：

1. **两模式都产出两种形态**：push 在 f3（动态控制）产出确认卡（G3 首轮 3/3 确定式完成是单话术伪象）；pull 在 f2（低频清理）不经确认直达 done。outcome 形态由话术×模式共同条件化，不存在"pull 只会提案确认 / push 只会确定式完成"的模态锁死。
2. **pull 能到达 push 的全部判定站**：直达终局（f2 done）、确认往返后应用+重观察+终局（§2.1 mix_tick_applied_reobserved）、预算止损站（§2.3）。站图相同；差异在**倾向**（pull 首轮提案待确认的比例更高：本采样 3/4 健康轮 vs push 1/4），与 G3-RULING §3 归因假设一致（精简装配下模型倾向先提案）。
3. **等价性口径限定**：等价性判在"到达结构化判定"（站图），不断言质量等价（结构化 judgment_ok 两模式均未产出，与 G3 首轮口径一致——completed≠judgment_ok）；也不断言成本等价（pull 轮级 LLM 用量更高，G3-RULING §2.2 既有事实，本采样 17 vs 6 同向）。
4. **对 G3 切换裁定的含义**：行为差异轴不再构成"未证等价"的阻塞项；切换前置仍卡在前缀纪律轴（归因卡②：P1 计量+协议段拆分修复）与成本轴——维持 G3-RULING"不切换"建议的依据结构不变，行为轴从"待归因"改记"已归因=风格差异"。

## 4. 覆盖边界与诚实声明

1. **采样规模**：每（族×模式）1 轮（≥3 族×2 模式卡面要求满足）；倾向性比例是方向性读数不是统计估计——卡面验收=分层工件+等价性结论成文，未要求功效检验。
2. **会话内前缀漂移**：本卡不重测 P2（G3 首轮已测 pull 6/13）；注入面为会话首装设计，续跑轮 context merge 的快照刷新语义=引擎态不变则字节恒等，变则如实断裂（与 pullharness.protocol 同类，登记面归归因卡②拆分修复）。
3. **pull base r1 边界持久化失败**（单次）：`durable_checkpoint_persist_failed`——内部链已 completed/done（agent 日志 [continuation.arm]），响应边界因 workspace 持久化失败降级为 failed（`recordGoalResult`→`persistCurrentProjectWorkspaceChecked` 锁竞争/写入路径候选；同 run 后续 2 个 completed pull goal 同路径成功）。非判定链缺陷、非本卡文件域；**登记主管**：边界持久化失败的可复现性/加固可另立小卡（该降级路径两模式共用）。
4. VSP realtime publish（127.0.0.1:8787）全程拒连 WARN（berth 无该服务）：两模式同载、不影响判定链，不入失败类；如实披露。
5. chat 直连管线未迁移（OQ-H3 不变）；渲染面零改动。

## 5. 给主管的登记项

- ①边界持久化降级面：`durable_checkpoint_persist_failed` 单次瞬态（§4.3）——建议小卡取证或加固（非本卡域）。
- ②冷启动续跑快照刷新语义（§4.2）：若归因卡②拆分修复时希望底座段绝对字节恒定，可在 continuation merge 面保留首装快照键（一行语义，非本卡验收面）。
- ③切换裁定：行为轴清偿（§3 结论），前缀纪律轴与成本轴仍前置（归因卡②范围不变）。
