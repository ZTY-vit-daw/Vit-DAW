# EQ 控制拓扑单调重做研究结论

> 状态：研究与设计完成，生产识别器未按本报告修改。
>
> 分支：`codex/eq-structural-matrix-unification`
> 研究目标：使新识别器成为旧结构识别能力的严格超集，而不是用插件名称、型号白名单或逐插件兼容规则恢复能力。

## 1. 结论

当前 `EQControlTopology` 的问题不是“安全规则太严格”这么简单，而是分层错误：识别器在请求出现之前，就把局部事实压缩成了五个动作布尔值，并让任意辅助参数产生的 exclusion code 否决整个 section。结果同时破坏了三个性质：

1. 旧结构能力不再单调保留；
2. 无关辅助参数影响静态核心；
3. `modify`、`disable`、`remove`、`undo` 被错误地绑定到 `upsert` 是否成立。

重做后的唯一规范数据源必须是“事实拓扑”，不能是预计算 capability 矩阵：

```text
observed parameters
  -> surface/component ownership
  -> local section graph
  -> static-core bindings + auxiliary bindings
  -> shape/addressing/channel/activation/domain facts
  -> request-specific obligations
  -> action plan
  -> atomic execution/readback/rollback
```

三个公开分类名可以继续作为事实拓扑的兼容投影，但不能参与执行决策。

## 2. 差分证据

研究将 `artifacts/eq_compat_matrix/20260727_184532/capture` 中 16 个真实历史 capture 转为带原始 `template_role`、`plugin_class` 与 `plugin_identity` 上下文的离线 replay 输入，再由当前生产 `BuildEQBandSummary` 重放。

- 历史 capture：16；
- 旧真实正向烟测成功：14；
- 旧负例：2（耦合模拟网络与 compressor）；
- 当前仍可形成 EQ 模型：14；
- 当前至少有一种 shape 可执行：12；
- 对旧同一 Bell 请求在 capability gate 前已经确定回退：4。

四个回退结构：

| 结构 | 旧真实请求 | 旧结果 | 当前静态核心 | 当前否决原因 |
|---|---|---|---|---|
| TDR Nova | 3400 Hz / -3 dB / Q 0.5 | 精确写入 Frequency/Gain/Q/Shape | 完整；Bell/Low Shelf/High Shelf 可达 | 同 section 存在动态字段 |
| FreeEQ8 | 3400 Hz / -3 dB / Q 0.5 | 精确写入 Frequency/Gain/Q/Shape | 完整；五种公开 shape 可达 | 同 section 存在 Threshold/Attack/Release 等字段 |
| Pro-Q 3 | 3400 Hz / -3 dB / Q 0.5 | 精确创建 slot；Activation 最后写 | 完整；24 个 allocatable slot；五种公开 shape 可达 | 同 section 存在 Dynamic 与 External Side Chain 字段 |
| F6 | 3400 Hz / -3 dB / Q 0.5 | 旧真实烟测成功 | 静态 Frequency/Gain/Q/Type 完整 | 同 section 存在动态字段 |

如果只把 `dynamic_section_excluded` 和 `sidechain_or_detector_section_excluded` 从 section 级否决改为辅助事实、其余规则保持不变，现有事实已经足以恢复：

- TDR Nova：Bell、Low Shelf、High Shelf；
- FreeEQ8：Bell、Low Shelf、High Shelf、Low Cut、High Cut；
- Pro-Q 3：Bell、Low Shelf、High Shelf、Low Cut、High Cut；
- F6：Bell、Low Shelf、High Shelf；其独立 Low/High Cut 继续保留。

这说明回退不是参数结构无法识别，而是完整结构在 capability 编译阶段被错误否决。

Marvel GEQ 不计入这组回退。它没有可观察的中心频率表，不能仅凭 `1EQ0...1EQ15` 将自然语言 Hz 目标映射到具体 fader；这是频率锚点不可观测问题，而不是动态辅助字段污染问题。

## 3. 当前实现为什么不是单调扩展

### 3.1 两套平行模型发生语义漂移

当前识别器同时生成 `Bands` 与 `Sections`：旧规划器使用 `Bands`，新规划器只使用 `Sections`。两者来自相同参数，却由不同完整性与排除规则处理。兼容入口改走新 planner 后，旧 `Bands` 已经证明的写计划不再构成新 planner 的事实下界。

重做后只能有一个 canonical section graph；旧三分类、旧 band rows、新 action planner 都必须是这个 graph 的投影或消费者。

### 3.2 exclusion 作用域错误

当前算法只要发现某个参数名称含 `threshold`、`attack`、`release`、`ratio`、`range`、`dynamic`，就向所属 section 添加 `dynamic_section_excluded`。`capabilitiesForEQSection` 又把 section 的全部 exclusion codes 放进每个 shape/action 的公共前置集合。

