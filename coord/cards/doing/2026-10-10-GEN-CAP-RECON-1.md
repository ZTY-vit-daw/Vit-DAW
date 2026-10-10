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

- 领取：（时间 / origin/main hash / owner）
- 回执：（报告链接）
- 验收：（裁定文件）
