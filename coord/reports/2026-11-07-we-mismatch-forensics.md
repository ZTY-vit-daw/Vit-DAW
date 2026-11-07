# WE-MISMATCH-FORENSICS-1 取证报告：`kernel_prepared_waveform_envelope_1055` 行来源与 D1 全局 waveform 面 settle 相位翻转机理

- 执行：GLM-5.3 ZCode 执行子代理（PC）/ 2026-11-07 / 卡 293 / 分支 `port/we-mismatch-forensics-1` / 基线 `c2f308cd`
- 输入：274 回执相邻勘察①（`coord/cards/done/2026-11-07-D1D3-DIGEST-WIRING-1.md`）+265/266 两 run 工件（只读）+主仓 agent 源码（只读，零改动）
- 工件：`coord/runs/WE-MISMATCH-FORENSICS-1/`（obs_we_trajectory_266v1.json / obs_we_trajectory_265reverify.json / mixboard_snapshot_three_state_diff.json / acoustic_store_packages_excerpt.json / log_evidence_census.md）

## 0. 结论（三选一裁定 + 前提修正）

**结论=设计内行为（读侧诚实披露），物化链无罪；且卡面前提「settle 相位产生 _1055 替换行」经证不成立——run 内从未发生任何 _1055 行的写入或槽替换。** 翻转的真身是：266 run 的 D1 settle 流程（L2 render probe 写）更换了快照 `latest_request` 身份，使读侧「声学包复用桥」脱落，令 fixture 继承的 2026-09-05 遗留槽行（_1055）首次以裸槽形态进入 project-scope 读，被 `currentProjectFeatureRowOrMissing` 按工程态判失配→诚实 stub（missing + `project_material_identity_mismatch`）。**可操作缺陷面不在 harness 物化链，而在 fixture 卫生**：`semantic_processor_project_smoke_v1_80085263a651cf20` 携带 9 月 5 日 `.vit_agent`/`.vit_derived` 遗留（含 clip 1055 行与声学包），与 9 月 6 日工程态（无 clip 1055）内在不一致，随 fixture 复制进入每个 run。建议面开 fixture 卫生卡（见 §5），harness/mixboard 不开修卡。

274 回执中「WE 槽行从 `_1031` 被替换为 `_1055`」的表述按本取证修正为：**obs_171423（track 1027 scope，逐轨视图=_1031）与 obs_171506（project scope，裸槽视图=_1055）是两条读路径对同一未变文件的两种视图**，非写侧行替换。272「设计内新鲜度失效、诚实缺失不升级」裁定在行级时序上进一步精化为「fixture 遗留失配态 + 读侧桥脱落」。

## 1. `_1055` 行的产生者（原始物化锚点，2026-09-05，合法）

| 环节 | 锚点 |
|---|---|
| 触发事件 | kernel 遥测 `audio_feature_data_ready` + `feature_type=waveform_envelope`，track 1007 / clip 1055，材质=`D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\strip_silence_agent_smoke_8764_1788595162174.wav`（文件今在场：194084B，mtime 2026-09-05 15:59 +08） |
| 摄入分发 | `agent/internal/harness/harness.go:4432-4452`（`IngestKernelTelemetry` case `audio_feature_data_ready`+`waveform_envelope`）→ `ingestKernelWaveformTelemetry` **harness.go:4529-4552** |
| request id 构造 | `kernelFeatureMaterializerRequestID` **harness.go:4863-4878**：`kernel_prepared_<feature_type>_<clip_id>`——`_1055` 即 clip id 1055（track/clip id 直接取自遥测事件字段，harness.go:4530-4531） |
| WE 槽写入 | `writeMixboardReadyProjectTrackWaveformSnapshot` **harness.go:6729-6760**：`existing["waveform_envelope"] = waveform`（:6743），门=`projectWaveformMatchesResolvedTarget` **harness.go:7060-7068**——仅比较行 track/clip 与本事件 resolved_target 相等（按构造恒真），**不含工程态材质校验** |
| 落点 | fixture 工程 `.vit_agent/vitproj_b3f2838183d64add9ca7824b8b3ac557/mixboard/mixboard_feature_snapshot.json`（WE 槽+twe 末行 (1007,1055)）+ `.vit_derived/.../acoustic_package_status.json` (1007,1055) 声学包 |
| 时点证据 | 行内 `first_received_at=2026-09-05T07:59:32Z`；行内 size/mtime 与在盘 wav 逐位一致；声学包 (1007,1055) updated 2026-09-05T15:59:32 +08；fixture 存在 `spv1_p01.vit.bak_20260906_preclip1055` 备份（clip 1055 的历史存在与 9/6 移除边界由 fixture 自身命名指证） |