因此“该动态参数不属于通用工具”被错误转换成“该 section 的静态 Frequency/Gain/Q/Shape 不可控制”。

正确作用域是：

```text
parameter role exclusion != component ownership exclusion != request obligation failure
```

- 动态、sidechain、detector 参数默认只是不进入静态 core；
- 只有整个局部 component 被证明属于 detector/sidechain 时，才拒绝该 component；
- 只有请求实际依赖的 core role 缺失、歧义或越界时，才拒绝该请求。

### 3.3 请求无关的动作布尔值表达力不足

当前 `EQShapeCapability` 预计算 `upsert/modify/disable/remove/undo`。但这些动作的依赖并不相同：

- `modify gain` 不需要 Frequency 可写；
- `modify frequency` 不需要 Activation；
- `disable` 只依赖可证明的 inactive 值；
- `remove` 只依赖 allocatable lifecycle；
- `undo` 只依赖运行时 `operation_ref`、postimage 与事务日志，和插件拓扑无关。

把它们预先压缩成布尔值必然产生错误耦合。

### 3.4 聚合指标隐藏局部回退

TDR Nova 和 F6 当前仍因独立 Cut 而被统计为“插件可执行”，但旧 Bell 已经回退。插件级 `any shape` 指标无法验收单调性；验收单位必须是：

```text
parameter digest × concrete request × action plan
```

## 4. 新的事实拓扑

建议内部 schema 升级为 `eq-control-topology/v2`。它只表达事实，不表达最终动作结论。

### 4.1 SurfaceScope

```text
ownership:
  main_audible_eq
  auxiliary_detector_eq
  sidechain_eq
  stateful_or_calibration_surface
  unknown
```

归属证据可使用插件类别/模板角色、display group、参数局部命名、重复块关系和主控制结构，但不能使用插件型号分支。

- EQ 插件中 `Band N External Side Chain` 是 main band 的辅助字段，不会把 main band 变成 sidechain section；
- Channel Strip 中独立 `SC Filter` component 属于 sidechain，不能进入 main EQ；
- compressor 内部滤波器若缺少 main-audible 证据，保持 auxiliary/unknown，不自动提升为通用 EQ。

### 4.2 EQSectionFacts

每个 section 保存以下正交事实：

- `section_key`：来自局部重复结构；
- `ownership`：主 EQ、辅助或未知；
- `addressing`：anchored、resident、allocatable；
- `static_core_bindings`：Frequency、Gain、Q、Filter Kind、Slope、Activation；
- `auxiliary_bindings`：Dynamic Range、Threshold、Attack、Release、Ratio、Sidechain 等；
- `reachable_shapes`：从 dedicated structure 或真实枚举推导；
- `physical_domains`：每个 core binding 的范围、曲线、枚举和值域；
- `channel_contract`：shared、完整 linked mirror 或不安全非对称；
- `activation_contract`：当前状态、active/inactive/unused 可达值和写入顺序；
- `frequency_anchor`：writable、enumerated、fixed-known 或 fixed-unknown；
- `gain_law`：arbitrary bipolar、unipolar、coupled 或 unknown；
- `evidence`：每个事实的参数 ID 和来源。

动态或 sidechain 扩展是 auxiliary facts，不是 section exclusion。

### 4.3 不再存储动作布尔值

Topology 可以公开“观察到哪些字段、哪些 shape 可达”，但不再保存最终 `upsert=true` 等结论。若 UI 需要摘要，只能由标准最小请求即时投影，并明确它是 projection，不是执行门。

## 5. 请求依赖编译器

planner 为每个候选 section 生成 obligations，并仅验证当前请求实际需要的 obligations。

### 5.1 Upsert

| Shape | 必需事实 | 条件事实 |
|---|---|---|
| Bell | main ownership；Bell 可达；可寻址 Frequency；arbitrary bipolar Gain | 请求给 Q 时要求 Q；inactive/allocatable 时要求安全 Activation |
| Low Shelf | main ownership；Low Shelf 可达；Frequency；arbitrary bipolar Gain | 请求给 Q 时要求 Q；必要时 Activation |
| High Shelf | main ownership；High Shelf 可达；Frequency；arbitrary bipolar Gain | 请求给 Q 时要求 Q；必要时 Activation |
| Low Cut | main ownership；Low Cut/High Pass 可达；Frequency | 请求给 Q/Slope 时分别要求对应 binding；必要时 Activation |
| High Cut | main ownership；High Cut/Low Pass 可达；Frequency | 请求给 Q/Slope 时分别要求对应 binding；必要时 Activation |

动态与 sidechain auxiliary bindings 不出现在上述依赖中。

### 5.2 Modify

`modify` 先校验 `control_ref`，然后仅对显式提交字段建立 obligations：

