# A-F 全族体检报告（257 AF-FAMILY-HEALTH-1）

- 编制：ZCode GLM-5.3 L1 PC 执行会话 / 2026-11-05 晚窗
- 被测基线：origin/main `15ef1050`（领取提交 d2a77b9d，worktree `D:/Vit_DAW_wt_257impl`）
- 依据：[任务卡](../cards/done/2026-11-05-AF-FAMILY-HEALTH-1.md)（领取时在 doing）+ [终局裁定](../../decisions/2026-11-05-af-scaffolding-endstate.md)（F/H/A 三态）+ [A-F 开线勘察](2026-11-05-af-line-survey.md)
- 性质：双面体检——面 1 横向回归扫描（B/C 族既有腿当前 main 重跑+引用收口）、面 2 自主性分级首盘（固定路由面/harness 可达面/拆减准备度矩阵+ablation 腿模板设计，**不执行 ablation run**）

---

## 一、面 1：全族回归扫描健康矩阵

### 1.1 零栈腿（go test 面，worktree 内执行）

| 族 | 腿 | 跑法（§8 口径） | 结果 | 判定 |
|---|---|---|---|---|
| C1 | `scripts/run_c1_frequency_cleanup_smoke.ps1`（frequencycleanup+capabilitycontext+capabilityadapters+orchestration(+runtime)+chat+harness+mixboard 七包） | 确定性 go test，零 LLM 零真栈；成功=测试全 ok | 首轮 chat/harness 两包 5 测试 FAIL → 取证后**删残留复跑全绿**（chat 全量 159.4s ok） | **PASS**（首轮失败=测试隔离缺陷非回归，见 §二） |
| B1 | agentloop B 族契约定向：`TestB1PackSurvivesProjectChangeInRun`/`TestB1PackStalenessRevisionSemantics`/`TestGainStaging*`×3/`TestB12Completion*`×2 | 同上 | ok 0.167s | **PASS** |
| B(queue) | `TestMixWorkflowQueueStatusRequestPredicate`/`...NodeStateCommandParse`/`...FastPathRegistrationContract` | 同上 | 同批 ok | **PASS** |
| C2/C3 | chat 包单测 `TestC2*`×33 / `TestC3*`×15 / `TestDecodeC1TreatmentPlan*`（随 chat 全量） | 同上 | 全量 ok | **PASS** |
| harness 横切 | `TestV2ProjectStoreSurvivesRestartAndContinues`（首轮 FAIL） | 隔离复跑 | ok 0.285s | **PASS**（同轮污染形态） |

### 1.2 真栈腿（§2.2 分批登记，与 256 错峰）

