# RESULTS — FE-TELEM-L3BROADCAST-NOISE-1（215）l3noise_20261029_1

执行：GLM-5.3 ZCode PC 量产执行会话（L1 档）/ 2026-10-29 晚窗
被测：前端仓 worktree `D:/Godot/project/vit-daw-frontend-l3noise1` @ `port/fe-telem-l3noise-1`（基线=前端 main 37dc4b6）
性质：**离线卡（零真栈腿、零端口触碰、写盘隔离）**——场景探针进程设 `VIT_DAW_DEV_ROOT=<临时目录>`，mixboard 快照写盘隔离到临时工作区（§10）

## 一、根因与判别定案

同一命令名两种形态（双侧内核锚点亲读复核）：WaveformEnvelopeBaker float 引用形态（`feature_type=waveform_envelope`+shared_memory+float_count，**无 schema_version 戳记**）vs L3AcousticAnalyzer `stampCommon` 证据内嵌形态（`schema_version="dad_l3_"+featureName+".v1"`、`source="kernel_l3_offline_analyzer"`、`capture_mode=offline_full_song`，无 shared_memory/float_count，受众=agent 观察层）。前端 `_handle_audio_feature_data_ready` float 门（缺 shared_memory）落 invalid 分支 `push_error` 报红——形态不匹配非协议违规，数据受众本非前端。

**判别条件（执行侧定案披露）**：`_is_l3_offline_embedded_feature(d)` = `schema_version.strip_edges().begins_with("dad_l3_")` **AND** `source.strip_edges() == "kernel_l3_offline_analyzer"`，置于 invalid 分支内 push_error 之前。取双戳记 AND 的理由：
- 真协议违规保红：baker 系 float 期望包无 schema_version 字段（锚点亲读）——破损形态（缺 shared_memory）不携带任一 L3 戳记，照旧 `push_error`+`audio_feature.invalid` 计数；
- 单标记防误放：仅带 dad_l3 schema 而来源漂移、或仅带 kernel_l3 source 而 schema 漂移的形态均不命中（探针 L3a/L3b 断言钉死）；
- WaveformEnvelopeBaker float 形态零误伤：健康形态有 shared_memory 根本不入 invalid 分支；探针 F1 以健康 envelope+time_energy 双 float 形态过全消费链（reader 读长+receiver 派发）实证。

## 二、修复面

`app/kernel/autoloads/telemetry_manager.gd` 单文件 +24：①`_is_l3_offline_embedded_feature` 判别 helper（含双戳记 AND 理由成文注释）；②invalid 分支内前置静默路径——命中即 `VitDebugFlags.log_wave_diag`（对齐 :1112 ignored unsupported 同形态）+`_perf_count("audio_feature.ignored_embedded_l3_evidence", 1)`+return；未命中原样 `push_error`+`audio_feature.invalid`。不改内核、不改 agent 侧（卡面约束遵守）。

## 三、探针面（场景模式，213 同族）

`tools/probe_telem_l3_noise.tscn`+`_scene.gd`：真 VitTelemetryManager+真 VitPerfMonitor（引擎装配），payload 为双侧内核戳记锚点亲读镜像；mock receiver（vit_track_rows 组+get_bound_kernel_track_id 绑定）与 fake reader（定长返回）为单元语义替身（真 reader=GDExtension headless 缺位；真 receiver=轨行场景件；本卡被测面=autoload 内形态判别与消费契约，非 GUI 装配，§5.1 不适用）。15 检：L1 静默面/L2 真违规保红/L3a-L3b AND 严格面/F1a-F1d 双 float 形态全消费链+计数不误触/P1 真 ingest_vsp_event_telemetry 入口（VSP 事件路）排空后同断言/W6 源面。

**边界披露**：push_error 本体不可脚本拦截——零 push_error 经 invalid 分支计数器（`audio_feature.invalid` 零增）断言，该计数器与 push_error 同分支相邻语句，等价性在卷。

## 四、验收数据

- 前端 commit=**ae2213e**（port/fe-telem-l3noise-1 已推，3 文件 +293）。
- 探针 **15/15×2 连续两轮 exit 0**（迭代轮留证：r1=F1a 探针断言缺陷[双 float 包后误断言 reader 调用==1，实为 2]、parse error 轮=GDScript 不支持列表推导式[Python 语法残留]，均探针面零生产改动）。
- **全家零回退（fam3）**：26 个 -s 件全 PASS（排除法修正：`probe_pipeline_selftest*` 通配排除生效）+本卡探针 PASS+file_reply 场景件 PASS。fam3 附带跑 213 探针 EXIT=1=**资源缺失非断言面**（wsrecover 探针文件是 213 分支专属提交，不在本分支——跨分支断言行为留待主管两卡合并面验证，此处不作跨分支断言）。

## 五、边界声明（AGENTS §5 义务）

- **离线级交付，无真栈腿**。理由：①被测面=前端 handler 形态判别，payload 以内核戳记锚点亲读镜像构造，保真度等价于真广播（真栈复现需含完整音频轨工程触发 L3 全曲分析——用户手测 2026-10-29 12:11 会话已复现 6 条同型 ERROR，即本卡根因取证）；②触发面在内核 L3 分析器（本卡不改内核），真栈腿无法额外验证前端判别面；③消费链经真 ingest 入口+真 handler 全链。用户手测重验收口（可选）：同 12:11 形态打开含 stems 工程，godot.log 不再出现 `audio_feature_data_ready payload invalid` 报错（波形/车道显示此前已确认不受影响，本卡零观感变化——噪音消除卡）。
- 探针进程全部 headless 自退；UDP 4444 未触碰（VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE=1）；mixboard 快照写盘隔离临时目录。

## 六、上交主管知悉

1. **领取时序披露（纪律 slip，如实记录）**：本卡领取提交与实现同批推送（前端 worktree 创建+基线记录 19:23 即实质领取，领取提交推送晚于实现开始）——卡面为发卡主管本地未跟踪件（远端不存在）、无跨会话争用面、用户明示本会话串行两卡，风险面为零；协议字面顺序（领取推送先于实现）未守，下不为例。
2. **fam3 附带项更正**：跑批脚本误含 213 探针——其文件不在本分支（213 分支专属提交），EXIT 1=资源缺失非断言红；两探针跨分支行为（213 修复+215 判别共存态）留待主管合并后验证，属合并面非本卡面。
3. summary 白名单含 `l3_acoustic_summary`：若未来 L3 内嵌广播以 summary 类型承载将走 summary handler（mixboard l3 快照行）非本卡判别面——当前广播形态为 time_energy（卡面锚点），不扩域处置，仅备知。
