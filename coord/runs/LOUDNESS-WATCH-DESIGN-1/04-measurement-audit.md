# 设计④：测量口径核对（integrated / short-term / momentary / true peak 全口径就位度）

LOUDNESS-WATCH-DESIGN-1 设计件 4/6。结论先行：**当前全栈只有"近似 integrated + sample peak + crest"三样生产者，且全部是 clip 域**；RLM/manifest/配置面的窗口与 TP 字段是"形备量缺"。缺口按内核补齐面列出（单独实现卡候选）。

## 1. 核对范围与判定方法

- 内核生产面：`VitApp/Source/Service/L3AcousticAnalyzer.cpp`、`AudioFeatureTypes.h`、`AudioFeatureService.cpp`。
- agent 消费面：`rlm`（profile/disclosure/projection）、`acousticpackage`、`capabilitycontext`、`mixboard`、`fxm`、`harness`。
- 判定口径：有生产者=就位；只有 schema 字段/披露格式无生产者=形备量缺；字段都不存在=缺。

## 2. 就位表（有生产者）

| 口径 | 生产者 | 锚点 | grade | 消费面已接 |
|---|---|---|---|---|
| integrated（近似） | 内核 LoudnessSummary：`integrated_lufs = rmsDb - 0.691`，`algorithm="approximate_rms_lufs_v1"` | `L3AcousticAnalyzer.cpp:791-810`（`:807-808` 赋值，`:810` 算法名） | **近似**：未加 K 加权、无绝对/相对门控，是 RMS 偏移近似 | `acousticpackage/status.go:350,378`（loudness_summary 特征行）；`capabilitycontext/context_manifest.go:144-145`（integrated_lufs=Primary 位 + approximate_lufs=Approximate fallback 位，模式层已备真口径槽位）；`capabilitycontext/gain_staging.go:359`；`mixboard/catalog.go:204-205`；`harness/harness.go:6320`（bridge 质量分） |
| sample peak | `peak_dbfs`（`analysis.peakAbs` 取 dB） | `L3AcousticAnalyzer.cpp:800,804`；帧级 `:42-43` rmsDbfs/peakDbfs | sample 精度（无过采样） | 同上 manifest `:150` peak_dbfs=headroom 位 |
| crest | `crest_db = peak - rms` | `L3AcousticAnalyzer.cpp:812` | 同上 | FXM delta 消费（`fxm/projection.go:106-108,138-139`） |
| AudioFeatureTypes 类型位 | `AudioFeatureType::LoudnessSummary` 枚举 | `AudioFeatureTypes.h:16` | — | bake 请求链 `harness/harness.go:3860,3872`（loudness_summary 在观察就位门必需特征表） |

## 3. 缺口表（无生产者，按影响排序）

| # | 缺口 | 证据 | 对 watch 的影响 |
|---|---|---|---|
| 1 | **BS.1770 K 加权+门控 integrated**（-70 LUFS 绝对门 + -10 LU 相对门 + K 加权预滤波） | `L3AcousticAnalyzer.cpp:807-810` 只有 RMS 偏移式；全文件无 K-weighting/gating 实现 | v1 verdict 只能 approximate 级（设计③ §1.3 强制标注） |
| 2 | **short-term max（3s 窗）** | RLM 形备：`rlm/profile.go:60` ShortTermMax 字段、`disclosure.go:87` 披露格式；无任何生产者 | 配置可存该阈值但比对恒 unmeasurable（该窗）——如实，不伪造 |
| 3 | **momentary max（400ms 窗）** | 同上：`rlm/profile.go:61`、`disclosure.go` 只格式化 | 同上 |
| 4 | **true peak（过采样 dBTP）** | `L3AcousticAnalyzer.cpp:270-273` 显式声明"不是 sample-accurate band true-peak"；全栈无过采样 TP 表 | sample peak 对 TP 上限只能做**必要条件检查**（sample_peak > TP 上限 ⇒ 必 breach；sample_peak ≤ 上限 ⇒ inter-sample 峰不可见，不能判合规）——verdict 的 peak 维度只产 breach 或"未见 breach"，不产 pass |
| 5 | **母线/成品域测量**（master bus 或 render 产物分析） | 特征 bake 强制 clip 源：`harness/harness.go:4927-4931`（缺 track_id/clip_id 即 blocked `clip_source_required_for_current_feature_bakers`）；full_project 域也只是逐可见轨 clip 迭代（`:5020-5030`）；无母线 tap、无 render 文件分析路径 | v1 watch = **逐 clip 近似检查**（对配置带逐 clip 出 breach）；"整曲 integrated 对准预设带"的交付级判定依赖本缺口，后置（与 D/E 母带实现后置裁定一致） |
| 6 | FXM Measurement.LUFS 生产者 | 字段在（`fxm/types.go:69-71` LUFS/Peak/Crest、`:123-125` delta），勘察未发现填充点（`fxm.Build` 唯一调用 `mixboard/mixboard.go:3996` 传空 Input） | 如实登记；不阻塞 watch（watch 不走 FXM） |

## 4. 内核补齐面（实现卡候选，规格入 05 拆卡件）

**KERNEL-LOUDNESS-BS1770-1（候选，不发卡，内核线排序归主管）**：

- 文件域：`VitApp/Source/Service/L3AcousticAnalyzer.cpp`（publishLoudnessSummary 扩展）、`AudioFeatureTypes.h`（如需新枚举/字段）、对应测试。
- 内容：①K 加权预滤波（BS.1770-4 一阶/二阶节）+双门控 integrated（真口径 `integrated_lufs` 原位升级，`approximate_lufs` 字段保留作对照）；②short-term（3s 滑窗）max 与 momentary（400ms 滑窗）max 输出；③4× 过采样 true peak（dBTP）。
- 验收：合成已知信号断言——`agent/internal/probeaudio/suite.go:53` Generate 已有 logSweep/multitone/steppedSine/transientBurst/stereoProbe/tailProbe 合成信号族（`:106-185`）可作 fixture 源；恒幅 -20 dBFS 立体声正弦的 BS.1770 integrated 期望值误差 ≤0.1 LU；TP 对方波/瞬态信号的 inter-sample 超峰可检出。
- 完成后 watch 侧动作：verdict grade 升 `bs1770`、ST/M/TP 维度启用、peak 维度升可判 pass（实现卡 C 预留 grade 分支，内核卡合入即生效，无需改 watch）。

## 5. v1 watch 的口径决定（钉死）

1. v1 测量源 = 既有 LoudnessSummary 行（approximate integrated + sample peak），**不新增测量生产者**。
2. verdict 全量强制 `measurement_grade="approximate"` + 文案限定词；peak 维度只产"未见 breach"，不产 pass。
3. ST/M 阈值配置面可存、比对面恒 unmeasurable（该维度），状态面显示原因。
4. 母线级/交付级判定明确后置到缺口 5 的内核卡，不在 watch v1 范围内伪装。
5. 以上边界全部写进用户可见文案与模型可见 verdict 字段——把"能测什么"本身作为披露内容，而非隐藏假设。
