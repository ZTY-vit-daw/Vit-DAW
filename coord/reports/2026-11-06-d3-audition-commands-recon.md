# audition.* 十命令族全仓勘察（D3-REF-ALIGN-THIN-1 前置裁定证据）

- 勘察目的：为 D3-REF-ALIGN-THIN-1 的前置裁定「补 agent catalog 面 or 维持缺口登记」收集证据。
- 缺口登记出处：MIXLAYER-M1-4（commit e0e2f4ac，提交信息原文：「D3 RLM user intent+RenderBindings fail-closed 披露+参考曲命令缺口如实登记(audition.* 十命令在内核 dispatcher 不在 agent catalog,本卡不补)」）；卡片登记见 `coord/cards/done/2026-10-11-MIXLAYER-M1-4-d1d3-observation-pack.md`:16 与 `agent/internal/chat/d1d3_premaster_observation_runtime.go:62`（`d1d3ReferenceTrackGap` 常量）。
- 勘察方式：纯只读（grep/rg + 定点读文件），零构建、零测试运行、零仓库改动（本报告除外）。

## ① 内核侧命令清单（VitApp/，C++）

### 注册与门禁位置

- **命令注册**：`VitApp/Source/Service/CommandDispatcher.cpp` `registerBuiltinCommands()` 2431–2491 行，十条 `handlers.emplace` 全部委托 `AuditionPreviewService` 对应 handler；service 不可用时统一 `makeErrorReply("Audition preview service unavailable")`。
- **渲染期门禁**：同文件 2372–2403 行 `renderAllowlist`——十条 audition.* 全部在允许清单内（内核渲染中仍可响应，其他命令被拒 "Engine is busy rendering"）。
- **VSP 映射**：`VitApp/Source/Service/VspKernelReference.cpp` 131–140 行，十条命令的 VSP 别名映射，**isMutation 标志全部 = false**（非变更类）。
- **只读分类**：同文件 146–185 行 `legacyReadOnlyCommands()`——十条全部列入只读命令集。
- **能力声明**：同文件 309 行 capabilityList 含 `audition.preview` 能力；1064–1068 行会话 hello_ack flags：`audition.preview=true`、`audition.preview.audio_file_target=true`、`block_boundary_switch=true`、`crossfade=true`、`full_project=false`（目标音频 spike 只支持 audio_file 目标，不支持整工程预览）。
- **确认语义**：CommandDispatcher 内无 audition 专属确认门（grep confirm 无相关命中）；权限语义体现为上面三张分类表（只读/非变更/渲染期放行）+ 两个 fail-closed 存根命令。

### 实现文件

`VitApp/Source/Service/AuditionPreviewService.{h,cpp}`（JSON 命令适配层，537 行）、`AuditionPreviewState.{h,cpp}`（状态机，SessionStatus=preparing/ready/playing/stopped/stale/failed；CandidateStatus=preparing/ready/stale/failed）、`AuditionPreviewAudioPlane.{h,cpp}`（音频面：解码/重采样/warm-up/块边界切换/交叉淡化）。单测：`VitApp/Tests/AuditionPreviewStateTests.cpp`、`AuditionPreviewAudioPlaneTests.cpp`。

功能语义：A/B 试听预览面（Audition Preview Plane）。prepare 两个候选音频文件（当前喂入者=agent 侧 mix-tick/semantic-dynamic 流程经 `start_render` 渲出的 before/after WAV），内核解码+重采样到设备采样率+warm-up 后自产 preview_ref（客户端传入的 preview_ref 在 AuditionPreviewService.cpp:106 被清除，不作为就绪依据）；select 在块边界切换音源并交叉淡化，工程 transport 只作位置锚（AUDITION-PLAY-1：select 默认 auto_start=true，选了就出声）；会话状态机从不 checkout/render/reload/mutate 活动工程（AuditionPreviewState.h 注释明示）。

### 十命令逐条（入口均在 `AuditionPreviewService.cpp`，返回面共用 `resultToReply`:475 信封 `{status, command, message, code(仅错误), session}`；session 含 session_id/conversation_id/scope/status/state_revision/active_candidate_id/active_project_plane/audition_preview_plane/transport/candidates[]）

