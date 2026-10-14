# 腿 E 取证卷（rerun4，裁定三授权；只读，未启栈）

基线：origin/main 4476b9ed（含腿 D cherry-pick 3ec1cf96）；worktree D:/Vit_DAW_wt_fsdc1r2 @ port/fix-fs-settle-deadchain-1-rerun4。
证据源：coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/（debug jsonl run8×5/run9×4、telemetry、agent log 回指 worktree artifacts/free_state_d1_s1/）+ 工件 journal/observations/mixboard sessions + 代码锚点。全部只读取证，零栈启动。

## 一、run8 子形态：防重复门 5 连弹正当性 + 替代路径可达性

### 1.1 事实时间线（agent_last.log 20261014_111306 + journal/000001.jsonl）

| 时刻(+08) | 事件 | 证据 |
|---|---|---|
| 11:15:16→21 | 观察一 obs_373aa85e90fe：target=project, views=[project.structure, mix.multitrack_relationship], status=ready | journal rec 6/7 |
| 11:15:46→52 | 观察二 obs_e091826f6b60：target=track 1032(vocals), views=[**track.stereo_space, track.time_dynamics, track.activity_structure**], status=**partial** | journal rec 8/9 |
| 11:16:14 | revision=6 checkpoint；improvement_proposal 确认流 | agent log |
| 11:17:02 | rack_add_node=Pro-R 装载（mutation），rev 7→8 checkpoint（11:17:07/11:17:24） | agent log |
| 11:17:19 | `free_state_post_action_refresh` 仅刷 shadow（kernel.vsp_state_snapshot+refresh_shadow），无新观察 | agent log |
| 11:17:23–11:18:59 | 5 次 post-action 重观测请求全部被 `free_state_final_gate` 弹回（消息一致="the requested per-track CCB views already returned usable evidence for this track; decide from the recorded evidence or request a different cataloged view or target instead of repeating it"） | agent_message_loop_debug_run8.jsonl rec 0-4 |
| 11:18:59 | capability_blocked "closure made no material progress within its bounded observation window"，cont 4/6 | agent log |

5 次请求形态：rec 0-2 = views=[stereo_space,time_dynamics]；rec 3-4 收窄=[stereo_space]；rec 4 口头称"换一个尚未使用过的轨道级空间视图"但 view_ids 实际仍=track.stereo_space（debug raw 原文在卷）。

### 1.2 弹回正当性判定：按门自身语义=正当，但语义本身与阶段矛盾

- 弹回来自 per-view 可用性分支（agentloop/free_state_reasoning.go:1841-1854）：`available_views["track:1032::<view>"]` 有 usable 条目即弹。观察二（partial）按 `messageLoopFreeStateObservationStatusUsable`（:1750-1761，ready|partial 均算 usable）把三个视图都记进 available_views（落账链 :2219-2270）。模型 5 次请求的每个视图确实已在账 → **弹回按门自身规则正当**。
- 但这些条目全部录自 **revision 6（mutation 前）**；mutation 后 rev 7/8，模型需要的是 post-action 新鲜证据——per-view 分支**无 revision/新鲜度感知**（分支注释自述意图=防续轮浪费，:1834-1840），与 post_action_evaluation 阶段的强制新鲜证据要求直接矛盾。
- **revision 豁免结构性失灵**：`freeStateReObservationAdmitted`（agentloop/free_state_gate.go:1055）须先按 **整集 fingerprint** 在 receipts 找到 prior（:1061-1076），才能走 audioclosure.AdmitReObservation 的 admission 2（新 revision，audioclosure/diagnostic_round.go:359-363）。receipt 的 view set 是三视图整集，模型收窄为两视图/一视图后 fingerprint 不匹配 → prior==nil → **豁免对合法收窄的请求永远够不着**。豁免锚定"整集 receipt"、弹回锚定"per-view 可用性"，粒度错位。
- **提示规则与门矛盾**：ccb_model_prompt.go:670 明令 post_action 段"your FIRST action must be a ccb.observation_request **for the admitted experiment's view set on the applied target**"——教的恰是被弹的形状。

### 1.3 替代路径可达性判定：一半可达一半被封死

- **"decide from the recorded evidence"（从已录证据决断）= 结构性不可达**。requires_post_action_observation=true 时：settle 报告（free_state_reasoning.go:1085-1087）、blocked（:1214-1216）、needs_action（:1026-1031）、satisfied/diagnostic_complete/no_candidate（:1188-1189）**全部**要求本 reasoning cycle 内有成功观察请求（`messageLoopHasSuccessfulCCBObservationRequest`，:2666-2693，只认 state.executed/continuation trace）。弹回消息教的这条路被至少四门封死。
- **"request a different cataloged view or target"（换 view/target）= 原则上可达**。目录存在该轨未请求过的 track.* 视图（如 stereo_space/time_dynamics/activity_structure 之外），请求它们不被 per-view 分支弹回。模型 5 次均未真正换 view（rec 4 声称换实际没换）→ **run8 死因=教学含不可达分支（产品面）+模型未走可达出口（模型行为）的混合**。

### 1.4 run8 子形态结论

产品缺陷候选（非完全结构性阻断）：①弹回教学含不可达路径；②per-view 可用性分支无 revision/阶段感知、与提示规则 670 直接矛盾；③revision 豁免 fingerprint 整集锚定对收窄请求结构性失灵。修复面=agentloop 提示/门/豁免三处，域外于本卡腿 E 文件域，上交决策侧。

## 二、run9 子形态：观察包部分返回（工程绑定缺失）不入 D1 门

### 2.1 事实时间线（agent_last.log 20261014_112339 + journal）