**266 run 内零新物化**：agent_last.log（全量）/runner_stdout/telemetry 窗口/console 逐件 grep "1055" 全零；run 终态 WE 槽行仍是 9 月 5 日原行（first_received/材质字段原样）；六条规范 twe 行的 first_received 全为 2026-09-05T05:25Z（无 run 时重烘焙）。266 文件的 run 时写入（两次 L2 probe 写、请求快照写、L3 桥写）均**保留或复用** WE 旧值：probe 写（harness.go:6882-6911）不动 WE；请求快照写（harness.go:7809-7840 → `pendingMixboardFeatureRow` :8254 → `reusableReadyMixboardFeatureRow` **:8302-8326**）在包目标无 track/clip 身份时无条件复用任一 ready WE 行。末笔 17:15:12.7 的 `freshness=stale/superseded_by_request` 是 `annotateMixboardSnapshotFreshness`（harness.go:7894-7961）每笔写入的标注，非替换。

## 2. 两 run 差异驱动表（为何 266 翻转、265 未）

| 维度 | 265 reverify（wt_265rv 20261106_232316） | 266 verifyrun1（wt_266impl 20261107_011211） |
|---|---|---|
| 继承基线（同一 fixture） | WE=_1055 ready / lr=2026-09-05 probe_1007 / twe 19 行 | 同左（三态 diff 工件亲核，逐字段一致） |
| run 对快照文件的写入 | **零**（终态 updated_at=2026-09-05T08:01:28 与基线同值） | 有：L2 probe×2 换写 latest_request（probe_1027_171422 / probe_1027_171511）+L3 桥/请求写；WE 槽值从未被换 |
| 观察内嵌 latest_request | 全程=9 月 probe_1007（resolved_target=1007/1011 bass.wav，5 观察） | 17:13:54/17:14:08=9 月 probe_1007 → 17:14:22 起=probe_1027（1027/1031 piano.wav） |
| project-scope WE 视图 | 全程 `_1011` ready（复用桥常开） | 17:13:54/17:14:08 `_1011` ready → **17:15:06 起 `_1055` missing+mismatch**（桥脱落后裸槽浮出）→至 run 末 |
| track-scope WE 视图 | `_1036`（track 1032）等，逐轨行就绪 | `_1031`（track 1027，obs_171423/171512，requestedTarget 分支） |
| 触发差异根因 | D2 门组流程=mix.read 只读，无 probe/后台填充写此文件 | D1 settle 流程发 L2 render probe（mix.observe 内 `trigger=mix.observe_l2_render_probe`，harness.go:4001-4048）——唯一把 lr 换成 run 时身份的写 |

**读侧桥锚点**（project-scope 视图 `_1011` 的来源，随 lr 身份开合）：`applyAcousticPackageStatusToFeatureSnapshot`（mixboard.go:1190 调用、:1563-1577 实现把声学包 l1_static waveform 行写回 WE 视图），门=`acousticPackageMatchesLatestRequest` **mixboard.go:1855-1885**（status 身份 vs `latest_request.resolved_target` 的 track/clip/path/revision 相容判定）；声学包行由 `acousticpackage.BuildStatus` 的 `bestFeatureRow`（acousticpackage/status.go:337-362）从槽+行集选优。265/266 前段 lr=(1007,1011) 与 (1007,1011) 声学包相容→桥开；266 17:14:22 后 lr=(1027,1031) 与其失配→桥关→裸槽 _1055 进入归一化。诚实标注：服务 `_1011` 视图的确切行号在「声学包 apply 桥」与 `normalizeFeatureSnapshot` 的 `promoteBestL3FeatureRows`（mixboard.go:1558）两候选中未逐行分离——两者共司同一差分输入（latest_request 身份），结论不依赖该分辨；逐轨分支（`_1031/_1036`）由 `freshWaveformRowOrMissing` requestedTarget 分支与 `preserveTargetWaveformTimeSegments`（mixboard.go:2065-2108）服务，与 274 回执一致。

