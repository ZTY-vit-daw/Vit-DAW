# 通用静态 EQ 生产识别器、规划器与原子执行器（第三阶段）

## 1. 验收结论

第三阶段已在 `codex/eq-structural-matrix-unification` 分支完成生产实现与真实链验证。

- 通用公开 shape：`bell`、`low_shelf`、`high_shelf`、`low_cut`、`high_cut`。
- 通用 action：`upsert`、`modify`、`disable`、`remove`、`undo`。
- 原子入口：`plugin_grabber.apply_eq_edits`；`plugin_grabber.set_eq_point` 仅保留为单 edit 兼容适配器。
- 50 个第一阶段真实 Waves Stereo capture 全部由生产 Go 识别器离线回放；没有重新烟测 50 次。
- 44/50 形成结构模型，38/50 至少有一种可执行通用 shape，6 个形成模型但全部通用 shape 拒绝，6 个不形成通用静态 EQ 模型。
- 1250 个型号 × shape × action 行相对第二阶段：1237 一致、13 个安全能力扩展、0 回退。
- 13 个扩展全部来自真实枚举中已观测的 `Off`/`Out` sentinel，因此生产识别器能够证明 `disable`，不是名称猜测或放宽安全门。
- 真实 Godot → Kernel → Agent 链只使用 5 个正向实例和 3 个只读拒绝实例；所有正向写入均 fresh readback、撤销并验证 touched 参数恢复。
- Q10 的中文自然语言请求“控制当前效果器将3400Hz降低3dB，Q值为0.5”实际路由为 `explain_controls → apply_eq_edits`，读回 3396 Hz、-3.0 dB、Q=0.5；随后完成 modify、disable 和逐事务 undo。
- 未执行音频主动探测、完整 learn、profile、SPAL、B4 或 Plugin Alliance 参数读取。

权威产物：

- `artifacts/waves_eq_topology_census/20260727_201627/analysis/static_eq_phase3/production_replay.json`
- `artifacts/waves_eq_topology_census/20260727_201627/analysis/static_eq_phase3/production_replay_cases.csv`
- `artifacts/waves_eq_topology_census/20260727_201627/analysis/static_eq_phase3/production_replay_capabilities.csv`
- `artifacts/waves_eq_topology_census/20260727_201627/analysis/static_eq_phase3/production_replay_summary.md`
- `artifacts/waves_eq_topology_census/20260727_201627/analysis/static_eq_phase3/live_smoke/20260727_224911/summary.json`
- `artifacts/waves_eq_topology_census/20260727_201627/analysis/static_eq_phase3/live_smoke/20260727_224911/run_manifest.json`

## 2. 从用户需求重新得到的最小聚类

用户最终需求不是“辨认某个 EQ 品牌”，而是把自然语言声学目标可靠地落实为参数动作。因此最小需求空间是两个正交集合：

1. 声学 shape：Bell/Peak、Low Shelf、High Shelf、Low Cut、High Cut。
2. 生命周期 action：创建或设置、修改、关闭、删除、撤销。

它们不能再由一个插件级标签表达。例如同一 F6 实例的动态 Bell 必须拒绝，但独立 Low/High Cut 可以执行；同一 Channel Strip 内的 EQ、sidechain filter 和 detector 也必须局部拆分。因此生产判定单位是 section × shape × action，而不是插件名或插件级 `set_eq_point_supported`。

内部最小寻址聚类为：

| 内部寻址原型 | 结构含义 | 可执行含义 |
|---|---|---|
| `anchored` | 固定中心频率、无频率写参数 | 选最近的已证明频点，只写本地增益；频率可量化 |
| `resident` | section 常驻，频率可写或枚举选择 | 可设置/修改；只有观测到关闭值才可 disable；不能 remove |
| `allocatable` | 存在完整 `Used ↔ Unused` slot 生命周期 | 可分配、修改、disable，并在有 `Unused` 证据时 remove |

这三个寻址原型与五种 shape、物理域、连续/枚举量化、激活策略、通道镜像和排除证据正交。旧公开名 `fixed_freq`、`fixed_slot_adjustable`、`free_floating` 继续输出，但只作为内部矩阵的兼容投影；规划器不以它们作为执行门。