| 族 | 腿 | LLM 预期 | 结果 |
|---|---|---|---|
| C2 | `scripts/run_de_esser_control_validation.ps1 -Phase Matrix`（自起全栈：build agent+vsphub→Godot→内核 5555/5556→7878/8787，py matrix 驱动，finally 自拆） | **零 LLM**（grep chat/llm 零命中） | **FAIL=腿过时非回归**：9/9 case 同型 `pca_load_gate: missing non-forgeable exact selection authorization`——PCA 装载门（4fc5dff0 2026-08-08 落位 harness.go:2247 `enforceAgentProcessorLoadGate`）要求 UI 选择授权或 full-project-access context；matrix py 腿共享 invoke（compressor_compat_matrix_smoke.py:49）POST /agent/invoke **不带 context** → 装载第一步结构性被拦。门本身拦截正确（设计内），腿自 2026-08-08 同日落位后未跟上授权面。**compressor 系列三腿共享同模块=同型根因不再跑**（G5）。工件 coord/runs/AF-FAMILY-HEALTH-1/deesser_matrix_20261105_224002/（9 case json）；栈 finally 自拆亲证（端口/进程全零） |
| B1-B3 | `scripts/run_mix_single_tick_e2e.ps1`（mix 工具腿=contract spec 执行链 mix_propose_tick→apply→track.volume/pan） | LLM 参与（/agent/chat，settle 720s；flash 引擎预算 ≈¥1-3/run，§8 单 run 无重跑） | **FAIL=模型行为分支非链路回归**：observe turn 断言 `got 'done', want needs_confirmation/improvement_proposal`（run 224459）。**工具链健康亲证**：`ccb.observation_catalog`→`ccb.observation_request`×2 真实走通（executed=3），首 turn 186.9s limit_reached 切片→durable continuation 排干 completed/done，63 字终局。话术=「帮我看整体混音，只建议一个小幅音量调整，**先等我确认**，不要用插件」——话术含确认期待，模型以终局文字给建议未挂正式 proposal 卡（256 措辞漂移假说同族：M3 链后自由态行为面变化）。**待主管行为面裁定**：needs_confirmation 期望是否仍是现行契约（是→行为回归卡；否→断言过时修腿）。形式失败两轮（反斜杠吞路径/无内核二进制）栈未起零 LLM；内核=wt_228 stock F9E3DA0A 只读复用；工件 coord/runs/AF-FAMILY-HEALTH-1/mix_tick_20261105_224459/；throw 路径不自拆 owner 手动拆核净 22:51:27 |
| B4 | `scripts/b13b_frontier_masking_smoke.ps1`（masking frontier：B4 观察面 frequency_relationship+masking） | LLM 参与（同型 prompt 驱动） | **不跑（决策记录）**：同为 LLM 行为面断言腿（closure frontier 形态依赖模型行为面），批 2 已代表性地暴露行为面断言错配风险；B4 观察面单测绿（chat 全量含 masking 投影）+词表/守卫面单测绿；预算节制（§8） |

真栈不跑项（记录理由）：`run_project_package_reopen_c1_smoke.ps1`（C1 持久化重开面——写 `VitApp/Workspace/Artifacts` 工件区、两阶段 product path 耗时长；C1 能力契约面已由零栈腿覆盖，该腿属持久化回归非能力面健康，留后续按需）；`run_static_eq_external_holdout_smoke.ps1`/`run_waves_static_eq_phase3_smoke.ps1`（generic-EQ/holdout 面属 semantic_eq 闭环而非 A-F 族腿）；compressor 系列三腿（COM 观察/派生面，非 C2 控制面主线；de-esser Matrix 已代表 C2）。

### 1.3 引用收口（不重跑，卡面省钱纪律）

| 族 | 引用 | 终态 |
|---|---|---|
| C4 | [M3 链收口报告](2026-11-05-m3-chain-closeout.md)（2026-11-05，全链验收合入） | 组 1 时域渐变**稳定绿**（247 首绿+248 三 run 3/3+253 回归 4/5，tool_chain_ok 13/15，残差 2/15=模型行为观察项）；组 3 时间窗 **5/5 lexicon_hit 转绿**；组 2 空间类=设计内诚实边界（group_pass=false 定案）；治理红线全链成立（零裸写/单次确认/park 零风暴） |
| D1 | 254 终验（c3833c67：实现面 pass 合入，运行面挂起）+256 在飞（verifyrun1 占用中） | **in-flight**：实现面=LW 触发链+D1 settle 面+queue view D1 节点点亮；运行面挂起形态=armrun1 排干超时（瞬态延迟峰）+armrun2 零 mutation 诚实分支×133 断言「恰好一次」错配——两形态归因 256 取证中 |
| A1-A5 | journey/probe 时代腿（FS-LARGEPROJECT-SMOKE 基线：A1/A4 pass；A2/A3 软记录 capability_blocked 归 139 返工线；A5=段落标记能力 225 已入能力目录披露） | 抽查记录：A2/A3 观察底座 TIM/TOM 投影在库（`agent/internal/tim`/`tom`），journey 腿 GREEN 记录在案；本卡不重跑（勘察报告口径：A=可选抽查） |
| C5/D2-D4/E1-E4/F1 | 勘察报告+queue view 源码 `mixWorkflowQueueNodes`（capability IDs=nil） | **not_built**（无 runtime 可体检） |

