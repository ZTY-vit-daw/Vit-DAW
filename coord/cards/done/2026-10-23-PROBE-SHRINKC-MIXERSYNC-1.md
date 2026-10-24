# PROBE-SHRINKC-MIXERSYNC-1：shrink_c mixersync 腿预存不稳定修复

- 发卡：GLM 主管决策流 / 2026-10-23 早窗（依据=2026-10-22 gate 风险③；§11 重复出现规则触发）
- 派发确认：已确认（gate 2026-10-22 §3 风险③裁定「应开修复卡」）
- 验收负责人：GLM 主管决策流
- 池序 179；目标仓库=以勘察固定（**主管勘误 2026-10-24**：shrink 探针实位于前端仓 D:\Godot\project\vit-daw-frontend\tools\probe_fe_gui_shrink_phase_c.gd，主仓无此文件；mixersync 断言腿亦可能在主仓 scripts 驱动侧——领取时勘察定位并在领取记录回填文件域）；P3（测试面卫生）/ 0.25 天 / 依赖=无；模型分级：L1 / PC 执行会话（flash 可）
- 与 176/178（前端执行流）文件域不相交可并行，但**真栈腿与在飞卡错峰**（§2.2 登记）。

## 现象

166 卡回执如实上交：shrink_c 场景 mixersync 腿连续三轮各挂 1-2 腿、终轮全过，基线与本支同现=**预存不稳定非本卡引入**。原始失败输出在 166 工件（coord/runs/FE-SEND-UX-UNIFY-1/sendux_gui_20261022_1906/RESULTS.md 邻域）。

## 范围

①读 166 工件失败腿原始输出，失败类型定位（时序竞态/等待窗口不足/环境残留/确定性缺陷）；②隔离复跑取证（≥3 轮）；③按结论修复（等待窗/settle 判据/夹具隔离——修复探针与断言面，不得为凑过改期望值）；④若确属环境类不可修，按 §11 给隔离证据+known flaky 登记（禁止无条件永久豁免）。

## 文件域

以勘察固定（预期 shrink_c 场景探针 mixersync 腿邻域）。越域即停。

## 验收标准

修复后连续 ≥3 轮该腿全绿（或 known flaky 登记含隔离复跑+整包复跑+diff 排除证据）；既有探针零回退；工件入 coord/runs/PROBE-SHRINKC-MIXERSYNC-1/。

## 停止条件

失败根因位于卡片域外（产品 mixersync 逻辑本身）→ 锚点+复现上交另裁。

