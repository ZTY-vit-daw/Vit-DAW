# RLM-PROFILE-2 目标 3：kernel 流式计量现状实勘（2026-09-29，Mac 执行侧）

范围：只取证不接线。供后续"render 后/播放中响度监督"卡设计。

## 1. AudioFeatureTypes loudness 面

- 枚举：`AudioFeatureType::LoudnessSummary`（schema `loudness_summary.v1`；`VitApp/Source/Service/AudioFeatureTypes.cpp:16/83`）；解析别名 `loudness`/`lufs_summary`/`lufs_analysis`（:42-43）。
- 同级还有 `L3AcousticSummary`（`l3_acoustic_summary.v1`，:17/84）——loudness 是 L3 声学分析的一个发布原语。

## 2. 计量实现现状：近似，非 BS.1770

`VitApp/Source/Service/L3AcousticAnalyzer.cpp:790-812 publishLoudnessSummary`：

- `integrated_lufs = rmsDb − 0.691`（RMS 一阶近似），同值重复发布为 `approximate_lufs`；
- 显式自标 `approximate=true`、`algorithm="approximate_rms_lufs_v1"`（诚实标注，下游可分辨）；
- **无** K-weighting 滤波、**无** BS.1770 gating（相对门/绝对门）、**无** short-term(3s)/momentary(400ms) 窗口量；
- **无 true peak 计量**：全内核 grep `true_peak|truepeak|oversampl` 仅命中插件参数词表（`VitApp/Source/Core/VitPluginTemplateRegistry.cpp:184`，匹配的是 oversampling 参数名），不存在过采样真峰值计量器。

结论：现 loudness 面精度不足以支撑 delivery profile 断言（profile 容差 ±0.5~2 LU；RMS−0.691 与真 integrated LUFS 的偏差随素材 crest/gating 结构典型可达数 LU）。

## 3. 触发路径与遥测暴露

- 触发：音频导入期——`VitApp/Source/Service/ImportService.cpp:588`（导入时构造 `l3Request.featureType = L3AcousticSummary`）→ `AudioFeatureService.cpp:199-202` 分发 `L3AcousticAnalyzer::startAnalyze`（异步）→ publish 回调出结果。
- bake 面门槛：`AudioFeatureService.cpp:128-134 audio_feature_bake_status` 在 feature baker 未晋升时显式拒（reason `feature_baker_not_yet_promoted`）——任何挂接 bake 面的后续卡要过晋升门。
- agent 侧可见性：loudness_summary 经 L3 发布进观察面（agent 侧 harness `loudness_summary`、rlm `Input.AudioAnalysisStatus` 消费同族字段：`integrated_lufs`/`approximate_lufs`——rlm 行已消费近似 LUFS 并标 approximate 语义）。
- **render 完成路径零计量挂接**：`VitApp/Source/Service/VitProductionCoordinator.cpp:724` render_done/render_failed 遥测只报 job 状态+file_path；render 产物不进任何分析/计量。

## 4. 监督可行落点评估（事实性，不做设计）

- 落点 A（agent 侧，零内核改动）：render_done 后 agent 已持有交付文件路径（`RenderResult.FilePath`，harness renderResults）→ agent 侧读文件做真 BS.1770 计量（需在 Go 侧新建 K-weighting+gating 计量模块；agent 现无音频 DSP 基建，属新能力建设）。
- 落点 B（内核扩容）：L3AcousticAnalyzer 增真 BS.1770 + true peak（4x 过采样）计算并挂 render_done 路径——改内核分析器+可能扩命令面，须决策侧另立卡。
- 边界事实：交付级断言（profile 的 LUFS 目标带+TP ≤−1/−2 dBTP）以真 BS.1770/BS.1770-4 true peak 为必要条件；现近似算法不可作断言依据，只能作粗筛。

## 实勘结论

内核 loudness=导入期近似（`approximate_rms_lufs_v1`，诚实自标），render 后与播放中均无响度计量挂接，true peak 全缺。后续流式监督卡的最小落点=先 agent 侧文件计量（复用 RenderResult.FilePath），内核真 BS.1770 为交付级断言的前置建设项。
