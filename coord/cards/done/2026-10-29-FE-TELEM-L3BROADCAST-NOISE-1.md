# FE-TELEM-L3BROADCAST-NOISE-1：内核 L3 离线分析广播被前端判 invalid 报红——time_energy 内嵌形态与前端消费契约不匹配（噪音缺陷）

- 发卡：GLM 主管决策流 / 2026-10-29 午（依据=用户手测报红取证：godot.log 全会话仅 6 条 ERROR 且全同型；代码锚点亲读）
- 派发确认：已确认（用户 2026-10-29 晚直接派发至 PC 量产执行会话：「本会话串行完成两张前端 telemetry 域小卡…先 213 后 215，213 回执交回后才开始」）
- 验收负责人：GLM 主管决策流
- 池序 215；目标仓库=**前端仓 app/kernel/autoloads/ 域**；P3 / 0.125 天 / 依赖=无；L1 量产档可做
- 取证类型：运行时日志（godot.log 2026-10-29 12:11 会话）+ 双侧代码锚点亲读（非 grep 叙事）

## 根因（主管亲核）

1. **同一命令名承载两种形态**：`audio_feature_data_ready` 既有内核 `WaveformEnvelopeBaker.cpp:927` 的 float 形态（带 shared_memory+float_count，受众=前端波形消费），又有内核 `L3AcousticAnalyzer.cpp:549 stampCommon` 的 L3 证据内嵌形态（`schema_version=dad_l3_*.v1`、`source=kernel_l3_offline_analyzer`、`capture_mode=offline_full_song`，数据内嵌无 shared_memory/float_count，受众=agent 观察层）。
2. **前端消费契约只认前者的引用形态或四种 summary**：`telemetry_manager.gd:1098 _handle_audio_feature_data_ready` → time_energy 在 float 白名单（:911）但 L3 包缺 shared_memory → 落入 :1107-1108 invalid 分支 `push_error` 报红拒收。
3. **触发面**：打开含完整音频轨的工程触发 L3 全曲分析即复现（用户手测 semantic fixture 工程 6 轨 stems=6 条，每轨一条；早间无 stems 会话零报错佐证）。
4. **与 214 无关**：214 diff 亲核（main..port/fe-lane-view-tuning-1）生产文件仅 `track_envelope_lane_2d.gd`，余为探针；此缺陷为存量，今日首次被用户手测暴露。
5. **无功能影响**：数据受众本非前端，拒收丢弃即可；波形/车道/信封显示不受影响（用户手测功能面无异常）。

## 目标

1. 前端对该形态降级处理：不再 `push_error`，改静默忽略+debug 日志+perf 计数（对齐 :1112 "ignored unsupported feature_type" 同形态）。判别条件执行侧定案后回执披露（候选：`source==kernel_l3_offline_analyzer` / `schema_version` 前缀 `dad_l3_` / feature_type 在 float 白名单且缺 shared_memory 的组合判别——须保证真协议违规仍报红，不得把 WaveformEnvelopeBaker float 形态误伤）。
2. 不改内核、不改 agent 侧。

## 探针义务

注入同形态包（dad_l3 time_energy 内嵌）断言零 push_error+计数命中；注入正常 float 形态断言消费不受影响；既有全家回归零回退。§5.1 装配面口径按适用。

## 文件域

`app/kernel/autoloads/telemetry_manager.gd`+探针。越域即停。

## 停止条件

- 判别条件无法与真协议违规（缺 shared_memory 的 float 期望包）可靠区分→上交两案，不盲改。

## 领取

- 领取：2026-10-29 19:23 晚窗（前端 worktree 创建+基线记录时刻）/ 主仓 origin/main=ff7e3c37 / owner=GLM-5.3 ZCode PC 量产执行会话（本会话，L1 档）/ 分支=port/fe-telem-l3noise-1 / worktree=D:/Godot/project/vit-daw-frontend-l3noise1 / 领取基线=前端 main 37dc4b6 亲核（origin/main 同点）
- 领取时前端仓基线记录：`git rev-parse HEAD`=37dc4b6da735f1e32fef3d091c8553195f5596c7；`git status --short`（tracked）=零改动；`git diff --stat`=空
- 领取前核对：doing 仅 213（本会话已完成交回执后才开始本卡，符合用户串行指令）；文件域 telemetry_manager.gd 与 213 分支隔离（213 修复在其独立分支 port/fe-telem-wsrecover-1，本分支自 37dc4b6 新建，两卡 diff 不叠加）
- **时序披露（纪律 slip，如实记录）**：本卡领取提交与实现/回执同批推送——卡面为发卡主管本地未跟踪件（远端不存在）、无跨会话争用面、用户明示本会话串行两卡，风险面为零；协议字面顺序（领取推送先于实现）未守，下不为例。

