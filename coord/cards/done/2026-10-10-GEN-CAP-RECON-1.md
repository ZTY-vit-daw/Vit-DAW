# GEN-CAP-RECON-1：生成能力线现状勘察（Mac 线候选①）

- 发卡：GLM 主管决策侧 / 2026-10-10 夜（依据：用户裁定三线放行+Mac 线候选讨论）
- 派发确认：已确认（用户 2026-10-10 裁定）
- 验收负责人：GLM 主管决策流
- 池序 56；目标仓库=D:\Vit_DAW（+前端仓只读）；只读勘察零代码
- 优先级 / 预估 / 依赖：P1 / 1-2h / 无
- 模型分级：L0 / 任意引擎（含闲时）

## 目标（报告四节）

1. **生成链底盘**：VitApp GeneratedAssetService 能力面（生成什么、命令面、落盘链）；MIDI 生成链现状（MIDI-RECON-1/MIDI-CMD-REGISTER-1 产出后的命令注册面）；素材/stem 生成相关 Service 清点。
2. **中枢冻存卡重核**（hub queue 五张，2026-10-05 冻结——当时假设与现 harness 状态可能有偏差）：AIGC-DRUM-GEN-1（鼓型生成）/AIGC-DRUM-GEN-2（多候选伴奏）/SHOW-REPRO-EXP-1（生成可复现性）/TIMBRE-SWITCH-1（对话式音色切换）——逐卡：目标/依赖的假设是否仍成立/在自由态 harness 上的新承载形态建议。
3. **Mac 面核验**：生成线所需栈面在 Mac 的就绪度（GeneratedAssetService/MIDI/资产链是否 PORT 系列已覆盖；Mac 栈跑生成链的已知缺口）。
4. **启动建议输入**：若 Mac 选生成——首批 2-3 张卡建议（含中枢卡迁移登记路径）。

## 文件域

零代码；报告落 coord/runs/GEN-CAP-RECON-1/（中枢卡文件只读引用，不迁移不动）。

## 验收标准

四节齐+锚点；冻存卡重核逐张给"假设仍成立/需修订"结论。

- 领取：2026-10-10 21:08 / origin/main=1c1a447a（卡移动经并行会话 MIX 领取提交 076ce106 一并入库，领取回填由本流单独提交）/ owner=GLM-5.3 flash（GEN-CAP-RECON-1 执行会话）@ D:\Vit_DAW 主工作树（PC）
- 回执：**报告 [coord/runs/GEN-CAP-RECON-1/report.md](../../runs/GEN-CAP-RECON-1/report.md)（四节齐+锚点+附录 A）**。零代码零真栈运行（SSH 仅 2 次只读 git/ls 核查 Mac 仓，无写操作）。核心结论：①内核 assets 三命令+ midi 写命令族（K2 后七条）+ start_render 全部已注册，LLM 工具面齐备，无注册缺口；②端到端出声两腿：K1 渲染冻结已修复（79b8ecdc）、**A1 instrument 装载通道仍缺（PCA 零 instrument 族，维持待用户裁定）**；③冻存卡重核：DRUM-GEN-1/2 需修订（小：出声腿挂 A1+候选承载形态），SHOW-REPRO 假设全成立，TIMBRE-SWITCH 阻塞在用户 A1 裁定；hub queue 实为 8 张，生成线相关 4 张（卡面"五张"差异已如实记录）；④**Mac 关键缺口：main 缺 K1/K2 修复（merge-base 实证）+ 内核二进制停在 9-18 构建 + PORT 系列对生成资产链零覆盖**；⑤首批建议：GEN-MAC-BASELINE-1（基线对齐+五命令首勘）→AIGC-DRUM-GEN-1（数据级验收）+A1-MAC-FORM-1（并行勘察），迁移登记路径见报告 §4.2（主管执行，本卡未动中枢文件）。附录 A 注记：勘察收尾时用户裁定 AMV/成片并入生成线（VIS-FILM-DESIGN-1 入池 15a7264b）——第 4 节按主管要求留待与其合并复核。端测覆盖边界：本卡为只读勘察，不涉 webui 渲染面与用户旅程，无烟测义务。
- 验收：（裁定文件）
