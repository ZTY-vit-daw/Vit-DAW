# Waves EQ 控制拓扑普查与分类重构设计（第一阶段）

> 本报告来自真实 Godot → Kernel → Agent 只读参数链。没有插件参数写入、音频主动探测、learn/profile、SPAL、B4 或 Plugin Alliance 参数读取。

## 1. 普查结论摘要

- 目标型号：50
- 完整采集：50
- 明确失败：0
- 数据驱动簇：27
- 公开投影：fixed_slot_adjustable=42，fixed_freq=6，free_floating=0，unresolved=2
- 结构执行结论：full=6，partial_quantized=29，unsupported=15
- 原始产物：`D:\Vit_DAW\artifacts\waves_eq_topology_census\20260727_201627`
- 所有执行能力结论均为参数结构推断，未经写入烟测或音频行为验证。

## 2. 方法与边界

每个 Stereo 型号按 128 条一页循环读取 `plugin.get_parameters`，校验 total 稳定、offset 连续、参数 ID 唯一以及实际条数等于 total。`surface_signature` 保留精确参数表；`topology_signature` 删除插件身份后使用名称 token、分组、顺序、物理域、枚举、激活、通道和局部重复块。聚类使用平均链接层次聚类，不预设 Waves 产品族。

五点域采样只调用宿主 value-to-string 映射，不改变参数值，也不输入音频。Channel Strip 只从外围控制中提取局部 EQ 块。

## 3. 型号归属、能力与拒绝矩阵

| 型号 | 参数/页 | 公开分类 | 簇 | 完整块 | 能力 | 拒绝或限制 |
|---|---:|---|---|---:|---|---|
| Abbey Road EMI TG12345 Ch Stereo | 38/1 | fixed_freq | C01 | 1 | unsupported | incomplete gain-bearing EQ blocks remain |
| Abbey Road REDD.17 Stereo | 19/1 | fixed_freq | C03 | 0 | unsupported | fixed frequency unknown or frequency role incomplete; incomplete gain-bearing EQ blocks remain; no complete local EQ set-point block |
| Abbey Road REDD.37.51 Stereo | 23/1 | fixed_freq | C03 | 0 | unsupported | fixed frequency unknown or frequency role incomplete; incomplete gain-bearing EQ blocks remain; no complete local EQ set-point block |
| Abbey Road RS56 Passive EQ Stereo | 30/1 | fixed_slot_adjustable | C04 | 3 | partial_quantized | one or more requested roles are quantized |
| Abbey Road The King's Microphones Stereo | 5/1 | unresolved | C02 | 0 | unsupported | no complete local EQ set-point block |
| API-550A Stereo | 16/1 | fixed_slot_adjustable | C05 | 3 | partial_quantized | one or more requested roles are quantized |
| API-550B Stereo | 17/1 | fixed_slot_adjustable | C05 | 4 | partial_quantized | one or more requested roles are quantized |
| API-560 Stereo | 17/1 | fixed_freq | C06 | 10 | full | 无结构性拒绝；仍需后续写入验证 |
| AudioTrack Stereo | 35/1 | fixed_slot_adjustable | C07 | 4 | partial_quantized | one or more requested roles are quantized |
| CLA MixHub Lite Stereo | 1430/12 | fixed_slot_adjustable | C08 | 8 | partial_quantized | one or more requested roles are quantized |
| CLA MixHub Stereo | 1430/12 | fixed_slot_adjustable | C08 | 8 | partial_quantized | one or more requested roles are quantized |
| Curves AQ Live Stereo | 103/1 | fixed_slot_adjustable | C09 | 12 | unsupported | adaptive/stateful/capture topology; channel asymmetry; incomplete gain-bearing EQ blocks remain |
| Curves AQ Stereo | 105/1 | fixed_slot_adjustable | C09 | 12 | unsupported | adaptive/stateful/capture topology; channel asymmetry; incomplete gain-bearing EQ blocks remain |
| EMO-F2 Stereo | 12/1 | fixed_slot_adjustable | C10 | 0 | unsupported | no complete local EQ set-point block |
| EMO-Q4 Stereo | 49/1 | fixed_slot_adjustable | C07 | 4 | partial_quantized | one or more requested roles are quantized |
| F6 Stereo | 94/1 | fixed_slot_adjustable | C11 | 6 | partial_quantized | one or more requested roles are quantized |
| F6-RTA Stereo | 95/1 | fixed_slot_adjustable | C11 | 6 | partial_quantized | one or more requested roles are quantized |
| GEQ Classic Stereo | 83/1 | fixed_freq | C12 | 31 | full | 无结构性拒绝；仍需后续写入验证 |
| GEQ Modern Stereo | 82/1 | fixed_freq | C12 | 31 | full | 无结构性拒绝；仍需后续写入验证 |
| H-EQ Stereo | 61/1 | fixed_slot_adjustable | C13 | 5 | partial_quantized | one or more requested roles are quantized |
| H-EQ-Light Stereo | 51/1 | fixed_slot_adjustable | C13 | 5 | partial_quantized | one or more requested roles are quantized |
| Kramer HLS Stereo | 19/1 | fixed_slot_adjustable | C14 | 2 | unsupported | incomplete gain-bearing EQ blocks remain; one or more requested roles are quantized; one-sided or unknown gain law cannot satisfy arbitrary set-point gain |
| LinEQ Broadband Stereo | 38/1 | fixed_slot_adjustable | C13 | 6 | partial_quantized | one or more requested roles are quantized |
| LinEQ Lowband Stereo | 23/1 | fixed_slot_adjustable | C15 | 3 | partial_quantized | one or more requested roles are quantized |
| Magma Channel Strip Stereo | 16/1 | fixed_slot_adjustable | C16 | 1 | unsupported | incomplete gain-bearing EQ blocks remain |
| MannyM EQ Stereo | 21/1 | fixed_slot_adjustable | C17 | 4 | partial_quantized | one or more requested roles are quantized |
| PuigTec EQP1A Stereo | 14/1 | fixed_slot_adjustable | C18 | 0 | unsupported | coupled boost/attenuate topology; incomplete gain-bearing EQ blocks remain; no complete local EQ set-point block |
| PuigTec MEQ5 Stereo | 12/1 | fixed_slot_adjustable | C05 | 3 | unsupported | one or more requested roles are quantized; one-sided or unknown gain law cannot satisfy arbitrary set-point gain |
| Q-Clone Stereo | 8/1 | unresolved | C02 | 0 | unsupported | adaptive/stateful/capture topology; no complete local EQ set-point block |
| Q1 Stereo | 15/1 | fixed_slot_adjustable | C20 | 1 | partial_quantized | one or more requested roles are quantized |
| Q10 Stereo | 60/1 | fixed_slot_adjustable | C19 | 10 | partial_quantized | one or more requested roles are quantized |
| Q2 Stereo | 20/1 | fixed_slot_adjustable | C21 | 2 | partial_quantized | one or more requested roles are quantized |
| Q3 Stereo | 25/1 | fixed_slot_adjustable | C15 | 3 | partial_quantized | one or more requested roles are quantized |
| Q4 Stereo | 30/1 | fixed_slot_adjustable | C07 | 4 | partial_quantized | one or more requested roles are quantized |
| Q6 Stereo | 40/1 | fixed_slot_adjustable | C13 | 6 | partial_quantized | one or more requested roles are quantized |
| Q8 Stereo | 50/1 | fixed_slot_adjustable | C19 | 8 | partial_quantized | one or more requested roles are quantized |
| RChannel Stereo | 54/1 | fixed_slot_adjustable | C07 | 4 | partial_quantized | one or more requested roles are quantized |
| REQ 2 Stereo | 17/1 | fixed_slot_adjustable | C21 | 2 | partial_quantized | one or more requested roles are quantized |
| REQ 4 Stereo | 29/1 | fixed_slot_adjustable | C07 | 4 | partial_quantized | one or more requested roles are quantized |
| REQ 6 Stereo | 41/1 | fixed_slot_adjustable | C13 | 6 | partial_quantized | one or more requested roles are quantized |
| Scheps 73 Stereo | 34/1 | fixed_slot_adjustable | C22 | 2 | unsupported | incomplete gain-bearing EQ blocks remain; one or more requested roles are quantized |
| Scheps Omni Channel 2 Stereo | 1819/15 | fixed_slot_adjustable | C23 | 4 | partial_quantized | one or more requested roles are quantized |
| SSL EV2 Channel Stereo | 46/1 | fixed_slot_adjustable | C24 | 4 | full | 无结构性拒绝；仍需后续写入验证 |
| SSLChannel Stereo | 36/1 | fixed_slot_adjustable | C24 | 4 | partial_quantized | one or more requested roles are quantized |
| SSLEQ Stereo | 21/1 | fixed_slot_adjustable | C25 | 4 | full | 无结构性拒绝；仍需后续写入验证 |
| SSLGChannel Stereo | 36/1 | fixed_slot_adjustable | C24 | 4 | full | 无结构性拒绝；仍需后续写入验证 |
| TRACT LinPhase Stereo | 117/1 | fixed_slot_adjustable | C26 | 8 | unsupported | adaptive/stateful/capture topology; incomplete gain-bearing EQ blocks remain |
| TRACT Stereo | 117/1 | fixed_slot_adjustable | C26 | 8 | unsupported | adaptive/stateful/capture topology; incomplete gain-bearing EQ blocks remain |
| VEQ3 Stereo | 14/1 | fixed_slot_adjustable | C05 | 3 | partial_quantized | one or more requested roles are quantized |
| VEQ4 Stereo | 21/1 | fixed_slot_adjustable | C27 | 4 | partial_quantized | one or more requested roles are quantized |

