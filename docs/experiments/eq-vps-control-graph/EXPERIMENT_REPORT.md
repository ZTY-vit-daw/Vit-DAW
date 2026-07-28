# EQ VPS Control Graph 通用平台原型实验报告

## 结论

原型达到了本实验定义的“外部 VPS 补足控制、用户不修改底层代码”目标。

`vit.eq_vps.control_graph.v1` 能用有限、声明式表达式描述一个公共 EQ
请求如何展开成任意有限组插件参数目标。运行时不识别插件名称、厂商、参数
ID，也不识别某个命名的复合 Gain 结构。编译结果仍进入既有
`plugin_grabber.apply_eq_edits` 原子执行器；前像、批量写、fresh readback、
意外副作用检查、失败回滚、正式撤销和零漂移检查没有下放给 VPS。

三种结构已经由同一平台实测：

- Q10：普通双极 Gain，10 个固定槽位，Frequency/Gain/Q/Shape/Activation；
- museq：非负 Gain 幅度加 Boost/Cut 极性，以及定向 Shelf/Cut。
- AMEK EQ 250：左右镜像参数写入，以及 Q 数值域前置 `Shelf` sentinel。

第二个插件的运行没有引入第二个运行时分支。两个插件之间只替换 VPS、
verification 和实验输入数据，`agent/` 与通用 smoke runner 没有插件特例。

## 工作树与范围

- 分支：`codex/eq-vps-control-graph`
- 基线：`39e4c785e7c73ad753105038f871eccefdf8339c`
- 工作树：`D:\Vit_DAW_worktrees\eq-vps-control-graph`
- Q10 范文：`D:\Vit_DAW\docs\VPS_REFERENCE_Q10_STEREO.md`
- 未修改生产 EQ 识别规则；未开发音频主动探测、抽象 EQ 语义、B4、profile、
  learn 或 SPAL。

## 平台边界

Control Graph 是外部数据，不是脚本。JSON 使用 unknown-field rejection；
表达式只有常量、输入、有限算术/比较、`present`、布尔运算、`select` 和
`lookup`。没有循环、函数调用、文件/网络、进程或主机命令。文件大小、目录
条目、section、binding、curve、enum、program、input、表达式深度/节点和
write 数量都有硬上限。

加载时会用 fresh Vit surface 重新核对：

1. 插件名称、厂商、格式和版本；
2. 完整参数 surface signature；
3. 每个引用参数的 ID 与名称；
4. 每个数值曲线点；
5. 每个枚举 normalized 值与标签；
6. VPS 哈希及 verification sidecar。

同名插件的版本或厂商变化、签名变化、ID/名称/曲线/枚举变化、sidecar
缺失或哈希过期都会 fail-closed。`explain_controls` 与 `apply_eq_edits`
采用同一失败策略。目录中没有匹配 VPS 时才回到原有通用识别器。

## 新 VPS 实际制作流程

1. Agent 读取通用 EQ 要求和 Q10 范文，得到文档组织与安全约束；
2. Vit 对真实安装实例执行完整参数分页及只读 value-to-string 观察；
3. Agent 将观察事实组合为 section、shape、action contract 和表达式；
4. `eqvps candidate` 逐项核对完整分页并生成 hash-bound candidate sidecar；
5. 隔离环境启用 candidate，运行通用 A/B/C runner；
6. `eqvps promote` 只在 apply/readback、正式撤销、卸载恢复、最终零漂移和
   禁止项审计全部通过后生成 verified sidecar；
7. 正式加载不允许 candidate 测试开关。

因此，仅凭 VST 文档可以写语义草图，但不能产生可安装 VPS。实时参数 ID、
版本、surface signature、枚举 normalized 值、显示曲线、量化和实际写入行为
仍必须由 Vit 观察。烟测是从 observed candidate 晋升到 verified 的必要步骤，
不是让 Agent 借烟测猜参数语义的手段。

## 数据文件与生成比例

| 文档 | 大小 | 行数 | bindings | sections | fresh 静态检查 |
|---|---:|---:|---:|---:|---:|
| `elysia_museq_mix.vps.json` | 26,029 bytes | 1,155 | 17 | 5 | 102 |
| `q10_stereo.vps.json` | 95,498 bytes | 4,269 | 50 | 10 | 324 |
| `amek_eq_250.vps.json` | 13,122 bytes | 352 | 16 | 3 | 98 |

三份 JSON 的序列化和字段录入均由 Agent 完成：自动生成 100%，人工逐字段录入
0%。这个比例不代表语义判断也已自动化。来源必须分开记录：

- Vit observed：身份、版本、签名、全部引用 ID/名称、曲线、枚举值与标签；
- Agent inferred：section 边界、shared channel、公共 shape、action contract、
  museq 的 `abs(gain_db)` 与正负条件选择 Boost/Cut、always-active、排除项；
- human supplied target fields：0；用户提供的是目标、边界和 Q10 范文；
- smoke verified：写入目标、实际读回、量化、拒绝、正式撤销和卸载恢复。

人工若参与产品流程，应只承担发布审核或纠正语义，而不应手抄 ID、曲线和
枚举。无法由证据证明的关系必须写入 `unresolved`，不能交给用户猜测。

Q10 文档明显偏长，是 v1 的一个真实代价：10 个相同槽位尚无安全模板/继承
压缩机制。该结构能够表达，但冗余较高；产品化前可考虑纯数据宏展开，同时
保持展开后文档可验证且不增加运行时可编程能力。

## museq A/B/C

最终正式 sidecar（未启用 candidate override）证据：
`artifacts/eq_vps_control_graph/20260728_231614/summary.json`。

