# PORT-PCA-FULL-CANDIDATES-PC-1 · 步 1 盘点（PC 侧，2026-09-23）

零仓库代码改动；本目录三个工件：本报告 + `step1_pc_pca_inventory.json`（机读全表）+ `step1_parse.py`（解析脚本，可重跑）。

## TL;DR

- **24 pca_job 全 pass**（apply/restore 全 exact，拓扑代稳定），全 Waves `WaveShell1-VST3 17.1_x64.vst3` 壳内主体；"C2-PCR 12 族 24 体"口径核实 = **12 条插件族系 × Mono/Stereo 成对**（ruling `2026-09-18-C2PCR-pass.md` §"每族 Mono/Stereo 齐"一致）。
- 派生真源 = attestation store（rev 53，53 收据 → **36 去重主体，全部 promoted**，17 条 stale 均为同主体被取代收据）。24 认证主体中 **22 已晋升**；**C1 comp Mono/Stereo 认证 pass 但未晋升**（compressor 族在 store 无消费方）。
- 七族实验轴覆盖判定（冻结规则：promoted ∧ 轴覆盖 ∧ 四源锚点）→ **严格可派生候选 27**：de_esser 6 / limiter 6 / gate 3 / transient 2 / multiband 10 / static_eq 0 / broadband 0。歧义 4 例 + 未晋升 2 例上交裁定（§6）。
- **卡面前提偏差**："预期 EQ≥2（bx_hybrid+其他）"不成立——PC 认证库 **0 个 static_eq 主体**（mac 有 3，PC 无）；且 v5 的 bx_hybrid/Vertigo 两主体**不在晋升集**（EQ-1 探测通道锚定，非 PCA 派生）。严格派生下 static_eq/broadband 两族候选=0，v6 若不保留 legacy 条目将使两族实验轴断供——上交决策。

## 0. 领取基线（§12 纪律）