## 4. 数据驱动聚类

### C01

- 成员：Abbey Road EMI TG12345 Ch Stereo
- 主投影：fixed_freq
- 参数数范围：[38, 38]
- EQ 块数范围：[3, 3]
- 频率模式：opaque, writable_continuous
- 激活模式：always_active_or_unexposed
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C02

- 成员：Abbey Road The King's Microphones Stereo, Q-Clone Stereo
- 主投影：unresolved
- 参数数范围：[5, 8]
- EQ 块数范围：[0, 0]
- 频率模式：none
- 激活模式：none
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C03

- 成员：Abbey Road REDD.17 Stereo, Abbey Road REDD.37.51 Stereo
- 主投影：fixed_freq
- 参数数范围：[19, 23]
- EQ 块数范围：[2, 2]
- 频率模式：opaque
- 激活模式：always_active_or_unexposed
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C04

- 成员：Abbey Road RS56 Passive EQ Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[30, 30]
- EQ 块数范围：[3, 3]
- 频率模式：writable_discrete
- 激活模式：explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C05

- 成员：API-550A Stereo, API-550B Stereo, PuigTec MEQ5 Stereo, VEQ3 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[12, 17]
- EQ 块数范围：[3, 4]
- 频率模式：writable_discrete
- 激活模式：always_active_or_unexposed
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C06

- 成员：API-560 Stereo
- 主投影：fixed_freq
- 参数数范围：[17, 17]
- EQ 块数范围：[10, 10]
- 频率模式：fixed_label
- 激活模式：always_active_or_unexposed
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C07

- 成员：AudioTrack Stereo, EMO-Q4 Stereo, Q4 Stereo, RChannel Stereo, REQ 4 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[29, 54]
- EQ 块数范围：[4, 4]
- 频率模式：writable_continuous
- 激活模式：explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C08

- 成员：CLA MixHub Lite Stereo, CLA MixHub Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[1430, 1430]
- EQ 块数范围：[12, 12]
- 频率模式：writable_continuous
- 激活模式：always_active_or_unexposed, explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C09

- 成员：Curves AQ Live Stereo, Curves AQ Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[103, 105]
- EQ 块数范围：[15, 15]
- 频率模式：writable_continuous
- 激活模式：always_active_or_unexposed, explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C10

- 成员：EMO-F2 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[12, 12]
- EQ 块数范围：[2, 2]
- 频率模式：writable_continuous
- 激活模式：explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C11

- 成员：F6-RTA Stereo, F6 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[94, 95]
- EQ 块数范围：[8, 8]
- 频率模式：writable_continuous
- 激活模式：explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C12

- 成员：GEQ Classic Stereo, GEQ Modern Stereo
- 主投影：fixed_freq
- 参数数范围：[82, 83]
- EQ 块数范围：[31, 31]
- 频率模式：fixed_label, writable_continuous
- 激活模式：always_active_or_unexposed, explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C13

- 成员：H-EQ-Light Stereo, H-EQ Stereo, LinEQ Broadband Stereo, Q6 Stereo, REQ 6 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[38, 61]
- EQ 块数范围：[6, 7]
- 频率模式：writable_continuous
- 激活模式：explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C14

- 成员：Kramer HLS Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[19, 19]
- EQ 块数范围：[3, 3]
- 频率模式：opaque, writable_discrete
- 激活模式：always_active_or_unexposed
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C15

- 成员：LinEQ Lowband Stereo, Q3 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[23, 25]
- EQ 块数范围：[3, 3]
- 频率模式：writable_continuous
- 激活模式：explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C16

- 成员：Magma Channel Strip Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[16, 16]
- EQ 块数范围：[4, 4]
- 频率模式：opaque, writable_continuous, writable_discrete
- 激活模式：always_active_or_unexposed
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C17

- 成员：MannyM EQ Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[21, 21]
- EQ 块数范围：[6, 6]
- 频率模式：writable_continuous, writable_discrete
- 激活模式：always_active_or_unexposed, explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C18

- 成员：PuigTec EQP1A Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[14, 14]
- EQ 块数范围：[2, 2]
- 频率模式：writable_discrete
- 激活模式：always_active_or_unexposed
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C19

- 成员：Q10 Stereo, Q8 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[50, 60]
- EQ 块数范围：[8, 10]
- 频率模式：writable_continuous
- 激活模式：explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C20

- 成员：Q1 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[15, 15]
- EQ 块数范围：[1, 1]
- 频率模式：writable_continuous
- 激活模式：explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C21

- 成员：Q2 Stereo, REQ 2 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[17, 20]
- EQ 块数范围：[2, 2]
- 频率模式：writable_continuous
- 激活模式：explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C22

- 成员：Scheps 73 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[34, 34]
- EQ 块数范围：[4, 4]
- 频率模式：opaque, writable_discrete
- 激活模式：explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C23

- 成员：Scheps Omni Channel 2 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[1819, 1819]
- EQ 块数范围：[8, 8]
- 频率模式：writable_continuous
- 激活模式：always_active_or_unexposed, explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C24

- 成员：SSL EV2 Channel Stereo, SSLChannel Stereo, SSLGChannel Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[36, 46]
- EQ 块数范围：[6, 6]
- 频率模式：writable_continuous
- 激活模式：always_active_or_unexposed, explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C25

- 成员：SSLEQ Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[21, 21]
- EQ 块数范围：[5, 5]
- 频率模式：writable_continuous
- 激活模式：always_active_or_unexposed, explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C26

- 成员：TRACT LinPhase Stereo, TRACT Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[117, 117]
- EQ 块数范围：[9, 9]
- 频率模式：opaque, writable_continuous
- 激活模式：always_active_or_unexposed, explicit_binding
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

### C27

