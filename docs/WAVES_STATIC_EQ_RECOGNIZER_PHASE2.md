# 通用静态 EQ 控制需求重聚类与识别器详细设计（第二阶段）

> 本阶段只重放第一阶段的真实只读参数证据并进行符号规划；没有加载新插件、写参数、主动音频探测、learn/profile、SPAL、B4 或 Plugin Alliance 参数读取。

## 1. 结论

- Waves 型号：50。
- 通用 shape：bell, low_shelf, high_shelf, low_cut, high_cut。
- 通用 action：upsert, modify, disable, remove, undo。
- 逐 shape/action 行数：1250；exact=495，quantized=36，rejected=719。
- 规范自然语言意图重放：550；exact=231，quantized=25，rejected=294。
- 精确 plan 分区：78；寻址最小分区：3；最小性成立=true。

核心结论：`anchored_band`、`resident_section`、`allocatable_section` 不是插件类型，而是逐 section、逐请求推导的三个最小寻址原型。Bell/Shelf/Cut 与寻址原型正交。

## 2. 控制需求边界

纳入：Bell/Peak、Low Shelf、High Shelf、Low Cut、High Cut；upsert、modify、disable、remove、undo；linked stereo；默认原子批量。

排除：动态 EQ 外设、耦合模拟音色网络、捕获/匹配/校准状态、M/S 或非对称声道控制、Notch。动态插件中的静态 section 只有在局部独立性可证明时才允许进入通用集合。

## 3. 最小聚类方法

聚类对象是 `控制需求 × section 事实` 产生的符号写入程序，不是插件。先生成身份无关 `control_plan_signature`，再依次按 action program、addressing、shape obligations、selection、conversion、write order、activation、channel、precondition、postcondition 和 rollback 做精确分区细化。全程不用距离阈值。

### 三个寻址原型不可合并

| 原型 | Frequency 写入 | 槽位分配 | 激活顺序 |
|---|---:|---|---|
| anchored | False | none | if_needed |
| resident | True | none | if_needed |
| allocatable | True | reserve_inactive_slot | activation_last |

任意两类至少在 Frequency 是否写入、是否分配槽位或 Activation 是否必须最后写之一不同；合并后会改变前置条件或写入程序，因此三类是行为最小分区。Waves 本次没有 allocatable 实例，该类来自当前 Agent ‘创建频段’需求的身份无关契约原型，不使用 Plugin Alliance 数据。

## 4. Request-agnostic 识别器

```text
flat parameter observations
  -> role candidates + physical domains + enums
  -> local section graph
  -> per-shape reachability evidence
  -> addressing / channel / activation / gain-law facts
  -> EQControlTopology (no execution decision)
```

识别器仅输出事实。具体 section 选择、量化、字段硬约束和拒绝由 `EQIntentPlanner` 根据请求计算。插件级 `set_eq_point_supported` 应被逐 shape/action capability 替代。

### Shape 完整性矩阵

| Shape | 必需角色 | 可选角色 | 额外证据 |
|---|---|---|---|
| Bell/Peak | Frequency + 双极 Gain | Q | 显式 Bell、完整 peak section 或固定 graphic anchor |
| Low Shelf | Frequency + 双极 Gain | Q | 明确 Low 方向与 Shelf 证据 |
| High Shelf | Frequency + 双极 Gain | Q | 明确 High 方向与 Shelf 证据 |
| Low Cut | Frequency | Q、Slope | 明确 Low Cut/High Pass；不要求 Gain |
| High Cut | Frequency | Q、Slope | 明确 High Cut/Low Pass；不要求 Gain |

## 5. Action 语义

- `upsert`：按 shape 和可达域选择 resident/anchor，或在 allocatable surface 上复用/预留槽位。
- `modify`：必须解析当前 topology generation 下的 `control_ref`，不得重新猜 band。
- `disable`：只允许 explicit activation、implicit sentinel 或 slot lifecycle；无可证明关闭操作时拒绝。
- `remove`：只适用于 allocatable slot；resident/anchored 只能 disable 或 undo。
- `undo`：使用 `operation_ref` 恢复事务前快照，不通过识别器猜逆操作。

