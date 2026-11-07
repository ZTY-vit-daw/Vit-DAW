# VSTI-PARAM-PROBE-1：音源参数控制探针（控制实证先行，准入后置链第 2 位）

- 发卡：GLM 主管决策流 / 2026-11-07 午前（依据=[用户裁定 2026-11-07：PCA 准入后置于控制实现](../../decisions/2026-11-07-pca-admission-after-control.md)+[277 段 3a-r2 证据](../done/2026-11-05-ARR-SRCLOAD-1.md)——装载已实证/参数控制未做；效果器族先例=M2-TIMEFX-1 参数面探测（内核 ZMQ 直连装载→get_plugin_parameters→apply/readback/restore/快照核对/临时轨清理自含）与 G3-PROBE-METER 链）
- 派发确认：已确认（用户 2026-11-07「这些足够进行实验吗？如果足够完全可以进行实验开发」=现有音源开展控制实验授权）；验收负责人：GLM 主管决策流
- 池序 298；目标仓库=主仓 agent（探测腿+工件，复用 M2-TIMEFX 探针模式）；P1 / 0.5-1 天 / 依赖=无硬依赖（**与 297 渲染取证可并行**——参数控制不依赖出声链）；L1-L2 / PC；真栈占用一次（登记协议照旧，候栈排队与 296/297 错峰）

## 目标

1. **两件音源参数控制实证**（Grand Rhapsody Piano Stereo 首选+Surge XT 对照，均本机在库）：装载（既有链）→ `get_plugin_parameters` 全量参数清单落工件（含范围/默认值/枚举）→ 选 2-3 个可写参数做 apply→readback→restore 往返（写前快照+写后核对+恢复核对，M2-TIMEFX 自含模式）；
2. **控制证据工件**（准入后置链的输入）：per-plugin 参数清单+往返记录+版本指纹（processorattestation.FingerprintPath 同源）落 `coord/runs/VSTI-PARAM-PROBE-1/`——**本卡不写 attestation 库**（准入届时另卡，用户裁定）；
3. 差异观察：VSTi 参数面与效果器族的结构差异（预设/音色切换参数？MIDI 通道暴露？）如实记录（编曲线后续语义控制的素材）；
4. §8 跑前成文（确定性腿预期零 LLM；若走 chat 驱动装载变体则 flash 预算 ≈¥1-3）。

## 约束

- 文件域=探测脚本（复用/扩 M2-TIMEFX 同款 ps1/py 模式）+工件；**源码零改动预期**（如装载/参数接口暴露缺口属产品缺陷→取证转卡不扩修）；
- PCA attestation 库零写入（准入后置裁定）；kernel 零改动；
- 临时轨/插件实例清理自含（M2-TIMEFX 先例：即建即删）。

## 停止条件

参数接口对 VSTi 不可用（如 get_plugin_parameters 对 instrument 返回空/报错）→取证上交（=「尚不可控制」的如实证据，直接改变准入链评估）；装载环节回归失败→关联 297 取证分列。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