- 成员：VEQ4 Stereo
- 主投影：fixed_slot_adjustable
- 参数数范围：[21, 21]
- EQ 块数范围：[6, 6]
- 频率模式：writable_discrete
- 激活模式：always_active_or_unexposed
- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。

## 5. 各型号拓扑证据详表

### Abbey Road EMI TG12345 Ch Stereo

- identifier：`VST3-Abbey Road EMI TG12345 Ch Stereo-695648d-cd7ad640`
- 参数/分页：38 条，1 页，complete=True
- surface signature：`149b14a359635603573a8b77ed5fe0f3afac220fa71a3dd756d4b90720a8c45b`
- topology signature：`1969e3457931f2ae6f37a7af3407ad3919ad3f7ebe78dc64f94198de74e08668`
- 公开投影/簇：`fixed_freq` / `C01`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：incomplete gain-bearing EQ blocks remain
- 全局控制：activation=0，bypass=3，unknown_toggles=4
- 局部 section/band：
  - `treble`：placement=opaque；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=frequency anchor absent
  - `presence`：placement=writable_continuous；roles=[frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `bass`：placement=opaque；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=frequency anchor absent

### Abbey Road REDD.17 Stereo

- identifier：`VST3-Abbey Road REDD.17 Stereo-695648d-4ee746c3`
- 参数/分页：19 条，1 页，complete=True
- surface signature：`6ba0e037f81d3cdcdc921533f2e767aff7d94f1f4ab0730691099eba4036d65b`
- topology signature：`c54e5df4c01883c944dc3049556029c25356cb43b1defe2073dfcc9fc1956143`
- 公开投影/簇：`fixed_freq` / `C03`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": false, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：fixed frequency unknown or frequency role incomplete; incomplete gain-bearing EQ blocks remain; no complete local EQ set-point block
- 全局控制：activation=0，bypass=1，unknown_toggles=2
- 局部 section/band：
  - `tone low`：placement=opaque；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=frequency anchor absent
  - `tone high`：placement=opaque；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=frequency anchor absent

### Abbey Road REDD.37.51 Stereo

- identifier：`VST3-Abbey Road REDD.37.51 Stereo-695648d-8c2708be`
- 参数/分页：23 条，1 页，complete=True
- surface signature：`01a4146ce4424a2ee6c3f9108169f5d96c46a44b26c0235191536c9bad3d9be7`
- topology signature：`ac06c7bd706fc4dbf4fa816dce5c842274b2fb8090523892d620c6b4bca7ae82`
- 公开投影/簇：`fixed_freq` / `C03`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": false, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：fixed frequency unknown or frequency role incomplete; incomplete gain-bearing EQ blocks remain; no complete local EQ set-point block
- 全局控制：activation=0，bypass=1，unknown_toggles=2
- 局部 section/band：
  - `tone low`：placement=opaque；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=frequency anchor absent
  - `tone high`：placement=opaque；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=frequency anchor absent

### Abbey Road RS56 Passive EQ Stereo

- identifier：`VST3-Abbey Road RS56 Passive EQ Stereo-695648d-f2f6a4b2`
- 参数/分页：30 条，1 页，complete=True
- surface signature：`c686295ce2b9e6cc112de2fc568fea235bc664c870faf7979cf01f8ebf5c1a4f`
- topology signature：`56e1270b42d382a0d7caede263394ce084877cf22250ef73e3a140ace6350ec0`
- 公开投影/簇：`fixed_slot_adjustable` / `C04`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": false, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=2
- 局部 section/band：
  - `top`：placement=writable_discrete；roles=[activation×1, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `treble`：placement=writable_discrete；roles=[activation×1, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `bass`：placement=writable_discrete；roles=[activation×1, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none

### Abbey Road The King's Microphones Stereo

- identifier：`VST3-Abbey Road The King's Microphones Stereo-695648d-8615f608`
- 参数/分页：5 条，1 页，complete=True
- surface signature：`7112cd5811136f93456d3ed69c47234cd5334f0705fcc836375b5194460432de`
- topology signature：`bf9d4f7322125b7e761f1415b48791f80907a164032748020a25ffc2b071abe1`
- 公开投影/簇：`unresolved` / `C02`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": false, "gain": false, "q": false, "slope": false}`
- 拒绝或限制：no complete local EQ set-point block
- 全局控制：activation=0，bypass=1，unknown_toggles=0
- 局部 section/band：未从静态参数面恢复；归入待解决特殊结构并拒绝执行。

### API-550A Stereo

- identifier：`VST3-API-550A Stereo-695648d-874091cb`
- 参数/分页：16 条，1 页，complete=True
- surface signature：`79b1ab2a424a3eb3f7bafec8d920251bc4a5a34aecf4c0b3b50b5fef46af328c`
- topology signature：`b83f5e1cbbec915d2cc482b79afa76c742bfa30ebc125dd553e87ee6d8fe02f9`
- 公开投影/簇：`fixed_slot_adjustable` / `C05`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": true, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=1，bypass=1，unknown_toggles=2
- 局部 section/band：
  - `high`：placement=writable_discrete；roles=[filter_kind×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, filter_kind=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `mid`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `low`：placement=writable_discrete；roles=[filter_kind×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, filter_kind=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### API-550B Stereo

- identifier：`VST3-API-550B Stereo-695648d-8b0192cb`
- 参数/分页：17 条，1 页，complete=True
- surface signature：`d0d7adfee903ea628b33f9ebb127ac24d55c707055b8fcea8daf8f0942c65a08`
- topology signature：`c1bf51ade87c5837d79dfbc9afc7155fea42454b1fe2d83e773e022c32e5082f`
- 公开投影/簇：`fixed_slot_adjustable` / `C05`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": true, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=1，bypass=1，unknown_toggles=2
- 局部 section/band：
  - `high`：placement=writable_discrete；roles=[filter_kind×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, filter_kind=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high mid`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `low mid`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `low`：placement=writable_discrete；roles=[filter_kind×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, filter_kind=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### API-560 Stereo

- identifier：`VST3-API-560 Stereo-695648d-6b34511e`
- 参数/分页：17 条，1 页，complete=True
- surface signature：`b1c7595cd04c6b5f06942696a32918c979b18695abdf2adc0f6b531347af5bbd`
- topology signature：`881d0848f82526cc0291a603acd45d42887f11e8c7ec5b0eba303bc79042c53c`
- 公开投影/簇：`fixed_freq` / `C06`
- 执行能力：`full`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": false, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：无结构性拒绝；仍未进行写入验证
- 全局控制：activation=1，bypass=1，unknown_toggles=2
- 局部 section/band：
  - `band 16`：placement=fixed_label；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `band 8`：placement=fixed_label；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=fixed_label；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=fixed_label；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `band 1`：placement=fixed_label；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `band 500`：placement=fixed_label；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `band 250`：placement=fixed_label；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `band 125`：placement=fixed_label；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `band 63`：placement=fixed_label；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `band 31`：placement=fixed_label；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### AudioTrack Stereo

- identifier：`VST3-AudioTrack Stereo-695648d-bff70bc2`
- 参数/分页：35 条，1 页，complete=True
- surface signature：`7200fbecaccc626af471d3a7e8fd09cb7152597a71f139d1b2db8f0cdf6e22d2`
- topology signature：`1da7bc512ffd009853155a6ed167523c67c1a6a6b4923993a1a6e93ba67e43bb`
- 公开投影/簇：`fixed_slot_adjustable` / `C07`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=2，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### CLA MixHub Lite Stereo

- identifier：`VST3-CLA MixHub Lite Stereo-695648d-7fdafd28`
- 参数/分页：1430 条，12 页，complete=True
- surface signature：`abdcb19fa270e9862aec9b61f1287113e4c46b122e81e6cf98fb6108b9b60825`
- topology signature：`fa1a4398582bbfca6c80e40e72df052eacea5aa4c76a1f6ae46967a9d9598e53`
- 公开投影/簇：`fixed_slot_adjustable` / `C08`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=19
- 局部 section/band：
  - `pre lp`：placement=writable_continuous；roles=[activation×2, frequency×2]；channels=[frequency=left+right, activation=left+right]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `pre hp`：placement=writable_continuous；roles=[activation×2, frequency×2]；channels=[frequency=left+right, activation=left+right]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `eq low`：placement=writable_continuous；roles=[frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `eq low mid`：placement=writable_continuous；roles=[frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `eq high mid`：placement=writable_continuous；roles=[frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `eq high`：placement=writable_continuous；roles=[frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `dyn sc hp`：placement=writable_continuous；roles=[frequency×2]；channels=[frequency=left+right]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `dyn sc lp`：placement=writable_continuous；roles=[frequency×2]；channels=[frequency=left+right]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `dyn sc low`：placement=writable_continuous；roles=[filter_kind×2, frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right, filter_kind=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `dyn sc low mid`：placement=writable_continuous；roles=[frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `dyn sc high mid`：placement=writable_continuous；roles=[frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `dyn sc high`：placement=writable_continuous；roles=[filter_kind×2, frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right, filter_kind=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### CLA MixHub Stereo

- identifier：`VST3-CLA MixHub Stereo-695648d-d921fd28`
- 参数/分页：1430 条，12 页，complete=True
- surface signature：`abdcb19fa270e9862aec9b61f1287113e4c46b122e81e6cf98fb6108b9b60825`
- topology signature：`fa1a4398582bbfca6c80e40e72df052eacea5aa4c76a1f6ae46967a9d9598e53`
- 公开投影/簇：`fixed_slot_adjustable` / `C08`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=19
- 局部 section/band：
  - `pre lp`：placement=writable_continuous；roles=[activation×2, frequency×2]；channels=[frequency=left+right, activation=left+right]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `pre hp`：placement=writable_continuous；roles=[activation×2, frequency×2]；channels=[frequency=left+right, activation=left+right]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `eq low`：placement=writable_continuous；roles=[frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `eq low mid`：placement=writable_continuous；roles=[frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `eq high mid`：placement=writable_continuous；roles=[frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `eq high`：placement=writable_continuous；roles=[frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `dyn sc hp`：placement=writable_continuous；roles=[frequency×2]；channels=[frequency=left+right]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `dyn sc lp`：placement=writable_continuous；roles=[frequency×2]；channels=[frequency=left+right]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `dyn sc low`：placement=writable_continuous；roles=[filter_kind×2, frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right, filter_kind=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `dyn sc low mid`：placement=writable_continuous；roles=[frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `dyn sc high mid`：placement=writable_continuous；roles=[frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `dyn sc high`：placement=writable_continuous；roles=[filter_kind×2, frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right, filter_kind=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### Curves AQ Live Stereo

- identifier：`VST3-Curves AQ Live Stereo-695648d-a69f1966`
- 参数/分页：103 条，1 页，complete=True
- surface signature：`a049521c2c97c5e07ffce56925d0cde96fd64d0de38abea8d1a79f173f55c5f8`
- topology signature：`d5d8616eeaedaa2b0d1ee008ab8a0fc24e09bb3f48dbd02fa37234c937dc80b1`
- 公开投影/簇：`fixed_slot_adjustable` / `C09`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：adaptive/stateful/capture topology; channel asymmetry; incomplete gain-bearing EQ blocks remain
- 全局控制：activation=0，bypass=2，unknown_toggles=13
- 局部 section/band：
  - `tilt`：placement=writable_continuous；roles=[filter_kind×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, filter_kind=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `hpf`：placement=writable_continuous；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `lpf`：placement=writable_continuous；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `node 1`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 2`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 3`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 4`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 5`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 6`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 7`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 8`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `anchor 1`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=left, gain=left]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=channel asymmetry
  - `anchor 2 f`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `anchor 3 h`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `anchor 4 a`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### Curves AQ Stereo

- identifier：`VST3-Curves AQ Stereo-695648d-84d61966`
- 参数/分页：105 条，1 页，complete=True
- surface signature：`0389fe817062302c7d3876ad239821db19fd476c25f69db98b0c6cbfb5b340d1`
- topology signature：`fafb97efddb036b50c8cd23b4726910ff34ca79b49face221243b62bb28cc64a`
- 公开投影/簇：`fixed_slot_adjustable` / `C09`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：adaptive/stateful/capture topology; channel asymmetry; incomplete gain-bearing EQ blocks remain
- 全局控制：activation=0，bypass=2，unknown_toggles=13
- 局部 section/band：
  - `tilt`：placement=writable_continuous；roles=[filter_kind×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, filter_kind=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `hpf`：placement=writable_continuous；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `lpf`：placement=writable_continuous；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `node 1`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 2`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 3`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 4`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 5`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 6`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 7`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `node 8`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `anchor 1`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=left, gain=left]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=channel asymmetry
  - `anchor 2 f`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `anchor 3 h`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `anchor 4 a`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### EMO-F2 Stereo

- identifier：`VST3-EMO-F2 Stereo-695648d-2236a1d0`
- 参数/分页：12 条，1 页，complete=True
- surface signature：`779bfa1d432076c677649552584e9a475a8ba67b05a262774617283ee0fc61e7`
- topology signature：`9ea321fbdd5168e980e08c5829d5f2e629322c562edadc41cc4f2f19cdb47d8c`
- 公开投影/簇：`fixed_slot_adjustable` / `C10`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": false, "frequency": true, "gain": false, "q": false, "slope": false}`
- 拒绝或限制：no complete local EQ set-point block
- 全局控制：activation=0，bypass=1，unknown_toggles=1
- 局部 section/band：
  - `hpf`：placement=writable_continuous；roles=[activation×2, frequency×2]；channels=[frequency=left+right, activation=left+right]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `lpf`：placement=writable_continuous；roles=[activation×2, frequency×2]；channels=[frequency=left+right, activation=left+right]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent

### EMO-Q4 Stereo

- identifier：`VST3-EMO-Q4 Stereo-695648d-15232edb`
- 参数/分页：49 条，1 页，complete=True
- surface signature：`29c1935e9bcebd69dfc6aa5bba083b0c1e1b0b1c54fbe0b5fd5efd95e467167d`
- topology signature：`a3769261d5d1c36d3d6ab887242f459210f7cc1b8e634f4fb0527db30c88ca9d`
- 公开投影/簇：`fixed_slot_adjustable` / `C07`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=1
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×2, filter_kind×2, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=left+right, filter_kind=left+right]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×2, filter_kind×2, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=left+right, filter_kind=left+right]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×2, filter_kind×2, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=left+right, filter_kind=left+right]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×2, filter_kind×2, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=left+right, filter_kind=left+right]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none

### F6 Stereo

- identifier：`VST3-F6 Stereo-695648d-caaf7070`
- 参数/分页：94 条，1 页，complete=True
- surface signature：`d3f62165bf3b0138984557195a20728687270fd56644ce96da0a1db5e4bdf93c`
- topology signature：`6b0c9302fb162052a321b77f7d8601266d2baf6fc753b43a395d37a344e302cc`
- 公开投影/簇：`fixed_slot_adjustable` / `C11`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 重复轴：period=13，repetitions=6，addressed=True，addressed repeated band/section axis
- 全局控制：activation=1，bypass=1，unknown_toggles=8
- 局部 section/band：
  - `hpf`：placement=writable_continuous；roles=[activation×1, frequency×1, q×1]；channels=[frequency=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `lpf`：placement=writable_continuous；roles=[activation×1, frequency×1, q×1]；channels=[frequency=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `band 1`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 5`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 6`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none

### F6-RTA Stereo

- identifier：`VST3-F6-RTA Stereo-695648d-52d55f75`
- 参数/分页：95 条，1 页，complete=True
- surface signature：`0bd6186dea6c365082709b37d754e762e97d707c226e18d8f648fc52152316e5`
- topology signature：`aee8f8cae42cc0547a9a50c34ee96f0c4a282b4708212ce573b48b338985e8f1`
- 公开投影/簇：`fixed_slot_adjustable` / `C11`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 重复轴：period=13，repetitions=6，addressed=True，addressed repeated band/section axis
- 全局控制：activation=1，bypass=1，unknown_toggles=8
- 局部 section/band：
  - `hpf`：placement=writable_continuous；roles=[activation×1, frequency×1, q×1]；channels=[frequency=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `lpf`：placement=writable_continuous；roles=[activation×1, frequency×1, q×1]；channels=[frequency=shared, q=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `band 1`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 5`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 6`：placement=writable_continuous；roles=[activation×1, dynamic_peripheral×4, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none

### GEQ Classic Stereo

- identifier：`VST3-GEQ Classic Stereo-695648d-b376e770`
- 参数/分页：83 条，1 页，complete=True
- surface signature：`0a043bf66185f037a581ea87c1f8f66d0e23058a10db225b94c6cf119bca9154`
- topology signature：`468e6e1c8cbab530ccd7ad01e530dd13d305d29915ad0436c83056403430a78f`
- 公开投影/簇：`fixed_freq` / `C12`
- 执行能力：`full`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": false, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：无结构性拒绝；仍未进行写入验证
- 全局控制：activation=0，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `25`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `31`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `40`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `50`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `63`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `80`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `100`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `125`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `160`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `200`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `250`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `315`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `400`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `500`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `630`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `800`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `1`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `1.25`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `1.6`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `2`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `2.5`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `3.15`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `4`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `5`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `6.3`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `8`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `10`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `12.5`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `16`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `20`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `bell`：placement=writable_continuous；roles=[activation×2, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=left+right]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### GEQ Modern Stereo

- identifier：`VST3-GEQ Modern Stereo-695648d-fc834f19`
- 参数/分页：82 条，1 页，complete=True
- surface signature：`0eba71c85bac1a6e82b28c684d1e684387aa720261d3ee047b632945ccffd1cb`
- topology signature：`2bff000efd01e59f48f93334f5d77afe651bcce43514d4acf9fe12d0eaaaac14`
- 公开投影/簇：`fixed_freq` / `C12`
- 执行能力：`full`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": false, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：无结构性拒绝；仍未进行写入验证
- 全局控制：activation=0，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `25`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `31`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `40`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `50`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `63`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `80`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `100`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `125`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `160`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `200`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `250`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `315`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `400`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `500`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `630`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `800`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `1`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `1.25`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `1.6`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `2`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `2.5`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `3.15`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `4`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `5`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `6.3`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `8`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `10`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `12.5`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `16`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `20`：placement=fixed_label；roles=[gain×2]；channels=[gain=left+right]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `bell`：placement=writable_continuous；roles=[activation×2, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=left+right]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### H-EQ Stereo

- identifier：`VST3-H-EQ Stereo-695648d-f4241872`
- 参数/分页：61 条，1 页，complete=True
- surface signature：`ee29ff9c1069395c3d0f466025115b63cdcb9c8cfe43747bf15640d3659ad734`
- topology signature：`a12a8e9384e2606db169ce4a43c09eb25f0aced71858a887496c6f525a243e0e`
- 公开投影/簇：`fixed_slot_adjustable` / `C13`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=11
- 局部 section/band：
  - `hp`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, q×1]；channels=[frequency=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `low`：placement=writable_continuous；roles=[activation×1, filter_kind×2, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `low mid`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `mf`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `high mid`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `high`：placement=writable_continuous；roles=[activation×1, filter_kind×2, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `lp`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, q×1]；channels=[frequency=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent

### H-EQ-Light Stereo

- identifier：`VST3-H-EQ-Light Stereo-695648d-e652bede`
- 参数/分页：51 条，1 页，complete=True
- surface signature：`f559eaed571a204cf4cf4a7b10b90c960c8ee7420c29bf80c6a777c448d9c79a`
- topology signature：`bb623bf447e70a390a2269f7c9d75e058bc91ec86c581694c4ea45e7a3baa2a0`
- 公开投影/簇：`fixed_slot_adjustable` / `C13`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=8
- 局部 section/band：
  - `hp`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, q×1]；channels=[frequency=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `low`：placement=writable_continuous；roles=[activation×1, filter_kind×2, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `low mid`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `mf`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `high mid`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `high`：placement=writable_continuous；roles=[activation×1, filter_kind×2, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `lp`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, q×1]；channels=[frequency=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent

### Kramer HLS Stereo

- identifier：`VST3-Kramer HLS Stereo-695648d-3a8eb30a`
- 参数/分页：19 条，1 页，complete=True
- surface signature：`c938fed46136be93acecc6f46602f3c6338d795eceafb6966f30d568015d2c8f`
- topology signature：`9e5cf46b7c38eaa00b0c3ea7ed6f65a51045a7e1d81975a2146ad4988409e5df`
- 公开投影/簇：`fixed_slot_adjustable` / `C14`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：incomplete gain-bearing EQ blocks remain; one or more requested roles are quantized; one-sided or unknown gain law cannot satisfy arbitrary set-point gain
- 全局控制：activation=0，bypass=1，unknown_toggles=1
- 局部 section/band：
  - `high`：placement=opaque；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=frequency anchor absent
  - `mid`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=unknown_or_one_sided；complete=True；issues=none
  - `lo`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=boost_only；complete=True；issues=none

### LinEQ Broadband Stereo

- identifier：`VST3-LinEQ Broadband Stereo-695648d-b6199bbe`
- 参数/分页：38 条，1 页，complete=True
- surface signature：`cbe17ec4e3162279c0903751a324b41e82aadd22fa4132a929915cd6b0f6da02`
- topology signature：`4f09bad98dd8b61dca254347c9e67c055392a00db8798fcbe98a30d3df9b30b9`
- 公开投影/簇：`fixed_slot_adjustable` / `C13`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 5`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 6`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none

### LinEQ Lowband Stereo

- identifier：`VST3-LinEQ Lowband Stereo-695648d-f9b598c6`
- 参数/分页：23 条，1 页，complete=True
- surface signature：`aa832702b7e36cc829a2c0ebe7b18f17633d78718a508fbfb36b6f6d6b3c9974`
- topology signature：`b16310c01f3306f4c07bca95cf19f1777e3575bc23ebfd6ec32c4858ef01ea69`
- 公开投影/簇：`fixed_slot_adjustable` / `C15`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none

### Magma Channel Strip Stereo

- identifier：`VST3-Magma Channel Strip Stereo-695648d-72d4ffe0`
- 参数/分页：16 条，1 页，complete=True
- surface signature：`6af507e002568b4de9b1cbac956c7f52be64ff239baf8dde57e068dd02f4507b`
- topology signature：`43003e24aaae72865227c72c51cdee25d45ff75d48c32912a5aa7758c28110dc`
- 公开投影/簇：`fixed_slot_adjustable` / `C16`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：incomplete gain-bearing EQ blocks remain
- 全局控制：activation=0，bypass=1，unknown_toggles=2
- 局部 section/band：
  - `bass`：placement=opaque；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=frequency anchor absent
  - `mid`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high`：placement=opaque；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=False；issues=frequency anchor absent
  - `hp`：placement=writable_discrete；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent

### MannyM EQ Stereo

- identifier：`VST3-MannyM EQ Stereo-695648d-15dd1ed8`
- 参数/分页：21 条，1 页，complete=True
- surface signature：`896a05a8eefd4631c9ada64f661dfe12e4c19f2ef8477a688002297e49578b8a`
- topology signature：`c887366b22d346d5433d4dae72529df955d37894fd6cd3ea98a2caf35d727d86`
- 公开投影/簇：`fixed_slot_adjustable` / `C17`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=1
- 局部 section/band：
  - `band 1`：placement=writable_discrete；roles=[activation×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, activation=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_discrete；roles=[activation×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, activation=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_discrete；roles=[activation×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, activation=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_discrete；roles=[activation×1, filter_kind×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `high pass`：placement=writable_continuous；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `low pass`：placement=writable_continuous；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent

### PuigTec EQP1A Stereo

- identifier：`VST3-PuigTec EQP1A Stereo-695648d-4e816891`
- 参数/分页：14 条，1 页，complete=True
- surface signature：`b8d27793c57cbbd3ab068df2be740e5ee8820b15287de6e4cf2e09098084b3a1`
- topology signature：`6a7e8a7e593854744b27a84ecf5a47ade964f922330e6bc8352798b02ac22cd7`
- 公开投影/簇：`fixed_slot_adjustable` / `C18`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：coupled boost/attenuate topology; incomplete gain-bearing EQ blocks remain; no complete local EQ set-point block
- 全局控制：activation=1，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `low`：placement=writable_discrete；roles=[frequency×1, gain×2]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=split_boost_attenuate；complete=False；issues=coupled boost/attenuate topology
  - `hi`：placement=writable_discrete；roles=[frequency×1, gain×2]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=split_boost_attenuate；complete=False；issues=coupled boost/attenuate topology

### PuigTec MEQ5 Stereo

- identifier：`VST3-PuigTec MEQ5 Stereo-695648d-3d756899`
- 参数/分页：12 条，1 页，complete=True
- surface signature：`d0dd1a83935b381a314d6793489562ef1d62a7caad37b93dd2536a385aaa669d`
- topology signature：`6cfd5868b08cf9619a1f04ee8505884fae76d69264330bcbee860c261368e1d2`
- 公开投影/簇：`fixed_slot_adjustable` / `C05`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized; one-sided or unknown gain law cannot satisfy arbitrary set-point gain
- 全局控制：activation=1，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `lm`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=boost_only；complete=True；issues=none
  - `mid`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=attenuate_only；complete=True；issues=none
  - `hm`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=boost_only；complete=True；issues=none

### Q-Clone Stereo

- identifier：`VST3-Q-Clone Stereo-695648d-82bb63e2`
- 参数/分页：8 条，1 页，complete=True
- surface signature：`6eaaaa077f612add286e3acf0b7f9c1a19503644dd1bbcde0948153ece5515ef`
- topology signature：`6e534291fdf04148bcf3d7a96fc2135706dcd969041ac6ed51289e306aaf05db`
- 公开投影/簇：`unresolved` / `C02`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": false, "gain": false, "q": false, "slope": false}`
- 拒绝或限制：adaptive/stateful/capture topology; no complete local EQ set-point block
- 全局控制：activation=0，bypass=1，unknown_toggles=2
- 局部 section/band：未从静态参数面恢复；归入待解决特殊结构并拒绝执行。

### Q1 Stereo

- identifier：`VST3-Q1 Stereo-695648d-eda62547`
- 参数/分页：15 条，1 页，complete=True
- surface signature：`7646642ffe064a5a5022863d8796492a4f975ccc155d9fd41105cfad58b259ca`
- topology signature：`7c228d75e00bee1aee4b733661b11dc7d45489dec861c4984def4f2b4e31a994`
- 公开投影/簇：`fixed_slot_adjustable` / `C20`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=4，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### Q10 Stereo

- identifier：`VST3-Q10 Stereo-695648d-3a8f251e`
- 参数/分页：60 条，1 页，complete=True
- surface signature：`156095b8ae4b7ac8be7a8a29db52bdde5a9c89be4cf2dc65866b37ea64fbb219`
- topology signature：`0d9ab98b824b59dd27d27a21c3fa2e62c83b8316511bf1d251cc37938044c62a`
- 公开投影/簇：`fixed_slot_adjustable` / `C19`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=4，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 5`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 6`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 7`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 8`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 9`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 10`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### Q2 Stereo

- identifier：`VST3-Q2 Stereo-695648d-10672547`
- 参数/分页：20 条，1 页，complete=True
- surface signature：`1dcfb06f7fb7f676bfc22037c0da027989cf1490e7bf145214bc9106b8b0e185`
- topology signature：`cade6d632064c330a3ab2acd8943c08257a2a238f7f6e1c6958ab9c18c8c6392`
- 公开投影/簇：`fixed_slot_adjustable` / `C21`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=4，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### Q3 Stereo

- identifier：`VST3-Q3 Stereo-695648d-33282547`
- 参数/分页：25 条，1 页，complete=True
- surface signature：`2ca8d377ed037e38c1418258e0e1cbecbc33e13d56bac9175c1c081494ae1ce8`
- topology signature：`dd23644b119aaf15be7cbd975f0ac37f51399c91f02d2770f279b44c1d3c8cb3`
- 公开投影/簇：`fixed_slot_adjustable` / `C15`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=4，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### Q4 Stereo

- identifier：`VST3-Q4 Stereo-695648d-55e92547`
- 参数/分页：30 条，1 页，complete=True
- surface signature：`0a833c381a15a87049fd066737a9fae15604deaf2e800e48b07a02595a935af0`
- topology signature：`924f25dc923e8f2076a463ac9cd0009a67c6d3e601782b78b366d6acea444677`
- 公开投影/簇：`fixed_slot_adjustable` / `C07`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=4，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### Q6 Stereo

- identifier：`VST3-Q6 Stereo-695648d-9b6b2547`
- 参数/分页：40 条，1 页，complete=True
- surface signature：`18d090e524b961f703256c640db2585de4ee196adc353ccd78c2f89c031b34c4`
- topology signature：`752712c3a3ab8fec7e6f85722f4a10c5a7206d265e09b837c6e75be101b8c25b`
- 公开投影/簇：`fixed_slot_adjustable` / `C13`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=4，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 5`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 6`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### Q8 Stereo

- identifier：`VST3-Q8 Stereo-695648d-e0ed2547`
- 参数/分页：50 条，1 页，complete=True
- surface signature：`1dc6ae1ee1beadeaae01968e5361927b87096693872f987a669267148d085d3e`
- topology signature：`799bfc328c3b1879e006d6dc09d3a9ace460d42f078585492bc266fbbfa28675`
- 公开投影/簇：`fixed_slot_adjustable` / `C19`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=4，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 5`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 6`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 7`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 8`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### RChannel Stereo

- identifier：`VST3-RChannel Stereo-695648d-4db7e9d2`
- 参数/分页：54 条，1 页，complete=True
- surface signature：`060e491e66f166679bd8da3193afec342ec03abe3690d7873046ee22071df2ac`
- topology signature：`8766b4f02ae5aa8d3a1b7e342f09c2beadc94381ec5f64b3ec100b48b5584b42`
- 公开投影/簇：`fixed_slot_adjustable` / `C07`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=1
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### REQ 2 Stereo

- identifier：`VST3-REQ 2 Stereo-695648d-2330bd3`
- 参数/分页：17 条，1 页，complete=True
- surface signature：`f79b9ebea27afbd8e10d1b38fa410b8e0540646b721e3fde1659a701e1ff58e3`
- topology signature：`eb72750f8c67168fefc7e5395e426de4de65806b5235084560ef13d52425bf32`
- 公开投影/簇：`fixed_slot_adjustable` / `C21`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 6`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### REQ 4 Stereo

- identifier：`VST3-REQ 4 Stereo-695648d-9b50c11`
- 参数/分页：29 条，1 页，complete=True
- surface signature：`16386eb10e31556c54a33be40763a9db26bdec4f7fc728636396d2d9b378fe96`
- topology signature：`9327b8b27b3f6f0c0d943319335e8727da439c77a6124625ccb34a7888dda184`
- 公开投影/簇：`fixed_slot_adjustable` / `C07`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 5`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 6`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### REQ 6 Stereo

- identifier：`VST3-REQ 6 Stereo-695648d-11370c4f`
- 参数/分页：41 条，1 页，complete=True
- surface signature：`ad72ae0d0f7799c5fb11493f1c06e1f11aff200ad34bac80df76fa7e8536473b`
- topology signature：`4ea5cbb7322b32c6edf139750c110b9698f89da01fc289ceb68612d10640b498`
- 公开投影/簇：`fixed_slot_adjustable` / `C13`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=0
- 局部 section/band：
  - `band 1`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 2`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 3`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 4`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 5`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `band 6`：placement=writable_continuous；roles=[activation×1, filter_kind×1, frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, filter_kind=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none

### Scheps 73 Stereo

- identifier：`VST3-Scheps 73 Stereo-695648d-7f906368`
- 参数/分页：34 条，1 页，complete=True
- surface signature：`af9d1f6a3188a9441bf05b0ad41b882c65106ee408db9713a894550372251065`
- topology signature：`e971928367ad46c71dd8e1fcd6979c5ed87063a05da838df29a5ccd64fc0aa60`
- 公开投影/簇：`fixed_slot_adjustable` / `C22`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": false, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：incomplete gain-bearing EQ blocks remain; one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=4
- 局部 section/band：
  - `high`：placement=opaque；roles=[activation×1, gain×2]；channels=[gain=left+right, activation=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=False；issues=frequency anchor absent
  - `mid`：placement=writable_discrete；roles=[activation×1, frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right, activation=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `low`：placement=writable_discrete；roles=[activation×1, frequency×2, gain×2]；channels=[frequency=left+right, gain=left+right, activation=shared]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `high pass`：placement=writable_discrete；roles=[activation×2, frequency×2]；channels=[frequency=left+right, activation=left+right]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent

### Scheps Omni Channel 2 Stereo

- identifier：`VST3-Scheps Omni Channel 2 Stereo-695648d-3aab2cf`
- 参数/分页：1819 条，15 页，complete=True
- surface signature：`425429918ef227b96a8e4b2198a589bd8717a79870dafcb4dbf28cd25c40b901`
- topology signature：`29f21c1509bea02b416c9a231512c239018afdcc4b798b74b7b02ef496766088`
- 公开投影/簇：`fixed_slot_adjustable` / `C23`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": true}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=19
- 局部 section/band：
  - `pre hp`：placement=writable_continuous；roles=[activation×2, frequency×2, q×2, slope×1]；channels=[frequency=left+right, q=left+right, activation=left+right, slope=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `pre lp`：placement=writable_continuous；roles=[activation×2, frequency×2, q×2, slope×2]；channels=[frequency=left+right, q=left+right, activation=left+right, slope=left+right]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `comp sc hp f`：placement=writable_continuous；roles=[frequency×2]；channels=[frequency=left+right]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `comp sc bell f`：placement=writable_continuous；roles=[frequency×2]；channels=[frequency=left+right]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `eq low`：placement=writable_continuous；roles=[activation×2, filter_kind×2, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=left+right, filter_kind=left+right]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `eq tone`：placement=writable_continuous；roles=[activation×2, filter_kind×2, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=left+right, filter_kind=left+right]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `eq mid`：placement=writable_continuous；roles=[activation×2, filter_kind×2, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=left+right, filter_kind=left+right]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none
  - `eq high`：placement=writable_continuous；roles=[activation×2, filter_kind×2, frequency×2, gain×2, q×2]；channels=[frequency=left+right, gain=left+right, q=left+right, activation=left+right, filter_kind=left+right]；activation=explicit_binding/active；gain_law=bipolar；complete=True；issues=none

### SSL EV2 Channel Stereo

- identifier：`VST3-SSL EV2 Channel Stereo-695648d-8b57361e`
- 参数/分页：46 条，1 页，complete=True
- surface signature：`2edbf83c5738a558b3e2a3ce55e88a975c74cd381b233969523609bba81d7bfd`
- topology signature：`e938cf8ba5d61d315faa5a00e22813bcd06c97bf391863fe5698a1e9843d52e4`
- 公开投影/簇：`fixed_slot_adjustable` / `C24`
- 执行能力：`full`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": false, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：无结构性拒绝；仍未进行写入验证
- 全局控制：activation=0，bypass=2，unknown_toggles=7
- 局部 section/band：
  - `hp`：placement=writable_continuous；roles=[activation×1, frequency×1]；channels=[frequency=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `lp`：placement=writable_continuous；roles=[activation×1, frequency×1]；channels=[frequency=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `low`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `low mid`：placement=writable_continuous；roles=[frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high mid`：placement=writable_continuous；roles=[frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### SSLChannel Stereo

- identifier：`VST3-SSLChannel Stereo-695648d-b931a17d`
- 参数/分页：36 条，1 页，complete=True
- surface signature：`14d7daf05ee52f08a618f78ff5bd972ed47f82c3b411cbb2c5887cfd4e9be611`
- topology signature：`f8aa13b5b13e31c9a0973f6162539343c8aeff399d7386980a5a646351475228`
- 公开投影/簇：`fixed_slot_adjustable` / `C24`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": true, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=5
- 局部 section/band：
  - `hp`：placement=writable_continuous；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `lp`：placement=writable_continuous；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `low`：placement=writable_continuous；roles=[filter_kind×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, filter_kind=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `low mid`：placement=writable_continuous；roles=[frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high mid`：placement=writable_continuous；roles=[frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high`：placement=writable_continuous；roles=[filter_kind×1, frequency×1, gain×1]；channels=[frequency=shared, gain=shared, filter_kind=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### SSLEQ Stereo

- identifier：`VST3-SSLEQ Stereo-695648d-62ffe694`
- 参数/分页：21 条，1 页，complete=True
- surface signature：`dedd81b430ea7843619149f9b537b572ee4b36773c131638fcf90bf7d9b59386`
- topology signature：`0d0b7396676eba367942598dedc5f81a13692a0ee252be7d5922de36e43dfc3e`
- 公开投影/簇：`fixed_slot_adjustable` / `C25`
- 执行能力：`full`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": false, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：无结构性拒绝；仍未进行写入验证
- 全局控制：activation=0，bypass=1，unknown_toggles=3
- 局部 section/band：
  - `hp`：placement=writable_continuous；roles=[activation×1, frequency×1]；channels=[frequency=shared, activation=shared]；activation=explicit_binding/inactive；gain_law=absent；complete=False；issues=gain role absent
  - `low`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `low mid`：placement=writable_continuous；roles=[frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high mid`：placement=writable_continuous；roles=[frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### SSLGChannel Stereo

- identifier：`VST3-SSLGChannel Stereo-695648d-bc641ceb`
- 参数/分页：36 条，1 页，complete=True
- surface signature：`ed0b91c1922b3cc7876474f433ed711922a9e70348a1c866bd3a6bd20c62bc6a`
- topology signature：`0ebf8a10963091120b7db684f83516aa119eba9e6a32dc39e6576f2256453ed4`
- 公开投影/簇：`fixed_slot_adjustable` / `C24`
- 执行能力：`full`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": true, "gain": true, "q": true, "slope": false}`
- 拒绝或限制：无结构性拒绝；仍未进行写入验证
- 全局控制：activation=0，bypass=1，unknown_toggles=7
- 局部 section/band：
  - `hp`：placement=writable_continuous；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `lp`：placement=writable_continuous；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `low`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `low mid`：placement=writable_continuous；roles=[frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high mid`：placement=writable_continuous；roles=[frequency×1, gain×1, q×1]；channels=[frequency=shared, gain=shared, q=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high`：placement=writable_continuous；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### TRACT LinPhase Stereo

- identifier：`VST3-TRACT LinPhase Stereo-695648d-46e2a1bb`
- 参数/分页：117 条，1 页，complete=True
- surface signature：`6bf4a2fb809ae1243297ae574cedf1ff6480adaf9b8cae83f0d7de86ef6cc0bd`
- topology signature：`1159dd2657348db3d7d405a1c2c2fbdfad7daf6b0107762cf61952ca6cb225f9`
- 公开投影/簇：`fixed_slot_adjustable` / `C26`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": false, "frequency": true, "gain": true, "q": true, "slope": true}`
- 拒绝或限制：adaptive/stateful/capture topology; incomplete gain-bearing EQ blocks remain
- 全局控制：activation=1，bypass=1，unknown_toggles=5
- 局部 section/band：
  - `1`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `2`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `3`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `4`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `5`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `6`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `7`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `8`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `rng amp low`：placement=opaque；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=unknown_or_one_sided；complete=False；issues=frequency anchor absent

### TRACT Stereo

- identifier：`VST3-TRACT Stereo-695648d-4328acb2`
- 参数/分页：117 条，1 页，complete=True
- surface signature：`6bf4a2fb809ae1243297ae574cedf1ff6480adaf9b8cae83f0d7de86ef6cc0bd`
- topology signature：`1159dd2657348db3d7d405a1c2c2fbdfad7daf6b0107762cf61952ca6cb225f9`
- 公开投影/簇：`fixed_slot_adjustable` / `C26`
- 执行能力：`unsupported`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": true, "filter_kind": false, "frequency": true, "gain": true, "q": true, "slope": true}`
- 拒绝或限制：adaptive/stateful/capture topology; incomplete gain-bearing EQ blocks remain
- 全局控制：activation=1，bypass=1，unknown_toggles=5
- 局部 section/band：
  - `1`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `2`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `3`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `4`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `5`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `6`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `7`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `8`：placement=writable_continuous；roles=[activation×1, frequency×1, gain×1, q×2, slope×2]；channels=[frequency=shared, gain=shared, q=shared, activation=shared, slope=shared]；activation=explicit_binding/inactive；gain_law=bipolar；complete=True；issues=none
  - `rng amp low`：placement=opaque；roles=[gain×1]；channels=[gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=unknown_or_one_sided；complete=False；issues=frequency anchor absent

### VEQ3 Stereo

- identifier：`VST3-VEQ3 Stereo-695648d-bd93853a`
- 参数/分页：14 条，1 页，complete=True
- surface signature：`d87a73e34216ec109e6fda004341a9c09d58ec00e5b70a28aba9a18203c99887`
- topology signature：`afbb7e8336ac5bcac16fb9ba3c121169b68e5b17dfb0abaa089b2e1cc84184b6`
- 公开投影/簇：`fixed_slot_adjustable` / `C05`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=3
- 局部 section/band：
  - `hp`：placement=writable_discrete；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `low`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `mf`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

### VEQ4 Stereo

- identifier：`VST3-VEQ4 Stereo-695648d-bd974732`
- 参数/分页：21 条，1 页，complete=True
- surface signature：`210dd30eadf42371249d034c11d403f7e10d1faea6538d87cf788eca449a179c`
- topology signature：`7abb9ce99edaf44e91552585200c9c327b74b092fa981a9148461c69cc996174`
- 公开投影/簇：`fixed_slot_adjustable` / `C27`
- 执行能力：`partial_quantized`；证据级别：`structurally_inferred_not_write_tested`
- capabilities：`{"activation": false, "filter_kind": false, "frequency": true, "gain": true, "q": false, "slope": false}`
- 拒绝或限制：one or more requested roles are quantized
- 全局控制：activation=0，bypass=1，unknown_toggles=7
- 局部 section/band：
  - `hp`：placement=writable_discrete；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `lp`：placement=writable_discrete；roles=[frequency×1]；channels=[frequency=shared]；activation=always_active_or_unexposed/unknown；gain_law=absent；complete=False；issues=gain role absent
  - `low`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `low mid`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high mid`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none
  - `high`：placement=writable_discrete；roles=[frequency×1, gain×1]；channels=[frequency=shared, gain=shared]；activation=always_active_or_unexposed/unknown；gain_law=bipolar；complete=True；issues=none

## 6. 内部 EQControlTopology 方案

三个公开分类名只保留为兼容投影。建议内部模型为：

```text
EQControlTopology
  evidence: ordered parameter observations + confidence
  sections: local repeated EQ blocks
  axes:
    placement: writable_continuous | writable_discrete | fixed_label | opaque
    role: frequency | gain | q | filter_kind | slope | activation
    channel: shared | left/right mirrored | linked | asymmetric
    activation: always_active | explicit_binding | slot_lifecycle | unknown
    gain_law: bipolar | quantized | split_boost_attenuate | one_sided
    interaction: independent | coupled | dynamic_peripheral | stateful
  execution_policy:
    supported_roles + quantization + ordering + fail_closed_reasons
```

投影规则：有 slot lifecycle 的可创建槽投影为 `free_floating`；有可写频率的固定 section 投影为 `fixed_slot_adjustable`；只有可恢复中心频率和 Gain 的图形/固定频段投影为 `fixed_freq`。无法恢复结构时不强行投影，返回 unresolved 并拒绝执行。

## 7. 安全执行原则（后续阶段设计，不在本阶段实现）

- 只允许完整局部块进入执行。
- 动态 EQ 的 Range/Threshold/Attack/Release 永远不属于静态 set-point 写集。
- inactive slot 必须先写 Shape/Frequency/Q/Gain，最后写 Activation。
- L/R 镜像必须拥有完整、对称的角色绑定。
- 离散频率、增益和 Q 返回 requested/actual/quantized。
- split boost/attenuate、状态型捕获、自适应匹配、未知固定频率一律 fail-closed。
- topology signature 和聚类只能提供结构证据，不能替代未来的写入回读验证或音频行为证据。

## 8. 待解决结构

- 参数名无法表达的共享模式、内部联动和滤波器行为仍需后续受控实验。
- `always_active_or_unexposed` 只能说明没有观察到 activation 参数，不能证明 DSP 永远启用。
- 枚举标签若不能恢复物理值，应继续拒绝，而不是按 normalized 值猜测。
- 状态型、自适应和校准型 EQ 不能仅凭静态参数表获得通用 set-point 能力。
- 当前生产 `buildMarvelGEQSummary` 仍按 `Marvel GEQ` 名称进入专用频率表，违反新的无插件名称生产分支方向；本阶段仅登记，不修改。

## 9. 禁用路径与现场保护证明

- Agent 工具调用：`{"plugin.get_parameters": 136, "plugin.load_to_rack": 50, "track.add_audio": 50}`
- Kernel 命令：`{"plugin_search": 50}`
- 插件参数写入：0
- 音频主动探测：0
- learn/profile：0/0
- SPAL/B4：0/0
- Plugin Alliance 参数读取：0

生产文件哈希、受保护运行文件哈希和 Git 状态前后对比见同一产物目录下的 `run_manifest.json` 与最终 `acceptance.json`。

## 10. 失败清单

- 无。50 个目标均完成参数分页采集。

## 11. 停止点

第一阶段到此结束。本报告没有修改生产识别器、执行器或公开分类 API；下一阶段必须经用户确认后才能开始。
