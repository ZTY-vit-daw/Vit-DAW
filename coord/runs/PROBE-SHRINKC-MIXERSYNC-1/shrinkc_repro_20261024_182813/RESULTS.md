# PROBE-SHRINKC-MIXERSYNC-1 回执：shrink_c mixersync 腿预存不稳定——根因定位与修复

- run=shrinkc_repro_20261024_182813（本目录）；执行 owner=GLM-5.3-flash PC 执行会话（ZCode，179 卡会话）
- 基线：主仓 origin/main=c1285d68（领取后）→ 本卡工件分支 port/probe-shrinkc-mixersync-1；前端仓 origin/main=ee8fa0e → 修复分支同名 c55b3cd
- 预声明：PREDECLARATION.md（跑前锁定，机理假设、轮次预算、失败分类、止损线均在卷）

## 一、失败类型定位（卡面范围①）

**166 工件侧核实**：sendux_gui_20261022_1906 只存终轮全过日志（reg_probe_fe_gui_shrink_phase_c.log=342 checks failed=0 PROBE PASS），三轮失败轮原始输出未入工件；失败类型定位以本卡复跑取证为准（领取记录已声明）。

**根因（真栈复现第 1 轮即坐实，legB_round1_phase_c.log 在卷）**：

- **分类：时序竞态（探针夹具 × 生产事件链），非确定性缺陷、非环境残留。**
- 机理：探针 mixersync 腿以 `_apply_kernel_project_reply_and_emit(注入 reply)` → `await _wait_frames(3)` → 读 `mixer_state()` 断言。注入链本身**全同步**（repository apply → project_loaded → `_rebuild_strips` → strip/fader 就绪无 await/deferred），但 3 帧等待窗把**活栈下内核电平遥测驱动的结构 resync**（MIXER-LIVE-REFRESH-1 正确生产行为：真电平流 track 集合 ≠ 基线 → `_maybe_detect_track_structure_change` → 500ms 节流调度 → `vit_refresh_project_from_kernel` VSP/IPC 双传输 → `_apply_kernel_project_reply_and_emit(内核真状态)` → 重建）让进了断言窗口。
- 失败 detail 铁证：`track_ids=["1007","1012","1017","1022","1027","1032","1037"]`（内核真轨集 bass/drums/guitar/other/piano/vocals/Reverb Bus，注入 trk_lr_a/b/c 被覆盖）+ `resync_reason="telemetry_track_set_changed"`。
- 复现条件=smoke 活栈窗口内跑探针（166 形态）：无栈时无回写源 → 确定过（Leg A 三轮全绿实证）；活栈时真遥测/回写与断言竞速 → 间歇 1-2 腿挂（本轮挂 add+fader 两腿，与 166 「add/delete/fader 时序腿」签名一致）；500ms 节流窗+回写落点随机性解释「连续三轮各挂 1-2 腿、终轮全过」。

## 二、隔离复跑取证（卡面范围②）

| 腿 | 条件 | 轮次 | 结果 |
|---|---|---|---|
| Leg A（修复前无栈基线） | 无栈，探针独跑 | 3 | 3/3 exit 0 PROBE PASS 342/342（legA_round1-3）|
| Leg B（修复前活栈复现） | kernel 12740+vsphub 25304+agent 36548（bringup_state.json），探针独跑 | 1（预算内首轮即中，止损不再盲跑） | exit 1 PROBE FAIL，挂 mixersync add/fader 两腿（legB_round1）|

- 栈隔离形态：kernel=主检出 K3 722A79D7 亲核只读，VIT_PROJECT_XML 指 run 目录副本+独立 kernel_workspace（JOURNEY 隔离），主检出 Workspace 零触碰；agent=主检出 Go 树新构建 VitAgent_shrinkc.exe（SHA256 前 16=6C41DAAB8BF00B20，4c7736c9 与 c1285d68 Go 零差异）；§2.2 占用登记 06caae56 推 main 后才起栈，拆栈后复核进程=0、监听=0（仅余 TIME_WAIT 客户端套接字）。
- LLM=0（预声明记账兑现：探针+确定性栈，无 chat 腿）。

## 三、修复（卡面范围③——夹具隔离+settle 判据，期望值零改动）

前端仓 `tools/probe_fe_gui_shrink_phase_c.gd` @ port/probe-shrinkc-mixersync-1@c55b3cd（diff 30+/12-，仅此一文件）：

