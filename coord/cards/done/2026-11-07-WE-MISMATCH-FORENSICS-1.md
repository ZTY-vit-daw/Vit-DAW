# WE-MISMATCH-FORENSICS-1：D1 波形包络特征物化失配取证（274 相邻勘察①转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[274 回执相邻勘察①](../done/2026-11-07-D1D3-DIGEST-WIRING-1.md)+[274 裁定](../../rulings/2026-11-07-D1D3-DIGEST-WIRING-1-pass.md)④——D1 全局 waveform 面 settle 相位 ready→missing 翻转：266v1 settle 相位内 WE 槽行被 `kernel_prepared_waveform_envelope_1055`（track 1007/clip 1055）替换且材质失配 `project_material_identity_mismatch`（`currentProjectFeatureRowOrMissing`，agent/internal/mixboard/mixboard.go:2534/:2544-2559）；265 同链 WE=`_1011` 匹配未翻转。**1055 行产生者/失配来源在 harness 特征物化链，尚未定位**——本卡取证定位，不预设修法）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 293；目标仓库=主仓 agent（harness 特征物化链勘察）；P2 / 0.5 天 / 依赖=274 工件在库；L1-L2 / PC；零栈（265/266 两 run 工件在库对照）

## 目标（取证卡）

1. 定位 `kernel_prepared_waveform_envelope_1055` 行的产生者（harness 特征物化链锚点）与 project material identity 的失配来源（为何 266v1 settle 相位产生 _1055 而 265 同链为 _1011——差异驱动：两 run 工件+console+时间线对照）；
2. 产出最小修建议（若定位到确定性根因）或如实记录「设计内行为」结论（若 1055 替换属 settle 相位的合法重铸——需证据钉死，不得含糊）；
3. 建议面开修卡 or 记录收案，交主管裁定。

## 约束

- 文件域=coord/runs/WE-MISMATCH-FORENSICS-1/ 工件+报告；**源码零改动**（取证卡，发现缺陷只定位不上手修）；
- 265/266 run 工件只读（禁止写回既有 run 目录）。

## 停止条件

证据不足以区分「物化缺陷」vs「设计内重铸」→两案并陈上交（272 waveform 面先例），不得强行择一。

## 领取/回执/验收

