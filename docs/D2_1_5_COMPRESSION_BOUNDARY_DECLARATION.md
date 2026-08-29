# 压缩域能力边界声明（D2-1.5-S2f 家族裁定转写）

**现行·边界声明（2026-08-29）**

本文是 S2f 家族（压缩域问题类边界）已冻结裁定的忠实转写，docs-only，不含新裁定。关键裁定逐字引用自下列权威源，口径以源为准；有出入时以源为准：

- 母卡：`queue/todo/2026-08-28-D2-1-5-S2f-compression-instance-binding.md`（"用户裁定"、"修订后设计"、"撤后路线"、"S2f-1 实验记录"、"分层"各节）
- S2f-3 卡：`queue/done/2026-08-29-D2-1-5-S2f3-context-forensics.md`（"GLM 裁定"节）
- S2f-3 报告：`queue/reports/2026-08-29-s2f3-context-forensics.md`

## ① 不可逆性边界

用户裁定（2026-08-28 晚，音频专业判断）逐字：

> **压缩对已烘焙素材是不可逆操作**：素材导出时动态即已丢失（"蛋糕切开粘不回去，且我们手里只有被切开的这一半"）。"过压缩"作为问题类，对压缩域**不可修复**——不存在"反压缩"动作。LLM 提案"+1 threshold 减压"是概念层错误：它以为压缩域有逆向动作。真正的工程解法（EQ/扩展器塑造新阈值、再创造压缩弹性）是复杂多步策略，**超出当前"单步有界动作"能力模型**，声明为已知边界而非假装可解。GLM 方向 2'（引导负增量）被否决：引导模型做声学上无意义的动作等于奖励"看起来合理的空转"，A/B 兜底的诚实失败不等于理解。

母卡"分层"节第 5 条（本卡的直接出处）逐字：

> **边界声明文档（不变）**：多步弹性重建（EQ/扩展器塑形）超出当前单步能力模型，远期方向记录；亦记录"垂类自训时本域轨迹即训练语料"。

即：多步弹性重建（EQ/扩展器塑形、再创造压缩弹性）超出当前单步有界动作能力模型，声明为已知边界而非假装可解；远期方向仅记录，不排实现。

## ② 认知路线结论

（a）**prompt 激活问句四轮对照实验证伪**（S2f-1，已按协议 revert 92bfe21）。母卡"S2f-1 实验记录"节逐字：

> 基线（无问句态，HEAD=bc68f19）：20260828_210428 / 210716，**2/2 fail 且形态完全一致**——模型提案 threshold +1，delta 机器诚实拒绝 "delta target 12.8 dB (current 11.8 + 1) is outside the reachable normalized range"。基线行为确定性成立（概念性 +1 提案 → 物理墙）。

> **实验结果（有效干预臂）**：212058 / 212401，2/2 与基线**完全一致**——模型仍提 threshold_db +1，物理拒绝同文本。布线已核实（ccb_model_prompt.go:144 渲染 PromptParameterHint，二进制重建确认）。

> **初判（n=2/臂，签名确定性）**：激活问句零效果。知识要么非该模型类潜伏可检索，要么场景框架（flavor prompt 的命令式"给一个有界的小步改进建议"）压倒软问句。按用户协议（好则留差则撤）：**建议 revert 问句行**；防御回归物理 fail-closed + A/B 兜底；域可测性由 S2f-2 正向用例承载。

（b）**crest/RMS/peak 客观测量在场、模型仍提 +1**（S2f-3 机制链取证）。S2f-3 卡"GLM 裁定"节第 1 条逐字：

> **认知路线（p01+compression 四轮）关闭**。crest/RMS/peak 客观测量在场（链 A 热层 + 链 B 暖层，四轮一致），模型握着 basic_energy 数据仍提 threshold +1；结合 S2f-1 软问句零效果，"缺数据"与"prompt 未激活"两条解释均被排除。不可逆性知识缺口归宿=垂类自训（失败轨迹即 SFT 语料）。

具体数值（S2f-3 报告 §2）：目标轨 track 1012 的 `crest_db=15.611`（210428/212058/212401 三轮）、track 1007 的 `crest_db=14.383`（210716 轮），连同 rms/peak/headroom 数值同请求在场；证据成立的机制链（报告 §3）：链 A 热层 active_observation（message_loop.go:466 → helpers.go:60 → model_projection.go:1268-1272）+ 链 B 暖层 observation_ledger（free_state_reasoning.go:1544 → model_projection.go:321/:685-687），四轮一致。GLM 裁定第 4 条同时关闭了补实证的口子（逐字）：

> 插桩转储（字节级请求体实证）**不必要**——机制链证据已足裁定。

（c）**结论：此类领域物理知识须来自训练（基座进化或垂类自训），prompt 路线关闭。** 母卡两处裁定逐字：

> 用户裁定要义：压缩不可逆性这类领域物理本应落在 LLM 参量里（模型知识是潜伏的、缺的是触发不是内容）；未来靠基座进化或自训垂类 LLM 拥有。

