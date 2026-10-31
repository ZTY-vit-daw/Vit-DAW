# M3-OBS-IDENTITY-BIND-1 真栈工件与结果（RESULTS）

- 卡：coord/cards/doing/2026-10-31-M3-OBS-IDENTITY-BIND-1.md（池序 231）
- 执行：ZCode GLM-5.3 PC 执行会话（231 卡会话）；worktree D:/Vit_DAW_wt_231impl @ port/m3-obs-identity-bind-1
- 领取基线：origin/main=ab2bcde6（230 合入后主干），领取提交 eb0e4911；实现 commit 9f8dbe07（已 push）
- 栈登记：7d43dbc9（19:2x 置顶，fetch 重读+进程/端口全零亲核+登记先于启动+启动前即时复核 RECHECK_ZERO）；拆栈见文末
- 命令：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_231impl/scripts/dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -AutomationLLMRouteSmoke -ChatAuthStampSmoke -NoChatSmoke -NoStripSilenceSmoke `
  -KernelExe D:/Vit_DAW_wt_228/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe `
  -RunArtifactsDir D:/Vit_DAW_wt_231impl/coord/runs/M3-OBS-IDENTITY-BIND-1/identitybind_1
```

- run：identitybind_1；**ps1 exit 0（dev agent smoke completed）**；kernel=wt_228 VitApp.exe sha256 f9e3da0a 运行前后零漂移（只读复用，本卡 C++ 零改动）。

## 一、链路进度（§7 分列一）

### 诊断定案+单一真相源裁定（目标 1，先诊断后修）

- 四 id 铸造点图谱成文：`evidence/identity_mint_map.md`——全部**内核铸造**（agent 侧 grep 零铸造点）：boot 装载保留 default 文件 id（本轮实测=ce9e）；`new_project`/`save_as` 各自 forceNew 铸新 id（fork 语义，内核设计如此）；agent 三消费面（store 绑定/bake 盖章/观察解析）各自滞后、**无对账层**=230 levels null 根因；version_project_* 传播通知由 Godot 前端发（内核源零命中），raw ZMQ 生命周期零通知零跟随。
- 单一真相源裁定：**权威=内核当前 loaded project 的 edit state UUID**（`get_project_state` 顶层 project_uuid+parent_project_uuid 亲核）；跟随机制=fork 跟随（内核 parent==当前绑定 → 复用既有 ForkDerived+ForkAgentStore 传播件再绑定事实）vs switch 跟随（无关身份只激活不搬运）。

### 修复实现（目标 3，域内：acousticpackage+harness/projectworkspace+测试）

- `reconcileProjectIdentityForObservation`（新文件 harness/project_identity_reconcile.go）挂 `requestMixObservation` 顶部（覆盖 mix_observe+ccb.observation_request 全观察面）：generic 态零内核交互（保持 read-first 零命令契约，既有测试回归绿）；concrete 分歧先刷新 shadow 再按内核 parent 判 fork/switch；同 id 异路径再激活；未保存身份不物化 draft（FIX-START-DRAFT-SIDE-EFFECT-1）；全程 fail-open。
- `ForkDerived` 补树级 `RebindProjectTree`（acoustic store/观察快照随行再绑定到 fork 新 id）；manifest 头部重写 fail-open（§11 旧形态兼容，异形 manifest 不再阻塞 fork）。
- **安全面零触碰**：MergeStatus/rowIdentityMismatchReason 的 project_mismatch 防御判定、ingest 事件门、FS 准入门/治理门/预算语义均未改。
- 单测 8 个全绿：reconcile 五形态（fork 往返=l1 事实非 stale+levels 数值在 ref / switch 不搬运 / stale-shadow 自愈 / unsaved 跳过 / aligned 零内核命令）+ ForkDerived 三契约（全树 rebind / legacy 容忍 / 无源 noop）。
- §6 健康检查全绿：五条 Go 测试+webui 471 测试+AB 冒测 exit 0（worktree 内执行）。

### 真栈兑现（目标 2+3）

- **identity 收敛亲证**（evidence/identity_convergence_and_gate_anchor.json）：`.vit_derived`/`.vit_agent` 各**仅一个 id**（vitproj_19e078b8，=save_as 落盘工程 id）；L3 摄取同 id 盖章；acoustic store live 包 **status=ready、l1 peak_rms=ready、rms_dbfs=-11.889**（230 同事实曾 stale/project_mismatch）；boot 态 orphan 行（ce9e）如实标 stale 留档——防御语义保留，live-id Find 命中 ready 包。
- **模型面 levels 非 null 亲证**：r3 自由态循环内 `track.basic_energy.facts.levels` = **rms_dbfs=-11.889 / peak_dbfs=-4.188 / status=ready**（rounds/g1_r3.json）——230 全 null 的同一读数在模型观察面恢复。（setup 摘要 excerpt 的 null=attempt-1 bake 在途采样时点，与 230 同形态，终态以 store+r3 为准。）
- **真实旅程判定（目标 2）**：
  - agent 重启旅程（226 形态）：chat_auth_stamp 双相全绿——raw new_project+save_as 工程形态+**真 agent 重启**+durable restore full_project_access+全跨度直执行（stop_reason=automation_choreo_settled，written=2）→ **重启旅程不暴露**；
  - save_as 旅程：自动化 setup 即 raw-ZMQ new_project+save_as 形态，save_as 后观察 1 次尝试返回事实、identity 单点收敛 → **不暴露**（GUI 通知面 save_as 传播件 diff 零触碰+代码核验在位）；
  - **诚实披露**：本轮收敛由 chat 激活面时序有利完成（agent log 时间线：19:12:40 绑 ce9e draft→19:12:41 激活 19e078b8），reconcile 对账处于对齐态零日志行——**对账机制点火由单测覆盖**（TestObservationIdentityReconcileForkFollowsSaveAs 完整往返），真栈腿验证的是终态收敛与旅程不暴露；激活时序不利时的保证层=reconcile（设计意图，非本轮实测点火）。

## 二、验收门（229/230 原口径，第三承）——未达，停止条件 3 承接上交

- **组 1 no_route 消失：未达**（verdicts=[no_route,no_route,no_route]，三连止损 llm_samples=3；组 2/3 按 220 止损设计 skipped）。
- **set_points 确认链端到端首绿：未达**（三轮 kernel volume points 恒 0，异步续跑窗口内未落写）。
- **失败形态锚点（与 230 不同型）**：每轮 exchange-1 自由态循环完整运行（观察执行在场，r3 含 track.basic_energy）→ **stop_reason=limit_reached**（预算尽）→ 任务转异步续跑；exchange-2 复问收到 **stop_reason=automatic_continuation_already_scheduled**，回复「任务已在自动续跑队列中；这条消息不会创建新任务或重复执行。」→ 判 no_route（排队文本形态，230 遗留观察预判「残留另议」的形态未消失）。
- **停止条件 3 成立上交**：证据齐（identity 对账=单 id 收敛在卷+观察事实在场=模型循环内 levels ready 亲证）+ 组 1 三连 no_route → **模型能力/预算-异步续跑边界面上交主管，不再逐层折卡**。决策面候选：排队文本的判定归类（采样协议 vs 生产语义）、limit_reached 预算与同步采样窗口的匹配、异步续跑完成度可观测性——非本卡文件域。

## 三、§8 计费与失败分类

- LLM 真实计费：**30 调用 deepseek-flash，418,295 input / 44,682 output tokens**（telemetry 逐调用 logs/agent_llm_telemetry.jsonl；auth-stamp 腿为确定性执行无计费 LLM）。
- 失败分类：no_route×3（全部=异步续跑排队文本形态）；零 env_interrupt/confirm_broken/write_fail/lexicon_hit/wrong_tool/capability_blocked。
- 运行次数：1 腿一次到位；止损后按设计停止；无同版本同断点重跑。

## 四、工件清单

- automation_llm_route_summary.json / console.txt；rounds/g1_r1-r3.json（逐轮 exchanges+verdicts+kernel 证据）
- chat_auth_stamp_summary.json / _after.json / console×2（重启旅程双相）；agent_phase_before.log（自动化相 agent 全日志）
- evidence/：identity_mint_map.md（诊断图谱+设计裁定）、identity_convergence_and_gate_anchor.json（本轮收敛+验收门锚点）、acoustic_package_status_final.json（终态 store 全文）
- logs/：agent_llm_telemetry.jsonl、agent_prompt_render.jsonl、agent_authstamp_phase_after_full.log
- automation_llm_route_clip.wav（确定性 fixture，230 同款）；派生态剔除按 218/222/229 先例（.vit_agent/.vit_history/.vit_derived/default_project.xml/工程 .vit 摘录后剔除）

## 五、拆栈

- owner 手拆 VitAgent 13888（重启相）/VitApp 27748 → 复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787/5178 listen=0、UDP 4444/4445=0（TEARDOWN_ALL_ZERO）。kernel f9e3da0a 运行前后零漂移。释放记录随卡面回执提交。