- 领取：2026-11-07 / origin/main=`c2f308cd` / owner=GLM-5.3（ZCode 执行子代理，PC） / 分支 `port/we-mismatch-forensics-1`（工件+报告）+ `coord/293-claim`（卡状态） / worktree `D:/Vit_DAW_wt_293forensic`（实现）+ `D:/Vit_DAW_wt_293claim`（协调）/ 领取基线：HEAD=`c2f308cd`，status/diff --stat 均空（新 worktree 自 origin/main 检出）
- 回执（2026-11-07，执行侧自验完成待验收）：
  - **结论（三选一）**：**设计内行为（读侧诚实披露），物化链无罪**——且卡面前提「settle 相位产生 _1055 替换行」经证**不成立**：266 run 内从未发生 _1055 行的写入或槽替换（agent_last.log 全量/runner/telemetry 窗口/console 逐件 grep "1055" 全零；run 终态 WE 槽行仍是 2026-09-05 原行，first_received/material 字段原样）。翻转真身=266 D1 settle 流程的 L2 render probe 写（harness.go:6882-6911）换写快照 `latest_request` 身份 → 读侧声学包复用桥（mixboard.go:1190→:1563-1577，门=`acousticPackageMatchesLatestRequest` **mixboard.go:1855-1885**）脱落 → fixture 继承的 9 月遗留槽行 _1055 首次以裸槽进 project-scope 归一 → `currentProjectFeatureRowOrMissing`（**mixboard.go:2534/:2544-2559**）判 clip 1055 ∉ 现 track 1007 clips → 诚实 stub（missing+`project_material_identity_mismatch`）→ buildWaveformMetrics 值全 null。274 回执「_1031 被替换为 _1055」修正为：obs_171423（track 1027 逐轨视图）与 obs_171506（project 裸槽视图）系两条读路径对同一未变文件的视图差，非写侧替换。**可操作缺陷面=fixture 卫生**（建议面开卡，见下）。
  - **_1055 产生者锚点**：2026-09-05 07:59:32Z 主仓 workspace strip_silence smoke（材质 `D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\strip_silence_agent_smoke_8764_1788595162074.wav` 今在场，size/mtime 与行内一致；当时工程态 track 1007 含 clip 1055，fixture `spv1_p01.vit.bak_20260906_preclip1055` 备份命名指证移除边界）→ kernel 遥测 `audio_feature_data_ready`+`waveform_envelope` → `ingestKernelWaveformTelemetry` **harness.go:4529-4552** → request id 构造 `kernelFeatureMaterializerRequestID` **harness.go:4863-4878**（`kernel_prepared_<feature_type>_<clip_id>`，track/clip 取自事件字段）→ WE 槽写 `writeMixboardReadyProjectTrackWaveformSnapshot` **harness.go:6729-6760**（门=`projectWaveformMatchesResolvedTarget` **:7060-7068** 仅行/事件目标相等，无工程态校验）→ fixture `.vit_agent/vitproj_b3f2838183d64add9ca7824b8b3ac557` + `.vit_derived` (1007,1055) 声学包。9/6 工程态前进而 fixture 的 .vit_agent/.vit_derived 未随之更新=内在不一致，随 run 工作区复制继承。
  - **差异驱动（两 run 对照，全字段亲核）**：265 reverify（wt_265rv 20261106_232316）快照文件**9 月冻结零写入**（终态 updated_at=2026-09-05T08:01:28 与 fixture 基线同值），5 观察内嵌 lr 全程=9 月 probe_1007（resolved_target=1007/1011）→ 复用桥常开，project-scope WE 视图全程 `_1011` ready；266 同一基线，但 D1 settle 的两次 L2 probe 换写 lr（probe_1027_171422/171511）→ 17:13:54/17:14:08 视图 _1011 ready（lr 尚 9 月）→ 17:14:22 起 lr=(1027,1031) 与 (1007,1011) 声学包失配→桥关→17:15:06 起 project-scope 视图=_1055 missing 至 run 末；逐轨视图（requestedTarget 分支）不受影响（obs_171423/171512=_1031 ready）。材质失配机理=「工程态判定输入变」（现工程无 clip 1055，fixture `spv1_p01.vit` id 全集 1001-1036 亲核）而非「行内材质字段错」（行内字段与 9 月材质逐位自洽）。
  - **报告与工件**：报告 [coord/reports/2026-11-07-we-mismatch-forensics.md](../../reports/2026-11-07-we-mismatch-forensics.md)（锚点矩阵+两 run 对照表+结论+处置建议）；工件 `coord/runs/WE-MISMATCH-FORENSICS-1/`（obs_we_trajectory_266v1.json / obs_we_trajectory_265reverify.json / mixboard_snapshot_three_state_diff.json / acoustic_store_packages_excerpt.json / log_evidence_census.md）——均为只读摘录，未写回 265/266 既有 run 目录。工件+报告在分支 **port/we-mismatch-forensics-1** commit `dd1eff6c`（基线 c2f308cd，6 文件 +677，**源码零改动**，取证卡红线遵守）。
  - **（缺陷面）最小修建议**：harness/mixboard 不开修卡（物化与披露均按设计，272/274 裁定延续）；**建议面开 fixture 卫生卡**——重建/清理 `temp/semantic-processor-agent-project-smoke-v1/fixtures/semantic_processor_project_smoke_v1_80085263a651cf20/projects/spv1_p01` 的 9 月遗留（最小修=剔除或重置 `.vit_agent/vitproj_b3f2838183d64add9ca7824b8b3ac557` 与 `.vit_derived` 中 (1007,1055) 声学包，使 fixture 工程态与观察态自洽；涉 sealed fixture 纪律须走重建流程核 manifest 哈希）。可选加固（交裁定，非必需）：`reusableReadyMixboardFeatureRow`（harness.go:8302-8326）对无 track/clip 身份包复用 ready WE 行时补工程态材质校验——属设计变更须单独立项。266 D1 门 A 真栈回归门维持 274 backlog 记法，fixture 卫生落地后随后续 run 验证。
  - **端测边界声明**：零栈零源码改动取证卡——验证边界=只读工件亲核+源码锚点亲读（基线 c2f308cd），无运行时验证；266v1 exit-1 的直接失败项=loudness digest 面（274 已修域），WE 面翻转系同观察内相邻现象，本卡已收口（报告 §4）。
  - 过程纪律自述：协调 worktree 曾误用 `git reset --soft` 对齐前进后的 main（未弃任何改动；领取提交 acc17453 已在 main 历史亲证），按 PROTOCOL §2.1.3 丢弃过期检出态、自新 origin/main 重建协调 worktree（coord/293-close）完成收口。
- 验收：（主管回填）