---

## 二、面 1 问题清单（三分类）

### 断言过时（assertion stale）：**0 项定性 + 1 项待裁定（G6）**

零栈面 5 处失败全部取证为测试隔离缺陷，无「断言期望落后于已合入行为」定性形态（D1 教训判定流程逐个走完：隔离复跑→判词表/行为分支→定性）。真栈面 G6（mix_single_tick observe 断言）为**待裁定形态**：是断言过时还是行为回归取决于「needs_confirmation 是否仍是现行契约」——交主管裁定，本卡不预判。

### 真回归（true regression）：**0 项**

零栈面隔离复跑+全量复跑全绿；真栈面批 1（G5=腿过时被门拦）/批 2（G6=模型行为分支）均非能力/链路回归——批 2 工具链健康亲证（观察工具三步真实执行+continuation 正常排干）。

### 缺口/缺陷（gap，含 flaky/腿过时/行为面）：**7 项（G1-G7）**

| # | 问题 | 证据 | 处置建议 |
|---|---|---|---|
| G1 | [flaky·测试隔离] `loudnessWatchVerdictStorePath()` 落**全局 Temp 路径**（`%TEMP%\loudness_watch_verdicts.json`）无 per-test 隔离：`TestSettleWithEnabledWatchRecordsVerdict` 真写该路径，同轮后跑的 `TestSettleWithoutWatcherStaysSilent`/`TestD1LoudnessSettleRealtimeConfigDisclosesGapWithoutVerdict` 的 must-not-exist 断言假失败；残留文件跨轮存活（本机 22:25 残留实证） | 首轮 chat FAIL×3 → 删残留后 4 测试隔离复跑 ok+chat 全量 159.4s ok | **转修复卡**（chat 域，文件域=loudness_watch_trigger_test.go+verdict store 路径注入面；修法=t.TempDir 注入或 t.Cleanup 删残留） |
| G2 | [flaky·全局态] `TestRecordFreeStateSettlementPersistsAcrossSessions` 依赖 projectstore 全局 Activate 态，包内顺序敏感（他测试 Deactivate 后 conclusion dir 不落盘） | 同上复跑绿 | 并入 G1 修复卡（同族：chat 包全局态隔离） |
| G3 | [脚本断言缺口] `run_c1_frequency_cleanup_smoke.ps1` 三条 `go test` 后**不检查 `$LASTEXITCODE`**——go test FAIL 时脚本仍 exit 0（首轮实证：chat/harness FAIL、PS exit 0） | 首轮任务通知 exit code 0+脚本源码零检查点 | **转脚本修复卡**（+1 行 `if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }`×3；本卡红线=各族脚本零改动，不代修） |
| G4 | [flaky·顺序敏感] harness `TestV2ProjectStoreSurvivesRestartAndContinues` 首轮 FAIL（evidence refs=nil）→隔离复跑 ok 0.285s；同轮 projectstore/mixboard 全局态交叉嫌疑 | 见 §1.1 | 记录观察；若再现并入 G1 族修复卡（复现条件未钉死，不单独开卡） |
| G5 | [腿过时·PCA 授权面] de-esser/limiter/transient/compressor matrix 腿族（共享 `compressor_compat_matrix_smoke.py` invoke）经 /agent/invoke 不带 context，被 `enforceAgentProcessorLoadGate`（harness.go:2247）结构性拦截——腿写于门落位日（2026-08-08 4fc5dff0）后未跟上授权面；当前 main 上这些腿**无法作为验收门槛**（任何依赖其 exit 0 的验收都是假绿或假红） | 批 1 亲证（§1.2）：9/9 case 同型 gate 拒绝；栈/门/agent 全部按设计工作 | **转腿修复卡**（P2：matrix py invoke 接 full-project-access context 或 PCA 授权注入面；本卡红线不代修）；修前 C2 真栈健康以 33 个 TestC2* 单测为准 |
| G6 | [行为面断言错配·待裁定] mix_single_tick observe turn 断言期望 `needs_confirmation/improvement_proposal_confirmation_required`，当前模型（flash）对含「先等我确认」话术的观察请求走「观察→终局文字建议」分支（63 字终局，未挂正式卡）。工具链本身健康（观察工具三步真实执行）。**归模型行为观察项**（256 措辞漂移假说同族——M3 链后自由态行为面变化），非 B 族工具回归 | 批 2 亲证（§1.2）：agent 日志 executed=3+continuation completed/done+reply_len=63；257 行 telemetry 在工件 | **交主管行为面裁定**：①needs_confirmation 仍是现行契约→开行为回归取证卡（复现+归因 M3 链措辞）；②不再是契约→mix_single_tick 腿 observe 断言面更新（与 G3 同批修）。裁定前该腿 exit 1 不作 B 族健康反证 |
| G7 | [环境陷阱复现·零成本] 批 1/批 2 首轮形式失败：Git Bash 反斜杠吞路径（`D:Vit_DAW_wt_257impl`，AGENTS §5 已知陷阱，254 armrun1 同型）+worktree 无内核二进制需 -KernelExe（gitignored 构建产物不入库）——均栈未起零 LLM，不构成有效运行轮 | 两次 throw 均在栈启动前 | 记录入卡面回执；后续跑腿清单写明「正斜杠路径+显式 -KernelExe」两前置 |

