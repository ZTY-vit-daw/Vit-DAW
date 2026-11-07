# WE-MISMATCH-FORENSICS-1 日志证据清点（只读摘录，2026-11-07）

## 266 verifyrun1（wt_266impl artifacts/free_state_d1_s1/20261107_011211）run 窗口内 "1055" 零命中

| 文件 | 命中数 |
|---|---|
| agent_last.log（全量 254 行） | 0 |
| runner_stdout.log | 0 |
| e1_extract.txt | 0 |
| telemetry_run_window.jsonl | 0 |
| console_attempt1.txt | 0 |
| d1_smoke_report.json / d1_settle_chat.json / loudness_watch_observe1.json | 0（仅 obs id 数字串无关命中，逐件 grep 亲核） |

结论：run 内不存在任何 clip 1055 的 kernel 遥测（audio_feature_data_ready/waveform_envelope）或含 1055 的写请求——`kernel_prepared_waveform_envelope_1055` 行非 run 时产生。

## 266 run 内 kernel 遥测落盘件（wt_266impl/VitApp/Workspace/Logs/BridgeReplies，run 窗口全列）

- 20261107_011423_271216200_telemetry_l2_render_probe_ready.json（56917B，clip_id=1031，track 1027 probe 遥测，亲读 payload）
- 20261107_011512_708247400_telemetry_l2_render_probe_ready.json（同形，第二次 probe）

仅此两件，均无 1055。

## 266 agent_last.log 关键时序（本地时 UTC+8，观察 id 为 UTC）

- 01:13-01:14:08 LW 门组与 D1 runner 前段（观察 obs_171354/171408，project scope，WE 视图=_1011 ready，lr=2026-09-05 probe_1007——观察内嵌快照亲核）
- 01:14:22-23 L2 probe（track 1027/clip 1031）：`telemetry spilled ... l2_render_probe_ready`；probe 写换 latest_request=probe_1027_171422（writeMixboardReadyL2RenderProbeSnapshot，harness.go:6882-6911）；obs_171423（track 1027 scope）WE 视图=_1031 ready
- 01:14:38 version.checkpoint（rev 2，c_20261106T171438）
- 01:14:47/01:14:56 mix.tick.pending stored / 用户确认（对 Track 1027 静态 EQ）
- 01:15:04 mutation 执行（shadow deltas：1039-1042 节点/PLUGININSTANCE/CONNECTION 加入）
- 01:15:05 kernel.vsp_state_snapshot reason=mix_request_observation_project_scope_state
- 01:15:06.08Z obs_171506（project scope，settle 相位）WE=_1055 missing + reason=project_material_identity_mismatch ← 翻转时点
- 01:15:11 mix.observe 完成（total_ms=5211）；第二次 probe（lr=probe_1027_171511）
- 01:15:12.7Z 文件末笔（updated_at=17:15:12.7222466Z，WE=_1055 freshness=stale superseded_by=probe_171511——annotateMixboardSnapshotFreshness 盖章非替换）；obs_171512（track 1027 scope）WE 视图=_1031 ready
- 01:16:22Z-01:17:21Z 后续 project scope 观察（obs_171622/171638/171703/171721）全部 WE=_1055 missing 至 run 结束

## 265 reverify（wt_265rv artifacts/free_state_d1_s1/20261106_232316）

- vitproj_b3f28381 mixboard_feature_snapshot.json updated_at=2026-09-05T08:01:28.4787333Z（与 fixture 基线逐字节同值）——run 全程零写入
- 5 个观察（obs_152xxx 系列）内嵌 lr 全部=2026-09-05 probe_1007（resolved_target clip 1011 bass.wav 亲核）；project scope 观察 WE 视图=_1011 ready、track 1032 scope 观察 WE=_1036 ready

## fixture 侧（temp/semantic-processor-agent-project-smoke-v1/fixtures/semantic_processor_project_smoke_v1_80085263a651cf20/projects/spv1_p01）

- spv1_p01.vit（25151B，mtime 2026-09-06 13:30）：数字 id 集合=1001-1036，**无 1055**（regex 全扫亲核）
- spv1_p01.vit.bak_20260905 / spv1_p01.vit.bak_20260906_preclip1055 两个历史备份在场（命名直接指证 clip 1055 的历史存在与移除边界）
- .vit_agent/vitproj_b3f28381 mixboard 快照（基线）：WE=_1055 ready first_received=2026-09-05T07:59:32Z，lr=probe_1007_20260905T080127，twe 19 行（末行=(1007,1055)）
- .vit_derived/vitproj_b3f28381 acoustic_package_status.json：7 包，含 (1007,1055) 包（updated 2026-09-05T15:59:32.767+08 态，source=VitApp/Workspace/Artifacts/smoke/strip_silence_agent_smoke_8764_1788595162074.wav）
- 材质文件在场：D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\strip_silence_agent_smoke_8764_1788595162074.wav（194084 字节，mtime 2026-09-05 15:59 +08，与行内 size=194084/mtime=1788595162174 一致）