> **发现的价值**：本结果是"极简激活式提示无法触发不可逆性推理"的能力边界证据点——支持"此类知识须来自训练（基座进化/垂类自训）"的论点，轨迹照录即为未来 SFT 语料。

## ③ 训练语料注记

p01+compression 失败轨迹（概念性 +1 提案 → delta 机器诚实拒绝）照录即为未来垂类 SFT 语料。母卡红线逐字：

> 红线：不加声明式物理知识条目；prompt 增量总量以"一行问句"为上限；失败样本照录不美化（未来 SFT 语料价值）。

失败轨迹形态（照录自"S2f-1 实验记录"）：模型提案 threshold +1，delta 机器诚实拒绝 "delta target 12.8 dB (current 11.8 + 1) is outside the reachable normalized range"；四轮有效采样形态完全一致。本域实验产物目录与四轮 stamp 列表：

- 产物目录：`D:\Vit_DAW\artifacts\free_state_d1_s1\`

| stamp | 臂 | 说明 |
|---|---|---|
| 20260828_210428 | 基线（无问句态） | HEAD=bc68f19 |
| 20260828_210716 | 基线（无问句态） | HEAD=bc68f19 |
| 20260828_212058 | 干预（有问句） | 含构建钉 HEAD=53f851d |
| 20260828_212401 | 干预（有问句） | 复用新二进制 |

（另：首轮干预采样 20260828_211324 / 211639 **无效作废**——"-SkipBuild 复用了 S2f-1 落地前的 agent 二进制（dev_agent_smoke.ps1:398-430 缓存语义），问句未进模型"，不作为语料证据轮。）

S2f-3 取证报告同属本域实验产物：`queue/reports/2026-08-29-s2f3-context-forensics.md`（四轮 `agent_runtime_state.json` 逐轮证据表与机制链引用）。

## ④ 观察层缺口

p01 fixture 上，`track.time_dynamics` 视图（COM source_dynamics 类素材动态特征）被 CCB 以 "source evidence is missing" 拒绝——**模型四轮请求、四轮全拒**。S2f-3 报告结论速览逐字：

> COM `source_dynamics` 结构本体 | **否，从未发生**：模型四轮均显式请求 `track.time_dynamics`，CCB 全部以 "source evidence is missing" 拒绝（disclosure_bytes=2）

S2f-3 卡"GLM 裁定"节第 2、3 条逐字：

> **喂测量对照实验暂缓不开**。严格口径缺口（COM source_dynamics）不是投喂问题而是**证据源头缺失**——模型四轮均显式请求 `track.time_dynamics`，CCB 以 "source evidence is missing" 全部拒绝；渲染潜能在位（model_projection.go:1273-1277 / :688-693）但无源头证据可喂，想喂也无对象。

> **缺口归置并入 S2f-2**：正向 fixture 构建时必须保证 COM source_dynamics 证据可产出（`track.time_dynamics` 可披露）——这同时是 S2f-2 判据机器（crest/LUFS/瞬态密度即测）的数据前提，已作为强制项写入 S2f-2 卡。fixture 证据链完整后，喂测量对照实验才有可执行对象，届时按需另开卡。

证据链补齐归 S2f-2 / S2f-2a（`queue/todo/2026-08-29-D2-1-5-S2f2a-time-dynamics-evidence-gap-forensics.md`：p01 fixture 上 time_dynamics 证据缺失根因勘察，S2f-2 前置）。

对照事实：crest/RMS/peak 客观测量本身**不在此缺口内**——`track.basic_energy` 数值经热层 active_observation + 暖层 observation_ledger 双通道进入请求，四轮一致（见 ②(b)）。

## ⑤ 防御纵深现状

母卡"分层"节引言逐字：

> 分层（防御纵深不变，重保证在代码与真人判定，prompt 只抬开局正确率）：

| 层 | 现状 |
|---|---|
| 1. 激活问句（唯一 prompt 增量，≤25 tokens 英文疑问式，零事实陈述） | 已实验证伪并 revert（S2f-1，92bfe21）——prompt 路线关闭，此层退场 |
| 2. 系统特定语义（域表本分，非音频知识）："本域动作=在透明新实例上施加有界压缩" | 在位 |
| 3. 证据指针（零 prompt 成本） | 在位；渲染潜能在位，p01 证据源头缺失归 S2f-2/S2f-2a（见 ④） |
| 4. 正向用例 fixture（S2f-2） | 进行中；压缩域端到端验收迁此 |
| 5. 边界声明文档 | 即本文 |

底层保证（母卡"撤后路线"节逐字）：

> 用户确认撤（"撤肯定是要撤"）。撤后无需其他立即改动：物理 fail-closed + A/B 兜底已遏制问题（p01+compression 上浪费一个循环但绝不撒谎）。

即：**物理 fail-closed（delta 机器顶格不可达诚实拒绝）+ A/B 兜底在位；p01+compression 降级为观察项**（母卡"分层"节第 4 条："压缩域端到端验收迁此；p01+compression 降级为观察项"）。