---

## 三、面 2：固定路由面清单（逐族钉死）

> 固定路由面=拆掉对象：确定性意图词路由、固定 runtime 入口、choreo 词表。以下行号均指 `15ef1050`。

### 汇总表

| 族/节点 | 意图词/词表入口（确定性路由） | 固定 runtime 入口 | 固定面重量 |
|---|---|---|---|
| B1 | `static_mix_gain_staging_context.go:1719 messageLoopGainStagingCapabilityRequest`（fader reset/explicit 子词表）→ agentloop preflight `preflightStaticMixGainStagingContextPack`（:16，deterministic default mix.observe 重写） | agentloop preflight 链+`capabilitycontext` B1 pack | 中（~1700 行文件，preflight+pack 组装） |
| B2 | `capability_owner.go:212`（12+ 自然语言词：静态平衡/推子平衡/主次关系/突出主唱…+b2/b5 别名） | `capability_runtime_canary.go:33 handleCapabilityRuntimeCanary` B2 分支（Frozen ActionSet→Execution Coordinator→VSP readback+fresh MOM） | 重 |
| B3 | `capability_owner.go:221`（声像布局/pan layout/b3） | 同上 B3 分支（`handlePanLayoutRuntimeCanary`） | 重 |
| B4 | `capability_owner.go:226`（低频关系/低频遮蔽/b4…）+语义守卫（:52 普通 EQ 命令不进 B4，仅显式 marker `explicitB4InvocationMarker`:106） | 同上 B4 分支（`handleLowEndRelationRuntimeCanary`；v0 优先观察建议） | 重 |
| C1 | `capability_owner.go:231`（频段清理/频率清理/频谱清理/c1…）+守卫（:55 选中轨普通 EQ 不进，`explicitC1InvocationMarker`:111 含全工程/整首/full project） | `c1_frequency_cleanup_runtime.go`（613 行 canary）+`c1_frequency_cleanup_planner.go`（153 行 decode） | 中 |
| C2 | `capability_owner.go:239` **仅显式标记**（`fine_mix.dynamic_control`/`c2`，注释明示不做词法后继） | `c2_dynamic_control_runtime.go`（995 行：observation digest→CCB 观察→PCA admission→frozen plan→leaf 执行 stage 机） | **最重**（33 个单测锚定） |
| C3 | `capability_owner.go:246` **仅显式标记**（`fine_mix.space_depth`/`c3`） | `c3_space_depth_runtime.go`（421 行：只读白名单+观察+报告组装） | 中 |
| C4 | `automation_intent.go`：时间变体限定词 13 中文+11 英文（慢慢/逐渐/渐强/推起来/秒之后…slowly/gradually/ramp up…）+语义结果词表 level/space/tone 三类 30+ 词——**pre-LLM 武装**（注释自认 routing authority） | `automation_choreo.go`（choreo：pending confirmation→确认→automation curve 写内核→读回）+`automation_clarify_handoff.go`（澄清出口） | 重（M3 链 229→253 产物） |
| D1 | `capability_owner.go:256`（d1/d2/d3/全局响度检查/峰值检查/母带前检查/loudness check…）→观察 pack 内 `d1_loudness_settle_runtime.go:43 d1LoudnessSettleIntent`（9 词）分支 settle | `d1d3_premaster_observation_runtime.go`（观察）+`d1_loudness_settle_runtime.go`（settle）+`loudnesswatch` 包（trigger/compare/config ~1300 行） | 重（254 合入） |
| queue | `mix_workflow_queue_trigger.go:18` 标注正则（队列标记 X1 延期…）+:44 读词表（混音工作队列/队列状态…）——**记账面，非能力派发**（注释明示 never a capability dispatch entry） | agentloop fastpath×2（:449-450） | 轻（bookkeeping） |