| 命令 | 入口（文件:行） | 参数面 | 返回面 | 权限/确认语义 |
|---|---|---|---|---|
| audition.prepare | handlePrepare :131 | 必填 session_id/scope/active_project_ref/active_project_revision/timeline_revision；候选=candidate_a+candidate_b 成对（或 candidates[]，agent 侧封装硬性要求恰两个）；每候选必填 source_kind+source_ref，checkpoint_ref/commit_id、branch_ref/worktree_ref、project_revision 按来源补齐校验（:77–96 六类 validation_error） | ok: session+逐候选 audition.candidate.ready 事件；错误: validation_error / 音频面失败 markFailed+audition.failed 事件 | 只读分类；音频面硬要求 `source_kind=audio_file`（AuditionPreviewAudioPlane.cpp:340），source_ref 解析为文件路径（支持 `file:` 前缀 :239），文件不存在=candidate_audio_missing |
| audition.status | handleStatus :207 | session_id | session 快照 | 纯读 |
| audition.ready | handleReady :219 | session_id/candidate_id | ok: session + audition.ready 事件；音频面未预备=candidate_not_prepared | 就绪判定权在内核音频面（preparedPreviewRef 由内核自产） |
| audition.stale | handleStale :240 | session_id | session + audition.stale 事件 | 失效音频面+会话置 stale |
| audition.failed | handleFailed :254 | session_id | session + audition.failed 事件 | 失效音频面+会话置 failed |
| audition.select | handleSelect :268 | session_id/candidate_id；可选 auto_start(bool, 默认 true) | ok: message=audition.select.changed + session + 事件；候选未预备=candidate_not_prepared；切换失败=audio_source_switch_failed | 块边界切换+交叉淡化；transport 仅锚位；auto_start=false 保守静默选择（AUDITION-PLAY-1） |
| audition.position | handlePosition :326 | session_id；position_seconds(finite≥0) 与/或 is_playing(bool) 至少一项 | session + audition.position.changed 事件 | 音频面无会话=audio_session_not_found |
| audition.stop | handleStop :364 | session_id | session + audition.stopped 事件 | 停音频面播放+会话 stop |
| audition.inspect_candidate | handleInspectCandidate :379 | session_id/candidate_id（先校验格式） | **恒定错误** `capability_not_supported`（"explicit Active Project Plane operation…not implemented by the target audio spike"） | fail-closed 存根 |
| audition.apply_candidate | handleApplyCandidate :394 | session_id/candidate_id（先校验格式） | **恒定错误** `capability_not_supported`（"explicit project mutation…not implemented"） | fail-closed 存根（项目变更类，明确不做） |

事件面（publishStateEvent :500）：topic=audition，subtopic=audition.prepare.started / audition.ready / audition.candidate.ready / audition.select.changed / audition.position.changed / audition.stopped / audition.stale / audition.failed（错误统一 audition.failed）。

## ② agent 侧缺席亲证

**检索 1（工具目录本体）**：`grep -ic "audition"` → `agent/internal/tools/catalog.go`（1279 行，DefaultCatalog+defaultSpecs+别名表 :76–158）= **0**；`capability_pack.go` = **0**；`catalog_test.go` = **0**。目录无任何 audition 工具 spec、无别名、无 raw 内核命令逃逸口（无 raw_command 类工具；`kernel_command` 仅是 Tool 结构字段）。

**检索 2（能力/执行面）**：`grep -rni audition agent/internal/{harness,tools,capabilityadapters,capabilityruntime,capabilitycontext,toolpolicy,executor}` = **0 命中**。

**检索 3（全 agent 树命令字符串）**：`grep -rn "audition\.(prepare|select|stop|position|status|ready|stale|failed|inspect_candidate|apply_candidate)" --include=*.go`（非测试）按文件分布：audition_events.go 20、kernel/audition.go 11、semantic_dynamic_audition.go 6、mix_tick_audition.go 6、audition_candidate_adoption.go 4、task_semantics.go / free_state_reasoning_loop.go / free_state_d1_runtime.go 各 1。**全部是程序化通道或事件词，无一在 LLM 工具目录。**

agent 侧相关存在物（缺口之外的完整清单）：