1. **注入链断言读窗零帧化**：4 组 inject（2 轨/add/rename/delete）的 `await _wait_frames(3)` 全部移除，apply→`mixer_state()` 读回零帧同步——断言只读本腿注入结果，同步契约不让渡窗口；**期望值/断言文案/checks 总数零改动**（342 保持）。
2. **遥测调度腿置前+归因收紧**：移到注入链之前（新装配 adapter 无 resync 历史、节流窗空闲）；判据从「pending 或 reason 含 telemetry」（活栈下 reason 可被内核 resync 污染=虚过）收紧为 **pending 迁移 pre=false→post=true**（两次同步读之间无 _process 可插队，归因只能落在本注入）；被节流时按 ≥620ms 墙钟间隔重试（≤4 次，生产行为=被节流不更新基线后续重试，故必收敛；OS.delay_msec 阻塞等待不耗 FRAME_BUDGET）。
3. 函数头注释补断言纪律说明（约束面向后续维护者）。

**不是什么**：未改 adapter/strip 生产代码（勘察确认三条墙钟依赖链为正确生产行为：结构 resync 500ms 节流/本地推子卫窗/外部拓扑冷却 1500ms——失败根因在探针读窗，不在产品）；未改任何期望值或断言阈值；未动 battery 驱动 ps1。

## 四、修后验证（卡面验收标准）

| 验证 | 条件 | 轮次 | 结果 |
|---|---|---|---|
| 修复后活栈轮 | 同 Leg B 活栈形态 | **4 连续** | 4/4 exit 0 PROBE PASS 342/342（legBfix_round1-4）——**≥3 轮全绿达标** |
| 修复后无栈轮 | 拆栈后 | 3 | 3/3 exit 0 PROBE PASS 342/342（legAfix_round1-3）|
| 整包回归 | 活栈窗口，run_fe_gui_shrink_phase_c_probe.ps1 十探针套件 | 1 | **PSEXIT=0 全过**：shrink_c 342/342+shrink_b 119/119+shrink_a 87/87+popup 111+vitnote 六套（battery_full/ 在卷）——既有探针零回退 |

- 红线门说明：battery 默认 BaseCommit=924318de（92 卡时代基线），与现行 main 的历史差集含后继卡合入的 telemetry_manager.gd（红线第 1 文件）——首跑被拦（如实留档 battery_full 首跑即 throw 输出未存，重跑用 `-BaseCommit ee8fa0e`（本 worktree 基线）——门语义=worktree 自身 diff 不得触红线九文件，本卡 diff 仅 tools/probe_fe_gui_shrink_phase_c.gd，门过。此为参数选择非门放宽，门逻辑零改动。

## 五、端测边界声明

- 本卡全部断言面=headless 探针 DOM/装配层/状态字典（mixer_state/卡片/状态栏/锚钉），**不覆盖渲染像素与真实系统窗口外观**（92 卡既有边界不变）。
- 未覆盖：mixersync 腿之外的其他探针腿在活栈下的同类竞态排查（本轮整包十探针活栈全绿为经验证据，但未逐腿做同款竞态审计——如后续他腿在活栈复跑再现间歇挂，按 §11 另行处理）。
- 166 三轮失败轮原始输出已不可考（工件未存），本卡以独立复现补证；若主管认定需 166 原始失败输出本身，则该证据不存在，只能以本卡复现件替代（如实上交）。

## 六、工件清单（本目录）

- PREDECLARATION.md（预声明）/ bringup_shrinkc.ps1 + bringup_state.json（栈形态与 pids）/ godot_import.log
- legA_round{1,2,3}_phase_c.log（修复前无栈基线）
- legB_round1_phase_c.log（**修复前活栈复现轮——根因铁证**）
- legBfix_round{1,2,3,4}_phase_c.log（修复后活栈 4 连绿）
- legAfix_round{1,2,3}_phase_c.log（修复后无栈）
- battery_full/（整包十探针日志）
- agent_last.log / vsphub_last.log / kernel_workspace/（栈侧留证；kernel_workspace 为内核运行时工作区副本）
- 重件留盘（不入库，.gitignore 声明）：VitAgent_shrinkc.exe（36M 构建产物，SHA256 前 16=6C41DAAB8BF00B20）、BridgeReplies/（33M bridge 回执缓存）。