### 固定面共同结构特征

1. **B2-C3 共用 canary seam**：`handleCapabilityRuntimeCanary`（capability_runtime_canary.go:30 注释"permanent B2/B3 abstraction seam"）——B2/B3/B4/C1/C2/C3 六能力一族一个 dispatch 面，sticky session+OwnerPolicy。
2. **词法后继退化梯度**：B2（12+ 词）>B3/B4/C1（4-8 词）>C2/C3（仅显式标记）——后来的族（MIXLAYER 时代起）已吸收「不做词法后继」教训，越新越薄。
3. **B 族契约披露面**（static_mix_capability_contract.go:74 询问词+能力词→确定性 contract reply）是纯披露，无执行；执行工具映射在 spec 内明示（mix.propose_tick→apply_tick→track.volume/pan/clip.gain）。
4. **A 族无 owner 路由**（A1-A5 不在 `inferProjectAwareCapability` 词表）；A 面走 TIM/TOM/DAD 观察投影+journey/probe 面。

---

## 四、面 2：harness 可达面清单（拆掉后的承接面）

### 4.1 工具暴露面（`harness.go` Invoke case 全量提取，与 A-F 相关子集）

- **观察族**：`mix_observe`/`mix_read`/`mix_derive`/`mix_report`/`ccb_observation_catalog`/`ccb_observation_request`/`band_energy_summary`/`spectral_field`/`stereo_relation_summary`/`loudness_summary`/`ref_query`/`ref_diff`
- **mix 执行族**：`mix_propose_tick`/`mix_apply_tick`/`mix_apply_static_balance_batch`（B2）/`mix_apply_pan_layout_batch`（B3）/`mix_rollback_tick`
- **DAW 原子执行**：`set_volume`/`set_pan`/`set_solo`/`set_mute`/`clip.gain.set_batch`/`clip.fade.set`/`rack_add_node`/`get_plugin_parameters`/`select_plugin`/`track.group.*`
- **A4 片段面**：`clip.strip_silence.suggest`/`clip.strip_silence.apply_batch`/`clip.fade.read`/`split_clip`/`resize_clip`/`move_clip`
- **D1 面**：`loudness_watch_config`/`loudness_watch_status`（hook 面需 free-state loop 先接线）
- **自动化面**：automation 曲线三命令走 VSP 内核通道（M3-KERNEL-1 交付：get_curve/set_points/clear_region），free-state invoke guard 有 automation admit 白名单（`free_state_invoke_guard_automation_admit_test.go`）

### 4.2 prompt 指引覆盖面