## 6. Agent-facing 工具协议

```json
{"edits":[{"action":"upsert","shape":"low_cut","frequency_hz":80,"slope_db_per_oct":24},{"action":"upsert","shape":"bell","frequency_hz":3400,"gain_db":-3,"q":0.5}],"atomic":true}
```

LLM 只提交声学目标，不提交插件名、band 编号、参数 ID、normalized value 或寻址原型。工具结果返回 `exact|quantized|rejected`、`control_ref`、`operation_ref`、requested/actual、量化字段、稳定拒绝码、回读和回滚状态。所有显式字段默认是硬要求。

## 7. 原子批量事务

1. 从同一不可变参数快照识别 topology。
2. 为全部 edits 选择或预留 section。
3. 检测 section 冲突、槽位不足和字段缺失。
4. 在写入前完成全部物理域转换与 dependency graph。
5. 以 topology generation/fingerprint 做过期检查。
6. 按依赖顺序写入，Activation 永远最后。
7. 对所有显式物理字段回读。
8. 任一失败则逆序恢复整个批次并验证回滚。

## 8. Waves 50 型号逐 shape 的 upsert 摘要

| 型号 | Bell | Low Shelf | High Shelf | Low Cut | High Cut |
|---|---|---|---|---|---|
| Abbey Road EMI TG12345 Ch Stereo | rejected | rejected | rejected | rejected | rejected |
| Abbey Road The King's Microphones Stereo | rejected | rejected | rejected | rejected | rejected |
| Abbey Road REDD.17 Stereo | rejected | rejected | rejected | rejected | rejected |
| Abbey Road REDD.37.51 Stereo | rejected | rejected | rejected | rejected | rejected |
| Abbey Road RS56 Passive EQ Stereo | quantized | rejected | rejected | rejected | rejected |
| API-550A Stereo | quantized | quantized | quantized | rejected | rejected |
| API-550B Stereo | quantized | quantized | quantized | rejected | rejected |
| API-560 Stereo | quantized | rejected | rejected | rejected | rejected |
| AudioTrack Stereo | exact | exact | exact | exact | exact |
| CLA MixHub Lite Stereo | exact | rejected | rejected | exact | exact |
| CLA MixHub Stereo | exact | rejected | rejected | exact | exact |
| Curves AQ Live Stereo | rejected | rejected | rejected | rejected | rejected |
| Curves AQ Stereo | rejected | rejected | rejected | rejected | rejected |
| EMO-F2 Stereo | rejected | rejected | rejected | exact | exact |
| EMO-Q4 Stereo | exact | exact | exact | exact | exact |
| F6-RTA Stereo | rejected | rejected | rejected | exact | exact |
| F6 Stereo | rejected | rejected | rejected | exact | exact |
| GEQ Classic Stereo | exact | rejected | rejected | exact | exact |
| GEQ Modern Stereo | exact | rejected | rejected | exact | exact |
| H-EQ-Light Stereo | exact | exact | exact | exact | exact |
| H-EQ Stereo | exact | exact | exact | exact | exact |
| Kramer HLS Stereo | rejected | rejected | rejected | rejected | rejected |
| LinEQ Broadband Stereo | exact | exact | exact | exact | exact |
| LinEQ Lowband Stereo | exact | exact | rejected | exact | rejected |
| Magma Channel Strip Stereo | exact | rejected | rejected | quantized | rejected |
| MannyM EQ Stereo | quantized | rejected | rejected | exact | exact |
| PuigTec EQP1A Stereo | rejected | rejected | rejected | rejected | rejected |
| PuigTec MEQ5 Stereo | rejected | rejected | rejected | rejected | rejected |
| Q10 Stereo | exact | exact | exact | exact | exact |
| Q1 Stereo | exact | exact | exact | exact | exact |
| Q2 Stereo | exact | exact | exact | exact | exact |
| Q3 Stereo | exact | exact | exact | exact | exact |
| Q4 Stereo | exact | exact | exact | exact | exact |
| Q6 Stereo | exact | exact | exact | exact | exact |
| Q8 Stereo | exact | exact | exact | exact | exact |
| Q-Clone Stereo | rejected | rejected | rejected | rejected | rejected |
| RChannel Stereo | exact | exact | exact | exact | exact |
| REQ 2 Stereo | exact | exact | exact | exact | exact |
| REQ 4 Stereo | exact | exact | exact | exact | exact |
| REQ 6 Stereo | exact | exact | exact | exact | exact |
| Scheps 73 Stereo | quantized | rejected | rejected | quantized | rejected |
| Scheps Omni Channel 2 Stereo | exact | exact | exact | exact | exact |
| SSL EV2 Channel Stereo | exact | exact | exact | exact | exact |
| SSLChannel Stereo | exact | rejected | rejected | exact | exact |
| SSLEQ Stereo | exact | rejected | rejected | exact | rejected |
| SSLGChannel Stereo | exact | rejected | rejected | exact | exact |
| TRACT LinPhase Stereo | rejected | rejected | rejected | rejected | rejected |
| TRACT Stereo | rejected | rejected | rejected | rejected | rejected |
| VEQ3 Stereo | rejected | rejected | rejected | quantized | rejected |
| VEQ4 Stereo | quantized | quantized | quantized | quantized | quantized |