1. **内核客户端封装**：`agent/internal/kernel/audition.go`（116 行）——十个类型化方法 AuditionPrepare/Status/Ready/Select/Position/Stop/InspectCandidate/ApplyCandidate/Stale/Failed，逐一 `SendVSPCommand` 透传。这是基础设施能力，不是 LLM 面。
2. **chat Server 内部接口**：`agent/internal/chat/audition_events.go:150-156` `auditionCommandClient` 接口**仅声明四命令**（Prepare/Status/Select/Stop），注释与 M1-4 卡一致（"仅四命令、只服务 A/B 试听回路"）。Ready/Stale/Failed/Position/InspectCandidate/ApplyCandidate 六个不进 chat 运行时接口（封装在 kernel.Client 上存在但未被 chat 消费，Ready 仅在 reseat 链路外无调用点）。
3. **程序化调用点**：`mix_tick_audition.go:382`、`semantic_dynamic_audition.go:385` 直接 `AuditionPrepare`（before/after 渲染 WAV 落 artifact store 后喂文件路径）；`audition_events.go:431/452/457/469` select/reseat 链路。
4. **HTTP 面（用户/UI 面）**：`server.go:693-698` 六条路由 `/agent/audition/{status,select,stop,judgment,inspect_candidate,apply_candidate}`。注意：agent 的 inspect/apply 是**agent 本地实现**（`audition_candidate_adoption.go`：从 candidate 恢复 checkpoint/commit/worktree 引用做候选采纳），**不是转发内核**——内核同名命令是 fail-closed 存根，同名异义。
5. **事件消费**：内核 audition.* 事件经 `server.go:681 → ingestAuditionTelemetry`（audition_events.go:175）进入会话，发 `vit.kernel_audition.v1` AgentEvent；webui `src/audition.ts` + 测试消费（判定卡/盲态/解盲披露）。
6. **提示词散文**（非命令族）：agentloop `ccb_model_prompt.go:162/164`、`free_state_reasoning.go:1268`（`human_audition_ready` outcome 枚举）、`full_access_autonomy.go:17/69/82`（"hand the result to the user's A/B audition"）——语义是「人耳 A/B 试听」边界表述。

结论：**预期=无 catalog 暴露，亲证成立**。缺口精确边界 = LLM 可见的工具目录零 audition.*；程序化通道（4 命令接口 + 全 10 命令封装）与用户 HTTP 面均存在。

## ③ 数据流关系与 D3 影响

**内核 audition.* 消费/生产**：
- 消费：候选音频文件（source_kind=audio_file 的文件路径；现役喂入源=agent 侧 `start_render` 渲出的 before/after WAV，落 artifact store `mix_tick_audition_render/` 与 semantic-dynamic 对应目录）；活动工程 transport 锚（position/is_playing/sampleRate，captureAuthoritativeTransport :526）；会话身份（session/scope/project_ref/project_revision/timeline_revision）。
- 生产：会话状态 JSON（audition_preview_plane + transport + candidates 全字段）；topic=audition 事件流；设备输出上的预览播放（块边界接管输出块）。
- 不消费、不产生任何 RLM 数据；不 mutate 工程（apply_candidate fail-closed）。

**RLM（agent/internal/rlm，reference_level_model.projection.v0）**：
- `rlm.Build` 入参（types.go Input）：UserIntent（用户原话 req.Message）、ProjectState（kernel VSPStateResult）、MixObservation（mix.observe digest）、AudioAnalysisStatus、RenderBindings（render→delivery-profile 绑定集）。
- 产出：参考电平行/校准行/指标 profile/limitations 的观察投影；RenderBindings 只在入参携带时披露（fail-closed，unresolvable 行原样可见）。
- 现役调用点三处：B1 gain staging（`capabilitycontext/gain_staging.go:88`）、harness 绑定披露（`harness/render_profile.go:56`）、**D3 面（`chat/d1d3_premaster_observation_runtime.go:291` d1d3BuildRLMProjection：UserIntent+ProjectState+MixObservation+`harness.RenderProfileBindingsSnapshot()`，B1 同款 Build 入参）**。

**同源性判定**： audition 与 RLM 只在「render 工件」这一层沾边（audition 候选=渲染 WAV 文件；RenderBindings 绑 render id→delivery profile），**无直接数据通路**：RLM 不读 audition 会话，audition 不读 RLM。D3 面（M1-4）已把 RLM 投影接为参考电平/绑定的**观察面**，缺口登记行由冒测门断言（`scripts/fs_largeproject_smoke.ps1` D1D3Observation 门，observation_gap_line 检查 :1639-1640）。