## 3. 材质失配机理（判定输入 vs 行内字段，逐项亲核）

翻转时 `currentProjectFeatureRowOrMissing(row=_1055, projectState, "waveform_envelope")`（**mixboard.go:2534 调用、:2544-2559 实现**）→ `trackWaveformRowMatchesProjectState`（**mixboard.go:2470-2511**）：

1. 工程态 tracks（kernel 实时 get_project_state，6 轨 1007-1032）非空 → 判定启用；
2. row.track_id=1007 → 命中轨 1007；
3. `featureRowMatchesProjectIdentity`（:2561-2572）：row.project_id="current"（泛化身份）→ 跳过 → 过；
4. clip 匹配（:2492-2510）：row.clip_id=**1055** 在轨 1007 clips（现工程=clip 1011）中**无命中 → return false**；
5. → stub `{status:missing, reason:project_material_identity_mismatch, request_id/track_id/clip_id 保留}` → `buildWaveformMetrics`（mixboard.go:3067-3089）值全 null+status missing（与 obs_171506 global_summary.feature_snapshot.waveform_envelope 实盘形态逐字段吻合）。

**失配性质=「工程态判定输入变」而非「行内材质字段错」**：行内材质字段（file_path/size/mtime/source_revision/clip_revision）与 9 月 5 日当时的材质逐位自洽；失配只因现工程态（9/6 后）不再含 clip 1055（fixture `spv1_p01.vit` 数字 id 全集 1001-1036 亲核无 1055）。归一化的轨行过滤（:2517-2533）同判据滤除 twe 中 (1007,1055) 行——obs 内嵌 twe 恒 6 行规范行的成因。

## 4. 与 266v1 exit-1 的关系澄清

266v1 门 A 断言失败（`loudness_peak_status was 'missing'`，ps1:1100-1105）的直接面是 loudness digest 面（274 已修的 `out["digest"]` 接线域）；本卡的 WE 面翻转（waveform_envelope 槽）是同一 settle 相位观察内的**相邻现象**，非该断言的直接失败项。两件事在 274 回执中并列，本卡收口后者。

## 5. 处置建议（交主管裁定）

1. **建议面开 fixture 卫生卡**（目标仓库=temp fixture，非 agent 源码）：重建/清理 `semantic_processor_project_smoke_v1_80085263a651cf20/projects/spv1_p01` 的 9 月遗留——最小修=剔除或重置 `.vit_agent/vitproj_b3f2838183d64add9ca7824b8b3ac557`（其 mixboard 快照与 observations 携带 1055/9 月 lr）与 `.vit_derived` 声学包中 (1007,1055) 包，使 fixture 工程态与观察态自洽；或以现工程态重烘焙基线快照。涉及 sealed fixture 纪律（AGENTS §10）：改动须走 fixture 重建流程并核对 manifest 哈希。
2. harness/mixboard **不开修卡**：物化与披露行为均按设计（272/274 裁定延续）。可选加固（非必需，交裁定）：`reusableReadyMixboardFeatureRow`（harness.go:8302-8326）对无 track/clip 身份的包复用 ready WE 行时可补工程态材质校验——但按「读侧诚实披露」现行设计，写侧留旧+读侧判失配是既定语义，改写侧属设计变更须单独立项。
3. 真栈回归门（266 D1 门 A）维持 274 回执的 backlog 记法，fixture 卫生卡落地后随后续 run 验证。

## 6. 证据清单（全部只读亲核，原件在库）

- 266 obs 时间线（9 观察）/265 obs 时间线（5 观察）：工件 `obs_we_trajectory_*.json`
- 三态快照 diff（fixture 基线/266 终态/265 冻结态）：`mixboard_snapshot_three_state_diff.json`
- 声学包 7 包摘录（含 (1007,1055) 包）：`acoustic_store_packages_excerpt.json`
- 日志零命中清点+关键时序+fixture 侧证：`log_evidence_census.md`
- 代码锚点矩阵：见 §1-§3 表内 file:line（基线 c2f308cd 亲读）