## 3. 统一 EQControlTopology

`eq-control-topology/v1` 的事实轴包括：

- section：本地重复块、主/辅助块和稳定 section key；
- addressing：`anchored`、`resident`、`allocatable`；
- role binding：frequency、gain、Q、filter kind、slope、activation；
- physical domain：最小/最大值、单位、比例、五点曲线或可达枚举；
- channel binding：shared 或已证明完整镜像的 linked Stereo/L-R/`L/M · R/S`；
- activation：显式 binding、频率域内 `Off/Out` sentinel、always active；
- shape reachability：只从专用结构或枚举可达值推导；
- shape/action capability：逐 section 计算，再向插件级做“存在可行 section”的聚合；
- exclusion evidence：动态、sidechain/detector、状态型/capture、非 linked channel、耦合增益、频率 law 不纯、结构不完整等稳定拒绝码。

`topology_generation` 对 schema、section、参数 ID、role、channel、activation kind、gain polarity、物理域、可达枚举和排除证据做规范化哈希。它不包含插件名称、manufacturer、identifier 或当前旋钮值，因此普通参数修改不会使引用失效，结构改变则会使引用过期。

生产识别器不包含 Waves 或任何具体型号的执行分支。插件名称只存在于测试数据、离线报告和真实链用例选择中。

## 4. 严格规划与原子执行

LLM 只提交声学字段，不提交 band、parameter ID、normalized value 或插件类型。规划器从同一份 live snapshot 完成全部 edit：

1. 解析并验证 shape/action、必填字段、有限数值和正值约束。
2. 对每个显式 Frequency/Gain/Q/Slope 做物理域写前检查；连续域越界拒绝，离散域内目标才允许量化。
3. `upsert` 选择未被本批次预留的可行 section；`modify/disable/remove` 必须使用带 target 与 topology generation 的 `control_ref`。
4. Cut 禁止 Gain；Bell/Shelf upsert 必须有 Gain；显式 Q/Slope 无 binding 时整批写前拒绝。
5. 同一批次 section 冲突或 parameter ID 值冲突整批拒绝。
6. 所有非 activation 写排在前面，activation 全局最后。
7. 合并 touched 参数 preimage，并只发一次 `plugin.set_params_batch`。
8. fresh readback 验证 normalized、枚举 label、激活状态和物理值；连续非线性映射最多做 12 次有界校正。
9. 任一步失败都用完整 preimage 补偿回滚并再次读回验证；回滚本身失败时明确报告，不能谎称已恢复。
10. 成功返回 `exact|quantized`、actual readback、`control_ref` 和 `operation_ref`。

`operation_ref` 对 postimage 做冲突检查，拒绝 out-of-order destructive undo；事务日志是容量 256 的进程内最旧优先淘汰队列，响应明确给出 `lifetime=process` 与 `expires_on_restart=true`。`remove` 只允许 `allocatable` section；常驻 Waves section 使用 `disable` 或 `undo`。

## 5. 50 个 Waves 型号生产归属

“可执行”表示至少一个 shape 的 `upsert` 可证明安全；“边界/拒绝证据”同时包含该型号中其他 shape/action 的局部拒绝，不能把它误读为整机不可执行。完整 1250 行逐 action 结果见 `production_replay_capabilities.csv`。

