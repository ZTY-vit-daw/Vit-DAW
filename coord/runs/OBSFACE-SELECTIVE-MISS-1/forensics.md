# OBSFACE-SELECTIVE-MISS-1 取证报告（2026-11-07，读档零 LLM 零代码改动）

- 执行：GLM 主会话 freeflash 子代理 @ D:/Vit_DAW_wt_272impl（port/obsface-selective-miss-1，基线 37fd7c97）
- 任务：266 两现「kernel 观察面选择性缺失」三组 run digest 逐字段对照 + 铸造路径勘察 + 定性
- 结论先行：**缺陷成立，但位置与卡面前提不同——主缺陷是 agent 侧 d1d3 观察消费面接线错位（确定性，先天），不是 kernel/铸造层非确定性缺失；stereo 面在铸造层从未缺失（13/13 采样点 ready）**。

---

## 1. 前提修正：「kernel 铸造」实际在 agent 侧 Go，不在 VitApp C++

- VitApp C++ 全树（Source/Bridge/Tests）grep `mix_observe` / `mix_observation_digest` / `mix.observe` **零命中**；`stereo_relation` 仅在 `VitApp/Source/Service/AudioFeatureTypes.cpp` 作为 L3 特征类型名出现。
- digest 铸造链（agent 侧）：
  - `agent/internal/harness/harness.go:3706` `requestMixObservation`（`mix_observe` 与 `mix_request_observation` 同 handler，:2792-2794）
  - → `agent/internal/mixboard/mixboard.go:1077` `RequestObservation` → `:1164` `buildObservation`（特征快照装载+新鲜度归一）
  - → `agent/internal/mixboard/catalog.go:93` `BuildDigest`（stereo 面 :130-137；全局 peak/rms/crest :115-119；waveform :121-123；rankings :204-218）
- C++ 内核只提供工程态/分析清单/L3 特征证据（输入面），不铸造 `mix_observation_digest.v1`。

## 2. 三组 run 五消费点对照表（卡面「三组 digest」实为 5 个可对照采样点）

原始 digest=run 工件目录持久化观察包（mixboard 每次 observe 落盘）；投影=chat 工件 workflow_data.observation（d1d3AssemblePreMasterReport 产物）。

| run / 时点(UTC) | 回合 | session | 持久化 digest：stereo | 持久化 digest：waveform(D1 全局) | rankings | 投影：loudness_peak_status | 投影：stereo_status |
|---|---|---|---|---|---|---|---|
| 265 reverify1 15:24:43–15:26:52 | D1 runner 4 点 | mix_605a…/mix_fe2e…/mix_5599…/mix_676f… | ready×4 | **ready×4（含实值）** | 有×4 | （无投影） | （无投影） |
| 265 reverify1 15:28:25 | D2 check | cap_…d2_check_20261106_232316_1 | **ready** | **ready（peak -15.873/rms -30.255/crest 14.383）** | 有 | **missing** | **missing** |
| 266v1 17:16:22 | LW 门1 | mix_cb96ffa9663e | ready | **missing**（键在场值全 null） | 有 | （原始 digest 直接在证） | — |
| 266v1 17:17:21 | D1 settle | cap_…d1_settle_2 | **ready** | missing | 有 | **missing** | **missing** |
| 266v2 17:36:32 | probe | mix_3395a1ac9bc6 | ready | missing | 有 | （原始 digest 直接在证） | — |
| 266v2 17:36:50 | D3 check | cap_…d3_check_20261107_013013_1 | **ready** | missing | 有 | **missing** | **missing** |

补充序列事实：
- 266v1 内 waveform 面 17:14:23 ready（项目域）→ 17:15:06 missing（`d1_session_turnfree_state_32` 项目域 observe，D1 settle 1-mutation 相位内），同刻逐轨域 observe（17:15:12 track 1027）仍 ready；v2 同型翻转在 17:34:00（`d1_session_turnfree_state_0c82`）。
- 265 同链（D1 1-mutation 主链）项目域 waveform 全程未翻转——翻转条件未定位（见 §5 未决）。
- **stereo_status 在全部 13 个持久化采样点（265:5 + 266v1:6 + 266v2:9 中的全部 run 时点）无一缺失**。

## 3. 主缺陷：d1d3 观察消费面接线错位（确定性，先天）

### 3.1 错位链