完整 1,250 行矩阵见 `shape_action_matrix.csv`；规范指令的目标值重放见 `canonical_intent_matrix.csv`。

## 9. 各型号详细能力与拒绝

### Abbey Road EMI TG12345 Ch Stereo

- `bell`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### Abbey Road The King's Microphones Stereo

- `bell`：upsert=rejected (no_candidate_section)；modify=rejected (no_candidate_section)；disable=rejected (no_candidate_section)；remove=rejected (no_candidate_section)；undo=rejected (no_candidate_section)
- `low_shelf`：upsert=rejected (no_candidate_section)；modify=rejected (no_candidate_section)；disable=rejected (no_candidate_section)；remove=rejected (no_candidate_section)；undo=rejected (no_candidate_section)
- `high_shelf`：upsert=rejected (no_candidate_section)；modify=rejected (no_candidate_section)；disable=rejected (no_candidate_section)；remove=rejected (no_candidate_section)；undo=rejected (no_candidate_section)
- `low_cut`：upsert=rejected (no_candidate_section)；modify=rejected (no_candidate_section)；disable=rejected (no_candidate_section)；remove=rejected (no_candidate_section)；undo=rejected (no_candidate_section)
- `high_cut`：upsert=rejected (no_candidate_section)；modify=rejected (no_candidate_section)；disable=rejected (no_candidate_section)；remove=rejected (no_candidate_section)；undo=rejected (no_candidate_section)

### Abbey Road REDD.17 Stereo

- `bell`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)
- `low_shelf`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)
- `high_cut`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)

### Abbey Road REDD.37.51 Stereo

- `bell`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)
- `low_shelf`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)
- `high_cut`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)

### Abbey Road RS56 Passive EQ Stereo

- `bell`：upsert=quantized；modify=quantized；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### API-550A Stereo

- `bell`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### API-550B Stereo

- `bell`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### API-560 Stereo

- `bell`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### AudioTrack Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### CLA MixHub Lite Stereo

- `bell`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (sidechain_or_detector_section_excluded)；modify=rejected (sidechain_or_detector_section_excluded)；disable=rejected (activation_binding_unavailable,sidechain_or_detector_section_excluded)；remove=rejected (section_not_deallocatable,sidechain_or_detector_section_excluded)；undo=rejected (forward_control_unavailable,sidechain_or_detector_section_excluded)
- `high_shelf`：upsert=rejected (sidechain_or_detector_section_excluded)；modify=rejected (sidechain_or_detector_section_excluded)；disable=rejected (activation_binding_unavailable,sidechain_or_detector_section_excluded)；remove=rejected (section_not_deallocatable,sidechain_or_detector_section_excluded)；undo=rejected (forward_control_unavailable,sidechain_or_detector_section_excluded)
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### CLA MixHub Stereo

