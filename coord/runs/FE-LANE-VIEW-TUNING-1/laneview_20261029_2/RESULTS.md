# FE-LANE-VIEW-TUNING-1（214）真栈 run 工件：laneview_20261029_2（收口轮复跑·工件补录）

- owner=ZCode GLM-5.3 PC 执行会话（ZCode，214 收口轮会话）；栈登记 91ce808e（18:52 推远端 a759c63c 合并后）/释放（本文件同批提交）
- kernel=主检出 D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe 只读复用（sha256 c1e32dfe… 亲核=上腿同源零漂移，gate 风险位「主检出二进制漂移联动检查」核过）pid 8436；agent=D:/Vit_DAW_wt_214stack 构建 VitAgent.exe（2026-10-29 10:02=上腿同款二进制）pid 24008；vsphub=主仓 agent/bin/VspHub.exe 只读手启（启前亲核 8787 空）pid 10208——三 pid 命令行逐一亲核=本腿所属（bringup_pids.txt）
- VIT_PROJECT_XML=run 目录 git 干净版隔离副本 project_iso.xml（blob 287956e7=202-212 腿同基线亲核；运行后 444b3fb7=内核写入被副本承接=隔离生效；主检出 default_project.xml mtime 10-29 12:12=用户手测遗留值零触碰；214stack worktree 运行后 git 干净）
- 前端被测 worktree=D:/Godot/project/vit-daw-frontend-lanetune1@port/fe-lane-view-tuning-1@**34ca579**（=用户 2026-10-29 12:11 手测观感通过同版，零代码改动纯复跑）

## 复跑背景与「丢失面」判定修正（如实记录）

1. 收口轮开场时的表面事实：主仓 coord/runs/FE-LANE-VIEW-TUNING-1/ 目录不存在+前端仓无 port/fe-lane-view-tuning-1-stack 分支 → 判「上腿工件丢失」，按指令登记（91ce808e）并复跑正式轮。
2. **拆栈后发现判定需修正**：上腿工件实际完整提交于 **D:/Vit_DAW_wt_214stack worktree 的本地分支 port/fe-lane-view-tuning-1-stack@98420a99（2026-10-29 10:23）**——含正式轮×2/env112/基线 DIAG/迭代 r1-r4 完整 stdout+RESULTS.md 六节。真态=**未推送远端、未同步主检出**（验收不可达），非数据丢失；登记条目中「原始 stdout 未捕获=丢失」一句以本节为准修正。
3. 上腿工件的两处留证缺口（摘要代替原始件）：真栈族 3 件仅 stack_family_replays.txt 结果行（5 行）；离线全家仅 offline_family_rerun.log 计数+红件清单（8 行）。**本腿复跑以完整原始 stdout 补齐这两块**（stack_autolane_*.log 三件全量+offline/ 28 件逐件日志）。
4. 并行环境留痕：本腿窗口与 213 会话离线探针（probe_telem_ws_recover_consumer.gd headless 对形态）共存，栈端口全零无冲突；拆栈后残留 Godot 进程=213 会话所属，§2.2 不擅自终止原样保留。

## 复跑验证矩阵（全部完整 stdout 落盘本目录）

| 面 | 结果 | 工件 |
|---|---|---|
| **正式轮 r1（窗口模式全腿）** | **31/31 PASS exit 0** | formal_r1_probe.log |
| **正式轮 r2（连续第二轮）** | **31/31 PASS exit 0** | formal_r2_probe.log |
| env 覆盖轮（VIT_GUI_LANE_STRIP_HEIGHT=112） | 31/31 exit 0（P0i rect=(315,332,2245×112) effective=112.0 source=env 实锚，min 48/max 256 在卷） | env112_r1_probe.log |
| **真栈族既有 3 件（完整 stdout）** | edit 30/30+rebuild 23/23+layout_grow 16/16 全 SUMMARY_JSON pass exit 0（128/24 新期望下零回退） | stack_autolane_edit/rebuild/layout_grow.log |
| **离线全家 28 件（逐件日志）** | 27 -s 件+file_reply 场景件 7/7 全 exit 0（211 集合 27+新增 probe_lane_view_tuning 26 断言） | offline_family.log+offline/*.log |

- 复跑轮与上腿（_1）正式轮数据完全一致：31/31×2+env112 31/31+真栈族 30/23/16——**跨会话双轮独立复现**。
- 上腿独有件（本腿不重复）：基线 DIAG 轮（96 取证，diag_baseline_probe.log）+探针迭代 r1-r4——取证面已定案，无复跑必要。

## 两轮工件互补关系（主管验收索引）

| 面 | laneview_20261029_1（上腿 10:00-10:22） | laneview_20261029_2（本腿 18:52-19:0x） |
|---|---|---|
| 正式轮×2 / env112 / 诉求腿 | 完整 stdout | 完整 stdout（独立复现） |
| 基线 DIAG+迭代 r1-r4 | 完整 stdout（独有） | —（取证已定案不重复） |
| 真栈族 3 件 | 结果行留证 | **完整 stdout**（补齐） |
| 离线全家 | 计数+红件清单摘要 | **28 件逐件完整日志**（补齐） |
| 入库通道 | 98420a99@214stack 本地分支→本轮同步主检出 | 本轮直接入主检出 |

## 验收数据摘要（与卡回执一致）

- 前端 commit=34ca579（port/fe-lane-view-tuning-1，8 文件 +1066/−22）；用户手测观感通过（2026-10-29 12:11，[等待用户] 面已闭环）
- 领取基线（卡 §领取 在卷）：前端 main 37dc4b6da735f1e32fef3d091c8553195f5596c7，status 零 tracked 改动，diff 空；主仓 origin/main=bbdb9b8d（claim b700146e）
- B 定案三候选：①渲染高==映射高 96=96 排除；②＋条 themed min 24>旧常量 22→stack 恒溢出 2px 常驻滚动条+条底裁剪（DIAG viewport 118 vs stack 120）→常量校准 24；③value_scale fader 0..1 而 0dB@0.7408→可用值域带窄=主症状源，A 128 主修
- C 拖柄：车头区右端 Btn_Envelope_Lane_Height（BaseButton=网关 interactive 让路零域外改动）；竖直拖调容器会话高多条统一，202 即态不持久化；budget 联动 wanted=24+N×实际条高
- A env：VIT_GUI_LANE_STRIP_HEIGHT 产品 env 覆盖，clamp[48,256] 不落持久化（env112 轮 source=env 实锚）
- lane 族探针期望值随高更新=授权面（198 先例披露）：probe_autolane_layout_grow/probe_autolane_rebuild/probe_envelope_lane_ui/stack_probe_autolane_layout_grow 四件+全族 _ADD_BAR 22→24 校准

## 端测覆盖边界声明

- 窗口模式探针=真实 GUI 派发管线+真实渲染像素采样（与用户手测同构），非用户键鼠；栈为探针最小腿（内核+agent+hub 手动 bringup，无 UI 全旅程）。
- 上腿四项上交知悉（B 候选③ dB 域观感小卡候选/＋条常量主题漂移由 P0k 拦/外部写自动重拉独立行为面/N=3 全展开语义随 128 失效）见 laneview_20261029_1/RESULTS.md §上交主管知悉——本腿无新增上交。