**D3-REF-ALIGN-THIN-1 的影响点**：
1. 参考曲对齐的**观察面已存在**（RLM 投影 + RenderBindings 披露，M1-4 收口）；缺的是**执行面**——agent/LLM 没有任何工具能驱动「听参考曲」（prepare/select/stop 一个参考音频文件的试听会话）。
2. 内核音频面**天然兼容参考曲文件**：source_kind=audio_file 只要文件路径，不关心来源——参考曲若经 import/artifact 注册拿到文件路径，audition.prepare 即可装载；`full_project=false` 限制的是整工程预览，不是外部音频文件目标。
3. 但「参考曲导入」本体也无 catalog 载体：现有 catalog 只有 `render.start`（:1217，RiskConfirm，mutates_project=true）等渲染面；参考曲文件的获取/注册/授权路径（workspace_blob_copy、media_register_assets 等在册工具能否覆盖）需裁定者另行核对，本勘察未展开（超范围）。
4. 同名异义陷阱：若未来补 catalog，`audition.inspect_candidate/apply_candidate` 两条**不能**直接编目转发内核（内核是 fail-closed 存根）；agent 已有本地候选采纳实现，语义归属需裁定。
5. agent 内部接口仅四命令是有意收窄（M1-4 注释明示「只服务 A/B 试听回路」）；补 catalog 若要走全十命令，六个未消费命令的参数面/风险语义需先对齐（Ready 的 preview_ref 语义已被内核收权，Stale/Failed 是会话治理命令）。

## ④ 两选项的证据要点（不替主管裁定）

**选项 A：补 agent catalog 面——支持证据**
- 内核面完备且有测试背书：十命令注册齐全、只读/非变更/渲染期放行三分类现成、状态机与音频面有专属单测（AuditionPreviewStateTests / AuditionPreviewAudioPlaneTests）。
- agent 已有全十命令类型化封装（kernel/audition.go），catalog 化的边际工作是 spec+argspec+executor 接线，不是新协议。
- 目录存在 `Implemented=false` 机制（CommandSpec.Implemented），inspect/apply 两条存根可如实编目为未实现，不破坏 fail-closed。
- 先例：`render.start`（RiskConfirm）已在目录，音频渲染物理面已 catalog 化；audition.prepare 消费的正是渲染产物，属同族执行面。
- D3 卡目标 2/3（判据+runtime）若走「建议挂确认链」以外的任何真执行（实际听参考曲），没有 catalog 载体则执行面必须留缺口（即卡内停止条件第 1 条形态）。

**选项 B：维持缺口登记——支持证据**
- 现役设计是**人耳在环**：audition.select 的现役触发者=用户（webui 判定卡→agent HTTP→内核），full_access 提示词明文「hand the result to the user's A/B audition」；把 select/position/stop 编目给 LLM 等于把播放控制权移交 agent，是对现有盲态判定语义（B12-1 blind/discardable）的语义扩张，不是薄补面。
- 八命令已有程序化通道服务两条现役回路（mix-tick/semantic-dynamic A/B）；真缺的是「参考曲导入+对齐编排」这层新编排，catalog 条目本身不解决文件获取/授权/盲态设计，缺口主体不在编目。
- M1-4 已把缺口登记为可断言事实（常量+冒测门+卡片），维持登记=单一诚实缺口线；扩 catalog 属能力边界扩张（D3 卡明文「禁止未经裁定自行扩 catalog」，裁定后也须按 §7 证据等级走真栈验收）。
- 风险语义未定：prepare 触发解码/warm-up（CPU 峰值）、select 触发音频输出接管，RiskLevel/RequiresConfirmation 如何定标无先例直援；编目前需一轮语义定标，非机械搬运。

**双侧共同事实**（裁定可依赖）：内核十命令面就绪且分类完备；agent 封装现成；chat 内部接口有意收窄为四命令；inspect/apply 同名异义（内核存根 vs agent 本地采纳）；RLM 观察面已收口且与 audition 无数据耦合；「参考曲对齐」执行面的最小充分载体=（参考曲文件入工程受控路径的手段）+（audition.prepare/select/stop 的编目或等价程序化编排），二者缺一则执行面留缺口。

---

勘察人=flash-worker 子代理（GLM 主会话派发）/ 2026-11-06 / 纯只读除本报告外零改动