- `bell`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (sidechain_or_detector_section_excluded)；modify=rejected (sidechain_or_detector_section_excluded)；disable=rejected (activation_binding_unavailable,sidechain_or_detector_section_excluded)；remove=rejected (section_not_deallocatable,sidechain_or_detector_section_excluded)；undo=rejected (forward_control_unavailable,sidechain_or_detector_section_excluded)
- `high_shelf`：upsert=rejected (sidechain_or_detector_section_excluded)；modify=rejected (sidechain_or_detector_section_excluded)；disable=rejected (activation_binding_unavailable,sidechain_or_detector_section_excluded)；remove=rejected (section_not_deallocatable,sidechain_or_detector_section_excluded)；undo=rejected (forward_control_unavailable,sidechain_or_detector_section_excluded)
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### Curves AQ Live Stereo

- `bell`：upsert=rejected (stateful_surface_dependency_unresolved)；modify=rejected (stateful_surface_dependency_unresolved)；disable=rejected (stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,stateful_surface_dependency_unresolved)
- `low_shelf`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)
- `low_cut`：upsert=rejected (stateful_surface_dependency_unresolved)；modify=rejected (stateful_surface_dependency_unresolved)；disable=rejected (activation_binding_unavailable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,stateful_surface_dependency_unresolved)
- `high_cut`：upsert=rejected (stateful_surface_dependency_unresolved)；modify=rejected (stateful_surface_dependency_unresolved)；disable=rejected (activation_binding_unavailable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,stateful_surface_dependency_unresolved)

### Curves AQ Stereo

- `bell`：upsert=rejected (stateful_surface_dependency_unresolved)；modify=rejected (stateful_surface_dependency_unresolved)；disable=rejected (stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,stateful_surface_dependency_unresolved)
- `low_shelf`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)
- `low_cut`：upsert=rejected (stateful_surface_dependency_unresolved)；modify=rejected (stateful_surface_dependency_unresolved)；disable=rejected (activation_binding_unavailable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,stateful_surface_dependency_unresolved)
- `high_cut`：upsert=rejected (stateful_surface_dependency_unresolved)；modify=rejected (stateful_surface_dependency_unresolved)；disable=rejected (activation_binding_unavailable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,stateful_surface_dependency_unresolved)

### EMO-F2 Stereo

