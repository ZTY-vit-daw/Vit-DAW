# MAT-D3 取证工件：label 补齐后 S2 仍红——真根因升级为"观察投影与物化输入的时序两态"

- 执行卡：coord/cards/todo/2026-09-29-MAT-D3.md
- 日期：2026-09-27
- 结论状态：**卡面停止条件触发，取证上交**（label 缺口本身已按卡面修复合成面闭环；
  但真栈 S2 的剩余分歧证明 label 只是输入奇偶性缺口的第一层——完整根因在观察链
  finalize 与物化尾挂取数的时序差，超出本卡"纯物化侧（adapters.go+测试）"文件域）。

## 一、本卡已完成并验证的部分（合成面闭环）

1. **红先行**：`agent/internal/materialize/label_parity_test.go`（MAT-D2 取证最小
   复现形态）——修前 `TestDOMAdapterLabelParityWithObservation` FAIL
   （hash cfa2bbbff638d6bb != cb70d076c3a72a77），修后绿；NE 用例
   （轨名缺失=留空不猜）绿。
2. **实现**：`DOMAdapter.Build` 合成包 `TargetRef.Label = shadowTrackLabel
   (deps.ProjectState, trackID)`——与观察侧 `harness.visibleTrackName` 同源
   （同一 shadow.Summary() 轨行集、同键序 name/track_name、按 track_id/id 匹配）。
   同源链核对：观察侧 `requestMixObservation → UserStateSummary(=
   userVisibleState(shadow.Summary())) → resolveMixObservationTargetContext →
   visibleTrackName`（harness.go:3372/6420/12797）；物化侧
   `materializeDepInputs → deps.ProjectState = shadow.Summary()`（materialize_
   shadow.go:208）。**同名同源成立，未触发卡面"同名异源"停止条件。**
3. **口径修正**（两处合成测试观察侧构造补 label，对齐真栈观察链输出形态——
   真栈 harness 恒从轨行解析 track_name 填 target.Label）：
   `recompute_test.go TestShadowReconcileSyntheticProjectZeroDivergence` 与
   `measurement_carried_test.go TestDOMRowFromObservationSnapshotSourcedNotMarked`。
   断言不变（仍 hash 相等），实际比修前更强（真校验 label 奇偶性）。
4. **门**：`go build ./...` + `go test ./...` 全量 0 FAIL（剥离探针后复跑确认）。

## 二、真栈 S2 重跑记录（label 修复合入后仍红）

| run | 结果 | 关键 hash（fresh/stored） |
|---|---|---|
| materialize_shadow_smoke_20260927_182947 | FAIL（S2 divergence=1→2） | 4ba4341c/a185bd74（两轮各自恒定，payload 相等） |
| materialize_shadow_smoke_20260927_183340 | FAIL（同形态） | fd150f63/dc0bdd2a |
| materialize_shadow_smoke_20260927_183718 | FAIL（同形态，行来源探针轮） | — |
| materialize_shadow_smoke_20260927_183946 | FAIL（同形态，三角重放探针轮） | — |

与 MAT-D2 时代（a17b6910/c80e049d）相比 fresh/stored 均已换值（label 修复改变了
两侧行 hash 基线），但分歧依旧：**进 hash 不进 payload 的字段差仍存在**。

## 三、剩余分歧的机械定位（临时探针，已从交付剥离）

### 3.1 投影逐字段 diff（非 volatile 差异族=band 数据）

观察侧（obs.DOMProjection）vs 物化侧（DOMAdapter 合成包重算，deps 同源）：

- 观察侧 `peak_structure.segment_peak_dbfs_distribution` / `segment_crest_db_
  distribution` 在场；物化侧缺失。
- 观察侧 `activity_structure`：status=ready、segment_count=1、valid=1、active=1、
  coverage_ratio=1、active_ratio=1；物化侧：status=missing、全 0、无 coverage。
- 两投影的 target_ref（label="Track 1"）、mode、status、conditions 前段一致
  ——**label 修复已被真栈数据证实生效**（mat_label=obs_label="Track 1"）。

### 3.2 行来源探针（deps 侧候选行）

deps.FeatureSnapshot（=obs.GlobalSummary["feature_snapshot"]，尾挂时）：
- top waveform_envelope：track_id=1007、analyzer=audio_feature.v1.2、
  time_segments=**false**、keys=30
- band_energy_summary（单数）与 band_energy_summaries 1007 行：analyzer=
  dad_l3_offline_analyzer.v1、band_dynamics 键在场、keys=44（两行一致）
- MixPackage.current_metrics.band_energy：analyzer=dad_l3、keys=30

### 3.3 三角重放探针（决定性证据）

```
matd3_replay_probe track=1007 replay_eq_obs=false
  obs=2953c89f6d0f3681  replay=cff6cab43ce8e27a  mat=3b45c01658555eb3
```

用**同一 obs 包**（尾挂时状态 + 空 Request）重放 dom.Build ≠ obs.DOMProjection：
**观察侧投影是 finalize 时点用当时 snapshot 算的；尾挂取到的
obs.GlobalSummary["feature_snapshot"] 在 finalize 之后已发生变化。**
req.Args 干扰已排除（observationArgs 只注入 feature_snapshot_path /
acoustic_package_status(_path)，与 domInputFromObservation 消费的
dom_mode/sample_rate/channel_count/dom_processor_scope 无交集）。

### 3.4 突变机制候选（供决策侧裁定，未继续深挖）

- `compactFeatureSnapshot → compactFeatureRow`（mixboard.go:2764/2892）新建行
  map 但**嵌套值浅引用**（time_segments/bands/band_dynamics 等是原行对象引用）：
  finalize 后任何持有行引用的链路（遥测 ingest 原地更新、观察链内部步骤）的
  写入都会穿透到 obs.GlobalSummary["feature_snapshot"]。
- 观察链 finalize 后续步骤（FinalizeObservationContext 内 applyBandStereo
  Projection / stripObservationRawKeys 仅在特定 wants 下触发；S2 观察命令形态
  是否触发未定论——strip 会递归删 time_segments/waveform_envelope/
  track_waveform_envelopes，与 deps 侧顶层键在场的事实部分矛盾，倾向遥测
  穿透，但未最终定案）。

## 四、修复方向候选（超出本卡文件域，交决策侧裁定）

- A. 物化尾挂取数改为 finalize 时快照（obs finalize 后深拷贝 feature_snapshot，
  或 materializeDepInputs 换时机）——动 harness/mixboard。
- B. 对账基准换源：DOMRowFromObservation 的基准从 obs.DOMProjection 改为
  "尾挂态重算"——动 materialize 对账语义（观察基准定义变更，需裁定）。
- C. MAT-D2 时代已否决的 DepInputs 扩轴携带 MixPackage/TimeRuler（方向①）——
  3.3 证明 replay（含 MixPackage/TimeRuler）≠ obs 投影，**扩轴也不够**
  （时序两态仍在），该方向获得新的否定证据。

## 五、G2-D 状态声明

**G2-D 未闭环。** label 缺口（MAT-D2 定位的第一层装配缺口）已修复并经合成面
与真栈投影 target_ref 双重验证；但真栈 S2 的 ShadowDivergences==0 还被时序两态
卡住。本卡交付=label 修复（独立成立、可合入）+ 本取证（下一张卡的输入）。