| Waves Stereo 型号 | 公开兼容投影 | 可执行 | 可执行 upsert shape | 边界/拒绝证据 |
|---|---|---:|---|---|
| Abbey Road RS56 Passive EQ Stereo | fixed_slot_adjustable | 是 | bell | forward_control_unavailable, section_not_deallocatable, shape_not_provably_reachable |
| Abbey Road The King's Microphones Stereo | unrecognized | 否 | — | not_static_eq |
| API-550A Stereo | fixed_slot_adjustable | 是 | bell, high_shelf, low_shelf | activation_binding_unavailable, forward_control_unavailable, section_not_deallocatable, shape_not_provably_reachable |
| API-550B Stereo | fixed_slot_adjustable | 是 | bell, high_shelf, low_shelf | activation_binding_unavailable, forward_control_unavailable, section_not_deallocatable, shape_not_provably_reachable |
| API-560 Stereo | fixed_freq | 是 | bell | activation_binding_unavailable, forward_control_unavailable, section_not_deallocatable, shape_not_provably_reachable |
| Curves AQ Live Stereo | fixed_slot_adjustable | 否 | — | stateful_surface_dependency_unresolved；其余为派生局部拒绝 |
| Curves AQ Stereo | fixed_slot_adjustable | 否 | — | stateful_surface_dependency_unresolved；其余为派生局部拒绝 |
| EMO-F2 Stereo | fixed_slot_adjustable | 是 | high_cut, low_cut | required_gain_binding_absent, section_not_deallocatable, shape_not_provably_reachable |
| EMO-Q4 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, low_cut, high_shelf, high_cut | section_not_deallocatable, shape_not_provably_reachable |
| F6 Stereo | fixed_slot_adjustable | 是 | high_cut, low_cut | dynamic_section_excluded；动态 Bell/Shelf 排除，独立 Cut 保留 |
| F6-RTA Stereo | fixed_slot_adjustable | 是 | high_cut, low_cut | dynamic_section_excluded；动态 Bell/Shelf 排除，独立 Cut 保留 |
| GEQ Classic Stereo | fixed_freq | 是 | bell, high_cut, low_cut | activation_binding_unavailable, required_gain_binding_absent, section_not_deallocatable |
| GEQ Modern Stereo | fixed_freq | 是 | bell, high_cut, low_cut | activation_binding_unavailable, required_gain_binding_absent, section_not_deallocatable |
| H-EQ Stereo | fixed_slot_adjustable | 是 | bell, high_shelf, low_shelf, high_cut, low_cut | required_gain_binding_absent, section_not_deallocatable, shape_not_provably_reachable |
| H-EQ-Light Stereo | fixed_slot_adjustable | 是 | bell, high_shelf, low_shelf, high_cut, low_cut | required_gain_binding_absent, section_not_deallocatable, shape_not_provably_reachable |
| Kramer HLS Stereo | fixed_slot_adjustable | 否 | — | frequency_law_not_pure, section_incomplete |
| LinEQ Broadband Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, high_shelf, low_cut, high_cut | section_not_deallocatable |
| LinEQ Lowband Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, low_cut | forward_control_unavailable, section_not_deallocatable, shape_not_provably_reachable |
| MannyM EQ Stereo | fixed_slot_adjustable | 是 | bell, high_cut, low_cut | required_gain_binding_absent, section_not_deallocatable, shape_not_provably_reachable |
| PuigTec EQP1A Stereo | unrecognized | 否 | — | not_static_eq；耦合 boost/attenuate 模拟网络 |
| PuigTec MEQ5 Stereo | unrecognized | 否 | — | not_static_eq；单边/耦合增益 law 不满足任意 bipolar set-point |
| Q1 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, high_shelf, low_cut, high_cut | section_not_deallocatable |
| Q2 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, high_shelf, low_cut, high_cut | section_not_deallocatable |
| Q3 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, high_shelf, low_cut, high_cut | section_not_deallocatable |
| Q4 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, high_shelf, low_cut, high_cut | section_not_deallocatable |
| Q6 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, high_shelf, low_cut, high_cut | section_not_deallocatable |
| Q8 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, high_shelf, low_cut, high_cut | section_not_deallocatable |
| Q10 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, high_shelf, low_cut, high_cut | section_not_deallocatable |
| Q-Clone Stereo | unrecognized | 否 | — | not_static_eq；stateful capture surface |
| REQ 2 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, low_cut, high_shelf, high_cut | section_not_deallocatable, shape_not_provably_reachable |
| REQ 4 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, low_cut, high_shelf, high_cut | section_not_deallocatable, shape_not_provably_reachable |
| REQ 6 Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, low_cut, high_shelf, high_cut | section_not_deallocatable, shape_not_provably_reachable |
| Scheps 73 Stereo | fixed_slot_adjustable | 是 | bell, low_cut | required_gain_binding_absent, section_not_deallocatable, shape_not_provably_reachable |
| SSLEQ Stereo | fixed_slot_adjustable | 是 | bell, low_cut | activation_binding_unavailable, required_gain_binding_absent, section_not_deallocatable |
| TRACT LinPhase Stereo | fixed_freq | 否 | — | frequency_anchor_unavailable, frequency_law_not_pure, section_incomplete |
| TRACT Stereo | fixed_freq | 否 | — | frequency_anchor_unavailable, frequency_law_not_pure, section_incomplete |
| VEQ3 Stereo | fixed_slot_adjustable | 是 | low_cut | required_gain_binding_absent, section_not_deallocatable, shape_not_provably_reachable |
| VEQ4 Stereo | fixed_slot_adjustable | 是 | bell, high_shelf, low_shelf, high_cut, low_cut | required_gain_binding_absent, section_not_deallocatable, shape_not_provably_reachable |
| Abbey Road EMI TG12345 Ch Stereo | fixed_freq | 否 | — | frequency_anchor_unavailable, frequency_law_not_pure, section_incomplete |
| Abbey Road REDD.17 Stereo | unrecognized | 否 | — | not_static_eq |
| Abbey Road REDD.37.51 Stereo | unrecognized | 否 | — | not_static_eq |
| AudioTrack Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, high_shelf, low_cut, high_cut | section_not_deallocatable, shape_not_provably_reachable |
| CLA MixHub Lite Stereo | fixed_slot_adjustable | 是 | bell, high_cut, low_cut | sidechain_or_detector_section_excluded；独立静态块保留 |
| CLA MixHub Stereo | fixed_slot_adjustable | 是 | bell, high_cut, low_cut | sidechain_or_detector_section_excluded；独立静态块保留 |
| Magma Channel Strip Stereo | fixed_slot_adjustable | 是 | bell, low_cut | activation_binding_unavailable, required_gain_binding_absent, section_not_deallocatable |
| RChannel Stereo | fixed_slot_adjustable | 是 | bell, low_shelf, low_cut, high_shelf, high_cut | sidechain_or_detector_section_excluded, section_incomplete；安全静态块保留 |
| Scheps Omni Channel 2 Stereo | fixed_slot_adjustable | 是 | bell, high_shelf, low_shelf, high_cut, low_cut | dynamic_section_excluded, sidechain_or_detector_section_excluded；安全静态块保留 |
| SSL EV2 Channel Stereo | fixed_slot_adjustable | 是 | bell, high_shelf, low_shelf, high_cut, low_cut | activation_binding_unavailable, required_gain_binding_absent, section_not_deallocatable |
| SSLChannel Stereo | fixed_slot_adjustable | 是 | bell, high_cut, low_cut | activation_binding_unavailable, required_gain_binding_absent, section_not_deallocatable |
| SSLGChannel Stereo | fixed_slot_adjustable | 是 | bell, high_cut, low_cut | activation_binding_unavailable, required_gain_binding_absent, section_not_deallocatable |

