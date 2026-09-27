# DOM-EVIDENCE-PRIORITY-RECON-1：band 证据来源优先级语义勘察（MAT-D 裁定③的决策输入）

- 优先级 / 预估 / 依赖：P1 / 0.3 天 / MAT-D 验收裁定③：回退序修正（特征快照优先、请求测量殿后）是"测量 vs 持久证据"的优先级语义变化，牵动 DAD 哲学与消费面，决策侧亲裁前需事实输入
- 模型分级：L1 / flash 可接（纯只读勘察+报告）
- **执行侧（PC 会话）**
- 目标：
  1. **回退序现状全景**：domInputFromObservation 的 band/waveform 回退序（snapshot 单数键→MixPackage.current_metrics→快照复数键——MAT-D 取证的锚点）逐分支列出触发条件+各分支数据语义（DAD 持久证据 vs 请求轮内测量）；其他投影（mom/com/tim）有无同款"测量遮蔽持久证据"的回退形态（对照盘点）。
  2. **消费面影响**：dom 投影的 band/peak_structure 族字段被谁消费（CCB 投影/LLM ContextPack/mix_read/静态平衡能力门——锚点），若回退序改为快照优先，各消费面拿到的证据形态会怎么变（测量值 vs 快照值的时效差异语义）。
  3. **真实数据分布**：真栈上该回退轴的激活频率量级（从 MAT-D 三个 run 的工件+观察日志估算——每轮观察 band 证据来自测量的比例），影响裁决权重。
  4. **先例对照**：F9 裁定（paired/测量随请求不预计算）与 DAD 证据哲学（持久证据 vs 轮内测量分层）在文档中的原文锚点——为裁决提供宪法依据。
  5. 报告落 `coord/runs/DOM-EVIDENCE-PRIORITY-RECON-1/`。
- 约束：零代码改动；锚点带文件:行；频率估算给方法与量级不给编造精确数。
- 验收：①回退序全景含其他投影对照 ②消费面清单带锚点 ③激活频率量级+方法 ④宪法依据原文锚点 ⑤报告入库
- 停止条件：常规勘察止损
- 领取：2026-09-29（PC / GLM-5.3 L1 执行侧）/ main `git pull` Already up to date（HEAD=58180a41）；领取时工作树既有改动=VitApp/Workspace/default_project.xml（内核运行时状态，非本卡域，未触碰）+ 他卡 coord/runs 未跟踪工件；零代码改动
- 回执：**执行完成，待决策验收。** 报告=`coord/runs/DOM-EVIDENCE-PRIORITY-RECON-1/report.md`，五项验收全落：
  - ①回退序全景：dom 四支逐分支（dom_projection.go:25-48）+ 语义精确化——current_metrics 的 waveform/band_energy 实为**同一 featureSnapshot 单数行的轮内派生视图**（mixboard.go:1219/1239），"测量遮蔽持久证据"实为"单数行派生视图遮蔽复数键目标行"；其他投影对照：com 有同款两支回退（登记型无对账面）、mom 反向兜底、tim/fxm/tom 无该形态；**dom 是唯一"precomputable+MixPackage 回退"投影，分歧结构性集中于它**。
  - ②消费面：CCB 五 view（free_state_observation.go:432-436/638-655/691-701）+ LLM ContextPack CompactFacts 含具体 dB 数值（dom/projection.go:598-632）+ mix_read 全投影（catalog.go:481-485）+ contextruntime（model_projection.go:1312-1313）+ 物化 contentIdentityHash（adapters.go:361-382）；静态平衡/B1 门不直接消费 dom band（走 MOM band_occupancy）。修正③的影响与时效注意点（featureRowForTarget 无最佳行比较，可能取较旧复数行——伴生决策点）已列。
  - ③频率：MAT-D-1 三 run 共 6 轮观察、6 次 reconcile、6 次分歧=**烟测场景 100%**（方法=shadow_round metrics 差分+divergence 行计数，可复算）；多轨工程如实申报无工件不外推，机制判断=结构性暴露、预计低于 100%。
  - ④宪法锚点：F9（MATERIALIZATION_V1_DESIGN.md:25/:96/:230）+ dom 输入域声明与代码的**文档-实现偏差**（adapters.go:301）+ 输入分层词表（:167/:173-176）+ 宪法行（:38）+ peer 拓扑（OBSERVATION_PROJECTION_MANIFEST.md:10-16）。
  - ⑤报告入库 ✓（含支持③/谨慎③的事实权重汇总与勘察边界申报）。