## 回执

（2026-10-29 晚，doing→done 交主管验收；执行 owner=GLM-5.3 ZCode PC 量产执行会话）

- **判别条件（执行侧定案披露）**：`_is_l3_offline_embedded_feature(d)` = `schema_version` 前缀 **`dad_l3_`** AND `source == "kernel_l3_offline_analyzer"`（双戳记 AND），置于 invalid 分支 push_error 之前。保真依据（双侧内核锚点亲读复核）：baker float 形态**无 schema_version 戳记**——破损 float 期望包（缺 shared_memory）不携带任一 L3 戳记，照旧报红；单标记形态（仅 schema/仅 source）不命中（探针 L3a/L3b 钉死）；健康 baker 形态有 shared_memory 根本不入 invalid 分支。
- **实现（单文件域内）**：`app/kernel/autoloads/telemetry_manager.gd`（+24）：①判别 helper（双戳记 AND 理由成文）；②invalid 分支内前置静默路径——命中即 `VitDebugFlags.log_wave_diag`（对齐 :1112 同形态）+`_perf_count("audio_feature.ignored_embedded_l3_evidence", 1)`+return；未命中原样 push_error+audio_feature.invalid。不改内核、不改 agent 侧（卡面约束遵守）。
- **探针（场景模式，213 同族）**：`tools/probe_telem_l3_noise.tscn`+`_scene.gd`——真 manager+真 perf monitor（引擎装配零自建），payload=双侧内核戳记锚点亲读镜像；mock receiver+fake reader 为单元语义替身（真 reader=GDExtension headless 缺位；非 GUI 装配卡 §5.1 不适用）。15 检：L1 静默/L2 真违规保红/L3a-L3b AND 严格/F1 双 float 形态（envelope+time_energy）全消费链+计数不误触/P1 真 ingest_vsp_event_telemetry 入口（VSP 事件路）/W6 源面。**边界披露**：push_error 本体不可脚本拦截——零 push_error 经 invalid 分支计数器（audio_feature.invalid 零增）断言，计数器与 push_error 同分支相邻语句等价。
- **验收数据**：前端 commit=**ae2213e**（port/fe-telem-l3noise-1 已推，3 文件 +293）；探针 **15/15×2 连续两轮 exit 0**（迭代轮 r1=探针断言缺陷、parse error 轮=GDScript 列表推导式不支持，均探针面零生产改动）；**全家零回退（fam3）**：26 个 -s 件全 PASS（pipeline_selftest 通配排除生效）+本卡探针 PASS+file_reply 场景件 PASS。工件=coord/runs/FE-TELEM-L3BROADCAST-NOISE-1/l3noise_20261029_1（RESULTS.md 六节+7 件日志，随 **port/fe-telem-l3noise-1-stack** 入库）。
- **端测覆盖边界声明（离线级交付，无真栈腿）**：被测面=前端 handler 形态判别，payload 以内核戳记锚点镜像构造；真栈触发面在内核 L3 分析器（本卡不改内核），用户手测 2026-10-29 12:11 会话已复现即根因取证；消费链经真 ingest 入口+真 handler 全链。用户手测重验收口（可选）：同形态打开含 stems 工程，godot.log 不再出现 `audio_feature_data_ready payload invalid`（观感零变化——噪音消除卡）。写盘隔离：探针进程 VIT_DAW_DEV_ROOT=<临时目录>（mixboard 快照写盘 §10 隔离）。
- **上交主管知悉**：ⓐfam3 附带跑 213 探针 EXIT=1=资源缺失非断言红（wsrecover 探针文件为 213 分支专属提交不在本分支）；两卡修复共存态的探针面（213 消费+215 判别同文件）留待主管合并后验证，属合并面；ⓑsummary 白名单含 l3_acoustic_summary——未来 L3 内嵌广播若以 summary 类型承载将走 summary handler 非本卡判别面，当前广播形态为 time_energy，仅备知不扩域。
- **[等待验收]** 主管裁定 pass/rework；实现分支 port/fe-telem-l3noise-1 待验收不自行 merge。