## 6. 13 个安全能力扩展

相对第二阶段保守模型，生产识别器新增以下 `disable`：Magma Channel Strip low_cut；MannyM EQ low/high_cut；SSLChannel low/high_cut；SSLGChannel low/high_cut；VEQ3 low_cut；VEQ4 的 bell、low/high shelf、low/high cut。

每一项都来自对应参数枚举的真实 `Off`/`Out` 可达值。upsert shape 矩阵完全一致；没有新增 Bell/Shelf/Cut 写入能力，也没有任何能力回退。

## 7. 真实链烟测

通过产物：`live_smoke/20260727_224911/summary.json`。

| 实例 | 路径 | 结果 | 恢复证据 |
|---|---|---|---|
| Q10 Stereo | 中文自然语言 → explain → atomic tool；另测 modify/disable/undo | exact；3396 Hz、-3.0 dB、Q=0.5 | 5 个 touched 参数恢复 |
| API-550A Stereo | Bell 3000 Hz / -3 dB | quantized | 2 个 touched 参数恢复 |
| API-560 Stereo | 固定频点 Bell，3400 Hz / -3 dB | quantized | 1 个 touched 参数恢复 |
| EMO-F2 Stereo | 打开 80 Hz Low Cut | exact | 4 个 touched 参数恢复 |
| SSL EV2 Channel Stereo | Low Cut + High Shelf 两 edit 原子批量 | exact | 5 个 touched 参数恢复 |
| F6 Stereo | 只读 | 动态 Bell 拒绝，独立 Low Cut 保留 | 写入 0 |
| PuigTec EQP1A Stereo | 只读 | `eq_band_summary=null` / not_static_eq | 写入 0 |
| Q-Clone Stereo | 只读 | `eq_band_summary=null` / not_static_eq | 写入 0 |

