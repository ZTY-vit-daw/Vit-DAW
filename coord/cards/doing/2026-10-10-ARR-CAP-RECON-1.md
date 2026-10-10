# ARR-CAP-RECON-1：编曲能力线现状勘察（Mac 线候选②）

- 发卡：GLM 主管决策侧 / 2026-10-10 夜（依据：用户裁定三线放行+Mac 线候选讨论）
- 派发确认：已确认（用户 2026-10-10 裁定）
- 验收负责人：GLM 主管决策流
- 池序 57；目标仓库=D:\Vit_DAW（+前端仓只读）；只读勘察零代码
- 优先级 / 预估 / 依赖：P1 / 1-2h / 无
- 模型分级：L0 / 任意引擎（含闲时）

## 目标（报告四节）

1. **段落投影消费链现状**（编曲的结构输入）：L2-2 段落原语（segmentation_primitives）真栈产出已证——但 agent 侧落盘白名单不含该型（L2-2 烟测发现）；盘点从"内核 DSP 原语→agent 投影→模型可寻址（ref schema 承载）"整链的缺口清单（白名单扩容/投影层/查询面）。
2. **编曲操作面现状**：clip/region 操作命令面（REGION-INTENT-WIRE/REGION-OP-RECON 产出）、MIDI 操作面（piano_roll/MIDI 命令）、时间轴操作——哪些已有命令可复用、哪些缺。
3. **与 PC 混音线的耦合面**：FREESTATE-REGION-IMPL-1（冻存待 B2）与 region/自动化面的交集——评估 Mac 编曲线与 PC 混音线并行时的文件域冲突风险（chat/agentloop 的 region/goal 面）。
4. **启动建议输入**：若 Mac 选编曲——首批 2-3 张卡建议（段落投影消费链应为第一张）。

## 文件域

零代码；报告落 coord/runs/ARR-CAP-RECON-1/。

## 验收标准

四节齐+锚点；耦合面评估给文件域冲突清单（并行安全性输入）。

- 领取：2026-10-10 / origin/main=1c1a447a4315d8b5800c5dd3c4df88dfdc62f915 / owner=GLM-5.3-Flash（ZCode flash 执行会话，PC win32）。零代码勘察未建 worktree，直接在主工作树执行（文件域仅 coord/runs/ARR-CAP-RECON-1/ + 本卡状态；领取提交按 pathspec 只含本卡，避开同工作树内并行流的暂存）。领取时 todo 卡存在、doing 为空、依赖无。
- 回执：（报告链接）
- 验收：（裁定文件）