| 时刻(+08) | 事件 | 证据 |
|---|---|---|
| 11:26:11/11:26:33 | pre-action 观察：project 级 obs_45e860a0175a + track 1017(guitar) obs_9699037f563e（views=[stereo_space,time_dynamics]），**绑定正常 revision=9/epoch/hash 在卷** | journal rec 10-13 + observations/*.json |
| 11:27:34 | rack_add_node mutation；11:27:39 revision=10 checkpoint | agent log |
| 11:27:58 | free_state_post_action_refresh + capacity_project_facts 两次 shadow 刷新（均 ok） | agent log |
| 11:28:04→08 | **post-action 重观测请求执行成功** obs_63be1343cd9e：请求带 `priority_reason=contradiction`+`unresolved_questions`（journal rec 26 args_summary 原文）→ 通过防重复门（AdmitReObservation 矛盾豁免路径），返回 stereo_space 相关度 0.689、time_dynamics partial | journal rec 26/27 + observations JSON |
| 11:29:05–11:32:44 | 4 次 settle 尝试打地鼠（state=""↔attempt=0 交替，2/4 连 evidence_refs 也缺）被 final gate 逐次拒收 | agent_message_loop_debug_run9.jsonl rec 0-3 |
| 终局 | terminal="D1 requires one post-action CCB observation"，exit 1 | d1_smoke_report + console_d1_reverb_run9.txt |

### 2.2 绑定缺失锚点链（产品面，实锤）

1. 持久化 ObservationPacket 对比（.vit_agent/vitproj_b3f2.../observations/）：
   - obs_20261014T032633_9699037f563e（pre-action）：project_package = {project_uuid ✓, project_epoch='epoch_fda404440200b4f4', **project_revision='9'**, project_state_hash='8d400c39029775b'}；
   - obs_20261014T032804_63be1343cd9e（post-action）：project_package = {project_uuid ✓, **project_epoch='', project_revision='', project_state_hash=''}** ——身份三元组丢失（uuid 仍在）。
2. 组装面：harness/ccb_observation.go:134-157 读 mixboard store `observation.binding` → mixboard/catalog.go:1017 `observationBinding()` 从 `obs.ProjectPackage` 抄身份 → ProjectPackage 由 mixboard/project_package.go:10 `buildProjectPackage(state)` 生成，state=harness.go:3357 `h.UserStateSummary(ctx)`（shadow 摘要）。
3. shadow 身份来源：harness.go:1844 `vspLegacyStateWithBinding`——仅当 VSP 快照 reply 带 `revision>0`/`project_epoch`/`snapshot_hash`（kernel/vsp.go:272-275 解析 reply 顶层字段）才写入；refresh 用 `shadow.Initialize` **整体覆盖**，快照缺身份时最后已知身份被清空。
4. 直接证据：run9 mix session mix_05c5a190bc7b/current.json 的 latest_change（shadow_change_000041）记录 `project_epoch: before=epoch_fda40444 → after=<缺失>`、`project_revision: before=11 → after=<缺失>`、`snapshot_hash: before=... → after=<缺失>`、`graph_revision: after=1`——**mutation 后某次 VSP 状态快照回复未带身份三元组**（graph_revision 计数器出现替代形态）。
5. 后果链：binding unbound → capabilitycontext/free_state_observation.go:522-531 canonicalBound=false → bundle limitation "canonical VSP project binding unavailable"、freshness=unbound、status=partial（模型自报"观察包为部分结果：工程绑定缺失"与之对应，debug rec 2 limitations 原文在卷）→ chat/free_state_reasoning_loop.go:1275 `freeStatePostActionObservationEligible` 要求 `row.ProjectRevision` 非空 → 该观察**永远不能记为 post_action=true** → round.observations 无 post-action 行 → D1 断言 `require(len(post_observations)==1, "D1 requires one post-action CCB observation")`（scripts/free_state_d1_smoke.py:1693-1694）结构性失败。
6. **对照组（间歇性证据）**：run6（20261014_012927）同代码路径的 post-action 观察 obs_20261013T173322_3ab2000a43df **revision='7'/epoch/hash 正常**——run6 因此越过 D1 post-action 门死在 materiality schema（terminal="acoustic materiality record is missing"）。同代码 run6 有绑定、run9 无 → **时序相关间歇缺陷**（快照落在 graph 变更窗口时身份丢失），非确定复现、非模型行为。

### 2.3 run9 子形态结论

**产品缺陷实锤（非模型行为）**：模型全程正确（矛盾豁免合法通过防重复门→观察执行成功→诚实报告 limitation→尝试 settle），是"post-action 证据获取→记账"链路断在工程绑定组装。任何模型档位在该时序触发下都会死同一门（v4-pro 空烧风险的裁定判断成立）。修复候选面（均在本卡文件域外，上交不扩域）：
- agent 侧硬化：harness 刷新时**保留最后已知身份**而非空身份整体覆盖（harness.go:1844 一带）；
- kernel/VSP 侧：graph 变更后 state.snapshot reply 恒带 revision/epoch/snapshot_hash（C++ 面，`graph_revision after=1` 形态）。

## 三、总裁定

①取证=**实锤产品缺口**（run9 子形态硬实锤=绑定链断；run8 子形态=缺陷候选+模型行为混合，教学不可达分支+豁免粒度错位+提示矛盾三锚点在卷）。按裁定三：**停止、不启栈、不扩域自修，卡面登记上交**；③复跑腿不执行（前提"①未实锤"不成立）。②打地鼠硬化照常交付（commit 1923b22a @ port/fix-fs-settle-deadchain-1-rerun4，已推 origin）。

60s 超时口径：本腿零真栈运行，无新超时样本；既有观察（4/14→1/14→0/13 无升级趋势）维持。