- `bell`：upsert=rejected (required_gain_binding_absent,shape_not_provably_reachable)；modify=rejected (required_gain_binding_absent,shape_not_provably_reachable)；disable=rejected (required_gain_binding_absent,shape_not_provably_reachable)；remove=rejected (required_gain_binding_absent,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,required_gain_binding_absent,shape_not_provably_reachable)
- `low_shelf`：upsert=rejected (required_gain_binding_absent,shape_not_provably_reachable)；modify=rejected (required_gain_binding_absent,shape_not_provably_reachable)；disable=rejected (required_gain_binding_absent,shape_not_provably_reachable)；remove=rejected (required_gain_binding_absent,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,required_gain_binding_absent,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (required_gain_binding_absent,shape_not_provably_reachable)；modify=rejected (required_gain_binding_absent,shape_not_provably_reachable)；disable=rejected (required_gain_binding_absent,shape_not_provably_reachable)；remove=rejected (required_gain_binding_absent,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,required_gain_binding_absent,shape_not_provably_reachable)
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### EMO-Q4 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### F6-RTA Stereo

- `bell`：upsert=rejected (dynamic_section_excluded)；modify=rejected (dynamic_section_excluded)；disable=rejected (dynamic_section_excluded)；remove=rejected (dynamic_section_excluded,section_not_deallocatable)；undo=rejected (dynamic_section_excluded,forward_control_unavailable)
- `low_shelf`：upsert=rejected (dynamic_section_excluded)；modify=rejected (dynamic_section_excluded)；disable=rejected (dynamic_section_excluded)；remove=rejected (dynamic_section_excluded,section_not_deallocatable)；undo=rejected (dynamic_section_excluded,forward_control_unavailable)
- `high_shelf`：upsert=rejected (dynamic_section_excluded)；modify=rejected (dynamic_section_excluded)；disable=rejected (dynamic_section_excluded)；remove=rejected (dynamic_section_excluded,section_not_deallocatable)；undo=rejected (dynamic_section_excluded,forward_control_unavailable)
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### F6 Stereo

- `bell`：upsert=rejected (dynamic_section_excluded)；modify=rejected (dynamic_section_excluded)；disable=rejected (dynamic_section_excluded)；remove=rejected (dynamic_section_excluded,section_not_deallocatable)；undo=rejected (dynamic_section_excluded,forward_control_unavailable)
- `low_shelf`：upsert=rejected (dynamic_section_excluded)；modify=rejected (dynamic_section_excluded)；disable=rejected (dynamic_section_excluded)；remove=rejected (dynamic_section_excluded,section_not_deallocatable)；undo=rejected (dynamic_section_excluded,forward_control_unavailable)
- `high_shelf`：upsert=rejected (dynamic_section_excluded)；modify=rejected (dynamic_section_excluded)；disable=rejected (dynamic_section_excluded)；remove=rejected (dynamic_section_excluded,section_not_deallocatable)；undo=rejected (dynamic_section_excluded,forward_control_unavailable)
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### GEQ Classic Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### GEQ Modern Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### H-EQ-Light Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### H-EQ Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### Kramer HLS Stereo

- `bell`：upsert=rejected (gain_law_not_arbitrary_bipolar)；modify=rejected (gain_law_not_arbitrary_bipolar)；disable=rejected (activation_binding_unavailable,gain_law_not_arbitrary_bipolar)；remove=rejected (gain_law_not_arbitrary_bipolar,section_not_deallocatable)；undo=rejected (forward_control_unavailable,gain_law_not_arbitrary_bipolar)
- `low_shelf`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；modify=rejected (frequency_anchor_unavailable,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)；remove=rejected (frequency_anchor_unavailable,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,frequency_anchor_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### LinEQ Broadband Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### LinEQ Lowband Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### Magma Channel Strip Stereo

- `bell`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### MannyM EQ Stereo

- `bell`：upsert=quantized；modify=quantized；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact

### PuigTec EQP1A Stereo

- `bell`：upsert=rejected (coupled_analog_network_excluded,shape_not_provably_reachable)；modify=rejected (coupled_analog_network_excluded,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,coupled_analog_network_excluded,shape_not_provably_reachable)；remove=rejected (coupled_analog_network_excluded,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (coupled_analog_network_excluded,forward_control_unavailable,shape_not_provably_reachable)
- `low_shelf`：upsert=rejected (coupled_analog_network_excluded,shape_not_provably_reachable)；modify=rejected (coupled_analog_network_excluded,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,coupled_analog_network_excluded,shape_not_provably_reachable)；remove=rejected (coupled_analog_network_excluded,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (coupled_analog_network_excluded,forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (coupled_analog_network_excluded,shape_not_provably_reachable)；modify=rejected (coupled_analog_network_excluded,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,coupled_analog_network_excluded,shape_not_provably_reachable)；remove=rejected (coupled_analog_network_excluded,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (coupled_analog_network_excluded,forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### PuigTec MEQ5 Stereo

- `bell`：upsert=rejected (gain_law_not_arbitrary_bipolar)；modify=rejected (gain_law_not_arbitrary_bipolar)；disable=rejected (activation_binding_unavailable,gain_law_not_arbitrary_bipolar)；remove=rejected (gain_law_not_arbitrary_bipolar,section_not_deallocatable)；undo=rejected (forward_control_unavailable,gain_law_not_arbitrary_bipolar)
- `low_shelf`：upsert=rejected (gain_law_not_arbitrary_bipolar,shape_not_provably_reachable)；modify=rejected (gain_law_not_arbitrary_bipolar,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,gain_law_not_arbitrary_bipolar,shape_not_provably_reachable)；remove=rejected (gain_law_not_arbitrary_bipolar,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,gain_law_not_arbitrary_bipolar,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (gain_law_not_arbitrary_bipolar,shape_not_provably_reachable)；modify=rejected (gain_law_not_arbitrary_bipolar,shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,gain_law_not_arbitrary_bipolar,shape_not_provably_reachable)；remove=rejected (gain_law_not_arbitrary_bipolar,section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,gain_law_not_arbitrary_bipolar,shape_not_provably_reachable)
- `low_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### Q10 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### Q1 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### Q2 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### Q3 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### Q4 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### Q6 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### Q8 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### Q-Clone Stereo

- `bell`：upsert=rejected (no_candidate_section)；modify=rejected (no_candidate_section)；disable=rejected (no_candidate_section)；remove=rejected (no_candidate_section)；undo=rejected (no_candidate_section)
- `low_shelf`：upsert=rejected (no_candidate_section)；modify=rejected (no_candidate_section)；disable=rejected (no_candidate_section)；remove=rejected (no_candidate_section)；undo=rejected (no_candidate_section)
- `high_shelf`：upsert=rejected (no_candidate_section)；modify=rejected (no_candidate_section)；disable=rejected (no_candidate_section)；remove=rejected (no_candidate_section)；undo=rejected (no_candidate_section)
- `low_cut`：upsert=rejected (no_candidate_section)；modify=rejected (no_candidate_section)；disable=rejected (no_candidate_section)；remove=rejected (no_candidate_section)；undo=rejected (no_candidate_section)
- `high_cut`：upsert=rejected (no_candidate_section)；modify=rejected (no_candidate_section)；disable=rejected (no_candidate_section)；remove=rejected (no_candidate_section)；undo=rejected (no_candidate_section)

### RChannel Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### REQ 2 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### REQ 4 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### REQ 6 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### Scheps 73 Stereo

- `bell`：upsert=quantized；modify=quantized；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=quantized；modify=quantized；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### Scheps Omni Channel 2 Stereo

- `bell`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### SSL EV2 Channel Stereo

- `bell`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact

### SSLChannel Stereo

- `bell`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact

### SSLEQ Stereo

- `bell`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (required_gain_binding_absent,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (required_gain_binding_absent,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=exact；modify=exact；disable=exact；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### SSLGChannel Stereo

- `bell`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=exact；modify=exact；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact

### TRACT LinPhase Stereo

- `bell`：upsert=rejected (stateful_surface_dependency_unresolved)；modify=rejected (stateful_surface_dependency_unresolved)；disable=rejected (stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,stateful_surface_dependency_unresolved)
- `low_shelf`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)
- `low_cut`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)
- `high_cut`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)

### TRACT Stereo

- `bell`：upsert=rejected (stateful_surface_dependency_unresolved)；modify=rejected (stateful_surface_dependency_unresolved)；disable=rejected (stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,stateful_surface_dependency_unresolved)
- `low_shelf`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)
- `low_cut`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)
- `high_cut`：upsert=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；modify=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；disable=rejected (shape_not_provably_reachable,stateful_surface_dependency_unresolved)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable,stateful_surface_dependency_unresolved)

### VEQ3 Stereo

- `bell`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `high_shelf`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)
- `low_cut`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=rejected (shape_not_provably_reachable)；modify=rejected (shape_not_provably_reachable)；disable=rejected (activation_binding_unavailable,shape_not_provably_reachable)；remove=rejected (section_not_deallocatable,shape_not_provably_reachable)；undo=rejected (forward_control_unavailable,shape_not_provably_reachable)

### VEQ4 Stereo

- `bell`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_shelf`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `high_shelf`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `low_cut`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact
- `high_cut`：upsert=quantized；modify=quantized；disable=rejected (activation_binding_unavailable)；remove=rejected (section_not_deallocatable)；undo=exact

## 10. 稳定拒绝码

- `activation_binding_unavailable`：The section has no proven disable operation.
- `coupled_analog_network_excluded`：Split boost/attenuate or another coupled analog network is outside generic static EQ.
- `dynamic_section_excluded`：Dynamic peripherals are local to this section, so static independence is not proven.
- `forward_control_unavailable`：No supported forward edit exists from which an undo operation could have been journalled.
- `frequency_anchor_unavailable`：Neither a writable frequency binding nor a recoverable fixed anchor exists.
- `frequency_out_of_reachable_domain`：The target frequency is outside every proven section domain.
- `gain_law_not_arbitrary_bipolar`：The gain law cannot prove arbitrary positive and negative dB targets.
- `gain_out_of_reachable_domain`：The target gain is outside the proven physical domain.
- `linked_stereo_contract_incomplete`：The linked-stereo role bindings are asymmetric or incomplete.
- `no_candidate_section`：No local section is a candidate for this shape.
- `physical_domain_unavailable`：An explicitly requested physical value has no recoverable domain or enum mapping.
- `q_out_of_reachable_domain`：The target Q is outside the proven physical domain.
- `repeated_bank_address_unstable`：A repeated bank has no stable address key.
- `required_gain_binding_absent`：Bell and shelf requests require a gain binding.
- `required_q_binding_absent`：The request explicitly supplied Q but the selected section exposes no Q binding.
- `required_slope_binding_absent`：The request explicitly supplied slope but the selected cut exposes no slope binding.
- `section_not_deallocatable`：Only an allocatable slot can be removed; resident and anchored sections can only be disabled or restored.
- `shape_not_provably_reachable`：The requested filter shape is not proven by names, enums, or local structure.
- `sidechain_or_detector_section_excluded`：A compressor side-chain or detector filter is not an audible main-EQ section.
- `slope_out_of_reachable_domain`：The target slope is outside the proven physical domain.
- `stateful_surface_dependency_unresolved`：Capture, match, learn, or calibration state is present and independence is unproven.
- `valid_control_ref_required`：Modify, remove, and precise disable operations require a current topology-scoped control_ref.
- `valid_operation_ref_required`：Undo requires a valid operation_ref and an unchanged topology generation.

## 11. 分阶段实施建议

1. 先引入只读 `EQControlTopology` 与逐 section shape capability，不改变现有执行。
2. 引入 request-dependent planner，并用 shadow comparison 对照旧三分类。
3. 接入单 edit 原子执行、严格字段与物理回读。
4. 接入 `control_ref` / `operation_ref` 和 modify/disable/undo。
5. 最后接入多 edit 资源预留与全批次回滚，再冻结 Plugin Alliance 外部测试集做验证。

## 12. 产物与可复现命令

- 输入 run：`D:\Vit_DAW\artifacts\waves_eq_topology_census\20260727_201627`
- 输出目录：`D:\Vit_DAW\artifacts\waves_eq_topology_census\20260727_201627\analysis\static_eq_phase2`
- 分析：`python scripts/waves_static_eq_requirement_cluster.py --run-dir <run> --corpus scripts/waves_static_eq_requirement_corpus.json --report-path docs/WAVES_STATIC_EQ_RECOGNIZER_PHASE2.md`
- 验收：`python scripts/waves_static_eq_requirement_acceptance.py --repo-root <repo> --run-dir <run> --corpus scripts/waves_static_eq_requirement_corpus.json --report docs/WAVES_STATIC_EQ_RECOGNIZER_PHASE2.md`

## 13. 停止点

第二阶段到此结束。本阶段没有修改生产识别器、执行器、工具 API 或生产测试；未经用户再次确认，不开始生产开发。