- **read-only 观察指引**（message_loop.go:4308-4312）：明示 ccb/mix 观察+read/list 白名单——观察族工具已获指引覆盖。
- **full-access autonomy 指令**（full_access_autonomy.go:68-76，FULLACCESS-AUTONOMY-1）：方向/剂量/处理器族/观察视图选择权**归 LLM**，observe→execute→explain→A/B 全链自主+guardrail bounce（:82 方向类问句打回）——**这是 harness 自主形态最强的现成指引面**。
- **普通 ReAct 骨架**（message_loop.go:4326+）：semantic_effect eq_plan 的 LLM 判断权（频点/增益/Q 由模型声学判断，禁 phrase-to-parameter 查表）+evidence 决策（reuse|read|derive|observe）——B4/EQ 闭环的 harness 形态基础。
- **能力目录披露**（225 交付）：澄清 LLM 轮次上下文含能力目录（含段落标记能力）——harness 轮次可自主提议打标记。

### 4.3 已有测试/运行证据中的 harness 形态

| 证据 | 形态 | 强度 |
|---|---|---|
| C4 组 1 tool_chain_ok **13/15**（M3 收口） | 措辞链产物在真栈经工具链自主走通确认→写达→读回 | **H 态实证**（残差 2/15=no_route 模型行为观察项） |
| C4 组 3 **5/5 lexicon_hit**（253 后） | 词表命中后**零 LLM 调用**确定性武装→澄清卡 | 固定面功效实证（拆掉前置依赖此项不回退） |
| 225 澄清 handoff（2026-11-05 pass ruling） | 词表命中但信息缺失→**带上下文移交 LLM 轮次**（判断权上移+冻结令：chat 域不得新增固定澄清文案） | harness 判断权已上移的先例 |
| G3-ATTRIB-1/QUOTA-RECOVERY-LEGS-1 `harness_ab` run exit 0 | 双模式健康链（quota_blocked=0）+full-access autonomy 形态在真栈 PASS | 横切面（非 A-F 节点专属） |
| C2 runtime 内部 `harness.Invoke` 调 `mix.observe`/`ccb.observation_request`（c2_dynamic_control_runtime.go:175-196） | 固定面自身经 harness 工具执行观察——工具面就是 C2 的执行基底 | 工具面复用可行性实证 |
| FULLACCESS-AUTONOMY-1 指令+guardrail | full access 下 observe→decide→execute→A/B 自主链指引就位 | 指引面就位实证 |

---

## 五、面 2：拆减准备度矩阵（节点 × 三态 × 证据 × 拆掉前置）

> 三态定义（终局裁定 §二-3）：F=仅固定路径可用；H=harness 路径已测存在；A=已过 ablation（可拆）。「拆掉前置」=关固定层后 harness 复现该能力所缺的件。