- d1d3 runtime 每回合经 `d1d3InvokeReadOnly`（chat/d1d3_premaster_observation_runtime.go:210-229）调 `mix.observe`，返回 **harness 响应原样**（`resp.Result`）。
- `requestMixObservation` 的响应顶层键：status/mix_session_id/observation_id/observation/com_projection/mom_projection/**digest**/catalog/mixboard/context_pack/acoustic_digest/…（harness.go:3893-3907，digest 嵌套在 `out["digest"]` :3903）。
- `d1d3AssemblePreMasterReport`（:315-398）却按**顶层键**读：`observe["peak_dbfs"]`/`observe["rms_dbfs"]`/`observe["crest_db"]`（:339-347）、`observe["waveform"]`（:348）、`observe["project_*_ranking_excerpt"]`（:349-351）、`observe["stereo_status"]||correlation_estimate||balance_db`（:355）——**这些键在顶层从不存在**。
- 拍平数据实际存在但：①在 `out["acoustic_digest"]`（:3915，构建于 mixObservationAcousticDigest :7305-7400）；②stereo 键名不同（`stereo_relation_status`，非 `stereo_status`）。
- 后果（代码路径确定）：`LoudnessPeakStatus` 恒 `missing`（:402-408 全输入恒空）、`StereoStatus` 恒 `missing`（:365-367 else 分支恒走 + 恒加 limitation「全局 stereo_relation 缺席」）。
- **先天非回归**：M1-4 引入 d1d3 的同一提交 e0e2f4ac 中 out map 已是 digest 嵌套形状（`git show e0e2f4ac` harness.go :186 `"digest": result.Observation.Digest`）——d1d3 上线起该投影在任何真实 run 都看不到 digest 面。

### 3.2 为什么单测全绿

`TestD1D3AssembleReportPresentSurfaces`（d1d3_premaster_observation_runtime_test.go:234-249）直接喂 digest 形状的 map（stereo_status/peak_dbfs 在顶层）——把错误形状假设钉死在测试里，真实响应形状从未被单测覆盖。

### 3.3 与三组 run 证据的吻合度（全部吻合）

- 265 reverify1：卡面/RUNBOOK 记「fixture 如实 stereo_face=missing」→ 实为投影错位；同回合持久化 digest stereo=ready 且 waveform ready 含实值（obs_20261106T152825）。D2 verdict=unmeasurable 的「全局 stereo_relation 缺席」半句由错位投影直接导致。
- 266v1 门 A 失败 `loudness_peak_status was 'missing' [expected present]`（ps1:1100-1105）：settle 回合持久化 digest stereo=ready+rankings 在场（obs_171721），投影仍双 missing。
- 266v2 挂账观察项（evidence.md §挂账）：check 回合投影双 missing，同回合持久化 digest 双 ready（obs_173650）——「单开关再现」实质=接线错位在单开关 run 同样必然出现，与开关组合无关（与该卡「候选②非确定性」推断的表象一致、机理不同）。
- 判据消费面健康（loudness_summaries 测量行、verdict=breach/aligned 如实产出）与观察面缺失并存=两源分离：判据走 loudnesswatch 测量行，不走 digest 投影——这与 266 evidence 的观察一致，机理上正确。
- 254（AF-D1-LOUDNESS-SETTLE-1）armrun1/2 均「LW/D1 settle 门未执行到」→ 观察断言 266 才首次真栈行使，先天缺陷此前无暴露机会。

## 4. 定性

1. **主缺陷（成立，建议修）**：agent 侧 d1d3 观察投影消费错位——设计内的「按需披露」不适用（响应顶层从未携带这些键，无任何在案注释或文档说明按顶层读的依据；单测假设与真实形状矛盾即内证）。分类=接线缺陷（消费面 bug），非 kernel 非确定性铸造。影响面=pre_master.observation.v0 投影 + D1 门 A 观察断言 + D2 verdict 的全局 stereo 判据 + D3 观察披露面；不影响 loudness_summaries 判据链与 D1/D3 verdict 本体。
2. **次级事实（真变动，设计内疑似，两案并陈）**：D1 全局 waveform 面在 266 两 run 的 D1 settle 相位内项目域 ready→missing（265 同链未翻转）。missing 形态=键在场值全 null+status:"missing"（诚实保留缺失、不升级，符合 M1-4 语义）；逐轨域同刻仍 ready。倾向设计内新鲜度失效（mutation/处理链后 legacy 项目域行失效，代码锚点：mixboard.go `normalizeTrackWaveformFeatureFreshness` :1185 / `normalizeProjectFeatureMaterialFreshness` :2513 / `listenScopeAllowsLegacyProjectWaveformRows`；另有 waveform_freshness_residual_test.go 在案），但 265 未翻转的差异驱动（LW after_processing？agent 版本 853bcfb1 vs F4B5A215？）本卡未定位——如实两案并陈交裁定。
3. **卡面候选②（kernel 观察面选择性缺失非确定性）不成立**：三组 run 的「选择性缺失」表象由①（消费错位，确定性 missing）与②（waveform 新鲜度翻转，相位相关非随机）叠加造成；铸造层 stereo 面零缺失。

## 5. 相邻发现（锚点记录，不扩查）