- 改 Gain：只要求 Gain binding 与目标值域；
- 改 Frequency：只要求 writable Frequency，或确认 anchored 只能量化选择而不能移动；
- 改 Q/Slope：只要求对应 binding；
- 改 Shape：只要求目标枚举可达；
- 未修改字段不得被重新写入。

因此 `modify` 不能等同于 `upsert`。

### 5.3 Disable、Remove、Undo

- `disable`：只要求 explicit inactive/disabled/off 值或已证明的 frequency sentinel；
- `remove`：只要求 allocatable lifecycle 中存在 `Used -> Unused`；
- `undo`：从 topology 中移除，归属事务层；要求有效 `operation_ref`、目标一致、generation/postimage 未冲突。

## 6. 动态 EQ 的正确边界

“动态 EQ 不进入通用功能”应解释为：

- 不暴露或写入 Threshold、Range、Attack、Release、Ratio、External Sidechain；
- 不把 Dynamic Range 误绑定成静态 Gain；
- 不尝试创建动态行为或解释动态声学结果；
- 静态 Frequency/Gain/Q/Shape 若独立可写，继续属于通用静态 EQ；
- 执行结果可报告 `preserved_auxiliary_state=true`，明确辅助状态未改变。

只有以下情况才因动态结构拒绝静态请求：

1. 找不到独立静态 Gain，只有 Dynamic Range；
2. 静态与动态参数共享一个不可区分的写入控制；
3. 请求的字段只能通过动态参数实现；
4. component ownership 被证明只属于 detector/sidechain，而非 main audible EQ。

## 7. 固定频率锚点边界

固定频率结构分为：

- `fixed_known`：参数名、枚举标签或其他可观察字段给出中心频率；可以按 Hz 选择并量化；
- `fixed_unknown`：只知道存在一组 gain fader，但中心频率不可观察；可以识别“这是固定频段结构”，不能执行 Hz 定位请求。

Marvel GEQ 属于 `fixed_unknown`。要恢复它的 Hz 控制，必须另行批准一种不依赖插件名称的、经过验证的 topology-signature anchor catalog；这不是本次动态污染回退的修复组成部分，也不能用测试后追加型号规则解决。

## 8. 单调扩展契约

对 digest `D`、请求 `R`，旧规划器与新规划器分别为 `OldPlan(D,R)`、`NewPlan(D,R)`。重做验收条件是：

```text
若 OldPlan(D,R) 曾通过真实 fresh readback，
则 NewPlan(D,R) 必须成功，并产生语义等价或更完整的静态 core 写计划。
```

这不是插件白名单。验证比较的是：

- 请求字段；
- 被选 section 的结构事实；
- 写入 role 与参数 ID；
- physical target；
- activation-last；
- fresh readback；
- rollback/undo。

新 planner 可以增加原子性、引用、范围校验和回滚，但不能因为请求无关的新轴而删除旧成功计划。

允许拒绝旧路径的唯一情况是：存在真实证据证明旧写入不满足请求或违反当前功能边界，例如未知固定频率却报告命中某 Hz、耦合 boost/attenuate 被错误当作 arbitrary bipolar Gain。拒绝必须给出与本次请求直接相关的 contradiction witness。

## 9. 建议的重做顺序

1. 暂停更多新插件烟测；当前矩阵不适合作为泛化评价对象。
2. 保留参数解析、物理域、枚举、事务执行器和引用机制；撤销 section 级动态/sidechain 否决传播。
3. 合并平行的 `Bands`/`Sections` 为一个 canonical fact graph。
4. 将 dynamic/sidechain/detector 拆为 auxiliary bindings，并新增 component ownership。
5. 用 request-specific obligation compiler 替换预计算 action booleans。
6. 将 `disable/remove/undo` 从 `upsert` 的公共前置条件中解耦。
7. 在所有历史 capture 上做旧计划与新计划的结构差分；任何无 contradiction witness 的能力缩小均阻止集成。
8. 先做完全离线 shadow comparison，再做少量旧对象真实恢复烟测。
9. 旧能力集合全部恢复且新增能力验收完成后，才重新开放真正未参与设计的新安装外部测试集。

## 10. 研究产物

- 历史 capture 转换器：`scripts/prepare_eq_monotonicity_replay.py`
- 离线 replay 输入：`artifacts/static_eq_monotonicity_research/20260727/legacy_replay_input`
- 当前生产 replay：`artifacts/static_eq_monotonicity_research/20260727/current_replay.json`
- case 摘要：`artifacts/static_eq_monotonicity_research/20260727/production_replay_cases.csv`
- shape/action 摘要：`artifacts/static_eq_monotonicity_research/20260727/production_replay_capabilities.csv`

## 11. 停止点

本报告完成后停止。本轮没有按上述方案修改生产识别器、planner 或 executor；下一步开发需重新确认。