| 节点 | 现三态 | 证据锚点 | 拆掉前置（缺口） | ablation 就绪度 |
|---|---|---|---|---|
| B1 增益分级 | **F** | 零栈契约测试绿（本卡 §1.1）；preflight 固定路由 §三 | 工具已暴露（mix.observe/propose/apply+set_volume/clip.gain）✓；**缺**：增益分级目标判据指引（headroom/safety 语义无指引词）+验证器面（levelsafety 包判据已存在可复用为验证器，无需新件） | 中——指引补一块+验证器接线 |
| B2 静态平衡 | **F** | canary runtime+词表（§三）；mix_apply_static_balance_batch 工具在 | 工具✓+full-access 指引已覆盖方向/剂量选择权✓；**缺**：主次关系（foreground/anchors/support/effects）目标判据指引+settle 后验证断言 | 中 |
| B3 声像 | **F** | 同 B2（pan_layout_batch 工具在） | 同 B2（声像布局判据指引缺） | 中 |
| B4 低频关系 | **F** | 词表+守卫（§三）；B13b masking frontier 腿=观察面证据 | 观察工具✓（frequency_relationship/masking view+mix.derive）+EQ 闭环 harness 形态✓（ReAct 骨架 eq_plan 指引）；**缺**：低频遮蔽判据指引（kick/bass 关系语义） | 中偏高（复用面最多） |
| C1 频段清理 | **F** | 零栈腿全绿（§1.1）；runtime 613 行+planner | 观察工具全✓；**缺**：全工程频段清理计划语义指引（defer non-static 判据现硬编码在 decode）+cleanup 验证器 | 中 |
| C2 动态控制 | **F** | 33 单测锚定+de-esser Matrix 腿（本卡真栈批）；runtime 995 行 stage 机 | 观察工具✓+PCA 链工具✓+full-access 指引✓；**缺**：coverage 词汇验证器（TestC2CoverageVocabularyRejectsInventedAxes 的判据目前是测试锚不是 runtime 验证器）+剂量判据指引 | **低**（固定面最重、stage 机拆解风险最大；建议最后拆） |
| C3 空间深度 | **F** | 15 单测（canary dispatch/只读白名单/报告组装） | 观察工具✓（spectral_field/stereo_relation/ccb）；**缺**：空间深度判据指引+报告组装的 harness 等价物（现固定组装在 runtime） | 中 |
| C4 段落自动化 | **H** | **tool_chain_ok 13/15**（M3 收口）+组 3 5/5+225 判断权上移 | harness 形态已测存在✓；**缺**：关词表 pre-model 武装后模型自主进入确认链的入口识别（残差 no_route 2/15 是拆掉后主要风险面） | **高**（各族首拆候选） |
| D1 全局响度 | **F**（in-flight） | 254 实现+256 取证在飞（§1.3） | loudness_summary/watch 工具✓+RLM 投影底座✓+阈值族在 loudnesswatch config；**缺**：响度判据指引+settle 决策面 harness 等价物；**前置=256 收口**（断言分支容忍裁定后才有稳定验收基线） | 低（等 256） |
| A1/A4 | **F**（观察/片段面绿） | fs_largeproject 基线 A1/A4 pass；A4 工具族+指引已在 | A4=clip 工具全暴露+strip_silence suggest 指引面（工具 reason 内）；**缺**：片段清理判据指引 | 中 |
| A2/A3 | **F**（capability_blocked 软记录） | TIM/TOM 投影在库；139 返工线 | 依赖观察投影链健康（归 139/255 复验线） | 低 |
| A5 段落地图 | **F**（披露面） | 225 能力目录披露 | 标记能力经 choreo 链可提议（225 已证 LLM 可提议打标记）；**缺**：无独立验证器 | 中 |
| C5/D2/D3/D4/E1-E4/F1 | **not_built** | capability IDs=nil（queue view 源码） | 按 §二-2 薄固定层原则新建（判据+验证器+编排归 harness）；D3 天然衔接 RLM；E 前置=C5 | 不适用（新建非拆减） |

### 分级首盘结论

- **A 态节点数（当前）=0**——进度计量基线归零，与裁定 §二-3「进度以 A 态节点数计量」一致。
- **H 态=C4 一个**（tool_chain_ok 13/15），首拆候选：其 harness 形态已测、词表边界清晰（拆=关 automation_intent pre-model 武装）、验证判据现成（M3 收口三组话术）。
- **F 态=B1-B4/C1-C3/D1+A 族观察面**；B4 拆减复用面最多（EQ 闭环 harness 形态已就位），C2 最重风险最大（建议排序最后）。
- **固定面重量梯度**：C2(995 行 stage 机)>C4(choreo+词表)>B2/B3/B4(canary runtime)>D1(settle+watch)>C1(613)>C3(421)>B1(preflight pack)——与「越新越薄」的词法后继退化梯度（§三-2）同向，印证裁定「禁重型 stage 机」原则已在新族落地。

---

## 六、ablation 腿模板（设计，本卡不执行）

每族 ablation 腿统一四件套（各族后续卡按此模板填充）：