- 逐轨 stereo 行：265 check 回合 mix.read `track.<id>.slow.stereo.summary` 7/7 missing，而 feature snapshot 内 19 条 `stereo_relation_summaries` 行全 ready（265 工件 `.vit_agent/vitproj_b3f…/mixboard/mixboard_feature_snapshot.json`）——mix.read 消费路径第二处错位候选（d1d3ReadTrackStereo :251-279 → readMixObservation harness.go:5083），建议并入修复卡一并取证。
- 266v2 一例 track 域 digest（17:32:34 track 1007）waveform 键整体缺席+rankings 无——与 status:"missing" 形态不同，未查（观察窗内单一例）。

## 6. 最小修方案草案（kernel 面=域外零改动；以下全部 agent 侧）

1. **接线修正（一处，最小）**：`d1d3InvokeReadOnly` 返回前把 `out["digest"]` 并入顶层（或 `d1d3AssemblePreMasterReport` 开头 `digest := mapValue(observe["digest"])` 后全量改读 digest 子图）——优先后者（显式、不污染响应契约；注意保持 `observation_id` 仍读顶层，顶层确有）。
2. **单测反钉**：改 `TestD1D3AssembleReportPresentSurfaces` 为喂真实响应形状（digest 嵌套 map），新增一条「out 形状含 acoustic_digest 拍平面」的接线测试防回退。
3. **真栈门槛已就位**：266 已有的 D1 门 A 观察断言（ps1:1100-1105）+D2 门 A 在修复后自然成为回归门（预期修复后 stereo/present+rankings 行随 digest 健康如实披露）。
4. 顺带裁定项交主管：D2 verdict 文案「fixture 如实」语义需修正历史口径（265 的 unmeasurable 由接线缺陷导致，非 fixture 事实）。

## 7. 证据件清单（绝对路径）

- 仓库内（本次取证引用，未改动）：
  - D:/Vit_DAW/coord/runs/D2-STEREO-THIN-1/reverify1/d2_check_chat.json（265 投影 missing 面）
  - D:/Vit_DAW/coord/runs/D3-REF-ALIGN-THIN-1/verifyrun1/evidence.md、loudness_watch_observe1.json、d1_settle_chat.json
  - D:/Vit_DAW/coord/runs/D3-REF-ALIGN-THIN-1/verifyrun2/evidence.md、d3_probe_chat.json、d3_check_chat.json
- run 侧持久化实盘（只读访问）：
  - D:/Vit_DAW_wt_265rv/artifacts/free_state_d1_s1/20261106_232316/project/.vit_agent/vitproj_b3f2838183d64add9ca7824b8b3ac557/observations/（obs_20261106T152443…152825 五件+mixboard/mixboard_feature_snapshot.json）
  - D:/Vit_DAW_wt_266impl/artifacts/free_state_d1_s1/20261107_011211/project/.vit_agent/vitproj_b3f2838183d64add9ca7824b8b3ac557/observations/（obs_20261106T171354…171721 九件）
  - D:/Vit_DAW_wt_266impl/artifacts/free_state_d1_s1/20261107_013013/project/.vit_agent/vitproj_b3f2838183d64add9ca7824b8b3ac557/observations/（obs_20261106T173143…173650 九件）
- 代码锚点（worktree D:/Vit_DAW_wt_272impl @ 37fd7c97，与 origin/main 同树）：
  - agent/internal/chat/d1d3_premaster_observation_runtime.go（:143 observe 调用、:315-398 组装、:339-369 错位读、:402-408 status 归一、:251-279 逐轨 mix.read）
  - agent/internal/harness/harness.go（:2792 分发、:3706 requestMixObservation、:3893-3907 out 形状、:3915 acoustic_digest、:7305-7400 拍平构建）
  - agent/internal/mixboard/catalog.go（:93 BuildDigest、:115-123/:130-137/:204-218 各面附加条件）
  - agent/internal/mixboard/mixboard.go（:1077/:1164/:1185/:1479-1513/:2513-2542）
  - agent/internal/chat/d1d3_premaster_observation_runtime_test.go（:234-249 形状假设）
  - scripts/run_free_state_d1_smoke.ps1（:1100-1105 门 A 观察断言；:762-771 LW 门 1 observe）

## 8. 边界与诚实声明

- 全程零代码改动、零栈启动、零 LLM；VitApp 只读 grep；run 工件目录只读访问（含 wt_265rv/wt_266impl artifacts）。
- 持久化观察包为 mixboard 落盘原件（含 created_at/session/digest），与 chat 工件的 observation_id 互证（obs_20261106T171721/173650/152825 三点 id 双侧一致）。
- 未决（本卡不扩查）：§4.2 翻转驱动差异、§5 mix.read 逐轨路径、§5 单例 track 域 digest 形态。