- HEAD `2b5b1513f41a8ee8e88870079eab5fe7d2a6f2a5`；领取时工作树已有改动：`VitApp/Workspace/default_project.xml`（M）+ 其他卡工件目录（coord/runs/*、extension 构建产物、godot-cpp/），与本卡文件域无重叠。
- 本卡步 1 零代码改动，全部输出落本目录。

## 1. 数据源（全部机器本地，解析时间 2026-09-23）

| 源 | 版本/规模 | 状态 |
|---|---|---|
| `C:\Users\timoz\.vit\pca_certifications\` | 24 job（2026-09-18T11:16–11:18Z，C2-PCR rerun 批次），每 job `summary.json` + `cases/*/evidence.json` | 全 pass |
| `C:\Users\timoz\.vit\processor_control_attestations.v2.json` | rev 53；53 收据 → 36 主体 | 36/36 promoted |
| `C:\Users\timoz\.vit\free_state_experiment_plugins.json` | schema `vit.free_state_experiment_plugins.v5`，7 族 × 1 条目 | 现行白名单 |
| `C:\Users\timoz\.vit\plugin_semantics.json` | 991 entries（2026-09-18T11:16Z，scan_plugins） | 38/38 关注主体全命中 |

## 2. 24 pca_job 主体表（身份 × 族 × 认证角色 × 参数锚点）

锚点来自 evidence `selected_forward_controls` 的 control_ref 解码（`role=param_id`）；multiband 锚点含 `band_key`。

| pca_job | 主体 | 形态 | expectation 族 | 认证角色（param 锚点） | 晋升 |
|---|---|---|---|---|---|
| 02cc0b2b | RDeEsser Stereo | St | de_esser | detector_filter=1, reduction_range=3 | ✓ |
| 8c18792d | RDeEsser Mono | Mo | de_esser | detector_filter=1, reduction_range=3 | ✓ |
| 7425bc72 | DeEsser Stereo | St | de_esser | detector_filter=2, threshold=0 | ✓ |
| 9c1c2206 | DeEsser Mono | Mo | de_esser | detector_filter=2, threshold=0 | ✓ |
| 145ea8bf | Sibilance Mono | Mo | de_esser | reduction_range=3, threshold=2 | ✓ |
| 3e6f7f50 | Sibilance Stereo | St | de_esser | reduction_range=3, threshold=2 | ✓ |
| 04d183a7 | L2 Stereo | St | limiter | threshold=0, ceiling=1 | ✓ |
| a8729807 | L2 Mono | Mo | limiter | threshold=0, ceiling=1 | ✓ |
| 0a48d560 | L1 limiter Stereo | St | limiter | threshold=2, ceiling=1 | ✓ |
| e2f19f2b | L1 limiter Mono | Mo | limiter | threshold=2, ceiling=1 | ✓ |
| b6a2c9bc | C4 Stereo | St | multiband_dynamics | band_1 gain=8, band_1 attack=10 | ✓ |
| f0716b53 | C4 Mono | Mo | multiband_dynamics | band_1 gain=8, band_1 attack=10 | ✓ |
| 61305f42 | C6 Stereo | St | multiband_dynamics | gain=11, attack=13 | ✓ |
| 3609a45f | C6 Mono | Mo | multiband_dynamics | gain=11, attack=13 | ✓ |
| 8c3dda7b | LinMB Stereo | St | multiband_dynamics | gain=2, attack=4 | ✓ |
| 14f3d76d | LinMB Mono | Mo | multiband_dynamics | gain=2, attack=4 | ✓ |
| 599d25df | PSE Mono | Mo | gate_expander | sidechain_highpass=2, range=6 | ✓ |
| c2d05113 | PSE Stereo | St | gate_expander | sidechain_highpass=2, range=6 | ✓ |
| 9b7053f1 | Smack Attack Mono | Mo | transient_shaper | output_gain=9, attack_duration=2 | ✓ |
| fb3a1f3d | Smack Attack Stereo | St | transient_shaper | output_gain=9, attack_duration=2 | ✓ |
| 9f4f2595 | TransX Wide Stereo | St | transient_shaper | transient_range=1, output_gain=0 | ✓ |
| f6523128 | TransX Wide Mono | Mo | transient_shaper | transient_range=1, output_gain=0 | ✓ |
| 5a5e6a0d | C1 comp Mono | Mo | **compressor** | threshold=7, attack=1 | **✗ 未晋升** |
| 77e417f3 | C1 comp Stereo | St | **compressor** | threshold=7, attack=1 | **✗ 未晋升** |

- 全部主体：identifier 形态 `VST3-<name>-695648d-<hash>`（WaveShell1 17.1 壳 CID `695648d` 平台稳定，PLUGIDENT 纪律 identifier 独占解析可用）；厂商 Waves；plugin_path 同壳文件。
- **C1 comp 两主体收据形态差异**：evidence 缺 `apply_tool`/`inspect_tool`/`processor_family` 元数据字段（compressor 通道未落这些字段），但证据实质完整——apply/restore 均 ok/exact、writes 含 `role=threshold param=7`/`role=attack param=1`（channel=shared）、inspect schema `compressor-control-topology/v1`（classification `bidirectional_curve` conf 0.9）。非证据缺陷。

## 3. 晋升集对账（36 promoted ←→ 24 pca_job）

- **22/24 认证主体在晋升集**。
- **14 个晋升主体无本地 pca_job 文件**（更早批次，收据 sha256 摘要在 store evidence 指针内可回指）：Waves 6（Sibilance-Live M/S、C6-SideChain M/S、C1 gate Stereo、C1 comp-gate Stereo）+ 非 Waves 8（Pro-DS、Pro-G、Pro-L 2、SPL、Lindell MBC、Lindell 354E、HUM LAAL、bx_limiter True Peak）。
- 晋升集族分布：de_esser 9 / multiband_dynamics 10 / limiter 7 / gate_expander 5 / transient_shaper 5 = 36。**store 无 static_eq、无 broadband、无 compressor 条目**。

## 4. 七族轴覆盖判定 + 可派生候选清单

判定门（冻结规则）：promoted ∧ 认证覆盖含该族实验轴（de_esser=threshold / limiter=ceiling / gate=range / transient=attack / multiband=band thresholds / static_eq=增益带 / broadband=threshold）。轴名双层映射见 §7。判定明细 36+2 主体全量在 `step1_pc_pca_inventory.json`。

### de_esser（轴=threshold）——pass 6 / ambiguous 1 / fail 2

- **pass**：DeEsser M/S（threshold=0/2）、Sibilance M/S（threshold=2/3）、Sibilance-Live M/S（att axis threshold_sensitivity；无本地 job 证据，锚点需 probe）
- **ambiguous**：Pro-DS——attestation coverage=detector_focus+sibilance_reduction **无 threshold_sensitivity**，但 v5 已有 probe 锚 `threshold_param_id=1`（EQ-1 通道）。轴门按 attestation 严格判不通过，按 v5 锚可过——上交。
- **fail（记因）**：RDeEsser M/S——认证角色 detector_filter+reduction_range，**无 threshold 写**。参数面有 9 参（threshold 参数大概率存在），但按冻结规则"认证覆盖含实验轴"不满足，如实不入列。

### limiter（轴=ceiling）——pass 6 / fail 1

- **pass**：L1 M/S（ceiling=1）、L2 M/S（ceiling=1）、Pro-L 2（att output_ceiling + v5 锚 ceiling=18）、HUM LAAL（att output_ceiling；锚点需 probe）
- **fail（记因）**：bx_limiter True Peak——att coverage=input_drive+output_normalization，**无 output_ceiling**，亦无 v5 锚。族属 limiter 但认证未覆盖 ceiling 轴。

### gate_expander（轴=range）——pass 3 / fail 2

- **pass**：PSE M/S（range=6）、Pro-G（att attenuation_floor + v5 锚 range=4）
- **fail（记因）**：C1 gate Stereo、C1 comp-gate Stereo——att coverage=output_normalization+state_timing，**无 range/attenuation_floor**。

### transient_shaper（轴=attack）——pass 2 / ambiguous 3

- **pass**：Smack Attack M/S（attack_duration=2）
- **ambiguous**：SPL（v5 锚 attack=1098151019 存在，att coverage=detector_focus+envelope_emphasis 无 envelope_timing）；TransX Wide M/S（认证角色 transient_range=1+output_gain=0，transient_range 即 attack 侧增益旋钮但无 attack 命名角色）。attack 轴的角色命名边界上交（这正是 AUTOSWEEP 分类器 transient 规则"attack+独立 sustain"要面对的判定）。

### multiband（轴=band thresholds）——pass 10

- C4 M/S、C6 M/S、LinMB M/S、C6-SideChain M/S、Lindell MBC、Lindell 354E 全部 att band_dynamics+band_timing。
- **锚点形态注意**：认证写的是 band 级 gain/attack（C4 实证 control_ref 带 `band_key=band_1, role=gain, param=8 / role=attack, param=10`），而白名单锚形态是 `band_threshold_param_ids[3]`——band threshold 参数级锚定需 probe 派生（v5 Lindell MBC 已有先例锚）。族级覆盖全过，参数级在步 2/构建器解决。

### static_eq（轴=增益带）——**0**

PC 认证库无任何 static_eq 主体（24 job 无、36 promoted 无）。v5 的 bx_hybrid V2 不在晋升集（EQ-1 探测通道锚定，5 bands 条目已在 v5）。

### broadband_compression（轴=threshold）——promoted 0；另有 2 个认证 pass 未晋升

v5 的 Vertigo VSC-2 不在晋升集。C1 comp M/S（compressor 族）认证角色 threshold=7+attack=1、apply/restore exact——若决策侧裁定 compressor 族认证可映射 broadband_compression（AUTOSWEEP 分类器定义"broadband=threshold(+ratio/makeup) 单频段"与 C1 单频段压缩结构一致）且补晋升 import，则本族 +2 候选（锚点现成）。

## 5. 族计数预期表（AUTOSWEEP 首跑对表基准）

| 白名单族 | 严格候选（pass） | +歧义裁定后 | +决策项后上限 | v5 现状 | 备注 |
|---|---|---|---|---|---|
| static_eq | **0** | 0 | 0（+1 legacy） | 1（bx_hybrid，非 promoted） | PC 无 EQ 认证，legacy 保留待裁 |
| broadband_compression | **0** | 0 | 2（C1 comp M/S）+1 legacy | 1（Vertigo，非 promoted） | compressor→broadband 映射 + 补晋升待裁 |
| de_esser | **6** | 7（+Pro-DS） | 7 | 1（Pro-DS） | RDeEsser M/S 记因排除 |
| limiter | **6** | 6 | 6 | 1（Pro-L 2） | bx_limiter TP 记因排除 |
| gate_expander | **3** | 3 | 3 | 1（Pro-G） | C1 gate/comp-gate 记因排除 |
| transient_shaper | **2** | 5（+SPL/TransX M/S） | 5 | 1（SPL） | attack 轴角色边界待裁 |
| multiband | **10** | 10 | 10 | 1（Lindell MBC） | band threshold 参数锚 probe 派生 |
| **合计** | **27** | 31 | 33（+2 legacy=35） | 7 | 决策点 B：Mono/Stereo 全入列已裁，上表已按 M/S 分列计 |

Waves/非 Waves 拆分（promoted 36）：Waves 28 / 非 Waves 8（Pro-DS、Pro-G、Pro-L 2、SPL、Lindell MBC、Lindell 354E、HUM LAAL、bx_limiter TP）。

pass 主体中锚点待 probe（无本地 job 证据且无 v5 锚）：Sibilance-Live M/S、HUM LAAL、C6-SideChain M/S、Lindell 354E = **6 个**（先例 0.7–1.1s/主体）。

## 6. 上交决策点（步 1 不擅自裁定）

1. **EQ 预期落空（卡面前提偏差）**：卡面"预期 EQ≥2：bx_hybrid+其他"不成立——PC 无 static_eq 认证主体。选项 A：v6 保留 v5 legacy 条目（bx_hybrid，记因"非 PCA 派生、EQ-1 通道锚定"，白名单职能=锚定绑定非策展——推荐，否则 EQ 实验轴在 PC 断供）；选项 B：严格派生 EQ 空族（EQ 实验不可用）。**broadband 同型问题**（Vertigo）。
2. **compressor→broadband 映射 + C1 comp M/S 补晋升**：认证证据齐（threshold=7 锚点现成），补一条 pcactl import 即得 broadband 双候选；不裁则 broadband 候选=0。
3. **attack 轴角色边界**：transient_range（TransX）/envelope_emphasis（SPL）是否计入 attack 轴覆盖。影响 transient 候选 2→5。
4. **Pro-DS de_esser 轴判定**：attestation 无 threshold_sensitivity 但 v5 有 probe 锚 threshold=1。影响 de_esser 6→7。
5. multiband band threshold 参数级锚定需 probe（族级覆盖全过）——步 2 执行项非决策项，列出备查。

## 7. AUTOSWEEP 分类器校准输入（PC 实测事实）

- **轴命名双层映射（job 角色 ↔ attestation 轴名）**：threshold↔threshold_sensitivity；ceiling↔output_ceiling；range↔attenuation_floor；attack/attack_duration↔envelope_timing（近轴 envelope_emphasis）；band 增益/时基↔band_dynamics+band_timing。分类器规则落参数拓扑层时需同时对接两层命名。
- **族内角色实测样本**（参数拓扑判族的真实正例）：de_esser={detector_filter, threshold, reduction_range}；limiter={threshold, ceiling}；gate={sidechain_highpass, range}；transient={attack_duration|transient_range, output_gain}；multiband={band_key×(gain, attack)}；compressor={threshold, attack}。
- **语义索引桶计数**（991 全库，scan_plugins 2026-09-18）：eq=146 / dynamics=187 / distortion=84 / reverb=59 / delay=26 / modulation=44 / pitch=22 / instrument=22 / analyzer=22 / filter=11 / meter=4 / synth=4 / compressor=2 / limiter=2 / unknown=356——sweep 分类器的可寻址空间与 unknown 桶规模。
- **壳内主体 identifier 稳定性**：24/24 同壳（`695648d`）不同 hash 后缀，identifier 独占解析在 C2-PCR 已双端实证，本盘点点名核对无冲突。
- **记因排除模式**（sweep 例外队列将复现的形态）：族正确但轴未覆盖（RDeEsser 无 threshold 写、bx_limiter TP 无 ceiling 写、C1 gate 无 range 写）——分类器不能只看族，必须验轴参数的写往返证据。

## 8. 工件与溯源

- `step1_pc_pca_inventory.json`——36+2 主体全量行（身份/族/晋升/认证角色/参数锚点/att 轴/判定/v5/语义命中），`sources` 节含四源版本指针。
- `step1_parse.py`——解析脚本（可重跑复现 JSON）。
- 溯源链：每主体 → `pca_certifications/pca_job_<id>/summary.json` + `cases/*/evidence.json`（24 个）或 store evidence 收据 sha256 指针（14 个）→ attestation rev 53。