1. **拆法（开关面）**：该族固定层关闭点（按 §三 清单选）：
   - B2-B4/C1：`resolveCapabilityOwner` 该族词表短路（inferProjectAwareCapability 返回 ""）+canary dispatch 该分支禁用（环境变量或测试注入，**不得改生产词表本身**——用注入面）；
   - B1：`messageLoopGainStagingCapabilityRequest` 短路（preflight 不触发 pack）；
   - C2/C3：仅显式标记路由——owner 面已最薄，拆=canary dispatch 禁用；
   - C4：`automationTimeVaryingQualifiers`+`automationSemanticResultWords` 武装关闭（pre-model 接管旁路）；
   - D1：`d1LoudnessSettleIntent` 分支禁用（观察 pack 保留）。
2. **场景腿**：该族验收话术（B2「帮我建立主次关系」/C1「全工程频段清理」/C4 组 1-3 话术族…）在纯 harness（通用循环+full-access 指引）下真栈跑——复用既有 smoke 脚本栈生命周期，不改脚本，经配置注入关固定面。
3. **判据（能力复现≠固定面绿）**：链路完整（观察→候选→确认→执行→读回）+诚实边界分支保留（组 2 形态：无插件不虚写）+`tool_chain_ok` 式计量（N 轮 M 比例）；**红线**：零裸写、确认卡单次、不谎报。
4. **通过线与止损**：参照 C4 先例（13/15 稳定线）+§8 两轮同型失败止损取证；ablation 失败≠回退固定面，=该节点保持 F/H 态+缺口入拆减准备度矩阵。

---

## 七、处置建议汇总

| 项 | 建议 |
|---|---|
| G1+G2 | 开 chat 域测试隔离修复卡（P2；文件域=loudness watch verdict store 路径注入+free_state conclusion bridge 测试全局态） |
| G3 | 开脚本修复卡（P2，一行级；修后 C1 冒测退出码才可作验收门槛）——可与 G6-②（若裁定断言过时）同批 |
| G5 | 开 matrix 腿族修复卡（P2：py invoke 接 full-project-access context 或授权注入；一次修复覆盖 de-esser/limiter/transient/compressor 四腿） |
| G6 | **交主管裁定**（needs_confirmation 契约存废）→ 裁定后开行为取证卡或断言更新（见 §二） |
| G7 | 无需开卡：入跑腿清单前置（正斜杠+显式 -KernelExe），已记 §1.2 |
| C4 ablation | **首拆卡候选**（P1；按 §六模板，判据=M3 三组话术在关词表后 harness 复现） |
| B4 ablation | 次选（复用面最多；EQ 闭环 harness 形态已就位） |
| C2 拆减 | 排最后（stage 机最重）；其 coverage 词汇验证器独立成小卡先行 |
| D1 | 等 256 收口后再排 ablation（验收基线先行） |
| C5/D3 | 新建走薄固定层原则（判据+验证器；D3 复用 RLM）——与勘察 backlog 一致 |

---

## 八、计费与运行记录

- 零栈腿：go test 七包（C1 冒测）+agentloop/chat/harness 定向复跑——**零 LLM 调用**。
- 真栈批 1（de-esser Matrix）：**零 LLM**（9/9 gate 拒绝在装载第一步，无 chat 轮）。
- 真栈批 2（mix_single_tick）：deepseek-flash 257 行 telemetry（observe turn 链：首轮 186.9s 切片+continuation 排干），粗估 ≈¥0.2-0.4——批 2 预算 ¥1-3 上界内。
- 形式失败三轮（批 1 反斜杠×1+批 2 无内核×1，另批 1 首跑前 netstat 检查轮）：均栈未起零 LLM。
- 栈占用：22:39 登记（4a416ef0）→22:51 释放，TEARDOWN_ALL_ZERO 22:51:27；与 256 错峰满足（其 22:33:40 释放在档）。
- 本卡全程**各族脚本/生产代码零改动**（git diff 核对见回执）；全部工件随 port/af-family-health-1 入库：coord/runs/AF-FAMILY-HEALTH-1/{deesser_matrix_20261105_224002（9 case json）, mix_tick_20261105_224459（console+agent_last.log+telemetry）}。