- 领取：2026-10-24 早窗（ZCode 本会话机器钟）/ origin hash=主仓 **c1285d68**（卡面基线 4c7736c9 之后仅 coord/ 卡池状态前移：183 领取提交）、前端仓 **ee8fa0e** / owner=GLM-5.3-flash PC 执行会话（ZCode，本卡会话）/ 分支=**port/probe-shrinkc-mixersync-1**（前端仓与主仓工件分支同名）/ worktree=前端 D:/Godot/project/vit-daw-frontend-shrinkc + 主仓工件 D:/Vit_DAW_wt_shrinkc / 领取提交=本次 coord/shrinkc-claim 推 main 提交
- **领取勘察回填（文件域固定）**：①探针=前端仓 `tools/probe_fe_gui_shrink_phase_c.gd`（主管勘误证实；mixersync 阶段=`_mixer_live_refresh_pins()` 15 断言，probe:766-843）；②battery 驱动=主仓 `scripts/run_fe_gui_shrink_phase_c_probe.ps1`（10 探针套件，166 失败窗口形态=smoke 活栈窗口内运行）；③断言面产品侧邻域（若根因落 adapter/strip 时序）=前端仓 `vit_dock/views/mixer_dock_adapter.gd`、`vit_dock/views/mixer/vit_mixer_channel_strip.gd`、`vit_mixer_fader.gd`——勘察发现 mixersync 腿途经三条墙钟依赖链（结构 resync 500ms 节流 adapter:622-651 / 本地推子卫窗 2500ms+30s strip:588-917 / 外部 send 拓扑冷却 1500ms adapter:551），与「add/delete/fader 时序腿」签名相符，修复域按复跑取证结果在此三者+探针内收敛；④工件=主仓 `coord/runs/PROBE-SHRINKC-MIXERSYNC-1/`（随本卡 port 分支入库，不落主检出盘）。166 工件侧核实：sendux_gui_20261022_1906 仅存终轮全过日志（342 checks failed=0），三轮失败轮原始输出未入工件，失败类型定位以本卡隔离复跑取证为准。
- 与 183（FE-SEND-LEVEL-FADER-1，doing@c1285d68）错峰：本卡探针复跑轮=活栈（kernel+agent+bridge，无第二 Godot 占 4444）形态，§2.2 登记先后，183 登记占用则等待其空窗（等待不算失败）。文件域与 183（lvlfader 推子 UI+stack_probe_sendroute_macro.gd）不相交。
- 回执：**失败类型=时序竞态（探针夹具×生产事件链），非确定性缺陷非环境残留**——smoke 活栈窗口下内核电平遥测驱动的结构 resync（MIXER-LIVE-REFRESH-1 正确生产行为：真电平流轨集≠基线→500ms 节流调度→VSP/IPC 回拉内核真状态→重建）在探针注入链 apply→check 的 `await _wait_frames(3)` 窗口用内核真状态覆盖注入态；166 三轮失败轮原始输出未入工件（仅存终轮全过日志，领取记录已声明），本卡以独立真栈复现补证。**commit=前端修复 port/probe-shrinkc-mixersync-1@c55b3cd**（tools/probe_fe_gui_shrink_phase_c.gd 30+/12-：注入链 apply→读回零帧同步+遥测调度腿置前+pending 迁移 pre/post 归因+节流重试；期望值/断言文案/checks=342 零改动；adapter/strip 生产代码零触碰）；主仓工件 port/probe-shrinkc-mixersync-1@f417a39a（run=shrinkc_repro_20261024_182813：PREDECLARATION 预声明/bringup_state/RESULTS.md 回执/十一份探针日志）。**复跑轮次与结果**：修复前无栈基线 3/3 过（确定过，证明无栈无回写源）；修复前活栈 Leg B 第 1 轮即复现 166 签名（add+fader 两腿挂，detail=内核 7 真轨集 bass/drums/guitar/other/piano/vocals/Reverb Bus 覆盖注入态+resync_reason=telemetry_track_set_changed=根因铁证，预算内首轮即中不盲跑）；**修复后活栈同条件 4/4 全绿 342/342（≥3 达标）**+拆栈后无栈 3/3+整包十探针活栈 PSEXIT=0（shrink a 87/b 119/c 342+popup 111+vitnote 六套）=既有探针零回退。栈=kernel 12740（K3 722A79D7 亲核只读，VIT_PROJECT_XML 隔离）+vsphub 25304+agent 36548（Go 树构建 6C41DAAB8BF00B20），§2.2 占用 06caae56–释放随本提交，拆栈复核进程/监听=0；与 183 全程无撞栈。LLM=0。附注：battery 红线门默认 BaseCommit=924318de（92 卡基线）与现行 main 差集含后继卡合入的 telemetry_manager.gd，重跑传 `-BaseCommit ee8fa0e`（本 worktree 基线）使门按语义只校验本卡 diff——门逻辑零改动（RESULTS.md §四在卷）。**端测边界声明**：断言面=headless 探针 DOM/装配层/状态字典，不覆盖渲染像素与真实系统窗口外观（92 卡既有边界）；mixersync 腿之外的探针腿未做活栈同类竞态逐腿审计（整包活栈全绿为经验证据，再现按 §11 另行处理）；166 原始失败输出不可考，以本卡复现件替代。
- 验收：**整卡通过**（GLM 主管决策流 2026-10-24 晚窗）。①diff 亲读全过：期望值/断言文案/checks=342 零改动（遥测腿判据从 OR 宽判据收紧为 pending pre→post 同步迁移=收紧非弱化）、四处 await 删除=注入链钉成零帧同步契约、adapter/strip 生产码零触碰；②工件亲核（盘上日志逐判定行）：legA 3/3、legB 复现轮铁证（内核真轨集 1007..1037 覆盖注入态+resync_reason=telemetry_track_set_changed 与回执逐字一致）、legBfix 4/4、legAfix 3/3、battery 十探针全 PASS——验收标准「连续 ≥3 轮全绿」达标（活栈 4 连绿）；③主管复跑（合并态 5da6360，§2.2=dda3ea5f）：活栈窗口 shrink_c ×2 连过+shrink_a/b+fader/popup/realinput/start_feed 全绿；④执行侧分支漏提交 leg/battery 日志（仅盘上）由主管验收时补入主仓 coord/runs/PROBE-SHRINKC-MIXERSYNC-1/（工件契约缺口，流程警示记 gate）。前端合入 **3e9d00d** 已推；known-flaky 不需要（根治性修复）。