烟测审计计数：自然语言 1 次、正向实例 5、只读拒绝实例 3、音频探测 0、learn 0、profile 0、SPAL 0、B4 0、Plugin Alliance 参数读取 0。隔离运行前后三个受保护 Workspace 文件 SHA-256 完全一致。

## 8. 测试与构建

- `go test ./...`：通过。
- `go test ./internal/workflows/plugingrabber ./internal/chat ./internal/agentloop ./internal/tools ./cmd/eqtopologyreplay`：通过。
- 上述本次相关包 `go vet`：通过。
- `go build ./cmd/...`：通过；真实链使用当前源码构建的 `agent/bin/VitAgent.exe`。
- `go vet ./...`：被仓库既有 Windows unsafe 告警阻断：`internal/vsphub/audio_feature_shm_windows.go:52` 与 `internal/harness/harness.go:4634`；本次 EQ 文件没有 vet 告警。
- `git diff --check`：通过。

新增测试覆盖：filter-only、动态局部排除、耦合网络拒绝、Stereo/L-R/`L/M · R/S` 镜像、topology generation 稳定性、引用校验与过期、写前域校验、离散量化、section 预留、参数冲突、activation-last、HTTP/Agent 路由、原子成功、部分批量失败回滚、fresh readback 失败回滚、回滚失败不虚报、undo、状态冲突、日志最旧淘汰以及兼容适配器引用返回。

## 9. 明确未纳入和待解决结构

以下不是第三阶段遗漏，而是通用静态 EQ 能力边界：

- 动态 EQ、threshold/range/attack/release、sidechain/detector：只保留可独立证明的静态 section。
- PuigTec 等耦合 boost/attenuate 模拟音色网络：需要专用效果器语义，不进入任意 bipolar set-point。
- Curves AQ、Q-Clone、TRACT 等 capture/match/calibration/stateful surface：保持拒绝。
- M/S、非对称 L/R 或运行时非 linked channel mode：保持拒绝；当前只接受已证明完整镜像的 linked 控制。
- Notch：内部可以辨认，但不属于公开通用协议。
- `remove` 的真实 Waves 样本没有 allocatable slot；生产实现与单测已覆盖，真实链只证明 resident disable/undo。
- `operation_ref` 为进程内临时事务引用，重启或超过 256 条后过期；若未来需要跨进程撤销，应进入持久化项目事务层，而不是放宽现有引用。
- Plugin Alliance 被保留为冻结外部测试集，本阶段按禁令未读取；后续只有得到明确确认才可运行。
- 两处全仓既有 unsafe vet 告警需由对应 Windows SHM/harness 所有者处理，与本 EQ 目标无关。

## 10. 需求逐项审计

| 需求 | 证据 | 结论 |
|---|---|---|
| 五种通用 shape | 生产 topology 与 1250 行 CSV | 完成 |
| 修改、关闭、删除、撤销 | planner/executor 单测；Q10 真实 modify/disable/undo | 完成；remove 仅 allocatable，真实 Waves 无该原型 |
| 原子批量 | 单次 batch、完整 preimage/fresh readback/rollback；SSL EV2 真实双 edit | 完成 |
| 多轴内部模型，公开名只作投影 | `EQControlTopology`、逐 section shape/action capability | 完成 |
| 不依赖插件名称 | 生产代码搜索与 generation payload 审计 | 完成 |
| 50 型号普查 | 50 个真实 capture 生产回放、cases CSV、capability CSV | 完成 |
| 真实 Godot → Kernel → Agent | 5 正向 + 3 只读拒绝，隔离 manifest | 完成 |
| 自然语言到工具 | Q10 中文请求实际执行新工具并读回 | 完成 |
| 禁止项 | live audit 全部为 0；PA 未读 | 完成 |

第三阶段在此停止。动态 EQ、模拟音色网络、状态型 EQ 和 Plugin Alliance 外部集均未被扩入通用功能。