| 阶段 | 请求 | 结果 | 证据 |
|---|---|---|---|
| A 无 VPS | Bell 3400 Hz / -3 dB / Q 0.5 | `not_static_eq` | 0 写入、零漂移 |
| A 无 VPS | Low Cut 80 Hz | `not_static_eq` | 0 写入、零漂移 |
| B 有 VPS | Bell 3400 Hz / -3 dB | `exact` | Top Frequency 实读 3395 Hz；Gain 幅度 3.0 dB；Mode=`Cut`；正式 undo 恢复 3 参数 |
| B 有 VPS | 同请求显式 Q=0.5 | `explicit_field_unavailable` | 写入前拒绝、零漂移 |
| B 有 VPS | Low Cut 80 Hz | `exact` | Type=`Cut`、Frequency=80 Hz；activation=`always_active`；正式 undo 恢复 2 参数 |
| B 有 VPS | Low Cut 500 Hz | `explicit_field_unavailable` | 超出 Low section 9–200 Hz，写入前拒绝 |
| C 卸载 | 与 A 相同请求 | 恢复 `not_static_eq` | 最终全参数零漂移 |

`always_active` 是明确能力边界：museq 没有段级 activation 参数，因此 Low
Cut 的验证是“Shape 写入 + section 始终有效”，不是伪造一个开关。

## Q10 A/B/C

最终正式 sidecar 证据：
`artifacts/eq_vps_control_graph/20260728_231703/summary.json`。

| 阶段 | 结果 |
|---|---|
| A 无 VPS | 原有 `generic_structural`，10 sections；Bell 与 Low Cut 均成功并正式撤销 |
| B 有 VPS | `vps_control_graph`，仍为 10 sections |
| B Bell | 3396 Hz / -3 dB / Q 0.5；实际 3392 Hz / -3.0 dB / 0.5；Shape=`Bell`、Activation=`In`；撤销恢复 5 参数 |
| B Low Cut | 80 Hz；实际 79 Hz、Shape=`Hi-Pass`、Activation=`In`；撤销恢复 3 参数 |
| B 越界 | 30000 Hz 在写入前拒绝并保持零漂移 |
| C 卸载 | topology 精确恢复为 A 的 `generic_structural` generation；最终零漂移 |

Q10 与 museq 的 B 阶段只有数据文件不同，证明普通双极 Gain 和复合
Gain/Mode 使用同一个表达式编译器及原子执行器。

## AMEK EQ 250 A/B/C

最终正式 sidecar 证据：
`artifacts/eq_vps_control_graph/20260728_233913/summary.json`。

- A 无 VPS：原 `generic_structural` topology 保持；Low Cut 80 Hz 单侧写入成功并正式撤销；
  Bell 3400 Hz / -3 dB / Q 0.5 因错误落到带 `Shelf` sentinel 的 Q 读回而安全回滚。
- B 有 VPS：发布 3 个可证明 section、16 个 bindings；Bell 在左右两侧分别精确读回
  3400 Hz、-3.0 dB、Q 0.5、Activation=`In`，正式 undo 恢复 8 个参数。
- B Low Cut：左右两侧均精确读回 80 Hz 与 Activation=`In`，正式 undo 恢复 4 个参数。
- B 越界：Low Cut 15000 Hz 在写入前以 `explicit_field_unavailable` 拒绝并保持零漂移。
- C 卸载：精确恢复 A 的 `generic_structural` generation、原 Bell 拒绝与原 Low Cut 能力；
  最终完整 83 参数零漂移。

本次没有改动任何 Go 文件或通用烟测脚本。LF/HF 的同一 Q 参数混合 `Shelf` sentinel
与数值 Q，v1 无法用一个 binding 同时安全表达两种类型，因此保持 `unresolved`；没有为通过
测试而扩展 schema 或执行器。

## 无法表达或故意不表达

- museq Bottom/Middle/Top Q 只有 `Wide/Narrow`，没有证据把它们换算为公共
  数值 Q；v1 保持显式 Q 拒绝；
- museq 没有段级 disable/remove；全局 Active/Bypass 不能冒充段 activation；
- 无名参数没有足够语义证据，保持排除；
- v1 不表达动态频谱、音频依赖行为、连续时间逻辑、循环、状态机或任意代码；
- Q10 的 PQ-Bell 不属于既定五种公共 shape，保持未发布；
- v1 可以表达有限 fan-out、条件与查表关系，但不允许 VPS 改写事务、安全、
  rollback 或 undo 规则。
- 自定义数值 `role` 仍会接受精确 normalized readback 校验，但现有执行器的物理量
  解析、容差与迭代校正只对 `freq`、`gain`、`q`、`slope` 四类规范 role 最完整。
  本实验的 Q10 与 museq 数值写入均使用这些规范 role；产品化前应将其定为发布约束，
  或另行设计可验证的通用物理读回契约，不能由 VPS 自行降低校验强度。

## 是否值得产品化

值得继续做受限产品化设计，但不应直接把原型 schema 当成最终公开格式。
正面证据是：外部文档补足了当前识别器明确缺失的复合结构；加载/卸载可逆；
两个结构复用同一执行器；正式安装不需要 Agent 更新。

产品化前仍需决定：schema 迁移与兼容政策、签名/发布者信任、用户目录安装
体验、数据模板压缩、verification evidence 的可携带路径、版本升级后的重新
观察流程，以及面向用户的语义审核界面。按本实验边界，到报告完成即停止，
不继续扩展生产识别规则、B4、profile、learn、SPAL 或音频探测。
